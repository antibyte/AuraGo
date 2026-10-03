package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type toolErrorEnvelope struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ErrorJSON encodes a tool failure envelope. encoding/json escapes the message,
// so quotes, backslashes and control characters in error text can neither break
// the envelope nor inject a second "status" key.
func ErrorJSON(message string) string {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(toolErrorEnvelope{Status: "error", Message: message}); err != nil {
		return `{"status":"error","message":"tool error could not be encoded"}`
	}
	return string(bytes.TrimRight(buf.Bytes(), "\n"))
}

// ErrorJSONf formats the message like fmt.Sprintf and encodes it with ErrorJSON.
func ErrorJSONf(format string, args ...any) string {
	return ErrorJSON(fmt.Sprintf(format, args...))
}
