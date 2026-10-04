package flows

import (
	"context"
	"encoding/json"
	"html"
	"strings"
)

// CatalogEnv answers configuration questions for the node catalog.
// Plan 1c implements it from the agent tool catalog and the configuration.
type CatalogEnv interface {
	// ToolAvailability reports whether a tool can run now, and why not.
	ToolAvailability(tool string) Availability
}

// StaticEnv lists tools with their availability; unlisted tools need setup.
// An empty State means available.
type StaticEnv map[string]Availability

// ToolAvailability implements CatalogEnv.
func (e StaticEnv) ToolAvailability(tool string) Availability {
	a, ok := e[tool]
	if !ok {
		return Availability{State: NeedsSetupState}
	}
	if a.State == "" {
		a.State = AvailableState
	}
	return a
}

func availabilityOf(env CatalogEnv, tool string) func() Availability {
	return func() Availability {
		if env == nil {
			return Availability{State: NeedsSetupState}
		}
		return env.ToolAvailability(tool)
	}
}

// anyAvailable is available when one of the tools is; otherwise it reports the first tool's state.
func anyAvailable(env CatalogEnv, tools ...string) func() Availability {
	return func() Availability {
		first := Availability{State: NeedsSetupState}
		for i, tool := range tools {
			a := availabilityOf(env, tool)()
			if a.State == AvailableState {
				return a
			}
			if i == 0 {
				first = a
			}
		}
		return first
	}
}

const (
	externalOpen  = "<external_data>"
	externalClose = "</external_data>"

	// maxToolMessageRunes bounds the text of a tool's own error message that a
	// NodeError echoes. Tool output can be huge or hostile, and the message ends
	// up in run records and the UI. It matches the issue message cap.
	maxToolMessageRunes = 300
)

// unwrapExternal removes AuraGo's <external_data> isolation wrapper
// (security.IsolateExternalData) and undoes its HTML escaping.
func unwrapExternal(s string) (string, bool) {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, externalOpen) || !strings.HasSuffix(t, externalClose) {
		return s, false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(t, externalOpen), externalClose)
	return html.UnescapeString(strings.Trim(inner, "\n")), true
}

func unwrapValues(v any) any {
	switch x := v.(type) {
	case string:
		if s, ok := unwrapExternal(x); ok {
			return s
		}
		return x
	case map[string]any:
		for k, item := range x {
			x[k] = unwrapValues(item)
		}
		return x
	case []any:
		for i, item := range x {
			x[i] = unwrapValues(item)
		}
		return x
	}
	return v
}

func stripToolPrefixes(text string) string {
	text = strings.TrimSpace(text)
	for _, prefix := range []string{"[Tool Output]", "Tool Output:"} {
		text = strings.TrimSpace(strings.TrimPrefix(text, prefix))
	}
	return text
}

// ParseToolOutput converts a raw tool result into a node output. "[Tool Output]" and
// "Tool Output:" prefixes and <external_data> wrappers (around the whole output or
// around single string values) are removed; a JSON object becomes the output, an array
// {"items": [...]}, a string {"text": ...}, other JSON scalars {"value": ...} and
// non-JSON text {"text": ...}. A status of error/failed or policy_denied yields a NodeError.
//
// The error is a *NodeError, nil when the tool succeeded. Compare it against nil
// as a *NodeError; never pass a possibly nil result on as an error value, that is a
// non-nil interface (callTool only does so after checking). Its message is cut to
// maxToolMessageRunes.
func ParseToolOutput(raw string) (map[string]any, *NodeError) {
	text := stripToolPrefixes(raw)
	if inner, ok := unwrapExternal(text); ok {
		text = stripToolPrefixes(inner)
	}
	if text == "" {
		return map[string]any{}, nil
	}
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return map[string]any{"text": text}, nil
	}
	v = unwrapValues(v)
	switch x := v.(type) {
	case map[string]any:
		return x, toolStatusError(x)
	case []any:
		return map[string]any{"items": x}, nil
	case string:
		return map[string]any{"text": x}, nil
	case nil:
		return map[string]any{}, nil
	default:
		return map[string]any{"value": x}, nil
	}
}

func toolStatusError(m map[string]any) *NodeError {
	switch strings.ToLower(Stringify(m["status"])) {
	case "error", "failed", "failure":
		return &NodeError{Code: "FLOW_TOOL_ERROR", Message: toolMessage(m, "the tool reported an error")}
	case "policy_denied", "denied":
		return &NodeError{Code: "FLOW_TOOL_DENIED", Message: toolMessage(m, "the tool call was denied")}
	}
	return nil
}

// toolMessage picks the tool's own description of a failure, cut to
// maxToolMessageRunes so a huge or hostile tool output cannot flood the run record.
func toolMessage(m map[string]any, fallback string) string {
	for _, key := range []string{"message", "error", "detail", "text"} {
		if s := strings.TrimSpace(Stringify(m[key])); s != "" {
			return truncateRunes(s, maxToolMessageRunes)
		}
	}
	return fallback
}

// invokeError converts an error returned by the tool invoker into a NodeError
// whose message is cut. It never returns nil for a non-nil err, and it copies,
// so an error shared by the invoker is not modified.
func invokeError(err error) *NodeError {
	ne := asNodeError(err)
	return &NodeError{Code: ne.Code, Message: truncateRunes(ne.Message, maxToolMessageRunes)}
}

func nodeIDOf(in ExecInput) string {
	if in.Node == nil {
		return ""
	}
	return in.Node.ID
}

// callTool runs one AuraGo tool for a node (only that tool is allowed) and parses its output.
// It returns a literal nil error on success and a *NodeError otherwise; the output may
// accompany an error.
func callTool(ctx context.Context, in ExecInput, tool string, args map[string]any) (map[string]any, error) {
	if in.Services == nil || in.Services.Tools == nil {
		return nil, NewNodeError("FLOW_TOOLS_UNAVAILABLE", "tools are not available in this run")
	}
	resp, err := in.Services.Tools.InvokeTool(ctx, ToolRequest{
		FlowID: in.Run.FlowID, RunID: in.Run.ID, NodeID: nodeIDOf(in), Mode: in.Run.Mode,
		Tool: tool, Args: args, AllowedTools: []string{tool},
	})
	if err != nil {
		return nil, invokeError(err)
	}
	out, ne := ParseToolOutput(resp.Output)
	if ne != nil {
		return out, ne
	}
	if resp.IsError {
		code := "FLOW_TOOL_ERROR"
		switch resp.Status {
		case "denied":
			code = "FLOW_TOOL_DENIED"
		case "needs_setup":
			code = "FLOW_NODE_UNAVAILABLE"
		}
		return out, &NodeError{Code: code, Message: toolMessage(out, "the tool reported an error")}
	}
	return out, nil
}

// setArg sets args[key] unless v is empty (nil, blank text, empty list or object).
func setArg(args map[string]any, key string, v any) {
	if !isEmptyValue(v) {
		args[key] = v
	}
}
