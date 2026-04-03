# Phase 3: Translation Audit and Polish - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-03
**Phase:** 3-translation-audit-and-polish
**Areas discussed:** Translation Audit Approach, Mixed-Language Fixes, Modal Consolidation, Translation Consistency

---

## Translation Audit Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Systematic key comparison | Compare all HTML data-i18n keys against all 15 language files | ✓ |
| Fix by priority | German first, then other high-usage languages | |
| Fix by section | Auth, Config, Setup sections independently | |

**User's choice:** [auto] Systematic key comparison across all 15 languages
**Notes:** [auto] Most thorough approach

---

## Mixed-Language Fixes

| Option | Description | Selected |
|--------|-------------|----------|
| Replace all English in non-English files | Every language file pure target language | ✓ |
| Keep multilingual labels | Some labels intentionally mixed (e.g., "Language / Sprache") | |

**User's choice:** [auto] All language files pure target language
**Notes:** [auto] Clarity and consistency

---

## Modal Consolidation

| Option | Description | Selected |
|--------|-------------|----------|
| Consolidate to modal-overlay + modal-card | Standard pattern from shared.css | ✓ |
| Keep both patterns | Maintain dual system | |
| Remove all modals, use native dialog | Too invasive | |

**User's choice:** [auto] Consolidate to standard modal-overlay + modal-card pattern
**Notes:** [auto] Phase 2 already unified components

---

## Translation Consistency

| Option | Description | Selected |
|--------|-------------|----------|
| Verify placeholder lengths | Check no UI overflow from long translations | ✓ |
| Ignore length variations | Accept differences | |

**User's choice:** [auto] Verify placeholder lengths for UI consistency
**Notes:** [auto] Prevents UI breaks

---

## Claude's Discretion

All areas auto-resolved with recommended defaults.

