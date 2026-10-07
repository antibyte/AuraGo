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
	"maps"
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

	run(*sourcePath, *templatePath, *outputPath)
}

// run merges the config at sourcePath onto the template at templatePath and
// writes the result to outputPath.
func run(sourcePath, templatePath, outputPath string) {
	ts := time.Now().Format("20060102_150405")

	// ── Step 1: Parse template (must always succeed — this is our controlled default) ──

	tmplData, err := readNormalized(templatePath)
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

	srcData, err := readNormalized(sourcePath)
	if err != nil {
		fmt.Printf("Source unreadable (%v), using template defaults\n", err)
		atomicWriteYAML(outputPath, tmplMap)
		return
	}

	srcMap, parseErr := parseYAMLMap(srcData)

	// An empty or comment-only file has no settings to keep. It is a fresh
	// install (the Docker guide has users `touch config.yaml` before the first
	// start), so it gets the template unchanged like an unreadable source;
	// the corruption path would back it up and apply upgrade grandfathers.
	if parseErr == nil && srcMap == nil {
		fmt.Println("Source has no settings, using template defaults")
		atomicWriteYAML(outputPath, tmplMap)
		return
	}

	if parseErr == nil {
		// ── Happy path: user config is valid YAML ──
		result := mergeUserConfig(tmplMap, srcMap)

		if !result.needsWrite() {
			fmt.Println("Config is up to date")
			if outputPath != sourcePath {
				atomicWriteYAML(outputPath, result.merged)
			}
			return
		}

		atomicWriteYAML(outputPath, result.merged)
		if len(result.missing) > 0 {
			sort.Strings(result.missing)
			fmt.Printf("Added %d new section(s): %s\n", len(result.missing), strings.Join(result.missing, ", "))
		}
		if result.sanitized || result.typeFixed {
			fmt.Println("Applied data shape fixes")
		}
		return
	}

	// ── Corruption path: section-by-section recovery ──

	log.Printf("Config YAML error: %v", parseErr)
	log.Printf("Attempting section-by-section recovery...")

	// Backup corrupted file for manual inspection
	backupPath := sourcePath + "." + ts + ".corrupted"
	if wErr := os.WriteFile(backupPath, []byte(srcData), 0644); wErr == nil {
		log.Printf("Corrupted config saved to: %s", backupPath)
	}

	salvaged := salvageSections(srcData)
	merged := recoverCorruptedConfig(tmplMap, salvaged)
	if len(salvaged) > 0 {
		total := countTopLevelKeys(srcData)
		log.Printf("Recovered %d/%d section(s); template defaults used for the rest", len(salvaged), total)
	} else {
		log.Printf("No sections could be recovered — using full template defaults")
	}

	atomicWriteYAML(outputPath, merged)
	fmt.Println("Config repaired successfully")
}

// mergeResult is the outcome of merging a parseable user config onto the
// template defaults.
type mergeResult struct {
	merged         map[string]interface{}
	missing        []string
	safetyAdjusted bool
	typeFixed      bool
	sanitized      bool
}

// needsWrite reports whether the merged config differs from the user's file.
func (r mergeResult) needsWrite() bool {
	return len(r.missing) > 0 || r.safetyAdjusted || r.typeFixed || r.sanitized
}

// mergeUserConfig merges a parseable user config onto the template defaults.
func mergeUserConfig(tmplMap, srcMap map[string]interface{}) mergeResult {
	result := mergeResult{missing: findMissingTopKeys(tmplMap, srcMap)}
	result.merged = deepMerge(tmplMap, srcMap)
	result.safetyAdjusted = applyUpgradeSafetyDefaults(result.merged, srcMap)
	result.typeFixed = enforceTemplateTypes(result.merged, tmplMap)
	if applyUpgradeGrandfathers(result.merged, srcMap) {
		result.safetyAdjusted = true
	}
	if preserveNewspaperLegacyDefaults(result.merged, srcMap) {
		result.safetyAdjusted = true
	}
	result.sanitized = sanitizeMergedConfig(result.merged)
	return result
}

// recoverCorruptedConfig rebuilds a config whose YAML does not parse as a whole
// from the top-level sections that still parse. The source file existed, so it
// belongs to an existing installation and keeps its grandfathered settings.
// A docker section that could not be salvaged loses an explicit
// allow_host_access: false and comes back as true; Docker itself is reset to
// the template's disabled state in that case.
func recoverCorruptedConfig(tmplMap, salvaged map[string]interface{}) map[string]interface{} {
	if len(salvaged) == 0 {
		// Nothing says how the installation ran, so only the docker grandfather
		// applies; auth, the web scraper, the webhook limit, the unsandboxed
		// shell and the MQTT relay keep the template's safer defaults.
		merged := deepMerge(tmplMap, nil)
		grandfatherDockerHostAccess(merged, nil)
		preserveNewspaperLegacyDefaults(merged, nil)
		return merged
	}
	merged := deepMerge(tmplMap, salvaged)
	applyUpgradeSafetyDefaults(merged, salvaged)
	enforceTemplateTypes(merged, tmplMap)
	applyUpgradeGrandfathers(merged, salvaged)
	preserveNewspaperLegacyDefaults(merged, salvaged)
	sanitizeMergedConfig(merged)
	return merged
}

// preserveNewspaperLegacyDefaults keeps template opt-ins from silently
// affecting an existing installation whose config predates those choices.
// Fresh installs that copy the template directly retain its auto preset.
func preserveNewspaperLegacyDefaults(merged, user map[string]interface{}) bool {
	newspaperMap, ok := asStringMap(merged["newspaper"])
	if !ok {
		return false
	}
	userNewspaper, _ := asStringMap(user["newspaper"])
	changed := false
	if mode, exists := userNewspaper["budget_mode"]; !exists || mode == nil {
		newspaperMap = maps.Clone(newspaperMap)
		newspaperMap["budget_mode"] = config.NewspaperBudgetFixed
		changed = true
	} else if _, valid := mode.(string); !valid {
		newspaperMap = maps.Clone(newspaperMap)
		newspaperMap["budget_mode"] = config.NewspaperBudgetFixed
		changed = true
	}
	if sources, exists := userNewspaper["overview_sources"]; !exists {
		newspaperMap = maps.Clone(newspaperMap)
		newspaperMap["overview_sources"] = []interface{}{}
		changed = true
	} else if _, valid := sources.([]interface{}); !valid {
		newspaperMap = maps.Clone(newspaperMap)
		newspaperMap["overview_sources"] = []interface{}{}
		changed = true
	}
	if changed {
		merged["newspaper"] = newspaperMap
	}
	return changed
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

// applyUpgradeSafetyDefaults prevents dangerous template defaults from silently
// activating features on existing installations when the user config lacks an
// explicit value.
func applyUpgradeSafetyDefaults(merged, user map[string]interface{}) bool {
	changed := false

	// auth.enabled runs again after enforceTemplateTypes (applyUpgradeGrandfathers),
	// which can swap the template's true in for a null value or section.
	if keepAuthDisabledWithoutValue(merged, user) {
		changed = true
	}

	// docker.allow_host_access is new; see grandfatherDockerHostAccess. The
	// merge pipeline applies it again after enforceTemplateTypes
	// (applyUpgradeGrandfathers), which can swap in the template's section.
	if grandfatherDockerHostAccess(merged, user) {
		changed = true
	}

	return changed
}

// applyUpgradeGrandfathers keeps behaviour that predates a new key for configs
// that never wrote it. It runs after enforceTemplateTypes, because that step
// replaces a null or scalar section with the template map, whose values are
// fresh-install defaults.
//
// Each rule fires for the cases config.Load treats as unset for that key (a
// missing key, a null value or a null parent section; see the rule) and reads
// the owner's intent only from the raw user map, never from merged, where
// enforceTemplateTypes has already replaced nulls and mistyped values with
// template defaults. Rules write typed literals into a cloned section, as
// merged may still share the template's section maps. Fresh installs copy the
// template, which writes every key, so they keep the template's safer defaults.
func applyUpgradeGrandfathers(merged, user map[string]interface{}) bool {
	changed := false
	for _, rule := range []func(merged, user map[string]interface{}) bool{
		keepAuthDisabledWithoutValue,
		grandfatherDockerHostAccess,
		grandfatherUnsandboxedShell,
		grandfatherWebScraperAndWebhookLimit,
		grandfatherUnauthenticatedMQTTRelay,
	} {
		if rule(merged, user) {
			changed = true
		}
	}
	return changed
}

// keepAuthDisabledWithoutValue: config.Load has no auth.enabled default, so a
// config without a value there (missing, null, or under a null auth section)
// runs without login; the template's true must not lock the owner out.
func keepAuthDisabledWithoutValue(merged, user map[string]interface{}) bool {
	authMap, ok := asStringMap(merged["auth"])
	if !ok {
		return false
	}
	userAuth, _ := asStringMap(user["auth"])
	if userAuth["enabled"] != nil || authMap["enabled"] != true {
		return false
	}
	authMap = maps.Clone(authMap)
	authMap["enabled"] = false
	merged["auth"] = authMap
	return true
}

// grandfatherUnsandboxedShell: the non-Windows host shell without a sandbox
// now needs agent.allow_unsandboxed_shell (or allow_unsafe_host_execution).
// Materialise the key on every upgrade so a merged config never relies on the
// load-time grandfather: true where the shell was already enabled (in any
// yaml.v3 bool spelling, as Load reads allow_shell), false otherwise. A
// written key, even null, is the owner's choice, as config.Load reads it.
func grandfatherUnsandboxedShell(merged, user map[string]interface{}) bool {
	userAgent, _ := asStringMap(user["agent"])
	if _, userSetUnsandboxed := userAgent["allow_unsandboxed_shell"]; userSetUnsandboxed {
		return false
	}
	agentMap, ok := asStringMap(merged["agent"])
	if !ok {
		return false
	}
	shellOn, _ := yamlBoolValue(userAgent["allow_shell"])
	agentMap = maps.Clone(agentMap)
	agentMap["allow_unsandboxed_shell"] = shellOn
	merged["agent"] = agentMap
	return true
}

// grandfatherWebScraperAndWebhookLimit keeps two template defaults from
// changing an existing installation.
//
// The template ships tools.web_scraper.enabled: false. A config without a
// value there ran with the scraper on unless the legacy agent.allow_web_scraper
// said otherwise, so write that effective value, mirroring config.Load: the
// code default is true, a null enabled leaves it (and, being present, skips
// the legacy migration), and the legacy key wins only when enabled is absent,
// including under a null tools or web_scraper section.
//
// The template ships webhooks.rate_limit: 60. A config without a value there
// (absent, null, or under a null webhooks section) ran unlimited (0); keep
// that, the webhooks_no_rate_limit security hint still reports it on
// internet-facing instances.
func grandfatherWebScraperAndWebhookLimit(merged, user map[string]interface{}) bool {
	changed := false
	userAgent, _ := asStringMap(user["agent"])
	userTools, _ := asStringMap(user["tools"])
	userScraper, _ := asStringMap(userTools["web_scraper"])
	if enabled, hasEnabled := userScraper["enabled"]; enabled == nil {
		if toolsMap, ok := asStringMap(merged["tools"]); ok {
			if scraperMap, ok := asStringMap(toolsMap["web_scraper"]); ok {
				scraperMap = maps.Clone(scraperMap)
				scraperMap["enabled"] = hasEnabled || legacyWebScraperEnabled(userAgent)
				toolsMap = maps.Clone(toolsMap)
				toolsMap["web_scraper"] = scraperMap
				merged["tools"] = toolsMap
				changed = true
			}
		}
	}

	userWebhooks, _ := asStringMap(user["webhooks"])
	if userWebhooks["rate_limit"] == nil {
		if webhooksMap, ok := asStringMap(merged["webhooks"]); ok {
			webhooksMap = maps.Clone(webhooksMap)
			webhooksMap["rate_limit"] = 0
			merged["webhooks"] = webhooksMap
			changed = true
		}
	}
	return changed
}

// grandfatherUnauthenticatedMQTTRelay: MQTT relays and MQTT-triggered missions
// now need broker authentication or mqtt.allow_unauthenticated_relay; the
// template ships false. A config without a value there (absent, null, or under
// a null mqtt section) ran them on any broker, so materialise what config.Load
// grandfathers: true for an enabled broker without username or client
// certificate over TLS (mission triggers live in the mission store, so no
// relay flag is needed), false otherwise. The mqtt_relay_no_auth security hint
// reports true as critical.
func grandfatherUnauthenticatedMQTTRelay(merged, user map[string]interface{}) bool {
	userMQTT, _ := asStringMap(user["mqtt"])
	if userMQTT["allow_unauthenticated_relay"] != nil {
		return false
	}
	mqttMap, ok := asStringMap(merged["mqtt"])
	if !ok {
		return false
	}
	mqttMap = maps.Clone(mqttMap)
	mqttMap["allow_unauthenticated_relay"] = mqttRanAnonymously(userMQTT)
	merged["mqtt"] = mqttMap
	return true
}

// mqttRanAnonymously mirrors the config.Load grandfather for
// mqtt.allow_unauthenticated_relay: MQTT enabled (in any yaml.v3 bool
// spelling) with neither a username nor a client certificate over TLS
// (config.MQTTBrokerAuthenticated). Whether the certificate is presented
// depends on mqtt.broker and mqtt.tls.enabled, so it asks
// config.MQTTEffectiveTLS with exactly those values.
func mqttRanAnonymously(userMQTT map[string]interface{}) bool {
	if enabled, _ := yamlBoolValue(userMQTT["enabled"]); !enabled {
		return false
	}
	if yamlStringSet(userMQTT["username"]) {
		return false
	}
	userTLS, _ := asStringMap(userMQTT["tls"])
	if !yamlStringSet(userTLS["cert_file"]) {
		return true
	}
	var probe config.Config
	probe.MQTT.Broker, _ = userMQTT["broker"].(string)
	probe.MQTT.TLS.Enabled, _ = yamlBoolValue(userTLS["enabled"])
	tlsOn, err := config.MQTTEffectiveTLS(&probe)
	return err != nil || !tlsOn
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

// grandfatherDockerHostAccess materialises docker.allow_host_access: true when
// the user config never wrote the key. Configurations that predate it keep the
// unrestricted agent Compose behaviour, so the template's false (meant for
// fresh installs) never reaches an upgraded config.
func grandfatherDockerHostAccess(merged, user map[string]interface{}) bool {
	userDocker, _ := asStringMap(user["docker"])
	if _, userSetHostAccess := userDocker["allow_host_access"]; userSetHostAccess {
		return false
	}
	dockerMap, ok := asStringMap(merged["docker"])
	if !ok {
		return false
	}
	// merged may still share the template's section map; never write into it.
	dockerMap = maps.Clone(dockerMap)
	dockerMap["allow_host_access"] = true
	merged["docker"] = dockerMap
	return true
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
