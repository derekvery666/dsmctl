package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ychiu1211/dsmctl/internal/domain/loginportal"
)

func newControlPanelLoginPortalCommand(opts *options) *cobra.Command {
	command := &cobra.Command{
		Use:     "login-portal",
		Aliases: []string{"loginportal"},
		Short:   "Inspect the Login Portal (DSM access, application portals, reverse proxy)",
		Long: "Read the Control Panel > Login Portal surface: the DSM web-service access settings (ports, HTTPS, " +
			"HTTP->HTTPS redirect, HSTS, HTTP/2, customized domain), the per-application portals, and the " +
			"reverse-proxy rules. This slice is read-only; guarded writes are a deferred follow-on.",
	}
	command.AddCommand(
		newLoginPortalCapabilitiesCommand(opts),
		newLoginPortalDSMCommand(opts),
		newLoginPortalApplicationsCommand(opts),
		newLoginPortalReverseProxyCommand(opts),
	)
	return command
}

func newLoginPortalCapabilitiesCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "capabilities",
		Short: "Show Login Portal read support and the selected backends",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetLoginPortalCapabilities(cmd.Context(), opts.nas)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "Module:\t%s\n", result.Capabilities.Module)
			fmt.Fprintf(writer, "dsm web service read\t%s\n", yesNo(result.Capabilities.DSMWebServiceRead))
			fmt.Fprintf(writer, "external domain read\t%s\n", yesNo(result.Capabilities.ExternalDomainRead))
			fmt.Fprintf(writer, "application portal read\t%s\n", yesNo(result.Capabilities.ApplicationPortalRead))
			fmt.Fprintf(writer, "reverse proxy read\t%s\n", yesNo(result.Capabilities.ReverseProxyRead))
			fmt.Fprintln(writer, "\nOPERATION\tSUPPORTED\tBACKEND\tAPI\tVERSION")
			for _, op := range result.Report.Operations {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\tv%d\n", op.Operation, yesNo(op.Supported), valueOrDash(op.Backend), valueOrDash(op.API), op.Version)
			}
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON")
	return command
}

func newLoginPortalDSMCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "dsm",
		Short: "Show DSM web-service access settings (ports, HTTPS, redirect, HSTS, HTTP/2, domain)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetDSMWebService(cmd.Context(), opts.nas)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			s := result.Settings
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "HTTP port:\t%d\n", s.HTTPPort)
			fmt.Fprintf(writer, "HTTPS port:\t%d\n", s.HTTPSPort)
			fmt.Fprintf(writer, "HTTPS enabled:\t%s\n", yesNo(s.HTTPSEnabled))
			fmt.Fprintf(writer, "HTTP->HTTPS redirect:\t%s\n", yesNo(s.HTTPRedirectEnabled))
			fmt.Fprintf(writer, "HSTS enabled:\t%s\n", yesNo(s.HSTSEnabled))
			fmt.Fprintf(writer, "HTTP/2 enabled:\t%s\n", yesNo(s.HTTP2Enabled))
			fmt.Fprintf(writer, "Customized domain enabled:\t%s\n", yesNo(s.CustomDomainEnabled))
			fmt.Fprintf(writer, "Customized domain:\t%s\n", valueOrDash(s.CustomDomain))
			if s.ExternalDomainSupported {
				fmt.Fprintf(writer, "External hostname:\t%s\n", valueOrDash(s.ExternalHostname))
			} else {
				fmt.Fprintf(writer, "External hostname:\t%s\n", "(not supported)")
			}
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON")
	return command
}

func newLoginPortalApplicationsCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:     "applications",
		Aliases: []string{"apps"},
		Short:   "Show the per-application portal list",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetApplicationPortals(cmd.Context(), opts.nas)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "Applications:\t%d\n", result.Portals.Total)
			if len(result.Portals.Portals) > 0 {
				fmt.Fprintln(writer, "\nAPPLICATION\tID\tHTTPS REDIRECT\tALIAS\tHTTP PORT\tHTTPS PORT")
				for _, portal := range result.Portals.Portals {
					fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n",
						valueOrDash(portal.DisplayName), portal.AppID, yesNo(portal.RedirectHTTPS),
						valueOrDash(portal.Alias), portOrDash(portal.HTTPPort), portOrDash(portal.HTTPSPort))
				}
			}
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON")
	return command
}

func newLoginPortalReverseProxyCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:     "reverse-proxy",
		Aliases: []string{"reverseproxy", "rp"},
		Short:   "Show the reverse-proxy rule list",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetReverseProxyRules(cmd.Context(), opts.nas)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "Reverse-proxy rules:\t%d\n", result.Rules.Total)
			if len(result.Rules.Rules) > 0 {
				fmt.Fprintln(writer, "\nDESCRIPTION\tFRONTEND\tBACKEND\tHSTS\tHTTP/2\tCERT\tHEADERS")
				for _, rule := range result.Rules.Rules {
					fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%d\n",
						valueOrDash(rule.Description), formatEndpoint(rule.Frontend), formatEndpoint(rule.Backend),
						yesNo(rule.HSTSEnabled), yesNo(rule.HTTP2Enabled), yesNo(rule.CertificatePresent), rule.CustomHeaderCount)
				}
			}
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON")
	return command
}

func formatEndpoint(endpoint loginportal.ReverseProxyEndpoint) string {
	host := endpoint.Hostname
	if host == "" {
		host = "*"
	}
	scheme := endpoint.Protocol
	if scheme != "" {
		scheme += "://"
	}
	if endpoint.Port > 0 {
		return fmt.Sprintf("%s%s:%d", scheme, host, endpoint.Port)
	}
	return fmt.Sprintf("%s%s", scheme, host)
}

func portOrDash(port int) string {
	if port <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d", port)
}
