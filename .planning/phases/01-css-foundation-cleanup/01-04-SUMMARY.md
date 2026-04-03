# Phase 1 Plan 4: CSS Specificity Audit Summary

**Plan:** 01-04
**Phase:** CSS Foundation Cleanup
**Completed:** 2026-04-03
**Commit:** 6c2031a

## Objective

Audit shared.css component definitions, identify CSS specificity conflicts between shared.css and page-specific CSS files, document override rules, and fix the worst conflicts.

## Tasks Executed

### Task 1: Audit shared.css Component Classes

**Action:** Grep-based audit of shared.css to identify main component class definitions and their specificity levels.

**Findings:**
- `.card` (line ~631) — single-class selector, low specificity
- `.badge` (line ~845) — single-class selector with variants `.badge-active`, `.badge-inactive`, `.badge-running`, `.badge-warning`, `.badge-hatching`, `.badge-idle`, `.badge-ssh`
- `.btn` (line ~424) — single-class selector with variants `.btn-primary`, `.btn-secondary`, `.btn-danger`, `.btn-success`, `.btn-sm`, `.btn-header`, `.btn-theme`, `.btn-speaker`
- `.pill` (line ~914) — single-class selector
- `.form-group` (line ~1114) — single-class selector
- `.modal` (line ~1350) — single-class selector
- `.status-card`, `.status-label`, `.status-value` — NOT in shared.css; defined only in missions.css

**Conflict identified:** missions.css defines `.badge-priority-*` and `.badge-type-*` badge variants that do NOT extend shared.css `.badge` base class — they are completely standalone. Similarly `.status-card` is entirely separate from `.card`.

### Task 2: Document Override Rules in shared.css

**Action:** Added a "CSS NAMING CONVENTION & OVERRIDE RULES" section to the shared.css header (after line 4), and added a "BASE/ATOMIC STYLES" note at the components section header (line ~227).

**Documentation includes:**
1. **Naming convention** (D-01/D-04): `.aura-*` prefix for new page-specific variants, single-class for base components
2. **Specificity rules** (CSS-03): shared.css uses low-specificity single-class selectors; page CSS should use descendant selectors (`.page-name .component {}`) — avoids specificity wars
3. **Golden rule**: Never re-define base component classes (.card, .badge, .btn) directly in page CSS
4. **Component architecture note** at the components section header explaining these are atomic base styles

### Task 3: Fix Hardcoded Colors in missions.css Badge Definitions

**Action:** Replaced hardcoded RGBA and hex color values with CSS variable references in all mission badge classes.

**Changes in `ui/css/missions.css`:**
- `.badge-priority-low`: `rgba(34,197,94,0.12)` → `rgba(var(--success-rgb,34,197,94),0.12)`
- `.badge-priority-medium`: `#fbbf24` → `var(--warning)`, background/border → CSS vars
- `.badge-priority-high`: `rgba(239,68,68,0.12)` → `rgba(var(--danger-rgb,239,68,68),0.12)`
- `.badge-type-scheduled`: hardcoded `rgba(45,212,191,0.12)` → `rgba(var(--accent-rgb,45,212,191),0.12)`
- `.badge-type-triggered`: `#fbbf24` → `var(--warning)`, background/border → CSS vars
- `.badge-prep-prepared`: hardcoded `rgba(16,185,129,...)` → CSS vars
- `.badge-prep-stale`: `#fbbf24` → `var(--warning)`, background/border → CSS vars
- `.badge-prep-error`: hardcoded `rgba(239,68,68,...)` → CSS vars
- `.badge-prep-low_confidence`: `#fbbf24` → `var(--warning)`, background/border → CSS vars

**Retained hardcoded values (no theme variable available):**
- `.badge-type-manual` and `.badge-prep-preparing`: indigo-400 `#818cf8` (no `--indigo` or similar CSS variable in theme)

## Files Modified

| File | Changes |
|------|---------|
| `ui/shared.css` | +~35 lines: CSS naming convention + override rules section in header; BASE/ATOMIC note at components section |
| `ui/css/missions.css` | +~15 net lines: all badge color values converted to CSS variable references |

## Verification

- `grep "Override Rules" ui/shared.css` → found (line 17)
- `grep "CSS NAMING CONVENTION" ui/shared.css` → found (line 7)
- `grep "#fbbf24" ui/css/missions.css` → only in `.badge-type-manual` (indigo fallback, acceptable)
- All priority, danger, success, warning badge colors now use CSS vars

## Deviations from Plan

**None** — plan executed as written.

## Requirements Covered

| Requirement | Status |
|-------------|--------|
| CSS-03 (resolve CSS specificity conflicts) | Documented in shared.css; worst conflicts identified and fixed |
