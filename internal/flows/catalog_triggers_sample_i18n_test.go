package flows

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// samplePath reads a dotted path ("zones.0" is the first zone) from a JSON-like value.
func samplePath(v any, path string) any {
	for _, part := range strings.Split(path, ".") {
		switch m := v.(type) {
		case map[string]any:
			v = m[part]
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(m) {
				return nil
			}
			v = m[i]
		default:
			return nil
		}
	}
	return v
}

// TestTriggerSampleTextKeys: every named text field is a text field of its built-in sample,
// the definition carries the keys for its catalog entry, and CatalogI18nKeys lists them, so
// the 16-locale test (catalog_i18n_test.go) covers them. No built-in sample is German.
func TestTriggerSampleTextKeys(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterCatalog(reg, nil); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, k := range CatalogI18nKeys(reg) {
		listed[k] = true
	}
	if len(sampleTextKeys) == 0 {
		t.Fatal("no sample text keys")
	}
	for typ, byEvent := range sampleTextKeys {
		tt, ok := triggerTypes()[typ]
		if !ok || len(tt.def.SampleTextKeys) != len(byEvent) {
			t.Errorf("%s: definition does not carry its sample text keys", typ)
		}
		for event, fields := range byEvent {
			params := map[string]any{}
			if event != "" {
				params["event"] = event
			}
			sample := TriggerSample(&Node{Type: typ, Params: params})
			for path, key := range fields {
				if _, ok := samplePath(sample, path).(string); !ok {
					t.Errorf("%s %q: %s is not a text field of %v", typ, event, path, sample)
				}
				if !listed[key] {
					t.Errorf("CatalogI18nKeys misses %s", key)
				}
			}
		}
	}
	for typ := range triggerTypes() {
		for _, event := range []string{"", "disconnected", "todo_overdue"} {
			text := fmt.Sprint(TriggerSample(&Node{Type: typ, Params: map[string]any{"event": event}}))
			for _, word := range []string{"Hallo", "Rechnung", "Anruf", "Zahnarzt", "Einfahrt"} {
				if strings.Contains(text, word) {
					t.Errorf("%s %q: built-in sample is German (%s): %s", typ, event, word, text)
				}
			}
		}
	}
}
