# Phase 3: Translation Audit and Polish - Context

**Gathered:** 2026-04-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Complete i18n audit and fix across all 15 languages:
- Every translation key has entries in all language files (no missing keys)
- Mixed-language entries corrected (English strings in German → proper German)
- Incorrect key references fixed
- Modal system consolidated to single pattern
- Translation consistency in length and tone

**Canonical refs:**
- `ui/lang/` — Translation JSON files structure
- `ui/shared.js` — Translation function `t()` usage
- `.planning/research/SUMMARY.md` — Research findings on translation issues
- `.planning/research/PITFALLS.md` — Translation pitfalls catalog

</domain>

<decisions>
## Implementation Decisions

### Translation Audit Approach
- **D-20:** Systematic key audit — compare all keys in HTML files against all 15 language JSON files
- **D-21:** German first — fix the most-used language first (known issues: mixed-language entries, wrong key references)
- **D-22:** Build complete key list from HTML `data-i18n` attributes and `t()` function calls

### Mixed-Language Fixes
- **D-23:** All language files should contain only the target language — no English strings in German files
- **D-24:** Create script to detect mixed-language entries (string contains English words in non-English context)

### Key Reference Fixes
- **D-25:** Fix `setup.language_custom` key reference (was pointing to wrong key)
- **D-26:** All `document.write(t(...))` patterns should be replaced with proper `data-i18n` attributes

### Modal Consolidation
- **D-27:** Consolidate to `modal-overlay + modal-card` pattern (Phase 2 established unified components)
- **D-28:** Remove `vault-modal-*` variant patterns — convert to standard modal
- **D-29:** Verify all modal triggers use consistent `showModal()` calls from shared.js

### Translation Consistency
- **D-30:** All 15 languages should have consistent key structure
- **D-31:** Placeholder lengths should be similar across languages (no UI overflow from long translations)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Translation Files
- `ui/lang/setup/de.json` — Known issues with mixed-language entries
- `ui/lang/setup/en.json` — Reference for English keys
- `ui/shared.js` — `t()` function and translation injection

### Research
- `.planning/research/SUMMARY.md` — Full research findings
- `.planning/research/PITFALLS.md` — Translation pitfalls (mixed language, wrong keys)

### Modal Code
- `ui/shared.js` — `showModal()` function
- `ui/index.html` — Modal usage patterns

</canonical_refs>

<codebase_context>
## Existing Code Insights

### Translation System
- Server-side i18n: Go injects translation JSON at render time
- `data-i18n` attributes and `t()` function for frontend
- 15 languages: cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh

### Modal Patterns
- `modal-overlay` + `modal-card` (standard pattern)
- `vault-modal-*` (legacy, to be removed)
- `showModal()` in shared.js

</codebase_context>

<specifics>
## Specific Ideas

**Known issues from research:**
- German file has `setup.header_subtitle: "Quick Setup"` (English in German)
- `setup.step0_language_label: "Language / Sprache"` (mixed German/English)
- `setup.language_custom` references wrong key `setup.step0_provider_custom`
- Some HTML files use `document.write(t(...))` instead of `data-i18n`

</specifics>

<deferred>
## Deferred Ideas

- Translation CI linting — add to build process (Phase 3 polish only, not in scope)

</deferred>

---

*Phase: 03-translation-audit-and-polish*
*Context gathered: 2026-04-03*
