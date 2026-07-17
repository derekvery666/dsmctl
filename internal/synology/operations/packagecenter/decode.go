package packagecenter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ychiu1211/dsmctl/internal/domain/packagecenter"
)

// decodePackages reads the installed-package list. The response envelope and its
// packages array are required, so a malformed shape errors instead of silently
// returning an empty inventory; individual per-package fields are optional
// because their presence varies across DSM releases.
func decodePackages(data json.RawMessage) ([]packagecenter.Package, error) {
	raw, err := decodeObject(data, "package list")
	if err != nil {
		return nil, err
	}
	rawList, ok := raw["packages"]
	if !ok {
		return nil, fmt.Errorf("decode package list: required field %q is missing", "packages")
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(rawList, &entries); err != nil {
		return nil, fmt.Errorf("decode package list field %q: %w", "packages", err)
	}
	packages := make([]packagecenter.Package, 0, len(entries))
	for index, entry := range entries {
		item, err := decodePackage(entry)
		if err != nil {
			return nil, fmt.Errorf("decode package %d: %w", index, err)
		}
		packages = append(packages, item)
	}
	return packages, nil
}

func decodePackage(data json.RawMessage) (packagecenter.Package, error) {
	raw, err := decodeObject(data, "package")
	if err != nil {
		return packagecenter.Package{}, err
	}
	additional := nestedObject(raw, "additional")
	id, ok := mergedString(raw, additional, "id")
	if !ok || strings.TrimSpace(id) == "" {
		return packagecenter.Package{}, fmt.Errorf("decode package: required field %q is missing", "id")
	}
	pkg := packagecenter.Package{ID: strings.TrimSpace(id)}
	if name, ok := mergedString(raw, additional, "name"); ok && strings.TrimSpace(name) != "" {
		pkg.Name = strings.TrimSpace(name)
	} else {
		pkg.Name = pkg.ID
	}
	if version, ok := mergedString(raw, additional, "version"); ok {
		pkg.Version = strings.TrimSpace(version)
	}
	if volume, ok := mergedString(raw, additional, "install_type"); ok && looksLikeVolume(volume) {
		pkg.Volume = volume
	}
	if volume, ok := mergedString(raw, additional, "volume"); ok && strings.TrimSpace(volume) != "" {
		pkg.Volume = strings.TrimSpace(volume)
	}
	if volume, ok := mergedString(raw, additional, "install_volume"); ok && strings.TrimSpace(volume) != "" {
		pkg.Volume = strings.TrimSpace(volume)
	}
	if beta, ok := mergedBool(raw, additional, "beta"); ok {
		pkg.Beta = beta
	}

	statusText, _ := mergedString(raw, additional, "status")
	installing, hasInstalling := mergedBool(raw, additional, "installing")
	pkg.Status = normalizeStatus(statusText, hasInstalling && installing)
	pkg.Running = pkg.Status == packagecenter.StatusRunning

	// DSM's `startable` marks a package that exposes a start/stop control (a
	// service package such as an app or daemon), not one that can be started
	// right now: a running service still reports startable=true, while runtimes
	// (Node.js, Python) and always-on system services report startable=false.
	// The actionable direction depends on the current run state. When the flag
	// is absent (older backends), fall back to the run state alone.
	if startable, ok := mergedBool(raw, additional, "startable"); ok {
		pkg.CanStart = startable && !pkg.Running
		pkg.CanStop = startable && pkg.Running
	} else {
		pkg.CanStart = !pkg.Running
		pkg.CanStop = pkg.Running
	}
	pkg.CanUninstall = decodeUninstallable(raw, additional)
	return pkg, nil
}

// decodeUninstallable prefers an explicit removable flag. Without one it falls
// back to the install type: DSM system packages are not user-removable. When no
// signal is present it defaults to true and lets DSM enforce removal at apply,
// rather than blocking every uninstall on releases that omit the flag.
func decodeUninstallable(raw, additional map[string]json.RawMessage) bool {
	if removable, ok := mergedBool(raw, additional, "removable"); ok {
		return removable
	}
	if installType, ok := mergedString(raw, additional, "install_type"); ok {
		if strings.EqualFold(strings.TrimSpace(installType), "system") {
			return false
		}
	}
	return true
}

// decodeSettings reads global Package Center settings. The response must be a
// non-empty object; every field is optional so a release that omits one yields
// documented defaults instead of a decode error.
func decodeSettings(data json.RawMessage) (packagecenter.Settings, error) {
	raw, err := decodeObject(data, "package settings")
	if err != nil {
		return packagecenter.Settings{}, err
	}
	settings := packagecenter.Settings{TrustLevel: packagecenter.TrustSynology}
	if level, ok := decodeTrustLevel(raw); ok {
		settings.TrustLevel = level
	}
	// `enable_autoupdate` is the master automatic-update toggle; `autoupdateall`
	// and `autoupdateimportant` distinguish all-vs-important when it is on. Older
	// backends without the master toggle fall back to `autoupdateall`.
	if enabled, ok := optionalBool(raw, "enable_autoupdate"); ok {
		settings.AutoUpdateEnabled = enabled
	} else if enabled, ok := optionalBool(raw, "autoupdateall"); ok {
		settings.AutoUpdateEnabled = enabled
	}
	if important, ok := optionalBool(raw, "autoupdateimportant"); ok {
		settings.AutoUpdateImportantOnly = important
	}
	return settings, nil
}

// decodeTrustLevel maps DSM's publisher-trust representation to the normalized
// TrustLevel. DSM uses an integer signature level (0 Synology, 1 Synology plus
// trusted, 2 any); a few releases expose a string instead.
func decodeTrustLevel(raw map[string]json.RawMessage) (packagecenter.TrustLevel, bool) {
	for _, name := range []string{"trust_level", "signature_level", "signature"} {
		value, ok := raw[name]
		if !ok {
			continue
		}
		if level, err := decodeInt(value); err == nil {
			switch level {
			case 0:
				return packagecenter.TrustSynology, true
			case 1:
				return packagecenter.TrustSynologyAndTrusted, true
			case 2:
				return packagecenter.TrustAny, true
			}
		}
		var text string
		if err := json.Unmarshal(value, &text); err == nil {
			switch strings.ToLower(strings.TrimSpace(text)) {
			case "synology", "syno":
				return packagecenter.TrustSynology, true
			case "synology_and_trusted", "trusted":
				return packagecenter.TrustSynologyAndTrusted, true
			case "any", "all":
				return packagecenter.TrustAny, true
			}
		}
	}
	return "", false
}

func normalizeStatus(text string, installing bool) packagecenter.Status {
	if installing {
		return packagecenter.StatusInstalling
	}
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "running", "run":
		return packagecenter.StatusRunning
	case "stop", "stopped", "nonactive", "non-active":
		return packagecenter.StatusStopped
	case "starting":
		return packagecenter.StatusStarting
	case "stopping":
		return packagecenter.StatusStopping
	case "installing", "downloading", "download", "install", "update", "updating":
		return packagecenter.StatusInstalling
	case "broken", "error", "start_failed", "stop_failed", "damaged":
		return packagecenter.StatusError
	case "":
		return packagecenter.StatusUnknown
	default:
		return packagecenter.StatusUnknown
	}
}

// looksLikeVolume keeps an install_type value only when it is actually a volume
// path. On many releases install_type carries a category string (e.g. "system")
// rather than a path; those are ignored so Volume stays empty instead of wrong.
func looksLikeVolume(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "/volume")
}

func decodeObject(data json.RawMessage, label string) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, fmt.Errorf("decode %s: expected a non-empty object", label)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return nil, fmt.Errorf("decode %s object: %w", label, err)
	}
	if raw == nil {
		return nil, fmt.Errorf("decode %s: expected an object", label)
	}
	return raw, nil
}

func nestedObject(raw map[string]json.RawMessage, name string) map[string]json.RawMessage {
	value, ok := raw[name]
	if !ok {
		return nil
	}
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil
	}
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &nested); err != nil {
		return nil
	}
	return nested
}

// mergedString reads a string field from the top-level object, then the nested
// additional object.
func mergedString(raw, additional map[string]json.RawMessage, name string) (string, bool) {
	if value, ok := optionalString(raw, name); ok {
		return value, true
	}
	if additional != nil {
		return optionalString(additional, name)
	}
	return "", false
}

func mergedBool(raw, additional map[string]json.RawMessage, name string) (bool, bool) {
	if value, ok := optionalBool(raw, name); ok {
		return value, true
	}
	if additional != nil {
		return optionalBool(additional, name)
	}
	return false, false
}

func optionalString(raw map[string]json.RawMessage, name string) (string, bool) {
	value, ok := raw[name]
	if !ok {
		return "", false
	}
	var result string
	if err := json.Unmarshal(value, &result); err != nil {
		return "", false
	}
	return result, true
}

func optionalBool(raw map[string]json.RawMessage, name string) (bool, bool) {
	value, ok := raw[name]
	if !ok {
		return false, false
	}
	var result bool
	if err := json.Unmarshal(value, &result); err == nil {
		return result, true
	}
	integer, err := decodeInt(value)
	if err != nil || (integer != 0 && integer != 1) {
		return false, false
	}
	return integer == 1, true
}

func decodeInt(value json.RawMessage) (int, error) {
	var integer int
	if err := json.Unmarshal(value, &integer); err == nil {
		return integer, nil
	}
	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		return 0, fmt.Errorf("expected integer")
	}
	integer, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return 0, fmt.Errorf("expected integer")
	}
	return integer, nil
}
