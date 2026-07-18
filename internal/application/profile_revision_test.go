package application

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/config"
)

type mutableConfigSource struct {
	mu  sync.Mutex
	cfg *config.Config
}

func (source *mutableConfigSource) Snapshot(context.Context) (*config.Config, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.cfg.Clone(), nil
}

func TestProfileRevisionRejectsChangedAndRemovedGatewayProfile(t *testing.T) {
	source := &mutableConfigSource{cfg: &config.Config{DefaultNAS: "office", NAS: map[string]config.Profile{
		"office": {URL: "https://office.example:5001", Revision: 7},
	}}}
	service := &Service{configSource: source}
	if err := service.verifyProfileRevision(context.Background(), "office", 7); err != nil {
		t.Fatalf("matching revision rejected: %v", err)
	}
	source.mu.Lock()
	profile := source.cfg.NAS["office"]
	profile.Revision = 8
	source.cfg.NAS["office"] = profile
	source.mu.Unlock()
	if err := service.verifyProfileRevision(context.Background(), "office", 7); err == nil || !strings.Contains(err.Error(), "changed after planning") {
		t.Fatalf("changed revision error = %v", err)
	}
	source.mu.Lock()
	delete(source.cfg.NAS, "office")
	source.mu.Unlock()
	if err := service.verifyProfileRevision(context.Background(), "office", 7); err == nil || !strings.Contains(err.Error(), "no longer configured") {
		t.Fatalf("removed profile error = %v", err)
	}
}

func TestNonzeroProfileRevisionParticipatesInPlanHash(t *testing.T) {
	first := IdentityPlan{APIVersion: managementAPIVersion, NAS: "office", ProfileRevision: 10}
	second := first
	second.ProfileRevision = 11
	firstHash, err := identityPlanHash(first)
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := identityPlanHash(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstHash == secondHash {
		t.Fatal("profile revision did not participate in the plan hash")
	}
}

func TestSecretReferenceValidationAllowsOpaqueVaultIDs(t *testing.T) {
	if !validSecretReference("env:DSMCTL_PASSWORD") || !validSecretReference("vault:0123456789abcdef0123456789abcdef") {
		t.Fatal("valid environment or vault reference was rejected")
	}
	for _, invalid := range []string{"vault:short", "vault:0123456789ABCDEF0123456789ABCDEF", "plaintext"} {
		if validSecretReference(invalid) {
			t.Fatalf("invalid secret reference %q was accepted", invalid)
		}
	}
}
