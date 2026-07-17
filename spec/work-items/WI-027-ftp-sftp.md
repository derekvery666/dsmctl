---
id: WI-027
title: Guarded FTP/FTPS and SFTP file services
status: proposed
priority: P2
owner: ""
depends_on: [WI-006]
parallel_group: C
touches:
  - internal/domain/controlpanel
  - internal/synology/operations/ftp
  - internal/synology/controlpanel.go
  - internal/application
  - internal/cli
  - internal/mcpserver/server.go
  - docs/control-panel.md
---

# WI-027 — Guarded FTP/FTPS and SFTP file services

## Outcome

A CLI user or MCP agent can read and, through hash-bound plan/apply, change the
FTP/FTPS and SFTP services: enable state, ports, TLS/security, passive-port
range, reported external IP, brute-force protection, UTF-8, anonymous access,
and transfer speed limits.

## Scope

- FTP and FTPS (explicit/implicit TLS) global service settings.
- SFTP (SSH file transfer) enable state and port.
- Independent read/set selection and capability reporting per protocol.
- Full-snapshot merge-and-submit ownership matching the other modules.

## Non-goals

- Per-user or per-group FTP privileges and speed limits.
- FTP over the DSM firewall / port-forwarding configuration.
- TFTP and rsync (WI-028).

## Design constraints

- Confirm the DSM API names/versions before wiring variants
  (candidates: `SYNO.Core.FileServ.FTP`, `SYNO.Core.FileServ.FTP.Security`,
  `SYNO.Core.FileServ.SFTP`); do not guess field names.
- Each protocol is a separate compatibility boundary and a separate CLI/MCP
  surface, even if DSM groups them on one page.
- Enabling FTP without TLS, opening a passive-port range, or enabling anonymous
  access is high risk; a UTF-8 or timeout tweak is medium.
- No live FTP/SFTP mutation without new explicit authorization.

## Acceptance criteria

- [ ] FTP/FTPS and SFTP states decode with semantic fields and strict
      validation.
- [ ] Read/set support selected independently per protocol with API evidence.
- [ ] Apply preserves every unspecified snapshot field and verifies.
- [ ] Request-capture tests lock each enabled set shape.
- [ ] CLI and MCP reuse one application contract.
- [ ] No live mutation ran without new explicit authorization.

## Verification

- Decoder fixtures and request-capture tests per protocol.
- `go test ./... -count=1`, `go vet ./...`, CLI and MCP builds.
- Read-only state/capability checks on the configured DSM 7.3.x NAS.

## Coordination

New operation package is an independent parallel boundary; only
`internal/mcpserver/server.go`, `internal/application/service.go`, and the
compatibility report overlap with the other file-protocol items.
