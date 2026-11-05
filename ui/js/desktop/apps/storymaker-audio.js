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
            // An answer can change only the music, and then no new compile follows, so the answer feeds the engine here.
            if (inst.report) audio.load(S.audioModel.infoFor(inst), inst.report.audio_events);
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
                audio.load(S.audioModel.infoFor(inst), report.audio_events);
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
    // ---- music and SFX: a layer around the narration engine -------------------------------------------------------------------
    (function addMusicAndSfx() {
        const SPEECH_LEVEL = 64;      // -45 dBFS on the 0..255 level scale: (-45 + 60) * 255 / 60 = 63.75, the threshold of mix.go
        const DUCK_DB = 6;
        const LOOK_MS = 50;
        const ATTACK_MS = 50;
        const RELEASE_MS = 400;
        const DRIFT_S = 0.12;
        const LATE_S = 0.05;          // a sound this little behind the clock still plays, at once, as startPiece does for a late piece
        const DEFAULT_VOLUME = 0.25;
        const BED_BELOW_NARRATION_LU = -12;   // bedBelowNarrLU of mix.go
        const BED_NO_NARRATION_LUFS = -18;    // bedNoNarrLUFS of mix.go
        const BED_MAX_GAIN_DB = 24;           // bedMaxGainDB of mix.go
        const SFX_BASE = '/img/storymaker/';
        const SFX_FILE = /^sfx\/[A-Za-z0-9._-]+\.wav$/;

        // Frames in which speech sounds above the threshold, from the stored per-frame levels of each block.
        function speechFrames(events, narration, fps, frameCount) {
            const speech = new Uint8Array(Math.max(0, Math.floor(frameCount) || 0));
            const blocks = {};
            Object.keys(narration || {}).forEach(sceneId => {
                const scene = narration[sceneId];
                (scene && Array.isArray(scene.blocks) ? scene.blocks : []).forEach(block => { if (block && block.id) blocks[block.id] = block; });
            });
            const done = new Set();
            (Array.isArray(events) ? events : []).forEach(event => {
                const block = event ? blocks[event.block_id] : null;
                if (!block || done.has(block.id) || !Array.isArray(block.pieces) || !Array.isArray(block.levels)) return;
                const piece = block.pieces[Number(event.piece) || 0];
                if (!piece) return;
                done.add(block.id);
                const first = Math.round((event.start_ms - piece.at_ms) / 1000 * fps);
                block.levels.forEach((level, k) => {
                    const frame = first + k;
                    if (frame >= 0 && frame < speech.length && level >= SPEECH_LEVEL) speech[frame] = 1;
                });
            });
            return speech;
        }

        // The bed's linear gain at a frame: full depth from 50 ms before speech, a 50 ms attack ahead of that, a 400 ms release after it.
        function duckAt(speech, frame, fps) {
            const msPerFrame = 1000 / fps;
            let depth = 0;
            const ahead = Math.ceil((LOOK_MS + ATTACK_MS) / msPerFrame);
            for (let k = 0; k <= ahead; k++) {
                if (!speech[frame + k]) continue;
                const ms = k * msPerFrame;
                depth = Math.max(depth, ms <= LOOK_MS ? 1 : Math.max(0, 1 - (ms - LOOK_MS) / ATTACK_MS));
                break;
            }
            const behind = Math.ceil(RELEASE_MS / msPerFrame);
            for (let k = 0; k <= behind; k++) {
                if (!speech[frame - k]) continue;
                depth = Math.max(depth, Math.max(0, 1 - k * msPerFrame / RELEASE_MS));
                break;
            }
            return Math.pow(10, -DUCK_DB * depth / 20);
        }

        // The narration level of computeMixLevels in mix.go, from the stored levels: the power mean of the speech frames on the byte
        // scale of the envelopes (level * 60 / 255 - 60 dB). null without speech.
        function narrationLufs(narration) {
            let sum = 0;
            let count = 0;
            Object.keys(narration || {}).forEach(sceneId => {
                const scene = narration[sceneId];
                (scene && Array.isArray(scene.blocks) ? scene.blocks : []).forEach(block => {
                    (block && Array.isArray(block.levels) ? block.levels : []).forEach(level => {
                        if (!(level >= SPEECH_LEVEL)) return;
                        sum += Math.pow(10, (level * 60 / 255 - 60) / 10);
                        count++;
                    });
                });
            });
            return count ? 10 * Math.log10(sum / count) : null;
        }

        // The level that bed and SFX aim at in the export: narration minus 12 LU, or -18 LUFS without narration, moved by the
        // volume of a bed relative to 0.25.
        function referenceLevel(info) {
            const narration = narrationLufs(info.narration);
            const base = narration === null ? BED_NO_NARRATION_LUFS : narration + BED_BELOW_NARRATION_LU;
            const volume = Number(info.volume);
            return info.music && volume > 0 ? base + 20 * Math.log10(volume / DEFAULT_VOLUME) : base;
        }

        // The info of createAudio.load from the instance. The volume is the story's, the music comes from the /narration answer.
        function infoFor(inst) {
            const answer = inst.audioInfo || {};
            const report = inst.report || {};
            const music = inst.story && inst.story.music;
            return {
                fps: report.fps,
                frame_count: report.frame_count,
                narration: answer.narration || {},
                music: answer.music || null,
                volume: music && typeof music.volume === 'number' ? music.volume : DEFAULT_VOLUME,
                catalog: inst.catalog || null
            };
        }

        const narrationEngine = S.createAudio;

        S.createAudio = function createAudioWithSound(options) {
            const engine = narrationEngine(options);
            const api = options.api;
            const projectId = options.projectId;
            let info = {};
            let sfxEvents = [];
            let speech = new Uint8Array(0);
            let reference = BED_NO_NARRATION_LUFS;   // what referenceLevel gave for the current info
            let ctx = null;
            let playing = false;
            let last = 0;
            let anchor = 0;       // the context time of film time 0
            let generation = 0;   // a decode that ends after a new schedule is dropped
            let bed = null;
            let disposed = false;
            const buffers = new Map();   // take file -> Promise of its AudioBuffer
            const live = new Map();      // event key -> the source node

            function fps() { return Number(info.fps) > 0 ? Number(info.fps) : 30; }

            function bedUrl() {
                const music = info.music;
                if (!music || !(Number(info.volume) > 0)) return '';
                return api.musicUrl(projectId, music.key, music.ext);
            }

            // Only what can really sound opens a second AudioContext: a bed, or an sfx event whose take is in the catalog. E1c's test feeds an
            // sfx event without a catalog and counts one context.
            function needsSound() { return !!bedUrl() || sfxEvents.some(event => takeOf(event)); }

            function openContext() {
                if (ctx || disposed) return ctx;
                const Ctor = window.AudioContext || window.webkitAudioContext;
                if (!Ctor) return null;
                ctx = new Ctor();
                return ctx;
            }

            // One <audio> element for the whole session: a new key only changes its src. Taking the src away needs load() after it, or the
            // element keeps its connection and the file handle open.
            function syncBed() {
                const url = bedUrl();
                if (!url) {
                    if (bed && bed.url) { bed.url = ''; bed.element.pause(); bed.element.removeAttribute('src'); bed.element.load(); }
                    return;
                }
                if (!ctx) return;
                if (!bed) {
                    const element = document.createElement('audio');
                    element.preload = 'auto';
                    element.loop = true;
                    const source = ctx.createMediaElementSource(element);
                    const gain = ctx.createGain();
                    source.connect(gain);
                    gain.connect(ctx.destination);
                    bed = { element, source, gain, url: '' };
                }
                if (bed.url !== url) {
                    bed.url = url;
                    bed.element.src = url;
                    bed.element.load();
                }
            }

            function placeBed(frame) {
                if (!bed || !bed.url || !info.music) return;
                const length = Number(info.music.duration_ms) / 1000;
                const seconds = frame / fps();
                const want = length > 0 ? seconds % length : seconds;
                if (Math.abs(bed.element.currentTime - want) > DRIFT_S) bed.element.currentTime = want;
            }

            function applyDuck(frame) {
                if (!bed || !bed.url || !ctx) return;
                // The export's bed gain when the server measured the track. Otherwise the raw file at the story volume, and the music panel says so.
                const measured = info.music ? info.music.loudness_i : null;
                const level = Number.isFinite(measured) ? Math.pow(10, Math.min(reference - measured, BED_MAX_GAIN_DB) / 20) : S.clamp(info.volume, 0, 1);
                bed.gain.gain.setTargetAtTime(level * duckAt(speech, frame, fps()), ctx.currentTime, 0.02);
            }

            // Only a take of a catalog sfx entry gives a file: ref and take are lookups, never parts of a URL.
            function takeOf(event) {
                const catalog = info.catalog;
                const entry = catalog && catalog.byId ? catalog.byId[event.ref] : null;
                if (!entry || entry.kind !== 'sfx' || !Array.isArray(entry.takes) || !Number.isInteger(event.take)) return null;
                const take = entry.takes[event.take];
                return take && typeof take.file === 'string' && SFX_FILE.test(take.file) ? { entry, take } : null;
            }

            // The gain of an effect in the export: the reference plus the entry's over_db minus the take's L50 (Mix in mix.go).
            function sfxGain(found) {
                const over = found.entry.hit && Number.isFinite(found.entry.hit.over_db) ? found.entry.hit.over_db : 0;
                return Number.isFinite(found.take.l50_lufs) ? Math.pow(10, (reference + over - found.take.l50_lufs) / 20) : 1;
            }

            function decode(file) {
                if (!buffers.has(file)) {
                    buffers.set(file, window.fetch(SFX_BASE + file, { credentials: 'same-origin' })
                        .then(response => {
                            if (!response.ok) throw new Error('sound effect not found');
                            return response.arrayBuffer();
                        })
                        .then(data => ctx.decodeAudioData(data)));
                }
                return buffers.get(file);
            }

            function scheduleWindow(frame) {
                if (!ctx || !playing) return;
                S.audioModel.windowFor(sfxEvents, frame / fps() * 1000).forEach(event => {
                    const eventKey = event.start_ms + '|' + event.ref + '|' + event.take;
                    if (live.has(eventKey)) return;
                    const when = anchor + event.start_ms / 1000;
                    if (when < ctx.currentTime - LATE_S) return;
                    const found = takeOf(event);
                    if (!found) return;
                    const level = sfxGain(found);
                    live.set(eventKey, null);
                    const seen = generation;
                    decode(found.take.file).then(buffer => {
                        if (disposed || seen !== generation || !playing || !buffer) return;
                        const at = anchor + event.start_ms / 1000;
                        const now = ctx.currentTime;
                        if (at < now - LATE_S) return;
                        const node = ctx.createBufferSource();
                        const gain = ctx.createGain();
                        node.buffer = buffer;
                        gain.gain.value = level;
                        node.connect(gain);
                        gain.connect(ctx.destination);
                        node.onended = () => gain.disconnect();
                        node.start(Math.max(at, now));
                        live.set(eventKey, { node, gain });
                    }, () => { /* a sound effect that cannot load stays silent in the preview */ });
                });
            }

            function stopSfx() {
                generation++;
                live.forEach(item => {
                    if (!item) return;
                    try { item.node.stop(); } catch (_) { /* already over */ }
                    item.gain.disconnect();
                });
                live.clear();
            }

            function begin(frame) {
                anchor = ctx.currentTime - frame / fps();   // the anchor of the narration engine, so a sound lands on the word
                stopSfx();
                scheduleWindow(frame);
                if (bed && bed.url) {
                    placeBed(frame);
                    applyDuck(frame);
                    const started = bed.element.play();
                    if (started && typeof started.catch === 'function') started.catch(() => { /* the owner shows the transport state */ });
                }
            }

            return {
                load(nextInfo, events) {
                    info = nextInfo || {};
                    const all = Array.isArray(events) ? events : [];
                    sfxEvents = all.filter(event => event && event.kind === 'sfx');
                    speech = speechFrames(all, info.narration, fps(), Number(info.frame_count) || 0);
                    reference = referenceLevel(info);
                    engine.load(nextInfo, all.filter(event => event && event.kind !== 'sfx'));
                    syncBed();
                    if (playing && needsSound() && openContext()) {
                        syncBed();
                        begin(last);
                    }
                },
                play(frame) {
                    engine.play(frame);
                    last = frame;
                    playing = true;
                    if (!needsSound() || !openContext()) return;
                    if (typeof ctx.resume === 'function') ctx.resume().catch(() => { /* the next play tries again */ });
                    syncBed();
                    begin(frame);
                },
                pause() {
                    engine.pause();
                    playing = false;
                    stopSfx();
                    if (bed) bed.element.pause();
                },
                seek(frame) {
                    engine.seek(frame);
                    last = frame;
                    if (!ctx) return;
                    if (playing) { begin(frame); return; }
                    stopSfx();
                    placeBed(frame);
                    applyDuck(frame);
                },
                sync(frame) {
                    engine.sync(frame);
                    last = frame;
                    if (!playing || !ctx) return;
                    if (Math.abs(ctx.currentTime - (anchor + frame / fps())) > DRIFT_S) begin(frame);
                    else scheduleWindow(frame);
                    placeBed(frame);
                    applyDuck(frame);
                },
                dispose() {
                    disposed = true;
                    engine.dispose();
                    stopSfx();
                    if (bed) {
                        bed.element.pause();
                        bed.element.removeAttribute('src');
                        bed.element.load();
                        bed.source.disconnect();
                        bed.gain.disconnect();
                        bed = null;
                    }
                    if (ctx && typeof ctx.close === 'function') ctx.close().catch(() => { /* gone */ });
                    ctx = null;
                }
            };
        };

        S.audioModel.speechFrames = speechFrames;
        S.audioModel.duckAt = duckAt;
        S.audioModel.infoFor = infoFor;
    })();
})();
