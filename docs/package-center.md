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

## Settings (read-only)

`dsmctl package settings` reads the global settings exposed by
`SYNO.Core.Package.Setting`: the publisher trust level (`synology`,
`synology_and_trusted`, or `any`) and the automatic-update policy. DSM's three
automatic-update choices map to two booleans:

| DSM choice | `auto_update_enabled` | `auto_update_important_only` |
| --- | --- | --- |
| Do not install automatically | `false` | (ignored) |
| Install important updates only | `true` | `true` |
| Install all updates | `true` | `false` |

Settings **changes are deferred** in this release; `capabilities` reports
`settings_set: false` and a `settings` plan is refused. DSM does not accept these
values through a single `set` call: the setting surface is split across
`SYNO.Core.Package.Setting.Update` (auto-update),
`SYNO.Core.Package.Setting.Volume` (default volume), and the base
`SYNO.Core.Package.Setting` (which handles only notifications and the update
channel and silently ignores trust level and auto-update). A correct settings
set therefore needs the per-section sub-APIs and is tracked as a follow-up; the
beta channel and default install volume come with that work.

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
- **settings changes**: `capabilities` reports `settings_set: false`, and a
  `settings` plan is refused, because the DSM set surface is split across
  per-section sub-APIs (see [Settings](#settings-read-only)).

The online catalog browse and per-package application-specific settings are also
tracked as a follow-up.

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
- Settings read: `SYNO.Core.Package.Setting` `get` v1. `trust_level` is an
  integer (0/1/2); `enable_autoupdate` is the master auto-update toggle, with
  `autoupdateimportant` / `autoupdateall` selecting important-only vs all.
  Settings set is deferred (fragmented across sub-APIs; see Deferred operations).
- Start/stop: `SYNO.Core.Package.Control` `start`/`stop` with `id` (verified live;
  DSM refuses to stop packages required by others, surfaced as an error).
- Uninstall: `SYNO.Core.Package.Uninstallation` `uninstall` with `id`.

Reads decode every optional field defensively, and each mutation operation gates
on `SYNO.API.Info` discovery: if a target does not advertise its API, that
operation reports unsupported and fails closed instead of issuing a wrong
request. Confirm the selected backends on any target with
`dsmctl package capabilities`.
