package server

import (
	"fmt"
	"strings"
)

// gameStarterWrappedSource accepts one provider-style file-write envelope as
// source data. Unlike the general tool parser, it consumes the entire answer,
// accepts no extra fields/operations, and binds the content to the server's
// current job, fixed entry path and revision. No tool dispatch takes place.
func gameStarterWrappedSource(text, jobID, revision string) (string, error) {
	if jobID == "" || revision == "" || len(text) > 4*1024*1024 {
		return "", fmt.Errorf("missing source binding or oversized response")
	}
	body, ok := strings.CutPrefix(strings.TrimSpace(text), "<tool_call>")
	if !ok {
		return "", fmt.Errorf("expected one file-write envelope")
	}
	body, ok = strings.CutSuffix(body, "</tool_call>")
	if !ok || strings.Contains(body, "<tool_call") || strings.Contains(body, "</tool_call>") {
		return "", fmt.Errorf("incomplete or multiple file-write envelopes")
	}
	name, args, ok := strings.Cut(body, "<arg_key>")
	if !ok || strings.TrimSpace(name) != "game_maker_file" {
		return "", fmt.Errorf("expected game_maker_file source data")
	}
	args = "<arg_key>" + args
	fields := make(map[string]string, 5)
	for strings.TrimSpace(args) != "" {
		keyStart, ok := strings.CutPrefix(strings.TrimSpace(args), "<arg_key>")
		if !ok {
			return "", fmt.Errorf("unexpected text outside source fields")
		}
		key, rest, ok := strings.Cut(keyStart, "</arg_key>")
		if !ok {
			return "", fmt.Errorf("incomplete source field name")
		}
		switch key {
		case "operation", "job_id", "path", "expected_sha256", "content":
		default:
			return "", fmt.Errorf("unknown source field")
		}
		if _, exists := fields[key]; exists {
			return "", fmt.Errorf("duplicate source field")
		}
		valueStart, ok := strings.CutPrefix(strings.TrimSpace(rest), "<arg_value>")
		if !ok {
			return "", fmt.Errorf("missing source field value")
		}
		value, remaining, ok := strings.Cut(valueStart, "</arg_value>")
		if !ok {
			return "", fmt.Errorf("incomplete source field value")
		}
		fields[key], args = value, remaining
	}
	if len(fields) != 5 || fields["operation"] != "write" || fields["path"] != "src/main.ts" || fields["job_id"] != jobID || fields["expected_sha256"] != revision {
		return "", fmt.Errorf("source write does not match the current entry revision")
	}
	code := strings.TrimSpace(fields["content"])
	if code == "" {
		return "", fmt.Errorf("empty source content")
	}
	return code, nil
}
