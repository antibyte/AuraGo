# Pages and Chat Default in the Desktop Standard Look: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** All pages (404, cheatsheets, config, containers, dashboard, invasion_control, knowledge, login, media, missions_v2, plans, setup, skills, truenas) get the desktop Standard look in dark, with a derived light variant. In the chat, only the default theme changes.

**Architecture:** Colours and gradients live as tokens in `.pw-page` and `[data-theme="light"] .pw-page` in `ui/css/precision-workspace.css`. Component rules consume them as `var(--pw-mat-*)`. The chat default theme is one new bundle part, `ui/css/chat-standard.css`, placed last. Every selector in it carries the prefix `:root:is([data-theme="dark"], :not([data-theme]))`.

**Tech Stack:** CSS custom properties, `color-mix()`, Go static asset tests in `ui/`, Node bundler `scripts/build-ui-bundles.js`, go-rod for the screenshot round.

**Spec:** `docs/superpowers/specs/2026-11-04-pages-standard-theme-design.md`

**Rules for every task:**
- Work only in the dedicated worktree from Task 0. Use path-limited `git add <paths>` and `git commit`.
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Before each commit, run GitNexus `detect_changes({scope: "all"})`. The CLI fallback from the worktree root is `node ../../../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo .`.
- CSS has no symbols. `impact` applies only to the JS functions named in Task 5.
- Line anchors refer to `main` before any edit. Within one file, edit bottom-up so anchors stay valid.
- If a test fails outside the pins named in a task, stop and report it. Do not edit that test.
- `rg` may not be on PATH. If it is missing, run the same pattern with the Grep tool.
- The scripts under `disposable/` are never committed; only the paths named in each commit step are added.
- Lean test rule (user request): no mutation, property or stress suites and no repeated browser runs. Tasks 3 to 5 add no new tests. Their proof is the grep shown plus the existing suites.

**Agents:** implementation `maker-agent`, test runs `test-agent`, docs `docs-agent`.

## WCAG ratios, measured with node (no failures)

| Pair | Dark | Light |
| --- | --- | --- |
| text on canvas / surface | 16.00 / 14.29 | 14.57 / 15.62 |
| muted on canvas / surface | 8.97 / 8.01 | 7.02 / 7.53 |
| subtle on canvas / surface | 8.18 / 7.31 | 5.50 / 5.89 |
| accent on canvas / surface | 6.30 / 5.63 | 5.21 / 5.59 |
| accent on elevated / soft | 5.03 / 5.05 | 4.81 / 4.68 |
| on-accent on accent | 6.63 | 5.59 |
| on-accent on accent-strong | 7.58 | 7.31 |

If the test in Task 1 disagrees and a value fails, darken it (light) or lighten it (dark) until it reaches 4.5, then update the token table in the spec in the same commit.

---

## File structure

| File | Change |
| --- | --- |
| `ui/pages_standard_theme_test.go` | New: contrast, gradient-location and chat scope tests |
| `ui/precision_pages_test.go` | Update pins; rename one test; remove `.orb` from one list |
| `ui/config_precision_workspace_test.go` | Update three pins |
| `ui/css/precision-workspace.css` | Tokens, `--pw-mat-*`, aliases, fields, primary button, page background |
| `ui/css/precision-pages.css` | Component rules consume `--pw-mat-*`; `--pw-status-*` values |
| `ui/css/dashboard.css`, `ui/css/dashboard-shell.css` | Card, sidebar and selected navigation |
| `ui/css/config-workspace.css`, `ui/css/config.css` | Remove the palette override; colour mapping; surfaces |
| `ui/css/login.css`, `ui/login.html`, `ui/js/login/main.js`, `ui/js/login/flame-wrap.js` | Remove aurora and orbs; recolour |
| `ui/css/setup.css`, `ui/css/not-found.css` | Colour mapping; surfaces |
| `ui/testdata/wave4_colours.txt` | Refreshed, counts go down only |
| `ui/css/chat-standard.css` | New bundle part |
| `scripts/build-ui-bundles.js`, `ui/css/chat.bundle.css` | Add the part; regenerate |
| `ui/AGENTS.md` | Document the contracts |

Frozen, must show no diff: `ui/shared-variables.css`, `ui/css/tokens.css`, `ui/shared-components.css`, `ui/css/enhancements.css`, `ui/css/chat.css`, `ui/css/chat-modules.css`, `ui/css/chat-themes.css`.

---

### Task 0: Dedicated worktree

Agent: `maker-agent`.

**Files:** none tracked.

- [ ] **Step 1: Create the worktree from `main`**

```powershell
git -C C:\Users\Andi\Documents\repo\AuraGo worktree add .worktrees/pages-standard-theme -b claude/pages-standard-theme main
```

All later commands run from `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\pages-standard-theme`.

- [ ] **Step 2: Confirm a clean baseline**

```powershell
go test ./ui/ -run 'Precision|Wave4'
npm run check:ui
```

Expected: both pass. If `check:ui` reports drift in any bundle, stop and report it, because later steps rely on a clean baseline.

---

### Task 1: Dark and light tokens, pins, contrast test

Implementation: `maker-agent`. Test runs: `test-agent`.

**Files:**
- Create: `ui/pages_standard_theme_test.go`
- Modify: `ui/precision_pages_test.go:17,70-97`
- Modify: `ui/config_precision_workspace_test.go:53-55`
- Modify: `ui/css/precision-workspace.css:3-20,34-37,42,154-172`
- Modify: `ui/css/config-workspace.css:4-18,42,58-69`

- [ ] **Step 1: Write the contrast test**

Create `ui/pages_standard_theme_test.go`. It reuses `parseCSSHexColor` and `cssContrastRatio` from `ui/desktop_theme_css_contract_test.go`.

```go
package ui

import (
	"regexp"
	"strings"
	"testing"
)

const (
	standardThemeDarkHeader  = ".pw-page {\n"
	standardThemeLightHeader = "[data-theme=\"light\"] .pw-page,\n.pw-page[data-theme=\"light\"] {\n"
)

var standardThemeHexToken = regexp.MustCompile(`(--pw-[a-z-]+):\s*(#[0-9a-fA-F]{6});`)

func standardThemeBlock(t *testing.T, css, header string) string {
	t.Helper()
	start := strings.Index(css, header)
	if start < 0 {
		t.Fatalf("token block %q not found", header)
	}
	end := strings.Index(css[start:], "\n}")
	if end < 0 {
		t.Fatalf("token block %q is not closed", header)
	}
	return css[start : start+end]
}

func TestPagesStandardThemeTokenContrast(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/precision-workspace.css"))
	for mode, header := range map[string]string{"dark": standardThemeDarkHeader, "light": standardThemeLightHeader} {
		tokens := map[string][3]float64{}
		for _, match := range standardThemeHexToken.FindAllStringSubmatch(standardThemeBlock(t, css, header), -1) {
			rgb, ok := parseCSSHexColor(match[2])
			if !ok {
				t.Fatalf("%s: cannot parse %s %s", mode, match[1], match[2])
			}
			tokens[match[1]] = rgb
		}
		check := func(fg, bg string) {
			foreground, okFg := tokens[fg]
			background, okBg := tokens[bg]
			if !okFg || !okBg {
				t.Errorf("%s: %s and %s must both be six-digit hex tokens", mode, fg, bg)
				return
			}
			if ratio := cssContrastRatio(foreground, background); ratio < 4.5 {
				t.Errorf("%s: %s on %s = %.2f:1, want at least 4.5:1", mode, fg, bg, ratio)
			}
		}
		for _, fg := range []string{"--pw-text", "--pw-muted", "--pw-subtle", "--pw-accent"} {
			for _, bg := range []string{"--pw-canvas", "--pw-surface"} {
				check(fg, bg)
			}
		}
		check("--pw-on-accent", "--pw-accent")
		check("--pw-on-accent", "--pw-accent-strong")
	}

	// Config once carried its own palette; the ratios above only hold if it inherits.
	config := normalizeAssetText(mustReadUIFile(t, "css/config-workspace.css"))
	for _, token := range []string{"--pw-canvas:", "--pw-surface:", "--pw-text:", "--pw-accent:", "--pw-line:", "--pw-radius-lg:"} {
		if strings.Contains(config, token) {
			t.Errorf("config-workspace.css must inherit %s from precision-workspace.css", token)
		}
	}
}
```

- [ ] **Step 2: Update the pins**

In `ui/precision_pages_test.go`, line 17:

```go
		`--pw-accent: #ed764d;`,
```

Replace lines 70-97. The function name changes through this ordinary edit, and no other reference to it exists under `ui/`. Lines 98-126 stay until Task 2.

```go
func TestPrecisionWorkspaceStandardPaletteTokens(t *testing.T) {
	t.Parallel()

	foundation := normalizeAssetText(mustReadUIFile(t, "css/precision-workspace.css"))
	for _, token := range []string{
		`--pw-canvas: #12161d;`, `--pw-surface: #1a212b;`,
		`--pw-surface-elevated: #222a35;`, `--pw-surface-soft: #1f2a38;`,
		`--pw-text: #edf1f7;`, `--pw-muted: #aeb7c6;`,
		`--pw-subtle: #a4afc0;`, `--pw-accent: #ed764d;`,
		`--pw-accent-strong: #f08763;`, `--pw-on-accent: #0c1016;`,
		`--pw-line: rgba(191, 210, 235, 0.15);`,
		`--pw-line-strong: rgba(191, 210, 235, 0.28);`,
	} {
		if !strings.Contains(foundation, token) {
			t.Errorf("missing dark Standard token %q", token)
		}
	}
	for _, token := range []string{
		`--pw-canvas: #f6f7f9;`, `--pw-surface: #ffffff;`,
		`--pw-surface-elevated: #eceef2;`, `--pw-surface-soft: #e8ebf0;`,
		`--pw-text: #1c2430;`, `--pw-muted: #4a5568;`,
		`--pw-subtle: #5a6577;`, `--pw-accent: #b93e0b;`,
		`--pw-accent-strong: #9a3412;`,
		`--pw-line: rgba(40, 52, 76, 0.14);`,
		`--pw-line-strong: rgba(40, 52, 76, 0.38);`,
	} {
		if !strings.Contains(foundation, token) {
			t.Errorf("missing light Standard token %q", token)
		}
	}
```

In `ui/config_precision_workspace_test.go`, lines 53-55:

```go
		`--pw-canvas: #12161d;`,
		`--pw-surface: #1a212b;`,
		`--pw-accent: #ed764d;`,
```

- [ ] **Step 3: Run and confirm failure**

```powershell
go test ./ui/ -run 'TestPagesStandardThemeTokenContrast|TestPrecisionWorkspaceStandardPaletteTokens|TestPrecisionWorkspaceFoundationComponentsAreScoped|TestConfigPrecisionWorkspaceTypographyAndDensityContract'
```

Expected: FAIL on missing Standard tokens and on config-workspace still defining the palette.

- [ ] **Step 4: Change the dark tokens**

`ui/css/precision-workspace.css`, lines 3-20:

```css
    --pw-canvas: #12161d;
    --pw-surface: #1a212b;
    --pw-surface-elevated: #222a35;
    --pw-surface-soft: #1f2a38;
    --pw-text: #edf1f7;
    --pw-muted: #aeb7c6;
    --pw-subtle: #a4afc0;
    --pw-accent: #ed764d;
    --pw-accent-strong: #f08763;
    --pw-danger: #ff7a72;
    --pw-warning: #f2b84b;
    --pw-success: #4ad29c;
    --pw-on-accent: #0c1016;
    --pw-scrim: color-mix(in srgb, #04070c 62%, transparent);
    --pw-terminal-bg: #050807;
    --pw-terminal-text: #d7e1ec;
    --pw-line: rgba(191, 210, 235, 0.15);
    --pw-line-strong: rgba(191, 210, 235, 0.28);
```

Lines 34-37:

```css
    --pw-radius-sm: 8px;
    --pw-radius-md: 10px;
    --pw-radius-lg: 14px;
    --pw-radius-xl: 14px;
```

Line 42:

```css
    --pw-shadow-soft: 0 22px 60px rgba(0, 0, 0, 0.35);
```

- [ ] **Step 5: Change the light tokens**

Lines 154-172:

```css
    --pw-canvas: #f6f7f9;
    --pw-surface: #ffffff;
    --pw-surface-elevated: #eceef2;
    --pw-surface-soft: #e8ebf0;
    --pw-text: #1c2430;
    --pw-muted: #4a5568;
    --pw-subtle: #5a6577;
    --pw-accent: #b93e0b;
    --pw-accent-strong: #9a3412;
    --pw-line: rgba(40, 52, 76, 0.14);
    --pw-line-strong: rgba(40, 52, 76, 0.38);
    --pw-danger: #be123c;
    --pw-warning: #a16207;
    --pw-success: #15803d;
    --pw-on-accent: #ffffff;
    --pw-scrim: color-mix(in srgb, #1c2430 45%, transparent);
    --pw-terminal-bg: #050807;
    --pw-terminal-text: #d7e1ec;
    --pw-shadow-soft: 0 24px 62px rgba(40, 52, 76, 0.13);
```

- [ ] **Step 6: Make config inherit**

In `ui/css/config-workspace.css`:
- Delete lines 58-69, keeping lines 70-72 inside the light rule.
- Line 42 becomes `--cfg-on-accent: var(--pw-on-accent);`
- Delete lines 4-18.

Without this, config keeps the old blue palette and the old radii.

- [ ] **Step 7: Run and confirm pass**

```powershell
go test ./ui/ -run 'TestPagesStandardThemeTokenContrast|TestPrecisionWorkspaceStandardPaletteTokens|TestPrecisionWorkspaceFoundationComponentsAreScoped|TestConfigPrecisionWorkspaceTypographyAndDensityContract|TestUIWave4EveryPwColourTokenHasLightOverride|TestConfigProviderActionsUseReadableAccentContrast'
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add ui/pages_standard_theme_test.go ui/precision_pages_test.go ui/config_precision_workspace_test.go ui/css/precision-workspace.css ui/css/config-workspace.css
git commit -m "feat(ui): Standard palette tokens for pages" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: `--pw-mat-*` tokens, component surfaces, status tokens

Implementation: `maker-agent`. Test runs: `test-agent`.

**How the gradient conflict resolves.**
- Gradient guards stay byte-identical: `TestPrecisionWorkspaceFoundationComponentsAreScoped`, `TestWave3FoundationCSS`, `TestPrecisionEntryStylesAreScopedResponsiveAndFlat`, `TestPrecisionOperationalStylesAreIntegratedAndPageScoped` and `assertPrecisionIntegratedRulesFlat`. They search for the literal `gradient(`, so `background: var(--pw-mat-card)` passes.
- The `...StaysFlat` and `...NeutralizesResidualGlows` tests pin toasts, status decorations and glows, none of which change.
- Gradient literals appear only on `--pw-mat-*` lines of `precision-workspace.css`, enforced by the new test below.
- The only foundation test changes are the alias pins and the primary text colour in `TestPrecisionWorkspaceStandardPaletteTokens`. Standard's field and bar are not derived from the old mixes, and the spec's ink is `--pw-on-accent`.

**Files:**
- Modify: `ui/pages_standard_theme_test.go`
- Modify: `ui/precision_pages_test.go` (alias list and primary check in `TestPrecisionWorkspaceStandardPaletteTokens`)
- Modify: `ui/css/precision-workspace.css`
- Modify: `ui/css/precision-pages.css`
- Modify: `ui/css/dashboard.css:24-29,91-106`
- Modify: `ui/css/dashboard-shell.css:24-25,78-81,88-93`

- [ ] **Step 1: Add the gradient-location test**

Append to `ui/pages_standard_theme_test.go`:

```go
func TestPagesStandardThemeGradientsLiveInMaterialTokens(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/precision-workspace.css"))
	for number, line := range strings.Split(css, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "gradient(") && !strings.HasPrefix(trimmed, "--pw-mat-") {
			t.Errorf("precision-workspace.css:%d: gradients belong in --pw-mat-* token values: %s", number+1, trimmed)
		}
	}
	dark := standardThemeBlock(t, css, standardThemeDarkHeader)
	light := standardThemeBlock(t, css, standardThemeLightHeader)
	for _, name := range []string{"bar", "side", "card", "well", "field", "btn", "primary", "selected", "hover", "divider"} {
		token := "--pw-mat-" + name + ":"
		if !strings.Contains(dark, token) {
			t.Errorf("dark block lacks %s", token)
		}
		if !strings.Contains(light, token) {
			t.Errorf("light block lacks %s", token)
		}
	}
}
```

- [ ] **Step 2: Update the alias pins**

In `TestPrecisionWorkspaceStandardPaletteTokens`, replace three alias lines:

```go
		`--header-bg: var(--pw-mat-bar-color);`,
		`--sidebar-bg: var(--pw-mat-side-color);`,
		`--input-bg: var(--pw-mat-field-color);`,
```

Replace the primary check:

```go
	if len(primaryButtonRule) != 2 || !strings.Contains(primaryButtonRule[1], `color: var(--pw-on-accent);`) {
		t.Error("Precision primary buttons must use the on-accent token for text")
	}
```

Keep the retired teal check unchanged.

- [ ] **Step 3: Run and confirm failure**

```powershell
go test ./ui/ -run 'TestPagesStandardThemeGradientsLiveInMaterialTokens|TestPrecisionWorkspaceStandardPaletteTokens'
```

Expected: FAIL (tokens and aliases missing).

- [ ] **Step 4: Add the dark material tokens**

In `ui/css/precision-workspace.css`, insert directly after `--pw-shadow-soft` in the first `.pw-page` block. Values are copied from `desktop-materials.css` line 64 on, resolved for accent `#E95420`.

```css
    --pw-mat-bar-color: #1a2029;
    --pw-mat-bar: linear-gradient(180deg, rgba(255, 255, 255, 0.05), rgba(255, 255, 255, 0) 70%), var(--pw-mat-bar-color);
    --pw-mat-bar-edge: inset 0 -1px 0 rgba(0, 0, 0, 0.35), inset 0 1px 0 rgba(255, 255, 255, 0.04);
    --pw-mat-side-color: #0e1218;
    --pw-mat-side: var(--pw-mat-side-color);
    --pw-mat-side-edge: inset -1px 0 0 rgba(255, 255, 255, 0.045), inset 1px 0 0 rgba(0, 0, 0, 0.3);
    --pw-mat-card: linear-gradient(180deg, rgba(255, 255, 255, 0.035), rgba(255, 255, 255, 0) 60%), var(--pw-surface);
    --pw-mat-card-border: rgba(191, 210, 235, 0.13);
    --pw-mat-card-shadow: 0 1px 2px rgba(0, 0, 0, 0.32), 0 8px 20px rgba(0, 0, 0, 0.2), inset 0 1px 0 rgba(255, 255, 255, 0.05);
    --pw-mat-well: #0b0e13;
    --pw-mat-well-shadow: inset 0 2px 5px rgba(0, 0, 0, 0.5), inset 0 0 0 1px rgba(0, 0, 0, 0.28);
    --pw-mat-field-color: #0c1016;
    --pw-mat-field: var(--pw-mat-field-color);
    --pw-mat-field-border: rgba(191, 210, 235, 0.17);
    --pw-mat-field-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.45);
    --pw-mat-btn: linear-gradient(180deg, rgba(255, 255, 255, 0.06), rgba(255, 255, 255, 0)), #222a35;
    --pw-mat-btn-hover: linear-gradient(180deg, rgba(255, 255, 255, 0.12), rgba(255, 255, 255, 0.03)), #222a35;
    --pw-mat-btn-border: rgba(191, 210, 235, 0.2);
    --pw-mat-btn-shadow: 0 1px 1px rgba(0, 0, 0, 0.3), inset 0 1px 0 rgba(255, 255, 255, 0.06);
    --pw-mat-primary: linear-gradient(180deg, #f08763, #ed764d), #ed764d;
    --pw-mat-primary-hover: linear-gradient(180deg, #f19575, #ef805a), #ed764d;
    --pw-mat-primary-border: #a33b16;
    --pw-mat-primary-shadow: 0 1px 2px rgba(0, 0, 0, 0.35), inset 0 1px 0 rgba(255, 255, 255, 0.22);
    --pw-mat-selected: linear-gradient(0deg, rgba(233, 84, 32, 0.24), rgba(233, 84, 32, 0.24)), #1f2a38;
    --pw-mat-selected-text: #f4f7fb;
    --pw-mat-hover: rgba(255, 255, 255, 0.07);
    --pw-mat-divider: rgba(191, 210, 235, 0.15);
```

- [ ] **Step 5: Add the light material tokens**

Insert after `--pw-shadow-soft` in the light block. Light primary gradients only darken from the accent, so white ink never drops below 5.59:1.

```css
    --pw-mat-bar-color: #f2f3f7;
    --pw-mat-bar: linear-gradient(180deg, #fbfbfd 0%, #eceef2 100%), var(--pw-mat-bar-color);
    --pw-mat-bar-edge: inset 0 1px 0 rgba(255, 255, 255, 0.9), inset 0 -1px 0 rgba(40, 52, 76, 0.14);
    --pw-mat-side-color: #e8ebf0;
    --pw-mat-side: linear-gradient(180deg, rgba(255, 255, 255, 0.4), rgba(255, 255, 255, 0) 55%), var(--pw-mat-side-color);
    --pw-mat-side-edge: inset -1px 0 0 rgba(40, 52, 76, 0.13);
    --pw-mat-card: var(--pw-surface);
    --pw-mat-card-border: rgba(40, 52, 76, 0.1);
    --pw-mat-card-shadow: 0 0 0 0.5px rgba(40, 52, 76, 0.06), 0 1px 2px rgba(40, 52, 76, 0.1), 0 8px 20px rgba(40, 52, 76, 0.08);
    --pw-mat-well: #eceef2;
    --pw-mat-well-shadow: inset 0 1px 3px rgba(40, 52, 76, 0.16), inset 0 0 0 1px rgba(40, 52, 76, 0.07);
    --pw-mat-field-color: #ffffff;
    --pw-mat-field: var(--pw-mat-field-color);
    --pw-mat-field-border: rgba(40, 52, 76, 0.24);
    --pw-mat-field-shadow: inset 0 1px 2px rgba(40, 52, 76, 0.12);
    --pw-mat-btn: linear-gradient(180deg, #ffffff, #f2f3f6), #f7f8fa;
    --pw-mat-btn-hover: linear-gradient(180deg, #ffffff, #e9ecf1), #f7f8fa;
    --pw-mat-btn-border: rgba(40, 52, 76, 0.22);
    --pw-mat-btn-shadow: 0 1px 1px rgba(40, 52, 76, 0.12), inset 0 1px 0 #ffffff;
    --pw-mat-primary: linear-gradient(180deg, #b93e0b, #a83809), #b93e0b;
    --pw-mat-primary-hover: linear-gradient(180deg, #9a3412, #8b2f10), #9a3412;
    --pw-mat-primary-border: #9a3412;
    --pw-mat-primary-shadow: 0 1px 2px rgba(20, 30, 50, 0.3), inset 0 1px 0 rgba(255, 255, 255, 0.18);
    --pw-mat-selected: #f8e8e2;
    --pw-mat-selected-text: #1c2430;
    --pw-mat-hover: rgba(194, 65, 12, 0.08);
    --pw-mat-divider: rgba(40, 52, 76, 0.14);
```

- [ ] **Step 6: Update the aliases in `.pw-page`**

Replace the `--header-bg`, `--sidebar-bg` and `--input-bg` lines, and add the status and modal aliases. These shared tokens otherwise keep their slate values on pages (for example `--modal-surface: #1e293b`, read by the radial menu).

```css
    --header-bg: var(--pw-mat-bar-color);
    --sidebar-bg: var(--pw-mat-side-color);
    --input-bg: var(--pw-mat-field-color);
    --success: var(--pw-success);
    --warning: var(--pw-warning);
    --danger: var(--pw-danger);
    --modal-bg: var(--pw-scrim);
    --modal-surface: var(--pw-surface);
    --modal-border: var(--pw-line-strong);
```

- [ ] **Step 7: Fields and primary button in `precision-workspace.css`**

Rule at lines 227-238:

```css
.pw-page .field-input,
.pw-page .field-select,
.pw-page .cfg-input {
    min-height: 46px;
    border: 1px solid var(--pw-mat-field-border);
    border-radius: var(--pw-radius-sm);
    background: var(--pw-mat-field);
    box-shadow: var(--pw-mat-field-shadow);
    transition:
        border-color var(--pw-duration-fast) var(--pw-ease),
        box-shadow var(--pw-duration-fast) var(--pw-ease),
        transform var(--pw-duration-fast) var(--pw-ease);
}
```

Rules at lines 256-278:

```css
.pw-page .btn-save,
.pw-page .btn-primary {
    min-height: 44px;
    border: 1px solid var(--pw-mat-primary-border);
    border-radius: var(--pw-radius-sm);
    padding: 0.65rem 1.15rem;
    font-weight: 650;
    background: var(--pw-mat-primary);
    color: var(--pw-on-accent);
    box-shadow: var(--pw-mat-primary-shadow);
    transition:
        transform var(--pw-duration-fast) var(--pw-ease),
        background var(--pw-duration-fast) var(--pw-ease),
        box-shadow var(--pw-duration-fast) var(--pw-ease);
}

.pw-page .btn-save:hover,
.pw-page .btn-primary:hover {
    background: var(--pw-mat-primary-hover);
    transform: none;
    box-shadow: var(--pw-mat-primary-shadow);
}
```

- [ ] **Step 8: Status tokens in `precision-pages.css`**

Keep these blocks last among the `.pw-page` blocks, because `TestWave3FoundationCSS` reads the last block. Lines 1432-1436:

```css
    --pw-status-ok: #4ad29c;
    --pw-status-warn: #f2b84b;
    --pw-status-error: #ff7a72;
    --pw-status-idle: #a4afc0;
    --pw-status-running: #f08763;
```

Lines 1444-1445:

```css
    --pw-status-idle: #5a6577;
    --pw-status-running: #9a3412;
```

- [ ] **Step 9: Component rules in `precision-pages.css`**

Replace only the named declarations. Leave the other declarations of each rule as they are.

| Anchor | Selector | New declarations |
| --- | --- | --- |
| 1364-1368 | `.pw-filter-pill[aria-pressed="true"]` | `border-color: color-mix(in srgb, var(--pw-accent) 32%, var(--pw-line)); background: var(--pw-mat-selected); color: var(--pw-mat-selected-text);` |
| 1000-1005 | `.view-toggle button.active` | `border-color: color-mix(in srgb, var(--pw-accent) 32%, var(--pw-line)); background: var(--pw-mat-selected); color: var(--pw-mat-selected-text);` |
| 968-977 | `.view-toggle` | `background: var(--pw-mat-well); box-shadow: var(--pw-mat-well-shadow);` |
| 820-834 | `.modal` | `background: var(--pw-mat-card);`, and delete the line `background-image: none;` |
| 812-818 | `.modal-overlay` | `background: var(--pw-scrim);` |
| 795-799 | `.card:hover` | `box-shadow: var(--pw-mat-card-shadow);` |
| 790-793 | `.card::before, .card::after` | add `display: none;` above `content: none;` (see Task 3) |
| 778-788 | `.card` | `border: 1px solid var(--pw-mat-card-border); background: var(--pw-mat-card); box-shadow: var(--pw-mat-card-shadow);` |
| 771-776 | `.tab.active` | `border-color: color-mix(in srgb, var(--pw-accent) 32%, var(--pw-line)); background: var(--pw-mat-selected); color: var(--pw-mat-selected-text);` |
| 766-769 | `.tab:hover:not(.active)` | `background: var(--pw-mat-hover);` |
| 723-735 | `.tabs` | `background: var(--pw-mat-well); box-shadow: var(--pw-mat-well-shadow);` |
| 712-715 | `.btn-success:hover` | `transform: none; box-shadow: none;` |
| 701-704 | `.btn-danger:hover` | `transform: none; box-shadow: none;` |
| 688-693 | `.btn-secondary:hover` | `background: var(--pw-mat-btn-hover); transform: none; box-shadow: var(--pw-mat-btn-shadow);` |
| 682-686 | `.btn-secondary` | `border-color: var(--pw-mat-btn-border); background: var(--pw-mat-btn); color: var(--pw-text); box-shadow: var(--pw-mat-btn-shadow);` |
| 656-669 | `.btn` | `border-radius: var(--pw-radius-sm);` |
| 613-619 | `.btn-theme` | `border-radius: var(--pw-radius-sm); background: var(--pw-mat-btn); box-shadow: var(--pw-mat-btn-shadow);` |
| 621-625 | `.btn-theme:hover` | `background: var(--pw-mat-btn-hover);` |
| 581-593 | `.app-header` | `border-bottom: 1px solid var(--pw-mat-divider); background: var(--pw-mat-bar); backdrop-filter: none; -webkit-backdrop-filter: none; box-shadow: var(--pw-mat-bar-edge);` |
| 481-489 | `.pw-modal` | `background: var(--pw-mat-card);` |
| 391-404 | `.pw-form-control` | `border: 1px solid var(--pw-mat-field-border); background: var(--pw-mat-field); box-shadow: var(--pw-mat-field-shadow);` |
| 362-364 | `.pw-table tbody tr:hover td` | `background: var(--pw-mat-hover);` |
| 271-278 | `.pw-panel, .pw-card` | `border: 1px solid var(--pw-mat-card-border); background: var(--pw-mat-card); box-shadow: var(--pw-mat-card-shadow);` |
| 202-207 | `.pw-tab[aria-selected="true"], .pw-tab.is-active` | `border-color: color-mix(in srgb, var(--pw-accent) 32%, var(--pw-line)); background: var(--pw-mat-selected); color: var(--pw-mat-selected-text);` |
| 198-200 | `.pw-tab:hover` | add `background: var(--pw-mat-hover);` |
| 162-173 | `.pw-tabs` | `background: var(--pw-mat-well); box-shadow: var(--pw-mat-well-shadow);` |
| 147-151 | `.pw-icon-btn:hover:not(:disabled)` | `background: var(--pw-mat-btn-hover);` |
| 125-145 | `.pw-icon-btn` | `border: 1px solid var(--pw-mat-btn-border); background: var(--pw-mat-btn); box-shadow: var(--pw-mat-btn-shadow);` |
| 96-112 | `.pw-search, input.pw-search` | `border: 1px solid var(--pw-mat-field-border); background: var(--pw-mat-field); box-shadow: var(--pw-mat-field-shadow);` |
| 2-15 | `.pw-app-header` | `border-bottom: 1px solid var(--pw-mat-divider); background: var(--pw-mat-bar); backdrop-filter: none; box-shadow: var(--pw-mat-bar-edge);` |

Then add this directly after the `.pw-page .btn` rule. `.btn` loads later than the foundation and at equal specificity, so it would otherwise reset the primary border and shadow.

```css
.pw-page .btn.btn-primary,
.pw-page .btn.btn-save {
    border-color: var(--pw-mat-primary-border);
    box-shadow: var(--pw-mat-primary-shadow);
}
```

Badges (`.pw-badge`) and filter pills keep `999px`. They are chips, not surfaces.

- [ ] **Step 10: Dashboard card and sidebar**

`ui/css/dashboard.css`, lines 24-29:

```css
.pw-page[data-workspace-page="dashboard"] .dash-card:hover {
    border-color: var(--pw-line-strong);
    background: var(--pw-mat-card);
    box-shadow: var(--pw-mat-card-shadow);
    transform: none;
}
```

Same file, the rule at lines 91-106: `border: 1px solid var(--pw-mat-card-border);`, `background: var(--pw-mat-card);`, `box-shadow: var(--pw-mat-card-shadow);`, and delete `background-image: none;` from this rule only. The markers `background-image: none;` and `box-shadow: none;` stay present elsewhere in the file.

`ui/css/dashboard-shell.css`:

```css
/* .dash-sidebar, lines 24-25 */
    border-right: 1px solid var(--pw-mat-divider);
    background: var(--pw-mat-side);
    box-shadow: var(--pw-mat-side-edge);

/* .dash-nav-link:hover, line 80 */
    background: var(--pw-mat-hover);

/* .dash-nav-link[aria-current="page"], lines 89-91 */
    color: var(--pw-mat-selected-text);
    border-color: color-mix(in srgb, var(--pw-accent) 32%, var(--pw-line));
    background: var(--pw-mat-selected);
```

- [ ] **Step 11: Run and confirm pass**

```powershell
go test ./ui/ -run 'Dashboard|Precision|Wave3Foundation|Wave4|StandardTheme'
```

Expected: PASS. Browser tests in this set skip without `AURAGO_RUN_BROWSER_SMOKE=1`.

- [ ] **Step 12: Commit**

```powershell
git add ui/pages_standard_theme_test.go ui/precision_pages_test.go ui/css/precision-workspace.css ui/css/precision-pages.css ui/css/dashboard.css ui/css/dashboard-shell.css
git commit -m "feat(ui): Standard material surfaces on pages" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Page background and enhancements

Implementation: `maker-agent`. Test runs: `test-agent`.

`ui/css/enhancements.css` is not edited, because chat themes load it. The foundation loads after it on every page that has it (pinned by the existing order tests), so overrides at equal specificity win.

**Files:**
- Modify: `ui/css/precision-workspace.css:134-137` and after line 143

- [ ] **Step 1: Selection without the teal shadow**

Lines 134-137:

```css
.pw-page ::selection {
    background: color-mix(in srgb, var(--pw-accent) 28%, transparent);
    color: var(--pw-text);
    text-shadow: none;
}
```

- [ ] **Step 2: Remove noise and orbs**

Insert after the `.pw-page a` rule. `body.pw-page::before` has specificity (0,1,2), the same as `[data-theme="dark"] body::before` in `enhancements.css:513` and `[data-theme="light"] body::before` in `shared-variables.css:256`. It loads later than both.

```css
/* A page corresponds to window content: no noise layer and no colour orbs. */
body.pw-page::before,
body.pw-page::after {
    content: none;
    animation: none;
}
```

Task 2 already added `display: none` to `.pw-page .card::before, .pw-page .card::after`. `[data-theme="dark"] .card:not(.radial-item)::before` has specificity (0,3,1) and beats `content: none` at (0,2,1), so the card shine would otherwise survive.

- [ ] **Step 3: Run**

```powershell
go test ./ui/ -run 'Precision|StandardTheme'
```

Expected: PASS. `assertPrecisionCSSScoped` accepts the selector because it contains `.pw-page`.

- [ ] **Step 4: Commit**

```powershell
git add ui/css/precision-workspace.css
git commit -m "feat(ui): no background decoration on pages" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Config colours and surfaces

Implementation: `maker-agent`. Test runs: `test-agent`.

**Files:**
- Modify: `ui/css/config.css`
- Modify: `ui/css/config-workspace.css`

**Mapping for `config.css`:**

| Old | New |
| --- | --- |
| `var(--success\|danger\|warning\|text-primary\|text-secondary, <hex>)` | same variable without the fallback |
| `var(--button-text, #fff)`, `color:#fff` (line 5623), `color: #0b0f1a` (line 1324) | `var(--pw-on-accent)` |
| `#ef4444`, `#f87171`, `#fca5a5` | `var(--pw-danger)` |
| `#22c55e`, `#48c78e`, `#16a34a`, `#15803d` | `var(--pw-success)` |
| `#eab308`, `#ca8a04`, `#92400e`, `#f59e0b` | `var(--pw-warning)` |
| `#60a5fa`, `#6366f1`, `#3b82f6` at 8 % (line 6460) | `var(--pw-accent)` |
| `#0f172a` | `var(--pw-canvas)` |
| `#ffffff` | `var(--pw-text)` |
| `color-mix(in srgb, #000000 50%\|55%, transparent)`, `color-mix(in srgb, #1e293b 35%, transparent)` | `var(--pw-scrim)` |
| `@keyframes pulse-glow-success` (lines 1288-1298) | delete. It is a glow, and no rule in `config*.css` or anywhere under `ui/` uses it |

**Kept on purpose:** `#fff` at 2867 (the TOTP QR code needs a white quiet zone), and `#3b82f6` / `#a855f7` at 4422-4428 (categorical index chips).

- [ ] **Step 1: Write the script**

Create the git-ignored `disposable/_standard_colours/config.js`:

```js
const fs = require('fs');

const file = 'ui/css/config.css';
let css = fs.readFileSync(file, 'utf8');

function must(from, to) {
  if (!css.includes(from)) throw new Error('missing: ' + from);
  css = css.split(from).join(to);
}

css = css.replace(/@keyframes pulse-glow-success \{[\s\S]*?\r?\n\}\r?\n(\r?\n)?/, '');
must('var(--button-text, #fff)', 'var(--pw-on-accent)');
must('background:var(--accent); color:#fff;', 'background:var(--accent); color:var(--pw-on-accent);');
must('color: #0b0f1a;', 'color: var(--pw-on-accent);');
must('color-mix(in srgb, #3b82f6 8%, transparent)', 'color-mix(in srgb, var(--pw-accent) 8%, transparent)');
must('color-mix(in srgb, #000000 50%, transparent)', 'var(--pw-scrim)');
must('color-mix(in srgb, #000000 55%, transparent)', 'var(--pw-scrim)');
must('color-mix(in srgb, #1e293b 35%, transparent)', 'var(--pw-scrim)');
css = css.replace(/var\(--(success|danger|warning|text-primary|text-secondary), #[0-9a-fA-F]{3,8}\)/g, 'var(--$1)');

const map = [
  [/#(?:ef4444|f87171|fca5a5)\b/g, 'var(--pw-danger)'],
  [/#(?:22c55e|48c78e|16a34a|15803d)\b/g, 'var(--pw-success)'],
  [/#(?:eab308|ca8a04|92400e|f59e0b)\b/g, 'var(--pw-warning)'],
  [/#(?:60a5fa|6366f1)\b/g, 'var(--pw-accent)'],
  [/#0f172a\b/g, 'var(--pw-canvas)'],
  [/#ffffff\b/g, 'var(--pw-text)'],
];
for (const [pattern, token] of map) css = css.replace(pattern, token);

fs.writeFileSync(file, css);
```

- [ ] **Step 2: Run it and prove completion**

```powershell
node disposable/_standard_colours/config.js
rg -n "#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?)\(" ui/css/config.css
```

Expected output is exactly five lines: `.auth-totp-qr` (`#fff`), and four `.idx-chip-doc` / `.idx-chip-img` lines.

Then change two `.btn-save` declarations by hand:

```css
/* .btn-save, line 652: replace these three declarations */
border:1px solid var(--pw-mat-primary-border); background:var(--pw-mat-primary); box-shadow:var(--pw-mat-primary-shadow);

/* .btn-save:hover, line 747: replace background */
background:var(--pw-mat-primary-hover);
```

- [ ] **Step 3: Mapping for `config-workspace.css`**

| Old | New |
| --- | --- |
| `var(--<token>, #hex)` everywhere | `var(--<token>)` (a regex replacement does this) |
| `var(--pw-canvas, var(--bg-primary, #0b1220))` (line 1336) | `var(--pw-canvas)` |
| `color-mix(in srgb, #48c78e 10%, transparent)` | `color-mix(in srgb, var(--pw-success) 10%, transparent)` |
| `color-mix(in srgb, #ffb74d 10%, transparent)` | `color-mix(in srgb, var(--pw-warning) 10%, transparent)` |
| `color-mix(in srgb, #ff5252 8%, transparent)` | `color-mix(in srgb, var(--pw-danger) 8%, transparent)` |
| `--surface-shadow: 0 6px 20px rgb(3 12 26 / 16%);` and the light `0 6px 20px rgb(31 67 111 / 7%)` | `--surface-shadow: var(--pw-mat-card-shadow);` (delete the light line) |
| `--surface-shadow-strong: 0 20px 64px rgb(3 12 26 / 36%);` | `--surface-shadow-strong: 0 20px 64px rgb(0 0 0 / 36%);` |
| light `--surface-shadow-strong: 0 20px 64px rgb(31 67 111 / 20%);` | `--surface-shadow-strong: 0 20px 64px rgb(40 52 76 / 20%);` |
| `--cfg-card-heading: linear-gradient(...)` (dark and light) | `--cfg-card-heading: transparent;` (delete the light line) |
| `--header-bg`, `--sidebar-bg`, `--input-bg` overrides (`--pw-canvas` based) | delete so the Task 2 aliases apply |

**Kept on purpose:** `#fff` knob on the Kaggle switch, and the five categorical `--ca-role-color` values.

Create `disposable/_standard_colours/config-workspace.js`:

```js
const fs = require('fs');

const file = 'ui/css/config-workspace.css';
let css = fs.readFileSync(file, 'utf8');

function must(from, to) {
  if (!css.includes(from)) throw new Error('missing: ' + from);
  css = css.split(from).join(to);
}

must('var(--pw-canvas, var(--bg-primary, #0b1220))', 'var(--pw-canvas)');
must('color-mix(in srgb, #48c78e 10%, transparent)', 'color-mix(in srgb, var(--pw-success) 10%, transparent)');
must('color-mix(in srgb, #ffb74d 10%, transparent)', 'color-mix(in srgb, var(--pw-warning) 10%, transparent)');
must('color-mix(in srgb, #ff5252 8%, transparent)', 'color-mix(in srgb, var(--pw-danger) 8%, transparent)');
css = css.replace(/var\((--[a-z0-9-]+), #[0-9a-fA-F]{3,8}\)/g, 'var($1)');
must('--surface-shadow: 0 6px 20px rgb(3 12 26 / 16%);', '--surface-shadow: var(--pw-mat-card-shadow);');
css = css.replace(/[ \t]*--surface-shadow: 0 6px 20px rgb\(31 67 111 \/ 7%\);\r?\n/, '');
must('rgb(3 12 26 / 36%)', 'rgb(0 0 0 / 36%)');
must('rgb(31 67 111 / 20%)', 'rgb(40 52 76 / 20%)');
css = css.replace(/--cfg-card-heading: linear-gradient\(110deg, rgb\(88 159 244[^;]*;/, '--cfg-card-heading: transparent;');
css = css.replace(/[ \t]*--cfg-card-heading: linear-gradient\(110deg, rgb\(91 160 242[^;]*;\r?\n/, '');
css = css.replace(/[ \t]*--(?:header|sidebar|input)-bg: var\(--pw-canvas\);\r?\n/g, '');

fs.writeFileSync(file, css);
```

Run and prove:

```powershell
node disposable/_standard_colours/config-workspace.js
rg -n "#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?)\(|\brgb\(|gradient\(" ui/css/config-workspace.css
```

Expected output is exactly: the Kaggle `#fff` knob, the five `--ca-role-color` lines, and the two `--surface-shadow-strong` lines.

- [ ] **Step 4: Config surfaces (anchors are `main`; find each by selector, Task 1 shifted them)**

In `ui/css/config-workspace.css`:

```css
/* :is(.btn-secondary, .btn-header) */
    color: var(--pw-text);
    background: var(--pw-mat-btn);
    border: 1px solid var(--pw-mat-btn-border);
    border-radius: var(--pw-radius-sm);
    box-shadow: var(--pw-mat-btn-shadow);

/* :is(.btn-secondary, .btn-header):hover */
    background: var(--pw-mat-btn-hover);

/* .modal-card */
    background: var(--pw-mat-card);
    border: 1px solid var(--pw-line-strong);
    border-radius: var(--pw-radius-lg);

/* .cfg-sidebar */
    background: var(--pw-mat-side);
    border-right: 1px solid var(--pw-mat-divider);
    box-shadow: var(--pw-mat-side-edge);

/* .sidebar-item:hover */
    background: var(--pw-mat-hover);

/* .sidebar-item.active */
    color: var(--pw-mat-selected-text);
    background: var(--pw-mat-selected);
    border-color: color-mix(in srgb, var(--pw-accent) 32%, var(--pw-line));

/* :is(.pw-panel, .cfg-topic, .cfg-object), .field-group.cfg-topic */
    border: 1px solid var(--pw-mat-card-border);
    border-radius: var(--pw-radius-lg);
    background: var(--pw-mat-card);

/* :is(.pw-panel-heading, .cfg-topic-heading) */
    border-radius: var(--pw-radius-lg) var(--pw-radius-lg) 0 0;

/* .pw-overview-card */
    background: var(--pw-mat-card);
    border: 1px solid var(--pw-mat-card-border);
    border-radius: var(--pw-radius-lg);

/* .pw-recent-card */
    background: var(--pw-mat-card);
    border: 1px solid var(--pw-mat-card-border);
    border-radius: var(--pw-radius-md);
```

The `.cfg-sidebar-search` input (rule ending at `background: var(--pw-surface);`, `main` line 307) becomes `background: var(--pw-mat-field);`.

Leave `.btn-speech-lab` unchanged, because `TestConfigSpeechLab*` pins its flat accent. It stays readable through `--cfg-on-accent`.

- [ ] **Step 5: Run**

```powershell
go test ./ui/ -run 'Config|Precision|StandardTheme'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add ui/css/config.css ui/css/config-workspace.css
git commit -m "feat(config): Standard colours and surfaces" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Login, setup, 404

Implementation: `maker-agent`. Test runs: `test-agent`.

Entry pages do not load `precision-pages.css` (pinned by `TestPrecisionEntryPagesOptInWithoutOperationalWorkspace`), so their adapters consume the tokens directly. Adapters must never contain `gradient(` or the word `glow`.

**Test change:** in `TestPrecisionEntryLoginCSSFallbackRemainsAvailable`, remove `.orb` from the selector list, because the spec removes the colour orbs. `.css-fallback-bg`, `.particle-grid` and `.scan-line` stay guarded.

**Files:**
- Modify: `ui/precision_pages_test.go:2403`
- Modify: `ui/login.html:51-66`
- Modify: `ui/css/login.css`
- Modify: `ui/js/login/main.js`
- Modify: `ui/js/login/flame-wrap.js:70-76`
- Modify: `ui/css/setup.css`
- Modify: `ui/css/not-found.css:75-115`
- Modify: `ui/testdata/wave4_colours.txt`

- [ ] **Step 1: Impact analysis for the JS functions**

Run `impact` upstream on `getWebGLTheme`, `applyThemeToWebGL`, `initWebGL`, `animate`, `createFloatingOrbs` and `colorForTheme`. Report callers, processes and risk. Warn on HIGH or CRITICAL. Treat `UNKNOWN` as unresolved and confirm with `rg` in `ui/js/login/`.

- [ ] **Step 2: Update the test**

`ui/precision_pages_test.go:2403`:

```go
	for _, selector := range []string{`.css-fallback-bg`, `.particle-grid`, `.scan-line`} {
```

- [ ] **Step 3: `login.html`**

Keep the hook ids. Lines 51-58 become:

```html
        <div id="login-color-aurora" class="login-color-aurora" aria-hidden="true"></div>
```

Delete lines 63-65 (the three `.orb` divs).

- [ ] **Step 4: `login.css` (bottom-up)**

| Lines | Edit |
| --- | --- |
| 1034-1035 | delete the `.login-aurora-blob` and `.login-aurora-sheen` selectors |
| 980-985 | remove `.login-card` from the selector list; keep `.corner-theme` |
| 965-970 | `background: var(--pw-mat-primary-hover); box-shadow: var(--pw-mat-primary-shadow); transform: none;` |
| 955-963 | see below |
| 932-940 | `border: 1px solid var(--pw-mat-field-border);` and `box-shadow: var(--pw-mat-field-shadow);` |
| 870-883 | see below |
| 698-711 | delete `.orb-1`, `.orb-2`, `.orb-3` |
| 646-649 | see below |
| 626-638 | delete |
| 344-352 | see below |
| 295-341 | delete orbs and `@keyframes orb-float` |
| 260-293 | delete `.css-fallback-bg::before`, `::after`, `@keyframes aurora-move` |
| 254-257 | `background: var(--pw-canvas);` |
| 225-239 | delete |
| 41-223 | delete all `.login-aurora-*` rules and their keyframes |
| 30-39 | see below |

```css
/* 955-963 */
.pw-page.pw-entry-page[data-entry-page="login"] .btn-login {
    min-height: 46px;
    border: 1px solid var(--pw-mat-primary-border);
    border-radius: var(--pw-radius-sm);
    background: var(--pw-mat-primary);
    color: var(--pw-on-accent);
    box-shadow: var(--pw-mat-primary-shadow);
}

/* 870-883 */
.pw-page.pw-entry-page[data-entry-page="login"] .login-card {
    position: relative;
    z-index: 1;
    width: 100%;
    max-width: none;
    padding: clamp(1.5rem, 5vw, 2.5rem);
    border: 1px solid var(--pw-mat-card-border);
    border-radius: var(--pw-radius-lg);
    background: var(--pw-mat-card);
    box-shadow: var(--pw-mat-card-shadow);
    backdrop-filter: none;
    overflow-wrap: anywhere;
}

/* 646-649 */
[data-theme="light"] .vignette {
    background: radial-gradient(ellipse at center, transparent 0%, color-mix(in srgb, var(--pw-canvas) 92%, transparent) 100%);
    opacity: 1;
}

/* 344-352 */
.particle-grid {
    position: absolute;
    inset: 0;
    background-image:
        linear-gradient(color-mix(in srgb, var(--pw-accent) 3%, transparent) 1px, transparent 1px),
        linear-gradient(90deg, color-mix(in srgb, var(--pw-accent) 3%, transparent) 1px, transparent 1px);
    background-size: 50px 50px;
    animation: grid-pulse 4s ease-in-out infinite;
}

/* 30-39 */
/* Solid backplate under the neural net / CSS fallback. */
.login-color-aurora {
    position: fixed;
    inset: 0;
    z-index: 0;
    overflow: hidden;
    pointer-events: none;
    background: var(--pw-canvas);
}
```

Proof, expected no output:

```powershell
rg -n "#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?)\(|login-aurora|\.orb" ui/css/login.css
```

- [ ] **Step 5: `main.js` and `flame-wrap.js`**

In `ui/js/login/main.js`, bottom-up:
- Delete lines 758-765 (`// Animate orbs` and its `forEach`).
- Delete lines 443-490 (`createFloatingOrbs`).
- Delete line 193.
- Line 186 comment becomes `// Transparent clear so the page canvas shows through the neural net.`
- Delete lines 142-153 (the orb recolour `forEach`).

Replace lines 90-115:

```js
        function getWebGLTheme(theme) {
            if (theme === 'light') {
                return {
                    clearColor: 0xf6f7f9,
                    fogColor: 0xf6f7f9,
                    particleStart: 0xb93e0b,
                    particleEnd: 0x9a3412,
                    nodeFlash: 0xffffff,
                    lineColor: 0xb93e0b,
                    trailColor: 0xed764d,
                    pulseColor: 0xe95420
                };
            }
            return {
                clearColor: 0x12161d,
                fogColor: 0x12161d,
                particleStart: 0xed764d,
                particleEnd: 0xb93e0b,
                nodeFlash: 0xedf1f7,
                lineColor: 0xed764d,
                trailColor: 0xf08763,
                pulseColor: 0xe95420
            };
        }
```

`ui/js/login/flame-wrap.js`, lines 70-76:

```js
function colorForTheme(theme) {
  // Standard orange fire (RGB 0-1): --pw-accent of each mode.
  if (theme === "light") {
    return [0.725, 0.243, 0.043];
  }
  return [0.929, 0.463, 0.302];
}
```

- [ ] **Step 6: `setup.css` mapping**

| Old | New |
| --- | --- |
| `rgba(45, 212, 191, A)`, `rgba(15, 118, 110, A)`, `rgba(99, 179, 237, A)`, `rgba(139, 92, 246, A)`, `rgba(109, 40, 217, A)` | `color-mix(in srgb, var(--pw-accent) A%, transparent)` |
| `rgba(239, 68, 68, A)` | same form with `var(--pw-danger)` |
| `rgba(245, 158, 11, A)` | same form with `var(--pw-warning)` |
| `rgba(72, 199, 142, A)` | same form with `var(--pw-success)` |
| `rgba(255, 255, 255, 0.3)` (spinner) | same form with `var(--pw-on-accent)` |
| `#2dd4bf`, `#14b8a6`, `#0d9488`, `#0f766e`, `#63b3ed`, `#a78bfa`, `#7c3aed` | `var(--pw-accent)` |
| `.is-match` and `.is-ok` colours | `var(--pw-success)` |
| `#ef4444`, `#e74c3c` | `var(--pw-danger)` |
| `#f59e0b`, `#eab308`, `#d6a53a`, `#8a6100` | `var(--pw-warning)` |
| `#48c78e` | `var(--pw-success)` |
| `color: #fff` | `color: var(--pw-on-accent)` |
| fallback in `var(--x, <literal>)` | fallback dropped; `--warning` and `--danger` become `--pw-warning` and `--pw-danger` |
| decorative gradients and glows | exact replacements in the script below |

Create `disposable/_standard_colours/setup.js`:

```js
const fs = require('fs');

const file = 'ui/css/setup.css';
let css = fs.readFileSync(file, 'utf8');

function must(from, to) {
  if (!css.includes(from)) throw new Error('missing: ' + from);
  css = css.split(from).join(to);
}

const ACCENT = 'color-mix(in srgb, var(--pw-accent)';

must('background: radial-gradient(circle at 50% 0%, rgba(45, 212, 191, 0.08), transparent 68%);', 'background: none;');
must('background: radial-gradient(circle at 50% 0%, rgba(45, 212, 191, 0.06), transparent 60%);', 'background: none;');
css = css.replace(/background:\s*radial-gradient\(circle at 50% 0%, rgba\(15, 118, 110, 0\.11\), transparent 68%\),\s*linear-gradient\(180deg, rgba\(255, 255, 255, 0\.05\), transparent 24%\);/, 'background: none;');
must('linear-gradient(135deg, #2dd4bf, #0d9488)', 'var(--pw-mat-primary)');
must('linear-gradient(135deg, #2dd4bf, #14b8a6)', 'var(--pw-mat-primary)');
must('box-shadow: 0 4px 16px rgba(45, 212, 191, 0.3);', 'box-shadow: var(--pw-mat-primary-shadow);');
must('box-shadow: 0 6px 24px rgba(45, 212, 191, 0.4);', 'box-shadow: var(--pw-mat-primary-shadow);');
must('box-shadow: 0 0 0 3px var(--accent-dim), 0 4px 16px rgba(45, 212, 191, 0.15);', 'box-shadow: 0 0 0 3px var(--accent-dim);');
must('box-shadow: 0 2px 8px rgba(45, 212, 191, 0.4);', 'box-shadow: none;');
must(`box-shadow: 0 0 0 5px rgba(15, 118, 110, 0.1), 0 0 18px rgba(15, 118, 110, 0.16);`, `box-shadow: 0 0 0 5px ${ACCENT} 10%, transparent);`);
must(`box-shadow: 0 0 0 3px rgba(15, 118, 110, 0.14), 0 0 22px rgba(15, 118, 110, 0.08);`, `box-shadow: 0 0 0 3px ${ACCENT} 14%, transparent);`);
must(`box-shadow: 0 0 0 3px rgba(15, 118, 110, 0.14), 0 4px 16px rgba(15, 118, 110, 0.1);`, `box-shadow: 0 0 0 3px ${ACCENT} 14%, transparent);`);
must('box-shadow: 0 10px 24px rgba(15, 118, 110, 0.22);', 'box-shadow: none;');
css = css.replace(/box-shadow:\s*0 24px 56px rgba\(83, 97, 122, 0\.16\),\s*inset 0 1px 0 rgba\(255, 255, 255, 0\.42\);/, 'box-shadow: var(--pw-shadow-soft);');
must('box-shadow: 0 12px 30px rgba(0, 0, 0, 0.28);', 'box-shadow: var(--pw-shadow-soft);');
must('box-shadow: 0 8px 24px rgba(0, 0, 0, 0.18);', 'box-shadow: var(--pw-shadow-soft);');
must('box-shadow: 0 24px 60px rgba(83, 97, 122, 0.2);', 'box-shadow: var(--pw-shadow-soft);');
must('box-shadow: 0 8px 18px rgba(83, 97, 122, 0.1);', 'box-shadow: var(--pw-shadow-soft);');
must('background: rgba(0, 0, 0, 0.6);', 'background: var(--pw-scrim);');
must('background: rgba(30, 41, 59, 0.35);', 'background: var(--pw-scrim);');
must('border-color: rgba(71, 85, 105, 0.16);', 'border-color: var(--pw-line);');
must('background: rgba(128, 128, 128, 0.12);', 'background: var(--pw-surface-soft);');
for (const surface of ['rgba(231, 239, 234, 0.78)', 'rgba(234, 242, 237, 0.96)', 'rgba(229, 238, 233, 0.94)', 'rgba(239, 246, 242, 0.98)']) {
  must(surface, 'var(--pw-surface)');
}
must('var(--border-color, #475569)', 'var(--pw-line-strong)');
must('color: #000;', 'color: var(--pw-canvas);');

css = css.replace(/var\(--warning, #[0-9a-fA-F]{3,8}\)/g, 'var(--pw-warning)');
css = css.replace(/var\(--danger, #[0-9a-fA-F]{3,8}\)/g, 'var(--pw-danger)');
css = css.replace(/var\((--[a-z0-9-]+), (?:#[0-9a-fA-F]{3,8}|rgba?\([^)]*\))\)/g, 'var($1)');
css = css.replace(/(is-match|is-ok) \{ color: #(?:2dd4bf|0f766e); \}/g, '$1 { color: var(--pw-success); }');

const channel = {
  '45, 212, 191': '--pw-accent', '15, 118, 110': '--pw-accent', '99, 179, 237': '--pw-accent',
  '139, 92, 246': '--pw-accent', '109, 40, 217': '--pw-accent',
  '239, 68, 68': '--pw-danger', '245, 158, 11': '--pw-warning', '72, 199, 142': '--pw-success',
  '255, 255, 255': '--pw-on-accent',
};
css = css.replace(/rgba\((\d+, \d+, \d+), ([\d.]+)\)/g, (whole, rgb, alpha) => {
  if (!channel[rgb]) throw new Error('unmapped: ' + whole);
  return `color-mix(in srgb, var(${channel[rgb]}) ${Math.round(parseFloat(alpha) * 100)}%, transparent)`;
});

css = css.replace(/color: #fff;/g, 'color: var(--pw-on-accent);');
const map = [
  [/#(?:2dd4bf|14b8a6|0d9488|0f766e|63b3ed|a78bfa|7c3aed)\b/g, 'var(--pw-accent)'],
  [/#(?:ef4444|e74c3c)\b/g, 'var(--pw-danger)'],
  [/#(?:f59e0b|eab308|d6a53a|8a6100)\b/g, 'var(--pw-warning)'],
  [/#48c78e\b/g, 'var(--pw-success)'],
];
for (const [pattern, token] of map) css = css.replace(pattern, token);

fs.writeFileSync(file, css);
```

Run and prove, expected no output:

```powershell
node disposable/_standard_colours/setup.js
rg -n "#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?)\(" ui/css/setup.css
```

Anything the script throws on, or the grep prints, is mapped by the table above and fixed by hand.

- [ ] **Step 7: Setup adapter surfaces**

In `ui/css/setup.css`, inside the adapter:

```css
/* .setup-header: replace background through backdrop-filter */
    border-bottom: 1px solid var(--pw-mat-divider);
    background: var(--pw-mat-bar);
    box-shadow: var(--pw-mat-bar-edge);
    backdrop-filter: none;

/* .setup-card: replace background, background-image and box-shadow */
    border: 1px solid var(--pw-mat-card-border);
    background: var(--pw-mat-card);
    box-shadow: var(--pw-mat-card-shadow);

/* .step-dot.active and .profile-recommended-bubble */
    color: var(--pw-on-accent);

/* .field-input, .field-select, textarea */
    border: 1px solid var(--pw-mat-field-border);
    box-shadow: var(--pw-mat-field-shadow);
```

Replace the rules at `main` lines 1154-1166 and add the secondary rule. Keep `.profile-recommended-bubble`'s pinned `background: var(--pw-accent)`.

```css
.pw-page.pw-entry-page[data-entry-page="setup"] .btn-next,
.pw-page.pw-entry-page[data-entry-page="setup"] .or-apply-btn {
    border-color: var(--pw-mat-primary-border);
    background: var(--pw-mat-primary);
    color: var(--pw-on-accent);
    box-shadow: var(--pw-mat-primary-shadow);
}

.pw-page.pw-entry-page[data-entry-page="setup"] .btn-next:hover,
.pw-page.pw-entry-page[data-entry-page="setup"] .or-apply-btn:hover {
    background: var(--pw-mat-primary-hover);
    box-shadow: var(--pw-mat-primary-shadow);
    transform: none;
}

.pw-page.pw-entry-page[data-entry-page="setup"] .btn-header,
.pw-page.pw-entry-page[data-entry-page="setup"] .btn-skip,
.pw-page.pw-entry-page[data-entry-page="setup"] .btn-theme {
    border-color: var(--pw-mat-btn-border);
    background: var(--pw-mat-btn);
    box-shadow: var(--pw-mat-btn-shadow);
}
```

- [ ] **Step 8: 404**

`ui/css/not-found.css`:

```css
/* .not-found-link: replace border, background, box-shadow */
    border: 1px solid var(--pw-mat-btn-border);
    background: var(--pw-mat-btn);
    box-shadow: var(--pw-mat-btn-shadow);

/* .not-found-link-primary */
    border-color: var(--pw-mat-primary-border);
    background: var(--pw-mat-primary);
    color: var(--pw-on-accent);
    box-shadow: var(--pw-mat-primary-shadow);

/* .not-found-link-primary:hover */
    background: var(--pw-mat-primary-hover);
    box-shadow: var(--pw-mat-primary-shadow);
    transform: none;

/* .not-found-link-secondary */
    background: var(--pw-mat-btn);
    color: var(--pw-text);
    border: 1px solid var(--pw-mat-btn-border);

/* .not-found-link-secondary:hover */
    border-color: var(--pw-accent);
    background: var(--pw-mat-btn-hover);
    color: var(--pw-text);
    transform: none;
```

The adapter still contains `box-shadow: none;` in `.not-found-code` and `background-image: none;` in its root.

- [ ] **Step 9: Refresh the ratchet**

```powershell
$env:AURAGO_UPDATE_WAVE4_BASELINES='1'; go test ./ui/ -run TestUIWave4; Remove-Item Env:AURAGO_UPDATE_WAVE4_BASELINES
git diff --stat -- ui/testdata
git diff -- ui/testdata
```

Expected: only lines removed or lowered, and `css/login.css` and `css/setup.css` disappear from `wave4_colours.txt`. If any count rises, stop and fix the CSS. Stage only baselines whose diff is explained by this task.

- [ ] **Step 10: Run**

```powershell
go test ./ui/ -run 'Precision|Wave4|StandardTheme|TestLoginFlame|TestCanvasUI'
```

Expected: PASS.

- [ ] **Step 11: Commit**

```powershell
git add ui/precision_pages_test.go ui/login.html ui/css/login.css ui/js/login/main.js ui/js/login/flame-wrap.js ui/css/setup.css ui/css/not-found.css ui/testdata
git commit -m "feat(ui): Standard look on login, setup and 404" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Chat default theme

Implementation: `maker-agent`. Test runs: `test-agent`.

**Bundle position:** last, after `chat-header-controls.css`.
- The existing test at `ui/chat_regression_test.go:1075` keeps `chat-header-controls.css` after `chat-themes.css`.
- `chat-standard.css` must be after `enhancements.css` so its overrides of the `[data-theme="dark"]` rules are ordered after them, and after `chat-header-controls.css` so nothing later beats it.
- Other themes are safe because of the prefix. Using `:root` also keeps it off `.chat-theme-option[data-theme="dark"]` in the theme dropdown, which a bare `[data-theme="dark"]` would match while another theme is active.
- `:root:is(...)` has specificity (0,2,0), one above `[data-theme="dark"]`.

**Files:**
- Create: `ui/css/chat-standard.css`
- Modify: `scripts/build-ui-bundles.js:278`
- Modify: `ui/css/chat.bundle.css` (generated)
- Modify: `ui/pages_standard_theme_test.go`

- [ ] **Step 1: Write the scope test**

Append to `ui/pages_standard_theme_test.go`:

```go
const chatStandardPrefix = `:root:is([data-theme="dark"], :not([data-theme]))`

func standardThemeSplitSelectors(header string) []string {
	var selectors []string
	depth, start := 0, 0
	for i, char := range header {
		switch char {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				selectors = append(selectors, strings.TrimSpace(header[start:i]))
				start = i + 1
			}
		}
	}
	return append(selectors, strings.TrimSpace(header[start:]))
}

func TestChatStandardThemeIsScoped(t *testing.T) {
	t.Parallel()

	raw := normalizeAssetText(mustReadUIFile(t, "css/chat-standard.css"))
	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(raw, "")
	depth, start, rules := 0, 0, 0
	for i, char := range css {
		switch char {
		case '{':
			if depth == 0 {
				header := strings.TrimSpace(css[start:i])
				rules++
				if strings.HasPrefix(header, "@") {
					if !strings.HasPrefix(header, "@keyframes chat-standard-") {
						t.Errorf("only @keyframes chat-standard-* at-rules are allowed: %q", header)
					}
				} else {
					for _, selector := range standardThemeSplitSelectors(header) {
						if selector != chatStandardPrefix && !strings.HasPrefix(selector, chatStandardPrefix+" ") {
							t.Errorf("selector must start with the default-theme prefix: %q", selector)
						}
					}
				}
			}
			depth++
		case '}':
			depth--
			if depth == 0 {
				start = i + 1
			}
		}
	}
	if rules == 0 {
		t.Fatal("chat-standard.css has no rules")
	}

	bundle := normalizeAssetText(mustReadUIFile(t, "css/chat.bundle.css"))
	standardAt := strings.Index(bundle, `/* ui/css/chat-standard.css */`)
	if standardAt < 0 || standardAt < strings.Index(bundle, `/* ui/css/chat-header-controls.css */`) {
		t.Errorf("chat-standard.css must be the last chat bundle part, found at %d", standardAt)
	}
}
```

- [ ] **Step 2: Run and confirm failure**

```powershell
go test ./ui/ -run 'TestChatStandardThemeIsScoped'
```

Expected: FAIL (file missing).

- [ ] **Step 3: Create `ui/css/chat-standard.css`**

```css
/* Chat default theme ("Standard") in the desktop Standard look. Every selector starts with
   :root:is([data-theme="dark"], :not([data-theme])), so no other chat theme is touched
   (TestChatStandardThemeIsScoped). Values come from desktop-materials.css and desktop-app-chat.css. */

:root:is([data-theme="dark"], :not([data-theme])) {
    color-scheme: dark;
    --bg-primary: #12161d;
    --bg-secondary: #1a212b;
    --bg-tertiary: #222a35;
    --bg-elevated: #222a35;
    --bg-glass: rgba(255, 255, 255, 0.035);
    --header-bg: #1a2029;
    --accent: #ed764d;
    --accent-secondary: #f08763;
    --accent-hover: #f08763;
    --accent-active: #ed764d;
    --accent-dim: rgba(233, 84, 32, 0.16);
    --accent-glow: rgba(233, 84, 32, 0.22);
    --user-bubble: linear-gradient(135deg, color-mix(in srgb, rgba(233, 84, 32, 0.16) 92%, transparent), color-mix(in srgb, rgba(233, 84, 32, 0.16) 55%, transparent));
    --user-bubble-tail: transparent;
    --user-bubble-shadow: transparent;
    --bot-bubble: linear-gradient(180deg, rgba(255, 255, 255, 0.035), rgba(255, 255, 255, 0) 60%), #1a212b;
    --bot-bubble-tail: transparent;
    --text-primary: #edf1f7;
    --text-secondary: #aeb7c6;
    --text-muted: #a4afc0;
    --text-inverse: #0c1016;
    --strong-color: #f3f6fb;
    --border: rgba(191, 210, 235, 0.15);
    --border-subtle: rgba(191, 210, 235, 0.15);
    --border-accent: rgba(233, 84, 32, 0.38);
    --modal-bg: rgba(4, 7, 12, 0.62);
    --modal-surface: #1a212b;
    --modal-border: rgba(191, 210, 235, 0.28);
    --code-bg: #0b0e13;
    --inline-code-bg: rgba(255, 255, 255, 0.07);
    --sidebar-bg: #0e1218;
    --card-bg: #1a212b;
    --input-bg: #0c1016;
    --input-border: rgba(191, 210, 235, 0.17);
    --success: #4ad29c;
    --success-bg: rgba(74, 210, 156, 0.15);
    --warning: #f2b84b;
    --warning-bg: rgba(242, 184, 75, 0.15);
    --danger: #ff7a72;
    --danger-bg: rgba(255, 122, 114, 0.15);
    --surface-shadow: 0 1px 2px rgba(0, 0, 0, 0.32), 0 8px 20px rgba(0, 0, 0, 0.2);
    --surface-shadow-strong: 0 32px 88px rgba(0, 0, 0, 0.56), 0 10px 28px rgba(0, 0, 0, 0.34);
    --surface-highlight: rgba(255, 255, 255, 0.05);
    --page-orb-opacity: 0;
    --chat-image-opacity: 0;
    /* chat-modules.css and stt-overlay.css read these before their teal fallbacks. */
    --vd-mat-primary: linear-gradient(180deg, #f08763, #ed764d), #ed764d;
    --vd-mat-primary-hover: linear-gradient(180deg, #f19575, #ef805a), #ed764d;
    --vd-mat-primary-text: #0c1016;
    --vd-mat-primary-shadow: 0 1px 2px rgba(0, 0, 0, 0.35), inset 0 1px 0 rgba(255, 255, 255, 0.22);
    --vd-mat-card-border: rgba(191, 210, 235, 0.13);
}

/* Background: no noise, orbs, tint or logo. */
:root:is([data-theme="dark"], :not([data-theme])) body::before,
:root:is([data-theme="dark"], :not([data-theme])) body::after,
:root:is([data-theme="dark"], :not([data-theme])) #chat-box::before,
:root:is([data-theme="dark"], :not([data-theme])) #chat-box::after {
    content: none;
    animation: none;
}

:root:is([data-theme="dark"], :not([data-theme])) #chat-box {
    background-image: none;
}

:root:is([data-theme="dark"], :not([data-theme])) ::selection {
    background: rgba(233, 84, 32, 0.28);
    text-shadow: none;
}

:root:is([data-theme="dark"], :not([data-theme])) ::-moz-selection {
    background: rgba(233, 84, 32, 0.28);
}

/* Header */
:root:is([data-theme="dark"], :not([data-theme])) .app-header {
    background: linear-gradient(180deg, rgba(255, 255, 255, 0.05), rgba(255, 255, 255, 0) 70%), #1a2029;
    box-shadow: inset 0 -1px 0 rgba(0, 0, 0, 0.35), inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

:root:is([data-theme="dark"], :not([data-theme])) .app-header::after {
    background: rgba(191, 210, 235, 0.15);
    opacity: 1;
}

:root:is([data-theme="dark"], :not([data-theme])) .logo .logo-wordmark-accent {
    background: none;
    color: #ed764d;
    -webkit-text-fill-color: currentColor;
    text-shadow: none;
}

/* Bubbles follow .vd-chat-bubble in desktop-app-chat.css. */
:root:is([data-theme="dark"], :not([data-theme])) .bubble.bot {
    backdrop-filter: none;
    border: 1px solid rgba(191, 210, 235, 0.13);
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.32), 0 8px 20px rgba(0, 0, 0, 0.2), inset 0 1px 0 rgba(255, 255, 255, 0.05);
}

:root:is([data-theme="dark"], :not([data-theme])) .bubble.user {
    border: 1px solid rgba(233, 84, 32, 0.38);
    color: #edf1f7;
    box-shadow: none;
}

:root:is([data-theme="dark"], :not([data-theme])) .bubble.bot::before,
:root:is([data-theme="dark"], :not([data-theme])) .bubble.user::before {
    content: none;
}

:root:is([data-theme="dark"], :not([data-theme])) .msg-row:hover .bubble.bot {
    transform: none;
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.32), 0 8px 20px rgba(0, 0, 0, 0.2), inset 0 1px 0 rgba(255, 255, 255, 0.05);
}

:root:is([data-theme="dark"], :not([data-theme])) .msg-row:hover .bubble.user {
    transform: none;
    box-shadow: none;
}

:root:is([data-theme="dark"], :not([data-theme])) .avatar.bot {
    background: rgba(233, 84, 32, 0.16);
    box-shadow: none;
}

/* Primary actions */
:root:is([data-theme="dark"], :not([data-theme])) .btn-send,
:root:is([data-theme="dark"], :not([data-theme])) .modal-btn.confirm {
    border: 1px solid #a33b16;
    background: linear-gradient(180deg, #f08763, #ed764d), #ed764d;
    color: #0c1016;
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.35), inset 0 1px 0 rgba(255, 255, 255, 0.22);
}

:root:is([data-theme="dark"], :not([data-theme])) .btn-send:hover,
:root:is([data-theme="dark"], :not([data-theme])) .modal-btn.confirm:hover {
    background: linear-gradient(180deg, #f19575, #ef805a), #ed764d;
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.35), inset 0 1px 0 rgba(255, 255, 255, 0.22);
    filter: none;
    transform: none;
}

/* Tool buttons */
:root:is([data-theme="dark"], :not([data-theme])) .btn-more:hover,
:root:is([data-theme="dark"], :not([data-theme])) .btn-more.is-open,
:root:is([data-theme="dark"], :not([data-theme])) .btn-upload:hover,
:root:is([data-theme="dark"], :not([data-theme])) .btn-voice:hover,
:root:is([data-theme="dark"], :not([data-theme])) .btn-push:hover {
    background: rgba(233, 84, 32, 0.15);
}

:root:is([data-theme="dark"], :not([data-theme])) .scroll-to-bottom-btn:hover,
:root:is([data-theme="dark"], :not([data-theme])) .scroll-to-bottom-btn:active {
    color: #0c1016;
}

:root:is([data-theme="dark"], :not([data-theme])) .scroll-to-bottom-btn.has-new {
    color: #0c1016;
    animation-name: chat-standard-pulse-dot;
}

@keyframes chat-standard-pulse-dot {
    0%, 100% { box-shadow: 0 0 0 0 rgba(233, 84, 32, 0.4); }
    50% { box-shadow: 0 0 0 8px rgba(233, 84, 32, 0); }
}

:root:is([data-theme="dark"], :not([data-theme])) .trait-bar-fill[data-trait="affinity"] {
    background: linear-gradient(90deg, #ed764d, #f08763);
}

/* enhancements.css: input focus, scrollbars, hover sweep */
:root:is([data-theme="dark"], :not([data-theme])) .input-wrap textarea:focus {
    border-color: #ed764d;
    background: #0c1016;
    box-shadow: 0 0 0 3px rgba(233, 84, 32, 0.22);
}

:root:is([data-theme="dark"], :not([data-theme])) #chat-content::-webkit-scrollbar-thumb,
:root:is([data-theme="dark"], :not([data-theme])) .session-drawer ::-webkit-scrollbar-thumb {
    background: rgba(191, 210, 235, 0.28);
    background-clip: padding-box;
}

:root:is([data-theme="dark"], :not([data-theme])) #chat-content::-webkit-scrollbar-track,
:root:is([data-theme="dark"], :not([data-theme])) .session-drawer ::-webkit-scrollbar-track {
    background: transparent;
}

:root:is([data-theme="dark"], :not([data-theme])) .chat-theme-option::after,
:root:is([data-theme="dark"], :not([data-theme])) .session-item::after {
    background: linear-gradient(90deg, transparent, rgba(233, 84, 32, 0.45), transparent);
}
```

- [ ] **Step 4: Add the bundle part**

`scripts/build-ui-bundles.js`, lines 278-279:

```js
      'ui/css/chat-header-controls.css',
      'ui/css/chat-standard.css'
    ]
```

- [ ] **Step 5: Build, and stage only the chat bundle**

```powershell
npm run build:ui
git status --short
```

Expected: only `scripts/build-ui-bundles.js`, `ui/css/chat-standard.css`, `ui/css/chat.bundle.css` and `ui/pages_standard_theme_test.go` changed. If another bundle changed, stop and report it. Do not stage it.

- [ ] **Step 6: Prove no literal teal remains active**

```powershell
rg -n "#2dd4bf|#5eead4|#14b8a6|#0d9488|45, ?212, ?191|13, ?148, ?136|99, ?102, ?241" ui/css/chat-standard.css
```

Expected: no output. Base files stay untouched and hold the old values for other themes. The overrides above cover every active literal.

- [ ] **Step 7: Run**

```powershell
go test ./ui/ -run 'Chat|StandardTheme'
npm run check:ui
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add scripts/build-ui-bundles.js ui/css/chat-standard.css ui/css/chat.bundle.css ui/pages_standard_theme_test.go
git commit -m "feat(chat): default theme in the desktop Standard look" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Docs and final check

Docs: `docs-agent`. Test runs and screenshots: `test-agent`.

**Files:**
- Modify: `ui/AGENTS.md` (after line 134)

- [ ] **Step 1: Document the contracts**

Insert after line 134 of `ui/AGENTS.md`:

```markdown
- Pages use the desktop Standard look. Dark and light colours are tokens in
  `css/precision-workspace.css`; surfaces are `--pw-mat-*` tokens there. Gradient
  literals appear only on `--pw-mat-*` lines
  (`TestPagesStandardThemeGradientsLiveInMaterialTokens`). Component rules use
  `background: var(--pw-mat-card)` and never set `background-image: none` next to it.
  Page tokens must keep 4.5:1 (`TestPagesStandardThemeTokenContrast`). Config inherits
  the palette and must not redefine it. The accent is fixed and independent of the
  desktop's `appearance.accent`.
- The chat default theme lives in `css/chat-standard.css`, the last chat bundle part.
  Every selector starts with `:root:is([data-theme="dark"], :not([data-theme]))`
  (`TestChatStandardThemeIsScoped`). Do not use a bare `[data-theme="dark"]` there: it
  also matches the Standard entry in the theme dropdown. Do not change
  `shared-variables.css` or `tokens.css` for it. Run `npm run build:ui` after edits.
```

- [ ] **Step 2: Static verification**

```powershell
go test ./ui/ -run 'Precision|Wave4|StandardTheme'
npm run check:ui
git diff main --stat -- ui/shared-variables.css ui/css/tokens.css ui/shared-components.css ui/css/enhancements.css ui/css/chat.css ui/css/chat-modules.css ui/css/chat-themes.css
```

Expected: tests pass, check passes, and the frozen-file diff is empty.

- [ ] **Step 3: Start a scratch instance**

Follow `C:\Users\Andi\.claude\projects\C--Users-Andi-Documents-repo-AuraGo\memory\reference_local_scratch_instance_windows.md`:
- Build from a `git archive` snapshot of the branch.
- Use the assetpack ldflags and `-install-assets`.
- Use its own `AURAGO_MASTER_KEY`, a dead `DOCKER_HOST=tcp://127.0.0.1:1`, and `server.https.enabled: false`.
- Use `server.ui_language: en` and a short run dir.
- Set a password. Stop the instance only by its own PID.

- [ ] **Step 4: Screenshot runner**

Create `disposable/_standard_shots/main.go`. Build it to `D:/tmp/go-build/standard-shots.exe`, because repo-built binaries cannot start Chrome.

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func main() {
	origin, out, password := os.Args[1], os.Args[2], os.Args[3]
	if err := os.MkdirAll(out, 0o755); err != nil {
		panic(err)
	}
	bin, ok := launcher.LookPath()
	if !ok {
		panic("Chrome not found")
	}
	browser := rod.New().ControlURL(launcher.New().Bin(bin).NoSandbox(true).MustLaunch()).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage(origin + "/auth/login").Timeout(60 * time.Second)
	page.MustSetViewport(1440, 900, 1, false)
	nav := 0
	shoot := func(name, route, theme string) {
		nav++
		page.MustEval(`theme => localStorage.setItem('aurago-theme', theme)`, theme)
		page.MustNavigate(fmt.Sprintf("%s%s?nav=%d", origin, route, nav)).MustWaitLoad()
		time.Sleep(1500 * time.Millisecond)
		file := filepath.Join(out, name+"-"+theme+".png")
		if err := os.WriteFile(file, page.MustScreenshot(), 0o644); err != nil {
			panic(err)
		}
		fmt.Println(name, theme, page.MustInfo().URL)
	}

	shoot("login", "/auth/login", "dark")
	shoot("login", "/auth/login", "light")
	page.MustElement("#password").MustInput(password)
	page.MustElement("#btnLogin").MustClick()
	time.Sleep(2 * time.Second)

	routes := map[string]string{
		"404": "/no-such-page", "cheatsheets": "/cheatsheets", "config": "/config", "containers": "/containers",
		"dashboard": "/dashboard", "invasion": "/invasion", "knowledge": "/knowledge", "media": "/media",
		"missions": "/missions/v2", "plans": "/plans", "setup": "/setup", "skills": "/skills", "truenas": "/truenas",
	}
	for name, route := range routes {
		shoot(name, route, "dark")
		shoot(name, route, "light")
	}
	shoot("chat-standard", "/", "dark")
	shoot("chat-light", "/", "light")
	shoot("chat-cyberwar", "/", "cyberwar")
}
```

Run it once. The printed URL shows any feature-gated redirect. If a route redirects away, enable that feature in the scratch config and shoot that page only. If `/setup` redirects for a configured instance, shoot it from a fresh run dir before setting the password.

- [ ] **Step 5: Review one round**

Look at every PNG once, against the desktop Settings window in Standard.

Check for:
- near-black neutral surfaces in dark, white cards in light
- orange primary and selection
- 8-14 px radii
- no blue or teal
- no orbs or logo
- login without aurora
- readable text in both modes
- chat bubbles like the desktop app
- `chat-light` and `chat-cyberwar` unchanged in look

Fix what is wrong in the owning task's file, then re-shoot only the affected page. Do not repeat the full round.

- [ ] **Step 6: Commit and hand over**

```powershell
git add ui/AGENTS.md
git commit -m "docs(ui): Standard theme contracts for pages and chat" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Stop the scratch instance by its PID. Report the branch `claude/pages-standard-theme` and the screenshot folder. Landing on `main` and final acceptance on aurago-test belong to the user. Do not push or deploy.

---

## Spec corrections

- **Config did not inherit the tokens.** `config-workspace.css` redefines the whole `--pw-*` palette and the radii on `[data-workspace-page="config"]` (lines 4-18, 58-68). Task 1 deletes them and the contrast test guards this.
- **Component rule location.** The spec says central rules live in `precision-workspace.css`. Most existing rules are in `precision-pages.css`, which loads later at equal specificity, and entry pages do not load it. The plan keeps tokens in `precision-workspace.css`, edits rules where they already are, and styles entry pages in their adapters.
- **Login has hardcoded colours in JS.** `getWebGLTheme` in `ui/js/login/main.js` and `colorForTheme` in `ui/js/login/flame-wrap.js` are blue, and `createFloatingOrbs` draws three.js orbs. The spec's table lists CSS only. The plan removes the WebGL orbs, the CSS aurora and the CSS fallback orbs. Not settled by the spec: the neural net, the flame around the card and the scan line stay, recoloured to orange. If the user wants those gone too, that is a small follow-up, but some selectors are pinned by `TestPrecisionEntryLoginCSSFallbackRemainsAvailable`.
- **One guard test must change.** `TestPrecisionEntryLoginCSSFallbackRemainsAvailable` required `.orb` CSS, which conflicts with "orbs disappear". `.orb` leaves that list only.
- **Pages have no background logo.** `--bg-logo` is read only by `#chat-box::after` in `chat.css`. The page decoration is the noise layer and orbs on `body::before` and `body::after` in `shared-variables.css`, plus `enhancements.css:513`.
- **`enhancements.css` needs no edit.** The table lists it as manual work, but it is shared with other chat themes. Overrides from `precision-workspace.css` and `chat-standard.css` are enough, so it stays untouched.
- **Chat scope selector.** A bare `[data-theme="dark"]` also matches the Standard entry in the theme dropdown while another theme is active (`.chat-theme-option[data-theme]`), so those themes would not stay pixel-identical. The plan uses `:root:is([data-theme="dark"], :not([data-theme]))`, which matches the same `<html>` states.
- **Teal line counts overstate the work.** 19 of the 21 `chat-modules.css` lines are inert `var(--accent, #2dd4bf)` fallbacks; only the `pulse-dot` keyframes are active. Many of the `chat.css` lines are inert the same way. Conversely, the spec's chat section omits the active teal in `enhancements.css` (selection, focus, scrollbars, hover sweep, bubble hover) and the `--vd-mat-*` fallbacks in `stt-overlay.css`. The plan covers all of these.
- **Shared status and modal tokens.** `--success`, `--warning`, `--danger` and `--modal-surface` on pages resolve to old `shared-variables.css` values unless aliased. Task 2 adds aliases to `.pw-page`.
- **Light accent changed.** `#c2410c` reads 4.46:1 on `--pw-surface-elevated` and 4.33:1 on `--pw-surface-soft`, so the light `--pw-accent` is `#b93e0b` (4.81 and 4.68). The spec table is updated.
- **Mixed colours.** `#f08763`, `#ed764d`, `#f19575`, `#ef805a` and `#a33b16` are confirmed by computing the mixes of `#E95420`.
