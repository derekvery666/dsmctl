package credentials

import (
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
