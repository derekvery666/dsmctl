---
id: WI-026
title: Guarded SMB advanced settings and service discovery
status: proposed
priority: P2
owner: ""
depends_on: [WI-012]
parallel_group: C
touches:
  - internal/domain/controlpanel
  - internal/synology/operations/fileservices
  - internal/synology/operations/servicediscovery
  - internal/synology/controlpanel.go
  - internal/application
  - internal/cli
  - internal/mcpserver/server.go
  - docs/control-panel.md
---

# WI-026 — Guarded SMB advanced settings and service discovery

## Outcome

The SMB module exposes the DSM "Advanced Settings" surface beyond the WI-012
base six fields, and a separate service-discovery module exposes Bonjour,
WS-Discovery, and Time Machine over SMB. Both use the hash-bound plan/apply
flow.

## Scope

- SMB advanced settings (extending the `SYNO.Core.FileServ.SMB` snapshot):
  Local Master Browser, opportunistic locking, SMB durable handles, allow
  symbolic links within and across shared folders, veto files, WINS server,
  macOS-compatible extensions, and the transfer-log switch plus syslog target.
- A `SYNO.Core.FileServ.ServiceDiscovery` module: Bonjour service advertising,
  Time Machine broadcast over SMB, and WS-Discovery.
- Independent read/set selection and capability reporting for each surface.

## Non-goals

- Custom Windows ACL entries and share-level SMB permissions (WI-008).
- Active Directory / LDAP / domain join and Kerberos.
- SMB per-share overrides.

## Design constraints

- Confirm the exact `SYNO.Core.FileServ.SMB` advanced field names and the
  `ServiceDiscovery` API name/version against DSM source and NAS API discovery
  before wiring variants (WI-012 evidence discipline). Do not guess field names.
- SMB advanced set is full-snapshot: refresh, merge approved fields, submit,
  verify. SMB and service discovery remain separate compatibility boundaries.
- Enabling symbolic-link following or disabling signing/oplock protections is
  high risk; a transfer-log toggle is medium.
- No live SMB or service-discovery mutation without new explicit authorization.

## Acceptance criteria

- [ ] SMB advanced fields decode with strict validation and semantic names.
- [ ] Service discovery is an independent module with its own capability row.
- [ ] Advanced/discovery apply preserves every unspecified snapshot field.
- [ ] Request-capture tests lock each enabled set shape.
- [ ] CLI and MCP reuse the file-service application contract.
- [ ] No live mutation ran without new explicit authorization.

## Verification

- Decoder fixtures and request-capture tests per surface.
- `go test ./... -count=1`, `go vet ./...`, CLI and MCP builds.
- Read-only state/capability checks on the configured DSM 7.3.x NAS.

## Coordination

Shares the fileservices package with WI-012/WI-025 and `server.go` with the
other file-protocol items. Confirm the ServiceDiscovery API surface first.
