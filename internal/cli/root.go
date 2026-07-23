package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/derekvery666/dsmctl/internal/config"
)

type options struct {
	configPath string
	nas        string
	logLevel   string
}

func New(version string) *cobra.Command {
	opts := &options{}
	root := &cobra.Command{
		Use:   "dsmctl",
		Short: "Manage one or more Synology NAS devices",
		Long: `Manage one or more Synology NAS devices through typed, compatibility-aware operations.

Start by adding a named NAS profile and authenticating it. In automation, pass
--nas explicitly; otherwise dsmctl uses the configured default profile. Inspect
capabilities before relying on an operation whose DSM support may vary.

All configuration mutations are guarded: first run the matching plan command,
review the returned target, intent, risk, summary, warnings, precondition, and
approval hash, then pass that exact unmodified plan and hash to apply. Planning
reads DSM state but never mutates it. Apply re-reads state, rejects stale or
modified plans, performs the typed operation, and verifies the postcondition.
Typed request shapes are module-specific; use the project README and module
guides when available, or discover every request shape offline with
'dsmctl schema list' and 'dsmctl schema show <command path...>'. Never infer
JSON fields from a command summary.

Discover the complete CLI surface offline with 'dsmctl commands list --json'.
Use 'dsmctl commands show <command path...> --json' for one command's exact
usage, flags, defaults, required inputs, workflow role, and request-schema link.

Passwords and OTPs do not belong in command JSON or plan files. Use auth login
for DSM authentication and use only documented credential references for
operations that require an apply-time secret.`,
		Example: `  # First-time setup
  dsmctl nas add office --url https://nas.example.com:5001 --username automation --default
  dsmctl auth login --nas office

  # Safe discovery and reads
  dsmctl nas list
  dsmctl auth status --nas office
  dsmctl nas capabilities --nas office
  dsmctl storage inventory --nas office --json

  # Offline request discovery (does not contact a NAS)
  dsmctl commands list --prefix account --runnable-only
  dsmctl commands show account inventory --json
  dsmctl schema list
  dsmctl schema show account plan

  # Guarded mutation pattern (module-specific request JSON is required)
  dsmctl account plan --nas office --file request.json --output plan.json
  dsmctl account apply --file plan.json --approve <hash-from-plan>`,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		// Stamp a per-invocation correlation id so all of a command's DSM calls
		// share one id in the diagnostic log (when logging is enabled).
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetContext(withCorrelationID(cmd.Context()))
			return nil
		},
	}
	root.PersistentFlags().StringVar(&opts.configPath, "config", config.DefaultPath(), "configuration file path")
	root.PersistentFlags().StringVar(&opts.nas, "nas", "", "NAS profile name (defaults to the configured default)")
	root.PersistentFlags().StringVar(&opts.logLevel, "log-level", "", "diagnostic log level: debug, info, warn, or error (default: off; also DSMCTL_LOG_LEVEL)")
	root.AddCommand(
		newAccessCommand(opts),
		newAccountCommand(opts),
		newAccountProtectionCommand(opts),
		newAuthCommand(opts),
		newBackupCommand(opts),
		newControlPanelCommand(opts),
		newCommandCatalogCommand(root),
		newDirectoryCommand(opts),
		newDiscoverCommand(opts),
		newDiskSMARTCommand(opts),
		newDownloadCommand(opts),
		newDSMUpdateCommand(opts),
		newCertificateCommand(opts),
		newDriveCommand(opts),
		newExternalAccessCommand(opts),
		newExternalDeviceCommand(opts),
		newFileCommand(opts),
		newHardwareCommand(opts),
		newInstallCommand(opts),
		newFirewallCommand(opts),
		newKMIPCommand(opts),
		newLogCommand(opts),
		newNASCommand(opts),
		newNetworkCommand(opts),
		newNotificationCommand(opts),
		newOfficeCommand(opts),
		newPackageCommand(opts),
		newPhotoCommand(opts),
		newProvisionCommand(opts),
		newResourceMonitorCommand(opts),
		newSANCommand(opts),
		newRequestSchemaCommand(root),
		newShareCommand(opts),
		newSnapshotReplicationCommand(opts),
		newStorageCommand(opts),
		newSurveillanceCommand(opts),
		newSystemCommand(opts),
		newSecurityAdvisorCommand(opts),
		newTaskSchedulerCommand(opts),
		newUniversalSearchCommand(opts),
	)
	decorateWorkflowHelp(root)
	if err := decorateRequestSchemaHelp(root); err != nil {
		panic(err)
	}
	decorateFallbackLongHelp(root)
	decorateCommandCatalogHelp(root)
	return root
}

const planWorkflowHelp = `This is the read-only first half of a guarded mutation. Select every target
explicitly in automation (usually with --nas; some commands use source and
destination flags), provide the typed intent requested by this command, and
save the returned plan. Review its target, intent, risk, summary, warnings,
observed-state precondition, and approval hash. Planning contacts DSM to read
current state but does not mutate it.

If this command accepts a request JSON file, its exact offline
'dsmctl schema show ...' invocation appears on this help page. Commands that
take flags instead describe those fields in Options. Use 'dsmctl schema list'
to enumerate every typed request and never guess fields.

Only after explicit operator approval, run the matching apply command with the
exact unmodified plan and its approval hash. Do not synthesize, edit, or reuse a
plan after its observed state changes.`

const applyWorkflowHelp = `This is the mutating half of a guarded plan/apply workflow. Pass the exact
unmodified plan returned by the matching plan command and the exact approval
hash after an operator has reviewed the plan's target, intent, risk, summary,
warnings, and precondition. A plan already binds its NAS and cannot be
retargeted with flags.

Apply re-reads current state, rejects a stale or modified plan, performs the
typed operation, and verifies the postcondition. High-risk remote applies may
also require a separate out-of-band approval.`

// decorateWorkflowHelp gives every plan/apply leaf a consistent safety and
// discovery contract without duplicating the application workflow in each
// module adapter. Module-specific Long text remains first and authoritative.
func decorateWorkflowHelp(command *cobra.Command) {
	for _, child := range command.Commands() {
		name := child.Name()
		switch {
		case isPlanCommandName(name):
			appendWorkflowHelp(child, planWorkflowHelp)
		case isApplyCommandName(name):
			appendWorkflowHelp(child, applyWorkflowHelp)
		}
		decorateWorkflowHelp(child)
	}
}

func isPlanCommandName(name string) bool {
	return name == "plan" || strings.HasPrefix(name, "plan-") || strings.HasSuffix(name, "-plan")
}

func isApplyCommandName(name string) bool {
	return name == "apply" || strings.HasPrefix(name, "apply-") || strings.HasSuffix(name, "-apply")
}

func appendWorkflowHelp(command *cobra.Command, guidance string) {
	base := strings.TrimSpace(command.Long)
	if base == "" {
		base = strings.TrimSpace(command.Short)
	}
	command.Long = base + "\n\n" + guidance
}

// decorateFallbackLongHelp makes every project command self-routing even when
// it does not need bespoke prose. Existing Long descriptions remain untouched.
func decorateFallbackLongHelp(command *cobra.Command) {
	for _, child := range command.Commands() {
		if strings.TrimSpace(child.Long) == "" {
			base := strings.TrimSpace(child.Short)
			switch {
			case child.HasSubCommands() && child.Runnable():
				child.Long = base + "\n\nThis command can run directly and also has subcommands. The usage and Options sections below describe the direct operation; choose a listed subcommand for narrower behavior."
			case child.HasSubCommands():
				child.Long = base + "\n\nThis is a command group. Choose one of the subcommands below, then run '" + child.CommandPath() + " <subcommand> --help' for its complete inputs and workflow."
			default:
				child.Long = base + "\n\nThe usage and Options sections below enumerate this command's accepted CLI inputs. In automation, provide target and non-default inputs explicitly; do not invent undocumented flags or JSON fields."
			}
		}
		decorateFallbackLongHelp(child)
	}
}

func Execute(ctx context.Context, version string) error {
	return New(version).ExecuteContext(ctx)
}
