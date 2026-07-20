---
id: WI-061
title: Core structured logging with a redaction guarantee
status: done
priority: P1
owner: ""
depends_on: []
parallel_group: E
touches:
  - internal/synology/client.go
  - internal/runtime/manager.go
  - internal/cli/root.go
  - internal/mcpserver/server.go
  - internal/remotepolicy/context.go
  - internal/observability
  - docs
---

# WI-061 — Core structured logging with a redaction guarantee

## Outcome

An operator can turn on leveled, structured diagnostic logging for the `dsmctl`
CLI and the stdio MCP server and see, per DSM operation, a correlation id, the
selected API/method/version/backend, the HTTP status, and the round-trip
duration — with a hard guarantee that passwords, OTPs, encryption keys,
recovery material, SIDs, and SynoTokens never appear in any log record. Logging
is silent by default, so current CLI and MCP output is unchanged unless the
operator opts in.

Today structured `slog` logging, per-request correlation ids, and redacted
audit exist only inside the gateway (WI-016). The core request path
(`internal/synology/client.go`), the runtime manager, and the CLI/MCP-stdio
front ends have no leveled logging, no per-request timing, and no opt-in
verbosity. This item closes that gap for the core without re-implementing the
gateway's remote audit store.

## Scope

- A small `internal/observability` package that constructs a leveled
  `*slog.Logger` writing to `stderr`, plus a redacting `slog.Handler` wrapper
  and the shared secret-key denylist.
- A logger seam on the DSM facade: add a `Logger *slog.Logger` field to
  `synology.Options` (nil = disabled). The request path
  (`requestLocked`/`requestScriptLocked`/`executeLocked`) emits one `debug`
  record per DSM call with `correlation_id`, `api`, `method`, `version`,
  `path`, `http_status`, and `duration_ms`.
- Per-operation correlation ids propagated through `context.Context`, reusing
  the existing `remotepolicy` correlation-id helpers (`WithCorrelationID` /
  `CorrelationID`) rather than introducing a second mechanism. When the CLI or
  stdio MCP invokes an operation without an inbound id, one is generated at the
  entry point.
- Opt-in verbosity: a persistent CLI flag (`--log-level`, values
  `error|warn|info|debug`, default effectively off/`error` with no records at
  normal use) and a `DSMCTL_LOG_LEVEL` environment variable; the flag wins over
  the env var. The stdio MCP server honors the same env var.
- Redaction applied at both the structured-attribute boundary (denylisted
  attribute keys are replaced with a `"[redacted]"` placeholder) and to any
  request URL embedded in error strings (continue using `url.URL.Redacted`).
- Optional lightweight timing: a `debug` summary record per CLI command
  reporting the number of DSM round-trips and total elapsed time.
- Documentation of the flag, env var, log format, and the redaction guarantee.

## Non-goals

- The structured DSM error taxonomy, CI test matrix, packaging, and release
  policy — those are the other WI-010 siblings and are out of scope here.
- OpenTelemetry, distributed tracing export, Prometheus/metrics endpoints, or
  any network log/metric sink. Logging is local stderr only.
- Changing or extending the gateway's WI-016 audit store, its HTTP access log,
  or its retention semantics.
- Logging request or response bodies, decoded state payloads, or plan contents
  beyond the fixed non-secret attribute set above.

## Design constraints

- Dependency direction: the facade may depend only on the standard library
  (`log/slog`) and the platform-neutral `remotepolicy` correlation helper. It
  must not import Cobra, the MCP server, config files, or prompt code. The
  logger is injected through `synology.Options`; the correlation id travels in
  `context.Context`.
- Secrets and identity (architecture-contracts): passwords, OTPs, encryption
  keys, recovery material, SIDs, and SynoTokens must not enter logs. The
  denylist must at minimum cover the request parameter and attribute keys
  `passwd`, `password`, `otp_code`, `_sid`, `SynoToken`, `device_id`, and any
  key/passphrase parameter, plus the `id` session cookie and `X-SYNO-TOKEN`
  header. New secret-bearing parameters added elsewhere must be redactable by
  extending this single denylist.
- MCP stdio purity: the stdio MCP server multiplexes JSON-RPC on stdout. All log
  output must go to stderr; enabling `debug` must never write a non-JSON-RPC
  byte to stdout.
- Default-silent: with no flag and no env var set, no log records are emitted
  and existing human/JSON command output is byte-for-byte unchanged.
- Reuse, do not duplicate: correlation ids use
  `remotepolicy.WithCorrelationID`/`CorrelationID`; the gateway's existing
  `correlateAndLog` HTTP access log is left as-is.

## Acceptance criteria

Shipped 2026-07-20 (on main):

- [x] `internal/observability` builds a leveled `*slog.Logger` over an
      injectable `io.Writer` (stderr in production) with a redacting
      `ReplaceAttr` hook and an authoritative secret-key denylist.
- [x] `synology.Options.Logger` seam; a nil logger is silent with a guarded
      early return on the hot path (no record built).
- [x] `--log-level debug` emits one record per DSM call with `correlation_id`
      (when set), `api`, `method`, `version`, `path`, `http_status`,
      `duration_ms` — live-verified against the lab (4 records, one shared id).
- [x] `--log-level` wins over `DSMCTL_LOG_LEVEL`; both parse; unit-tested.
- [x] Redaction unit test over passwd/otp_code/_sid/SynoToken/device_id/key
      asserts no secret value survives and each denylisted key renders
      `[redacted]`; the request path additionally logs metadata only (never a
      parameter value), pinned by a client test.
- [x] stdio-MCP stdout purity: live smoke with `DSMCTL_LOG_LEVEL=debug` shows
      0 non-JSON-RPC bytes on stdout and the diagnostic records on stderr.
- [x] Correlation ids generated at the CLI entry (root `PersistentPreRunE`),
      reusing `remotepolicy` helpers; one id shared across a command's DSM
      calls; unit-tested for generation + idempotence.
- [x] Default silent: nil logger (no flag/env) leaves output unchanged.
- [x] `docs/logging.md` documents the flag, env var, record schema, and the
      never-logged secret list.

Deferred (small follow-on): the **stdio MCP server does not yet stamp a
per-tool-call correlation id** — its records emit with the id omitted. Adding
one needs the same central tool-handler wrapper as WI-060's MCP category
field (a single interceptor over ~146 tools), so the two are best done
together. The optional per-command timing-summary record was not added.

## Verification

- `go test ./...` and `go vet ./...`.
- New unit tests: redaction denylist, flag-over-env precedence, per-request
  record shape and timing, forced-error redaction, correlation-id reuse, and
  stdio-MCP stdout purity. All are fixture/in-memory; no live NAS is required.
- No live-mutation policy applies — this item performs no DSM mutations. Manual
  smoke check: run any read command with `--log-level debug` against the lab
  NAS and confirm stderr records carry no secret values.

## Coordination

- `internal/synology/client.go` — request path is where per-call records and
  URL redaction are added; coordinate with any sibling touching the client.
- `internal/mcpserver/server.go` and `internal/application/service.go` are
  high-contention (agent-workflow); the stderr-only logging wiring must be
  coordinated before editing.
- `internal/gateway/server.go` (`correlateAndLog`) and
  `internal/remotepolicy/context.go` — reuse the correlation-id helper; do not
  fork it. WI-016 owns the gateway audit store and must remain unchanged.
- Sibling WI-010 splits (structured errors, CI matrix, packaging, release
  policy) may touch overlapping files; sequence edits to `cli/root.go` and
  `mcpserver/server.go`.

## Handoff

Fill this only when pausing incomplete work.
