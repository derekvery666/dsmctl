// Package storagemodelconstraints reads the model-specific filesystem choices
// used by Storage Manager. DSM exposes these definitions as JavaScript rather
// than a normal JSON WebAPI envelope, so this operation uses a deliberately
// narrow raw-script executor and returns only normalized, non-sensitive data.
package storagemodelconstraints

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/derekvery666/dsmctl/internal/synology/compatibility"
)

const (
	APIName        = "SYNO.Core.Desktop.Defs"
	OperationName  = "storage.model.constraints"
	CapabilityName = "storage.model.constraints"
)

type Result struct {
	SupportedFileSystems []string
	DefaultFileSystem    string
}

type scriptExecutor interface {
	ExecuteScript(context.Context, compatibility.Request) ([]byte, error)
}

var definitionAssignment = regexp.MustCompile(`(?m)(?:var\s+)?_SYNOINFODEF\s*=\s*`)

var operation = compatibility.Operation[struct{}, Result]{
	Name: OperationName,
	Variants: []compatibility.Variant[struct{}, Result]{
		{
			Name: "desktop-defs-getjs-v1", API: APIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(APIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, _ struct{}) (Result, error) {
				raw, ok := executor.(scriptExecutor)
				if !ok {
					return Result{}, fmt.Errorf("selected executor cannot read DSM definition scripts")
				}
				script, err := raw.ExecuteScript(ctx, compatibility.Request{API: APIName, Version: 1, Method: "getjs"})
				if err != nil {
					return Result{}, fmt.Errorf("call %s.getjs v1: %w", APIName, err)
				}
				return decode(script)
			},
		},
	},
}

func APINames() []string { return operation.APINames() }

func Select(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := operation.Select(target)
	return selection, err
}

func Execute(ctx context.Context, target compatibility.Target, executor compatibility.Executor) (Result, compatibility.Selection, error) {
	return operation.Run(ctx, target, executor, struct{}{})
}

func decode(script []byte) (Result, error) {
	location := definitionAssignment.FindIndex(script)
	if location == nil {
		return Result{}, fmt.Errorf("DSM definition script does not contain _SYNOINFODEF")
	}
	decoder := json.NewDecoder(strings.NewReader(string(script[location[1]:])))
	decoder.UseNumber()
	var definitions map[string]any
	if err := decoder.Decode(&definitions); err != nil {
		return Result{}, fmt.Errorf("decode DSM model definitions: %w", err)
	}

	filesystems := make([]string, 0, 2)
	if yesDefinition(definitions["support_btrfs"]) {
		filesystems = append(filesystems, "btrfs")
	}
	if yesDefinition(definitions["supportext4"]) {
		filesystems = append(filesystems, "ext4")
	}
	sort.Strings(filesystems)
	if len(filesystems) == 0 {
		return Result{}, fmt.Errorf("DSM definition script did not advertise btrfs or ext4 support")
	}
	defaultFileSystem, _ := definitions["defaultfs"].(string)
	defaultFileSystem = strings.ToLower(strings.TrimSpace(defaultFileSystem))
	if defaultFileSystem != "btrfs" && defaultFileSystem != "ext4" {
		defaultFileSystem = ""
	}
	return Result{SupportedFileSystems: filesystems, DefaultFileSystem: defaultFileSystem}, nil
}

func yesDefinition(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "yes") || strings.EqualFold(strings.TrimSpace(typed), "true") || strings.TrimSpace(typed) == "1"
	case json.Number:
		return typed.String() == "1"
	case float64:
		return typed == 1
	default:
		return false
	}
}
