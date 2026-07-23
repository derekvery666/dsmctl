package synology

import (
	"testing"

	"github.com/derekvery666/dsmctl/internal/domain/identity"
)

func TestApplicationPrivilegePrincipalsExpandsOnlySelectedUserGroups(t *testing.T) {
	state := IdentityState{
		Users:  []identity.User{{Name: "alice"}, {Name: "bob"}},
		Groups: []identity.Group{{Name: "users"}, {Name: "dev"}, {Name: "sales"}},
		Memberships: []identity.Membership{
			{User: "alice", Groups: []string{"users", "dev"}},
			{User: "bob", Groups: []string{"users", "sales"}},
		},
	}
	targets, err := applicationPrivilegePrincipals(state, identity.StateQuery{
		PrincipalType: identity.PrincipalUser, Principal: "alice",
		IncludeRelatedGroupApplicationPrivileges: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []identityPrincipal{
		{kind: identity.PrincipalUser, name: "alice"},
		{kind: identity.PrincipalGroup, name: "users"},
		{kind: identity.PrincipalGroup, name: "dev"},
	}
	if len(targets) != len(want) {
		t.Fatalf("targets=%#v, want %#v", targets, want)
	}
	for index := range want {
		if targets[index] != want[index] {
			t.Fatalf("targets[%d]=%#v, want %#v", index, targets[index], want[index])
		}
	}
}
