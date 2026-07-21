package config

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Config is the user-visible configuration shared by the CLI and MCP server.
// Passwords are deliberately not represented here.
type Config struct {
	DefaultNAS string             `json:"default_nas,omitempty"`
	NAS        map[string]Profile `json:"nas"`
}

// Profile describes how to connect to one NAS.
//
// A NAS is addressed by stable Identity (hardware serial/MAC, which do not
// change) plus an ordered Endpoints failover list, so a profile keeps working
// across LAN-IP, DDNS, and hostname changes. URL is the legacy single endpoint,
// kept mirrored to the top-priority direct endpoint so older binaries and
// external readers still connect. Identity and Endpoints are omitempty, so a
// pre-identity config file round-trips unchanged.
type Profile struct {
	URL                    string     `json:"url"`
	Identity               Identity   `json:"identity,omitempty"`
	Endpoints              []Endpoint `json:"endpoints,omitempty"`
	Username               string     `json:"username"`
	PasswordEnv            string     `json:"password_env,omitempty"`
	InsecureSkipTLSVerify  bool       `json:"insecure_skip_tls_verify,omitempty"`
	TLSMode                string     `json:"tls_mode,omitempty"`
	CertificateFingerprint string     `json:"certificate_fingerprint,omitempty"`
	TimeoutSeconds         int        `json:"timeout_seconds,omitempty"`
	// Role is "managed" (the default, absent value) or "target". A target-only
	// profile holds connection material and credentials so it can be used as an
	// outbound destination (a Snapshot Replication / Hyper Backup target), but it
	// is excluded from the management surfaces (read/apply tools, the managed NAS
	// count, token allowlists). omitempty keeps pre-role config files reading as
	// managed.
	Role string `json:"role,omitempty"`
	// Revision is supplied by a dynamic gateway repository. It is runtime
	// coordination metadata, not part of the portable CLI configuration file.
	Revision uint64 `json:"-"`
}

// Profile role values. A missing/empty Role is treated as managed.
const (
	ProfileRoleManaged = "managed"
	ProfileRoleTarget  = "target"
)

// Managed reports whether this profile participates in the management surfaces.
// Only an explicit "target" role opts out; every other value (including the
// empty default) is managed, so existing profiles keep their behavior.
func (p Profile) Managed() bool {
	return !strings.EqualFold(strings.TrimSpace(p.Role), ProfileRoleTarget)
}

// NormalizeRole canonicalizes a role string to "managed"/"target" (case- and
// whitespace-insensitive), returning an error for any other value. The managed
// default canonicalizes to the empty string so it stays omitempty on disk.
func NormalizeRole(role string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "", ProfileRoleManaged:
		return "", nil
	case ProfileRoleTarget:
		return ProfileRoleTarget, nil
	default:
		return "", fmt.Errorf("role must be %q or %q", ProfileRoleManaged, ProfileRoleTarget)
	}
}

// Identity is the stable hardware key for a NAS. Serial is the primary key;
// MACs let LAN re-discovery match a box whose findhost reply omitted the serial.
// Because it never changes, a profile keyed by Identity survives IP, DDNS, and
// hostname churn.
type Identity struct {
	Serial string   `json:"serial,omitempty"` // SYNO.Core.System serial — primary stable key
	MAC    string   `json:"mac,omitempty"`    // representative NIC MAC (lower, colon form)
	MACs   []string `json:"macs,omitempty"`   // all known NIC MACs, for discovery matching
	Model  string   `json:"model,omitempty"`  // advisory only
}

// Endpoint kinds.
const (
	EndpointLANIP        = "lan_ip"       // direct http(s) to a LAN address
	EndpointDDNS         = "ddns"         // Synology/other DDNS hostname
	EndpointHostname     = "hostname"     // manual DNS name / static host
	EndpointQuickConnect = "quickconnect" // QuickConnect alias (relay/direct)
)

// Endpoint is one way to reach a NAS. Lower Priority is tried first; ties fall
// back to slice order then LastOK (sticky last-good). Direct kinds carry a URL;
// a quickconnect endpoint carries a QuickConnectID instead.
type Endpoint struct {
	Kind           string    `json:"kind"`                      // one of the Endpoint* consts
	URL            string    `json:"url,omitempty"`             // scheme://host[:port] for direct kinds
	QuickConnectID string    `json:"quickconnect_id,omitempty"` // alias when Kind == quickconnect
	Priority       int       `json:"priority,omitempty"`        // lower first; default 0
	Source         string    `json:"source,omitempty"`          // "manual"|"discovered"|"ddns"|"quickconnect"
	LastOK         time.Time `json:"last_ok,omitempty"`         // last time this endpoint served a request
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
	Serial                 string `json:"serial,omitempty" jsonschema:"NAS hardware serial number"`
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
	// Migrate legacy single-URL profiles to the endpoint list so the runtime
	// resolver always has a candidate to try. Idempotent: only a profile with
	// no endpoints and a URL is synthesized.
	for name, profile := range c.NAS {
		if len(profile.Endpoints) == 0 && profile.URL != "" {
			profile.Endpoints = []Endpoint{{
				Kind:     classifyEndpoint(profile.URL),
				URL:      profile.URL,
				Priority: 0,
				Source:   "manual",
			}}
			c.NAS[name] = profile
		}
	}
}

// classifyEndpoint labels a legacy URL: a literal IP host is a LAN address,
// anything else is treated as a hostname. DDNS and QuickConnect kinds are only
// assigned when a profile is provisioned or discovered, never guessed here.
func classifyEndpoint(raw string) string {
	if parsed, err := url.ParseRequestURI(raw); err == nil && parsed.Host != "" {
		if net.ParseIP(parsed.Hostname()) != nil {
			return EndpointLANIP
		}
	}
	return EndpointHostname
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
		if err := validateProfileReach(name, profile); err != nil {
			return err
		}
		if profile.TimeoutSeconds < 0 {
			return fmt.Errorf("NAS profile %q: timeout_seconds cannot be negative", name)
		}
		if profile.InsecureSkipTLSVerify && profile.TLSMode != "" {
			return fmt.Errorf("NAS profile %q: insecure_skip_tls_verify and tls_mode cannot be combined", name)
		}
		if err := validateProfileTLS(name, profile); err != nil {
			return err
		}
	}
	return nil
}

// validateProfileReach enforces that a profile can be reached: either a valid
// legacy URL or at least one usable endpoint. Direct-kind endpoints need a valid
// absolute http/https URL; a quickconnect endpoint needs a QuickConnect ID.
func validateProfileReach(name string, profile Profile) error {
	usable := 0
	if profile.URL != "" {
		if err := validateDirectURL(profile.URL); err != nil {
			return fmt.Errorf("NAS profile %q: %w", name, err)
		}
		usable++
	}
	for i, ep := range profile.Endpoints {
		switch ep.Kind {
		case EndpointLANIP, EndpointDDNS, EndpointHostname:
			if err := validateDirectURL(ep.URL); err != nil {
				return fmt.Errorf("NAS profile %q endpoint %d (%s): %w", name, i, ep.Kind, err)
			}
			usable++
		case EndpointQuickConnect:
			if strings.TrimSpace(ep.QuickConnectID) == "" {
				return fmt.Errorf("NAS profile %q endpoint %d: quickconnect endpoint requires quickconnect_id", name, i)
			}
			usable++
		default:
			return fmt.Errorf("NAS profile %q endpoint %d: unsupported endpoint kind %q", name, i, ep.Kind)
		}
	}
	if usable == 0 {
		return fmt.Errorf("NAS profile %q: needs a url or at least one endpoint", name)
	}
	return nil
}

// validateProfileTLS validates the TLS mode and, for pinned_fingerprint, that
// every direct endpoint the profile can be reached at is https (a fingerprint
// cannot be pinned on a cleartext connection).
func validateProfileTLS(name string, profile Profile) error {
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
		for _, raw := range profile.directURLs() {
			parsed, err := url.ParseRequestURI(raw)
			if err != nil || parsed.Scheme != "https" {
				return fmt.Errorf("NAS profile %q: pinned_fingerprint TLS mode requires https endpoints", name)
			}
		}
	default:
		return fmt.Errorf("NAS profile %q: unsupported TLS mode %q", name, profile.TLSMode)
	}
	return nil
}

func validateDirectURL(raw string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("URL must be an absolute http or https URL")
	}
	return nil
}

// directURLs returns every http(s) URL this profile can be reached at: the
// legacy URL plus any direct-kind endpoints. QuickConnect endpoints are
// addressed by alias and are not included.
func (p Profile) directURLs() []string {
	var urls []string
	if p.URL != "" {
		urls = append(urls, p.URL)
	}
	for _, ep := range p.Endpoints {
		switch ep.Kind {
		case EndpointLANIP, EndpointDDNS, EndpointHostname:
			if ep.URL != "" {
				urls = append(urls, ep.URL)
			}
		}
	}
	return urls
}

func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	clone := &Config{DefaultNAS: c.DefaultNAS, NAS: make(map[string]Profile, len(c.NAS))}
	for name, profile := range c.NAS {
		clone.NAS[name] = profile.clone()
	}
	return clone
}

// clone deep-copies the slice fields so a cloned Config can be mutated without
// aliasing the original's Endpoints or Identity.MACs backing arrays.
func (p Profile) clone() Profile {
	cp := p
	if len(p.Endpoints) > 0 {
		cp.Endpoints = append([]Endpoint(nil), p.Endpoints...)
	}
	if len(p.Identity.MACs) > 0 {
		cp.Identity.MACs = append([]string(nil), p.Identity.MACs...)
	}
	return cp
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
		// With nothing configured, 'nas use' has nothing to select from; the
		// first profile has to be added before any NAS can be chosen.
		if len(c.NAS) == 0 {
			return "", Profile{}, errors.New("no NAS configured; add one with 'dsmctl nas add <name> --url https://nas.example.com:5001'")
		}
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
			Serial:                 profile.Identity.Serial,
			Revision:               profile.Revision,
		})
	}
	return result
}
