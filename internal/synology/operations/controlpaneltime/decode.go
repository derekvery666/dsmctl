package controlpaneltime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ychiu1211/dsmctl/internal/domain/controlpanel"
)

type timeResponse struct {
	TimeZone   *string `json:"timezone"`
	DateFormat *string `json:"date_format"`
	TimeFormat *string `json:"time_format"`
	Mode       *string `json:"enable_ntp"`
	Server     *string `json:"server"`
}

func decode(data json.RawMessage, requireFormats bool) (controlpanel.TimeState, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return controlpanel.TimeState{}, errors.New("decode time configuration: empty response")
	}
	if trimmed[0] != '{' {
		return controlpanel.TimeState{}, errors.New("decode time configuration: expected an object")
	}

	var response timeResponse
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return controlpanel.TimeState{}, fmt.Errorf("decode time configuration object: %w", err)
	}
	if raw == nil {
		return controlpanel.TimeState{}, errors.New("decode time configuration: expected an object")
	}
	if err := decodeRequiredString(raw, "timezone", &response.TimeZone); err != nil {
		return controlpanel.TimeState{}, err
	}
	if err := decodeRequiredString(raw, "enable_ntp", &response.Mode); err != nil {
		return controlpanel.TimeState{}, err
	}
	if err := decodeOptionalString(raw, "server", &response.Server); err != nil {
		return controlpanel.TimeState{}, err
	}
	if requireFormats {
		if err := decodeRequiredString(raw, "date_format", &response.DateFormat); err != nil {
			return controlpanel.TimeState{}, err
		}
		if err := decodeRequiredString(raw, "time_format", &response.TimeFormat); err != nil {
			return controlpanel.TimeState{}, err
		}
	} else {
		if err := decodeOptionalString(raw, "date_format", &response.DateFormat); err != nil {
			return controlpanel.TimeState{}, err
		}
		if err := decodeOptionalString(raw, "time_format", &response.TimeFormat); err != nil {
			return controlpanel.TimeState{}, err
		}
	}

	mode := controlpanel.TimeSynchronizationMode(strings.TrimSpace(*response.Mode))
	if mode != controlpanel.TimeSynchronizationManual && mode != controlpanel.TimeSynchronizationNTP {
		return controlpanel.TimeState{}, fmt.Errorf("decode time configuration: unsupported enable_ntp value %q", *response.Mode)
	}
	servers := splitServers(stringOrEmpty(response.Server))
	if mode == controlpanel.TimeSynchronizationNTP && len(servers) == 0 {
		return controlpanel.TimeState{}, errors.New("decode time configuration: NTP mode has no configured server")
	}

	return controlpanel.TimeState{
		TimeZone:            strings.TrimSpace(*response.TimeZone),
		DateFormat:          stringOrEmpty(response.DateFormat),
		TimeFormat:          stringOrEmpty(response.TimeFormat),
		SynchronizationMode: mode,
		NTPServers:          servers,
	}, nil
}

func decodeRequiredString(raw map[string]json.RawMessage, name string, destination **string) error {
	value, ok := raw[name]
	if !ok {
		return fmt.Errorf("decode time configuration: required field %q is missing", name)
	}
	var decoded string
	if err := json.Unmarshal(value, &decoded); err != nil {
		return fmt.Errorf("decode time configuration field %q: %w", name, err)
	}
	if strings.TrimSpace(decoded) == "" {
		return fmt.Errorf("decode time configuration: required field %q is empty", name)
	}
	*destination = &decoded
	return nil
}

func decodeOptionalString(raw map[string]json.RawMessage, name string, destination **string) error {
	value, ok := raw[name]
	if !ok || bytes.Equal(value, []byte("null")) {
		return nil
	}
	var decoded string
	if err := json.Unmarshal(value, &decoded); err != nil {
		return fmt.Errorf("decode time configuration field %q: %w", name, err)
	}
	*destination = &decoded
	return nil
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func splitServers(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	servers := make([]string, 0, len(parts))
	for _, part := range parts {
		if server := strings.TrimSpace(part); server != "" {
			servers = append(servers, server)
		}
	}
	return servers
}
