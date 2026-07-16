// Package controlpaneltime implements the independently selectable DSM
// operation for reading Control Panel time and NTP configuration.
package controlpaneltime

import (
	"context"
	"fmt"

	"github.com/ychiu1211/dsmctl/internal/domain/controlpanel"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
)

const (
	APIName        = "SYNO.Core.Region.NTP"
	CapabilityName = "controlpanel.time.read"
	OperationName  = "controlpanel.time.read"
)

type Input struct{}

var operation = compatibility.Operation[Input, controlpanel.TimeState]{
	Name: OperationName,
	Variants: []compatibility.Variant[Input, controlpanel.TimeState]{
		coreVariant("core-region-ntp-v3", 3, 30, true),
		coreVariant("core-region-ntp-v2", 2, 20, true),
		coreVariant("core-region-ntp-v1-legacy", 1, 10, false),
	},
}

func APINames() []string {
	return operation.APINames()
}

func Select(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := operation.Select(target)
	return selection, err
}

func Execute(ctx context.Context, target compatibility.Target, executor compatibility.Executor) (controlpanel.TimeState, compatibility.Selection, error) {
	return operation.Run(ctx, target, executor, Input{})
}

func coreVariant(name string, version, priority int, requireFormats bool) compatibility.Variant[Input, controlpanel.TimeState] {
	return compatibility.Variant[Input, controlpanel.TimeState]{
		Name:     name,
		API:      APIName,
		Version:  version,
		Priority: priority,
		Match:    compatibility.APIVersion(APIName, version),
		Execute: func(ctx context.Context, executor compatibility.Executor, _ Input) (controlpanel.TimeState, error) {
			data, err := executor.Execute(ctx, compatibility.Request{
				API:     APIName,
				Version: version,
				Method:  "get",
			})
			if err != nil {
				return controlpanel.TimeState{}, fmt.Errorf("call %s.get v%d: %w", APIName, version, err)
			}
			return decode(data, requireFormats)
		},
	}
}
