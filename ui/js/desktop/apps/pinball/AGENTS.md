# AuraPinball runtime

## Purpose

Four local 2.5D pinball tables (Neon, Leviathan, Temple and the Clockwork Observatory): a custom deterministic physics engine with ball spin, a rules framework with
per-table rule packs, procedural three.js tables, insert lamps, a 192×48 DMD, synthesized sound and
music, and a per-window Desktop game lifecycle.

## Ownership

- The sibling `pinball.js` owns `window.PinballApp` and lazy loading. This directory owns physics,
  rules, tables, lights, DMD, audio, render, input and views.
- `assets/pinball/AGENTS.md` owns the retained art masters, prompts and the observatory GLB build
  script. Payloads are under `ui/3d/pinball/v1/` and follow `ui/3d/pinball/AGENTS.md`.
- Registration is in `internal/desktop/types.go` and the Desktop loader/router. `pinball.*` keys
  belong to all sixteen Desktop locale bundles.

## Module layout

- DOM-free: `config.js`, `contracts.js`, `session.js`, `presenter.js`, `physics/*`, `rules/*`, `lights/*`,
  `dmd/core.js`, `dmd/scenes-*.js`, `dmd/font.js`, `dmd/raster.js`, `audio/songs/*`,
  `audio/samples.js`, `audio/recipes.js`, `audio/music.js`, `tables/index.js`,
  `tables/validate.js`, `tables/validate-paths.js`, `tables/kit/*` (including `cabinet-dims.js` and
  `plastics.js`), and each table's `layout.js`, `lamps.js` and `rules.js`.
- DOM or WebGL: `game.js` (mount and frame loop), `input.js`, `audio/engine.js`, `audio/sfx.js`, `audio/voice.js`, `audio/instruments.js`,
  `dmd/view.js`, `render/*`, `ui/*`, and each table's `art.js`, `toys.js` and `tables/visuals.js`.
- `physics/` is the 480 Hz engine. `world.js` steps and owns the event queue. `spin.js` owns ball
  spin and its friction. `devices*.js` and `paths.js` implement the device and ramp systems behind
  the contracts in `contracts.js`; `devices-motion.js` holds the observatory's `orrery` and `pendulum`.
- `rules/core.js` is the lifecycle, `rules/features.js` the per-event pipeline (points, bonus, lanes,
  skill, combos), `rules/modes.js` the modes, locks, multiball and wizard, `rules/surprises.js` the
  deterministic surprise awards and per-table secret combos, plus `rules/lamps.js` and
  `rules/highscore.js`.
- `render/fx.js` owns pooled 3D ball speed trails, playfield bounce halos, impact spark bursts and
  expanding shockwave rings, wired through `scene.hit` and `scene.celebrate` and gated by reduced motion.
- `tables/<id>/` contains `layout.js` (geometry, devices, paths, shots, lamps), `rules.js` (the rule
  pack), `lamps.js`, `art.js`, `art-accents.js` (lit accents over a painted playfield) and `toys.js`.
  `tables/kit/` holds the shared shell, lower playfield, cabinet dimensions and printed plastics sheet.
- `render/` builds the scene: `cabinet.js`, `apron.js` and `room.js` (wedge cabinet, apron, room),
  `ramps.js` with `ramp-builders.js` and `clearance.js` (habitrails, clear ramps, standoffs, signs and the
  glass audit), `hardware.js` and `plastics.js`, `reflect.js` (playfield reflection), `models.js` (GLB
  loading) and `builders-motion.js` (the procedural orrery disc and pendulum, following the snapshot anim slots).

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
- Keyboard presses are deduplicated per physical `event.code`, so a repeat-flagged second modifier
  still counts as a fresh press. A keyup whose modifier family reports inactive (`getModifierState`,
  with `shiftKey`/`ctrlKey`/`altKey`/`metaKey` as fallback) releases every action bound to that
  family: Windows unifies a left/right pair and can drop one keyup when both are released together.
  Once the edge queue drains, `held` is authoritative for the coils; the Start button stays edge-only.
- Tilt disables coils, GI, music and bonus. It never disables passive collision.
- Balls returned to the shooter retry the plunger. Each resting ball is searched independently. A
  bounded search failure replaces the ball without consuming the player's turn.
- One to four alternating players have independent lock, mode, lane and score state. Training uses
  the same physics and never writes highscores.
- Persist only validated settings and ten highscores per table (`config.js` keys). Never persist
  active physics worlds.
- Table layouts pass `tables/validate.js` (unique ids, resolved references, at most 200 lamps and six
  flashers, clearance) and the reachability and soak tests. Layouts reference only `pinball.*` keys.
- Cabinet dimensions come only from `tables/kit/cabinet-dims.js` (`CABINET`, `glassHeight(y)`, `topAt(y)`,
  `pathHalfWidth`, `pathTop`, `maxPathHeight`, the margins). Render and validation import it; do not
  hard-code a glass height. The underside of the glass is 0.065 m above the playfield at the lockdown bar
  and 0.14 m at the back wall. A path def may set `top` to override its kind's top height.
- Path fit (`tables/validate-paths.js`, called from `validate.js`): on every sample of every ramp, wire
  and tube, the path top plus `GLASS_MARGIN` (8 mm) stays under the glass (`path-glass`); the footprint
  stays inside the outer wall with `OUTLINE_MARGIN` (`path-outline`); two paths that overlap in plan
  are at least `CROSS_SEP` (37 mm) apart vertically (`path-cross`). A crossing is exempt within 0.07 m of
  arc length of a shared endpoint (chained paths and fan-outs). When a layout fails, lower the path
  by a few millimetres and keep its end points, so return feeds still reach their inlanes.
- Ball spin: balls carry `wx, wy, wz` (rad/s, playfield frame: x right, y up the table, z out of the
  playfield). Rolling on a static floor means `vx = R*wy` and `vy = -R*wx`. `physics/spin.js` owns floor
  and wall friction; `SPIN` in `physics/constants.js` holds the constants. Snapshot ball entries carry
  the three fields and `lerpSnapshot` interpolates them. Launches, holes and magnet releases clear spin.
  Keep spin.js allocation-free and deterministic. The ball mesh turns by the integrated spin each frame
  (render axes ω = (wx, wz, −wy)). Without spin fields it falls back to the displacement rule, and it does
  not turn on teleports above 0.05 m.
- Moving devices (`physics/devices-motion.js`): `orrery` is a flush turntable that registers a moving
  floor (`ctx.addFloor`), so a ball over it is dragged round. `pendulum` is a bob on a rod in the vertical
  plane that strikes balls as it swings. Commands: `{type: 'orrery', id, speed}` (rad/s, clamped to
  `maxSpeed`) and `{type: 'pendulum', id, mode: 'park' | 'swing' | 'fast'}`. Events, both scoring:
  `orrery {dir: 1}` once per ball that enters the disc, and `pendulum {speed}` when the bob strikes a ball
  (approach speed in m/s, at most once per 0.12 s). Both are in `EVENT_TYPES` and `PHYSICS_COMMANDS`
  in `contracts.js`. The disc angle and the pendulum angle are anim slots; the disc slot is listed in the
  snapshot's `spin` array.
- Rule packs may call `ctx.startMultiball()` (`rules/modes.js`). It starts the pack's `multiball` without a
  lock and returns `true`; it returns `false` when a multiball or the wizard is running or the pack has no
  `multiball`.
- Painted art: a manifest table with `artRev` 2 has a painted playfield. The runtime then paints only
  the functional accents (`art-accents.js`) and contact shadows over it (`kit.contactShadows()`). `artRev`
  1 keeps the procedural paint. `render/scene.js` passes `entry.artRev || 1`.
- Printed plastics: `tables/kit/plastics.js` is the DOM-free contract for the 1024×1536 sheet. `SHEET`
  holds the cell rectangles, copied from the plastics template (`node scripts/pinball-art-guide.mjs plastics
  --check` fails if they drift). `cellUV(id)` gives a cell's texture rectangle. `plasticPieces(layout)` returns
  the seven playfield pieces (`slingL`, `slingR`, `guideL`, `guideR`, `cornerL`, `cornerR`, `badge`) with
  their rectangle, height, flip and standoff posts. Bumper caps and ramp signs use the `cap`, `sign1` and
  `sign2` cells directly. `render/plastics.js` builds the alpha-tested quads and chrome standoffs. Without a
  sheet image the slings keep their own plates.
- GLB toys: `render/models.js` loads the GLB files that the manifest lists under `glb` (only `observatory`
  has one, key `toys`) with `GLTFLoader`, an 8-second timeout and an abort signal. A missing file or root is
  absent, and the toy falls back to its procedural builder. Each toy is one root node named after its kind
  (`orrery`, `pendulum`, `telescope`, `gears`, `dome`) whose origin is the toy's base point. The runtime
  clones the root, places it and turns the named children from device state. Those children are
  `orrery_arm_1` to `orrery_arm_4`, `orrery_rings`, `pendulum_arm`, `telescope_barrel`, `gear_1` to
  `gear_5`, `dome_shutter`, and the clock hands `clock_hand_hour` and `clock_hand_minute`, which sit on the clock
  face of the `dome` root. Materials named `glow_*` are emissive
  and not tone-mapped; `glass_*` materials are glass. The map has one key, `toys`, and `tables/observatory/toys.js`
  looks up roots by these names. The telescope barrel is aimed along the telescope tube path.
- Playfield reflection (`render/reflect.js`): `REFLECT_LAYER` is 3. `createPlayfieldReflection` renders the
  objects on that layer (balls, flasher domes, GI bulbs, bumper caps and cores, flipper tops and the lights)
  into a target. High uses half the canvas size, every frame, strength 0.22. Medium uses a quarter, every
  second frame, strength 0.15. Low has no target and no extra pass. The playfield shader patch cache key is
  `pf-lamp-v4`. The CPU fallback path skips the reflection. `stats().reflectCalls` reports its draws.
- Glass audit: `render/clearance.js` checks every built mesh vertex against `glassHeight(y)` minus 6 mm;
  the scene reports the offenders as `stats().glassViolations`. Every toy mesh stays under the glass with an
  8 mm margin, and the count is 0 on all four tables at high and low quality. Toys that stand in a back corner sit on the
  printed corner panel at `h` 0.043, with their top at or below `glassHeight(y) - 0.008`.
- Leviathan's ship scoop ejects past the sonar bank into the lower playfield without immediate
  recapture. Verify the release trajectory across seeded eject variation in `test-pinball-layout-leviathan.mjs`.
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
- `node scripts/test-pinball-tables.mjs` (includes the path-fit checks) and `node scripts/test-pinball-devices.mjs`
  (orrery and pendulum) for changes to layouts and moving devices.
- `TestDesktopPinballBrowser` covers all four tables and fails when `graphics.glassViolations` is not 0.
- Hardware FPS and human flipper/catch acceptance are separate from software-rendered browser tests.

## Child DOX Index

None.
