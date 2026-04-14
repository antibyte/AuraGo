package outputcompress

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// compressHAOutput routes to the appropriate Home Assistant compressor
// based on the JSON output structure.
func compressHAOutput(output string) (string, string) {
	trimmed := strings.TrimSpace(output)

	// Detect operation type from top-level JSON keys
	hasStates := strings.Contains(trimmed, `"states"`) && strings.Contains(trimmed, `"count"`)
	hasEntity := strings.Contains(trimmed, `"entity"`) && !strings.Contains(trimmed, `"affected_entities"`)
	hasService := strings.Contains(trimmed, `"service"`) && strings.Contains(trimmed, `"affected_entities"`)
	hasServices := strings.Contains(trimmed, `"services"`) && strings.Contains(trimmed, `"domain"`)

	switch {
	case hasStates:
		return compressHAGetStates(output), "ha-states"
	case hasEntity:
		return compressHAGetState(output), "ha-state"
	case hasService:
		return compressHACallService(output), "ha-call-service"
	case hasServices:
		return compressHAListServices(output), "ha-list-services"
	default:
		return compressGeneric(output), "ha-generic"
	}
}

// compressHAGetStates compresses HA get_states output by grouping entities
// by domain and showing state distribution. Unavailable entities are highlighted.
func compressHAGetStates(output string) string {
	var envelope struct {
		Status string                   `json:"status"`
		Count  int                      `json:"count"`
		States []map[string]interface{} `json:"states"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		return compressGeneric(output)
	}

	if envelope.Count == 0 {
		return "HA States: 0 entities\n"
	}

	// Group by domain
	type domainStats struct {
		total       int
		stateCounts map[string]int
		unavailable []string
	}
	domains := make(map[string]*domainStats)
	var domainOrder []string

	for _, s := range envelope.States {
		entityID, _ := s["entity_id"].(string)
		state, _ := s["state"].(string)
		if entityID == "" {
			continue
		}

		parts := strings.SplitN(entityID, ".", 2)
		domain := parts[0]

		ds, exists := domains[domain]
		if !exists {
			ds = &domainStats{stateCounts: make(map[string]int)}
			domains[domain] = ds
			domainOrder = append(domainOrder, domain)
		}
		ds.total++
		ds.stateCounts[state]++

		// Track unavailable entities
		if state == "unavailable" || state == "unknown" {
			name := entityID
			if fn, ok := s["friendly_name"].(string); ok && fn != "" {
				name = fn + " (" + entityID + ")"
			}
			ds.unavailable = append(ds.unavailable, name)
		}
	}

	sort.Strings(domainOrder)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("HA States: %d entities across %d domains\n", envelope.Count, len(domains)))
	sb.WriteString("Domain summary:\n")

	for _, domain := range domainOrder {
		ds := domains[domain]
		// Sort state names for consistency
		var states []string
		for st := range ds.stateCounts {
			states = append(states, st)
		}
		sort.Strings(states)

		var parts []string
		for _, st := range states {
			parts = append(parts, fmt.Sprintf("%d %s", ds.stateCounts[st], st))
		}
		sb.WriteString(fmt.Sprintf("  %s: %s\n", domain, strings.Join(parts, ", ")))
	}

	// Show unavailable entities prominently
	var allUnavailable []string
	for _, domain := range domainOrder {
		ds := domains[domain]
		allUnavailable = append(allUnavailable, ds.unavailable...)
	}

	if len(allUnavailable) > 0 {
		sb.WriteString(fmt.Sprintf("\nUnavailable entities (%d):\n", len(allUnavailable)))
		for _, name := range allUnavailable {
			sb.WriteString(fmt.Sprintf("  ⚠ %s\n", name))
		}
	}

	return sb.String()
}

// haAttrAllowlist defines attributes worth keeping for single entity view.
var haAttrAllowlist = map[string]bool{
	"friendly_name":       true,
	"device_class":        true,
	"unit_of_measurement": true,
	"icon":                true,
	"brightness":          true,
	"color_temp":          true,
	"color_mode":          true,
	"temperature":         true,
	"humidity":            true,
	"pressure":            true,
	"current_temperature": true,
	"target_temperature":  true,
	"fan_mode":            true,
	"hvac_mode":           true,
	"hvac_action":         true,
	"media_title":         true,
	"media_artist":        true,
	"volume_level":        true,
	"battery_level":       true,
	"battery":             true,
	"power":               true,
	"energy":              true,
	"voltage":             true,
	"current":             true,
	"state_class":         true,
	"supported_features":  true,
	"min_temp":            true,
	"max_temp":            true,
	"preset_mode":         true,
	"effect":              true,
	"rgb_color":           true,
	"xy_color":            true,
	"hs_color":            true,
	"kelvin":              true,
}

// compressHAGetState compresses a single HA entity by keeping only
// essential fields and filtering attributes to an allowlist.
func compressHAGetState(output string) string {
	var envelope struct {
		Status string                 `json:"status"`
		Entity map[string]interface{} `json:"entity"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		return compressGeneric(output)
	}

	entity := envelope.Entity
	if entity == nil {
		return compressGeneric(output)
	}

	var sb strings.Builder
	entityID, _ := entity["entity_id"].(string)
	state, _ := entity["state"].(string)
	sb.WriteString(fmt.Sprintf("Entity: %s\n", entityID))
	sb.WriteString(fmt.Sprintf("State: %s\n", state))

	// Extract and filter attributes
	if attrs, ok := entity["attributes"].(map[string]interface{}); ok {
		var keptAttrs []string
		var keys []string
		for k := range attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			if haAttrAllowlist[k] {
				v := attrs[k]
				keptAttrs = append(keptAttrs, fmt.Sprintf("  %s: %v", k, v))
			}
		}

		if len(keptAttrs) > 0 {
			sb.WriteString("Attributes:\n")
			for _, a := range keptAttrs {
				sb.WriteString(a + "\n")
			}
		}
		omitted := len(attrs) - len(keptAttrs)
		if omitted > 0 {
			sb.WriteString(fmt.Sprintf("  [%d attributes omitted]\n", omitted))
		}
	}

	// Keep last_changed and last_updated if present
	if lc, ok := entity["last_changed"].(string); ok && lc != "" {
		sb.WriteString(fmt.Sprintf("Last changed: %s\n", lc))
	}
	if lu, ok := entity["last_updated"].(string); ok && lu != "" {
		sb.WriteString(fmt.Sprintf("Last updated: %s\n", lu))
	}

	return sb.String()
}

// compressHACallService compresses HA call_service output.
// The output is already compact, so we just format it cleanly.
func compressHACallService(output string) string {
	var envelope struct {
		Status           string   `json:"status"`
		Service          string   `json:"service"`
		AffectedEntities []string `json:"affected_entities"`
		Count            int      `json:"count"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		return compressGeneric(output)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✓ Service %s called successfully\n", envelope.Service))
	sb.WriteString(fmt.Sprintf("Affected entities (%d):\n", envelope.Count))
	for _, eid := range envelope.AffectedEntities {
		sb.WriteString(fmt.Sprintf("  %s\n", eid))
	}

	return sb.String()
}

// compressHAListServices compresses HA list_services output by sorting
// domains and limiting service names per domain.
func compressHAListServices(output string) string {
	var envelope struct {
		Status   string `json:"status"`
		Count    int    `json:"count"`
		Services []struct {
			Domain   string   `json:"domain"`
			Services []string `json:"services"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		return compressGeneric(output)
	}

	// Sort domains alphabetically
	sort.Slice(envelope.Services, func(i, j int) bool {
		return envelope.Services[i].Domain < envelope.Services[j].Domain
	})

	const maxServicesPerDomain = 15

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("HA Services: %d domains\n", envelope.Count))

	for _, svc := range envelope.Services {
		sorted := make([]string, len(svc.Services))
		copy(sorted, svc.Services)
		sort.Strings(sorted)

		if len(sorted) <= maxServicesPerDomain {
			sb.WriteString(fmt.Sprintf("  %s (%d): %s\n", svc.Domain, len(sorted), strings.Join(sorted, ", ")))
		} else {
			shown := sorted[:maxServicesPerDomain]
			remaining := len(sorted) - maxServicesPerDomain
			sb.WriteString(fmt.Sprintf("  %s (%d): %s, ... +%d more\n",
				svc.Domain, len(sorted), strings.Join(shown, ", "), remaining))
		}
	}

	return sb.String()
}
