package synology

import (
	"context"
	"fmt"
	"strings"

	"github.com/ychiu1211/dsmctl/internal/domain/storage"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
	"github.com/ychiu1211/dsmctl/internal/synology/operations/storageflashcachemutation"
	"github.com/ychiu1211/dsmctl/internal/synology/operations/storageinventory"
	"github.com/ychiu1211/dsmctl/internal/synology/operations/storagemodelconstraints"
	"github.com/ychiu1211/dsmctl/internal/synology/operations/storagepoolmutation"
	"github.com/ychiu1211/dsmctl/internal/synology/operations/volumemutation"
)

type StorageState = storage.State
type StorageCapabilities = storage.Capabilities
type StorageChangeRequest = storage.ChangeRequest

type StorageMutationInput struct {
	Request               storage.ChangeRequest
	State                 storage.State
	ResolvedCapacityBytes uint64
}

type StorageMutationResult struct {
	ResourceID string `json:"resource_id,omitempty"`
	Operation  string `json:"operation"`
}

func (c *Client) StorageState(ctx context.Context) (StorageState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	apiNames := append(storageinventory.APINames(), storagemodelconstraints.APINames()...)
	apiNames = append(apiNames, storageflashcachemutation.APINames()...)
	apiNames = append(apiNames, storageflashcachemutation.ProtectionAPIName)
	if err := c.prepareCompatibilityTargetLocked(ctx, apiNames...); err != nil {
		return StorageState{}, fmt.Errorf("prepare storage inventory target: %w", err)
	}
	state, _, err := storageinventory.Execute(ctx, c.target, lockedExecutor{client: c})
	if err != nil {
		return StorageState{}, fmt.Errorf("get storage inventory: %w", err)
	}
	c.target.AddCapability(storageinventory.CapabilityName)
	cacheSelections, cacheErr := storageflashcachemutation.Select(c.target)
	if cacheErr != nil && !compatibility.IsUnsupported(cacheErr) {
		return StorageState{}, fmt.Errorf("select SSD cache backends: %w", cacheErr)
	}
	state.CacheCreation.SupportsReadOnly = storageflashcachemutation.Supported(cacheSelections, 0)
	state.CacheCreation.SupportsProtection = c.target.SupportsAPI(storageflashcachemutation.ProtectionAPIName, 1)
	state.CacheCreation.SupportsReadWrite = state.CacheCreation.SupportsReadOnly && state.CacheCreation.SupportsProtection
	modelSelection, selectErr := storagemodelconstraints.Select(c.target)
	if selectErr == nil && modelSelection.Supported {
		constraints, selection, executeErr := storagemodelconstraints.Execute(ctx, c.target, lockedExecutor{client: c})
		if executeErr != nil {
			return StorageState{}, fmt.Errorf("get storage model constraints: %w", executeErr)
		}
		state.VolumeCreation.SupportedFileSystems = append([]string(nil), constraints.SupportedFileSystems...)
		c.target.AddCapability(selection.Operation)
	} else if selectErr != nil && !compatibility.IsUnsupported(selectErr) {
		return StorageState{}, fmt.Errorf("select storage model constraints backend: %w", selectErr)
	}
	return state, nil
}

func (c *Client) StorageCapabilities(ctx context.Context) (StorageCapabilities, CompatibilityReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	apiNames := append(storageinventory.APINames(), storagemodelconstraints.APINames()...)
	apiNames = append(apiNames, storagepoolmutation.APINames()...)
	apiNames = append(apiNames, volumemutation.APINames()...)
	apiNames = append(apiNames, storageflashcachemutation.APINames()...)
	apiNames = append(apiNames, storageflashcachemutation.ProtectionAPIName)
	if err := c.prepareCompatibilityTargetLocked(ctx, apiNames...); err != nil {
		return StorageCapabilities{}, CompatibilityReport{}, fmt.Errorf("prepare storage capabilities target: %w", err)
	}
	selection, err := storageinventory.Select(c.target)
	if err != nil && !compatibility.IsUnsupported(err) {
		return StorageCapabilities{}, CompatibilityReport{}, fmt.Errorf("select storage inventory backend: %w", err)
	}
	supported := selection.Supported
	if supported {
		c.target.AddCapability(storageinventory.CapabilityName)
	}
	mutationSelections, err := storagepoolmutation.Select(c.target)
	if err != nil {
		return StorageCapabilities{}, CompatibilityReport{}, fmt.Errorf("select storage-pool mutation backends: %w", err)
	}
	poolCreate := storagepoolmutation.Supported(mutationSelections, 0)
	poolExpand := storagepoolmutation.Supported(mutationSelections, 1)
	poolDelete := storagepoolmutation.Supported(mutationSelections, 2)
	modelSelection, err := storagemodelconstraints.Select(c.target)
	if err != nil && !compatibility.IsUnsupported(err) {
		return StorageCapabilities{}, CompatibilityReport{}, fmt.Errorf("select storage model constraints backend: %w", err)
	}
	volumeSelections, err := volumemutation.Select(c.target)
	if err != nil {
		return StorageCapabilities{}, CompatibilityReport{}, fmt.Errorf("select volume mutation backends: %w", err)
	}
	volumeCreate := modelSelection.Supported && volumemutation.Supported(volumeSelections, 0)
	volumeExpand := volumemutation.Supported(volumeSelections, 1)
	volumeDelete := volumemutation.Supported(volumeSelections, 2)
	cacheSelections, err := storageflashcachemutation.Select(c.target)
	if err != nil {
		return StorageCapabilities{}, CompatibilityReport{}, fmt.Errorf("select SSD cache backends: %w", err)
	}
	cacheCreate := storageflashcachemutation.Supported(cacheSelections, 0)
	cacheDelete := storageflashcachemutation.Supported(cacheSelections, 1)
	capabilities := StorageCapabilities{
		InventoryRead: supported,
		DiskStatus:    supported,
		PoolStatus:    supported,
		VolumeStatus:  supported,
		PoolCreate:    poolCreate,
		PoolUpdate:    poolExpand,
		PoolDelete:    poolDelete,
		VolumeCreate:  volumeCreate,
		VolumeUpdate:  volumeExpand,
		VolumeDelete:  volumeDelete,
		CacheStatus:   supported,
		CacheCreate:   cacheCreate,
		CacheDelete:   cacheDelete,
		// This DSM's SSD cache API exposes only create and remove; expand and
		// mode conversion have no backend method and stay fail-closed.
		CacheExpand:  false,
		CacheConvert: false,
		Mutations:    poolCreate || poolExpand || poolDelete || volumeCreate || volumeExpand || volumeDelete || cacheCreate || cacheDelete,
	}
	c.updateDerivedCapabilitiesLocked()
	selections := append([]compatibility.Selection{selection, modelSelection}, mutationSelections...)
	selections = append(selections, volumeSelections...)
	selections = append(selections, cacheSelections...)
	return capabilities, c.target.Report(selections...), nil
}

// ApplyStorageChange executes the independently selected pool or volume
// operation for this DSM target. Planning, stale-state checks, and
// postcondition verification remain in the shared CLI/MCP application layer.
func (c *Client) ApplyStorageChange(ctx context.Context, input StorageMutationInput) (StorageMutationResult, error) {
	request, state := input.Request, input.State
	if request.Resource == storage.ResourcePool && request.Action == storage.ActionUpdate && request.Pool != nil && request.Pool.TargetRAIDType != nil {
		return StorageMutationResult{}, fmt.Errorf("storage-pool RAID migration is not implemented")
	}
	if request.Resource == storage.ResourcePool {
		if request.Pool == nil || request.Volume != nil {
			return StorageMutationResult{}, fmt.Errorf("storage-pool backend requires exactly one pool intent")
		}
		return c.applyStoragePoolChange(ctx, request, state)
	}
	if request.Resource == storage.ResourceVolume {
		if request.Volume == nil || request.Pool != nil {
			return StorageMutationResult{}, fmt.Errorf("volume backend requires exactly one volume intent")
		}
		return c.applyVolumeChange(ctx, request, state, input.ResolvedCapacityBytes)
	}
	if request.Resource == storage.ResourceCache {
		if request.Cache == nil || request.Pool != nil || request.Volume != nil {
			return StorageMutationResult{}, fmt.Errorf("SSD cache backend requires exactly one cache intent")
		}
		return c.applyCacheChange(ctx, request, state)
	}
	return StorageMutationResult{}, fmt.Errorf("unsupported storage resource %q", request.Resource)
}

// applyCacheChange resolves the DSM reference path (parent volume identifier) and
// the device RAID string from observed state, then runs the selected SSD cache
// operation. Only create and delete are backed on this DSM; expand and convert
// are rejected earlier in the application layer.
func (c *Client) applyCacheChange(ctx context.Context, request StorageChangeRequest, state StorageState) (StorageMutationResult, error) {
	change := request.Cache
	referencePath, cacheType, protectionRAID := "", "", change.ProtectionRAID
	switch request.Action {
	case storage.ActionCreate:
		referencePath = change.VolumeID
		cacheType = change.CacheType
	case storage.ActionDelete:
		for _, cache := range state.Caches {
			if cache.ID == change.ID {
				referencePath = cache.VolumeID
				cacheType = cache.CacheType
				break
			}
		}
		if referencePath == "" {
			return StorageMutationResult{}, fmt.Errorf("SSD cache %q is missing from observed state", change.ID)
		}
	default:
		return StorageMutationResult{}, fmt.Errorf("unsupported SSD cache action %q", request.Action)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareCompatibilityTargetLocked(ctx, storageflashcachemutation.APINames()...); err != nil {
		return StorageMutationResult{}, fmt.Errorf("prepare SSD cache mutation target: %w", err)
	}
	input := storageflashcachemutation.Input{
		Action:        request.Action,
		CacheType:     cacheType,
		ReferencePath: referencePath,
		RAIDType:      cacheDeviceType(cacheType, protectionRAID, len(change.DiskIDs)),
		DiskIDs:       change.DiskIDs,
	}
	if request.Action == storage.ActionCreate {
		size, sizeErr := storageflashcachemutation.EstimateRAIDSize(ctx, lockedExecutor{client: c}, change.DiskIDs, input.RAIDType)
		if sizeErr != nil {
			return StorageMutationResult{}, fmt.Errorf("resolve SSD cache size: %w", sizeErr)
		}
		input.SizeBytes = size
		input.IsMax = true
	}
	result, selection, err := storageflashcachemutation.Execute(ctx, c.target, lockedExecutor{client: c}, input)
	if err != nil {
		return StorageMutationResult{}, fmt.Errorf("apply SSD cache %s: %w", request.Action, err)
	}
	c.target.AddCapability(selection.Operation)
	return StorageMutationResult{ResourceID: result.ResourceID, Operation: result.Operation}, nil
}

// cacheDeviceType maps the normalized cache mode and protection RAID to DSM's
// flashcache device string. A read-write cache uses its protection RAID
// (raid_1, raid_5, or raid_6); a read-only cache stripes across its SSDs as
// raid_0, or uses basic for a single SSD.
func cacheDeviceType(cacheType, protectionRAID string, diskCount int) string {
	if cacheType == storage.CacheModeReadWrite {
		switch protectionRAID {
		case storage.RAID1:
			return "raid_1"
		case storage.RAID5:
			return "raid_5"
		case storage.RAID6:
			return "raid_6"
		}
	}
	if diskCount <= 1 {
		return "basic"
	}
	return "raid_0"
}

func (c *Client) applyStoragePoolChange(ctx context.Context, request StorageChangeRequest, state StorageState) (StorageMutationResult, error) {

	currentRAID := ""
	if request.Action != storage.ActionCreate {
		for _, pool := range state.Pools {
			if pool.ID == request.Pool.ID {
				currentRAID = normalizeStorageRAID(pool.RAIDType)
				break
			}
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareCompatibilityTargetLocked(ctx, storagepoolmutation.APINames()...); err != nil {
		return StorageMutationResult{}, fmt.Errorf("prepare storage-pool mutation target: %w", err)
	}
	result, selection, err := storagepoolmutation.Execute(ctx, c.target, lockedExecutor{client: c}, storagepoolmutation.Input{
		Action: request.Action, Pool: *request.Pool, CurrentRAID: currentRAID,
	})
	if err != nil {
		return StorageMutationResult{}, fmt.Errorf("apply storage-pool %s: %w", request.Action, err)
	}
	c.target.AddCapability(selection.Operation)
	return StorageMutationResult{ResourceID: result.ResourceID, Operation: result.Operation}, nil
}

func (c *Client) applyVolumeChange(ctx context.Context, request StorageChangeRequest, state StorageState, resolvedCapacityBytes uint64) (StorageMutationResult, error) {
	poolID := request.Volume.PoolID
	var observedVolume storage.Volume
	if request.Action != storage.ActionCreate {
		for _, volume := range state.Volumes {
			if volume.ID == request.Volume.ID {
				observedVolume = volume
				poolID = volume.PoolID
				break
			}
		}
		if observedVolume.ID == "" {
			return StorageMutationResult{}, fmt.Errorf("volume %q is missing from observed state", request.Volume.ID)
		}
	}
	var pool storage.Pool
	for _, candidate := range state.Pools {
		if candidate.ID == poolID {
			pool = candidate
			break
		}
	}
	if pool.ID == "" {
		return StorageMutationResult{}, fmt.Errorf("parent storage pool %q is missing from observed state", poolID)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareCompatibilityTargetLocked(ctx, volumemutation.APINames()...); err != nil {
		return StorageMutationResult{}, fmt.Errorf("prepare volume mutation target: %w", err)
	}
	result, selection, err := volumemutation.Execute(ctx, c.target, lockedExecutor{client: c}, volumemutation.Input{
		Action: request.Action, Volume: *request.Volume, PoolPath: pool.Path, SpacePath: pool.SpacePath,
		SingleVolume:          pool.Layout == "single" || observedVolume.SingleVolume,
		ResolvedCapacityBytes: resolvedCapacityBytes,
	})
	if err != nil {
		return StorageMutationResult{}, fmt.Errorf("apply volume %s: %w", request.Action, err)
	}
	c.target.AddCapability(selection.Operation)
	return StorageMutationResult{ResourceID: result.ResourceID, Operation: result.Operation}, nil
}

func normalizeStorageRAID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "shr", "shr_without_disk_protect", "shr_with_1_disk_protect":
		return storage.RAIDSHR
	case "shr2", "shr_2", "shr_with_2_disk_protect":
		return storage.RAIDSHR2
	case "raid_0", "raid0":
		return storage.RAID0
	case "raid_1", "raid1":
		return storage.RAID1
	case "raid_5", "raid5":
		return storage.RAID5
	case "raid_6", "raid6":
		return storage.RAID6
	case "raid_10", "raid10":
		return storage.RAID10
	case "raid_linear", "jbod":
		return storage.RAIDJBOD
	case "basic":
		return storage.RAIDBasic
	default:
		return value
	}
}
