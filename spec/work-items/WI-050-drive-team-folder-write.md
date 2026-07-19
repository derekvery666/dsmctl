---
id: WI-050
title: Guarded Drive team-folder enable/disable and versioning
status: done
priority: P1
owner: ""
depends_on: [WI-022, WI-031]
parallel_group: C
touches:
  - internal/domain/driveadmin
  - internal/synology/operations/driveadmin
  - internal/synology/driveadmin.go
  - internal/application/drive_admin.go
  - internal/cli/drive.go
  - internal/mcpserver/server.go
  - internal/mcpserver/read_only.go
  - docs/drive-admin.md
---

# WI-050 — Guarded Drive team-folder enable/disable and versioning

## Outcome

Replaces the WI-022 `drive.admin.teamfolders.set` fail-closed stub with a
verified backend: a CLI user or MCP agent can enable a shared folder as a Drive
team folder (with explicit versioning), disable it, or change an enabled team
folder's versioning, through the standard hash-bound plan/apply contract. This
completes the "set up Drive and open team folders" journey that previously
dead-ended at the stub.

## Scope

- Extend the team-folder read (`SYNO.SynologyDrive.Share` `list`) with the
  fields the write must bind to: share type and, for enabled folders, the
  versioning triple (`rotate_cnt` → max_versions, `rotate_policy` → fifo/smart,
  `rotate_days` → retention days). Drive reports these as the literal string
  `"-"` on disabled folders; the decoder maps that to absent.
- Team-folder write via `SYNO.SynologyDrive.Share` `set` (v1), package-gated on
  `SynologyDrive >= 3.0` like every other Drive Admin operation:
  - `enable`: activates a shared folder as a team folder. DSM requires
    `rotate_cnt` at enable time, so the intent requires `max_versions`
    (0..32; 0 = versioning off) and, when versioning is on, an explicit
    `version_policy` (`fifo` or `smart`) so nothing depends on server-side
    struct defaults. `retention_days` (0..120, 0 = keep) defaults to 0.
  - `disable`: deactivates the team folder. High risk and destructive: Drive
    deletes its team-folder database including version history (files in the
    shared folder are not touched).
  - `set_versioning`: patches any of the three versioning fields on an enabled
    team folder. DSM merges omitted fields server-side from the current view
    settings; the plan computes and shows the merged result. Reducing
    max_versions, disabling versioning, or tightening retention prunes stored
    versions and is high risk.
- Plans bind to the observed team-folder entry (name, enabled, type, status,
  versioning) via fingerprint; apply re-reads, rejects stale state, performs
  the typed set, and verifies the postcondition against the re-read list. The
  Share.set handler silently skips ineligible shares (non-syncable, or
  `surveillance` for config-only changes), so the postcondition re-read is the
  authority; a not-yet-converged state returns an explicit not-yet-confirmed
  error after bounded retries.
- The Drive home entry (`homes/mydrive*`) is rejected: My Drive activation
  follows the DSM home service and its versioning write shares state across
  every user home, out of scope for a per-team-folder change. `surveillance`
  is rejected because Drive ignores it silently.
- CLI: `drive admin team-folders plan|apply` under the existing list command;
  the list table gains TYPE and versioning columns.
- MCP: `plan_drive_team_folder_change` + `apply_drive_team_folder_plan`, both
  stripped from the read-only gateway; the remote gateway classifies them by
  the standard plan_/apply_ prefixes and the high-risk approval flow applies.

## Non-goals

- Watermark and download-restriction fields on the same API (Advanced
  Features license area).
- Node locking (BSM), hybrid share rotate span, index-home migration methods.
- Connection disconnection and other Drive Admin writes.

## Design constraints

- Source evidence (synosyncfolder, SynologyDrive-4-0-2025Q4-official-branch;
  identical on 4.1/master):
  `server/ui-web/src/handlers/share/set.cpp` — method `set` takes a `share`
  array; presence of `share_enable` routes an entry to enable/disable
  (ShareSave), absence to versioning-only (RotateCountSet). Enable requires
  `rotate_cnt` ("rotate_cnt is required for enabling a Team Folder");
  `rotate_cnt` 0..32, `rotate_policy` fifo|smart, `rotate_days` 0..120;
  `rotate_cnt == 0` forces fifo/0. Disable removes the share user and view
  database ("Share … deleted since user has disable it as a teamfolder").
  `server/ui-web/src/handlers/share/list.cpp` — items carry `share_type` and,
  when enabled, the versioning triple; `"-"` otherwise. The registry
  (`server/ui-web/webapi/admin-console/SYNO.SynologyDrive.py`) exposes `set`
  at v1 and v2 with identical parameters (v2 differs only in publish
  metadata), POST, admin-only.
- Field names and behavior must be live-verified against the configured lab
  NAS (Drive 4.0.3) before trusting source, per the standing evidence policy.
- One change per plan: the `share` array is sent with exactly one entry so a
  plan maps to one team folder and the postcondition is unambiguous.
- Live mutations only on a disposable `dsmctl-e2e-*` shared folder created for
  the test and deleted afterwards with stable-ID-verified cleanup.

## Acceptance criteria

- [x] Team-folder read surfaces share type and versioning with `"-"` mapped to
      absent; request-capture/decoder tests updated.
- [x] `drive.admin.teamfolders.set` selects a verified v1 backend gated on the
      package baseline; capabilities report it truthfully.
- [x] Guarded enable/disable/set_versioning with entry-bound fingerprint,
      merged-versioning summary, risk classification (disable and
      version-pruning changes high risk), and bounded postcondition retries.
- [x] CLI plan/apply plus versioning columns; MCP plan/apply tools registered,
      excluded from the read-only gateway, and shaped for the remote pending-
      approval recorder.
- [x] DSM live verification (lab, authorized, fully reverted): create
      `dsmctl-e2e-*` share → enable with versioning → set_versioning →
      disable → delete share.

## Verification

- `go test ./... -count=1`, `go vet ./...`, CLI and MCP builds.
- Live enable/set_versioning/disable cycle on the DSM 7.3.2 lab NAS
  (Drive 4.0.3-27892), 2026-07-20, fully reverted.
