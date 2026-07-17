package gateway

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/config"
)

func TestValidateConfigBoundsFleetAndTimeout(t *testing.T) {
	cfg := config.New()
	for index := 0; index < MaxProfiles+1; index++ {
		cfg.NAS[fmt.Sprintf("nas-%d", index)] = config.Profile{URL: "https://nas.example.test", Username: "operator"}
	}
	if err := ValidateConfig(cfg); err == nil || !strings.Contains(err.Error(), "at most 32") {
		t.Fatalf("ValidateConfig() error = %v", err)
	}

	cfg = config.New()
	cfg.NAS["lab"] = config.Profile{URL: "https://nas.example.test", Username: "operator", TimeoutSeconds: MaxTimeoutSeconds + 1}
	if err := ValidateConfig(cfg); err == nil || !strings.Contains(err.Error(), "maximum 120") {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
}

func TestReadDevelopmentToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(testToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	token, err := ReadDevelopmentToken(path)
	if err != nil {
		t.Fatalf("ReadDevelopmentToken() error = %v", err)
	}
	if token != testToken {
		t.Fatalf("token = %q", token)
	}
	if !DevelopmentTokenMatches(DevelopmentTokenDigest(token), token) {
		t.Fatal("DevelopmentTokenMatches() = false")
	}

	if err := os.WriteFile(path, []byte("too-short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDevelopmentToken(path); err == nil {
		t.Fatal("short token was accepted")
	}
}
