// AuraPinball offline audio renderer (opt-in dev tool, see ui/desktop_pinball_audio_render_browser_test.go).
// Renders the game's real synthesized audio (engine buses + limiter + reverb, SFX recipes layered over the CC0 samples,
// ambience beds, procedural music, a scripted gameplay mix) into OfflineAudioContext buffers and hands them out as WAV.
// Determinism: the engine and music code are driven by a virtual clock (ctx.currentTime), a virtual setTimeout queue and
// a fake music timer, so everything is scheduled ahead of startRendering().
// API: window.pinballAudioNames() -> [{name, group, seconds}], window.renderPinballAudio(filter?) -> [{name, group, seconds, peak, rms, nan, wavBase64}]
// filter: undefined | string (RegExp source tested against the name) | array of exact names.
import { createEngine, volumeGain } from '/js/desktop/apps/pinball/audio/engine.js';
import { createSfx, soundFor, cueSoundFor } from '/js/desktop/apps/pinball/audio/sfx.js';
import { createMusic, stingerLength } from '/js/desktop/apps/pinball/audio/music.js';
import { RECIPES, RECIPE_NAMES } from '/js/desktop/apps/pinball/audio/recipes.js';
import neon from '/js/desktop/apps/pinball/audio/songs/neon.js';
import leviathan from '/js/desktop/apps/pinball/audio/songs/leviathan.js';
import temple from '/js/desktop/apps/pinball/audio/songs/temple.js';
import { mixScript } from '/testdata/pinball-audio-mix.js';

const RATE = 44100, STEP = 0.025;
const SONGS = { neon, leviathan, temple };
const TABLES = Object.keys(SONGS);
const VOLUMES = { effects: 0.65, music: 0.5, ambience: 0.3 };
const SAMPLE_COUNT = 13;

// ---- shared state of the running job ----
const clock = { now: 0 };
const queue = [];          // virtual timeouts and scripted actions: {at (s), fn, id}
const musicTimers = [];    // fake setInterval handlers of the music schedulers
let idCounter = 0;
const sampleCache = new Map();

const fetchImpl = async url => {
    if (!sampleCache.has(url)) {
        const r = await fetch(url);
        if (!r.ok) throw Error('sample ' + url);
        sampleCache.set(url, await r.arrayBuffer());
    }
    const data = sampleCache.get(url);
    return { ok: true, arrayBuffer: async () => data.slice(0) };
};

function at(t, fn) { queue.push({ at: t, fn, id: ++idCounter }); }
const musicTimer = {
    start(fn) { musicTimers.push(fn); return fn; },
    stop(h) { const i = musicTimers.indexOf(h); if (i >= 0) musicTimers.splice(i, 1); }
};

function drive(total) {
    for (let T = 0; T <= total + 1e-9; T += STEP) {
        for (;;) {
            queue.sort((a, b) => a.at - b.at || a.id - b.id);
            if (!queue.length || queue[0].at > T) break;
            const e = queue.shift();
            clock.now = Math.max(clock.now, e.at);
            e.fn();
        }
        clock.now = Math.max(clock.now, T);
        for (const f of musicTimers.slice()) f();
    }
}

function wrapContext(real) {
    return new Proxy(real, {
        get(t, k) {
            if (k === 'currentTime') return clock.now;
            if (k === 'state') return 'running';
            const v = t[k];
            return typeof v === 'function' ? v.bind(t) : v;
        },
        set(t, k, v) { t[k] = v; return true; }
    });
}

// Runs one job: build the context + engine, let `build` schedule things, drive the virtual clock, render.
async function runJob(spec) {
    const length = Math.ceil(spec.seconds * RATE);
    const offline = new OfflineAudioContext(2, length, RATE);
    const proxy = wrapContext(offline);
    class FakeAudioContext { constructor() { return proxy; } }
    clock.now = 0; queue.length = 0; musicTimers.length = 0;
    const engine = createEngine({ tableId: spec.table || 'neon', assetURL: p => p, settings: { ...VOLUMES }, AudioContextCtor: FakeAudioContext, fetchImpl });
    if (!(await engine.unlock())) throw Error('engine did not unlock');
    await engine.loadSamples();
    const samples = engine.inspect().buffers;
    if (samples < SAMPLE_COUNT) throw Error(`only ${samples}/${SAMPLE_COUNT} samples loaded`);
    // The live engine eases the bus gains in with setTargetAtTime; offline renders start at t=0, so set them directly.
    for (const key of Object.keys(VOLUMES)) {
        const g = engine.buses[key].gain;
        g.cancelScheduledValues(0); g.setValueAtTime(volumeGain(VOLUMES[key]), 0);
    }
    const realSet = globalThis.setTimeout, realClear = globalThis.clearTimeout;
    globalThis.setTimeout = (fn, ms) => { const e = { at: clock.now + (ms || 0) / 1000, fn, id: ++idCounter }; queue.push(e); return e.id; };
    globalThis.clearTimeout = id => { const i = queue.findIndex(e => e.id === id); if (i >= 0) queue.splice(i, 1); };
    try {
        const sfx = createSfx(engine);
        spec.build({ engine, sfx, ctx: proxy, at });
        drive(spec.seconds);
    } finally { globalThis.setTimeout = realSet; globalThis.clearTimeout = realClear; }
    const buffer = await offline.startRendering();
    return buffer;
}

function analyse(buffer) {
    let peak = 0, sum = 0, n = 0, nan = 0;
    for (let c = 0; c < buffer.numberOfChannels; c++) {
        const d = buffer.getChannelData(c);
        for (let i = 0; i < d.length; i++) {
            const v = d[i];
            if (!Number.isFinite(v)) { nan++; continue; }
            const a = Math.abs(v);
            if (a > peak) peak = a;
            sum += v * v; n++;
        }
    }
    return { peak, rms: n ? Math.sqrt(sum / n) : 0, nan };
}

function encodeWav(buffer) {
    const ch = buffer.numberOfChannels, len = buffer.length, bytes = len * ch * 2;
    const out = new DataView(new ArrayBuffer(44 + bytes));
    const str = (o, s) => { for (let i = 0; i < s.length; i++) out.setUint8(o + i, s.charCodeAt(i)); };
    str(0, 'RIFF'); out.setUint32(4, 36 + bytes, true); str(8, 'WAVE'); str(12, 'fmt ');
    out.setUint32(16, 16, true); out.setUint16(20, 1, true); out.setUint16(22, ch, true); out.setUint32(24, buffer.sampleRate, true);
    out.setUint32(28, buffer.sampleRate * ch * 2, true); out.setUint16(32, ch * 2, true); out.setUint16(34, 16, true);
    str(36, 'data'); out.setUint32(40, bytes, true);
    const chans = [];
    for (let c = 0; c < ch; c++) chans.push(buffer.getChannelData(c));
    let o = 44;
    for (let i = 0; i < len; i++) {
        for (let c = 0; c < ch; c++) {
            const v = Math.max(-1, Math.min(1, Number.isFinite(chans[c][i]) ? chans[c][i] : 0));
            out.setInt16(o, v < 0 ? v * 0x8000 : v * 0x7fff, true); o += 2;
        }
    }
    return out.buffer;
}

function toBase64(arrayBuffer) {
    const bytes = new Uint8Array(arrayBuffer);
    let bin = '';
    for (let i = 0; i < bytes.length; i += 0x8000) bin += String.fromCharCode.apply(null, bytes.subarray(i, i + 0x8000));
    return btoa(bin);
}

// ---- catalogue ----
const snd = (recipe, gain = 0.9, rate = 1, pan = 0) => ({ recipe, gain, pan, rate });
const ev = e => soundFor(e);
const cueS = name => cueSoundFor(name);

// recipe -> the sound the game would play for a typical event
const SFX_CASES = {
    flipperUp: ev({ type: 'flipper', up: true, id: 'flipperL' }), flipperDown: ev({ type: 'flipper', up: false, id: 'flipperL' }),
    bumper: ev({ type: 'bumper', id: 'pop1' }), sling: ev({ type: 'sling', id: 'slingL' }), drop: ev({ type: 'drop', id: 'drop1' }),
    bankReset: ev({ type: 'bankReset', id: 'bank1' }), standup: ev({ type: 'standup', id: 'st1' }), rollover: ev({ type: 'rollover', id: 'lane1' }),
    gate: ev({ type: 'gate', id: 'gate1' }), scoop: ev({ type: 'hole', id: 'scoop1' }), eject: ev({ type: 'eject', id: 'scoop1' }),
    kickback: ev({ type: 'kickback', id: 'kb' }), knocker: cueS('knocker'), magnetOn: ev({ type: 'magnet', state: 'grab', id: 'm1' }),
    plungerRelease: ev({ type: 'launch', speed: 8 }), clack: ev({ type: 'clack', speed: 2 }),
    impactWood: ev({ type: 'impact', material: 'wood', speed: 1.5 }), impactMetal: ev({ type: 'impact', material: 'metal', speed: 1.5 }),
    impactPlastic: ev({ type: 'impact', material: 'plastic', speed: 1.5 }), impactRubber: ev({ type: 'impact', material: 'rubber', speed: 1.5 }),
    impactFlipper: ev({ type: 'impact', material: 'flipper', speed: 1.5 }), impactTarget: ev({ type: 'impact', material: 'target', speed: 1.5 }),
    impactBall: ev({ type: 'impact', material: 'ball', speed: 1.5 }), drain: ev({ type: 'drain' }),
    tiltBuzzer: cueS('tilt'), tiltWarnBuzz: cueS('tiltWarning'), bell: cueS('modeComplete'), bellHigh: cueS('lockLit'),
    jackpotHit: cueS('jackpot'), lockThunk: cueS('lock'), riser: cueS('multiball'), descend: cueS('gameOver'), tick: cueS('bonusTick'), bonusTotal: cueS('bonusTotal')
};
const SEQUENCES = { spinnerTick: 1.2, plungerRatchet: 1.6 }; // recipes rendered as a sequence instead of a single hit

function sequenceBuild(name) {
    if (name === 'spinnerTick') {
        return ({ sfx, at: sched }) => {
            let t = 0.05;
            for (let i = 0; i < 20; i++) {
                const k = i / 19;
                sched(t, () => sfx.play(snd('spinnerTick', 0.6 - 0.35 * k, 1.25 - 0.33 * k)));
                t += 0.035 + 0.02 * k * k * 3;
            }
        };
    }
    return ({ sfx, at: sched }) => { // plunger pulled over ~1.3 s: one ratchet click per 1/12 of travel, then released
        for (let i = 1; i <= 12; i++) sched(0.05 + i * 0.1, () => sfx.plunger(i / 12));
        sched(1.5, () => sfx.plunger(0));
    };
}

function catalog() {
    const jobs = [];
    const add = (name, group, seconds, build, table) => jobs.push({ name, group, seconds, build, table });
    const seen = new Set();
    for (const [recipe, s] of Object.entries(SFX_CASES)) {
        if (!RECIPES[recipe] || !s) continue;
        seen.add(recipe);
        const seconds = Math.min(2.5, Math.max(0.5, RECIPES[recipe].dur + 0.8));
        add('sfx-' + recipe, 'sfx', seconds, ({ sfx, at: sched }) => sched(0.02, () => sfx.play(s)));
    }
    for (const [recipe, seconds] of Object.entries(SEQUENCES)) { seen.add(recipe); add('sfx-' + recipe + '-seq', 'sfx', seconds + 0.6, sequenceBuild(recipe)); }
    for (const recipe of RECIPE_NAMES) { // anything a future recipe adds still gets rendered
        if (seen.has(recipe)) continue;
        add('sfx-' + recipe, 'sfx', Math.min(2.5, RECIPES[recipe].dur + 0.8), ({ sfx, at: sched }) => sched(0.02, () => sfx.play(snd(recipe))));
    }
    add('sfx-magnetHum', 'sfx', 2.5, ({ sfx, at: sched }) => { sched(0.1, () => sfx.event({ type: 'magnet', state: 'grab', id: 'm1' })); sched(1.8, () => sfx.event({ type: 'magnet', state: 'release', id: 'm1' })); });
    const balls = { field: { mode: 'free', speed: 1.5, x: 0 }, plastic: { mode: 'path', pathKind: 'ramp', speed: 1.5, x: 0 }, wire: { mode: 'path', pathKind: 'wire', speed: 1.5, x: 0 }, tube: { mode: 'path', pathKind: 'tube', speed: 1.5, x: 0 } };
    for (const [kind, ball] of Object.entries(balls)) {
        add('roll-' + kind, 'roll', 2.3, ({ sfx, at: sched }) => { for (let t = 0; t < 2; t += 0.05) sched(t, () => sfx.roll([ball])); sched(2, () => sfx.roll([])); });
    }
    for (const table of TABLES) add(`ambience-${table}`, 'ambience', table === 'leviathan' ? 16 : 8, ({ engine }) => engine.startAmbience(table), table);
    for (const table of TABLES) {
        const song = SONGS[table];
        for (const section of ['main', 'multiball', 'wizard', 'attract']) {
            add(`music-${table}-${section}`, 'music', 16.3, ({ ctx, engine }) => { createMusic(ctx, engine.buses.music, song, { timer: musicTimer }).section(section); }, table);
        }
        const stepSec = 60 / song.bpm / 4;
        for (const cue of ['jackpot', 'multiball', 'extraBall', 'drain', 'gameOver', 'highScore']) {
            if (!song.stingers[cue]) continue;
            const seconds = Math.min(10, stingerLength(song, cue) * stepSec + 2.5);
            add(`stinger-${table}-${cue}`, 'stinger', seconds, ({ ctx, engine, sfx, at: sched }) => {
                const music = createMusic(ctx, engine.buses.music, song, { timer: musicTimer });
                sched(0.2, () => { sfx.cue(cue); music.stinger(cue); });
            }, table);
        }
        add(`mix-${table}`, 'mix', 20, ({ ctx, engine, sfx, at: sched }) => {
            const music = createMusic(ctx, engine.buses.music, song, { timer: musicTimer });
            engine.startAmbience(table);
            music.section('main');
            mixScript({ sched, sfx, music });
        }, table);
    }
    return jobs;
}

function select(jobs, filter) {
    if (filter === undefined || filter === null || filter === '') return jobs;
    if (Array.isArray(filter)) return jobs.filter(j => filter.includes(j.name));
    const re = new RegExp(String(filter));
    return jobs.filter(j => re.test(j.name));
}

window.pinballAudioNames = () => catalog().map(j => ({ name: j.name, group: j.group, seconds: j.seconds }));
window.renderPinballAudio = async filter => {
    const out = [];
    for (const job of select(catalog(), filter)) {
        const buffer = await runJob(job);
        const stats = analyse(buffer);
        out.push({ name: job.name, group: job.group, seconds: buffer.duration, peak: stats.peak, rms: stats.rms, nan: stats.nan, wavBase64: toBase64(encodeWav(buffer)) });
    }
    return out;
};
window.pinballAudioRenderReady = true;
