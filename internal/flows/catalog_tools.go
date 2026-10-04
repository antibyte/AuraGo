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

	// maxToolOutputBytes caps the raw tool output callTool accepts. json.Unmarshal
	// cannot be cancelled and needs about 50 times the input in memory, so the
	// engine's MaxOutputBytes (checked after parsing) is no protection.
	maxToolOutputBytes = 16 << 20

	replacementRune = "�"
)

// validUTF8 replaces invalid UTF-8 with U+FFFD. Strings that reach a node output
// or an error message must be valid, or they fail JSON encoding downstream.
func validUTF8(s string) string { return strings.ToValidUTF8(s, replacementRune) }

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
// maxToolMessageRunes. Text is made valid UTF-8.
//
// Limits: numbers decode as float64 (engine-wide), so 64-bit ids must be strings;
// JSON followed by trailing text is not JSON and falls back to {"text": ...}; JSON
// nested deeper than about 10000 levels falls back to text, and the engine fails
// a node whose output is nested beyond its own JSON depth limit. The input size is
// not limited here; callTool rejects output over maxToolOutputBytes first.
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
		return map[string]any{"text": validUTF8(text)}, nil
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
			return truncateRunes(validUTF8(s), maxToolMessageRunes)
		}
	}
	return fallback
}

// invokeError converts an error returned by the tool invoker into a NodeError
// whose message is valid UTF-8 and cut. It never returns nil for a non-nil err,
// and it copies, so an error shared by the invoker is not modified.
func invokeError(err error) *NodeError {
	ne := asNodeError(err)
	return &NodeError{Code: ne.Code, Message: truncateRunes(validUTF8(ne.Message), maxToolMessageRunes)}
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
//
// A cancelled context fails before the tool is invoked. args is passed on as it is
// (nil becomes an empty map): the invoker must treat it as read-only, because it may
// alias run data such as resolved parameters. Raw output over maxToolOutputBytes
// fails with FLOW_OUTPUT_TOO_LARGE before it is parsed.
//
// The failure is classified by the response status first: "denied" and "policy_denied"
// give FLOW_TOOL_DENIED and "needs_setup" gives FLOW_NODE_UNAVAILABLE, whatever the body
// says and even when IsError is unset. Then an error status in the body, or IsError,
// give FLOW_TOOL_ERROR (or FLOW_TOOL_DENIED for a denied body status).
func callTool(ctx context.Context, in ExecInput, tool string, args map[string]any) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, invokeError(err)
	}
	if in.Services == nil || in.Services.Tools == nil {
		return nil, NewNodeError("FLOW_TOOLS_UNAVAILABLE", "tools are not available in this run")
	}
	if args == nil {
		args = map[string]any{}
	}
	resp, err := in.Services.Tools.InvokeTool(ctx, ToolRequest{
		FlowID: in.Run.FlowID, RunID: in.Run.ID, NodeID: nodeIDOf(in), Mode: in.Run.Mode,
		Tool: tool, Args: args, AllowedTools: []string{tool},
	})
	if err != nil {
		return nil, invokeError(err)
	}
	if len(resp.Output) > maxToolOutputBytes {
		return nil, NewNodeError("FLOW_OUTPUT_TOO_LARGE", "the tool returned %d bytes; the limit is %d", len(resp.Output), maxToolOutputBytes)
	}
	out, ne := ParseToolOutput(resp.Output)
	switch strings.ToLower(strings.TrimSpace(resp.Status)) {
	case "denied", "policy_denied":
		return out, &NodeError{Code: "FLOW_TOOL_DENIED", Message: toolMessage(out, "the tool call was denied")}
	case "needs_setup":
		return out, &NodeError{Code: "FLOW_NODE_UNAVAILABLE", Message: toolMessage(out, "the tool is not set up")}
	}
	if ne != nil {
		return out, ne
	}
	if resp.IsError {
		return out, &NodeError{Code: "FLOW_TOOL_ERROR", Message: toolMessage(out, "the tool reported an error")}
	}
	return out, nil
}

// setArg sets args[key] unless v is empty (nil, blank text, empty list or object).
func setArg(args map[string]any, key string, v any) {
	if !isEmptyValue(v) {
		args[key] = v
	}
}
