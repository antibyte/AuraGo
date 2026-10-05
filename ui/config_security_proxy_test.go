package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecurityProxyFeedbackTranslationsCoverAllLocales(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	groups := map[string][]string{
		filepath.Join("lang", "config", "security_proxy"): {
			"config.security_proxy.action_pending",
			"config.security_proxy.rate_limit_build_hint",
		},
		filepath.Join("lang", "backend"): {
			"backend.proxy_basic_auth_credentials_missing",
			"backend.proxy_basic_auth_credentials_invalid",
			"backend.proxy_rate_limit_image_unavailable",
			"backend.proxy_docker_placement_failed",
			"backend.proxy_caddy_exited",
			"backend.proxy_config_rejected",
			"backend.proxy_not_running",
		},
	}
	for dir, keys := range groups {
		english := readLocaleStrings(t, filepath.Join(dir, "en.json"))
		for _, locale := range locales {
			values := readLocaleStrings(t, filepath.Join(dir, locale+".json"))
			for _, key := range keys {
				value := strings.TrimSpace(values[key])
				if value == "" {
					t.Errorf("%s/%s.json lacks %s", dir, locale, key)
					continue
				}
				if locale != "en" && value == english[key] {
					t.Errorf("%s/%s.json copies the English text for %s", dir, locale, key)
				}
			}
		}
	}
}

func TestSecurityProxyConfigExplainsRateLimitBuildAndPendingActions(t *testing.T) {
	module := string(mustReadUIFile(t, "cfg/security_proxy.js"))
	for _, required := range []string{
		"t('config.security_proxy.rate_limit_build_hint')",
		"t('config.security_proxy.action_pending')",
		"btn.disabled = true",
		"btn.disabled = false",
		`role="status" aria-live="polite"`,
	} {
		if !strings.Contains(module, required) {
			t.Errorf("security proxy config module lacks %q", required)
		}
	}
	for _, forbidden := range []string{"alert(", "confirm(", "prompt("} {
		if strings.Contains(module, forbidden) {
			t.Errorf("security proxy config module uses forbidden browser dialog %q", forbidden)
		}
	}
}

func readLocaleStrings(t *testing.T, path string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var values map[string]string
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return values
}
