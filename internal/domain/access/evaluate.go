package access

import (
	"fmt"
	"sort"
	"strings"

	"github.com/derekvery666/dsmctl/internal/domain/identity"
	"github.com/derekvery666/dsmctl/internal/domain/share"
)

// ExplainShare reproduces DSM Admin Center's direct-plus-inherited permission
// table for one shared-folder root. Custom/masked ACLs, the homes default, and
// Advanced Share Permissions remain indeterminate because their action- or
// filesystem-level semantics are not represented by the coarse API.
func ExplainShare(query Query, groups []string, folder share.SharedFolder) Explanation {
	evidence := make([]Evidence, 0, len(groups)+1)
	var direct share.Permission
	directFound := false
	for _, permission := range folder.Permissions {
		source, include := sourceFor(query, groups, permission.PrincipalType, permission.Principal)
		if !include {
			continue
		}
		evidence = append(evidence, Evidence{
			Source: source, PrincipalType: permission.PrincipalType, Principal: permission.Principal,
			Access: permission.Access, Inherited: permission.Inherited, InheritedAccess: permission.InheritedAccess, Custom: permission.Custom,
			Masked: permission.Masked, Reason: shareEvidenceReason(permission),
		})
		if source == SourceDirect {
			direct = permission
			directFound = true
		}
	}
	sortEvidence(evidence)

	result := baseExplanation(query, evidence)
	limitation := shareLimitation(query, folder, direct, directFound, evidence)
	if limitation != "" {
		return indeterminateShare(result, query, limitation)
	}

	var effective string
	if query.PrincipalType == identity.PrincipalGroup {
		effective = groupShareAccess(direct, folder.ACLMode)
	} else {
		effective = userShareAccess(direct, groups, folder)
	}
	if effective == share.AccessCustom {
		return indeterminateShare(result, query, "DSM computed a custom Windows ACL that dsmctl does not model")
	}
	if effective == "default" {
		return indeterminateShare(result, query, "DSM reports the special homes default permission, whose per-home filesystem ACL is outside this shared-folder-root model")
	}
	if effective == "unknown" || effective == "" {
		return indeterminateShare(result, query, "DSM returned an unknown inherited permission code")
	}
	result.Determinate = true
	result.EffectiveAccess = effective
	result.Summary = shareSummary(query, result.EffectiveAccess, evidence)
	return result
}

// ExplainApplication retains explicit user/group rules as evidence and uses
// DSM App.preview as the authoritative final decision because it also includes
// everyone/default and built-in account policy.
func ExplainApplication(query Query, groups []string, assignments []identity.ApplicationPrivilegeAssignment, preview identity.ApplicationPrivilegeAssignment) Explanation {
	appID := query.Resource
	evidence := make([]Evidence, 0, len(groups)+1)
	principals := applicablePrincipals(query, groups)
	for _, principal := range principals {
		permission, found := applicationPermission(assignments, principal.kind, principal.name, appID)
		access := identity.ApplicationAccessInherit
		reason := "no explicit rule was observed; this principal inherits DSM application policy"
		var allowIP, denyIP []string
		if found {
			access = permission.Access
			allowIP = append([]string(nil), permission.AllowIP...)
			denyIP = append([]string(nil), permission.DenyIP...)
			reason = "explicit application privilege rule"
		}
		evidence = append(evidence, Evidence{
			Source: principal.source, PrincipalType: principal.kind, Principal: principal.name,
			Access: access, Custom: access == identity.ApplicationAccessCustom,
			AllowIP: allowIP, DenyIP: denyIP, Reason: reason,
		})
	}
	previewPermission, previewFound := applicationPermission([]identity.ApplicationPrivilegeAssignment{preview}, preview.PrincipalType, preview.Principal, appID)
	if previewFound {
		evidence = append(evidence, Evidence{
			Source: SourcePreview, PrincipalType: preview.PrincipalType, Principal: preview.Principal,
			Access: previewPermission.Access, Custom: previewPermission.Access == identity.ApplicationAccessCustom,
			Reason: "DSM final preview including direct, group, everyone/default, and built-in account policy",
		})
	}
	sortEvidence(evidence)

	result := baseExplanation(query, evidence)
	if !previewFound {
		result.EffectiveAccess = AccessIndeterminate
		result.Limitations = []string{"DSM application preview omitted the requested application"}
		result.Summary = fmt.Sprintf("Access to application %s is indeterminate: DSM did not return a final preview.", appID)
		return result
	}
	if limitation := applicationLimitation([]Evidence{evidenceForPreview(previewPermission, preview)}); limitation != "" {
		result.EffectiveAccess = AccessIndeterminate
		result.Limitations = []string{limitation}
		result.Summary = fmt.Sprintf("Access to application %s is indeterminate: %s.", appID, limitation)
		return result
	}

	switch previewPermission.Access {
	case identity.ApplicationAccessAllow:
		result.EffectiveAccess = AccessAllow
		result.Determinate = true
	case identity.ApplicationAccessDeny:
		result.EffectiveAccess = AccessDeny
		result.Determinate = true
	default:
		result.EffectiveAccess = AccessIndeterminate
		result.Limitations = []string{fmt.Sprintf("DSM returned unsupported final preview %q", previewPermission.Access)}
		result.Summary = fmt.Sprintf("Access to application %s is indeterminate: DSM returned an unsupported final preview.", appID)
		return result
	}
	result.Summary = applicationSummary(query, result.EffectiveAccess, evidence)
	return result
}

func evidenceForPreview(permission identity.ApplicationPermission, preview identity.ApplicationPrivilegeAssignment) Evidence {
	return Evidence{
		Source: SourcePreview, PrincipalType: preview.PrincipalType, Principal: preview.Principal,
		Access: permission.Access, Custom: permission.Access == identity.ApplicationAccessCustom,
		Reason: "DSM final preview including inherited and default policy",
	}
}

func baseExplanation(query Query, evidence []Evidence) Explanation {
	return Explanation{
		PrincipalType: query.PrincipalType, Principal: query.Principal,
		ResourceType: query.ResourceType, Resource: query.Resource,
		EffectiveAccess: AccessIndeterminate, Evidence: evidence,
	}
}

type applicablePrincipal struct {
	source string
	kind   string
	name   string
}

func applicablePrincipals(query Query, groups []string) []applicablePrincipal {
	result := []applicablePrincipal{{source: SourceDirect, kind: query.PrincipalType, name: query.Principal}}
	if query.PrincipalType != identity.PrincipalUser {
		return result
	}
	seen := map[string]struct{}{strings.ToLower(query.Principal): {}}
	for _, group := range groups {
		key := strings.ToLower(strings.TrimSpace(group))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, applicablePrincipal{source: SourceGroup, kind: identity.PrincipalGroup, name: group})
	}
	return result
}

func sourceFor(query Query, groups []string, principalType, principal string) (string, bool) {
	if strings.EqualFold(principalType, query.PrincipalType) && strings.EqualFold(principal, query.Principal) {
		return SourceDirect, true
	}
	if query.PrincipalType == identity.PrincipalUser && principalType == identity.PrincipalGroup && containsFold(groups, principal) {
		return SourceGroup, true
	}
	return "", false
}

func shareLimitation(query Query, folder share.SharedFolder, direct share.Permission, directFound bool, evidence []Evidence) string {
	if !directFound {
		return fmt.Sprintf("DSM did not return a direct permission row for %s %s", query.PrincipalType, query.Principal)
	}
	if folder.UnifiedPermissions {
		return "DSM Advanced Share Permissions are enabled for this shared folder; their action-specific rules are not yet included"
	}
	for _, rule := range evidence {
		if rule.Masked {
			return fmt.Sprintf("DSM marked the permission controls for %s %s as masked; the cause is not represented by this API", rule.PrincipalType, rule.Principal)
		}
	}
	if query.PrincipalType == identity.PrincipalUser && !direct.InheritanceObserved {
		return "DSM did not return its computed inherited group permission"
	}
	return ""
}

func userShareAccess(direct share.Permission, groups []string, folder share.SharedFolder) string {
	inherited := direct.InheritedAccess
	switch inherited {
	case share.AccessDeny:
		return share.AccessDeny
	case share.AccessCustom:
		if direct.Access == share.AccessDeny {
			return share.AccessDeny
		}
		return share.AccessCustom
	case share.AccessWrite:
		switch direct.Access {
		case share.AccessDeny:
			return share.AccessDeny
		case share.AccessCustom:
			return share.AccessCustom
		default:
			return share.AccessWrite
		}
	case share.AccessRead:
		switch direct.Access {
		case share.AccessDeny:
			return share.AccessDeny
		case share.AccessCustom:
			return share.AccessCustom
		case share.AccessWrite:
			return share.AccessWrite
		default:
			if folder.ACLMode && containsFold(groups, "administrators") {
				return share.AccessWrite
			}
			return share.AccessRead
		}
	case share.AccessNone:
		switch direct.Access {
		case share.AccessDeny, share.AccessCustom, share.AccessWrite, share.AccessRead:
			return direct.Access
		}
		if strings.EqualFold(folder.Name, "homes") {
			return "default"
		}
		return share.AccessNone
	default:
		return "unknown"
	}
}

func groupShareAccess(direct share.Permission, aclMode bool) string {
	switch direct.Access {
	case share.AccessDeny, share.AccessCustom, share.AccessWrite, share.AccessRead:
		if direct.Access == share.AccessRead && aclMode && strings.EqualFold(direct.Principal, "administrators") {
			return share.AccessWrite
		}
		return direct.Access
	case share.AccessNone:
		if aclMode && strings.EqualFold(direct.Principal, "administrators") {
			return share.AccessWrite
		}
		return share.AccessNone
	default:
		return "unknown"
	}
}

func indeterminateShare(result Explanation, query Query, limitation string) Explanation {
	result.EffectiveAccess = AccessIndeterminate
	result.Limitations = []string{limitation}
	result.Summary = fmt.Sprintf("Access to shared folder %s is indeterminate: %s.", query.Resource, limitation)
	return result
}

func applicationLimitation(evidence []Evidence) string {
	for _, rule := range evidence {
		// The normalized allow/deny forms retain DSM's 0.0.0.0 sentinel in
		// their evidence. Only rules decoded as custom represent an IP-aware
		// policy that this evaluator cannot safely reduce to allow or deny.
		if rule.Custom || rule.Access == identity.ApplicationAccessCustom {
			return fmt.Sprintf("%s %s has an IP-specific or custom application rule", rule.PrincipalType, rule.Principal)
		}
	}
	return ""
}

func shareEvidenceReason(permission share.Permission) string {
	parts := []string{"shared-folder root permission reported by DSM"}
	if permission.Inherited {
		parts = append(parts, "DSM returned inherited group access "+permission.InheritedAccess)
	} else if permission.InheritanceObserved {
		parts = append(parts, "DSM returned no inherited group rule")
	}
	if permission.Custom || permission.Access == share.AccessCustom {
		parts = append(parts, "custom ACL semantics are not modeled")
	}
	if permission.Masked {
		parts = append(parts, "DSM marked it masked")
	}
	return strings.Join(parts, "; ")
}

func shareSummary(query Query, effective string, evidence []Evidence) string {
	source := strongestSource(effective, evidence)
	if source == "" {
		return fmt.Sprintf("%s %s has no observed grant on shared folder %s.", query.PrincipalType, query.Principal, query.Resource)
	}
	return fmt.Sprintf("%s %s has %s access to shared folder %s; %s.", query.PrincipalType, query.Principal, effective, query.Resource, source)
}

func applicationSummary(query Query, effective string, evidence []Evidence) string {
	_ = evidence
	return fmt.Sprintf("%s %s has %s access to application %s; DSM final preview includes direct, group, everyone/default, and built-in account policy.", query.PrincipalType, query.Principal, effective, query.Resource)
}

func strongestSource(effective string, evidence []Evidence) string {
	for _, rule := range evidence {
		if rule.Access == effective {
			if rule.Source == SourceDirect {
				return fmt.Sprintf("a direct %s rule applies", effective)
			}
			return fmt.Sprintf("group %s contributes a %s rule", rule.Principal, effective)
		}
	}
	return "DSM precedence was applied to the observed rules"
}

func applicationPermission(assignments []identity.ApplicationPrivilegeAssignment, principalType, principal, appID string) (identity.ApplicationPermission, bool) {
	for _, assignment := range assignments {
		if !strings.EqualFold(assignment.PrincipalType, principalType) || !strings.EqualFold(assignment.Principal, principal) {
			continue
		}
		for _, permission := range assignment.Permissions {
			if strings.EqualFold(permission.ApplicationID, appID) {
				return permission, true
			}
		}
	}
	return identity.ApplicationPermission{}, false
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func sortEvidence(evidence []Evidence) {
	sort.SliceStable(evidence, func(i, j int) bool {
		if evidence[i].Source != evidence[j].Source {
			return evidence[i].Source < evidence[j].Source
		}
		if evidence[i].PrincipalType != evidence[j].PrincipalType {
			return evidence[i].PrincipalType < evidence[j].PrincipalType
		}
		return strings.ToLower(evidence[i].Principal) < strings.ToLower(evidence[j].Principal)
	})
}
