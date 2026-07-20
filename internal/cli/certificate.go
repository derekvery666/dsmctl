package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/ychiu1211/dsmctl/internal/application"
	"github.com/ychiu1211/dsmctl/internal/domain/certificate"
)

func newCertificateCommand(opts *options) *cobra.Command {
	command := &cobra.Command{
		Use:     "certificate",
		Aliases: []string{"cert"},
		Short:   "Inspect and manage DSM certificates (Control Panel > Security > Certificate)",
	}
	command.AddCommand(
		newCertificateCapabilitiesCommand(opts),
		newCertificateListCommand(opts),
		newCertificatePlanCommand(opts),
		newCertificateApplyCommand(opts),
		newCertificateExportCommand(opts),
	)
	return command
}

func newCertificatePlanCommand(opts *options) *cobra.Command {
	var inputPath, outputPath string
	command := &cobra.Command{
		Use:   "plan",
		Short: "Validate a certificate import/set-default/bind/delete and emit an approval plan as JSON",
		Long: "Validate a high-risk certificate change (import, set_default, bind_service, delete), read " +
			"the current certificate store, and return a hash-bound approval plan. The private key is supplied " +
			"by a credential reference (env:NAME) and is resolved only at apply time; it never enters the plan. " +
			"This command never mutates DSM.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var request certificate.ChangeRequest
			if err := decodeJSONInput(cmd, inputPath, &request); err != nil {
				return fmt.Errorf("read certificate change: %w", err)
			}
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			plan, err := service.PlanCertificateChange(cmd.Context(), opts.nas, request)
			if err != nil {
				return err
			}
			return encodeJSONOutput(cmd, outputPath, plan)
		},
	}
	command.Flags().StringVarP(&inputPath, "file", "f", "-", "certificate change JSON file, or - for stdin")
	command.Flags().StringVarP(&outputPath, "output", "o", "-", "plan JSON file, or - for stdout")
	return command
}

func newCertificateApplyCommand(opts *options) *cobra.Command {
	var inputPath, approveHash string
	command := &cobra.Command{
		Use:   "apply",
		Short: "Apply a certificate plan after validating its approval hash and precondition",
		Long: "Apply an unmodified certificate plan only when its approval hash and observed-state precondition " +
			"still match, then verify DSM. Replacing or deleting the certificate that serves the current dsmctl " +
			"session requires acknowledge_current_session in the plan; dsmctl re-pins to the new leaf for the " +
			"post-apply re-read.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var plan application.CertificatePlan
			if err := decodeJSONInput(cmd, inputPath, &plan); err != nil {
				return fmt.Errorf("read certificate plan: %w", err)
			}
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.ApplyCertificatePlan(cmd.Context(), plan, approveHash)
			if err != nil {
				return err
			}
			return encodeIndentedJSON(cmd.OutOrStdout(), result)
		},
	}
	command.Flags().StringVarP(&inputPath, "file", "f", "-", "certificate plan JSON file, or - for stdin")
	command.Flags().StringVar(&approveHash, "approve", "", "exact SHA-256 hash printed by certificate plan")
	_ = command.MarkFlagRequired("approve")
	return command
}

func newCertificateExportCommand(opts *options) *cobra.Command {
	var certID, outputPath string
	command := &cobra.Command{
		Use:   "export",
		Short: "Export a certificate archive to a local file (WARNING: extracts private-key material)",
		Long: "Download the archive DSM produces for a certificate to a local file. The archive CONTAINS the " +
			"private key, so this command writes secret material to disk (mode 0600) and returns no key bytes on " +
			"stdout. It does not change the NAS. Treat the output file as a secret.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.ExportCertificate(cmd.Context(), opts.nas, certID, outputPath)
			if err != nil {
				return err
			}
			return encodeIndentedJSON(cmd.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(&certID, "id", "", "certificate id to export")
	command.Flags().StringVarP(&outputPath, "output", "o", "", "local file path for the archive (contains the private key)")
	_ = command.MarkFlagRequired("id")
	_ = command.MarkFlagRequired("output")
	return command
}

func newCertificateCapabilitiesCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "capabilities",
		Short: "Show certificate operation support and the selected backend",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetCertificateCapabilities(cmd.Context(), opts.nas)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "certificates read\t%s\n", yesNo(result.Capabilities.CertificatesRead))
			fmt.Fprintf(writer, "import (high risk)\t%s\n", yesNo(result.Capabilities.Import))
			fmt.Fprintf(writer, "set default (high risk)\t%s\n", yesNo(result.Capabilities.SetDefault))
			fmt.Fprintf(writer, "bind service (high risk)\t%s\n", yesNo(result.Capabilities.BindService))
			fmt.Fprintf(writer, "delete (high risk)\t%s\n", yesNo(result.Capabilities.Delete))
			fmt.Fprintf(writer, "export (extracts key)\t%s\n", yesNo(result.Capabilities.Export))
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

func newCertificateListCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "list",
		Short: "List installed certificates, their expiry, and the services they serve",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetCertificates(cmd.Context(), opts.nas)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "Total:\t%d\n", result.Certificates.Total)
			if len(result.Certificates.Certificates) == 0 {
				fmt.Fprintln(writer, "No certificates installed.")
				return writer.Flush()
			}
			fmt.Fprintln(writer, "\nSUBJECT\tDEFAULT\tEXPIRES\tRENEWABLE\tBROKEN\tSERVICES\tID")
			for _, cert := range result.Certificates.Certificates {
				subject := cert.Subject.CommonName
				if subject == "" {
					subject = valueOrDash(cert.Description)
				}
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					subject, yesNo(cert.IsDefault), certExpiry(cert.ValidTill, cert.ValidTillUnix),
					yesNo(cert.Renewable), yesNo(cert.IsBroken), certServiceList(cert), cert.ID)
			}
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON")
	return command
}

// certExpiry renders the not-after with a computed days-to-expiry hint.
func certExpiry(raw string, unix int64) string {
	if unix <= 0 {
		return valueOrDash(raw)
	}
	days := int(time.Until(time.Unix(unix, 0)).Hours() / 24)
	when := time.Unix(unix, 0).Local().Format("2006-01-02")
	switch {
	case days < 0:
		return fmt.Sprintf("%s (expired)", when)
	case days == 0:
		return fmt.Sprintf("%s (today)", when)
	default:
		return fmt.Sprintf("%s (%dd)", when, days)
	}
}

func certServiceList(cert certificate.Certificate) string {
	if len(cert.Services) == 0 {
		return "-"
	}
	names := make([]string, 0, len(cert.Services))
	for _, svc := range cert.Services {
		name := svc.DisplayName
		if name == "" {
			name = svc.Service
		}
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}
