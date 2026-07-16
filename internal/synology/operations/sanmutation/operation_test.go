package sanmutation

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/domain/san"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

type executorFunc func(context.Context, compatibility.Request) (json.RawMessage, error)

func (function executorFunc) Execute(ctx context.Context, request compatibility.Request) (json.RawMessage, error) {
	return function(ctx, request)
}

func TestTargetMutationsCaptureTypedRequests(t *testing.T) {
	target := supportedTarget()
	t.Run("create mutual CHAP", func(t *testing.T) {
		result, selection, err := ExecuteTarget(context.Background(), target, capture(t, func(request compatibility.Request) json.RawMessage {
			if request.API != TargetAPIName || request.Method != "create" || request.Version != 1 {
				t.Fatalf("request = %#v", request)
			}
			want := map[string]any{
				"name": "db", "iqn": "iqn.2000-01.com.synology:nas.db", "auth_type": 2,
				"user": "initiator", "password": "chap-secret", "mutual_user": "target", "mutual_password": "mutual-secret",
			}
			if !reflect.DeepEqual(request.JSONParameters, want) {
				t.Fatalf("parameters = %#v, want %#v", request.JSONParameters, want)
			}
			if !reflect.DeepEqual(request.EncryptedParameters, []string{"password", "mutual_password"}) {
				t.Fatalf("encrypted = %#v", request.EncryptedParameters)
			}
			return json.RawMessage(`{"target_id":42}`)
		}), TargetInput{Action: san.ActionCreate, Name: "db", IQN: "iqn.2000-01.com.synology:nas.db", Authentication: san.AuthenticationMutualCHAP, CHAPUser: "initiator", CHAPPassword: "chap-secret", MutualCHAPUser: "target", MutualCHAPPassword: "mutual-secret"})
		if err != nil || result.ResourceID != "42" || selection.Operation != TargetCreateOperationName {
			t.Fatalf("result=%#v selection=%#v err=%v", result, selection, err)
		}
	})

	t.Run("patch properties", func(t *testing.T) {
		newName, newIQN, auth := "db-new", "iqn.2000-01.com.synology:nas.db-new", san.AuthenticationNone
		_, _, err := ExecuteTarget(context.Background(), target, capture(t, func(request compatibility.Request) json.RawMessage {
			want := map[string]any{"target_id": "42", "name": newName, "iqn": newIQN, "auth_type": 0}
			if request.Method != "set" || !reflect.DeepEqual(request.JSONParameters, want) {
				t.Fatalf("request = %#v, want parameters %#v", request, want)
			}
			return json.RawMessage(`{}`)
		}), TargetInput{Action: san.ActionUpdate, ID: "42", NewName: &newName, NewIQN: &newIQN, NewAuthentication: &auth})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("disable", func(t *testing.T) {
		enabled := false
		_, _, err := ExecuteTarget(context.Background(), target, capture(t, func(request compatibility.Request) json.RawMessage {
			if request.Method != "disable" || !reflect.DeepEqual(request.JSONParameters, map[string]any{"target_id": "42"}) {
				t.Fatalf("request = %#v", request)
			}
			return json.RawMessage(`{}`)
		}), TargetInput{Action: san.ActionUpdate, ID: "42", Enabled: &enabled})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		result, _, err := ExecuteTarget(context.Background(), target, capture(t, func(request compatibility.Request) json.RawMessage {
			if request.Method != "delete" || !reflect.DeepEqual(request.JSONParameters, map[string]any{"target_id": "42"}) {
				t.Fatalf("request = %#v", request)
			}
			return json.RawMessage(`{}`)
		}), TargetInput{Action: san.ActionDelete, ID: "42"})
		if err != nil || result.ResourceID != "42" {
			t.Fatalf("result=%#v err=%v", result, err)
		}
	})
}

func TestLUNMutationsCaptureTypedRequests(t *testing.T) {
	target := supportedTarget()
	t.Run("create thin btrfs", func(t *testing.T) {
		result, selection, err := ExecuteLUN(context.Background(), target, capture(t, func(request compatibility.Request) json.RawMessage {
			if request.API != LUNAPIName || request.Method != "create" {
				t.Fatalf("request = %#v", request)
			}
			if request.JSONParameters["name"] != "data" || request.JSONParameters["location"] != "/volume1" || request.JSONParameters["size"] != uint64(1<<30) || request.JSONParameters["type"] != "BLUN" {
				t.Fatalf("parameters = %#v", request.JSONParameters)
			}
			attributes, ok := request.JSONParameters["dev_attribs"].([]map[string]any)
			if !ok || len(attributes) != 5 || attributes[4]["dev_attrib"] != "can_snapshot" || attributes[4]["enable"] != 1 {
				t.Fatalf("attributes = %#v", request.JSONParameters["dev_attribs"])
			}
			return json.RawMessage(`{"uuid":"lun-uuid"}`)
		}), LUNInput{Action: san.ActionCreate, Name: "data", BackingVolumePath: "/volume1", BackingFileSystem: "btrfs", SizeBytes: 1 << 30, Provisioning: san.ProvisioningThin})
		if err != nil || result.ResourceID != "lun-uuid" || selection.Operation != LUNCreateOperationName {
			t.Fatalf("result=%#v selection=%#v err=%v", result, selection, err)
		}
	})

	t.Run("patch", func(t *testing.T) {
		name, description, path, size := "data-new", "", "/volume2", uint64(2<<30)
		_, _, err := ExecuteLUN(context.Background(), target, capture(t, func(request compatibility.Request) json.RawMessage {
			want := map[string]any{"uuid": "lun-uuid", "is_soft_feas_ignored": false, "new_name": name, "description": description, "new_location": path, "new_size": size}
			if request.Method != "set" || !reflect.DeepEqual(request.JSONParameters, want) {
				t.Fatalf("request=%#v want=%#v", request, want)
			}
			return json.RawMessage(`{}`)
		}), LUNInput{Action: san.ActionUpdate, ID: "lun-uuid", NewName: &name, NewDescription: &description, NewBackingPath: &path, NewSizeBytes: &size})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("delete exact stable ID", func(t *testing.T) {
		_, _, err := ExecuteLUN(context.Background(), target, capture(t, func(request compatibility.Request) json.RawMessage {
			want := map[string]any{"uuid": "", "uuids": []string{"lun-uuid"}, "is_soft_feas_ignored": false}
			if request.Method != "delete" || !reflect.DeepEqual(request.JSONParameters, want) {
				t.Fatalf("request=%#v want=%#v", request, want)
			}
			return json.RawMessage(`{}`)
		}), LUNInput{Action: san.ActionDelete, ID: "lun-uuid"})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestMappingMutationsNeverDeleteEndpoints(t *testing.T) {
	target := supportedTarget()
	for _, test := range []struct{ action, method, operation string }{
		{san.ActionAttach, "map_target", MappingAttachOperationName},
		{san.ActionDetach, "unmap_target", MappingDetachOperationName},
	} {
		t.Run(test.action, func(t *testing.T) {
			result, selection, err := ExecuteMapping(context.Background(), target, capture(t, func(request compatibility.Request) json.RawMessage {
				want := map[string]any{"uuid": "lun-uuid", "target_ids": []string{"42"}}
				if request.API != LUNAPIName || request.Method != test.method || !reflect.DeepEqual(request.JSONParameters, want) {
					t.Fatalf("request=%#v want=%#v", request, want)
				}
				return json.RawMessage(`{}`)
			}), MappingInput{Action: test.action, TargetID: "42", LUNID: "lun-uuid"})
			if err != nil || selection.Operation != test.operation || result.ResourceID != "42:lun-uuid" {
				t.Fatalf("result=%#v selection=%#v err=%v", result, selection, err)
			}
		})
	}
}

func TestCreateRequiresStableIDResponse(t *testing.T) {
	_, _, err := ExecuteLUN(context.Background(), supportedTarget(), capture(t, func(compatibility.Request) json.RawMessage {
		return json.RawMessage(`{}`)
	}), LUNInput{Action: san.ActionCreate, BackingFileSystem: "btrfs", Provisioning: san.ProvisioningThin})
	if err == nil {
		t.Fatal("create accepted response without stable UUID")
	}
}

func TestSelectReportsEightIndependentOperations(t *testing.T) {
	selections, err := Select(compatibility.NewTarget())
	if err != nil || len(selections) != 8 {
		t.Fatalf("selections=%#v err=%v", selections, err)
	}
	for _, selection := range selections {
		if selection.Supported {
			t.Fatalf("unexpected supported selection %#v", selection)
		}
	}

	targetOnly := compatibility.NewTarget()
	targetOnly.SetAPI(TargetAPIName, compatibility.APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 1})
	selections, err = Select(targetOnly)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 3; index++ {
		if !Supported(selections, index) {
			t.Fatalf("target operation %d unsupported: %#v", index, selections[index])
		}
	}
	for index := 3; index < 8; index++ {
		if Supported(selections, index) {
			t.Fatalf("LUN operation %d unexpectedly supported: %#v", index, selections[index])
		}
	}
}

func TestExecutorFailurePreservesOperationContext(t *testing.T) {
	want := errors.New("DSM rejected request")
	_, _, err := ExecuteMapping(context.Background(), supportedTarget(), executorFunc(func(context.Context, compatibility.Request) (json.RawMessage, error) {
		return nil, want
	}), MappingInput{Action: san.ActionAttach, TargetID: "42", LUNID: "lun"})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}

func capture(t *testing.T, inspect func(compatibility.Request) json.RawMessage) executorFunc {
	t.Helper()
	return func(_ context.Context, request compatibility.Request) (json.RawMessage, error) {
		return inspect(request), nil
	}
}

func supportedTarget() compatibility.Target {
	target := compatibility.NewTarget()
	target.SetAPI(TargetAPIName, compatibility.APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 1, RequestFormat: "JSON"})
	target.SetAPI(LUNAPIName, compatibility.APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 1, RequestFormat: "JSON"})
	return target
}
