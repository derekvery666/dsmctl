// Package saninventory implements operation-scoped compatibility selection
// for the bulk SAN Manager inventory APIs.
package saninventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/derekvery666/dsmctl/internal/domain/san"
	"github.com/derekvery666/dsmctl/internal/synology/compatibility"
)

const (
	TargetAPIName = "SYNO.Core.ISCSI.Target"
	LUNAPIName    = "SYNO.Core.ISCSI.LUN"

	CapabilityName        = "san.inventory"
	TargetCapabilityName  = "san.targets.read"
	LUNCapabilityName     = "san.luns.read"
	MappingCapabilityName = "san.mappings.read"

	TargetOperationName = "san.targets.list"
	LUNOperationName    = "san.luns.list"
)

var targetAdditional = []string{"mapped_lun", "acls", "connected_sessions", "status"}

var lunTypes = []string{
	"BLOCK", "FILE", "THIN", "ADV", "SINK", "CINDER", "CINDER_BLUN",
	"CINDER_BLUN_THICK", "BLUN", "BLUN_THICK", "BLUN_SINK", "BLUN_THICK_SINK",
}

var lunAdditional = []string{
	"is_action_locked", "is_mapped", "extent_size", "allocated_size", "status",
	"allow_bkpobj", "flashcache_status", "family_config", "sync_progress",
	"snapshot_info", "acls",
}

type Input struct{}

type targetInventory struct {
	Targets  []san.Target
	Mappings []san.Mapping
}

var targetOperation = compatibility.Operation[Input, targetInventory]{
	Name: TargetOperationName,
	Variants: []compatibility.Variant[Input, targetInventory]{
		{
			Name:     "core-iscsi-target-v1",
			API:      TargetAPIName,
			Version:  1,
			Priority: 10,
			Match:    compatibility.APIVersion(TargetAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, _ Input) (targetInventory, error) {
				data, err := executor.Execute(ctx, compatibility.Request{
					API:     TargetAPIName,
					Version: 1,
					Method:  "list",
					JSONParameters: map[string]any{
						"additional": targetAdditional,
					},
				})
				if err != nil {
					return targetInventory{}, fmt.Errorf("call %s.list v1: %w", TargetAPIName, err)
				}
				return decodeTargets(data)
			},
		},
	},
}

var lunOperation = compatibility.Operation[Input, []san.LUN]{
	Name: LUNOperationName,
	Variants: []compatibility.Variant[Input, []san.LUN]{
		{
			Name:     "core-iscsi-lun-v1",
			API:      LUNAPIName,
			Version:  1,
			Priority: 10,
			Match:    compatibility.APIVersion(LUNAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, _ Input) ([]san.LUN, error) {
				data, err := executor.Execute(ctx, compatibility.Request{
					API:     LUNAPIName,
					Version: 1,
					Method:  "list",
					JSONParameters: map[string]any{
						"types":      lunTypes,
						"additional": lunAdditional,
					},
				})
				if err != nil {
					return nil, fmt.Errorf("call %s.list v1: %w", LUNAPIName, err)
				}
				return decodeLUNs(data)
			},
		},
	},
}

func APINames() []string {
	names := append(targetOperation.APINames(), lunOperation.APINames()...)
	sort.Strings(names)
	return names
}

// Select reports the target and LUN operations independently. Unsupported
// selections are retained so an absent SAN Manager package is visible in a
// capability report without breaking unrelated features.
func Select(target compatibility.Target) ([]compatibility.Selection, error) {
	selections := make([]compatibility.Selection, 0, 2)
	for _, selectOperation := range []func(compatibility.Target) (compatibility.Selection, error){selectTargets, selectLUNs} {
		selection, err := selectOperation(target)
		selections = append(selections, selection)
		if err != nil && !compatibility.IsUnsupported(err) {
			return nil, err
		}
	}
	return selections, nil
}

func Execute(ctx context.Context, target compatibility.Target, executor compatibility.Executor) (san.State, []compatibility.Selection, error) {
	targets, targetSelection, err := targetOperation.Run(ctx, target, executor, Input{})
	if err != nil {
		return san.State{}, []compatibility.Selection{targetSelection}, err
	}
	luns, lunSelection, err := lunOperation.Run(ctx, target, executor, Input{})
	selections := []compatibility.Selection{targetSelection, lunSelection}
	if err != nil {
		return san.State{}, selections, err
	}
	state := san.State{Targets: targets.Targets, LUNs: luns, Mappings: targets.Mappings}
	markMappedLUNs(&state)
	return state, selections, nil
}

func selectTargets(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := targetOperation.Select(target)
	return selection, err
}

func selectLUNs(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := lunOperation.Select(target)
	return selection, err
}

func TargetsSupported(selections []compatibility.Selection) bool {
	return len(selections) > 0 && selections[0].Supported
}

func LUNsSupported(selections []compatibility.Selection) bool {
	return len(selections) > 1 && selections[1].Supported
}

func InventorySupported(selections []compatibility.Selection) bool {
	return TargetsSupported(selections) && LUNsSupported(selections)
}

func markMappedLUNs(state *san.State) {
	mapped := make(map[string]struct{}, len(state.Mappings))
	for _, mapping := range state.Mappings {
		mapped[mapping.LUNID] = struct{}{}
	}
	for index := range state.LUNs {
		_, state.LUNs[index].Mapped = mapped[state.LUNs[index].ID]
	}
}
