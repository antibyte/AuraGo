# AuraGolf P1 – Schlag & Physik: Umsetzungsplan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** AuraGolf bekommt einen kräftigen, berechenbaren Arcade-Schlag (28 m volle Stärke, federnde Banden), Stärke nach Bildschirm-Pixeln und eine Laufbahnvorschau bis zur ersten Bande – auf den bestehenden 18 Bahnen.

**Architecture:** Die Physik (`minigolf-physics.js`) ersetzt Luftdämpfung und Kontaktreibung durch ein eigenes Rollmodell (konstanter + geschwindigkeitsabhängiger Widerstand je Untergrund, Bodenerkennung per Ray), federnde Wände, `FIX_INTERNAL_EDGES` und eine Contact Skin. `power` bedeutet künftig „Anteil der 28-m-Rollweite“. Eine lazily erzeugte Geisterwelt (`World.restoreSnapshot`) simuliert die Vorschau mit demselben Rollcode. Die Szene misst die Zugweite in CSS-Pixeln und zeichnet die Vorschau; das Spiel trennt UI-Stärke `strength` von Physik-`power` (`power = strength^1,5`).

**Tech Stack:** Vanilla ES-Module, three.js 0.186.1, Rapier 0.21.0 (`@dimforge/rapier3d-compat`), esbuild-Bundle (`scripts/build-minigolf.mjs`), Node-Testskripte (`node:assert`), Go/rod-Browser-Smoke-Test.

**Spec:** [2026-10-18-p1-schlag-physik-design.md](2026-10-18-p1-schlag-physik-design.md)

---

## Arbeitsumgebung und Regeln

- Arbeite im Worktree `.worktrees/auragolf-p1` (Task 0), nie direkt im Haupt-Checkout: dort arbeiten andere Sessions parallel.
- Nach jeder Code-Änderung committen (Nutzerwunsch). Nur eigene Pfade stagen.
- Vor dem Bearbeiten einer Funktion: GitNexus-Impact ausführen (Projektregel), vor jedem Commit `detect-changes`. Die Befehle stehen in den Tasks. Ein `PARTIAL RESULT` mit fremden Dateien ist kein Fehler deines Commits; prüfe, dass deine eigenen Dateien erscheinen.
- `minigolf-game.js`, `minigolf-scene.js`, `minigolf-physics.js` und `minigolf-views.js` werden ins Bundle `ui/js/vendor/minigolf/game.esm.js` gebaut. Wer eine dieser Dateien ändert, baut im selben Commit neu: `node scripts/build-minigolf.mjs` (nicht `npm run build:minigolf`, das schreibt auch die Musik neu).
- Tests schlank halten: nur die im Plan genannten Prüfungen.

## Dateien

| Datei | Verantwortung | Tasks |
|---|---|---|
| `ui/js/desktop/apps/minigolf-physics.js` | Rollmodell, Wände, Abschuss, `predict` | 1, 2 |
| `scripts/test-minigolf-physics.mjs` | Physik-Abnahme | 1, 2 |
| `scripts/test-minigolf-courses.mjs` | Routen + Suchmodus mit Wartezeiten | 3 |
| `ui/js/desktop/apps/minigolf-scene.js` | Pixel-Stärke, Abbruch, Vorschau-Darstellung | 4 |
| `ui/js/desktop/apps/minigolf-game.js` | `strength` vs. `power`, Vorschau-Takt, Escape | 5 |
| `ui/js/desktop/apps/minigolf-views.js` | Anzeige „35% · ≈6 m“ | 5 |
| `ui/js/vendor/minigolf/game.esm.js`, `manifest.json` | gebautes Bundle | 1, 2, 4, 5 |
| `ui/desktop_minigolf_browser_test.go` | Smoke: Vorschau hat Punkte | 6 |
| `documentation/minigolf.md` | Nutzerdoku | 6 |

---

### Task 0: Worktree anlegen

**Files:** keine Quelländerung

- [ ] **Step 1: Worktree und Branch erstellen**

```bash
cd /c/Users/Andi/Documents/repo/AuraGo
git status --short -- ui/js/desktop/apps/minigolf-*.js scripts/test-minigolf-*.mjs ui/desktop_minigolf_browser_test.go documentation/minigolf.md
git worktree add .worktrees/auragolf-p1 -b feature/auragolf-p1 main
```

Expected: Die `git status`-Zeile gibt nichts aus (keine fremden Änderungen an diesen Dateien). Falls doch: stoppen und den Nutzer fragen, ob eine andere Session daran arbeitet.

- [ ] **Step 2: `node_modules` per Junction verfügbar machen**

```bash
cmd //c mklink /J "C:\\Users\\Andi\\Documents\\repo\\AuraGo\\.worktrees\\auragolf-p1\\node_modules" "C:\\Users\\Andi\\Documents\\repo\\AuraGo\\node_modules"
cd /c/Users/Andi/Documents/repo/AuraGo/.worktrees/auragolf-p1
node scripts/test-minigolf-physics.mjs
```

Expected: `minigolf physics: all checks passed`. Alle weiteren Befehle laufen in `.worktrees/auragolf-p1`.

---

### Task 1: Rollmodell, federnde Wände, Abschuss mit Drall

**Files:**
- Modify: `ui/js/desktop/apps/minigolf-physics.js` (Konstanten oben, `createPhysics` Kollider- und Ballaufbau, `resolveConditions`, `step`, `shoot`)
- Modify: `scripts/test-minigolf-physics.mjs`

- [ ] **Step 1: Impact prüfen**

```bash
node /c/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs impact createPhysics --direction upstream --repo AuraGo
```

Expected: Aufrufer sind `minigolf-game.js` (`loadCourse`) und die Testskripte. Ein Risiko HIGH/CRITICAL im Bericht notieren und im Abschlussbericht nennen.

- [ ] **Step 2: Tests anpassen und neue Abnahmetests schreiben**

In `scripts/test-minigolf-physics.mjs`:

Zeile 2 ersetzen durch:

```js
import { createPhysics, MAX_ROLL, launchSpeedFor, rollDistance, strengthToPower } from '../ui/js/desktop/apps/minigolf-physics.js';
```

In `travelledAt` die Zeile `assert.ok(actual[0] - snapshot.position[0] <= 12 * fixedStep + 1e-4, …)` ersetzen durch:

```js
    assert.ok(actual[0] - snapshot.position[0] <= launchSpeedFor(0.5) * fixedStep + 1e-4, `${fps} Hz render pose stays within one fixed step`);
```

In der Interpolationsschleife `assert.ok(renderPositions[i] - renderPositions[i - 1] < 0.055, …)` ersetzen durch:

```js
    assert.ok(renderPositions[i] - renderPositions[i - 1] < launchSpeedFor(0.5) / 144 + 0.01, '144 Hz interpolation avoids fixed-step position jumps');
```

Die Hopser-Prüfung beim flachen Putt (`firstLaunchStep.position[1] <= 0.1601`) ersetzen durch:

```js
assert.ok(firstLaunchStep.position[1] <= 0.16 + 0.005 + 1e-4, `flat putt launched without a hop beyond the contact skin: y=${firstLaunchStep.position[1]}`);
```

Den gesamten Block ab `const distanceEvents = [];` bis einschließlich der Sand-Zeile `assert.ok(sandDistance < grassDistance * 0.65, …);` löschen und an seiner Stelle einfügen:

```js
// --- Arcade rolling model (P1) ----------------------------------------------
const wideBounds = { min: [-60, -5, -60], max: [60, 10, 60] };
const wideFloor = (surface = 'grass') => ({ type: 'box', position: [0, -0.1, 0], halfExtents: [50, 0.1, 50], surface });
const wide = (extra = {}) => course({ bounds: wideBounds, cup: [55, 0, 55], colliders: [wideFloor()], ...extra });

async function rollOut(definition, direction, power, maxSteps = 2400) {
    const events = [];
    const physics = await createPhysics(definition, { onEvent: event => events.push(event) });
    const start = physics.getBall();
    assert.equal(physics.shoot(direction, power), true);
    const launch = Math.hypot(...physics.getSnapshot().velocity);
    let after100ms = null, maxHop = 0, steps = 0;
    while (steps < maxSteps && !events.some(event => event.type !== 'impact')) {
        physics.step(fixedStep);
        steps++;
        const velocity = physics.getSnapshot().velocity;
        if (steps === 12) after100ms = Math.hypot(velocity[0], velocity[2]);
        maxHop = Math.max(maxHop, Math.abs(physics.getBall()[1] - start[1]));
    }
    const end = physics.getBall();
    physics.dispose();
    return { launch, after100ms, maxHop, time: steps * fixedStep, distance: Math.hypot(end[0] - start[0], end[2] - start[2]), terminal: events.find(event => event.type !== 'impact')?.type };
}

function gridMesh(size = 60, cell = 0.25) {
    const cells = Math.round(size / cell), vertices = [], indices = [];
    for (let i = 0; i <= cells; i++) for (let j = 0; j <= cells; j++) vertices.push(-size / 2 + i * cell, 0, -size / 2 + j * cell);
    for (let i = 0; i < cells; i++) for (let j = 0; j < cells; j++) {
        const a = i * (cells + 1) + j, b = a + 1, c = a + cells + 1, d = c + 1;
        if ((i + j) % 2) indices.push(a, b, d, a, d, c); else indices.push(a, b, c, b, d, c);
    }
    return { type: 'mesh', vertices, indices, surface: 'grass' };
}

let previousSpeed = 0;
for (let i = 1; i <= 20; i++) {
    const launchSpeed = launchSpeedFor(i / 20);
    assert.ok(launchSpeed > previousSpeed, `launch speed rises with power ${i / 20}`);
    previousSpeed = launchSpeed;
}
close(launchSpeedFor(1), 16.77, 0.05, 'full-power launch speed');
close(rollDistance(launchSpeedFor(0.5)), 0.5 * MAX_ROLL, 1e-6, 'launch speed inverts the grass roll distance');
close(strengthToPower(0.5), 0.5 ** 1.5, 1e-12, 'strength curve');
assert.equal(strengthToPower(1), 1);

const full = await rollOut(wide(), [1, 0], 1);
assert.equal(full.terminal, 'rest', 'a full-power flat shot comes to rest');
assert.ok(Math.abs(full.distance - MAX_ROLL) <= MAX_ROLL * 0.1, `full power rolls 28 m ± 10 % (${full.distance.toFixed(2)} m)`);
assert.ok(full.time <= 6, `full-power shot rests within 6 s (${full.time.toFixed(2)} s)`);
assert.ok(full.after100ms >= full.launch * 0.95, `no slide-to-roll loss (${(full.after100ms / full.launch * 100).toFixed(1)} % after 0.1 s)`);
for (const power of [0.1, 0.25, 0.5]) {
    const shot = await rollOut(wide(), [1, 0], power);
    assert.ok(Math.abs(shot.distance - power * MAX_ROLL) <= power * MAX_ROLL * 0.05, `power ${power} rolls ${(power * MAX_ROLL).toFixed(2)} m ± 5 % (${shot.distance.toFixed(2)} m)`);
}
const putt = await rollOut(wide(), [1, 0], 0.01);
assert.ok(putt.distance > 0.1 && putt.distance <= 0.3, `shortest putt rolls 0.1–0.3 m (${putt.distance.toFixed(3)} m)`);

const tessellated = await rollOut(wide({ tee: [-28, 0.16, 0.13], colliders: [gridMesh()] }), [1, 0], 1);
assert.ok(Math.abs(tessellated.distance - full.distance) <= full.distance * 0.05, `trimesh seams keep the roll distance (${tessellated.distance.toFixed(2)} m vs ${full.distance.toFixed(2)} m)`);
assert.ok(tessellated.maxHop <= 0.01, `no seam hops above 1 cm (${(tessellated.maxHop * 100).toFixed(2)} cm)`);

const bankRotation = [0, Math.sin(Math.PI / 8), 0, Math.cos(Math.PI / 8)];
for (const surface of ['wood', 'stone']) {
    const events = [];
    const physics = await createPhysics(wide({ colliders: [wideFloor(), { type: 'box', position: [4, 0.3, 0], halfExtents: [0.1, 0.3, 3], rotation: bankRotation, surface }] }), { onEvent: event => events.push(event) });
    assert.equal(physics.shoot([1, 0], 0.4), true);
    let before = null, contactStep = -1, after = null;
    for (let step = 0; step < 600 && after === null; step++) {
        const velocity = physics.getSnapshot().velocity;
        physics.step(fixedStep);
        if (contactStep < 0 && events.some(event => event.type === 'impact' && event.surface === surface)) {
            before = Math.hypot(velocity[0], velocity[2]);
            contactStep = step;
        } else if (contactStep >= 0 && step >= contactStep + 6) {
            const bounced = physics.getSnapshot().velocity;
            after = Math.hypot(bounced[0], bounced[2]);
        }
    }
    physics.dispose();
    assert.ok(before > 0 && after / before >= 0.65, `45° ${surface} bank keeps ≥ 65 % speed (${(after / before * 100).toFixed(0)} %)`);
}

const gentleEvents = [];
const gentleSlope = await createPhysics(course({
    tee: [1, 0.12, 0],
    colliders: [{ type: 'box', position: [0, -0.05, 0], halfExtents: [5, 0.05, 5], rotation: [0, 0, -Math.sin(0.022), Math.cos(0.022)], surface: 'grass' }]
}), { onEvent: event => gentleEvents.push(event) });
assert.equal(gentleSlope.shoot([0, 1], 0.01), true);
for (let i = 0; i < 360 && !gentleEvents.some(event => event.type === 'rest'); i++) gentleSlope.step(fixedStep);
assert.ok(gentleEvents.some(event => event.type === 'rest'), 'a ball on a 2.5° slope comes to rest');
assert.ok(Math.abs(gentleSlope.getBall()[0] - 1) < 0.2, `2.5° slope holds the ball near x=1 (${gentleSlope.getBall()[0].toFixed(3)})`);
gentleSlope.dispose();

const grassRoll = await rollOut(wide(), [1, 0], 0.5);
const sandRoll = await rollOut(wide({ colliders: [wideFloor('sand')] }), [1, 0], 0.5);
assert.ok(sandRoll.distance < grassRoll.distance * 0.65, `sand materially shortens rolling distance (${sandRoll.distance.toFixed(2)} m vs ${grassRoll.distance.toFixed(2)} m)`);
```

- [ ] **Step 3: Tests laufen lassen, Fehlschlag bestätigen**

Run: `node scripts/test-minigolf-physics.mjs`
Expected: FAIL mit `SyntaxError: The requested module … does not provide an export named 'MAX_ROLL'`.

- [ ] **Step 4: Konstanten und Hilfsfunktionen in `minigolf-physics.js`**

Direkt nach `const GRAVITY = 9.81;` einfügen:

```js
const CONTACT_SKIN = 0.005;
const GROUND_PROBE = BALL_RADIUS + 0.04;
const DOWN = { x: 0, y: -1, z: 0 };
// Flat-grass roll distance of a full-power shot; `power` is a fraction of it.
export const MAX_ROLL = 28;
// Rolling resistance per floor surface: constant deceleration a (m/s²) plus
// speed-proportional drag k (1/s). Grass is the distance reference.
export const ROLLING = Object.freeze({
    grass: Object.freeze({ a: 0.8, k: 0.45 }),
    sand: Object.freeze({ a: 2.6, k: 1.3 }),
    wood: Object.freeze({ a: 0.7, k: 0.42 }),
    stone: Object.freeze({ a: 0.65, k: 0.4 })
});
const FLOOR_SURFACES = new Set(['grass', 'sand']);
const WALL_RESTITUTION = { wood: 0.78, stone: 0.82 };

export function rollDistance(speed, surface = 'grass') {
    const { a, k } = ROLLING[surface] || ROLLING.grass;
    return speed / k - a / (k * k) * Math.log(1 + k * speed / a);
}

export function launchSpeedFor(power) {
    const target = Math.max(0, Math.min(1, power)) * MAX_ROLL;
    let low = 0, high = 60;
    for (let i = 0; i < 48; i++) {
        const middle = (low + high) / 2;
        if (rollDistance(middle) < target) low = middle; else high = middle;
    }
    return (low + high) / 2;
}

// UI strength (drag, slider, keys) to physics power: fine control for short putts.
export const strengthToPower = strength => Math.max(0, Math.min(1, strength)) ** 1.5;
```

Nach der Funktion `contains(bounds, position)` (vor `export async function createPhysics`) einfügen:

```js
const validShot = (direction, power) => finiteVec3([direction?.[0], 0, direction?.[1]])
    && Number.isFinite(power) && power > 0 && power <= 1 && Math.hypot(direction[0], direction[1]) >= 1e-6;

// Rolling without slipping on the floor under the ball. Rapier contact
// friction is disabled on the ball, so this is the only tangential force.
function applyRolling(world, body, collider, surfaceOf) {
    const hit = world.castRayAndGetNormal(new RAPIER.Ray(body.translation(), DOWN), GROUND_PROBE, true, undefined, undefined, collider);
    if (!hit || hit.normal.y < 0.55) return;
    const n = hit.normal;
    const { a, k } = ROLLING[surfaceOf(hit.collider.handle)] || ROLLING.grass;
    const v = body.linvel();
    const along = v.x * n.x + v.y * n.y + v.z * n.z;
    const tx = v.x - along * n.x, ty = v.y - along * n.y, tz = v.z - along * n.z;
    const tangential = Math.hypot(tx, ty, tz);
    if (tangential <= 1e-9) return;
    const scale = Math.max(0, tangential - (a * n.y + k * tangential) * FIXED_STEP) / tangential;
    const rx = tx * scale, ry = ty * scale, rz = tz * scale;
    body.setLinvel({ x: rx + along * n.x, y: ry + along * n.y, z: rz + along * n.z }, true);
    body.setAngvel({ x: (n.y * rz - n.z * ry) / BALL_RADIUS, y: (n.z * rx - n.x * rz) / BALL_RADIUS, z: (n.x * ry - n.y * rx) / BALL_RADIUS }, true);
}

function launch(body, direction, power) {
    const length = Math.hypot(direction[0], direction[1]);
    const dx = direction[0] / length, dz = direction[1] / length, velocity = launchSpeedFor(power);
    body.setBodyType(RAPIER.RigidBodyType.Dynamic, true);
    body.wakeUp();
    body.setLinvel({ x: dx * velocity, y: 0, z: dz * velocity }, true);
    body.setAngvel({ x: dz * velocity / BALL_RADIUS, y: 0, z: -dx * velocity / BALL_RADIUS }, true);
}
```

- [ ] **Step 5: Wände federnd, Trimesh mit Kanten-Fix, Ball ohne Reibung**

In `createPhysics` direkt nach `const surfaceFriction = { … };` einfügen:

```js
    // Floors keep a dead bounce; every other collider (rails, gates) is a wall.
    const asWall = (desc, surface) => FLOOR_SURFACES.has(surface) ? desc
        : desc.setRestitution(WALL_RESTITUTION[surface] ?? 0.78).setRestitutionCombineRule(RAPIER.CoefficientCombineRule.Max);
```

In `addBox` die Zeile `const collider = world.createCollider(desc, body);` ersetzen durch:

```js
        const collider = world.createCollider(asWall(desc, surface || 'grass'), body);
```

Die Trimesh-Zeile `const collider = world.createCollider(RAPIER.ColliderDesc.trimesh(new Float32Array(shape.vertices), new Uint32Array(shape.indices)).setFriction(…)…, fixed);` ersetzen durch:

```js
                const desc = RAPIER.ColliderDesc.trimesh(new Float32Array(shape.vertices), new Uint32Array(shape.indices), RAPIER.TriMeshFlags.FIX_INTERNAL_EDGES)
                    .setFriction(surfaceFriction[shape.surface] ?? 0.75).setRestitution(0.02).setActiveEvents(RAPIER.ActiveEvents.COLLISION_EVENTS);
                const collider = world.createCollider(asWall(desc, shape.surface || 'grass'), fixed);
```

Die beiden Ball-Zeilen (`ballBody = …` und `ballCollider = …`) ersetzen durch:

```js
        ballBody = world.createRigidBody(RAPIER.RigidBodyDesc.dynamic().setTranslation(...course.tee).setLinearDamping(0).setAngularDamping(0).setCcdEnabled(true));
        // Frictionless ball: rolling resistance and spin come from applyRolling.
        // The contact skin stops penetration recovery from braking on trimesh seams.
        ballCollider = world.createCollider(RAPIER.ColliderDesc.ball(BALL_RADIUS).setContactSkin(CONTACT_SKIN).setMass(0.22)
            .setFriction(0).setFrictionCombineRule(RAPIER.CoefficientCombineRule.Min).setRestitution(0.12)
            .setActiveEvents(RAPIER.ActiveEvents.COLLISION_EVENTS), ballBody);
```

- [ ] **Step 6: Rollmodell im Fixschritt, Ruheschwelle, Abschuss**

In `step` den Block

```js
            if (activeShot && [...activeSurfaceContacts.values()].includes('sand')) {
                …
            }
```

vollständig ersetzen durch:

```js
            if (activeShot) applyRolling(world, ballBody, ballCollider, handle => surfaces.get(handle));
```

In `resolveConditions` `speed(angular) < 0.45` ersetzen durch `speed(angular) < 0.075 / BALL_RADIUS + 0.05`.

Die Funktion `shoot` vollständig ersetzen durch:

```js
    function shoot(direction, power) {
        if (disposed || activeShot || shotLocked || !validShot(direction, power)) return false;
        launch(ballBody, direction, power);
        activeShot = true;
        quietSteps = 0;
        shotStartedAt = time;
        return true;
    }
```

- [ ] **Step 7: Tests laufen lassen**

Run: `node scripts/test-minigolf-physics.mjs`
Expected: `minigolf physics: all checks passed`. Referenzwerte aus dem Prototyp: volle Stärke ≈ 28,0 m in ≈ 5,4 s, Banden 86 %/88 %.

Run: `node scripts/test-minigolf-model.mjs && node scripts/test-minigolf-assets.mjs`
Expected: beide PASS. `test-minigolf-courses.mjs` schlägt bis Task 3 erwartungsgemäß fehl (alte Routen).

- [ ] **Step 8: Bundle bauen und committen**

```bash
node scripts/build-minigolf.mjs
node scripts/build-minigolf.mjs --check
node /c/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs detect-changes --scope all --repo AuraGo
git add ui/js/desktop/apps/minigolf-physics.js scripts/test-minigolf-physics.mjs ui/js/vendor/minigolf/game.esm.js ui/js/vendor/minigolf/manifest.json
git commit -m "feat(minigolf): arcade rolling model with bouncy rails

Rolling resistance per surface replaces air damping and contact friction;
the ball launches with rolling spin, rails bounce (wood .78, stone .82),
trimeshes fix internal edges and a 5 mm contact skin keeps seams from
braking. power is now a fraction of the 28 m flat roll.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Expected: Build meldet `AuraGolf runtime built: … bytes` (≤ 5 MiB), `--check` meldet `verified`.

---

### Task 2: Laufbahnvorhersage `predict`

**Files:**
- Modify: `ui/js/desktop/apps/minigolf-physics.js` (`createPhysics`: Zustand, neue Funktion `predict`, `dispose`, Rückgabeobjekt)
- Modify: `scripts/test-minigolf-physics.mjs` (Anhang vor der Schlusszeile)

- [ ] **Step 1: Fehlschlagenden Test schreiben**

In `scripts/test-minigolf-physics.mjs` direkt vor `console.log('minigolf physics: all checks passed');` einfügen:

```js
// --- Trajectory preview -----------------------------------------------------
const pathLength = (points, from = 0) => points.slice(from + 1).reduce((sum, point, index) => sum + Math.hypot(point[0] - points[from + index][0], point[2] - points[from + index][2]), 0);
const wallAhead = () => wide({ colliders: [wideFloor(), { type: 'box', position: [3, 0.3, 0], halfExtents: [0.1, 0.3, 3], surface: 'wood' }] });

const predictEvents = [];
const predicting = await createPhysics(wallAhead(), { onEvent: event => predictEvents.push(event) });
const ballBefore = predicting.getBall(), timeBefore = predicting.getSnapshot().time;
const preview = predicting.predict([1, 0], 0.5);
assert.ok(preview && preview.points.length >= 2, 'preview returns a path');
assert.ok(preview.contactIndex >= 0, 'preview marks the first wall contact');
assert.ok(pathLength(preview.points, preview.contactIndex) <= 1.05, `preview ends ≤ 1 m after the wall (${pathLength(preview.points, preview.contactIndex).toFixed(2)} m)`);
assert.equal(preview.hazard, null);
assert.deepEqual(predicting.getBall(), ballBefore, 'preview leaves the live ball in place');
assert.equal(predicting.getSnapshot().time, timeBefore, 'preview leaves live time untouched');
assert.equal(predictEvents.length, 0, 'preview emits no live events');

async function restAfterShot(physics, events, checkPreview = false) {
    assert.equal(physics.shoot([1, 0], 0.5), true);
    if (checkPreview) assert.equal(physics.predict([1, 0], 0.5), null, 'no preview while a shot is resolving');
    for (let i = 0; i < 2400 && !events.some(event => event.type !== 'impact'); i++) physics.step(fixedStep);
    return physics.getBall();
}
const predictedRest = await restAfterShot(predicting, predictEvents, true);
const freshEvents = [];
const fresh = await createPhysics(wallAhead(), { onEvent: event => freshEvents.push(event) });
const freshRest = await restAfterShot(fresh, freshEvents);
freshRest.forEach((value, axis) => close(value, predictedRest[axis], 1e-9, 'a preview does not change the following live shot'));
predicting.dispose();
fresh.dispose();

const openField = await createPhysics(wide());
const longPreview = openField.predict([1, 0], 1);
assert.equal(longPreview.contactIndex, -1, 'open field has no wall contact');
assert.ok(pathLength(longPreview.points) <= 0.45 * MAX_ROLL + 0.2, `long preview stops at 45 % of the roll (${pathLength(longPreview.points).toFixed(2)} m)`);
openField.dispose();
```

- [ ] **Step 2: Fehlschlag bestätigen**

Run: `node scripts/test-minigolf-physics.mjs`
Expected: FAIL mit `TypeError: predicting.predict is not a function`.

- [ ] **Step 3: Impact prüfen**

```bash
node /c/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs impact createPhysics --direction upstream --repo AuraGo
```

- [ ] **Step 4: `predict` implementieren**

In `createPhysics` nach `let disposed = false;` einfügen:

```js
    let ghost = null;
    const cupOpening = Number.isFinite(course.cupRadius) ? course.cupRadius : 0.32;
```

Vor `function dispose()` einfügen:

```js
    // Simulates the aimed shot in a lazily cloned world with the same rolling
    // code. The live world, time and events stay untouched.
    function predict(direction, power) {
        if (disposed || activeShot || shotLocked || !validShot(direction, power)) return null;
        if (!ghost) {
            const copy = RAPIER.World.restoreSnapshot(world.takeSnapshot());
            copy.timestep = FIXED_STEP;
            ghost = {
                world: copy, events: new RAPIER.EventQueue(true),
                ball: copy.getRigidBody(ballBody.handle), collider: copy.getCollider(ballCollider.handle),
                obstacles: obstacleBodies.map(({ spec, body }) => ({ spec, body: copy.getRigidBody(body.handle) }))
            };
        }
        const { world: copy, events: queue, ball, collider, obstacles } = ghost;
        let atTime = time;
        for (const { spec, body } of obstacles) {
            const pose = obstaclePose(spec, atTime);
            body.setTranslation(vec3(pose.position), true);
            body.setRotation(pose.rotation, true);
        }
        const start = getBall();
        ball.setBodyType(RAPIER.RigidBodyType.Fixed, true);
        ball.setTranslation(vec3(start), true);
        ball.setRotation({ x: 0, y: 0, z: 0, w: 1 }, true);
        launch(ball, direction, power);
        const limit = Math.max(1.5, 0.45 * power * MAX_ROLL);
        const points = [start];
        let last = start, travelled = 0, contactAt = -1, contactIndex = -1, hazard = null, quiet = 0;
        for (let stepIndex = 0; stepIndex < 360; stepIndex++) {
            atTime += FIXED_STEP;
            for (const { spec, body } of obstacles) {
                const pose = obstaclePose(spec, atTime);
                body.setNextKinematicTranslation(vec3(pose.position));
                body.setNextKinematicRotation(pose.rotation);
            }
            copy.step(queue);
            let touchedWall = false;
            queue.drainCollisionEvents((first, second, started) => {
                const other = first === collider.handle ? second : second === collider.handle ? first : null;
                if (started && other !== null && !FLOOR_SURFACES.has(surfaces.get(other))) touchedWall = true;
            });
            applyRolling(copy, ball, collider, handle => surfaces.get(handle));
            const position = array3(ball.translation());
            travelled += Math.hypot(position[0] - last[0], position[1] - last[1], position[2] - last[2]);
            last = position;
            const previous = points[points.length - 1];
            if (touchedWall && contactAt < 0) {
                contactAt = travelled;
                points.push(position);
                contactIndex = points.length - 1;
            } else if (Math.hypot(position[0] - previous[0], position[2] - previous[2]) >= 0.08) points.push(position);
            if ((course.water || []).some(volume => contains(volume, position))) { hazard = 'water'; break; }
            if (!contains(course.bounds, position)) { hazard = 'out'; break; }
            if (Math.hypot(position[0] - course.cup[0], position[2] - course.cup[2]) < cupOpening) break;
            if ((contactAt >= 0 && travelled - contactAt >= 0.9) || travelled >= limit) break;
            quiet = speed(ball.linvel()) < 0.075 ? quiet + 1 : 0;
            if (quiet >= 30) break;
        }
        if (points[points.length - 1] !== last) points.push(last);
        ball.setBodyType(RAPIER.RigidBodyType.Fixed, true);
        return { points, contactIndex, hazard };
    }
```

In `dispose()` vor `events.free();` einfügen:

```js
        if (ghost) { ghost.events.free(); ghost.world.free(); ghost = null; }
```

Rückgabezeile ersetzen durch:

```js
    return { step, shoot, predict, setBall, getBall, getSnapshot, getObstacles, resetTime, dispose };
```

- [ ] **Step 5: Tests laufen lassen**

Run: `node scripts/test-minigolf-physics.mjs`
Expected: `minigolf physics: all checks passed`.

- [ ] **Step 6: Bundle bauen und committen**

```bash
node scripts/build-minigolf.mjs
node scripts/build-minigolf.mjs --check
node /c/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs detect-changes --scope all --repo AuraGo
git add ui/js/desktop/apps/minigolf-physics.js scripts/test-minigolf-physics.mjs ui/js/vendor/minigolf/game.esm.js ui/js/vendor/minigolf/manifest.json
git commit -m "feat(minigolf): ghost-world trajectory prediction

predict() clones the live world once per course and resimulates the aimed
shot with the shared rolling code until the first wall contact plus
0.9 m, 45 % of the roll, a hazard, the cup or rest.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Routen neu aufzeichnen (Suchmodus mit Wartezeiten)

**Files:**
- Modify: `scripts/test-minigolf-courses.mjs` (neue Funktionen `replay`, `timedRoute`; Routenwahl; `ROUTES`)

- [ ] **Step 1: Suchmodus um Wiederholungsprüfung und Wartezeiten erweitern**

Nach der Funktion `findCupPath` einfügen:

```js
// Replays a route with continuous obstacle time, like the real game. With
// `period`, each shot first waits until the gates return to phase 0 (the
// state the search assumed for every shot) and records that wait in the route.
function replay(physics, course, path, events, period = 0) {
    let position = course.tee;
    physics.resetTime();
    for (let index = 0; index < path.length; index++) {
        if (period) {
            const wait = Math.round((period - physics.getSnapshot().time % period) % period * 120) / 120;
            if (wait > 0) path[index].wait = wait; else delete path[index].wait;
        }
        for (let tick = 0; tick < Math.round((path[index].wait || 0) * 120); tick++) physics.step(1 / 120);
        const terminal = runShot(physics, position, path[index], events, { resetTime: false });
        if (!terminal || terminal.type !== (index === path.length - 1 ? 'cup' : 'rest')) return false;
        position = terminal.position;
    }
    return true;
}

// The search starts every shot at obstacle time 0. Routes behind a moving
// gate therefore get phase-aligned waits (whole periods of 2π/speed).
function timedRoute(physics, course, path, events) {
    if (!path) return null;
    const route = path.map(shot => ({ ...shot }));
    if (replay(physics, course, route, events)) return route;
    const periods = [...new Set((course.obstacles || []).map(obstacle => 2 * Math.PI / obstacle.speed))];
    if (periods.length !== 1 || !replay(physics, course, route, events, periods[0])) return null;
    return replay(physics, course, route, events) ? route : null;
}
```

Die Zeile

```js
        const route = process.env.AURAGO_MINIGOLF_SEARCH === '1' ? findCupPath(physics, course, events) : ROUTES[course.id];
```

ersetzen durch:

```js
        const route = process.env.AURAGO_MINIGOLF_SEARCH === '1' ? timedRoute(physics, course, findCupPath(physics, course, events), events) : ROUTES[course.id];
```

- [ ] **Step 2: Suche ausführen**

Run: `AURAGO_MINIGOLF_SEARCH=1 node scripts/test-minigolf-courses.mjs`
Expected: 18 Zeilen `…: reachable in N shot(s) [ … ]` und am Ende `minigolf courses: all 18 manifest courses loaded …`. Laufzeit ca. 35 s. Bei garden-06 und coast-11 erscheinen `wait`-Felder (Prototyp: garden-06 3 Schläge mit 6,06 s und 7,26 s Wartezeit, coast-11 2 Schläge mit 4,29 s).

- [ ] **Step 3: `ROUTES` ersetzen**

Ersetze den gesamten Inhalt des Objekts `const ROUTES = { … };` durch die 18 ausgegebenen JSON-Arrays, je Zeile `'<course-id>': <array>,` in Bahnreihenfolge, letzte Zeile ohne Komma. Die Zahlen werden unverändert aus der Ausgabe übernommen.

- [ ] **Step 4: Aufgezeichnete Routen prüfen**

Run: `node scripts/test-minigolf-courses.mjs`
Expected: 18 Zeilen `recorded route reaches the cup …` und `minigolf courses: all 18 manifest courses loaded and accepted grounded tee shots`.

- [ ] **Step 5: Committen**

```bash
node /c/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs detect-changes --scope all --repo AuraGo
git add scripts/test-minigolf-courses.mjs
git commit -m "test(minigolf): re-record routes for the arcade physics

Search mode now replays each found route with continuous obstacle time
and searches waits up to one full gate period for moving obstacles.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Szene – Stärke nach Pixeln, Abbruch, Vorschau-Linie

**Files:**
- Modify: `ui/js/desktop/apps/minigolf-scene.js` (Zielhilfe-Objekte, `updateAim`, Eingabe `down`/`move`/`up`, neue `ballScreen`/`dragDirection`/`cancelDrag`/`setPreview`, `stats`, Rückgabeobjekt)

Die Szene hat keinen Node-Test (WebGL); geprüft wird in Task 6 per Browser-Smoke-Test.

- [ ] **Step 1: Impact prüfen**

```bash
node /c/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs impact createScene --direction upstream --repo AuraGo
```

- [ ] **Step 2: Pfeil und gerade Ziellinie durch Vorschau-Linien ersetzen**

Oberhalb von `export function createScene` einfügen:

```js
const PREVIEW_POINTS = 400;
const DRAG_DEAD_ZONE = 14;

function dashedLine(opacity) {
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.BufferAttribute(new Float32Array(PREVIEW_POINTS * 3), 3));
    geometry.setDrawRange(0, 0);
    const line = new THREE.Line(geometry, new THREE.LineDashedMaterial({ color: 0xdf914f, transparent: true, opacity, dashSize: .26, gapSize: .18, depthWrite: false }));
    line.visible = false;
    line.frustumCulled = false;
    return line;
}

function fillLine(line, points, from, to) {
    const positions = line.geometry.attributes.position;
    const count = Math.max(0, Math.min(PREVIEW_POINTS, to - from));
    for (let i = 0; i < count; i++) {
        const point = points[from + i];
        positions.setXYZ(i, point[0], point[1] - .145, point[2]);
    }
    positions.needsUpdate = true;
    line.geometry.setDrawRange(0, count);
    line.computeLineDistances();
    return count >= 2;
}
```

In `createScene` die Zeilen von `const aim = new THREE.ArrowHelper(…)` bis einschließlich `aimLine.frustumCulled = false;` ersetzen durch:

```js
    const previewLine = dashedLine(.9), previewTail = dashedLine(.45);
    const previewEnd = new THREE.Mesh(new THREE.SphereGeometry(.07, 12, 8), new THREE.MeshBasicMaterial({ color: 0xffedb3, transparent: true, opacity: .9, depthWrite: false }));
    previewEnd.visible = false;
    let preview = null, previewDirty = false, lastPreviewPower = -1;
    const forward = new THREE.Vector3();
```

`scene.add(aimLine, powerRing, restMark, blob);` ersetzen durch:

```js
    scene.add(previewLine, previewTail, previewEnd, powerRing, restMark, blob);
```

- [ ] **Step 3: `updateAim` ersetzen und `setPreview` ergänzen**

Die Funktion `updateAim` vollständig ersetzen durch:

```js
    function updateAim() {
        const aiming = canAim && !cameraMode;
        powerRing.visible = aiming;
        const points = aiming && preview?.points?.length >= 2 ? preview.points : null;
        if (points && (previewDirty || lastPreviewPower !== aimPower)) {
            const split = preview.contactIndex >= 0 ? preview.contactIndex + 1 : points.length;
            const tint = powerColor(aimPower);
            previewLine.userData.ready = fillLine(previewLine, points, 0, split);
            previewTail.userData.ready = fillLine(previewTail, points, Math.max(0, split - 1), points.length);
            previewLine.material.color.copy(tint);
            previewTail.material.color.copy(tint);
            const end = points[points.length - 1];
            previewEnd.position.set(end[0], end[1] - .1, end[2]);
            previewEnd.material.color.setHex(preview.hazard ? 0xff6b5e : 0xffedb3);
            previewDirty = false;
            lastPreviewPower = aimPower;
        }
        previewLine.visible = !!points && !!previewLine.userData.ready;
        previewTail.visible = !!points && !!previewTail.userData.ready;
        previewEnd.visible = !!points;
        if (aiming) {
            const ground = ball.position.y - .155;
            powerRing.position.set(ball.position.x, ground + .012, ball.position.z);
            powerRing.scale.setScalar(.4 + 1.05 * aimPower);
            powerRing.material.color.copy(powerColor(aimPower));
        }
        if (putter) {
            putter.visible = canAim || swing > 0;
            putter.position.copy(ball.position).add(new THREE.Vector3(-aimDirection[0] * (.38 + swing * .25), -.13, -aimDirection[1] * (.38 + swing * .25)));
            putter.rotation.y = Math.atan2(aimDirection[0], aimDirection[1]);
        }
    }
    function setPreview(result) {
        preview = result && Array.isArray(result.points) ? result : null;
        previewDirty = true;
        updateAim();
    }
```

- [ ] **Step 4: Eingabe auf Pixel-Stärke umstellen**

Die Funktionen `down`, `move` und `up` vollständig ersetzen durch:

```js
    function ballScreen() {
        const rect = canvas.getBoundingClientRect(), projected = ball.position.clone().project(camera);
        return { x: rect.left + (projected.x + 1) * rect.width / 2, y: rect.top + (1 - projected.y) * rect.height / 2, width: rect.width, height: rect.height };
    }
    function dragDirection(event, screen) {
        if (pointAt(event)) {
            const x = ball.position.x - hit.x, z = ball.position.z - hit.z, length = Math.hypot(x, z);
            if (length > 1e-4) return [x / length, z / length];
        }
        // Ground ray missed: map the screen pull-back through the camera yaw.
        camera.getWorldDirection(forward);
        forward.y = 0;
        if (forward.lengthSq() < 1e-8) return null;
        forward.normalize();
        const sx = screen.x - event.clientX, sy = screen.y - event.clientY;
        const x = sx * -forward.z - sy * forward.x, z = sx * forward.x - sy * forward.z, length = Math.hypot(x, z);
        return length > 1e-6 ? [x / length, z / length] : null;
    }
    function cancelDrag() {
        if (pointerId === null) return false;
        if (canvas.hasPointerCapture(pointerId)) canvas.releasePointerCapture(pointerId);
        pointerId = null;
        dragged = false;
        return true;
    }
    function down(event) {
        if (event.button !== 0 || !canAim || cameraMode) return;
        if (event.pointerType !== 'touch') {
            const screen = ballScreen();
            if (Math.hypot(event.clientX - screen.x, event.clientY - screen.y) > 46) return;
        }
        pointerId = event.pointerId; dragged = false; canvas.setPointerCapture(pointerId); event.preventDefault(); canvas.focus();
    }
    function move(event) {
        if (pointerId !== event.pointerId) return;
        if (event.pointerType === 'mouse' && (event.buttons & 2)) { cancelDrag(); return; }
        const screen = ballScreen();
        const pixels = Math.hypot(event.clientX - screen.x, event.clientY - screen.y);
        dragged = pixels >= DRAG_DEAD_ZONE;
        if (!dragged) return;
        const span = Math.max(140, Math.min(320, .3 * Math.min(screen.width, screen.height)));
        const direction = dragDirection(event, screen);
        if (!direction) return;
        setAim(direction, Math.min(1, (pixels - DRAG_DEAD_ZONE) / (span - DRAG_DEAD_ZONE)), true);
        options.onAim?.(aimDirection, aimPower);
    }
    function up(event) {
        if (pointerId !== event.pointerId) return;
        const shot = event.type === 'pointerup' && dragged && aimPower > 0 && canAim;
        cancelDrag();
        if (shot) options.onShot?.(aimDirection, aimPower);
    }
```

- [ ] **Step 5: `stats` und Rückgabeobjekt erweitern**

In `stats()` nach `geometries: renderer.info.memory.geometries,` einfügen: `preview: preview?.points?.length || 0,`

In der Rückgabezeile `canvas, load, update, setBall, setGhosts, setAim, focusBall, burst, setQuality, frameTurn, setRestMark, panAt,` ergänzen um `setPreview, cancelDrag,`.

- [ ] **Step 6: Syntax prüfen, Bundle bauen, committen**

```bash
node --check ui/js/desktop/apps/minigolf-scene.js
node scripts/build-minigolf.mjs
node scripts/build-minigolf.mjs --check
node /c/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs detect-changes --scope all --repo AuraGo
git add ui/js/desktop/apps/minigolf-scene.js ui/js/vendor/minigolf/game.esm.js ui/js/vendor/minigolf/manifest.json
git commit -m "feat(minigolf): pixel-based drag strength and preview line

Drag strength follows CSS pixels from the ball (14 px dead zone, span
30 % of the canvas, 140-320 px); right button or releasing in the dead
zone cancels. The arrow and straight guide give way to a dashed preview
path that dims after the first wall contact.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Expected: `node --check` ohne Ausgabe, Build `verified`.

---

### Task 5: Spiel – Stärke vs. Power, Vorschau-Takt, Escape, Anzeige

**Files:**
- Modify: `ui/js/desktop/apps/minigolf-game.js` (Import, Zustandsvariablen, `updateAim`, `setTurn`, `loadCourse`, `shoot`, `input`, `key`, `frame`)
- Modify: `ui/js/desktop/apps/minigolf-views.js:118` (`power`)

- [ ] **Step 1: Impact prüfen**

```bash
node /c/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs impact mountGame --direction upstream --repo AuraGo
```

- [ ] **Step 2: Import und Zustand**

Zeile 2 ersetzen durch:

```js
import { createPhysics, strengthToPower, MAX_ROLL } from './minigolf-physics.js';
```

In Zeile 22 `power = .35` durch `strength = .35` ersetzen und direkt darunter einfügen:

```js
    // strength is the UI value (drag, slider, keys); physics power = strengthToPower(strength).
    let previewAt = 0, previewKey = '';
```

- [ ] **Step 3: `updateAim` ersetzen und Vorschau-Takt ergänzen**

Die Funktion `updateAim` vollständig ersetzen durch:

```js
    function showStrength() { view.power(strength, strengthToPower(strength) * MAX_ROLL); }
    function updateAim() {
        const enabled = allowAim();
        scene?.setAim(direction(), strength, enabled); showStrength();
        if (!enabled) { scene?.setPreview?.(null); previewKey = ''; }
        root.querySelector('[data-action="shoot"]').disabled = !enabled;
        root.querySelector('[data-power]').disabled = !enabled;
    }
    // Recompute at most every 66 ms while the aim changes, every 150 ms on
    // courses with moving obstacles, and never for an unchanged static aim.
    function refreshPreview() {
        if (!scene?.setPreview || !physics || cameraMode || !allowAim()) return;
        const dir = snapTowardCup(direction()), ball = physics.getBall();
        const key = [dir[0], dir[1], strength, ball[0], ball[2]].map(value => value.toFixed(3)).join(',');
        const interval = key !== previewKey ? 66 : course?.obstacles?.length ? 150 : Infinity;
        const now = performance.now();
        if (now - previewAt < interval) return;
        previewAt = now; previewKey = key;
        scene.setPreview(physics.predict(dir, strengthToPower(strength)));
    }
```

- [ ] **Step 4: Übrige Stellen auf `strength` umstellen**

In `setTurn`: `power = .35;` → `strength = .35;`

In `loadCourse` (Szenen-Optionen) `onAim(dir, amount) { angle = Math.atan2(dir[0], -dir[1]); power = amount; view.power(power); },` ersetzen durch:

```js
                onAim(dir, amount) { angle = Math.atan2(dir[0], -dir[1]); strength = amount; showStrength(); },
```

In `loadCourse` nach `physics?.dispose(); physics = null; course = next;` einfügen: `previewKey = '';`

Die Funktion `shoot` vollständig ersetzen durch:

```js
    function shoot(dir = direction(), amount = strength) {
        if (!allowAim() || cameraMode || !visible || context.isActive?.() === false) return;
        dir = snapTowardCup(dir);
        if (!physics.shoot(dir, strengthToPower(amount))) return;
        if (!beginShot(round)) { physics.setBall(round.players[round.turnIndex].position); return; }
        angle = Math.atan2(dir[0], -dir[1]); strength = amount; busy = true; clearHeld();
        const ballPosition = physics.getBall();
        audio.effect('strike', { speed: amount * 12, pan: scene?.panAt?.(ballPosition) ?? 0 });
        scene.burst('strike', ballPosition, amount * 2);
        scene.setRestMark?.(null);
        view.notice('rolling'); view.hud(round, course, busy); updateAim();
    }
```

In `input`: `if (element.matches('[data-power]')) { power = Number(element.value) / 100; updateAim(); }` → `if (element.matches('[data-power]')) { strength = Number(element.value) / 100; updateAim(); }`

In `key` die beiden Zeilen für `up`/`down` ersetzen durch:

```js
        if (name === 'up') strength = Math.min(1, strength + (fine ? .005 : .02));
        if (name === 'down') strength = Math.max(.02, strength - (fine ? .005 : .02));
```

In `key` im Escape-Zweig direkt nach `if (event.type !== 'keydown') return;` einfügen:

```js
            if (scene?.cancelDrag?.()) { event.preventDefault(); return; }
```

In `frame`: `if (lift) power = Math.max(.02, Math.min(1, power + lift * .55 * dt));` → `if (lift) strength = Math.max(.02, Math.min(1, strength + lift * .55 * dt));`

In `frame` direkt nach dem Block `if ((held.left || …) { … }` einfügen:

```js
        if (!busy && !panelKind && !loading) refreshPreview();
```

Danach darf `power` in `minigolf-game.js` nur noch in `view.power`, `[data-power]` und `strengthToPower` vorkommen:

Run: `grep -n "\bpower\b" ui/js/desktop/apps/minigolf-game.js`
Expected: genau vier Treffer – die Kommentarzeile „strength is the UI value …“, `showStrength` (`view.power`), sowie zweimal `[data-power]` (`updateAim`, `input`).

- [ ] **Step 5: Anzeige in `minigolf-views.js`**

Zeile 118 (`power(value) { … },`) ersetzen durch:

```js
        power(value, meters) {
            const percent = Math.round(value * 100);
            root.querySelector('[data-power]').value = percent;
            const distance = Number.isFinite(meters) ? (meters < 10 ? Math.round(meters * 2) / 2 : Math.round(meters)) : null;
            root.querySelector('[data-power-value]').textContent = percent + '%' + (distance === null ? '' : ' · ≈' + distance.toLocaleString(document.documentElement.lang || undefined) + ' m');
        },
```

- [ ] **Step 6: Syntax, Node-Tests, Bundle, Commit**

```bash
node --check ui/js/desktop/apps/minigolf-game.js
node --check ui/js/desktop/apps/minigolf-views.js
node scripts/test-minigolf-model.mjs
node scripts/test-minigolf-physics.mjs
node scripts/test-minigolf-courses.mjs
node scripts/build-minigolf.mjs
node scripts/build-minigolf.mjs --check
node /c/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs detect-changes --scope all --repo AuraGo
git add ui/js/desktop/apps/minigolf-game.js ui/js/desktop/apps/minigolf-views.js ui/js/vendor/minigolf/game.esm.js ui/js/vendor/minigolf/manifest.json
git commit -m "feat(minigolf): wire strength curve, preview and drag cancel

The UI keeps a strength value and converts it with strengthToPower; the
aim preview is throttled to 66 ms on change and 150 ms for moving
obstacles, Escape cancels an active drag, and the power readout shows
the flat roll estimate.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Expected: alle drei Node-Tests PASS, Build `verified`.

---

### Task 6: Browser-Smoke, Dokumentation, Gesamtprüfung

**Files:**
- Modify: `ui/desktop_minigolf_browser_test.go` (vor dem Tipp-/Zieh-Test)
- Modify: `documentation/minigolf.md` (Abschnitt „Playing“ und „Assets and builds“)

- [ ] **Step 1: Smoke-Prüfung für die Vorschau ergänzen**

In `ui/desktop_minigolf_browser_test.go` direkt nach

```go
	page.MustElement(`[data-action="focus"]`).MustClick()
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,180))`)
```

einfügen:

```go
	page.Timeout(10 * time.Second).MustWait(`()=>(golfFixture.inspect().graphics?.preview||0)>=2`)
```

- [ ] **Step 2: Dokumentation aktualisieren**

In `documentation/minigolf.md`, Abschnitt „Playing“, den Satz

```
Pull back from the ball and
release to putt; a dashed guide line, a color-graded power ring at the ball
and the strength slider show the shot.
```

ersetzen durch:

```
Pull back from the ball and
release to putt. Strength follows the on-screen drag distance (independent of
camera zoom); releasing next to the ball, a right click or Escape cancels the
drag. A dashed preview path shows the roll up to the first rail and a short
stretch of the rebound, a color-graded ring marks the strength, and the
strength readout adds the estimated flat roll distance (full strength rolls
28 m on flat felt).
```

Im Abschnitt „Assets and builds“ nach dem Satz, der mit `The isolated renderer uses Three.js 0.186.1 and Rapier 0.21.0` beginnt und mit `bounded catch-up work.` endet, einfügen:

```
The ball rolls with its own model instead of Rapier contact friction: a
constant plus a speed-proportional deceleration per floor surface, rolling
spin from launch, rails that bounce (wood 0.78, stone 0.82), trimesh
internal-edge fixing and a 5 mm contact skin so seams do not brake the ball.
```

- [ ] **Step 3: Gesamte Node-Suite**

Run: `npm run test:minigolf`
Expected: alle fünf Skripte PASS (`test-minigolf-audio.mjs` baut die Musik neu; danach `git status --short ui/3d/minigolf/music` prüfen – erwartet: keine Änderung).

- [ ] **Step 4: Go-Tests ohne Browser**

Run: `go test ./ui -run '^TestDesktopMinigolf(RegistrationAndLocalResources|Locales)$' -count=1`
Expected: `ok`.

- [ ] **Step 5: Browser-Smoke**

Run: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '^TestDesktopMinigolfBrowser$' -count=1 -timeout 20m`
Expected: `ok`. Bei Fehlern im Layout (420 px) prüfen, ob die längere Stärkeanzeige „35% · ≈6 m“ überläuft; dann in `ui/css/desktop-app-minigolf.css` das `output[data-power-value]` mit `white-space: nowrap; font-variant-numeric: tabular-nums;` versehen und den Test wiederholen.

- [ ] **Step 6: Committen**

```bash
node /c/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs detect-changes --scope all --repo AuraGo
git add ui/desktop_minigolf_browser_test.go documentation/minigolf.md
git commit -m "docs(minigolf): arcade shot controls and preview smoke check

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

(Falls Step 5 die CSS-Datei geändert hat, `ui/css/desktop-app-minigolf.css` mit stagen.)

---

### Task 7: Auf `main` landen

**Files:** keine

- [ ] **Step 1: Auf aktuellen `main` rebasen und erneut prüfen**

```bash
git -C /c/Users/Andi/Documents/repo/AuraGo branch --show-current
git rebase main
node scripts/build-minigolf.mjs --check
npm run test:minigolf
```

Expected: Haupt-Checkout steht auf `main`; Rebase ohne Konflikte; Bundle `verified`; Tests PASS. Ist der Haupt-Checkout nicht auf `main`: stattdessen `git fetch . feature/auragolf-p1:main` aus dem Worktree verwenden (nur Fast-Forward).

- [ ] **Step 2: Fast-Forward in den Haupt-Checkout**

```bash
git -C /c/Users/Andi/Documents/repo/AuraGo merge --ff-only feature/auragolf-p1
git -C /c/Users/Andi/Documents/repo/AuraGo log --oneline -6
```

Expected: Fast-Forward ohne „local changes would be overwritten“. Bei dieser Meldung: nicht erzwingen, sondern den Nutzer informieren (eine andere Session hat dieselben Dateien offen).

- [ ] **Step 3: Aufräumen**

```bash
cmd //c rmdir "C:\\Users\\Andi\\Documents\\repo\\AuraGo\\.worktrees\\auragolf-p1\\node_modules"
git -C /c/Users/Andi/Documents/repo/AuraGo worktree remove .worktrees/auragolf-p1
git -C /c/Users/Andi/Documents/repo/AuraGo branch -d feature/auragolf-p1
```

Expected: Junction entfernt (nicht das Ziel!), Worktree und Branch gelöscht. Nicht pushen.
