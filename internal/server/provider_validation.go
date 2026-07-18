package server

import (
	"fmt"
	"regexp"
	"strings"
)

var providerIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

func isValidProviderID(id string) bool {
	return providerIDPattern.MatchString(strings.TrimSpace(id))
}

func validateProviderID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("Provider ID must not be empty")
	}
	if !isValidProviderID(id) {
		return fmt.Errorf("Provider ID %q is invalid; use lowercase letters, numbers, hyphens, or underscores, starting with a letter or number", id)
	}
	return nil
}

// validateProviderIDForSave enforces the strict ID pattern for newly introduced
// provider IDs. Existing config entries that predate validation (e.g. "llama-3.1-8b"
// or "crof.ai") may be re-saved unchanged so a single legacy ID does not block
// the entire provider list PUT.
func validateProviderIDForSave(id string, existingIDs map[string]bool) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("Provider ID must not be empty")
	}
	if existingIDs[id] {
		return nil
	}
	return validateProviderID(id)
}
