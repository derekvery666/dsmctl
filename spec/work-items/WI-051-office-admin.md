---
id: WI-051
title: Synology Office settings module
status: done
priority: P2
owner: ""
depends_on: [WI-019, WI-022]
parallel_group: C
touches:
  - internal/domain/office
  - internal/synology/operations/office
  - internal/synology/office.go
  - internal/application/office.go
  - internal/cli/office.go
  - internal/mcpserver/server.go
  - internal/mcpserver/read_only.go
  - docs/office.md
---

# WI-051 — Synology Office settings module

## Outcome

A CLI user or MCP agent can read the Synology Office deployment info, the
system (administrator) setting, the caller's own editor preferences, and the
font inventory — and change the system setting and typed preferences through
the shared hash-bound plan/apply contract. The module is package-version gated
on the installed `Spreadsheet` package (product name Synology Office),
mirroring the Photos module (WI-030).

## Scope

- Read `SYNO.Office.Info` (get v1): package version, whether the session user
  is an Office manager, document schema versions.
- Read `SYNO.Office.Setting.System` (get v1): `history_prune` — automatic
  version-history cleanup, the one system-wide Office setting (surfaced in the
  Drive Admin Console "history prune" module).
- Read `SYNO.Office.Setting` (get v1): the caller's own editor preferences,
  typed subset only: ruler, formula preview, formula panel opened/expanded,
  default locale, AI translator language, AI helper languages.
- Read `SYNO.Office.Setting.Font` (list v1): normalized font inventory
  (name + optional localized display name).
- Guarded writes through one plan/apply pair with two patch scopes:
  - `system`: `history_prune` via `SYNO.Office.Setting.System` set v1.
  - `preferences`: the typed per-user subset via `SYNO.Office.Setting` set v1.
  Patch-only: omitted fields are never sent, DSM preserves them (verified
  live). Postcondition re-read verifies the requested change took effect.
- Package-version gating on `Spreadsheet` (>= 3.0, verified on 3.7.2-22592) so
  a NAS without Office fails closed with package evidence.

## Non-goals

- Font mutations: `SYNO.Office.Setting.Font` add/enable/disable/delete
  (custom-font upload needs the binary transport and a disposable font asset).
- Per-object state: `SYNO.Office.Setting.UI` and `SYNO.Office.Setting.Person`
  (get/set take a per-document `object_id`; notification prefs are per file).
- Opaque editor UI-state blobs in `SYNO.Office.Setting` (`formatting_marks`,
  `preference_settings`, `focus_mode`, `hide_hint`, `side_panel_width`).
- Document/content surfaces (`SYNO.Office.Node*`, `File`, `Permission*`,
  `Template*`, `Snapshot*`) and collaboration internals (`SYNO.Office.Shard*`).

## Design constraints

- DSM field names confirmed live on Synology Office 3.7.2-22592 (lab, DSM 7.3)
  and against the Office 3.7 WebAPI definition source
  (`synoffice/webapi/mgr/setting`): system get/set carry only `history_prune`
  (optional bool, patch); user set accepts optional `formula_preview`, `ruler`,
  `formula_panel_opened`, `formula_panel_expanded`, `default_locale`,
  `ai_translator_language`, `ai_helper_languages`.
- Live-verified patch semantics: an empty `set` is a DSM no-op success, so the
  application layer must reject an empty patch itself; a one-field patch
  changes only that field (verified with a fully reverted `ruler` round-trip).
- The font list is a JSON object keyed by font name; normalize to a sorted
  slice so output is stable.
- Disabling `history_prune` grows version storage unbounded; enabling it
  deletes old document versions — the `system` scope is high risk in the
  enabling direction (data removal) and medium otherwise.
- Preferences writes touch only the calling account's own editor settings and
  are medium risk.

## Acceptance criteria

- [x] Info, system setting, preferences, and font list decode with semantic
      names; system decode requires `history_prune` to catch API drift; font
      map normalizes to a sorted list.
- [x] Package-version gating: every operation fails closed without the
      `Spreadsheet` package, with package evidence in capabilities and errors.
- [x] Guarded plan/apply with request-capture tests proving patch-only DSM
      field encoding for both scopes, plus postcondition verification.
- [x] CLI (`office capabilities|info|settings|preferences|fonts|plan|apply`)
      and MCP tools (`get_office_capabilities`, `get_office_info`,
      `get_office_settings`, `get_office_preferences`, `get_office_fonts`,
      `plan_office_change`, `apply_office_plan`) with read-only-gateway
      exclusion of plan/apply.
- [x] DSM 7.3 live verification (lab, authorized, fully reverted): read all
      four surfaces; toggle `history_prune` true→false→true and one
      preference through plan/apply with postcondition verification.

## Verification

- Decoder + request-capture tests; `go test ./... -count=1`, `go vet ./...`
  (pass).
- Live on the DSM 7.3 lab NAS (Office 3.7.2-22592), all reverted:
  - CLI reads: `office capabilities|info|settings|preferences|fonts --nas lab`.
  - CLI writes: `history_prune` true→false (medium) →true (high, with the
    data-removal warning) and `ruler` true→false→true, each through
    plan/apply with postcondition verification.
  - MCP stdio: `get_office_settings`, `get_office_info`, and
    `plan_office_change` (plan only) against the built `dsmctl-mcp`.

## Coordination

Parallel group C. No file overlap with in-progress WI-017 (gateway
distribution); shares `internal/mcpserver/server.go` with any concurrent
module work — none active at claim time.
