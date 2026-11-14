package tools

import (
	"regexp"
	"strings"
)

var haConcreteEntity = regexp.MustCompile(`^[a-z_][a-z0-9_]*\.[a-z0-9_]+$`)

// HAServiceTargetRefusal checks the effective actions behind generic HA calls.
// It does not resolve groups remotely: membership can change between check and call.
func HAServiceTargetRefusal(domain, service string, payload map[string]interface{}, allowed, blocked []string) string {
	domain, service = strings.ToLower(strings.TrimSpace(domain)), strings.ToLower(strings.TrimSpace(service))
	if d, s, ok := strings.Cut(service, "."); ok && (domain == "" || domain == d) {
		domain, service = d, s
	}
	if msg := HAServiceRefusal(domain, service, allowed, blocked); msg != "" {
		return msg
	}
	if domain != "homeassistant" || service != "turn_on" && service != "turn_off" && service != "toggle" {
		return ""
	}
	const unresolved = "Home Assistant generic service requires concrete entity targets; groups, areas, devices and all-target calls cannot be safely authorized"
	var entities []string
	var collect func(interface{}) bool
	collect = func(value interface{}) bool {
		switch v := value.(type) {
		case string:
			ids := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' })
			if len(ids) == 0 {
				return false
			}
			for _, id := range ids {
				if !haConcreteEntity.MatchString(id) || strings.HasPrefix(id, "group.") {
					return false
				}
				entities = append(entities, id)
			}
		case []string:
			if len(v) == 0 {
				return false
			}
			for _, id := range v {
				if !collect(id) {
					return false
				}
			}
		case []interface{}:
			if len(v) == 0 {
				return false
			}
			for _, id := range v {
				if !collect(id) {
					return false
				}
			}
		default:
			return false
		}
		return true
	}
	for key, value := range payload {
		switch key {
		case "entity_id":
			if !collect(value) {
				return unresolved
			}
		case "area_id", "device_id", "label_id", "floor_id":
			return unresolved
		case "target":
			target, ok := value.(map[string]interface{})
			if !ok || len(target) != 1 || !collect(target["entity_id"]) {
				return unresolved
			}
		}
	}
	if len(entities) == 0 {
		return unresolved
	}
	for _, entity := range entities {
		d, _, _ := strings.Cut(entity, ".")
		if msg := HAServiceRefusal(d, service, allowed, blocked); msg != "" {
			return msg
		}
	}
	return ""
}
