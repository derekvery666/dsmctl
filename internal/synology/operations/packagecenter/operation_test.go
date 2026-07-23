package packagecenter

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/derekvery666/dsmctl/internal/domain/packagecenter"
	"github.com/derekvery666/dsmctl/internal/synology/compatibility"
)

type captureExecutor struct {
	responses map[string]json.RawMessage
	requests  []compatibility.Request
}

func (executor *captureExecutor) Execute(_ context.Context, request compatibility.Request) (json.RawMessage, error) {
	executor.requests = append(executor.requests, request)
	if response, ok := executor.responses[request.API+"."+request.Method]; ok {
		return response, nil
	}
	return json.RawMessage(`{}`), nil
}

func TestInventoryReadContract(t *testing.T) {
	target := compatibility.NewTarget()
	target.SetAPI(InventoryAPIName, compatibility.APIInfo{MinVersion: 1, MaxVersion: 2})
	// Shape mirrors DSM 7.3 SYNO.Core.Package.list: status, beta, startable, and
	// install_type live inside the per-package `additional` object.
	executor := &captureExecutor{responses: map[string]json.RawMessage{
		InventoryAPIName + ".list": json.RawMessage(`{"packages":[
			{"id":"SynologyDrive","name":"Synology Drive Server","version":"3.5.1-26050",
			 "additional":{"status":"running","beta":false,"startable":true,"install_type":""}},
			{"id":"WebStation","name":"Web Station","version":"3.1.0-0800",
			 "additional":{"status":"stop","beta":true,"startable":true,"install_type":"system"}},
			{"id":"Node.js_v20","name":"Node.js v20","version":"20.19.5-1014",
			 "additional":{"status":"running","beta":false,"startable":false,"install_type":""}}
		]}`),
	}}

	state, selection, err := ExecuteInventory(context.Background(), target, executor)
	if err != nil {
		t.Fatalf("ExecuteInventory() error = %v", err)
	}
	if selection.Backend != "core-package-v2" {
		t.Fatalf("inventory backend = %q", selection.Backend)
	}
	if got := executor.requests[len(executor.requests)-1].Parameters.Get("additional"); got != inventoryAdditional {
		t.Fatalf("inventory additional = %q, want %q", got, inventoryAdditional)
	}
	if len(state.Packages) != 3 {
		t.Fatalf("package count = %d", len(state.Packages))
	}
	// Running service: can stop, cannot start, removable (non-system).
	drive := state.Packages[0]
	if drive.Status != packagecenter.StatusRunning || !drive.Running || !drive.CanStop || drive.CanStart || !drive.CanUninstall || drive.Beta {
		t.Fatalf("drive package = %#v", drive)
	}
	// Stopped system service: can start, cannot stop, system install_type blocks uninstall.
	web := state.Packages[1]
	if web.Status != packagecenter.StatusStopped || web.Running || !web.Beta || !web.CanStart || web.CanStop || web.CanUninstall {
		t.Fatalf("web package = %#v (system install_type must block uninstall)", web)
	}
	// Running runtime with startable=false: neither start nor stop, still removable.
	node := state.Packages[2]
	if node.Status != packagecenter.StatusRunning || !node.Running || node.CanStart || node.CanStop || !node.CanUninstall {
		t.Fatalf("node package = %#v (non-startable package must not be stoppable)", node)
	}
}

func TestSettingsReadContract(t *testing.T) {
	target := compatibility.NewTarget()
	target.SetAPI(SettingAPIName, compatibility.APIInfo{MinVersion: 1, MaxVersion: 2})
	// Shape mirrors DSM 7.3 SYNO.Core.Package.Setting.get: trust_level is an
	// integer, and enable_autoupdate is the master auto-update toggle.
	executor := &captureExecutor{responses: map[string]json.RawMessage{
		SettingAPIName + ".get": json.RawMessage(`{"trust_level":1,"enable_autoupdate":true,
			"autoupdateall":false,"autoupdateimportant":true,"update_channel":true}`),
	}}

	settings, selection, err := ExecuteSettingsRead(context.Background(), target, executor)
	if err != nil {
		t.Fatalf("ExecuteSettingsRead() error = %v", err)
	}
	if selection.Version != 2 {
		t.Fatalf("settings read version = %d", selection.Version)
	}
	want := packagecenter.Settings{
		TrustLevel: packagecenter.TrustSynologyAndTrusted, AutoUpdateEnabled: true,
		AutoUpdateImportantOnly: true,
	}
	if !reflect.DeepEqual(settings, want) {
		t.Fatalf("settings = %#v, want %#v", settings, want)
	}
}

func TestSettingsSetContract(t *testing.T) {
	target := compatibility.NewTarget()
	target.SetAPI(SettingAPIName, compatibility.APIInfo{MinVersion: 1, MaxVersion: 2})
	executor := &captureExecutor{responses: map[string]json.RawMessage{}}

	// Auto-update: enabled + important-only. Trust level must NOT be written
	// (no DSM endpoint accepts it), even though the desired state carries one.
	desired := packagecenter.Settings{
		TrustLevel: packagecenter.TrustAny, AutoUpdateEnabled: true, AutoUpdateImportantOnly: true,
	}
	result, selection, err := ExecuteSettingsSet(context.Background(), target, executor, desired)
	if err != nil {
		t.Fatalf("ExecuteSettingsSet() error = %v", err)
	}
	want := map[string]any{
		"enable_autoupdate": true, "autoupdateimportant": true, "autoupdateall": false,
	}
	request := executor.requests[len(executor.requests)-1]
	if selection.Version != 2 || request.API != SettingAPIName || request.Method != "set" || !reflect.DeepEqual(request.JSONParameters, want) {
		t.Fatalf("settings set request = %#v, want parameters %#v", request, want)
	}
	if result.Method != "set" || result.Action != packagecenter.KindSettings {
		t.Fatalf("settings set result = %#v", result)
	}
}

func TestControlAndUninstallContract(t *testing.T) {
	target := compatibility.NewTarget()
	target.SetAPI(ControlAPIName, compatibility.APIInfo{MinVersion: 1, MaxVersion: 1})
	target.SetAPI(UninstallAPIName, compatibility.APIInfo{MinVersion: 1, MaxVersion: 1})
	executor := &captureExecutor{responses: map[string]json.RawMessage{}}

	if _, _, err := ExecuteControl(context.Background(), target, executor, ControlInput{Action: packagecenter.ActionStop, PackageID: "SynologyDrive"}); err != nil {
		t.Fatalf("ExecuteControl() error = %v", err)
	}
	control := executor.requests[len(executor.requests)-1]
	if control.API != ControlAPIName || control.Method != "stop" || control.Parameters.Get("id") != "SynologyDrive" {
		t.Fatalf("control request = %#v", control)
	}

	result, _, err := ExecuteUninstall(context.Background(), target, executor, UninstallInput{PackageID: "WebStation"})
	if err != nil {
		t.Fatalf("ExecuteUninstall() error = %v", err)
	}
	uninstall := executor.requests[len(executor.requests)-1]
	if uninstall.API != UninstallAPIName || uninstall.Method != "uninstall" || uninstall.Parameters.Get("id") != "WebStation" {
		t.Fatalf("uninstall request = %#v", uninstall)
	}
	if result.Action != packagecenter.ActionUninstall || result.PackageID != "WebStation" {
		t.Fatalf("uninstall result = %#v", result)
	}
}

func TestInstallAndUpdateFailClosed(t *testing.T) {
	target := compatibility.NewTarget()
	target.SetAPI(InventoryAPIName, compatibility.APIInfo{MinVersion: 1, MaxVersion: 2})
	for _, selectOperation := range []func(compatibility.Target) (compatibility.Selection, error){SelectInstall, SelectUpdate} {
		selection, err := selectOperation(target)
		if err == nil || selection.Supported || !compatibility.IsUnsupported(err) {
			t.Fatalf("deferred operation selected: %#v, %v", selection, err)
		}
	}
}

func TestDecodersRejectMalformed(t *testing.T) {
	if _, err := decodePackages(json.RawMessage(`{}`)); err == nil {
		t.Fatal("decodePackages() accepted a response without a packages array")
	}
	if _, err := decodePackages(json.RawMessage(`{"packages":[{"name":"x"}]}`)); err == nil {
		t.Fatal("decodePackages() accepted a package without an id")
	}
	if _, err := decodeSettings(json.RawMessage(`[]`)); err == nil {
		t.Fatal("decodeSettings() accepted a non-object response")
	}
}
