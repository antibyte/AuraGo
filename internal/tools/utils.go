// Package tools provides utility functions for the AuraGo agent tool system.
//
// This file (utils.go) serves as a collection point for helper functions that are used
// across multiple tool implementations (truncateUTF8Safe: used by ansible.go and email.go
// for safe UTF-8 string truncation without breaking multi-byte characters).
//
// Headless browsers that open untrusted URLs are never built here or with
// launcher.New(): use security.LaunchEgressBrowser (see AGENTS.md).
package tools

import (
	"unicode/utf8"
)

// truncateUTF8Safe truncates a string to a given maximum byte length
// without breaking UTF-8 multi-byte characters.
func truncateUTF8Safe(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}

	// We know it exceeds maxBytes. We just iterate backwards from maxBytes
	// to find a valid utf8 boundary.
	for i := maxBytes; i >= 0; i-- {
		if utf8.RuneStart(s[i]) {
			// If we stop at a rune start, we just need to ensure the rune fits
			// inside maxBytes.
			// Actually, if we just slice up to i, it cleanly lops off the
			// overflowing rune.
			return s[:i]
		}
	}

	return ""
}
