package ui

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"aurago/internal/config"
)

// wave3SchemaName is the key the config schema gives a field (see buildSchema in
// internal/server): the yaml name, or for vault-only fields the json or vault name.
func wave3SchemaName(field reflect.StructField) string {
	jsonTag := strings.Split(field.Tag.Get("json"), ",")[0]
	if jsonTag == "-" {
		return ""
	}
	yamlTag := strings.Split(field.Tag.Get("yaml"), ",")[0]
	vaultTag := strings.Split(field.Tag.Get("vault"), ",")[0]
	if yamlTag == "-" {
		switch {
		case jsonTag != "":
			return jsonTag
		case vaultTag != "" && vaultTag != "-":
			return vaultTag
		default:
			return ""
		}
	}
	if yamlTag == "" {
		return strings.ToLower(field.Name)
	}
	return yamlTag
}

// wave3VaultPaths are the secret paths the server adds to the config (vaultKeyMap in internal/server).
func wave3VaultPaths(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile("../internal/server/config_handlers_main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ReplaceAll(string(raw), "\r\n", "\n")
	start := strings.Index(source, "var vaultKeyMap = map[string]string{")
	if start < 0 {
		t.Fatal("vaultKeyMap not found")
	}
	block := source[start:]
	block = block[:strings.Index(block, "\n}\n")]
	paths := map[string]bool{}
	for _, match := range regexp.MustCompile(`(?m)^\s*"([a-z0-9_.]+)":`).FindAllStringSubmatch(block, -1) {
		paths[match[1]] = true
	}
	return paths
}

// wave3ConfigType follows a dotted yaml path through the Config struct.
func wave3ConfigType(path string) (reflect.Type, bool) {
	typ := reflect.TypeOf(config.Config{})
	for _, part := range strings.Split(path, ".") {
		for typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct {
			return nil, false
		}
		found := false
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if wave3SchemaName(field) == part {
				typ = field.Type
				found = true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	return typ, true
}

func wave3SectionKeys(t *testing.T) []string {
	t.Helper()
	source := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	start := strings.Index(source, "const SECTIONS = [")
	end := strings.Index(source, "const SECTION_FEATURE_MAP")
	if start < 0 || end < start {
		t.Fatal("cannot find SECTIONS in main.js")
	}
	var keys []string
	for _, match := range regexp.MustCompile(`key: '([a-z0-9_]+)'`).FindAllStringSubmatch(source[start:end], -1) {
		keys = append(keys, match[1])
	}
	if len(keys) < 100 {
		t.Fatalf("found only %d section keys", len(keys))
	}
	return keys
}

// Every Config section resolves to a state rule, and every path of the table exists in the config.
func TestConfigWave3SectionStatePathsContract(t *testing.T) {
	t.Parallel()

	catalog := normalizeAssetText(mustReadUIFile(t, "js/config/catalog.js"))
	start := strings.Index(catalog, "const sectionStatePaths = Object.freeze({")
	if start < 0 {
		t.Fatal("catalog.js has no sectionStatePaths")
	}
	block := catalog[start:]
	block = block[:strings.Index(block, "});")]
	entries := map[string]string{}
	for _, match := range regexp.MustCompile(`(?m)^\s+([a-z0-9_]+): (\{.*\}),?$`).FindAllStringSubmatch(block, -1) {
		entries[match[1]] = match[2]
	}
	if len(entries) < 40 {
		t.Fatalf("parsed only %d sectionStatePaths entries", len(entries))
	}

	sections := map[string]bool{}
	for _, key := range wave3SectionKeys(t) {
		sections[key] = true
		entry, ok := entries[key]
		if ok && strings.Contains(entry, "none: true") {
			continue
		}
		if ok && strings.Contains(entry, "on: [") {
			continue
		}
		// No flag listed: the default "<key>.enabled" must exist and be a switch.
		typ, exists := wave3ConfigType(key + ".enabled")
		if !exists || typ.Kind() != reflect.Bool {
			t.Errorf("section %q has no state rule and %s.enabled is not a config switch", key, key)
		}
	}

	vault := wave3VaultPaths(t)
	actionPaths := regexp.MustCompile(`'([a-z0-9_]+\.[a-z0-9_.]+)'`)
	for key, entry := range entries {
		if !sections[key] {
			t.Errorf("sectionStatePaths names %q, which is not a section", key)
		}
		if strings.Contains(entry, "none: true") && strings.Contains(entry, "on:") {
			t.Errorf("section %q is both none and on", key)
		}
		if on := regexp.MustCompile(`on: \[([^\]]*)\]`).FindStringSubmatch(entry); on != nil {
			for _, path := range actionPaths.FindAllStringSubmatch(on[1], -1) {
				name := strings.TrimPrefix(path[1], "set:")
				typ, exists := wave3ConfigType(name)
				if !exists && vault[name] {
					continue
				}
				if !exists {
					t.Errorf("section %q: path %q is not in the config", key, name)
				} else if !strings.HasPrefix(path[1], "set:") && typ.Kind() != reflect.Bool {
					t.Errorf("section %q: flag %q is not a bool", key, name)
				}
			}
			for _, path := range regexp.MustCompile(`'(set:[a-z0-9_.]+)'`).FindAllStringSubmatch(on[1], -1) {
				if _, exists := wave3ConfigType(strings.TrimPrefix(path[1], "set:")); !exists && !vault[strings.TrimPrefix(path[1], "set:")] {
					t.Errorf("section %q: path %q is not in the config", key, path[1])
				}
			}
		}
		if rule := regexp.MustCompile(`rule: '([^']+)'`).FindStringSubmatch(entry); rule != nil {
			line := regexp.MustCompile(`(?m)^\s*'` + regexp.QuoteMeta(rule[1]) + `': (\{.*\}),?$`).FindStringSubmatch(catalog)
			if line == nil {
				t.Errorf("section %q: actionRules has no %q", key, rule[1])
				continue
			}
			if !strings.Contains(line[1], "credentialPaths") {
				t.Errorf("section %q: rule %q needs a credential (defaults must not look like a setup)", key, rule[1])
			}
			for _, path := range actionPaths.FindAllStringSubmatch(line[1], -1) {
				if _, exists := wave3ConfigType(path[1]); !exists && !vault[path[1]] {
					t.Errorf("rule %q: path %q is not in the config", rule[1], path[1])
				}
			}
		}
	}

	html := normalizeAssetText(mustReadUIFile(t, "config.html"))
	catalogAt := strings.Index(html, "/js/config/catalog.js")
	stateAt := strings.Index(html, "/js/config/section_state.js")
	mainAt := strings.Index(html, "/js/config/main.js")
	if catalogAt < 0 || stateAt < catalogAt || mainAt < stateAt {
		t.Error("config.html must load section_state.js after catalog.js and before main.js")
	}
}

// Every path an action rule waits for exists in the config schema or is a vault secret.
func TestConfigWave3ActionRulePathsExist(t *testing.T) {
	t.Parallel()

	catalog := normalizeAssetText(mustReadUIFile(t, "js/config/catalog.js"))
	start := strings.Index(catalog, "const actionRules = Object.freeze({")
	if start < 0 {
		t.Fatal("catalog.js has no actionRules")
	}
	block := catalog[start:]
	block = block[:strings.Index(block, "});")]
	vault := wave3VaultPaths(t)
	pathPattern := regexp.MustCompile(`'([a-z0-9_]+\.[a-z0-9_.]+)'`)
	checked := 0
	for _, match := range regexp.MustCompile(`(?m)^\s*'([a-z0-9-]+)': (\{.*\}),?$`).FindAllStringSubmatch(block, -1) {
		for _, path := range pathPattern.FindAllStringSubmatch(match[2], -1) {
			checked++
			if _, exists := wave3ConfigType(path[1]); !exists && !vault[path[1]] {
				t.Errorf("rule %q: path %q is not a config field or vault path", match[1], path[1])
			}
		}
	}
	if checked < 50 {
		t.Fatalf("checked only %d action rule paths", checked)
	}
}
