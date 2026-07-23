package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/derekvery666/dsmctl/internal/application"
	"github.com/derekvery666/dsmctl/internal/config"
	"github.com/derekvery666/dsmctl/internal/domain/snapshotreplication"
)

func newSnapshotReplicationCommand(opts *options) *cobra.Command {
	command := &cobra.Command{
		Use:     "snapshot",
		Aliases: []string{"snapshot-replication", "snap"},
		Short:   "Inspect and manage shared-folder snapshots and Snapshot Replication",
	}
	command.AddCommand(
		newSnapshotCapabilitiesCommand(opts),
		newSnapshotStateCommand(opts),
		newSnapshotShareCommand(opts),
		newSnapshotReplicationStatusCommand(opts),
		newSnapshotLogCommand(opts),
		newSnapshotPlanCommand(opts),
		newSnapshotApplyCommand(opts),
		newSnapshotRelationCommand(opts),
	)
	return command
}

func newSnapshotRelationCommand(opts *options) *cobra.Command {
	command := &cobra.Command{
		Use:   "relation",
		Short: "Create and tear down Snapshot Replication relations between two NAS profiles",
	}
	command.AddCommand(
		newSnapshotRelationPlanCommand(opts),
		newSnapshotRelationApplyCommand(opts),
		newSnapshotRelationSyncCommand(opts),
		newSnapshotRelationStopCommand(opts),
		newSnapshotRelationDeleteCommand(opts),
	)
	return command
}

func newSnapshotRelationSyncCommand(opts *options) *cobra.Command {
	var planID, description string
	var encrypted bool
	command := &cobra.Command{
		Use:   "sync",
		Short: "Trigger a manual sync of an existing replication relation by plan id",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.SyncSnapshotReplicationRelation(cmd.Context(), opts.nas, planID, encrypted, description)
			if err != nil {
				return err
			}
			return encodeIndentedJSON(cmd.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(&planID, "plan-id", "", "replication plan id to sync")
	command.Flags().StringVar(&description, "description", "", "optional description recorded on the sync")
	command.Flags().BoolVar(&encrypted, "encrypted", true, "send the sync over an encrypted transport")
	_ = command.MarkFlagRequired("plan-id")
	return command
}

func newSnapshotRelationStopCommand(opts *options) *cobra.Command {
	var planID string
	command := &cobra.Command{
		Use:   "stop",
		Short: "Stop (pause) replication for an existing relation by plan id",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.StopSnapshotReplicationRelation(cmd.Context(), opts.nas, planID)
			if err != nil {
				return err
			}
			return encodeIndentedJSON(cmd.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(&planID, "plan-id", "", "replication plan id to stop")
	_ = command.MarkFlagRequired("plan-id")
	return command
}

func newSnapshotRelationPlanCommand(opts *options) *cobra.Command {
	var sourceNAS, destNAS, share, destVolume, outputPath string
	var encrypted bool
	command := &cobra.Command{
		Use:   "plan",
		Short: "Plan a share replication relation from a source NAS to a destination NAS",
		Long: `Plan a shared-folder replication relation, source NAS -> destination NAS.

Both --source and --dest are configured NAS profiles. The destination admin
credential is resolved from its own vault profile ONLY at apply time; it never
enters the plan, its approval hash, logs, or MCP arguments.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(sourceNAS) == "" {
				sourceNAS = opts.nas
			}
			request := snapshotreplication.RelationCreate{SourceShare: share, DestVolume: destVolume}
			if cmd.Flags().Changed("encrypted") {
				request.SendEncrypted = &encrypted
			}
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			plan, err := service.PlanSnapshotReplicationRelation(cmd.Context(), sourceNAS, destNAS, request)
			if err != nil {
				return err
			}
			return encodeJSONOutput(cmd, outputPath, plan)
		},
	}
	command.Flags().StringVar(&sourceNAS, "source", "", "source NAS profile (defaults to --nas)")
	command.Flags().StringVar(&destNAS, "dest", "", "destination NAS profile")
	command.Flags().StringVar(&share, "share", "", "source shared folder to replicate")
	command.Flags().StringVar(&destVolume, "dest-volume", "/volume1", "destination volume path for the replica")
	command.Flags().BoolVar(&encrypted, "encrypted", true, "encrypt replication traffic (defaults to on for HTTPS destinations)")
	command.Flags().StringVarP(&outputPath, "output", "o", "-", "plan JSON file, or - for stdout")
	_ = command.MarkFlagRequired("dest")
	_ = command.MarkFlagRequired("share")
	return command
}

func newSnapshotRelationApplyCommand(opts *options) *cobra.Command {
	var inputPath, approvalHash string
	var promptDestCred bool
	command := &cobra.Command{
		Use:   "apply",
		Short: "Apply a replication relation plan after hash and stale-state validation",
		Long: `Apply a replication relation plan after hash and stale-state validation.

dsmctl mints the DR pairing credential headlessly: it authenticates to the
destination NAS by account (DSM's SYNO.DR.Node.Credential auth:"account" mode),
resolving the destination admin password from its vault profile only at apply
time. The password never enters the plan, its hash, logs, or MCP arguments. No
browser sign-in is required.

If the destination profile has no stored credential, pass
--prompt-dest-credential to enter the destination admin account and password
interactively at the terminal (used once for pairing, never stored). A
destination account that enforces interactive 2FA is not supported for headless
pairing (use a dedicated automation account).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var plan application.SnapshotReplicationRelationPlan
			if err := decodeJSONInput(cmd, inputPath, &plan); err != nil {
				return fmt.Errorf("read replication plan: %w", err)
			}
			var destCredential *application.SnapshotReplicationDestCredential
			if promptDestCred {
				credential, err := promptDestinationCredential(cmd, opts, plan.DestNAS)
				if err != nil {
					return err
				}
				destCredential = credential
			}
			return applyRelation(cmd, opts, plan, approvalHash, destCredential)
		},
	}
	command.Flags().StringVarP(&inputPath, "file", "f", "-", "replication plan JSON file, or - for stdin")
	command.Flags().StringVar(&approvalHash, "approve", "", "exact SHA-256 hash printed by the replication plan")
	command.Flags().BoolVar(&promptDestCred, "prompt-dest-credential", false, "enter the destination admin account and password at the terminal (for a destination with no stored vault credential)")
	_ = command.MarkFlagRequired("approve")
	return command
}

func applyRelation(cmd *cobra.Command, opts *options, plan application.SnapshotReplicationRelationPlan, approvalHash string, destCredential *application.SnapshotReplicationDestCredential) error {
	service, err := loadService(opts)
	if err != nil {
		return err
	}
	defer closeService(service)
	result, err := service.ApplySnapshotReplicationRelationPlan(cmd.Context(), plan, approvalHash, destCredential)
	if err != nil {
		if destCredential == nil && strings.Contains(err.Error(), "no password available") {
			return fmt.Errorf("%w\n\nThe destination profile has no stored credential. Re-run with --prompt-dest-credential to enter it at the terminal, or store it first with `dsmctl auth login`.", err)
		}
		return err
	}
	return encodeIndentedJSON(cmd.OutOrStdout(), result)
}

// promptDestinationCredential reads the destination NAS admin credential from
// the terminal for a one-shot DR pairing (used when the destination profile has
// no stored vault credential). The password is read without echo. It is never
// stored, logged, or placed in the plan — it is handed straight to the apply
// path and discarded.
func promptDestinationCredential(cmd *cobra.Command, opts *options, destNAS string) (*application.SnapshotReplicationDestCredential, error) {
	cfg, err := config.NewStore(opts.configPath).Load()
	if err != nil {
		return nil, err
	}
	name, profile, err := cfg.Resolve(destNAS)
	if err != nil {
		return nil, fmt.Errorf("resolve destination NAS %q: %w", destNAS, err)
	}
	out := cmd.ErrOrStderr()
	in := cmd.InOrStdin()

	fmt.Fprintf(out, "Enter the destination %q admin credential for Snapshot Replication pairing.\n", name)
	fmt.Fprintf(out, "Account [%s]: ", profile.Username)
	account, err := readTerminalLine(in)
	if err != nil {
		return nil, fmt.Errorf("read destination account: %w", err)
	}
	if strings.TrimSpace(account) == "" {
		account = profile.Username
	}
	if strings.TrimSpace(account) == "" {
		return nil, fmt.Errorf("a destination account is required")
	}

	fmt.Fprint(out, "Password: ")
	password, err := readTerminalSecret(in)
	fmt.Fprintln(out)
	if err != nil {
		return nil, fmt.Errorf("read destination password: %w", err)
	}
	if password == "" {
		return nil, fmt.Errorf("a destination password is required")
	}

	fmt.Fprint(out, "OTP code (leave blank if none): ")
	otp, err := readTerminalLine(in)
	if err != nil {
		return nil, fmt.Errorf("read destination OTP: %w", err)
	}

	return &application.SnapshotReplicationDestCredential{
		Account:  strings.TrimSpace(account),
		Password: password,
		OTPCode:  strings.TrimSpace(otp),
	}, nil
}

// readTerminalLine reads one line from in without buffering past the newline, so
// a following secret read from the same descriptor is not consumed early.
func readTerminalLine(in io.Reader) (string, error) {
	var line []byte
	buf := make([]byte, 1)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				break
			}
			if buf[0] != '\r' {
				line = append(line, buf[0])
			}
		}
		if err != nil {
			if len(line) > 0 {
				break
			}
			return "", err
		}
	}
	return string(line), nil
}

// readTerminalSecret reads a secret without echo when in is a terminal, and
// falls back to a plain line read when it is piped (non-interactive).
func readTerminalSecret(in io.Reader) (string, error) {
	if file, ok := in.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		secret, err := term.ReadPassword(int(file.Fd()))
		return string(secret), err
	}
	return readTerminalLine(in)
}

func newSnapshotRelationDeleteCommand(opts *options) *cobra.Command {
	var planID string
	command := &cobra.Command{
		Use:   "delete",
		Short: "Delete a replication relation on a NAS by plan id (does not delete replicated data)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.DeleteSnapshotReplicationRelation(cmd.Context(), opts.nas, planID)
			if err != nil {
				return err
			}
			return encodeIndentedJSON(cmd.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(&planID, "plan-id", "", "replication plan id to delete")
	_ = command.MarkFlagRequired("plan-id")
	return command
}

func newSnapshotCapabilitiesCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "capabilities",
		Short: "Show Snapshot Replication operation support and selected DSM backends",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetSnapshotReplicationCapabilities(cmd.Context(), opts.nas)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			capabilities := result.Capabilities
			pkg := capabilities.Package
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "Package:\t%s %s (%s)\n", pkg.ID, valueOrDash(pkg.Version), packageRunState(pkg.Installed, pkg.Running))
			fmt.Fprintf(writer, "Snapshots read:\t%s\n", yesNo(capabilities.SnapshotsRead))
			fmt.Fprintf(writer, "Share config read:\t%s\n", yesNo(capabilities.ShareConfigRead))
			fmt.Fprintf(writer, "Retention read:\t%s\n", yesNo(capabilities.RetentionRead))
			fmt.Fprintf(writer, "Log read:\t%s\n", yesNo(capabilities.LogRead))
			fmt.Fprintf(writer, "Node read:\t%s\n", yesNo(capabilities.NodeRead))
			fmt.Fprintf(writer, "Replication read:\t%s\n", yesNo(capabilities.ReplicationRead))
			fmt.Fprintf(writer, "Snapshot create:\t%s\n", yesNo(capabilities.SnapshotCreate))
			fmt.Fprintf(writer, "Snapshot attribute set:\t%s\n", yesNo(capabilities.SnapshotSetAttributes))
			fmt.Fprintf(writer, "Snapshot delete:\t%s\n", yesNo(capabilities.SnapshotDelete))
			fmt.Fprintf(writer, "Share config set:\t%s\n", yesNo(capabilities.ShareConfigSet))
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

func newSnapshotStateCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "state",
		Short: "Summarize snapshots across every snapshot-capable shared folder",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetSnapshotReplicationState(cmd.Context(), opts.nas)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "Package:\t%s %s (%s)\n", result.Package.ID, valueOrDash(result.Package.Version), packageRunState(result.Package.Installed, result.Package.Running))
			fmt.Fprintf(writer, "Node:\t%s (%s)\n\n", valueOrDash(result.Node.Hostname), valueOrDash(result.Node.NodeID))
			fmt.Fprintln(writer, "SHARE\tVOLUME\tSNAPSHOTS\tLATEST\tBROWSING\tRETENTION TASK")
			for _, share := range result.Shares {
				fmt.Fprintf(writer, "%s\t%s\t%d\t%s\t%s\t%s\n", share.Share, valueOrDash(share.VolumePath), share.Total, valueOrDash(share.Latest), yesNo(share.SnapshotBrowsing), yesNo(share.RetentionTask))
			}
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON")
	return command
}

func newSnapshotShareCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "share <name>",
		Short: "Show one shared folder's snapshots, configuration, and retention policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetSnapshotReplicationShare(cmd.Context(), opts.nas, args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "Share:\t%s\n", result.Snapshots.Share)
			fmt.Fprintf(writer, "Snapshot browsing:\t%s\n", yesNo(result.Config.SnapshotBrowsing))
			fmt.Fprintf(writer, "Local-time names:\t%s\n", yesNo(result.Config.LocalTimeFormat))
			if result.Retention.TaskID >= 0 {
				fmt.Fprintf(writer, "Retention task:\t%d (policy %d, keep recent %d, retain days %d, GFS %d/%d/%d/%d/%d)\n",
					result.Retention.TaskID, result.Retention.PolicyType, result.Retention.KeepRecent, result.Retention.RetainDays,
					result.Retention.Hourly, result.Retention.Daily, result.Retention.Weekly, result.Retention.Monthly, result.Retention.Yearly)
			} else {
				fmt.Fprintf(writer, "Retention task:\tnone\n")
			}
			fmt.Fprintf(writer, "Snapshots:\t%d\n\n", result.Snapshots.Total)
			fmt.Fprintln(writer, "TIME\tLOCKED\tSCHEDULED\tWORM\tDESCRIPTION")
			for _, snapshot := range result.Snapshots.Snapshots {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", snapshot.Time, yesNo(snapshot.Locked), yesNo(snapshot.ScheduleCreated), yesNo(snapshot.WormLocked), valueOrDash(snapshot.Description))
			}
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON")
	return command
}

func newSnapshotReplicationStatusCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "replication",
		Short: "Show replication plans (requires the SnapshotReplication package)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetSnapshotReplicationReplication(cmd.Context(), opts.nas)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "Package:\t%s %s (%s)\n", result.Package.ID, valueOrDash(result.Package.Version), packageRunState(result.Package.Installed, result.Package.Running))
			if !result.Supported {
				fmt.Fprintf(writer, "Replication:\tnot available: %s\n", result.Reason)
				return writer.Flush()
			}
			fmt.Fprintf(writer, "Plans:\t%d\n\n", result.Plans.Total)
			fmt.Fprintln(writer, "ID\tROLE\tTARGET\tTYPE\tSTATUS\tMAIN SITE\tDR SITE\tCAN FAILOVER")
			for _, plan := range result.Plans.Plans {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					valueOrDash(plan.ID), valueOrDash(plan.Role), valueOrDash(plan.TargetName), valueOrDash(plan.TargetType),
					valueOrDash(plan.Status), valueOrDash(plan.MainSite.Hostname), valueOrDash(plan.DRSite.Hostname), yesNo(plan.Can.CanFailover))
			}
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON")
	return command
}

func newSnapshotLogCommand(opts *options) *cobra.Command {
	var jsonOutput bool
	var offset, limit int
	command := &cobra.Command{
		Use:   "log",
		Short: "Show the Snapshot Replication log feed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.GetSnapshotReplicationLog(cmd.Context(), opts.nas, offset, limit)
			if err != nil {
				return err
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), result)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintf(writer, "NAS:\t%s\n", result.NAS)
			fmt.Fprintf(writer, "Total:\t%d (info %d, warn %d, error %d)\n\n", result.Log.Total, result.Log.InfoCount, result.Log.WarnCount, result.Log.ErrorCount)
			fmt.Fprintln(writer, "TIME\tLEVEL\tUSER\tMESSAGE")
			for _, entry := range result.Log.Entries {
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", valueOrDash(entry.Time), valueOrDash(entry.Level), valueOrDash(entry.User), valueOrDash(entry.Message))
			}
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON")
	command.Flags().IntVar(&offset, "offset", 0, "entries to skip")
	command.Flags().IntVar(&limit, "limit", 50, "maximum entries to return (max 1000)")
	return command
}

func newSnapshotPlanCommand(opts *options) *cobra.Command {
	var inputPath, outputPath string
	command := &cobra.Command{
		Use:   "plan",
		Short: "Validate a snapshot change and emit an approval plan as JSON",
		Long: `Validate a snapshot change and emit an approval plan as JSON.

The change document selects one action:
  {"action":"create","share":"data","description":"before upgrade","lock":true}
  {"action":"set_attributes","share":"data","snapshot":"GMT+08-...","lock":false}
  {"action":"delete","share":"data","snapshots":["GMT+08-..."]}
  {"action":"set_share_config","share":"data","snapshot_browsing":true}`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var request snapshotreplication.Change
			if err := decodeJSONInput(cmd, inputPath, &request); err != nil {
				return fmt.Errorf("read snapshot change: %w", err)
			}
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			plan, err := service.PlanSnapshotReplicationChange(cmd.Context(), opts.nas, request)
			if err != nil {
				return err
			}
			return encodeJSONOutput(cmd, outputPath, plan)
		},
	}
	command.Flags().StringVarP(&inputPath, "file", "f", "-", "snapshot change JSON file, or - for stdin")
	command.Flags().StringVarP(&outputPath, "output", "o", "-", "plan JSON file, or - for stdout")
	return command
}

func newSnapshotApplyCommand(opts *options) *cobra.Command {
	var inputPath, approvalHash string
	command := &cobra.Command{
		Use:   "apply",
		Short: "Apply a snapshot plan after hash and stale-state validation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var plan application.SnapshotReplicationPlan
			if err := decodeJSONInput(cmd, inputPath, &plan); err != nil {
				return fmt.Errorf("read snapshot plan: %w", err)
			}
			service, err := loadService(opts)
			if err != nil {
				return err
			}
			defer closeService(service)
			result, err := service.ApplySnapshotReplicationPlan(cmd.Context(), plan, approvalHash)
			if err != nil {
				return err
			}
			return encodeIndentedJSON(cmd.OutOrStdout(), result)
		},
	}
	command.Flags().StringVarP(&inputPath, "file", "f", "-", "snapshot plan JSON file, or - for stdin")
	command.Flags().StringVar(&approvalHash, "approve", "", "exact SHA-256 hash printed by the snapshot plan")
	_ = command.MarkFlagRequired("approve")
	return command
}
