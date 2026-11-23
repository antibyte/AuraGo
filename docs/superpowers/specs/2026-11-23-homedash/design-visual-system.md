# HomeDash visual system design

> Part of the HomeDash design. Where this part differs from section 4 of `../2026-11-23-homedash-design.md`, that document wins. This part was harmonised with it on 2026-11-23.

Status: design input for the lead, 2026-11-23. Read-only research; nothing was modified. "R" means `C:\Users\Andi\Documents\repo\AuraGo`. Values marked UNVERIFIED were not confirmed in code or need a measurement in the lab.

---

## 0. Verified repository constraints (a–g)

### 0.a Materials adoption guard (`ui/desktop_app_materials_test.go`)

- **What it demands.** `TestDesktopAppMaterialsAdoptedByApps` (:680-708) reads every `/css/*.css` path in `commonAppStyles` and `DESKTOP_APP_ASSETS` of `ui/js/desktop/core/module-loader.js` (:650-675; `commonAppStyles` is at module-loader.js:16). It requires each file on its own to contain the substring `--vd-mat-` (:694). If a file does not, it must be listed in `desktopAppMaterialsExempt` (:624-643) with a reason. Two more cases fail: an exempt file that reads the tokens (:699-700), and an exempt entry that no app loads any more (:703-706).
- **Exemption list (:624-643).** Runenpfad, xterm, cm-chessboard, both highlight.js themes, Radio, TeeVee, RTL-SDR, Galaxa, SIP phone, Minigolf, Aura Drift, Retro Games, Pinball, HA Switchboard ("walnut and brass … cabinet"), go2rtc-source-form ("no painted region"), Quick Connect ("console deck tinted by the theme accent"), theme-optics.
- **Path pattern trap.** `desktopAppStylesheetPattern = '(/css/[A-Za-z0-9_.\-]+\.css)'` (:645) has no `/` in the file part. Stylesheets in a sub-folder such as `/css/homedash/x.css` are never checked. HomeDash must not take advantage of this: its stylesheets live flat in `/css/`.
- **Line and size limits apply to material files only.** The 260-line limit (:18, enforced at :323-325) and the 4096-byte data-URI limit (:21, :374-376) apply to `desktopMaterialFileViolations`, which runs only on `css/desktop-materials.css` and `css/desktop-materials-<id>.css` (:438-458). No line limit for app CSS was found. The 1000-line limit at `desktop_theme_css_contract_test.go:18` applies to skins `desktop-theme-<id>.css` only.
- **Naming traps.** `filepath.Glob("css/desktop-materials-*.css")` fails for any file that belongs to no registry theme (:460-468). The bundle test also rejects stray `ui/css/desktop-materials*` parts (:509-512). Never name a HomeDash file `desktop-materials-*` or `desktop-theme-*`.
- **`[data-theme]` rule.** `documentation/desktop-themes.md:261`: "App CSS and JS do not select on `[data-theme…]` and do not use a theme id. Do not add `!important`." Other adoption rules: colour maths only on `-color` tokens (:256); semantic colours keep their own colours (:259); surface tokens only in the `background:` shorthand, no `background-size`/`-repeat` after them (:260). The theme-independent skins break this rule on purpose with the attribute-presence selector `.desktop-body[data-theme] .vd-window[data-app-id="…"]`, which outranks the skins and lets them own the window frame: ha-switchboard.css:2, radio.css:3, teevee.css:1-4 ("deliberately independent of Desktop themes"). HomeDash does **not** take over its window frame, so it needs no such selector.

### 0.b CSS lint rules: global vs. per app

| Rule | Scope | Source |
| --- | --- | --- |
| No remote `@import`/`url(//…)`; no Google Fonts or analytics hosts; no remote `<link>`/`src` | **Global**, every shipped .css/.html/.js | `ui/external_assets_test.go:12-22, 57-71` |
| Every skin selector rooted at its theme; keyframes animate only transform/opacity; 1000 lines | Skins only | `desktop_theme_css_contract_test.go:14-30` |
| Material files: custom properties only, no motion, `url()` only `/fonts/` or `data:image/svg+xml` ≤ 4096 B, `@font-face` names start with "Desktop Theme " | Material files only | `desktop_app_materials_test.go:312-389` |
| Marker strings must stay | Listed files only (HomeDash not listed) | `desktop_theme_consistency_test.go:59-182` |
| No hex or literal colour functions outside `color-mix()`, no named colours, no `[data-theme`, no `cc-`, no uppercase/monospace labels, motion only inside `@media (prefers-reduced-motion: no-preference)` on selectors containing `:not([data-animations="false"])` | **Docker Manager only** (and Infra) | `scripts/test-docker-manager-ui.mjs:2062-2115`; `scripts/test-infra-ui.mjs:1747, 1761`; summary in `ui/js/desktop/apps/AGENTS.md:3756-3760` |
| Never use the `cc-` prefix | **Global** project rule (docs only) | `ui/AGENTS.md:18-19` |

**Consequence.** HomeDash brings its own Node guard, `scripts/test-homedash-ui.mjs`, modelled on the Docker Manager guard but adapted to rich materials (rules in §9.4). Literal colours, gradients, shadows and SVG are allowed only as values of `--hd-*` custom properties inside named token blocks. Painting declarations use `var()` and `color-mix()` only.

### 0.c How a theme-independent skin app is built

ha-switchboard.css:1-11 sets its whole palette on `.desktop-body[data-theme] .vd-window[data-app-id="ha-switchboard"]`, restyles the titlebar (:12-63) and uses raster textures (`/img/ha-switchboard/walnut.png`, :3) and literal hex everywhere. Radio (radio.css:3-22) and TeeVee (teevee.css:4, :48) follow the same pattern. They are exempt in the guard, and `desktop_theme_consistency_test.go:54-56` even forbids TeeVee selectors in the shared bridge.

Quick Connect is the closer precedent for Instrument: a dark deck whose only theme input is `--vd-accent` (AGENTS.md:3778-3784).

Decision: HomeDash is **not** an exempt skin. Its stylesheet bundle contains the bridge, which reads `--vd-mat-*`. Tactile and Instrument draw their own hardware materials *inside widgets*, the way Calculator or Terminal keep dark readout surfaces (AGENTS.md:3785-3786).

### 0.d Contrast rules and status tokens

- `desktop_theme_token_contrast_browser_test.go:33-78` measures, for every theme variant and accent preset:
  - the four status tokens (`--ds-color-success|warning|danger|info-fg`, :45) as text on `--vd-theme-app-bg`, `--vd-theme-panel-bg` and `--vd-theme-panel-bg-strong` (:54-67), at 4.5:1;
  - `--vd-accent-contrast` on `--vd-accent` (:68-72), at 4.5:1.
- Material pairs at 4.5:1 (`desktop_app_materials_test.go:54-74`):
  - `--vd-text` on canvas, bar, side, card, well, field and btn colours;
  - `--vd-theme-muted` on canvas, bar, side and card colours only (**not on well**);
  - btn-text/btn, primary-text/primary, selected-text/selected;
  - heading and label colours on canvas and card.
- **Gap.** Status colours are **not** tested on `--vd-mat-card-color`. HomeDash cards sit on that colour, so the HomeDash lab must measure status-as-text itself (§8).
- Default status colours: success `#4ad29c`, warning `#f2b84b`, danger `#ff7a72`, info `#5aa9ff` (desktop-base.css:103-106). Each skin overrides them.

### 0.e Bundled fonts (no external fonts)

- `ui/fonts/fonts.css`:
  - Geist 300/400/500/600/700 (:1-40), Geist Mono 400/500/700 (:42-65), Datatype (:67-76), Darker Grotesque 400-700 (:78-184), Exo 2 400-700 (:186-365);
  - Inter 300-700 with Cyrillic, Greek and Latin-ext subsets (:367-680), Oxanium 400-700 Latin and Latin-ext (:684-755; every weight points to the same file, probably a variable font, UNVERIFIED);
  - Special Elite, Schoolbell, Shadows Into Light Two, Press Start 2P (:757-813).
- `ui/fonts/jetbrains-mono.css:1-25`: JetBrains Mono 400 and 600, kept out of fonts.css on purpose.
- Barlow Condensed SemiBold (TTF) and Share Tech Mono (TTF) exist under `/fonts/` but are declared only app-locally: `pixel-text3d.css:3-41`, `galaxa-deluxe.css:1`, and Orbit as `"Desktop Theme Barlow Condensed"` (desktop-theme-orbit.css:36-41). HomeDash therefore declares its own faces as `"HomeDash Barlow Condensed"` and `"HomeDash Share Tech Mono"`.

| Family | UI / labels | Numerals | Notes |
| --- | --- | --- | --- |
| Glass | Inter 400/500 (`"cv11"`, `"ss03"` off) | Inter 300 (hero), 500 (small), `font-variant-numeric: tabular-nums` | Inter covers Greek and Cyrillic (fonts.css:367-680) |
| Tactile | "HomeDash Barlow Condensed" 600 for engraved legends, letter-spacing .06em; Inter fallback | Barlow 600 on dial faces; Geist Mono 500 in the VFD readout window | Barlow is Latin only, so non-Latin labels fall back to Inter 600 |
| Minimal | Geist 400/500 | Geist 300 at hero sizes, 500 below 24 px; tabular-nums | Swiss, the most typographic family |
| Instrument | Oxanium 500 (labels, headings) | SVG seven-segment digits (hero); "HomeDash Share Tech Mono" (secondary readouts) | No DSEG font is bundled, so segments are drawn (§3) |

Tabular figures in Inter, Geist, Oxanium and Barlow are UNVERIFIED for these subset files. Lab check: with tnum on, `"1111"` and `"8888"` must have equal width. Fallback: Geist Mono or JetBrains Mono for every changing readout.

### 0.f Current quality baseline (to exceed clearly)

- **HA Switchboard switch** (ha-switchboard.js:37-50, :125; css :85-101):
  - A raster PNG atlas with three poses cross-faded by opacity (`transition: opacity 160ms ease`, :87). There is no physical travel.
  - Every switch carries `filter: saturate(.22) drop-shadow(...)`, and hover adds a `brightness()` filter (:86, :91), which costs a repaint per hover.
  - The CSS fallback is crude, with `ridge` borders (:93-95).
  - Lamp colours are hard-coded (:99-102).
  - The needle uses `350ms ease` with no inertia (:117).
  - Motion is gated only by `prefers-reduced-motion: reduce` (:184), not by the desktop animation switch.
  - Buttons stay `disabled` until data arrives.
- **Docker Manager charts** (docker-manager-charts.js:53-105):
  - Sparklines are straight polylines with `preserveAspectRatio="none"`, saved by `vector-effect: non-scaling-stroke` (desktop-app-docker-manager.css:470). There is no curve, no end point and no scrub.
  - The "gauge" is a 4 px bar (:99).
  - Solid, token-correct and accessible (`role="meter"`, :101-103), but utilitarian.
- **System Info** (system-info.js:58-87): a Chart.js canvas half-doughnut with a hard-coded track `rgba(255,255,255,0.08)` that disappears on light themes, an ungated `animation: {duration: 260}`, and no ticks, zones or numerals on the dial.
- **Sysmon widget** (widget-sysmon-runtime.js:71-73): a stretched SVG path sparkline.

HomeDash must beat all of these on curve quality, physical motion, theme correctness, state legibility and crispness.

### 0.g How the 14 themes differ (relevant to the bridge)

| Theme | Scheme | Card colour / radius | Primary (on) | Risk for controls |
| --- | --- | --- | --- | --- |
| standard | dark | `#1a212b` / 10 | accent + 20% white, ink `#0c1016` | user accent |
| fruity / fruity-dark | light / dark | `#ffffff`, `#2a2a2d` / 12 | accent + 20% white, ink `#0b1220` | **very light**, user accent |
| amiga | light (grey) | `#BBBBBB` / **0** | `#6688BB`, black text | **retro**: VGA heading font, 20 ms motion (theme-runtime.js:8), `:focus-visible { outline: 2px dashed #000 !important }` (desktop-theme-amiga.css:167) |
| cyberpunk | dark | `#0C1328` / 0, cut-corner decorations in `--vd-mat-card` (materials-cyberpunk.css:26) | `#F3E600` | **saturated** neon |
| black_gloss | dark | `#18181C` / 12 | raw accent + `--vd-accent-contrast` | user accent, glow |
| crazy | dark | `#2E0D63` / 18 | `#FF4FA8` | **most saturated** (violet, pink, green) |
| lollipop | light | `#FFFFFF` / 18 on a pink canvas `#FFE9F4` | `#B5145F` | **light and tinted** |
| organic | dark | `#4F3320` / 18, brass studs in `--vd-mat-card` (materials-organic.css:37-40) | `#DFB458` | serif headings |
| electronic_dreams | dark | `#1A2B26` / 8 | `#E6A83A` | — |
| windows_xp | light | `#FAFBFE` / 3 on cream | `#1D5FAF` | **light**, retro |
| green_slime | dark | `#26321A` / `16px 19px 14px 20px` (materials-green-slime.css:36) | `#A5D941` | **4-value radius** breaks `calc()` |
| voxel_world | light | `#D0D0D0` / 0 on parchment | `#476B3C` | **pixel**, radius 0 |
| orbit | dark | `#121B38` / 6 | `#5CE1FF` | — |
| minimal / minimal-dark | light / dark | `#FFFFFF`, `#202020` / 8 | `#161616`, `#EDEDED` (monochrome) | the accent is ink, so "on" is not coloured |

Themes most likely to break a control:

- **Light:** Fruity light, Minimal light, Lollipop, Windows XP, Voxel World. Shadows must stay soft, and a white thumb on a white card disappears.
- **Mid-grey:** Amiga. Both glass highlights and shadows go flat.
- **Saturated:** Crazy, Cyberpunk, Green Slime. The accent fights the status colours.
- **Pixel/retro:** Amiga, Voxel World, Windows XP. Round, soft controls look foreign at radius 0.

Two bridge rules follow from the table:

- **Never use the decorated `--vd-mat-card` as a widget background.** It would repeat studs and cut corners on every tile. Use only `-color`, `-radius`, `-border` and `-shadow`.
- **Never `calc()` on `--vd-mat-card-radius`.** Pass it through whole.

---

## 1. Token architecture `--hd-*`

### 1.1 Ownership

- **P = palette** sets it (colours).
- **F = family** sets it (geometry, material, light, motion; may derive from P with `color-mix()`).
- **D = dashboard look** (density, radius scale, backdrop).
- **W = widget may override** (inline custom property on `.hd-w`).

| Group | Tokens | Owner |
| --- | --- | --- |
| Backdrop | `--hd-canvas` (opaque), `--hd-canvas-2` (gradient partner), `--hd-canvas-fill` (full background value), `--hd-scrim` (colour), `--hd-scrim-alpha` | P (+D) |
| Surfaces | `--hd-surface-solid` (opaque, used for contrast maths), `--hd-surface` (painted, may be translucent), `--hd-surface-raised` (popover), `--hd-sunken` (tracks, readout windows), `--hd-field` | P solid, F painted |
| Ink | `--hd-ink`, `--hd-ink-muted`, `--hd-ink-faint` (decoration only, never text), `--hd-ink-on-accent`, `--hd-on-ink`, `--hd-selected-ink`, `--hd-btn-ink` | P |
| Accent | `--hd-accent`, `--hd-accent-soft` (= mix 16%), `--hd-accent-2`, `--hd-on` (lit/on fill), `--hd-selected`, `--hd-btn` | P, **W** (`--hd-accent`) |
| State | `--hd-ok`, `--hd-warn`, `--hd-alert`, `--hd-info`, `--hd-pending` (= warn), `--hd-unavailable` (= ink-faint), plus `-ink` text versions (`--hd-alert-ink` …) | P |
| Domain | `--hd-heat` (heating), `--hd-cool` (cooling), `--hd-ct-2200`, `--hd-ct-4000`, `--hd-ct-6500` (colour-temperature stops), `--hd-light-on` (warm lamp glow) | P, **W** |
| Track / thumb | `--hd-track`, `--hd-track-fill` (default `var(--hd-on)`), `--hd-thumb`, `--hd-thumb-edge` | F |
| Light | `--hd-edge-tint` (highlight colour), `--hd-shadow-ink` (shadow colour), `--hd-edge-hi`, `--hd-edge-lo`, `--hd-hairline`, `--hd-glow` (box-shadow value or `none`) | P colour, F recipe |
| Shadows | `--hd-shadow-contact`, `--hd-shadow-card`, `--hd-shadow-pop`, `--hd-shadow-press` | F |
| Radii | `--hd-radius-card`, `--hd-radius-control`, `--hd-radius-pill` (9999px), `--hd-radius-scale` (D, 0.5–1.5; applied only to family radii, never to theme radii) | F, D |
| Strokes | `--hd-stroke-hair` (1px), `--hd-stroke-1` (1.5px), `--hd-stroke-2` (2px), `--hd-stroke-arc` (fraction of dial diameter, e.g. `0.075`), `--hd-stroke-tick` (1.25px) | F |
| Spacing | `--hd-unit` (4px × density factor), `--hd-space-1..8` = 1,2,3,4,5,6,8,12 × unit, `--hd-pad-card` | D |
| Type | `--hd-font-ui`, `--hd-font-label`, `--hd-font-num`, `--hd-font-heading`, `--hd-num-features`, sizes `--hd-text-xs 11px`, `-s 12.5px`, `-m 14px`, `-l 17px`, `-xl 21px`, numeral sizes in `cqmin` (§2), `--hd-label-tracking` | F |
| Motion | `--hd-dur-press 90ms`, `--hd-dur-quick 140ms`, `--hd-dur-state 220ms`, `--hd-dur-value 420ms`, `--hd-dur-needle 1100ms`, `--hd-ease-standard cubic-bezier(.2,0,0,1)`, `--hd-ease-out cubic-bezier(.16,1,.3,1)`, `--hd-ease-spring cubic-bezier(.34,1.45,.64,1)`, `--hd-ease-snap cubic-bezier(.6,0,.25,1.25)`, `--hd-ease-needle` (below) | F |
| Focus | `--hd-focus`, `--hd-focus-gap` (= surface-solid) | P |

Damped needle spring (ζ = 0.67, about 6% overshoot, settles within 1100 ms), computed from the closed-form spring and sampled:

```css
--hd-ease-needle: linear(0, .038 4.2%, .135 8.3%, .262 12.5%, .402 16.7%, .664 25%, .862 33.3%,
                         .984 41.7%, 1.042 50%, 1.059 58.3%, 1.052 66.7%, 1.037 75%, 1.021 83.3%, 1);
/* @supports not (transition-timing-function: linear(0, 1)): cubic-bezier(.3, 1.3, .5, 1) */
```

### 1.2 Scoping selectors

Data attributes are namespaced `data-hd-*` so they never collide with the desktop's `data-theme` and `data-scheme`.

```
.hd-root                                         defaults + neutral sets --hd-n-* (dark and light)
.hd-root[data-hd-scheme="dark"|"light"]          which neutral set backs the fallbacks
.hd-root[data-hd-palette="theme"]                bridge (§1.3); outside the desktop resolves to --hd-n-*
.hd-root[data-hd-palette="dark"|"light"]         neutral palettes, --hd-* = --hd-n-*
.hd-root[data-hd-palette="custom"]               JS writes every P token inline after a contrast solve (§1.5)
[data-hd-family="glass|tactile|minimal|instrument"]   on .hd-root AND on any .hd-w with a family override
.hd-w { --hd-accent: … }                         per-widget accent override, inline style, from the document
```

- Family token blocks are declared on `[data-hd-family=…]`. A widget override re-resolves the F tokens against the inherited P tokens.
- `data-hd-scheme` is set by the host. The desktop host copies `document.body.dataset.scheme`, as AGENTS.md:3769 requires (apps read the scheme from the dataset, never from the theme id). The kiosk host uses the kiosk setting or `prefers-color-scheme`.
- `.hd-root` sets `color-scheme: dark|light` from `data-hd-scheme` for native form controls. The engine does not use `light-dark()`; scheme-dependent values are selected with `[data-hd-scheme]`.

### 1.3 Theme bridge (`palette = theme`)

```css
.hd-root[data-hd-palette="theme"] {
  --hd-canvas:        var(--vd-mat-canvas-color, var(--hd-n-canvas));
  --hd-canvas-fill:   var(--vd-mat-canvas, var(--hd-canvas));            /* backdrop "theme" may show the pattern */
  --hd-surface-solid: var(--vd-mat-card-color, var(--hd-n-surface));
  --hd-sunken:        var(--vd-mat-well-color, var(--hd-n-sunken));
  --hd-field:         var(--vd-mat-field-color, var(--hd-n-sunken));
  --hd-ink:           var(--vd-text, var(--hd-n-ink));                   /* tested 4.5 on canvas/card/well/field */
  --hd-ink-muted:     var(--vd-theme-muted, var(--hd-n-ink-muted));      /* tested on canvas/card only: no muted text in wells */
  --hd-hairline:      var(--vd-mat-divider, var(--hd-n-hairline));
  --hd-accent:        var(--vd-accent, var(--hd-n-accent));
  --hd-ink-on-accent: var(--vd-accent-contrast, var(--hd-n-ink-on-accent));
  --hd-on:            var(--vd-mat-primary-color, var(--hd-n-on));       /* tested pair with -primary-text */
  --hd-on-ink:        var(--vd-mat-primary-text, var(--hd-n-on-ink));
  --hd-selected:      var(--vd-mat-selected-color, var(--hd-n-selected));
  --hd-selected-ink:  var(--vd-mat-selected-text, var(--hd-n-selected-ink));
  --hd-btn:           var(--vd-mat-btn-color, var(--hd-n-btn));
  --hd-btn-ink:       var(--vd-mat-btn-text, var(--hd-n-btn-ink));
  --hd-accent-2:      var(--vd-mat-accent-2, var(--hd-n-accent-2));
  --hd-ok:    var(--ds-color-success-fg, var(--hd-n-ok));   --hd-warn: var(--ds-color-warning-fg, var(--hd-n-warn));
  --hd-alert: var(--ds-color-danger-fg, var(--hd-n-alert)); --hd-info: var(--ds-color-info-fg, var(--hd-n-info));
  --hd-focus: var(--ds-color-border-focus, var(--hd-accent));
  --hd-theme-radius: var(--vd-mat-card-radius, 14px);      /* passed whole: Green Slime has 4 values */
  --hd-theme-border: var(--vd-mat-card-border, transparent);
  --hd-theme-edge:   var(--vd-mat-card-shadow, none);
  --hd-theme-glow:   var(--vd-mat-glow, none);
  --hd-font-heading: var(--vd-mat-heading-font, var(--hd-font-ui));
}
```

Status text uses the same mix as Docker Manager, selected by scheme instead of `light-dark()`: under `[data-hd-scheme="light"]`, `--hd-alert-ink: color-mix(in srgb, var(--hd-alert) 34%, var(--hd-ink))`; under `[data-hd-scheme="dark"]`, `--hd-alert-ink: var(--hd-alert)`. The same holds for ok, warn and info.

What each family takes from the bridge:

- **Glass** takes surfaces, ink, accent, theme radius, border and edge ("theme-native glass": Amiga becomes bevelled grey, Organic gets a brass border).
- **Minimal** takes surfaces, ink and theme radius; no shadows.
- **Tactile and Instrument** take **only** accent, `on`, status and focus. Their surfaces are their own (§2).

### 1.4 Neutral palettes (kiosk, and fallbacks)

| Token | Dark | Light |
| --- | --- | --- |
| canvas / canvas-2 | `#0E1116` / `#171C26` | `#EEF0F4` / `#E2E6ED` |
| surface-solid / raised / sunken | `#1A1F29` / `#222834` / `#0A0D12` | `#FFFFFF` / `#FFFFFF` / `#E7EAF0` |
| ink / muted / faint | `#F1F4F9` / `#A3ADBD` (7.3:1 on surface) / `#5E6878` | `#12161D` / `#4D5666` / `#9AA2AF` |
| accent / on / on-ink | `#7AA7FF` / `#8FBFFF` / `#0B1220` | `#2F6FE0` / `#2F6FE0` / `#FFFFFF` (4.7:1) |
| ok / warn / alert / info | `#4AD29C` / `#F2B84B` / `#FF7A72` / `#5AA9FF` | `#1A7D52` (5.1) / `#9A6200` (5.1) / `#C4352B` (5.4) / `#1F63C7` |
| heat / cool | `#FF9A57` / `#5CC8FF` | `#D9622B` / `#1E8FCB` |
| ct-2200 / 4000 / 6500 | `#FFA552` / `#FFE2B8` / `#E4EEFF` | same |
| edge-tint / shadow-ink | `#FFFFFF` / `#000000` | `#FFFFFF` / `#1B2333` |

The light ratios in parentheses are hand-computed against `#FFFFFF`. The lab must re-measure them.

**Theme palette on the kiosk page.** When a dashboard uses the `theme` palette, the designer stores a snapshot of the resolved palette tokens (hex colours) in `look.palette.snapshot` on every save. The kiosk page applies the snapshot as custom properties, so a wall display shows the colours of the desktop theme the user designed with. Decorated theme materials (patterns, studs) are not part of the snapshot. Without a snapshot the neutral palette is used.

### 1.5 Custom palettes

The user picks an accent (hex), a surface hue (0–360) and light/dark. **Derived colours are computed in JS** (OKLCH), deterministically at render time from the stored inputs, and written inline on `.hd-root`. CSS cannot measure contrast, so CSS `color-mix()` is used only for non-critical washes (hover 8%, soft 16%, glow).

Solver:

1. surface-solid: L 0.22 (dark) or 0.985 (light), chroma 0.012 at the chosen hue. canvas: ∓0.04 L. sunken: ∓0.07 L.
2. ink: L 0.96 or 0.18. muted: lower the contrast until it reaches 5.0:1 on surface-solid **and** 4.5:1 on canvas.
3. on = accent. on-ink = black or white, whichever is higher. If that is below 4.5:1, step the lightness of `on` by 0.02 until it reaches 4.5:1.
4. The accent as a non-text graphic must reach ≥ 3:1 against surface-solid (WCAG 1.4.11). Otherwise adjust its lightness.
5. Status colours start from the neutral set and their lightness is adjusted to 4.5:1 on surface-solid.

A Node test runs 500 random inputs and asserts every pair.

---

## 2. The four families

Shared rule: a state is never shown by colour alone. Every family combines **position** (thumb, needle, paddle), **shape or glyph** (I/O, check, triangle, dash) and **text** in the status line.

### 2.1 Glass: "layered translucent glass lit from above that takes the theme's colours"

- **Light model.** Soft overhead light. Depth comes from: top inner highlight 1px, bottom inner shade 1px, a contact shadow and a long soft ambient shadow. Pressed elements lose the ambient shadow and gain an inset shade.
- **Surface recipe** (card):

```css
--hd-glass-alpha: 72%;       /* dark min 62%, light min 70% (§8) */
--hd-glass-blur: 18px;       /* only when the backdrop is gradient */
--hd-surface: color-mix(in srgb, var(--hd-surface-solid) var(--hd-glass-alpha), transparent);
.hd-card[data-hd-family="glass"] {           /* illustrative; the selector form is in §9 */
  background: linear-gradient(180deg, color-mix(in srgb, var(--hd-edge-tint) 9%, transparent), transparent 46%),
              var(--hd-surface);
  border: 1px solid color-mix(in srgb, var(--hd-edge-tint) 14%, var(--hd-theme-border));
  border-radius: var(--hd-theme-radius);
  box-shadow: inset 0 1px 0 color-mix(in srgb, var(--hd-edge-tint) 20%, transparent),
              inset 0 -1px 0 color-mix(in srgb, var(--hd-shadow-ink) 14%, transparent),
              0 1px 2px color-mix(in srgb, var(--hd-shadow-ink) 14%, transparent),
              0 14px 32px -10px color-mix(in srgb, var(--hd-shadow-ink) 32%, transparent);
}
.hd-root[data-hd-backdrop="gradient"] { --hd-glass-on: 1; }
/* backdrop-filter: blur(18px) saturate(1.35) only on .hd-glass-pane and only under these two backdrops */
```

  In light schemes the shadow alphas are ×0.6, so shadows never turn into a grey smudge on Fruity, Lollipop or Minimal.
- **Geometry.** Card radius = theme radius (fallback 16px). Control radius = pill. Strokes: arcs 7.5% of diameter, round caps. Thumb diameter = track height − 6px.
- **Type.** Inter. Label 12.5px/500, muted. Value 300. Hero numeral `clamp(44px, 30cqmin, 112px)`, letter-spacing −0.02em. Unit at 0.42× hero size, 400 weight, muted, baseline-aligned. Headings use `--hd-font-heading` (theme-native: Georgia in Organic, Oxanium in Cyberpunk).
- **States.**
  - ON: track filled `--hd-on` with an I glyph on the left at 55% on-ink, and a 1px inner top light.
  - OFF: track = ink 14% wash with an O ring glyph on the right.
  - PENDING: a conic accent sweep ring on the thumb.
  - UNAVAILABLE: 1px dashed hairline outline, no fill, hollow thumb, content opacity 0.5, en dash value.
  - ALERT: a 2px inset ring in `--hd-alert` on the card plus a triangle badge.
- **Motion.** Thumb `transform` 280ms `--hd-ease-spring` (≈ 1.5px overshoot). Press: thumb widens to 1.18× (scaleX, origin on the travel side) in 90ms. Value arcs: `stroke-dasharray` 420ms `--hd-ease-out`. Glow: opacity of a pre-rendered `::after` shadow, 220ms.
  - Reduced motion: every duration 0, states change instantly, no ambience.
- **Never:** blur stacks (glass inside glass), text on a blurred image without the scrim (§8), a rainbow or multi-hue gradient, glass alpha below 0.62, a thumb that is white on a white card (light schemes give the thumb a 0.5px `--hd-shadow-ink` 22% edge plus a contact shadow).

### 2.2 Tactile: "believable, quiet hardware from one well-made appliance"

- **Light model.** Key light from the top-left at 45°, fixed for every widget, so all highlights sit top-left and all shadows bottom-right. Depth comes from bevel highlights, inner recess shadows and real cast shadows (y-offset 2–6px, blur 4–12px).
- **Materials.** Selected by scheme. Each widget may use at most one panel material plus one control material.

| Material | Dark scheme | Light scheme |
| --- | --- | --- |
| Panel | anodised graphite `#2A2E35` → `#1F2228`, vertical 180° gradient, 2px brushed grain | brushed aluminium `#D9DDE3` → `#BFC4CC` |
| Control | black bakelite `#1C1A18` with an elliptical specular at 30%/20% (`#FFFFFF` 14% → 0 at 55%) | ivory bakelite `#ECE6D8` with a specular at 22% |
| Metal (knob caps, rims) | satin steel: conic sheen | satin steel: conic sheen |
| Readout window | black glass `#07090B`, inset 0 2px 6px black 70% | same |
| LED | 5mm dome, colour from state | same |

  Recipes, with literals living only in the Tactile token block:

```css
--hd-t-brush: repeating-linear-gradient(90deg, color-mix(in srgb, var(--hd-edge-tint) 5%, transparent) 0 1px, transparent 1px 3px);
--hd-t-steel: conic-gradient(from 18deg, #C7CCD4, #F3F5F8 11%, #A8AEB7 24%, #E4E7EC 37%, #979EA8 50%,
              #EFF1F4 63%, #B0B6BF 76%, #ECEFF3 89%, #C7CCD4);          /* anisotropic satin, no animation */
--hd-t-knurl: repeating-conic-gradient(#3A3E45 0 2.6deg, #5A606A 3.4deg 5.6deg, #3A3E45 6.4deg 7.2deg);
              /* soft 0.8deg transitions: Chrome aliases hard conic stops (UNVERIFIED per browser; check at DPR 1) */
--hd-t-led: radial-gradient(circle at 36% 30%, color-mix(in srgb, var(--hd-led) 35%, #FFFFFF) 0 16%,
            var(--hd-led) 46%, color-mix(in srgb, var(--hd-led) 55%, #000000) 100%);
```

- **Engraved legends.**
  - On aluminium: `color: color-mix(in srgb, #2B2F35 90%, var(--hd-t-panel))` with `text-shadow: 0 1px 0 color-mix(in srgb, #FFFFFF 50%, transparent)`.
  - On graphite: `#C9CDD3` with `text-shadow: 0 -1px 0 #000000 60%`.
  - Both must reach 4.5:1 against the panel's mid stop.
- **Geometry.** Card radius 12px × radius-scale (instrument housing). Control radius 6px (rockers) or full (knobs). Bezels 3–5% of the widget's short side. Dials use a 240° sweep.
- **Type.** Barlow Condensed 600 legends at 11–13px, letter-spacing .06em, no text-transform. Power symbols I/O (IEC 60417) instead of words on switches, so they are language-neutral. Dial numerals Barlow 600. Digital readouts Geist Mono 500 in a black window with `color: #FFC266` and `text-shadow: 0 0 6px #FFB54780` (amber VFD).
- **States.**
  - ON: paddle or lever in the on position plus a lit LED (green `--hd-ok` or accent).
  - OFF: LED dark (the LED colour mixed 22% into `#111`).
  - PENDING: LED blinks amber at 2 Hz (opacity 1 ↔ 0.25).
  - UNAVAILABLE: the panel gets a "covered" look (diagonal hatch, 6% ink), the LED is a hollow ring and the legend "—".
  - ALERT: a red LED with a 1px red rim around the bezel.
- **Motion.** Mechanical.
  - Toggle and rocker: 120ms `--hd-ease-snap`, a fast run with a hard detent stop and a 1–2px bounce.
  - Knobs follow the pointer 1:1 and settle remote changes in 320ms `--hd-ease-out`.
  - Needles use `--hd-ease-needle` for 1100ms.
  - LEDs switch in 60ms (incandescent warm-up is fake; no fade).
  - Reduced motion: positions jump, LEDs switch, the pending blink becomes a steady amber LED plus the "Sending" text.
- **Never:** wood, leather or photo textures; more than one rivet or screw pair, and none below 2×2 cells; brass and chrome together; drop shadows in two directions; legends below 10px; a knurl or brush period under 2 device px (moiré at DPR 1).

### 2.3 Minimal: "type and hairlines; nothing that does not carry information"

- **Light model.** None: flat, no shadows, no gradients. Depth comes only from a hairline and spacing.
- **Surface.** `background: var(--hd-surface-solid); border: 1px solid var(--hd-hairline); border-radius: var(--hd-theme-radius); box-shadow: none`. The "borderless" density option drops the border and uses 24px gaps.
- **Geometry.** Tracks 2px, thumbs solid, arcs 4% of diameter with butt caps, 1px ticks only where they carry scale information.
- **Type.** Geist. Hero numerals 300 at `clamp(52px, 36cqmin, 128px)`, letter-spacing −0.035em. Label 12.5px/500 muted, letter-spacing .01em. At most three text sizes per widget.
- **States.**
  - ON: control filled with `--hd-ink` and the thumb inverted to `--hd-surface-solid`. Monochrome: in Minimal the accent is reserved for focus and value arcs.
  - OFF: 1.5px ink outline.
  - PENDING: a hairline 270° ring rotating around the thumb (1.2s linear), plus "…" after the state word.
  - UNAVAILABLE: everything at ink-faint, value "—".
  - ALERT: the value turns `--hd-alert-ink` with a ▲ glyph before it.
- **Motion.** 160ms `--hd-ease-standard`, no overshoot, no ambience. Reduced motion: none at all.
- **Never:** shadows, gradients, icons larger than the numeral cap height, centred text blocks over 2 lines, coloured card backgrounds.

### 2.4 Instrument: "a calm dark technical HUD that glows only where data is"

- **Light model.** Self-luminous: there is no external light, only emitted light. Depth comes from darkness plus glow. It is always dark, even in light palettes, where it reads as an inset instrument panel.
- **Surface recipe.**

```css
--hd-i-panel: linear-gradient(180deg, #0B0F14, #07090C);
--hd-i-hud: color-mix(in srgb, var(--hd-accent) 85%, #FFFFFF);   /* lifts dark accents to ≥ 3:1 on the panel */
--hd-i-corners: linear-gradient(var(--hd-i-hud), var(--hd-i-hud)) 0 0 / 10px 1.5px no-repeat,
                linear-gradient(var(--hd-i-hud), var(--hd-i-hud)) 0 0 / 1.5px 10px no-repeat;   /* × 4 corners */
--hd-i-scan: repeating-linear-gradient(0deg, color-mix(in srgb, #000000 22%, transparent) 0 1px, transparent 1px 3px);
```

  The card has a 1px border of HUD 18%, corner brackets in HUD 70%, and radius 4px × scale. Scan lines appear **only** inside readout windows, at overlay opacity 0.35, never on the whole card.
- **Geometry.** Arcs are segmented (40 segments with 1.35° gaps). Ticks 1.25px. Butt caps. Angular cut corners allowed only on the readout window (8px chamfer via `clip-path: polygon`).
- **Type.** Oxanium 500 labels at 11–12px, letter-spacing .12em, no uppercase transform. Hero numerals are SVG seven-segment digits with ghost segments at 7%. Secondary readouts Share Tech Mono.
- **States.**
  - ON: key light bar at HUD 100% with glow `0 0 10px` HUD 55%.
  - OFF: HUD 22% outline.
  - PENDING: a segment chase (one lit segment travelling, 80ms per step).
  - UNAVAILABLE: "NO DATA"-style i18n text in ink-faint inside a dashed frame.
  - ALERT: the affected segments and readout switch to `--hd-alert`, with a 1Hz 0.6 ↔ 1 opacity pulse limited to 6 cycles, then steady.
- **Motion.** Snappy: 110ms `cubic-bezier(.2,.8,.2,1)` plus a 90ms flash (opacity 1 → 0.55 → 1) on change. Needles use `--hd-ease-needle` at 0.8× duration.
  - Reduced motion: no flash, no chase, no pulse. Pending becomes a static half-lit arc plus text.
- **Never:** more than one glow colour per widget, full-card scan lines, glitch effects, animated noise, cyan + magenta + yellow together, text under 11px.

---

## 3. Primitive components

Common to every primitive:

- Hit area ≥ 44×44px. Small visuals are padded with transparent hit padding.
- Focus ring: `box-shadow: 0 0 0 2px var(--hd-focus-gap), 0 0 0 4px var(--hd-focus)` on the interactive element. A box-shadow ring is used rather than `outline`, so Amiga's `!important` dashed outline only adds to it and never replaces it.
- Disabled: opacity 0.45, no hover, `aria-disabled="true"`.
- Hover (pointer: fine only): +6% edge-tint wash.
- Press: `--hd-shadow-press` and translateY(1px) for Tactile; scale(.97) for Glass; none for Minimal and Instrument (they use the flash).
- **"SVG draws geometry, CSS paints material."** Materials are layered CSS backgrounds on HTML elements. SVG holds strokes, ticks and needles with flat `fill`/`stroke: var(--hd-…)`. No SVG gradient defs are needed, so there are no id collisions between two dashboards in two windows. Ticks are **one `<path>`** with many subpaths. Arcs are `<path pathLength="100">` driven by `stroke-dasharray: calc(var(--hd-v) * var(--hd-sweep-frac)) 100`.

| Primitive | Anatomy / sizes | Glass | Tactile | Minimal | Instrument | Technique / cost |
| --- | --- | --- | --- | --- | --- | --- |
| Toggle (track + thumb) | M 52×32, L 64×38; thumb = h − 6 | §4 hero | slide switch in a recessed slot | outline → ink fill | latching key | CSS only; transform transition |
| Rocker | paddle 1:1.6, pivot centre | frosted paddle | §4 hero | two stacked halves | two stacked keys | rotateX ±9° with perspective 420px; cheap |
| Push button | M 44h, L 64h, tile | glass chip | domed bakelite cap | text + hairline | bracketed key | CSS |
| Slider H/V | track 6px (G) / 4px (T slot) / 2px (M) / segmented (I); thumb 24–28px | pill fill + white thumb | fader cap with grip lines | hairline + 12px dot | LED ladder (20 segments) | CSS width via `transform: scaleX` on the fill (no layout); pointer capture |
| Knob / dial with arc | D = 80–240px; arc 270° | ring with handle dot | knurled knob, steel cap, pointer line | thin arc + numeral | segmented arc + seg digits | SVG arc + CSS cap; conic knurl masked to a ring |
| Stepper | − value + ; buttons 44×44 | glass pills | two bakelite keys | text buttons | bracket keys | CSS; long press repeats at 400ms then 80ms |
| Segmented control | 2–5 segments, 36–44h | sliding glass puck | rotary mode selector (a knob with detents for ≤ 5 modes) | underline indicator | lit tab | `role=radiogroup`; puck translateX |
| Numeral readout | integer / decimal / unit / trend | Inter 300 | VFD window | Geist 300 | 7-segment SVG | fixed `min-width` in ch; tabular; no layout shift |
| Unit label | 0.42× numeral | muted | engraved | muted | HUD 60% | text |
| Sparkline | 2×1 to 4×2 widgets; line 1.75px | area gradient 22% → 0 | amber trace in a black window | 1.25px ink line, no area | HUD line plus a faint grid | monotone cubic (Fritsch–Carlson), gaps for null, end dot 3px, no `preserveAspectRatio="none"` (redrawn on resize) |
| Radial arc gauge | 270°, stroke 7.5% D | §4 hero | (uses the needle gauge) | §4 hero | segmented | dasharray transition |
| Needle gauge | 240°, ticks, zones, hub | (uses the arc) | §4 hero | thin needle, no face | luminous needle | needle = transformed `<g>`, `transform: rotate(var(--hd-angle))` |
| Linear bar / level / tank | h 8–12px or vertical tank | rounded fill | glass tube with meniscus | hairline + fill | segmented ladder | scaleX/scaleY |
| LED / state lamp | 10, 14, 20px | soft dot with halo | 5mm dome (`--hd-t-led`) | 8px dot or ring | square pixel with glow | glow = opacity of a pre-rendered `::after` |
| Icon badge | 24 grid, 1.75 stroke, 32/40/56 tiles | tinted circle | engraved | bare icon | bracketed | HomeDash's own line-icon set (§10) |
| Card frame / bezel | radius per family | glass | instrument housing | hairline | brackets | CSS |
| Popover sheet | anchored ≥ 600px wide, bottom sheet below | the **only** element with a second blur | steel-rimmed plate | flat + hairline | HUD panel | one popover at a time |
| Colour wheel / CT strip | wheel 200–260px; strip 12px | ring thumb | (Glass look inside a steel rim) | thin rim | HUD rim | wheel rendered **once** to a canvas at DPR (HSV); CT strip CSS linear-gradient of `--hd-ct-*` |

**Forbidden techniques** (lint plus review):

- `backdrop-filter` anywhere except `.hd-glass-pane` and `.hd-sheet`; blur > 24px; nested blur.
- Animated `filter`, animated `box-shadow`, animated `background-position` over large areas.
- `feTurbulence` at runtime; SVG filters on elements that change more often than once a second.
- `mix-blend-mode` on areas larger than 200×200px.
- Width, height or top animations.
- More than 1 infinite animation per widget (ambience counts at dashboard level only).

**Budget per widget:** ≤ 150 SVG nodes, `contain: layout paint style`, `container-type: size` on the widget body. Off-screen pages get `content-visibility: auto`.

---

## 4. Widget catalog (version 1)

Cell sizes are in grid cells w×h. The nominal cell is about 96px at "comfortable" density (UNVERIFIED: the grid spec decides).

| Type | Variants (v1) | Min / default | Primitives |
| --- | --- | --- | --- |
| Switch | pill, rocker, push, tile | 1×1 / 2×1; rocker 1×2 / 2×2; tile 1×1 / 2×2 | toggle, rocker, button, lamp, badge |
| Light / dimmer | slider-h, fader-v, dial-ring, tile (drag to dim) + CT/colour popover | 2×1 / 4×1; 1×2 / 1×3; 2×2 / 2×2; 1×1 / 2×2 | slider, knob, wheel, CT strip, sheet |
| Climate | dial, stepper, scale-v; mode selector in each | 2×2 / 3×3; 2×1 / 2×1; 1×2 / 1×3 | knob arc, stepper, segmented, numeral |
| Temperature | numeral (+ trend), thermometer, comfort ring (+ humidity) | 1×1 / 2×1; 1×2 / 1×2; 2×2 / 2×2 | numeral, bar, arc, spark |
| Sensor value | numeral, numeral + sparkline, icon tile | 1×1 / 2×1; 2×1 / 2×2; 1×1 / 1×1 | numeral, spark, badge |
| Gauge | radial arc, needle, linear bar, tank | 2×2 / 2×2; 2×2 / 2×2; 2×1 / 3×1; 1×2 / 1×2 | arc, needle, bar |
| Binary state | indicator (icon + state word), LED lamp | 1×1 / 2×1; 1×1 / 1×1 | badge, lamp |
| Cover / blind | position slider with shutter picture, up/stop/down | 1×2 / 2×2; 1×1 / 1×2 | slider-v, buttons |
| Scene / script | button | 1×1 / 2×1 | push button + success tick |
| Select / mode | segmented, list in sheet | 2×1 / 3×1; 1×1 / 2×1 | segmented, sheet |
| Number | stepper, slider | 2×1 / 2×1 | stepper, slider |
| Fan | speed segments + power | 2×1 / 2×2 | segmented, toggle, rotor icon |
| Lock | slide to unlock (+ confirm) | 2×1 / 3×1 | slider (locked track), lamp |
| Media player | compact, cover art | 2×1 / 4×1; 2×2 / 3×2 | buttons, slider, numeral |
| History chart | line/area, binary timeline | 3×2 / 4×2; 3×1 / 4×1 | own SVG, shares the sparkline maths; no Chart.js, for family consistency |
| Camera | snapshot/live frame | 2×2 / 4×3 | card frame only; media stays dark |
| Status list | 3–8 entities | 2×2 / 2×3 | lamp, numeral, badge |
| Battery / level | battery glyph + percent | 1×1 / 1×1 | bar |
| Room card | title, temp, humidity, 2–4 quick toggles | 2×2 / 3×2 | numeral, toggle, badge |
| Clock / date | digital, analogue | 1×1 / 2×1; 2×2 / 2×2 | numeral, needle |
| Heading | — | 1×1 / 4×1 | type |
| Navigation button | — | 1×1 / 1×1 | push button |
| Weather (HA weather entity) | compact, 5-day forecast | 2×1 / 2×1; 3×2 / 4×2 | numeral, own weather icon set (14 conditions) |
| Entity (fallback) | tile | 1×1 / 2×1 | badge, numeral, lamp |

Type and variant ids are fixed in section 4.3 of the main document.

**Variant count:** 47 variants × 4 families = 188 renderings.

- 32 are bespoke hero renderings (8 heroes × 4 families, specified below).
- About 110 are composed from primitives with family-specific layout tweaks.
- About 46 are frame-only (text, group, camera, nav, clock-digital, status list), where the family changes only the frame and type.

**Cut list (quality over count):**

- 3D/isometric room views, liquid-fill animations and animated weather scenes (static, designed icons instead).
- A Minimal "thermometer" bulb. In Minimal the thermometer becomes a hairline scale.
- Analogue clock in Instrument (it renders the digital clock).
- A second chart style per family.
- Per-widget backdrop images.
- Moved to v1.1: media "large art" layout, colour presets grid.
- Group frames and spacer widgets (a page can switch compaction off instead).

**Effort (honest):**

- Visual system plus control library: about 16 work packages (§11). Roughly 6 L, 8 M, 2 S, or about 30–40 maker-agent days including lab reviews and rework.
- The style lab alone (32 hero renderings plus revisions) is about 30% of that.

### 4.1 Hero specifications

Notation: D = dial diameter = min(body w, h). Angles: 0° = 12 o'clock, clockwise.

**H1 Toggle switch (pill).** Track 52×32 (M), ratio 1.625. Thumb ⌀ 26, travel 20px. Hit area 64×44.

- **Glass.**
  - Off track: `color-mix(ink 14%)`, inset `0 1px 2px shadow-ink 22%`, O ring glyph (⌀ 8, 1.5px) at x 70%.
  - On track: `--hd-on`, inner top `edge-tint 30%`, I glyph (1.5×9px) at x 30% in on-ink 55%.
  - Thumb: `--hd-thumb` (white mixed 96% with surface), shadows `0 1px 1px shadow-ink 18%, 0 3px 8px shadow-ink 22%`, 0.5px edge.
  - Off→on: thumb translateX 280ms spring, track fill crossfade 180ms. Press: thumb width +18%.
- **Tactile.**
  - Recessed slot `--hd-sunken`, inset `0 2px 4px #000 60%`, radius 6.
  - Lever cap 24×26 in satin steel with 5 grip lines (2px period).
  - Engraved I and O legends beside the slot; LED 6px to the right.
  - Off→on: 120ms snap, LED lights at the end of travel.
- **Minimal.**
  - Off: 1.5px ink outline, thumb ink ⌀ 20.
  - On: track ink fill, thumb `--hd-surface-solid`.
  - The state word in the status line changes at the same frame. 160ms standard.
- **Instrument.**
  - Key 56×30, radius 3, border HUD 22%.
  - Top light bar 3px: HUD 100% plus glow when on, HUD 12% when off.
  - Centre text I / O in Share Tech Mono. 110ms plus flash.

**H2 Rocker switch.** Paddle w:h 1:1.6 in a bezel inset 6% of width. The pressed half is the visible state.

- **Tactile (signature).**
  - Bezel: graphite or aluminium frame, radius 8, outer cast shadow `0 4px 10px #000 45%`, inner lip 1px highlight top-left.
  - Paddle: bakelite. The raised half is lighter with a specular; the pressed half is darker by 12% and 4% shorter (perspective).
  - Engraved I on the upper half, O on the lower. LED in the bezel top-right.
  - Toggle: `rotateX(±9deg)` 120ms snap, shadow shift 2px.
- **Glass.** A frosted paddle (surface + 10% edge-tint) in a glass bezel. The pressed half carries the `--hd-on` wash at 70% when on, with a hairline divider.
- **Minimal.** Two stacked rectangles separated by a hairline. The active half is ink-filled with the inverted label.
- **Instrument.** Two stacked keys. The active one has a HUD underline 2px plus glow and HUD text; the inactive one HUD 30%.

**H3 Dimmer dial (dial-ring).**

- Geometry: arc 270° from 225° to 135°. Track stroke 7.5% D. Value arc from the start to the value. Handle ⌀ 11% D (hit 44px). Centre numeral (brightness %) at 26cqmin. Label above, state below. Tapping the centre toggles power.
- Drag: angular, with a 15° dead zone around the gap. Alternative: vertical drag at 220px per full range. Keys: ←↓ −1, →↑ +1, PgUp/PgDn ±10, Home/End.
- **Glass.** Track ink 12%. Arc `--hd-light-on` (warm, W-overridable). A soft under-glow arc (stroke +6px at 16% opacity, static). White handle with contact shadow.
- **Tactile.** A real knob: outer knurl ring (`--hd-t-knurl` masked to ⌀ 100% → 86%), steel cap ⌀ 62% D with `--hd-t-steel`, pointer line 2×18% D in ink. A ring of 11 printed ticks plus "0" and "100" legends. The arc becomes an amber LED crescent of 21 dots outside the ticks.
- **Minimal.** Track hairline 2px, arc ink 3px, no handle (a 6px dot), Geist 300 numeral.
- **Instrument.** 40-segment arc. Lit segments HUD; the leading segment 100% with glow, the others 70%. Seven-segment percent readout.
- Transitions: during drag none (follows the pointer); remote updates 320ms out (Tactile and Instrument 220ms).

**H4 Thermostat dial.**

- 3×3 default. D = body short side × 0.86.
- Sweep 270° over the climate range (default 5–30 °C, from the entity).
- Track stroke 6% D. Target handle ⌀ 12% D (hit 48px).
- Current-temperature marker: a 6px triangle just outside the arc. With Minimal and Instrument also a tick.
- Active segment between current and target: `--hd-heat` when heating, `--hd-cool` when cooling, ink 35% when idle (two hues at most; never a full rainbow arc).
- Centre: target numeral (hero), the line "Heating to" / "Cooling to" / "Idle" (i18n), current value with a thermometer glyph at 0.32× hero size.
- −/+ buttons (44px) sit in the bottom gap, step 0.5. Mode segmented control below the dial (heat / cool / auto / off; icons plus text).
- **Glass.** A dot every 1 °C at ink 25%. The handle is a glass puck with the target colour rim.
- **Tactile.** A large knob ⌀ 58% D with a pointer to the target. The face has minor ticks per 0.5 °C, majors per 5 °C with Barlow numerals, and a red current-value needle under the knob rim. The mode selector is a 4-detent rotary.
- **Minimal.** Only the arc, a handle dot and numerals. No ticks.
- **Instrument.** A tick scale per 0.5 °C; target and current shown as two seven-segment readouts (TARGET large, CURRENT small); the heat/cool segment glows.

**H5 Analogue needle gauge.** Sweep 240° (−120° … +120°).

- Ticks: major every 10% of range (2px, 9% D long), minor every 2% (1.25px, 5% D).
- At most 6 numerals, at radius 0.68.
- Zone bands (warn, alert) on the inner edge, width 4% D, at 70% opacity of the state colour.
- Needle: tapered polygon, base 3.2% D, tip 0.8% D, length 0.44 D, tail 0.12 D.
- Hub ⌀ 9% D with a highlight. Unit and label below the hub. Digital sub-readout at the bottom.
- **Tactile (signature).**
  - Bezel: steel ring 4% D (`--hd-t-steel`).
  - Face: ivory (light) or black (dark), with a radial vignette of 6% at the edge.
  - Glass reflection: one static ellipse (top-left, 8% white, 60% × 30% D).
  - Needle red `#D2463A` (token `--hd-t-needle`) with a shadow offset 2px at 30%.
- **Instrument.** No face. Luminous ticks HUD 55%. Needle HUD 100% with a soft glow line (a duplicate polyline at 25%, 3px wider). Zones as segment colours.
- **Glass and Minimal.** Glass renders it as a thin-needle gauge without a bezel (face = card). Minimal: a 1px needle, ticks only at majors.
- Motion: `transform: rotate()` with `--hd-ease-needle` 1100ms. Changes smaller than 0.25% of range are ignored (no jitter). Reduced motion: jump.

**H6 Radial arc gauge.** Sweep 270°. Stroke 7.5% D (Glass), 4% (Minimal), segmented (Instrument).

- Value colour: accent, or the zone colour when the value lies in a warn or alert zone.
- End-cap dot ⌀ 1.35 × stroke in `--hd-thumb` (Glass only).
- Centre numeral 28cqmin, unit, label below the arc gap.
- Min and max legends at the arc ends in muted 11px.
- **Glass.** Round caps, under-glow arc (static).
- **Minimal.** Butt caps, track hairline 1px.
- **Instrument.** 40 segments.
- **Tactile.** Renders the needle gauge instead (an arc gauge has no hardware analogue).
- Motion: dasharray 420ms out; zone colour crossfade 220ms.

**H7 Big temperature readout.**

- Integer at hero size. Decimal at 0.5× hero, top-aligned to cap height. Degree sign at 0.45×, top-aligned. Optional unit letter in muted.
- Trend glyph (↑ ↓ →, 0.3× hero) when the 30-min slope exceeds ±0.3 °C/h, coloured heat or cool at 70%.
- Sub-line: 24h min and max in muted 12.5px. Optional comfort label ("Comfortable" / "Dry" / "Humid").
- **Glass.** Inter 300.
- **Minimal.** Geist 300 at 36cqmin, which makes it the largest numeral of any family.
- **Tactile.** A VFD window (black glass with inset shadow, amber Geist Mono 500 with glow 6px at 50%), engraved label plate above.
- **Instrument.** Seven-segment digits with ghost 7%, a decimal point segment, °C in Share Tech Mono.
- Updates: no counting animation (it is noise). A 180ms opacity dip to 0.6 on change, only in Glass and Instrument.

**H8 Sensor tile with sparkline.** 2×1 minimum.

- Header row: icon badge 24 and label (1 line, ellipsis; 2 lines at 2×2).
- Value row: numeral 22cqmin + unit.
- The sparkline is full-bleed at the bottom, 38% of height, inset 0 horizontally, clipped by the card radius.
- Line 1.75px round joins. Area: a vertical gradient of `--hd-accent` from 22% to 0. End dot 3px with a halo ring of 6px at 20%.
- Y range = data min/max padded 12% (not zero-based). Optional dashed min and max markers.
- Scrub: on hover or focus, a vertical hairline plus a value chip. ←/→ move by one sample.
- **Glass** as specified.
- **Minimal.** 1.25px ink line, no area, an end dot only.
- **Tactile.** The graph sits in a black window with an amber trace and a 4×3 faint grid.
- **Instrument.** HUD line, grid at HUD 8%, end dot with glow.
- The sparkline is redrawn on resize (ResizeObserver), never stretched.

### 4.2 Compact specs (remaining types)

- **Light tile.** The whole tile is the dimmer: a horizontal drag sets brightness, and the fill rises from the left as a `--hd-light-on` wash at 18–40% by level. Tap toggles; long press opens the sheet with CT strip and wheel.
- **Fader-v.** 1×3, cap 40×24, scale ticks every 10%.
- **Cover.** The shutter picture is 8–12 slats (repeating-linear-gradient) masked by position; up/stop/down buttons 44px; the position numeral is in the status line.
- **Lock.** A track with "slide to unlock" (i18n). The thumb must travel ≥ 85% of the track; on release it springs back unless confirmed. Locked state shows a lock glyph plus the `--hd-ok` LED; unlocked shows `--hd-warn`.
- **Fan.** 3–5 speed segments plus power; the rotor icon rotates at 0.5/1/1.5 rps by speed (only with motion allowed, `transform`).
- **Media.** Title and artist in 2 lines, transport buttons, volume slider; cover art in the sheet or 2×2 variant, with a 50% scrim under the text.
- **History chart.** The same maths as the sparkline plus axes: 3–5 y gridlines at hairline, time labels in muted 11px, tabular. Binary timeline: on segments in `--hd-on` at 70%, off in track.
- **Camera.** A dark media surface (exempt from palette like AGENTS.md:3787-3790); the family frames it (Glass glass rim, Tactile monitor bezel, Minimal hairline, Instrument brackets with a REC lamp).
- **Status list.** Rows 40px: lamp, name, value right-aligned and tabular; at most 8 rows, then "+N".
- **Room card.** Title (heading font), temperature numeral L, humidity S, 2–4 mini toggles (36px visual, 44px hit), alert badge.
- **Weather.** Own 14-condition icon set (line, 1.75 stroke). Current temperature hero; 5-day rows with min and max as a range bar.

---

## 5. Dashboard-level look

- **Backdrops.**
  - `theme`: `--hd-canvas-fill`, i.e. the theme's canvas material, patterns included.
  - `solid`: `--hd-canvas`.
  - `gradient`: `linear-gradient(160deg, var(--hd-canvas), var(--hd-canvas-2))` plus `radial-gradient(60% 50% at 15% 0%, color-mix(in srgb, var(--hd-accent) 9%, transparent), transparent)`.

Image backdrops are deferred. The glass blur is used only with the `gradient` backdrop.

- **Ambience** (opt-in, one per dashboard, only with motion allowed and the tab visible):
  - Glass: two radial light pools drifting with translate over 60s alternate, opacity 0.5.
  - Instrument: one horizontal scan sweep every 14s at 3% opacity.
  - Tactile and Minimal: none (hardware and paper do not move).
  - Reduced motion or `data-animations="false"`: static frame. Kiosk "low power" turns ambience and blur off.
- **Density.** compact / comfortable / spacious set `--hd-unit` to 3.4 / 4 / 4.6px, card padding 12 / 16 / 20, gap 8 / 12 / 16.
- **Card treatment.** Per family, §2. `look.radius_scale` multiplies family radii only. Theme radii pass through unchanged.
- **Mixed families.**
  - The dashboard family owns the backdrop, section frames, headings and grid gap.
  - An overriding widget changes only its own body and frame.
  - Rule: at most 2 families per section. The editor warns on a third. A Tactile or Instrument widget on a Glass or Minimal dashboard gets the dashboard family's outer contact shadow, so it looks mounted rather than floating.
- **Headings and sections.**
  - Glass: section title in `--hd-font-heading` 15px/600 ink, gap 8px, no frame.
  - Minimal: 13px/500 muted with a hairline extending to the right edge.
  - Tactile: an engraved label plate (Barlow 600 13px, plate 2px inset).
  - Instrument: Oxanium 500 12px, .12em tracking, bracket corners on the section frame.

---

## 6. Editor chrome (palette, inspector, toolbars)

- The chrome is a normal desktop app surface painted **only** from `--vd-mat-*` with fallbacks, the way Docker Manager does it:
  - toolbars `--vd-mat-bar` + `-bar-edge`;
  - widget palette and inspector `--vd-mat-side` + `-side-edge`;
  - inputs `--vd-mat-field*`; buttons `--vd-mat-btn*`;
  - selected list rows `--vd-mat-selected*`; hover `--vd-mat-hover`;
  - headings and labels from the type tokens.
- The canvas sits in a `--vd-mat-well` frame with `--vd-mat-well-shadow`, so the dashboard reads as a document.
- **Never compete with the canvas.** The chrome has:
  - no blur, glass or glow;
  - no accent fill except the single primary action ("Save"/"Done");
  - type at `--vd-font-ui` 13px;
  - icons 16px monochrome `currentColor`.
- Edit-mode overlays:
  - grid lines at hairline 8% ink;
  - selection = 1.5px dashed `--vd-accent` outline offset 3px, with 8×8 handles (hit 24px) in `--vd-mat-btn`;
  - drop target = 12% accent wash;
  - a "family/variant" chip on the selected widget in `--vd-mat-card`.
- Motion uses `--ds-motion-*`, gated like the rest.
- Kiosk has no chrome. Its exit and unlock affordance is a 44px corner hot zone (owned by the kiosk spec).

---

## 7. States and feedback (one language for all families)

| State | Trigger | Visual (shared rule) | Family expression |
| --- | --- | --- | --- |
| Optimistic | tap | the control moves to the target state immediately | — |
| Pending | no confirmation after **250ms** | the "indicator in motion" appears; nothing before 250ms (no flicker) | Glass conic sweep on the thumb; Tactile LED blink 2Hz; Minimal hairline ring; Instrument segment chase |
| Confirmed | state echo | indicator stops; check glyph in the status line 600ms then fades | Glass thumb pulse 1 → 1.06 → 1 (180ms); Instrument flash; Tactile steady LED; Minimal none |
| Failed | error or 8s timeout | **rollback** to the previous state with the normal transition, then the error cue; inline error text (alert-ink) for 6s; `aria-live="polite"` | Glass and Minimal shake translateX 6/4px, 3 cycles, 320ms; Tactile red LED blinks 3×; Instrument alert flash |
| Unavailable | `available = false` | dashed outline, value "—", state word "Unavailable", no affordance, `aria-disabled` | per §2 |
| Stale | age > max(2 × expected interval, 5 min) | value at ink-muted, small clock badge 12px top-right, tooltip/description "Updated 14 min ago" | same badge in all families |
| Read-only | `readonly = true` | lock badge 12px top-right, thumb and handles without shadow or hover, `aria-readonly`, cursor default | same badge |
| Confirm required | `widget.confirm` | first tap **arms**: a ring or underline countdown of 4s and the text "Tap again to confirm"; second tap sends; locks use slide to unlock instead | ring in accent (Glass), amber LED (Tactile), text only (Minimal), HUD ring (Instrument) |
| Alert value | value in an alert zone or binary alert | alert colour **plus** a ▲ glyph **plus** a word | per §2 |

The status line under each control (12.5px, 1 line, reserved height) carries the words. Its height is always reserved, so state changes never shift the layout.

---

## 8. Accessibility

- **Contrast.** Text ≥ 4.5:1, numerals ≥ 3:1 at ≥ 24px but held to 4.5 anyway, non-text graphics (tracks against surface, arcs, thumbs, focus rings) ≥ 3:1. These are measured by the lab probe on `--hd-surface-solid` and on the composited glass colour.
- **Glass over arbitrary backdrops.** The guarantee is worst-case composition:
  - Image backdrops are deferred, so the image worst case below applies only once they exist. In version 1 the cases are the theme, solid and gradient backdrops.
  - With the image backdrop the scrim is mandatory: scrim-alpha ≥ 0.45 of `--hd-scrim` (black, or the theme canvas colour, in dark; white in light).
  - Glass alpha ≥ 0.62 (dark) or 0.70 (light).
  - Hand-computed worst case: under a 0.45 black scrim a pure-white image pixel is at most channel 140. Glass `#1A1F29` at 0.62 over it gives channel ≈ 64 (L ≈ 0.052). Muted `#A3ADBD` then reaches 4.8:1 and ink about 14:1. The light case: min channel 115 → composite ≈ 209 → muted `#4D5666` about 5.3:1.
  - With the theme backdrop, the glass composite is a per-channel mix of two colours (`--vd-mat-card-color`, `--vd-mat-canvas-color`) that both pass with `--vd-text`, so the mix passes too. Luminance is monotone per channel.
  - The lab asserts these numbers with a probe like `desktopThemeTokenProbeJS` (contrast test :37-78): it composites ink over glass over both pure white and pure black under the scrim.
- **Non-colour cues** everywhere (§2, §7).
  - On/off: position + I/O glyph + word.
  - Alert: ▲ shape + word.
  - ok vs alert (green and coral are confused by deuteranopes): never colour alone. The lab renders every state sheet through a deuteranopia/protanopia matrix (`feColorMatrix` on the screenshot only, in the harness) and the reviewer checks that the states can still be told apart.
- **Focus-visible.** A double ring `0 0 0 2px gap, 0 0 0 4px focus` on every interactive element.
  - Glass also adds `--hd-theme-glow`.
  - Instrument uses HUD 100% as the focus colour.
  - Tactile also lifts the hardware by 1px.
  - Ring radius = control radius + 4. Never removed.
- **Hit targets.** ≥ 44×44 CSS px everywhere, including the desktop (brief §4.4). Inside 1×1 tiles the whole tile is the target.
- **Text scaling.** Numerals scale with the container (cqmin). Labels use rem plus `--hd-text-scale` (kiosk setting 1.0–1.5). At 1.5× labels clamp to 2 lines with `hyphens: auto`. The lab sweeps de and el strings for long words.
- **Keyboard and ARIA.**
  - Toggle and rocker: `role=switch`, `aria-checked`.
  - Push: `button`.
  - Slider, dial, thermostat target: `role=slider` with `aria-valuemin/max/now/valuetext` ("21.5 °C, heating").
  - Segmented control: `radiogroup`.
  - Gauges: `role=meter` (as docker-manager-charts.js:101).
  - Binary: text and status.
  - Sparkline: `role=img` with a summary label ("min 19.8, max 23.1, now 21.4").

---

## 9. CSS architecture

### 9.1 Files

Parts are concatenated by `scripts/build-ui-bundles.js`, which already builds CSS bundles (`cssBundles`, :263-320), into **one** bundle. All files sit flat in `ui/css/`, never `desktop-materials-*` or `desktop-theme-*`.

| Part | Purpose | Approx. lines |
| --- | --- | --- |
| `homedash-tokens.css` | `.hd-root` defaults, neutral `--hd-n-*` sets, motion and type scales, `@font-face "HomeDash Barlow Condensed"` and `"HomeDash Share Tech Mono"` (`/fonts/…`) | 200 |
| `homedash-bridge.css` | `[data-hd-palette="theme"]` → `--vd-mat-*`/`--ds-*` (§1.3); dark/light palette blocks | 120 |
| `homedash-base.css` | grid host, card frame skeleton, status line, badges, focus, states (§7), reduced-motion and animation gates, skin guards | 450 |
| `homedash-primitives.css` | family-neutral structure of every primitive (sizes, hit areas, layout) | 700 |
| `homedash-family-glass.css` | token block + Glass painting | 600 |
| `homedash-family-tactile.css` | token block (materials) + painting | 800 |
| `homedash-family-minimal.css` | token block + painting | 350 |
| `homedash-family-instrument.css` | token block + painting | 650 |
| `homedash-widgets.css` | per-widget layout (container queries) | 800 |
| → `homedash.bundle.css` | **the only engine stylesheet loaded** (desktop + kiosk) | ~4.7k |
| `desktop-app-homedash.css` | desktop host and editor chrome (§6), `--vd-mat-*` only | 500 |

- **Desktop.** `DESKTOP_APP_ASSETS['homedash'] = { styles: appStyles('/css/homedash.bundle.css', '/css/desktop-app-homedash.css'), … }`. Both files contain `--vd-mat-`, so the adoption guard passes with **no exemption**. The bundle contains the bridge.
- **Kiosk page.** Links `/fonts/fonts.css` and `/css/homedash.bundle.css`. The `--vd-*` fallbacks resolve to `--hd-n-*`.
- Each part stays ≤ 900 lines (HomeDash's own rule). The guard's file pattern sees only the bundle.

### 9.2 Naming and layering

- Classes: `hd-` prefix only, never `cc-` (ui/AGENTS.md:18). Data attributes: `data-hd-*`.
- Order: tokens → bridge → base → primitives → families → widgets.
- Family painting rules are all written as `[data-hd-family="x"] .hd-…`, at equal specificity, so the nearest family wins through a reset pattern: `[data-hd-family] [data-hd-family] …` is not needed because each `.hd-w` that overrides re-declares the F tokens. Rules that differ structurally per family use `:where([data-hd-family="x"]) .hd-toggle`, at zero specificity for the family part, to avoid specificity wars.
- No `!important` except `[hidden]` (Docker Manager precedent, desktop-app-docker-manager.css:8).

### 9.3 Skin guards

Skins reach into apps for `:focus-visible`, `::selection`, scrollbars, input font size and `kbd` (AGENTS.md:2871-2879; for example Amiga `:focus-visible … !important`, desktop-theme-amiga.css:167; `::selection` in 7 skins). HomeDash's own focus ring is a box-shadow, so the Amiga outline only adds a dashed black line. A guard block at the end of `homedash-base.css` sets `::selection`, scrollbars and input font on `.hd-root.hd-root`, inside the canvas only.

### 9.4 HomeDash Node guard (`scripts/test-homedash-ui.mjs`)

Modelled on test-docker-manager-ui.mjs:2062-2115:

1. Painting declarations contain no hex, `rgb/hsl/oklch(` literal or named colour (except `transparent` and `currentColor`). Literal colours, gradients and shadows are allowed **only** as values of `--hd-*` custom properties in token blocks. A token block's selector matches `^\.hd-root(\[data-hd-(palette|scheme)="\w+"\])?$` or `^\[data-hd-family="\w+"\]$`.
2. No `[data-theme`, no theme id, no `cc-`; every class starts with `hd-`.
3. Every `transition`/`animation` sits inside `@media (prefers-reduced-motion: no-preference)` on selectors gated by `.hd-root:not([data-hd-motion="reduced"]):not([data-hd-motion="none"])`. The desktop host mirrors the desktop animation switch into `data-hd-motion`.
4. Keyframes animate only `transform` and `opacity` (mirrors the skin rule, contract test :26-30).
5. No `text-transform: uppercase` (i18n: German ß, Greek accents).
6. `backdrop-filter` only on `.hd-glass-pane` and `.hd-sheet`, blur ≤ 24px.
7. `url()` only `/fonts/…` or `data:image/svg+xml` ≤ 4096 bytes without script, foreignObject or remote addresses (mirrors desktop_app_materials_test.go:370-387).
8. `@font-face` family names start with `"HomeDash "`.
9. Each part ≤ 900 lines.
10. `stroke-width`, `border-radius` and `box-shadow` values in family files reference `var(--hd-…)` (forces the token discipline that keeps strokes and radii consistent).
11. No `light-dark()`.
12. JavaScript under `ui/js/homedash` never calls `setAttribute('style'`; styles are set through the CSSOM (kiosk CSP).

The global external-assets test (`external_assets_test.go`) covers the rest.

---

## 10. Quality process

### 10.1 Style lab (before mass production)

- **Content.** 8 heroes × 4 families = 32 reference renderings.
  - Each is shown in 6 states (off, on, pending mid-frame, unavailable, alert, focus) and at 2 sizes (min, default).
  - Palettes: kiosk dark, kiosk light, and theme in Standard, Fruity light, Organic, Crazy, Amiga.
  - DPR 1 and 2.
  - Plus a motion strip per hero: 6 frames sampled at 0/50/100/200/400/800ms after a state change.
- **Acceptance bar** (the lead judges screenshots):
  - Each family is recognisable from a single cropped control.
  - The 4 families sit side by side on one sheet without one looking unfinished.
  - No control looks like a native browser widget.
  - Text and graphics pass the probes.
  - The motion strips show the intended curve (spring overshoot ≤ 2px for the thumb, needle peak about 6%, Tactile detent visible in 1–2 frames).
  - Everything is crisp at DPR 1: no half-pixel hairlines, no knurl moiré.

### 10.2 Control lab harness

- A test-only gallery, never shipped as an app route. It renders `type × variant × family × state (off, on, pending, disabled, unavailable, stale, read-only, error, min, max) × palette`.
- Built like the desktop preview and theme-polish harnesses: a Rod-driven Go browser test plus a fixture page. Test name inside the CI shard range: UNVERIFIED which range, see §12.
- **Sheets.**
  1. hero sheet per family;
  2. widget-type sheet (variants × states) per family;
  3. theme sweep: all 16 theme variants (14 + Fruity dark + Minimal dark) × Glass and Minimal on 4 representative widgets;
  4. risk sweep: Tactile and Instrument on the 6 risk themes (Fruity light, Lollipop, Amiga, Crazy, Cyberpunk, Green Slime);
  5. kiosk dark/light plus 3 custom palettes (very light warm, saturated purple accent, near-black);
  6. DPR 1 and 2; reduced motion; `data-animations="false"`; colour-blind matrices; locale de and el; text-scale 1.5.
- **Probes per screenshot.**
  - Contrast pairs, including glass worst case.
  - Hit areas ≥ 44.
  - No layout shift on state change (bounding boxes stable).
  - No running animations after settle with reduced motion.
  - Number of `backdrop-filter` elements ≤ cards + 1.

### 10.3 Per-control acceptance checklist

1. Matches its family spec (layers, ratios, tokens); reviewed on the hero or widget sheet.
2. All 10 states render and are distinguishable in greyscale.
3. Keyboard: role, arrows, Home/End, PgUp/PgDn, Space/Enter; `aria-valuetext` localised.
4. Focus ring visible on every palette and in Amiga.
5. Hit area ≥ 44.
6. Text 4.5, graphics 3.
7. Reduced-motion and animations-off variants are correct.
8. Crisp at DPR 1 and 2.
9. Optimistic → pending (250ms) → confirmed or rollback.
10. No layout shift.
11. Only compositor-friendly motion.
12. ≤ 150 SVG nodes.
13. Strings via i18n only; untrusted names via `textContent`.

### 10.4 The 10 most likely ways it ends up looking cheap, and the rule against each

| # | Failure | Rule |
| --- | --- | --- |
| 1 | Muddy grey glass with unreadable text over photos | mandatory scrim ≥ 0.45 + glass alpha minimum + worst-case probe (§8) |
| 2 | Inconsistent stroke widths and radii between widgets | strokes and radii only via tokens (guard rule 10) |
| 3 | Jittering numbers | tabular numerals, fixed readout widths in ch, no counting animations |
| 4 | Rainbow or neon overload | at most 2 hues per widget (accent + one state); gradients are lightness ramps of one hue; Instrument has one glow colour |
| 5 | Fake skeuomorphism (wood, screws, photo textures) | Tactile: 1 panel + 1 control material, no raster textures, no screws below 2×2 |
| 6 | Blurry 1x rendering | hairlines on .5 coordinates, integer viewBox to px, ticks ≥ 1.25px, knurl/brush period ≥ 2 device px; DPR 1 sheets mandatory |
| 7 | Mixed icon styles (theme packs are coloured app icons) | one HomeDash line-icon set: 24 grid, 1.75 stroke, round joins, about 70 glyphs (device classes + 14 weather) |
| 8 | Spinners everywhere, flicker | 250ms pending delay; one indicator per control |
| 9 | Laggy motion and layout jumps on tablets | transform/opacity only, reserved status line, `contain`, no animated filters or shadows |
| 10 | Crowded or empty tiles | spacing tokens, at most 3 text sizes per widget, label/value/state hierarchy; min sizes enforced by the catalog |
| (11) | Broken long locale strings | 2-line clamp + `hyphens: auto`, de and el sweep |

---

## 11. Work packages (visual system)

Each package is sized for one maker-agent. **R** marks a lead screenshot review gate before the next package starts.

The main document (section 8) merges some of these packages with frontend packages: P0 replaces V2, P1 replaces V8, and W1–W4 replace V9–V12.

| # | Goal | Files | Acceptance | Effort |
| --- | --- | --- | --- | --- |
| V0 | Tokens, neutral palettes, theme bridge, fonts, Node guard, bundle entry | homedash-tokens/bridge/base.css, build-ui-bundles.js entry, scripts/test-homedash-ui.mjs | guard green; `go test ./ui -run DesktopAppMaterials` green with the new module-loader entry; probe: bridge resolves in 16 theme variants and kiosk | M |
| V1 | Control-lab harness skeleton (fixture page, state matrix, sheet writer, contrast/hit/shift probes) | ui test + fixture JS | renders one placeholder control in every palette and theme; probes report | M |
| V2 | Geometry helpers: arc/tick/needle path builders, monotone sparkline, seven-segment digits, number formatting (tabular, decimal split) | `ui/js/homedash/` geometry modules + node tests | unit tests (paths, monotone without overshoot, segment map 0–9, `-`, `.`) | M |
| V3 | Style lab Glass: 8 heroes | primitives + family-glass.css | lab hero sheet | L **R** |
| V4 | Style lab Tactile: 8 heroes | family-tactile.css | sheet; DPR 1 moiré check | L **R** |
| V5 | Style lab Minimal | family-minimal.css | sheet | M **R** |
| V6 | Style lab Instrument | family-instrument.css | sheet | L **R** |
| V7 | Cross-family consistency + risk-theme sweep fixes; freeze the token list | all family files | side-by-side sheet; risk sweep clean | M **R (gate to mass production)** |
| V8 | Interaction layer: drag/keyboard/ARIA for slider, knob, dial, stepper, segmented; pending/confirm/rollback/confirm-required visuals | primitives JS + base.css | keyboard tests; state sheet | M |
| V9 | Widgets batch 1: Switch (4), Light (4 + sheet with CT and wheel), Scene, Select, Number | widgets.css + renderers | widget sheets × 4 families | L **R** |
| V10 | Batch 2: Climate (3), Temperature (3), Sensor (3), Gauge (4), Battery | — | sheets | L |
| V11 | Batch 3: Binary, Cover, Fan, Lock, Status list, Room card | — | sheets | L |
| V12 | Batch 4: Media, History chart, Camera frame, Clock, Text, Group, Nav, Weather + icon set | + icon sprite (inline SVG symbols, own set) | sheets | L |
| V13 | Dashboard look: backdrops, scrim, ambience, density, sections, mixed-family rules | base + family files | dashboard sheets (4 families × 3 backdrops × 2 densities) | M **R** |
| V14 | Editor chrome styling | desktop-app-homedash.css | survey in 16 theme variants | M |
| V15 | Accessibility and performance pass (tablet profile: CPU 4× throttle, DPR 2) | — | 60 fps state changes on a 24-widget dashboard; probes green | M |
| V16 | Final sweep + acceptance sheets for the user | — | full sheet set | S **R** |

---

## 12. Open questions, risks, UNVERIFIED

1. **Tablet browser baseline (UNVERIFIED).** These features need recent engines: `color-mix()` (Chrome 111, Safari 16.2), container queries (Chrome 105, Safari 16), `light-dark()` (Chrome 123, Safari 17.5), `linear()` easing (Chrome 113, Safari 17.2). Older wall tablets need fallbacks. Decide the kiosk minimum (proposal: Chrome/WebView 123, iPadOS 17.5). Provide `@supports` fallbacks for `linear()` and `light-dark()`. Decision: Chromium/WebView 111, Safari/iPadOS 16.4, Firefox 113; no `light-dark()`; `@supports` fallback for `linear()`.
2. **Tabular figures** in the bundled Inter, Geist, Oxanium and Barlow subsets are UNVERIFIED; the lab checks them (§0.e).
3. **Barlow Condensed** exists only as a SemiBold Latin TTF (UNVERIFIED glyph coverage). Non-Latin Tactile legends fall back to Inter 600.
4. **Grid cell size** (96px nominal) is an assumption; the grid spec owns it.
5. **CI shard name range** for new browser tests is UNVERIFIED (brief §3). The lab test name must be chosen against it.
6. **Kiosk "theme" palette.** Solved by the palette snapshot (section 1.4).
7. **Image backdrops:** a pre-blurred variant and the upload path need the backend (UNVERIFIED).
8. **Bundle vs. separate files.** If the lead rejects a new CSS bundle, the alternative is separate files. Each one that does not read `--vd-mat-*` (primitives, families, widgets) must then go into `desktopAppMaterialsExempt` with a reason, which bends that list's intent ("not a themed surface"). The bundle is recommended. Decision: the bundle.
9. **The user-accent themes** (Standard, Fruity, Black Gloss) put arbitrary accents into `--hd-on`. The materials already guarantee primary-text/primary contrast, but accent arcs against cards (3:1) are not tested anywhere. The lab sweeps all accent presets (desktop_theme_token_contrast_browser_test.go:12-31 shows how to list them).
10. **Chrome conic-gradient hard-stop aliasing** (knurling) and moiré at DPR 1 are a known risk. The softened stops in §2.2 are the mitigation; verify in V4.
11. **Effort.** The style lab is the main schedule risk. If V3–V6 need more than one revision each, cut Instrument's seven-segment digits to Share Tech Mono for v1 rather than lowering the hero bar.
12. **HA Switchboard stays untouched.** HomeDash does not reuse its raster atlas; its Tactile rocker is pure CSS/SVG.

Key files cited: `C:\Users\Andi\Documents\repo\AuraGo\ui\desktop_app_materials_test.go`, `C:\Users\Andi\Documents\repo\AuraGo\documentation\desktop-themes.md`, `C:\Users\Andi\Documents\repo\AuraGo\ui\js\desktop\apps\AGENTS.md`, `C:\Users\Andi\Documents\repo\AuraGo\scripts\test-docker-manager-ui.mjs`, `C:\Users\Andi\Documents\repo\AuraGo\ui\external_assets_test.go`, `C:\Users\Andi\Documents\repo\AuraGo\ui\desktop_theme_token_contrast_browser_test.go`, `C:\Users\Andi\Documents\repo\AuraGo\ui\fonts\fonts.css`, `C:\Users\Andi\Documents\repo\AuraGo\ui\fonts\jetbrains-mono.css`, `C:\Users\Andi\Documents\repo\AuraGo\ui\css\ha-switchboard.css`, `C:\Users\Andi\Documents\repo\AuraGo\ui\css\desktop-materials.css`, `C:\Users\Andi\Documents\repo\AuraGo\ui\css\desktop-materials-organic.css`, `C:\Users\Andi\Documents\repo\AuraGo\ui\css\desktop-materials-cyberpunk.css`, `C:\Users\Andi\Documents\repo\AuraGo\ui\js\desktop\core\theme-runtime.js`, `C:\Users\Andi\Documents\repo\AuraGo\scripts\build-ui-bundles.js`.
