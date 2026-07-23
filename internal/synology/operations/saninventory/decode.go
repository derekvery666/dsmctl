package saninventory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/derekvery666/dsmctl/internal/domain/san"
)

func decodeTargets(data json.RawMessage) (targetInventory, error) {
	root, err := decodeObject(data, "target inventory")
	if err != nil {
		return targetInventory{}, err
	}
	items, err := requiredObjectList(root, "targets")
	if err != nil {
		return targetInventory{}, fmt.Errorf("decode target inventory: %w", err)
	}

	result := targetInventory{Targets: make([]san.Target, 0, len(items)), Mappings: make([]san.Mapping, 0)}
	seenTargets := make(map[string]struct{}, len(items))
	seenMappings := make(map[string]struct{})
	for index, item := range items {
		id := stringValue(item, "target_id", "id")
		if id == "" {
			return targetInventory{}, fmt.Errorf("decode target inventory: targets[%d] has no stable target_id", index)
		}
		if _, exists := seenTargets[id]; exists {
			return targetInventory{}, fmt.Errorf("decode target inventory: duplicate target_id %q", id)
		}
		seenTargets[id] = struct{}{}

		enabled := boolValue(item, "is_enabled", "enabled")
		status := strings.TrimSpace(stringValue(item, "status"))
		result.Targets = append(result.Targets, san.Target{
			ID:                id,
			Name:              stringValue(item, "name"),
			Description:       stringValue(item, "description", "desc"),
			Protocol:          san.ProtocolISCSI,
			IQN:               stringValue(item, "iqn"),
			Enabled:           enabled,
			Status:            status,
			Health:            targetHealth(status, enabled),
			Authentication:    authenticationValue(item),
			ConnectedSessions: listLength(item, "connected_sessions", "sessions"),
		})

		mappedLUNs, present, err := optionalObjectList(item, "mapped_luns", "mapped_lun")
		if err != nil {
			return targetInventory{}, fmt.Errorf("decode target inventory: targets[%d]: %w", index, err)
		}
		if !present {
			continue
		}
		for mappingIndex, mapping := range mappedLUNs {
			lunID := stringValue(mapping, "lun_uuid", "uuid")
			if lunID == "" {
				return targetInventory{}, fmt.Errorf("decode target inventory: targets[%d].mapped_luns[%d] has no stable lun_uuid", index, mappingIndex)
			}
			key := id + "\x00" + lunID
			if _, exists := seenMappings[key]; exists {
				continue
			}
			seenMappings[key] = struct{}{}
			result.Mappings = append(result.Mappings, san.Mapping{TargetID: id, LUNID: lunID})
		}
	}
	sort.Slice(result.Targets, func(i, j int) bool { return result.Targets[i].ID < result.Targets[j].ID })
	sort.Slice(result.Mappings, func(i, j int) bool {
		if result.Mappings[i].TargetID == result.Mappings[j].TargetID {
			return result.Mappings[i].LUNID < result.Mappings[j].LUNID
		}
		return result.Mappings[i].TargetID < result.Mappings[j].TargetID
	})
	return result, nil
}

func decodeLUNs(data json.RawMessage) ([]san.LUN, error) {
	root, err := decodeObject(data, "LUN inventory")
	if err != nil {
		return nil, err
	}
	items, err := requiredObjectList(root, "luns")
	if err != nil {
		return nil, fmt.Errorf("decode LUN inventory: %w", err)
	}

	result := make([]san.LUN, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		id := stringValue(item, "uuid", "lun_uuid")
		if id == "" {
			return nil, fmt.Errorf("decode LUN inventory: luns[%d] has no stable uuid", index)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("decode LUN inventory: duplicate uuid %q", id)
		}
		seen[id] = struct{}{}

		status := strings.TrimSpace(stringValue(item, "status"))
		typeCode, hasType := integerValue(item, "type")
		result = append(result, san.LUN{
			ID:              id,
			NumericID:       stringValue(item, "lun_id", "id"),
			Name:            stringValue(item, "name"),
			Description:     stringValue(item, "description", "desc"),
			Protocol:        san.ProtocolISCSI,
			Status:          status,
			Health:          lunHealth(status),
			SizeBytes:       uint64Value(item, "size", "size_bytes"),
			AllocatedBytes:  uint64Value(item, "allocated_size", "allocated_bytes"),
			BlockSizeBytes:  uint64Value(item, "block_size", "block_size_bytes"),
			Provisioning:    provisioning(typeCode, hasType, stringValue(item, "type_str")),
			BackingKind:     backingKind(typeCode, hasType),
			BackingLocation: stringValue(item, "location", "volume_path"),
			Mapped:          boolValue(item, "is_mapped", "mapped"),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func decodeObject(data json.RawMessage, label string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var root map[string]json.RawMessage
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	if root == nil {
		return nil, fmt.Errorf("decode %s: response is not an object", label)
	}
	return root, nil
}

func requiredObjectList(root map[string]json.RawMessage, key string) ([]map[string]any, error) {
	raw, ok := root[key]
	if !ok {
		return nil, fmt.Errorf("response has no %q array", key)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("%q is null, not an array", key)
	}
	var items []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&items); err != nil {
		return nil, fmt.Errorf("%q is not an object array: %w", key, err)
	}
	if items == nil {
		items = make([]map[string]any, 0)
	}
	return items, nil
}

func optionalObjectList(values map[string]any, keys ...string) ([]map[string]any, bool, error) {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		items, ok := value.([]any)
		if !ok {
			return nil, true, fmt.Errorf("%q is not an array", key)
		}
		result := make([]map[string]any, 0, len(items))
		for index, item := range items {
			object, ok := item.(map[string]any)
			if !ok {
				return nil, true, fmt.Errorf("%s[%d] is not an object", key, index)
			}
			result = append(result, object)
		}
		return result, true, nil
	}
	return nil, false, nil
}

func stringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			return typed
		case json.Number:
			return typed.String()
		case float64:
			return strconv.FormatFloat(typed, 'f', -1, 64)
		}
	}
	return ""
}

func uint64Value(values map[string]any, keys ...string) uint64 {
	value, ok := integerValue(values, keys...)
	if !ok || value < 0 {
		return 0
	}
	return uint64(value)
}

func integerValue(values map[string]any, keys ...string) (int64, bool) {
	for _, key := range keys {
		switch typed := values[key].(type) {
		case json.Number:
			value, err := strconv.ParseInt(typed.String(), 10, 64)
			return value, err == nil
		case float64:
			return int64(typed), true
		case string:
			value, err := strconv.ParseInt(typed, 10, 64)
			return value, err == nil
		}
	}
	return 0, false
}

func boolValue(values map[string]any, keys ...string) bool {
	for _, key := range keys {
		switch typed := values[key].(type) {
		case bool:
			return typed
		case json.Number:
			return typed.String() != "0"
		case float64:
			return typed != 0
		case string:
			value, err := strconv.ParseBool(typed)
			return (err == nil && value) || typed == "1"
		}
	}
	return false
}

func listLength(values map[string]any, keys ...string) int {
	for _, key := range keys {
		if items, ok := values[key].([]any); ok {
			return len(items)
		}
	}
	return 0
}

func authenticationValue(values map[string]any) string {
	value, ok := integerValue(values, "auth_type", "authentication_type")
	if !ok {
		return "unknown"
	}
	switch value {
	case 0:
		return "none"
	case 1:
		return "chap"
	case 2:
		return "mutual_chap"
	default:
		return "unknown"
	}
}

func provisioning(typeCode int64, hasType bool, typeName string) string {
	if hasType {
		if typeCode&4 != 0 {
			return san.ProvisioningThin
		}
		return san.ProvisioningThick
	}
	typeName = strings.ToUpper(typeName)
	if strings.Contains(typeName, "THIN") || typeName == "BLUN" {
		return san.ProvisioningThin
	}
	if typeName != "" {
		return san.ProvisioningThick
	}
	return san.ProvisioningUnknown
}

func backingKind(typeCode int64, hasType bool) string {
	if !hasType {
		return san.BackingUnknown
	}
	if typeCode&2 != 0 {
		return san.BackingVolume
	}
	return san.BackingStoragePool
}

func targetHealth(status string, enabled bool) string {
	switch strings.ToLower(status) {
	case "connected", "online":
		return "healthy"
	case "processing", "creating", "deleting":
		return "transitional"
	case "offline":
		if enabled {
			return "warning"
		}
		return "disabled"
	case "disabled":
		return "disabled"
	case "":
		return "unknown"
	default:
		return "unknown"
	}
}

func lunHealth(status string) string {
	switch strings.ToLower(status) {
	case "normal", "finished":
		return "healthy"
	case "soft_limit_reach", "volume_offline":
		return "warning"
	case "hard_limit_reach", "unhealthy", "unavailabling", "crashed":
		return "critical"
	case "creating", "deleting", "cloning", "defragging", "expanding", "processing":
		return "transitional"
	case "":
		return "unknown"
	default:
		return "unknown"
	}
}
