# Synology Office

A settings module for the Synology Office package, package-version gated on
the installed `Spreadsheet` package (the DSM id of Synology Office) like the
Photos, Surveillance, and Download Station modules. A NAS without Office — or
below the verified 3.0 baseline — fails closed with the package evidence in
capabilities and errors.

The module covers the Office **settings** surface: deployment info, the
system-wide administrator setting, the calling user's own editor preferences,
and the font inventory. Document/content APIs (`SYNO.Office.Node*`,
`Permission*`, `Snapshot*`, ...) and collaboration internals
(`SYNO.Office.Shard*`) are deliberately out of scope.

## Reads

```console
dsmctl office capabilities --nas office
dsmctl office info --nas office
dsmctl office settings --nas office
dsmctl office preferences --nas office
dsmctl office fonts --nas office --json
```

- **`capabilities`** reports the installed package evidence (installed,
  version, running) and which operations are available, each selected
  independently.
- **`info`** reads `SYNO.Office.Info` (`get`): the Office version, whether the
  session user is an Office **manager** (can change system settings), and the
  document/spreadsheet/slides schema versions.
- **`settings`** reads `SYNO.Office.Setting.System` (`get`): the one
  system-wide Office setting, `history_prune` — automatic cleanup of old
  document version history (the same toggle the Drive Admin Console exposes
  for Office).
- **`preferences`** reads `SYNO.Office.Setting` (`get`): the calling user's
  own typed editor preferences — ruler, formula preview, formula panel
  opened/expanded, default locale, AI translator language, and AI helper
  languages. Opaque UI-state blobs (panel widths, dismissed hints, formatting
  marks) are not modeled.
- **`fonts`** reads `SYNO.Office.Setting.Font` (`list`) and normalizes DSM's
  name-keyed object into a stable name-sorted list with the localized display
  name when one exists.

MCP exposes the same reads through `get_office_capabilities`,
`get_office_info`, `get_office_settings`, `get_office_preferences`, and
`get_office_fonts`.

## Guarded settings writes

Office changes use the same hash-bound plan/apply contract as the other
modules. One request targets **exactly one scope**:

- `system` — the system-wide configuration (requires an Office manager):

  ```json
  { "system": { "history_prune": false } }
  ```

- `preferences` — the calling account's own editor preferences:

  ```json
  { "preferences": { "ruler": false, "default_locale": "zh-TW" } }
  ```

```console
echo '{"system":{"history_prune":false}}' | dsmctl office plan --nas office -o office.plan.json
dsmctl office apply -f office.plan.json --approve <hash-from-plan>
```

Both scopes are **patch-only**: an omitted field is never sent and DSM
preserves its current value (verified live — an empty `set` is a DSM no-op, so
dsmctl itself rejects an empty patch). The plan records and hashes the
complete current state of the targeted scope; apply rejects a stale state,
re-applies only the approved patch, and re-reads the scope to verify the
requested change actually took effect.

Enabling `history_prune` permanently deletes older document versions, so a
plan that turns it on is **high** risk with an explicit warning; other changes
are medium risk. Preference changes affect only the calling account.

MCP exposes the same contract through `plan_office_change` and
`apply_office_plan`. The read-only gateway strips both.

## DSM backends (verified on DSM 7.3, Synology Office 3.7.2)

API names, methods, and fields are verified live against Synology Office
3.7.2-22592 and the Office 3.7 WebAPI definitions:

- Info: `SYNO.Office.Info` `get` v1 (`version` object rendered as
  `major.minor.hotfix-build`, `is_manager`, `schema_doc`, `schema_sheet`,
  `schema_slide`).
- System settings: `SYNO.Office.Setting.System` `get`/`set` v1. The whole
  surface is the optional boolean `history_prune`; the decoder requires it so
  API drift fails loudly.
- Preferences: `SYNO.Office.Setting` `get`/`set` v1 with the optional typed
  fields `ruler` (required by the decoder as the drift guard),
  `formula_preview`, `formula_panel_opened`, `formula_panel_expanded`,
  `default_locale`, `ai_translator_language`, `ai_helper_languages`.
- Fonts: `SYNO.Office.Setting.Font` `list` v1.

Font mutations (`add`/`enable`/`disable`/`delete`) and the per-document
`SYNO.Office.Setting.UI` / `SYNO.Office.Setting.Person` state (both take an
`object_id`) exist in the API but are deferred. Every operation gates on
`SYNO.API.Info` discovery plus the installed-package inventory; confirm the
selected backends on any target with `dsmctl office capabilities`.
