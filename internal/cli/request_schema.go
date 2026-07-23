package cli

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/spf13/cobra"

	"github.com/derekvery666/dsmctl/internal/application"
	"github.com/derekvery666/dsmctl/internal/domain/accountprotection"
	"github.com/derekvery666/dsmctl/internal/domain/certificate"
	"github.com/derekvery666/dsmctl/internal/domain/controlpanel"
	"github.com/derekvery666/dsmctl/internal/domain/downloadstation"
	"github.com/derekvery666/dsmctl/internal/domain/driveadmin"
	"github.com/derekvery666/dsmctl/internal/domain/externalaccess"
	"github.com/derekvery666/dsmctl/internal/domain/filestation"
	"github.com/derekvery666/dsmctl/internal/domain/firewall"
	"github.com/derekvery666/dsmctl/internal/domain/ftpservices"
	"github.com/derekvery666/dsmctl/internal/domain/hyperbackup"
	"github.com/derekvery666/dsmctl/internal/domain/identity"
	"github.com/derekvery666/dsmctl/internal/domain/loginportal"
	"github.com/derekvery666/dsmctl/internal/domain/network"
	"github.com/derekvery666/dsmctl/internal/domain/nfsexport"
	"github.com/derekvery666/dsmctl/internal/domain/office"
	"github.com/derekvery666/dsmctl/internal/domain/packagecenter"
	"github.com/derekvery666/dsmctl/internal/domain/photos"
	"github.com/derekvery666/dsmctl/internal/domain/rsyncservice"
	"github.com/derekvery666/dsmctl/internal/domain/san"
	"github.com/derekvery666/dsmctl/internal/domain/securityadvisor"
	"github.com/derekvery666/dsmctl/internal/domain/servicediscovery"
	"github.com/derekvery666/dsmctl/internal/domain/share"
	"github.com/derekvery666/dsmctl/internal/domain/snapshotreplication"
	"github.com/derekvery666/dsmctl/internal/domain/storage"
	"github.com/derekvery666/dsmctl/internal/domain/surveillance"
	"github.com/derekvery666/dsmctl/internal/domain/terminalsnmp"
	"github.com/derekvery666/dsmctl/internal/domain/tftpservice"
)

const jsonSchemaDraft202012 = "https://json-schema.org/draft/2020-12/schema"

type requestSchemaDefinition struct {
	commandPath string
	requestType reflect.Type
}

// requestSchemaDefinitions is the binary-only discovery index for every CLI
// command whose --file input is a typed request (as opposed to an apply plan).
// The reflected types are the same values decoded by the command adapters.
var requestSchemaDefinitions = []requestSchemaDefinition{
	{"account plan", reflect.TypeFor[identity.ChangeRequest]()},
	{"account-protection autoblock-plan", reflect.TypeFor[accountprotection.AutoBlockChange]()},
	{"account-protection enforce-2fa-plan", reflect.TypeFor[accountprotection.EnforceTwoFactorChange]()},
	{"account-protection list-plan", reflect.TypeFor[accountprotection.IPListEdit]()},
	{"account-protection protection-plan", reflect.TypeFor[accountprotection.AccountProtectionChange]()},
	{"backup lun-backups plan", reflect.TypeFor[hyperbackup.LunBackupChange]()},
	{"backup tasks plan", reflect.TypeFor[hyperbackup.TaskChange]()},
	{"certificate plan", reflect.TypeFor[certificate.ChangeRequest]()},
	{"control-panel time plan", reflect.TypeFor[controlpanel.TimeChange]()},
	{"download settings plan", reflect.TypeFor[downloadstation.SettingsChange]()},
	{"download tasks plan", reflect.TypeFor[downloadstation.TaskChange]()},
	{"drive admin restore plan", reflect.TypeFor[application.NodeRestoreChange]()},
	{"drive admin team-folders plan", reflect.TypeFor[driveadmin.TeamFolderChange]()},
	{"drive config plan", reflect.TypeFor[driveadmin.ServerConfigChange]()},
	{"external-access ddns plan", reflect.TypeFor[externalaccess.DDNSRecordChange]()},
	{"external-access quickconnect config plan", reflect.TypeFor[externalaccess.QuickConnectConfigChange]()},
	{"external-access quickconnect permission plan", reflect.TypeFor[externalaccess.QuickConnectPermissionChange]()},
	{"external-access quickconnect plan", reflect.TypeFor[externalaccess.QuickConnectChange]()},
	{"file plan", reflect.TypeFor[filestation.ChangeRequest]()},
	{"control-panel file-services discovery plan", reflect.TypeFor[servicediscovery.Change]()},
	{"control-panel file-services ftp plan", reflect.TypeFor[ftpservices.Change]()},
	{"control-panel file-services nfs export plan", reflect.TypeFor[nfsexport.ChangeRequest]()},
	{"control-panel file-services plan", reflect.TypeFor[controlpanel.FileServiceChangeRequest]()},
	{"control-panel file-services rsync plan", reflect.TypeFor[rsyncservice.Change]()},
	{"control-panel file-services tftp plan", reflect.TypeFor[tftpservice.Change]()},
	{"firewall enable-plan", reflect.TypeFor[firewall.EnableChange]()},
	{"firewall profile-plan", reflect.TypeFor[firewall.ProfileChange]()},
	{"control-panel login-portal application-plan", reflect.TypeFor[loginportal.ApplicationPortalChange]()},
	{"control-panel login-portal dsm-plan", reflect.TypeFor[loginportal.DSMWebServiceChange]()},
	{"control-panel login-portal reverse-proxy-create-plan", reflect.TypeFor[loginportal.ReverseProxyRuleCreate]()},
	{"control-panel login-portal reverse-proxy-delete-plan", reflect.TypeFor[loginportal.ReverseProxyRuleDelete]()},
	{"network general-plan", reflect.TypeFor[network.GeneralChange]()},
	{"network interface-plan", reflect.TypeFor[network.InterfaceChange]()},
	{"office plan", reflect.TypeFor[office.Change]()},
	{"package plan", reflect.TypeFor[packagecenter.ChangeRequest]()},
	{"photo plan", reflect.TypeFor[photos.AdminChange]()},
	{"san plan", reflect.TypeFor[san.ChangeRequest]()},
	{"security-advisor plan", reflect.TypeFor[securityadvisor.ScheduleChange]()},
	{"share plan", reflect.TypeFor[share.ChangeRequest]()},
	{"snapshot plan", reflect.TypeFor[snapshotreplication.Change]()},
	{"storage plan", reflect.TypeFor[storage.ChangeRequest]()},
	{"surveillance homemode plan", reflect.TypeFor[surveillance.HomeModeChange]()},
	{"control-panel terminal-snmp snmp-plan", reflect.TypeFor[terminalsnmp.SNMPChange]()},
	{"control-panel terminal-snmp terminal-plan", reflect.TypeFor[terminalsnmp.TerminalChange]()},
}

func newRequestSchemaCommand(root *cobra.Command) *cobra.Command {
	command := &cobra.Command{
		Use:   "schema",
		Short: "Discover typed request JSON accepted by CLI plan commands",
		Long: `Discover request JSON shapes without repository files, a NAS connection, or an MCP client.

Use 'dsmctl schema list' to find every command that accepts a typed request
file, then pass its command path to 'dsmctl schema show'. The emitted JSON
Schema describes required fields, nested objects, JSON types, accepted semantic
values, credential-reference rules, and whether extra properties are allowed.

JSON Schema checks structure only. The matching plan command performs the full
semantic, capability, current-state, and safety validation and does not mutate
DSM. Never bypass planning or invent fields that are absent from the schema.`,
		Example: `  dsmctl schema list
  dsmctl schema list --json
  dsmctl schema show account plan
  dsmctl schema show storage plan > storage-request.schema.json`,
	}
	command.AddCommand(newRequestSchemaListCommand(root), newRequestSchemaShowCommand())
	return command
}

func newRequestSchemaListCommand(root *cobra.Command) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "list",
		Short: "List CLI commands with typed request JSON schemas",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			type item struct {
				Command     string `json:"command"`
				Description string `json:"description"`
			}
			definitions := sortedRequestSchemaDefinitions()
			items := make([]item, 0, len(definitions))
			for _, definition := range definitions {
				target, err := findCommandPath(root, definition.commandPath)
				if err != nil {
					return err
				}
				items = append(items, item{Command: definition.commandPath, Description: target.Short})
			}
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), items)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(writer, "COMMAND\tDESCRIPTION")
			for _, item := range items {
				fmt.Fprintf(writer, "%s\t%s\n", item.Command, item.Description)
			}
			return writer.Flush()
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output a structured JSON array")
	return command
}

func newRequestSchemaShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <command path...>",
		Short: "Emit the request JSON Schema for one CLI command",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := normalizeSchemaCommandPath(args)
			definition, ok := requestSchemaDefinitionForPath(path)
			if !ok {
				return fmt.Errorf("no typed request schema for %q; run 'dsmctl schema list' to see valid command paths", path)
			}
			schema, err := requestJSONSchema(definition)
			if err != nil {
				return err
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(schema)
		},
	}
}

func requestJSONSchema(definition requestSchemaDefinition) (*jsonschema.Schema, error) {
	schema, err := jsonschema.ForType(definition.requestType, nil)
	if err != nil {
		return nil, fmt.Errorf("generate request schema for dsmctl %s: %w", definition.commandPath, err)
	}
	schema.Schema = jsonSchemaDraft202012
	schema.Title = "dsmctl " + definition.commandPath + " request"
	schema.Description = "Typed request JSON accepted by 'dsmctl " + definition.commandPath +
		" --file'. This schema validates structure only; run the plan command for semantic, capability, current-state, and safety validation."
	return schema, nil
}

func requestSchemaDefinitionForPath(path string) (requestSchemaDefinition, bool) {
	for _, definition := range requestSchemaDefinitions {
		if definition.commandPath == path {
			return definition, true
		}
	}
	return requestSchemaDefinition{}, false
}

func sortedRequestSchemaDefinitions() []requestSchemaDefinition {
	definitions := append([]requestSchemaDefinition(nil), requestSchemaDefinitions...)
	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].commandPath < definitions[j].commandPath
	})
	return definitions
}

func normalizeSchemaCommandPath(args []string) string {
	path := strings.TrimSpace(strings.Join(args, " "))
	path = strings.TrimPrefix(path, "dsmctl ")
	return strings.Join(strings.Fields(path), " ")
}

func findCommandPath(root *cobra.Command, path string) (*cobra.Command, error) {
	target, remaining, err := root.Find(strings.Fields(path))
	if err != nil || target == nil || len(remaining) != 0 || strings.TrimPrefix(target.CommandPath(), root.Name()+" ") != path {
		return nil, fmt.Errorf("registered request schema command %q does not exist", path)
	}
	return target, nil
}

func decorateRequestSchemaHelp(root *cobra.Command) error {
	for _, definition := range requestSchemaDefinitions {
		command, err := findCommandPath(root, definition.commandPath)
		if err != nil {
			return err
		}
		guidance := fmt.Sprintf(`Request JSON discovery (offline):
  dsmctl schema show %s

The schema is generated from the exact typed request decoded by this command.
It describes structure; this plan command performs semantic and current-state
validation. Run 'dsmctl schema list' to discover every request-bearing command.`, definition.commandPath)
		appendWorkflowHelp(command, guidance)
		if strings.TrimSpace(command.Example) == "" {
			command.Example = fmt.Sprintf("  dsmctl schema show %s\n  dsmctl %s --nas <profile> --file request.json --output plan.json", definition.commandPath, definition.commandPath)
		}
	}
	return nil
}
