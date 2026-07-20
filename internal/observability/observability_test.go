package observability

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]struct {
		level slog.Level
		ok    bool
	}{
		"debug": {slog.LevelDebug, true},
		"INFO":  {slog.LevelInfo, true},
		"warn":  {slog.LevelWarn, true},
		"warning": {slog.LevelWarn, true},
		"error": {slog.LevelError, true},
		"":      {0, false},
		"loud":  {0, false},
	}
	for name, want := range cases {
		level, ok := ParseLevel(name)
		if ok != want.ok || (ok && level != want.level) {
			t.Errorf("ParseLevel(%q) = (%v,%v), want (%v,%v)", name, level, ok, want.level, want.ok)
		}
	}
}

func TestRedactionDenylist(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelDebug)
	logger.LogAttrs(context.Background(), slog.LevelDebug, "dsm request",
		slog.String("api", "SYNO.API.Auth"),
		slog.String("passwd", "hunter2"),
		slog.String("otp_code", "123456"),
		slog.String("_sid", "SECRET-SID"),
		slog.String("SynoToken", "SECRET-TOKEN"),
		slog.String("device_id", "SECRET-DEVICE"),
		slog.String("key", "SECRET-KEY"),
	)
	out := buf.String()
	for _, secret := range []string{"hunter2", "123456", "SECRET-SID", "SECRET-TOKEN", "SECRET-DEVICE", "SECRET-KEY"} {
		if strings.Contains(out, secret) {
			t.Fatalf("log output leaked %q: %s", secret, out)
		}
	}
	// Redaction placeholder present for each denylisted key; non-secret kept.
	if strings.Count(out, Redacted) < 6 {
		t.Fatalf("expected 6 redactions, got: %s", out)
	}
	if !strings.Contains(out, "SYNO.API.Auth") {
		t.Fatalf("non-secret attribute dropped: %s", out)
	}
}

func TestRedactionIsCaseInsensitiveAndTypeAgnostic(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelDebug)
	logger.LogAttrs(context.Background(), slog.LevelInfo, "m",
		slog.Bool("Password", true),      // non-string secret value
		slog.String("X-SYNO-TOKEN", "T"), // header-style key
	)
	out := buf.String()
	if strings.Contains(out, "=T") || strings.Contains(out, "=true") {
		t.Fatalf("secret value survived: %s", out)
	}
}

func TestLevelGating(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)
	logger.Debug("suppressed")
	if buf.Len() != 0 {
		t.Fatalf("debug record emitted at info level: %s", buf.String())
	}
	logger.Info("kept")
	if !strings.Contains(buf.String(), "kept") {
		t.Fatalf("info record missing: %s", buf.String())
	}
}

func TestIsSecretKey(t *testing.T) {
	for _, key := range []string{"passwd", "PASSWORD", " _sid ", "SynoToken", "device_id"} {
		if !IsSecretKey(key) {
			t.Errorf("IsSecretKey(%q) = false, want true", key)
		}
	}
	for _, key := range []string{"api", "method", "version", "path", "correlation_id"} {
		if IsSecretKey(key) {
			t.Errorf("IsSecretKey(%q) = true, want false", key)
		}
	}
}
