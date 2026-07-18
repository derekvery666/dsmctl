---
id: WI-036
title: Guarded Surveillance Station Home Mode switch
status: done
priority: P2
owner: ""
depends_on: [WI-034]
parallel_group: C
touches:
  - internal/domain/surveillance
  - internal/synology/operations/surveillance
  - internal/synology/surveillance.go
  - internal/application/surveillance.go
  - internal/cli/surveillance.go
  - internal/mcpserver/server.go
---

# WI-036 — Guarded Surveillance Station Home Mode switch

## Outcome

Extends the read-only Surveillance module (WI-034) with its first guarded write:
switching Home Mode on or off, through the hash-bound plan/apply flow.

## Scope

- Read Home Mode state (`SYNO.SurveillanceStation.HomeMode` `GetInfo` → `on`).
- Guarded switch (`SYNO.SurveillanceStation.HomeMode` `Switch` with `on`), a
  patch-only bool; plan records the observed state and hashes it, apply rejects a
  changed state and verifies the postcondition (`on` matches).
- Package-version gated on `SurveillanceStation`.
- CLI (`surveillance homemode state|plan|apply`) and three MCP tools.

## Non-goals

- Home Mode schedule, geofence, per-profile recording/notification settings
  (the `Save*` methods); camera and recording management.

## Design constraints

- Confirmed live on Surveillance 9.2.5: `GetInfo` returns a large object; only
  the top-level `on` bool is consumed. `Switch` takes `on` (bool). Both v1.
- Switching Home Mode changes the active recording and notification profile
  (medium risk); it is fully reversible (switch back).

## Acceptance criteria

- [x] Home Mode decodes `on`; the Switch request sends `on` (request-capture test).
- [x] Package-version gated; fails closed without SurveillanceStation.
- [x] Guarded plan/apply with stale-state rejection and postcondition
      verification; CLI + three MCP tools (get read-only, plan read-only, apply
      mutation) with read-only-gateway exclusion of plan/apply (tool count
      79 -> 82).
- [x] DSM 7.3.2 live verification (lab, authorized, reverted): switched Home Mode
      off -> on -> off through plan/apply.

## Verification

- Decoder/request-capture test; `go test ./... -count=1`, `go vet ./...`.
- Live reversible toggle on the DSM 7.3.2 lab NAS (Surveillance 9.2.5).
