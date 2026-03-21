package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// haHTTPClient is a shared HTTP client for Home Assistant API calls.
var haHTTPClient = &http.Client{Timeout: 30 * time.Second}

// HAConfig holds the Home Assistant connection parameters.
type HAConfig struct {
	URL         string
	AccessToken string
}

// haRequest performs a generic HTTP request against the HA REST API.
func haRequest(cfg HAConfig, method, endpoint string, body string) ([]byte, int, error) {
	url := strings.TrimRight(cfg.URL, "/") + endpoint

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := haHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

// HAGetStates retrieves entity states, optionally filtered by domain prefix.
func HAGetStates(cfg HAConfig, domain string) string {
	data, code, err := haRequest(cfg, "GET", "/api/states", "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to fetch states: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	// Parse and optionally filter by domain
	var states []map[string]interface{}
	if err := json.Unmarshal(data, &states); err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to parse states: %v"}`, err)
	}

	if domain != "" {
		prefix := domain + "."
		var filtered []map[string]interface{}
		for _, s := range states {
			if eid, ok := s["entity_id"].(string); ok && strings.HasPrefix(eid, prefix) {
				filtered = append(filtered, s)
			}
		}
		states = filtered
	}

	// Compact output: entity_id, state, friendly_name, device_class, unit_of_measurement
	type compactState struct {
		EntityID          string `json:"entity_id"`
		State             string `json:"state"`
		Name              string `json:"friendly_name,omitempty"`
		DeviceClass       string `json:"device_class,omitempty"`
		UnitOfMeasurement string `json:"unit_of_measurement,omitempty"`
	}
	var result []compactState
	for _, s := range states {
		cs := compactState{
			EntityID: fmt.Sprintf("%v", s["entity_id"]),
			State:    fmt.Sprintf("%v", s["state"]),
		}
		if attrs, ok := s["attributes"].(map[string]interface{}); ok {
			if fn, ok := attrs["friendly_name"].(string); ok {
				cs.Name = fn
			}
			if dc, ok := attrs["device_class"].(string); ok {
				cs.DeviceClass = dc
			}
			if uom, ok := attrs["unit_of_measurement"].(string); ok {
				cs.UnitOfMeasurement = uom
			}
		}
		result = append(result, cs)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"count":  len(result),
		"states": result,
	})
	return string(out)
}

// HAGetState retrieves the state for a single entity.
func HAGetState(cfg HAConfig, entityID string) string {
	if entityID == "" {
		return `{"status":"error","message":"'entity_id' is required"}`
	}

	data, code, err := haRequest(cfg, "GET", "/api/states/"+entityID, "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to fetch state: %v"}`, err)
	}
	if code == 404 {
		return fmt.Sprintf(`{"status":"error","message":"Entity '%s' not found"}`, entityID)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	// Return the full entity state
	var entity map[string]interface{}
	if err := json.Unmarshal(data, &entity); err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to parse state: %v"}`, err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"entity": entity,
	})
	return string(out)
}

// HACallService calls a Home Assistant service (e.g. light/turn_on).
func HACallService(cfg HAConfig, domain, service, entityID string, serviceData map[string]interface{}) string {
	if domain == "" || service == "" {
		return `{"status":"error","message":"'domain' and 'service' are required (e.g. domain='light', service='turn_on')"}`
	}

	// Build request body
	payload := make(map[string]interface{})
	if entityID != "" {
		payload["entity_id"] = entityID
	}
	// Merge additional service_data into payload
	for k, v := range serviceData {
		if k != "entity_id" && k != "domain" && k != "service" {
			payload[k] = v
		}
	}

	body, _ := json.Marshal(payload)
	endpoint := fmt.Sprintf("/api/services/%s/%s", domain, service)

	data, code, err := haRequest(cfg, "POST", endpoint, string(body))
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Service call failed: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	// HA returns an array of affected entity states
	var affected []map[string]interface{}
	if err := json.Unmarshal(data, &affected); err != nil {
		return fmt.Sprintf(`{"status":"success","message":"Service %s.%s called successfully","raw_response":%q}`, domain, service, string(data))
	}

	var entityIDs []string
	for _, e := range affected {
		if eid, ok := e["entity_id"].(string); ok {
			entityIDs = append(entityIDs, eid)
		}
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":            "success",
		"service":           domain + "." + service,
		"affected_entities": entityIDs,
		"count":             len(affected),
	})
	return string(out)
}

// HAListServices lists all available services, optionally filtered by domain.
func HAListServices(cfg HAConfig, domain string) string {
	data, code, err := haRequest(cfg, "GET", "/api/services", "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to fetch services: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var services []map[string]interface{}
	if err := json.Unmarshal(data, &services); err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to parse services: %v"}`, err)
	}

	// Filter by domain if specified
	if domain != "" {
		var filtered []map[string]interface{}
		for _, svc := range services {
			if d, ok := svc["domain"].(string); ok && d == domain {
				filtered = append(filtered, svc)
			}
		}
		services = filtered
	}

	// Compact output: domain → list of service names
	type svcEntry struct {
		Domain   string   `json:"domain"`
		Services []string `json:"services"`
	}
	var result []svcEntry
	for _, svc := range services {
		d := fmt.Sprintf("%v", svc["domain"])
		var svcNames []string
		if svcs, ok := svc["services"].(map[string]interface{}); ok {
			for name := range svcs {
				svcNames = append(svcNames, name)
			}
		}
		result = append(result, svcEntry{Domain: d, Services: svcNames})
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":   "success",
		"count":    len(result),
		"services": result,
	})
	return string(out)
}
