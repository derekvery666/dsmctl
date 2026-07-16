package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/ychiu1211/dsmctl/internal/domain/access"
	"github.com/ychiu1211/dsmctl/internal/domain/identity"
	"github.com/ychiu1211/dsmctl/internal/domain/share"
)

type EffectiveAccessResult struct {
	NAS         string             `json:"nas" jsonschema:"NAS profile used for the request"`
	Explanation access.Explanation `json:"explanation" jsonschema:"Effective access decision, evidence, and limitations"`
}

// ExplainEffectiveAccess is a read-only facade shared by CLI and MCP. It first
// reads only the requested principal's membership/application expansion, then
// asks DSM for share permissions only for that principal and contributing
// groups. It never enumerates unrelated principals' permission rules.
func (s *Service) ExplainEffectiveAccess(ctx context.Context, requestedNAS string, query access.Query) (EffectiveAccessResult, error) {
	if err := validateAccessQuery(query); err != nil {
		return EffectiveAccessResult{}, err
	}
	name, client, err := s.manager.Client(ctx, requestedNAS)
	if err != nil {
		return EffectiveAccessResult{}, err
	}

	identityQuery := identity.StateQuery{
		PrincipalType: query.PrincipalType,
		Principal:     query.Principal,
	}
	if query.PrincipalType == identity.PrincipalUser {
		identityQuery.IncludeMemberships = true
	}
	if query.ResourceType == access.ResourceApplication {
		identityQuery.IncludeApplicationPrivileges = true
		identityQuery.IncludeRelatedGroupApplicationPrivileges = query.PrincipalType == identity.PrincipalUser
	}
	state, err := client.IdentityState(ctx, identityQuery)
	if err != nil {
		return EffectiveAccessResult{}, authenticationError(name, err)
	}

	canonicalPrincipal, groups, err := accessPrincipalContext(state, query.PrincipalType, query.Principal)
	if err != nil {
		return EffectiveAccessResult{}, fmt.Errorf("NAS %q: %w", name, err)
	}
	query.Principal = canonicalPrincipal

	var explanation access.Explanation
	switch query.ResourceType {
	case access.ResourceShare:
		principals := []share.Principal{{Type: query.PrincipalType, Name: canonicalPrincipal}}
		for _, group := range groups {
			principals = append(principals, share.Principal{Type: share.PrincipalGroup, Name: group})
		}
		shareState, stateErr := client.ShareStateForPrincipals(ctx, principals)
		if stateErr != nil {
			return EffectiveAccessResult{}, authenticationError(name, stateErr)
		}
		folder, found := findShare(shareState.Shares, query.Resource)
		if !found {
			return EffectiveAccessResult{}, fmt.Errorf("NAS %q: shared folder %q does not exist", name, query.Resource)
		}
		query.Resource = folder.Name
		explanation = access.ExplainShare(query, groups, folder)
	case access.ResourceApplication:
		application, found := findApplication(state.Applications, query.Resource)
		if !found {
			return EffectiveAccessResult{}, fmt.Errorf("NAS %q: application %q does not exist", name, query.Resource)
		}
		query.Resource = application.ID
		preview, previewErr := client.ApplicationPrivilegePreview(ctx, query.PrincipalType, canonicalPrincipal)
		if previewErr != nil {
			return EffectiveAccessResult{}, authenticationError(name, previewErr)
		}
		explanation = access.ExplainApplication(query, groups, state.ApplicationPrivileges, preview)
	}
	return EffectiveAccessResult{NAS: name, Explanation: explanation}, nil
}

func validateAccessQuery(query access.Query) error {
	if query.PrincipalType != identity.PrincipalUser && query.PrincipalType != identity.PrincipalGroup {
		return fmt.Errorf("principal_type must be user or group")
	}
	if strings.TrimSpace(query.Principal) == "" {
		return fmt.Errorf("principal is required")
	}
	if query.ResourceType != access.ResourceShare && query.ResourceType != access.ResourceApplication {
		return fmt.Errorf("resource_type must be share or application")
	}
	if strings.TrimSpace(query.Resource) == "" {
		return fmt.Errorf("resource is required")
	}
	return nil
}

func accessPrincipalContext(state identity.State, principalType, principal string) (string, []string, error) {
	switch principalType {
	case identity.PrincipalUser:
		user, found := findUser(state.Users, principal)
		if !found {
			return "", nil, fmt.Errorf("user %q does not exist", principal)
		}
		membership, found := findMembership(state.Memberships, user.Name)
		if !found {
			return "", nil, fmt.Errorf("membership state for user %q was not returned", user.Name)
		}
		return user.Name, append([]string(nil), membership.Groups...), nil
	case identity.PrincipalGroup:
		group, found := findGroup(state.Groups, principal)
		if !found {
			return "", nil, fmt.Errorf("group %q does not exist", principal)
		}
		return group.Name, nil, nil
	default:
		return "", nil, fmt.Errorf("unsupported principal type %q", principalType)
	}
}
