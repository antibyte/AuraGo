// Aura Drift audio pack: sfx/*.ogg, engine/*.ogg, music/*.ogg and manifest.json under ui/3d/aura-drift/audio.
//   node scripts/build-aura-drift-audio.mjs                         verify the committed pack (read-only, no ffmpeg)
//   node scripts/build-aura-drift-audio.mjs --check                 same as above
//   node scripts/build-aura-drift-audio.mjs --write [--sources DIR] rebuild the pack: own synthesis plus approved CC0 overrides in DIR
//   node scripts/build-aura-drift-audio.mjs --preview DIR [--only REGEX]   render normalized WAVs for listening (nothing is written to the repo)
//   node scripts/build-aura-drift-audio.mjs --list                  print every job id
// Every sound is synthesized by scripts/aura-drift-synth/*.mjs (seeded, deterministic). A `sources.json` override replaces one
// sfx group with a recording from a hash-pinned download; see assets/aura-drift/audio/README.md.
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { writeFileSync, rmSync } from 'node:fs';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { Worker } from 'node:worker_threads';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const recipeFile = path.join(root, 'assets/aura-drift/audio/sources.json');
const outDir = path.join(root, 'ui/3d/aura-drift/audio');
const synthDir = path.join(root, 'scripts/aura-drift-synth');
const LICENSE = 'CC0-1.0', OWN = 'own-synthesis';
export const LIMITS = { sfx: 4 * 1024 * 1024, engine: 1.5 * 1024 * 1024, music: 10 * 1024 * 1024, manifest: 128 * 1024 };
const PATH_RE = /^(sfx|engine|music)\/[a-z0-9-]+\.ogg$/;
const SFX_PEAK_DB = -1, ENGINE_PEAK_DB = -3, MUSIC_LUFS = -16, MUSIC_PEAK_DB = -1;
const QUALITY = { sfx: 4, engine: 3, music: 1 };
const CROSSFADE_CURVE = 'squ';
const SEAM_LIMIT = 8;
const FFMPEG = ['-hide_banner', '-nostdin', '-loglevel', 'error', '-y'];

const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');
const fail = message => { throw new Error(message); };
const posix = file => file.split(path.sep).join('/');
const rel = file => posix(path.relative(root, file));
const isArchive = source => /\.(zip|7z)$/i.test(source.file);
const round = (value, digits = 6) => +value.toFixed(digits);
const slug = text => text.replace(/[._]/g, '-');
const registry = () => import(pathToFileURL(path.join(synthDir, 'registry.mjs')).href);

// Hash of every synth script (line endings normalised): --check refuses a pack that is older than its recipes.
async function synthHash() {
    const names = (await fs.readdir(synthDir)).filter(name => name.endsWith('.mjs')).sort();
    const h = createHash('sha256');
    for (const name of names) h.update(name + '\n').update((await fs.readFile(path.join(synthDir, name), 'utf8')).replace(/\r\n/g, '\n') + '\n');
    return h.digest('hex');
}

// Expands jobs and overrides into the exact list of output files: [{ path, group?, kind, job, source?, member?, override? }].
async function loadPlan() {
    const recipe = JSON.parse(await fs.readFile(recipeFile, 'utf8'));
    if (recipe.version !== 1) fail('sources.json: version must be 1');
    if (!Array.isArray(recipe.sources) || !Array.isArray(recipe.overrides)) fail('sources.json: sources and overrides must be arrays');
    const ids = new Set(recipe.sources.map(source => source.id));
    if (ids.size !== recipe.sources.length) fail('sources.json: duplicate source id');
    const reg = await registry();
    const sfxGroups = new Set(reg.SFX_RECIPES.map(item => item.group));
    const overrides = new Map();
    for (const entry of recipe.overrides) {
        if (!sfxGroups.has(entry.group)) fail(`sources.json: override for unknown sfx group "${entry.group}"`);
        if (!ids.has(entry.source)) fail(`sources.json: override ${entry.group} uses unknown source "${entry.source}"`);
        if (!Array.isArray(entry.members) || !entry.members.length) fail(`sources.json: override ${entry.group} needs members`);
        if (overrides.has(entry.group)) fail(`sources.json: duplicate override for ${entry.group}`);
        overrides.set(entry.group, entry);
    }
    const files = [], seen = new Set();
    const push = file => {
        if (!PATH_RE.test(file.path) || seen.has(file.path)) fail(`invalid or duplicate output name ${file.path}`);
        seen.add(file.path); files.push(file);
    };
    for (const job of reg.jobs()) {
        if (job.type === 'sfx') {
            const override = overrides.get(job.group);
            if (override) {
                if (job.variant === 0) override.members.forEach((member, i) => push({ path: `sfx/${slug(job.group)}-${i + 1}.ogg`, kind: 'sfx', group: job.group, override: { ...override, member }, source: override.source, member, channels: 1, loop: !!override.loop }));
            } else push({ path: job.path, kind: 'sfx', group: job.group, job, channels: job.channels, loop: job.loop });
        } else if (job.type === 'engine') push({ path: job.path, kind: 'engine', job, car: job.car, layer: job.layer, channels: 1, loop: true });
        else if (job.type === 'music') for (const [layer, file] of Object.entries(job.paths)) push({ path: file, kind: 'music', job, biome: job.biome, layer, channels: 2, loop: true });
        else push({ path: job.path, kind: 'music', job, name: job.name, channels: 2, loop: true });
    }
    return { recipe, files, reg, overrides };
}

function buildManifest(plan, measured, hash) {
    const manifest = { version: 1, synth: hash, sfx: {}, engine: {}, music: {}, files: [], sources: plan.recipe.sources, overrides: plan.recipe.overrides };
    for (const file of plan.files) {
        const m = measured.get(file.path);
        const entry = { path: file.path, bytes: m.bytes, sha256: m.sha256, seconds: m.seconds, channels: file.channels, loop: !!file.loop, origin: file.source ?? OWN };
        if (file.source) { entry.member = file.member; entry.license = LICENSE; } else entry.license = OWN;
        manifest.files.push(entry);
        if (file.kind === 'sfx') (manifest.sfx[file.group] ??= []).push(file.path);
        else if (file.kind === 'engine') (manifest.engine[file.car] ??= {})[file.layer] = file.path;
        else if (file.biome) (manifest.music[file.biome] ??= {})[file.layer] = file.path;
        else manifest.music[file.name] = file.path;
    }
    return manifest;
}
const manifestText = manifest => {
    const { files, ...rest } = manifest;
    const head = JSON.stringify(rest, null, 2).replace(/\n}$/, '');
    return `${head},\n  "files": [\n${files.map(file => '    ' + JSON.stringify(file)).join(',\n')}\n  ]\n}\n`;
};

async function listOgg(dir) {
    const entries = await fs.readdir(dir, { withFileTypes: true, recursive: true }).catch(() => []);
    return entries.filter(entry => entry.isFile() && entry.name.endsWith('.ogg')).map(entry => posix(path.relative(dir, path.join(entry.parentPath, entry.name)))).sort();
}

// ---- --check ----
async function check() {
    const plan = await loadPlan();
    const problems = [], bad = message => problems.push(message);
    const manifestFile = path.join(outDir, 'manifest.json');
    const raw = await fs.readFile(manifestFile).catch(error => fail(`Cannot read ${rel(manifestFile)} (${error.message}); run with --write`));
    const manifest = JSON.parse(raw.toString('utf8'));
    if (raw.length > LIMITS.manifest) bad(`manifest is ${raw.length} bytes (limit ${LIMITS.manifest})`);
    if (manifest.version !== 1) bad('manifest version must be 1');
    if (JSON.stringify(manifest.sources) !== JSON.stringify(plan.recipe.sources)) bad('manifest sources differ from sources.json');
    if (JSON.stringify(manifest.overrides) !== JSON.stringify(plan.recipe.overrides)) bad('manifest overrides differ from sources.json');
    for (const source of plan.recipe.sources) {
        if (!/^https:\/\//.test(source.page) || !/^https:\/\//.test(source.download)) bad(`source ${source.id}: page and download must be https URLs`);
        if (source.license !== LICENSE || !/^[0-9a-f]{64}$/.test(source.sha256)) bad(`source ${source.id}: license must be ${LICENSE} and sha256 a hex digest`);
    }
    if (manifest.synth !== await synthHash()) bad('scripts/aura-drift-synth changed since the pack was built; run with --write');
    const describe = list => list.map(file => [file.path, file.origin ?? file.source ?? OWN, file.member ?? ''].join(' | ')).join('\n');
    const files = manifest.files ?? [];
    if (describe(files) !== describe(plan.files.map(file => ({ path: file.path, origin: file.source ?? OWN, member: file.member })))) bad('manifest files do not match the planned output list');
    const expected = buildManifest(plan, new Map(files.map(file => [file.path, file])), manifest.synth);
    for (const key of ['sfx', 'engine', 'music']) if (JSON.stringify(manifest[key]) !== JSON.stringify(expected[key])) bad(`manifest ${key} map does not match the recipes`);
    const totals = { sfx: 0, engine: 0, music: 0 }, vorbisId = Buffer.from([1, ...Buffer.from('vorbis')]);
    for (const file of files) {
        if (!PATH_RE.test(file.path)) { bad(`invalid path ${file.path}`); continue; }
        const bytes = await fs.readFile(path.join(outDir, file.path)).catch(() => null);
        if (!bytes) { bad(`${file.path}: file is missing`); continue; }
        totals[file.path.split('/')[0]] += bytes.length;
        if (bytes.length !== file.bytes || sha256(bytes) !== file.sha256) bad(`${file.path}: size or SHA-256 differs from the manifest`);
        if (bytes.toString('latin1', 0, 4) !== 'OggS' || !bytes.subarray(0, 64).includes(vorbisId)) bad(`${file.path}: not an Ogg Vorbis file`);
        if (!(file.seconds > 0)) bad(`${file.path}: invalid duration`);
    }
    const listed = new Set(files.map(file => file.path));
    for (const found of await listOgg(outDir)) if (!listed.has(found)) bad(`${found}: not listed in the manifest (stale file)`);
    for (const key of ['sfx', 'engine', 'music']) if (totals[key] > LIMITS[key]) bad(`${key} total ${totals[key]} bytes exceeds ${LIMITS[key]}`);
    if (problems.length) fail(`audio pack verification failed:\n  - ${problems.join('\n  - ')}`);
    console.log(`Aura Drift audio verified: ${files.length} files, sfx ${totals.sfx} B, engine ${totals.engine} B, music ${totals.music} B, manifest ${raw.length} B`);
}

// ---- processes ----
function run(command, args) {
    const result = spawnSync(command, args, { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024, windowsHide: true });
    if (result.error) fail(`Cannot run ${command}: ${result.error.message}`);
    if (result.status !== 0) fail(`${command} failed (exit ${result.status}): ${args.join(' ')}\n${(result.stderr || '').trim().split('\n').slice(-6).join('\n')}`);
    return `${result.stderr}\n${result.stdout}`;
}
const tarCommand = () => process.platform === 'win32' ? path.join(process.env.SystemRoot ?? 'C:\\Windows', 'System32', 'tar.exe') : 'bsdtar';

async function runPool(ids, onResult) {
    const count = Math.max(1, Math.min(8, os.cpus().length - 2, ids.length)), queue = [...ids];
    const workerFile = path.join(synthDir, 'worker.mjs');
    await Promise.all(Array.from({ length: count }, () => new Promise((resolve, reject) => {
        const worker = new Worker(workerFile);
        const next = () => { const id = queue.shift(); if (id === undefined) { worker.terminate(); resolve(); } else worker.postMessage({ id }); };
        worker.on('message', async msg => {
            if (msg.error) { worker.terminate(); reject(new Error(`${msg.id}: ${msg.error}`)); return; }
            try { await onResult(msg.id, msg.payload); next(); } catch (error) { worker.terminate(); reject(error); }
        });
        worker.on('error', reject);
        next();
    })));
}

// ---- synthesized output ----
const peakOf = chans => { let p = 0; for (const ch of chans) for (let i = 0; i < ch.length; i++) { const a = Math.abs(ch[i]); if (a > p) p = a; } return p; };
const toDb = v => 20 * Math.log10(Math.max(v, 1e-12));
function wavF32(chans, gain = 1) {
    const n = chans[0].length, c = chans.length, buf = Buffer.alloc(44 + n * c * 4);
    buf.write('RIFF', 0); buf.writeUInt32LE(36 + n * c * 4, 4); buf.write('WAVEfmt ', 8); buf.writeUInt32LE(16, 16); buf.writeUInt16LE(3, 20); buf.writeUInt16LE(c, 22);
    buf.writeUInt32LE(44100, 24); buf.writeUInt32LE(44100 * c * 4, 28); buf.writeUInt16LE(c * 4, 32); buf.writeUInt16LE(32, 34); buf.write('data', 36); buf.writeUInt32LE(n * c * 4, 40);
    let off = 44;
    for (let i = 0; i < n; i++) for (let k = 0; k < c; k++) { buf.writeFloatLE(Math.fround(chans[k][i] * gain), off); off += 4; }
    return buf;
}
// Loop seam: step across the wrap point relative to the typical sample-to-sample step (1 means indistinguishable from the rest).
function seamRatio(chans) {
    let worst = 0;
    for (const x of chans) {
        let d2 = 0; for (let i = 1; i < x.length; i++) { const d = x[i] - x[i - 1]; d2 += d * d; }
        const rms = Math.sqrt(d2 / x.length) || 1e-9;
        worst = Math.max(worst, Math.abs(x[0] - x[x.length - 1]) / rms);
    }
    return worst;
}
function decodedPeakDb(file) {
    const out = run('ffmpeg', ['-hide_banner', '-nostats', '-i', file, '-af', 'astats=measure_perchannel=none:measure_overall=Peak_level', '-f', 'null', '-']);
    return Number(/Peak level dB: (-?[\d.]+)/.exec(out)?.[1]);
}
// Lossy coding overshoots dense, clipped sounds; lower the gain until the decoded file honours the target peak.
function encodeGuarded(chans, wav, dest, quality, targetDb) {
    let gain = 10 ** ((targetDb - toDb(peakOf(chans))) / 20), decoded = NaN;
    for (let attempt = 0; attempt < 5; attempt++) {
        writeFileSync(wav, wavF32(chans, gain));
        encodeWav(wav, dest, quality);
        decoded = decodedPeakDb(dest);
        if (!Number.isFinite(decoded) || decoded <= targetDb + 0.15) break;
        gain *= 10 ** (-(decoded - targetDb + 0.05) / 20);
    }
    rmSync(wav);
    return decoded;
}
function encodeWav(wavPath, dest, quality) {
    run('ffmpeg', [...FFMPEG, '-i', wavPath, '-map_metadata', '-1', '-fflags', '+bitexact', '-flags:a', '+bitexact', '-c:a', 'libvorbis', '-q:a', String(quality), '-ar', '44100', dest]);
}
function loudness(wavPath) {
    const out = run('ffmpeg', ['-hide_banner', '-nostats', '-i', wavPath, '-af', 'ebur128=framelog=quiet', '-f', 'null', '-']);
    const lufs = Number(/\bI:\s+(-?[\d.]+) LUFS/.exec(out)?.[1]);
    if (!Number.isFinite(lufs)) fail(`cannot measure loudness of ${wavPath}`);
    return lufs;
}
const sumChans = list => list[0].map((_, c) => { const o = new Float32Array(list[0][0].length); for (const stem of list) for (let i = 0; i < o.length; i++) o[i] += stem[c][i]; return o; });

// Gain that puts the three summed stems at -16 LUFS (limited by -1 dBFS peak); the same gain applies to all three.
function musicGain(stems, tmp) {
    const sum = sumChans(stems), file = path.join(tmp, `sum-${process.hrtime.bigint()}.wav`);
    writeFileSync(file, wavF32(sum));
    const lufs = loudness(file), peak = toDb(peakOf(sum));
    rmSync(file);
    return { gain: Math.min(MUSIC_LUFS - lufs, MUSIC_PEAK_DB - peak), lufs, peak };
}

// ---- recordings (overrides) ----
async function materialize(source, sourcesDir, tmp) {
    const file = path.join(sourcesDir, source.file);
    const bytes = await fs.readFile(file).catch(() => fail(`Missing ${source.file} in ${sourcesDir}; download it from ${source.download}`));
    if (bytes.length !== source.bytes || sha256(bytes) !== source.sha256) fail(`${source.file} does not match sources.json (expected ${source.bytes} bytes, SHA-256 ${source.sha256}); download it again from ${source.download}`);
    if (!isArchive(source)) return sourcesDir;
    for (const name of run(tarCommand(), ['-tf', file]).split(/\r?\n/).filter(Boolean)) {
        if (/^([a-zA-Z]:|[\\/])/.test(name) || name.split(/[\\/]/).includes('..')) fail(`${source.file} contains an unsafe member: ${name}`);
    }
    const dir = path.join(tmp, source.id);
    await fs.mkdir(dir);
    run(tarCommand(), ['-xf', file, '-C', dir]);
    return dir;
}
async function resolveMember(base, source, member) {
    const target = path.resolve(base, member);
    if (path.isAbsolute(member) || member.split(/[\\/]/).includes('..') || !target.startsWith(base + path.sep)) fail(`${source.id}: unsafe member ${member}`);
    if (!isArchive(source) && member !== source.file) fail(`${source.id}: member must be ${source.file}`);
    const stat = await fs.lstat(target).catch(() => null);
    if (!stat?.isFile()) fail(`${source.id}: member ${member} not found in ${source.file}`);
    return target;
}
function filterGraph(job) {
    const { start = 0, end, crossfade: fade = 0 } = job;
    const head = ['aresample=44100', 'aformat=channel_layouts=mono'];
    if (job.trimSilence !== false) head.push('silenceremove=start_periods=1:start_threshold=-50dB:start_silence=0.002:detection=peak');
    const cut = (from, to) => `atrim=start=${round(from)}${to === undefined ? '' : `:end=${round(to)}`},asetpts=PTS-STARTPTS`;
    if (fade > 0) {
        if (end === undefined) fail(`${job.group}: a crossfade needs an end`);
        return `[0:a]${head.join(',')},asplit=2[x][y];[x]${cut(start + fade, end)}[a];[y]${cut(start, start + fade)}[b];[a][b]acrossfade=d=${fade}:curve1=${CROSSFADE_CURVE}:curve2=${CROSSFADE_CURVE}[out]`;
    }
    const tail = [cut(start, end)];
    if (job.fadeIn) tail.push(`afade=t=in:st=0:d=${job.fadeIn}`);
    if (job.fadeOut) tail.push(`afade=t=out:st=${job.fadeOut.start}:d=${job.fadeOut.duration}`);
    return `[0:a]${head.concat(tail).join(',')}[out]`;
}
function encodeRecording(override, input, dest, tmp) {
    const stage = path.join(tmp, 'stage.wav');
    run('ffmpeg', [...FFMPEG, '-i', input, '-filter_complex', filterGraph(override), '-map', '[out]', '-c:a', 'pcm_f32le', stage]);
    const stats = run('ffmpeg', ['-hide_banner', '-nostats', '-i', stage, '-af', 'astats=measure_perchannel=none:measure_overall=Peak_level', '-f', 'null', '-']);
    const peak = Number(/Peak level dB: (-?[\d.]+)/.exec(stats)?.[1]);
    if (!Number.isFinite(peak) || peak < -60) fail(`${override.group}: processed audio is silent or unreadable`);
    run('ffmpeg', [...FFMPEG, '-i', stage, '-af', `volume=${(SFX_PEAK_DB - peak).toFixed(3)}dB`, '-map_metadata', '-1', '-fflags', '+bitexact', '-flags:a', '+bitexact', '-c:a', 'libvorbis', '-q:a', String(QUALITY.sfx), '-ar', '44100', dest]);
}

async function write(sourcesDir) {
    const plan = await loadPlan(), hash = await synthHash();
    const version = run('ffmpeg', ['-hide_banner', '-version']);
    if (Number(/ffmpeg version [nN]?(\d+)/.exec(version)?.[1]) < 8) fail('ffmpeg 8 or newer is required');
    if (!/\blibvorbis\b/.test(run('ffmpeg', ['-hide_banner', '-encoders']))) fail('this ffmpeg build lacks the libvorbis encoder');
    if (plan.overrides.size && !sourcesDir) fail('sources.json has overrides: pass --sources <dir> with the pinned downloads');
    const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'aurago-drift-audio-'));
    try {
        const staged = path.join(tmp, 'out');
        for (const folder of ['sfx', 'engine', 'music']) await fs.mkdir(path.join(staged, folder), { recursive: true });
        const byPath = new Map(plan.files.map(file => [file.path, file]));
        const measured = new Map();
        // recordings first (serial, uses ffmpeg only)
        const bases = new Map();
        for (const file of plan.files.filter(item => item.override)) {
            const source = plan.recipe.sources.find(candidate => candidate.id === file.source);
            if (!bases.has(source.id)) bases.set(source.id, await materialize(source, path.resolve(sourcesDir), tmp));
            encodeRecording({ ...file.override, group: file.group }, await resolveMember(bases.get(source.id), source, file.member), path.join(staged, file.path), tmp);
        }
        const jobIds = [...new Set(plan.files.filter(file => file.job).map(file => file.job.id))];
        const started = Date.now();
        await runPool(jobIds, async (id, payload) => {
            const outputs = plan.files.filter(file => file.job?.id === id);
            if (id.startsWith('music:')) {
                const stems = Array.isArray(payload) ? [payload] : Object.values(payload);
                const { gain, lufs, peak } = musicGain(stems, tmp);
                const layers = Array.isArray(payload) ? [[outputs[0].path, payload]] : Object.entries(payload).map(([layer, chans]) => [outputs.find(item => item.layer === layer).path, chans]);
                for (const [file, chans] of layers) {
                    const wav = path.join(tmp, file.replace('/', '_') + '.wav');
                    await fs.writeFile(wav, wavF32(chans, 10 ** (gain / 20)));
                    encodeWav(wav, path.join(staged, file), QUALITY.music); await fs.rm(wav);
                }
                // a drum or pluck starting on beat one is a legitimate onset at the loop seam, so music stems are not seam-checked here
                console.log(`${id.padEnd(22)} mix ${lufs.toFixed(1)} LUFS peak ${peak.toFixed(1)} dB, gain ${gain.toFixed(2)} dB`);
            } else {
                const chans = payload, file = outputs[0].path, peak = toDb(peakOf(chans)), target = file.startsWith('engine/') ? ENGINE_PEAK_DB : SFX_PEAK_DB;
                if (peak < -60) fail(`${id}: rendered audio is silent`);
                encodeGuarded(chans, path.join(tmp, file.replace('/', '_') + '.wav'), path.join(staged, file), file.startsWith('engine/') ? QUALITY.engine : QUALITY.sfx, target);
                if (outputs[0].loop) { const ratio = seamRatio(chans); if (ratio > SEAM_LIMIT) console.warn(`WARNING ${id}: loop seam ${ratio.toFixed(1)}x a typical step`); }
            }
        });
        console.log(`rendered ${jobIds.length} jobs in ${((Date.now() - started) / 1000).toFixed(1)} s`);
        for (const file of plan.files) {
            const dest = path.join(staged, file.path), bytes = await fs.readFile(dest);
            const seconds = round(Number(run('ffprobe', ['-v', 'error', '-show_entries', 'format=duration', '-of', 'csv=p=0', dest]).trim()), 6);
            measured.set(file.path, { bytes: bytes.length, sha256: sha256(bytes), seconds });
        }
        void byPath;
        const manifest = buildManifest(plan, measured, hash);
        for (const folder of ['sfx', 'engine', 'music']) {
            await fs.mkdir(path.join(outDir, folder), { recursive: true });
            for (const name of await fs.readdir(path.join(outDir, folder))) if (name.endsWith('.ogg')) await fs.rm(path.join(outDir, folder, name));
        }
        for (const file of manifest.files) await fs.copyFile(path.join(staged, file.path), path.join(outDir, file.path));
        await fs.writeFile(path.join(outDir, 'manifest.json'), manifestText(manifest));
    } finally {
        await fs.rm(tmp, { recursive: true, force: true });
    }
    await check();
}

// ---- --preview ----
async function preview(dir, only) {
    const reg = await registry();
    const re = only ? new RegExp(only) : /./;
    const ids = reg.jobs().map(job => job.id).filter(id => re.test(id));
    if (!ids.length) fail('no job matches --only');
    await fs.mkdir(dir, { recursive: true });
    const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'aurago-drift-preview-'));
    try {
        await runPool(ids, async (id, payload) => {
            const name = id.replace(/[:.]/g, '_');
            if (id.startsWith('music:')) {
                const stems = Array.isArray(payload) ? [payload] : Object.values(payload), { gain, lufs, peak } = musicGain(stems, tmp);
                const entries = Array.isArray(payload) ? [[name, payload]] : Object.entries(payload).map(([layer, chans]) => [`${name}_${layer}`, chans]);
                for (const [file, chans] of entries) await fs.writeFile(path.join(dir, file + '.wav'), wavF32(chans, 10 ** (gain / 20)));
                if (!Array.isArray(payload)) await fs.writeFile(path.join(dir, `${name}_mix.wav`), wavF32(sumChans(stems), 10 ** (gain / 20)));
                console.log(`${id}: mix ${lufs.toFixed(1)} LUFS, peak ${peak.toFixed(1)} dB, gain ${gain.toFixed(2)} dB, ${(stems[0][0].length / 44100).toFixed(3)} s`);
            } else {
                const peak = toDb(peakOf(payload));
                await fs.writeFile(path.join(dir, name + '.wav'), wavF32(payload, 10 ** (((id.startsWith('engine:') ? ENGINE_PEAK_DB : SFX_PEAK_DB) - peak) / 20)));
                console.log(`${id}: ${(payload[0].length / 44100).toFixed(3)} s, raw peak ${peak.toFixed(1)} dB`);
            }
        });
    } finally { await fs.rm(tmp, { recursive: true, force: true }); }
}

const args = process.argv.slice(2);
const valueOf = flag => { const at = args.indexOf(flag); return at >= 0 ? args[at + 1] : undefined; };
try {
    const flags = ['--check', '--write', '--sources', '--preview', '--only', '--list'];
    const values = new Set(['--sources', '--preview', '--only'].map(flag => args.indexOf(flag) + 1).filter(i => i > 0));
    const unknown = args.filter((arg, i) => !flags.includes(arg) && !values.has(i));
    if (unknown.length) fail(`unknown argument ${unknown[0]}; usage: [--check] | --write [--sources <dir>] | --preview <dir> [--only <regex>] | --list`);
    if (args.includes('--list')) for (const job of (await registry()).jobs()) console.log(job.id);
    else if (args.includes('--preview')) await preview(valueOf('--preview') ?? fail('--preview needs a directory'), valueOf('--only'));
    else if (args.includes('--write')) { if (args.includes('--check')) fail('--write and --check are exclusive'); await write(valueOf('--sources')); }
    else if (args.includes('--sources')) fail('--sources is only used with --write');
    else await check();
} catch (error) {
    console.error(`Aura Drift audio: ${error.message}`);
    process.exitCode = 1;
}
