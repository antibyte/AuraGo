package main

import "testing"

func TestLLMRouterMergePreservesUserOffAndEmpty(t *testing.T) {
	template, err := parseYAMLMap("llm_router:\n  enabled: false\n  helper_fallback: true\n  helper_max_calls_per_hour: 20\n  areas:\n    coding: {provider: '', model: ''}\n    writing: {provider: '', model: ''}\n")
	if err != nil {
		t.Fatal(err)
	}
	user, err := parseYAMLMap("llm_router:\n  enabled: true\n  helper_fallback: false\n  helper_max_calls_per_hour: 0\n  areas:\n    coding: {provider: code, model: ''}\n")
	if err != nil {
		t.Fatal(err)
	}
	merged := deepMerge(template, user)
	router, _ := asStringMap(merged["llm_router"])
	areas, _ := asStringMap(router["areas"])
	code, _ := asStringMap(areas["coding"])
	if router["enabled"] != true || router["helper_fallback"] != false || router["helper_max_calls_per_hour"] != 0 || code["provider"] != "code" || code["model"] != "" || areas["writing"] == nil {
		t.Fatal("router choices lost during upgrade merge")
	}
}
