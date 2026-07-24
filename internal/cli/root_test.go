package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootHelpGuidesFirstTimeAgent(t *testing.T) {
	output := compactWhitespace(executeHelp(t, "--help"))
	for _, required := range []string{
		"dsmctl nas add office",
		"dsmctl auth login --nas office",
		"pass --nas explicitly",
		"dsmctl nas capabilities --nas office",
		"dsmctl schema list",
		"dsmctl schema show account plan",
		"dsmctl account plan --nas office",
		"exact unmodified plan and hash",
		"Passwords and OTPs do not belong",
	} {
		if !strings.Contains(output, compactWhitespace(required)) {
			t.Errorf("root help missing %q:\n%s", required, output)
		}
	}
}

func TestPlanAndApplyHelpExplainGuardedWorkflow(t *testing.T) {
	planHelp := compactWhitespace(executeHelp(t, "account", "plan", "--help"))
	for _, required := range []string{
		"read-only first half of a guarded mutation",
		"does not mutate it",
		"explicit operator approval",
		"Do not synthesize, edit, or reuse",
		"dsmctl schema show account plan",
		"dsmctl schema list",
	} {
		if !strings.Contains(planHelp, compactWhitespace(required)) {
			t.Errorf("plan help missing %q:\n%s", required, planHelp)
		}
	}

	applyHelp := compactWhitespace(executeHelp(t, "storage", "apply", "--help"))
	for _, required := range []string{
		"mutating half of a guarded plan/apply workflow",
		"exact unmodified plan",
		"cannot be retargeted with flags",
		"rejects a stale or modified plan",
		"confirmation in the active MCP conversation",
		"one-time administrator approval",
	} {
		if !strings.Contains(applyHelp, compactWhitespace(required)) {
			t.Errorf("apply help missing %q:\n%s", required, applyHelp)
		}
	}
}

func TestEveryCommandHasLongFormHelp(t *testing.T) {
	root := New("test")
	var commandCount int
	var inspect func(*cobra.Command)
	inspect = func(command *cobra.Command) {
		commandCount++
		if strings.TrimSpace(command.Long) == "" {
			t.Errorf("command %q lacks long-form help", command.CommandPath())
		}
		for _, child := range command.Commands() {
			inspect(child)
		}
	}
	inspect(root)
	if commandCount < 300 {
		t.Fatalf("command-tree scan found only %d commands", commandCount)
	}
}

func TestEveryPlanAndApplyCommandHasWorkflowHelp(t *testing.T) {
	root := New("test")
	var planCommands, applyCommands int
	var inspect func(*cobra.Command)
	inspect = func(command *cobra.Command) {
		for _, child := range command.Commands() {
			switch {
			case isPlanCommandName(child.Name()):
				planCommands++
				if !strings.Contains(child.Long, "read-only first half of a guarded mutation") {
					t.Errorf("plan command %q lacks workflow help", child.CommandPath())
				}
			case isApplyCommandName(child.Name()):
				applyCommands++
				if !strings.Contains(child.Long, "mutating half of a guarded plan/apply workflow") {
					t.Errorf("apply command %q lacks workflow help", child.CommandPath())
				}
			}
			inspect(child)
		}
	}
	inspect(root)
	if planCommands == 0 || applyCommands == 0 {
		t.Fatalf("workflow scan found %d plan and %d apply commands", planCommands, applyCommands)
	}
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func executeHelp(t *testing.T, args ...string) string {
	t.Helper()
	command := New("test")
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(args)
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext(%v) error = %v", args, err)
	}
	return output.String()
}
