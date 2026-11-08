# AuraPinball rebuild — implementation plan (2026-11-08)

Goal: replace the Rapier/GLB runtime with a custom deterministic 2.5D engine, procedural three.js tables, insert lamps + light shows, a 192×48 DMD, synthesized SFX/music, and three rebuilt tables (Neon → Leviathan → Temple).

Milestones: **M1** Neon playable end-to-end (T01–T19; Leviathan/Temple load as bare shell+lower tables). **M2** Leviathan (T20–T22). **M3** Temple (T23–T25). **M4** browser test, docs, acceptance, landing (T26–T28).

---
## 0. Ground rules (every task)
1. Work only in worktree `.worktrees/pinball-rebuild`, branch `claude/pinball-rebuild` (already created from local main). Other sessions commit in the main checkout and keep `ui/lang/desktop/*.json` dirty.
2. Commit only your paths: `git commit -m "…" -- <paths>`. Never `git add -A`. Parallel tasks share the worktree.
3. Each first-party JS file under `ui/js/desktop` ≤ 700 lines (repo hard limit 1100, `ui/desktop_js_line_budget_test.go`). Split by responsibility.
4. No network resources. Canvas text uses `FONT_STACK` from `config.js`.
5. DOM-free modules (`physics/*`, `rules/*`, `tables/**` except `art.js`/`toys.js`/`visuals.js`, `lights/*`, `dmd/core|scenes|font|raster.js`, `audio/songs/*`, `session.js`, `config.js`, `contracts.js`) must import under Node 20 with no window/document access at load.
6. Visible copy uses `pinball.*` keys in all 16 locales (cs da de el en es fr hi it ja nl no pl pt sv zh). Proper names (table titles), letters (N-E-O-N, S-U-N) and digits stay literal.
7. Write locale strings as UTF-8 characters. Never use `\u` escapes, because the Write tool decodes them.
8. Lean tests: write only the tests listed in your task, plus one test per real bug you fix.
9. Determinism: no `Math.random`, `Date.now` or `performance.now` in physics, rules, shows, DMD scenes or music expansion. Use `rngNext(holder)` (mulberry32 on `holder.seed`, a uint32, so it is JSON-serializable).
10. CLAUDE.md: run GitNexus impact before editing existing exported symbols (`mountGame`, `createInput`, `normalizeSettings`, build scripts). If the index cannot see the worktree, say so in your report.
11. Units are SI (m, s, rad, m/s); angles in data are radians unless they are named `…Deg`.
12. three r186: there is no `PCFSoftShadowMap` (use `PCFShadowMap`), and `scene.environment` ignores per-material `envMapIntensity` (set `material.envMap` explicitly where intensity matters). Headless rod Chrome uses software GL, so pin `quality=low` and never assert FPS.

---
## 1. Coordinates, constants
- **Playfield plane:** x across, x∈[-0.2575, 0.2575]; y up the table, y∈[0, 1.07] (y=0 is the apron edge); h is height above the playfield. Ball r=0.0135 m, m=0.08 kg.
- **three.js:** `tableRoot` local X=x, Y=h, Z=−y, with `tableRoot.rotation.x = +6.5°`. The player looks from +Z, so screen-right = +x and no mirroring is needed.
- **Texture mapping:** u=(x+0.2575)/0.515 and v=y/1.07. The art canvas uses row 0 = top (`CanvasTexture.flipY=true`), so canvas pixel = (u·W, (1−v)·H). The lamp mask is a `DataTexture` with `flipY=false` and row 0 at v=0. One helper, `lights/shapes.js: toUV(x,y)`, is used by the mask, the art kit and the shader.
- **`physics/constants.js`:**
  - `STEP=1/480`, `BALL_R=0.0135`, `BALL_M=0.08`, `G=9.81`.
  - `G_SLOPE=9.81·sin6.5°=1.11046` (along −y on the field); `G_NORMAL=9.74695`.
  - `MAX_SPEED=12`, `SUBSTEP_TRAVEL=0.35·BALL_R`, substeps 2..32.
  - `ROLL_DECEL={field:0.055, plastic:0.16, wire:0.26, tube:0.30}` (m/s²).
  - `MATERIALS` (restitution `e = vn<0.03 ? 0 : clamp(e0−eSlope·vn, eMin, e0)`, friction `mu`):

| material | e0 | eSlope | eMin | mu |
|---|---|---|---|---|
| wood | 0.55 | 0.05 | 0.35 | 0.15 |
| metal | 0.45 | 0.04 | 0.30 | 0.10 |
| plastic | 0.50 | 0.05 | 0.30 | 0.20 |
| rubber | 0.88 | 0.07 | 0.55 | 0.60 |
| flipper | 0.82 | 0.03 | 0.60 | 0.80 |
| target | 0.45 | 0.04 | 0.30 | 0.20 |
| ball | 0.93 | 0 | 0.93 | 0.05 |

  - `FLIPPER` defaults: `length 0.059` (pivot to tip centre; about 3" overall), `rBase 0.012`, `rTip 0.006`, `rest −0.5236`, `up +0.4887`, `alphaUp 3000 rad/s²`, `omegaMax 50` (T02 tuned; range 38–50), `alphaDown 1200`, `omegaDownMax 26`, `stopRebound 0.15`, `restRebound 0.25`.
  - Refinement against the brief: ω 30–35 cannot reach a 5–8 m/s tip exit with a kinematic bat. Real bats peak at 40–60 rad/s.
- **`config.js`** exports:
  - `STORAGE` (keys `aurago.desktop.pinball.settings.v1` / `.scores.v1`), `TABLE_IDS`, `ASSET_ROOT='/3d/pinball/v1/'`.
  - `FONT_STACK='"Segoe UI","Noto Sans","Helvetica Neue",Arial,"Microsoft YaHei","PingFang SC","Hiragino Sans","Nirmala UI","Noto Sans Devanagari",sans-serif'`.
  - `clamp`, `finite`, `copy`, `rngNext`, `DEFAULT_KEYS`, `DEFAULT_PAD`, `normalizeSettings`, `normalizeScores` (behaviour unchanged).

---
## 2. File layout (`ui/js/desktop/apps/pinball/`), owner task, target lines
```
game.js T17 ≤550 (DOM orchestration; mountGame)   session.js T17 ≤450 (DOM-free core loop)
config.js T01   contracts.js T01   input.js T18 (extend)
physics/ constants.js T01 · collide.js T02 · grid.js T02 · flipper.js T02 · world.js T02 · snapshot.js T02
         devices.js T03 (bumper sling dropBank standup spinner rollover trigger gate door kickback diverter)
         devices-holes.js T03 (hole magnet captive search) · paths.js T04
rules/   core.js T06 (lifecycle, players, drains, save, tilt, EB, replay, match, training, dmdState)
         features.js T06 (points, ctx, lanes, bonusX, skill, kickback, combos, shot dispatch)
         modes.js T06 (modes, select, locks, multiball, jackpots, wizard) · lamps.js T06 · highscore.js T06
tables/  index.js T01 · validate.js T01 · visuals.js T12
         kit/ geom.js shell.js lower.js compose.js T01
         neon/ index.js T01 · layout.js lamps.js T14 · rules.js T15 · art.js toys.js T16
         leviathan/ (same) T20/T21/T22 · temple/ (same) T23/T24/T25
lights/  shapes.js mask.js controller.js shows.js T07
dmd/     core.js scenes-play.js scenes-events.js font.js raster.js view.js T08
audio/   engine.js sfx.js voice.js T09 · music.js instruments.js songs/{neon,leviathan,temple}.js T10 · index.js T17
render/  scene.js materials.js playfield.js art-kit.js lighting.js camera.js cabinet.js T12
         builders.js mechs.js ramps.js balls.js T13
ui/      views.js hud.js T18
```
Old files `physics.js rules.js scene.js audio.js views.js tables.js` stay until T17 deletes them.

Other paths:
- `scripts/`: `test-pinball.mjs` (runner), `test-pinball-*.mjs`, `lib/pinball-sim.mjs` (T02), `lib/pinball-shots.mjs` (T14), `pinball-dev-server.mjs` (T05).
- `ui/testdata/`: `pinball-harness.html`, `pinball-harness.js`, `pinball-debug2d.js` (T05), `pinball-bot.js` (T14).
- `ui/desktop_pinball_survey_browser_test.go` (T05).

---
## 3. Contracts

### 3.1 Table = `{ layout, rules }` (`tables/<id>/index.js` default export)
```js
layout = {
  id, version: 1,
  meta: { title, accent, secondary, cx: -0.017, troughSize: 4, giColor: {lower, upper},
          attract: [[showName, ms], ...] },
  flippers: [{ id, key: 'left'|'right', x, y, side: +1 /*points +x*/ | -1, ...FLIPPER overrides }],
  plunger: { x: 0.2405, yRest: 0.0535, travel: 0.06, pullRate: 1.6, vMin: 0.55, vMax: 3.4, vAuto: 3.0,
             laneX: [0.2235, 0.2575], zoneYMax: 0.12 },
  drainY: 0.075,
  walls:  [{ id, material, points: [[x, y], ...], closed: false, render: 'wood'|'metal'|'rubber'|'plastic'|'none',
             height: 0.03 }],
  posts:  [{ id, x, y, r: 0.005, material: 'rubber'|'metal', render: 'post'|'none' }],
  devices: [DeviceDef],        // §3.2
  paths:   [PathDef],          // §3.3
  shots:   [{ id, on: 'pattern', combo: true, bonus: 'ramps', setup: [physicsCommand] }],   // physical shot definitions
  lamps:   [LampDef], flashers: [FlasherDef], gi: [GIDef],
  toys:    [{ id, kind, x, y, h, rot, scale }],
  labels:  [{ key, x, y, size, angle, color }],
  allowGaps: [[idA, idB]],     // clearance whitelist
}
```
Event pattern string: `'<type>:<id|*>[@±1]'`, for example `'pathMade:leftRamp'`, `'trigger:leftOrbitUp@1'`, `'bumper:*'`.

### 3.2 DeviceDef (defaults in brackets)
- `bumper {x,y,r[0.0215],kick[2.0],lamp}`: on contact with coils on and cooldown 0.06 s, set v_n' = kick+0.3·vn and v_t ×0.9.
- `sling {face: wallId, kick[1.9], threshold[0.30]}`: attaches a kicker to the existing rubber wall shape.
- `dropBank {targets: [{id,x,y}], angle (normal pointing to the incoming side), width[0.024], threshold[0.25]}`.
- `standup {x,y,angle,width[0.020],threshold[0.15]}`.
- `spinner {a,b,gain[60],maxRate[360]}`:
  - No collider. On crossing, ω0 = min(maxRate, gain·|v·n|) and the ball loses 2 % speed.
  - Decay dω/dt = −(25+1.2ω).
  - Emits one `spinner` event per revolution.
- `rollover {x,y,r[0.008]}`. `trigger {a,b}`.
- `gate {a,b,pass:[nx,ny],mode['oneWay'|'open'|'closed']}`. `door {a,b,open[false],material['plastic']}`.
- `hole {kind:'saucer'|'scoop'|'vuk'|'kickhole', x,y, r[0.016], captureSpeed[saucer 0.9 | scoop/vuk 2.8 | kickhole 1.2], capacity[1], hidden[false], eject:{angleDeg,speed} | {path,speed}, autoEject[6], enabled[true]}`.
- `kickback {x,y,r[0.012],speed[3.6],armed[false]}`.
- `magnet {x,y,r[0.035],pull[60],damp[8],maxAccel[6]}`.
- `captive {channel:{a,b,width[0.034]}, rest:[x,y], endSwitch:id}`: auto-generates the channel walls, two mouth posts (r 0.003, gap 0.020) and an end trigger.
- `diverter {at:[x,y,h], on[false]}` (path-level).

### 3.3 PathDef
```js
{ id, kind: 'plastic'|'wire'|'tube', entry: { a, b }, points: [[x, y, h], ...] /* points[0] = entry midpoint, h 0 */,
  width: 0.05|0.034, minSpeed: 0, switches: [{ id, at: 0..1 }], color,
  exit: { kind: 'field', dir: [dx, dy], speedScale: 0.9 } | { kind: 'hole', hole } | { kind: 'path', path }
      | { kind: 'switch', diverter, on: Exit, off: Exit } }
```
Paths are sampled as centripetal Catmull–Rom, 200 samples each, with an arc-length table.

### 3.4 Physics events `{type, id, ball, tick, speed?, x?, y?, ...}`
- `launch {speed, auto}`, `drain`, `flipper {up}`
- `bumper`, `sling`, `drop {bank}`, `bankDown`, `bankReset`, `standup`, `spinner {rate}`
- `rollover {dir}`, `trigger {dir}`, `gate {dir}`
- `hole {via:'field'|'path', count}`, `eject`, `kickback`
- `magnet {state:'grab'|'held'|'release'|'throw'}`, `captive` (ball = the striking ball)
- `pathEnter`, `pathSwitch {path}`, `pathMade {exit}`, `pathFail`
- sound-only: `impact {material}` (vn > 0.15, throttled to one per ball per 20 ms), `clack`
- `search {cycle}`, `searchFail`, `shooter` (ball served).

`contracts.js` exports:
- `EVENT_TYPES`, `SCORING_EVENTS` (excludes impact, clack, flipper, launch, search, shooter, eject, pathEnter, magnet).
- `PHYSICS_COMMANDS`, `PRESENTATION_COMMANDS`, `LAMP_MODES`, `CUES`, `MUSIC_SECTIONS`, `SHOWS`.
- `parsePattern(p)`, `matchEvent(pattern, ev)`.

### 3.5 Commands
**Physics:**
- `addBall {id, auto}`, `coils {on}`
- `eject {id, count: 1|'all', delay}`, `lockBall {id}`, `releaseToTrough {id}`, `holeEnable {id, on}`
- `resetBank {id}`, `dropBank {id}`, `diverter {id, on}`, `gate {id, mode}`, `door {id, open}`
- `kickback {id, armed}`, `magnet {id, mode: 'off'|'grab'|'release'|'throw', angle, speed}`
- `kick {id}`, `nudge {dx, dy}` (ball Δv in m/s), `removeBall {id}`, `clearBalls`.

**Presentation:**
- `dmd {scene, params, ms, priority 0–9, transition}`, `dmdBase {scene, params}`
- `show {name, ms, priority, x, y, color, replace}`, `flash {id|'all', ms}`
- `gi {id|'all', mode: 'on'|'off'|'dim'|'flicker', ms}`
- `cue {name}`, `music {section}`, `say {key}`, `shake {strength}`, `lampColor {id, color}`.

DMD priorities: base 0; info 3; award 5; big 7; tilt 9.

### 3.6 Snapshot (`physics/snapshot.js`)
```js
{ tick, time, balls: [{ id, x, y, h, vx, vy, mode: 'field'|'path'|'held'|'shooter'|'captive', hidden }],
  flippers: Float64Array(2n) /* [angle, omega] per layout order */, plunger, anim: Float32Array }
```
- `buildAnimIndex(layout) → Map<id, i>`: one slot per device, plus one per drop target (keyed by target id).
- Anim scalars:
  - bumper fire 1→0 over 60 ms; sling 1→0 over 50 ms; drop target 0 (up)..1 (down); standup wobble 1→0 over 120 ms
  - spinner angle (rad); rollover 1→0 over 100 ms; gate swing 0..1.2 rad; door 0..1 (eased over 0.4 s)
  - hole kick 1→0; diverter 0..1; kickback 1→0; magnet 0..1.
- `lerpSnapshot(a, b, t, out)` matches balls by id and interpolates spinner angles by the shortest delta.

### 3.7 World API (`physics/world.js`)
**Public:**
- `createWorld(layout, {seed=1}) → world`
- `world.step({left, right, plunger: 0..1})` advances one STEP.
- `world.events()` drains the event queue (in emit order).
- `world.command(cmd)`, `world.snapshot(out?)`, `world.reset()` (clears balls, raises banks, closes doors, diverters off, returns held balls to the trough)
- `world.placeBall(id, x, y, vx, vy)`, `world.stats() → {balls, held, pending, tick, substeps, searchCount}`
- `world.debug() → {segments, circles, flippers, devices, paths}`, `world.dispose()`
- `world.animIndex`, `world.trough`

**Internal `ctx` handed to `createDevices(defs, ctx)` / `createPaths(defs, ctx)`:**
- Fields: `layout`, `rng {seed}`, `tick` (getter), `coils` (getter), `balls: Map<id, Ball>`.
- `shapes: {add, byId, setActive, all}`.
- `emit(type, id, ball, extra)`, `addBallAt(id, x, y, vx, vy, mode)`, `removeBall(id)`, `returnToTrough(id)`.
- `Ball = {id, x, y, vx, vy, px, py, h, mode, hidden, captive, holder, path: {id, s, v} | null, lastSwitch}`.
- `Shape = {id, kind: 'seg'|'circle', ax, ay, bx, by | cx, cy, r, material, active, oneWay, owner, onHit(ball, {nx, ny, vn, px, py})}`.

**System interfaces:**
- `DeviceSystem = {ids, preTick(), substep(h), postTick(), command(cmd) → bool, search(cycle) → bool, reset(), writeAnim(anim), debug()}`.
- `PathSystem = {substep(h), command(cmd) → bool, reset(), position(ball) → [x, y, h], debug()}`.
- T02 ships stub `devices.js` and `paths.js` with exactly these exports. T03 and T04 replace them.

### 3.8 Rules API (`rules/core.js`)
**Functions:**
- `createGame(table, {players: [names], training, seed, scores: top10})`, `startGame(game)`
- `tick(game, dt)`, `handle(game, ev)`, `button(game, name: 'left'|'right'|'start'|'launch', down)`
- `nudge(game, strength) → allowed`, `take(game) → Command[]`
- `lampModes(game, out: Uint8Array)`, `dmdState(game)`, `toyState(game)`
- `resetTraining(game)`, `debugBalls(game, ids)` (testing).

**`game` state** (plain JSON):
- `{v, table, seed, training, phase: 'ballStart'|'play'|'bonus'|'match'|'initials'|'over', time, players, turn}`
- `balls: {active, nextId, locked}`, `ballSave: {armed, until, unlimited}`, `tiltEnergy`, `nextWarning`
- `skill: {lane, live, until, superUntil}`, `combo: {last, until, count}`
- `mb: {state: 'off'|'running'|'wizard', jackpots, value, litShots, superUntil, addABallUsed}`
- `mode: {id, until, count, target}`, `select`, `bonusCount`, `match`, `initials`, `results`, `queue`.

**Player:**
- `{name, score, ball, extraBalls, ebLit, bonusX, counters: {}, lanesTop: [], lanesLower: [], kickbackLit}`
- `{lockLit, locks, modeReady, modesPlayed: [], modesCompleted: [], wizardReady, replayAwarded, tilt, warnings, table: pack.init()}`.

**Framework numbers:**
- 3 balls per player.
- Ball save 8 s plus 1.5 s grace. The timer starts at the first non-shooter switch. MB save 10 s, wizard 20 s.
- Skill: live until the first switch other than top-lane, bumper, shooter or gate, or 10 s after launch. Super skill is awarded on the pack's super pattern within its window.
- Combo window 3.0 s. Award `base·(chain−1)` for chain ≥ 2.
- Mode select: 4 s, flippers cycle, Start or Launch picks. Default hole hold 0.9 s (1.8 s when a priority ≥ 5 DMD scene started this tick).
- Bonus count: 0.45 s per counter line, 0.6 s for the ×-line, 1.0 s for the total. Holding both flippers skips the count.
- Tilt: each nudge adds 0.9·strength; the energy decays 0.7/s. A warning fires above 1.5 (at least 1 s apart); the 3rd warning tilts. Tilt sends `coils` off, GI off, music off, and zeroes the bonus.
- Match: `floor(rng·10)·10` compared with `score % 100`. Initials timeout 45 s; alphabet `A–Z 0–9 space ←`.
- All point values are multiples of 10.
- Event flow:
  1. If tilted, only drains count.
  2. Points (pack patterns, default 1,010 for `SCORING_EVENTS`).
  3. Bonus counters.
  4. Lanes.
  5. Skill.
  6. `pack.hooks.onEvent`.
  7. Shots → combos, mode progress, jackpots, lock, extra ball, mode start/wizard, then `onShot`.
  8. Replay check.

### 3.9 Rule pack (`tables/<id>/rules.js` default export)
```js
{ points: { 'bumper:*': 3010, ..., default: 1010 }, shotPoints: { shotId: n }, bonusFrom: { pattern: counterId },
  bonusCounters: [{ id, key: 'dmd_bonus_ramps', value }],
  topLanes: { switches, lamps, award: 'bonusX' } | null,
  lowerLanes: { switches, lamps, award: 'kickback'|'extraBall' },
  bonusX: { lamps: ['bonus2x', 'bonus3x', 'bonus4x', 'bonus5x'], max: 5 },
  skill: { value: 250000, super: { on, window: 6, value: 1000000 } },
  ballSave: { seconds: 8, lamp: 'shootAgain' },
  kickback: { device, lamp, litAtStart: true } | null,
  extraBall: { shot, lamp: 'extraBall' },
  lock: { hole | shot, physical: bool, lamp: 'lockLit', diverter: { id, shot } | null },
  multiball: { name, balls: 3, ballSave: 10, jackpotShots, jackpotBase: 1e6, jackpotStep: 250000, superAfter: 4,
               superShot, superWindow: 20, addABallShot },
  modeStart: { shot, hole, lamp: 'modeStart' },
  modes: [{ id, name, hint, kind: 'count'|'roving'|'sequence', targets: [shotId|pattern], count, duration,
            moveEvery: 4, shotValue, step: 50000, completeValue, show, gi }],   // exactly 4
  wizard: { id, name, balls: 4, ballSave: 20, jackpotValue: 5e6, step: 1e6, superShot, superAfter: 6 },
  combos: { window: 3.0, base: 100000 }, replay: 40000000,
  hooks: { init(), onEvent(ctx, ev), onShot(ctx, shotId), lamps(ctx, set), toy(game) } }
```
**`ctx`:**
- `{game, player, now, score(n, {bonus}), bonus(id, n), cmd(c), dmd(scene, params, ms, prio), show, flash, gi, cue, say, shake}`
- `{lightLock(), lightExtraBall(), lightKickback(), lightModeStart(), advanceBonusX(), modeActive(), mbActive(), rng(), table}`.

**Lamp conventions** (derived in `rules/lamps.js`; required by the validator):
- `shootAgain`: slow during save, fast in its last 2 s, on with EB pending.
- `bonus2x..5x`, `extraBall`, `lockLit`, `lock1..3`, `modeStart` (slow when ready / fast when wizard ready).
- `mode_<id>`: off = unplayed, pulse = played, on = completed, fast = running, slow = select candidate.
- `wizard`, `superJackpot`, `kickbackL|R`, lane lamps.
- `arrow_<shot>` for every shot; `jp_<shot>` for every jackpot shot; `combo_<shot>` for every combo shot.
- Arrow priority: wizard/MB jackpot (fast) > mode target (slow; roving fast) > lock (on) > extra ball (pulse) > mode start (slow) > combo (pulse) > off.

**Lamp mode codes:** 0 off, 1 on, 2 slow (2 Hz), 3 fast (6 Hz), 4 pulse (1.25 Hz sine, min 0.15), 5 flicker (seeded), 6 dim (0.3).

### 3.10 Lamps, flashers, GI
- `LampDef {id, x, y, shape: 'circle'|'oval'|'triangle'|'arrow'|'rect'|'roundRect'|'diamond'|'star'|'shield'|'chevron', w, h, angle (0 = pointing +y), color, label?: i18n key, text?: literal, mount: 'insert'|'bumper'|'toy'|'backbox', device?, group?, rgb?}`.
- Maximum 200 lamps. Lamp index = array index + 1.
- `FlasherDef {id, x, y, h: 0.05, color, r: 0.012}`. At most 6.
- `GIDef {id: 'lower'|'upper', color, yMin, yMax, bulbs: [[x, y]]}`.
- `lights/shapes.js`: `lampPolygon(lamp, seg=24)` (metres), `toUV`, `texelOf(x, y, w, h)`, `canvasPx(x, y, W, H)`.
- `lights/mask.js`: `buildLampMask(layout, {width: 1024, height: 2048}) → {width, height, data: Uint8Array RGBA}`:
  - R = insert lamp index.
  - G = halo lamp index; B = halo strength `255·(1−d/0.006)²` (6 mm halo outside the insert).
  - A = 255.
- `lights/controller.js`: `createLamps(layout) → {setModes(Uint8Array), setColor(id, hex), show(name, opts), stopShows(), flash(id, ms), gi(id, mode, ms), update(dt), rgba: Uint8Array(1024), intensity: Float32Array(n), flash: Float32Array, giLevel: Float32Array, inspect() → {count, lit, shows}}`.
  - Incandescent smoothing: τ rise 0.022 s, τ fall 0.065 s.
  - Shows combine as max(base, show), or replace the base when `replace: true`. A higher-priority show wins.
  - texel 0 stays black.
- `lights/shows.js`: `SHOWS[name](lamp, t, params) → 0..1` for:
  - `sweepUp`, `sweepDown`, `sweepLeft`, `sweepRight`, `radial`, `ring`, `chase` (by group order)
  - `strobe` (12 Hz), `sparkle` (seeded), `spiral`, `breath`, `wipeDiagonal`, `rainbow` (hue for rgb lamps), `blackout`.

### 3.11 DMD
- `dmd/core.js`: `createDmd({raster, tr, locale}) → {width: 192, height: 48, frame: Uint8Array(9216) levels 0..3, version, setBase(scene, params), play(scene, params, {ms, priority, transition: 'cut'|'wipeLeft'|'wipeRight'|'wipeUp'|'dissolve'|'zoom'}), update(dt, state) → changed, clear(), inspect() → {base, scene, queue, text}}`.
  - Queue max 6. Higher priority interrupts; equal priority queues FIFO.
- **Draw API** passed to scenes:
  - `clear`, `text(strOrKey, x, y, {size: 's'|'m'|'l'|'xl', align, level, maxWidth, vars})`, `number(n, x, y, {font: 'l'|'m'|'s', align, level})`
  - `rect`, `line`, `sprite(points, x, y, scale, rot, level)`, `sparkle(n, seed)`, `progress`, `invert`.
- Text renders through `raster.js` (canvas `fillText`, FONT_STACK bold, sizes s 9, m 13, l 20, xl 30 px; LRU cache of 64).
  - Quantize alpha: <0.2 → 0, <0.45 → 1, <0.7 → 2, else 3.
  - `needsLargeGlyphs(str)` (CJK/Hangul/Devanagari) bumps the size to at least 13 px.
  - Text that does not fit shrinks, then scrolls as a marquee.
  - Node fallback (no canvas): a deterministic block of width 0.6·size·len.
- Digits use bitmap fonts in `font.js`: l 10×16, m 6×10, s 4×6 (0–9 `,` `.` space).
- **Scenes:**
  - Base scenes: `score` (active player big; others small on the bottom row; `BALL n` / `TRAINING`), `modeStatus` (name, timer, progress, score), `attract` (pages every 4 s: title, high scores, `PRESS START`, `REPLAY AT`, last game).
  - Event scenes: `big` (generic zoom word + sparkles), `jackpot` (count-up), `superJackpot`, `lock`, `multiball`, `modeStart`, `modeComplete`, `modeTotal`, `select`, `skillShot`, `ballSaved`, `extraBall`, `replay`, `combo`, `tilt`, `tiltWarning`, `bonus`, `match`, `initials`, `gameOver`, `shootAgain`, `wizardStart`.
- `dmd/view.js` (DOM): `createDmdView(parent) → {draw(frame, version), dispose()}`.
  - Scale = floor(cssWidth·dpr/192) ≥ 2. ImageData → `drawImage` without smoothing, then a dot mask via `destination-in`, then unlit dots `#2b1606` via `destination-over`.
  - Amber `#ff8c1a`, levels [0, 0.35, 0.65, 1]. CSS drop-shadow glow, disabled when `reduced`.

### 3.12 Audio
`audio/index.js` (T17): `createAudio({tableId, settings, assetURL, locale, tr}) → {...}`:
- Lifecycle: `unlock() → Promise<bool>`, `ready()`, `resume()`, `pause()`, `setSettings(s)`, `dispose()`.
- Sound: `event(ev)`, `cue(name)`, `music(section)`, `roll(balls: [{speed, mode, pathKind, x}])`, `magnet(on)`, `say(key)`.
- `inspect() → {unlocked, state, voices, buffers, paused, music}`.

`CUES`: jackpot, superJackpot, lock, lockLit, multiball, extraBall, replay, modeStart, modeComplete, skillShot, ballSaved, drain, gameOver, highScore, tiltWarning, tilt, combo, bonusTick, bonusTotal, match, knocker, startGame, shootAgain, wizard.

`MUSIC_SECTIONS`: off, attract, main, mode, multiball, wizard, bonus, hiscore.

### 3.13 Renderer (`render/scene.js`)
- `createScene(host, {tableId, layout, settings, assetURL, signal, tr}) → Promise<scene>`.
  - It loads `manifest.json` (accepts v1 `{playfield}` or v2 `{underlay}`), the backglass webp and the underlay webp.
  - A missing manifest throws `load_error`; a WebGL failure throws `webgl_error`.
- Methods: `update({snap, lamps, dmd: {frame, version}, toyState, dt, time, paused, reduced, interactive})`, `setQuality(q)`, `setCamera('play'|'top'|'showcase'|'attract')`, `setInsets({top: px})`, `shake(dx, dy, strength)`, `hit(ev)`, `dispose()`.
- `stats() → {ready, quality, frames, drawCalls, triangles, textures, geometries, width, height, flipperOrder, renderer}`.
- `tables/visuals.js`: `VISUALS[id] = {paintArt(kit) | null, buildToys(THREE, kit, layout) → {group, update(dt, t, snap, toyState, lamps), dispose} | null}`.

### 3.14 Session, game, view
- `session.js`: `createSession(table, {seed}) → {world, game, startGame({players, training, seed, scores}), endGame(), tick(controls /* {left, right, plunger, start} */), nudge(dx, dy) → allowed, onEvent(fn), onPresentation(fn), lampModes(out), dmdState(), toyState(), snapshotPair(prev, curr), resetTraining(), stress(n), dispose()}`.
  - `tick` turns edges into `rules.button`, runs `world.step`, sends events to `rules.handle` and listeners, runs `rules.tick`, then routes commands (physics → `world.command`; the rest → `onPresentation`).
- `game.js`: `mountGame(host, {t, assetURL, isActive, testing})` → `{inspect, dispose, ...(testing ? {advance(ticks, command), stress(), fire(ev), internals()} : {})}`.
- **`inspect()`:**
  - `{table, panel, loading, paused, attract, settings, round: copy(game)|null}`
  - `physics: {tick, balls: [{id, x, y, h, vx, vy, mode}], flippers: [{id, angle, up}]}`
  - `stats`, `graphics`, `audio`, `input`, `lamps: {count, lit, shows}`, `dmd: {base, scene, version, text}`.
- Panels (unchanged names): `loading`, `error`, `lobby`, `pause`, `settings`, `help`, `scores`, `game_over`, `confirm_exit`, `binding`, `showcase`, `''`.
- `ui/views.js`: `createView(host, tr, assetURL, tables) → {root, stage, dmdHost, panel(kind, data), kind(), notice(key), names(), focus(), hud: {setPlunger(v, visible), setBallSave(s), setInitialsHint(on)}, setCallouts(available), dispose()}`.

### 3.15 Settings (extend `normalizeSettings`, tolerant of old data)
```js
{ quality: 'high', effects: .65, music: .5, ambience: .3, callouts: true, camera: 'play'|'top', reduced: false,
  keys: { left: 'ShiftLeft', right: 'ShiftRight', plunger: 'Space', start: 'Digit1', nudgeLeft: 'ArrowLeft',
          nudgeRight: 'ArrowRight', nudgeUp: 'ArrowUp', pause: 'Escape', camera: 'KeyC', reset: 'KeyR' },
  pad:  { left: 4, right: 5, plunger: 7, start: 8, pause: 9, camera: 3, reset: 2, nudgeLeft: 14, nudgeRight: 15, nudgeUp: 12 } }
```
- If the new `start` default collides with an existing binding, use the first free key of Digit1, Digit2, KeyS, KeyT. Other duplicate bindings still reset all keys.
- Scores format is unchanged: `{name, score, date}`, top 10 per table. `name` = entered initials.
- Nudge mapping (game.js):
  - `nudgeLeft` → dx −0.20; `nudgeRight` → dx +0.20; `nudgeUp` → dy +0.14.
  - Pad axes: dx = 0.20·v, dy = 0.14·|v| (stick up).
  - Camera shake goes the opposite way.

---
## 4. Physics model (T02–T04)
- **Per tick:**
  1. `devices.preTick`, flipper energise from input & coils, plunger.
  2. Substeps n = clamp(ceil((vmaxBall + maxTipSpeed)·STEP/SUBSTEP_TRAVEL), 2, 32).
  3. Each substep:
     - set px/py
     - integrate field balls (vy −= G_SLOPE·h; rolling decel 0.055 opposite v, never reversing; clamp 12 m/s)
     - advance flippers
     - two passes over contacts (grid shapes, flippers, ball-ball)
     - `devices.substep`, `paths.substep`
     - drain (y < drainY and x < 0.2235 → `drain`, trough+1)
     - plunger seat.
  4. `devices.postTick`, search timer.
- **Contact:**
  - Closest point on segment/circle; penetration p = r_ball − d (+ shape r).
  - Push out along n. If y rose by Δy > 0, scale |v| by `sqrt(max(0, 1 − 2·G_SLOPE·Δy/|v|²))` so push-out never adds energy.
  - Relative velocity against the surface velocity (flippers): vn = −v_rel·n. If vn > 0: v_n' = e·vn; tangential change = min(mu·(1+e)·vn, (2/7)|v_t|).
  - `onHit` fires after resolution with the pre-impact vn.
  - One-way shapes collide only when v·pass < 0. Inactive shapes are skipped.
- **Grid:** cell 0.04 m over x[-0.30, 0.30], y[-0.05, 1.12]. Static and device shapes are inserted by AABB+r. Query by the swept AABB.
- **Flipper:**
  - State a (local), w. World angle φ = side>0 ? a : π−a; ω_world = side·w.
  - Energised: w = min(w+alphaUp·h, omegaMax), and at `up` w = −stopRebound·w.
  - Released: w = max(w−alphaDown·h, −omegaDownMax), and at `rest` w = −restRebound·w.
  - Collision is a tapered capsule: t = projection on pivot→tip; radius rBase+(rTip−rBase)t; surface velocity ω×(X−P).
  - Emits `flipper` on energise and release edges.
- **Plunger:**
  - pull moves toward the command at pullRate/s.
  - A ball in the zone (x > laneX[0], y < zoneYMax, not moving up) is seated at y = yRest − travel·pull.
  - Release (pull > 0.05 and command < 0.02) → v = (0, vMin+(vMax−vMin)·pull) and `launch`. Auto balls launch after 0.6 s at vAuto.
  - `addBall` queues; a ball is served when the lane is free (`shooter` event).
- **Ball-ball:** equal-mass impulse along the centre line, e 0.93, symmetric push-out with Δy compensation; emits `clack` if vn > 0.2.
- **Paths:**
  - Entry is a segment crossing with v·dir > 0, where dir is the entry normal toward points[1]. Initial v = max(0, v·t0).
  - 1D motion: a = −G_SLOPE·t_y − G_NORMAL·t_h − decel·sign(v); |v| ≤ 12.
  - If s ≤ 0 with v < 0, the ball returns to the field at its entry crossing point with velocity −|v|·dir, and `pathFail` fires.
  - At s ≥ L the exit spec applies and `pathMade` fires. Switch exits read the diverter at arrival.
  - Two balls on the same path within 2r swap velocities ×0.9.
- **Holes:**
  - Capture when the centre is within r and speed < captureSpeed. A saucer lip-out gives v ×0.85 and a ±0.15 rad turn.
  - Held balls are positioned at the hole (hidden if `hidden`). Capacity stacks.
  - `eject` releases the oldest after its delay; `'all'` releases them 0.6 s apart.
  - Re-capture cooldown 0.35 s. `autoEject` safety at 6 s unless the ball was locked with `lockBall`. With coils off, eject after 0.5 s.
  - `releaseToTrough` removes held balls (trough+1 each).
- **Magnet** (grab):
  - a = pull·(c−p) − damp·v, |a| ≤ maxAccel, plus +G_SLOPE (gravity cancel) within 0.5r.
  - The ball is held when |p−c| < 0.003 and |v| < 0.04.
- **Ball search:**
  - Trigger: no switch for 10 s, a field ball present, no flipper held.
  - Every 2 s: eject unlocked held balls, kick bumpers and slings touching a ball, give 0.3 m/s (seeded direction, upward) to any ball under 0.02 m/s. Emit `search`.
  - After 5 cycles: `searchFail` and remove the ball.
- **Tilt (coils off):** flippers drop, kickers act passive, magnet off, holes eject.

---
## 5. Shared kit (T01) — `kit/shell.js`, `kit/lower.js`
**Shell** (absolute x):
- Outer walls x = ±0.2575 from y 0 to 0.8125. Arc centre (0, 0.8125), r 0.2575, 37 points from 180° to 0°.
- Shooter wall x 0.2235 (y 0→0.770), post (0.2235, 0.775) r 0.004. Gate `shooterGate` a (0.2235, 0.800), b (0.2575, 0.835), pass [0, 1].
- **Left orbit lane** x∈[-0.2575, -0.2215]:
  - Inner guide: x −0.2215 from y 0.620 to 0.8125, then r 0.2215 from 180° to 150°.
  - Bottom curve: centre (-0.170, 0.620); inner r 0.0515 and outer r 0.0875, both from 180° to 240°.
  - Filler segment (-0.2138, 0.5442)→(-0.2575, 0.500).
  - Trigger `leftOrbitUp` from (-0.2575, 0.700) to (-0.2215, 0.700).
- **Right orbit lane** x∈[0.1875, 0.2235]:
  - Inner guide: x 0.1875 from 0.620 to 0.930.
  - Bottom curve: centre (0.1395, 0.620); inner r 0.048 and outer r 0.084, from 0° to −60°.
  - Filler (0.1815, 0.5473)→(0.2235, 0.500).
  - Trigger `rightOrbitUp` from (0.1875, 0.700) to (0.2235, 0.700).
- Plunger as in §3.1; drainY 0.075.

**Lower** (u = x − cx, cx −0.017, mirror u → −u):
- Flippers `left`/`right`: pivots (u ∓0.080, 0.148). Rest tips leave a 0.0458 m centre gap.
- Divider: metal (u −0.2045, 0.400)→(u −0.2045, 0.2376); rubber post (u −0.2045, 0.4045) r 0.0045.
- Inlane guide: metal (u −0.2045, 0.2376)→(u −0.0738, 0.1582), tangent into the top of the resting bat. The cradle is the V against the raised bat.
- Sling: S1 (u −0.1665, 0.360), S2 (u −0.1665, 0.2526), S3 (u −0.1150, 0.2213); rubber posts r 0.0055. Band walls are offset outward by the post radius: `slingL_face` S1→S3, `slingL_side`, `slingL_base`. Device `slingL {face: 'slingL_face'}`.
- Rollovers `inlaneL` (u −0.1855, 0.300) and `outlaneL` (u −0.2225, 0.300). Optional `kickbackL` (u −0.2225, 0.150).
- Lamps:
  - `inlaneL`/`outlaneL` arrows at y 0.335.
  - `kickbackL` roundRect (u −0.2225, 0.205), label `lamp_kickback`.
  - `shootAgain` shield (u 0, 0.215), 0.030×0.024, label `lamp_shoot_again`.
  - `bonus2x..5x` circles d 0.016 at (u −0.045, 0.285), (u −0.015, 0.295), (u 0.015, 0.295), (u 0.045, 0.285), text 2×..5×.
- Outer boundaries: u ±0.2405, which is x −0.2575 (left) and x 0.2235 (shooter wall). Outlanes and inlanes are 0.036 wide; the inlane channel under the sling is 0.0327 wide (perpendicular).

**Validator** (`tables/validate.js: validateTable(table) → {ok, errors, warnings}`):
- Unique ids across all collections. Every reference resolves (path exits/holes/diverters, sling faces, lamp devices, shots' patterns, pack shots/lamps/holes/modes/targets).
- Lamps ≤ 200, flashers ≤ 6, coordinates in bounds, paths ≥ 2 points.
- Convention lamps exist (§3.9). Every i18n key referenced by layout and pack is listed (returned for the en.json check).
- **Clearance:** no distance between distinct, unconnected colliders in (0.002, 0.030) m unless the pair is in `allowGaps`.

**Layout recipe:**
- Seal narrow slots with a joining wall.
- Channel walls must share a vertex with neighbouring guides.
- Never leave a V pocket that opens upward.

---
## 6. Tables
All three reuse the shell, the lower kit, and the two ramp entry channels:
- **Left channel:** left wall (-0.1958, 0.5754)→(-0.180, 0.598)→(-0.180, 0.670); right wall (-0.132, 0.570)→(-0.132, 0.670); entry y 0.625.
- **Right channel:** left wall (0.104, 0.590)→(0.104, 0.690); right wall (0.1635, 0.5784)→(0.152, 0.600)→(0.152, 0.690)→(0.1875, 0.715); entry y 0.640.

Table tasks may move anchors ≤ 0.04 m to pass the validator, soak and reachability tests, but must keep every feature.

### 6.1 Cyber Neon Metropolis (accent #37d9ff, secondary #ff48c4)
```
y1.07      .~~~~~~~~ arc ~~~~~~~~.
0.93      [L1][L2][L3]            \ shooter gate
0.84   (B1)      (B2)    upper-R flipper
0.75 VAULT   (B3)                 | R orbit
0.70 FIREWALL  [spire]            |
0.62 L-RAMP  [DATA CORE]  R-RAMP  |
0.53   N  E          O  N         |
0.16      \_flip    flip_/        plunger
```
- **Top area:**
  - Top lanes `laneT1..3` at x −0.045 / 0 / 0.045, y 0.930. Guides x ±0.0675 and ±0.0225, y 0.905–0.955.
  - Pops `popL` (−0.045, 0.835), `popR` (0.045, 0.845), `popB` (0, 0.775), lamps `popL/R/B` (mount bumper).
- **Data Core** VUK `dataCore` (0, 0.640) r 0.017:
  - Channel x ±0.024, y 0.585–0.665; spire base circle (0, 0.690) r 0.025.
  - Eject `{path: 'coreTube', speed: 2.6}`; `coreTube` tube (0, 0.640, 0)→(0, 0.650, 0.06)→(0.005, 0.730, 0.090)→(0, 0.800, 0.070)→(0, 0.880, 0.03); exit field (0, 0.885) dir (0, −1), speedScale 0.3.
- **FIREWALL** `firewall` targets `fw1..3` at (−0.128, 0.700), (−0.104, 0.711), (−0.080, 0.722); normal (0.423, −0.906).
  - VAULT kickhole `vault` (−0.100, 0.752), capacity 3, hidden, eject 295°/1.4.
  - Enclosure (−0.139, 0.695)→(−0.148, 0.750)→(−0.112, 0.790)→(−0.065, 0.770)→(−0.069, 0.727).
- **Left orbit:** spinner `spinnerL` across y 0.660.
- **Left ramp** `leftRamp` (plastic, #ff48c4): (−0.156, 0.625, 0)→(−0.156, 0.700, 0.035)→(−0.150, 0.790, 0.068)→(−0.110, 0.870, 0.090); exit path `leftReturn`.
  - `leftReturn` (wire): →(0, 0.935, 0.100)→(0.120, 0.900, 0.100)→(0.200, 0.780, 0.090)→(0.208, 0.620, 0.065)→(0.192, 0.490, 0.035)→(0.172, 0.430, 0.008); exit field (0.1685, 0.418) dir (0, −1) → right inlane.
- **Right ramp** `rightRamp` (wire): (0.128, 0.640, 0)→(0.130, 0.720, 0.040)→(0.130, 0.800, 0.085)→(0.100, 0.880, 0.125)→(0.020, 0.965, 0.155)→(−0.080, 0.955, 0.150)→(−0.150, 0.885, 0.130).
  - Exit switch `lockDiverter`:
    - off → `rightReturn` (wire): →(−0.205, 0.800, 0.110)→(−0.218, 0.680, 0.085)→(−0.212, 0.540, 0.050)→(−0.2025, 0.440, 0.012); exit field (−0.2025, 0.425) dir (0, −1) → left inlane.
    - on → `vaultFeed` (tube): →(−0.165, 0.895, 0.06)→(−0.172, 0.900, 0.03); exit hole `vault` (subway cap).
- **Upper right flipper** `upperRight` (key right): pivot (0.170, 0.780), side −1, length 0.050.
- **Standups:** `neonN` (−0.100, 0.525), `neonE` (−0.070, 0.585), `neonO` (0.070, 0.585), `neonN2` (0.100, 0.525), facing the flippers.
- **Shots:**
  - `leftOrbit` = `trigger:leftOrbitUp@1`; `leftRamp` = `pathMade:leftRamp`
  - `vault` = `hole:vault` (setup `dropBank firewall`); `core` = `hole:dataCore`
  - `rightRamp` = `pathMade:rightRamp`; `rightOrbit` = `trigger:rightOrbitUp@1`.
- **Lamps (about 80):**
  - Per shot: `arrow_`, `jp_`, `combo_` stacked toward the flippers (arrow at the entry, then 0.025 and 0.045 below).
  - `mode_hack/blackout/chase/overclock` in a diamond around the core; `wizard` star (0, 0.500).
  - `extraBall` (0, 0.530); `modeStart` (0, 0.575); `superJackpot` (0, 0.603).
  - `lockLit`, `lock1..3` in front of FIREWALL; `fw1..3`; `neonN/E/O/N2` (letters); `laneT1..3`; `spinnerLit`.
  - 16 `stream1..16` rects at x ±0.035, y 0.25–0.46 (shows only); `skyline1..6` (mount toy); lower kit lamps.
- **Flashers:** `flCore`, `flVault`, `flLeftRamp`, `flRightRamp`, `flPops`. **GI:** lower #d8f0ff, upper #c9b6ff.
- **Toys:** `skyline` (24 instanced buildings with emissive windows along the arc), `spire` (rotating holographic twisted prism with rings over the core; additive shader), `subway` cap.
- **Rules:**
  - Points: bumper 3,010; sling 510; spinner 1,010 (5,010 lit); rollover 2,510; standup 5,010; drop 7,510; bankDown 50,000; ramps 50,000; orbits 30,000; holes 25,000.
  - FIREWALL bankDown lights the lock and sets the diverter on when lit. A lock: `lockBall` + `addBall`, `resetBank`, diverter off.
  - Third lock → GRIDLOCK MULTIBALL: `dropBank firewall`, eject all from the vault, top up to 3 balls. Jackpots: leftRamp, rightRamp, leftOrbit, rightOrbit. Super jackpot at the core after 4. Add-a-ball at the vault.
  - NEON complete: bonusX +1 and 100k. The 2nd completion lights an extra ball; the 3rd lights the kickback.
  - Extra ball at the core (also lit after 2 modes completed). Mode start at the core, relit by 10 spinner revolutions or a NEON completion.
  - Modes:
    - `hack` count [leftRamp, rightRamp]×6, 45 s, 400k, complete 3M
    - `blackout` roving over 5 shots ×4, 45 s, 750k, complete 4M, GI dim
    - `chase` count orbits ×5, 40 s, 500k, complete 3M, spinner lit
    - `overclock` count `bumper:*` ×30, 35 s, 100k, complete 2.5M.
  - Wizard SYSTEM MELTDOWN at the core. Super skill = `hole:dataCore` within 6 s. Replay 40M.
  - Bonus counters: ramps 25k, orbits 15k, targets 5k, bumpers 1k, spins 500, modes 250k, combos 50k.
  - Toy state: `{spire: {speed: 0.4 | 2 mode | 4 MB | 6 wizard, color}, vault: count}`.
- **Attract:** sweepUp 1600, chase 2400, radial 1500, sparkle 2500, spiral 2000, strobe 600, rainbow 3000.
- **Music:** synthwave, 112 BPM, A minor (Am–F–C–G).

### 6.2 Deep Sea Leviathan (accent #43e3d5, secondary #dcad63; kickback right)
- **Top area:** top lanes x −0.090, −0.050, −0.010 (y 0.945); coral pops `coral1` (−0.090, 0.860), `coral2` (−0.010, 0.870), `coral3` (−0.050, 0.800).
- **Upper left flipper** `upperLeft` (key left): (−0.196, 0.760), side +1, length 0.050. Mount block to the orbit guide.
- **Captive pearl** `pearl`: channel (−0.075, 0.600)→(−0.075, 0.700); end switch `pearlTop`.
- **Magnet** `abyss` (0.010, 0.540).
- **Ship scoop** `shipScoop` (0.070, 0.780) r 0.020, mouth facing down-left, eject 240°/1.5; head footprint circle (0.105, 0.880) r 0.035.
- **Sonar standups:** `sonar1..3` at (−0.020/0.010/0.040, 0.650–0.665) facing down; `sonar4` (−0.130, 0.500); `sonar5` (0.105, 0.520).
- **Spinner** `spinnerR` across the right orbit at y 0.660.
- **Current ramp** `currentRamp` (wire, left channel): up the left, over the top at h 0.12, down the right; exit to the right inlane (0.1685, 0.418).
- **Tentacle ramp** `tentacleRamp` (plastic, right channel): up, 270° helix around the head (r 0.05, h 0.10→0.14), over the top-left at h 0.17 (above the current), down the left; exit to the left inlane (−0.2025, 0.425). Switch `tentacleTop` at 0.5.
- **Shots:** leftOrbit, `current` (pathMade), `pearl` (`captive:pearl`), `ship` (`hole:shipScoop`), `tentacle` (pathMade), rightOrbit.
- **Toys:** Leviathan head (lathe and extrude shapes; jaw opens on MB/wizard; emissive eye whose pupil tracks the nearest ball), sunken ship hull around the scoop, coral pop caps, kelp strands (instanced, sway).
- **Rules:**
  - Virtual lock: 3 pearl strikes light the lock (2 for relights). A tentacle shot while lit = lock. The 3rd lock starts ABYSS MULTIBALL (+2 auto balls). Jackpots: current, tentacle, leftOrbit, rightOrbit. Super at the ship.
  - Sonar 5/5: lights Deep Grab (magnet grab 10 s: hold 1.5 s, then throw toward the ship at speed 1.4; scoop capture within 2 s = 1M) and relights the kickback.
  - Modes at the ship:
    - `tsunami` count ramps ×5, 45 s
    - `sonar` roving over `standup:sonar*` ×5, 40 s
    - `treasure` count ship ×3, 50 s, magnet assist
    - `kraken` roving over 6 shots ×4, 45 s.
  - Wizard LEVIATHAN AWAKENS. Extra ball at the ship after 2 modes. Super skill = `hole:shipScoop` within 6 s.
  - Toy state: `{eye: {color, open}, jaw}`.
- **Look and sound:** GI lower #bff6ff, upper #7fd3ff. Music: ambient/sonar, 84 BPM, D dorian.

### 6.3 Lost Temple of the Sun (accent #ffcb65, secondary #8ad6a3; kickback left)
- **Top area:** top lanes and pops placed as on Leviathan (`jungle1..3`).
- **SUN bank** `sun` targets `sunS`, `sunU`, `sunN`, `sunStar` at x −0.036 / −0.012 / 0.012 / 0.036, y 0.600, facing −y, width 0.022.
- **Idol scoop** `idolScoop` (0, 0.720) r 0.020, channel x ±0.026, y 0.650–0.745. Door `idolDoors` (−0.026, 0.660)→(0.026, 0.660). Idol footprint circle (0, 0.790) r 0.030.
- **Jewel standups:** `jewelL` (−0.090, 0.560) and `jewelR` (0.090, 0.560).
- **Upper right flipper** `upperRight` (key right): (0.160, 0.500), side −1, length 0.050. It is fed by the right orbit exit.
- **Orbit lock** `orbitLock`: kickhole (0.2055, 0.770) r 0.016, capacity 3, hidden, `enabled: false`, eject 270°/0.6.
- **Spinner** `spinnerL` in the left orbit.
- **Stone ramp** `leftRamp` (plastic, left channel): up the left, across the top at h 0.10, around the pyramid (0.120, 0.930, square footprint 0.06), down the right; exit to the right inlane.
- **Gold ramp** `rightRamp` (wire, right channel): up, over the top at h 0.155, down the left; exit to the left inlane.
- **Shots:** leftOrbit, leftRamp, `idol` (setup `door idolDoors open`), rightRamp, rightOrbit, `lock` (`hole:orbitLock`, setup `holeEnable`).
- **Toys:** idol head (box/extrude, gold, eyes are lamps `idolEyeL/R` with mount toy), stepped pyramid, stone doors (animated), instanced jungle foliage on the side rails, sun disc.
- **Rules:**
  - SUN bankDown: doors open + mode ready (or wizard ready); DMD "THE DOORS OPEN" / "SUN ALIGNED". After a mode starts: doors close, `resetBank`.
  - 20 pop hits light the lock (`holeEnable orbitLock`). Physical locks; the 3rd starts GOLDEN MULTIBALL (eject all from `orbitLock`, doors forced open). Jackpots: leftRamp, rightRamp, leftOrbit, idol. Super at the idol.
  - Modes:
    - `expedition` orbits ×5
    - `serpent` ramps ×5
    - `riddle` sequence leftRamp → rightRamp → leftOrbit → rightOrbit
    - `eclipse` count `bumper:*` + `drop:*` ×25.
  - Wizard TEMPLE OF THE SUN. Extra ball at the left ramp after 2 modes. Super skill = `pathMade:leftRamp` within 8 s.
  - Toy state: `{idolEyes, doors, pyramidGlow, sunRays}`.
- **Look and sound:** GI lower #ffd59a, upper #ffb347. Music: tribal, 100 BPM, E minor pentatonic (marimba, log drums, flute).

---
## 7. Look, presentation, performance
- **Playfield art** (`art-kit.js: createArtKit(ctx2d, {W, H, layout, tr, palette, underlay}) → kit`):
  - Methods: `px`, `m`, `underlay(img, {brightness: 0.42, saturate: 0.9})`, `autoArt()`, `inserts()`, `labels()`, `glowLine`, `text`, `arrow`, `grain`, `vignette`.
  - `autoArt` layer order: underlay, theme decoration (`paintArt`), lane strips, wall shadows, inserts (unlit = colour×0.3 + rim highlight + label ink), labels, rollover stars, vignette.
  - Canvas 2048×4096 (low tier 1024×2048).
- **Playfield shader** (`materials.js` `onBeforeCompile`):
  - Samples `lampMask` (nearest) and `lampState` (256×1 RGBA8).
  - Emissive += lit·lampGain(2.6) + halo·0.35·lampGain.
  - The GI map (64×128 RG weights) scales diffuse by mix(0.35, 1, gi).
  - Fallback if compile fails: `emissiveMap` canvas recomposited at 30 Hz.
- **Lights and lamps in the scene:** hemisphere light scaled by GI; key directional light; 6 fixed point lights for flashers (2 on low; no shader recompiles at runtime); GI bulbs as an instanced emissive mesh.
- **Ball:** chrome, env from a one-time CubeCamera capture (256 px) of the built table; blob shadow decal per ball; spin is derived from displacement in `balls.js`.
- **Cameras:**
  - `play`: FOV 36°, 52° elevation over the target (0, 0.46).
  - `top`: FOV 30°, looking along the playfield normal.
  - `showcase`: OrbitControls, auto-rotate after 4 s idle.
  - `attract`: ±12° swing.
  - `fitDistance(camera, corners, aspect, insets, margin=0.03)` is pure (binary search).
  - Shake: spring k 900, damping 30; amplitudes nudge 0.004, tilt 0.010, MB 0.003, jackpot 0.002; off when `reduced`.
- **No pause on long frames:** `dt = min(0.05, Δt)`, at most 32 ticks per frame. Pause only on focus loss, hidden window, controller disconnect, or a menu.
- **DMD dock:** top centre, `width: clamp(320px, 52vw, 720px)`, 4:1, inside the HUD layer; toolbar buttons at the edges. The camera fit uses the dock height as the top inset. The 3D backbox shows the backglass webp (emissive) above a speaker panel whose DMD screen is an R8 192×48 `DataTexture` drawn with a dot shader.
- **Event visuals** (rules emit):
  - jackpot: radial show from the shot lamp, 1.2 s + flash all twice
  - super jackpot: strobe 0.8 + radial 1.5; MB start: sweepUp ×3; lock: chase 1 s
  - mode start: spiral; mode complete: radial; skill: ring; extra ball: sparkle 2 s; ball saved: sweepUp 0.8
  - drain: sweepDown + GI flicker 0.4 s; tilt: blackout + GI off; wizard: rainbow loop.
- **SFX recipes** (`sfx.js`; synth layered with CC0 samples):
  - Flippers: flipperUp = noise bp 2.2 kHz 8 ms + sine thump 110→60 Hz + sample `flipper`×0.5; flipperDown = softer click.
  - Bumper: sample + square 70 Hz 30 ms + 1.2 kHz ring 120 ms. Sling: sample + lowpass thump.
  - Targets: drop = sample `target` + 500 Hz clunk; bankReset = 3 clunks + saw 120 Hz buzz; standup.
  - Switches: spinner tick 3.2 kHz 4 ms; rollover; gate ping 2.8 kHz.
  - Holes and coils: scoop (`scoop` sample); eject/VUK thump; kickback; knocker (`knock` + 60 Hz boom).
  - Plunger: ratchet while pulling; release = `launch` + 180→90 Hz boing.
  - Clack. Impacts by material, gain by speed/3.
  - Continuous roll per ball (max 4): field = bandpass 400–900 Hz; plastic = resonant 1.6 kHz with tremolo; wire = HP 3 kHz with random AM 30–60 Hz; tube = LP 600.
  - Drain sweep; tilt buzzer (square 110 Hz, 8 Hz AM); magnet hum 60/120 Hz.
  - Voice limit 24, lowest priority dropped. Pan = clamp(x/0.26)·0.8.
- **Music** (`music.js`):
  - Instruments: sawBass, squareArp, pad, pluck, marimba, flute, subDrone, ping (feedback delay), kick, snare, hat, clap, tom, shaker, log.
  - Song schema `{bpm, instruments, patterns: {name: {bars, tracks: {inst: [[step16, midi, len, vel]]}}}, sections: {attract, main, mode, multiball, wizard, bonus, hiscore: [patternNames]}, stingers: {cue: pattern}}`.
  - Lookahead scheduler (25 ms timer, 0.12 s horizon). Section changes quantise to the next bar. Stingers duck the loop by 8 dB.
  - One shared convolver (2.2 s Leviathan, 1.2 s others); disabled when `reduced`.
- **Voice:** `speechSynthesis` only with `localService && lang` matching the locale; minimum 1.2 s between callouts; priority cancels; silent if no voice. Callout text = `tr('dmd_' + key)`.

| Tier | DPR cap | Bloom | Shadows | Art | Point lights | Draw calls | Triangles |
|---|---|---|---|---|---|---|---|
| low | 1.0 | off (emissive boost) | blob only | 1024×2048 | 2 | ≤110 | ≤250k |
| medium | 1.25 | half-res 0.55 / r 0.4 / th 0.85 | 1024 PCF | 2048×4096 | 6 | ≤170 | ≤400k |
| high | 1.5 | full 0.7 | 2048 | 2048×4096, aniso 8 | 6 | ≤230 (+shadow pass) | ≤600k |

- **Budgets** (60 fps, 4 balls):
  - CPU: physics ≤ 2.0 ms, rules ≤ 0.3, lamps ≤ 0.3, DMD ≤ 0.6 (redraw ≤ 30 Hz), audio ≤ 0.2, render submit ≤ 3.
  - GPU: 1080p high ≤ 9 ms on RTX 3060 / RX 6600 class; GPU textures ≤ 160 MB.
  - Bundle cap 3 MiB.
- **How to stay within budget:** merge static geometry per material; instance posts, rubbers, bolts, bulbs, pops and ribs; ≤ 25 toy draw calls per table.

---
## 8. Tasks
Deps → parallel groups (tasks in one group have disjoint files):

| Wave | Tasks |
|---|---|
| 0 | T01 |
| 1a | T11 |
| 1b (∥) | T02, T06, T07, T08, T09, T10, T18 |
| 2 (∥, after T02) | T03, T04, T05 |
| 3 | T12 (after T05, T07) · T14 (after T03, T04, T05) |
| 4 | T13 (after T12, T03, T04) · T15 (after T06, T14) |
| 5 | T16 (after T13, T14) |
| 6 | T17 (after T06–T10, T13, T15, T16, T18) → T19 |
| 7 (∥) | Leviathan T20 → (T21 ∥ T22) and Temple T23 → (T24 ∥ T25), after T19 |
| 8 | T26 ∥ T27 → T28 |

Every task commits its own paths and runs `npm run test:pinball`.

### T01 Contracts, config, kit, validator, runner
- **Create:** `config.js`, `contracts.js`, `physics/constants.js`, `tables/kit/{geom,shell,lower,compose}.js`, `tables/validate.js`, `tables/index.js`, `tables/{neon,leviathan,temple}/{index,layout,rules}.js` (stubs: shell + lower + meta, empty shots/modes), `scripts/test-pinball.mjs`, `scripts/test-pinball-config.mjs`, `scripts/test-pinball-tables.mjs`.
- **Modify:** `package.json` → `"test:pinball": "node scripts/test-pinball.mjs"`.
- **APIs:** §1, §3.1–3.5 lists, §5.
  - `geom.js`: `arcPoints`, `bandSegments(points, r)`, `mirrorX`, `distPointSeg`, `distSegSeg`, `segIntersect`, `pointInPolygon`, `rotate`.
  - `compose.js`: `composeLayout(...parts)`.
  - Runner order: config, rules, input, physics, devices, paths, tables, layout-neon, layout-leviathan, layout-temple, pack-neon, pack-leviathan, pack-temple, lights, dmd, audio, music, session, assets. Missing files are skipped with a note; the runner exits non-zero on the first failure.
- **Tests:**
  - config: old settings object → music 0.5, callouts true, camera 'play', keys.start 'Digit1'; an old `Digit1` binding moves start to Digit2; duplicates reset; scores clamp to 10 and drop invalid rows.
  - tables: all 3 stubs validate; flipper pivots and sling vertices mirror about cx within 1e-9; a broken copy (duplicate id, dangling exit, 256 lamps, a 0.015 m gap) yields those 4 errors.
  - Key scan: every literal `/(dmd|mode|lamp|card|help)_[a-z0-9_]+/` in `pinball/**/*.js` exists as `pinball.<key>` in `en.json`.
- **Accept:** the old tests still pass; the new tests pass.

### T11 i18n (single commit, all 16 locales)
- Add the §9 keys to `ui/lang/desktop/{16}.json`. DMD, mode and lamp strings are uppercase where the script has case. Keep `{n}` / `{score}` placeholders.
- Do not remove old keys yet (T27 does).
- **Test:** the existing asset-locale check plus the T01 key scan.

### T02 Physics core
- **Files:** `physics/{collide,grid,flipper,world,snapshot}.js`, stub `physics/devices.js` and `physics/paths.js` (§3.7 interfaces), `scripts/lib/pinball-sim.mjs` (`run(world, ticks, inputFn)`, `energy(ball)`, `outline(layout)`, `crossesWall`), and rewrite `scripts/test-pinball-physics.mjs`.
- **Behaviour:** §4 core, plunger, trough, drain, ball-ball, nudge.
- **Tests:**
  1. A synthetic box with thin walls and posts, 200 seeded 10 m/s shots × 0.4 s: no prev→curr crossing of any wall, always inside the outline.
  2. Passive energy, 50 seeded balls × 2 s: E = ½mv² + m·G_SLOPE·y non-increasing per tick (tolerance 1e-9 J).
  3. Stroke reaches `up` in 0.026–0.040 s and returns in ≤ 0.060 s; |w| ≤ omegaMax.
  4. Bare Neon, ball down the left inlane at 0.3 m/s, flip delay sweep 0..80 ticks step 2: max exit speed ∈ [5.0, 8.0] m/s; unpressed rebound < 2.0.
  5. Cradle: held flipper, ball dropped on the bat, 3 s: speed < 0.02, drift over the last 2 s < 0.003, within 0.05 of the pivot. Releasing the flipper frees the ball (> 0.02 m in 0.5 s).
  6. Plunger: pull 1 → launch 3.40 ± 0.05 and the ball passes y 0.80 within 1 s; pull 0.3 → 1.405 ± 0.02; auto launch within 0.8 s.
  7. Drain event; trough restored.
  8. Head-on ball-ball: momentum conserved to 1e-9, e 0.93 ± 0.01.
  9. Nudge dx 0.2 → Δvx exactly 0.2.
  10. Determinism: 3 balls, scripted inputs, 3 s at 30/60/144 Hz frame accumulators → `deepEqual` snapshots and event lists.

### T03 Devices
- **Files:** `physics/devices.js`, `physics/devices-holes.js`, `scripts/test-pinball-devices.mjs`.
- **Behaviour:** §3.2, §4 (holes, magnet, captive, search), coils-off rules, `reset`, `writeAnim`.
- **Tests:**
  - bumper: kick ≥ 2.0 ± 0.1 with event; coils off → no event and no energy gain.
  - sling: 0.6 m/s → kick ≥ 1.9; 0.2 m/s → no event.
  - dropBank: 3 drops + bankDown, then the ball passes through; reset → bankReset; reset blocked while a ball overlaps.
  - spinner: 4 m/s → 15–35 events; 1 m/s → fewer than 8.
  - gate: passes one way, blocks the other.
  - hole: capture at 0.5 m/s; eject ±5 % speed / ±0.05 rad; a 1.5 m/s saucer pass is not captured; autoEject at 6 s; capacity-3 `lockBall` + `eject all` 0.6 s apart; `releaseToTrough`.
  - kickback: armed → vy ≥ 3.5, then disarmed.
  - magnet: holds within 1 s; throw honours angle and speed.
  - captive: strike → `captive` event; 1000 seeded hits never leave the channel.
  - door blocks/passes.
  - search: 5 cycles → `searchFail`.

### T04 Paths
- **Files:** `physics/paths.js`, `scripts/test-pinball-paths.mjs`.
- **Tests:**
  - arc length within 1 % of the polyline length.
  - 0.9 m/s entry → `pathFail` and the ball returns moving away from the ramp.
  - 3.5 m/s → `pathMade`, exit position and direction correct, speed = v_end·0.9.
  - diverter off → field; on → hole event `via: 'path'`.
  - VUK eject travels its path; a backward crossing does not enter.
  - two balls on one path do not pass each other.
  - snapshot h equals the spline h and mode is `'path'`.

### T05 Dev tools
- **Files:** `scripts/pinball-dev-server.mjs` (port 8792), `ui/testdata/pinball-harness.{html,js}`, `ui/testdata/pinball-debug2d.js`, `ui/desktop_pinball_survey_browser_test.go`.
- **Server:** serves `ui/` plus `/node_modules/three/*` only; `/harness` → harness.html; no-store.
- **Harness:**
  - Import map `{"three": "/node_modules/three/build/three.module.js", "three/addons/": "/node_modules/three/examples/jsm/"}`.
  - Params: `table`, `mode=2d|engine|game`, `quality`, `camera`, `debug=1`, `lang`, `scene`, `seed`, `autoplay`.
  - Modules are dynamic-imported with catch, so missing ones are tolerated.
- **debug2d:**
  - Draws walls by material, posts, live flippers, devices, ids, path polylines coloured by h, entries/exits, lamps with index, balls with velocity, grid (G), and an event log of 20.
  - Keys: Shift flippers, Space plunger, N/M/U nudge, B ball at cursor, drag-throw, P pause, `.` step, 1–9 time scale, R reset.
- **Survey test:**
  - Opt-in `AURAGO_PINBALL_SURVEY=1`; `AURAGO_PINBALL_SCENES="neon:2d:low,..."`.
  - Serves `ui/` via `http.Dir(".")` + `../node_modules/three`; PNGs go to `AURAGO_BROWSER_ARTIFACT_DIR` or `reports/pinball-survey/`.
  - Scenes: `2d`, `play`, `top`, `showcase`, `attract`, `action`, `closeup:x,y,h`, `dmd:<scene>`.
- **Accept:** `neon-2d.png` shows the bare table; plunge and flip work in 2d mode.
- `harness.js` is later edited only by T12, T13 and T17 (sequential).

### T06 Rules framework
- **Files:** `rules/{core,features,modes,lamps,highscore}.js`; rewrite `scripts/test-pinball-rules.mjs` with an in-file synthetic pack (4 modes, physical lock, lanes, kickback).
- **API:** §3.8, §3.9.
- **Tests:**
  1. Start 1–4 players → addBall; launch → play.
  2. Save within 8 s → auto addBall + `ballSaved`, ball not advanced; after 9.5 s → bonus.
  3. Skill: lit lane awards; unlit does not; super in window awards, out of window does not.
  4. Lane change rotates on flipper buttons; lanes complete → bonusX 2; cap 5.
  5. Lock ×3 → lockBall+addBall ×2, then eject all + top-up to 3; MB running; save 10 s.
  6. Jackpot base, then base+step; super after 4 = 5× value; MB ends at ≤ 1 ball.
  7. Mode select cycles; confirm → timer; count completes with award; timeout ends; 4 played → wizard ready → 4 balls.
  8. Extra ball repeats the same ball number; shootAgain lamp on.
  9. Bonus = Σ·X added after the count; both flippers skip within 0.2 s.
  10. Tilt on the 3rd warning: coils off, no score, bonus 0; the next ball restores coils.
  11. 2 players → order P1B1, P2B1, P1B2, …, then match.
  12. Seeded match; equal digits → knocker cue.
  13. Initials: left/right wrap, start confirms → `results [{name: 'ABC', score}]`; non-qualifier skips; 45 s timeout.
  14. Training: endless serve, no results.
  15. Replay once → extraBalls +1.
  16. Combo within 3 s awards; after 4 s does not.
  17. `searchFail` re-serves without ending the ball.
  18. Same script → identical `copy(game)`.
- `lampModes` length = lamp count.

### T07 Lights
- **Files:** `lights/{shapes,mask,controller,shows}.js`, `scripts/test-pinball-lights.mjs`.
- **Tests:**
  - every shape is closed with area > 0 and centroid within 1 mm.
  - mask: centre texel R = index; 3 mm outside → G = index, B > 0; far away → 0.
  - `texelOf` and `canvasPx` agree within 0.5 texel.
  - fade: on ≥ 0.95 at 0.1 s; off ≤ 0.05 at 0.25 s; slow blink at 2 Hz.
  - sweepUp ordering by y; replace and priority rules.
  - GI off ≤ 0.05 in 0.2 s; flash peak ≥ 0.9, ≤ 0.05 after 0.4 s.
  - rgba length 1024, texel 0 black.

### T08 DMD
- **Files:** `dmd/{core,scenes-play,scenes-events,font,raster,view}.js`, `scripts/test-pinball-dmd.mjs`.
- **Tests (Node fallback raster):**
  - frame 9216 bytes, levels 0..3.
  - score 1,234,560 → a stable hash across two runs.
  - priority interrupt, queueing and return to base.
  - wipeLeft at 50 % → left columns from the new scene.
  - quantise thresholds.
  - `needsLargeGlyphs('日本')` true, `'JACKPOT'` false.
  - `inspect().text` returns the last text.

### T09 Audio engine, SFX, voice
- **Files:** `audio/{engine,sfx,voice}.js`, `scripts/test-pinball-audio.mjs`.
- **Modify:** `scripts/build-pinball-assets.mjs` selected map, adding clack = impact-metal-light-2, rubber = impact-soft-light-1, scoop = cup-rim-1, knock = impact-wood-heavy-1. Run it to regenerate `ui/3d/pinball/v1/{audio/*.ogg, manifest.json}`.
- **Engine:** buses effects/music/ambience → limiter; unlock/suspend; sample loader with an AbortSignal; ambience beds (Neon city hum, Leviathan rumble, Temple jungle chirps).
- **Pure helpers:** `soundFor(ev) → {recipe, gain, pan, rate} | null`, `createVoiceLimiter(max)`.
- **Tests:** every sound-producing event type and every CUE maps to a recipe; impact < 0.15 → null; pan clamp; the limiter drops the lowest priority.

### T10 Music
- **Files:** `audio/{music,instruments}.js`, `audio/songs/{neon,leviathan,temple}.js`, `scripts/test-pinball-music.mjs`.
- **API:** `createMusic(ctx, out, song) → {section(name), stinger(cue), stop(), dispose()}`, pure `expandBar(song, section, bar)`, `createScheduler(clock, timer)`.
- **Tests:** `expandBar` is deterministic; every section and stinger reference resolves; notes are in range; a fake jittery clock over 10 s schedules each event exactly once; section changes land on a bar boundary.

### T18 Views, HUD, CSS, input
- **Files:** `ui/views.js`, `ui/hud.js`, `ui/css/desktop-app-pinball.css`, `input.js`, `ui/i18n_lint_test.go` (map `js/desktop/apps/pinball/ui/views.js` and `ui/hud.js` to `{name: "tr", prefix: "pinball."}`, replacing the `views.js` entry), `scripts/test-pinball-input.mjs`.
- **View API:** §3.14.
  - Lobby keeps the concept cards.
  - Settings: quality, camera, effects, music, ambience, callouts (disabled + hint when unavailable), reduced, bindings including `start_action`.
  - Help: `help_controls`, `help_rules`, `help_<table>`, credits (Kenney CC0 link + `credits_engine`).
  - Game over shows final scores and the match number.
- **HUD:** DMD dock, vertical plunger meter by the shooter lane, ball-save chip, initials hint, a visually hidden live region. Remove `.pb-missions` and `.pb-scoreboard`.
- **Input:** `start` becomes a timestamped transition; `at()` returns `{left, right, plunger, start}`; when not playing, `start` → `action('start')`.
- **Tests:** start edges between frames are preserved; pad button 8 maps to start; existing assertions still pass.

### T12 Render core
- **Files:** `render/{scene,materials,playfield,art-kit,lighting,camera,cabinet}.js`, `tables/visuals.js`, harness engine mode.
- **Behaviour:** §3.13, §7 (playfield, shader, GI, flashers, cabinet, rails, lockdown bar, apron with translated card canvases, backbox with backglass and DMD screen, cameras, quality, stats).
- **Test (`test-pinball-tables.mjs` append):**
  - `fitDistance` keeps all corners inside the NDC margin for 16:9, 4:3 and 9:16 with a 15 % top inset.
  - `art-kit.px` equals `canvasPx`.
- **Accept:** survey `neon:play:low`, `neon:attract:high`, `neon:top:low` look right; bare table at low ≤ 60 draw calls.

### T13 Render mechanics
- **Files:** `render/{builders,mechs,ramps,balls}.js`.
- Builders return `{object, update(anim, snap)}` per type (§3.6 scalars).
- Ramps: plastic U-channel with lips, translucent; wire = 4 tubes r 0.0015 + instanced ribs every 0.06; tube; diverter flap; entry flaps.
- **Test:** `pathFrames(path)` orthonormal and continuous (adjacent frame dot product > 0.95).
- **Accept:** survey `action` scene shows flippers, pops, slings, drops, spinner and ramps animating; draw calls within the §7 table.

### T14 Neon layout
- **Files:** `tables/neon/{layout,lamps}.js`, `scripts/lib/pinball-shots.mjs` (`sweep`, `soak`), `ui/testdata/pinball-bot.js` (`createBot(layout, seed)`), `scripts/test-pinball-tables.mjs` (generic reachability + soak for every table with shots), `scripts/test-pinball-layout-neon.mjs`.
- **Sweep:** starts `cradle` and `drop` per flipper (upper flipper: drop 0.06 above mid-bat); delays 0..120 ticks step 3; 3 s runs; shot setup commands applied.
- **Assert:** every shot is reached; each lower flipper reaches ≥ 4 shots, including a cross shot.
- **Soak:** 4 balls, bot flipping, 60 s; no NaN/escape, no field ball within 0.02 m for 8 s, zero `searchFail`, ≥ 30 distinct switches.
- **Neon tests:**
  - leftRamp at 3.5 m/s reaches the right flipper area (y < 0.25) within 2 s.
  - rightRamp off → left inlane; on → vault.
  - coreTube → a bumper event within 2 s.
  - vault captures through the dropped bank.

### T15 Neon rules pack
- **Files:** `tables/neon/rules.js`, `scripts/test-pinball-pack-neon.mjs`.
- **Tests (scripted events):**
  - bankDown → lockLit + diverter on.
  - 3 locks → Gridlock (dropBank, eject all, 3 active).
  - each mode completes.
  - NEON ×1 → bonusX; ×2 → extra ball lit.
  - wizard after 4 played.
  - toyState spire speed per state.

### T16 Neon art + toys
- **Files:** `tables/neon/{art,toys}.js`, visuals entry.
- **Art:** neon circuit lines, skyline silhouette, perspective grid floor, hex accents over the underlay.
- **Toys:** §6.1, ≤ 25 draw calls.
- **Accept:** survey play/top/showcase/closeup of the spire at high and low look finished.

### T17 Session + game orchestration (M1)
- **Files:**
  - Create `session.js`, rewrite `game.js` (§3.14, attract loop: shows playlist, DMD attract, music attract once unlocked), `audio/index.js`, harness `game` mode + `autoplay`, `scripts/test-pinball-session.mjs`.
  - Delete `physics.js`, `rules.js`, `scene.js`, `audio.js`, `views.js`, `tables.js`.
  - Rebuild `ui/js/vendor/pinball/*` with the current build script.
- **Frame loop:** §7 no-pause rule; prev/curr snapshot around the last tick; alpha = acc/STEP; `scene.update`; `dmdView.draw`; `audio.roll`.
- **Tests:**
  - A seeded Neon game with the bot for 30 s sim: balls served, phases advance, presentation commands routed, lamp modes length = lamp count.
  - Same input edges at 30/60/144 Hz → identical `copy(game)` and snapshot.
- **Accept:** in the harness game mode, Neon is playable with DMD, lamps, sound and music.

### T19 Build, assets, cleanup
- **`build-pinball.mjs`:** drop the Rapier pin and license (keep the dependency: minigolf uses it); cap 3 MiB; manifest `{three, entry, bytes, sha256}`.
- **`build-pinball-assets.mjs`:** manifest `{version: 2, tables: [{id, backglass, preview, underlay: '<id>-playfield.webp'}], files, audio, sources}`; cap 20 MiB.
- **Delete:** `ui/3d/pinball/v1/*.glb`, `*-physics.json`, `*-normal.png`, `*-ao.png`, `assets/pinball/generate.py`, `assets/pinball/kit/`. Trim `assets/pinball/.gitignore`. Keep `masters/`, `prompts.json`, `artwork.json`, `prepare.py`.
- Remove v1 support in the scene loader.
- **Rewrite `test-pinball-assets.mjs`:** manifest hashes, no orphan files, CC0 audio, 9 artworks + prompts, all `pinball.*` en keys in the 16 locales.
- Rebuild both bundles; both `--check` modes pass.

### T20/T23 layout · T21/T24 rules · T22/T25 art+toys (Leviathan / Temple)
Same files, tests and acceptance as T14/T15/T16 with §6.2 / §6.3:
- Leviathan tests: pearl strike, magnet Deep Grab throw reaching the ship, tentacle → left inlane.
- Temple tests: doors block until open, orbitLock captures only when enabled, upper flipper fed by the right orbit.

### T26 Browser test
Rewrite `ui/desktop_pinball_browser_test.go` (same fixture and faults):
- lobby + attract (`dmd.base === 'attract'`, `lamps.lit > 0`); showcase toggle.
- each table: `graphics.ready && flipperOrder`, one `.pb-canvas`, `.pb-dmd canvas` exists.
- training: `advance(520, {plunger: 1})` → phase `play`, finite balls.
- rebound K key → `flippers[0].angle > 0` after `advance(48)`.
- a 300 ms busy frame does not pause.
- focus loss pauses and needs explicit resume.
- effects 0 and music 0.2 persist; resize, fullscreen, dispose cleanup.
- webgl and assets faults show a translated retry.
- Keep the benchmark mode (`stress()` 4 balls) and the screenshot artifacts.

### T27 Docs + AGENTS
- Rewrite `documentation/pinball.md` (tables, controls, rules per table, settings, local data, dev server/survey, verification, credits).
- Update the pinball blocks in `AGENTS.md` (row ~193), `ui/AGENTS.md` (30–35, 1444–1447), `ui/js/desktop/apps/AGENTS.md` (718–726, 5464–5466), `pinball/AGENTS.md`, `ui/3d/pinball/AGENTS.md`, `assets/pinball/AGENTS.md`.
- Remove obsolete keys from the 16 locales: missions, missions_hint, hack, uplink, security, overload, ruins, relics, current, awakening, expedition, sun_relics, chamber, sunrise, finale_ready, mission_complete, performance_pause, locks.

### T28 Acceptance + landing
- Run the full verification (§10).
- Survey all tables × play/top/attract/action at low and high, plus DMD in de, ja, hi, zh.
- Record draw calls per tier.
- Land per the repo recipe (rebase, `merge --ff-only`). Root `AGENTS.md` is assume-unchanged in the main checkout; if the fast-forward refuses, ask the user.
- Do not push.

---
## 9. New i18n keys (`pinball.` prefix, English)
**UI**
- music "Music" · callouts "Voice callouts" · callouts_unavailable "No offline voice is installed for this language, so callouts stay silent."
- start_action "Start button" · initials_hint "Flippers choose a letter. Start or Launch confirms." · match "Match"
- help_controls "Flippers: left/right Shift. Launch: hold Space, then release. Nudge: arrow keys. Start: 1. Pause: Escape. Camera: C. Training reset: R. All keys can be changed in Settings."
- help_rules "Each player gets three balls. A ball saver protects a quick drain. Hit the lit top lane after launching for a skill shot. Complete the lanes to raise the bonus multiplier up to 5×. Shots in quick succession score combos. Start the four table modes at the mode shot; after playing all four, the wizard mode lights. Lock three balls for multiball, then shoot the lit jackpots. Repeated hard nudges cause a tilt."
- help_neon "FIREWALL: drop all three targets to light the lock in the vault; the right ramp can also feed it. N-E-O-N raises the bonus multiplier. The Data Core starts modes and awards the super jackpot."
- help_leviathan "Strike the captive pearl to light the lock on the tentacle ramp. The sonar targets light the deep grab magnet and relight the kickback. The sunken ship starts modes."
- help_temple "Drop the SUN targets to open the idol's doors and light a mode. The pop bumpers light the lock in the right orbit. Three locked balls start golden multiball."
- credits_engine "Graphics: Three.js (MIT). Music and most sounds are synthesized in real time."
- card_replay "REPLAY AT {score}" · card_rules_neon "DROP FIREWALL · LOCK 3 BALLS\nDATA CORE STARTS MODES\nN-E-O-N ADVANCES BONUS ×" · card_rules_leviathan "PEARL LIGHTS LOCK\nSHIP STARTS MODES\nSONAR LIGHTS DEEP GRAB" · card_rules_temple "SUN OPENS THE IDOL\nPOPS LIGHT THE LOCK\nIDOL STARTS MODES"

**DMD (`dmd_`)**
- player "PLAYER {n}" · ball "BALL {n}" · training "TRAINING" · press_start "PRESS START" · free_play "FREE PLAY" · high_scores "HIGH SCORES" · replay_at "REPLAY AT" · game_over "GAME OVER"
- jackpot "JACKPOT" · super_jackpot "SUPER JACKPOT" · multiball "MULTIBALL" · ball_locked "BALL {n} LOCKED" · lock_lit "LOCK IS LIT" · extra_ball "EXTRA BALL" · extra_ball_lit "EXTRA BALL IS LIT" · shoot_again "SHOOT AGAIN"
- skill_shot "SKILL SHOT" · super_skill_shot "SUPER SKILL SHOT" · ball_saved "BALL SAVED" · replay "REPLAY" · tilt "TILT" · danger "DANGER"
- bonus "BONUS" · bonus_x "BONUS ×{n}" · total_bonus "TOTAL BONUS" · match "MATCH" · combo "{n}× COMBO" · kickback_lit "KICKBACK LIT"
- mode_complete "MODE COMPLETE" · mode_total "MODE TOTAL" · to_go "{n} TO GO" · add_a_ball "ADD-A-BALL" · enter_initials "ENTER INITIALS" · high_score "HIGH SCORE #{n}" · grand_champion "GRAND CHAMPION"
- wizard_lit "WIZARD MODE IS LIT" · jackpots_lit "JACKPOTS ARE LIT" · select_mode "SELECT A MODE"
- Bonus counters: bonus_ramps "RAMPS" · bonus_orbits "ORBITS" · bonus_targets "TARGETS" · bonus_bumpers "BUMPERS" · bonus_spins "SPINS" · bonus_modes "MODES" · bonus_locks "LOCKS" · bonus_combos "COMBOS"
- Hints: hint_ramps "SHOOT THE RAMPS" · hint_orbits "SHOOT THE ORBITS" · hint_lit "HIT THE LIT SHOT" · hint_bumpers "HIT THE POP BUMPERS" · hint_targets "HIT THE TARGETS" · hint_sequence "FOLLOW THE ARROWS" · hint_scoop "SHOOT THE SCOOP" · hint_all "SHOOT EVERYTHING"
- Table events: neon_firewall "FIREWALL DOWN" · neon_letters "N-E-O-N COMPLETE" · leviathan_pearl "PEARL STRIKE" · leviathan_grab "DEEP GRAB" · leviathan_sonar "SONAR LOCKED ON" · temple_doors "THE DOORS OPEN" · temple_sun "SUN ALIGNED"

**Modes (`mode_`)**
- neon_hack "HACK THE GRID" · neon_blackout "BLACKOUT" · neon_chase "NEON CHASE" · neon_overclock "OVERCLOCK" · neon_wizard "SYSTEM MELTDOWN" · neon_mb "GRIDLOCK MULTIBALL"
- leviathan_tsunami "TSUNAMI" · leviathan_sonar "SONAR SWEEP" · leviathan_treasure "SUNKEN TREASURE" · leviathan_kraken "KRAKEN HUNT" · leviathan_wizard "LEVIATHAN AWAKENS" · leviathan_mb "ABYSS MULTIBALL"
- temple_expedition "EXPEDITION" · temple_serpent "SERPENT'S PATH" · temple_riddle "RIDDLE OF THE IDOL" · temple_eclipse "ECLIPSE" · temple_wizard "TEMPLE OF THE SUN" · temple_mb "GOLDEN MULTIBALL"

**Lamps (`lamp_`)**
- extra_ball "EXTRA BALL" · shoot_again "SHOOT AGAIN" · lock "LOCK" · jackpot "JACKPOT" · super "SUPER" · kickback "KICKBACK" · mode "MODE" · combo "COMBO" · wizard "WIZARD" · spinner "SPINNER"

---
## 10. Verification
```
npm run test:pinball
node scripts/build-pinball.mjs --check ; node scripts/build-pinball-assets.mjs --check
go test ./ui -run "TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestFrontend_StaticI18nKeysExistInEnglishBundle|TestDesktopDynamicI18nPrefixKeysExistInAllLanguages|TestShippedWebAssetsLoadNothingExternal" -count=1
go test ./internal/desktop -run TestBuiltinPinballApp -count=1
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopPinballBrowser -count=1 -timeout 20m
AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_PINBALL_SURVEY=1 go test ./ui -run TestDesktopPinballSurvey -count=1   (visual review)
```
**Final acceptance:**
- All 3 tables pass the validator, reachability and soak tests.
- Tip shots 5–8 m/s; cradles hold; ramps fail and succeed correctly.
- DMD, inserts, flashers and GI react to every listed event. Music changes section across attract, main, mode, MB and wizard. No pause dialog on long frames.
- Every `pinball/` file is ≤ 700 lines. 16 locales complete. No GLB/Rapier remains in the pinball bundle. Docs and AGENTS are updated.
- Human feel tuning on aurago-test is the user's acceptance.

---
## 11. Risks → mitigations
1. **Physics feel** (flipper power, ramps too easy or hard): all constants live in `physics/constants.js` with per-flipper overrides. Tip-speed and reachability tests, plus the 2D harness with time scale. Final tuning is the user's.
2. **Ball traps in authored geometry:** clearance validator, 60 s soak per table, ball search as a fallback.
3. **Lamp shader breaks with three r186 internals:** keep the patch small and isolated, with a CPU emissive-canvas fallback; survey on SwiftShader.
4. **DMD legibility (CJK/Devanagari):** size bump, shrink-then-marquee, ≥ 3 px/dot in the DOM dock; surveys in ja, zh, hi.
5. **Audio CPU:** voice limiter, pooled noise buffers, one convolver; `reduced` disables reverb and delay.
6. **Line budget:** the split in §2; every task checks `wc -l`.
7. **Determinism drift:** no Math.random/Date.now; seeded rng; 30/60/144 Hz parity tests at physics and session level.
8. **Shared checkout and landing conflicts:** worktree, path-limited commits, the landing recipe in T28.
9. **Procedural art looks cheap:** the AI playfield painting as underlay, theme decoration layers, survey review per table at two tiers.
10. **Software GL in tests:** pin `low` and assert no FPS.
