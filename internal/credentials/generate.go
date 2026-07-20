package credentials

import (
	"crypto/rand"
	"errors"
	"math/big"
)

// DefaultGeneratedPasswordLength is the length of a dsmctl-generated admin
// password. 24 characters of the mixed alphabet below is comfortably above
// DSM's strong-password policy and any practical brute-force reach.
const DefaultGeneratedPasswordLength = 24

// The generation alphabet deliberately omits visually ambiguous characters
// (l/1/I, o/0) so a password read off a screen or clipboard is transcribable.
const (
	genLower   = "abcdefghijkmnpqrstuvwxyz" // no l, no o
	genUpper   = "ABCDEFGHJKLMNPQRSTUVWXYZ" // no I, no O
	genDigits  = "23456789"                 // no 0, no 1
	genSymbols = "!@#$%^&*()-_=+[]{}"
)

// GeneratePassword returns a cryptographically random password of the given
// length that always contains at least one lower-case letter, one upper-case
// letter, one digit, and one symbol, drawn from an unambiguous alphabet. It is
// the source of a dsmctl-generated admin password: the plaintext exists only in
// the returned string, which the caller stores in the OS keyring and never
// prints. Lengths below 16 are rejected so a caller cannot accidentally
// provision a guessable administrator.
func GeneratePassword(length int) (string, error) {
	if length < 16 {
		return "", errors.New("generated password length must be at least 16")
	}
	classes := []string{genLower, genUpper, genDigits, genSymbols}
	all := genLower + genUpper + genDigits + genSymbols

	out := make([]byte, 0, length)
	// Guarantee one character from each class, then fill from the full alphabet.
	for _, class := range classes {
		c, err := randomChar(class)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}
	for len(out) < length {
		c, err := randomChar(all)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}
	// Shuffle so the guaranteed class characters are not always in front.
	if err := shuffle(out); err != nil {
		return "", err
	}
	return string(out), nil
}

func randomChar(alphabet string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
	if err != nil {
		return 0, err
	}
	return alphabet[n.Int64()], nil
}

// shuffle performs a crypto/rand Fisher–Yates shuffle in place.
func shuffle(b []byte) error {
	for i := len(b) - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return err
		}
		j := int(jBig.Int64())
		b[i], b[j] = b[j], b[i]
	}
	return nil
}
