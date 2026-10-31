package agent

import (
	"encoding/json"
	"sort"
	"strings"

	"aurago/internal/kaggle"
	openai "github.com/sashabaranov/go-openai"
)

func expandedKaggleSchema(s *kaggle.Schema) map[string]interface{} {
	return expandKaggleSchema(s, map[*kaggle.Schema]bool{})
}

func expandKaggleSchema(s *kaggle.Schema, active map[*kaggle.Schema]bool) map[string]interface{} {
	if s.Ref != "" {
		return expandKaggleSchema(kaggle.Definitions()[strings.TrimPrefix(s.Ref, "#/$defs/")], active)
	}
	// Provider schemas cannot expand recursive SDK dataset-column structures.
	// Encode only the recursive edge as JSON; the service validates its typed contract.
	if active[s] {
		return prop("string", "JSON encoding of the recursive Kaggle object, validated against the selected operation's SDK contract.")
	}
	active[s] = true
	defer delete(active, s)
	m := map[string]interface{}{"type": s.Type}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}
	if s.Type == "object" {
		props := map[string]interface{}{}
		for key, v := range s.Properties {
			props[key] = expandKaggleSchema(v, active)
		}
		m["properties"] = props
		m["additionalProperties"] = false
	}
	if s.Items != nil {
		m["items"] = expandKaggleSchema(s.Items, active)
	}
	return m
}
func appendKaggleSchemas(existing []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
	if !ff.KaggleEnabled {
		return existing
	}
	for _, domain := range []string{"datasets", "models", "notebooks", "competitions", "benchmarks", "forums"} {
		ops := []string{}
		props := map[string]interface{}{}
		for _, op := range kaggle.Catalog() {
			if op.Domain != domain {
				continue
			}
			ops = append(ops, op.Name)
			for key, s := range op.Schema.Properties {
				next := expandedKaggleSchema(s)
				if previous, ok := props[key]; ok {
					a, _ := json.Marshal(previous)
					b, _ := json.Marshal(next)
					if string(a) != string(b) {
						// The same SDK field can have incompatible operation-specific
						// object/array shapes. Keep its semantic JSON sparse instead of
						// making Strict providers synthesize another operation's fields.
						props[key] = prop("string", "JSON encoding of the selected operation's "+key+" structure. The service validates its exact SDK shape; omit unrelated fields.")
					}
				} else {
					props[key] = next
				}
			}
		}
		sort.Strings(ops)
		existing = append(existing, tool("kaggle_"+domain, "Kaggle "+domain+" through the shared account and grants. Read the manual for the selected operation's exact fields. Mutations create durable jobs; poll kaggle_runtime jobs. Use upload:<job-id> references for uploaded files. Never send credentials. Kaggle notebooks run whole versions; interactive cells use kaggle_runtime.", schema(map[string]interface{}{
			"operation":  operationProperty("Fixed Kaggle operation", ops),
			"parameters": schema(props),
			"id":         prop("string", "Optional 32-hex idempotency ID; reuse only for exactly the same request."),
			"project_id": prop("string", "Managed project ID for transfers."),
			"file":       prop("string", "Selected project filename for uploads, or new destination filename for downloads."),
		}, "operation", "parameters")))
	}
	existing = append(existing, tool("kaggle_runtime", "Manage local Kaggle projects, revisioned nbformat-4 notebooks, durable remote jobs, and explicitly local Jupyter kernels. Save a notebook before executing its revision/cell. No code is automatically rerun. Selected files are copied into an isolated CPU container; credentials never enter the kernel.", schema(map[string]interface{}{
		"operation":  operationProperty("Workspace or local kernel operation", []string{"status", "projects", "create_project", "files", "read_file", "load_notebook", "save_notebook", "jobs", "events", "cancel_job", "sessions", "start", "execute", "complete", "interrupt", "restart", "stop"}),
		"project_id": prop("string", "Managed project ID."), "file": prop("string", "Notebook .ipynb filename."), "title": prop("string", "New project title."), "revision": prop("string", "Observed notebook revision; * only creates a new notebook."), "cell_id": prop("string", "Stable cell ID; cell-N for an imported notebook without IDs."), "session_id": prop("string", "Local kernel session ID."), "job_id": prop("string", "Durable Kaggle job ID."), "files": map[string]interface{}{"type": "array", "items": prop("string", "Explicitly selected project file.")}, "internet": prop("boolean", "Request separately granted local internet access."), "code": prop("string", "Code prefix for completion only."), "cursor": prop("integer", "Completion cursor offset."), "after": prop("integer", "Last observed event cursor; 0 starts the bounded replay. Poll events for local execution output and retain next_cursor."), "notebook_json": prop("string", "Complete nbformat-4 JSON for save_notebook. Preserve metadata, attachments and outputs from load_notebook."),
	}, "operation")))
	return existing
}

func kaggleToolArguments(tc ToolCall) json.RawMessage {
	args := map[string]any{}
	for key, value := range tc.Params {
		args[key] = value
	}
	if tc.Operation != "" {
		args["operation"] = tc.Operation
	}
	if tc.ID != "" {
		args["id"] = tc.ID
	}
	if tc.Parameters != nil {
		args["parameters"] = tc.Parameters
	}
	delete(args, "action")
	delete(args, "learning_reason")
	if text, ok := args["notebook_json"].(string); ok {
		args["content"] = json.RawMessage(text)
	}
	delete(args, "notebook_json")
	b, _ := json.Marshal(args)
	return b
}
