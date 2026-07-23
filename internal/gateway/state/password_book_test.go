package state

import (
	"context"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/derekvery666/dsmctl/internal/config"
	"github.com/derekvery666/dsmctl/internal/credentials"
)

func TestPasswordBookMultiAccount(t *testing.T) {
	repo, _ := openTestRepository(t)
	ctx := context.Background()
	profile, err := repo.CreateProfile(ctx, ProfileInput{Name: "nas", URL: "https://nas.example:5001"})
	if err != nil {
		t.Fatal(err)
	}

	// Primary enrollment sets the login and stores the primary secret.
	rev, err := repo.SavePasswordForAccount(ctx, "nas", profile.Revision, "admin", "admin-pw", credentials.TrustedDevice{})
	if err != nil {
		t.Fatal(err)
	}
	// The runtime resolver (unchanged Password path) returns the primary.
	if pw, err := repo.Password(ctx, "nas", config.Profile{}); err != nil || pw != "admin-pw" {
		t.Fatalf("primary Password = %q, err=%v", pw, err)
	}
	accounts, err := repo.PasswordAccounts(ctx, "nas")
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || !accounts[0].Primary || accounts[0].Account != "admin" {
		t.Fatalf("initial accounts = %#v", accounts)
	}

	// A second account is stored without disturbing the primary login.
	rev, err = repo.SavePasswordForAccount(ctx, "nas", rev, "backup", "backup-pw", credentials.TrustedDevice{})
	if err != nil {
		t.Fatal(err)
	}
	if pw, _ := repo.Password(ctx, "nas", config.Profile{}); pw != "admin-pw" {
		t.Fatalf("primary changed after secondary add: %q", pw)
	}
	profileAfter, _ := repo.Profile(ctx, "nas")
	if profileAfter.Username != "admin" {
		t.Fatalf("secondary add changed the login account to %q", profileAfter.Username)
	}

	// Every account reveals its own secret; empty and case-insensitive selectors
	// resolve as expected.
	for account, want := range map[string]string{"admin": "admin-pw", "backup": "backup-pw", "": "admin-pw", "BACKUP": "backup-pw"} {
		if pw, err := repo.RevealPasswordForAccount(ctx, "nas", account); err != nil || pw != want {
			t.Fatalf("reveal %q = %q, err=%v (want %q)", account, pw, err, want)
		}
	}

	accounts, _ = repo.PasswordAccounts(ctx, "nas")
	if len(accounts) != 2 || !accounts[0].Primary || accounts[0].Account != "admin" || accounts[1].Primary || accounts[1].Account != "backup" {
		t.Fatalf("two-account book = %#v", accounts)
	}

	// Deleting a secondary leaves the primary intact.
	removed, _, err := repo.DeletePasswordForAccount(ctx, "nas", "backup")
	if err != nil || !removed {
		t.Fatalf("delete secondary removed=%v err=%v", removed, err)
	}
	if pw, _ := repo.Password(ctx, "nas", config.Profile{}); pw != "admin-pw" {
		t.Fatalf("primary lost after secondary delete: %q", pw)
	}
	if accounts, _ = repo.PasswordAccounts(ctx, "nas"); len(accounts) != 1 || accounts[0].Account != "admin" {
		t.Fatalf("post-delete book = %#v", accounts)
	}
	_ = rev
}

func TestSavePrimaryPasswordAndSessionIsAtomic(t *testing.T) {
	repo, _ := openTestRepository(t)
	ctx := context.Background()
	profile, err := repo.CreateProfile(ctx, ProfileInput{Name: "nas", URL: "https://nas.example:5001"})
	if err != nil {
		t.Fatal(err)
	}
	wantSession := credentials.SessionCredential{
		SID: "password-sid", SynoToken: "token", Account: "admin",
		IssuedAt: time.Now().UTC(), LastVerified: time.Now().UTC(),
	}
	revision, err := repo.SavePasswordForAccountWithSession(ctx, "nas", profile.Revision, "admin", "admin-pw", credentials.TrustedDevice{}, wantSession)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := repo.Session(ctx, "nas")
	if err != nil {
		t.Fatal(err)
	}
	if stored.SID != wantSession.SID || stored.SynoToken != wantSession.SynoToken || stored.Account != "admin" {
		t.Fatalf("stored session = %#v", stored)
	}
	if state, err := repo.Profile(ctx, "nas"); err != nil || !state.PasswordStored || !state.SessionStored || state.Username != "admin" {
		t.Fatalf("profile after atomic enrollment = %#v, err=%v", state, err)
	}

	if _, err := repo.SavePasswordForAccountWithSession(ctx, "nas", revision, "backup", "backup-pw", credentials.TrustedDevice{}, credentials.SessionCredential{SID: "wrong-sid", Account: "backup"}); err == nil {
		t.Fatal("secondary password unexpectedly accepted a runtime session")
	}
	after, err := repo.Profile(ctx, "nas")
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != revision {
		t.Fatalf("rejected secondary session advanced revision to %d, want %d", after.Revision, revision)
	}
	if _, err := repo.RevealPasswordForAccount(ctx, "nas", "backup"); err == nil {
		t.Fatal("rejected secondary session still stored its password")
	}
}

// A password stored before account labels existed (empty Account metadata) still
// appears in the book as the primary account, reporting the profile login.
func TestPasswordBookLegacyPrimaryFallsBackToUsername(t *testing.T) {
	repo, _ := openTestRepository(t)
	ctx := context.Background()
	if _, err := repo.CreateProfile(ctx, ProfileInput{Name: "legacy", URL: "https://legacy.example:5001"}); err != nil {
		t.Fatal(err)
	}
	// Simulate a pre-label secret: username set, password secret carries no label.
	if err := repo.db.Update(func(tx *bolt.Tx) error {
		record, err := readProfile(tx, "legacy")
		if err != nil {
			return err
		}
		record.Username = "operator"
		id, err := repo.putSecret(tx, &record, secretPassword, []byte("legacy-pw"), "", "")
		if err != nil {
			return err
		}
		record.PasswordSecretID = id
		record.UpdatedAt = time.Now().UTC()
		return putProfile(tx, record)
	}); err != nil {
		t.Fatal(err)
	}
	accounts, err := repo.PasswordAccounts(ctx, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || !accounts[0].Primary || accounts[0].Account != "operator" {
		t.Fatalf("legacy book = %#v", accounts)
	}
	if pw, err := repo.RevealPasswordForAccount(ctx, "legacy", "operator"); err != nil || pw != "legacy-pw" {
		t.Fatalf("legacy reveal by username = %q, err=%v", pw, err)
	}
}

func TestProfileRoleTargetPersistsAndStaysManagedByDefault(t *testing.T) {
	repo, _ := openTestRepository(t)
	ctx := context.Background()

	created, err := repo.CreateProfile(ctx, ProfileInput{Name: "dst", URL: "https://dst.example:5001", Role: config.ProfileRoleTarget})
	if err != nil {
		t.Fatal(err)
	}
	if created.Role != config.ProfileRoleTarget {
		t.Fatalf("created role = %q", created.Role)
	}
	managed, err := repo.CreateProfile(ctx, ProfileInput{Name: "src", URL: "https://src.example:5001"})
	if err != nil {
		t.Fatal(err)
	}
	if managed.Role != "" {
		t.Fatalf("managed role = %q, want empty default", managed.Role)
	}

	// Snapshot carries the role into the config profile the runtime resolves.
	cfg, err := repo.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if p := cfg.NAS["dst"]; p.Role != config.ProfileRoleTarget || p.Managed() {
		t.Fatalf("dst snapshot role=%q managed=%v", p.Role, p.Managed())
	}
	if p := cfg.NAS["src"]; p.Role != "" || !p.Managed() {
		t.Fatalf("src snapshot role=%q managed=%v", p.Role, p.Managed())
	}

	// The standard edit path leaves the role unchanged (role is set at creation).
	updated, err := repo.UpdateProfile(ctx, "dst", created.Revision, ProfileInput{Name: "dst", URL: "https://dst2.example:5001"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Role != config.ProfileRoleTarget {
		t.Fatalf("role flipped on connection edit: %q", updated.Role)
	}

	// An unknown role is rejected at creation.
	if _, err := repo.CreateProfile(ctx, ProfileInput{Name: "bogus", URL: "https://b.example:5001", Role: "sometimes"}); err == nil {
		t.Fatal("expected an error for an unknown role")
	}
}
