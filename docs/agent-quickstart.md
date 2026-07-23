# AI agent quick start

This guide is the shortest safe path for an AI agent with no prior dsmctl
context. The CLI and MCP server use the same application operations, target
selection, compatibility routing, and guarded mutation policy.

## Rules that prevent incorrect operation

1. Work with a named NAS profile. In automation, always state the profile
   explicitly rather than depending on a local default.
2. Never send a password or OTP in CLI JSON or an MCP argument. Authentication
   is prepared out of band with `dsmctl auth login --nas <name>` or the gateway
   console.
3. Prefer a narrow read tool or command. Check the matching `capabilities`
   operation before relying on DSM- or package-dependent behavior.
4. Never invent a raw DSM request. An unavailable typed operation fails closed.
5. A mutation is two separate decisions: create a read-only plan, then apply
   that exact plan only after a human reviews and approves it.

## CLI workflow

Prepare and inspect a target:

```console
dsmctl nas add office --url https://nas.example.com:5001 --username automation --default
dsmctl auth login --nas office
dsmctl nas list
dsmctl auth status --nas office
dsmctl nas capabilities --nas office
```

Use structured output for agent reads:

```console
dsmctl system info --nas office --json
dsmctl storage capabilities --nas office --json
dsmctl storage inventory --nas office --json
```

Discover the complete CLI surface directly from the binary. These commands are
offline and do not require a configuration file, authentication, or NAS
connection:

```console
dsmctl commands list --runnable-only --json
dsmctl commands list --prefix account
dsmctl commands list --prefix "control-panel file-services"
dsmctl commands list --prefix drive
dsmctl commands show account inventory --json
dsmctl commands show control-panel file-services plan --json
dsmctl commands show drive config plan --json
```

The catalog covers command groups and every runnable operation. `commands
show` returns the canonical path, summary, full description, usage, aliases,
subcommands, local and inherited flags, required/default values, workflow role,
structured-output support, and request-schema command where applicable. Roles
are conservative: `plan` is read-only planning, `apply` mutates, `group` is
navigation, and `operation` follows its command-specific description.

For the 44 plan commands that accept an external typed request JSON file,
discover the module-specific request shape with:

```console
dsmctl schema list
dsmctl schema list --json
dsmctl schema show account plan
dsmctl account plan --help
```

`schema show` emits JSON Schema generated from the exact Go type decoded by the
selected plan command. It includes required fields, nested object and array
shapes, field descriptions, credential-reference rules, and
`additionalProperties: false`. Every request-bearing plan help page prints its
exact `schema show` command and a complete plan invocation. Do not guess fields.

Then follow the guarded sequence:

```console
dsmctl <module> plan --nas office --file request.json --output plan.json
# Review target, request, risk, summary, warnings, precondition, and hash.
dsmctl <module> apply --file plan.json --approve <hash-from-plan>
```

Do not edit `plan.json`, substitute a different target, or reuse it after DSM
state changes. Apply re-reads current state and rejects stale or modified plans.
Secrets required by a typed operation use only its documented apply-time
credential reference; they never belong in the request or plan.

## MCP workflow

Two ways to connect a client:

- **Local (stdio):** install the bundled `dsmctl-mcp` binary, run `dsmctl nas
  add` + `dsmctl auth login`, then register it with `claude mcp add <name>
  <path-to-dsmctl-mcp>`. No certificate or DNS — dsmctl pins the NAS's
  certificate by fingerprint.
- **Remote (gateway):** paste the gateway's HTTPS `/mcp` URL into an
  OAuth-capable client. For access from other machines, use HTTPS with a
  certificate the client trusts (free on a domain-less NAS via Synology DDNS +
  Let's Encrypt); plain HTTP is available on a trusted LAN for development only.

MCP clients receive the workflow in the server's initialize instructions. Use
the tool schemas as the source of truth for arguments and follow this order:

1. `list_nas {}` to discover exact configured profile names.
2. `get_auth_status {"nas":"office"}` when authentication is uncertain.
3. A narrow `get_*_capabilities` tool when support may vary.
4. The required `get_*` read with `nas` set explicitly.
5. For a mutation, the matching `plan_*` tool with `nas` plus its typed
   request. Show the returned plan to the user.
6. Only after explicit approval, call the matching `apply_*` tool with the
   exact returned `plan` object and exact `approval_hash`.

Local stdio MCP can resolve a configured default profile, but agents should
still pass `nas` explicitly so the same call is safe and portable to a remote
gateway. Remote tokens also need that NAS in their allowlist. High-risk remote
apply may require a separate, short-lived out-of-band approval.

If authentication is missing, stop and ask the user to authenticate through
the CLI or gateway console. MCP intentionally has no password or OTP argument.
If plan/apply tools are absent, the endpoint is read-only; do not work around
that boundary.

## Where to find request shapes

- MCP: inspect the selected tool's generated input schema.
- CLI operations and flags: run `dsmctl commands list --json`, optionally with
  `--prefix` and `--runnable-only`, then
  `dsmctl commands show <command path...> --json`.
- CLI request bodies: run `dsmctl schema list`, then
  `dsmctl schema show <command path...>`. Every normal command help page links
  to its catalog entry; request-bearing plan help also links to its exact
  schema. Repository module guides provide additional operational context but
  are not required for discovery.
- Compatibility: use `dsmctl nas capabilities --nas <name>` or the matching
  `get_*_capabilities` tool. Do not infer support from the DSM release alone.
