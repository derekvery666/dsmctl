// Package san contains stable, DSM-version-independent models for block
// storage exported through Synology SAN Manager.
package san

const (
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionAttach = "attach"
	ActionDetach = "detach"

	ResourceTarget  = "target"
	ResourceLUN     = "lun"
	ResourceMapping = "mapping"

	ProtocolISCSI = "iscsi"

	AuthenticationNone       = "none"
	AuthenticationCHAP       = "chap"
	AuthenticationMutualCHAP = "mutual_chap"

	ProvisioningThin    = "thin"
	ProvisioningThick   = "thick"
	ProvisioningUnknown = "unknown"

	BackingVolume      = "volume"
	BackingStoragePool = "storage_pool"
	BackingUnknown     = "unknown"
)

// State is a point-in-time SAN inventory. Mappings use the stable DSM target
// ID and LUN UUID rather than display names.
type State struct {
	Targets  []Target  `json:"targets" jsonschema:"iSCSI targets reported by SAN Manager"`
	LUNs     []LUN     `json:"luns" jsonschema:"SAN LUNs reported by SAN Manager"`
	Mappings []Mapping `json:"mappings" jsonschema:"Target-to-LUN mappings derived from the bulk target inventory"`
}

type Target struct {
	ID                string `json:"id" jsonschema:"Stable DSM target identifier"`
	Name              string `json:"name" jsonschema:"Human-readable target name"`
	Description       string `json:"description,omitempty" jsonschema:"Target description"`
	Protocol          string `json:"protocol" jsonschema:"Target protocol; currently iscsi"`
	IQN               string `json:"iqn,omitempty" jsonschema:"iSCSI qualified name"`
	Enabled           bool   `json:"enabled" jsonschema:"Whether DSM reports the target enabled"`
	Status            string `json:"status,omitempty" jsonschema:"Normalized target state reported by DSM"`
	Health            string `json:"health,omitempty" jsonschema:"Normalized health classification derived from DSM state"`
	Authentication    string `json:"authentication,omitempty" jsonschema:"Authentication mode: none, chap, mutual_chap, or unknown"`
	ConnectedSessions int    `json:"connected_sessions" jsonschema:"Number of connected initiator sessions"`
}

type LUN struct {
	ID              string `json:"id" jsonschema:"Stable LUN UUID"`
	NumericID       string `json:"numeric_id,omitempty" jsonschema:"DSM numeric LUN identifier when reported"`
	Name            string `json:"name" jsonschema:"Human-readable LUN name"`
	Description     string `json:"description,omitempty" jsonschema:"LUN description"`
	Protocol        string `json:"protocol" jsonschema:"Protocol family; currently iscsi"`
	Status          string `json:"status,omitempty" jsonschema:"LUN state reported by DSM"`
	Health          string `json:"health,omitempty" jsonschema:"Normalized health classification derived from DSM state"`
	SizeBytes       uint64 `json:"size_bytes" jsonschema:"Configured LUN capacity in bytes"`
	AllocatedBytes  uint64 `json:"allocated_bytes,omitempty" jsonschema:"Physical allocation reported by DSM"`
	BlockSizeBytes  uint64 `json:"block_size_bytes,omitempty" jsonschema:"Logical block size in bytes"`
	Provisioning    string `json:"provisioning" jsonschema:"Provisioning mode: thin, thick, or unknown"`
	BackingKind     string `json:"backing_kind" jsonschema:"Backing resource kind: volume, storage_pool, or unknown"`
	BackingLocation string `json:"backing_location,omitempty" jsonschema:"DSM volume or storage-pool location"`
	Mapped          bool   `json:"mapped" jsonschema:"Whether at least one target maps this LUN"`
}

type Mapping struct {
	TargetID string `json:"target_id" jsonschema:"Stable DSM target identifier"`
	LUNID    string `json:"lun_id" jsonschema:"Stable LUN UUID"`
}

// Capabilities keeps the read-only milestone explicit. Discovery of SAN
// Manager APIs never implies authorization for a mutation.
type Capabilities struct {
	InventoryRead bool `json:"inventory_read" jsonschema:"Complete target, LUN, and mapping inventory can be read"`
	TargetRead    bool `json:"target_read" jsonschema:"iSCSI targets can be read"`
	LUNRead       bool `json:"lun_read" jsonschema:"SAN LUNs can be read"`
	MappingRead   bool `json:"mapping_read" jsonschema:"Target-to-LUN mappings can be read"`
	TargetCreate  bool `json:"target_create" jsonschema:"Targets can be created through guarded plan/apply"`
	TargetUpdate  bool `json:"target_update" jsonschema:"Targets can be updated through guarded plan/apply"`
	TargetDelete  bool `json:"target_delete" jsonschema:"Unmapped targets without active sessions can be deleted through guarded plan/apply"`
	LUNCreate     bool `json:"lun_create" jsonschema:"LUNs can be created through guarded plan/apply"`
	LUNUpdate     bool `json:"lun_update" jsonschema:"LUNs can be updated through guarded plan/apply"`
	LUNDelete     bool `json:"lun_delete" jsonschema:"Unmapped LUNs can be deleted through guarded plan/apply"`
	MappingAttach bool `json:"mapping_attach" jsonschema:"Existing targets and LUNs can be attached through guarded plan/apply"`
	MappingDetach bool `json:"mapping_detach" jsonschema:"Target-to-LUN mappings can be detached through guarded plan/apply"`
	Mutations     bool `json:"mutations" jsonschema:"Any SAN mutation is currently exposed"`
}

// ChangeRequest is the stable SAN intent shared by CLI and MCP. Create owns
// complete initial settings; update is patch-only; delete, attach, and detach
// identify existing resources exclusively by stable DSM IDs.
type ChangeRequest struct {
	Action   string         `json:"action" jsonschema:"SAN action: create, update, delete, attach, or detach"`
	Resource string         `json:"resource" jsonschema:"SAN resource: target, lun, or mapping"`
	Target   *TargetChange  `json:"target,omitempty" jsonschema:"Target intent when resource is target"`
	LUN      *LUNChange     `json:"lun,omitempty" jsonschema:"LUN intent when resource is lun"`
	Mapping  *MappingChange `json:"mapping,omitempty" jsonschema:"Mapping intent when resource is mapping"`
}

type TargetChange struct {
	ID                    string  `json:"id,omitempty" jsonschema:"Stable DSM target ID for update or delete"`
	Name                  string  `json:"name,omitempty" jsonschema:"Target name for create"`
	IQN                   string  `json:"iqn,omitempty" jsonschema:"iSCSI qualified name for create"`
	Authentication        string  `json:"authentication,omitempty" jsonschema:"Authentication mode for create: none, chap, or mutual_chap"`
	CHAPUser              string  `json:"chap_user,omitempty" jsonschema:"CHAP username; never a password"`
	CHAPPasswordRef       string  `json:"chap_password_ref,omitempty" jsonschema:"Apply-time CHAP password reference using env:NAME"`
	MutualCHAPUser        string  `json:"mutual_chap_user,omitempty" jsonschema:"Mutual CHAP username; never a password"`
	MutualCHAPPasswordRef string  `json:"mutual_chap_password_ref,omitempty" jsonschema:"Apply-time mutual CHAP password reference using env:NAME"`
	NewName               *string `json:"new_name,omitempty" jsonschema:"Patch-only target name replacement"`
	NewIQN                *string `json:"new_iqn,omitempty" jsonschema:"Patch-only target IQN replacement"`
	NewAuthentication     *string `json:"new_authentication,omitempty" jsonschema:"Patch-only authentication replacement"`
	Enabled               *bool   `json:"enabled,omitempty" jsonschema:"Patch-only enable or disable request; cannot be combined with other target patches"`
}

type LUNChange struct {
	ID                 string  `json:"id,omitempty" jsonschema:"Stable LUN UUID for update or delete"`
	Name               string  `json:"name,omitempty" jsonschema:"LUN name for create"`
	Description        string  `json:"description,omitempty" jsonschema:"Initial LUN description"`
	BackingVolumeID    string  `json:"backing_volume_id,omitempty" jsonschema:"Stable DSM volume ID for create"`
	SizeBytes          uint64  `json:"size_bytes,omitempty" jsonschema:"Initial configured capacity in bytes"`
	Provisioning       string  `json:"provisioning,omitempty" jsonschema:"Initial provisioning policy: thin or thick"`
	NewName            *string `json:"new_name,omitempty" jsonschema:"Patch-only LUN name replacement"`
	NewDescription     *string `json:"new_description,omitempty" jsonschema:"Patch-only LUN description replacement; empty clears it"`
	NewBackingVolumeID *string `json:"new_backing_volume_id,omitempty" jsonschema:"Patch-only stable destination volume ID"`
	NewSizeBytes       *uint64 `json:"new_size_bytes,omitempty" jsonschema:"Patch-only expanded capacity; shrinking is forbidden"`
}

type MappingChange struct {
	TargetID string `json:"target_id" jsonschema:"Stable DSM target ID"`
	LUNID    string `json:"lun_id" jsonschema:"Stable LUN UUID"`
}
