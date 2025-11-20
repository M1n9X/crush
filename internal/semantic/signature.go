package semantic

import "strings"

// normalizeSignature trims whitespace and lowercases to allow loose matching.
func normalizeSignature(sig string) string {
	return strings.ToLower(strings.TrimSpace(sig))
}

// signatureMatches allows matching patterns that include "(" against the detailed signature string.
func signatureMatches(pattern, signature string) bool {
	if pattern == "" || signature == "" {
		return false
	}
	p := normalizeSignature(pattern)
	s := normalizeSignature(signature)
	return strings.Contains(s, p)
}
