package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONRawOrStringParsesJSON(t *testing.T) {
	value := jsonRawOrString([]byte(`{"name":"demo","count":2}`))
	parsed, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("expected parsed object, got %T", value)
	}
	if parsed["name"] != "demo" {
		t.Fatalf("name = %v, want demo", parsed["name"])
	}
}

func TestJSONRawOrStringFallsBackToString(t *testing.T) {
	raw := []byte("line1\n\"quoted\"")
	value := jsonRawOrString(raw)
	if value != string(raw) {
		t.Fatalf("value = %q, want %q", value, string(raw))
	}
}

func TestMarshalPrefixedToolJSONProducesValidJSON(t *testing.T) {
	out := marshalPrefixedToolJSON(map[string]interface{}{"status": "success", "message": "line1\n\"quoted\""})
	if !strings.HasPrefix(out, "Tool Output: ") {
		t.Fatalf("unexpected prefix: %q", out)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(out, "Tool Output: ")), &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if parsed["message"] != "line1\n\"quoted\"" {
		t.Fatalf("message = %q", parsed["message"])
	}
}
