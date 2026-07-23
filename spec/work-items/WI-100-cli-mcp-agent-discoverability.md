---
id: WI-100
title: Make CLI and MCP operation discovery agent-ready
status: done
priority: P1
owner: ""
depends_on: []
parallel_group: E
touches:
  - cmd/dsmctl
  - internal/mcpserver
  - docs
  - README.md
---

# WI-100 - Make CLI and MCP operation discovery agent-ready

## Outcome

An AI agent with no prior dsmctl context can discover how to select a NAS,
distinguish reads from guarded mutations, and invoke the CLI or MCP surface
without guessing hidden prerequisites or bypassing the shared application
boundary.

## Scope

- Audit top-level and representative nested CLI help as a first-time user.
- Audit MCP server instructions and tool descriptions as a first-time agent.
- Add concise workflow guidance, prerequisites, and examples where the current
  surface is ambiguous.
- Add focused regression coverage for help and description contracts that are
  important to safe operation discovery.

## Non-goals

- Rename stable commands, flags, MCP tools, or schemas.
- Change application behavior, DSM compatibility routing, or mutation risk
  policy.
- Exercise live DSM mutations.
- Define the broader schema-stability policy tracked by WI-063.

## Design constraints

- CLI and MCP remain thin adapters over the shared application layer.
- Help must describe, not weaken, hash-bound plan/apply and profile-selection
  requirements.
- No generic raw WebAPI mutation escape hatch may be documented or exposed.
- Guidance must remain concise enough for interactive CLI users and MCP tool
  discovery payloads.

## Acceptance criteria

- [x] Top-level CLI help gives an unfamiliar agent a reliable starting
  workflow and points to deeper command help.
- [x] Representative read and mutation help states the relevant profile and
  plan/apply prerequisites without implying unsafe shortcuts.
- [x] MCP server instructions and tool descriptions explain target selection
  and guarded mutation sequencing where required.
- [x] Focused tests protect the added discovery and safety guidance.
- [x] `go test ./...` and `go vet ./...` pass.

## Verification

- Inspect rendered `dsmctl --help` and representative nested help.
- Inspect the MCP tool list and server instructions through tests or a local
  stdio client fixture.
- Run focused CLI/MCP tests, `go test ./...`, and `go vet ./...`.
- No live NAS mutation is authorized or required.

Verified 2026-07-23:

- Rendered and reviewed `dsmctl --help`, `dsmctl account plan --help`,
  `dsmctl storage apply --help`, and `dsmctl-mcp --help`.
- `go test ./internal/cli ./internal/mcpserver ./cmd/dsmctl-mcp` passed.
- `go test ./...` passed.
- `go vet ./...` passed.
- No live DSM request or mutation was run.

## Coordination

WI-060 and WI-063 may touch adjacent MCP error/schema text. This item avoids
their behavior and policy scope and limits changes to discoverability copy and
focused tests.

## Handoff
