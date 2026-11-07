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
    'ui.count', 'ui.go', 'ui.lap', 'ui.bank', 'whoosh', 'stinger.final_lap', 'stinger.disco', 'fanfare.win', 'fanfare.podium', 'fanfare.lose',
    'sting.near_miss', 'sting.style_bank', 'sting.drift_link', 'sting.cloak', 'sting.sector',
    'weather.thunder', 'weather.gust', 'weather.splash', 'weather.shift'
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
// Every biome plays ONE generated song (YuE2 Studio): a single stereo looping file with the loop crossfade baked in.
// (The manifest format still allows a biome with three aligned stems; the runtime test below covers that shape with a synthetic manifest.)
const YUE_ORIGIN = 'YuE2 Studio (local generation)';
const songRecord = JSON.parse(fs.readFileSync(path.join(root, 'assets/aura-drift/audio/yue2-songs.json'), 'utf8'));
ok(songRecord.origin === YUE_ORIGIN && songRecord.license === 'unverified', 'yue2-songs.json: origin and the unverified licence must be recorded');
ok(/NOT been verified/.test(songRecord.note) && /NOT been verified/.test(fs.readFileSync(path.join(audioDir, 'LICENSE.txt'), 'utf8')), 'the unverified model licence must be stated in yue2-songs.json and LICENSE.txt');
const songFiles = [];
for (const biome of BIOMES) {
    ok(TEMPI[biome] > 0, `no tempo for ${biome} in aura-drift-music.js`);
    const entry = byPath.get(manifest.music[biome]), want = `music/${biome}-song.ogg`;
    ok(manifest.music[biome] === want && entry, `music ${biome}: expected the song entry "${want}", got ${JSON.stringify(manifest.music[biome])}`);
    ok(!manifest.files.some(f => new RegExp(`^music/${biome}-(bed|drive|peak)\\.ogg$`).test(f.path)), `music ${biome}: stems must be removed once the biome has a song`);
    if (!entry) continue;
    songFiles.push(entry);
    ok(entry.channels === 2 && entry.loop === true, `music ${biome}: the song must be a stereo loop`);
    ok(entry.origin === YUE_ORIGIN && entry.license === 'unverified', `music ${biome}: origin / license must be ${YUE_ORIGIN} / unverified`);
    ok(entry.seconds >= 40 && entry.seconds <= 125, `music ${biome}: ${entry.seconds} s is not a plausible song length`);
    ok(entry.title && entry.style && entry.bpm === TEMPI[biome] && /^[0-9a-f]{64}$/.test(entry.sourceSha256), `music ${biome}: the generation prompt (title, style, BPM) and the master SHA-256 must be recorded`);
    ok(entry.bpm === songRecord.songs[biome]?.bpm && entry.sourceSha256 === songRecord.songs[biome]?.sourceSha256 && entry.style === songRecord.songs[biome]?.style, `music ${biome}: manifest and yue2-songs.json disagree`);
    ok(entry.loopFade === 3.5 && entry.kbps >= 64 && Math.abs(entry.seconds - (entry.sourceSeconds - entry.loopFade)) <= 1, `music ${biome}: loop crossfade not recorded or the length does not fit master minus fade`);
    ok(entry.lufs >= -17 && entry.lufs <= -15.5 && entry.peakDb <= -1 + 0.15 && Number.isFinite(entry.gainDb), `music ${biome}: recorded level ${entry.lufs} LUFS / ${entry.peakDb} dB / gain ${entry.gainDb} dB`);
}
for (const name of ['menu', 'results']) ok(byPath.has(manifest.music[name]), `music ${name} missing`);
ok(Object.keys(manifest.music).length === BIOMES.length + 2, 'music map has unexpected entries');
ok(Object.keys(songRecord.songs).length === BIOMES.length, 'yue2-songs.json must describe one song per biome');

// ---- 5. budgets ----
const totals = { sfx: 0, engine: 0, music: 0 };
for (const file of manifest.files) {
    const stat = fs.statSync(path.join(audioDir, file.path));
    ok(stat.size === file.bytes, `${file.path}: size differs from the manifest`);
    totals[file.path.split('/')[0]] += stat.size;
}
for (const key of Object.keys(totals)) ok(totals[key] <= LIMITS[key], `${key} total ${totals[key]} exceeds ${LIMITS[key]}`);
ok(manifest.files.every(f => f.origin === 'own-synthesis' || (f.license === 'CC0-1.0' && manifest.sources.some(s => s.id === f.origin)) || (f.origin === YUE_ORIGIN && f.license === 'unverified' && /^music\/[a-z]+-song\.ogg$/.test(f.path))), 'every file is own synthesis, a pinned CC0 source or one of the (licence-unverified) YuE2 race songs');

// ---- 6. decoded checks ----
function decode(file, channels, rate = 44100) {
    const out = spawnSync('ffmpeg', ['-hide_banner', '-nostdin', '-loglevel', 'error', '-i', path.join(audioDir, file), '-f', 'f32le', '-ac', String(channels), '-ar', String(rate), '-'], { maxBuffer: 512 * 1024 * 1024 });
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
        for (const biome of BIOMES) { // each song lands at about -16 LUFS (peak-limited ones a little lower) and its loop wraps without a click
            const file = path.join(audioDir, manifest.music[biome]);
            const out = spawnSync('ffmpeg', ['-hide_banner', '-nostats', '-nostdin', '-i', file, '-af', 'ebur128=framelog=quiet', '-f', 'null', '-'], { encoding: 'utf8' });
            const lufs = Number(/\bI:\s+(-?[\d.]+) LUFS/.exec(`${out.stderr}\n${out.stdout}`)?.[1]);
            ok(Number.isFinite(lufs) && lufs <= -15 && lufs >= -17.2, `music ${biome}: song measures ${lufs} LUFS, expected about -16`);
            const x = decode(manifest.music[biome], 2, 48000); // native rate: a resampler would add its own edge artefacts at the wrap point
            if (!x) continue;
            const n = x.length / 2;
            for (let c = 0; c < 2; c++) {
                let d2 = 0;
                for (let i = 1; i < n; i++) { const d = x[2 * i + c] - x[2 * (i - 1) + c]; d2 += d * d; }
                const step = Math.sqrt(d2 / n) || 1e-9, wrap = Math.abs(x[c] - x[2 * (n - 1) + c]);
                let seam = 0; // largest sample step within 200 samples either side of the wrap point
                for (let k = -200; k < 200; k++) { const i = (n + k) % n, j = (i + n - 1) % n; seam = Math.max(seam, Math.abs(x[2 * i + c] - x[2 * j + c])); }
                ok(wrap / step <= 2, `music ${biome} ch${c}: first and last sample differ by ${(wrap / step).toFixed(2)}x a typical step (loop click)`);
                ok(seam / step <= 5, `music ${biome} ch${c}: largest step at the loop seam is ${(seam / step).toFixed(2)}x a typical step (click)`);
            }
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
    function load(fetchOk, { pack = manifest, failBytes = null } = {}) {
        const bytes = path => (fetchOk && !(failBytes && failBytes.test(path)) ? Promise.resolve(Object.assign(new ArrayBuffer(8), { path })) : Promise.reject(new Error('no pack')));
        const AD = {
            clamp: (v, lo, hi) => Math.min(hi, Math.max(lo, v)),
            assetsFetchBytes: bytes,
            assetsFetchJson: () => (fetchOk ? Promise.resolve(pack) : Promise.reject(new Error('no pack')))
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
            const musicPaths = ctx.sources.filter(s => s.loop && s.started && s.buffer.bytes && /music\//.test(s.buffer.bytes.path)).map(s => s.buffer.bytes.path);
            const metro = ctx.sources.filter(s => s.loop && s.started && s.buffer.bytes && /music\/metro-song\.ogg/.test(s.buffer.bytes.path));
            ok(metro.length === 1, `music with pack: the metro song must play as one looping source (got ${metro.length}; loops started: ${musicPaths})`);
            ok(musicPaths.every(p => /\/music\/([a-z]+-song|menu|results)\.ogg$/.test(p)), `music with pack: only song, menu and results files may play (got ${musicPaths})`);
            ok(ctx.sources.some(s => !s.loop && s.started), 'music with pack: no one-shot (stinger or fanfare) was played');
        }
        music.stop();
        music.stop();
        ok(ctx.sources.every(s => !s.started || s.stopped), `music ${label}: stop() left a source running`);
        music.setBiome('forest'); music.update(0.016, 0.5, {}); music.event({ type: 'finish', car: 0 });
    }
    // a song that fails to load leaves the live synthesiser in charge (no music file plays, nothing throws)
    {
        const AD = load(true, { failBytes: /music\/forest-song\.ogg/ });
        const ctx = makeCtx();
        const music = AD.createMusicEngine(ctx, { fake: 'noise' }, ctx.createGain());
        let threw = null;
        try { music.setBiome('forest'); await tick(); music.update(0.016, 0.9, { intensity: 0.5 }); music.event({ type: 'final_lap', car: 0 }); await tick(); } catch (error) { threw = error; }
        ok(!threw, `music song load failure: threw ${threw && threw.stack}`);
        ok(!ctx.sources.some(s => s.loop && s.started && s.buffer && s.buffer.bytes && /music\//.test(s.buffer.bytes.path)), 'music song load failure: no music file may play');
        ok(ctx.sources.some(s => s.started), 'music song load failure: the live fallback (ambience beds) must be running');
        music.stop();
    }
    // a biome that keeps three aligned stems (the format still allows it) stays adaptive: three sources start together, the song shape never layers
    {
        const stemPack = { ...manifest, music: { ...manifest.music, alpine: Object.fromEntries(STEMS.map(s => [s, `music/alpine-${s}.ogg`])) } };
        const AD = load(true, { pack: stemPack });
        const ctx = makeCtx();
        const music = AD.createMusicEngine(ctx, { fake: 'noise' }, ctx.createGain());
        let threw = null;
        try {
            music.setBiome('alpine'); await tick();
            music.update(0.016, 0.9, { intensity: 0.5 }, { nitro: true }); music.event({ type: 'disco_on', car: 0 });
            music.setBiome('metro'); await tick(); music.update(0.4, 0.9, {}, { nitro: true }); music.event({ type: 'disco_on', car: 0 });
        } catch (error) { threw = error; }
        ok(!threw, `music stems and songs side by side: threw ${threw && threw.stack}`);
        const stems = ctx.sources.filter(s => s.loop && s.started && s.buffer.bytes && /music\/alpine-(bed|drive|peak)\.ogg/.test(s.buffer.bytes.path));
        ok(stems.length === 3 && new Set(stems.map(s => s.startedAt)).size === 1, `music stems: the three alpine stems must start together (got ${stems.length})`);
        ok(ctx.sources.filter(s => s.loop && s.started && s.buffer.bytes && /music\/metro-song\.ogg/.test(s.buffer.bytes.path)).length === 1, 'music stems and songs side by side: the metro song must be one source');
        music.stop();
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
    {
        const sb = makeSandbox(true, false), AD = sb.AuraDriftCore;
        const started = [], release = [];
        AD.assets.fetchBytes = p => { started.push(p); return new Promise(resolve => release.push(resolve)); };
        const ctx = new sb.AudioContext(), bank = AD.createSampleBank(ctx, ctx.destination);
        await bank.ready();
        bank.load(['impact.light', 'impact.heavy'], 1);
        await new Promise(resolve => setImmediate(resolve));
        const occupied = started.length;
        ok(occupied > 0 && bank.stats().queued > 0, 'music priority: effect preloads must fill the queue');
        bank.musicBuffer('music/alpine-song.ogg');
        release.shift()(new ArrayBuffer(16));
        await new Promise(resolve => setImmediate(resolve));
        ok(started[occupied]?.endsWith('/music/alpine-song.ogg'), 'requested music must load before the remaining effect preloads');
        bank.dispose();
        for (const resolve of release) resolve(new ArrayBuffer(16));
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
        if (withSamples) {
            guard('stopWorld', () => {
                audio.event({ type: 'flame_start', car: 0, x: 3, y: 1 }, race);
                audio.event({ type: 'hole_open', x: 5, y: 2 }, race);
                audio.update(race, true, pose);
                const before = audio.worldLoopCount();
                audio.stopWorld();
                ok(before >= 1, `${label}: expected live world loops before stopWorld, got ${before}`);
                ok(audio.worldLoopCount() === 0, `${label}: stopWorld left ${audio.worldLoopCount()} world loops`);
                audio.setView('menu'); audio.setView('results'); audio.setView('race');
                audio.stopWorld(); audio.stopWorld();
            });
        }
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
