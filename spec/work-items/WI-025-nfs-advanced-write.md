---
id: WI-025
title: Complete guarded NFS advanced-setting writes
status: proposed
priority: P1
owner: ""
depends_on: [WI-012]
parallel_group: C
touches:
  - internal/domain/controlpanel
  - internal/synology/operations/fileservices
  - internal/synology/controlpanel.go
  - internal/application/file_services.go
  - internal/cli/file_services.go
  - internal/mcpserver/server.go
  - docs/control-panel.md
---

# WI-025 — Complete guarded NFS advanced-setting writes

## Outcome

The NFS module reports `set_advanced: true` and can write the NFSv4 ID-mapping
domain and the NFS packet-size and UNIX-permission advanced settings through the
existing hash-bound plan/apply flow. This removes the WI-012 fail-closed on
`nfsv4_domain` writes.

## Scope

- Extend the normalized NFS advanced state to include the full snapshot DSM's
  advanced-setting form submits: read/write packet size and the UNIX-permission
  switch, in addition to the already-read NFSv4 domain.
- Enable the `SelectNFSAdvancedSet`/`ExecuteNFSAdvancedSet` path with a complete
  merge-and-submit encoder so no unspecified advanced field is silently reset.
- Extend the NFS change intent with the advanced fields, keeping advanced writes
  planned separately from NFS base settings (as WI-012 already requires for
  `nfsv4_domain`).

## Non-goals

- Per-shared-folder NFS export rules (WI-024).
- Kerberos and ID-map management APIs.
- Changing NFS base protocol enablement inside the advanced path.

## Design constraints

- DSM evidence: `SYNO.Core.FileServ.NFS.AdvancedSetting` v1 `get`/`set` in
  `webapi-NFS/src/SYNO.Core.FileServ.NFS.cpp` and `src/nfsAdv.cpp`; the full
  advanced snapshot fields observed in `synoc2-ansible/cms/ds_configure.sh`:
  `nfs_v4_domain`, `read_size`, `write_size`, `unix_pri_enable`
  (with `enable_nfs`, `enable_nfs_v4`, `enabled_minor_ver` owned by base set).
- Advanced set is full-snapshot: apply refreshes the complete advanced state,
  merges only the approved fields, submits the whole snapshot, and verifies.
- `read_size`/`write_size` accept only the DSM-permitted discrete values;
  reject anything else before any write.
- Changing the NFSv4 domain or packet size can disrupt active clients and is
  high risk; toggling UNIX permissions is high risk.
- Domain writes still require NFSv4 to be enabled, matching DSM behavior.
- No live advanced `set` runs without new explicit authorization.

## Acceptance criteria

- [ ] NFS advanced state decodes domain, packet sizes, and the UNIX-permission
      switch with strict validation.
- [ ] `set_advanced` is reported `true` only when the advanced set backend is
      selected.
- [ ] Advanced apply refreshes and preserves every unspecified snapshot field.
- [ ] Request-capture test locks the advanced `set` snapshot shape.
- [ ] CLI and MCP expose the advanced write through the existing file-service
      plan/apply tools.
- [ ] No live advanced `set` ran without new explicit authorization.

## Verification

- Advanced `get` decoder fixture and advanced `set` request-capture test.
- `go test ./... -count=1`, `go vet ./...`, CLI and MCP builds.
- Read-only advanced `get` on the configured DSM 7.3.x NAS.

## Coordination

Edits the same fileservices package and `file_services.go` application/CLI as
WI-012 and shares `internal/mcpserver/server.go` with WI-024/WI-026. Only one
owner should hold the fileservices package at a time.
