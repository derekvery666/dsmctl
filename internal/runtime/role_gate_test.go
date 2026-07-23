package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/derekvery666/dsmctl/internal/config"
)

// A destination-only ("target") profile is refused by Client — the choke point
// every management operation resolves through — but remains usable via
// DestinationClient, the bypass reserved for outbound-destination callers.
func TestManagerGatesTargetRoleFromClient(t *testing.T) {
	cfg := config.New()
	cfg.NAS["src"] = config.Profile{URL: "https://src.example:5001", Username: "u"}
	cfg.NAS["dst"] = config.Profile{URL: "https://dst.example:5001", Username: "u", Role: config.ProfileRoleTarget}
	manager := NewManager(cfg, resolverFunc(func(_ context.Context, name string, _ config.Profile) (string, error) {
		return name + "-password", nil
	}))
	ctx := context.Background()

	if _, _, err := manager.Client(ctx, "src"); err != nil {
		t.Fatalf("managed Client error = %v", err)
	}
	if _, _, err := manager.Client(ctx, "dst"); err == nil || !strings.Contains(err.Error(), "destination-only") {
		t.Fatalf("target Client error = %v, want a destination-only refusal", err)
	}
	if name, client, err := manager.DestinationClient(ctx, "dst"); err != nil || name != "dst" || client == nil {
		t.Fatalf("DestinationClient(dst) name=%q client=%v err=%v", name, client, err)
	}
}
