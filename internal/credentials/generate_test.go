package credentials

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestGeneratePasswordMeetsPolicyAndLength(t *testing.T) {
	for i := 0; i < 200; i++ {
		pw, err := GeneratePassword(DefaultGeneratedPasswordLength)
		if err != nil {
			t.Fatalf("GeneratePassword error = %v", err)
		}
		if len(pw) != DefaultGeneratedPasswordLength {
			t.Fatalf("length = %d, want %d", len(pw), DefaultGeneratedPasswordLength)
		}
		if !strings.ContainsAny(pw, genLower) || !strings.ContainsAny(pw, genUpper) ||
			!strings.ContainsAny(pw, genDigits) || !strings.ContainsAny(pw, genSymbols) {
			t.Fatalf("password %q is missing a required character class", pw)
		}
		if strings.ContainsAny(pw, "lo1O0I") {
			t.Fatalf("password %q contains an ambiguous character", pw)
		}
	}
}

func TestGeneratePasswordRejectsShort(t *testing.T) {
	if _, err := GeneratePassword(8); err == nil {
		t.Fatal("GeneratePassword(8) error = nil, want rejection")
	}
}

func TestGeneratePasswordIsRandom(t *testing.T) {
	a, err := GeneratePassword(24)
	if err != nil {
		t.Fatalf("GeneratePassword error = %v", err)
	}
	b, err := GeneratePassword(24)
	if err != nil {
		t.Fatalf("GeneratePassword error = %v", err)
	}
	if a == b {
		t.Fatal("two generated passwords were identical")
	}
}

func TestMemoryReferenceResolverRoundTripAndForget(t *testing.T) {
	r := NewMemoryReferenceResolver()
	ref := r.Put("s3cr3t")
	if !strings.HasPrefix(ref, "mem:") {
		t.Fatalf("Put() = %q, want mem: prefix", ref)
	}
	got, err := r.ResolveSecret(context.Background(), ref)
	if err != nil || got != "s3cr3t" {
		t.Fatalf("ResolveSecret() = %q, %v", got, err)
	}
	r.Forget(ref)
	if _, err := r.ResolveSecret(context.Background(), ref); err == nil {
		t.Fatal("ResolveSecret() after Forget error = nil, want error")
	}
	if _, err := r.ResolveSecret(context.Background(), "env:FOO"); err == nil {
		t.Fatal("ResolveSecret(env:) error = nil, want scheme rejection")
	}
}

func TestChainReferenceResolverPrefersMatchingScheme(t *testing.T) {
	mem := NewMemoryReferenceResolver()
	ref := mem.Put("value")
	chain := ChainReferenceResolver(mem, NewEnvironmentReferenceResolver())
	got, err := chain.ResolveSecret(context.Background(), ref)
	if err != nil || got != "value" {
		t.Fatalf("chain(mem) = %q, %v", got, err)
	}
	if _, err := chain.ResolveSecret(context.Background(), "mem:absent"); err == nil {
		t.Fatal("chain(absent) error = nil, want error")
	}
}

func TestRevealPasswordIsKeyringOnly(t *testing.T) {
	backend := newMemoryKeyring()
	backend.values[keyringService+":"+passwordKey("office")] = "stored-pw"
	store := &SecureStore{keyring: backend, environment: &Environment{lookup: func(string) (string, bool) {
		return "env-pw", true // must be ignored: reveal never consults the environment fallback
	}}}
	got, err := store.RevealPassword(context.Background(), "office")
	if err != nil || got != "stored-pw" {
		t.Fatalf("RevealPassword() = %q, %v", got, err)
	}
	if _, err := store.RevealPassword(context.Background(), "absent"); !errors.Is(err, ErrNoStoredPassword) {
		t.Fatalf("RevealPassword(absent) error = %v, want ErrNoStoredPassword", err)
	}
}
