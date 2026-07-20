---
id: WI-058
title: Guarded Drive node restore (deleted files and versions)
status: done
priority: P2
owner: ""
depends_on: [WI-057]
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

# WI-058 — Guarded Drive node restore (deleted files and versions)

## Outcome

Complete the rescue story WI-057 started: restore removed nodes (and, if the
API allows selecting one, a specific stored version) in a Drive view through
a guarded plan/apply built on Drive's asynchronous restore task.

## Source evidence (synosyncfolder, 4.0 official branch, gathered in WI-057)

`server/ui-web/src/handlers/node/restore/{start,status,finish}.cpp`:

- `SYNO.SynologyDrive.Node.Restore` `start` (POST, enabled-user with admin
  switch via `view_role`): `target` (user or share, same forms as Node.list),
  `copy_to`, `override` (default true), `include_removed` (default false),
  `nodes` (JSON array). Forks a child that walks the nodes; only one restore
  task runs at a time (`HANDLER_ERR_RESTORE_TASK_RUNNING`), progress is kept
  in shared memory (`task_id`, `current`, `total`, `last_update_time`).
- `status` polls that singleton task (no id param) and answers
  `{current,total}` or an error carrying the task's code; `finish` clears it
  with no params. `start` answers `{task_id}`.
- Confirmed live: each `nodes` entry is `{node_id, sync_id, file_type, path,
  name}` (all from the files read; sync_id defaults to "0" for removed
  nodes). The full source is in `restore/start.cpp` — the child forks,
  restores each item, and `include_removed` recurses into removed folders.

## Implemented scope

- **Restore removed nodes to their latest version**, in place or (with
  `copy_to`) into another folder. Per-node version rollback of a
  *currently-present* file (via `ver_ctime_upper_bound`) is a clean extension
  point but out of this slice — the deleted-file rescue is the high-value,
  fully verifiable case.
- Plan resolves each requested path against a fresh recursive view read
  (including removed entries), binds the resolved nodes by fingerprint, and
  classifies risk: recovering removed nodes is additive (medium); an in-place
  restore that would overwrite a currently-present file is high risk.
- Apply re-plans for the stale check, starts the task, polls `status` to
  completion (10-minute bound), calls `finish`, then verifies via the files
  read. The node view's `is_removed` flag lags the task completion (observed
  live), so the postcondition re-read is bounded-retried before failing.

## Acceptance criteria

- [x] Restore start/status/finish operations with request-capture tests
      (nodes array shape, sync_id default, copy_to forwarding).
- [x] Guarded plan/apply bound to the resolved nodes, async poll, and a
      bounded not-yet-confirmed postcondition.
- [x] CLI (`drive admin restore plan|apply`) + MCP (`plan_drive_restore` /
      `apply_drive_restore_plan`) with read-only gateway exclusion.
- [x] Live verification on Drive 4.0.3-27892 (2026-07-20): uploaded a file to
      a disposable team folder, let Drive index it, deleted it via
      FileStation (Drive marked it removed), restored it via the new write
      (re-read confirmed not-removed and the file returned on disk), then
      disabled the team folder and deleted the share.

## Deferred

Per-node version rollback (`ver_ctime_upper_bound` from `list_version`),
admin `view_role` impersonation to restore another user's My Drive, and
`Node.Download`/`Node.Delete` (scheduled purge) remain out of scope.
