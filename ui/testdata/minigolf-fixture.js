import { mountGame } from '/js/vendor/minigolf/game.esm.js';

const host = document.getElementById('golf-host');
const errors = [];
const assetRequests = [];
const originalFetch = window.fetch.bind(window);
const audioContexts = [];
let failCourseLoads = new URLSearchParams(location.search).has('fail-course');
const assetURL = value => `${value}?v=minigolf-browser-test`;

// Chrome 146 exposes playback underrun counters on AudioContext. Wrap the
// constructor before mounting the app so this fixture can compare live-game
// output with offline-render work without adding hooks to production code.
const NativeAudioContext = window.AudioContext || window.webkitAudioContext;
if (NativeAudioContext) {
    const TrackedAudioContext = class extends NativeAudioContext {
        constructor(...args) {
            super(...args);
            audioContexts.push(this);
        }
    };
    window.AudioContext = TrackedAudioContext;
    if (window.webkitAudioContext === NativeAudioContext) window.webkitAudioContext = TrackedAudioContext;
}

window.addEventListener('error', event => errors.push(event.message || String(event.error)));
window.addEventListener('unhandledrejection', event => errors.push(String(event.reason)));
window.fetch = (input, init) => {
    const url = new URL(typeof input === 'string' ? input : input.url, location.href);
    assetRequests.push(url.pathname + url.search);
    if (failCourseLoads && /\/3d\/minigolf\/course-\d+\.glb$/.test(url.pathname)) {
        return Promise.resolve(new Response('fixture course failure', { status: 503 }));
    }
    return originalFetch(input, init);
};

if (new URLSearchParams(location.search).has('no-webgl')) {
    const getContext = HTMLCanvasElement.prototype.getContext;
    HTMLCanvasElement.prototype.getContext = function (kind, ...args) {
        if (/^webgl2?$|^experimental-webgl$/i.test(kind)) return null;
        return getContext.call(this, kind, ...args);
    };
}

window.I18N = await (await originalFetch('/lang/desktop/en.json')).json();
window.tr = key => window.I18N[key] || key;
let game;
const mount = () => {
    game = mountGame(host, { t: window.tr, assetURL });
    return game;
};

function shortMusicProject(project) {
    const clone = structuredClone(project);
    clone.loop = { enabled: false, start: 0, end: 0 };
    const endTick = Math.floor(5 * clone.tempo * clone.ppq / 60);
    clone.tracks = clone.tracks.map(track => ({
        ...track,
        mute: false,
        solo: false,
        volume: Math.min(0.65, Math.max(0.2, track.volume ?? 0.8)),
        clips: (track.clips || []).filter(clip => clip.start < endTick).map(clip => {
            const clipEnd = Math.max(1, endTick - clip.start);
            const notes = clip.notes.filter(note => note.start < clipEnd).map(note => ({
                ...note,
                duration: Math.max(1, Math.min(note.duration, clipEnd - note.start))
            }));
            return {
                ...clip,
                length: Math.min(clip.length || clipEnd, clipEnd),
                notes,
                controllers: (clip.controllers || []).filter(control => control.tick <= clipEnd)
            };
        })
    })).filter(track => track.clips.length);
    return clone;
}

async function renderMusicSamples() {
    const manifest = await (await originalFetch(assetURL('/3d/minigolf/music/manifest.json'))).json();
    const results = [];
    for (const track of manifest.tracks) {
        const project = await (await originalFetch(assetURL('/3d/minigolf/music/' + track.project))).json();
        const blob = await SynthStudioAudio.renderWav(shortMusicProject(project));
        const bytes = await blob.arrayBuffer();
        const view = new DataView(bytes);
        if (view.getUint32(0, false) !== 0x52494646 || view.getUint32(8, false) !== 0x57415645 || view.getUint32(40, true) !== bytes.byteLength - 44) {
            throw new Error(`Invalid offline WAV for ${track.id}`);
        }
        const samples = new Int16Array(bytes, 44);
        let squareSum = 0, peak = 0, fullScale = 0;
        for (const sample of samples) {
            const normalized = sample / 32768;
            if (!Number.isFinite(normalized)) throw new Error(`Non-finite PCM for ${track.id}`);
            const magnitude = Math.abs(normalized);
            squareSum += normalized * normalized;
            peak = Math.max(peak, magnitude);
            if (magnitude >= 0.999) fullScale++;
        }
        const seconds = view.getUint32(40, true) / (44100 * 2 * 2);
        const result = {
            id: track.id,
            seconds,
            rms: Math.sqrt(squareSum / Math.max(1, samples.length)),
            peak,
            fullScaleFraction: fullScale / Math.max(1, samples.length)
        };
        // renderWav retains its fixed release tail after this five-second note window.
        if (result.seconds < 4 || result.seconds > 10 || result.rms < 0.0005 || result.peak <= 0.01 || result.fullScaleFraction > 0.001) {
            throw new Error(`Offline render bounds failed for ${track.id}: ${JSON.stringify(result)}`);
        }
        if (new URLSearchParams(location.search).has('save-renders')) {
            const response = await fetch('/__minigolf-render/' + encodeURIComponent(track.id), { method: 'POST', body: blob });
            if (!response.ok) throw new Error(`Could not save rendered sample for ${track.id}`);
        }
        results.push(result);
    }
    return results;
}

async function measureFrameIntervals(frames = 240) {
    const intervals = [];
    let previous = 0;
    const startedAt = performance.now();
    await new Promise(resolve => {
        const sample = time => {
            if (previous !== 0) intervals.push(time - previous);
            previous = time;
            if (intervals.length >= frames) resolve();
            else requestAnimationFrame(sample);
        };
        requestAnimationFrame(sample);
    });
    const totalMs = performance.now() - startedAt;
    const ordered = [...intervals].sort((a, b) => a - b);
    const percentile = fraction => ordered[Math.min(ordered.length - 1, Math.floor((ordered.length - 1) * fraction))] || 0;
    return {
        frames: intervals.length,
        totalMs,
        fps: intervals.length * 1000 / Math.max(totalMs, 1),
        medianFrameMs: percentile(0.5),
        p95FrameMs: percentile(0.95)
    };
}

function snapshotAudioPlayback() {
    return audioContexts.map((context, index) => {
        let stats = null;
        try {
            const source = context.playbackStats ?? context.playoutStats;
            const value = typeof source === 'function' ? source.call(context) : source;
            if (value) {
                stats = Object.fromEntries([
                    'underrunEvents', 'underrunDuration', 'totalDuration',
                    'averageLatency', 'minimumLatency', 'maximumLatency'
                ].map(key => [key, Number.isFinite(value[key]) ? value[key] : null]));
            }
        } catch (error) {
            stats = { error: String(error) };
        }
        return {
            index,
            state: context.state,
            currentTime: context.currentTime,
            baseLatency: Number.isFinite(context.baseLatency) ? context.baseLatency : null,
            outputLatency: Number.isFinite(context.outputLatency) ? context.outputLatency : null,
            stats
        };
    });
}

async function measureAudioPlaybackStats(durationMs = 15000) {
    const before = snapshotAudioPlayback();
    await new Promise(resolve => setTimeout(resolve, durationMs));
    const after = snapshotAudioPlayback();
    const deltas = after.map((sample, index) => {
        const initial = before[index];
        const fields = ['underrunEvents', 'underrunDuration', 'totalDuration'];
        const statsDelta = {};
        for (const field of fields) {
            const from = initial?.stats?.[field];
            const to = sample.stats?.[field];
            statsDelta[field] = Number.isFinite(from) && Number.isFinite(to) ? to - from : null;
        }
        return { index: sample.index, state: sample.state, before: initial?.stats, after: sample.stats, delta: statsDelta };
    });
    return { durationMs, before, after, deltas, supported: after.some(sample => sample.stats && !sample.stats.error) };
}

async function measureActivePlayAudioStats(durationMs = 15000) {
    const before = snapshotAudioPlayback();
    const startedAt = performance.now();
    let acceptedShots = 0;
    let nextHoles = 0;
    while (performance.now() - startedAt < durationMs) {
        const state = game.inspect();
        if (state.panel === 'scorecard') {
            const next = host.querySelector('[data-action="next-hole"]');
            if (next && !next.disabled) {
                next.click();
                nextHoles++;
            }
        } else if (!state.loading && state.panel === '' && state.visible && !state.busy) {
            const shoot = host.querySelector('[data-action="shoot"]');
            if (shoot && !shoot.disabled) {
                // Trigger only the normal enabled DOM control; the game owns
                // shot acceptance, audio effects, physics, and score panels.
                shoot.click();
                if (game.inspect().busy) acceptedShots++;
            }
        }
        await new Promise(resolve => setTimeout(resolve, 50));
    }
    const after = snapshotAudioPlayback();
    const deltas = after.map((sample, index) => {
        const initial = before[index];
        const statsDelta = {};
        for (const field of ['underrunEvents', 'underrunDuration', 'totalDuration']) {
            const from = initial?.stats?.[field];
            const to = sample.stats?.[field];
            statsDelta[field] = Number.isFinite(from) && Number.isFinite(to) ? to - from : null;
        }
        return { index: sample.index, state: sample.state, before: initial?.stats, after: sample.stats, delta: statsDelta };
    });
    return {
        durationMs: performance.now() - startedAt,
        acceptedShots,
        nextHoles,
        playing: !!game.inspect().audio.playing,
        panel: game.inspect().panel,
        course: game.inspect().course,
        before,
        after,
        deltas,
        supported: after.some(sample => sample.stats && !sample.stats.error)
    };
}

async function benchmarkCurrentScene() {
    const canvas = host.querySelector('.mg-canvas canvas');
    const gl = canvas?.getContext('webgl2') || canvas?.getContext('webgl');
    if (!gl) throw new Error('Minigolf benchmark could not obtain the live WebGL context.');
    const debug = gl.getExtension('WEBGL_debug_renderer_info');
    const renderer = debug ? gl.getParameter(debug.UNMASKED_RENDERER_WEBGL) : gl.getParameter(gl.RENDERER);
    const timing = await measureFrameIntervals(240);
    const stats = game.inspect().graphics;
    return {
        renderer,
        width: innerWidth,
        height: innerHeight,
        ...timing,
        rendererFPS: stats?.fps || 0,
        triangles: stats?.triangles || 0,
        calls: stats?.calls || 0,
        geometries: stats?.geometries || 0,
        quality: stats?.quality
    };
}

function startIndependentPlayers() {
    const first = SynthStudioAudio.create();
    const second = SynthStudioAudio.create();
    const project = (id, pitch) => ({
        version: 1, tempo: 120, ppq: 480, loop: { enabled: false, start: 0, end: 0 },
        tracks: [{ id, name: id, instrument: 'lead-saw', volume: 0.35, pan: 0, mute: false, solo: false,
            params: { tone: 0.5, resonance: 0, drive: 0, attack: 0.01, release: 0.12, chorus: 0, phaser: 0, reverb: 0, delay: 0 },
            clips: [{ id: id + '-clip', start: 0, length: 4800, notes: [{ id: id + '-note', pitch, start: 0, duration: 4800, velocity: 0.5 }], controllers: [] }] }]
    });
    return (async () => {
        try {
            const ready = await Promise.all([first.unlock(), second.unlock()]);
            if (!ready.every(Boolean)) return { independent: false, reason: 'unlock' };
            const firstDone = first.play(project('minigolf-audio-a', 60));
            const secondDone = second.play(project('minigolf-audio-b', 67));
            await new Promise(resolve => setTimeout(resolve, 320));
            const beforeStop = second.currentTick();
            first.stop();
            await new Promise(resolve => setTimeout(resolve, 150));
            const afterStop = second.currentTick();
            second.stop();
            await Promise.all([firstDone, secondDone]);
            return { independent: beforeStop > 0 && afterStop > beforeStop, beforeStop, afterStop };
        } finally {
            first.dispose();
            second.dispose();
        }
    })();
}

mount();
window.golfFixture = {
    errors,
    assetRequests,
    inspect: () => game.inspect(),
    renderMusicSamples,
    measureFrameIntervals,
    measureAudioPlaybackStats,
    measureActivePlayAudioStats,
    audioContexts,
    benchmarkCurrentScene,
    armIndependentPlayerTest(button) {
        button.addEventListener('pointerdown', () => {
            this.independentPlayersPromise = startIndependentPlayers();
        }, { once: true });
    },
    reopen() { game?.dispose(); return mount(); },
    setCourseFailure(value) { failCourseLoads = !!value; },
    dispose() { game.dispose(); },
    host
};
