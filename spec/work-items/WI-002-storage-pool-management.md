---
id: WI-002
title: Implement guarded storage-pool management
status: done
priority: P0
owner: ""
depends_on: [WI-001]
parallel_group: A
touches:
  - internal/synology/operations/storagepoolmutation
  - internal/synology/storage.go
  - internal/synology/compatibility_report.go
  - internal/application
  - integration
---

# WI-002 — Implement guarded storage-pool management

## Outcome

A user or agent can plan supported storage-pool creation, expansion, and
deletion with a DSM-version-specific backend and explicit destructive warnings.

## Scope

- Discover and record actual DSM request/response schemas from primary sources.
- Implement independently selectable create, expand, and delete operations.
- Calculate applicable RAID choices from model capability and selected disks.
- Re-read disk/pool state before apply and verify topology afterward.
- Report unsupported operations rather than approximating them.

## Non-goals

- Pool repair, drive replacement, RAID migration, SSD cache, or hot spare.
- Assuming SHR behavior from conventional RAID behavior.
- Running a destructive call on the currently configured NAS.

## Design constraints

- Create, expand, and delete are separate compatibility selections.
- A disk must be healthy, unused, and unchanged since planning.
- Deletion is always high risk and requires the observed pool stable ID.
- API discovery evidence and sample responses belong in tests/fixtures without
  credentials, hostnames, serial numbers, or user data.

## Acceptance criteria

- [x] Capability output describes create/expand/delete independently.
- [x] Request-capture tests cover each supported RAID/topology mapping.
- [x] Stale disk or pool state invalidates apply.
- [x] Postconditions verify member disks, RAID type, and pool status.
- [x] Unsupported DSM versions remain read-only.
- [x] CLI and MCP expose the same guarded intent.

## Verification

- `go test ./...`
- `go vet ./...`
- No live mutation without explicit authorization for a disposable NAS and
  disks. Read-only capability checks are allowed.

## Coordination

The WI-001 dependency is complete. WI-003 must preserve the shared pool/volume
inventory fields and keep volume mutations in their own operation package.

## Handoff

- Working-tree state: uncommitted WI-002 implementation in
  `internal/synology/operations/storagepoolmutation`, the shared storage
  facade/application/runtime boundary, storage inventory safety fields, CLI
  wording, tests, and `docs/storage-management.md`.
- Completed: three independently selected `SYNO.Storage.CGI.Pool` v1 variants
  for `create`, `expand_by_add_disk`, and `remove`; DSM RAID `device_type`
  mappings; model/disk-count applicable RAID choices; healthy/unused disk and
  pool actionability checks; topology plus safety fingerprints; pre-apply
  reread; and create/expand/delete postcondition verification.
- Primary evidence: read-only API catalog, Desktop Initdata, and
  `SYNO.Storage.CGI.Storage.load_info` discovery against DSM 7.3.2, plus the
  NAS-local Storage Manager `storage_panel.js` and `utils.js` request assembly.
  Sanitized request captures and inventory subsets are recorded in unit tests.
- Verification: `go test ./... -count=1`, `go vet ./...`, and
  `git diff --check` pass. CLI and MCP both use the same application
  `plan_storage_change`/`apply_storage_plan` intent and approval hash.
- Remaining: volume mutation remains intentionally fail closed for WI-003;
  RAID migration, repair, replacement,
  cache, spare, and RAID-group composition remain non-goals.
- Blockers: none.
- Temporary resources: none. Discovery was read-only; no live storage
  mutation was executed and no NAS resource was created, changed, or deleted.
