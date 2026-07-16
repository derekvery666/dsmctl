package access

const (
	ResourceShare       = "share"
	ResourceApplication = "application"

	AccessNone          = "none"
	AccessRead          = "read"
	AccessWrite         = "write"
	AccessAllow         = "allow"
	AccessDeny          = "deny"
	AccessIndeterminate = "indeterminate"

	SourceDirect  = "direct"
	SourceGroup   = "group"
	SourcePreview = "dsm_preview"
)

// Query identifies the one principal and one resource whose effective access
// should be explained. The first implementation supports local DSM users and
// groups and never evaluates filesystem ACLs below a shared-folder root.
type Query struct {
	PrincipalType string `json:"principal_type" jsonschema:"Principal kind: user or group"`
	Principal     string `json:"principal" jsonschema:"Local DSM user or group name"`
	ResourceType  string `json:"resource_type" jsonschema:"Resource kind: share or application"`
	Resource      string `json:"resource" jsonschema:"Shared-folder name or application ID"`
}

// Explanation is shared unchanged by the CLI and MCP surfaces. Determinate is
// false when custom/masked ACLs, Advanced Share Permissions, the homes
// filesystem default, or IP-aware application rules prevent a safe answer.
type Explanation struct {
	PrincipalType   string     `json:"principal_type" jsonschema:"Principal kind: user or group"`
	Principal       string     `json:"principal" jsonschema:"Local DSM user or group name"`
	ResourceType    string     `json:"resource_type" jsonschema:"Resource kind: share or application"`
	Resource        string     `json:"resource" jsonschema:"Shared-folder name or application ID"`
	EffectiveAccess string     `json:"effective_access" jsonschema:"Effective access: none, read, write, allow, deny, or indeterminate"`
	Determinate     bool       `json:"determinate" jsonschema:"Whether dsmctl has enough modeled evidence to make a final access decision"`
	Summary         string     `json:"summary" jsonschema:"Concise human-readable explanation"`
	Evidence        []Evidence `json:"evidence" jsonschema:"Every direct/group rule observed plus DSM's authoritative application preview when applicable"`
	Limitations     []string   `json:"limitations,omitempty" jsonschema:"Unmodeled rules that prevent a safe conclusion"`
}

type Evidence struct {
	Source          string   `json:"source" jsonschema:"Rule source: direct, group, or dsm_preview"`
	PrincipalType   string   `json:"principal_type" jsonschema:"Principal kind that owns the rule"`
	Principal       string   `json:"principal" jsonschema:"Principal that owns the rule"`
	Access          string   `json:"access" jsonschema:"Normalized observed rule"`
	Inherited       bool     `json:"inherited,omitempty" jsonschema:"Whether DSM marked the rule as inherited"`
	InheritedAccess string   `json:"inherited_access,omitempty" jsonschema:"DSM-computed inherited group aggregate attached to a direct user rule"`
	Custom          bool     `json:"custom,omitempty" jsonschema:"Whether DSM reported an unmodeled custom rule"`
	Masked          bool     `json:"masked,omitempty" jsonschema:"Whether DSM reported a masked share rule"`
	AllowIP         []string `json:"allow_ip,omitempty" jsonschema:"Observed application allow IP ranges"`
	DenyIP          []string `json:"deny_ip,omitempty" jsonschema:"Observed application deny IP ranges"`
	Reason          string   `json:"reason" jsonschema:"How this rule contributes to the explanation"`
}
