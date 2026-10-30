package agent

import "aurago/internal/decisions/consumers"

// ConsumersFrom returns the consumer view of a decision service handed to a
// bot, or nil when the service does not offer one.
func ConsumersFrom(s TaskDecisionService) consumers.Service {
	if cs, ok := s.(consumers.Service); ok {
		return cs
	}
	return nil
}
