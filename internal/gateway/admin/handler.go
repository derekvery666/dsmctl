package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ychiu1211/dsmctl/internal/credentials"
	"github.com/ychiu1211/dsmctl/internal/gateway/state"
	"github.com/ychiu1211/dsmctl/internal/runtime"
	"github.com/ychiu1211/dsmctl/internal/synology"
	"github.com/ychiu1211/dsmctl/internal/weblogin"
)

const (
	maxAdminBody   = int64(1 << 20)
	enrollmentTTL  = 5 * time.Minute
	networkTimeout = 10 * time.Second
)

type Options struct {
	Repository *state.Repository
	Manager    *runtime.Manager
	PublicURL  string
}

type Handler struct {
	repository *state.Repository
	manager    *runtime.Manager
	publicURL  string

	pendingMu sync.Mutex
	pending   map[string]pendingEnrollment
}

type pendingEnrollment struct {
	ProfileName string
	Enrollment  *weblogin.Enrollment
	ExpiresAt   time.Time
}

type profileInput struct {
	state.ProfileInput
	ConfirmCertificateFingerprint bool `json:"confirm_certificate_fingerprint,omitempty"`
}

func (input profileInput) validateFingerprintConfirmation() error {
	if input.TLSMode == state.TLSPinnedFingerprint && !input.ConfirmCertificateFingerprint {
		return errors.New("pinned_fingerprint TLS mode requires explicit certificate fingerprint confirmation")
	}
	return nil
}

func New(options Options) (*Handler, error) {
	if options.Repository == nil {
		return nil, errors.New("gateway state repository is required")
	}
	if options.Manager == nil {
		return nil, errors.New("gateway runtime manager is required")
	}
	publicURL := strings.TrimRight(strings.TrimSpace(options.PublicURL), "/")
	if publicURL != "" {
		parsed, err := url.Parse(publicURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
			return nil, errors.New("admin public URL must be an absolute http or https origin")
		}
		publicURL = parsed.Scheme + "://" + parsed.Host
	}
	return &Handler{repository: options.Repository, manager: options.Manager, publicURL: publicURL, pending: make(map[string]pendingEnrollment)}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if strings.HasPrefix(req.URL.Path, "/admin/api/") {
		if req.ContentLength > maxAdminBody {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		req.Body = http.MaxBytesReader(w, req.Body, maxAdminBody)
	}
	switch req.URL.Path {
	case "/admin", "/admin/":
		if req.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		_, _ = io.WriteString(w, indexHTML)
		return
	case "/admin/api/bootstrap":
		h.bootstrap(w, req)
		return
	}
	if !strings.HasPrefix(req.URL.Path, "/admin/api/") {
		http.NotFound(w, req)
		return
	}
	if err := h.authenticate(req); err != nil {
		w.Header().Set("WWW-Authenticate", `Bearer realm="dsmctl-gateway-admin"`)
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if req.URL.Path == "/admin/api/status" {
		h.status(w, req)
		return
	}
	if req.URL.Path == "/admin/api/admin-token/rotate" {
		h.rotateAdminToken(w, req)
		return
	}
	if req.URL.Path == "/admin/api/orphan-secrets" || strings.HasPrefix(req.URL.Path, "/admin/api/orphan-secrets/") {
		h.orphanSecrets(w, req)
		return
	}
	if req.URL.Path == "/admin/api/profiles" {
		h.profiles(w, req)
		return
	}
	if strings.HasPrefix(req.URL.Path, "/admin/api/profiles/") {
		h.profile(w, req)
		return
	}
	http.NotFound(w, req)
}

func (h *Handler) authenticate(req *http.Request) error {
	parts := strings.Fields(req.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return state.ErrUnauthorized
	}
	return h.repository.AuthenticateAdministrator(req.Context(), parts[1])
}

func (h *Handler) bootstrap(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var input struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(req, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	token, err := h.repository.EstablishAdministrator(req.Context(), input.Token)
	input.Token = ""
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, state.ErrBootstrapConsumed) {
			status = http.StatusConflict
		}
		writeError(w, status, "bootstrap failed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"admin_token": token})
}

func (h *Handler) status(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	health, err := h.repository.Health(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read gateway status")
		return
	}
	writeJSON(w, http.StatusOK, health)
}

func (h *Handler) profiles(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		profiles, err := h.repository.Profiles(req.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list profiles")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"profiles": profiles})
	case http.MethodPost:
		var input profileInput
		if err := decodeJSON(req, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := input.validateFingerprintConfirmation(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var created state.Profile
		err := h.manager.MutateProfile(req.Context(), input.Name, func() error {
			var err error
			created, err = h.repository.CreateProfile(req.Context(), input.ProfileInput)
			return err
		})
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) profile(w http.ResponseWriter, req *http.Request) {
	relative := strings.TrimPrefix(req.URL.Path, "/admin/api/profiles/")
	parts := strings.Split(relative, "/")
	name, err := url.PathUnescape(parts[0])
	if err != nil || name == "" {
		writeError(w, http.StatusBadRequest, "invalid profile name")
		return
	}
	action := ""
	if len(parts) > 1 {
		action = strings.Join(parts[1:], "/")
	}
	switch action {
	case "":
		h.profileRecord(w, req, name)
	case "default":
		h.setDefault(w, req, name)
	case "test":
		h.testProfile(w, req, name)
	case "credentials/status":
		h.credentialStatus(w, req, name)
	case "credentials/password":
		h.passwordEnrollment(w, req, name)
	case "credentials/session":
		h.removeSession(w, req, name)
	case "credentials/trusted-device":
		h.removeTrustedDevice(w, req, name)
	case "weblogin/start":
		h.startWebLogin(w, req, name)
	case "weblogin/complete":
		h.completeWebLogin(w, req, name)
	case "secrets":
		h.applySecrets(w, req, name, "")
	default:
		if strings.HasPrefix(action, "secrets/") {
			id, _ := url.PathUnescape(strings.TrimPrefix(action, "secrets/"))
			h.applySecrets(w, req, name, id)
			return
		}
		http.NotFound(w, req)
	}
}

func (h *Handler) rotateAdminToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	token, err := h.repository.RotateAdministrator(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rotate administrator token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"admin_token": token})
}

func (h *Handler) orphanSecrets(w http.ResponseWriter, req *http.Request) {
	id := strings.TrimPrefix(req.URL.Path, "/admin/api/orphan-secrets/")
	if req.URL.Path == "/admin/api/orphan-secrets" {
		id = ""
	}
	if id == "" {
		if req.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		secrets, err := h.repository.OrphanedSecrets(req.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list retained secrets")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"secrets": secrets})
		return
	}
	if req.Method != http.MethodDelete {
		methodNotAllowed(w, http.MethodDelete)
		return
	}
	decodedID, err := url.PathUnescape(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid secret identifier")
		return
	}
	removed, err := h.repository.DeleteOrphanedSecret(req.Context(), decodedID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"removed": removed})
}

func (h *Handler) profileRecord(w http.ResponseWriter, req *http.Request, name string) {
	switch req.Method {
	case http.MethodGet:
		profile, err := h.repository.Profile(req.Context(), name)
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, profile)
	case http.MethodPut:
		var input struct {
			profileInput
			ExpectedRevision uint64 `json:"expected_revision"`
		}
		if err := decodeJSON(req, &input); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := input.validateFingerprintConfirmation(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		var updated state.Profile
		err := h.manager.MutateProfile(req.Context(), name, func() error {
			var err error
			updated, err = h.repository.UpdateProfile(req.Context(), name, input.ExpectedRevision, input.ProfileInput)
			return err
		})
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		revision, err := strconv.ParseUint(req.URL.Query().Get("revision"), 10, 64)
		if err != nil || revision == 0 {
			writeError(w, http.StatusBadRequest, "revision query parameter is required")
			return
		}
		retain := req.URL.Query().Get("retain_credentials") == "true"
		var removed state.Profile
		var revocationError string
		if !retain {
			revokeCtx, cancel := context.WithTimeout(req.Context(), networkTimeout)
			_, revokeErr := h.manager.RevokeStoredSession(revokeCtx, name)
			cancel()
			if revokeErr != nil {
				revocationError = revokeErr.Error()
			}
		}
		err = h.manager.MutateProfile(req.Context(), name, func() error {
			var err error
			removed, err = h.repository.DeleteProfile(req.Context(), name, revision, retain)
			return err
		})
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		response := map[string]any{"removed": removed.Name, "credentials_retained": retain}
		if revocationError != "" {
			response["session_revocation_error"] = revocationError
		}
		if retain {
			retained, _ := h.repository.SecretMetadataForProfile(req.Context(), removed.ID)
			response["retained_secrets"] = retained
		}
		writeJSON(w, http.StatusOK, response)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
	}
}

func (h *Handler) setDefault(w http.ResponseWriter, req *http.Request, name string) {
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if err := h.manager.MutateProfile(req.Context(), "", func() error { return h.repository.SetDefault(req.Context(), name) }); err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"default": name})
}

type testStage struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Error  string `json:"error,omitempty"`
}

func (h *Handler) testProfile(w http.ResponseWriter, req *http.Request, name string) {
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	profile, err := h.repository.Profile(req.Context(), name)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	parsed, _ := url.Parse(profile.URL)
	stages := make([]testStage, 0, 4)
	ctx, cancel := context.WithTimeout(req.Context(), networkTimeout)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupHost(ctx, parsed.Hostname())
	if err != nil || len(addresses) == 0 {
		stages = append(stages, failedStage("dns", err))
		writeJSON(w, http.StatusBadGateway, map[string]any{"nas": name, "stages": stages})
		return
	}
	stages = append(stages, testStage{Name: "dns", Passed: true})
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(parsed.Hostname(), port))
	if err != nil {
		stages = append(stages, failedStage("tcp", err))
		writeJSON(w, http.StatusBadGateway, map[string]any{"nas": name, "stages": stages})
		return
	}
	_ = connection.Close()
	stages = append(stages, testStage{Name: "tcp", Passed: true})
	cfg, _ := h.repository.Snapshot(ctx)
	runtimeProfile := cfg.NAS[name]
	httpRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, profile.URL+"/webapi/query.cgi?api=SYNO.API.Info&version=1&method=query&query=SYNO.API.Auth", nil)
	response, err := runtime.HTTPClient(runtimeProfile).Do(httpRequest)
	if err != nil {
		stages = append(stages, failedStage("tls_http", err))
		writeJSON(w, http.StatusBadGateway, map[string]any{"nas": name, "stages": stages})
		return
	}
	_ = response.Body.Close()
	stages = append(stages, testStage{Name: "tls_http", Passed: true})
	_, client, err := h.manager.Client(ctx, name)
	if err == nil {
		err = client.Authenticate(ctx)
	}
	if err == nil {
		_, err = client.SystemInfo(ctx)
	}
	if err != nil {
		stages = append(stages, failedStage("dsm", err))
		writeJSON(w, http.StatusBadGateway, map[string]any{"nas": name, "stages": stages})
		return
	}
	stages = append(stages, testStage{Name: "dsm", Passed: true})
	writeJSON(w, http.StatusOK, map[string]any{"nas": name, "stages": stages})
}

func (h *Handler) credentialStatus(w http.ResponseWriter, req *http.Request, name string) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	profile, err := h.repository.Profile(req.Context(), name)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	meta, err := h.repository.SessionMeta(req.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read credential status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"nas": profile.Name, "revision": profile.Revision,
		"password_stored": profile.PasswordStored, "trusted_device_stored": profile.TrustedDeviceStored,
		"session": meta,
	})
}

func (h *Handler) passwordEnrollment(w http.ResponseWriter, req *http.Request, name string) {
	if req.Method == http.MethodDelete {
		var removed bool
		err := h.manager.MutateProfile(req.Context(), name, func() error {
			var err error
			removed, _, err = h.repository.DeletePassword(req.Context(), name)
			return err
		})
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"removed": removed})
		return
	}
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost, http.MethodDelete)
		return
	}
	var input struct {
		Password string `json:"password"`
		OTP      string `json:"otp,omitempty"`
	}
	if err := decodeJSON(req, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer func() { input.Password, input.OTP = "", "" }()
	cfg, err := h.repository.Snapshot(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load profile")
		return
	}
	profile, ok := cfg.NAS[name]
	if !ok {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}
	device := credentials.TrustedDevice{Name: "dsmctl-gateway"}
	client, err := synology.NewClient(synology.Options{
		BaseURL: profile.URL, Username: profile.Username, Password: input.Password,
		DeviceName: device.Name, HTTPClient: runtime.HTTPClient(profile),
		OTPProvider: func(context.Context) (string, error) {
			if input.OTP == "" {
				return "", errors.New("one-time password was not supplied")
			}
			return input.OTP, nil
		},
		SaveDeviceID: func(_ context.Context, id string) error { device.ID = id; return nil },
	})
	if err == nil {
		err = client.Authenticate(req.Context())
	}
	if client != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), networkTimeout)
		_ = client.Close(closeCtx)
		cancel()
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "DSM rejected password enrollment")
		return
	}
	if device.ID == "" {
		device = credentials.TrustedDevice{}
	}
	err = h.manager.MutateProfile(req.Context(), name, func() error {
		_, err := h.repository.EnrollPassword(req.Context(), name, input.Password, device)
		return err
	})
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"nas": name, "password_stored": true, "trusted_device_stored": device.ID != ""})
}

func (h *Handler) removeSession(w http.ResponseWriter, req *http.Request, name string) {
	if req.Method != http.MethodDelete {
		methodNotAllowed(w, http.MethodDelete)
		return
	}
	revokeCtx, cancel := context.WithTimeout(req.Context(), networkTimeout)
	revoked, revokeErr := h.manager.RevokeStoredSession(revokeCtx, name)
	cancel()
	var removed bool
	err := h.manager.MutateProfile(req.Context(), name, func() error {
		var err error
		removed, err = h.repository.DeleteSession(req.Context(), name)
		return err
	})
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	response := map[string]any{"removed": removed, "revoked": revoked}
	if revokeErr != nil {
		response["revocation_error"] = revokeErr.Error()
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) applySecrets(w http.ResponseWriter, req *http.Request, name, id string) {
	profile, err := h.repository.Profile(req.Context(), name)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	if id == "" {
		switch req.Method {
		case http.MethodGet:
			metadata, err := h.repository.SecretMetadataForProfile(req.Context(), profile.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "list vault references")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"secrets": metadata})
		case http.MethodPost:
			var input struct {
				Value string `json:"value"`
			}
			if err := decodeJSON(req, &input); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			metadata, err := h.repository.StoreApplySecret(req.Context(), name, input.Value)
			input.Value = ""
			if err != nil {
				writeRepositoryError(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"secret": metadata, "reference": "vault:" + metadata.ID})
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
		return
	}
	if req.Method != http.MethodDelete {
		methodNotAllowed(w, http.MethodDelete)
		return
	}
	removed, err := h.repository.DeleteApplySecret(req.Context(), profile.ID, id)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"removed": removed})
}

func (h *Handler) removeTrustedDevice(w http.ResponseWriter, req *http.Request, name string) {
	if req.Method != http.MethodDelete {
		methodNotAllowed(w, http.MethodDelete)
		return
	}
	var removed bool
	err := h.manager.MutateProfile(req.Context(), name, func() error {
		var err error
		removed, _, err = h.repository.DeleteTrustedDevice(req.Context(), name)
		return err
	})
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"removed": removed})
}

func (h *Handler) startWebLogin(w http.ResponseWriter, req *http.Request, name string) {
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	profile, err := h.repository.Profile(req.Context(), name)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	cfg, _ := h.repository.Snapshot(req.Context())
	opener := h.publicURL
	if opener == "" {
		scheme := "http"
		if req.TLS != nil {
			scheme = "https"
		}
		opener = scheme + "://" + req.Host
	}
	enrollment, start, err := weblogin.BeginEnrollment(profile.URL, opener+"/admin/", weblogin.Options{HTTPClient: runtime.HTTPClient(cfg.NAS[name])})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create enrollment")
		return
	}
	expires := time.Now().Add(enrollmentTTL)
	h.pendingMu.Lock()
	h.prunePendingLocked(time.Now())
	h.pending[id] = pendingEnrollment{ProfileName: name, Enrollment: enrollment, ExpiresAt: expires}
	h.pendingMu.Unlock()
	parsedNAS, _ := url.Parse(profile.URL)
	writeJSON(w, http.StatusCreated, map[string]any{"enrollment_id": id, "login_url": start.LoginURL, "state": start.State, "nas_origin": parsedNAS.Scheme + "://" + parsedNAS.Host, "expires_at": expires})
}

func (h *Handler) completeWebLogin(w http.ResponseWriter, req *http.Request, name string) {
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var input struct {
		EnrollmentID string `json:"enrollment_id"`
		Code         string `json:"code"`
		RS           string `json:"rs"`
		State        string `json:"state"`
	}
	if err := decodeJSON(req, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.pendingMu.Lock()
	pending, ok := h.pending[input.EnrollmentID]
	delete(h.pending, input.EnrollmentID)
	h.pendingMu.Unlock()
	if !ok || pending.ProfileName != name || time.Now().After(pending.ExpiresAt) {
		writeError(w, http.StatusGone, "web-login enrollment expired or was already used")
		return
	}
	result, err := pending.Enrollment.Complete(req.Context(), input.Code, input.RS, input.State)
	input.Code, input.RS = "", ""
	if err != nil {
		writeError(w, http.StatusBadGateway, "DSM web-login exchange failed")
		return
	}
	now := time.Now().UTC()
	session := credentials.SessionCredential{
		SID: result.SID, SynoToken: result.SynoToken, DeviceID: result.DeviceID,
		ServerPublicKey: result.ServerPublicKey, LocalPublicKey: result.LocalPublicKey, LocalPrivateKey: result.LocalPrivateKey,
		Account: result.Account, IssuedAt: now, LastVerified: now,
	}
	err = h.manager.MutateProfile(req.Context(), name, func() error {
		_, err := h.repository.EnrollSession(req.Context(), name, session)
		return err
	})
	if err != nil {
		writeRepositoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"nas": name, "account": result.Account, "session_stored": true, "renewable": session.CanResume()})
}

func (h *Handler) prunePendingLocked(now time.Time) {
	for id, pending := range h.pending {
		if now.After(pending.ExpiresAt) {
			delete(h.pending, id)
		}
	}
}

func failedStage(name string, err error) testStage {
	message := "no result"
	if err != nil {
		message = err.Error()
	}
	return testStage{Name: name, Passed: false, Error: message}
}

func decodeJSON(req *http.Request, target any) error {
	if mediaType := strings.ToLower(strings.TrimSpace(strings.Split(req.Header.Get("Content-Type"), ";")[0])); mediaType != "application/json" {
		return errors.New("Content-Type must be application/json")
	}
	decoder := json.NewDecoder(io.LimitReader(req.Body, maxAdminBody+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON request: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request must contain one JSON object")
	}
	return nil
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeRepositoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, state.ErrNotFound):
		writeError(w, http.StatusNotFound, "profile not found")
	case errors.Is(err, state.ErrRevisionConflict):
		writeError(w, http.StatusConflict, "profile revision conflict")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func randomID() (string, error) {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
