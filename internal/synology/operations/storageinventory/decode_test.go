package storageinventory

import (
	"testing"
)

func TestDecodePoolMutationSafetyFields(t *testing.T) {
	// Sanitized subset of SYNO.Storage.CGI.Storage.load_info from DSM 7.3.
	state, err := decode([]byte(`{
		"env":{"bay_number":"6","support":{"sysdef":true}},
		"disks":[
			{"id":"sata1","status":"normal","overview_status":"normal","used_by":"reuse_1","compatibility":"support","action":{"selectable":true}},
			{"id":"sata5","status":"normal","overview_status":"normal","used_by":"","compatibility":"support","action":{"selectable":true}}
		],
		"storagePools":[{"id":"reuse_1","status":"normal","is_writable":true,"is_actioning":false,"limited_disk_number":24,"can_do":{"expand_by_disk":1,"delete":true},"disks":["sata1"]}],
		"volumes":[]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if !state.PoolCreation.SupportsSHR || state.PoolCreation.MaxDisks != 6 {
		t.Fatalf("pool creation constraints = %#v", state.PoolCreation)
	}
	if len(state.Disks) != 2 || !state.Disks[0].InUse || state.Disks[0].UsedBy != "reuse_1" || state.Disks[1].InUse || !state.Disks[1].Selectable || state.Disks[1].Compatibility != "support" {
		t.Fatalf("disks = %#v", state.Disks)
	}
	if len(state.Pools) != 1 || !state.Pools[0].Writable || state.Pools[0].Actioning || !state.Pools[0].CanExpand || !state.Pools[0].CanDelete || state.Pools[0].MaxDiskCount != 24 {
		t.Fatalf("pool = %#v", state.Pools)
	}
}

func TestDecodeVolumeMutationConstraintsAndActionability(t *testing.T) {
	state, err := decode([]byte(`{
		"env":{"max_fs_bytes":"118747255799808"},
		"storagePools":[{
			"id":"reuse_1","pool_path":"reuse_1","space_path":"/dev/vg1",
			"device_type":"raid_5","raidType":"multiple","status":"normal",
			"is_writable":true,"is_actioning":false,"compatibility":true,
			"size":{"total":"42949672960","used":"10737418240"}
		}],
		"volumes":[{
			"id":"volume_1","pool_path":"reuse_1","raidType":"multiple","fs_type":"btrfs",
			"status":"normal","is_writable":true,"is_actioning":false,"max_fs_size":"1099511627776",
			"size":{"total":"10737418240","used":"1073741824","total_device":"10737418240"},
			"can_do":{"delete":true}
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if state.VolumeCreation.MinimumSizeBytes != 10<<30 || state.VolumeCreation.MaxFileSystemBytes != 118747255799808 {
		t.Fatalf("volume creation constraints = %#v", state.VolumeCreation)
	}
	pool := state.Pools[0]
	if pool.Path != "reuse_1" || pool.SpacePath != "/dev/vg1" || pool.RAIDType != "raid_5" || pool.Layout != "multiple" || !pool.Compatible || !pool.CanCreateVolume || pool.AvailableBytes != 30<<30 {
		t.Fatalf("pool = %#v", pool)
	}
	volume := state.Volumes[0]
	if volume.AllocatedBytes != 10<<30 || volume.MaxFileSystemBytes != 1<<40 || !volume.Writable || volume.Actioning || volume.SingleVolume || !volume.CanExpand || !volume.CanDelete {
		t.Fatalf("volume = %#v", volume)
	}
}

func TestDecodeVolumePathUsesExplicitDSMField(t *testing.T) {
	state, err := decode([]byte(`{"volumes":[{"id":"volume_1","vol_path":"/volume1"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Volumes) != 1 || state.Volumes[0].Path != "/volume1" {
		t.Fatalf("volumes = %#v", state.Volumes)
	}
}

func TestDecodeVolumePathFromCanonicalStableID(t *testing.T) {
	state, err := decode([]byte(`{"volumes":[{"id":"volume_12"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := state.Volumes[0].Path; got != "/volume12" {
		t.Fatalf("derived path = %q, want /volume12", got)
	}
}

func TestNormalizedVolumePathRejectsNonCanonicalOrContradictoryValues(t *testing.T) {
	for _, test := range []struct{ id, path string }{
		{id: "volume_01"},
		{id: "volume_1", path: "/volume01"},
		{id: "volume_1", path: "/volume2"},
		{id: "volume_1", path: "/usbshare1"},
	} {
		if got := normalizedVolumePath(test.id, test.path); got != "" {
			t.Errorf("normalizedVolumePath(%q, %q) = %q, want empty", test.id, test.path, got)
		}
	}
}
