---
id: WI-097
title: Gateway MCP-token least-privilege defaults and lifetime cap
status: proposed
priority: P1
owner: ""
depends_on: []
parallel_group: G
touches:
  - internal/gateway/admin/ui.go
  - internal/gateway/oauth/handler.go
  - internal/gateway/state/policy.go
---

# WI-097 — Gateway MCP-token least-privilege defaults and lifetime cap

## Provenance

Design-review follow-up (2026-07-22 adversarial review, re-validated against
`cc8d160`). All three findings still hold on current main. Theme: the gateway
issues plan/apply-capable credentials whose *defaults* are maximum-privilege, so
an operator who accepts the defaults grants far more than the common case needs.

## Outcome

Creating an MCP credential defaults to the least privilege that is useful, and a
credential cannot be minted with an unbounded or perpetual lifetime, so
over-grant is a deliberate opt-in rather than the one-click default.

## Scope

- **Manual-token wizard defaults** (`internal/gateway/admin/ui.go`,
  `openAccessWizard()` ~line 692). Today it hardcodes `tokenExpiry = 365`, the
  `authorityPreset = full` (which `applyAuthorityPreset()` maps to all four
  scopes including `nas.apply`), and `renderTokenNASChoices()` renders every
  non-target NAS checkbox pre-checked. Flip these to least privilege: default the
  preset to `observer` (`nas.read` only), default the lifetime to a short window
  (e.g. 30 days), and render NAS checkboxes unchecked so each NAS is opted in
  deliberately. (The `authorityPreset` dropdown and target-role skip added by the
  recent rework stay.)
- **OAuth grant defaults** (`internal/gateway/oauth/handler.go`). `normalizeScopes`
  (~line 606) substitutes `defaultScopeString` (all four scopes, ~line 35) when a
  client sends no `scope`; change the scope-less default to `nas.read` only, so a
  client that needs plan/apply must request it explicitly.
  `validateAuthorizationRequest` (~lines 433–446) unconditionally builds the NAS
  allowlist from every profile (and, unlike the manual path, does **not** skip
  `role: target` profiles). Exclude target-role profiles, and add a mechanism to
  scope the granted NAS set per authorization (honour a requested NAS/resource
  parameter, or an admin selection at the consent step) instead of always
  granting all profiles.
- **Server-side lifetime cap** (`internal/gateway/state/policy.go`,
  `normalizeMCPTokenInput` ~line 736). Today any future `ExpiresAt` is accepted
  and a nil `ExpiresAt` means never-expires. Add a `MaxMCPTokenLifetime` and
  reject `ExpiresAt` beyond `now+cap`; either reject a nil `ExpiresAt` or auto-set
  it to `now+cap` unless an explicit "no expiry" flag is set, so a perpetual
  token is always deliberate and auditable. Mirror the existing approval-TTL cap
  pattern (`policy.go` ~lines 358–359).

## Non-goals

- Removing the ability to create a broad/long-lived token entirely — advanced
  operators may still opt in; only the *default* and the *unbounded* cases change.
- Scope/approval-model semantics (immutability, high-risk approval binding) —
  unchanged; see the model already documented in `docs/gateway.md`.
- Any transport, forwarded-header, or docs change (WI-096/WI-098/WI-099).

## Design constraints

- The server-side cap is authoritative: it must reject an over-long or perpetual
  lifetime regardless of the client/UI, so the UI default change is convenience
  and the policy cap is the guarantee.
- Preserve the credential-list states (never-used/used/expired/revoked) and the
  digest-only storage; this item changes defaults and bounds, not storage.
- Keep scopes/allowlist immutable after creation (changing authority still means
  issuing a new credential).

## Acceptance criteria

- [ ] With no fields changed, the manual-token wizard mints a `nas.read`-only,
      ≤30-day token with no NAS pre-selected; unit/UI assertions cover the
      defaults.
- [ ] A scope-less OAuth authorization yields `nas.read` only; requesting
      `nas.plan`/`nas.apply` still works; target-role profiles are excluded from
      the OAuth NAS allowlist.
- [ ] `normalizeMCPTokenInput` rejects an `ExpiresAt` beyond the cap and rejects
      (or explicitly gates) a perpetual token; a table test covers the boundary.
- [ ] Existing gateway/state and admin tests stay green; new behaviour is tested.

## Verification

- Create a token via the UI accepting all defaults; confirm scopes/lifetime/NAS.
- Drive an OAuth flow with and without a `scope` parameter; confirm granted
  scopes and that target NAS are excluded.
- `go test ./internal/gateway/... -count=1`.

## Coordination

- `internal/gateway/admin/ui.go` and `oauth/handler.go` are actively edited by
  the gateway stream (WI-045/WI-048/WI-091/WI-092/WI-095); rebase and re-verify
  line references before editing.
- Product decision touchpoint: the exact default lifetime and the cap value
  should align with the credential-model decisions ([[dsmctl-credential-model]],
  [[dsmctl-product-decisions]]).

## Handoff

Fill this only when pausing incomplete work.
