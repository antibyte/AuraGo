package agent

import (
	"encoding/json"
	"regexp"
	"strings"
)

// A native tool call whose arguments do not decode used to be answered with one
// generic sentence, so a weak model resent the same bad payload again and again.
// The decoder knows which argument it choked on (ToolCall.NativeArgsError holds
// the text of Go's error); these helpers turn that text into a model-facing
// message that names the argument, what was sent and what is expected.

const (
	invalidArgumentsGenericMessage = "Supply a complete JSON object matching the tool schema."
	invalidArgumentsResendHint     = "Resend the call with that value corrected and send only the arguments you need: omit optional arguments instead of sending empty or default values."
	invalidArgumentsObjectMessage  = "The arguments must be one JSON object that maps argument names to values. Resend the call with a complete JSON object."
	invalidArgumentsSyntaxMessage  = "The arguments are not one complete JSON object (they may have been cut off). Resend the call with a complete JSON object."
)

var (
	// json: cannot unmarshal <kind>[ <number text>] into Go struct field <Struct>.<field path> of type <GoType>
	// The struct name never contains a dot or a space; the field path never contains a space.
	nativeArgsTypeMismatchPattern = regexp.MustCompile(`^json: cannot unmarshal ([a-z]+)(?: \S+)? into Go struct field [^\s.]*\.(\S+) of type (.+)$`)
	// json: cannot unmarshal <kind> into Go value of type <GoType> (the top level is not an object)
	nativeArgsNotObjectPattern = regexp.MustCompile(`^json: cannot unmarshal [a-z]+(?: \S+)? into Go value of type .+$`)
	// encoding/json syntax errors.
	nativeArgsSyntaxPattern = regexp.MustCompile(`^(?:unexpected end of JSON input|invalid character .+)$`)
	// Map keys under a map-typed field come from the model, so the path is checked
	// before it is echoed back.
	nativeArgsFieldPathPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,63}$`)
)

// invalidNativeArgsDetail explains a native argument decoding error to the
// model. It returns the offending argument (empty when the error names none) and
// a message. It returns ("", "") whenever it cannot name something useful, which
// selects the generic wording.
func invalidNativeArgsDetail(errText string) (field, message string) {
	errText = strings.TrimSpace(errText)
	if match := nativeArgsTypeMismatchPattern.FindStringSubmatch(errText); match != nil {
		path := match[2]
		if !nativeArgsFieldPathPattern.MatchString(path) {
			return "", ""
		}
		sent := invalidArgsJSONKind(match[1])
		if want := invalidArgsGoType(match[3]); want != "" {
			return path, "Argument '" + path + "' was sent as " + sent + " but must be " + want + ". " + invalidArgumentsResendHint
		}
		return path, "Argument '" + path + "' was sent as " + sent + " and has the wrong type for this tool. " + invalidArgumentsResendHint
	}
	if nativeArgsNotObjectPattern.MatchString(errText) {
		return "", invalidArgumentsObjectMessage
	}
	if nativeArgsSyntaxPattern.MatchString(errText) {
		return "", invalidArgumentsSyntaxMessage
	}
	return "", ""
}

// invalidArgsJSONKind names the JSON kind reported by encoding/json.
func invalidArgsJSONKind(kind string) string {
	switch kind {
	case "bool":
		return "a boolean"
	case "string":
		return "a string"
	case "number":
		return "a number"
	case "array":
		return "an array"
	case "object":
		return "an object"
	}
	return "a value of another type"
}

// invalidArgsGoType names the JSON type a Go type expects. It returns "" for a
// type that has no plain JSON counterpart (custom types).
func invalidArgsGoType(goType string) string {
	goType = strings.TrimLeft(strings.TrimSpace(goType), "*")
	switch goType {
	case "string":
		return "a string"
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return "an integer"
	case "float32", "float64":
		return "a number"
	case "bool":
		return "a boolean"
	}
	switch {
	case strings.HasPrefix(goType, "[]"):
		return "an array"
	case strings.HasPrefix(goType, "map["):
		return "an object"
	}
	return ""
}

// invalidArgumentsEnvelope is the tool result for rejected arguments. The key
// order is part of the contract.
type invalidArgumentsEnvelope struct {
	Status  string `json:"status"`
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// invalidArgumentsResult returns the JSON text of the tool result for a native
// call whose arguments did not decode. Without usable detail it is the generic
// result, byte for byte.
func invalidArgumentsResult(errText string) string {
	field, message := invalidNativeArgsDetail(errText)
	if message == "" {
		field, message = "", invalidArgumentsGenericMessage
	}
	encoded, err := json.Marshal(invalidArgumentsEnvelope{Status: "error", Code: "invalid_arguments", Field: field, Message: message})
	if err != nil {
		return `{"status":"error","code":"invalid_arguments","message":"` + invalidArgumentsGenericMessage + `"}`
	}
	return string(encoded)
}

// FormatInvalidNativeToolArgsFeedback is FormatInvalidNativeToolFeedback with the
// decoder's finding added: the detail follows the sentence that says the call was
// discarded and the remaining instructions are kept. Without detail it returns
// the generic feedback unchanged.
func FormatInvalidNativeToolArgsFeedback(toolName, errText string) string {
	base := FormatInvalidNativeToolFeedback(toolName)
	_, detail := invalidNativeArgsDetail(errText)
	if detail == "" {
		return base
	}
	// The tool name precedes this sentence, so the last occurrence is the fixed one.
	const marker = "was discarded. "
	if index := strings.LastIndex(base, marker); index >= 0 {
		cut := index + len(marker)
		return base[:cut] + detail + " " + base[cut:]
	}
	return base + " " + detail
}
