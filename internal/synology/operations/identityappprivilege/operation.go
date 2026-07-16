package identityappprivilege

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/ychiu1211/dsmctl/internal/domain/identity"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

const (
	AppAPIName            = "SYNO.Core.AppPriv.App"
	RuleAPIName           = "SYNO.Core.AppPriv.Rule"
	ReadCapabilityName    = "identity.application_privileges.read"
	SetCapabilityName     = "identity.application_privileges.set"
	AppListOperationName  = "identity.applications.list"
	RuleReadOperationName = "identity.application_privileges.get"
	RuleSetOperationName  = "identity.application_privileges.set"
	PreviewOperationName  = "identity.application_privileges.preview"
	PreviewCapabilityName = "identity.application_privileges.preview"
)

type PrincipalInput struct {
	PrincipalType string
	Principal     string
}
type SetInput struct {
	PrincipalType string
	Principal     string
	Permissions   []identity.ApplicationPermissionChange
}
type Result struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Name     string `json:"name"`
}

var appListOperation = compatibility.Operation[struct{}, []identity.Application]{
	Name: AppListOperationName,
	Variants: []compatibility.Variant[struct{}, []identity.Application]{
		{
			Name:     "core-apppriv-app-list-v2",
			API:      AppAPIName,
			Version:  2,
			Priority: 10,
			Match:    compatibility.APIVersion(AppAPIName, 2),
			Execute: func(ctx context.Context, executor compatibility.Executor, _ struct{}) ([]identity.Application, error) {
				data, err := executor.Execute(ctx, compatibility.Request{API: AppAPIName, Version: 2, Method: "list", Parameters: url.Values{"offset": {"0"}, "limit": {"-1"}}})
				if err != nil {
					return nil, fmt.Errorf("call %s.list v2: %w", AppAPIName, err)
				}
				return decodeApplications(data)
			},
		},
	},
}

var ruleReadOperation = compatibility.Operation[PrincipalInput, identity.ApplicationPrivilegeAssignment]{
	Name: RuleReadOperationName,
	Variants: []compatibility.Variant[PrincipalInput, identity.ApplicationPrivilegeAssignment]{
		{
			Name:     "core-apppriv-rule-get-v1",
			API:      RuleAPIName,
			Version:  1,
			Priority: 10,
			Match:    compatibility.APIVersion(RuleAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input PrincipalInput) (identity.ApplicationPrivilegeAssignment, error) {
				data, err := executor.Execute(ctx, compatibility.Request{API: RuleAPIName, Version: 1, Method: "get", Parameters: url.Values{"entity_type": {input.PrincipalType}, "entity_name": {input.Principal}}})
				if err != nil {
					return identity.ApplicationPrivilegeAssignment{}, fmt.Errorf("call %s.get v1 for %s %q: %w", RuleAPIName, input.PrincipalType, input.Principal, err)
				}
				permissions, err := decodePermissions(data)
				if err != nil {
					return identity.ApplicationPrivilegeAssignment{}, err
				}
				return identity.ApplicationPrivilegeAssignment{PrincipalType: input.PrincipalType, Principal: input.Principal, Permissions: permissions}, nil
			},
		},
	},
}

var previewOperation = compatibility.Operation[PrincipalInput, identity.ApplicationPrivilegeAssignment]{
	Name: PreviewOperationName,
	Variants: []compatibility.Variant[PrincipalInput, identity.ApplicationPrivilegeAssignment]{
		{
			Name:     "core-apppriv-app-preview-v2",
			API:      AppAPIName,
			Version:  2,
			Priority: 10,
			Match:    compatibility.APIVersion(AppAPIName, 2),
			Execute: func(ctx context.Context, executor compatibility.Executor, input PrincipalInput) (identity.ApplicationPrivilegeAssignment, error) {
				parameters := map[string]any{}
				switch input.PrincipalType {
				case identity.PrincipalUser:
					parameters["username"] = input.Principal
				case identity.PrincipalGroup:
					parameters["groups"] = []string{input.Principal}
				default:
					return identity.ApplicationPrivilegeAssignment{}, fmt.Errorf("unsupported application preview principal type %q", input.PrincipalType)
				}
				data, err := executor.Execute(ctx, compatibility.Request{API: AppAPIName, Version: 2, Method: "preview", JSONParameters: parameters})
				if err != nil {
					return identity.ApplicationPrivilegeAssignment{}, fmt.Errorf("call %s.preview v2 for %s %q: %w", AppAPIName, input.PrincipalType, input.Principal, err)
				}
				permissions, err := decodePreview(data)
				if err != nil {
					return identity.ApplicationPrivilegeAssignment{}, err
				}
				return identity.ApplicationPrivilegeAssignment{PrincipalType: input.PrincipalType, Principal: input.Principal, Permissions: permissions}, nil
			},
		},
	},
}

var ruleSetOperation = compatibility.Operation[SetInput, Result]{
	Name: RuleSetOperationName,
	Variants: []compatibility.Variant[SetInput, Result]{
		{
			Name:     "core-apppriv-rule-set-v1",
			API:      RuleAPIName,
			Version:  1,
			Priority: 10,
			Match:    compatibility.APIVersion(RuleAPIName, 1),
			Execute:  executeRuleSet,
		},
	},
}

func executeRuleSet(ctx context.Context, executor compatibility.Executor, input SetInput) (Result, error) {
	deletes := make([]map[string]any, 0)
	sets := make([]map[string]any, 0)
	for _, permission := range input.Permissions {
		rule := map[string]any{"entity_type": input.PrincipalType, "entity_name": input.Principal, "app_id": permission.ApplicationID}
		switch permission.Access {
		case identity.ApplicationAccessInherit:
			deletes = append(deletes, rule)
		case identity.ApplicationAccessAllow:
			rule["allow_ip"] = []string{"0.0.0.0"}
			rule["deny_ip"] = []string{}
			sets = append(sets, rule)
		case identity.ApplicationAccessDeny:
			rule["allow_ip"] = []string{}
			rule["deny_ip"] = []string{"0.0.0.0"}
			sets = append(sets, rule)
		}
	}
	if len(deletes) > 0 {
		if _, err := executor.Execute(ctx, compatibility.Request{API: RuleAPIName, Version: 1, Method: "delete", JSONParameters: map[string]any{"rules": deletes}}); err != nil {
			return Result{}, fmt.Errorf("call %s.delete v1: %w", RuleAPIName, err)
		}
	}
	if len(sets) > 0 {
		if _, err := executor.Execute(ctx, compatibility.Request{API: RuleAPIName, Version: 1, Method: "set", JSONParameters: map[string]any{"rules": sets}}); err != nil {
			return Result{}, fmt.Errorf("call %s.set v1: %w", RuleAPIName, err)
		}
	}
	return Result{Resource: identity.ResourceApplicationPrivilege, Action: identity.ActionSet, Name: input.PrincipalType + ":" + input.Principal}, nil
}

func decodeApplications(data json.RawMessage) ([]identity.Application, error) {
	applications, err := requiredObjectArray(data, "applications", "application inventory")
	if err != nil {
		return nil, err
	}
	result := make([]identity.Application, 0, len(applications))
	for index, item := range applications {
		id, _ := item["app_id"].(string)
		if strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("decode application inventory: applications[%d] has no app_id", index)
		}
		result = append(result, identity.Application{ID: id, Name: displayName(item["name"]), GrantTypes: stringSlice(item["grant_type"]), SupportsIP: boolean(item["supportIP"])})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func decodePermissions(data json.RawMessage) ([]identity.ApplicationPermission, error) {
	rules, err := requiredObjectArray(data, "rules", "application privilege rules")
	if err != nil {
		return nil, err
	}
	result := make([]identity.ApplicationPermission, 0, len(rules))
	for index, rule := range rules {
		appID, _ := rule["app_id"].(string)
		if strings.TrimSpace(appID) == "" {
			return nil, fmt.Errorf("decode application privilege rules: rules[%d] has no app_id", index)
		}
		allowIP, err := requiredStringArray(rule, "allow_ip")
		if err != nil {
			return nil, fmt.Errorf("decode application privilege rules: rules[%d]: %w", index, err)
		}
		denyIP, err := requiredStringArray(rule, "deny_ip")
		if err != nil {
			return nil, fmt.Errorf("decode application privilege rules: rules[%d]: %w", index, err)
		}
		access := identity.ApplicationAccessCustom
		if len(allowIP) == 1 && allowIP[0] == "0.0.0.0" && len(denyIP) == 0 {
			access = identity.ApplicationAccessAllow
		}
		if len(denyIP) == 1 && denyIP[0] == "0.0.0.0" && len(allowIP) == 0 {
			access = identity.ApplicationAccessDeny
		}
		result = append(result, identity.ApplicationPermission{ApplicationID: appID, Access: access, AllowIP: allowIP, DenyIP: denyIP})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ApplicationID < result[j].ApplicationID })
	return result, nil
}

func decodePreview(data json.RawMessage) ([]identity.ApplicationPermission, error) {
	var response struct {
		Applications json.RawMessage `json:"applications"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode application privilege preview: %w", err)
	}
	if response.Applications == nil {
		return nil, fmt.Errorf("decode application privilege preview: required field %q is missing", "applications")
	}
	if string(bytes.TrimSpace(response.Applications)) == "null" {
		return nil, fmt.Errorf("decode application privilege preview: field %q must be an array", "applications")
	}
	var applications []struct {
		AppID     string  `json:"app_id"`
		Privilege *string `json:"privilelge"`
	}
	if err := json.Unmarshal(response.Applications, &applications); err != nil {
		return nil, fmt.Errorf("decode application privilege preview applications: %w", err)
	}
	if applications == nil {
		applications = make([]struct {
			AppID     string  `json:"app_id"`
			Privilege *string `json:"privilelge"`
		}, 0)
	}
	result := make([]identity.ApplicationPermission, 0, len(applications))
	for index, application := range applications {
		if strings.TrimSpace(application.AppID) == "" {
			return nil, fmt.Errorf("decode application privilege preview: applications[%d] has no app_id", index)
		}
		if application.Privilege == nil {
			return nil, fmt.Errorf("decode application privilege preview: applications[%d] has no privilelge field", index)
		}
		switch *application.Privilege {
		case identity.ApplicationAccessAllow, identity.ApplicationAccessDeny, identity.ApplicationAccessCustom:
		default:
			return nil, fmt.Errorf("decode application privilege preview: applications[%d] has unsupported privilelge %q", index, *application.Privilege)
		}
		result = append(result, identity.ApplicationPermission{ApplicationID: application.AppID, Access: *application.Privilege})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ApplicationID < result[j].ApplicationID })
	return result, nil
}

func requiredObjectArray(data json.RawMessage, key, label string) ([]map[string]any, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	raw, ok := root[key]
	if !ok {
		return nil, fmt.Errorf("decode %s: required field %q is missing", label, key)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var result []map[string]any
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode %s field %q: %w", label, key, err)
	}
	if result == nil {
		return nil, fmt.Errorf("decode %s: field %q must be an array", label, key)
	}
	return result, nil
}

func requiredStringArray(values map[string]any, key string) ([]string, error) {
	raw, ok := values[key]
	if !ok {
		return nil, fmt.Errorf("required field %q is missing", key)
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("field %q must be a string array", key)
	}
	result := make([]string, 0, len(items))
	for index, item := range items {
		value, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("field %q item %d must be a string", key, index)
		}
		result = append(result, value)
	}
	return result, nil
}

func displayName(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		for _, item := range typed {
			if name, ok := item.(string); ok && name != "" {
				return name
			}
		}
	}
	return ""
}
func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	case []string:
		return typed
	case string:
		if typed != "" {
			return strings.Split(typed, ",")
		}
	}
	return nil
}
func boolean(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case string:
		return typed == "true" || typed == "1"
	}
	return false
}

func APINames() []string { return []string{AppAPIName, RuleAPIName} }
func Select(target compatibility.Target) ([]compatibility.Selection, error) {
	selectors := []func(compatibility.Target) (compatibility.Selection, error){selectApps, selectRules, selectSet, selectPreview}
	result := make([]compatibility.Selection, 0, len(selectors))
	for _, selector := range selectors {
		selection, err := selector(target)
		result = append(result, selection)
		if err != nil && !compatibility.IsUnsupported(err) {
			return nil, err
		}
	}
	return result, nil
}
func selectApps(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := appListOperation.Select(target)
	return selection, err
}
func selectRules(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := ruleReadOperation.Select(target)
	return selection, err
}
func selectSet(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := ruleSetOperation.Select(target)
	return selection, err
}
func selectPreview(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := previewOperation.Select(target)
	return selection, err
}
func ExecuteApps(ctx context.Context, target compatibility.Target, executor compatibility.Executor) ([]identity.Application, compatibility.Selection, error) {
	return appListOperation.Run(ctx, target, executor, struct{}{})
}
func ExecuteRead(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input PrincipalInput) (identity.ApplicationPrivilegeAssignment, compatibility.Selection, error) {
	return ruleReadOperation.Run(ctx, target, executor, input)
}
func ExecutePreview(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input PrincipalInput) (identity.ApplicationPrivilegeAssignment, compatibility.Selection, error) {
	return previewOperation.Run(ctx, target, executor, input)
}
func ExecuteSet(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input SetInput) (Result, compatibility.Selection, error) {
	return ruleSetOperation.Run(ctx, target, executor, input)
}
