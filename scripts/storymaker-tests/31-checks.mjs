// Task C2 and following: the measured checks of checks.js, pure functions on the series the frame probe records.
import assert from 'node:assert/strict';
import { plain, loadHost } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
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
