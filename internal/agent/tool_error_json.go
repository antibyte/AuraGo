package agent

import "aurago/internal/tools"

// toolErrorJSON returns a model-facing tool failure. Never build this envelope
// with fmt.Sprintf: an error containing quotes or backslashes breaks the JSON
// (the result becomes unclassified) or can append a second "status" key.
func toolErrorJSON(message string) string {
	return "Tool Output: " + tools.ErrorJSON(message)
}

// toolErrorf formats the message like fmt.Sprintf and encodes it with toolErrorJSON.
func toolErrorf(format string, args ...any) string {
	return "Tool Output: " + tools.ErrorJSONf(format, args...)
}
