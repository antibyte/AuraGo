# Part C: Frame probe, flash and code-scene checks, slow_hook, 9:16 caption

> Generated from the drafted and tested commits 8b8e5922a..962b42edc. Apply the tasks in order. "replace ... with" blocks are exact: the old text occurs once in the file before the task.

### Evidence C1: measure the frame probe cost in the real page and pick the variant

Nothing to apply; recorded while the plan was drafted.

> The probe renders each frame at 64 px width through Engine.render(n, {width: 64}), draws it to a
> small canvas and reads it with getImageData, in the compile check's page after the layout run.
> Measured with a throwaway harness (not committed) on the development box (Windows, Chrome 156,
> 16 logical CPUs, one TestCompileCheckBrowser-style run each):
>
>   16:9, 10 storyboard scenes of 9 s + one 5 s code scene, 91 s, 2730 frames:  13.1 s (4.8 ms/frame)
>   9:16, 10 storyboard scenes of 9 s, no code scene, 86.5 s, 2595 frames:       7.3 s (2.8 ms/frame)
>   9:16, 6 storyboard scenes of 9 s + three 10 s code scenes, 80.5 s, 2415 fr.: 35.7 s (14.8 ms/frame)
>
> Storyboard frames cost about 3 to 5 ms each; a code-scene frame costs 30 to 40 ms (scene worker
> round trip, prepare_frame and its own render). So a 90 s storyboard film fits in the 20 s deadline
> with room to spare, and only code scenes can overrun it.
>
> Decision: probe every frame in film order, storyboard and code scenes alike, and stop at
> PROBE_DEADLINE_MS = 20000. Frames not reached make one flash_unverified. The cut-window variant
> for storyboard scenes is not needed and is not built. A film with more than about 550 code-scene
> frames (18 s) on this box, or a slower machine, ends in flash_unverified for its tail.

### Task C2: the flash rule on a luminance series

**Files:**
- Create: `scripts/storymaker-tests/31-checks.mjs`
- Modify: `ui/js/storymaker/checks.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

Create `scripts/storymaker-tests/31-checks.mjs`:

```js
// Task C2 and following: the measured checks of checks.js, pure functions on the series the frame probe records.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check } = t;
const SM = t.core();
const FPS = 30;

// pulses(period, count) = a series that holds 0.6, then dips to 0.1 and comes back `count` times, one dip every `period` frames.
// Each dip is two opposite transitions of 0.5, so one flash; the dip of flash k starts at frame 1 + k * period.
function pulses(period, count, tail) {
    const y = [0.6, 0.6];
    for (let k = 0; k < count; k++) for (let i = 0; i < period; i++) y.push(i < period / 2 ? 0.1 : 0.6);
    while (y.length < 2 + count * period + (tail || 0)) y.push(0.6);
    return y;
}

check('checks.flashes: a dip and a recovery is one flash and its start frame is the first frame of the dip', () => {
    assert.deepEqual(plain(SM.checks.flashes(pulses(4, 1, 6), FPS)), [1]);
    assert.deepEqual(plain(SM.checks.flashes(pulses(4, 3, 6), FPS)), [1, 5, 9]);
});

check('checks.flashes: three flashes in a second pass the rule only as a list, four give a window', () => {
    assert.equal(SM.checks.flashes(pulses(4, 3, 20), FPS).length, 3);
    assert.equal(SM.checks.flashes(pulses(4, 4, 20), FPS).length, 4);
    assert.deepEqual(plain(SM.checks.flashWindows([1, 5, 9], FPS)), []);
    assert.deepEqual(plain(SM.checks.flashWindows([1, 5, 9, 13], FPS)), [1]);
    // the fourth start exactly one second after the first is outside the window of fps frames
    assert.deepEqual(plain(SM.checks.flashWindows([1, 11, 21, 31], FPS)), []);
    assert.deepEqual(plain(SM.checks.flashWindows([1, 11, 21, 30], FPS)), [1]);
});

check('checks.flashes: a change below 0.10 is no transition', () => {
    const y = [];
    for (let i = 0; i < 60; i++) y.push(i % 2 ? 0.55 : 0.46);
    assert.deepEqual(plain(SM.checks.flashes(y, FPS)), []);
});

check('checks.flashes: alternation with the darker end at 0.80 or above is no flash', () => {
    const bright = [];
    for (let i = 0; i < 60; i++) bright.push(i % 4 < 2 ? 0.8 : 1.0);
    assert.deepEqual(plain(SM.checks.flashes(bright, FPS)), []);
    const dim = bright.map(v => v - 0.01);
    assert.ok(SM.checks.flashes(dim, FPS).length > 4, 'the same swing just below 0.80 is flashing');
});

check('checks.flashes: a slow ramp is one transition, not a flash, and a ramp splits only at its turning point', () => {
    const ramp = [];
    for (let i = 0; i < 30; i++) ramp.push(0.1 + i * 0.02);
    assert.deepEqual(plain(SM.checks.flashes(ramp, FPS)), []);
    const there = ramp.concat(ramp.slice().reverse());
    assert.deepEqual(plain(SM.checks.flashes(there, FPS)), [0]);
});

check('checks.flashes: short and empty series are safe', () => {
    assert.deepEqual(plain(SM.checks.flashes([], FPS)), []);
    assert.deepEqual(plain(SM.checks.flashes([0.5], FPS)), []);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL checks.flashes: a dip and a recovery is one flash and its start frame is the first frame of the dip: TypeError: SM.checks.flashes is not a function

- [ ] **Step 3: Implement**

In `ui/js/storymaker/checks.js`, replace:

```js
// Storymaker player: layout checks. Phase 1 has none: run() returns an empty list. The code scenes are listed as
// unverified (layout inside a code scene is never verified, only code_error, deadlines and the duration bounds are).
// The real checks (off_stage, overlap_face, text_overflow, ...) arrive in phase 3. Classic script; publishes
// StorymakerCore.checks.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    // run(scenes) = check results of the compiled scenes (phase 1: none).
    function run() { return []; }

    // unverified(scenes) = one minor layout_unverified issue per code scene.
```

with:

```js
// Storymaker player: the measured checks. The frame probe of the compile check page records two numbers per frame (see
// host-render.js), and the pure functions here turn that series into findings: flashes (WCAG 2.3.1 style), frozen or empty
// code scenes. Layout findings come from the layout run and Go, not from here. The code scenes are listed as unverified
// (layout inside a code scene is never verified, only code_error, deadlines and the duration bounds are). Classic script;
// publishes StorymakerCore.checks.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var FLASH_STEP = 0.10, FLASH_BRIGHT = 0.80, FLASH_MAX = 3, LEVEL_NOISE = 0.002;

    // transitions(y) = the monotone runs of the luminance series whose total change is at least 0.10 and whose darker end is
    // below 0.80, as {from, to, dir} (dir 1 brightens). A run ends at its first extreme frame and the next one starts on the
    // last frame of the plateau; flat frames inside a run keep it.
    function transitions(y) {
        var runs = [], from = 0, to = 0, dir = 0, i;
        function close() {
            if (dir === 0) return;
            var change = Math.abs(y[to] - y[from]);
            if (change >= FLASH_STEP && Math.min(y[from], y[to]) < FLASH_BRIGHT) runs.push({ from: from, to: to, dir: dir });
        }
        for (i = 1; i < y.length; i++) {
            var dy = y[i] - y[i - 1], s = dy > LEVEL_NOISE ? 1 : dy < -LEVEL_NOISE ? -1 : 0;
            if (s === 0) continue;
            if (dir === 0) { dir = s; from = i - 1; to = i; continue; }
            if (s === dir) { to = i; continue; }
            close();
            from = i - 1; dir = s; to = i;
        }
        close();
        return runs;
    }

    // flashes(y) = the start frames of the flashes in a luminance series: two consecutive transitions in opposite
    // directions, paired from the front, so four transitions are two flashes.
    function flashes(y) {
        var runs = transitions(y || []), out = [];
        for (var i = 0; i + 1 < runs.length; i++) {
            if (runs[i].dir === runs[i + 1].dir) continue;
            out.push(runs[i].from);
            i++;
        }
        return out;
    }

    // flashWindows(starts, fps) = the start frame of every window of fps frames that holds more than 3 flash starts.
    function flashWindows(starts, fps) {
        var out = [];
        for (var i = 0; i + FLASH_MAX < starts.length; i++) {
            if (starts[i + FLASH_MAX] - starts[i] < fps) out.push(starts[i]);
        }
        return out;
    }

    // run(probe, timeline) = the findings of the measured series. Without a probe there is nothing to say.
    function run() { return []; }

    // unverified(scenes) = one minor layout_unverified issue per code scene.
```

In `ui/js/storymaker/checks.js`, replace:

```js
        return out;
    }

    SM.checks = { run: run, unverified: unverified };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

with:

```js
        return out;
    }

    SM.checks = { run: run, unverified: unverified, flashes: flashes, flashWindows: flashWindows };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

- [ ] **Step 4: Rebuild the generated files**

Run: `npm run build:ui` (then `npm run check:ui`).

- [ ] **Step 5: Run the test and see it pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: 241 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/31-checks.mjs ui/js/storymaker/checks.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: the flash rule on a luminance series"
```

### Task C3: SM.checks.run reports flash per scene and flash_unverified

**Files:**
- Modify: `scripts/storymaker-tests/31-checks.mjs`
- Modify: `ui/js/storymaker/checks.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/31-checks.mjs`, replace:

```js
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check } = t;
const SM = t.core();
const FPS = 30;
```

with:

```js
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
const SM = t.core();
const FPS = 30;
```

In `scripts/storymaker-tests/31-checks.mjs`, replace:

```js
    assert.deepEqual(plain(SM.checks.flashes([], FPS)), []);
    assert.deepEqual(plain(SM.checks.flashes([0.5], FPS)), []);
});
```

with:

```js
    assert.deepEqual(plain(SM.checks.flashes([], FPS)), []);
    assert.deepEqual(plain(SM.checks.flashes([0.5], FPS)), []);
});

// ---- Task C3: SM.checks.run turns the probe into flash and flash_unverified issues.
// film(frames) = a timeline of two scenes, a storyboard scene "a" over frames 0..59 and a code scene "b" over 60..frames-1.
const film = frames => ({
    fps: FPS,
    schedule: {
        fps: FPS, frame_count: frames,
        scenes: [
            { id: 'a', kind: 'storyboard', start_frame: 0, end_frame: 60, tf: 0, transition: 'cut' },
            { id: 'b', kind: 'code', start_frame: 60, end_frame: frames, tf: 0, transition: 'cut' }
        ]
    }
});
// strobe(from, to, level) = a series of `to` frames at `level`, with 0.1 / 0.9 alternating every 2 frames from `from` to `to`.
const strobe = (n, from, to, level) => Array.from({ length: n }, (_, i) => i >= from && i < to ? (Math.floor((i - from) / 2) % 2 ? 0.9 : 0.1) : level);
const probeOf = (y, extra) => ({ fps: FPS, frame_count: 120, y, d: y.map(() => 1), uniform: {}, ...extra });

check('checks.run: no probe, no findings', () => {
    assert.deepEqual(plain(SM.checks.run(null, film(120))), []);
    assert.deepEqual(plain(SM.checks.run(undefined, film(120))), []);
});

check('checks.run: a calm film raises nothing', () => {
    assert.deepEqual(plain(SM.checks.run(probeOf(new Array(120).fill(0.4)), film(120))), []);
});

check('checks.run: a strobing code scene raises one flash issue for that scene at its scene-local window start', () => {
    const list = plain(SM.checks.run(probeOf(strobe(120, 70, 110, 0.4)), film(120)));
    assert.equal(list.length, 1, JSON.stringify(list));
    const f = list[0];
    assert.equal(f.code, 'flash');
    assert.equal(f.severity, 'warning');
    assert.equal(f.scene_id, 'b');
    assert.equal(f.path, '/scenes/1');
    near(f.at_seconds, (69 - 60) / FPS, 1e-9);   // the dip starts on the last bright frame before the first dark one
    assert.match(f.message, /flash/i);
    assert.match(f.fix, /third of a second/);
});

check('checks.run: many flashes in a scene stay one finding, and a second scene gets its own', () => {
    const both = strobe(120, 10, 100, 0.4);
    const list = plain(SM.checks.run(probeOf(both), film(120)));
    assert.deepEqual(list.map(i => i.scene_id), ['a', 'b']);
});

check('checks.run: frames the probe did not reach give one flash_unverified in the scene of the first unreached frame', () => {
    const list = plain(SM.checks.run(probeOf(new Array(90).fill(0.4)), film(120)));
    assert.equal(list.length, 1, JSON.stringify(list));
    assert.equal(list[0].code, 'flash_unverified');
    assert.equal(list[0].severity, 'warning');
    assert.equal(list[0].scene_id, 'b');
    assert.equal(list[0].path, '/scenes/1');
    near(list[0].at_seconds, (90 - 60) / FPS, 1e-9);
    // what was reached is still judged
    const flashing = plain(SM.checks.run(probeOf(strobe(90, 10, 40, 0.4)), film(120)));
    assert.deepEqual(flashing.map(i => i.code).sort(), ['flash', 'flash_unverified']);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL checks.run: a strobing code scene raises one flash issue for that scene at its scene-local window start: AssertionError [ERR_ASSERTION]: []

- [ ] **Step 3: Implement**

In `ui/js/storymaker/checks.js`, replace:

```js
        return out;
    }

    // run(probe, timeline) = the findings of the measured series. Without a probe there is nothing to say.
    function run() { return []; }

    // unverified(scenes) = one minor layout_unverified issue per code scene.
    function unverified(scenes) {
```

with:

```js
        return out;
    }

    // sceneAt(sched, frame) = the index of the scene that owns a film frame: the last scene that has started.
    function sceneAt(sched, frame) {
        var scenes = sched.scenes || [], at = 0;
        for (var i = 0; i < scenes.length; i++) if (scenes[i].start_frame <= frame) at = i; else break;
        return at;
    }

    // run(probe, timeline) = the findings of the measured series. Times in at_seconds are scene-local, like the lint's. The
    // probe is {fps, frame_count, y, d, uniform}: one y and one d per frame it reached, in order; without a probe there is
    // nothing to say.
    function run(probe, timeline) {
        var out = [];
        if (!probe || !timeline || !timeline.schedule) return out;
        var sched = timeline.schedule, scenes = sched.scenes || [], fps = probe.fps > 0 ? probe.fps : timeline.fps;
        var y = probe.y || [];
        function add(index, code, message, fix, frame) {
            out.push(SM.plan.issue('warning', scenes[index].id, '/scenes/' + index, code, message, { at_seconds: (frame - scenes[index].start_frame) / fps, fix: fix }));
        }

        var reported = {};
        flashWindows(flashes(y), fps).forEach(function (frame) {
            var index = sceneAt(sched, frame);
            if (reported[index]) return;
            reported[index] = true;
            add(index, 'flash', 'The picture flashes more than 3 times in one second here.',
                'Slow the change down: keep big brightness jumps at least a third of a second apart and avoid blinking full-screen colours. Rewrite this scene with patch_scene or put_scene, then check again.', frame);
        });

        if (y.length < sched.frame_count && scenes.length) {
            add(sceneAt(sched, y.length), 'flash_unverified', 'Flashing was not checked from here: the check ran out of time.',
                'Nothing to fix; a shorter film or fewer code scenes can be checked completely.', y.length);
        }
        return out;
    }

    // unverified(scenes) = one minor layout_unverified issue per code scene.
    function unverified(scenes) {
```

- [ ] **Step 4: Rebuild the generated files**

Run: `npm run build:ui` (then `npm run check:ui`).

- [ ] **Step 5: Run the test and see it pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: 246 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/31-checks.mjs ui/js/storymaker/checks.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: SM.checks.run reports flash per scene and flash_unverified"
```

### Task C4: code scenes get dead_stretch and first_frame_empty from the probe

**Files:**
- Modify: `scripts/storymaker-tests/31-checks.mjs`
- Modify: `ui/js/storymaker/checks.js`
- Modify: `ui/js/storymaker/host.bundle.js` (generated)
- Modify: `ui/js/storymaker/player.bundle.js` (generated)
- Modify: `ui/js/storymaker/schedule.js`

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/31-checks.mjs`, replace:

```js

// ---- Task C3: SM.checks.run turns the probe into flash and flash_unverified issues.
// film(frames) = a timeline of two scenes, a storyboard scene "a" over frames 0..59 and a code scene "b" over 60..frames-1.
const film = frames => ({
    fps: FPS,
    schedule: {
        fps: FPS, frame_count: frames,
        scenes: [
            { id: 'a', kind: 'storyboard', start_frame: 0, end_frame: 60, tf: 0, transition: 'cut' },
            { id: 'b', kind: 'code', start_frame: 60, end_frame: frames, tf: 0, transition: 'cut' }
        ]
    }
});
// strobe(from, to, level) = a series of `to` frames at `level`, with 0.1 / 0.9 alternating every 2 frames from `from` to `to`.
const strobe = (n, from, to, level) => Array.from({ length: n }, (_, i) => i >= from && i < to ? (Math.floor((i - from) / 2) % 2 ? 0.9 : 0.1) : level);
const probeOf = (y, extra) => ({ fps: FPS, frame_count: 120, y, d: y.map(() => 1), uniform: {}, ...extra });

check('checks.run: no probe, no findings', () => {
    assert.deepEqual(plain(SM.checks.run(null, film(120))), []);
```

with:

```js

// ---- Task C3: SM.checks.run turns the probe into flash and flash_unverified issues.
// film(frames) = a timeline of two scenes, a storyboard scene "a" over frames 0..59 and a code scene "b" over 60..frames-1.
const film = (frames, tf = 0, transition = 'cut') => ({
    fps: FPS,
    schedule: {
        fps: FPS, frame_count: frames,
        scenes: [
            { id: 'a', kind: 'storyboard', start_frame: 0, end_frame: 60, tf: 0, transition: 'cut' },
            { id: 'b', kind: 'code', start_frame: 60, end_frame: frames, tf, transition }
        ]
    }
});
// strobe(from, to, level) = a series of `to` frames at `level`, with 0.1 / 0.9 alternating every 2 frames from `from` to `to`.
const strobe = (n, from, to, level) => Array.from({ length: n }, (_, i) => i >= from && i < to ? (Math.floor((i - from) / 2) % 2 ? 0.9 : 0.1) : level);
const probeOf = (y, extra) => ({ fps: FPS, frame_count: y.length, y, d: y.map(() => 1), uniform: {}, ...extra });

check('checks.run: no probe, no findings', () => {
    assert.deepEqual(plain(SM.checks.run(null, film(120))), []);
```

In `scripts/storymaker-tests/31-checks.mjs`, replace:

```js
    const flashing = plain(SM.checks.run(probeOf(strobe(90, 10, 40, 0.4)), film(120)));
    assert.deepEqual(flashing.map(i => i.code).sort(), ['flash', 'flash_unverified']);
});
```

with:

```js
    const flashing = plain(SM.checks.run(probeOf(strobe(90, 10, 40, 0.4)), film(120)));
    assert.deepEqual(flashing.map(i => i.code).sort(), ['flash', 'flash_unverified']);
});

// ---- Task C4: code scenes are judged on every probed frame: dead_stretch (d below 0.01 for more than 3 s) and first_frame_empty.
const calm = n => new Array(n).fill(0.4);
// still(n, from, count) = a d series of n frames, all moving (0.2) except `count` frames from `from` that do not (0).
const still = (n, from, count) => Array.from({ length: n }, (_, i) => i >= from && i < from + count ? 0 : 0.2);

check('checks.run: a code scene that holds one picture for more than 3 s is a dead_stretch at its scene-local start', () => {
    const list = plain(SM.checks.run(probeOf(calm(260), { d: still(260, 80, 91) }), film(260)));
    assert.equal(list.length, 1, JSON.stringify(list));
    const f = list[0];
    assert.equal(f.code, 'dead_stretch');
    assert.equal(f.severity, 'warning');
    assert.equal(f.scene_id, 'b');
    assert.equal(f.path, '/scenes/1');
    near(f.at_seconds, (80 - 60) / FPS, 1e-9);
    assert.match(f.message, /3\.\d s/);
    assert.ok(f.fix.length > 20);
});

check('checks.run: exactly 3 s of stillness, a still storyboard scene and unreached frames are no dead_stretch', () => {
    assert.deepEqual(plain(SM.checks.run(probeOf(calm(260), { d: still(260, 80, 90) }), film(260))), []);
    // storyboard scenes keep their timeline lint, never the probe: the whole of scene a is still here
    assert.deepEqual(plain(SM.checks.run(probeOf(calm(260), { d: still(260, 0, 59) }), film(260))), []);
    // a stretch that runs into frames the probe never reached is judged on what it measured
    const y = calm(200);
    assert.deepEqual(plain(SM.checks.run({ fps: FPS, frame_count: 260, y, d: still(200, 80, 200), uniform: {} }, film(260)).map(i => i.code)), ['dead_stretch', 'flash_unverified']);
});

check('checks.run: a uniform first frame after the entry transition of a code scene is first_frame_empty, at the end of the transition', () => {
    const list = plain(SM.checks.run(probeOf(calm(260), { uniform: { 1: true } }), film(260, 15, 'fade')));
    assert.equal(list.length, 1, JSON.stringify(list));
    assert.equal(list[0].code, 'first_frame_empty');
    assert.equal(list[0].scene_id, 'b');
    assert.equal(list[0].severity, 'warning');
    near(list[0].at_seconds, 15 / FPS, 1e-9);
    assert.deepEqual(plain(SM.checks.run(probeOf(calm(260), { uniform: { 1: false } }), film(260, 15, 'fade'))), []);
    // a storyboard scene keeps its timeline lint
    assert.deepEqual(plain(SM.checks.run(probeOf(calm(260), { uniform: { 0: true } }), film(260))), []);
});

check('checks.schedule.settledFrame: the first frame after the entry transition, and the start frame without one', () => {
    const sched = film(260, 15, 'fade').schedule;
    assert.equal(SM.schedule.settledFrame(sched, 0), 0);
    assert.equal(SM.schedule.settledFrame(sched, 1), 75);
    assert.equal(SM.schedule.settledFrame(film(260, 15, 'cut').schedule, 1), 60);
    assert.equal(SM.schedule.settledFrame(film(260, 0, 'fade').schedule, 1), 60);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL checks.run: a code scene that holds one picture for more than 3 s is a dead_stretch at its scene-local start: AssertionError [ERR_ASSERTION]: []

- [ ] **Step 3: Implement**

In `ui/js/storymaker/checks.js`, replace:

```js
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var FLASH_STEP = 0.10, FLASH_BRIGHT = 0.80, FLASH_MAX = 3, LEVEL_NOISE = 0.002;

    // transitions(y) = the monotone runs of the luminance series whose total change is at least 0.10 and whose darker end is
    // below 0.80, as {from, to, dir} (dir 1 brightens). A run ends at its first extreme frame and the next one starts on the
```

with:

```js
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var FLASH_STEP = 0.10, FLASH_BRIGHT = 0.80, FLASH_MAX = 3, LEVEL_NOISE = 0.002, DEAD_DIFF = 0.01, DEAD_SECONDS = 3;

    // transitions(y) = the monotone runs of the luminance series whose total change is at least 0.10 and whose darker end is
    // below 0.80, as {from, to, dir} (dir 1 brightens). A run ends at its first extreme frame and the next one starts on the
```

In `ui/js/storymaker/checks.js`, replace:

```js
                'Slow the change down: keep big brightness jumps at least a third of a second apart and avoid blinking full-screen colours. Rewrite this scene with patch_scene or put_scene, then check again.', frame);
        });

        if (y.length < sched.frame_count && scenes.length) {
            add(sceneAt(sched, y.length), 'flash_unverified', 'Flashing was not checked from here: the check ran out of time.',
                'Nothing to fix; a shorter film or fewer code scenes can be checked completely.', y.length);
```

with:

```js
                'Slow the change down: keep big brightness jumps at least a third of a second apart and avoid blinking full-screen colours. Rewrite this scene with patch_scene or put_scene, then check again.', frame);
        });

        var d = probe.d || [], reach = Math.min(y.length, d.length);
        scenes.forEach(function (sc, index) {
            if (sc.kind !== 'code') return;
            // dead_stretch: a run of frames equal to the one before them. The first frame of the scene differs from the last
            // frame of the scene before it, so the run is counted from the second.
            var run = 0, last = Math.min(sc.end_frame, reach);
            function stretch(end) {
                if (run / fps > DEAD_SECONDS) {
                    var secs = (run / fps).toFixed(1);
                    add(index, 'dead_stretch', 'The picture barely changes for ' + secs + ' s in this code scene.',
                        'Add motion in this stretch (a tween, a gesture, a new element) or make the scene shorter. Rewrite this scene with patch_scene or put_scene, then check again.', end - run);
                }
                run = 0;
            }
            for (var n = sc.start_frame + 1; n < last; n++) {
                if (d[n] < DEAD_DIFF) run++; else stretch(n);
            }
            stretch(last);
            var settled = SM.schedule.settledFrame(sched, index);
            if (probe.uniform && probe.uniform[index] === true && settled < reach) {
                add(index, 'first_frame_empty', 'The first frame after the transition is one flat colour.',
                    'Draw something at the very start of this code scene: a background, an actor or a text. Rewrite this scene with patch_scene or put_scene, then check again.', settled);
            }
        });

        if (y.length < sched.frame_count && scenes.length) {
            add(sceneAt(sched, y.length), 'flash_unverified', 'Flashing was not checked from here: the check ran out of time.',
                'Nothing to fix; a shorter film or fewer code scenes can be checked completely.', y.length);
```

In `ui/js/storymaker/schedule.js`, replace:

```js
    function frameToSeconds(fps, n) { return n / fps; }
    function secondsToFrames(fps, s) { return Math.round(s * fps + 1e-9); }

    SM.schedule = { framePlan: framePlan, frameToSeconds: frameToSeconds, secondsToFrames: secondsToFrames };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

with:

```js
    function frameToSeconds(fps, n) { return n / fps; }
    function secondsToFrames(fps, s) { return Math.round(s * fps + 1e-9); }

    // settledFrame(sched, i) = the first film frame of scene i after its entry transition (its start frame when it has none).
    function settledFrame(sched, i) {
        var sc = sched.scenes[i];
        return sc.start_frame + (i > 0 && sc.tf > 0 && sc.transition && sc.transition !== 'cut' ? sc.tf : 0);
    }

    SM.schedule = { framePlan: framePlan, settledFrame: settledFrame, frameToSeconds: frameToSeconds, secondsToFrames: secondsToFrames };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

- [ ] **Step 4: Rebuild the generated files**

Run: `npm run build:ui` (then `npm run check:ui`).

- [ ] **Step 5: Run the test and see it pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: 250 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/31-checks.mjs ui/js/storymaker/checks.js ui/js/storymaker/host.bundle.js ui/js/storymaker/player.bundle.js ui/js/storymaker/schedule.js
git commit -m "storymaker: code scenes get dead_stretch and first_frame_empty from the probe"
```

### Task C5: per-frame luminance, difference and uniformity of the probe

**Files:**
- Modify: `scripts/storymaker-tests/31-checks.mjs`
- Modify: `ui/js/storymaker/host-render.js`
- Modify: `ui/js/storymaker/host.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/31-checks.mjs`, replace:

```js
// Task C2 and following: the measured checks of checks.js, pure functions on the series the frame probe records.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
```

with:

```js
// Task C2 and following: the measured checks of checks.js, pure functions on the series the frame probe records.
import assert from 'node:assert/strict';
import { plain, loadHost } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
```

In `scripts/storymaker-tests/31-checks.mjs`, replace:

```js
    assert.equal(SM.schedule.settledFrame(film(260, 15, 'cut').schedule, 1), 60);
    assert.equal(SM.schedule.settledFrame(film(260, 0, 'fade').schedule, 1), 60);
});
```

with:

```js
    assert.equal(SM.schedule.settledFrame(film(260, 15, 'cut').schedule, 1), 60);
    assert.equal(SM.schedule.settledFrame(film(260, 0, 'fade').schedule, 1), 60);
});

// ---- Task C5: the per-frame numbers of the probe: mean luminance y, difference d to the previous frame, uniform.
const H = loadHost().StorymakerHost;
// frame(r, g, b, n) = n RGBA pixels of one colour
const frame = (r, g, b, n = 64 * 36) => {
    const px = new Uint8ClampedArray(n * 4);
    for (let i = 0; i < n; i++) px.set([r, g, b, 255], i * 4);
    return px;
};

check('host frameStats: y is the mean relative luminance of the linearised sRGB channels', () => {
    near(H.frameStats(frame(0, 0, 0), null).y, 0, 1e-12);
    near(H.frameStats(frame(255, 255, 255), null).y, 1, 1e-9);
    near(H.frameStats(frame(128, 128, 128), null).y, 0.2159, 1e-3);
    near(H.frameStats(frame(255, 0, 0), null).y, 0.2126, 1e-9);
    near(H.frameStats(frame(0, 255, 0), null).y, 0.7152, 1e-9);
    // half black, half white
    const half = frame(0, 0, 0);
    for (let i = 0; i < half.length / 8; i++) half.set([255, 255, 255, 255], i * 4);
    near(H.frameStats(half, null).y, 0.5, 1e-9);
});

check('host frameStats: d is the mean absolute difference per colour channel to the previous frame, 0 for the first', () => {
    assert.equal(H.frameStats(frame(10, 20, 30), null).d, 0);
    assert.equal(H.frameStats(frame(10, 20, 30), frame(10, 20, 30)).d, 0);
    near(H.frameStats(frame(255, 255, 255), frame(0, 0, 0)).d, 1, 1e-12);
    near(H.frameStats(frame(51, 0, 0), frame(0, 0, 0)).d, 0.2 / 3, 1e-12);
    // the alpha channel is not a colour channel
    const clear = frame(0, 0, 0);
    for (let i = 0; i < clear.length; i += 4) clear[i + 3] = 0;
    assert.equal(H.frameStats(frame(0, 0, 0), clear).d, 0);
});

check('host frameStats: uniform means every channel of every pixel is within 1 % of full scale of the frame mean', () => {
    assert.equal(H.frameStats(frame(40, 90, 200), null).uniform, true);
    const off = frame(100, 100, 100);
    off.set([103, 100, 100, 255], 40);   // 3/255 from the mean is over 1 %
    assert.equal(H.frameStats(off, null).uniform, false);
    const near1 = frame(100, 100, 100);
    near1.set([102, 100, 100, 255], 40);   // 2/255 is under 1 %
    assert.equal(H.frameStats(near1, null).uniform, true);
    // a two-tone frame is not uniform
    const split = frame(0, 0, 0);
    for (let i = 0; i < split.length / 8; i++) split.set([255, 255, 255, 255], i * 4);
    assert.equal(H.frameStats(split, null).uniform, false);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL host frameStats: y is the mean relative luminance of the linearised sRGB channels: TypeError: H.frameStats is not a function

- [ ] **Step 3: Implement**

In `ui/js/storymaker/host-render.js`, replace:

```js
        return out;
    }

    // mountRender(opts): opts = {canvas, base, fetch?, window?, env?}. Resolves when the job is over (done, stopped,
    // refused, failed) or, in screenshot mode, when the page is ready for __smRenderFrame calls.
    async function mountRender(opts) {
```

with:

```js
        return out;
    }

    // sRGB channel (0..255) to linear light, built once.
    var LINEAR = new Float64Array(256);
    for (var li = 0; li < 256; li++) {
        var cs = li / 255;
        LINEAR[li] = cs <= 0.04045 ? cs / 12.92 : Math.pow((cs + 0.055) / 1.055, 2.4);
    }
    var UNIFORM_SPREAD = 0.01 * 255;   // every channel within 1 % of full scale of the frame mean

    // frameStats(px, prev) = {y, d, uniform} of one probed frame: px and prev are RGBA bytes of the same size (prev may be
    // null). y is the mean relative luminance (0.2126 R + 0.7152 G + 0.0722 B of the linearised channels), d the mean absolute
    // difference per colour channel to prev (0..1, 0 without prev), uniform whether every pixel is within 1 % of the mean.
    function frameStats(px, prev) {
        var n = px.length >> 2, i, r = 0, g = 0, b = 0, lum = 0, diff = 0;
        for (i = 0; i < px.length; i += 4) {
            r += px[i]; g += px[i + 1]; b += px[i + 2];
            lum += 0.2126 * LINEAR[px[i]] + 0.7152 * LINEAR[px[i + 1]] + 0.0722 * LINEAR[px[i + 2]];
            if (prev) diff += Math.abs(px[i] - prev[i]) + Math.abs(px[i + 1] - prev[i + 1]) + Math.abs(px[i + 2] - prev[i + 2]);
        }
        var mr = r / n, mg = g / n, mb = b / n, uniform = true;
        for (i = 0; i < px.length && uniform; i += 4) {
            if (Math.abs(px[i] - mr) > UNIFORM_SPREAD || Math.abs(px[i + 1] - mg) > UNIFORM_SPREAD || Math.abs(px[i + 2] - mb) > UNIFORM_SPREAD) uniform = false;
        }
        return { y: n ? lum / n : 0, d: prev && n ? diff / (n * 3 * 255) : 0, uniform: uniform };
    }

    // mountRender(opts): opts = {canvas, base, fetch?, window?, env?}. Resolves when the job is over (done, stopped,
    // refused, failed) or, in screenshot mode, when the page is ready for __smRenderFrame calls.
    async function mountRender(opts) {
```

In `ui/js/storymaker/host-render.js`, replace:

```js
    }

    H.mountRender = mountRender;
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

with:

```js
    }

    H.mountRender = mountRender;
    H.frameStats = frameStats;
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

- [ ] **Step 4: Rebuild the generated files**

Run: `npm run build:ui` (then `npm run check:ui`).

- [ ] **Step 5: Run the test and see it pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: 253 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/31-checks.mjs ui/js/storymaker/host-render.js ui/js/storymaker/host.bundle.js
git commit -m "storymaker: per-frame luminance, difference and uniformity of the probe"
```

### Task C6: the probe loop renders every frame at 64 px until its deadline

**Files:**
- Modify: `scripts/storymaker-tests/31-checks.mjs`
- Modify: `ui/js/storymaker/host-render.js`
- Modify: `ui/js/storymaker/host.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/31-checks.mjs`, replace:

```js
    for (let i = 0; i < split.length / 8; i++) split.set([255, 255, 255, 255], i * 4);
    assert.equal(H.frameStats(split, null).uniform, false);
});
```

with:

```js
    for (let i = 0; i < split.length / 8; i++) split.set([255, 255, 255, 255], i * 4);
    assert.equal(H.frameStats(split, null).uniform, false);
});

// ---- Task C6: the probe loop renders every frame at 64 px width in film order, until the deadline.
// A fake engine: frame n is the colour colours[n] as a 64 x 36 bitmap; the fake canvas reads that bitmap back.
function probeRig(colours, over = {}) {
    const closed = [], widths = [];
    const engine = {
        render: async (n, opts) => {
            widths.push(opts.width);
            if (over.failAt === n) throw new Error('frame ' + n + ' failed');
            const [r, g, b] = colours[n];
            return { pixels: frame(r, g, b), close: () => closed.push(n) };
        }
    };
    const makeCanvas = (w, h) => {
        let last = null;
        return { width: w, height: h, getContext: () => ({ clearRect() {}, drawImage: bmp => { last = bmp; }, getImageData: () => ({ data: last.pixels }) }) };
    };
    let clock = 0;
    const now = () => { const v = clock; clock += over.step || 0; return v; };
    const schedule = {
        fps: 30, frame_count: colours.length,
        scenes: [
            { id: 'a', kind: 'storyboard', start_frame: 0, end_frame: 3, tf: 0, transition: 'cut' },
            { id: 'b', kind: 'code', start_frame: 3, end_frame: colours.length, tf: 1, transition: 'fade' }
        ]
    };
    return { closed, widths, opts: { engine, schedule, width: 1920, height: 1080, makeCanvas, now, deadlineMs: over.deadlineMs || 20000 } };
}
const SIX = [[0, 0, 0], [0, 0, 0], [255, 255, 255], [90, 90, 90], [90, 90, 90], [255, 0, 0]];

check('host runProbe: every frame at 64 px width in film order, y and d per frame, uniform at the settled frame of a code scene', async () => {
    const rig = probeRig(SIX);
    const probe = plain(await H.runProbe(rig.opts));
    assert.equal(probe.fps, 30);
    assert.equal(probe.frame_count, 6);
    assert.deepEqual(rig.widths, [64, 64, 64, 64, 64, 64]);
    assert.deepEqual(rig.closed, [0, 1, 2, 3, 4, 5]);
    assert.equal(probe.y.length, 6);
    assert.equal(probe.d.length, 6);
    near(probe.y[0], 0, 1e-12); near(probe.y[2], 1, 1e-9); near(probe.y[5], 0.2126, 1e-9);
    assert.equal(probe.d[0], 0);
    near(probe.d[2], 1, 1e-12);
    assert.equal(probe.d[4], 0);
    // scene b (index 1) starts at frame 3 and its fade lasts 1 frame, so frame 4 is the first one after it: flat grey
    assert.deepEqual(probe.uniform, { 1: true });
});

check('host runProbe: it stops at the deadline and reports only the frames it reached', async () => {
    const rig = probeRig(SIX, { step: 100, deadlineMs: 250 });
    const probe = plain(await H.runProbe(rig.opts));
    assert.equal(probe.y.length, 2);
    assert.equal(probe.d.length, 2);
    assert.equal(probe.frame_count, 6);
    assert.deepEqual(probe.uniform, {});
});

check('host runProbe: a frame that fails ends the probe there instead of failing the check', async () => {
    const rig = probeRig(SIX, { failAt: 4 });
    const probe = plain(await H.runProbe(rig.opts));
    assert.equal(probe.y.length, 4);
    assert.deepEqual(probe.uniform, {});
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL host runProbe: every frame at 64 px width in film order, y and d per frame, uniform at the settled frame of a code scene: TypeError: H.runProbe is not a function

- [ ] **Step 3: Implement**

In `ui/js/storymaker/host-render.js`, replace:

```js
        var cs = li / 255;
        LINEAR[li] = cs <= 0.04045 ? cs / 12.92 : Math.pow((cs + 0.055) / 1.055, 2.4);
    }
    var UNIFORM_SPREAD = 0.01 * 255;   // every channel within 1 % of full scale of the frame mean

    // frameStats(px, prev) = {y, d, uniform} of one probed frame: px and prev are RGBA bytes of the same size (prev may be
```

with:

```js
        var cs = li / 255;
        LINEAR[li] = cs <= 0.04045 ? cs / 12.92 : Math.pow((cs + 0.055) / 1.055, 2.4);
    }
    var PROBE_WIDTH = 64, PROBE_DEADLINE_MS = 20000;
    var UNIFORM_SPREAD = 0.01 * 255;   // every channel within 1 % of full scale of the frame mean

    // frameStats(px, prev) = {y, d, uniform} of one probed frame: px and prev are RGBA bytes of the same size (prev may be
```

In `ui/js/storymaker/host-render.js`, replace:

```js
        return { y: n ? lum / n : 0, d: prev && n ? diff / (n * 3 * 255) : 0, uniform: uniform };
    }

    // mountRender(opts): opts = {canvas, base, fetch?, window?, env?}. Resolves when the job is over (done, stopped,
    // refused, failed) or, in screenshot mode, when the page is ready for __smRenderFrame calls.
    async function mountRender(opts) {
```

with:

```js
        return { y: n ? lum / n : 0, d: prev && n ? diff / (n * 3 * 255) : 0, uniform: uniform };
    }

    // runProbe(o) = the frame probe of the compile check: o = {engine, schedule, width, height, makeCanvas(w, h), now(), deadlineMs}.
    // Every frame of the film is rendered in order at PROBE_WIDTH and reduced to y and d (frameStats), until the deadline or
    // the first frame that fails; the frames it did not reach are simply missing from y and d, and checks.js reports that.
    // uniform[i] holds the flat-frame test of the first frame after the entry transition of code scene i, when it was reached.
    async function runProbe(o) {
        var sched = o.schedule, total = sched.frame_count, pw = PROBE_WIDTH, ph = Math.max(1, Math.round(pw * o.height / o.width));
        var canvas = o.makeCanvas(pw, ph), cx = canvas.getContext('2d', { willReadFrequently: true });
        var settled = {}, y = [], d = [], uniform = {}, prev = null, started = o.now();
        (sched.scenes || []).forEach(function (sc, i) {
            if (sc.kind === 'code') settled[root.StorymakerCore.schedule.settledFrame(sched, i)] = i;
        });
        for (var n = 0; n < total; n++) {
            if (o.now() - started >= o.deadlineMs) break;
            try {
                var bmp = await o.engine.render(n, { width: pw });
                cx.clearRect(0, 0, pw, ph);
                cx.drawImage(bmp, 0, 0, pw, ph);
                if (typeof bmp.close === 'function') bmp.close();
            } catch (e) { break; }
            var px = cx.getImageData(0, 0, pw, ph).data, st = frameStats(px, prev);
            y.push(st.y);
            d.push(st.d);
            if (settled[n] !== undefined) uniform[settled[n]] = st.uniform;
            prev = px;
        }
        return { fps: sched.fps, frame_count: total, y: y, d: d, uniform: uniform };
    }

    // mountRender(opts): opts = {canvas, base, fetch?, window?, env?}. Resolves when the job is over (done, stopped,
    // refused, failed) or, in screenshot mode, when the page is ready for __smRenderFrame calls.
    async function mountRender(opts) {
```

In `ui/js/storymaker/host-render.js`, replace:

```js

    H.mountRender = mountRender;
    H.frameStats = frameStats;
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

with:

```js

    H.mountRender = mountRender;
    H.frameStats = frameStats;
    H.runProbe = runProbe;
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

- [ ] **Step 4: Rebuild the generated files**

Run: `npm run build:ui` (then `npm run check:ui`).

- [ ] **Step 5: Run the test and see it pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: 256 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/31-checks.mjs ui/js/storymaker/host-render.js ui/js/storymaker/host.bundle.js
git commit -m "storymaker: the probe loop renders every frame at 64 px until its deadline"
```

### Task C7: the compile page probes the film and puts the findings into the report

**Files:**
- Modify: `scripts/storymaker-tests/31-checks.mjs`
- Modify: `ui/js/storymaker/compile.js`
- Modify: `ui/js/storymaker/host-render.js`
- Modify: `ui/js/storymaker/host.bundle.js` (generated)
- Modify: `ui/js/storymaker/player.bundle.js` (generated)
- Modify: `ui/js/storymaker/worker-player.js`

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/31-checks.mjs`, replace:

```js
    assert.equal(probe.y.length, 4);
    assert.deepEqual(probe.uniform, {});
});
```

with:

```js
    assert.equal(probe.y.length, 4);
    assert.deepEqual(probe.uniform, {});
});

// ---- Task C7: the player worker answers the host's probe with the findings of checks.run on its own timeline.
check('worker player: the checks message runs the probe through checks.run on the compiled timeline', async () => {
    const player = new SM.workerPlayer.Runtime({});
    await assert.rejects(player.handle({ type: 'checks', probe: probeOf(calm(120)) }), { code: 'code_error', message: /before compile/ });
    player.timeline = film(120);
    const calmAnswer = await player.handle({ type: 'checks', probe: probeOf(calm(120)) });
    assert.deepEqual(plain(calmAnswer.payload.checks), []);
    const strobing = await player.handle({ type: 'checks', probe: probeOf(strobe(120, 70, 110, 0.4)) });
    assert.deepEqual(plain(strobing.payload.checks.map(i => i.code)), ['flash']);
});

check('compile: the report keeps an empty checks list, because only the probe fills it', () => {
    assert.deepEqual(plain(t.compiled().report.checks), []);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL worker player: the checks message runs the probe through checks.run on the compiled timeline: AssertionError [ERR_ASSERTION]: The input did not match the regular expression /before compile/.

- [ ] **Step 3: Implement**

In `ui/js/storymaker/compile.js`, replace:

```js
            frame_count: frameCount, fps: fps, width: fmt.width, height: fmt.height,
            duration_ms: Math.round(frameCount * 1000 / fps), catalog_version: cat.version,
            scenes: scenes.map(function (s) { return { id: s.id, start_frame: s.start_frame, end_frame: s.end_frame, kind: s.kind, cues: s.narration.cueList }; }),
            audio_events: audioEvents, captions: SM.captions.settle(captionList, filmMs), cue_precision: cuePrecision(scenes), checks: SM.checks.run(scenes), unverified: SM.checks.unverified(scenes), issues: issues
        };
        var timeline = {
            version: 1, fps: fps, width: fmt.width, height: fmt.height, aspect: fmt.aspect, frame_count: frameCount,
```

with:

```js
            frame_count: frameCount, fps: fps, width: fmt.width, height: fmt.height,
            duration_ms: Math.round(frameCount * 1000 / fps), catalog_version: cat.version,
            scenes: scenes.map(function (s) { return { id: s.id, start_frame: s.start_frame, end_frame: s.end_frame, kind: s.kind, cues: s.narration.cueList }; }),
            audio_events: audioEvents, captions: SM.captions.settle(captionList, filmMs), cue_precision: cuePrecision(scenes), checks: SM.checks.run(null, null), unverified: SM.checks.unverified(scenes), issues: issues
        };
        var timeline = {
            version: 1, fps: fps, width: fmt.width, height: fmt.height, aspect: fmt.aspect, frame_count: frameCount,
```

In `ui/js/storymaker/host-render.js`, replace:

```js
                    rep.layout_error = String(layoutErr && layoutErr.message || layoutErr).slice(0, 200);
                }
            }
            // Parallel path: only PNG is encoded in the worker, and a main engine in the document-realm fallback stays serial.
            // The helpers start before the report, so their start-up cannot trip the 10 s frame watchdog of Go.
            var parallel = boot.transport === 'png' && boot.workers > 1 && boot.chunk > 0 && engine.mode === 'workers';
```

with:

```js
                    rep.layout_error = String(layoutErr && layoutErr.message || layoutErr).slice(0, 200);
                }
            }
            if (boot.layout === true) {
                // The compile check also probes every frame for flashing and frozen code scenes (checks.js). Like the layout,
                // a failure here costs the findings, never the report.
                try {
                    var probe = await runProbe({
                        engine: engine, schedule: engine.schedule, width: boot.width, height: boot.height, deadlineMs: PROBE_DEADLINE_MS,
                        makeCanvas: env.makeCanvas, now: env.now
                    });
                    var judged = await engine.send({ type: 'checks', probe: probe }, 15000);
                    rep.checks = (rep.checks || []).concat(judged.checks || []);
                } catch (probeErr) {
                    rep.probe_error = String(probeErr && probeErr.message || probeErr).slice(0, 200);
                }
            }
            // Parallel path: only PNG is encoded in the worker, and a main engine in the document-realm fallback stays serial.
            // The helpers start before the report, so their start-up cannot trip the 10 s frame watchdog of Go.
            var parallel = boot.transport === 'png' && boot.workers > 1 && boot.chunk > 0 && engine.mode === 'workers';
```

In `ui/js/storymaker/worker-player.js`, replace:

```js
// Storymaker player: the player runtime (trusted, our code only). It holds the compiled timeline and the loaded assets
// and answers the host's messages: plan (timing facts for code scenes), compile (timeline + compile report), assets
// (catalog JSON and fonts, the readiness barrier) and render (one frame as an ImageBitmap). The same Runtime serves the
// player worker and, when workers are unavailable, the host realm (storyboard fallback). Classic script; publishes
// StorymakerCore.workerPlayer.
```

with:

```js
// Storymaker player: the player runtime (trusted, our code only). It holds the compiled timeline and the loaded assets
// and answers the host's messages: plan (timing facts for code scenes), compile (timeline + compile report), checks (the findings of the frame probe), assets
// (catalog JSON and fonts, the readiness barrier) and render (one frame as an ImageBitmap). The same Runtime serves the
// player worker and, when workers are unavailable, the host realm (storyboard fallback). Classic script; publishes
// StorymakerCore.workerPlayer.
```

In `ui/js/storymaker/worker-player.js`, replace:

```js
                };
                return { payload: { layout: SM.layout.measure(ltl, lassets) } };
            }
            case 'dispose':
                this.disposeGraphics();
                this.timeline = null;
```

with:

```js
                };
                return { payload: { layout: SM.layout.measure(ltl, lassets) } };
            }
            case 'checks': {
                if (!this.timeline) throw fail('code_error', 'checks before compile');
                return { payload: { checks: SM.checks.run(msg.probe, this.timeline) } };
            }
            case 'dispose':
                this.disposeGraphics();
                this.timeline = null;
```

- [ ] **Step 4: Rebuild the generated files**

Run: `npm run build:ui` (then `npm run check:ui`).

- [ ] **Step 5: Run the test and see it pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: 258 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/31-checks.mjs ui/js/storymaker/compile.js ui/js/storymaker/host-render.js ui/js/storymaker/host.bundle.js ui/js/storymaker/player.bundle.js ui/js/storymaker/worker-player.js
git commit -m "storymaker: the compile page probes the film and puts the findings into the report"
```

### Task C8: the compile check keeps the page's probe findings as issues

**Files:**
- Modify: `internal/storymaker/render/compilecheck.go`
- Modify: `internal/storymaker/render/compilecheck_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/render/compilecheck_test.go`, replace:

```go
		t.Errorf("a compile check published %v", fx.pub.names())
	}
}
```

with:

```go
		t.Errorf("a compile check published %v", fx.pub.names())
	}
}

func TestCompileCheckKeepsTheProbeFindings(t *testing.T) {
	page := &fakePage{crashed: make(chan struct{})}
	page.script = play(t, playOptions{frames: 60, mutateRep: func(rep map[string]any) {
		rep["issues"] = []any{map[string]any{"severity": "warning", "scene_id": "s1", "path": "/scenes/0", "code": "dead_stretch", "message": "nothing moves"}}
		rep["checks"] = []any{
			map[string]any{"severity": "warning", "scene_id": "s1", "path": "/scenes/0", "code": "flash", "message": "flashes", "at_seconds": 0.3, "fix": "slow it down"},
			map[string]any{"severity": "warning", "scene_id": "s1", "path": "/scenes/0", "code": "flash_unverified", "message": "not checked"},
			// the page is trusted for these four codes only, and only as warnings
			map[string]any{"severity": "error", "scene_id": "s1", "path": "/scenes/0", "code": "flash", "message": "an error"},
			map[string]any{"severity": "warning", "scene_id": "s1", "path": "/scenes/0", "code": "compile_failed", "message": "not a probe code"},
			map[string]any{"severity": "warning", "scene_id": "elsewhere", "path": "/scenes/7", "code": "first_frame_empty", "message": "unknown scene"},
		}
	}})
	runner, id := compileCheckRunner(t, page)
	got, err := runner.CompileCheck(context.Background(), id, Settings{Limits: storymaker.DefaultLimits()})
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, is := range got.Issues {
		codes = append(codes, is.Code)
		if is.Code == storymaker.CodeFlash && is.Severity != storymaker.SeverityWarning {
			t.Errorf("a probe finding is a warning: %+v", is)
		}
		if is.Message == "flashes" && (is.Severity != storymaker.SeverityWarning || is.SceneID != "s1" || is.Fix != "slow it down" || is.AtSeconds == nil) {
			t.Errorf("flash = %+v", is)
		}
		if is.Code == storymaker.CodeFirstFrameEmpty && is.SceneID != "" {
			t.Errorf("a scene the story does not have must be dropped: %+v", is)
		}
	}
	want := []string{storymaker.CodeDeadStretch, storymaker.CodeFlash, storymaker.CodeFlashUnverified, storymaker.CodeFlash, storymaker.CodeFirstFrameEmpty}
	if !reflect.DeepEqual(codes, want) {
		t.Errorf("issue codes = %v, want %v", codes, want)
	}
	if storymaker.HasErrors(got.Issues) {
		t.Errorf("a probe finding must never be an error: %+v", got.Issues)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/render -run '^TestCompileCheckKeepsTheProbeFindings$' -count=1`
Expected: compilecheck_test.go:172: issue codes = [dead_stretch], want [dead_stretch flash flash_unverified flash first_frame_empty]

- [ ] **Step 3: Implement**

In `internal/storymaker/render/compilecheck.go`, replace:

```go
	FrameCount  int                      `json:"frame_count"`
	Scenes      []compileCheckScene      `json:"scenes"`
	Issues      []storymaker.Issue       `json:"issues"`
	Layout      []storymaker.LayoutScene `json:"layout"`
	LayoutError string                   `json:"layout_error"`
}
```

with:

```go
	FrameCount  int                      `json:"frame_count"`
	Scenes      []compileCheckScene      `json:"scenes"`
	Issues      []storymaker.Issue       `json:"issues"`
	Checks      []storymaker.Issue       `json:"checks"`
	Layout      []storymaker.LayoutScene `json:"layout"`
	LayoutError string                   `json:"layout_error"`
}
```

In `internal/storymaker/render/compilecheck.go`, replace:

```go
			Fix:     "Shorten the narration or remove a scene.",
		})
	}
	scenes := make([]storymaker.CompileScene, 0, len(rep.Scenes))
	storyboard, code := 0, 0
	for _, sc := range rep.Scenes {
```

with:

```go
			Fix:     "Shorten the narration or remove a scene.",
		})
	}
	issues = storymaker.CapIssues(append(issues, cleanProbeFindings(rep.Checks, want)...), storymaker.MaxHTTPIssues)
	scenes := make([]storymaker.CompileScene, 0, len(rep.Scenes))
	storyboard, code := 0, 0
	for _, sc := range rep.Scenes {
```

In `internal/storymaker/render/compilecheck.go`, replace:

```go
	return block
}

// cleanCompileIssues removes the render secrets from the page's text and bounds it. An error issue becomes compile_failed
// and keeps its own code in the message.
func cleanCompileIssues(list []storymaker.Issue, want reportExpect) []storymaker.Issue {
```

with:

```go
	return block
}

// probeCodes are the codes the frame probe of the page may report. The page is trusted for measurements only: any other
// code is dropped, and a probe finding is always a warning, never an error that fails the check.
var probeCodes = []string{storymaker.CodeFlash, storymaker.CodeFlashUnverified, storymaker.CodeDeadStretch, storymaker.CodeFirstFrameEmpty}

// cleanProbeFindings keeps the page's probe findings that have a probe code, as warnings, cleaned like the other issues.
func cleanProbeFindings(list []storymaker.Issue, want reportExpect) []storymaker.Issue {
	kept := make([]storymaker.Issue, 0, len(list))
	for _, is := range list {
		if !slices.Contains(probeCodes, is.Code) {
			continue
		}
		is.Severity = storymaker.SeverityWarning
		kept = append(kept, is)
	}
	return cleanCompileIssues(kept, want)
}

// cleanCompileIssues removes the render secrets from the page's text and bounds it. An error issue becomes compile_failed
// and keeps its own code in the message.
func cleanCompileIssues(list []storymaker.Issue, want reportExpect) []storymaker.Issue {
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/render -run '^TestCompileCheckKeepsTheProbeFindings$' -count=1`
Expected: ok  	aurago/internal/storymaker/render	0.705s (TestCompileCheckKeepsTheProbeFindings and TestCompileCheckStopsAfterReport PASS)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/compilecheck.go internal/storymaker/render/compilecheck_test.go
git commit -m "storymaker: the compile check keeps the page's probe findings as issues"
```

### Task C9: slow_hook for a silent or slow-opening first scene

**Files:**
- Modify: `scripts/storymaker-tests/11-real-catalog.mjs`
- Modify: `scripts/storymaker-tests/14-lint.mjs`
- Modify: `ui/js/storymaker/compile.js`
- Modify: `ui/js/storymaker/lint.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/11-real-catalog.mjs`, replace:

```js

check('real catalog: firewall, minimal and the cast sheet compile with only the issues of a silent film and the lint they earn', () => {
    // firewall s2 fades into another environment, minimal shows nothing on its first frame, the cast sheet: see below
    const allowed = new Set(['narration_missing', 'not_supported_yet', 'layout_unverified', 'fade_between_worlds', 'first_frame_empty', 'line_unread', 'entry_monotony']);
    for (const [name, s] of [['firewall', firewall], ['minimal', story('minimal')], ['castsheet', castsheet]]) {
        const out = compileReal(s), report = out.report, codes = [...report.issues, ...report.unverified].map(i => i.code);
        assert.deepEqual(codes.filter(c => !allowed.has(c)), [], name + ': ' + JSON.stringify(report.issues.filter(i => !allowed.has(i.code))));
        assert.equal(report.catalog_version, catalog.version);
        assert.ok(out.needs.includes('catalog:rig/human.json') || name === 'minimal', name + ' needs the rig');
    }
    // A 4-word label holds 1.7 s in the sheet's 1.2 s scenes, and every show has entry none, so only these two codes fire.
    assert.deepEqual([...new Set(plain(compileReal(castsheet).report.issues).map(i => i.code))].sort(), ['entry_monotony', 'line_unread']);
    const ranges = compiled.report.scenes;
    assert.deepEqual(plain(ranges.map(s => s.id)), ['s1', 's2', 's3']);
    assert.ok(ranges.every((s, i) => s.end_frame > s.start_frame && (i === 0 || s.start_frame < ranges[i - 1].end_frame)), 'scenes follow each other, transitions overlap');
```

with:

```js

check('real catalog: firewall, minimal and the cast sheet compile with only the issues of a silent film and the lint they earn', () => {
    // firewall s2 fades into another environment, minimal shows nothing on its first frame, the cast sheet: see below
    const allowed = new Set(['narration_missing', 'not_supported_yet', 'layout_unverified', 'fade_between_worlds', 'first_frame_empty', 'line_unread', 'entry_monotony', 'slow_hook']);
    for (const [name, s] of [['firewall', firewall], ['minimal', story('minimal')], ['castsheet', castsheet]]) {
        const out = compileReal(s), report = out.report, codes = [...report.issues, ...report.unverified].map(i => i.code);
        assert.deepEqual(codes.filter(c => !allowed.has(c)), [], name + ': ' + JSON.stringify(report.issues.filter(i => !allowed.has(i.code))));
        assert.equal(report.catalog_version, catalog.version);
        assert.ok(out.needs.includes('catalog:rig/human.json') || name === 'minimal', name + ' needs the rig');
    }
    // A 4-word label holds 1.7 s in the sheet's 1.2 s scenes, and every show has entry none, so only these two codes fire, and
    // slow_hook because the sheet's first scene has no narration.
    assert.deepEqual([...new Set(plain(compileReal(castsheet).report.issues).map(i => i.code))].sort(), ['entry_monotony', 'line_unread', 'slow_hook']);
    const ranges = compiled.report.scenes;
    assert.deepEqual(plain(ranges.map(s => s.id)), ['s1', 's2', 's3']);
    assert.ok(ranges.every((s, i) => s.end_frame > s.start_frame && (i === 0 || s.start_frame < ranges[i - 1].end_frame)), 'scenes follow each other, transitions overlap');
```

In `scripts/storymaker-tests/14-lint.mjs`, replace:

```js
    // control: a 6 s wait has no narration, no dialogue and no beat in progress
    s11Assert.ok(s11Dead({ scenes: [{ id: 'a', beats: [{ do: 'wait', seconds: 6 }] }] }, {}).length > 0, 'a wait still counts as dead');
});
```

with:

```js
    // control: a 6 s wait has no narration, no dialogue and no beat in progress
    s11Assert.ok(s11Dead({ scenes: [{ id: 'a', beats: [{ do: 'wait', seconds: 6 }] }] }, {}).length > 0, 'a wait still counts as dead');
});

// ---- Task C9: slow_hook, a film lint on scene 1: no narration, or the first show beat lands after 0.5 s. The other lint
// fixtures above stay free of it, because LINT does not name it.
const hookOf = (story, cat) => plain(SM.compile(story, {}, cat || catalog).report.issues).filter(i => i.code === 'slow_hook');
check('lint: slow_hook fires on a silent first scene, once, at the film start', () => {
    const list = hookOf({ cast, scenes: [{ id: 'a', actors, beats: [{ do: 'show', text: 'Hallo', entry: 'none' }] }, { id: 'b', actors, beats: [{ do: 'show', text: 'Welt', entry: 'none' }] }] });
    const hook = one(list, 'slow_hook');
    assert.equal(hook.severity, 'warning');
    assert.equal(hook.scene_id, 'a');
    assert.equal(hook.path, '/scenes/0');
    near(hook.at_seconds, 0, 1e-9);
    assert.ok(hook.fix && hook.fix.length > 20);
});

check('lint: slow_hook fires when the first show beat of a narrated first scene lands after 0.5 s, at its landing', () => {
    const late = one(hookOf({ cast, scenes: [{ id: 'a', actors, narration: 'Hallo Welt, sch�n dass du da bist.', beats: [{ do: 'show', text: 'Hallo', entry: 'none', delay: 1 }] }] }), 'slow_hook');
    assert.equal(late.path, '/scenes/0');
    near(late.at_seconds, 1, 1e-6);
    // the first show beat counts, so a later one does not rescue a slow opening
    const rescued = hookOf({ cast, scenes: [{ id: 'a', actors, narration: 'Hallo Welt, sch�n dass du da bist.', beats: [{ do: 'show', text: 'Hallo', entry: 'none', delay: 1 }, { do: 'show', text: 'Welt', entry: 'none', delay: 0 }] }] });
    assert.equal(rescued.length, 1);
});

check('lint: slow_hook stays quiet for a narrated first scene that shows something at once, or shows nothing, and for later scenes', () => {
    const quick = { id: 'a', actors, narration: 'Hallo Welt, sch�n dass du da bist.', beats: [{ do: 'show', text: 'Hallo', entry: 'none' }] };
    assert.deepEqual(hookOf({ cast, scenes: [quick] }), []);
    assert.deepEqual(hookOf({ cast, scenes: [{ id: 'a', actors, narration: 'Hallo Welt, sch�n dass du da bist.' }] }), []);
    assert.deepEqual(hookOf({ cast, scenes: [quick, { id: 'b', actors, beats: [{ do: 'show', text: 'Welt', entry: 'none', delay: 2 }] }] }), []);
});

check('lint: slow_hook on a first code scene looks at its narration only', () => {
    const code = narration => ({ cast, scenes: [{ id: 'c', kind: 'code', ...(narration ? { narration } : {}) }] });
    const compiled = story => plain(SM.compile(story, {}, catalog, { codeScenes: { c: { duration: 5 } } }).report.issues).filter(i => i.code === 'slow_hook');
    assert.equal(compiled(code('')).length, 1);
    assert.equal(compiled(code('Hallo Welt, sch�n dass du da bist.')).length, 0);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL lint: slow_hook fires on a silent first scene, once, at the film start: AssertionError [ERR_ASSERTION]: slow_hook in []

- [ ] **Step 3: Implement**

In `ui/js/storymaker/compile.js`, replace:

```js
        if (glyphIdx) needGlyphs(String.fromCharCode(0x2026));
        var rhythmFactor = SM.motion.factor(story.style);

        scenesIn.forEach(function (sc, i) {
            var plan = plans[i], isCode = sc.kind === 'code';
            var envId = sc.environment || envPrev || 'env.plain-stage';
```

with:

```js
        if (glyphIdx) needGlyphs(String.fromCharCode(0x2026));
        var rhythmFactor = SM.motion.factor(story.style);

        var firstScene = null;
        scenesIn.forEach(function (sc, i) {
            var plan = plans[i], isCode = sc.kind === 'code';
            var envId = sc.environment || envPrev || 'env.plain-stage';
```

In `ui/js/storymaker/compile.js`, replace:

```js
                code: codeInfo ? { error: codeInfo.error || null, duration: codeInfo.duration || null } : null, stages: {}
            };
            scenes.push(scene);
            issues.push.apply(issues, SM.lint.scene({ scene: scene, index: i, plan: plan, built: built, stage: stage, cat: cat, fmt: fmt, envChanged: envChanged, factor: rhythmFactor }));

            // narration windows (global seconds) and, with measured audio, one audio event per piece
```

with:

```js
                code: codeInfo ? { error: codeInfo.error || null, duration: codeInfo.duration || null } : null, stages: {}
            };
            scenes.push(scene);
            if (i === 0) firstScene = { id: sc.id, narrated: n.words > 0, log: built.log || [] };
            issues.push.apply(issues, SM.lint.scene({ scene: scene, index: i, plan: plan, built: built, stage: stage, cat: cat, fmt: fmt, envChanged: envChanged, factor: rhythmFactor }));

            // narration windows (global seconds) and, with measured audio, one audio event per piece
```

In `ui/js/storymaker/compile.js`, replace:

```js
        });
        if (scenes.some(function (s) { return s.kind === 'code'; }) && catalog && catalog.rig && catalog.rig.file) needs['catalog:' + catalog.rig.file] = true;

        issues.push.apply(issues, SM.lint.film({ scenesIn: scenesIn, plans: plans }));

        var frameCount = 0;
        scenes.forEach(function (s) { if (s.end_frame > frameCount) frameCount = s.end_frame; });
```

with:

```js
        });
        if (scenes.some(function (s) { return s.kind === 'code'; }) && catalog && catalog.rig && catalog.rig.file) needs['catalog:' + catalog.rig.file] = true;

        issues.push.apply(issues, SM.lint.film({ scenesIn: scenesIn, plans: plans, first: firstScene }));

        var frameCount = 0;
        scenes.forEach(function (s) { if (s.end_frame > frameCount) frameCount = s.end_frame; });
```

In `ui/js/storymaker/lint.js`, replace:

```js
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var DEAD_STRETCH_SECONDS = 3, FONT_FLOOR = 0.026, MONOTONY_SHOWS = 3, MAX_TRANSITION_KINDS = 5, EPSILON = 1e-6;
    var SECONDARY_STYLES = { caption: true, label: true };

    function seconds(v) { return v.toFixed(1); }
```

with:

```js
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var DEAD_STRETCH_SECONDS = 3, HOOK_SECONDS = 0.5, FONT_FLOOR = 0.026, MONOTONY_SHOWS = 3, MAX_TRANSITION_KINDS = 5, EPSILON = 1e-6;
    var SECONDARY_STYLES = { caption: true, label: true };

    function seconds(v) { return v.toFixed(1); }
```

In `ui/js/storymaker/lint.js`, replace:

```js
        return out;
    }

    // film(c): c = {scenesIn, plans}. The first scene plays no transition (plan.js transitionFor), so it does not count as a kind.
    function film(c) {
        var out = [], entries = [], kinds = {};
        c.scenesIn.forEach(function (sc, i) {
            if (i > 0 && c.plans[i]) kinds[c.plans[i].requested] = true;
            if (!sc || sc.kind === 'code') return;
```

with:

```js
        return out;
    }

    // film(c): c = {scenesIn, plans, first}. first = {id, narrated, log} describes scene 1 (its schedule log is empty for a code
    // scene). The first scene plays no transition (plan.js transitionFor), so it does not count as a kind.
    function film(c) {
        var out = [], entries = [], kinds = {};
        if (c.first) {
            var landing = null;
            (c.first.log || []).forEach(function (l) {
                if (l.kind === 'show' && (landing === null || landingOf(l) < landing)) landing = landingOf(l);
            });
            var hookPath = '/scenes/0';
            if (!c.first.narrated) {
                out.push(SM.plan.issue('warning', c.first.id, hookPath, 'slow_hook', 'The first scene has no narration, so the film opens in silence.',
                    { at_seconds: 0, fix: 'Give the first scene a narration that starts the film with its key point.' }));
            } else if (landing !== null && landing > HOOK_SECONDS + EPSILON) {
                out.push(SM.plan.issue('warning', c.first.id, hookPath, 'slow_hook', 'The first thing appears after ' + seconds(landing) + ' s; the film should show something within ' + HOOK_SECONDS + ' s.',
                    { at_seconds: landing, fix: 'Show the first item or text at the start of the first scene, with no delay or entry that takes long.' }));
            }
        }
        c.scenesIn.forEach(function (sc, i) {
            if (i > 0 && c.plans[i]) kinds[c.plans[i].requested] = true;
            if (!sc || sc.kind === 'code') return;
```

- [ ] **Step 4: Rebuild the generated files**

Run: `npm run build:ui` (then `npm run check:ui`).

- [ ] **Step 5: Run the test and see it pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: 262 passed, 0 failed (the real-catalog test now allows slow_hook for firewall and the cast sheet)

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/11-real-catalog.mjs scripts/storymaker-tests/14-lint.mjs ui/js/storymaker/compile.js ui/js/storymaker/lint.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: slow_hook for a silent or slow-opening first scene"
```

### Task C10: the 9:16 caption keeps out of the platform overlays

**Files:**
- Modify: `scripts/storymaker-tests/18-captions.mjs`
- Modify: `scripts/storymaker-tests/30-layout.mjs`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)
- Modify: `ui/js/storymaker/render.js`

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/18-captions.mjs`, replace:

```js

const S9 = s9.core();

s9.check('captions: the box bottom sits at 92 % of the height in 16:9 and 1:1, and at 82 % in 9:16', () => {
    // a boxed text.subtitle, so the bottom edge can be read from the call log
    const entries = s9.catalog.entries.map(e => e.id === 'text.subtitle' ? { ...e, box: { fill: '$bg2', pad: 0.01, radius: 0.01 } } : e);
    s9Assert.ok(entries.some(e => e.id === 'text.subtitle'), 'S5 added the style to the mini catalog');
    const boxed = { ...s9.catalog, entries };
    for (const [aspect, share] of [['16:9', 0.92], ['1:1', 0.92], ['9:16', 0.82]]) {
        const tl = S9.compile({ format: { aspect, fps: 30 }, scenes: [{ id: 'a', narration: 'hallo liebe welt heute' }] }, {}, boxed).timeline;
        const cv = new S9Canvas(tl.width, tl.height);
        S9.renderFrame(tl, 10, cv.ctx, s9Assets());
```

with:

```js

const S9 = s9.core();

s9.check('captions: the box bottom sits at 92 % of the height in 16:9 and 1:1, and at 78 % in 9:16', () => {
    // a boxed text.subtitle, so the bottom edge can be read from the call log
    const entries = s9.catalog.entries.map(e => e.id === 'text.subtitle' ? { ...e, box: { fill: '$bg2', pad: 0.01, radius: 0.01 } } : e);
    s9Assert.ok(entries.some(e => e.id === 'text.subtitle'), 'S5 added the style to the mini catalog');
    const boxed = { ...s9.catalog, entries };
    for (const [aspect, share] of [['16:9', 0.92], ['1:1', 0.92], ['9:16', 0.78]]) {
        const tl = S9.compile({ format: { aspect, fps: 30 }, scenes: [{ id: 'a', narration: 'hallo liebe welt heute' }] }, {}, boxed).timeline;
        const cv = new S9Canvas(tl.width, tl.height);
        S9.renderFrame(tl, 10, cv.ctx, s9Assets());
```

In `scripts/storymaker-tests/30-layout.mjs`, replace:

```js
    assert.equal(out.report.layout, undefined);
    assert.equal(out.timeline.scenes[0].entities.some(e => e.type === 'prop' && e.hero), true);
});
```

with:

```js
    assert.equal(out.report.layout, undefined);
    assert.equal(out.timeline.scenes[0].entities.some(e => e.type === 'prop' && e.hero), true);
});

// ---- Task C10: in 9:16 the burned caption keeps out of the platform overlays: bottom at 78 % and centred in the zone band.
// The zone is internal/storymaker/layout.go SafeZone: top 0.115, right 0.130, bottom 0.219, left 0.056.
check('layout: a two-line 9:16 caption lies inside the safe zone, centred in its band; 16:9 keeps 92 %', () => {
    const long = 'Eine Firewall prueft jedes einzelne Paket am Eingang deines Netzwerks, bevor es weiterkommt.';
    const portrait = measure({ ...film({}, { narration: long }), format: { aspect: '9:16', fps: 30 } })[0].caption;
    assert.ok(portrait, 'the caption is on screen');
    const [x, y, w, h] = portrait;
    assert.ok(x >= 0.056 - 1e-9 && x + w <= 1 - 0.130 + 1e-9, 'inside the left and right zone: ' + portrait);
    assert.ok(y >= 0.115 && y + h <= 1 - 0.219 + 1e-9, 'inside the top and bottom zone: ' + portrait);
    near(y + h, 0.78, 0.01, 'the box bottom');
    near(x + w / 2, (0.056 + 0.870) / 2, 0.002, 'centred in the zone band');
    const wide = measure(film({}, { narration: long }))[0].caption;
    near(wide[1] + wide[3], 0.92, 0.01, '16:9 bottom');
    near(wide[0] + wide[2] / 2, 0.5, 0.002, '16:9 stays centred');
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL layout: a two-line 9:16 caption lies inside the safe zone, centred in its band; 16:9 keeps 92 %: AssertionError [ERR_ASSERTION]: inside the left and right zone: 0.0382,0.7371,0.9236,0.0829

- [ ] **Step 3: Implement**

In `ui/js/storymaker/render.js`, replace:

```js
    }

    // drawCaptions(ctx, timeline, n, rc): the burned caption of frame n in screen space, over whatever the frame drew. The
    // box bottom sits at 92 % of the height, 82 % in 9:16. The caption time comes from n alone.
    function drawCaptions(ctx, timeline, n, rc) {
        if (!timeline.style.captions || !timeline.captions || !timeline.captions.length) return;
        var cap = SM.captions.at(timeline.captions, n * 1000 / timeline.fps);
```

with:

```js
    }

    // drawCaptions(ctx, timeline, n, rc): the burned caption of frame n in screen space, over whatever the frame drew. The
    // box bottom sits at 92 % of the height, 78 % in 9:16. The caption time comes from n alone.
    function drawCaptions(ctx, timeline, n, rc) {
        if (!timeline.style.captions || !timeline.captions || !timeline.captions.length) return;
        var cap = SM.captions.at(timeline.captions, n * 1000 / timeline.fps);
```

In `ui/js/storymaker/render.js`, replace:

```js
        ctx.globalAlpha = 1;
    }

    // captionLayout(ctx, cap, rc): the layout, the style and the anchor (x, y of the block centre) of a burned caption. The box
    // bottom sits at 92 % of the height, 82 % in 9:16. drawCaptions draws it and the layout run measures it.
    function captionLayout(ctx, cap, rc) {
        var style = rc.cat.textStyles.subtitle || rc.cat.textStyles.title || FALLBACK_TEXT_STYLE;
        var x = rc.W / 2, bottom = rc.H * (rc.aspect === '9:16' ? 0.82 : 0.92);
        var lay = SM.text.layout(ctx, cap.lines.join('\n'), style, {
            canvasH: rc.H, short: Math.min(rc.W, rc.H), maxWidth: SM.text.maxWidthFor(rc.W, rc.safe, x, style.align || 'center'), family: familyOf(rc, style)
        });
        return { style: style, lay: lay, x: x, y: bottom - lay.height / 2 - lay.pad };
    }
```

with:

```js
        ctx.globalAlpha = 1;
    }

    // The 9:16 caption keeps out of the overlays of TikTok, Reels and Shorts: its box bottom sits at 78 % of the height and its
    // width is the band between the left and right zone, centred in that band. The numbers repeat the 9:16 row of
    // internal/storymaker/layout.go SafeZone (top 0.115, right 0.130, bottom 0.219, left 0.056); change both together.
    var PORTRAIT_CAPTION = { bottom: 0.78, left: 0.056, right: 0.130 };

    // captionLayout(ctx, cap, rc): the layout, the style and the anchor (x, y of the block centre) of a burned caption. The box
    // bottom sits at 92 % of the height, 78 % in 9:16 (see PORTRAIT_CAPTION). drawCaptions draws it and the layout run measures it.
    function captionLayout(ctx, cap, rc) {
        var style = rc.cat.textStyles.subtitle || rc.cat.textStyles.title || FALLBACK_TEXT_STYLE;
        var portrait = rc.aspect === '9:16', x = rc.W / 2, bottom = rc.H * 0.92, band = 0;
        if (portrait) {
            var left = rc.W * PORTRAIT_CAPTION.left, right = rc.W * (1 - PORTRAIT_CAPTION.right);
            x = (left + right) / 2;
            bottom = rc.H * PORTRAIT_CAPTION.bottom;
            band = right - left;
        }
        function layoutAt(maxWidth) {
            return SM.text.layout(ctx, cap.lines.join('\n'), style, { canvasH: rc.H, short: Math.min(rc.W, rc.H), maxWidth: maxWidth, family: familyOf(rc, style) });
        }
        var lay = layoutAt(portrait ? band : SM.text.maxWidthFor(rc.W, rc.safe, x, style.align || 'center'));
        // the padding of the box is outside the text width, so the box must be narrowed once more to stay in the band
        if (portrait && lay.width + lay.pad * 2 > band) lay = layoutAt(Math.max(band * 0.25, band - lay.pad * 2));
        return { style: style, lay: lay, x: x, y: bottom - lay.height / 2 - lay.pad };
    }
```

- [ ] **Step 4: Rebuild the generated files**

Run: `npm run build:ui` (then `npm run check:ui`).

- [ ] **Step 5: Run the test and see it pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: 263 passed, 0 failed (the 18-captions position check now pins 78 % in 9:16)

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/18-captions.mjs scripts/storymaker-tests/30-layout.mjs ui/js/storymaker/player.bundle.js ui/js/storymaker/render.js
git commit -m "storymaker: the 9:16 caption keeps out of the platform overlays"
```

### Task C11: a flash starts at the first frame that changed, so a cut leads into the right scene

**Files:**
- Modify: `scripts/storymaker-tests/31-checks.mjs`
- Modify: `ui/js/storymaker/checks.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated)

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/31-checks.mjs`, replace:

```js
const FPS = 30;

// pulses(period, count) = a series that holds 0.6, then dips to 0.1 and comes back `count` times, one dip every `period` frames.
// Each dip is two opposite transitions of 0.5, so one flash; the dip of flash k starts at frame 1 + k * period.
function pulses(period, count, tail) {
    const y = [0.6, 0.6];
    for (let k = 0; k < count; k++) for (let i = 0; i < period; i++) y.push(i < period / 2 ? 0.1 : 0.6);
```

with:

```js
const FPS = 30;

// pulses(period, count) = a series that holds 0.6, then dips to 0.1 and comes back `count` times, one dip every `period` frames.
// Each dip is two opposite transitions of 0.5, so one flash; the first changed frame of flash k is 2 + k * period.
function pulses(period, count, tail) {
    const y = [0.6, 0.6];
    for (let k = 0; k < count; k++) for (let i = 0; i < period; i++) y.push(i < period / 2 ? 0.1 : 0.6);
```

In `scripts/storymaker-tests/31-checks.mjs`, replace:

```js
    return y;
}

check('checks.flashes: a dip and a recovery is one flash and its start frame is the first frame of the dip', () => {
    assert.deepEqual(plain(SM.checks.flashes(pulses(4, 1, 6), FPS)), [1]);
    assert.deepEqual(plain(SM.checks.flashes(pulses(4, 3, 6), FPS)), [1, 5, 9]);
});

check('checks.flashes: three flashes in a second pass the rule only as a list, four give a window', () => {
```

with:

```js
    return y;
}

check('checks.flashes: a dip and a recovery is one flash and its start frame is the first frame that changed', () => {
    assert.deepEqual(plain(SM.checks.flashes(pulses(4, 1, 6), FPS)), [2]);
    assert.deepEqual(plain(SM.checks.flashes(pulses(4, 3, 6), FPS)), [2, 6, 10]);
});

check('checks.flashes: three flashes in a second pass the rule only as a list, four give a window', () => {
```

In `scripts/storymaker-tests/31-checks.mjs`, replace:

```js
    for (let i = 0; i < 30; i++) ramp.push(0.1 + i * 0.02);
    assert.deepEqual(plain(SM.checks.flashes(ramp, FPS)), []);
    const there = ramp.concat(ramp.slice().reverse());
    assert.deepEqual(plain(SM.checks.flashes(there, FPS)), [0]);
});

check('checks.flashes: short and empty series are safe', () => {
```

with:

```js
    for (let i = 0; i < 30; i++) ramp.push(0.1 + i * 0.02);
    assert.deepEqual(plain(SM.checks.flashes(ramp, FPS)), []);
    const there = ramp.concat(ramp.slice().reverse());
    assert.deepEqual(plain(SM.checks.flashes(there, FPS)), [1]);
});

check('checks.flashes: short and empty series are safe', () => {
```

In `scripts/storymaker-tests/31-checks.mjs`, replace:

```js
    assert.equal(f.severity, 'warning');
    assert.equal(f.scene_id, 'b');
    assert.equal(f.path, '/scenes/1');
    near(f.at_seconds, (69 - 60) / FPS, 1e-9);   // the dip starts on the last bright frame before the first dark one
    assert.match(f.message, /flash/i);
    assert.match(f.fix, /third of a second/);
});
```

with:

```js
    assert.equal(f.severity, 'warning');
    assert.equal(f.scene_id, 'b');
    assert.equal(f.path, '/scenes/1');
    near(f.at_seconds, (70 - 60) / FPS, 1e-9);   // the first frame that differs from the one before it
    assert.match(f.message, /flash/i);
    assert.match(f.fix, /third of a second/);
});
```

- [ ] **Step 2: Run it and see it fail**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL checks.run: a strobing code scene raises one flash issue for that scene at its scene-local window start: AssertionError [ERR_ASSERTION]: value: 0.3 vs 0.3333333333333333

- [ ] **Step 3: Implement**

In `ui/js/storymaker/checks.js`, replace:

```js
    }

    // flashes(y) = the start frames of the flashes in a luminance series: two consecutive transitions in opposite
    // directions, paired from the front, so four transitions are two flashes.
    function flashes(y) {
        var runs = transitions(y || []), out = [];
        for (var i = 0; i + 1 < runs.length; i++) {
            if (runs[i].dir === runs[i + 1].dir) continue;
            out.push(runs[i].from);
            i++;
        }
        return out;
```

with:

```js
    }

    // flashes(y) = the start frames of the flashes in a luminance series: two consecutive transitions in opposite
    // directions, paired from the front, so four transitions are two flashes. A flash starts at the first frame that differs
    // from the one before it, so a flash that begins with a cut belongs to the scene the cut leads into.
    function flashes(y) {
        var runs = transitions(y || []), out = [];
        for (var i = 0; i + 1 < runs.length; i++) {
            if (runs[i].dir === runs[i + 1].dir) continue;
            out.push(runs[i].from + 1);
            i++;
        }
        return out;
```

- [ ] **Step 4: Rebuild the generated files**

Run: `npm run build:ui` (then `npm run check:ui`).

- [ ] **Step 5: Run the test and see it pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: 263 passed, 0 failed

- [ ] **Step 6: Commit**

```bash
git add scripts/storymaker-tests/31-checks.mjs ui/js/storymaker/checks.js ui/js/storymaker/player.bundle.js
git commit -m "storymaker: a flash starts at the first frame that changed, so a cut leads into the right scene"
```

### Task C12: a real-browser check run reports flash for a strobing code scene and none for a calm story

**Files:**
- Modify: `internal/storymaker/render/compilecheck_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/render/compilecheck_test.go`, replace:

```go
		t.Errorf("a probe finding must never be an error: %+v", got.Issues)
	}
}
```

with:

```go
		t.Errorf("a probe finding must never be an error: %+v", got.Issues)
	}
}

// TestCompileCheckFlashBrowser runs the frame probe in the real page: a code scene that alternates a black and a white frame
// every two frames raises flash for its scene, and a calm story raises none.
func TestCompileCheckFlashBrowser(t *testing.T) {
	chrome := browserSmoke(t)
	fx := newRealFixture(t, chrome)
	fx.rebuild(t)
	ctx := context.Background()
	check := func(doc string) []storymaker.Issue {
		t.Helper()
		if _, err := fx.projects.SaveStory(ctx, fx.projectID, []byte(doc), ""); err != nil {
			t.Fatal(err)
		}
		got, err := fx.runner.CompileCheck(ctx, fx.projectID, fx.settings)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Available || storymaker.HasErrors(got.Issues) {
			t.Fatalf("summary = %+v", got)
		}
		return got.Issues
	}
	calm := `{"schema":"storymaker/1","title":"Calm","scenes":[
	 {"id":"s1","environment":"env.office","beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop"}]}]}`
	for _, is := range check(calm) {
		if is.Code == storymaker.CodeFlash || is.Code == storymaker.CodeFlashUnverified {
			t.Errorf("a calm story raised %+v", is)
		}
	}
	strobing := `{"schema":"storymaker/1","title":"Strobe","scenes":[
	 {"id":"s1","environment":"env.office","beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop"}]},
	 {"id":"c1","kind":"code"}]}`
	// 15 flashes per second: the picture flips between black and white every second frame
	src := "export default function scene(api) {\n  api.duration(3);\n  api.onFrame(function (t, g) {\n    g.fill(Math.floor(t * 15) % 2 ? '#ffffff' : '#000000');\n    g.rect(0, 0, 1920, 1080);\n  });\n}\n"
	if _, res, err := fx.projects.SaveCodeScene(ctx, fx.projectID, "c1", []byte(src), ""); err != nil || len(res.Errors) != 0 {
		t.Fatalf("SaveCodeScene: %v %+v", err, res)
	}
	var flashes []storymaker.Issue
	for _, is := range check(strobing) {
		if is.Code == storymaker.CodeFlash {
			flashes = append(flashes, is)
		}
		if is.Code == storymaker.CodeFlashUnverified {
			t.Errorf("the probe must reach every frame of a 4 s film: %+v", is)
		}
	}
	if len(flashes) != 1 {
		t.Fatalf("flash issues = %+v", flashes)
	}
	f := flashes[0]
	if f.SceneID != "c1" || f.Path != "/scenes/1" || f.Severity != storymaker.SeverityWarning || f.AtSeconds == nil || !storymaker.IssueModelFixable(f) {
		t.Errorf("flash = %+v", f)
	}
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render -run '^TestCompileCheckFlashBrowser$' -count=1 -v -timeout 10m`
Expected: compilecheck_test.go:228: flash issues = [{... SceneID:s1 ... Code:flash ...} {... SceneID:c1 ... Code:flash ...}] (first run, before C11 moved the flash start onto the first changed frame)

- [ ] **Step 3: Run the test and see it pass**

Run: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render -run '^TestCompileCheckFlashBrowser$' -count=1 -v -timeout 10m`
Expected: --- PASS: TestCompileCheckFlashBrowser (5.17s)

- [ ] **Step 4: Commit**

```bash
git add internal/storymaker/render/compilecheck_test.go
git commit -m "storymaker: a real-browser check run reports flash for a strobing code scene and none for a calm story"
```

### Task C13: a probe failure is reported as flash_unverified

**Files:**
- Modify: `internal/storymaker/render/compilecheck.go`
- Modify: `internal/storymaker/render/compilecheck_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/render/compilecheck_test.go`, replace:

```go
		t.Errorf("flash = %+v", f)
	}
}
```

with:

```go
		t.Errorf("flash = %+v", f)
	}
}

func TestCompileCheckReportsAFailedProbe(t *testing.T) {
	run := func(mutate func(rep map[string]any)) []storymaker.Issue {
		page := &fakePage{crashed: make(chan struct{})}
		page.script = play(t, playOptions{frames: 60, mutateRep: mutate})
		runner, id := compileCheckRunner(t, page)
		got, err := runner.CompileCheck(context.Background(), id, Settings{Limits: storymaker.DefaultLimits()})
		if err != nil {
			t.Fatal(err)
		}
		return got.Issues
	}
	count := func(list []storymaker.Issue) (n int) {
		for _, is := range list {
			if is.Code == storymaker.CodeFlashUnverified {
				n++
			}
		}
		return n
	}
	t.Run("a probe error with no checks is flash_unverified", func(t *testing.T) {
		list := run(func(rep map[string]any) { rep["probe_error"] = "the probe threw" })
		if count(list) != 1 {
			t.Fatalf("issues = %+v", list)
		}
		is := list[len(list)-1]
		if is.Severity != storymaker.SeverityWarning || is.Path != "/scenes/0" || is.SceneID != "s1" || is.Message == "" {
			t.Errorf("issue = %+v", is)
		}
	})
	t.Run("a flash_unverified from the page is not doubled", func(t *testing.T) {
		list := run(func(rep map[string]any) {
			rep["probe_error"] = "late"
			rep["checks"] = []any{map[string]any{"severity": "warning", "scene_id": "s1", "path": "/scenes/0", "code": "flash_unverified", "message": "not checked"}}
		})
		if count(list) != 1 {
			t.Fatalf("issues = %+v", list)
		}
	})
	t.Run("no probe error, no issue", func(t *testing.T) {
		if n := count(run(func(map[string]any) {})); n != 0 {
			t.Errorf("flash_unverified count = %d", n)
		}
	})
}
```

- [ ] **Step 2: Run it and see it fail**

Run: `go test ./internal/storymaker/render -run '^TestCompileCheckReportsAFailedProbe$' -count=1 -v`
Expected: compilecheck_test.go:258: issues = []

- [ ] **Step 3: Implement**

In `internal/storymaker/render/compilecheck.go`, replace:

```go
	Checks      []storymaker.Issue       `json:"checks"`
	Layout      []storymaker.LayoutScene `json:"layout"`
	LayoutError string                   `json:"layout_error"`
}

type compileCheckScene struct {
```

with:

```go
	Checks      []storymaker.Issue       `json:"checks"`
	Layout      []storymaker.LayoutScene `json:"layout"`
	LayoutError string                   `json:"layout_error"`
	ProbeError  string                   `json:"probe_error"`
}

type compileCheckScene struct {
```

In `internal/storymaker/render/compilecheck.go`, replace:

```go
			Fix:     "Shorten the narration or remove a scene.",
		})
	}
	issues = storymaker.CapIssues(append(issues, cleanProbeFindings(rep.Checks, want)...), storymaker.MaxHTTPIssues)
	scenes := make([]storymaker.CompileScene, 0, len(rep.Scenes))
	storyboard, code := 0, 0
	for _, sc := range rep.Scenes {
```

with:

```go
			Fix:     "Shorten the narration or remove a scene.",
		})
	}
	probe := cleanProbeFindings(rep.Checks, want)
	if rep.ProbeError != "" && !slices.ContainsFunc(probe, func(is storymaker.Issue) bool { return is.Code == storymaker.CodeFlashUnverified }) {
		// The probe or its judging threw: flashing was not checked, which is the open check flash_unverified and never a pass.
		first := ""
		if len(want.SceneIDs) > 0 {
			first = want.SceneIDs[0]
		}
		probe = append(probe, storymaker.Issue{
			Severity: storymaker.SeverityWarning, SceneID: first, Path: "/scenes/0", Code: storymaker.CodeFlashUnverified,
			Message: "The flash probe failed, so flashing was not checked: " + want.clean(clip(rep.ProbeError, 200), 200),
			Fix:     "Nothing to fix in the story; check again.",
		})
	}
	issues = storymaker.CapIssues(append(issues, probe...), storymaker.MaxHTTPIssues)
	scenes := make([]storymaker.CompileScene, 0, len(rep.Scenes))
	storyboard, code := 0, 0
	for _, sc := range rep.Scenes {
```

- [ ] **Step 4: Run the test and see it pass**

Run: `go test ./internal/storymaker/render -run '^TestCompileCheckReportsAFailedProbe$' -count=1 -v`
Expected: --- PASS: TestCompileCheckReportsAFailedProbe (1.19s)

- [ ] **Step 5: Commit**

```bash
git add internal/storymaker/render/compilecheck.go internal/storymaker/render/compilecheck_test.go
git commit -m "storymaker: a probe failure is reported as flash_unverified"
```

### Task C14: the 9:16 caption test holds a one-line and a two-line caption inside the zone

**Files:**
- Modify: `scripts/storymaker-tests/18-captions.mjs`

- [ ] **Step 1: Write the failing test**

In `scripts/storymaker-tests/18-captions.mjs`, replace:

```js
    s9Assert.ok(!needs({ ...narrated, style: { captions: false } }).includes('font:default/400'), 'captions off: nothing draws');
    s9Assert.ok(!needs({ scenes: [{ id: 'a' }] }).includes('font:default/400'), 'no narration: no caption to draw');
});
```

with:

```js
    s9Assert.ok(!needs({ ...narrated, style: { captions: false } }).includes('font:default/400'), 'captions off: nothing draws');
    s9Assert.ok(!needs({ scenes: [{ id: 'a' }] }).includes('font:default/400'), 'no narration: no caption to draw');
});

// ---- Task C14: in 9:16 the whole caption box, one line or two, lies inside the platform zone (layout.go SafeZone).
s9.check('captions: a one-line and a two-line 9:16 caption box lie inside the zone, bottom at most 0.781 of the height', () => {
    const entries = s9.catalog.entries.map(e => e.id === 'text.subtitle' ? { ...e, box: { fill: '$bg2', pad: 0.01, radius: 0.01 } } : e);
    const boxed = { ...s9.catalog, entries };
    const lines = [];
    for (const narration of ['hallo liebe welt heute', 'Eine Firewall prueft jedes einzelne Paket am Eingang deines Netzwerks, bevor es weiterkommt.']) {
        const tl = S9.compile({ format: { aspect: '9:16', fps: 30 }, scenes: [{ id: 'a', narration }] }, {}, boxed).timeline;
        const cv = new S9Canvas(tl.width, tl.height);
        S9.renderFrame(tl, 10, cv.ctx, s9Assets());
        // the four corners of the rounded box are the first control point of each arcTo
        const corners = cv.ctx.log.filter(l => l.startsWith('arcTo(')).map(l => l.slice(6, -1).split(',').map(Number));
        s9Assert.equal(corners.length, 4, 'one box with four corners');
        const xs = corners.map(p => p[0]), ys = corners.map(p => p[1]);
        const left = Math.min(...xs), right = Math.max(...xs), top = Math.min(...ys), bottom = Math.max(...ys);
        s9Assert.ok(bottom <= 0.781 * tl.height + 0.001 * tl.height, narration + ': bottom ' + bottom / tl.height);
        s9Assert.ok(left >= 0.056 * tl.width - 0.001 * tl.width, narration + ': left ' + left / tl.width);
        s9Assert.ok(right <= 0.870 * tl.width + 0.001 * tl.width, narration + ': right ' + right / tl.width);
        s9Assert.ok(top >= 0.115 * tl.height, narration + ': top ' + top / tl.height);
        lines.push(cv.ctx.log.filter(l => l.startsWith('fillText(')).length);
    }
    s9Assert.ok(lines.some(n => n >= 2), 'one of the captions has two lines: ' + lines);
});
```

- [ ] **Step 2: Run the test and see it pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: 264 passed, 0 failed

(No red state: the current render.js already satisfies the new asserts: bottom at most 0.781 H, x-range inside 0.056 W..0.870 W, top at least 0.115 H, for both captions; no render.js change was needed.)

- [ ] **Step 3: Commit**

```bash
git add scripts/storymaker-tests/18-captions.mjs
git commit -m "storymaker: the 9:16 caption test holds a one-line and a two-line caption inside the zone"
```

