package loginportal

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

// recordingExecutor returns a canned response per (api, method) and records the
// requests it saw, so tests can assert both decode results and the request
// contract (method, version) sent to DSM.
type recordingExecutor struct {
	responses map[string]json.RawMessage
	requests  []compatibility.Request
}

func (e *recordingExecutor) Execute(_ context.Context, request compatibility.Request) (json.RawMessage, error) {
	e.requests = append(e.requests, request)
	if resp, ok := e.responses[request.API+"."+request.Method]; ok {
		return resp, nil
	}
	return json.RawMessage(`{}`), nil
}

func (e *recordingExecutor) ExecuteScript(_ context.Context, _ compatibility.Request) ([]byte, error) {
	return nil, nil
}

func lpTarget() compatibility.Target {
	target := compatibility.NewTarget()
	target.SetAPI(WebDSMAPIName, compatibility.APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 2})
	target.SetAPI(WebDSMExternalAPIName, compatibility.APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 1})
	target.SetAPI(AppPortalAPIName, compatibility.APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 2})
	target.SetAPI(ReverseProxyAPIName, compatibility.APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 1})
	return target
}

// Live shapes captured from the DSM 7.3 lab.
const (
	liveWebDSMv1     = `{"enable_avahi":true,"enable_custom_domain":false,"enable_hsts":false,"enable_https":true,"enable_https_redirect":false,"enable_max_connections":true,"enable_reuseport":true,"enable_server_header":true,"enable_spdy":true,"enable_ssdp":true,"fqdn":null,"http_port":5000,"https_port":5001,"main_app":"DSM","max_connections":131070,"max_connections_limit":{"lower":2048,"upper":131070},"server_header":"nginx","support_reuseport":true}`
	liveExternal     = `{"hostname":""}`
	liveAppPortal    = `{"portal":[{"display_name":"Download Station","enable_redirect":false,"id":"SYNO.SDS.DownloadStation.Application"},{"display_name":"File Station","enable_redirect":false,"id":"SYNO.SDS.App.FileStation3.Instance"}]}`
	liveReverseProxy = `{"entries":[]}`
)

func TestSelectorsRequireTheirAPI(t *testing.T) {
	full := lpTarget()
	empty := compatibility.NewTarget()
	cases := []struct {
		name    string
		backend string
		selectF func(compatibility.Target) (compatibility.Selection, error)
	}{
		{"dsm", "login-portal-web-dsm-get-v1", SelectDSMWebService},
		{"external", "login-portal-web-dsm-external-get-v1", SelectExternalDomain},
		{"applications", "login-portal-appportal-list-v1", SelectApplicationPortal},
		{"reverse-proxy", "login-portal-reverse-proxy-list-v1", SelectReverseProxy},
	}
	for _, tc := range cases {
		selection, err := tc.selectF(full)
		if err != nil || !selection.Supported || selection.Backend != tc.backend {
			t.Fatalf("%s: selection=%#v err=%v", tc.name, selection, err)
		}
		selection, err = tc.selectF(empty)
		if !compatibility.IsUnsupported(err) || selection.Supported {
			t.Fatalf("%s: expected unsupported, got selection=%#v err=%v", tc.name, selection, err)
		}
	}
}

// TestIndependentBoundaries proves one area being absent never disables another:
// a target with only the reverse-proxy API reports that supported and the rest
// unsupported (e.g. the External sibling being missing does not fail DSM access).
func TestIndependentBoundaries(t *testing.T) {
	target := compatibility.NewTarget()
	target.SetAPI(ReverseProxyAPIName, compatibility.APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 1})
	got := map[string]bool{}
	for _, s := range Select(target) {
		got[s.Operation] = s.Supported
	}
	if !got[ReverseProxyReadCapabilityName] {
		t.Fatalf("reverse proxy should be supported: %#v", got)
	}
	for _, op := range []string{DSMWebServiceReadCapabilityName, ExternalDomainReadCapabilityName, ApplicationPortalReadCapabilityName} {
		if got[op] {
			t.Fatalf("%s should be unsupported when only reverse proxy is present: %#v", op, got)
		}
	}
	if SupportsExternalDomain(target) {
		t.Fatalf("external domain API should be absent")
	}
}

func TestExecuteDSMWebServiceDecodesLiveShape(t *testing.T) {
	exec := &recordingExecutor{responses: map[string]json.RawMessage{
		WebDSMAPIName + ".get": json.RawMessage(liveWebDSMv1),
	}}
	settings, selection, err := ExecuteDSMWebService(context.Background(), lpTarget(), exec)
	if err != nil {
		t.Fatalf("ExecuteDSMWebService() error = %v", err)
	}
	if selection.Backend != "login-portal-web-dsm-get-v1" {
		t.Fatalf("backend = %q", selection.Backend)
	}
	if settings.HTTPPort != 5000 || settings.HTTPSPort != 5001 || !settings.HTTPSEnabled ||
		settings.HTTPRedirectEnabled || settings.HSTSEnabled || !settings.HTTP2Enabled ||
		settings.CustomDomainEnabled || settings.CustomDomain != "" {
		t.Fatalf("settings = %#v", settings)
	}
	req := exec.requests[0]
	if req.API != WebDSMAPIName || req.Version != 1 || req.Method != "get" || !req.ReadOnly {
		t.Fatalf("request = %#v", req)
	}
}

func TestExecuteDSMWebServiceRejectsUnknownShape(t *testing.T) {
	exec := &recordingExecutor{responses: map[string]json.RawMessage{
		WebDSMAPIName + ".get": json.RawMessage(`{"unexpected":1}`),
	}}
	if _, _, err := ExecuteDSMWebService(context.Background(), lpTarget(), exec); err == nil || !strings.Contains(err.Error(), "no recognized fields") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecuteExternalDomainDecodesLiveShape(t *testing.T) {
	exec := &recordingExecutor{responses: map[string]json.RawMessage{
		WebDSMExternalAPIName + ".get": json.RawMessage(liveExternal),
	}}
	settings, _, err := ExecuteExternalDomain(context.Background(), lpTarget(), exec)
	if err != nil {
		t.Fatalf("ExecuteExternalDomain() error = %v", err)
	}
	if !settings.ExternalDomainSupported || settings.ExternalHostname != "" {
		t.Fatalf("external = %#v", settings)
	}
	// A configured hostname decodes through.
	exec.responses[WebDSMExternalAPIName+".get"] = json.RawMessage(`{"hostname":"dsm.example.com"}`)
	settings, _, err = ExecuteExternalDomain(context.Background(), lpTarget(), exec)
	if err != nil || settings.ExternalHostname != "dsm.example.com" {
		t.Fatalf("external = %#v err = %v", settings, err)
	}
}

func TestExecuteExternalDomainRejectsUnknownShape(t *testing.T) {
	exec := &recordingExecutor{responses: map[string]json.RawMessage{
		WebDSMExternalAPIName + ".get": json.RawMessage(`{"something":1}`),
	}}
	if _, _, err := ExecuteExternalDomain(context.Background(), lpTarget(), exec); err == nil || !strings.Contains(err.Error(), "no hostname") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecuteApplicationPortalsDecodesLiveShape(t *testing.T) {
	exec := &recordingExecutor{responses: map[string]json.RawMessage{
		AppPortalAPIName + ".list": json.RawMessage(liveAppPortal),
	}}
	portals, selection, err := ExecuteApplicationPortals(context.Background(), lpTarget(), exec)
	if err != nil {
		t.Fatalf("ExecuteApplicationPortals() error = %v", err)
	}
	if selection.Backend != "login-portal-appportal-list-v1" {
		t.Fatalf("backend = %q", selection.Backend)
	}
	if portals.Total != 2 || len(portals.Portals) != 2 {
		t.Fatalf("portals = %#v", portals)
	}
	if portals.Portals[1].AppID != "SYNO.SDS.App.FileStation3.Instance" || portals.Portals[1].DisplayName != "File Station" || portals.Portals[1].RedirectHTTPS {
		t.Fatalf("portal = %#v", portals.Portals[1])
	}
}

func TestExecuteApplicationPortalsDecodesCustomPortal(t *testing.T) {
	// Synthetic: a custom alias/port portal (not present on the lab, decoded leniently).
	exec := &recordingExecutor{responses: map[string]json.RawMessage{
		AppPortalAPIName + ".list": json.RawMessage(`{"portal":[{"id":"SYNO.SDS.App.FileStation3.Instance","display_name":"File Station","enable_redirect":true,"alias":"files","http_port":7000,"https_port":7001}]}`),
	}}
	portals, _, err := ExecuteApplicationPortals(context.Background(), lpTarget(), exec)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	p := portals.Portals[0]
	if !p.RedirectHTTPS || p.Alias != "files" || p.HTTPPort != 7000 || p.HTTPSPort != 7001 {
		t.Fatalf("portal = %#v", p)
	}
}

func TestExecuteApplicationPortalsRejectsUnknownShape(t *testing.T) {
	exec := &recordingExecutor{responses: map[string]json.RawMessage{
		AppPortalAPIName + ".list": json.RawMessage(`{"items":[]}`),
	}}
	if _, _, err := ExecuteApplicationPortals(context.Background(), lpTarget(), exec); err == nil || !strings.Contains(err.Error(), "no portal array") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecuteReverseProxyDecodesEmptyLiveShape(t *testing.T) {
	exec := &recordingExecutor{responses: map[string]json.RawMessage{
		ReverseProxyAPIName + ".list": json.RawMessage(liveReverseProxy),
	}}
	rules, selection, err := ExecuteReverseProxyRules(context.Background(), lpTarget(), exec)
	if err != nil {
		t.Fatalf("ExecuteReverseProxyRules() error = %v", err)
	}
	if selection.Backend != "login-portal-reverse-proxy-list-v1" {
		t.Fatalf("backend = %q", selection.Backend)
	}
	if rules.Total != 0 || len(rules.Rules) != 0 {
		t.Fatalf("rules = %#v", rules)
	}
}

func TestExecuteReverseProxyDecodesSyntheticRule(t *testing.T) {
	// The lab has zero rules; this synthetic entry (spec-derived shape) exercises
	// the per-rule decoder. It must map the frontend/backend/flags and report the
	// certificate as presence-only and headers as a count.
	exec := &recordingExecutor{responses: map[string]json.RawMessage{
		ReverseProxyAPIName + ".list": json.RawMessage(`{"entries":[{"uuid":"rp-1","description":"media","frontend":{"protocol":"https","fqdn":"media.example.com","port":443,"hsts":true,"http2":true,"certificate":"cert-abc"},"backend":{"protocol":"http","fqdn":"127.0.0.1","port":8096},"customize_headers":[{"name":"X-Real-IP","value":"$remote_addr"},{"name":"Upgrade","value":"$http_upgrade"}]}]}`),
	}}
	rules, _, err := ExecuteReverseProxyRules(context.Background(), lpTarget(), exec)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if rules.Total != 1 || len(rules.Rules) != 1 {
		t.Fatalf("rules = %#v", rules)
	}
	r := rules.Rules[0]
	if r.UUID != "rp-1" || r.Description != "media" {
		t.Fatalf("rule = %#v", r)
	}
	if r.Frontend.Protocol != "https" || r.Frontend.Hostname != "media.example.com" || r.Frontend.Port != 443 {
		t.Fatalf("frontend = %#v", r.Frontend)
	}
	if r.Backend.Protocol != "http" || r.Backend.Hostname != "127.0.0.1" || r.Backend.Port != 8096 {
		t.Fatalf("backend = %#v", r.Backend)
	}
	if !r.HSTSEnabled || !r.HTTP2Enabled || !r.CertificatePresent || r.CustomHeaderCount != 2 {
		t.Fatalf("flags = %#v", r)
	}
}

func TestExecuteReverseProxyRejectsUnknownShape(t *testing.T) {
	exec := &recordingExecutor{responses: map[string]json.RawMessage{
		ReverseProxyAPIName + ".list": json.RawMessage(`{"something":true}`),
	}}
	if _, _, err := ExecuteReverseProxyRules(context.Background(), lpTarget(), exec); err == nil || !strings.Contains(err.Error(), "no entries array") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecuteReverseProxyRejectsNonArrayEntries(t *testing.T) {
	exec := &recordingExecutor{responses: map[string]json.RawMessage{
		ReverseProxyAPIName + ".list": json.RawMessage(`{"entries":{"nope":1}}`),
	}}
	if _, _, err := ExecuteReverseProxyRules(context.Background(), lpTarget(), exec); err == nil || !strings.Contains(err.Error(), "not an array") {
		t.Fatalf("error = %v", err)
	}
}
