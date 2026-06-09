package ui

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestConfigComposioAPIKeyParticipatesInNormalConfigSave(t *testing.T) {
	t.Parallel()

	composioJS := readDesktopAssetText(t, "cfg/composio.js")
	for _, marker := range []string{
		"async function renderComposioSection",
		`data-path="composio.api_key"`,
		`key: 'composio_api_key'`,
		"/api/composio/status",
		"/api/vault/secrets",
	} {
		if !strings.Contains(composioJS, marker) {
			t.Fatalf("composio config module missing marker %q", marker)
		}
	}
	if strings.Contains(composioJS, "alert(") {
		t.Fatal("composio config module must use modals/toasts instead of alert()")
	}
}

func TestConfigComposioPickerUsesOpaqueModalSurfaces(t *testing.T) {
	t.Parallel()

	configCSS := strings.ReplaceAll(readDesktopAssetText(t, "css/config.css"), "\r\n", "\n")
	for _, marker := range []string{
		".cmp-modal-overlay",
		"background: rgba(2, 6, 23, 0.86);",
		"backdrop-filter: blur(8px);",
		".cmp-modal {\n",
		"background: var(--bg-primary);",
		".cmp-list-panel {\n    border-right: 1px solid var(--border-subtle);\n    background: var(--bg-secondary);",
		".cmp-detail-panel {\n    padding: 1.1rem;\n    background: var(--bg-primary);",
	} {
		if !strings.Contains(configCSS, marker) {
			t.Fatalf("composio modal CSS missing opaque surface marker %q", marker)
		}
	}
}

func TestConfigComposioButtonStylesFitLocalizedLabels(t *testing.T) {
	t.Parallel()

	configCSS := strings.ReplaceAll(readDesktopAssetText(t, "css/config.css"), "\r\n", "\n")
	for _, marker := range []string{
		".cmp-toolbar .cfg-save-btn-sm,\n.cmp-modal-controls .cfg-save-btn-sm,\n.cmp-detail-head .cfg-save-btn-sm,\n.cmp-detail-actions .cfg-save-btn-sm",
		".cmp-secret-row .cfg-save-btn-sm",
		"min-width: max-content;",
		"min-height: 36px;",
		"line-height: 1.2;",
		".cmp-small-toggle {\n    min-width: 96px;",
	} {
		if !strings.Contains(configCSS, marker) {
			t.Fatalf("composio button CSS missing localized-label marker %q", marker)
		}
	}
}

func TestConfigComposioTranslationsUseNativeDiacritics(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]string{
		"de": {
			"config.composio.allow_nl":               "Natürliche Sprache erlauben",
			"config.composio.api_key_saved":          "API-Schlüssel im Vault gespeichert",
			"config.composio.close":                  "Schließen",
			"config.composio.filter_selected":        "Ausgewählt",
			"config.composio.status_missing_api_key": "API-Schlüssel fehlt",
		},
		"da": {
			"config.composio.allow_destructive":  "Tillad destruktive værktøjer",
			"config.composio.api_key_saved":      "API-nøgle gemt i vault",
			"config.composio.no_tools":           "Ingen værktøjer",
			"config.composio.search_placeholder": "Søg toolkits, f.eks. github eller gmail",
			"config.composio.status_loading":     "Indlæser Composio-status...",
		},
		"sv": {
			"config.composio.allow_nl":           "Tillåt naturligt språk",
			"config.composio.close":              "Stäng",
			"config.composio.modal_subtitle":     "Välj toolkits, anslut konton och styr riskpolicy.",
			"config.composio.open_picker":        "Bläddra bland integrationer",
			"config.composio.status_ready":       "Composio är redo",
			"config.composio.search_placeholder": "Sök toolkits, t.ex. github eller gmail",
		},
		"no": {
			"config.composio.allow_destructive":  "Tillat destruktive verktøy",
			"config.composio.allow_nl":           "Tillat naturlig språk",
			"config.composio.api_key_saved":      "API-nøkkel lagret i vault",
			"config.composio.open_picker":        "Bla gjennom integrasjoner",
			"config.composio.search_placeholder": "Søk toolkits, f.eks. github eller gmail",
		},
		"cs": {
			"config.composio.accounts":               "Účty",
			"config.composio.api_key_saved":          "API klíč uložen ve vaultu",
			"config.composio.status_missing_api_key": "Chybí API klíč",
			"config.composio.tools_preview":          "Náhled nástrojů",
		},
		"pl": {
			"config.composio.allow_nl":       "Zezwól na język naturalny",
			"config.composio.connect":        "Połącz",
			"config.composio.no_results":     "Brak wyników",
			"config.composio.status_loading": "Ładowanie statusu Composio...",
			"config.composio.tools_preview":  "Podgląd narzędzi",
		},
		"fr": {
			"config.composio.allowed":       "Autorisé",
			"config.composio.api_key_saved": "Clé API enregistrée dans le vault",
			"config.composio.modal_title":   "Intégrations Composio",
			"config.composio.tools_preview": "Aperçu des outils",
		},
		"es": {
			"config.composio.modal_subtitle":  "Elija toolkits, conecte cuentas y controle la política de riesgo.",
			"config.composio.save_selection":  "Guardar selección",
			"config.composio.status_ready":    "Composio está listo",
			"config.composio.test_connection": "Probar conexión",
		},
		"pt": {
			"config.composio.modal_title":     "Integrações Composio",
			"config.composio.open_picker":     "Explorar integrações",
			"config.composio.save_selection":  "Salvar seleção",
			"config.composio.test_connection": "Testar conexão",
		},
	}

	for lang, expected := range cases {
		values := readComposioLangMap(t, lang)
		for key, want := range expected {
			if got := values[key]; got != want {
				t.Fatalf("%s %s = %q, want %q", lang, key, got, want)
			}
		}
	}
}

func TestConfigComposioInlineHandlersEscapeJSONStringArguments(t *testing.T) {
	t.Parallel()

	composioJS := readDesktopAssetText(t, "cfg/composio.js")
	if !strings.Contains(composioJS, `return escapeAttr(JSON.stringify(String(value || '')));`) {
		t.Fatal("composioJSArg must HTML-escape JSON strings before embedding them in double-quoted inline handlers")
	}
	if strings.Contains(composioJS, `return JSON.stringify(String(value || ''));`) {
		t.Fatal("composioJSArg must not return raw JSON strings for inline HTML attributes")
	}
}

func TestConfigComposioModuleUsesBuildVersionCacheBusting(t *testing.T) {
	t.Parallel()

	mainJS := readDesktopAssetText(t, "js/config/main.js")
	pageHTML := readDesktopAssetText(t, "config.html")
	if !strings.Contains(pageHTML, `window.AURAGO_BUILD_VERSION = "{{.BuildVersion}}"`) {
		t.Fatal("config.html must expose BuildVersion for lazy config modules")
	}
	if !strings.Contains(mainJS, "window.AURAGO_BUILD_VERSION") {
		t.Fatal("config main JS must use BuildVersion for lazy config module cache busting")
	}
	if strings.Contains(pageHTML, `/cfg/form-builder.js?v=21`) ||
		strings.Contains(pageHTML, `/js/config/main.js?v=21`) ||
		strings.Contains(mainJS, `CONFIG_ASSET_VERSION = '21'`) {
		t.Fatal("config assets must not keep using the stale fixed v=21 cache key")
	}
}

func TestConfigComposioUsesCatalogMetadataFallbacks(t *testing.T) {
	t.Parallel()

	composioJS := readDesktopAssetText(t, "cfg/composio.js")
	for _, marker := range []string{
		"function composioToolkitDescription",
		"function composioToolDescription",
		"tk.meta && tk.meta.description",
		"tool.human_description",
		"tool.meta && tool.meta.description",
	} {
		if !strings.Contains(composioJS, marker) {
			t.Fatalf("composio config module missing metadata fallback marker %q", marker)
		}
	}
}

func TestConfigComposioPreviewRequestsPolicyPreviewAndSortsUsefulToolsFirst(t *testing.T) {
	t.Parallel()

	composioJS := readDesktopAssetText(t, "cfg/composio.js")
	for _, marker := range []string{
		"'&limit=25&preview=1'",
		"function composioToolSortScore",
		".sort((a, b) => composioToolSortScore(a) - composioToolSortScore(b))",
		"decision.allowed === true",
	} {
		if !strings.Contains(composioJS, marker) {
			t.Fatalf("composio config module missing preview marker %q", marker)
		}
	}
}

func TestConfigComposioConnectOpensPopupBeforeAwaitedFetch(t *testing.T) {
	t.Parallel()

	composioJS := readDesktopAssetText(t, "cfg/composio.js")
	for _, marker := range []string{
		"const popup = window.open('about:blank', '_blank');",
		"popup.location.href = url;",
		"if (popup && !popup.closed) popup.close();",
	} {
		if !strings.Contains(composioJS, marker) {
			t.Fatalf("composio connect flow missing popup marker %q", marker)
		}
	}
	if strings.Contains(composioJS, "window.open(url, '_blank', 'noopener');") {
		t.Fatal("composio connect flow must not open the final URL only after the awaited fetch")
	}
}

func TestConfigComposioConnectDoesNotDependOnPreloadedAuthConfigs(t *testing.T) {
	t.Parallel()

	composioJS := readDesktopAssetText(t, "cfg/composio.js")
	for _, marker := range []string{
		"function composioSetConnectStatus",
		`id="composio-connect-status"`,
		"const preferred = composioPreferredAuthConfig();",
		"if (preferred && preferred.id) body.auth_config_id = preferred.id;",
		"body: JSON.stringify(body)",
	} {
		if !strings.Contains(composioJS, marker) {
			t.Fatalf("composio connect flow missing marker %q", marker)
		}
	}
	if strings.Contains(composioJS, "if (!preferred || !preferred.id)") {
		t.Fatal("composio connect flow must let the backend create an auth config when none is preloaded")
	}
}

func TestConfigComposioConnectRequiresActivePersistedToolkit(t *testing.T) {
	t.Parallel()

	composioJS := readDesktopAssetText(t, "cfg/composio.js")
	for _, marker := range []string{
		"const connectDisabled = isSelected ? '' : ' disabled aria-disabled=\"true\"';",
		"' + connectDisabled + ' onclick=\"composioConnectToolkit('",
		"async function composioToggleToolkit",
		"await composioSaveSelection(false);",
		"const selected = composioSelectedMap();",
		"if (!selected[normalized.toLowerCase()])",
	} {
		if !strings.Contains(composioJS, marker) {
			t.Fatalf("composio connect gating missing marker %q", marker)
		}
	}
}

func TestConfigComposioHandlesCallbackMessage(t *testing.T) {
	t.Parallel()

	composioJS := readDesktopAssetText(t, "cfg/composio.js")
	for _, marker := range []string{
		"function composioHandleConnectMessage",
		"aurago:composio-connected",
		"window.addEventListener('message', composioHandleConnectMessage);",
		"composioLoadToolkitDetail(composioState.selectedSlug);",
	} {
		if !strings.Contains(composioJS, marker) {
			t.Fatalf("composio callback message handling missing marker %q", marker)
		}
	}
}

func TestConfigToastsRenderAboveComposioModal(t *testing.T) {
	t.Parallel()

	sharedCSS := strings.ReplaceAll(readDesktopAssetText(t, "shared-components.css"), "\r\n", "\n")
	configCSS := strings.ReplaceAll(readDesktopAssetText(t, "css/config.css"), "\r\n", "\n")
	toastZ := cssZIndex(t, cssBlock(t, sharedCSS, ".toast {"))
	modalZ := cssZIndex(t, cssBlock(t, configCSS, ".cmp-modal-overlay {"))
	if toastZ <= modalZ {
		t.Fatalf("toast z-index = %d, composio modal z-index = %d; toast must render above modal", toastZ, modalZ)
	}
}

func cssBlock(t *testing.T, css, selector string) string {
	t.Helper()
	start := strings.Index(css, selector)
	if start < 0 {
		t.Fatalf("missing CSS selector %q", selector)
	}
	end := strings.Index(css[start:], "}")
	if end < 0 {
		t.Fatalf("missing closing brace for selector %q", selector)
	}
	return css[start : start+end+1]
}

func cssZIndex(t *testing.T, block string) int {
	t.Helper()
	const key = "z-index:"
	idx := strings.Index(block, key)
	if idx < 0 {
		t.Fatalf("missing z-index in CSS block: %s", block)
	}
	rest := strings.TrimSpace(block[idx+len(key):])
	end := strings.Index(rest, ";")
	if end >= 0 {
		rest = rest[:end]
	}
	rest = strings.TrimSpace(rest)
	// Handle CSS variable references: var(--name, fallback)
	if strings.HasPrefix(rest, "var(") {
		comma := strings.LastIndex(rest, ",")
		if comma >= 0 {
			rest = strings.TrimSpace(rest[comma+1 : len(rest)-1])
		}
	}
	value, err := strconv.Atoi(strings.TrimSpace(rest))
	if err != nil {
		t.Fatalf("parse z-index %q: %v", rest, err)
	}
	return value
}

func readComposioLangMap(t *testing.T, lang string) map[string]string {
	t.Helper()
	var values map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/config/composio/"+lang+".json")), &values); err != nil {
		t.Fatalf("parse composio %s translations: %v", lang, err)
	}
	return values
}
