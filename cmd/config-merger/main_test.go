package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/config"

	"gopkg.in/yaml.v3"
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

func TestSplitTopLevelSections_HyphenatedKeys(t *testing.T) {
	content := `remote-control:
    enabled: true
cloudflare-tunnel:
    enabled: true
server:
    port: 8088`

	sections := splitTopLevelSections(content)

	for _, name := range []string{"remote-control", "cloudflare-tunnel", "server"} {
		if _, ok := sections[name]; !ok {
			t.Errorf("section %q not found in %v", name, keys(sections))
		}
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

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat output: %v", err)
		}
		if perms := info.Mode().Perm(); perms != 0o600 {
			t.Fatalf("expected permissions 0600, got %04o", perms)
		}
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

func TestApplyUpgradeSafetyDefaults_DisablesAuthWhenMissingFromUserConfig(t *testing.T) {
	merged := map[string]interface{}{
		"auth": map[string]interface{}{
			"enabled": true,
		},
	}
	user := map[string]interface{}{
		"llm": map[string]interface{}{"provider": "main"},
	}

	changed := applyUpgradeSafetyDefaults(merged, user)

	if !changed {
		t.Fatal("expected safety adjustment to be applied")
	}
	auth, _ := asStringMap(merged["auth"])
	if auth["enabled"] != false {
		t.Fatalf("auth.enabled = %v, want false", auth["enabled"])
	}
}

func TestApplyUpgradeSafetyDefaults_PreservesExplicitAuthEnabled(t *testing.T) {
	merged := map[string]interface{}{
		"auth": map[string]interface{}{
			"enabled": true,
		},
	}
	user := map[string]interface{}{
		"auth": map[string]interface{}{
			"enabled": true,
		},
	}

	changed := applyUpgradeSafetyDefaults(merged, user)

	if changed {
		t.Fatal("expected no safety adjustment for explicit auth.enabled")
	}
	auth, _ := asStringMap(merged["auth"])
	if auth["enabled"] != true {
		t.Fatalf("auth.enabled = %v, want true", auth["enabled"])
	}
}

func TestConfigTemplateBudgetBlockUsesCanonicalRoot(t *testing.T) {
	templatePath := filepath.Join("..", "..", "config_template.yaml")
	content, err := readNormalized(templatePath)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	tmplMap, err := parseYAMLMap(content)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	agent, ok := asStringMap(tmplMap["agent"])
	if !ok {
		t.Fatal("template agent section missing or invalid")
	}
	if _, legacy := agent["budget"]; legacy {
		t.Fatal("obsolete agent.budget block remains")
	}
	budget, ok := asStringMap(tmplMap["budget"])
	if !ok {
		t.Fatal("template budget section missing or invalid")
	}
	if budget["daily_limit_usd"] != 5 {
		t.Fatalf("budget.daily_limit_usd = %v, want 5", budget["daily_limit_usd"])
	}
	if _, hasTopLevel := tmplMap["daily_limit_usd"]; hasTopLevel {
		t.Fatal("template leaked daily_limit_usd to top level")
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

// ── enforceTemplateTypes tests ────────────────────────────────────────

func TestEnforceTemplateTypes_StringToArray(t *testing.T) {
	// User has "" (string) where template has [] (array) — must fix
	tmpl := map[string]interface{}{
		"remote_control": map[string]interface{}{
			"allowed_paths": []interface{}{},
		},
	}
	merged := map[string]interface{}{
		"remote_control": map[string]interface{}{
			"allowed_paths": "",
		},
	}

	fixed := enforceTemplateTypes(merged, tmpl)

	if !fixed {
		t.Error("expected enforceTemplateTypes to return true")
	}
	rc, _ := asStringMap(merged["remote_control"])
	if _, ok := rc["allowed_paths"].([]interface{}); !ok {
		t.Errorf("allowed_paths should be []interface{}, got %T", rc["allowed_paths"])
	}
}

func TestEnforceTemplateTypes_ArrayPreserved(t *testing.T) {
	// User has a proper array — must NOT be replaced
	tmpl := map[string]interface{}{
		"indexing": map[string]interface{}{
			"directories": []interface{}{},
		},
	}
	merged := map[string]interface{}{
		"indexing": map[string]interface{}{
			"directories": []interface{}{"/home/user", "/var/data"},
		},
	}

	fixed := enforceTemplateTypes(merged, tmpl)

	if fixed {
		t.Error("correctly typed array should not trigger a fix")
	}
	idx, _ := asStringMap(merged["indexing"])
	dirs, ok := idx["directories"].([]interface{})
	if !ok || len(dirs) != 2 {
		t.Errorf("user's array should be preserved, got %v", idx["directories"])
	}
}

func TestEnforceTemplateTypes_StringToBool(t *testing.T) {
	tmpl := map[string]interface{}{
		"agent": map[string]interface{}{
			"allow_shell": true,
		},
	}
	merged := map[string]interface{}{
		"agent": map[string]interface{}{
			"allow_shell": "true",
		},
	}

	fixed := enforceTemplateTypes(merged, tmpl)

	if !fixed {
		t.Error("expected fix for string→bool coercion")
	}
	ag, _ := asStringMap(merged["agent"])
	if ag["allow_shell"] != true {
		t.Errorf("expected true (bool), got %v (%T)", ag["allow_shell"], ag["allow_shell"])
	}
}

func TestEnforceTemplateTypes_MapVsScalar(t *testing.T) {
	// Template has a map but merged has a string — map wins
	tmpl := map[string]interface{}{
		"server": map[string]interface{}{
			"host": "127.0.0.1",
			"port": 8088,
		},
	}
	merged := map[string]interface{}{
		"server": "broken_value",
	}

	fixed := enforceTemplateTypes(merged, tmpl)

	if !fixed {
		t.Error("expected fix for scalar→map replacement")
	}
	if _, ok := asStringMap(merged["server"]); !ok {
		t.Errorf("server should be restored to map, got %T", merged["server"])
	}
}

func TestEnforceTemplateTypes_CorrectTypesUntouched(t *testing.T) {
	// All types match — nothing should change
	tmpl := map[string]interface{}{
		"agent": map[string]interface{}{
			"debug_mode": false,
			"max_tools":  10,
		},
	}
	merged := map[string]interface{}{
		"agent": map[string]interface{}{
			"debug_mode": true,
			"max_tools":  20,
		},
	}

	fixed := enforceTemplateTypes(merged, tmpl)

	if fixed {
		t.Error("correctly typed values should not trigger any fix")
	}
	ag, _ := asStringMap(merged["agent"])
	if ag["debug_mode"] != true {
		t.Error("user's bool value should be preserved")
	}
	if ag["max_tools"] != 20 {
		t.Error("user's int value should be preserved")
	}
}

// ── sanitizeMergedConfig extended tests ──────────────────────────────

func TestSanitizeMergedConfig_StringArrayFields(t *testing.T) {
	// Simulates corrupted []string fields that are strings instead of arrays
	m := map[string]interface{}{
		"remote_control": map[string]interface{}{
			"allowed_paths": "",
		},
		"indexing": map[string]interface{}{
			"directories": "single_path",
		},
		"github": map[string]interface{}{
			"allowed_repos": 42, // wrong type entirely
		},
	}

	changed := sanitizeMergedConfig(m)

	if !changed {
		t.Error("expected sanitizeMergedConfig to return true")
	}
	rc, _ := asStringMap(m["remote_control"])
	if _, ok := rc["allowed_paths"].([]interface{}); !ok {
		t.Errorf("allowed_paths should be reset to [], got %T", rc["allowed_paths"])
	}
	idx, _ := asStringMap(m["indexing"])
	if _, ok := idx["directories"].([]interface{}); !ok {
		t.Errorf("directories should be reset to [], got %T", idx["directories"])
	}
	gh, _ := asStringMap(m["github"])
	if _, ok := gh["allowed_repos"].([]interface{}); !ok {
		t.Errorf("allowed_repos should be reset to [], got %T", gh["allowed_repos"])
	}
}

func TestSanitizeMergedConfig_ValidArraysUntouched(t *testing.T) {
	m := map[string]interface{}{
		"remote_control": map[string]interface{}{
			"allowed_paths": []interface{}{"/home", "/var"},
		},
		"indexing": map[string]interface{}{
			// Bare string items — sanitize must convert them to {path: "..."} objects
			"directories": []interface{}{"/docs"},
		},
	}

	changed := sanitizeMergedConfig(m)

	// Bare strings in indexing.directories must be fixed
	if !changed {
		t.Error("sanitizeMergedConfig should modify bare-string directory items")
	}
	idx, _ := asStringMap(m["indexing"])
	dirs := idx["directories"].([]interface{})
	if len(dirs) != 1 {
		t.Errorf("directories len = %d, want 1", len(dirs))
	}
	dir0, ok := asStringMap(dirs[0])
	if !ok || dir0["path"] != "/docs" {
		t.Errorf("directories[0] = %v, want {path: /docs}", dirs[0])
	}
}

func TestSanitizeMergedConfig_DirectoryObjectsUntouched(t *testing.T) {
	// Properly formatted {path: ..., collection: ...} objects must NOT be modified
	m := map[string]interface{}{
		"indexing": map[string]interface{}{
			"directories": []interface{}{
				map[string]interface{}{"path": "./knowledge", "collection": "kb"},
				map[string]interface{}{"path": "./docs", "collection": ""},
			},
		},
	}

	changed := sanitizeMergedConfig(m)

	if changed {
		t.Error("properly formatted directory objects should not be modified")
	}
	idx, _ := asStringMap(m["indexing"])
	dirs := idx["directories"].([]interface{})
	if len(dirs) != 2 {
		t.Errorf("directories len = %d, want 2", len(dirs))
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

func TestApplyUpgradeSafetyDefaults_MaterialisesDockerHostAccess(t *testing.T) {
	template := map[string]interface{}{
		"docker": map[string]interface{}{
			"enabled":           false,
			"host":              "",
			"readonly":          false,
			"allow_host_access": false,
		},
	}
	cases := []struct {
		name        string
		userDocker  map[string]interface{}
		want        bool
		wantChanged bool
	}{
		{"docker on, key absent", map[string]interface{}{"enabled": true}, true, true},
		{"docker off, key absent", map[string]interface{}{"enabled": false}, true, true},
		{"no docker section", nil, true, true},
		{"key written false", map[string]interface{}{"enabled": true, "allow_host_access": false}, false, false},
		{"key written true", map[string]interface{}{"enabled": true, "allow_host_access": true}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := map[string]interface{}{}
			if tc.userDocker != nil {
				user["docker"] = tc.userDocker
			}
			merged := deepMerge(template, user)

			changed := applyUpgradeSafetyDefaults(merged, user)

			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tc.wantChanged)
			}
			docker, _ := asStringMap(merged["docker"])
			if docker["allow_host_access"] != tc.want {
				t.Fatalf("docker.allow_host_access = %v, want %v", docker["allow_host_access"], tc.want)
			}
		})
	}
}

func repositoryTemplateMap(t *testing.T) (string, map[string]interface{}) {
	t.Helper()
	content, err := readNormalized(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	tmplMap, err := parseYAMLMap(content)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	return content, tmplMap
}

func templateDockerHostAccess(t *testing.T, tmplMap map[string]interface{}) interface{} {
	t.Helper()
	docker, ok := asStringMap(tmplMap["docker"])
	if !ok {
		t.Fatalf("template docker section missing: %#v", tmplMap["docker"])
	}
	return docker["allow_host_access"]
}

// loadMergedDockerHostAccess writes the merger output and loads it the way
// AuraGo does at startup, so the test pins the effective value, not only the
// YAML map.
func loadMergedDockerHostAccess(t *testing.T, merged map[string]interface{}) bool {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	atomicWriteYAML(path, merged)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load(merged) error = %v", err)
	}
	return cfg.Docker.AllowHostAccess
}

// Pre-K5 configurations keep agent Compose host access through an upgrade,
// whatever shape their docker section has. enforceTemplateTypes replaces a
// null or scalar docker section with the template map, so the grandfather has
// to run after it.
func TestMergeUserConfig_GrandfathersDockerHostAccess(t *testing.T) {
	templateContent, tmplMap := repositoryTemplateMap(t)
	if got := templateDockerHostAccess(t, tmplMap); got != false {
		t.Fatalf("template docker.allow_host_access = %#v, want false", got)
	}
	cases := []struct {
		name string
		user string
		want bool
	}{
		{"docker null (children commented out)", "server:\n  port: 8088\ndocker:\n  # enabled: true\n", true},
		{"docker scalar false", "server:\n  port: 8088\ndocker: false\n", true},
		{"docker empty map", "server:\n  port: 8088\ndocker: {}\n", true},
		{"pre-K5 docker section", "server:\n  port: 8088\ndocker:\n  enabled: true\n  readonly: false\n", true},
		{"no docker section", "server:\n  port: 8088\n", true},
		{"explicit false", "server:\n  port: 8088\ndocker:\n  enabled: true\n  allow_host_access: false\n", false},
		{"explicit true", "server:\n  port: 8088\ndocker:\n  enabled: true\n  allow_host_access: true\n", true},
		// A written key ends the grandfather even without a value; the template
		// type (bool false) replaces the null, as config.Load reads it.
		{"explicit null", "server:\n  port: 8088\ndocker:\n  enabled: true\n  allow_host_access:\n", false},
		{"fresh install copied from the template", templateContent, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srcMap, err := parseYAMLMap(tc.user)
			if err != nil {
				t.Fatalf("parse user config: %v", err)
			}

			result := mergeUserConfig(tmplMap, srcMap)

			docker, ok := asStringMap(result.merged["docker"])
			if !ok {
				t.Fatalf("merged docker section = %#v, want a map", result.merged["docker"])
			}
			if docker["allow_host_access"] != tc.want {
				t.Fatalf("merged docker.allow_host_access = %#v, want %v", docker["allow_host_access"], tc.want)
			}
			if got := loadMergedDockerHostAccess(t, result.merged); got != tc.want {
				t.Fatalf("loaded Docker.AllowHostAccess = %v, want %v", got, tc.want)
			}
			if got := templateDockerHostAccess(t, tmplMap); got != false {
				t.Fatalf("merge mutated the template: docker.allow_host_access = %#v", got)
			}
		})
	}
}

func TestMergeUserConfig_LeavesFreshTemplateConfigUnchanged(t *testing.T) {
	templateContent, tmplMap := repositoryTemplateMap(t)
	srcMap, err := parseYAMLMap(templateContent)
	if err != nil {
		t.Fatal(err)
	}
	result := mergeUserConfig(tmplMap, srcMap)
	if result.needsWrite() {
		t.Fatalf("a config copied from the template must be up to date: missing=%v safety=%v types=%v sanitized=%v", result.missing, result.safetyAdjusted, result.typeFixed, result.sanitized)
	}
}

// The corruption path keeps the grandfather for existing installations. A
// docker section that cannot be salvaged loses its explicit false and comes
// back as true (Docker itself is reset to the template's disabled state).
func TestRecoverCorruptedConfig_GrandfathersDockerHostAccess(t *testing.T) {
	_, tmplMap := repositoryTemplateMap(t)
	cases := []struct {
		name        string
		source      string
		want        bool
		wantEnabled bool
	}{
		{"corrupt docker section with explicit false", "server:\n  port: 8088\ndocker:\n  enabled: true\n  allow_host_access: false\n   bad: [\n", true, false},
		{"salvaged docker section with explicit false", "server:\n  port: [\ndocker:\n  enabled: true\n  allow_host_access: false\n", false, true},
		{"salvaged pre-K5 docker section", "server:\n  port: [\ndocker:\n  enabled: true\n", true, true},
		{"salvaged docker null", "server:\n  port: [\ndocker:\n  # enabled: true\n", true, false},
		{"nothing salvageable", "server: [broken\n", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseYAMLMap(tc.source); err == nil {
				t.Fatal("test source must be unparseable as a whole")
			}

			merged := recoverCorruptedConfig(tmplMap, salvageSections(tc.source))

			docker, ok := asStringMap(merged["docker"])
			if !ok {
				t.Fatalf("recovered docker section = %#v, want a map", merged["docker"])
			}
			if docker["allow_host_access"] != tc.want || docker["enabled"] != tc.wantEnabled {
				t.Fatalf("recovered docker section = %#v, want allow_host_access %v and enabled %v", docker, tc.want, tc.wantEnabled)
			}
			if got := templateDockerHostAccess(t, tmplMap); got != false {
				t.Fatalf("recovery mutated the template: docker.allow_host_access = %#v", got)
			}
		})
	}
}

// An empty or comment-only config.yaml (the Docker guide has users `touch` it
// before the first start) is a fresh install: it gets the template unchanged,
// without a .corrupted backup. A source with top-level keys that does not
// parse is an existing installation and keeps the grandfather.
func TestRun_EmptySourceIsFreshInstall(t *testing.T) {
	templatePath := filepath.Join("..", "..", "config_template.yaml")
	_, tmplMap := repositoryTemplateMap(t)
	cases := []struct {
		name         string
		source       string
		want         bool
		wantTemplate bool
		wantBackup   bool
	}{
		{"empty file", "", false, true, false},
		{"comment-only file", "# my config\n", false, true, false},
		{"comment-only file with CRLF", "# my config\r\n\r\n", false, true, false},
		{"unparseable source with top-level keys", "server: [broken\n", true, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sourcePath := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(sourcePath, []byte(tc.source), 0o600); err != nil {
				t.Fatal(err)
			}

			run(sourcePath, templatePath, sourcePath)

			rawOut, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			templateOut, err := yaml.Marshal(tmplMap)
			if err != nil {
				t.Fatal(err)
			}
			if got := string(rawOut) == string(templateOut); got != tc.wantTemplate {
				t.Fatalf("output is the unchanged template = %v, want %v", got, tc.wantTemplate)
			}

			outData, err := readNormalized(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			outMap, err := parseYAMLMap(outData)
			if err != nil {
				t.Fatalf("merger output is not valid YAML: %v", err)
			}
			docker, _ := asStringMap(outMap["docker"])
			if docker["allow_host_access"] != tc.want {
				t.Fatalf("output docker.allow_host_access = %#v, want %v", docker["allow_host_access"], tc.want)
			}
			cfg, err := config.Load(sourcePath)
			if err != nil {
				t.Fatalf("config.Load(output) error = %v", err)
			}
			if cfg.Docker.AllowHostAccess != tc.want {
				t.Fatalf("loaded Docker.AllowHostAccess = %v, want %v", cfg.Docker.AllowHostAccess, tc.want)
			}
			backups, err := filepath.Glob(sourcePath + ".*.corrupted")
			if err != nil {
				t.Fatal(err)
			}
			if (len(backups) > 0) != tc.wantBackup {
				t.Fatalf(".corrupted backups = %v, want present = %v", backups, tc.wantBackup)
			}
		})
	}
}
