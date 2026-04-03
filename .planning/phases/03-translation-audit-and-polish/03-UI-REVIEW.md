# Phase 3 - UI Review

**Audited:** 2026-04-03
**Baseline:** Abstract 6-pillar standards (no UI-SPEC.md present)
**Screenshots:** Not captured (no dev server running)

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 2/4 | German fixed; 7 other languages still have English "Skip" in buttons; plans.html has 2 missing translation keys |
| 2. Visuals | 3/4 | Modal consolidation successful; plans.html uses different modal inner class (.modal-content) than config.html (.modal-card) |
| 3. Color | 3/4 | Modal danger styling consistent; inline hardcoded colors acceptable for one-off cases |
| 4. Typography | 3/4 | Fluid typography (clamp()) consistent; Inter font weights 300-700 applied uniformly |
| 5. Spacing | 3/4 | Consistent spacing patterns in shared.css; vault-confirm-input has appropriate specific spacing |
| 6. Experience Design | 2/4 | Modal activation consistent; missing translations for plans.html placeholders |

**Overall: 16/24**

---

## Top 3 Priority Fixes

1. **Missing plans.html translations** — Users see English placeholders instead of localized text — Create `ui/lang/plans/en.json` with `plans.block_prompt` and `plans.split_placeholder` keys, then add translations for all 15 languages

2. **English "Skip" in 7 languages** — cs, da, el, hi, nl, no, sv have untranslated "Skip ✕" and "Skip setup" in `setup.skip_button` and `setup.skip_button_title` — Translate to native language in each file

3. **Copy-paste error in confirm_skip_setup** — cs, da, no, hi have Czech text "Preskocit nastaveni?" instead of native language — Fix `setup.confirm_skip_setup` in each of these 4 files

---

## Detailed Findings

### Pillar 1: Copywriting (2/4)

**Evidence:**
- German translations fixed in `ui/lang/setup/de.json` — "Schnellkonfiguration", "Sprache", "Benutzerdefiniert" (all proper German)
- 7 languages still have English "Skip" in buttons:
  - `cs.json`: `"setup.skip_button": "Skip ✕"`, `"setup.skip_button_title": "Skip setup"`
  - `da.json`: same English issue
  - `el.json`: same English issue
  - `hi.json`: same English issue
  - `nl.json`: same English issue
  - `no.json`: same English issue
  - `sv.json`: same English issue
- Copy-paste errors: cs, da, no, hi have `"setup.confirm_skip_setup": "Preskocit nastaveni?"` (Czech text) instead of native
- plans.html uses 2 placeholder keys with no language file:
  - `plans.block_prompt` — used at line 75 of plans.html
  - `plans.split_placeholder` — used at line 91 of plans.html
  - `ui/lang/plans/` directory does not exist

**Assessment:** Phase 3 partially completed translation fixes. German is now correct. Remaining 7 languages need skip_button translation. 2 plans.html keys are untranslated.

### Pillar 2: Visuals (3/4)

**Evidence:**
- Modal consolidation successful in config.html lines 98-115:
  - `vault-delete-overlay` now uses `class="modal-overlay"` (was `vault-modal-overlay`)
  - Inner card uses `class="modal-card modal-card-danger"` (was `vault-modal-card`)
  - Title uses `class="modal-title"` with inline `style="color:#f87171"`
- `modal-card-danger` added to shared.css line 1590 with danger border
- Plans.html modals (lines 69-98) use `class="modal-overlay"` but inner content uses `class="modal-content plan-modal"` instead of `modal-card` — inconsistent with config.html pattern

**Assessment:** Modal system consolidated correctly in config.html. Plans.html uses a different inner modal class (`.modal-content`) which is visually/structurally different from the `.modal-card` pattern.

### Pillar 3: Color (3/4)

**Evidence:**
- `modal-card-danger` in shared.css line 1591: `border-color: rgba(239, 68, 68, 0.4)` — consistent with danger theme, acceptable
- `modal-title` in config.html line 101: `style="color:#f87171"` — hardcoded red inline style, acceptable for one-off danger title
- No widespread hardcoded colors found in CSS files — CSS variables used throughout shared.css
- Dark theme CSS variables in shared.css lines 45-94 define all semantic colors properly

**Assessment:** Color usage is consistent. Hardcoded inline colors are acceptable for specific danger cases.

### Pillar 4: Typography (3/4)

**Evidence:**
- Inter font loaded via Google Fonts: `font-family: 'Inter', system-ui, -apple-system, sans-serif`
- Font weights 300-700 applied uniformly across pages
- Fluid typography via `clamp()` in shared.css lines 96-99:
  - `--text-base: clamp(0.875rem, 0.8rem + 0.25vw, 1rem)`
  - `--text-sm: clamp(0.75rem, 0.7rem + 0.2vw, 0.85rem)`
  - `--text-lg: clamp(1.1rem, 1rem + 0.3vw, 1.3rem)`
  - `--text-xs: clamp(0.65rem, 0.6rem + 0.15vw, 0.75rem)`
- No arbitrary font sizes found

**Assessment:** Typography is consistent and uses fluid scaling appropriately.

### Pillar 5: Spacing (3/4)

**Evidence:**
- Consistent spacing via CSS custom properties and Tailwind CDN config
- `vault-confirm-input` in config.css line 981-985 has specific centering/letter-spacing: `text-align: center; letter-spacing: 0.12em; font-size: 1rem;`
- Modal spacing handled by `.modal-card` class in shared.css
- No arbitrary spacing values like `[12px]` or `[1.5rem]` found in the examined CSS

**Assessment:** Spacing is consistent and follows the established scale.

### Pillar 6: Experience Design (2/4)

**Evidence:**
- Modal activation pattern in `ui/js/config/main.js` lines 1312-1343 uses `classList.add('active')` and `classList.remove('active')` — consistent with shared.js initModals() pattern
- Vault delete confirmation still requires typing "DELETE" to enable confirm button — proper destructive action safeguard
- Missing translation keys in plans.html — placeholders show English text instead of translated text when running in non-English locale
- German skip_button properly fixed with "Uberspringen" in de.json

**Assessment:** Modal UX is consistent and secure. Missing translations in plans.html degrade experience for non-English users.

---

## Registry Safety

Registry audit: Not applicable (no shadcn/ui, no third-party component registries detected in Phase 3 changes)

---

## Files Audited

- `ui/lang/setup/de.json` — German translation file (fixed)
- `ui/lang/setup/en.json` — English reference translations
- `ui/config.html` — Vault delete modal HTML (consolidated)
- `ui/css/config.css` — Vault confirm input styling (cleanup)
- `ui/css/shared.css` — Modal patterns and modal-card-danger
- `ui/js/config/main.js` — Modal activation JS (consolidated)
- `ui/plans.html` — Modal with missing translation keys
- `ui/lang/setup/cs.json`, `da.json`, `el.json`, `hi.json`, `nl.json`, `no.json`, `sv.json` — Languages with English "Skip" remaining
