# Diagnostic logging

`dsmctl` and the stdio MCP server can emit opt-in, leveled diagnostic logs. It is
**silent by default** — normal CLI and MCP output is unchanged unless you turn it
on — and every record is redacted so a password, OTP, SID, SynoToken, device id,
or key can never appear in a log.

## Turning it on

```console
dsmctl --log-level debug office settings --nas office     # flag
DSMCTL_LOG_LEVEL=debug dsmctl office settings --nas office # env var
DSMCTL_LOG_LEVEL=debug dsmctl-mcp                          # stdio MCP server
```

Levels are `debug`, `info`, `warn`, and `error`. The `--log-level` flag wins over
the `DSMCTL_LOG_LEVEL` environment variable; an empty or unrecognized value leaves
logging off. All records are written to **stderr** — never stdout — so logging is
safe alongside the stdio MCP server's JSON-RPC on stdout and alongside a command's
`--json` output.

## Per-DSM-call records

At `debug`, each DSM API call emits one record with non-secret metadata:

```
level=DEBUG msg="dsm request" correlation_id=b42c7b44 api=SYNO.Office.Setting.System \
  method=get version=1 path=entry.cgi http_status=200 duration_ms=18
```

- `correlation_id` groups all DSM calls made by one CLI command (stamped at the
  CLI entry point). The stdio MCP server does not yet stamp a per-tool-call id, so
  its records omit the field — see
  [WI-061](../spec/work-items/WI-061-core-observability-logging-redaction.md).
- Only metadata is logged — never a request parameter value — so no secret can
  reach a record even before redaction applies.

## Redaction guarantee

The logger installs a redaction hook: any attribute whose key names
authentication material (`passwd`, `password`, `otp_code`/`otp`, `_sid`/`sid`,
`SynoToken`, `X-SYNO-TOKEN`, `device_id`, `key`/`private_key`, `passphrase`,
`recovery`) is written as `[redacted]` regardless of value type. New
secret-bearing keys must be added to the single denylist in
`internal/observability`. This complements the transfer-URL redaction (download,
upload, thumbnail) and the error-message hygiene documented in
[errors.md](errors.md).
