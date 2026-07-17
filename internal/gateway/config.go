package gateway

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/ychiu1211/dsmctl/internal/config"
)

const (
	MaxProfiles       = 32
	MaxTimeoutSeconds = 120
	MinTokenBytes     = 32
)

// ValidateConfig applies daemon-specific fleet bounds after shared config
// validation.
func ValidateConfig(cfg *config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if len(cfg.NAS) > MaxProfiles {
		return fmt.Errorf("gateway supports at most %d NAS profiles", MaxProfiles)
	}
	for name, profile := range cfg.NAS {
		if profile.TimeoutSeconds > MaxTimeoutSeconds {
			return fmt.Errorf("NAS profile %q: timeout_seconds exceeds gateway maximum %d", name, MaxTimeoutSeconds)
		}
	}
	return nil
}

// ReadDevelopmentToken reads and validates the explicit WI-014 read-only
// credential. Persistent and scoped remote tokens belong to WI-016.
func ReadDevelopmentToken(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("development read-only token file is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read development token file: %w", err)
	}
	if len(data) > 4096 {
		return "", errors.New("development token file is too large")
	}
	token := strings.TrimSpace(string(data))
	if err := ValidateDevelopmentToken(token); err != nil {
		return "", err
	}
	return token, nil
}

func ValidateDevelopmentToken(token string) error {
	if len(token) < MinTokenBytes {
		return fmt.Errorf("development token must be at least %d bytes", MinTokenBytes)
	}
	if strings.IndexFunc(token, unicode.IsSpace) >= 0 {
		return errors.New("development token must not contain whitespace")
	}
	return nil
}

// DevelopmentTokenDigest avoids retaining another plaintext copy in
// readiness callbacks.
func DevelopmentTokenDigest(token string) [sha256.Size]byte {
	return sha256.Sum256([]byte(token))
}

func DevelopmentTokenMatches(expected [sha256.Size]byte, token string) bool {
	actual := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(expected[:], actual[:]) == 1
}
