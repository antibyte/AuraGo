package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestConfigWave0ListsDoNotCacheFailedLoads(t *testing.T) {
	email := string(mustReadUIFile(t, "cfg/email.js"))
	mcp := string(mustReadUIFile(t, "cfg/mcp.js"))

	for name, src := range map[string]string{"email.js": email, "mcp.js": mcp} {
		for _, bad := range []string{
			`resp.ok ? await resp.json() : []`,
			`resp.ok ? await resp.json() : {}`,
			`Cache = [];`,
			`mcpPreferencesCache = {};`,
		} {
			if strings.Contains(src, bad) {
				t.Fatalf("%s still caches an empty value on a failed load: %q", name, bad)
			}
		}
	}
	for _, want := range []string{"emailLoadError", "emailRetryLoad", "emailAccountsCache = nextAccounts"} {
		if !strings.Contains(email, want) {
			t.Fatalf("email.js missing %q", want)
		}
	}
	// the cache must only be replaced after the PUT succeeded
	put := strings.Index(email, "method: 'PUT'")
	assign := strings.Index(email, "emailAccountsCache = nextAccounts")
	if put < 0 || assign < put {
		t.Fatal("email.js must assign the cache only after the PUT")
	}
	for _, want := range []string{"mcpLoadErrors", "mcpRetryLoad", "mcp-m-error", "mcp-s-error"} {
		if !strings.Contains(mcp, want) {
			t.Fatalf("mcp.js missing %q", want)
		}
	}
	// the server modal closes only after a successful save
	save := strings.Index(mcp, "const res = await mcpServerSave(next);")
	closeAt := strings.Index(mcp[save:], "overlay.remove();")
	failAt := strings.Index(mcp[save:], "if (!res.ok)")
	if save < 0 || closeAt < 0 || failAt < 0 || failAt > closeAt {
		t.Fatal("mcp.js server modal must stay open when the PUT fails")
	}
}

func TestConfigWave0SecretsModalClosesAfterSuccess(t *testing.T) {
	src := string(mustReadUIFile(t, "cfg/secrets.js"))
	start := strings.Index(src, "document.getElementById('secret-save-btn').onclick")
	if start < 0 {
		t.Fatal("secrets save handler not found")
	}
	body := src[start:]
	awaitAt := strings.Index(body, "await onSave(key, value)")
	closeAt := strings.Index(body, "overlay.remove();")
	if awaitAt < 0 || closeAt < awaitAt {
		t.Fatal("secrets modal must close only after onSave succeeded")
	}
	for _, want := range []string{"secret-modal-error", "saveBtn.disabled = true", "secretsEsc(txt)"} {
		if !strings.Contains(src, want) {
			t.Fatalf("secrets.js missing %q", want)
		}
	}
	if regexp.MustCompile(`<div class="field-label">(Key|Value)</div>`).MatchString(src) || strings.Contains(src, `>system</span>`) {
		t.Fatal("secrets.js still has hard-coded English labels")
	}
}

func TestConfigWave0ComposioToggleSendsOnlyToolkits(t *testing.T) {
	src := string(mustReadUIFile(t, "cfg/composio.js"))
	for _, want := range []string{
		"{ saved: true }",
		"composioSaveSelection(false, nextToolkits)",
		"{ toolkits: toolkitsOnly }",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("composio.js missing %q", want)
		}
	}
}

func TestConfigWave0WebhooksReportFailures(t *testing.T) {
	src := string(mustReadUIFile(t, "cfg/webhooks.js"))
	for _, want := range []string{"whLoadState", "whRetryLoad", "notRunning", "whShowError('wh-panel-tokens'", "whShowError('wh-outgoing-list'", "og-save-btn", "wh-editor-save-btn"} {
		if !strings.Contains(src, want) {
			t.Fatalf("webhooks.js missing %q", want)
		}
	}
	if strings.Contains(src, "if (!resp.ok) return;\n        whLog") {
		t.Fatal("webhook log must show an error on a non-OK response")
	}
}

func TestConfigWave0ListsLocalesHaveNewKeys(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := map[string][]string{
		"email":    {"load_failed", "load_failed_disabled", "retry", "saved"},
		"mcp":      {"load_failed", "load_failed_disabled", "retry"},
		"secrets":  {"key_column", "system_badge", "key_label", "value_label"},
		"webhooks": {"load_failed", "retry", "not_running_notice", "not_running_desc"},
	}
	for module, ks := range keys {
		for _, loc := range locales {
			raw, err := os.ReadFile(filepath.Join("lang", "config", module, loc+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var m map[string]string
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatalf("%s/%s: %v", module, loc, err)
			}
			for _, k := range ks {
				if strings.TrimSpace(m["config."+module+"."+k]) == "" {
					t.Fatalf("%s/%s missing config.%s.%s", module, loc, module, k)
				}
			}
		}
	}
}
