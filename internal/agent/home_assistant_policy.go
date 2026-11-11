package agent

import (
	"aurago/internal/tools"
	"strings"
)

func homeAssistantServiceGate(domain, service string, allowedServices, blockedServices []string) string {
	return tools.HAServiceRefusal(domain, service, allowedServices, blockedServices)
}

func normalizeHomeAssistantService(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
