package main

import (
	"strings"
	"testing"
)

func TestMCPUsagePreambleExplainsAgentWorkflow(t *testing.T) {
	usage := mcpUsagePreamble("dsmctl-mcp")
	for _, required := range []string{
		"MCP server over stdio",
		"stdout is reserved for JSON-RPC",
		"dsmctl nas add",
		"dsmctl auth login --nas <name>",
		"begin with list_nas and get_auth_status",
		"plan_* then approved apply_*",
	} {
		if !strings.Contains(usage, required) {
			t.Errorf("usage missing %q:\n%s", required, usage)
		}
	}
}
