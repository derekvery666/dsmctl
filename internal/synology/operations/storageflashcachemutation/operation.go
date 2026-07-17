// Package storageflashcachemutation implements typed, operation-scoped Storage
// Manager SSD cache (DSM "flashcache") writes. DSM exposes SSD cache creation and
// removal as SYNO.Storage.CGI.Flashcache "enable" and "remove"; the request
// shapes were captured from the DSM 7.3 local Storage Manager Admin Center assets
// and are locked by request-capture tests. This DSM has no separate expand or
// convert method, so those actions are not selectable here and fail closed in the
// application layer.
package storageflashcachemutation

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/ychiu1211/dsmctl/internal/domain/storage"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

const (
	APIName = "SYNO.Storage.CGI.Flashcache"
	// ProtectionAPIName backs the RAID protection required for a read-write
	// cache. Read-write support is only reported when this API is present.
	ProtectionAPIName = "SYNO.Storage.CGI.Cache.Protection"

	CreateOperationName = "storage.cache.create"
	DeleteOperationName = "storage.cache.delete"

	CreateCapabilityName = "storage.cache.create"
	DeleteCapabilityName = "storage.cache.delete"
)

// Input is the typed cross-layer SSD cache write intent. The application layer
// resolves ReferencePath (the parent volume identifier DSM addresses the cache
// by), the DSM RAIDType device string, and the byte Size before apply.
type Input struct {
	Action        string
	CacheType     string   // storage.CacheModeReadOnly or storage.CacheModeReadWrite
	ReferencePath string   // parent volume identifier, e.g. "volume_1"
	RAIDType      string   // DSM device string: raid_0, raid_1, raid_5, raid_6, basic
	DiskIDs       []string // stable SSD identifiers backing the cache
	SizeBytes     uint64   // resolved cache capacity in bytes; DSM rejects a zero size
	IsMax         bool     // whether SizeBytes is the backend maximum
}

type Result struct {
	ResourceID string `json:"resource_id,omitempty"`
	Operation  string `json:"operation"`
}

var createOperation = compatibility.Operation[Input, Result]{
	Name: CreateOperationName,
	Variants: []compatibility.Variant[Input, Result]{
		{
			Name: "storage-cgi-flashcache-enable-v1", API: APIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(APIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input Input) (Result, error) {
				mode, err := cacheMode(input.CacheType)
				if err != nil {
					return Result{}, err
				}
				parameters := map[string]any{
					"cacheMode":      mode,
					"reference_path": input.ReferencePath,
					"create_type":    "shared_cache_and_alloc_cache",
					"raidType":       input.RAIDType,
					"disk_id":        append([]string(nil), input.DiskIDs...),
					"isMax":          input.IsMax,
					"size":           strconv.FormatUint(input.SizeBytes, 10),
					"skipSeqIO":      true,
					"metadataCache":  false,
					"force":          false,
					"check_lock":     true,
				}
				if _, err := executor.Execute(ctx, compatibility.Request{API: APIName, Version: 1, Method: "enable", JSONParameters: parameters}); err != nil {
					return Result{}, fmt.Errorf("call %s.enable v1: %w", APIName, err)
				}
				return Result{Operation: CreateOperationName}, nil
			},
		},
	},
}

var deleteOperation = compatibility.Operation[Input, Result]{
	Name: DeleteOperationName,
	Variants: []compatibility.Variant[Input, Result]{
		{
			Name: "storage-cgi-flashcache-remove-v1", API: APIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(APIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input Input) (Result, error) {
				parameters := map[string]any{
					"reference_path": input.ReferencePath,
					"check_lock":     true,
				}
				if _, err := executor.Execute(ctx, compatibility.Request{API: APIName, Version: 1, Method: "remove", JSONParameters: parameters}); err != nil {
					return Result{}, fmt.Errorf("call %s.remove v1: %w", APIName, err)
				}
				return Result{ResourceID: input.ReferencePath, Operation: DeleteOperationName}, nil
			},
		},
	},
}

func APINames() []string {
	names := append(createOperation.APINames(), deleteOperation.APINames()...)
	sort.Strings(names)
	result := names[:0]
	for _, name := range names {
		if len(result) == 0 || result[len(result)-1] != name {
			result = append(result, name)
		}
	}
	return result
}

// Select returns a fixed-index slice: index 0 is create, index 1 is delete.
func Select(target compatibility.Target) ([]compatibility.Selection, error) {
	selectors := []func(compatibility.Target) (compatibility.Selection, error){selectCreate, selectDelete}
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
	case storage.ActionDelete:
		return deleteOperation.Run(ctx, target, executor, input)
	default:
		return Result{}, compatibility.Selection{}, fmt.Errorf("unsupported SSD cache action %q", input.Action)
	}
}

func Supported(selections []compatibility.Selection, index int) bool {
	return index >= 0 && index < len(selections) && selections[index].Supported
}

func selectCreate(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := createOperation.Select(target)
	return selection, err
}

func selectDelete(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := deleteOperation.Select(target)
	return selection, err
}

func cacheMode(cacheType string) (string, error) {
	switch cacheType {
	case storage.CacheModeReadOnly:
		return "readCache", nil
	case storage.CacheModeReadWrite:
		return "writeCache", nil
	default:
		return "", fmt.Errorf("unsupported SSD cache type %q", cacheType)
	}
}

// EstimateRAIDSize asks DSM for the byte capacity a cache of the given SSDs and
// RAID type would provide. Cache creation requires an explicit non-zero size, so
// the facade resolves the maximum with this read-only call before enable.
func EstimateRAIDSize(ctx context.Context, executor compatibility.Executor, diskIDs []string, raidType string) (uint64, error) {
	data, err := executor.Execute(ctx, compatibility.Request{API: APIName, Version: 1, Method: "estimate_raid_size", JSONParameters: map[string]any{
		"cache_devices": append([]string(nil), diskIDs...),
		"raid_type":     raidType,
	}})
	if err != nil {
		return 0, fmt.Errorf("call %s.estimate_raid_size v1: %w", APIName, err)
	}
	var estimate struct {
		SizeByte  json.Number `json:"size_byte"`
		Size      json.Number `json:"size"`
		RAIDSize  json.Number `json:"raid_size"`
		RAIDBytes json.Number `json:"raid_size_byte"`
	}
	if err := json.Unmarshal(data, &estimate); err != nil {
		return 0, fmt.Errorf("decode %s.estimate_raid_size result: %w", APIName, err)
	}
	for _, candidate := range []json.Number{estimate.SizeByte, estimate.Size, estimate.RAIDSize, estimate.RAIDBytes} {
		if value, convErr := strconv.ParseUint(candidate.String(), 10, 64); convErr == nil && value > 0 {
			return value, nil
		}
	}
	return 0, fmt.Errorf("%s.estimate_raid_size returned no usable size", APIName)
}
