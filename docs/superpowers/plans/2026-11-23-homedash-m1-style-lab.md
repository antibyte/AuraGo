# HomeDash M1 Style Lab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the eight hero widgets (H1–H8) as real widget modules that render every display state, in all four style families (Glass, Tactile, Minimal, Instrument). Judge the 32 reference renderings on control-lab screenshot sheets through six lead gates (S, G, T, M, I, X). Then freeze the tokens and give the user a sign-off pack before any other control is built.

**Architecture:**
- Build the family-neutral parts first: DOM, geometry and state wiring. These are primitive builder extensions, four hero modules, `homedash-primitives.css` and `homedash-widgets.css`. They are reviewed in the `placeholder` look, because wrong proportions cannot be painted over.
- Then each family paints that geometry with one `--hd-*` token block plus painting rules that use only `var()` and `color-mix()`. Each family has its own stylesheet.
- Every visual claim is proven on sheets from `TestDesktopHomeDashControlLabBrowser`.
- V7 lines the families up side by side, fixes the risk themes and freezes the token rules behind a Node guard.

**Tech Stack:** Vanilla JS IIFE engine modules (`ui/js/homedash`, CSSOM only, no innerHTML). Plain CSS parts bundled by `scripts/build-ui-bundles.js` into `ui/css/homedash.bundle.css`. A Go + rod browser test (the control lab). Node guards (`npm run test:homedash`). Go standard-library image code for the colour-vision sheets.

---

## File structure

### Conventions every task uses

- **W** is the worktree `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\homedash-m1` on branch `claude/homedash-m1-style-lab`. All paths are relative to W.
- **M0 owns the contract; this plan uses M0's names.** The governing text is `docs/superpowers/plans/2026-11-23-homedash-m0-frontend.md`, section "Contracts":
  - C2: primitive markup (authoritative);
  - C3: widget host;
  - C4: lab URL and `window.hdLab`;
  - C5: sheets file grammar;
  - C6: probes.
  M1 extends primitives only through the table in Task 1. Everything else is exactly C2.
- **DOM.**
  - The wrapper is `div.hd-widget[data-hd-type][data-hd-variant][data-hd-family][data-hd-size][data-hd-status][data-hd-on?][data-hd-pending?][data-hd-stale?][data-hd-readonly?][data-hd-tone?]`. The host always writes `data-hd-family` (C3).
  - Inside it: `div.hd-frame` > `div.hd-head` (`span.hd-icon`, `span.hd-title`, `span.hd-sub`), `div.hd-body`, `div.hd-foot[hidden]`, `div.hd-state` > `span.hd-state-text`. `button.hd-more` is absent, because every M1 sheet sets `"more": false`.
  - Tone values are M0's: `ok`, `warn`, `crit`.
- **Host api.** `update(view)` receives `{mode, entities: {primary: viewState|null}}`. The api provides `family`, `setTone(tone|null)`, `setStatus(text|null)` and `history()`, which resolves to `{points: [[t_ms, value|null], …], source}`. The state line follows the host precedence (C3).
- **Lab.**
  - The page is `/testdata/homedash-lab.html`.
  - Fixtures are registered with `hdLab.fixtures.define(type, variant, entry)`, and named scenarios are run through `hdLab.applyState`. Extra probe results go to `hdLab.report(key, value)`; an `ok: false` value is a failure (C6).
  - Sheets are data in `ui/testdata/homedash-lab-sheets.json` (C5).
  - Output goes to the repository-root `reports/homedash-lab/`, as `<id>-NN.png` plus `probes.json`.
- **Painting selectors** always have the form `.hd-widget[data-hd-family="<f>"] .hd-…`. Family token blocks use the exact selector `[data-hd-family="<f>"]`. They take effect on widgets because the host writes `data-hd-family` on each one; on `.hd-root` the palette bridge outranks them.
- **Motion gate.** Every `transition` and `animation` sits inside `@media (prefers-reduced-motion: no-preference)`, on selectors that contain `.hd-root:not([data-hd-motion="reduced"]):not([data-hd-motion="none"])`. Tables call this wrapper **MG**.
  - Transitions may only animate `transform`, `opacity` and `stroke-dasharray`.
  - Keyframes may only animate `transform` and `opacity`.
- **The M0 guard** (`npm run test:homedash`, M0 Task 9) must stay green:
  - Literal colours only in token blocks (`.hd-root`, `.hd-root[data-hd-palette|scheme="…"]`, `[data-hd-family="…"]`). Painting declarations use `var()` and `color-mix()` with `transparent`.
  - In family files, `stroke-width`, `border-radius` and `box-shadow` contain `var(--hd-…)`, or are `0` or `none`.
  - `backdrop-filter` only in rules whose last compound selector is `.hd-frame` or `.hd-sheet`.
  - Class names start with `hd-`; no `!important` except `[hidden]`; `url()` only `/fonts/…` or `data:image/svg+xml` up to 4096 bytes.
- **Lab geometry.** The cell is 96 px and the gap 12 px:
  - 1×1 = 96×96, 2×1 = 204×96, 1×2 = 96×204, 2×2 = 204×204, 3×3 = 312×312.
  - The lab panel is 1180 CSS px wide; cells wrap.
- **Dial notation.** D = dial diameter in CSS px. Every dial and gauge SVG uses `viewBox="0 0 200 200"` (C2), centre (100,100), R = 100 units. The H5 crop is 168, the H6 crop is 172.
  - The handle, cap dot, needle and current marker are CSS-rotated groups (`homedash-base.css`).
  - Value arcs are one `path[pathLength=1000]`, driven by `stroke-dasharray: <lit> 1000`.
- **i18n.**
  - M1 Task 1 creates `ui/lang/homedash/<16 locales>.json`.
  - From then on, engine files call `t('homedash.…')` with full literal keys. This lifts M0's rule "never call `t('literal.key')` until `ui/lang/homedash` exists".
- **Code rules.**
  - No `light-dark()`, no `text-transform: uppercase`, no innerHTML or `setAttribute('style', …)`; CSSOM only.
  - Hit targets at least 44×44 CSS px, measured on the role element.
  - No external assets. LF line endings, no BOM.
  - New files under `docs/` and new `AGENTS.md` files need `git add -f`.
  - Commit after every task.
  - JS modules warn above 700 lines and fail at 1100. CSS parts are at most 900 lines.

### Files

| File | New / modified | Responsibility in M1 |
| --- | --- | --- |
| `ui/css/homedash-tokens.css` | modified (M0) | Appends §T0: scheme ladders, unconditional `--hd-light-on`, masks, defaults for shared tokens, square-corner rule |
| `ui/css/homedash-primitives.css` | modified (M0 starter) | Family-neutral structure: hidden-by-default decor, dial, gauge and spark layout, shared keyframes (Task 4) |
| `ui/css/homedash-widgets.css` | modified (M0 starter) | Frame grid and painting from card tokens, body-size properties, container queries, per-hero layout, state badges (Task 4) |
| `ui/css/homedash-family-glass.css` | modified | §T1 plus Glass painting, including the gradient-backdrop blur on `.hd-frame` |
| `ui/css/homedash-family-tactile.css` | modified | §T2 (dark and light material sets, scheme switch, LED state rules) plus painting |
| `ui/css/homedash-family-minimal.css` | modified | §T3 plus painting |
| `ui/css/homedash-family-instrument.css` | modified | §T4 (light mount, crit switch) plus painting |
| `ui/css/homedash.bundle.css` | regenerated | `npm run build:ui` |
| `ui/js/homedash/hd-prim-controls.js`, `hd-prim-displays.js` | modified (M0) | Only the additions in the Task 1 table |
| `ui/js/homedash/hd-colour.js` | modified (M0) | Adds `hudFor(hex)` (pure) |
| `ui/js/homedash/hd-widgets.js` | modified (M0) | Adds `E.widgets.applyLook(root)` only. Family, tone, status and history already come from M0 (C3). |
| `ui/js/homedash/hd-icons.js` | modified (M0) | Adds missing glyphs: `power`, `mode-auto`, `plug`, `cpu`, `disk`, `fan-heater`, `trend-up`, `trend-down`, `trend-flat` (24 grid, 1.75 stroke, round caps and joins) |
| `ui/js/homedash/widgets/hd-w-switch.js` | new | `switch`: pill = H1, rocker = H2; push and tile have empty bodies |
| `ui/js/homedash/widgets/hd-w-light.js` | new | `light`: dial = H3; slider, fader and tile empty |
| `ui/js/homedash/widgets/hd-w-climate.js` | new | `climate`: dial = H4; stepper and scale empty |
| `ui/js/homedash/widgets/hd-w-sensor.js` | new | `gauge` (needle = H5, arc = H6; bar and tank empty); `temperature` (numeral = H7; thermometer and comfort empty); `sensor` (sparkline = H8; numeral uses the H7 builder without degree; icon empty) |
| `ui/lang/homedash/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` | new | The M1 keys (Task 1) |
| `ui/i18n_lint_test.go` | modified | Adds a `TestTranslations_SectionFilesOnlyOwnPrefixes` case for `lang/homedash` with prefix `homedash.` |
| `ui/testdata/homedash-lab.js` | modified (M0, ≤ 12 lines) | Loads `/lang/homedash/en.json` and `<lang>.json` over `LAB_WORDS` before rendering (Task 1) |
| `ui/testdata/homedash-lab.html` | modified (M0) | Script tags for the four widget modules (Tasks 2–3) |
| `scripts/homedash-tests/harness.mjs` | modified (M0) | `ENGINE_ORDER` lists the widget modules alphabetically after `widgets/hd-w-misc.js` (Tasks 2–3) |
| `ui/testdata/homedash-lab/hero-fixtures.js` | new | Hero fixtures and scenarios through `hdLab.fixtures.define`, focus marking, `applyLook`, extra probes `m1-tabular` and `m1-svgcount` (Task 0) |
| `ui/testdata/homedash-lab-sheets.json` | modified (M0) | M1 named lists and sheet entries (Task 0) |
| `ui/desktop_homedash_lab_cvd_test.go` | new | Colour-vision and greyscale copies of hero sheets (Task 0) |
| `scripts/homedash-tests/m1.test.mjs` | new | `hudFor`, the primitive additions, the token freeze (Tasks 1, 22) |
| `scripts/homedash-freeze-tokens.mjs` | new | Token-rule extraction and the writer for `token-freeze.json` (Task 1; first run in Task 22) |
| `ui/testdata/homedash-lab/token-freeze.json` | new (Task 22) | Frozen token rules |
| `documentation/homedash-visual-system.md` | new (Task 24) | Recipes, frozen token table, lab and review usage, host contract notes |
| `ui/js/homedash/AGENTS.md` | modified (Task 24) | Style-lab rules |
| `ui/AGENTS.md` | modified (Task 24) | One pointer line |

## Token and recipe sheet

This section is the implementer's only source for values. Literal colours appear only in the token blocks here. Every token a painting rule reads is declared in T0–T4 or by M0 (`homedash-tokens.css`, `homedash-bridge.css`).

### T0. Additions to `homedash-tokens.css` (all families)

```css
/* M1 additions: scheme ladders, masks and defaults of the tokens the shared rules read. */
.hd-root {
  --hd-corners-control: 9999px;
  --hd-fade-down: linear-gradient(180deg, #000000, transparent);
  --hd-ring-mask: radial-gradient(farthest-side, transparent calc(100% - 2.5px), #000000 calc(100% - 2px));
  --hd-arc-mask-75: conic-gradient(#000000 0 75%, transparent 75% 100%);
  --hd-ease-hud: cubic-bezier(.2, .8, .2, 1);
  --hd-mask-clock: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Ccircle cx='12' cy='12' r='9'/%3E%3Cpath d='M12 7v5l3 2'/%3E%3C/svg%3E");
  --hd-mask-lock: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Crect x='5' y='11' width='14' height='10' rx='2'/%3E%3Cpath d='M8 11V8a4 4 0 0 1 8 0v3'/%3E%3C/svg%3E");
  --hd-mask-alert: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M12 3.5 2.5 20.5h19Z'/%3E%3Cpath d='M12 10v4.5M12 17.5v.01'/%3E%3C/svg%3E");
  --hd-mask-error: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Ccircle cx='12' cy='12' r='9'/%3E%3Cpath d='M12 7.5v5.5M12 16.5v.01'/%3E%3C/svg%3E");
  --hd-card-bg: var(--hd-surface-solid); --hd-card-border: 1px solid var(--hd-hairline); --hd-dash-unavail: 1px dashed var(--hd-hairline);
  --hd-shadow-card: none; --hd-ring-alert: inset 0 0 0 2px var(--hd-alert);
  --hd-label-size: 12.5px; --hd-label-weight: 500; --hd-num-weight: 400; --hd-num-weight-small: 500; --hd-num-tracking: 0em;
  --hd-num-cap: 0.74; --hd-num-hero-max: 96px; --hd-unit-scale: 0.42; --hd-dial-num-k: 0.26; --hd-thermo-num-k: 0.24;
}
.hd-root[data-hd-scheme="dark"] {
  --hd-sa-1: 14%; --hd-sa-2: 18%; --hd-sa-3: 24%; --hd-sa-4: 34%; --hd-sa-5: 48%;
  --hd-glass-alpha-s: 72%; --hd-fill-edge-k: 100%;
  --hd-light-on: #FFB65C;
}
.hd-root[data-hd-scheme="light"] {
  --hd-sa-1: 8%; --hd-sa-2: 11%; --hd-sa-3: 14%; --hd-sa-4: 20%; --hd-sa-5: 29%;
  --hd-glass-alpha-s: 78%; --hd-fill-edge-k: 72%;
  --hd-light-on: #C77A12;
}
.hd-root[data-hd-corners="square"] { --hd-corners-control: 0px; }
```

Reasons:
- **`--hd-light-on`.** The bridge sets it on `.hd-root` at specificity (0,1,0). The scheme blocks above (0,2,0) always win, so the light amber `#C77A12` (3.4:1 on white) applies under every light scheme. Custom-palette inline values win over both.
- **Glass alpha** is 72/78 %, above the minimums of 62/70 %.
- **`--hd-fill-edge-k`** darkens the edge of an "on" fill in light schemes. Fruity primary `color-mix(accent 80%, #fff)` measures 2.8:1 on white; the edge measures 5.2:1.
- **The masks** draw the stale, readonly, alert and error badges in CSS, so the frame needs no extra DOM. The bundled Inter Latin subset lacks ▲ (U+25B2).
- **The easing fallback** is M0's (`@supports` plus `.hd-root[data-hd-ease="fallback"]`), so nothing here.

### T1. Glass token block

```css
[data-hd-family="glass"] {
  --hd-surface: color-mix(in srgb, var(--hd-surface-solid) var(--hd-glass-alpha-s), transparent);
  --hd-g-sheen: linear-gradient(180deg, color-mix(in srgb, var(--hd-edge-tint) 9%, transparent), transparent 46%);
  --hd-card-bg: var(--hd-g-sheen), var(--hd-surface);
  --hd-card-border: 1px solid color-mix(in srgb, var(--hd-edge-tint) 14%, var(--hd-theme-border));
  --hd-dash-unavail: 1px dashed var(--hd-hairline);
  --hd-radius-card: var(--hd-theme-radius); --hd-radius-control: var(--hd-corners-control);
  --hd-g-bezel-r: 14px; --hd-g-paddle-r: 10px;
  --hd-glass-blur: 18px;
  --hd-edge-hi: inset 0 1px 0 color-mix(in srgb, var(--hd-edge-tint) 20%, transparent);
  --hd-edge-lo: inset 0 -1px 0 color-mix(in srgb, var(--hd-shadow-ink) var(--hd-sa-1), transparent);
  --hd-shadow-contact: 0 1px 2px color-mix(in srgb, var(--hd-shadow-ink) var(--hd-sa-1), transparent);
  --hd-shadow-card: var(--hd-edge-hi), var(--hd-edge-lo), var(--hd-shadow-contact),
                    0 14px 32px -10px color-mix(in srgb, var(--hd-shadow-ink) var(--hd-sa-4), transparent);
  --hd-shadow-press: inset 0 1px 3px color-mix(in srgb, var(--hd-shadow-ink) var(--hd-sa-3), transparent);
  --hd-ring-alert: inset 0 0 0 2px var(--hd-alert);
  --hd-track: color-mix(in srgb, var(--hd-ink) 14%, transparent);
  --hd-track-inset: inset 0 1px 2px color-mix(in srgb, var(--hd-shadow-ink) var(--hd-sa-3), transparent);
  --hd-track-fill: var(--hd-on);
  --hd-track-fill-hi: inset 0 1px 0 color-mix(in srgb, var(--hd-edge-tint) 30%, transparent),
                      inset 0 0 0 1px color-mix(in srgb, var(--hd-on) var(--hd-fill-edge-k), #000000);
  --hd-thumb: color-mix(in srgb, #FFFFFF 96%, var(--hd-surface-solid));
  --hd-thumb-edge: color-mix(in srgb, var(--hd-shadow-ink) 22%, transparent);
  --hd-thumb-shadow: 0 0 0 0.5px var(--hd-thumb-edge),
                     0 1px 1px color-mix(in srgb, var(--hd-shadow-ink) var(--hd-sa-2), transparent),
                     0 3px 8px color-mix(in srgb, var(--hd-shadow-ink) var(--hd-sa-3), transparent);
  --hd-pending-ring: conic-gradient(var(--hd-accent) 0 25%, transparent 25% 100%);
  --hd-stroke-hair: 1px; --hd-stroke-1: 1.5px; --hd-stroke-2: 2px; --hd-stroke-arc: 0.075; --hd-stroke-tick: 1.25px;
  --hd-g-edge-u: 0.5; --hd-g-capdot-r: 10; --hd-g-shadow-a: 0.22; --hd-arc-underglow: 0.16;
  --hd-value: var(--hd-accent);
  --hd-font-ui: "Inter", "Geist", system-ui, sans-serif; --hd-font-label: var(--hd-font-ui); --hd-font-num: "Inter", "Geist", system-ui, sans-serif;
  --hd-num-features: "tnum" 1, "cv11" 1; --hd-num-weight: 300; --hd-num-weight-small: 500; --hd-num-tracking: -0.02em;
  --hd-num-cap: 0.74; --hd-num-hero-max: 112px; --hd-unit-scale: 0.42;
  --hd-label-size: 12.5px; --hd-label-weight: 500; --hd-label-tracking: 0em;
  --hd-dur-toggle: 280ms; --hd-ease-toggle: var(--hd-ease-spring); --hd-dur-fill: 180ms; --hd-dur-xfade: 180ms;
  --hd-dur-remote: 320ms; --hd-ease-remote: var(--hd-ease-out); --hd-dur-zone: 220ms; --hd-dur-needle-f: 1100ms; --hd-dur-dip: 180ms;
}
```

Reasons:
- `--hd-num-cap` 0.74 matches Inter's figure height (about 0.727 em).
- `--hd-g-capdot-r` 10 is 1.35 × the 15-unit stroke. C2's radius 7 would hide inside the stroke; CSS `r` overrides it.

### T2. Tactile token block, scheme switch and LED states

```css
[data-hd-family="tactile"] {
  --hd-t-panel-hi-d: #2A2E35; --hd-t-panel-lo-d: #1F2228; --hd-t-panel-mid-d: #24282E;
  --hd-t-panel-hi-l: #D9DDE3; --hd-t-panel-lo-l: #BFC4CC; --hd-t-panel-mid-l: #CCD0D7;
  --hd-t-bakelite-d: #1C1A18; --hd-t-bakelite-l: #ECE6D8; --hd-t-spec-d: 14%; --hd-t-spec-l: 22%;
  --hd-t-legend-d: #C9CDD3; --hd-t-legend-l: #33373D; --hd-t-muted-d: #9AA1AB; --hd-t-muted-l: #4A4F57;
  --hd-t-face-d: #101215; --hd-t-face-l: #F2EDE1; --hd-t-faceink-d: #E4E1D8; --hd-t-faceink-l: #23262B;
  --hd-t-emboss-d: 0 -1px 0 color-mix(in srgb, #000000 60%, transparent);
  --hd-t-emboss-l: 0 1px 0 color-mix(in srgb, #FFFFFF 50%, transparent);
  --hd-t-panel-hi: var(--hd-t-panel-hi-d); --hd-t-panel-lo: var(--hd-t-panel-lo-d); --hd-t-panel-mid: var(--hd-t-panel-mid-d);
  --hd-t-bakelite-c: var(--hd-t-bakelite-d); --hd-t-spec: var(--hd-t-spec-d);
  --hd-t-legend: var(--hd-t-legend-d); --hd-t-muted: var(--hd-t-muted-d); --hd-t-emboss: var(--hd-t-emboss-d);
  --hd-t-face: var(--hd-t-face-d); --hd-t-faceink: var(--hd-t-faceink-d);
  --hd-t-window: #07090B; --hd-t-amber: #FFB547; --hd-t-vfd: #FFC266; --hd-t-needle: #D2463A;
  --hd-t-led-green: #3DDC84; --hd-t-led-amber: #FFB547; --hd-t-led-red: #FF4D3D; --hd-t-led-base: #111111;
  --hd-t-knurl-a: #3A3E45; --hd-t-knurl-b: #5A606A;
  --hd-led: var(--hd-t-led-green);
  --hd-t-brush: repeating-linear-gradient(180deg, transparent 0, color-mix(in srgb, #FFFFFF 5%, transparent) 2px, transparent 4px);
  --hd-t-panel: var(--hd-t-brush), linear-gradient(180deg, var(--hd-t-panel-hi), var(--hd-t-panel-lo));
  --hd-t-bakelite: radial-gradient(ellipse 70% 55% at 30% 20%, color-mix(in srgb, #FFFFFF var(--hd-t-spec), transparent), transparent 55%), var(--hd-t-bakelite-c);
  --hd-t-steel: conic-gradient(from 18deg, #C7CCD4, #F3F5F8 11%, #A8AEB7 24%, #E4E7EC 37%, #979EA8 50%, #EFF1F4 63%, #B0B6BF 76%, #ECEFF3 89%, #C7CCD4);
  --hd-t-steel-lin: linear-gradient(135deg, #E9ECF0, #B9BFC7 45%, #DDE1E6 60%, #A9AFB8);
  --hd-t-grip: repeating-linear-gradient(90deg, color-mix(in srgb, #000000 28%, transparent) 0 1px, color-mix(in srgb, #FFFFFF 40%, transparent) 1px 2px) center / 10px 12px no-repeat;
  --hd-t-knurl: repeating-conic-gradient(var(--hd-t-knurl-a) 0deg 2.6deg, var(--hd-t-knurl-b) 3.4deg 5.6deg, var(--hd-t-knurl-a) 6.4deg 7.2deg);
  --hd-t-ring-86: radial-gradient(farthest-side, transparent 86%, #000000 87%);
  --hd-t-ring-92: radial-gradient(farthest-side, transparent 91.5%, #000000 92.5%);
  --hd-t-pointer: linear-gradient(180deg, var(--hd-t-faceink-l) 0 62%, transparent 62%);
  --hd-t-led: radial-gradient(circle at 36% 30%, color-mix(in srgb, var(--hd-led) 35%, #FFFFFF) 0 16%, var(--hd-led) 46%, color-mix(in srgb, var(--hd-led) 55%, #000000) 100%);
  --hd-t-led-off: radial-gradient(circle at 36% 30%, color-mix(in srgb, #FFFFFF 12%, transparent) 0 14%, transparent 30%), color-mix(in srgb, var(--hd-led) 22%, var(--hd-t-led-base));
  --hd-t-led-halo: 0 0 6px 1px color-mix(in srgb, var(--hd-led) 60%, transparent);
  --hd-t-dot-off: color-mix(in srgb, var(--hd-t-amber) 18%, var(--hd-t-led-base));
  --hd-t-shade: color-mix(in srgb, #000000 12%, transparent);
  --hd-t-needle-shadow: color-mix(in srgb, #000000 30%, transparent);
  --hd-t-grid-ink: color-mix(in srgb, var(--hd-t-amber) 10%, transparent);
  --hd-t-hatch: repeating-linear-gradient(135deg, color-mix(in srgb, var(--hd-ink) 6%, transparent) 0 2px, transparent 2px 6px);
  --hd-t-vignette: radial-gradient(circle, transparent 82%, color-mix(in srgb, #000000 6%, transparent) 100%);
  --hd-t-reflection: radial-gradient(ellipse 60% 30% at 34% 24%, color-mix(in srgb, #FFFFFF 8%, transparent), transparent 70%);
  --hd-surface-solid: var(--hd-t-panel-mid); --hd-surface: var(--hd-t-panel-mid);
  --hd-ink: var(--hd-t-legend); --hd-ink-muted: var(--hd-t-muted);
  --hd-ink-faint: color-mix(in srgb, var(--hd-t-legend) 45%, var(--hd-t-panel-mid));
  --hd-sunken: var(--hd-t-window); --hd-focus-gap: var(--hd-t-panel-mid);
  --hd-hairline: color-mix(in srgb, #000000 30%, var(--hd-t-panel-mid));
  --hd-card-bg: var(--hd-t-panel);
  --hd-card-border: 1px solid color-mix(in srgb, #000000 35%, var(--hd-t-panel-lo));
  --hd-dash-unavail: 1px dashed color-mix(in srgb, #000000 35%, var(--hd-t-panel-lo));
  --hd-radius-card: calc(12px * var(--hd-radius-scale)); --hd-radius-control: calc(6px * var(--hd-radius-scale));
  --hd-t-bezel-r: calc(8px * var(--hd-radius-scale));
  --hd-shadow-card: inset 1px 1px 0 color-mix(in srgb, #FFFFFF 10%, transparent), inset -1px -1px 0 color-mix(in srgb, #000000 30%, transparent),
                    2px 3px 8px color-mix(in srgb, #000000 var(--hd-sa-4), transparent);
  --hd-shadow-contact: 1px 2px 3px color-mix(in srgb, #000000 var(--hd-sa-4), transparent);
  --hd-shadow-cast: 2px 4px 10px color-mix(in srgb, #000000 var(--hd-sa-5), transparent);
  --hd-recess: inset 1px 2px 4px color-mix(in srgb, #000000 60%, transparent), inset -1px -1px 0 color-mix(in srgb, #FFFFFF 8%, transparent);
  --hd-window-shadow: inset 0 2px 6px color-mix(in srgb, #000000 70%, transparent);
  --hd-lip: inset 1px 1px 0 color-mix(in srgb, #FFFFFF 22%, transparent), inset -1px -1px 0 color-mix(in srgb, #000000 30%, transparent);
  --hd-ring-alert: inset 0 0 0 1px var(--hd-t-led-red);
  --hd-vfd-glow: 0 0 6px color-mix(in srgb, var(--hd-t-amber) 50%, transparent);
  --hd-stroke-hair: 1px; --hd-stroke-1: 1.5px; --hd-stroke-2: 2px; --hd-stroke-arc: 0.06; --hd-stroke-tick: 1.25px;
  --hd-t-needle-w: 2.5;
  --hd-font-ui: "Inter", system-ui, sans-serif;
  --hd-font-label: "HomeDash Barlow Condensed", "Inter", sans-serif; --hd-font-num: "HomeDash Barlow Condensed", "Inter", sans-serif;
  --hd-font-vfd: "Geist Mono", "JetBrains Mono", monospace;
  --hd-num-features: "tnum" 1; --hd-num-weight: 600; --hd-num-weight-small: 600; --hd-num-tracking: 0em; --hd-num-cap: 0.74;
  --hd-num-hero-max: 72px; --hd-unit-scale: 0.42;
  --hd-label-size: 12px; --hd-label-weight: 600; --hd-label-tracking: 0.06em;
  --hd-dur-toggle: 120ms; --hd-ease-toggle: var(--hd-ease-snap); --hd-dur-led: 60ms; --hd-dur-fill: 60ms; --hd-dur-xfade: 60ms;
  --hd-dur-remote: 220ms; --hd-ease-remote: var(--hd-ease-out); --hd-dur-needle-f: 1100ms; --hd-dur-zone: 60ms;
}
.hd-root[data-hd-scheme="light"] [data-hd-family="tactile"],
.hd-root[data-hd-scheme="light"][data-hd-family="tactile"] {
  --hd-t-panel-hi: var(--hd-t-panel-hi-l); --hd-t-panel-lo: var(--hd-t-panel-lo-l); --hd-t-panel-mid: var(--hd-t-panel-mid-l);
  --hd-t-bakelite-c: var(--hd-t-bakelite-l); --hd-t-spec: var(--hd-t-spec-l);
  --hd-t-legend: var(--hd-t-legend-l); --hd-t-muted: var(--hd-t-muted-l); --hd-t-emboss: var(--hd-t-emboss-l);
  --hd-t-face: var(--hd-t-face-l); --hd-t-faceink: var(--hd-t-faceink-l);
}
.hd-widget[data-hd-family="tactile"][data-hd-pending="true"] { --hd-led: var(--hd-t-led-amber); }
.hd-widget[data-hd-family="tactile"][data-hd-status="error"], .hd-widget[data-hd-family="tactile"][data-hd-tone="crit"] { --hd-led: var(--hd-t-led-red); }
```

Reasons:
- **LED scoping.** `--hd-led` is set on the widget, the same element whose family block declares `--hd-t-led`, `--hd-t-led-off` and `--hd-t-led-halo`. They therefore resolve with the state colour, and children read the computed recipes.
- **Scheme switch.** The guard's token-block regex has no scheme-qualified family selector, so both literal sets live in the block and a `var()`-only rule selects one.
- **LEDs use fixed emission colours.** Light-theme status inks (Amiga `#004300`) would look like unlit lamps. In Tactile the accent appears only as the focus colour.
- **Remote motion** is 220 ms (visual §4.1 H3).
- **Contrast** (hand-computed): legend on panel mid 9.3:1 dark, 7.7:1 light; muted 5.6:1 and 5.3:1.

### T3. Minimal token block

```css
[data-hd-family="minimal"] {
  --hd-surface: var(--hd-surface-solid); --hd-card-bg: var(--hd-surface-solid);
  --hd-card-border: 1px solid var(--hd-hairline); --hd-dash-unavail: 1px dashed var(--hd-hairline);
  --hd-radius-card: var(--hd-theme-radius); --hd-radius-control: var(--hd-corners-control);
  --hd-edge-hi: none; --hd-edge-lo: none; --hd-shadow-contact: none; --hd-shadow-card: none; --hd-shadow-press: none; --hd-ring-alert: none;
  --hd-track: transparent; --hd-track-line: var(--hd-ink); --hd-track-fill: var(--hd-ink);
  --hd-thumb: var(--hd-ink); --hd-thumb-on: var(--hd-surface-solid);
  --hd-stroke-hair: 1px; --hd-stroke-1: 1.5px; --hd-stroke-2: 2px; --hd-stroke-arc: 0.04; --hd-stroke-tick: 1px;
  --hd-m-arc-w: 3px; --hd-m-spark-w: 1.25px; --hd-m-dot-r: 4;
  --hd-value: var(--hd-accent);
  --hd-font-ui: "Geist", "Inter", system-ui, sans-serif; --hd-font-label: var(--hd-font-ui); --hd-font-num: var(--hd-font-ui);
  --hd-num-features: "tnum" 1; --hd-num-weight: 300; --hd-num-weight-small: 500; --hd-num-tracking: -0.035em;
  --hd-num-cap: 0.71; --hd-num-hero-max: 128px; --hd-unit-scale: 0.42; --hd-dial-num-k: 0.28; --hd-thermo-num-k: 0.26;
  --hd-label-size: 12.5px; --hd-label-weight: 500; --hd-label-tracking: 0.01em;
  --hd-dur-toggle: 160ms; --hd-ease-toggle: var(--hd-ease-standard); --hd-dur-fill: 160ms; --hd-dur-xfade: 160ms;
  --hd-dur-remote: 160ms; --hd-ease-remote: var(--hd-ease-standard); --hd-dur-needle-f: 420ms; --hd-dur-zone: 160ms;
}
```

Reasons:
- `--hd-num-cap` 0.71 (Geist figures are about 0.70 em high) and the larger dial factors make Minimal's numerals the largest in the same box.
- The needle takes 420 ms on the standard curve: there is no overshoot, and 160 ms reads as a jump.

### T4. Instrument token block, light mount and crit switch

```css
[data-hd-family="instrument"] {
  --hd-i-panel-hi: #0B0F14; --hd-i-panel-lo: #07090C; --hd-i-mid: #090C10; --hd-i-window: #040608;
  --hd-i-ink: #D5DEE8; --hd-i-muted: #8B97A6; --hd-i-faint: #4A5361;
  --hd-i-ok: #4AD29C; --hd-i-warn: #F2B84B; --hd-i-alert: #FF7A72; --hd-i-info: #5AA9FF;
  --hd-i-hud: var(--hd-hud, color-mix(in srgb, var(--hd-accent) 85%, #FFFFFF));
  --hd-i-val: var(--hd-i-hud);
  --hd-i-br: color-mix(in srgb, var(--hd-i-hud) 70%, transparent);
  --hd-i-corners:
    linear-gradient(var(--hd-i-br), var(--hd-i-br)) left 0 top 0 / 10px 2px no-repeat, linear-gradient(var(--hd-i-br), var(--hd-i-br)) left 0 top 0 / 2px 10px no-repeat,
    linear-gradient(var(--hd-i-br), var(--hd-i-br)) right 0 top 0 / 10px 2px no-repeat, linear-gradient(var(--hd-i-br), var(--hd-i-br)) right 0 top 0 / 2px 10px no-repeat,
    linear-gradient(var(--hd-i-br), var(--hd-i-br)) left 0 bottom 0 / 10px 2px no-repeat, linear-gradient(var(--hd-i-br), var(--hd-i-br)) left 0 bottom 0 / 2px 10px no-repeat,
    linear-gradient(var(--hd-i-br), var(--hd-i-br)) right 0 bottom 0 / 10px 2px no-repeat, linear-gradient(var(--hd-i-br), var(--hd-i-br)) right 0 bottom 0 / 2px 10px no-repeat;
  --hd-i-scan: repeating-linear-gradient(0deg, transparent 0, color-mix(in srgb, #000000 22%, transparent) 1px, transparent 2px 3px);
  --hd-i-glowline: 0 0 10px color-mix(in srgb, var(--hd-i-val) 55%, transparent);
  --hd-i-ghost: color-mix(in srgb, var(--hd-i-hud) 7%, transparent);
  --hd-i-gap-dash: 0 20 5 0;
  --hd-i-mount-l: 0 1px 2px color-mix(in srgb, #000000 30%, transparent), 0 6px 16px -6px color-mix(in srgb, #000000 35%, transparent);
  --hd-surface-solid: var(--hd-i-mid); --hd-surface: var(--hd-i-mid); --hd-sunken: var(--hd-i-window);
  --hd-ink: var(--hd-i-ink); --hd-ink-muted: var(--hd-i-muted); --hd-ink-faint: var(--hd-i-faint);
  --hd-ok: var(--hd-i-ok); --hd-warn: var(--hd-i-warn); --hd-alert: var(--hd-i-alert); --hd-info: var(--hd-i-info);
  --hd-ok-ink: var(--hd-i-ok); --hd-warn-ink: var(--hd-i-warn); --hd-alert-ink: var(--hd-i-alert); --hd-info-ink: var(--hd-i-info);
  --hd-hairline: color-mix(in srgb, var(--hd-i-hud) 18%, transparent);
  --hd-focus: var(--hd-i-hud); --hd-focus-gap: var(--hd-i-mid);
  --hd-card-bg: var(--hd-i-corners), linear-gradient(180deg, var(--hd-i-panel-hi), var(--hd-i-panel-lo));
  --hd-card-border: 1px solid color-mix(in srgb, var(--hd-i-hud) 18%, transparent);
  --hd-dash-unavail: 1px dashed color-mix(in srgb, var(--hd-i-hud) 30%, transparent);
  --hd-radius-card: calc(4px * var(--hd-radius-scale)); --hd-radius-control: calc(3px * var(--hd-radius-scale));
  --hd-edge-hi: none; --hd-edge-lo: none; --hd-shadow-contact: none; --hd-shadow-card: none; --hd-shadow-press: none; --hd-ring-alert: none;
  --hd-glow: var(--hd-i-glowline);
  --hd-stroke-hair: 1px; --hd-stroke-1: 1.5px; --hd-stroke-2: 2px; --hd-stroke-arc: 0.08; --hd-stroke-tick: 1.25px;
  --hd-i-glow-add: 6; --hd-i-gap-add: 2;
  --hd-font-ui: "Oxanium", "Geist", system-ui, sans-serif; --hd-font-label: var(--hd-font-ui);
  --hd-font-num: "HomeDash Share Tech Mono", "Geist Mono", monospace;
  --hd-num-features: "tnum" 1; --hd-num-weight: 400; --hd-num-weight-small: 400; --hd-num-tracking: 0em; --hd-num-cap: 0.74;
  --hd-num-hero-max: 96px; --hd-unit-scale: 0.42;
  --hd-label-size: 11.5px; --hd-label-weight: 500; --hd-label-tracking: 0.12em;
  --hd-dur-toggle: 110ms; --hd-ease-toggle: var(--hd-ease-hud); --hd-dur-flash: 90ms; --hd-dur-fill: 110ms; --hd-dur-xfade: 110ms;
  --hd-dur-remote: 220ms; --hd-ease-remote: var(--hd-ease-hud); --hd-dur-needle-f: 880ms; --hd-dur-chase: 80ms; --hd-dur-zone: 110ms; --hd-dur-dip: 180ms;
}
.hd-root[data-hd-scheme="light"] [data-hd-family="instrument"],
.hd-root[data-hd-scheme="light"][data-hd-family="instrument"] { --hd-shadow-card: var(--hd-i-mount-l); }
.hd-widget[data-hd-family="instrument"][data-hd-tone="crit"] { --hd-i-val: var(--hd-i-alert); }
```

Reasons:
- **HUD colour.** `--hd-hud` is computed by `E.colour.hudFor()` and written on `.hd-root` by `applyLook`. The fallback mix fails on Minimal's accent `#161616` (about 1.7:1).
- **Fixed status set.** Light-theme status inks fail on the always-dark panel.
- **Bracket bars are 2 px**, because 1.5 px smears at DPR 1.
- **Segment gaps.** `--hd-i-gap-dash` paints 40 segment gaps on a `pathLength=1000` arc (period 25 units, gap 5 units, about 1.35°).

### T5. Shared painting rules (written once per family file)

`homedash-widgets.css` paints the frame, the unavailable and alert rings and the state badges for every family:

```css
.hd-widget > .hd-frame { background: var(--hd-card-bg); border: var(--hd-card-border); border-radius: var(--hd-radius-card); box-shadow: var(--hd-shadow-card); overflow: hidden; }
.hd-widget[data-hd-status="unavailable"] > .hd-frame { border: var(--hd-dash-unavail); }
.hd-widget[data-hd-tone="crit"] > .hd-frame::before { content: ""; position: absolute; inset: 0; border-radius: inherit; box-shadow: var(--hd-ring-alert); pointer-events: none; z-index: 2; }
.hd-head::after { content: ""; display: none; flex: none; inline-size: 12px; block-size: 12px; background: var(--hd-ink-muted);
  -webkit-mask: var(--hd-badge-mask) center / contain no-repeat; mask: var(--hd-badge-mask) center / contain no-repeat; }
.hd-widget[data-hd-stale="true"] .hd-head::after { display: block; --hd-badge-mask: var(--hd-mask-clock); }
.hd-widget[data-hd-readonly="true"] .hd-head::after { display: block; --hd-badge-mask: var(--hd-mask-lock); }
.hd-widget[data-hd-tone="crit"] .hd-head::after { display: block; --hd-badge-mask: var(--hd-mask-alert); background: var(--hd-alert); }
.hd-widget[data-hd-status="error"] .hd-state { color: var(--hd-alert-ink); }
.hd-widget[data-hd-status="error"] .hd-state::before { content: ""; display: inline-block; inline-size: 12px; block-size: 12px; margin-inline-end: 4px; vertical-align: -1px;
  background: var(--hd-alert-ink); -webkit-mask: var(--hd-mask-error) center / contain no-repeat; mask: var(--hd-mask-error) center / contain no-repeat; }
.hd-widget [data-hd-disabled="true"] { opacity: 0.45; }
.hd-widget[data-hd-stale="true"] :is(.hd-numeral, .hd-seg7) { color: var(--hd-ink-muted); }
```

The focus ring comes from M0 (`homedash-base.css`): `:is(:focus-visible, [data-hd-focus="ring"])` on the role elements, `box-shadow: 0 0 0 2px var(--hd-focus-gap), 0 0 0 4px var(--hd-focus)`.

The families add their own layers. They never compose a theme token that can be `none` into a list:

```css
.hd-widget[data-hd-family="glass"] > .hd-frame::after { content: ""; position: absolute; inset: 0; border-radius: inherit; box-shadow: var(--hd-theme-edge); pointer-events: none; }
.hd-widget[data-hd-family="glass"] :is(.hd-toggle, .hd-rocker, .hd-dial, .hd-seg-opt, .hd-push):is(:focus-visible, [data-hd-focus="ring"])::before { content: ""; position: absolute; inset: 0; border-radius: inherit; box-shadow: var(--hd-theme-glow); pointer-events: none; }
.hd-root[data-hd-backdrop="gradient"] .hd-widget[data-hd-family="glass"] > .hd-frame { -webkit-backdrop-filter: blur(var(--hd-glass-blur)) saturate(1.35); backdrop-filter: blur(var(--hd-glass-blur)) saturate(1.35); }
.hd-widget[data-hd-family="tactile"] :is(.hd-toggle, .hd-rocker, .hd-dial):is(:focus-visible, [data-hd-focus="ring"]) :is(.hd-toggle-thumb, .hd-rocker-paddle, .hd-dial-cap) { translate: 0 -1px; }
```

**Shared state language** (the host writes the wrapper attributes; widgets paint only their controls):

| State | Glass | Tactile | Minimal | Instrument |
| --- | --- | --- | --- | --- |
| unavailable | Dashed border; `.hd-body` opacity 0.5; thumb hollow (transparent fill, `inset 0 0 0 var(--hd-stroke-1) var(--hd-ink-faint)`); value "–" | `--hd-card-bg: var(--hd-t-hatch), var(--hd-t-panel)` on the wrapper; lamp hollow ring; legends "—" | Every ink `--hd-ink-faint`; value "–" | `.hd-body > *` has `visibility: hidden`. `.hd-body::after` shows `attr(data-hd-nodata)` (written by the module) in `--hd-ink-faint`, 11.5px Oxanium, centred, inside a `--hd-dash-unavail` box with 8 px padding. |
| stale | Clock badge; numerals muted (all families) | same | same | same |
| readonly | Lock badge; thumb and handle shadows removed (`box-shadow: none`) | Lock badge; no `--hd-shadow-cast` | Lock badge | Lock badge |
| disabled | Control opacity 0.45 (`[data-hd-disabled]`); the module writes the reason | same | same | same |
| error | Error icon plus `--hd-alert-ink` state line (host word "Not confirmed") | same, LED red (`--hd-led` switch) | same | Lit segments and readout in alert through `--hd-i-val`? No: error keeps HUD; only the state line turns alert. |
| alert (tone crit) | `--hd-ring-alert`, triangle badge in alert colour, word | Red LED and 1px red rim | Triangle badge; value `--hd-alert-ink`; no ring | `--hd-i-val` becomes alert. MG: pulse 1 Hz, 0.6↔1, 6 cycles (`hd-pulse`) on the lit elements |
| focus | M0 ring plus the theme-glow layer | Ring, hardware lifted 1px | Ring | Ring in HUD (`--hd-focus`) |

## Hero build sheets

### Shared frame geometry (`homedash-widgets.css`, family-neutral)

```css
.hd-widget > .hd-frame {
  position: absolute; inset: 0; box-sizing: border-box; display: grid;
  --hd-pad-b: 16px; --hd-pad-i: 16px; --hd-head-h: 18px; --hd-state-h: 16px; --hd-gap-in: 6px;
  padding: var(--hd-pad-b) var(--hd-pad-i); row-gap: var(--hd-gap-in); column-gap: 12px;
  grid-template: "head" var(--hd-head-h) "body" minmax(0, 1fr) "state" var(--hd-state-h) / minmax(0, 1fr);
  --hd-body-w: calc(100cqw - 2 * var(--hd-pad-i) - 2px);
  --hd-body-h: calc(100cqh - 2 * var(--hd-pad-b) - 2px - var(--hd-head-h) - var(--hd-state-h) - 2 * var(--hd-gap-in));
}
.hd-widget .hd-head { grid-area: head; }
.hd-widget .hd-title { font: var(--hd-label-weight) var(--hd-label-size) / var(--hd-head-h) var(--hd-font-label); letter-spacing: var(--hd-label-tracking); color: var(--hd-ink-muted); }
.hd-widget .hd-body { grid-area: body; display: grid; place-items: center; }
.hd-widget .hd-state { grid-area: state; font: 400 12.5px / var(--hd-state-h) var(--hd-font-ui); }
@container hd-widget (max-height: 139px) { .hd-widget > .hd-frame { --hd-pad-b: 10px; } }
@container hd-widget (max-width: 139px) { .hd-widget > .hd-frame { --hd-pad-i: 10px; } }
@container hd-widget (max-width: 139px) and (max-height: 139px) {
  .hd-widget > .hd-frame { --hd-pad-b: 8px; --hd-pad-i: 8px; --hd-head-h: 14px; --hd-state-h: 14px; --hd-gap-in: 3px; }
  .hd-widget .hd-title, .hd-widget .hd-state { font-size: 11px; }
}
```

| Size | 1×1 | 2×1 | 1×2 | 2×2 | 3×3 |
| --- | --- | --- | --- | --- | --- |
| Body w × h (stack layout) | 78 × 44 | 170 × 30 | 74 × 124 | 170 × 124 | 278 × 232 |

Shared structure in `homedash-primitives.css`:

```css
:where(.hd-dial-svg, .hd-gauge-svg) path { fill: none; }
.hd-toggle-key, .hd-toggle-legend, .hd-toggle-led, .hd-dial-glow, .hd-dial-lead, .hd-dial-gaps, .hd-dial-dots, .hd-dial-dots-lit, .hd-dial-ticks, .hd-dial-ticks-major,
.hd-dial-numeral, .hd-dial-needle, .hd-dial-knob-shadow, .hd-dial-knurl, .hd-dial-cap, .hd-dial-legend, .hd-dial-word, .hd-dial-caption, .hd-seg-rotary,
.hd-gauge-face, .hd-gauge-needle-glow, .hd-gauge-needle-shadow, .hd-gauge-glow, .hd-gauge-lead, .hd-gauge-gaps, .hd-gauge-arc-zone, .hd-spark-grid, .hd-spark-halo { display: none; }
.hd-gauge-needle-shadow { transform-box: view-box; transform-origin: 100px 100px; transform: translate(2px, 2px) rotate(var(--hd-angle, 0deg)); }
@keyframes hd-spin { to { transform: rotate(360deg); } }
@keyframes hd-blink { 0% { opacity: 1; } 50% { opacity: 0.25; } }
@keyframes hd-flash-a { 0%, 100% { opacity: 1; } 50% { opacity: 0.55; } }
@keyframes hd-flash-b { 0%, 100% { opacity: 1; } 50% { opacity: 0.55; } }
@keyframes hd-dip-a { 0%, 100% { opacity: 1; } 40% { opacity: 0.6; } }
@keyframes hd-dip-b { 0%, 100% { opacity: 1; } 40% { opacity: 0.6; } }
@keyframes hd-pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.6; } }
```

Value changes that flash or dip are keyed by `data-hd-tick` (`a` or `b`, flipped by the module on every displayed-value change): `[data-hd-tick="a"]` uses `hd-flash-a` or `hd-dip-a`, `[data-hd-tick="b"]` uses the `-b` copy, so the animation restarts. Text sizes per widget: label, state and hero numeral; units derive from the numeral (× 0.42).

### H1 Toggle switch: `switch` / `pill` (hd-w-switch.js)

**DOM** (C2 toggle with the Task 1 `decor` addition):

```
.hd-body > button.hd-toggle[type=button][role=switch][aria-label][aria-checked][data-hd-on]
  span.hd-toggle-track[aria-hidden=true]
    span.hd-toggle-glyph[data-glyph=on]  span.hd-toggle-glyph[data-glyph=off]  span.hd-toggle-key (addition)  span.hd-toggle-thumb
  span.hd-toggle-legend[data-glyph=off] "O"  span.hd-toggle-legend[data-glyph=on] "I"  span.hd-toggle-led   (additions)
```

**Geometry.** The track is 52×32 (ratio 1.625), the thumb 26 (h − 6), travel 20. The button (hit box) is 64×44. 2×1 uses the row layout; 1×1 uses the stack layout (body 78×44).

```css
.hd-toggle { --hd-tgl-w: 52px; --hd-tgl-h: 32px; --hd-tgl-inset: 3px; --hd-tgl-travel: calc(var(--hd-tgl-w) - var(--hd-tgl-h));
  position: relative; display: inline-grid; place-items: center; inline-size: 64px; block-size: 44px; padding: 0; border: 0; background: transparent;
  color: inherit; font: inherit; border-radius: var(--hd-radius-control); cursor: pointer; -webkit-tap-highlight-color: transparent; }
.hd-toggle-track { position: relative; inline-size: var(--hd-tgl-w); block-size: var(--hd-tgl-h); box-sizing: border-box; border-radius: var(--hd-radius-control); }
.hd-toggle-thumb { position: absolute; z-index: 1; inset-block-start: var(--hd-tgl-inset); inset-inline-start: var(--hd-tgl-inset);
  inline-size: calc(var(--hd-tgl-h) - 2 * var(--hd-tgl-inset)); block-size: calc(var(--hd-tgl-h) - 2 * var(--hd-tgl-inset)); border-radius: var(--hd-radius-control); }
.hd-toggle[aria-checked="true"] .hd-toggle-thumb { transform: translateX(var(--hd-tgl-travel)); }
.hd-toggle-glyph { display: none; position: absolute; pointer-events: none; }
.hd-toggle-key, .hd-toggle-legend, .hd-toggle-led { position: absolute; pointer-events: none; }
@container hd-widget (aspect-ratio >= 1.6) { .hd-widget[data-hd-type="switch"][data-hd-variant="pill"] > .hd-frame { grid-template: "head body" 1fr "state body" 1fr / minmax(0, 1fr) auto; } }
```

**Paint:**

```css
.hd-widget[data-hd-family="glass"] .hd-toggle-track { background: var(--hd-track); box-shadow: var(--hd-track-inset); }
.hd-widget[data-hd-family="glass"] .hd-toggle-track::before { content: ""; position: absolute; inset: 0; border-radius: inherit; background: var(--hd-track-fill); box-shadow: var(--hd-track-fill-hi); opacity: 0; }
.hd-widget[data-hd-family="glass"] .hd-toggle[aria-checked="true"] .hd-toggle-track::before { opacity: 1; }
.hd-widget[data-hd-family="glass"] .hd-toggle-glyph { display: block; z-index: 1; inset-block-start: 50%; translate: 0 -50%; }
.hd-widget[data-hd-family="glass"] .hd-toggle-glyph[data-glyph="on"] { inset-inline-start: 15px; inline-size: 2px; block-size: 10px; background: color-mix(in srgb, var(--hd-on-ink) 55%, transparent); }
.hd-widget[data-hd-family="glass"] .hd-toggle-glyph[data-glyph="off"] { inset-inline-start: 32px; inline-size: 8px; block-size: 8px; border-radius: var(--hd-radius-pill); box-shadow: inset 0 0 0 var(--hd-stroke-1) var(--hd-ink-muted); }
.hd-widget[data-hd-family="glass"] .hd-toggle-thumb { background: var(--hd-thumb); box-shadow: var(--hd-thumb-shadow); }
.hd-widget[data-hd-family="glass"] .hd-toggle-thumb::after { content: ""; position: absolute; inset: -3px; border-radius: var(--hd-radius-pill); background: var(--hd-pending-ring); -webkit-mask: var(--hd-ring-mask); mask: var(--hd-ring-mask); opacity: 0; }
.hd-widget[data-hd-family="glass"][data-hd-pending="true"] .hd-toggle-thumb::after { opacity: 1; }

.hd-widget[data-hd-family="tactile"] .hd-toggle { --hd-tgl-w: 48px; --hd-tgl-h: 14px; --hd-tgl-travel: 20px; }
.hd-widget[data-hd-family="tactile"] .hd-toggle-track { translate: -6px -6px; background: var(--hd-t-window); box-shadow: var(--hd-recess); }
.hd-widget[data-hd-family="tactile"] .hd-toggle-thumb { inset-block-start: -6px; inset-inline-start: 2px; inline-size: 24px; block-size: 26px; border-radius: var(--hd-radius-control);
  background: var(--hd-t-grip), var(--hd-t-steel-lin); box-shadow: var(--hd-lip), var(--hd-shadow-cast); }
.hd-widget[data-hd-family="tactile"] .hd-toggle-legend { display: block; inset-block-start: 31px; font: 600 11px / 11px var(--hd-font-label); color: var(--hd-t-legend); text-shadow: var(--hd-t-emboss); }
.hd-widget[data-hd-family="tactile"] .hd-toggle-legend[data-glyph="off"] { inset-inline-start: 4px; }
.hd-widget[data-hd-family="tactile"] .hd-toggle-legend[data-glyph="on"] { inset-inline-start: 44px; }
.hd-widget[data-hd-family="tactile"] .hd-toggle-led { display: block; inset-inline-end: 2px; inset-block-start: 13px; inline-size: 6px; block-size: 6px; border-radius: var(--hd-radius-pill); background: var(--hd-t-led-off); }
.hd-widget[data-hd-family="tactile"] .hd-toggle-led::after { content: ""; position: absolute; inset: 0; border-radius: inherit; background: var(--hd-t-led); box-shadow: var(--hd-t-led-halo); opacity: 0; }
.hd-widget[data-hd-family="tactile"]:is([data-hd-on="true"], [data-hd-pending="true"], [data-hd-tone="crit"], [data-hd-status="error"]) .hd-toggle-led::after { opacity: 1; }

.hd-widget[data-hd-family="minimal"] .hd-toggle { --hd-tgl-inset: 6px; }
.hd-widget[data-hd-family="minimal"] .hd-toggle-track { box-shadow: inset 0 0 0 var(--hd-stroke-1) var(--hd-track-line); }
.hd-widget[data-hd-family="minimal"] .hd-toggle-track::before { content: ""; position: absolute; inset: 0; border-radius: inherit; background: var(--hd-track-fill); opacity: 0; }
.hd-widget[data-hd-family="minimal"] .hd-toggle[aria-checked="true"] .hd-toggle-track::before { opacity: 1; }
.hd-widget[data-hd-family="minimal"] .hd-toggle-thumb { background: var(--hd-thumb); }
.hd-widget[data-hd-family="minimal"] .hd-toggle[aria-checked="true"] .hd-toggle-thumb { background: var(--hd-thumb-on); }
.hd-widget[data-hd-family="minimal"] .hd-toggle-thumb::after { content: ""; position: absolute; inset: -4px; border-radius: var(--hd-radius-pill); box-shadow: inset 0 0 0 var(--hd-stroke-hair) var(--hd-ink); -webkit-mask: var(--hd-arc-mask-75); mask: var(--hd-arc-mask-75); opacity: 0; }
.hd-widget[data-hd-family="minimal"][data-hd-pending="true"] .hd-toggle-thumb::after { opacity: 1; }

.hd-widget[data-hd-family="instrument"] .hd-toggle { --hd-tgl-w: 56px; --hd-tgl-h: 30px; }
.hd-widget[data-hd-family="instrument"] .hd-toggle-thumb { display: none; }
.hd-widget[data-hd-family="instrument"] .hd-toggle-track { background: color-mix(in srgb, var(--hd-i-hud) 4%, transparent); box-shadow: inset 0 0 0 var(--hd-stroke-hair) color-mix(in srgb, var(--hd-i-hud) 22%, transparent); }
.hd-widget[data-hd-family="instrument"] .hd-toggle-track::before, .hd-widget[data-hd-family="instrument"] .hd-toggle-track::after { content: ""; position: absolute; inset: 3px 6px auto; block-size: 3px; border-radius: 0; }
.hd-widget[data-hd-family="instrument"] .hd-toggle-track::before { background: color-mix(in srgb, var(--hd-i-hud) 12%, transparent); }
.hd-widget[data-hd-family="instrument"] .hd-toggle-track::after { background: var(--hd-i-val); box-shadow: var(--hd-glow); opacity: 0; }
.hd-widget[data-hd-family="instrument"] .hd-toggle[aria-checked="true"] .hd-toggle-track::after { opacity: 1; }
.hd-widget[data-hd-family="instrument"][data-hd-pending="true"] .hd-toggle-track::after { opacity: 0.5; }
.hd-widget[data-hd-family="instrument"] .hd-toggle-key { display: block; inset: 9px 0 0; text-align: center; font: 400 13px / 18px var(--hd-font-num); color: color-mix(in srgb, var(--hd-i-hud) 60%, transparent); }
.hd-widget[data-hd-family="instrument"] .hd-toggle[aria-checked="true"] .hd-toggle-key { color: var(--hd-i-val); }
```

**States** (the shared ones are in §T5):

| State | Glass | Tactile | Minimal | Instrument |
| --- | --- | --- | --- | --- |
| off | Thumb left, O ring, track wash | Lever left, LED dark | Outline track, ink thumb left | Bar at 12 %, key "O" at HUD 60 % |
| on | Thumb right, on fill, I bar | Lever right, green LED | Ink fill, surface thumb right | Bar lit with glow, key "I" at HUD 100 % |
| pending | Static quarter ring; MG spins it 1 s linear infinite | Amber LED; MG blinks it at 2 Hz | Static 270° ring; MG spins it 1.2 s linear | Bar at 0.5; MG blinks it (`hd-blink`, 4 × `--hd-dur-chase`, steps(1)) |
| alert | On position plus §T5 alert | Red LED and rim | Triangle badge, word | Bar and key in alert, pulse |

**Motion** (all inside MG):

| Family | Element and property | Duration | Easing |
| --- | --- | --- | --- |
| Glass | thumb transform | `--hd-dur-toggle` | `--hd-ease-toggle` |
| Glass | track::before opacity | `--hd-dur-fill` | `--hd-ease-standard` |
| Tactile | thumb transform | `--hd-dur-toggle` | `--hd-ease-snap` |
| Tactile | LED::after opacity | `--hd-dur-led`, delay `--hd-dur-toggle` | linear |
| Minimal | thumb transform and track::before opacity | `--hd-dur-toggle` | `--hd-ease-standard` |
| Instrument | track::after opacity | `--hd-dur-toggle` | `--hd-ease-hud` |
| Instrument | flash on `.hd-toggle[data-hd-tick]` | `hd-flash-a`/`-b` over `--hd-dur-flash` | — |

Reduced or none: no transitions, and the static pending forms.

**Accessibility:**
- `role=switch`, `aria-checked`, `aria-label` = widget title.
- Unavailable or disabled: `aria-disabled`. Read-only: `aria-readonly` (C2).

**Fixtures:** `switch/pill` in `hero-fixtures.js`. Lab sizes 1×1 and 2×1. Scenarios: `toggle`, `pending`.

### H2 Rocker switch: `switch` / `rocker` (hd-w-switch.js)

**DOM** (C2 rocker; no additions):
- `button.hd-rocker[type=button][role=switch][aria-label][aria-checked][data-hd-on]` > `span.hd-rocker-bezel[aria-hidden=true]`
- the bezel holds `span.hd-rocker-paddle` > (`span.hd-rocker-half[data-half=on]` > `span.hd-rocker-legend` "I"; `span.hd-rocker-half[data-half=off]` > `span.hd-rocker-legend` "O"), then `span.hd-rocker-led`.

**Geometry.** The button (hit box) is `min(body-h, body-w / 0.654)` high with aspect 0.654: 124×81 at 2×2, 113×74 at 1×2. The bezel fills it with a 6 % inset (padding 6 % of its width). The halves split 50/50.

```css
.hd-rocker { display: block; padding: 0; border: 0; background: transparent; cursor: pointer; block-size: min(var(--hd-body-h), calc(var(--hd-body-w) / 0.654)); aspect-ratio: 0.654; border-radius: var(--hd-radius-control); }
.hd-rocker-bezel { position: relative; display: block; box-sizing: border-box; inline-size: 100%; block-size: 100%; padding: 6%; perspective: 420px; border-radius: inherit; }
.hd-rocker-paddle { position: relative; display: grid; grid-template-rows: 1fr 1fr; block-size: 100%; transform-style: preserve-3d; border-radius: var(--hd-radius-control); }
.hd-rocker-half { position: relative; display: grid; place-items: center; }
.hd-rocker-led { position: absolute; inset-block-start: 4%; inset-inline-end: 4%; inline-size: 6px; block-size: 6px; display: none; }
```

**Paint:**

```css
.hd-widget[data-hd-family="tactile"] .hd-rocker-bezel { background: var(--hd-t-panel); box-shadow: var(--hd-lip), var(--hd-shadow-cast); border-radius: var(--hd-t-bezel-r); }
.hd-widget[data-hd-family="tactile"] .hd-rocker-paddle { background: var(--hd-t-bakelite); box-shadow: var(--hd-shadow-contact); transform: rotateX(-9deg); }
.hd-widget[data-hd-family="tactile"] .hd-rocker[aria-checked="true"] .hd-rocker-paddle { transform: rotateX(9deg); }
.hd-widget[data-hd-family="tactile"] .hd-rocker-half::after { content: ""; position: absolute; inset: 0; border-radius: inherit; background: var(--hd-t-shade); opacity: 0; }
.hd-widget[data-hd-family="tactile"] .hd-rocker[aria-checked="true"] .hd-rocker-half[data-half="on"]::after,
.hd-widget[data-hd-family="tactile"] .hd-rocker[aria-checked="false"] .hd-rocker-half[data-half="off"]::after { opacity: 1; }
.hd-widget[data-hd-family="tactile"] .hd-rocker-legend { font: 600 clamp(11px, 14cqmin, 16px) / 1 var(--hd-font-label); color: var(--hd-t-legend); text-shadow: var(--hd-t-emboss); }
.hd-widget[data-hd-family="tactile"] .hd-rocker-led { display: block; border-radius: var(--hd-radius-pill); background: var(--hd-t-led-off); }
.hd-widget[data-hd-family="tactile"] .hd-rocker-led::after { content: ""; position: absolute; inset: 0; border-radius: inherit; background: var(--hd-t-led); box-shadow: var(--hd-t-led-halo); opacity: 0; }
.hd-widget[data-hd-family="tactile"]:is([data-hd-on="true"], [data-hd-pending="true"], [data-hd-tone="crit"], [data-hd-status="error"]) .hd-rocker-led::after { opacity: 1; }

.hd-widget[data-hd-family="glass"] .hd-rocker-bezel { background: var(--hd-surface); box-shadow: var(--hd-edge-hi), var(--hd-shadow-contact); border-radius: var(--hd-g-bezel-r); }
.hd-widget[data-hd-family="glass"] .hd-rocker-paddle { background: var(--hd-g-sheen), color-mix(in srgb, var(--hd-edge-tint) 10%, var(--hd-surface)); box-shadow: var(--hd-edge-hi), var(--hd-edge-lo); border-radius: var(--hd-g-paddle-r); }
.hd-widget[data-hd-family="glass"] .hd-rocker-half + .hd-rocker-half { box-shadow: inset 0 var(--hd-stroke-hair) 0 var(--hd-hairline); }
.hd-widget[data-hd-family="glass"] .hd-rocker-half::before { content: ""; position: absolute; inset: 0; border-radius: inherit; background: color-mix(in srgb, var(--hd-on) 70%, transparent); box-shadow: var(--hd-shadow-press); opacity: 0; }
.hd-widget[data-hd-family="glass"] .hd-rocker[aria-checked="true"] .hd-rocker-half[data-half="on"]::before { opacity: 1; }
.hd-widget[data-hd-family="glass"] .hd-rocker[aria-checked="false"] .hd-rocker-half[data-half="off"]::before { opacity: 1; background: transparent; }
.hd-widget[data-hd-family="glass"] .hd-rocker-legend { position: relative; font: 500 15px / 1 var(--hd-font-ui); color: var(--hd-ink-muted); }
.hd-widget[data-hd-family="glass"] .hd-rocker[aria-checked="true"] .hd-rocker-half[data-half="on"] .hd-rocker-legend { color: var(--hd-on-ink); }

.hd-widget[data-hd-family="minimal"] .hd-rocker-bezel { padding: 0; box-shadow: inset 0 0 0 var(--hd-stroke-1) var(--hd-ink); }
.hd-widget[data-hd-family="minimal"] .hd-rocker-half + .hd-rocker-half { box-shadow: inset 0 var(--hd-stroke-hair) 0 var(--hd-hairline); }
.hd-widget[data-hd-family="minimal"] .hd-rocker-half::before { content: ""; position: absolute; inset: 0; background: var(--hd-ink); opacity: 0; }
.hd-widget[data-hd-family="minimal"] .hd-rocker[aria-checked="true"] .hd-rocker-half[data-half="on"]::before,
.hd-widget[data-hd-family="minimal"] .hd-rocker[aria-checked="false"] .hd-rocker-half[data-half="off"]::before { opacity: 1; }
.hd-widget[data-hd-family="minimal"] .hd-rocker-legend { position: relative; font: 500 15px / 1 var(--hd-font-ui); color: var(--hd-ink); }
.hd-widget[data-hd-family="minimal"] .hd-rocker[aria-checked="true"] .hd-rocker-half[data-half="on"] .hd-rocker-legend,
.hd-widget[data-hd-family="minimal"] .hd-rocker[aria-checked="false"] .hd-rocker-half[data-half="off"] .hd-rocker-legend { color: var(--hd-surface-solid); }

.hd-widget[data-hd-family="instrument"] .hd-rocker-bezel { padding: 0; }
.hd-widget[data-hd-family="instrument"] .hd-rocker-paddle { gap: 4px; }
.hd-widget[data-hd-family="instrument"] .hd-rocker-half { box-shadow: inset 0 0 0 var(--hd-stroke-hair) color-mix(in srgb, var(--hd-i-hud) 30%, transparent); border-radius: var(--hd-radius-control); }
.hd-widget[data-hd-family="instrument"] .hd-rocker-half::after { content: ""; position: absolute; inset: auto 8px 4px; block-size: 2px; background: var(--hd-i-val); box-shadow: var(--hd-glow); opacity: 0; }
.hd-widget[data-hd-family="instrument"] .hd-rocker[aria-checked="true"] .hd-rocker-half[data-half="on"]::after,
.hd-widget[data-hd-family="instrument"] .hd-rocker[aria-checked="false"] .hd-rocker-half[data-half="off"]::after { opacity: 1; }
.hd-widget[data-hd-family="instrument"] .hd-rocker-legend { font: 400 15px / 1 var(--hd-font-num); color: color-mix(in srgb, var(--hd-i-hud) 30%, transparent); }
.hd-widget[data-hd-family="instrument"] .hd-rocker[aria-checked="true"] .hd-rocker-half[data-half="on"] .hd-rocker-legend,
.hd-widget[data-hd-family="instrument"] .hd-rocker[aria-checked="false"] .hd-rocker-half[data-half="off"] .hd-rocker-legend { color: var(--hd-i-val); }
```

**States:**
- pending: Tactile amber LED blink. Glass shows a quarter ring at the pressed half (`::after`, `--hd-pending-ring` with `--hd-ring-mask`). Minimal shows "…" in the state word plus a hairline ring (`box-shadow`) around the bezel. Instrument blinks the active underline.
- off and on as painted. Alert per §T5.

**Motion** (all inside MG):
- Tactile paddle transform over `--hd-dur-toggle` / `--hd-ease-snap`; shade opacity in the same time.
- Glass and Minimal: halves `::before` opacity over `--hd-dur-fill`.
- Instrument: underline opacity over `--hd-dur-toggle`, plus the flash.

**Accessibility:** as H1. **Fixtures:** `switch/rocker`. Lab sizes 1×2 and 2×2. Scenarios: `toggle`, `pending`.

### H3 Dimmer dial: `light` / `dial` (hd-w-light.js)

**Primitive call.**

```
E.prim.knob({label, variant:'ring', min:0, max:100, unit:'%', decimals:0, ticks:10, legends:true,
  readout: inst ? 'seg7' : 'numeral', quantize: inst ? 40 : 0, gaps: inst ? 40 : 0, lead: inst, glow: glass || inst,
  dots: tact ? {count:21, r:94, size:4.5, lit:true} : null, hardware: tact, word: true, handleShadow: glass})
```

**DOM** (C2 plus additions marked *):

```
div.hd-dial[role=slider][tabindex=0][data-hd-variant=ring][data-hd-quantize?][aria-label][aria-value*][data-hd-word?]
  svg.hd-dial-svg[viewBox="0 0 200 200"]
    path.hd-dial-dots* path.hd-dial-dots-lit* path.hd-dial-ticks (11 marks, r 80–88) path.hd-dial-track[pathLength=1000]
    path.hd-dial-glow*[pathLength=1000] path.hd-dial-arc[pathLength=1000] path.hd-dial-lead* path.hd-dial-gaps*[pathLength=1000]
    g.hd-dial-handle > circle.hd-dial-knob-shadow*[cx=100][cy=13.5][r=11], circle.hd-dial-knob[cx=100][cy=12][r=11]
  span.hd-dial-knurl* span.hd-dial-cap > span.hd-dial-pointer
  span.hd-dial-readout > span.hd-numeral | span.hd-seg7, span.hd-dial-word*
  span.hd-dial-legend[data-end=min] "0"  span.hd-dial-legend[data-end=max] "100"
```

**Geometry.**
- D = min(body-w, body-h): 124 at 2×2, 232 at 3×3. Track and arc are `arc(100,100,88,-135,270)` (C2).
- Glass stroke 15 units. Minimal: track 2px, arc 3px (non-scaling). Instrument: 16 units plus the gap overlay.
- Tactile:
  - LED dots: 21 at r 94, radius 4.5.
  - Ticks: 11 marks, r 80–88.
  - Knurl: ring from r 74 to r 64 (inset 13 %, `--hd-t-ring-86` mask).
  - Steel cap: ⌀ 62 % D (C2 `.hd-dial-cap`, inset 19 %).
  - Pointer: from r 58 to r 22, 4 units wide, rotated by `--hd-angle`.
- Readout numeral: `D × --hd-dial-num-k` (0.26; Minimal 0.28), unit × 0.42. Instrument seven-segment readout height 0.20 D.
- Legends at the arc ends (r 84, ±135°).

```css
.hd-dial { position: relative; box-sizing: border-box; inline-size: min(var(--hd-body-w), var(--hd-body-h)); aspect-ratio: 1; border-radius: var(--hd-radius-pill); touch-action: none; }
.hd-dial-svg { position: absolute; inset: 0; inline-size: 100%; block-size: 100%; overflow: visible; }
.hd-dial-readout { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; pointer-events: none; }
.hd-dial-readout > .hd-numeral { font-size: calc(min(var(--hd-body-w), var(--hd-body-h)) * var(--hd-dial-num-k)); line-height: 1; }
.hd-dial-readout > .hd-seg7 { block-size: calc(min(var(--hd-body-w), var(--hd-body-h)) * 0.2); }
.hd-dial[data-hd-word="true"] .hd-dial-readout > :is(.hd-numeral, .hd-seg7) { display: none; }
.hd-dial[data-hd-word="true"] .hd-dial-word { display: block; font-size: calc(min(var(--hd-body-w), var(--hd-body-h)) * var(--hd-dial-num-k) * 0.42); color: var(--hd-ink-muted); }
.hd-dial-knurl { position: absolute; inset: 13%; border-radius: var(--hd-radius-pill); }
.hd-dial-cap { position: absolute; inset: 19%; border-radius: var(--hd-radius-pill); }
.hd-dial-pointer { position: absolute; inset: 3.2% 48.4% 50% 48.4%; transform-origin: 50% 100%; transform: rotate(var(--hd-angle, 0deg)); }
.hd-dial-legend { position: absolute; inset-block-start: 79.7%; translate: -50% -50%; font: 600 10px / 1 var(--hd-font-label); }
.hd-dial-legend[data-end="min"] { inset-inline-start: 20.3%; } .hd-dial-legend[data-end="max"] { inset-inline-start: 79.7%; }
```

**Paint:**

```css
.hd-widget[data-hd-family="glass"] .hd-dial-track { stroke: var(--hd-track); stroke-width: calc(var(--hd-stroke-arc) * 200); stroke-linecap: round; }
.hd-widget[data-hd-family="glass"] .hd-dial-glow { display: inline; stroke: var(--hd-light-on); stroke-width: calc(var(--hd-stroke-arc) * 200 + 6); stroke-linecap: round; opacity: var(--hd-arc-underglow); }
.hd-widget[data-hd-family="glass"] .hd-dial-arc { stroke: var(--hd-light-on); stroke-width: calc(var(--hd-stroke-arc) * 200); stroke-linecap: round; }
.hd-widget[data-hd-family="glass"] .hd-dial-knob { fill: var(--hd-thumb); stroke: var(--hd-thumb-edge); stroke-width: var(--hd-g-edge-u); }
.hd-widget[data-hd-family="glass"] .hd-dial-knob-shadow { display: inline; fill: var(--hd-shadow-ink); opacity: var(--hd-g-shadow-a); }
.hd-widget[data-hd-family="glass"] .hd-dial-readout > .hd-numeral { font-weight: var(--hd-num-weight); letter-spacing: var(--hd-num-tracking); color: var(--hd-ink); }

.hd-widget[data-hd-family="tactile"] :is(.hd-dial-track, .hd-dial-arc, .hd-dial-handle, .hd-dial-readout) { display: none; }
.hd-widget[data-hd-family="tactile"] :is(.hd-dial-dots, .hd-dial-dots-lit, .hd-dial-ticks, .hd-dial-legend) { display: inline; }
.hd-widget[data-hd-family="tactile"] :is(.hd-dial-knurl, .hd-dial-cap) { display: block; }
.hd-widget[data-hd-family="tactile"] .hd-dial-knurl { background: var(--hd-t-knurl); -webkit-mask: var(--hd-t-ring-86); mask: var(--hd-t-ring-86); box-shadow: var(--hd-shadow-cast); }
.hd-widget[data-hd-family="tactile"] .hd-dial-cap { background: var(--hd-t-steel); box-shadow: var(--hd-lip), var(--hd-shadow-contact); }
.hd-widget[data-hd-family="tactile"] .hd-dial-pointer { background: var(--hd-t-pointer); }
.hd-widget[data-hd-family="tactile"] .hd-dial-ticks { stroke: var(--hd-t-legend); stroke-width: var(--hd-stroke-tick); vector-effect: non-scaling-stroke; }
.hd-widget[data-hd-family="tactile"] .hd-dial-dots { fill: var(--hd-t-dot-off); }
.hd-widget[data-hd-family="tactile"] .hd-dial-dots-lit { fill: var(--hd-t-amber); }
.hd-widget[data-hd-family="tactile"] .hd-dial-legend { color: var(--hd-t-legend); text-shadow: var(--hd-t-emboss); }

.hd-widget[data-hd-family="minimal"] .hd-dial-track { stroke: var(--hd-hairline); stroke-width: var(--hd-stroke-2); vector-effect: non-scaling-stroke; }
.hd-widget[data-hd-family="minimal"] .hd-dial-arc { stroke: var(--hd-ink); stroke-width: var(--hd-m-arc-w); vector-effect: non-scaling-stroke; stroke-linecap: butt; }
.hd-widget[data-hd-family="minimal"] .hd-dial-knob { fill: var(--hd-ink); r: var(--hd-m-dot-r); }
.hd-widget[data-hd-family="minimal"] .hd-dial-readout > .hd-numeral { font-weight: var(--hd-num-weight); letter-spacing: var(--hd-num-tracking); }

.hd-widget[data-hd-family="instrument"] .hd-dial-handle { display: none; }
.hd-widget[data-hd-family="instrument"] .hd-dial-track { stroke: color-mix(in srgb, var(--hd-i-hud) 10%, transparent); stroke-width: calc(var(--hd-stroke-arc) * 200); }
.hd-widget[data-hd-family="instrument"] .hd-dial-arc { stroke: var(--hd-i-val); stroke-width: calc(var(--hd-stroke-arc) * 200); opacity: 0.7; }
.hd-widget[data-hd-family="instrument"] .hd-dial-glow { display: inline; stroke: var(--hd-i-val); stroke-width: calc(var(--hd-stroke-arc) * 200 + var(--hd-i-glow-add)); opacity: 0.25; }
.hd-widget[data-hd-family="instrument"] .hd-dial-lead { display: inline; stroke: var(--hd-i-val); stroke-width: calc(var(--hd-stroke-arc) * 200); }
.hd-widget[data-hd-family="instrument"] .hd-dial-gaps { display: inline; stroke: var(--hd-i-mid); stroke-width: calc(var(--hd-stroke-arc) * 200 + var(--hd-i-gap-add)); stroke-dasharray: var(--hd-i-gap-dash); }
.hd-widget[data-hd-family="instrument"] .hd-seg7-seg { fill: var(--hd-i-ghost); }
.hd-widget[data-hd-family="instrument"] .hd-seg7-seg[data-lit="true"], .hd-widget[data-hd-family="instrument"] .hd-seg7-dp[data-lit="true"] { fill: var(--hd-i-val); }
```

The Instrument glow uses the arc's dasharray, so it covers the lit segments; the gap overlay paints the gaps in the panel colour over track, arc, glow and lead.

**States:**

| State | Glass | Tactile | Minimal | Instrument |
| --- | --- | --- | --- | --- |
| off | Empty arc, handle at start, word "Off" | Pointer at 0, all dots dark | Empty arc, dot at start, "Off" | No lit segments, word "Off" |
| on (64 %) | Warm arc to 64, handle at 64, "64 %" | Pointer at 64, 13 dots lit, state line "On · 64 %" | Ink arc, "64 %" | 26 segments, lead at 100 %, seven-segment "64" |
| min (1 %) / max (100 %) | Stub with round cap / full arc | 1 / 21 dots | Hairline stub / full | 1 / 40 segments |
| pending | Quarter ring around the handle (`::after` is not available on SVG, so `.hd-dial::after` with `--hd-pending-ring` masked to a ring at the handle radius, static); MG spins it | Lit dots blink amber (opacity, MG) | Ring as in Glass with the 75 % mask | Lead blinks (`hd-blink`) |

**Motion** (all inside MG). The value arc sweeps in step with the handle:

| Family | Element and property | Duration | Easing |
| --- | --- | --- | --- |
| Glass, Minimal | `.hd-dial-arc`, `.hd-dial-glow` stroke-dasharray | `--hd-dur-remote` | `--hd-ease-remote` |
| Glass, Minimal | `.hd-dial-handle` transform | same | same |
| Tactile | `.hd-dial-pointer` transform; dots switch instantly | `--hd-dur-remote` | `--hd-ease-remote` |
| Instrument | quantized arc snaps; flash `hd-flash-a`/`-b` on `.hd-dial-readout[data-hd-tick]` | `--hd-dur-flash` | — |

**Accessibility:**
- `role=slider`, min 0, max 100, `aria-valuenow` = percentage.
- `aria-valuetext` = `t('homedash.light.valuetext', {value, state})` ("64 %, on") or "Off".

**Fixtures:** `light/dial`. Lab sizes 2×2 (min = default) and 3×3. Scenarios: `remote` (20→80), `pending`.

### H4 Thermostat dial: `climate` / `dial` (hd-w-climate.js)

**Primitive calls.**

```
E.prim.knob({label, variant:'ring', min:min_temp, max:max_temp, step:0.5, decimals:1, unit:DEG_C, activeSegment:true, legends:true,
  ticks: inst||tact ? (max-min)*2 : 0, ticksMajor: tact ? (max-min)/5 : 0, numerals: tact ? 6 : 0,
  dots: glass ? {count:max-min+1, r:74, size:1.4, lit:false} : null, hardware: tact, needle: tact, handleShadow: glass,
  readout: inst ? 'seg7' : 'numeral', extra: true, subReadout: inst ? 'seg7' : 'numeral', subIcon: 'thermometer',
  captions: inst ? [t('homedash.climate.target'), t('homedash.climate.current')] : null})
E.prim.pushButton({label: t('homedash.climate.lower'), icon:'minus', dir:-1, variant:'chip'}) and {label: t('homedash.climate.raise'), icon:'plus', dir:1}
E.prim.segmented({label, options:[off|power, heat|flame, cool|snowflake, auto|mode-auto], rotary: tact})
```

**DOM:**

```
.hd-body[data-hd-action=heating|cooling|idle|off] > div.hd-climate-dialbox > (knob.el, div.hd-climate-steps > button.hd-push[data-dir=-1], button.hd-push[data-dir=1]); seg.el
knob adds: path.hd-dial-active[data-buf=a|b], g.hd-dial-current > path.hd-dial-marker (C2), path.hd-dial-needle* (in g.hd-dial-current),
           path.hd-dial-ticks-major*, text.hd-dial-numeral*, readout: span.hd-dial-caption*[data-for=main], span.hd-dial-action*, main readout,
           span.hd-dial-sub* (svg.hd-ico thermometer + numeral or seg7), span.hd-dial-caption*[data-for=sub]
seg adds: span.hd-seg-rotary* > span.hd-seg-rotary-pointer
```

**Geometry.**
- Body grid: rows `minmax(0,1fr) 44px`, gap 8.
- Dialbox D = `--hd-th-d` = `0.86 × min(body-w, body-h − 52px)`: 155 at 3×3. Below `(max-height: 259px)` (2×2), D = `0.86 × min(body-w, body-h)` (107) and the mode bar and steppers are hidden: 44 px buttons do not fit the gap chord at D 107.
- Track and active segment: r 88 (C2), stroke `200 × --hd-stroke-arc`; T0 sets the Glass and Minimal default and the thermostat uses 0.06 × 200 = 12 units in every family, via `--hd-th-stroke: 12`, set in each family block as part of Task 6, 11, 14 and 17's token additions.
- The current marker (C2: apex at r 100, base at r 91) is moved out by `translate: 0 -5px` (user units), so its base sits at r 96, clear of the stroke edge at r 94.
- Glass dots: every 1 °C at r 74, radius 1.4. Instrument ticks: per 0.5 °C at r 80–88.
- Tactile: steel cap and pointer to the target (cap inset 21 %, about ⌀ 58 % D); minor ticks per 0.5 °C at r 80–88; majors per 5 °C at r 76–88; numerals 5…30 at r 67 (hidden at D < 140 except 10, 20 and 30); red needle from r 86 to r 58 at the current angle.
- Steppers: centres at x 33.5 % and 66.5 %, y 87 % of D, 44×44, `translate: -50% -50%`.
- Readout stack: the action line in label style; the target at `D × --hd-thermo-num-k`; the current value at 0.32 × that with the thermometer icon. Instrument: target seven-segment at 0.18 D and current at 0.08 D, each with an Oxanium caption.
- Mode bar: 44 px tall, width `min(100%, 260px)`. Tactile rotary: a 44 px knob with the four options as 22 px icon legends at −45°, −15°, 15° and 45° on a 40 px radius; pointer `rotate(calc(var(--hd-index) * 30deg - 45deg))`.

```css
.hd-widget[data-hd-type="climate"] .hd-body { grid-template-rows: minmax(0, 1fr) 44px; row-gap: 8px; --hd-th-d: calc(min(var(--hd-body-w), var(--hd-body-h) - 52px) * 0.86); }
@container hd-widget (max-height: 259px) { .hd-widget[data-hd-type="climate"] .hd-body { grid-template-rows: minmax(0, 1fr); --hd-th-d: calc(min(var(--hd-body-w), var(--hd-body-h)) * 0.86); }
  .hd-widget[data-hd-type="climate"] :is(.hd-seg, .hd-climate-steps) { display: none; } }
.hd-climate-dialbox { position: relative; inline-size: var(--hd-th-d); block-size: var(--hd-th-d); }
.hd-climate-dialbox .hd-dial { inline-size: 100%; }
.hd-climate-steps .hd-push { position: absolute; inline-size: 44px; block-size: 44px; translate: -50% -50%; inset-block-start: 87%; }
.hd-climate-steps .hd-push[data-dir="-1"] { inset-inline-start: 33.5%; } .hd-climate-steps .hd-push[data-dir="1"] { inset-inline-start: 66.5%; }
.hd-climate-steps .hd-push-label { position: absolute; inline-size: 1px; block-size: 1px; overflow: hidden; clip: rect(0 0 0 0); }
.hd-widget[data-hd-type="climate"] .hd-dial-readout > .hd-numeral { font-size: calc(var(--hd-th-d) * var(--hd-thermo-num-k)); }
.hd-widget[data-hd-type="climate"] .hd-dial-marker { translate: 0 -5px; }
.hd-body[data-hd-action="heating"] { --hd-act: var(--hd-heat); } .hd-body[data-hd-action="cooling"] { --hd-act: var(--hd-cool); }
.hd-body[data-hd-action="idle"] { --hd-act: color-mix(in srgb, var(--hd-ink) 35%, transparent); } .hd-body[data-hd-action="off"] .hd-dial-active { display: none; }
```

**Paint** (in every family the active segment has `stroke: var(--hd-act); stroke-width: calc(var(--hd-th-stroke) * 1)`):
- **G:** round caps. Handle = glass puck (`fill: var(--hd-thumb)`, `stroke: var(--hd-act)`, `stroke-width: var(--hd-th-ring-u)` with `--hd-th-ring-u: 2.5` in T1). Dots `fill: color-mix(in srgb, var(--hd-ink) 25%, transparent)`. Marker `fill: var(--hd-ink-muted)`. Steppers `background: var(--hd-surface); box-shadow: var(--hd-edge-hi), var(--hd-shadow-contact); border-radius: var(--hd-radius-pill)`. Mode: `.hd-seg-indicator { background: var(--hd-thumb); box-shadow: var(--hd-thumb-shadow); transform: translateX(calc(var(--hd-index) * 100%)); inline-size: calc(100% / var(--hd-count)); }`.
- **T:** track and handle hidden. Face = the card panel with ticks (`--hd-t-legend`, `--hd-stroke-tick`, non-scaling). Cap, pointer and knurl as in H3. Needle `stroke: var(--hd-t-needle); stroke-width: var(--hd-t-needle-w)`. Active segment and marker hidden; the action shows as `E.prim.lamp` beside the readout plate (Tactile LED recipe on `.hd-lamp-dot`, lit when heating or cooling). Steppers `background: var(--hd-t-bakelite); box-shadow: var(--hd-lip), var(--hd-shadow-cast); border-radius: var(--hd-radius-control)`. Rotary: `.hd-seg-rotary` shown with steel cap and pointer; `.hd-seg-indicator` hidden.
- **M:** active segment 3px non-scaling (`var(--hd-m-arc-w)`), butt caps. Handle dot `r: var(--hd-m-dot-r)`. No ticks. Steppers are text buttons with `box-shadow: inset 0 0 0 var(--hd-stroke-hair) var(--hd-hairline)`. Mode indicator: 2px ink underline.
- **I:** active segment glows (a duplicate is not available on the active path, so the glow is `filter`-free: the active path gets `stroke-width: calc(var(--hd-th-stroke) + var(--hd-i-glow-add))` at 0.25 opacity in buffer `a`'s sibling? No — the glow comes from the ticks inside the segment span tinted `--hd-act` at 100 %, while other ticks stay HUD 55 %). Marker in HUD. Steppers are bracket keys (`box-shadow: inset 0 0 0 var(--hd-stroke-hair) color-mix(in srgb, var(--hd-i-hud) 30%, transparent)`). Mode: lit tab, `.hd-seg-indicator` a 2px `--hd-i-val` bar with `box-shadow: var(--hd-glow)`.

**States:**

| State | Glass | Tactile | Minimal | Instrument |
| --- | --- | --- | --- | --- |
| off | Track only, action line "Off", mode = off | LED dark, rotary on Off | as Glass | Readouts at HUD 30 % |
| on (heat 21.5 / 19.5) | Orange segment 19.5→21.5 | Needle at 19.5, pointer at 21.5, LED lit | Orange line | Ticks 19.5–21.5 tinted, target "21.5" |
| min (target 5, idle) | Idle segment 5→19.5 | Pointer at 5 | idle line | idle tint |
| max (target 30) | Handle at the end | Pointer at the end | as Glass | as Glass |
| readonly (cool 22 / 24.5) | Cool segment, lock badge | Pointer without shadow, lock | cool line | cool tint |
| pending | Ring on the handle | LED amber blink | Ring around the dot | Target readout blinks |

**Motion** (all inside MG):
- Handle and cap pointer transform, `g.hd-dial-current` transform: `--hd-dur-remote` / `--hd-ease-remote`.
- Active segment: buffered pair, opacity over `--hd-dur-xfade`.
- Mode indicator transform: `--hd-dur-fill`. Rotary pointer: `--hd-dur-toggle` / `--hd-ease-snap`.

**Accessibility:**
- Dial: `role=slider`, min and max from the entity, `aria-valuenow` = target, `aria-valuetext` = `t('homedash.climate.valuetext', {value, action})`.
- Steppers: label text from the keys above.
- Mode: radiogroup with checked radios (C2).

**Fixtures:** `climate/dial`. Lab sizes 2×2 and 3×3. Scenarios: `target` (19→23), `pending`.

### H5 Needle gauge: `gauge` / `needle` (hd-w-sensor.js)

**Primitive call:** `E.prim.needleGauge({label, min, max, unit, decimals:0, zones, majors:10, minors:50, numerals:6, crop: tact ? 200 : 168, face: tact, needleFx: tact || inst, readout: inst ? 'seg7' : 'numeral'})`.

**DOM** (C2 plus additions marked *):

```
div.hd-gauge[data-hd-gauge=needle][role=meter][aria-label][aria-value*][data-hd-tone?]
  span.hd-gauge-face* > span.hd-gauge-bezel*, span.hd-gauge-glass*
  svg.hd-gauge-svg[viewBox="0 0 200 168|200"]
    path.hd-gauge-ticks[data-major=true] (r 72–88), path.hd-gauge-ticks[data-minor=true] (r 80–88), path.hd-gauge-zone[data-tone]* (r 84),
    text.hd-gauge-numeral ×6 (r 54), g.hd-gauge-needle-shadow* > path, g.hd-gauge-needle > path.hd-gauge-needle-glow*, path.hd-gauge-needle-shape, circle.hd-gauge-hub[r=9]
  span.hd-gauge-readout > numeral | seg7;  span.hd-gauge-min, span.hd-gauge-max (hidden for the needle)
```

**Geometry.**
- The box is D × 0.84 D with D = min(body-w, body-h / 0.84): 147.6 at 2×2.
- Tactile: D × D with D = min(body-w, body-h), viewBox 200.
- Zone bands: stroke 8 units (4 % D) at r 84, spanning r 80–88 under the minor ticks (C2 radius).
- Readout centred at y 130 (77.4 % of 168; Tactile 65 %), numeral `0.11 D`.
- Tactile face: inset 4 % (r < 92). Bezel: ring r 92–100 (`--hd-t-ring-92` mask). Reflection: ellipse at top-left.

```css
.hd-widget[data-hd-type="gauge"] .hd-gauge { position: relative; inline-size: min(var(--hd-body-w), calc(var(--hd-body-h) / 0.84)); aspect-ratio: 200 / 168; }
.hd-widget[data-hd-type="gauge"][data-hd-family="tactile"] .hd-gauge { inline-size: min(var(--hd-body-w), var(--hd-body-h)); aspect-ratio: 1; }
.hd-gauge-svg { position: absolute; inset: 0; inline-size: 100%; block-size: 100%; overflow: visible; }
.hd-gauge-face { position: absolute; inset: 0; border-radius: var(--hd-radius-pill); }
.hd-gauge-bezel, .hd-gauge-glass { position: absolute; border-radius: var(--hd-radius-pill); } .hd-gauge-bezel { inset: 0; } .hd-gauge-glass { inset: 4%; }
.hd-gauge-readout { position: absolute; inset-inline-start: 50%; inset-block-start: 77.4%; translate: -50% -50%; font-size: calc(min(var(--hd-body-w), calc(var(--hd-body-h) / 0.84)) * 0.11); }
.hd-widget[data-hd-family="tactile"] .hd-gauge[data-hd-gauge="needle"] .hd-gauge-readout { inset-block-start: 65%; }
.hd-gauge[data-hd-gauge="needle"] :is(.hd-gauge-min, .hd-gauge-max) { display: none; }
.hd-gauge-numeral { font-size: 15px; text-anchor: middle; dominant-baseline: middle; }
```

**Paint:**
- **G:** no face. Majors `stroke: var(--hd-ink-muted)`, minors `var(--hd-ink-faint)`, `stroke-width: var(--hd-stroke-2)` / `var(--hd-stroke-tick)`, non-scaling. Zones `stroke-width: 8` via `var(--hd-g-zone-u)` (T1 addition `--hd-g-zone-u: 8` in Task 7), `[data-tone="warn"] { stroke: var(--hd-warn) }`, `[data-tone="crit"] { stroke: var(--hd-alert) }`, opacity 0.7. Needle shape `fill: var(--hd-ink)`, thinned with `transform-box: view-box; transform-origin: 100px 100px; transform: scaleX(.65)`. Hub `fill: var(--hd-thumb); stroke: var(--hd-thumb-edge); stroke-width: var(--hd-g-edge-u)`. Numerals `fill: var(--hd-ink-muted)`.
- **T:** `.hd-gauge-face { display: block; }`. `.hd-gauge-bezel { background: var(--hd-t-steel); -webkit-mask: var(--hd-t-ring-92); mask: var(--hd-t-ring-92); }`. `.hd-gauge-glass { background: var(--hd-t-reflection), var(--hd-t-vignette), var(--hd-t-face); }`. Ticks and numerals `--hd-t-faceink` (numerals in Barlow). Needle `fill: var(--hd-t-needle)`. Shadow group shown, `fill: var(--hd-t-needle-shadow)`. Hub `fill: var(--hd-t-faceink)`.
- **M:** majors only (minors hidden). Needle `transform: scaleX(.25)` (about 1 px). Hub `r: 3`, ink. Zones as 2px non-scaling arcs.
- **I:** no face. Ticks HUD 55 %; zone colours per tone. Needle `fill: var(--hd-i-val)`. Glow path shown: `fill: none; stroke: var(--hd-i-val); stroke-width: 3; opacity: .25` via `var(--hd-i-needle-glow-u)` (T4 addition `--hd-i-needle-glow-u: 3` in Task 18). Hub is a HUD ring.

**States:** on (42); min (0); max (100, tone crit from its zone); alert (91, crit); unavailable (needle at min, faint, readout "–"; Instrument NO DATA box); stale (muted, badge).

**Motion** (all inside MG):
- `g.hd-gauge-needle` and `g.hd-gauge-needle-shadow`: transform over `--hd-dur-needle-f` with `--hd-ease-needle`. Minimal uses 420 ms with `--hd-ease-standard`.
- Changes below 0.25 % of the range keep the angle (C2 `needleChanged`).
- Zone and readout tone changes are instant except Instrument (flash).

**Accessibility:** `role=meter` with value attributes (C2); `aria-valuetext` = value plus unit.

**Fixtures:** `gauge/needle`. Lab sizes 2×2 and 3×3. Scenario: `value` (20→80).

### H6 Arc gauge: `gauge` / `arc` (hd-w-sensor.js)

**Primitive call.** `E.prim.arcGauge({label, min, max, unit, decimals:0, zones, quantize: inst ? 40 : 0, crop: 172, glow: glass || inst, gaps: inst ? 40 : 0, lead: inst, zoneOverlay: true, readout: inst ? 'seg7' : 'numeral'})`.

**Tactile substitution.** When `api.family === 'tactile'`, the module calls `needleGauge` with H6's options instead: range 0–100 %, zones warn 80 / crit 92, numerals 0…100, a D × D round instrument with face, needle shadow and readout "64 %". It is painted exactly as Tactile H5.

**DOM** (C2 plus additions marked *): `div.hd-gauge[data-hd-gauge=arc][role=meter]` holds:
- svg (`0 0 200 172`): `path.hd-gauge-track[pathLength=1000]`, `path.hd-gauge-zone[data-tone]` (r 76), `path.hd-gauge-glow*[pathLength=1000]`, `path.hd-gauge-arc[pathLength=1000]` (r 88), `path.hd-gauge-arc-zone*[pathLength=1000]`, `path.hd-gauge-lead*`, `path.hd-gauge-gaps*[pathLength=1000]`, `g.hd-gauge-cap > circle.hd-gauge-capdot[cx=100][cy=12][r=7]`
- `span.hd-gauge-readout`, `span.hd-gauge-min`, `span.hd-gauge-max`.

**Geometry.**
- Box D × 0.86 D with D = min(body-w, body-h / 0.86): 144 at 2×2.
- Glass: stroke 15, cap dot `r: var(--hd-g-capdot-r)` (10).
- Minimal: arc stroke 8 units (4 %), track 1px non-scaling.
- Instrument: stroke 16 plus gaps and lead.
- Zone bands (r 76) as 2px non-scaling arcs at 0.7 opacity in every family except Tactile.
- Readout numeral `min(28cqmin, 0.30 D)`, centred at y 100 (58.1 %).
- Min and max legends at the arc ends: bottom 0, start and end at 19.5 %, 11px muted.

```css
.hd-widget[data-hd-type="gauge"] .hd-gauge[data-hd-gauge="arc"] { inline-size: min(var(--hd-body-w), calc(var(--hd-body-h) / 0.86)); aspect-ratio: 200 / 172; }
.hd-gauge[data-hd-gauge="arc"] .hd-gauge-readout { inset-block-start: 58.1%; font-size: min(28cqmin, calc(min(var(--hd-body-w), calc(var(--hd-body-h) / 0.86)) * 0.3)); }
.hd-gauge[data-hd-gauge="arc"] :is(.hd-gauge-min, .hd-gauge-max) { position: absolute; inset-block-end: 0; font-size: 11px; color: var(--hd-ink-muted); }
.hd-gauge[data-hd-gauge="arc"] .hd-gauge-min { inset-inline-start: 19.5%; translate: -50% 0; } .hd-gauge[data-hd-gauge="arc"] .hd-gauge-max { inset-inline-start: 80.5%; translate: -50% 0; }
.hd-gauge-arc-zone { opacity: 0; } .hd-gauge[data-hd-tone="warn"] .hd-gauge-arc-zone { stroke: var(--hd-warn); opacity: 1; } .hd-gauge[data-hd-tone="crit"] .hd-gauge-arc-zone { stroke: var(--hd-alert); opacity: 1; }
```

**Paint:**
- Value colour `--hd-value` (the accent); the zone overlay colours the arc in a zone.
- **G:** `.hd-gauge-track { stroke: var(--hd-track); stroke-width: calc(var(--hd-stroke-arc) * 200); stroke-linecap: round }`. Arc, arc-zone and glow with round caps; glow at `var(--hd-arc-underglow)`. Capdot `fill: var(--hd-thumb); stroke: var(--hd-thumb-edge); stroke-width: var(--hd-g-edge-u); r: var(--hd-g-capdot-r)`.
- **M:** butt caps. Arc `stroke-width: calc(var(--hd-stroke-arc) * 200)`. Track `stroke: var(--hd-hairline); stroke-width: var(--hd-stroke-hair); vector-effect: non-scaling-stroke`. Cap dot hidden.
- **I:** as H3 Instrument (arc 0.7, lead, glow, gaps), with the arc-zone overlay set to opacity 0 because `--hd-i-val` already switches at crit. Warn tints the lead with `--hd-warn`.

**States:** on (64, accent); min 0; max 100 (crit); alert 95; unavailable; stale. The `value` scenario 20→85 ends in the warn zone.

**Motion** (all inside MG):
- `.hd-gauge-arc`, `.hd-gauge-arc-zone`, `.hd-gauge-glow` stroke-dasharray and `g.hd-gauge-cap` transform: `--hd-dur-remote` / `--hd-ease-remote`, in step.
- Arc-zone opacity: `--hd-dur-zone`.
- Instrument: snap plus the flash.

**Accessibility:** as H5. **Fixtures:** `gauge/arc`. Lab sizes 2×2 and 3×3. Scenario: `value`.

### H7 Big temperature readout: `temperature` / `numeral` (hd-w-sensor.js)

**Primitive calls:**
- `E.prim.numeral({chars:4, decimals:1, unit:'C', degree:true, trendIcon:true, lang})` (C2 numeral plus addition `trendIcon`)
- Instrument: `E.prim.segReadout({digits:4, decimals:1, unit: DEG_C})`.

**DOM:**

```
.hd-body > div.hd-tread > div.hd-vfd (module element; painted by Tactile only)
  > span.hd-numeral[aria-hidden=true] > span.hd-num-int, span.hd-num-frac, span.hd-num-deg "°", span.hd-num-unit "C", span.hd-num-trend[data-trend] > svg.hd-ico
    | span.hd-seg7[role=img][aria-label] (Instrument; no sr text needed)
  span.hd-sr-only (e.g. "21.4 °C, falling")
```

The state line (`setStatus`) carries `t('homedash.temperature.range', {min, max})`.

**Geometry.** H7 uses its own `--hd-gap-in: 2px`. The body row is the numeral row.

```css
.hd-widget[data-hd-type="temperature"] > .hd-frame, .hd-widget[data-hd-type="sensor"][data-hd-variant="numeral"] > .hd-frame { --hd-gap-in: 2px; }
.hd-tread { display: flex; align-items: flex-end; inline-size: 100%; }
.hd-tread .hd-numeral { font-size: min(calc(var(--hd-body-h) / var(--hd-num-cap)), calc(var(--hd-body-w) / 2.1), var(--hd-num-hero-max)); line-height: var(--hd-num-cap);
  font-weight: var(--hd-num-weight); letter-spacing: var(--hd-num-tracking); display: inline-flex; align-items: flex-start; }
.hd-tread .hd-num-int { min-inline-size: calc(var(--hd-num-chars) * 0.5ch); text-align: end; }
.hd-tread :is(.hd-num-frac, .hd-num-deg) { line-height: 1; } .hd-tread .hd-num-frac { font-size: 0.5em; } .hd-tread .hd-num-deg { font-size: 0.45em; }
.hd-tread .hd-num-unit { font-size: var(--hd-unit-scale); color: var(--hd-ink-muted); line-height: 1; }
.hd-tread .hd-num-trend { align-self: flex-end; margin-inline-start: 0.08em; } .hd-tread .hd-num-trend .hd-ico { inline-size: 0.3em; block-size: 0.3em; }
.hd-tread .hd-seg7 { block-size: calc(var(--hd-body-h) * 0.92); }
```

Font size: 2×1 gives body 170 × 36, so 48.6 px; 1×1 gives 78 × 46, so 37 px (width-limited).

**Paint:**
- **G:** Inter 300, `--hd-ink`. Trend icon colour `color-mix(in srgb, var(--hd-heat) 70%, transparent)` for `[data-trend="up"]`, `--hd-cool` at 70 % for down, muted for flat.
- **T:** `.hd-vfd { background: var(--hd-t-window); box-shadow: var(--hd-window-shadow); border-radius: var(--hd-radius-control); padding: 3px 8px; }`. Numeral `font-family: var(--hd-font-vfd); font-weight: 500; color: var(--hd-t-vfd); text-shadow: var(--hd-vfd-glow); font-size × 0.86` (a `.hd-vfd .hd-numeral` rule with the same `min()` × 0.86). The title is an engraved plate: Barlow with `--hd-t-emboss`.
- **M:** Geist 300 with cap factor 0.71.
- **I:** ghosts `--hd-i-ghost`, lit `--hd-i-val`, the decimal point is a segment, unit "°C" in Share Tech Mono at 0.42. `--hd-i-scan` overlay at opacity .35 on `.hd-seg7::after`.

**States:**
- on 21.4 (falling); min −12.5 (the minus sign sits in the integer box); max 38.9; alert 31.6 (tone crit).
- unavailable "–" (the box width is kept); stale muted; error "Unknown entity".

**Motion** (all inside MG): Glass and Instrument apply `hd-dip-a`/`-b` (opacity 0.6) over `--hd-dur-dip` on `.hd-tread[data-hd-tick]`. Tactile and Minimal have none. No counting animation anywhere.

**Accessibility:** the visual numeral is `aria-hidden`; `.hd-sr-only` has value, unit and trend word (`homedash.trend.*`).

**Fixtures:** `temperature/numeral`. Lab sizes 1×1 and 2×1. Scenario: `value` (21.4→21.9).

### H8 Sensor tile with sparkline: `sensor` / `sparkline` (hd-w-sensor.js)

**Primitive calls:**
- `E.prim.sparkline({label, w, h, area:true, fade: glass, grid: tact || inst ? [4,3] : null, halo: true})`, with additions `fade`, `grid` and `halo`.
- `E.prim.numeral({chars:5, decimals:0, unit, lang})`, or `segReadout` for Instrument.
- The head icon is the frame's `span.hd-icon`, from `widget.icon` (`plug`), painted as an icon badge.

**DOM:**

```
.hd-head > span.hd-icon > svg.hd-ico, span.hd-title …
.hd-body > numeral | seg7;  svg.hd-spark-fill*[aria-hidden] > path.hd-spark-area (fade only);
           svg.hd-spark[role=img][aria-label][viewBox="0 0 w h"][width][height] > path.hd-spark-grid*, path.hd-spark-line, circle.hd-spark-halo*[r=6], circle.hd-spark-last[r=3]
```

**Geometry.**
- `.hd-body` is `position: static` here. Both spark SVGs are `position: absolute; inset-inline-start: 0; inset-block-end: 0` against the frame, which clips them with its radius.
- The widget passes `w = size.px.width − 2` and `h = round(size.px.height × 0.38)`. Tactile: `w − 16`, `h − 8`, inset 8 px.
- 2×2: grid `"head" var(--hd-head-h) "body" auto "state" var(--hd-state-h) "." 1fr`.
- 2×1 (aspect ≥ 1.6): `"head head" var(--hd-head-h) "body state" auto ". ." 1fr / minmax(0,1fr) auto`.
- Numeral `22cqmin`, unit × 0.42. Y range = data min/max padded 12 % (C2 geometry). End dot r 3, halo r 6. Grid at integer +0.5 px.

```css
.hd-widget[data-hd-type="sensor"][data-hd-variant="sparkline"] .hd-body { position: static; place-items: start; }
.hd-widget[data-hd-type="sensor"][data-hd-variant="sparkline"] > .hd-frame { grid-template: "head" var(--hd-head-h) "body" auto "state" var(--hd-state-h) "." 1fr / minmax(0, 1fr); }
@container hd-widget (aspect-ratio >= 1.6) { .hd-widget[data-hd-type="sensor"][data-hd-variant="sparkline"] > .hd-frame { grid-template: "head head" var(--hd-head-h) "body state" auto ". ." 1fr / minmax(0, 1fr) auto; } }
.hd-widget[data-hd-type="sensor"] :is(.hd-spark, .hd-spark-fill) { position: absolute; inset-inline-start: 0; inset-block-end: 0; }
.hd-widget[data-hd-type="sensor"][data-hd-family="tactile"] :is(.hd-spark, .hd-spark-fill) { inset-inline-start: 8px; inset-block-end: 8px; }
.hd-widget[data-hd-type="sensor"] .hd-body > .hd-numeral { font-size: 22cqmin; }
```

**Paint:**
- **G:** `.hd-spark-fill { -webkit-mask: var(--hd-fade-down); mask: var(--hd-fade-down); }`. `.hd-spark-area { fill: var(--hd-accent); fill-opacity: .22; }`. Line `stroke: var(--hd-accent); stroke-width: var(--hd-g-spark-w); stroke-linejoin: round; stroke-linecap: round` (T1 addition `--hd-g-spark-w: 1.75px` in Task 7). Dot `fill: var(--hd-accent)`. Halo shown at opacity .2. Icon badge: `.hd-icon { background: var(--hd-accent-soft); border-radius: var(--hd-radius-pill); color: var(--hd-accent); }`.
- **T:** `.hd-spark { background: var(--hd-t-window); box-shadow: var(--hd-window-shadow); border-radius: var(--hd-radius-control); }`. Trace `stroke: var(--hd-t-amber)`. Grid `stroke: var(--hd-t-grid-ink)`, 1px non-scaling. Area hidden. Dot amber.
- **M:** line `stroke: var(--hd-ink); stroke-width: var(--hd-m-spark-w)`. Area, halo and grid hidden. Dot ink.
- **I:** line `stroke: var(--hd-i-val)`. Grid `color-mix(in srgb, var(--hd-i-hud) 8%, transparent)`. Halo `fill: var(--hd-i-val)` at .35 (the glow). Area hidden.

**States:** on 412 W; min 0; max 2860; alert 2480 (tone crit); unavailable (line hidden, "–"); stale (muted); error.

**Motion** (all inside MG): numeral dip (Glass and Instrument) on a change. The spark redraw is instant.

**Accessibility:** the spark has `role=img` with `aria-label` = `t('homedash.sensor.summary', {min, max, now})`; the numeral is as in H7.

**Fixtures:** `sensor/*`, so `sensor/numeral` shares the data. Lab sizes 2×1 and 2×2. Scenario: `value` (412→980).

### Widget module contracts (Tasks 2–3)

Every module is an IIFE:

```js
(function () { 'use strict'; const E = window.HomeDashEngine; if (!E || !E.widgets || !E.prim) throw new Error('hd-w-…: engine missing');
  E.widgets.register(type, { bindings: ['primary'], variants: [/* catalog order */], create(api) { /* … */ return { update(view) {}, resize(size) {}, destroy() {} }; } }); })();
```

Behaviour rules:
1. `create(api)` builds DOM only with `api.prim.*`, `api.icons` and `E.el`/`E.svg`. It never reads layout and never uses innerHTML. Family-dependent options and DOM come from `api.family` (C3 resolution). The host recreates the widget when the family changes.
2. A non-hero variant returns `{update(){ api.setStatus(null); }}` with an empty body. Exception: `sensor/numeral` reuses the H7 builder without degree and trend.
3. `update(view)` reads `const e = view.entities.primary`. It writes only:
   - primitive `set()` keys;
   - `api.setTone(...)`;
   - `api.setStatus(...)`;
   - `data-*` attributes and textContent on module elements (`data-hd-action`, `data-hd-tick`, `data-hd-nodata` once at create from `t('homedash.state.no_data')`).
4. Tone: `api.setTone(e && e.tone ? e.tone : zoneTone)`. `zoneTone` is `warn` or `crit` from `widget.options.zones` for gauges; null otherwise. Values follow M0: `ok|warn|crit`.
5. Status: when `e` is null, unavailable or pending, call `setStatus(null)` (the host words win). Otherwise:
   - read-only → `${word} · ${t('homedash.state.readonly')}`;
   - `!api.can('primary', cap)` → `t('homedash.state.blocked')`;
   - stale → `t('homedash.state.updated_ago', {age: api.fmt.relTime(e.attributes.updated_at)})`;
   - otherwise the hero text below.
6. Switch: `on = api.fmt.isOn(e) === true`; `set({on, status: e.status, pending: e.pending, readonly: e.readonly, disabled: !api.can('primary', 'toggle')})`; status text = on/off word.
7. Light: pct = round(brightness / 2.55), 0 when off; `set({value: on ? pct : 0, word: on ? null : t('homedash.state.off'), valueText})`; status text = `${on word} · ${pct} %`.
8. Climate:
   - action from `hvac_action` (heating, cooling, idle; mode off → off) written to the body;
   - `set({value: temperature, current: current_temperature, action: t(action key), sub: current_temperature, valueText})`;
   - segmented `set({value: mode})`; status text = mode word.
9. Gauges: `set({value: e.value})`. The primitives handle zone tone (C2) and the needle dead band.
10. Temperature and sensor:
    - On create, call `api.history('primary', '24h').then(s => …)` with the shape `{points, source}`.
    - min/max from the non-null `points[i][1]`.
    - Trend over the last 30 min: above +0.3 /h up, below −0.3 /h down, else flat.
    - Sparkline: `set({points: s.points.map(p => p[1]), w, h, summary})` on history arrival and in `resize(size)`. It is never stretched.
    - Flip `data-hd-tick` when the displayed value changes.

## Tasks

Roster: maker-agent implements. test-agent runs the named tests. reader-agent does one combined review per batch (Critical/Important only). docs-agent writes Task 24. debug-agent handles failures. The lead alone judges gates.

**Standard commit step** (every task):
1. Run `node C:\Users\Andi\.gitnexus\stores\AuraGo-08496d38b623\checkouts\AuraGo-08496d38b623\run.cjs impact "<symbol>" --direction upstream --repo C:\Users\Andi\Documents\repo\AuraGo` for every existing symbol edited, or MCP `impact` with that repo path. Warn on HIGH or CRITICAL; treat UNKNOWN as unresolved.
2. If both fail, confirm callers with Grep and write "GitNexus unavailable; callers confirmed with grep: …" in the commit body.
3. MCP `detect_changes({scope:"all", worktree:"<W>"})`, or the same note.
4. `npm run build:ui`, then `npm run check:ui`, then `npm run test:homedash`.
5. `git add <own paths>` (`-f` for docs and new AGENTS.md), then `git commit`. The worktree has its own index.

**Lab run** (used by every task). PowerShell with the sandbox off, from `<W>\ui`, with the prebuilt binary:

```powershell
$env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:HOMEDASH_LAB_SHEETS='<ids>'
& D:\tmp\go-build\hd-m1-ui.test.exe -test.run '^TestDesktopHomeDashControlLabBrowser$' -test.count 1 -test.timeout 30m -test.v
```

- Add `$env:HOMEDASH_LAB_REPORT_ONLY='1'` while iterating; gate runs go without it.
- Build the binary from W with `$env:GOTMPDIR='D:\tmp\go-build'; go test -c -o D:\tmp\go-build\hd-m1-ui.test.exe ./ui` (prefix `GOTOOLCHAIN=go1.27.4` if `vet` is missing). Rebuild only after Go or testdata changes; the lab reads CSS and JS from disk (`ui.Content` is `os.DirFS`).
- Output: `<W>\reports\homedash-lab\<id>-NN.png` and `probes.json`.

### Task 0: Workspace, fixtures, sheet definitions, colour-vision test, baseline

**Files:** `ui/testdata/homedash-lab/hero-fixtures.js` (new), `ui/testdata/homedash-lab-sheets.json` (M1 entries), `ui/desktop_homedash_lab_cvd_test.go` (new).
**Read first:** M0 plan C2–C6 and Task 16; this plan's Conventions and Review protocol; memory notes on Chrome launch, leftover rod Chromes and worktree node_modules.

- [ ] **Base.** Confirm M0 has landed (`git -C C:\Users\Andi\Documents\repo\AuraGo log --oneline -30 main` shows M0 Tasks 8–16). Create the worktree:
  - `git -C C:\Users\Andi\Documents\repo\AuraGo worktree add -b claude/homedash-m1-style-lab .worktrees\homedash-m1 main`
  - Copy node_modules for real (no junction): `robocopy C:\Users\Andi\Documents\repo\AuraGo\node_modules <W>\node_modules /E /MT:16 /NFL /NDL /NJH /NP`.
- [ ] **Fixtures.** Write `hero-fixtures.js` verbatim (below).
- [ ] **Sheets.** Merge the M1 fragment (below) into `ui/testdata/homedash-lab-sheets.json`:
  - add the `states` sets to `lists.states`;
  - append the `sheets` entries after M0's entries.
  - Do not edit M0's entries; a duplicate id is a load error.
- [ ] **CVD test.** Write `ui/desktop_homedash_lab_cvd_test.go` verbatim (below).
- [ ] **Resolver check.** `go test ./ui -run '^TestHomeDashLab(SheetResolve|Pack)$' -count=1`. Expected: PASS (it validates the file, including the new entries).
- [ ] **Baseline.** Check for leftover rod Chromes and report them. Build the binary. Run the lab with `HOMEDASH_LAB_SHEETS=hero-H1-placeholder` and report-only. Expected: the PNG exists, and probes.json has `extra.m1-tabular` and `extra.m1-svgcount`. Hero widgets do not exist yet, so the lab reports `bad param types`. Record that error string as the expected baseline, then run `prim-placeholder` to time a sheet.
- [ ] Commit `test(homedash): M1 hero fixtures, sheet definitions and colour-vision sheets`.

**Done when:** the resolver test passes and the hand-back lists the baseline timings and the exact error for the not-yet-registered types.

`ui/testdata/homedash-lab/hero-fixtures.js`:

```js
/* HomeDash M1 style lab: hero fixtures and scenarios (hdLab.fixtures.define), look application, focus marking for widget
 * cells and the M1 extra probes m1-tabular and m1-svgcount (hdLab.report). Test-only; loaded through a sheet's "scripts". */
(function () {
  'use strict';
  const lab = window.hdLab;
  if (!lab || !lab.fixtures || typeof lab.fixtures.define !== 'function' || typeof lab.report !== 'function') {
    throw new Error('hero-fixtures.js needs window.hdLab with fixtures.define and report (M0 C4)');
  }
  const NOW = Date.UTC(2026, 10, 23, 18, 0, 0), MIN = 60000;
  const iso = (ms) => new Date(ms).toISOString();
  function merge(base, over) { const o = over || {}; return Object.assign({}, base, o, { attributes: Object.assign({}, base.attributes, o.attributes) }); }
  function entity(id, kind, name, extra) {
    return merge({ id, kind, name, area: 'Living room', unit: '', device_class: '', value: null, attributes: { updated_at: iso(NOW - 2 * MIN) },
      available: true, status: 'ok', readonly: false, capabilities: [], pending: false, stale: false, error: null, tone: null }, extra);
  }
  const STALE = { stale: true, attributes: { updated_at: iso(NOW - 14 * MIN) } };
  // Every lab state resolves for every hero, so sheets that mix heroes never fall back to the built-in labView().
  function states(on, off, extra) {
    return Object.assign({ off, on, focus: on, min: off, max: on, pending: merge(on, { pending: true }), disabled: merge(on, { capabilities: [] }),
      unavailable: merge(off, { available: false, status: 'unavailable', value: null }), stale: merge(on, STALE), readonly: merge(on, { readonly: true }),
      error: merge(off, { status: 'error', error: { code: 'timeout' } }), alert: merge(on, { tone: 'crit' }) }, extra || {});
  }
  function displayStates(normal, extra) {
    return states(normal, normal, Object.assign({ pending: normal, disabled: normal, error: merge(normal, { status: 'unknown', value: null }) }, extra));
  }
  function series(base, amp, ripple, gapAt, last3) {
    const points = [];
    for (let i = 0; i < 96; i++) {
      const day = Math.sin((i / 96) * Math.PI * 2 - 1.9), r = Math.sin(i * 0.9) * 0.35 + Math.sin(i * 0.37 + 1) * 0.65;
      const gap = gapAt >= 0 && i >= gapAt && i < gapAt + 3;
      points.push([NOW - (95 - i) * 15 * MIN, gap ? null : Math.round((base + amp * day + ripple * r) * 10) / 10]);
    }
    last3.forEach((v, k) => { points[93 + k][1] = v; });
    return { points, source: 'ring' };
  }

  const lamp = entity('ha:switch.desk_lamp', 'switch', 'Desk lamp', { value: 'on', capabilities: ['toggle'] }), lampOff = merge(lamp, { value: 'off' });
  lab.fixtures.define('switch', 'pill', { widget: { title: 'Desk lamp' }, states: states(lamp, lampOff),
    scenarios: { toggle: { from: lampOff, to: lamp }, pending: { from: lamp, to: merge(lamp, { pending: true }) } } });
  const heater = entity('ha:switch.fan_heater', 'switch', 'Fan heater', { value: 'on', capabilities: ['toggle'] }), heaterOff = merge(heater, { value: 'off' });
  lab.fixtures.define('switch', 'rocker', { widget: { title: 'Fan heater' }, states: states(heater, heaterOff),
    scenarios: { toggle: { from: heaterOff, to: heater }, pending: { from: heater, to: merge(heater, { pending: true }) } } });

  const light = entity('ha:light.reading', 'light', 'Reading light', { value: 'on', attributes: { brightness: 163 }, capabilities: ['toggle', 'brightness'] });
  const lightOff = merge(light, { value: 'off', attributes: { brightness: null } });
  lab.fixtures.define('light', 'dial', { widget: { title: 'Reading light' },
    states: states(light, lightOff, { min: merge(light, { attributes: { brightness: 3 } }), max: merge(light, { attributes: { brightness: 255 } }) }),
    scenarios: { remote: { from: merge(light, { attributes: { brightness: 51 } }), to: merge(light, { attributes: { brightness: 204 } }) },
      pending: { from: light, to: merge(light, { pending: true }) } } });

  const clim = entity('ha:climate.living', 'climate', 'Living room', { value: 'heat', capabilities: ['set_temperature', 'set_hvac_mode'],
    attributes: { current_temperature: 19.5, temperature: 21.5, min_temp: 5, max_temp: 30, target_temp_step: 0.5, hvac_action: 'heating', hvac_modes: ['off', 'heat', 'cool', 'auto'] } });
  lab.fixtures.define('climate', 'dial', { widget: { title: 'Living room' },
    states: states(clim, merge(clim, { value: 'off', attributes: { hvac_action: 'off' } }), {
      min: merge(clim, { attributes: { temperature: 5, hvac_action: 'idle' } }), max: merge(clim, { attributes: { temperature: 30 } }),
      alert: merge(clim, { attributes: { current_temperature: 12 }, tone: 'crit' }),
      // readonly doubles as the cooling sample (cool mode, target 22, current 24.5)
      readonly: merge(clim, { readonly: true, value: 'cool', attributes: { temperature: 22, current_temperature: 24.5, hvac_action: 'cooling' } }) }),
    scenarios: { target: { from: merge(clim, { attributes: { temperature: 19 } }), to: merge(clim, { attributes: { temperature: 23 } }) },
      pending: { from: clim, to: merge(clim, { pending: true }) } } });

  const cpu = entity('system:cpu_load', 'sensor', 'CPU load', { value: 42, unit: '%' });
  lab.fixtures.define('gauge', 'needle', { widget: { title: 'CPU load', icon: 'cpu', options: { min: 0, max: 100, zones: [{ from: 70, to: 85, tone: 'warn' }, { from: 85, to: 100, tone: 'crit' }] } },
    states: displayStates(cpu, { min: merge(cpu, { value: 0 }), max: merge(cpu, { value: 100 }), alert: merge(cpu, { value: 91 }) }),
    scenarios: { value: { from: merge(cpu, { value: 20 }), to: merge(cpu, { value: 80 }) } } });
  const disk = entity('system:disk_data', 'sensor', 'Disk /data', { value: 64, unit: '%' });
  lab.fixtures.define('gauge', 'arc', { widget: { title: 'Disk /data', icon: 'disk', options: { min: 0, max: 100, zones: [{ from: 80, to: 92, tone: 'warn' }, { from: 92, to: 100, tone: 'crit' }] } },
    states: displayStates(disk, { min: merge(disk, { value: 0 }), max: merge(disk, { value: 100 }), alert: merge(disk, { value: 95 }) }),
    scenarios: { value: { from: merge(disk, { value: 20 }), to: merge(disk, { value: 85 }) } } });

  const temp = entity('ha:sensor.living_temperature', 'sensor', 'Living room', { value: 21.4, unit: String.fromCodePoint(0xb0) + 'C', device_class: 'temperature' });
  lab.fixtures.define('temperature', 'numeral', { widget: { title: 'Living room', icon: 'thermometer' },
    states: displayStates(temp, { min: merge(temp, { value: -12.5 }), max: merge(temp, { value: 38.9 }), alert: merge(temp, { value: 31.6, tone: 'crit' }) }),
    history: { default: series(21.2, 1.4, 0.3, -1, [21.9, 21.65, 21.4]), min: series(-10.5, 2.5, 0.4, -1, [-12.0, -12.3, -12.5]), max: series(35.5, 2.4, 0.4, -1, [38.1, 38.6, 38.9]) },
    scenarios: { value: { from: temp, to: merge(temp, { value: 21.9 }) } } });
  const power = entity('ha:sensor.kitchen_power', 'sensor', 'Kitchen power', { value: 412, unit: 'W', device_class: 'power' });
  lab.fixtures.define('sensor', '*', { widget: { title: 'Kitchen power', icon: 'plug' },
    states: displayStates(power, { min: merge(power, { value: 0 }), max: merge(power, { value: 2860 }), alert: merge(power, { value: 2480, tone: 'crit' }) }),
    history: { default: series(380, 220, 120, 40, [520, 455, 412]), min: series(60, 50, 30, 40, [12, 4, 0]), max: series(1400, 700, 300, 40, [2410, 2700, 2860]),
      alert: series(1300, 650, 280, 40, [2290, 2400, 2480]) },
    scenarios: { value: { from: power, to: merge(power, { value: 980 }) } } });

  // Look and focus: applyLook once per panel; widget cells in the focus state get the M0 lab focus hook on their role element.
  let looked = false;
  function decorate() {
    const root = document.querySelector('.hd-lab-panel');
    if (!root) return;
    const E = window.HomeDashEngine;
    if (!looked && E && E.widgets && typeof E.widgets.applyLook === 'function') { E.widgets.applyLook(root); looked = true; }
    for (const cell of root.querySelectorAll('.hd-lab-cell[data-hd-lab-state="focus"]')) {
      const target = cell.querySelector('.hd-widget [role="switch"], .hd-widget [role="slider"]');
      if (target && !target.hasAttribute('data-hd-focus')) target.setAttribute('data-hd-focus', 'ring');
    }
  }
  new MutationObserver(decorate).observe(document.documentElement, { childList: true, subtree: true });

  // Extra probes, computed when the Go test asks for probes (after render), then stored with hdLab.report.
  function widthOf(host, stack, features, text) {
    const s = document.createElement('span'); s.textContent = text;
    s.style.setProperty('font-family', stack); s.style.setProperty('font-size', '40px'); s.style.setProperty('font-feature-settings', features || 'normal'); s.style.setProperty('white-space', 'pre');
    host.appendChild(s); const w = s.getBoundingClientRect().width; s.remove(); return w;
  }
  function tabular(root) {
    const out = [];
    for (const family of ['glass', 'tactile', 'minimal', 'instrument']) {
      const host = document.createElement('div'); host.className = 'hd-widget'; host.setAttribute('data-hd-family', family);
      host.style.setProperty('position', 'absolute'); host.style.setProperty('visibility', 'hidden'); host.style.setProperty('inset-inline-start', '-10000px');
      root.appendChild(host);
      const cs = getComputedStyle(host), features = cs.getPropertyValue('--hd-num-features').trim();
      for (const name of ['--hd-font-num', '--hd-font-label', '--hd-font-vfd']) {
        const stack = cs.getPropertyValue(name).trim(); if (!stack) continue;
        const widths = ['1111', '8888', '0000', '4747'].map((t) => widthOf(host, stack, features, t));
        const spread = Math.max.apply(null, widths) - Math.min.apply(null, widths);
        out.push({ family, token: name, stack, loaded: document.fonts.check('40px ' + stack.split(',')[0].trim()), spread, pass: spread <= 0.5 });
      }
      host.remove();
    }
    return out;
  }
  function svgCount(root) {
    return Array.from(root.querySelectorAll('.hd-widget')).map((w) => {
      const n = w.querySelectorAll('svg *').length;
      return { type: w.getAttribute('data-hd-type'), variant: w.getAttribute('data-hd-variant'), family: w.getAttribute('data-hd-family'), nodes: n, pass: n <= 150 };
    });
  }
  function runM1() {
    const root = document.querySelector('.hd-lab-panel'); if (!root) return;
    decorate();
    const tab = tabular(root), svg = svgCount(root);
    lab.report('m1-tabular', { ok: tab.every((r) => r.pass), message: tab.filter((r) => !r.pass).map((r) => r.family + ' ' + r.token + ' spread ' + r.spread.toFixed(2) + 'px').join('; '), results: tab });
    lab.report('m1-svgcount', { ok: svg.every((r) => r.pass), message: svg.filter((r) => !r.pass).map((r) => r.type + '/' + r.variant + ' ' + r.family + ': ' + r.nodes + ' nodes').join('; '), results: svg });
  }
  let inner = typeof lab.probes === 'function' ? lab.probes : null;
  const wrapped = async function () { runM1(); return inner ? inner.apply(this, arguments) : {}; };
  Object.defineProperty(lab, 'probes', { configurable: true, enumerable: true, get() { return wrapped; }, set(fn) { inner = fn; } });
})();
```

M1 fragment for `ui/testdata/homedash-lab-sheets.json`. The `S` value of every entry is `["/testdata/homedash-lab/hero-fixtures.js"]`, written out in the file.

```json
{
  "lists": { "states": {
    "switchCore": ["off", "on", "pending", "unavailable", "alert", "focus"], "switchExtra": ["disabled", "stale", "readonly", "error"],
    "display": ["on", "min", "max", "alert", "unavailable", "stale"], "displayExtra": ["error"], "signoffDisplay": ["on", "max", "alert"] } },
  "sheets": [
    {"id": "hero-H1-{family}", "kind": "hero", "types": ["switch"], "variants": ["pill"], "palettes": ["dark", "light"], "states": "switchCore+switchExtra", "sizes": ["1x1", "2x1"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H2-{family}", "kind": "hero", "types": ["switch"], "variants": ["rocker"], "palettes": ["dark", "light"], "states": "switchCore+switchExtra", "sizes": ["1x2", "2x2"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H3-{family}", "kind": "hero", "types": ["light"], "variants": ["dial"], "palettes": ["dark", "light"], "states": "core+extra", "sizes": ["2x2", "3x3"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H4-{family}", "kind": "hero", "types": ["climate"], "variants": ["dial"], "palettes": ["dark", "light"], "states": "core+extra", "sizes": ["2x2", "3x3"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H5-{family}", "kind": "hero", "types": ["gauge"], "variants": ["needle"], "palettes": ["dark", "light"], "states": "display+displayExtra", "sizes": ["2x2", "3x3"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H6-{family}", "kind": "hero", "types": ["gauge"], "variants": ["arc"], "palettes": ["dark", "light"], "states": "display+displayExtra", "sizes": ["2x2", "3x3"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H7-{family}", "kind": "hero", "types": ["temperature"], "variants": ["numeral"], "palettes": ["dark", "light"], "states": "display+displayExtra", "sizes": ["1x1", "2x1"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H8-{family}", "kind": "hero", "types": ["sensor"], "variants": ["sparkline"], "palettes": ["dark", "light"], "states": "display+displayExtra", "sizes": ["2x1", "2x2"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H1-{family}~dpr2", "kind": "hero", "types": ["switch"], "variants": ["pill"], "palettes": ["dark", "light"], "states": "switchCore", "sizes": ["2x1"], "dpr": 2, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H2-{family}~dpr2", "kind": "hero", "types": ["switch"], "variants": ["rocker"], "palettes": ["dark", "light"], "states": "switchCore", "sizes": ["2x2"], "dpr": 2, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H3-{family}~dpr2", "kind": "hero", "types": ["light"], "variants": ["dial"], "palettes": ["dark", "light"], "states": "core", "sizes": ["2x2"], "dpr": 2, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H4-{family}~dpr2", "kind": "hero", "types": ["climate"], "variants": ["dial"], "palettes": ["dark", "light"], "states": "core", "sizes": ["3x3"], "dpr": 2, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H5-{family}~dpr2", "kind": "hero", "types": ["gauge"], "variants": ["needle"], "palettes": ["dark", "light"], "states": "display", "sizes": ["2x2"], "dpr": 2, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H6-{family}~dpr2", "kind": "hero", "types": ["gauge"], "variants": ["arc"], "palettes": ["dark", "light"], "states": "display", "sizes": ["2x2"], "dpr": 2, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H7-{family}~dpr2", "kind": "hero", "types": ["temperature"], "variants": ["numeral"], "palettes": ["dark", "light"], "states": "display", "sizes": ["2x1"], "dpr": 2, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-H8-{family}~dpr2", "kind": "hero", "types": ["sensor"], "variants": ["sparkline"], "palettes": ["dark", "light"], "states": "display", "sizes": ["2x2"], "dpr": 2, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-tactile~dpr15", "kind": "hero", "families": ["tactile"], "types": ["switch", "light", "climate", "gauge", "temperature", "sensor"], "variants": ["pill", "rocker", "dial", "needle", "arc", "numeral", "sparkline"], "palettes": ["dark", "light"], "states": ["on"], "sizes": ["2x2"], "dpr": 1.5, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-glass~gradient", "kind": "hero", "families": ["glass"], "types": ["switch", "light", "climate", "gauge", "temperature", "sensor"], "variants": ["pill", "rocker", "dial", "needle", "arc", "numeral", "sparkline"], "palettes": ["dark", "light"], "backdrop": "gradient", "states": "trio", "sizes": ["2x2"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-glass~gradient-dpr2", "kind": "hero", "families": ["glass"], "types": ["switch", "light", "gauge"], "variants": ["pill", "dial", "arc"], "palettes": ["dark", "light"], "backdrop": "gradient", "states": ["on"], "sizes": ["2x2"], "dpr": 2, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-{family}~custom", "kind": "hero", "types": ["switch", "light", "climate", "gauge", "temperature", "sensor"], "variants": ["pill", "rocker", "dial", "needle", "arc", "numeral", "sparkline"], "palettes": ["custom"], "custom": "custom3", "states": "trio", "sizes": ["2x2"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-themes-{family}", "kind": "themes", "families": ["glass", "minimal"], "types": ["switch", "light", "climate", "gauge", "temperature", "sensor"], "variants": ["pill", "rocker", "dial", "needle", "arc", "numeral", "sparkline"], "palettes": ["theme"], "themes": "themes16", "states": ["on", "alert", "focus"], "sizes": ["2x2"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-risk-{family}", "kind": "risk", "families": ["tactile", "instrument"], "types": ["switch", "light", "climate", "gauge", "temperature", "sensor"], "variants": ["pill", "rocker", "dial", "needle", "arc", "numeral", "sparkline"], "palettes": ["theme"], "themes": "risk8", "states": ["off", "on", "alert", "focus"], "sizes": ["2x2"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "hero-minimal-theme~instrument", "kind": "risk", "families": ["instrument"], "types": ["light", "gauge", "temperature"], "variants": ["dial", "arc", "numeral"], "palettes": ["theme"], "themes": "minimal2", "states": ["on", "alert"], "sizes": ["2x2"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "motion-H1-{family}", "kind": "motion", "types": ["switch"], "variants": ["pill"], "palettes": ["dark"], "states": ["on"], "sizes": ["2x1"], "motion": ["full", "reduced"], "frames": [0, 50, 100, 200, 400, 800], "scenarios": ["toggle", "pending"], "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "motion-H2-{family}", "kind": "motion", "types": ["switch"], "variants": ["rocker"], "palettes": ["dark"], "states": ["on"], "sizes": ["2x2"], "motion": ["full", "reduced"], "frames": [0, 50, 100, 200, 400, 800], "scenarios": ["toggle", "pending"], "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "motion-H3-{family}", "kind": "motion", "types": ["light"], "variants": ["dial"], "palettes": ["dark"], "states": ["on"], "sizes": ["2x2"], "motion": ["full", "reduced"], "frames": [0, 50, 100, 200, 400, 800], "scenarios": ["remote", "pending"], "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "motion-H4-{family}", "kind": "motion", "types": ["climate"], "variants": ["dial"], "palettes": ["dark"], "states": ["on"], "sizes": ["3x3"], "motion": ["full", "reduced"], "frames": [0, 50, 100, 200, 400, 800], "scenarios": ["target", "pending"], "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "motion-H5-{family}", "kind": "motion", "types": ["gauge"], "variants": ["needle"], "palettes": ["dark"], "states": ["on"], "sizes": ["2x2"], "motion": ["full", "reduced"], "frames": [0, 100, 200, 400, 600, 800, 1100], "scenarios": ["value"], "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "motion-H6-{family}", "kind": "motion", "types": ["gauge"], "variants": ["arc"], "palettes": ["dark"], "states": ["on"], "sizes": ["2x2"], "motion": ["full", "reduced"], "frames": [0, 50, 100, 200, 400, 800], "scenarios": ["value"], "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "motion-H7-{family}", "kind": "motion", "types": ["temperature"], "variants": ["numeral"], "palettes": ["dark"], "states": ["on"], "sizes": ["2x1"], "motion": ["full", "reduced"], "frames": [0, 50, 100, 200, 400, 800], "scenarios": ["value"], "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "motion-H8-{family}", "kind": "motion", "types": ["sensor"], "variants": ["sparkline"], "palettes": ["dark"], "states": ["on"], "sizes": ["2x2"], "motion": ["full", "reduced"], "frames": [0, 50, 100, 200, 400, 800], "scenarios": ["value"], "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "motion-H5-tactile~fallback", "kind": "motion", "families": ["tactile"], "types": ["gauge"], "variants": ["needle"], "palettes": ["dark"], "states": ["on"], "sizes": ["2x2"], "motion": ["full"], "ease": "fallback", "frames": [0, 100, 200, 400, 600, 800, 1100], "scenarios": ["value"], "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H1", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["switch"], "variants": ["pill"], "palettes": ["dark", "light"], "states": "switchCore", "sizes": ["1x1", "2x1"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H2", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["switch"], "variants": ["rocker"], "palettes": ["dark", "light"], "states": "switchCore", "sizes": ["1x2", "2x2"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H3", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["light"], "variants": ["dial"], "palettes": ["dark", "light"], "states": "core", "sizes": ["2x2", "3x3"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H4", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["climate"], "variants": ["dial"], "palettes": ["dark", "light"], "states": "core", "sizes": ["2x2", "3x3"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H5", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["gauge"], "variants": ["needle"], "palettes": ["dark", "light"], "states": "display", "sizes": ["2x2", "3x3"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H6", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["gauge"], "variants": ["arc"], "palettes": ["dark", "light"], "states": "display", "sizes": ["2x2", "3x3"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H7", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["temperature"], "variants": ["numeral"], "palettes": ["dark", "light"], "states": "display", "sizes": ["1x1", "2x1"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H8", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["sensor"], "variants": ["sparkline"], "palettes": ["dark", "light"], "states": "display", "sizes": ["2x1", "2x2"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H1~signoff", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["switch"], "variants": ["pill"], "palettes": ["dark", "light"], "states": "trio", "sizes": ["2x1"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H2~signoff", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["switch"], "variants": ["rocker"], "palettes": ["dark", "light"], "states": "trio", "sizes": ["2x2"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H3~signoff", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["light"], "variants": ["dial"], "palettes": ["dark", "light"], "states": "trio", "sizes": ["2x2"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H4~signoff", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["climate"], "variants": ["dial"], "palettes": ["dark", "light"], "states": "trio", "sizes": ["3x3"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H5~signoff", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["gauge"], "variants": ["needle"], "palettes": ["dark", "light"], "states": "signoffDisplay", "sizes": ["2x2"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H6~signoff", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["gauge"], "variants": ["arc"], "palettes": ["dark", "light"], "states": "signoffDisplay", "sizes": ["2x2"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H7~signoff", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["temperature"], "variants": ["numeral"], "palettes": ["dark", "light"], "states": "signoffDisplay", "sizes": ["2x1"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]},
    {"id": "side-by-side-H8~signoff", "kind": "side-by-side", "families": ["glass", "tactile", "minimal", "instrument"], "types": ["sensor"], "variants": ["sparkline"], "palettes": ["dark", "light"], "states": "signoffDisplay", "sizes": ["2x2"], "dpr": 1, "motion": "none", "more": false, "scripts": ["/testdata/homedash-lab/hero-fixtures.js"]}
  ]
}
```

Also add to `lists.themes`: `"minimal2": [{"theme": "minimal", "scheme": "light"}, {"theme": "minimal", "scheme": "dark"}]`.

Placeholder sheets use the same entries with family `placeholder`, for example `hero-H1-placeholder`. `{family}` accepts every entry of `lists.families`.

`ui/desktop_homedash_lab_cvd_test.go`:

```go
package ui

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHomeDashLabColourVisionSheets writes deuteranopia, protanopia and greyscale copies of the HomeDash hero sheets
// (M1 review rubric U4/U5) from a previous control-lab run. No browser. Opt-in: HOMEDASH_LAB_CVD=1.
func TestHomeDashLabColourVisionSheets(t *testing.T) {
	if os.Getenv("HOMEDASH_LAB_CVD") != "1" {
		t.Skip("set HOMEDASH_LAB_CVD=1 after a control-lab run")
	}
	dir := filepath.Join("..", "reports", "homedash-lab")
	all, err := filepath.Glob(filepath.Join(dir, "hero-*.png"))
	if err != nil || len(all) == 0 {
		t.Fatalf("no hero sheets in %s (err %v)", dir, err)
	}
	// Machado, Oliveira and Fernandes (2009), severity 1.0, applied in linear RGB.
	protan := [3][3]float64{{0.152286, 1.052583, -0.204868}, {0.114503, 0.786281, 0.099216}, {-0.003882, -0.048116, 1.051998}}
	deutan := [3][3]float64{{0.367322, 0.860646, -0.227968}, {0.280085, 0.672501, 0.047413}, {-0.011820, 0.042940, 0.968881}}
	var lin [256]float64
	for i := range lin {
		c := float64(i) / 255
		if c <= 0.04045 {
			lin[i] = c / 12.92
		} else {
			lin[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	enc := func(v float64) uint8 {
		v = math.Max(0, math.Min(1, v))
		if v <= 0.0031308 {
			v *= 12.92
		} else {
			v = 1.055*math.Pow(v, 1/2.4) - 0.055
		}
		return uint8(math.Round(v * 255))
	}
	mul := func(m [3][3]float64, r, g, b float64) (float64, float64, float64) {
		return m[0][0]*r + m[0][1]*g + m[0][2]*b, m[1][0]*r + m[1][1]*g + m[1][2]*b, m[2][0]*r + m[2][1]*g + m[2][2]*b
	}
	transforms := map[string]func(r, g, b float64) (float64, float64, float64){
		"cvd-protan-": func(r, g, b float64) (float64, float64, float64) { return mul(protan, r, g, b) },
		"cvd-deutan-": func(r, g, b float64) (float64, float64, float64) { return mul(deutan, r, g, b) },
		"grey-":       func(r, g, b float64) (float64, float64, float64) { y := 0.2126*r + 0.7152*g + 0.0722*b; return y, y, y },
	}
	count := 0
	for _, path := range all {
		base := filepath.Base(path)
		if strings.HasPrefix(base, "cvd-") || strings.HasPrefix(base, "grey-") {
			continue
		}
		in, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		src, err := png.Decode(in)
		in.Close()
		if err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		b := src.Bounds()
		for prefix, fn := range transforms {
			dst := image.NewNRGBA(b)
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
					r, g, bl := fn(lin[c.R], lin[c.G], lin[c.B])
					dst.SetNRGBA(x, y, color.NRGBA{R: enc(r), G: enc(g), B: enc(bl), A: c.A})
				}
			}
			out, err := os.Create(filepath.Join(dir, prefix+base))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(out, dst); err != nil {
				out.Close()
				t.Fatal(err)
			}
			out.Close()
		}
		count++
	}
	t.Logf("wrote colour-vision and greyscale copies of %d sheets", count)
}
```

### Task 1: Primitive additions, `hudFor`, `applyLook`, icons, i18n section, lab words, M1 Node test

**Files:** `ui/js/homedash/hd-prim-controls.js`, `hd-prim-displays.js`, `hd-colour.js`, `hd-widgets.js`, `hd-icons.js`; `ui/lang/homedash/*.json` (16, new); `ui/i18n_lint_test.go`; `ui/testdata/homedash-lab.js`; `scripts/homedash-tests/m1.test.mjs` (new); `scripts/homedash-freeze-tokens.mjs` (new).
**Read first:** M0 C2 and Tasks 11–14; this plan's Hero build sheets (DOM blocks); `ui/i18n_lint_test.go:446-503, 669-800, 1276-1309`.

- [ ] Run impact on `knob`, `toggle`, `segmented`, `needleGauge`, `arcGauge`, `numeral`, `sparkline`, `applyPalette` (to confirm hd-colour has no conflicting export) and `register` (hd-widgets).
- [ ] Write `m1.test.mjs` and `homedash-freeze-tokens.mjs` (below). Run `npm run test:homedash`. Expected: FAIL (the additions are missing).
- [ ] **Primitive additions.** Implement exactly this table. Defaults leave C2 output unchanged; nothing else may differ from C2.

| Builder | New option (default) | New element, attribute or property | Written by `set()` | Needed by |
| --- | --- | --- | --- | --- |
| toggle | `decor` (false) | Inside the track before the thumb: `span.hd-toggle-key`. After the track in the button: `span.hd-toggle-legend[data-glyph=off]` "O", `span.hd-toggle-legend[data-glyph=on]` "I", `span.hd-toggle-led` | key text "I"/"O" from `on` | H1 |
| knob | `glow` (false) | `path.hd-dial-glow[pathLength=1000]` before the arc, d = arc d | the same dasharray as the arc | H3 Glass and Instrument |
| knob | `gaps` (0) | `path.hd-dial-gaps[pathLength=1000]` last in the svg, d = track d | – | H3 Instrument |
| knob | `lead` (false; needs `quantize`) | `path.hd-dial-lead` after the arc | d = `arc(100,100,88, a(k−1), step)` for the last lit segment k, empty when none | H3 Instrument |
| knob | `dots` (null) = `{count, r, size, lit}` | `path.hd-dial-dots` and `path.hd-dial-dots-lit` first in the svg: `count` circles of radius `size` at radius `r`, evenly over the sweep (one path, circle subpaths) | lit d = the first round(frac·count) circles when `lit` | H3 Tactile, H4 Glass |
| knob | `ticksMajor` (0) | `path.hd-dial-ticks-major` = `ticks(100,100,76,88,start,sweep,ticksMajor)` | – | H4 Tactile |
| knob | `numerals` (0, at most 6) | `text.hd-dial-numeral` at r 67, values min…max | – | H4 Tactile |
| knob | `hardware` (false) | `span.hd-dial-knurl` before `span.hd-dial-cap` | – | H3, H4 Tactile |
| knob | `needle` (false; needs `activeSegment`) | `path.hd-dial-needle[d="M100 14V42"]` inside `g.hd-dial-current` | – | H4 Tactile |
| knob | `handleShadow` (false) | `circle.hd-dial-knob-shadow[cx=100][cy=13.5][r=11]` first in `g.hd-dial-handle` | – | H3, H4 Glass |
| knob | `word` (false) | `span.hd-dial-word` in the readout | set key `word` (string or null): text, root `data-hd-word="true"` when non-empty | H3 |
| knob | `extra` (false), `subReadout` (numeral), `subIcon`, `captions` ([main, sub] or null) | Readout children in order: `span.hd-dial-caption[data-for=main]`, `span.hd-dial-action`, main readout, `span.hd-dial-sub` > (`svg.hd-ico` subIcon, numeral or seg7), `span.hd-dial-caption[data-for=sub]` | set keys `action` (text) and `sub` (number or null) | H4 |
| segmented | `rotary` (false) | `span.hd-seg-rotary[aria-hidden=true] > span.hd-seg-rotary-pointer`, last child | – | H4 Tactile |
| needleGauge | `face` (false) | Before the svg: `span.hd-gauge-face[aria-hidden=true] > span.hd-gauge-bezel, span.hd-gauge-glass` | – | H5, H6 Tactile |
| needleGauge | `needleFx` (false) | `g.hd-gauge-needle-shadow > path` (needle d) before `g.hd-gauge-needle`; `path.hd-gauge-needle-glow` (needle d) first inside `g.hd-gauge-needle`. The shadow group copies `--hd-angle` through CSS. | – | H5 Tactile, Instrument |
| needleGauge, arcGauge | `readout` (numeral) | `seg7` builds `E.prim.segReadout({digits:4, decimals, unit})` in `span.hd-gauge-readout` | readout value | H5, H6 Instrument |
| arcGauge | `glow`, `gaps`, `lead` (as for the knob) | `path.hd-gauge-glow[pathLength=1000]` before the arc; `path.hd-gauge-lead` after it; `path.hd-gauge-gaps[pathLength=1000]` last | glow dasharray; lead d | H6 |
| arcGauge | `zoneOverlay` (false) | `path.hd-gauge-arc-zone[pathLength=1000]` after the arc, d = arc d | the same dasharray as the arc | H6 |
| numeral | `trendIcon` (false) | `span.hd-num-trend` holds `svg.hd-ico` (`trend-up`, `trend-down` or `trend-flat`) instead of the text glyph; `data-trend` unchanged | icon swap | H7 |
| sparkline | `fade` (false) | The area path moves into `svg.hd-spark-fill[aria-hidden=true][viewBox][width][height]`, returned as the extra field `fill`; `el` keeps line and dot | both svgs' d and size | H8 Glass |
| sparkline | `grid` (null) = `[cols, rows]` | `path.hd-spark-grid` first in `el` | d: interior lines at `round(w·i/cols)+0.5` and `round(h·j/rows)+0.5` | H8 Tactile, Instrument |
| sparkline | `halo` (false) | `circle.hd-spark-halo[r=6]` before `.hd-spark-last` | cx, cy and `hidden` like the end dot | H8 |

- [ ] **Engine additions** (not primitives):
  - **`E.colour.hudFor(hex)`** (pure). Convert to OKLCH. If chroma < 0.04, return `#e8edf2`. Otherwise L′ = max(L, 0.72); raise L′ in steps of 0.02 until `contrast(hex′, '#090c10') >= 4.5` (cap 0.97), reducing chroma in 0.01 steps when out of gamut. Return lowercase `#rrggbb`.
  - **`E.widgets.applyLook(root)`**:
    - Resolve `var(--hd-accent)` through a hidden probe span (`style.setProperty('color', 'var(--hd-accent)')`, then `getComputedStyle(probe).color` parsed to rgb), write `--hd-hud` with `style.setProperty`, and remove the probe.
    - Read `getComputedStyle(root).getPropertyValue('--vd-mat-card-radius')`. If it is non-empty and every number in it is 0, set `data-hd-corners="square"`, otherwise remove it.
    - The M2 desktop and kiosk hosts call it after a palette or theme change (documented in Task 24).
- [ ] **Icons.** Add the missing glyphs from the File structure row.
- [ ] **i18n section.** Create `ui/lang/homedash/<16>.json` with these keys and real translations (German with "Du" and real umlauts; each file a flat JSON object, prefix `homedash.`):

| Key | English |
| --- | --- |
| `homedash.state.on` / `.off` | On / Off |
| `homedash.state.pending` | Sending… |
| `homedash.state.unavailable` | Unavailable |
| `homedash.state.unknown` | Unknown entity |
| `homedash.state.error` | Not confirmed |
| `homedash.state.updated_ago` | Updated {{age}} ago |
| `homedash.state.readonly` | Read-only |
| `homedash.state.blocked` | Blocked by settings |
| `homedash.state.no_data` | No data |
| `homedash.climate.heating_to` / `.cooling_to` / `.idle` | Heating to / Cooling to / Idle |
| `homedash.climate.mode_off` / `_heat` / `_cool` / `_auto` | Off / Heat / Cool / Auto |
| `homedash.climate.lower` / `.raise` | Lower target / Raise target |
| `homedash.climate.target` / `.current` | Target / Now |
| `homedash.climate.valuetext` | {{value}}, {{action}} |
| `homedash.light.valuetext` | {{value}} %, {{state}} |
| `homedash.temperature.range` | {{min}} – {{max}} today |
| `homedash.sensor.summary` | min {{min}}, max {{max}}, now {{now}} |
| `homedash.trend.rising` / `.falling` / `.steady` | rising / falling / steady |

  Add the case `{relPath: filepath.Join("lang", "homedash"), prefixes: []string{"homedash."}}` to `TestTranslations_SectionFilesOnlyOwnPrefixes`. This lifts M0's M0-only rule: engine files now call `t('homedash.…')` with full literal keys.
- [ ] **Lab words.** In `ui/testdata/homedash-lab.js`, after `await document.fonts.ready` (rule 5): `const words = Object.assign({}, LAB_WORDS); for (const l of ['en', lang]) { const r = await fetch('/lang/homedash/' + l + '.json'); if (r.ok) Object.assign(words, await r.json()); }`. Use `E.core.makeT(words, {warn(){}})` in the widget cell (rule 10). Check in the next lab run that a hero cell's state line reads "Sending…".
- [ ] Run `npm run test:homedash`, then `go test ./ui -run '(?i)^Test(Translations_|Frontend_StaticI18nKeysExistInEnglishBundle|HomeDashLab)' -count=1`. Expected: PASS.
- [ ] Commit `feat(homedash): M1 primitive additions, hud colour, look helper, icons and homedash locale section`.

**Done when:** both commands pass and the `prim-placeholder` sheet is pixel-identical to the Task 0 baseline (defaults unchanged). Check it with the theme-polish `pngdiff` tool, or by viewing both sheets.

`scripts/homedash-freeze-tokens.mjs`:

```js
#!/usr/bin/env node
// HomeDash token freeze (M1 Task 22): extracts every rule whose declarations are all --hd-* custom properties.
// `node scripts/homedash-freeze-tokens.mjs` writes ui/testdata/homedash-lab/token-freeze.json (only with the lead's approval).
import fs from 'node:fs'; import path from 'node:path'; import { pathToFileURL } from 'node:url';
export const ROOT = path.resolve(import.meta.dirname, '..');
export const FREEZE_PATH = 'ui/testdata/homedash-lab/token-freeze.json';
export const FREEZE_FILES = ['ui/css/homedash-tokens.css', 'ui/css/homedash-bridge.css', 'ui/css/homedash-family-glass.css',
  'ui/css/homedash-family-tactile.css', 'ui/css/homedash-family-minimal.css', 'ui/css/homedash-family-instrument.css'];
export function tokenRules(text) {
  const css = text.replace(/\/\*[\s\S]*?\*\//g, ''), rules = {};
  for (const m of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    const selector = m[1].trim().replace(/\s+/g, ' ');
    if (selector.startsWith('@')) continue;
    const decls = m[2].split(';').map((d) => d.trim()).filter(Boolean);
    if (!decls.length || !decls.every((d) => d.startsWith('--hd-'))) continue;
    const body = rules[selector] || {};
    for (const d of decls) { const i = d.indexOf(':'); body[d.slice(0, i).trim()] = d.slice(i + 1).trim().replace(/\s+/g, ' '); }
    rules[selector] = body;
  }
  return rules;
}
export const current = () => Object.fromEntries(FREEZE_FILES.map((f) => [f, tokenRules(fs.readFileSync(path.join(ROOT, f), 'utf8'))]));
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  fs.writeFileSync(path.join(ROOT, FREEZE_PATH), JSON.stringify(current(), null, 2) + '\n');
  console.log('token freeze written to ' + FREEZE_PATH + '; commit it only with the lead\'s approval');
}
```

`scripts/homedash-tests/m1.test.mjs`:

```js
// HomeDash M1: hudFor contrast, the M1 primitive additions (Task 1 table) and the token freeze.
import assert from 'node:assert/strict'; import fs from 'node:fs'; import path from 'node:path';
import { loadEngine, root, ENGINE_ORDER } from './harness.mjs';
import { current, FREEZE_PATH } from '../homedash-freeze-tokens.mjs';
const PRIMS = ENGINE_ORDER.slice(0, ENGINE_ORDER.indexOf('hd-prim-controls.js') + 1);
const dash = (el) => el.style.getPropertyValue('stroke-dasharray');
export const tests = [
  ['hudFor reaches 4.5:1 on the Instrument panel and turns monochrome accents into white phosphor', () => {
    const { E } = loadEngine(['hd-core.js', 'hd-colour.js']);
    assert.equal(E.colour.hudFor('#161616'), '#e8edf2'); assert.equal(E.colour.hudFor('#ededed'), '#e8edf2');
    for (const hex of ['#ff2bd6', '#1d5faf', '#b5145f', '#c2186b', '#6688bb', '#f3e600', '#5ce1ff', '#e95420', '#0a84ff']) {
      const hud = E.colour.hudFor(hex); assert.match(hud, /^#[0-9a-f]{6}$/); assert.ok(E.colour.contrast(hud, '#090c10') >= 4.5, hex + ' -> ' + hud);
    } }],
  ['M1 additions follow the Task 1 table and leave C2 defaults unchanged', () => {
    const { E } = loadEngine(PRIMS, { dom: true });
    const plain = E.prim.knob({ label: 'k' }); assert.equal(plain.el.querySelector('.hd-dial-glow'), null); assert.equal(plain.el.querySelector('.hd-dial-dots'), null);
    const t = E.prim.toggle({ label: 'x', decor: true }); t.set({ on: true });
    assert.equal(t.el.querySelector('.hd-toggle-key').textContent, 'I'); assert.equal(t.el.querySelectorAll('.hd-toggle-legend').length, 2); assert.ok(t.el.querySelector('.hd-toggle-led'));
    const k = E.prim.knob({ label: 'd', glow: true, word: true, dots: { count: 21, r: 94, size: 4.5, lit: true }, hardware: true, handleShadow: true });
    k.set({ value: 64 }); assert.equal(dash(k.el.querySelector('.hd-dial-glow')), dash(k.el.querySelector('.hd-dial-arc')));
    assert.equal((k.el.querySelector('.hd-dial-dots-lit').getAttribute('d').match(/M/g) || []).length, 13); assert.ok(k.el.querySelector('.hd-dial-knurl'));
    k.set({ value: 0, word: 'Off' }); assert.equal(k.el.querySelector('.hd-dial-word').textContent, 'Off'); assert.equal(k.el.getAttribute('data-hd-word'), 'true');
    const q = E.prim.knob({ label: 'q', quantize: 40, lead: true, gaps: 40 }); q.set({ value: 50 }); assert.ok(q.el.querySelector('.hd-dial-lead').getAttribute('d')); assert.ok(q.el.querySelector('.hd-dial-gaps'));
    const th = E.prim.knob({ label: 'T', min: 5, max: 30, decimals: 1, activeSegment: true, needle: true, extra: true, subIcon: 'thermometer', ticksMajor: 5, numerals: 6 });
    th.set({ value: 21.5, current: 19.5, action: 'Heating to', sub: 19.5 }); assert.equal(th.el.querySelector('.hd-dial-action').textContent, 'Heating to');
    assert.ok(th.el.querySelector('.hd-dial-current .hd-dial-needle')); assert.equal(th.el.querySelectorAll('.hd-dial-numeral').length, 6);
    const s = E.prim.segmented({ label: 'm', options: [{ id: 'a', label: 'A' }, { id: 'b', label: 'B' }], rotary: true }); assert.ok(s.el.querySelector('.hd-seg-rotary .hd-seg-rotary-pointer'));
    const n = E.prim.needleGauge({ label: 'n', min: 0, max: 100, face: true, needleFx: true }); n.set({ value: 40 });
    assert.equal(n.el.querySelector('.hd-gauge-needle-shadow path').getAttribute('d'), n.el.querySelector('.hd-gauge-needle-shape').getAttribute('d')); assert.ok(n.el.querySelector('.hd-gauge-face .hd-gauge-bezel'));
    const a = E.prim.arcGauge({ label: 'a', min: 0, max: 100, zoneOverlay: true, glow: true, readout: 'seg7' }); a.set({ value: 64 });
    assert.equal(dash(a.el.querySelector('.hd-gauge-arc-zone')), dash(a.el.querySelector('.hd-gauge-arc'))); assert.ok(a.el.querySelector('.hd-gauge-readout .hd-seg7'));
    const nu = E.prim.numeral({ chars: 4, degree: true, unit: 'C', trendIcon: true }); nu.set({ value: 21.4, decimals: 1, trend: 'down' });
    assert.ok(nu.el.querySelector('.hd-num-trend svg')); assert.equal(nu.el.querySelector('.hd-num-trend').textContent, '');
    const sp = E.prim.sparkline({ label: 's', fade: true, grid: [4, 3], halo: true }); sp.set({ points: [1, 2, 3, 2], w: 120, h: 40, summary: 'x' });
    assert.ok(sp.fill && sp.fill.querySelector('.hd-spark-area')); assert.equal(sp.el.querySelector('.hd-spark-area'), null);
    assert.equal((sp.el.querySelector('.hd-spark-grid').getAttribute('d').match(/M/g) || []).length, 5);
    assert.equal(sp.el.querySelector('.hd-spark-halo').getAttribute('cx'), sp.el.querySelector('.hd-spark-last').getAttribute('cx')); }],
  ['the token rules equal the frozen set once Task 22 has written it', () => {
    const file = path.join(root, FREEZE_PATH); if (!fs.existsSync(file)) return;
    const frozen = JSON.parse(fs.readFileSync(file, 'utf8')), now = current(), problems = [];
    for (const f of new Set([...Object.keys(frozen), ...Object.keys(now)])) {
      const a = frozen[f] || {}, b = now[f] || {};
      for (const sel of new Set([...Object.keys(a), ...Object.keys(b)])) for (const name of new Set([...Object.keys(a[sel] || {}), ...Object.keys(b[sel] || {})])) {
        const was = (a[sel] || {})[name], is = (b[sel] || {})[name]; if (was !== is) problems.push(`${f} ${sel} ${name}: frozen ${was ?? '(absent)'} now ${is ?? '(absent)'}`); }
    }
    assert.deepEqual(problems, [], 'token freeze violated; a change needs the lead, then node scripts/homedash-freeze-tokens.mjs'); }]
];
```

### Task 2: Hero modules H1–H3 (hd-w-switch.js, hd-w-light.js)

**Files:** `ui/js/homedash/widgets/hd-w-switch.js`, `ui/js/homedash/widgets/hd-w-light.js` (new); `ui/testdata/homedash-lab.html` (two script tags after `widgets/hd-w-misc.js`); `scripts/homedash-tests/harness.mjs` (`ENGINE_ORDER`, alphabetical among `widgets/`).
**Read first:** H1–H3 sheets, Widget module contracts, M0 C3 and Task 13.

- [ ] Implement registration and the module rules 1–7 and 10. Variants in catalog order: switch `['pill', 'rocker', 'push', 'tile']`, light `['slider', 'fader', 'dial', 'tile']`.
- [ ] Add the script tags and the `ENGINE_ORDER` entries. `npm run test:homedash` must pass, including M0's load contract.
- [ ] Lab run, report-only: `hero-H1-placeholder,hero-H2-placeholder,hero-H3-placeholder`. Check every state cell renders and the console has no errors (probes.json `errors` empty).
- [ ] Commit `feat(homedash): switch and light hero modules (H1-H3)`.

**Done when:** the three placeholder sheets show every state with a real hero body.

### Task 3: Hero modules H4–H8 (hd-w-climate.js, hd-w-sensor.js)

**Files:** `ui/js/homedash/widgets/hd-w-climate.js`, `ui/js/homedash/widgets/hd-w-sensor.js` (new); `ui/testdata/homedash-lab.html`; `scripts/homedash-tests/harness.mjs`.
**Read first:** H4–H8 sheets, Widget module contracts (rules 8–10).

- [ ] Implement:
  - climate `['dial', 'stepper', 'scale']`;
  - gauge `['arc', 'needle', 'bar', 'tank']`;
  - temperature `['numeral', 'thermometer', 'comfort']`;
  - sensor `['numeral', 'sparkline', 'icon']` (the catalog order from M0's golden catalog wins if it differs).
  - `hd-w-sensor.js` stays under 700 lines. If it grows above that, move the gauge into `hd-w-gauge.js` and report it.
- [ ] Script tags and `ENGINE_ORDER` as in Task 2; `npm run test:homedash`.
- [ ] Lab run, report-only: `hero-H4-placeholder,…,hero-H8-placeholder`. `extra.m1-svgcount` must be ok.
- [ ] Commit `feat(homedash): climate, gauge, temperature and sensor hero modules (H4-H8)`.

**Done when:** all eight placeholder hero sheets render every state, and `m1-svgcount` passes.

### Task 4: Structure pass, then GATE S

**Files:** `ui/css/homedash-primitives.css`, `ui/css/homedash-widgets.css`, `ui/css/homedash.bundle.css`.
**Read first:** Shared frame geometry and every Geometry block of the hero sheets; visual part §3 (budget, forbidden techniques).

- [ ] Write the shared frame CSS, §T5's widgets.css block, the primitives.css shared structure and every Geometry block verbatim. Decor is hidden by default. No colours beyond `var()`.
- [ ] Add the T0 block to `homedash-tokens.css` (the shared defaults are needed by the frame rules).
- [ ] `npm run build:ui`; `npm run test:homedash`.
- [ ] Gate S lab run without report-only: `hero-H1-placeholder` … `hero-H8-placeholder`.
- [ ] Self-check against S1–S8 and list the sheet paths.
- [ ] Commit `feat(homedash): family-neutral hero geometry`. Then **LEAD REVIEW GATE S**.

**Done when:** the lead closes Gate S.

### Glass (V3): Tasks 5–8, then GATE G

**Task 5: Glass tokens and shared painting.**
- **Files:** `ui/css/homedash-family-glass.css`, bundle.
- **Read first:** §T1, §T5, visual part §2.1.
- [ ] Write §T1 verbatim and §T5's Glass rules (frame edge layer, glow layer, gradient blur).
- [ ] Paint title, state and icon badge per T5. `npm run build:ui`; `npm run test:homedash`.
- [ ] Lab run, report-only: `hero-H1-glass,hero-glass~gradient`. Expected: frames with sheen and edge, and blur visible on the gradient sheet.
- [ ] Commit `feat(homedash): Glass tokens and shared painting`.
- **Done when:** the guard is green and both sheets show the Glass frame on dark and light.

**Task 6: Glass H1–H4.**
- **Files:** `homedash-family-glass.css`, bundle.
- **Read first:** H1–H4 sheets (Glass rows).
- [ ] Paint H1–H4 as specified, including the pending forms and the MG transitions from each Motion table. Add `--hd-th-stroke: 12` and `--hd-th-ring-u: 2.5` to §T1's block (pre-approved here).
- [ ] Lab run, report-only: `hero-H1-glass` … `hero-H4-glass` and `motion-H1-glass` … `motion-H4-glass`. Fix every probe failure in these sheets.
- [ ] Commit `feat(homedash): Glass H1-H4`.
- **Done when:** the sheets match the hero sheets and those probes are clean.

**Task 7: Glass H5–H8.**
- **Files:** `homedash-family-glass.css`, bundle.
- **Read first:** H5–H8 sheets (Glass rows).
- [ ] Add `--hd-g-zone-u: 8` and `--hd-g-spark-w: 1.75px` to §T1's block. Paint H5–H8, including the MG transitions.
- [ ] Lab run, report-only: `hero-H5-glass` … `hero-H8-glass` and `motion-H5-glass` … `motion-H8-glass`. Fix probe failures.
- [ ] Commit `feat(homedash): Glass H5-H8`.
- **Done when:** the sheets match and those probes are clean.

**Task 8: Glass state and motion pass, gate sheets.**
- **Files:** `homedash-family-glass.css`, bundle.
- **Read first:** §T5 state table; the Glass rubric lines G1–G7.
- [ ] Check all 12 states on the hero sheets. Motion: thumb overshoot ≤ 2 px; the arc end stays under the handle and cap dot in every H3 and H6 frame.
- [ ] Gate G run without report-only (Review protocol command). Then `$env:HOMEDASH_LAB_CVD='1'; go test ./ui -run '^TestHomeDashLabColourVisionSheets$' -count=1` from W.
- [ ] Fix every probe failure. Commit `feat(homedash): Glass states and motion`. Then **LEAD REVIEW GATE G**.
- **Done when:** the lead closes Gate G.

### Tactile (V4): Tasks 9–13, then GATE T

**Task 9: Tactile tokens, materials, LED states and shared painting.**
- **Files:** `ui/css/homedash-family-tactile.css`, bundle.
- **Read first:** §T2, §T5, visual part §2.2.
- [ ] Write §T2 verbatim, including the scheme switch and the LED state rules.
- [ ] Paint the shared states (hatch for unavailable) and the engraved titles (Barlow with `--hd-t-emboss`).
- [ ] Lab run, report-only: `hero-tactile~dpr15`. Check for brush moiré before going on.
- [ ] Commit `feat(homedash): Tactile tokens and materials`.
- **Done when:** panels show graphite on dark and aluminium on light, without stripes at DPR 1.5.

**Task 10: Tactile H1–H3** (slide switch, rocker, knob with LED dots).
- **Files:** `homedash-family-tactile.css`, bundle.
- [ ] Paint per the sheets. Lab run, report-only: `hero-H1-tactile,hero-H2-tactile,hero-H3-tactile,hero-H3-tactile~dpr2,motion-H1-tactile,motion-H2-tactile,motion-H3-tactile`.
- [ ] Commit `feat(homedash): Tactile H1-H3`.
- **Done when:** knurl and grip show no moiré at DPR 1, 1.5 and 2, and the LED lights after the travel ends.

**Task 11: Tactile H4–H6** (thermostat knob and rotary, needle instrument, H6 alias).
- **Files:** `homedash-family-tactile.css`, bundle.
- [ ] Add `--hd-th-stroke: 12` to §T2. Paint per the sheets.
- [ ] Lab run, report-only: `hero-H4-tactile,hero-H5-tactile,hero-H6-tactile,motion-H4-tactile,motion-H5-tactile,motion-H5-tactile~fallback,motion-H6-tactile`.
- [ ] Commit `feat(homedash): Tactile H4-H6`.
- **Done when:** the needle shadow stays down-right at every angle, and the H6 cells show the needle instrument.

**Task 12: Tactile H7–H8** (VFD readout, trace window).
- **Files:** `homedash-family-tactile.css`, bundle.
- [ ] Paint per the sheets. Lab run, report-only: `hero-H7-tactile,hero-H8-tactile,motion-H7-tactile,motion-H8-tactile`.
- [ ] Commit `feat(homedash): Tactile H7-H8`.
- **Done when:** the VFD digits glow amber within the window, and the trace sits inside the inset window.

**Task 13: Tactile state, motion and gate sheets.**
- [ ] Check all states. The detent is visible within 1–2 frames on `motion-H1-tactile` and `motion-H2-tactile`.
- [ ] Gate T run; colour-vision run.
- [ ] Fix probe failures. Commit `feat(homedash): Tactile states and motion`. Then **LEAD REVIEW GATE T**.
- **Done when:** the lead closes Gate T.

### Minimal (V5): Tasks 14–15, then GATE M

**Task 14: Minimal tokens, shared painting, H1–H4.**
- **Files:** `ui/css/homedash-family-minimal.css`, bundle.
- **Read first:** §T3, the H1–H4 Minimal rows.
- [ ] Write §T3 verbatim plus `--hd-th-stroke: 12`. Paint the shared states and H1–H4.
- [ ] Lab run, report-only: `hero-H1-minimal` … `hero-H4-minimal` and `motion-H1-minimal` … `motion-H4-minimal`.
- [ ] Commit `feat(homedash): Minimal tokens and H1-H4`.
- **Done when:** no shadow or gradient anywhere, and the sheets match.

**Task 15: Minimal H5–H8, states, motion and gate sheets.**
- **Files:** `homedash-family-minimal.css`, bundle.
- [ ] Paint H5–H8. Gate M run; colour-vision run.
- [ ] Fix probe failures. Commit `feat(homedash): Minimal H5-H8 and states`. Then **LEAD REVIEW GATE M**.
- **Done when:** the lead closes Gate M.

### Instrument (V6): Tasks 16–19, then GATE I

**Task 16: Instrument tokens, gap overlay, seven-segment painting and scan windows.**
- **Files:** `ui/css/homedash-family-instrument.css`, bundle.
- **Read first:** §T4, the Instrument rows of H3, H6 and H7.
- [ ] Write §T4 verbatim. Paint the shared states (NO DATA box), seven-segment ghosts and lit segments, and the `--hd-i-scan` overlay inside seven-segment readouts only.
- [ ] Lab run, report-only: `hero-minimal-theme~instrument`. The HUD must read on Minimal light and dark (`applyLook` wrote `--hd-hud`).
- [ ] Commit `feat(homedash): Instrument tokens and readouts`.
- **Done when:** the HUD measures at least 4.5:1 in probes.json on both Minimal variants.

**Task 17: Instrument H1–H4.**
- **Files:** `homedash-family-instrument.css`, bundle.
- [ ] Add `--hd-th-stroke: 12` to §T4. Paint per the sheets.
- [ ] Lab run, report-only: `hero-H1-instrument` … `hero-H4-instrument` and `motion-H1-instrument` … `motion-H4-instrument`.
- [ ] Commit `feat(homedash): Instrument H1-H4`.
- **Done when:** segment gaps are uniform and there is one glow colour per widget.

**Task 18: Instrument H5–H8.**
- **Files:** `homedash-family-instrument.css`, bundle.
- [ ] Add `--hd-i-needle-glow-u: 3` to §T4. Paint per the sheets.
- [ ] Lab run, report-only: `hero-H5-instrument` … `hero-H8-instrument` and the matching motion sheets.
- [ ] Commit `feat(homedash): Instrument H5-H8`.
- **Done when:** the sheets match and those probes are clean.

**Task 19: Instrument state and motion pass, gate sheets.**
- [ ] Check flash, blink and pulse (6 cycles). Gate I run; colour-vision run.
- [ ] Fix probe failures. Commit `feat(homedash): Instrument states and motion`. Then **LEAD REVIEW GATE I**.
- **Done when:** the lead closes Gate I.

Gate order rules:
- G first.
- T after G.
- M and I may run in parallel after G closes: their files are disjoint.
- Any structure change after Gate S (primitives or widgets CSS, modules, the Task 1 additions) is a structure change request to the lead, with before and after sheets of every family already passed.

### Task 20: Cross-family consistency (V7)

**Files:** the four family files and the bundle. A structure change needs the lead.
**Read first:** rubric X1–X6.

- [ ] Lab run without report-only: `side-by-side-H1` … `side-by-side-H8` and `hero-glass~custom,hero-tactile~custom,hero-minimal~custom,hero-instrument~custom`.
- [ ] Blind-crop check (X1) and density check (X2). Fix every finding.
- [ ] Commit `fix(homedash): cross-family consistency`.

**Done when:** X1–X4 pass on fresh sheets.

### Task 21: Risk-theme and theme-sweep fixes (V7)

**Files:** the four family files, `homedash-tokens.css` (T0 only), bundle.
**Read first:** Risks and fallbacks.

- [ ] Lab run: `hero-themes-glass,hero-themes-minimal,hero-risk-tactile,hero-risk-instrument,hero-minimal-theme~instrument`. Fix:
  - Amiga focus;
  - Green Slime radius;
  - square corners in Amiga, Cyberpunk and Voxel World;
  - Crazy and Cyberpunk saturation;
  - Fruity, Lollipop, XP and Voxel light contrast;
  - the Instrument HUD on Minimal.
- [ ] Only through tokens or `applyLook`.
- [ ] Commit `fix(homedash): risk themes`.

**Done when:** X5 passes and the contrast probes are clean on the five sheets.

### Task 22: Token freeze, then GATE X

**Files:** `ui/testdata/homedash-lab/token-freeze.json` (new).

- [ ] Full regression run of every gate set without report-only.
- [ ] `node scripts/homedash-freeze-tokens.mjs`.
- [ ] `npm run test:homedash` must pass with the freeze active.
- [ ] Commit `test(homedash): freeze the M1 token rules`. Then **LEAD REVIEW GATE X**.

**Done when:** the lead closes Gate X.

### Task 23: User sign-off pack, then USER GATE

**Files:** none tracked (output under `reports/homedash-lab/signoff/`).

- [ ] Lab run: `side-by-side-H1~signoff` … `side-by-side-H8~signoff`, `hero-themes-glass`, `hero-risk-instrument`.
- [ ] Copy into `<W>\reports\homedash-lab\signoff\` as:
  - `01-toggle.png`, `02-rocker.png`, `03-dimmer.png`, `04-thermostat.png`, `05-needle.png`, `06-arc.png`, `07-temperature.png`, `08-sensor.png`;
  - `09-glass-in-themes-<n>.png` (the first two parts: Standard to Crazy);
  - `10-instrument-risk-<n>.png` (the first part).
- [ ] Write `signoff\CAPTIONS.txt` in German, one line per image saying what to look at.
- [ ] The lead shows the pack to the user. No M3+ widget batch starts before the user's yes.

**Done when:** the user has given the yes.

### Task 24: Documentation (docs-agent)

**Files:** `documentation/homedash-visual-system.md` (new), `ui/js/homedash/AGENTS.md` (M0 file, edit), `ui/AGENTS.md` (one pointer line).
**Read first:** this plan; `token-freeze.json`.

- [ ] Document:
  - the four family recipes (T1–T4 summaries);
  - the frozen token table, generated from `token-freeze.json`;
  - the lab sheet ids and commands;
  - the review protocol and anti-drift rules;
  - the Task 1 primitive additions;
  - the host duties for M2: call `E.widgets.applyLook(root)` after palette or theme changes; compute the alert tone from widget options and device classes and pass it as the view `tone`, the way the lab fixtures do.
- [ ] `git add -f` where needed; verify with `git ls-files`.
- [ ] Commit `docs(homedash): visual system and style-lab rules`.
- [ ] Landing on main is the lead's decision after user sign-off (rebase, `check:ui`, `merge --ff-only`).

**Done when:** `git ls-files` lists the three files with the new content.

## Review protocol

### How the lead looks

- **Order per gate:**
  1. probes.json first, including `extra` (any failure is an automatic finding);
  2. the hero sheets `hero-Hn-<family>` at 100 % zoom (1×, never scaled down to fit), core rows first, then the extra states;
  3. the same sheets at 400 % for crispness;
  4. the `~dpr2` sheets;
  5. the theme or risk sweep;
  6. the motion strips;
  7. greyscale and colour-vision copies.
- **Blind crop.** For the cross-family rubric, cover the labels and crop a 120×120 px region of each family's default H1 and H3 tile.
- **Fresh sheets only.** Every judgement uses sheets with a timestamp newer than the last commit of the task under review.

### Commands (PowerShell, sandbox off, from `<W>\ui`, using the prebuilt binary)

```powershell
$env:AURAGO_RUN_BROWSER_SMOKE='1'; Remove-Item Env:HOMEDASH_LAB_REPORT_ONLY -ErrorAction SilentlyContinue
$f = 'glass'; $h = 1..8 | ForEach-Object { "H$_" }
$ids = @($h | ForEach-Object { "hero-$_-$f" }) + @($h | ForEach-Object { "hero-$_-$f~dpr2" }) + @($h | ForEach-Object { "motion-$_-$f" })
$env:HOMEDASH_LAB_SHEETS = ($ids + @('<gate extras>')) -join ','
& D:\tmp\go-build\hd-m1-ui.test.exe -test.run '^TestDesktopHomeDashControlLabBrowser$' -test.count 1 -test.timeout 30m -test.v
# colour-vision copies afterwards (from <W>):
$env:HOMEDASH_LAB_CVD='1'; go test ./ui -run '^TestHomeDashLabColourVisionSheets$' -count=1
```

- Rebuild the binary only after a Go or testdata change: `go test -c -o D:\tmp\go-build\hd-m1-ui.test.exe ./ui` from W. CSS and engine JS are read from disk.
- Output: `<W>\reports\homedash-lab\<id>-NN.png` and `probes.json` (the repository-root `reports/`, git-ignored).

| Gate | `$f` | `<gate extras>` | Expected time |
| --- | --- | --- | --- |
| S | placeholder | none; use only the `hero-$_-$f` ids | ≤ 5 min |
| G | glass | `hero-themes-glass`, `hero-glass~gradient`, `hero-glass~gradient-dpr2` | ≤ 25 min |
| T | tactile | `hero-risk-tactile`, `hero-tactile~dpr15`, `motion-H5-tactile~fallback` | ≤ 25 min |
| M | minimal | `hero-themes-minimal` | ≤ 25 min |
| I | instrument | `hero-risk-instrument`, `hero-minimal-theme~instrument` | ≤ 25 min |
| X | – | `side-by-side-H1` … `side-by-side-H8`, `hero-glass~custom`, `hero-tactile~custom`, `hero-minimal~custom`, `hero-instrument~custom`, then the G/T/M/I sets again | ≤ 70 min |

If a run takes more than twice the expected time, check for leftover rod Chromes and report them. Never stop processes by image name.

### Rubric: universal (every gate, every family)

| ID | Pass criterion, checkable on the sheet |
| --- | --- |
| U1 | probes.json: zero contrast, hit-area, layout-shift, settle, backdrop-count, `m1-svgcount` failures for the gate's sheets |
| U2 | At DPR 1 and 400 % zoom, every straight hairline is one solid device row or column. No two-row grey smear. |
| U3 | Within each hero row, the title baseline, the control box and the state line sit on the same pixel rows in every state column (check with a horizontal ruler line). |
| U4 | On `grey-…`, each core state is identified by a non-colour cue. The lead writes the cue per state; a state without one fails. |
| U5 | On `cvd-deutan-…` and `cvd-protan-…`, ok and alert remain distinguishable by glyph or word. |
| U6 | No control resembles a browser default: no UA focus outline (except Amiga's added dashed line), no grey bevel button, no serif fallback font. |
| U7 | Tabular figures: in the min and max columns the numeral's right edge is at the same x (± 0.5 px), and `m1-tabular` passes for every family font in use. |
| U8 | At most three text sizes per widget. |
| U9 | At most two hues per widget (accent plus one state hue); Instrument exactly one glow colour. |
| U10 | The focus ring is visible against card and canvas on dark and light. In Amiga the dashed outline hugs the same shape as the ring. |
| U11 | Minimum size: at least 4 px clear space between any text and any control edge. Nothing clipped except the full-bleed spark. |
| U12 | Pending: exactly one moving or static indicator per control, and the word "Sending…" is present. |
| U13 | No tofu (missing-glyph boxes). Arrows and triangles are icons, not text. |

### Rubric: family

| ID | Pass criterion |
| --- | --- |
| G1 | Light palette, 1×: the white thumb is separated from a white card by a visible edge ring plus contact shadow. |
| G2 | Dark palette: the card's top 1px highlight is visible as a lighter row along the top edge. |
| G3 | Fruity light, Lollipop and Minimal light: there is no grey band under cards. The shadow fades out within 32 px and the card edge stays crisp. |
| G4 | Theme-native: Amiga card with black border and bevel, Organic brass border visible, Green Slime's four-value radius preserved, Amiga, Cyberpunk and Voxel World controls square (`data-hd-corners`). |
| G5 | Motion strip H1: thumb overshoot visible at the 200 ms frame and ≤ 2 px; settled at 400 ms. H3 and H6: the arc end sits under the handle or cap dot in every frame. |
| G6 | The "on" fill reads as a fill with an edge on Fruity light (not a pale blob). |
| G7 | `hero-glass~gradient` and `~gradient-dpr2`: the card reads as glass (the backdrop's accent pool is visible, softened, through the card), every text passes the contrast probe, and the backdrop-filter count equals the number of cards. |
| T1 | DPR 1, 1.5 and 2: no beat bands on the knurl ring, no stripes in the brushed panel at 1× (grain visible only at 2×). |
| T2 | One light direction: every highlight top-left, every cast shadow bottom-right (the needle shadow too, at every angle), no shadow in a second direction. |
| T3 | Legends ≥ 10 px and engraved (1px emboss row visible at 2×) on both panel materials. |
| T4 | At most one panel and one control material per widget. No screws or rivets in M1 sizes. |
| T5 | A lit LED shows a bright hot spot plus halo. An unlit LED reads as dark glass, not flat grey. Pending is amber, alert and error red. |
| T6 | Motion strips H1 and H2: the detent bounce is visible within 1–2 frames. The LED lights only after the travel ends. |
| T7 | The reflection ellipse is subtle (does not read as a white blob at 1×). The fallback strip `motion-H5-tactile~fallback` settles cleanly. |
| M1 | Zoom 400 %: flat fills only, no gradient, no shadow anywhere. |
| M2 | Hairlines exactly 1 device px at DPR 1. |
| M3 | In the side-by-side sheets Minimal's numerals are the largest at equal size. |
| M4 | Accent appears only in the focus ring and value arcs. |
| M5 | Label, numeral and state line share one left edge. |
| I1 | One glow colour per widget, including alert (alert replaces HUD). |
| I2 | Seven-segment digits have uniform segment thickness. Ghosts are visible at 7 % but never read as another digit at arm's length (judged on the 1× sheet). |
| I3 | Segment gaps are uniform along the arc at 1×; no merged or missing gap. |
| I4 | Scan lines only inside readout windows. |
| I5 | Corner brackets are solid 2 px lines on every palette. |
| I6 | No text under 11 px. The HUD is readable on Minimal light and dark (`hero-minimal-theme~instrument`). |

### Rubric: hero (every family)

| ID | Pass criterion |
| --- | --- |
| H1a | Thumb centred vertically within 0.5 px. End gaps equal in off and on. Glyph or legend readable at 1×. |
| H2a | The pressed half is unmistakable in greyscale. The paddle ratio is 1:1.6 ± 2 %. The bezel inset is even on all four sides. |
| H3a | Arc ends symmetric about the vertical axis. The handle is centred on the stroke. The numeral's box centre lies within 2 px of the ring centre. "Off" is not shown as "0 %". |
| H4a | The active segment uses at most two hues. The current marker sits outside the arc without touching it. The steppers are inside the gap chord with ≥ 4 px clearance. The mode indicator is aligned to its option. |
| H5a | Ticks crisp. ≤ 6 numerals. Motion strip: the needle peak exceeds the target by 5–8 % of the change at the 400–600 ms frames, and the fallback strip settles without visible overshoot artefacts. |
| H6a | The cap dot is centred on the stroke end. The zone colour shows at alert. The min/max legends align to the arc ends. The Tactile alias is a needle instrument. |
| H7a | The top of the decimal and of the degree sign align with the top of the integer within 1 px. The negative value fits without shifting the box. The trend glyph is 0.3× the numeral. |
| H8a | The end dot is a circle, not an ellipse (not stretched). Round joins. The area fades to 0 at the bottom. Clipped exactly by the card radius (no corner bleed). The gap in the data is visible as a break. |

### Rubric: Gate S (structure) and Gate X (cross-family)

| ID | Pass criterion |
| --- | --- |
| S1 | Every hero renders every lab state of its sheet; probes.json has no `errors` entries. |
| S2 | Measured at 2× zoom: toggle 52×32 with thumb 26; rocker ratio 1:1.6 ± 2 %; dial D equals the formula ± 1 px; needle box 0.84 D; arc box 0.86 D; spark 38 % of the height. |
| S3 | The hit probe in probes.json passes for every role element. At 400 %, no control box overlaps title or state text. |
| S4 | Title and state rows at the same y across states (as U3). |
| S5 | 1×1 and 2×1: nothing clipped, at least 4 px clearance. |
| S6 | Numerals centred in dials (as H3a). |
| S7 | `extra.m1-svgcount` is ok (≤ 150 nodes everywhere). |
| S8 | `extra.m1-tabular` is recorded. A failing font triggers its fallback (Risks) before painting starts. |
| X1 | Blind 120 px crops: the lead names all four families correctly for H1 and H3. |
| X2 | Side by side, no family looks unfinished: equal detail density and equal visual weight of the "on" state. |
| X3 | Structure identical across families at equal size: head, body and state rows at the same y. Families differ by paint only. |
| X4 | The custom palettes pass U1 for all families. |
| X5 | The risk sweeps show no illegible state on the eight risk themes. |
| X6 | The token freeze is written and `npm run test:homedash` enforces it. |

### Iteration loop

1. **Findings file.** The lead writes `reports/homedash-lab/findings/<gate>-r<round>.md`. It holds a numbered table with the columns ID (`G-R1-07`), Sheet file, Hero, Family, State, Size, Palette or theme, Observation, Required change, Rubric ID. Every finding must be closed; there is no "minor" tier, because this is the quality bar.
2. **Hand-off.** The same implementing agent (or a new maker-agent given the findings file plus this plan) fixes the findings. It replies per ID with: the change (file and rule), the fresh sheet path showing the fix, and any side effect it saw.
3. **Re-check.** The lead runs, or has run, the complete gate set again (fixes break neighbours). It closes each ID on the fresh sheets and adds new findings as round r+1.
4. **Pass.** A gate passes only when every finding of every round is closed on sheets newer than the last commit. There is no round limit for heroes.
5. **Schedule instead of bar.**
   - A family in revision blocks V7 and M3. It does not block parallel families whose files are disjoint.
   - If the same finding survives three rounds, the lead takes a design decision from the Risks fallbacks (an approach change). The bar never drops.
   - The lead reports the schedule impact to the user.

### Anti-drift rules for implementers (put them in every dispatch)

1. Values come only from this plan's Token and recipe sheet and the Hero build sheets.
2. Names and markup come only from M0's C2 plus the Task 1 additions table. Never rename or restructure an M0 element.
3. A missing value, new token or deviation is proposed in the hand-back with a reason and a sheet showing why. Never improvise. Until the lead approves, use the plan's value.
4. No literal colour outside a token block; the guard enforces it. Gradients, shadow stacks and masks are tokens too. `stroke-width`, `border-radius` and `box-shadow` in family files go through `var(--hd-…)`.
5. A token that can be `none` (`--hd-theme-edge`, `--hd-theme-glow`, Minimal and Instrument shadows) is never placed inside a comma list.
6. Never `calc()` on `--hd-theme-radius`.
7. Every visual claim in a hand-back cites a sheet file path and the state, size and palette cell. A claim without a sheet counts as not done.
8. No edits outside the task's Files list. A structure change after Gate S needs a structure change request.
9. After Task 22, token rule changes need the lead, and then `node scripts/homedash-freeze-tokens.mjs`.

## Risks and fallbacks

| Risk | Detection | Fallback that keeps the bar |
| --- | --- | --- |
| Glass alpha and contrast on light themes | Contrast probe on `hero-themes-glass` light rows (Fruity, Lollipop, XP, Voxel, Minimal) and `hero-glass~custom` warm-light | Raise `--hd-glass-alpha-s` light to 84 %, then 88 %. If text still fails, drop the sheen to 5 % and use `--hd-ink` for labels. |
| Glass blur invisible because `.hd-widget` has `contain: layout paint style` (M0 base) | `hero-glass~gradient`, rubric G7 | Structure change request: `contain: layout style` for Glass widgets under the gradient backdrop. The blur stays on `.hd-frame` only. |
| Glass "on" fill below 3:1 on white cards (Fruity primary 2.8:1) | Graphics probe on `.hd-toggle-thumb` and the track fill | The fill edge through `--hd-fill-edge-k`; tighten to 64 % if still flagged. |
| Value-arc sweep and handle drift apart | Motion strips H3 and H6 (G5) | Both use `--hd-dur-remote` / `--hd-ease-remote`; the arc's `d` is the full sweep (C2), so `stroke-dasharray` and the rotation share one angle mapping. If a frame still differs, align the handle to `quantize` steps for Instrument only. |
| Conic-gradient knurling aliasing or moiré | `hero-tactile~dpr15`, DPR 1 and 2 (T1) | Widen the soft stops to 1.2° and reduce the ridges to 36. Next: 48 radial strokes as one knob path (a Task 1 addition by structure change request). |
| Brushed grain moiré at fractional DPR | `hero-tactile~dpr15` | Period 6 px with the same soft profile, or no grain below 2×2. |
| Seven-segment digits look crude | I2 on `hero-Hn-instrument~dpr2` | Adjust thickness and chamfer in M0's `segmentPaths` (M0 test updated, reported to the lead). Last resort, by lead decision: Share Tech Mono for hero readouts. |
| Tabular figures missing in a bundled subset | `extra.m1-tabular` (S8) | Glass → Geist; Minimal → Geist Mono for changing readouts; Tactile VFD already Geist Mono; Instrument secondary readouts already monospace. |
| Needle easing fallback | `motion-H5-tactile~fallback` (T7) | M0's `.hd-root[data-hd-ease="fallback"]` value. If it looks wrong, propose `cubic-bezier(.25,1.25,.5,1)` to the M0 owner. |
| Four-value theme radius (Green Slime) | `hero-themes-*` Green Slime rows (G4) | Radius passed whole; inner elements use family radii; the spark is clipped by the frame's `overflow: hidden`. |
| Amiga's forced `:focus-visible` dashed outline | `hero-themes-*` Amiga focus column (U10). The lab uses `data-hd-focus`, so Amiga's real `:focus-visible` outline is checked once by hand: Tab to a control in `hero-themes-glass` at Amiga with Chrome DevTools | The ring sits on the hit box with the control radius, and corners go square through `data-hd-corners`. |
| Instrument HUD on monochrome accents | `hero-minimal-theme~instrument` | `hudFor` returns white phosphor `#e8edf2` for chroma < 0.04. |
| Light-theme status inks on Instrument and Tactile LEDs | `hero-risk-*` alert column | Fixed status set (T4) and fixed LED colours (T2). |
| The fixture script's `probes` wrapper misses M0's assignment order | `probes.json` lacks `extra.m1-*` | The wrapper keeps any earlier `hdLab.probes` and traps later assignments. If the lab freezes `hdLab`, propose to the M0 owner that the lab calls a registered pre-probe hook. |
| Focus cells of widgets: M0's lab marks only `.hd-more`, which M1 sheets omit | Focus column without a ring | `hero-fixtures.js` marks the role element. Proposed to M0 for its rule 10. |
| Headless Chrome inherits the Windows reduced-motion setting | `full` motion rows show no intermediate frames | Report it to the M0 owner (the lab must emulate `prefers-reduced-motion: no-preference` for `motion=full`). |
| Leftover rod Chromes | Run time over twice the table | Report profile ids and dates to the lead; stop them only with the owner's yes. |
| A family stylesheet exceeds 900 lines (Tactile is likeliest) | Guard line limit | With the lead's OK, add `homedash-family-tactile-heroes.css` to the bundle after the family file (M0 bundle test updated). |
| GitNexus unavailable | Standard commit step | Grep the callers and write the note in the commit body. |

## Self-review

### Coverage

| Requirement | Where |
| --- | --- |
| Main §6: tokens, families, shared rules (non-colour cues, contrast, 44 px, motion limits, icon set, theme behaviour) | T0–T5, frame geometry, Task 1 icons and masks, rubrics U4–U13 |
| Main §7.1 (32 renderings, 6 states, 2 sizes, palettes and themes, 1× and 2×, motion strips) | The M1 sheet entries (Task 0), Tasks 8, 13, 15, 19 |
| Main §7.2–7.3 (lead gates on sheets, user sign-off) | Review protocol, Gates S/G/T/M/I/X, Task 23 |
| Main §7.4–7.5 (control lab probes, checklist, cheap list) | Rubrics; M0 probes plus `m1-tabular` and `m1-svgcount`; anti-drift rules |
| Main §8 M1 = V3–V7 | Tasks 5–22 |
| Visual §2 (families) | T1–T4 and the paint blocks of every hero |
| Visual §4.1 H1–H8 | Hero build sheets |
| Visual §7 (states) | Shared state language; per-hero state tables; host precedence plus module rule 5 |
| Visual §8 (accessibility) | A11y lines per hero; M0 focus ring plus §T5 layers; probes; colour-vision test |
| Visual §10.1 | Sheet entries and gate rubrics; Glass blur through `hero-glass~gradient` (G7) |
| Visual §10.3 | Items 1, 2, 4–8, 10–13 → rubrics and tasks. Items 3 and 9 (keyboard, optimistic flow) are P1. |
| Visual §10.4 | #1 G3/G7/U1; #2 tokens plus guard; #3 U7; #4 U9; #5 T4; #6 U2/T1; #7 icons and masks; #8 U12; #9 transform/opacity plus swept arcs on small SVGs; #10 U8/U11/S5; #11 locale sweep stays in V15 |

### Contradictions and gaps in the design (with the resolution used)

1. **DOM names.** Visual §1.2 and §2.1 use `.hd-w` and `.hd-card`. M0's C2 and C3 are authoritative: `.hd-widget`, `.hd-frame`, `data-glyph`, `.hd-rocker-legend`, `.hd-rocker-led`, `data-hd-gauge`, `.hd-gauge-min`/`-max`, `svg.hd-spark`.
2. **Guard regex vs scheme-dependent family literals.** Both literal sets sit in the family block; a `var()`-only rule switches between them (T2).
3. **Specificity.** The palette bridge outranks family blocks on `.hd-root`. M0's host writes `data-hd-family` on every widget (C3), so family tokens resolve per widget.
4. **Status colours.** Main §6 says Tactile and Instrument take status colours, but light-theme inks fail on Instrument's panel and look unlit as LEDs. Fixed sets are used (T2, T4).
5. **HUD lift.** `color-mix(accent 85%, white)` fails on Minimal's `#161616`; `hudFor` plus `applyLook` fix it.
6. **`none` in lists.** `--vd-mat-card-shadow` and `--vd-mat-glow` can be `none`, so they go on separate pseudo layers (§T5).
7. **Arc motion.** Value arcs sweep (`stroke-dasharray` transition, allowed by M0's guard) in step with the rotated handle and cap. Buffers exist only for H4's active segment (C2). Instrument's segmented arcs need no buffer: they snap, and a gap overlay path draws the segments (Task 1 additions).
8. **SVG area gradient** (H8) vs "no gradient defs". A separate fill SVG masked with `--hd-fade-down` (addition `fade`).
9. **Sizes.** H3, H5 and H6 have min = default; the lab adds 3×3. H4 at 2×2 hides the mode bar and steppers. Overview sheets (themes, risk, custom, gradient, DPR 1.5) use 2×2 for every hero, because one sheet entry has one size list. Hero, side-by-side and motion sheets are per hero, with exact sizes.
10. **Glyph coverage.** ▲ and → are not in the bundled Inter subset. The numeral `trendIcon` addition and the CSS mask badges replace them.
11. **Alerts.** The view state's `tone` carries alerts: lab fixtures set `tone: 'crit'` (M0's tone vocabulary `ok|warn|crit`; the reconciliation's "alert" maps to `crit`), and gauges add zone tones through `api.setTone`. `attributes.hd_alert` is gone. M2 computes the product tone (Task 24).
12. **Zone bands.** M0 places needle-gauge zones at r 84 (under the minor ticks) rather than the visual part's inner edge; C2 wins.
13. **Current marker.** C2's marker base (r 91) would touch the 12-unit thermostat stroke. It is moved out 5 units with CSS `translate`; no markup change.
14. **History series format** is M0's `{points:[[t,v|null]], source}`; the sparkline receives `points.map(p => p[1])` (C2 `set` keys `points, w, h, summary`).
15. **Knob remote settle.** Visual §2.2 (320 ms) vs H3 (220 ms) for Tactile and Instrument: 220 ms.
16. **H8 end dot** reads as radius 3 (C2 `r=3`), halo r 6.
17. **Minimal needle timing.** 420 ms on the standard curve, no overshoot.
18. **Lab gaps proposed to M0** (worked around in M1 test code only): focus marking for widget cells without `.hd-more`; a pre-probe hook instead of the `probes` wrapper; reduced-motion emulation for `motion=full`.

### Key files

- `C:\Users\Andi\Documents\repo\AuraGo\docs\superpowers\plans\2026-11-23-homedash-m0-frontend.md` (C1–C6, Tasks 8–16)
- `C:\Users\Andi\Documents\repo\AuraGo\docs\superpowers\plans\2026-11-23-homedash-m0-frontend-base.md` (saved code referenced by M0)
- `C:\Users\Andi\Documents\repo\AuraGo\docs\superpowers\specs\2026-11-23-homedash-design.md`
- `C:\Users\Andi\Documents\repo\AuraGo\docs\superpowers\specs\2026-11-23-homedash\design-visual-system.md`
- `C:\Users\Andi\Documents\repo\AuraGo\ui\i18n_lint_test.go`
- `C:\Users\Andi\Documents\repo\AuraGo\ui\css\desktop-materials.css`, `desktop-materials-amiga.css`, `desktop-materials-green-slime.css`, `desktop-theme-minimal.css`
- `C:\Users\Andi\Documents\repo\AuraGo\ui\fonts\fonts.css`

## Change log

- **S1** Lab through `window.hdLab` (fixtures, scenarios, `report`). Changed: File structure, Task 0 (hero-fixtures.js), Review protocol "How the lead looks".
- **S2** Own Go sheet expansion removed. The M1 lists and entries go into `ui/testdata/homedash-lab-sheets.json`. The colour-vision test reads and writes `../reports/homedash-lab`. Changed: File structure, Task 0, Commands.
- **S3** C2 names everywhere. Task 1 holds the single additions table. Changed: Hero build sheets (all), Task 1, Self-review 1, 12–14, 16.
- **S4** All CSS guard-clean: literals moved into T1–T4 tokens (`--hd-t-shade`, `--hd-t-needle-shadow`, `--hd-g-edge-u`, `--hd-m-arc-w` and others); widths, radii and shadows through tokens. Changed: Token and recipe sheet, Hero build sheets.
- **S5** Single swept value arcs; buffers only for the H4 active segment. Instrument segments use a gap overlay; 200-unit geometry with CSS-rotated groups. Changed: H3, H4, H6, Task 1 table, Risks, Self-review 7.
- **S6** Every read token is set: `--hd-glass-blur`, `--hd-g-bezel-r`, `--hd-g-paddle-r`, `--hd-t-pointer`, masks, shared defaults, `--hd-led` on the widget with state rules, `--hd-light-on` unconditional under the light scheme. The blur is on `.hd-frame` under Glass and the gradient backdrop. Changed: T0–T5.
- **S7** Family, tone, status and history from M0's host. Alerts through the view `tone` and `api.setTone`; `hd_alert` removed. Changed: Widget module contracts, Task 1, Task 24, fixtures, Self-review 11.
- **S8** Task 1 creates `ui/lang/homedash/` (16 locales), adds the lint case, lifts M0's literal-key rule, and lets the lab load the section words. Changed: Conventions, Files, Task 1.
- **S9** Placeholders closed: the Instrument glow is defined through the arc dasharray (no "same as lead"). Every task has its own Files, steps, commands and Done when. Changed: Tasks.
- **S10** Theme variants as `{theme, scheme}` (M0 lists; new `minimal2`); custom palettes via `custom3`; the easing strip via `"ease": "fallback"`. Changed: Task 0 fragment.
- **S11** Amendments folded: arcs sweep (A1); Glass blur judged on `hero-glass~gradient` with rubric G7 (A2); alert path through the tone (A3); the M0 interface and repository-root output (A4). The "Lead amendments" block is removed. Changed: Rubric: family, Commands, Tasks 5 and 8, Risks.
