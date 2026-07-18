package application

import "regexp"

var secretReferencePattern = regexp.MustCompile(`^(env:[A-Za-z_][A-Za-z0-9_]*|vault:[a-f0-9]{32})$`)

func validSecretReference(reference string) bool {
	return secretReferencePattern.MatchString(reference)
}
