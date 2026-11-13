package tools

import (
	"strings"
	"testing"
)

func TestHAExecutableServicesNeedExplicitGrant(t *testing.T) {
	for _, domain := range []string{"shell_command", "python_script", "script", "hassio", "automation", "rest_command", "pyscript"} {
		t.Run(domain, func(t *testing.T) {
			if got := HAServiceRefusal(domain, "run", nil, nil); !strings.Contains(got, "explicit") {
				t.Fatalf("missing explicit-grant denial: %q", got)
			}
			allowed := []string{strings.ToUpper(domain) + ".RUN"}
			if got := HAServiceRefusal(" "+domain+" ", " run ", allowed, nil); got != "" {
				t.Fatal(got)
			}
			if got := HAServiceRefusal(domain, "run", allowed, []string{domain + ".run"}); !strings.Contains(got, "blocked") {
				t.Fatalf("blocklist did not win: %q", got)
			}
			if HAServiceAllowed(HAConfig{ReadOnly: true, AllowedServices: allowed}, domain, "run") {
				t.Fatal("allowlist bypassed read-only")
			}
		})
	}
	if got := HAServiceRefusal("light", "turn_on", nil, nil); got != "" {
		t.Fatalf("ordinary switch denied: %s", got)
	}
}

func TestHAGenericTargetsCannotBypassPolicy(t *testing.T) {
	for _, tc := range []struct {
		name             string
		payload          map[string]interface{}
		allowed, blocked []string
		denied           bool
	}{
		{"ordinary light", map[string]interface{}{"entity_id": "light.desk"}, nil, nil, false},
		{"script", map[string]interface{}{"entity_id": "script.run"}, nil, nil, true},
		{"nested automation", map[string]interface{}{"target": map[string]interface{}{"entity_id": []interface{}{"light.desk", "automation.run"}}}, nil, nil, true},
		{"explicit script", map[string]interface{}{"entity_id": "script.run"}, []string{"homeassistant.turn_on", "script.turn_on"}, nil, false},
		{"generic grant insufficient", map[string]interface{}{"entity_id": "script.run"}, []string{"homeassistant.turn_on"}, nil, true},
		{"blocked target wins", map[string]interface{}{"entity_id": "script.run"}, []string{"homeassistant.turn_on", "script.turn_on"}, []string{"script.turn_on"}, true},
		{"group", map[string]interface{}{"entity_id": "group.room"}, nil, nil, true},
		{"all", map[string]interface{}{"entity_id": "all"}, nil, nil, true},
		{"missing", nil, nil, nil, true},
		{"area", map[string]interface{}{"entity_id": "light.desk", "area_id": "room"}, nil, nil, true},
		{"nested device", map[string]interface{}{"target": map[string]interface{}{"entity_id": "light.desk", "device_id": "device"}}, nil, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := HAServiceTargetRefusal("homeassistant", "turn_on", tc.payload, tc.allowed, tc.blocked); (got != "") != tc.denied {
				t.Fatalf("refusal=%q, denied=%v", got, tc.denied)
			}
		})
	}
	for _, service := range []string{"stop", "restart", "reload_all", "reload_core_config", "reload_config_entry"} {
		if got := HAServiceRefusal("homeassistant", service, nil, nil); !strings.Contains(got, "explicit") {
			t.Fatalf("critical %s admitted: %s", service, got)
		}
	}
}
