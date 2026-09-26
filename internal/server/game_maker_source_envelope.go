package server

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// gameStarterWrappedSource accepts one provider-style file-write envelope as
// source data. It consumes the entire answer and accepts no extra operations.
// Omitted write metadata inherits the server's current job and source revision;
// explicit metadata must match. No tool dispatch takes place.
func gameStarterWrappedSource(text, jobID, revision string) (string, error) {
	if jobID == "" || revision == "" || len(text) > 4*1024*1024 {
		return "", fmt.Errorf("missing source binding or oversized response")
	}
	body := strings.TrimSpace(text)
	inner, wrapped := strings.CutPrefix(body, "<tool_call>")
	if wrapped {
		var closed bool
		body, closed = strings.CutSuffix(inner, "</tool_call>")
		if !closed {
			return "", fmt.Errorf("incomplete file-write envelope")
		}
	}
	if strings.Contains(body, "<tool_call") || strings.Contains(body, "</tool_call>") {
		return "", fmt.Errorf("incomplete or multiple file-write envelopes")
	}
	body = strings.TrimSpace(body)
	var fields map[string]string
	var err error
	if strings.HasPrefix(body, "{") {
		fields, err = gameStarterJSONSourceFields(body)
	} else if wrapped {
		fields, err = gameStarterXMLSourceFields(body)
	} else {
		return "", fmt.Errorf("expected one file-write envelope")
	}
	if err != nil {
		return "", err
	}
	for key := range fields {
		switch key {
		case "operation", "job_id", "path", "expected_sha256", "content":
		default:
			return "", fmt.Errorf("unknown source field")
		}
	}
	for key, want := range map[string]string{"operation": "write", "job_id": jobID, "expected_sha256": revision} {
		if value, supplied := fields[key]; supplied && value != want {
			return "", fmt.Errorf("source write does not match the current entry revision")
		}
	}
	if fields["path"] != "src/main.ts" {
		return "", fmt.Errorf("source write must target src/main.ts")
	}
	code := strings.TrimSpace(fields["content"])
	if code == "" {
		return "", fmt.Errorf("empty source content")
	}
	return code, nil
}

// Provider markup contains raw TypeScript, not XML-escaped text. Parse only
// known delimiters; an XML decoder would alter entities or reject comparisons.
func gameStarterXMLSourceFields(body string) (map[string]string, error) {
	args, function := strings.CutPrefix(body, "<function=game_maker_file>")
	if function {
		var closed bool
		args, closed = strings.CutSuffix(strings.TrimSpace(args), "</function>")
		if !closed {
			return nil, fmt.Errorf("incomplete source function")
		}
	} else {
		name, rest, ok := strings.Cut(body, "<arg_key>")
		if !ok || strings.TrimSpace(name) != "game_maker_file" {
			return nil, fmt.Errorf("expected game_maker_file source data")
		}
		args = "<arg_key>" + rest
	}
	fields := make(map[string]string, 5)
	for strings.TrimSpace(args) != "" {
		openKey, closeKey, openValue, closeValue := "<arg_key>", "</arg_key>", "<arg_value>", "</arg_value>"
		if function {
			openKey, closeKey, openValue, closeValue = "<parameter=", ">", "", "</parameter>"
		}
		keyStart, ok := strings.CutPrefix(strings.TrimSpace(args), openKey)
		if !ok {
			return nil, fmt.Errorf("unexpected text outside source fields")
		}
		key, rest, ok := strings.Cut(keyStart, closeKey)
		if !ok {
			return nil, fmt.Errorf("incomplete source field name")
		}
		if _, exists := fields[key]; exists {
			return nil, fmt.Errorf("duplicate source field")
		}
		if !function {
			rest, ok = strings.CutPrefix(strings.TrimSpace(rest), openValue)
			if !ok {
				return nil, fmt.Errorf("missing source field value")
			}
		}
		value, remaining, ok := strings.Cut(rest, closeValue)
		if !ok {
			return nil, fmt.Errorf("incomplete source field value")
		}
		fields[key], args = value, remaining
	}
	return fields, nil
}

func gameStarterJSONSourceFields(body string) (map[string]string, error) {
	fields, err := gameStarterJSONObject(body)
	if err != nil {
		return nil, err
	}
	name, flat := fields["action"]
	if flat {
		delete(fields, "action")
	} else {
		name = fields["name"]
		if len(fields) != 2 || fields["arguments"] == nil {
			return nil, fmt.Errorf("expected one source function with arguments")
		}
		args := string(fields["arguments"])
		if strings.HasPrefix(args, `"`) {
			if err := json.Unmarshal(fields["arguments"], &args); err != nil {
				return nil, fmt.Errorf("invalid source arguments")
			}
		}
		fields, err = gameStarterJSONObject(args)
		if err != nil {
			return nil, err
		}
	}
	var tool string
	if json.Unmarshal(name, &tool) != nil || tool != "game_maker_file" {
		return nil, fmt.Errorf("expected game_maker_file source data")
	}
	values := make(map[string]string, len(fields))
	for key, raw := range fields {
		var value string
		if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &value) != nil {
			return nil, fmt.Errorf("source fields must be strings")
		}
		values[key] = value
	}
	return values, nil
}

// Reject duplicate keys and consume exactly one complete object.
func gameStarterJSONObject(text string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, fmt.Errorf("expected a source object")
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || fields[key] != nil {
			return nil, fmt.Errorf("invalid or duplicate source field")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("incomplete source field")
		}
		fields[key] = value
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
		return nil, fmt.Errorf("incomplete source object")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected text after source object")
	}
	return fields, nil
}
