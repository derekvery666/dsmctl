# Control Panel modules

Control Panel support is organized as small typed modules. Each module owns
its state, capability names, DSM API variants, CLI subtree, and MCP tools. This
keeps a DSM-version change in one setting area from turning the shared wrapper
into an untyped `set key=value` API.

## Time and NTP

The first module is read-only and returns the configured time zone, DSM date
and time display formats, synchronization mode, and ordered NTP server list:

```console
dsmctl control-panel time capabilities --nas office
dsmctl control-panel time state --nas office --json
```

MCP exposes the same application results through
`get_control_panel_time_capabilities` and `get_control_panel_time_state`.

The compatibility layer selects `SYNO.Core.Region.NTP` v3, then v2, then a
legacy v1 decoder. V1 does not provide the display-format fields, so they are
omitted instead of synthesized. A missing API makes only this module
unsupported; it does not disable storage, SAN, account, or share features.

No time or NTP mutation is exposed. A future module-specific change contract
must define clock-change risk, stale-state protection, postcondition checks,
and NTP reachability behavior before `set` can be enabled.

## Adding another module

Add a dedicated type under `internal/domain/controlpanel`, an operation package
with strict response decoding and version-scoped variants, and one Synology
facade. Then expose that facade through the shared application service, CLI,
MCP, and compatibility report. Do not add raw DSM response maps or a generic
settings mutation endpoint.
