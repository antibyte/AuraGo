package telnyx

import (
	"errors"
	"fmt"
	"log/slog"

	"aurago/internal/config"
)

type numberPolicy struct {
	allowed  []string
	readOnly bool
}

// errNoNumberPolicy is returned by every mutation of a client built with
// NewClient: without telnyx.allowed_numbers and telnyx.read_only it may only read.
var errNoNumberPolicy = errors.New("telnyx client has no number policy; use the configured client")

// NewConfiguredClient returns a client bound to cfg's number policy: SMS, MMS,
// calls and transfers only reach telnyx.allowed_numbers, and read-only mode
// (or a disabled integration) refuses every mutation. Use it for anything
// that sends or calls; NewClient is for read-only account queries.
func NewConfiguredClient(cfg *config.Config, logger *slog.Logger) *Client {
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
	if c.policy == nil {
		return errNoNumberPolicy
	}
	if err := ValidateE164(number); err != nil {
		return err
	}
	if !allowedNumber(number, c.policy.allowed) {
		return fmt.Errorf("destination is not in telnyx.allowed_numbers")
	}
	return nil
}
