package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSafeTemplateDataJSONEscapesScriptInjection(t *testing.T) {
	t.Parallel()

	want := `</script><script>alert("x")</script>`
	raw := safeTemplateDataJSON(map[string]any{
		"i18n": map[string]string{
			"danger": want,
		},
	})
	html := string(raw)
	if strings.Contains(html, "</script>") || strings.Contains(html, "<script>") {
		t.Fatalf("template data JSON contains executable script boundary: %s", html)
	}

	var parsed struct {
		I18N map[string]string `json:"i18n"`
	}
	if err := json.Unmarshal([]byte(html), &parsed); err != nil {
		t.Fatalf("template data JSON must remain parseable: %v", err)
	}
	if got := parsed.I18N["danger"]; got != want {
		t.Fatalf("parsed translation = %q, want %q", got, want)
	}
}
