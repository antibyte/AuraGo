// Config-Merger V5 — Bulletproof YAML-aware config merging for AuraGo updates.
//
// Strategy:
//  1. Parse template (our controlled default) into a generic YAML map.
//  2. Parse user config into a generic YAML map.
//     a. SUCCESS → deep-merge: user values overlay template defaults.
//     b. FAILURE → split into top-level sections, parse each individually,
//     salvage every section that parses, skip corrupted ones.
//  3. Deep-merge salvaged/parsed user values onto template defaults.
//  4. Marshal result via yaml.Marshal → output is always valid YAML.
//
// This guarantees:
//   - Output is always syntactically valid (marshalled from parsed data).
//   - New config keys from template are added automatically (including nested).
//   - User values are preserved whenever they are parseable.
//   - Corrupted sections are isolated; they don't break other sections.
//   - Corrupted files are backed up for manual inspection.
package main

import (
	"aurago/internal/config"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func main() {
	sourcePath := flag.String("source", "", "Path to the existing config.yaml")
	templatePath := flag.String("template", "", "Path to the template config.yaml (upstream defaults)")
	outputPath := flag.String("output", "", "Path to save merged config (defaults to source)")
	flag.Parse()

	if *sourcePath == "" || *templatePath == "" {
		flag.Usage()
		os.Exit(1)
	}
	if *outputPath == "" {
		*outputPath = *sourcePath
	}

	ts := time.Now().Format("20060102_150405")

	// ── Step 1: Parse template (must always succeed — this is our controlled default) ──

	tmplData, err := readNormalized(*templatePath)
	if err != nil {
		log.Fatalf("Cannot read template: %v", err)
	}
	tmplMap, err := parseYAMLMap(tmplData)
	if err != nil {
		log.Fatalf("Template YAML is invalid (this is a release bug): %v", err)
	}
	if tmplMap == nil {
		log.Fatalf("Template config is empty")
	}

	// ── Step 2: Parse user config ──

	srcData, err := readNormalized(*sourcePath)
	if err != nil {
		fmt.Printf("Source unreadable (%v), using template defaults\n", err)
		atomicWriteYAML(*outputPath, tmplMap)
		return
	}

	srcMap, parseErr := parseYAMLMap(srcData)

	if parseErr == nil && srcMap != nil {
		// ── Happy path: user config is valid YAML ──
		missing := findMissingTopKeys(tmplMap, srcMap)
		res := mergeWithTemplate(tmplMap, srcMap)

		if len(missing) == 0 && !res.Sanitized && !res.TypeFixed && !res.SafetyAdjusted {
			fmt.Println("Config is up to date")
			if *outputPath != *sourcePath {
				atomicWriteYAML(*outputPath, res.Config)
			}
			return
		}

		atomicWriteYAML(*outputPath, res.Config)
		if len(missing) > 0 {
			sort.Strings(missing)
			fmt.Printf("Added %d new section(s): %s\n", len(missing), strings.Join(missing, ", "))
		}
		if res.Sanitized || res.TypeFixed {
			fmt.Println("Applied data shape fixes")
		}
		return
	}

	// ── Corruption path: section-by-section recovery ──

	log.Printf("Config YAML error: %v", parseErr)
	log.Printf("Attempting section-by-section recovery...")

	// Backup corrupted file for manual inspection
	backupPath := *sourcePath + "." + ts + ".corrupted"
	if wErr := os.WriteFile(backupPath, []byte(srcData), 0644); wErr == nil {
		log.Printf("Corrupted config saved to: %s", backupPath)
	}

	salvaged := salvageSections(srcData)

	var merged map[string]interface{}
	if len(salvaged) > 0 {
		merged = mergeWithTemplate(tmplMap, salvaged).Config
		total := countTopLevelKeys(srcData)
		log.Printf("Recovered %d/%d section(s); template defaults used for the rest", len(salvaged), total)
	} else {
		merged = tmplMap
		log.Printf("No sections could be recovered — using full template defaults")
	}

	atomicWriteYAML(*outputPath, merged)
	fmt.Println("Config repaired successfully")
}

// ── YAML Helpers ─────────────────────────────────────────────────────────────

// readNormalized reads a file and normalizes line endings to LF, tabs to spaces.
func readNormalized(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	// Tabs → 4 spaces (common hand-edit mistake that breaks YAML)
	s = strings.ReplaceAll(s, "\t", "    ")
	return s, nil
}

// parseYAMLMap unmarshals YAML content into a generic map.
func parseYAMLMap(content string) (map[string]interface{}, error) {
	var m map[string]interface{}
	// Migrate the user document before overlaying template defaults, otherwise a
	// new default would incorrectly win over an explicit value on an old path.
	normalized, err := config.NormalizeToolDisclosureConfig([]byte(content))
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(normalized, &m)
	return m, err
}

// ── Deep Merge ───────────────────────────────────────────────────────────────

// mergeResult is the merged config and which fix-up passes changed it.
type mergeResult struct {
	Config         map[string]interface{}
	TypeFixed      bool
	SafetyAdjusted bool
	Sanitized      bool
}

// mergeWithTemplate overlays user onto a deep copy of tmpl and applies the
// fixes in order: enforceTemplateTypes first, so a null or mistyped user value
// or section falls back to the template's shape; then
// applyUpgradeSafetyDefaults, which decides from the raw user config and can
// therefore still see such a null and keep the pre-upgrade behaviour; then
// sanitizeMergedConfig. deepMerge and the fix-ups share and mutate nested
// template maps, so the copy keeps tmpl itself unmodified.
func mergeWithTemplate(tmpl, user map[string]interface{}) mergeResult {
	base, _ := deepCopyYAML(tmpl).(map[string]interface{})
	merged := deepMerge(base, user)
	res := mergeResult{Config: merged}
	res.TypeFixed = enforceTemplateTypes(merged, base)
	res.SafetyAdjusted = applyUpgradeSafetyDefaults(merged, user)
	res.Sanitized = sanitizeMergedConfig(merged)
	return res
}

// deepCopyYAML copies the maps and slices of a parsed YAML value.
func deepCopyYAML(v interface{}) interface{} {
	switch v := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for k, e := range v {
			out[k] = deepCopyYAML(e)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, e := range v {
			out[i] = deepCopyYAML(e)
		}
		return out
	default:
		return v
	}
}

// deepMerge recursively merges overlay into base and returns a new map.
//   - Keys in both: overlay wins (recurse for nested maps).
//   - Keys only in base: kept (new options from template).
//   - Keys only in overlay: kept (user's custom additions).
func deepMerge(base, overlay map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(base)+len(overlay))

	// Copy all base (template) keys first
	for k, v := range base {
		result[k] = v
	}

	// Apply overlay (user) values
	for k, ov := range overlay {
		bv, inBase := result[k]
		if !inBase {
			// User-only key: preserve
			result[k] = ov
			continue
		}

		bMap, bIsMap := asStringMap(bv)
		oMap, oIsMap := asStringMap(ov)

		if bIsMap && oIsMap {
			result[k] = deepMerge(bMap, oMap)
		} else {
			// Leaf value or type mismatch: user wins
			result[k] = ov
		}
	}

	return result
}

// applyUpgradeSafetyDefaults keeps template defaults from silently changing
// behaviour on existing installations when the user config has no value for
// a key: dangerous defaults must not activate features, and new gates must not
// switch off what the installation already used. Each rule fires for a
// missing key, a null value or a null parent section, all of which
// config.Load treats as unset; fresh installs copy the template, which writes
// every key, so they keep the template's safer defaults.
//
// Contract: the rules read the owner's intent only from the raw user map
// (never from merged, where enforceTemplateTypes has already replaced nulls
// and mistyped values with template defaults) and write only typed literals
// into merged, because type enforcement has already run (see
// mergeWithTemplate).
func applyUpgradeSafetyDefaults(merged, user map[string]interface{}) bool {
	changed := false

	// config.Load has no auth.enabled default: a config without a value there
	// runs without login, so the template's true must not lock the owner out.
	// Indexing a nil map covers a missing or null auth section too.
	if authMap, ok := asStringMap(merged["auth"]); ok {
		userAuth, _ := asStringMap(user["auth"])
		if userAuth["enabled"] == nil && authMap["enabled"] == true {
			authMap["enabled"] = false
			merged["auth"] = authMap
			changed = true
		}
	}

	// The non-Windows host shell without a sandbox now needs
	// agent.allow_unsandboxed_shell (or allow_unsafe_host_execution). Materialise
	// the key on every upgrade so a merged config never relies on the load-time
	// grandfather: true where the shell was already enabled (in any yaml.v3
	// bool spelling, as Load reads allow_shell), false otherwise.
	userAgent, _ := asStringMap(user["agent"])
	if _, userSetUnsandboxed := userAgent["allow_unsandboxed_shell"]; !userSetUnsandboxed {
		if agentMap, ok := asStringMap(merged["agent"]); ok {
			shellOn, _ := yamlBoolValue(userAgent["allow_shell"])
			agentMap["allow_unsandboxed_shell"] = shellOn
			merged["agent"] = agentMap
			changed = true
		}
	}

	// The template ships tools.web_scraper.enabled: false. A config without a
	// value there ran with the scraper on unless the legacy
	// agent.allow_web_scraper said otherwise, so write that effective value,
	// mirroring config.Load: the code default is true, a null enabled leaves
	// it (and, being present, skips the legacy migration), and the legacy key
	// wins only when enabled is absent, including under a null tools or
	// web_scraper section.
	userTools, _ := asStringMap(user["tools"])
	userScraper, _ := asStringMap(userTools["web_scraper"])
	if enabled, hasEnabled := userScraper["enabled"]; enabled == nil {
		if toolsMap, ok := asStringMap(merged["tools"]); ok {
			if scraperMap, ok := asStringMap(toolsMap["web_scraper"]); ok {
				scraperMap["enabled"] = hasEnabled || legacyWebScraperEnabled(userAgent)
				toolsMap["web_scraper"] = scraperMap
				merged["tools"] = toolsMap
				changed = true
			}
		}
	}

	// The template ships webhooks.rate_limit: 60. A config without a value
	// there (absent, null, or under a null webhooks section) ran unlimited (0);
	// keep that, the webhooks_no_rate_limit security hint still reports it on
	// internet-facing instances.
	userWebhooks, _ := asStringMap(user["webhooks"])
	if userWebhooks["rate_limit"] == nil {
		if webhooksMap, ok := asStringMap(merged["webhooks"]); ok {
			webhooksMap["rate_limit"] = 0
			merged["webhooks"] = webhooksMap
			changed = true
		}
	}

	// MQTT relays and MQTT-triggered missions now need broker authentication
	// or mqtt.allow_unauthenticated_relay; the template ships false. A config
	// without a value there (absent, null, or under a null mqtt section) ran
	// them on any broker, so materialise what config.Load grandfathers: true
	// for an enabled broker without username or client certificate (mission
	// triggers live in the mission store, so no relay flag is needed), false
	// otherwise. The mqtt_relay_no_auth security hint reports true as critical.
	userMQTT, _ := asStringMap(user["mqtt"])
	if userMQTT["allow_unauthenticated_relay"] == nil {
		if mqttMap, ok := asStringMap(merged["mqtt"]); ok {
			mqttMap["allow_unauthenticated_relay"] = mqttRanAnonymously(userMQTT)
			merged["mqtt"] = mqttMap
			changed = true
		}
	}

	return changed
}

// mqttRanAnonymously mirrors the config.Load grandfather for
// mqtt.allow_unauthenticated_relay: MQTT enabled (in any yaml.v3 bool
// spelling) with neither a username nor a tls.cert_file.
func mqttRanAnonymously(userMQTT map[string]interface{}) bool {
	if enabled, _ := yamlBoolValue(userMQTT["enabled"]); !enabled {
		return false
	}
	userTLS, _ := asStringMap(userMQTT["tls"])
	return !yamlStringSet(userMQTT["username"]) && !yamlStringSet(userTLS["cert_file"])
}

// yamlStringSet reports whether a parsed user-config value reaches a string
// field in config.Load as non-blank: a string with non-space content, or any
// other scalar, which yaml.v3 decodes as written. Null is unset.
func yamlStringSet(v interface{}) bool {
	switch v := v.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(v) != ""
	default:
		return true
	}
}

// legacyWebScraperEnabled returns the scraper state config.Load derived for a
// config without tools.web_scraper.enabled: the code default true when the
// legacy agent.allow_web_scraper is absent or null, else its value in any
// yaml.v3 bool spelling. Any other value counts as false, the safe side, as
// enforceTemplateTypes falls back to the template's false for it.
func legacyWebScraperEnabled(userAgent map[string]interface{}) bool {
	v := userAgent["allow_web_scraper"]
	if v == nil {
		return true
	}
	enabled, ok := yamlBoolValue(v)
	return ok && enabled
}

// yamlBoolSpelling interprets s the way yaml.v3 does when it decodes a scalar
// into a typed bool (config.Load): the YAML 1.1 forms y/yes/on and n/no/off in
// the three spellings yaml.v3 lists, which a generic map keeps as strings,
// and true/false, accepted here in any case as enforceTemplateTypes always
// did for quoted values.
func yamlBoolSpelling(s string) (value, ok bool) {
	switch s {
	case "y", "Y", "yes", "Yes", "YES", "on", "On", "ON":
		return true, true
	case "n", "N", "no", "No", "NO", "off", "Off", "OFF":
		return false, true
	}
	switch strings.ToLower(s) {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// yamlBoolValue reports the bool a parsed user-config value stands for: a
// bool, or a string yamlBoolSpelling accepts.
func yamlBoolValue(v interface{}) (value, ok bool) {
	switch v := v.(type) {
	case bool:
		return v, true
	case string:
		return yamlBoolSpelling(v)
	default:
		return false, false
	}
}

// asStringMap converts a value to map[string]interface{} if possible.
// yaml.v3 always produces map[string]interface{}, but we handle both forms
// for robustness.
func asStringMap(v interface{}) (map[string]interface{}, bool) {
	switch m := v.(type) {
	case map[string]interface{}:
		return m, true
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, v := range m {
			out[fmt.Sprint(k)] = v
		}
		return out, true
	}
	return nil, false
}

// ── Section Salvage (Corruption Recovery) ────────────────────────────────────

// salvageSections splits corrupted YAML into top-level sections, tries to parse
// each independently, and returns a merged map of all sections that succeed.
func salvageSections(content string) map[string]interface{} {
	result := make(map[string]interface{})
	sections := splitTopLevelSections(content)

	// Sort for deterministic log output
	names := make([]string, 0, len(sections))
	for name := range sections {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		sectionText := sections[name]
		parsed, err := parseYAMLMap(sectionText)
		if err != nil {
			log.Printf("  ✗ Section '%s': corrupted (%s)", name, truncate(err.Error(), 80))
			continue
		}
		if parsed == nil {
			continue
		}
		for k, v := range parsed {
			result[k] = v
			log.Printf("  ✓ Section '%s': recovered", k)
		}
	}

	return result
}

// topKeyRe matches a top-level YAML key at column 0, including common dashed keys.
var topKeyRe = regexp.MustCompile(`^([a-zA-Z_][\w-]*)\s*:`)

// splitTopLevelSections splits YAML content into chunks by top-level key.
// Each chunk includes the key line and all following indented/blank/comment lines.
func splitTopLevelSections(content string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(content, "\n")

	var curName string
	var curLines []string

	flush := func() {
		if curName != "" {
			sections[curName] = strings.Join(curLines, "\n")
		}
	}

	for _, line := range lines {
		if m := topKeyRe.FindStringSubmatch(line); m != nil {
			flush()
			curName = m[1]
			curLines = []string{line}
		} else if curName != "" {
			curLines = append(curLines, line)
		}
		// Lines before the first top-level key (e.g. file-level comments) are skipped.
	}
	flush()

	return sections
}

// ── Output ───────────────────────────────────────────────────────────────────

// atomicWriteYAML marshals a map to YAML and writes atomically (tmp → rename).
func atomicWriteYAML(path string, data map[string]interface{}) {
	out, err := yaml.Marshal(data)
	if err != nil {
		log.Fatalf("Failed to marshal merged config: %v", err)
	}

	if err := config.WriteFileAtomic(path, out, 0o600); err != nil {
		log.Fatalf("Failed to write merged config %s: %v", path, err)
	}
}

// ── Utilities ────────────────────────────────────────────────────────────────

// findMissingTopKeys returns top-level keys in tmpl that are absent from src.
func findMissingTopKeys(tmpl, src map[string]interface{}) []string {
	var missing []string
	for k := range tmpl {
		if _, ok := src[k]; !ok {
			missing = append(missing, k)
		}
	}
	return missing
}

// countTopLevelKeys counts lines that look like top-level YAML keys.
func countTopLevelKeys(content string) int {
	n := 0
	for _, line := range strings.Split(content, "\n") {
		if topKeyRe.MatchString(line) {
			n++
		}
	}
	return n
}

// truncate shortens a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// enforceTemplateTypes recursively walks merged and compares leaf value types
// against the template. When the template defines a specific type (array, map,
// bool, int) but the merged value has a different type (typically a string from
// a corrupted or old-format config), the template default is used instead;
// bool slots keep strings in a yaml.v3 bool spelling (yamlBoolSpelling).
// This prevents type mismatches from causing config.Load() unmarshal errors.
func enforceTemplateTypes(merged, tmpl map[string]interface{}) bool {
	fixed := false
	for key, tmplVal := range tmpl {
		mergedVal, exists := merged[key]
		if !exists {
			continue
		}

		tMap, tIsMap := asStringMap(tmplVal)
		mMap, mIsMap := asStringMap(mergedVal)

		if tIsMap && mIsMap {
			// Both maps: recurse
			if enforceTemplateTypes(mMap, tMap) {
				fixed = true
			}
			continue
		}

		if tIsMap && !mIsMap {
			// Template has a map but merged has a scalar — use template
			log.Printf("enforceTypes: %s should be a map, got %T — using template default", key, mergedVal)
			merged[key] = tmplVal
			fixed = true
			continue
		}

		// Template is a leaf: check type compatibility
		switch tmplVal.(type) {
		case []interface{}:
			if _, ok := mergedVal.([]interface{}); !ok {
				log.Printf("enforceTypes: %s should be an array, got %T — using template default", key, mergedVal)
				merged[key] = tmplVal
				fixed = true
			}
		case bool:
			if _, ok := mergedVal.(bool); !ok {
				// A string in a yaml.v3 bool spelling (yes/on/no/off/...,
				// true/false) keeps the owner's value, as config.Load would
				// read it; anything else falls back to the template.
				if b, ok := yamlBoolValue(mergedVal); ok {
					merged[key] = b
				} else {
					merged[key] = tmplVal
				}
				log.Printf("enforceTypes: %s should be bool, got %T — corrected", key, mergedVal)
				fixed = true
			}
		case int:
			if _, ok := mergedVal.(int); !ok {
				log.Printf("enforceTypes: %s should be int, got %T — using template default", key, mergedVal)
				merged[key] = tmplVal
				fixed = true
			}
		case float64:
			switch mergedVal.(type) {
			case float64, int:
				// Compatible numeric types — keep user value
			default:
				log.Printf("enforceTypes: %s should be numeric, got %T — using template default", key, mergedVal)
				merged[key] = tmplVal
				fixed = true
			}
		}
	}
	return fixed
}

// sanitizeMergedConfig fixes known data shape problems that would cause
// config.Load() to fail with unmarshal errors. Returns true if any fix was applied.
//
// Known issues:
//   - budget.models must be []map; plain strings (legacy format before V5) → reset to []
//   - Various []string fields must not be strings (e.g. allowed_paths, directories)
func sanitizeMergedConfig(m map[string]interface{}) bool {
	changed := false

	if budget, ok := asStringMap(m["budget"]); ok {
		if models, exists := budget["models"]; exists {
			if items, ok := models.([]interface{}); ok {
				for _, item := range items {
					if _, isMap := asStringMap(item); !isMap {
						// At least one item is a plain string (or other non-map scalar).
						// These are model names without cost definitions — an old config
						// format that ModelCost cannot unmarshal. Reset to empty list so
						// the application starts; costs can be re-added via the Web UI.
						budget["models"] = []interface{}{}
						log.Printf("sanitize: budget.models had plain-string items (old format) — reset to []")
						changed = true
						break
					}
				}
			}
		}
	}

	// Ensure known []string fields are actually arrays, not strings.
	// This catches corruption from old Web UI saves or manual edits.
	arrayFields := []struct {
		section string
		key     string
	}{
		{"remote_control", "allowed_paths"},
		{"indexing", "directories"},
		{"indexing", "extensions"},
		{"github", "allowed_repos"},
		{"mqtt", "topics"},
		{"circuit_breaker", "retry_intervals"},
		{"meshcentral", "blocked_operations"},
	}
	for _, af := range arrayFields {
		if sec, ok := asStringMap(m[af.section]); ok {
			if val, exists := sec[af.key]; exists {
				if _, isArr := val.([]interface{}); !isArr {
					log.Printf("sanitize: %s.%s should be an array, got %T — reset to []", af.section, af.key, val)
					sec[af.key] = []interface{}{}
					changed = true
				} else if af.section == "indexing" && af.key == "directories" {
					// indexing.directories entries must be {path: "...", collection: "..."} objects,
					// not bare strings like "- ./knowledge". Fix each string item by wrapping it.
					arr := val.([]interface{})
					for i, item := range arr {
						if _, isMap := asStringMap(item); !isMap {
							// Bare string — convert to {path: item}
							log.Printf("sanitize: indexing.directories[%d] is a bare string %q — wrapping as {{path: %q}}", i, item, item)
							arr[i] = map[string]interface{}{"path": item}
							changed = true
						}
					}
				}
			}
		}
	}

	return changed
}
