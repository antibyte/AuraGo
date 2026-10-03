package tools

import (
	"encoding/json"
	"testing"
)

func TestErrorJSONEscapesMessage(t *testing.T) {
	message := `quote " backslash \ newline` + "\n" + `injected","status":"ok`
	out := ErrorJSON(message)
	var decoded map[string]string
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("ErrorJSON produced invalid JSON %s: %v", out, err)
	}
	if decoded["status"] != "error" || decoded["message"] != message || len(decoded) != 2 {
		t.Fatalf("decoded envelope = %#v", decoded)
	}
	if got := ErrorJSONf("code %d: %s", 7, "<x>"); got != `{"status":"error","message":"code 7: <x>"}` {
		t.Fatalf("ErrorJSONf = %s", got)
	}
}
