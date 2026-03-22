# Web UI Scorecards – Phase 1.1 (Detailed)

This document continues Phase 1 with concrete per-page scorecards for the first three target pages:
- `ui/index.html`
- `ui/config.html`
- `ui/dashboard.html`

Scoring scale per criterion:
- 0–1: poor / inconsistent
- 2–3: mixed quality, needs refactor
- 4: mostly consistent
- 5: excellent / system-aligned

---

## 1) `ui/index.html` scorecard

| Criterion | Score | Notes |
|---|---:|---|
| Brand consistency | 4 | Header structure aligns with shared pattern, minor inline brand styling remains. |
| Token compliance | 3 | Uses shared tokens + CSS, but still has inline style usage in header/actions. |
| Component reuse | 4 | Good reuse of shared classes (`app-header`, `pill`, `btn-theme`). |
| Layout rhythm | 4 | Chat layout cadence is coherent and readable. |
| Interaction consistency | 4 | Buttons/states look aligned with chat module styles. |
| State clarity | 3 | Hidden states still mixed with inline `display:none`. |
| Theme parity | 4 | Theme preload and shared tokens are in place. |
| Accessibility baseline | 3 | Basic semantics exist; deeper keyboard/aria pass still needed. |
| **Average** | **3.6** | |

### Top 5 inconsistencies (`index`)
1. Inline branding style in header text spans.
2. Inline visibility handling on credits pill.
3. Inline link style on logout button.
4. Inline hidden state on file input.
5. Mixed strategy (Tailwind + shared CSS + page CSS) can drift over time.

### Evidence
- Tailwind + shared stack: `ui/index.html` lines 9–49
- Inline styles in header/actions: `ui/index.html` lines 55, 65, 89, 113

### Phase-2 target (`index`)
- Raise from **3.6 → 4.2+** by replacing inline styles with shared utility/component classes.

---

## 2) `ui/config.html` scorecard

| Criterion | Score | Notes |
|---|---:|---|
| Brand consistency | 4 | Branding is recognizable but subtitle and spacing are manually styled. |
| Token compliance | 2 | Large modal and loader blocks use hardcoded/inline visual values. |
| Component reuse | 2 | Multiple one-off inline button/layout styles bypass reusable classes. |
| Layout rhythm | 2 | Spacing cadence varies due to local inline margins/paddings. |
| Interaction consistency | 3 | Core interactions are clear, but control variants are visually uneven. |
| State clarity | 2 | Visibility states mostly done via inline `display:none`. |
| Theme parity | 3 | Token usage exists but is partially bypassed by inline styles. |
| Accessibility baseline | 3 | Some labels/titles exist, but modal/button semantics need harmonization. |
| **Average** | **2.6** | |

### Top 5 inconsistencies (`config`)
1. Header wrapper uses inline flex/gap/min-width.
2. Restart button uses inline color/border overrides.
3. Loading placeholder block is fully inline-styled.
4. Delete-vault modal is inline-styled end-to-end.
5. Modal actions use per-button inline styling instead of shared variants.

### Evidence
- Header + loading inline styles: `ui/config.html` lines 48–60, 79–82
- Vault modal inline styles: `ui/config.html` lines 97–114

### Phase-2 target (`config`)
- Raise from **2.6 → 4.0+** by extracting modal/layout/button variants into shared classes.

---

## 3) `ui/dashboard.html` scorecard

| Criterion | Score | Notes |
|---|---:|---|
| Brand consistency | 4 | Good structure, but subtitle and logo text still inline-styled. |
| Token compliance | 2 | Numerous inline styles for spacing, visibility and chart containers. |
| Component reuse | 3 | Card/grid system exists, but many blocks bypass shared helpers. |
| Layout rhythm | 2 | Repeated ad-hoc margin/height values create inconsistent rhythm. |
| Interaction consistency | 3 | Tabs/cards are coherent but state widgets vary in implementation. |
| State clarity | 2 | Many sections hidden via inline `display:none`. |
| Theme parity | 3 | Uses shared foundation, but inline overrides weaken consistency. |
| Accessibility baseline | 3 | Structure is mostly semantic; requires focused keyboard/contrast pass. |
| **Average** | **2.8** | |

### Top 5 inconsistencies (`dashboard`)
1. Inline style in logo/subtitle.
2. Inline width handling in context gauge fill.
3. Inline spacing on stats and chart wrappers.
4. Extensive inline hidden states across tab panels and card sections.
5. Repeated inline heading styles for section titles.

### Evidence
- Header inline styles: `ui/dashboard.html` lines 21–23
- Inline state/spacing examples: `ui/dashboard.html` lines 41, 106, 134, 139–141, 146, 154, 165, 186, 200

### Phase-2 target (`dashboard`)
- Raise from **2.8 → 4.0+** via shared state utilities and section-heading/layout helpers.

---

## 4) Immediate implementation tickets (ready for Phase 2)

### P0-T1: Header unification package ✅
- Scope: `ui/index.html`, `ui/config.html`, `ui/dashboard.html`
- Goal: remove inline branding/subtitle styles and apply one canonical header structure.
- Acceptance:
  - No static inline header styles on target pages.
  - Header score +1 on Token/Re-use criteria.
 - Status: Implemented in `ui/index.html`, `ui/config.html`, `ui/dashboard.html`.

### P0-T2: Visibility state utilities ✅
- Scope: all three pages above
- Goal: replace static `style="display:none"` with class-based state utilities.
- Acceptance:
  - Shared `.is-hidden` class used for static hidden blocks.
  - State clarity score +1 on all pages.
 - Status: Implemented with shared utility class + migrated static hidden blocks.

### P0-T3: Config modal extraction ✅
- Scope: `ui/config.html`
- Goal: convert vault modal inline styles into shared modal component variants.
- Acceptance:
  - Modal styles moved to CSS classes.
  - Config token compliance score +1.5.
 - Status: Implemented via `vault-*` modal classes in `ui/css/config.css`.

---

## 5) Design-token guardrail note

Shared color/typography/surface tokens already exist centrally in `ui/shared.css`.
Phase 2 should prefer token consumption over new hardcoded visual values.
