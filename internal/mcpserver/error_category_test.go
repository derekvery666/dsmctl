package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ychiu1211/dsmctl/internal/synology"
)

type categoryProbeInput struct{}

type categoryProbeOutput struct {
	OK bool `json:"ok"`
}

// TestCategoryMiddlewareAttachesCategory proves the single central hook labels a
// failed tool result with the DSM category derived from the handler's typed Go
// error (matching synology.Classify), and never leaks a SID/token into the
// rendered result.
func TestCategoryMiddlewareAttachesCategory(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want synology.Category
	}{
		{
			name: "wrapped dsm permission code",
			err:  fmt.Errorf(`NAS "lab": %w`, &synology.APIError{API: "SYNO.Core.Share", Method: "set", Code: 105}),
			want: synology.CategoryPermission,
		},
		{
			name: "session expired maps to auth",
			err:  &synology.SessionExpiredError{NAS: "lab"},
			want: synology.CategoryAuth,
		},
		{
			name: "otp required maps to auth",
			err:  fmt.Errorf("login: %w", &synology.OTPRequiredError{}),
			want: synology.CategoryAuth,
		},
		{
			name: "plain failure is unknown",
			err:  errors.New("something opaque failed"),
			want: synology.CategoryUnknown,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			result := callProbeTool(t, test.err)
			if !result.IsError {
				t.Fatalf("result.IsError = false, want true for %v", test.err)
			}
			category, message := structuredError(t, result)
			if category != string(test.want) {
				t.Fatalf("category = %q, want %q", category, test.want)
			}
			if category != string(synology.Classify(test.err)) {
				t.Fatalf("category %q disagrees with synology.Classify = %q", category, synology.Classify(test.err))
			}
			if message == "" {
				t.Fatal("structured error carried an empty message")
			}
			assertNoSecretLeak(t, result)
		})
	}
}

func callProbeTool(t *testing.T, toolErr error) *mcp.CallToolResult {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "dsmctl-test", Version: "test"}, nil)
	server.AddReceivingMiddleware(categoryErrorMiddleware())
	mcp.AddTool(server, &mcp.Tool{Name: "probe"}, func(context.Context, *mcp.CallToolRequest, categoryProbeInput) (*mcp.CallToolResult, categoryProbeOutput, error) {
		return nil, categoryProbeOutput{}, toolErr
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "probe", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	return result
}

func structuredError(t *testing.T, result *mcp.CallToolResult) (category, message string) {
	t.Helper()
	payload, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent is %T, want a JSON object with a category field", result.StructuredContent)
	}
	category, _ = payload["category"].(string)
	message, _ = payload["message"].(string)
	if category == "" {
		t.Fatalf("structured content %#v has no category", payload)
	}
	return category, message
}

func assertNoSecretLeak(t *testing.T, result *mcp.CallToolResult) {
	t.Helper()
	serialized, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	rendered := string(serialized)
	// The DSM parameter names that would carry a secret value. (The English word
	// "password" appears in legitimate OTP guidance prose, so it is not a marker.)
	for _, secret := range []string{"_sid", "SynoToken", "passwd", "otp_code", "synotoken"} {
		if strings.Contains(rendered, secret) {
			t.Fatalf("serialized tool error %q leaked %q", rendered, secret)
		}
	}
}
