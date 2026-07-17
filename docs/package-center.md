# Package Center

Package Center is a focused, typed module with its own state, capability names,
DSM API variants, CLI subtree, and MCP tools. It manages installed packages and
the global Package Center configuration without exposing a raw DSM installation
or settings proxy.

## Inventory and capabilities

```console
dsmctl package capabilities --nas office
dsmctl package inventory --nas office --json
dsmctl package settings --nas office --json
```

`inventory` returns each installed package with normalized, semantic fields: the
stable DSM id, display name, installed version, a normalized run status
(`running`, `stopped`, `starting`, `stopping`, `installing`, `error`, or
`unknown`), a running flag, a beta flag, the install volume when DSM reports it,
and whether DSM allows the package to be started, stopped, or uninstalled.

`capabilities` reports which operations are available and the DSM backend
selected for each. `install` and `update` are deliberately reported as
unsupported (see [Deferred operations](#deferred-operations)).

MCP exposes the same application results through `get_package_capabilities`,
`get_package_state`, and `get_package_settings`.

The inventory backend is `SYNO.Core.Package` `list`; settings use
`SYNO.Core.Package.Setting`. A missing Package Center API makes only this module
unsupported; storage, SAN, account, share, and Control Panel features are
unaffected.

## Settings

`dsmctl package settings` reads the global settings exposed by
`SYNO.Core.Package.Setting`: the publisher trust level (`synology`,
`synology_and_trusted`, or `any`) and the automatic-update policy. DSM's three
automatic-update choices map to two booleans:

| DSM choice | `auto_update_enabled` | `auto_update_important_only` |
| --- | --- | --- |
| Do not install automatically | `false` | (ignored) |
| Install important updates only | `true` | `true` |
| Install all updates | `true` | `false` |

The **automatic-update policy is writable** through the same hash-bound
plan/apply flow. A settings change is patch-only: an omitted field keeps its
current value. The plan records and hashes the complete current settings state;
apply rejects a changed state, merges the patch into a freshly read full state,
submits the three DSM auto-update fields consistently through
`SYNO.Core.Package.Setting.set`, and verifies the requested fields afterward.

```json
{
  "kind": "settings",
  "settings": { "auto_update_enabled": true, "auto_update_important_only": true }
}
```

```console
dsmctl package plan --nas office --file settings.json --output settings.plan.json
dsmctl package apply --file settings.plan.json --approve <hash-from-plan>
```

**Trust level is read-only** and cannot be changed: no DSM WebAPI writes it, and
the base `set` silently ignores it, so `trust_level` is not accepted in a change.
The beta channel and default install volume are likewise not writable here (see
[Deferred operations](#deferred-operations)).

## Guarded package lifecycle

The lifecycle actions on already-installed packages are `start`, `stop`, and
`uninstall`. A lifecycle change identifies the package by its stable DSM id and
binds the plan to the observed package state.

Example lifecycle request:

```json
{
  "kind": "lifecycle",
  "lifecycle": {
    "action": "stop",
    "package_id": "WebStation"
  }
}
```

```console
dsmctl package plan --nas office --file stop.json --output stop.plan.json
dsmctl package apply --file stop.plan.json --approve <hash-from-plan>
```

Planning refuses a no-op (starting a running package or stopping a stopped one),
refuses `uninstall` when DSM reports the package is not removable, and requires
the matching verified backend. `stop` is high risk because it interrupts the
package's service and any dependents; `uninstall` is destructive and high risk
because it removes the package and may delete its configuration and data.

Apply re-reads the inventory and verifies the terminal state: `start` expects a
running package, `stop` expects a stopped package, and `uninstall` expects the
package to be absent. If DSM is still mid-transition, apply returns an explicit
not-yet-confirmed error and asks the caller to re-check `package inventory`
rather than reporting a false success.

MCP exposes the same contract through `plan_package_change` and
`apply_package_plan`.

## Deferred operations

The following are modeled so capabilities and request validation can name them,
but they are **not implemented** and fail closed:

- `install` (from the online repository) and `update`: `capabilities` reports
  `install: false` / `update: false`, and a plan with `action: install` or
  `action: update` is rejected. They contact Synology's online repository, run
  asynchronously over minutes, and download and run remote code, which does not
  fit the synchronous plan/apply postcondition contract.

Writing the **trust level**, **beta channel**, and **default install volume** is
also not supported: trust level has no DSM write endpoint, and the beta channel
(base `Setting.set` `update_channel`) and default volume
(`SYNO.Core.Package.Setting.Volume.set`) are separate follow-ups. The online
catalog browse and per-package application-specific settings are deferred too.

## DSM backends (verified on DSM 7.3)

The API names and fields are verified against DSM 7.3-81168:

- Inventory: `SYNO.Core.Package` `list` v2 with
  `additional=["status","beta","startable","install_type"]`. That API rejects the
  whole request (error 120) if any requested key is unknown; `stoppable`,
  `removable`, and `installing` are **not** valid keys. `status`, `beta`,
  `startable`, and `install_type` are returned inside each package's `additional`
  object. `startable` marks a package that exposes a start/stop control (not one
  that can start right now), so `can_stop` is `startable && running`, `can_start`
  is `startable && not running`, and `can_uninstall` is `install_type != system`.
- Settings: `SYNO.Core.Package.Setting` `get`/`set` v1. `trust_level` is an
  integer (0/1/2, read-only); `enable_autoupdate` is the master auto-update
  toggle, with `autoupdateimportant` / `autoupdateall` selecting important-only vs
  all. The `set` write applies the auto-update fields even though its response
  echoes only the notification/channel fields (verified live); it silently
  ignores `trust_level`, which is why trust is read-only.
- Start/stop: `SYNO.Core.Package.Control` `start`/`stop` with `id` (verified live;
  DSM refuses to stop packages required by others, surfaced as an error).
- Uninstall: `SYNO.Core.Package.Uninstallation` `uninstall` with `id`.

Reads decode every optional field defensively, and each mutation operation gates
on `SYNO.API.Info` discovery: if a target does not advertise its API, that
operation reports unsupported and fails closed instead of issuing a wrong
request. Confirm the selected backends on any target with
`dsmctl package capabilities`.
