---
id: WI-028
title: Guarded rsync service and TFTP file services
status: proposed
priority: P3
owner: ""
depends_on: [WI-006]
parallel_group: C
touches:
  - internal/domain/controlpanel
  - internal/synology/operations/rsync
  - internal/synology/operations/tftp
  - internal/synology/controlpanel.go
  - internal/application
  - internal/cli
  - internal/mcpserver/server.go
  - docs/control-panel.md
---

# WI-028 — Guarded rsync service and TFTP file services

## Outcome

A CLI user or MCP agent can read and, through hash-bound plan/apply, change the
rsync network-backup service (enable state, port, rsync account switch) and the
TFTP service (enable state, root folder, allowed clients).

## Scope

- rsync service global settings used by network backup.
- TFTP service global settings.
- Independent read/set selection and capability reporting per protocol.

## Non-goals

- rsync backup task or destination management.
- AFP (deprecated in DSM 7.x) and WebDAV (a separate installable package best
  handled by the WI-022 package-scoped framework, not core File Services).

## Design constraints

- Confirm DSM API names/versions before wiring variants
  (candidates: `SYNO.Core.FileServ.Rsync`, `SYNO.Core.FileServ.TFTP`); do not
  guess field names.
- TFTP root folder must resolve to an existing shared-folder path; reject
  otherwise before any write.
- Exposing TFTP (no authentication) or enabling the rsync account is high risk.
- No live rsync/TFTP mutation without new explicit authorization.

## Acceptance criteria

- [ ] rsync and TFTP states decode with semantic fields and strict validation.
- [ ] Read/set support selected independently with API evidence.
- [ ] Apply preserves unspecified fields and verifies the postcondition.
- [ ] Request-capture tests lock each enabled set shape.
- [ ] CLI and MCP reuse one application contract.
- [ ] No live mutation ran without new explicit authorization.

## Verification

- Decoder fixtures and request-capture tests per protocol.
- `go test ./... -count=1`, `go vet ./...`, CLI and MCP builds.
- Read-only state/capability checks on the configured DSM 7.3.x NAS.

## Coordination

Lowest-priority file-protocol items; independent operation packages minimize
overlap. AFP and WebDAV are explicitly out of scope and recorded here so they
are not re-proposed without a product decision.
