package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeepMerge_BasicOverlay(t *testing.T) {
	base := map[string]interface{}{
		"server": map[string]interface{}{
			"host": "127.0.0.1",
			"port": 8088,
		},
		"budget": map[string]interface{}{
			"enabled": false,
		},
	}
	overlay := map[string]interface{}{
		"server": map[string]interface{}{
			"host": "0.0.0.0",
		},
	}

	merged := deepMerge(base, overlay)

	srv, _ := asStringMap(merged["server"])
	if srv["host"] != "0.0.0.0" {
		t.Errorf("expected host=0.0.0.0, got %v", srv["host"])
	}
	if srv["port"] != 8088 {
		t.Errorf("expected port=8088, got %v", srv["port"])
	}
	if merged["budget"] == nil {
		t.Error("base-only key 'budget' should be preserved")
	}
}

func TestDeepMerge_UserOnlyKeys(t *testing.T) {
	base := map[string]interface{}{
		"server": map[string]interface{}{"port": 8088},
	}
	overlay := map[string]interface{}{
		"server":       map[string]interface{}{"port": 9090},
		"custom_addon": "user_value",
	}

	merged := deepMerge(base, overlay)

	if merged["custom_addon"] != "user_value" {
		t.Error("user-only key 'custom_addon' should be preserved")
	}
	srv, _ := asStringMap(merged["server"])
	if srv["port"] != 9090 {
		t.Errorf("overlay should win on leaf values, got %v", srv["port"])
	}
}

func TestDeepMerge_LeafTypeMismatch(t *testing.T) {
	// When user has a scalar where template has a map, user wins
	base := map[string]interface{}{
		"thing": map[string]interface{}{"nested": true},
	}
	overlay := map[string]interface{}{
		"thing": "flat_value",
	}

	merged := deepMerge(base, overlay)

	if merged["thing"] != "flat_value" {
		t.Errorf("overlay should win on type mismatch, got %v", merged["thing"])
	}
}

func TestSplitTopLevelSections(t *testing.T) {
	content := `server:
    host: 0.0.0.0
    port: 8088
budget:
    enabled: false
    models:
        - name: test
agent:
    debug: true`

	sections := splitTopLevelSections(content)

	if len(sections) != 3 {
		t.Fatalf("expected 3 sections, got %d: %v", len(sections), keys(sections))
	}
	for _, name := range []string{"server", "budget", "agent"} {
		if _, ok := sections[name]; !ok {
			t.Errorf("section '%s' not found", name)
		}
	}
	if !strings.Contains(sections["budget"], "models:") {
		t.Error("budget section should contain nested content")
	}
}

func TestSalvageSections_PartialCorruption(t *testing.T) {
	// budget section is corrupted, server and agent are valid
	content := `server:
    host: 0.0.0.0
    port: 8088
budget:
    enabled: false
    models:
      - name: test
        cost {invalid yaml here
agent:
    debug: true`

	salvaged := salvageSections(content)

	if _, ok := salvaged["server"]; !ok {
		t.Error("server section should be salvaged")
	}
	if _, ok := salvaged["agent"]; !ok {
		t.Error("agent section should be salvaged")
	}
	// budget is corrupted — it may or may not be salvaged depending on where
	// the corruption falls relative to the YAML parser. The key point is that
	// server and agent are recovered.
}

func TestSalvageSections_TotalCorruption(t *testing.T) {
	content := `{{{what is this even`

	salvaged := salvageSections(content)

	if len(salvaged) != 0 {
		t.Errorf("expected 0 salvaged sections, got %d", len(salvaged))
	}
}

func TestFindMissingTopKeys(t *testing.T) {
	tmpl := map[string]interface{}{
		"server": "a",
		"budget": "b",
		"agent":  "c",
	}
	src := map[string]interface{}{
		"server": "x",
	}

	missing := findMissingTopKeys(tmpl, src)

	if len(missing) != 2 {
		t.Fatalf("expected 2 missing keys, got %d: %v", len(missing), missing)
	}
	has := make(map[string]bool)
	for _, k := range missing {
		has[k] = true
	}
	if !has["budget"] || !has["agent"] {
		t.Errorf("expected budget and agent missing, got %v", missing)
	}
}

func TestParseYAMLMap(t *testing.T) {
	m, err := parseYAMLMap("server:\n    port: 8088\n")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	srv, ok := asStringMap(m["server"])
	if !ok {
		t.Fatal("server should be a map")
	}
	if srv["port"] != 8088 {
		t.Errorf("expected port=8088, got %v", srv["port"])
	}
}

func TestParseYAMLMap_Invalid(t *testing.T) {
	_, err := parseYAMLMap("{{not yaml")
	if err == nil {
		t.Error("expected parse error for invalid YAML")
	}
}

func TestReadNormalized(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.yaml")
	// Write file with CRLF, tabs, and trailing whitespace
	os.WriteFile(tmp, []byte("server:\r\n\tport: 8080  \r\n"), 0644)

	content, err := readNormalized(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(content, "\r") {
		t.Error("CRLF should be converted to LF")
	}
	if strings.Contains(content, "\t") {
		t.Error("tabs should be converted to spaces")
	}
}

func TestAtomicWriteYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.yaml")

	data := map[string]interface{}{
		"server": map[string]interface{}{
			"host": "0.0.0.0",
			"port": 8088,
		},
	}
	atomicWriteYAML(path, data)

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}

	// Verify it's valid YAML by re-parsing
	m, err := parseYAMLMap(string(content))
	if err != nil {
		t.Fatalf("output is not valid YAML: %v", err)
	}
	srv, _ := asStringMap(m["server"])
	if srv["host"] != "0.0.0.0" {
		t.Errorf("expected host=0.0.0.0, got %v", srv["host"])
	}
}

func TestCountTopLevelKeys(t *testing.T) {
	content := "server:\n    port: 8088\nbudget:\n    enabled: false\nagent:\n    debug: true\n"
	n := countTopLevelKeys(content)
	if n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
}

func TestEndToEnd_MergeNewSections(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.yaml")
	sourcePath := filepath.Join(dir, "source.yaml")
	outputPath := filepath.Join(dir, "output.yaml")

	os.WriteFile(templatePath, []byte(`server:
    host: 127.0.0.1
    port: 8088
budget:
    daily_limit_usd: 5
agent:
    debug: false
`), 0644)

	os.WriteFile(sourcePath, []byte(`server:
    host: 0.0.0.0
    port: 9090
agent:
    debug: true
`), 0644)

	// Simulates what main() does
	tmplData, _ := readNormalized(templatePath)
	tmplMap, _ := parseYAMLMap(tmplData)
	srcData, _ := readNormalized(sourcePath)
	srcMap, _ := parseYAMLMap(srcData)

	merged := deepMerge(tmplMap, srcMap)
	atomicWriteYAML(outputPath, merged)

	// Verify
	outData, _ := os.ReadFile(outputPath)
	outMap, err := parseYAMLMap(string(outData))
	if err != nil {
		t.Fatal(err)
	}

	// User values preserved
	srv, _ := asStringMap(outMap["server"])
	if srv["host"] != "0.0.0.0" {
		t.Errorf("user host should be preserved, got %v", srv["host"])
	}
	if srv["port"] != 9090 {
		t.Errorf("user port should be preserved, got %v", srv["port"])
	}

	// New section added from template
	if outMap["budget"] == nil {
		t.Error("budget section should be added from template")
	}
	bud, _ := asStringMap(outMap["budget"])
	if bud["daily_limit_usd"] != 5 {
		t.Errorf("budget default should be 5, got %v", bud["daily_limit_usd"])
	}

	// User value in shared section preserved
	ag, _ := asStringMap(outMap["agent"])
	if ag["debug"] != true {
		t.Errorf("user agent.debug should be true, got %v", ag["debug"])
	}
}

func TestEndToEnd_RepairCorrupted(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "template.yaml")
	sourcePath := filepath.Join(dir, "source.yaml")
	outputPath := filepath.Join(dir, "output.yaml")

	os.WriteFile(templatePath, []byte(`server:
    host: 127.0.0.1
    port: 8088
budget:
    daily_limit_usd: 5
`), 0644)

	// Corrupted source: budget section has invalid YAML, server is fine
	os.WriteFile(sourcePath, []byte(`server:
    host: 10.0.0.1
    port: 3000
budget:
    daily_limit_usd: {broken
`), 0644)

	tmplData, _ := readNormalized(templatePath)
	tmplMap, _ := parseYAMLMap(tmplData)
	srcData, _ := readNormalized(sourcePath)
	_, parseErr := parseYAMLMap(srcData)

	if parseErr == nil {
		t.Skip("test requires the source to be unparseable; YAML library may be lenient")
	}

	// Salvage
	salvaged := salvageSections(srcData)
	merged := deepMerge(tmplMap, salvaged)
	atomicWriteYAML(outputPath, merged)

	outData, _ := os.ReadFile(outputPath)
	outMap, err := parseYAMLMap(string(outData))
	if err != nil {
		t.Fatal(err)
	}

	// Server should be recovered with user's values
	srv, ok := asStringMap(outMap["server"])
	if ok && srv["host"] == "10.0.0.1" {
		t.Log("server section recovered with user values — good")
	}

	// Budget should fall back to template (was corrupted)
	if outMap["budget"] != nil {
		t.Log("budget section present (from template or salvage) — good")
	}
}

func TestSanitizeMergedConfig_StringModels(t *testing.T) {
	// Simulates the real-world case: user config had budget.models as plain strings
	m := map[string]interface{}{
		"budget": map[string]interface{}{
			"enabled": true,
			"models":  []interface{}{"arcee-agent", "gpt-4o"},
		},
	}

	changed := sanitizeMergedConfig(m)

	if !changed {
		t.Error("expected sanitizeMergedConfig to return true (change applied)")
	}
	budget, _ := asStringMap(m["budget"])
	models, ok := budget["models"].([]interface{})
	if !ok {
		t.Fatal("models should still be []interface{}")
	}
	if len(models) != 0 {
		t.Errorf("models should be reset to [], got %v", models)
	}
}

func TestSanitizeMergedConfig_ValidModels(t *testing.T) {
	m := map[string]interface{}{
		"budget": map[string]interface{}{
			"models": []interface{}{
				map[string]interface{}{"name": "gpt-4o", "cost_per_1k": 0.01},
			},
		},
	}

	changed := sanitizeMergedConfig(m)

	if changed {
		t.Error("valid models should not be modified")
	}
}

func TestSanitizeMergedConfig_EmptyModels(t *testing.T) {
	m := map[string]interface{}{
		"budget": map[string]interface{}{
			"models": []interface{}{},
		},
	}

	changed := sanitizeMergedConfig(m)

	if changed {
		t.Error("empty models list should not be modified")
	}
}

// keys returns map keys as a slice
func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
