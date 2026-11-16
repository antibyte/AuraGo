package config

import (
	"fmt"
	"regexp"
)

var telnyxOwnerNumberPattern = regexp.MustCompile(`^\+[1-9][0-9]{1,14}$`)

// ValidateTelnyxOwner never infers owner privileges from the sender allowlist.
func ValidateTelnyxOwner(cfg *Config) error {
	owner := cfg.Telnyx.OwnerNumber
	if owner == "" {
		return nil
	}
	if !telnyxOwnerNumberPattern.MatchString(owner) {
		return fmt.Errorf("telnyx.owner_number must be an E.164 number")
	}
	for _, allowed := range cfg.Telnyx.AllowedNumbers {
		if allowed == owner {
			return nil
		}
	}
	return fmt.Errorf("telnyx.owner_number must also be listed in telnyx.allowed_numbers")
}
