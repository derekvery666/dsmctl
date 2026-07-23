package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type commandCatalogItem struct {
	Path             string `json:"path"`
	Summary          string `json:"summary"`
	Role             string `json:"role"`
	Runnable         bool   `json:"runnable"`
	StructuredOutput bool   `json:"structured_output"`
	RequestSchema    string `json:"request_schema,omitempty"`
}

type commandCatalogDescription struct {
	Path             string                   `json:"path"`
	Summary          string                   `json:"summary"`
	Description      string                   `json:"description"`
	Usage            string                   `json:"usage"`
	Role             string                   `json:"role"`
	Runnable         bool                     `json:"runnable"`
	Aliases          []string                 `json:"aliases,omitempty"`
	Subcommands      []commandCatalogItem     `json:"subcommands,omitempty"`
	Flags            []commandFlagDescription `json:"flags,omitempty"`
	InheritedFlags   []commandFlagDescription `json:"inherited_flags,omitempty"`
	StructuredOutput bool                     `json:"structured_output"`
	RequestSchema    string                   `json:"request_schema,omitempty"`
}

type commandFlagDescription struct {
	Name      string `json:"name"`
	Shorthand string `json:"shorthand,omitempty"`
	Type      string `json:"type"`
	Usage     string `json:"usage"`
	Default   string `json:"default"`
	Required  bool   `json:"required"`
}

func newCommandCatalogCommand(root *cobra.Command) *cobra.Command {
	command := &cobra.Command{
		Use:   "commands",
		Short: "Discover every CLI command and its exact input contract",
		Long: `Inspect the complete live CLI command tree without configuration, credentials, a NAS connection, or MCP.

'dsmctl commands list' provides a concise index over command groups and
executable operations. Filter it to one module with --prefix or to executable
operations with --runnable-only. 'dsmctl commands show' reports the exact
summary, long description, usage, aliases, subcommands, flags, defaults,
required flags, structured-output support, workflow role, and request-schema
lookup for one command.

Roles are conservative: plan is read-only planning, apply is mutating, group is
navigation, and operation may be a read, local action, authentication action,
or direct mutation as stated by its command-specific description.`,
		Example: `  dsmctl commands list
  dsmctl commands list --prefix drive --runnable-only
  dsmctl commands list --prefix control-panel --json
  dsmctl commands show account inventory --json
  dsmctl commands show control-panel file-services plan --json`,
	}
	command.AddCommand(newCommandCatalogListCommand(root), newCommandCatalogShowCommand(root))
	return command
}

func newCommandCatalogListCommand(root *cobra.Command) *cobra.Command {
	var jsonOutput, runnableOnly bool
	var prefix string
	command := &cobra.Command{
		Use:   "list",
		Short: "List visible CLI groups and operations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			items := commandCatalogItems(root, prefix, runnableOnly)
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), items)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(writer, "COMMAND\tROLE\tJSON\tREQUEST SCHEMA\tSUMMARY")
			for _, item := range items {
				requestSchema := valueOrDash(item.RequestSchema)
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", item.Path, item.Role, yesNo(item.StructuredOutput), requestSchema, item.Summary)
			}
			return writer.Flush()
		},
	}
	command.Flags().StringVar(&prefix, "prefix", "", "limit to this canonical command path, for example drive or control-panel file-services")
	command.Flags().BoolVar(&runnableOnly, "runnable-only", false, "omit navigation-only command groups")
	command.Flags().BoolVar(&jsonOutput, "json", false, "output a structured JSON array")
	return command
}

func newCommandCatalogShowCommand(root *cobra.Command) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "show <command path...>",
		Short: "Show the complete input and discovery contract for one command",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := normalizeCatalogPath(args)
			target, err := findCatalogCommand(root, path)
			if err != nil {
				return err
			}
			description := describeCatalogCommand(root, target)
			if jsonOutput {
				return encodeIndentedJSON(cmd.OutOrStdout(), description)
			}
			return writeCommandCatalogDescription(cmd, description)
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "output structured JSON")
	return command
}

func commandCatalogItems(root *cobra.Command, prefix string, runnableOnly bool) []commandCatalogItem {
	prefix = normalizeCatalogPath([]string{prefix})
	var items []commandCatalogItem
	var collect func(*cobra.Command)
	collect = func(parent *cobra.Command) {
		for _, command := range parent.Commands() {
			if !isCatalogCommand(root, command) {
				continue
			}
			path := canonicalCommandPath(root, command)
			if catalogPathMatchesPrefix(path, prefix) && (!runnableOnly || command.Runnable()) {
				items = append(items, catalogItemForCommand(root, command))
			}
			collect(command)
		}
	}
	collect(root)
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return items
}

func describeCatalogCommand(root, command *cobra.Command) commandCatalogDescription {
	subcommands := make([]commandCatalogItem, 0, len(command.Commands()))
	for _, child := range command.Commands() {
		if isCatalogCommand(root, child) {
			subcommands = append(subcommands, catalogItemForCommand(root, child))
		}
	}
	sort.Slice(subcommands, func(i, j int) bool { return subcommands[i].Path < subcommands[j].Path })
	path := canonicalCommandPath(root, command)
	return commandCatalogDescription{
		Path:             path,
		Summary:          strings.TrimSpace(command.Short),
		Description:      strings.TrimSpace(command.Long),
		Usage:            command.UseLine(),
		Role:             commandCatalogRole(command),
		Runnable:         command.Runnable(),
		Aliases:          append([]string(nil), command.Aliases...),
		Subcommands:      subcommands,
		Flags:            describeFlagSet(command.LocalNonPersistentFlags()),
		InheritedFlags:   describeFlagSet(command.InheritedFlags()),
		StructuredOutput: commandHasStructuredOutput(command),
		RequestSchema:    requestSchemaLookup(path),
	}
}

func catalogItemForCommand(root, command *cobra.Command) commandCatalogItem {
	path := canonicalCommandPath(root, command)
	return commandCatalogItem{
		Path:             path,
		Summary:          strings.TrimSpace(command.Short),
		Role:             commandCatalogRole(command),
		Runnable:         command.Runnable(),
		StructuredOutput: commandHasStructuredOutput(command),
		RequestSchema:    requestSchemaLookup(path),
	}
}

func describeFlagSet(flags *pflag.FlagSet) []commandFlagDescription {
	var descriptions []commandFlagDescription
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Name == "help" {
			return
		}
		_, required := flag.Annotations[cobra.BashCompOneRequiredFlag]
		descriptions = append(descriptions, commandFlagDescription{
			Name:      flag.Name,
			Shorthand: flag.Shorthand,
			Type:      flag.Value.Type(),
			Usage:     flag.Usage,
			Default:   flag.DefValue,
			Required:  required,
		})
	})
	sort.Slice(descriptions, func(i, j int) bool { return descriptions[i].Name < descriptions[j].Name })
	return descriptions
}

func writeCommandCatalogDescription(cmd *cobra.Command, description commandCatalogDescription) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Command: %s\nRole: %s\nRunnable: %s\nStructured JSON output: %s\nSummary: %s\nUsage: %s\n",
		description.Path, description.Role, yesNo(description.Runnable), yesNo(description.StructuredOutput), description.Summary, description.Usage)
	if len(description.Aliases) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Aliases: %s\n", strings.Join(description.Aliases, ", "))
	}
	if description.RequestSchema != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Request schema: %s\n", description.RequestSchema)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nDescription:\n%s\n", description.Description)
	if len(description.Subcommands) > 0 {
		writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		fmt.Fprintln(writer, "\nSUBCOMMAND\tROLE\tSUMMARY")
		for _, child := range description.Subcommands {
			fmt.Fprintf(writer, "%s\t%s\t%s\n", child.Path, child.Role, child.Summary)
		}
		if err := writer.Flush(); err != nil {
			return err
		}
	}
	return writeCommandFlags(cmd, "FLAGS", description.Flags, description.InheritedFlags)
}

func writeCommandFlags(cmd *cobra.Command, heading string, local, inherited []commandFlagDescription) error {
	if len(local) == 0 && len(inherited) == 0 {
		return nil
	}
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintf(writer, "\n%s\nSCOPE\tFLAG\tTYPE\tREQUIRED\tDEFAULT\tDESCRIPTION\n", heading)
	for _, entry := range local {
		fmt.Fprintf(writer, "local\t%s\t%s\t%s\t%s\t%s\n", formatCatalogFlag(entry), entry.Type, yesNo(entry.Required), entry.Default, entry.Usage)
	}
	for _, entry := range inherited {
		fmt.Fprintf(writer, "inherited\t%s\t%s\t%s\t%s\t%s\n", formatCatalogFlag(entry), entry.Type, yesNo(entry.Required), entry.Default, entry.Usage)
	}
	return writer.Flush()
}

func formatCatalogFlag(flag commandFlagDescription) string {
	if flag.Shorthand != "" {
		return "-" + flag.Shorthand + ", --" + flag.Name
	}
	return "--" + flag.Name
}

func commandCatalogRole(command *cobra.Command) string {
	if !command.Runnable() {
		return "group"
	}
	name := command.Name()
	summary := strings.ToLower(strings.TrimSpace(command.Short))
	switch {
	case isPlanCommandName(name) || strings.Contains(summary, "emit an approval plan") || strings.HasPrefix(summary, "plan a "):
		return "plan"
	case isApplyCommandName(name):
		return "apply"
	default:
		return "operation"
	}
}

func commandHasStructuredOutput(command *cobra.Command) bool {
	role := commandCatalogRole(command)
	return role == "plan" || role == "apply" || command.Flags().Lookup("json") != nil
}

func requestSchemaLookup(path string) string {
	if _, ok := requestSchemaDefinitionForPath(path); ok {
		return "dsmctl schema show " + path
	}
	return ""
}

func findCatalogCommand(root *cobra.Command, path string) (*cobra.Command, error) {
	command, remaining, err := root.Find(strings.Fields(path))
	if err != nil || command == nil || len(remaining) != 0 || canonicalCommandPath(root, command) != path || !isCatalogCommand(root, command) {
		return nil, fmt.Errorf("unknown CLI command %q; run 'dsmctl commands list' or filter it with --prefix", path)
	}
	return command, nil
}

func canonicalCommandPath(root, command *cobra.Command) string {
	return strings.TrimPrefix(command.CommandPath(), root.Name()+" ")
}

func normalizeCatalogPath(args []string) string {
	path := strings.Join(strings.Fields(strings.Join(args, " ")), " ")
	path = strings.TrimPrefix(path, "dsmctl ")
	return strings.TrimSpace(path)
}

func catalogPathMatchesPrefix(path, prefix string) bool {
	return prefix == "" || path == prefix || strings.HasPrefix(path, prefix+" ")
}

func isCatalogCommand(root, command *cobra.Command) bool {
	if command.Hidden {
		return false
	}
	if command.Parent() == root && (command.Name() == "help" || command.Name() == "completion") {
		return false
	}
	return true
}

func decorateCommandCatalogHelp(root *cobra.Command) {
	for _, command := range root.Commands() {
		if !isCatalogCommand(root, command) {
			continue
		}
		path := canonicalCommandPath(root, command)
		guidance := "Offline command metadata:\n  dsmctl commands show " + path + " --json"
		appendWorkflowHelp(command, guidance)
		decorateCommandCatalogHelpFrom(root, command)
	}
}

func decorateCommandCatalogHelpFrom(root, parent *cobra.Command) {
	for _, command := range parent.Commands() {
		if !isCatalogCommand(root, command) {
			continue
		}
		path := canonicalCommandPath(root, command)
		guidance := "Offline command metadata:\n  dsmctl commands show " + path + " --json"
		appendWorkflowHelp(command, guidance)
		decorateCommandCatalogHelpFrom(root, command)
	}
}
