## Part R: Reconcile the player with the real starter catalog

Parts C and D were drafted in parallel: C built the player against the stub catalog `internal/storymaker/testdata/catalog-mini/`, D built the real catalog against its own SVG renderer of contract section 3a. This part renders D's real catalog with C's player, fixes every place where the two disagreed, and proves that A, C and D fit together: a Node test over the real catalog, a Go guard for the catalog and the fixture stories, and a browser test that renders a cast sheet story and `firewall.json` in 16:9, 9:16 and 1:1 through the real render page and writes PNG frames for a visual comparison with D's review sheets. Nothing at the contract section 6, 7 and 8 interfaces changes.

Execute it after C12 (C's last task), D8 and A8. Every task was replayed in a scratch tree that holds the wave-1 state of parts C and D: its new checks fail first (R1 only removes random failures and has none), the whole suite is green afterwards (pass counts 53, 56, 57, 58, 59, 61, 64, 71; part C ends with 53) and the end state is byte-identical to the verified worktree (117 files compared). Before each commit run the repository's graph check (`detect_changes`, CLAUDE.md); the edited functions (`rigState`, `layout`, `draw`, `handsAt`, `stage.spot`, `drawEnvironment`, `drawText`) live in files that part C creates, so the GitNexus index of the main checkout does not know them: confirm their callers with grep (all callers are inside `ui/js/storymaker` and its tests) and say so in the commit review.

| Mismatch found | Task | Decision |
| --- | --- | --- |
| Tests of the host with real timers fail by chance on a busy machine (250 ms frame deadline) | R1 | the host tests run with generous deadlines |
| Seated drop: D has `pose.offset_y` 215 and `rig.seat_drop`, C used the `lower_leg_l` length (205) | R2 | `pose.offset_y`, else seated pose: `seat_drop`, `seated_offset`, old fallback; blends with the pose blend |
| Text box `pad` and `radius`: D fractions of the stage height, C read em | R3 | fractions of the canvas height; no numbers = old em fallback |
| Point poses when an actor faces left | R4 | `point-left` and `point-right` swap entries |
| Gesture hands: D's `count` sets `point` once and expects it to hold; C cleared it at the next key | R5 | hands hold per side until a later key sets them; pose hands return at the end |
| Environment layers: C sorted by `parallax`, D lists back to front and parallax is not monotonic | R6 | list order is the draw order |
| Spots in 9:16 and 1:1: C squeezed spots without an override, D draws furniture at the design position (the desk) | R6 | design position, squeezed only when outside the slice |
| Titles invisible on night variants and on `abstract-tech` (dark text on dark); label box beige on dark | R7 | text colours resolve from `env.tokens[variant]` first; D's dark variants get a light `text` token (catalog version `ab33e88a6b77`) |
| Camera framing cut the head off (`ACTOR_REACH` 200 px was tuned to the 0.36 stub rig, the real rig is 0.55) | R7 | 297 px |
| Everything else of the brief's list (limb sides, slot params, `hair_back`, eyes/brows keys, expression mouths, outfit tokens, body types, prop size and anchor, parallax, tokens in gradient stops, spot `depth`, aspects, `cycle` keys, `walk.speed`, pose `view`, fonts) | R8 | checked against the real data, no change needed; guarded by the new tests |

### Task R1: Make the real-time host tests independent of the machine speed

> Found while running the suite many times: the host tests of C (08 app host, 09 render driver) drive the real host with real timers and the production deadlines (250 ms per frame, 2 s per scene build). On a busy machine they fail by chance (`took longer than 250 ms for "render" and was stopped`): in 18 runs of the suite of part C, 6 parallel instances at a time, 6 runs failed; it also happened on single runs right after many files had been written. The tests are not about the speed of the machine, so they run with generous deadlines; a test that is about a deadline passes its own limits (it already does).

**Files:**
- Modify: `scripts/storymaker-test-support.mjs` (`TEST_LIMITS`)
- Modify: `scripts/storymaker-tests/08-host-app.mjs` (`makeApp`)
- Modify: `scripts/storymaker-tests/09-host-render.mjs` (`runRender`)

This task has no new check: it removes a source of random failures that would make every "Expected: PASS" of the later tasks unreliable.

- [ ] **Step 1: Show the problem (six instances at a time)**

```bash
for round in 1 2 3; do
  for j in 1 2 3 4 5 6; do (node scripts/test-storymaker-player.mjs > /tmp/sm-par-$round-$j.txt 2>&1 &); done
  sleep 30
done
grep -h "passed," /tmp/sm-par-*.txt | sort | uniq -c
grep -h "^FAIL" /tmp/sm-par-*.txt | cut -c1-200 | sort | uniq -c
```

Expected: on a busy machine some instances print `FAIL host app: ...` or `FAIL render mode: ...` (the render driver reports `took longer than 250 ms for "render" and was stopped`); it is a matter of chance, so repeat the rounds if all instances pass. Here, with the suite of part C alone, 6 of 18 instances failed.

- [ ] **Step 2: Implement**

In `scripts/storymaker-test-support.mjs` (line 240 at this point) replace:

```js
// loadHost(): the real host.bundle.js parts in a fresh vm context (no document: nothing auto-mounts).
```

with:

```js
// Deadlines for the tests that drive the host with real timers. The production limits (250 ms per frame, 2 s per scene build) make
// such a test fail when the machine is busy, so the tests run with generous ones; a test that is about a deadline passes its own.
export const TEST_LIMITS = { compileDeadlineMs: 60000, sceneBuildDeadlineMs: 20000, assetsDeadlineMs: 60000, helloDeadlineMs: 20000, frameDeadlineMs: 20000, warmupDeadlineMs: 20000 };

// loadHost(): the real host.bundle.js parts in a fresh vm context (no document: nothing auto-mounts).
```

In `scripts/storymaker-tests/08-host-app.mjs` (line 5 at this point) replace:

```js
    plain, loadHost, until, settle, loadPlayer, FakeCanvas, InProcessWorker, realmGlobals
} from
```

with:

```js
    plain, loadHost, until, settle, loadPlayer, FakeCanvas, InProcessWorker, realmGlobals, TEST_LIMITS
} from
```

In `scripts/storymaker-tests/08-host-app.mjs` (line 44 at this point) replace:

```js
        limits: opts.limits, ...(opts.env || {})
```

with:

```js
        limits: { ...TEST_LIMITS, ...(opts.limits || {}) }, ...(opts.env || {})
```

In `scripts/storymaker-tests/09-host-render.mjs` (line 6 at this point) replace:

```js
import { root, read, plain, loadHost, FakeCanvas, InProcessWorker, MINI } from
```

with:

```js
import { root, read, plain, loadHost, FakeCanvas, InProcessWorker, MINI, TEST_LIMITS } from
```

In `scripts/storymaker-tests/09-host-render.mjs` (line 79 at this point) replace:

```js
now: () => Date.now(), limits: opts.limits
```

with:

```js
now: () => Date.now(), limits: { ...TEST_LIMITS, ...(opts.limits || {}) }
```

- [ ] **Step 3: Run the load test again**

Run: the same loop as in step 1
Expected: every instance prints `53 passed, 0 failed` (18 of 18 here, with 71 checks once the later tasks are in) and no `FAIL` line; `node scripts/test-storymaker-player.mjs` alone is still green

- [ ] **Step 4: Commit**

```bash
git add scripts/storymaker-test-support.mjs scripts/storymaker-tests/08-host-app.mjs scripts/storymaker-tests/09-host-render.mjs
git commit -m "test(storymaker): run the real-timer host tests with generous deadlines" -m "The production deadlines made them fail by chance on a busy machine. Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task R2: Seated root offset: pose.offset_y, seat_drop, then the old fallback (and the real-catalog test support)

> D writes the seated drop on the pose (`pose.offset_y`, 215 rig units; `rig.seat_drop` repeats it), C derived it from the length of the `lower_leg_l` bone (205). 10 rig units are 6 px on screen: the woman would sit 6 px too high on the chair. Rule: `pose.offset_y` when it is a number; else for a `seated` pose `rig.seat_drop`, then `rig.seated_offset`, then the old fallback (the contract section 3b mini catalog has `seated` without `offset_y`). The drop blends with the pose blend.

**Files:**
- Modify: `scripts/storymaker-test-support.mjs` (real catalog helpers)
- Create: `scripts/storymaker-tests/11-real-catalog.mjs`
- Modify: `ui/js/storymaker/rig.js` (`seatedOffset`, new `poseDrop`, export list)
- Modify: `ui/js/storymaker/render.js` (`rigState`)
- Regenerate: `ui/js/storymaker/player.bundle.js` (`npm run build:ui`)

- [ ] **Step 1: Write the failing tests**

In `scripts/storymaker-test-support.mjs` (line 26 at this point) replace:

```js
export const story = name => readJSON('internal/storymaker/testdata/stories/' + name + '.json');
```

with:

```js
export const story = name => readJSON('internal/storymaker/testdata/stories/' + name + '.json');
// stories that only the player tests use (the cast sheet that the parity browser test renders)
export const playerStory = name => readJSON('internal/storymaker/testdata/player/' + name + '.json');

// The real starter catalog (ui/img/storymaker, part D): the manifest, the rig, every prop and every environment file.
export const REAL = 'ui/img/storymaker/';
export const realCatalog = () => readJSON(REAL + 'catalog.json');
let realCache = null;
export function realFiles() {
    if (!realCache) {
        const cat = realCatalog(), props = {}, envs = {};
        for (const e of cat.entries) {
            if (e.kind === 'prop') props[e.file] = readJSON(REAL + e.file);
            else if (e.kind === 'environment') envs[e.file] = readJSON(REAL + e.file);
        }
        realCache = { rig: readJSON(REAL + cat.rig.file), props, envs };
    }
    return realCache;
}
```

In `scripts/storymaker-test-support.mjs` (line 159 at this point) replace:

```js
export function renderAssets(extra = {}) {
    const f = miniFiles(), canvases = {};
```

with:

```js
export function renderAssets(extra = {}, files = miniFiles()) {
    const f = files, canvases = {};
```

In `scripts/storymaker-test-support.mjs` (line 174 at this point) replace:

```js
// ---- in-process worker --------------------------------------------------------------------------------------------
```

with:

```js
// Render assets of the real starter catalog.
export const realAssets = (extra = {}) => renderAssets(extra, realFiles());

// ---- in-process worker --------------------------------------------------------------------------------------------
```

Create `scripts/storymaker-tests/11-real-catalog.mjs` (the runner `scripts/test-storymaker-player.mjs` picks up every module of the folder in file name order):

```js
// Task R: the player against the real starter catalog (ui/img/storymaker, part D). The rules where the two parts had to be
// reconciled (seated root offset, text box units, point poses when mirrored, gesture hands, environment layer order, spots in
// the narrow aspects, text colours on dark environments, the camera's actor reach), and a spread of frames of firewall.json and
// of the cast sheet (internal/storymaker/testdata/player/castsheet.json) that must draw without an unresolved token.
import assert from 'node:assert/strict';
import { plain, realCatalog, realFiles, realAssets, renderAssets, playerStory, story, FakeCanvas } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

// The async checks of the earlier modules (the host and render driver) run against real timers and deadlines; let them finish before
// this module's long synchronous work (the real rig, a hundred rendered frames) blocks the event loop.
await Promise.all(t.pending);

const { check, near } = t;
const SM = t.core();
const catalog = realCatalog(), files = realFiles();
const entry = id => catalog.entries.find(e => e.id === id);
const firewall = story('firewall');
const codeScenes = { s3: { duration: 5 } };
const compileReal = (s, opts) => SM.compile(s, {}, catalog, { codeScenes, ...opts });
const compiled = compileReal(firewall);
const idle = { type: 'actor', pose: { a: 'stand', b: 'stand', u: 1 }, expression: 'neutral', look: null, walkTime: null, phase: 0, gesture: null, facing: 1 };
const rigStateOf = (over, time) => SM.render.rigState({ ...idle, ...over }, time || 0, SM.render.makeContext(compiled.timeline, realAssets(), 1920, 1080), files.rig);

// ---- seated root offset -------------------------------------------------------------------------------------------------

check('real catalog: a seated pose drops the root by pose.offset_y, blended with the pose blend', () => {
    assert.equal(entry('pose.typing').offset_y, 215);
    assert.equal(rigStateOf({ pose: { a: 'stand', b: 'typing', u: 1 } }).offsetY, 215);
    assert.equal(rigStateOf({ pose: { a: 'stand', b: 'typing', u: 0.5 } }).offsetY, 107.5);
    assert.equal(rigStateOf({ pose: { a: 'sit', b: 'typing', u: 0.5 } }).offsetY, 215, 'two seated poses do not add up');
    assert.equal(rigStateOf({ pose: { a: 'typing', b: 'stand', u: 0.25 } }).offsetY, 161.25);
    assert.equal(rigStateOf({ pose: { a: 'stand', b: 'stand', u: 1 } }).offsetY, 0);
});

check('rig: the drop of a pose is offset_y, else seat_drop, seated_offset, the lower leg, a fraction of the height (seated poses only)', () => {
    const drop = (rig, pose) => SM.rig.poseDrop(rig, pose);
    assert.equal(drop({ seat_drop: 215 }, { seated: true, offset_y: 180 }), 180, 'offset_y wins');
    assert.equal(drop({ seat_drop: 215 }, { offset_y: 40 }), 40, 'offset_y also works on a pose that is not seated');
    assert.equal(drop({ seat_drop: 215, seated_offset: 99 }, { seated: true }), 215);
    assert.equal(drop({ seated_offset: 99 }, { seated: true }), 99);
    assert.equal(drop({ units_height: 1000, bones: [{ name: 'lower_leg_l', length: 205 }] }, { seated: true }), 205);
    assert.equal(drop({ units_height: 1000, bones: [] }, { seated: true }), 240);
    assert.equal(drop({ seat_drop: 215 }, { bones: {} }), 0, 'a standing pose never drops');
    assert.equal(drop({ seat_drop: 215 }, null), 0);
    // the mini catalog's typing pose has seated without offset_y and the mini rig has seated_offset 92
    assert.equal(drop(t.files.rig, t.byId('pose.typing')), 92);
});

check('real catalog: the typing woman at the desk spot sits on the chair seat of the office (D pins the seat at 774 px)', () => {
    const tl = compileReal({ scenes: [{ id: 'a', environment: 'env.office', actors: [{ cast: 'anna', at: 'desk', pose: 'typing' }] }], cast: [{ id: 'anna', character: 'char.office-woman-1' }] }).timeline;
    const sc = tl.scenes[0], st = SM.scene.evalEntity(sc.entities[0], 0), stage = SM.render.stageFor(sc, SM.render.makeContext(tl, realAssets(), 1920, 1080));
    const s = SM.rig.scaleFor(files.rig, 1080, stage.depthScale(st.y, st.depth), 1, SM.rig.bodyOf(files.rig, 'average'));
    const hips = st.y + s * (-440 + 215);
    near(hips, 774, 15, 'hip joint height in design pixels');
});

```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL `real catalog: a seated pose drops the root by pose.offset_y ...` (`205 !== 215`) and FAIL `rig: the drop of a pose ...` (`SM.rig.poseDrop is not a function`); every other check stays green (54 passed, 2 failed)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/rig.js` (line 199 at this point) replace:

```js
    // seatedOffset(rig): how far the root drops (rig units) for seated poses.
    function seatedOffset(rig) {
        if (typeof rig.seated_offset === 'number') return rig.seated_offset;
        var idx = index(rig), lower = idx.byName.lower_leg_l;
        return lower && lower.length > 0 ? lower.length : 0.24 * (rig.units_height || 400);
    }
```

with:

```js
    // seatedOffset(rig): how far the root drops (rig units) for a seated pose that has no offset_y of its own: the rig's
    // seat_drop, else seated_offset, else the length of the lower leg, else a fraction of the height.
    function seatedOffset(rig) {
        if (typeof rig.seat_drop === 'number') return rig.seat_drop;
        if (typeof rig.seated_offset === 'number') return rig.seated_offset;
        var idx = index(rig), lower = idx.byName.lower_leg_l;
        return lower && lower.length > 0 ? lower.length : 0.24 * (rig.units_height || 400);
    }

    // poseDrop(rig, pose): how far a pose lowers the root (rig units, positive = down, like the cycle key offset_y): the
    // pose's offset_y when it is a number, else seatedOffset for a seated pose, else 0.
    function poseDrop(rig, pose) {
        if (!pose) return 0;
        if (typeof pose.offset_y === 'number') return pose.offset_y;
        return pose.seated ? seatedOffset(rig) : 0;
    }
```

In `ui/js/storymaker/rig.js` (line 237 at this point) replace:

```js
        seatedOffset: seatedOffset, pickVariant: pickVariant, drawActor: drawActor
```

with:

```js
        seatedOffset: seatedOffset, poseDrop: poseDrop, pickVariant: pickVariant, drawActor: drawActor
```

In `ui/js/storymaker/render.js` (line 79 at this point) replace:

```js
        var bones = rg.lerpBones(poseA.bones, poseB.bones, u);
        var seated = ((poseA.seated ? 1 - u : 0) + (poseB.seated ? u : 0));
```

with:

```js
        var bones = rg.lerpBones(poseA.bones, poseB.bones, u);
        var drop = rg.poseDrop(rig, poseA) * (1 - u) + rg.poseDrop(rig, poseB) * u;
```

In `ui/js/storymaker/render.js` (line 101 at this point) replace:

```js
            offsetY: seated * rg.seatedOffset(rig) + (cd.offset_y || 0) + (gd.offset_y || 0)
```

with:

```js
            offsetY: drop + (cd.offset_y || 0) + (gd.offset_y || 0)
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
node scripts/test-storymaker-player.mjs
npm run build:ui
npm run check:ui
```
Expected: `56 passed, 0 failed`, the build rewrites `ui/js/storymaker/player.bundle.js`, the check is green

- [ ] **Step 5: Commit**

```bash
git add scripts/storymaker-test-support.mjs ui/js/storymaker/rig.js ui/js/storymaker/render.js scripts/storymaker-tests/11-real-catalog.mjs ui/js/storymaker/player.bundle.js
git commit -m "fix(storymaker): seated poses drop the root by pose.offset_y" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task R3: Text box units: pad and radius are fractions of the canvas height

> D's `text_style.box.pad` and `box.radius` are fractions of the stage height (0.012 and 0.02, like `size`); C read them as em, so a caption box got a padding of 0.012 em (1 px). Decision: fractions of the canvas height. A box without numbers keeps the em fallback (0.4 em padding, 0.3 em corners).

**Files:**
- Modify: `scripts/storymaker-tests/11-real-catalog.mjs` (append a section)
- Modify: `ui/js/storymaker/text.js` (`layout`, `draw`, file comment)
- Regenerate: `ui/js/storymaker/player.bundle.js`

- [ ] **Step 1: Write the failing tests**

Append to `scripts/storymaker-tests/11-real-catalog.mjs`:

```js
// ---- text box units -----------------------------------------------------------------------------------------------------

check('text: box pad and radius are fractions of the canvas height, like the size (real caption and label styles)', () => {
    const caption = entry('text.caption'), label = entry('text.label'), title = entry('text.title');
    const lay = (style, h) => SM.text.layout(new FakeCanvas(10, 10).ctx, 'Hallo Welt', style, { canvasH: h, maxWidth: 1500, family: 'Geist' });
    assert.equal(lay(caption, 1080).pad, 13, 'round(0.012 x 1080)');
    assert.equal(lay(caption, 1080).radius, 13);
    assert.equal(lay(caption, 540).pad, 6);
    assert.equal(lay(label, 1080).pad, 11, 'round(0.010 x 1080)');
    assert.equal(lay(label, 1080).radius, 22, 'round(0.020 x 1080)');
    assert.equal(lay(title, 1080).pad, 0, 'no box, no padding');
    // a box without numbers falls back to em: 0.4 x the font size, radius 0.3 x the font size
    const bare = lay({ ...caption, box: { fill: '#000000' } }, 1000);
    assert.equal(bare.pad, Math.round(0.4 * bare.px));
    assert.equal(bare.radius, Math.round(0.3 * bare.px));
    // and the corners that are drawn use the radius
    const cv = new FakeCanvas(1920, 1080), l = lay(caption, 1080);
    SM.text.draw(cv.ctx, l, caption, 960, 900, SM.parts.makeResolver([{}]), 1);
    assert.equal(cv.ctx.log.filter(x => x.startsWith('arcTo(') && x.endsWith(',13)')).length, 4, 'four rounded corners of radius 13');
});

```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL `text: box pad and radius are fractions of the canvas height ...` (`1 !== 13`); every other check stays green (56 passed, 1 failed)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/text.js` (line 4 at this point) replace:

```js
// publishes StorymakerCore.text.
```

with:

```js
// publishes StorymakerCore.text. A text style's box.pad and box.radius are fractions of the canvas height, like size.
```

In `ui/js/storymaker/text.js` (line 66 at this point) replace:

```js
        var pad = style.box ? Math.round((style.box.pad > 0 ? style.box.pad : 0.4) * px) : 0;
        return {
            lines: wrapped.lines, truncated: wrapped.truncated, px: px, lineH: px * LINE_HEIGHT,
            width: width, height: wrapped.lines.length * px * LINE_HEIGHT, pad: pad, font: ctx.font
        };
```

with:

```js
        // box.pad and box.radius are fractions of the canvas height; a box without them gets 0.4 em padding and 0.3 em corners
        var pad = 0, radius = 0;
        if (style.box) {
            pad = Math.round(style.box.pad > 0 ? style.box.pad * opts.canvasH : 0.4 * px);
            radius = Math.round(style.box.radius >= 0 ? style.box.radius * opts.canvasH : 0.3 * px);
        }
        return {
            lines: wrapped.lines, truncated: wrapped.truncated, px: px, lineH: px * LINE_HEIGHT,
            width: width, height: wrapped.lines.length * px * LINE_HEIGHT, pad: pad, radius: radius, font: ctx.font
        };
```

In `ui/js/storymaker/text.js` (line 101 at this point) replace:

```js
            var radius = Math.round((style.box.radius >= 0 ? style.box.radius : 0.3) * lay.px);
            ctx.fillStyle = resolve(style.box.fill || '$bg2');
            roundRect(ctx, left - lay.pad, y - lay.height / 2 - lay.pad, lay.width + lay.pad * 2, lay.height + lay.pad * 2, radius);
```

with:

```js
            ctx.fillStyle = resolve(style.box.fill || '$bg2');
            roundRect(ctx, left - lay.pad, y - lay.height / 2 - lay.pad, lay.width + lay.pad * 2, lay.height + lay.pad * 2, lay.radius);
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
node scripts/test-storymaker-player.mjs
npm run build:ui
npm run check:ui
```
Expected: `57 passed, 0 failed`, the build rewrites `ui/js/storymaker/player.bundle.js`, the check is green

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/text.js scripts/storymaker-tests/11-real-catalog.mjs ui/js/storymaker/player.bundle.js
git commit -m "fix(storymaker): text box pad and radius are fractions of the canvas height" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task R4: Point poses keep their screen direction when the actor faces left

> D recommends swapping `point-left` and `point-right` when an actor faces left, so that the name keeps meaning the direction on the screen. A left-facing actor is drawn mirrored, so `point-right` (the near arm forward, three-quarter view) would reach the left edge. The swap picks the other catalog entry (its bones, hands and `view` hint) and needs both entries to exist.

**Files:**
- Modify: `scripts/storymaker-tests/11-real-catalog.mjs` (append a section)
- Modify: `ui/js/storymaker/render.js` (new `poseFor`, `rigState`)
- Regenerate: `ui/js/storymaker/player.bundle.js`

- [ ] **Step 1: Write the failing tests**

Append to `scripts/storymaker-tests/11-real-catalog.mjs`:

```js
// ---- point poses and mirroring ------------------------------------------------------------------------------------------

// The screen side that the hand of a pose reaches: x of the hand tip relative to the root, after the mirror of a left-facing actor.
function handSide(poseName, facing) {
    const rs = rigStateOf({ pose: { a: poseName, b: poseName, u: 1 }, facing }), body = SM.rig.bodyOf(files.rig, 'average');
    const world = SM.rig.boneMatrices(files.rig, rs.view, body, rs.offsets);
    const hand = rs.hands.l === 'point' ? world.lower_arm_l : world.lower_arm_r, tip = hand[2] * 135 + hand[4];
    return Math.sign(tip) * facing;
}

check('rig state: point-left and point-right keep their screen direction when the actor faces left', () => {
    assert.equal(handSide('point-right', 1), 1);
    assert.equal(handSide('point-left', 1), -1);
    assert.equal(handSide('point-right', -1), 1, 'facing left: point-right still reaches the right edge of the screen');
    assert.equal(handSide('point-left', -1), -1);
    // the swap picks the other pose entry: its bones and its view hint
    const left = rigStateOf({ pose: { a: 'point-right', b: 'point-right', u: 1 }, facing: -1 });
    assert.equal(left.offsets.upper_arm_l, 88, 'the bones of point-left');
    assert.equal(left.hands.l, 'point');
    assert.equal(left.view, 'front');
    assert.equal(rigStateOf({ pose: { a: 'point-right', b: 'point-right', u: 1 }, facing: 1 }).view, 'three_quarter');
    // other poses are not mirrored by name
    assert.equal(rigStateOf({ pose: { a: 'wave', b: 'wave', u: 1 }, facing: -1 }).offsets.upper_arm_r, -128);
    // a catalog that has no point-left keeps the name (the mini catalog has neither)
    const mini = SM.compile({ cast: [{ id: 'a', character: 'char.office-woman-1' }], scenes: [{ id: 's', actors: [{ cast: 'a', at: 'left', pose: 'stand' }] }] }, {}, t.catalog);
    const rcMini = SM.render.makeContext(mini.timeline, renderAssets(), 1920, 1080);
    assert.equal(SM.render.rigState({ ...idle, pose: { a: 'stand', b: 'stand', u: 1 }, facing: -1 }, 0, rcMini, t.files.rig).view, 'three_quarter');
});

```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL `rig state: point-left and point-right keep their screen direction ...` (`-1 !== 1`: facing left, point-right reaches the left edge); every other check stays green (57 passed, 1 failed)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/render.js` (line 73 at this point) replace:

```js
    // rigState(st, t, rc, rig): the draw state of an actor from its logical state (pose blend, gesture, walk or idle cycle).
    function rigState(st, t, rc, rig) {
        var cat = rc.cat, rg = R();
        var poseA = cat.poses[st.pose.a] || cat.poses.stand || { bones: {} };
        var poseB = cat.poses[st.pose.b] || cat.poses.stand || { bones: {} };
```

with:

```js
    var MIRROR_POSES = { 'point-left': 'point-right', 'point-right': 'point-left' };

    // poseFor(cat, name, facing): the catalog entry of a pose name. A left-facing actor is drawn mirrored, so point-left and
    // point-right swap entries and the name keeps meaning the direction on the screen. The swap needs both entries.
    function poseFor(cat, name, facing) {
        var other = facing < 0 && MIRROR_POSES[name];
        var pick = other && cat.poses[other] && cat.poses[name] ? other : name;
        return cat.poses[pick] || cat.poses.stand || { bones: {} };
    }

    // rigState(st, t, rc, rig): the draw state of an actor from its logical state (pose blend, gesture, walk or idle cycle).
    function rigState(st, t, rc, rig) {
        var cat = rc.cat, rg = R();
        var poseA = poseFor(cat, st.pose.a, st.facing);
        var poseB = poseFor(cat, st.pose.b, st.facing);
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
node scripts/test-storymaker-player.mjs
npm run build:ui
npm run check:ui
```
Expected: `58 passed, 0 failed`, the build rewrites `ui/js/storymaker/player.bundle.js`, the check is green

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/render.js scripts/storymaker-tests/11-real-catalog.mjs ui/js/storymaker/player.bundle.js
git commit -m "fix(storymaker): point-left and point-right keep their screen side when mirrored" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task R5: Gesture hand shapes hold until a later key sets them again

> C treated a key without `hands` as "clear the override". D's `count` gesture sets `hands: {r: "point"}` only at its first key (t = 0.2) and expects the finger to keep pointing while the arm counts; with C's rule the hand opened at t = 0.35. New rule: the hands of a key hold, per side, until a later key sets that side again; at the end of the gesture (`u >= 1`) the pose hands return.

**Files:**
- Modify: `scripts/storymaker-tests/11-real-catalog.mjs` (append a section)
- Modify: `ui/js/storymaker/rig.js` (`handsAt`)
- Regenerate: `ui/js/storymaker/player.bundle.js`

- [ ] **Step 1: Write the failing tests**

Append to `scripts/storymaker-tests/11-real-catalog.mjs`:

```js
// ---- gesture hands ------------------------------------------------------------------------------------------------------

check('rig: a gesture hand shape holds until a later key sets another one (D: count points only at its first key)', () => {
    const count = entry('gesture.count'), celebrate = entry('gesture.celebrate');
    assert.deepEqual(plain(SM.rig.handsAt(count.keys, 0.1)), null);
    assert.deepEqual(plain(SM.rig.handsAt(count.keys, 0.5)), { r: 'point' }, 'the finger keeps pointing while the arm counts');
    assert.equal(SM.rig.handsAt(count.keys, 1), null, 'the end of the gesture returns to the pose hands');
    assert.deepEqual(plain(SM.rig.handsAt(celebrate.keys, 0.5)), { l: 'open', r: 'open' });
    assert.deepEqual(plain(SM.rig.handsAt([{ t: 0, bones: {}, hands: { l: 'fist' } }, { t: 0.5, bones: {}, hands: { r: 'point' } }], 0.7)), { l: 'fist', r: 'point' }, 'hands merge per side');
    assert.equal(rigStateOf({ gesture: { name: 'count', frac: 0.5 } }).hands.r, 'point');
    assert.equal(rigStateOf({ pose: { a: 'think', b: 'think', u: 1 } }).hands.r, 'fist', 'a pose hand shape without a gesture');
});

```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL `rig: a gesture hand shape holds until a later key sets another one ...` (`null` instead of `{ r: 'point' }` at t = 0.5 of count); every other check stays green (58 passed, 1 failed)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/rig.js` (line 128 at this point) replace:

```js
    // handsAt(keys, u): the hand override of the last key at or before u (a key without hands clears the override).
    function handsAt(keys, u) {
        var hands = null;
        for (var i = 0; i < (keys || []).length; i++) {
            if (keys[i].t <= u) hands = keys[i].hands || null; else break;
        }
        return hands;
    }
```

with:

```js
    // handsAt(keys, u): the hand shapes a gesture sets at u. The hands of a key hold, per side, until a later key sets that
    // side again (the catalog's count gesture points only at its first key); at the end of the gesture (u >= 1) the hands of
    // the pose return. null = the gesture has not set any hand yet.
    function handsAt(keys, u) {
        if (u >= 1) return null;
        var hands = null;
        for (var i = 0; i < (keys || []).length; i++) {
            if (keys[i].t > u) break;
            if (!keys[i].hands) continue;
            hands = hands || {};
            for (var side in keys[i].hands) hands[side] = keys[i].hands[side];
        }
        return hands;
    }
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
node scripts/test-storymaker-player.mjs
npm run build:ui
npm run check:ui
```
Expected: `59 passed, 0 failed`, the build rewrites `ui/js/storymaker/player.bundle.js`, the check is green

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/rig.js scripts/storymaker-tests/11-real-catalog.mjs ui/js/storymaker/player.bundle.js
git commit -m "fix(storymaker): gesture hand shapes hold until another key sets them" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task R6: Environment layers draw in their listed order; a spot keeps its design position in 9:16 and 1:1

> Two defects. (1) C sorted environment layers by `parallax`; D lists the layers back to front and `parallax` is not monotonic (`office`: wall 0.1, floor 1, furniture 0.7; `abstract-tech`: ... floor 1, hud 0.7), so the furniture was painted under the floor. The list order is the draw order; `parallax` only says how far a layer follows the camera. (2) C squeezed every spot without a 9:16 or 1:1 override into the visible slice; D lists overrides only for spots that would fall outside, and the other spots are drawn where the art is (the office `desk` spot with the chair and the desk at design x = 36 %): squeezed, the seated woman sat 68 px beside her chair. A spot without an override keeps its design position and is squeezed only when it would be outside the slice (3 percent margin), which keeps the mini catalog (no overrides at all) working. An override may also carry `face`.

**Files:**
- Modify: `scripts/storymaker-tests/11-real-catalog.mjs` (append a section)
- Modify: `scripts/storymaker-tests/02-stage-parts-text.mjs` (the mini-catalog desk expectation)
- Modify: `ui/js/storymaker/render.js` (`drawEnvironment`)
- Modify: `ui/js/storymaker/stage.js` (`SLICE_MARGIN`, `stage.spot`)
- Regenerate: `ui/js/storymaker/player.bundle.js`

- [ ] **Step 1: Write the failing tests**

In `scripts/storymaker-tests/02-stage-parts-text.mjs` (line 27 at this point) replace:

```js
    near(tall.spot('center').x, 960, 1e-9, 'a spot without an override is squeezed into the visible slice');
    near(tall.spot('board').x, tall.vis.x + 0.74 * tall.vis.w, 1e-9);
```

with:

```js
    near(tall.spot('center').x, 960, 1e-9, 'a spot without an override and inside the slice keeps its design position');
    near(tall.spot('board').x, tall.vis.x + 0.74 * tall.vis.w, 1e-9, 'a spot without an override that is outside the slice is squeezed into it');
```

In `scripts/storymaker-tests/02-stage-parts-text.mjs` (line 31 at this point) replace:

```js
    near(officeTall.spot('desk').x, officeTall.vis.x + 0.36 * officeTall.vis.w, 1e-9);
```

with:

```js
    near(officeTall.spot('desk').x, 0.36 * 1920, 1e-9, 'the desk is inside the slice: the furniture is drawn there, so the actor stays there');
    assert.ok(officeTall.spot('far-left').x >= officeTall.vis.x, 'far-left would be outside the slice and is squeezed into it');
```

Append to `scripts/storymaker-tests/11-real-catalog.mjs`:

```js
// ---- environments -------------------------------------------------------------------------------------------------------

check('render: environment layers draw in their listed order, parallax only moves them', () => {
    const env = {
        viewbox: [0, 0, 1920, 1080], tokens: { day: {}, night: {} },
        layers: [
            { name: 'wall', parallax: 0.1, shapes: [{ d: 'M0,0 L1,1', fill: '#111111' }] },
            { name: 'floor', parallax: 1, shapes: [{ d: 'M0,0 L2,2', fill: '#222222' }] },
            { name: 'furniture', parallax: 0.7, shapes: [{ d: 'M0,0 L3,3', fill: '#333333' }] }
        ]
    };
    const tl = SM.compile({ scenes: [{ id: 'a', environment: 'env.office', silent: true }] }, {}, catalog).timeline;
    const cv = new FakeCanvas(tl.width, tl.height);
    SM.renderFrame(tl, 3, cv.ctx, realAssets({ envs: { 'environments/office.json': env } }));
    const order = cv.ctx.log.filter(l => /^set fillStyle=#[123]{6}$/.test(l)).map(l => l.slice(-6));
    assert.deepEqual(order, ['111111', '222222', '333333']);
    // the real files list the layers back to front, and parallax is not monotonic in them
    for (const name of ['office', 'abstract-tech']) {
        const par = files.envs['environments/' + name + '.json'].layers.map(l => l.parallax);
        assert.ok(par.some((p, i) => i && p < par[i - 1]), name + ': parallax is not sorted, so the order must come from the list');
    }
});

check('stage: in 9:16 and 1:1 every spot of the real environments is inside the visible slice, and a spot without an override keeps its design position', () => {
    for (const env of catalog.entries.filter(e => e.kind === 'environment')) {
        for (const aspect of ['9:16', '1:1']) {
            const stage = SM.stage.create({ aspect, env, safe: catalog.stage.safe });
            const over = env.aspects[aspect], vis = stage.vis;
            for (const name of Object.keys(env.spots)) {
                const s = stage.spot(name);
                assert.ok(s.x >= vis.x + 0.03 * vis.w - 1e-6 && s.x <= vis.x + 0.97 * vis.w + 1e-6, env.id + ' ' + aspect + ' ' + name + ' at ' + Math.round(s.x) + ' is outside ' + Math.round(vis.x) + '..' + Math.round(vis.x + vis.w));
                const o = over.spots && over.spots[name];
                near(s.x, (o ? o.x : env.spots[name].x) / 100 * 1920, 1e-6, env.id + ' ' + aspect + ' ' + name);
                near(s.y, (o && o.y !== undefined ? o.y : env.spots[name].y) / 100 * 1080, 1e-6);
            }
            near(stage.vis.x + stage.vis.w / 2, over.focus_x / 100 * 1920, 1e-6, 'the slice is centred on focus_x');
        }
    }
    // a catalog without overrides (the mini catalog) still squeezes a spot that is outside the slice into it
    const mini = SM.stage.create({ aspect: '9:16', env: t.byId('env.office'), safe: t.catalog.stage.safe }), far = mini.spot('far-left');
    assert.ok(far.x >= mini.vis.x && far.x <= mini.vis.x + mini.vis.w);
});

```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL `render: environment layers draw in their listed order ...` (the fills come out as `111111, 333333, 222222`), FAIL `stage: in 9:16 and 1:1 every spot of the real environments ...` (`env.plain-stage 1:1 left: 744 vs 576`) and the updated existing check FAIL `stage: mapping for 16:9, 9:16 and 1:1 ...` (`the desk is inside the slice ...: 682.95 vs 691.2`); every other check stays green (58 passed, 3 failed)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/render.js` (line 52 at this point) replace:

```js
        var layers = (env.layers || []).map(function (l, k) { return { l: l, k: k }; })
            .sort(function (a, b) { return (a.l.parallax || 0) - (b.l.parallax || 0) || a.k - b.k; });
        for (var i = 0; i < layers.length; i++) {
            var p = typeof layers[i].l.parallax === 'number' ? layers[i].l.parallax : 1;
            var dx = (1 - p) * (win.x - bw.x), dy = (1 - p) * (win.y - bw.y);
            setM(ctx, R().mul(Mv, R().translate(dx, dy)));
            SM.parts.drawShapes(ctx, layers[i].l.shapes, resolve, 1);
        }
```

with:

```js
        // the layers are listed back to front; parallax only says how much a layer follows the camera (it is not a draw order)
        var layers = env.layers || [];
        for (var i = 0; i < layers.length; i++) {
            var p = typeof layers[i].parallax === 'number' ? layers[i].parallax : 1;
            var dx = (1 - p) * (win.x - bw.x), dy = (1 - p) * (win.y - bw.y);
            setM(ctx, R().mul(Mv, R().translate(dx, dy)));
            SM.parts.drawShapes(ctx, layers[i].shapes, resolve, 1);
        }
```

In `ui/js/storymaker/stage.js` (line 18 at this point) replace:

```js
    var BASE_SPOTS = ['far-left', 'left', 'center', 'right', 'far-right'];
```

with:

```js
    var BASE_SPOTS = ['far-left', 'left', 'center', 'right', 'far-right'];
    var SLICE_MARGIN = 0.03;   // a spot closer than this fraction of the slice width to its edge counts as outside
```

In `ui/js/storymaker/stage.js` (line 79 at this point) replace:

```js
        // spot(name) = {x, y, face, depth, name} in design pixels, or null for an unknown name. The per-aspect override
        // wins; otherwise a spot of a cropped aspect is squeezed into the visible slice so left/centre/right stay on screen.
        stage.spot = function (name) {
            var src = (env && env.spots && env.spots[name]) || DEFAULT_SPOTS[name];
            if (!src) return null;
            var o = over && over.spots && over.spots[name];
            var xPct = num(o && o.x, src.x), yPct = num(o && o.y, src.y), x;
            if (o && typeof o.x === 'number') x = xPct / 100 * DESIGN_W;
            else if (cropped) x = vis.x + xPct / 100 * vis.w;
            else x = xPct / 100 * DESIGN_W;
            return { name: name, x: x, y: yPct / 100 * DESIGN_H, face: src.face || null, depth: typeof src.depth === 'number' ? src.depth : null };
        };
```

with:

```js
        // spot(name) = {x, y, face, depth, name} in design pixels, or null for an unknown name. The per-aspect override wins;
        // without one the spot keeps its design position (furniture such as the desk is drawn there), unless a cropped aspect
        // would put it outside the visible slice (3 percent margin): then it is squeezed into the slice, so a catalog that
        // lists no overrides still keeps left, centre and right on screen.
        stage.spot = function (name) {
            var src = (env && env.spots && env.spots[name]) || DEFAULT_SPOTS[name];
            if (!src) return null;
            var o = over && over.spots && over.spots[name];
            var xPct = num(o && o.x, src.x), yPct = num(o && o.y, src.y), x = xPct / 100 * DESIGN_W;
            if (cropped && !(o && typeof o.x === 'number') && (x < vis.x + SLICE_MARGIN * vis.w || x > vis.x + (1 - SLICE_MARGIN) * vis.w)) {
                x = vis.x + xPct / 100 * vis.w;
            }
            var face = (o && o.face) || src.face || null;
            return { name: name, x: x, y: yPct / 100 * DESIGN_H, face: face, depth: typeof src.depth === 'number' ? src.depth : null };
        };
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
node scripts/test-storymaker-player.mjs
npm run build:ui
npm run check:ui
```
Expected: `61 passed, 0 failed`, the build rewrites `ui/js/storymaker/player.bundle.js`, the check is green

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/render.js ui/js/storymaker/stage.js scripts/storymaker-tests/02-stage-parts-text.mjs scripts/storymaker-tests/11-real-catalog.mjs ui/js/storymaker/player.bundle.js
git commit -m "fix(storymaker): environment layers draw in listed order, spots keep their design position" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task R7: Texts on dark environments are readable; the camera frames an actor of the real catalog (catalog data fix)

> Found by the visual parity run (R9). (1) A `title` is `$text` (dark navy) and has no box, so on every night variant and on the always dark `abstract-tech` environment it was invisible; the `label` box fill `$bg2` stayed beige there. The player cannot know which environment is dark, so the environment says it: its colour tokens now come first when text colours are resolved (`env.tokens[variant]`, then the palette, the same chain as the environment shapes), and D's dark variants carry a light `text` token (the night variants already override `bg2`; `abstract-tech` gets `bg2` and `text` in both variants; none of its shapes uses `$bg2`). This is a real defect of D's data that the player cannot absorb, hence the catalog change (new catalog version `ab33e88a6b77`, was `70d5cf51a9a9`). (2) `ACTOR_REACH` (the camera target height of an actor) was 200 px for the stub rig (0.36 of the height); the real rig is 0.55 of the height, so a close framing cut the head off. It is now 297 px, half of `rig.height` x 1080 (the manifest carries no `rig.height`).

**Files:**
- Modify: `scripts/storymaker-tests/11-real-catalog.mjs` (append a section)
- Modify: `ui/js/storymaker/render.js` (`envTokens`, `drawEnvironment`, `drawText`, `drawScene`)
- Modify: `ui/js/storymaker/beats.js` (`ACTOR_REACH`)
- Modify: `assets/storymaker/lib/environments.mjs` (light `text` token for every dark environment variant, dark `bg2` for abstract-tech)
- Regenerate: `ui/img/storymaker/catalog.json`, `ui/img/storymaker/environments/*.json` (`node scripts/build-storymaker-catalog.mjs`), `ui/js/storymaker/player.bundle.js`

- [ ] **Step 1: Write the failing tests**

Append to `scripts/storymaker-tests/11-real-catalog.mjs`:

```js
// ---- text colours and the camera ----------------------------------------------------------------------------------------

check('render: text colours resolve from the environment variant first, then the palette', () => {
    const env = { viewbox: [0, 0, 1920, 1080], layers: [], tokens: { day: {}, night: { text: '#f2f4ff', bg2: '#2c356b' } } };
    const story2 = variant => ({ scenes: [{ id: 'a', environment: 'env.office', variant, beats: [
        { do: 'show', text: 'Titel', entry: 'none' }, { do: 'show', text: 'Marke', style: 'label', at: { x: 50, y: 60 }, entry: 'none', with: 'prev' }] }] });
    const fills = variant => {
        const tl = compileReal(story2(variant)).timeline, cv = new FakeCanvas(tl.width, tl.height);
        SM.renderFrame(tl, 5, cv.ctx, realAssets({ envs: { 'environments/office.json': env } }));
        return cv.ctx.log.filter(l => l.startsWith('set fillStyle=')).map(l => l.slice('set fillStyle='.length));
    };
    assert.ok(fills('night').includes('#f2f4ff'), 'the title takes the night text colour');
    assert.ok(fills('night').includes('#2c356b'), 'the label box takes the night bg2');
    assert.ok(fills('day').includes('#1f2a44') && !fills('day').includes('#f2f4ff'), 'the day variant has no text token: the palette decides');
});

// WCAG contrast of two #rrggbb colours
const luminance = hex => [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16) / 255).map(c => c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4).reduce((s, c, i) => s + c * [0.2126, 0.7152, 0.0722][i], 0);
const contrast = (a, b) => { const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x); return (hi + 0.05) / (lo + 0.05); };

check('real catalog: the label and title text colours are readable in every environment, variant and palette', () => {
    const label = entry('text.label');
    for (const name of ['plain-stage', 'office', 'home-office', 'abstract-tech']) {
        const env = files.envs['environments/' + name + '.json'];
        for (const variant of ['day', 'night']) {
            for (const pal of catalog.palettes) {
                const tokens = { ...pal.tokens, ...env.tokens[variant] };
                const boxFill = tokens[label.box.fill.slice(1)], labelInk = tokens[label.color.slice(1)];
                assert.ok(contrast(labelInk, boxFill) >= 4.5, name + ' ' + variant + ' ' + pal.name + ': label ' + labelInk + ' on ' + boxFill);
                // the title has no box and the palette's text colour is dark: every dark environment (night, abstract-tech) carries a light one
                const dark = variant === 'night' || name === 'abstract-tech';
                if (dark) assert.ok(env.tokens[variant].text && luminance(env.tokens[variant].text) > 0.6, name + ' ' + variant + ' has a light text token');
            }
        }
    }
});

check('camera: a close framing on an actor of the real catalog shows the head and the feet', () => {
    const out = compileReal({ cast: [{ id: 'anna', character: 'char.office-woman-1' }], scenes: [{ id: 's', environment: 'env.office', camera: { framing: 'close', target: 'anna' }, actors: [{ cast: 'anna', at: 'desk', pose: 'typing' }] }] });
    const sc = out.timeline.scenes[0], stage = SM.render.stageFor(sc, SM.render.makeContext(out.timeline, realAssets(), 1920, 1080));
    const st = SM.scene.evalEntity(sc.entities[0], 0), win = stage.cameraWindow(SM.camera.at(sc.camera, 0));
    const depth = stage.depthScale(st.y, st.depth), headTop = st.y - 0.55 * 1080 * depth;
    assert.ok(win.y <= headTop && win.y + win.h >= st.y, 'window ' + Math.round(win.y) + '..' + Math.round(win.y + win.h) + ' holds ' + Math.round(headTop) + '..' + Math.round(st.y));
    near(win.h, 1080 / 1.9, 1e-6);
});

```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL `render: text colours resolve from the environment variant first ...` (`the title takes the night text colour`), FAIL `real catalog: the label and title text colours are readable ...` (`plain-stage night sunny: label #1f2a44 on #2c356b`), FAIL `camera: a close framing on an actor ...` (`window 436..1004 holds 352..907`); every other check stays green (61 passed, 3 failed)

- [ ] **Step 3: Implement**

In `ui/js/storymaker/render.js` (line 47 at this point) replace:

```js
    function drawEnvironment(ctx, sc, rc, win, bw, Mv) {
        var env = sc.env && rc.assets.envs && rc.assets.envs[sc.env.file];
        if (!env) return;
        var tokens = env.tokens && (env.tokens[sc.variant] || env.tokens.day);
        var resolve
```

with:

```js
    // envTokens(sc, rc): the colour tokens of the scene's environment variant (the day tokens when the variant has none), or null.
    function envTokens(sc, rc) {
        var env = sc.env && rc.assets.envs && rc.assets.envs[sc.env.file];
        return (env && env.tokens && (env.tokens[sc.variant] || env.tokens.day)) || null;
    }

    function drawEnvironment(ctx, sc, rc, win, bw, Mv) {
        var env = sc.env && rc.assets.envs && rc.assets.envs[sc.env.file];
        if (!env) return;
        var tokens = envTokens(sc, rc);
        var resolve
```

In `ui/js/storymaker/render.js` (line 164 at this point) replace:

```js
    function drawText(ctx, st, rc, stage, bw) {
```

with:

```js
    // drawText: tokens (optional) are the environment's colour tokens; they win over the palette, so a dark environment can carry
    // its own text colour and label box colour.
    function drawText(ctx, st, rc, stage, bw, tokens) {
```

In `ui/js/storymaker/render.js` (line 172 at this point) replace:

```js
        SM.text.draw(ctx, lay, style, pos.x, pos.y, SM.parts.makeResolver([rc.palette]), st.alpha);
```

with:

```js
        SM.text.draw(ctx, lay, style, pos.x, pos.y, SM.parts.makeResolver([tokens, rc.palette]), st.alpha);
```

In `ui/js/storymaker/render.js` (line 189 at this point) replace:

```js
        states.filter(function (s) { return s.type === 'text'; }).forEach(function (s) { drawText(ctx, s, rc, stage, bw); });
```

with:

```js
        var tokens = envTokens(sc, rc);
        states.filter(function (s) { return s.type === 'text'; }).forEach(function (s) { drawText(ctx, s, rc, stage, bw, tokens); });
```

In `ui/js/storymaker/beats.js` (line 11 at this point) replace:

```js
    var ACTOR_REACH = 200;   // design pixels from the feet to the visual centre of a standing actor at scale 1
```

with:

```js
    // design pixels from the feet to the visual centre of a standing actor at depth scale 1: half of rig.height (0.55 of the
    // design height in the starter catalog). The manifest does not carry rig.height, so the camera uses this constant.
    var ACTOR_REACH = 297;
```

In `assets/storymaker/lib/environments.mjs` (line 3 at this point) replace:

```js
// tokens override bg/bg2 and the local tokens. Local tokens exist in both variants.
```

with:

```js
// tokens override bg/bg2, the text colour (light: the player draws texts with these tokens first) and the local tokens.
// Local tokens exist in both variants. The dark abstract-tech environment carries bg2 and text in both variants.
```

In `assets/storymaker/lib/environments.mjs` (line 89 at this point) replace:

```js
      night: { bg: '#1b2142', bg2: '#2c356b', star: '#dfe6ff' },
```

with:

```js
      night: { bg: '#1b2142', bg2: '#2c356b', text: '#f2f4ff', star: '#dfe6ff' },
```

In `assets/storymaker/lib/environments.mjs` (line 248 at this point) replace:

```js
      night: { bg: '#232a4d', bg2: '#2f3865', wall2:
```

with:

```js
      night: { bg: '#232a4d', bg2: '#2f3865', text: '#f2f4ff', wall2:
```

In `assets/storymaker/lib/environments.mjs` (line 342 at this point) replace:

```js
      night: { bg: '#2c2540', bg2: '#3d3560', wallh:
```

with:

```js
      night: { bg: '#2c2540', bg2: '#3d3560', text: '#f2f4ff', wallh:
```

In `assets/storymaker/lib/environments.mjs` (line 401 at this point) replace:

```js
      day: { t1: '#16295f', t2: '#2c58c8', grid: '#7fbaff', panel: '#0e1a47' },
      night: { t1: '#090d2b', t2: '#1b2160', grid: '#4f7dff', panel: '#070b22' },
```

with:

```js
      day: { bg2: '#0e1a47', text: '#f4f7ff', t1: '#16295f', t2: '#2c58c8', grid: '#7fbaff', panel: '#0e1a47' },
      night: { bg2: '#070b22', text: '#f4f7ff', t1: '#090d2b', t2: '#1b2160', grid: '#4f7dff', panel: '#070b22' },
```

Regenerate the catalog outputs (the environment JSON files and the version in `catalog.json` change, nothing else):

```bash
node scripts/build-storymaker-catalog.mjs
```
Expected: `storymaker catalog ab33e88a6b77: 105 files, 2.37 MiB in ui/img/storymaker, 16 locale files`

- [ ] **Step 4: Run the tests to verify they pass**

```bash
node scripts/test-storymaker-player.mjs
node scripts/build-storymaker-catalog.mjs --check
node --test assets/storymaker/test/environments.test.mjs
node --test assets/storymaker/test/catalog.test.mjs
go test ./ui -run 'TestStorymakerCatalog' -count=1
npm run build:ui
npm run check:ui
```
Expected: `64 passed, 0 failed`, `storymaker catalog ab33e88a6b77 is up to date`, 4 and 6 node tests pass, `ok aurago/ui`, the build rewrites `ui/js/storymaker/player.bundle.js`, the check is green

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/render.js ui/js/storymaker/beats.js assets/storymaker/lib/environments.mjs scripts/storymaker-tests/11-real-catalog.mjs ui/js/storymaker/player.bundle.js ui/img/storymaker
git commit -m "fix(storymaker): readable texts on dark environments, camera reach for the real rig" -m "Catalog version becomes ab33e88a6b77. Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task R8: Real-catalog integration checks, the cast sheet story and the Go guard for catalog and stories

> These are guards of the integrated state: they pass as soon as the fixes of R2 to R7 are in and fail when one of the parts drifts. The Node checks load the real catalog and compile `firewall.json`, `minimal.json` and the cast sheet; every character x view x expression resolves every slot to a variant or to a deliberate `"none"`, the compile report holds only `narration_missing`, `not_supported_yet` and `layout_unverified`, a spread of frames in all three aspects and every scene of the cast sheet (both palettes) draw without a `#ff00ff` colour, a prop stands on its spot, a left-facing actor is mirrored, and the remaining rules of the catalog (pose view hints, walk cycle and speed, body types, spot depth, four font weights). The Go test loads the real catalog with A's `LoadCatalog`, verifies that every referenced file exists and validates `minimal.json`, `firewall.json` and the cast sheet with zero issues.

**Files:**
- Create: `internal/storymaker/testdata/player/castsheet.json`
- Modify: `scripts/storymaker-tests/11-real-catalog.mjs` (append two sections)
- Create: `internal/storymaker/catalog_real_test.go`

- [ ] **Step 1: Write the failing Go guard test**

```go
package storymaker

import (
	"os"
	"sync"
	"testing"
)

// The real starter catalog (ui/img/storymaker, produced by scripts/build-storymaker-catalog.mjs) and the fixture stories must
// keep agreeing: this guards the Go core against drifting away from the catalog (names, spots, files).

var (
	realOnce sync.Once
	realCat  *Catalog
	realErr  error
)

func loadReal(t testing.TB) *Catalog {
	t.Helper()
	realOnce.Do(func() {
		realCat, realErr = LoadCatalog(os.DirFS("../../ui"), "img/storymaker/catalog.json")
	})
	if realErr != nil {
		t.Fatalf("LoadCatalog(ui/img/storymaker): %v", realErr)
	}
	return realCat
}

func TestRealCatalogLoadsAndItsFilesExist(t *testing.T) {
	cat := loadReal(t)
	if err := cat.VerifyFiles(os.DirFS("../../ui"), "img/storymaker"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"char.office-woman-1", "char.office-man-1", "char.it-tech-1", "char.senior-man-1", "env.plain-stage", "env.office",
		"env.home-office", "env.abstract-tech", "prop.lock", "prop.shield", "pose.typing", "gesture.point", "expr.surprised", "cycle.walk", "text.caption", "trans.wipe"} {
		if _, ok := cat.Lookup(id); !ok {
			t.Errorf("the starter catalog has no %s", id)
		}
	}
	for _, kind := range []string{"pose", "gesture", "expression"} {
		if len(cat.Names(kind)) == 0 {
			t.Errorf("no %s names", kind)
		}
	}
}

func TestRealCatalogValidatesTheFixtureStories(t *testing.T) {
	cat := loadReal(t)
	for _, name := range []string{"stories/minimal", "stories/firewall", "player/castsheet"} {
		data, err := os.ReadFile("testdata/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		s := mustDecode(t, string(data))
		if issues := Validate(&s, cat, DefaultLimits()); len(issues) != 0 {
			t.Errorf("%s: %s", name, dump(issues))
		}
		ApplyDefaults(&s)
		if issues := Validate(&s, cat, DefaultLimits()); len(issues) != 0 {
			t.Errorf("%s after defaults: %s", name, dump(issues))
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/storymaker/ -run 'TestRealCatalog' -count=1`
Expected: `TestRealCatalogLoadsAndItsFilesExist` passes, `TestRealCatalogValidatesTheFixtureStories` FAILS (`open testdata/player/castsheet.json: no such file or directory`)

- [ ] **Step 3: Write the cast sheet story and the Node checks**

The cast sheet is a story over the real catalog IDs: all four characters in front (a `look` beat at the camera), three-quarter (default) and profile (walking) views; the poses stand, think, wave, present, arms-crossed, hands-on-hips, point-left and point-right, `point-left`/`point-right` facing both ways, `sit` and `typing` at the desk spots of `env.office` and `env.home-office`; the ten expressions (the last two through an `emote` beat); the nine gestures (every gesture is in its held part 0.7 s into its scene); every environment in day and night with the four characters on its spots and a prop on the `board` spot; the 23 props on two scenes; a title, a wrapped caption and a label text (with umlauts); a close and a medium framing with a pan (parallax). Every scene lasts 1.2 s and cuts in, so scene `i` starts at frame `36 i` and frame `36 i + 21` shows its held state; each scene carries a label text that names what it shows. Create `internal/storymaker/testdata/player/castsheet.json`:

```json
{"schema":"storymaker/1","title":"Cast sheet","format":{"aspect":"16:9","fps":30},"style":{"palette":"sunny","font":"default","captions":true},"voice":null,"music":null,
 "cast":[{"id":"anna","name":"Anna","character":"char.office-woman-1"},{"id":"ben","name":"Ben","character":"char.office-man-1"},{"id":"cem","name":"Cem","character":"char.it-tech-1"},{"id":"dieter","name":"Dieter","character":"char.senior-man-1"}],
 "scenes":[
  {"id":"views-front","environment":"env.plain-stage","transition":"cut","duration":1.2,"actors":[{"cast":"anna","at":"far-left","pose":"stand"},{"cast":"ben","at":"left","pose":"stand"},{"cast":"cem","at":"right","pose":"stand"},{"cast":"dieter","at":"far-right","pose":"stand"}],"beats":[{"do":"look","who":"anna","target":"camera"},{"do":"look","who":"ben","target":"camera","with":"prev"},{"do":"look","who":"cem","target":"camera","with":"prev"},{"do":"look","who":"dieter","target":"camera","with":"prev"},{"do":"show","text":"front","at":"center","style":"label","entry":"none","with":"prev"}]},
  {"id":"views-three-quarter","environment":"env.plain-stage","transition":"cut","duration":1.2,"actors":[{"cast":"anna","at":"far-left","pose":"stand"},{"cast":"ben","at":"left","pose":"stand"},{"cast":"cem","at":"right","pose":"stand"},{"cast":"dieter","at":"far-right","pose":"stand"}],"beats":[{"do":"show","text":"three quarter","at":"center","style":"label","entry":"none"}]},
  {"id":"views-profile","environment":"env.plain-stage","transition":"cut","duration":1.2,"actors":[{"cast":"anna","at":"far-left"},{"cast":"ben","at":"left"},{"cast":"cem","at":"center"},{"cast":"dieter","at":"right"}],"beats":[{"do":"walk","who":"anna","to":"left"},{"do":"walk","who":"ben","to":"center","with":"prev"},{"do":"walk","who":"cem","to":"right","with":"prev"},{"do":"walk","who":"dieter","to":"far-right","with":"prev"},{"do":"show","text":"profile (walking)","at":"center","style":"label","entry":"none","with":"prev"}]},
  {"id":"poses-1","environment":"env.plain-stage","transition":"cut","duration":1.2,"actors":[{"cast":"anna","at":"far-left","pose":"stand"},{"cast":"ben","at":"left","pose":"think"},{"cast":"cem","at":"right","pose":"wave"},{"cast":"dieter","at":"far-right","pose":"present"}],"beats":[{"do":"show","text":"stand think wave present","at":"center","style":"label","entry":"none"}]},
  {"id":"poses-2","environment":"env.plain-stage","transition":"cut","duration":1.2,"actors":[{"cast":"anna","at":"far-left","pose":"arms-crossed"},{"cast":"ben","at":"left","pose":"hands-on-hips"},{"cast":"cem","at":"right","pose":"point-left"},{"cast":"dieter","at":"far-right","pose":"point-right"}],"beats":[{"do":"show","text":"arms-crossed hands-on-hips point-left point-right","at":"center","style":"label","entry":"none"}]},
  {"id":"poses-point-facing","environment":"env.plain-stage","transition":"cut","duration":1.2,"actors":[{"cast":"anna","at":"far-left","pose":"point-left","facing":"right"},{"cast":"ben","at":"left","pose":"point-left","facing":"left"},{"cast":"cem","at":"right","pose":"point-right","facing":"right"},{"cast":"dieter","at":"far-right","pose":"point-right","facing":"left"}],"beats":[{"do":"show","text":"point-left / point-right, facing right and left","at":"center","style":"label","entry":"none"}]},
  {"id":"desk-typing","environment":"env.office","transition":"cut","duration":1.2,"actors":[{"cast":"anna","at":"desk","pose":"typing"},{"cast":"ben","at":"center","pose":"point-right"},{"cast":"cem","at":"right"},{"cast":"dieter","at":"far-right","pose":"present"}],"beats":[{"do":"show","text":"typing at the desk","at":"center","style":"label","entry":"none"}]},
  {"id":"desk-sit","environment":"env.home-office","transition":"cut","duration":1.2,"variant":"night","actors":[{"cast":"anna","at":"desk","pose":"sit","expression":"happy"},{"cast":"ben","at":"center","pose":"hands-on-hips"},{"cast":"cem","at":"right"},{"cast":"dieter","at":"far-right","pose":"present"}],"beats":[{"do":"show","text":"sit at the desk","at":"center","style":"label","entry":"none"}]},
  {"id":"expr-1","environment":"env.plain-stage","transition":"cut","duration":1.2,"actors":[{"cast":"anna","at":"far-left","pose":"stand","expression":"neutral"},{"cast":"ben","at":"left","pose":"stand","expression":"happy"},{"cast":"cem","at":"right","pose":"stand","expression":"laugh"},{"cast":"dieter","at":"far-right","pose":"stand","expression":"sad"}],"beats":[{"do":"show","text":"neutral happy laugh sad","at":"center","style":"label","entry":"none"}]},
  {"id":"expr-2","environment":"env.plain-stage","transition":"cut","duration":1.2,"actors":[{"cast":"anna","at":"far-left","pose":"stand","expression":"angry"},{"cast":"ben","at":"left","pose":"stand","expression":"surprised"},{"cast":"cem","at":"right","pose":"stand","expression":"confused"},{"cast":"dieter","at":"far-right","pose":"stand","expression":"thinking"}],"beats":[{"do":"show","text":"angry surprised confused thinking","at":"center","style":"label","entry":"none"}]},
  {"id":"expr-3","environment":"env.plain-stage","transition":"cut","duration":1.2,"actors":[{"cast":"anna","at":"left","expression":"worried"},{"cast":"ben","at":"right"}],"beats":[{"do":"emote","who":"ben","expression":"determined"},{"do":"show","text":"worried, determined (emote beat)","at":"center","style":"label","entry":"none","with":"prev"}]},
  {"id":"gest-1","environment":"env.plain-stage","transition":"cut","duration":1.2,"actors":[{"cast":"anna","at":"far-left","pose":"stand"},{"cast":"ben","at":"left","pose":"stand"},{"cast":"cem","at":"right","pose":"stand"},{"cast":"dieter","at":"far-right","pose":"stand"}],"beats":[{"do":"gesture","who":"anna","gesture":"point"},{"do":"gesture","who":"ben","gesture":"wave","with":"prev"},{"do":"gesture","who":"cem","gesture":"shrug","with":"prev"},{"do":"gesture","who":"dieter","gesture":"think","with":"prev"},{"do":"show","text":"point wave shrug think","at":"center","style":"label","entry":"none","with":"prev"}]},
  {"id":"gest-2","environment":"env.plain-stage","transition":"cut","duration":1.2,"actors":[{"cast":"anna","at":"far-left","pose":"stand"},{"cast":"ben","at":"left","pose":"stand"},{"cast":"cem","at":"right","pose":"stand"},{"cast":"dieter","at":"far-right","pose":"stand"}],"beats":[{"do":"gesture","who":"anna","gesture":"present"},{"do":"gesture","who":"ben","gesture":"thumbs-up","with":"prev"},{"do":"gesture","who":"cem","gesture":"celebrate","with":"prev"},{"do":"gesture","who":"dieter","gesture":"facepalm","with":"prev"},{"do":"show","text":"present thumbs-up celebrate facepalm","at":"center","style":"label","entry":"none","with":"prev"}]},
  {"id":"gest-3","environment":"env.plain-stage","transition":"cut","duration":1.2,"actors":[{"cast":"anna","at":"far-left"},{"cast":"ben","at":"left"}],"props":[{"id":"padlock","item":"prop.lock","at":"far-right","layer":"front"}],"beats":[{"do":"gesture","who":"anna","gesture":"count"},{"do":"gesture","who":"ben","gesture":"point","target":"padlock","with":"prev"},{"do":"show","text":"count, point at the padlock","at":"center","style":"label","entry":"none","with":"prev"}]},
  {"id":"env-plain-stage-day","environment":"env.plain-stage","transition":"cut","duration":1.2,"variant":"day","actors":[{"cast":"anna","at":"left"},{"cast":"ben","at":"center","pose":"point-right"},{"cast":"cem","at":"right"},{"cast":"dieter","at":"far-right","pose":"present"}],"beats":[{"do":"show","text":"plain-stage day","at":"center","style":"label","entry":"none"}],"props":[{"item":"prop.shield","at":"board","layer":"front"}]},
  {"id":"env-plain-stage-night","environment":"env.plain-stage","transition":"cut","duration":1.2,"variant":"night","actors":[{"cast":"anna","at":"left"},{"cast":"ben","at":"center","pose":"point-right"},{"cast":"cem","at":"right"},{"cast":"dieter","at":"far-right","pose":"present"}],"beats":[{"do":"show","text":"plain-stage night","at":"center","style":"label","entry":"none"}],"props":[{"item":"prop.shield","at":"board","layer":"front"}]},
  {"id":"env-office-day","environment":"env.office","transition":"cut","duration":1.2,"variant":"day","actors":[{"cast":"anna","at":"desk","pose":"typing"},{"cast":"ben","at":"center","pose":"point-right"},{"cast":"cem","at":"right"},{"cast":"dieter","at":"far-right","pose":"present"}],"beats":[{"do":"show","text":"office day","at":"center","style":"label","entry":"none"}],"props":[{"item":"prop.lock","at":"board","layer":"front"}]},
  {"id":"env-office-night","environment":"env.office","transition":"cut","duration":1.2,"variant":"night","actors":[{"cast":"anna","at":"desk","pose":"typing"},{"cast":"ben","at":"center","pose":"point-right"},{"cast":"cem","at":"right"},{"cast":"dieter","at":"far-right","pose":"present"}],"beats":[{"do":"show","text":"office night","at":"center","style":"label","entry":"none"}],"props":[{"item":"prop.lock","at":"board","layer":"front"}]},
  {"id":"env-home-office-day","environment":"env.home-office","transition":"cut","duration":1.2,"variant":"day","actors":[{"cast":"anna","at":"desk","pose":"typing"},{"cast":"ben","at":"center","pose":"point-right"},{"cast":"cem","at":"right"},{"cast":"dieter","at":"far-right","pose":"present"}],"beats":[{"do":"show","text":"home-office day","at":"center","style":"label","entry":"none"}]},
  {"id":"env-home-office-night","environment":"env.home-office","transition":"cut","duration":1.2,"variant":"night","actors":[{"cast":"anna","at":"desk","pose":"typing"},{"cast":"ben","at":"center","pose":"point-right"},{"cast":"cem","at":"right"},{"cast":"dieter","at":"far-right","pose":"present"}],"beats":[{"do":"show","text":"home-office night","at":"center","style":"label","entry":"none"}]},
  {"id":"env-abstract-tech-day","environment":"env.abstract-tech","transition":"cut","duration":1.2,"variant":"day","actors":[{"cast":"anna","at":"left"},{"cast":"ben","at":"center","pose":"point-right"},{"cast":"cem","at":"right"},{"cast":"dieter","at":"far-right","pose":"present"}],"beats":[{"do":"show","text":"abstract-tech day","at":"center","style":"label","entry":"none"}],"props":[{"item":"prop.cloud","at":"board","layer":"front"}]},
  {"id":"env-abstract-tech-night","environment":"env.abstract-tech","transition":"cut","duration":1.2,"variant":"night","actors":[{"cast":"anna","at":"left"},{"cast":"ben","at":"center","pose":"point-right"},{"cast":"cem","at":"right"},{"cast":"dieter","at":"far-right","pose":"present"}],"beats":[{"do":"show","text":"abstract-tech night","at":"center","style":"label","entry":"none"}],"props":[{"item":"prop.cloud","at":"board","layer":"front"}]},
  {"id":"props-1","environment":"env.plain-stage","transition":"cut","duration":1.2,"props":[{"item":"prop.laptop","at":{"x":8,"y":48},"layer":"front"},{"item":"prop.phone","at":{"x":24,"y":48},"layer":"front"},{"item":"prop.monitor","at":{"x":40,"y":48},"layer":"front"},{"item":"prop.server","at":{"x":56,"y":48},"layer":"front"},{"item":"prop.router","at":{"x":72,"y":48},"layer":"front"},{"item":"prop.cloud","at":{"x":88,"y":48},"layer":"front"},{"item":"prop.database","at":{"x":8,"y":92},"layer":"front"},{"item":"prop.lock","at":{"x":24,"y":92},"layer":"front"},{"item":"prop.key","at":{"x":40,"y":92},"layer":"front"},{"item":"prop.shield","at":{"x":56,"y":92},"layer":"front"},{"item":"prop.mail","at":{"x":72,"y":92},"layer":"front"},{"item":"prop.document","at":{"x":88,"y":92},"layer":"front"}],"beats":[{"do":"show","text":"props 1 of 2","at":"center","style":"label","entry":"none"}]},
  {"id":"props-2","environment":"env.plain-stage","transition":"cut","duration":1.2,"props":[{"item":"prop.coffee-cup","at":{"x":8,"y":48},"layer":"front"},{"item":"prop.desk","at":{"x":24,"y":48},"layer":"front"},{"item":"prop.chair","at":{"x":40,"y":48},"layer":"front"},{"item":"prop.plant","at":{"x":56,"y":48},"layer":"front"},{"item":"prop.check","at":{"x":72,"y":48},"layer":"front"},{"item":"prop.cross","at":{"x":88,"y":48},"layer":"front"},{"item":"prop.warning","at":{"x":8,"y":92},"layer":"front"},{"item":"prop.idea","at":{"x":24,"y":92},"layer":"front"},{"item":"prop.question","at":{"x":40,"y":92},"layer":"front"},{"item":"prop.arrow-right","at":{"x":56,"y":92},"layer":"front"},{"item":"prop.gear","at":{"x":72,"y":92},"layer":"front"}],"beats":[{"do":"show","text":"props 2 of 2","at":"center","style":"label","entry":"none"}]},
  {"id":"text-styles","environment":"env.abstract-tech","transition":"cut","duration":1.2,"actors":[{"cast":"anna","at":"left","pose":"present"}],"beats":[{"do":"show","text":"Was ist eine Firewall? ÄÖÜ äöüß","at":{"x":50,"y":14},"entry":"none"},{"do":"show","text":"Plötzlich klopft ein Unbekannter an die Tür und fragt, ob er herein darf, weil er das Paket abholen möchte.","at":{"x":50,"y":88},"style":"caption","entry":"none","with":"prev"},{"do":"show","text":"Paket 3 von 4","at":{"x":72,"y":46},"style":"label","entry":"none","with":"prev"}]},
  {"id":"cam-close","environment":"env.office","transition":"cut","duration":1.2,"camera":{"framing":"close","target":"anna"},"actors":[{"cast":"anna","at":"desk","pose":"typing"},{"cast":"ben","at":"center","pose":"present"}],"beats":[{"do":"show","text":"close framing","at":"center","style":"label","entry":"none"}]},
  {"id":"cam-pan","environment":"env.office","transition":"cut","duration":1.2,"camera":{"framing":"medium"},"actors":[{"cast":"anna","at":"desk","pose":"typing"},{"cast":"ben","at":"center","pose":"point-right"},{"cast":"cem","at":"right"},{"cast":"dieter","at":"far-right","pose":"present"}],"beats":[{"do":"camera","op":"pan","amount":30},{"do":"show","text":"medium framing, pan right","at":"center","style":"label","entry":"none","with":"prev"}]}
 ]}
```

Append these two sections to `scripts/storymaker-tests/11-real-catalog.mjs` (and nothing else changes in the file):

```js
// ---- the data of the real catalog ---------------------------------------------------------------------------------------

const castsheet = playerStory('castsheet');

check('real catalog: every character x view x expression resolves every slot to a variant or a deliberate "none"', () => {
    const rig = files.rig, R = SM.rig, expressions = catalog.entries.filter(e => e.kind === 'expression'), characters = catalog.entries.filter(e => e.kind === 'character');
    assert.equal(characters.length, 4);
    assert.equal(expressions.length, 10);
    const valueOf = (params, slot) => {
        const outfit = rig.params.outfit[params.outfit];
        switch (slot.param) {
            case 'hair_back': return params.hair;
            case 'top': case 'bottom': case 'shoes': return outfit[slot.param];
            case 'eyes': case 'brows': case 'mouth': case 'hand': return 'drawn';
            default: return params[slot.param];
        }
    };
    let resolved = 0, none = 0;
    for (const ch of characters) {
        for (const view of R.VIEWS) {
            for (const expr of expressions) {
                const state = { expr, hands: {}, mouth: null, blink: false };
                for (const slot of rig.slots) {
                    const shapes = R.pickVariant(rig, ch.params, slot, state, view), where = ch.id + ' ' + view + ' ' + expr.name + ' ' + slot.name;
                    if (shapes === null) {
                        assert.equal(valueOf(ch.params, slot), 'none', where + ' has no variant and is not a deliberate none');
                        none++;
                    } else {
                        assert.ok(Array.isArray(shapes), where);
                        resolved++;
                    }
                }
            }
        }
    }
    assert.ok(resolved > 4 * 3 * 10 * 20 && none > 0, resolved + ' resolved, ' + none + ' deliberate');
    // every variant has all three views, so no view silently falls back to another
    for (const slot of rig.slots) {
        for (const [key, v] of Object.entries(rig.variants[slot.name])) for (const view of R.VIEWS) assert.ok(Array.isArray(v[view]), slot.name + '/' + key + ' lacks ' + view);
    }
    // every hand shape a pose or gesture names, and every mouth an expression names, exists
    const handShapes = Object.keys(rig.variants.hand_l), mouths = Object.keys(rig.variants.mouth);
    for (const e of catalog.entries) {
        for (const h of Object.values(e.hands || {})) assert.ok(handShapes.includes(h), e.id + ' hand ' + h);
        for (const k of e.keys || []) for (const h of Object.values(k.hands || {})) assert.ok(handShapes.includes(h), e.id + ' key hand ' + h);
        if (e.kind === 'expression') assert.ok(mouths.includes(e.mouth), e.id + ' mouth ' + e.mouth);
    }
});

check('real catalog: firewall, minimal and the cast sheet compile with only the issues of a silent phase 1', () => {
    const allowed = new Set(['narration_missing', 'not_supported_yet', 'layout_unverified']);
    for (const [name, s] of [['firewall', firewall], ['minimal', story('minimal')], ['castsheet', castsheet]]) {
        const out = compileReal(s), report = out.report, codes = [...report.issues, ...report.unverified].map(i => i.code);
        assert.deepEqual(codes.filter(c => !allowed.has(c)), [], name + ': ' + JSON.stringify(report.issues.filter(i => !allowed.has(i.code))));
        assert.equal(report.catalog_version, catalog.version);
        assert.ok(out.needs.includes('catalog:rig/human.json') || name === 'minimal', name + ' needs the rig');
    }
    assert.equal(compileReal(castsheet).report.issues.length, 0, 'the cast sheet has no issue at all');
    const ranges = compiled.report.scenes;
    assert.deepEqual(plain(ranges.map(s => s.id)), ['s1', 's2', 's3']);
    assert.ok(ranges.every((s, i) => s.end_frame > s.start_frame && (i === 0 || s.start_frame < ranges[i - 1].end_frame)), 'scenes follow each other, transitions overlap');
    assert.equal(compiled.report.frame_count, Math.max(...ranges.map(s => s.end_frame)));
});

// drawn(tl, n, width): the call log of frame n, with the logs of the two transition canvases appended
function drawn(tl, n, width) {
    const cv = new FakeCanvas(width || tl.width, width ? Math.round(width * tl.height / tl.width) : tl.height), extra = [];
    const canvas = (name, w, h) => { const c = new FakeCanvas(w, h); extra.push(c); return { canvas: c, ctx: c.ctx }; };
    SM.renderFrame(tl, n, cv.ctx, realAssets({ canvas }), width ? { width } : undefined);
    return [...cv.ctx.log, ...extra.flatMap(c => c.ctx.log)];
}
const noMagenta = (log, what) => assert.ok(!log.some(l => /#ff00ff/i.test(l)), what + ' draws an unresolved token colour');

check('real catalog: frames of firewall.json in 16:9, 9:16 and 1:1 draw without an unresolved token', () => {
    SM.parts.resetUnresolved();
    for (const aspect of ['16:9', '9:16', '1:1']) {
        const tl = compileReal({ ...firewall, format: { aspect, fps: 30 } }).timeline;
        for (const n of [0, 40, 100, 114, 125, 150, 200, 233, 300, tl.frame_count - 1]) {
            const log = drawn(tl, n);
            assert.ok(log.length > 10, aspect + ' frame ' + n + ' draws something (a code scene draws its placeholder)');
            noMagenta(log, aspect + ' frame ' + n);
        }
    }
    assert.deepEqual(plain(SM.parts.unresolvedTokens()), []);
});

check('real catalog: every scene of the cast sheet, in both palettes, draws without an unresolved token or an exception', () => {
    SM.parts.resetUnresolved();
    for (const palette of ['sunny', 'calm']) {
        const tl = compileReal({ ...castsheet, style: { ...castsheet.style, palette } }).timeline;
        assert.equal(tl.scenes.length, castsheet.scenes.length);
        for (const sc of tl.scenes) {
            const log = drawn(tl, sc.start_frame + 21);
            assert.ok(log.length > 100, sc.id + ' draws something');
            noMagenta(log, palette + ' ' + sc.id);
            // the scene's entities are all on the canvas: every actor's drawn parts exist (rig slots) and the text was laid out
            if (sc.entities.some(e => e.type === 'actor')) assert.ok(log.filter(l => l.startsWith('setTransform')).length > 25, sc.id + ' draws the actors');
        }
    }
    assert.deepEqual(plain(SM.parts.unresolvedTokens()), []);
});

check('real catalog: a prop stands on its spot (anchor bottom centre, size.h of the stage height) and a text keeps its style', () => {
    const tl = compileReal({ scenes: [{ id: 'a', environment: 'env.plain-stage', props: [{ item: 'prop.lock', at: 'right' }], beats: [{ do: 'show', text: 'Paket', style: 'caption', at: 'center', entry: 'none' }] }] }).timeline;
    const log = drawn(tl, 5), sc = tl.scenes[0], stage = SM.render.stageFor(sc, SM.render.makeContext(tl, realAssets(), 1920, 1080));
    const lock = entry('prop.lock'), vb = files.props['props/lock.json'].viewbox, depth = stage.depthScale(0.88 * 1080), k = lock.size.h * 1080 * depth / vb[3];
    const tx = 0.7 * 1920 - k * (vb[0] + 0.5 * vb[2]), ty = 0.88 * 1080 - k * (vb[1] + vb[3]);
    assert.ok(log.some(l => l.startsWith('setTransform(' + [k, 0, 0, k, tx, ty].map(v => String(Math.round(v * 1000) / 1000)).join(','))), 'the padlock is placed with its bottom centre on the spot');
    assert.ok(log.some(l => l === 'set font=500 45px "Geist", sans-serif'), 'the caption uses the weight 500 face of the default font at 0.042 of the height');
    assert.ok(log.some(l => l.startsWith('fillText(Paket,960,')), 'the caption is centred');
});

check('real catalog: a left-facing actor is mirrored as a whole about the root', () => {
    const mk = at => compileReal({ cast: [{ id: 'a', character: 'char.office-man-1' }], scenes: [{ id: 's', actors: [{ cast: 'a', at, facing: 'left' }] }] }).timeline;
    const log = drawn(mk('center'), 3), flips = log.filter(l => l.startsWith('setTransform(-'));
    assert.ok(flips.length > 20, 'the rig parts are drawn under a mirrored matrix');
});

// ---- the remaining rules of the catalog: views, walking, bodies, spots with a depth, fonts -------------------------------

check('real catalog: pose view hints, the walk cycle and its speed, body types, spot depth and the four font weights', () => {
    // pose.view hints, walking wins over everything
    assert.equal(rigStateOf({ pose: { a: 'arms-crossed', b: 'arms-crossed', u: 1 } }).view, 'front');
    assert.equal(rigStateOf({ pose: { a: 'wave', b: 'wave', u: 1 } }).view, 'three_quarter');
    assert.equal(rigStateOf({ pose: { a: 'arms-crossed', b: 'arms-crossed', u: 1 }, walkTime: 0.1 }).view, 'profile');
    // cycle keys: offset_y is positive = down (the root drops at the contact pose), the walk is periodic with its catalog period
    assert.equal(rigStateOf({ walkTime: 0 }).offsetY, 21);
    assert.equal(rigStateOf({ walkTime: 0.25 }).offsetY, 0);
    assert.equal(rigStateOf({ walkTime: 1 }).offsetY, 21, 'one period later: the same pose');
    // walk.speed (16) is percent of the design width per second at depth 1: left (30 %) to centre (50 %) at 88 % height
    const out = compileReal({ cast: [{ id: 'anna', character: 'char.office-woman-1' }], scenes: [{ id: 's', actors: [{ cast: 'anna', at: 'left' }], beats: [{ do: 'walk', who: 'anna', to: 'center' }] }] });
    const actor = out.timeline.scenes[0].entities[0], stage = SM.render.stageFor(out.timeline.scenes[0], SM.render.makeContext(out.timeline, realAssets(), 1920, 1080));
    near(actor.x.end(), 0.2 * 1920 / (0.16 * 1920 * stage.depthScale(0.88 * 1080)), 1e-6, 'seconds of the walk');
    // body types: a rest override merges per coordinate, the scale multiplies the screen scale
    const R = SM.rig, rig = files.rig, broad = R.boneMatrices(rig, 'front', R.bodyOf(rig, 'broad'), {}), child = R.boneMatrices(rig, 'front', R.bodyOf(rig, 'child'), {});
    near(broad.upper_arm_r[4], 96, 1e-9); near(broad.upper_arm_r[5], -440 - 25 - 209, 1e-9);
    near(child.hips[5], -375, 1e-9);
    near(R.scaleFor(rig, 1080, 1, 1, R.bodyOf(rig, 'elder')), 0.55 * 1080 * 0.97 / 1000, 1e-9);
    // a spot with a depth (0 far .. 1 near) overrides the floor lookup; the starter catalog has none, so build one
    const env = { ...entry('env.office'), spots: { ...entry('env.office').spots, shelf: { x: 80, y: 60, depth: 1 } } };
    const st = SM.stage.create({ aspect: '16:9', env }), shelf = st.spot('shelf');
    assert.equal(shelf.depth, 1);
    near(st.depthScale(shelf.y, shelf.depth), env.floor.scale_near, 1e-9);
    // fonts: the default font has four weights, the text styles pick 700, 500 and 600
    assert.deepEqual(catalog.fonts.map(f => f.weight), [400, 500, 600, 700]);
    const texts = compileReal({ scenes: [{ id: 'a', beats: [
        { do: 'show', text: 'a', entry: 'none' }, { do: 'show', text: 'b', style: 'caption', at: { x: 50, y: 80 }, entry: 'none', with: 'prev' },
        { do: 'show', text: 'c', style: 'label', at: { x: 20, y: 50 }, entry: 'none', with: 'prev' }] }] });
    assert.deepEqual(plain(texts.needs.filter(n => n.startsWith('font:'))), ['font:default/500', 'font:default/600', 'font:default/700']);
});
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: `71 passed, 0 failed` (the new checks are guards of the integrated state and pass at once)
Run: `go test ./internal/storymaker/... -count=1`
Expected: `ok` (the Go guard validates `minimal.json`, `firewall.json` and the cast sheet with zero issues, before and after `ApplyDefaults`)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/testdata/player/castsheet.json scripts/storymaker-tests/11-real-catalog.mjs internal/storymaker/catalog_real_test.go
git commit -m "test(storymaker): real catalog integration checks and cast sheet story" -m "Node checks over the real starter catalog, the cast sheet story and a Go guard for catalog and fixture stories. Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task R9: Visual parity browser test: the real catalog rendered by the real render page

> Part D reviewed its catalog with its own SVG renderer; this renders the same catalog with the player through the render page (contract section 7, png transport) in the installed Chrome and writes chosen frames as PNG: the cast sheet (27 scenes, one frame each at 0.7 s) and `firewall.json` in 16:9, 9:16 and 1:1 (three frames per storyboard scene, the middle of the code scene), plus half-size contact sheets. The test asserts what a machine can judge (frame size, no compile error, no `#ff00ff` pixel, not flat); the comparison with D's sheets is a human look at the PNGs. Gated by `AURAGO_RUN_BROWSER_SMOKE=1` and `AURAGO_STORYMAKER_PARITY_DIR`.

**Files:**
- Create: `ui/storymaker_parity_browser_test.go`
- Output (git-ignored, not committed): `disposable/storymaker-parity/*.png`

- [ ] **Step 1: Write the browser test**

The harness serves the render page (contract section 6 markup under the render CSP of section 7), `boot.json` with the four Geist faces of the catalog, the story, the real catalog files, the player bundles and an ingest at one fixed URL, exactly like the firewall render test of the gate file; it answers `{stop:true}` after the last wanted frame, so only the frames up to it are rendered.

```go
package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/go-rod/rod"
)

// Storymaker visual parity (part R): the real starter catalog (ui/img/storymaker, rendered by part D's own SVG renderer for its
// review sheets) is rendered by the real player through the real render page (contract section 7, png transport) in the
// installed Chrome, and chosen frames are written as PNG files to look at: the cast sheet in 16:9, firewall.json in 16:9, 9:16
// and 1:1. The test asserts only what a machine can judge (frame size, the story reports no error, no pixel is the unresolved
// token colour #ff00ff, the frames are not flat); the comparison with D's sheets is a human look at the PNGs.
//
//	AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_STORYMAKER_PARITY_DIR=<dir> go test ./ui -run TestStorymakerParityBrowser -count=1 -timeout 20m
//
// AURAGO_STORYMAKER_PARITY_ONLY=castsheet (or firewall) renders only the jobs whose name contains the text.

type storymakerParityScene struct {
	ID         string
	Start, End int
}

type storymakerParityJob struct {
	name          string
	story         string // a file below internal/storymaker/testdata
	aspect        string // overrides story.format.aspect when set
	width, height int
	cols          int // columns of the contact sheets (half-size tiles)
	rows          int
	pick          func(scenes []storymakerParityScene) (frames map[int]string, order []string)
}

type storymakerParityResult struct {
	frames map[string][]byte
	report map[string]any
	errors []map[string]any
}

// parityScenes reads the scene ranges of a compile report.
func parityScenes(report map[string]any) []storymakerParityScene {
	var out []storymakerParityScene
	raw, _ := report["scenes"].([]any)
	for _, s := range raw {
		m, _ := s.(map[string]any)
		id, _ := m["id"].(string)
		start, _ := m["start_frame"].(float64)
		end, _ := m["end_frame"].(float64)
		out = append(out, storymakerParityScene{ID: id, Start: int(start), End: int(end)})
	}
	return out
}

// parityStory reads a fixture story and optionally changes its aspect.
func parityStory(t *testing.T, rel, aspect string) []byte {
	t.Helper()
	data := storymakerFile(t, "../internal/storymaker/testdata/"+rel)
	if aspect == "" {
		return data
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}
	format, _ := doc["format"].(map[string]any)
	format["aspect"] = aspect
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func runStorymakerParity(t *testing.T, browser *rod.Browser, job storymakerParityJob) storymakerParityResult {
	t.Helper()
	const prefix = "/fedcba9876543210fedcba9876543210"
	res := storymakerParityResult{frames: map[string][]byte{}}
	var catalog struct {
		Fonts []struct {
			Name, Family, File string
			Weight             int
		} `json:"fonts"`
	}
	if err := json.Unmarshal(storymakerFile(t, "img/storymaker/catalog.json"), &catalog); err != nil {
		t.Fatalf("catalog.json: %v", err)
	}
	storyJSON := parityStory(t, job.story, job.aspect)
	var mu sync.Mutex
	var wanted map[int]string
	last, total := -1, 0
	done := make(chan struct{})
	var closeOnce sync.Once
	finish := func() { closeOnce.Do(func() { close(done) }) }
	var origin string
	mux := http.NewServeMux()
	handle := func(p string, fn http.HandlerFunc) {
		mux.HandleFunc(prefix+p, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-SM-Frame")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			fn(w, r)
		})
	}
	handle("/", func(w http.ResponseWriter, r *http.Request) {
		base := origin + prefix
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", fmt.Sprintf(storymakerRenderCSP, base))
		_, _ = fmt.Fprintf(w, `<!doctype html><html data-sm-mode="render" data-sm-base="%[1]s"><head><meta charset="utf-8"></head><body><canvas id="sm-stage" width="%[2]d" height="%[3]d"></canvas><script src="%[1]s/js/storymaker/host.bundle.js"></script></body></html>`, base, job.width, job.height)
	})
	handle("/boot.json", func(w http.ResponseWriter, r *http.Request) {
		base := origin + prefix
		fonts := []map[string]any{}
		for _, f := range catalog.Fonts {
			fonts = append(fonts, map[string]any{"name": f.Name, "family": f.Family, "weight": f.Weight, "url": base + f.File})
		}
		codeBundles := map[string]string{}
		if strings.Contains(job.story, "firewall") {
			codeBundles["s3"] = base + "/scenes/s3.js"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mode": "render", "fps": 30, "width": job.width, "height": job.height, "transport": "png", "frame_limit": 4000,
			"story_url": base + "/story.json", "catalog_url": base + "/catalog/catalog.json", "assets_base": base + "/catalog/",
			"code_bundles": codeBundles, "fonts": fonts, "audio_info": map[string]any{},
		})
	})
	handle("/story.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(storyJSON)
	})
	handle("/catalog/", func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, prefix+"/catalog/")
		if strings.Contains(rel, "..") || !strings.HasSuffix(rel, ".json") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(storymakerFile(t, "img/storymaker/"+path.Clean(rel)))
	})
	handle("/fonts/", func(w http.ResponseWriter, r *http.Request) {
		name := path.Base(r.URL.Path)
		allowed := false
		for _, f := range catalog.Fonts {
			allowed = allowed || path.Base(f.File) == name
		}
		if !allowed {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "font/woff2")
		_, _ = w.Write(storymakerFile(t, "fonts/"+name))
	})
	for _, bundle := range []string{"host", "player"} {
		file := bundle + ".bundle.js"
		handle("/js/storymaker/"+file, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			_, _ = w.Write(storymakerFile(t, "js/storymaker/"+file))
		})
	}
	handle("/scenes/s3.js", func(w http.ResponseWriter, r *http.Request) {
		src := storymakerFile(t, "../internal/storymaker/testdata/scenes/s3.js")
		out := api.Transform(string(src), api.TransformOptions{Format: api.FormatIIFE, GlobalName: "__smSceneModule", Target: api.ES2020, LegalComments: api.LegalCommentsNone, Loader: api.LoaderJS, Sourcefile: "scenes/s3.js"})
		if len(out.Errors) != 0 {
			http.Error(w, out.Errors[0].Text, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = w.Write(append(out.Code, []byte("\nself.__smScene=__smSceneModule.default;\n")...))
	})
	handle("/report", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		mu.Lock()
		defer mu.Unlock()
		_ = json.Unmarshal(body, &res.report)
		total = int(res.report["frame_count"].(float64))
		wanted, _ = job.pick(parityScenes(res.report))
		for n := range wanted {
			last = max(last, n)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"start":true,"from_frame":0}`)
	})
	handle("/error", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body)
		mu.Lock()
		res.errors = append(res.errors, body)
		mu.Unlock()
		finish()
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	handle("/frame", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(io.LimitReader(r.Body, 64<<20))
		var n int
		_, _ = fmt.Sscanf(r.Header.Get("X-SM-Frame"), "%d", &n)
		mu.Lock()
		defer mu.Unlock()
		if name, ok := wanted[n]; ok {
			res.frames[name] = data
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case n >= last:
			_, _ = io.WriteString(w, `{"stop":true}`)
			finish()
		case n == total-1:
			_, _ = io.WriteString(w, `{"ok":true,"done":true}`)
			finish()
		default:
			_, _ = io.WriteString(w, `{"ok":true}`)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	origin = srv.URL

	page := browser.MustPage(origin + prefix + "/").Timeout(10 * time.Minute)
	defer page.Close()
	select {
	case <-done:
	case <-time.After(9 * time.Minute):
		mu.Lock()
		defer mu.Unlock()
		t.Fatalf("%s: the render page did not finish: frames=%d errors=%v status=%s", job.name, len(res.frames), res.errors, page.MustEval(`() => JSON.stringify(window.__smStatus || null)`).Str())
	}
	mu.Lock()
	defer mu.Unlock()
	return res
}

// halve averages 2x2 pixel blocks.
func halveImage(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()/2, b.Dy()/2))
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			var r, g, bl, a uint32
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					pr, pg, pb, pa := src.At(b.Min.X+2*x+dx, b.Min.Y+2*y+dy).RGBA()
					r, g, bl, a = r+pr, g+pg, bl+pb, a+pa
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r >> 10), uint8(g >> 10), uint8(bl >> 10), uint8(a >> 10)})
		}
	}
	return dst
}

// writeContactSheets tiles half-size copies of the frames, cols x rows tiles per sheet, and writes a text file with the tile names.
func writeContactSheets(t *testing.T, dir, prefix string, names []string, frames map[string][]byte, w, h, cols, rows int) {
	t.Helper()
	per := cols * rows
	for sheet := 0; sheet*per < len(names); sheet++ {
		tiles := names[sheet*per : min(len(names), (sheet+1)*per)]
		img := image.NewRGBA(image.Rect(0, 0, cols*(w/2), rows*(h/2)))
		for i, name := range tiles {
			src, err := png.Decode(bytes.NewReader(frames[name]))
			if err != nil {
				t.Fatalf("decode %s: %v", name, err)
			}
			half := halveImage(src)
			at := image.Pt((i%cols)*(w/2), (i/cols)*(h/2))
			for y := 0; y < half.Bounds().Dy(); y++ {
				for x := 0; x < half.Bounds().Dx(); x++ {
					img.SetRGBA(at.X+x, at.Y+y, half.RGBAAt(x, y))
				}
			}
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		stem := filepath.Join(dir, fmt.Sprintf("%s-sheet-%02d", prefix, sheet+1))
		if err := os.WriteFile(stem+".png", buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(stem+".txt", []byte(strings.Join(tiles, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// checkParityFrame asserts what a machine can judge about one frame.
func checkParityFrame(t *testing.T, name string, data []byte, w, h int) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Errorf("%s: not a PNG: %v", name, err)
		return
	}
	if b := img.Bounds(); b.Dx() != w || b.Dy() != h {
		t.Errorf("%s: %dx%d, want %dx%d", name, b.Dx(), b.Dy(), w, h)
		return
	}
	colours := map[uint32]bool{}
	magenta := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += 1 {
		for x := b.Min.X; x < b.Max.X; x += 1 {
			r, g, bl, _ := img.At(x, y).RGBA()
			r8, g8, b8 := r>>8, g>>8, bl>>8
			if r8 == 255 && g8 == 0 && b8 == 255 {
				magenta++
			}
			if len(colours) < 4096 {
				colours[r8<<16|g8<<8|b8] = true
			}
		}
	}
	if magenta > 0 {
		t.Errorf("%s: %d pixels have the unresolved-token colour #ff00ff", name, magenta)
	}
	if len(colours) < 64 {
		t.Errorf("%s: only %d distinct colours, the frame is nearly flat", name, len(colours))
	}
}

func TestStorymakerParityBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Storymaker parity test")
	}
	dir := os.Getenv("AURAGO_STORYMAKER_PARITY_DIR")
	if dir == "" {
		t.Skip("set AURAGO_STORYMAKER_PARITY_DIR to the folder that receives the PNG frames")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	only := os.Getenv("AURAGO_STORYMAKER_PARITY_ONLY")

	// the cast sheet: every scene lasts 1.2 s with a cut in, so 0.7 s into a scene (21 frames) is where its beats are in their held part
	castPick := func(scenes []storymakerParityScene) (map[int]string, []string) {
		frames, order := map[int]string{}, []string{}
		for i, s := range scenes {
			name := fmt.Sprintf("%02d-%s", i+1, s.ID)
			frames[s.Start+21] = name
			order = append(order, name)
		}
		return frames, order
	}
	// firewall: three frames of each storyboard scene and the middle of the code scene
	firePick := func(scenes []storymakerParityScene) (map[int]string, []string) {
		frames, order := map[int]string{}, []string{}
		for _, s := range scenes {
			for _, pct := range []int{25, 55, 85} {
				if s.ID == "s3" && pct != 55 {
					continue
				}
				name := fmt.Sprintf("%s-%d", s.ID, pct)
				frames[s.Start+(s.End-s.Start)*pct/100] = name
				order = append(order, name)
			}
		}
		return frames, order
	}
	jobs := []storymakerParityJob{
		{name: "castsheet", story: "player/castsheet.json", width: 1920, height: 1080, cols: 2, rows: 3, pick: castPick},
		{name: "firewall-16x9", story: "stories/firewall.json", width: 1920, height: 1080, cols: 2, rows: 3, pick: firePick},
		{name: "firewall-9x16", story: "stories/firewall.json", aspect: "9:16", width: 1080, height: 1920, cols: 4, rows: 1, pick: firePick},
		{name: "firewall-1x1", story: "stories/firewall.json", aspect: "1:1", width: 1080, height: 1080, cols: 3, rows: 2, pick: firePick},
	}
	browser := newSmokeBrowser(t)
	for _, job := range jobs {
		if only != "" && !strings.Contains(job.name, only) {
			continue
		}
		t.Run(job.name, func(t *testing.T) {
			started := time.Now()
			res := runStorymakerParity(t, browser, job)
			if len(res.errors) != 0 {
				t.Fatalf("page errors: %v", res.errors)
			}
			issues, _ := res.report["issues"].([]any)
			for _, raw := range issues {
				if is, _ := raw.(map[string]any); is["severity"] == "error" {
					t.Errorf("the compile report has an error issue: %v", is)
				}
			}
			_, order := job.pick(parityScenes(res.report))
			for _, name := range order {
				data, ok := res.frames[name]
				if !ok {
					t.Errorf("frame %s was never posted", name)
					continue
				}
				checkParityFrame(t, name, data, job.width, job.height)
				if err := os.WriteFile(filepath.Join(dir, job.name+"-"+name+".png"), data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if len(res.frames) == len(order) {
				writeContactSheets(t, dir, job.name, order, res.frames, job.width, job.height, job.cols, job.rows)
			}
			t.Logf("%s: %d frames written to %s in %s", job.name, len(res.frames), dir, time.Since(started).Round(time.Second))
		})
	}
}
```

- [ ] **Step 2: Run the test without its gate variables**

Run: `go test ./ui -run TestStorymakerParityBrowser -count=1 -v`
Expected: `SKIP` (`set AURAGO_RUN_BROWSER_SMOKE=1 to run the Storymaker parity test`)

- [ ] **Step 3: Render the frames**

On this Windows box Chrome only starts from a test binary that lives in `D:\tmp\go-build` (contract rule 0), outside the sandbox, and `AURAGO_STORYMAKER_PARITY_DIR` must be an absolute path (the folder `disposable/` is git-ignored):

```bash
mkdir -p disposable/storymaker-parity
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_STORYMAKER_PARITY_DIR="$(pwd -W)/disposable/storymaker-parity" \
  go test ./ui -run TestStorymakerParityBrowser -count=1 -timeout 20m -v
```
Expected: `PASS` after about two minutes (`castsheet` 27 frames, `firewall-16x9`, `firewall-9x16` and `firewall-1x1` 7 frames each) and the files `castsheet-NN-<scene>.png`, `castsheet-sheet-0N.png` (2 x 3 half-size tiles; the `.txt` next to a sheet lists its tiles), `firewall-<aspect>-<scene>-<pct>.png` and `firewall-<aspect>-sheet-01.png` in the folder. `AURAGO_STORYMAKER_PARITY_ONLY=castsheet` (or `firewall`) renders only the matching jobs.

- [ ] **Step 4: Look at the PNGs next to part D's sheets**

Open `castsheet-sheet-01.png` to `-05.png` and compare them with D's `cast-sheet.png` and `env-prop-sheet.png` (`node scripts/build-storymaker-catalog.mjs --review <dir>` renders D's sheets with its own SVG renderer). What must match: the shapes and colours of all four characters in the three views; the draw order of the limbs (near arm over the torso in three-quarter and profile, both arms in front in the front view); a left-facing actor is the mirror image; the seated woman sits on the chair of the office and the home office (hips at the chair seat, thighs forward, feet on the floor, desk in front); `point-left` and `point-right` reach the named screen side whichever way the actor faces; props stand on the floor line with their bottom centre on the spot, sized by the floor depth; the parallax layers (sky, hills, wall, network) stay behind and move less than the floor in `cam-pan`; title, caption and label are drawn in Geist with the box padding and radius of D's style (a title on a dark environment is light, a label box follows the environment). Known differences that are not defects: the player adds a soft ground shadow under every actor, the idle breathing sways the characters by a pixel or two, and the eyes never blink (the catalog has no blink variants).

- [ ] **Step 5: Run everything once more**

```bash
npm run build:ui
npm run check:ui
node scripts/test-storymaker-player.mjs
node scripts/build-storymaker-catalog.mjs --check
go test ./internal/storymaker/... -count=1
go test ./ui -run 'TestStorymakerCatalog|TestShippedWebAssetsLoadNothingExternal|TestTranslations_' -count=1
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestStorymaker(PlayerGate|RenderMode|FirewallRender|RenderScreenshot)Browser' -count=1 -timeout 15m
```
Expected: bundles unchanged after the build, `71 passed, 0 failed`, `storymaker catalog ab33e88a6b77 is up to date`, `ok` for the Go packages, the four existing browser tests PASS (the firewall render test still renders twice bit-identically).

- [ ] **Step 6: Commit**

```bash
git add ui/storymaker_parity_browser_test.go
git commit -m "test(storymaker): render the real catalog through the render page and dump frames" -m "Opt-in parity test (AURAGO_RUN_BROWSER_SMOKE and AURAGO_STORYMAKER_PARITY_DIR): cast sheet and firewall in three aspects. Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
