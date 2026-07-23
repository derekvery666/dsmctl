package packagecenter

import "github.com/derekvery666/dsmctl/internal/domain/packagecenter"

// encodeSettings serializes the writable settings to DSM `set` parameters. Only
// the automatic-update policy is written; the three DSM fields are kept
// consistent: `enable_autoupdate` is the master toggle, and `autoupdateimportant`
// / `autoupdateall` select important-only vs all updates when it is on. Trust
// level is intentionally omitted because no DSM endpoint writes it. The
// application layer merges the patch into a freshly read full state before
// calling this, so an omitted patch field cannot silently reset a DSM value.
func encodeSettings(desired packagecenter.Settings) map[string]any {
	return map[string]any{
		"enable_autoupdate":   desired.AutoUpdateEnabled,
		"autoupdateimportant": desired.AutoUpdateEnabled && desired.AutoUpdateImportantOnly,
		"autoupdateall":       desired.AutoUpdateEnabled && !desired.AutoUpdateImportantOnly,
	}
}
