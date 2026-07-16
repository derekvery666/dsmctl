---
id: WI-001
title: Define the storage mutation contract
status: done
priority: P0
owner: ""
depends_on: []
parallel_group: A
touches:
  - internal/domain/storage
  - internal/application
  - internal/cli/storage.go
  - internal/mcpserver/server.go
  - docs/architecture.md
---

# WI-001 — Define the storage mutation contract

## Outcome

Storage changes have stable manifest and plan schemas shared by CLI and MCP,
without enabling a DSM storage write yet. Later pool/volume variants can plug
into the contract without changing agent-facing intent.

## Scope

- Define resource types for storage pools and volumes.
- Define create/update/delete intents and explicit topology references.
- Define a storage plan with stable IDs, fingerprints, warnings, destructive
  consequences, and an approval hash.
- Define capability fields for each future mutation independently.
- Add CLI/MCP plan/apply schemas that return unsupported until a matching write
  backend exists, or keep adapters unregistered until the agreed contract test
  passes. Record the chosen behavior.
- Add canonical hashing and validation tests.

## Non-goals

- Calling a DSM storage mutation API.
- Live storage mutation testing.
- Encrypted volumes, remote volumes, SSD cache, hot spare, or repair workflows.

## Design constraints

- Disk identity must use stable DSM identifiers, not display order.
- RAID type, selected disks, filesystem, and capacity policy are explicit.
- The plan must distinguish data-destructive deletion from additive expansion.
- A partial manifest must not reset unspecified pool or volume properties.
- Plan schema decisions must support operation-specific DSM variants.

## Acceptance criteria

- [x] Domain models express SHR, RAID 0/1/5/6/10, JBOD, and Basic when the
      inventory reports them, without assuming every NAS supports every type.
- [x] Invalid topology references and duplicate disks are rejected.
- [x] Plan hashes change when intent, stable IDs, or observed topology changes.
- [x] CLI and MCP consume the same application request and plan types.
- [x] No storage mutation request reaches the WebAPI executor.
- [x] Architecture and examples document the ownership semantics.

## Verification

- `go test ./...`
- `go vet ./...`
- Request/plan fixtures only. Live mutations are forbidden for this item.

## Coordination

This item owns shared storage plan types and should land before WI-002 or
WI-003 edits the same domain/application files.

## Completion record

- Completed on 2026-07-16 with pool/volume manifests, canonical plans,
  stable-reference preconditions, topology fingerprints, destructive
  consequences, and shared CLI/MCP schemas.
- Both plan/apply adapters fail closed with
  `ErrStorageMutationBackendUnavailable` before runtime or DSM access.
- Verified with `go test ./... -count=1`, `go vet ./...`, and
  `git diff --check`. No live storage mutation was run.
