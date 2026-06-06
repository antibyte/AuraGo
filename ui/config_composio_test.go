package ui

import (
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
		"'&limit=100&preview=1'",
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
