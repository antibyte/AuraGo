package gog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Field struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Flag        string   `json:"flag,omitempty"`
	Positional  bool     `json:"positional,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}
type Operation struct {
	Service   string  `json:"service"`
	Operation string  `json:"operation"`
	Grant     string  `json:"grant"`
	Command   string  `json:"command"`
	Fields    []Field `json:"fields"`
	Input     bool    `json:"input,omitempty"`
	Output    bool    `json:"output,omitempty"`
}

//go:embed catalog.json
var catalogJSON []byte
var catalog = func() []Operation {
	var c []Operation
	if e := json.Unmarshal(catalogJSON, &c); e != nil {
		panic(e)
	}
	return c
}()

func Catalog() []Operation { return slices.Clone(catalog) }
func Lookup(service, operation string) (Operation, bool) {
	for _, o := range catalog {
		if o.Service == service && o.Operation == operation {
			return o, true
		}
	}
	return Operation{}, false
}

var unsafeMarkdown = regexp.MustCompile("(?im)!\\[|<\\s*(img|svg|iframe|script)|^[ \\t]*(`{3,}|~{3,})[ \\t]*mermaid|fa[srb]?:|fontawesome")

func Arguments(req Request, op Operation, dir string, files *Files) ([]string, error) {
	args := strings.Split(op.Command, ".")
	if op.Service == "drive" && (op.Operation == "get" || op.Operation == "list") {
		mask := "id,name,mimeType,size,modifiedTime,parents,version"
		if op.Operation == "list" {
			mask = "nextPageToken,files(" + mask + ")"
		}
		args = append(args, "--fields="+mask)
	}
	var positional []string
	known := map[string]bool{}
	for _, f := range op.Fields {
		known[f.Name] = true
		value, present := req.Args[f.Name]
		if value == nil {
			present = false
		}
		if !present {
			if f.Required {
				return nil, fmt.Errorf("%s is required", f.Name)
			}
			continue
		}
		var values []string
		switch f.Type {
		case "string":
			v, ok := value.(string)
			if !ok || len(v) > 128<<10 || strings.ContainsRune(v, 0) || strings.HasPrefix(v, "@") {
				return nil, fmt.Errorf("invalid %s", f.Name)
			}
			textReplacement := op.Operation == "replace_text" && (f.Name == "replace" || f.Name == "replacement")
			if f.Required && v == "" && !textReplacement {
				return nil, fmt.Errorf("%s is required", f.Name)
			}
			if len(f.Enum) > 0 && !slices.Contains(f.Enum, v) {
				return nil, fmt.Errorf("invalid %s", f.Name)
			}
			values = []string{v}
		case "boolean":
			v, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("invalid %s", f.Name)
			}
			values = []string{strconv.FormatBool(v)}
		case "integer":
			v, ok := value.(float64)
			if !ok {
				if n, yes := value.(int); yes {
					v = float64(n)
					ok = true
				}
				if n, yes := value.(json.Number); yes {
					v, _ = strconv.ParseFloat(string(n), 64)
					ok = true
				}
			}
			if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < 1 || v != math.Trunc(v) || v > float64(1<<53-1) || (f.Name == "max" && v > 100) {
				return nil, fmt.Errorf("invalid %s", f.Name)
			}
			values = []string{strconv.FormatFloat(v, 'f', 0, 64)}
		case "array":
			v, ok := value.([]any)
			if !ok || len(v) > 50 {
				return nil, fmt.Errorf("invalid %s", f.Name)
			}
			for _, entry := range v {
				str, ok := entry.(string)
				if !ok || len(str) > 2048 || strings.ContainsRune(str, 0) || strings.HasPrefix(str, "@") {
					return nil, fmt.Errorf("invalid %s", f.Name)
				}
				values = append(values, str)
			}
		default:
			return nil, fmt.Errorf("unsupported parameter type")
		}
		for _, value := range values {
			if f.Positional {
				positional = append(positional, value)
			} else {
				args = append(args, "--"+f.Flag+"="+value)
			}
		}
	}
	for k, v := range req.Args {
		if v != nil && !known[k] {
			return nil, fmt.Errorf("unsupported argument %s", k)
		}
	}
	if op.Service == "gmail" && op.Operation == "label" {
		add, _ := req.Args["add"].([]any)
		for _, value := range add {
			// gog splits each repeated --add flag on commas before resolving labels.
			for _, label := range strings.Split(value.(string), ",") {
				if strings.EqualFold(strings.TrimSpace(label), "TRASH") {
					return nil, fmt.Errorf("use the trash operation for this label")
				}
			}
		}
	}
	if op.Service == "calendar" {
		for _, key := range []string{"from", "to", "original_start"} {
			if v, ok := req.Args[key].(string); ok && v != "" {
				_, e := time.Parse(time.RFC3339, v)
				if e != nil {
					_, e = time.Parse("2006-01-02", v)
				}
				if e != nil {
					return nil, fmt.Errorf("%s requires an ISO date or RFC3339 time", key)
				}
			}
		}
		for _, key := range []string{"timezone", "start_timezone", "end_timezone"} {
			if v, ok := req.Args[key].(string); ok && v != "" {
				if _, e := time.LoadLocation(v); e != nil {
					return nil, fmt.Errorf("invalid timezone")
				}
			}
		}
		if scope, _ := req.Args["scope"].(string); scope == "single" || scope == "future" {
			if req.Args["original_start"] == nil || req.Args["original_start"] == "" {
				return nil, fmt.Errorf("original_start is required for this recurrence scope")
			}
		}
	}
	if op.Service == "slides" && op.Operation == "create" {
		v, _ := req.Args["content"].(string)
		if unsafeMarkdown.MatchString(v) {
			return nil, fmt.Errorf("Markdown images, diagrams and external asset rendering are not supported")
		}
	}
	if v, ok := req.Args["values_json"].(string); ok {
		var rows [][]any
		if json.Unmarshal([]byte(v), &rows) != nil || len(rows) > 1000 {
			return nil, fmt.Errorf("values_json must be a bounded array of rows")
		}
	}
	if op.Grant == "delete" {
		args = append(args, "--force")
	}
	if op.Input {
		p, e := files.stage(dir)
		if e != nil {
			return nil, e
		}
		positional = append([]string{p}, positional...)
	}
	if op.Output {
		if files == nil || files.Output == nil {
			return nil, fmt.Errorf("output_file_required")
		}
		args = append(args, "--out="+filepath.Join(dir, "output"))
	}
	if len(positional) > 0 {
		args = append(args, "--")
		args = append(args, positional...)
	}
	return args, nil
}
