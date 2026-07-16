---
id: WI-005
title: Implement guarded SAN management
status: done
priority: P1
owner: ""
depends_on: [WI-004, WI-001]
parallel_group: B
touches:
  - internal/domain/san
  - internal/synology/operations/sanmutation
  - internal/synology/san.go
  - internal/application
  - internal/cli
  - internal/mcpserver/server.go
---

# WI-005 — Implement guarded SAN management

## Outcome

Users and agents can compose target, LUN, and mapping lifecycle operations using
typed plan/apply rather than raw DSM calls.

## Scope

- Target create/update/delete.
- LUN create/update/delete with explicit backing volume, capacity, and
  provisioning policy.
- Mapping attach/detach as a separate resource.
- Stable-ID preconditions and referential-integrity validation.
- Independent operation variants and capabilities.

## Non-goals

- Snapshot/clone, replication, backup, or initiator-side configuration.
- Automatic deletion of mapped LUNs or targets with active sessions.
- Live target or mapping mutations on the configured NAS.
- Modifying or deleting any LUN not created by the current test run.

## Acceptance criteria

- [x] Delete plans refuse active sessions and unexpected mappings by default.
- [x] LUN capacity and backing-volume free space are checked at plan and apply.
- [x] Mapping changes never delete either endpoint.
- [x] Partial failure produces an actionable, retryable state report.
- [x] CLI and MCP share intent, plan, and result types.
- [x] Postconditions verify stable IDs and the mapping graph.

## Verification

- Request-capture and state-machine tests.
- `go test ./...` and `go vet ./...`.
- A live create/delete test is authorized for one disposable, unmapped LUN:
  - generate a unique `dsmctl-e2e-lun-*` name;
  - record the baseline and refuse to continue if that name already exists;
  - choose a backing volume from read-only inventory rather than guessing;
  - use the smallest practical test capacity and no target mapping;
  - capture the stable DSM LUN ID after create;
  - verify create state before deletion;
  - delete only when the current stable ID equals the captured ID;
  - verify the LUN is absent afterward and report any cleanup failure.
- This authorization does not cover target create/delete, mapping changes,
  snapshots, clones, expansion, or existing LUNs.

## Coordination

Reuse WI-001 plan conventions, but keep SAN domain types independent from
storage pool/volume manifests.

## Handoff

- Implemented stable target/LUN/mapping intents, eight independently selected
  DSM operations, capability reporting, guarded application plan/apply, CLI
  `san plan/apply`, and MCP `plan_san_change`/`apply_san_plan` using the same
  intent, plan, and result types.
- Safety validation refuses target deletes with sessions or mappings, LUN
  deletes with mappings, mapping changes with sessions, shrink requests,
  unresolved/readonly/unhealthy backing volumes, insufficient capacity, stale
  stable IDs, changed mapping graphs, and tampered approval artifacts. Target
  enable/disable cannot be combined with another patch.
- Postconditions verify returned stable IDs, requested target/LUN properties,
  LUN backing path and unmapped create state, mapping endpoint preservation,
  and mapping graph changes. Failure results re-read current state and include
  retryability, state fingerprint, resource existence, and mapping existence.
- Request-capture, planner, state-machine, failure-path, MCP schema, and strict
  volume-path decoder tests are present. `go test ./... -count=1` and
  `go vet ./...` passed on 2026-07-16.
- Authorized live DSM 7.3.2-86009 Update 1 test passed through MCP: created and
  deleted one 1 GiB thin, unmapped LUN named
  `dsmctl-e2e-lun-0c1bf9687212` on `volume_1` (`/volume1`), captured stable UUID
  `3bdcbda5-6e95-40c1-8761-3db5860cdc4b`, verified exact UUID/name/unmapped
  state before a fresh delete plan, then verified absence. Final target, LUN,
  and mapping inventory was empty; no temporary resources remain.
