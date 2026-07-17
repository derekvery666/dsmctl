package synology

import (
	"context"
	"fmt"

	"github.com/ychiu1211/dsmctl/internal/domain/packagecenter"
	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
	pkgops "github.com/ychiu1211/dsmctl/internal/synology/operations/packagecenter"
)

type PackageState = packagecenter.State
type PackageSettings = packagecenter.Settings
type PackageCapabilities = packagecenter.Capabilities
type PackageChangeRequest = packagecenter.ChangeRequest
type PackageSettingsChange = packagecenter.SettingsChange
type PackageLifecycleChange = packagecenter.LifecycleChange
type PackageMutationResult = pkgops.MutationResult

// PackageState reads the installed-package inventory without requiring any other
// Package Center operation to be supported.
func (c *Client) PackageState(ctx context.Context) (PackageState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.prepareCompatibilityTargetLocked(ctx, pkgops.InventoryAPIName); err != nil {
		return PackageState{}, fmt.Errorf("prepare Package Center inventory target: %w", err)
	}
	state, _, err := pkgops.ExecuteInventory(ctx, c.target, lockedExecutor{client: c})
	if err != nil {
		return PackageState{}, fmt.Errorf("get Package Center inventory: %w", err)
	}
	c.target.AddCapability(pkgops.InventoryCapabilityName)
	return state, nil
}

// PackageSettings reads the global Package Center configuration.
func (c *Client) PackageSettings(ctx context.Context) (PackageSettings, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.prepareCompatibilityTargetLocked(ctx, pkgops.SettingAPIName); err != nil {
		return PackageSettings{}, fmt.Errorf("prepare Package Center settings target: %w", err)
	}
	settings, _, err := pkgops.ExecuteSettingsRead(ctx, c.target, lockedExecutor{client: c})
	if err != nil {
		return PackageSettings{}, fmt.Errorf("get Package Center settings: %w", err)
	}
	c.target.AddCapability(pkgops.SettingsReadCapabilityName)
	return settings, nil
}

// PackageCapabilities reports each Package Center operation's selection. A
// missing API makes only the affected operation unsupported.
func (c *Client) PackageCapabilities(ctx context.Context) (PackageCapabilities, CompatibilityReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.prepareCompatibilityTargetLocked(ctx, pkgops.APINames()...); err != nil {
		return PackageCapabilities{}, CompatibilityReport{}, fmt.Errorf("prepare Package Center capabilities target: %w", err)
	}
	selections, err := pkgops.Select(c.target)
	if err != nil {
		return PackageCapabilities{}, CompatibilityReport{}, fmt.Errorf("select Package Center backends: %w", err)
	}
	c.addPackageCapabilitiesLocked(selections)
	capabilities := packageCapabilitiesFromSelections(selections)
	return capabilities, c.target.Report(selections...), nil
}

// ApplyPackageSettingsChange submits the complete desired settings. The caller
// merges the patch into a freshly read full state so no unspecified field is
// reset.
func (c *Client) ApplyPackageSettingsChange(ctx context.Context, desired PackageSettings) (PackageMutationResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.prepareCompatibilityTargetLocked(ctx, pkgops.SettingAPIName); err != nil {
		return PackageMutationResult{}, fmt.Errorf("prepare Package Center settings mutation target: %w", err)
	}
	result, _, err := pkgops.ExecuteSettingsSet(ctx, c.target, lockedExecutor{client: c}, desired)
	if err != nil {
		return PackageMutationResult{}, fmt.Errorf("apply Package Center settings: %w", err)
	}
	return result, nil
}

// ApplyPackageLifecycleChange starts, stops, or uninstalls one package.
func (c *Client) ApplyPackageLifecycleChange(ctx context.Context, change PackageLifecycleChange) (PackageMutationResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.prepareCompatibilityTargetLocked(ctx, pkgops.APINames()...); err != nil {
		return PackageMutationResult{}, fmt.Errorf("prepare Package Center lifecycle target: %w", err)
	}
	switch change.Action {
	case packagecenter.ActionStart, packagecenter.ActionStop:
		result, _, err := pkgops.ExecuteControl(ctx, c.target, lockedExecutor{client: c}, pkgops.ControlInput{Action: change.Action, PackageID: change.PackageID})
		if err != nil {
			return PackageMutationResult{}, fmt.Errorf("apply Package Center %s: %w", change.Action, err)
		}
		return result, nil
	case packagecenter.ActionUninstall:
		result, _, err := pkgops.ExecuteUninstall(ctx, c.target, lockedExecutor{client: c}, pkgops.UninstallInput{PackageID: change.PackageID})
		if err != nil {
			return PackageMutationResult{}, fmt.Errorf("apply Package Center uninstall: %w", err)
		}
		return result, nil
	default:
		return PackageMutationResult{}, fmt.Errorf("unsupported package lifecycle action %q", change.Action)
	}
}

func packageCapabilitiesFromSelections(selections []compatibility.Selection) PackageCapabilities {
	supported := func(index int) bool { return index < len(selections) && selections[index].Supported }
	return PackageCapabilities{
		Module:        packagecenter.ModuleName,
		InventoryRead: supported(0),
		SettingsRead:  supported(1),
		SettingsSet:   supported(2),
		Start:         supported(3),
		Stop:          supported(3),
		Uninstall:     supported(4),
		Install:       supported(5),
		Update:        supported(6),
	}
}

func (c *Client) addPackageCapabilitiesLocked(selections []compatibility.Selection) {
	names := []string{
		pkgops.InventoryCapabilityName,
		pkgops.SettingsReadCapabilityName,
		pkgops.SettingsSetCapabilityName,
		pkgops.ControlCapabilityName,
		pkgops.UninstallCapabilityName,
		pkgops.InstallCapabilityName,
		pkgops.UpdateCapabilityName,
	}
	for index, name := range names {
		if index < len(selections) && selections[index].Supported {
			c.target.AddCapability(name)
		}
	}
}
