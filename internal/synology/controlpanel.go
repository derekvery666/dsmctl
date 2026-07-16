package synology

import (
	"context"
	"fmt"

	"github.com/ychiu1211/dsmctl/internal/domain/controlpanel"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
	"github.com/ychiu1211/dsmctl/internal/synology/operations/controlpaneltime"
)

type ControlPanelTimeState = controlpanel.TimeState
type ControlPanelTimeCapabilities = controlpanel.TimeCapabilities

// ControlPanelTimeState reads the focused time module without requiring or
// coupling to any other Control Panel module API.
func (c *Client) ControlPanelTimeState(ctx context.Context) (ControlPanelTimeState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.prepareCompatibilityTargetLocked(ctx, controlpaneltime.APINames()...); err != nil {
		return ControlPanelTimeState{}, fmt.Errorf("prepare Control Panel time target: %w", err)
	}
	state, _, err := controlpaneltime.Execute(ctx, c.target, lockedExecutor{client: c})
	if err != nil {
		return ControlPanelTimeState{}, fmt.Errorf("get Control Panel time configuration: %w", err)
	}
	c.target.AddCapability(controlpaneltime.CapabilityName)
	return state, nil
}

// ControlPanelTimeCapabilities reports only this module's selection. The
// module can therefore be unsupported without changing another module's
// capability result.
func (c *Client) ControlPanelTimeCapabilities(ctx context.Context) (ControlPanelTimeCapabilities, CompatibilityReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.prepareCompatibilityTargetLocked(ctx, controlpaneltime.APINames()...); err != nil {
		return ControlPanelTimeCapabilities{}, CompatibilityReport{}, fmt.Errorf("prepare Control Panel time capabilities target: %w", err)
	}
	selection, err := controlpaneltime.Select(c.target)
	if err != nil && !compatibility.IsUnsupported(err) {
		return ControlPanelTimeCapabilities{}, CompatibilityReport{}, fmt.Errorf("select Control Panel time backend: %w", err)
	}
	if selection.Supported {
		c.target.AddCapability(controlpaneltime.CapabilityName)
	}
	capabilities := ControlPanelTimeCapabilities{
		Module: controlpanel.ModuleTime,
		Read:   selection.Supported,
		Set:    false,
	}
	return capabilities, c.target.Report(selection), nil
}
