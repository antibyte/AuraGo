---
phase: 03-translation-audit-and-polish
plan: A
subsystem: ui
tags: [i18n, translations, audit, json]

# Dependency graph
requires: []
provides:
  - "Audit report: All 15 setup languages have 100% key coverage (151 keys each)"
  - "Audit report: All 15 config languages have 100% key coverage (9 keys each)"
  - "Audit report: 13 of 14 HTML pages have complete i18n coverage"
  - "Gap identified: plans.html has 2 placeholder keys missing from all language files"
affects: [03-B, 03-C, 03-D]

# Tech tracking
tech-stack:
  added: []
  patterns: [Python-based JSON audit script]

key-files:
  created: []
  modified: []

key-decisions:
  - "Split audit by language section (setup vs config) to avoid false cross-contamination"
  - "plans.html maps to common/ directory (login.html and plans.html share common keys)"

patterns-established: []

requirements-completed: [I18N-01, I18N-04]

# Metrics
duration: 3min
completed: 2026-04-03
---

# Phase 3 Plan A: Translation Audit Summary

**Complete i18n audit: All 16 languages (15 + en) have 100% coverage for setup (151 keys) and config (9 keys). Two placeholder keys in plans.html have no language file.**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-03T16:27:01Z
- **Completed:** 2026-04-03T16:30:11Z
- **Tasks:** 3
- **Files analyzed:** 32 language JSON files + 14 HTML files

## Accomplishments

- Extracted complete English reference key set: 160 keys (151 setup + 9 config)
- Verified all 15 language files for setup and config sections are 100% complete
- Cross-referenced all HTML data-i18n attributes against language files
- Identified gap: plans.html uses 2 placeholder keys that have no corresponding language file

## Task Analysis

This is an audit-only plan. All tasks were read-only analysis with no file modifications.

**Task 1: Extract all translation keys from English reference**
- Extracted 160 unique keys (151 setup + 9 config) from en.json files
- No files modified

**Task 2: Compare all 15 language files for missing keys**
- All 15 setup language files: 100% coverage (151/151 keys each)
- All 15 config language files: 100% coverage (9/9 keys each)
- No files modified

**Task 3: Identify data-i18n usage in HTML files**
- Found 51 unique i18n keys across 14 HTML files
- 13 HTML pages have complete language coverage
- plans.html has 2 missing keys: `plans.block_prompt`, `plans.split_placeholder`
- No files modified

## Key Audit Findings

### Setup/Config Language Coverage (all 16 languages including English)
| Language | Setup Keys | Config Keys | Status |
|----------|-----------|-------------|--------|
| All 16 (cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh) | 151/151 | 9/9 | COMPLETE |

### HTML Page Coverage
| Page | Keys Used | Covered | Notes |
|------|-----------|---------|-------|
| setup.html | 4 | YES | setup/ language file |
| config.html | 1 | YES | config/ language file |
| knowledge.html | 16 | YES | knowledge/ language file |
| skills.html | 14 | YES | skills/ language file |
| dashboard.html | 8 | YES | dashboard/ language file |
| invasion_control.html | 8 | YES | invasion/ language file |
| containers.html | 2 | YES | containers/ language file |
| media.html | 2 | YES | media/ language file |
| gallery.html | 2 | YES | gallery/ language file |
| cheatsheets.html | 1 | YES | cheatsheets/ language file |
| truenas.html | 1 | YES | truenas/ language file |
| login.html | 1 | YES | common/ language file |
| plans.html | 3 | PARTIAL | 1 key in common/, 2 keys MISSING |
| index.html | 0 | N/A | chat page |

### Gap Details
**plans.html** uses 2 placeholder keys not present in any language file:
- `plans.block_prompt` - used in blocker modal textarea (line 75 of plans.html)
- `plans.split_placeholder` - used in split modal textarea (line 91 of plans.html)

**Missing language directory**: `ui/lang/plans/` does not exist.

## Decisions Made

- Split audit into setup vs config sections to avoid false cross-contamination (setup files should not have config keys and vice versa)
- Used corrected HTML-to-language-directory mapping (e.g., `invasion_control.html` -> `invasion/` directory, not `invasion_control/`)

## Deviations from Plan

None - plan executed exactly as written. This was an audit-only plan with no code modifications.

## Issues Encountered

None - all verification scripts ran successfully.

## Audit Recommendations

1. **plans.html gap**: Create `ui/lang/plans/en.json` with `plans.block_prompt` and `plans.split_placeholder` keys, then add translations for all 15 languages
2. **Login language directory**: `ui/lang/login/` does not exist but login.html only uses `common.toggle_theme` which is in `common/en.json` - functionally OK but a dedicated login/ directory would be cleaner
3. **Cross-language contamination**: The earlier audit showed apparent "missing keys" when comparing all setup files against ALL 160 English keys (setup + config combined) - this was expected since setup files only have setup keys and config files only have config keys

## Next Phase Readiness

- Setup and config translations are fully complete across all 15 languages
- Ready to proceed with Plans B, C, D which may address the plans.html gap or other translation work
- No blockers identified

---
*Phase: 03-translation-audit-and-polish - Plan A*
*Completed: 2026-04-03*
