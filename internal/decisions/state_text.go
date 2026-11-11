package decisions

import (
	"bytes"
	"encoding/json"
)

// EncodeStateText renders a decision state as the text the model reads. A string state is
// returned as it is. Anything else becomes JSON without HTML escaping, so the
// <external_data> markers that fence untrusted values stay readable (json.Marshal would
// write "<" as an escape sequence). The encoder's trailing newline is cut. A state without
// <, > or & gives the same bytes as json.Marshal. buildPrompt, openRouterTextState and
// ValidateRequest all use it, so the size check measures the bytes that are sent.
func EncodeStateText(state any) (string, error) {
	if text, ok := state.(string); ok {
		return text, nil
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(state); err != nil {
		return "", err
	}
	return string(bytes.TrimSuffix(buf.Bytes(), []byte("\n"))), nil
}
