package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRequestSchemaRegistryCoversEveryRequestFileCommand(t *testing.T) {
	root := New("test")
	registered := make(map[string]bool, len(requestSchemaDefinitions))
	for _, definition := range requestSchemaDefinitions {
		if registered[definition.commandPath] {
			t.Errorf("duplicate request schema registration for %q", definition.commandPath)
		}
		registered[definition.commandPath] = true
		command, err := findCommandPath(root, definition.commandPath)
		if err != nil {
			t.Error(err)
			continue
		}
		if !commandConsumesRequestFile(command) {
			t.Errorf("registered command %q does not consume a request JSON file", definition.commandPath)
		}
		if !strings.Contains(command.Long, "dsmctl schema show "+definition.commandPath) {
			t.Errorf("command %q does not link to its request schema", definition.commandPath)
		}
		if !strings.Contains(command.Example, "dsmctl schema show "+definition.commandPath) {
			t.Errorf("command %q does not demonstrate its request schema", definition.commandPath)
		}
		schema, err := requestJSONSchema(definition)
		if err != nil {
			t.Errorf("requestJSONSchema(%q) error = %v", definition.commandPath, err)
			continue
		}
		if schema.Schema != jsonSchemaDraft202012 || schema.Title == "" || schema.Description == "" {
			t.Errorf("schema for %q lacks discovery metadata", definition.commandPath)
		}
		if schema.AdditionalProperties == nil || schema.AdditionalProperties.Not == nil {
			t.Errorf("schema for %q does not reject unknown root properties", definition.commandPath)
		}
	}

	var discovered int
	var inspect func(*cobra.Command)
	inspect = func(command *cobra.Command) {
		if commandConsumesRequestFile(command) {
			discovered++
			path := strings.TrimPrefix(command.CommandPath(), root.Name()+" ")
			if !registered[path] {
				t.Errorf("request-file command %q lacks a schema registration", path)
			}
		}
		for _, child := range command.Commands() {
			inspect(child)
		}
	}
	inspect(root)
	if discovered != len(requestSchemaDefinitions) {
		t.Fatalf("discovered %d request-file commands, registry has %d", discovered, len(requestSchemaDefinitions))
	}
}

func TestRequestSchemaCommandsAreOfflineAndDescriptive(t *testing.T) {
	listOutput := executeCLI(t, "schema", "list", "--json")
	var items []struct {
		Command     string `json:"command"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(listOutput), &items); err != nil {
		t.Fatalf("decode schema list: %v\n%s", err, listOutput)
	}
	if len(items) != len(requestSchemaDefinitions) {
		t.Fatalf("schema list returned %d items, want %d", len(items), len(requestSchemaDefinitions))
	}
	if items[0].Command != "account plan" || items[0].Description == "" {
		t.Fatalf("unexpected first schema list item: %+v", items[0])
	}

	showOutput := executeCLI(t, "schema", "show", "account", "plan")
	for _, required := range []string{
		`"$schema": "https://json-schema.org/draft/2020-12/schema"`,
		`"title": "dsmctl account plan request"`,
		`"additionalProperties": false`,
		`"action"`,
		"Change action: create, update, delete, or set",
		"Password reference such as env:DSMCTL_NEW_USER_PASSWORD; never a plaintext password",
		"semantic, capability, current-state, and safety validation",
	} {
		if !strings.Contains(showOutput, required) {
			t.Errorf("account schema missing %q:\n%s", required, showOutput)
		}
	}

	storageOutput := executeCLI(t, "schema", "show", "dsmctl", "storage", "plan")
	if !strings.Contains(storageOutput, `"title": "dsmctl storage plan request"`) ||
		!strings.Contains(storageOutput, `"pool"`) || !strings.Contains(storageOutput, `"volume"`) {
		t.Errorf("storage schema lacks nested mutation shapes:\n%s", storageOutput)
	}
}

func TestRequestSchemaShowRejectsUnknownCommand(t *testing.T) {
	command := New("test")
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"schema", "show", "made-up", "plan"})
	err := command.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "dsmctl schema list") {
		t.Fatalf("unknown schema error = %v, want discovery guidance", err)
	}
}

func commandConsumesRequestFile(command *cobra.Command) bool {
	flag := command.Flags().Lookup("file")
	if flag == nil {
		return false
	}
	usage := strings.ToLower(flag.Usage)
	return strings.Contains(command.Name(), "plan") && strings.Contains(usage, "json file") && !strings.Contains(usage, "plan json")
}

func executeCLI(t *testing.T, args ...string) string {
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
