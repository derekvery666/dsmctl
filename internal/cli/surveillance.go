package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ychiu1211/dsmctl/internal/application"
	"github.com/ychiu1211/dsmctl/internal/domain/surveillance"
)

func newSurveillanceCommand(opts *options) *cobra.Command {
	command := &cobra.Command{
		Use:     "surveillance",
		Aliases: []string{"svs"},
		Short:   "Inspect the Synology Surveillance Station package",
	}
	command.AddCommand(
		newSurveillanceCapabilitiesCommand(opts),
		newSurveillanceInfoCommand(opts),
		newSurveillanceCamerasCommand(opts),
		newSurveillanceHomeModeCommand(opts),
	)
	return command
}

func newSurveillanceHomeModeCommand(opts *options) *cobra.Command {
	command := &cobra.Command{
		Use:   "homemode",
		Short: "Inspect and switch Surveillance Station Home Mode",
	}
	command.AddCommand(
		newSurveillanceHomeModeStateCommand(opts),
		newSurveillanceHomeModePlanCommand(opts),
		newSurveillanceHomeModeApplyCommand(opts),
	)
	return command
}

func newSurveillanceHomeModeStateCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "state",
		Short: "Show whether Home Mode is on",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts.configPath)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetSurveillanceHomeMode(cmd.Context(), opts.nas)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "Home Mode:\t%s\n", yesNo(result.HomeMode.On))
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON")
	return command
}

func newSurveillanceHomeModePlanCommand(opts *options) *cobra.Command {
	var inputPath, outputPath string
	command := &cobra.Command{
		Use:   "plan",
		Short: "Validate a Home Mode patch and emit an approval plan",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var request surveillance.HomeModeChange
			if err := decodeJSONInput(cmd, inputPath, &request); err != nil {
				return fmt.Errorf("read home mode change: %w", err)
			}
			service, err := loadService(opts.configPath)
			if err != nil {
				return err
			}
			defer closeService(service)
			plan, err := service.PlanSurveillanceHomeModeChange(cmd.Context(), opts.nas, request)
			if err != nil {
				return err
			}
			return encodeJSONOutput(cmd, outputPath, plan)
		},
	}
	command.Flags().StringVarP(&inputPath, "file", "f", "-", "home mode change JSON file, or - for stdin")
	command.Flags().StringVarP(&outputPath, "output", "o", "-", "plan JSON file, or - for stdout")
	return command
}

func newSurveillanceHomeModeApplyCommand(opts *options) *cobra.Command {
	var inputPath, approvalHash string
	command := &cobra.Command{
		Use:   "apply",
		Short: "Apply a Home Mode plan after hash and stale-state validation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var plan application.SurveillanceHomeModePlan
			if err := decodeJSONInput(cmd, inputPath, &plan); err != nil {
				return fmt.Errorf("read home mode plan: %w", err)
			}
			service, err := loadService(opts.configPath)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.ApplySurveillanceHomeModePlan(cmd.Context(), plan, approvalHash)
			if err != nil {
				return err
			}
			return encodeIndentedJSON(cmd.OutOrStdout(), result)
		},
	}
	command.Flags().StringVarP(&inputPath, "file", "f", "-", "home mode plan JSON file, or - for stdin")
	command.Flags().StringVar(&approvalHash, "approve", "", "exact SHA-256 hash printed by the home mode plan")
	_ = command.MarkFlagRequired("approve")
	return command
}

func newSurveillanceCapabilitiesCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "capabilities",
		Short: "Show Surveillance Station support and the installed package",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts.configPath)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetSurveillanceCapabilities(cmd.Context(), opts.nas)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			pkg := result.Capabilities.Package
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "Package:\t%s %s (%s)\n", pkg.ID, valueOrDash(pkg.Version), packageRunState(pkg.Installed, pkg.Running))
			fmt.Fprintf(writer, "Info read:\t%s\n", yesNo(result.Capabilities.InfoRead))
			fmt.Fprintf(writer, "Camera read:\t%s\n", yesNo(result.Capabilities.CameraRead))
			fmt.Fprintf(writer, "Home Mode read:\t%s\n", yesNo(result.Capabilities.HomeModeRead))
			fmt.Fprintf(writer, "Home Mode set:\t%s\n", yesNo(result.Capabilities.HomeModeSet))
			fmt.Fprintln(writer, "\nOPERATION\tSUPPORTED\tBACKEND\tAPI\tVERSION")
			for _, operation := range result.Report.Operations {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\tv%d\n", operation.Operation, yesNo(operation.Supported), valueOrDash(operation.Backend), valueOrDash(operation.API), operation.Version)
			}
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON")
	return command
}

func newSurveillanceInfoCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "info",
		Short: "Show Surveillance Station system information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts.configPath)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetSurveillanceInfo(cmd.Context(), opts.nas)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			info := result.Info
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "Version:\t%s\n", valueOrDash(info.Version))
			fmt.Fprintf(writer, "Hostname:\t%s\n", valueOrDash(info.Hostname))
			fmt.Fprintf(writer, "Cameras:\t%d\n", info.CameraNumber)
			fmt.Fprintf(writer, "Max cameras:\t%d\n", info.MaxCameraSupport)
			fmt.Fprintf(writer, "Licenses:\t%d\n", info.LicenseNumber)
			fmt.Fprintf(writer, "Timezone:\t%s\n", valueOrDash(info.Timezone))
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON")
	return command
}

func newSurveillanceCamerasCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "cameras",
		Short: "List configured cameras",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts.configPath)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetSurveillanceCameras(cmd.Context(), opts.nas)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "Total cameras:\t%d\n\n", result.Cameras.Total)
			fmt.Fprintln(writer, "ID\tNAME\tIP\tVENDOR\tMODEL\tENABLED")
			for _, cam := range result.Cameras.Cameras {
				fmt.Fprintf(writer, "%d\t%s\t%s\t%s\t%s\t%s\n", cam.ID, valueOrDash(cam.Name), valueOrDash(cam.IP), valueOrDash(cam.Vendor), valueOrDash(cam.Model), yesNo(cam.Enabled))
			}
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON")
	return command
}
