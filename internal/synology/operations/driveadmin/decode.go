package driveadmin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ychiu1211/dsmctl/internal/domain/driveadmin"
)

// Decoders are strict about the response envelope and the list container so a
// changed Drive response shape surfaces as an explicit error instead of a
// silently empty state, and lenient about per-item fields because their
// presence varies across package versions.

// decodeServiceStatus reads get_status. Verified live on Drive 4.0.3: the
// service state is enable_status ("enabled"); the response also carries
// QuickConnect relay fields and freeze flags that stay unmodeled.
func decodeServiceStatus(data json.RawMessage) (driveadmin.ServiceStatus, error) {
	root, err := decodeObject(data, "Drive service status")
	if err != nil {
		return driveadmin.ServiceStatus{}, err
	}
	status := stringValue(root, "enable_status", "status", "service_status", "state")
	if status == "" {
		return driveadmin.ServiceStatus{}, fmt.Errorf("decode Drive service status: no status field among %s", availableKeys(root))
	}
	return driveadmin.ServiceStatus{Status: strings.ToLower(status)}, nil
}

func decodeConnections(data json.RawMessage) (driveadmin.Connections, error) {
	root, err := decodeObject(data, "Drive connection list")
	if err != nil {
		return driveadmin.Connections{}, err
	}
	items, ok := objectList(root, "items", "connections", "data")
	if !ok {
		return driveadmin.Connections{}, fmt.Errorf("decode Drive connection list: no connection array among %s", availableKeys(root))
	}
	result := driveadmin.Connections{Connections: make([]driveadmin.Connection, 0, len(items))}
	for _, item := range items {
		result.Connections = append(result.Connections, driveadmin.Connection{
			User:       stringValue(item, "username", "user", "owner"),
			DeviceName: stringValue(item, "device_name", "computer_name", "hostname", "device"),
			ClientType: strings.ToLower(stringValue(item, "client_type", "type", "platform")),
			Address:    stringValue(item, "address", "ip", "ip_address"),
		})
	}
	result.Total = intValue(root, "total")
	if result.Total == 0 {
		result.Total = len(result.Connections)
	}
	return result, nil
}

// decodeTeamFolders reads Share.list. Verified live on Drive 4.0.3: items carry
// share_name, the share_enable team-folder activation flag, and share_status
// ("normal"); watermark and rotation settings stay unmodeled.
func decodeTeamFolders(data json.RawMessage) (driveadmin.TeamFolders, error) {
	root, err := decodeObject(data, "Drive team folder list")
	if err != nil {
		return driveadmin.TeamFolders{}, err
	}
	items, ok := objectList(root, "items", "shares", "team_folders", "data")
	if !ok {
		return driveadmin.TeamFolders{}, fmt.Errorf("decode Drive team folder list: no team folder array among %s", availableKeys(root))
	}
	result := driveadmin.TeamFolders{TeamFolders: make([]driveadmin.TeamFolder, 0, len(items))}
	for index, item := range items {
		name := stringValue(item, "share_name", "name", "title")
		if name == "" {
			return driveadmin.TeamFolders{}, fmt.Errorf("decode Drive team folder %d: no name field among %s", index, availableKeys(item))
		}
		enabled, _ := boolValue(item, "share_enable", "enabled")
		result.TeamFolders = append(result.TeamFolders, driveadmin.TeamFolder{
			Name:    name,
			Enabled: enabled,
			Status:  strings.ToLower(stringValue(item, "share_status", "status", "state")),
		})
	}
	result.Total = intValue(root, "total")
	if result.Total == 0 {
		result.Total = len(result.TeamFolders)
	}
	return result, nil
}

// decodeLog reads Log.list. Verified live on Drive 4.0.3: entries are
// template-coded — a numeric event type plus substitution slots (s1..s5 paths,
// p1..p5 values) — rather than rendered text, so the structured fields are
// surfaced directly.
func decodeLog(data json.RawMessage) (driveadmin.Log, error) {
	root, err := decodeObject(data, "Drive log list")
	if err != nil {
		return driveadmin.Log{}, err
	}
	items, ok := objectList(root, "items", "logs", "data")
	if !ok {
		return driveadmin.Log{}, fmt.Errorf("decode Drive log list: no log array among %s", availableKeys(root))
	}
	result := driveadmin.Log{Entries: make([]driveadmin.LogEntry, 0, len(items))}
	for _, item := range items {
		result.Entries = append(result.Entries, driveadmin.LogEntry{
			TimeUnix:   int64Value(item, "time"),
			Username:   stringValue(item, "username", "user"),
			ClientType: strings.ToLower(stringValue(item, "client_type")),
			IPAddress:  stringValue(item, "ip_address", "ip"),
			EventType:  intValue(item, "type"),
			Path:       stringValue(item, "s1"),
			TeamFolder: stringValue(item, "share_name"),
		})
	}
	result.Total = intValue(root, "total")
	if result.Total == 0 {
		result.Total = len(result.Entries)
	}
	return result, nil
}

func decodeObject(data json.RawMessage, what string) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("decode %s: %w", what, err)
	}
	if root == nil {
		return nil, fmt.Errorf("decode %s: response is not an object", what)
	}
	return root, nil
}

// objectList reads the first present array field, keeping object items. It
// reports whether any candidate key held an array so callers can distinguish
// an empty list from an unrecognized response shape.
func objectList(root map[string]any, keys ...string) ([]map[string]any, bool) {
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
		return result, true
	}
	return nil, false
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
	return int(int64Value(values, keys...))
}

func int64Value(values map[string]any, keys ...string) int64 {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case json.Number:
			if parsed, err := typed.Int64(); err == nil {
				return parsed
			}
		case float64:
			return int64(typed)
		}
	}
	return 0
}

// boolValue reads the first present boolean field. Drive reports "-" for
// fields that do not apply to an item (seen live on disabled shares), so
// non-boolean values are skipped rather than treated as false.
func boolValue(values map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		if typed, ok := value.(bool); ok {
			return typed, true
		}
	}
	return false, false
}

func availableKeys(values map[string]any) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return "[" + strings.Join(keys, ", ") + "]"
}
