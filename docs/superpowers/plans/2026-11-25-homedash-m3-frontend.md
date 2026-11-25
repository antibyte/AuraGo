# HomeDash M3 Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every W1 control of HomeDash work end to end with pointer, touch and keyboard: P1 (interaction layer: command client with `request_id` per action, optimistic overlay, pending after 250 ms, contract confirmation with tolerances and late adoption, rollback with a 6 s cue, 1250 ms continuous throttle, 400 ms keyboard trailing send, confirm-required arming, read-only reasons, ARIA, focus, reduced motion) and W1 (switch push/tile, light slider/fader/tile plus the M1 dial on `brightness_pct`, action button, select segmented/list, number stepper/slider, light more-info sheet) in all four families at the M1 bar. Plus the M2 carry-overs (first values within 1.5 s, the approved Instrument numeral token change, header icons), the `home_assistant` template flow, per-source notices, and at the end one browser test against the real HomeDash handlers with `hasstest` as Home Assistant.

**Architecture:** Everything interactive lives in the engine (`ui/js/homedash/`), so the kiosk (M6) inherits it. New engine parts: `hd-input.js` (`E.input`: pure value maths plus `press`/`gesture`/`keys`/`repeat` binders on pointer events), `hd-control.js` (`E.control`: capabilities, ranges, read-only reasons, cue texts, the state-line rule shared by all control widgets), `hd-popover.js` (`E.popover`: the popover sheet, a listbox anchored at ≥ 600 px or a bottom sheet), `hd-moreinfo.js` (`E.moreinfo`: the light sheet), three widget modules (`hd-w-action.js`, `hd-w-number.js`, `hd-w-select.js`), and in the app layer `hd-command.js` (`E.command`: one command client per JS realm with the overlay; it listens to the stream for confirmation and the render layer merges its overlay into entity views). Primitives get an optional `input` contract; without it their markup is unchanged. `hd-render` routes `can`/`command`/`moreInfo` to the command client; `hd-view` owns the more-info controller; `hd-app` boots in parallel and offers the `home_assistant` template. The fixture API in `ui` gains the commands route and the contract's demo entities until the backend lands.

**Tech stack:** Vanilla ES2020 classic IIFE scripts, CSS custom properties and container queries, Node 22 tests (`npm run test:homedash`, strict fake DOM), Go 1.27 rod browser tests in `./ui` (and one in `./internal/server`, see Q1).

The code below was written without running it. Implementers may fix errors (typos, a selector, a missing import) but must not change the M3 API contract, the engine contracts in `ui/js/homedash/AGENTS.md`, or a rule stated in this plan without reporting it to the lead.

## Lead answers (2026-11-25)

All ten defaults in "Questions for the lead" are accepted. Notes:

- **Q1** Default accepted: the test-only file `internal/server/desktop_homedash_live_browser_test.go` (package `server`, no production change) is the only `internal/` path the frontend track touches. It is written in Task 28, after the backend branch has landed on main, and reuses backend Task 13's `newHomeDashHAEnv(t, ha *hasstest.Server) *homeDashTestEnv` and `handleHomeDash(env.s)` (backend plan lines 5096 and 5207) plus `hasstest.New`/`hasstest.DemoOptions()`. If a backend name differs when Task 28 starts, use the landed name and report it; do not add production code.
- **Q2** Accepted: the `homedash` entry of `module-loader.js`, `TestHomeDashWiring`, `homedash-home.js` and `desktop-app-homedash.css` are frontend files.
- **Q3** Accepted: `--hd-mask-check` joins the `.hd-root` token block. If Task 16 and Task 21 run in the same batch, regenerate the freeze once for both changes; otherwise twice, each with its own commit naming the approved change.
- **Q4** SCR-M3-1…7 approved. Before/after sheets of the M1 heroes for SCR-M3-1/2 are part of gate W-X; any hero regression found there is a gate failure, not a minor note.
- **Q5–Q10** Accepted as written.

---

## Lead decisions applied

| Decision | Applied in |
|---|---|
| D1 scope: P1 + W1; more-info for lights only; no presets; no history in the sheet; battery and sensor sheet stay in M5 | Tasks 3, 4, 7–15; Task 9 (sheet contents); hold on non-light entities is a no-op (Task 10 `moreInfo`) |
| D2 contract first; consume verbatim | Section "Contract consumed"; fixture (Task 23) implements §2/§4/§5 exactly |
| D3 expected per command; momentary without expected | Task 4 (`predict`, `matches`, momentary confirm on 202) |
| D4 `home_assistant` template, localized `name`/`unassigned_title`, `omitted` | Tasks 2, 10, 22, 23, 24 |
| D5 timings, confirmation, 1250 ms throttle, 429 rules, 400 ms keyboard, arming, read-only reasons, pointer/touch, keyboard/ARIA, motion, engine-owned | Tasks 3, 4, 7, 8, 10–16, 18–21 |
| D6 `brightness_pct` truth; attribute names; ranges/options from capabilities | Task 7 (`E.control.range/ctRange/options/lightModel`), Task 12, Task 17 (lab fixtures) |
| D7 partial catalogs, `source_unavailable`+`source`, shutdown frame (client already reconnects) | Task 10 (banners per used source, `catalog_failed`), Task 7 (cue text with source), Task 22 |
| D8 first values (parallel requests, visibility unknown = visible, titles before snapshot, 1.5 s); Instrument token change; header icons | Tasks 5, 10, 22, 24; Task 21 (token); Tasks 1, 7, 23 |
| D9 icon names | Task 1 (all new names of contract §6 plus UI glyphs), Task 7 (`iconFor`) |
| D10 service policy unchanged | Read-only reasons per `blocked_by` (Task 7) |
| D11 gates W and P, strict probes, German preview pack | Tasks 17–21, Gates W-S/W-G/W-T/W-M/W-I/W-X/P, Task 26 |
| D12 security review | Backend only; the frontend never renders HA text except entity names/options through `textContent` |
| D13 lean tests; live-handler test last | Every task; Task 24; Task 28 |
| D14 live HA is the user's acceptance | Hand-over (Task 27) |
| D15 ownership; fixture until backend lands; hand-over | File structure, R8, Q1/Q2; Task 27 |
| Backend Lead answer Q1 (widget `icon`) | Task 7 (`iconFor`), Task 23 (starter testdata icons) |
| Q2 (cached bootstrap; client re-reads) | Task 10 (`refreshTemplates` on home entry, visibility return, `ha` → live) |
| Q3 (tolerances + late adoption) | Task 4 (rule 4, full code) |
| Q4 (qualification by HA policy) | No client logic; with `homedash.readonly` the template is offered and controls show read-only |
| Q5 (`omitted` always present; `422 template_empty`) | Tasks 10, 22, 23 |
| Q6 (no camera scaling) | n/a (M5) |
| Q7 (non-W1 capabilities mapped, no expected) | `predict` covers W1 verbs only; other verbs are never sent by W1 widgets |
| Q8 (`hasstest` importable) | Task 28 |
| Q9 (polling window) | `polling` is not a notice state; banners only for `degraded`/`unavailable` |

## Contract consumed (M3 delta + M2 where unchanged)

| Contract item | Used by |
|---|---|
| §2 entity shapes, capabilities with `allowed`/`blocked_by`/`sensitive`/ranges/options, `readonly_reason`, value types, `available`, `stale` | `hd-store.view` (+`readonlyReason`), `E.control`, widgets |
| §3 source states, partial catalogs (`catalog_failed`) | `E.view.banners` (per used source), host banner texts |
| §4 request, 202 (`accepted`, `superseded`, `expected`), confirmation rule, late adoption, error bodies (+`source`, `blocked_by`, `retry_after_ms`), limits | `hd-api.command`, `hd-command` |
| §5 template request/answer, `omitted`, `template_empty`, bootstrap `templates` + re-read | `hd-api.createFromTemplate`, `hd-app.create/refreshTemplates`, host |
| §6 icon names, widget `icon` | `hd-icons`, `E.widgets.iconFor` |
| §8 duplicate state frames, state before websocket | `hd-command` frame handling (idempotent) |
| §10 `revoked shutdown` | unchanged M2 reconnect |
| §12 "Do not assume" | `predict`/`matches` merge only expected attributes; scene/button values never parsed; ranges read from capabilities; CT grid anchored at `min` (`E.input.snap`) |

## File structure

Created (all frontend-owned):
- `ui/js/homedash/hd-input.js` — `E.input`: snap/keys/fraction/dial maths; `press`, `gesture`, `keys`, `repeat` binders.
- `ui/js/homedash/hd-control.js` — `E.control`: capability/range/options helpers, `lightModel`, read-only reasons, cue texts, state-line rule, tap/hold actions, on/off command choice, arm marker.
- `ui/js/homedash/hd-popover.js` — `E.popover.listbox`: anchored listbox popover / bottom sheet.
- `ui/js/homedash/hd-moreinfo.js` — `E.moreinfo.build` (static light sheet, used by the lab) and `create` (modal).
- `ui/js/homedash/hd-command.js` — `E.command`: command client, overlay, confirmation, throttle, arming, cues.
- `ui/js/homedash/widgets/hd-w-action.js`, `hd-w-number.js`, `hd-w-select.js` — new W1 types.
- `ui/testdata/homedash-lab/w1-fixtures.js` — lab fixtures for W1, the light sheet, P1 scenarios.
- `ui/testdata/homedash-m3/ha-entities.json`, `ha-dashboard.json` — contract demo data shared by Node tests and the Go fixture.
- `ui/desktop_homedash_fixture_m3_test.go` — fixture M3 routes and controls.
- `ui/desktop_homedash_controls_browser_test.go` — `TestDesktopHomeDashControlsBrowser`.
- `internal/server/desktop_homedash_live_browser_test.go` — only after lead approval (Q1), Task 28.
- Node tests: `scripts/homedash-tests/icons.test.mjs`, `input.test.mjs`, `command.test.mjs`, `control.test.mjs`, `overlay.test.mjs` (sheet/popover), `w1.test.mjs`.
- `docs/superpowers/specs/2026-11-23-homedash/m3-handover-frontend.md` (force-add).

Modified:
- Engine: `hd-icons.js`, `hd-api.js`, `hd-store.js`, `hd-stream.js`, `hd-widgets.js`, `hd-prim-displays.js` (`frame` more glyph), `hd-prim-controls.js`, `hd-sheet.js`, `hd-render.js`, `hd-view.js`, `hd-app.js`, `widgets/hd-w-switch.js`, `widgets/hd-w-light.js`, `AGENTS.md`.
- CSS: `homedash-tokens.css` (one token, Q3), `homedash-primitives.css`, `homedash-widgets.css`, `homedash-page.css`, four family files, `homedash.bundle.css` (generated), `desktop-app-homedash.css`.
- Host: `ui/js/desktop/apps/homedash.js`, `homedash-home.js`; `ui/js/desktop/core/module-loader.js` (HomeDash entry only, Q2).
- Lab: `ui/testdata/homedash-lab.html`, `homedash-lab.js`, `homedash-lab.css`, `homedash-lab-probes.js` (stricter hit selector), `homedash-lab-sheets.json`, `homedash-lab/hero-fixtures.js`, `homedash-lab/token-freeze.json` (regenerated twice, approved), `ui/desktop_homedash_lab_plan_test.go`.
- Tests: `harness.mjs` (orders), `contract.test.mjs`, `app.test.mjs`, `store.test.mjs`, `stream.test.mjs`, `render.test.mjs`, `view.test.mjs`, `home.test.mjs`, `host.test.mjs`, `i18n.test.mjs`, `controls.test.mjs`, `api.test.mjs`; `ui/desktop_homedash_wiring_test.go`, `ui/desktop_homedash_fixture_test.go`.
- Testdata: `ui/testdata/homedash-m2/starter-dashboard.json` (widget icons), `system-entities.json` (uptime hint `timer`).
- Locales: `ui/lang/homedash/*.json` (16).
- Docs: `ui/js/desktop/apps/AGENTS.md`, `documentation/homedash-visual-system.md`, root `AGENTS.md` (HomeDash row).

Not touched: anything under `internal/` except Q1's single test file; the widget catalog and its golden file; HA Switchboard files.

Load orders (pinned by `harness.mjs`, module-loader, `TestHomeDashWiring`, lab html, `contract.test.mjs`):
- `ENGINE_ORDER` = `hd-core, hd-format, hd-colour, hd-grid, hd-layout, hd-doc, hd-history, hd-icons, hd-geometry, hd-input, hd-prim-displays, hd-prim-controls, hd-widgets, hd-control, hd-popover, widgets/hd-w-misc, widgets/hd-w-climate, widgets/hd-w-light, widgets/hd-w-sensor, widgets/hd-w-switch, widgets/hd-w-action, widgets/hd-w-number, widgets/hd-w-select, hd-moreinfo` (`.js`).
- `APP_ORDER` = `ENGINE_ORDER` + `hd-tone, hd-api, hd-stream, hd-store, hd-command, hd-catalog, hd-render, hd-sheet, hd-view, hd-app`.
- Each task that creates an engine file updates the five pinning places in the same commit ("wiring step W").

---

## P1 interaction rules (normative; quoted verbatim in `hd-command.js`, `hd-input.js` and `AGENTS.md`)

1. **Request ids.** Every user action gets a new `request_id` (`E.core.randomId('r', 20)`); a 429 retry gets a new one (the server caches failures for 30 s).
2. **Overlay.** The predicted state (contract `expected` table) applies at once to every widget bound to the entity (and the sheet). `pending` appears 250 ms after a send that is still unconfirmed, never before; continuous input that is not yet sent has no pending.
3. **Confirmation** (contract §4): a `state`, `snapshot` or `entity` frame received after sending confirms when `available` is true, `value` equals `expected.value` (booleans/strings exactly, numbers within half the capability step, exactly without step) and every expected attribute matches (`brightness_pct` ±1, `color_temp_kelvin` ±2 %, `hs_color` hue ±4° circular and saturation ±4, strings exactly). The server state already in the store at the 202 counts. A 202 without `expected` (activate, press, toggle from an unknown state) confirms at once. Confirmation shows the check glyph in the state line for 600 ms.
4. **Errors.** `superseded:true` ends silently. Errors roll back at once with a 6 s cue in the state line (alert ink, error icon) and the same text in the root's `aria-live="polite"` region; `409 confirmation_required` arms instead; `403 read_only` and `403 desktop_readonly` roll back and mark the entity read-only with the localized `blocked_by` reason (no red cue, announced); `desktop_readonly` also switches the app to the read-only desktop.
5. **Timeout.** 8 s without confirmation: if a frame since sending had `available:true` and a matching `value`, the overlay drops silently and the reported attributes show (late adoption); otherwise rollback and the timeout cue. Later frames simply update the store.
6. **Continuous** (`brightness`, `color_temp`, `color`, `set_value`): the overlay follows every input frame; intermediate sends at most every 1250 ms (leading edge included); commit (release) always sends the latest value; a 429 on an intermediate is dropped; a 429 on the final is retried once after `retry_after_ms` (default 1000 ms) if it is still the latest value. Pointercancel after a drag started commits.
7. **Keyboard** changes (any command) move the overlay at once and send one trailing command 400 ms after the last key.
8. **Confirm-required** (capability `sensitive`, or widget `confirm:true` on discrete commands): the first activation arms the widget for 4 s (countdown bar + state line "Tap again to confirm · N s", announced); the second activation within 4 s sends `confirmed:true`; a `409` arms. Continuous controls ignore widget `confirm` (Q6).
9. **Read-only** (L2 stays): controls of read-only entities keep full strength with the lock badge; activation does nothing and announces the long reason; the state line reads `word · short reason`. Unavailable entities: `aria-disabled`, word "Unavailable", no affordance.
10. **Pointer/touch:** pointer events only; primary pointer only; pointer capture (try/catch, window listeners as fallback); 4 px separates tap from drag and cancels hold; hold = 500 ms; `touch-action`: horizontal sliders and the light tile `pan-y`, vertical faders `pan-x`, dials and the colour wheel `none`; no hover-only affordance.
11. **More-info** for light entities only: hold, the visible `.hd-more` button (≥ 2×1), Shift+Enter or the ContextMenu key, or tap when `actions.tap.do = more_info`. Hold on other entities does nothing. Right-click is not an affordance in M3 (Q9).
12. **Switch commands:** one activation sends `turn_on`/`turn_off` from the shown state when allowed, `toggle` only as fallback (Q5).

### Keyboard and ARIA per control

| Control | Role / ARIA | Keys |
|---|---|---|
| pill, rocker, switch push, switch tile, light power chip, on/off light tile | `button[role=switch][aria-checked]`, name "title: Power" where needed | Space/Enter toggle (native click); Shift+Enter/ContextMenu more-info (lights) |
| light slider, fader, number slider, sheet brightness | `role=slider` + `aria-valuemin/max/now`, `aria-valuetext` in the unit ("64 %, On"; "35 %"; "55 °C") | Arrow ±step, PgUp/PgDn ±10 % (≥ 1 step), Home/End, Shift+Enter more-info |
| light dial | `role=slider` | as slider; Enter/Space toggle power |
| dimmable light tile | `role=slider`, `aria-roledescription` "dimmer", `aria-keyshortcuts="Enter Space"` | Arrows/PgUp/PgDn/Home/End dim; Enter/Space toggle |
| CT strip | `role=slider`, valuetext "3000 K" | as slider; End = last grid value ≤ max |
| colour wheel | `role=slider`, valuetext "Hue 27°, saturation 56 %" | Left/Right hue ∓/±5° (wrap), PgUp/PgDn hue ±30°, Up/Down saturation ±5, Home/End saturation 0/100 |
| action button | `button` (aria-disabled for read-only) | Enter/Space |
| segmented | `radiogroup` > `button[role=radio]`, roving tabindex | Arrows move focus and selection (trailing 400 ms send); Space/Enter select now |
| list | `button.hd-pick[aria-haspopup=listbox][aria-expanded]`; popover `ul[role=listbox][aria-activedescendant]` > `li[role=option][aria-selected]` | Enter/Space/ArrowDown open; Up/Down, Home/End, typeahead; Enter/Space pick; Escape/Tab close; focus returns to the trigger |
| stepper | `group` > two `button`s + `span[role=spinbutton]` | spinbutton: Arrows/PgUp/PgDn/Home/End (trailing send); buttons: click steps, hold repeats 400 ms then 80 ms |
| light sheet | `role=dialog aria-modal=true aria-labelledby`, focus trap, Escape, backdrop click, focus return; siblings inert except the live region | Tab cycles; first focus = power toggle |

Focus ring: the M0 box-shadow ring on every interactive role including `option` and `listbox`; family extras as M1. Reduced motion: no keyframes or transitions run (probe), countdown and pending become static plus words.

---

## W1 build sheets (structure is family-neutral; families paint)

Cell 96 px, gap 12 px. "1-row" = height 1 with ≥ 2 columns: the frame becomes `"head state" var(--hd-head-h) / "body body" 1fr` (body 50 px at 2×1; SCR-M3-5). 1×1 keeps three rows (44 px body).

| Variant | Sizes (gate) | Body |
|---|---|---|
| switch/push | 1×1, 2×1 | `button.hd-push[data-hd-variant=chip][role=switch]` 44 h, icon 20 + On/Off word, 2×1 width `min(100%,152px)` at the start; 1×1 full width |
| switch/tile | 1×1, 2×2 | `button.hd-push[data-hd-variant=tile][role=switch]` fills the body; icon 28 (40 at ≥ 200 px tall); label visually hidden |
| light/slider | 2×1, 4×1 | `.hd-lrow`: power chip 44×44 (bulb) · slider (flex, 44 hit, track 6) · numeral 4.5ch (hidden below 300 px) |
| light/fader | 1×2, 1×3 | `.hd-lcol`: numeral (only ≥ 260 px tall) · vertical slider (min 64, cap 40×24) · power chip 44×44 |
| light/tile | 1×1, 2×2 | `.hd-ltile` fills the body: wash layer `scaleX(--hd-value)` from the left (18–40 % `--hd-light-on`), icon badge top-left, numeral bottom-left (22cqmin; 18 px at 1×1) |
| light/dial | M1 H3 | unchanged geometry; centre tap toggles |
| action/button | 1×1, 2×1 | `button.hd-push[data-hd-variant=chip]` 44 h: kind icon (scene/script/button) + verb (Activate/Run/Press); check icon for 600 ms after success |
| select/segmented | 2×1, 3×1 | `.hd-seg` full width (overrides H4's 260 px cap); > 5 options render the list control (Q10 default in self-review) |
| select/list | 1×1, 2×1 | `button.hd-pick` 44 h full width: value (ellipsis) + chevron 16 |
| number/stepper | 2×1, 3×1 | `.hd-stepper` centred: − 44 · value numeral 4ch + unit · + 44 |
| number/slider | 2×1, 4×1 | `.hd-nrow`: slider (flex) · numeral 5ch with unit |
| light sheet | 7×6 (744×636), 4×7 (420×744) | dialog `.hd-sheet.hd-widget[data-kind=moreinfo]` 640×600 max centred; < 600 px root: bottom sheet (100 % wide, ≤ 85 % high). Frame head (bulb/icon, name, area as sub, close 44), body grid rows power / brightness / CT / effect, wheel (220 px) in a right column when ≥ 560 px wide; state row = state word or reason; foot = "Source: Home Assistant · updated 2 min ago" |
| listbox popover | — | `.hd-sheet.hd-widget[data-kind=listbox]` in `.hd-pop-layer`; width `max(anchor, 200px)`; rows 44; check on the selected; max 352 px high; below the anchor (above when no room); bottom sheet below 600 px |

State visuals per family (shared language of visual-system §7):

| State | Glass | Tactile | Minimal | Instrument |
|---|---|---|---|---|
| pending (after 250 ms) | conic sweep ring (`--hd-pending-ring`) on thumb/icon disc | LED amber 2 Hz blink (static amber reduced) | 270° hairline ring rotating; "…" word | segment chase / blinking lead; static half-lit reduced |
| armed | accent ring around the control + `.hd-arm` bar (accent) | amber LED blink + bar hidden | text only | HUD ring + bar (`--hd-i-val`) |
| error cue | shake translateX 6 px, 3 cycles, 320 ms | red LED 3 blinks | shake 4 px | alert flash (HUD → alert, 6 cycles max) |
| confirmed | thumb/disc pulse 1→1.06→1 180 ms + check glyph | steady LED + check | check only | 90 ms flash + check |
| drag | thumb ×1.12, shadow deepens | cap pressed 1 px, shadow shift | dot ×1.25 | lead segment glow |
| on (tile) | `--hd-on` wash 22 %, disc filled `--hd-on` with on-ink glyph | LED lit, legend I | disc ink-filled, glyph inverted | HUD top bar + glow |

Paint tokens: only frozen tokens (`--hd-on`, `--hd-on-ink`, `--hd-light-on`, `--hd-track`, `--hd-track-inset`, `--hd-track-fill`, `--hd-thumb`, `--hd-thumb-shadow`, `--hd-pending-ring`, `--hd-g-*`, `--hd-t-*`, `--hd-m-*`, `--hd-i-*`, `--hd-ct-2200/4000/6500`, `--hd-card-*`, `--hd-shadow-*`, `--hd-alert-ink`, `--hd-ok-ink`, `--hd-accent`, `--hd-value-colour`). A missing value is proposed in the hand-back with a sheet, never invented.

### Structure change requests (approve before Task 7; Q4)

- **SCR-M3-1** Head icon for every widget: `widget.icon` → entity `icon_hint` → kind icon (contract §6). M1 hero heads gain icons (≥ 2×1; Minimal hides head icons by ruling; 1×1 hides).
- **SCR-M3-2** `frame({more})` puts the `more` glyph in `.hd-more`; production structure 44×44 top-right, hidden at 1×1; the host wires it to `moreInfo`.
- **SCR-M3-3** Overlays (more-info dialog, listbox panel) carry `hd-widget` + `data-hd-family` so B-1 family tokens and family paint apply; `.hd-sheet[data-kind]` is the "popover sheet" primitive of visual-system §3 (the only second blur).
- **SCR-M3-4** Primitive input contract: optional `input` on toggle, rocker, pushButton, slider, knob, stepper, segmented, ctStrip, colourWheel; `pushButton({toggle})` (role switch) and `set({icon})`; new primitive `picker`. Without `input`, markup equals today's.
- **SCR-M3-5** 1-row frame layout for W1 types at height 1 and ≥ 2 columns.
- **SCR-M3-6** Production structure for slider, CT strip, wheel, stepper, picker (never existed); the slider thumb moves with `transform` in track container units so value changes may transition.
- **SCR-M3-7** State attributes `data-hd-armed`, `data-hd-confirmed` (wrapper), `data-hd-dragging`, `data-hd-input` (control) and the `.hd-arm` marker in the body.
- **SCR-M3-8** (approved by D8) Instrument wide sensor numeral token.

---

## Project rules (every task; each dispatch quotes the ones it needs)

- **R1 Worktree.** Work only in the session worktree; use its branch; no `git worktree add`. Copy a real `node_modules` (Task 0).
- **R2 GitNexus.** The CLI fails in this repo; use MCP. Before dispatching a task that edits an existing symbol, the **track lead** runs `impact({target, direction:"upstream", repo:"AuraGo"})` for every symbol under "Impact", pastes callers and risk into the dispatch, warns on HIGH/CRITICAL. `UNKNOWN`/empty → the maker greps the callers and reports them. Makers **stage only**. The lead runs `detect_changes({scope:"all", repo:"AuraGo", worktree:"<path>"})` (re-run on `partial`/`truncated`) and commits by path. Without MCP answer: commit body line `GitNexus unavailable; callers checked with grep: <symbols>`.
- **R3 Commits.** One commit per task: `git commit -m "<msg>" -- <paths>`; never `git add -A`; never `git restore`, `git reset`, `git stash`, `git checkout -- <file>`. New `AGENTS.md` and anything under `docs/`: `git add -f`, confirm with `git ls-files`.
- **R4 Files.** LF, no BOM. JS < 1100 lines (aim < 700). No `innerHTML`/`outerHTML`/`insertAdjacentHTML`/`setAttribute('style'`/`.style =`/`cssText`; CSSOM via `style.setProperty`/`E.core.setVars`. Untrusted text via `textContent`. Literal i18n keys `t('homedash.…')` (no built keys). Non-ASCII in JS via `String.fromCodePoint`. Go: never a literal backslash-u escape (use `\x5c`). No external assets. Write scripts with the Write tool, not heredocs.
- **R5 Builds.** After any `ui/css/homedash-*.css` edit: `npm run build:ui`, commit `ui/css/homedash.bundle.css`; `npm run check:ui` must pass.
- **R6 Go env.** `GOTMPDIR` backslash form, short (`C:\tmp\go-build` or `D:\tmp\go-build`); on `no such tool "vet"` prefix `GOTOOLCHAIN=go1.27.4`; D: full → trim `D:\dev-cache\go-build` entries older than 3 days (1 day if that frees nothing). Repository-wide checks only in the worktree.
- **R7 Browser tests.** Build the binary into `D:\tmp\go-build`, run it from the package directory with the shell sandbox off, quote every flag: `go test -c -o D:\tmp\go-build\hd-m3-ui.test.exe ./ui` then `cd ui; $env:AURAGO_RUN_BROWSER_SMOKE='1'; & D:\tmp\go-build\hd-m3-ui.test.exe '-test.run=^TestX$' '-test.v' '-test.timeout=30m'`. CSS/JS/testdata edits need no rebuild; Go edits do. Never stop Chrome by image name; report leftover rod Chromes to the lead.
- **R8 Scope.** Only frontend paths (file structure). The module-loader `homedash` entry and Q1's file need the lead's OK (Q1, Q2). The catalog golden hash stays unchanged.
- **R9 Translations.** Every new key in all 16 locales with real translations; German "Du", real umlauts; the L5 test stays green; allowlist only brand names/units.
- **R10 Visual.** Anti-drift rules of `documentation/homedash-visual-system.md` ("Review protocol"); token changes only with the lead plus `node scripts/homedash-freeze-tokens.mjs`; family files paint only; never weaken a probe; claims cite a fresh sheet file and cell.

---

## Task 0: Workspace

- [ ] `git branch --show-current`; `git merge-base --is-ancestor 2676770e5 HEAD; echo $LASTEXITCODE` → `0`.
- [ ] `robocopy C:\Users\Andi\Documents\repo\AuraGo\node_modules <worktree>\node_modules /E /MT:16 /NFL /NDL /NJH /NP`.
- [ ] Baseline into the session scratchpad: `npm run test:homedash` (count), `npm run check:ui`, `go test ./ui -run '(?i)^(TestHomeDashWiring|TestHomeDashLab.*|TestTranslations_|TestFrontend_StaticI18nKeysExistInEnglishBundle|TestDesktopAppMaterialsAdoptedByApps)$' -count=1`, `go test ./ui -run '^TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget$' -count=1` (record known failures; no HomeDash file may appear), `go vet ./ui`, `go test ./internal/audit -run '^TestUIBrowserCIShardsCoverEveryTest$' -count=1`.
- [ ] One MCP `impact` call (`target:"realmFor"`) to confirm MCP answers.
- [ ] `git log --oneline -1 main -- internal/homedash/sources/hass/hasstest/server.go` (empty = backend not landed; matters for Task 28).

**Done when:** baseline recorded, MCP route known.

---

## Batch A — interaction core

### Task 1: Icons (contract §6 and UI glyphs)

**Files:** modify `ui/js/homedash/hd-icons.js`; create `scripts/homedash-tests/icons.test.mjs`. **Impact:** `PATHS`, `icon` (LOW, additive).

- [ ] Test:
```js
import assert from 'node:assert/strict'; import { loadEngine } from './harness.mjs';
const NEW = ['memory', 'download', 'upload', 'timer', 'outlet', 'scene', 'script', 'button', 'list', 'number', 'brightness', 'color-temp', 'color', 'effect', 'chevron-down', 'close', 'home'];
const CONTRACT = ['cpu', 'memory', 'disk', 'download', 'upload', 'timer', 'bulb', 'power', 'outlet', 'scene', 'script', 'button', 'list', 'number', 'thermometer', 'droplet', 'gauge', 'lock', 'brightness', 'color-temp', 'color', 'effect'];
export const tests = [['icons: every contract name and the M3 UI glyphs are drawn on the 24 grid', () => {
  const { E } = loadEngine(['hd-core.js', 'hd-icons.js'], { dom: true });
  for (const n of CONTRACT.concat(NEW)) { assert.ok(E.icons.has(n), n); const svg = E.icons.icon(n); assert.equal(svg.getAttribute('data-hd-icon'), n); assert.ok(svg.querySelectorAll('path').length > 0, n); } }]];
```
- [ ] Run `npm run test:homedash` → FAIL (`memory`).
- [ ] Add to `PATHS` after `refresh`:
```js
        memory: ['M4 7h16v10H4z', 'M8 17v3', 'M12 17v3', 'M16 17v3', 'M7.5 10.5h2.5v3H7.5z', 'M14 10.5h2.5v3H14z'],
        download: ['M12 4v11', 'M7.5 10.5L12 15l4.5-4.5', 'M5 19.5h14'], upload: ['M12 15V4', 'M7.5 8.5L12 4l4.5 4.5', 'M5 19.5h14'],
        timer: ['M12 5.5a7.5 7.5 0 1 0 0 15a7.5 7.5 0 1 0 0-15z', 'M12 9.5V13l2.5 2', 'M10 2.5h4', 'M12 2.5v3', 'M18.4 6.6l1.4-1.4'],
        outlet: ['M6 3.5h12a2.5 2.5 0 0 1 2.5 2.5v12a2.5 2.5 0 0 1-2.5 2.5H6A2.5 2.5 0 0 1 3.5 18V6A2.5 2.5 0 0 1 6 3.5z', 'M9.5 9v2.5', 'M14.5 9v2.5', 'M10 16.5a2 2 0 0 1 4 0'],
        scene: ['M11 3.5l1.7 4.3L17 9.5l-4.3 1.7L11 15.5l-1.7-4.3L5 9.5l4.3-1.7z', 'M18 14l.9 2.1 2.1.9-2.1.9L18 20l-.9-2.1L15 17l2.1-.9z'],
        script: ['M6.5 3.5h8l4 4v13h-12z', 'M14.5 3.5v4h4', 'M10.5 11v5.5l4.5-2.75z'],
        button: ['M12 4a8 8 0 1 0 0 16a8 8 0 1 0 0-16z', 'M12 8.5a3.5 3.5 0 1 0 0 7a3.5 3.5 0 1 0 0-7z'],
        list: ['M9 6.5h11', 'M9 12h11', 'M9 17.5h11', 'M4.5 6.5h.01', 'M4.5 12h.01', 'M4.5 17.5h.01'], number: ['M5 9h14', 'M5 15h14', 'M10.5 4L8.5 20', 'M15.5 4l-2 16'],
        brightness: ['M12 8a4 4 0 1 0 0 8a4 4 0 1 0 0-8z', 'M12 2.5v2', 'M12 19.5v2', 'M2.5 12h2', 'M19.5 12h2', 'M5.3 5.3l1.4 1.4', 'M17.3 17.3l1.4 1.4', 'M5.3 18.7l1.4-1.4', 'M17.3 6.7l1.4-1.4'],
        'color-temp': ['M12 3a9 9 0 1 0 0 18a9 9 0 1 0 0-18z', 'M12 3v18', 'M12 7.5L7.8 11.7', 'M12 12.5l-3.5 3.5'],
        color: ['M12 3a9 9 0 0 0 0 18c1.4 0 2-.9 2-1.9 0-.6-.3-1-.6-1.4-.3-.4-.6-.8-.6-1.4 0-1.1.9-1.8 2-1.8h1.7A4.5 4.5 0 0 0 21 11c0-4.4-4-8-9-8z', 'M7.5 11.5h.01', 'M9.5 7.5h.01', 'M14.5 7.5h.01'],
        effect: ['M4 20L14.5 9.5', 'M16 3.5v4', 'M14 5.5h4', 'M19.5 9v3', 'M18 10.5h3', 'M9.5 3.5v2', 'M8.5 4.5h2'],
        'chevron-down': ['M6.5 9.5L12 15l5.5-5.5'], close: ['M6.5 6.5l11 11', 'M17.5 6.5l-11 11'], home: ['M3.5 11L12 4l8.5 7', 'M6 9.5V20h12V9.5', 'M10 20v-5.5h4V20']
```
  Update the file header ("starter set" → "M2/M3 set; ~70 glyphs complete in W4").
- [ ] `npm run test:homedash` → new PASS, no FAIL. Commit `hd-icons.js icons.test.mjs` — `feat(homedash): contract icon names and M3 control glyphs`.

### Task 2: hd-api — commands route and template fields

**Files:** `ui/js/homedash/hd-api.js`, `scripts/homedash-tests/api.test.mjs`. **Impact:** `create` (hd-api), `createFromTemplate` (callers `hd-app.create`, tests; LOW).

- [ ] Add to `api.test.mjs`:
```js
  ['hd-api posts commands and template fields (name, unassigned_title) for home_assistant', async () => {
    const E = load(), tr = fakeTransport([{ status: 'ok' }, { status: 'ok' }, { status: 'ok', accepted: true }]), api = E.api.create({ transport: tr });
    await api.createFromTemplate('system', 'System'); await api.createFromTemplate('home_assistant', { name: 'Zuhause', unassigned_title: 'Ohne Raum', other: 'x' });
    await api.command({ entity: 'ha:light.kitchen', command: 'brightness', args: { value: 40 }, request_id: 'r_1', confirmed: false });
    eq(JSON.parse(tr.calls[0].init.body), { template: 'system', name: 'System' });
    eq(JSON.parse(tr.calls[1].init.body), { template: 'home_assistant', name: 'Zuhause', unassigned_title: 'Ohne Raum' });
    assert.equal(tr.calls[2].init.method + ' ' + tr.calls[2].url, 'POST /api/desktop/homedash/commands');
    eq(JSON.parse(tr.calls[2].init.body).args, { value: 40 }); }]
```
- [ ] Implement in `create()`:
```js
        const templateBody = (template, fields) => { const f = typeof fields === 'string' ? { name: fields } : fields && typeof fields === 'object' ? fields : {}, out = { template };
            for (const k of ['name', 'unassigned_title']) if (typeof f[k] === 'string' && f[k]) out[k] = f[k]; return out; };
        // in the frozen object:
            createFromTemplate: (template, fields, extra) => call('POST', 'dashboards', null, templateBody(template, fields), extra),
            command: (body, extra) => call('POST', 'commands', null, body, extra),
```
  Header: routes now include `commands`; `normalizeError` already keeps `data.source`, `data.blocked_by`, `retryAfter`.
- [ ] Test PASS. Commit — `feat(homedash): hd-api commands route and template fields`.

### Task 3: hd-input.js

**Files:** create `ui/js/homedash/hd-input.js`, `scripts/homedash-tests/input.test.mjs`; wiring step W (harness `ENGINE_ORDER` after `hd-geometry.js`, module-loader entry, `TestHomeDashWiring` engine list, `ui/testdata/homedash-lab.html` script list, `contract.test.mjs` `PARTS['hd-input.js']='input'`). **Impact:** `ENGINE_ORDER` (many tests; MEDIUM — run the whole suite), module-loader `homedash` entry, `TestHomeDashWiring`.

- [ ] `input.test.mjs` (fake DOM; `win` falls back to the element, so dispatch everything on the element):
```js
import assert from 'node:assert/strict'; import { loadEngine } from './harness.mjs'; import { fakeClock } from './live-fakes.mjs';
const boot = () => loadEngine(['hd-core.js', 'hd-input.js'], { dom: true });
const pe = (type, x, y, extra) => Object.assign({ type, pointerId: 1, isPrimary: true, button: 0, pointerType: 'touch', clientX: x, clientY: y, preventDefault() {} }, extra);
export const tests = [
  ['input: snap anchors the grid at min and never passes max; keys step, page 10 %, Home/End', () => { const I = boot().E.input, ct = { min: 2202, max: 6535, step: 50 };
    assert.equal(I.snap(6535, ct), 6502); assert.equal(I.snap(2230, ct), 2252); assert.equal(I.snap(2220, ct), 2202); assert.equal(I.keyValue('End', 3000, ct), 6502); assert.equal(I.keyValue('Home', 3000, ct), 2202);
    const pct = { min: 0, max: 100, step: 1 }; assert.equal(I.keyValue('ArrowRight', 40, pct), 41); assert.equal(I.keyValue('PageDown', 40, pct), 30); assert.equal(I.keyValue('ArrowUp', 100, pct), 100);
    assert.equal(I.keyValue('ArrowUp', 55, { min: 0, max: 100, step: 5 }), 60); assert.equal(I.snap(0.30000000004, { min: 0, max: 1, step: 0.1 }), 0.3); assert.equal(I.keyValue('a', 1, pct), null); }],
  ['input: fractions for tracks and the dial (15 deg dead zone keeps the last value deep in the gap)', () => { const I = boot().E.input, r = { left: 0, top: 0, width: 200, height: 200 };
    assert.equal(I.fraction(50, 0, r, 'x'), 0.25); assert.equal(I.fraction(0, 50, r, 'y'), 0.75);
    assert.equal(I.dialFraction(100, 0, r, 0.3), 0.5); assert.equal(I.dialFraction(200, 100, r, 0.3), 225 / 270);
    assert.equal(I.dialFraction(100, 200, r, 0.3), 0.3); assert.equal(I.verticalFraction(0.5, -110), 1); }],
  ['input: gesture splits tap and drag at 4 px, holds after 500 ms, swallows the click after a drag', async () => { const { E, document } = boot(), clock = fakeClock(), el = document.createElement('div'), log = [];
    E.input.gesture(el, { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, rect: () => ({ left: 0, top: 0, width: 100, height: 44 }),
      onTap: () => log.push('tap'), onHold: () => log.push('hold'), onDragStart: () => log.push('start'), onDrag: p => log.push('drag:' + p.x), onDragEnd: () => log.push('end') });
    el.dispatchEvent(pe('pointerdown', 10, 10)); el.dispatchEvent(pe('pointermove', 13, 10)); el.dispatchEvent(pe('pointerup', 13, 10)); assert.deepEqual(log, ['tap']);
    log.length = 0; el.dispatchEvent(pe('pointerdown', 10, 10)); el.dispatchEvent(pe('pointermove', 15, 10)); el.dispatchEvent(pe('pointermove', 30, 10)); el.dispatchEvent(pe('pointerup', 30, 10));
    assert.deepEqual(log, ['start', 'drag:15', 'drag:30', 'end']);
    log.length = 0; el.dispatchEvent(pe('pointerdown', 10, 10)); await clock.advance(500); el.dispatchEvent(pe('pointerup', 10, 10)); assert.deepEqual(log, ['hold']);
    log.length = 0; el.dispatchEvent(pe('pointerdown', 10, 10, { isPrimary: false })); el.dispatchEvent(pe('pointerup', 10, 10, { isPrimary: false })); assert.deepEqual(log, []); }],
  ['input: press activates on click, not after a move or a hold; Shift+Enter runs hold; a disabled control reports blocked', async () => { const { E, document } = boot(), clock = fakeClock(), b = document.createElement('button'), log = []; let ok = true;
    E.input.press(b, { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, enabled: () => ok, onBlocked: () => log.push('blocked'), onActivate: () => log.push('act'), onHold: () => log.push('hold') });
    b.dispatchEvent({ type: 'click' }); b.dispatchEvent(pe('pointerdown', 0, 0)); b.dispatchEvent(pe('pointermove', 9, 0)); b.dispatchEvent(pe('pointerup', 9, 0)); b.dispatchEvent({ type: 'click', preventDefault() {} });
    b.dispatchEvent({ type: 'keydown', key: 'Enter', shiftKey: true, preventDefault() {} }); ok = false; b.dispatchEvent({ type: 'click' });
    assert.deepEqual(log, ['act', 'hold', 'blocked']); }],
  ['input: repeat steps once per click and 400 ms then every 80 ms while held', async () => { const { E, document } = boot(), clock = fakeClock(), b = document.createElement('button'); let n = 0;
    E.input.repeat(b, { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, onStep: () => n++ });
    b.dispatchEvent({ type: 'click' }); assert.equal(n, 1); b.dispatchEvent(pe('pointerdown', 0, 0)); await clock.advance(400 + 80 * 3); b.dispatchEvent(pe('pointerup', 0, 0)); b.dispatchEvent({ type: 'click', preventDefault() {} }); assert.equal(n, 5); }]
];
```
  (Dry-run 2026-11-25: Tasks 3 and 4 code and tests as written pass, 5/5 and 11/11, plus the engine guard.)
- [ ] Implement `hd-input.js` exactly as shown (all binders are complete):
```js
// HomeDash input (P1, decision D5; design-frontend 6, 10). Pure maths for keyboard and pointer values, and binders on pointer events (never HTML5
// drag and drop). press(): buttons (activation through the native click, so Enter and Space work; 500 ms hold; a move of 4 px or more swallows the
// click). gesture(): draggable controls (pointer capture, 4 px between tap and drag, hold). keys(): Arrow +-step, PageUp/PageDown +-10 % of the range
// (at least one step), Home/End, Enter/Space toggle where a control has one, Shift+Enter and ContextMenu run hold. repeat(): one step per click,
// repeats while held (400 ms, then 80 ms). Primary pointer only; a mouse gesture also ends on a move without a pressed button. touch-action is CSS.
// Values snap to a grid anchored at min that never passes max (contract M3 section 4). Timers, clock and window are injectable.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-input.js: load hd-core.js first');
    const THRESHOLD_PX = 4, HOLD_MS = 500, REPEAT_FIRST_MS = 400, REPEAT_MS = 80, SWALLOW_MS = 400, DEAD_ZONE_DEG = 15, DIAL_PX = 220;
    const finite = v => typeof v === 'number' && Number.isFinite(v);
    const places = n => { const s = String(n), i = s.indexOf('.'); return i < 0 ? 0 : Math.min(6, s.length - i - 1); };
    function range(r) { const x = r || {}, min = finite(x.min) ? x.min : 0, max = finite(x.max) && x.max > min ? x.max : min + 100, step = finite(x.step) && x.step > 0 ? x.step : 1;
        return Object.freeze({ min, max, step, top: min + Math.floor((max - min) / step + 1e-9) * step, dp: Math.max(places(step), places(min)) }); }
    const fix = (v, dp) => Number(v.toFixed(dp));
    function snap(value, r) { const g = range(r); if (!finite(value)) return null; const k = Math.round((Math.min(g.max, Math.max(g.min, value)) - g.min) / g.step); return fix(Math.min(g.top, g.min + k * g.step), g.dp); }
    function page(r) { const g = range(r); return Math.max(g.step, Math.round((g.max - g.min) / 10 / g.step) * g.step); }
    function keyValue(key, value, r) { const g = range(r), v = finite(value) ? value : g.min;
        switch (key) { case 'ArrowRight': case 'ArrowUp': return snap(v + g.step, g); case 'ArrowLeft': case 'ArrowDown': return snap(v - g.step, g);
            case 'PageUp': return snap(v + page(g), g); case 'PageDown': return snap(v - page(g), g); case 'Home': return fix(g.min, g.dp); case 'End': return fix(g.top, g.dp); default: return null; } }
    function fraction(x, y, rect, axis) { if (!rect) return 0; const f = axis === 'y' ? (rect.height > 0 ? (rect.top + rect.height - y) / rect.height : 0) : (rect.width > 0 ? (x - rect.left) / rect.width : 0); return Math.min(1, Math.max(0, f)); }
    const valueAt = (f, r) => { const g = range(r); return snap(g.min + Math.min(1, Math.max(0, f)) * (g.max - g.min), g); };
    // Angles from 12 o'clock clockwise; the arc starts at `start` (-135) and sweeps `sweep` (270). Within 15 deg past an end the value pins to it; deeper in the gap the last fraction stays.
    function dialFraction(x, y, rect, last, start, sweep) { const s0 = finite(start) ? start : -135, sw = finite(sweep) ? sweep : 270, cx = rect.left + rect.width / 2, cy = rect.top + rect.height / 2;
        const rel = ((Math.atan2(x - cx, cy - y) * 180 / Math.PI + 360) % 360 - s0 + 720) % 360;
        if (rel <= sw) return rel / sw; if (rel - sw <= DEAD_ZONE_DEG) return 1; if (360 - rel <= DEAD_ZONE_DEG) return 0; return finite(last) ? last : (rel - sw < 360 - rel ? 1 : 0); }
    const verticalFraction = (from, dy) => Math.min(1, Math.max(0, from - dy / DIAL_PX));
    const isHoldKey = ev => !!ev && ((ev.key === 'Enter' && ev.shiftKey) || ev.key === 'ContextMenu');
    const stop = ev => { if (ev && typeof ev.preventDefault === 'function') ev.preventDefault(); };
    const primary = ev => ev.isPrimary !== false && !(ev.pointerType === 'mouse' && ev.button > 0);
    function env(el, o) { const doc = el.ownerDocument; return { win: o.win || (doc && doc.defaultView) || el, setT: o.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT: o.clearTimeout || (id => clearTimeout(id)), now: o.now || (() => Date.now()), life: E.core.bag() }; }
    const allowed = (o, ev) => { if (!o.enabled || o.enabled()) return true; if (o.onBlocked) o.onBlocked(ev); return false; };
    function press(el, opts) { const o = opts || {}, ctx = env(el, o); let g = null, swallowAt = -Infinity, heldAt = -Infinity;
        function drop() { const s = g; g = null; if (s) { if (s.timer !== null) ctx.clearT(s.timer); s.bag.dispose(); } return s; }
        function end(cancelled) { const s = drop(); if (s && (s.moved || s.held || cancelled)) swallowAt = ctx.now(); }
        function hold(ev) { heldAt = ctx.now(); o.onHold(ev); } // never gated by enabled: more-info works when read-only (rule 11)
        ctx.life.on(el, 'pointerdown', ev => { if (!primary(ev)) return; drop(); // a stale gesture (lost pointerup) ends silently
            g = { id: ev.pointerId, type: ev.pointerType, x: ev.clientX, y: ev.clientY, moved: false, held: false, timer: null, bag: E.core.bag() };
            const s = g, cap = { capture: true }, mine = e => g === s && e.pointerId === s.id;
            if (o.onHold) s.timer = ctx.setT(() => { s.timer = null; if (g === s && !s.moved) { s.held = true; hold(null); } }, HOLD_MS);
            s.bag.on(ctx.win, 'pointermove', e => { if (!mine(e)) return;
                if (s.type === 'mouse' && e.buttons === 0) { drop(); return; } // the button came up outside the window: no click follows
                if (!s.moved && Math.hypot(e.clientX - s.x, e.clientY - s.y) >= THRESHOLD_PX) { s.moved = true; if (s.timer !== null) { ctx.clearT(s.timer); s.timer = null; } } }, cap);
            s.bag.on(ctx.win, 'pointerup', e => { if (mine(e)) end(false); }, cap);
            s.bag.on(ctx.win, 'pointercancel', e => { if (mine(e)) end(true); }, cap); });
        ctx.life.on(el, 'click', ev => { if (ctx.now() - swallowAt < SWALLOW_MS) { swallowAt = -Infinity; stop(ev); return; } // swallow one click only
            if (o.onActivate && allowed(o, ev)) o.onActivate(ev); });
        ctx.life.on(el, 'keydown', ev => { if (ev.altKey || ev.ctrlKey || ev.metaKey || !o.onHold || !isHoldKey(ev)) return; stop(ev); hold(ev); });
        ctx.life.on(el, 'contextmenu', ev => { if ((g && g.type !== 'mouse') || ctx.now() - heldAt < SWALLOW_MS) stop(ev); });
        ctx.life.add(drop);
        return () => ctx.life.dispose(); }
    // gesture() binds no keyboard: keys() owns it (pass onHold to keys() too, e.g. the light tile and bindLinear), so Shift+Enter runs hold once.
    function gesture(el, opts) { const o = opts || {}, ctx = env(el, o); let g = null, swallowAt = -Infinity;
        const rect = () => (o.rect ? o.rect() : typeof el.getBoundingClientRect === 'function' ? el.getBoundingClientRect() : null);
        const point = ev => ({ x: ev.clientX, y: ev.clientY, rect: rect(), type: ev.type });
        function drop() { const s = g; g = null; if (!s) return null;
            if (s.timer !== null) ctx.clearT(s.timer); s.bag.dispose(); if (s.drag) el.removeAttribute('data-hd-dragging');
            if (s.captured && typeof el.releasePointerCapture === 'function') { try { el.releasePointerCapture(s.id); } catch (err) { /* released with the pointerup */ } }
            return s; }
        // how: 'up'; 'cancel' (pointercancel or a replaced gesture; its coordinates may be 0/0, so a drag ends at the last move point);
        // 'lost' (mouse move without a button: the up happened outside the window, no click follows). A started drag always ends in onDragEnd (rule 6 commits).
        function finish(ev, how) { const s = drop(); if (!s) return; const cancelled = how === 'cancel';
            if (how !== 'lost' && (s.held || s.moved || cancelled)) swallowAt = ctx.now();
            if (s.drag) { if (o.onDragEnd) o.onDragEnd(cancelled ? s.last : point(ev), s.start, cancelled); return; }
            if (how === 'up' && !s.held && !s.moved && o.onTap && allowed(o, ev)) o.onTap(s.start); }
        ctx.life.on(el, 'pointerdown', ev => { if (!primary(ev)) return; if (g) finish(null, 'cancel');
            if (ev.pointerType === 'mouse') { stop(ev); if (typeof el.focus === 'function') el.focus({ preventScroll: true }); }
            const start = point(ev);
            g = { id: ev.pointerId, type: ev.pointerType, start, last: start, moved: false, held: false, drag: false, captured: false, timer: null, bag: E.core.bag() };
            const s = g, cap = { capture: true }, mine = e => g === s && e.pointerId === s.id;
            if (typeof el.setPointerCapture === 'function') { try { el.setPointerCapture(ev.pointerId); s.captured = true; } catch (err) { /* synthetic pointer: the window listeners carry it */ } }
            if (o.onHold) s.timer = ctx.setT(() => { s.timer = null; if (g === s && !s.moved) { s.held = true; o.onHold(null); } }, HOLD_MS);
            s.bag.on(ctx.win, 'pointermove', e => { if (!mine(e)) return;
                if (s.type === 'mouse' && e.buttons === 0) { finish(e, 'lost'); return; }
                if (s.held) return;
                if (!s.moved) { if (Math.hypot(e.clientX - start.x, e.clientY - start.y) < THRESHOLD_PX) return;
                    s.moved = true; if (s.timer !== null) { ctx.clearT(s.timer); s.timer = null; }
                    s.drag = allowed(o, e); if (s.drag) { el.setAttribute('data-hd-dragging', 'true'); if (o.onDragStart) o.onDragStart(start); } }
                if (s.drag && g === s) { s.last = point(e); if (o.onDrag) o.onDrag(s.last, start); } }, cap);
            s.bag.on(ctx.win, 'pointerup', e => { if (mine(e)) finish(e, 'up'); }, cap);
            s.bag.on(ctx.win, 'pointercancel', e => { if (mine(e)) finish(e, 'cancel'); }, cap); });
        ctx.life.on(el, 'click', ev => { if (ctx.now() - swallowAt < SWALLOW_MS) { swallowAt = -Infinity; stop(ev); } });
        ctx.life.add(drop);
        return () => ctx.life.dispose(); }
    function keys(el, opts) { const o = opts || {}, life = E.core.bag();
        life.on(el, 'keydown', ev => {
            if (ev.altKey || ev.ctrlKey || ev.metaKey) return;
            if (isHoldKey(ev)) { if (o.onHold) { stop(ev); o.onHold(ev); } return; }
            if ((ev.key === 'Enter' || ev.key === ' ') && o.onToggle) { stop(ev); if (allowed(o, ev)) o.onToggle(ev); return; }
            if (!o.range || !o.onKey) return;
            const cur = o.get ? o.get() : null, next = keyValue(ev.key, cur, o.range()); if (next === null) return;
            stop(ev); if (allowed(o, ev) && next !== cur) o.onKey(next, ev.key); });
        return () => life.dispose(); }
    function repeat(el, opts) { const o = opts || {}, ctx = env(el, o); let g = null, swallowAt = -Infinity;
        function halt(swallow) { const s = g; g = null; if (!s) return; if (s.timer !== null) ctx.clearT(s.timer); s.bag.dispose(); if (swallow && s.repeated) swallowAt = ctx.now(); }
        ctx.life.on(el, 'pointerdown', ev => { if (!primary(ev)) return; halt(false);
            g = { id: ev.pointerId, type: ev.pointerType, repeated: false, timer: null, bag: E.core.bag() };
            const s = g, cap = { capture: true }, mine = e => g === s && e.pointerId === s.id;
            const tick = () => { s.timer = null; if (g !== s) return; s.repeated = true; // set first: the click after release is swallowed even when blocked
                if (!o.onStep || !allowed(o, null)) return; // blocked: announced once, the chain stops
                o.onStep(); if (g === s) s.timer = ctx.setT(tick, REPEAT_MS); };
            s.timer = ctx.setT(tick, REPEAT_FIRST_MS);
            s.bag.on(ctx.win, 'pointerup', e => { if (mine(e)) halt(true); }, cap);
            s.bag.on(ctx.win, 'pointercancel', e => { if (mine(e)) halt(true); }, cap);
            s.bag.on(ctx.win, 'pointermove', e => { if (mine(e) && s.type === 'mouse' && e.buttons === 0) halt(false); }, cap);
            s.bag.on(el, 'pointerleave', e => { if (mine(e)) halt(false); }); }); // the up lands elsewhere: no click to swallow
        ctx.life.on(el, 'click', ev => { if (ctx.now() - swallowAt < SWALLOW_MS) { swallowAt = -Infinity; stop(ev); return; } if (o.onStep && allowed(o, ev)) o.onStep(); });
        ctx.life.on(el, 'contextmenu', ev => { if (g && g.type !== 'mouse') stop(ev); });
        ctx.life.add(() => halt(false));
        return () => ctx.life.dispose(); }
    E.input = Object.freeze({ THRESHOLD_PX, HOLD_MS, REPEAT_FIRST_MS, REPEAT_MS, DEAD_ZONE_DEG, DIAL_PX, range, snap, page, keyValue, fraction, valueAt, dialFraction, verticalFraction, isHoldKey, press, gesture, keys, repeat });
})();
```
- [ ] Wiring step W. Run `npm run test:homedash` (all PASS), `go test ./ui -run '(?i)^(TestHomeDashWiring|TestHomeDashLab.*)$' -count=1`.
- [ ] Commit the eight paths — `feat(homedash): input maths and pointer/keyboard binders (P1)`.

### Task 4: hd-command.js (full code)

**Files:** create `ui/js/homedash/hd-command.js`, `scripts/homedash-tests/command.test.mjs`; wiring step W (`APP_ORDER` after `hd-store.js`). **Impact:** `APP_ORDER`, module-loader entry, `TestHomeDashWiring`.

- [ ] `command.test.mjs`:
```js
import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs'; import { fakeClock } from './live-fakes.mjs';
const cap = (name, x) => Object.assign({ name, allowed: true }, x);
const LIGHT = { id: 'ha:light.kitchen', kind: 'light', value: true, available: true, status: 'ok', attributes: { brightness_pct: 40 }, capabilities: [cap('toggle'), cap('turn_on'), cap('turn_off'), cap('brightness', { min: 0, max: 100, step: 1 }), cap('color_temp', { min: 2202, max: 6535, step: 50 }), cap('color')] };
const SW = { id: 'ha:switch.coffee', kind: 'switch', value: false, available: true, status: 'ok', attributes: {}, capabilities: [cap('toggle'), cap('turn_on'), cap('turn_off')] };
const SCRIPT = { id: 'ha:script.goodnight', kind: 'script', value: false, available: true, status: 'ok', attributes: {}, capabilities: [cap('activate', { sensitive: true })] };
const NUM = { id: 'ha:number.boiler_target', kind: 'number', value: 55, available: true, status: 'ok', attributes: {}, capabilities: [cap('set_value', { min: 30, max: 70, step: 1 })] };
function setup() {
  const { E } = loadEngine(['hd-core.js', 'hd-format.js', 'hd-api.js', 'hd-command.js']), clock = fakeClock(), stream = E.core.emitter(), views = new Map([LIGHT, SW, SCRIPT, NUM].map(v => [v.id, v])), calls = []; let n = 0;
  const api = { base: '/b', command(body) { let ok, no; const p = new Promise((a, b) => { ok = a; no = b; }); calls.push({ body, at: clock.now(), ok, no }); return p; } };
  const cmd = E.command.create({ api, stream, store: { view: id => views.get(id) || null }, setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, requestId: () => 'r' + (++n) });
  const fail = (status, body) => Object.assign(new Error('HTTP ' + status), { status, body });
  const frame = (id, st) => stream.emit('state', { changes: { [id]: Object.assign({ available: true, updated_at: 'x' }, st) } });
  return { E, clock, views, calls, cmd, fail, frame, d: (id, k) => cmd.decorate(id, views.get(id), k) };
}
const exp = (value, attributes) => ({ status: 'ok', accepted: true, superseded: false, expected: Object.assign({ value, available: true, updated_at: 'x' }, attributes ? { attributes } : {}) });
export const tests = [
  ['command: predict mirrors the contract expected table; matches uses the tolerances', () => { const { E } = setup(), P = E.command.predict, M = E.command.matches;
    eq(P(SW, 'turn_on', {}), { value: true }); eq(P(LIGHT, 'brightness', { value: 0 }), { value: false }); eq(P(LIGHT, 'brightness', { value: 60 }), { value: true, attributes: { brightness_pct: 60 } });
    eq(P(LIGHT, 'color', { h: 10, s: 50 }), { value: true, attributes: { hs_color: [10, 50], color_mode: 'hs' } }); assert.equal(P(SCRIPT, 'activate', {}), null); eq(P(NUM, 'set_value', { value: 56 }), { value: 56 });
    const st = a => ({ value: true, available: true, attributes: a });
    assert.ok(M(st({ brightness_pct: 61 }), { value: true, attributes: { brightness_pct: 60 } }, 1)); assert.ok(!M(st({ brightness_pct: 62 }), { value: true, attributes: { brightness_pct: 60 } }, 1));
    assert.ok(M(st({ color_temp_kelvin: 3060 }), { value: true, attributes: { color_temp_kelvin: 3000 } })); assert.ok(!M(st({ color_temp_kelvin: 3070 }), { value: true, attributes: { color_temp_kelvin: 3000 } }));
    assert.ok(M(st({ hs_color: [358, 52] }), { value: true, attributes: { hs_color: [2, 49] } })); assert.ok(!M(st({ hs_color: [8, 50] }), { value: true, attributes: { hs_color: [2, 50] } }));
    assert.ok(M({ value: 55.4, available: true }, { value: 55 }, 1)); assert.ok(!M({ value: 55.6, available: true }, { value: 55 }, 1)); assert.ok(!M({ value: true, available: false }, { value: true })); }],
  ['command: the overlay applies at once, pending appears after 250 ms (not before), a matching frame confirms and flashes 600 ms', async () => { const { clock, calls, cmd, frame, d, views } = setup();
    cmd.send({ entity: SW.id, command: 'turn_on', args: {}, widgetKey: 'w1' }); assert.equal(d(SW.id).value, true); assert.equal(d(SW.id).pending, false);
    await clock.advance(249); assert.equal(d(SW.id).pending, false); await clock.advance(2); assert.equal(d(SW.id).pending, true);
    calls[0].ok(exp(true)); await clock.advance(0); frame(SW.id, { value: true }); assert.equal(d(SW.id).pending, false); assert.equal(d(SW.id).confirmed, true);
    await clock.advance(600); assert.equal(d(SW.id), views.get(SW.id)); }],
  // after the 600 ms flash decorate() returns the store view itself (same reference), so the strict equal holds
  ['command: a frame before the 202 confirms when expected arrives; the server state already equal at the 202 confirms; duplicates are harmless', async () => { const { clock, calls, cmd, frame, d, views } = setup();
    cmd.send({ entity: SW.id, command: 'turn_on', args: {} }); frame(SW.id, { value: true }); calls[0].ok(exp(true)); await clock.advance(0); frame(SW.id, { value: true }); assert.equal(d(SW.id).pending, false); assert.equal(d(SW.id).confirmed, true);
    cmd.send({ entity: LIGHT.id, command: 'turn_on', args: {} }); calls[1].ok(exp(true)); await clock.advance(0); assert.equal(d(LIGHT.id).confirmed, true); assert.equal(views.size, 4); }],
  ['command: superseded ends silently; momentary commands confirm on the 202', async () => { const { clock, calls, cmd, d, views } = setup();
    cmd.send({ entity: NUM.id, command: 'set_value', args: { value: 60 }, phase: 'commit' }); calls[0].ok({ status: 'ok', accepted: true, superseded: true }); await clock.advance(0); assert.equal(d(NUM.id), views.get(NUM.id));
    cmd.send({ entity: SCRIPT.id, command: 'activate', args: {}, confirmed: true, widgetKey: 'w' }); }],
  ['command: an error rolls back at once with a 6 s cue; 403 read_only marks the entity read-only with its reason and no cue', async () => { const { clock, calls, cmd, fail, d } = setup();
    cmd.send({ entity: SW.id, command: 'turn_on', args: {} }); calls[0].no(fail(503, { status: 'error', code: 'source_unavailable', source: 'ha' })); await clock.advance(0);
    assert.equal(d(SW.id).value, false); assert.equal(d(SW.id).status, 'error'); eq(d(SW.id).error, { code: 'source_unavailable', source: 'ha', blockedBy: '' }); await clock.advance(6000); assert.equal(d(SW.id).status, 'ok');
    cmd.send({ entity: SW.id, command: 'turn_on', args: {} }); calls[1].no(fail(403, { status: 'error', code: 'read_only', blocked_by: 'home_assistant.allowed_services' })); await clock.advance(0);
    assert.equal(d(SW.id).status, 'ok'); assert.equal(d(SW.id).readonly, true); assert.equal(d(SW.id).readonlyReason, 'home_assistant.allowed_services'); }],
  ['command: 8 s without confirmation rolls back with the timeout cue unless a frame with the expected value arrived (late adoption)', async () => { const { clock, calls, cmd, frame, d } = setup();
    cmd.send({ entity: LIGHT.id, command: 'color_temp', args: { kelvin: 6502 }, phase: 'commit' }); calls[0].ok(exp(true, { color_temp_kelvin: 6502 })); await clock.advance(0);
    frame(LIGHT.id, { value: true, attributes: { color_temp_kelvin: 6300 } }); await clock.advance(8000); assert.equal(d(LIGHT.id).status, 'ok'); assert.ok(!d(LIGHT.id).pending);
    cmd.send({ entity: SW.id, command: 'turn_on', args: {} }); calls[1].ok(exp(true)); await clock.advance(8000); assert.equal(d(SW.id).value, false); eq(d(SW.id).error.code, 'timeout'); }],
  ['command: continuous input sends at most every 1250 ms with a leading edge, the final always on commit, and drops a 429 on an intermediate', async () => { const { clock, calls, cmd, fail, d } = setup();
    for (let i = 0; i < 180; i++) { cmd.send({ entity: LIGHT.id, command: 'brightness', args: { value: 10 + Math.floor(i / 3) }, phase: 'input' }); await clock.advance(1000 / 60); }
    assert.equal(calls.length, 3); for (let i = 1; i < calls.length; i++) assert.ok(calls[i].at - calls[i - 1].at >= 1250);
    calls[1].no(fail(429, { code: 'rate_limited', retry_after_ms: 900 })); await clock.advance(0); assert.equal(d(LIGHT.id).attributes.brightness_pct, 69);
    cmd.send({ entity: LIGHT.id, command: 'brightness', args: { value: 70 }, phase: 'commit' }); assert.equal(calls.length, 4); eq(calls[3].body.args, { value: 70 }); assert.equal(d(LIGHT.id).attributes.brightness_pct, 70); }],
  ['command: a 429 on the final retries once with a new request_id after retry_after_ms while it is the latest', async () => { const { clock, calls, cmd, fail } = setup();
    cmd.send({ entity: NUM.id, command: 'set_value', args: { value: 60 }, phase: 'commit' }); calls[0].no(fail(429, { code: 'rate_limited', retry_after_ms: 700 })); await clock.advance(699); assert.equal(calls.length, 1);
    await clock.advance(1); assert.equal(calls.length, 2); assert.notEqual(calls[1].body.request_id, calls[0].body.request_id); calls[1].no(fail(429, { code: 'rate_limited', retry_after_ms: 700 })); await clock.advance(2000); assert.equal(calls.length, 2); }],
  ['command: keys move the overlay at once and send one command 400 ms after the last key', async () => { const { clock, calls, cmd, d } = setup();
    for (const v of [41, 42, 43]) { cmd.send({ entity: LIGHT.id, command: 'brightness', args: { value: v }, phase: 'key' }); await clock.advance(100); }
    assert.equal(d(LIGHT.id).attributes.brightness_pct, 43); assert.equal(calls.length, 0); await clock.advance(300); assert.equal(calls.length, 1); eq(calls[0].body.args, { value: 43 }); }],
  ['command: confirm-required arms 4 s with a countdown; the second activation sends confirmed:true; 409 arms', async () => { const { clock, calls, cmd, fail, d } = setup();
    const go = () => cmd.send({ entity: SCRIPT.id, command: 'activate', args: {}, widgetKey: 'w9' });
    assert.equal((await go()).status, 'armed'); assert.equal(calls.length, 0); assert.equal(d(SCRIPT.id, 'w9').armed.seconds, 4); await clock.advance(1000); assert.equal(d(SCRIPT.id, 'w9').armed.seconds, 3);
    go(); assert.equal(calls[0].body.confirmed, true); await clock.advance(5000); assert.equal(d(SCRIPT.id, 'w9').armed, undefined);
    cmd.send({ entity: SW.id, command: 'turn_on', args: {}, widgetKey: 'w2' }); calls[1].no(fail(409, { code: 'confirmation_required' })); await clock.advance(0); assert.equal(d(SW.id, 'w2').armed.command, 'turn_on');
    cmd.send({ entity: SW.id, command: 'turn_on', args: {}, widgetKey: 'w2' }); assert.equal(calls[2].body.confirmed, true); }],
  ['command: every action has its own request_id; the body carries dashboard and widget ids', async () => { const { calls, cmd } = setup();
    cmd.send({ entity: SW.id, command: 'turn_on', args: {}, dashboardId: 'd_m3x7q2k5a4bc', widgetId: 'w2' }); cmd.send({ entity: SW.id, command: 'turn_off', args: {} });
    eq(calls[0].body, { entity: SW.id, command: 'turn_on', args: {}, request_id: 'r1', confirmed: false, dashboard_id: 'd_m3x7q2k5a4bc', widget_id: 'w2' }); assert.equal(calls[1].body.request_id, 'r2'); }]
];
```
  (Test 2 relies on `decorate()` returning the store view by reference once no overlay, pending, cue or flash is left; keep that property.)
- [ ] Implement `ui/js/homedash/hd-command.js` exactly:
```js
// HomeDash command client (P1, decision D5; contract "API contract (frozen for M3)" section 4 of docs/superpowers/plans/2026-11-25-homedash-m3-backend.md).
// One client per JS realm (shared(), keyed by the API base path): every window and the more-info sheet see the same overlay. Rules 1-8 of the plan's
// "P1 interaction rules" are the normative text; summary: new request_id per action; overlay at once, pending after 250 ms; confirm by the contract
// match (tolerances, frames from sending, server state at the 202, 202 without expected confirms); superseded silent; errors roll back with a 6 s cue,
// 409 arms, 403 read_only/desktop_readonly mark read-only; 8 s timeout with late adoption; continuous: 1250 ms throttle with leading edge, final on
// commit, 429 intermediate dropped, 429 final retried once; keys: one trailing send after 400 ms; confirm-required arms 4 s. send() never rejects.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.api || !E.format) throw new Error('hd-command.js: load hd-core.js, hd-format.js and hd-api.js first');
    const TIMING = Object.freeze({ PENDING_MS: 250, CONFIRM_MS: 8000, CUE_MS: 6000, THROTTLE_MS: 1250, KEY_MS: 400, ARM_MS: 4000, FLASH_MS: 600, RETRY_MS: 1000 });
    const CONTINUOUS = Object.freeze(['brightness', 'color_temp', 'color', 'set_value']);
    const ARMABLE = Object.freeze(['toggle', 'turn_on', 'turn_off', 'activate', 'press', 'select_option', 'effect']);
    const isObj = v => v !== null && typeof v === 'object' && !Array.isArray(v), finite = v => typeof v === 'number' && Number.isFinite(v);
    function capOf(view, name) { for (const c of view && Array.isArray(view.capabilities) ? view.capabilities : []) { if (c === name) return { name, allowed: true }; if (isObj(c) && c.name === name) return c; } return null; }
    function predict(view, command, args) {
        const a = isObj(args) ? args : {}, on = E.format.isOn(view);
        switch (command) {
            case 'toggle': return on === null ? null : { value: !on };
            case 'turn_on': return { value: true };
            case 'turn_off': return { value: false };
            case 'brightness': return Number(a.value) > 0 ? { value: true, attributes: { brightness_pct: Number(a.value) } } : { value: false };
            case 'color_temp': return { value: true, attributes: { color_temp_kelvin: Number(a.kelvin), color_mode: 'color_temp' } };
            case 'color': return { value: true, attributes: { hs_color: [Number(a.h), Number(a.s)], color_mode: 'hs' } };
            case 'effect': return { value: true, attributes: { effect: String(a.option) } };
            case 'select_option': return { value: String(a.option) };
            case 'set_value': return { value: Number(a.value) };
            default: return null;
        }
    }
    function sameValue(actual, want, step) {
        if (typeof want === 'number') return typeof actual === 'number' && (step > 0 ? Math.abs(actual - want) <= step / 2 + 1e-9 : actual === want);
        return actual === want;
    }
    function attrMatches(key, actual, want) {
        if (key === 'brightness_pct') return finite(actual) && Math.abs(actual - want) <= 1;
        if (key === 'color_temp_kelvin') return finite(actual) && Math.abs(actual - want) <= Math.abs(want) * 0.02;
        if (key === 'hs_color') { if (!Array.isArray(actual) || !Array.isArray(want) || !finite(actual[0]) || !finite(actual[1])) return false;
            return Math.abs(((actual[0] - want[0]) % 360 + 540) % 360 - 180) <= 4 && Math.abs(actual[1] - want[1]) <= 4; }
        return isObj(want) || Array.isArray(want) ? JSON.stringify(actual) === JSON.stringify(want) : actual === want;
    }
    function matches(state, expected, step) {
        if (!isObj(state) || !isObj(expected) || state.available !== true) return false;
        if (Object.prototype.hasOwnProperty.call(expected, 'value') && !sameValue(state.value, expected.value, step)) return false;
        const want = isObj(expected.attributes) ? expected.attributes : {}, have = isObj(state.attributes) ? state.attributes : {};
        for (const k of Object.keys(want)) if (!attrMatches(k, have[k], want[k])) return false;
        return true;
    }
    const valueMatches = (state, expected, step) => isObj(state) && isObj(expected) && state.available === true && sameValue(state.value, expected.value, step);
    const stateOf = v => (v ? { value: v.value, attributes: v.attributes, available: v.available !== false && v.status !== 'unavailable' && v.status !== 'unknown' } : null);

    function create(options) {
        const o = options || {}, api = o.api, stream = o.stream, store = o.store;
        if (!api || typeof api.command !== 'function' || !stream || !store) throw new Error('hd-command: api (with command), stream and store are required');
        const later = o.setTimeout || ((fn, ms) => setTimeout(fn, ms)), clearT = o.clearTimeout || (id => clearTimeout(id));
        const now = o.now || (() => Date.now()), requestId = o.requestId || (() => E.core.randomId('r', 20));
        const bus = E.core.emitter(), overlays = new Map(), cues = new Map(), flashes = new Map(), armed = new Map(), lanes = new Map(), locks = new Map(), unhook = [];
        let seq = 0, disposed = false;
        const emit = id => { if (!disposed) bus.emit('change', [id]); };
        const dropCue = id => { const c = cues.get(id); if (c) { clearT(c.timer); cues.delete(id); } };
        function stop(rec) { clearT(rec.pendingTimer); clearT(rec.confirmTimer); rec.pendingTimer = rec.confirmTimer = 0; }
        function release(rec) { if (overlays.get(rec.id) !== rec) return false; stop(rec); overlays.delete(rec.id); return true; }
        function cue(id, entry) {
            dropCue(id); const c = { entry: Object.freeze(entry), timer: 0 };
            c.timer = later(() => { if (cues.get(id) === c) { cues.delete(id); emit(id); } }, TIMING.CUE_MS);
            cues.set(id, c); bus.emit('cue', { id, cue: c.entry });
        }
        function confirmRec(rec) {   // not named after the browser dialog: the engine JS guard rejects that call name
            if (!release(rec)) return;
            const old = flashes.get(rec.id); if (old) clearT(old);
            flashes.set(rec.id, later(() => { flashes.delete(rec.id); emit(rec.id); }, TIMING.FLASH_MS));
            emit(rec.id); bus.emit('confirmed', { id: rec.id, command: rec.command });
        }
        function settle(rec) { if (release(rec)) emit(rec.id); }
        function rollback(rec, e) {
            if (!release(rec)) return;
            const code = (e && e.code) || 'timeout', data = (e && e.data) || {};
            if (code === 'read_only' || code === 'desktop_readonly') {
                const reason = code === 'desktop_readonly' ? 'virtual_desktop.readonly' : (typeof data.blocked_by === 'string' && data.blocked_by) || 'homedash.readonly';
                locks.set(rec.id, reason); bus.emit('readonly', { id: rec.id, reason, desktop: code === 'desktop_readonly' });
            } else cue(rec.id, { code, source: typeof data.source === 'string' ? data.source : '', blockedBy: '' });
            emit(rec.id);
        }
        function track(s, patch, cap) {
            const id = s.entity, prev = overlays.get(id); if (prev) stop(prev);
            const rec = { id, seq: ++seq, command: s.command, patch, step: finite(cap && cap.step) ? cap.step : 0, expected: null, frame: null, pending: false, pendingTimer: 0, confirmTimer: 0, sent: true };
            overlays.set(id, rec); dropCue(id);
            rec.pendingTimer = later(() => { if (overlays.get(id) === rec) { rec.pending = true; emit(id); } }, TIMING.PENDING_MS);
            rec.confirmTimer = later(() => expire(rec), TIMING.CONFIRM_MS);
            emit(id); return rec;
        }
        function local(s, patch) {
            const id = s.entity, prev = overlays.get(id); if (prev && prev.sent) stop(prev);
            const rec = prev && !prev.sent ? prev : { id, seq: ++seq, command: s.command, patch: null, step: 0, expected: null, frame: null, pending: false, pendingTimer: 0, confirmTimer: 0, sent: false };
            rec.patch = patch; overlays.set(id, rec); dropCue(id); emit(id);
        }
        function expire(rec) {
            if (overlays.get(rec.id) !== rec) return;
            if (rec.expected && rec.frame && valueMatches(rec.frame, rec.expected, rec.step)) { settle(rec); return; }   // late adoption (lead answer Q3)
            rollback(rec, { code: 'timeout', data: {} });
        }
        function post(s, confirmed) {
            const body = { entity: s.entity, command: s.command, args: isObj(s.args) ? s.args : {}, request_id: requestId(), confirmed: !!confirmed };
            if (typeof s.dashboardId === 'string' && s.dashboardId) body.dashboard_id = s.dashboardId;
            if (typeof s.widgetId === 'string' && s.widgetId) body.widget_id = s.widgetId;
            return api.command(body);
        }
        function dispatch(s, cap, patch, confirmed, retried) {
            const rec = track(s, patch, cap);
            return post(s, confirmed).then(answer => accepted(rec, answer), err => failed(rec, E.api.normalizeError(err), s, cap, patch, confirmed, retried));
        }
        function accepted(rec, answer) {
            const a = isObj(answer) ? answer : {};
            if (disposed || overlays.get(rec.id) !== rec) return { status: 'stale' };
            if (a.superseded === true) { settle(rec); return { status: 'superseded' }; }
            if (!isObj(a.expected)) { confirmRec(rec); return { status: 'confirmed' }; }
            rec.expected = a.expected;
            if ((rec.frame && matches(rec.frame, rec.expected, rec.step)) || matches(stateOf(store.view(rec.id)), rec.expected, rec.step)) confirmRec(rec);
            return { status: 'accepted' };
        }
        function failed(rec, e, s, cap, patch, confirmed, retried) {
            if (disposed) return { status: 'failed', code: e.code };
            const current = overlays.get(rec.id) === rec;
            if (e.code === 'confirmation_required') { if (current) settle(rec); arm(s); return { status: 'armed' }; }
            if (e.status === 429 && current && !retried && CONTINUOUS.includes(s.command)) {
                const wait = finite(e.retryAfter) && e.retryAfter > 0 ? Math.round(e.retryAfter * 1000) : TIMING.RETRY_MS;
                return new Promise(resolve => later(() => resolve(disposed || overlays.get(rec.id) !== rec ? { status: 'stale' } : dispatch(s, cap, patch, confirmed, true)), wait));
            }
            if (current) rollback(rec, e);
            return { status: 'failed', code: e.code };
        }
        function arm(s) {
            const key = s.widgetKey || s.entity + '|-'; disarm(key);
            const a = { id: s.entity, command: s.command, until: now() + TIMING.ARM_MS, timers: [] };
            for (let k = 1; k < 4; k++) a.timers.push(later(() => emit(a.id), k * 1000));
            a.timers.push(later(() => { if (armed.get(key) === a) { armed.delete(key); emit(a.id); } }, TIMING.ARM_MS));
            armed.set(key, a); emit(a.id); bus.emit('armed', { id: a.id, widgetKey: key });
        }
        function disarm(key) { const a = armed.get(key); if (!a) return; for (const t of a.timers) clearT(t); armed.delete(key); emit(a.id); }
        function discrete(s, cap, patch) {
            const key = s.widgetKey || s.entity + '|-', a = armed.get(key), armedHere = !!a && a.command === s.command && a.id === s.entity;   // a 409 arms too (rule 4)
            if ((s.confirm === true && ARMABLE.includes(s.command)) || cap.sensitive === true || armedHere) {
                if (!s.confirmed) {
                    if (!armedHere) { const r = overlays.get(s.entity); if (r && !r.sent) settle(r); arm(s); return Promise.resolve({ status: 'armed' }); }
                    disarm(key); s.confirmed = true;
                }
            }
            return dispatch(s, cap, patch, !!s.confirmed, false);
        }
        function lane(s) { const key = s.entity + '|' + s.command; let l = lanes.get(key); if (!l) { l = { key, spec: null, cap: null, patch: null, sentAt: -Infinity, sentSig: '', timer: 0, kind: '' }; lanes.set(key, l); } return l; }
        function intermediate(l) {
            const sig = JSON.stringify(l.spec.args); if (sig === l.sentSig) return;
            l.sentAt = now(); l.sentSig = sig;
            post(l.spec, false).then(() => {}, () => {});   // 429 dropped (D5); any other failure is decided by the final send
        }
        function finish(l) {
            clearT(l.timer); l.timer = 0; l.kind = ''; l.sentAt = now(); l.sentSig = JSON.stringify(l.spec.args);
            return CONTINUOUS.includes(l.spec.command) ? dispatch(l.spec, l.cap, l.patch, false, false) : discrete(l.spec, l.cap, l.patch);
        }
        function send(spec) {
            if (disposed) return Promise.resolve({ status: 'disposed' });
            const s = Object.assign({}, spec), view = store.view(s.entity), cap = capOf(view, s.command);
            if (!view || !cap || cap.allowed === false) return Promise.resolve({ status: 'refused' });
            let phase = s.phase || 'discrete';
            if ((phase === 'input' || phase === 'commit') && !CONTINUOUS.includes(s.command)) phase = 'discrete';
            const patch = predict(decorate(s.entity, view), s.command, s.args);
            if (phase === 'discrete') return discrete(s, cap, patch);
            const l = lane(s); l.spec = s; l.cap = cap; l.patch = patch; local(s, patch);
            if (phase === 'input') {
                if (l.kind !== 'input') { clearT(l.timer); l.timer = 0; } l.kind = 'input';
                if (!l.timer) l.timer = later(() => { l.timer = 0; intermediate(l); }, Math.max(0, l.sentAt + TIMING.THROTTLE_MS - now()));
                return Promise.resolve({ status: 'local' });
            }
            if (phase === 'key') { clearT(l.timer); l.kind = 'key'; l.timer = later(() => { l.timer = 0; finish(l); }, TIMING.KEY_MS); return Promise.resolve({ status: 'local' }); }
            return finish(l);   // commit
        }
        function decorate(id, view, widgetKey) {
            if (!view) return view;
            const rec = overlays.get(id), c = cues.get(id), fl = flashes.has(id), a = widgetKey ? armed.get(widgetKey) : null, lock = locks.get(id);
            if (!rec && !c && !fl && !(a && a.id === id) && !lock) return view;
            const out = Object.assign({}, view);
            if (rec && rec.patch) {
                if (Object.prototype.hasOwnProperty.call(rec.patch, 'value')) out.value = rec.patch.value;
                if (isObj(rec.patch.attributes)) out.attributes = Object.freeze(Object.assign({}, view.attributes, rec.patch.attributes));
            }
            out.pending = !!(rec && rec.pending);
            if (lock && view.available !== false) { out.readonly = true; out.readonlyReason = lock; }
            if (c && !rec && out.status === 'ok') { out.status = 'error'; out.error = c.entry; }
            if (fl && !rec && !c) out.confirmed = true;
            if (a && a.id === id) out.armed = Object.freeze({ command: a.command, seconds: Math.max(1, Math.ceil((a.until - now()) / 1000)) });
            return Object.freeze(out);
        }
        function onFrame(id, state) { const rec = overlays.get(id); if (!rec || !rec.sent || !isObj(state)) return; rec.frame = state; if (rec.expected && matches(state, rec.expected, rec.step)) confirmRec(rec); }
        const listen = (type, fn) => { stream.on(type, fn); unhook.push(() => stream.off(type, fn)); };
        const ents = d => (d && Array.isArray(d.entities) ? d.entities : []).filter(x => isObj(x) && typeof x.id === 'string');
        listen('state', d => { if (d && isObj(d.changes)) for (const [id, st] of Object.entries(d.changes)) onFrame(id, st); });
        listen('snapshot', d => { for (const x of ents(d)) onFrame(x.id, x.state); });
        listen('entity', d => { for (const x of ents(d)) { if (locks.delete(x.id)) emit(x.id); onFrame(x.id, x.state); } });
        function clearLocks(reason) { for (const [id, r] of [...locks]) if (!reason || r === reason) { locks.delete(id); emit(id); } }
        function dispose() {
            if (disposed) return; disposed = true; for (const fn of unhook.splice(0)) fn();
            for (const r of overlays.values()) stop(r); for (const c of cues.values()) clearT(c.timer); for (const t of flashes.values()) clearT(t);
            for (const a of armed.values()) for (const t of a.timers) clearT(t); for (const l of lanes.values()) clearT(l.timer);
            overlays.clear(); cues.clear(); flashes.clear(); armed.clear(); lanes.clear(); locks.clear();
        }
        return Object.freeze({ send, decorate, clearLocks, on: bus.on, off: bus.off, dispose, get disposed() { return disposed; } });
    }
    const instances = new Map();
    function shared(options) { const key = options && options.api ? options.api.base : '', hit = instances.get(key); if (hit && !hit.disposed) return hit; const made = create(options); instances.set(key, made); return made; }
    E.command = Object.freeze({ create, shared, predict, matches, valueMatches, capOf, TIMING, CONTINUOUS });
})();
```
- [ ] Wiring step W; `npm run test:homedash` → 11 new PASS, none FAIL. Commit — `feat(homedash): command client with overlay, confirmation, throttle and arming (P1)`.

### Task 5: Store read-only reason, stream leading PUT (D8)

**Files:** `hd-store.js`, `hd-stream.js`, `store.test.mjs`, `stream.test.mjs`. **Impact:** `create`/`view` (hd-store; callers render, app — MEDIUM), `refreshWant` (hd-stream; LOW).

- [ ] Tests: store — a `readonly` script with `readonly_reason: 'home_assistant.allowed_services'` → `view.readonlyReason` equals it; desktop read-only → `'virtual_desktop.readonly'`; a system sensor → `readonly:false`, `readonlyReason:''`. Stream — after `hello` with no ids, `handle.set(['system:cpu'])` PUTs at `advance(0)`; a second change PUTs only after 250 ms.
- [ ] Store `view()`:
```js
            const ro = hasCommand(caps) && (m.readonly === true || desktopReadonly);
            const blocked = caps.find(c => isObj(c) && !READ_CAPS.includes(c.name) && typeof c.blocked_by === 'string' && c.blocked_by);
            // in the frozen view: readonly: ro, readonlyReason: !ro ? '' : desktopReadonly ? 'virtual_desktop.readonly' : (typeof m.readonly_reason === 'string' && m.readonly_reason) || (blocked ? blocked.blocked_by : 'homedash.readonly'),
```
  Update header rule 3 and the View line.
- [ ] Stream `refreshWant`: `if (phase === 'live') schedulePut(sentSig === '' ? 0 : DEBOUNCE_MS);` and header: "the first ids after an empty subscription go out at once".
- [ ] Commit — `feat(homedash): read-only reason on entity views and an immediate first subscription`.

### Task 6: Translations

**Files:** `ui/lang/homedash/*.json` (16), `scripts/homedash-tests/i18n.test.mjs` (`SAME_VALUES` adds `'Home Assistant'`).

- [ ] Add these keys (en / de); the maker translates the other 14 locales for real (L5 test enforces):

| key | en | de |
|---|---|---|
| homedash.state.armed | Tap again to confirm · {{seconds}} s | Zum Bestätigen erneut tippen · {{seconds}} s |
| homedash.state.done | Done | Erledigt |
| homedash.arm.announce | Tap again within 4 seconds to confirm. | Tippe innerhalb von 4 Sekunden erneut, um zu bestätigen. |
| homedash.cue.timeout | Not confirmed by the device | Vom Gerät nicht bestätigt |
| homedash.cue.command_failed | Command not accepted | Befehl nicht angenommen |
| homedash.cue.source_unavailable | {{source}} is not reachable | {{source}} ist nicht erreichbar |
| homedash.cue.rate_limited | Too many commands, try again | Zu viele Befehle, versuch es gleich noch mal |
| homedash.cue.network | No connection to the server | Keine Verbindung zum Server |
| homedash.cue.invalid | Value not accepted | Wert nicht angenommen |
| homedash.cue.not_found | Device not found | Gerät nicht gefunden |
| homedash.cue.generic | Command failed ({{code}}) | Befehl fehlgeschlagen ({{code}}) |
| homedash.reason.homedash_readonly / _long | HomeDash is read-only / HomeDash is set to read-only in the settings. | HomeDash ist schreibgeschützt / HomeDash ist in den Einstellungen schreibgeschützt. |
| homedash.reason.desktop_readonly / _long | Desktop is read-only / The desktop is read-only, so devices cannot be switched. | Desktop ist schreibgeschützt / Der Desktop ist schreibgeschützt, Geräte lassen sich nicht schalten. |
| homedash.reason.source_control / _long | Control is switched off / Control of Home Assistant devices is switched off in the HomeDash settings. | Steuerung ist ausgeschaltet / Die Steuerung von Home-Assistant-Geräten ist in den HomeDash-Einstellungen ausgeschaltet. |
| homedash.reason.allowed_services / _long | Service not allowed / This Home Assistant service is not on the allow list. | Dienst nicht erlaubt / Dieser Home-Assistant-Dienst steht nicht auf der Erlaubt-Liste. |
| homedash.reason.blocked_services / _long | Service blocked / This Home Assistant service is on the block list. | Dienst gesperrt / Dieser Home-Assistant-Dienst steht auf der Sperrliste. |
| homedash.reason.ha_readonly / _long | Home Assistant is read-only / The Home Assistant integration is set to read-only. | Home Assistant ist schreibgeschützt / Die Home-Assistant-Anbindung ist schreibgeschützt. |
| homedash.reason.unknown_long | This device cannot be controlled from here. | Dieses Gerät lässt sich hier nicht steuern. |
| homedash.control.power / brightness / color_temp / color / effect | Power / Brightness / Colour temperature / Colour / Effect | Ein/Aus / Helligkeit / Farbtemperatur / Farbe / Effekt |
| homedash.control.hue_sat | Hue {{hue}}°, saturation {{sat}} % | Farbton {{hue}}°, Sättigung {{sat}} % |
| homedash.control.decrease / increase / choose / dimmer | Decrease / Increase / Choose / dimmer | Verringern / Erhöhen / Auswählen / Dimmer |
| homedash.action.activate / run / press | Activate / Run / Press | Aktivieren / Ausführen / Drücken |
| homedash.kind.scene / script / button | Scene / Script / Button | Szene / Skript / Taster |
| homedash.moreinfo.open / close | More information / Close | Weitere Informationen / Schließen |
| homedash.moreinfo.source / updated | Source: {{source}} / updated {{when}} | Quelle: {{source}} / aktualisiert {{when}} |
| homedash.source.ha | Home Assistant | Home Assistant |
| homedash.home.create_ha / create_ha_hint | Home Assistant dashboard / One page per room with every device HomeDash can control. | Home-Assistant-Dashboard / Eine Seite pro Raum mit allen Geräten, die HomeDash steuern kann. |
| homedash.home.ha_name / ha_unassigned | Home Assistant / Unassigned | Zuhause / Ohne Raum |
| homedash.home.notice_template_empty | Home Assistant has no device this dashboard can control. | Home Assistant hat kein Gerät, das dieses Dashboard steuern kann. |
| homedash.home.notice_source_unavailable | {{source}} is not reachable. Try again later. | {{source}} ist nicht erreichbar. Versuch es später noch einmal. |
| homedash.banner.template_omitted | Left out because of the dashboard limits: {{count}} devices. | Wegen der Dashboard-Grenzen weggelassen: {{count}} Geräte. |
| homedash.banner.source_catalog_failed | {{source}} could not load its devices. | {{source}} konnte seine Geräte nicht laden. |
| homedash.menu.new_ha | New Home Assistant dashboard | Neues Home-Assistant-Dashboard |

- [ ] Run `npm run test:homedash` (i18n PASS) and `go test ./ui -run '(?i)^(TestTranslations_|TestFrontend_StaticI18nKeysExistInEnglishBundle)' -count=1` (literal keys appear later; this run checks file structure).
- [ ] Commit — `feat(homedash): M3 texts in 16 locales`.

**Batch A review (reader-agent, Critical/Important only):** command rules 1–8 vs contract §4 and D5 (late adoption from sending, retry new id, never-reject, timer leaks on dispose), snap grid, gesture/press edge cases (synthetic pointers, mouse without buttons), store reason, stream leading PUT.

---

## Batch B — engine wiring

### Task 7: hd-control.js and the widget host

**Files:** create `ui/js/homedash/hd-control.js`, `scripts/homedash-tests/control.test.mjs`; modify `hd-widgets.js`, `hd-prim-displays.js` (`frame`), wiring step W (`hd-control.js` after `hd-widgets.js`). **Impact:** `host`, `writeState`, `update` (hd-widgets: every widget; HIGH by reach — keep additive), `frame` (all hosts and lab; MEDIUM).

- [ ] `control.test.mjs`: (1) `reasonText` returns the short and long texts for each of the six `blocked_by` keys and `virtual_desktop.readonly` (with `makeT` over en.json); (2) `cueText` per code incl. source name "Home Assistant"; (3) `iconFor`: `widget.icon` known → it; else `icon_hint` known → it; else kind icon (`light`→`bulb`, `select`→`list`); unknown icon → next rule; (4) `statusText` precedence: host words (null) > armed > read-only "On · Service not allowed" > stale > word; (5) host: an entity with `error:{text}` and status error shows the text in `.hd-state-text`, `data-hd-armed`/`data-hd-confirmed` follow the view, head icon swaps `power`→`outlet` when the hint arrives, title falls back to `titleFallback()` with no entity; `.hd-more` click calls `moreInfo('primary')` and contains `svg[data-hd-icon="more"]`.
- [ ] `hd-control.js` (key code):
```js
// HomeDash control kit (P1/W1): rules shared by every control widget and the light sheet. Capabilities, ranges and options come from the entity's
// capabilities (attributes may be null; contract M3 section 2). Read-only reasons and command cues are localized here with literal keys. State line
// rule: host words first (unavailable, unknown, error cue, pending), then armed countdown, read-only "word · reason", blocked capability, stale age, word.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.format || !E.widgets || !E.icons) throw new Error('hd-control.js: load hd-widgets.js first');
    const F = E.format, MID = ' ' + String.fromCodePoint(0xb7) + ' ', num = v => (typeof v === 'number' && Number.isFinite(v) ? v : null);
    const cap = (e, name) => (E.command && E.command.capOf ? E.command.capOf(e, name) : null) || capOf(e, name);   // capOf: same lookup as hd-command (string or object entries)
    function range(e, name, d) { const c = cap(e, name) || {}, a = (e && e.attributes) || {}, x = d || {};
        return { min: num(c.min) ?? num(a.min) ?? x.min ?? 0, max: num(c.max) ?? num(a.max) ?? x.max ?? 100, step: num(c.step) ?? num(a.step) ?? x.step ?? 1 }; }
    function ctRange(e) { const c = cap(e, 'color_temp') || {}, a = (e && e.attributes) || {}, min = num(c.min) ?? num(a.min_color_temp_kelvin) ?? 2000, max = num(c.max) ?? num(a.max_color_temp_kelvin) ?? 6500;
        return { min, max: max > min ? max : min + 100, step: num(c.step) ?? 50 }; }
    function options(e, name) { const c = cap(e, name), list = c && Array.isArray(c.options) ? c.options : e && e.attributes && Array.isArray(e.attributes.options) ? e.attributes.options : []; return list.filter(x => typeof x === 'string').slice(0, 64); }
    function lightModel(e) { const dead = !e || e.available === false || e.status === 'unavailable' || e.status === 'unknown', on = !dead && F.isOn(e) === true;
        const raw = e && e.attributes ? Number(e.attributes.brightness_pct) : NaN, known = on && Number.isFinite(raw) && raw > 0;
        return { dead, on, known, pct: dead ? null : known ? Math.min(100, Math.max(1, Math.round(raw))) : on ? 100 : 0, dimmable: !!cap(e, 'brightness') }; }
    function reasonKey(e, name) { const c = name ? cap(e, name) : null; return (c && typeof c.blocked_by === 'string' && c.blocked_by) || (e && e.readonlyReason) || ''; }
    function reasonText(t, key, long) { switch (key) {
        case 'homedash.readonly': return long ? t('homedash.reason.homedash_readonly_long') : t('homedash.reason.homedash_readonly');
        case 'virtual_desktop.readonly': return long ? t('homedash.reason.desktop_readonly_long') : t('homedash.reason.desktop_readonly');
        case 'homedash.sources.home_assistant.control': return long ? t('homedash.reason.source_control_long') : t('homedash.reason.source_control');
        case 'home_assistant.allowed_services': return long ? t('homedash.reason.allowed_services_long') : t('homedash.reason.allowed_services');
        case 'home_assistant.blocked_services': return long ? t('homedash.reason.blocked_services_long') : t('homedash.reason.blocked_services');
        case 'home_assistant.readonly': return long ? t('homedash.reason.ha_readonly_long') : t('homedash.reason.ha_readonly');
        default: return long ? t('homedash.reason.unknown_long') : t('homedash.state.readonly'); } }
    const sourceName = (t, src) => (src === 'ha' ? t('homedash.source.ha') : src === 'system' ? t('homedash.source.system') : String(src || ''));
    function cueText(t, err) { switch (err && err.code) {
        case 'timeout': return t('homedash.cue.timeout'); case 'command_failed': case 'source_error': return t('homedash.cue.command_failed');
        case 'source_unavailable': case 'hub_closed': return t('homedash.cue.source_unavailable', { source: sourceName(t, err.source || 'ha') });
        case 'rate_limited': return t('homedash.cue.rate_limited'); case 'network': case 'aborted': return t('homedash.cue.network');
        case 'invalid_args': case 'unsupported_command': case 'bad_request': return t('homedash.cue.invalid'); case 'entity_not_found': return t('homedash.cue.not_found');
        default: return t('homedash.cue.generic', { code: (err && err.code) || '?' }); } }
    const enabled = (api, e, caps) => !!e && e.available !== false && !e.readonly && [].concat(caps).some(c => api.can('primary', c));
    function announceBlocked(api, e, name) { if (!e) return; api.announce(e.available === false ? api.t('homedash.state.unavailable') : reasonText(api.t, reasonKey(e, name), true)); }
    function onOffCommand(api, e) { const on = F.isOn(e); if (on === true && api.can('primary', 'turn_off')) return 'turn_off'; if (on === false && api.can('primary', 'turn_on')) return 'turn_on'; return api.can('primary', 'toggle') ? 'toggle' : null; }
    function activateOnOff(api, e) { if (!e || e.available === false) return false; if (e.readonly) { announceBlocked(api, e, 'toggle'); return false; } const c = onOffCommand(api, e); if (!c) return false; api.command('primary', c, {}); return true; }
    function actions(widget) { const a = (widget && widget.actions) || {}, tap = a.tap && a.tap.do, hold = a.hold && a.hold.do;
        return { tap: ['default', 'toggle', 'more_info', 'none'].includes(tap) ? tap : 'default', hold: hold === 'none' ? 'none' : 'more_info' }; }   // navigate is not an M3 action
    function tap(api, e, primary) { const a = actions(api.widget); if (a.tap === 'none') return; if (a.tap === 'more_info') { api.moreInfo('primary'); return; } primary(); }
    function hold(api) { if (actions(api.widget).hold === 'more_info') api.moreInfo('primary'); }
    const label = (api, e) => api.widget.title || (e && e.name) || '';
    function statusText(api, e, word, o) {
        if (!e || e.available === false || ['unavailable', 'unknown', 'error'].includes(e.status) || e.pending) return null;
        if (e.armed) return api.t('homedash.state.armed', { seconds: e.armed.seconds });
        const c = o && o.cap ? cap(e, o.cap) : null;
        if (e.readonly) return word + MID + reasonText(api.t, reasonKey(e, o && o.cap), false);
        if (c && c.allowed === false) return word + MID + reasonText(api.t, reasonKey(e, o.cap), false);
        if (e.stale) { const old = E.widgets.heroStatus.staleText(api, e); if (old) return old; }
        if (e.confirmed && o && o.done) return api.t('homedash.state.done');
        return word;
    }
    const armMarker = () => E.el('span', { class: 'hd-arm', 'aria-hidden': 'true' });
    const decimalsOf = step => { const s = String(step), i = s.indexOf('.'); return i < 0 ? 0 : Math.min(3, s.length - i - 1); };
    E.control = Object.freeze({ cap, range, ctRange, options, lightModel, reasonKey, reasonText, sourceName, cueText, enabled, announceBlocked, onOffCommand, activateOnOff, actions, tap, hold, label, statusText, armMarker, decimalsOf, MID });
})();
```
  (Define a local `capOf` with the same rule as `hd-command` because `hd-control` loads before `hd-command`; drop the `E.command` indirection.)
- [ ] `hd-widgets.js` host (additive):
  - `KIND_ICONS = {light:'bulb', switch:'power', scene:'scene', script:'script', button:'button', select:'list', number:'number', sensor:'gauge', climate:'thermometer', lock:'lock'}`; `iconFor(widget, e)` (widget.icon if `E.icons.has` → `e.icon_hint` if known → `KIND_ICONS[e.kind] || 'entity'`; no entity → `widget.icon` or null); export `E.widgets.iconFor`.
  - `writeState(p)`: cue text when `F.stateWord(p) === 'error' && p.error && p.error.text`; title = `widget.title || p.name || (o.titleFallback ? o.titleFallback() : '')`; `frame.set({…, icon: iconFor(widget, p)})`.
  - `update()`: also writes `data-hd-armed` (`!!p.armed`) and `data-hd-confirmed` (`!!p.confirmed`).
  - When `o.moreLabel`: `resources.on(frame.el.querySelector('.hd-more'), 'click', () => { if (mode === 'view') fns.moreInfo(def.bindings[0]); })`.
- [ ] `frame()`: `if (o.more) node.appendChild(el('button', { class: 'hd-more', type: 'button', 'aria-label': o.more }, E.icons.icon('more')));` (SCR-M3-2).
- [ ] Wiring step W; `npm run test:homedash` PASS. Commit — `feat(homedash): control kit, head icons, cue text and more button in the widget host`.

### Task 8: Primitive input contract (SCR-M3-4)

**Files:** `hd-prim-controls.js`, `controls.test.mjs`, `contract.test.mjs` (`PRIMS` + `picker`). **Impact:** `toggle`, `rocker`, `pushButton`, `slider`, `knob`, `stepper`, `segmented`, `colourWheel`, `ctStrip` (callers: hero modules, lab, tests; MEDIUM — no change without `input`).

- [ ] Tests: (a) without `input` the markup of every builder equals today's (no `data-hd-input`, wheel `role=img`, pushButton no role); (b) `slider({input})`: ArrowRight → `onKey(v+1)` and paints `aria-valuenow`; a set() during a drag keeps the drag value (simulate with `pointerdown`/`pointermove` on the element and a stubbed track rect); (c) toggle click → `onActivate`; disabled → `onBlocked`; (d) stepper `+` click → `onStep(value+step)` snapped; (e) segmented ArrowRight checks the next radio and calls `onPick(id, true)`; (f) `picker` sets `aria-expanded` and text; (g) `pushButton({toggle:true})` has `role=switch` and `aria-checked` follows `on`; `set({icon})` swaps the glyph.
- [ ] Implementation outline (all bindings go through `E.input`, required lazily: `const inputApi = () => { if (!E.input) throw new Error('hd-prim-controls.js: load hd-input.js for interactive controls'); return E.input; };`; every builder with `input` returns a `destroy` that unbinds):
  - toggle/rocker: `setAttr(node, 'data-hd-input', 'press'); off = inputApi().press(node, input)`.
  - pushButton: `role: o.toggle ? 'switch' : null`, `aria-checked`; `applyState(..., { interactive: true, readonlyAria: o.toggle ? undefined : false })`; `set({icon})` replaces the icon node before the label when the name changes; `input` → press.
  - slider and ctStrip use `bindLinear(node, measure, axis, input, show, current)`:
```js
    function bindLinear(node, measure, axis, input, show, current) {
        const I = inputApi(), r = () => input.range(), at = p => I.valueAt(I.fraction(p.x, p.y, measure(), axis), r());
        setAttr(node, 'data-hd-input', 'drag');
        const offG = I.gesture(node, { enabled: input.enabled, onBlocked: input.onBlocked, onHold: input.onHold,
            onTap: p => { const v = at(p); show(v, false); input.onCommit(v); },
            onDrag: p => { const v = at(p); show(v, true); input.onInput(v); },
            onDragEnd: p => { const v = at(p); show(v, false); input.onCommit(v); } });
        const offK = I.keys(node, { enabled: input.enabled, onBlocked: input.onBlocked, onHold: input.onHold, onToggle: input.onToggle, range: r, get: current, onKey: v => { show(v, false); input.onKey(v); } });
        return () => { offG(); offK(); };
    }
```
    `show(v, lock)` paints `v` through `writeValue` (valueText null so the unit text is computed) and sets `locked = lock`; `set(state)` paints `locked ? held : state.value` (input lock during a drag). The slider measures `.hd-slider-track`; the CT strip measures itself and accepts `set({min, max})`.
  - knob: `bindDial` — `gesture` with `rect: () => node.getBoundingClientRect()`; tap in the centre (distance < 0.31 × width) → `input.onToggle()`; tap elsewhere → `onCommit(valueAt(dialFraction))`; drag starting in the centre → vertical mode (`verticalFraction(startFrac, dy)`), else angular (`dialFraction(…, last)`); `onDrag` → `onInput`, `onDragEnd` → `onCommit(last value)`; `keys` with `onToggle` for Enter/Space.
  - stepper: `inputApi().repeat(dec|inc, { onStep: () => step(∓1), enabled, onBlocked })` where `step(dir)` snaps `current ± step` within `input.range()`, paints the numeral and `spin` at once, then `input.onStep(v)`; `keys(spin, {range, get, onKey: v => { paint(v); input.onStep(v); }})`.
  - segmented: each option `press(b, { onActivate: () => { render(i); input.onPick(list[i].id, false); } })`; `keydown` on the group moves focus and selection with Arrow/Home/End (gated by `enabled`), calls `onPick(id, true)`.
  - colourWheel with `input`: `role=slider`, `tabindex=0`, `data-hd-input=drag`, `aria-valuemin=0`, `aria-valuemax=360`; pointer position → `{h, s}` (h = atan2 from 12 o'clock clockwise, s = radius 0–100) for `onTap`→`onCommit`, `onDrag`→`onInput`, `onDragEnd`→`onCommit`; keys per the ARIA table → `onKey({h,s})`.
  - `picker(opts)`: `button.hd-pick[type=button][aria-haspopup=listbox][aria-expanded=false][aria-label]` > `span.hd-pick-label` + `chevron-down` icon; `set({text, expanded, label, …state})`; `input` → press.
- [ ] Commit — `feat(homedash): optional input contract on the control primitives and the picker`.

### Task 9: Sheet trap, popover, light sheet

**Files:** `hd-sheet.js` (add `trap`, refactor `create` onto it), create `hd-popover.js`, `hd-moreinfo.js`, `scripts/homedash-tests/overlay.test.mjs`; wiring step W (`hd-popover.js` after `hd-control.js`, `hd-moreinfo.js` last in `ENGINE_ORDER`; `PARTS` entries). **Impact:** `create`/`open` in hd-sheet (host delete confirm; sheet.test must stay green; LOW).

- [ ] `overlay.test.mjs`: (1) `trap` inerts the container's other children except `[aria-live]`, cycles Tab, Escape calls `onEscape`, release removes inert and focuses the opener; `E.sheet.create` still passes `sheet.test.mjs`; (2) `E.popover.listbox` with 3 options and value `b`: listbox has `aria-activedescendant` on `b`, ArrowDown→`c`, Enter → `onPick('c')`, panel removed, anchor focused; Escape closes without pick; a second listbox on the same root closes the first; panel carries `hd-widget` and the family; root narrower than 600 px → `data-hd-pop="sheet"` (stub `getBoundingClientRect`); (3) `E.moreinfo.build` for the kitchen light shows rows brightness, CT, colour and effect; for `light.sofa` (on/off only) only the power row; read-only → controls `aria-readonly`, foot contains the long reason; unavailable → `aria-disabled`; (4) `E.moreinfo.create().open(req)` appends `.hd-sheet-layer[data-kind=moreinfo]` to the root, focuses the power toggle, Escape closes and refocuses the opener, `req.subscribe` callback re-renders.
- [ ] `hd-sheet.js`: add `trap({container, layer, dialog, opener, onEscape, onBackdrop})` → `release()` (inert all `container.children` except `layer` and `[aria-live]`; layer keydown Escape/Tab; layer click on itself → onBackdrop; release restores inert and focus). `create()` keeps its API and uses `trap`. Export `E.sheet = { create, trap }`.
- [ ] `hd-popover.js` (key code):
```js
// HomeDash popover sheet (visual system 3; D5 list select). listbox(o) opens one non-modal listbox for a trigger: anchored below (above when no room)
// when the root is at least 600 px wide, a bottom sheet otherwise; one popover per root. APG listbox keys; pointer down outside closes; focus returns
// to the trigger. The panel is .hd-sheet.hd-widget[data-kind=listbox] with the trigger's family (SCR-M3-3). Geometry is read once at open.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.icons) throw new Error('hd-popover.js: load hd-core.js and hd-icons.js first');
    const { el, setAttr, setVars, uid } = E.core, OPEN = new WeakMap(), ROW = 44, PAD = 8, MAX_H = 352, NARROW = 600;
    function listbox(o) {
        const root = o.root, prev = OPEN.get(root); if (prev) prev.close(false);
        const id = uid('hd-lb'), life = E.core.bag(), doc = root.ownerDocument;
        const layer = Array.from(root.children).find(n => n.classList && n.classList.contains('hd-pop-layer')) || root.appendChild(el('div', { class: 'hd-pop-layer' }));
        const list = el('ul', { class: 'hd-listbox', role: 'listbox', id, tabindex: '0', 'aria-label': o.label || '' });
        const items = o.options.map((x, i) => el('li', { class: 'hd-option', role: 'option', id: id + '-' + i, 'aria-selected': x.id === o.value ? 'true' : 'false', 'data-hd-option': x.id }, [E.icons.icon('check'), el('span', { class: 'hd-option-label', text: x.label })]));
        list.append(...items);
        const panel = el('div', { class: 'hd-sheet hd-widget', 'data-kind': 'listbox', 'data-hd-family': o.family || 'glass' }, list); layer.appendChild(panel);
        const rr = root.getBoundingClientRect(), ar = o.anchor.getBoundingClientRect(), narrow = rr.width < NARROW, h = Math.min(MAX_H, items.length * ROW + 2 * PAD), w = Math.min(rr.width - 16, Math.max(ar.width, 200));
        let y = ar.bottom - rr.top + 4; if (y + h > rr.height && ar.top - rr.top - 4 - h >= 0) y = ar.top - rr.top - 4 - h;
        const x = Math.max(8, Math.min(ar.left - rr.left, rr.width - w - 8));
        setAttr(panel, 'data-hd-pop', narrow ? 'sheet' : 'anchored');
        setVars(panel, { '--hd-pop-x': Math.round(x) + 'px', '--hd-pop-y': Math.round(y) + 'px', '--hd-pop-w': Math.round(narrow ? rr.width : w) + 'px', '--hd-pop-h': h + 'px' });
        let active = Math.max(0, o.options.findIndex(v => v.id === o.value)), done = false, typed = '', typedAt = 0;
        const activate = i => { active = Math.max(0, Math.min(items.length - 1, i)); setAttr(list, 'aria-activedescendant', items[active] ? items[active].id : null); items.forEach((n, k) => setAttr(n, 'data-hd-active', k === active)); };
        const api = { el: panel, close };
        function close(refocus) { if (done) return; done = true; life.dispose(); panel.remove(); if (OPEN.get(root) === api) OPEN.delete(root); if (refocus !== false && o.anchor.focus) o.anchor.focus(); if (o.onClose) o.onClose(); }
        function pick(i) { const v = o.options[i]; if (!v) return; close(true); o.onPick(v.id); }
        life.on(list, 'keydown', ev => {
            const k = ev.key, nav = { ArrowDown: active + 1, ArrowUp: active - 1, Home: 0, End: items.length - 1 };
            if (k in nav) { ev.preventDefault(); activate(nav[k]); return; }
            if (k === 'Enter' || k === ' ') { ev.preventDefault(); pick(active); return; }
            if (k === 'Escape') { ev.preventDefault(); ev.stopPropagation(); close(true); return; }
            if (k === 'Tab') { close(false); return; }
            if (k.length === 1) { const t = Date.now(); typed = (t - typedAt < 700 ? typed : '') + k.toLowerCase(); typedAt = t; const i = o.options.findIndex(v => v.label.toLowerCase().startsWith(typed)); if (i >= 0) activate(i); }
        });
        life.on(list, 'click', ev => { const n = ev.target && ev.target.closest ? ev.target.closest('.hd-option') : null; if (n) pick(items.indexOf(n)); });
        life.on(doc, 'pointerdown', ev => { if (!panel.contains(ev.target) && !o.anchor.contains(ev.target)) close(false); }, { capture: true });
        OPEN.set(root, api); activate(active); list.focus(); return api;
    }
    E.popover = Object.freeze({ listbox, close(root) { const p = OPEN.get(root); if (p) p.close(false); } });
})();
```
- [ ] `hd-moreinfo.js` (shape; complete it):
  - Header: "HomeDash more-info for lights (W1; D1: lights only, no presets, no history). build(o) renders the dialog (static; the lab shows it inline); create(o) runs it modal inside .hd-root through E.sheet.trap. Rows follow the capabilities: power; brightness (`brightness`); colour temperature (`color_temp`, grid anchored at min); colour wheel (`color`); effect (`effect` with options). Foot: source, last update, read-only reason."
  - `build({family, api, popoverRoot})` where `api` has the widget-api shape (`widget`, `t`, `lang`, `entity('primary')`, `can('primary', cap)`, `command('primary', cap, args, {phase})`, `announce`, `moreInfo`). Dialog = `div.hd-sheet.hd-widget[role=dialog][aria-modal=true][aria-labelledby=frame title][data-kind=moreinfo][data-hd-family][data-hd-type=light]` containing `E.prim.frame({title:'', icon:'bulb'})`; body `div.hd-mi` rows `[data-row=power|bright|ct|wheel|effect]` each with `span.hd-mi-label` (icon + literal-key text) and `div.hd-mi-line`; close button `button.hd-sheet-close` (`close` glyph, `homedash.moreinfo.close`) last in DOM.
  - Controls: `E.prim.toggle({input:{onActivate: () => C.activateOnOff(api, cur), enabled, onBlocked}})`; `E.prim.slider({size:'l', min:0, max:100, unit:'%', input:{range:()=>({min:0,max:100,step:1}), onInput/onCommit/onKey → api.command('primary','brightness',{value},{phase})}})`; `E.prim.ctStrip({input:{range: () => C.ctRange(cur), … 'color_temp' {kelvin}}})`; `E.prim.colourWheel({diameter:220, input:{… 'color' {h,s}}})`; `E.prim.picker({input:{onActivate: openEffects}})` with `E.popover.listbox({root: o.popoverRoot || dialog.parentNode, anchor: picker.el, family, options: C.options(cur,'effect').map(x=>({id:x,label:x})), value: attrs.effect, onPick: v => api.command('primary','effect',{option:v})})`.
  - `update()`: `cur = api.entity('primary')`, `m = C.lightModel(cur)`; frame title/sub (name, area), icon via `E.widgets.iconFor`; state text from host-like rule (unavailable/pending/error cue words, else `C.statusText`); rows hidden by capability; brightness `value: m.dead ? null : m.on ? m.pct : 0`, valueText as the light widget; CT `{value: color_mode==='color_temp' ? color_temp_kelvin : null, min, max}`; wheel `{hue, sat: s/100, valueText: t('homedash.control.hue_sat', …)}` from `hs_color`; effect text; dialog attributes `data-hd-status/on/pending/readonly`; foot `t('homedash.moreinfo.source', {source: C.sourceName(t, cur.source)})` + `t('homedash.moreinfo.updated', {when: F.relTime(...)})` + long reason when read-only.
  - `focusFirst()` focuses the power toggle; `destroy()` unbinds all primitives.
  - `create({root})` → `{open(req), close(), isOpen(), destroy()}`: `req = {api, family, opener, subscribe(fn) → unsub}`; builds a `div.hd-sheet-layer[data-kind=moreinfo][data-hd-pop=center|sheet]` (sheet when root width < 600), appends to root, `E.sheet.trap({container: root, layer, dialog, opener, onEscape: close, onBackdrop: close})`, subscribes, `focusFirst()`; close releases trap, closes a popover on the layer, removes the layer.
- [ ] Wiring step W; tests PASS. Commit — `feat(homedash): modal trap, listbox popover and the light more-info sheet`.

### Task 10: Render, view and app integration

**Files:** `hd-render.js`, `hd-view.js`, `hd-app.js`, tests `render.test.mjs`, `view.test.mjs`, `app.test.mjs`. **Impact:** `page`/`api` (hd-render), `create`/`banners` (hd-view), `realmFor`, `mount`, `boot`, `home`, `open`, `create` (hd-app) — HIGH by reach (every window): keep the M2 behaviour when `commands` is absent.

- [ ] Tests first:
  - render: with a fake `commands` (`decorate` passthrough + overlay, `send` recorder, emitter): `can` false without commands (M2 test stays), true for an allowed capability of an available, writable entity; `command` sends `{entity, command, args, phase, widgetKey:'w|<doc>|<wid>', widgetId, dashboardId, confirm}`; a cue `{code:'timeout'}` reaches the state line as `homedash.cue.timeout`; `commands` change re-renders bound widgets in one frame; a widget whose entity has no view shows `homedash.system.cpu` as title (titles before the first snapshot); `moreInfo` calls `opts.moreInfo` only for `kind: 'light'`.
  - view: `banners` lists `source_degraded`/`source_unavailable`/`source_catalog_failed` only for `usedSources`; `template_omitted` with `count`.
  - app: boot issues `bootstrap`, `widget-catalog` and (with `dashboardId`) `dashboard` before any answer; update "dispose during boot" to `['bootstrap','widget-catalog']`; `createHomeAssistant()` posts `{template:'home_assistant', name:'homedash.home.ha_name', unassigned_title:'homedash.home.ha_unassigned'}` and sets `notice:'template_omitted'`, `noticeCount` from `omitted`; `422 template_empty` → home notice `template_empty`; `503 source_unavailable` → `source_unavailable` with `noticeSource:'ha'`; `home()` re-reads bootstrap and `state.templates` follows it; a command `desktop_readonly` sets `desktopReadonly`.
- [ ] hd-render: new opts `commands`, `moreInfo(req)`. Inside `page()`:
```js
        const commands = o.commands || null, docId = doc.id || '', sheetSubs = new Map();
        function decorated(rec, id, key) {
            const v = store.view(id); if (!v) return null;
            const base = Object.assign({}, v, { tone: E.tone.of(v, rec.widget.options) }), sys = v.source === 'system' ? systemName(id) : null; if (sys) base.name = sys;
            const out = commands ? commands.decorate(id, Object.freeze(base), key) : base;
            return out && out.error && !out.error.text ? Object.freeze(Object.assign({}, out, { error: Object.freeze(Object.assign({}, out.error, { text: E.control.cueText(t, out.error) })) })) : out;
        }
        function canFor(id, capName, mode) {
            if (!commands || mode !== 'view' || !id) return false;
            const v = store.view(id), d = v && commands.decorate(id, v, null);
            if (!d || d.available === false || d.status === 'unknown' || d.status === 'unavailable' || d.readonly) return false;
            const c = E.control.cap(d, capName); return !!c && c.allowed !== false;
        }
        function apiFor(rec, key, mode, onlyId) {   // the widget api, also used for the sheet (key 'sheet|…')
            const idOf = b => { if (onlyId) return onlyId; const id = rec.widget.bindings && rec.widget.bindings[b]; return isEntityId(id) ? id : null; };
            return {
                widget: rec.widget, t, lang,
                entity: b => { const id = idOf(b); return id ? decorated(rec, id, key) : null; },
                can: (b, c) => canFor(idOf(b), c, mode),
                command(b, c, args, opts) { const id = idOf(b); if (!commands) return viewOnly(); if (!canFor(id, c, mode)) return Promise.resolve({ status: 'refused' });
                    return commands.send({ entity: id, command: c, args: args || {}, phase: opts && opts.phase, widgetKey: key, widgetId: rec.widget.id, dashboardId: docId, confirm: rec.widget.confirm === true }); },
                moreInfo(b) { const id = idOf(b), v = id ? store.view(id) : null; if (!v || v.kind !== 'light' || typeof o.moreInfo !== 'function') return false;
                    const owner = o.stage.ownerDocument; o.moreInfo({ family: rec.inst ? rec.inst.family : family, opener: owner.activeElement, api: apiFor(rec, 'sheet|' + docId + '|' + rec.widget.id, 'view', id), subscribe: fn => subscribe(id, fn) }); return true; },
                history: …unchanged…, announce: text => { if (typeof o.announce === 'function') o.announce(String(text)); }
            };
        }
        function api(rec) { const key = 'w|' + docId + '|' + rec.widget.id; return ({ mode }) => apiFor(rec, key, mode, null); }
```
  Host creation adds `moreLabel: widget.type === 'light' ? t('homedash.moreinfo.open') : null` and `titleFallback: () => (rec.ids[0] && systemName(rec.ids[0])) || ''`. Command changes: `commands.on('change', ids => { for (const id of ids) pendingIds.add(id); flushCmd(); })` with `flushCmd = E.core.frame(() => { const ids = [...pendingIds]; pendingIds.clear(); onChange(ids); }, {raf, caf})`; `onChange` also calls `sheetSubs` listeners. Announce new cues and armings of bound ids (`commands.on('cue')` → `cueText`; `'armed'` → `t('homedash.arm.announce')`). `destroy()` removes the listeners and `E.popover.close` on the nearest `.hd-root`. M2 view-only path unchanged when `commands` is null.
- [ ] hd-view: `create(opts)` gains `commands`; builds `moreinfo = E.moreinfo.create({ root })` and passes `moreInfo: req => moreinfo.open(req)` and `commands` to `E.render.page`; adds `sources()` (unique id prefixes of `entityIds()`); `destroy` closes moreinfo and popovers. `banners(s)`: source loop skips sources not in `s.usedSources` (when given); `unavailable` with `reason === 'catalog_failed'` → kind `source_catalog_failed`; after the readonly banner push `{id:'omitted', kind:'template_omitted', tone:'info', role:'status', count}` when `s.notice === 'template_omitted' && s.noticeCount > 0`.
- [ ] hd-app:
  - realm: `realm.commands = E.command.shared(Object.assign({ api, stream, store }, clock));`
  - mount: `life.add(realm.commands.on('readonly', ev => { if (ev.desktop) applyReadonly(true); }))`; `setReadonly(false)` also calls `realm.commands.clearLocks('virtual_desktop.readonly')`; view gets `commands: realm.commands`.
  - `set()` passes `usedSources: view ? view.sources() : []`, `notice`, `noticeCount` to `E.view.banners`.
  - parallel boot:
```js
        async function boot() {
            set({ mode: 'loading', error: null, notice: null });
            const id = o.dashboardId || null, docP = id ? api.dashboard(id).then(a => ({ a }), e => ({ e })) : null;
            try {
                const [b, first] = await Promise.all([api.bootstrap(), store.widgetCatalog()]);
                if (disposed) return;
                store.setSources(b.sources); realm.stream.resume();
                const ro = hostRo || b.desktop_readonly === true; store.setDesktopReadonly(ro);
                const cat = first && first.hash === b.catalog_hash ? first : await store.widgetCatalog(b.catalog_hash);
                if (disposed) return;
                catalog = E.catalog.create(cat); set({ desktopReadonly: ro, templates: templatesOf(b) });
                if (id) await open(id, o.pageId, docP); else await home(null);
            } catch (err) { fail(err, e => set({ mode: 'error', error: Object.freeze({ code: e.code, status: e.status }) })); }
        }
```
    `open(id, pageId, pre)` awaits `pre` (`{a}` or throws `pre.e`) instead of a new request when given. `templatesOf = b => Array.isArray(b && b.templates) ? b.templates.filter(x => typeof x === 'string') : []`.
  - `refreshTemplates()` = `api.bootstrap().then(b => { if (!disposed) { store.setSources(b.sources); set({ templates: templatesOf(b) }); } }, () => {})`; called at the end of `home()`, on visibility return while in home mode, and from `hook('sources', …)` when the `ha` status changes to `live` while in home mode.
  - `create(kind)` adds `'home_assistant'`: `api.createFromTemplate('home_assistant', { name: t('homedash.home.ha_name'), unassigned_title: t('homedash.home.ha_unassigned') })`; after `show()`: `if (Number(answer.omitted) > 0) set({ notice: 'template_omitted', noticeCount: Number(answer.omitted) })`; failure notices add `template_empty` and `source_unavailable` (+`noticeSource: e.data.source || 'ha'`). Public `createHomeAssistant: () => create('home_assistant')`.
- [ ] `npm run test:homedash` PASS. Commit — `feat(homedash): commands, more-info, parallel boot and the Home Assistant template in the engine`.

**Batch B review (reader-agent):** host additivity (M1/M2 tests green), primitive markup unchanged without input, trap/popover a11y and focus return, render key format and never-reject path, parallel boot races (`openSeq`, dispose), banner scoping.

---

## Batch C — W1 widgets

Shared widget pattern (each module): keep `cur` (last view) for input callbacks; `input = { enabled: () => C.enabled(api, cur, caps), onBlocked: () => C.announceBlocked(api, cur, cap), …, onHold: () => C.hold(api) }`; `update(view)` writes only primitive `set()`, `aria-label` (`C.label(api, e)` + control name), `api.setTone`, `api.setStatus(C.statusText(…))`; append `C.armMarker()` to the body; `disabled` only when not read-only and the capability is missing (M1 rule). Tests in `scripts/homedash-tests/w1.test.mjs` host each variant with a fake api (records `command` calls, `can` from capabilities, `moreInfo` recorder) on views built from `ui/testdata/homedash-m3/ha-entities.json` (created in Task 23 — create the file in Task 11 already and reference it; Task 23 only adds the dashboard file). The file stays in wire shape (contract §2: `state:{value,text,attributes,available,stale,updated_at}`), because the Go fixture serves it; widgets read flattened store views. So `w1.test.mjs` feeds the wire objects through a real `E.store` (load `hd-store.js`, apply a `snapshot` frame) and reads `store.view(id)`; only if that needs too much setup, add a `toView(wire)` helper that mirrors `hd-store.js` `view()` field by field (read it; do not guess the field list).

### Task 11: Switch (pill, rocker with input; push; tile)

**Files:** `widgets/hd-w-switch.js`, `w1.test.mjs` (create), `ui/testdata/homedash-m3/ha-entities.json` (create: the 11 entity objects of contract §2 verbatim, order light.kitchen, light.sofa, switch.coffee, input_boolean.vacation_mode, scene.movie, script.goodnight, button.doorbell_test, select.heater_mode, input_select.radio, number.boiler_target, input_number.volume). **Impact:** `hero`, `statusText`, registration (`switch` hosts; M1 heroes — re-judged at gate P).

- [ ] Tests: pill click on `switch.coffee` (off) sends `turn_on` (not toggle); read-only entity sends nothing and announces the long reason; push variant is `button[role=switch]` with the On/Off word; tile fills the body; hold on a switch entity does not call `moreInfo`… but a pill bound to `light.sofa` with hold calls `moreInfo('primary')`; `aria-label` = entity name.
- [ ] Implement `make(api, build)` for all four variants (`pill`: `toggle({decor:true, input})`, `rocker`: `rocker({input})`, `push`: `pushButton({variant:'chip', icon from iconFor, toggle:true, input})`, `tile`: `pushButton({variant:'tile', toggle:true, input})` with label hidden); `onActivate: () => C.tap(api, cur, () => C.activateOnOff(api, cur))`; update as M1 plus `aria-label`, `text` (push: On/Off word; tile: ''), `icon` (`E.widgets.iconFor(api.widget, e) || 'power'`), state line `C.statusText(api, e, onOffWord, { cap: 'toggle' })`.
- [ ] Commit — `feat(homedash): switch controls with input; push and tile variants (W1)`.

### Task 12: Light (dial on brightness_pct with input; slider; fader; tile)

**Files:** `widgets/hd-w-light.js`, `w1.test.mjs`. **Impact:** `dial`, `statusText`, registration (H3 hero).

- [ ] Tests: dial reads `brightness_pct: 40` → `aria-valuenow 40`, "On · 40 %"; ArrowRight on the dial sends `brightness {value:41}` with `phase:'key'`; Enter toggles (`turn_off`); slider tap on the track sends `phase:'commit'`; slider on `light.sofa` (no brightness) is disabled while the power chip still sends `turn_on`; tile on `light.sofa` is `role=switch`, on `light.kitchen` `role=slider` with `aria-roledescription` = `homedash.control.dimmer`; tile drag sends `input` then `commit`, tile tap sends `turn_off`; hold (or Shift+Enter) calls `moreInfo`; unavailable → readout en dash, never "Off".
- [ ] Implement with `C.lightModel`. `controller(api)` builds `dimInput` (range 0–100 step 1; `onInput/onCommit/onKey` → `api.command('primary','brightness',{value:v},{phase})`; `onToggle` → `C.activateOnOff`; `onHold`) and `powerInput` (press → `C.tap(api, cur, () => C.activateOnOff(api, cur))`). Dial = M1 knob options + `input: dimInput`. Slider = `.hd-lrow` (power `pushButton({variant:'chip', icon:'bulb', toggle:true})`, `slider({variant:'h', min:0, max:100, unit:'%', input: dimInput})`, numeral). Fader = `.hd-lcol` (numeral, `slider({variant:'v'})`, power chip). Tile = `div.hd-ltile[tabindex=0]` with `span.hd-ltile-wash`, `iconBadge`, numeral; binds `E.input.gesture` (tap → power, drag → horizontal fraction across the tile → dim input/commit, hold) and `E.input.keys` (dimmable: range keys + Enter/Space toggle; else Enter/Space toggle; both with `onHold`, because `gesture()` binds no keyboard); role/ARIA set in `update` (switch vs slider), `--hd-value` = `pct/100` (0 when off) through `setVars`. Remove all `/ 2.55` code (D6).
- [ ] Commit — `feat(homedash): light controls on brightness_pct with drag, keys and more-info (W1)`.

### Task 13: Action button

**Files:** create `widgets/hd-w-action.js`; `w1.test.mjs`; wiring step W. 
- [ ] Tests: scene click sends `activate`; button sends `press`; `script.goodnight` default is read-only (no command, reason "Service not allowed"); with `allowed:true, sensitive:true` the fake `command` resolving `{status:'armed'}` shows the armed text when the view carries `armed:{seconds:3}`; `confirmed:true` shows the check (state "Done").
- [ ] Implement: `pushButton({variant:'chip', icon:'scene', toggle:false, input})`; `cmdOf(e)` = `activate` or `press`; text = verb (`homedash.action.activate|run|press` by kind); icon by kind (`scene|script|button`); state line `C.statusText(api, e, kindWord, { cap: cmdOf(e), done: true })` with kind words `homedash.kind.*`.
- [ ] Commit — `feat(homedash): action button for scenes, scripts and buttons (W1)`.

### Task 14: Select (segmented, list)

**Files:** create `widgets/hd-w-select.js`; `w1.test.mjs`; wiring step W.
- [ ] Tests: segmented builds options from the capability (`eco, comfort, boost`), click `boost` sends `select_option {option:'boost'}` (`phase` discrete), ArrowRight sends with `phase:'key'`; options change rebuilds once; > 5 options renders the picker; list: picker shows "Jazz", activation opens a listbox under the nearest `.hd-root` (test wraps the widget in a root with a stubbed rect), Enter on "Rock" sends `select_option`, `aria-expanded` returns to false; picking the current value sends nothing.
- [ ] Implement `segmented(api)` (rebuild on options signature, like the climate range rule) and `list(api, host)` (picker + `E.popover.listbox({root: api.body.closest('.hd-root'), anchor, family: api.family, label, options, value, onPick, onClose})`). State line word = current option text (`textContent` only).
- [ ] Commit — `feat(homedash): select segmented and list with the listbox popover (W1)`.

### Task 15: Number (stepper, slider)

**Files:** create `widgets/hd-w-number.js`; `w1.test.mjs`; wiring step W.
- [ ] Tests: stepper on boiler (55, 30–70 step 1): `+` sends `set_value {value:56}` with `phase:'key'`; at 70 the `+` button is `aria-disabled`; slider on volume (step 5) End → `set_value 100` `phase:'key'`; drag → `input`/`commit`; valuetext "55 °C".
- [ ] Implement: range from `C.range(e, 'set_value', {min:0,max:100,step:1})`, decimals `C.decimalsOf(step)`, unit `e.unit`; rebuild the primitive when the range signature changes; slider row `.hd-nrow` with numeral.
- [ ] Commit — `feat(homedash): number stepper and slider (W1)`.

**Batch C review (reader-agent):** catalog registry match (`catalog.test.mjs`), every variant reads ranges/options from capabilities, no layout reads in `update`, aria names and valuetexts, M1 hero markup unchanged except SCRs, explicit on/off verbs.

---

## Batch D — structure, lab, paint (gates W)

### Task 16: W1 structure CSS and the check mask token (SCR-M3-5/6/7; Q3)

**Files:** `ui/css/homedash-primitives.css`, `homedash-widgets.css`, `homedash-page.css`, `homedash-tokens.css` (`--hd-mask-check` in the `.hd-root` token block, same data-URL form as `--hd-mask-error`, path `M5 12.5l4.5 4.5L19 7`), `ui/testdata/homedash-lab/token-freeze.json` (regenerate), bundle.
- [ ] Write the rules of the build sheets. Required content (exact numbers above):
  - primitives: slider (`touch-action: pan-y`, `v` → `pan-x`, 44 hit, track `container-type: inline-size` (`size` for v), fill `scaleX/scaleY(var(--hd-value))`, thumb `transform: translateX(calc(var(--hd-value) * 100cqi))` / `translateY(calc(var(--hd-value) * -100cqb))`, thumb size vars `--hd-thumb-w/-h`), CT strip (track gradient `linear-gradient(90deg, var(--hd-ct-2200), var(--hd-ct-4000), var(--hd-ct-6500))`, `container-type: inline-size`, thumb by cqi), wheel (`touch-action: none`, thumb by `--hd-wheel-x/y`), stepper grid `44px minmax(4ch,auto) 44px`, picker, `.hd-arm` (absolute bar at the body bottom, hidden unless `[data-hd-armed="true"]`), keyframes `hd-arm-run` (scaleX 1→0), `hd-shake` (translateX), `hd-fade-out` (opacity), focus ring extension to `[role="listbox"]:focus-visible [role="option"][data-hd-active="true"]` and `[role="option"][data-hd-focus="ring"]`.
  - widgets: 1-row frame for W1 types (`@container hd-widget (max-height: 139px) and (min-width: 140px)`), push/tile/action/light row/column/tile/select/number layouts, `.hd-more` structure (44×44 top-right, hidden at 1×1, head padding), confirmed check (`.hd-widget[data-hd-confirmed="true"] .hd-state::before` with `--hd-mask-check`, `--hd-ok-ink`), sheet internals (`.hd-mi`, two columns ≥ 560 px, frame with a `foot` row, `.hd-sheet-close`).
  - page: `.hd-root { position: relative; }`, moreinfo layer (scrim `color-mix(in srgb, var(--hd-scrim) calc(var(--hd-scrim-alpha) * 100%), transparent)`, centre or bottom sheet), dialog sizes, `.hd-pop-layer`, listbox panel from `--hd-pop-*`, base listbox paint from card tokens.
  - Motion (inside the gate selector only): armed bar animation 4000 ms linear forwards; error shake on the first body child 320 ms; slider/thumb transitions on `transform` only when not `[data-hd-dragging]`; check fade 600 ms.
- [ ] Lead approval for the token (Q3) → `node scripts/homedash-freeze-tokens.mjs`; `npm run build:ui`; `npm run test:homedash` (css-lint, guard, freeze) PASS; `npm run check:ui`.
- [ ] Commit (CSS parts, bundle, freeze JSON) — `feat(homedash): W1 and P1 structure, overlay layers and the check mask token`.

### Task 17: Lab — states, W1 fixtures, sheets, light-sheet cell, stricter hit probe

**Files:** `ui/testdata/homedash-lab.js`, `homedash-lab.css`, `homedash-lab-probes.js`, `homedash-lab-sheets.json`, `homedash-lab/hero-fixtures.js`, create `homedash-lab/w1-fixtures.js`, `ui/desktop_homedash_lab_plan_test.go`, `scripts/homedash-tests/lab.test.mjs` (if it pins state names). **Impact:** `homedashLabHero`, `homedashLabExpand` (lab plan; LOW), lab `LAB_STATES`, `addWidget`, `render`, `main`.
- [ ] Go: `homedashLabHero` gets `Sizes []string \`json:"sizes"\``; in `homedashLabExpand`, when the spec has no `sizes` and a `{hero}` matched, use the hero's sizes. Extend `TestHomeDashLabSheetResolve`: `w1-W3-glass` → 2 panels, `sizes=2x1,4x1`; `p1-H3-tactile` → `sizes=2x2,3x3`; `sheet-light-minimal` → `types=moreinfo`.
- [ ] Sheets JSON: `lists.states` adds `w1` (off, on, mid, pending, armed, error, unavailable, readonly, stale), `w1Core` (off, on, mid, pending, unavailable), `w1Trio` (off, on, mid), `p1` (pending, armed, error, focus, drag), `sheet` (on, colour, dimonly, off, pending, unavailable, readonly), `open` (open). `lists.heroes` adds sizes to H1 (1x1,2x1), H2 (1x2,2x2), H3 (2x2,3x3) and W1–W10 per the build sheets (`W1` switch/push … `W10` number/slider). Sheets (scripts `hero-fixtures.js` + `w1-fixtures.js`, `more: true` except P sheets of heroes):
  `w1-{hero}-{family}` (dark, light; `w1`; dpr 1; motion none), `w1-{hero}-{family}~dpr2` (`w1Core`, dpr 2), `w1-themes-{family}` (glass, minimal; themes16; `w1Trio`; all W1 types/variants; 2x2), `w1-risk-{family}` (tactile, instrument; risk8; off,on,mid,focus; 2x2), `w1-{family}~custom` (custom3; `w1Trio`; 2x2), `w1-glass~gradient`, `w1-side-by-side-{hero}` (four families; `w1Core`), `w1-list-open-{family}` (select/list; `open`; 2x1), `sheet-light-{family}` (types moreinfo, variants light; `sheet`; sizes 7x6, 4x7), `sheet-light-{family}~dpr2` (on, colour; 7x6; dpr 2), `p1-{hero}-{family}` (dark, light; `p1`; motion none), `p1-{hero}-{family}~reduced` (dark; `p1`; motion reduced), `motion-p1-{hero}-{family}` (kind motion; frames 0,50,100,200,400,800; scenarios press, confirm, rollback, arm; motion full, reduced), `hero-H8-instrument~wide` (instrument; sensor/sparkline; display states; 2x1, 4x2, 5x2).
- [ ] `homedash-lab.js`: `LAB_STATES` adds `mid, armed, drag, colour, dimonly, open`; `addWidget` after the first render: `drag` → find `[data-hd-input="drag"]`, dispatch synthetic `PointerEvent`s (`pointerdown` at 25 %, `pointermove` to 60 % of its rect, primary, `pointerType:'mouse'`, `buttons:1`) and leave the gesture open; `open` → click the `.hd-pick`; `focus` → `data-hd-focus="ring"` on the first control in `.hd-body` (`FOCUS_SELECTOR`), else `.hd-more`. Special type `moreinfo` (accepted in `main`): `addSheet` renders `E.moreinfo.build({family, api, popoverRoot: layer})` in a `div.hd-sheet-layer[data-kind=moreinfo][data-hd-pop=center|sheet]` sized by the cell box (lab css: `.hd-lab-stage > .hd-sheet-layer { position: relative; inset: auto; inline-size: var(--hd-lab-w); block-size: var(--hd-lab-h); }`). The lab widget api gains `command: () => Promise.resolve({ status: 'local' })` (unchanged otherwise).
- [ ] Probes: `INTERACTIVE` adds `'[role="option"]'` (stricter; never weaker).
- [ ] `hero-fixtures.js`: the H3 light moves to `brightness_pct` (163→64, 3→1, 255→100, 51→20, 204→80), `color_mode`, `supported_color_modes`, object capabilities as in contract §2.
- [ ] `w1-fixtures.js`: for each W1 variant and the `moreinfo/light` sheet define views for every state (`mid` = brightness 50 / value mid-range / option 2 of 3; `armed` = `armed:{command, seconds:3}`; `error` = previous value + `status:'error', error:{code:'timeout', text:'Not confirmed by the device'}`; `readonly` with `readonlyReason: 'home_assistant.allowed_services'`; `stale`; `unavailable`), using the contract demo entities (names, units, options); scenarios `press` (off → on), `confirm` (pending → confirmed), `rollback` (pending → error with previous value), `arm` (off → armed); add the same four scenarios to the H1–H3 entries.
- [ ] Self-check runs (placeholder family): `w1-W1-placeholder` … `w1-W10-placeholder`, `sheet-light-placeholder`, `w1-list-open-placeholder`, `p1-H1-placeholder`, `p1-W3-placeholder` with `HOMEDASH_LAB_REPORT_ONLY` unset; zero `errors`.
- [ ] `go test ./ui -run '(?i)^TestHomeDashLab' -count=1`, `npm run test:homedash`. Commit — `test(homedash): lab states, W1 fixtures, P1 scenarios and M3 sheets`.

### Gate W-S (lead; structure)

Run the placeholder set of Task 17 (R7 commands; `$env:HOMEDASH_LAB_SHEETS` = the list). Judge with the M1 rubric S1–S8 and U1/U3/U11 plus: 44 px hits for every control incl. options, no layout shift across `w1` states, 1-row layout keeps title ≥ 6 characters, sheet layout centred (7×6) and bottom (4×7), popover anchored. Findings in `reports/homedash-lab/findings/W-S-r<n>.md`; fix tasks touch only the structure files; pass on fresh sheets.

### Tasks 18–21: Family paint (one task per family, then its gate)

Each task: **Files** = that family file + bundle; **read first** the build sheets, the family's M1 section in `documentation/homedash-visual-system.md`, the frozen token table, the M1 hero recipes of the same primitive (pill/rocker/dial/segmented/climate steps). Paint: push chip, switch tile, light power chip, slider (h/v, fader cap), light tile wash and disc, action button, picker, stepper keys and readout, number readouts, listbox panel and options, light sheet (dialog card, close button, labels, CT strip rim, wheel rim and thumb, effect picker), `.hd-more`, and the state table (pending, armed, error, confirmed, drag, on, read-only, unavailable, stale, focus). Motion only inside the gate selector, transform/opacity/stroke-dasharray only. `npm run build:ui`; `npm run test:homedash` (lint + freeze) PASS. Commit per family — `feat(homedash): <Family> paint for W1 controls, the light sheet and P1 states`.

- **Task 18 Glass** (recipes: glass chip = `--hd-surface` + `--hd-edge-hi` + `--hd-g-surface-edge`; on = `--hd-on` + `--hd-g-fill-hi`, ink `--hd-on-ink`; thumbs `--hd-thumb` + `--hd-thumb-shadow`; light fill/wash `--hd-light-on`; number fill `--hd-value-colour`; pending ring `--hd-pending-ring(-on)`; sheet = card tokens + `backdrop-filter: blur(var(--hd-glass-blur))` only under the gradient backdrop; focus + `--hd-theme-glow`). → **Gate W-G**.
- **Task 19 Tactile** (domed bakelite keys `--hd-t-bakelite` + `--hd-lip` + `--hd-shadow-cast`, pressed `--hd-recess`; pilot LED `--hd-t-pilot`/`--hd-t-led` with `--hd-led` state colours; slider slot `--hd-sunken` + `--hd-recess`, fader cap bakelite + `--hd-t-grip`; readouts in VFD windows `--hd-t-window`/`--hd-t-vfd`/`--hd-vfd-glow`; sheet = steel rim `--hd-t-steel-lin` around `--hd-t-panel`; one light direction; legends ≥ 10 px engraved). → **Gate W-T**.
- **Task 20 Minimal** (text + hairline; on = ink fill with inverted glyph (`--hd-thumb-on`); tracks 2 px `--hd-track-line`, dot thumbs `--hd-m-dot-r`; accent only in focus and value arcs; pending hairline ring; no shadows/gradients; sheet flat with hairline). → **Gate W-M**.
- **Task 21 Instrument + D8 token** (bracketed keys `--hd-i-key-wash`/`--hd-i-key-edge`, HUD top light bar + `--hd-glow`; sliders as 20-segment LED ladder over the fill; readouts seven-segment / Share Tech Mono in `--hd-i-val`; sheet = HUD panel `--hd-card-bg` with `--hd-i-corners`; one glow colour). **D8 change** (approved): in `homedash-family-instrument.css` replace the `@container hd-widget (aspect-ratio >= 1.6)` value `--hd-i-seg-h: 15px;` with `--hd-i-seg-h: calc(22cqmin * 0.72);` (the cap height of the Glass 22cqmin numeral: 15.2 px at 2×1, 32.3 px at 4×2 and 5×2; the 11 px state text rule stays). Then `node scripts/homedash-freeze-tokens.mjs`, `npm run test:homedash`, bundle; update the "Instrument wide sensors" ruling in Task 25. → **Gate W-I** (includes the re-judged Instrument sheets).

**Gate W-<family> (lead):** build `go test -c -o D:\tmp\go-build\hd-m3-ui.test.exe ./ui`; from `ui` (sandbox off): `$env:AURAGO_RUN_BROWSER_SMOKE='1'; Remove-Item Env:HOMEDASH_LAB_REPORT_ONLY -ErrorAction SilentlyContinue; $f='<family>'; $h = 1..10 | % { "W$_" }; $ids = @($h | % { "w1-$_-$f" }) + @($h | % { "w1-$_-$f~dpr2" }) + @("sheet-light-$f", "sheet-light-$f~dpr2", "w1-list-open-$f", "w1-$f~custom") + <extras>; $env:HOMEDASH_LAB_SHEETS = $ids -join ','; & D:\tmp\go-build\hd-m3-ui.test.exe '-test.run=^TestDesktopHomeDashControlLabBrowser$' '-test.count=1' '-test.timeout=30m' '-test.v'`; then from the worktree `$env:HOMEDASH_LAB_CVD='1'; go test ./ui -run '^TestHomeDashLabColourVisionSheets$' -count=1`. Extras: Glass `w1-themes-glass`, `w1-glass~gradient`; Tactile `w1-risk-tactile`; Minimal `w1-themes-minimal`; Instrument `w1-risk-instrument`, `hero-H8-instrument`, `hero-H8-instrument~dpr2`, `hero-H8-instrument~wide`, `hero-risk-instrument`, `side-by-side-H8`. Order: `probes.json` first (any failure is a finding: contrast incl. gradient worst case and SVG host, aria-hidden HTML text 4.5:1, unavailable measured, 44 px hits incl. options, layout shift, running animations under reduced/none), sheets at 100 % and 400 %, dpr2, sweeps, greyscale/CVD. Rubric: U1–U13, the family rubric (G/T/M/I 1–7), plus W1a–W10a and MI-a:
  - W1a push: icon and word centred, 44 h, on unmistakable in greyscale. W2a tile: whole body is the target; on reads at arm's length. W3a/W10a slider: thumb centred on the track, fill ends under the thumb centre at 0/50/100, tabular readout. W4a fader: cap 40×24 centred. W5a light tile: wash 18–40 % by level, text contrast holds on the wash. W6a action: verb legible, check visible. W7a segmented: indicator aligned, ellipsis on long options. W8a list: picker ellipsis, chevron 16, popover rows 44 with check, anchored/bottom correct. W9a stepper: buttons 44, end button disabled at min/max. MI-a sheet: title/area/state hierarchy, controls on one grid, wheel crisp at 1× and 2×, CT gradient smooth, close 44, footer muted; same family identity as its widgets.
  Findings `reports/homedash-lab/findings/W-<F>-r<n>.md` (ID, sheet, variant, family, state, size, palette/theme, observation, required change, rubric); no minor tier; every finding closed on fresh sheets; a finding surviving three rounds needs an approach change. Expected time per family ≤ 40 min.
**Gate W-X (lead):** `w1-side-by-side-W1` … `W10`, `side-by-side-H1`, `side-by-side-H2`, `side-by-side-H3` (head icons, SCR-M3-1), plus `TestDesktopHomeDashBrowser` with `HOMEDASH_M2_SHOTS=1` (starter in four families with header icons and the Instrument wide sensors) and `TestDesktopHomeDashControlsBrowser` with `HOMEDASH_M3_SHOTS=1` (HA dashboard pages in four families). Rubric X1–X5 (blind crops of push, slider, tile).

---

## Batch E — host, fixture, browser test (may run while the lead judges W sheets; disjoint files)

### Task 22: Desktop host

**Files:** `ui/js/desktop/apps/homedash.js`, `homedash-home.js`, `ui/css/desktop-app-homedash.css`, `home.test.mjs`, `host.test.mjs`. **Impact:** `visibilitySignal`, `render` (host), `create`/`render`/`noticeText` (home); LOW.
- [ ] Tests: visibility with an IntersectionObserver that has not reported yet is visible (stub `IntersectionObserver` class); home with `templates:['system','home_assistant']` shows the third choice card and the header button `create-ha`, clicking calls `onHa`; hidden when read-only or when the template is missing; notices `template_empty` and `source_unavailable` (text contains the source name).
- [ ] `visibilitySignal`: `let inView = null` (unknown); `compute = () => !document.hidden && inView !== false && (…window checks…)`.
- [ ] Host: `Home.create({…, onHa: () => inst.createHomeAssistant()})`; menu item `homedash.menu.new_ha` when `s.templates` includes `home_assistant` (menus signature adds templates); `sourceName` uses `homedash.source.ha`; `bannerText` handles `source_catalog_failed` and `template_omitted` (`count`); `BANNER_ICONS` add `source_catalog_failed: 'alert'`, `template_omitted: 'info'`; `menus()` re-runs when templates change.
- [ ] Home: choice `create-ha` (icon `home`, `homedash.home.create_ha` / `_hint`), header button `create-ha`; `render(s)` toggles both by `s.templates` and read-only; `noticeText` cases `template_empty`, `source_unavailable` (`t('homedash.home.notice_source_unavailable', { source })` — pass `st.noticeSource` mapped through the host's `sourceName` via a new option `sourceName`).
- [ ] CSS: `.hdx-choices` lays out 2 or 3 equal cards (`grid-template-columns: repeat(auto-fit, minmax(220px, 1fr))`), equal heights; materials only.
- [ ] `npm run test:homedash`, `npm run check:ui`, `go test ./ui -run '(?i)^(TestDesktopAppMaterialsAdoptedByApps|TestHomeDashWiring)$' -count=1`. Commit — `feat(homedash): Home Assistant template choice, notices and visibility in the desktop host`.

### Task 23: Fixture M3 and shared testdata

**Files:** create `ui/desktop_homedash_fixture_m3_test.go`, `ui/testdata/homedash-m3/ha-dashboard.json` (the `dashboard` object of contract §5's 201 answer verbatim); modify `ui/desktop_homedash_fixture_test.go`, `ui/testdata/homedash-m2/starter-dashboard.json` (widget `icon`: cpu, memory, disk, download, upload, timer), `system-entities.json` (uptime `icon_hint` `timer`). **Impact:** `homeDashFixture`, `homeDashRoutes`, `ServeHTTP`, `entityLocked`, `subscribe`, `create` (fixture; LOW, test-only).
- [ ] Edits to the M2 fixture: struct field `m3 *homeDashM3`; route `"commands": {http.MethodPost: true}`; after the availability/read-only checks, sleep `m3.latency` for `bootstrap`, `widget-catalog`, `dashboards`, `dashboard`, `stream/subscriptions`; bootstrap `templates` adds `home_assistant` when `m3.listed`, `sources` adds `m3.sources["ha"]`; `entityLocked` returns a clone of an `ha` entity with its current state; `subscribe` accepts known `ha:` ids (snapshot after `m3.snapDelay` via `time.AfterFunc`, guarded by `f.quit`); `create` branches `home_assistant` to `f.createHA`; `unassigned_title` with another template → 400.
- [ ] `desktop_homedash_fixture_m3_test.go` (types and functions):
```go
// M3 additions to the HomeDash fixture (contract "API contract (frozen for M3)" of docs/superpowers/plans/2026-11-25-homedash-m3-backend.md):
// the ha demo entities of hasstest.DemoOptions(), the commands route with expected states and state frames, the home_assistant template,
// source statuses, latency, and switches for every P1 path (delayed, dropped, duplicated or skewed confirmation; error answers).
type homeDashCommand struct { Entity, Command, RequestID, DashboardID, WidgetID string; Args map[string]any; Confirmed bool; At time.Time }
type homeDashAnswer struct { status int; body map[string]any; times int; entity string }
type homeDashConfirm struct { delay time.Duration; drop, duplicate bool; skew map[string]any }
type homeDashM3 struct {
	ha map[string]map[string]any; haDoc map[string]any; listed bool; omitted int; mode string // "", "empty", "down"
	sources map[string]map[string]any; commands []homeDashCommand; answers []*homeDashAnswer; confirm homeDashConfirm
	latency, snapDelay time.Duration
}
func (f *homeDashFixture) enableM3(t *testing.T)                                   // loads ha-entities.json and ha-dashboard.json; ha source idle; confirm delay 80 ms
func (f *homeDashFixture) command(w http.ResponseWriter, raw []byte)              // contract §4 order: body (400 bad_request), queued answer, entity (404), capability (400 unsupported_command), allowed (403 read_only + blocked_by), sensitive without confirmed (409), args (400 invalid_args), then 202 with expected and a state frame after confirm.delay (none when drop; twice when duplicate; skew attributes merged into the frame)
func (f *homeDashFixture) createHA(w http.ResponseWriter, body homeDashCreateBody) // mode down → 503 source_unavailable {"source":"ha"}; empty → 422 template_empty; else the template doc with name (default "Home Assistant"), last page name = unassigned_title (default "Unassigned"), slug home-assistant(-n), 201 with "omitted": m3.omitted
func homeDashExpected(st map[string]any, command string, args map[string]any) map[string]any // the contract table (nil for activate/press and toggle from a non-boolean)
func homeDashApply(st map[string]any, command string, args map[string]any) map[string]any    // the next state (brightness 0 or turn_off clears brightness_pct; scene/button value = now)
func homeDashArgsError(cp map[string]any, command string, args map[string]any) string      // brightness int 0-100; kelvin min+k*step (step 50) and ≤ max; h 0-360, s 0-100; option in options; set_value min+k*step within 1e-6*step
// controls
func (f *homeDashFixture) answerNext(status int, body map[string]any, times int, entity string)
func (f *homeDashFixture) setConfirm(c homeDashConfirm)
func (f *homeDashFixture) commandLog() []homeDashCommand
func (f *homeDashFixture) pushHAState(id string, st map[string]any)   // stores and sends a state frame to subscribed streams
func (f *homeDashFixture) setSource(src, state, reason string)        // bootstrap status and a source frame to streams holding ids of src
func (f *homeDashFixture) setHATemplate(listed bool, omitted int, mode string)
func (f *homeDashFixture) setLatency(d time.Duration)
func (f *homeDashFixture) setSnapshotDelay(d time.Duration)
func (f *homeDashFixture) setWidgetConfirm(t *testing.T, dashboardID, widgetID string, on bool) // bumps revision, dashboard_changed
func (f *homeDashFixture) setScriptAllowed(on bool)                    // script.goodnight: allowed, sensitive, readonly false (+ entity frame)
func (f *homeDashFixture) seedDashboard(doc map[string]any) string    // stores a copy with a new id, returns it
```
  Error bodies follow the contract table exactly (`{"status":"error","code":…,"message":…}` plus `blocked_by`, `retry_after_ms` + `Retry-After`, `source`).
- [ ] `go vet ./ui`; `go test ./ui -run '(?i)^TestDesktopHomeDashBrowser$' -count=1` (skips without smoke; compiles). Node tests that read the starter still PASS. Commit — `test(homedash): fixture commands route, ha demo entities and template (M3 contract)`.

### Task 24: TestDesktopHomeDashControlsBrowser

**Files:** create `ui/desktop_homedash_controls_browser_test.go`. Name matches shard "Desktop H-R".
- [ ] Steps (each with explicit waits; `w(id)` = `[data-hd-wid="<id>"]`; German words through `window.fixtureWords`; `desktopAuditBrowserWithAPI` with `windows.animations=false`, so motion is reduced and assertions do not depend on animations):
  0. **First values (D8).** `fixture.setLatency(250ms)`; `seedDashboard(starter)`; open the app; in one `MustEval`, click the list card and poll in rAF until `[data-hd-wid="cpu"] .hd-gauge[aria-valuenow]` exists; return elapsed; assert ≤ 1500 ms. Header icons present (`[data-hd-wid="memory"] .hd-icon svg[data-hd-icon="memory"]`). `setLatency(0)`.
  1. **Template flow.** Back home; `setHATemplate(true, 0, "empty")` + back/forward to re-read bootstrap → click `create-ha` → notice `homedash.home.notice_template_empty`; mode `down` → notice with "Home Assistant"; mode `""` with omitted 2 → POST body `{"template":"home_assistant","name":"Zuhause","unassigned_title":"Ohne Raum"}`; three tabs (Kitchen, Living room, Ohne Raum); banner `template_omitted` text contains "2".
  2. **Toggle.** `setConfirm({delay: 900ms})`; install a MutationObserver recording when `w2` gets `data-hd-pending`; click `w2 [role=switch]`; `aria-checked` true within one frame; command `turn_on` for `ha:switch.coffee`, non-empty request_id; pending appeared ≥ 240 ms after the click; after the frame pending is gone and `data-hd-confirmed` showed.
  3. **Dim by keyboard.** Focus `w1 .hd-slider`; ArrowRight ×3 within 300 ms; `aria-valuenow` 43 at once; exactly one `brightness` command with value 43 about 400 ms after the last key.
  4. **Dim by pointer.** Rod mouse: down at 20 % of the track, 26 moves of 100 ms to 80 %, up. Intermediate brightness commands during the drag ≤ 3 and ≥ 1200 ms apart; the last command equals `aria-valuenow` after release; confirmed (no pending, no error).
  5. **Select.** Click `w3 [data-hd-option="boost"]` → `select_option boost`; on page Ohne Raum click `w9 .hd-pick` → listbox visible; ArrowDown, Enter → `select_option Rock`; listbox gone; focus on the picker.
  6. **Number.** `w10` click `+` → one `set_value 56` after ~400 ms; Living room `w6` slider focus, End → `set_value 100`.
  7. **Armed confirm.** `haID` = the dashboard id returned by the step 1 POST. `fixture.setWidgetConfirm(t, haID, "w5", true)`; click `w5` → no command, `data-hd-armed=true`, state text contains the armed word; click again → `activate` with `confirmed:true`. Arm again and wait 4.5 s → armed gone, no command. `fixture.answerNext(409, map[string]any{"status": "error", "code": "confirmation_required", "message": "this command needs a confirmation"}, 1, "ha:button.doorbell_test")`; click `w8` → command (confirmed false) then armed; click → `press` with `confirmed:true`.
  8. **Rollback without confirmation.** `setConfirm({drop:true})`; click `w7` (vacation, off) → on; after ≤ 12 s: off again, `data-hd-status=error`, state text = `homedash.cue.timeout`, live region contains it; ≤ 7 s later status ok. `setConfirm({delay:80ms})`.
  9. **Error cues.** `fixture.answerNext(502, map[string]any{"status": "error", "code": "command_failed", "message": "Home Assistant did not accept the command"}, 1, "ha:switch.coffee")` → click `w2` → immediate rollback + `homedash.cue.command_failed`; `fixture.answerNext(503, map[string]any{"status": "error", "code": "source_unavailable", "message": "Home Assistant is not reachable", "source": "ha"}, 1, "ha:switch.coffee")` → click `w2` → cue contains "Home Assistant"; `fixture.answerNext(429, map[string]any{"status": "error", "code": "rate_limited", "message": "too many commands", "retry_after_ms": 700}, 1, "ha:light.kitchen")` then keyboard End on `w1` → two `brightness` commands with the same args, different request_ids, ~700 ms apart; final confirmed.
  10. **Read-only.** `fixture.answerNext(403, map[string]any{"status": "error", "code": "read_only", "message": "commands are switched off", "blocked_by": "home_assistant.allowed_services"}, 1, "ha:switch.coffee")` → click `w2` → no error status, `data-hd-readonly=true`, state contains `homedash.reason.allowed_services`. Then `setReadonly(true)` + policy event → click `w2` → no new command; state contains `homedash.reason.desktop_readonly`. `setReadonly(false)` + event.
  11. **Unavailable.** `pushHAState(coffee, {value:nil, available:false, text:"unavailable"})` → `[role=switch]` `aria-disabled=true`, state "Nicht verfügbar" (`homedash.state.unavailable`).
  12. **Light sheet + late adoption.** Focus `w1` slider, Shift+Enter → `.hd-sheet[data-kind=moreinfo][role=dialog]` visible, focus inside; `setConfirm({delay:80ms, skew:{"color_temp_kelvin":6300}})`; focus the CT strip, End → `color_temp {kelvin:6502}`; after 9 s no error on the dialog and CT `aria-valuenow` 6300; Escape → dialog gone, focus back on the slider. Hold with the mouse on `w4` (sofa tile, 600 ms) opens the sheet with only the power row; Escape.
  13. **Per-source notices.** `setSource("ha","degraded","unreachable")` → banner `source_degraded` containing "Home Assistant"; `("ha","unavailable","catalog_failed")` → `source_catalog_failed`; back to `live`.
  14. **Screenshots** when `HOMEDASH_M3_SHOTS=1` to `reports/homedash-m3/` (git-ignored): pages per family (via `fixture.setFamily`) in standard-dark and fruity-light, sheet (Glass, Tactile), armed, error cue, list open, read-only, mid-drag (taken with the mouse button still down).
  15. `window.fixtureErrors` empty.
- [ ] Run (R7) → PASS; `go test ./internal/audit -run '^TestUIBrowserCIShardsCoverEveryTest$' -count=1` → PASS. Commit — `test(homedash): browser test for W1 controls, P1 paths and the Home Assistant template`.

**Batch E review (reader-agent):** fixture vs contract (status/code bodies, expected table, arg checks, frame order), test asserts every D13 path, no flaky fixed sleeps where a wait works, host a11y of the new choice.

### Gate P (lead)

After Gate W-X and Task 24. Sheets per family: `p1-{W1..W10,H1,H2,H3}-$f`, `p1-…-$f~reduced`, `motion-p1-{…}-$f` (run per family), plus the Task 24 screenshots (real mid-drag, armed, error, sheet). Rubric:
- P-a pending: no indicator at the 200 ms frame, one indicator at 400 ms, word "Sending…"; static under reduced motion.
- P-b armed: countdown bar (or LED/text per family) and text "· N s"; static bar + text under reduced motion.
- P-c rollback: control back to the previous state, cue text in alert ink with icon, family error motion in the strip; none under reduced motion.
- P-d focus: ring visible on every control including options, tile and wheel; Amiga outline adds only.
- P-e mid-drag: handle under the pointer, readout follows, no transition lag; Tactile cap pressed; Glass thumb grown.
- P-f reduced motion: probes report no running animation; all states distinguishable in greyscale.
Findings `reports/homedash-lab/findings/P-r<n>.md`; same round rules as W.

---

## Batch F — docs, preview, hand-over, live test

### Task 25: Documentation (docs-agent)

**Files:** `ui/js/homedash/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `documentation/homedash-visual-system.md`, root `AGENTS.md` (HomeDash row).
- [ ] Engine guide: parts `E.input`, `E.control`, `E.popover`, `E.moreinfo`, `E.command`; load orders; "P1 interaction rules" 1–12 verbatim; the widget api (`can`, `command(binding, cap, args, {phase})` resolving `{status}`, `moreInfo` lights only, entity view fields `pending`, `error{code,text}`, `armed{command,seconds}`, `confirmed`, `readonly`, `readonlyReason`); primitive input contract; store rule 3 with `readonlyReason`; first-value rules (parallel boot, immediate first PUT, visibility unknown = visible); new tests in Verification; the controls browser test command.
- [ ] Apps guide (HomeDash section): template choice and re-read, notices, menus, visibility rule.
- [ ] Visual system: "M3 additions" (W1 build sheets summary, state table, SCR-M3-1…8, `--hd-mask-check`, sheet ids in use, `[role=option]` hit probe); "Lead rulings" updates (Instrument wide sensors → `calc(22cqmin * 0.72)`; every ruling the gates produced).
- [ ] `go test ./internal/audit -count=1` (worktree). Commit (`git add` tracked files) — `docs(homedash): M3 interaction layer, W1 controls and visual rulings`.

### Task 26: German preview pack (not blocking)

- [ ] Run `TestDesktopHomeDashControlsBrowser` with `HOMEDASH_M3_SHOTS=1` (German words are already loaded) and the lab sheets `w1-side-by-side-W3`, `w1-side-by-side-W5`, `sheet-light-glass`, `sheet-light-tactile`. The lead copies 10–14 PNGs into `reports/homedash-m3/preview-de/` (HA dashboard in four families, light sheet, armed, error cue, list open, read-only, mid-drag) and sends the user the folder path with a five-line German summary in chat. No repository file.

### Task 27: Final verification and hand-over

- [ ] test-agent: `npm run test:homedash` (count), `npm run check:ui`, `go vet ./ui`, `go test ./ui -run '(?i)^(TestHomeDashWiring|TestHomeDashLab.*|TestTranslations_|TestFrontend_StaticI18nKeysExistInEnglishBundle|TestDesktopAppMaterialsAdoptedByApps|TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget)$' -count=1 -v` (budget failures equal the Task 0 baseline), `go test ./internal/desktop -count=1`, `go test ./internal/audit -count=1`, `TestDesktopHomeDashBrowser` and `TestDesktopHomeDashControlsBrowser` (R7) PASS, lab default sheets clean.
- [ ] Lead: MCP `detect_changes({scope:"compare", base_ref:"main", repo:"AuraGo", worktree})`; re-run if partial.
- [ ] docs-agent writes `docs/superpowers/specs/2026-11-23-homedash/m3-handover-frontend.md` (≤ 100 lines): commits, test results with counts, gate rounds and pass dates, deviations, contradictions resolved (self-review), lead answers to Q1–Q10, uncovered contract paths (401, 413, 500, 504 on commands; `schema_too_new`; `hub_closed`), open items (kiosk inherits P1 in M6; sensor sheet M5; right-click menus M4), "nothing pushed or deployed". `git add -f`, `git ls-files`, commit — `docs(homedash): M3 frontend hand-over`.

### Task 28: Live-handler browser test (last; after the backend landed)

**Precondition:** the backend branch is on `main` (`git log --oneline -1 main -- internal/homedash/sources/hass/hasstest/server.go` not empty) and Q1 is answered.
- [ ] Rebase this branch onto `main` (no restore/reset/stash; resolve conflicts by keeping both sides' disjoint files); `npm run build:ui`, `npm run check:ui`, `npm run test:homedash`.
- [ ] Default (Q1 approved): create `internal/server/desktop_homedash_live_browser_test.go`:
```go
package server

// TestDesktopHomeDashLiveHABrowser (D13, M2's D9): the desktop shell with the real HomeDash handlers (handleHomeDash) and hasstest as Home Assistant.
// It opens HomeDash, creates the home_assistant dashboard, switches the coffee machine on and dims the kitchen light, then checks the fake's service
// calls and that the UI confirmed (no pending, no error, values from the state frames). Needs AURAGO_RUN_BROWSER_SMOKE=1 and Chrome.
func TestDesktopHomeDashLiveHABrowser(t *testing.T) {
	browser := personalRadioBrowser(t)                       // skips without AURAGO_RUN_BROWSER_SMOKE
	shortenHomeDashStreamTimers(t)
	ha := hasstest.New(t, hasstest.DemoOptions())
	env := newHomeDashHAEnv(t, ha)                           // backend Task 13 helper
	sourcetest.WaitFor(t, 10*time.Second, func() bool {
		var boot struct{ Templates []string `json:"templates"` }
		hdDecode(t, env.do(t, env.admin, http.MethodGet, "bootstrap", nil), &boot)
		return slices.Contains(boot.Templates, "home_assistant")
	}, "bootstrap lists home_assistant")
	i18n.Load(ui.Content, slog.Default())
	read := func(p string) string { b, err := ui.Content.ReadFile(p); if err != nil { t.Fatal(err) }; return string(b) }
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(read("desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script type="application/json" id="aurago-template-data">__DATA__</script><script src="/js/shared/template-data.js"></script><script>window._auragoSharedInitialized=true;window.hdErrors=[];addEventListener('error',e=>hdErrors.push(e.message));addEventListener('unhandledrejection',e=>hdErrors.push(String(e.reason)));</script><script src="/js/shared/shared-core.js"></script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/hd-shell.js"></script></body>`, 1)
	shell := read("js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 { t.Fatal("shell seam missing") }
	shell = shell[:cut] + `window.hdTest={state,openApp};})();`
	api := handleHomeDash(env.s)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(ui.Content)))
	mux.HandleFunc("/api/i18n", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":%s}`, getI18NJSONForSections(normalizeLang(r.URL.Query().Get("lang")), strings.Split(r.URL.Query().Get("sections"), ",")...))
	})
	mux.Handle(homeDashAPIPrefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { r.Header.Set("Authorization", "Bearer "+env.admin); api.ServeHTTP(w, r) }))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "application/json"); fmt.Fprint(w, `{}`) })
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, strings.Replace(html, "__DATA__", fmt.Sprint(uiTemplateData("de", "desktop")["TemplateDataJSON"]), 1))
	})
	mux.HandleFunc("/hd-shell.js", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "text/javascript"); fmt.Fprint(w, shell) })
	origin := httptest.NewServer(mux)
	defer origin.Close()
	page := browser.MustPage().Timeout(2 * time.Minute)
	defer page.Close()
	page.MustSetViewport(1280, 900, 1, false)
	page.MustNavigate(origin.URL + "/fixture").MustWaitLoad()
	page.MustEval(`()=>{hdTest.state.bootstrap={enabled:true,builtin_apps:[{id:'homedash',name:'HomeDash',icon:'homedash'}],apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{'appearance.theme':'standard','windows.restore_session':false,'windows.animations':'false'}};document.body.dataset.animations='false';const d=document.getElementById('vd-disabled');if(d)d.hidden=true;hdTest.openApp('homedash');}`)
	page.MustWait(`()=>!!document.querySelector('.hdx-home [data-action="create-ha"]:not([hidden])')`)
	page.MustEval(`()=>document.querySelector('.hdx-home [data-action="create-ha"]:not([hidden])').click()`)
	page.MustWait(`()=>document.querySelectorAll('.hdx-view .hd-tab').length===3&&!!document.querySelector('[data-hd-wid="w2"] [role="switch"]')`)
	page.Timeout(20*time.Second).MustWait(`()=>document.querySelector('.hdx-view .hd-root').dataset.hdConn==='live'&&document.querySelector('[data-hd-wid="w2"] [role="switch"]').getAttribute('aria-checked')==='false'`)
	page.MustEval(`()=>document.querySelector('[data-hd-wid="w2"] [role="switch"]').click()`)
	waitCall := func(domain, service string, match func(map[string]any) bool) {
		sourcetest.WaitFor(t, 10*time.Second, func() bool {
			for _, c := range ha.Calls() { if c.Domain == domain && c.Service == service && match(c.Data) { return true } }
			return false
		}, domain+"."+service)
	}
	waitCall("switch", "turn_on", func(d map[string]any) bool { return d["entity_id"] == "switch.coffee" })
	page.Timeout(10*time.Second).MustWait(`()=>{const w=document.querySelector('[data-hd-wid="w2"]');return w.querySelector('[role="switch"]').getAttribute('aria-checked')==='true'&&w.dataset.hdPending!=='true'&&w.dataset.hdStatus==='ok';}`)
	page.MustElement(`[data-hd-wid="w1"] .hd-slider`).MustFocus()
	page.Keyboard.MustType(input.ArrowRight, input.ArrowRight)
	waitCall("light", "turn_on", func(d map[string]any) bool { return d["entity_id"] == "light.kitchen" && d["brightness_pct"] == 42.0 })
	page.Timeout(10*time.Second).MustWait(`()=>{const w=document.querySelector('[data-hd-wid="w1"]');return w.querySelector('.hd-slider').getAttribute('aria-valuenow')==='42'&&w.dataset.hdPending!=='true'&&w.dataset.hdStatus==='ok';}`)
	if strings.Contains(page.MustEval(`()=>document.body.textContent`).Str(), hasstest.UpstreamSecret) { t.Fatal("Home Assistant text reached the page") }
	if errs := page.MustEval(`()=>hdErrors.join('\n')`).Str(); errs != "" { t.Fatalf("page errors:\n%s", errs) }
}
```
  (Imports: `fmt`, `log/slog`, `net/http`, `net/http/httptest`, `regexp`, `slices`, `strings`, `testing`, `time`, `aurago/internal/i18n`, `aurago/internal/homedash/sources/hass/hasstest`, `aurago/internal/homedash/sourcetest`, `aurago/ui`, `github.com/go-rod/rod/lib/input`. Mirrors `bluetooth_browser_test.go`; adjust the bootstrap object if the shell needs more fields.)
- [ ] Run: `go test -c -o D:\tmp\go-build\hd-m3-server.test.exe ./internal/server`; `cd internal\server; $env:AURAGO_RUN_BROWSER_SMOKE='1'; & D:\tmp\go-build\hd-m3-server.test.exe '-test.run=^TestDesktopHomeDashLiveHABrowser$' '-test.v' '-test.timeout=30m'` → PASS twice. Without smoke → SKIP.
- [ ] Append one line to the hand-over (result, date). Commit (test file + hand-over, `git add -f` for the doc) — `test(homedash): live HomeDash handlers with a fake Home Assistant in the browser (D13)`.

---

## Batches and reviews

| Batch | Tasks | test-agent | reader-agent focus (once, Critical/Important) |
|---|---|---|---|
| A interaction core | 0–6 | `npm run test:homedash` | contract §4 fidelity, timers, never-reject, snap grid, gesture edge cases |
| B engine wiring | 7–10 | `npm run test:homedash`, `npm run check:ui`, wiring/lab Go tests | host additivity, markup unchanged without input, a11y of trap/popover/sheet, boot races |
| C widgets | 11–15 | `npm run test:homedash`, catalog contract | capability-driven ranges, ARIA names/valuetexts, explicit on/off |
| D look | 16–21 + gates W-S, W-G, W-T, W-M, W-I, W-X | lint, freeze, bundle, lab runs | lint/motion/token discipline only; the look is judged on sheets by the lead |
| E host and browser | 22–24 | host tests, both browser tests, shard test | fixture fidelity, D13 coverage |
| Gate P | — | lab + screenshots | lead only |
| F docs and live | 25–28 | final verification, live test | guides match code |

Lean workflow: one review per batch, findings fixed in one follow-up commit per batch, no re-review rounds for minor notes; gate findings have no minor tier.

---

## Self-review

### Coverage
P1: command client + overlay + confirmation + late adoption + rollback/cue + superseded + duplicates (Task 4), continuous throttle + 429 rules + keyboard 400 ms (Task 4, bound in Tasks 8, 12, 15), arming (Task 4, visuals 16–21), read-only/unavailable reasons (Tasks 5, 7), pointer/touch (Tasks 3, 8, 16), keyboard/ARIA (Tasks 8, 11–15, table), focus/reduced motion (Tasks 16–21, Gate P), engine-owned (all in `ui/js/homedash`). W1: all 10 variants + 3 heroes + light sheet (Tasks 9, 11–15), four families (18–21). Icons (1, 7). Carry-overs: first values (5, 10, 22, 24), token (21), header icons (1, 7, 23). App flows (10, 22). Fixture (23). Gates W/P with strict probes (17–21, P). Tests (every task, 24). Live handlers (28).

### Contradictions found and resolutions
1. design-frontend 2.4 throttle 250 ms vs D5 1250 ms → D5.
2. design-frontend 2.4 `widget.confirm` → `E.sheet.confirm` vs D5 arming → D5 (no dialogs).
3. design-frontend 2.4 errors → toast `homedash.error.<code>` vs D5 inline cue with `aria-live` → D5, literal cue keys.
4. design-frontend 2.4 confirmation "state event with `updated_at >= sentAt`" vs contract `expected` match → contract; frames counted from sending (lead Q3).
5. design-frontend 6 more-info for many kinds and colour presets vs D1/visual cut list → lights only, no presets, no history.
6. design-frontend 6 hold cancels at 8 px vs D5 4 px → 4 px everywhere.
7. design-frontend 9.3 sliders `touch-action: none` vs D5 `pan-y`/`pan-x` → D5 (dials and wheel stay `none`).
8. visual 4.2 "select list in sheet" vs D5 "listbox popover" → the visual-system "popover sheet" primitive: anchored ≥ 600 px, bottom sheet below.
9. `hd-w-light.js` reads `brightness / 2.55` vs D6 → `brightness_pct` everywhere; lab fixtures migrated.
10. Contract `toggle` expected depends on the server's current state, which races with the overlay on double taps → switches send `turn_on`/`turn_off` from the shown state (Q5).
11. "Retry once" of a 429 with the same `request_id` would return the cached failure (30 s dedupe) → new `request_id` for the retry.
12. `turn_on` on an already-on light may produce no state change and no frame → the server state at the 202 counts for confirmation.
13. B-1 scopes family colour tokens to `.hd-widget[data-hd-family]`, but the sheet and popover live outside widgets → they carry `hd-widget` + family (SCR-M3-3) instead of new token selectors (no freeze change).
14. A 2×1 frame leaves a 28 px body, below the 44 px hit size → 1-row layout (SCR-M3-5).
15. M2 handover: `.hd-more`, `.hd-sheet` have no production CSS; host styles the delete confirm → engine styles the more-info sheet and popover; the host confirm stays host-styled.
16. Header icons "unknown hint → kind icon" vs M1 heroes without head icons → SCR-M3-1, re-judged at gates W-X/P.
17. D8 "titles from document and catalog" → read as document title, cached entity names and the built-in system names; HA entities have no name before their snapshot (no flash of raw ids).
18. D13 "import the server routes in the test" — impossible from `ui` (`handleHomeDash` and the env helpers are unexported, `server.Start` boots everything) → a test file in package `server` (Q1) or an exported constructor (alternative).
19. M2 handover "M3 live test against a scratch instance" vs D13 fake HA → D13.
20. Visual 7 stale "age > max(2× interval, 5 min)" vs L3 server flag → L3 unchanged.
21. M2 app test "dispose during boot: only bootstrap" vs parallel boot → assertion updated (no request after dispose).
22. M2 fixture `subscribe` rejects non-system ids → accepts known `ha:` ids.
23. Contract §8 `source` frames only reach streams holding ids of that source, so the empty home rarely sees `ha → live` → bootstrap is also re-read on home entry and visibility return.
24. M2 banners showed any source's status → only sources bound by the open dashboard (Q8).
25. `homedash.state.blocked` (M1) assumed `can()` false means blocked; with real gates a blocked capability is read-only with its reason → `statusText` uses reasons; `blocked` key stays for legacy views.
26. Stale testdata: uptime hint `clock` vs server `timer` → testdata fixed.
27. D15 lists only `ui/js/desktop/apps/homedash.js`; `homedash-home.js`, `desktop-app-homedash.css` and the module-loader entry are HomeDash frontend files the backend never touches → included (Q2).
28. Token freeze covers all-`--hd-*` rules: `--hd-mask-check` needs approval and a regeneration (Q3); the D8 change is a second, approved regeneration.

### Depth notes
Full code: `hd-command.js`, the pure part of `hd-input.js`, `hd-popover.js`, `hd-control.js`, render/app integration cores, the command tests, the live test. Outlined with exact rules: input binders' bodies, primitive bindings, light sheet, widget bodies, CSS (geometry fixed, paint by recipe and gates), fixture functions (signatures + contract rules), controls browser test (steps with selectors and assertions). Translations: English and German given; 14 locales by the maker under L5.

---

## Questions for the lead

Each has a default the makers implement unless the lead answers otherwise.

- **Q1 (D13, D15) Live-handler test location.** Default: the frontend track adds the test-only file `internal/server/desktop_homedash_live_browser_test.go` (package `server`, no production change), reusing backend Task 13's `newHomeDashHAEnv` and `handleHomeDash`, served like `bluetooth_browser_test.go`. Alternative if `internal/` stays closed to the frontend: the backend plan adds, in a non-test file without a `testing` import, `func NewHomeDashTestHandler(ctx context.Context, dataDir, haURL, haToken string) (handler http.Handler, closeFn func(), err error)` (auth off, virtual desktop and HomeDash on, `system` and `ha` sources from the given URL/token, store under `dataDir`); the test then lives in `ui` and imports `aurago/internal/server` (cost: the `ui` test binary compiles `internal/server`).
- **Q2 (D15) Shared wiring files.** Default: the frontend edits the `homedash` entry of `ui/js/desktop/core/module-loader.js`, `TestHomeDashWiring`, `homedash-home.js` and `desktop-app-homedash.css`.
- **Q3 (visual 7) Check glyph.** Default: add `--hd-mask-check` to the `.hd-root` token block and regenerate the freeze in Task 16. Alternative: no glyph; confirmation shows only by the indicator stopping.
- **Q4 SCR-M3-1…7.** Default: approved as listed; judged at gates W-S/W/P with before/after sheets of the M1 heroes for SCR-M3-1/2.
- **Q5 (D3) Switch verbs.** Default: `turn_on`/`turn_off` from the shown state, `toggle` only when those are not allowed.
- **Q6 (D5) Widget `confirm` on continuous controls.** Default: ignored (arming applies to discrete activations only).
- **Q7 (D5) 403 answers.** Default: rollback + read-only state with the localized reason + announcement, no red cue; `desktop_readonly` also switches the app read-only.
- **Q8 (D7 c) Source notices.** Default: banners only for sources bound by the open dashboard; `catalog_failed` has its own text.
- **Q9 (D5) Right-click.** Default: not a more-info affordance in M3 (desktop context menus arrive with M4).
- **Q10 (W1) Segmented with more than five options.** Default: renders the list control inside the same widget (the template never produces it).
