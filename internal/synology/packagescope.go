package synology

import (
	"context"
	"fmt"

	"github.com/ychiu1211/dsmctl/internal/synology/compatibility"
	pkgops "github.com/ychiu1211/dsmctl/internal/synology/operations/packagecenter"
)

// preparePackageScopedTargetLocked prepares the compatibility target for an
// operation owned by an installed package rather than by DSM itself. On top of
// API discovery and the DSM release bootstrap it always refreshes the
// installed-package catalog through the verified Package Center inventory
// backend, so every package-scoped command re-checks the installed package
// version first and a package updated mid-session cannot keep a stale variant
// selection.
func (c *Client) preparePackageScopedTargetLocked(ctx context.Context, apiNames ...string) error {
	names := append([]string{pkgops.InventoryAPIName}, apiNames...)
	if err := c.prepareCompatibilityTargetLocked(ctx, names...); err != nil {
		return err
	}
	return c.refreshInstalledPackageCatalogLocked(ctx)
}

// refreshInstalledPackageCatalogLocked replaces the target's installed-package
// catalog with a fresh inventory read. Failing to read the inventory is an
// error rather than an empty catalog: package-scoped selection must not
// conclude "not installed" from missing evidence.
func (c *Client) refreshInstalledPackageCatalogLocked(ctx context.Context) error {
	state, _, err := pkgops.ExecuteInventory(ctx, c.target, lockedExecutor{client: c})
	if err != nil {
		if compatibility.IsUnsupported(err) {
			return fmt.Errorf("this NAS does not expose a verified Package Center inventory backend, so installed package versions cannot be checked: %w", err)
		}
		return fmt.Errorf("read installed-package inventory for package-scoped operation selection: %w", err)
	}
	installed := make([]compatibility.InstalledPackage, 0, len(state.Packages))
	for _, pkg := range state.Packages {
		installed = append(installed, compatibility.InstalledPackage{
			ID:      pkg.ID,
			Version: compatibility.ParsePackageVersion(pkg.Version),
			Running: pkg.Running,
		})
	}
	c.target.SetInstalledPackages(installed)
	c.target.AddCapability(pkgops.InventoryCapabilityName)
	return nil
}
