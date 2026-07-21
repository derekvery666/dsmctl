package application

import (
	"context"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/config"
)

// list_nas is the managed-NAS listing; a destination-only ("target") profile is
// held for outbound use, not managed, and must not appear there.
func TestListNASContextExcludesTargetProfiles(t *testing.T) {
	cfg := &config.Config{NAS: map[string]config.Profile{
		"managed": {URL: "https://m.example:5001"},
		"backup":  {URL: "https://b.example:5001", Role: config.ProfileRoleTarget},
	}}
	service := &Service{config: cfg, configSource: config.StaticSource{Config: cfg}}
	summaries, err := service.ListNASContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || summaries[0].Name != "managed" {
		t.Fatalf("summaries = %#v; the target profile must be excluded", summaries)
	}
}
