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
	sensitiveValues = nil
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
