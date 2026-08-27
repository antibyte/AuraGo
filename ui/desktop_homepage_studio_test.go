package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDesktopHomepageStudioRouting(t *testing.T) {
	t.Parallel()

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, want := range []string{
		"appId === 'homepage-studio'",
		"loadAppScript('homepage-studio')",
		"window.HomepageStudioApp.render",
		"readonly: desktopReadonly()",
		"confirmDialog",
	} {
		if !strings.Contains(routing, want) {
			t.Fatalf("menus-and-routing missing homepage studio marker %q", want)
		}
	}

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	if !strings.Contains(foundation, "'homepage-studio': 'HomepageStudioApp'") {
		t.Fatalf("desktop-foundation missing homepage-studio dispose mapping")
	}
}

func TestHomepageStudioRedesignMarkers(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-homepage-studio.css")
	for _, want := range []string{
		"--hp-accent:",
		"--hp-glass:",
		"focus-visible",
		"vd-hp-preview-skeleton",
		"text-wrap: balance",
		"prefers-reduced-motion",
		"vd-hp-splitter",
		"vd-hp-viewport-toolbar",
		"vd-hp-device-seg",
		"vd-hp-drift",
		"vd-hp-site",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("homepage studio css missing redesign marker %q", want)
		}
	}

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"<aside class=\"vd-hp-chat\"",
		"<main class=\"vd-hp-preview-zone\"",
		"vd-hp-preview-skeleton",
		"preview_empty_title",
		"externalBtn.disabled",
		"data-hp-splitter=\"chat\"",
		"data-hp-splitter=\"inspector\"",
		"vd-hp-status-pill",
		"data-hp-device=\"desktop\"",
		"data-hp-fullscreen",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio js missing redesign marker %q", want)
		}
	}
}

func TestHomepageStudioUsesStatusPreviewURL(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")

	for _, want := range []string{
		"function homepageStatusPreviewURL(data, target)",
		"state.previewUrl = homepageStatusPreviewURL(data, state.target);",
		"id=\"hp-url-${windowId}\"",
		"class=\"vd-hp-preview-url\"",
		"previewPanel.insertBefore(iframe, previewLoading);",
		"case 'vercel':",
		"case 'netlify':",
		"case 'here_now':",
		"case 'remote':",
		"data.vercel_url",
		"data.netlify_url",
		"data.here_now_url",
		"data.remote_url",
		"data.preview_url",
		"data.web_container.browser_url",
		"data.python_server.browser_url",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio missing preview URL marker %q", want)
		}
	}

	for _, unwanted := range []string{
		"state.previewUrl = 'http://localhost:' + port;",
		"const port = 8080;",
		"previewBody.insertBefore(iframe, previewLoading);",
		"state.previewUrl = homepageStatusPreviewURL(data);",
	} {
		if strings.Contains(source, unwanted) {
			t.Fatalf("homepage studio still contains hard-coded local URL marker %q", unwanted)
		}
	}
}

func TestHomepageStudioUsesExternalHomepageTargets(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"deploymentTargets: []",
		"function loadHomepageTargets()",
		"function collectHomepageTargetsFromSite(",
		"function homepageExternalTargetURL(",
		"/api/homepage/sites",
		"/api/integrations/webhosts",
		"deploy_targets",
		"last_deploy_url",
		"provider_target_id",
		"remote_path",
		"homepageExternalTargetURL(target, state.deploymentTargets)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio external target support missing marker %q", want)
		}
	}
	if strings.Contains(source, "sites.slice(0,") {
		t.Fatal("homepage studio must not cap managed site detail loading before resolving external targets")
	}
}

func TestHomepageStudioUsesLedgerDeploymentsAndObservationsForExternalTargets(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"site.deployments",
		"site.remote_observations",
		"deployment.provider",
		"deployment.url",
		"observation.provider",
		"observation.url",
		"provider_deploy_id",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio external target fallback missing marker %q", want)
		}
	}
}

func TestHomepageStudioCloudTargetsFallbackToKnownExternalURL(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"const externalTargets = ['remote', 'vercel', 'netlify', 'here_now'];",
		"externalTargets.includes(selected)",
		"item.provider !== 'local'",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio cloud target fallback missing marker %q", want)
		}
	}
}

func TestHomepageStudioPreviewSandboxKeepsOpaqueOrigin(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"iframe.sandbox = 'allow-scripts allow-forms';",
		"iframe.referrerPolicy = 'no-referrer';",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio preview iframe missing sandbox marker %q", want)
		}
	}
	for _, unwanted := range []string{
		"allow-same-origin",
		"allow-popups",
	} {
		if strings.Contains(source, unwanted) {
			t.Fatalf("homepage studio preview iframe must not use sandbox flag %q", unwanted)
		}
	}
}

func TestHomepageStudioChatPayloadCarriesHomepageScope(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"function homepageWindowContext()",
		"window_context: homepageWindowContext()",
		"homepage_mode: true",
		"Use homepage_project, homepage_file, homepage_quality, homepage_deploy, and homepage_git.",
		"Do not use virtual_desktop apps, widgets, or files for Homepage Studio site changes.",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio chat payload missing scope marker %q", want)
		}
	}
}

func TestHomepageStudioHistoryPanelMarkers(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-homepage-studio.css")
	for _, want := range []string{
		".vd-hp-preview-tabs",
		".vd-hp-history-panel",
		".vd-hp-history-controls",
		".vd-hp-history-entry",
		".vd-hp-history-type-decision",
		".vd-hp-history-delete",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("homepage studio css missing history marker %q", want)
		}
	}

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"hp-tab-history-",
		"vd-hp-history-controls",
		"vd-hp-history-search",
		"vd-hp-history-list",
		"history_tab",
		"history_search_placeholder",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio js missing history shell marker %q", want)
		}
	}

	module := readDesktopAssetText(t, "js/desktop/apps/homepage-studio-history.js")
	for _, want := range []string{
		"function loadHistory",
		"function renderHistory",
		"/api/homepage/history",
		"history_delete_confirm",
		"homepage_studio.",
		"history_type_",
		"history_load_more",
		"confirmDialog",
		"params.set('offset'",
		"params.set('project_id'",
	} {
		if !strings.Contains(module, want) {
			t.Fatalf("homepage studio history module missing marker %q", want)
		}
	}

	var en map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/en.json")), &en); err != nil {
		t.Fatalf("parse English desktop translations: %v", err)
	}
	for _, key := range []string{
		"homepage_studio.history_tab",
		"homepage_studio.history_search_placeholder",
		"homepage_studio.history_filter_label",
		"homepage_studio.history_filter_all",
		"homepage_studio.history_loading",
		"homepage_studio.history_empty",
		"homepage_studio.history_error",
		"homepage_studio.history_delete",
		"homepage_studio.history_delete_confirm",
		"homepage_studio.history_type_decision",
		"homepage_studio.history_load_more",
	} {
		if en[key] == "" {
			t.Fatalf("English desktop translation missing key %q", key)
		}
	}
}

func TestHomepageStudioGermanUsesInformalAddress(t *testing.T) {
	t.Parallel()

	var values map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/de.json")), &values); err != nil {
		t.Fatalf("parse German desktop translations: %v", err)
	}
	for key, want := range map[string]string{
		"homepage_studio.chat_placeholder":    "Beschreibe deine Website-Änderungen...",
		"homepage_studio.preview_unavailable": "Vorschau nicht verfügbar — starte zuerst den Homepage-Container",
		"homepage_studio.preview_empty_title": "Noch keine Live-Vorschau",
		"homepage_studio.welcome":             "Willkommen im Homepage-Studio! Beschreibe die Website, die du erstellen möchtest, und ich erstelle sie für dich.",
	} {
		if values[key] != want {
			t.Fatalf("%s = %q, want %q", key, values[key], want)
		}
	}
}

func TestHomepageStudioModuleSplitAndLoadOrder(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	scripts := []string{
		"/js/desktop/apps/homepage-studio-preview.js",
		"/js/desktop/apps/homepage-studio-sites.js",
		"/js/desktop/apps/homepage-studio-history.js",
		"/js/desktop/apps/homepage-studio.js",
	}
	lastIndex := -1
	for _, script := range scripts {
		idx := strings.Index(loader, "'"+script+"'")
		if idx < 0 {
			t.Fatalf("module-loader missing homepage-studio script %q", script)
		}
		if idx < lastIndex {
			t.Fatalf("module-loader loads %q out of order (helpers must load before the main module)", script)
		}
		lastIndex = idx
	}

	for global, file := range map[string]string{
		"window.HomepageStudioPreview": "js/desktop/apps/homepage-studio-preview.js",
		"window.HomepageStudioSites":   "js/desktop/apps/homepage-studio-sites.js",
		"window.HomepageStudioHistory": "js/desktop/apps/homepage-studio-history.js",
		"window.HomepageStudioApp":     "js/desktop/apps/homepage-studio.js",
	} {
		source := readDesktopAssetText(t, file)
		if !strings.Contains(source, global) {
			t.Fatalf("%s does not expose %s", file, global)
		}
	}
}

func TestHomepageStudioPreviewModuleMarkers(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio-preview.js")
	for _, want := range []string{
		"window.HomepageStudioPreview = { create",
		"data-hp-device",
		"data-hp-fullscreen",
		"requestFullscreen",
		"fullscreenchange",
		"dispose",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio preview module missing marker %q", want)
		}
	}
}

func TestHomepageStudioSitesPanelMarkers(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio-sites.js")
	for _, want := range []string{
		"window.HomepageStudioSites = { create }",
		"/api/homepage/sites",
		"/reconcile",
		"drift_status",
		"deploy_targets",
		"remote_observations",
		"last_deployed_at",
		"vd-hp-drift-",
		"noopener noreferrer",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio sites module missing marker %q", want)
		}
	}

	main := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"window.HomepageStudioSites.create",
		"window.HomepageStudioHistory.create",
		"window.HomepageStudioPreview.create",
		"onSiteSelected",
	} {
		if !strings.Contains(main, want) {
			t.Fatalf("homepage studio main module missing module wiring marker %q", want)
		}
	}
}

func TestHomepageStudioHasNoNativeDialogs(t *testing.T) {
	t.Parallel()

	files := []string{
		"js/desktop/apps/homepage-studio.js",
		"js/desktop/apps/homepage-studio-preview.js",
		"js/desktop/apps/homepage-studio-sites.js",
		"js/desktop/apps/homepage-studio-history.js",
	}
	for _, file := range files {
		source := readDesktopAssetText(t, file)
		for _, unwanted := range []string{"confirm(", "alert(", "prompt("} {
			if strings.Contains(source, unwanted) {
				t.Fatalf("%s must use shell dialogs instead of native %q", file, unwanted)
			}
		}
	}
}

func TestHomepageStudioPersistsWorkbenchDraft(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"aurago.desktop.homepage.draft.",
		"persistHomepageDraft",
		"chatCollapsed",
		"inspectorCollapsed",
		"inspectorTab",
		"selectedSiteId",
		"historyQuery",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio persistence missing marker %q", want)
		}
	}
}

func TestHomepageStudioWindowPreset(t *testing.T) {
	t.Parallel()

	shell := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, want := range []string{
		"'homepage-studio': { width: 1240, height: 760 }",
		"'homepage-studio': true",
	} {
		if !strings.Contains(shell, want) {
			t.Fatalf("window-shell-runtime missing homepage studio preset marker %q", want)
		}
	}
}

func TestHomepageStudioNewTranslations(t *testing.T) {
	t.Parallel()

	newKeys := []string{
		"homepage_studio.brand_subtitle",
		"homepage_studio.assistant_toggle",
		"homepage_studio.inspector_toggle",
		"homepage_studio.suggest_landing",
		"homepage_studio.device_desktop",
		"homepage_studio.device_tablet",
		"homepage_studio.device_mobile",
		"homepage_studio.fullscreen",
		"homepage_studio.agent_working",
		"homepage_studio.sites_tab",
		"homepage_studio.sites_reconcile",
		"homepage_studio.sites_drift_clean",
		"homepage_studio.sites_drift_not_deployed",
		"homepage_studio.readonly_hint",
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			var values map[string]string
			if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+lang+".json")), &values); err != nil {
				t.Fatalf("parse %s desktop translations: %v", lang, err)
			}
			for _, key := range newKeys {
				if values[key] == "" {
					t.Fatalf("%s desktop translation missing new key %q", lang, key)
				}
			}
		})
	}
}
