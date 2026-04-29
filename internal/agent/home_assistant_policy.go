package agent

import "strings"

func homeAssistantServiceGate(domain, service string, allowedServices, blockedServices []string) string {
	full := strings.ToLower(strings.TrimSpace(domain) + "." + strings.TrimSpace(service))
	if strings.Trim(full, ".") == "" {
		return ""
	}
	for _, blocked := range blockedServices {
		if normalizeHomeAssistantService(blocked) == full {
			return "Home Assistant service " + full + " is blocked by home_assistant.blocked_services"
		}
	}
	if len(allowedServices) == 0 {
		return ""
	}
	for _, allowed := range allowedServices {
		if normalizeHomeAssistantService(allowed) == full {
			return ""
		}
	}
	return "Home Assistant service " + full + " is not allowed by home_assistant.allowed_services"
}

func normalizeHomeAssistantService(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
