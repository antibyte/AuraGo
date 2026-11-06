// Storymaker host (document realm): the parallel frame scheduler of the render page. Chunks of C frames go to the player
// engines in a fixed deal, the frames come back as PNG blobs encoded in the workers, and one poster sends them in frame order
// with one POST in flight, so the ingest rule of Go stays as it is. Classic script; publishes StorymakerHost.runPool.
(function (root) {
    'use strict';
    var H = root.StorymakerHost || (root.StorymakerHost = {});

    // frameHasCode(schedule, f): true when frame f draws a code scene, also as the outgoing layer of a transition.
    function frameHasCode(schedule, f) {
        var layers = root.StorymakerCore.schedule.framePlan(schedule, f).layers;
        for (var i = 0; i < layers.length; i++) {
            if (schedule.scenes[layers[i].i].kind === 'code') return true;
        }
        return false;
    }

    // dealChunks: chunk j goes to worker j % N, and a chunk with code-scene frames to worker 0, the only one with scene workers.
    function dealChunks(schedule, limit, chunk, workers) {
        var deal = [], w, j, f;
        for (w = 0; w < workers; w++) deal.push([]);
        for (j = 0; j * chunk < limit; j++) {
            var hasCode = false, end = Math.min(limit, (j + 1) * chunk);
            for (f = j * chunk; f < end && !hasCode; f++) hasCode = frameHasCode(schedule, f);
            deal[hasCode ? 0 : j % workers].push(j);
        }
        return deal;
    }

    function wake(gen) {
        var waiting = gen.waiters;
        gen.waiters = [];
        for (var i = 0; i < waiting.length; i++) waiting[i]();
    }
    function sleepOn(gen) {
        return new Promise(function (resolve) { gen.waiters.push(resolve); });
    }
    function abort(gen, err) {
        if (err && !gen.err) gen.err = err;
        gen.abort = true;
        wake(gen);
    }

    // renderChunks: one worker's loop. It waits while its next chunk starts span frames or more past the next frame to post.
    // The worker that owns the next frame never waits, so the window cannot deadlock.
    async function renderChunks(gen, o, engine, chunks, span) {
        try {
            for (var i = 0; i < chunks.length; i++) {
                var start = chunks[i] * o.chunk, end = Math.min(o.limit, start + o.chunk), f;
                while (!gen.abort && start - gen.next >= span) await sleepOn(gen);
                if (gen.abort) return;
                for (f = Math.max(0, start - o.preRoll); f < start; f++) {
                    await engine.render(f, { png: true });   // pre-roll: painted, dropped
                    if (gen.abort) return;
                }
                for (f = start; f < end; f++) {
                    var blob = await engine.render(f, { png: true });
                    if (gen.abort) return;
                    gen.buffer[f] = blob;
                    wake(gen);
                }
            }
        } catch (err) {
            abort(gen, err);
        }
    }

    // postFrames: the one in-order poster. A reply without stop, restart or done moves it to the next frame.
    async function postFrames(gen, o) {
        try {
            while (!gen.abort) {
                var n = gen.next, blob = gen.buffer[n];
                if (blob === undefined) { await sleepOn(gen); continue; }
                delete gen.buffer[n];
                var reply = await o.post(n, blob);
                o.status.frame = n;
                if (reply.stop) { gen.outcome = { phase: 'stopped', error: reply.error || '', message: reply.message || '' }; break; }
                if (reply.restart) break;
                if (reply.done) { gen.outcome = { phase: 'done' }; break; }
                gen.next = n + 1;
                if (gen.next >= o.limit) throw H.fail('render_failed', 'The page ran past the last frame (' + o.limit + ') without a done answer.');
                wake(gen);
            }
            abort(gen);
        } catch (err) {
            abort(gen, err);
        }
    }

    // runPool(o): o = {engines, limit, chunk, preRoll, post(n, blob), status}. engines[0] holds the code scenes. Resolves with
    // {phase: 'done'} or {phase: 'stopped', error, message}. A crash, a deadline, a failed POST or an overrun rejects. On a
    // restart answer the generation is abandoned, its buffer goes with it, and the same engines run again from frame 0.
    H.runPool = async function (o) {
        var workers = o.engines.length, span = workers * o.chunk * 2;
        var deal = dealChunks(o.engines[0].schedule, o.limit, o.chunk, workers);
        o.status.workers = workers;
        for (;;) {
            var gen = { abort: false, next: 0, buffer: {}, waiters: [], err: null, outcome: null };
            var tasks = [postFrames(gen, o)];
            for (var w = 0; w < workers; w++) tasks.push(renderChunks(gen, o, o.engines[w], deal[w], span));
            await Promise.all(tasks);
            if (gen.err) throw gen.err;
            if (gen.outcome) return gen.outcome;
            o.status.restarts = (o.status.restarts || 0) + 1;
        }
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
