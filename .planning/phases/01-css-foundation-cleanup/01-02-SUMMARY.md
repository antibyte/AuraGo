# Phase 1 Plan 2: CSS Color Fallbacks and Fluid Min-Width Summary

**Plan:** 01-02
**Phase:** CSS Foundation Cleanup
**Completed:** 2026-04-03
**Commit:** c7a3ba6

## Objective

Replace all hardcoded color fallbacks in CSS files (particularly `var(--warning,#f9a825)` patterns in config.css) with proper CSS variable references, and replace fixed min-width values with fluid equivalents.

## Tasks Executed

### Task 1: Fix hardcoded color fallbacks in config.css

**Action:** Found and removed all `var(--variable,#hex)` fallback patterns from config.css. Added missing `--warning-bg` CSS variable to shared.css since it was referenced but not defined.

**Changes in `ui/shared.css`:**
- Added `--warning-bg: rgba(234, 179, 8, 0.15);` in dark theme `:root` block (after `--warning`)

**Changes in `ui/css/config.css`:**
- `.wh-notice-warning`: `var(--warning-bg, rgba(...))` and `var(--warning,#f9a825)` → `var(--warning-bg)` and `var(--warning)`
- `.ts-warning-box`: Removed all `#3d2e00` and `#f9a825` fallbacks from background, border-color, and color
- `.ts-color-warning`: Removed `#f9a825` fallback
- `.ts-warning-detail`: Removed all `#3d2e00` and `#f9a825` fallbacks
- `.ts-login-banner`: Removed `#3d2e00` and `#f9a825` fallbacks
- `.ts-login-title`: Removed `#f9a825` fallback
- `.em-badge-danger`: `var(--danger,#e55)` → `var(--danger)` (color #fff retained as direct value)
- `.em-badge-warning`: `var(--warning,#f90)` → `var(--warning)` (color #000 retained as direct value)

### Task 2: Fix .cfg-input-flex and .cfg-password-input min-width

**Action:** Replaced fixed `min-width:240px` with `min-width:0` for flex child elements, since flex items naturally collapse to content minimum and `flex:1` already provides grow behavior.

**Changes in `ui/css/config.css`:**
- `.cfg-input-flex`: `min-width:240px` → `min-width:0`
- `.cfg-password-input`: `min-width:240px` → `min-width:0`

### Task 3: Fix var() fallbacks in chat-modules.css and chat.css

**Action:** Scanned both files for `var(--x,#color)` fallback patterns.

**Findings:**
- `chat-modules.css`: No `var(...,#...)` patterns found — file already compliant
- `chat.css`: One occurrence found and fixed

**Changes in `ui/css/chat.css`:**
- `.voice-recorder.paused .recorder-pulse`: `var(--warning, #f59e0b)` → `var(--warning)`

## Files Modified

| File | Changes |
|------|---------|
| `ui/shared.css` | +1 line: Added `--warning-bg: rgba(234, 179, 8, 0.15);` to dark theme |
| `ui/css/config.css` | 11 replacements: removed hardcoded color fallbacks from 7 selectors; fixed 2 min-width rules |
| `ui/css/chat.css` | 1 replacement: removed hardcoded color fallback from recorder pulse |

## Verification

| Check | Result |
|-------|--------|
| `grep 'var([^)]*,#' ui/css/config.css` | 0 matches |
| `grep 'var([^)]*,#' ui/css/chat.css` | 0 matches |
| `grep 'var([^)]*,#' ui/css/chat-modules.css` | 0 matches (already clean) |
| `grep 'min-width:240px' ui/css/config.css` | 0 matches |

## Deviations from Plan

**None** — plan executed as written.

## Deviations from Plan (Auto-fixed Issues)

No auto-fixes were required; all issues were identified in the plan context and addressed directly.

## Requirements Covered

| Requirement | Status |
|-------------|--------|
| CSS-02 (fixed pixel min-widths) | Fixed: .cfg-input-flex and .cfg-password-input now use min-width:0 |
| CONS-02 (hardcoded color fallbacks) | Fixed: all var(--warning,#f9a825), var(--danger,#e55), var(--warning,#f90), and similar patterns removed |

## Notes

- `chat-modules.css` was already compliant with no `var(...,#...)` fallback patterns — no changes needed for that file
- `--warning-bg` was added to `shared.css` with the same value as `--warning-dim` from `tokens.css` (`rgba(234, 179, 8, 0.15)`) per the plan's specification
- Badge `.em-badge-warning` and `.em-badge-danger` retain their direct color values (#000, #fff) for text color as those are intentional design values, not var() fallbacks
