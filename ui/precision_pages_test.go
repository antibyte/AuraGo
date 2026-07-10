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
		`--pw-accent: #6f98bd;`,
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

func TestPrecisionWorkspaceSlatePaletteTokens(t *testing.T) {
	t.Parallel()

	foundation := normalizeAssetText(mustReadUIFile(t, "css/precision-workspace.css"))
	for _, token := range []string{
		`--pw-canvas: #10161e;`, `--pw-surface: #18212b;`,
		`--pw-surface-elevated: #202b37;`, `--pw-surface-soft: #2a3745;`,
		`--pw-text: #edf2f7;`, `--pw-muted: #aab7c4;`,
		`--pw-subtle: #7d8b99;`, `--pw-accent: #6f98bd;`,
		`--pw-accent-strong: #91b5d6;`,
	} {
		if !strings.Contains(foundation, token) {
			t.Errorf("missing dark slate token %q", token)
		}
	}
	for _, token := range []string{
		`--pw-canvas: #eef2f6;`, `--pw-surface: #fbfcfe;`,
		`--pw-surface-elevated: #f1f5f9;`, `--pw-surface-soft: #e3eaf1;`,
		`--pw-text: #182431;`, `--pw-muted: #5f6f7f;`,
		`--pw-subtle: #7b8997;`, `--pw-accent: #426d93;`,
		`--pw-accent-strong: #5d87aa;`,
	} {
		if !strings.Contains(foundation, token) {
			t.Errorf("missing light slate token %q", token)
		}
	}
	for _, alias := range []string{
		`--bg-primary: var(--pw-canvas);`,
		`--bg-secondary: var(--pw-surface);`,
		`--bg-tertiary: var(--pw-surface-elevated);`,
		`--header-bg: color-mix(in srgb, var(--pw-canvas) 88%, transparent);`,
		`--sidebar-bg: color-mix(in srgb, var(--pw-surface) 94%, transparent);`,
		`--card-bg: var(--pw-surface);`,
		`--input-bg: color-mix(in srgb, var(--pw-surface-elevated) 82%, var(--pw-canvas));`,
		`--text-primary: var(--pw-text);`,
		`--text-secondary: var(--pw-muted);`,
		`--text-tertiary: var(--pw-subtle);`,
		`--accent: var(--pw-accent);`,
		`--accent-dim: color-mix(in srgb, var(--pw-accent) 14%, transparent);`,
		`--border-subtle: var(--pw-line);`,
		`--border-accent: color-mix(in srgb, var(--pw-accent) 34%, transparent);`,
	} {
		if !strings.Contains(foundation, alias) {
			t.Errorf("compatibility alias must keep using Precision tokens: %q", alias)
		}
	}
	if strings.Contains(foundation, `#2dd4bf`) || strings.Contains(foundation, `#5eead4`) ||
		strings.Contains(foundation, `#0f766e`) || strings.Contains(foundation, `#0d9488`) {
		t.Fatal("Precision foundation must not retain the teal accent")
	}
}

func TestPrecisionWorkspaceTableDensityUsesTableCompatibleHeight(t *testing.T) {
	t.Parallel()

	components := normalizeAssetText(mustReadUIFile(t, "css/precision-pages.css"))
	start := strings.Index(components, `.pw-page .pw-table th,`)
	if start < 0 {
		t.Fatal("Precision table cell rule not found")
	}
	end := strings.Index(components[start:], `}`)
	if end < 0 {
		t.Fatal("Precision table cell rule is not closed")
	}
	rule := components[start : start+end]
	if !regexp.MustCompile(`(?m)^\s*height:\s*var\(--pw-row-height\);`).MatchString(rule) {
		t.Error("Precision table cells must use table-compatible height for density rows")
	}
	if strings.Contains(rule, `min-height:`) {
		t.Error("min-height does not control table-cell row density")
	}
}

func TestPrecisionWorkspaceOperationalMobileTargetsAreCentralAndEntrySafe(t *testing.T) {
	t.Parallel()

	components := normalizeAssetText(mustReadUIFile(t, "css/precision-pages.css"))
	mobileAt := strings.LastIndex(components, `@media (max-width: 639px)`)
	if mobileAt < 0 {
		t.Fatal("Precision operational mobile layer not found")
	}
	mobile := components[mobileAt:]
	for _, selector := range []string{
		`.pw-page.pw-operational-page button`,
		`.pw-page.pw-operational-page a[href]`,
		`.pw-page.pw-operational-page [role="button"]`,
		`.pw-page.pw-operational-page [role="tab"]`,
		`.pw-page.pw-operational-page input:not([type="checkbox"]):not([type="radio"]):not([type="range"]):not([type="hidden"])`,
		`.pw-page.pw-operational-page select`,
		`.pw-page.pw-operational-page textarea:not(.xterm-helper-textarea)`,
	} {
		if !strings.Contains(mobile, selector) {
			t.Errorf("Precision mobile target layer missing %q", selector)
		}
	}
	rule := regexp.MustCompile(`(?s)\.pw-page\.pw-operational-page button\s*,.*?\{([^}]*)\}`).FindStringSubmatch(mobile)
	if len(rule) != 2 {
		t.Fatal("Precision operational mobile target rule not found")
	}
	for _, declaration := range []string{`min-width: 44px !important;`, `min-height: 44px !important;`} {
		if !strings.Contains(rule[1], declaration) {
			t.Errorf("Precision operational mobile target rule missing cascade-safe %q", declaration)
		}
	}
	if strings.Contains(mobile, `.pw-page.pw-entry-page button`) {
		t.Error("operational mobile target rule must not opt entry pages into workspace sizing")
	}
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
		`/css/dashboard.css?v={{.BuildVersion}}`,
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

	dashboardAt := strings.Index(html, `/css/dashboard.css?v={{.BuildVersion}}`)
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
		prefix = `.pw-page[data-workspace-page="dashboard"]`
	)
	styles := css

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
		if !strings.Contains(styles, marker) {
			t.Errorf("Dashboard Precision styles missing marker %q", marker)
		}
	}
	assertPrecisionIntegratedRulesFlat(t, styles, prefix)

	comments := regexp.MustCompile(`(?s)/\*.*?\*/`)
	uncommented := comments.ReplaceAllString(styles, "")
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
				if strings.Contains(selector, ".pw-page") && !strings.HasPrefix(selector, prefix) {
					t.Errorf("Dashboard Precision styles selector must start with %q: %q", prefix, selector)
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
		mobileStart = `@media (max-width: 640px)`
		mobileEnd   = `@media (prefers-reduced-motion: reduce)`
		prefix      = `.pw-page[data-workspace-page="dashboard"][data-density="compact"]`
	)
	styles := css
	mobileAt := strings.Index(styles, mobileStart)
	reducedMotionAt := strings.Index(styles, mobileEnd)
	if mobileAt < 0 || reducedMotionAt <= mobileAt {
		t.Fatalf("Dashboard Precision styles missing ordered mobile block: mobile=%d reduced-motion=%d", mobileAt, reducedMotionAt)
	}
	mobile := styles[mobileAt:reducedMotionAt]

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
		prefix = `.pw-page[data-workspace-page="dashboard"]`
	)
	styles := css
	ruleAt := strings.Index(styles, prefix+` .dash-card:hover canvas,`)
	if ruleAt < 0 {
		t.Fatal("Dashboard Precision styles missing residual-glow suppression rule")
	}
	ruleEnd := strings.Index(styles[ruleAt:], "}")
	if ruleEnd < 0 {
		t.Fatal("Dashboard residual-glow suppression rule is not closed")
	}
	rule := styles[ruleAt : ruleAt+ruleEnd]
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
		prefix = `.pw-page[data-workspace-page="dashboard"]`
	)
	styles := css
	pulseSuppression := regexp.MustCompile(
		`(?s)` +
			regexp.QuoteMeta(prefix+` .pill-running`) + `\s*,\s*` +
			regexp.QuoteMeta(prefix+` .status-dot.green`) +
			`\s*\{[^}]*animation:\s*none;`,
	)
	if !pulseSuppression.MatchString(styles) {
		t.Error("Dashboard Precision styles must disable pulse-glow animation for running pills and green status dots")
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
			styles := css
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
				if !strings.Contains(styles, marker) {
					t.Errorf("%s Precision styles missing marker %q", test.name, marker)
				}
			}
			assertPrecisionIntegratedRulesFlat(t, styles, prefix)
			assertPrecisionAdapterSelectorsScoped(t, styles, prefix)
		})
	}
}

func assertPrecisionIntegratedRulesFlat(t *testing.T, css, prefix string) {
	t.Helper()

	rules := regexp.MustCompile(`(?s)([^{}]+)\{([^{}]*)\}`)
	for _, rule := range rules.FindAllStringSubmatch(css, -1) {
		if strings.Contains(rule[1], prefix) && strings.Contains(strings.ToLower(rule[2]), "gradient(") {
			t.Errorf("integrated Precision rule %q must not introduce gradients", strings.TrimSpace(rule[1]))
		}
	}
}

func assertPrecisionAdapterSelectorsScoped(t *testing.T, styles, prefix string) {
	t.Helper()

	comments := regexp.MustCompile(`(?s)/\*.*?\*/`)
	uncommented := comments.ReplaceAllString(styles, "")
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
				if strings.Contains(selector, ".pw-page") && !strings.HasPrefix(selector, prefix) {
					t.Errorf("Precision styles selector must start with %q: %q", prefix, selector)
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
		prefix = `.pw-page[data-workspace-page="missions"]`
	)
	styles := css
	pulseSuppression := regexp.MustCompile(
		`(?s)` +
			regexp.QuoteMeta(prefix+` .badge-prep-preparing`) + `\s*,\s*` +
			regexp.QuoteMeta(prefix+` .mc-status-chip--running`) +
			`\s*\{[^}]*animation:\s*none;[^}]*box-shadow:\s*none;`,
	)
	if !pulseSuppression.MatchString(styles) {
		t.Error("Missions Precision styles must disable preparation and running-chip pulse glows")
	}

	statusPulseSuppression := regexp.MustCompile(
		`(?s)` + regexp.QuoteMeta(prefix+` .status-card.running`) +
			`\s*\{[^}]*animation:\s*none;[^}]*box-shadow:\s*none;`,
	)
	if !statusPulseSuppression.MatchString(styles) {
		t.Error("Missions Precision styles must disable the running summary-card pulse while keeping its semantic styling")
	}

	preparedGlowSuppression := regexp.MustCompile(
		`(?s)` + regexp.QuoteMeta(prefix+` .badge-prep-prepared`) +
			`\s*\{[^}]*box-shadow:\s*none;`,
	)
	if !preparedGlowSuppression.MatchString(styles) {
		t.Error("Missions Precision styles must remove the prepared badge decorative glow")
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
		prefix = `.pw-page[data-workspace-page="missions"]`
	)
	styles := css

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
		if !strings.Contains(styles, marker) {
			t.Errorf("Missions compact renderer styles missing marker %q", marker)
		}
	}
}

func TestPrecisionWorkspaceMissionsCompactListActionsStayTouchSizedOnMobile(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/missions.css"))
	const (
		mobileStart = `@media (max-width: 640px)`
		mobileEnd   = `@media (prefers-reduced-motion: reduce)`
		prefix      = `.pw-page[data-workspace-page="missions"][data-density="compact"] .card-actions .mc-btn`
	)
	styles := css

	desktopCompact := regexp.MustCompile(
		`(?s)` + regexp.QuoteMeta(prefix) +
			`\s*\{[^}]*width:\s*36px;[^}]*height:\s*36px;[^}]*min-height:\s*36px;`,
	)
	if !desktopCompact.MatchString(styles) {
		t.Error("Missions compact list actions must preserve the 36px desktop density contract")
	}

	mobileAt := strings.Index(styles, mobileStart)
	reducedMotionAt := strings.Index(styles, mobileEnd)
	if mobileAt < 0 || reducedMotionAt <= mobileAt {
		t.Fatalf("Missions styles missing ordered mobile block: mobile=%d reduced-motion=%d", mobileAt, reducedMotionAt)
	}
	mobile := styles[mobileAt:reducedMotionAt]
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
			styles := css
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
				if !strings.Contains(styles, marker) {
					t.Errorf("%s Precision styles missing marker %q", test.name, marker)
				}
			}
			assertPrecisionIntegratedRulesFlat(t, styles, prefix)
			assertPrecisionAdapterSelectorsScoped(t, styles, prefix)
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
			styles := css
			prefix := `.pw-page[data-workspace-page="` + test.page + `"]`
			for _, selector := range test.selectors {
				if !strings.Contains(styles, prefix+` `+selector) {
					t.Errorf("%s styles does not explicitly suppress legacy decoration for %s", test.name, selector)
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
			styles := css
			mobileAt := strings.Index(styles, `@media (max-width: 640px)`)
			reducedAt := strings.Index(styles, `@media (prefers-reduced-motion: reduce)`)
			if mobileAt < 0 || reducedAt <= mobileAt {
				t.Fatalf("%s missing ordered mobile and reduced-motion blocks", test.stylesheet)
			}
			mobile := styles[mobileAt:reducedAt]
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
	styles := css
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
		if !strings.Contains(styles, marker) {
			t.Errorf("Skills fullscreen contract missing %q", marker)
		}
	}

	toastColors := map[string]string{
		`.sk-toast-success`: `var(--pw-success)`,
		`.sk-toast-error`:   `var(--pw-danger)`,
		`.sk-toast-info`:    `var(--pw-accent)`,
	}
	for selector, color := range toastColors {
		rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(prefix+` `+selector) + `\s*\{([^}]*)\}`).FindStringSubmatch(styles)
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
		`function isModalBoundary(element)`,
		`!isModalBoundary(overlay)`,
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
		`attributeFilter: ['class', 'style', 'hidden']`,
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
		`modalBoundaryForElement(record.target)`,
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

func TestPrecisionWorkspaceModalFocusContractSupportsStandaloneDialogs(t *testing.T) {
	t.Parallel()

	client := normalizeAssetText(mustReadUIFile(t, "js/precision/workspace.js"))
	for _, marker := range []string{
		`const MODAL_OVERLAY_SELECTOR = '.modal-overlay';`,
		`const STANDALONE_DIALOG_SELECTOR = '[role="dialog"][aria-modal="true"]';`,
		`const MODAL_BOUNDARY_SELECTOR = MODAL_OVERLAY_SELECTOR + ', ' + STANDALONE_DIALOG_SELECTOR;`,
		`function isModalBoundary(element)`,
		`element.matches(MODAL_BOUNDARY_SELECTOR)`,
		`element.parentElement && element.parentElement.closest(MODAL_BOUNDARY_SELECTOR)`,
		`function modalBoundaryForElement(element)`,
		`function modalBoundariesWithin(node)`,
		`.filter(isModalBoundary)`,
		`document.querySelectorAll(MODAL_BOUNDARY_SELECTOR)`,
		`overlay.matches(STANDALONE_DIALOG_SELECTOR) ? overlay`,
		`overlay.hidden`,
		`overlay.classList.contains('is-hidden')`,
		`overlay.matches(MODAL_OVERLAY_SELECTOR)`,
		`window.getComputedStyle(overlay)`,
		`computed.display !== 'none'`,
		`computed.visibility !== 'hidden'`,
		`computed.opacity !== '0'`,
		`computed.pointerEvents !== 'none'`,
	} {
		if !strings.Contains(client, marker) {
			t.Errorf("Precision standalone modal focus contract missing %q", marker)
		}
	}

	isOpenStart := strings.Index(client, `function isModalOpen(overlay)`)
	isOpenEnd := strings.Index(client, `function focusModal(overlay)`)
	if isOpenStart < 0 || isOpenEnd <= isOpenStart {
		t.Fatal("cannot locate isModalOpen")
	}
	isOpen := client[isOpenStart:isOpenEnd]
	hiddenAt := strings.Index(isOpen, `overlay.hidden`)
	activeAt := strings.Index(isOpen, `overlay.classList.contains('active')`)
	inlineAt := strings.Index(isOpen, `if (inlineDisplay) return true;`)
	classicClosedAt := strings.Index(isOpen, `if (overlay.matches(MODAL_OVERLAY_SELECTOR)) return false;`)
	computedAt := strings.Index(isOpen, `window.getComputedStyle(overlay)`)
	if hiddenAt < 0 || activeAt <= hiddenAt || inlineAt <= activeAt || classicClosedAt <= inlineAt || computedAt <= classicClosedAt {
		t.Error("isModalOpen must support explicit display, then keep classic overlays closed before standalone computed visibility fallback")
	}
}

func TestPrecisionWorkspaceStandaloneModalVisibilityMatchesOperationsOpenPaths(t *testing.T) {
	t.Parallel()

	client := normalizeAssetText(mustReadUIFile(t, "js/precision/workspace.js"))
	isOpenStart := strings.Index(client, `function isModalOpen(overlay)`)
	isOpenEnd := strings.Index(client, `function focusModal(overlay)`)
	if isOpenStart < 0 || isOpenEnd <= isOpenStart {
		t.Fatal("cannot locate isModalOpen")
	}
	isOpen := client[isOpenStart:isOpenEnd]
	for _, marker := range []string{
		`computed.display !== 'none'`,
		`computed.visibility !== 'hidden'`,
		`computed.opacity !== '0'`,
		`computed.pointerEvents !== 'none'`,
	} {
		if !strings.Contains(isOpen, marker) {
			t.Errorf("standalone computed visibility must reject hidden Dashboard-style state via %q", marker)
		}
	}
	activeAt := strings.Index(isOpen, `overlay.classList.contains('active')`)
	computedAt := strings.Index(isOpen, `window.getComputedStyle(overlay)`)
	if activeAt < 0 || computedAt <= activeAt {
		t.Error("active/open modal state must remain accepted before standalone computed checks")
	}

	media := normalizeAssetText(mustReadUIFile(t, "media.html")) + normalizeAssetText(mustReadUIFile(t, "js/media/main.js"))
	for _, marker := range []string{`id="lightbox" class="lightbox is-hidden" role="dialog"`, `modal.classList.remove('is-hidden');`} {
		if !strings.Contains(media, marker) {
			t.Errorf("Media standalone dialog open path missing %q", marker)
		}
	}
	truenas := normalizeAssetText(mustReadUIFile(t, "truenas.html")) + normalizeAssetText(mustReadUIFile(t, "js/truenas.js"))
	for _, marker := range []string{`class="truenas-modal" role="dialog"`, `classList.add('active')`} {
		if !strings.Contains(truenas, marker) {
			t.Errorf("TrueNAS standalone dialog open path missing %q", marker)
		}
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
		styles := css
		for _, fragile := range []string{`:has(`, `.is-hidden[style`} {
			if strings.Contains(styles, fragile) {
				t.Errorf("%s styles must not depend on fragile hidden reveal selector %q", test.name, fragile)
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

func TestPrecisionWorkspaceOperationsIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		template    string
		stylesheet  string
		page        string
		mainScript  string
		hooks       []string
		hiddenHooks []string
	}{
		{
			name: "Containers", template: "containers.html", stylesheet: "/css/containers.css", page: "containers", mainScript: "/js/containers/main.js",
			hooks:       []string{`id="ct-status-bar"`, `id="ct-search"`, `id="ct-grid"`, `id="terminal-output"`, `id="terminal-status"`},
			hiddenHooks: []string{`id="ct-empty"`, `id="ct-disabled"`},
		},
		{
			name: "Media", template: "media.html", stylesheet: "/css/media.css", page: "media", mainScript: "/js/media/main.js",
			hooks:       []string{`id="media-search"`, `id="gallery-grid"`, `id="audio-grid"`, `id="video-grid"`, `id="doc-list"`},
			hiddenHooks: []string{`id="gallery-pagination"`, `id="audio-pagination"`},
		},
		{
			name: "TrueNAS", template: "truenas.html", stylesheet: "/css/truenas.css", page: "truenas", mainScript: "/js/truenas.js",
			hooks:       []string{`id="status-indicator"`, `id="pools-container"`, `id="datasets-container"`, `id="snapshots-container"`, `id="shares-container"`},
			hiddenHooks: []string{`id="nfs-share-fields"`},
		},
		{
			name: "Invasion", template: "invasion_control.html", stylesheet: "/css/invasion.css", page: "invasion", mainScript: "/js/invasion/main.js",
			hooks:       []string{`id="nests-grid"`, `id="eggs-grid"`, `id="nest-save-btn"`, `id="egg-save-btn"`, `id="config-history-list"`},
			hiddenHooks: []string{`id="nests-empty"`, `id="eggs-empty"`},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			html := normalizeAssetText(mustReadUIFile(t, test.template))
			body := `<body class="pw-page pw-operational-page" data-workspace-page="` + test.page + `" data-density="comfortable">`
			if !strings.Contains(html, body) {
				t.Errorf("%s missing Precision body opt-in", test.template)
			}
			pageCSSAt := strings.Index(html, test.stylesheet)
			enhancementsAt := strings.Index(html, `/css/enhancements.css?v=20260425a`)
			foundationAt := strings.Index(html, `/css/precision-workspace.css?v={{.BuildVersion}}`)
			componentsAt := strings.Index(html, `/css/precision-pages.css?v={{.BuildVersion}}`)
			if pageCSSAt < 0 || enhancementsAt < 0 || foundationAt < 0 || componentsAt < 0 ||
				!(pageCSSAt < enhancementsAt && enhancementsAt < foundationAt && foundationAt < componentsAt) {
				t.Errorf("%s Precision CSS order = page:%d enhancements:%d foundation:%d components:%d", test.name, pageCSSAt, enhancementsAt, foundationAt, componentsAt)
			}
			workspaceAt := strings.Index(html, `/js/precision/workspace.js?v={{.BuildVersion}}`)
			mainAt := strings.Index(html, test.mainScript)
			if workspaceAt < 0 || mainAt < 0 || workspaceAt >= mainAt {
				t.Errorf("%s script order = workspace:%d main:%d", test.name, workspaceAt, mainAt)
			}
			if regexp.MustCompile(`(?i)\sstyle\s*=`).MatchString(html) {
				t.Errorf("%s must not retain template inline styles", test.template)
			}
			for _, hook := range append(test.hooks, test.hiddenHooks...) {
				if !strings.Contains(html, hook) {
					t.Errorf("%s lost functional hook %q", test.template, hook)
				}
			}
			for _, hook := range test.hiddenHooks {
				at := strings.Index(html, hook)
				start := strings.LastIndex(html[:at], `<`)
				end := strings.Index(html[at:], `>`)
				if at < 0 || start < 0 || end < 0 || !strings.Contains(html[start:at+end], `is-hidden`) {
					t.Errorf("%s hidden hook %q must use is-hidden", test.template, hook)
				}
			}
		})
	}
}

func TestPrecisionWorkspaceOperationsModalARIAContract(t *testing.T) {
	t.Parallel()

	modals := map[string][]string{
		"containers.html":       {"log-modal", "inspect-modal", "terminal-modal", "update-modal", "delete-modal"},
		"media.html":            {"lightbox", "audio-modal"},
		"truenas.html":          {"modal-dataset", "modal-snapshot", "modal-share"},
		"invasion_control.html": {"nest-modal", "egg-modal", "reconfigure-modal", "config-history-modal", "delete-modal"},
	}
	for template, ids := range modals {
		html := normalizeAssetText(mustReadUIFile(t, template))
		for _, id := range ids {
			tag := regexp.MustCompile(`<div[^>]*id="` + regexp.QuoteMeta(id) + `"[^>]*>`).FindString(html)
			if tag == "" {
				t.Errorf("%s missing modal %s", template, id)
				continue
			}
			if !strings.Contains(tag, `role="dialog"`) || !strings.Contains(tag, `aria-modal="true"`) {
				t.Errorf("%s modal %s lacks dialog semantics: %s", template, id, tag)
			}
			label := regexp.MustCompile(`aria-labelledby="([^"]+)"`).FindStringSubmatch(tag)
			if len(label) != 2 {
				t.Errorf("%s modal %s lacks aria-labelledby", template, id)
				continue
			}
			if !strings.Contains(html, `id="`+label[1]+`"`) {
				t.Errorf("%s modal %s references missing title %q", template, id, label[1])
			}
		}
	}
}

func TestPrecisionWorkspaceOperationsAdaptersAreScopedAndResponsive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		stylesheet string
		page       string
		markers    []string
	}{
		{name: "Containers", stylesheet: "css/containers.css", page: "containers", markers: []string{`.ct-status-bar`, `.ct-grid`, `.ct-card`, `.ct-log-output`, `.ct-terminal-output`, `.ct-terminal-output .xterm`}},
		{name: "Media", stylesheet: "css/media.css", page: "media", markers: []string{`.media-tabs`, `.gallery-grid`, `.media-audio-card`, `.media-video-card`, `.media-doc-row`, `.lightbox-content`, `.media-modal-content`}},
		{name: "TrueNAS", stylesheet: "css/truenas.css", page: "truenas", markers: []string{`.connection-status`, `.stats-grid`, `.pool-card`, `.dataset-item`, `.snapshot-item`, `.share-item`, `.truenas-modal`}},
		{name: "Invasion", stylesheet: "css/invasion.css", page: "invasion", markers: []string{`.invasion-tabs`, `.cards-grid`, `.card`, `.inv-telemetry-row`, `.config-history-item`, `.modal-overlay`}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
			styles := css
			prefix := `.pw-page[data-workspace-page="` + test.page + `"]`
			for _, marker := range append([]string{
				prefix + ` {`, `overflow-x: clip;`, `background-image: none;`, `box-shadow: none;`, `filter: none;`,
				prefix + `[data-density="compact"]`, `:root[data-theme="light"] *`, `@media (max-width: 1024px)`, `@media (max-width: 640px)`,
				`min-height: 44px;`, `min-height: 100dvh;`, `max-height: calc(100dvh - 1rem);`, `border-radius: 20px 20px 0 0;`,
				`overflow-wrap: anywhere;`, `@media (prefers-reduced-motion: reduce)`,
			}, test.markers...) {
				if !strings.Contains(styles, marker) {
					t.Errorf("%s styles missing %q", test.name, marker)
				}
			}
			assertPrecisionIntegratedRulesFlat(t, styles, prefix)
			assertPrecisionAdapterSelectorsScoped(t, styles, prefix)
		})
	}
}

func TestPrecisionWorkspaceOperationsAdaptersCoverRenderedStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		stylesheet string
		script     string
		page       string
		selectors  []string
	}{
		{name: "Containers", stylesheet: "css/containers.css", script: "js/containers/main.js", page: "containers", selectors: []string{".ct-card", ".ct-card-actions", ".ct-card-status.running", ".ct-card-state.running"}},
		{name: "Media", stylesheet: "css/media.css", script: "js/media/main.js", page: "media", selectors: []string{".media-audio-card", ".media-video-card", ".media-doc-row", ".gallery-empty"}},
		{name: "TrueNAS", stylesheet: "css/truenas.css", script: "js/truenas.js", page: "truenas", selectors: []string{".pool-card", ".dataset-item", ".snapshot-item", ".share-item", ".alert.error"}},
		{name: "Invasion", stylesheet: "css/invasion.css", script: "js/invasion/main.js", page: "invasion", selectors: []string{".card", ".inv-telemetry-row", ".config-history-item", ".config-history-error"}},
	}
	for _, test := range tests {
		css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
		script := normalizeAssetText(mustReadUIFile(t, test.script))
		prefix := `.pw-page[data-workspace-page="` + test.page + `"] `
		for _, selector := range test.selectors {
			className := strings.TrimPrefix(strings.Split(selector, ".")[1], "")
			if !strings.Contains(script, className) {
				t.Errorf("%s test selector %s is not derived from rendered JS", test.name, selector)
			}
			if !strings.Contains(css, prefix+selector) {
				t.Errorf("%s styles misses rendered state %s", test.name, selector)
			}
		}
	}
}

func TestPrecisionWorkspaceOperationsCompactMobileControlsWinCascade(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, stylesheet, page string
		controls               []string
	}{
		{name: "Containers", stylesheet: "css/containers.css", page: "containers", controls: []string{".ct-search", ".ct-filter-btn", ".btn", ".modal-close"}},
		{name: "Media", stylesheet: "css/media.css", page: "media", controls: []string{".gallery-search", ".media-tab", ".btn-gallery-nav", ".btn-gallery-action"}},
		{name: "TrueNAS", stylesheet: "css/truenas.css", page: "truenas", controls: []string{".nav-btn", ".btn", "input", "select"}},
		{name: "Invasion", stylesheet: "css/invasion.css", page: "invasion", controls: []string{".invasion-tab", ".btn", "input", "select"}},
	}
	for _, test := range tests {
		css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
		styles := css
		mobileAt := strings.Index(styles, `@media (max-width: 640px)`)
		reducedAt := strings.Index(styles, `@media (prefers-reduced-motion: reduce)`)
		if mobileAt < 0 || reducedAt <= mobileAt {
			t.Fatalf("%s mobile block must precede reduced motion", test.name)
		}
		mobile := styles[mobileAt:reducedAt]
		prefix := `.pw-page[data-workspace-page="` + test.page + `"][data-density="compact"] `
		for _, control := range test.controls {
			if !regexp.MustCompile(`(?s)` + regexp.QuoteMeta(prefix+control) + `[^{}]*\{[^}]*min-height:\s*44px;`).MatchString(mobile) {
				t.Errorf("%s compact mobile %s needs 44px override", test.name, control)
			}
		}
	}
}

func TestPrecisionWorkspaceOperationsHiddenStatesRemainRevealable(t *testing.T) {
	t.Parallel()

	containers := normalizeAssetText(mustReadUIFile(t, "js/containers/main.js"))
	for _, marker := range []string{
		`document.getElementById('ct-disabled').classList.remove('is-hidden');`,
		`disabled.classList.add('is-hidden');`,
	} {
		if !strings.Contains(containers, marker) {
			t.Errorf("Containers hidden-state migration missing %q", marker)
		}
	}

	truenas := normalizeAssetText(mustReadUIFile(t, "js/truenas.js"))
	if !strings.Contains(truenas, `nfsFields.classList.toggle('is-hidden', type !== 'nfs');`) {
		t.Error("TrueNAS NFS fields must toggle the migrated is-hidden class")
	}
}

func TestPrecisionWorkspaceOperationsSemanticStatesStayFlat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		stylesheet string
		page       string
		selector   string
		color      string
	}{
		{stylesheet: "css/containers.css", page: "containers", selector: ".ct-toast.success", color: "var(--pw-success)"},
		{stylesheet: "css/containers.css", page: "containers", selector: ".ct-toast.error", color: "var(--pw-danger)"},
		{stylesheet: "css/invasion.css", page: "invasion", selector: ".toast.success", color: "var(--pw-success)"},
		{stylesheet: "css/invasion.css", page: "invasion", selector: ".toast.error", color: "var(--pw-danger)"},
	}
	for _, test := range tests {
		css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
		selector := `.pw-page[data-workspace-page="` + test.page + `"] ` + test.selector
		rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(selector) + `\s*\{([^}]*)\}`).FindStringSubmatch(css)
		if len(rule) != 2 {
			t.Errorf("%s missing flat semantic state %s", test.stylesheet, test.selector)
			continue
		}
		for _, marker := range []string{test.color, `background-image: none;`, `box-shadow: none;`} {
			if !strings.Contains(rule[1], marker) {
				t.Errorf("%s %s missing %q", test.stylesheet, test.selector, marker)
			}
		}
	}

	media := normalizeAssetText(mustReadUIFile(t, "css/media.css"))
	selector := `.pw-page[data-workspace-page="media"] .media-tab.active::after`
	rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(selector) + `\s*\{([^}]*)\}`).FindStringSubmatch(media)
	if len(rule) != 2 || !strings.Contains(rule[1], `box-shadow: none;`) {
		t.Error("Media active tab must suppress the inherited light-theme glow")
	}
}

func TestPrecisionWorkspaceOperationsFocusVisibleRulesArePageScoped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, stylesheet, page string
		controls               []string
	}{
		{name: "Containers", stylesheet: "css/containers.css", page: "containers", controls: []string{".ct-search", ".ct-filter-btn", ".ct-card-actions .btn", ".modal-actions .btn", ".modal-close", `.ct-checkbox-label input[type="checkbox"]`}},
		{name: "Media", stylesheet: "css/media.css", page: "media", controls: []string{".gallery-search", ".gallery-filter", ".media-tab", ".btn-gallery-nav", ".btn-gallery-action", ".lightbox-close", ".audio-play-btn", ".audio-speed-btn", ".audio-download-btn", ".media-doc-row-actions a", ".media-doc-row-actions button"}},
		{name: "TrueNAS", stylesheet: "css/truenas.css", page: "truenas", controls: []string{".nav-btn", ".btn", "input", "select", "textarea"}},
		{name: "Invasion", stylesheet: "css/invasion.css", page: "invasion", controls: []string{".invasion-tab", ".btn", ".modal-close", "input", "select", "textarea"}},
	}

	for _, test := range tests {
		css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
		styles := css
		prefix := `.pw-page[data-workspace-page="` + test.page + `"] `
		for _, control := range test.controls {
			selector := prefix + control + `:focus-visible`
			rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(selector) + `[^{}]*\{([^}]*)\}`).FindStringSubmatch(styles)
			if len(rule) != 2 {
				t.Errorf("%s missing scoped focus rule for %s", test.name, control)
				continue
			}
			for _, declaration := range []string{`outline: 2px solid var(--pw-accent);`, `outline-offset: 2px;`, `box-shadow: none;`} {
				if !strings.Contains(rule[1], declaration) {
					t.Errorf("%s focus rule for %s missing %q", test.name, control, declaration)
				}
			}
		}
	}
}

func TestPrecisionWorkspaceOperationsMobileActionTargetsCoverBothDensities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, stylesheet, page string
		controls               []string
		sources                []string
		renderedClasses        []string
	}{
		{
			name: "Containers", stylesheet: "css/containers.css", page: "containers",
			controls: []string{".btn-theme", ".ct-search", ".ct-filter-btn", ".btn", ".ct-card-actions .btn", ".modal-actions .btn", ".modal-close", `.ct-checkbox-label input[type="checkbox"]`},
			sources:  []string{"containers.html", "js/containers/main.js"}, renderedClasses: []string{"btn-theme", "ct-search", "ct-filter-btn", "ct-card-actions", "modal-actions", "modal-close", "ct-checkbox-label"},
		},
		{
			name: "Media", stylesheet: "css/media.css", page: "media",
			controls: []string{".btn-theme", ".gallery-search", ".gallery-filter", ".media-tab", ".btn-gallery-nav", ".btn-gallery-action", ".media-select-check-wrap", ".lightbox-close", ".lightbox-actions .btn-gallery-action", ".audio-play-btn", ".audio-speed-btn", ".audio-download-btn", ".media-doc-row-actions a", ".media-doc-row-actions button"},
			sources:  []string{"media.html", "js/media/main.js", "js/gallery/main.js", "js/chat/audio-player.js"}, renderedClasses: []string{"btn-theme", "gallery-search", "gallery-filter", "media-tab", "btn-gallery-nav", "btn-gallery-action", "media-select-check-wrap", "lightbox-close", "lightbox-actions", "audio-play-btn", "audio-speed-btn", "audio-download-btn", "media-doc-row-actions"},
		},
		{
			name: "TrueNAS", stylesheet: "css/truenas.css", page: "truenas",
			controls: []string{".btn-theme", ".nav-btn", ".btn", "input", "select", "textarea", ".pool-actions .btn", ".dataset-actions .btn", ".snapshot-actions .btn", ".share-actions .btn", ".form-actions .btn"},
			sources:  []string{"truenas.html", "js/truenas.js"}, renderedClasses: []string{"btn-theme", "nav-btn", "pool-actions", "dataset-actions", "snapshot-actions", "share-actions", "form-actions"},
		},
		{
			name: "Invasion", stylesheet: "css/invasion.css", page: "invasion",
			controls: []string{".btn-theme", ".invasion-tab", ".btn", "input", "select", "textarea", ".card-actions .btn", ".modal-actions .btn", ".modal-close", ".rev-actions .btn"},
			sources:  []string{"invasion_control.html", "js/invasion/main.js"}, renderedClasses: []string{"btn-theme", "invasion-tab", "card-actions", "modal-actions", "modal-close", "rev-actions"},
		},
	}

	for _, test := range tests {
		css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
		styles := css
		mobileAt := strings.Index(styles, `@media (max-width: 640px)`)
		reducedAt := strings.Index(styles, `@media (prefers-reduced-motion: reduce)`)
		if mobileAt < 0 || reducedAt <= mobileAt {
			t.Fatalf("%s missing ordered styles/mobile contract", test.name)
		}
		mobile := styles[mobileAt:reducedAt]
		prefixes := map[string]string{
			"comfortable": `.pw-page[data-workspace-page="` + test.page + `"] `,
			"compact":     `.pw-page[data-workspace-page="` + test.page + `"][data-density="compact"] `,
		}
		for density, prefix := range prefixes {
			for _, control := range test.controls {
				rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(prefix+control) + `[^{}]*\{([^}]*)\}`).FindStringSubmatch(mobile)
				if len(rule) != 2 {
					t.Errorf("%s missing cascade-safe %s mobile rule for %s", test.name, density, control)
					continue
				}
				for _, declaration := range []string{`min-height: 44px;`, `min-width: 44px;`} {
					if !strings.Contains(rule[1], declaration) {
						t.Errorf("%s %s mobile %s missing %q", test.name, density, control, declaration)
					}
				}
			}
		}
		var source string
		for _, path := range test.sources {
			source += normalizeAssetText(mustReadUIFile(t, path))
		}
		for _, className := range test.renderedClasses {
			if !strings.Contains(source, className) {
				t.Errorf("%s target class %s is not present in its template/renderer", test.name, className)
			}
		}
	}
}

func TestPrecisionWorkspaceMediaSelectionGlowOverrideWinsImportantLegacyRule(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/media.css"))
	selector := `.pw-page[data-workspace-page="media"] .media-card-selected`
	rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(selector) + `\s*\{([^}]*)\}`).FindStringSubmatch(css)
	if len(rule) != 2 || !strings.Contains(rule[1], `box-shadow: none !important;`) {
		t.Error("Media selected cards must override the legacy important glow with a scoped important flat rule")
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

func TestPrecisionEntryPagesOptInWithoutOperationalWorkspace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		template   string
		entryPage  string
		pageCSS    string
		pageCSSPos string
	}{
		{name: "Login", template: "login.html", entryPage: "login", pageCSS: `/css/login.css`, pageCSSPos: `/css/enhancements.css?v=20260425a`},
		{name: "Setup", template: "setup.html", entryPage: "setup", pageCSS: `/css/setup.css`, pageCSSPos: `/css/enhancements.css?v=20260425a`},
		{name: "NotFound", template: "404.html", entryPage: "not-found", pageCSS: `/css/not-found.css?v={{.BuildVersion}}`, pageCSSPos: `/css/precision-workspace.css?v={{.BuildVersion}}`},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			html := normalizeAssetText(mustReadUIFile(t, test.template))
			body := `<body class="pw-page pw-entry-page" data-entry-page="` + test.entryPage + `">`
			if !strings.Contains(html, body) {
				t.Errorf("%s missing exact entry body marker %q", test.template, body)
			}

			foundationAt := strings.Index(html, `/css/precision-workspace.css?v={{.BuildVersion}}`)
			entryAt := strings.Index(html, `/css/precision-entry.css?v={{.BuildVersion}}`)
			pageAt := strings.Index(html, test.pageCSS)
			pageBoundaryAt := strings.Index(html, test.pageCSSPos)
			if foundationAt < 0 || entryAt < 0 || pageAt < 0 || pageBoundaryAt < 0 || foundationAt >= entryAt {
				t.Errorf("%s missing ordered entry assets: page=%d boundary=%d foundation=%d entry=%d", test.template, pageAt, pageBoundaryAt, foundationAt, entryAt)
			}
			if test.template == "404.html" {
				if !(pageAt < foundationAt && foundationAt < entryAt) {
					t.Errorf("404 stylesheet order = not-found:%d foundation:%d entry:%d", pageAt, foundationAt, entryAt)
				}
			} else if !(pageAt < pageBoundaryAt && pageBoundaryAt < foundationAt && foundationAt < entryAt) {
				t.Errorf("%s stylesheet order = page:%d enhancements:%d foundation:%d entry:%d", test.template, pageAt, pageBoundaryAt, foundationAt, entryAt)
			}

			for _, forbidden := range []string{
				`precision-pages.css`, `js/precision/workspace.js`, `data-density=`, `data-workspace-page=`,
				`data-pw-density-toggle`, `radialMenuAnchor`,
			} {
				if strings.Contains(html, forbidden) {
					t.Errorf("entry template %s must not include operational marker %q", test.template, forbidden)
				}
			}
		})
	}
}

func TestPrecisionEntryTemplatesPreserveHooksWithoutInlineStyles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		template string
		hooks    []string
	}{
		{template: "login.html", hooks: []string{
			`id="bg-canvas"`, `id="css-bg"`, `id="password"`, `id="totpSection"`, `id="totpCode"`,
			`id="btnLogin"`, `id="loginError"`, `/js/vendor/three.min.js`, `/js/login/main.js`, `submitLogin()`,
		}},
		{template: "setup.html", hooks: []string{
			`id="step-indicator"`, `id="btn-back"`, `id="btn-skip-step"`, `id="btn-next"`, `id="btn-skip-setup"`,
			`id="plan-select"`, `id="plan-quick"`, `id="llm-provider"`, `id="llm-model"`, `openOpenRouterBrowser`,
			`id="success-screen"`, `id="btn-go-to-chat"`, `/js/setup/main.js`,
		}},
		{template: "404.html", hooks: []string{
			`id="main-content"`, `href="/"`, `href="/dashboard"`, `data-i18n="notfound.title"`,
			`data-i18n="notfound.description"`, `data-i18n="notfound.go_home"`, `data-i18n="notfound.go_dashboard"`,
		}},
	}

	for _, test := range tests {
		html := normalizeAssetText(mustReadUIFile(t, test.template))
		if regexp.MustCompile(`(?i)\sstyle\s*=`).MatchString(html) {
			t.Errorf("%s must not contain style attributes", test.template)
		}
		if regexp.MustCompile(`(?i)<style\b`).MatchString(html) {
			t.Errorf("%s must not contain style elements", test.template)
		}
		for _, hook := range test.hooks {
			if !strings.Contains(html, hook) {
				t.Errorf("%s lost functional hook %q", test.template, hook)
			}
		}
	}

	setup := normalizeAssetText(mustReadUIFile(t, "setup.html"))
	if !strings.Contains(setup, `class="spinner profile-loading-spinner"`) {
		t.Error("setup loading spinner must use the scoped semantic spacing class")
	}
}

func TestPrecisionEntryStylesAreScopedResponsiveAndFlat(t *testing.T) {
	t.Parallel()

	shared := normalizeAssetText(mustReadUIFile(t, "css/precision-entry.css"))
	sharedPrefix := `.pw-page.pw-entry-page`
	for _, marker := range []string{
		sharedPrefix + ` {`, `min-height: 100dvh;`, `overflow-x: clip;`, `background-image: none;`,
		sharedPrefix + `:where(:root[data-theme="light"] *)`, `min-height: 44px;`, `min-width: 44px;`,
		`@media (max-width: 1024px)`, `@media (max-width: 768px)`, `@media (max-width: 640px)`,
		`@media (prefers-reduced-motion: reduce)`, `outline: 2px solid var(--pw-accent);`,
	} {
		if !strings.Contains(shared, marker) {
			t.Errorf("precision-entry.css missing %q", marker)
		}
	}
	if strings.Contains(strings.ToLower(shared), "gradient(") || strings.Contains(strings.ToLower(shared), "glow") {
		t.Error("precision-entry.css must not introduce gradients or glow")
	}
	assertPrecisionAdapterSelectorsScoped(t, shared, sharedPrefix)

	tests := []struct {
		name       string
		stylesheet string
		page       string
		selectors  []string
	}{
		{name: "Login", stylesheet: "css/login.css", page: "login", selectors: []string{`.login-card`, `.login-input`, `.btn-login`, `#bg-canvas`, `.css-fallback-bg`}},
		{name: "Setup", stylesheet: "css/setup.css", page: "setup", selectors: []string{`.setup-header`, `.setup-card`, `.setup-section`, `.btn-setup`, `.profile-loading-spinner`, `.or-browser-modal`}},
		{name: "NotFound", stylesheet: "css/not-found.css", page: "not-found", selectors: []string{`.not-found-page`, `.not-found-code`, `.not-found-actions`, `.not-found-link`, `.not-found-logo`}},
	}
	for _, test := range tests {
		css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
		startMarker := `/* === Precision Entry ` + test.name + ` Adapter: start === */`
		endMarker := `/* === Precision Entry ` + test.name + ` Adapter: end === */`
		start, end := strings.Index(css, startMarker), strings.Index(css, endMarker)
		if start < 0 || end <= start {
			t.Fatalf("%s missing delimited Precision entry adapter", test.stylesheet)
		}
		adapter := css[start:end]
		prefix := `.pw-page.pw-entry-page[data-entry-page="` + test.page + `"]`
		for _, marker := range append([]string{
			prefix + ` {`, `background-image: none;`, `box-shadow: none;`, `overflow-wrap: anywhere;`,
			prefix + `:where(:root[data-theme="light"] *)`, `@media (max-width: 768px)`, `@media (max-width: 640px)`,
			`min-height: 44px;`, `@media (prefers-reduced-motion: reduce)`,
		}, test.selectors...) {
			if !strings.Contains(adapter, marker) {
				t.Errorf("%s entry adapter missing %q", test.name, marker)
			}
		}
		if strings.Contains(strings.ToLower(adapter), "gradient(") || strings.Contains(strings.ToLower(adapter), "glow") {
			t.Errorf("%s entry adapter must remain flat", test.name)
		}
		assertPrecisionAdapterSelectorsScoped(t, adapter, prefix)
	}
}

func TestPrecisionEntrySetupOpenRouterDialogContract(t *testing.T) {
	t.Parallel()

	script := normalizeAssetText(mustReadUIFile(t, "js/setup/main.js"))
	start := strings.Index(script, `async function openOpenRouterBrowser(onSelect) {`)
	if start < 0 {
		t.Fatal("cannot locate openOpenRouterBrowser source block")
	}
	end := strings.Index(script[start:], `// ── Provider Change Handler`)
	if end < 0 {
		t.Fatal("cannot locate openOpenRouterBrowser source block")
	}
	block := script[start : start+end]
	for _, marker := range []string{
		`role="dialog"`, `aria-modal="true"`, `aria-labelledby="or-browser-title"`, `id="or-browser-title"`,
		`aria-label="${escapeAttr(t('common.close'))}"`, `const previouslyFocused = document.activeElement;`,
		`event.key !== 'Tab'`, `event.key === 'Escape'`, `focusable[0].focus();`,
		`focusable[focusable.length - 1].focus();`, `previouslyFocused.focus();`,
	} {
		if !strings.Contains(block, marker) {
			t.Errorf("OpenRouter dialog contract missing %q", marker)
		}
	}
	if regexp.MustCompile(`(?i)\sstyle\s*=`).MatchString(block) {
		t.Error("OpenRouter dialog markup must not create inline styles")
	}
}

func TestPrecisionEntryNotFoundExternalStylesPreserveActionSpacing(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/not-found.css"))
	selector := `.pw-page.pw-entry-page[data-entry-page="not-found"] .not-found-link`
	rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(selector) + `\s*\{([^}]*)\}`).FindStringSubmatch(css)
	if len(rule) != 2 || !strings.Contains(rule[1], `gap: var(--pw-space-2);`) {
		t.Error("external 404 link styles must preserve the original icon/text spacing")
	}
}

func TestPrecisionEntrySetupOverridesWinLegacyDecorativeEffects(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/setup.css"))
	start := strings.Index(css, `/* === Precision Entry Setup Adapter: start === */`)
	end := strings.Index(css, `/* === Precision Entry Setup Adapter: end === */`)
	if start < 0 || end <= start {
		t.Fatal("setup Precision Entry adapter is missing")
	}
	adapter := css[start:end]
	prefix := `.pw-page.pw-entry-page[data-entry-page="setup"]`
	tests := []struct {
		selector     string
		declarations []string
	}{
		{selector: prefix + ` .setup-card::before`, declarations: []string{`background: none;`, `background-image: none;`, `box-shadow: none;`}},
		{selector: prefix + ` .profile-card::before`, declarations: []string{`background: none;`, `background-image: none;`, `box-shadow: none;`}},
		{selector: prefix + ` .profile-recommended-bubble`, declarations: []string{`background: var(--pw-accent);`, `background-image: none;`, `box-shadow: none;`, `text-shadow: none;`}},
		{selector: prefix + ` .https-warning`, declarations: []string{`background-image: none;`, `box-shadow: none;`, `filter: none;`}},
		{selector: prefix + ` #step-indicator .step-dot.active`, declarations: []string{`background-image: none;`, `box-shadow: none;`, `text-shadow: none;`, `filter: none;`}},
	}
	for _, test := range tests {
		rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(test.selector) + `\s*\{([^}]*)\}`).FindStringSubmatch(adapter)
		if len(rule) != 2 {
			t.Errorf("Setup adapter missing cascade-safe rule %s", test.selector)
			continue
		}
		for _, declaration := range test.declarations {
			if !strings.Contains(rule[1], declaration) {
				t.Errorf("Setup rule %s missing %q", test.selector, declaration)
			}
		}
	}
}

func TestPrecisionEntrySetupDynamicStatesAvoidInlineStyles(t *testing.T) {
	t.Parallel()

	script := normalizeAssetText(mustReadUIFile(t, "js/setup/main.js"))
	blocks := []struct {
		name, start, end string
		markers          []string
	}{
		{name: "loadProfiles", start: `async function loadProfiles() {`, end: `function getFeatureBadges`, markers: []string{`class="profile-state-message profile-load-error"`}},
		{name: "renderProfileCards", start: `function renderProfileCards(list) {`, end: `function isMiniMaxQuickProfile`, markers: []string{`class="profile-state-message profile-empty-state"`}},
		{name: "renderStepIndicator", start: `function renderStepIndicator() {`, end: `function updateNextButtonState`, markers: []string{`<button type="button"`, `aria-label="${escapeAttr(label)}"`, `onclick="goToStep(${i})"`, `aria-current="step"`}},
	}
	for _, test := range blocks {
		start := strings.Index(script, test.start)
		if start < 0 {
			t.Fatalf("cannot locate %s", test.name)
		}
		end := strings.Index(script[start:], test.end)
		if end < 0 {
			t.Fatalf("cannot find end of %s", test.name)
		}
		block := script[start : start+end]
		if regexp.MustCompile(`(?i)\sstyle\s*=`).MatchString(block) {
			t.Errorf("%s must not render style attributes", test.name)
		}
		for _, marker := range test.markers {
			if !strings.Contains(block, marker) {
				t.Errorf("%s missing semantic renderer marker %q", test.name, marker)
			}
		}
	}
	if regexp.MustCompile(`(?i)\sstyle\s*=`).MatchString(script) {
		t.Error("Setup JavaScript must not generate style attributes")
	}
}

func TestPrecisionEntrySetupStepButtonsKeepNativeKeyboardAndTargetContract(t *testing.T) {
	t.Parallel()

	script := normalizeAssetText(mustReadUIFile(t, "js/setup/main.js"))
	start := strings.Index(script, `function renderStepIndicator() {`)
	if start < 0 {
		t.Fatal("cannot locate renderStepIndicator")
	}
	end := strings.Index(script[start:], `function updateNextButtonState`)
	if end < 0 {
		t.Fatal("cannot locate renderStepIndicator")
	}
	block := script[start : start+end]
	for _, forbidden := range []string{`role="button"`, `tabindex="0"`, `onkeydown=`} {
		if strings.Contains(block, forbidden) {
			t.Errorf("step renderer must rely on native button keyboard behavior, found %q", forbidden)
		}
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/setup.css"))
	adapterStart := strings.Index(css, `/* === Precision Entry Setup Adapter: start === */`)
	adapterEnd := strings.Index(css, `/* === Precision Entry Setup Adapter: end === */`)
	if adapterStart < 0 || adapterEnd <= adapterStart {
		t.Fatal("Setup adapter is missing")
	}
	adapter := css[adapterStart:adapterEnd]
	prefix := `.pw-page.pw-entry-page[data-entry-page="setup"] #step-indicator .step-dot`
	rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(prefix) + `\s*\{([^}]*)\}`).FindStringSubmatch(adapter)
	if len(rule) != 2 {
		t.Fatalf("Setup adapter missing desktop step target rule")
	}
	for _, declaration := range []string{`width: 44px;`, `height: 44px;`, `min-width: 44px;`, `min-height: 44px;`, `appearance: none;`} {
		if !strings.Contains(rule[1], declaration) {
			t.Errorf("desktop step target missing %q", declaration)
		}
	}
	mobileAt := strings.LastIndex(adapter, `@media (max-width: 640px)`)
	if mobileAt < 0 {
		t.Fatal("Setup adapter missing ordered mobile/reduced-motion blocks")
	}
	reducedAt := strings.Index(adapter[mobileAt:], `@media (prefers-reduced-motion: reduce)`)
	if reducedAt < 0 {
		t.Fatal("Setup adapter missing ordered mobile/reduced-motion blocks")
	}
	mobile := adapter[mobileAt : mobileAt+reducedAt]
	mobileRule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(prefix) + `\s*\{([^}]*)\}`).FindStringSubmatch(mobile)
	if len(mobileRule) != 2 || !strings.Contains(mobileRule[1], `width: 44px;`) || !strings.Contains(mobileRule[1], `height: 44px;`) {
		t.Error("mobile step buttons must retain a 44x44 target")
	}
}

func TestPrecisionEntrySetupNarrowHeaderRemovesRadialClearance(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/setup.css"))
	start := strings.Index(css, `/* === Precision Entry Setup Adapter: start === */`)
	end := strings.Index(css, `/* === Precision Entry Setup Adapter: end === */`)
	if start < 0 || end <= start {
		t.Fatal("Setup adapter is missing")
	}
	adapter := css[start:end]
	mediaAt := strings.Index(adapter, `@media (max-width: 1100px)`)
	if mediaAt < 0 {
		t.Fatal("Setup adapter missing narrow-header block before tablet rules")
	}
	mediaEnd := strings.Index(adapter[mediaAt:], `@media (max-width: 768px)`)
	if mediaEnd < 0 {
		t.Fatal("Setup adapter missing narrow-header block before tablet rules")
	}
	narrow := adapter[mediaAt : mediaAt+mediaEnd]
	prefix := `.pw-page.pw-entry-page[data-entry-page="setup"] .cfg-header .header-actions`
	for _, test := range []struct {
		selector     string
		declarations []string
	}{
		{selector: prefix, declarations: []string{`padding-right: 0;`}},
		{selector: prefix + `::after`, declarations: []string{`content: none;`, `display: none;`, `min-width: 0;`}},
	} {
		rule := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(test.selector) + `\s*\{([^}]*)\}`).FindStringSubmatch(narrow)
		if len(rule) != 2 {
			t.Errorf("narrow Setup header missing %s", test.selector)
			continue
		}
		for _, declaration := range test.declarations {
			if !strings.Contains(rule[1], declaration) {
				t.Errorf("narrow Setup header %s missing %q", test.selector, declaration)
			}
		}
	}
}

func TestPrecisionChangedPageAssetsUseReleaseBuildVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		template string
		assets   []string
	}{
		{template: "config.html", assets: []string{`/css/precision-workspace.css?v={{.BuildVersion}}`, `/css/precision-pages.css?v={{.BuildVersion}}`, `/js/precision/workspace.js?v={{.BuildVersion}}`, `/js/config/main.js?v={{.BuildVersion}}`}},
		{template: "dashboard.html", assets: []string{`/css/dashboard.css?v={{.BuildVersion}}`, `/css/precision-workspace.css?v={{.BuildVersion}}`, `/css/precision-pages.css?v={{.BuildVersion}}`, `/js/precision/workspace.js?v={{.BuildVersion}}`}},
		{template: "plans.html", assets: []string{`/css/plans.css?v={{.BuildVersion}}`, `/css/precision-workspace.css?v={{.BuildVersion}}`, `/css/precision-pages.css?v={{.BuildVersion}}`, `/js/precision/workspace.js?v={{.BuildVersion}}`}},
		{template: "missions_v2.html", assets: []string{`/css/missions.css?v={{.BuildVersion}}`, `/css/precision-workspace.css?v={{.BuildVersion}}`, `/css/precision-pages.css?v={{.BuildVersion}}`, `/js/precision/workspace.js?v={{.BuildVersion}}`}},
		{template: "cheatsheets.html", assets: []string{`/css/cheatsheets.css?v={{.BuildVersion}}`, `/css/precision-workspace.css?v={{.BuildVersion}}`, `/css/precision-pages.css?v={{.BuildVersion}}`, `/js/precision/workspace.js?v={{.BuildVersion}}`}},
		{template: "knowledge.html", assets: []string{`/css/knowledge.css?v={{.BuildVersion}}`, `/css/precision-workspace.css?v={{.BuildVersion}}`, `/css/precision-pages.css?v={{.BuildVersion}}`, `/js/precision/workspace.js?v={{.BuildVersion}}`, `/js/knowledge/main.js?v={{.BuildVersion}}`}},
		{template: "skills.html", assets: []string{`/css/skills.css?v={{.BuildVersion}}`, `/css/precision-workspace.css?v={{.BuildVersion}}`, `/css/precision-pages.css?v={{.BuildVersion}}`, `/js/precision/workspace.js?v={{.BuildVersion}}`, `/js/skills/main.js?v={{.BuildVersion}}`}},
		{template: "containers.html", assets: []string{`/css/containers.css?v={{.BuildVersion}}`, `/css/precision-workspace.css?v={{.BuildVersion}}`, `/css/precision-pages.css?v={{.BuildVersion}}`, `/js/precision/workspace.js?v={{.BuildVersion}}`, `/js/containers/main.js?v={{.BuildVersion}}`}},
		{template: "media.html", assets: []string{`/css/media.css?v={{.BuildVersion}}`, `/css/precision-workspace.css?v={{.BuildVersion}}`, `/css/precision-pages.css?v={{.BuildVersion}}`, `/js/precision/workspace.js?v={{.BuildVersion}}`}},
		{template: "truenas.html", assets: []string{`/css/truenas.css?v={{.BuildVersion}}`, `/css/precision-workspace.css?v={{.BuildVersion}}`, `/css/precision-pages.css?v={{.BuildVersion}}`, `/js/precision/workspace.js?v={{.BuildVersion}}`, `/js/truenas.js?v={{.BuildVersion}}`}},
		{template: "invasion_control.html", assets: []string{`/css/invasion.css?v={{.BuildVersion}}`, `/css/precision-workspace.css?v={{.BuildVersion}}`, `/css/precision-pages.css?v={{.BuildVersion}}`, `/js/precision/workspace.js?v={{.BuildVersion}}`}},
		{template: "login.html", assets: []string{`/css/login.css?v={{.BuildVersion}}`, `/css/precision-workspace.css?v={{.BuildVersion}}`, `/css/precision-entry.css?v={{.BuildVersion}}`}},
		{template: "setup.html", assets: []string{`/css/setup.css?v={{.BuildVersion}}`, `/css/precision-workspace.css?v={{.BuildVersion}}`, `/css/precision-entry.css?v={{.BuildVersion}}`, `/js/setup/main.js?v={{.BuildVersion}}`}},
		{template: "404.html", assets: []string{`/css/not-found.css?v={{.BuildVersion}}`, `/css/precision-workspace.css?v={{.BuildVersion}}`, `/css/precision-entry.css?v={{.BuildVersion}}`}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.template, func(t *testing.T) {
			t.Parallel()
			html := normalizeAssetText(mustReadUIFile(t, test.template))
			for _, asset := range test.assets {
				assetRef := regexp.MustCompile(`(?:href|src)=["']` + regexp.QuoteMeta(asset) + `["']`)
				if !assetRef.MatchString(html) {
					t.Errorf("%s must load changed asset with release BuildVersion: %s", test.template, asset)
				}
			}
		})
	}
}

func TestPrecisionOperationalStylesAreIntegratedAndPageScoped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		stylesheet string
		page       string
	}{
		{name: "Dashboard", stylesheet: "css/dashboard.css", page: "dashboard"},
		{name: "Plans", stylesheet: "css/plans.css", page: "plans"},
		{name: "Missions", stylesheet: "css/missions.css", page: "missions"},
		{name: "Cheatsheets", stylesheet: "css/cheatsheets.css", page: "cheatsheets"},
		{name: "Knowledge", stylesheet: "css/knowledge.css", page: "knowledge"},
		{name: "Skills", stylesheet: "css/skills.css", page: "skills"},
		{name: "Containers", stylesheet: "css/containers.css", page: "containers"},
		{name: "Media", stylesheet: "css/media.css", page: "media"},
		{name: "TrueNAS", stylesheet: "css/truenas.css", page: "truenas"},
		{name: "Invasion", stylesheet: "css/invasion.css", page: "invasion"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			css := normalizeAssetText(mustReadUIFile(t, test.stylesheet))
			for _, forbidden := range []string{
				`Precision Workspace page foundation and component rules`,
				`Premium Redesign`,
				`glassmorphism`,
				`gradient(`,
				`var(--bg-glass`,
				`var(--card-bg`,
				`var(--input-bg`,
			} {
				if strings.Contains(css, forbidden) {
					t.Errorf("%s retains a separate Precision/legacy layer marker %q", test.stylesheet, forbidden)
				}
			}
			if strings.Contains(css, `Precision Workspace `+test.name+` Adapter`) {
				t.Errorf("%s must integrate Precision declarations instead of retaining an adapter block", test.stylesheet)
			}
			prefix := `.pw-page[data-workspace-page="` + test.page + `"]`
			if !strings.Contains(css, prefix+` {`) {
				t.Errorf("%s must retain its integrated page-scoped Precision root %q", test.stylesheet, prefix)
			}
			assertOperationalStylesheetFullyScoped(t, css, prefix)
		})
	}
}

func assertOperationalStylesheetFullyScoped(t *testing.T, css, prefix string) {
	t.Helper()

	comments := regexp.MustCompile(`(?s)/\*.*?\*/`)
	css = comments.ReplaceAllString(css, "")
	keyframeStack := make([]bool, 0, 8)
	segmentStart := 0
	for index, char := range css {
		switch char {
		case '{':
			header := strings.TrimSpace(css[segmentStart:index])
			insideKeyframes := len(keyframeStack) > 0 && keyframeStack[len(keyframeStack)-1]
			lowerHeader := strings.ToLower(header)
			startsKeyframes := strings.HasPrefix(lowerHeader, "@keyframes") || strings.HasPrefix(lowerHeader, "@-webkit-keyframes")
			if header != "" && !strings.HasPrefix(header, "@") && !insideKeyframes {
				for _, selector := range strings.Split(header, ",") {
					selector = strings.TrimSpace(selector)
					if selector != "" && !strings.HasPrefix(selector, prefix) {
						t.Errorf("operational selector must be integrated under %q: %q", prefix, selector)
					}
				}
			}
			keyframeStack = append(keyframeStack, insideKeyframes || startsKeyframes)
			segmentStart = index + 1
		case '}':
			if len(keyframeStack) > 0 {
				keyframeStack = keyframeStack[:len(keyframeStack)-1]
			}
			segmentStart = index + 1
		case ';':
			segmentStart = index + 1
		}
	}
}

func TestPrecisionEntryLoginCSSFallbackRemainsAvailable(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/login.css"))
	prefix := `.pw-page.pw-entry-page[data-entry-page="login"]`
	for _, selector := range []string{`.css-fallback-bg`, `.particle-grid`, `.scan-line`, `.orb`} {
		pattern := regexp.MustCompile(`(?s)([^{}]*` + regexp.QuoteMeta(selector) + `[^{}]*)\{([^}]*)\}`)
		matches := pattern.FindAllStringSubmatch(css, -1)
		if len(matches) == 0 {
			t.Errorf("login.css missing expected CSS fallback selector %s", selector)
			continue
		}
		for _, match := range matches {
			if !strings.Contains(match[1], prefix) {
				continue
			}
			declarations := match[2]
			for _, forbidden := range []string{`display: none;`, `background: transparent;`, `background-image: none;`} {
				if strings.Contains(declarations, forbidden) {
					t.Errorf("Login CSS fallback selector %s must remain available; found %q", selector, forbidden)
				}
			}
		}
	}
	if !strings.Contains(css, `@media (prefers-reduced-motion: reduce)`) {
		t.Error("login.css must preserve reduced-motion behavior")
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
