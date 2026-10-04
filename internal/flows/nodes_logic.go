package flows

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Logic node types.
const (
	TypeIf     = "logic.if"
	TypeSwitch = "logic.switch"
	TypeMerge  = "logic.merge"
	TypeWait   = "logic.wait"
	TypeSet    = "logic.set"
	TypeStop   = "logic.stop"
)

const (
	maxSwitchCases = 20
	maxWait        = time.Hour
)

var clockPattern = regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`)

// RegisterLogicNodes registers the built-in logic nodes.
func RegisterLogicNodes(reg *Registry) error {
	for _, def := range []*NodeDef{ifNodeDef(), switchNodeDef(), mergeNodeDef(), waitNodeDef(), setNodeDef(), stopNodeDef()} {
		if err := reg.Register(def); err != nil {
			return err
		}
	}
	return nil
}

func logicDef(typ, icon string) *NodeDef {
	key := strings.ReplaceAll(typ, ".", "_")
	return &NodeDef{
		Type: typ, Version: 1, Category: "logic", Icon: icon, Color: "logic",
		LabelKey:       "easydrag.node." + key + ".label",
		DescriptionKey: "easydrag.node." + key + ".description",
		SummaryKey:     "easydrag.node." + key + ".summary",
	}
}

func option(value, key string) Option {
	return Option{Value: value, LabelKey: "easydrag.option." + key}
}

func paramIssue(n *Node, code string, sev Severity, param, msg string) Issue {
	return Issue{Code: code, Severity: sev, NodeID: n.ID, Param: param, Message: msg}
}

func evaluateConditionParam(v any, loc *time.Location) (bool, error) {
	group, err := DecodeConditionGroup(v)
	if err != nil {
		return false, NewNodeError("FLOW_CONDITION_INVALID", "%v", err)
	}
	ok, err := group.Evaluate(loc)
	if err != nil {
		return false, NewNodeError("FLOW_CONDITION_FAILED", "%v", err)
	}
	return ok, nil
}

func conditionIssues(n *Node, param string, v any) []Issue {
	if v == nil {
		return nil
	}
	if _, err := DecodeConditionGroup(v); err != nil {
		return []Issue{paramIssue(n, IssueParamInvalid, SeverityError, param, err.Error())}
	}
	return nil
}

func ifNodeDef() *NodeDef {
	def := logicDef(TypeIf, "git-branch")
	def.Outputs = []string{PortTrue, PortFalse}
	def.Params = []ParamSpec{{Name: "condition", Kind: ParamConditionGroup, LabelKey: "easydrag.param.condition", Required: true, Templatable: true}}
	def.OutputFields = []FieldSpec{{Name: "result", Type: "bool", Primary: true}}
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		return conditionIssues(n, "condition", n.Params["condition"])
	}
	def.Execute = func(_ context.Context, in ExecInput) (ExecResult, error) {
		ok, err := evaluateConditionParam(in.Params["condition"], in.Services.Loc())
		if err != nil {
			return ExecResult{}, err
		}
		port := PortFalse
		if ok {
			port = PortTrue
		}
		return ExecResult{Output: map[string]any{"result": ok}, Ports: []string{port}}, nil
	}
	return def
}

func casePort(i int) string { return "case_" + strconv.Itoa(i+1) }

func switchNodeDef() *NodeDef {
	def := logicDef(TypeSwitch, "arrows-split")
	def.Params = []ParamSpec{
		{Name: "cases", Kind: ParamCases, LabelKey: "easydrag.param.cases", Required: true, Templatable: true},
		{Name: "mode", Kind: ParamSegmented, LabelKey: "easydrag.param.switch_mode", Default: "first",
			Options: []Option{option("first", "switch_first"), option("all", "switch_all")}},
	}
	def.OutputsFunc = func(n *Node) []string {
		cases, _ := n.Params["cases"].([]any)
		ports := make([]string, 0, len(cases)+1)
		for i := range cases {
			ports = append(ports, casePort(i))
		}
		return append(ports, PortDefault)
	}
	def.OutputFields = []FieldSpec{{Name: "case", Type: "text", Primary: true}, {Name: "matched", Type: "list"}}
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		cases, _ := n.Params["cases"].([]any)
		var issues []Issue
		if len(cases) > maxSwitchCases {
			issues = append(issues, paramIssue(n, IssueParamInvalid, SeverityError, "cases", fmt.Sprintf("at most %d cases", maxSwitchCases)))
		}
		for i, c := range cases {
			m, _ := c.(map[string]any)
			issues = append(issues, conditionIssues(n, fmt.Sprintf("cases[%d].condition", i), m["condition"])...)
		}
		return issues
	}
	def.Execute = func(_ context.Context, in ExecInput) (ExecResult, error) {
		cases, _ := in.Params["cases"].([]any)
		matchAll := Stringify(in.Params["mode"]) == "all"
		matched := []any{}
		var ports []string
		label := ""
		for i, c := range cases {
			m, _ := c.(map[string]any)
			ok, err := evaluateConditionParam(m["condition"], in.Services.Loc())
			if err != nil {
				return ExecResult{}, err
			}
			if !ok {
				continue
			}
			if label == "" {
				label = Stringify(m["label"])
				if label == "" {
					label = casePort(i)
				}
			}
			matched = append(matched, casePort(i))
			ports = append(ports, casePort(i))
			if !matchAll {
				break
			}
		}
		if len(ports) == 0 {
			ports = []string{PortDefault}
		}
		return ExecResult{Output: map[string]any{"case": label, "matched": matched}, Ports: ports}, nil
	}
	return def
}

func mergeNodeDef() *NodeDef {
	def := logicDef(TypeMerge, "arrow-merge")
	def.Params = []ParamSpec{
		{Name: "mode", Kind: ParamSegmented, LabelKey: "easydrag.param.merge_mode", Default: "wait_all",
			Options: []Option{option("wait_all", "merge_wait_all"), option("append", "merge_append")}},
		{Name: "field", Kind: ParamText, LabelKey: "easydrag.param.merge_field", Default: "items",
			VisibleIf: &Visibility{Param: "mode", Equals: []string{"append"}}},
	}
	def.OutputFields = []FieldSpec{{Name: "items", Type: "list", Primary: true}}
	// Execute builds fresh maps and slices and only reads the received inputs
	// (see the read-only contract on ExecInput).
	def.Execute = func(_ context.Context, in ExecInput) (ExecResult, error) {
		if Stringify(in.Params["mode"]) != "append" {
			out := make(map[string]any, len(in.Inputs))
			for _, input := range in.Inputs {
				out[input.Key] = input.Output
			}
			return ExecResult{Output: out}, nil
		}
		field := Stringify(in.Params["field"])
		if field == "" {
			field = "items"
		}
		items := []any{}
		for _, input := range in.Inputs {
			if list, ok := input.Output[field].([]any); ok {
				items = append(items, list...)
				continue
			}
			items = append(items, input.Output)
		}
		return ExecResult{Output: map[string]any{"items": items, "count": float64(len(items))}, ItemCount: len(items)}, nil
	}
	return def
}

func waitNodeDef() *NodeDef {
	def := logicDef(TypeWait, "hourglass")
	def.DefaultTimeout = maxWait + time.Minute
	def.Params = []ParamSpec{
		{Name: "mode", Kind: ParamSegmented, LabelKey: "easydrag.param.wait_mode", Default: "duration",
			Options: []Option{option("duration", "wait_duration"), option("until", "wait_until")}},
		{Name: "seconds", Kind: ParamNumber, LabelKey: "easydrag.param.wait_seconds", Required: true, Templatable: true, Default: 60.0,
			VisibleIf: &Visibility{Param: "mode", Equals: []string{"duration"}}},
		{Name: "until", Kind: ParamText, LabelKey: "easydrag.param.wait_until", Required: true,
			VisibleIf: &Visibility{Param: "mode", Equals: []string{"until"}}},
	}
	def.OutputFields = []FieldSpec{{Name: "waited_seconds", Type: "number", Primary: true}}
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		if Stringify(n.Params["mode"]) == "until" {
			if v, ok := n.Params["until"].(string); ok && v != "" && !HasTemplate(v) && !clockPattern.MatchString(v) {
				return []Issue{paramIssue(n, IssueParamInvalid, SeverityError, "until", "use the format HH:MM")}
			}
			return nil
		}
		raw, present := n.Params["seconds"]
		if !present {
			return nil
		}
		if s, ok := raw.(string); ok && HasTemplate(s) {
			return nil
		}
		if f, ok := toNumber(raw); !ok || f < 1 || f > maxWait.Seconds() {
			return []Issue{paramIssue(n, IssueParamInvalid, SeverityError, "seconds", "seconds must be between 1 and 3600")}
		}
		return nil
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		d, err := waitDuration(in)
		if err != nil {
			return ExecResult{}, err
		}
		if err := in.Services.Sleep(ctx, d); err != nil {
			return ExecResult{}, err
		}
		return ExecResult{Output: map[string]any{"waited_seconds": d.Seconds()}}, nil
	}
	return def
}

func waitDuration(in ExecInput) (time.Duration, error) {
	if Stringify(in.Params["mode"]) == "until" {
		value := strings.TrimSpace(Stringify(in.Params["until"]))
		m := clockPattern.FindStringSubmatch(value)
		if m == nil {
			return 0, NewNodeError("FLOW_PARAM_INVALID", "until must use the format HH:MM, got %s", quoteForError(value))
		}
		hour, _ := strconv.Atoi(m[1])
		minute, _ := strconv.Atoi(m[2])
		now := in.Services.Now().In(in.Services.Loc())
		target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if !target.After(now) {
			target = target.AddDate(0, 0, 1)
		}
		d := target.Sub(now)
		if d > maxWait {
			return 0, NewNodeError("FLOW_WAIT_TOO_LONG", "waiting until %s would take longer than one hour", value)
		}
		return d, nil
	}
	f, ok := toNumber(in.Params["seconds"])
	if !ok || f < 1 || f > maxWait.Seconds() {
		return 0, NewNodeError("FLOW_PARAM_INVALID", "seconds must be between 1 and 3600")
	}
	return time.Duration(f * float64(time.Second)), nil
}

func setNodeDef() *NodeDef {
	def := logicDef(TypeSet, "pencil")
	def.Params = []ParamSpec{
		{Name: "fields", Kind: ParamFields, LabelKey: "easydrag.param.fields", Required: true, Templatable: true,
			Fields: []ParamSpec{
				{Name: "name", Kind: ParamText, LabelKey: "easydrag.param.field_name", Required: true},
				{Name: "type", Kind: ParamSelect, LabelKey: "easydrag.param.field_type", Default: "auto",
					Options: []Option{option("auto", "type_auto"), option("text", "type_text"), option("number", "type_number"),
						option("bool", "type_bool"), option("list", "type_list"), option("object", "type_object")}},
				{Name: "value", Kind: ParamText, LabelKey: "easydrag.param.field_value", Templatable: true},
			}},
		{Name: "keep_input", Kind: ParamBool, LabelKey: "easydrag.param.keep_input", Default: false},
	}
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		fields, _ := n.Params["fields"].([]any)
		seen := map[string]bool{}
		var issues []Issue
		for i, f := range fields {
			m, _ := f.(map[string]any)
			name := strings.TrimSpace(Stringify(m["name"]))
			param := fmt.Sprintf("fields[%d].name", i)
			if name == "" {
				issues = append(issues, paramIssue(n, IssueParamRequired, SeverityError, param, "field name is required"))
				continue
			}
			if seen[name] {
				issues = append(issues, paramIssue(n, IssueParamInvalid, SeverityWarning, param, "duplicate field name "+quoteForError(name)))
			}
			seen[name] = true
		}
		return issues
	}
	// Execute starts from a new map: the first input's output is copied
	// key by key and never written to (see the read-only contract on ExecInput).
	def.Execute = func(_ context.Context, in ExecInput) (ExecResult, error) {
		out := map[string]any{}
		if truthy(in.Params["keep_input"]) && len(in.Inputs) > 0 {
			for k, v := range in.Inputs[0].Output {
				out[k] = v
			}
		}
		fields, _ := in.Params["fields"].([]any)
		for _, f := range fields {
			m, _ := f.(map[string]any)
			name := strings.TrimSpace(Stringify(m["name"]))
			if name == "" {
				continue
			}
			v, err := convertValue(m["value"], Stringify(m["type"]))
			if err != nil {
				return ExecResult{}, NewNodeError("FLOW_VALUE_TYPE", "field %s: %v", quoteForError(name), err)
			}
			out[name] = v
		}
		return ExecResult{Output: out}, nil
	}
	return def
}

func convertValue(v any, typ string) (any, error) {
	switch typ {
	case "", "auto":
		return v, nil
	case "text":
		return Stringify(v), nil
	case "number":
		f, ok := toNumber(v)
		if !ok {
			return nil, fmt.Errorf("%s is not a number", quoteForError(Stringify(v)))
		}
		return f, nil
	case "bool":
		return truthy(v), nil
	case "list":
		if list, ok := v.([]any); ok {
			return list, nil
		}
		if s, ok := v.(string); ok {
			var list []any
			if err := json.Unmarshal([]byte(s), &list); err == nil && list != nil {
				return list, nil
			}
		}
		return nil, fmt.Errorf("the value is not a list")
	case "object":
		if m, ok := v.(map[string]any); ok {
			return m, nil
		}
		if s, ok := v.(string); ok {
			var m map[string]any
			if err := json.Unmarshal([]byte(s), &m); err == nil && m != nil {
				return m, nil
			}
		}
		return nil, fmt.Errorf("the value is not an object")
	}
	return nil, fmt.Errorf("unknown type %s", quoteForError(typ))
}

func stopNodeDef() *NodeDef {
	def := logicDef(TypeStop, "player-stop")
	def.Outputs = []string{}
	def.Params = []ParamSpec{
		{Name: "status", Kind: ParamSegmented, LabelKey: "easydrag.param.stop_status", Default: "success",
			Options: []Option{option("success", "stop_success"), option("error", "stop_error")}},
		{Name: "message", Kind: ParamText, LabelKey: "easydrag.param.stop_message", Templatable: true},
	}
	def.Execute = func(_ context.Context, in ExecInput) (ExecResult, error) {
		status := RunSuccess
		if Stringify(in.Params["status"]) == "error" {
			status = RunError
		}
		msg := Stringify(in.Params["message"])
		return ExecResult{
			Output: map[string]any{"status": string(status), "message": msg},
			Stop:   &StopSignal{Status: status, Message: msg},
		}, nil
	}
	return def
}
