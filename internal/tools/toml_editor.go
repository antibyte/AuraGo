package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// TomlEditorResult is the JSON response returned for toml_editor operations.
type TomlEditorResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ExecuteTomlEditor handles TOML file editing operations, sandboxed to workspaceDir.
func ExecuteTomlEditor(operation, filePath, tomlPath string, value interface{}, workspaceDir string) string {
	encode := func(r TomlEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if tomlEditorOperationWrites(operation) {
		if err := requireFilesystemWritePermission(); err != nil {
			return encode(TomlEditorResult{Status: "error", Message: err.Error()})
		}
	}

	if filePath == "" {
		return encode(TomlEditorResult{Status: "error", Message: "'file_path' is required"})
	}

	resolved, err := secureResolve(workspaceDir, filePath)
	if err != nil {
		return encode(TomlEditorResult{Status: "error", Message: err.Error()})
	}

	switch operation {
	case "get":
		return tomlGet(resolved, tomlPath, encode)
	case "set":
		return tomlSet(resolved, tomlPath, value, encode)
	case "delete":
		return tomlDelete(resolved, tomlPath, encode)
	case "keys":
		return tomlKeys(resolved, tomlPath, encode)
	case "validate":
		return tomlValidate(resolved, encode)
	default:
		return encode(TomlEditorResult{Status: "error", Message: fmt.Sprintf("Unknown toml_editor operation '%s'. Valid: get, set, delete, keys, validate", operation)})
	}
}

func tomlEditorOperationWrites(operation string) bool {
	switch strings.TrimSpace(operation) {
	case "set", "delete":
		return true
	default:
		return false
	}
}

func readTomlDocument(path string) (map[string]interface{}, error) {
	var doc map[string]interface{}
	if _, err := toml.DecodeFile(path, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		doc = map[string]interface{}{}
	}
	return doc, nil
}

func tomlGet(resolved, tomlPath string, encode func(TomlEditorResult) string) string {
	if tomlPath == "" {
		return encode(TomlEditorResult{Status: "error", Message: "'toml_path' is required for get"})
	}
	doc, err := readTomlDocument(resolved)
	if err != nil {
		return encode(TomlEditorResult{Status: "error", Message: fmt.Sprintf("Invalid TOML: %v", err)})
	}
	val, found := tomlNavigate(doc, strings.Split(tomlPath, "."))
	if !found {
		return encode(TomlEditorResult{Status: "error", Message: fmt.Sprintf("Path '%s' not found", tomlPath)})
	}
	return encode(TomlEditorResult{Status: "success", Data: val})
}

func tomlSet(resolved, tomlPath string, value interface{}, encode func(TomlEditorResult) string) string {
	if tomlPath == "" {
		return encode(TomlEditorResult{Status: "error", Message: "'toml_path' is required for set"})
	}
	doc, err := readTomlDocument(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			doc = map[string]interface{}{}
		} else {
			return encode(TomlEditorResult{Status: "error", Message: fmt.Sprintf("Invalid TOML: %v", err)})
		}
	}
	if err := tomlSetPath(doc, strings.Split(tomlPath, "."), value); err != nil {
		return encode(TomlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to set path '%s': %v", tomlPath, err)})
	}
	if err := writeTomlDocument(resolved, doc); err != nil {
		return encode(TomlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
	}
	return encode(TomlEditorResult{Status: "success", Message: fmt.Sprintf("Set '%s'", tomlPath)})
}

func tomlDelete(resolved, tomlPath string, encode func(TomlEditorResult) string) string {
	if tomlPath == "" {
		return encode(TomlEditorResult{Status: "error", Message: "'toml_path' is required for delete"})
	}
	doc, err := readTomlDocument(resolved)
	if err != nil {
		return encode(TomlEditorResult{Status: "error", Message: fmt.Sprintf("Invalid TOML: %v", err)})
	}
	if !tomlDeletePath(doc, strings.Split(tomlPath, ".")) {
		return encode(TomlEditorResult{Status: "error", Message: fmt.Sprintf("Path '%s' not found", tomlPath)})
	}
	if err := writeTomlDocument(resolved, doc); err != nil {
		return encode(TomlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
	}
	return encode(TomlEditorResult{Status: "success", Message: fmt.Sprintf("Deleted '%s'", tomlPath)})
}

func tomlKeys(resolved, tomlPath string, encode func(TomlEditorResult) string) string {
	doc, err := readTomlDocument(resolved)
	if err != nil {
		return encode(TomlEditorResult{Status: "error", Message: fmt.Sprintf("Invalid TOML: %v", err)})
	}
	target := interface{}(doc)
	if tomlPath != "" && tomlPath != "." {
		var found bool
		target, found = tomlNavigate(doc, strings.Split(tomlPath, "."))
		if !found {
			return encode(TomlEditorResult{Status: "error", Message: fmt.Sprintf("Path '%s' not found", tomlPath)})
		}
	}
	m, ok := target.(map[string]interface{})
	if !ok {
		return encode(TomlEditorResult{Status: "error", Message: "Target is not a table"})
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return encode(TomlEditorResult{Status: "success", Data: keys})
}

func tomlValidate(resolved string, encode func(TomlEditorResult) string) string {
	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(TomlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}
	var doc map[string]interface{}
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return encode(TomlEditorResult{Status: "error", Message: fmt.Sprintf("Invalid TOML: %v", err)})
	}
	return encode(TomlEditorResult{Status: "success", Message: "Valid TOML"})
}

func writeTomlDocument(path string, doc map[string]interface{}) error {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(doc); err != nil {
		return err
	}
	return writeFileAtomic(path, buf.Bytes())
}

func tomlNavigate(doc map[string]interface{}, keys []string) (interface{}, bool) {
	var current interface{} = doc
	for _, key := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		val, exists := m[key]
		if !exists {
			return nil, false
		}
		current = val
	}
	return current, true
}

func tomlSetPath(doc map[string]interface{}, keys []string, value interface{}) error {
	if len(keys) == 0 {
		return fmt.Errorf("empty path")
	}
	current := doc
	for _, key := range keys[:len(keys)-1] {
		next, ok := current[key]
		if !ok {
			created := map[string]interface{}{}
			current[key] = created
			current = created
			continue
		}
		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return fmt.Errorf("path segment %q is not a table", key)
		}
		current = nextMap
	}
	current[keys[len(keys)-1]] = value
	return nil
}

func tomlDeletePath(doc map[string]interface{}, keys []string) bool {
	if len(keys) == 0 {
		return false
	}
	current := doc
	for _, key := range keys[:len(keys)-1] {
		next, ok := current[key].(map[string]interface{})
		if !ok {
			return false
		}
		current = next
	}
	last := keys[len(keys)-1]
	if _, ok := current[last]; !ok {
		return false
	}
	delete(current, last)
	return true
}
