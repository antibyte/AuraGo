# Requirements: AuraGo UI/UX Overhaul

**Defined:** 2026-04-03
**Core Value:** Every page must be usable, consistent, and translated — no half-finished sections, no orphaned UI elements, no language gaps.

## v1 Requirements

### Layout & Cards

- [x] **LAY-01**: Fix Mission Control pills placement — badges/badges must not overflow card boundaries
- [x] **LAY-02**: Fix Mission Control title overflow — mission headings must not be truncated
- [x] **LAY-03**: Fix Mission Control button cutoff — action buttons must not overflow card/container
- [x] **LAY-04**: Establish consistent card grid system across all pages (no wasted space, no overflow)
- [ ] **LAY-05**: Fix status bar responsive layout — 4-column forced grid breaks on small screens

### Consistency

- [ ] **CONS-01**: Unify badge/pill component classes across all CSS files
- [ ] **CONS-02**: Replace all hardcoded colors with CSS variables (no `var(--warning,#f9a825)` fallbacks)
- [ ] **CONS-03**: Consolidate modal systems (modal-overlay, modal-card, vault-modal) into unified pattern
- [ ] **CONS-04**: Unify toggle implementation — single pattern across all pages (no checkbox vs class-based duplication)
- [ ] **CONS-05**: Establish consistent responsive breakpoint scale across all CSS files

### Translation

- [ ] **I18N-01**: Audit all 15 language files for missing translation keys
- [x] **I18N-02**: Fix mixed-language entries (e.g., `setup.header_subtitle: "Quick Setup"` in German)
- [x] **I18N-03**: Fix incorrect key references (e.g., `setup.language_custom` referencing wrong key)
- [ ] **I18N-04**: Ensure all 15 languages have complete, consistent translations

### CSS Architecture

- [x] **CSS-01**: Extract duplicate keyframe definitions (fadeIn, card-enter, pulse) to single animations.css
- [ ] **CSS-02**: Audit and fix fixed pixel min-widths that break on small viewports
- [ ] **CSS-03**: Resolve CSS specificity conflicts between shared.css and page-specific CSS
- [x] **CSS-04**: Establish naming convention for component classes (BEM or similar)

## v2 Requirements

### Polish

- **POL-01**: Add skeleton loading states to cards
- **POL-02**: Animation audit — ensure consistent easing/transition curves
- **POL-03**: Accessibility audit (focus states, ARIA labels)
- **POL-04**: Polish radial menu and mood widget

### Future

- **FUT-01**: Component library documentation
- **FUT-02**: Drag-and-drop extension for card reordering

## Post-v1 Extension Requirements

### Web Publishing

- [ ] **WEB-VERCEL-01**: Add a dedicated Vercel integration with token-based authentication, team selection support, and project lookup/create/update flows
- [ ] **WEB-VERCEL-02**: Add `homepage.deploy_vercel` so AuraGo can build a homepage project and publish it to Vercel from the homepage workspace
- [ ] **WEB-VERCEL-03**: Add permission-gated management for Vercel domains, aliases, and environment variables
- [ ] **WEB-VERCEL-04**: Add a Config UI section for Vercel with vault-backed token storage, connection diagnostics, and default project/team settings
- [ ] **WEB-VERCEL-05**: Update registry logging, manuals, prompts, translations, and tests so the Vercel workflow is documented and verifiable

## Out of Scope

The table below applies to the original UI/UX overhaul track. The Phase 4 Vercel follow-up intentionally adds backend/config/tooling work to support web publishing.

| Feature | Reason |
|---------|--------|
| Backend functionality changes | UI-only project |
| Database schema changes | Out of scope |
| New features | Focus is consistency and polish, not new functionality |
| True SPA migration | Multi-page SPA architecture is sufficient |
| CSS bundling/minification | Build optimization, not UI consistency |
| Mobile app | Web-only for now |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| LAY-01 | Phase 2 | Complete |
| LAY-02 | Phase 2 | Complete |
| LAY-03 | Phase 2 | Complete |
| LAY-04 | Phase 2 | Complete |
| LAY-05 | Phase 2 | Pending |
| CONS-01 | Phase 2 | Pending |
| CONS-02 | Phase 2 | Pending |
| CONS-03 | Phase 3 | Pending |
| CONS-04 | Phase 2 | Pending |
| CONS-05 | Phase 2 | Pending |
| I18N-01 | Phase 3 | Pending |
| I18N-02 | Phase 3 | Complete |
| I18N-03 | Phase 3 | Complete |
| I18N-04 | Phase 3 | Pending |
| CSS-01 | Phase 1 | Complete |
| CSS-02 | Phase 1 | Pending |
| CSS-03 | Phase 1 | Pending |
| CSS-04 | Phase 1 | Complete |
| WEB-VERCEL-01 | Phase 4 | Pending |
| WEB-VERCEL-02 | Phase 4 | Pending |
| WEB-VERCEL-03 | Phase 4 | Pending |
| WEB-VERCEL-04 | Phase 4 | Pending |
| WEB-VERCEL-05 | Phase 4 | Pending |

**Coverage:**
- v1 requirements: 18 total
- Mapped to phases: 18
- Unmapped: 0 ✓

---
*Requirements defined: 2026-04-03*
*Last updated: 2026-04-03 after initial definition*
