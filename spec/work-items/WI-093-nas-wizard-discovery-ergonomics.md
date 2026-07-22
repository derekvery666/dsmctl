---
id: WI-093
title: Improve Add NAS discovery ergonomics
status: done
priority: P1
owner: ""
depends_on: [WI-023, WI-047]
parallel_group: G
touches:
  - internal/gateway/admin/ui.go
  - internal/gateway/admin/handler_test.go
  - docs/gateway-admin-guide.md
  - spec/roadmap.md
---

# WI-093 — Improve Add NAS discovery ergonomics

## Outcome

The Add NAS wizard keeps direct address entry immediately available and treats
LAN discovery as an optional, user-triggered aid. Large discovery result sets
can be narrowed instantly by NAS name, model, IP address, OS version, or other
non-secret device metadata already returned by discovery.

## Scope

- Place manual IP, hostname, or DSM URL entry before LAN discovery in step one.
- Do not start a LAN broadcast scan merely because the wizard opened.
- Keep the discovery result area collapsed until the user deliberately starts
  a scan, then show progress and results in place.
- Add a localized, accessible client-side search field for discovery results.
- Match case-insensitively across hostname, model, every IPv4 address, OS
  version, serial, MAC address, and reported state without issuing a new scan.
- Preserve disabled states and explanations for already-added or not-ready
  devices after filtering.

## Non-goals

- No change to the findhost wire protocol, broadcast targets, timeout, API
  response, profile persistence, TLS enrollment, or DSM sign-in behavior.
- No server-side search endpoint and no discovery of non-Synology devices.
- No live DSM mutation.

## Design constraints

- Preserve the embedded/offline Admin UI, CSP, five locales, responsive layout,
  and existing authenticated discovery endpoint.
- Filtering is presentation-only and must not alter the last discovery result
  set or send additional LAN traffic.
- An empty filter result is distinct from a scan that found no devices.
- The manual address flow remains keyboard accessible without scrolling through
  discovery results.

## Acceptance criteria

- [x] Opening Add NAS shows manual address entry first and sends no discovery
      request until the user selects Search local network.
- [x] LAN discovery expands on demand, reports progress, and keeps Search again
      available after completion.
- [x] The result search filters immediately across name, model, IP, OS version,
      serial, MAC, and state, with a clear localized no-match message.
- [x] Selecting a filtered device and manual address entry preserve the existing
      connection wizard behavior.
- [x] Desktop and narrow layouts remain usable and all five locales have the new
      copy.
- [x] Focused tests, `go test ./...`, `go vet ./...`, JavaScript syntax checking,
      browser walkthrough, and `git diff --check` pass.

## Verification

- `go test ./internal/gateway/admin -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- Extract the embedded script and run `node --check`.
- Browser-walk opening, manual entry, first scan, search/no-match, clearing the
  filter, and selecting a result. The scan is read-only; do not create or edit a
  NAS profile during verification.
- `git diff --check`

## Coordination

`internal/gateway/admin/ui.go` and `handler_test.go` overlap the in-progress
WI-091/WI-092 working tree owned by the same `codex` owner. Preserve those
changes and keep this item limited to wizard discovery presentation and tests.

## Handoff

Completed 2026-07-22. Manual address entry is first and focused; opening the
wizard leaves LAN discovery collapsed and makes no automatic request. The
explicit scan keeps its 3-second authenticated endpoint and adds a localized
client-side filter over hostname, model, address, OS/version, serial, MAC, and
state while preserving already-added/not-ready disabled rows. Results scroll
inside a bounded card and the existing narrow breakpoint stacks manual and
device rows.

Verification: focused Admin tests, `go test ./... -count=1`, `go vet ./...`,
embedded JavaScript parsing, and `git diff --check` pass. Live DSM walkthrough
on `192.0.2.235` with package `7.3.2-8` found 168 devices only after the scan
button was pressed; model `DS3018xs` narrowed to 8, IP `192.0.2.51` to 1, a
no-match query showed the localized empty result, clearing restored all 168,
and both a manual address and filtered `nas255` advanced to connection setup.
The wizard was cancelled and the Profile count remained 1.
