package storagemodelconstraints

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

func TestExecuteParsesSanitizedModelDefinitions(t *testing.T) {
	target := compatibility.NewTarget()
	target.SetAPI(APIName, compatibility.APIInfo{Path: "entry.cgi", MinVersion: 1, MaxVersion: 1})
	executor := scriptExecutorFixture{script: []byte(`window.before=true;var _SYNOINFODEF={"support_btrfs":"yes","supportext4":"yes","defaultfs":"btrfs","unrelated":"discarded"};function _D(){}`)}

	result, selection, err := Execute(context.Background(), target, executor)
	if err != nil {
		t.Fatal(err)
	}
	if selection.Operation != OperationName || selection.Backend != "desktop-defs-getjs-v1" {
		t.Fatalf("selection = %#v", selection)
	}
	if strings.Join(result.SupportedFileSystems, ",") != "btrfs,ext4" || result.DefaultFileSystem != "btrfs" {
		t.Fatalf("result = %#v", result)
	}
}

func TestDecodeFailsClosedWhenCapabilitiesAreMissingOrMalformed(t *testing.T) {
	for _, script := range []string{
		`window.other={};`,
		`var _SYNOINFODEF=not_json;`,
		`var _SYNOINFODEF={"support_btrfs":"no","supportext4":"no"};`,
	} {
		if _, err := decode([]byte(script)); err == nil {
			t.Fatalf("decode(%q) unexpectedly succeeded", script)
		}
	}
}

type scriptExecutorFixture struct{ script []byte }

func (executor scriptExecutorFixture) Execute(context.Context, compatibility.Request) (json.RawMessage, error) {
	return nil, nil
}

func (executor scriptExecutorFixture) ExecuteScript(_ context.Context, request compatibility.Request) ([]byte, error) {
	if request.API != APIName || request.Version != 1 || request.Method != "getjs" {
		panic("unexpected request")
	}
	return executor.script, nil
}
