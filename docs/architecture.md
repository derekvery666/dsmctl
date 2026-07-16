# v0.1 Architecture

## Goal

The first release proves one complete vertical path shared by both products:

1. Select one of multiple named NAS profiles.
2. Resolve its credential without putting the password in the config file.
3. Discover the DSM API path and supported version.
4. Log in and retain an independent session for that NAS.
5. Read normalized system information.
6. Expose the same operation through CLI and MCP.

## Dependency direction

```text
CLI adapter ─┐
             ├── Application service ── Runtime/session manager ── Synology client ── DSM
MCP adapter ─┘                                 │
                                              ├── Config
                                              └── Credential resolver
```

Dependencies only point to the right. In particular:

- MCP does not invoke the CLI process.
- CLI and MCP do not construct raw DSM WebAPI calls.
- The Synology client has no knowledge of CLI commands or MCP tools.
- Password values never enter configuration models or display models.

## Session model

`runtime.Manager` owns a map keyed by NAS profile name. Each entry is a separate `synology.Client`, containing its own SID, SynoToken, discovered API versions and HTTP transport. Clients are created lazily and reused until the CLI command or MCP process exits. DSM session error codes 106 and 119 cause one automatic re-login and retry.

Calls to one NAS are serialized inside that NAS client to protect session state. Calls to different NAS profiles use different clients and can proceed independently.

## Public surfaces in v0.1

CLI:

```text
dsmctl nas add <name>
dsmctl nas list
dsmctl nas use <name>
dsmctl nas remove <name>
dsmctl system info [--nas <name>] [--json]
```

MCP:

```text
list_nas
get_system_info { nas?: string }
```

## Extension rule

A new management feature should normally add three small pieces:

1. A typed method and response model under `internal/synology`.
2. A use case under `internal/application`, including validation, idempotency and safety policy.
3. Thin CLI and MCP adapters.

Raw generic DSM calls must not be exposed as MCP tools. Mutating operations should later use a plan/apply pattern so the CLI and MCP host can inspect potentially destructive changes before execution.

## Planned follow-ups

- OS keychain credential resolver and interactive `credentials set` CLI.
- MFA/OTP login challenge support.
- DSM error-code descriptions and structured application errors.
- Capability reporting and DSM compatibility contract tests.
- Control Panel read operations, followed by plan/apply mutations.
- SAN inventory, followed by guarded LUN and target mutations.
