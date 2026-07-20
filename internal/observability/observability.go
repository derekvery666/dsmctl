// Package observability builds the opt-in, leveled diagnostic logger for the
// CLI and stdio MCP server. Logging is silent by default; a caller enables it
// with a level. Every logger carries a redaction guarantee: attribute keys that
// name authentication material are replaced with a placeholder so a SID,
// SynoToken, password, OTP, or device id can never reach a log record.
package observability

import (
	"io"
	"log/slog"
	"strings"
)

// Redacted is the placeholder written in place of a secret-bearing attribute.
const Redacted = "[redacted]"

// secretKeys are the attribute keys whose values are never logged. New
// secret-bearing parameters added elsewhere must be added here so the single
// denylist stays authoritative. Matching is case-insensitive.
var secretKeys = map[string]struct{}{
	"passwd":       {},
	"password":     {},
	"otp_code":     {},
	"otp":          {},
	"_sid":         {},
	"sid":          {},
	"synotoken":    {},
	"x-syno-token": {},
	"device_id":    {},
	"passphrase":   {},
	"key":          {},
	"private_key":  {},
	"recovery":     {},
}

// IsSecretKey reports whether an attribute key names authentication material.
func IsSecretKey(key string) bool {
	_, ok := secretKeys[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

// redactAttr is the slog ReplaceAttr hook: a denylisted key's value becomes the
// placeholder regardless of type, so no secret is ever formatted into output.
func redactAttr(_ []string, attr slog.Attr) slog.Attr {
	if IsSecretKey(attr.Key) {
		return slog.String(attr.Key, Redacted)
	}
	return attr
}

// New returns a leveled text logger writing to w (stderr in production) with the
// redaction hook installed. It never writes to stdout, so it is safe alongside
// the stdio MCP server's JSON-RPC on stdout.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redactAttr,
	}))
}

// ParseLevel maps a level name to a slog.Level. An empty or unrecognized value
// returns ok=false so the caller can leave logging disabled.
func ParseLevel(name string) (slog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return 0, false
	}
}
