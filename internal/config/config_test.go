package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreRoundTripAndResolve(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "nested", "config.json"))
	cfg := New()
	cfg.DefaultNAS = "office"
	cfg.NAS["office"] = Profile{
		URL:         "https://nas.example.test:5001",
		Username:    "automation",
		PasswordEnv: "OFFICE_PASSWORD",
	}
	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	name, profile, err := loaded.Resolve("")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if name != "office" || profile.Username != "automation" {
		t.Fatalf("Resolve() = %q, %#v", name, profile)
	}

	loaded.NAS["lab"] = Profile{URL: "https://lab.example.test:5001", Username: "lab-user"}
	if err := store.Save(loaded); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
}

// An empty configuration cannot be repaired with 'nas use', which only selects
// among profiles that already exist. The first NAS has to be added.
func TestResolveWithoutProfilesPointsToNASAdd(t *testing.T) {
	cfg := New()
	_, _, err := cfg.Resolve("")
	if err == nil {
		t.Fatal("Resolve() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "nas add") {
		t.Fatalf("Resolve() error = %q, want it to point at 'nas add'", err)
	}
}

func TestResolveWithoutDefaultPointsToNASUse(t *testing.T) {
	cfg := New()
	cfg.NAS["office"] = Profile{URL: "https://office.example.test:5001"}
	cfg.NAS["lab"] = Profile{URL: "https://lab.example.test:5001"}
	_, _, err := cfg.Resolve("")
	if err == nil {
		t.Fatal("Resolve() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "nas use") {
		t.Fatalf("Resolve() error = %q, want it to point at 'nas use'", err)
	}
}

func TestResolveUsesOnlyProfile(t *testing.T) {
	cfg := New()
	cfg.NAS["lab"] = Profile{URL: "http://127.0.0.1:5000", Username: "test"}
	name, _, err := cfg.Resolve("")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if name != "lab" {
		t.Fatalf("Resolve() name = %q, want lab", name)
	}
}
