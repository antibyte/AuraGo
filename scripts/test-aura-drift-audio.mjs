// Aura Drift audio pack contract test.
//   node scripts/test-aura-drift-audio.mjs          full run (verifies the pack, decodes every file with ffmpeg)
//   node scripts/test-aura-drift-audio.mjs --fast   structure and budgets only (no decoding)
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const audioDir = path.join(root, 'ui/3d/aura-drift/audio');
const fast = process.argv.includes('--fast');
let failures = 0, checks = 0;
const ok = (cond, message) => { checks++; if (!cond) { failures++; console.error(`FAIL: ${message}`); } };

// ---- the contract T6.3 (audio runtime) codes against ----
const CARS = ['vektor', 'kobalt', 'lumen', 'tundra', 'softeis', 'maehkoenig', 'couch', 'bertha'];
const LAYERS = ['idle', 'low', 'mid', 'high'];
const BIOMES = ['alpine', 'desert', 'harbour', 'salt', 'forest', 'industry', 'glacier', 'metro'];
const STEMS = ['bed', 'drive', 'peak'];
export const AUDIO_GROUPS = [
    'impact.light', 'impact.heavy', 'impact.metal', 'scrape.metal', 'tyre.squeal', 'explosion.small', 'explosion.big', 'crate.break',
    'prop.hay', 'prop.cone', 'prop.barrel', 'prop.tyres', 'crowd.cheer', 'crowd.aww',
    'animal.cow', 'animal.sheep', 'animal.penguin', 'animal.llama', 'animal.crab',
    'amb.wind', 'amb.sea', 'amb.forest', 'amb.machinery', 'amb.city', 'amb.snow', 'amb.desert',
    'weapon.oil', 'weapon.mine_drop', 'weapon.emp_charge', 'weapon.emp_pulse', 'weapon.rocket_fire', 'weapon.rocket_lock', 'weapon.mg', 'weapon.shield_on', 'weapon.shield_hit',
    'weapon.duck_launch', 'weapon.duck_quack', 'weapon.rail_charge', 'weapon.rail_fire', 'weapon.flame_loop', 'weapon.frost_throw', 'weapon.frost_burst', 'weapon.horn_blast',
    'weapon.hole_loop', 'weapon.disco_on', 'weapon.glove', 'weapon.harpoon_fire', 'weapon.harpoon_clank', 'weapon.cow_launch', 'weapon.cow_land', 'weapon.cloak_on', 'weapon.overdrive_on',
    'sting.perfect', 'sting.pancake', 'sting.strike', 'sting.slingshot', 'sting.moo', 'sting.trick', 'turbo.1', 'turbo.2', 'turbo.3', 'turbo.tick', 'pad.boost', 'ramp.launch',
    'land.soft', 'land.hard', 'roulette.tick', 'roulette.done', 'bomb.tick', 'bomb.blast', 'jingle.softeis', ...CARS.map(id => `horn.${id}`), 'ufo.beam',
    'ui.count', 'ui.go', 'ui.lap', 'ui.bank', 'whoosh', 'stinger.final_lap', 'stinger.disco', 'fanfare.win', 'fanfare.podium', 'fanfare.lose'
];
const LIMITS = { sfx: 4 * 1024 * 1024, engine: 1.5 * 1024 * 1024, music: 10 * 1024 * 1024, manifest: 128 * 1024 };
const PATH_RE = /^(sfx|engine|music)\/[a-z0-9-]+\.ogg$/;

// ---- 1. the build script's own verification ----
const verify = spawnSync(process.execPath, [path.join(root, 'scripts/build-aura-drift-audio.mjs'), '--check'], { encoding: 'utf8' });
ok(verify.status === 0, `build-aura-drift-audio.mjs --check failed:\n${verify.stderr || verify.stdout}`);

const manifestFile = path.join(audioDir, 'manifest.json');
const raw = fs.readFileSync(manifestFile);
const manifest = JSON.parse(raw.toString('utf8'));
ok(raw.length <= LIMITS.manifest, `manifest is ${raw.length} bytes`);
ok(manifest.version === 1, 'manifest version');
const byPath = new Map(manifest.files.map(file => [file.path, file]));
ok(byPath.size === manifest.files.length, 'duplicate manifest paths');
for (const file of manifest.files) ok(PATH_RE.test(file.path), `bad path ${file.path}`);

// ---- 2. groups: manifest against the runtime contract ----
const groups = Object.keys(manifest.sfx);
const sorted = list => [...list].sort();
ok(JSON.stringify(sorted(groups)) === JSON.stringify(sorted(AUDIO_GROUPS)), `sfx groups differ from the contract: missing ${AUDIO_GROUPS.filter(g => !groups.includes(g))}, extra ${groups.filter(g => !AUDIO_GROUPS.includes(g))}`);
for (const [group, paths] of Object.entries(manifest.sfx)) {
    ok(Array.isArray(paths) && paths.length > 0, `${group}: no members`);
    for (const p of paths) ok(byPath.has(p) && p.startsWith('sfx/'), `${group}: ${p} is not an sfx file in the manifest`);
}
// AD.AUDIO_GROUPS in the runtime (T6.3) must list exactly the groups that exist; scanned by regex once the file declares it.
const runtime = path.join(root, 'ui/js/desktop/apps/aura-drift-audio.js');
if (fs.existsSync(runtime)) {
    const text = fs.readFileSync(runtime, 'utf8');
    const match = /AD\.AUDIO_GROUPS\s*=\s*\[([\s\S]*?)\]\s*;/.exec(text);
    if (match) {
        const declared = [...match[1].matchAll(/'([A-Za-z0-9._<>]+)'/g)].flatMap(m => m[1].includes('<carId>') ? CARS.map(id => m[1].replace('<carId>', id)) : [m[1]]);
        ok(JSON.stringify(sorted(declared)) === JSON.stringify(sorted(groups)), `AD.AUDIO_GROUPS differs from the manifest: missing ${groups.filter(g => !declared.includes(g))}, unknown ${declared.filter(g => !groups.includes(g))}`);
    } else console.log('note: aura-drift-audio.js does not declare AD.AUDIO_GROUPS yet; checked against the contract list only');
}

// ---- 3. engines ----
for (const car of CARS) {
    ok(manifest.engine[car], `engine ${car} missing`);
    for (const layer of LAYERS) {
        const p = manifest.engine[car]?.[layer], entry = byPath.get(p);
        ok(entry && p === `engine/${car}-${layer}.ogg`, `engine ${car}/${layer}: unexpected path ${p}`);
        ok(entry && Math.abs(entry.seconds - 2) <= 0.002 && entry.channels === 1 && entry.loop, `engine ${car}/${layer}: must be a 2 s mono loop (got ${entry?.seconds})`);
    }
}
ok(Object.keys(manifest.engine).length === CARS.length, 'engine map has extra cars');

// ---- 4. music: aligned stems at the live score tempi ----
const musicSource = fs.readFileSync(path.join(root, 'ui/js/desktop/apps/aura-drift-music.js'), 'utf8');
const tempoBlock = /const TEMPI = \{([\s\S]*?)\};/.exec(musicSource)?.[1] ?? '';
const TEMPI = Object.fromEntries([...tempoBlock.matchAll(/(\w+):\s*(\d+)/g)].map(m => [m[1], Number(m[2])]));
for (const biome of BIOMES) {
    ok(TEMPI[biome] > 0, `no tempo for ${biome} in aura-drift-music.js`);
    const stems = STEMS.map(stem => byPath.get(manifest.music[biome]?.[stem]));
    ok(stems.every(Boolean), `music ${biome}: bed, drive and peak are required`);
    if (!stems.every(Boolean)) continue;
    const spread = Math.max(...stems.map(s => s.seconds)) - Math.min(...stems.map(s => s.seconds));
    ok(spread <= 0.001, `music ${biome}: stem durations differ by ${(spread * 1000).toFixed(2)} ms`);
    const expected = 16 * 4 * 60 / TEMPI[biome];
    ok(Math.abs(stems[0].seconds - expected) <= 0.005, `music ${biome}: ${stems[0].seconds} s, expected ${expected.toFixed(3)} s for 16 bars at ${TEMPI[biome]} BPM`);
    ok(stems.every(s => s.channels === 2 && s.loop), `music ${biome}: stems must be stereo loops`);
}
for (const name of ['menu', 'results']) ok(byPath.has(manifest.music[name]), `music ${name} missing`);
ok(Object.keys(manifest.music).length === BIOMES.length + 2, 'music map has unexpected entries');

// ---- 5. budgets ----
const totals = { sfx: 0, engine: 0, music: 0 };
for (const file of manifest.files) {
    const stat = fs.statSync(path.join(audioDir, file.path));
    ok(stat.size === file.bytes, `${file.path}: size differs from the manifest`);
    totals[file.path.split('/')[0]] += stat.size;
}
for (const key of Object.keys(totals)) ok(totals[key] <= LIMITS[key], `${key} total ${totals[key]} exceeds ${LIMITS[key]}`);
ok(manifest.files.every(f => f.origin === 'own-synthesis' || (f.license === 'CC0-1.0' && manifest.sources.some(s => s.id === f.origin))), 'every file is own synthesis or a pinned CC0 source');

// ---- 6. decoded checks ----
function decode(file, channels) {
    const out = spawnSync('ffmpeg', ['-hide_banner', '-nostdin', '-loglevel', 'error', '-i', path.join(audioDir, file), '-f', 'f32le', '-ac', String(channels), '-ar', '44100', '-'], { maxBuffer: 512 * 1024 * 1024 });
    if (out.status !== 0) return null;
    const buf = out.stdout;
    return new Float32Array(buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.length));
}
const db = v => 20 * Math.log10(Math.max(v, 1e-9));
if (!fast) {
    const ffmpeg = spawnSync('ffmpeg', ['-version']);
    if (ffmpeg.status !== 0) console.log('note: ffmpeg not found, decoded checks skipped');
    else {
        for (const file of manifest.files) {
            const x = decode(file.path, file.channels);
            ok(x, `${file.path}: ffmpeg cannot decode it`);
            if (!x) continue;
            let peak = 0; for (let i = 0; i < x.length; i++) peak = Math.max(peak, Math.abs(x[i]));
            const kind = file.path.split('/')[0];
            if (kind === 'sfx') ok(db(peak) <= -0.3 && db(peak) >= -4, `${file.path}: peak ${db(peak).toFixed(2)} dBFS, expected about -1`);
            if (kind === 'engine') ok(db(peak) <= -1.8 && db(peak) >= -5, `${file.path}: peak ${db(peak).toFixed(2)} dBFS, expected about -3`);
            if (kind === 'music') ok(db(peak) <= -0.5, `${file.path}: peak ${db(peak).toFixed(2)} dBFS`);
            const lag = file.seconds - x.length / file.channels / 44100; // ffmpeg's Vorbis decoder drops up to 128 priming samples and may return a partial last block; browsers use the granule length, loops must still match
            ok(lag >= (file.loop ? -0.002 : -0.025) && lag <= 0.0035, `${file.path}: decoded length ${(x.length / file.channels / 44100).toFixed(4)} s differs from the manifest`);
        }
        for (const biome of BIOMES) { // the three stems played together must land at -16 LUFS
            const args = ['-hide_banner', '-nostats', '-nostdin', ...STEMS.flatMap(s => ['-i', path.join(audioDir, manifest.music[biome][s])]), '-filter_complex', 'amix=inputs=3:normalize=0,ebur128=framelog=quiet', '-f', 'null', '-'];
            const out = spawnSync('ffmpeg', args, { encoding: 'utf8' });
            const lufs = Number(/\bI:\s+(-?[\d.]+) LUFS/.exec(`${out.stderr}\n${out.stdout}`)?.[1]);
            ok(Number.isFinite(lufs) && lufs <= -14.5 && lufs >= -18, `music ${biome}: stems summed measure ${lufs} LUFS, expected about -16`);
        }
    }
}
// ---- 7. music runtime against a fake AudioContext (T6.4) ----
async function checkMusicRuntime() {
    const vm = await import('node:vm');
    const tick = () => new Promise(resolve => setTimeout(resolve, 5));
    const fakeParam = () => ({ value: 0, setValueAtTime() {}, linearRampToValueAtTime() {}, exponentialRampToValueAtTime() {}, setTargetAtTime() {}, cancelScheduledValues() {} });
    function makeCtx() {
        const ctx = { currentTime: 1, sources: [], decodeAudioData: bytes => Promise.resolve({ duration: 8, bytes }) };
        const node = extra => {
            const n = { connect(t) { return t; }, disconnect() {}, gain: fakeParam(), frequency: fakeParam(), detune: fakeParam(), Q: fakeParam(), ...extra };
            return n;
        };
        ctx.createGain = () => node();
        ctx.createBiquadFilter = () => node();
        ctx.createOscillator = () => node({ start() {}, stop() {} });
        ctx.createBufferSource = () => {
            const n = node({ loop: false, buffer: null, stopped: false, start(when) { n.startedAt = when; n.started = true; }, stop() { n.stopped = true; } });
            ctx.sources.push(n);
            return n;
        };
        return ctx;
    }
    function load(fetchOk) {
        const bytes = path => (fetchOk ? Promise.resolve(Object.assign(new ArrayBuffer(8), { path })) : Promise.reject(new Error('no pack')));
        const AD = {
            clamp: (v, lo, hi) => Math.min(hi, Math.max(lo, v)),
            assetsFetchBytes: bytes,
            assetsFetchJson: () => (fetchOk ? Promise.resolve(manifest) : Promise.reject(new Error('no pack')))
        };
        const sandbox = { window: { AuraDriftCore: AD }, setTimeout, clearTimeout, Math, Object, Promise };
        vm.runInNewContext(musicSource, sandbox, { filename: 'aura-drift-music.js' });
        return AD;
    }
    for (const withPack of [false, true]) {
        const label = withPack ? 'with pack' : 'without pack';
        const AD = load(withPack);
        const ctx = makeCtx();
        const music = AD.createMusicEngine(ctx, { fake: 'noise' }, ctx.createGain());
        let threw = null;
        try {
            music.setBiome('alpine');
            music.update(0.016, 0.9, { intensity: 0.5 });
            for (const type of ['pickup', 'lap', 'disco_on', 'final_lap', 'finish', 'boom', 'style_bank']) music.event({ type, car: 0 });
            music.duck(6, 0.4);
            music.setView('menu'); await tick();
            music.setView('results'); await tick();
            music.setView('race'); await tick();
            music.setBiome('metro'); await tick();
            music.update(0.016, 0.1, {}, { nitro: true });
            music.event({ type: 'finish', car: 0, position: 5 });
            await tick();
        } catch (error) { threw = error; }
        ok(!threw, `music ${label}: threw ${threw && threw.stack}`);
        if (withPack) {
            const metro = ctx.sources.filter(s => s.loop && s.started && s.buffer.bytes && /music\/metro-(bed|drive|peak)\.ogg/.test(s.buffer.bytes.path));
            ok(metro.length === 3 && new Set(metro.map(s => s.startedAt)).size === 1, `music with pack: the three metro stems must start together (got ${metro.length} sources at ${[...new Set(metro.map(s => s.startedAt))]})`);
            ok(ctx.sources.some(s => !s.loop && s.started), 'music with pack: no one-shot (stinger or fanfare) was played');
        }
        music.stop();
        music.stop();
        ok(ctx.sources.every(s => !s.started || s.stopped), `music ${label}: stop() left a source running`);
        music.setBiome('forest'); music.update(0.016, 0.5, {}); music.event({ type: 'finish', car: 0 });
    }
    // the sad trombone: a poor finish picks fanfare.lose, a win picks fanfare.win
    const picks = [];
    {
        const AD = load(true);
        const ctx = makeCtx();
        const bank = { groupBuffer: name => { picks.push(name); return Promise.resolve({ duration: 2 }); } };
        const music = AD.createMusicEngine(ctx, {}, ctx.createGain(), bank);
        music.event({ type: 'finish', car: 0, position: 6 });
        music.event({ type: 'finish', car: 0, position: 1 });
        music.event({ type: 'finish', car: 0, position: 3 });
        await tick();
        music.stop();
    }
    ok(picks.includes('fanfare.lose') && picks.includes('fanfare.win') && picks.includes('fanfare.podium'), `music fanfares by position: ${picks}`);
}
await checkMusicRuntime();

// ---- 7. runtime against a fake AudioContext: every catalogue event, with and without samples ----
{
    const vm = await import('node:vm');
    const appsDir = path.join(root, 'ui/js/desktop/apps');
    const EVENT_TYPES = ['count', 'go', 'perfect_start', 'wheelspin', 'lap', 'final_lap', 'sector', 'finish', 'position', 'respawn', 'wall_hit', 'car_hit', 'drift_start', 'drift_level', 'turbo_burst', 'style_bank',
        'near_miss', 'slingshot', 'takeoff', 'trick', 'landing', 'reverse_hero', 'j_turn', 'boost_pad', 'pickup', 'smash', 'animal_cross', 'animal_hit', 'strike', 'ufo_abduct', 'weapon', 'weapon_end', 'hit', 'shield',
        'shield_block', 'rocket_lock', 'emp_charge', 'emp_pulse', 'emp_hit', 'rail_charge', 'rail_fire', 'flame_start', 'flame_end', 'flame_hit', 'toast', 'frost_throw', 'frost_burst', 'horn', 'hole_open', 'hole_close',
        'disco_on', 'disco_off', 'glove', 'harpoon_fire', 'harpoon_attach', 'harpoon_break', 'cow_launch', 'cow_land', 'duck_launch', 'duck_hit', 'cloak_on', 'cloak_off', 'overdrive_on', 'overdrive_off',
        'elimination_warn', 'eliminated', 'bomb_given', 'bomb_pass', 'bomb_tick', 'bomb_blast', 'clock', 'sweeper', 'weather'];
    const WEAPONS = ['oil', 'gnome_mine', 'emp', 'rocket', 'mg', 'shield', 'ducks', 'railgun', 'flamethrower', 'frost', 'air_horn', 'black_hole', 'disco', 'glove', 'harpoon', 'cow', 'cloak', 'overdrive'];
    const param = (v = 0) => ({ value: v, setValueAtTime() { return this; }, linearRampToValueAtTime() { return this; }, exponentialRampToValueAtTime() { return this; }, setTargetAtTime() { return this; }, cancelScheduledValues() { return this; } });
    class FakeNode {
        constructor() { for (const k of ['gain', 'frequency', 'detune', 'Q', 'pan', 'playbackRate', 'threshold', 'knee', 'ratio', 'attack', 'release', 'delayTime']) this[k] = param(k === 'playbackRate' ? 1 : 0); this.type = 'x'; }
        connect(n) { return n; }
        disconnect() {}
        start() { this.started = true; }
        stop() { const f = this.onended; this.onended = null; if (f) queueMicrotask(() => f()); }
    }
    function makeSandbox(withSamples, failBytes) {
        const sb = { console, setTimeout, clearTimeout, navigator: {}, Math, Date, JSON, Float32Array, Promise, queueMicrotask };
        sb.window = sb;
        let created = 0;
        sb.AudioContext = class {
            constructor() { created++; this.currentTime = 0; this.sampleRate = 48000; this.state = 'running'; this.destination = new FakeNode(); }
            resume() { return Promise.resolve(); }
            suspend() { return Promise.resolve(); }
            close() { this.state = 'closed'; return Promise.resolve(); }
            createBuffer(ch, len, sr) { return { numberOfChannels: ch, length: len, sampleRate: sr, duration: len / sr, getChannelData: () => new Float32Array(Math.min(len, 64)) }; }
            decodeAudioData(bytes, ok) { const b = this.createBuffer(1, 96000, 48000); if (ok) ok(b); return Promise.resolve(b); }
            createBufferSource() { return new FakeNode(); }
            createOscillator() { return new FakeNode(); }
            createGain() { return new FakeNode(); }
            createBiquadFilter() { return new FakeNode(); }
            createDynamicsCompressor() { return new FakeNode(); }
            createConvolver() { return new FakeNode(); }
            createStereoPanner() { return new FakeNode(); }
            createDelay() { return new FakeNode(); }
            createWaveShaper() { return new FakeNode(); }
            createAnalyser() { return new FakeNode(); }
        };
        vm.createContext(sb);
        for (const f of ['core', 'audio-bank', 'engine', 'music', 'audio']) vm.runInContext(fs.readFileSync(path.join(appsDir, `aura-drift-${f}.js`), 'utf8'), sb, { filename: `aura-drift-${f}.js` });
        if (withSamples) {
            sb.AuraDriftCore.assets = {
                fetchJson: async () => manifest,
                fetchBytes: async (p) => { if (failBytes) throw new Error('offline'); return new ArrayBuffer(16); }
            };
        }
        return sb;
    }
    function fakeRace() {
        const cars = CARS.slice(0, 6).map((id, i) => ({ index: i, lap: 2, finished: false, v: { x: i * 30, y: i * 4, psi: 0, vx: 45, vy: 1, rpm: 3000 + i * 700, gear: 3, throttle: i % 2 ? 0 : 1, slip: 3.5, surface: 'asphalt', impact: 0.3, boost: i === 0 ? 1 : 0, airborne: false, slipstream: 0.2, car: { id } } }));
        return { cars, standings: cars.slice().reverse(), laps: 3, time: 5, conditions: { state: { ambience: { wind: 0.5, water: 0, machinery: 0 } } } };
    }
    function payloads(type) {
        const base = { type, t: 1, x: 12, y: 6, a: 0, b: 1, by: 1, target: 0, other: 1, value: 2, level: 2, lap: 2, time: 61, id: 'rocket', kind: 'cow', quality: 'crash', heavy: true, speed: 30, impulse: 15, hit: true, fuse: 3, seconds: 5, count: 3, multiplier: 3, chain: 2, index: 0, owner: 0, r: 8, points: 100, hits: 1 };
        const list = [];
        for (const car of [0, 2]) list.push({ ...base, car }, { ...base, car, x: 70 + car, y: 0, heavy: false, quality: 'perfect', kind: 'hay' });
        list.push({ ...base, car: 3, x: 500, y: 500 }, { type, t: 1 });
        if (type === 'weapon') for (const id of WEAPONS) list.push({ ...base, id, car: 0 });
        if (type === 'smash') for (const kind of ['hay', 'cone', 'barrel', 'tyre_stack', 'mystery']) list.push({ ...base, kind });
        if (type === 'animal_hit' || type === 'animal_cross') for (const kind of ['cow', 'sheep', 'penguin', 'llama', 'crab']) list.push({ ...base, kind });
        if (type === 'landing') for (const quality of ['perfect', 'good', 'wobble', 'crash']) list.push({ ...base, quality });
        if (type === 'turbo_burst') for (const level of [0, 1, 2, 3, 4]) list.push({ ...base, level });
        return list;
    }
    async function runPass(label, withSamples, failBytes) {
       
        const sb = makeSandbox(withSamples, failBytes);
        const AD = sb.AuraDriftCore;
        const audio = AD.createAudio({ volume: 0.8, muted: false });
        ok(audio.unlock() === true, `${label}: unlock`);
        audio.setBiome('harbour'); audio.setCar('softeis'); audio.setView('menu'); audio.setView('race');
        for (let i = 0; i < 20; i++) await new Promise(r => setImmediate(r));
        const race = fakeRace(), pose = { heading: 0.3 };
        let thrown = null;
        const guard = (what, fn) => { try { fn(); } catch (e) { thrown = thrown || `${what}: ${e && e.stack || e}`; } };
        for (let i = 0; i < 4; i++) guard('update', () => audio.update(race, true, pose));
        for (const type of EVENT_TYPES) for (const evt of payloads(type)) guard(`event ${type} ${JSON.stringify(evt)}`, () => audio.event(evt, race));
        guard('finish', () => audio.event({ type: 'finish', t: 1, car: 0, time: 80 }, race));
        guard('flame', () => { audio.event({ type: 'flame_start', car: 2 }, race); audio.update(race, true, pose); audio.event({ type: 'weapon_end', id: 'flamethrower', car: 2 }, race); audio.event({ type: 'hole_open', x: 5, y: 5, r: 9, owner: 0 }, race); audio.update(race, true, pose); audio.event({ type: 'hole_close', x: 5, y: 5 }, race); });
        for (let i = 0; i < 100; i++) guard('burst', () => audio.event({ type: 'hit', car: 0, x: 3, y: 1, heavy: true }, race));
        guard('horn', () => { audio.horn(); audio.horn(); });
        for (const k of ['confirm', 'back', 'move', 'start']) guard('ui', () => audio.ui(k));
        guard('suspend', () => { audio.suspend(); audio.resume(); audio.setSettings({ volume: 0.4, muted: true }); audio.event({ type: 'hit', car: 0 }, race); audio.setSettings({ volume: 0.4, muted: false }); });
        guard('inactive', () => audio.update(race, false, pose));
        for (let i = 0; i < 20; i++) await new Promise(r => setImmediate(r));
        guard('update2', () => audio.update(race, true, pose));
        const stats = audio.stats();
        guard('dispose', () => audio.dispose());
        guard('dispose2', () => audio.dispose());
        ok(!thrown, `${label}: ${thrown}`);
        return stats;
    }
    const none = await runPass('no samples', false, false);
    ok(none.samples === 0 && none.engine === 'osc', `no samples: expected the live engine, got ${JSON.stringify(none)}`);
    ok(none.fallback > 0, 'no samples: events must fall back to the live recipes');
    ok(none.voices <= 32, `no samples: voice cap exceeded (${none.voices})`);
    const full = await runPass('with samples', true, false);
    ok(full.samples > 100 && full.failed === 0, `with samples: expected the pack decoded, got ${JSON.stringify(full)}`);
    ok(full.engine === 'sample', `with samples: expected the sample engine, got ${full.engine}`);
    ok(full.voices <= 32, `with samples: voice cap exceeded (${full.voices})`);
    const broken = await runPass('failed fetches', true, true);
    ok(broken.failed > 0 && broken.engine === 'osc', `failed fetches: expected counted failures and the live engine, got ${JSON.stringify(broken)}`);
    const sbw = makeSandbox(false); const W = sbw.AuraDriftCore.engineWeights;
    for (const rpm of [500, 1000, 1800, 2600, 3500, 4600, 5600, 6600, 7200]) { const w = W(rpm); ok(Math.abs(w.reduce((s, x) => s + x * x, 0) - 1) < 1e-9, `engine weights at ${rpm} rpm are not equal power`); }
    const gainMax = Math.max(...Object.values(sbw.AuraDriftCore.GROUP_GAIN_DB));
    ok(gainMax <= 4, `group trims boost by ${gainMax} dB, expected at most +4`);
    ok(groups.every(g => sbw.AuraDriftCore.GROUP_GAIN_DB[g] !== undefined && sbw.AuraDriftCore.SYNTH_FALLBACK[g] !== undefined || /^amb\.|^(scrape\.metal|tyre\.squeal|weapon\.(flame|hole)_loop)$/.test(g)), 'every one-shot group needs a trim and a live fallback');
}
console.log(`${checks - failures}/${checks} checks passed`);
if (failures) { console.error(`${failures} failure(s)`); process.exit(1); }
