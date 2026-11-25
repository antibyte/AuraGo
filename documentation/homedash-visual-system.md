# HomeDash visual system

This page holds the paint rules of the four HomeDash style families, the frozen token table, the control lab used to judge
them, and the review rules. Engine contracts (DOM, widget host, styling rules) live in `ui/js/homedash/AGENTS.md`. Design
background: `docs/superpowers/specs/2026-11-23-homedash-design.md` section 6. Gate records (findings and passes) are local working files in the
git-ignored `reports/homedash-lab/` folder and are not in git. The versioned record of the rulings that bind later controls is the "Lead rulings that bind later controls" section at the end of this page.

Status: the M1 style lab passed Gates S, G, T, M, I and X (2026-11-24 to 2026-11-25). Structure has been frozen since Gate S,
except the recorded M2 changes SCR-M2-1 to SCR-M2-3 below. The tokens have been frozen since Task 22 (Gate X) and M2 leaves them unchanged.
The M2 lead review gate passed on 2026-11-25 in round 4, on fresh screenshots in all four families.

## The four families

Each family paints the same structure. Families differ by paint only, and every family takes its colours, radii and
type from its own `--hd-*` token block. A widget reads only `--hd-*` tokens.

### Glass

- **Light model:** a soft sheen from the top (edge tint at 5 %) and a 1 px top highlight. Cards are translucent: the surface
  is 72 % opaque in dark schemes and 78 % in light ones. The backdrop is blurred only under the gradient backdrop, and only on
  `.hd-frame`. Card shadows fade out within about 34 px.
- **Materials:** white thumbs and handles with a 0.5 px edge ring and a contact shadow. The value arc is in the accent
  colour (`--hd-value-colour`). The under-glow is dropped (`--hd-arc-underglow` stays in the token block but is unused).
  A zero value draws no caps: the arc uses butt caps under `@container style(--hd-value: 0.0000)` in `homedash-family-glass.css`.
  Verified in Task 20 with a temporary red probe: only the value-0 cell changed.
- **Type:** Inter (Geist as fallback). Numerals are weight 300 with `tnum` and `cv11`. Labels are 12.5 px, weight 500.
- **Theme behaviour:** surfaces, ink, radii, edge and glow come from the desktop theme (`--vd-mat-*`, through the bridge).
  In square-corner themes the controls are square.

### Tactile

- **Light model:** one light from the top left. Every highlight is top-left and every cast shadow is bottom-right, including
  the needle shadow. The brushed panel shows grain only at 2x device pixel ratio.
- **Materials:** knurled knobs with a steel cap, bakelite keys and a rocker, engraved legends (the "O" is drawn as an IEC
  circle), and VFD windows in Geist Mono. The mode selector is four bakelite push keys; the selected key is pressed in with
  an amber pilot dot.
- **Type:** Barlow Condensed SemiBold (600) for labels and numerals (`HomeDash Barlow Condensed`). Geist Mono for VFD windows.
- **Scheme switch:** the dark and light material sets are both in the token block; a `var()`-only rule picks the set from
  `data-hd-scheme`.
- **State language:** the LED is the state hue. Pending is a static half-lit amber with a 2 Hz blink under motion. Error and
  crit use red. An unlit LED reads as dark glass, not flat grey. LED colours are fixed emission colours, not theme colours.

### Minimal

- **Light model:** none. Fills are flat; there are no shadows and no gradients. Hairlines are exactly one device pixel at
  1x. Flatness holds at 400 % zoom.
- **Materials:** ink-filled toggle with an inverted thumb; a two-rectangle rocker; a 3 px ink arc with a dot. The accent
  appears only in the focus ring and in value arcs.
- **Type:** Geist, numerals weight 300. At equal size the numerals are the largest of the four families wherever the body does not
  limit them (judged on the side-by-side sheets).
- **Layout:** the head icon is hidden. Label, numeral and state line share one left edge. The H7 integer box is left-aligned.
- **State language:** unavailable uses ink-faint; the words carry the state.

### Instrument

- **Light model:** a dark HUD panel with a fixed palette (`#090C10`, `#040608`, and so on) on every theme. Four solid 2 px
  corner brackets. Scan lines appear only inside seven-segment windows.
- **Materials:** 40-segment arcs with 40 even gaps and a bright lead segment. Seven-segment digits with 7 % ghost segments.
  One glow colour per widget: `--hd-i-val` is the HUD colour `--hd-hud`, which `applyLook` computes from the accent with
  `E.colour.hudFor`. Alert replaces
  the HUD colour as a whole. The mount shadow applies only in light schemes.
- **Type:** Oxanium for labels (11.5 px, tracking 0.12em). Share Tech Mono (`HomeDash Share Tech Mono`) for numerals and
  seven-segment readouts. No text under 11 px.
- **State language:** crit sets `--hd-i-val` to the alert colour. Unavailable shows a dashed box with the "No data" word
  centred in the body.

### Shared state language

The host writes the wrapper attributes; widgets paint only their own controls (plan section T5).

| State | Glass | Tactile | Minimal | Instrument |
| --- | --- | --- | --- | --- |
| unavailable | dashed border; body at 0.6 opacity with text kept at full ink; arcs hidden; hollow thumb and handle | hatched card, hollow lamp, legends "—", needle hidden | ink set to the muted ink (`--hd-ink: var(--hd-ink-muted)`); lamp and legends as Tactile | body children hidden, dashed "No data" box |
| stale | clock badge, muted numerals | same | same | same |
| readonly | lock badge, no lift shadows, full strength | lock badge, no cast shadow | lock badge | lock badge |
| disabled | 0.45 opacity (`data-hd-disabled`) | same | same | same |
| error | error icon, alert-ink state line | red LED | same | HUD kept, state line alert |
| alert (tone crit) | alert ring, triangle badge, alert word | red LED, 1 px red rim | triangle badge, alert value ink, no ring | value becomes alert; pulse 1 Hz, 6 cycles under motion |
| focus | M0 ring plus theme glow layer | ring, hardware lifted 1 px | ring | ring in HUD colour |

Readonly is not disabled: a read-only entity stays at full strength with the lock badge and "Read-only" in the state line.

## Structure and paint

- **Structure** (frozen): `ui/css/homedash-primitives.css`, `ui/css/homedash-widgets.css`, `ui/css/homedash-base.css`, the
  primitive builders (`hd-prim-*.js`), the frame host (`hd-widgets.js`) and the hero modules (`widgets/hd-w-*.js`). Markup,
  class names, child order and geometry belong to structure.
- **Paint** (family files): `ui/css/homedash-family-{glass,tactile,minimal,instrument}.css`. Family files set colours,
  shadows, gradients, masks, radii of paint and motion, using `var()` and `color-mix()` only.
- After Gate S, structure changed only through approved structure change requests recorded in the local, git-ignored gate findings. Any further
  structure change needs a request and before/after sheets of every family.
- After any CSS change run `npm run build:ui` and commit `ui/css/homedash.bundle.css`.

## Frozen token table

Source: `ui/testdata/homedash-lab/token-freeze.json` (45 selectors; 617 declarations per the Gate X record). The JSON is
authoritative. `scripts/homedash-tests/m1.test.mjs` compares the six frozen CSS parts with the JSON, so this table is a
readable copy. The rule of change is:

- A token change needs the lead's approval, then `node scripts/homedash-freeze-tokens.mjs` to rewrite the JSON, then
  `npm run test:homedash`.
- `npm run test:homedash` fails with "token freeze violated" on any difference (proof: changing `--hd-glass-blur` fails).

The freeze covers every rule whose declarations are all `--hd-*` custom properties in these files:
`homedash-tokens.css`, `homedash-bridge.css`, `homedash-family-glass.css`, `homedash-family-tactile.css`,
`homedash-family-minimal.css`, `homedash-family-instrument.css`.

### `ui/css/homedash-tokens.css`

**`.hd-root`**

| Token | Frozen value |
| --- | --- |
| `--hd-n-canvas` | `#0E1116` |
| `--hd-n-canvas-2` | `#171C26` |
| `--hd-n-surface` | `#1A1F29` |
| `--hd-n-raised` | `#222834` |
| `--hd-n-sunken` | `#0A0D12` |
| `--hd-n-ink` | `#F1F4F9` |
| `--hd-n-ink-muted` | `#A3ADBD` |
| `--hd-n-ink-faint` | `#5E6878` |
| `--hd-n-accent` | `#7AA7FF` |
| `--hd-n-on` | `#8FBFFF` |
| `--hd-n-on-ink` | `#0B1220` |
| `--hd-n-ink-on-accent` | `#0B1220` |
| `--hd-n-selected` | `#2A3A57` |
| `--hd-n-selected-ink` | `#F1F4F9` |
| `--hd-n-btn` | `#262D3A` |
| `--hd-n-btn-ink` | `#F1F4F9` |
| `--hd-n-accent-2` | `#F2B84B` |
| `--hd-n-hairline` | `#2E3542` |
| `--hd-n-ok` | `#4AD29C` |
| `--hd-n-warn` | `#F2B84B` |
| `--hd-n-alert` | `#FF7A72` |
| `--hd-n-info` | `#5AA9FF` |
| `--hd-n-heat` | `#FF9A57` |
| `--hd-n-cool` | `#5CC8FF` |
| `--hd-n-ct-2200` | `#FFA552` |
| `--hd-n-ct-4000` | `#FFE2B8` |
| `--hd-n-ct-6500` | `#E4EEFF` |
| `--hd-n-edge-tint` | `#FFFFFF` |
| `--hd-n-shadow-ink` | `#000000` |
| `--hd-font-ui` | `"Inter", system-ui, sans-serif` |
| `--hd-font-label` | `var(--hd-font-ui)` |
| `--hd-font-num` | `var(--hd-font-ui)` |
| `--hd-font-heading` | `var(--hd-font-ui)` |
| `--hd-num-features` | `"tnum" 1` |
| `--hd-text-xs` | `11px` |
| `--hd-text-s` | `12.5px` |
| `--hd-text-m` | `14px` |
| `--hd-text-l` | `17px` |
| `--hd-text-xl` | `21px` |
| `--hd-label-tracking` | `0` |
| `--hd-dur-press` | `90ms` |
| `--hd-dur-quick` | `140ms` |
| `--hd-dur-state` | `220ms` |
| `--hd-dur-value` | `420ms` |
| `--hd-dur-needle` | `1100ms` |
| `--hd-ease-standard` | `cubic-bezier(.2, 0, 0, 1)` |
| `--hd-ease-out` | `cubic-bezier(.16, 1, .3, 1)` |
| `--hd-ease-spring` | `cubic-bezier(.34, 1.45, .64, 1)` |
| `--hd-ease-snap` | `cubic-bezier(.6, 0, .25, 1.25)` |
| `--hd-ease-needle` | `cubic-bezier(.3, 1.3, .5, 1)` |
| `--hd-radius-scale` | `1` |
| `--hd-radius-card` | `calc(14px * var(--hd-radius-scale))` |
| `--hd-radius-control` | `calc(8px * var(--hd-radius-scale))` |
| `--hd-radius-pill` | `9999px` |
| `--hd-stroke-hair` | `1px` |
| `--hd-stroke-1` | `1.5px` |
| `--hd-stroke-2` | `2px` |
| `--hd-stroke-arc` | `0.075` |
| `--hd-stroke-tick` | `1.25px` |
| `--hd-icon-stroke` | `1.75` |
| `--hd-unit` | `4px` |
| `--hd-space-1` | `var(--hd-unit)` |
| `--hd-space-2` | `calc(var(--hd-unit) * 2)` |
| `--hd-space-3` | `calc(var(--hd-unit) * 3)` |
| `--hd-space-4` | `calc(var(--hd-unit) * 4)` |
| `--hd-space-5` | `calc(var(--hd-unit) * 5)` |
| `--hd-space-6` | `calc(var(--hd-unit) * 6)` |
| `--hd-space-7` | `calc(var(--hd-unit) * 8)` |
| `--hd-space-8` | `calc(var(--hd-unit) * 12)` |
| `--hd-pad-card` | `16px` |
| `--hd-glow` | `none` |
| `--hd-scrim-alpha` | `0.45` |
| `--hd-corners-control` | `9999px` |
| `--hd-fade-down` | `linear-gradient(180deg, #000000, transparent)` |
| `--hd-ring-mask` | `radial-gradient(farthest-side, transparent calc(100% - 2.5px), #000000 calc(100% - 2px))` |
| `--hd-arc-mask-75` | `conic-gradient(#000000 0 75%, transparent 75% 100%)` |
| `--hd-ease-hud` | `cubic-bezier(.2, .8, .2, 1)` |
| `--hd-mask-clock` | `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Ccircle cx='12' cy='12' r='9'/%3E%3Cpath d='M12 7v5l3 2'/%3E%3C/svg%3E")` |
| `--hd-mask-lock` | `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Crect x='5' y='11' width='14' height='10' rx='2'/%3E%3Cpath d='M8 11V8a4 4 0 0 1 8 0v3'/%3E%3C/svg%3E")` |
| `--hd-mask-alert` | `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M12 3.5 2.5 20.5h19Z'/%3E%3Cpath d='M12 10v4.5M12 17.5v.01'/%3E%3C/svg%3E")` |
| `--hd-mask-error` | `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Ccircle cx='12' cy='12' r='9'/%3E%3Cpath d='M12 7.5v5.5M12 16.5v.01'/%3E%3C/svg%3E")` |
| `--hd-card-bg` | `var(--hd-surface-solid)` |
| `--hd-card-border` | `1px solid var(--hd-hairline)` |
| `--hd-dash-unavail` | `1px dashed var(--hd-hairline)` |
| `--hd-shadow-card` | `none` |
| `--hd-ring-alert` | `inset 0 0 0 2px var(--hd-alert)` |
| `--hd-label-size` | `12.5px` |
| `--hd-label-weight` | `500` |
| `--hd-num-weight` | `400` |
| `--hd-num-weight-small` | `500` |
| `--hd-num-tracking` | `0em` |
| `--hd-num-cap` | `0.74` |
| `--hd-num-hero-max` | `96px` |
| `--hd-unit-scale` | `0.42` |
| `--hd-dial-num-k` | `0.26` |
| `--hd-thermo-num-k` | `0.24` |

**`.hd-root[data-hd-scheme="light"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-n-canvas` | `#EEF0F4` |
| `--hd-n-canvas-2` | `#E2E6ED` |
| `--hd-n-surface` | `#FFFFFF` |
| `--hd-n-raised` | `#FFFFFF` |
| `--hd-n-sunken` | `#E7EAF0` |
| `--hd-n-ink` | `#12161D` |
| `--hd-n-ink-muted` | `#4D5666` |
| `--hd-n-ink-faint` | `#9AA2AF` |
| `--hd-n-accent` | `#2F6FE0` |
| `--hd-n-on` | `#2F6FE0` |
| `--hd-n-on-ink` | `#FFFFFF` |
| `--hd-n-ink-on-accent` | `#FFFFFF` |
| `--hd-n-selected` | `#DCE7FB` |
| `--hd-n-selected-ink` | `#12161D` |
| `--hd-n-btn` | `#E7EAF0` |
| `--hd-n-btn-ink` | `#12161D` |
| `--hd-n-accent-2` | `#9A6200` |
| `--hd-n-hairline` | `#D5DAE2` |
| `--hd-n-ok` | `#1A7D52` |
| `--hd-n-warn` | `#9A6200` |
| `--hd-n-alert` | `#C4352B` |
| `--hd-n-info` | `#1F63C7` |
| `--hd-n-heat` | `#D9622B` |
| `--hd-n-cool` | `#1E8FCB` |
| `--hd-n-shadow-ink` | `#1B2333` |
| `--hd-sa-1` | `8%` |
| `--hd-sa-2` | `11%` |
| `--hd-sa-3` | `14%` |
| `--hd-sa-4` | `20%` |
| `--hd-sa-5` | `29%` |
| `--hd-glass-alpha-s` | `78%` |
| `--hd-fill-edge-k` | `72%` |
| `--hd-light-on` | `#C77A12` |

**`.hd-root[data-hd-scheme="dark"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-sa-1` | `14%` |
| `--hd-sa-2` | `18%` |
| `--hd-sa-3` | `24%` |
| `--hd-sa-4` | `34%` |
| `--hd-sa-5` | `48%` |
| `--hd-glass-alpha-s` | `72%` |
| `--hd-fill-edge-k` | `100%` |
| `--hd-light-on` | `#FFB65C` |

**`.hd-root[data-hd-density="compact"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-unit` | `3.4px` |
| `--hd-pad-card` | `12px` |

**`.hd-root[data-hd-density="spacious"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-unit` | `4.6px` |
| `--hd-pad-card` | `20px` |

**`.hd-root[data-hd-ease="fallback"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-ease-needle` | `cubic-bezier(.3, 1.3, .5, 1)` |

**`.hd-root[data-hd-corners="square"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-corners-control` | `0px` |

### `ui/css/homedash-bridge.css`

**`.hd-root`**

| Token | Frozen value |
| --- | --- |
| `--hd-canvas` | `var(--hd-n-canvas)` |
| `--hd-canvas-2` | `var(--hd-n-canvas-2)` |
| `--hd-canvas-fill` | `var(--hd-canvas)` |
| `--hd-surface-solid` | `var(--hd-n-surface)` |
| `--hd-surface-raised` | `var(--hd-n-raised)` |
| `--hd-sunken` | `var(--hd-n-sunken)` |
| `--hd-field` | `var(--hd-n-sunken)` |
| `--hd-ink` | `var(--hd-n-ink)` |
| `--hd-ink-muted` | `var(--hd-n-ink-muted)` |
| `--hd-ink-faint` | `var(--hd-n-ink-faint)` |
| `--hd-ink-on-accent` | `var(--hd-n-ink-on-accent)` |
| `--hd-accent` | `var(--hd-n-accent)` |
| `--hd-accent-2` | `var(--hd-n-accent-2)` |
| `--hd-on` | `var(--hd-n-on)` |
| `--hd-on-ink` | `var(--hd-n-on-ink)` |
| `--hd-selected` | `var(--hd-n-selected)` |
| `--hd-selected-ink` | `var(--hd-n-selected-ink)` |
| `--hd-btn` | `var(--hd-n-btn)` |
| `--hd-btn-ink` | `var(--hd-n-btn-ink)` |
| `--hd-hairline` | `var(--hd-n-hairline)` |
| `--hd-ok` | `var(--hd-n-ok)` |
| `--hd-warn` | `var(--hd-n-warn)` |
| `--hd-alert` | `var(--hd-n-alert)` |
| `--hd-info` | `var(--hd-n-info)` |
| `--hd-heat` | `var(--hd-n-heat)` |
| `--hd-cool` | `var(--hd-n-cool)` |
| `--hd-ct-2200` | `var(--hd-n-ct-2200)` |
| `--hd-ct-4000` | `var(--hd-n-ct-4000)` |
| `--hd-ct-6500` | `var(--hd-n-ct-6500)` |
| `--hd-light-on` | `var(--hd-n-ct-2200)` |
| `--hd-edge-tint` | `var(--hd-n-edge-tint)` |
| `--hd-shadow-ink` | `var(--hd-n-shadow-ink)` |
| `--hd-scrim` | `var(--hd-shadow-ink)` |
| `--hd-focus` | `var(--hd-accent)` |
| `--hd-theme-radius` | `14px` |
| `--hd-theme-border` | `transparent` |
| `--hd-theme-edge` | `none` |
| `--hd-theme-glow` | `none` |
| `--hd-accent-soft` | `color-mix(in srgb, var(--hd-accent) 16%, transparent)` |
| `--hd-pending` | `var(--hd-warn)` |
| `--hd-unavailable` | `var(--hd-ink-faint)` |
| `--hd-focus-gap` | `var(--hd-surface-solid)` |
| `--hd-surface` | `var(--hd-surface-solid)` |
| `--hd-track` | `color-mix(in srgb, var(--hd-ink) 14%, transparent)` |
| `--hd-track-fill` | `var(--hd-on)` |
| `--hd-thumb` | `var(--hd-surface-raised)` |
| `--hd-thumb-edge` | `var(--hd-hairline)` |
| `--hd-ok-ink` | `var(--hd-ok)` |
| `--hd-warn-ink` | `var(--hd-warn)` |
| `--hd-alert-ink` | `var(--hd-alert)` |
| `--hd-info-ink` | `var(--hd-info)` |

**`.hd-root[data-hd-palette="theme"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-canvas` | `var(--vd-mat-canvas-color, var(--hd-n-canvas))` |
| `--hd-canvas-fill` | `var(--vd-mat-canvas, var(--hd-canvas))` |
| `--hd-surface-solid` | `var(--vd-mat-card-color, var(--hd-n-surface))` |
| `--hd-sunken` | `var(--vd-mat-well-color, var(--hd-n-sunken))` |
| `--hd-field` | `var(--vd-mat-field-color, var(--hd-n-sunken))` |
| `--hd-ink` | `var(--vd-text, var(--hd-n-ink))` |
| `--hd-ink-muted` | `var(--vd-theme-muted, var(--hd-n-ink-muted))` |
| `--hd-hairline` | `var(--vd-mat-divider, var(--hd-n-hairline))` |
| `--hd-accent` | `var(--vd-accent, var(--hd-n-accent))` |
| `--hd-ink-on-accent` | `var(--vd-accent-contrast, var(--hd-n-ink-on-accent))` |
| `--hd-on` | `var(--vd-mat-primary-color, var(--hd-n-on))` |
| `--hd-on-ink` | `var(--vd-mat-primary-text, var(--hd-n-on-ink))` |
| `--hd-selected` | `var(--vd-mat-selected-color, var(--hd-n-selected))` |
| `--hd-selected-ink` | `var(--vd-mat-selected-text, var(--hd-n-selected-ink))` |
| `--hd-btn` | `var(--vd-mat-btn-color, var(--hd-n-btn))` |
| `--hd-btn-ink` | `var(--vd-mat-btn-text, var(--hd-n-btn-ink))` |
| `--hd-accent-2` | `var(--vd-mat-accent-2, var(--hd-n-accent-2))` |
| `--hd-ok` | `var(--ds-color-success-fg, var(--hd-n-ok))` |
| `--hd-warn` | `var(--ds-color-warning-fg, var(--hd-n-warn))` |
| `--hd-alert` | `var(--ds-color-danger-fg, var(--hd-n-alert))` |
| `--hd-info` | `var(--ds-color-info-fg, var(--hd-n-info))` |
| `--hd-focus` | `var(--ds-color-border-focus, var(--hd-accent))` |
| `--hd-theme-radius` | `var(--vd-mat-card-radius, 14px)` |
| `--hd-theme-border` | `var(--vd-mat-card-border, transparent)` |
| `--hd-theme-edge` | `var(--vd-mat-card-shadow, none)` |
| `--hd-theme-glow` | `var(--vd-mat-glow, none)` |
| `--hd-font-heading` | `var(--vd-mat-heading-font, var(--hd-font-ui))` |

**`.hd-root[data-hd-scheme="light"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-ok-ink` | `color-mix(in srgb, var(--hd-ok) 34%, var(--hd-ink))` |
| `--hd-warn-ink` | `color-mix(in srgb, var(--hd-warn) 34%, var(--hd-ink))` |
| `--hd-alert-ink` | `color-mix(in srgb, var(--hd-alert) 34%, var(--hd-ink))` |
| `--hd-info-ink` | `color-mix(in srgb, var(--hd-info) 34%, var(--hd-ink))` |

### `ui/css/homedash-family-glass.css`

**`[data-hd-family="glass"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-g-sheen` | `linear-gradient(180deg, color-mix(in srgb, var(--hd-edge-tint) 5%, transparent), transparent 46%)` |
| `--hd-g-bezel-r` | `14px` |
| `--hd-g-paddle-r` | `10px` |
| `--hd-g-thumb` | `color-mix(in srgb, #FFFFFF 96%, var(--hd-surface-solid))` |
| `--hd-g-fill-hi` | `inset 0 1px 0 color-mix(in srgb, var(--hd-edge-tint) 30%, transparent), inset 0 0 0 1px color-mix(in srgb, var(--hd-on) var(--hd-fill-edge-k), #000000)` |
| `--hd-g-surface-edge` | `var(--hd-thumb-ring), var(--hd-shadow-contact)` |
| `--hd-g-wash` | `color-mix(in srgb, var(--hd-ink) 7%, transparent)` |
| `--hd-g-edge-u` | `0.5` |
| `--hd-g-capdot-r` | `10` |
| `--hd-g-shadow-a` | `0.22` |
| `--hd-arc-underglow` | `0.16` |
| `--hd-g-zone-u` | `8` |
| `--hd-g-spark-w` | `1.75px` |
| `--hd-glass-blur` | `18px` |
| `--hd-stroke-hair` | `1px` |
| `--hd-stroke-1` | `1.5px` |
| `--hd-stroke-2` | `2px` |
| `--hd-stroke-arc` | `0.075` |
| `--hd-stroke-tick` | `1.25px` |
| `--hd-num-features` | `"tnum" 1, "cv11" 1` |
| `--hd-num-weight` | `300` |
| `--hd-num-weight-small` | `500` |
| `--hd-num-tracking` | `-0.02em` |
| `--hd-num-cap` | `0.74` |
| `--hd-num-hero-max` | `112px` |
| `--hd-unit-scale` | `0.42` |
| `--hd-label-size` | `12.5px` |
| `--hd-label-weight` | `500` |
| `--hd-label-tracking` | `0em` |
| `--hd-dur-toggle` | `280ms` |
| `--hd-ease-toggle` | `var(--hd-ease-spring)` |
| `--hd-dur-fill` | `180ms` |
| `--hd-dur-xfade` | `180ms` |
| `--hd-dur-remote` | `320ms` |
| `--hd-ease-remote` | `var(--hd-ease-out)` |
| `--hd-dur-zone` | `220ms` |
| `--hd-dur-needle-f` | `1100ms` |
| `--hd-dur-dip` | `180ms` |
| `--hd-th-stroke` | `12` |
| `--hd-th-ring-u` | `2.5` |

**`.hd-widget[data-hd-family="glass"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-surface` | `color-mix(in srgb, var(--hd-surface-solid) var(--hd-glass-alpha-s), transparent)` |
| `--hd-card-bg` | `var(--hd-g-sheen), var(--hd-surface)` |
| `--hd-card-border` | `1px solid color-mix(in srgb, var(--hd-edge-tint) 14%, var(--hd-theme-border))` |
| `--hd-dash-unavail` | `1px dashed var(--hd-hairline)` |
| `--hd-radius-card` | `var(--hd-theme-radius)` |
| `--hd-radius-control` | `var(--hd-corners-control)` |
| `--hd-edge-hi` | `inset 0 1px 0 color-mix(in srgb, var(--hd-edge-tint) 20%, transparent)` |
| `--hd-edge-lo` | `inset 0 -1px 0 color-mix(in srgb, var(--hd-shadow-ink) var(--hd-sa-1), transparent)` |
| `--hd-shadow-contact` | `0 1px 2px color-mix(in srgb, var(--hd-shadow-ink) var(--hd-sa-1), transparent)` |
| `--hd-shadow-card` | `var(--hd-edge-hi), var(--hd-edge-lo), var(--hd-shadow-contact), 0 14px 32px -10px color-mix(in srgb, var(--hd-shadow-ink) var(--hd-sa-4), transparent)` |
| `--hd-shadow-press` | `inset 0 1px 3px color-mix(in srgb, var(--hd-shadow-ink) var(--hd-sa-3), transparent)` |
| `--hd-ring-alert` | `inset 0 0 0 2px var(--hd-alert)` |
| `--hd-track` | `color-mix(in srgb, var(--hd-ink) 14%, transparent)` |
| `--hd-track-inset` | `inset 0 1px 2px color-mix(in srgb, var(--hd-shadow-ink) var(--hd-sa-3), transparent)` |
| `--hd-track-fill` | `var(--hd-on)` |
| `--hd-track-fill-hi` | `var(--hd-g-fill-hi)` |
| `--hd-thumb` | `var(--hd-g-thumb)` |
| `--hd-thumb-edge` | `color-mix(in srgb, var(--hd-shadow-ink) 22%, transparent)` |
| `--hd-thumb-ring` | `0 0 0 0.5px var(--hd-thumb-edge)` |
| `--hd-thumb-shadow` | `var(--hd-thumb-ring), 0 1px 1px color-mix(in srgb, var(--hd-shadow-ink) var(--hd-sa-2), transparent), 0 3px 8px color-mix(in srgb, var(--hd-shadow-ink) var(--hd-sa-3), transparent)` |
| `--hd-pending-ring` | `conic-gradient(var(--hd-ink) 0 25%, transparent 25% 100%)` |
| `--hd-pending-ring-on` | `conic-gradient(var(--hd-on-ink) 0 25%, transparent 25% 100%)` |
| `--hd-value-colour` | `var(--hd-accent)` |
| `--hd-font-ui` | `"Inter", "Geist", system-ui, sans-serif` |
| `--hd-font-label` | `var(--hd-font-ui)` |
| `--hd-font-num` | `"Inter", "Geist", system-ui, sans-serif` |

**`.hd-root[data-hd-corners="square"] [data-hd-family="glass"], .hd-root[data-hd-corners="square"][data-hd-family="glass"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-g-bezel-r` | `0px` |
| `--hd-g-paddle-r` | `0px` |

**`.hd-widget[data-hd-family="glass"][data-hd-status="unavailable"] .hd-body`**

| Token | Frozen value |
| --- | --- |
| `--hd-ink-muted` | `var(--hd-ink)` |

### `ui/css/homedash-family-tactile.css`

**`[data-hd-family="tactile"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-t-panel-hi-d` | `#2A2E35` |
| `--hd-t-panel-lo-d` | `#1F2228` |
| `--hd-t-panel-mid-d` | `#24282E` |
| `--hd-t-panel-hi-l` | `#D9DDE3` |
| `--hd-t-panel-lo-l` | `#BFC4CC` |
| `--hd-t-panel-mid-l` | `#CCD0D7` |
| `--hd-t-bakelite-d` | `#1C1A18` |
| `--hd-t-bakelite-l` | `#ECE6D8` |
| `--hd-t-spec-d` | `14%` |
| `--hd-t-spec-l` | `22%` |
| `--hd-t-legend-d` | `#C9CDD3` |
| `--hd-t-legend-l` | `#33373D` |
| `--hd-t-muted-d` | `#9AA1AB` |
| `--hd-t-muted-l` | `#4A4F57` |
| `--hd-t-face-d` | `#101215` |
| `--hd-t-face-l` | `#F2EDE1` |
| `--hd-t-faceink-d` | `#E4E1D8` |
| `--hd-t-faceink-l` | `#23262B` |
| `--hd-t-emboss-d` | `0 -1px 0 color-mix(in srgb, #000000 60%, transparent)` |
| `--hd-t-emboss-l` | `0 1px 0 color-mix(in srgb, #FFFFFF 50%, transparent)` |
| `--hd-t-panel-hi` | `var(--hd-t-panel-hi-d)` |
| `--hd-t-panel-lo` | `var(--hd-t-panel-lo-d)` |
| `--hd-t-panel-mid` | `var(--hd-t-panel-mid-d)` |
| `--hd-t-bakelite-c` | `var(--hd-t-bakelite-d)` |
| `--hd-t-spec` | `var(--hd-t-spec-d)` |
| `--hd-t-legend` | `var(--hd-t-legend-d)` |
| `--hd-t-muted` | `var(--hd-t-muted-d)` |
| `--hd-t-emboss` | `var(--hd-t-emboss-d)` |
| `--hd-t-face` | `var(--hd-t-face-d)` |
| `--hd-t-faceink` | `var(--hd-t-faceink-d)` |
| `--hd-t-window` | `#07090B` |
| `--hd-t-amber` | `#FFB547` |
| `--hd-t-vfd` | `#FFC266` |
| `--hd-t-needle` | `#D2463A` |
| `--hd-t-led-green` | `#3DDC84` |
| `--hd-t-led-amber` | `#FFB547` |
| `--hd-t-led-red` | `#FF4D3D` |
| `--hd-t-led-base` | `#111111` |
| `--hd-t-knurl-a` | `#3A3E45` |
| `--hd-t-knurl-b` | `#5A606A` |
| `--hd-led` | `var(--hd-t-led-green)` |
| `--hd-t-brush` | `repeating-linear-gradient(180deg, transparent 0, color-mix(in srgb, #FFFFFF 5%, transparent) 2px, transparent 4px)` |
| `--hd-t-panel` | `var(--hd-t-brush), linear-gradient(180deg, var(--hd-t-panel-hi), var(--hd-t-panel-lo))` |
| `--hd-t-bakelite` | `radial-gradient(ellipse 70% 55% at 30% 20%, color-mix(in srgb, #FFFFFF var(--hd-t-spec), transparent), transparent 55%), var(--hd-t-bakelite-c)` |
| `--hd-t-steel` | `conic-gradient(from 18deg, #C7CCD4, #F3F5F8 11%, #A8AEB7 24%, #E4E7EC 37%, #979EA8 50%, #EFF1F4 63%, #B0B6BF 76%, #ECEFF3 89%, #C7CCD4)` |
| `--hd-t-steel-lin` | `linear-gradient(135deg, #E9ECF0, #B9BFC7 45%, #DDE1E6 60%, #A9AFB8)` |
| `--hd-t-grip` | `repeating-linear-gradient(90deg, color-mix(in srgb, #000000 28%, transparent) 0 1px, color-mix(in srgb, #FFFFFF 40%, transparent) 1px 2px) center / 10px 12px no-repeat` |
| `--hd-t-knurl` | `repeating-conic-gradient(var(--hd-t-knurl-a) 0deg 2.6deg, var(--hd-t-knurl-b) 3.4deg 5.6deg, var(--hd-t-knurl-a) 6.4deg 7.2deg)` |
| `--hd-t-ring-86` | `radial-gradient(farthest-side, transparent 86%, #000000 87%)` |
| `--hd-t-ring-92` | `radial-gradient(farthest-side, transparent 91.5%, #000000 92.5%)` |
| `--hd-t-pointer` | `linear-gradient(180deg, var(--hd-t-faceink-l) 0 62%, transparent 62%)` |
| `--hd-t-led` | `radial-gradient(circle at 36% 30%, color-mix(in srgb, var(--hd-led) 35%, #FFFFFF) 0 16%, var(--hd-led) 46%, color-mix(in srgb, var(--hd-led) 55%, #000000) 100%)` |
| `--hd-t-led-off` | `radial-gradient(circle at 36% 30%, color-mix(in srgb, #FFFFFF 12%, transparent) 0 14%, transparent 30%), color-mix(in srgb, var(--hd-led) 22%, var(--hd-t-led-base))` |
| `--hd-t-led-halo` | `0 0 6px 1px color-mix(in srgb, var(--hd-led) 60%, transparent)` |
| `--hd-t-dot-off-d` | `color-mix(in srgb, var(--hd-t-amber) 18%, var(--hd-t-led-base))` |
| `--hd-t-dot-off-l` | `color-mix(in srgb, var(--hd-t-amber) 10%, #8B9099)` |
| `--hd-t-dot-off` | `var(--hd-t-dot-off-d)` |
| `--hd-t-hl-d` | `26%` |
| `--hd-t-hl-l` | `60%` |
| `--hd-t-dn-d` | `45%` |
| `--hd-t-dn-l` | `22%` |
| `--hd-t-hl` | `var(--hd-t-hl-d)` |
| `--hd-t-dn` | `var(--hd-t-dn-d)` |
| `--hd-t-persp` | `300px` |
| `--hd-t-tilt-up` | `-12deg` |
| `--hd-t-tilt-dn` | `12deg` |
| `--hd-t-half-up` | `radial-gradient(ellipse 85% 70% at 28% 24%, color-mix(in srgb, #FFFFFF var(--hd-t-hl), transparent), transparent 62%), linear-gradient(180deg, color-mix(in srgb, #FFFFFF 5%, transparent), transparent 55%)` |
| `--hd-t-half-down` | `linear-gradient(color-mix(in srgb, #000000 var(--hd-t-dn), transparent), color-mix(in srgb, #000000 var(--hd-t-dn), transparent)), var(--hd-t-bakelite-c)` |
| `--hd-t-edge-low` | `inset 0 -2px 0 color-mix(in srgb, #000000 38%, transparent)` |
| `--hd-t-pilot` | `radial-gradient(circle at 36% 30%, color-mix(in srgb, var(--hd-t-amber) 35%, #FFFFFF) 0 20%, var(--hd-t-amber) 55%, color-mix(in srgb, var(--hd-t-amber) 55%, #000000) 100%)` |
| `--hd-t-shade` | `color-mix(in srgb, #000000 12%, transparent)` |
| `--hd-t-needle-shadow` | `color-mix(in srgb, #000000 30%, transparent)` |
| `--hd-t-grid-ink` | `color-mix(in srgb, var(--hd-t-amber) 10%, transparent)` |
| `--hd-t-hatch` | `repeating-linear-gradient(135deg, color-mix(in srgb, var(--hd-ink) 6%, transparent) 0 2px, transparent 2px 6px)` |
| `--hd-t-vignette` | `radial-gradient(circle, transparent 82%, color-mix(in srgb, #000000 6%, transparent) 100%)` |
| `--hd-t-reflection` | `radial-gradient(ellipse 60% 30% at 34% 24%, color-mix(in srgb, #FFFFFF 8%, transparent), transparent 70%)` |
| `--hd-t-hairline` | `color-mix(in srgb, #000000 30%, var(--hd-t-panel-mid))` |
| `--hd-t-card-border` | `1px solid color-mix(in srgb, #000000 35%, var(--hd-t-panel-lo))` |
| `--hd-t-dash-unavail` | `1px dashed color-mix(in srgb, #000000 35%, var(--hd-t-panel-lo))` |
| `--hd-t-shadow-card` | `inset 1px 1px 0 color-mix(in srgb, #FFFFFF 10%, transparent), inset -1px -1px 0 color-mix(in srgb, #000000 30%, transparent), 2px 3px 8px color-mix(in srgb, #000000 var(--hd-sa-4), transparent)` |
| `--hd-t-shadow-contact` | `1px 2px 3px color-mix(in srgb, #000000 var(--hd-sa-4), transparent)` |
| `--hd-shadow-cast` | `2px 4px 10px color-mix(in srgb, #000000 var(--hd-sa-5), transparent)` |
| `--hd-recess` | `inset 1px 2px 4px color-mix(in srgb, #000000 60%, transparent), inset -1px -1px 0 color-mix(in srgb, #FFFFFF 8%, transparent)` |
| `--hd-window-shadow` | `inset 0 2px 6px color-mix(in srgb, #000000 70%, transparent)` |
| `--hd-lip` | `inset 1px 1px 0 color-mix(in srgb, #FFFFFF 22%, transparent), inset -1px -1px 0 color-mix(in srgb, #000000 30%, transparent)` |
| `--hd-vfd-glow` | `0 0 6px color-mix(in srgb, var(--hd-t-amber) 50%, transparent)` |
| `--hd-stroke-hair` | `1px` |
| `--hd-stroke-1` | `1.5px` |
| `--hd-stroke-2` | `2px` |
| `--hd-stroke-arc` | `0.06` |
| `--hd-stroke-tick` | `1.25px` |
| `--hd-t-needle-w` | `2.5` |
| `--hd-th-stroke` | `12` |
| `--hd-font-vfd` | `"Geist Mono", "JetBrains Mono", monospace` |
| `--hd-num-features` | `"tnum" 1` |
| `--hd-num-weight` | `600` |
| `--hd-num-weight-small` | `600` |
| `--hd-num-tracking` | `0em` |
| `--hd-num-cap` | `0.74` |
| `--hd-num-hero-max` | `72px` |
| `--hd-unit-scale` | `0.42` |
| `--hd-label-size` | `12px` |
| `--hd-label-weight` | `600` |
| `--hd-label-tracking` | `0.06em` |
| `--hd-dur-toggle` | `120ms` |
| `--hd-ease-toggle` | `var(--hd-ease-snap)` |
| `--hd-dur-led` | `60ms` |
| `--hd-dur-fill` | `60ms` |
| `--hd-dur-xfade` | `60ms` |
| `--hd-dur-remote` | `220ms` |
| `--hd-ease-remote` | `var(--hd-ease-out)` |
| `--hd-dur-needle-f` | `1100ms` |
| `--hd-dur-zone` | `60ms` |

**`.hd-widget[data-hd-family="tactile"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-surface-solid` | `var(--hd-t-panel-mid)` |
| `--hd-surface` | `var(--hd-t-panel-mid)` |
| `--hd-ink` | `var(--hd-t-legend)` |
| `--hd-ink-muted` | `var(--hd-t-muted)` |
| `--hd-ink-faint` | `color-mix(in srgb, var(--hd-t-legend) 45%, var(--hd-t-panel-mid))` |
| `--hd-sunken` | `var(--hd-t-window)` |
| `--hd-focus-gap` | `var(--hd-t-panel-mid)` |
| `--hd-hairline` | `var(--hd-t-hairline)` |
| `--hd-card-bg` | `var(--hd-t-panel)` |
| `--hd-card-border` | `var(--hd-t-card-border)` |
| `--hd-dash-unavail` | `var(--hd-t-dash-unavail)` |
| `--hd-radius-card` | `calc(12px * var(--hd-radius-scale))` |
| `--hd-radius-control` | `calc(6px * var(--hd-radius-scale))` |
| `--hd-t-bezel-r` | `calc(8px * var(--hd-radius-scale))` |
| `--hd-shadow-card` | `var(--hd-t-shadow-card)` |
| `--hd-shadow-contact` | `var(--hd-t-shadow-contact)` |
| `--hd-ring-alert` | `inset 0 0 0 1px var(--hd-t-led-red)` |
| `--hd-font-ui` | `"Inter", system-ui, sans-serif` |
| `--hd-font-label` | `"HomeDash Barlow Condensed", "Inter", sans-serif` |
| `--hd-font-num` | `"HomeDash Barlow Condensed", "Inter", sans-serif` |
| `--hd-unavailable` | `var(--hd-ink-faint)` |
| `--hd-thumb-edge` | `var(--hd-hairline)` |
| `--hd-track` | `color-mix(in srgb, var(--hd-ink) 14%, transparent)` |
| `--hd-ok-ink` | `var(--hd-ok)` |
| `--hd-warn-ink` | `var(--hd-warn)` |
| `--hd-alert-ink` | `var(--hd-alert)` |
| `--hd-info-ink` | `var(--hd-info)` |

**`.hd-root[data-hd-scheme="light"] .hd-widget[data-hd-family="tactile"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-ok-ink` | `color-mix(in srgb, var(--hd-ok) 34%, var(--hd-ink))` |
| `--hd-warn-ink` | `color-mix(in srgb, var(--hd-warn) 34%, var(--hd-ink))` |
| `--hd-alert-ink` | `color-mix(in srgb, var(--hd-alert) 34%, var(--hd-ink))` |
| `--hd-info-ink` | `color-mix(in srgb, var(--hd-info) 34%, var(--hd-ink))` |

**`.hd-root[data-hd-scheme="light"] [data-hd-family="tactile"], .hd-root[data-hd-scheme="light"][data-hd-family="tactile"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-t-panel-hi` | `var(--hd-t-panel-hi-l)` |
| `--hd-t-panel-lo` | `var(--hd-t-panel-lo-l)` |
| `--hd-t-panel-mid` | `var(--hd-t-panel-mid-l)` |
| `--hd-t-bakelite-c` | `var(--hd-t-bakelite-l)` |
| `--hd-t-spec` | `var(--hd-t-spec-l)` |
| `--hd-t-legend` | `var(--hd-t-legend-l)` |
| `--hd-t-muted` | `var(--hd-t-muted-l)` |
| `--hd-t-emboss` | `var(--hd-t-emboss-l)` |
| `--hd-t-face` | `var(--hd-t-face-l)` |
| `--hd-t-faceink` | `var(--hd-t-faceink-l)` |
| `--hd-t-dot-off` | `var(--hd-t-dot-off-l)` |
| `--hd-t-hl` | `var(--hd-t-hl-l)` |
| `--hd-t-dn` | `var(--hd-t-dn-l)` |

**`.hd-widget[data-hd-family="tactile"][data-hd-pending="true"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-led` | `var(--hd-t-led-amber)` |

**`.hd-widget[data-hd-family="tactile"][data-hd-status="error"], .hd-widget[data-hd-family="tactile"][data-hd-tone="crit"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-led` | `var(--hd-t-led-red)` |

**`.hd-widget[data-hd-family="tactile"][data-hd-status="unavailable"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-card-bg` | `var(--hd-t-hatch), var(--hd-t-panel)` |

**`.hd-widget[data-hd-family="tactile"][data-hd-readonly="true"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-shadow-cast` | `0 0 0 0 transparent` |

**`.hd-widget[data-hd-family="tactile"] .hd-toggle`**

| Token | Frozen value |
| --- | --- |
| `--hd-tgl-w` | `48px` |
| `--hd-tgl-h` | `14px` |
| `--hd-tgl-travel` | `20px` |

**`.hd-root[data-hd-corners="square"] .hd-widget[data-hd-family="tactile"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-radius-card` | `0px` |
| `--hd-radius-control` | `0px` |
| `--hd-t-bezel-r` | `0px` |

### `ui/css/homedash-family-minimal.css`

**`[data-hd-family="minimal"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-stroke-hair` | `1px` |
| `--hd-stroke-1` | `1.5px` |
| `--hd-stroke-2` | `2px` |
| `--hd-stroke-arc` | `0.04` |
| `--hd-stroke-tick` | `1px` |
| `--hd-m-arc-w` | `3px` |
| `--hd-m-spark-w` | `1.25px` |
| `--hd-m-dot-r` | `4` |
| `--hd-m-arc-u` | `4` |
| `--hd-num-features` | `"tnum" 1` |
| `--hd-num-weight` | `300` |
| `--hd-num-weight-small` | `500` |
| `--hd-num-tracking` | `-0.035em` |
| `--hd-num-cap` | `0.71` |
| `--hd-num-hero-max` | `128px` |
| `--hd-unit-scale` | `0.42` |
| `--hd-dial-num-k` | `0.28` |
| `--hd-thermo-num-k` | `0.26` |
| `--hd-label-size` | `12.5px` |
| `--hd-label-weight` | `500` |
| `--hd-label-tracking` | `0.01em` |
| `--hd-dur-toggle` | `160ms` |
| `--hd-ease-toggle` | `var(--hd-ease-standard)` |
| `--hd-dur-fill` | `160ms` |
| `--hd-dur-xfade` | `160ms` |
| `--hd-dur-remote` | `160ms` |
| `--hd-ease-remote` | `var(--hd-ease-standard)` |
| `--hd-dur-needle-f` | `420ms` |
| `--hd-dur-zone` | `160ms` |
| `--hd-th-stroke` | `12` |

**`.hd-widget[data-hd-family="minimal"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-surface` | `var(--hd-surface-solid)` |
| `--hd-card-bg` | `var(--hd-surface-solid)` |
| `--hd-card-border` | `1px solid var(--hd-hairline)` |
| `--hd-dash-unavail` | `1px dashed var(--hd-hairline)` |
| `--hd-radius-card` | `var(--hd-theme-radius)` |
| `--hd-radius-control` | `var(--hd-corners-control)` |
| `--hd-edge-hi` | `none` |
| `--hd-edge-lo` | `none` |
| `--hd-shadow-contact` | `none` |
| `--hd-shadow-card` | `none` |
| `--hd-shadow-press` | `none` |
| `--hd-ring-alert` | `none` |
| `--hd-track` | `transparent` |
| `--hd-track-line` | `var(--hd-ink)` |
| `--hd-track-fill` | `var(--hd-ink)` |
| `--hd-thumb` | `var(--hd-ink)` |
| `--hd-thumb-on` | `var(--hd-surface-solid)` |
| `--hd-value-colour` | `var(--hd-accent)` |
| `--hd-font-ui` | `"Geist", "Inter", system-ui, sans-serif` |
| `--hd-font-label` | `var(--hd-font-ui)` |
| `--hd-font-num` | `var(--hd-font-ui)` |
| `--hd-m-rect-r` | `min(var(--hd-corners-control), 8px)` |

**`.hd-widget[data-hd-family="minimal"][data-hd-status="unavailable"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-ink` | `var(--hd-ink-muted)` |

**`.hd-widget[data-hd-family="minimal"] .hd-toggle`**

| Token | Frozen value |
| --- | --- |
| `--hd-tgl-inset` | `6px` |
| `--hd-m-ring` | `var(--hd-ink)` |

**`.hd-widget[data-hd-family="minimal"] .hd-toggle[aria-checked="true"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-m-ring` | `var(--hd-thumb-on)` |

**`.hd-widget[data-hd-family="minimal"] .hd-dial`**

| Token | Frozen value |
| --- | --- |
| `--hd-m-d` | `min(var(--hd-body-w), var(--hd-body-h))` |

**`.hd-widget[data-hd-family="minimal"][data-hd-type="climate"] .hd-dial`**

| Token | Frozen value |
| --- | --- |
| `--hd-m-d` | `var(--hd-th-d)` |

### `ui/css/homedash-family-instrument.css`

**`[data-hd-family="instrument"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-i-panel-hi` | `#0B0F14` |
| `--hd-i-panel-lo` | `#07090C` |
| `--hd-i-mid` | `#090C10` |
| `--hd-i-window` | `#040608` |
| `--hd-i-ink` | `#D5DEE8` |
| `--hd-i-muted` | `#8B97A6` |
| `--hd-i-faint` | `#4A5361` |
| `--hd-i-ok` | `#4AD29C` |
| `--hd-i-warn` | `#F2B84B` |
| `--hd-i-alert` | `#FF7A72` |
| `--hd-i-info` | `#5AA9FF` |
| `--hd-i-hud` | `var(--hd-hud, color-mix(in srgb, var(--hd-accent) 85%, #FFFFFF))` |
| `--hd-i-val` | `var(--hd-i-hud)` |
| `--hd-i-br` | `color-mix(in srgb, var(--hd-i-hud) 70%, transparent)` |
| `--hd-i-corners` | `linear-gradient(var(--hd-i-br), var(--hd-i-br)) left 0 top 0 / 10px 2px no-repeat, linear-gradient(var(--hd-i-br), var(--hd-i-br)) left 0 top 0 / 2px 10px no-repeat, linear-gradient(var(--hd-i-br), var(--hd-i-br)) right 0 top 0 / 10px 2px no-repeat, linear-gradient(var(--hd-i-br), var(--hd-i-br)) right 0 top 0 / 2px 10px no-repeat, linear-gradient(var(--hd-i-br), var(--hd-i-br)) left 0 bottom 0 / 10px 2px no-repeat, linear-gradient(var(--hd-i-br), var(--hd-i-br)) left 0 bottom 0 / 2px 10px no-repeat, linear-gradient(var(--hd-i-br), var(--hd-i-br)) right 0 bottom 0 / 10px 2px no-repeat, linear-gradient(var(--hd-i-br), var(--hd-i-br)) right 0 bottom 0 / 2px 10px no-repeat` |
| `--hd-i-scan` | `repeating-linear-gradient(0deg, transparent 0, color-mix(in srgb, #000000 22%, transparent) 1px, transparent 2px 3px)` |
| `--hd-i-glowline` | `0 0 10px color-mix(in srgb, var(--hd-i-val) 55%, transparent)` |
| `--hd-i-ghost` | `color-mix(in srgb, var(--hd-i-hud) 7%, transparent)` |
| `--hd-i-key-wash` | `color-mix(in srgb, var(--hd-i-val) 9%, transparent)` |
| `--hd-i-key-edge` | `color-mix(in srgb, var(--hd-i-val) 45%, transparent)` |
| `--hd-i-gap-dash` | `0 20 5 0` |
| `--hd-i-mount-l` | `0 1px 2px color-mix(in srgb, #000000 30%, transparent), 0 6px 16px -6px color-mix(in srgb, #000000 35%, transparent)` |
| `--hd-stroke-hair` | `1px` |
| `--hd-stroke-1` | `1.5px` |
| `--hd-stroke-2` | `2px` |
| `--hd-stroke-arc` | `0.08` |
| `--hd-stroke-tick` | `1.25px` |
| `--hd-i-glow-add` | `2` |
| `--hd-i-gap-add` | `2` |
| `--hd-th-stroke` | `12` |
| `--hd-i-needle-glow-u` | `3` |
| `--hd-num-features` | `"tnum" 1` |
| `--hd-num-weight` | `400` |
| `--hd-num-weight-small` | `400` |
| `--hd-num-tracking` | `0em` |
| `--hd-num-cap` | `0.74` |
| `--hd-num-hero-max` | `96px` |
| `--hd-unit-scale` | `0.42` |
| `--hd-label-size` | `11.5px` |
| `--hd-label-weight` | `500` |
| `--hd-label-tracking` | `0.12em` |
| `--hd-dur-toggle` | `110ms` |
| `--hd-ease-toggle` | `var(--hd-ease-hud)` |
| `--hd-dur-flash` | `90ms` |
| `--hd-dur-fill` | `110ms` |
| `--hd-dur-xfade` | `110ms` |
| `--hd-dur-remote` | `220ms` |
| `--hd-ease-remote` | `var(--hd-ease-hud)` |
| `--hd-dur-needle-f` | `880ms` |
| `--hd-dur-chase` | `80ms` |
| `--hd-dur-zone` | `110ms` |
| `--hd-dur-dip` | `180ms` |

**`.hd-widget[data-hd-family="instrument"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-surface-solid` | `var(--hd-i-mid)` |
| `--hd-surface` | `var(--hd-i-mid)` |
| `--hd-sunken` | `var(--hd-i-window)` |
| `--hd-ink` | `var(--hd-i-ink)` |
| `--hd-ink-muted` | `var(--hd-i-muted)` |
| `--hd-ink-faint` | `var(--hd-i-faint)` |
| `--hd-ok` | `var(--hd-i-ok)` |
| `--hd-warn` | `var(--hd-i-warn)` |
| `--hd-alert` | `var(--hd-i-alert)` |
| `--hd-info` | `var(--hd-i-info)` |
| `--hd-ok-ink` | `var(--hd-i-ok)` |
| `--hd-warn-ink` | `var(--hd-i-warn)` |
| `--hd-alert-ink` | `var(--hd-i-alert)` |
| `--hd-info-ink` | `var(--hd-i-info)` |
| `--hd-hairline` | `color-mix(in srgb, var(--hd-i-hud) 18%, transparent)` |
| `--hd-focus` | `var(--hd-i-hud)` |
| `--hd-focus-gap` | `var(--hd-i-mid)` |
| `--hd-card-bg` | `linear-gradient(180deg, var(--hd-i-panel-hi), var(--hd-i-panel-lo))` |
| `--hd-card-border` | `1px solid color-mix(in srgb, var(--hd-i-hud) 18%, transparent)` |
| `--hd-dash-unavail` | `1px dashed color-mix(in srgb, var(--hd-i-hud) 30%, transparent)` |
| `--hd-radius-card` | `calc(4px * var(--hd-radius-scale))` |
| `--hd-radius-control` | `calc(3px * var(--hd-radius-scale))` |
| `--hd-edge-hi` | `none` |
| `--hd-edge-lo` | `none` |
| `--hd-shadow-contact` | `none` |
| `--hd-shadow-card` | `none` |
| `--hd-shadow-press` | `none` |
| `--hd-ring-alert` | `none` |
| `--hd-glow` | `var(--hd-i-glowline)` |
| `--hd-font-ui` | `"Oxanium", "Geist", system-ui, sans-serif` |
| `--hd-font-label` | `var(--hd-font-ui)` |
| `--hd-font-num` | `"HomeDash Share Tech Mono", "Geist Mono", monospace` |

**`.hd-root[data-hd-scheme="light"] .hd-widget[data-hd-family="instrument"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-shadow-card` | `var(--hd-i-mount-l)` |

**`.hd-widget[data-hd-family="instrument"][data-hd-tone="crit"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-i-val` | `var(--hd-i-alert)` |

**`.hd-widget[data-hd-family="instrument"][data-hd-stale="true"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-i-val` | `var(--hd-ink-muted)` |

**`.hd-widget[data-hd-family="instrument"] .hd-dial-readout > .hd-seg7`**

| Token | Frozen value |
| --- | --- |
| `--hd-i-seg-h` | `calc(min(var(--hd-body-w), var(--hd-body-h)) * 0.2)` |

**`.hd-widget[data-hd-family="instrument"] .hd-gauge[data-hd-gauge="arc"] .hd-gauge-readout .hd-seg7`**

| Token | Frozen value |
| --- | --- |
| `--hd-i-seg-h` | `0.66em` |

**`.hd-widget[data-hd-family="instrument"] .hd-gauge[data-hd-gauge="needle"] .hd-gauge-readout .hd-seg7`**

| Token | Frozen value |
| --- | --- |
| `--hd-i-seg-h` | `1em` |

**`.hd-widget[data-hd-family="instrument"] .hd-tread .hd-seg7`**

| Token | Frozen value |
| --- | --- |
| `--hd-i-seg-h` | `min(calc(var(--hd-body-h) * 0.92), calc((var(--hd-body-w) - 24px) / 3.2))` |

**`.hd-widget[data-hd-family="instrument"] .hd-toggle`**

| Token | Frozen value |
| --- | --- |
| `--hd-tgl-w` | `56px` |
| `--hd-tgl-h` | `30px` |

**`.hd-widget[data-hd-family="instrument"][data-hd-type="climate"] .hd-dial-readout > .hd-seg7`**

| Token | Frozen value |
| --- | --- |
| `--hd-i-seg-h` | `calc(var(--hd-th-d) * 0.18)` |

**`.hd-widget[data-hd-family="instrument"][data-hd-type="climate"] .hd-dial-sub .hd-seg7`**

| Token | Frozen value |
| --- | --- |
| `--hd-i-seg-h` | `max(12px, calc(var(--hd-th-d) * 0.08))` |

**`.hd-widget[data-hd-family="instrument"][data-hd-type="sensor"] .hd-body > .hd-seg7`**

| Token | Frozen value |
| --- | --- |
| `--hd-i-seg-h` | `15px` |

**`.hd-root[data-hd-corners="square"] .hd-widget[data-hd-family="instrument"]`**

| Token | Frozen value |
| --- | --- |
| `--hd-radius-card` | `0px` |
| `--hd-radius-control` | `0px` |

## Task 1 additions and engine helpers

### Primitive additions

Every addition defaults to the M0 markup. Defaults leave the M0 output unchanged. Markup is what the family CSS depends on.

| Builder | Option (default) | Markup added | `set()` keys or notes |
| --- | --- | --- | --- |
| toggle | `decor` (false) | `span.hd-toggle-key` in the track before the thumb; after the track `span.hd-toggle-legend[data-glyph=off]` "O", `span.hd-toggle-legend[data-glyph=on]` "I", `span.hd-toggle-led` | key text "I" or "O" from `on` |
| knob | `glow` (false) | `path.hd-dial-glow[pathLength=1000]` before the arc | same dash array as the arc |
| knob | `gaps` (0) | `path.hd-dial-gaps[pathLength=1000]` last in the svg | none |
| knob | `lead` (false; needs `quantize`) | `path.hd-dial-lead` after the arc | `d` of the last lit segment; empty when none |
| knob | `dots` (null) `{count, r, size, lit}` | `path.hd-dial-dots` and `path.hd-dial-dots-lit` first in the svg | lit `d` = first round(frac x count) circles when `lit` |
| knob | `ticksMajor` (0) | `path.hd-dial-ticks-major` | none |
| knob | `numerals` (0, at most 6) | `text.hd-dial-numeral` at radius 67 | none |
| knob | `hardware` (false) | `span.hd-dial-knurl` before `span.hd-dial-cap` | none |
| knob | `needle` (false; needs `activeSegment`) | `path.hd-dial-needle` with `d="M100 14V42"` in `g.hd-dial-current` | none |
| knob | `handleShadow` (false) | `circle.hd-dial-knob-shadow` first in `g.hd-dial-handle` | none |
| knob | `word` (false) | `span.hd-dial-word` in the readout | `word` (string or null) |
| knob | `extra`, `subReadout`, `subIcon`, `captions` | readout children in order: `span.hd-dial-caption[data-for=main]`, `span.hd-dial-action`, main readout, `span.hd-dial-sub`, `span.hd-dial-caption[data-for=sub]` | `action` (text), `sub` (number or null) |
| segmented | `rotary` (false) | `span.hd-seg-rotary[aria-hidden=true] > span.hd-seg-rotary-pointer`, last child | none; Tactile climate passes `rotary: false` |
| needleGauge | `face` (false) | `span.hd-gauge-face[aria-hidden=true] > span.hd-gauge-bezel, span.hd-gauge-glass` before the svg | none |
| needleGauge | `needleFx` (false) | `g.hd-gauge-needle-shadow > path` before `g.hd-gauge-needle`; `path.hd-gauge-needle-glow` first in the needle group | none |
| needleGauge, arcGauge | `readout` (numeral or seg7) | `span.hd-gauge-readout` | readout value |
| arcGauge | `glow`, `gaps`, `lead` | `path.hd-gauge-glow[pathLength=1000]` before the arc; `path.hd-gauge-lead` after it; `path.hd-gauge-gaps[pathLength=1000]` last | glow dash array; lead `d` |
| arcGauge | `zoneOverlay` (false) | `path.hd-gauge-arc-zone[pathLength=1000]` after the arc | same dash array as the arc |
| numeral | `trendIcon` (false) | `span.hd-num-trend` holds `svg.hd-ico` (`trend-up`, `trend-down`, `trend-flat`) instead of the text glyph | icon swap; `data-trend` unchanged |
| sparkline | `fade` (false) | area path moves into `svg.hd-spark-fill[aria-hidden=true]`, returned as extra field `fill` | both `d` and size |
| sparkline | `grid` (null) `[cols, rows]` | `path.hd-spark-grid` first in `el` | interior lines at rounded positions |
| sparkline | `halo` (false) | `circle.hd-spark-halo[r=6]` before `.hd-spark-last` | `cx`, `cy` and `hidden` like the end dot |

Icons added in Task 1 (24 px grid, 1.75 stroke): `power`, `mode-auto`, `plug`, `cpu`, `disk`, `fan-heater`, `trend-up`,
`trend-down`, `trend-flat`.

Values changed after Task 1 (from the gate records): the colour token is `--hd-value-colour` (the fraction `--hd-value` is
the 0-1 value the primitives write). `--hd-m-arc-u` is the user-unit arc width for Minimal.

### Engine helpers

- **`E.colour.hudFor(hex)`** (pure, `hd-colour.js`): the HUD ink for the Instrument family. Returns lowercase `#rrggbb`.
  An unparseable input or an accent with OKLCH chroma below 0.04 returns `#e8edf2` (white phosphor). Otherwise lightness starts at
  0.72 and rises in 0.02 steps (cap 0.97) until the contrast on `#090c10` is at least 4.5:1. Chroma drops in 0.01 steps while the
  colour is outside sRGB.
- **`E.colour.ensureContrast(hex, against, min)`** (pure): `against` is one hex or an array. The input comes back unchanged when its
  worst-case contrast already reaches `min`. Otherwise OKLCH lightness moves in 0.01 steps toward whichever extreme (black or
  white) gives the better worst case; hue and chroma are kept. If no lightness reaches `min`, the best extreme is returned.
  Unparseable input comes back unchanged.
- **`E.widgets.applyLook(root)`** (`hd-widgets.js`): resolves the look on `root` (`.hd-root`):
  - Resolves colours through a 1x1 canvas, so `oklch()`, `color()` and `color-mix()` values work. Writes `--hd-hud` from
    `hudFor(accent)`, and removes it when the accent does not resolve.
  - Checks `--hd-accent`, `--hd-light-on` and `--hd-focus` against the card (`--hd-surface-solid`; for Glass the card is that
    colour at `--hd-glass-alpha-s` over `--hd-canvas`). Focus is also checked against the canvas. A value under 3:1 is replaced by
    an inline value from `ensureContrast` (target 3.05:1, same hue). A corrected accent also gets black or white
    `--hd-ink-on-accent` when the theme ink is under 4.5:1 on it.
  - Removes the inline values it wrote on the previous call first, so a theme switch back is clean.
  - Sets `data-hd-corners="square"` when every number in `--vd-mat-card-radius` is 0; removes the attribute otherwise.
- **`E.widgets.heroStatus.staleText(api, e)`** (`hd-widgets.js`): "Updated N min ago" in whole minutes (at least 1). Returns
  `null` when `updated_at` does not parse, so the hero text is shown instead. Shared by the four hero modules.

## M2 additions (desktop host, view-only)

- **Page part.** `ui/css/homedash-page.css` is bundle part 10, the last part. It holds the scroll stage, the page placement from
  `--hd-x`/`--hd-y`/`--hd-w`/`--hd-h`, the page tabs, the empty-page note and the unsupported cell. It reads `--hd-*` tokens only.
- **Host chrome.** The desktop host's bar, banners, home list, state panel and confirm sheet (`ui/css/desktop-app-homedash.css`) are
  painted from the desktop theme (`--vd-mat-*` with `--vd-theme-*` and `--ds-*` fallbacks), not from HomeDash tokens. The dashboard
  canvas stays the engine's `.hd-root`.
- **SCR-M2-1 (entity tile geometry and duration text).** The `entity` tile in `homedash-widgets.css` places its numeral top-aligned
  with the sensor numerals and its state glyph bottom-left at the card inset. Stale values use the stale ink. Duration sensors read
  through the duration format (lead rulings below). In Instrument the numeral and glyph paint in the HUD readout colour (`--hd-i-val`).
- **SCR-M2-2 (sparkline live tail).** The `sensor` sparkline keeps the points that arrive after its history as a live tail, so a new
  live value moves the line without a new history request. The store appends live numeric states to the cached series.
- **SCR-M2-3 (decimals).** `options.decimals` (integer 0 to 3) sets the readout precision of the gauge and sensor widgets. Any other
  value keeps 0. The temperature widget keeps its 1 decimal.

## M3 additions

Gates W-S, W-G, W-T, W-M, W-I, W-X and P all passed (2026-11-25 and 2026-11-26). Each finding and its round is in the lead rulings
below. The gate records are local and git-ignored.

- **W1 build sheets.** A cell is 96 px with a 12 px gap. A "1-row" frame is a height of 1 with two or more columns. Its frame
  becomes `"head state" var(--hd-head-h) / "body body" 1fr` (SCR-M3-5), so the body is 50 px at 2×1. A 1×1 keeps three rows
  (44 px body). The variants and their sizes:

  | Variant | Sizes | Body |
  | --- | --- | --- |
  | switch/push | 1×1, 2×1 | `button.hd-push[data-hd-variant=chip][role=switch]`, 44 px high, icon 20 px with the On/Off word (icon only at 1×1, lead rule A3); 2×1 is `min(100%,152px)` at the start |
  | switch/tile | 1×1, 2×2 | `button.hd-push[data-hd-variant=tile][role=switch]` fills the body; icon 28 px (40 px at 200 px tall or more); label visually hidden |
  | light/slider | 2×1, 4×1 | `.hd-lrow`: power chip 44×44 · slider (flex, 44 px hit, 6 px track) · numeral 4.5ch (hidden below 300 px) |
  | light/fader | 1×2, 1×3 | `.hd-lcol`: numeral (only at 260 px tall or more) · vertical slider (min 64 px, cap 40×24) · power chip 44×44 |
  | light/tile | 1×1, 2×2 | `.hd-ltile` fills the body: wash `scaleX(--hd-value)` from the left, icon badge top-left (hidden at 1×1), numeral bottom-left (22cqmin; 18 px at 1×1) |
  | light/dial | M1 H3 | unchanged geometry; a centre tap toggles |
  | action/button | 1×1, 2×1 | `button.hd-push[data-hd-variant=chip]`, 44 px high: kind icon (scene, script or button) and the verb (Activate, Run or Press); check icon for 600 ms after success |
  | select/segmented | 2×1, 3×1 | `.hd-seg` full width (overrides the H4 260 px cap); more options than the capacity render the list control (lead rule A2) |
  | select/list | 1×1, 2×1 | `button.hd-pick`, 44 px high, full width: value (ellipsis) and chevron 16 px |
  | number/stepper | 2×1, 3×1 | `.hd-stepper` centred: − 44 · value numeral 4ch with unit · + 44 |
  | number/slider | 2×1, 4×1 | `.hd-nrow`: slider (flex) · numeral 5ch with unit |
  | light sheet | 7×6 (744×636), 4×7 (420×744) | `.hd-sheet.hd-widget[data-kind=moreinfo]`, 640×600 at most, centred; below 600 px root width a bottom sheet (full width, 85 % high at most) |
  | listbox popover | — | `.hd-sheet.hd-widget[data-kind=listbox]` in `.hd-pop-layer`; width `max(anchor, 200px)`; rows 44 px; a check on the selected row; 352 px high at most |

- **State table per family.** The shared state language of §7, as the four families paint it.

  | State | Glass | Tactile | Minimal | Instrument |
  | --- | --- | --- | --- | --- |
  | pending (after 250 ms) | conic sweep ring on the thumb or icon disc | LED amber 2 Hz blink (static amber under reduced motion) | 270° hairline ring rotating; "…" word | segment chase or blinking lead; static half-lit under reduced motion |
  | armed | accent ring around the control and `.hd-arm` bar (accent) | amber LED blink; bar hidden | text only | HUD ring and bar (`--hd-i-val`) |
  | error cue | shake 6 px, 3 cycles, 320 ms | red LED, 3 blinks | shake 4 px | alert flash (HUD to alert, 6 cycles at most) |
  | confirmed | thumb or disc pulse 1 → 1.06 → 1 in 180 ms, plus check glyph | steady LED and check | check only | 90 ms flash and check |
  | drag | thumb ×1.12, deeper shadow | cap pressed 1 px, shadow shift | dot ×1.25 | lead segment glow |
  | on (tile) | `--hd-on` wash 22 %, disc filled `--hd-on` with on-ink glyph | LED lit, legend "I" | disc ink-filled, glyph inverted | HUD top bar and glow |

  `.hd-arm` defaults to the accent colour. Tactile and Minimal hide it (lead ruling, W-S round 1).

- **Structure change requests (SCR-M3-1 to SCR-M3-8).** SCR-M3-1 to SCR-M3-7 were approved by the lead (Q4, 2026-11-25). SCR-M3-8 was
  approved with decision D8.
  - **SCR-M3-1** Head icon for every widget: `widget.icon`, then the entity's `icon_hint`, then the kind icon. The M1 hero heads
    gain icons at 2×1 and larger. Minimal hides head icons, and 1×1 hides them too.
  - **SCR-M3-2** `frame({more})` puts the more glyph in `.hd-more`: 44×44, top right, hidden at 1×1. The host wires it to `moreInfo`.
  - **SCR-M3-3** Overlays (the more-info dialog and the listbox panel) carry `hd-widget` and `data-hd-family`, so the family tokens
    and paint apply. `.hd-sheet[data-kind]` is the popover sheet primitive of §3, the only second blur.
  - **SCR-M3-4** Primitive input contract: an optional `input` on toggle, rocker, pushButton, slider, knob, stepper, segmented, ctStrip
    and colourWheel, `pushButton({toggle})` (role switch), `set({icon})`, and the new primitive `picker`. Without `input` the markup is
    unchanged. The callbacks are in the engine guide.
  - **SCR-M3-5** 1-row frame layout for the W1 types at height 1 with two or more columns (see the build sheets above).
  - **SCR-M3-6** Production structure for slider, CT strip, colour wheel, stepper and picker, which had none before. The slider thumb
    moves with `transform` in track container units, so value changes can transition.
  - **SCR-M3-7** State attributes: `data-hd-armed`, `data-hd-confirmed` (on the wrapper), `data-hd-dragging` and `data-hd-input` (on
    the control), and the `.hd-arm` marker in the body.
  - **SCR-M3-8** Instrument wide sensor numeral: `calc(22cqmin * 0.72)` (see the lead rulings).

- **`--hd-mask-check`.** A token in the `.hd-root` token block of `ui/css/homedash-tokens.css` (lead answer Q3). It is an inline SVG
  check mark as a data URL, which the stylesheet lint allows. `homedash-widgets.css` uses it as `-webkit-mask` and `mask` on the check
  glyph, painted with `--hd-ok-ink`. It is in the token freeze, `ui/testdata/homedash-lab/token-freeze.json`.

- **Sheet ids in use (M3).** Placeholders are `{family}` (glass, tactile, minimal, instrument, placeholder), `{hero}` (a W1 cell, W1 to
  W10, or a hero H1 to H8) and `~variant` suffixes:
  - W1 cells: `w1-{hero}-{family}` and `w1-{hero}-{family}~dpr2` (core states); `w1-side-by-side-{hero}` (four families, dark and light).
  - Themes, risk and custom: `w1-themes-{family}` (Glass, Minimal), `w1-risk-{family}` (Tactile, Instrument), `w1-{family}~custom`,
    `w1-glass~gradient`.
  - List open: `w1-list-open-{family}`.
  - Light sheet: `sheet-light-{family}` (7×6 and 4×7) and `sheet-light-{family}~dpr2`.
  - P1 states: `p1-{hero}-{family}`, `p1-{hero}-{family}~reduced`, `motion-p1-{hero}-{family}` (frames at 0, 50, 100, 200, 400 and 800 ms).
  - Regression: `hero-H4-{family}` and `hero-H4-{family}~dpr2`, the climate dial, which must stay byte-identical in Minimal.

- **`[role=option]` hit probe.** The 44×44 hit probe in `ui/testdata/homedash-lab-probes.js` selects interactive nodes in the lab stage
  (buttons, `[role="slider"]`, `[role="switch"]`, `[role="radio"]`, `[role="spinbutton"]`, `[role="option"]` and `[tabindex="0"]`). It
  also selects `.hd-pop-layer [role="option"]`, so listbox rows are measured even though the popover sits outside the stage.

- **Hardened colour probe.** The colour probe in `homedash-lab-probes.js` never passes a colour it cannot read.
  - `rgb()` and `rgba()` are parsed exactly. Other syntaxes (`oklch()`, `oklab()`, `lab()`, `lch()`, `color()`, `color-mix()`) are
    normalised to sRGB through a 1×1 canvas.
  - A string the canvas cannot parse is reported as an unreadable colour, never read as transparent. `rgba()` throws on it.
  - A self-check runs in the browser. Known syntaxes must resolve to known sRGB values, a nonsense string must throw, and an
    `oklch()` alpha must read correctly. The self-check returns its problems as a list; an empty list means the parser is sound.

## M3 documented deviations

These are recorded deviations from the family rules. None of them changes a token value.

- **M1 family geometry overrides (approved at the M1 gates; lead finding I3, batch D+E review).** Some family files set geometry
  that structure normally owns. They keep the M1 hero recipes that the M1 gates approved. The rule "structure owns geometry" still
  holds for every W1 control. The examples below are not the full list; the family files are. Examples:
  - Glass: the pill toggle's glyphs (`.hd-toggle-glyph`, inset 15 px for on and 32 px for off), the rocker's pending disc (26 px) and
    the 15 % marker of the dial (`.hd-dial::after`).
  - Instrument: the rocker bezel keeps 2.2 % side padding (`.hd-rocker-bezel`, rubric H2a).

- **Dark light-tile wash, per family.** The base wash is `--hd-light-on` at 18 to 40 % opacity, grown from the left by `--hd-value`.
  Each family's dark-scheme rule differs as follows.

  | Family | Scheme | Wash rule | Gate |
  | --- | --- | --- | --- |
  | Glass | dark only | opacity `calc(0.55 + 0.3 * value)`, so the wash reads as light and not as brown card (accepted deviation, G-2) | W-G round 1 |
  | Tactile | both | a backlit lens gradient of `--hd-light-on`, opacity `calc(0.7 + 0.3 * value)`; no dark-only rule | W-T round 1 (passed; the base 18 to 40 % does not apply) |
  | Minimal | dark only | a flat opaque mix `color-mix(in oklch, var(--hd-light-on) 58%, var(--hd-shadow-ink))` at opacity 1, with no plate behind the numeral (M-1, after two rounds) | W-M round 1 |
  | Instrument | both | a HUD gradient of `--hd-i-val` (20 % to 6 %) at opacity 1; 0.6 when on but empty, 0.5 when pending; no dark-only rule | W-I round 1 (no findings) |

## Host duties (M2 desktop host and kiosk page)

Implemented in M2 by: duty 1, the desktop shell (no HomeDash file loads fonts; `homedash-tokens.css` declares the two HomeDash faces);
duty 2, `hd-view` (`applyLook` after the first render and after every look change); duty 3, `hd-view` (never writes or clears the five
names; the solved accent goes through `--hd-n-accent`); duty 4, `hd-render` (`E.tone.of` on each entity, zones from `E.tone.zones`);
duty 5, `hd-render` (`resize({w, h, px})` when a widget's pixel size changes); duty 6, `hd-view` (scheme, palette and motion on
`.hd-root`, supplied by `homedash.js` from the desktop body); duty 7, `hd-view` (`data-hd-family` on `.hd-root`; the widget host
writes it on each `.hd-widget`). The desktop host draws the banners that `hd-view` computes.

1. **Fonts.** Load `/fonts/fonts.css` before rendering. It supplies Inter, Geist, Geist Mono and Oxanium. HomeDash itself declares
   only `HomeDash Barlow Condensed` (weight 600) and `HomeDash Share Tech Mono` (weight 400), in `homedash-tokens.css`.
2. **Look.** Call `E.widgets.applyLook(root)` on the `.hd-root` after the first render, and after every palette, theme or
   scheme change, once the palette tokens are written.
3. **Owned inline values.** Never write or clear `--hd-accent`, `--hd-light-on`, `--hd-focus`, `--hd-ink-on-accent` or `--hd-hud`
   inline. `applyLook` owns them.
4. **Alert tone.** Compute the tone from the widget options (gauge `zones`) and device classes. Pass it as the view `tone`
   (`ok`, `warn` or `crit`). The modules use `e.tone`, else the zone tone. The lab fixtures set `tone: 'crit'` on alert states.
5. **Resize.** Call `resize({w, h, px: {width, height}})` with the widget's pixel size. The sparkline and the Tactile trace need `px`;
   a resize without `px` is ignored by the sensor module.
6. **Root attributes.** Set `data-hd-scheme`, `data-hd-palette` and `data-hd-motion` on `.hd-root` from the user's settings. The motion
   gate applies only when `data-hd-motion` is neither `reduced` nor `none`. Do not write `data-hd-corners` (applyLook owns it).
7. **Family attribute.** `data-hd-family` may sit on `.hd-root` (the lab does it); the widget host also writes it on each `.hd-widget` (contract C3). After B-1, all colour and font tokens of a family are scoped to `.hd-widget[data-hd-family]`. What the bare `[data-hd-family]` blocks still declare on the root are family-scoped tokens (`--hd-g-*`, `--hd-t-*`, `--hd-m-*`, `--hd-i-*`) and non-colour scale tokens (strokes, numeral and label metrics, durations, `--hd-led`) that only widgets read. They are harmless for page chrome.

## Control lab

Use the lab for gates and for the sign-off pack. The lab is test-only and never shipped.

- **Page:** `/testdata/homedash-lab.html` with query parameters `family`, `palette`, `scheme`, `theme`, `accent`, `hue`, `backdrop`,
  `ease`, `more`, `types`, `variants`, `states`, `sizes`, `dpr`, `motion`, `lang`, `columns`, `graphics`, `scripts`. The header of
  `ui/testdata/homedash-lab.js` lists the values and defaults.
- **Sheets:** `ui/testdata/homedash-lab-sheets.json`. A sheet id is a literal id or a pattern with `{family}`, `{hero}` or `{type}`,
  plus an optional `~variant` suffix. Output: `reports/homedash-lab/<id>.png` (or `<id>-NN.png` when split at 2400 device px per side)
  and `probes.json` (`generated`, `report_only`, `sheets`, `failures`).
- **Sheet ids in use:**
  - hero: `hero-H1-{family}` ... `hero-H8-{family}` (H1 and H2 switches, H3 light dial, H4 climate dial, H5 needle, H6 arc,
    H7 numeral, H8 sparkline), with `{family}` in glass, tactile, minimal, instrument, placeholder;
  - dpr 2: `hero-H1-{family}~dpr2` ... `hero-H8-{family}~dpr2`; `hero-tactile~dpr15`;
  - gradient backdrop: `hero-glass~gradient`, `hero-glass~gradient-dpr2`;
  - custom palettes: `hero-{family}~custom`;
  - themes and risk: `hero-themes-glass`, `hero-themes-minimal`, `hero-risk-tactile`, `hero-risk-instrument`,
    `hero-minimal-theme~instrument`;
  - motion strips: `motion-H1-{family}` ... `motion-H8-{family}`, `motion-H5-tactile~fallback`;
  - side by side: `side-by-side-H1` ... `side-by-side-H8` (all four families in one sheet), and `side-by-side-H1~signoff` ...
    `side-by-side-H8~signoff`;
  - primitives and types (M0): `prim-{family}`, `prim-{family}~reduced`, `prim-{family}~gradient`, `prim-{family}~custom`,
    `type-{type}-{family}`, `themes-{family}`, `motion-type-{type}-{family}`.
  - Default sheets (`defaultSheets`): `prim-placeholder`, `prim-placeholder~reduced`, `type-entity-placeholder`, `themes-placeholder`.
- **Environment:** `HOMEDASH_LAB_SHEETS` (comma list of sheet ids; empty runs the default sheets). `HOMEDASH_LAB_REPORT_ONLY=1` writes the
  report and logs probe failures without failing the test. Never use it to pass a gate. `HOMEDASH_LAB_CVD=1` enables the colour-vision
  and greyscale copies.
- **Test:** `TestDesktopHomeDashControlLabBrowser` in `ui/desktop_homedash_control_lab_browser_test.go`. Chrome must start, so run the
  built binary outside the repository (for example `D:\tmp\go-build`) with the shell sandbox off. Build from the repository root, run from
  the `ui` directory, and quote the flags (PowerShell splits an unquoted `-test.run` at the dot):

```powershell
go test -c -o D:\tmp\go-build\hd-m1-ui.test.exe ./ui
cd ui
Remove-Item Env:HOMEDASH_LAB_REPORT_ONLY -ErrorAction SilentlyContinue
$env:AURAGO_RUN_BROWSER_SMOKE = '1'
$env:HOMEDASH_LAB_SHEETS = 'hero-H1-glass,hero-H1-glass~dpr2,motion-H1-glass'
& D:\tmp\go-build\hd-m1-ui.test.exe '-test.run=^TestDesktopHomeDashControlLabBrowser$' '-test.count=1' '-test.timeout=30m' '-test.v'
cd ..
$env:HOMEDASH_LAB_CVD = '1'
go test ./ui -run '^TestHomeDashLabColourVisionSheets$' -count=1
```

  The colour-vision test is skipped unless `HOMEDASH_LAB_CVD=1` is set. It reads the previous run's `reports/homedash-lab/hero-*.png`
  and writes `cvd-deutan-`, `cvd-protan-` and `grey-` copies next to them. The Bash form (Git Bash; not run by the author):

```bash
go test -c -o 'D:/tmp/go-build/hd-m1-ui.test.exe' ./ui
(cd ui && AURAGO_RUN_BROWSER_SMOKE=1 HOMEDASH_LAB_SHEETS='hero-H1-glass,hero-H1-glass~dpr2' 'D:/tmp/go-build/hd-m1-ui.test.exe' '-test.run=^TestDesktopHomeDashControlLabBrowser$' -test.count=1 -test.timeout=30m -test.v)
HOMEDASH_LAB_CVD=1 go test ./ui -run '^TestHomeDashLabColourVisionSheets$' -count=1
```
- **Gate sets (from the review protocol):** `$f` is the family. S: `hero-H1..H8-placeholder`. G: `hero-themes-glass`,
  `hero-glass~gradient`, `hero-glass~gradient-dpr2`. T: `hero-risk-tactile`, `hero-tactile~dpr15`, `motion-H5-tactile~fallback`.
  M: `hero-themes-minimal`. I: `hero-risk-instrument`, `hero-minimal-theme~instrument`. X: `side-by-side-H1..H8`,
  `hero-glass~custom`, `hero-tactile~custom`, `hero-minimal~custom`, `hero-instrument~custom`, then the G, T, M and I sets again.
  Expected run times per gate are in the plan (25 minutes per family gate, 70 minutes for X).
- **What the probes measure now:**
  - Text: 4.5:1 for HTML text, including `aria-hidden` HTML text (H7, H8 numerals). SVG text inside an `aria-hidden` element
    (dial and gauge numerals, tick labels) is held to 3:1.
  - Graphics (arcs, needles, sparkline line, bar fills, lamps, lit seven-segment segments, toggle thumbs): 3:1 against the worst-case
    background. Gradients are composited stop by stop, so the worst colour counts. A `url()` background is replaced by the root's resolved
    `--hd-canvas` as the underlay (wallpaper themes).
  - SVG graphics start their background walk at the svg itself.
  - Visually hidden text (`.hd-sr-only`, clipped or 1 px boxes) is skipped.
  - Unavailable widgets are measured; only disabled control subtrees (`[aria-disabled="true"]`, `[disabled]`) are exempt.
  - Toggle thumbs that show a decor glyph are not held to 3:1. Their position, glyph and track fill carry the state.
  - Theme colours (`palette=theme`) fail the same way as other palettes. Warnings are reported only for token pairs in theme palettes.
  - Hit targets 44x44; layout shift in every state; animations under reduced motion; backdrop-filter count against the number of frames;
    the two HomeDash faces loaded (Barlow Condensed 600, Share Tech Mono 400); tokens resolved; `m1-tabular` (tabular figures) and
    `m1-svgcount` (at most 150 svg nodes) as `hdLab.report` extras.
- **What the probes cannot see:**
  - Text painted by pseudo-elements, for example the "No data" box (`.hd-body::after` with `attr(data-hd-nodata)`).
  - Box-shadow rings (focus ring, alert ring, card shadows) and CSS-mask badges (stale, readonly, alert, error icons).
  - Images other than the canvas underlay, and anything behind a `url()` background.
  - Judgement: greyscale and colour-vision copies, equal visual weight, blind crops, and motion strips are checked by eye on the sheets.

## Review protocol (short form)

- **Order per gate:** `probes.json` first, including `extra`. Any probe failure is a finding. Then the hero sheets at 100 % (never
  scaled down), the same sheets at 400 %, the `~dpr2` sheets, the theme and risk sweeps, the motion strips, and the greyscale and colour
  vision copies.
- **Fresh sheets only:** a judgement uses sheets newer than the last commit of the task under review. Old sheets do not count.
- **Findings:** the lead writes a local, git-ignored `reports/homedash-lab/findings/<gate>-r<round>.md` (not in git) with a numbered table (ID, sheet file, hero, family,
  state, size, palette or theme, observation, required change, rubric ID). There is no minor tier. Every finding is closed on fresh
  sheets; a new round adds new findings.
- **Pass:** a gate passes only when every finding of every round is closed on fresh sheets. No round limit applies to heroes.
  A finding that survives three rounds needs an approach change; the bar does not drop.
- **Scheduling:** a family in revision blocks V7 and M3, as the plan words it. It does not block families whose files are disjoint.
- **Rubric IDs:** U1 to U13 (universal), G1 to G7 (Glass), T1 to T7 (Tactile), M1 to M5 (Minimal), I1 to I6 (Instrument),
  H1a to H8a (hero), S1 to S8 (structure), X1 to X6 (cross-family). Each ID is checked on sheets, not from code.

Anti-drift rules for implementers:

1. Values come only from the token tables and the recipe sheet. A missing value or a new token is proposed in the hand-back with a
   sheet that shows why. Use the frozen value until the lead approves.
2. Names and markup come from the M0 primitive contract plus the Task 1 additions. Never rename or restructure an M0 element.
3. No literal colour outside a token block. Gradients, shadow stacks and masks are tokens too. `stroke-width`, `border-radius` and
   `box-shadow` in family files go through `var(--hd-...)`, or are `0` or `none`.
4. A token that can be `none` is never placed inside a comma list.
5. Never `calc()` on `--hd-theme-radius`.
6. Every visual claim in a hand-back cites a sheet file path and the state, size and palette cell. A claim without a sheet does not count.
7. Edits stay inside the task's file list. A structure change after Gate S needs a structure change request.
8. After the token freeze, a token change needs the lead, then `node scripts/homedash-freeze-tokens.mjs`, then `npm run test:homedash`.

## Lead rulings that bind later controls

One line each. This section is the versioned record of the rulings; the local gate records they came from are not in git. A later control follows these unless the lead rules otherwise.

- **Ellipsis:** on one-cell-wide tiles (1x1 and 1x2) a long state text may end in an ellipsis; a title next to a badge keeps at least
  6 characters. At 2x1 and larger, the fixture state texts fit.
- **Tactile U9:** instrument emission colours that are materials (VFD amber, needle red) are not state hues. Each widget has at most one
  state hue, the LED or pilot.
- **Needles without tail:** the Minimal and Instrument needle shapes are clipped at the hub (`clip-path: inset(0 0 21.4286% 0) fill-box`,
  leaving 88 of 112 units). The hub dot stays.
- **Minimal head icon:** the head icon is hidden in Minimal; the title names the device.
- **Instrument sensor 2x1:** the digits match the Glass 2x1 numeral cap height within 1 px; the state text keeps 11 px and may end in an
  ellipsis; at least 8 px separates the readout from the state text.
- **Read-only is not disabled:** a read-only entity is shown at full strength with the lock badge. It is disabled only when it is not
  read-only and the capability is missing.
- **Empty numerals:** when a value is missing, only the en dash shows; the unit, degree and trend are hidden, and the box width is kept.
- **Unavailable light:** the readout is "–", never "Off"; a light that is on without a brightness reads "On", never "On · 0 %".
- **Stale ages:** whole minutes; the stale branch is taken only when `updated_at` parses.
- **Range text:** "min – max today" with the unit on the maximum only, for example "19.7 – 22.9 °C today".
- **Climate ranges:** the knob is rebuilt on a live range change only; while the entity is dead it keeps the last range.
- **Tactile mode selector:** four bakelite push keys; the climate module passes `rotary: false`.
- **Glass under-glow:** dropped; the arc has no under-glow.
- **Probes:** the 3:1 threshold never applies to HTML text, and theme colours are fixed through tokens or `applyLook`. A probe threshold is never lowered.
- **M3 (Minimal numerals largest):** judged on the side-by-side sheets; equal size is accepted where the body height limits the numeral.
- **Fonts:** no new `@font-face` in the family files; fonts come from `/fonts/fonts.css` (see host duties).
- **Duration format:** `E.format.duration` shows the largest non-zero unit and only the next smaller unit, which is omitted when it is zero:
  7215 s reads "2h", 3900 s reads "1h 5m". Negative or non-finite input shows the en dash.
- **Tile glyph inset:** the entity tile's state glyph keeps the card inset, so its bottom inset equals its left inset.
- **System names:** the client localizes the six built-in `system` entity names through `homedash.system.<native>` when a widget has no
  title. The server contract keeps English names, so other clients show English.
- **Instrument wide sensors:** at an aspect of 1.6 or more the Instrument sensor numeral is `calc(22cqmin * 0.72)` through
  `--hd-i-seg-h` (SCR-M3-8, D8, approved in M3). It replaces M1's fixed 15 px readout, which read small at 5x2. The token freeze
  changed only this line. At 2x1 the numeral is 15.2 px; at 4x2 and 5x2 it is 32.3 px (Gate W-I round 1, no findings).
- **Read-only and `aria-disabled` (lead, after the Batch C review, 2026-11-25):** read-only never sets `aria-disabled` on a control,
  buttons included. Read-only controls stay enabled and announce the long reason. `aria-disabled` is for an unavailable entity, a
  missing capability, or a stepper at its limit.
- **Segmented capacity (lead, Q10 extended, Gate W-S round 1):** a select shows 3 options below width 3 and 5 from width 3; more
  options render the list control. Four or five segments at 2×1 gave about 42 px per segment, under the 44 px hit size.
- **Colour wheel thumb (lead, Gate W-S round 1):** the thumb colour is the JS-set `--hd-wheel-colour`, because `--hd-wheel-rgb` as
  bare channels cannot be painted (the lint forbids a literal `rgb(`).
- **Chip colour specificity (Gate W-S round 1, WS-11):** structure sets no colour on the chips at a specificity of 0,4,0 (`:where()`),
  so family paint wins at normal specificity.
- **Icon-only chips at 1×1 (Gate W-G round 1, G-1; applied everywhere at Gate W-X, X2):** a switch push and an action button at 1×1
  show the icon only, centred. The word stays in the accessible name. The pending ring sits on the icon.
- **Title width (Gate W-S round 1, WS-1):** the title keeps at least 6 characters plus an ellipsis in the one-row head. The state
  text and its icon truncate first. This applies to every W1 type.
- **Fader layout (Gate W-S round 1, WS-2):** at 1×2 the fader's slider (min 64 px), 8 px and the 44 px chip sit inside the body, and
  the state line has its own row. Nothing overlaps at 1×2 or 1×3.
- **More button at width 1 (Gate W-S round 1, WS-3):** the more button is hidden at width 1 at every height. Hold, Shift+Enter and
  ContextMenu still open the sheet.
- **Light tile badge at 1×1 (Gate W-S round 1, WS-4):** the icon badge is hidden at 1×1, so the numeral stands alone, bottom-left.
- **Unit size (Gate W-S round 1, WS-5):** units are at least 11 px (or 0.5 em when that is larger), baseline-aligned, and the number
  uses tabular figures.
- **Armed bar (Gate W-S round 1, WS-6):** the armed bar never overlaps a control. Its height is reserved at the card's bottom inset.
- **Light sheet height (Gate W-S round 1, WS-7):** the sheet sizes to its content, at most `min(600 px, 85 %)`, and the foot follows the
  content.
- **Light sheet head (Gate W-S round 1, WS-8):** the state row sits directly under the head. The area is a muted subtitle, aligned
  with the name. The wheel column's top aligns with the first row, and its label sits above the wheel.
- **Slider thumbs (Gate W-S round 1, WS-9):** the CT and brightness thumbs are centred vertically on their track at every value.
- **List popover height (Gate W-S round 1, WS-10):** no scrollbar until the rows exceed the 352 px maximum. The border and padding count
  in that size.
- **Glass dark wash (Gate W-G round 1, G-2):** the dark light tile reads as warm light, never as flat brown. Numeral and unit contrast is
  at least 4.5:1 and 3:1. The dark-scheme opacity deviation is listed under M3 documented deviations.
- **Tactile on and off in the dark (Gate W-T round 1, T-1):** the on key differs from the off key by luminance or form, not only by the
  LED. The pressed key shows a lit amber legend and a glow; the off key keeps an unlit legend. The same applies to the switch tile, the
  light power chip, the segmented key bank and the picker trigger.
- **Tactile bezel (Gate W-T round 1, T-2):** the light sheet's steel rim and the wheel rim use a brushed tonal gradient from the steel
  tokens. No edge is brighter than the strongest widget edge highlight.
- **Tactile M1 heroes (Gate W-T round 1, T-3):** every listed sheet was inspected. H1 to H3 are unchanged by construction, and H4 was
  re-judged at Gate W-X.
- **Minimal dark wash (Gate W-M round 1, M-1, closed after two rounds):** no plate or box behind the numeral. The wash is a flat opaque
  mix, and the numeral reads directly on the tile. The mix is listed under M3 documented deviations.
- **Instrument light tile (Gate W-I round 1, no findings):** the light tile uses the HUD tint, with a level bar and corner brackets. On
  and off read by luminance in both schemes.
- **Cross-family judgement (Gate W-X, 2026-11-26, no findings):** X1 each family is recognisable from a blind crop of push, slider and
  tile. X2 the same structure, sizes and state positions in every family. X3 on and off are unmistakable in all four, in both schemes.
  X4 the real page renders the W1 controls with German state words. X5 the M1 heroes H1 to H4 are unchanged.
- **Pending, armed, error and focus (Gate P round 1, 2026-11-26, no findings):** P-a pending, P-b armed, P-c rollback and error, and P-d
  focus pass. Open: no lab cell focuses a climate `+` or `−` step. The C2 rule composes the same ring, but no sheet shows it.
- **Focus rings (lead, batch D+E review, fixes C1 and C2):** the family focus rules compose the M0 ring with the resting shadow. Tactile
  `.hd-seg-opt` and `.hd-step-value` keep their ring. The climate steps (`.hd-push`) keep theirs in Glass, Minimal and Instrument.
- **Drag transitions (lead, batch D+E review, fix I1):** the dial arc and pointer transitions are off in all four families while
  `[data-hd-dragging]` is set.
- **Live regions (lead, batch D+E review, fixes I4 and I5):** the host keeps its notice node and banner container in the DOM, so
  announcements fire. Create buttons use `aria-disabled` while busy and keep focus. The click guard still applies.
- **Freeze keys (lead, batch D+E review, fix I2):** the token freeze keys each rule by its selector and at-rule chain. Coverage changed;
  no token value did.
