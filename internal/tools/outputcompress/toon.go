package outputcompress

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

func normalizeTOONJSONConfig(cfg TOONJSONConfig) TOONJSONConfig {
	if cfg.MinSavingsPercent <= 0 {
		cfg.MinSavingsPercent = 10
	}
	if cfg.MaxRows <= 0 {
		cfg.MaxRows = 200
	}
	return cfg
}

func compressTOONJSON(toolName, output string, cfg TOONJSONConfig) (string, bool) {
	cfg = normalizeTOONJSONConfig(cfg)
	if !cfg.Enabled || !isTOONAllowedTool(toolName) {
		return "", false
	}
	trimmed := strings.TrimSpace(output)
	if !strings.HasPrefix(trimmed, "[") {
		return "", false
	}

	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.UseNumber()
	var rows []map[string]interface{}
	if err := decoder.Decode(&rows); err != nil {
		return "", false
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", false
	}
	if len(rows) < 2 || len(rows) > cfg.MaxRows {
		return "", false
	}

	keys, ok := homogeneousScalarKeys(rows)
	if !ok || len(keys) == 0 {
		return "", false
	}

	var b strings.Builder
	fmt.Fprintf(&b, "[toon-json rows=%d cols=%d]\n", len(rows), len(keys))
	b.WriteString(strings.Join(keys, "|"))
	b.WriteByte('\n')
	for _, row := range rows {
		for i, key := range keys {
			if i > 0 {
				b.WriteByte('|')
			}
			b.WriteString(formatTOONValue(row[key]))
		}
		b.WriteByte('\n')
	}
	result := strings.TrimRight(b.String(), "\n")
	if !meetsSavingsThreshold(output, result, cfg.MinSavingsPercent) {
		return "", false
	}
	return result, true
}

func isTOONAllowedTool(toolName string) bool {
	switch toolName {
	case "docker", "docker_compose", "proxmox", "homeassistant", "home_assistant",
		"kubernetes", "github", "sql_query", "list_processes":
		return true
	default:
		return false
	}
}

func homogeneousScalarKeys(rows []map[string]interface{}) ([]string, bool) {
	if len(rows) == 0 || len(rows[0]) == 0 {
		return nil, false
	}
	keys := make([]string, 0, len(rows[0]))
	for key, value := range rows[0] {
		if !isScalarTOONValue(value) {
			return nil, false
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, row := range rows[1:] {
		if len(row) != len(keys) {
			return nil, false
		}
		for _, key := range keys {
			value, exists := row[key]
			if !exists || !isScalarTOONValue(value) {
				return nil, false
			}
		}
	}
	return keys, true
}

func isScalarTOONValue(value interface{}) bool {
	switch value.(type) {
	case nil, string, bool, json.Number, float64:
		return true
	default:
		return false
	}
}

func formatTOONValue(value interface{}) string {
	if value == nil {
		return ""
	}
	var raw []byte
	switch v := value.(type) {
	case json.Number:
		raw = []byte(v.String())
	case string:
		encoded, _ := json.Marshal(v)
		raw = encoded
	case bool:
		if v {
			raw = []byte("true")
		} else {
			raw = []byte("false")
		}
	default:
		encoded, _ := json.Marshal(v)
		raw = encoded
	}
	raw = bytes.ReplaceAll(raw, []byte("\n"), []byte(`\n`))
	raw = bytes.ReplaceAll(raw, []byte("|"), []byte(`\|`))
	return string(raw)
}
