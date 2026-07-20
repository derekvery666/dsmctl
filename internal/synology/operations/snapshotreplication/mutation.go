package snapshotreplication

import (
	"context"
	"fmt"

	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

const (
	SnapshotCreateCapabilityName        = "snapshot.create"
	SnapshotSetAttributesCapabilityName = "snapshot.attributes.set"
	SnapshotDeleteCapabilityName        = "snapshot.delete"
	ShareConfigSetCapabilityName        = "snapshot.shareconfig.set"
)

// MutationResult records the selected backend for one snapshot mutation. For a
// create it also carries the snapshot time name DSM returned.
type MutationResult struct {
	Backend  string `json:"backend" jsonschema:"Selected DSM compatibility backend"`
	API      string `json:"api" jsonschema:"DSM WebAPI used for the change"`
	Version  int    `json:"version" jsonschema:"DSM WebAPI version used for the change"`
	Method   string `json:"method" jsonschema:"DSM WebAPI method used for the change"`
	Snapshot string `json:"snapshot,omitempty" jsonschema:"Created snapshot time name (create only)"`
}

// CreateInput takes one snapshot of a share. A nil Lock uses the DSM default
// (manual snapshots are locked).
type CreateInput struct {
	Share       string
	Description *string
	Lock        *bool
}

// SetInput patches one snapshot's attributes. Nil fields are preserved.
type SetInput struct {
	Share       string
	Snapshot    string
	Description *string
	Lock        *bool
}

// DeleteInput deletes the named snapshots of one share.
type DeleteInput struct {
	Share     string
	Snapshots []string
}

// ShareConfigSetInput patches the per-share snapshot configuration. Nil fields
// are preserved.
type ShareConfigSetInput struct {
	Share            string
	SnapshotBrowsing *bool
	LocalTimeFormat  *bool
}

// snapinfo builds the snapshot-attribute envelope shared by create and set.
// Only provided fields are sent so DSM applies its defaults (create) or keeps
// current values (set).
func snapinfo(description *string, lock *bool) map[string]any {
	info := map[string]any{}
	if description != nil {
		info[keySnapshotDescription] = *description
	}
	if lock != nil {
		info[keySnapshotLock] = *lock
	}
	return info
}

var snapshotCreateOperation = compatibility.Operation[CreateInput, MutationResult]{
	Name: SnapshotCreateCapabilityName,
	Variants: []compatibility.Variant[CreateInput, MutationResult]{
		{
			Name: "core-share-snapshot-create-v1", API: ShareSnapshotAPIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(ShareSnapshotAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input CreateInput) (MutationResult, error) {
				parameters := map[string]any{"name": input.Share}
				if info := snapinfo(input.Description, input.Lock); len(info) != 0 {
					parameters["snapinfo"] = info
				}
				data, err := executor.Execute(ctx, compatibility.Request{
					API: ShareSnapshotAPIName, Version: 1, Method: "create", JSONParameters: parameters,
				})
				if err != nil {
					return MutationResult{}, fmt.Errorf("call %s.create: %w", ShareSnapshotAPIName, err)
				}
				created, err := decodeCreatedSnapshot(data)
				if err != nil {
					return MutationResult{}, err
				}
				return MutationResult{Snapshot: created}, nil
			},
		},
	},
}

var snapshotSetOperation = compatibility.Operation[SetInput, MutationResult]{
	Name: SnapshotSetAttributesCapabilityName,
	Variants: []compatibility.Variant[SetInput, MutationResult]{
		{
			Name: "core-share-snapshot-set-v1", API: ShareSnapshotAPIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(ShareSnapshotAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input SetInput) (MutationResult, error) {
				if _, err := executor.Execute(ctx, compatibility.Request{
					API: ShareSnapshotAPIName, Version: 1, Method: "set",
					JSONParameters: map[string]any{
						"name":     input.Share,
						"snapshot": input.Snapshot,
						"snapinfo": snapinfo(input.Description, input.Lock),
					},
				}); err != nil {
					return MutationResult{}, fmt.Errorf("call %s.set: %w", ShareSnapshotAPIName, err)
				}
				return MutationResult{}, nil
			},
		},
	},
}

var snapshotDeleteOperation = compatibility.Operation[DeleteInput, MutationResult]{
	Name: SnapshotDeleteCapabilityName,
	Variants: []compatibility.Variant[DeleteInput, MutationResult]{
		{
			Name: "core-share-snapshot-delete-v1", API: ShareSnapshotAPIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(ShareSnapshotAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input DeleteInput) (MutationResult, error) {
				if _, err := executor.Execute(ctx, compatibility.Request{
					API: ShareSnapshotAPIName, Version: 1, Method: "delete",
					JSONParameters: map[string]any{"name": input.Share, "snapshots": input.Snapshots},
				}); err != nil {
					return MutationResult{}, fmt.Errorf("call %s.delete: %w", ShareSnapshotAPIName, err)
				}
				return MutationResult{}, nil
			},
		},
	},
}

var shareConfigSetOperation = compatibility.Operation[ShareConfigSetInput, MutationResult]{
	Name: ShareConfigSetCapabilityName,
	Variants: []compatibility.Variant[ShareConfigSetInput, MutationResult]{
		{
			Name: "core-share-snapshot-conf-v1", API: ShareSnapshotAPIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(ShareSnapshotAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, input ShareConfigSetInput) (MutationResult, error) {
				info := map[string]any{}
				if input.SnapshotBrowsing != nil {
					info[keyConfSnapshotBrowsing] = *input.SnapshotBrowsing
				}
				if input.LocalTimeFormat != nil {
					info[keyConfLocalTimeFormat] = *input.LocalTimeFormat
				}
				if _, err := executor.Execute(ctx, compatibility.Request{
					API: ShareSnapshotAPIName, Version: 1, Method: "set_share_conf",
					JSONParameters: map[string]any{"name": input.Share, "sharesnapinfo": info},
				}); err != nil {
					return MutationResult{}, fmt.Errorf("call %s.set_share_conf: %w", ShareSnapshotAPIName, err)
				}
				return MutationResult{}, nil
			},
		},
	},
}

func SelectSnapshotCreate(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := snapshotCreateOperation.Select(target)
	return selection, err
}

func SelectSnapshotSet(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := snapshotSetOperation.Select(target)
	return selection, err
}

func SelectSnapshotDelete(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := snapshotDeleteOperation.Select(target)
	return selection, err
}

func SelectShareConfigSet(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := shareConfigSetOperation.Select(target)
	return selection, err
}

func finishMutation(result MutationResult, selection compatibility.Selection, method string, err error) (MutationResult, compatibility.Selection, error) {
	if err == nil {
		result.Backend, result.API, result.Version, result.Method = selection.Backend, selection.API, selection.Version, method
	}
	return result, selection, err
}

func ExecuteSnapshotCreate(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input CreateInput) (MutationResult, compatibility.Selection, error) {
	result, selection, err := snapshotCreateOperation.Run(ctx, target, executor, input)
	return finishMutation(result, selection, "create", err)
}

func ExecuteSnapshotSet(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input SetInput) (MutationResult, compatibility.Selection, error) {
	result, selection, err := snapshotSetOperation.Run(ctx, target, executor, input)
	return finishMutation(result, selection, "set", err)
}

func ExecuteSnapshotDelete(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input DeleteInput) (MutationResult, compatibility.Selection, error) {
	result, selection, err := snapshotDeleteOperation.Run(ctx, target, executor, input)
	return finishMutation(result, selection, "delete", err)
}

func ExecuteShareConfigSet(ctx context.Context, target compatibility.Target, executor compatibility.Executor, input ShareConfigSetInput) (MutationResult, compatibility.Selection, error) {
	result, selection, err := shareConfigSetOperation.Run(ctx, target, executor, input)
	return finishMutation(result, selection, "set_share_conf", err)
}
