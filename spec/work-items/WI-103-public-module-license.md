---
id: WI-103
title: Adopt the public Go module identity and Apache-2.0 license
status: done
priority: P1
owner: ""
depends_on:
  - WI-101
parallel_group: E
touches:
  - go.mod
  - "**/*.go"
  - README.md
  - docs
  - LICENSE
  - scripts/check-no-lab-data.sh
  - spec/roadmap.md
---

# WI-103 - Adopt the public Go module identity and Apache-2.0 license

## Outcome

The repository has one permanent public identity: the GitHub repository,
Go module, internal imports, installation examples, and release documentation
all use `github.com/derekvery666/dsmctl`, and downstream users receive the
project under Apache License 2.0.

## Scope

- Add the official Apache License 2.0 text as the root `LICENSE`.
- Migrate `go.mod` and all self-imports from the historical owner path to the
  permanent public GitHub path.
- Replace source-install and planned-release placeholders with the canonical
  repository owner.
- Keep the internal-lab leak scanner strict while allowing only the exact
  canonical public repository path that necessarily contains the owner's name.

## Non-goals

- Change CLI, MCP, gateway, DSM compatibility, or operation behavior.
- Publish a GitHub Release, tag, package, installer, or SPK.
- Add copyright or trademark claims beyond the chosen standard license text.
- Make live DSM requests or mutations.

## Design constraints

- The module path and all self-import paths must change atomically so every
  package continues to compile.
- Existing in-flight README, documentation, schema, and gateway changes must
  be preserved.
- The scanner exception must match the public repository path, not broadly
  exempt the owner name.

## Acceptance criteria

- [x] The root `LICENSE` is the official Apache License 2.0 text.
- [x] `go.mod`, Go self-imports, and project documentation use
  `github.com/derekvery666/dsmctl`; the historical module path is absent.
- [x] README source installation and release-plan URLs use the canonical
  public repository.
- [x] The no-lab-data scanner allows the exact public repository path while
  continuing to reject other owner-name and internal-IP occurrences.
- [x] `go test ./...`, `go vet ./...`, representative builds, and
  `git diff --check` pass.

## Verification

- Search the tracked tree for the historical module path and unresolved owner
  placeholders.
- Run the no-lab-data scan and a focused positive/negative filter check.
- Run `go mod tidy`, `go test ./...`, `go vet ./...`, CLI/MCP builds, and
  `git diff --check`.
- No live NAS request or mutation is authorized or required.

Verified 2026-07-23:

- Compared `LICENSE` byte-for-byte with Apache's official
  `LICENSE-2.0.txt`; it matched.
- Migrated 424 files containing the historical module prefix. Repository-wide
  searches found zero historical paths, `<repository-url>` values, or `OWNER`
  placeholders; `go list ./...` resolved all 120 packages under the new path.
- The real no-lab-data script passed. Focused filter checks proved the exact
  repository path is allowed while another owner-name or `10.17.x` occurrence
  on the same line is still rejected.
- `go mod tidy`, `go test ./... -count=1`, `go vet ./...`, native CLI/MCP/
  Gateway builds, `linux/amd64` cross-builds, and `git diff --check` passed.
  The first full test attempt had one message-free Windows compiler exit for
  the Download Station package; its focused test and the uncached full rerun
  both passed.
- No NAS connection or DSM request was made.

## Coordination

WI-101 is complete. WI-102 (the CLI operation catalog) appeared concurrently
after this migration was claimed and overlaps `internal/cli`, README, and docs;
the migration preserves that work and changes only its historical module-path
prefix. Preserve the already-delivered schema discovery and public landing-page
content. This item records the owner decisions that resolve two blockers in
`docs/public-release-plan.md`; packaging and GitHub Release automation remain
follow-up work.

## Handoff

Complete. Packaging, installers, tagging, GitHub Release publication, and the
remaining release-policy decisions stay in the public distribution follow-up.
