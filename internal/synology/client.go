package synology

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	authAPI       = "SYNO.API.Auth"
	systemInfoAPI = "SYNO.Core.System"
	maxBodySize   = 8 << 20
)

type Options struct {
	BaseURL    string
	Username   string
	Password   string
	HTTPClient *http.Client
}

type APIInfo struct {
	Path          string `json:"path"`
	MinVersion    int    `json:"minVersion"`
	MaxVersion    int    `json:"maxVersion"`
	RequestFormat string `json:"requestFormat"`
}

type Client struct {
	baseURL    *url.URL
	username   string
	password   string
	httpClient *http.Client

	mu        sync.Mutex
	apis      map[string]APIInfo
	sid       string
	synoToken string
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
	if strings.TrimSpace(options.Username) == "" {
		return nil, errors.New("username is required")
	}
	if options.Password == "" {
		return nil, errors.New("password is required")
	}
	baseURL.RawQuery = ""
	baseURL.Fragment = ""
	baseURL.Path = strings.TrimRight(baseURL.Path, "/")

	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:    baseURL,
		username:   options.Username,
		password:   options.Password,
		httpClient: httpClient,
		apis:       make(map[string]APIInfo),
	}, nil
}

func (c *Client) ensureAPIsLocked(ctx context.Context, names ...string) error {
	missing := make([]string, 0, len(names))
	for _, name := range names {
		if _, ok := c.apis[name]; !ok {
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
		info, ok := discovered[name]
		if !ok || info.Path == "" || info.MaxVersion == 0 {
			return fmt.Errorf("Synology API %s is not available on this NAS", name)
		}
		c.apis[name] = info
	}
	return nil
}

func (c *Client) loginLocked(ctx context.Context) error {
	if c.sid != "" {
		return nil
	}
	if err := c.ensureAPIsLocked(ctx, authAPI); err != nil {
		return err
	}
	info := c.apis[authAPI]
	params := url.Values{
		"api":               {authAPI},
		"version":           {strconv.Itoa(info.MaxVersion)},
		"method":            {"login"},
		"account":           {c.username},
		"passwd":            {c.password},
		"session":           {"DSMCTL"},
		"format":            {"sid"},
		"enable_syno_token": {"yes"},
	}
	data, err := c.requestLocked(ctx, info.Path, params, authAPI, "login")
	if err != nil {
		return fmt.Errorf("log in to DSM: %w", err)
	}
	var result struct {
		SID       string `json:"sid"`
		SynoToken string `json:"synotoken"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("decode DSM login: %w", err)
	}
	if result.SID == "" {
		return errors.New("DSM login response did not contain a session ID")
	}
	c.sid = result.SID
	c.synoToken = result.SynoToken
	return nil
}

func (c *Client) callLocked(ctx context.Context, api, method string, parameters url.Values) (json.RawMessage, error) {
	if err := c.ensureAPIsLocked(ctx, api); err != nil {
		return nil, err
	}
	if err := c.loginLocked(ctx); err != nil {
		return nil, err
	}
	info := c.apis[api]
	params := cloneValues(parameters)
	params.Set("api", api)
	params.Set("version", strconv.Itoa(info.MaxVersion))
	params.Set("method", method)
	params.Set("_sid", c.sid)
	if c.synoToken != "" {
		params.Set("SynoToken", c.synoToken)
	}

	data, err := c.requestLocked(ctx, info.Path, params, api, method)
	if isSessionError(err) {
		c.sid = ""
		c.synoToken = ""
		if loginErr := c.loginLocked(ctx); loginErr != nil {
			return nil, loginErr
		}
		params.Set("_sid", c.sid)
		params.Del("SynoToken")
		if c.synoToken != "" {
			params.Set("SynoToken", c.synoToken)
		}
		return c.requestLocked(ctx, info.Path, params, api, method)
	}
	return data, err
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

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", endpoint.Redacted(), err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("request %s returned HTTP %s", endpoint.Redacted(), response.Status)
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

func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sid == "" {
		return nil
	}
	if err := c.ensureAPIsLocked(ctx, authAPI); err != nil {
		return err
	}
	info := c.apis[authAPI]
	params := url.Values{
		"api":     {authAPI},
		"version": {strconv.Itoa(info.MaxVersion)},
		"method":  {"logout"},
		"session": {"DSMCTL"},
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
