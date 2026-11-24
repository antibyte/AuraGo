# HomeDash M0 frontend track — notes (session in worktree recursing-jang-008adb)

Branch: `claude/recursing-jang-008adb` (started at main `db51fa524`).

## State (2026-11-24, after the usage-limit interruption; an earlier version of this note said "24 commits", which was a miscount)

**DONE, ready to land (not merged, not pushed):** `claude/recursing-jang-008adb`, 21 commits `8acc9f52a` …
`40beb5e77` on top of main `db51fa524`; working tree clean. Tasks 1–17, three review-fix commits (engine core,
primitives and host, lab) and the lab fixes from the first browser run. `npm run test:homedash` 79/79,
`npm run check:ui` green, `go vet ./ui`, the audit package and the resolver/pack unit tests green,
`TestDesktopHomeDashControlLabBrowser` PASS with 0 probe failures on the default sheets and, in separate runs, on
`prim-placeholder~custom`, `prim-placeholder~gradient` and `motion-type-entity-placeholder`.

Not exercised in a real browser: dpr 2 panels, the `scripts`, `columns` and `more=0` parameters, a real fixture
scenario (checked in the fake DOM only), real CSS transitions under `seek` (the placeholder look has none), and the
deferred `probes.json` write after a fatal screenshot error.

Smoke command that works in PowerShell (the plan's form fails: PowerShell splits `-test.run` at the dot), run from
`ui\` with the shell sandbox off:
`$env:AURAGO_RUN_BROWSER_SMOKE='1'; & D:\tmp\go-build\hdlab-ui.test.exe '-test.run=^TestDesktopHomeDashControlLabBrowser$' '-test.v' '-test.timeout=30m'`

The root `AGENTS.md` routing table got no HomeDash row on purpose (shared with the backend track): add one combined
row when both tracks land. `ui/AGENTS.md` and `scripts/AGENTS.md` each got one line.

Placeholder look, known cosmetic limits (lab-only, replaced by M1's family CSS): in the 1x1 entity tile the title is
cut to one letter beside the 44 px "more" button, the button is an empty bordered box, and the lamp is hidden.

## Baseline findings (2026-11-24, before any HomeDash change)

- `go test ./ui -run '^TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget$'` FAILS on untouched main `db51fa524`:
  `js/desktop/apps/notes.js` 1142 lines, `software-store.js` 1404, `synth-studio-audio.js` 1108,
  `synth-studio-fx.js` 1443 (limit < 1100). Not caused by HomeDash; any session that runs this test sees it.
- `TestDesktopAppMaterialsAdoptedByApps` passes. `npm run check:ui` passes (2 s). `./ui` test compile: about 100 s.
- GitNexus: MCP `detect_changes` works for a session worktree when called with
  `repo: C:\Users\Andi\Documents\repo\AuraGo` and `worktree: <worktree path>`; `impact` answers too
  (index 18 commits behind, new files unresolved).
- Three headless rod Chrome profiles were running at 09:40 (two started 24.11. 09:39, one 23.11. 15:07). Not touched.

## Contract deviations (C1–C6 of the M0 frontend plan)

No row of C1–C6 was changed. Readings where the contract left room (all implemented and tested):

**C2 (primitives)**
- segReadout: segment paths are `segmentPaths(12, 20, 2.4, 0.6)` (value from the base plan's C2; the governing C2 does
  not state it).
- numeral: trend names are `up`, `down`, `flat`; with `degree` the degree sign also shows for an empty value (M1 CSS
  may hide `.hd-num-deg` under `[data-hd-empty]`); a `text` override clears frac, deg and unit.
- arcGauge: the track path is the full sweep at radius 88 (same `d` as the arc).
- needleGauge: below the jitter threshold the needle keeps its reference value and `--hd-angle`; `--hd-value`,
  `aria-value*` and the readout are still written on every `set`. `numerals` 0 or 1 are guarded.
- bar: `span.hd-bar-readout` exists only with `opts.readout`. sheet: `data-kind` is absent without `opts.kind`.
  frame: `opts.foot` takes a string or a node; `button.hd-more` has no child.
- sparkline: the end dot has `hidden="true"` from build until a last point exists (`.hd-root [hidden]` hides it).
- rocker: `span.hd-rocker-led` is a child of the bezel, after the paddle (as the M1 plan writes it).
- knob: `g.hd-dial-current` is a sibling of `g.hd-dial-handle` under the svg. Without a numeric `current` (or with a
  null value) the root loses `data-buf` and `--hd-angle-cur`, so both buffers stay at opacity 0. `step` writes no
  markup. `ticks: 0` gives an empty tick path.
- stepper: read-only gives `aria-readonly` on the spinbutton and `aria-disabled` on the two step buttons.
- colourWheel: `valueText` becomes the root `aria-label`; the canvas is sized by `devicePixelRatio`.
- pushButton default variant `chip`, slider `h`, knob `ring`, bar `h`.
- Primitives write `data-hd-on="true"` or omit the attribute.

**C3 (widget host)**
- With no primary entity (a type without bindings, or `entity()` returning null) the state line shows the widget's
  `setStatus` text or stays empty, and `data-hd-status` is `ok`. (The plan's rule text would have shown "Unavailable".)
- The wrapper writes `data-hd-on` as `"true"`, `"false"` or absent (absent = not a binary entity). Select with
  `[data-hd-on="true"]`.
- `setTone`/`setStatus` re-read the primary entity through the factory's `entity()`.
- `entity(binding)` is called with the binding NAME (`'primary'`), not an entity id.
- In thumb mode the wrapper gets `data-hd-motion="none"`; the CSS motion gate reads the attribute on `.hd-root` only.

## Open points from the batch A review (engine core), not fixed in M0

- **Solver, `on` vs surface (design question, visual 1.5 step 3).** `solvePalette` follows the design: `on` is the raw
  accent, stepped only until `on-ink` reaches 4.5:1; only `accent` is stepped to 3:1 against the surface. A custom
  accent close to the surface lightness (for example `#1E2A78` on dark) therefore gives an ON fill of about 1.3:1
  against the card. The three custom palettes in the M1 sheets are not affected. Decide in the design whether `on`
  should also reach 3:1.
- **`hd-doc.normalise`** keeps unknown top-level and `look` keys but rebuilds pages and `grid` from the known keys and
  always writes `schema_version: 1`. Fine for schema v1 with a strict server; revisit before a v2.
- **`hd-doc.normalise` has no constraints argument** (catalog `minW`/`maxW` are not applied on load) and
  **`hd-layout.effective` memoises on the lg array only**, so a constraints change alone does not refresh derived
  md/sm layouts. Both matter for the editor milestone, not for M1.
- **`hd-doc.setLayout`** does not check that the layout's ids match the page's widgets.
- Test harness: `eq` compares through JSON, so NaN equals null and -0 equals 0 there; use `assert.equal` or
  `Object.is` for those cases. The fake DOM has no layout (`getBoundingClientRect` is zeros) and no event bubbling.

## Notes for the M1 style lab (from the batch D review of the lab, probes and Go test)

- **C4 clarification, `applyState`:** a scenario name that an instance's fixture defines wins over a lab state of the
  same spelling (M1 defines a scenario `pending`). Only when no instance has such a scenario is the name treated as a
  state. An unknown state name inside a scenario (`to: 'pendng'`) throws instead of rendering the default view.
- **C4 additions:** the lab scope carries `data-fruity-mode`; `scripts` entries with `..` and `graphics` entries that
  are not valid selectors are `bad param` errors.
- **Gradient backdrops (C6 as written):** text or graphics whose nearest background is a background IMAGE are counted
  as `unmeasured`, never as failures. In the M0 lab the cells have opaque backgrounds, so only the panel title and row
  heads are unmeasured (28 of 586). If M1 makes cells or glass cards translucent over the gradient, its G7 gate must
  look at `contrast.unmeasured` itself, or decide on a rule for measuring over gradients.
- **Motion panels (plan contract 5):** no probes run and `hdLab.report` values are not collected for panels with
  `frames`; `probes` is `null` for them in `probes.json`.
- **M0 motion sheet `motion-type-{type}-{family}`** uses the scenario name `on`; without a fixture this falls back to
  the state `on`, i.e. the transition from the rendered state to "on".
- **M1's "lab gaps proposed to M0":** focus marking for widget cells without `.hd-more` is still open (the lab marks
  only `.hd-more`); there is no pre-probe hook (M1's wrapper around `hdLab.probes` works); reduced-motion emulation
  for `motion=full` is done (the Go test emulates `no-preference`, and now also `prefers-color-scheme`).
- **Locale:** the lab has six built-in state words (`LAB_WORDS`) and does not load `/lang/homedash/<lang>.json`;
  `lang=de` therefore shows English state words until M1 wires its locale files in.
- **Sheets file:** named references (palettes, theme lists, custom lists, state sets, motion values, families) are
  validated when the file is loaded, so a typo fails `TestHomeDashLabSheetResolve` instead of a browser run.

## Notes for the M1 style lab (from the batch B review of tokens, base CSS and the guard; no code was changed for these)

- **Guard rule set kept exactly as the M0 plan defines it.** Known holes, left open on purpose because M1 was planned
  against these rules:
  - `transition-duration` alone (inside the gate) passes although the default `transition-property` is `all`.
  - `-webkit-transition` is not checked.
  - The 24px blur cap only sees literal `blur(<n>px)`; `blur(var(--hd-glass-blur))` (which M1 Task uses) and non-px
    units are not measured. So the cap on `--hd-glass-blur` is M1's own responsibility.
  - The `transition` shorthand must name the property FIRST (`transition: opacity 200ms`); `transition: 200ms opacity`
    is rejected with the message "transition of 200ms". A property given through `var()` is rejected by design.
- **Derived tokens resolve at `.hd-root`.** `--hd-focus`, `--hd-accent-soft`, `--hd-focus-gap`, `--hd-surface`,
  `--hd-track`, `--hd-track-fill`, `--hd-thumb*`, `--hd-*-ink`, `--hd-pending`, `--hd-unavailable` are declared with
  `var()` on `.hd-root` (`homedash-bridge.css`), so a family or widget block that overrides a base token
  (`--hd-accent`, `--hd-on`, `--hd-ink`, `--hd-surface-solid` …) must re-declare the derived ones it relies on.
- **C2 reading:** `g.hd-dial-current` is a sibling of `g.hd-dial-handle` inside the dial svg (both rotate by absolute
  angles), not a child of it.
- **Design 9.3** asks the skin-guard block of `homedash-base.css` to set the input font too; M0 has `::selection` and
  scrollbars only (no input element exists in M0). Add the input rule when the first input lands.
- Design 9.4 #1 ("gradients only in token blocks") vs the plan's gradient backdrop rule in `homedash-base.css`: the
  plan's rule was kept (it uses only token colours).
