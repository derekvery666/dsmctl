---
id: WI-006
title: Establish focused Control Panel modules
status: done
priority: P1
owner: ""
depends_on: []
parallel_group: C
touches:
  - internal/domain/controlpanel
  - internal/synology/operations
  - internal/synology/controlpanel.go
  - internal/application
  - internal/cli
  - internal/mcpserver/server.go
---

# WI-006 — Establish focused Control Panel modules

## Outcome

Control Panel settings grow as typed, capability-driven modules instead of a
generic key/value or raw API proxy. The first change ships one read-only module
end to end and leaves a repeatable module pattern.

## Selected first module: `time`

The first slice is read-only regional time and NTP configuration. Its stable
operation and capability name is `controlpanel.time.read`; the DSM backend is
selected independently from every future Control Panel module.

Primary DSM evidence collected from the configured DSM 7.3.2 test NAS on
2026-07-16:

- `SYNO.API.Info.query` advertises `SYNO.Core.Region.NTP` at `entry.cgi`,
  versions 1 through 3, using JSON request format.
- DSM Admin Center's `Region.NTPTab` declares `SYNO.Core.Region.NTP` version 3
  with `get` and `set` methods.
- Read-only live `get` calls succeeded at all three advertised versions. V2
  and V3 returned `timezone`, `date_format`, `time_format`, `enable_ntp`, and
  `server`; V1 returned the same configuration except the two display-format
  fields.

This module was selected because one side-effect-free call returns a small,
stable configuration boundary, the compatibility difference is confined to
one decoder, and its absence can be reported without disabling another module.
Volatile wall-clock response fields are intentionally omitted from the domain
state. A later mutation work item must define time-change/NTP safety, stale
state, and postcondition rules before the advertised `set` method is exposed.

## Scope

- Inventory candidate modules and choose the first low-risk slice using primary
  DSM API evidence. Recommended first candidates are time/NTP, regional options,
  or service status; network/firewall writes are not first.
- Define module naming, state query, capabilities, and future change intent.
- Implement one read-only module through domain, variant, facade, CLI, and MCP.
- Document how later modules register without expanding one giant state object.

## Non-goals

- A generic `controlpanel set key=value` command.
- Network, firewall, certificate, reverse proxy, or update mutations in the
  first slice.
- Returning every undocumented DSM field.

## Acceptance criteria

- [x] The selected module and choice rationale are recorded in this spec.
- [x] Its operations are independently selectable by API/version.
- [x] Missing module APIs do not break other Control Panel modules.
- [x] CLI and MCP use the same application query/result.
- [x] Fixture and read-only integration tests pass.
- [x] A follow-up work item describes mutation safety for that module.

## Verification

- Read-only live checks are allowed on an explicitly configured NAS.
- `go test ./...` and `go vet ./...`.

## Coordination

This item owns the initial module registry/pattern. Other Control Panel agents
should wait for that pattern or coordinate before adding parallel modules.

## Handoff

- Working-tree state: uncommitted focused core in
  `internal/domain/controlpanel`,
  `internal/synology/operations/controlpaneltime`, and
  `internal/synology/controlpanel.go`.
- Completed: typed time module state/capabilities, API-version-scoped V1/V2/V3
  selectors, strict stable-field decoders, request/fixture tests, facade, and
  primary DSM evidence above.
- Verification: focused `go test` and `go vet` pass for the new operation,
  domain, and Synology packages. A read-only facade check against DSM 7.3.2
  selected `core-region-ntp-v3` and returned normalized time/NTP state.
- Remaining: only the separately scoped follow-up mutation-safety work item;
  application, CLI/MCP, compatibility reporting, and user docs are complete.
- Blockers: none. V1/V2 response shapes were verified by explicitly requesting
  those advertised versions on DSM 7.3.2; a physical older DSM remains untested.
- Temporary resources: none; live verification performed read-only calls only.

## Completion record

- Completed end to end on 2026-07-16; follow-up mutation policy is tracked in
  [WI-011](WI-011-control-panel-time-mutation.md).
- `dsmctl control-panel time state --json` returned `Taipei`, `Y-m-d`, `H:i`,
  NTP mode, and `time.google.com` on DSM 7.3.2 through the selected v3 backend.
- Verified with `go test ./... -count=1`, `go vet ./...`, and
  `git diff --check`. No Control Panel mutation was run.
