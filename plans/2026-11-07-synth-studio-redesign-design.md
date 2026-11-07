# Synth Studio Redesign — Design

Date: 2026-11-07 · Status: approved by user in chat, awaiting spec review

## Problem

Synth Studio shows library, arrangement, workbench tabs and sound design side by
side. Everything is compressed, knobs/sliders are small, and nothing is hidden
when it is not needed. The user wants a modern, spacious, 3D-looking surface
with rotary knobs and sliders (reference: dark teal/neon DAW mock-up with glossy
knobs, raised faders, glass panels) that hides what is not needed.

## Decisions (from the user)

- Layout: **mode view** (one mode at a time, full area).
- 3D look: **CSS/SVG only**, no WebGL, no library, no external assets
  (guard `TestShippedWebAssetsLoadNothingExternal`).
- Colours: **theme tokens**, dark teal + orange accent as the Standard default;
  the 14 desktop themes and accent colour keep working.

## Scope

In: new shell, five modes, drawers, control components (knob, fader, toggle),
token/CSS rework, migration of the sound-design (designer) and arrangement
surfaces, Strudel and Mixer/Export placement, tests, docs, 16 locales.
Out: audio engine, project model/format (`.aurasynth`), storage, Strudel engine,
MIDI/export logic. These keep their current behaviour and APIs.

## 1. Shell and modes

- Top bar: transport (play/stop/record/loop), BPM, time signature, master knob,
  level meter. The file actions stay compact icon buttons (no header menu); the tour and rarely used
  options go into a "More" menu.
- Mode switcher: **Arrange · Sound Design · Perform · Strudel · Mixer** (Export stays in the header).
  Each mode uses the full area. Only that mode's content is mounted visible.
- Library is a slide-out drawer (default closed) in Arrange and Sound Design.
- Persisted per browser (localStorage, try/catch, UI works without it): active
  mode, drawer states, lane height. Collapsed sections are kept only while the editor stays open, not in localStorage (key `aurago:synth-studio:view:2`).
- Replaces `synth-studio-workspace.js` layout/splitter logic; `render()` in
  `synth-studio.js` mounts modes instead of fixed columns.

## 2. Intelligent hiding

- Note editor (piano roll / drum grid) appears only when a clip is selected and
  slides open under the arrangement; closes on deselect.
- Sound Design sections (Oscillators, Filter, Envelope, LFO, Modulation, Effects,
  EQ) are collapsible cards. Default open = sections active for the selected
  sound; inactive ones collapsed to a one-line header with an enable switch.
- Rare actions (tour, import, clear data) live in the More menu.
- Empty states are explicit (e.g. "Select a track to design its sound").

## 3. 3D control components

New file `synth-studio-controls.js` (+ CSS in `desktop-app-synth-studio.css`, or a
split `desktop-app-synth-studio-controls.css`):

- **Knob**: SVG arc ring with glow for value, glossy cap with highlight and
  shadow, value label and unit below. Drag (vertical), wheel, arrow keys,
  Shift = fine, double-click = reset to default. `role="slider"`, `aria-valuenow`,
  focus ring. Touch via pointer events.
- **Fader**: raised thumb on an inset groove with filled track; same input
  methods; vertical and horizontal variants.
- **Toggle/power**: pill with glow LED.
- Glass panels: translucent surface, light top edge, soft drop shadow.
- Respect `prefers-reduced-motion`; no animation required for function.
- All colours via `--ds-*`/studio tokens (existing token spectrum from commit
  0fa1acb33); contrast checked by the existing token-contrast test.
- Controls are plain DOM/SVG factories with one interface
  (`create({min,max,step,value,label,unit,onInput,onChange})` → element +
  `setValue`/`destroy`), independent of the audio code, so they can be tested alone.

## 4. Migration order

1. Controls + tokens (no behaviour change; unit/browser test of the controls).
2. Shell with mode bar and drawers; existing panels mounted into modes unchanged.
3. Sound Design mode on knobs/faders (designer, FX, envelope, LFO, filter).
4. Arrange mode (tracks, clips, conditional note editor, mixer strips as faders).
5. Strudel and Mixer & Export modes; polish, responsive widths.
Each step is shippable and keeps the project format and exports unchanged.

## 5. Risks

- Browser tests (`ui/desktop_synth_studio_browser_test.go`) depend on current
  selectors/range inputs → update alongside; keep `data-action` hooks stable where
  possible.
- Very dense source (`synth-studio.js` long lines); split when touched.
- Keyboard/a11y parity: every knob/fader must remain operable by keyboard.
- Windows animation setting affects headless `prefers-reduced-motion` in tests.

## 6. Testing and docs

- Controls: pointer, wheel, keyboard, reset, ARIA, bounds/step clamping.
- Browser: mode switching, persisted state, clip-select opens note editor,
  collapsed-section defaults, playback still starts/stops, Strudel stop rules.
- Screenshot check per mode (desktop width + narrow width), all themes smoke.
- Update `documentation/synth-studio.md`; add new strings to all 16
  `ui/lang/desktop/*.json`; run the external-asset guard.
