package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// YamlEditorResult is the JSON response returned for yaml_editor operations.
type YamlEditorResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ExecuteYamlEditor handles YAML file editing operations, sandboxed to workspaceDir.
func ExecuteYamlEditor(operation, filePath, yamlPath string, value interface{}, workspaceDir string) string {
	encode := func(r YamlEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if filePath == "" {
		return encode(YamlEditorResult{Status: "error", Message: "'file_path' is required"})
	}

	resolved, err := secureResolve(workspaceDir, filePath)
	if err != nil {
		return encode(YamlEditorResult{Status: "error", Message: err.Error()})
	}

	switch operation {
	case "get":
		return yamlGet(resolved, yamlPath, encode)
	case "set":
		return yamlSet(resolved, yamlPath, value, encode)
	case "delete":
		return yamlDelete(resolved, yamlPath, encode)
	case "keys":
		return yamlKeys(resolved, yamlPath, encode)
	case "validate":
		return yamlValidate(resolved, encode)
	default:
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Unknown yaml_editor operation '%s'. Valid: get, set, delete, keys, validate", operation)})
	}
}

// yamlGet reads a value at a dot-separated path from a YAML file.
func yamlGet(resolved, yamlPath string, encode func(YamlEditorResult) string) string {
	if yamlPath == "" {
		return encode(YamlEditorResult{Status: "error", Message: "'json_path' is required for get"})
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	var doc interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Invalid YAML: %v", err)})
	}

	val, found := yamlNavigate(doc, strings.Split(yamlPath, "."))
	if !found {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Path '%s' not found", yamlPath)})
	}

	return encode(YamlEditorResult{Status: "success", Data: val})
}

// yamlSet sets a value at a dot-separated path in a YAML file, preserving comments.
func yamlSet(resolved, yamlPath string, value interface{}, encode func(YamlEditorResult) string) string {
	if yamlPath == "" {
		return encode(YamlEditorResult{Status: "error", Message: "'json_path' is required for set"})
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte("{}")
		} else {
			return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
		}
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Invalid YAML: %v", err)})
	}

	// Ensure we have a document node
	if node.Kind == 0 {
		node = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}

	keys := strings.Split(yamlPath, ".")
	if err := yamlNodeSet(&node, keys, value); err != nil {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to set path '%s': %v", yamlPath, err)})
	}

	out, err := yaml.Marshal(&node)
	if err != nil {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to marshal YAML: %v", err)})
	}

	if err := writeFileAtomic(resolved, out); err != nil {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
	}

	return encode(YamlEditorResult{Status: "success", Message: fmt.Sprintf("Set '%s'", yamlPath)})
}

// yamlDelete removes a key at a dot-separated path from a YAML file.
func yamlDelete(resolved, yamlPath string, encode func(YamlEditorResult) string) string {
	if yamlPath == "" {
		return encode(YamlEditorResult{Status: "error", Message: "'json_path' is required for delete"})
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Invalid YAML: %v", err)})
	}

	keys := strings.Split(yamlPath, ".")
	if !yamlNodeDelete(&node, keys) {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Path '%s' not found", yamlPath)})
	}

	out, err := yaml.Marshal(&node)
	if err != nil {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to marshal YAML: %v", err)})
	}

	if err := writeFileAtomic(resolved, out); err != nil {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
	}

	return encode(YamlEditorResult{Status: "success", Message: fmt.Sprintf("Deleted '%s'", yamlPath)})
}

// yamlKeys lists keys at a dot-separated path in a YAML file.
func yamlKeys(resolved, yamlPath string, encode func(YamlEditorResult) string) string {
	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	var doc interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Invalid YAML: %v", err)})
	}

	var target interface{}
	if yamlPath == "" || yamlPath == "." {
		target = doc
	} else {
		var found bool
		target, found = yamlNavigate(doc, strings.Split(yamlPath, "."))
		if !found {
			return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Path '%s' not found", yamlPath)})
		}
	}

	m, ok := target.(map[string]interface{})
	if !ok {
		return encode(YamlEditorResult{Status: "error", Message: "Target is not a mapping"})
	}

	var keys []string
	for k := range m {
		keys = append(keys, k)
	}

	return encode(YamlEditorResult{Status: "success", Data: keys})
}

// yamlValidate checks if a YAML file is valid.
func yamlValidate(resolved string, encode func(YamlEditorResult) string) string {
	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	var doc interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return encode(YamlEditorResult{Status: "error", Message: fmt.Sprintf("Invalid YAML: %v", err)})
	}

	return encode(YamlEditorResult{Status: "success", Message: "Valid YAML"})
}

// yamlNavigate traverses a decoded YAML structure by a sequence of keys.
func yamlNavigate(doc interface{}, keys []string) (interface{}, bool) {
	current := doc
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

// yamlNodeSet sets a value in a yaml.Node tree, navigating by keys.
func yamlNodeSet(node *yaml.Node, keys []string, value interface{}) error {
	// Unwrap document node
	target := node
	if target.Kind == yaml.DocumentNode {
		if len(target.Content) == 0 {
			target.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
		}
		target = target.Content[0]
	}

	// Navigate/create intermediate nodes
	for i, key := range keys[:len(keys)-1] {
		_ = i
		found := false
		if target.Kind == yaml.MappingNode {
			for j := 0; j < len(target.Content)-1; j += 2 {
				if target.Content[j].Value == key {
					target = target.Content[j+1]
					found = true
					break
				}
			}
		}
		if !found {
			// Create intermediate mapping
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
			valNode := &yaml.Node{Kind: yaml.MappingNode}
			target.Content = append(target.Content, keyNode, valNode)
			target = valNode
		}
	}

	// Set the value at the final key
	finalKey := keys[len(keys)-1]
	valNode := &yaml.Node{}
	if err := valNode.Encode(value); err != nil {
		return fmt.Errorf("failed to encode value: %w", err)
	}

	if target.Kind == yaml.MappingNode {
		for j := 0; j < len(target.Content)-1; j += 2 {
			if target.Content[j].Value == finalKey {
				target.Content[j+1] = valNode
				return nil
			}
		}
		// Key doesn't exist, add it
		target.Content = append(target.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: finalKey},
			valNode,
		)
		return nil
	}

	return fmt.Errorf("cannot set key on non-mapping node")
}

// yamlNodeDelete removes a key from a yaml.Node tree.
func yamlNodeDelete(node *yaml.Node, keys []string) bool {
	target := node
	if target.Kind == yaml.DocumentNode {
		if len(target.Content) == 0 {
			return false
		}
		target = target.Content[0]
	}

	// Navigate to parent
	for _, key := range keys[:len(keys)-1] {
		found := false
		if target.Kind == yaml.MappingNode {
			for j := 0; j < len(target.Content)-1; j += 2 {
				if target.Content[j].Value == key {
					target = target.Content[j+1]
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}

	// Delete the final key
	finalKey := keys[len(keys)-1]
	if target.Kind == yaml.MappingNode {
		for j := 0; j < len(target.Content)-1; j += 2 {
			if target.Content[j].Value == finalKey {
				target.Content = append(target.Content[:j], target.Content[j+2:]...)
				return true
			}
		}
	}
	return false
}
