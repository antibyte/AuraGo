# Phase 2: Component Unification and Responsive Fixes - Context

**Gathered:** 2026-04-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Fix user-facing UI issues across all pages:
- Mission Control overflow/cutoff (pills, titles, buttons)
- Unified toggle implementation pattern
- Consistent responsive breakpoint scale
- Consistent card grid system

**Canonical refs:**
- `ui/css/missions.css` — Mission Control styles (LAY-01, LAY-02, LAY-03)
- `ui/css/config.css` — Config page styles
- `ui/css/shared.css` — Toggle implementation
- `.planning/research/SUMMARY.md` — Research findings
- `.planning/phases/01-css-foundation-cleanup/01-CONTEXT.md` — Phase 1 decisions

</domain>

<decisions>
## Implementation Decisions

### Mission Control Layout
- **D-07:** Fix pills overflow — use `overflow-wrap: break-word` and `min-width: 0` on pill containers
- **D-08:** Fix title truncation — ensure `.mission-name` has `overflow: hidden`, `text-overflow: ellipsis` with proper `min-width: 0` on flex parent
- **D-09:** Fix button cutoff — use `flex-shrink: 0` on action buttons, ensure `.mission-actions` doesn't overflow card width
- **D-10:** Badge consistency — Mission Control badges should use same `.aura-badge` classes as Dashboard

### Toggle Implementation
- **D-11:** Consolidate to single toggle pattern — use checkbox-based toggles (`.toggle` class from shared.css) as the single implementation
- **D-12:** Remove class-based `.on` toggle pattern from config.css — convert to checkbox-based `.toggle`
- **D-13:** Ensure all toggle switches have proper `for` attribute linking to hidden checkbox

### Responsive Breakpoints
- **D-14:** Establish unified breakpoint scale: 480px, 640px, 768px, 1024px (from research)
- **D-15:** Replace all page-specific breakpoint values with the unified scale
- **D-16:** Status bar: use `repeat(auto-fit, minmax(200px, 1fr))` instead of forced 4-column `repeat(4, minmax(0, 1fr))`

### Card Grid System
- **D-17:** Use CSS Grid with `auto-fill` and `minmax()` for fluid card layouts
- **D-18:** Consistent card gap: 20px across all pages
- **D-19:** Cards use `.aura-card` class (from Phase 1 naming convention) for consistent styling

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 1 Context
- `.planning/phases/01-css-foundation-cleanup/01-CONTEXT.md` — Naming convention, specificity rules

### CSS Files
- `ui/css/missions.css` — Mission Control layout issues (pills, titles, buttons)
- `ui/css/config.css` — Toggle duplication, fixed min-widths
- `ui/css/setup.css` — Toggle patterns
- `ui/shared.css` — `.toggle` class definition, card, badge base styles

### Research
- `.planning/research/SUMMARY.md` — Full research findings
- `.planning/research/PITFALLS.md` — Breakpoint inconsistencies, toggle duplication

</canonical_refs>

<codebase_context>
## Existing Code Insights

### Reusable Assets
- `.toggle` class in shared.css — checkbox-based toggle (use as standard)
- `.card` base class in shared.css
- `.badge` base class in shared.css

### Established Patterns
- Glassmorphism: `backdrop-filter: blur()`, semi-transparent backgrounds
- Phase 1 `.aura-*` prefix convention for new components

### Integration Points
- go:embed in `ui/embed.go` — CSS files listed for embedding
- Theme toggle in `shared.js` — CSS variable changes via attribute selector

</codebase_context>

<specifics>
## Specific Ideas

**Mission Control Fixes (from user feedback):**
- Pills unevenly placed → ensure consistent `flex-wrap: wrap` and gap
- Mission heading cut off → `.mission-name { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }`
- Buttons overflow card → `.mission-actions { flex-shrink: 0; }` and parent `min-width: 0`

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-component-unification-and-responsive-fixes*
*Context gathered: 2026-04-03*
