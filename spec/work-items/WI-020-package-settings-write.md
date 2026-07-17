---
id: WI-020
title: Guarded Package Center automatic-update settings write
status: done
owner: ""
priority: P2
depends_on: [WI-019]
parallel_group: C
touches:
  - internal/domain/packagecenter/model.go
  - internal/synology/operations/packagecenter
  - internal/application/package_center.go
  - docs/package-center.md
  - README.md
---

# WI-020 — Guarded Package Center automatic-update settings write

## Outcome

A CLI user or MCP agent can change the global Package Center automatic-update
policy through the existing hash-bound plan/apply flow, reusing the Package
Center `plan`/`apply` commands and the `plan_package_change` /
`apply_package_plan` MCP tools. This un-defers the settings-set that WI-019
shipped read-only.

## Scope

- Writable automatic-update policy (`auto_update_enabled` plus
  `auto_update_important_only`) via the base `SYNO.Core.Package.Setting` `set`,
  which writes the three DSM fields (`enable_autoupdate`, `autoupdateimportant`,
  `autoupdateall`) consistently.
- `capabilities` reports `settings_set: true`; the shared settings plan/apply
  path is enabled with patch-only ownership, stale-state rejection, and a fresh
  postcondition re-read.

## Non-goals

- **Trust level** stays read-only: no DSM WebAPI accepts a trust-level write and
  the base `set` silently ignores it. A trust-level change is not expressible in
  the settings patch.
- **Beta channel** (base `set` `update_channel`) and **default install volume**
  (`SYNO.Core.Package.Setting.Volume.set`) writes remain follow-ups.
- Install/update and the online catalog browse remain deferred (WI-019).

## Design constraints

- The settings set surface on DSM is fragmented, but the auto-update fields are
  accepted by the base `SYNO.Core.Package.Setting` `set` even though its response
  echoes only the notification/channel fields (verified live on DSM 7.3-81168).
- Apply merges the patch into a freshly read full settings state, submits the
  full auto-update triple, and verifies the requested fields; a set that did not
  take effect fails the postcondition rather than reporting a false success.

## Acceptance criteria

- [x] `capabilities` reports `settings_set: true`.
- [x] The encoder writes only the three auto-update fields; trust level is never
      sent.
- [x] A settings `plan`/`apply` toggles the auto-update policy and verifies the
      postcondition; a no-op patch and a trust-level field are rejected.
- [x] Request-capture and application unit tests cover the set shape and the
      plan/apply/stale/no-op/tamper paths.
- [x] Live plan/apply verified against a configured DSM NAS.

## Verification

- `go test ./... -count=1`, `go vet ./...`, `gofmt`, CLI and MCP builds.
- Live plan/apply round-trip on the configured NAS.

## Coordination

Follows WI-019 (Package Center) and touches the same package. The user requested
the settings-write follow-up on 2026-07-17 and authorized live changes on the
configured test NAS.

## Completion record

- Completed on 2026-07-17. The Package Center settings-set operation was
  un-deferred and scoped to the automatic-update policy, routed through the base
  `SYNO.Core.Package.Setting` `set`; trust level was removed from the writable
  patch and stays read-only.
- Verified with `go test ./... -count=1`, `go vet ./...`, `gofmt` clean, and all
  three binary builds.
- Live verification on a DS-model NAS running **DSM 7.3-81168** (user-authorized):
  a `settings` plan/apply disabled automatic updates (`applied: true`, verified
  "no"), then re-enabled it (verified "yes"), through
  `SYNO.Core.Package.Setting.set`. The NAS was left in its original state.
- The DSM `set` response echoes only notification/channel fields, so correctness
  relies on the postcondition re-read, which confirmed the change both ways.
