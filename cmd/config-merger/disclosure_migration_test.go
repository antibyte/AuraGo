package main

import "testing"

func TestLegacyDisclosureValuesWinOverNewTemplateDefaults(t *testing.T) {
	template, err := parseYAMLMap("circuit_breaker:\n  max_tool_calls: 20\nbudget:\n  enabled: false\n")
	if err != nil {
		t.Fatal(err)
	}
	user, err := parseYAMLMap("agent:\n  max_tool_calls: 15\n  budget:\n    enabled: true\n")
	if err != nil {
		t.Fatal(err)
	}
	merged := deepMerge(template, user)
	breaker, _ := asStringMap(merged["circuit_breaker"])
	budget, _ := asStringMap(merged["budget"])
	if breaker["max_tool_calls"] != 15 || budget["enabled"] != true {
		t.Fatalf("legacy values lost to new defaults: %#v", merged)
	}
	canonical, err := parseYAMLMap("agent:\n  max_tool_calls: 15\ncircuit_breaker:\n  max_tool_calls: 7\n")
	if err != nil {
		t.Fatal(err)
	}
	breaker, _ = asStringMap(canonical["circuit_breaker"])
	if breaker["max_tool_calls"] != 7 {
		t.Fatalf("explicit canonical value lost: %#v", canonical)
	}
}
