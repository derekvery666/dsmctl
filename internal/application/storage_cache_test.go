package application

import (
	"strings"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/domain/storage"
)

func cacheContractState() storage.State {
	return storage.State{
		Disks: []storage.Disk{
			{ID: "sda", Type: "SSD", Serial: "ssd-a", SizeBytes: 240_000_000_000, Status: "sys_partition_normal", Health: "normal", SMARTStatus: "normal", Selectable: true, Compatibility: "support"},
			{ID: "sdb", Type: "SSD", Serial: "ssd-b", SizeBytes: 240_000_000_000, Status: "sys_partition_normal", Health: "normal", SMARTStatus: "normal", Selectable: true, Compatibility: "support"},
			{ID: "sdc", Type: "HDD", Serial: "hdd-c", SizeBytes: 1_000_000_000_000, Status: "normal", Health: "normal", SMARTStatus: "normal", Selectable: true, Compatibility: "support", UsedBy: "reuse_1", InUse: true},
		},
		Pools:   []storage.Pool{{ID: "reuse_1", RAIDType: "raid_5", Status: "normal", Health: "normal", DiskIDs: []string{"sdc"}, Writable: true}},
		Volumes: []storage.Volume{{ID: "volume_1", Path: "/volume1", PoolID: "reuse_1", FileSystem: "btrfs", Status: "normal", Health: "normal", Writable: true, SizeBytes: 900_000_000_000}},
		CacheCreation: storage.CacheCreationConstraints{
			SupportsReadOnly: true, SupportsReadWrite: true, SupportsProtection: true,
			ProtectionRAIDTypes: []string{storage.RAID1, storage.RAID5, storage.RAID6},
			MinReadOnlyDisks:    1, MinReadWriteDisks: 2, MaxDisks: 6,
		},
	}
}

func readOnlyCacheCreate() storage.ChangeRequest {
	return storage.ChangeRequest{Action: storage.ActionCreate, Resource: storage.ResourceCache, Cache: &storage.CacheChange{
		Name: "cache", VolumeID: "volume_1", CacheType: storage.CacheModeReadOnly, DiskIDs: []string{"sda"},
	}}
}

func TestBuildStoragePlanReadOnlyCacheCreate(t *testing.T) {
	plan, err := BuildStoragePlan("lab", cacheContractState(), readOnlyCacheCreate())
	if err != nil {
		t.Fatalf("BuildStoragePlan() error = %v", err)
	}
	if plan.Hash == "" || plan.TopologyFingerprint == "" || plan.SafetyFingerprint == "" {
		t.Fatalf("plan missing hashes: %#v", plan)
	}
	if plan.Destructive || plan.Risk != "medium" {
		t.Fatalf("read-only cache create should be non-destructive medium risk: %#v", plan)
	}
	if plan.References.CacheVolumePath != "volume_1" {
		t.Fatalf("references = %#v", plan.References)
	}
}

func TestBuildStoragePlanReadWriteCacheCreate(t *testing.T) {
	request := storage.ChangeRequest{Action: storage.ActionCreate, Resource: storage.ResourceCache, Cache: &storage.CacheChange{
		Name: "cache", VolumeID: "volume_1", CacheType: storage.CacheModeReadWrite, ProtectionRAID: storage.RAID1, DiskIDs: []string{"sda", "sdb"},
	}}
	plan, err := BuildStoragePlan("lab", cacheContractState(), request)
	if err != nil {
		t.Fatalf("BuildStoragePlan() error = %v", err)
	}
	if plan.Destructive {
		t.Fatalf("read-write create is not itself destructive: %#v", plan)
	}
}

func TestBuildStoragePlanCacheCreateRejectsHDD(t *testing.T) {
	state := cacheContractState()
	// A free (unpooled) HDD passes the availability check but must be refused as
	// non-SSD cache media.
	state.Disks = append(state.Disks, storage.Disk{ID: "sdd", Type: "HDD", Serial: "hdd-d", Status: "normal", Health: "normal", SMARTStatus: "normal", Selectable: true, Compatibility: "support"})
	request := readOnlyCacheCreate()
	request.Cache.DiskIDs = []string{"sdd"}
	if _, err := BuildStoragePlan("lab", state, request); err == nil || !strings.Contains(err.Error(), "SSD") {
		t.Fatalf("expected SSD-media rejection, got %v", err)
	}
}

func TestBuildStoragePlanCacheCreateRejectsAlreadyCachedVolume(t *testing.T) {
	state := cacheContractState()
	state.Caches = []storage.Cache{{ID: "cache_1", VolumeID: "volume_1", CacheType: storage.CacheModeReadOnly, DiskIDs: []string{"sdb"}}}
	if _, err := BuildStoragePlan("lab", state, readOnlyCacheCreate()); err == nil || !strings.Contains(err.Error(), "already has SSD cache") {
		t.Fatalf("expected already-cached rejection, got %v", err)
	}
}

func TestBuildStoragePlanReadWriteCacheRequiresProtection(t *testing.T) {
	request := storage.ChangeRequest{Action: storage.ActionCreate, Resource: storage.ResourceCache, Cache: &storage.CacheChange{
		Name: "cache", VolumeID: "volume_1", CacheType: storage.CacheModeReadWrite, DiskIDs: []string{"sda", "sdb"},
	}}
	if _, err := BuildStoragePlan("lab", cacheContractState(), request); err == nil || !strings.Contains(err.Error(), "protection_raid") {
		t.Fatalf("expected protection_raid requirement, got %v", err)
	}
}

func TestBuildStoragePlanReadOnlyCacheRejectsProtection(t *testing.T) {
	request := readOnlyCacheCreate()
	request.Cache.ProtectionRAID = storage.RAID1
	if _, err := BuildStoragePlan("lab", cacheContractState(), request); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only protection rejection, got %v", err)
	}
}

func TestBuildStoragePlanCacheDeleteRiskByMode(t *testing.T) {
	base := cacheContractState()
	cases := []struct {
		mode            string
		wantDestructive bool
		wantRisk        string
	}{
		{storage.CacheModeReadOnly, false, "medium"},
		{storage.CacheModeReadWrite, true, "high"},
	}
	for _, test := range cases {
		state := base
		state.Caches = []storage.Cache{{ID: "cache_1", VolumeID: "volume_1", CacheType: test.mode, DiskIDs: []string{"sda"}, Status: "normal", Health: "normal", CanDelete: true}}
		plan, err := BuildStoragePlan("lab", state, storage.ChangeRequest{Action: storage.ActionDelete, Resource: storage.ResourceCache, Cache: &storage.CacheChange{ID: "cache_1"}})
		if err != nil {
			t.Fatalf("%s delete plan error = %v", test.mode, err)
		}
		if plan.Destructive != test.wantDestructive || plan.Risk != test.wantRisk {
			t.Fatalf("%s delete: destructive=%v risk=%q, want %v/%q", test.mode, plan.Destructive, plan.Risk, test.wantDestructive, test.wantRisk)
		}
		if test.wantDestructive && len(plan.DestructiveConsequences) == 0 {
			t.Fatalf("%s delete should enumerate a dirty-flush consequence", test.mode)
		}
	}
}

func TestCacheActionSupportedGating(t *testing.T) {
	capabilities := storage.Capabilities{CacheCreate: true, CacheDelete: true, CacheExpand: false, CacheConvert: false}
	if !storageActionSupported(capabilities, storage.ResourceCache, storage.ActionCreate) {
		t.Fatal("create should be supported")
	}
	if storageActionSupported(capabilities, storage.ResourceCache, storage.ActionUpdate) || storageActionSupported(capabilities, storage.ResourceCache, storage.ActionConvert) {
		t.Fatal("expand and convert must fail closed when unsupported")
	}
}
