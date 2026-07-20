package cli

import (
	"context"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/remotepolicy"
)

func TestBuildLoggerFlagWinsOverEnv(t *testing.T) {
	t.Setenv("DSMCTL_LOG_LEVEL", "error")
	// The flag value takes precedence over the env var.
	if logger := buildLogger("debug"); logger == nil {
		t.Fatal("buildLogger(\"debug\") returned nil despite a valid flag")
	}
	// With no flag, the env var is used.
	if logger := buildLogger(""); logger == nil {
		t.Fatal("buildLogger with DSMCTL_LOG_LEVEL=error returned nil")
	}
}

func TestBuildLoggerDisabledByDefault(t *testing.T) {
	t.Setenv("DSMCTL_LOG_LEVEL", "")
	if logger := buildLogger(""); logger != nil {
		t.Fatal("buildLogger with no flag and no env should be nil (logging off)")
	}
	if logger := buildLogger("nonsense"); logger != nil {
		t.Fatal("buildLogger with an unrecognized level should be nil")
	}
}

func TestWithCorrelationIDGeneratesAndPreserves(t *testing.T) {
	ctx := withCorrelationID(context.Background())
	id := remotepolicy.CorrelationID(ctx)
	if id == "" {
		t.Fatal("withCorrelationID did not stamp an id")
	}
	// A context that already has one keeps it (idempotent within a command).
	if again := remotepolicy.CorrelationID(withCorrelationID(ctx)); again != id {
		t.Fatalf("withCorrelationID replaced an existing id: %q -> %q", id, again)
	}
}
