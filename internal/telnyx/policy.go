package telnyx

import (
	"fmt"
	"log/slog"

	"aurago/internal/config"
)

type numberPolicy struct {
	allowed  []string
	readOnly bool
}

func newConfiguredClient(cfg *config.Config, logger *slog.Logger) *Client {
	c := NewClient(cfg.Telnyx.APIKey, logger)
	c.policy = &numberPolicy{allowed: append([]string(nil), cfg.Telnyx.AllowedNumbers...), readOnly: cfg.Telnyx.ReadOnly || !cfg.Telnyx.Enabled}
	return c
}

func allowedNumber(number string, allowed []string) bool {
	clean := normalizePhone(number)
	if ValidateE164(clean) != nil {
		return false
	}
	for _, candidate := range allowed {
		if clean == normalizePhone(candidate) {
			return true
		}
	}
	return false
}

func (c *Client) validateDestination(number string) error {
	if err := ValidateE164(number); err != nil {
		return err
	}
	if c.policy != nil && !allowedNumber(number, c.policy.allowed) {
		return fmt.Errorf("destination is not in telnyx.allowed_numbers")
	}
	return nil
}
