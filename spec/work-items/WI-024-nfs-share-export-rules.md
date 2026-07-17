---
id: WI-024
title: Guarded per-shared-folder NFS export rules
status: proposed
priority: P1
owner: ""
depends_on: [WI-012]
parallel_group: C
touches:
  - internal/domain/nfsexport
  - internal/synology/operations/nfsexport
  - internal/synology/nfsexport.go
  - internal/synology/compatibility_report.go
  - internal/application
  - internal/runtime
  - internal/cli
  - internal/mcpserver/server.go
  - docs/control-panel.md
---

# WI-024 — Guarded per-shared-folder NFS export rules

## Outcome

A CLI user or MCP agent can read the NFS export rule set of one shared folder,
discover whether the export backend is supported, and plan/apply a complete
desired rule set through the same hash-bound approval flow used by the other
file-service modules. This closes the first WI-012 non-goal ("per-shared-folder
NFS host export rules").

## Scope

- Read one shared folder's NFS export rules via
  `SYNO.Core.FileServ.NFS.SharePrivilege` v1 `load` (keyed by `share_name`).
- Normalized rule fields: client pattern, privilege (read-write / read-only),
  root squash mapping, security flavor, asynchronous writes, allow connections
  from non-privileged ports, and allow access to mounted subfolders.
- Independent read and set compatibility selection for the export backend.
- Full-desired-state ownership: a plan carries the complete replacement rule
  set for one shared folder; `save` submits the whole `rule` array. This mirrors
  the time module's `ntp_servers` set-replace field, not a per-rule patch.
- Hash-bound plan/apply with observed-state fingerprint, stale-state rejection,
  network-exposure warnings, and postcondition verification.
- Thin CLI and MCP adapters over one application contract.

## Non-goals

- Kerberos keytab upload and NFS ID-map management
  (`SYNO.Core.FileServ.NFS.Kerberos`, `.IDMap`); track separately.
- The ActiveBackup-only rule fields `fsid` and `share_subdir`.
- Creating, deleting, or otherwise mutating the shared folder itself
  (owned by the shared-folder mutation module).
- Enabling the global NFS service (owned by WI-012).

## Design constraints

- DSM evidence: API method table in `webapi-NFS/src/SYNO.Core.FileServ.NFS.cpp`
  (`SYNO.Core.FileServ.NFS.SharePrivilege` v1 `load`/`save`); rule field names
  and enumerations in `webapi-NFS/src/share_privilege.cpp`:
  `client`, `privilege` (`rw`/`ro`), `root_squash`
  (`root`/`admin`/`guest`/`all_admin`/`all_guest`), `async`, `insecure`,
  `crossmnt`, `security_flavor`
  (`sys`/`kerberos`/`kerberos_integrity`/`kerberos_privacy`), and `id`
  (blank to create, the previous `client` to rename).
- The domain exposes semantic enum names, never raw DSM strings. `insecure`
  becomes `allow_nonprivileged_ports`; `crossmnt` becomes
  `allow_subfolder_access`; `root_squash=root` becomes `no_mapping`.
- Because `save` replaces the whole set, apply reads the current rule set,
  rejects a changed observed fingerprint, submits the complete approved set,
  and re-reads to verify.
- Broadening access is high risk: a rule with a wildcard client (`*`) or a
  write privilege, or removing an existing restricting rule, is high risk;
  a strictly narrowing change is medium.
- No live NFS export mutation runs without new explicit authorization for the
  exact shared folder under test.

## Acceptance criteria

- [ ] Export decoder exposes only stable semantic fields and rejects malformed
      responses instead of returning an empty rule set.
- [ ] Read and set support is selected independently and reported in
      capabilities with API/version evidence.
- [ ] CLI and MCP share the same application plan/apply contract.
- [ ] Apply rejects stale observed state, submits the full desired rule set,
      and verifies a fresh postcondition.
- [ ] Request-capture tests lock the `save` request shape, including `id`
      handling for create versus rename.
- [ ] DSM 7.3.x read-only `load` verification passes on a real shared folder.
- [ ] No live `save` ran without new explicit authorization.

## Verification

- Sanitized `load` decoder fixtures and `save` request-capture tests.
- `go test ./... -count=1`, `go vet ./...`, CLI and MCP builds.
- Read-only export `load` on the configured DSM 7.3.x NAS.

## Coordination

Extends the file-service module family established by WI-012 and reuses the
shared-folder inventory to resolve share names. `internal/mcpserver/server.go`,
`internal/application/service.go`, and the compatibility report are
high-contention; coordinate with any concurrent WI-025/WI-026 owner. The user
approved starting this on 2026-07-18.
