# Snapshot Replication

The `snapshot` module manages btrfs shared-folder snapshots and reads the
Snapshot Replication surface of a NAS. The snapshot lifecycle, per-share
snapshot configuration, retention policy, log feed, and replication node
identity are **DSM core APIs** — live-verified on DSM 7.3-81168 with no
package installed — so they work on any DSM 7 NAS with a btrfs volume.
Only the **replication plan** surface requires the installed
`SnapshotReplication` package (dependency `ReplicationService`); without it,
`snapshot.replication.read` reports `(not supported)` and fails closed while
every other operation keeps working.

> DSM 7.3-81168 note: that build's own Package Center feed cannot install
> Snapshot Replication at all (the feed pairs SnapshotReplication 7.4.7-1859,
> which requires ReplicationService ≥ 0501, with ReplicationService 1.3.0-0423,
> while every ReplicationService ≥ 0501 requires DSM ≥ 7.3-81179). The module's
> core-API design keeps the whole snapshot surface usable despite that; the
> replication read stays fail-closed until the NAS runs a DSM where the package
> installs. Per-plan replication fields are source-derived and wire-unverified
> for the same reason.

## Reads

```console
dsmctl snapshot capabilities          # operation support + selected backends
dsmctl snapshot state                 # per-share snapshot overview + node identity
dsmctl snapshot share <name>          # one share: snapshots, config, retention
dsmctl snapshot replication           # replication plans (package-gated)
dsmctl snapshot log [--offset --limit]
```

- **State** summarizes every snapshot-capable shared folder: snapshot count,
  latest snapshot, whether user snapshot browsing (`#snapshot`) is enabled, and
  whether a retention task exists.
- **Share** lists each snapshot with its attributes: time name, description,
  lock (protection from retention pruning), schedule-created flag, and WORM
  lock.
- **Retention** reports the policy numbers DSM keeps per share (task id,
  policy type, keep-recent, retain-days, and the hourly/daily/weekly/monthly/
  yearly GFS rules).
- **Log** returns the Snapshot Replication event feed (time, level, acting
  user, message) with level counts.

All reads are available as MCP tools: `get_snapshot_capabilities`,
`get_snapshot_state`, `get_snapshot_share`, `get_snapshot_replication_status`,
and `get_snapshot_log`.

## Guarded snapshot changes (plan/apply)

Mutations go through the hash-bound plan/apply contract — there is no
convenience path. A plan binds the target share's **complete observed snapshot
inventory and configuration**; a snapshot taken or deleted out-of-band between
plan and apply invalidates the plan. Apply re-reads the share afterward and
verifies the postcondition.

```console
echo '{"action":"create","share":"data","description":"before upgrade","lock":true}' \
  | dsmctl snapshot plan -f -            # → plan JSON with approval hash
dsmctl snapshot apply -f plan.json --approve <hash>
```

| Action | Fields | Risk |
| --- | --- | --- |
| `create` | `share`, optional `description`, optional `lock` (DSM default: locked) | medium |
| `set_attributes` | `share`, `snapshot`, `description` and/or `lock` (patch-only) | medium |
| `delete` | `share`, `snapshots` (explicit list, each must exist) | **high — irreversible** |
| `set_share_config` | `share`, `snapshot_browsing` and/or `local_time_format` (patch-only) | medium |

- `create` returns the new snapshot's time name in the apply result and
  verifies it is listed.
- Deleting a **locked** snapshot is possible but adds an explicit warning to
  the plan; unlocking warns that the snapshot becomes eligible for retention
  pruning; enabling snapshot browsing warns that prior file versions become
  visible to every account with access to the share.
- MCP: `plan_snapshot_change` / `apply_snapshot_plan`. The read-only developer
  gateway strips both.

The full lifecycle (create with description+lock → edit attributes → toggle
browsing on and off → delete) was live-verified on the DSM 7.3-81168 lab
against a throwaway `dsmctl-e2e-snap-*` share, which was removed afterward.

## Deferred

- **Retention/schedule writes** (`SYNO.DisasterRecovery.Retention set` with an
  embedded schedule): the wire shape has many interacting fields and could not
  be end-to-end verified on the lab; the operation fails closed until a
  follow-on verifies it.
- **Replication mutations** (create/edit/delete relations, sync-now, failover,
  switchover, test-failover, re-protect): they need a second prepared NAS as a
  replication target and carry extreme risk.
- **Restore paths** (rollback a share to a snapshot, clone to a new share):
  destructive restore surfaces for a dedicated work item.
- **LUN snapshots** stay with the SAN module.
