package storagepoolmutation

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/domain/storage"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

// These captures are sanitized transcriptions of the request assembly in the
// DSM 7.3 local Storage Manager Admin Center storage_panel.js asset. Tests use
// a fake executor and never send a storage mutation to a NAS.
func TestCreateRequestCaptureForEverySupportedTopology(t *testing.T) {
	tests := []struct {
		name, raidType, deviceType string
		diskCount                  int
	}{
		{"shr-single", storage.RAIDSHR, "shr_without_disk_protect", 1},
		{"shr-protected", storage.RAIDSHR, "shr_with_1_disk_protect", 2},
		{"shr2", storage.RAIDSHR2, "shr_with_2_disk_protect", 4},
		{"raid0", storage.RAID0, "raid_0", 2},
		{"raid1", storage.RAID1, "raid_1", 2},
		{"raid5", storage.RAID5, "raid_5", 3},
		{"raid6", storage.RAID6, "raid_6", 4},
		{"raid10", storage.RAID10, "raid_10", 4},
		{"jbod", storage.RAIDJBOD, "raid_linear", 2},
		{"basic", storage.RAIDBasic, "basic", 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			disks := make([]string, test.diskCount)
			for index := range disks {
				disks[index] = "disk-" + string(rune('1'+index))
			}
			result, selection, err := Execute(context.Background(), supportedTarget(), capture(t, func(request compatibility.Request) {
				if request.API != APIName || request.Version != 1 || request.Method != "create" {
					t.Fatalf("request = %#v", request)
				}
				want := map[string]any{
					"disk_id": disks, "device_type": test.deviceType, "is_disk_check": true,
					"is_pool_child": false, "allocate_size": "0", "spare_disk_count": "0",
					"desc": "managed", "is_unused": false, "limitNum": "24", "force": false,
				}
				if !reflect.DeepEqual(request.JSONParameters, want) {
					t.Fatalf("parameters = %#v, want %#v", request.JSONParameters, want)
				}
			}), Input{Action: storage.ActionCreate, Pool: storage.PoolChange{Name: "managed", RAIDType: test.raidType, DiskIDs: disks}})
			if err != nil || selection.Operation != CreateOperationName || result.Operation != CreateOperationName || result.ResourceID != "" {
				t.Fatalf("result=%#v selection=%#v err=%v", result, selection, err)
			}
		})
	}
}

func TestExpandAndDeleteRequestCapture(t *testing.T) {
	t.Run("add disks without volume expansion or RAID conversion", func(t *testing.T) {
		result, selection, err := Execute(context.Background(), supportedTarget(), capture(t, func(request compatibility.Request) {
			want := map[string]any{
				"space_id": "pool-7", "disk_id": []string{"disk-5", "disk-6"}, "force": false,
				"diskGroups": []any{}, "do_expand_child_volume": false, "convert_shr_to_pool": false,
			}
			if request.API != APIName || request.Version != 1 || request.Method != "expand_by_add_disk" || !reflect.DeepEqual(request.JSONParameters, want) {
				t.Fatalf("request = %#v, want parameters %#v", request, want)
			}
		}), Input{Action: storage.ActionUpdate, CurrentRAID: storage.RAIDSHR, Pool: storage.PoolChange{ID: "pool-7", AddDiskIDs: []string{"disk-5", "disk-6"}}})
		if err != nil || selection.Operation != ExpandOperationName || result.ResourceID != "pool-7" {
			t.Fatalf("result=%#v selection=%#v err=%v", result, selection, err)
		}
	})

	t.Run("delete exact stable pool ID", func(t *testing.T) {
		result, selection, err := Execute(context.Background(), supportedTarget(), capture(t, func(request compatibility.Request) {
			want := map[string]any{"space_id": "pool-7", "force": true}
			if request.API != APIName || request.Version != 1 || request.Method != "remove" || !reflect.DeepEqual(request.JSONParameters, want) {
				t.Fatalf("request = %#v, want parameters %#v", request, want)
			}
		}), Input{Action: storage.ActionDelete, Pool: storage.PoolChange{ID: "pool-7"}})
		if err != nil || selection.Operation != DeleteOperationName || result.ResourceID != "pool-7" {
			t.Fatalf("result=%#v selection=%#v err=%v", result, selection, err)
		}
	})
}

func TestSelectReportsIndependentCreateExpandDeleteSupport(t *testing.T) {
	selections, err := Select(compatibility.NewTarget())
	if err != nil || len(selections) != 3 {
		t.Fatalf("selections=%#v err=%v", selections, err)
	}
	for _, selection := range selections {
		if selection.Supported {
			t.Fatalf("unexpected supported selection %#v", selection)
		}
	}

	selections, err = Select(supportedTarget())
	if err != nil || len(selections) != 3 {
		t.Fatalf("selections=%#v err=%v", selections, err)
	}
	for index, operation := range []string{CreateOperationName, ExpandOperationName, DeleteOperationName} {
		if !Supported(selections, index) || selections[index].Operation != operation {
			t.Fatalf("selection[%d] = %#v, want supported %q", index, selections[index], operation)
		}
	}
}

type executorFunc func(context.Context, compatibility.Request) (json.RawMessage, error)

func (function executorFunc) Execute(ctx context.Context, request compatibility.Request) (json.RawMessage, error) {
	return function(ctx, request)
}

func capture(t *testing.T, inspect func(compatibility.Request)) executorFunc {
	t.Helper()
	return func(_ context.Context, request compatibility.Request) (json.RawMessage, error) {
		inspect(request)
		return json.RawMessage(`{}`), nil
	}
}

func supportedTarget() compatibility.Target {
	target := compatibility.NewTarget()
	target.SetAPI(APIName, compatibility.APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 1, RequestFormat: "JSON"})
	return target
}
