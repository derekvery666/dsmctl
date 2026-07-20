# DSM error taxonomy and CLI exit codes

Every failure that originates from a DSM API is classified into a **stable,
closed** set of categories, so scripts and MCP clients can branch on the failure
class instead of matching opaque message text. The category strings and the CLI
exit codes below are part of the CLI contract — they do not change without a new
work item, and adding a category is likewise a contract change.

## Categories

`synology.Classify(err)` unwraps an error (through `fmt.Errorf("%w", …)`
wrapping) and returns one of:

| Category | Meaning |
| --- | --- |
| `auth` | The session or credentials are the problem — expired/invalidated session, wrong password, or a required/failed second factor. |
| `permission` | Authenticated, but the account lacks privilege (or is blocked by an access rule). |
| `not-found` | The target API, method, or resource does not exist. |
| `conflict` | The request conflicts with current state (duplicate, or a resource busy/in use). |
| `rate-limit` | DSM is throttling the caller. |
| `transient` | A temporary failure worth retrying (timeout, a 5xx from the web server, a reset). |
| `unsupported` | The operation or API version is not supported on this DSM. |
| `invalid-input` | A parameter was missing, malformed, or rejected. |
| `unknown` | No confident classification (the fallback). |

DSM API error codes map to categories as follows (unmapped codes fall back to
`unknown`): `101/114/120` → invalid-input; `102/103` → not-found; `104` →
unsupported; `105/108/402/407` → permission; `106/107/119` → auth (session);
`400/401/403/404/406` → auth (login / two-step). A `SessionExpiredError` and an
`OTPRequiredError` both classify as `auth`.

DSM answers most failures with an application code inside a 2xx envelope, but a
call can also fail below that layer with no code at all. Those HTTP-level
failures are typed too: an HTTP `429` classifies as `rate-limit`; a `5xx` status
and any transport error (timeout, connection reset/refused) classify as
`transient`. A caller-driven context cancellation is deliberately left
`unknown` so it is never mistaken for a retryable condition. Other non-2xx
statuses (for example a bare `4xx`) stay `unknown`.

## CLI exit codes

`dsmctl` exits with a category-specific code so a script can react without
parsing stderr. The human-readable stderr line is prefixed with the category
when one is confidently classified (for example `Error (auth): …`).

| Exit code | Category |
| --- | --- |
| 0 | success |
| 1 | generic / unclassified (non-DSM) failure |
| 2 | invalid-input |
| 3 | auth |
| 4 | permission |
| 5 | not-found |
| 6 | conflict |
| 7 | rate-limit |
| 8 | transient |
| 9 | unsupported |

## MCP structured error category

Every MCP tool error result carries the same category as a machine-readable
field, so a model or client can branch without parsing the prose. A failed
`tools/call` returns `isError: true` with structured content shaped like:

```json
{ "category": "auth", "message": "the DSM session for NAS \"lab\" has ended; sign in again with 'dsmctl auth login --nas lab'" }
```

The category is derived from the handler's typed Go error via
`synology.Classify` — not by string-matching the message — through a single
receiving-middleware hook, so all tools gain the field uniformly with no
per-tool wiring. `SessionExpiredError` and OTP guidance still classify as `auth`.

## Read-only retry

A DSM call that a read-only call site marks retry-eligible and that fails with a
`transient` or `rate-limit` HTTP-level error is retried automatically with
bounded, jittered exponential backoff (a fixed attempt cap and a total time
budget), honoring context cancellation. Eligibility is a property of the call
site, never the HTTP verb: every DSM call is a POST, so a plan/apply or any other
mutation is issued exactly once and is never auto-retried.

## Secret hygiene

A rendered DSM error never contains a SID, SynoToken, password, or OTP: the
`APIError` message is API/method/code only; an `HTTPError` renders only the
redacted endpoint and HTTP status (never the request parameters or body); and
binary-transfer errors mask the `_sid` / `SynoToken` query parameters
(`url.URL.Redacted` masks only userinfo, so the transport redacts those
explicitly — see the FileStation transfer notes). The MCP category hook forwards
that already-redacted message and adds only the fixed category enum, so it
introduces no new secret surface.
