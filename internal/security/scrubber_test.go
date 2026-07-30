package security

import (
	"strings"
	"testing"
)

func TestRedactSensitiveInfoRedactsShortCredentialValues(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "short_token", input: `token: abc12345`},
		{name: "short_password_equals", input: `password="hunter22"`},
		{name: "short_api_key", input: `api_key=sk-123456`},
		{name: "base64ish_secret", input: `secret: AbCdEf12+/=`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redacted := RedactSensitiveInfo(tt.input)
			if redacted == tt.input {
				t.Fatalf("expected %q to be redacted", tt.input)
			}
			if !containsRedacted(redacted) {
				t.Fatalf("expected redacted marker in %q", redacted)
			}
		})
	}
}

func TestRedactSensitiveInfoRedactsFragmentedAndEncodedValues(t *testing.T) {
	tests := []string{
		`token: s k - 1 2 3 4 5 6 7 8`,
		`secret=736563726574313233`,
		`api_key: c2VjcmV0MTIzNDU2`,
	}
	for _, input := range tests {
		if redacted := RedactSensitiveInfo(input); !containsRedacted(redacted) {
			t.Fatalf("expected redacted marker for %q, got %q", input, redacted)
		}
	}
}

func TestRedactSensitiveInfoLeavesBenignShortValuesUntouched(t *testing.T) {
	tests := []string{
		`monkey: banana`,
		`authorship: internal`,
		`token_bucket: enabled`,
		`passwordless login is enabled`,
	}

	for _, input := range tests {
		if redacted := RedactSensitiveInfo(input); redacted != input {
			t.Fatalf("expected benign input to stay unchanged: %q -> %q", input, redacted)
		}
	}
}

func TestScrubUsesVisiblePlaceholderForSensitiveValues(t *testing.T) {
	sensitiveMu.Lock()
	sensitiveValues = make(map[string]sensitiveEntry)
	scopedSensitiveValues = make(map[string]int)
	sensitiveMu.Unlock()
	RegisterSensitive("secret123")

	t.Run("exact", func(t *testing.T) {
		result := Scrub("token secret123 leaked")
		if !strings.Contains(result, "[redacted]") {
			t.Fatalf("expected visible placeholder, got %q", result)
		}
	})

	t.Run("fragmented", func(t *testing.T) {
		result := Scrub("token s e c r e t 1 2 3 leaked")
		if !strings.Contains(result, "[redacted]") {
			t.Fatalf("expected fragmented secret to be scrubbed, got %q", result)
		}
	})

	t.Run("hex", func(t *testing.T) {
		result := Scrub("token 736563726574313233 leaked")
		if !strings.Contains(result, "[redacted]") {
			t.Fatalf("expected hex secret to be scrubbed, got %q", result)
		}
	})

	t.Run("base64", func(t *testing.T) {
		result := Scrub("token c2VjcmV0MTIz leaked")
		if !strings.Contains(result, "[redacted]") {
			t.Fatalf("expected base64 secret to be scrubbed, got %q", result)
		}
	})
}

func TestRegisterSensitiveDeduplicatesAndIgnoresShortLiterals(t *testing.T) {
	sensitiveMu.Lock()
	sensitiveValues = make(map[string]sensitiveEntry)
	scopedSensitiveValues = make(map[string]int)
	sensitiveMu.Unlock()

	RegisterSensitive("duplicate-secret")
	RegisterSensitive("duplicate-secret")
	RegisterSensitive("1")

	sensitiveMu.RLock()
	count := len(sensitiveValues)
	sensitiveMu.RUnlock()
	if count != 1 {
		t.Fatalf("registered sensitive values = %d, want 1", count)
	}
	if got := Scrub("normal 1 output"); got != "normal 1 output" {
		t.Fatalf("short literal corrupted output: %q", got)
	}
	if got := RedactSensitiveInfo("pin: 1"); strings.Contains(got, "1") || !containsRedacted(got) {
		t.Fatalf("contextual short PIN was not redacted: %q", got)
	}
}

func TestScopedSensitiveExactIsRefcountedAndReleased(t *testing.T) {
	sensitiveMu.Lock()
	sensitiveValues = make(map[string]sensitiveEntry)
	scopedSensitiveValues = make(map[string]int)
	sensitiveMu.Unlock()

	const secret = "scoped-secret-value"
	releaseOne := RegisterScopedSensitiveExact(secret)
	releaseTwo := RegisterScopedSensitiveExact(secret)
	if got := Scrub("prefix " + secret + " suffix"); strings.Contains(got, secret) {
		t.Fatalf("scoped secret was not redacted: %q", got)
	}
	// Exact-only scopes deliberately do not add expanded encodings.
	if got := Scrub("73636f7065642d7365637265742d76616c7565"); !strings.Contains(got, "73636f") {
		t.Fatalf("scoped secret unexpectedly expanded to encoded patterns: %q", got)
	}
	releaseOne()
	if got := Scrub(secret); strings.Contains(got, secret) {
		t.Fatalf("first release removed a shared scope: %q", got)
	}
	releaseTwo()
	releaseTwo()
	if got := Scrub(secret); got != secret {
		t.Fatalf("released scope remained registered: %q", got)
	}
}

func TestScopedSensitiveExactAcceptsMaximumModalSizeWithoutExpansion(t *testing.T) {
	sensitiveMu.Lock()
	sensitiveValues = make(map[string]sensitiveEntry)
	scopedSensitiveValues = make(map[string]int)
	sensitiveMu.Unlock()

	secret := strings.Repeat("m", 64*1024)
	release := RegisterScopedSensitiveExact(secret)
	if got := Scrub("prefix" + secret + "suffix"); strings.Contains(got, secret) {
		t.Fatal("maximum-size scoped value was not redacted")
	}
	sensitiveMu.RLock()
	count := len(scopedSensitiveValues)
	sensitiveMu.RUnlock()
	if count != 1 {
		t.Fatalf("scoped registry entries = %d, want 1", count)
	}
	release()
	if got := Scrub(secret); got != secret {
		t.Fatal("maximum-size scoped value remained registered")
	}
}

func TestVisiblePlaceholderHelpers(t *testing.T) {
	if got := RedactedText(""); got != "[redacted]" {
		t.Fatalf("expected plain redacted placeholder, got %q", got)
	}
	if got := RedactedText("guardian blocked content"); got != "[redacted] guardian blocked content" {
		t.Fatalf("unexpected redacted helper output: %q", got)
	}
	if got := SanitizedText(""); got != "[sanitized]" {
		t.Fatalf("expected plain sanitized placeholder, got %q", got)
	}
	if got := SanitizedText("guardian scan flagged this message"); got != "[sanitized]: guardian scan flagged this message" {
		t.Fatalf("unexpected sanitized helper output: %q", got)
	}
}

func containsRedacted(value string) bool {
	return strings.Contains(value, "[redacted]")
}
