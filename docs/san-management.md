# SAN management

`dsmctl` exposes SAN Manager through typed inventory and guarded plan/apply
operations shared by the CLI and MCP server. Raw DSM calls are not exposed.

```console
dsmctl san capabilities --nas office
dsmctl san inventory --nas office --json
dsmctl san plan --nas office --file lun-create.json --output lun-create.plan.json
dsmctl san apply --file lun-create.plan.json --approve <plan-sha256>
```

MCP clients use `get_san_capabilities`, `get_san_state`, `plan_san_change`, and
`apply_san_plan`. CLI and MCP accept the same `san.ChangeRequest`, emit the same
hash-bound `application.SANPlan`, and return the same verified apply result.

## Intent examples

Create an initially unmapped LUN on a volume selected from storage inventory:

```json
{
  "action": "create",
  "resource": "lun",
  "lun": {
    "name": "example-lun",
    "description": "managed by dsmctl",
    "backing_volume_id": "volume_1",
    "size_bytes": 1073741824,
    "provisioning": "thin"
  }
}
```

Delete uses the stable LUN UUID returned by DSM, never the display name:

```json
{
  "action": "delete",
  "resource": "lun",
  "lun": { "id": "stable-lun-uuid" }
}
```

Target create/update/delete and mapping attach/detach use the same shape. CHAP
passwords are apply-time references such as `env:DSMCTL_CHAP_PASSWORD`; password
material is never stored in a request or plan. Enabling or disabling a target
must be planned separately from name, IQN, or authentication patches.

## Safety contract

Planning normalizes the request, resolves backing-volume IDs to DSM paths, and
binds the current SAN inventory, stable resource IDs, mapping edges, active
session counts, backing-volume status, and free capacity into the approval
hash. Apply re-reads that state and fails before mutation when the plan is
stale.

- LUN create requires a normal writable btrfs or ext4 volume, a whole-GiB size,
  sufficient free space, and an explicit thin/thick policy.
- LUN shrink is forbidden. Expansion and backing-volume moves recheck capacity.
- LUN delete refuses every mapped LUN. Target delete refuses active sessions
  and every mapping.
- Mapping attach/detach is a separate operation and never deletes either
  endpoint. Mapping changes refuse targets with active sessions.
- Create must return a stable DSM ID. Postconditions use that ID and verify the
  requested state; LUN create also verifies that it is unmapped and on the
  planned backing path.
- A failed or uncertain apply re-reads SAN inventory and reports its state
  fingerprint, whether the resource/mapping exists, and whether a fresh plan
  can be retried safely.

Inventory uses one bulk target call and one bulk LUN call. Mappings are derived
from stable target IDs and LUN UUIDs without per-resource N+1 requests. Unknown
DSM values remain `unknown` rather than becoming untyped response fields. An
absent SAN Manager package reports these operations as unsupported without
breaking other modules.

Snapshots, clones, replication, backup, initiator configuration, automatic
cascade deletion, and generic raw SAN API calls are not implemented. Live
testing remains narrower than product capability: only a unique disposable,
unmapped `dsmctl-e2e-lun-*` LUN may be created and deleted under WI-005; live
target, mapping, snapshot, clone, expansion, or existing-LUN mutation is not
authorized.
