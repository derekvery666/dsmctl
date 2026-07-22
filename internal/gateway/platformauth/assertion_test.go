package platformauth

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestAssertionRoundTripAndReplayDenial(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	key := bytes.Repeat([]byte{7}, 32)
	signer, err := newSigner(key, DefaultAudience, 30*time.Second, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := newVerifier(key, DefaultAudience, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	assertion, err := signer.Sign("dsm-admin")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := verifier.Verify(context.Background(), assertion)
	if err != nil || identity.Subject != "dsm-admin" {
		t.Fatalf("identity = %#v, err = %v", identity, err)
	}
	if _, err := verifier.Verify(context.Background(), assertion); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("replay error = %v", err)
	}
}

func TestAssertionFailsClosed(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	key := bytes.Repeat([]byte{9}, 32)
	signer, _ := newSigner(key, DefaultAudience, time.Second, func() time.Time { return now })
	assertion, _ := signer.Sign("admin")

	wrongAudience, _ := newVerifier(key, "other", func() time.Time { return now })
	if _, err := wrongAudience.Verify(context.Background(), assertion); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("wrong-audience error = %v", err)
	}
	expired, _ := newVerifier(key, DefaultAudience, func() time.Time { return now.Add(2 * time.Second) })
	if _, err := expired.Verify(context.Background(), assertion); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expired error = %v", err)
	}
	for _, subject := range []string{"", "bad\nname"} {
		if _, err := signer.Sign(subject); err == nil {
			t.Fatalf("subject %q was accepted", subject)
		}
	}
}

func TestReadKeyRequiresExactLength(t *testing.T) {
	path := t.TempDir() + "/key"
	if err := os.WriteFile(path, bytes.Repeat([]byte{1}, 31), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadKey(path); err == nil {
		t.Fatal("short key was accepted")
	}
}
