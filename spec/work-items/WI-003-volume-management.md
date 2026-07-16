---
id: WI-003
title: Implement guarded volume management
status: done
priority: P0
owner: ""
depends_on: [WI-001, WI-002]
parallel_group: A
touches:
  - internal/synology/operations/volumemutation
  - internal/synology/storage.go
  - internal/synology/compatibility_report.go
  - internal/application
  - integration
---

# WI-003 — Implement guarded volume management

## Outcome

A user or agent can plan and apply supported volume creation, capacity changes,
and deletion on a selected pool through the same storage contract.

## Scope

- Typed volume create/update/delete variants.
- Filesystem selection and capability validation.
- Capacity policy using explicit values or an explicit maximum policy.
- Pool free-space and volume stable-ID preconditions.
- Normalized postcondition verification.

## Non-goals

- Moving a volume between pools.
- Filesystem conversion, encryption lifecycle, snapshots, or package migration.
- Inferring a safe size from an omitted field.

## Acceptance criteria

- [x] Btrfs/ext4 choices are capability driven.
- [x] Capacity units are canonical and overflow checked.
- [x] Create/update/delete have independent compatibility selections.
- [x] Plan summaries show capacity and data-loss consequences.
- [x] Apply rejects changed pool capacity or volume identity.
- [x] CLI and MCP schemas remain identical at the application boundary.

## Verification

- Unit fixtures and request-capture tests.
- `go test ./...` and `go vet ./...`.
- No live mutation without explicit authorization for disposable storage.

## Coordination

Use the WI-001 contract unchanged where possible. Any required contract change
must be reviewed against the completed WI-002 pool behavior before implementation.
The user explicitly continued with the next P0 item on 2026-07-16; both
dependencies are complete. No live volume mutation is authorized.

## Completion evidence

- Added model-driven filesystem constraints from authenticated
  `SYNO.Core.Desktop.Defs.getjs` without placing SID or SynoToken in the URL.
- Added independent DSM v1 create, expand, and delete variants with sanitized
  request-capture tests for both single- and multi-volume layouts.
- Added explicit capacity resolution, GiB/MiB conversion checks, overflow
  rejection, stable pool paths/layout, stale-capacity checks, and normalized
  volume postconditions to the shared CLI/MCP plan/apply contract.
- `go test ./... -count=1`, `go vet ./...`, `go build ./cmd/dsmctl`, and
  `go build ./cmd/dsmctl-mcp` pass with Go 1.26.5.
- Read-only verification passed on DSM 7.3.2: the model advertised Btrfs and
  ext4, all three operation variants selected independently, and a create plan
  was rejected before apply because the existing pool lacked the minimum
  unallocated capacity. No volume mutation request was sent.
