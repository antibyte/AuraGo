# Domain Pitfalls: AuraGo UI/UX

**Domain:** Web UI / SPA Frontend
**Researched:** 2026-04-03
**Confidence:** MEDIUM-HIGH

## Executive Summary

AuraGo's embedded Web UI suffers from accumulated technical debt in CSS organization, responsive breakpoint inconsistencies, and incomplete internationalization coverage. The issues are concentrated in three areas: CSS specificity wars between shared.css and page-specific files, responsive grid systems that break at edge cases, and translation gaps where keys are missing, inconsistently named, or contain raw HTML. These are classic SPA and i18n pitfalls that compound as the codebase grows.

---

## Critical Pitfalls

### Pitfall 1: CSS Specificity Wars

**What goes wrong:** The UI uses a shared.css for common components plus page-specific CSS files (missions.css, config.css, setup.css). When both files define styles for the same class, the one loaded second usually wins, but not always due to selector specificity.

**Root cause:** No CSS architecture or naming convention (BEM, ITCSS, etc.) to prevent collisions. Classes like `.card`, `.field-group`, `.badge` are defined in shared.css but overridden in page-specific files.

**Consequences:**
- Mission Control badges render differently than Dashboard badges
- Form inputs in Config use different border-radius than Setup
- Theme overrides in setup.css (lines 936-1018) use `[data-theme="light"]` selectors that may conflict with shared.css dark/light theming

**Prevention:** Establish a CSS architecture. Recommended: use prefixed classes (e.g., `.cfg-card`, `.msn-card`) or adopt BEM strictly.

**Detection:** Load each page and visually compare identical components (cards, badges, inputs). Any visual difference is a specificity leak.

---

### Pitfall 2: Responsive Grid Breakpoint Accumulation

**What goes wrong:** Each CSS file defines its own breakpoints with no coordination.

**Evidence from missions.css:**
- Line 842: `@media (max-width: 768px)` - missions grid
- Line 883: `@media (max-width: 640px)` - queue section
- Line 911: `@media (max-width: 480px)` - status bar, page header
- Line 953: `@media (max-width: 380px)` - status bar again

**Evidence from setup.css:**
- Line 271: `@media (max-width: 560px)` - field-row
- Line 1020: `@media (max-width: 768px)` - setup header
- Line 1071: `@media (max-width: 560px)` - footer, or-browser

**Problem 1:** Status bar at line 847 uses `repeat(4, minmax(0, 1fr))` which creates 4 equal columns even on mobile. At 480px this becomes 4 cramped columns.

**Problem 2:** `.cfg-input-flex { flex:1; min-width:240px; }` in config.css uses fixed pixel width that does not scale with viewport.

**Problem 3:** The page-header fix at lines 932-951 (stack headline above buttons) is a band-aid for a layout that was not designed mobile-first.

**Prevention:** Define a single breakpoint scale in shared.css and use it consistently. Mobile-first development prevents reactive fixes.

---

### Pitfall 3: Translation Key Inconsistencies

**What goes wrong:** Translation keys are not consistently named across files, and some keys are missing entirely.

**Evidence:**

1. **Inconsistent naming:**
   - Step labels use `setup.step_label_0` through `setup.step_label_3` (en) but `setup.step_label_0` through `setup.step_label_3` (de) - consistent
   - BUT `setup.step0_title` vs `setup.step1_title` uses different patterns

2. **Mixed language in translations:**
   - German file: `setup.header_subtitle: "Quick Setup"` (English, not German)
   - German file: `setup.step0_language_label: "Language / Sprache"` (mixed)
   - German file: `setup.or_browser_free_button: "🆓 Kostenlose"` (German) but English is `"🆓 Free"`

3. **HTML entities in translation values:**
   - `setup.step0_native_functions_desc` uses `\u003cstrong\u003e` instead of actual `<strong>` tags
   - This pattern suggests a serialization issue during translation export

4. **Missing placeholder keys:**
   - `setup.step0_model_browse` in HTML but may not exist in all language files
   - `setup.language_custom` option at line 189 of setup.html references `data-i18n="setup.step0_provider_custom"` which is the wrong key (should be `setup.language_custom`)

**Prevention:** Use a translation management system (Phrase, Lokalise, or PO files with flat JSON). Enforce key naming conventions via linting.

---

## Moderate Pitfalls

### Pitfall 4: Hardcoded Color Fallbacks

**What goes wrong:** CSS variables are used with fallbacks that hardcode values, creating visual inconsistency when variables change.

**Evidence:**
- `config.css` line 161: `background:var(--warning-bg,#3d2e00); border-color:var(--warning,#f9a825);`
- `setup.css` line 572: `background: var(--bg-secondary, #1a1a2e);`
- `setup.css` line 578: `border: 1px solid var(--border-subtle, #2a2a3e);`

**Consequence:** Light/dark theme toggle may leave elements with wrong colors if fallback fires unexpectedly.

**Prevention:** Define all colors as CSS variables in tokens.css. Never use fallbacks except for truly optional enhancements.

---

### Pitfall 5: Toggle Component Duplication

**What goes wrong:** Two different toggle implementations exist.

**Evidence:**
- `setup.css` lines 296-336: checkbox-based toggle (`.toggle-switch` with `input:checked + .toggle-slider`)
- `config.css` lines 477-495: class-based toggle (`.toggle` with `.on` class toggled via JS)

**Consequence:** JavaScript must handle both patterns. Confusing to maintain. Accessibility may differ between implementations.

**Prevention:** Pick one pattern (checkbox-based is more accessible and requires no JS state management) and consolidate.

---

### Pitfall 6: HTML Data Attribute Inconsistencies

**What goes wrong:** i18n uses multiple attribute patterns without enforcement.

**Evidence from setup.html:**
- `data-i18n="key"` - text content
- `data-i18n-title="key"` - title attribute
- `data-i18n-placeholder="key"` - placeholder attribute
- `data-i18n-html="key"` - HTML content (dangerous XSS vector if not sanitized)

**Consequence:** `data-i18n-html` usage in line 199 (`setup.step0_native_functions_desc`) passes HTML directly. If translation files are compromised, XSS is possible.

**Prevention:** Avoid `data-i18n-html` where possible. If needed, sanitize on the server side before injecting into page.

---

### Pitfall 7: Fixed Pixel Values in Flexible Layouts

**What goes wrong:** Flex and grid items use `min-width` with fixed pixel values that break on small viewports.

**Evidence:**
- `config.css` line 9: `.cfg-input-flex { flex:1; min-width:240px; }`
- `missions.css` line 33: `grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));`

**Consequence:** At 320px viewport (small phones), 240px min-width forces horizontal scroll instead of stacking.

**Prevention:** Use `min(240px, 100%)` or `clamp()` for fluid minimums. Design mobile-first so stacking is the default.

---

## Minor Pitfalls

### Pitfall 8: Commented-Out CSS as Documentation

**What goes wrong:** missions.css lines 3-15 contain a long block comment listing shared.css components used. This is documentation that can drift from reality.

**Prevention:** If depending on shared.css, import it explicitly in the HTML rather than relying on comment documentation.

---

### Pitfall 9: Animation Keyframes Scoped Incorrectly

**What goes wrong:** `@keyframes card-enter` (missions.css line 370) uses nth-child delays up to n=8. If more than 8 cards exist, cards 9+ have no delay but still use the animation.

**Prevention:** Use `animation-delay: calc(var(--index) * 50ms)` with CSS custom properties instead of nth-child.

---

### Pitfall 10: Duplicate `.mission-stats` Definition

**What goes wrong:** missions.css line 475 and line 541 both define `.mission-stats` with different properties.

Line 475: `flex: 1; min-width: 0; display: flex; gap: 12px; flex-wrap: wrap; align-items: center;`
Line 541: `display: flex; gap: 12px; font-size: 0.8rem; color: var(--text-secondary);`

**Consequence:** Second definition wins. The `flex: 1; min-width: 0; align-items: center` from first definition is lost.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|-------------|
| Missions page refactor | CSS specificity overrides from shared.css | Use more specific selectors or prefixed class names |
| Config page audit | Fixed pixel min-widths break mobile | Replace 240px with fluid `min(240px, 100%)` |
| Translation audit | Missing keys discovered mid-refactor | Add translation linting to CI pipeline |
| Theme consolidation | Hardcoded color fallbacks fire unexpectedly | Audit all var() calls for missing fallbacks |
| Setup page refactor | Duplicate toggle implementations | Consolidate to checkbox-based toggles |

---

## Sources

- Project planning: `.planning/PROJECT.md`
- Codebase concerns: `.planning/codebase/CONCERNS.md`
- Translation files: `ui/lang/setup/*.json` (15 languages)
- CSS files analyzed: `ui/css/missions.css`, `ui/css/config.css`, `ui/css/setup.css`
- HTML template: `ui/setup.html`
