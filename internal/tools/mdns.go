package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
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

	// Channel to receive responses
	entriesCh := make(chan *mdns.ServiceEntry, 50)
	var (
		services   []MDNSService
		servicesMu sync.Mutex
	)

	// Start a goroutine to process entries
	go func() {
		for entry := range entriesCh {
			var ips []string
			if entry.AddrV4 != nil {
				ips = append(ips, entry.AddrV4.String())
			}
			if entry.AddrV6 != nil {
				ips = append(ips, entry.AddrV6.String())
			}

			servicesMu.Lock()
			services = append(services, MDNSService{
				Name: entry.Name,
				Host: entry.Host,
				IPs:  ips,
				Port: entry.Port,
				Info: strings.Join(entry.InfoFields, ", "),
			})
			servicesMu.Unlock()
		}
	}()

	params := mdns.DefaultParams(serviceType)
	params.Entries = entriesCh
	params.Timeout = time.Duration(timeout) * time.Second

	err := mdns.Query(params)
	close(entriesCh) // Ensure channel is closed after query finishes

	if err != nil {
		logger.Error("MDNS scan failed", "error", err)
		return fmt.Sprintf(`{"status": "error", "message": "MDNS scan failed: %v"}`, err)
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
