package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"

	"github.com/derekvery666/dsmctl/internal/observability"
	"github.com/derekvery666/dsmctl/internal/remotepolicy"
)

// buildLogger returns the diagnostic logger for the process, or nil when logging
// is disabled. The --log-level flag (opts.logLevel) wins over the
// DSMCTL_LOG_LEVEL environment variable; an empty or unrecognized value leaves
// logging off. Records go to stderr, never stdout.
func buildLogger(logLevel string) *slog.Logger {
	name := logLevel
	if name == "" {
		name = os.Getenv("DSMCTL_LOG_LEVEL")
	}
	level, ok := observability.ParseLevel(name)
	if !ok {
		return nil
	}
	return observability.New(os.Stderr, level)
}

// withCorrelationID returns ctx carrying a fresh short correlation id when it
// does not already have one, so all of a command's DSM calls share one id in the
// log. A rand failure leaves the context unchanged (the id is best-effort).
func withCorrelationID(ctx context.Context) context.Context {
	if remotepolicy.CorrelationID(ctx) != "" {
		return ctx
	}
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return ctx
	}
	return remotepolicy.WithCorrelationID(ctx, hex.EncodeToString(buf))
}
