package services

import (
	"strings"
	"testing"
)

// Existing installations keep their file-index state only if plain provider URLs
// (including a trailing slash) fingerprint exactly as before.
func TestFingerprintProviderKeepsPlainBaseURLsAndHidesQueryCredentials(t *testing.T) {
	for _, base := range []string{"", "https://api.openai.com/v1", "https://api.openai.com/v1/", "http://localhost:11434//"} {
		want := strings.Join([]string{"embeddings", "provider=p", "type=t", "base=" + strings.TrimSpace(base), "model=m", "key_sha256="}, ";")
		if got := fingerprintProvider("embeddings", "p", "t", " "+base+" ", "m", ""); got != want {
			t.Errorf("fingerprintProvider(base=%q) = %q, want %q", base, got, want)
		}
	}
	const secret = "fixture-query-credential-7485"
	got := fingerprintProvider("embeddings", "p", "t", "https://api.example.com/v1?key="+secret, "m", "")
	if strings.Contains(got, secret) || !strings.Contains(got, "base=https://api.example.com/v1?q=") {
		t.Fatalf("query credential survived in file-index fingerprint: %q", got)
	}
}
