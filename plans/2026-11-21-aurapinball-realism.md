# AuraPinball realism overhaul and the Clockwork Observatory table

Plan date 2026-11-21. Repo `C:\Users\Andi\Documents\repo\AuraGo`, branch `main`. All numbers marked *starting value* are meant to be tuned with `tables/validate.js`, the reach test and the 2D harness. Line counts are current counts of files that a package modifies.

---

## 0. Header

### 0.1 Goal

Make AuraPinball look and play more like a real machine:
- a wedge cabinet with real glass clearance, and every ramp, wire and tube fitting under the glass and inside the cabinet;
- real ball spin;
- believable hardware, plastics, reflections and room;
- new painted art for every table.

Add a fourth table, **Clockwork Observatory** (`observatory`), with new mechanics: a rotating orrery disc, a swinging pendulum, gravity-well magnets, a telescope VUK and a 12-hour clock ring.

### 0.2 Decisions (fixed, approved)

1. **Wedge cabinet.** The underside of the glass sits 0.065 m above the playfield at the lockdown bar and 0.14 m at the back. One DOM-free module, `tables/kit/cabinet-dims.js`, is the single source for render and validation.
2. **Every path fits.** Along every spline sample:
   - ball top plus guide allowance plus margin is at or below `glassHeight(y)`;
   - the path footprint stays inside the outline with a margin;
   - overlapping paths are vertically separated.

   The validator enforces this and the tables test fails on violations. Shots stay makeable, and return feeds keep their current end points.
3. **Ball spin.** Full angular velocity `(wx, wy, wz)`:
   - sliding to rolling on the floor (2/7 of the energy goes into rotation);
   - tangential and vertical friction exchange at walls, posts, rubbers and flippers, using the material `mu`;
   - deterministic and JSON-safe, carried in snapshots and lerp, and shown by `render/balls.js`.
4. **Render realism**, tier-aware and reduced-motion-aware, with no simulation changes: planar reflections, contact shadows, rubber rings and star posts, screws, printed RGBA plastics, metal lane guides, a designed apron, wooden side boards, a brighter room, and real habitrails, clear ramps, metal flaps and standoffs.
5. **New art for all four tables** (main session, Codex CLI): playfield underlay painted on a layout guide, an RGBA plastics atlas, apron art, side art, and an observatory backglass. The runtime combines them with procedural inserts, labels and lamps at their exact physical positions.
6. **New table `observatory`** (design in section 5).
7. **Blender GLB hero toys** (main session) for the orrery (armillary sphere), telescope, gear train and pendulum. Runtime loading uses the bundled three.js `GLTFLoader`, with a procedural fallback.

### 0.3 Ground rules (apply to every package)

- **No commits by implementers.** The coordinator commits each package path-scoped (paths are listed under each package).
- **Unrelated edits from another session.** Never stage, format, revert or "fix":
  - `internal/gamemaker/**`, `internal/agent/**`, `internal/server/**`, `prompts/**`
  - `documentation/game-maker-studio.md`
  - `ui/js/desktop/apps/aura-drift-woof-lines.js`
  - `.cursor/**`, `.artifacts/`, `ui/tmp/`
- **GitNexus impact before editing.**
  - Before editing an existing function, class or method, run from the repo root:
    `node ../../../.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .`
  - Report callers, processes and risk. If the result is HIGH or CRITICAL, stop and report before editing.
  - If the result is `UNKNOWN`, or the index is unusable, say so and confirm callers with Grep before editing. During planning the GitNexus registry reported no usable code index for this checkout, so expect the fallback.
- **Line budget.** Every first-party file under `ui/js/desktop/apps/pinball/` stays at or below 700 lines. New files are named in each package; split by responsibility.
- **Determinism.** `physics/*`, `rules/*`, lights, DMD scenes and music must not use `Math.random`, `Date.now` or `performance.now`. All state is plain JSON.
- **Lean tests.** Test the behaviour, plus one test per real bug. The only browser run is F1.
- **No bundle builds.** Implementers do not run `npm run build:pinball` or `node scripts/build-pinball.mjs`, because the vendor bundle is shared by both streams and would capture the other stream's half-done work. The coordinator rebuilds in F1. Exception: R4a and A3 run `node scripts/build-pinball-assets.mjs` (asset manifest only).
- **Temporary scripts and outputs** go to the session's disposable folder, never the repo. Guide and survey output goes to `reports/` (git-ignored).
- **Go tests** follow the repo's GOCACHE/GOTMPDIR conventions for drive D:.

### 0.4 Shared contracts

Both streams code against these before the other side lands.

**C1. Cabinet dimensions.** New file `ui/js/desktop/apps/pinball/tables/kit/cabinet-dims.js` (S0), DOM-free.

```js
export const CABINET = Object.freeze({
    frontY: -0.125, backY: 1.09,          // lockdown bar edge / back wall (playfield y, m)
    sideIn: 0.2675, sideOut: 0.2975, bottom: -0.24,
    glassFront: 0.065, glassBack: 0.14,   // underside of the glass above the playfield surface (m)
    glassThickness: 0.005, railLip: 0.006 // side-rail cap rises this much above the glass top
});
export const GLASS_MARGIN = 0.008;        // required air gap between any path top and the glass
export const OUTLINE_MARGIN = 0.004;      // path footprint to outer wall
export const CROSS_SEP = 0.037;           // 2*BALL_R + 0.010: min vertical separation of overlapping paths
export const PATH_TOP = Object.freeze({ wire: 0.029, plastic: 0.029, tube: 0.034 }); // highest point above the path line
export function glassHeight(y)            // linear frontY..backY, clamped: glassFront + (glassBack-glassFront)*t
export function topAt(y)                  // glassHeight(y) + glassThickness + railLip (side-rail cap height)
export function pathHalfWidth(def)        // plastic: (def.width ?? 0.05)/2 + 0.005; tube: 0.020; wire: max((def.width ?? 0.034)/2, 0.018)
export function maxPathHeight(y, kind)    // glassHeight(y) - PATH_TOP[kind] - GLASS_MARGIN
```

Reference values of `glassHeight(y)`:

| y | 0 | 0.44 | 0.54 | 0.64 | 0.70 | 0.80 | 0.88 | 0.94 | 0.985 | 1.03 |
|---|---|---|---|---|---|---|---|---|---|---|
| glass | .0727 | .0999 | .1061 | .1122 | .1159 | .1221 | .1270 | .1307 | .1335 | .1363 |

The wire and plastic limit is glass − 0.037. The tube limit is glass − 0.042.

**C2. Ball spin.** Ball fields `wx, wy, wz` (rad/s) in the playfield frame: x right, y up the table, z out of the playfield.
- Rolling on a static floor means `vx = R*wy` and `vy = -R*wx`.
- New constant `SPIN` in `physics/constants.js`:

```js
export const SPIN = Object.freeze({ inertia: 0.4, floorMu: 0.10, discMu: 0.18, zDecel: 3.0, maxW: 1000 });
```

- Snapshot ball entries become `{id, x, y, h, vx, vy, wx, wy, wz, mode, hidden}`. `lerpSnapshot` lerps `wx, wy, wz` linearly.
- In three.js coordinates (X = x, Y = h, Z = -y): `ω3 = (wx, wz, -wy)`.

**C3. New devices, events and commands** (S3).

```js
// layout device defs
{ id, type: 'orrery', x, y, r: 0.05, speed: 4, maxSpeed: 18, accel: 20, mu: 0.18 }    // speed rad/s, + = CCW seen from above
{ id, type: 'pendulum', pivot: [x, y], pivotH: 0.0885, length: 0.075, bobR: 0.011, axis: [1, 0],
  period: 1.8, fastPeriod: 1.15, amp: 0.42, park: 1.10, mode: 'swing', material: 'metal', model: 'pendulum' }
// commands
{ type: 'orrery', id, speed }                       // target angular speed, clamped to ±maxSpeed
{ type: 'pendulum', id, mode: 'park'|'swing'|'fast' }
// events (both scoring)
'orrery'   { dir: 1 }        // a field ball entered the disc
'pendulum' { speed }         // the bob struck a ball (approach speed, m/s)
// snapshot anim slots: orrery -> disc angle (rad, listed in anim spin[] for shortest-delta lerp); pendulum -> theta (rad)
```

`contracts.js`: add `'orrery', 'pendulum'` to `EVENT_TYPES` and `PHYSICS_COMMANDS`.

**C4. Plastics** (R4a). New file `tables/kit/plastics.js`, DOM-free.

```js
export const ATLAS = Object.freeze({ size: 2048, grid: 4, cell: 512, pad: 24 });
export function plasticPieces(layout, extras = [])
//  -> [{ id, points: [[x, y], ...] (CCW, m), h, cell: 0..15, standoffs: [[x, y], ...] }]
//  Fixed order: 0 slingL, 1 slingR, 2 inlaneL, 3 inlaneR (derived from lower-kit ids), then extras in declared order (max 12).
export function cellTransform(piece)   // -> { col, row, scale /* px per m */, minX, maxY, ox, oy } aspect-preserving fit into cell - 2*pad
export function atlasPx(piece, x, y)   // -> [px, py] in the 2048 atlas, row 0 at the top
export function atlasUV(piece, x, y)   // -> [u, v] for three (v = 1 - py/size)
```

Per-table extras live in `tables/<id>/plastics.js`, which exports `PLASTICS = [{ id, points, h }]`.

**C5. Asset manifest version 3** (R4a). Table entry:

```json
{ "id": "neon", "underlay": "neon-playfield.webp", "backglass": "neon-backglass.webp", "preview": "neon-preview.webp",
  "artRev": 2, "plastics": "neon-plastics.webp", "apron": "neon-apron.webp", "side": "neon-side.webp",
  "glb": { "orrery": "observatory-orrery.glb" } }
```

- `artRev` is 1 for the old procedural style and 2 when the painted underlay came from the new guide.
- Optional keys appear only when the file exists.
- `glb` appears only on `observatory`.
- The legacy keys `model`, `physics`, `normal`, `ao` and `playfield` stay forbidden.

**C6. Scene plumbing** (R1, then R3a, then R5). `render/scene.js` gives:
- the mechanics host: `host.images = { plastics, apron, side }` (ImageBitmap or null), `host.models` (Map) and `host.reflectLayer` (number);
- `buildToys`: `opts = { tier, low, models, reduced }`;
- `paintPlayfieldArt`: `artRev`.

**C7. Model loading** (R5). New file `render/models.js`:

```js
export async function loadModels(glbMap, { assetURL, signal, env, tier, timeoutMs = 8000 })
//  -> Map<key, { scene: THREE.Group, nodes: Map<string, THREE.Object3D> }>; a failed or missing key is simply absent
export function disposeModels(map)
```

**C8. Rules addition** (S4). `makeCtx(g).startMultiball()` returns a boolean. It starts the pack's `multiball` when no multiball or wizard is running and `pack.multiball` exists.

**C9. Guide and reference outputs** (R4a). Running `node scripts/pinball-art-guide.mjs <id|all>` writes into `reports/pinball-guide/`:
- `<id>-playfield-guide.svg` (2048×4096)
- `<id>-plastics-guide.svg` (2048×2048)
- `<id>-apron-guide.svg` (2048×480)
- `<id>-side-guide.svg` (2048×660)
- `<id>-legend.md`
- `<id>-reference.json` (Blender reference)

**C10. Observatory toy state** (S5 produces it, R5 consumes it):

```js
{ disc: { speed }, orrery: { speed, glow }, telescope: { glow, cap /*0|1*/ }, gears: { hour, run /*0..1*/ },
  pendulum: { mode }, clock: { hour, midnight /*bool*/ } }
```

### 0.5 Package graph

```
SIMULATION STREAM                          RENDER STREAM                   ART STREAM (main session)
S0 cabinet-dims ──────────────────────────► R1 wedge cabinet/glass/room/apron
 │                                          │
S1 ball spin                               R4a plastics data + guides + asset pipeline ──► A1 guides (needs S2, S5 for observatory)
 │                                          │                                               │
S2 path fit + validator + re-routes ──────► (guides exact after S2)                        A2 Codex images, A4 Blender GLBs
 │                                         R2 ramp look + toy glass audit                   │
S3 orrery + pendulum devices ────────────► (R5 needs S3 contract)                          A3 prepare + asset build
 │                                         R3a balls spin/reflection/contact shadows (soft-needs S1)
S4 ctx.startMultiball                      R3b hardware + plastics render
 │                                         R4b art runtime integration (artRev)
S5 observatory table data/rules/locale ──► R5 observatory visuals + GLB loading + registration (needs S3, S5)
S6 observatory audio (independent)
                         D1 docs   →   F1 final build + browser smoke (after everything incl. A3)
```

Shared-file order rules:
- `physics/paths.js`: S1 then S2. `physics/world.js`: S1 then S3. `physics/snapshot.js`: S1 then S3. `tables/validate.js`: S2 then S3.
- `ui/testdata/pinball-debug2d.js`: S2 then S3. `ui/testdata/pinball-harness.js`: S3 then R5.
- `render/scene.js`: R1, then R3a, then R5. `render/lighting.js`: R1 then R3a. `render/builders.js`: R3b then R5. `render/playfield.js`: R3a then R4b.
- `ui/desktop_pinball_browser_test.go`: R2 then R5.
- `tables/leviathan/lamps.js` and `layout.js`: S1 (eject retune only) then S2.
- `tables/leviathan/toys.js`: R2 only.

### 0.6 Commit boundaries

One commit per package, path-scoped to the files listed in each section. Suggested messages:
- `feat(pinball): cabinet dims` (S0)
- `feat(pinball): ball spin` (S1)
- `fix(pinball): paths fit under the glass` (S2)
- `feat(pinball): orrery disc and pendulum devices` (S3)
- `feat(pinball): rules ctx.startMultiball` (S4)
- `feat(pinball): Clockwork Observatory table` (S5)
- `feat(pinball): observatory music and ambience` (S6)
- `feat(pinball): wedge cabinet, apron and room` (R1)
- `feat(pinball): habitrails, clear ramps, toy clearance` (R2)
- `feat(pinball): ball rotation, reflections, contact shadows` (R3a)
- `feat(pinball): hardware and printed plastics` (R3b)
- `feat(pinball): plastics data, art guides, asset manifest v3` (R4a)
- `feat(pinball): painted-art runtime path` (R4b)
- `feat(pinball): observatory visuals and GLB toys` (R5)
- `docs(pinball): ...` (D1)
- `build(pinball): bundle + smoke` (F1)
- Art commits by the main session.

---

## 1. Simulation stream

### S0 — Cabinet dimensions module

**Owner:** simulation stream. **Runs first**; R1 and S2 depend on it.

**Files**
- New: `ui/js/desktop/apps/pinball/tables/kit/cabinet-dims.js` (about 45 lines). Exact API in C1.

**Steps**
1. Implement C1 exactly.
   - `glassHeight` clamps y to `[frontY, backY]`.
   - Freeze all objects.
   - No imports except `BALL_R` from `../../physics/constants.js` (document that `CROSS_SEP = 2*BALL_R + 0.010`).
2. Add a 2-line header comment: render (`cabinet.js`, `camera.js`, `ramps`, clearance) and validation (`validate-paths.js`) must import from here.

**Tests**
- No separate suite; covered by S2 and R1.
- Smoke: `node -e "import('./ui/js/desktop/apps/pinball/tables/kit/cabinet-dims.js').then(m=>console.log(m.glassHeight(0),m.glassHeight(1.09)))"` prints `0.0727…` and `0.14`.

**Acceptance:** imports under Node 20 with no `window`.

### S1 — Ball spin physics

**Owner:** simulation stream. **Depends on** S0 (nothing used yet). Lands before S2 and S3.

**Files**

| Action | File | Lines now |
|---|---|---|
| New | `physics/spin.js` | ~110 |
| Modify | `physics/constants.js` | 39 |
| Modify | `physics/collide.js` | 70 |
| Modify | `physics/world.js` | 398 |
| Modify | `physics/snapshot.js` | 81 |
| Modify | `physics/paths.js` | 301 |
| Modify | `physics/devices-holes.js` | 252 |
| Modify | `scripts/lib/pinball-sim.mjs` | 68 |
| Modify | `scripts/test-pinball-physics.mjs` | 307 |
| Retune only if needed | `scripts/test-pinball-devices.mjs` (294), `scripts/test-pinball-paths.mjs` (136) | |
| Retune only if needed | layout device params in `tables/*/layout.js` and `tables/kit/shell.js` (69) | |

All physics and table paths are under `ui/js/desktop/apps/pinball/`.

**Design**

`constants.js`: add `SPIN` (C2). `ROLL_DECEL` and `MATERIALS` stay unchanged.

`spin.js` (DOM-free, allocation-free). Let `K = SPIN.inertia` (I = K m R²), `INV_K = 1/K` = 2.5, `R = BALL_R`.

```js
export function specificEnergy(b, gSlope)   // 0.5*(vx²+vy²) + 0.5*K*R²*(wx²+wy²+wz²) + gSlope*b.y
export function clearSpin(b)                // wx = wy = wz = 0
export function setRolling(b, vx, vy)       // wx = -vy/R, wy = vx/R (wz untouched)
export function floorStep(b, h, decel, floor)  // floor = null | { svx, svy, mu, moving }
export function contactFriction(b, nx, ny, mu, j, rvx, rvy)  // used by collide.resolveVelocity
```

`floorStep` runs after gravity and before the position update:
1. Floor velocity `s = floor ? (svx, svy) : (0, 0)`; friction `mu = floor ? floor.mu : SPIN.floorMu`.
2. Contact-point slip `u = (vx - R*wy - svx, vy + R*wx - svy)`; `du = (1 + INV_K) * mu * G_NORMAL * h`.
3. If `|u| <= du`, snap to rolling: `v -= (K/(1+K)) * u` (that is, 2/7 of u), then `wx = -(vy - svy)/R`, `wy = (vx - svx)/R`.
4. Otherwise apply sliding friction: `dv = -(mu*G_NORMAL*h/|u|) * u`; `v += dv`; `wx += INV_K*dv.y/R`; `wy -= INV_K*dv.x/R`.
5. Rolling resistance relative to the floor: `rel = v - s`. If `|rel| > 0`, `k = max(0, |rel| - decel*h)/|rel|`; `v = s + rel*k`; `wx *= k`; `wy *= k`.
6. Pivot friction: `wz` moves toward 0 by `SPIN.zDecel*h`.
7. Clamp each `|w*|` to `SPIN.maxW`.

Invariant to document in the header: `vx + K*R*wy` is conserved by floor friction alone, so a sliding ball with no spin ends rolling at 5/7 of its speed.

`collide.js` → `resolveVelocity(ball, nx, ny, material, svx, svy)` keeps its signature. After the normal impulse `j = (1+e)*vn`, and only for `vn >= 0.03`, it calls `contactFriction(ball, nx, ny, mat.mu, j, rvx, rvy)`:

- Tangent `t = (-ny, nx)`.
- In-plane slip `vt = rvx*tx + rvy*ty - R*wz`; impulse needed to stop it: `jtStop = |vt| * K/(1+K)`.
- Vertical slip from the floor roll: `wt = -wx*ny + wy*nx` (the ω component about the wall-tangent axis); impulse needed to stop it: `jzStop = K*R*|wt|`.
- Coulomb cone: `scale = min(1, mu*j / hypot(jtStop, jzStop))`; `jt = jtStop*scale*sign(vt)`; `jz = jzStop*scale*sign(wt)`.
- Apply: `vx -= jt*tx`, `vy -= jt*ty`, `wz += jt/(K*R)`; `wx -= (jz/(K*R))*tx`, `wy -= (jz/(K*R))*ty`.
- Verify by algebra in the header comment: `vt` goes to 0 when unscaled, and `wt` goes to 0 when unscaled.

`capEnergy(ball, e0, gSlope)`: `ke = e0 - gSlope*y`; `cur = 0.5|v|² + 0.5*K*R²|ω|²`. If `cur > ke`, scale v and ω by `sqrt(max(0, ke)/cur)` (zero both when `ke <= 0`).

`world.js`:
- `makeBall` adds `wx: 0, wy: 0, wz: 0`.
- `addBallAt` resets them.
- `launch()` calls `clearSpin`.
- Integration in `substep` becomes:

```js
b.e0 = specificEnergy(b, G_SLOPE);
b.vy -= G_SLOPE * h;
floorStep(b, h, ROLL_DECEL.field, null);       // S3 replaces null with the moving-floor lookup
const s2 = b.vx * b.vx + b.vy * b.vy;
if (s2 > MAX_SPEED * MAX_SPEED) { const k = MAX_SPEED / Math.sqrt(s2); b.vx *= k; b.vy *= k; }
b.x += b.vx * h; b.y += b.vy * h;
```

`snapshot.js`: `ballEntry` and `writeSnapshot` copy `wx, wy, wz`. `lerpSnapshot` lerps them with the same `k` as position.

`paths.js` → `sync(ball, st, P)` also sets `ball.wx = -S.ty*st.v/R`, `ball.wy = S.tx*st.v/R`, `ball.wz = 0`. A ball leaves a path already rolling along the tangent; `toField` keeps ω.

`devices-holes.js`:
- `take`, `release` and the magnet's held and `free` paths call `clearSpin`. Inline the three assignments to avoid an import cycle, or import from `spin.js`.
- The captive `place()` sets rolling ω from `w*ux` and `w*uy`.

`pinball-sim.mjs` → `energy(ball)` adds `0.5*BALL_M*SPIN.inertia*BALL_R²*(wx²+wy²+wz²)`, using `?? 0` for missing fields.

**Tests** (lean; added to `test-pinball-physics.mjs`)
1. **11 Rolling.** In a closed flat box (existing `boxLayout`), place a ball at (0, 0.5) with v = (1, 0) and no spin. After 1 s, `|R*wy - vx| < 1e-3` and `vx` is within 5/7 ± 0.06.
2. **12 Spin exchange.** A ball moving at −y with `wz = +150` hits a rubber wall head-on and leaves with a tangential velocity of the sign the algebra predicts and `|vt| > 0.05`. The same ball without spin leaves with `|vt| < 1e-6`.
3. **§10 determinism:** assert `ref.snap.balls[0].wx !== undefined` (the snapshot carries spin).
4. **§2 passive energy:** keep it unchanged. It now uses total energy through the updated `energy()`.

**Spin-sensitive suites and how to retune without weakening them**

Before editing, record the baseline numbers from:
- `node scripts/test-pinball-physics.mjs` (add temporary logging in your disposable copy if needed): `tip.best`, `inlane.best`, plunger "ball passes y 0.80" tick, cradle drift;
- `node scripts/test-pinball-reach.mjs`: per-shot peak speeds and soak lines.

Report baseline and post-change numbers in the hand-back.

Which suites react:

| Suite | What reacts to spin |
|---|---|
| `test-pinball-physics.mjs` | §2 energy (must pass), §4 tip band 5.0–8.0 and unpressed rebound < 2.0, §5 cradle (speed < 0.02, drift < 3 mm), §5b inlane band 3.9–4.6 and aim, §6 plunger passes y 0.80 in < 1 s, §10 determinism |
| `test-pinball-devices.mjs` | magnet settle, captive, hole capture after ejects |
| `test-pinball-reach.mjs` | all shots reached, ≥ 4 shots and a cross shot per flipper, soak (8 s rest limit, no searchFail) |
| `test-pinball-layout-*.mjs` | fixed-speed ramp shots, scoop eject trajectories. The AGENTS contract on Leviathan's ship-scoop eject past the sonar bank must stay true |

Allowed levers, in this order:
1. `SPIN.floorMu` within 0.06–0.14.
2. `PLUNGER.vMax`/`vAuto` in `world.js` and `kit/shell.js`, at most +8%. Update the §6 `near(...)` expectations to the new constants, not to measured noise.
3. Device eject speeds and kick values in layouts, at most ±10%.
4. `FLIPPER.alphaUp`/`omegaMax`, at most ±5%, only if several tables lose reach.

Forbidden: loosening correctness assertions (energy, tunnelling, determinism, cradle stability, reach coverage, soak, searchFail).

Feel bands (tip 5.0–8.0, inlane 3.9–4.6) may be re-centred once to the measured value, keeping the band width and adding a comment with before and after numbers.

**Commands**
- `node scripts/test-pinball-physics.mjs`
- `node scripts/test-pinball-devices.mjs`
- `node scripts/test-pinball-paths.mjs`
- `node scripts/test-pinball-reach.mjs`
- `node scripts/test-pinball-layout-neon.mjs`, `test-pinball-layout-leviathan.mjs`, `test-pinball-layout-temple.mjs`
- `npm run test:pinball`

**Acceptance**
- All suites green.
- A struck ball visibly slides then rolls: 5/7 test passes.
- Snapshots are identical at 30, 60 and 144 Hz.
- No new file over 700 lines.

**Risks**
- The rolling ball accelerates down the table at 5/7 g·sin θ instead of g·sin θ, so the game feels floatier. Mitigation: `floorMu` and plunger levers. Report subjective feel for the coordinator's play test.

### S2 — Path fit: validator rule and re-routed ramps

**Owner:** simulation stream. **Depends on** S0 and S1 (retune against final physics).

**Files**

| Action | File | Lines now |
|---|---|---|
| New | `tables/validate-paths.js` | ~120 |
| Modify | `tables/validate.js` | 273 |
| Modify | `tables/neon/layout.js` | 128 |
| Modify | `tables/temple/layout.js` | 119 |
| Modify | `tables/leviathan/layout.js` | 136 |
| Modify | `tables/leviathan/lamps.js` | 89 |
| Modify | `scripts/test-pinball-tables.mjs` | 113 |
| Modify if speedScale or asserts need it | `scripts/test-pinball-layout-{neon,temple,leviathan}.mjs` | |
| Modify (optional, 2D debug colouring) | `ui/testdata/pinball-debug2d.js` | |

**validate-paths.js**

```js
import { createPaths } from '../physics/paths.js';
import { glassHeight, maxPathHeight, pathHalfWidth, PATH_TOP, GLASS_MARGIN, OUTLINE_MARGIN, CROSS_SEP } from './kit/cabinet-dims.js';
import { pointInPolygon, distPointSeg } from './kit/geom.js';
export function checkPathFit(layout, err)
```

1. **Samples.** `const dbg = createPaths(layout.paths, { layout }).debug()` (201 samples per path; no change to `paths.js`). The path kind comes from `dbg.kind`; the width from the layout def.
2. **Glass.** For each sample `[x, y, h]`: if `h + PATH_TOP[kind] + GLASS_MARGIN > glassHeight(y) + 1e-9`, report `err('path-glass', 'path "<id>" at (x, y) h <h> needs <mm> mm more glass')`, reporting the worst sample per path only.
3. **Outline.** The polygon is the `outer` wall points (the bottom edge closes implicitly). Each sample must be inside the polygon with edge distance ≥ `pathHalfWidth(def) + OUTLINE_MARGIN`. Otherwise `path-outline` (worst sample per path).
4. **Crossing.** For every pair of paths and every sample pair whose plan distance is below `halfA + halfB`, require `|hA - hB| >= CROSS_SEP`. Skip sample pairs where both samples lie within 0.07 m of arc length (cumulative 3D distance along the samples) of a shared endpoint, meaning any endpoint of A within 2 mm of an endpoint of B, which covers chained paths and diverter fan-outs. Otherwise `path-cross` (worst pair per path pair).

`validate.js`: import and call `checkPathFit(layout, err)` after `checkClearance`.

**Re-routed control points** (*starting values*; end points and plan routes unchanged unless noted)

Neon (`tables/neon/layout.js`):

```js
leftRamp:    [[-0.156,0.625,0],[-0.156,0.700,0.026],[-0.150,0.790,0.040],[-0.110,0.865,0.044]]
leftReturn:  [[-0.110,0.865,0.044],[0,0.928,0.050],[0.120,0.898,0.046],[0.200,0.780,0.042],[0.208,0.620,0.034],[0.192,0.490,0.020],[0.172,0.430,0.006],[0.1685,0.418,0]]
rightRamp:   [[0.128,0.640,0],[0.130,0.720,0.036],[0.128,0.800,0.064],[0.100,0.880,0.084],[0.020,0.962,0.090],[-0.080,0.952,0.090],[-0.150,0.885,0.082]]
rightReturn: [[-0.150,0.885,0.082],[-0.205,0.800,0.070],[-0.218,0.680,0.058],[-0.212,0.540,0.040],[-0.2025,0.440,0.012],[-0.2025,0.425,0]]
vaultFeed:   [[-0.150,0.885,0.082],[-0.165,0.895,0.048],[-0.172,0.900,0.030]]
coreTube:    [[0,0.640,0],[0,0.652,0.048],[0.005,0.730,0.068],[0,0.800,0.058],[0,0.880,0.030]]
// device lockDiverter.at -> [-0.150, 0.885, 0.082]; leftReturn entry.a/b y 0.860 stays; rightReturn/vaultFeed entry y stays.
```

Temple (`tables/temple/layout.js`):

```js
leftRamp:    [[-0.156,0.625,0],[-0.156,0.700,0.026],[-0.150,0.790,0.040],[-0.110,0.865,0.044]]
leftReturn:  [[-0.110,0.865,0.044],[-0.040,0.925,0.050],[0.040,0.935,0.050],[0.100,0.990,0.058],[0.150,0.990,0.058],[0.200,0.900,0.054],[0.208,0.700,0.046],[0.205,0.560,0.036],[0.192,0.490,0.022],[0.172,0.430,0.006],[0.1685,0.418,0]]
rightRamp:   [[0.128,0.640,0],[0.130,0.720,0.036],[0.125,0.800,0.064],[0.090,0.875,0.084],[0.020,0.920,0.090],[-0.080,0.940,0.090],[-0.150,0.885,0.084]]
rightReturn: [[-0.150,0.885,0.084],[-0.205,0.800,0.070],[-0.218,0.680,0.058],[-0.212,0.540,0.040],[-0.2025,0.440,0.012],[-0.2025,0.425,0]]
// idolExit tube already fits; leave it.
```

Leviathan (`tables/leviathan/layout.js` and `tables/leviathan/lamps.js`). The head toy moves out of the helix, because a 0.184 m head under 0.127 m of glass cannot stay. The helix now circles a slim tentacle coil.

```js
// lamps.js
export const HEAD_TOY = [0.205, 1.025];   // creature head in the upper-right dead corner (outside the arc), overhangs the orbit above h 0.035
// eyeL/eyeR lamps: HEAD_TOY[0] ∓ 0.014, HEAD_TOY[1] + 0.012 (mount 'toy'); HEAD stays [0.105, 0.880] (helix and headBase centre)
// layout toys: { id:'head', kind:'leviathan', x:HEAD_TOY[0], y:HEAD_TOY[1], ... } and new { id:'coil', kind:'tentacleCoil', x:HEAD[0], y:HEAD[1], h:0, rot:0, scale:1 }
currentRamp:  [[-0.156,0.625,0],[-0.158,0.700,0.028],[-0.150,0.790,0.044],[-0.115,0.880,0.050],[-0.050,0.960,0.050],[0.030,0.990,0.052],[0.130,0.985,0.052],[0.200,0.900,0.048],[0.212,0.780,0.042],[0.208,0.640,0.034],[0.192,0.500,0.020],[0.172,0.432,0.006],[0.1685,0.418,0]]
tentacleRamp: [[0.128,0.640,0],[0.140,0.720,0.034],[0.162,0.790,0.052], ...helix(radius 0.05 about HEAD, angles [-20,40,100,160,220,250], h [0.060,0.066,0.072,0.078,0.083,0.086]),
               [0.030,0.880,0.088],[-0.040,0.940,0.090],[-0.130,0.955,0.090],[-0.205,0.870,0.084],[-0.219,0.780,0.074],[-0.220,0.680,0.062],[-0.214,0.540,0.044],[-0.2025,0.440,0.014],[-0.2025,0.425,0]]
```

The tentacle coil needs headroom: its top must be ≤ 0.110. The coordinator passes this to R2.

**Tuning loop**
1. Run the validator. Adjust heights first, then plan points by at most 5 mm. Keep every return end point, entry segment and exit `kind`.
2. Exit speeds drop with lower ramps. Raise `exit.speedScale`, up to 1.0, until the layout tests pass (the inlane rollover is still reached).
3. Record the before and after exit speed at each return end in the hand-back.

**Tests** (`test-pinball-tables.mjs`, new section 8 "path fit")
- (a) A neon copy with `rightRamp` sample 5 raised to h 0.155 yields exactly `['path-glass']`.
- (b) A neon copy with `leftReturn` shifted to x 0.25 at one point yields `path-outline`.
- (c) Two synthetic wire paths crossing at (0, 0.5) with h 0.03 and 0.045 yield `path-cross`; at h 0.03 and 0.07 they yield no error.
- Section 1 (all tables validate) now enforces the rule.
- Section 3 ("exactly four named errors") must stay exactly four. The synthetic `bad` path fits; check it.

**Commands**
- `node scripts/test-pinball-tables.mjs`
- `node scripts/test-pinball-reach.mjs`
- `node scripts/test-pinball-layout-neon.mjs`, `test-pinball-layout-temple.mjs`, `test-pinball-layout-leviathan.mjs`
- `npm run test:pinball`

**Acceptance**
- No path errors on all three tables.
- The reach test is green, with peaks reported.
- Return feeds land in the same inlanes.

**Risks**
- Crossing pairs sit at 37–41 mm separation, which is tight. If spline overshoot breaks one, lower the under-path by 2–4 mm rather than raising the upper path.

### S3 — Orrery disc and pendulum devices

**Owner:** simulation stream. **Depends on** S1 and S2.

**Files**

| Action | File | Lines now |
|---|---|---|
| New | `physics/devices-motion.js` | ~210 |
| Modify | `physics/devices.js` | 417 (+4) |
| Modify | `physics/world.js` | ~405 after S1 (+10) |
| Modify | `physics/snapshot.js` | ~87 after S1 |
| Modify | `contracts.js` | 46 |
| Modify | `tables/validate.js` | ~276 after S2 |
| Modify | `scripts/lib/pinball-shots.mjs` | 147 |
| Modify | `scripts/test-pinball-devices.mjs` | 294 |
| Modify | `ui/testdata/pinball-debug2d.js` | |

**world.js moving floors**
- `ctx` gains `addFloor(floor)`, where `floor = { contains(x, y) -> bool, velocity(x, y, out) -> out }` and `out = { svx, svy, mu, moving }`.
- During integration, the first floor that `contains(b.x, b.y)` fills a reused `FL` and is passed to `floorStep`. If `FL.moving`, set `b.exempt = true` (the disc motor may add energy).

**devices-motion.js**

```js
export function createOrrery(def, ctx, clk)   // device object per devices-holes.js contract
export function createPendulum(def, ctx, clk)
```

Orrery:
- Registers `ctx.addFloor`: contains when `(x-cx)²+(y-cy)² < r²`; velocity `svx = -ω(y-cy)`, `svy = ω(x-cx)`, `mu = def.mu ?? SPIN.discMu`, `moving = ω ≠ 0`.
- `post()`: `want = ctx.coils ? target : 0`; ω moves toward `want` by `accel*STEP`; `angle += ω*STEP`, wrapped to (−π, π].
- `sub()` emits `'orrery'` `{dir: 1}` once per entry, tracking ids inside; `remove` and `clear` drop ids.
- `cmd` `{type: 'orrery', speed}` sets target, clamped to ±maxSpeed. `reset()` restores `def.speed`, angle 0.
- `anim`: `a[slot] = angle`. `dbg`: `{id, type: 'orrery', x, y, r, omega, target}`.

Pendulum:
- State: `mode`, `phase`, `A` (amplitude), `O` (offset angle), `theta`, `thetaDot`, `lastHit`.
- Per substep (`sub(h)`, always called): target `(A*, O*)` is `swing → (amp, 0)`, `fast → (amp, 0)`, `park → (0, park)`, and `(0, 0)` when coils are off. A and O move toward their targets at 1.5 rad/s. `phase += 2π/period*h`, where period is `fastPeriod` in fast mode. Then `theta = O + A*sin(phase)` and `thetaDot = dO/dt + dA/dt*sin(phase) + A*cos(phase)*2π/period`.
- Bob plan position: `pivot + axis*L*sin θ`. Bob centre height: `pivotH - L*cos θ`.
- Per field ball:
  - `dh = hBob - BALL_R`, `rr = BALL_R + bobR`; skip if `|dh| >= rr`.
  - `reff = sqrt(rr² - dh²)`; on contact (`d < reff`), push the ball out along n, then `resolveVelocity(b, nx, ny, def.material ?? 'metal', svx, svy)` with `(svx, svy) = axis*L*cos θ*thetaDot`, and set `b.exempt = true`.
  - Emit `'pendulum'` `{speed: vn}` if `vn > 0.15` and `clk.t - lastHit > 0.12`.
- `cmd` `{type: 'pendulum', mode}`. `reset()`: mode = `def.mode ?? 'swing'`, phase 0, A and O at target.
- `anim`: `a[slot] = theta`. `dbg`: `{id, type: 'pendulum', theta, mode, bob: [x, y], hBob}`.

**devices.js:** `FACTORIES.orrery = d => createOrrery(d, ctx, clk)` and `FACTORIES.pendulum = d => createPendulum(d, ctx, clk)`.

**snapshot.js** → `buildAnimIndex` pushes orrery slots into `spin`.

**contracts.js:** C3 additions.

**validate.js**
- `checkBounds`: check `d.pivot`, and for pendulums `pivot ± axis*length`.
- `DEVICE_COMMANDS` adds `'orrery', 'pendulum'`.
- Moving colliders take no part in clearance.

**pinball-shots.mjs:** add `'orrery', 'pendulum'` to `SWITCH_TYPES`.

**pinball-debug2d.js:** draw the disc (circle plus a radius line at the anim angle) and the bob (circle, filled only when collidable).

**Tests** (`test-pinball-devices.mjs`)
1. A ball placed at rest 0.03 m from the centre of a flat-box disc spinning at +8 rad/s gains velocity whose cross product with the radius vector is positive within 0.3 s, and leaves the disc within 3 s. At −8 the sign is reversed. Exactly one `orrery` event per entry.
2. A swinging pendulum strikes a ball rolled across its arc for some of 20 timings (`pendulum` events present); in `park` none do; across the 20 runs the ball is never left inside the bob (`d >= reff - 1e-6`).
3. Snapshots stay deterministic with both devices (one 30, 60 and 144 Hz comparison).

**Commands**
- `node scripts/test-pinball-devices.mjs`
- `node scripts/test-pinball-physics.mjs`
- `node scripts/test-pinball-tables.mjs`
- `npm run test:pinball`

**Acceptance:** green; new device types are documented in the `devices-motion.js` header and the `contracts.js` comments.

**Risks**
- A ball pinned between the bob and a wall end. The bob plane at y 0.568 clears the wall ends at y 0.585; verify in the soak.

### S4 — Rules framework: `ctx.startMultiball()`

**Owner:** simulation stream.

**Files**
- Modify: `rules/modes.js` (285, +4)
- Modify: `scripts/test-pinball-rules.mjs` (451)

**Step.** In `makeCtx` add:

```js
startMultiball: () => { if (mbActive(g) || !pack(g).multiball) return false; startMultiball(g, null); return true; },
```

**Test.** A stub pack with `multiball` and no `lock`: `ctx.startMultiball()` yields `g.mb.state === 'running'`, adds balls up to `balls`, and a second call returns `false`.

**Commands**
- `node scripts/test-pinball-rules.mjs`

**Acceptance:** green.

### S5 — Clockwork Observatory: layout, lamps, rules, registration, locale, tests

**Owner:** simulation stream. **Depends on** S0–S4. The full design is in section 5.

**Files**

| Action | File | Size |
|---|---|---|
| New | `tables/observatory/index.js` | 4 lines (copy neon) |
| New | `tables/observatory/layout.js` | ~150 |
| New | `tables/observatory/lamps.js` | ~120 |
| New | `tables/observatory/rules.js` | ~240 |
| New | `scripts/test-pinball-layout-observatory.mjs` | ~140 |
| New | `scripts/test-pinball-pack-observatory.mjs` | ~220 (structure copied from `test-pinball-pack-temple.mjs`) |
| Modify | `tables/index.js` | 8 |
| Modify | `config.js` (`TABLE_IDS` adds `'observatory'`) | 70 |
| Modify | `scripts/test-pinball.mjs` (`SUITES` adds `layout-observatory` and `pack-observatory`) | 22 |
| Modify | `scripts/test-pinball-config.mjs` (expectation follows `TABLE_IDS`) | 47 |
| Modify | `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` (17 keys, section 5.7) | |

**Steps**
1. Layout, lamps, flashers, GI and toys exactly as in section 5.3–5.5, built on `shell()` and `lowerKit({cx: CX, kickback: 'left'})`.
2. Rule pack as in section 5.6, using `ctx.startMultiball()`.
3. Register in `tables/index.js` and `config.js`. Do not touch `tables/visuals.js` (R5 owns it); `game.js` already tolerates a missing visual.
4. Locale keys in all 16 bundles, UTF-8 with no `\u` escapes. Insert them next to the other tables' keys.

**Tests**

Layout suite:
1. Validates, with the agreed shot ids, 2 flippers, 6 flashers, 70–110 lamps, and the device types `orrery` and `pendulum` present.
2. `comet` ramp at 3.2 m/s reaches the right inlane via `cometReturn`; `meridian` at 3.4 m/s reaches the left inlane.
3. With the pendulum parked, a slow ball up the channel is captured by `telescope`; eject makes `pathMade:telescopeTube`, then an `orrery` event within 2 s.
4. With the pendulum swinging, `pendulum` events occur for some timings and never trap the ball.
5. A ball resting on the disc picks up the disc's direction, and `{type: 'orrery', speed: -8}` reverses it.

Pack suite:
1. 12 hours lead to midnight: multiball running with 3 balls, clock back to 0. Jackpots on the jackpot shots; super jackpot on the telescope after 4 jackpots.
2. S-T-A-R completion: mode lit, `magnet grab` commands for both wells, `pendulum park` command, and slingshot pays 1,000,000 when the telescope follows within 2 s of the throw.
3. Telescope opens the mode select; playing all four modes lights the wizard; telescope starts GRAND ORRERY.
4. Secret combo `orrery → telescope → meridian` pays `dmd_secret_observatory`.
5. Super skill shot `hole:telescope` within 8 s pays 1,000,000.
6. The orrery shot flips the sign of the next `orrery` command.

The reach test picks up the new table automatically. Shot `telescope` carries the setup `{type: 'pendulum', id: 'pendulum', mode: 'park'}`.

**Commands**
- `node scripts/test-pinball-layout-observatory.mjs`
- `node scripts/test-pinball-pack-observatory.mjs`
- `node scripts/test-pinball-tables.mjs`
- `node scripts/test-pinball-reach.mjs`
- `node scripts/test-pinball-config.mjs`
- `npm run test:pinball`
- `go test ./ui -run "TestFrontend_StaticI18nKeysExistInEnglishBundle|TestDesktopDynamicI18nPrefixKeysExistInAllLanguages" -count=1`

**Acceptance**
- Validator clean, including path fit.
- Reach: left and right flippers each reach ≥ 4 shots and a cross shot.
- Soak: no searchFail and no 8 s rest.
- All 16 locale bundles carry the keys.

**Risks**
- Soak traps around the STAR bank gaps or the scope channel. Mitigation: mirror the proven neon and leviathan geometry exactly, as given.

### S6 — Observatory music, ambience and SFX mapping

**Owner:** simulation stream. Independent; only the event names from C3 are needed.

**Files**

| Action | File | Lines now |
|---|---|---|
| New | `audio/songs/observatory.js` | ~170 |
| Modify | `audio/index.js` | 75 |
| Modify | `audio/engine.js` | 220 |
| Modify | `audio/sfx.js` | 232 |
| Modify | `ui/testdata/pinball-audio-render.js` | |
| Modify | `scripts/test-pinball-music.mjs` | 144 |

**Design.** "Clockwork night": 92 BPM, swing 0.04, D minor (D E F G A Bb C). Use the notation helpers from `songs/notation.js`, structured like `songs/temple.js`.
- `CH` chord sets: Dm, Bb, F, C, Gm, A.
- Instruments, all existing voices:

```js
instruments: {
  drone: { type: 'subDrone', gain: 0.45 },
  pad: { type: 'pad', gain: 0.38, send: 0.55, cutoff: 1100, attack: 0.9, release: 0.9 },
  box: { type: 'ping', gain: 0.55, send: 0.5 },        // music box / celesta
  harp: { type: 'pluck', gain: 0.5, send: 0.35 },      // harpsichord arpeggios
  tick: { type: 'hat', gain: 0.28 },                   // escapement tick-tock
  tock: { type: 'marimba', gain: 0.45, decay: 0.2 },
  bass: { type: 'sawBass', gain: 0.45 },
  kick: { type: 'kick', gain: 0.6, pitch: 110, low: 48 },
  snare: { type: 'snare', gain: 0.4 },
  flute: { type: 'flute', gain: 0.45, send: 0.5, vibrato: 8 }
}
```

- Sections, each at least 8 bars: attract (box and pad), main A/B/C (tick-tock on every 4th step, harp arps, box melody), mode A/B, multiball A/B (clearly denser: kick on every 4th step, snare, harp 16ths), wizard, bonus, hiscore. Fills and transitions as in temple.
- Stingers for every `CUES` entry, 0.3–3 s long.
- `audio/engine.js`: `REVERB_SECONDS.observatory = 1.6`; `startAmbience` branch for `'observatory'`: a soft clock tick every 1.0 s (short bandpassed noise click, level 0.04), night crickets (pink noise bandpass 4200 Hz, Q 7, LFO 7 Hz), low wind (brown noise lowpass 260 Hz, level 0.12).
- `audio/sfx.js`: `case 'pendulum': return s('standup', .85, .8);` and `case 'orrery': return s('gate', .35, .7);`.
- `audio/index.js` and the audio render harness: add `observatory` to `SONGS`.

**Tests** (`test-pinball-music.mjs`): add `observatory` to `SONGS` with `BPM.observatory = 92`, and add `audio/songs/observatory.js` to the 700-line list. The existing assertions cover the rest.

**Commands**
- `node scripts/test-pinball-music.mjs`
- `node scripts/test-pinball-audio.mjs`

**Acceptance:** green; multiball density > main × 1.15; wizard density ≥ multiball × 0.9.

---

## 2. Render stream

### R1 — Wedge cabinet, glass, side boards, apron, room

**Owner:** render stream. **Depends on** S0.

**Files**

| Action | File | Size |
|---|---|---|
| New | `render/room.js` | ~110 (move `buildRoom`, `floorCanvas`, `wallCanvas` out of `cabinet.js`) |
| New | `render/apron.js` | ~160 |
| Modify | `render/cabinet.js` | 270 |
| Modify | `render/camera.js` | 204 |
| Modify | `render/lighting.js` | 164 |
| Modify | `render/scene.js` | 280 |

**Steps**

1. **Cabinet** (`cabinet.js`). Replace `DIM` with `CABINET` and `topAt` from C1, and delete `DIM`; only `cabinet.js` used it.
   - Add a helper `wedgeBox(x0, x1, y0, y1, hBottom, hTopAt)` that builds an 8-corner prism with a sloped top and flat normals (non-indexed).
   - Body side walls: `wedgeBox` from `bottom` to `topAt(y)`.
   - Chrome caps and side-rail caps follow `topAt(y)`.
   - Lockdown bar at `h = topAt(frontY) - 0.007`.
   - Side-art quads become trapezoids. The top edge is `topAt`; UV v is the height fraction of the bounding rectangle (bottom to `topAt(backY)`), so the trapezoid crops the art top-front as the side guide shows.
2. **Wooden side boards.** Inner faces at `x = ±sideIn` from h 0 to `glassHeight(y)`, with `mats.wood` and the procedural grain (wood texture repeat 1 per 0.25 m). These are the "wooden side rails". The outer rail caps stay brushed steel.
3. **Glass sheet.** A quad at `glassHeight(y)` with corners at `frontY + 0.012` and `backY - 0.015`; the existing sheen canvas is kept. Visibility rules are unchanged (hidden in low, top and reduced).
4. **Side art.** If `images.side` exists, use it as the `sideMat` map; otherwise keep the procedural `sideArt`.
5. **Apron** (`apron.js`):
   - `buildApron({ group, mats, image, accent, low }) -> { meshes, textures, dispose }`.
   - Shaped top at h 0.012 over `x ∈ [-0.2675, 0.2235]`, `y ∈ [-0.125, 0]`. A notch leaves the plunger lane `x ∈ [0.2235, 0.2575]` open.
   - Bevelled rear lip from h 0.004 at y 0 to 0.012 at y −0.012.
   - Chrome edge strip and 6 screw heads.
   - The texture is `images.apron` (RGBA, mapped over the full apron rectangle, alpha-tested at 0.5 on low) or the current dark-metal fallback.
   - Instruction cards move to h 0.0125 and stay drawn by `cabinet.js`.
6. **Camera** (`camera.js`): `FIELD_CORNERS_LOCAL` uses `[0, topAt(y)]` for each y corner instead of `[0, 0.075]`.
7. **Room** (`room.js`):
   - Warm parquet floor canvas (procedural planks plus a soft light pool) instead of the near-black grid.
   - Back wall in dark slate with two framed procedural posters (blurred accent and secondary colour fields), a warm sconce glow and a subtle neon sign stripe.
   - Floor and wall use `MeshBasicMaterial` brightness about 1.6× the current values.
   - No external assets.
8. **Lighting** (`lighting.js`):
   - `createEnvironment` panel multipliers: back softbox 0.65 → 1.0, front 0.8 → 1.1, side strips 1.2 → 1.0.
   - Hemisphere base 0.25 + 0.65·gi → 0.35 + 0.7·gi; key 1.1 → 1.25.
   - In `scene.js`, `toneMappingExposure` 0.98 → 1.05.
   - Keep every panel × `envMapIntensity` below the bloom threshold 1.65.
9. **Scene** (`scene.js`), manifest plumbing for the whole stream:
   - Load `entry.apron`, `entry.side` and `entry.plastics` bitmaps (same `loadBitmap`, in parallel with the underlay).
   - Pass `images` to `buildCabinet` and to the mechanics host (`addMechanics` adds `images`).
   - Pass `artRev: entry.artRev || 1` to both `paintPlayfieldArt` calls.
   - Close the bitmaps on dispose.
   - Import `buildRoom` from `./room.js`.

**Tests**
- `node scripts/test-pinball-tables.mjs`: §5 camera fit stays green with the new corners.
- Visual check: `node scripts/pinball-dev-server.mjs`, then `/harness?table=neon&mode=engine&quality=high&camera=showcase`. Screenshot via the coordinator's tooling; no browser test in this package.

**Acceptance**
- Glass and rails rise front to back.
- Ramps no longer poke through (combined with S2).
- No file over 700 lines.
- Low tier shows no glass.

**Risks**
- The follow camera can dip below the glass height. The glass is single-sided additive, so it is invisible from below. Verify.

### R2 — Habitrails, clear ramps, flaps, standoffs; toy glass audit

**Owner:** render stream. **Depends on** S0. Visual tuning is best after S2.

**Files**

| Action | File | Size |
|---|---|---|
| New | `render/ramp-builders.js` | ~280 (`plasticRamp`, `wireRamp`, `tubeRamp`, `supports` moved and rewritten) |
| New | `render/clearance.js` | ~60 |
| Modify | `render/ramps.js` | 258 (keep `pathFrames`, `at`, `sweep`, `buildRamps`, `update`; ~130 after the split) |
| Modify | `tables/neon/toys.js` | 278 |
| Modify | `tables/leviathan/toys.js` | 580 |
| Modify | `tables/temple/toys.js` | 383 |
| Modify | `render/scene.js` (stats field only, after R1) | |
| Modify | `ui/desktop_pinball_browser_test.go` (assertion) | |

**Steps**

1. **Wire habitrail.**
   - Wire radius `WR = 0.0021` (4.2 mm stainless).
   - Bottom rails at lateral ±0.0092, height `FU = BALL_R - sqrt((BALL_R+WR)² - 0.0092²)`.
   - Guide rails at lateral ±0.0158, height 0.0135.
   - Formed U straps every 0.05 m (box segments under and around); mounting bracket at each support.
   - Material: chrome clone with roughness 0.32 and `envMapIntensity` 0.55.
2. **Plastic ramp.**
   - Walls 0.022 high.
   - High tier: `MeshPhysicalMaterial({ transmission: 0.85, thickness: 0.0025, ior: 1.49, roughness: 0.12, attenuationColor: tint, attenuationDistance: 0.02, transparent: true })`.
   - Medium: opacity 0.42 with clearcoat 0.3. Low: opacity 0.35 with no clearcoat.
   - Delete the raised sign plate. Print the sign texture as a decal on the outer wall faces (UV along the wall at 40% of the ramp length).
   - Thin edge glow stays.
3. **Entry flap.** A stainless sheet 0.0008 thick, `width − 0.004` wide and 0.022 long, plus a hinge rod and two tabs, with the existing animation.
4. **Standoffs.**
   - Candidate stations every 0.10 m (0.20 m on low) where h > 0.015.
   - Keep a station only if its foot is within 0.012 m of a wall segment or post edge, or outside the outline. Always keep at least one station per ramp: the candidate closest to a wall.
   - Each station gets a hex standoff (r 0.0028, 6 sides), a base flange (r 0.005, h 0.001) and a bracket to the ramp.
5. **Tube.** Clear acrylic: opacity 0.12, collars kept.
6. **clearance.js:**

```js
export function glassViolations(roots, { glassHeight, margin = 0.006 })
// walks meshes and instanced meshes; computes each bounding box in tableRoot space (X = x, Y = h, Z = -y);
// a violation is box.max.y > glassHeight(-box.max.z) - margin (front edge of the box = lowest glass);
// returns [{ name, top, glass }]
```

   In `scene.js`, `stats()` adds `glassViolations: n`, computed lazily over `[mechanicsGroup, toys.group]` and invalidated on quality change and toy rebuild.
7. **Toy audit.** Every toy top must be ≤ `glassHeight(y) − 0.008`.
   - Neon: spire `HH 0.115 → 0.098`.
   - Leviathan: `HEAD_H 0.184 → 0.085`. The head is now built at the toy entry `kind:'leviathan'` (S2 moved it to `HEAD_TOY` (0.205, 1.025); it may overhang the orbit lane only above h 0.035). Add the `tentacleCoil` toy: a twisted tentacle column, radius ≤ 0.020, top ≤ 0.110, at the coil entry (inside the helix).
   - Temple: idol, pyramid, sun disc and foliage as needed.
   - Read toy positions from `layout.toys` rather than constants where a constant moved.
8. **Browser test** (`ui/desktop_pinball_browser_test.go`): in the per-table loop, assert `fixture.inspect().graphics.glassViolations === 0`.

**Tests**
- `node scripts/test-pinball-tables.mjs` (`pathFrames` §7 still green).
- The browser assertion runs in F1.

**Acceptance:** zero violations on all tables in F1; ramps look like stainless habitrails and clear polycarbonate.

**Risks**
- `leviathan/toys.js` is at 580 lines. If the head move adds more than 100 lines, split it into `tables/leviathan/toys-head.js`.

### R3a — True ball rotation, playfield reflections, contact shadows

**Owner:** render stream. **Depends on** R1 (`scene.js` and `lighting.js` order). Uses C2; falls back if `wx` is absent.

**Files**

| Action | File | Lines now |
|---|---|---|
| New | `render/reflect.js` | ~150 |
| Modify | `render/balls.js` | 129 |
| Modify | `render/playfield.js` | 165 |
| Modify | `render/lighting.js` | after R1 |
| Modify | `render/builders.js` (2 lines: bumper caps and flipper tops join the reflect layer) | 389 |
| Modify | `render/scene.js` | after R1 |

**Steps**

1. **balls.js**
   - Orientation: `ω3 = (b.wx, b.wz, -b.wy)`. When `|ω3| > 1e-6`, `dq = axisAngle(ω3/|ω3|, |ω3|*dt)` and `st.q.premultiply(dq)`.
   - If `b.wx === undefined`, keep the old displacement rule. On teleport (> 0.05 m), do not rotate.
   - Steel roughness 0.18 → 0.10.
   - Shadows: a tight contact shadow (plane 0.030, opacity 0.85) plus a soft key-light shadow offset `(+0.0025 + 0.5h, +0.002 + 0.4h)` in playfield metres, scaled `1 + 9h`. Keep the ramp-floor shadow.
2. **reflect.js**

```js
export const REFLECT_LAYER = 3;
export function createPlayfieldReflection({ renderer, scene, camera, tableRoot }) // -> { texture, matrix /*THREE.Matrix4*/, setTier(tier, w, h), update(), dispose(), strength }
```

   - Port the mirror-camera and oblique-clip math from `three/examples/jsm/objects/Reflector.js` (`onBeforeRender`). The plane is tableRoot's local XZ plane.
   - The mirror camera has `layers.set(REFLECT_LAYER)`.
   - Render target per tier:

     | Tier | Render target | Update rate | Strength |
     |---|---|---|---|
     | high | half the canvas size | every frame | 0.22 |
     | medium | quarter | every second frame | 0.15 |
     | low | none (1×1 black texture) | — | 0 |

3. **Reflect layer.** Objects enable `REFLECT_LAYER` in addition to layer 0: ball instanced mesh, flasher domes, GI bulbs (`lighting.js`), bumper caps and cores, flipper tops (`builders.js`).
4. **playfield.js**
   - Extend the patch: uniforms `reflectMap`, `reflectMatrix`, `reflectStrength`.
   - Vertex: after `#include <project_vertex>` add `vReflectUv = reflectMatrix * modelMatrix * vec4(transformed, 1.0);`.
   - Fragment: in `AFTER_EMISSIVE` add `totalEmissiveRadiance += texture2DProj(reflectMap, vReflectUv).rgb * reflectStrength;`.
   - Cache key becomes `pf-lamp-v4`.
   - The CPU fallback path skips the reflection.
5. **scene.js**: create the reflection after the composer; call `update()` before `composer.render`; `setTier` in `applyTier`; dispose.

**Tests:** visual only (F1 survey). Keep `pathFrames` and the other Node suites green.

**Acceptance**
- Balls show true rolling and english, also on ramps.
- Balls and flashers reflect in the playfield on high and medium.
- Low tier has no extra pass.
- Draw calls on high grow by no more than 8 (`stats().drawCalls` in the harness).

**Risks**
- Shader patch conflict with the lamp patch. Mitigation: both live in one `patch` function.

### R3b — Hardware and printed plastics

**Owner:** render stream. **Depends on** R3a (`builders.js` order) and R4a (`kit/plastics.js`).

**Files**

| Action | File | Lines now |
|---|---|---|
| New | `render/hardware.js` | ~230 |
| New | `render/plastics.js` | ~190 |
| Modify | `render/builders.js` | ~391 |
| Modify | `render/mechs.js` | 198 |
| Modify | `render/materials.js` | 133 |

**Steps**

1. **hardware.js** → `buildHardware(ctx)` pushes into `ctx.statics` (merged per material) plus one `InstancedMesh` of screws:
   - **Star posts:** rubber posts with r ≤ 0.006 (sling corners, divider posts). Translucent 5-point star prism, r 0.0068, h 0.024, coloured from accent and secondary, plus a black rubber torus ring (R = post r + 0.0026, tube 0.0024) at h 0.0135. Replace the plain rubber cylinder from `postGeometry` for those posts.
   - **Metal posts:** chrome post plus a top screw head.
   - **Metal lane guides** (walls with `render: 'metal'`): a rounded top bead (half cylinder, r 0.0008), mounting tabs every 0.08 m (0.006 × 0.0015 × 0.004) with screws.
   - **Screws** (dome r 0.0022) on plastics standoffs, lane-guide tabs and flipper pivots. Low tier skips lane-guide screws.
2. **materials.js:** `mats.rail` becomes brushed stainless (roughness 0.42, `envMapIntensity` 0.3); add `mats.starPost` (physical, transmission 0 and opacity 0.85) and `mats.screw`.
3. **plastics.js** → `buildPlastics(ctx, image, extras)`:
   - Pieces from `plasticPieces(layout, extras)` (C4).
   - Plate: the polygon extruded 0.0025 at `piece.h`. UVs via `atlasUV`.
   - Material: `MeshPhysicalMaterial({ map, transparent: true, alphaTest: 0.02, roughness: 0.22, clearcoat: low ? 0 : 0.6, emissive: '#ffffff', emissiveMap: map, emissiveIntensity: 0.18 })`. Low tier uses `alphaTest 0.5`, not transparent.
   - Chrome standoffs at `piece.standoffs` (r 0.003, from the wall top to the plate) with screws on top.
   - Fallback when `image` is null: a procedural 1024² atlas canvas (tinted translucent plastic, printed accent border and table monogram) drawn through `atlasPx`, so the UV path is identical.
4. **builders.js:**
   - Call `buildHardware(ctx)` and `buildPlastics(ctx, host.images?.plastics, extrasFor(layout.id))`. `extrasFor` imports `tables/<id>/plastics.js` through a small static map.
   - `buildSling` skips its plate when a piece `slingL`/`slingR` exists.
   - `buildPosts` skips posts handled by hardware.

**Tests:** visual only (F1). `node scripts/test-pinball-tables.mjs` stays green.

**Acceptance**
- Slings, inlanes and table extras show printed plastics with real alpha.
- Posts have rubber rings and star caps.
- Lane guides look stamped.

### R4a — Plastics data, art guides, reference JSON, asset pipeline (manifest v3)

**Owner:** render stream. **Depends on** S0. Run guides for the main session after S2 (and S5 for observatory).

**Files**

| Action | File | Size |
|---|---|---|
| New | `tables/kit/plastics.js` (C4) | ~110 |
| New | `tables/neon/plastics.js`, `tables/leviathan/plastics.js`, `tables/temple/plastics.js` | ~30 each |
| New | `scripts/pinball-art-guide.mjs` | ~320 |
| New | `scripts/lib/glb-bounds.mjs` | ~100 |
| New | `assets/pinball/stitch.py` | ~40 |
| New | `assets/pinball/models.json` | `{ "version": 1, "models": [] }` |
| Modify | `assets/pinball/prepare.py` | 33 |
| Modify | `scripts/build-pinball-assets.mjs` | 35 |
| Modify | `scripts/test-pinball-assets.mjs` | 53 |
| Modify (schema only) | `assets/pinball/prompts.json`, `assets/pinball/artwork.json` | |

**Steps**

1. **kit/plastics.js.** Derived pieces from the lower kit:
   - `slingL`/`slingR`: sling triangle `p1, p2, p3` inset 0.002, `h = 0.032`.
   - `inlaneL`/`inlaneR`: quad over the divider post and the inlane guide top, `(divider x ± 0.012, y 0.395–0.425)` plus a tail over the guide to y 0.33, `h = 0.034`.
   - Standoffs at the 2–3 post centres covered.
   - Per-table extras (*starting values*):
     - neon: `vaultCap` over `vaultWall`, `roofL` over `chanL_roof`, `laneCaps` over the four top lane guides (one piece).
     - leviathan: `pearlCap` over the pearl roof, `shipCap` over `shipRing`, `ulCap` over `ulMount`, `laneCaps`.
     - temple: `idolCap` over `idolFront`, `pyramidCap` over the pyramid, `laneCaps`.
   - h = the wall height underneath + 0.003. Each piece stays clear of path footprints in plan, or lies ≥ 0.012 below the path line.
2. **pinball-art-guide.mjs** (Node, DOM-free). Imports `TABLES`, `lights/shapes.js` (`canvasPx`, `lampPolygon`), `physics/paths.js` (`createPaths(...).debug()`), `kit/plastics.js` and `kit/cabinet-dims.js`. Per table:
   - **Playfield guide** (2048×4096, mapping `canvasPx`):
     - background `#808080`; outside the outline `#000000`;
     - walls at true thickness: wood `#5a3a1e` (0.010 outward), metal `#c0c0c0` (0.0042), plastic `#66ccff` (0.0036), rubber `#111111` (0.008);
     - posts and bumpers `#303030` with a white 2 px rim; standups and drop targets `#ffcc00`; holes `#000`; magnets: dashed ring; orrery disc: outline plus 12 hour ticks;
     - rollover lanes: dashed white;
     - every lamp polygon filled `#ff00ff` exactly; label areas `#00ffff` rectangles;
     - flipper rest silhouettes 25% white;
     - path footprints translucent (wire `#00ff0040`, plastic `#ff880040`, tube `#8800ff40`) with a centreline;
     - apron edge at y 0.
     - No text inside the image.
   - **Plastics guide** (2048×2048): 4×4 cells of 512 px, each piece outline in `#ff00ff` 4 px through `atlasPx`, interior `#ffffff20`, transparent outside.
   - **Apron guide** (2048×480 for `x ∈ [-0.2675, 0.2675]`, `y ∈ [-0.125, 0]`): reserved card rectangles `#00ffff` (rules card centre (−0.112, −0.064), 0.205×0.08; replay card (0.108, −0.064), 0.18×0.08), the plunger notch `x ≥ 0.2235` black, 6 screw dots.
   - **Side guide** (2048×660, rectangle 1.215 m × (`topAt(backY)` − bottom)): the trapezoid top edge, the cut region black, a 4% safe margin.
   - **Legend** (`<id>-legend.md`): colour codes, piece id → cell, lamp ids with positions.
   - **Reference JSON** (`<id>-reference.json`):

```json
{ "table": "observatory",
  "frames": { "playfield": "x right, y up-table, h up (m)", "gltf": "X = x, Y = h, Z = -y, origin = playfield origin" },
  "glass": { "front": 0.065, "back": 0.14, "frontY": -0.125, "backY": 1.09, "samples": [[0, 0.0727], ...] },
  "outline": [[x, y], ...], "walls": [...], "posts": [...], "devices": [...], "paths": [{ "id": "", "kind": "", "width": 0, "samples": [[x, y, h], ...] }],
  "toys": [...], "lamps": [...] }
```

   Rasterise with headless Chrome:
   `chrome --headless --disable-gpu --screenshot=<png> --window-size=2048,4096 file:///<svg>` (sizes per guide).
3. **stitch.py:** `masters/<id>-playfield-top.png` + `-bottom.png`, each covering half the guide with 128 px overlap, feather-blended into `masters/<id>-playfield.png` (the retained master).
4. **prepare.py.** Tables `['neon', 'leviathan', 'temple', 'observatory']`. Roles are processed only when the master exists:

   | Role | Delivered size | Encoding |
   |---|---|---|
   | playfield | 2048×4096 | q92 |
   | backglass | 2048×1152 | q92 |
   | plastics | 2048×2048 | `-c:v libwebp -quality 92 -alpha_quality 100 -pix_fmt yuva420p` |
   | apron | 2048×480 | RGBA, as plastics |
   | side | 2048×660 | q90 |

   Records gain `rev`: playfield `rev: 2` when its prompt record has `"guide": true`. The concept role stays optional.
5. **build-pinball-assets.mjs** (manifest v3, C5):
   - Table entries from `artwork.json` records, in `TABLE_IDS` order (import `TABLE_IDS` from `config.js`).
   - `artRev` from the playfield record's `rev`.
   - The preview is included when `<id>-preview.webp` exists.
   - GLBs from `models.json`: verify the source sha256, copy `assets/pinball/<file>` unchanged to `v1/<basename>` (check mode compares), and set `table.glb[key]`.
   - Cap: 20 MiB total, 1.5 MiB per GLB, 4 MiB for all GLBs.
6. **test-pinball-assets.mjs**
   - Version 3. Manifest table ids must be a subset of `TABLE_IDS`, in order.
   - Every table needs underlay, backglass and preview. Optional keys must be listed files.
   - Each `artwork.json` record has a matching prompt; the record count is ≥ 9 (no fixed 9).
   - GLB checks via `glb-bounds.mjs`: magic `glTF` v2; no Draco, meshopt or basisu extensions; `extensionsUsed` limited to `KHR_materials_emissive_strength`, `KHR_texture_transform`, `EXT_texture_webp`, `KHR_materials_transmission`, `KHR_materials_ior`; triangle budget per key (section 6); required node names per key; every mesh box top ≤ `glassHeight(yFront) − 0.006`.
   - Locale coverage is unchanged.
7. **glb-bounds.mjs**: `readGlb(bytes) -> { json, bin }`, `nodeWorldMatrices(json)`, `meshBoxes(json)` (POSITION accessor min/max × world matrix), `triangleCount(json)`.

**Tests**
- `node scripts/test-pinball-assets.mjs`
- `node scripts/build-pinball-assets.mjs --check` (after the coordinator rebuilds the manifest with `node scripts/build-pinball-assets.mjs`, which is allowed here)

**Acceptance**
- Guides are generated for all registered tables.
- Manifest v3 is valid with the current three tables (`artRev` 1).

### R4b — Painted-art runtime path

**Owner:** render stream. **Depends on** R3a (`playfield.js` order). Codes against `artRev`; works before and after the art lands.

**Files**

| File | Lines now |
|---|---|
| `render/art-kit.js` | 226 |
| `render/playfield.js` | ~185 |
| `tables/neon/art.js` | 295 |
| `tables/leviathan/art.js` | 391 |
| `tables/temple/art.js` | 296 |

**Steps**

1. **art-kit.js**
   - `createArtKit(ctx, {..., artRev = 1})` exposes `kit.artRev`.
   - New `kit.contactShadows()`: soft radial darkening under posts (r + 0.004), bumpers (r × 1.35), standups and targets (elongated), sling bases and wall feet. Offset `(+0.0025, +0.002)` m; alpha 0.35–0.5; deterministic.
   - `autoArt` order: `underlay → paint → contactShadows → lanes → wallShadows → inserts → labels → rolloverStars → vignette(artRev >= 2 ? 0.22 : 0.5) → grain(artRev >= 2 ? 0.035 : 0.07)`.
2. **playfield.js** → `paintPlayfieldArt({..., artRev})` passes `artRev` and uses `brightness: artRev >= 2 ? 0.95 : 0.78`, saturate 1.0 for rev 2.
3. **Each art.js:** `if (kit.artRev >= 2) { paintAccents(kit); return; }` before the current procedural code. `paintAccents` contains only:
   - functional glow lines along shot lanes toward their arrow lamps (`kit.glowLine`, alpha ≤ 0.35);
   - ramp-entry glows;
   - no full-canvas washes or scenery.

   The rev 1 path stays unchanged as the fallback.

**Tests:** `node scripts/test-pinball-tables.mjs` (§6 art-kit mapping).

**Acceptance:** with a rev 2 underlay, inserts, labels and lamps sit exactly on their positions over the painting (checked in the F1 survey).

### R5 — Observatory visuals, GLB loading, registration

**Owner:** render stream. **Depends on** R3b, S3 and S5 (layout and toy state); uses GLBs from A4 when present.

**Files**

| Action | File | Size |
|---|---|---|
| New | `render/models.js` (C7) | ~120 |
| New | `render/builders-motion.js` | ~210 |
| New | `tables/observatory/art.js` | ~260 |
| New | `tables/observatory/toys.js` | ~380 |
| New | `tables/observatory/plastics.js` | ~35 |
| Modify | `tables/visuals.js` | 16 |
| Modify | `render/builders.js` | after R3b |
| Modify | `render/scene.js` | after R3a |
| Modify | `ui/js/desktop/apps/pinball/ui/views.js` (preview `onerror` fallback) | 131 |
| Modify | `ui/css/desktop-app-pinball.css` (accent and a 4-card lobby grid) | |
| Modify | `ui/testdata/pinball-harness.js` (`PREVIEW_SPOTS.observatory = '-0.04,0.80,0.06'`; `HARNESS_TEXT.card_rules_observatory`) | |
| Modify | `ui/desktop_pinball_browser_test.go` (table list adds `"observatory"`) | |

**Steps**

1. **models.js**
   - `fetch` → `arrayBuffer` → `new GLTFLoader().parseAsync(bytes, '')`, imported from `three/addons/loaders/GLTFLoader.js`. Bundled by esbuild; the dev harness import map already maps `three/addons/`.
   - Per-key timeout and abort. On any failure log `console.warn` once and skip the key.
   - Post-process:
     - meshes: `castShadow` on medium and high;
     - materials: `envMap = env`, `envMapIntensity` 0.55;
     - `glow_*` materials: `toneMapped = false` and keep the emissive;
     - `glass_*` materials: transparent, `depthWrite` false (opaque tinted on low);
     - nodes indexed by name.
   - No Draco, meshopt or KTX2 loaders.
2. **scene.js**
   - After the manifest: `const models = entry.glb ? await loadModels(entry.glb, { assetURL, signal, env: env.texture, tier }) : new Map()`, awaited before toys and mechanics; aborting rejects as usual.
   - Pass `models` to `buildToys` opts and the mechanics host; `disposeModels` on dispose.
3. **builders-motion.js**
   - `buildOrrery(ctx, def)`: flush disc (CircleGeometry r, h 0.0004) with a procedural canvas texture (brass rim, engraved star chart, 12 hour marks; deterministic `rngNext`) plus a chrome rim ring. Rotation from the anim slot.
   - `buildPendulum(ctx, def)`: if `ctx.host.models.get(def.model)` has node `pendulum_arm`, clone the scene, add it, and set `arm.rotation.z = theta` (axis `[1, 0]`; for other axes wrap in a yaw group). Otherwise build procedurally: gantry beam (h 0.095) on two posts at the channel wall ends, a rod and a brass bob sphere (`bobR`).
   - `builders.js` dispatch: `orrery` → `buildOrrery`, `pendulum` → `buildPendulum`.
4. **tables/observatory/art.js** → `paintArt(kit)`.
   - rev 1 (procedural, until A3): mahogany-to-night-sky vertical gradient; brass engraved border along walls; a large brass clock dial around the disc (ring radius 0.058–0.074 with 12 tick marks and Roman numerals as literal text, size 0.006); star-chart dots and constellation lines (seeded); copper and brass lane borders; telescope-lane glow.
   - rev 2: `paintAccents`.
5. **tables/observatory/toys.js** → `buildToys(THREE, kit, layout, { tier, low, models, reduced })`.
   - Armillary at the `kind:'orrery'` toy. GLB `orrery`: drive `orrery_ring_1..3` (local Y rotation at disc speed × [1, −0.6, 0.35]), `orrery_arm_1..4` (× [1.6, 1.0, 0.62, 0.4]) and `glow_sun` (emissive 0.4 + 1.6·glow). Procedural fallback: 3 tilted tori, 4 arm spheres and a sun.
   - Telescope at `kind:'telescope'`. GLB: `telescope_cap` rotation.x eases to −1.6·cap; `glow_lens` emissive. Fallback: a brass cylinder along the `telescopeTube` samples 0.30–0.75 (outer r 0.024) plus a pedestal on the `scopeBase` post.
   - Gears at `kind:'gears'`. GLB: `gear_N` rotation.z += `ratio`·(escapement step 0.105 rad on each hour or tick, eased over 0.12 s); `gear_anchor` rocks ±0.25. Fallback: 5 procedural gears.
   - Clock bezel at `kind:'clockDial'`: a procedural brass ring flush around the disc (h ≤ 0.003).
   - Reduced motion: decorative motions frozen (armillary ≤ 0.2 rad/s, gears step without easing). Physics-driven visuals (disc, pendulum) always shown truthfully.
6. **tables/observatory/plastics.js** extras: `scopeCap` (over the scope channel top, h 0.034), `starBacker` (behind the S-T-A-R bank), `laneCaps` (top lane guides), `gantryPlate` (the pendulum gantry, h 0.100). All clear of path footprints.
7. **visuals.js** adds `observatory: { paintArt, buildToys }`.
8. **views.js:** `image.onerror = () => { image.hidden = true; choice.classList.add('pb-noimage'); }`.
9. **CSS:** `.pb-table[data-table=observatory]{--pb-accent:#d9a650}`; `.pb-tables` grid becomes `repeat(auto-fit,minmax(200px,1fr))` (check line 5/7 of the CSS for the current grid) so four cards fit.
10. **Preview image** (after A3): the survey command in `ui/3d/pinball/AGENTS.md` with scene `observatory:preview:high`, encoded to `ui/3d/pinball/v1/observatory-preview.webp` (≤ 120 KB), then the asset build. The coordinator may run this in F1.

**Tests:** F1 browser smoke (observatory included, `glassViolations === 0`).

**Acceptance:** the observatory renders with procedural fallbacks without GLBs and with GLBs when present; low tier works; no console errors.

---

## 3. Closing packages

### D1 — Documentation

**Files**
- `documentation/pinball.md`
- `ui/js/desktop/apps/pinball/AGENTS.md`
- `ui/3d/pinball/AGENTS.md`
- `assets/pinball/AGENTS.md`
- root `AGENTS.md`, line 217 ("Three original…" becomes four tables, painted art, plastics, GLB toys)
- `internal/desktop/types.go`, line 851 description ("Four original 3D pinball tables …")

**Content**
- **`documentation/pinball.md`:** observatory chapter (rules as section 5); spin; wedge cabinet; art pipeline.
- **Runtime `AGENTS.md`:**
  - four tables;
  - `cabinet-dims.js` contract;
  - path-fit validator rule;
  - spin fields;
  - new devices;
  - `models.js` and GLB node contract;
  - plastics atlas contract.
- **Payload `AGENTS.md`:**
  - manifest v3;
  - GLB files allowed (from `models.json` only, unchanged copies, ≤ 1.5 MiB each);
  - plastics, apron and side WebP.
- **Assets `AGENTS.md`:**
  - replace "no Blender pipeline" with the GLB and `.blend` retention rule (`assets/pinball/models/*.glb` and `assets/pinball/blender/*.blend`, hashes in `models.json`);
  - Codex CLI as the image generator;
  - guides via `scripts/pinball-art-guide.mjs`;
  - `stitch.py`.

**Commands**
- `go test ./internal/desktop -count=1` (types.go only)
- `go test ./ui -run "TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget" -count=1`

### F1 — Final build and verification

**Owner:** coordinator or one implementer, after all packages and A3.

**Steps**
1. Tighten `test-pinball-assets.mjs`: the manifest tables must equal all four `TABLE_IDS`.
2. Run:
   - `npm run build:pinball` (asset manifest and vendor bundle; check that the bundle stays < 3 MiB; GLTFLoader adds about 60–80 KB)
   - `node scripts/build-pinball.mjs --check`
   - `node scripts/build-pinball-assets.mjs --check`
   - `npm run test:pinball`
   - `go test ./ui -run "TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget|TestFrontend_StaticI18nKeysExistInEnglishBundle|TestDesktopDynamicI18nPrefixKeysExistInAllLanguages" -count=1`
   - One browser smoke: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopPinballBrowser -count=1 -timeout 20m`
   - Survey for human acceptance (not a gate): `AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_PINBALL_SURVEY=1 AURAGO_PINBALL_SCENES="neon:play:high,leviathan:play:high,temple:play:high,observatory:play:high,observatory:preview:high" go test -run TestDesktopPinballSurvey -count=1 -timeout 20m ./ui`
3. The coordinator does the play test for feel (spin and floatiness) on real hardware.

---

## 4. Art stream (main session)

- **A1 Guides.** After R4a and S2 (and S5 for observatory): `node scripts/pinball-art-guide.mjs all`, then rasterise the SVGs to PNG with headless Chrome.
- **A2 Images.** Codex CLI generates per section 6, with the guide PNG as the reference image.
  - Save masters to `assets/pinball/masters/`.
  - Add a prompt record per image to `prompts.json`: `{ id, kind, prompt, reference, generator: "codex-cli", guide: true }`.
  - Use `stitch.py` if generating halves.
- **A3 Delivery.** `python assets/pinball/prepare.py`, then `node scripts/build-pinball-assets.mjs`, then `node scripts/test-pinball-assets.mjs`. Commit masters, `artwork.json`, `prompts.json`, `v1/*.webp` and `manifest.json` together.
- **A4 GLBs.** Model in Blender against `observatory-reference.json` (section 6.2).
  - Export to `assets/pinball/models/`; save `.blend` files to `assets/pinball/blender/`.
  - Record them in `models.json` with sha256 and triangle counts.
  - Run `node scripts/build-pinball-assets.mjs` and `node scripts/test-pinball-assets.mjs`.

---

## 5. Clockwork Observatory: design

### 5.1 Theme

A steampunk observatory at night: mahogany and brass instruments under a deep night-sky blue dome, glass lenses and copper pipes. The palette:

| Name | Hex |
|---|---|
| BRASS (accent) | `#d9a650` |
| NIGHT (secondary) | `#5d8cff` |
| COPPER | `#c8743a` |
| MOON | `#e8eefc` |
| GI lower | `#ffd9a0` |
| GI upper | `#9fbfff` |

### 5.2 Unique mechanics

- **Orrery disc:** a flush spinning disc at (−0.040, 0.790), r 0.050. A ball on it is coupled through the moving-floor friction model, so it curls and circles realistically. Rules set its speed and direction.
- **Pendulum:** swings in the vertical x–h plane across the mouth of the telescope lane. The bob only collides near the bottom of its arc, which is exact 3D sphere contact projected to 2D. When parked it is lifted out of the way.
- **Gravity wells:** two magnets (existing device) that throw a held ball at the telescope ("gravity slingshot").
- **Telescope:** a VUK at (0.045, 0.640) that sends the ball up a tube through the brass telescope barrel and drops it onto the orrery disc.
- **12-lamp clock ring** around the disc. Midnight starts MIDNIGHT MULTIBALL.

### 5.3 Layout (*starting values*; `tables/observatory/layout.js` and `lamps.js`)

Base: `shell()` + `lowerKit({ cx: CX, kickback: 'left' })`. No upper flipper.

```js
// meta
{ id: 'observatory', version: 1, meta: { title: 'Clockwork Observatory', accent: '#d9a650', secondary: '#5d8cff', cx: CX, troughSize: 4,
  replayScore: 40000000, giColor: { lower: '#ffd9a0', upper: '#9fbfff' },
  attract: [['spiral', 2000], ['ring', 1600], ['sweepUp', 1600], ['sparkle', 2400], ['radial', 1500], ['breath', 1800]] } }

// top area
walls: laneGuide1..4  metal [[x, 0.940], [x, 0.990]] for x in [-0.0675, -0.0225, 0.0225, 0.0675]
devices: laneT1..3 rollover at x [-0.045, 0, 0.045], y 0.965
         popA bumper (-0.085, 0.885), popB bumper (0.060, 0.880)

// orrery
devices: { id: 'orrery', type: 'orrery', x: -0.040, y: 0.790, r: 0.050, speed: 4, maxSpeed: 18, accel: 20 }
         { id: 'orreryGate', type: 'trigger', a: [-0.120, 0.725], b: [0.012, 0.725] }   // @1 = upward

// telescope (mirrors the proven neon Data Core geometry, shifted to x 0.045)
walls: scopeChanL metal [[0.021, 0.585], [0.021, 0.693]], scopeChanR metal [[0.069, 0.585], [0.069, 0.693]]
posts: scopeBase (0.045, 0.700) r 0.025 metal render 'none'        // touches both wall tops; telescope pedestal sits on it
devices: { id: 'telescope', type: 'hole', kind: 'vuk', x: 0.045, y: 0.640, r: 0.017, capacity: 1, eject: { path: 'telescopeTube', speed: 2.0 } }
         { id: 'pendulum', type: 'pendulum', pivot: [0.045, 0.568], pivotH: 0.0885, length: 0.075, bobR: 0.011, axis: [1, 0],
           period: 1.8, fastPeriod: 1.15, amp: 0.42, park: 1.10, mode: 'swing', model: 'pendulum' }

// S-T-A-R bank: 4 standups on a line falling 25° to the right, 8 mm gaps (leviathan sonar recipe)
// p(n) = (0.112 + (n-2.5)*0.028*cos25°, 0.530 - (n-2.5)*0.028*sin25°), n = 1..4, ids starS, starT, starA, starR
//   = (0.0739, 0.5477) (0.0993, 0.5359) (0.1247, 0.5241) (0.1501, 0.5123); angle STAR_ANGLE = -(π/2 + 25°) = -2.0071; width 0.020
allowGaps: [['starS','starT'], ['starT','starA'], ['starA','starR']]

// wells
devices: { id: 'wellMars', type: 'magnet', x: -0.105, y: 0.470, r: 0.035 }, { id: 'wellSaturn', type: 'magnet', x: 0.071, y: 0.470, r: 0.035 }

// channels (leviathan variant) + spinner
walls: leftChannel().walls, chanL_roof plastic [[-0.2215, 0.700], [-0.180, 0.670]] height 0.04, rightChannel().walls
devices: { id: 'spinnerR', type: 'spinner', a: [0.1875, 0.660], b: [0.2235, 0.660] }

// paths
cometRamp     plastic w 0.05 COPPER entry leftChannel().entry
              [[-0.156,0.625,0],[-0.156,0.700,0.030],[-0.150,0.790,0.050],[-0.120,0.870,0.058]]  exit { kind: 'path', path: 'cometReturn' }
cometReturn   wire w 0.034 COPPER entry { a: [-0.140, 0.860], b: [-0.100, 0.860] } minSpeed 99
              [[-0.120,0.870,0.058],[-0.080,0.945,0.056],[0.000,0.985,0.054],[0.080,0.960,0.052],[0.150,0.890,0.048],
               [0.198,0.790,0.044],[0.206,0.640,0.036],[0.192,0.500,0.022],[0.172,0.432,0.006],[0.1685,0.418,0]]
              exit { kind: 'field', dir: [0, -1], speedScale: 0.8 }                     // right inlane
meridianRamp  wire w 0.034 NIGHT entry rightChannel().entry
              [[0.128,0.640,0],[0.130,0.720,0.035],[0.130,0.800,0.062],[0.115,0.900,0.082],[0.060,0.985,0.092],[-0.030,1.010,0.095],
               [-0.110,0.985,0.094],[-0.170,0.910,0.088],[-0.205,0.810,0.075],[-0.218,0.680,0.062],[-0.212,0.540,0.045],[-0.2025,0.440,0.012],[-0.2025,0.425,0]]
              exit { kind: 'field', dir: [0, -1], speedScale: 0.75 }                    // left inlane
telescopeTube tube w 0.034 BRASS entry { a: [0.028, 0.640], b: [0.062, 0.640] } minSpeed 99
              [[0.045,0.640,0],[0.045,0.656,0.036],[0.040,0.705,0.060],[0.020,0.760,0.064],[0.000,0.800,0.044],[-0.015,0.815,0.012]]
              exit { kind: 'field', dir: [-0.3, -1], speedScale: 0.3 }                  // drops onto the disc
// meridian crosses over cometReturn near (0, 0.99) and (0.07, 0.97): separation ~0.038-0.040. If path-cross fires, lower cometReturn by 2-4 mm.

// toys (positions also used by the Blender reference)
{ id: 'armillary', kind: 'orrery', x: 0.212, y: 1.030 }   // upper-right dead corner (outside the arc)
{ id: 'gearTrain', kind: 'gears', x: -0.212, y: 1.030 }   // upper-left dead corner
{ id: 'scope', kind: 'telescope', x: 0.045, y: 0.700 }
{ id: 'clock', kind: 'clockDial', x: -0.040, y: 0.790 }
labels: [{ key: 'lamp_jackpot', x: -0.156, y: 0.560, size: 0.007, angle: 0, color: BRASS }]

// flashers (6)
flTelescope (0.045, 0.705, h 0.08, BRASS)     flOrrery (-0.040, 0.860, 0.06, MOON)     flComet (-0.156, 0.720, 0.05, COPPER)
flMeridian (0.130, 0.760, 0.05, NIGHT)        flPendulum (0.0865, 0.585, 0.05, BRASS)  flWells (-0.155, 0.500, 0.04, NIGHT)
// GI: neonGI() pattern with the colours above
```

Checked clearances:
- popA and popB vs lane guides: ≥ 36 mm.
- `scopeChanR` vs `chanR_left`: 35 mm.
- starS end vs `scopeChanR` start: 33 mm.
- starR end vs the `orbitR` curve ends: ≥ 45 mm.
- The pendulum bob plane (y 0.557–0.579) vs the wall ends (0.585): clear.
- Dead-corner toys: outside the playfield arc incl. the 10 mm wood wall.
- All paths pass the C1 limits by hand calculation with 1–5 mm to spare. The validator decides.

### 5.4 Lamp list (`lamps.js`; lamp index = array index + 1, so the order matters for shows)

| Group | Lamps (x, y, shape, size, colour) |
|---|---|
| Shot stacks: arrow / jp / combo (arrow 0.015×0.022, jp diamond 0.014, combo chevron 0.014×0.012) | leftOrbit (−0.2395, 0.640) / (−0.2395, 0.613) / (−0.2345, 0.590) NIGHT · comet (−0.156, 0.625) / (−0.156, 0.600) / (−0.156, 0.580) COPPER · orrery (−0.060, 0.700) / (−0.060, 0.676) / (−0.060, 0.655) MOON · telescope (0.045, 0.598) / (0.006, 0.598) / (0.006, 0.576) BRASS · meridian (0.128, 0.640) / (0.128, 0.615) / (0.128, 0.595) NIGHT · rightOrbit (0.2055, 0.640) / (0.2055, 0.615) / (0.200, 0.595) NIGHT |
| Modes (circle 0.015, group `modes`) | mode_meteor (−0.050, 0.540) COPPER · mode_transit (−0.020, 0.540) MOON · mode_conjunction (0.010, 0.540) NIGHT · mode_supernova (0.040, 0.540) `#ff7a5c` |
| Centre column | modeStart (−0.017, 0.503) roundRect 0.026×0.016 `lamp_mode` · superJackpot (−0.017, 0.470) star 0.020 `lamp_super` · extraBall (−0.017, 0.436) roundRect 0.030×0.016 `lamp_extra_ball` · wizard (−0.017, 0.397) star 0.028 `lamp_wizard` |
| Wells | wellMarsLit (−0.105, 0.428) circle 0.018 COPPER · wellSaturnLit (0.071, 0.428) circle 0.018 NIGHT |
| Pendulum | pendulumLit (0.045, 0.512) circle 0.014 BRASS |
| S-T-A-R (device-linked, text S/T/A/R, circle 0.014, group `star`) | at each target + 0.024·(cos, sin)(STAR_ANGLE) = + (−0.0101, −0.0218) |
| Clock ring (circle 0.012, MOON, group `clock`) | clock1..clock12 at (−0.040 + 0.066·cos(90° − 30°·k), 0.790 + 0.066·sin(90° − 30°·k)); clock12 at the top |
| Top lanes | laneT1..3 at (x, 0.965) circle 0.016 `#ffb347`, device-linked, group `topLanes` |
| Spinner | spinnerLit (0.2055, 0.688) roundRect 0.014×0.012 `lamp_spinner` |
| Pops | popA, popB circle 0.030, mount `bumper`, device-linked |
| Sky (light show, group `sky`) | sky1..14: x = −0.017 ± 0.035 (alternating), y = 0.328 + floor(i/2)·0.0215, star 0.012 MOON/NIGHT |
| Lower kit (from `lowerKit`) | inlane/outlane ×4, kickbackL, shootAgain, bonus2x–5x |

Total is about 75 lamps; the limit is 200.

### 5.5 Shots

```js
{ id: 'leftOrbit',  on: 'trigger:leftOrbitUp@1',  combo: true, bonus: 'orbits', setup: [] },
{ id: 'comet',      on: 'pathMade:cometRamp',     combo: true, bonus: 'ramps',  setup: [] },
{ id: 'orrery',     on: 'trigger:orreryGate@1',   combo: true, setup: [] },
{ id: 'telescope',  on: 'hole:telescope',         combo: true, setup: [{ type: 'pendulum', id: 'pendulum', mode: 'park' }] },
{ id: 'meridian',   on: 'pathMade:meridianRamp',  combo: true, bonus: 'ramps',  setup: [] },
{ id: 'rightOrbit', on: 'trigger:rightOrbitUp@1', combo: true, bonus: 'orbits', setup: [] }
```

Cross shots:
- Left flipper (needs arrow x > CX + 0.03): telescope, meridian, rightOrbit.
- Right flipper (needs arrow x < CX − 0.03): leftOrbit, comet, orrery.

### 5.6 Rules (`tables/observatory/rules.js`)

**Pack fields**

```js
points: { 'bumper:*': 3010, 'sling:*': 510, 'spinner:*': 1010, 'rollover:*': 2510, 'standup:*': 5010, 'pendulum:*': 7510,
          'orrery:*': 1010, 'hole:*': 25000, 'pathMade:*': 0, default: 1010 },
shotPoints: { comet: 50000, meridian: 50000, leftOrbit: 30000, rightOrbit: 30000, orrery: 40000, telescope: 50000 },
bonusFrom: { 'standup:*': 'targets', 'pendulum:*': 'targets', 'bumper:*': 'bumpers', 'spinner:*': 'spins' },
bonusCounters: /* same eight as temple */,
topLanes: { switches: ['laneT1','laneT2','laneT3'], lamps: ['laneT1','laneT2','laneT3'], award: 'bonusX' },
lowerLanes: { switches: ['inlaneL','outlaneL','inlaneR','outlaneR'], lamps: [...same], award: 'kickback' },
bonusX: { lamps: ['bonus2x','bonus3x','bonus4x','bonus5x'], max: 5 },
skill: { value: 250000, super: { on: 'hole:telescope', window: 8, value: 1000000 } },
ballSave: { seconds: 8, lamp: 'shootAgain' },
kickback: { device: 'kickbackL', lamp: 'kickbackL', litAtStart: true },
extraBall: { shot: 'telescope', lamp: 'extraBall' },
multiball: { name: 'mode_observatory_mb', balls: 3, ballSave: 10, jackpotShots: ['comet','meridian','orrery','leftOrbit'],
             jackpotBase: 1000000, jackpotStep: 250000, superAfter: 4, superShot: 'telescope', superWindow: 20, addABallShot: 'rightOrbit' },
modeStart: { shot: 'telescope', hole: 'telescope', lamp: 'modeStart' },
modes: [
  { id: 'meteor', name: 'mode_observatory_meteor', hint: 'dmd_hint_ramps', kind: 'count', targets: ['comet','meridian'], count: 5, duration: 40, shotValue: 500000, step: 0, completeValue: 3000000 },
  { id: 'transit', name: 'mode_observatory_transit', hint: 'dmd_hint_sequence', kind: 'sequence', targets: ['leftOrbit','orrery','telescope','rightOrbit'], count: 4, duration: 45, shotValue: 750000, step: 250000, completeValue: 4000000 },
  { id: 'conjunction', name: 'mode_observatory_conjunction', hint: 'dmd_hint_lit', kind: 'roving', targets: SHOTS, count: 4, moveEvery: 5, duration: 45, shotValue: 750000, step: 0, completeValue: 4000000 },
  { id: 'supernova', name: 'mode_observatory_supernova', hint: 'dmd_hint_targets', kind: 'count', targets: ['bumper:*','standup:*','pendulum:*'], count: 20, duration: 35, shotValue: 100000, step: 0, completeValue: 2500000, gi: 'dim' }
],
wizard: { id: 'wizard', name: 'mode_observatory_wizard', balls: 4, ballSave: 20, jackpotValue: 5000000, step: 1000000, superShot: 'telescope', superAfter: 6 },
combos: { window: 3.0, base: 100000 },
surprises: { secret: ['orrery','telescope','meridian'], key: 'dmd_secret_observatory', value: 750000 },
replay: 40000000
```

**Hooks**

`init`:

```js
{ hour: 0, star: [0,0,0,0], stars: 0, latchUntil: 0, wellsLit: false, wellWindow: 0, hold: null, capture: 0,
  midnightPending: false, discDir: 1, discCmd: null, pendCmd: null, lastTick: -1, mb: false, ebModes: false }
```

**Clock**
- Each shot advances the hour by +1; telescope by +2.
- A `pendulum` event advances by +1, at most once per 0.6 s (`lastTick`).
- No advance while `ctx.mbActive()`.
- On each advance: `ctx.dmd('big', { text: 'dmd_observatory_hour', vars: { n: hour } }, 900, 3)`.
- When `hour >= 12`, start midnight unless the mode select is open (then set `midnightPending` and start in `onTick` once `!g.select`). Midnight does: `ctx.startMultiball()`, `hour = 0`, big `dmd_observatory_midnight`, `show('ring')`, `cue('multiball')`.

**S-T-A-R** (standup events `starS`, `starT`, `starA`, `starR`): when all four are lit:
- reset them;
- 100,000 points; `ctx.lightModeStart()`; clock +1;
- `wellsLit = true`, `wellWindow = now + 10`, magnets `grab` on both wells;
- `latchUntil = now + 12`;
- big `dmd_observatory_star`, then `dmd_observatory_wells` and `dmd_observatory_latched`.

**Wells**
- On a magnet event `held` for a well, set `hold = { id, at: now + 0.8 }`.
- In `onTick`, when the hold is due: throw with `angle = atan2(0.640 − wy, 0.045 − wx)` and speed 1.6; set `capture = now + 2`.
- When the window expires (not during conjunction), turn the magnets off.
- During conjunction the magnets stay in `grab` (assist).
- A `hole:telescope` event while `capture > now` pays 1,000,000 with `dmd_observatory_slingshot` and `cue('jackpot')`.

**Disc** (command only on change, `t.discCmd`):

| State | rad/s × discDir |
|---|---|
| wizard | 16 |
| multiball | 10 |
| meteor | 12 |
| transit | 6 |
| conjunction | 2 |
| supernova | 9 |
| idle | 4 |

The orrery shot sets `discDir *= -1` and shows big `dmd_observatory_reverse`.

**Pendulum** (command only on change): wizard or `latchUntil > now` → `park`; transit → `fast`; otherwise `swing`.

**Mode extra ball:** after 2 completed modes, as in temple and leviathan.

**Spinner** during meteor: +4,000 points each.

**`onMultiball`:** `t.mb = true`; flash `flTelescope`; say `multiball`.

**`onBallStart`:**
- reset the wells, hold, capture and pending state;
- magnets off;
- re-send the disc and pendulum commands;
- the clock persists per player across balls.

**`lamps(ctx, set)`:**
- clock k: `on` if k ≤ hour, `slow` if k = hour + 1; all `fast` during multiball.
- STAR lamps from state.
- Wells: `slow` when lit, `fast` while holding.
- pendulumLit: park → `on`, swing → `slow`, fast → `fast`.
- spinnerLit: `on` in meteor.

**`toy(g)`:** returns C10.

### 5.7 Locale keys (all 16 bundles; English and German given, others translated by the implementer)

| Key | en | de |
|---|---|---|
| `pinball.observatory_description` | Brass, glass and the clockwork sky | Messing, Glas und der Himmel im Uhrwerk |
| `pinball.help_observatory` | Every shot moves the clock forward; at midnight MIDNIGHT MULTIBALL starts. The telescope behind the swinging pendulum starts modes. Complete S-T-A-R to light the gravity wells and latch the pendulum. Shoot the orrery lane to reverse the spinning disc. | Jeder Treffer stellt die Uhr weiter; um Mitternacht startet der MITTERNACHTS-MULTIBALL. Das Teleskop hinter dem schwingenden Pendel startet Modi. S-T-A-R aktiviert die Gravitationsfelder und arretiert das Pendel. Ein Schuss auf die Orrery-Bahn kehrt die Drehscheibe um. |
| `pinball.card_rules_observatory` | SHOTS ADVANCE THE CLOCK\nMIDNIGHT STARTS MULTIBALL\nTELESCOPE STARTS MODES | TREFFER STELLEN DIE UHR\nMITTERNACHT STARTET MULTIBALL\nTELESKOP STARTET MODI |
| `pinball.mode_observatory_meteor` | METEOR SHOWER | METEORSCHAUER |
| `pinball.mode_observatory_transit` | TRANSIT OF VENUS | VENUSTRANSIT |
| `pinball.mode_observatory_conjunction` | GRAND CONJUNCTION | GROSSE KONJUNKTION |
| `pinball.mode_observatory_supernova` | SUPERNOVA | SUPERNOVA |
| `pinball.mode_observatory_wizard` | GRAND ORRERY | GROSSES PLANETARIUM |
| `pinball.mode_observatory_mb` | MIDNIGHT MULTIBALL | MITTERNACHTS-MULTIBALL |
| `pinball.dmd_observatory_hour` | {n} O'CLOCK | {n} UHR |
| `pinball.dmd_observatory_midnight` | MIDNIGHT | MITTERNACHT |
| `pinball.dmd_observatory_star` | S-T-A-R COMPLETE | S-T-A-R KOMPLETT |
| `pinball.dmd_observatory_wells` | GRAVITY WELLS LIT | GRAVITATIONSFELDER AN |
| `pinball.dmd_observatory_slingshot` | GRAVITY SLINGSHOT | SWING-BY |
| `pinball.dmd_observatory_latched` | PENDULUM LATCHED | PENDEL ARRETIERT |
| `pinball.dmd_observatory_reverse` | ORRERY REVERSED | ORRERY UMGEKEHRT |
| `pinball.dmd_secret_observatory` | STARGAZER | STERNENGUCKER |

The title `Clockwork Observatory` is a proper name and stays literal in `layout.meta.title`.

DMD scenes: none of the existing scenes are table-specific. The observatory uses `big` texts and the shared scenes; no new DMD scene is needed.

---

## 6. Assets the main session must deliver

### 6.1 Images (Codex CLI)

All masters go to `assets/pinball/masters/`; prepare.py writes the delivered files to `ui/3d/pinball/v1/`.

| # | File (master) | For | Master format | Delivered | Notes |
|---|---|---|---|---|---|
| 1 | `<id>-playfield.png` | neon, leviathan, temple, observatory | RGB, aspect exactly 1:2 (±0.5%), 2048×4096 preferred, min 1024×2048 | `<id>-playfield.webp` 2048×4096 | Paint over `reports/pinball-guide/<id>-playfield-guide.png` (same mapping). If the generator cannot emit 1:2, make `-top.png` and `-bottom.png` halves (each 2048×2176 aspect, with 128 px overlap, over the guide halves) and run `stitch.py` |
| 2 | `<id>-plastics.png` | all four | **RGBA with real alpha**, 2048×2048 (min 1024×1024), 4×4 cells of 512 px | `<id>-plastics.webp` 2048×2048 RGBA | One plastic piece per cell, inside its outline from the plastics guide (cell = `ATLAS` order in the legend). Alpha 255 for opaque print, 60–140 for clear tinted plastic, 0 outside the outline; 24 px transparent margin per cell |
| 3 | `<id>-apron.png` | all four | RGBA, aspect 4.267:1 (±1%), 2048×480 preferred | `<id>-apron.webp` 2048×480 | Keep the two cyan card rectangles plain dark (cards are drawn on top); alpha 0 in the plunger notch (`x ≥ 0.2235`) |
| 4 | `<id>-side.png` | all four | RGB, aspect 3.103:1 (±1%), 2048×660 preferred | `<id>-side.webp` 2048×660 | Cabinet side panel. Area above the sloped top line (front low, back high) is cropped. On the left panel the back of the machine is the image's left edge; on the right panel it is the right edge. Centre the composition, title optional |
| 5 | `observatory-backglass.png` | observatory | RGB 16:9, 2048×1152 preferred (min 1536×864) | `observatory-backglass.webp` 2048×1152 | Title "CLOCKWORK OBSERVATORY" allowed; no instructions |

Rules for every playfield painting (put these into the prompts):
- Follow the guide exactly. Walls, posts and targets are physical parts, so paint no 3D objects.
- Every magenta shape is an insert: fill it with a flat dark neutral (`#1c1c1c`) with a thin bevel ring, and add no text or glow there.
- Cyan rectangles are text areas: keep them calm.
- Translucent path footprints show ramps above, so art beneath them stays visible.
- No lettering except optional small decorative glyphs. The lower third (flippers, inlanes) stays readable and medium-dark.
- No controls or localized words.
- Keep 4 mm darker margins along walls.

Each image needs a prompt record in `prompts.json` with `guide: true` and the guide file name.

The lobby preview (`observatory-preview.webp`, 640×320) is a game render produced by the survey, not by Codex.

### 6.2 GLB toys (Blender, observatory)

**Common conventions**
- glTF 2.0 binary, Blender glTF exporter:
  - Format GLB; Include selected objects and Custom Properties.
  - Transform +Y Up.
  - Apply Modifiers, UVs and Normals; no tangents unless normal maps are used.
  - No animation, cameras or lights.
  - No Draco or meshopt. Textures JPEG or PNG (WebP allowed).
- **Units and frame.** Metres, scale 1.0, modelled in **table coordinates**: Blender X = playfield x (right), Blender Y = playfield y (toward the backbox), Blender Z = height above the playfield surface. Origin = playfield origin (x 0 on the centre line, y 0 at the playfield's bottom edge, h 0 on the surface). Do **not** model the 6.5° tilt.
- After export, the GLB is placed at identity inside `tableRoot` (glTF X = x, Y = h, Z = −y). Position everything from `reports/pinball-guide/observatory-reference.json`.
- **Animated nodes.** Object origin at the pivot; rotation applied to identity in the rest pose; no non-uniform or negative scale on animated nodes or their parents. Static parts may be joined.
- **Node names** are exact and case-sensitive.
- **Materials.** Principled BSDF (metal and roughness) only, ≤ 6 per GLB, textures ≤ 1024² (≤ 512² for small parts).
  - Emissive parts use materials named `glow_<name>` (the code drives intensity).
  - Glass uses `glass_<name>` (alpha BLEND); everything else OPAQUE or MASK.
- **Clearance.** Every vertex at height ≤ `glassHeight(y) − 0.008` (checked by `test-pinball-assets.mjs`). Nothing below h 0.030 inside ball corridors, except where stated below.
- **Budgets:** ≤ 1.5 MiB per file and ≤ 4 MiB total.
- **Delivery:** `assets/pinball/models/<file>`, `.blend` to `assets/pinball/blender/`, and a record in `assets/pinball/models.json`: `{ table: 'observatory', key, file, sha256, triangles, blend, blendSha256 }`.

| Key / file | Placement (playfield m) | Animated nodes (pivot and axis) | Budget |
|---|---|---|---|
| `orrery` / `observatory-orrery.glb` (armillary sphere) | Plinth centred at (0.212, 1.030), plinth radius ≤ 0.035 on the playfield; sphere centre h 0.082, outer radius ≤ 0.040; total top ≤ 0.128 | `orrery_ring_1`, `orrery_ring_2`, `orrery_ring_3`: spin about their own Blender local Z (= glTF local Y), each parented to a static tilt empty; `orrery_arm_1`..`orrery_arm_4`: spin about the vertical axis through the sphere centre (Blender local Z), planets at their tips; `orrery_sun` with material `glow_sun`; static `orrery_base` | ≤ 12k triangles |
| `telescope` / `observatory-telescope.glb` | Barrel around the `telescopeTube` centreline (path samples + BALL_R up) between about (0.040, 0.705, 0.060) and (0.010, 0.785, 0.064): open bore radius ≥ 0.0205, outer radius ≤ 0.026. Pedestal on the `scopeBase` footprint (0.045, 0.700), r ≤ 0.025. Nothing inside the channel `x 0.021–0.069, y < 0.693` below h 0.035 | `telescope_body` (static); `telescope_cap`: lens cover at the upper barrel end, pivot on its hinge, rotates about Blender local X (closed = 0, code opens to −1.6 rad); lens material `glow_lens` | ≤ 8k triangles |
| `gears` / `observatory-gears.glb` | Upper-left dead corner, inside x ∈ [−0.262, −0.157] and y ∈ [1.000, 1.085], everything at radial distance ≥ 0.268 from (0, 0.8125); vertical gears facing the player; top ≤ 0.125 | `gear_1`..`gear_N` (N 5–7): spin about Blender −Y (toward the player; glTF local +Z), custom property `ratio` (float, signed, `gear_1` = 1.0); `gear_anchor`: escapement anchor, rocks about the same axis | ≤ 10k triangles |
| `pendulum` / `observatory-pendulum.glb` | Gantry beam across the channel mouth at h 0.092–0.100, x 0.015–0.075, y 0.566–0.585; supports at the wall ends (0.021, 0.585) and (0.069, 0.585), r ≤ 0.004 | `pendulum_arm`: origin exactly at the pivot (0.045, 0.568, 0.0885), rod hanging straight down 0.075 to the bob centre, bob a brass sphere or lens of radius 0.011 (child mesh name contains `bob`). The code sets glTF `rotation.z = θ`; positive θ swings the bob toward +x. Rest pose hangs straight down | ≤ 4k triangles |

Missing or failed GLBs fall back to procedural toys. The low tier uses the GLBs without shadows; `glass_*` materials render opaque there.

---

## 7. Open risks and what to verify first

1. **Spin feel (highest risk).** Rolling acceleration at 5/7 changes the whole game.
   - S1 first runs the baseline and post-change metrics and stops for the coordinator's feel check before S2 retunes the layouts.
   - Lever order: `floorMu`, then plunger, then ejects, then flippers.
2. **Crossing separations are tight (37–41 mm)** on neon, temple and observatory, and spline overshoot can tip them over. Fix by lowering the under-path. The validator gives exact numbers.
3. **Leviathan head relocation.** This is a design change made necessary by the glass: the head moves to the corner and the helix circles a coil. Confirm visually in F1. The AGENTS contract on the ship-scoop eject must still pass (layout test).
4. **Exit speeds of lowered returns.** Verify that inlane rollovers are still reached; `speedScale` may go up to 1.0.
5. **Reflection pass cost and shader patch** coexisting with the lamp patch. Measure draw calls and FPS on medium; disable on medium if p95 frame time regresses by more than 2 ms.
6. **Payload budget.** The estimate is about 15 MiB of 20 MiB with all art and GLBs; the test enforces the cap.
7. **Codex aspect ratios.** The generator may only emit 1:1, 2:3 or 3:2. Use the halves and `stitch.py` for playfields and crop masters to the exact aspect before saving them (masters are never cropped by `prepare.py`).
8. **Pendulum soak traps** near the wall ends, and **disc orbiting** (a ball circling forever on the disc). Watch the reach soak (8 s rest limit). If a ball orbits, raise `discMu` or lower the idle speed.
9. **Observatory before its art exists.** The lobby needs the preview `onerror` fallback (R5), and the manifest test allows a subset until F1 tightens it.
10. **GitNexus index unusable** in this checkout. Implementers fall back to Grep and must say so in each hand-back.

---

## 8. Coordinator amendments (2026-11-21) — these OVERRIDE the sections above where they differ

The art stream ran ahead of the plan: most images already exist and were painted over guides the coordinator made. The formats below are final. Where a package section above disagrees with this section, this section wins.

### AM1 — Art formats (replaces C4, C5, C9, R4a steps 1–4, section 6.1, A1–A3)

**Existing source images** (coordinator's working folder `C:\disposable\pinball-r2\art\raw\`, prompts in `C:\disposable\pinball-r2\art\prompts\<name>.txt`, reference guides in `C:\disposable\pinball-r2\art\refs\`):

| Name | Size | Use |
|---|---|---|
| `neon-playfield.png`, `leviathan-playfield.png`, `temple-playfield.png` | 887×1774 RGB | painted playfield underlay, rev 2 (painted over the layout guide) |
| `neon-plastics.png`, `leviathan-plastics.png`, `temple-plastics.png`, `observatory-plastics.png` | 1024×1536 RGBA | plastics sprite sheet over the shared template |
| `neon-sideart.png`, `leviathan-sideart.png`, `temple-sideart.png`, `observatory-sideart.png` | 2172×724 RGB | cabinet side art |
| `observatory-backglass.png` | 1672×941 RGB | backglass |
| `room-wall.png` | 1672×941 RGB | back wall of the room, shared by all tables |

The observatory playfield painting follows after S5 (it needs the layout). There is **no apron image** and **no per-table plastics extras**. `stitch.py` is not needed.

**Guides.** The generators are `C:\disposable\pinball-r2\guide.mjs` (playfield guide, 887×1843 SVG) and `C:\disposable\pinball-r2\plastics-template.mjs` (plastics template, 1024×1536 SVG + JSON), rasterised by `svg2png.mjs`. R4a ports them into the repo as `scripts/pinball-art-guide.mjs` (subcommands `playfield <id|all>` and `plastics`; output to `reports/pinball-guide/`; keep the colour code and geometry unchanged so the existing paintings stay valid; add the `orrery` device to the dashed-circle case and `pendulum` as a dashed line across its swing). No legend, apron guide, side guide or reference JSON.

**Plastics sheet contract (C4, final).** New DOM-free file `tables/kit/plastics.js`:

```js
// Verbatim from C:\disposable\pinball-r2\guides\plastics-template.json. px = [x, y, w, h] in the 1024x1536 sheet (row 0 at the top);
// world = [x0, y0, x1, y1] in metres. For mirror pieces the world rect is the LEFT piece in kit coordinates u = x - cx
// (absolute x for `corner`); the right piece is the mirror image (about x = cx, or about x = 0 for `corner`) with U flipped.
export const SHEET = Object.freeze({ w: 1024, h: 1536, cells: Object.freeze({
    sling:  { px: [48.61, 44, 292.77, 732],      world: [...], mirror: true,  height: 0.034 },
    guide:  { px: [394, 87.08, 272, 645.83],     world: [...], mirror: true,  height: 0.030 },
    corner: { px: [714, 64.92, 272, 410.16],     world: [...], mirror: true,  height: 0.040, absolute: true },
    cap:    { px: [714, 544, 272, 272],          world: [-0.029, -0.029, 0.029, 0.029] },
    sign1:  { px: [113.33, 844, 473.33, 142],    world: [-0.030, -0.009, 0.030, 0.009] },
    sign2:  { px: [113.33, 1024, 473.33, 142],   world: [-0.030, -0.009, 0.030, 0.009] },
    badge:  { px: [144.17, 1214, 735.65, 282],   world: [-0.060, -0.023, 0.060, 0.023] }
}) });
export function cellUV(id, flipU = false)   // -> [u0, v0, u1, v1] for a three.js texture with flipY = true (v0 = bottom)
export function plasticPieces(layout)       // -> [{ id, cell, rect: [x0, y0, x1, y1] (playfield m), h, flipU, posts: [[x, y], ...] }]
```

`plasticPieces` returns, in this order: `slingL`, `slingR`, `guideL`, `guideR`, `cornerL`, `cornerR`, then `badge`. Read the exact `world` numbers from the JSON file. `cx` is `layout.meta.cx`. `posts` are the standoff feet under the piece: the three sling posts for slings; the divider post and two points on the inlane guide for guides; two points near the outer wall for corners; none for the badge. The badge rect is `[-0.068, -0.087, 0.052, -0.041]` at `h = 0.0128` (it lies flat on the apron, see AM5). Bumper caps and ramp signs use `cellUV('cap')`, `cellUV('sign1')`, `cellUV('sign2')` directly (AM6, AM7); they are not in `plasticPieces`.

**Rendering of plastics (replaces R3b step 3).** `render/plastics.js` → `buildPlastics(ctx, image)`: each piece is a flat quad at `piece.h` (three.js X = x, Y = h, Z = -y) with UVs from `cellUV`. One shared material for all pieces: the sheet texture as `map` (sRGB), `alphaTest: 0.5`, not transparent, `side: DoubleSide`, roughness 0.25, clearcoat 0.6 (0 on low), `emissive '#ffffff'`, `emissiveMap = map`, `emissiveIntensity 0.2`. A second quad 0.0022 below each piece with the same map and `color '#3a3a3a'` gives the acrylic edge. Chrome standoffs (r 0.003) from the playfield to the plate at `piece.posts`, each with a screw head on top. When `image` is null, skip plastics entirely (the slings keep their current plates). `buildSling` drops its own flat plate when the sheet image exists.

**Delivered files and manifest (C5, final).** `prepare.py` converts whatever masters exist:

| Role | Master in `assets/pinball/masters/` | Delivered in `ui/3d/pinball/v1/` |
|---|---|---|
| playfield | `<id>-playfield.png` (never upscale) | `<id>-playfield.webp`, q92 |
| backglass | `<id>-backglass.png` | `<id>-backglass.webp`, q92 |
| plastics | `<id>-plastics.png` | `<id>-plastics.webp`, 1024×1536, lossy RGBA q92 with `-alpha_quality 100`; before encoding clean the alpha with Pillow (`py -3.13`): alpha < 40 → 0, alpha ≥ 216 → 255 |
| side | `<id>-side.png` (copied from `<id>-sideart.png`) | `<id>-side.webp`, 2048×682, q90 |
| room | `room-wall.png` | `room-wall.webp`, q88 |

Manifest version 3: table entry `{ id, underlay, backglass, preview?, artRev, plastics?, side?, glb? }` plus a top-level `"room": { "wall": "room-wall.webp" }`. `artRev` is 2 when the playfield prompt record has `"guide": true`, else 1. `glb` is `{ "toys": "observatory-toys.glb" }` on the observatory only (AM2). Tables come from `TABLE_IDS` (import from `config.js`); a table without a delivered underlay is left out of `tables` until its art exists, and the test accepts a subset until F1. The old backglass and concept masters and their records stay. `prompts.json` gets one record per new image: `{ id, kind, prompt, generator: "codex-cli", guide?: true, reference?: "<guide file>" }` with the prompt text taken verbatim from the prompt files. The old playfield masters are replaced (their old prompt records are replaced too).

### AM2 — GLB toys (replaces section 6.2, C7 keys and the GLB parts of R5)

One file: `assets/pinball/models/observatory-toys.glb`, built by the retained script `assets/pinball/blender/observatory_toys.py` (Blender in background mode; the script is the source, no `.blend` is kept). `assets/pinball/models.json` records `{ table: 'observatory', key: 'toys', file, sha256, triangles, script }`. Delivered unchanged as `ui/3d/pinball/v1/observatory-toys.glb`. Budget: ≤ 40k triangles, ≤ 1.5 MiB.

Conventions: metres; Blender X = table x, Y = table y, Z = height, exported +Y up, so glTF X = x, Y = h, Z = -y. **Every toy is one root node whose origin is the toy's base point**; the runtime clones the root and places it. Nothing is animated in the file. Materials are plain PBR values without textures; emissive materials are named `glow_*`, glass `glass_*`.

| Root | Placed by the runtime at | Animated children (glTF axes) |
|---|---|---|
| `orrery` | the `orrery` device centre, h 0 (it stands on the `orreryHub` post in the middle of the disc) | `orrery_arm_1..4`: rotation.y = disc angle × [1.6, 1.0, 0.62, 0.4]; `orrery_rings`: rotation.y slow (0.3 rad/s × disc direction); `orrery_sun` material `glow_sun` |
| `pendulum` | the pendulum device pivot (x, y), h 0 | `pendulum_arm` (its origin is the pivot at h = `pivotH`): rotation.z = θ for axis [1, 0] (positive θ swings the bob toward +x) |
| `telescope` | the `telescope` toy position (the `scopeBase` post) | `telescope_barrel`: the runtime positions its origin (the lower end of the bore axis) at the `telescopeTube` sample at fraction `from` plus BALL_R up, and aims its local −Z (glTF; Blender +Y) at the sample at fraction `to` plus BALL_R. `from`/`to` come from the toy entry. Lens glow material `glow_star` |
| `gears` | the `gears` toy position (on the left back-corner panel, toy `h` 0.043) | `gear_1..5`: rotation.y, stepped like an escapement; ratios [1, -1.76, 1.48, -2.37, -1.28] |
| `dome` | the `dome` toy position (on the right back-corner panel, toy `h` 0.043) | `dome_shutter`: rotation.y 0 (closed) to 0.5 (open) during multiball and wizard; `clock_hand_hour` and `clock_hand_minute`: rotation.z from the clock hour (their local Z points at the player) |

`render/models.js` keeps the C7 API; the map has the single key `toys`, and the runtime looks roots up by name in `nodes`. A missing file or root falls back to the procedural toy for that root. `tables/observatory/toys.js` uses the root names above (not the node names in R5 step 5).

### AM3 — Back-corner panels

All four tables get printed panels over the two back corners (the area outside the top arch), pieces `cornerL`/`cornerR` at h 0.040. Toys that stand in a back corner sit **on** the panel: toy entry `h: 0.043`, top ≤ `glassHeight(y) − 0.008`. This applies to the Leviathan head (R2 step 7: keep the head in the upper-right corner as planned, but standing on the panel) and to the observatory `gears` and `dome` toys.

### AM4 — Observatory layout changes (amends section 5.3)

- Add a hub post in the middle of the disc: `post('orreryHub', -0.040, 0.790, 0.0055, 'metal', 'none')`. The orrery model's spindle stands on it; its arms turn above ball height (h ≥ 0.040, radius ≤ 0.042 including the planets).
- `telescopeTube` (starting values) has a straight inclined section for the telescope barrel and then drops steeply onto the disc rim, outside the reach of the orrery arms:
  `[[0.045,0.640,0],[0.0425,0.648,0.012],[0.0272,0.700,0.039],[0.012,0.752,0.066],[0.004,0.768,0.030],[-0.001,0.776,0]]`, exit `{ kind: 'field', dir: [-0.4, -1], speedScale: 0.3 }`. The landing point must stay on the disc (inside r 0.050 of its centre) and ≥ 0.042 from the disc centre. The barrel is brass (outer radius 0.0225 around the ball centre line), so for this path check the glass with a top allowance of 0.037 instead of `PATH_TOP.tube`: add an optional `top` field on a path def that `pathHalfWidth`/`checkPathFit` honour (S2 owns that small addition; `telescopeTube` sets `top: 0.037`), and `skin: 'telescope'` so the ramp renderer draws no glass tube for it.
- Toys (replaces the toy list): 
  `{ id: 'orrery', kind: 'orrery', x: -0.040, y: 0.790, h: 0, rot: 0, scale: 1 }`,
  `{ id: 'scope', kind: 'telescope', x: 0.045, y: 0.700, h: 0, rot: 0, scale: 1, path: 'telescopeTube', from: 0.10, to: 0.66 }`,
  `{ id: 'gearTrain', kind: 'gears', x: -0.207, y: 1.030, h: 0.043, rot: 0, scale: 1 }`,
  `{ id: 'dome', kind: 'dome', x: 0.207, y: 1.030, h: 0.043, rot: 0, scale: 1 }`,
  `{ id: 'clock', kind: 'clockDial', x: -0.040, y: 0.790, h: 0, rot: 0, scale: 1 }`.
- Toy state C10 stays, with `orrery` meaning the on-disc orrery and `dome: { open /*0..1*/ }` added.

### AM5 — Apron (amends R1 step 5)

No apron image. `apron.js` builds the shaped apron with a dark brushed-metal procedural texture, an accent pinstripe and chrome edge. The two instruction cards become narrower so the middle stays free for the badge: rules card centre (−0.1665, −0.064) size 0.177×0.080, replay card centre (0.1375, −0.064) size 0.151×0.080. The badge itself is drawn by `render/plastics.js` (piece `badge`), not by `apron.js`.

### AM6 — Room and side art (amends R1 steps 4, 7, 9)

`scene.js` loads `entry.side`, `entry.plastics` and the manifest's `room.wall` bitmaps (all optional). The back wall of the room uses the `room.wall` image when present (plane about 5.0 m wide at the image aspect, centred behind the machine, brightness ≈ 0.85 so the table stays the hero) and the procedural wall otherwise. The floor is the procedural parquet. Side art maps the 3:1 image over the bounding rectangle of the trapezoid; on the right panel the image runs front-to-back left-to-right as seen by a viewer standing at that side, on the left panel it is mirrored so it also reads correctly from outside.

### AM7 — Ramp signs and bumper caps (amends R2 step 2 and R3b)

- Ramp entrance signs: instead of the old sign plate or a wall decal, each ramp that is entered from the field (`minSpeed` below 99) gets a small sign plate (0.060×0.018 m) on two thin posts above its entrance, facing the player and tilted back 15°, lower edge 0.036 above the ramp floor. Texture: `cellUV('sign1')` for plastic ramps, `cellUV('sign2')` for wire ramps, from `host.images.plastics`; the procedural sign texture stays as the fallback. R2 imports `cellUV` from `tables/kit/plastics.js` (R4a lands first).
- Bumper caps: when the sheet image exists, R3b adds a printed disc with `cellUV('cap')` on top of every pop bumper cap (alpha-tested, emissive like the plastics, slightly domed is not needed).

### AM8 — Working method

- S1 does not stop for a feel check. It reports the before and after metrics and continues.
- Visual self-check for render packages: a dev server is already running on port 8793. Take a screenshot with
  `node C:/disposable/pinball-r2/shot.mjs --out C:/disposable/pinball-r2/shots/<name>.png --q "table=neon&mode=engine&quality=high&camera=play&scene=play"`
  (other useful queries: `camera=showcase`, `camera=top&scene=top` with `--w 700 --h 1100`, `scene=closeup:x,y,h`, `quality=low`), then look at the PNG with the Read tool. Run one screenshot at a time (each starts a headless Chrome). If the shot is blank, raise `--wait` to 12000. Do not start another server and do not stop this one.
- Another stream works in the same checkout at the same time. Run only the test commands listed for your package. If a suite fails inside files your package does not own, report it and do not fix or revert those files.
- Package order in the render stream is now: R4a → R1 → R2 → R3a → R3b → R4b → R5 (R1 may run in parallel with R4a).
