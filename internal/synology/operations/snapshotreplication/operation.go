// Package snapshotreplication implements the Snapshot Replication operations:
// btrfs shared-folder snapshot lifecycle (SYNO.Core.Share.Snapshot), per-share
// snapshot configuration, retention policy (SYNO.DisasterRecovery.Retention),
// the Snapshot Replication log feed (SYNO.DisasterRecovery.Log), the local
// replication node identity (SYNO.DR.Node), and the replication plan list
// (SYNO.DR.Plan).
//
// The snapshot, configuration, retention, log, and node APIs are DSM core:
// live-verified on DSM 7.3-81168 without the SnapshotReplication package
// installed, so those operations gate only on advertised API versions. The
// SYNO.DR.Plan family exists only with the package, so the replication read is
// additionally package-gated and fails closed without it. Field names stay
// behind this package; decoders are tolerant but reject malformed shapes.
package snapshotreplication

import (
	"context"
	"fmt"

	"github.com/ychiu1211/dsmctl/internal/domain/snapshotreplication"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

// PackageID is the DSM package that owns the replication surface. The snapshot
// surface answers without it.
const PackageID = "SnapshotReplication"

const (
	ShareSnapshotAPIName = "SYNO.Core.Share.Snapshot"
	RetentionAPIName     = "SYNO.DisasterRecovery.Retention"
	LogAPIName           = "SYNO.DisasterRecovery.Log"
	NodeAPIName          = "SYNO.DR.Node"
	PlanAPIName          = "SYNO.DR.Plan"

	SnapshotsReadCapabilityName   = "snapshot.snapshots.read"
	ShareConfigReadCapabilityName = "snapshot.shareconfig.read"
	RetentionReadCapabilityName   = "snapshot.retention.read"
	LogReadCapabilityName         = "snapshot.log.read"
	NodeReadCapabilityName        = "snapshot.node.read"
	ReplicationReadCapabilityName = "snapshot.replication.read"
)

// replicationPackage gates the replication surface on an installed
// SnapshotReplication 7.x+; the 7.x line covers every DSM 7 release. A future
// major with a verified difference adds a higher-priority variant.
var replicationPackage = compatibility.PackageVersionRange(
	PackageID, compatibility.ParsePackageVersion("7.0"), compatibility.PackageVersion{},
)

// ShareInput selects one shared folder.
type ShareInput struct {
	Share string
}

// LogInput pages the log feed.
type LogInput struct {
	Offset int
	Limit  int
}

type Input struct{}

var snapshotListOperation = compatibility.Operation[ShareInput, snapshotreplication.ShareSnapshots]{
	Name: SnapshotsReadCapabilityName,
	Variants: []compatibility.Variant[ShareInput, snapshotreplication.ShareSnapshots]{
		{
			Name: "core-share-snapshot-list-v2", API: ShareSnapshotAPIName, Version: 2, Priority: 10,
			Match: compatibility.APIVersion(ShareSnapshotAPIName, 2),
			Execute: func(ctx context.Context, executor compatibility.Executor, input ShareInput) (snapshotreplication.ShareSnapshots, error) {
				data, err := executor.Execute(ctx, compatibility.Request{
					API: ShareSnapshotAPIName, Version: 2, Method: "list",
					JSONParameters: map[string]any{
						"name":       input.Share,
						"offset":     0,
						"limit":      -1,
						"additional": []string{keySnapshotDescription, keySnapshotLock, keySnapshotSchedule, keySnapshotWormLock},
					},
					ReadOnly: true,
				})
				if err != nil {
					return snapshotreplication.ShareSnapshots{}, fmt.Errorf("call %s.list: %w", ShareSnapshotAPIName, err)
				}
				return decodeShareSnapshots(input.Share, data)
			},
		},
	},
}

var shareConfigOperation = compatibility.Operation[ShareInput, snapshotreplication.ShareConfig]{
	Name: ShareConfigReadCapabilityName,
	Variants: []compatibility.Variant[ShareInput, snapshotreplication.ShareConfig]{
		{
			Name: "core-share-snapshot-conf-v1", API: ShareSnapshotAPIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(ShareSnapshotAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input ShareInput) (snapshotreplication.ShareConfig, error) {
				data, err := executor.Execute(ctx, compatibility.Request{
					API: ShareSnapshotAPIName, Version: 1, Method: "get_share_conf",
					JSONParameters: map[string]any{
						"name":          input.Share,
						"sharesnapinfo": []string{keyConfLocalTimeFormat, keyConfSnapshotBrowsing},
					},
					ReadOnly: true,
				})
				if err != nil {
					return snapshotreplication.ShareConfig{}, fmt.Errorf("call %s.get_share_conf: %w", ShareSnapshotAPIName, err)
				}
				return decodeShareConfig(input.Share, data)
			},
		},
	},
}

var retentionOperation = compatibility.Operation[ShareInput, snapshotreplication.RetentionPolicy]{
	Name: RetentionReadCapabilityName,
	Variants: []compatibility.Variant[ShareInput, snapshotreplication.RetentionPolicy]{
		{
			Name: "disasterrecovery-retention-get-v1", API: RetentionAPIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(RetentionAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input ShareInput) (snapshotreplication.RetentionPolicy, error) {
				data, err := executor.Execute(ctx, compatibility.Request{
					API: RetentionAPIName, Version: 1, Method: "get",
					JSONParameters: map[string]any{"type": "share", "name": input.Share},
					ReadOnly:       true,
				})
				if err != nil {
					return snapshotreplication.RetentionPolicy{}, fmt.Errorf("call %s.get: %w", RetentionAPIName, err)
				}
				return decodeRetentionPolicy(input.Share, data)
			},
		},
	},
}

var logOperation = compatibility.Operation[LogInput, snapshotreplication.LogPage]{
	Name: LogReadCapabilityName,
	Variants: []compatibility.Variant[LogInput, snapshotreplication.LogPage]{
		{
			Name: "disasterrecovery-log-list-v1", API: LogAPIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(LogAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input LogInput) (snapshotreplication.LogPage, error) {
				data, err := executor.Execute(ctx, compatibility.Request{
					API: LogAPIName, Version: 1, Method: "list",
					JSONParameters: map[string]any{"offset": input.Offset, "limit": input.Limit},
					ReadOnly:       true,
				})
				if err != nil {
					return snapshotreplication.LogPage{}, fmt.Errorf("call %s.list: %w", LogAPIName, err)
				}
				return decodeLogPage(data)
			},
		},
	},
}

var nodeOperation = compatibility.Operation[Input, snapshotreplication.NodeIdentity]{
	Name: NodeReadCapabilityName,
	Variants: []compatibility.Variant[Input, snapshotreplication.NodeIdentity]{
		{
			Name: "dr-node-info-v1", API: NodeAPIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(NodeAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, _ Input) (snapshotreplication.NodeIdentity, error) {
				data, err := executor.Execute(ctx, compatibility.Request{
					API: NodeAPIName, Version: 1, Method: "info", ReadOnly: true,
				})
				if err != nil {
					return snapshotreplication.NodeIdentity{}, fmt.Errorf("call %s.info: %w", NodeAPIName, err)
				}
				return decodeNodeIdentity(data)
			},
		},
	},
}

// planAdditional mirrors the additional blocks the Snapshot Replication UI
// requests from SYNO.DR.Plan list (source-derived; wire-unverified because the
// verified lab cannot install the package).
var planAdditional = []string{
	"sync_policy", "sync_report", "main_site_info", "dr_site_info", "can_do",
	"op_info", "last_op_info", "topology", "testfailover_info", "retention_lock_report",
}

var planListOperation = compatibility.Operation[Input, snapshotreplication.ReplicationPlans]{
	Name: ReplicationReadCapabilityName,
	Variants: []compatibility.Variant[Input, snapshotreplication.ReplicationPlans]{
		{
			Name: "dr-plan-list-v1", API: PlanAPIName, Version: 1, Priority: 10,
			Match: compatibility.All(compatibility.APIVersion(PlanAPIName, 1), replicationPackage),
			Execute: func(ctx context.Context, executor compatibility.Executor, _ Input) (snapshotreplication.ReplicationPlans, error) {
				data, err := executor.Execute(ctx, compatibility.Request{
					API: PlanAPIName, Version: 1, Method: "list",
					JSONParameters: map[string]any{"additional": planAdditional},
					ReadOnly:       true,
				})
				if err != nil {
					return snapshotreplication.ReplicationPlans{}, fmt.Errorf("call %s.list: %w", PlanAPIName, err)
				}
				return decodeReplicationPlans(data)
			},
		},
	},
}

func APINames() []string {
	return []string{ShareSnapshotAPIName, RetentionAPIName, LogAPIName, NodeAPIName, PlanAPIName, NodeCredentialAPIName}
}

func SelectSnapshots(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := snapshotListOperation.Select(target)
	return selection, err
}

func SelectShareConfig(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := shareConfigOperation.Select(target)
	return selection, err
}

func SelectRetention(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := retentionOperation.Select(target)
	return selection, err
}

func SelectLog(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := logOperation.Select(target)
	return selection, err
}

func SelectNode(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := nodeOperation.Select(target)
	return selection, err
}

func SelectPlans(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := planListOperation.Select(target)
	return selection, err
}

func ExecuteSnapshots(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input ShareInput) (snapshotreplication.ShareSnapshots, compatibility.Selection, error) {
	return snapshotListOperation.Run(ctx, target, executor, input)
}

func ExecuteShareConfig(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input ShareInput) (snapshotreplication.ShareConfig, compatibility.Selection, error) {
	return shareConfigOperation.Run(ctx, target, executor, input)
}

func ExecuteRetention(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input ShareInput) (snapshotreplication.RetentionPolicy, compatibility.Selection, error) {
	return retentionOperation.Run(ctx, target, executor, input)
}

func ExecuteLog(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input LogInput) (snapshotreplication.LogPage, compatibility.Selection, error) {
	return logOperation.Run(ctx, target, executor, input)
}

func ExecuteNode(ctx context.Context, target compatibility.Target, executor compatibility.Executor) (snapshotreplication.NodeIdentity, compatibility.Selection, error) {
	return nodeOperation.Run(ctx, target, executor, Input{})
}

func ExecutePlans(ctx context.Context, target compatibility.Target, executor compatibility.Executor) (snapshotreplication.ReplicationPlans, compatibility.Selection, error) {
	return planListOperation.Run(ctx, target, executor, Input{})
}
