package server

import (
	"fmt"
	"regexp"
	"strings"
)

// providerIDPattern allows lowercase letters, digits, dots, hyphens, and
// underscores. Dots are intentional (model-style IDs like "llama-3.1-8b" or
// "crof.ai"). Display name and model fields are unrestricted separately.
var providerIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

func isValidProviderID(id string) bool {
	return providerIDPattern.MatchString(strings.TrimSpace(id))
}

func validateProviderID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("Provider ID must not be empty")
	}
	if !isValidProviderID(id) {
		return fmt.Errorf("Provider ID %q is invalid; use lowercase letters, numbers, dots, hyphens, or underscores, starting with a letter or number", id)
	}
	return nil
}

// validateProviderIDForSave enforces the ID pattern for newly introduced
// provider IDs. Existing config entries that predate validation (e.g. "EdenAi"
// with uppercase) may be re-saved unchanged so a single legacy ID does not
// block the entire provider list PUT.
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
