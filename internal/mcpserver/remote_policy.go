package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/derekvery666/dsmctl/internal/application"
	"github.com/derekvery666/dsmctl/internal/remotepolicy"
)

// NewRemote adds enforceable request policy to the complete MCP surface. New
// remains the local CLI/stdio server and is intentionally unaffected. adminURL
// is the gateway console's public origin (may be empty); it is used only to
// build credential-enrollment deep links in tool guidance, never as a secret.
func NewRemote(service *application.Service, version string, auditor remotepolicy.Auditor, adminURL string) *mcp.Server {
	server := New(service, version, WithAdminURL(adminURL))
	// get_certificate_export is named like a read but writes a certificate
	// archive containing the PRIVATE KEY to the gateway HOST's filesystem at a
	// caller-controlled path. The prefix-based ToolScope classifies any get_ tool
	// as ScopeRead, so without this it would be reachable by a nas.read-only
	// remote token. Exporting a private key to the gateway host is meaningless for
	// a remote caller, so it is stripped from the remote surface entirely — the
	// same posture NewReadOnly takes.
	server.RemoveTools("get_certificate_export")
	server.AddReceivingMiddleware(remotePolicyMiddleware(service, auditor))
	return server
}

// ToolScope is the authorization table for the MCP surface. Prefix-based
// grouping is deliberate and tested against every registered tool: planning
// and applying are independent scopes even when a plan tool is read-only at
// the DSM protocol level.
func ToolScope(name string) (string, bool) {
	switch {
	case name == "discover_lan_devices":
		return remotepolicy.ScopeLANDiscover, true
	case strings.HasPrefix(name, "provision_"), strings.HasPrefix(name, "install_"):
		return remotepolicy.ScopeProvision, true
	case name == "run_security_scan":
		// A load-heavy NAS action with no plan/apply cycle. It mutates no
		// configuration but must not be reachable by a read-only token, so it is
		// classified under the apply scope alongside the other write actions.
		return remotepolicy.ScopeApply, true
	case strings.HasPrefix(name, "plan_"):
		return remotepolicy.ScopePlan, true
	case strings.HasPrefix(name, "apply_"):
		return remotepolicy.ScopeApply, true
	case name == "list_nas", strings.HasPrefix(name, "get_"), strings.HasPrefix(name, "explain_"):
		return remotepolicy.ScopeRead, true
	default:
		return "", false
	}
}

type remotePlanResult struct {
	Plan struct {
		NAS                   string   `json:"nas"`
		ProfileRevision       uint64   `json:"profile_revision"`
		SourceNAS             string   `json:"source_nas"`
		SourceProfileRevision uint64   `json:"source_profile_revision"`
		Hash                  string   `json:"hash"`
		Risk                  string   `json:"risk"`
		Summary               []string `json:"summary"`
		References            struct {
			ResourceID string `json:"resource_id"`
		} `json:"references"`
	} `json:"plan"`
}

type remoteToolTarget struct {
	NAS       string `json:"nas"`
	SourceNAS string `json:"source_nas"`
	DestNAS   string `json:"dest_nas"`
	URL       string `json:"url"`
	Plan      struct {
		NAS                   string   `json:"nas"`
		ProfileRevision       uint64   `json:"profile_revision"`
		SourceNAS             string   `json:"source_nas"`
		SourceProfileRevision uint64   `json:"source_profile_revision"`
		DestNAS               string   `json:"dest_nas"`
		Hash                  string   `json:"hash"`
		Risk                  string   `json:"risk"`
		Summary               []string `json:"summary"`
	} `json:"plan"`
}

func remotePolicyMiddleware(service *application.Service, auditor remotepolicy.Auditor) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			ctx, principal, remote := currentRemotePrincipal(ctx, request)
			if !remote {
				return nil, remotepolicy.ErrDenied
			}
			if method == "tools/list" {
				result, err := next(ctx, method, request)
				if err != nil {
					return result, err
				}
				listed, ok := result.(*mcp.ListToolsResult)
				if !ok {
					return result, nil
				}
				allowed := listed.Tools[:0]
				for _, tool := range listed.Tools {
					scope, known := ToolScope(tool.Name)
					if known && principal.HasScope(scope) {
						allowed = append(allowed, tool)
					}
				}
				listed.Tools = allowed
				return result, nil
			}
			if method != "tools/call" {
				return next(ctx, method, request)
			}
			params, ok := request.GetParams().(*mcp.CallToolParamsRaw)
			if !ok {
				return nil, remotepolicy.ErrDenied
			}
			scope, known := ToolScope(params.Name)
			if !known || !principal.HasScope(scope) {
				auditRemote(ctx, auditor, principal, params.Name, "", "denied", "denied")
				return nil, remotepolicy.ErrDenied
			}
			var target remoteToolTarget
			if len(params.Arguments) > 0 && string(params.Arguments) != "null" {
				if err := json.Unmarshal(params.Arguments, &target); err != nil {
					return next(ctx, method, request)
				}
			}
			nas := target.NAS
			secondaryNAS := ""
			if params.Name == "plan_snapshot_replication_create" {
				nas, secondaryNAS = target.SourceNAS, target.DestNAS
			}
			if strings.HasPrefix(params.Name, "apply_") {
				nas = target.Plan.NAS
				if params.Name == "apply_snapshot_replication_create" {
					nas, secondaryNAS = target.Plan.SourceNAS, target.Plan.DestNAS
				}
			}
			// provision_discovered_nas / install_discovered_nas target a
			// not-yet-enrolled device by url, so they have no profile to resolve
			// against the allowlist; the nas.provision scope (already checked above)
			// is their authorization and the application layer restricts them to LAN
			// addresses. Audit by url.
			if params.Name == "provision_discovered_nas" || params.Name == "install_discovered_nas" {
				result, err := next(ctx, method, request)
				outcome := "success"
				if err != nil {
					outcome = "failure"
				}
				if callResult, ok := result.(*mcp.CallToolResult); ok && callResult != nil && callResult.IsError {
					outcome = "failure"
				}
				auditRemote(ctx, auditor, principal, params.Name, target.URL, outcome, "")
				return result, err
			}
			needsTarget := params.Name != "list_nas" && params.Name != "discover_lan_devices" && !(params.Name == "get_auth_status" && nas == "")
			if needsTarget {
				if strings.TrimSpace(nas) == "" {
					auditRemote(ctx, auditor, principal, params.Name, "", "denied", "denied")
					return nil, fmt.Errorf("remote MCP tool %q requires an explicit nas argument", params.Name)
				}
				resolved, err := service.AuthorizeRemoteTarget(ctx, nas)
				if err != nil {
					auditRemote(ctx, auditor, principal, params.Name, "", "denied", "denied")
					return nil, remotepolicy.ErrDenied
				}
				nas = resolved
				if strings.TrimSpace(secondaryNAS) != "" {
					if _, err := service.AuthorizeRemoteTarget(ctx, secondaryNAS); err != nil {
						auditRemote(ctx, auditor, principal, params.Name, nas, "denied", "denied")
						return nil, remotepolicy.ErrDenied
					}
				}
			} else if nas != "" && !principal.AllowsNAS(nas) {
				auditRemote(ctx, auditor, principal, params.Name, "", "denied", "denied")
				return nil, remotepolicy.ErrDenied
			}
			result, err := next(ctx, method, request)
			if callResult, ok := result.(*mcp.CallToolResult); ok &&
				callResult != nil &&
				interactiveApprovalRequired(callResult) {
				result, err = elicitHighRiskApproval(ctx, request, next, method, principal, target)
			}
			outcome := "success"
			if err != nil {
				outcome = "failure"
			}
			// A dispatch to an unknown/removed tool returns a typed-nil
			// *mcp.CallToolResult boxed in the result interface (ok is true,
			// callResult is nil), so the nil check is required before IsError —
			// without it a remote tools/call for any unregistered name panics
			// this middleware, crashing the gateway process.
			if callResult, ok := result.(*mcp.CallToolResult); ok && callResult != nil && callResult.IsError {
				outcome = "failure"
			}
			if err == nil &&
				outcome == "success" &&
				strings.HasPrefix(params.Name, "plan_") &&
				principal.ApprovalMode != remotepolicy.ApprovalModeInteractive {
				recordPendingApproval(ctx, auditor, principal, params.Name, result)
			}
			auditRemote(ctx, auditor, principal, params.Name, nas, outcome, "")
			return result, err
		}
	}
}

func currentRemotePrincipal(ctx context.Context, request mcp.Request) (context.Context, remotepolicy.Principal, bool) {
	extra := request.GetExtra()
	if extra != nil && extra.TokenInfo != nil {
		value, ok := extra.TokenInfo.Extra[remotepolicy.MCPPrincipalTokenInfoKey]
		principal, principalOK := value.(remotepolicy.Principal)
		if !ok || !principalOK || principal.TokenID == "" || principal.TokenID != extra.TokenInfo.UserID {
			return ctx, remotepolicy.Principal{}, false
		}
		ctx = remotepolicy.WithPrincipal(ctx, principal)
		return ctx, principal, true
	}
	principal, ok := remotepolicy.PrincipalFromContext(ctx)
	return ctx, principal, ok
}

func interactiveApprovalRequired(result *mcp.CallToolResult) bool {
	if result == nil || !result.IsError {
		return false
	}
	if errors.Is(result.GetError(), remotepolicy.ErrInteractiveApprovalRequired) {
		return true
	}
	expected := remotepolicy.ErrInteractiveApprovalRequired.Error()
	for _, item := range result.Content {
		if text, ok := item.(*mcp.TextContent); ok && text.Text == expected {
			return true
		}
	}
	return false
}

func elicitHighRiskApproval(
	ctx context.Context,
	request mcp.Request,
	next mcp.MethodHandler,
	method string,
	principal remotepolicy.Principal,
	target remoteToolTarget,
) (mcp.Result, error) {
	session, ok := request.GetSession().(*mcp.ServerSession)
	if !ok || session == nil || strings.TrimSpace(session.ID()) == "" {
		return interactiveApprovalFailure(
			"This MCP client cannot provide a session-bound confirmation. Use a client that supports MCP form elicitation, or change this credential to administrator approval mode.",
			errors.New("interactive high-risk approval requires a stateful MCP session"),
		), nil
	}
	nas, revision := target.Plan.NAS, target.Plan.ProfileRevision
	if nas == "" {
		nas, revision = target.Plan.SourceNAS, target.Plan.SourceProfileRevision
	}
	if nas == "" || revision == 0 || target.Plan.Hash == "" {
		return interactiveApprovalFailure(
			"The high-risk plan is missing approval-binding metadata. Create a new plan and try again.",
			errors.New("interactive high-risk approval metadata is incomplete"),
		), nil
	}
	message := highRiskApprovalMessage(nas, target.Plan.Hash, target.Plan.Summary)
	response, err := session.Elicit(ctx, &mcp.ElicitParams{
		Mode:    "form",
		Message: message,
		RequestedSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"approve": map[string]any{
					"type":        "boolean",
					"title":       "Approve this high-risk change",
					"description": "Approve only if the NAS, change summary, and plan identifier above are expected.",
				},
			},
			"required": []string{"approve"},
		},
	})
	if err != nil {
		return interactiveApprovalFailure(
			"This MCP client could not show the high-risk confirmation. No change was made. Use a client that supports MCP form elicitation, or change this credential to administrator approval mode.",
			fmt.Errorf("request interactive high-risk approval: %w", err),
		), nil
	}
	if response == nil {
		return interactiveApprovalFailure(
			"The MCP client returned an invalid high-risk confirmation. No change was made.",
			errors.New("interactive high-risk approval returned no response"),
		), nil
	}
	approved, valid := response.Content["approve"].(bool)
	if response.Action != "accept" || !valid || !approved {
		return interactiveApprovalFailure(
			"High-risk change declined or cancelled. No change was made.",
			errors.New("interactive high-risk approval was not accepted"),
		), nil
	}
	grant := remotepolicy.InteractiveApprovalGrant{
		TokenID: principal.TokenID, SessionID: session.ID(),
		NAS: nas, ProfileRevision: revision, PlanHash: target.Plan.Hash,
	}
	retryContext := remotepolicy.WithMCPSessionID(ctx, session.ID())
	retryContext = remotepolicy.WithInteractiveApproval(retryContext, grant)
	return next(retryContext, method, request)
}

func highRiskApprovalMessage(nas, planHash string, summary []string) string {
	items := make([]string, 0, len(summary))
	for _, item := range summary {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
		if len(items) == 4 {
			break
		}
	}
	change := "No summary was provided."
	if len(items) > 0 {
		change = strings.Join(items, "; ")
	}
	if len(change) > 1200 {
		change = change[:1200] + "…"
	}
	shortHash := planHash
	if len(shortHash) > 12 {
		shortHash = shortHash[:12]
	}
	return fmt.Sprintf(
		"High-risk NAS change\n\nNAS: %s\nChange: %s\nPlan: %s\n\nConfirm only if this is the exact change you requested. Approval applies to this call only.",
		nas, change, shortHash,
	)
}

func interactiveApprovalFailure(message string, err error) *mcp.CallToolResult {
	result := &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: message}},
	}
	result.SetError(err)
	return result
}

func recordPendingApproval(ctx context.Context, target any, principal remotepolicy.Principal, tool string, result mcp.Result) {
	recorder, ok := target.(remotepolicy.ApprovalRequestRecorder)
	if !ok {
		return
	}
	callResult, ok := result.(*mcp.CallToolResult)
	if !ok || callResult == nil || callResult.IsError || callResult.StructuredContent == nil {
		return
	}
	encoded, err := json.Marshal(callResult.StructuredContent)
	if raw, ok := callResult.StructuredContent.(json.RawMessage); ok {
		encoded = raw
		err = nil
	}
	if err != nil {
		return
	}
	var value remotePlanResult
	if json.Unmarshal(encoded, &value) != nil || !strings.EqualFold(value.Plan.Risk, "high") || value.Plan.ProfileRevision == 0 {
		if json.Unmarshal(encoded, &value) != nil || !strings.EqualFold(value.Plan.Risk, "high") || value.Plan.SourceProfileRevision == 0 {
			return
		}
	}
	nas, revision := value.Plan.NAS, value.Plan.ProfileRevision
	if nas == "" {
		nas, revision = value.Plan.SourceNAS, value.Plan.SourceProfileRevision
	}
	_ = recorder.RecordPendingApproval(ctx, remotepolicy.PendingApprovalRequest{
		PlanHash: value.Plan.Hash, NAS: nas, ProfileRevision: revision,
		RequestingTokenID: principal.TokenID, Tool: tool, Risk: value.Plan.Risk,
		ResourceID: value.Plan.References.ResourceID, Summary: strings.Join(value.Plan.Summary, "; "),
	})
}

func auditRemote(ctx context.Context, auditor remotepolicy.Auditor, principal remotepolicy.Principal, tool, nas, outcome, reason string) {
	if auditor == nil {
		return
	}
	_ = auditor.AppendAudit(ctx, remotepolicy.AuditEvent{
		CorrelationID: remotepolicy.CorrelationID(ctx), ActorType: "mcp_token", ActorID: principal.TokenID,
		Action: "mcp.tool", Tool: tool, NAS: nas, Outcome: outcome, Reason: reason,
	})
}
