package application

import (
	"strings"
	"testing"

	"github.com/derekvery666/dsmctl/internal/domain/access"
	"github.com/derekvery666/dsmctl/internal/domain/identity"
)

func TestValidateAccessQuery(t *testing.T) {
	valid := access.Query{PrincipalType: identity.PrincipalUser, Principal: "alice", ResourceType: access.ResourceShare, Resource: "projects"}
	if err := validateAccessQuery(valid); err != nil {
		t.Fatalf("validateAccessQuery(valid): %v", err)
	}
	for _, test := range []struct {
		name  string
		query access.Query
		want  string
	}{
		{name: "principal type", query: access.Query{Principal: "alice", ResourceType: access.ResourceShare, Resource: "projects"}, want: "principal_type"},
		{name: "principal", query: access.Query{PrincipalType: identity.PrincipalUser, ResourceType: access.ResourceShare, Resource: "projects"}, want: "principal is required"},
		{name: "resource type", query: access.Query{PrincipalType: identity.PrincipalUser, Principal: "alice", Resource: "projects"}, want: "resource_type"},
		{name: "resource", query: access.Query{PrincipalType: identity.PrincipalUser, Principal: "alice", ResourceType: access.ResourceShare}, want: "resource is required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateAccessQuery(test.query); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateAccessQuery() error=%v, want %q", err, test.want)
			}
		})
	}
}

func TestAccessPrincipalContextUsesOnlySelectedUserMembership(t *testing.T) {
	state := identity.State{
		Users: []identity.User{{Name: "Alice"}, {Name: "bob"}},
		Memberships: []identity.Membership{
			{User: "Alice", Groups: []string{"users", "dev"}},
			{User: "bob", Groups: []string{"users", "sales"}},
		},
	}
	name, groups, err := accessPrincipalContext(state, identity.PrincipalUser, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if name != "Alice" || len(groups) != 2 || groups[1] != "dev" {
		t.Fatalf("accessPrincipalContext() = %q %v", name, groups)
	}
}
