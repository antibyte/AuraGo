// Task W3: the parallel scheduler of the render page: in-order posting, the reorder window, the fixed deal of chunks, restart and crash.
import assert from 'node:assert/strict';
import { loadHost, plain, tick, settle, until } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check } = t;
const runPool = opts => loadHost().StorymakerHost.runPool(opts);

// One storyboard scene over the whole film unless a test passes scenes.
const scheduleOf = (limit, scenes) => ({
    fps: 30, frame_count: limit,
    scenes: scenes || [{ id: 'a', kind: 'storyboard', start_frame: 0, end_frame: limit, tf: 0, transition: 'cut' }]
});

// Fake engines: render(n) yields once, logs who drew what, and hands back a blob that names the frame.
function fakeEngines(count, schedule, log, failAt) {
    return Array.from({ length: count }, (_, w) => ({
        schedule,
        async render(n, o) {
            log.push({ w, n, png: o && o.png });
            await tick();
            if (failAt === n) throw { code: 'deadline', message: 'worker ' + w + ' overran frame ' + n };
            return { frame: n };
        }
    }));
}

check('pool: frames post in order with one POST in flight, each drawn by its worker in the fixed deal', async () => {
    // 3 workers, chunks of 4, 29 frames: frame f is in chunk floor(f/4), and chunk j goes to worker j % 3.
    const limit = 29, log = [], posted = [];
    let inFlight = 0, most = 0;
    const outcome = await runPool({
        engines: fakeEngines(3, scheduleOf(limit), log), limit, chunk: 4, preRoll: 0, status: {},
        post: async (n, blob) => {
            inFlight++;
            most = Math.max(most, inFlight);
            assert.equal(blob.frame, n);
            posted.push(n);
            await tick();
            inFlight--;
            return n === limit - 1 ? { ok: true, done: true } : { ok: true };
        }
    });
    assert.equal(plain(outcome).phase, 'done');
    assert.deepEqual(posted, [...Array(limit).keys()]);
    assert.equal(most, 1);
    assert.equal(log.length, limit, 'without pre-roll each frame renders once');
    for (const e of log) {
        assert.equal(e.w, Math.floor(e.n / 4) % 3, 'worker of frame ' + e.n);
        assert.equal(e.png, true, 'the PNG is encoded in the worker');
    }
});

check('pool: a worker waits while its chunk starts N*C*2 frames or more past the next frame to post', async () => {
    // 2 workers, chunks of 2: the window is 2*2*2 = 8. The first POST hangs, so the next frame stays 0.
    // Chunks starting at 0, 2, 4 and 6 may run, and they end at frame 7. A chunk starting at 8 may not.
    const limit = 40, log = [];
    let release;
    const hold = new Promise(resolve => { release = resolve; });
    const done = runPool({
        engines: fakeEngines(2, scheduleOf(limit), log), limit, chunk: 2, preRoll: 0, status: {},
        post: async n => {
            if (n === 0) await hold;
            return n === limit - 1 ? { ok: true, done: true } : { ok: true };
        }
    });
    await until(() => log.length >= 8, 'the window to fill');
    await settle(40);
    assert.equal(Math.max(...log.map(e => e.n)), 7, 'nothing past the window runs while frame 0 is in flight');
    release();
    assert.equal(plain(await done).phase, 'done');
    assert.equal(Math.max(...log.map(e => e.n)), limit - 1);
});

check('pool: a chunk with code-scene frames goes to worker 0, also for the outgoing layer of a transition', async () => {
    // 2 workers, chunks of 4, 40 frames. Scene b (code) runs 16..27. Scene c starts at 28 with a 2-frame fade, so frames 28
    // and 29 still draw b. Chunks 4, 5, 6 and 7 hold code frames and go to worker 0. Without code, 5 and 7 would go to worker 1.
    const limit = 40, log = [];
    const schedule = scheduleOf(limit, [
        { id: 'a', kind: 'storyboard', start_frame: 0, end_frame: 16, tf: 0, transition: 'cut' },
        { id: 'b', kind: 'code', start_frame: 16, end_frame: 28, tf: 0, transition: 'cut' },
        { id: 'c', kind: 'storyboard', start_frame: 28, end_frame: 40, tf: 2, transition: 'fade' }
    ]);
    await runPool({
        engines: fakeEngines(2, schedule, log), limit, chunk: 4, preRoll: 0, status: {},
        post: async n => (n === limit - 1 ? { ok: true, done: true } : { ok: true })
    });
    const byFrame = new Map(log.map(e => [e.n, e.w]));
    for (let f = 16; f < 32; f++) assert.equal(byFrame.get(f), 0, 'code chunk frame ' + f);
    for (const f of [4, 12, 36]) assert.equal(byFrame.get(f), 1, 'chunks 1, 3 and 9 keep the deal j % 2, frame ' + f);
    for (const f of [0, 8, 32]) assert.equal(byFrame.get(f), 0, 'chunks 0, 2 and 8 keep the deal j % 2, frame ' + f);
});

check('pool: a restart abandons the generation and runs the same engines again from frame 0', async () => {
    const limit = 12, log = [], posted = [], status = {};
    let restarted = false;
    const outcome = await runPool({
        engines: fakeEngines(2, scheduleOf(limit), log), limit, chunk: 4, preRoll: 0, status,
        post: async n => {
            posted.push(n);
            if (n === 5 && !restarted) { restarted = true; return { restart: true }; }
            return n === limit - 1 ? { ok: true, done: true } : { ok: true };
        }
    });
    assert.equal(plain(outcome).phase, 'done');
    assert.deepEqual(posted, [0, 1, 2, 3, 4, 5, ...Array(limit).keys()]);
    assert.equal(status.restarts, 1);
    assert.equal(log.filter(e => e.n === 0).length, 2, 'frame 0 is drawn again');
});

check('pool: a worker that fails rejects the run and stops the poster', async () => {
    // 2 workers, chunks of 4: frame 6 is in chunk 1 on worker 1 and fails, so no frame past 5 can ever post.
    const limit = 24, log = [], posted = [];
    const run = runPool({
        engines: fakeEngines(2, scheduleOf(limit), log, 6), limit, chunk: 4, preRoll: 0, status: {},
        post: async n => { posted.push(n); return { ok: true }; }
    });
    await assert.rejects(run, e => e.code === 'deadline');
    assert.ok(posted.every(n => n <= 5), JSON.stringify(posted));
});
