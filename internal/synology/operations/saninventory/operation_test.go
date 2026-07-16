package saninventory

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/domain/san"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

type executorFunc func(context.Context, compatibility.Request) (json.RawMessage, error)

func (function executorFunc) Execute(ctx context.Context, request compatibility.Request) (json.RawMessage, error) {
	return function(ctx, request)
}

func TestExecuteUsesTwoBulkCallsAndNormalizesCurrentResponse(t *testing.T) {
	target := supportedTarget()
	callCount := 0
	state, selections, err := Execute(context.Background(), target, executorFunc(func(_ context.Context, request compatibility.Request) (json.RawMessage, error) {
		callCount++
		if request.Version != 1 || request.Method != "list" {
			t.Fatalf("request = %#v", request)
		}
		switch request.API {
		case TargetAPIName:
			if got := stringSlice(t, request.JSONParameters["additional"]); !reflect.DeepEqual(got, targetAdditional) {
				t.Fatalf("target additional = %#v", got)
			}
			return fixture(t, "testdata/targets-current-v1.json"), nil
		case LUNAPIName:
			if got := stringSlice(t, request.JSONParameters["types"]); !reflect.DeepEqual(got, lunTypes) {
				t.Fatalf("LUN types = %#v", got)
			}
			if got := stringSlice(t, request.JSONParameters["additional"]); !reflect.DeepEqual(got, lunAdditional) {
				t.Fatalf("LUN additional = %#v", got)
			}
			return fixture(t, "testdata/luns-current-v1.json"), nil
		default:
			t.Fatalf("unexpected request %#v", request)
			return nil, nil
		}
	}))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if callCount != 2 {
		t.Fatalf("call count = %d, want 2 bulk calls", callCount)
	}
	if !InventorySupported(selections) || len(selections) != 2 {
		t.Fatalf("selections = %#v", selections)
	}
	if len(state.Targets) != 2 || state.Targets[0].ID != "3" || state.Targets[0].Health != "healthy" || state.Targets[0].Authentication != "mutual_chap" || state.Targets[0].ConnectedSessions != 1 {
		t.Fatalf("targets = %#v", state.Targets)
	}
	if state.Targets[1].Health != "warning" {
		t.Fatalf("second target = %#v", state.Targets[1])
	}
	if len(state.Mappings) != 1 || state.Mappings[0] != (san.Mapping{TargetID: "3", LUNID: "3a43f95a-beb7-4da5-8ea5-f9db46ba30fa"}) {
		t.Fatalf("mappings = %#v", state.Mappings)
	}
	if len(state.LUNs) != 2 || state.LUNs[0].ID != "3a43f95a-beb7-4da5-8ea5-f9db46ba30fa" || state.LUNs[0].Provisioning != san.ProvisioningThin || state.LUNs[0].BackingKind != san.BackingStoragePool || !state.LUNs[0].Mapped || state.LUNs[0].Health != "healthy" {
		t.Fatalf("LUNs = %#v", state.LUNs)
	}
	if state.LUNs[1].Provisioning != san.ProvisioningThick || state.LUNs[1].BackingKind != san.BackingVolume || state.LUNs[1].Health != "warning" {
		t.Fatalf("second LUN = %#v", state.LUNs[1])
	}
}

func TestExecuteAcceptsEmptyInventory(t *testing.T) {
	state, _, err := Execute(context.Background(), supportedTarget(), fixtureExecutor(t,
		"testdata/targets-empty-v1.json", "testdata/luns-empty-v1.json"))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if state.Targets == nil || state.LUNs == nil || state.Mappings == nil {
		t.Fatalf("empty state must use arrays, got %#v", state)
	}
	if len(state.Targets)+len(state.LUNs)+len(state.Mappings) != 0 {
		t.Fatalf("state = %#v", state)
	}
}

func TestExecuteAcceptsLegacyCompatibleMinimalResponse(t *testing.T) {
	state, _, err := Execute(context.Background(), supportedTarget(), fixtureExecutor(t,
		"testdata/targets-legacy-compatible-v1.json", "testdata/luns-legacy-compatible-v1.json"))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(state.Targets) != 1 || state.Targets[0].ID != "7" || state.Targets[0].Authentication != "chap" || state.Targets[0].Health != "unknown" {
		t.Fatalf("targets = %#v", state.Targets)
	}
	if len(state.LUNs) != 1 || state.LUNs[0].NumericID != "5" || state.LUNs[0].SizeBytes != 10*1024*1024*1024 || !state.LUNs[0].Mapped || state.LUNs[0].Health != "unknown" {
		t.Fatalf("LUNs = %#v", state.LUNs)
	}
}

func TestSelectReportsMissingSANManagerAsUnsupported(t *testing.T) {
	selections, err := Select(compatibility.NewTarget())
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if len(selections) != 2 || selections[0].Supported || selections[1].Supported || InventorySupported(selections) {
		t.Fatalf("selections = %#v", selections)
	}
	if selections[0].Operation != TargetOperationName || selections[1].Operation != LUNOperationName {
		t.Fatalf("operations = %#v", selections)
	}
}

func TestDecodersRejectMalformedSuccessfulShapes(t *testing.T) {
	for name, decode := range map[string]func(json.RawMessage) error{
		"targets root": func(raw json.RawMessage) error { _, err := decodeTargets(raw); return err },
		"LUN root":     func(raw json.RawMessage) error { _, err := decodeLUNs(raw); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := decode(json.RawMessage(`{}`)); err == nil {
				t.Fatal("decoder accepted a response without its required array")
			}
		})
	}
	if _, err := decodeTargets(json.RawMessage(`{"targets":[{"name":"missing-id"}]}`)); err == nil {
		t.Fatal("decodeTargets accepted a target without target_id")
	}
	if _, err := decodeLUNs(json.RawMessage(`{"luns":[{"lun_id":1}]}`)); err == nil {
		t.Fatal("decodeLUNs accepted a LUN without uuid")
	}
	if _, err := decodeLUNs(json.RawMessage(`{"luns":null}`)); err == nil {
		t.Fatal("decodeLUNs accepted null instead of an array")
	}
}

func supportedTarget() compatibility.Target {
	target := compatibility.NewTarget()
	target.SetAPI(TargetAPIName, compatibility.APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 1, RequestFormat: "JSON"})
	target.SetAPI(LUNAPIName, compatibility.APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 1, RequestFormat: "JSON"})
	return target
}

func fixtureExecutor(t *testing.T, targetsPath, lunsPath string) executorFunc {
	t.Helper()
	return func(_ context.Context, request compatibility.Request) (json.RawMessage, error) {
		switch request.API {
		case TargetAPIName:
			return fixture(t, targetsPath), nil
		case LUNAPIName:
			return fixture(t, lunsPath), nil
		default:
			t.Fatalf("unexpected request %#v", request)
			return nil, nil
		}
	}
}

func stringSlice(t *testing.T, value any) []string {
	t.Helper()
	items, ok := value.([]string)
	if !ok {
		t.Fatalf("value = %#v, want []string", value)
	}
	return items
}

func fixture(t *testing.T, path string) json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}
