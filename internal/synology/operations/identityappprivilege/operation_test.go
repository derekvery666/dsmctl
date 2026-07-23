package identityappprivilege

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/derekvery666/dsmctl/internal/domain/identity"
	"github.com/derekvery666/dsmctl/internal/synology/compatibility"
)

type appPrivilegeExecutor struct{ requests []compatibility.Request }

func (executor *appPrivilegeExecutor) Execute(_ context.Context, request compatibility.Request) (json.RawMessage, error) {
	executor.requests = append(executor.requests, request)
	if request.API == AppAPIName {
		if request.Method == "preview" {
			return json.RawMessage(`{"applications":[{"app_id":"SYNO.Desktop","privilelge":"deny"},{"app_id":"custom","privilelge":"custom"}]}`), nil
		}
		return json.RawMessage(`{"applications":[{"app_id":"SYNO.Desktop","name":["DSM"],"grant_type":["local"],"supportIP":true}]}`), nil
	}
	if request.Method == "get" {
		return json.RawMessage(`{"rules":[{"app_id":"SYNO.Desktop","allow_ip":[],"deny_ip":["0.0.0.0"]},{"app_id":"custom","allow_ip":["10.0.0.0/8"],"deny_ip":[]}]}`), nil
	}
	return json.RawMessage(`{}`), nil
}

func TestApplicationInventoryRulesAndPartialSet(t *testing.T) {
	target := compatibility.NewTarget()
	target.SetAPI(AppAPIName, compatibility.APIInfo{MinVersion: 1, MaxVersion: 3})
	target.SetAPI(RuleAPIName, compatibility.APIInfo{MinVersion: 1, MaxVersion: 1})
	executor := &appPrivilegeExecutor{}
	applications, _, err := ExecuteApps(context.Background(), target, executor)
	if err != nil || len(applications) != 1 || applications[0].Name != "DSM" || !applications[0].SupportsIP {
		t.Fatalf("applications=%#v err=%v", applications, err)
	}
	assignment, _, err := ExecuteRead(context.Background(), target, executor, PrincipalInput{PrincipalType: identity.PrincipalUser, Principal: "alice"})
	if err != nil || len(assignment.Permissions) != 2 || assignment.Permissions[0].Access != identity.ApplicationAccessDeny || assignment.Permissions[1].Access != identity.ApplicationAccessCustom {
		t.Fatalf("assignment=%#v err=%v", assignment, err)
	}
	preview, selection, err := ExecutePreview(context.Background(), target, executor, PrincipalInput{PrincipalType: identity.PrincipalUser, Principal: "alice"})
	if err != nil || !selection.Supported || len(preview.Permissions) != 2 || preview.Permissions[0].Access != identity.ApplicationAccessDeny {
		t.Fatalf("preview=%#v selection=%#v err=%v", preview, selection, err)
	}
	previewRequest := executor.requests[len(executor.requests)-1]
	if previewRequest.Method != "preview" || previewRequest.JSONParameters["username"] != "alice" {
		t.Fatalf("preview request = %#v", previewRequest)
	}
	executor.requests = nil
	_, _, err = ExecuteSet(context.Background(), target, executor, SetInput{PrincipalType: identity.PrincipalUser, Principal: "alice", Permissions: []identity.ApplicationPermissionChange{
		{ApplicationID: "old", Access: identity.ApplicationAccessInherit}, {ApplicationID: "SYNO.Desktop", Access: identity.ApplicationAccessAllow},
	}})
	if err != nil || len(executor.requests) != 2 {
		t.Fatalf("set requests=%d err=%v", len(executor.requests), err)
	}
	if executor.requests[0].Method != "delete" || executor.requests[1].Method != "set" {
		t.Fatalf("methods=%q,%q", executor.requests[0].Method, executor.requests[1].Method)
	}
	rules, ok := executor.requests[1].JSONParameters["rules"].([]map[string]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("set rules = %#v", executor.requests[1].JSONParameters["rules"])
	}
	allow := rules[0]["allow_ip"].([]string)
	deny := rules[0]["deny_ip"].([]string)
	if len(allow) != 1 || allow[0] != "0.0.0.0" || len(deny) != 0 {
		t.Fatalf("allow=%v deny=%v", allow, deny)
	}
}

func TestDecodePreviewRejectsMalformedOrUnknownResults(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
	}{
		{name: "missing array", data: `{}`},
		{name: "missing app id", data: `{"applications":[{"privilelge":"allow"}]}`},
		{name: "correctly spelled field is not accepted", data: `{"applications":[{"app_id":"app","privilege":"allow"}]}`},
		{name: "unknown privilege", data: `{"applications":[{"app_id":"app","privilelge":"maybe"}]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodePreview(json.RawMessage(test.data)); err == nil {
				t.Fatal("decodePreview() succeeded, want error")
			}
		})
	}
}

func TestPrivilegeDecodersRejectMissingCollectionsAndFields(t *testing.T) {
	for _, data := range []string{`{}`, `{"rules":null}`, `{"rules":[{"allow_ip":[],"deny_ip":[]}]}`, `{"rules":[{"app_id":"app","deny_ip":[]}]}`, `{"rules":[{"app_id":"app","allow_ip":[1],"deny_ip":[]}]}`} {
		if _, err := decodePermissions(json.RawMessage(data)); err == nil {
			t.Fatalf("decodePermissions(%s) succeeded, want error", data)
		}
	}
	for _, data := range []string{`{}`, `{"applications":null}`, `{"applications":[{"name":"missing id"}]}`} {
		if _, err := decodeApplications(json.RawMessage(data)); err == nil {
			t.Fatalf("decodeApplications(%s) succeeded, want error", data)
		}
	}
}
