package config

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Config is the user-visible configuration shared by the CLI and MCP server.
// Passwords are deliberately not represented here.
type Config struct {
	DefaultNAS string             `json:"default_nas,omitempty"`
	NAS        map[string]Profile `json:"nas"`
}

// Profile describes how to connect to one NAS.
type Profile struct {
	URL                    string `json:"url"`
	Username               string `json:"username"`
	PasswordEnv            string `json:"password_env,omitempty"`
	InsecureSkipTLSVerify  bool   `json:"insecure_skip_tls_verify,omitempty"`
	TLSMode                string `json:"tls_mode,omitempty"`
	CertificateFingerprint string `json:"certificate_fingerprint,omitempty"`
	TimeoutSeconds         int    `json:"timeout_seconds,omitempty"`
	// Revision is supplied by a dynamic gateway repository. It is runtime
	// coordination metadata, not part of the portable CLI configuration file.
	Revision uint64 `json:"-"`
}

// Summary is safe to display. It never contains credential values.
type Summary struct {
	Name                   string `json:"name" jsonschema:"Configured NAS profile name"`
	URL                    string `json:"url" jsonschema:"DSM base URL"`
	Username               string `json:"username" jsonschema:"DSM account name"`
	PasswordEnv            string `json:"password_env" jsonschema:"Environment variable holding the password"`
	Default                bool   `json:"default" jsonschema:"Whether this is the default NAS"`
	InsecureSkipTLSVerify  bool   `json:"insecure_skip_tls_verify" jsonschema:"Whether TLS certificate verification is disabled"`
	TLSMode                string `json:"tls_mode,omitempty" jsonschema:"TLS verification policy"`
	CertificateFingerprint string `json:"certificate_fingerprint,omitempty" jsonschema:"Pinned SHA-256 server-certificate fingerprint"`
	Revision               uint64 `json:"revision,omitempty" jsonschema:"Persistent gateway profile revision"`
}

// Source provides immutable configuration snapshots. CLI processes use a
// StaticSource; the gateway state repository implements the same interface so
// committed profile changes become visible without restarting the process.
type Source interface {
	Snapshot(context.Context) (*Config, error)
}

type StaticSource struct {
	Config *Config
}

func (s StaticSource) Snapshot(ctx context.Context) (*Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.Config == nil {
		return nil, errors.New("config is nil")
	}
	return s.Config.Clone(), nil
}

func New() *Config {
	return &Config{NAS: make(map[string]Profile)}
}

func (c *Config) Normalize() {
	if c.NAS == nil {
		c.NAS = make(map[string]Profile)
	}
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}
	c.Normalize()

	if c.DefaultNAS != "" {
		if _, ok := c.NAS[c.DefaultNAS]; !ok {
			return fmt.Errorf("default NAS %q is not configured", c.DefaultNAS)
		}
	}

	for name, profile := range c.NAS {
		if err := ValidateName(name); err != nil {
			return fmt.Errorf("NAS profile %q: %w", name, err)
		}
		parsed, err := url.ParseRequestURI(profile.URL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("NAS profile %q: URL must be an absolute http or https URL", name)
		}
		if profile.TimeoutSeconds < 0 {
			return fmt.Errorf("NAS profile %q: timeout_seconds cannot be negative", name)
		}
		if profile.InsecureSkipTLSVerify && profile.TLSMode != "" {
			return fmt.Errorf("NAS profile %q: insecure_skip_tls_verify and tls_mode cannot be combined", name)
		}
		switch profile.TLSMode {
		case "", "system_ca":
			if profile.CertificateFingerprint != "" {
				return fmt.Errorf("NAS profile %q: certificate_fingerprint requires pinned_fingerprint TLS mode", name)
			}
		case "pinned_fingerprint":
			fingerprint := strings.ReplaceAll(strings.TrimSpace(profile.CertificateFingerprint), ":", "")
			if len(fingerprint) != 64 {
				return fmt.Errorf("NAS profile %q: certificate_fingerprint must be a SHA-256 fingerprint", name)
			}
			for _, character := range strings.ToLower(fingerprint) {
				if !strings.ContainsRune("0123456789abcdef", character) {
					return fmt.Errorf("NAS profile %q: certificate_fingerprint must contain only hexadecimal characters", name)
				}
			}
			if parsed.Scheme != "https" {
				return fmt.Errorf("NAS profile %q: pinned_fingerprint TLS mode requires an https URL", name)
			}
		default:
			return fmt.Errorf("NAS profile %q: unsupported TLS mode %q", name, profile.TLSMode)
		}
	}
	return nil
}

func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	clone := &Config{DefaultNAS: c.DefaultNAS, NAS: make(map[string]Profile, len(c.NAS))}
	for name, profile := range c.NAS {
		clone.NAS[name] = profile
	}
	return clone
}

func ValidateName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	for i, r := range name {
		valid := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.'
		if !valid || i == 0 && (r == '-' || r == '_' || r == '.') {
			return errors.New("name must start with a letter or number and contain only letters, numbers, '.', '_' or '-'")
		}
	}
	return nil
}

// Resolve selects an explicit profile, the configured default, or the sole
// configured profile in that order.
func (c *Config) Resolve(requested string) (string, Profile, error) {
	if c == nil {
		return "", Profile{}, errors.New("config is nil")
	}
	c.Normalize()
	name := requested
	if name == "" {
		name = c.DefaultNAS
	}
	if name == "" && len(c.NAS) == 1 {
		for only := range c.NAS {
			name = only
		}
	}
	if name == "" {
		return "", Profile{}, errors.New("no NAS selected; pass --nas or configure a default with 'dsmctl nas use <name>'")
	}
	profile, ok := c.NAS[name]
	if !ok {
		return "", Profile{}, fmt.Errorf("NAS profile %q is not configured", name)
	}
	return name, profile, nil
}

func (c *Config) Summaries(defaultPasswordEnv func(string) string) []Summary {
	if c == nil {
		return nil
	}
	names := make([]string, 0, len(c.NAS))
	for name := range c.NAS {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]Summary, 0, len(names))
	for _, name := range names {
		profile := c.NAS[name]
		passwordEnv := profile.PasswordEnv
		if passwordEnv == "" {
			passwordEnv = defaultPasswordEnv(name)
		}
		result = append(result, Summary{
			Name:                   name,
			URL:                    profile.URL,
			Username:               profile.Username,
			PasswordEnv:            passwordEnv,
			Default:                name == c.DefaultNAS,
			InsecureSkipTLSVerify:  profile.InsecureSkipTLSVerify,
			TLSMode:                profile.TLSMode,
			CertificateFingerprint: profile.CertificateFingerprint,
			Revision:               profile.Revision,
		})
	}
	return result
}
