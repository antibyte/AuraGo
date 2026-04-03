# Project Research Summary

**Project:** AuraGo UI/UX Overhaul
**Domain:** Embedded Web SPA (vanilla JS/CSS, Go-embedded)
**Researched:** 2026-04-03
**Confidence:** HIGH

## Executive Summary

AuraGo's embedded Web UI is a vanilla JavaScript SPA using a glassmorphism design language with CSS custom properties for dark/light theming. The codebase has a well-established foundation (shared components, theme system, i18n for 15 languages) but suffers from severe CSS technical debt: files of 3000+ lines with duplicate keyframes, hardcoded colors, inconsistent naming, and conflicting specificity between shared.css and page-specific stylesheets. The architecture is sound -- multi-page Go-embedded SPA with server-side translation injection -- but the execution has accumulated enough debt to cause real user-facing issues (Mission Control overflow, translation gaps, responsive breaks).

Experts building similar UIs follow a disciplined CSS architecture (ITCSS, BEM, or utility classes) to prevent specificity wars, enforce a single breakpoint scale, and centralize design tokens. The recommended approach for AuraGo is a three-phase refactor: (1) CSS foundation cleanup, (2) component unification and responsive fixes, (3) polish and accessibility. The biggest risk is attempting too much in parallel -- each phase should be validated before moving to the next.

## Key Findings

### Recommended Stack

**Summary from STACK.md**

The UI uses vanilla CSS with no framework. Design tokens live in `tokens.css` (CSS custom properties) and shared components in `shared.css` (~3000 lines), with page-specific stylesheets layered on top. The glassmorphism aesthetic uses `backdrop-filter: blur()`, semi-transparent backgrounds, and a unified teal accent (`#2dd4bf`). Dark/light themes are toggled via `[data-theme]` attribute selectors on `:root`. CDN resources (Chart.js, CodeMirror 6) are referenced but minimally used.

**Core technologies:**
- **Vanilla CSS (CSS Custom Properties)** -- All styling via CSS variables, no Tailwind/Bootstrap. This keeps the binary small and embedded asset count low, but requires discipline to avoid chaos.
- **Glassmorphism design** -- `backdrop-filter: blur()`, semi-transparent cards, accent glow on hover. This is the AuraGo visual identity and should be preserved.
- **CSS two-tier architecture (tokens.css + shared.css + page CSS)** -- Good separation of concerns in theory, but broken in practice by file size and duplication.
- **Go `go:embed`** -- All UI assets embedded at compile time for single-binary deployment. This constrains what frameworks can be used (no npm builds).
- **Server-side i18n injection** -- Go server injects translation JSON into each page at render time, avoiding translation loading race conditions.

### Expected Features

**Summary from FEATURES.md**

**Must have (table stakes):**
- Unified Card System -- `.card` class applied consistently across all pages (currently varies)
- Theme System completion -- Dark/light mode with no flash on load, CSS variables throughout
- Responsive Layout -- Consistent breakpoints across all pages, no overflow/clipping (Mission Control is broken)
- Translation Coverage -- All 15 languages with no missing keys, no `document.write()` inline translation
- Toast Notification -- Unified toast system for all feedback (currently per-page implementations)
- Modal System -- Single modal component with variants (currently dual systems: `modal-overlay`+`modal-card` and `vault-modal-*`)
- Form Components -- Consistent inputs, selects, toggles across config and setup pages

**Should have (competitive):**
- Radial Menu refinement -- Quick-access actions, but mobile overlaps with hamburger navigation
- Mood Widget -- Emotion visualization for personality system
- Loading States -- Skeleton loaders for data-heavy pages
- Connection State Pills -- Real-time animated status indicators (well-designed, preserve this)

**Defer (v2+):**
- Command Palette (Cmd+K) -- Alternative to radial menu, not essential
- Multi-step Wizard -- Complex setup flows
- Drag-and-Drop in config -- Already exists in chat modules
- Real-time Collaboration Indicators -- Only if multi-user support added

### Architecture Approach

**Summary from ARCHITECTURE.md**

AuraGo is NOT a true SPA that dynamically loads content. It uses a multi-page architecture where each route (`/`, `/config`, `/setup`, `/login`) is a separate HTML file served by the Go server. JavaScript is page-specific (`js/chat/main.js`) combined with `shared.js` for cross-cutting concerns. CSS follows a two-tier system: design tokens in `tokens.css` (CSS custom properties) and shared components in `shared.css`, with page-specific stylesheets layered on top.

**Major components:**
1. **Go server** -- Serves different HTML shells per URL, injects i18n JSON, handles SSE for real-time updates
2. **shared.js** (~1280 lines) -- Cross-cutting: `t()`, `showModal()`, `showToast()`, `toggleTheme()`, `AuraSSE`, `initPWA()`, `injectRadialMenu()`
3. **shared.css** (~3000 lines) -- Theme variables, shared components (buttons, cards, modals, badges, forms, toggles, tabs, toasts)
4. **tokens.css** -- CSS custom properties only (colors, spacing, typography, z-index, shadows, keyframe definitions)
5. **Page-specific CSS + JS** -- ~15 pages each with own stylesheet and main.js entry point

### Critical Pitfalls

**Top findings from PITFALLS.md**

1. **CSS Specificity Wars** -- shared.css defines `.card`, `.badge`, `.form-group` but page-specific files override them with different values. Mission Control badges render differently than Dashboard badges. Prevention: establish prefixed class names (e.g., `.cfg-card`, `.msn-card`) or adopt strict BEM.

2. **Responsive Grid Breakpoint Accumulation** -- Each CSS file defines its own breakpoints with no coordination (480px, 560px, 640px, 768px, 900px, 1100px). Status bar uses `repeat(4, minmax(0, 1fr))` creating 4 cramped columns on mobile. Config uses fixed `min-width:240px` that forces horizontal scroll on small phones. Prevention: single breakpoint scale in shared.css, mobile-first development.

3. **Translation Key Inconsistencies** -- Mixed language in German translations (`"Quick Setup"` in English inside de.json), HTML entities instead of actual tags (`\u003cstrong\u003e`), wrong key references (`setup.step0_provider_custom` instead of `setup.language_custom`). Prevention: translation linting in CI, key naming conventions.

4. **Dual Modal Systems** -- `modal-overlay`+`modal-card` and `vault-modal-*` are separate code paths. JavaScript must handle both patterns. Prevention: consolidate to single `.modal` component with `data-variant` attribute.

5. **Hardcoded Color Fallbacks** -- `background:var(--warning-bg,#3d2e00)` in config.css creates visual inconsistency when theme changes. Prevention: define all colors as CSS variables, remove fallbacks except for optional enhancements.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: CSS Foundation Cleanup
**Rationale:** All other UI work depends on a sane CSS foundation. The current files are 3000+ lines each with massive duplication. This phase establishes the architectural constraints that make later phases tractable.

**Delivers:**
- Extracted duplicate keyframes into single `animations.css`
- Consolidated status badge classes into unified `.status-badge` with modifier classes
- Replaced all hardcoded colors with CSS variable references
- Split `shared.css` by responsibility (reset, themes, components, utilities)
- Established CSS naming convention (BEM or prefixed)
- Added CSS linting configuration to catch hardcoded colors and duplicate definitions

**Addresses:** Critical Pitfalls 1, 5 (CSS specificity, hardcoded colors)

**Avoids:** Starting component work on a broken foundation

### Phase 2: Component Unification and Responsive Fixes
**Rationale:** With a clean CSS foundation, we can now systematically fix components across all pages. This phase tackles the user-facing bugs and inconsistencies identified in the feature research.

**Delivers:**
- Unified modal system (single `.modal` with variants)
- Consolidated toggle implementation (checkbox-based, accessible)
- Fixed Mission Control layout (overflow, pill placement, button cutoff)
- Fixed Config page save bar (sticky on mobile)
- Unified breakpoint scale applied consistently
- Fixed responsive grid issues (replace fixed `min-width` with fluid `min()` or `clamp()`)
- Unified card system enforced across all pages

**Addresses:** Features MVP items (Modal System, Toast, Form Components), Pitfalls 2, 4, 6, 7

**Avoids:** Fixing one page only to have the same issue recur in another

### Phase 3: Polish and i18n Audit
**Rationale:** By phase 3 the structural issues are resolved. This phase focuses on translation completeness, animation quality, and accessibility.

**Delivers:**
- Complete translation audit across all 15 languages
- Fixed translation key naming conventions and HTML entity issues
- Animation audit -- remove gratuitous animations, respect `prefers-reduced-motion`
- Loading skeletons for data-heavy pages
- ARIA labels and keyboard navigation audit
- Radial menu mobile optimization

**Addresses:** Pitfall 3 (translation keys), remaining Feature MVP items (Translation Coverage, Loading States, Accessibility Audit)

**Avoids:** Shipping with incomplete translations or motion accessibility issues

### Phase Ordering Rationale

- **Phase 1 first** because CSS architecture problems affect every page and every component. Working on components without fixing the foundation creates more debt.
- **Phase 2 second** because user-facing bugs (Mission Control overflow, modal confusion) are the most visible problems and need the foundation from Phase 1 to fix correctly.
- **Phase 3 last** because translation and animation polish matters less if the layout is broken or modals behave inconsistently.
- **Grouping rationale:** Each phase builds on the previous -- Phase 1 enables Phase 2's component fixes, Phase 2's unified components enable Phase 3's systematic audit.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1:** CSS architecture patterns (ITCSS vs BEM vs prefixed naming) -- the research identified the problem but did not recommend a specific methodology. Needs a decision during planning.
- **Phase 2:** Modal accessibility implementation -- dual modal systems suggest complex state management. Needs investigation of how modals are currently opened/closed across pages.

Phases with standard patterns (skip research-phase):
- **Phase 1:** Keyframe extraction and file splitting -- straightforward refactoring with well-known patterns
- **Phase 3:** Translation audit -- operational work guided by existing linting conventions

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Vanilla CSS confirmed by direct file analysis; CDN resources catalogued; no framework adoption needed |
| Features | HIGH | Component inventory from direct code review; feature landscape confirmed against live HTML/CSS files |
| Architecture | HIGH | Multi-page SPA pattern confirmed via embed.go and template analysis; i18n system traced through Go server |
| Pitfalls | MEDIUM-HIGH | Pitfalls identified from direct CSS analysis but some (translation gaps) may be incomplete -- full key audit across all 15 language files was not performed |

**Overall confidence:** HIGH

The research was conducted via direct analysis of source files (shared.css 3000+ lines, missions.css, config.css, setup.css, shared.js, embed.go, translation JSON files). Confidence is high for CSS and architecture findings. Medium-high for pitfalls because some issues (translation key completeness across all 15 languages) would require a full automated scan to confirm.

### Gaps to Address

- **CSS architecture decision** -- The research identifies the problem (no architecture) but does not prescribe a solution. During Phase 1 planning, the team must decide between BEM, ITCSS, or a prefixed naming convention (e.g., `.aura-card`, `.aura-badge`). This decision affects every CSS file.
- **Full translation audit** -- The translation pitfalls were identified via spot checks of setup/de.json. A complete scan of all 15 language files is needed before Phase 3 to scope the work accurately.
- **JavaScript modal state management** -- The dual modal system implies complex state. Before Phase 2, audit all `showModal()` calls to understand the full state machine.
- **Animation scope** -- Some animations may be load-bearing for UX (e.g., connection state pills use animation to communicate state). The animation audit must distinguish between decorative and functional animations.

## Sources

### Primary (HIGH confidence)
- `ui/shared.css` (3042 lines) -- Component library, theme system, duplicate keyframes
- `ui/css/tokens.css` (198 lines) -- Design token definitions
- `ui/css/missions.css` (1021 lines) -- Mission Control styles, breakpoint issues
- `ui/css/config.css` -- Config page styles, hardcoded fallbacks
- `ui/css/setup.css` -- Setup wizard styles, toggle duplication
- `ui/shared.js` (1280 lines) -- Cross-page JavaScript
- `ui/embed.go` -- Embedded file manifest

### Secondary (MEDIUM-HIGH confidence)
- `ui/lang/setup/de.json`, `ui/lang/setup/en.json` -- Translation spot checks revealing mixed-language and key mismatch issues
- `ui/setup.html`, `ui/index.html` -- HTML structure patterns, inline translation scripts

### Tertiary (LOW confidence)
- Claims about Chart.js and CodeMirror usage -- CDN references found but usage not traced to code

---
*Research completed: 2026-04-03*
*Ready for roadmap: yes*
