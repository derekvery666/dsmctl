# Drive Admin

The Drive Admin module manages functionality provided by the **Synology Drive
Server package** (`SynologyDrive`), not by DSM itself. It is the first consumer
of dsmctl's package-scoped operation selection: Drive's WebAPI behavior follows
the installed package release rather than the DSM release, so before every
command dsmctl re-reads the installed-package inventory, verifies the package
and its version, and routes to the operation variant whose package-version
range matches. Two NAS on the same DSM build with different Drive versions can
therefore select different backends, and a Drive release older than the
verified baseline fails closed instead of receiving untested requests.

This slice is **read-only**: service status, active connections, team folders,
and Drive server logs. Team-folder changes are modeled in capabilities but fail
closed (see [Deferred operations](#deferred-operations)).

## Capabilities and package evidence

```console
dsmctl drive admin capabilities --nas office
```

Reports, per operation, whether a verified backend was selected, plus the
installed-package evidence used for selection: whether `SynologyDrive` is
installed, the observed version, and whether the package service was running.
The compatibility report's `packages` list and each selection reason carry the
same evidence, so an unsupported module is diagnosable from the output alone:

- package not installed → every operation unsupported with
  "package SynologyDrive is not installed";
- package below the verified baseline (3.0) → unsupported with the observed
  version and required range;
- package installed but stopped → operations stay selected, reads fail with
  guidance to start the package through a Package Center lifecycle plan.

MCP exposes the same result through `get_drive_admin_capabilities`.

## Reads

```console
dsmctl drive admin status --nas office
dsmctl drive admin connections --nas office --json
dsmctl drive admin team-folders --nas office
dsmctl drive admin log list --nas office --limit 50
dsmctl drive admin log list --nas office --username alice --keyword report --from "2026-07-01" --to "2026-07-17"
```

- `status` returns the Drive service status as reported by the package
  (lowercased, for example `enabled`) plus the package evidence observed with
  this exact call.
- `connections` lists active Drive client sessions with the user, device,
  client type, and address fields Drive reports.
- `team-folders` lists shared folders from the admin team-folder view: the
  name, whether each is enabled as a Drive team folder, and Drive's share
  status. Drive's home entry appears as `homes/mydrive_home`.
- `log list` reads Drive server logs. Keyword, username, team-folder scope,
  offset, and the Unix-seconds/`"2006-01-02 15:04:05"` time range are applied
  by Drive; the page size is bounded (default 100, maximum 1000). Drive stores
  log text as a numeric event code plus substitution fields rather than a
  rendered message, so entries surface the structured fields: time, username,
  client type, IP address, event code, path, and team folder.

MCP tools: `get_drive_admin_status`, `get_drive_admin_connections`,
`get_drive_admin_team_folders`, `get_drive_admin_logs`.

## Deferred operations

`drive.admin.teamfolders.set` (enable/disable team folders) is modeled so
capabilities can name it, but it has **no backend and fails closed**
(`team_folders_set: false`). The first verified Drive write will ship through
the same hash-bound plan/apply contract used by Package Center, binding the
plan to the observed team-folder state and the installed package version.
Drive Config/settings writes, connection disconnection, index management, the
end-user file API (`SYNO.SynologyDrive.Files`), sharing links, labels, and
ShareSync are likewise out of scope for this slice.

## DSM backends (verified live on Drive 4.0.3-27892)

API names, versions, request shapes, and response fields were verified against
the configured lab NAS (read-only) with Synology Drive Server **4.0.3-27892**
installed, guided by `SYNO.API.Info` discovery, the package's own Admin
Console assets, and the Drive server source's WebAPI registry
(`synosyncfolder` `server/ui-web/webapi/admin-console/SYNO.SynologyDrive.py`
and `handlers/log/list.cpp`, whose release branches confirm the per-package-
version API surface):

- Status: `SYNO.SynologyDrive` `get_status` v1. The service state is
  `enable_status`; QuickConnect relay fields stay unmodeled.
- Connections: `SYNO.SynologyDrive.Connection` `list` v1 (target advertises
  v1-2; the v1 shape is the verified baseline).
- Team folders: `SYNO.SynologyDrive.Share` `list` v1. The request is rejected
  (error 120) without paging and a valid sort column, so the backend always
  sends `offset`/`limit` with `sort_by: share_name`. Items expose `share_name`,
  the `share_enable` activation flag, and `share_status`.
- Logs: `SYNO.SynologyDrive.Log` `list` v1. `target` is required: the
  all-scopes view is `share_type: all` with `target: user`, and one team
  folder is `share_type: share` with an `@`-prefixed shared-folder name.
  `log_type` is Drive's numeric event-code array filter (sent empty), and
  `keyword`, `username`, `offset`, `limit`, `datefrom`, and `dateto` are
  applied by Drive. Entries are template-coded (numeric `type` plus `s*`/`p*`
  substitution slots).

Every variant additionally requires `SynologyDrive >= 3.0` through the
package-version matcher (see
[the compatibility guide](compatibility.md#package-scoped-operations)).
Response decoders are defensive: a malformed envelope or an unrecognized list
shape returns an explicit decode error naming the available fields instead of
silently returning an empty state — this is exactly how the initial field
assumptions were corrected during live verification. Confirm the selected
backends on any target with `dsmctl drive admin capabilities`.
