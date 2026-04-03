# Phase 1: CSS Foundation Cleanup - Context

**Gathered:** 2026-04-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Establish clean CSS architecture for AuraGo's embedded Web UI:
- Extract duplicate keyframes to single animations.css
- Replace all hardcoded colors with CSS variables
- Resolve CSS specificity conflicts between shared.css and page-specific CSS
- Establish CSS naming convention applied consistently

**Canonical refs:**
- `ui/css/tokens.css` — CSS custom properties (design tokens)
- `ui/css/shared.css` — Shared components (~3000 lines, needs splitting)
- `ui/css/missions.css` — Page-specific styles (pills overflow issue)
- `ui/css/config.css` — Hardcoded color fallbacks issue
- `.planning/research/SUMMARY.md` — Research findings on CSS debt

</domain>

<decisions>
## Implementation Decisions

### Naming Convention
- **D-01:** Use **prefixed naming** (`.aura-*`) for all new component classes
  - Rationale: BEM requires rewriting all existing classes; prefixed is pragmatic
  - Examples: `.aura-card`, `.aura-badge`, `.aura-btn`
  - Existing classes (`.card`, `.badge`, `.btn`) remain as-is for backward compatibility
  - New components MUST use `.aura-*` prefix

### File Organization
- **D-02:** Create `ui/css/animations.css` for all keyframe definitions
  - Extract fadeIn, card-enter, pulse, and all other @keyframes from shared.css and page CSS
  - Import animations.css before other CSS files
- **D-03:** Split shared.css into logical sections (via comments, not separate files)
  - Keep file count low to avoid import complexity in go:embed
  - Add section comments: /* === RESET === */, /* === TOKENS === */, etc.

### CSS Specificity Resolution
- **D-04:** Use `.aura-*` prefixed classes for all NEW component variants
  - Existing specificity conflicts resolved by NOT overriding shared.css classes directly
  - New styles either: (a) use `.aura-*` prefix, or (b) extend shared.css classes with modifier pattern

### Hardcoded Color Fixes
- **D-05:** Replace ALL hardcoded color fallbacks (e.g., `var(--warning,#f9a825)`) with proper CSS variable references
  - No fallback colors in variable usage — variables must be defined in tokens.css
  - Exception: Only `transparent` and `currentColor` are acceptable fallback values

### Min-width Fixes
- **D-06:** Replace fixed `min-width` values with `min()`, `clamp()`, or percentage-based values
  - Target problematic values: `.cfg-input-flex { min-width:240px }` in config.css
  - Mobile-first approach: default to fluid, constrain only where needed

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### CSS Architecture
- `ui/css/tokens.css` — Existing CSS custom properties (must use these, not hardcoded colors)
- `ui/css/shared.css` — Current component library (~3000 lines, source of duplication)
- `ui/css/animations.css` (TO BE CREATED) — Centralized keyframes

### Page-Specific CSS
- `ui/css/missions.css` — Mission Control styles (LAY-01, LAY-02, LAY-03)
- `ui/css/config.css` — Config page styles (hardcoded color fallbacks, fixed min-width)
- `ui/css/setup.css` — Setup wizard styles (toggle duplication)

### Research
- `.planning/research/SUMMARY.md` — Full research findings
- `.planning/research/STACK.md` — CSS stack analysis
- `.planning/research/PITFALLS.md` — CSS pitfalls catalog

</canonical_refs>

<codebase_context>
## Existing Code Insights

### Reusable Assets
- `tokens.css` already defines comprehensive CSS custom properties — use these, don't add new ones
- `shared.css` has well-designed base components — extend, don't replace

### Established Patterns
- Glassmorphism design language: `backdrop-filter: blur()`, semi-transparent backgrounds
- Dual theme: `[data-theme="dark"]` / `[data-theme="light"]` on `:root`
- Component classes: `.card`, `.btn`, `.badge`, `.modal`, `.form-*` already exist

### Integration Points
- go:embed in `ui/embed.go` — CSS files must be listed for embedding
- All HTML pages reference shared.css and tokens.css
- Theme toggle in `shared.js` — CSS variable changes via attribute selector

</codebase_context>

<specifics>
## Specific Ideas

No specific design references given. Open to standard approaches for:
- Keyframe naming conventions (e.g., `.fade-in` vs `fadeIn`)
- Breakpoint scale (use existing from research: 480px, 640px, 768px)
- Animation duration conventions

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-css-foundation-cleanup*
*Context gathered: 2026-04-03*
