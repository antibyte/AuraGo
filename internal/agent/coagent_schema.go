package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/xeipuuv/gojsonschema"
)

const (
	coAgentOutputSchemaMaxBytes         = 16 * 1024
	coAgentOutputSchemaMaxDepth         = 10
	coAgentOutputSchemaMaxNodes         = 200
	coAgentOutputSchemaMaxPatternLength = 256
)

// CoAgentStructuredResult stores schema-validation metadata for a completed co-agent.
type CoAgentStructuredResult struct {
	SchemaUsed bool
	Valid      bool
	Result     json.RawMessage
	Error      string
}

func sanitizeCoAgentOutputSchema(schema map[string]interface{}) (map[string]interface{}, error) {
	if len(schema) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("output_schema is not JSON-serializable: %w", err)
	}
	if len(raw) > coAgentOutputSchemaMaxBytes {
		return nil, fmt.Errorf("output_schema is too large: %d bytes (max %d)", len(raw), coAgentOutputSchemaMaxBytes)
	}

	var normalized map[string]interface{}
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return nil, fmt.Errorf("output_schema is invalid JSON: %w", err)
	}
	if err := inspectCoAgentOutputSchema(normalized, 1, new(int)); err != nil {
		return nil, err
	}
	if _, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(normalized)); err != nil {
		return nil, fmt.Errorf("output_schema is not a valid JSON Schema: %w", err)
	}
	return normalized, nil
}

func inspectCoAgentOutputSchema(value interface{}, depth int, nodes *int) error {
	if depth > coAgentOutputSchemaMaxDepth {
		return fmt.Errorf("output_schema is too deeply nested: depth %d exceeds max %d", depth, coAgentOutputSchemaMaxDepth)
	}
	*nodes = *nodes + 1
	if *nodes > coAgentOutputSchemaMaxNodes {
		return fmt.Errorf("output_schema is too complex: node count exceeds max %d", coAgentOutputSchemaMaxNodes)
	}

	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			switch key {
			case "$ref", "$dynamicRef", "$recursiveRef":
				return fmt.Errorf("output_schema must not use %s", key)
			case "pattern":
				if pattern, ok := child.(string); ok && len(pattern) > coAgentOutputSchemaMaxPatternLength {
					return fmt.Errorf("output_schema pattern is too long: %d bytes (max %d)", len(pattern), coAgentOutputSchemaMaxPatternLength)
				}
			}
			if err := inspectCoAgentOutputSchema(child, depth+1, nodes); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, child := range typed {
			if err := inspectCoAgentOutputSchema(child, depth+1, nodes); err != nil {
				return err
			}
		}
	}
	return nil
}

func appendCoAgentOutputSchemaPrompt(prompt string, schema map[string]interface{}) string {
	if len(schema) == 0 {
		return prompt
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return prompt
	}
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(prompt))
	sb.WriteString("\n\n## Required Structured Output\n")
	sb.WriteString("Return only one JSON object or JSON array that validates against this JSON Schema. Do not wrap it in Markdown fences and do not add prose before or after the JSON.\n")
	sb.WriteString("Schema:\n")
	sb.Write(raw)
	return sb.String()
}

func evaluateCoAgentStructuredResult(result string, schema map[string]interface{}, maxBytes int) CoAgentStructuredResult {
	if len(schema) == 0 {
		return CoAgentStructuredResult{}
	}
	structured := CoAgentStructuredResult{SchemaUsed: true}
	if maxBytes > 0 && len(result) > maxBytes {
		structured.Error = fmt.Sprintf("co-agent result exceeds max_result_bytes (%d bytes) before schema validation", maxBytes)
		return structured
	}

	jsonBytes, err := extractCoAgentJSONResult(result)
	if err != nil {
		structured.Error = err.Error()
		return structured
	}

	validation, err := gojsonschema.Validate(gojsonschema.NewGoLoader(schema), gojsonschema.NewBytesLoader(jsonBytes))
	if err != nil {
		structured.Error = fmt.Sprintf("schema validation failed: %v", err)
		return structured
	}
	if !validation.Valid() {
		messages := make([]string, 0, len(validation.Errors()))
		for _, desc := range validation.Errors() {
			messages = append(messages, desc.String())
			if len(messages) >= 4 {
				break
			}
		}
		structured.Error = "structured result does not match output_schema: " + strings.Join(messages, "; ")
		return structured
	}

	structured.Valid = true
	structured.Result = append(json.RawMessage(nil), jsonBytes...)
	return structured
}

func extractCoAgentJSONResult(result string) ([]byte, error) {
	trimmed := strings.TrimSpace(result)
	if trimmed == "" {
		return nil, fmt.Errorf("co-agent result is empty; expected JSON object or array")
	}
	if bytes.HasPrefix([]byte(trimmed), []byte("```")) {
		if unfenced := stripCoAgentJSONFence(trimmed); unfenced != "" {
			trimmed = unfenced
		}
	}
	if isCoAgentJSONContainer(trimmed) {
		if compact, ok := compactCoAgentJSON([]byte(trimmed)); ok {
			return compact, nil
		}
	}
	if extracted, ok := firstBalancedCoAgentJSONContainer(trimmed); ok {
		if compact, ok := compactCoAgentJSON([]byte(extracted)); ok {
			return compact, nil
		}
	}
	return nil, fmt.Errorf("co-agent result does not contain a valid JSON object or array")
}

func stripCoAgentJSONFence(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return ""
	}
	first := strings.TrimSpace(lines[0])
	last := strings.TrimSpace(lines[len(lines)-1])
	if !strings.HasPrefix(first, "```") || last != "```" {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
}

func isCoAgentJSONContainer(s string) bool {
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

func compactCoAgentJSON(raw []byte) ([]byte, bool) {
	var decoded interface{}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		return nil, false
	}
	var trailing interface{}
	if err := dec.Decode(&trailing); err != io.EOF {
		return nil, false
	}
	switch decoded.(type) {
	case map[string]interface{}, []interface{}:
	default:
		return nil, false
	}
	compact, err := json.Marshal(decoded)
	if err != nil {
		return nil, false
	}
	return compact, true
}

func firstBalancedCoAgentJSONContainer(s string) (string, bool) {
	for start, r := range s {
		if r != '{' && r != '[' {
			continue
		}
		if end, ok := balancedCoAgentJSONEnd(s[start:], r); ok {
			return s[start : start+end], true
		}
	}
	return "", false
}

func balancedCoAgentJSONEnd(s string, opener rune) (int, bool) {
	stack := []rune{map[rune]rune{'{': '}', '[': ']'}[opener]}
	inString := false
	escaped := false
	for i, r := range s {
		if i == 0 {
			continue
		}
		if !utf8.ValidRune(r) {
			return 0, false
		}
		if inString {
			switch {
			case escaped:
				escaped = false
			case r == '\\':
				escaped = true
			case r == '"':
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
		case '{':
			stack = append(stack, '}')
		case '[':
			stack = append(stack, ']')
		case '}', ']':
			if len(stack) == 0 || stack[len(stack)-1] != r {
				return 0, false
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + len(string(r)), true
			}
		}
	}
	return 0, false
}
