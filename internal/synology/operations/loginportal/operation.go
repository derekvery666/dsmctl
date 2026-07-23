// Package loginportal implements the read-only DSM operations for the Control
// Panel > Login Portal surface. Each area is a separate DSM API (a separate
// compatibility boundary) and selects its own backend per operation, so a NAS
// missing one area leaves the others usable and reports it unsupported rather
// than erroring the whole module.
//
// Live-verified on DSM 7.3 (lab). The actual API/field names:
//   - DSM access: SYNO.Core.Web.DSM get. Both v1 and v2 are advertised, but v2's
//     get OMITS enable_https and enable_hsts, so v1 is used deliberately (it
//     carries the complete field set). Fields: {http_port, https_port,
//     enable_https, enable_https_redirect, enable_hsts, enable_spdy (HTTP/2),
//     enable_custom_domain, fqdn, ...}.
//   - Customized external hostname: SYNO.Core.Web.DSM.External get v1 →
//     {hostname}. This is an independently gated enrichment of the DSM-access
//     area, folded in by the facade when the sibling API is present.
//   - Application portals: SYNO.Core.AppPortal list v1 → {portal: [{id,
//     display_name, enable_redirect}]}. On 7.3 (no custom portals configured)
//     only those three fields appear; alias/http_port/https_port are decoded
//     leniently and surface only when a custom portal is configured.
//   - Reverse-proxy rules: SYNO.Core.AppPortal.ReverseProxy list v1 →
//     {entries: []}. The list envelope and rule COUNT are live-verified (the lab
//     has zero rules); the per-rule field mapping is derived from the WI-070 spec
//     and could not be live-verified this pass (empty list, and codesearch was
//     not reachable in this session), so it is decoded leniently — an unknown key
//     yields an empty/zero field, never a wrong value — and re-verifying a
//     populated rule shape is a prerequisite of the Slice-B write follow-on.
package loginportal

import (
	"context"
	"fmt"

	"github.com/derekvery666/dsmctl/internal/domain/loginportal"
	"github.com/derekvery666/dsmctl/internal/synology/compatibility"
)

const (
	WebDSMAPIName         = "SYNO.Core.Web.DSM"
	WebDSMExternalAPIName = "SYNO.Core.Web.DSM.External"
	AppPortalAPIName      = "SYNO.Core.AppPortal"
	ReverseProxyAPIName   = "SYNO.Core.AppPortal.ReverseProxy"

	DSMWebServiceReadCapabilityName     = "login_portal.dsm_web_service.read"
	ExternalDomainReadCapabilityName    = "login_portal.external_domain.read"
	ApplicationPortalReadCapabilityName = "login_portal.application_portal.read"
	ReverseProxyReadCapabilityName      = "login_portal.reverse_proxy.read"
)

// Input is the empty input for the parameterless reads.
type Input struct{}

// dsmWebServiceOperation reads the DSM access settings. v1 is selected
// deliberately: the DSM 7.3 v2 get drops enable_https and enable_hsts, so v1 is
// the only version that carries the complete normalized field set.
var dsmWebServiceOperation = compatibility.Operation[Input, loginportal.DSMWebService]{
	Name: DSMWebServiceReadCapabilityName,
	Variants: []compatibility.Variant[Input, loginportal.DSMWebService]{
		{
			Name: "login-portal-web-dsm-get-v1", API: WebDSMAPIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(WebDSMAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, _ Input) (loginportal.DSMWebService, error) {
				data, err := executor.Execute(ctx, compatibility.Request{API: WebDSMAPIName, Version: 1, Method: "get", ReadOnly: true})
				if err != nil {
					return loginportal.DSMWebService{}, fmt.Errorf("call %s.get: %w", WebDSMAPIName, err)
				}
				return decodeDSMWebService(data)
			},
		},
	},
}

// externalDomainOperation reads the customized external hostname. It is an
// independent boundary: absent on some NAS models, present as a sibling of the
// DSM-access area otherwise.
var externalDomainOperation = compatibility.Operation[Input, loginportal.DSMWebService]{
	Name: ExternalDomainReadCapabilityName,
	Variants: []compatibility.Variant[Input, loginportal.DSMWebService]{
		{
			Name: "login-portal-web-dsm-external-get-v1", API: WebDSMExternalAPIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(WebDSMExternalAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, _ Input) (loginportal.DSMWebService, error) {
				data, err := executor.Execute(ctx, compatibility.Request{API: WebDSMExternalAPIName, Version: 1, Method: "get", ReadOnly: true})
				if err != nil {
					return loginportal.DSMWebService{}, fmt.Errorf("call %s.get: %w", WebDSMExternalAPIName, err)
				}
				return decodeExternalDomain(data)
			},
		},
	},
}

var applicationPortalOperation = compatibility.Operation[Input, loginportal.ApplicationPortals]{
	Name: ApplicationPortalReadCapabilityName,
	Variants: []compatibility.Variant[Input, loginportal.ApplicationPortals]{
		{
			Name: "login-portal-appportal-list-v1", API: AppPortalAPIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(AppPortalAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, _ Input) (loginportal.ApplicationPortals, error) {
				data, err := executor.Execute(ctx, compatibility.Request{API: AppPortalAPIName, Version: 1, Method: "list", ReadOnly: true})
				if err != nil {
					return loginportal.ApplicationPortals{}, fmt.Errorf("call %s.list: %w", AppPortalAPIName, err)
				}
				return decodeApplicationPortals(data)
			},
		},
	},
}

var reverseProxyOperation = compatibility.Operation[Input, loginportal.ReverseProxyRules]{
	Name: ReverseProxyReadCapabilityName,
	Variants: []compatibility.Variant[Input, loginportal.ReverseProxyRules]{
		{
			Name: "login-portal-reverse-proxy-list-v1", API: ReverseProxyAPIName, Version: 1, Priority: 10,
			Match: compatibility.APIVersion(ReverseProxyAPIName, 1),
			Execute: func(ctx context.Context, executor compatibility.Executor, _ Input) (loginportal.ReverseProxyRules, error) {
				data, err := executor.Execute(ctx, compatibility.Request{API: ReverseProxyAPIName, Version: 1, Method: "list", ReadOnly: true})
				if err != nil {
					return loginportal.ReverseProxyRules{}, fmt.Errorf("call %s.list: %w", ReverseProxyAPIName, err)
				}
				return decodeReverseProxyRules(data)
			},
		},
	},
}

// APINames lists every DSM API this module reads so the facade can discover them
// in one call before selecting variants.
func APINames() []string {
	return []string{
		WebDSMAPIName,
		WebDSMExternalAPIName,
		AppPortalAPIName,
		ReverseProxyAPIName,
	}
}

func SelectDSMWebService(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := dsmWebServiceOperation.Select(target)
	return selection, err
}

func SelectExternalDomain(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := externalDomainOperation.Select(target)
	return selection, err
}

func SelectApplicationPortal(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := applicationPortalOperation.Select(target)
	return selection, err
}

func SelectReverseProxy(target compatibility.Target) (compatibility.Selection, error) {
	_, selection, err := reverseProxyOperation.Select(target)
	return selection, err
}

func ExecuteDSMWebService(ctx context.Context, target compatibility.Target, executor compatibility.Executor) (loginportal.DSMWebService, compatibility.Selection, error) {
	return dsmWebServiceOperation.Run(ctx, target, executor, Input{})
}

func ExecuteExternalDomain(ctx context.Context, target compatibility.Target, executor compatibility.Executor) (loginportal.DSMWebService, compatibility.Selection, error) {
	return externalDomainOperation.Run(ctx, target, executor, Input{})
}

func ExecuteApplicationPortals(ctx context.Context, target compatibility.Target, executor compatibility.Executor) (loginportal.ApplicationPortals, compatibility.Selection, error) {
	return applicationPortalOperation.Run(ctx, target, executor, Input{})
}

func ExecuteReverseProxyRules(ctx context.Context, target compatibility.Target, executor compatibility.Executor) (loginportal.ReverseProxyRules, compatibility.Selection, error) {
	return reverseProxyOperation.Run(ctx, target, executor, Input{})
}

// SupportsExternalDomain reports whether the customized-domain sibling API is
// advertised, so the DSM-access facade read can fold in the external hostname
// only when it is present without failing the whole area.
func SupportsExternalDomain(target compatibility.Target) bool {
	return target.SupportsAPI(WebDSMExternalAPIName, 1)
}

// Select returns the selection for every read area so the facade can build a
// capability report in one call. Unsupported areas carry a diagnosable reason
// rather than an error.
func Select(target compatibility.Target) []compatibility.Selection {
	selections := make([]compatibility.Selection, 0, 4)
	for _, sel := range []func(compatibility.Target) (compatibility.Selection, error){
		SelectDSMWebService, SelectExternalDomain, SelectApplicationPortal, SelectReverseProxy,
	} {
		selection, _ := sel(target)
		selections = append(selections, selection)
	}
	return selections
}
