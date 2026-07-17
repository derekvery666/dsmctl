package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/config"
	"github.com/ychiu1211/dsmctl/internal/credentials"
	"github.com/ychiu1211/dsmctl/internal/runtime"
)

type fakeCredentialStore struct {
	passwords map[string]bool
	devices   map[string]bool
	envSet    map[string]bool
	sessions  map[string]credentials.SessionMeta
	probeErr  error
}

func (store *fakeCredentialStore) HasPassword(_ context.Context, profileName string) (bool, error) {
	if store.probeErr != nil {
		return false, store.probeErr
	}
	return store.passwords[profileName], nil
}

func (store *fakeCredentialStore) HasTrustedDevice(_ context.Context, profileName string) (bool, error) {
	if store.probeErr != nil {
		return false, store.probeErr
	}
	return store.devices[profileName], nil
}

func (store *fakeCredentialStore) PasswordEnvironment(profileName string, profile config.Profile) (string, bool) {
	name := profile.PasswordEnv
	if name == "" {
		name = credentials.DefaultEnvironmentVariable(profileName)
	}
	return name, store.envSet[name]
}

func (store *fakeCredentialStore) SessionMeta(_ context.Context, profileName string) (credentials.SessionMeta, error) {
	return store.sessions[profileName], nil
}

func credentialTestService(store CredentialStore) *Service {
	cfg := config.New()
	cfg.DefaultNAS = "office"
	cfg.NAS["office"] = config.Profile{URL: "https://office.example:5001", Username: "admin"}
	cfg.NAS["lab"] = config.Profile{URL: "https://lab.example:5001", Username: "admin", PasswordEnv: "LAB_PASSWORD"}
	manager := runtime.NewManager(cfg, credentials.NewEnvironment())
	return NewService(cfg, manager, WithCredentialStore(store))
}

func TestGetAuthStatusReportsAllProfilesWithoutSecrets(t *testing.T) {
	store := &fakeCredentialStore{
		passwords: map[string]bool{"office": true},
		devices:   map[string]bool{"office": true},
		envSet:    map[string]bool{"LAB_PASSWORD": true},
	}
	service := credentialTestService(store)

	result, err := service.GetAuthStatus(context.Background(), "")
	if err != nil {
		t.Fatalf("GetAuthStatus() error = %v", err)
	}
	if len(result.Statuses) != 2 {
		t.Fatalf("statuses = %#v", result.Statuses)
	}
	lab, office := result.Statuses[0], result.Statuses[1]
	if lab.NAS != "lab" || office.NAS != "office" {
		t.Fatalf("status order = %q, %q", lab.NAS, office.NAS)
	}
	if !office.Default || !office.PasswordStored || !office.TrustedDeviceStored || office.PasswordEnv != "DSMCTL_PASSWORD_OFFICE" || office.PasswordEnvSet {
		t.Fatalf("office status = %#v", office)
	}
	if lab.Default || lab.PasswordStored || lab.TrustedDeviceStored || lab.PasswordEnv != "LAB_PASSWORD" || !lab.PasswordEnvSet {
		t.Fatalf("lab status = %#v", lab)
	}
	if office.ClientCached || office.SessionHeld {
		t.Fatalf("office session state = %#v", office)
	}
}

func TestGetAuthStatusFiltersToOneProfile(t *testing.T) {
	service := credentialTestService(&fakeCredentialStore{})
	result, err := service.GetAuthStatus(context.Background(), "lab")
	if err != nil {
		t.Fatalf("GetAuthStatus(lab) error = %v", err)
	}
	if len(result.Statuses) != 1 || result.Statuses[0].NAS != "lab" {
		t.Fatalf("statuses = %#v", result.Statuses)
	}
	if _, err := service.GetAuthStatus(context.Background(), "missing"); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("GetAuthStatus(missing) error = %v", err)
	}
}

func TestGetAuthStatusSurfacesStoreErrorPerProfile(t *testing.T) {
	service := credentialTestService(&fakeCredentialStore{probeErr: errors.New("keychain is locked")})
	result, err := service.GetAuthStatus(context.Background(), "")
	if err != nil {
		t.Fatalf("GetAuthStatus() error = %v", err)
	}
	for _, status := range result.Statuses {
		if !strings.Contains(status.StoreError, "keychain is locked") {
			t.Fatalf("status = %#v", status)
		}
		if status.PasswordStored || status.TrustedDeviceStored {
			t.Fatalf("status with probe error claimed stored credentials: %#v", status)
		}
	}
}

func TestGetAuthStatusRequiresStore(t *testing.T) {
	cfg := config.New()
	manager := runtime.NewManager(cfg, credentials.NewEnvironment())
	service := NewService(cfg, manager)
	if _, err := service.GetAuthStatus(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "credential store") {
		t.Fatalf("GetAuthStatus() error = %v", err)
	}
}
