---
id: WI-108
title: Confirm high-risk MCP applies in conversation
status: done
priority: P1
owner: ""
depends_on: [WI-016, WI-038, WI-048]
parallel_group: G
touches:
  - internal/remotepolicy
  - internal/gateway/state
  - internal/gateway
  - internal/gateway/admin
  - internal/gateway/oauth
  - internal/mcpserver
  - docs/gateway-admin-guide.md
  - spec/architecture-contracts.md
  - spec/gateway-deployment.md
  - spec/mcp-power-user-connection-design.md
  - spec/roadmap.md
---

# WI-108 - Confirm high-risk MCP applies in conversation

## Outcome

An owner using an interactive MCP client approves or rejects one high-risk
`apply_*` call inside the conversation instead of navigating to the Gateway
Admin approvals page. The confirmation names the NAS, exact plan summary, and
material consequences. It is bound to the authenticated token, current MCP
session, plan hash, and NAS profile revision, is usable only by the suspended
tool call, and never becomes a model-visible approval token.

The existing administrator-page approval remains available as an explicitly
selected hardened policy for unattended clients or operators who do not trust
their MCP host to present user elicitation faithfully.

## Product decision

The repository's previous policy treated every MCP host as untrusted and made
out-of-band administrator approval mandatory for every remote high-risk apply.
The owner has explicitly changed that product default for new connections:

- new OAuth and manual MCP credentials default to `interactive`;
- credentials created before this migration remain `administrator`, so an
  upgrade never silently weakens an existing trust boundary;
- `interactive` trusts the MCP host's declared form-elicitation capability and
  explicit user response for this one call;
- `administrator` retains the WI-016/WI-038 short-lived, single-use Admin UI
  approval with no conversational bypass.

## Scope

- Add a closed token approval policy: `interactive` or `administrator`.
- Persist the policy on MCP credentials. Migrate every existing credential to
  `administrator`; default newly issued manual and OAuth credentials to
  `interactive`.
- Run the managed MCP transport statefully so the server can issue
  `elicitation/create` during a tool call. Bind each MCP session to its
  authenticated token identity and keep bounded session lifetime/state.
- For an `interactive` token, a high-risk `apply_*` without an exact
  conversational grant returns to the remote MCP middleware before mutation.
  The middleware asks the client to show the exact plan confirmation. Only an
  explicit accept plus affirmative confirmation retries the same call with an
  in-process, exact-binding grant.
- Consume the conversational grant during the retried application admission.
  It exists only in the request context, cannot be supplied as a tool argument
  or header, and cannot authorize another plan, NAS, revision, token, session,
  or retry.
- Preserve Admin UI pending requests and standard approvals for
  `administrator` credentials and as visible audit/context for operators.
- Let the manual connection wizard select the policy, with interactive
  confirmation selected by default. OAuth copy explains its interactive
  default.
- Fail closed with actionable MCP output if the client does not advertise form
  elicitation, declines, cancels, or returns malformed confirmation content.

## Non-goals

- Treating a model message such as "approved" as authority.
- Adding an `approve_*` MCP tool, approval argument, caller-controlled header,
  reusable approval token, URL-mode elicitation, deep link, or automatic
  browser navigation.
- Silently converting existing administrator-approved credentials to
  interactive confirmation.
- Weakening plan hashing, current-state revalidation, stable-ID checks,
  postcondition verification, audit fail-closed behavior, or token/NAS scopes.
- Live high-risk mutation testing.

## Design constraints

- Interactive authority originates only from a server-to-client MCP
  `elicitation/create` request on the authenticated session. The private
  context grant is constructed after an affirmative response and checked again
  at the application/state boundary.
- The grant binding contains token ID, MCP session ID, NAS, profile revision,
  and plan hash. Comparison is exact; the grant is not persisted.
- A pre-existing exact administrator approval may still admit an interactive
  token and is consumed under the existing WI-016 transaction. An
  `administrator` token never accepts an interactive grant.
- The approval decision and apply admission are audited. Audit output contains
  no plan payload, credential, bearer value, session secret, or elicitation
  response body.
- Stateful Streamable HTTP sessions are bounded and bound to the token that
  initialized them. Requests carrying a session ID under another token fail
  before MCP dispatch.
- Clients without form elicitation fail closed and receive a concise
  explanation that the connection must use a compatible interactive client or
  an administrator-approval credential.
- Local CLI and stdio behavior is unchanged.

## Acceptance criteria

- [x] Existing stored MCP credentials migrate to `administrator`; new manual
      and OAuth credentials default to `interactive`; invalid policy values are
      rejected.
- [x] A high-risk apply under an interactive credential presents one
      conversation confirmation containing NAS, plan summary, risk, and a
      shortened plan identifier, then proceeds only after explicit affirmative
      input.
- [x] Decline, cancel, malformed content, missing form-elicitation capability,
      token/session mismatch, plan/revision/hash mismatch, and replay all fail
      before any mutation method.
- [x] The conversational grant is invisible on the wire and is consumed by the
      same application admission boundary used by every `apply_*`.
- [x] Administrator-policy credentials retain the existing exact, ten-minute,
      single-use Admin UI approval flow with no conversational bypass.
- [x] Stateful managed MCP sessions are token-bound, expire when idle, support
      normal initialize/list/call/close behavior, and retain all existing OAuth,
      rate-limit, host/origin, and scope enforcement.
- [x] The connection UI exposes the two policies with interactive selected by
      default and localized copy; token lists show the effective policy.
- [x] Audit records distinguish interactive confirmation from administrator
      approval without storing confirmation content or secrets.
- [x] Focused state, MCP, gateway transport, OAuth, and Admin UI tests pass;
      `go test ./... -count=1` and `go vet ./...` pass.

## Verification

- `go test ./internal/gateway ./internal/mcpserver ./internal/gateway/state -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- `git diff --check`
- Browser visual QA covered both approval-policy choices in the localized token
  wizard with no console errors.
- All execution tests used fakes and captured requests. No live DSM mutation
  was performed.

## Coordination

This item deliberately supersedes WI-038's "no approval through MCP" non-goal
only for server-initiated, session-bound user elicitation. It overlaps the
Gateway state and Admin UI files currently carrying WI-105 recovery work; edits
must stay narrow and preserve those uncommitted changes. It also changes the
managed transport used by all remote MCP work, so transport tests are required
before completion.
