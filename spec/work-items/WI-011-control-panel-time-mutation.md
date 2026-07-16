---
id: WI-011
title: Define guarded Control Panel time changes
status: proposed
priority: P2
owner: ""
depends_on: [WI-006]
parallel_group: C
touches:
  - internal/domain/controlpanel
  - internal/synology/operations/controlpaneltime
  - internal/synology/controlpanel.go
  - internal/application
  - internal/cli
  - internal/mcpserver/server.go
---

# WI-011 — Define guarded Control Panel time changes

## Outcome

Time zone, display format, and NTP configuration can be changed through a typed
hash-bound plan/apply contract without exposing raw `SYNO.Core.Region.NTP.set`.

## Scope

- Separate intent fields for time zone, date/time display format, NTP mode,
  and ordered NTP servers.
- Current-state fingerprint, explicit risk summary, approval hash, and
  postcondition verification.
- Version-scoped `set` variants with strict request-capture tests.
- Fail closed when a time zone or NTP server cannot be validated.

## Safety requirements

- Re-read configuration immediately before apply and reject stale plans.
- Do not set wall-clock time in this item; switching to manual mode requires a
  separate decision and safety review.
- Treat removal of the last NTP server and loss of synchronization as high
  risk; never infer a replacement server.
- Do not claim NTP reachability from syntax validation alone.
- Verify the normalized configuration after DSM accepts the change and return
  an actionable partial-failure result when synchronization does not converge.

## Verification

- Fixture and request-capture tests, `go test ./...`, and `go vet ./...`.
- No live time/NTP mutation without separate explicit authorization.
