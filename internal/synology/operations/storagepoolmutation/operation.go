// Package storagepoolmutation implements typed, operation-scoped Storage
// Manager pool writes. The request shapes are taken from the DSM 7.3 local
// Storage Manager Admin Center assets and are locked by request-capture tests.
package storagepoolmutation

import (
	"context"
	"fmt"
	"sort"

	"github.com/ychiu1211/dsmctl/internal/domain/storage"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

const (
	APIName = "SYNO.Storage.CGI.Pool"

	CreateOperationName = "storage.pool.create"
	ExpandOperationName = "storage.pool.expand"
	DeleteOperationName = "storage.pool.delete"

	CreateCapabilityName = "storage.pool.create"
	ExpandCapabilityName = "storage.pool.expand"
	DeleteCapabilityName = "storage.pool.delete"
)

type Input struct {
	Action      string
	Pool        storage.PoolChange
	CurrentRAID string
}

type Result struct {
	ResourceID string `json:"resource_id,omitempty"`
	Operation  string `json:"operation"`
}

var createOperation = compatibility.Operation[Input, Result]{
	Name: CreateOperationName,
	Variants: []compatibility.Variant[Input, Result]{
		{
			Name: "storage-cgi-pool-create-v1", API: APIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(APIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input Input) (Result, error) {
				deviceType, err := deviceType(input.Pool.RAIDType, len(input.Pool.DiskIDs))
				if err != nil {
					return Result{}, err
				}
				parameters := map[string]any{
					"disk_id":          append([]string(nil), input.Pool.DiskIDs...),
					"device_type":      deviceType,
					"is_disk_check":    true,
					"is_pool_child":    false,
					"allocate_size":    "0",
					"spare_disk_count": "0",
					"desc":             input.Pool.Name,
					"is_unused":        false,
					"limitNum":         "24",
					"force":            false,
				}
				if _, err := executor.Execute(ctx, compatibility.Request{API: APIName, Version: 1, Method: "create", JSONParameters: parameters}); err != nil {
					return Result{}, fmt.Errorf("call %s.create v1: %w", APIName, err)
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
			Name: "storage-cgi-pool-expand-by-add-disk-v1", API: APIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(APIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input Input) (Result, error) {
				parameters := map[string]any{
					"space_id":               input.Pool.ID,
					"disk_id":                append([]string(nil), input.Pool.AddDiskIDs...),
					"force":                  false,
					"diskGroups":             []any{},
					"do_expand_child_volume": false,
				}
				if input.CurrentRAID == storage.RAIDSHR || input.CurrentRAID == storage.RAIDSHR2 {
					parameters["convert_shr_to_pool"] = false
				}
				if _, err := executor.Execute(ctx, compatibility.Request{API: APIName, Version: 1, Method: "expand_by_add_disk", JSONParameters: parameters}); err != nil {
					return Result{}, fmt.Errorf("call %s.expand_by_add_disk v1: %w", APIName, err)
				}
				return Result{ResourceID: input.Pool.ID, Operation: ExpandOperationName}, nil
			},
		},
	},
}

var deleteOperation = compatibility.Operation[Input, Result]{
	Name: DeleteOperationName,
	Variants: []compatibility.Variant[Input, Result]{
		{
			Name: "storage-cgi-pool-remove-v1", API: APIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(APIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input Input) (Result, error) {
				parameters := map[string]any{"space_id": input.Pool.ID, "force": true}
				if _, err := executor.Execute(ctx, compatibility.Request{API: APIName, Version: 1, Method: "remove", JSONParameters: parameters}); err != nil {
					return Result{}, fmt.Errorf("call %s.remove v1: %w", APIName, err)
				}
				return Result{ResourceID: input.Pool.ID, Operation: DeleteOperationName}, nil
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
		return Result{}, compatibility.Selection{}, fmt.Errorf("unsupported storage-pool action %q", input.Action)
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

func deviceType(raidType string, diskCount int) (string, error) {
	switch raidType {
	case storage.RAIDSHR:
		if diskCount == 1 {
			return "shr_without_disk_protect", nil
		}
		return "shr_with_1_disk_protect", nil
	case storage.RAIDSHR2:
		return "shr_with_2_disk_protect", nil
	case storage.RAID0:
		return "raid_0", nil
	case storage.RAID1:
		return "raid_1", nil
	case storage.RAID5:
		return "raid_5", nil
	case storage.RAID6:
		return "raid_6", nil
	case storage.RAID10:
		return "raid_10", nil
	case storage.RAIDJBOD:
		return "raid_linear", nil
	case storage.RAIDBasic:
		return "basic", nil
	default:
		return "", fmt.Errorf("unsupported storage-pool RAID type %q", raidType)
	}
}
