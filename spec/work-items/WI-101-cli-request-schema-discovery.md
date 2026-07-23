---
id: WI-101
title: Expose CLI request schemas for offline agents
status: done
priority: P1
owner: ""
depends_on:
  - WI-100
parallel_group: E
touches:
  - internal/cli
  - docs
  - README.md
  - go.mod
  - spec/roadmap.md
---

# WI-101 - Expose CLI request schemas for offline agents

## Outcome

An unfamiliar agent that can execute only the `dsmctl` binary can discover
every file-backed mutation request shape and the command workflow without
opening repository documentation or guessing JSON fields.

## Scope

- Add an offline CLI command that lists request-bearing commands and emits
  their JSON Schema from the same typed domain models used at execution time.
- Link each applicable plan command's rendered help directly to its schema.
- Make command-group and leaf help self-routing when a custom long description
  is absent.
- Document the binary-only discovery workflow and protect full coverage with
  command-tree tests.

## Non-goals

- Change request types, plan/apply behavior, DSM compatibility routing, or MCP
  tool schemas.
- Replace semantic plan validation with JSON Schema validation.
- Define schema compatibility or release policy; WI-063 owns that policy.
- Run live DSM requests or mutations.

## Design constraints

- Schema generation is an offline CLI adapter concern over existing typed
  domain models; it must not import or shell out to the MCP adapter.
- Generated schemas must reject unknown JSON properties, matching
  `decodeJSONInput` behavior.
- Secrets remain credential references only and must never appear as example
  values.
- Coverage must fail when a new file-backed request command lacks a registered
  schema.

## Acceptance criteria

- [x] `dsmctl schema list` identifies every CLI command that consumes a typed
  request JSON file.
- [x] `dsmctl schema show <command path...>` emits a descriptive JSON Schema
  offline and reports unknown paths clearly.
- [x] Each registered plan command's `--help` gives the exact schema-list/show
  discovery commands and no longer points only to external docs.
- [x] Every CLI command has useful long-form routing guidance, whether custom
  or generated, and command-tree tests enforce schema/help coverage.
- [x] README and the agent quickstart document the binary-only workflow.
- [x] `go test ./...`, `go vet ./...`, and rendered CLI help/schema inspection
  pass.

## Verification

- Run focused CLI tests, `go test ./...`, `go vet ./...`, and
  `git diff --check`.
- Build and inspect root help, representative plan help, schema list, and
  representative simple and nested schemas.
- No live NAS request or mutation is authorized or required.

Verified 2026-07-23:

- `dsmctl schema list` exposed all 44 file-backed typed request commands; the
  JSON form was decoded in a focused test.
- Rendered and reviewed `dsmctl --help`, `dsmctl account plan --help`,
  `dsmctl schema list`, and the account and storage request schemas.
- `go test ./internal/cli`, `go test ./...`, `go vet ./...`, and
  `git diff --check` passed.
- No NAS connection or DSM request was made.

## Coordination

WI-063 may later add golden-file stability policy over this surface. WI-101
adds discoverability only and does not claim a compatibility guarantee.
README and `docs/` also contain an in-flight public landing-page, refreshed
Gateway screenshot, and distribution-plan update. Preserve that product copy
and limit WI-101's documentation edits to the binary-only schema workflow.

## Handoff
