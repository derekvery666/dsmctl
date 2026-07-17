// Package syslog holds the stable, DSM-version-independent model for reading DSM
// system logs (Log Center). It is read-only: dsmctl never mutates or clears logs.
package syslog

const (
	// LogType* are the canonical DSM log categories accepted by the log_type
	// filter. DSM reports the same values in each entry's Type field.
	LogTypeSystem       = "system"
	LogTypeConnection   = "connection"
	LogTypeFileTransfer = "fileTransfer"

	// Level* are the normalized severities DSM reports on each entry.
	LevelInfo    = "info"
	LevelWarning = "warn"
	LevelError   = "error"
)

// Entry is one normalized DSM log record.
type Entry struct {
	Time    string `json:"time" jsonschema:"Local DSM timestamp, for example 2026/07/17 13:35:55"`
	Level   string `json:"level" jsonschema:"Normalized severity reported by DSM: info, warn, or error"`
	Type    string `json:"type,omitempty" jsonschema:"Canonical DSM log category such as system, connection, or fileTransfer"`
	Who     string `json:"who,omitempty" jsonschema:"Account or actor associated with the entry"`
	Message string `json:"message" jsonschema:"Human-readable log description"`
}

// State is a page of DSM log entries plus the whole-log severity counts DSM
// reports for the current filter.
type State struct {
	Total      int     `json:"total" jsonschema:"Total number of log entries matching the query before pagination"`
	InfoCount  int     `json:"info_count" jsonschema:"Number of informational entries reported by DSM"`
	WarnCount  int     `json:"warn_count" jsonschema:"Number of warning entries reported by DSM"`
	ErrorCount int     `json:"error_count" jsonschema:"Number of error entries reported by DSM"`
	Entries    []Entry `json:"entries" jsonschema:"Log entries for the requested page"`
}

// StateQuery selects and pages DSM log entries. Keyword and LogType are applied
// by DSM; Level is applied by dsmctl to the retrieved page because DSM does not
// expose a stable server-side severity filter.
type StateQuery struct {
	Limit   int    `json:"limit,omitempty" jsonschema:"Maximum entries to return; defaults to a bounded page size"`
	Offset  int    `json:"offset,omitempty" jsonschema:"Number of newest entries to skip for pagination"`
	Keyword string `json:"keyword,omitempty" jsonschema:"Case-insensitive substring filter applied by DSM"`
	LogType string `json:"log_type,omitempty" jsonschema:"DSM log category filter: system, connection, or fileTransfer"`
	Level   string `json:"level,omitempty" jsonschema:"Client-side severity filter over the retrieved page: info, warn, or error"`
}

// Capabilities reports whether DSM log reading is available on the target.
type Capabilities struct {
	Read bool `json:"read" jsonschema:"Whether DSM system logs can be read"`
}
