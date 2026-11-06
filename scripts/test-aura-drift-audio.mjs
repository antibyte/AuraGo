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
        const declared = [...match[1].matchAll(/'([a-z0-9._<>]+)'/g)].flatMap(m => m[1].includes('<carId>') ? CARS.map(id => m[1].replace('<carId>', id)) : [m[1]]);
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

console.log(`${checks - failures}/${checks} checks passed`);
if (failures) { console.error(`${failures} failure(s)`); process.exit(1); }
