// Storymaker app preview audio: the parent side of the sound. The player iframe has no audio or connect access, so this module
// plays the narration pieces of the project's /narration answer next to it, on the schedule of report.audio_events. Pieces are
// decoded in a sliding window, 2 s behind to 20 s ahead of the playhead. The owner calls sync(frame) on each time message while
// playing, and a drift above 120 ms reschedules. Audio URLs come only from api.narrationUrl with values of the parent's own
// answer, never from message text. bindAudio joins it to the editor. Classic script, publishes S.audioModel, S.createAudio and
// S.bindAudio.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const WINDOW_BACK_MS = 2000;
    const WINDOW_AHEAD_MS = 20000;
    const DRIFT_MS = 120;
    const SPOKEN_KINDS = ['narration', 'dialogue'];

    // eachBlock(narration, fn): every block of an answer's narration map, {<scene id>: {blocks}}.
    function eachBlock(narration, fn) {
        Object.keys(narration || {}).forEach(sceneId => {
            const scene = narration[sceneId];
            (scene && Array.isArray(scene.blocks) ? scene.blocks : []).forEach(fn);
        });
    }

    // windowFor(events, ms): the events that start from 2 s behind to 20 s ahead of the playhead.
    function windowFor(events, ms) {
        return (Array.isArray(events) ? events : []).filter(event => event.start_ms >= ms - WINDOW_BACK_MS && event.start_ms <= ms + WINDOW_AHEAD_MS);
    }

    // waveform(report, narration): one level per frame of the film. Block levels sit at the block's frames, everything else is 0.
    // An event's start_ms is the block start plus the piece's at_ms, so any event of a block gives the block start.
    function waveform(report, narration) {
        const r = report || {};
        const count = r.frame_count > 0 ? r.frame_count : 0;
        const fps = r.fps > 0 ? r.fps : 30;
        const wave = new Uint8Array(count);
        const blocks = new Map();
        eachBlock(narration, block => blocks.set(block.id, block));
        const done = new Set();
        (Array.isArray(r.audio_events) ? r.audio_events : []).forEach(event => {
            const block = blocks.get(event.block_id);
            if (!block || done.has(block.id)) return;
            const piece = Array.isArray(block.pieces) ? block.pieces[event.piece] : null;
            if (!piece) return;
            done.add(block.id);
            const first = Math.round((event.start_ms - piece.at_ms) * fps / 1000);
            (Array.isArray(block.levels) ? block.levels : []).forEach((level, k) => {
                const frame = first + k;
                if (frame >= 0 && frame < count && level > wave[frame]) wave[frame] = level;
            });
        });
        return wave;
    }

    // createAudio({api, projectId}) -> {load(info, events), play(frame), pause(), seek(frame), sync(frame), dispose()}.
    // info = {fps, frame_count, narration, music, volume, catalog}, events = report.audio_events. A piece's hash and ext come from
    // info.narration. An event whose ref is not in that answer is ignored.
    function createAudio(options) {
        const o = options || {};
        const api = o.api, projectId = o.projectId;
        const lifetime = new AbortController();
        const buffers = new Map();
        const nodes = new Set();
        let ctx = null, info = {}, pieces = new Map(), events = [], started = new Set();
        let playing = false, disposed = false, origin = 0, generation = 0;

        function msOf(frame) {
            return Math.max(0, frame) * 1000 / (info.fps > 0 ? info.fps : 30);
        }

        function piecesOf(narration) {
            const map = new Map();
            eachBlock(narration, block => (Array.isArray(block.pieces) ? block.pieces : []).forEach(piece => map.set(piece.hash, piece)));
            return map;
        }

        // Own copies with end_ms, the film time where the used part of the piece ends.
        function spokenEvents(list) {
            const out = [];
            (Array.isArray(list) ? list : []).forEach(event => {
                const piece = event && pieces.get(event.ref);
                if (!piece || SPOKEN_KINDS.indexOf(event.kind) < 0) return;
                out.push({ ref: event.ref, start_ms: event.start_ms, end_ms: event.start_ms + piece.out_ms - piece.in_ms });
            });
            return out;
        }

        function stopAll() {
            generation++;
            started = new Set();
            nodes.forEach(node => {
                node.onended = null;
                try { node.stop(); } catch (_) { /* already over */ }
                try { node.disconnect(); } catch (_) { /* already gone */ }
            });
            nodes.clear();
        }

        async function fetchAndDecode(hash) {
            const url = api.narrationUrl(projectId, hash, pieces.get(hash).ext);
            if (!url) throw new Error('refused');
            const response = await window.fetch(url, { credentials: 'same-origin', signal: lifetime.signal });
            if (!response.ok) throw new Error('audio answer ' + response.status);
            return ctx.decodeAudioData(await response.arrayBuffer());
        }

        // A piece that fails stays silent and leaves the cache, so a later schedule may try it again.
        function decode(hash) {
            if (!buffers.has(hash)) buffers.set(hash, fetchAndDecode(hash).catch(() => { buffers.delete(hash); return null; }));
            return buffers.get(hash);
        }

        function startPiece(event, buffer) {
            const piece = pieces.get(event.ref);
            const when = origin + event.start_ms / 1000;
            const now = ctx.currentTime;
            const late = Math.max(0, now - when);
            const length = (piece.out_ms - piece.in_ms) / 1000;
            if (late >= length) return;
            const source = ctx.createBufferSource();
            source.buffer = buffer;
            source.connect(ctx.destination);
            source.start(Math.max(when, now), piece.in_ms / 1000 + late, length - late);
            source.onended = () => nodes.delete(source);
            nodes.add(source);
        }

        // The window of windowFor, plus an older piece that is still sounding, so a seek into a long sentence plays its rest.
        // Buffers outside the result are dropped, which bounds the decoded memory.
        function fill(ms) {
            const wanted = windowFor(events, ms).concat(events.filter(event => event.start_ms < ms - WINDOW_BACK_MS && event.end_ms > ms));
            const keep = new Set(wanted.map(event => event.ref));
            Array.from(buffers.keys()).forEach(hash => { if (!keep.has(hash)) buffers.delete(hash); });
            const mine = generation;
            wanted.forEach(event => {
                if (started.has(event)) return;
                started.add(event);
                decode(event.ref).then(buffer => { if (buffer && playing && mine === generation) startPiece(event, buffer); });
            });
        }

        function reschedule(ms) {
            stopAll();
            origin = ctx.currentTime - ms / 1000;
            fill(ms);
        }

        return {
            load(nextInfo, nextEvents) {
                info = nextInfo || {};
                pieces = piecesOf(info.narration);
                events = spokenEvents(nextEvents);
                if (playing) reschedule((ctx.currentTime - origin) * 1000);
            },
            // Call this from the click that starts playback: the browser allows sound only after a user gesture.
            play(frame) {
                if (disposed) return;
                if (!ctx) {
                    const Context = window.AudioContext || window.webkitAudioContext;
                    if (!Context) return;
                    ctx = new Context();
                }
                if (ctx.state === 'suspended') ctx.resume().catch(() => { /* stays silent */ });
                playing = true;
                reschedule(msOf(frame));
            },
            pause() {
                playing = false;
                stopAll();
            },
            seek(frame) {
                if (playing) reschedule(msOf(frame));
            },
            sync(frame) {
                if (!playing) return;
                const ms = msOf(frame);
                if (Math.abs((ctx.currentTime - origin) * 1000 - ms) > DRIFT_MS) reschedule(ms);
                else fill(ms);
            },
            dispose() {
                if (disposed) return;
                disposed = true;
                playing = false;
                stopAll();
                lifetime.abort();
                buffers.clear();
                if (ctx) ctx.close().catch(() => { /* already closed */ });
                ctx = null;
            }
        };
    }

    S.audioModel = { waveform, windowFor };
    const AUDIO_JOB_KINDS = ['narrate', 'music'];
    const JOB_DONE = ['succeeded', 'failed', 'cancelled', 'interrupted'];

    // bindAudio(inst) -> {refresh(), dispose()} joins the preview audio to the editor. It sets inst.audio, inst.audioInfo and
    // inst.refreshAudio, and emits bus event audio on each fresh answer. It asks for the /narration answer on project open, after a
    // successful save (afterStorySaved calls inst.refreshAudio), and once per narrate or music job on its terminal status. When the
    // narration of a fresh answer differs from the one last sent, the player compiles again and the next report feeds
    // createAudio.load. inst.loadSeq inside reloadPlayer makes the newest load win. An older answer, or one for another project,
    // is ignored. A failed request keeps the last answer and sets inst.audioError, and a fresh answer clears it. Only without a last
    // answer does the failure emit audio, because that event also clears the export's refusal and the music start issue.
    function bindAudio(inst) {
        const audio = S.createAudio({ api: inst.api, projectId: inst.projectId });
        const finished = new Set();
        let seq = 0, disposed = false, sent = '{}';

        async function refresh() {
            const projectId = inst.projectId;
            const mine = ++seq;
            let answer;
            try {
                answer = await inst.api.narration(projectId);
            } catch (err) {
                if (disposed || mine !== seq || inst.projectId !== projectId || (err && err.aborted)) return null;
                inst.audioError = err;
                if (!inst.audioInfo) inst.bus.emit('audio');
                return null; // the last answer stays, and the next trigger asks again
            }
            if (disposed || mine !== seq || inst.projectId !== projectId) return null;
            inst.audioInfo = answer;
            inst.audioError = null;
            inst.bus.emit('audio');
            const narration = JSON.stringify(answer && answer.narration ? answer.narration : {});
            if (narration !== sent) {
                sent = narration;
                inst.reloadPlayer();
            }
            return answer;
        }

        const off = [
            inst.bus.on('report', () => {
                const report = inst.report;
                if (!report) return;
                const answer = inst.audioInfo || {};
                // volume stays 1 until the story has music
                audio.load({ fps: report.fps, frame_count: report.frame_count, narration: answer.narration || {}, music: answer.music || null, volume: 1, catalog: inst.catalog }, report.audio_events);
            }),
            inst.bus.on('job', job => {
                if (!job || AUDIO_JOB_KINDS.indexOf(job.kind) < 0 || JOB_DONE.indexOf(job.status) < 0 || finished.has(job.id)) return;
                finished.add(job.id);
                refresh();
            }),
            inst.player.on('loading', () => audio.pause()),
            inst.player.on('time', ({ frame, playing }) => {
                if (playing) audio.sync(frame);
                else audio.pause();
            })
        ];

        inst.audio = audio;
        inst.audioError = null;
        inst.audioInfo = null;
        inst.refreshAudio = refresh;
        refresh();

        return {
            refresh,
            dispose() {
                if (disposed) return;
                disposed = true;
                off.forEach(fn => fn());
                audio.dispose();
                inst.audio = null;
                inst.audioInfo = null;
                inst.audioError = null;
                inst.refreshAudio = null;
            }
        };
    }

    S.createAudio = createAudio;
    S.bindAudio = bindAudio;
})();
