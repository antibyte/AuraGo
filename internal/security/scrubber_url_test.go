package security

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func TestScrubRegisteredURLForms(t *testing.T) {
	percent := regexp.MustCompile(`%[0-9A-Fa-f]{2}`)
	for _, scoped := range []bool{false, true} {
		value := "CaseSensitive+/ space?=#value"
		if scoped {
			value = "TemporaryCase+/ different?=#secret"
		}
		var release func()
		if scoped {
			release = RegisterScopedSensitiveExact(value)
		} else {
			RegisterSensitive(value)
		}
		for _, encode := range []func(string) string{url.QueryEscape, url.PathEscape} {
			encoded := encode(value)
			mixed := percent.ReplaceAllStringFunc(encoded, func(s string) string { return strings.ToLower(s) })
			for _, form := range []string{value, encoded, mixed} {
				for _, scrub := range []func(string) string{Scrub, ScrubSecrets} {
					if got := scrub("https://example.test/" + form); got != "https://example.test/[redacted]" {
						t.Fatalf("scoped=%v: URL form escaped redaction: %q", scoped, got)
					}
				}
			}
			if got := Scrub(strings.ToLower(encoded)); strings.Contains(got, "[redacted]") {
				t.Fatal("percent matching changed the case sensitivity of literal secret bytes")
			}
		}
		if release != nil {
			release()
			release()
			if got := Scrub(url.QueryEscape(value)); got != url.QueryEscape(value) {
				t.Fatal("released temporary URL form remained registered")
			}
		}
	}
	short := "a+b"
	release := RegisterScopedSensitiveExact(short)
	defer release()
	RegisterSensitive(short)
	if got := Scrub(url.QueryEscape(short)); got != url.QueryEscape(short) {
		t.Fatal("short literals must retain the minimum registration length")
	}
}
