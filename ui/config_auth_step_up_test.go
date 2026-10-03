package ui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestConfigAuthSendsStepUpCredentials(t *testing.T) {
	data, err := os.ReadFile("cfg/auth.js")
	if err != nil {
		t.Fatalf("read cfg/auth.js: %v", err)
	}
	js := string(data)
	for _, want := range []string{
		`id="auth-current-pw"`,
		`id="auth-current-totp"`,
		`id="auth-totp-disable-current-pw"`,
		`id="auth-totp-disable-current-totp"`,
		`id="totp-setup-current-pw"`,
		`autocomplete="current-password"`,
		`autocomplete="one-time-code"`,
		"current_password: pw",
		"current_totp_code: code",
		"Object.assign({ new_password: pw }, stepUp)",
		"Object.assign({ secret: _totpNewSecret, code }, stepUp)",
		"body: JSON.stringify(stepUp)",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("cfg/auth.js missing %q", want)
		}
	}
}

func TestConfigAuthStepUpTranslationsCoverAllLocales(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	groups := map[string][]string{
		"lang/config/auth/": {
			"config.auth.current_password_placeholder",
			"config.auth.current_totp_placeholder",
			"config.auth.current_password_required",
			"config.auth.current_totp_required",
			"config.auth.step_up_hint",
		},
		"lang/backend/": {
			"backend.auth_current_password_required",
			"backend.auth_current_totp_required",
		},
	}
	for dir, keys := range groups {
		english := readStepUpLocale(t, dir+"en.json")
		for _, locale := range locales {
			values := readStepUpLocale(t, dir+locale+".json")
			for _, key := range keys {
				value, _ := values[key].(string)
				if strings.TrimSpace(value) == "" {
					t.Fatalf("%s%s.json missing %q", dir, locale, key)
				}
				if locale != "en" && value == english[key] {
					t.Fatalf("%s%s.json uses the English text for %q", dir, locale, key)
				}
			}
		}
	}
}

func readStepUpLocale(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return values
}
