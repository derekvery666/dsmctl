package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/derekvery666/dsmctl/internal/application"
	"github.com/derekvery666/dsmctl/internal/config"
	"github.com/derekvery666/dsmctl/internal/credentials"
	gatewaystate "github.com/derekvery666/dsmctl/internal/gateway/state"
	"github.com/derekvery666/dsmctl/internal/remotepolicy"
	"github.com/derekvery666/dsmctl/internal/runtime"
)

type approvalRecordingAuditor struct {
	requests []remotepolicy.PendingApprovalRequest
}

func (*approvalRecordingAuditor) AppendAudit(context.Context, remotepolicy.AuditEvent) error {
	return nil
}

func (a *approvalRecordingAuditor) RecordPendingApproval(_ context.Context, request remotepolicy.PendingApprovalRequest) error {
	a.requests = append(a.requests, request)
	return nil
}

func TestHighRiskStructuredPlanRecordsClosedPendingApproval(t *testing.T) {
	auditor := &approvalRecordingAuditor{}
	principal := remotepolicy.Principal{TokenID: "token-1"}
	result := &mcp.CallToolResult{StructuredContent: json.RawMessage(`{"plan":{"nas":"office","profile_revision":42,"hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","risk":"high","summary":["delete pool","verify topology"],"references":{"resource_id":"pool-7"},"observed":{"password":"secret-canary-must-not-persist"}}}`)}
	recordPendingApproval(context.Background(), auditor, principal, "plan_storage_change", result)
	if len(auditor.requests) != 1 {
		t.Fatalf("pending requests = %#v", auditor.requests)
	}
	request := auditor.requests[0]
	if request.RequestingTokenID != "token-1" || request.NAS != "office" || request.ProfileRevision != 42 || request.ResourceID != "pool-7" || request.Summary != "delete pool; verify topology" {
		t.Fatalf("pending request = %#v", request)
	}
	encoded, _ := json.Marshal(request)
	for _, forbidden := range []string{"observed", "password", "synotoken", "ciphertext", "secret-canary-must-not-persist"} {
		if strings.Contains(strings.ToLower(string(encoded)), forbidden) {
			t.Fatalf("pending request contains forbidden plan material %q: %s", forbidden, encoded)
		}
	}
}

func TestInteractiveApprovalRequiredSurvivesSDKResultAdaptation(t *testing.T) {
	original := &mcp.CallToolResult{}
	original.SetError(remotepolicy.ErrInteractiveApprovalRequired)
	if !interactiveApprovalRequired(original) {
		t.Fatal("typed sentinel was not detected")
	}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var adapted mcp.CallToolResult
	if err := json.Unmarshal(encoded, &adapted); err != nil {
		t.Fatal(err)
	}
	if adapted.GetError() != nil {
		t.Fatal("test no longer models SDK error-identity loss")
	}
	if !interactiveApprovalRequired(&adapted) {
		t.Fatal("SDK-adapted exact sentinel text was not detected")
	}
	adapted.Content = []mcp.Content{&mcp.TextContent{Text: "wrapped: " + remotepolicy.ErrInteractiveApprovalRequired.Error()}}
	if interactiveApprovalRequired(&adapted) {
		t.Fatal("substring error text was accepted as the exact sentinel")
	}
	adapted.IsError = false
	adapted.Content = []mcp.Content{&mcp.TextContent{Text: remotepolicy.ErrInteractiveApprovalRequired.Error()}}
	if interactiveApprovalRequired(&adapted) {
		t.Fatal("successful result text triggered interactive approval")
	}
}

func TestHighRiskReplicationPlanRecordsSourceApproval(t *testing.T) {
	auditor := &approvalRecordingAuditor{}
	principal := remotepolicy.Principal{TokenID: "token-1"}
	result := &mcp.CallToolResult{StructuredContent: json.RawMessage(`{"plan":{"source_nas":"source","source_profile_revision":17,"dest_nas":"dest","dest_profile_revision":23,"hash":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","risk":"high","summary":["replicate source to dest"]}}`)}
	recordPendingApproval(context.Background(), auditor, principal, "plan_snapshot_replication_create", result)
	if len(auditor.requests) != 1 {
		t.Fatalf("pending requests = %#v", auditor.requests)
	}
	request := auditor.requests[0]
	if request.NAS != "source" || request.ProfileRevision != 17 || request.PlanHash == "" {
		t.Fatalf("pending request = %#v", request)
	}
}

func TestStructuredPlanSecretCanaryNeverReachesPersistedApprovalState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.db")
	repository, err := gatewaystate.Open(path, bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	canary := "secret-canary-must-not-persist"
	result := &mcp.CallToolResult{StructuredContent: json.RawMessage(`{"plan":{"nas":"office","profile_revision":42,"hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","risk":"high","summary":["delete pool"],"observed":{"password":"` + canary + `","sid":"` + canary + `"}}}`)}
	recordPendingApproval(context.Background(), repository, remotepolicy.Principal{TokenID: "token-1"}, "plan_storage_change", result)
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(persisted, []byte(canary)) {
		t.Fatal("structured plan secret canary reached persisted approval state")
	}
}

func TestEveryRemoteTargetedToolRejectsOmittedNAS(t *testing.T) {
	service := &application.Service{}
	server := New(service, "test")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "target-test", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	principal := remotepolicy.Principal{TokenID: "all-scopes", Scopes: map[string]struct{}{remotepolicy.ScopeRead: {}, remotepolicy.ScopePlan: {}, remotepolicy.ScopeApply: {}, remotepolicy.ScopeLANDiscover: {}, remotepolicy.ScopeProvision: {}}, NAS: map[string]struct{}{}}
	remoteContext := remotepolicy.WithPrincipal(context.Background(), principal)
	for _, tool := range tools.Tools {
		if strings.Contains(tool.Name, "approval") {
			t.Fatalf("MCP exposes administrator-only approval state through %q", tool.Name)
		}
		called := false
		next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
			called = true
			return &mcp.CallToolResult{}, nil
		}
		handler := remotePolicyMiddleware(service, nil)(next)
		request := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: tool.Name, Arguments: json.RawMessage(`{}`)}}
		_, err := handler(remoteContext, "tools/call", request)
		targetless := tool.Name == "list_nas" || tool.Name == "discover_lan_devices" || tool.Name == "get_auth_status" || tool.Name == "provision_discovered_nas" || tool.Name == "install_discovered_nas"
		if targetless {
			if err != nil || !called {
				t.Errorf("targetless tool %q err=%v called=%v", tool.Name, err, called)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), "explicit nas") || called {
			t.Errorf("targeted tool %q err=%v called=%v", tool.Name, err, called)
		}
	}
}

func TestSnapshotReplicationRemotePolicyAuthorizesBothSites(t *testing.T) {
	cfg := &config.Config{NAS: map[string]config.Profile{
		"source": {URL: "https://source.example"},
		"dest":   {URL: "https://dest.example"},
	}}
	service := application.NewService(cfg, runtime.NewManager(cfg, credentials.NewEnvironment()))
	called := false
	next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
		called = true
		return &mcp.CallToolResult{}, nil
	}
	handler := remotePolicyMiddleware(service, nil)(next)
	request := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      "plan_snapshot_replication_create",
		Arguments: json.RawMessage(`{"source_nas":"source","dest_nas":"dest","request":{"source_share":"data","dest_volume":"/volume1"}}`),
	}}

	allowed := remotepolicy.Principal{
		TokenID: "operator",
		Scopes:  map[string]struct{}{remotepolicy.ScopePlan: {}},
		NAS:     map[string]struct{}{"source": {}, "dest": {}},
	}
	if _, err := handler(remotepolicy.WithPrincipal(context.Background(), allowed), "tools/call", request); err != nil || !called {
		t.Fatalf("both-site authorization err=%v called=%v", err, called)
	}

	called = false
	sourceOnly := allowed
	sourceOnly.NAS = map[string]struct{}{"source": {}}
	if _, err := handler(remotepolicy.WithPrincipal(context.Background(), sourceOnly), "tools/call", request); !errors.Is(err, remotepolicy.ErrDenied) || called {
		t.Fatalf("destination allowlist bypass err=%v called=%v", err, called)
	}

	called = false
	applyRequest := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      "apply_snapshot_replication_create",
		Arguments: json.RawMessage(`{"plan":{"source_nas":"source","dest_nas":"dest"},"approval_hash":"hash"}`),
	}}
	applyAllowed := allowed
	applyAllowed.Scopes = map[string]struct{}{remotepolicy.ScopeApply: {}}
	if _, err := handler(remotepolicy.WithPrincipal(context.Background(), applyAllowed), "tools/call", applyRequest); err != nil || !called {
		t.Fatalf("two-site apply authorization err=%v called=%v", err, called)
	}
}

// TestRemotePolicyMiddlewareToleratesTypedNilResult is a regression test for a
// remote denial-of-service: a tools/call for an unknown or removed tool makes
// the go-sdk return a typed-nil *mcp.CallToolResult boxed in the result
// interface (the assertion succeeds but the pointer is nil). The policy
// middleware must nil-check before dereferencing IsError — the panic would run
// in a bare goroutine with no recover and crash the entire gateway process,
// reachable by any principal holding a valid read token.
func TestRemotePolicyMiddlewareToleratesTypedNilResult(t *testing.T) {
	service := &application.Service{}
	principal := remotepolicy.Principal{
		TokenID: "reader",
		Scopes:  map[string]struct{}{remotepolicy.ScopeRead: {}},
		NAS:     map[string]struct{}{},
	}
	remoteContext := remotepolicy.WithPrincipal(context.Background(), principal)
	next := func(context.Context, string, mcp.Request) (mcp.Result, error) {
		// The typed-nil the SDK produces for an unknown-tool dispatch.
		var typedNil *mcp.CallToolResult
		return typedNil, nil
	}
	// get_auth_status is read-scoped and targetless, so it reaches next; the
	// fake next stands in for the unknown-tool typed-nil result.
	handler := remotePolicyMiddleware(service, nil)(next)
	request := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: "get_auth_status", Arguments: json.RawMessage(`{}`)}}
	if _, err := handler(remoteContext, "tools/call", request); err != nil {
		t.Fatalf("unexpected error (must not panic): %v", err)
	}
}
