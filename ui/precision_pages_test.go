package ui

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestPrecisionWorkspaceFoundationComponentsAreScoped(t *testing.T) {
	t.Parallel()

	foundation := normalizeAssetText(mustReadUIFile(t, "css/precision-workspace.css"))
	for _, marker := range []string{
		`.pw-page {`,
		`font-family: 'Geist'`,
		`--pw-accent: #2dd4bf;`,
		`[data-theme="light"] .pw-page`,
		`.pw-page[data-density="compact"]`,
		`@media (prefers-reduced-motion: reduce)`,
	} {
		if !strings.Contains(foundation, marker) {
			t.Fatalf("precision-workspace.css missing foundation marker %q", marker)
		}
	}

	components := normalizeAssetText(mustReadUIFile(t, "css/precision-pages.css"))
	for _, marker := range []string{
		`.pw-page .pw-app-header`,
		`.pw-page .pw-page-frame`,
		`.pw-page .pw-page-heading`,
		`.pw-page .pw-toolbar`,
		`.pw-page .pw-tabs`,
		`.pw-page .pw-status-strip`,
		`.pw-page .pw-stat-strip`,
		`.pw-page .pw-panel`,
		`.pw-page .pw-card`,
		`.pw-page .pw-table`,
		`.pw-page .pw-form-control`,
		`.pw-page .pw-state-empty`,
		`.pw-page .pw-state-error`,
		`.pw-page .pw-state-loading`,
		`.pw-page .pw-modal`,
		`@media (min-width: 1200px)`,
		`@media (max-width: 899px)`,
		`@media (max-width: 639px)`,
	} {
		if !strings.Contains(components, marker) {
			t.Errorf("precision-pages.css missing component marker %q", marker)
		}
	}
	if strings.Contains(strings.ToLower(components), "gradient(") {
		t.Fatal("precision-pages.css must not introduce gradients")
	}
	assertPrecisionCSSScoped(t, foundation)
	assertPrecisionCSSScoped(t, components)
}

func TestPrecisionWorkspaceClientContract(t *testing.T) {
	t.Parallel()

	client := normalizeAssetText(mustReadUIFile(t, "js/precision/workspace.js"))
	for _, marker := range []string{
		`(function () {`,
		`window.AuraPrecisionWorkspace`,
		`init: init`,
		`getDensity: getDensity`,
		`setDensity: setDensity`,
		`aurago.workspace.density.v1`,
		`aurago.config.density.v1`,
		`comfortable`,
		`compact`,
		`localStorage.getItem`,
		`localStorage.setItem`,
		`try {`,
		`catch`,
		`document.body.dataset.density`,
		`[data-pw-density-toggle]`,
		`common.workspace_density_toggle`,
		`common.workspace_density_comfortable`,
		`common.workspace_density_compact`,
		`aurago:workspace-density-change`,
		`MutationObserver`,
		`radialMenuAnchor`,
		`aria-current`,
		`'/missions/v2': '/missions'`,
		`'/gallery': '/media'`,
		`<svg`,
	} {
		if !strings.Contains(client, marker) {
			t.Errorf("workspace client missing contract marker %q", marker)
		}
	}
}

func TestConfigPrecisionWorkspaceSharedIntegration(t *testing.T) {
	t.Parallel()

	html := normalizeAssetText(mustReadUIFile(t, "config.html"))
	for _, marker := range []string{
		`<body class="pw-page" data-workspace-page="config"`,
		`data-pw-density-toggle`,
		`common.workspace_density_toggle`,
		`common.workspace_density_comfortable`,
	} {
		if !strings.Contains(html, marker) {
			t.Errorf("config.html missing shared workspace marker %q", marker)
		}
	}

	foundationAt := strings.Index(html, `/css/precision-workspace.css?v={{.BuildVersion}}`)
	componentsAt := strings.Index(html, `/css/precision-pages.css?v={{.BuildVersion}}`)
	configAt := strings.Index(html, `/css/config-workspace.css?v={{.BuildVersion}}`)
	if foundationAt < 0 || componentsAt < 0 || configAt < 0 || !(foundationAt < componentsAt && componentsAt < configAt) {
		t.Errorf("Config Precision CSS order = foundation:%d components:%d config:%d", foundationAt, componentsAt, configAt)
	}

	workspaceAt := strings.Index(html, `/js/precision/workspace.js?v={{.BuildVersion}}`)
	mainAt := strings.Index(html, `/js/config/main.js?v={{.BuildVersion}}`)
	if workspaceAt < 0 || mainAt < 0 || workspaceAt >= mainAt {
		t.Errorf("Config script order = workspace:%d main:%d", workspaceAt, mainAt)
	}

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	start := strings.Index(mainJS, "function applyConfigDensity(")
	end := strings.Index(mainJS, "function hasVisibleSection(")
	if start < 0 || end <= start {
		t.Fatal("cannot locate the applyConfigDensity integration block")
	}
	densityBlock := mainJS[start:end]
	for _, marker := range []string{`window.AuraPrecisionWorkspace`, `.init()`, `.getDensity()`, `.setDensity(`} {
		if !strings.Contains(densityBlock, marker) {
			t.Errorf("applyConfigDensity block missing workspace delegation marker %q", marker)
		}
	}
	if strings.Contains(densityBlock, "localStorage") {
		t.Fatal("Config density integration must delegate storage ownership to AuraPrecisionWorkspace")
	}
}

func TestPrecisionWorkspaceDashboardIntegration(t *testing.T) {
	t.Parallel()

	html := normalizeAssetText(mustReadUIFile(t, "dashboard.html"))
	for _, marker := range []string{
		`<body class="pw-page pw-operational-page" data-workspace-page="dashboard" data-density="comfortable">`,
		`href="#tab-overview"`,
		`/css/dashboard.css?v={{.BuildVersion}}-dashboard-agent-grid`,
		`data-tab="overview" id="dash-tab-overview"`,
		`data-tab="agent" id="dash-tab-agent"`,
		`id="tab-overview" role="tabpanel"`,
		`id="tab-agent" role="tabpanel"`,
		`id="card-agent-banner"`,
		`id="agent-banner"`,
		`id="card-system"`,
		`data-card="card-system"`,
		`id="cpu-chart"`,
		`id="card-personality"`,
		`id="card-knowledge-graph-visual"`,
		`id="knowledge-graph-visual" class="knowledge-visual-wrap"`,
		`id="log-viewer"`,
	} {
		if !strings.Contains(html, marker) {
			t.Errorf("dashboard.html missing Precision or preserved hook marker %q", marker)
		}
	}

	dashboardAt := strings.Index(html, `/css/dashboard.css?v={{.BuildVersion}}-dashboard-agent-grid`)
	enhancementsAt := strings.Index(html, `/css/enhancements.css?v=20260425a`)
	foundationAt := strings.Index(html, `/css/precision-workspace.css?v={{.BuildVersion}}`)
	componentsAt := strings.Index(html, `/css/precision-pages.css?v={{.BuildVersion}}`)
	if dashboardAt < 0 || enhancementsAt < 0 || foundationAt < 0 || componentsAt < 0 ||
		!(dashboardAt < enhancementsAt && enhancementsAt < foundationAt && foundationAt < componentsAt) {
		t.Errorf("Dashboard Precision CSS order = dashboard:%d enhancements:%d foundation:%d components:%d", dashboardAt, enhancementsAt, foundationAt, componentsAt)
	}

	workspaceAt := strings.Index(html, `/js/precision/workspace.js?v={{.BuildVersion}}`)
	mainAt := strings.Index(html, `/js/dashboard/main.js`)
	if workspaceAt < 0 || mainAt < 0 || workspaceAt >= mainAt {
		t.Errorf("Dashboard script order = workspace:%d main:%d", workspaceAt, mainAt)
	}
	if regexp.MustCompile(`(?i)\sstyle\s*=`).MatchString(html) {
		t.Fatal("dashboard.html must not add inline style attributes during Precision migration")
	}
}

func TestPrecisionWorkspaceDashboardAdapterIsScopedAndResponsive(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/dashboard.css"))
	const (
		adapterStart = `/* === Precision Workspace Dashboard Adapter: start === */`
		adapterEnd   = `/* === Precision Workspace Dashboard Adapter: end === */`
		prefix       = `.pw-page[data-workspace-page="dashboard"]`
	)
	start := strings.Index(css, adapterStart)
	end := strings.Index(css, adapterEnd)
	if start < 0 || end <= start {
		t.Fatalf("dashboard.css missing delimited Precision adapter: start=%d end=%d", start, end)
	}
	adapter := css[start:end]

	for _, marker := range []string{
		prefix + ` {`,
		`--pw-dashboard-frame: 1440px;`,
		`overflow-x: clip;`,
		`var(--pw-line)`,
		`border-radius: 14px;`,
		`border-radius: 20px;`,
		`background-image: none;`,
		`box-shadow: none;`,
		`content: none;`,
		prefix + `[data-density="compact"]`,
		`:root[data-theme="light"] *`,
		`@media (max-width: 1024px)`,
		`@media (max-width: 640px)`,
		`min-height: 44px;`,
		`@media (prefers-reduced-motion: reduce)`,
		prefix + ` #tab-agent .dash-grid`,
		`grid-template-columns: repeat(2, minmax(0, 1fr));`,
	} {
		if !strings.Contains(adapter, marker) {
			t.Errorf("Dashboard Precision adapter missing marker %q", marker)
		}
	}
	if strings.Contains(strings.ToLower(adapter), "gradient(") {
		t.Fatal("Dashboard Precision adapter must not introduce visible gradients")
	}

	comments := regexp.MustCompile(`(?s)/\*.*?\*/`)
	uncommented := comments.ReplaceAllString(adapter, "")
	segmentStart := 0
	for index, char := range uncommented {
		switch char {
		case '{':
			header := strings.TrimSpace(uncommented[segmentStart:index])
			segmentStart = index + 1
			if header == "" || strings.HasPrefix(header, "@") {
				continue
			}
			for _, selector := range strings.Split(header, ",") {
				selector = strings.TrimSpace(selector)
				if selector != "" && !strings.HasPrefix(selector, prefix) {
					t.Errorf("Dashboard Precision adapter selector must start with %q: %q", prefix, selector)
				}
			}
		case '}':
			segmentStart = index + 1
		}
	}

	for _, preserved := range []string{
		`.chart-wrap-sm`,
		`.log-viewer`,
		`.knowledge-visual-wrap`,
		`min-height: 360px;`,
	} {
		if !strings.Contains(css, preserved) {
			t.Errorf("dashboard.css lost chart/log/KG layout contract %q", preserved)
		}
	}
}

func TestPrecisionWorkspaceDashboardCompactMobileControlsStayTouchSized(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/dashboard.css"))
	const (
		adapterStart = `/* === Precision Workspace Dashboard Adapter: start === */`
		adapterEnd   = `/* === Precision Workspace Dashboard Adapter: end === */`
		mobileStart  = `@media (max-width: 640px)`
		mobileEnd    = `@media (prefers-reduced-motion: reduce)`
		prefix       = `.pw-page[data-workspace-page="dashboard"][data-density="compact"]`
	)
	start := strings.Index(css, adapterStart)
	end := strings.Index(css, adapterEnd)
	if start < 0 || end <= start {
		t.Fatalf("dashboard.css missing delimited Precision adapter: start=%d end=%d", start, end)
	}
	adapter := css[start:end]
	mobileAt := strings.Index(adapter, mobileStart)
	reducedMotionAt := strings.Index(adapter, mobileEnd)
	if mobileAt < 0 || reducedMotionAt <= mobileAt {
		t.Fatalf("Dashboard Precision adapter missing ordered mobile block: mobile=%d reduced-motion=%d", mobileAt, reducedMotionAt)
	}
	mobile := adapter[mobileAt:reducedMotionAt]

	for _, marker := range []string{
		prefix + ` .dash-tab {`,
		prefix + ` input,`,
		prefix + ` select,`,
		prefix + ` textarea,`,
		prefix + ` button {`,
	} {
		if !strings.Contains(mobile, marker) {
			t.Errorf("Dashboard compact mobile block missing touch-target selector %q", marker)
		}
	}
	if matches := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(prefix+` .dash-tab`) + `\s*\{[^}]*min-height:\s*44px;`).FindString(mobile); matches == "" {
		t.Error("Dashboard compact mobile tab rule must explicitly restore min-height: 44px")
	}
	controlsAt := strings.Index(mobile, prefix+` input,`)
	if controlsAt < 0 {
		t.Fatal("Dashboard compact mobile controls rule not found")
	}
	controlsEnd := strings.Index(mobile[controlsAt:], "}")
	if controlsEnd < 0 || !strings.Contains(mobile[controlsAt:controlsAt+controlsEnd], `min-height: 44px;`) {
		t.Error("Dashboard compact mobile controls rule must explicitly restore min-height: 44px")
	}
}

func TestPrecisionWorkspaceDashboardAdapterNeutralizesResidualGlows(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/dashboard.css"))
	const (
		adapterStart = `/* === Precision Workspace Dashboard Adapter: start === */`
		adapterEnd   = `/* === Precision Workspace Dashboard Adapter: end === */`
		prefix       = `.pw-page[data-workspace-page="dashboard"]`
	)
	start := strings.Index(css, adapterStart)
	end := strings.Index(css, adapterEnd)
	if start < 0 || end <= start {
		t.Fatalf("dashboard.css missing delimited Precision adapter: start=%d end=%d", start, end)
	}
	adapter := css[start:end]
	ruleAt := strings.Index(adapter, prefix+` .dash-card:hover canvas,`)
	if ruleAt < 0 {
		t.Fatal("Dashboard Precision adapter missing residual-glow suppression rule")
	}
	ruleEnd := strings.Index(adapter[ruleAt:], "}")
	if ruleEnd < 0 {
		t.Fatal("Dashboard residual-glow suppression rule is not closed")
	}
	rule := adapter[ruleAt : ruleAt+ruleEnd]
	for _, marker := range []string{
		prefix + ` .dash-card:hover canvas`,
		prefix + ` .conf-3`,
		prefix + ` .pill-completed`,
		prefix + ` .gh-badge-tracked`,
		`filter: none;`,
		`box-shadow: none;`,
	} {
		if !strings.Contains(rule, marker) {
			t.Errorf("Dashboard residual-glow suppression rule missing %q", marker)
		}
	}
}

func TestPrecisionWorkspaceDashboardAdapterStopsStatusPulseGlows(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/dashboard.css"))
	const (
		adapterStart = `/* === Precision Workspace Dashboard Adapter: start === */`
		adapterEnd   = `/* === Precision Workspace Dashboard Adapter: end === */`
		prefix       = `.pw-page[data-workspace-page="dashboard"]`
	)
	start := strings.Index(css, adapterStart)
	end := strings.Index(css, adapterEnd)
	if start < 0 || end <= start {
		t.Fatalf("dashboard.css missing delimited Precision adapter: start=%d end=%d", start, end)
	}
	adapter := css[start:end]
	pulseSuppression := regexp.MustCompile(
		`(?s)` +
			regexp.QuoteMeta(prefix+` .pill-running`) + `\s*,\s*` +
			regexp.QuoteMeta(prefix+` .status-dot.green`) +
			`\s*\{[^}]*animation:\s*none;`,
	)
	if !pulseSuppression.MatchString(adapter) {
		t.Error("Dashboard Precision adapter must disable pulse-glow animation for running pills and green status dots")
	}
}

func TestPrecisionWorkspacePlansMissionsCheatsheetsIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		template         string
		page             string
		pageStylesheet   string
		pageScript       string
		inlineStyleCount int
		hooks            []string
	}{
		{
			name:             "Plans",
			template:         "plans.html",
			page:             "plans",
			pageStylesheet:   `/css/plans.css`,
			pageScript:       `/js/plans/main.js`,
			inlineStyleCount: 0,
			hooks: []string{
				`id="status-filter"`,
				`id="include-archived"`,
				`id="refresh-btn"`,
				`id="plan-list"`,
				`id="plan-detail"`,
				`id="blocker-modal"`,
				`id="split-modal"`,
			},
		},
		{
			name:             "Missions",
			template:         "missions_v2.html",
			page:             "missions",
			pageStylesheet:   `/css/missions.css`,
			pageScript:       `/js/missions/main.js`,
			inlineStyleCount: 0,
			hooks: []string{
				`id="view-toggle"`,
				`data-view-mode="grid"`,
				`data-view-mode="list"`,
				`id="queue-section"`,
				`data-filter="scheduled"`,
				`id="missions-grid"`,
				`id="mission-form"`,
				`id="prep-modal"`,
			},
		},
		{
			name:             "Cheatsheets",
			template:         "cheatsheets.html",
			page:             "cheatsheets",
			pageStylesheet:   `/css/cheatsheets.css`,
			pageScript:       `/js/cheatsheets/main.js`,
			inlineStyleCount: 0,
			hooks: []string{
				`id="view-toggle"`,
				`onclick="setViewMode('grid')"`,
				`id="tab-user"`,
				`id="tab-agent"`,
				`id="panel-user"`,
				`id="panel-agent"`,
				`id="sheet-content"`,
				`id="attachments-list"`,
				`id="knowledge-picker-modal"`,
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			html := normalizeAssetText(mustReadUIFile(t, test.template))
			bodyMarker := `<body class="pw-page pw-operational-page" data-workspace-page="` + test.page + `" data-density="comfortable">`
			if !strings.Contains(html, bodyMarker) {
				t.Errorf("%s missing exact Precision opt-in body marker %q", test.template, bodyMarker)
			}

			pageAt := strings.Index(html, test.pageStylesheet)
			enhancementsAt := strings.Index(html, `/css/enhancements.css`)
			foundationAt := strings.Index(html, `/css/precision-workspace.css?v={{.BuildVersion}}`)
			componentsAt := strings.Index(html, `/css/precision-pages.css?v={{.BuildVersion}}`)
			if pageAt < 0 || foundationAt < 0 || componentsAt < 0 || !(pageAt < foundationAt && foundationAt < componentsAt) {
				t.Errorf("%s Precision CSS order = page:%d enhancements:%d foundation:%d components:%d", test.template, pageAt, enhancementsAt, foundationAt, componentsAt)
			}
			if enhancementsAt >= 0 && enhancementsAt >= foundationAt {
				t.Errorf("%s enhancements stylesheet must load before Precision foundation: enhancements=%d foundation=%d", test.template, enhancementsAt, foundationAt)
			}

			workspaceAt := strings.Index(html, `/js/precision/workspace.js?v={{.BuildVersion}}`)
			mainAt := strings.Index(html, test.pageScript)
			if workspaceAt < 0 || mainAt < 0 || workspaceAt >= mainAt {
				t.Errorf("%s script order = workspace:%d main:%d", test.template, workspaceAt, mainAt)
			}

			for _, hook := range test.hooks {
				if !strings.Contains(html, hook) {
					t.Errorf("%s lost functional hook %q", test.template, hook)
				}
			}
			if got := len(regexp.MustCompile(`(?i)\sstyle\s*=`).FindAllString(html, -1)); got != test.inlineStyleCount {
				t.Errorf("%s inline style count = %d, want preserved baseline count %d", test.template, got, test.inlineStyleCount)
			}
		})
	}
}

func TestPrecisionWorkspacePlanningPagesUseSemanticClassesInsteadOfInlineStyles(t *testing.T) {
	t.Parallel()

	for _, template := range []string{"plans.html", "missions_v2.html", "cheatsheets.html"} {
		html := normalizeAssetText(mustReadUIFile(t, template))
		if regexp.MustCompile(`(?i)\sstyle\s*=`).MatchString(html) {
			t.Errorf("%s must not contain inline style attributes", template)
		}
	}

	missions := normalizeAssetText(mustReadUIFile(t, "missions_v2.html"))
	if !strings.Contains(missions, `<div class="modal modal-prep-context">`) {
		t.Error("missions_v2.html prepared-context dialog must use the semantic modal-prep-context class")
	}

	cheatsheets := normalizeAssetText(mustReadUIFile(t, "cheatsheets.html"))
	if !strings.Contains(cheatsheets, `id="attachment-file-input" accept=".txt,.md" class="is-hidden"`) {
		t.Error("cheatsheets.html file input must use the existing is-hidden utility class")
	}

	utilities := normalizeAssetText(mustReadUIFile(t, "shared-utilities.css"))
	isHidden := regexp.MustCompile(`(?s)\.is-hidden\s*\{([^}]*)\}`).FindStringSubmatch(utilities)
	if len(isHidden) != 2 || !strings.Contains(isHidden[1], "display: none;") || strings.Contains(isHidden[1], "!important") {
		t.Error("is-hidden must remain a non-important class rule so an inline display state can override it")
	}
}

func TestPrecisionWorkspacePlansMissionsCheatsheetsAdaptersAreScopedAndResponsive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		stylesheet    string
		page          string
		layoutMarkers []string
	}{
		{
			name:       "Plans",
			stylesheet: "css/plans.css",
			page:       "plans",
			layoutMarkers: []string{
				`.plans-layout`,
				`.plan-list`,
				`.plan-detail`,
				`.plan-modal`,
				`overflow-wrap: anywhere;`,
			},
		},
		{
			name:       "Missions",
			stylesheet: "css/missions.css",
			page:       "missions",
			layoutMarkers: []string{
				`.status-bar`,
				`.queue-section`,
				`.missions-grid`,
				`.mc-log-body`,
				`.modal`,
			},
		},
		{
			name:       "Cheatsheets",
			stylesheet: "css/cheatsheets.css",
			page:       "cheatsheets",
			layoutMarkers: []string{
				`.cheatsheet-tabs`,
				`.cards-grid`,
				`.editor-tabs`,
				`.attachments-section`,
				`pre`,
				`overflow-wrap: anywhere;`,
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
			startMarker := `/* === Precision Workspace ` + test.name + ` Adapter: start === */`
			endMarker := `/* === Precision Workspace ` + test.name + ` Adapter: end === */`
			start := strings.Index(css, startMarker)
			end := strings.Index(css, endMarker)
			if start < 0 || end <= start {
				t.Fatalf("%s missing delimited Precision adapter: start=%d end=%d", test.stylesheet, start, end)
			}
			adapter := css[start:end]
			prefix := `.pw-page[data-workspace-page="` + test.page + `"]`

			for _, marker := range append([]string{
				prefix + ` {`,
				`overflow-x: clip;`,
				`background-image: none;`,
				`box-shadow: none;`,
				prefix + `[data-density="compact"]`,
				`:root[data-theme="light"] *`,
				`@media (max-width: 1024px)`,
				`@media (max-width: 640px)`,
				`min-height: 44px;`,
				`min-height: 100dvh;`,
				`max-height: calc(100dvh - 1rem);`,
				`border-radius: 20px 20px 0 0;`,
				`@media (prefers-reduced-motion: reduce)`,
			}, test.layoutMarkers...) {
				if !strings.Contains(adapter, marker) {
					t.Errorf("%s Precision adapter missing marker %q", test.name, marker)
				}
			}
			if strings.Contains(strings.ToLower(adapter), "gradient(") {
				t.Fatalf("%s Precision adapter must not introduce gradient expressions", test.name)
			}
			assertPrecisionAdapterSelectorsScoped(t, adapter, prefix)
		})
	}
}

func assertPrecisionAdapterSelectorsScoped(t *testing.T, adapter, prefix string) {
	t.Helper()

	comments := regexp.MustCompile(`(?s)/\*.*?\*/`)
	uncommented := comments.ReplaceAllString(adapter, "")
	segmentStart := 0
	for index, char := range uncommented {
		switch char {
		case '{':
			header := strings.TrimSpace(uncommented[segmentStart:index])
			segmentStart = index + 1
			if header == "" || strings.HasPrefix(header, "@") {
				continue
			}
			for _, selector := range strings.Split(header, ",") {
				selector = strings.TrimSpace(selector)
				if selector != "" && !strings.HasPrefix(selector, prefix) {
					t.Errorf("Precision adapter selector must start with %q: %q", prefix, selector)
				}
			}
		case '}':
			segmentStart = index + 1
		}
	}
}

func TestPrecisionWorkspaceMissionsAdapterStopsDecorativeStatusGlows(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/missions.css"))
	const (
		adapterStart = `/* === Precision Workspace Missions Adapter: start === */`
		adapterEnd   = `/* === Precision Workspace Missions Adapter: end === */`
		prefix       = `.pw-page[data-workspace-page="missions"]`
	)
	start := strings.Index(css, adapterStart)
	end := strings.Index(css, adapterEnd)
	if start < 0 || end <= start {
		t.Fatalf("missions.css missing delimited Precision adapter: start=%d end=%d", start, end)
	}
	adapter := css[start:end]
	pulseSuppression := regexp.MustCompile(
		`(?s)` +
			regexp.QuoteMeta(prefix+` .badge-prep-preparing`) + `\s*,\s*` +
			regexp.QuoteMeta(prefix+` .mc-status-chip--running`) +
			`\s*\{[^}]*animation:\s*none;[^}]*box-shadow:\s*none;`,
	)
	if !pulseSuppression.MatchString(adapter) {
		t.Error("Missions Precision adapter must disable preparation and running-chip pulse glows")
	}

	statusPulseSuppression := regexp.MustCompile(
		`(?s)` + regexp.QuoteMeta(prefix+` .status-card.running`) +
			`\s*\{[^}]*animation:\s*none;[^}]*box-shadow:\s*none;`,
	)
	if !statusPulseSuppression.MatchString(adapter) {
		t.Error("Missions Precision adapter must disable the running summary-card pulse while keeping its semantic styling")
	}

	preparedGlowSuppression := regexp.MustCompile(
		`(?s)` + regexp.QuoteMeta(prefix+` .badge-prep-prepared`) +
			`\s*\{[^}]*box-shadow:\s*none;`,
	)
	if !preparedGlowSuppression.MatchString(adapter) {
		t.Error("Missions Precision adapter must remove the prepared badge decorative glow")
	}
}

func TestPrecisionWorkspaceMissionsAdapterStylesActualCompactRendererContract(t *testing.T) {
	t.Parallel()

	client := normalizeAssetText(mustReadUIFile(t, "js/missions/main.js"))
	for _, rendererClass := range []string{
		`class="card-compact"`,
		`class="card-badges"`,
		`class="card-actions"`,
	} {
		if !strings.Contains(client, rendererClass) {
			t.Fatalf("missions renderer no longer emits expected compact-list hook %q", rendererClass)
		}
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/missions.css"))
	const (
		adapterStart = `/* === Precision Workspace Missions Adapter: start === */`
		adapterEnd   = `/* === Precision Workspace Missions Adapter: end === */`
		prefix       = `.pw-page[data-workspace-page="missions"]`
	)
	start := strings.Index(css, adapterStart)
	end := strings.Index(css, adapterEnd)
	if start < 0 || end <= start {
		t.Fatalf("missions.css missing delimited Precision adapter: start=%d end=%d", start, end)
	}
	adapter := css[start:end]

	for _, marker := range []string{
		prefix + ` .card-compact {`,
		`grid-template-columns: auto minmax(0, 1fr) auto auto;`,
		prefix + ` .card-badges {`,
		prefix + ` .card-actions {`,
		prefix + `[data-density="compact"] .card-compact`,
		`@media (max-width: 1024px)`,
		`@media (max-width: 640px)`,
		`grid-template-columns: auto minmax(0, 1fr);`,
		`grid-column: 1 / -1;`,
	} {
		if !strings.Contains(adapter, marker) {
			t.Errorf("Missions compact renderer adapter missing marker %q", marker)
		}
	}
}

func TestPrecisionWorkspaceMissionsCompactListActionsStayTouchSizedOnMobile(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/missions.css"))
	const (
		adapterStart = `/* === Precision Workspace Missions Adapter: start === */`
		adapterEnd   = `/* === Precision Workspace Missions Adapter: end === */`
		mobileStart  = `@media (max-width: 640px)`
		mobileEnd    = `@media (prefers-reduced-motion: reduce)`
		prefix       = `.pw-page[data-workspace-page="missions"][data-density="compact"] .card-actions .mc-btn`
	)
	start := strings.Index(css, adapterStart)
	end := strings.Index(css, adapterEnd)
	if start < 0 || end <= start {
		t.Fatalf("missions.css missing delimited Precision adapter: start=%d end=%d", start, end)
	}
	adapter := css[start:end]

	desktopCompact := regexp.MustCompile(
		`(?s)` + regexp.QuoteMeta(prefix) +
			`\s*\{[^}]*width:\s*36px;[^}]*height:\s*36px;[^}]*min-height:\s*36px;`,
	)
	if !desktopCompact.MatchString(adapter) {
		t.Error("Missions compact list actions must preserve the 36px desktop density contract")
	}

	mobileAt := strings.LastIndex(adapter, mobileStart)
	reducedMotionAt := strings.Index(adapter, mobileEnd)
	if mobileAt < 0 || reducedMotionAt <= mobileAt {
		t.Fatalf("Missions adapter missing ordered mobile block: mobile=%d reduced-motion=%d", mobileAt, reducedMotionAt)
	}
	mobile := adapter[mobileAt:reducedMotionAt]
	mobileTouchTarget := regexp.MustCompile(
		`(?s)` + regexp.QuoteMeta(prefix) +
			`\s*\{[^}]*width:\s*44px;[^}]*height:\s*44px;[^}]*min-width:\s*44px;[^}]*min-height:\s*44px;`,
	)
	if !mobileTouchTarget.MatchString(mobile) {
		t.Error("Missions compact list action buttons need an equal-specificity 44px mobile override")
	}
}

func TestPrecisionWorkspaceKnowledgeSkillsIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		template       string
		page           string
		pageStylesheet string
		pageScript     string
		hiddenHooks    []string
		hooks          []string
	}{
		{
			name:           "Knowledge",
			template:       "knowledge.html",
			page:           "knowledge",
			pageStylesheet: `/css/knowledge.css`,
			pageScript:     `/js/knowledge/main.js`,
			hiddenHooks: []string{
				`class="empty-state is-hidden" id="devices-empty"`,
				`id="file-preview-frame" class="kc-preview-frame is-hidden"`,
			},
			hooks: []string{
				`id="tab-files"`,
				`id="panel-files"`,
				`id="credentials-table"`,
				`id="file-preview-modal"`,
				`id="file-preview-text"`,
				`id="credential-modal"`,
			},
		},
		{
			name:           "Skills",
			template:       "skills.html",
			page:           "skills",
			pageStylesheet: `/css/skills.css`,
			pageScript:     `/js/skills/main.js`,
			hiddenHooks: []string{
				`class="sk-toolbar-actions is-hidden" id="agent-toolbar-actions"`,
				`id="agent-file-upload-input" class="is-hidden"`,
				`class="modal-overlay is-hidden" id="agent-resource-path-modal"`,
				`id="upload-file" accept=".py" class="is-hidden"`,
			},
			hooks: []string{
				`id="sk-tab-python"`,
				`id="sk-tab-agent"`,
				`id="sk-grid"`,
				`id="agent-resource-browser"`,
				`id="code-editor-container"`,
				`id="upload-modal"`,
				`id="agent-skill-modal"`,
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			html := normalizeAssetText(mustReadUIFile(t, test.template))
			bodyMarker := `<body class="pw-page pw-operational-page" data-workspace-page="` + test.page + `" data-density="comfortable">`
			if !strings.Contains(html, bodyMarker) {
				t.Errorf("%s missing exact Precision opt-in body marker %q", test.template, bodyMarker)
			}

			pageAt := strings.Index(html, test.pageStylesheet)
			enhancementsAt := strings.Index(html, `/css/enhancements.css`)
			foundationAt := strings.Index(html, `/css/precision-workspace.css?v={{.BuildVersion}}`)
			componentsAt := strings.Index(html, `/css/precision-pages.css?v={{.BuildVersion}}`)
			if pageAt < 0 || foundationAt < 0 || componentsAt < 0 || !(pageAt < foundationAt && foundationAt < componentsAt) {
				t.Errorf("%s Precision CSS order = page:%d enhancements:%d foundation:%d components:%d", test.template, pageAt, enhancementsAt, foundationAt, componentsAt)
			}
			if enhancementsAt >= 0 && enhancementsAt >= foundationAt {
				t.Errorf("%s enhancements stylesheet must load before Precision foundation: enhancements=%d foundation=%d", test.template, enhancementsAt, foundationAt)
			}

			workspaceAt := strings.Index(html, `/js/precision/workspace.js?v={{.BuildVersion}}`)
			mainAt := strings.Index(html, test.pageScript)
			if workspaceAt < 0 || mainAt < 0 || workspaceAt >= mainAt {
				t.Errorf("%s script order = workspace:%d main:%d", test.template, workspaceAt, mainAt)
			}

			if regexp.MustCompile(`(?i)\sstyle\s*=`).MatchString(html) {
				t.Errorf("%s must not contain inline style attributes", test.template)
			}
			for _, hook := range append(test.hooks, test.hiddenHooks...) {
				if !strings.Contains(html, hook) {
					t.Errorf("%s lost or failed to preserve functional hook %q", test.template, hook)
				}
			}
		})
	}
}

func TestPrecisionWorkspaceKnowledgeSkillsAdaptersAreScopedAndResponsive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		stylesheet    string
		page          string
		layoutMarkers []string
	}{
		{
			name:       "Knowledge",
			stylesheet: "css/knowledge.css",
			page:       "knowledge",
			layoutMarkers: []string{
				`.kc-tabs`,
				`.kc-panel`,
				`.kc-table-wrap`,
				`.kc-preview-body`,
				`.modal`,
				`overflow-wrap: anywhere;`,
			},
		},
		{
			name:       "Skills",
			stylesheet: "css/skills.css",
			page:       "skills",
			layoutMarkers: []string{
				`.sk-tabs`,
				`.sk-toolbar`,
				`.sk-grid`,
				`.sk-code-editor-container`,
				`.sk-resource-list`,
				`.modal`,
				`overflow-wrap: anywhere;`,
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
			startMarker := `/* === Precision Workspace ` + test.name + ` Adapter: start === */`
			endMarker := `/* === Precision Workspace ` + test.name + ` Adapter: end === */`
			start := strings.Index(css, startMarker)
			end := strings.Index(css, endMarker)
			if start < 0 || end <= start {
				t.Fatalf("%s missing delimited Precision adapter: start=%d end=%d", test.stylesheet, start, end)
			}
			adapter := css[start:end]
			prefix := `.pw-page[data-workspace-page="` + test.page + `"]`

			for _, marker := range append([]string{
				prefix + ` {`,
				`overflow-x: clip;`,
				`background-image: none;`,
				`box-shadow: none;`,
				`filter: none;`,
				prefix + `[data-density="compact"]`,
				`:root[data-theme="light"] *`,
				`@media (max-width: 1024px)`,
				`@media (max-width: 640px)`,
				`min-height: 44px;`,
				`min-height: 100dvh;`,
				`max-height: calc(100dvh - 1rem);`,
				`border-radius: 20px 20px 0 0;`,
				`@media (prefers-reduced-motion: reduce)`,
			}, test.layoutMarkers...) {
				if !strings.Contains(adapter, marker) {
					t.Errorf("%s Precision adapter missing marker %q", test.name, marker)
				}
			}
			if strings.Contains(strings.ToLower(adapter), "gradient(") {
				t.Fatalf("%s Precision adapter must not introduce gradient expressions", test.name)
			}
			assertPrecisionAdapterSelectorsScoped(t, adapter, prefix)
		})
	}
}

func TestPrecisionWorkspaceKnowledgeSkillsAdaptersSuppressLegacyDecoration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		stylesheet string
		page       string
		selectors  []string
	}{
		{
			name:       "Knowledge",
			stylesheet: "css/knowledge.css",
			page:       "knowledge",
			selectors: []string{
				`.kc-skeleton-cell::after`,
				`.kc-tabs-wrap::before`,
				`.kc-tabs-wrap::after`,
				`.kc-preview-pdf-wrap canvas`,
				`.kc-preview-fallback`,
				`.kc-sync-card`,
				`.kc-picker-dropdown`,
				`.kc-todo-progress-fill`,
			},
		},
		{
			name:       "Skills",
			stylesheet: "css/skills.css",
			page:       "skills",
			selectors: []string{
				`.sk-daemon-settings-section`,
				`.sk-dropzone:hover`,
				`.empty-state .icon`,
				`.sk-toast`,
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
			start := strings.Index(css, `/* === Precision Workspace `+test.name+` Adapter: start === */`)
			end := strings.Index(css, `/* === Precision Workspace `+test.name+` Adapter: end === */`)
			if start < 0 || end <= start {
				t.Fatalf("%s missing delimited Precision adapter", test.stylesheet)
			}
			adapter := css[start:end]
			prefix := `.pw-page[data-workspace-page="` + test.page + `"]`
			for _, selector := range test.selectors {
				if !strings.Contains(adapter, prefix+` `+selector) {
					t.Errorf("%s adapter does not explicitly suppress legacy decoration for %s", test.name, selector)
				}
			}
		})
	}
}

func TestPrecisionWorkspaceKnowledgeSkillsHiddenStateRemainsInlineOverridable(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		stylesheet string
		page       string
	}{
		{stylesheet: "css/knowledge.css", page: "knowledge"},
		{stylesheet: "css/skills.css", page: "skills"},
	} {
		css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
		prefix := `.pw-page[data-workspace-page="` + test.page + `"]`
		hiddenRule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(prefix+` .is-hidden`) + `\s*\{([^}]*)\}`).FindStringSubmatch(css)
		if len(hiddenRule) != 2 || !strings.Contains(hiddenRule[1], `display: none;`) {
			t.Errorf("%s needs a page-scoped baseline is-hidden rule", test.stylesheet)
			continue
		}
		if strings.Contains(hiddenRule[1], `!important`) {
			t.Errorf("%s is-hidden rule must remain overridable by normal inline display values", test.stylesheet)
		}
	}
}

func TestPrecisionWorkspaceKnowledgeSkillsCompactMobileControlsWinCascade(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		stylesheet string
		page       string
		controls   []string
	}{
		{
			name:       "Knowledge",
			stylesheet: "css/knowledge.css",
			page:       "knowledge",
			controls:   []string{".kc-tab", ".kc-search", ".kc-filter-select", ".btn", ".kc-icon-btn"},
		},
		{
			name:       "Skills",
			stylesheet: "css/skills.css",
			page:       "skills",
			controls:   []string{".sk-tab", ".sk-search", ".sk-input", ".sk-select", ".btn"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
			start := strings.Index(css, `/* === Precision Workspace `+test.name+` Adapter: start === */`)
			end := strings.Index(css, `/* === Precision Workspace `+test.name+` Adapter: end === */`)
			if start < 0 || end <= start {
				t.Fatalf("%s missing delimited Precision adapter", test.stylesheet)
			}
			adapter := css[start:end]
			mobileAt := strings.LastIndex(adapter, `@media (max-width: 640px)`)
			reducedAt := strings.Index(adapter, `@media (prefers-reduced-motion: reduce)`)
			if mobileAt < 0 || reducedAt <= mobileAt {
				t.Fatalf("%s missing ordered mobile and reduced-motion blocks", test.stylesheet)
			}
			mobile := adapter[mobileAt:reducedAt]
			prefix := `.pw-page[data-workspace-page="` + test.page + `"][data-density="compact"] `
			for _, control := range test.controls {
				rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(prefix+control) + `[^{}]*\{[^}]*min-height:\s*44px;`)
				if !rule.MatchString(mobile) {
					t.Errorf("%s compact mobile %s needs an equal-or-higher-specificity 44px override", test.name, control)
				}
			}
		})
	}
}

func TestPrecisionWorkspaceSkillsFullscreenAndSemanticToasts(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/skills.css"))
	start := strings.Index(css, `/* === Precision Workspace Skills Adapter: start === */`)
	end := strings.Index(css, `/* === Precision Workspace Skills Adapter: end === */`)
	if start < 0 || end <= start {
		t.Fatal("skills.css missing delimited Precision adapter")
	}
	adapter := css[start:end]
	prefix := `.pw-page[data-workspace-page="skills"]`

	for _, marker := range []string{
		prefix + ` .modal-overlay.sk-code-overlay-fullscreen .modal {`,
		`width: 100vw;`,
		`height: 100dvh;`,
		`max-height: none;`,
		`border-radius: 0;`,
		prefix + ` .modal-overlay.sk-code-overlay-fullscreen .modal-body {`,
		prefix + ` .modal-overlay.sk-code-overlay-fullscreen .sk-code-editor-container`,
	} {
		if !strings.Contains(adapter, marker) {
			t.Errorf("Skills fullscreen contract missing %q", marker)
		}
	}

	toastColors := map[string]string{
		`.sk-toast-success`: `var(--pw-success)`,
		`.sk-toast-error`:   `var(--pw-danger)`,
		`.sk-toast-info`:    `var(--pw-accent)`,
	}
	for selector, color := range toastColors {
		rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(prefix+` `+selector) + `\s*\{([^}]*)\}`).FindStringSubmatch(adapter)
		if len(rule) != 2 {
			t.Errorf("Skills semantic toast rule missing for %s", selector)
			continue
		}
		for _, declaration := range []string{color, `background-image: none;`, `box-shadow: none;`} {
			if !strings.Contains(rule[1], declaration) {
				t.Errorf("Skills %s toast rule missing %q", selector, declaration)
			}
		}
	}
}

func TestPrecisionWorkspaceKnowledgeSkillsActiveDecorationsStayFlat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		stylesheet   string
		page         string
		selector     string
		declarations []string
	}{
		{
			name:         "Knowledge upload progress",
			stylesheet:   "css/knowledge.css",
			page:         "knowledge",
			selector:     ".kc-upload-progress-fill",
			declarations: []string{"background: var(--pw-accent);", "background-image: none;", "box-shadow: none;"},
		},
		{
			name:         "Skills active dropzone",
			stylesheet:   "css/skills.css",
			page:         "skills",
			selector:     ".sk-dropzone-active",
			declarations: []string{"background-image: none;", "box-shadow: none;", "transform: none;"},
		},
	}

	for _, test := range tests {
		css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
		prefix := `.pw-page[data-workspace-page="` + test.page + `"] `
		rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(prefix+test.selector) + `\s*\{([^}]*)\}`).FindStringSubmatch(css)
		if len(rule) != 2 {
			t.Errorf("%s rule is missing", test.name)
			continue
		}
		for _, declaration := range test.declarations {
			if !strings.Contains(rule[1], declaration) {
				t.Errorf("%s rule missing %q", test.name, declaration)
			}
		}
	}
}

func TestPrecisionWorkspaceKnowledgeSkillsModalARIAContract(t *testing.T) {
	t.Parallel()

	for _, template := range []string{"knowledge.html", "skills.html"} {
		html := normalizeAssetText(mustReadUIFile(t, template))
		overlays := regexp.MustCompile(`<div class="modal-overlay[^"]*"[^>]*>`).FindAllString(html, -1)
		if len(overlays) == 0 {
			t.Fatalf("%s has no modal overlays", template)
		}
		for _, overlay := range overlays {
			if !strings.Contains(overlay, `role="dialog"`) || !strings.Contains(overlay, `aria-modal="true"`) {
				t.Errorf("%s modal overlay lacks dialog semantics: %s", template, overlay)
			}
			label := regexp.MustCompile(`aria-labelledby="([^"]+)"`).FindStringSubmatch(overlay)
			if len(label) != 2 {
				t.Errorf("%s modal overlay lacks aria-labelledby: %s", template, overlay)
				continue
			}
			if !strings.Contains(html, `id="`+label[1]+`"`) {
				t.Errorf("%s modal references missing label id %q", template, label[1])
			}
		}
	}

	client := normalizeAssetText(mustReadUIFile(t, "js/skills/main.js"))
	for _, marker := range []string{
		`overlay.setAttribute('role', 'dialog');`,
		`overlay.setAttribute('aria-modal', 'true');`,
		`overlay.setAttribute('aria-labelledby', 'sk-doc-editor-title');`,
		`<h2 id="sk-doc-editor-title">`,
		`class="sk-input sk-textarea sk-doc-editor-textarea"`,
		`class="sk-form-group sk-doc-editor-upload"`,
		`aria-label="${esc(t('common.close'))}"`,
	} {
		if !strings.Contains(client, marker) {
			t.Errorf("Dynamic Skills documentation modal missing %q", marker)
		}
	}
}

func TestPrecisionWorkspaceModalFocusContractIsGenericAndIdempotent(t *testing.T) {
	t.Parallel()

	client := normalizeAssetText(mustReadUIFile(t, "js/precision/workspace.js"))
	for _, marker := range []string{
		`const activeModalOverlays = new Set();`,
		`let modalObserver = null;`,
		`function enhanceModalOverlay(overlay)`,
		`!overlay.matches(MODAL_SELECTOR)`,
		`syncModalSemantics(overlay);`,
		`overlay.dataset.pwModalBound === 'true'`,
		`overlay.dataset.pwModalBound = 'true'`,
		`function isModalOpen(overlay)`,
		`overlay.classList.contains('active')`,
		`overlay.classList.contains('open')`,
		`overlay.style.display`,
		`function activateModal(overlay)`,
		`function deactivateModal(overlay)`,
		`previousFocus`,
		`event.key !== 'Tab'`,
		`window.requestAnimationFrame`,
		`new MutationObserver`,
		`attributeFilter: ['class', 'style']`,
		`removedNodes`,
		`observeModalOverlays();`,
	} {
		if !strings.Contains(client, marker) {
			t.Errorf("Precision modal focus contract missing %q", marker)
		}
	}
}

func TestPrecisionWorkspaceModalSemanticsHandleNestedAndLateDialogContent(t *testing.T) {
	t.Parallel()

	client := normalizeAssetText(mustReadUIFile(t, "js/precision/workspace.js"))
	for _, marker := range []string{
		`function dialogTargetForOverlay(overlay)`,
		`overlay.querySelector('[role="dialog"]') || overlay`,
		`function syncModalSemantics(overlay)`,
		`dialogTarget !== overlay`,
		`overlay.removeAttribute('role');`,
		`overlay.removeAttribute('aria-modal');`,
		`dialogTarget.setAttribute('aria-modal', 'true');`,
		`document.getElementById(labelledBy)`,
		`dialogTarget.hasAttribute('aria-label')`,
		`dialogTarget.querySelector('.modal-title[id]')`,
		`dialogTarget.querySelector('.modal-title')`,
		`dialogTarget.querySelector('.modal-header h1, .modal-header h2, .modal-header h3')`,
		`dialogTarget.querySelector('h1, h2, h3')`,
		`record.target.closest(MODAL_SELECTOR)`,
	} {
		if !strings.Contains(client, marker) {
			t.Errorf("Precision nested/late modal semantic contract missing %q", marker)
		}
	}

	semanticsAt := strings.Index(client, `syncModalSemantics(overlay);`)
	boundAt := strings.Index(client, `overlay.dataset.pwModalBound === 'true'`)
	if semanticsAt < 0 || boundAt < 0 || semanticsAt >= boundAt {
		t.Error("Precision modal semantics must resync before the idempotent binding early return")
	}

	titleMarkers := []string{
		`document.getElementById(labelledBy)`,
		`dialogTarget.hasAttribute('aria-label')`,
		`dialogTarget.querySelector('.modal-title[id]')`,
		`dialogTarget.querySelector('.modal-title')`,
		`dialogTarget.querySelector('.modal-header h1, .modal-header h2, .modal-header h3')`,
		`dialogTarget.querySelector('h1, h2, h3')`,
	}
	previous := -1
	for _, marker := range titleMarkers {
		at := strings.Index(client, marker)
		if at <= previous {
			t.Errorf("Precision modal title resolution order is wrong at %q", marker)
		}
		previous = at
	}
}

func TestPrecisionWorkspaceKnowledgeModalChromeStaysPinned(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/knowledge.css"))
	for _, test := range []struct {
		selector string
		edge     string
	}{
		{selector: `.pw-page[data-workspace-page="knowledge"] .modal-header`, edge: `top: 0;`},
		{selector: `.pw-page[data-workspace-page="knowledge"] .modal-actions`, edge: `bottom: 0;`},
	} {
		rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(test.selector) + `\s*\{([^}]*)\}`).FindStringSubmatch(css)
		if len(rule) != 2 {
			t.Errorf("Knowledge modal chrome rule missing for %s", test.selector)
			continue
		}
		for _, declaration := range []string{`position: sticky;`, test.edge, `z-index: 2;`} {
			if !strings.Contains(rule[1], declaration) {
				t.Errorf("Knowledge modal chrome %s missing %q", test.selector, declaration)
			}
		}
	}
}

func TestPrecisionWorkspaceHiddenRevealUsesExplicitDisplayValues(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		stylesheet string
		name       string
	}{
		{stylesheet: "css/knowledge.css", name: "Knowledge"},
		{stylesheet: "css/skills.css", name: "Skills"},
	} {
		css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
		start := strings.Index(css, `/* === Precision Workspace `+test.name+` Adapter: start === */`)
		end := strings.Index(css, `/* === Precision Workspace `+test.name+` Adapter: end === */`)
		adapter := css[start:end]
		for _, fragile := range []string{`:has(`, `.is-hidden[style`} {
			if strings.Contains(adapter, fragile) {
				t.Errorf("%s adapter must not depend on fragile hidden reveal selector %q", test.name, fragile)
			}
		}
	}

	skills := normalizeAssetText(mustReadUIFile(t, "js/skills/main.js"))
	for _, marker := range []string{
		`document.getElementById('sk-disabled').style.display = 'block';`,
		`document.getElementById('agent-toolbar-actions').style.display = currentSkillMode === 'agent' ? 'flex' : 'none';`,
		`empty.style.display = 'block';`,
		`document.getElementById('agent-resource-browser').style.display = 'block';`,
		`document.getElementById('agent-file-editor').style.display = 'block';`,
		`document.getElementById('agent-binary-download').style.display = 'block';`,
		`errorEl.style.display = message ? 'block' : 'none';`,
		`warn.style.display = (ext === 'sh' || ext === 'js') ? 'flex' : 'none';`,
		`metaWrap.style.display = 'block';`,
		`document.getElementById('sk-selected-file').style.display = 'flex';`,
		`descDiv.style.display = 'block';`,
		`emptyEl.style.display = 'block';`,
		`if (bridgeOffEl) bridgeOffEl.style.display = 'block';`,
	} {
		if !strings.Contains(skills, marker) {
			t.Errorf("Skills explicit hidden reveal contract missing %q", marker)
		}
	}

	knowledge := normalizeAssetText(mustReadUIFile(t, "js/knowledge/main.js"))
	if !strings.Contains(knowledge, `empty.style.display = 'block';`) {
		t.Error("Knowledge device empty state must reveal with an explicit block display value")
	}
}

func TestPrecisionWorkspaceTranslations(t *testing.T) {
	t.Parallel()

	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"common.workspace_density_toggle",
		"common.workspace_density_comfortable",
		"common.workspace_density_compact",
	}
	valuesByLocale := make(map[string]map[string]string, len(locales))
	for _, locale := range locales {
		var values map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, "lang/common/"+locale+".json"), &values); err != nil {
			t.Fatalf("parse %s common translations: %v", locale, err)
		}
		valuesByLocale[locale] = values
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Errorf("%s is missing non-empty translation %q", locale, key)
			}
		}
	}
	for _, locale := range locales {
		if locale == "en" {
			continue
		}
		if valuesByLocale[locale][keys[0]] == valuesByLocale["en"][keys[0]] {
			t.Errorf("%s workspace density toggle must be translated, not copied from English", locale)
		}
	}
}

func TestPrecisionWorkspaceProtectedPagesStayOptedOut(t *testing.T) {
	t.Parallel()

	for _, page := range []string{"index.html", "desktop.html", "gallery.html"} {
		content := normalizeAssetText(mustReadUIFile(t, page))
		for _, forbidden := range []string{
			`precision-workspace.css`,
			`precision-pages.css`,
			`js/precision/workspace.js`,
			`data-workspace-page=`,
		} {
			if strings.Contains(content, forbidden) {
				t.Errorf("protected %s must not load or opt into %q", page, forbidden)
			}
		}
	}
}

func assertPrecisionCSSScoped(t *testing.T, css string) {
	t.Helper()

	comments := regexp.MustCompile(`(?s)/\*.*?\*/`)
	css = comments.ReplaceAllString(css, "")
	segmentStart := 0
	for index, char := range css {
		switch char {
		case '{':
			header := strings.TrimSpace(css[segmentStart:index])
			segmentStart = index + 1
			if header == "" || strings.HasPrefix(header, "@") {
				continue
			}
			for _, selector := range strings.Split(header, ",") {
				selector = strings.TrimSpace(selector)
				if selector != "" && !strings.Contains(selector, ".pw-page") {
					t.Errorf("unscoped Precision selector %q", selector)
				}
			}
		case '}':
			segmentStart = index + 1
		}
	}
}
