package snapshotreplication

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ychiu1211/dsmctl/internal/domain/snapshotreplication"
)

// DSM field names, live-verified on DSM 7.3-81168 unless noted.
const (
	keySnapshotTime        = "time"
	keySnapshotDescription = "desc"
	keySnapshotLock        = "lock"
	keySnapshotSchedule    = "schedule_snapshot"
	keySnapshotWormLock    = "worm_lock"

	keyConfSnapshotBrowsing = "enable_snapshot_browsing"
	keyConfLocalTimeFormat  = "snapshot_local_time_format"
)

func decodeShareSnapshots(share string, data json.RawMessage) (snapshotreplication.ShareSnapshots, error) {
	raw, err := decodeObject(data, "share snapshots")
	if err != nil {
		return snapshotreplication.ShareSnapshots{}, err
	}
	items, ok := raw["snapshots"]
	if !ok {
		return snapshotreplication.ShareSnapshots{}, fmt.Errorf("decode share snapshots: required field %q is missing", "snapshots")
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(items, &entries); err != nil {
		return snapshotreplication.ShareSnapshots{}, fmt.Errorf("decode share snapshots list: %w", err)
	}
	snapshots := make([]snapshotreplication.Snapshot, 0, len(entries))
	for index, entry := range entries {
		time := optionalString(entry, keySnapshotTime)
		if time == "" {
			return snapshotreplication.ShareSnapshots{}, fmt.Errorf("decode share snapshots: snapshot %d has no time name", index)
		}
		snapshots = append(snapshots, snapshotreplication.Snapshot{
			Time:            time,
			Description:     optionalString(entry, keySnapshotDescription),
			Locked:          optionalBool(entry, keySnapshotLock),
			ScheduleCreated: optionalBool(entry, keySnapshotSchedule),
			WormLocked:      optionalBool(entry, keySnapshotWormLock),
		})
	}
	result := snapshotreplication.ShareSnapshots{Share: share, Snapshots: snapshots}
	result.Total = intOr(raw, "total", len(snapshots))
	return result, nil
}

func decodeShareConfig(share string, data json.RawMessage) (snapshotreplication.ShareConfig, error) {
	raw, err := decodeObject(data, "share snapshot configuration")
	if err != nil {
		return snapshotreplication.ShareConfig{}, err
	}
	browsing, err := requiredBool(raw, keyConfSnapshotBrowsing, "share snapshot configuration")
	if err != nil {
		return snapshotreplication.ShareConfig{}, err
	}
	return snapshotreplication.ShareConfig{
		Share:            share,
		SnapshotBrowsing: browsing,
		LocalTimeFormat:  optionalBool(raw, keyConfLocalTimeFormat),
	}, nil
}

func decodeRetentionPolicy(share string, data json.RawMessage) (snapshotreplication.RetentionPolicy, error) {
	raw, err := decodeObject(data, "retention policy")
	if err != nil {
		return snapshotreplication.RetentionPolicy{}, err
	}
	// tid is the stable field: -1 means no retention task; require it to catch
	// API drift, decode the policy numbers leniently.
	taskID, ok := parseInt(raw["tid"])
	if !ok {
		return snapshotreplication.RetentionPolicy{}, fmt.Errorf("decode retention policy: required field %q is missing or not a number", "tid")
	}
	scheduled := false
	if schedule, present := raw["schedule"]; present {
		trimmed := bytes.TrimSpace(schedule)
		scheduled = len(trimmed) != 0 && !bytes.Equal(trimmed, []byte("null"))
	}
	return snapshotreplication.RetentionPolicy{
		Share:      share,
		TaskID:     taskID,
		PolicyType: intOr(raw, "policyType", 0),
		KeepRecent: intOr(raw, "recently", 0),
		RetainDays: intOr(raw, "retainDay", 0),
		Hourly:     intOr(raw, "hourly", 0),
		Daily:      intOr(raw, "daily", 0),
		Weekly:     intOr(raw, "weekly", 0),
		Monthly:    intOr(raw, "monthly", 0),
		Yearly:     intOr(raw, "yearly", 0),
		Scheduled:  scheduled,
	}, nil
}

func decodeLogPage(data json.RawMessage) (snapshotreplication.LogPage, error) {
	raw, err := decodeObject(data, "snapshot replication log")
	if err != nil {
		return snapshotreplication.LogPage{}, err
	}
	items, ok := raw["log_list"]
	if !ok {
		return snapshotreplication.LogPage{}, fmt.Errorf("decode snapshot replication log: required field %q is missing", "log_list")
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(items, &entries); err != nil {
		return snapshotreplication.LogPage{}, fmt.Errorf("decode snapshot replication log list: %w", err)
	}
	page := snapshotreplication.LogPage{
		Total:      intOr(raw, "total", len(entries)),
		ErrorCount: intOr(raw, "error_count", 0),
		WarnCount:  intOr(raw, "warn_count", 0),
		InfoCount:  intOr(raw, "info_count", 0),
		Entries:    make([]snapshotreplication.LogEntry, 0, len(entries)),
	}
	// Entry shape live-verified on DSM 7.3-81168: time is a formatted string
	// and the text lives under "event".
	for _, entry := range entries {
		page.Entries = append(page.Entries, snapshotreplication.LogEntry{
			Time:    optionalString(entry, "time"),
			Level:   optionalString(entry, "level"),
			User:    optionalString(entry, "user"),
			Message: firstString(entry, "event", "log", "msg"),
		})
	}
	return page, nil
}

func decodeNodeIdentity(data json.RawMessage) (snapshotreplication.NodeIdentity, error) {
	raw, err := decodeObject(data, "replication node identity")
	if err != nil {
		return snapshotreplication.NodeIdentity{}, err
	}
	identity := snapshotreplication.NodeIdentity{
		Hostname: optionalString(raw, "hostname"),
		NodeID:   optionalString(raw, "node_id"),
		Serial:   optionalString(raw, "serial"),
	}
	if identity.Hostname == "" && identity.NodeID == "" {
		return snapshotreplication.NodeIdentity{}, fmt.Errorf("decode replication node identity: neither hostname nor node_id is present")
	}
	return identity, nil
}

func decodeReplicationPlans(data json.RawMessage) (snapshotreplication.ReplicationPlans, error) {
	raw, err := decodeObject(data, "replication plans")
	if err != nil {
		return snapshotreplication.ReplicationPlans{}, err
	}
	items := firstRaw(raw, "plans", "plan_list")
	if items == nil {
		return snapshotreplication.ReplicationPlans{}, fmt.Errorf("decode replication plans: no plan list field is present")
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(items, &entries); err != nil {
		return snapshotreplication.ReplicationPlans{}, fmt.Errorf("decode replication plan list: %w", err)
	}
	plans := make([]snapshotreplication.ReplicationPlan, 0, len(entries))
	for _, entry := range entries {
		plans = append(plans, snapshotreplication.ReplicationPlan{
			ID:         firstString(entry, "plan_id", "id"),
			Name:       firstString(entry, "name", "display_name"),
			TargetType: firstString(entry, "target_type", "type"),
			Status:     firstString(entry, "status", "state"),
		})
	}
	result := snapshotreplication.ReplicationPlans{Plans: plans}
	result.Total = intOr(raw, "total", len(plans))
	return result, nil
}

// decodeCreatedSnapshot reads the create response: DSM returns the new
// snapshot's time name as a bare JSON string (live-verified); an object with a
// snapshot/time field is tolerated.
func decodeCreatedSnapshot(data json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(data)
	var name string
	if err := json.Unmarshal(trimmed, &name); err == nil && name != "" {
		return name, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err == nil {
		if name := firstString(raw, "snapshot", keySnapshotTime); name != "" {
			return name, nil
		}
	}
	return "", fmt.Errorf("decode snapshot create response: no snapshot time name returned")
}

func decodeObject(data json.RawMessage, what string) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, fmt.Errorf("decode %s: expected a non-empty object", what)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return nil, fmt.Errorf("decode %s object: %w", what, err)
	}
	return raw, nil
}

func firstRaw(raw map[string]json.RawMessage, names ...string) json.RawMessage {
	for _, name := range names {
		if value, ok := raw[name]; ok {
			return value
		}
	}
	return nil
}

func firstString(raw map[string]json.RawMessage, names ...string) string {
	for _, name := range names {
		if value := optionalString(raw, name); value != "" {
			return value
		}
	}
	return ""
}

func optionalString(raw map[string]json.RawMessage, name string) string {
	if value, ok := raw[name]; ok {
		var text string
		if err := json.Unmarshal(value, &text); err == nil {
			return text
		}
	}
	return ""
}

func requiredBool(raw map[string]json.RawMessage, name, what string) (bool, error) {
	value, ok := raw[name]
	if !ok {
		return false, fmt.Errorf("decode %s: required field %q is missing", what, name)
	}
	result, ok := parseBool(value)
	if !ok {
		return false, fmt.Errorf("decode %s field %q: expected boolean", what, name)
	}
	return result, nil
}

func optionalBool(raw map[string]json.RawMessage, name string) bool {
	if value, ok := raw[name]; ok {
		if result, parsed := parseBool(value); parsed {
			return result
		}
	}
	return false
}

func parseBool(value json.RawMessage) (bool, bool) {
	var result bool
	if err := json.Unmarshal(value, &result); err == nil {
		return result, true
	}
	var integer int
	if err := json.Unmarshal(value, &integer); err == nil && (integer == 0 || integer == 1) {
		return integer == 1, true
	}
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		if parsed, convErr := strconv.Atoi(strings.TrimSpace(text)); convErr == nil && (parsed == 0 || parsed == 1) {
			return parsed == 1, true
		}
	}
	return false, false
}

func intOr(raw map[string]json.RawMessage, name string, fallback int) int {
	if value, ok := parseInt(raw[name]); ok {
		return value
	}
	return fallback
}

func parseInt(value json.RawMessage) (int, bool) {
	parsed, ok := parseInt64(value)
	return int(parsed), ok
}

func parseInt64(value json.RawMessage) (int64, bool) {
	if value == nil {
		return 0, false
	}
	var number int64
	if err := json.Unmarshal(value, &number); err == nil {
		return number, true
	}
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		if parsed, convErr := strconv.ParseInt(strings.TrimSpace(text), 10, 64); convErr == nil {
			return parsed, true
		}
	}
	return 0, false
}
