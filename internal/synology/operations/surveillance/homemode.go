package surveillance

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ychiu1211/dsmctl/internal/domain/surveillance"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

// HomeModeAPIName is the Surveillance Station Home Mode API.
const HomeModeAPIName = "SYNO.SurveillanceStation.HomeMode"

const (
	HomeModeReadCapabilityName = "surveillance.homemode.read"
	HomeModeSetCapabilityName  = "surveillance.homemode.set"
)

// HomeModeMutationResult records the selected backend for a Home Mode switch.
type HomeModeMutationResult struct {
	Backend string `json:"backend" jsonschema:"Selected DSM compatibility backend"`
	API     string `json:"api" jsonschema:"DSM WebAPI used for the change"`
	Version int    `json:"version" jsonschema:"DSM WebAPI version used for the change"`
	Method  string `json:"method" jsonschema:"DSM WebAPI method used for the change"`
}

var homeModeReadOperation = compatibility.Operation[Input, surveillance.HomeMode]{
	Name: HomeModeReadCapabilityName,
	Variants: []compatibility.Variant[Input, surveillance.HomeMode]{
		{
			Name: "surveillance-homemode-v1", API: HomeModeAPIName, Version: 1, Priority: 10,
			Match: compatibility.All(compatibility.APIVersion(HomeModeAPIName, 1), baselinePackage),
			Execute: func(ctx context.Context, executor compatibility.Executor, _ Input) (surveillance.HomeMode, error) {
				data, err := executor.Execute(ctx, compatibility.Request{API: HomeModeAPIName, Version: 1, Method: "GetInfo"})
				if err != nil {
					return surveillance.HomeMode{}, fmt.Errorf("call %s.GetInfo: %w", HomeModeAPIName, err)
				}
				return decodeHomeMode(data)
			},
		},
	},
}

var homeModeSetOperation = compatibility.Operation[bool, HomeModeMutationResult]{
	Name: HomeModeSetCapabilityName,
	Variants: []compatibility.Variant[bool, HomeModeMutationResult]{
		{
			Name: "surveillance-homemode-v1", API: HomeModeAPIName, Version: 1, Priority: 10,
			Match: compatibility.All(compatibility.APIVersion(HomeModeAPIName, 1), baselinePackage),
			Execute: func(ctx context.Context, executor compatibility.Executor, on bool) (HomeModeMutationResult, error) {
				if _, err := executor.Execute(ctx, compatibility.Request{
					API: HomeModeAPIName, Version: 1, Method: "Switch",
					JSONParameters: map[string]any{"on": on},
				}); err != nil {
					return HomeModeMutationResult{}, fmt.Errorf("call %s.Switch: %w", HomeModeAPIName, err)
				}
				return HomeModeMutationResult{}, nil
			},
		},
	},
}

func SelectHomeModeRead(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := homeModeReadOperation.Select(target)
	return selection, err
}

func SelectHomeModeSet(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := homeModeSetOperation.Select(target)
	return selection, err
}

func ExecuteHomeModeRead(ctx context.Context, target compatibility.Target, executor compatibility.Executor) (surveillance.HomeMode, compatibility.Selection, error) {
	return homeModeReadOperation.Run(ctx, target, executor, Input{})
}

func ExecuteHomeModeSet(ctx context.Context, target compatibility.Target, executor compatibility.Executor, on bool) (HomeModeMutationResult, compatibility.Selection, error) {
	result, selection, err := homeModeSetOperation.Run(ctx, target, executor, on)
	if err == nil {
		result.Backend, result.API, result.Version, result.Method = selection.Backend, selection.API, selection.Version, "Switch"
	}
	return result, selection, err
}

func decodeHomeMode(data json.RawMessage) (surveillance.HomeMode, error) {
	raw, err := decodeObject(data)
	if err != nil {
		return surveillance.HomeMode{}, err
	}
	return surveillance.HomeMode{On: firstBool(raw, "on")}, nil
}
