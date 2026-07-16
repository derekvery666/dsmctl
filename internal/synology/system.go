package synology

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type SystemInfo struct {
	Hostname        string   `json:"hostname,omitempty" jsonschema:"NAS hostname"`
	Model           string   `json:"model,omitempty" jsonschema:"Synology model name"`
	Serial          string   `json:"serial,omitempty" jsonschema:"Device serial number"`
	DSMVersion      string   `json:"dsm_version,omitempty" jsonschema:"Installed DSM version"`
	CPU             string   `json:"cpu,omitempty" jsonschema:"CPU description"`
	CPUCores        int      `json:"cpu_cores,omitempty" jsonschema:"Number of CPU cores"`
	MemoryMiB       int64    `json:"memory_mib,omitempty" jsonschema:"Installed memory in MiB"`
	Uptime          string   `json:"uptime,omitempty" jsonschema:"NAS uptime reported by DSM"`
	TimeZone        string   `json:"time_zone,omitempty" jsonschema:"Configured time zone"`
	TemperatureC    *float64 `json:"temperature_c,omitempty" jsonschema:"System temperature in Celsius"`
	TemperatureWarn bool     `json:"temperature_warning,omitempty" jsonschema:"Whether DSM reports a temperature warning"`
}

func (c *Client) SystemInfo(ctx context.Context) (SystemInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureAPIsLocked(ctx, authAPI, systemInfoAPI); err != nil {
		return SystemInfo{}, fmt.Errorf("discover system info API: %w", err)
	}
	data, err := c.callLocked(ctx, systemInfoAPI, "info", url.Values{})
	if err != nil {
		return SystemInfo{}, fmt.Errorf("get system info: %w", err)
	}
	return decodeSystemInfo(data)
}

func decodeSystemInfo(data json.RawMessage) (SystemInfo, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		return SystemInfo{}, fmt.Errorf("decode system info: %w", err)
	}

	cpuParts := compactStrings(
		stringValue(raw, "cpu_vendor"),
		stringValue(raw, "cpu_family"),
		stringValue(raw, "cpu_series"),
	)
	temperature, hasTemperature := floatValue(raw, "sys_temp", "temperature")
	var temperaturePointer *float64
	if hasTemperature {
		temperaturePointer = &temperature
	}

	return SystemInfo{
		Hostname:        stringValue(raw, "hostname", "server_name"),
		Model:           stringValue(raw, "model"),
		Serial:          stringValue(raw, "serial"),
		DSMVersion:      stringValue(raw, "firmware_ver", "version_string"),
		CPU:             strings.Join(cpuParts, " "),
		CPUCores:        int(int64Value(raw, "cpu_cores")),
		MemoryMiB:       int64Value(raw, "ram_size", "memory_size"),
		Uptime:          stringValue(raw, "up_time", "uptime"),
		TimeZone:        stringValue(raw, "time_zone", "timezone"),
		TemperatureC:    temperaturePointer,
		TemperatureWarn: boolValue(raw, "sys_tempwarn", "temperature_warning"),
	}, nil
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

func int64Value(values map[string]any, keys ...string) int64 {
	for _, key := range keys {
		switch typed := values[key].(type) {
		case json.Number:
			result, _ := typed.Int64()
			return result
		case float64:
			return int64(typed)
		case string:
			result, _ := strconv.ParseInt(typed, 10, 64)
			return result
		}
	}
	return 0
}

func floatValue(values map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		switch typed := values[key].(type) {
		case json.Number:
			result, err := typed.Float64()
			return result, err == nil
		case float64:
			return typed, true
		case string:
			result, err := strconv.ParseFloat(typed, 64)
			return result, err == nil
		}
	}
	return 0, false
}

func boolValue(values map[string]any, keys ...string) bool {
	for _, key := range keys {
		switch typed := values[key].(type) {
		case bool:
			return typed
		case string:
			result, _ := strconv.ParseBool(typed)
			return result
		}
	}
	return false
}

func compactStrings(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(result) > 0 && result[len(result)-1] == value {
			continue
		}
		result = append(result, value)
	}
	return result
}
