---
id: WI-094
title: Refine NAS discovery dialog layout
status: done
priority: P1
owner: ""
depends_on: [WI-093]
parallel_group: G
touches:
  - internal/gateway/admin/ui.go
  - internal/gateway/admin/handler_test.go
  - docs/gateway-admin-guide.md
  - spec/roadmap.md
---

# WI-094 — Refine NAS discovery dialog layout

## Outcome

The Add NAS wizard has one intentional scroll surface while LAN discovery is
expanded, and each discovered NAS presents serial and MAC identity without
turning a large result set into a visually noisy wall of text.

## Scope

- Keep the wizard chrome, manual address entry, discovery heading/filter, and
  footer actions fixed within the viewport while only the discovery result
  list scrolls.
- Remove the nested dialog-plus-result scrollbar in the discovery state.
- Present hostname/model as the primary line, DSM version/state as secondary
  status, serial and MAC as compact tertiary identifiers, and IP addresses in
  a distinct monospace column.
- Deduplicate MAC values and summarize additional interfaces compactly.
- Preserve the existing single-column narrow layout and accessible button
  names.

## Non-goals

- No change to discovery traffic, result ordering, filter semantics, API
  response, NAS profile persistence, or sign-in behavior.
- No new DSM operation or live mutation.

## Design constraints

- The dialog itself must not scroll while discovery results are present; the
  result list is the one scroll owner for step one.
- Steps two and three remain usable on short viewports through a single
  step-local scroll surface.
- Serial and MAC are non-secret discovery metadata, but absent values must not
  leave empty labels or placeholder clutter.
- Preserve all five locales, the embedded/offline UI, and CSP.

## Acceptance criteria

- [x] Expanded discovery has exactly one vertical scroll owner, the result
      list, on a standard desktop viewport.
- [x] Manual entry, search/filter controls, Search again, and Cancel remain
      visible while scrolling results.
- [x] Result cards show serial and a deduplicated primary MAC when available,
      with a compact count for additional MAC addresses.
- [x] Hostname/model, status/version, identifiers, and IP retain a clear visual
      hierarchy on desktop and stack cleanly on narrow layouts.
- [x] Filtering and device selection still work without creating a Profile
      during verification.
- [x] Focused/full tests, `go vet ./...`, embedded JavaScript syntax checking,
      browser walkthrough, SPK validation, and `git diff --check` pass.

## Verification

- `go test ./internal/gateway/admin -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- Parse the embedded script with Node.
- Live browser walkthrough on the DSM package: expand discovery, inspect the
  scroll owners, filter by serial/MAC/IP/model, and select then cancel a result.
- `git diff --check`

## Coordination

This is a focused continuation of WI-093 and overlaps the same embedded Admin
UI currently carrying WI-091/WI-092 changes under the same `codex` owner.
Preserve all unrelated dirty-worktree changes.

## Handoff

Completed 2026-07-22. Discovery step one now gives vertical overflow ownership
to `#discoveredNAS`; the viewport-bounded dialog, wizard chrome, manual entry,
filter, and actions remain fixed. Result cards use a desktop identity/IP grid,
stack on narrow widths, and show serial plus a normalized, deduplicated primary
MAC with a compact `+N` additional-interface count.

Verification passed: focused and full Go tests, `go vet ./...`, embedded
JavaScript parsing, SPK structural validation, and `git diff --check`. The
`7.3.2-9` SPK was installed and repaired on the DSM lab NAS; core and bridge
health endpoints returned healthy and the portal returned HTTP 200. A live LAN
scan returned 168 devices. Filtering by serial `2040QWRGP8B40` returned the
single expected DS720+ card, showing its serial, primary MAC, `+1` secondary
MAC count, and both IP addresses. No Profile was created during verification.
