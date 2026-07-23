package volumemutation

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/derekvery666/dsmctl/internal/domain/storage"
	"github.com/derekvery666/dsmctl/internal/synology/compatibility"
)

const gib = uint64(1) << 30

func TestCreateRequestCapturesBothPoolLayouts(t *testing.T) {
	t.Run("multi-volume exact allocation uses MiB", func(t *testing.T) {
		result, selection, err := Execute(context.Background(), supportedTarget(), capture(t, func(request compatibility.Request) {
			want := map[string]any{
				"pool_path": "reuse_7", "allocate_size": "10240", "fs_type": "btrfs",
				"vol_attr": "generic", "vol_desc": "managed", "atime_opt": "noatime",
				"force": false, "enable_dedupe": false,
			}
			if request.API != APIName || request.Version != 1 || request.Method != "create_on_existing_pool" || !reflect.DeepEqual(request.JSONParameters, want) {
				t.Fatalf("request = %#v, want parameters %#v", request, want)
			}
		}), Input{Action: storage.ActionCreate, PoolPath: "reuse_7", ResolvedCapacityBytes: 10 * gib, Volume: storage.VolumeChange{Name: "managed", FileSystem: "btrfs"}})
		if err != nil || selection.Operation != CreateOperationName || result.Operation != CreateOperationName || result.ResourceID != "" {
			t.Fatalf("result=%#v selection=%#v err=%v", result, selection, err)
		}
	})

	t.Run("single-volume deploy consumes unused pool", func(t *testing.T) {
		_, _, err := Execute(context.Background(), supportedTarget(), capture(t, func(request compatibility.Request) {
			want := map[string]any{
				"space_path": "/dev/vg7", "fs_type": "ext4", "vol_attr": "generic",
				"vol_desc": "single", "atime_opt": "noatime", "force": false, "enable_dedupe": false,
			}
			if request.Method != "deploy_unused" || !reflect.DeepEqual(request.JSONParameters, want) {
				t.Fatalf("request = %#v, want parameters %#v", request, want)
			}
		}), Input{Action: storage.ActionCreate, SingleVolume: true, SpacePath: "/dev/vg7", ResolvedCapacityBytes: 10 * gib, Volume: storage.VolumeChange{Name: "single", FileSystem: "ext4"}})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestExpandAndDeleteRequestCaptures(t *testing.T) {
	t.Run("multi-volume expansion sends byte target", func(t *testing.T) {
		_, _, err := Execute(context.Background(), supportedTarget(), capture(t, func(request compatibility.Request) {
			want := map[string]any{"space_id": "volume_7", "new_size": "21474836480"}
			if request.Method != "expand_pool_child" || !reflect.DeepEqual(request.JSONParameters, want) {
				t.Fatalf("request = %#v, want parameters %#v", request, want)
			}
		}), Input{Action: storage.ActionUpdate, ResolvedCapacityBytes: 20 * gib, Volume: storage.VolumeChange{ID: "volume_7"}})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("single-volume expansion consumes unallocated capacity", func(t *testing.T) {
		_, _, err := Execute(context.Background(), supportedTarget(), capture(t, func(request compatibility.Request) {
			want := map[string]any{"space_id": "volume_7"}
			if request.Method != "expand_unallocated" || !reflect.DeepEqual(request.JSONParameters, want) {
				t.Fatalf("request = %#v, want parameters %#v", request, want)
			}
		}), Input{Action: storage.ActionUpdate, SingleVolume: true, Volume: storage.VolumeChange{ID: "volume_7"}})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("delete sends exact stable ID array", func(t *testing.T) {
		result, _, err := Execute(context.Background(), supportedTarget(), capture(t, func(request compatibility.Request) {
			want := map[string]any{"space_id": []string{"volume_7"}, "force": true}
			if request.Method != "delete" || !reflect.DeepEqual(request.JSONParameters, want) {
				t.Fatalf("request = %#v, want parameters %#v", request, want)
			}
		}), Input{Action: storage.ActionDelete, Volume: storage.VolumeChange{ID: "volume_7"}})
		if err != nil || result.ResourceID != "volume_7" {
			t.Fatalf("result=%#v err=%v", result, err)
		}
	})
}

func TestAllocationRejectsNonWholeOrZeroMiB(t *testing.T) {
	for _, bytes := range []uint64{0, mebibyte + 1} {
		if _, err := allocationMiB(bytes); err == nil {
			t.Fatalf("allocationMiB(%d) unexpectedly succeeded", bytes)
		}
	}
}

func TestSelectionsAreIndependent(t *testing.T) {
	selections, err := Select(supportedTarget())
	if err != nil || len(selections) != 3 {
		t.Fatalf("selections=%#v err=%v", selections, err)
	}
	for index, operation := range []string{CreateOperationName, ExpandOperationName, DeleteOperationName} {
		if !Supported(selections, index) || selections[index].Operation != operation {
			t.Fatalf("selection[%d] = %#v", index, selections[index])
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
