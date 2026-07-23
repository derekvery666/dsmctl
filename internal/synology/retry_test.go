package synology

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/derekvery666/dsmctl/internal/synology/compatibility"
)

// scriptedTransport returns a programmed response (or transport error) per call,
// repeating the final step once the script is exhausted, and honors context
// cancellation like the real transport does.
type scriptedTransport struct {
	steps []func(*http.Request) (*http.Response, error)
	calls int
}

func (t *scriptedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if err := request.Context().Err(); err != nil {
		return nil, err
	}
	index := t.calls
	t.calls++
	if index >= len(t.steps) {
		index = len(t.steps) - 1
	}
	return t.steps[index](request)
}

func httpStep(status int, body string) func(*http.Request) (*http.Response, error) {
	return func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	}
}

func transportErrorStep(err error) func(*http.Request) (*http.Response, error) {
	return func(*http.Request) (*http.Response, error) { return nil, err }
}

func newScriptedClient(t *testing.T, baseURL string, transport *scriptedTransport) *Client {
	t.Helper()
	client, err := NewClient(Options{
		BaseURL:    baseURL,
		SessionID:  "seed-sid",
		HTTPClient: &http.Client{Transport: transport},
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func fastRetry(maxAttempts int) retryPolicy {
	return retryPolicy{MaxAttempts: maxAttempts, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, Budget: time.Hour}
}

func TestRequestLockedClassifiesHTTPFailures(t *testing.T) {
	cases := []struct {
		name string
		step func(*http.Request) (*http.Response, error)
		want Category
	}{
		{"service unavailable is transient", httpStep(http.StatusServiceUnavailable, ""), CategoryTransient},
		{"bad gateway is transient", httpStep(http.StatusBadGateway, ""), CategoryTransient},
		{"too many requests is rate-limit", httpStep(http.StatusTooManyRequests, ""), CategoryRateLimit},
		{"transport error is transient", transportErrorStep(errors.New("dial tcp: i/o timeout")), CategoryTransient},
		{"client status stays unclassified", httpStep(http.StatusNotFound, ""), CategoryUnknown},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			transport := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){test.step}}
			client := newScriptedClient(t, "https://nas.example", transport)
			_, err := client.requestLocked(context.Background(), "entry.cgi", url.Values{}, "SYNO.Test", "get")
			if err == nil {
				t.Fatalf("requestLocked() error = nil, want a failure")
			}
			if got := Classify(err); got != test.want {
				t.Fatalf("Classify(err) = %q, want %q (err = %v)", got, test.want, err)
			}
		})
	}
}

func TestRequestLockedTreatsContextCancellationAsUnknown(t *testing.T) {
	transport := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){
		httpStep(http.StatusServiceUnavailable, ""),
	}}
	client := newScriptedClient(t, "https://nas.example", transport)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.requestLocked(ctx, "entry.cgi", url.Values{}, "SYNO.Test", "get")
	if err == nil {
		t.Fatal("requestLocked() error = nil, want a cancellation failure")
	}
	if got := Classify(err); got != CategoryUnknown {
		t.Fatalf("Classify(cancelled) = %q, want unknown (a cancelled call must never look retryable)", got)
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		t.Fatalf("a cancelled request must not be typed as a transient HTTPError: %v", err)
	}
}

func TestReadOnlyRetrySucceedsAfterTransientFailures(t *testing.T) {
	transport := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){
		httpStep(http.StatusServiceUnavailable, ""),
		httpStep(http.StatusTooManyRequests, ""),
		transportErrorStep(errors.New("connection reset by peer")),
		httpStep(http.StatusOK, `{"success":true,"data":{"ok":true}}`),
	}}
	client := newScriptedClient(t, "https://nas.example", transport)
	client.retry = fastRetry(5)
	sleeps := 0
	client.sleep = func(ctx context.Context, _ time.Duration) error {
		sleeps++
		return ctx.Err()
	}

	data, err := client.requestWithRetryLocked(context.Background(), "entry.cgi", url.Values{}, "SYNO.Test", "get", true)
	if err != nil {
		t.Fatalf("requestWithRetryLocked() error = %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("data = %s", data)
	}
	if transport.calls != 4 {
		t.Fatalf("transport calls = %d, want 4 (three failures then a success)", transport.calls)
	}
	if sleeps != 3 {
		t.Fatalf("backoff sleeps = %d, want 3", sleeps)
	}
}

func TestMutatingCallIsNeverRetried(t *testing.T) {
	transport := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){
		httpStep(http.StatusServiceUnavailable, ""),
		httpStep(http.StatusOK, `{"success":true,"data":{}}`),
	}}
	client := newScriptedClient(t, "https://nas.example", transport)
	client.retry = fastRetry(5)
	client.sleep = func(context.Context, time.Duration) error {
		t.Fatal("a mutating call must not sleep for a retry")
		return nil
	}

	_, err := client.requestWithRetryLocked(context.Background(), "entry.cgi", url.Values{}, "SYNO.Test", "set", false)
	if Classify(err) != CategoryTransient {
		t.Fatalf("Classify(err) = %q, want transient (err = %v)", Classify(err), err)
	}
	if transport.calls != 1 {
		t.Fatalf("transport calls = %d, want 1; a mutation is issued exactly once", transport.calls)
	}
}

func TestReadOnlyRetryStopsOnNonRetryableError(t *testing.T) {
	// A DSM application error (permission) arrives as a 2xx envelope and must not
	// be retried even on a read-only call.
	transport := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){
		httpStep(http.StatusOK, `{"success":false,"error":{"code":105}}`),
		httpStep(http.StatusOK, `{"success":true,"data":{}}`),
	}}
	client := newScriptedClient(t, "https://nas.example", transport)
	client.retry = fastRetry(5)
	client.sleep = func(context.Context, time.Duration) error {
		t.Fatal("a permission error must not be retried")
		return nil
	}

	_, err := client.requestWithRetryLocked(context.Background(), "entry.cgi", url.Values{}, "SYNO.Test", "get", true)
	if Classify(err) != CategoryPermission {
		t.Fatalf("Classify(err) = %q, want permission", Classify(err))
	}
	if transport.calls != 1 {
		t.Fatalf("transport calls = %d, want 1", transport.calls)
	}
}

func TestRetryHonorsContextCancellationDuringBackoff(t *testing.T) {
	transport := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){
		httpStep(http.StatusServiceUnavailable, ""),
	}}
	client := newScriptedClient(t, "https://nas.example", transport)
	client.retry = fastRetry(5)
	client.sleep = func(context.Context, time.Duration) error { return context.Canceled }

	_, err := client.requestWithRetryLocked(context.Background(), "entry.cgi", url.Values{}, "SYNO.Test", "get", true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if transport.calls != 1 {
		t.Fatalf("transport calls = %d, want 1; cancellation must abort promptly", transport.calls)
	}
}

func TestRetryStopsWhenBudgetExhausted(t *testing.T) {
	transport := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){
		httpStep(http.StatusServiceUnavailable, ""),
	}}
	client := newScriptedClient(t, "https://nas.example", transport)
	client.retry = retryPolicy{MaxAttempts: 5, BaseDelay: time.Second, MaxDelay: time.Second, Budget: 0}
	client.sleep = func(context.Context, time.Duration) error {
		t.Fatal("an exhausted budget must not sleep")
		return nil
	}

	_, err := client.requestWithRetryLocked(context.Background(), "entry.cgi", url.Values{}, "SYNO.Test", "get", true)
	if Classify(err) != CategoryTransient {
		t.Fatalf("Classify(err) = %q, want transient", Classify(err))
	}
	if transport.calls != 1 {
		t.Fatalf("transport calls = %d, want 1; the budget was already spent", transport.calls)
	}
}

func TestHTTPErrorRedactsCredentialsAndCarriesNoSecret(t *testing.T) {
	transport := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){
		httpStep(http.StatusServiceUnavailable, ""),
	}}
	client := newScriptedClient(t, "https://admin:s3cr3tpass@nas.example", transport)

	_, err := client.requestLocked(context.Background(), "entry.cgi", url.Values{"_sid": {"leaky-sid"}, "SynoToken": {"leaky-token"}}, "SYNO.Test", "get")
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("err = %v, want *HTTPError", err)
	}
	if httpErr.Category() != CategoryTransient {
		t.Fatalf("category = %q, want transient", httpErr.Category())
	}
	for _, secret := range []string{"s3cr3tpass", "leaky-sid", "leaky-token", "_sid", "SynoToken"} {
		if strings.Contains(httpErr.Error(), secret) {
			t.Fatalf("HTTPError message %q leaked %q", httpErr.Error(), secret)
		}
	}
}

func TestExecuteLockedThreadsReadOnlyFlagIntoRetry(t *testing.T) {
	newClient := func(t *testing.T, transport *scriptedTransport) *Client {
		t.Helper()
		client := newScriptedClient(t, "https://nas.example", transport)
		client.retry = fastRetry(5)
		client.sleep = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
		client.target.SetAPI("SYNO.Core.System", APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 3})
		client.apiChecked["SYNO.Core.System"] = true
		return client
	}
	call := compatibility.Request{API: "SYNO.Core.System", Version: 3, Method: "info"}

	t.Run("read-only call retries", func(t *testing.T) {
		transport := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){
			httpStep(http.StatusServiceUnavailable, ""),
			httpStep(http.StatusServiceUnavailable, ""),
			httpStep(http.StatusOK, `{"success":true,"data":{"model":"DS923+"}}`),
		}}
		client := newClient(t, transport)
		readOnly := call
		readOnly.ReadOnly = true
		client.mu.Lock()
		_, err := client.executeLocked(context.Background(), readOnly)
		client.mu.Unlock()
		if err != nil {
			t.Fatalf("executeLocked() error = %v", err)
		}
		if transport.calls != 3 {
			t.Fatalf("transport calls = %d, want 3", transport.calls)
		}
	})

	t.Run("mutating call does not retry", func(t *testing.T) {
		transport := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){
			httpStep(http.StatusServiceUnavailable, ""),
			httpStep(http.StatusOK, `{"success":true,"data":{}}`),
		}}
		client := newClient(t, transport)
		client.mu.Lock()
		_, err := client.executeLocked(context.Background(), call) // ReadOnly defaults to false
		client.mu.Unlock()
		if Classify(err) != CategoryTransient {
			t.Fatalf("Classify(err) = %q, want transient", Classify(err))
		}
		if transport.calls != 1 {
			t.Fatalf("transport calls = %d, want 1; a write is never auto-retried", transport.calls)
		}
	})
}
