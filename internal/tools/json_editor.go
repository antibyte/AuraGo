package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// JsonEditorResult is the JSON response returned for json_editor operations.
type JsonEditorResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ExecuteJsonEditor handles JSON file editing operations, sandboxed to workspaceDir.
func ExecuteJsonEditor(operation, filePath, jsonPath string, value interface{}, content string, workspaceDir string) string {
	encode := func(r JsonEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if filePath == "" {
		return encode(JsonEditorResult{Status: "error", Message: "'file_path' is required"})
	}

	resolved, err := secureResolve(workspaceDir, filePath)
	if err != nil {
		return encode(JsonEditorResult{Status: "error", Message: err.Error()})
	}

	switch operation {
	case "get":
		return jsonGet(resolved, jsonPath, encode)
	case "set":
		return jsonSet(resolved, jsonPath, value, encode)
	case "delete":
		return jsonDelete(resolved, jsonPath, encode)
	case "keys":
		return jsonKeys(resolved, jsonPath, encode)
	case "validate":
		return jsonValidate(resolved, encode)
	case "format":
		return jsonFormat(resolved, encode)
	default:
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Unknown json_editor operation '%s'. Valid: get, set, delete, keys, validate, format", operation)})
	}
}

func jsonGet(resolved, jsonPath string, encode func(JsonEditorResult) string) string {
	if jsonPath == "" {
		return encode(JsonEditorResult{Status: "error", Message: "'json_path' is required for get"})
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	result := gjson.Get(string(data), jsonPath)
	if !result.Exists() {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Path '%s' not found", jsonPath)})
	}

	return encode(JsonEditorResult{Status: "success", Data: result.Value()})
}

func jsonSet(resolved, jsonPath string, value interface{}, encode func(JsonEditorResult) string) string {
	if jsonPath == "" {
		return encode(JsonEditorResult{Status: "error", Message: "'json_path' is required for set"})
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte("{}")
		} else {
			return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
		}
	}

	// Validate it's valid JSON before editing
	if !gjson.Valid(string(data)) {
		return encode(JsonEditorResult{Status: "error", Message: "File contains invalid JSON"})
	}

	result, err := sjson.Set(string(data), jsonPath, value)
	if err != nil {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Failed to set path '%s': %v", jsonPath, err)})
	}

	// Pretty-print the result
	var pretty json.RawMessage
	if err := json.Unmarshal([]byte(result), &pretty); err == nil {
		if formatted, err := json.MarshalIndent(json.RawMessage(result), "", "  "); err == nil {
			result = string(formatted)
		}
	}

	if err := writeFileAtomic(resolved, []byte(result+"\n")); err != nil {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
	}

	return encode(JsonEditorResult{Status: "success", Message: fmt.Sprintf("Set '%s'", jsonPath)})
}

func jsonDelete(resolved, jsonPath string, encode func(JsonEditorResult) string) string {
	if jsonPath == "" {
		return encode(JsonEditorResult{Status: "error", Message: "'json_path' is required for delete"})
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	if !gjson.Valid(string(data)) {
		return encode(JsonEditorResult{Status: "error", Message: "File contains invalid JSON"})
	}

	result, err := sjson.Delete(string(data), jsonPath)
	if err != nil {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Failed to delete path '%s': %v", jsonPath, err)})
	}

	// Pretty-print
	if formatted, err := json.MarshalIndent(json.RawMessage(result), "", "  "); err == nil {
		result = string(formatted)
	}

	if err := writeFileAtomic(resolved, []byte(result+"\n")); err != nil {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
	}

	return encode(JsonEditorResult{Status: "success", Message: fmt.Sprintf("Deleted '%s'", jsonPath)})
}

func jsonKeys(resolved, jsonPath string, encode func(JsonEditorResult) string) string {
	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	text := string(data)
	var target gjson.Result
	if jsonPath == "" || jsonPath == "." {
		target = gjson.Parse(text)
	} else {
		target = gjson.Get(text, jsonPath)
	}

	if !target.Exists() {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Path '%s' not found", jsonPath)})
	}

	if !target.IsObject() && !target.IsArray() {
		return encode(JsonEditorResult{Status: "error", Message: "Target is not an object or array"})
	}

	var keys []string
	if target.IsObject() {
		target.ForEach(func(key, _ gjson.Result) bool {
			keys = append(keys, key.String())
			return true
		})
	} else {
		// For arrays, return indices
		target.ForEach(func(key, _ gjson.Result) bool {
			keys = append(keys, key.String())
			return true
		})
	}

	return encode(JsonEditorResult{Status: "success", Data: keys})
}

func jsonValidate(resolved string, encode func(JsonEditorResult) string) string {
	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	text := strings.TrimSpace(string(data))
	if gjson.Valid(text) {
		return encode(JsonEditorResult{Status: "success", Message: "Valid JSON"})
	}

	// Try to give a useful error message
	var js json.RawMessage
	if err := json.Unmarshal([]byte(text), &js); err != nil {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Invalid JSON: %v", err)})
	}

	return encode(JsonEditorResult{Status: "error", Message: "Invalid JSON"})
}

func jsonFormat(resolved string, encode func(JsonEditorResult) string) string {
	data, err := os.ReadFile(resolved)
	if err != nil {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Failed to read file: %v", err)})
	}

	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Invalid JSON: %v", err)})
	}

	formatted, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Failed to format: %v", err)})
	}

	if err := writeFileAtomic(resolved, append(formatted, '\n')); err != nil {
		return encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("Failed to write file: %v", err)})
	}

	return encode(JsonEditorResult{Status: "success", Message: "File formatted"})
}
