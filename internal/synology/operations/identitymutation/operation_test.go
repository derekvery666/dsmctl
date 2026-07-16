package identitymutation

import (
	"reflect"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/domain/identity"
)

func TestUserRequestCreateUsesPasswordOnlyInDSMParameters(t *testing.T) {
	description := "Automation account"
	email := "bot@example.com"
	expired := "normal"
	cannotChange := true
	neverExpires := true
	method, parameters, resultName, err := userRequest(UserInput{
		Action: identity.ActionCreate,
		Change: identity.UserChange{
			Name:                 "dsmctl-bot",
			Description:          &description,
			Email:                &email,
			Expired:              &expired,
			CannotChangePassword: &cannotChange,
			PasswordNeverExpires: &neverExpires,
			CredentialRef:        "env:DSMCTL_TEST_PASSWORD",
		},
		Password: "resolved-secret",
	})
	if err != nil {
		t.Fatalf("userRequest() error = %v", err)
	}
	if method != "create" || resultName != "dsmctl-bot" {
		t.Fatalf("method=%q resultName=%q", method, resultName)
	}
	for key, want := range map[string]any{
		"name":                "dsmctl-bot",
		"password":            "resolved-secret",
		"description":         description,
		"email":               email,
		"expired":             expired,
		"cannot_chg_passwd":   true,
		"passwd_never_expire": true,
		"notify_by_email":     false,
	} {
		if got := parameters[key]; !reflect.DeepEqual(got, want) {
			t.Errorf("parameter %s = %#v, want %#v", key, got, want)
		}
	}
	if _, found := parameters["credential_ref"]; found {
		t.Fatal("credential_ref leaked into DSM parameters")
	}
}

func TestUserAndGroupUpdateAndDeleteRequests(t *testing.T) {
	newUserName := "new-user"
	method, parameters, resultName, err := userRequest(UserInput{Action: identity.ActionUpdate, Change: identity.UserChange{Name: "old-user", NewName: &newUserName}})
	if err != nil || method != "set" || resultName != newUserName || parameters["name"] != "old-user" || parameters["new_name"] != newUserName {
		t.Fatalf("user update: method=%q params=%v result=%q err=%v", method, parameters, resultName, err)
	}

	method, parameters, _, err = userRequest(UserInput{Action: identity.ActionDelete, Change: identity.UserChange{Name: "old-user"}})
	if err != nil || method != "delete" {
		t.Fatalf("user delete: method=%q err=%v", method, err)
	}
	names, ok := parameters["name"].([]string)
	if !ok || len(names) != 1 || names[0] != "old-user" {
		t.Fatalf("user delete names = %#v", parameters["name"])
	}

	description := "renamed group"
	newGroupName := "new-group"
	groupMethod, groupParameters, groupResultName, groupErr := groupRequest(GroupInput{Action: identity.ActionUpdate, Change: identity.GroupChange{Name: "old-group", NewName: &newGroupName, Description: &description}})
	if groupErr != nil || groupMethod != "set" || groupResultName != newGroupName || groupParameters.Get("new_name") != newGroupName || groupParameters.Get("description") != description {
		t.Fatalf("group update: method=%q params=%v result=%q err=%v", groupMethod, groupParameters, groupResultName, groupErr)
	}
}

func TestUserCreateRejectsMissingResolvedPassword(t *testing.T) {
	_, _, _, err := userRequest(UserInput{Action: identity.ActionCreate, Change: identity.UserChange{Name: "missing-password"}})
	if err == nil {
		t.Fatal("userRequest() accepted an empty resolved password")
	}
}
