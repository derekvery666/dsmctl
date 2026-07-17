package syslogread

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ychiu1211/dsmctl/internal/domain/syslog"
)

func decode(data json.RawMessage) (syslog.State, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return syslog.State{}, fmt.Errorf("decode log list: %w", err)
	}
	if root == nil {
		return syslog.State{}, fmt.Errorf("decode log list: response is not an object")
	}
	state := syslog.State{
		Total:      intValue(root, "total"),
		InfoCount:  intValue(root, "infoCount", "info_count"),
		WarnCount:  intValue(root, "warnCount", "warn_count"),
		ErrorCount: intValue(root, "errorCount", "error_count"),
		Entries:    make([]syslog.Entry, 0),
	}
	for _, item := range objectList(root, "items", "data", "logs") {
		state.Entries = append(state.Entries, syslog.Entry{
			Time:    stringValue(item, "time"),
			Level:   normalizeLevel(stringValue(item, "level")),
			Type:    stringValue(item, "orginalLogType", "originalLogType", "logtype"),
			Who:     stringValue(item, "who", "user"),
			Message: stringValue(item, "descr", "description", "message"),
		})
	}
	return state, nil
}

// objectList reads the first present array-of-objects field. DSM returns log
// rows as decoded map[string]any objects because the response uses json.Number.
func objectList(root map[string]any, keys ...string) []map[string]any {
	for _, key := range keys {
		value, ok := root[key]
		if !ok || value == nil {
			continue
		}
		items, ok := value.([]any)
		if !ok {
			continue
		}
		result := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if object, ok := item.(map[string]any); ok {
				result = append(result, object)
			}
		}
		return result
	}
	return nil
}

func stringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				return trimmed
			}
		case json.Number:
			return typed.String()
		}
	}
	return ""
}

func intValue(values map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case json.Number:
			if parsed, err := typed.Int64(); err == nil {
				return int(parsed)
			}
		case float64:
			return int(typed)
		}
	}
	return 0
}

// normalizeLevel maps DSM severity spellings to the syslog.Level* constants.
func normalizeLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "info", "information", "notice":
		return syslog.LevelInfo
	case "warn", "warning":
		return syslog.LevelWarning
	case "err", "error", "crit", "critical", "alert", "emerg":
		return syslog.LevelError
	default:
		return strings.TrimSpace(value)
	}
}
