package server

import (
	"fmt"
	"regexp"
	"strings"
)

var providerIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

func validateProviderID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("Provider ID must not be empty")
	}
	if !providerIDPattern.MatchString(id) {
		return fmt.Errorf("Provider ID %q is invalid; use lowercase letters, numbers, hyphens, or underscores, starting with a letter or number", id)
	}
	return nil
}
