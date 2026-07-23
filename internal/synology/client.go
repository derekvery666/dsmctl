package synology

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/derekvery666/dsmctl/internal/remotepolicy"
	"github.com/derekvery666/dsmctl/internal/synology/compatibility"
)

const (
	authAPI        = "SYNO.API.Auth"
	maxBodySize    = 8 << 20
	maxOTPAttempts = 3
	dsmctlSession  = "DSMCTL"
)

type OTPProvider func(ctx context.Context) (string, error)

type DeviceIDSaver func(ctx context.Context, deviceID string) error

type Options struct {
	BaseURL      string
	Username     string
	Password     string
	DeviceName   string
	DeviceID     string
	OTPProvider  OTPProvider
	SaveDeviceID DeviceIDSaver
	HTTPClient   *http.Client

	// SessionID and SynoToken seed the client with a session obtained elsewhere
	// (for example a web login). When SessionID is set the client reuses it
	// instead of logging in, so a password is not required.
	SessionID string
	SynoToken string

	// Resume, when set, renews an expired injected session without a password.
	// It is tried instead of the password path when a session error occurs.
	// The runtime wires it for clients seeded from the persisted web-login
	// session: it re-reads the store (picking up a session renewed by another
	// process's 'auth login' — essential for long-running processes such as
	// the MCP server, whose cached client would otherwise stay dead forever)
	// and then attempts a browserless Noise resume with the stored renewal
	// keys. It returns the new session ID and SynoToken.
	Resume func(ctx context.Context) (sid, synoToken string, err error)

	// SaveSession, when set, persists a session established by the password
	// fallback so later processes reuse it instead of logging in again. It runs
	// after a successful login (not resume, which persists itself) and is
	// best-effort: a persistence failure never fails the login.
	SaveSession func(ctx context.Context, sid, synoToken string) error

	// PasswordFunc resolves the account password lazily, only when a login is
	// actually needed. A seeded client uses it as a fallback when a session can
	// no longer be resumed, so reusing a live session never resolves a password.
	PasswordFunc func(ctx context.Context) (string, error)

	// Logger, when non-nil, receives one debug record per DSM call (api,
	// method, version, path, HTTP status, duration, and the request's
	// correlation id). It must already have the redaction hook installed; the
	// request path never logs a secret parameter regardless. A nil Logger
	// disables logging with no added work on the hot path.
	Logger *slog.Logger

	// PreserveSessionOnClose keeps the DSM session alive when the client is
	// closed: Close drops the in-memory session instead of calling
	// SYNO.API.Auth logout. Set it for clients seeded from a persisted
	// web-login session, which must outlive this process so later commands can
	// reuse it; revoking it is then an explicit action (Logout), not a side
	// effect of process exit. Leave it unset for password logins, whose
	// sessions belong to this process alone.
	PreserveSessionOnClose bool
}

type APIInfo = compatibility.APIInfo

type Client struct {
	baseURL         *url.URL
	username        string
	password        string
	deviceName      string
	deviceID        string
	otp             OTPProvider
	saveDeviceID    DeviceIDSaver
	resume          func(ctx context.Context) (string, string, error)
	saveSession     func(ctx context.Context, sid, synoToken string) error
	passwordFunc    func(ctx context.Context) (string, error)
	httpClient      *http.Client
	preserveSession bool
	logger          *slog.Logger

	retry retryPolicy
	// sleep waits for d honoring ctx cancellation. It is a field so tests can
	// inject a deterministic, non-blocking sleeper instead of the wall clock.
	sleep func(ctx context.Context, d time.Duration) error

	mu         sync.Mutex
	target     compatibility.Target
	apiChecked map[string]bool
	sid        string
	synoToken  string
}

// retryPolicy bounds automatic retry of a read-only call whose failure
// classifies as transient or rate-limit: at most MaxAttempts tries (including
// the first), with jittered exponential backoff from BaseDelay, each capped at
// MaxDelay, and no more than Budget spent sleeping in total.
type retryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Budget      time.Duration
}

// defaultRetryPolicy is applied to read-only calls. Mutating calls ignore it
// (they pass readOnly=false and run exactly once).
func defaultRetryPolicy() retryPolicy {
	return retryPolicy{
		MaxAttempts: 3,
		BaseDelay:   200 * time.Millisecond,
		MaxDelay:    2 * time.Second,
		Budget:      8 * time.Second,
	}
}

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code int `json:"code"`
	} `json:"error,omitempty"`
}

func NewClient(options Options) (*Client, error) {
	baseURL, err := url.Parse(options.BaseURL)
	if err != nil || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return nil, errors.New("base URL must be an absolute http or https URL")
	}
	if options.SessionID == "" && strings.TrimSpace(options.Username) == "" {
		return nil, errors.New("username is required")
	}
	if options.Password == "" && options.SessionID == "" {
		return nil, errors.New("a password or an existing session is required")
	}
	baseURL.RawQuery = ""
	baseURL.Fragment = ""
	baseURL.Path = strings.TrimRight(baseURL.Path, "/")

	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:         baseURL,
		username:        options.Username,
		password:        options.Password,
		deviceName:      options.DeviceName,
		deviceID:        options.DeviceID,
		otp:             options.OTPProvider,
		saveDeviceID:    options.SaveDeviceID,
		resume:          options.Resume,
		saveSession:     options.SaveSession,
		passwordFunc:    options.PasswordFunc,
		httpClient:      httpClient,
		preserveSession: options.PreserveSessionOnClose,
		logger:          options.Logger,
		retry:           defaultRetryPolicy(),
		sleep:           sleepWithContext,
		target:          compatibility.NewTarget(),
		apiChecked:      make(map[string]bool),
		sid:             options.SessionID,
		synoToken:       options.SynoToken,
	}, nil
}

func (c *Client) discoverAPIsLocked(ctx context.Context, names ...string) error {
	missing := make([]string, 0, len(names))
	for _, name := range names {
		if !c.apiChecked[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	params := url.Values{
		"api":     {"SYNO.API.Info"},
		"version": {"1"},
		"method":  {"query"},
		"query":   {strings.Join(missing, ",")},
	}
	data, err := c.requestLocked(ctx, "entry.cgi", params, "SYNO.API.Info", "query")
	if err != nil {
		return fmt.Errorf("discover Synology APIs: %w", err)
	}
	var discovered map[string]APIInfo
	if err := json.Unmarshal(data, &discovered); err != nil {
		return fmt.Errorf("decode Synology API discovery: %w", err)
	}
	for _, name := range missing {
		c.apiChecked[name] = true
		info, ok := discovered[name]
		if !ok || info.Path == "" || info.MaxVersion == 0 {
			continue
		}
		c.target.SetAPI(name, info)
	}
	c.updateDerivedCapabilitiesLocked()
	return nil
}

func (c *Client) ensureAPIsLocked(ctx context.Context, names ...string) error {
	if err := c.discoverAPIsLocked(ctx, names...); err != nil {
		return err
	}
	for _, name := range names {
		if _, ok := c.target.API(name); !ok {
			return fmt.Errorf("Synology API %s is not available on this NAS", name)
		}
	}
	return nil
}

func (c *Client) loginLocked(ctx context.Context) error {
	if c.sid != "" {
		return nil
	}
	if c.password == "" && c.passwordFunc != nil {
		// Resolve the password lazily, only now that a login is unavoidable, so
		// reusing a live session never touches credentials.
		if resolved, err := c.passwordFunc(ctx); err == nil {
			c.password = resolved
		}
	}
	if c.password == "" {
		return errors.New("DSM session is unavailable and no password is configured to re-authenticate; run 'dsmctl auth login' to sign in again")
	}
	if err := c.ensureAPIsLocked(ctx, authAPI); err != nil {
		return err
	}
	info, _ := c.target.API(authAPI)
	// DSM 7.3 grants privileged control-plane mutations to Auth v7 sessions.
	// preferredVersion keeps older DSM releases on their highest advertised
	// version instead of duplicating the login implementation per release.
	version := preferredVersion(info, 7)
	params := url.Values{
		"api":     {authAPI},
		"version": {strconv.Itoa(version)},
		"method":  {"login"},
		"account": {c.username},
		"passwd":  {c.password},
		"session": {dsmctlSession},
		"format":  {"sid"},
	}
	if version >= 6 {
		params.Set("enable_syno_token", "yes")
		if c.deviceID != "" && c.deviceName != "" {
			params.Set("device_name", c.deviceName)
			params.Set("device_id", c.deviceID)
		}
	}

	data, err := c.requestLocked(ctx, info.Path, params, authAPI, "login")
	if isOTPChallenge(err) {
		data, err = c.loginWithOTPLocked(ctx, info.Path, version, params, err)
	}
	if err != nil {
		return fmt.Errorf("log in to DSM: %w", err)
	}
	return c.acceptLoginLocked(ctx, data)
}

func (c *Client) loginWithOTPLocked(ctx context.Context, path string, version int, base url.Values, challenge error) (json.RawMessage, error) {
	if c.otp == nil {
		return nil, &OTPRequiredError{Cause: challenge}
	}
	var lastErr error
	for attempt := 0; attempt < maxOTPAttempts; attempt++ {
		code, err := c.otp(ctx)
		if err != nil {
			return nil, fmt.Errorf("obtain one-time password: %w", err)
		}
		code = strings.TrimSpace(code)
		if code == "" {
			return nil, errors.New("one-time password cannot be empty")
		}
		params := cloneValues(base)
		params.Del("device_id")
		params.Set("otp_code", code)
		if version >= 6 && c.deviceName != "" {
			params.Set("enable_device_token", "yes")
			params.Set("device_name", c.deviceName)
		}
		data, requestErr := c.requestLocked(ctx, path, params, authAPI, "login")
		if requestErr == nil {
			return data, nil
		}
		lastErr = requestErr
		if !isInvalidOTP(requestErr) {
			return nil, requestErr
		}
	}
	return nil, fmt.Errorf("DSM rejected the one-time password after %d attempts: %w", maxOTPAttempts, lastErr)
}

func (c *Client) acceptLoginLocked(ctx context.Context, data json.RawMessage) error {
	var result struct {
		SID        string `json:"sid"`
		SynoToken  string `json:"synotoken"`
		DID        string `json:"did"`
		DeviceIDV7 string `json:"device_id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("decode DSM login: %w", err)
	}
	if result.SID == "" {
		return errors.New("DSM login response did not contain a session ID")
	}
	c.sid = result.SID
	c.synoToken = result.SynoToken
	deviceID := result.DID
	if deviceID == "" {
		deviceID = result.DeviceIDV7
	}
	if deviceID != "" {
		c.deviceID = deviceID
		if c.saveDeviceID != nil {
			if err := c.saveDeviceID(ctx, deviceID); err != nil {
				return fmt.Errorf("save DSM trusted device: %w", err)
			}
		}
	}
	if c.saveSession != nil {
		// Best-effort: persist the freshly logged-in session so later processes
		// reuse it. A persistence failure must not fail an otherwise good login.
		_ = c.saveSession(ctx, c.sid, c.synoToken)
	}
	return nil
}

// Authenticate establishes a DSM session without calling a management API.
func (c *Client) Authenticate(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loginLocked(ctx)
}

// reestablishLocked recovers a lost session. An injected session with a resume
// function is refreshed without a password first; otherwise it falls back to
// the password login path (which errors when no password is configured).
func (c *Client) reestablishLocked(ctx context.Context) error {
	if c.resume != nil {
		sid, synoToken, err := c.resume(ctx)
		if err == nil {
			c.sid = sid
			c.synoToken = synoToken
			return nil
		}
		// The stored session could not be resumed (DSM evicted it after its
		// idle timeout, so the server refuses a Noise resume). If a password is
		// available, recover automatically with a fresh login instead of forcing
		// an interactive sign-in; otherwise report the resume failure.
		if c.password == "" && c.passwordFunc == nil {
			return err
		}
	}
	return c.loginLocked(ctx)
}

// ValidateSession reports whether the session currently held by the client is
// still accepted by DSM. It issues one cheap authenticated request and, unlike
// the normal request path, never tries to re-authenticate: an expired or
// rejected session is reported as (false, nil), a missing session as
// (false, nil), and only transport or unexpected API failures return an error.
// It is the authoritative, online counterpart to HasSession.
func (c *Client) ValidateSession(ctx context.Context) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sid == "" {
		return false, nil
	}
	if err := c.probeSessionLocked(ctx); err != nil {
		if isSessionError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// probeSessionLocked issues a single authenticated read that any DSM release
// exposes, without the session-retry that executeLocked performs, so callers
// can observe a session failure instead of silently re-authenticating.
func (c *Client) probeSessionLocked(ctx context.Context) error {
	const probeAPI = "SYNO.Core.System"
	if err := c.ensureAPIsLocked(ctx, probeAPI); err != nil {
		return err
	}
	info, _ := c.target.API(probeAPI)
	params := url.Values{
		"api":     {probeAPI},
		"version": {strconv.Itoa(info.MaxVersion)},
		"method":  {"info"},
		"_sid":    {c.sid},
	}
	if c.synoToken != "" {
		params.Set("SynoToken", c.synoToken)
	}
	_, err := c.requestLocked(ctx, info.Path, params, probeAPI, "info")
	return err
}

func (c *Client) executeLocked(ctx context.Context, call compatibility.Request) (json.RawMessage, error) {
	if err := c.ensureAPIsLocked(ctx, call.API); err != nil {
		return nil, err
	}
	if err := c.loginLocked(ctx); err != nil {
		return nil, err
	}
	info, _ := c.target.API(call.API)
	version := call.Version
	if version == 0 {
		version = info.MaxVersion
	}
	if !info.Supports(version) {
		return nil, fmt.Errorf("Synology API %s does not support requested version %d (available %d-%d)", call.API, version, info.MinVersion, info.MaxVersion)
	}
	params := cloneValues(call.Parameters)
	var err error
	if call.JSONParameters != nil {
		params, err = c.encodeJSONParametersLocked(ctx, call.JSONParameters, call.EncryptedParameters)
		if err != nil {
			return nil, fmt.Errorf("prepare JSON parameters for %s.%s: %w", call.API, call.Method, err)
		}
	} else if len(call.EncryptedParameters) != 0 {
		return nil, fmt.Errorf("encrypted parameters require typed JSON parameters")
	}
	params.Set("api", call.API)
	params.Set("version", strconv.Itoa(version))
	params.Set("method", call.Method)
	params.Set("_sid", c.sid)
	if c.synoToken != "" {
		params.Set("SynoToken", c.synoToken)
	}

	data, err := c.requestWithRetryLocked(ctx, info.Path, params, call.API, call.Method, call.ReadOnly)
	if isSessionError(err) {
		c.sid = ""
		c.synoToken = ""
		if reErr := c.reestablishLocked(ctx); reErr != nil {
			// DSM rejected the session and it could not be renewed
			// automatically; report a typed, detectable "session ended" error
			// so the CLI and MCP can tell the user to sign in again.
			if IsSessionExpired(reErr) {
				return nil, reErr
			}
			return nil, &SessionExpiredError{Cause: reErr}
		}
		params.Set("_sid", c.sid)
		params.Del("SynoToken")
		if c.synoToken != "" {
			params.Set("SynoToken", c.synoToken)
		}
		return c.requestWithRetryLocked(ctx, info.Path, params, call.API, call.Method, call.ReadOnly)
	}
	return data, err
}

func (c *Client) executeScriptLocked(ctx context.Context, call compatibility.Request) ([]byte, error) {
	if err := c.ensureAPIsLocked(ctx, call.API); err != nil {
		return nil, err
	}
	if err := c.loginLocked(ctx); err != nil {
		return nil, err
	}
	info, _ := c.target.API(call.API)
	version := call.Version
	if version == 0 {
		version = info.MaxVersion
	}
	if !info.Supports(version) {
		return nil, fmt.Errorf("Synology API %s does not support requested version %d (available %d-%d)", call.API, version, info.MinVersion, info.MaxVersion)
	}
	params := cloneValues(call.Parameters)
	params.Set("api", call.API)
	params.Set("version", strconv.Itoa(version))
	params.Set("method", call.Method)
	return c.requestScriptLocked(ctx, info.Path, params, call.API)
}

func (c *Client) requestScriptLocked(ctx context.Context, apiPath string, params url.Values, api string) ([]byte, error) {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/webapi/" + strings.TrimLeft(apiPath, "/")
	endpoint.RawQuery = params.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/javascript, text/javascript, */*;q=0.1")
	request.Header.Set("User-Agent", "dsmctl/0.1")
	if c.sid != "" {
		request.AddCookie(&http.Cookie{Name: "id", Value: c.sid})
	}
	if c.synoToken != "" {
		request.Header.Set("X-SYNO-TOKEN", c.synoToken)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", endpoint.Redacted(), err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("request %s returned HTTP %s", endpoint.Redacted(), response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodySize+1))
	if err != nil {
		return nil, fmt.Errorf("read %s script response: %w", api, err)
	}
	if len(body) > maxBodySize {
		return nil, fmt.Errorf("%s script response exceeds %d bytes", api, maxBodySize)
	}
	return body, nil
}

// requestFileLocked POSTs form-encoded params and returns the raw response
// body. It is for WebAPI methods that answer a file download (DSM's
// RESPONSE_FILE) rather than the JSON envelope. When the body is a JSON error
// envelope (some handlers still emit one), it is surfaced as an APIError.
func (c *Client) requestFileLocked(ctx context.Context, apiPath string, params url.Values, api, method string) ([]byte, error) {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/webapi/" + strings.TrimLeft(apiPath, "/")

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewBufferString(params.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "dsmctl/0.1")
	if c.sid != "" {
		request.AddCookie(&http.Cookie{Name: "id", Value: c.sid})
	}
	if c.synoToken != "" {
		request.Header.Set("X-SYNO-TOKEN", c.synoToken)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", endpoint.Redacted(), err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("request %s returned HTTP %s", endpoint.Redacted(), response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodySize+1))
	if err != nil {
		return nil, fmt.Errorf("read %s file response: %w", api, err)
	}
	if len(body) > maxBodySize {
		return nil, fmt.Errorf("%s file response exceeds %d bytes", api, maxBodySize)
	}
	// A JSON error envelope means the download did not happen.
	if trimmed := bytes.TrimSpace(body); len(trimmed) > 0 && trimmed[0] == '{' {
		var result envelope
		if json.Unmarshal(trimmed, &result) == nil && !result.Success {
			code := 0
			if result.Error != nil {
				code = result.Error.Code
			}
			return nil, &APIError{API: api, Method: method, Code: code}
		}
	}
	return body, nil
}

// logRequest emits one debug record per DSM call when a logger is configured.
// It carries only non-secret metadata (never a parameter value) plus the
// request's correlation id, and writes to the logger's sink (stderr), never
// stdout, so it is safe alongside the stdio MCP server.
func (c *Client) logRequest(ctx context.Context, api, method, version, apiPath string, status int, started time.Time) {
	if c.logger == nil {
		return
	}
	attrs := make([]slog.Attr, 0, 7)
	if id := remotepolicy.CorrelationID(ctx); id != "" {
		attrs = append(attrs, slog.String("correlation_id", id))
	}
	attrs = append(attrs,
		slog.String("api", api),
		slog.String("method", method),
		slog.String("version", version),
		slog.String("path", apiPath),
		slog.Int("http_status", status),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
	c.logger.LogAttrs(ctx, slog.LevelDebug, "dsm request", attrs...)
}

func (c *Client) requestLocked(ctx context.Context, apiPath string, params url.Values, api, method string) (json.RawMessage, error) {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/webapi/" + strings.TrimLeft(apiPath, "/")

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewBufferString(params.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "dsmctl/0.1")
	// Some DSM Core APIs only recognize session credentials from the same
	// locations used by the DSM web UI. Keep request parameters for documented
	// WebAPI compatibility and also send the equivalent secure cookie/header.
	if c.sid != "" {
		request.AddCookie(&http.Cookie{Name: "id", Value: c.sid})
	}
	if c.synoToken != "" {
		request.Header.Set("X-SYNO-TOKEN", c.synoToken)
	}

	started := time.Now()
	response, err := c.httpClient.Do(request)
	if err != nil {
		c.logRequest(ctx, api, method, params.Get("version"), apiPath, 0, started)
		// A caller-driven cancellation or deadline is not a transient DSM
		// failure and must never be retried; surface it unclassified (Classify
		// returns unknown) while still wrapping it so errors.Is keeps working.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("request %s: %w", endpoint.Redacted(), err)
		}
		// Any other transport-level failure (timeout, connection reset/refused,
		// DNS) is a temporary condition worth retrying on a read-only call.
		return nil, &HTTPError{Endpoint: endpoint.Redacted(), category: CategoryTransient, Cause: err}
	}
	defer response.Body.Close()
	c.logRequest(ctx, api, method, params.Get("version"), apiPath, response.StatusCode, started)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		switch {
		case response.StatusCode == http.StatusTooManyRequests:
			return nil, &HTTPError{Endpoint: endpoint.Redacted(), Status: response.StatusCode, StatusText: response.Status, category: CategoryRateLimit}
		case response.StatusCode >= 500:
			return nil, &HTTPError{Endpoint: endpoint.Redacted(), Status: response.StatusCode, StatusText: response.Status, category: CategoryTransient}
		default:
			return nil, fmt.Errorf("request %s returned HTTP %s", endpoint.Redacted(), response.Status)
		}
	}

	decoder := json.NewDecoder(io.LimitReader(response.Body, maxBodySize))
	var result envelope
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", api, err)
	}
	if !result.Success {
		code := 0
		if result.Error != nil {
			code = result.Error.Code
		}
		return nil, &APIError{API: api, Method: method, Code: code}
	}
	return result.Data, nil
}

// requestWithRetryLocked performs a single requestLocked call and, only when the
// call site marked itself readOnly, retries a transient or rate-limit HTTP-level
// failure with bounded, jittered exponential backoff. A mutating call
// (readOnly=false) is issued exactly once and never auto-retried — DSM POSTs are
// not idempotent, so eligibility is a property of the call site, not the verb.
// Retry stops on the first success, the first non-retryable error, the attempt
// cap, an exhausted time budget, or context cancellation.
func (c *Client) requestWithRetryLocked(ctx context.Context, apiPath string, params url.Values, api, method string, readOnly bool) (json.RawMessage, error) {
	if !readOnly || c.retry.MaxAttempts <= 1 {
		return c.requestLocked(ctx, apiPath, params, api, method)
	}
	deadline := time.Now().Add(c.retry.Budget)
	delay := c.retry.BaseDelay
	var lastErr error
	for attempt := 1; ; attempt++ {
		data, err := c.requestLocked(ctx, apiPath, params, api, method)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if !isRetryable(err) || attempt >= c.retry.MaxAttempts {
			return nil, err
		}
		wait := jitterDelay(delay, c.retry.MaxDelay)
		if remaining := time.Until(deadline); remaining <= 0 {
			return nil, lastErr
		} else if wait > remaining {
			wait = remaining
		}
		if sleepErr := c.sleep(ctx, wait); sleepErr != nil {
			// The context was cancelled or expired during backoff; report that,
			// not the retryable HTTP error, so the caller sees the cancellation.
			return nil, sleepErr
		}
		if delay < c.retry.MaxDelay {
			delay *= 2
		}
	}
}

// isRetryable reports whether err is safe to retry automatically: only a
// transient or rate-limit HTTP-level failure qualifies. Auth, permission,
// not-found, invalid-input, unsupported, conflict, and unknown never retry.
func isRetryable(err error) bool {
	switch Classify(err) {
	case CategoryTransient, CategoryRateLimit:
		return true
	default:
		return false
	}
}

// jitterDelay applies full jitter to base — a uniformly random value in
// [base/2, base] — capped at maxDelay, so concurrent retriers do not synchronize
// their back-off.
func jitterDelay(base, maxDelay time.Duration) time.Duration {
	if base > maxDelay {
		base = maxDelay
	}
	if base <= 0 {
		return 0
	}
	half := base / 2
	return half + time.Duration(rand.Int63n(int64(base-half)+1))
}

// sleepWithContext waits for d or until ctx is done, whichever comes first,
// returning ctx.Err() when the context ends first. It is the production sleeper;
// tests inject a deterministic one.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// HasSession reports whether this client currently holds a DSM session ID
// from an earlier login. It never contacts the NAS, so the session may have
// expired server-side.
func (c *Client) HasSession() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sid != ""
}

// Close releases the client's DSM session. It logs the session out server-side
// unless the client was created with PreserveSessionOnClose, in which case only
// the in-memory copy is dropped and the session stays valid for later reuse.
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.preserveSession {
		c.sid = ""
		c.synoToken = ""
		return nil
	}
	return c.logoutLocked(ctx)
}

// Logout asks DSM to invalidate the client's session and drops the in-memory
// copy. Unlike Close it always revokes, even on a client created with
// PreserveSessionOnClose — it is the explicit sign-out verb, used when the
// user asks for the persisted session to stop working.
func (c *Client) Logout(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.logoutLocked(ctx)
}

func (c *Client) logoutLocked(ctx context.Context) error {
	if c.sid == "" {
		return nil
	}
	if err := c.ensureAPIsLocked(ctx, authAPI); err != nil {
		return err
	}
	info, _ := c.target.API(authAPI)
	params := url.Values{
		"api":     {authAPI},
		"version": {strconv.Itoa(info.MaxVersion)},
		"method":  {"logout"},
		"session": {dsmctlSession},
		"_sid":    {c.sid},
	}
	_, err := c.requestLocked(ctx, info.Path, params, authAPI, "logout")
	c.sid = ""
	c.synoToken = ""
	if err != nil {
		return fmt.Errorf("log out from DSM: %w", err)
	}
	return nil
}

func cloneValues(values url.Values) url.Values {
	clone := make(url.Values, len(values))
	for key, items := range values {
		clone[key] = append([]string(nil), items...)
	}
	return clone
}

func preferredVersion(info APIInfo, preferred int) int {
	if preferred > info.MaxVersion {
		return info.MaxVersion
	}
	if preferred < info.MinVersion {
		return info.MinVersion
	}
	return preferred
}
