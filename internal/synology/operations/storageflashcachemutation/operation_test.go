package storageflashcachemutation

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/domain/storage"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

// These captures are sanitized transcriptions of the SSD cache request assembly
// in the DSM 7.3 local Storage Manager Admin Center storage_panel.js asset
// (Flashcache "enable"/"remove"). Tests use a fake executor and never send a
// storage mutation to a NAS.
func TestEnableRequestCaptureForBothModes(t *testing.T) {
	tests := []struct {
		name, cacheType, wantMode, raidType string
		disks                               []string
		sizeBytes                           uint64
		isMax                               bool
	}{
		{"read-only-max", storage.CacheModeReadOnly, "readCache", "raid_0", []string{"sda", "sdb"}, 479962595328, true},
		{"read-write-sized", storage.CacheModeReadWrite, "writeCache", "raid_1", []string{"sda", "sdb"}, 1073741824, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, selection, err := Execute(context.Background(), supportedTarget(), capture(t, func(request compatibility.Request) {
				want := map[string]any{
					"cacheMode": test.wantMode, "reference_path": "volume_1",
					"create_type": "shared_cache_and_alloc_cache", "raidType": test.raidType,
					"disk_id": test.disks, "isMax": test.isMax, "size": strconv.FormatUint(test.sizeBytes, 10),
					"skipSeqIO": true, "metadataCache": false, "force": false, "check_lock": true,
				}
				if request.API != APIName || request.Version != 1 || request.Method != "enable" {
					t.Fatalf("request = %#v", request)
				}
				if !reflect.DeepEqual(request.JSONParameters, want) {
					t.Fatalf("parameters = %#v, want %#v", request.JSONParameters, want)
				}
			}), Input{Action: storage.ActionCreate, CacheType: test.cacheType, ReferencePath: "volume_1", RAIDType: test.raidType, DiskIDs: test.disks, SizeBytes: test.sizeBytes, IsMax: test.isMax})
			if err != nil || selection.Operation != CreateOperationName || result.Operation != CreateOperationName || result.ResourceID != "" {
				t.Fatalf("result=%#v selection=%#v err=%v", result, selection, err)
			}
		})
	}
}

func TestRemoveRequestCapture(t *testing.T) {
	result, selection, err := Execute(context.Background(), supportedTarget(), capture(t, func(request compatibility.Request) {
		want := map[string]any{"reference_path": "volume_1", "check_lock": true}
		if request.API != APIName || request.Version != 1 || request.Method != "remove" || !reflect.DeepEqual(request.JSONParameters, want) {
			t.Fatalf("request = %#v, want parameters %#v", request, want)
		}
	}), Input{Action: storage.ActionDelete, ReferencePath: "volume_1"})
	if err != nil || selection.Operation != DeleteOperationName || result.ResourceID != "volume_1" {
		t.Fatalf("result=%#v selection=%#v err=%v", result, selection, err)
	}
}

func TestSelectReportsIndependentCreateDeleteSupport(t *testing.T) {
	selections, err := Select(compatibility.NewTarget())
	if err != nil || len(selections) != 2 {
		t.Fatalf("selections=%#v err=%v", selections, err)
	}
	for _, selection := range selections {
		if selection.Supported {
			t.Fatalf("unexpected supported selection %#v", selection)
		}
	}

	selections, err = Select(supportedTarget())
	if err != nil || len(selections) != 2 {
		t.Fatalf("selections=%#v err=%v", selections, err)
	}
	for index, operation := range []string{CreateOperationName, DeleteOperationName} {
		if !Supported(selections, index) || selections[index].Operation != operation {
			t.Fatalf("selection[%d] = %#v, want supported %q", index, selections[index], operation)
		}
	}
}

func TestUnsupportedActionFailsClosed(t *testing.T) {
	for _, action := range []string{storage.ActionUpdate, storage.ActionConvert} {
		if _, _, err := Execute(context.Background(), supportedTarget(), capture(t, func(compatibility.Request) {
			t.Fatalf("unexpected request for action %q", action)
		}), Input{Action: action, ReferencePath: "volume_1"}); err == nil {
			t.Fatalf("action %q should fail closed", action)
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
