// Package remotepolicy contains transport-neutral identity and admission
// metadata for requests that entered through the remote gateway. Local CLI and
// stdio calls deliberately carry no Principal and keep their existing policy.
package remotepolicy

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

const (
	ScopeRead        = "nas.read"
	ScopePlan        = "nas.plan"
	ScopeApply       = "nas.apply"
	ScopeLANDiscover = "lan.discover"
	// MCPPrincipalTokenInfoKey is the private in-process TokenInfo.Extra key
	// shared by the authenticated HTTP boundary and the remote MCP policy
	// middleware. The value is a freshly authenticated Principal for the
	// current HTTP request; it is never accepted from MCP tool input.
	MCPPrincipalTokenInfoKey = "dsmctl.remote-principal.v1"
	// ApprovalModeInteractive asks the connected MCP client to confirm an exact
	// high-risk plan in the conversation that initiated the apply.
	ApprovalModeInteractive = "interactive"
	// ApprovalModeAdministrator preserves the hardened, out-of-band approval
	// path through the gateway administration console.
	ApprovalModeAdministrator = "administrator"
	// ScopeProvision admits creating a fresh NAS's first administrator (WI-086).
	// It is deliberately distinct from ScopeApply: provisioning mints a new
	// credential rather than mutating an enrolled resource, so it is never a
	// sub-privilege of apply and is never granted to a token by default.
	ScopeProvision = "nas.provision"
)

var (
	ErrDenied                      = errors.New("remote request is not authorized")
	ErrInteractiveApprovalRequired = errors.New("interactive approval is required for this exact high-risk plan")
)

type Principal struct {
	TokenID      string
	Name         string
	ApprovalMode string
	Scopes       map[string]struct{}
	NAS          map[string]struct{}
}

// InteractiveApprovalGrant is transport-issued, request-local evidence that
// the human using the authenticated MCP session accepted one exact high-risk
// plan. The unexported context key prevents tool arguments or HTTP headers from
// fabricating a grant. It is deliberately never persisted or returned to the
// model.
type InteractiveApprovalGrant struct {
	TokenID         string
	SessionID       string
	NAS             string
	ProfileRevision uint64
	PlanHash        string
	used            *atomic.Bool
}

// AuditEvent intentionally has a closed, scalar schema: callers cannot attach
// request bodies, headers, credentials, DSM responses, or ciphertext.
type AuditEvent struct {
	ID            string    `json:"id"`
	Time          time.Time `json:"time"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	ActorType     string    `json:"actor_type"`
	ActorID       string    `json:"actor_id,omitempty"`
	Action        string    `json:"action"`
	Tool          string    `json:"tool,omitempty"`
	NAS           string    `json:"nas,omitempty"`
	Outcome       string    `json:"outcome"`
	Reason        string    `json:"reason,omitempty"`
}

type Auditor interface {
	AppendAudit(context.Context, AuditEvent) error
}

// PendingApprovalRequest is the closed, non-secret subset of a typed plan
// result that the remote gateway may expose to its local administrator. It
// deliberately cannot carry the plan payload, request body, or DSM response.
type PendingApprovalRequest struct {
	PlanHash          string
	NAS               string
	ProfileRevision   uint64
	RequestingTokenID string
	Tool              string
	Risk              string
	ResourceID        string
	Summary           string
}

// ApprovalRequestRecorder is advisory UI state. Recording failures must never
// weaken or block the existing manual, out-of-band approval path.
type ApprovalRequestRecorder interface {
	RecordPendingApproval(context.Context, PendingApprovalRequest) error
}

func (p Principal) HasScope(scope string) bool {
	_, ok := p.Scopes[scope]
	return ok
}

func (p Principal) AllowsNAS(name string) bool {
	_, ok := p.NAS[name]
	return ok
}

type principalKey struct{}
type correlationKey struct{}
type interactiveApprovalKey struct{}
type mcpSessionKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok && principal.TokenID != ""
}

func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationKey{}, id)
}

func CorrelationID(ctx context.Context) string {
	id, _ := ctx.Value(correlationKey{}).(string)
	return id
}

// WithMCPSessionID records the authenticated transport session in a private
// context slot. Remote input cannot supply this value; the HTTP gateway sets it
// only after authorizing the session/token binding.
func WithMCPSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, mcpSessionKey{}, id)
}

func MCPSessionID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(mcpSessionKey{}).(string)
	return id, ok && id != ""
}

func WithInteractiveApproval(ctx context.Context, grant InteractiveApprovalGrant) context.Context {
	grant.used = &atomic.Bool{}
	return context.WithValue(ctx, interactiveApprovalKey{}, grant)
}

func InteractiveApprovalFromContext(ctx context.Context) (InteractiveApprovalGrant, bool) {
	grant, ok := ctx.Value(interactiveApprovalKey{}).(InteractiveApprovalGrant)
	return grant, ok &&
		grant.TokenID != "" &&
		grant.SessionID != "" &&
		grant.NAS != "" &&
		grant.ProfileRevision != 0 &&
		grant.PlanHash != ""
}

func (g InteractiveApprovalGrant) ConsumeMatches(tokenID, sessionID, nas string, revision uint64, planHash string) bool {
	matches := g.TokenID == tokenID &&
		g.SessionID == sessionID &&
		g.NAS == nas &&
		g.ProfileRevision == revision &&
		g.PlanHash == planHash
	return matches && g.used != nil && g.used.CompareAndSwap(false, true)
}
