---
id: WI-102
title: Expose the complete CLI operation catalog
status: done
priority: P1
owner: ""
depends_on:
  - WI-101
parallel_group: E
touches:
  - internal/cli
  - docs
  - README.md
  - spec/roadmap.md
---

# WI-102 - Expose the complete CLI operation catalog

## Outcome

An unfamiliar binary-only agent can discover every CLI group and executable
operation, not only file-backed mutation requests, with exact usage, flags,
defaults, required inputs, workflow role, subcommands, structured-output
support, and request-schema linkage.

## Scope

- Add an offline, structured catalog over the complete Cobra command tree with
  focused list/filter and exact show operations.
- Describe commands conservatively as group, read-only planning, mutating
  apply, or direct operation; do not mislabel an arbitrary direct operation as
  read-only.
- Include canonical path, summary, long description, usage, aliases,
  subcommands, local and inherited flags, required/default metadata, JSON
  output availability, and the exact request-schema command when applicable.
- Add an exact catalog lookup to every rendered project command help page.
- Verify representative account/user, SMB, and Drive operations as well as
  exhaustive command-tree coverage.

## Non-goals

- Change an operation's behavior, arguments, output schema, compatibility
  routing, or risk policy.
- Claim that every direct operation is read-only; the catalog is conservative.
- Duplicate field-level request schemas in prose or hand-maintain a second
  command registry.
- Run live DSM requests or mutations.

## Design constraints

- Discovery is derived from the live Cobra tree and existing typed request
  registry so it cannot silently drift from executable commands.
- The catalog operates without configuration, credentials, MCP, or NAS access.
- Generated help complements module-specific prose and flag descriptions; it
  must not overwrite bespoke safety text.
- Built-in Cobra help/completion topics and hidden commands are not DSM
  operations and are excluded from the catalog.

## Acceptance criteria

- [x] `dsmctl commands list` and `--json` enumerate every visible project
  command, support prefix and runnable-only filters, and expose concise roles.
- [x] `dsmctl commands show <command path...>` emits complete human or JSON
  metadata and rejects unknown paths with a discovery hint.
- [x] Every project command's help links to its exact offline catalog lookup.
- [x] Account/user management, SMB state/settings, and Drive settings/admin
  commands expose correct usage, flags, workflow role, and request-schema link
  where applicable.
- [x] Exhaustive tests fail when a visible project command is missing from the
  catalog or lacks summary, long help, usage, or exact discovery guidance.
- [x] README and agent quickstart distinguish the full operation catalog from
  the 44 file-backed request schemas.
- [x] `go test ./...`, `go vet ./...`, rendered help/catalog inspection, and
  `git diff --check` pass.

## Verification

- Run focused CLI tests, `go test ./...`, `go vet ./...`, and
  `git diff --check`.
- Inspect root help, catalog list/JSON, and account inventory, File Services
  SMB state/plan, and Drive config/admin descriptions.
- No live NAS request or mutation is authorized or required.

Verified 2026-07-23:

- The live catalog exposed 357 visible project commands and 298 runnable
  operations; the separate request-schema index remained 44 file-backed plans.
- Rendered and reviewed the account inventory contract, File Services SMB/NFS
  plan contract, Drive runnable-operation index, and Drive config plan help.
- Exhaustive tests matched every visible command to exactly one catalog entry,
  checked every exact help link, and verified account, SMB, Drive, required
  approval, filtering, human output, JSON output, and unknown-path behavior.
- `go test ./internal/cli`, `go test ./...`, `go vet ./...`, and
  `git diff --check` passed.
- No NAS connection or DSM request was made.

## Coordination

WI-100/WI-101 add adjacent help and request-schema discovery. WI-102 builds on
them without changing MCP or operation semantics. Preserve the unrelated
in-flight public landing-page, Gateway screenshot, and distribution changes.

## Handoff
