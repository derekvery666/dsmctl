// Package volumemutation implements typed, independently selectable Storage
// Manager volume writes. Request shapes are based on the DSM 7.3 local Storage
// Manager Admin Center assets and are locked by request-capture tests.
package volumemutation

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/ychiu1211/dsmctl/internal/domain/storage"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

const (
	APIName = "SYNO.Storage.CGI.Volume"

	CreateOperationName = "storage.volume.create"
	ExpandOperationName = "storage.volume.expand"
	DeleteOperationName = "storage.volume.delete"

	CreateCapabilityName = "storage.volume.create"
	ExpandCapabilityName = "storage.volume.expand"
	DeleteCapabilityName = "storage.volume.delete"
)

const mebibyte = uint64(1) << 20

type Input struct {
	Action                string
	Volume                storage.VolumeChange
	PoolPath              string
	SpacePath             string
	SingleVolume          bool
	ResolvedCapacityBytes uint64
}

type Result struct {
	ResourceID string `json:"resource_id,omitempty"`
	Operation  string `json:"operation"`
}

var createOperation = compatibility.Operation[Input, Result]{
	Name: CreateOperationName,
	Variants: []compatibility.Variant[Input, Result]{
		{
			Name: "storage-cgi-volume-create-v1", API: APIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(APIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input Input) (Result, error) {
				parameters := map[string]any{
					"fs_type":       input.Volume.FileSystem,
					"vol_attr":      "generic",
					"vol_desc":      input.Volume.Name,
					"atime_opt":     "noatime",
					"force":         false,
					"enable_dedupe": false,
				}
				method := "create_on_existing_pool"
				if input.SingleVolume {
					if input.SpacePath == "" {
						return Result{}, fmt.Errorf("single-volume creation requires stable DSM space_path")
					}
					method = "deploy_unused"
					parameters["space_path"] = input.SpacePath
				} else {
					if input.PoolPath == "" {
						return Result{}, fmt.Errorf("multi-volume creation requires stable DSM pool_path")
					}
					allocation, err := allocationMiB(input.ResolvedCapacityBytes)
					if err != nil {
						return Result{}, err
					}
					parameters["pool_path"] = input.PoolPath
					parameters["allocate_size"] = allocation
				}
				if _, err := executor.Execute(ctx, compatibility.Request{API: APIName, Version: 1, Method: method, JSONParameters: parameters}); err != nil {
					return Result{}, fmt.Errorf("call %s.%s v1: %w", APIName, method, err)
				}
				return Result{Operation: CreateOperationName}, nil
			},
		},
	},
}

var expandOperation = compatibility.Operation[Input, Result]{
	Name: ExpandOperationName,
	Variants: []compatibility.Variant[Input, Result]{
		{
			Name: "storage-cgi-volume-expand-v1", API: APIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(APIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input Input) (Result, error) {
				parameters := map[string]any{"space_id": input.Volume.ID}
				method := "expand_unallocated"
				if !input.SingleVolume {
					if input.ResolvedCapacityBytes == 0 {
						return Result{}, fmt.Errorf("multi-volume expansion requires a resolved target capacity")
					}
					method = "expand_pool_child"
					parameters["new_size"] = strconv.FormatUint(input.ResolvedCapacityBytes, 10)
				}
				if _, err := executor.Execute(ctx, compatibility.Request{API: APIName, Version: 1, Method: method, JSONParameters: parameters}); err != nil {
					return Result{}, fmt.Errorf("call %s.%s v1: %w", APIName, method, err)
				}
				return Result{ResourceID: input.Volume.ID, Operation: ExpandOperationName}, nil
			},
		},
	},
}

var deleteOperation = compatibility.Operation[Input, Result]{
	Name: DeleteOperationName,
	Variants: []compatibility.Variant[Input, Result]{
		{
			Name: "storage-cgi-volume-delete-v1", API: APIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(APIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input Input) (Result, error) {
				parameters := map[string]any{"space_id": []string{input.Volume.ID}, "force": true}
				if _, err := executor.Execute(ctx, compatibility.Request{API: APIName, Version: 1, Method: "delete", JSONParameters: parameters}); err != nil {
					return Result{}, fmt.Errorf("call %s.delete v1: %w", APIName, err)
				}
				return Result{ResourceID: input.Volume.ID, Operation: DeleteOperationName}, nil
			},
		},
	},
}

func APINames() []string {
	names := append(createOperation.APINames(), expandOperation.APINames()...)
	names = append(names, deleteOperation.APINames()...)
	sort.Strings(names)
	result := names[:0]
	for _, name := range names {
		if len(result) == 0 || result[len(result)-1] != name {
			result = append(result, name)
		}
	}
	return result
}

func Select(target compatibility.Target) ([]compatibility.Selection, error) {
	selectors := []func(compatibility.Target) (compatibility.Selection, error){selectCreate, selectExpand, selectDelete}
	selections := make([]compatibility.Selection, 0, len(selectors))
	for _, selector := range selectors {
		selection, err := selector(target)
		selections = append(selections, selection)
		if err != nil && !compatibility.IsUnsupported(err) {
			return nil, err
		}
	}
	return selections, nil
}

func Execute(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input Input) (Result, compatibility.Selection, error) {
	switch input.Action {
	case storage.ActionCreate:
		return createOperation.Run(ctx, target, executor, input)
	case storage.ActionUpdate:
		return expandOperation.Run(ctx, target, executor, input)
	case storage.ActionDelete:
		return deleteOperation.Run(ctx, target, executor, input)
	default:
		return Result{}, compatibility.Selection{}, fmt.Errorf("unsupported volume action %q", input.Action)
	}
}

func Supported(selections []compatibility.Selection, index int) bool {
	return index >= 0 && index < len(selections) && selections[index].Supported
}

func selectCreate(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := createOperation.Select(target)
	return selection, err
}

func selectExpand(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := expandOperation.Select(target)
	return selection, err
}

func selectDelete(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := deleteOperation.Select(target)
	return selection, err
}

func allocationMiB(bytes uint64) (string, error) {
	if bytes == 0 || bytes%mebibyte != 0 {
		return "", fmt.Errorf("volume allocation %d bytes must be a positive whole MiB value", bytes)
	}
	return strconv.FormatUint(bytes/mebibyte, 10), nil
}
