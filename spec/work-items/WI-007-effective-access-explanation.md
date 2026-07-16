---
id: WI-007
title: Explain effective access
status: done
priority: P1
owner: ""
depends_on: []
parallel_group: D
touches:
  - internal/domain/identity
  - internal/domain/share
  - internal/application
  - internal/cli
  - internal/mcpserver/server.go
  - docs/account-share-management.md
---

# WI-007 — Explain effective access

## Outcome

A user or LLM can ask why a principal can or cannot access a shared folder or
application and receive a deterministic explanation of direct, group, deny,
and inherited rules.

For example, instead of returning only disconnected inventory records, the
result can explain:

```text
user alice -> member of group developers
developers -> write permission on shared folder projects
alice -> no direct deny
effective result -> write, because developers grants write
```

If a direct deny, custom Windows ACL, or IP-specific application rule prevents
a safe conclusion, the result identifies that evidence and returns deny or
indeterminate rather than guessing.

## Scope

- Read-only application use case combining existing memberships, share
  permissions, and application privilege state.
- Explain direct user rules, contributing group rules, explicit deny priority,
  application inheritance, and rules not modeled by dsmctl.
- Focused queries by principal plus share or application.
- CLI and MCP read-only surfaces with structured evidence and a concise summary.

## Non-goals

- Editing permissions.
- Claiming to fully evaluate custom Windows ACLs or IP-specific app rules.
- Simulating filesystem ACL inheritance below a shared-folder root.

## Acceptance criteria

- [x] The result identifies every observed rule used in the conclusion.
- [x] Explicit unknown/custom rules yield `indeterminate`, not a guessed allow.
- [x] User/group precedence is covered by table-driven tests.
- [x] Queries avoid expanding unrelated principals.
- [x] CLI and MCP return the same evidence model.
- [x] No new DSM mutation operation is introduced.

## Verification

- Table-driven unit tests for allow, deny, inherit, conflict, and custom cases.
- Read-only live comparison against temporary user/group/share resources is
  allowed under the existing `dsmctl-e2e-*` safeguards.
- `go test ./...` and `go vet ./...`.

## Coordination

Coordinate with any account/share schema work before editing shared domain
models. Prefer a new explanation result type over adding derived fields to raw
inventory models.

## DSM evidence

- DSM 7.3.2 Admin Center computes a user's coarse share permission from direct
  `is_*` flags plus the `inherit` string aggregate returned by
  `SYNO.Core.Share.Permission.list_by_user`: `na`, `cu`, `rw`, `ro`, or `-`.
- `SYNO.Core.AppPriv.App.preview` v2 returns the final application privilege in
  the deliberately misspelled `applications[].privilelge` field. Its result
  includes group, `everyone`/default, and built-in account policy, so it is the
  authoritative conclusion while `Rule.get` remains explanation evidence.
- `SYNO.Core.Share.list` exposes per-share `unite_permission`, meaning Advanced
  Share Permissions. Because those rules contain separate browse, modify, and
  download restrictions, the first explanation slice returns indeterminate
  whenever that flag is enabled.
- A read-only live check on DSM 7.3.2 returned a determinate inherited-write
  share explanation and an application allow confirmed by DSM preview. No
  account, share, ACL, or application rule was changed.

## Completion record

- Completed on 2026-07-16 with a shared structured evidence model, focused
  `access explain` CLI, and `explain_effective_access` MCP tool.
- Strict decoders reject missing permission/rule/member arrays and unknown DSM
  inheritance/preview values instead of treating them as empty access state.
- Unit tables cover direct/group allow, deny, read/write priority, custom,
  masked, administrators+ACL, homes default, Advanced Share Permissions,
  DSM preview/default policy, and missing preview behavior.
- Verified with `go test ./... -count=1`, `go vet ./...`, read-only CLI checks
  on DSM 7.3.2, and no mutation calls.
