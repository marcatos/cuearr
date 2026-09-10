package auth

import "crypto/subtle"

// ValidAPIKey reports whether provided matches expected using constant-time comparison.
func ValidAPIKey(provided, expected string) bool {
	if expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
