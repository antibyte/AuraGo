package flows

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
)

// Parameter and tool-output helpers shared by the catalog action nodes: bounded
// file paths and names, enumerated choices, text rendering and the success check
// for a tool answer.

const (
	// maxFilePathBytes bounds a file path parameter (PATH_MAX on Linux). A flow
	// document can carry megabytes of text, and the path is passed on to a tool
	// and echoed in a result; a path this long is never meant.
	maxFilePathBytes = 4096
	// maxFileNameBytes bounds the output name of a created document (NAME_MAX).
	maxFileNameBytes = 255
)

// checkFileText rejects text that cannot be a file path or name: over limit bytes,
// not valid UTF-8, or holding a NUL byte (which C level file APIs read as the end
// of the name). The message names what, never echoes the text.
func checkFileText(s string, limit int, what string) error {
	switch {
	case len(s) > limit:
		return NewNodeError("FLOW_PARAM_INVALID", "the %s is %d bytes; the limit is %d", what, len(s), limit)
	case !utf8.ValidString(s) || strings.IndexByte(s, 0) >= 0:
		return NewNodeError("FLOW_PARAM_INVALID", "the %s contains characters that are not allowed", what)
	}
	return nil
}

// filePathParam reads a file parameter (a path or a file object). An empty value
// fails with missing as the message; an over-long or malformed one says so. It
// returns a literal nil error on success.
func filePathParam(v any, missing string) (string, error) {
	p := FilePath(v)
	if p == "" {
		return "", NewNodeError("FLOW_PARAM_INVALID", "%s", missing)
	}
	if err := checkFileText(p, maxFilePathBytes, "file path"); err != nil {
		return "", err
	}
	return p, nil
}

// fileNameParam reads the optional output name of a document: absent or null is
// "", text is trimmed, numbers and booleans use their natural form, anything else
// (a list, an object) is rejected instead of being rendered into a name.
func fileNameParam(v any) (string, error) {
	s := ""
	switch x := v.(type) {
	case nil:
	case string:
		s = strings.TrimSpace(x)
	case bool, float64, float32, int, int64, json.Number:
		s = Stringify(x)
	default:
		return "", NewNodeError("FLOW_PARAM_INVALID", "the file name must be text")
	}
	if err := checkFileText(s, maxFileNameBytes, "file name"); err != nil {
		return "", err
	}
	return s, nil
}

// documentText renders a parameter that becomes document or file text. Scalars use
// their natural form; a list or object becomes compact JSON, and one that cannot
// be encoded (NaN, a cycle) fails the node, because "<unserializable>" written
// into a file would be silent data loss.
func documentText(name string, v any) (string, error) {
	switch v.(type) {
	case nil, string, bool, float64, float32, int, int64, json.Number, time.Time:
		return Stringify(v), nil
	}
	s, err := marshalCompact(v)
	if err != nil {
		return "", NewNodeError("FLOW_PARAM_INVALID", "%s cannot be written as text: it holds a value that is not valid JSON", name)
	}
	return s, nil
}

// choiceParam reads an enumerated parameter. Absent, null and blank text give def;
// text equal to one of allowed (ignoring case and blanks) gives that entry as
// spelled in allowed; anything else, including a non-text value, is not ok.
func choiceParam(v any, def string, allowed ...string) (string, bool) {
	switch x := v.(type) {
	case nil:
		return def, true
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return def, true
		}
		for _, a := range allowed {
			if strings.EqualFold(s, a) {
				return a, true
			}
		}
	}
	return "", false
}

func choiceList(allowed []string) string {
	if len(allowed) < 2 {
		return strings.Join(allowed, "")
	}
	return strings.Join(allowed[:len(allowed)-1], ", ") + " or " + allowed[len(allowed)-1]
}

// choiceIssue reports an enumerated parameter that Execute would reject. The node
// is raw: the value can be any type, and a template is only resolved at run time,
// so it cannot be judged here (Execute checks the resolved value).
func choiceIssue(n *Node, param, def string, allowed []string) []Issue {
	v := n.Params[param]
	if s, ok := v.(string); ok && HasTemplate(s) {
		return nil
	}
	if _, ok := choiceParam(v, def, allowed...); ok {
		return nil
	}
	return []Issue{paramIssue(n, IssueParamInvalid, SeverityError, param, param+" must be "+choiceList(allowed))}
}

// choiceError is the error Execute returns for a value choiceIssue reports.
func choiceError(param string, allowed []string) error {
	return NewNodeError("FLOW_PARAM_INVALID", "%s must be %s", param, choiceList(allowed))
}

// outString returns a text field of a tool output, "" when it is absent or not text.
func outString(out map[string]any, key string) string {
	s, _ := out[key].(string)
	return s
}

// outText returns the first non-blank text among the message fields of a tool
// output ("message", "error", "detail", "text"), or "". It reads text values only
// and never renders a structure, so a hostile output costs nothing.
func outText(out map[string]any) string {
	for _, key := range []string{"message", "error", "detail", "text"} {
		if s := strings.TrimSpace(outString(out, key)); s != "" {
			return s
		}
	}
	return ""
}

// requireSuccess fails closed on a tool answer that does not say it succeeded.
// callTool only reports what the answer says is wrong: a status of error or denied,
// a response status or IsError. The dispatcher, though, also refuses with plain text
// ("[PERMISSION DENIED] filesystem write operations are disabled", "pdf_extractor is
// disabled in settings"), which ParseToolOutput maps to {"text": …} without an error
// whenever the invoker does not set IsError. Every tool these nodes use answers a
// success with "status":"success" (any letter case), so anything else is not one.
// what names the tool in the message ("the file tool did not report success"); the
// text the tool gave is echoed through quoteForError. It returns a literal nil error
// on success.
func requireSuccess(out map[string]any, what string) error {
	if strings.EqualFold(strings.TrimSpace(outString(out, "status")), "success") {
		return nil
	}
	if text := outText(out); text != "" {
		return NewNodeError("FLOW_TOOL_ERROR", "the %s did not report success: %s", what, quoteForError(text))
	}
	return NewNodeError("FLOW_TOOL_ERROR", "the %s did not report success", what)
}

// flagValue reads a yes/no parameter that may arrive as a template result: a bool,
// a number (zero is off) or one of the words true, yes, 1, ja, on and false, no, off,
// 0, nein (ignoring case and blanks). known is false for anything else, null
// included, so that a caller can keep its default instead of guessing: a flag that
// protects something (fail_on_error) must only be switched off by an explicit off.
func flagValue(v any) (on, known bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "true", "yes", "1", "ja", "on":
			return true, true
		case "false", "no", "off", "0", "nein":
			return false, true
		}
		return false, false
	}
	if f, ok := toNumber(v); ok {
		return f != 0, true
	}
	return false, false
}
