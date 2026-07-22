---
id: WI-091
title: Add DSM-delegated Gateway administrator login
status: in_progress
priority: P0
owner: "codex"
depends_on: [WI-032]
parallel_group: G
touches:
  - cmd/dsmctl-gateway
  - cmd/dsmctl-synology-auth
  - internal/gateway/admin
  - internal/gateway/oauth
  - internal/gateway/platformauth
  - internal/gateway/state
  - internal/synologyauth
  - deploy/synology
  - docs/gateway.md
  - docs/synology-package.md
  - spec/architecture-contracts.md
  - spec/gateway-deployment.md
---

# WI-091 - Add DSM-delegated Gateway administrator login

## Outcome

A Synology SPK installation lets a currently authenticated DSM administrator
open the Gateway through DSM Web Login without first creating a separate
Gateway account. The administrator may later configure one independent local
Gateway username/password as an explicit fallback. A generic container has no
DSM trust source and keeps the existing mandatory one-hour local-administrator
setup before any administration is possible.

## Scope

- Add an optional, platform-neutral signed-administrator assertion interface
  to the Gateway and enable it only when an explicit assertion key is supplied.
- Add a Synology host-side loopback bridge which validates the request's DSM
  cookie with DSM's `authenticate.cgi`, verifies membership in the DSM
  `administrators` group, strips caller-supplied identity headers, and signs a
  short-lived audience-bound assertion for the loopback-only Gateway backend.
- Require a fresh valid DSM assertion matching the DSM-backed Gateway session
  on every authenticated admin request, so DSM logout, session expiry, or
  administrator-group removal fails closed immediately.
- On a fresh SPK, disable the unauthenticated one-hour local setup endpoint and
  present DSM Web Login as the only entry method. After DSM login, allow an
  administrator to configure the single local fallback account.
- On an upgraded SPK that already has a local administrator, preserve it and
  offer both DSM Web Login and local username/password login.
- On generic Linux, preserve the existing setup window, local login, cookie,
  rate-limit, origin, and readiness behavior with no DSM-specific dependency.
- Identify audit actors and session metadata as `dsm:<subject>` or
  `local:<username>` without storing DSM cookies, SIDs, or assertion values.
- Permit DSM Web Login on the MCP OAuth authorization page; local
  username/password authorization is shown only after a local account exists.
- Generate and preserve a distinct 32-byte DSM assertion key in package-private
  storage, mount it read-only into the container, and include it in upgrade
  recovery copies without mixing it with the vault master key.

## Non-goals

- Authorizing ordinary DSM users. Only effective members of the DSM
  `administrators` group are Gateway administrators.
- Sending a DSM password, OTP, cookie, SID, SynoToken, or group list to the
  container or persisting it in Gateway state.
- Treating forwarded username headers, source IP, the desktop shortcut, or the
  host NAS profile as proof of identity.
- Adding DSM-specific paths, commands, or package variables to the core
  container image.
- Multiple local fallback accounts, per-DSM-user Gateway roles, OIDC, SAML, or
  Internet-facing multi-tenant administration.

## Design constraints

- This item is an explicit approved exception to WI-032's identical-admin-mode
  contract. The core remains portable: it verifies an abstract signed
  assertion; only the SPK bridge executes DSM commands and interprets DSM
  group membership.
- The public DSM reverse proxy targets only the loopback bridge, while the
  Gateway backend remains loopback-only. The bridge removes every incoming
  assertion header before optionally adding its own.
- Assertions use a random ID, an audience, issued/expiry times, an
  administrator claim, HMAC-SHA-256, a maximum one-minute lifetime, and
  bounded replay detection. Unknown claims/providers fail closed.
- DSM-backed Gateway browser sessions remain digest-only HttpOnly/SameSite
  cookies, but each use additionally requires a current matching DSM
  assertion. Local sessions never become valid merely because DSM SSO is
  unavailable.
- The SPK's local fallback account is optional. Generic Linux readiness still
  requires it; SPK readiness may instead be satisfied by a configured DSM
  assertion verifier.
- Password reveal/export keeps its independent human-gated reauthentication
  rules. A deployment with no local fallback password must not silently treat
  an ambient DSM session as password reauthentication.

## Acceptance criteria

- [x] Fresh generic container exposes only the existing local setup flow and
      cannot accept DSM assertions or use DSM login.
- [ ] Fresh SPK exposes only DSM Web Login; unauthenticated local setup and
      local password login are unavailable until a signed-in DSM administrator
      explicitly configures the fallback account.
- [x] DSM administrators can enter the Admin UI and authorize an MCP OAuth
      client without sending DSM credentials to the container.
- [x] Non-administrators, missing/expired DSM sessions, forged/replayed/wrong-
      audience assertions, non-loopback bridge callers, and identity mismatch
      all fail closed.
- [x] A DSM-backed Gateway session stops working when its current DSM assertion
      is absent or names a different subject.
- [ ] After local fallback setup, DSM and local login both work and produce
      distinguishable audit actors; logout and session revocation work for both.
- [ ] Existing SPK local administrators survive upgrade and gain DSM login;
      generic Linux behavior and existing local sessions remain compatible.
- [x] The assertion key is exact-length, private, preserved across upgrade,
      absent from ordinary database backups/logs/responses, and deletion follows
      the existing explicit package-data deletion choice.
- [x] Focused state/admin/OAuth/bridge tests, `go test ./... -count=1`,
      `go vet ./...`, deterministic image/SPK builds, and offline validation pass.
- [ ] Live DSM verification covers admin/non-admin/session-expiry behavior,
      fresh-or-reset login state, local fallback enablement, HTTPS portal login,
      package restart, and upgrade without performing DSM configuration
      mutations.

## Verification

- Unit tests inject time, assertion keys, DSM validators, and session records;
  request-capture tests prove cookie/assertion/header redaction and replay denial.
- Follow Synology's documented package-CGI authentication contract:
  <https://help.synology.com/developer-guide/integrate_dsm/web_authentication.html>.
- `go test ./... -count=1`, `go vet ./...`, deterministic image/SPK build and
  `deploy/synology/validate-spk.sh`.
- DSM live tests are authentication/read-only lifecycle tests. They do not
  authorize storage, network, firewall, account, or other DSM mutations.

## Coordination

WI-017 is still in progress only for broader distribution certification; its
implemented SPK assets are an integration surface, not a prerequisite, so it
is deliberately coordination rather than `depends_on`. This item overlaps
WI-017 in Synology packaging and WI-084 in the Admin UI/state files.
Preserve WI-017's current package, host-network, icon, and release changes.
Do not change WI-084's NAS credential-store/reveal semantics; the local
fallback availability is only surfaced so those independent human gates can
continue to fail closed.

## Handoff

- Last known good state: the dual-mode Gateway, DSM host bridge, optional local
  fallback, DSM OAuth path, and WI-092 saved-password connection fix are
  implemented as release `7.3.2-5`. Full Go tests, `go vet`, `git diff --check`,
  offline SPK validation, and two fixed-input SPK builds pass. Both current
  SPKs have SHA-256
  `080de8be8dffe6073d5b24c49e429a7f37137615a2b54c696c6dec50ace81a9a`;
  the image ID is
  `sha256:ade35a29ead22498fa03605b97593b6b9c403b271c97263eebac352b00c9f98c`.
- Blocker: `ssh nas` authenticates as DSM administrator `lab-admin`, but `sudo`
  still requires a password and root public-key SSH is unavailable. The
  in-app DSM `:5001` session has expired, so Package Center upload is paused at
  the visible DSM login page. No password was requested, captured, or passed
  through a command. Resume after the user signs in to that page.
- Temporary resources: the current reproducible artifacts remain under
  `dist/wi092-a` and `dist/wi092-b`, and the verified SPK is staged at
  `/tmp/dsmctl-gateway-7.3.2-5-x86_64.spk` on the NAS. The NAS is still on
  `7.3.2-4`; no package or persistent data was changed during this paused
  attempt.
