package synologyauth

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/ychiu1211/dsmctl/internal/gateway/platformauth"
)

var ErrUnauthorized = errors.New("DSM administrator authentication failed")

const gatewayAdministratorCookie = "dsmctl_admin_session"

type Validator interface {
	Validate(*http.Request) (string, error)
}

type Options struct {
	Backend         *url.URL
	Signer          *platformauth.Signer
	Validator       Validator
	Logger          *slog.Logger
	RequireLoopback bool
}

func New(options Options) (http.Handler, error) {
	if options.Backend == nil || options.Backend.Scheme != "http" || options.Backend.Host == "" || options.Backend.User != nil {
		return nil, errors.New("Synology auth backend must be an absolute private HTTP URL")
	}
	if options.Signer == nil || options.Validator == nil {
		return nil, errors.New("Synology auth signer and DSM validator are required")
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	proxy := httputil.NewSingleHostReverseProxy(options.Backend)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = options.Backend.Host
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		options.Logger.Error("gateway backend unavailable", "path", safePath(req.URL.Path), "error", err)
		http.Error(w, "gateway backend unavailable", http.StatusBadGateway)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if options.RequireLoopback && !requestFromLoopback(req) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		forwardedHost := strings.TrimSpace(req.Header.Get("X-Forwarded-Host"))
		if forwardedHost == "" {
			forwardedHost = req.Host
		}
		forwardedProto := strings.TrimSpace(req.Header.Get("X-Forwarded-Proto"))
		if forwardedProto != "http" && forwardedProto != "https" {
			forwardedProto = "http"
			if req.TLS != nil {
				forwardedProto = "https"
			}
		}
		prefix := strings.TrimRight(strings.TrimSpace(req.Header.Get("X-Forwarded-Prefix")), "/")
		if prefix != "" && strings.HasPrefix(req.URL.Path, prefix+"/") {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
		}

		// Never pass a caller-provided assertion or DSM session cookie into the
		// container. Validate first, then preserve only the Gateway cookie.
		req.Header.Del(platformauth.HeaderName)
		subject, validationErr := "", ErrUnauthorized
		if needsDSMAssertion(req.URL.Path) {
			subject, validationErr = options.Validator.Validate(req)
		}
		retainGatewayCookie(req)
		if validationErr == nil {
			assertion, err := options.Signer.Sign(subject)
			if err != nil {
				options.Logger.Error("sign DSM administrator assertion", "error", err)
				http.Error(w, "administrator authentication unavailable", http.StatusServiceUnavailable)
				return
			}
			req.Header.Set(platformauth.HeaderName, assertion)
		} else if req.URL.Path == "/admin/api/dsm-login" {
			options.Logger.Warn("DSM administrator Web Login denied")
			http.Error(w, "DSM administrator Web Login required", http.StatusUnauthorized)
			return
		}

		req.Header.Set("X-Forwarded-Proto", forwardedProto)
		req.Header.Set("X-Forwarded-Host", forwardedHost)
		if prefix != "" {
			req.Header.Set("X-Forwarded-Prefix", prefix)
		} else {
			req.Header.Del("X-Forwarded-Prefix")
		}
		proxy.ServeHTTP(w, req)
	}), nil
}

func needsDSMAssertion(path string) bool {
	return strings.HasPrefix(path, "/admin/api/") || path == "/oauth/authorize"
}

func retainGatewayCookie(req *http.Request) {
	var values []string
	for _, cookie := range req.Cookies() {
		if cookie.Name == gatewayAdministratorCookie {
			values = append(values, cookie.Name+"="+cookie.Value)
		}
	}
	if len(values) == 0 {
		req.Header.Del("Cookie")
		return
	}
	req.Header.Set("Cookie", strings.Join(values, "; "))
}

func requestFromLoopback(req *http.Request) bool {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}
	address := net.ParseIP(strings.TrimSpace(host))
	return address != nil && address.IsLoopback()
}

func safePath(path string) string {
	switch {
	case path == "/mcp", path == "/healthz", path == "/readyz", path == "/oauth/authorize", strings.HasPrefix(path, "/admin"):
		return path
	default:
		return "other"
	}
}
