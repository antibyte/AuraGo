package kaggle

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

//go:embed contracts.json
var contractJSON []byte

type Schema struct {
	Ref        string             `json:"$ref,omitempty"`
	Type       string             `json:"type,omitempty"`
	Properties map[string]*Schema `json:"properties,omitempty"`
	Items      *Schema            `json:"items,omitempty"`
	Enum       []string           `json:"enum,omitempty"`
	Additional any                `json:"additionalProperties,omitempty"`
	MaxItems   int                `json:"maxItems,omitempty"`
}

type Operation struct {
	Domain   string  `json:"domain"`
	Name     string  `json:"operation"`
	Service  string  `json:"service"`
	Method   string  `json:"method"`
	Request  string  `json:"request"`
	Response string  `json:"response"`
	Grant    string  `json:"grant"`
	Schema   *Schema `json:"schema,omitempty"`
}

var contracts struct {
	Operations  []Operation        `json:"operations"`
	Definitions map[string]*Schema `json:"$defs"`
}
var operations = map[string]Operation{}

func init() {
	if err := json.Unmarshal(contractJSON, &contracts); err != nil {
		panic(err)
	}
	// Every mutation is explicit. The pinned catalog has no server admin operations.
	grants := map[string]string{}
	for grant, names := range map[string]string{
		"create":   "create_dataset upload_dataset_file create_model create_model_instance upload_model_file create_benchmark_task",
		"update":   "update_dataset_metadata create_dataset_version create_model_instance_version update_model update_model_instance save_kernel update_benchmark_task update_benchmark_task_version",
		"delete":   "delete_dataset delete_model delete_model_instance delete_model_instance_version delete_kernel",
		"submit":   "create_submission create_code_submission start_submission_upload",
		"execute":  "batch_schedule_benchmark_task_runs",
		"cancel":   "cancel_kernel_session api_cancel_benchmark_task_runs",
		"publish":  "publish_benchmark_task",
		"download": "download_dataset download_model_instance_version download_kernel_output download_kernel_output_zip download_leaderboard download_submission download_data_files download_data_file download_benchmark_task_run_output get_kernel_session_logs_stream get_benchmark_task_run_logs",
	} {
		for _, name := range strings.Fields(names) {
			grants[name] = grant
		}
	}
	for _, op := range contracts.Operations {
		op.Grant = grants[op.Name]
		if op.Grant == "" {
			op.Grant = "read"
		}
		op.Schema = contracts.Definitions[op.Request]
		operations[op.Domain+"/"+op.Name] = op
	}
}

func Catalog() []Operation {
	out := make([]Operation, 0, len(operations))
	for _, op := range operations {
		if op.Domain != "internal" {
			out = append(out, op)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Domain+out[i].Name < out[j].Domain+out[j].Name })
	return out
}

func Definitions() map[string]*Schema { return contracts.Definitions }

func Lookup(domain, name string) (Operation, error) {
	op, ok := operations[domain+"/"+name]
	if !ok || domain == "internal" {
		return Operation{}, fmt.Errorf("unknown Kaggle operation")
	}
	return op, nil
}

// Decode validates the exact SDK wire shape and decodes into its generated Go type.
func Decode(op Operation, raw json.RawMessage) (map[string]any, any, error) {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	if len(raw) > 20<<20 {
		return nil, nil, fmt.Errorf("request exceeds 20 MiB")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v map[string]any
	if err := dec.Decode(&v); err != nil {
		return nil, nil, fmt.Errorf("invalid request JSON")
	}
	if v == nil {
		return nil, nil, fmt.Errorf("request must be an object")
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, nil, fmt.Errorf("trailing request data")
	}
	expanded, err := decodeStructuredStrings(op.Schema, v, 0)
	if err != nil {
		return nil, nil, err
	}
	v = expanded.(map[string]any)
	removeOptionalNulls(v)
	if err := validateSchema(op.Schema, v, "request", 0); err != nil {
		return nil, nil, err
	}
	typed := newWireRequest(op.Request)
	if typed == nil {
		return nil, nil, fmt.Errorf("missing wire request")
	}
	canonical, _ := json.Marshal(v)
	if err := json.Unmarshal(canonical, typed); err != nil {
		return nil, nil, fmt.Errorf("invalid typed request")
	}
	return v, typed, nil
}

// Recursive SDK fields use JSON strings at the agent schema's recursion edge.
// They are decoded before validation, never forwarded as unchecked API payloads.
func decodeStructuredStrings(s *Schema, v any, depth int) (any, error) {
	if s == nil || depth > 24 {
		return nil, fmt.Errorf("invalid or excessively nested request")
	}
	if s.Ref != "" {
		return decodeStructuredStrings(contracts.Definitions[strings.TrimPrefix(s.Ref, "#/$defs/")], v, depth+1)
	}
	if raw, ok := v.(string); ok && (s.Type == "object" || s.Type == "array") {
		d := json.NewDecoder(strings.NewReader(raw))
		d.UseNumber()
		if d.Decode(&v) != nil || d.Decode(new(any)) != io.EOF {
			return nil, fmt.Errorf("invalid structured JSON")
		}
	}
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if field := s.Properties[k]; field != nil {
				value, err := decodeStructuredStrings(field, child, depth+1)
				if err != nil {
					return nil, err
				}
				x[k] = value
			}
		}
	case []any:
		for i, child := range x {
			value, err := decodeStructuredStrings(s.Items, child, depth+1)
			if err != nil {
				return nil, err
			}
			x[i] = value
		}
	}
	return v, nil
}

func removeOptionalNulls(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, v := range x {
			if v == nil {
				delete(x, k)
			} else {
				removeOptionalNulls(v)
			}
		}
	case []any:
		for _, v := range x {
			removeOptionalNulls(v)
		}
	}
}

func validateSchema(s *Schema, v any, path string, depth int) error {
	if depth > 24 || s == nil {
		return fmt.Errorf("invalid contract at %s", path)
	}
	if s.Ref != "" {
		return validateSchema(contracts.Definitions[strings.TrimPrefix(s.Ref, "#/$defs/")], v, path, depth+1)
	}
	if v == nil {
		return nil
	} // Optional wire fields are omitted by typed marshaling.
	ok := false
	switch s.Type {
	case "object":
		m, valid := v.(map[string]any)
		if !valid {
			break
		}
		ok = true
		for k, val := range m {
			field, found := s.Properties[k]
			if !found {
				return fmt.Errorf("unknown field %s.%s", path, k)
			}
			if err := validateSchema(field, val, path+"."+k, depth+1); err != nil {
				return err
			}
		}
	case "array":
		items, valid := v.([]any)
		if !valid {
			break
		}
		ok = true
		if len(items) > 1000 {
			return fmt.Errorf("too many items at %s", path)
		}
		for _, item := range items {
			if err := validateSchema(s.Items, item, path+"[]", depth+1); err != nil {
				return err
			}
		}
	case "string":
		str, valid := v.(string)
		ok = valid
		if valid && len(s.Enum) > 0 {
			ok = false
			for _, e := range s.Enum {
				if str == e {
					ok = true
					break
				}
			}
		}
	case "boolean":
		_, ok = v.(bool)
	case "number":
		_, ok = v.(json.Number)
	case "integer":
		n, valid := v.(json.Number)
		if valid {
			_, err := n.Int64()
			ok = err == nil
		}
	}
	if !ok {
		return fmt.Errorf("invalid %s at %s", s.Type, path)
	}
	return nil
}
