---
id: WI-092
title: Connect Gateway NAS profiles with stored passwords
status: in_progress
priority: P1
owner: "codex"
depends_on: [WI-032]
parallel_group: G
touches:
  - internal/gateway/state/vault.go
  - internal/runtime/manager.go
  - internal/gateway/admin/handler.go
  - internal/gateway/admin/ui.go
  - docs/gateway-admin-guide.md
---

# WI-092 - Connect Gateway NAS profiles with stored passwords

## Outcome

A NAS password accepted and stored by the Gateway immediately establishes a
persisted DSM session. The NAS list owns every connection action: it offers
**Connect with saved password** when one exists, and otherwise accepts a
password/OTP with an explicit remember-password choice. The Passwords page is
only a vault-data manager, while Web Login remains an independent option.

## Scope

- Preserve the SID/SynoToken returned by successful password/OTP enrollment so
  the profile is connected as soon as its primary password is saved.
- Add an authenticated Admin API action that deliberately creates a fresh DSM
  session from the profile's primary vault password without returning the
  password or accepting it in the request.
- Add the saved-password action only to the NAS profile row. The password book
  remains data-only (add/change/reveal/remove) and never initiates a connection.
- Let the NAS row accept a fresh password/OTP when no password is stored (or an
  operator wants to replace it), with an explicit store choice. Store=false
  retains only the authenticated DSM session, not the password or newly issued
  trusted-device credential.
- Keep Web Login available in the same action menu and distinguish `session
  active`, `saved password ready`, and `not signed in` states.
- Reuse automatic TLS trust challenges, current origin/session/audit controls,
  and runtime trusted-device handling.

## Non-goals

- Sending a stored password to the browser or MCP client.
- Choosing a secondary password-book account as the runtime identity.
- Suppressing DSM OTP when the saved trusted-device registration is absent or
  rejected; the operator must re-enter password/OTP in that case.
- Removing Web Login, changing desktop CLI credential behavior, or changing
  WI-084's human-gated password reveal rules.

## Design constraints

- The runtime/session manager owns DSM client creation and session persistence;
  the Admin handler remains a thin authenticated adapter.
- Explicit saved-password connect bypasses a currently stored Web Login session
  only for the new login attempt, then replaces the locally persisted session
  after DSM accepts the password. The old DSM session is not exposed and may
  expire server-side normally.
- Passwords, OTPs, SIDs, and SynoTokens stay out of responses, UI models, audit
  details, and logs.
- This item consumes the already implemented password-book/runtime portions of
  WI-084 but does not depend on its remaining CLI live verification.

## Acceptance criteria

- [x] Successful primary password enrollment returns `session_stored: true`
      and the profile immediately renders as connected without Web Login.
- [x] A profile with a primary stored password exposes **Connect with saved
      password** on the NAS page. Without one, the NAS row accepts password/OTP
      and an explicit remember-password choice; the Passwords page has no
      connection action.
- [x] The explicit action resolves only the server-side primary vault password,
      logs in through the runtime manager, persists the new session, and returns
      only redacted profile/session metadata.
- [x] Web Login remains available independently, and the password book stays a
      data-only surface with no misleading connect action.
- [x] Missing password, rejected password/OTP, TLS failure, and session-store
      failure fail closed with no secret in HTTP/log/audit output.
- [x] Focused runtime/Admin/UI tests, `go test ./... -count=1`, `go vet ./...`,
      and `git diff --check` pass.
- [ ] The rebuilt SPK passes offline validation and a live read-only DSM auth
      check when a suitable test profile is available.

## Verification

- `go test ./internal/runtime ./internal/gateway/admin -count=1`
- `go test ./... -count=1 && go vet ./... && git diff --check`
- Live behavior is limited to DSM authentication and a read-only system-info
  probe. It performs no DSM configuration mutation.

## Coordination

Overlaps the in-progress WI-084 files `internal/gateway/admin/handler.go` and
`ui.go`. Preserve its password book, reveal/re-authentication, export, and
multi-account behavior. WI-091 shares the same SPK release and Admin UI; finish
this item before rebuilding/installing that pending `7.3.2-5` package.

## Handoff

- Last known good state: all connection actions now live on the NAS page. A
  profile with a stored primary password offers **Connect with saved password**;
  otherwise the NAS page accepts password/OTP and an explicit remember-password
  choice. Store=false persists only the encrypted DSM session, while the
  Passwords page remains data-only. Safe saved-password failures distinguish an
  OTP challenge from a rejected credential and reopen the NAS login form without
  exposing a secret. Full Go tests, vet, embedded-JavaScript syntax validation,
  and `git diff --check` pass.
- Reproducible build: both `dist/wi092-6a` and `dist/wi092-6b` produced identical
  `dsmctl-gateway-7.3.2-6-x86_64.spk` files. Offline SPK validation and every
  `SHA256SUMS` entry pass. SPK SHA-256 is
  `d6d19ab05cb5adbe028255191aa633adfe3764757097f36b9fcdc556e0e2daba`;
  image ID is
  `sha256:c9a92ecbd7669f848c270bbd4be59de694093b8c33c8ef5b114b2dde2c7bce87`.
- Blocker: the verified SPK is staged on the NAS at
  `/tmp/dsmctl-gateway-7.3.2-6-x86_64.spk`, but SSH `lab-admin` has no passwordless
  sudo and browser automation cannot populate the native file-upload control.
  Package Center's manual-install dialog is open and waiting for the operator to
  select the local SPK. The installed package remains `7.3.2-5` and is healthy.
- Temporary resources: the two local deterministic build directories above and
  the staged NAS SPK remain. No live DSM configuration mutation was performed.
