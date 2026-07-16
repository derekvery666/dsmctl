package controlpaneltime

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/domain/controlpanel"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

type executorFunc func(context.Context, compatibility.Request) (json.RawMessage, error)

func (function executorFunc) Execute(ctx context.Context, request compatibility.Request) (json.RawMessage, error) {
	return function(ctx, request)
}

func TestV3VariantNormalizesTimeConfiguration(t *testing.T) {
	target := targetWithVersions(1, 3)
	state, selection, err := Execute(context.Background(), target, executorFunc(func(_ context.Context, request compatibility.Request) (json.RawMessage, error) {
		if request.API != APIName || request.Version != 3 || request.Method != "get" {
			t.Fatalf("request = %#v", request)
		}
		if len(request.Parameters) != 0 || request.JSONParameters != nil {
			t.Fatalf("read request carried parameters: %#v", request)
		}
		return fixture(t, "testdata/current-v3.json"), nil
	}))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if selection.Backend != "core-region-ntp-v3" || selection.Version != 3 {
		t.Fatalf("selection = %#v", selection)
	}
	if state.TimeZone != "Taipei" || state.DateFormat != "Y-m-d" || state.TimeFormat != "H:i" || state.SynchronizationMode != controlpanel.TimeSynchronizationNTP {
		t.Fatalf("state = %#v", state)
	}
	wantServers := []string{"time.google.com", "pool.ntp.org"}
	if !reflect.DeepEqual(state.NTPServers, wantServers) {
		t.Fatalf("NTPServers = %#v, want %#v", state.NTPServers, wantServers)
	}
}

func TestV2VariantIsSelectedIndependently(t *testing.T) {
	target := targetWithVersions(1, 2)
	_, selection, err := Execute(context.Background(), target, executorFunc(func(_ context.Context, request compatibility.Request) (json.RawMessage, error) {
		if request.Version != 2 {
			t.Fatalf("request version = %d", request.Version)
		}
		return fixture(t, "testdata/current-v3.json"), nil
	}))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if selection.Backend != "core-region-ntp-v2" || selection.Version != 2 {
		t.Fatalf("selection = %#v", selection)
	}
}

func TestLegacyV1AllowsUnavailableDisplayFormats(t *testing.T) {
	target := targetWithVersions(1, 1)
	state, selection, err := Execute(context.Background(), target, executorFunc(func(_ context.Context, request compatibility.Request) (json.RawMessage, error) {
		if request.Version != 1 {
			t.Fatalf("request version = %d", request.Version)
		}
		return fixture(t, "testdata/legacy-v1.json"), nil
	}))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if selection.Backend != "core-region-ntp-v1-legacy" {
		t.Fatalf("selection = %#v", selection)
	}
	if state.DateFormat != "" || state.TimeFormat != "" || state.SynchronizationMode != controlpanel.TimeSynchronizationManual || len(state.NTPServers) != 0 {
		t.Fatalf("state = %#v", state)
	}
}

func TestUnsupportedTargetDoesNotExecute(t *testing.T) {
	called := false
	_, selection, err := Execute(context.Background(), compatibility.NewTarget(), executorFunc(func(context.Context, compatibility.Request) (json.RawMessage, error) {
		called = true
		return nil, nil
	}))
	if !compatibility.IsUnsupported(err) {
		t.Fatalf("Execute() error = %v, want unsupported", err)
	}
	if selection.Supported || selection.Operation != OperationName || called {
		t.Fatalf("selection=%#v called=%v", selection, called)
	}
}

func TestDecoderRejectsMalformedShapes(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "not object", data: `[]`, want: "expected an object"},
		{name: "missing timezone", data: `{"enable_ntp":"manual"}`, want: `field "timezone" is missing`},
		{name: "wrong timezone type", data: `{"timezone":7,"enable_ntp":"manual"}`, want: `field "timezone"`},
		{name: "unknown mode", data: `{"timezone":"UTC","date_format":"Y-m-d","time_format":"H:i","enable_ntp":"automatic"}`, want: "unsupported enable_ntp"},
		{name: "missing v3 format", data: `{"timezone":"UTC","enable_ntp":"manual"}`, want: `field "date_format" is missing`},
		{name: "ntp without server", data: `{"timezone":"UTC","date_format":"Y-m-d","time_format":"H:i","enable_ntp":"ntp","server":""}`, want: "has no configured server"},
		{name: "wrong server type", data: `{"timezone":"UTC","date_format":"Y-m-d","time_format":"H:i","enable_ntp":"ntp","server":[]}`, want: `field "server"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := decode(json.RawMessage(test.data), true)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("decode() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestAPINamesAreScopedToTimeModule(t *testing.T) {
	if names := APINames(); !reflect.DeepEqual(names, []string{APIName}) {
		t.Fatalf("APINames() = %#v", names)
	}
}

func targetWithVersions(minimum, maximum int) compatibility.Target {
	target := compatibility.NewTarget()
	target.SetAPI(APIName, compatibility.APIInfo{Path: "entry.cgi", MinVersion: minimum, MaxVersion: maximum})
	return target
}

func fixture(t *testing.T, path string) json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}
