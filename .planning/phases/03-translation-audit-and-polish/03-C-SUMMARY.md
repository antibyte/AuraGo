---
phase: "03-translation-audit-and-polish"
plan: "C"
subsystem: "ui"
tags:
  - "modal"
  - "consolidation"
  - "vault"
  - "css"
dependency_graph:
  requires: []
  provides:
    - "vault-delete-overlay uses standard modal-overlay + modal-card"
  affects:
    - "ui/config.html"
    - "ui/js/config/main.js"
    - "ui/css/config.css"
    - "ui/shared.css"
tech_stack:
  added:
    - "modal-card-danger CSS class in shared.css"
  patterns:
    - "Standard modal-overlay + modal-card pattern for vault delete confirmation"
key_files:
  created: []
  modified:
    - "ui/config.html"
    - "ui/js/config/main.js"
    - "ui/css/config.css"
    - "ui/shared.css"
decisions:
  - "Converted vault-modal-overlay to modal-overlay (standard pattern)"
  - "Converted vault-modal-card to modal-card + modal-card-danger"
  - "JS modal activation uses classList.add/remove('active') matching shared.js initModals()"
  - "DELETE word input styling preserved via .vault-confirm-input class"
metrics:
  duration: 199
  tasks_completed: 3
  files_modified: 4
  commits: 3
---

# Phase 03 Plan C: Vault Delete Modal Consolidation

## One-liner

Converted vault-delete-overlay from custom vault-modal-* pattern to standard modal-overlay + modal-card with classList.add/remove('active') activation.

## Completed Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Read vault-modal implementation | - | ui/config.html, ui/js/config/main.js |
| 2 | Convert vault-delete-overlay HTML to standard modal pattern | e93c795 | ui/config.html |
| 3 | Update JS to use classList.add/remove('active') | c5bc215 | ui/js/config/main.js |
| 4 | Clean up vault-modal CSS + add modal-card-danger | 048cf81 | ui/css/config.css, ui/shared.css |

## What Was Built

**Consolidated the vault delete confirmation modal** from a custom vault-modal-* pattern to the standard modal-overlay + modal-card pattern established in Phase 2:

### HTML (ui/config.html)
- `vault-modal-overlay` replaced with `modal-overlay` class
- `vault-modal-card` replaced with `modal-card modal-card-danger` classes
- Title element changed from `vault-modal-title` to `modal-title` (with inline danger color)
- Description moved inside `modal-message` div with `modal-desc` class
- Buttons changed from `vault-btn vault-btn-cancel/confirm` to `btn btn-secondary/btn-danger`

### JavaScript (ui/js/config/main.js)
- `vaultDeletePrompt()`: now uses `classList.add('active')` instead of `classList.remove('is-hidden')`
- `vaultDeleteCancel()`: now uses `classList.remove('active')` instead of `classList.add('is-hidden')`
- `vaultDeleteConfirm()`: now uses `classList.remove('active')` on success
- DELETE word confirmation logic (`vaultCheckWord()`) preserved unchanged

### CSS (ui/css/config.css)
- Removed: `.vault-modal-overlay`, `.vault-modal-card`, `.vault-modal-icon`, `.vault-modal-title`, `.vault-modal-desc`, `.vault-modal-actions`, `.vault-btn`, `.vault-btn-cancel`, `.vault-btn-confirm`
- Kept: `.vault-confirm-input` (centered, letter-spaced input styling still needed)
- Light theme override updated: `.vault-modal-overlay` -> `#vault-delete-overlay`

### CSS (ui/shared.css)
- Added `.modal-card-danger { border-color: rgba(239,68,68,0.4); }` for danger modal variant

## Must-Haves Verification

- [x] All vault-modal-* patterns replaced with standard modal-overlay + modal-card
- [x] Delete confirmation modal uses consistent modal pattern
- [x] vault-delete-overlay uses modal-overlay + modal-card classes
- [x] Delete confirmation still requires typing "DELETE" to confirm
- [x] Modal opens/closes correctly via JS classList.add/remove('active')

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check: PASSED

- [x] ui/config.html uses modal-overlay + modal-card
- [x] ui/js/config/main.js uses classList.add/remove('active')
- [x] ui/css/config.css vault-modal rules removed (except .vault-confirm-input)
- [x] ui/shared.css has modal-card-danger class
- [x] All 3 commits present: e93c795, c5bc215, 048cf81
