package synologyauth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/gateway/platformauth"
)

type validatorFunc func(*http.Request) (string, error)

func (f validatorFunc) Validate(req *http.Request) (string, error) { return f(req) }

func TestProxyAttachesAssertionAndStripsDSMCookie(t *testing.T) {
	key := bytes.Repeat([]byte{3}, 32)
	signer, _ := platformauth.NewSigner(key, platformauth.DefaultAudience)
	verifier, _ := platformauth.NewVerifier(key, platformauth.DefaultAudience)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		identity, err := verifier.Verify(context.Background(), req.Header.Get(platformauth.HeaderName))
		if err != nil || identity.Subject != "dsm-admin" {
			http.Error(w, "bad assertion", http.StatusUnauthorized)
			return
		}
		if cookie := req.Header.Get("Cookie"); cookie != "dsmctl_admin_session=gateway" {
			http.Error(w, "cookie leaked: "+cookie, http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, req.URL.Path)
	}))
	defer backend.Close()
	backendURL, _ := url.Parse(backend.URL)
	handler, err := New(Options{Backend: backendURL, Signer: signer, RequireLoopback: true, Validator: validatorFunc(func(req *http.Request) (string, error) {
		if !strings.Contains(req.Header.Get("Cookie"), "DSM_SESSION=secret") {
			return "", ErrUnauthorized
		}
		return "dsm-admin", nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/dsmctl/admin/api/dsm-login", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("Cookie", "DSM_SESSION=secret; dsmctl_admin_session=gateway")
	request.Header.Set("X-Forwarded-Prefix", "/dsmctl")
	request.Header.Set(platformauth.HeaderName, "forged")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "/admin/api/dsm-login" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestProxyLetsLocalLoginPassWithoutDSMIdentity(t *testing.T) {
	signer, _ := platformauth.NewSigner(bytes.Repeat([]byte{4}, 32), platformauth.DefaultAudience)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get(platformauth.HeaderName) != "" {
			http.Error(w, "unexpected assertion", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()
	backendURL, _ := url.Parse(backend.URL)
	handler, _ := New(Options{Backend: backendURL, Signer: signer, Validator: validatorFunc(func(*http.Request) (string, error) { return "", errors.New("not logged in") })})
	request := httptest.NewRequest(http.MethodPost, "/admin/api/login", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("local login response = %d", response.Code)
	}
}

func TestProxyFailsClosedForDSMLoginAndNonLoopback(t *testing.T) {
	signer, _ := platformauth.NewSigner(bytes.Repeat([]byte{5}, 32), platformauth.DefaultAudience)
	backendURL, _ := url.Parse("http://127.0.0.1:1")
	handler, _ := New(Options{Backend: backendURL, Signer: signer, RequireLoopback: true, Validator: validatorFunc(func(*http.Request) (string, error) { return "", ErrUnauthorized })})

	nonLoopback := httptest.NewRequest(http.MethodPost, "/admin/api/dsm-login", nil)
	nonLoopback.RemoteAddr = "192.0.2.1:1234"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, nonLoopback)
	if response.Code != http.StatusForbidden {
		t.Fatalf("non-loopback status = %d", response.Code)
	}

	loopback := httptest.NewRequest(http.MethodPost, "/admin/api/dsm-login", nil)
	loopback.RemoteAddr = "127.0.0.1:1234"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, loopback)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("missing DSM login status = %d", response.Code)
	}
}
