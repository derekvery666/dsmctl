package synology

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/derekvery666/dsmctl/internal/observability"
	"github.com/derekvery666/dsmctl/internal/remotepolicy"
)

func loggingTestClient(t *testing.T, server *httptest.Server, logger *slog.Logger) *Client {
	t.Helper()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return &Client{baseURL: parsed, httpClient: server.Client(), logger: logger}
}

func TestRequestLoggedRecordShapeAndCorrelationID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{}}`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	client := loggingTestClient(t, server, observability.New(&buf, slog.LevelDebug))
	ctx := remotepolicy.WithCorrelationID(context.Background(), "corr-123")
	// passwd is in the params but requestLocked logs metadata only, never a
	// parameter value — so no secret can reach the record.
	params := url.Values{"version": {"2"}, "passwd": {"hunter2"}}
	if _, err := client.requestLocked(ctx, "entry.cgi", params, "SYNO.Test", "get"); err != nil {
		t.Fatalf("requestLocked() error = %v", err)
	}
	out := buf.String()
	for _, want := range []string{"api=SYNO.Test", "method=get", "version=2", "http_status=200", "correlation_id=corr-123", "duration_ms="} {
		if !strings.Contains(out, want) {
			t.Errorf("log record missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "hunter2") {
		t.Fatalf("log record leaked a parameter value: %s", out)
	}
}

func TestRequestNilLoggerIsSilentAndSafe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{}}`))
	}))
	defer server.Close()
	client := loggingTestClient(t, server, nil) // logging disabled
	if _, err := client.requestLocked(context.Background(), "entry.cgi", url.Values{"version": {"1"}}, "SYNO.Test", "get"); err != nil {
		t.Fatalf("requestLocked() with nil logger error = %v", err)
	}
}
