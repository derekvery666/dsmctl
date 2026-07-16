package access

import (
	"testing"

	"github.com/ychiu1211/dsmctl/internal/domain/identity"
	"github.com/ychiu1211/dsmctl/internal/domain/share"
)

func TestExplainSharePrecedence(t *testing.T) {
	tests := []struct {
		name        string
		permissions []share.Permission
		groups      []string
		folderName  string
		aclMode     bool
		want        string
		determinate bool
	}{
		{name: "no grant", permissions: []share.Permission{userSharePermission(share.AccessNone, share.AccessNone)}, want: AccessNone, determinate: true},
		{name: "inherited read", permissions: []share.Permission{userSharePermission(share.AccessNone, share.AccessRead), {PrincipalType: share.PrincipalGroup, Principal: "users", Access: share.AccessRead}}, groups: []string{"users"}, want: AccessRead, determinate: true},
		{name: "inherited write outranks direct read", permissions: []share.Permission{userSharePermission(share.AccessRead, share.AccessWrite), {PrincipalType: share.PrincipalGroup, Principal: "dev", Access: share.AccessWrite}}, groups: []string{"dev"}, want: AccessWrite, determinate: true},
		{name: "inherited deny outranks direct write", permissions: []share.Permission{userSharePermission(share.AccessWrite, share.AccessDeny), {PrincipalType: share.PrincipalGroup, Principal: "blocked", Access: share.AccessDeny}}, groups: []string{"blocked"}, want: AccessDeny, determinate: true},
		{name: "direct deny outranks inherited custom", permissions: []share.Permission{userSharePermission(share.AccessDeny, share.AccessCustom)}, want: AccessDeny, determinate: true},
		{name: "custom is unknown", permissions: []share.Permission{userSharePermission(share.AccessCustom, share.AccessNone)}, want: AccessIndeterminate, determinate: false},
		{name: "masked is unknown", permissions: []share.Permission{userSharePermission(share.AccessNone, share.AccessWrite), {PrincipalType: share.PrincipalGroup, Principal: "dev", Access: share.AccessWrite, Masked: true}}, groups: []string{"dev"}, want: AccessIndeterminate, determinate: false},
		{name: "administrator ACL special case", permissions: []share.Permission{userSharePermission(share.AccessNone, share.AccessRead), {PrincipalType: share.PrincipalGroup, Principal: "administrators", Access: share.AccessRead, ACLMode: true}}, groups: []string{"administrators"}, aclMode: true, want: AccessWrite, determinate: true},
		{name: "homes default is outside root model", permissions: []share.Permission{userSharePermission(share.AccessNone, share.AccessNone)}, folderName: "homes", want: AccessIndeterminate, determinate: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			folderName := test.folderName
			if folderName == "" {
				folderName = "projects"
			}
			got := ExplainShare(Query{PrincipalType: identity.PrincipalUser, Principal: "alice", ResourceType: ResourceShare, Resource: folderName}, test.groups, share.SharedFolder{Name: folderName, ACLMode: test.aclMode, Permissions: test.permissions})
			if got.EffectiveAccess != test.want || got.Determinate != test.determinate {
				t.Fatalf("ExplainShare() access=%q determinate=%v, want %q/%v", got.EffectiveAccess, got.Determinate, test.want, test.determinate)
			}
			if len(got.Evidence) != len(test.permissions) {
				t.Fatalf("ExplainShare() evidence=%d, want %d", len(got.Evidence), len(test.permissions))
			}
		})
	}
}

func TestExplainShareIgnoresUnrelatedPrincipals(t *testing.T) {
	got := ExplainShare(Query{PrincipalType: identity.PrincipalUser, Principal: "alice", ResourceType: ResourceShare, Resource: "projects"}, []string{"dev"}, share.SharedFolder{Permissions: []share.Permission{
		userSharePermission(share.AccessNone, share.AccessRead),
		{PrincipalType: share.PrincipalUser, Principal: "bob", Access: share.AccessDeny},
		{PrincipalType: share.PrincipalGroup, Principal: "sales", Access: share.AccessDeny},
		{PrincipalType: share.PrincipalGroup, Principal: "dev", Access: share.AccessRead},
	}})
	if got.EffectiveAccess != AccessRead || len(got.Evidence) != 2 || got.Evidence[1].Principal != "dev" {
		t.Fatalf("ExplainShare() = %#v", got)
	}
}

func TestExplainShareGroupAdministratorACLMode(t *testing.T) {
	got := ExplainShare(Query{PrincipalType: identity.PrincipalGroup, Principal: "administrators", ResourceType: ResourceShare, Resource: "projects"}, nil, share.SharedFolder{ACLMode: true, Permissions: []share.Permission{
		{PrincipalType: share.PrincipalGroup, Principal: "administrators", Access: share.AccessNone, ACLMode: true},
	}})
	if got.EffectiveAccess != AccessWrite || !got.Determinate {
		t.Fatalf("ExplainShare(group administrators) = %#v", got)
	}
}

func TestExplainShareAdvancedPermissionsAreIndeterminate(t *testing.T) {
	got := ExplainShare(Query{PrincipalType: identity.PrincipalUser, Principal: "alice", ResourceType: ResourceShare, Resource: "projects"}, nil, share.SharedFolder{
		Name: "projects", UnifiedPermissions: true,
		Permissions: []share.Permission{userSharePermission(share.AccessWrite, share.AccessNone)},
	})
	if got.EffectiveAccess != AccessIndeterminate || got.Determinate || len(got.Limitations) != 1 {
		t.Fatalf("ExplainShare(advanced permissions) = %#v", got)
	}
}

func userSharePermission(direct, inherited string) share.Permission {
	return share.Permission{
		PrincipalType:       share.PrincipalUser,
		Principal:           "alice",
		Access:              direct,
		Inherited:           inherited != share.AccessNone,
		InheritedAccess:     inherited,
		InheritanceObserved: true,
		Custom:              direct == share.AccessCustom,
	}
}

func TestExplainApplicationPrecedenceAndInheritance(t *testing.T) {
	tests := []struct {
		name        string
		assignments []identity.ApplicationPrivilegeAssignment
		preview     string
		want        string
		determinate bool
	}{
		{name: "default policy allow", preview: identity.ApplicationAccessAllow, want: AccessAllow, determinate: true},
		{name: "group allow", assignments: []identity.ApplicationPrivilegeAssignment{{PrincipalType: identity.PrincipalGroup, Principal: "dev", Permissions: []identity.ApplicationPermission{{ApplicationID: "SYNO.FTP", Access: identity.ApplicationAccessAllow}}}}, preview: identity.ApplicationAccessAllow, want: AccessAllow, determinate: true},
		{name: "normalized whole-network sentinel remains allow", assignments: []identity.ApplicationPrivilegeAssignment{{PrincipalType: identity.PrincipalUser, Principal: "alice", Permissions: []identity.ApplicationPermission{{ApplicationID: "SYNO.FTP", Access: identity.ApplicationAccessAllow, AllowIP: []string{"0.0.0.0"}}}}}, preview: identity.ApplicationAccessAllow, want: AccessAllow, determinate: true},
		{name: "deny outranks allow", assignments: []identity.ApplicationPrivilegeAssignment{
			{PrincipalType: identity.PrincipalUser, Principal: "alice", Permissions: []identity.ApplicationPermission{{ApplicationID: "SYNO.FTP", Access: identity.ApplicationAccessAllow}}},
			{PrincipalType: identity.PrincipalGroup, Principal: "blocked", Permissions: []identity.ApplicationPermission{{ApplicationID: "SYNO.FTP", Access: identity.ApplicationAccessDeny}}},
		}, preview: identity.ApplicationAccessDeny, want: AccessDeny, determinate: true},
		{name: "everyone deny overrides direct allow", assignments: []identity.ApplicationPrivilegeAssignment{{PrincipalType: identity.PrincipalUser, Principal: "alice", Permissions: []identity.ApplicationPermission{{ApplicationID: "SYNO.FTP", Access: identity.ApplicationAccessAllow}}}}, preview: identity.ApplicationAccessDeny, want: AccessDeny, determinate: true},
		{name: "IP rule is unknown", assignments: []identity.ApplicationPrivilegeAssignment{{PrincipalType: identity.PrincipalUser, Principal: "alice", Permissions: []identity.ApplicationPermission{{ApplicationID: "SYNO.FTP", Access: identity.ApplicationAccessCustom, AllowIP: []string{"10.0.0.0/8"}}}}}, preview: identity.ApplicationAccessCustom, want: AccessIndeterminate, determinate: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preview := identity.ApplicationPrivilegeAssignment{PrincipalType: identity.PrincipalUser, Principal: "alice"}
			if test.preview != "" {
				preview.Permissions = []identity.ApplicationPermission{{ApplicationID: "SYNO.FTP", Access: test.preview}}
			}
			got := ExplainApplication(Query{PrincipalType: identity.PrincipalUser, Principal: "alice", ResourceType: ResourceApplication, Resource: "SYNO.FTP"}, []string{"dev", "blocked"}, test.assignments, preview)
			if got.EffectiveAccess != test.want || got.Determinate != test.determinate {
				t.Fatalf("ExplainApplication() access=%q determinate=%v, want %q/%v", got.EffectiveAccess, got.Determinate, test.want, test.determinate)
			}
			if len(got.Evidence) != 4 {
				t.Fatalf("ExplainApplication() evidence=%d, want direct plus two groups and DSM preview", len(got.Evidence))
			}
		})
	}
}

func TestExplainApplicationRequiresDSMPreviewForFinalDecision(t *testing.T) {
	got := ExplainApplication(
		Query{PrincipalType: identity.PrincipalUser, Principal: "alice", ResourceType: ResourceApplication, Resource: "SYNO.FTP"},
		nil,
		[]identity.ApplicationPrivilegeAssignment{{PrincipalType: identity.PrincipalUser, Principal: "alice", Permissions: []identity.ApplicationPermission{{ApplicationID: "SYNO.FTP", Access: identity.ApplicationAccessAllow}}}},
		identity.ApplicationPrivilegeAssignment{PrincipalType: identity.PrincipalUser, Principal: "alice"},
	)
	if got.EffectiveAccess != AccessIndeterminate || got.Determinate {
		t.Fatalf("ExplainApplication(missing preview) = %#v", got)
	}
}
