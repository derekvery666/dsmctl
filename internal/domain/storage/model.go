package storage

const (
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"

	ResourcePool   = "pool"
	ResourceVolume = "volume"

	RAIDSHR   = "shr"
	RAIDSHR2  = "shr2"
	RAID0     = "raid0"
	RAID1     = "raid1"
	RAID5     = "raid5"
	RAID6     = "raid6"
	RAID10    = "raid10"
	RAIDJBOD  = "jbod"
	RAIDBasic = "basic"

	FileSystemBtrfs = "btrfs"
	FileSystemExt4  = "ext4"

	CapacityMaximum = "maximum"
	CapacityExact   = "exact"
)

// State is a stable, DSM-version-independent snapshot of the storage resources
// currently visible to the authenticated account.
type State struct {
	Disks          []Disk                    `json:"disks" jsonschema:"Physical disks reported by DSM"`
	Pools          []Pool                    `json:"pools" jsonschema:"Storage pools and RAID groups reported by DSM"`
	Volumes        []Volume                  `json:"volumes" jsonschema:"Volumes reported by DSM"`
	PoolCreation   PoolCreationConstraints   `json:"pool_creation" jsonschema:"Model-level constraints used to calculate applicable pool RAID choices"`
	VolumeCreation VolumeCreationConstraints `json:"volume_creation" jsonschema:"Model-level constraints used to validate volume creation"`
}

type PoolCreationConstraints struct {
	SupportsSHR bool `json:"supports_shr" jsonschema:"Whether the DSM model reports Synology Hybrid RAID support"`
	MaxDisks    int  `json:"max_disks,omitempty" jsonschema:"Maximum internal disk bays reported by DSM when available"`
}

type VolumeCreationConstraints struct {
	SupportedFileSystems []string `json:"supported_file_systems" jsonschema:"Filesystem types explicitly advertised by this DSM model"`
	MinimumSizeBytes     uint64   `json:"minimum_size_bytes" jsonschema:"Minimum volume allocation accepted by Storage Manager"`
	MaxFileSystemBytes   uint64   `json:"max_file_system_bytes,omitempty" jsonschema:"Model-level filesystem size limit reported by DSM when available"`
}

type Disk struct {
	ID            string   `json:"id" jsonschema:"Stable DSM disk identifier"`
	Name          string   `json:"name,omitempty" jsonschema:"Human-readable disk name"`
	Device        string   `json:"device,omitempty" jsonschema:"DSM or operating-system device name"`
	Slot          string   `json:"slot,omitempty" jsonschema:"Physical slot or bay identifier"`
	Vendor        string   `json:"vendor,omitempty" jsonschema:"Disk vendor"`
	Model         string   `json:"model,omitempty" jsonschema:"Disk model"`
	Serial        string   `json:"serial,omitempty" jsonschema:"Disk serial number"`
	Firmware      string   `json:"firmware,omitempty" jsonschema:"Disk firmware version"`
	Type          string   `json:"type,omitempty" jsonschema:"Disk media type such as HDD or SSD"`
	Interface     string   `json:"interface,omitempty" jsonschema:"Disk interface such as SATA or NVMe"`
	Status        string   `json:"status,omitempty" jsonschema:"Normalized DSM disk status code"`
	Health        string   `json:"health,omitempty" jsonschema:"DSM health or overview status"`
	SMARTStatus   string   `json:"smart_status,omitempty" jsonschema:"Latest SMART status reported by DSM"`
	UsedBy        string   `json:"used_by,omitempty" jsonschema:"Stable DSM resource identifier currently using the disk"`
	InUse         bool     `json:"in_use" jsonschema:"Whether DSM reports the disk allocated to an existing resource"`
	Selectable    bool     `json:"selectable" jsonschema:"Whether Storage Manager reports the disk selectable for management actions"`
	Compatibility string   `json:"compatibility,omitempty" jsonschema:"DSM drive compatibility status"`
	SizeBytes     uint64   `json:"size_bytes,omitempty" jsonschema:"Disk capacity in bytes"`
	TemperatureC  *float64 `json:"temperature_c,omitempty" jsonschema:"Disk temperature in Celsius when available"`
}

type Pool struct {
	ID              string   `json:"id" jsonschema:"Stable DSM storage-pool identifier"`
	Name            string   `json:"name,omitempty" jsonschema:"Human-readable storage-pool name"`
	Path            string   `json:"path,omitempty" jsonschema:"Stable pool_path required by DSM volume operations"`
	SpacePath       string   `json:"space_path,omitempty" jsonschema:"DSM space path used when deploying a single-volume pool"`
	RAIDType        string   `json:"raid_type,omitempty" jsonschema:"RAID or Synology Hybrid RAID type"`
	Layout          string   `json:"layout,omitempty" jsonschema:"DSM volume layout: single or multiple"`
	Status          string   `json:"status,omitempty" jsonschema:"Normalized DSM storage-pool status code"`
	Health          string   `json:"health,omitempty" jsonschema:"DSM health or overview status"`
	SizeBytes       uint64   `json:"size_bytes,omitempty" jsonschema:"Total storage-pool capacity in bytes"`
	UsedBytes       uint64   `json:"used_bytes,omitempty" jsonschema:"Used storage-pool capacity in bytes"`
	AvailableBytes  uint64   `json:"available_bytes,omitempty" jsonschema:"Available storage-pool capacity in bytes"`
	DiskIDs         []string `json:"disk_ids" jsonschema:"DSM disk identifiers belonging to the pool"`
	Writable        bool     `json:"writable" jsonschema:"Whether DSM reports the storage pool writable"`
	Actioning       bool     `json:"actioning" jsonschema:"Whether a storage-pool operation is already in progress"`
	Compatible      bool     `json:"compatible" jsonschema:"Whether DSM reports the pool compatible with volume creation"`
	CanCreateVolume bool     `json:"can_create_volume" jsonschema:"Whether current pool state permits a volume creation request"`
	CanExpand       bool     `json:"can_expand" jsonschema:"Whether DSM reports add-disk expansion as available"`
	CanDelete       bool     `json:"can_delete" jsonschema:"Whether DSM reports deletion as available"`
	MaxDiskCount    int      `json:"max_disk_count,omitempty" jsonschema:"DSM-reported disk limit for this storage pool"`
}

type Volume struct {
	ID                 string `json:"id" jsonschema:"Stable DSM volume identifier"`
	Name               string `json:"name,omitempty" jsonschema:"Human-readable volume name"`
	Path               string `json:"path,omitempty" jsonschema:"DSM volume path used by child resources, for example /volume1"`
	PoolID             string `json:"pool_id,omitempty" jsonschema:"Storage pool containing the volume"`
	FileSystem         string `json:"file_system,omitempty" jsonschema:"Volume file-system type"`
	Status             string `json:"status,omitempty" jsonschema:"Normalized DSM volume status code"`
	Health             string `json:"health,omitempty" jsonschema:"DSM health or overview status"`
	SizeBytes          uint64 `json:"size_bytes,omitempty" jsonschema:"Total volume capacity in bytes"`
	AllocatedBytes     uint64 `json:"allocated_bytes,omitempty" jsonschema:"Pool device bytes allocated to this volume"`
	MaxFileSystemBytes uint64 `json:"max_file_system_bytes,omitempty" jsonschema:"Maximum filesystem size DSM reports for this volume"`
	UsedBytes          uint64 `json:"used_bytes,omitempty" jsonschema:"Used volume capacity in bytes"`
	AvailableBytes     uint64 `json:"available_bytes,omitempty" jsonschema:"Available volume capacity in bytes"`
	ReadOnly           bool   `json:"read_only" jsonschema:"Whether DSM reports the volume as read-only"`
	Writable           bool   `json:"writable" jsonschema:"Whether DSM reports the volume writable"`
	Actioning          bool   `json:"actioning" jsonschema:"Whether a volume operation is already in progress"`
	SingleVolume       bool   `json:"single_volume" jsonschema:"Whether the parent pool uses DSM's single-volume layout"`
	CanExpand          bool   `json:"can_expand" jsonschema:"Whether current pool and volume state permit expansion"`
	CanDelete          bool   `json:"can_delete" jsonschema:"Whether DSM reports volume deletion available"`
}

// Capabilities deliberately distinguishes the read-only first milestone from
// future mutation support so an agent cannot infer that discovery implies write
// access.
type Capabilities struct {
	InventoryRead bool `json:"inventory_read" jsonschema:"Disk, pool, and volume inventory can be read"`
	DiskStatus    bool `json:"disk_status" jsonschema:"Disk status and health can be read"`
	PoolStatus    bool `json:"pool_status" jsonschema:"Storage-pool status and health can be read"`
	VolumeStatus  bool `json:"volume_status" jsonschema:"Volume status and health can be read"`
	PoolCreate    bool `json:"pool_create" jsonschema:"Storage pools can be created through guarded plan/apply"`
	PoolUpdate    bool `json:"pool_update" jsonschema:"Storage pools can be expanded by adding disks through guarded plan/apply"`
	PoolDelete    bool `json:"pool_delete" jsonschema:"Storage pools can be deleted through guarded plan/apply"`
	VolumeCreate  bool `json:"volume_create" jsonschema:"Volumes can be created through guarded plan/apply"`
	VolumeUpdate  bool `json:"volume_update" jsonschema:"Volumes can be expanded through guarded plan/apply"`
	VolumeDelete  bool `json:"volume_delete" jsonschema:"Volumes can be deleted through guarded plan/apply"`
	Mutations     bool `json:"mutations" jsonschema:"Any storage mutation is currently exposed"`
}

// ChangeRequest is the stable storage intent shared by CLI and MCP. The
// action determines which fields are owned: create owns the complete initial
// topology, while update is patch-only and can only add disks, select a target
// RAID type, or expand a volume. Delete names an existing resource by stable
// DSM ID.
type ChangeRequest struct {
	Action   string        `json:"action" jsonschema:"Storage action: create, update, or delete"`
	Resource string        `json:"resource" jsonschema:"Storage resource: pool or volume"`
	Pool     *PoolChange   `json:"pool,omitempty" jsonschema:"Storage-pool intent when resource is pool"`
	Volume   *VolumeChange `json:"volume,omitempty" jsonschema:"Volume intent when resource is volume"`
}

// PoolChange uses DiskIDs only for create and AddDiskIDs only for update.
// TargetRAIDType is an explicit migration target; omitting it preserves the
// current RAID type. Existing disks can never be removed by an update intent.
type PoolChange struct {
	ID             string   `json:"id,omitempty" jsonschema:"Stable DSM storage-pool identifier for update or delete"`
	Name           string   `json:"name,omitempty" jsonschema:"New storage-pool name for create"`
	RAIDType       string   `json:"raid_type,omitempty" jsonschema:"Initial RAID type for create: shr, shr2, raid0, raid1, raid5, raid6, raid10, jbod, or basic"`
	DiskIDs        []string `json:"disk_ids,omitempty" jsonschema:"Complete initial set of stable DSM disk identifiers for create"`
	AddDiskIDs     []string `json:"add_disk_ids,omitempty" jsonschema:"Stable DSM disk identifiers to add during patch-only update"`
	TargetRAIDType *string  `json:"target_raid_type,omitempty" jsonschema:"Optional explicit RAID migration target during update; omitted preserves the current type"`
}

// VolumeChange owns PoolID, FileSystem, and Capacity only during create.
// ExpandTo is patch-only and must never request a smaller exact size.
type VolumeChange struct {
	ID         string          `json:"id,omitempty" jsonschema:"Stable DSM volume identifier for update or delete"`
	Name       string          `json:"name,omitempty" jsonschema:"New volume name for create"`
	PoolID     string          `json:"pool_id,omitempty" jsonschema:"Stable DSM parent storage-pool identifier for create"`
	FileSystem string          `json:"file_system,omitempty" jsonschema:"Initial filesystem for create, such as btrfs or ext4"`
	Capacity   *CapacityPolicy `json:"capacity,omitempty" jsonschema:"Explicit initial capacity policy for create"`
	ExpandTo   *CapacityPolicy `json:"expand_to,omitempty" jsonschema:"Patch-only target capacity for update"`
}

// CapacityPolicy is explicit: maximum consumes the backend-supported maximum
// and requires size_bytes=0; exact requires a positive byte count.
type CapacityPolicy struct {
	Mode      string `json:"mode" jsonschema:"Capacity policy: maximum or exact"`
	SizeBytes uint64 `json:"size_bytes,omitempty" jsonschema:"Requested bytes when mode is exact; zero when mode is maximum"`
}
