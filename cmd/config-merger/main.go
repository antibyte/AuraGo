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
		merged := deepMerge(tmplMap, srcMap)
		typeFixed := enforceTemplateTypes(merged, tmplMap)
		sanitized := sanitizeMergedConfig(merged)

		if len(missing) == 0 && !sanitized && !typeFixed {
			fmt.Println("Config is up to date")
			if *outputPath != *sourcePath {
				atomicWriteYAML(*outputPath, merged)
			}
			return
		}

		atomicWriteYAML(*outputPath, merged)
		if len(missing) > 0 {
			sort.Strings(missing)
			fmt.Printf("Added %d new section(s): %s\n", len(missing), strings.Join(missing, ", "))
		}
		if sanitized || typeFixed {
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
		merged = deepMerge(tmplMap, salvaged)
		enforceTemplateTypes(merged, tmplMap)
		sanitizeMergedConfig(merged)
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
	err := yaml.Unmarshal([]byte(content), &m)
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

// topKeyRe matches a top-level YAML key (word at column 0 followed by colon).
var topKeyRe = regexp.MustCompile(`^([a-zA-Z_]\w*)\s*:`)

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

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0644); err != nil {
		log.Fatalf("Failed to write temp file %s: %v", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		log.Fatalf("Failed to rename %s → %s: %v", tmp, path, err)
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
// a corrupted or old-format config), the template default is used instead.
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
				// Try to parse string "true"/"false"
				if s, ok := mergedVal.(string); ok {
					switch strings.ToLower(s) {
					case "true":
						merged[key] = true
					case "false":
						merged[key] = false
					default:
						merged[key] = tmplVal
					}
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
				}
			}
		}
	}

	return changed
}
