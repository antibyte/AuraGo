# Roadmap: AuraGo UI/UX Overhaul

**Milestone:** v1
**Granularity:** Standard
**Model:** sonnet
**Created:** 2026-04-03

## Phases

- [x] **Phase 1: CSS Foundation Cleanup** — Establish clean CSS architecture (keyframes, variables, specificity, naming)
- [ ] **Phase 2: Component Unification and Responsive Fixes** (1/4) — Fix Mission Control, unify components, establish consistent breakpoints
- [ ] **Phase 3: Translation Audit and Polish** — Complete i18n coverage, consolidate modals, final polish

---

## Phase Details

### Phase 1: CSS Foundation Cleanup

**Goal:** AuraGo has a clean, maintainable CSS architecture with no duplication, no hardcoded colors, and resolved specificity conflicts.

**Depends on:** None

**Requirements:** CSS-01, CSS-02, CSS-03, CSS-04

**Success Criteria** (what must be TRUE):
1. All duplicate keyframe definitions (fadeIn, card-enter, pulse) exist in a single animations.css file and are not duplicated elsewhere
2. All hardcoded color values in CSS files are replaced with CSS variable references (no `#f9a825` or similar without var() wrapper)
3. CSS specificity conflicts between shared.css and page-specific CSS files are resolved (specific component overrides work predictably)
4. A consistent CSS naming convention is established and applied across all CSS files

**Plans:** 4/4 plans complete
Plans:
- [x] 01-01-PLAN.md -- Extract and centralize all @keyframes to animations.css
- [x] 01-02-PLAN.md -- Replace hardcoded color fallbacks and fixed min-widths
- [x] 01-03-PLAN.md -- Document CSS naming convention in shared.css header
- [x] 01-04-PLAN.md -- Audit and document CSS specificity override rules

---

### Phase 2: Component Unification and Responsive Fixes

**Goal:** All pages render consistently with unified components, proper responsive behavior, and no overflow or cutoff issues.

**Depends on:** Phase 1

**Requirements:** LAY-01, LAY-02, LAY-03, LAY-04, LAY-05, CONS-01, CONS-02, CONS-04, CONS-05

**Success Criteria** (what must be TRUE):
1. Mission Control pills/badges do not overflow card boundaries and render consistently with Dashboard badges
2. Mission Control mission headings are fully visible (not truncated) and action buttons fit within their containers
3. A consistent card grid system is used across all pages with no wasted space on large screens and no overflow on small screens
4. Status bar uses a responsive layout that adapts from 4 columns on large screens to fewer columns on small screens without breaking
5. A single toggle implementation pattern is used consistently across all pages (no duplicate checkbox vs class-based patterns)
6. A single responsive breakpoint scale is applied consistently across all CSS files

**Plans:** 3/4 plans executed
Plans:
- [x] 02-01-PLAN.md -- Mission Control layout fixes (LAY-01, LAY-02, LAY-03)
- [x] 02-02-PLAN.md -- Unified card grid system (LAY-04)
- [ ] 02-03-PLAN.md -- Toggle and badge consolidation (CONS-01, CONS-04)
- [x] 02-04-PLAN.md -- Responsive breakpoints and status bar (CONS-05, LAY-05)

---

### Phase 3: Translation Audit and Polish

**Goal:** All 15 languages have complete, correct translations and the modal system is fully consolidated.

**Depends on:** Phase 2

**Requirements:** CONS-03, I18N-01, I18N-02, I18N-03, I18N-04

**Success Criteria** (what must be TRUE):
1. Every translation key used in any HTML file has a corresponding entry in all 15 language JSON files (no missing keys)
2. All mixed-language entries are corrected (e.g., English strings in German translations are replaced with proper German)
3. All incorrect key references are fixed (e.g., setup.language_custom points to the correct key)
4. A single unified modal pattern (modal-overlay + modal-card) is used consistently across all pages, replacing any vault-modal-* variants
5. All 15 languages have translations that are consistent in length and tone (no clipped or overlapping UI text)

**Plans:** TBD

---

## Progress Table

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. CSS Foundation Cleanup | 4/4 | Complete   | 2026-04-03 |
| 2. Component Unification and Responsive Fixes | 3/4 | In Progress|  |
| 3. Translation Audit and Polish | 0/5 | Not started | - |

---

## Coverage

**Requirements:** 18 v1 requirements

| Requirement | Phase |
|-------------|-------|
| CSS-01 | Phase 1 |
| CSS-02 | Phase 1 |
| CSS-03 | Phase 1 |
| CSS-04 | Phase 1 |
| LAY-01 | Phase 2 |
| LAY-02 | Phase 2 |
| LAY-03 | Phase 2 |
| LAY-04 | Phase 2 |
| LAY-05 | Phase 2 |
| CONS-01 | Phase 2 |
| CONS-02 | Phase 2 |
| CONS-03 | Phase 3 |
| CONS-04 | Phase 2 |
| CONS-05 | Phase 2 |
| I18N-01 | Phase 3 |
| I18N-02 | Phase 3 |
| I18N-03 | Phase 3 |
| I18N-04 | Phase 3 |

**Mapped:** 18/18 v1 requirements
**Unmapped:** 0
