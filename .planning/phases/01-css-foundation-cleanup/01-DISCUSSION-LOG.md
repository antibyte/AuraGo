# Phase 1: CSS Foundation Cleanup - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-03
**Phase:** 1-css-foundation-cleanup
**Areas discussed:** Naming Convention, File Organization, Hardcoded Color Fixes, Min-width Fixes

---

## Naming Convention

| Option | Description | Selected |
|--------|-------------|----------|
| BEM | `.card__title`, `.card--featured` — requires rewriting all existing classes | |
| Prefixed (`.aura-*`) | `.aura-card`, `.aura-badge` — pragmatic, clear namespace | ✓ |
| ITCSS | Full architecture refactor with components, utilities, settings | |

**User's choice:** Prefixed naming (`.aura-*`) — pragmatic approach
**Notes:** [auto] Selected recommended default

---

## File Organization

| Option | Description | Selected |
|--------|-------------|----------|
| Create animations.css + split shared.css | Centralized keyframes, logical sections in shared.css | ✓ |
| Full ITCSS refactor | Components/utilities/settings layers | |
| Merge into tokens.css | Put everything in tokens.css | |

**User's choice:** Create animations.css + organize shared.css with section comments
**Notes:** [auto] Selected recommended default

---

## Hardcoded Color Fixes

| Option | Description | Selected |
|--------|-------------|----------|
| Replace all fallbacks | No `var(--x,#color)` patterns anywhere | ✓ |
| Only critical ones | Fix worst offenders, leave others | |

**User's choice:** Replace ALL hardcoded color fallbacks
**Notes:** [auto] Selected recommended default

---

## Min-width Fixes

| Option | Description | Selected |
|--------|-------------|----------|
| Fluid-first with min()/clamp() | Replace fixed values with fluid equivalents | ✓ |
| Remove constraints | Let content determine size | |

**User's choice:** Use fluid values (min(), clamp(), %) — mobile-first approach
**Notes:** [auto] Selected recommended default

---

## Claude's Discretion

Areas where auto-resolve selected recommended defaults:
- Naming convention: `.aura-*` prefix for new components
- File organization: animations.css + section-commented shared.css
- Keyframe extraction: single animations.css
- Min-width: fluid values replacing fixed pixels

