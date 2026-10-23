package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func fingerprintQueryDigest(rawQuery string) string {
	sum := sha256.Sum256([]byte(rawQuery))
	return "q=" + hex.EncodeToString(sum[:])[:16]
}

// Plain endpoints must keep today's exact fingerprint so existing installations
// do not re-embed their corpus.
func TestFingerprintEmbeddingBaseURLKeepsPlainURLsByteIdentical(t *testing.T) {
	for _, raw := range []string{
		"",
		"   ",
		"http://localhost:11434",
		"http://localhost:11434/",
		"  http://localhost:11434/v1/  ",
		"https://api.openai.com/v1",
		"https://api.openai.com/v1/",
		"https://api.openai.com/v1//",
		"https://openrouter.ai/api/v1",
		"http://[::1]:8080/v1/",
		"https://host.example/a/b/c",
		"https://host.example/v1?",
		"https://host.example/v1#",
		"http://[::1",
		"not a url",
		"%zz",
	} {
		want := strings.TrimRight(strings.TrimSpace(raw), "/")
		if got := FingerprintEmbeddingBaseURL(raw); got != want {
			t.Errorf("FingerprintEmbeddingBaseURL(%q) = %q, want byte-identical %q", raw, got, want)
		}
	}
}

func TestFingerprintEmbeddingBaseURLHidesQueryCredentials(t *testing.T) {
	const secret = "sk-fixture-query-credential-7485"
	query := FingerprintEmbeddingBaseURL("https://api.example.com/v1?key=" + secret)
	if strings.Contains(query, secret) || strings.Contains(query, "key=") {
		t.Fatalf("query credential survived in fingerprint: %q", query)
	}
	if want := "https://api.example.com/v1?" + fingerprintQueryDigest("key="+secret); query != want {
		t.Fatalf("query fingerprint = %q, want %q", query, want)
	}
	if again := FingerprintEmbeddingBaseURL(" https://api.example.com/v1?key=" + secret + " "); again != query {
		t.Fatalf("fingerprint is not deterministic: %q vs %q", again, query)
	}
	other := FingerprintEmbeddingBaseURL("https://api.example.com/v1?key=" + secret + "-rotated")
	if other == query {
		t.Fatal("different queries produced one fingerprint")
	}

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"userinfo", "https://alice:fixture-password-7485@api.example.com/v1", "https://api.example.com/v1"},
		{"user only", "https://token-7485@api.example.com/v1/", "https://api.example.com/v1"},
		{"fragment", "https://api.example.com/v1#access_token=fixture-7485", "https://api.example.com/v1"},
		{"all three", "https://alice:pw@api.example.com/v1/?api-version=2024-02-01#frag", "https://api.example.com/v1/?" + fingerprintQueryDigest("api-version=2024-02-01")},
		{"trailing slash before query", "https://api.example.com/v1/?key=" + secret, "https://api.example.com/v1/?" + fingerprintQueryDigest("key="+secret)},
	}
	for _, tc := range cases {
		got := FingerprintEmbeddingBaseURL(tc.raw)
		if got != tc.want {
			t.Errorf("%s: FingerprintEmbeddingBaseURL(%q) = %q, want %q", tc.name, tc.raw, got, tc.want)
		}
		for _, leaked := range []string{secret, "fixture-password-7485", "token-7485", "fixture-7485", "alice", "frag"} {
			if strings.Contains(got, leaked) {
				t.Errorf("%s: fingerprint %q leaks %q", tc.name, got, leaked)
			}
		}
	}
}

// The file indexer keeps trailing slashes in its provider fingerprints today; the
// shared redaction must not trim them, or every indexed file would be re-embedded.
func TestRedactURLForFingerprintKeepsTrailingSlashAndHidesCredentials(t *testing.T) {
	for _, raw := range []string{"", "https://api.openai.com/v1", "https://api.openai.com/v1/", "http://localhost:11434//"} {
		if got, want := RedactURLForFingerprint(" "+raw+" "), strings.TrimSpace(raw); got != want {
			t.Errorf("RedactURLForFingerprint(%q) = %q, want %q", raw, got, want)
		}
	}
	got := RedactURLForFingerprint("https://bob:pw@api.example.com/v1/?key=fixture-secret-7485#x")
	if want := "https://api.example.com/v1/?" + fingerprintQueryDigest("key=fixture-secret-7485"); got != want {
		t.Fatalf("RedactURLForFingerprint = %q, want %q", got, want)
	}
}

func TestBuildEmbeddingFingerprintKeepsExistingFormatAndHidesQueryCredentials(t *testing.T) {
	if got, want := buildEmbeddingFingerprint(" external ", " http://localhost:11434/ ", " nomic-embed-text "), "external|http://localhost:11434|nomic-embed-text"; got != want {
		t.Fatalf("plain fingerprint = %q, want old format %q", got, want)
	}
	if got, want := buildEmbeddingFingerprint("disabled", "", ""), "disabled||"; got != want {
		t.Fatalf("disabled fingerprint = %q, want %q", got, want)
	}
	const secret = "fixture-query-credential-7485"
	got := buildEmbeddingFingerprint("external", "https://api.example.com/v1?key="+secret, "m")
	if strings.Contains(got, secret) || !strings.HasPrefix(got, "external|https://api.example.com/v1?q=") || !strings.HasSuffix(got, "|m") {
		t.Fatalf("query fingerprint = %q", got)
	}
}
