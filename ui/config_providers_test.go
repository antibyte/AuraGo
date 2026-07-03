package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigProviderActionsUseReadableAccentContrast(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/config.css")

	pill := configProviderCSSRuleBody(t, css, ".prov-provider-pill")
	for _, want := range []string{
		"background: linear-gradient(135deg, #ccfbf1, #5eead4);",
		"color: #08312d;",
		"border: 1px solid rgba(20, 184, 166, 0.42);",
		"text-shadow: none;",
	} {
		if !strings.Contains(pill, want) {
			t.Fatalf("provider pill contrast style missing %q in block:\n%s", want, pill)
		}
	}
	if strings.Contains(pill, "color: #fff") || strings.Contains(pill, "background: var(--accent)") {
		t.Fatalf("provider pill must not use white text on the bright accent background:\n%s", pill)
	}

	addButton := configProviderCSSRuleBody(t, css, ".prov-section-actions .btn-save.prov-btn-sm")
	for _, want := range []string{
		"background: linear-gradient(135deg, #0f766e, #115e59);",
		"color: #f0fdfa;",
		"border: 1px solid rgba(94, 234, 212, 0.42);",
		"text-shadow: none;",
	} {
		if !strings.Contains(addButton, want) {
			t.Fatalf("provider add button contrast style missing %q in block:\n%s", want, addButton)
		}
	}
}

func TestConfigCSSCacheBustForProviderContrast(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "config.html")
	if !strings.Contains(html, `/css/config.css?v=20260520a`) {
		t.Fatal("config.html must bust config.css cache for provider contrast styling")
	}
}

func TestConfigProvidersGuardLoadFailureBeforeMutation(t *testing.T) {
	t.Parallel()

	mainJS := readDesktopAssetText(t, "js/config/main.js")
	providersJS := readDesktopAssetText(t, "cfg/providers.js")
	for _, marker := range []string{
		"let providersLoaded = false;",
		"let providersLoadError = '';",
		"async function loadProviders()",
		"providersLoaded = true;",
		"providersLoadError =",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing provider load guard marker %q", marker)
		}
	}
	for _, marker := range []string{
		"config.providers.load_failed_title",
		"config.providers.retry_load",
		"async function providerRetryLoad()",
		"function providerEnsureLoaded()",
		"if (!providerEnsureLoaded()) return;",
		"if (!providerEnsureLoaded()) return false;",
	} {
		if !strings.Contains(providersJS, marker) {
			t.Fatalf("providers.js missing provider mutation guard marker %q", marker)
		}
	}
}

func TestConfigProvidersPricingRequiresSavedNonOpenRouterProvider(t *testing.T) {
	t.Parallel()

	providersJS := readDesktopAssetText(t, "cfg/providers.js")
	for _, marker := range []string{
		"const providerIsSaved =",
		"provType === 'openrouter'",
		"config.providers.pricing_save_first",
		"url = '/api/providers/pricing?id=' + encodeURIComponent(provId);",
		"url = '/api/openrouter/models';",
	} {
		if !strings.Contains(providersJS, marker) {
			t.Fatalf("providers.js missing safe pricing marker %q", marker)
		}
	}
	if strings.Contains(providersJS, "} else {\r\n                    url = '/api/openrouter/models';") ||
		strings.Contains(providersJS, "} else {\n                    url = '/api/openrouter/models';") {
		t.Fatal("providers.js still falls back to OpenRouter pricing for every unsaved provider")
	}
}

func TestConfigProvidersWarnBeforeRiskyModalAndDeleteActions(t *testing.T) {
	t.Parallel()

	providersJS := readDesktopAssetText(t, "cfg/providers.js")
	for _, marker := range []string{
		"function providerModalDirty()",
		"config.providers.discard_changes_confirm_title",
		"config.providers.discard_changes_confirm",
		"function providerReferenceLabel(ref)",
		"p.references",
		"config.providers.delete_references_warning",
	} {
		if !strings.Contains(providersJS, marker) {
			t.Fatalf("providers.js missing safer action marker %q", marker)
		}
	}
}

func TestConfigProvidersCopilotAndGuardTranslationsExistInAllLocales(t *testing.T) {
	t.Parallel()

	required := []string{
		"config.providers.load_failed_title",
		"config.providers.load_failed_body",
		"config.providers.retry_load",
		"config.providers.load_required",
		"config.providers.pricing_save_first",
		"config.providers.discard_changes_confirm_title",
		"config.providers.discard_changes_confirm",
		"config.providers.delete_references_warning",
		"config.providers.delete_references_item",
		"config.providers.copilot_authorized",
		"config.providers.copilot_not_authorized",
		"config.providers.copilot_requesting",
		"config.providers.copilot_start_auth",
		"config.providers.copilot_visit_code",
		"config.providers.copilot_check_auth",
		"config.providers.copilot_waiting",
		"config.providers.copilot_auth_success",
		"config.providers.copilot_auth_failed",
		"config.providers.copilot_error",
		"config.providers.copilot_unknown_error",
	}
	files, err := filepath.Glob(filepath.Join("lang", "config", "providers", "*.json"))
	if err != nil {
		t.Fatalf("glob provider translations: %v", err)
	}
	if len(files) < 15 {
		t.Fatalf("expected all provider language files, got %d", len(files))
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("unmarshal %s: %v", path, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing %s", path, key)
			}
		}
	}
}

func TestConfigProvidersCopilotFlowUsesI18n(t *testing.T) {
	t.Parallel()

	providersJS := readDesktopAssetText(t, "cfg/providers.js")
	for _, forbidden := range []string{
		"Requesting...",
		"Start GitHub Authorization",
		"Waiting for authorization...",
		"Visit the URL below and enter the code:",
		"Check Authorization",
		"GitHub Copilot authorized successfully",
		"Copilot auth failed:",
	} {
		if strings.Contains(providersJS, forbidden) {
			t.Fatalf("providers.js contains hardcoded Copilot copy %q", forbidden)
		}
	}
}

func configProviderCSSRuleBody(t *testing.T, source, selector string) string {
	t.Helper()

	needle := "\n" + selector + " {"
	start := strings.Index(source, needle)
	if start < 0 {
		t.Fatalf("config CSS missing selector %q", selector)
	}
	start++
	open := strings.Index(source[start:], "{")
	if open < 0 {
		t.Fatalf("config CSS selector %q is missing opening brace", selector)
	}
	bodyStart := start + open + 1
	close := strings.Index(source[bodyStart:], "}")
	if close < 0 {
		t.Fatalf("config CSS selector %q is missing closing brace", selector)
	}
	return source[bodyStart : bodyStart+close]
}
