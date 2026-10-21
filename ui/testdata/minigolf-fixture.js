import { mountGame } from '/js/vendor/minigolf/game.esm.js';

const host = document.getElementById('golf-host');
const errors = [];
const assetRequests = [];
const originalFetch = window.fetch.bind(window);
const audioContexts = [];
let failCourseLoads = new URLSearchParams(location.search).has('fail-course');
const assetURL = value => `${value}?v=minigolf-browser-test`;
// foreignResources() reads the whole page session, so keep every resource entry.
performance.setResourceTimingBufferSize(2000);

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

// The real desktop sets the lang attribute; the game formats numbers from it.
document.documentElement.lang = 'en';
window.I18N = await (await originalFetch('/lang/desktop/en.json')).json();
window.tr = key => window.I18N[key] || key;
let game;
const mount = () => {
    game = mountGame(host, { t: window.tr, assetURL });
    return game;
};

async function decodeAudioSamples() {
    const manifest = await (await originalFetch(assetURL('/3d/minigolf/audio/manifest.json'))).json();
    const results = [];
    for (const track of manifest.music) {
        const bytes = await (await originalFetch(assetURL('/3d/minigolf/audio/' + track.file))).arrayBuffer();
        // Decode at the engine's sample rate; length 1 is enough for decodeAudioData.
        const buffer = await new OfflineAudioContext(2, 1, 44100).decodeAudioData(bytes);
        let squareSum = 0, peak = 0, fullScale = 0, count = 0, seam = 0;
        for (let channel = 0; channel < buffer.numberOfChannels; channel++) {
            const data = buffer.getChannelData(channel);
            for (const sample of data) {
                if (!Number.isFinite(sample)) throw new Error(`Non-finite PCM for ${track.id}`);
                const magnitude = Math.abs(sample);
                squareSum += sample * sample;
                peak = Math.max(peak, magnitude);
                if (magnitude >= 0.999) fullScale++;
                count++;
            }
            seam = Math.max(seam, Math.abs(data[data.length - 1] - data[0]));
        }
        results.push({
            id: track.id,
            seconds: buffer.duration,
            expectedSeconds: track.seconds,
            rms: Math.sqrt(squareSum / Math.max(1, count)),
            peak,
            fullScaleFraction: fullScale / Math.max(1, count),
            seam
        });
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

let storageFails = false;
const nativeSetItem = Storage.prototype.setItem;
Storage.prototype.setItem = function (...args) {
    if (storageFails) throw new DOMException('fixture quota', 'QuotaExceededError');
    return nativeSetItem.apply(this, args);
};

// Horizontal overflow anywhere, vertical overflow outside panels, panels that scroll sideways, text below 13 px.
function layoutIssues() {
    const root = host.querySelector('.mg-app'), box = root.getBoundingClientRect(), issues = [];
    const shown = element => element.getClientRects().length > 0 && getComputedStyle(element).visibility !== 'hidden';
    const name = element => element.tagName.toLowerCase() + (element.classList[0] ? '.' + element.classList[0] : '') + (element.dataset.action ? '[' + element.dataset.action + ']' : '');
    for (const element of root.querySelectorAll('button,input,select,canvas,.mg-chip,.mg-plaque,.mg-banner,.mg-toast,.mg-round-card,.mg-hole-tile,.mg-player-card,.mg-step')) {
        if (!shown(element) || element.closest('.mg-score-scroll')) continue;
        const rect = element.getBoundingClientRect();
        if (rect.left < box.left - 1 || rect.right > box.right + 1) issues.push('x ' + name(element));
        if (!element.closest('.mg-overlay') && (rect.top < box.top - 1 || rect.bottom > box.bottom + 1)) issues.push('y ' + name(element));
    }
    for (const panel of root.querySelectorAll('.mg-panel')) if (panel.scrollWidth > panel.clientWidth + 1) issues.push('panel scrolls sideways');
    for (const element of root.querySelectorAll('*')) {
        if (element.closest('.mg-announcer') || !shown(element)) continue;
        if ([...element.childNodes].some(node => node.nodeType === Node.TEXT_NODE && node.textContent.trim()) && parseFloat(getComputedStyle(element).fontSize) < 13) issues.push('font ' + name(element));
    }
    return [...new Set(issues)];
}

// A round.v2 save on the last hole of a world with both players finished.
async function seedFinalHole(mode = 'temple') {
    const manifest = await (await originalFetch(assetURL('/3d/minigolf/manifest.json'))).json();
    const courses = manifest.courses.filter(course => course.world === mode), last = courses.at(-1);
    const player = (name, color, score) => ({ name, color, scores: Array(courses.length - 1).fill(score), position: last.cup.slice(), shots: score, strokes: score, penalties: 0, done: true });
    localStorage.setItem('aurago.desktop.minigolf.round.v2', JSON.stringify({ version: 2, mode, courseIds: courses.map(course => course.id), holeIndex: courses.length - 1, players: [player('Ada', 'teal', 2), player('Lin', 'gold', 3)], turnIndex: 0, completed: false }));
}

mount();
window.golfFixture = {
    errors,
    assetRequests,
    inspect: () => game.inspect(),
    decodeAudioSamples,
    foreignResources: () => performance.getEntriesByType('resource').map(entry => entry.name).filter(name => new URL(name, location.href).origin !== location.origin),
    measureFrameIntervals,
    measureAudioPlaybackStats,
    measureActivePlayAudioStats,
    audioContexts,
    benchmarkCurrentScene,
    layoutIssues,
    seedFinalHole,
    failStorage(value) { storageFails = !!value; },
    reopen() { game?.dispose(); return mount(); },
    setCourseFailure(value) { failCourseLoads = !!value; },
    dispose() { game.dispose(); },
    host
};
