# AuraPinball runtime

## Purpose

Three local 2.5D pinball tables: a custom deterministic physics engine, a rules framework with
per-table rule packs, procedural three.js tables, insert lamps, a 192×48 DMD, synthesized sound and
music, and a per-window Desktop game lifecycle.

## Ownership

- The sibling `pinball.js` owns `window.PinballApp` and lazy loading. This directory owns physics,
  rules, tables, lights, DMD, audio, render, input and views.
- `assets/pinball/AGENTS.md` owns the retained ImageGen masters and prompts. Payloads are under
  `ui/3d/pinball/v1/` and follow `ui/3d/pinball/AGENTS.md`.
- Registration is in `internal/desktop/types.go` and the Desktop loader/router. `pinball.*` keys
  belong to all sixteen Desktop locale bundles.

## Module layout

- DOM-free: `config.js`, `contracts.js`, `session.js`, `presenter.js`, `physics/*`, `rules/*`, `lights/*`,
  `dmd/core.js`, `dmd/scenes-*.js`, `dmd/font.js`, `dmd/raster.js`, `audio/songs/*`,
  `audio/samples.js`, `audio/recipes.js`, `audio/music.js`, `tables/index.js`,
  `tables/validate.js`, `tables/kit/*`, and each table's `layout.js`, `lamps.js` and `rules.js`.
- DOM or WebGL: `game.js` (mount and frame loop), `input.js`, `audio/engine.js`, `audio/sfx.js`, `audio/voice.js`, `audio/instruments.js`,
  `dmd/view.js`, `render/*`, `ui/*`, and each table's `art.js`, `toys.js` and `tables/visuals.js`.
- `physics/` is the 480 Hz engine. `world.js` steps and owns the event queue. `devices*.js` and
  `paths.js` implement the device and ramp systems behind the contracts in `contracts.js`.
- `rules/core.js` is the lifecycle, `rules/features.js` the per-event pipeline (points, bonus, lanes,
  skill, combos), `rules/modes.js` the modes, locks, multiball and wizard, plus `rules/lamps.js` and
  `rules/highscore.js`.
- `tables/<id>/` contains `layout.js` (geometry, devices, paths, shots, lamps), `rules.js` (the rule
  pack), `lamps.js`, `art.js` and `toys.js`. `tables/kit/` holds the shared shell and lower playfield.

## Local Contracts

- Pure three.js-free simulation: physics, rules, lights, shows, DMD scenes, music expansion and
  session must import under Node 20 with no `window`/`document` access at load.
- Determinism: no `Math.random`, `Date.now` or `performance.now` in physics, rules, lights, DMD
  scenes or music expansion. Use `rngNext(holder)` (mulberry32 on `holder.seed`, a JSON-serialisable
  uint32). Game state is plain JSON. The same seed and inputs must produce identical `copy(game)`
  and snapshots at 30/60/144 Hz.
- Fixed physics at 480 Hz (`STEP = 1/480`), SI units (m, s, rad), 27 mm / 80 g balls on a playfield
  tilted 6.5 degrees. Render interpolation never changes rules or contacts.
- A frame advances at most `dt = 0.05 s` and 32 ticks. It never pauses. Pause only for focus loss,
  a hidden window, a disconnected controller or a menu.
- Keyboard and gamepad edges are timestamped and consumed at simulation ticks, so short taps between
  frames are kept. Menu Space/Enter activation stays native.
- Tilt disables coils, GI, music and bonus. It never disables passive collision.
- Balls returned to the shooter retry the plunger. Each resting ball is searched independently. A
  bounded search failure replaces the ball without consuming the player's turn.
- One to four alternating players have independent lock, mode, lane and score state. Training uses
  the same physics and never writes highscores.
- Persist only validated settings and ten highscores per table (`config.js` keys). Never persist
  active physics worlds.
- Table layouts pass `tables/validate.js` (unique ids, resolved references, at most 200 lamps and six
  flashers, clearance) and the reachability and soak tests. Layouts reference only `pinball.*` keys.
- Keep every first-party file under `ui/js/desktop/apps/pinball/` at or below 700 lines (repo hard
  limit 1100 in `ui/desktop_js_line_budget_test.go`). Split by responsibility rather than trimming.
- Cameras: `follow` (ball cam, default, `render/follow.js` is the pure target/zoom/spring maths), `play`,
  `top`, `showcase` and `attract`. Reduced motion shows the static play framing instead of chasing the
  ball. Saved `camera: 'play'` without `cameraRev` migrates to `follow` once (`config.js`).
- Use `pb-*` classes, semantic menus, remappable controls, reduced-motion support and `FONT_STACK`
  from `config.js` for canvas text. No remote fonts, scripts or images.

## Locale rules

- Visible copy uses `pinball.*` keys in all sixteen locales (cs da de el en es fr hi it ja nl no pl
  pt sv zh). Write UTF-8 characters, never `\u` escapes.
- Dynamic lookups use fixed prefixes: `dmd_` (DMD scenes, callouts, table events), `mode_` (mode and
  multiball names), `lamp_` (lamp labels), `help_`/`card_` (lobby and help copy). Proper names,
  letters (N-E-O-N, S-U-N) and digits stay literal.
- Register new translation helpers in `ui/i18n_lint_test.go` (`js/desktop/apps/pinball/...` entries
  with `prefix: "pinball."`).
- Remove a key only after grepping `ui/js/desktop/apps/pinball/**`, `ui/js/desktop/apps/pinball.js`,
  the Go tests and the scripts for the bare name, the prefixed name and dynamic `prefix + name`
  patterns.

## Work Guidance

- Scene snapshots and rule state are read-only observations for the presenter and views. Commands go
  through `take(game)` or `world.command`, never through direct mutation.
- Every new rule, device or table feature needs a scripted test in the matching `scripts/test-pinball-*.mjs`
  suite. Keep each suite lean: the behaviour, plus one test per real bug fixed.
- Build the runtime and shell bundles together after registration or export edits:
  `npm run build:pinball`.

## Verification

- `npm run test:pinball` (suites: config, rules, input, physics, devices, paths, tables, reach,
  layout-*, pack-*, lights, dmd, audio, music, session, assets)
- `node scripts/build-pinball.mjs --check`
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopPinballBrowser -count=1 -timeout 20m`
- `go test ./ui -run "TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestFrontend_StaticI18nKeysExistInEnglishBundle|TestDesktopDynamicI18nPrefixKeysExistInAllLanguages" -count=1`
- Hardware FPS and human flipper/catch acceptance are separate from software-rendered browser tests.

## Child DOX Index

None.
