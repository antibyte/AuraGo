# AuraGo UI/UX Overhaul

## What This Is

Systematic improvement of AuraGo's embedded Web UI — fixing layout issues, achieving visual consistency across all pages, and ensuring complete internationalization. The goal is a polished, professional interface that feels cohesive across all areas (Setup, Chat, Dashboard, Config, Missions, etc.).

## Core Value

Every page must be usable, consistent, and translated — no half-finished sections, no orphaned UI elements, no language gaps.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] **UI-01**: Fix Mission Control layout (pills placement, header overflow, button cutoff)
- [ ] **UI-02**: Establish consistent card styling across all pages
- [ ] **UI-03**: Audit and fix Config page layout inconsistencies
- [ ] **UI-04**: Complete translation coverage for all 15 supported languages
- [ ] **UI-05**: Establish consistent spacing/grid system
- [ ] **UI-06**: Fix responsive breakpoints across all pages

### Follow-up Extension

- [ ] **WEB-VERCEL-01**: AuraGo can authenticate against Vercel and manage projects through a dedicated integration
- [ ] **WEB-VERCEL-02**: The `homepage` workflow can deploy a web workspace project to Vercel without manual CLI interaction
- [ ] **WEB-VERCEL-03**: The agent can manage Vercel domains, aliases, and environment variables with permission gates
- [ ] **WEB-VERCEL-04**: The Config UI exposes a Vercel section with vault-backed token handling, status, and connection testing
- [ ] **WEB-VERCEL-05**: Registry, docs, and tests cover the Vercel publishing workflow end to end

### Out of Scope

- Backend functionality changes
- Database schema changes
- New features or functionality

## Context

AuraGo has a single-file SPA UI embedded via `go:embed`. The UI consists of:
- **15+ pages**: setup, login, dashboard, chat, config, missions, containers, media, knowledge, gallery, skills, plans, cheatsheets, invasion_control, truenas
- **Mixed styling approach**: shared.css for common components, page-specific CSS files
- **15 languages**: cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh
- **Current issues**:
  - Mission Control: pills not consistently placed, mission title truncated, buttons overflow container
  - Config area: inconsistent layout and design
  - Translation gaps: some texts untranslated or incorrectly translated
  - Cards waste space or overflow on various screen sizes

Follow-up web publishing work extends this completed UI overhaul with a provider-backed publishing path for the existing homepage tool. The integration should feel native to AuraGo's current web publishing area, reuse the Netlify patterns where they already fit, and keep Vercel credentials inside the vault.

**Existing codebase map**: `.planning/codebase/` documents the full tech stack and architecture.

## Constraints

- **Tech**: Vanilla JS SPA, CSS custom properties (CSS variables), no framework changes
- **Compatibility**: Must maintain dark/light theme support
- **Scope**: The original v1 overhaul is frontend/UI-only; the Vercel follow-up extension explicitly includes backend, config, prompt, and test changes required for provider-backed web publishing
- **Languages**: All 15 languages must have complete, correct translations

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Systematic approach | Many areas affected — fix root causes, not individual symptoms | — Pending |
| CSS variables as source of truth | Consistent theming via custom properties | — Pending |
| Translation audit as separate track | Can run in parallel with layout fixes | — Pending |

---

*Last updated: 2026-04-03 after initialization*
