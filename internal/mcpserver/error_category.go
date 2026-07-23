package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/derekvery666/dsmctl/internal/synology"
)

// callToolMethod is the JSON-RPC method the MCP SDK dispatches a tool call under.
const callToolMethod = "tools/call"

// toolErrorCategory is the machine-readable structured content attached to every
// failed tool result: a stable category string from the closed DSM taxonomy plus
// the human-readable message. A client or model can branch on Category instead
// of parsing the prose in Message.
type toolErrorCategory struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

// categoryErrorMiddleware returns receiving middleware that classifies the Go
// error behind every failed tools/call result and attaches its DSM category as
// structured content. It is the SINGLE interception point for all tool handlers,
// so no per-tool wiring is required: the SDK stashes the handler's original error
// on the CallToolResult (recoverable with GetError), letting the category come
// from the typed error via synology.Classify rather than from string-matching the
// message. SessionExpiredError and OTP guidance therefore still classify as auth.
//
// Secret hygiene: the attached message is the error string the tool already
// returns, which is redacted at its source (an APIError carries only API/method/
// code; an HTTPError renders the redacted endpoint), so this hook introduces no
// SID, SynoToken, password, OTP, or request body.
func categoryErrorMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			result, err := next(ctx, method, req)
			if err != nil || method != callToolMethod {
				return result, err
			}
			callResult, ok := result.(*mcp.CallToolResult)
			if !ok || callResult == nil || !callResult.IsError {
				return result, err
			}
			toolErr := callResult.GetError()
			callResult.StructuredContent = toolErrorCategory{
				Category: string(synology.Classify(toolErr)),
				Message:  toolErrorMessage(callResult, toolErr),
			}
			return callResult, nil
		}
	}
}

// toolErrorMessage recovers the human message for a failed tool result,
// preferring the original error and falling back to the text content the SDK
// already populated on the result.
func toolErrorMessage(result *mcp.CallToolResult, err error) string {
	if err != nil {
		return err.Error()
	}
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok && text.Text != "" {
			return text.Text
		}
	}
	return ""
}
