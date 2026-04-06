package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"
)

// MDNSService represents a discovered MDNS service.
type MDNSService struct {
	Name string   `json:"name"`
	Host string   `json:"host"`
	IPs  []string `json:"ips"`
	Port int      `json:"port"`
	Info string   `json:"info"`
}

// MDNSScan scans the local network for a specific MDNS service type.
// If serviceType is empty, it queries for a generic list (though specific types usually work better).
func MDNSScan(logger *slog.Logger, serviceType string, timeout int) string {
	if timeout <= 0 {
		timeout = 5
	}
	if serviceType == "" {
		serviceType = "_services._dns-sd._udp" // Default fallback to discover services
	}
	// Validate service type: only allow safe label characters to prevent injection.
	validServiceType := regexp.MustCompile(`^[a-zA-Z0-9_\-]+(\.[a-zA-Z0-9_\-]+)*$`)
	base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(serviceType, "local."), "local"), ".")
	if !strings.Contains(base, "_dns-sd") && !validServiceType.MatchString(strings.ReplaceAll(base, ".", "-")) {
		return `{"status":"error","message":"invalid service type"}`
	}
	// Add .local. if it's missing (hashicorp/mdns usually appends it or needs it based on context, but let's be safe if they only provide prefix)
	if !strings.HasSuffix(serviceType, "local.") && !strings.HasSuffix(serviceType, "local") && !strings.Contains(serviceType, "_dns-sd") {
		if !strings.HasSuffix(serviceType, ".") {
			serviceType = serviceType + "."
		}
		serviceType = serviceType + "local."
	}

	logger.Info("Starting MDNS scan", "service", serviceType, "timeout_seconds", timeout)

	entries, err := mdnsQueryServices(serviceType, time.Duration(timeout)*time.Second)
	if err != nil {
		logger.Error("MDNS scan failed", "error", err)
		return fmt.Sprintf(`{"status": "error", "message": "MDNS scan failed: %v"}`, err)
	}

	services := make([]MDNSService, 0, len(entries))
	for _, e := range entries {
		services = append(services, MDNSService{
			Name: e.Name,
			Host: e.Host,
			IPs:  e.IPs,
			Port: e.Port,
			Info: strings.Join(e.TXTs, ", "),
		})
	}

	if len(services) == 0 {
		return fmt.Sprintf(`{"status": "success", "message": "No %s devices found"}`, serviceType)
	}

	b, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"count":   len(services),
		"devices": services,
	})
	return string(b)
}
