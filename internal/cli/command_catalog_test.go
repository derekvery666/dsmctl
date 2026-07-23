package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandCatalogCoversEveryVisibleProjectCommand(t *testing.T) {
	root := New("test")
	items := commandCatalogItems(root, "", false)
	cataloged := make(map[string]commandCatalogItem, len(items))
	for _, item := range items {
		if _, duplicate := cataloged[item.Path]; duplicate {
			t.Errorf("duplicate catalog path %q", item.Path)
		}
		cataloged[item.Path] = item
		if item.Summary == "" || item.Role == "" {
			t.Errorf("catalog item %q lacks summary or role: %+v", item.Path, item)
		}
	}

	var visible, runnable int
	var inspect func(*cobra.Command)
	inspect = func(parent *cobra.Command) {
		for _, command := range parent.Commands() {
			if !isCatalogCommand(root, command) {
				continue
			}
			visible++
			if command.Runnable() {
				runnable++
			}
			path := canonicalCommandPath(root, command)
			if _, ok := cataloged[path]; !ok {
				t.Errorf("visible command %q missing from catalog", path)
			}
			description := describeCatalogCommand(root, command)
			if description.Summary == "" || description.Description == "" || description.Usage == "" {
				t.Errorf("catalog description for %q is incomplete: %+v", path, description)
			}
			if !strings.Contains(command.Long, "dsmctl commands show "+path+" --json") {
				t.Errorf("command %q lacks exact offline catalog guidance", path)
			}
			inspect(command)
		}
	}
	inspect(root)
	if visible != len(items) {
		t.Fatalf("visible command count %d != catalog count %d", visible, len(items))
	}
	if visible < 300 || runnable < 250 {
		t.Fatalf("unexpectedly small CLI tree: %d visible, %d runnable", visible, runnable)
	}
	if got := len(commandCatalogItems(root, "", true)); got != runnable {
		t.Fatalf("runnable-only catalog count %d, want %d", got, runnable)
	}
}

func TestCommandCatalogRepresentativeAccountSMBAndDriveContracts(t *testing.T) {
	root := New("test")
	tests := []struct {
		path             string
		role             string
		structuredOutput bool
		requestSchema    string
		flags            []string
	}{
		{
			path:             "account inventory",
			role:             "operation",
			structuredOutput: true,
			flags:            []string{"application-privileges", "json", "memberships", "principal", "principal-type", "quotas"},
		},
		{
			path:             "account plan",
			role:             "plan",
			structuredOutput: true,
			requestSchema:    "dsmctl schema show account plan",
			flags:            []string{"file", "output"},
		},
		{
			path:             "control-panel file-services smb state",
			role:             "operation",
			structuredOutput: true,
			flags:            []string{"json"},
		},
		{
			path:             "control-panel file-services plan",
			role:             "plan",
			structuredOutput: true,
			requestSchema:    "dsmctl schema show control-panel file-services plan",
			flags:            []string{"file", "output"},
		},
		{
			path:             "drive config plan",
			role:             "plan",
			structuredOutput: true,
			requestSchema:    "dsmctl schema show drive config plan",
			flags:            []string{"file", "output"},
		},
		{
			path:             "drive admin connections",
			role:             "operation",
			structuredOutput: true,
			flags:            []string{"json"},
		},
		{
			path:             "drive admin connections kick",
			role:             "plan",
			structuredOutput: true,
			flags:            []string{"session"},
		},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			command, err := findCatalogCommand(root, test.path)
			if err != nil {
				t.Fatal(err)
			}
			description := describeCatalogCommand(root, command)
			if description.Role != test.role || description.StructuredOutput != test.structuredOutput || description.RequestSchema != test.requestSchema {
				t.Errorf("metadata = role %q, structured %v, schema %q", description.Role, description.StructuredOutput, description.RequestSchema)
			}
			for _, flag := range test.flags {
				if !catalogHasFlag(description, flag) {
					t.Errorf("catalog for %q lacks flag --%s: %+v", test.path, flag, description.Flags)
				}
			}
			if !catalogHasFlag(description, "nas") {
				t.Errorf("catalog for %q lacks inherited --nas", test.path)
			}
		})
	}

	apply, err := findCatalogCommand(root, "account apply")
	if err != nil {
		t.Fatal(err)
	}
	applyDescription := describeCatalogCommand(root, apply)
	if applyDescription.Role != "apply" || !catalogFlagRequired(applyDescription, "approve") {
		t.Errorf("account apply does not expose mutating role and required approval: %+v", applyDescription)
	}
}

func TestCommandCatalogCommandsFilterAndRenderOffline(t *testing.T) {
	output := executeCLI(t, "commands", "list", "--prefix", "drive", "--runnable-only", "--json")
	var items []commandCatalogItem
	if err := json.Unmarshal([]byte(output), &items); err != nil {
		t.Fatalf("decode drive command list: %v\n%s", err, output)
	}
	if len(items) < 15 {
		t.Fatalf("drive catalog returned only %d operations", len(items))
	}
	for _, item := range items {
		if !item.Runnable || !strings.HasPrefix(item.Path, "drive ") {
			t.Errorf("unexpected filtered item: %+v", item)
		}
	}

	show := executeCLI(t, "commands", "show", "control-panel", "file-services", "plan", "--json")
	var description commandCatalogDescription
	if err := json.Unmarshal([]byte(show), &description); err != nil {
		t.Fatalf("decode command show: %v\n%s", err, show)
	}
	if description.Role != "plan" || description.RequestSchema == "" || description.Usage == "" || len(description.Flags) < 2 {
		t.Errorf("incomplete File Services plan description: %+v", description)
	}

	human := executeCLI(t, "commands", "show", "account", "inventory")
	for _, required := range []string{"Command: account inventory", "Role: operation", "Structured JSON output: yes", "--memberships", "--nas"} {
		if !strings.Contains(human, required) {
			t.Errorf("human catalog output missing %q:\n%s", required, human)
		}
	}
}

func TestCommandCatalogShowRejectsUnknownCommand(t *testing.T) {
	command := New("test")
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"commands", "show", "made-up"})
	err := command.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "dsmctl commands list") {
		t.Fatalf("unknown command error = %v, want catalog discovery guidance", err)
	}
}

func catalogHasFlag(description commandCatalogDescription, name string) bool {
	for _, flag := range append(append([]commandFlagDescription(nil), description.Flags...), description.InheritedFlags...) {
		if flag.Name == name {
			return true
		}
	}
	return false
}

func catalogFlagRequired(description commandCatalogDescription, name string) bool {
	for _, flag := range append(append([]commandFlagDescription(nil), description.Flags...), description.InheritedFlags...) {
		if flag.Name == name {
			return flag.Required
		}
	}
	return false
}
