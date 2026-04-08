package llm

import "testing"

func TestLookupKnownContextWindow_PrefixMatchingOnly(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		wantSize int
		wantOK   bool
	}{
		{name: "direct prefix match", model: "qwen2.5-14b", wantSize: 131_072, wantOK: true},
		{name: "provider style segment match", model: "openrouter/qwen2.5-14b", wantSize: 131_072, wantOK: true},
		{name: "slash model match", model: "meta-llama/llama-3.1-70b-instruct", wantSize: 131_072, wantOK: true},
		{name: "no contains fallback", model: "my-custom-qwen-adapter", wantSize: 0, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := lookupKnownContextWindow(tt.model)
			if ok != tt.wantOK {
				t.Fatalf("ok=%v, want %v (got=%d)", ok, tt.wantOK, got)
			}
			if got != tt.wantSize {
				t.Fatalf("got=%d, want %d (ok=%v)", got, tt.wantSize, ok)
			}
		})
	}
}

