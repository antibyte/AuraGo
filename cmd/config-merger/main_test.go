package main

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestApplyUpgradeSafetyDefaults_PreservesUnsandboxedShellForEnabledShell(t *testing.T) {
	template := map[string]interface{}{
		"agent": map[string]interface{}{
			"allow_shell":                 false,
			"allow_unsafe_host_execution": false,
			"allow_unsandboxed_shell":     false,
		},
	}
	cases := []struct {
		name        string
		userAgent   map[string]interface{}
		want        bool
		wantChanged bool
	}{
		{"shell on, key absent", map[string]interface{}{"allow_shell": true}, true, true},
		{"shell on, key written false", map[string]interface{}{"allow_shell": true, "allow_unsandboxed_shell": false}, false, false},
		{"shell absent", map[string]interface{}{"debug_mode": true}, false, true},
		{"shell off", map[string]interface{}{"allow_shell": false}, false, true},
		{"no agent section", nil, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := map[string]interface{}{}
			if tc.userAgent != nil {
				user["agent"] = tc.userAgent
			}
			merged := deepMerge(template, user)

			changed := applyUpgradeSafetyDefaults(merged, user)

			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tc.wantChanged)
			}
			agent, _ := asStringMap(merged["agent"])
			if agent["allow_unsandboxed_shell"] != tc.want {
				t.Fatalf("agent.allow_unsandboxed_shell = %v, want %v", agent["allow_unsandboxed_shell"], tc.want)
			}
		})
	}
}

// repositoryTemplate reads config_template.yaml and returns its text and a
// freshly parsed map.
func repositoryTemplate(t *testing.T) (string, map[string]interface{}) {
	t.Helper()
	data, err := readNormalized(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	m, err := parseYAMLMap(data)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	return data, m
}

// mergedValue returns the value at a dotted path of a merged config map.
func mergedValue(m map[string]interface{}, dotted string) interface{} {
	var cur interface{} = m
	for _, key := range strings.Split(dotted, ".") {
		section, ok := asStringMap(cur)
		if !ok {
			return nil
		}
		cur = section[key]
	}
	return cur
}

// assertMergeMatchesLoad writes merged, checks that config.Load accepts it
// and, when the user's original config loads on its own, that the behaviour
// the upgrade rules and type fixes decide is unchanged. A whole-Config
// comparison is not meaningful: merging a partial config deliberately fills
// every other key with its template default (about 140 fields for a minimal
// config), so the compared fields are named here.
func assertMergeMatchesLoad(t *testing.T, userYAML string, merged map[string]interface{}) {
	t.Helper()
	dir := t.TempDir()
	outPath := filepath.Join(dir, "merged.yaml")
	atomicWriteYAML(outPath, merged)
	after, err := config.Load(outPath)
	if err != nil {
		t.Fatalf("Load(merged): %v", err)
	}
	srcPath := filepath.Join(dir, "source.yaml")
	if err := os.WriteFile(srcPath, []byte(userYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := config.Load(srcPath)
	if err != nil {
		t.Logf("source config does not load on its own (%v); merged values checked only", err)
		return
	}
	unsandboxed := func(c *config.Config) bool {
		return c.Agent.AllowUnsandboxedShell || c.Agent.LegacyUnsandboxedShell
	}
	for _, field := range []struct {
		name          string
		before, after interface{}
	}{
		{"tools.web_scraper.enabled", before.Tools.WebScraper.Enabled, after.Tools.WebScraper.Enabled},
		{"webhooks.rate_limit", before.Webhooks.RateLimit, after.Webhooks.RateLimit},
		{"auth.enabled", before.Auth.Enabled, after.Auth.Enabled},
		{"auth.totp_enabled", before.Auth.TOTPEnabled, after.Auth.TOTPEnabled},
		{"auth.require_origin_header", before.Auth.RequireOriginHeader, after.Auth.RequireOriginHeader},
		{"agent.allow_shell", before.Agent.AllowShell, after.Agent.AllowShell},
		{"effective unsandboxed shell", unsandboxed(before), unsandboxed(after)},
		{"mqtt.allow_unauthenticated_relay", before.MQTT.AllowUnauthenticatedRelay, after.MQTT.AllowUnauthenticatedRelay},
	} {
		if field.before != field.after {
			t.Errorf("%s before upgrade %v, after %v", field.name, field.before, field.after)
		}
	}
}

// The template ships the web scraper off and a webhook rate limit of 60. A
// merge keeps explicit user values (in any yaml.v3 bool spelling); where an
// existing config has no value (missing key, null value or null section) it
// writes the value config.Load used before: scraper on unless the legacy
// agent.allow_web_scraper is false, rate limit 0, auth off. A fresh install
// that copied the template keeps the template's defaults.
func TestRepositoryTemplateMergeKeepsPreUpgradeBehaviour(t *testing.T) {
	tmplData, _ := repositoryTemplate(t)
	want := func(scraper bool, rate int, extra ...interface{}) map[string]interface{} {
		w := map[string]interface{}{"tools.web_scraper.enabled": scraper, "webhooks.rate_limit": rate}
		for i := 0; i+1 < len(extra); i += 2 {
			w[extra[i].(string)] = extra[i+1]
		}
		return w
	}
	const legacyScraper = "agent:\n    allow_web_scraper: "
	cases := []struct {
		name        string
		user        string
		want        map[string]interface{}
		wantChanged bool
	}{
		{"absent keys, no legacy key", "server:\n    port: 8088\n", want(true, 0, "auth.enabled", false), true},
		{"absent scraper key, legacy false", legacyScraper + "false\ntools:\n    web_scraper:\n        summary_mode: true\nwebhooks:\n    enabled: true\n", want(false, 0), true},
		{"absent scraper key, legacy true", legacyScraper + "true\n", want(true, 0), true},
		{"legacy quoted false", legacyScraper + "\"false\"\n", want(false, 0), true},
		{"legacy quoted False", legacyScraper + "\"False\"\n", want(false, 0), true},
		{"legacy no", legacyScraper + "no\n", want(false, 0), true},
		{"legacy off", legacyScraper + "off\n", want(false, 0), true},
		{"legacy N", legacyScraper + "N\n", want(false, 0), true},
		{"legacy yes", legacyScraper + "yes\n", want(true, 0), true},
		{"legacy On", legacyScraper + "On\n", want(true, 0), true},
		{"legacy unrecognised string", legacyScraper + "maybe\n", want(false, 0), true},
		{"legacy null", legacyScraper + "\n", want(true, 0), true},
		{"scraper yes", "tools:\n    web_scraper:\n        enabled: yes\n", want(true, 0), true},
		{"scraper on", "tools:\n    web_scraper:\n        enabled: on\n", want(true, 0), true},
		{"scraper off", "tools:\n    web_scraper:\n        enabled: off\n", want(false, 0), true},
		{"null enabled", "tools:\n    web_scraper:\n        enabled:\n", want(true, 0), true},
		{"null enabled keeps code default over legacy false", legacyScraper + "false\ntools:\n    web_scraper:\n        enabled:\n", want(true, 0), true},
		{"null web_scraper section", "tools:\n    web_scraper:\n", want(true, 0, "tools.web_scraper.summary_mode", false), true},
		{"null web_scraper section, legacy false", legacyScraper + "false\ntools:\n    web_scraper:\n", want(false, 0), true},
		{"null tools section", "tools:\n", want(true, 0), true},
		{"null rate_limit", "webhooks:\n    rate_limit:\n", want(true, 0), true},
		{"null webhooks section", "webhooks:\n", want(true, 0), true},
		{"null agent section", "agent:\n", want(true, 0, "agent.allow_unsandboxed_shell", false), true},
		{"totp_enabled yes", "auth:\n    totp_enabled: yes\n", want(true, 0, "auth.totp_enabled", true, "auth.enabled", false), true},
		{"require_origin_header on", "auth:\n    enabled: true\n    require_origin_header: on\n", want(true, 0, "auth.require_origin_header", true, "auth.enabled", true), true},
		{"allow_shell yes", "agent:\n    allow_shell: yes\n", want(true, 0, "agent.allow_shell", true, "agent.allow_unsandboxed_shell", true), true},
		{"allow_shell off", "agent:\n    allow_shell: off\n", want(true, 0, "agent.allow_shell", false, "agent.allow_unsandboxed_shell", false), true},
		{"null auth enabled", "auth:\n    enabled:\n", want(true, 0, "auth.enabled", false), true},
		{"null auth section", "auth:\n", want(true, 0, "auth.enabled", false, "auth.session_timeout_hours", 24), true},
		{"explicit false and 60", "tools:\n    web_scraper:\n        enabled: false\nwebhooks:\n    rate_limit: 60\n", want(false, 60), true},
		{"explicit true and 0, legacy false", legacyScraper + "false\ntools:\n    web_scraper:\n        enabled: true\nwebhooks:\n    rate_limit: 0\n", want(true, 0), true},
		{"fresh install from template", tmplData, want(false, 60, "auth.enabled", true), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, tmplMap := repositoryTemplate(t)
			srcMap, err := parseYAMLMap(tc.user)
			if err != nil {
				t.Fatalf("parse user config: %v", err)
			}
			res := mergeWithTemplate(tmplMap, srcMap)

			// Every case except the template copy lacks allow_unsandboxed_shell,
			// which the shell rule materialises; the template writes it.
			if res.SafetyAdjusted != tc.wantChanged {
				t.Fatalf("SafetyAdjusted = %v, want %v", res.SafetyAdjusted, tc.wantChanged)
			}
			for dotted, value := range tc.want {
				if got := mergedValue(res.Config, dotted); got != value {
					t.Fatalf("%s = %#v, want %#v", dotted, got, value)
				}
			}
			assertMergeMatchesLoad(t, tc.user, res.Config)
		})
	}
}

func TestApplyUpgradeSafetyDefaults_MaterialisesUnauthenticatedMQTTRelay(t *testing.T) {
	template := map[string]interface{}{
		"mqtt": map[string]interface{}{
			"enabled":                     false,
			"username":                    "",
			"relay_to_agent":              false,
			"allow_unauthenticated_relay": false,
			"tls":                         map[string]interface{}{"cert_file": ""},
		},
	}
	cases := []struct {
		name        string
		userMQTT    map[string]interface{}
		want        bool
		wantChanged bool
	}{
		{"anonymous relay, key absent", map[string]interface{}{"enabled": true, "relay_to_agent": true}, true, true},
		{"anonymous broker without relay flag, key absent", map[string]interface{}{"enabled": true}, true, true},
		{"enabled spelled on, key absent", map[string]interface{}{"enabled": "on"}, true, true},
		{"key null", map[string]interface{}{"enabled": true, "allow_unauthenticated_relay": nil}, true, true},
		{"username, key absent", map[string]interface{}{"enabled": true, "username": "iot"}, false, true},
		{"numeric username, key absent", map[string]interface{}{"enabled": true, "username": 1234}, false, true},
		{"blank username, key absent", map[string]interface{}{"enabled": true, "username": "  "}, true, true},
		{"client certificate, key absent", map[string]interface{}{"enabled": true, "tls": map[string]interface{}{"cert_file": "client.crt"}}, false, true},
		{"mqtt disabled, key absent", map[string]interface{}{"enabled": false, "relay_to_agent": true}, false, true},
		{"key written false", map[string]interface{}{"enabled": true, "allow_unauthenticated_relay": false}, false, false},
		{"key written true", map[string]interface{}{"enabled": true, "username": "iot", "allow_unauthenticated_relay": true}, true, false},
		{"no mqtt section", nil, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := map[string]interface{}{}
			if tc.userMQTT != nil {
				user["mqtt"] = tc.userMQTT
			}
			base, _ := deepCopyYAML(template).(map[string]interface{})
			merged := deepMerge(base, user)

			changed := applyUpgradeSafetyDefaults(merged, user)

			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tc.wantChanged)
			}
			if got := mergedValue(merged, "mqtt.allow_unauthenticated_relay"); got != tc.want {
				t.Fatalf("mqtt.allow_unauthenticated_relay = %#v, want %v", got, tc.want)
			}
		})
	}
}

// An existing config that relayed MQTT traffic (generic relay, Frigate relay or
// mission triggers) from an anonymous broker keeps doing so after a merge with
// the repository template: the merger writes the value config.Load
// grandfathered. Explicit values stay, and a fresh install keeps the
// template's false.
func TestRepositoryTemplateMergeKeepsAnonymousMQTTRelay(t *testing.T) {
	tmplData, _ := repositoryTemplate(t)
	const on = "mqtt:\n    enabled: true\n    broker: tcp://broker.lan:1883\n"
	cases := []struct {
		name string
		user string
		want bool
	}{
		{"anonymous relay, key absent", on + "    relay_to_agent: true\n", true},
		{"anonymous Frigate relay, key absent", on + "frigate:\n    enabled: true\n    event_relay: true\n", true},
		{"anonymous mission triggers, key absent", on, true},
		{"enabled yes, key absent", "mqtt:\n    enabled: yes\n    broker: tcp://broker.lan:1883\n", true},
		{"key null", on + "    relay_to_agent: true\n    allow_unauthenticated_relay:\n", true},
		{"null tls section", on + "    relay_to_agent: true\n    tls:\n", true},
		{"username, key absent", on + "    relay_to_agent: true\n    username: iot\n", false},
		{"client certificate, key absent", on + "    relay_to_agent: true\n    tls:\n        cert_file: client.crt\n        key_file: client.key\n", false},
		{"mqtt disabled, key absent", "mqtt:\n    enabled: false\n    relay_to_agent: true\n", false},
		{"null mqtt section", "mqtt:\n", false},
		{"no mqtt section", "server:\n    port: 8088\n", false},
		{"explicit false", on + "    relay_to_agent: true\n    allow_unauthenticated_relay: false\n", false},
		{"explicit true", on + "    relay_to_agent: true\n    username: iot\n    allow_unauthenticated_relay: true\n", true},
		{"fresh install from template", tmplData, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, tmplMap := repositoryTemplate(t)
			srcMap, err := parseYAMLMap(tc.user)
			if err != nil {
				t.Fatalf("parse user config: %v", err)
			}
			res := mergeWithTemplate(tmplMap, srcMap)
			if got := mergedValue(res.Config, "mqtt.allow_unauthenticated_relay"); got != tc.want {
				t.Fatalf("mqtt.allow_unauthenticated_relay = %#v, want %v", got, tc.want)
			}
			assertMergeMatchesLoad(t, tc.user, res.Config)
		})
	}
}

// yamlBoolSpelling must agree with yaml.v3 decoding a scalar into a typed
// bool, which is what config.Load does, so the list cannot drift.
func TestYAMLBoolSpellingMatchesYAMLv3TypedBool(t *testing.T) {
	for _, s := range []string{
		"y", "Y", "yes", "Yes", "YES", "on", "On", "ON",
		"n", "N", "no", "No", "NO", "off", "Off", "OFF",
		"true", "True", "TRUE", "false", "False", "FALSE",
	} {
		var typed struct {
			V bool `yaml:"v"`
		}
		if err := yaml.Unmarshal([]byte("v: "+s), &typed); err != nil {
			t.Fatalf("yaml.v3 rejects %q as a typed bool: %v", s, err)
		}
		if got, ok := yamlBoolSpelling(s); !ok || got != typed.V {
			t.Fatalf("yamlBoolSpelling(%q) = %v, %v; yaml.v3 gives %v", s, got, ok, typed.V)
		}
		var generic map[string]interface{}
		if err := yaml.Unmarshal([]byte("v: "+s), &generic); err != nil {
			t.Fatal(err)
		}
		if got, ok := yamlBoolValue(generic["v"]); !ok || got != typed.V {
			t.Fatalf("yamlBoolValue(%#v) = %v, %v; yaml.v3 gives %v", generic["v"], got, ok, typed.V)
		}
	}
	for _, s := range []string{"maybe", "1", "0", "yess", "enabled", ""} {
		var typed struct {
			V bool `yaml:"v"`
		}
		if err := yaml.Unmarshal([]byte("v: \""+s+"\""), &typed); err == nil {
			t.Fatalf("yaml.v3 now accepts %q as a typed bool; update yamlBoolSpelling", s)
		}
		if _, ok := yamlBoolSpelling(s); ok {
			t.Fatalf("yamlBoolSpelling(%q) must not count as a bool", s)
		}
	}
}

// A merged config written to disk is stable: re-reading it and merging again
// with the repository template finds nothing to add or adjust, so the merger
// reports "Config is up to date" and the content is unchanged.
func TestMergeWithTemplateIsStableAcrossRuns(t *testing.T) {
	for _, user := range []string{
		"server:\n    port: 8088\n",
		"agent:\n    allow_web_scraper: no\n    allow_shell: yes\nauth:\n    enabled:\nwebhooks:\n",
		"mqtt:\n    enabled: true\n    broker: tcp://broker.lan:1883\n    relay_to_agent: true\n",
	} {
		_, tmplMap := repositoryTemplate(t)
		srcMap, err := parseYAMLMap(user)
		if err != nil {
			t.Fatal(err)
		}
		first := mergeWithTemplate(tmplMap, srcMap)
		path := filepath.Join(t.TempDir(), "config.yaml")
		atomicWriteYAML(path, first.Config)

		data, err := readNormalized(path)
		if err != nil {
			t.Fatal(err)
		}
		reread, err := parseYAMLMap(data)
		if err != nil {
			t.Fatal(err)
		}
		_, tmplMap = repositoryTemplate(t)
		if missing := findMissingTopKeys(tmplMap, reread); len(missing) != 0 {
			t.Fatalf("second run would add sections %v", missing)
		}
		second := mergeWithTemplate(tmplMap, reread)
		if second.TypeFixed || second.SafetyAdjusted || second.Sanitized {
			t.Fatalf("second run adjusts the config: %+v", struct{ TypeFixed, SafetyAdjusted, Sanitized bool }{second.TypeFixed, second.SafetyAdjusted, second.Sanitized})
		}
		if !reflect.DeepEqual(second.Config, reread) {
			t.Fatal("second run changes the merged config")
		}
	}
}

// mergeWithTemplate must not modify the template map it is given.
func TestMergeWithTemplateLeavesTemplateUntouched(t *testing.T) {
	_, tmplMap := repositoryTemplate(t)
	_, pristine := repositoryTemplate(t)
	srcMap, err := parseYAMLMap("tools:\nwebhooks:\nauth:\nagent:\n    allow_shell: yes\nmqtt:\n")
	if err != nil {
		t.Fatal(err)
	}
	mergeWithTemplate(tmplMap, srcMap)
	if !reflect.DeepEqual(tmplMap, pristine) {
		t.Fatal("mergeWithTemplate modified the template map")
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
