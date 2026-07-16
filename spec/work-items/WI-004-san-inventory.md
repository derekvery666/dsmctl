---
id: WI-004
title: Add read-only SAN inventory
status: done
priority: P0
owner: ""
depends_on: []
parallel_group: B
touches:
  - internal/domain/san
  - internal/synology/operations/saninventory
  - internal/synology/san.go
  - internal/application/service.go
  - internal/cli
  - internal/mcpserver/server.go
  - docs
---

# WI-004 — Add read-only SAN inventory

## Outcome

CLI and MCP can read normalized iSCSI targets, LUNs, target/LUN mappings, and
health/state from a selected NAS without exposing raw DSM responses.

## Scope

- Discover target, LUN, and mapping APIs from DSM primary sources.
- Stable domain models for IDs, names, protocol/state, capacity, provisioning,
  backing volume, mappings, and health fields actually reported.
- Operation-scoped variants and capability reporting.
- `san capabilities` and `san inventory` CLI commands.
- Matching read-only MCP tools.

## Non-goals

- Any SAN mutation.
- Fibre Channel unless the test model advertises and the spec is extended.
- Snapshot, replication, backup, or VMware/VAAI policy.

## Design constraints

- Mappings reference stable target/LUN IDs, not names alone.
- Unknown DSM fields are not copied wholesale into public models.
- Missing optional SAN packages should report unsupported, not fail unrelated
  NAS capabilities.

## Acceptance criteria

- [x] Inventory composes targets, LUNs, and mappings without N+1 calls when a
      bulk API is available.
- [x] Thin and thick provisioning are normalized when DSM reports them.
- [x] Capability reports name every selected inventory operation.
- [x] CLI and MCP return equivalent structured state.
- [x] Fixture tests cover empty, current, and legacy-compatible responses.

## Verification

- Read-only checks on an explicitly configured NAS are allowed.
- `go test ./...` and `go vet ./...`.
- Record tested DSM version and missing-package behavior in the work item.

## Coordination

This item may run in parallel with storage work by owning new `domain/san` and
operation packages. Coordinate only when editing shared MCP/service files.

## Discovery evidence

- DSM `7.3.2-86009 Update 1` on the configured DS1621+ advertises
  `SYNO.Core.ISCSI.Target` and `SYNO.Core.ISCSI.LUN` v1 at `entry.cgi` with
  JSON request format.
- The installed SAN Manager `ScsiTarget/iscsi.js` uses one bulk target `list`
  call with `mapped_lun`, `acls`, `connected_sessions`, and `status`, plus one
  bulk LUN `list` call with the supported SAN LUN type filter and additional
  state/allocation fields. Target `mapped_luns[].lun_uuid` is the mapping
  source used by the DSM UI itself.
- Read-only live calls with those exact parameters returned valid empty
  `targets` and `luns` arrays. No live mutation was performed.
- Fixture selection tests model a missing SAN Manager package as two explicit
  unsupported operation selections rather than an error in unrelated modules.

## Completion record

- Completed on 2026-07-16 through domain, operation variants, Synology facade,
  application service, CLI, MCP, and global compatibility reporting.
- `dsmctl san inventory --json` returned empty target/LUN/mapping arrays on DSM
  7.3.2, and both operations selected their v1 bulk backend.
- Verified with `go test ./... -count=1`, `go vet ./...`, and
  `git diff --check`. No SAN mutation was run.
