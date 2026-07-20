package synology

import (
	"context"
	"fmt"

	"github.com/ychiu1211/dsmctl/internal/domain/loginportal"
	lpops "github.com/ychiu1211/dsmctl/internal/synology/operations/loginportal"
)

type DSMWebService = loginportal.DSMWebService
type ApplicationPortals = loginportal.ApplicationPortals
type ReverseProxyRules = loginportal.ReverseProxyRules
type LoginPortalCapabilities = loginportal.Capabilities

// DSMWebService reads the Control Panel > Login Portal > DSM tab settings (DSM
// ports, HTTPS, HTTP->HTTPS redirect, HSTS, HTTP/2, customized domain). Login
// Portal is DSM core, so the plain compatibility target is used. The customized
// external hostname is an independent sibling API: it is folded in only when
// present, and its absence never fails the DSM-access read.
func (c *Client) DSMWebService(ctx context.Context) (DSMWebService, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareCompatibilityTargetLocked(ctx, lpops.APINames()...); err != nil {
		return DSMWebService{}, fmt.Errorf("prepare login portal target: %w", err)
	}
	settings, _, err := lpops.ExecuteDSMWebService(ctx, c.target, lockedExecutor{client: c})
	if err != nil {
		return DSMWebService{}, fmt.Errorf("get DSM web service settings: %w", err)
	}
	c.target.AddCapability(lpops.DSMWebServiceReadCapabilityName)
	if lpops.SupportsExternalDomain(c.target) {
		external, _, err := lpops.ExecuteExternalDomain(ctx, c.target, lockedExecutor{client: c})
		if err != nil {
			return DSMWebService{}, fmt.Errorf("get DSM external domain settings: %w", err)
		}
		settings.ExternalDomainSupported = external.ExternalDomainSupported
		settings.ExternalHostname = external.ExternalHostname
		c.target.AddCapability(lpops.ExternalDomainReadCapabilityName)
	}
	return settings, nil
}

// ApplicationPortals reads the Login Portal > Applications tab: the per-app
// portal list.
func (c *Client) ApplicationPortals(ctx context.Context) (ApplicationPortals, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareCompatibilityTargetLocked(ctx, lpops.APINames()...); err != nil {
		return ApplicationPortals{}, fmt.Errorf("prepare login portal target: %w", err)
	}
	portals, _, err := lpops.ExecuteApplicationPortals(ctx, c.target, lockedExecutor{client: c})
	if err != nil {
		return ApplicationPortals{}, fmt.Errorf("get application portals: %w", err)
	}
	c.target.AddCapability(lpops.ApplicationPortalReadCapabilityName)
	return portals, nil
}

// ReverseProxyRules reads the Login Portal > Advanced tab: the reverse-proxy
// rule list. The list envelope and rule count are live-verified; per-rule fields
// are decoded leniently and never surface certificate key material or header
// values.
func (c *Client) ReverseProxyRules(ctx context.Context) (ReverseProxyRules, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareCompatibilityTargetLocked(ctx, lpops.APINames()...); err != nil {
		return ReverseProxyRules{}, fmt.Errorf("prepare login portal target: %w", err)
	}
	rules, _, err := lpops.ExecuteReverseProxyRules(ctx, c.target, lockedExecutor{client: c})
	if err != nil {
		return ReverseProxyRules{}, fmt.Errorf("get reverse proxy rules: %w", err)
	}
	c.target.AddCapability(lpops.ReverseProxyReadCapabilityName)
	return rules, nil
}

// LoginPortalCapabilities reports which Login Portal reads dsmctl exposes for the
// selected NAS, plus the discovered backends. Each area is an independent
// boundary: one being absent leaves the others usable.
func (c *Client) LoginPortalCapabilities(ctx context.Context) (LoginPortalCapabilities, CompatibilityReport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareCompatibilityTargetLocked(ctx, lpops.APINames()...); err != nil {
		return LoginPortalCapabilities{}, CompatibilityReport{}, fmt.Errorf("prepare login portal capabilities target: %w", err)
	}

	dsmWeb, err := selectSupported(lpops.SelectDSMWebService, c.target)
	if err != nil {
		return LoginPortalCapabilities{}, CompatibilityReport{}, fmt.Errorf("select DSM web service backend: %w", err)
	}
	externalDomain, err := selectSupported(lpops.SelectExternalDomain, c.target)
	if err != nil {
		return LoginPortalCapabilities{}, CompatibilityReport{}, fmt.Errorf("select external domain backend: %w", err)
	}
	appPortal, err := selectSupported(lpops.SelectApplicationPortal, c.target)
	if err != nil {
		return LoginPortalCapabilities{}, CompatibilityReport{}, fmt.Errorf("select application portal backend: %w", err)
	}
	reverseProxy, err := selectSupported(lpops.SelectReverseProxy, c.target)
	if err != nil {
		return LoginPortalCapabilities{}, CompatibilityReport{}, fmt.Errorf("select reverse proxy backend: %w", err)
	}

	if dsmWeb.Supported {
		c.target.AddCapability(lpops.DSMWebServiceReadCapabilityName)
	}
	if externalDomain.Supported {
		c.target.AddCapability(lpops.ExternalDomainReadCapabilityName)
	}
	if appPortal.Supported {
		c.target.AddCapability(lpops.ApplicationPortalReadCapabilityName)
	}
	if reverseProxy.Supported {
		c.target.AddCapability(lpops.ReverseProxyReadCapabilityName)
	}

	capabilities := LoginPortalCapabilities{
		Module:                loginportal.ModuleName,
		DSMWebServiceRead:     dsmWeb.Supported,
		ExternalDomainRead:    externalDomain.Supported,
		ApplicationPortalRead: appPortal.Supported,
		ReverseProxyRead:      reverseProxy.Supported,
		Mutations:             false,
	}
	return capabilities, c.target.Report(dsmWeb, externalDomain, appPortal, reverseProxy), nil
}
