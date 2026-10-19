// AuraGolf audio pack: sfx/*.ogg, music/*.ogg and manifest.json under ui/3d/minigolf/audio.
//   node scripts/build-minigolf-audio.mjs                       verify the committed pack (read-only, no ffmpeg)
//   node scripts/build-minigolf-audio.mjs --check               same as above
//   node scripts/build-minigolf-audio.mjs --write --sources DIR rebuild the pack from the CC0 downloads in DIR
// assets/minigolf/audio/sources.json is the only hand-edited input; see assets/minigolf/audio/README.md.
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const recipeFile = path.join(root, 'assets/minigolf/audio/sources.json');
const outDir = path.join(root, 'ui/3d/minigolf/audio');
const LICENSE = 'CC0-1.0';
const LIMITS = { sfx: 2 * 1024 * 1024, music: 6 * 1024 * 1024, manifest: 128 * 1024 };
const PATH_RE = /^(sfx|music)\/[a-z0-9-]+\.ogg$/;
const SFX_QUALITY = 4, SFX_PEAK_DB = -1, MUSIC_LUFS = -16, MUSIC_PEAK_DB = -1;
// Square-root gains make the two halves of a loop crossfade sum to constant power (linear in power).
const CROSSFADE_CURVE = 'squ';
const FFMPEG = ['-hide_banner', '-nostdin', '-loglevel', 'error', '-y'];

const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');
const fail = message => { throw new Error(message); };
const posix = file => file.split(path.sep).join('/');
const rel = file => posix(path.relative(root, file));
const isArchive = source => /\.(zip|7z)$/i.test(source.file);
const round = (value, digits = 6) => +value.toFixed(digits);

// Expands sources.json into one job per output file (SFX members in order, then the music tracks).
async function loadRecipe() {
    const recipe = JSON.parse(await fs.readFile(recipeFile, 'utf8'));
    if (recipe.version !== 1) fail('sources.json: version must be 1');
    const ids = new Set(recipe.sources.map(source => source.id));
    if (ids.size !== recipe.sources.length) fail('sources.json: duplicate source id');
    const jobs = [];
    for (const { members, ...entry } of recipe.sfx) members.forEach((member, i) => jobs.push({ ...entry, kind: 'sfx', member, path: `sfx/${entry.group.replaceAll('.', '-')}-${i + 1}.ogg` }));
    for (const entry of recipe.music) jobs.push({ ...entry, kind: 'music', path: `music/${entry.id}.ogg` });
    const seen = new Set();
    for (const job of jobs) {
        if (!ids.has(job.source)) fail(`sources.json: ${job.path} uses unknown source "${job.source}"`);
        if (!PATH_RE.test(job.path) || seen.has(job.path)) fail(`sources.json: invalid or duplicate output name ${job.path}`);
        seen.add(job.path);
    }
    return { recipe, jobs };
}

async function listOgg(dir) {
    const entries = await fs.readdir(dir, { withFileTypes: true, recursive: true }).catch(() => []);
    return entries.filter(entry => entry.isFile() && entry.name.endsWith('.ogg')).map(entry => posix(path.relative(dir, path.join(entry.parentPath, entry.name)))).sort();
}

// ---- --check: read-only verification of the committed pack ----
async function check() {
    const { recipe, jobs } = await loadRecipe();
    const problems = [];
    const bad = message => problems.push(message);
    const manifestFile = path.join(outDir, 'manifest.json');
    const raw = await fs.readFile(manifestFile).catch(error => fail(`Cannot read ${rel(manifestFile)} (${error.message}); run with --write --sources <dir>`));
    const manifest = JSON.parse(raw.toString('utf8'));
    if (raw.length > LIMITS.manifest) bad(`manifest is ${raw.length} bytes (limit ${LIMITS.manifest})`);
    if (manifest.version !== 1) bad('manifest version must be 1');
    if (JSON.stringify(manifest.sources) !== JSON.stringify(recipe.sources)) bad('manifest sources differ from sources.json');
    for (const source of recipe.sources) {
        if (!/^https:\/\//.test(source.page) || !/^https:\/\//.test(source.download)) bad(`source ${source.id}: page and download must be https URLs`);
        if (source.license !== LICENSE || !/^[0-9a-f]{64}$/.test(source.sha256)) bad(`source ${source.id}: license must be ${LICENSE} and sha256 a hex digest`);
    }
    const expectedSfx = {};
    for (const job of jobs.filter(job => job.kind === 'sfx')) (expectedSfx[job.group] ??= []).push(job.path);
    if (JSON.stringify(manifest.sfx) !== JSON.stringify(expectedSfx)) bad('manifest sfx map does not match sources.json');
    const files = manifest.files ?? [];
    const describe = list => list.map(file => [file.path, file.source, file.member].join(' | ')).join('\n');
    if (describe(files) !== describe(jobs)) bad('manifest files do not match the sources.json recipe');
    const totals = { sfx: 0, music: 0 };
    const vorbisId = Buffer.from([1, ...Buffer.from('vorbis')]);
    for (const file of files) {
        if (!PATH_RE.test(file.path)) { bad(`invalid path ${file.path}`); continue; }
        if (file.license !== LICENSE) bad(`${file.path}: license must be ${LICENSE}`);
        const bytes = await fs.readFile(path.join(outDir, file.path)).catch(() => null);
        if (!bytes) { bad(`${file.path}: file is missing`); continue; }
        totals[file.path.split('/')[0]] += bytes.length;
        if (bytes.length !== file.bytes || sha256(bytes) !== file.sha256) bad(`${file.path}: size or SHA-256 differs from the manifest`);
        if (bytes.toString('latin1', 0, 4) !== 'OggS' || !bytes.subarray(0, 64).includes(vorbisId)) bad(`${file.path}: not an Ogg Vorbis file`);
        if (!(file.seconds > 0)) bad(`${file.path}: invalid duration`);
    }
    const music = (manifest.music ?? []).map(track => [track.id, track.file, track.title, track.author, track.seconds].join('|'));
    const expectedMusic = recipe.music.map(track => [track.id, `music/${track.id}.ogg`, track.title, track.author, files.find(file => file.path === `music/${track.id}.ogg`)?.seconds].join('|'));
    if (music.join('\n') !== expectedMusic.join('\n')) bad('manifest music list does not match sources.json');
    const listed = new Set(files.map(file => file.path));
    for (const found of await listOgg(outDir)) if (!listed.has(found)) bad(`${found}: not listed in the manifest (stale file)`);
    if (totals.sfx > LIMITS.sfx) bad(`SFX total ${totals.sfx} bytes exceeds ${LIMITS.sfx}`);
    if (totals.music > LIMITS.music) bad(`music total ${totals.music} bytes exceeds ${LIMITS.music}`);
    if (problems.length) fail(`audio pack verification failed:\n  - ${problems.join('\n  - ')}`);
    console.log(`AuraGolf audio verified: ${files.length} files, SFX ${totals.sfx} bytes, music ${totals.music} bytes`);
}

// ---- --write: process the downloads ----
function run(command, args) {
    const result = spawnSync(command, args, { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024, windowsHide: true });
    if (result.error) fail(`Cannot run ${command}: ${result.error.message}`);
    if (result.status !== 0) fail(`${command} failed (exit ${result.status}): ${args.join(' ')}\n${(result.stderr || '').trim().split('\n').slice(-6).join('\n')}`);
    return `${result.stderr}\n${result.stdout}`;
}

// Windows ships bsdtar as System32\tar.exe (GNU tar from Git Bash cannot read 7z); elsewhere use bsdtar.
const tarCommand = () => process.platform === 'win32' ? path.join(process.env.SystemRoot ?? 'C:\\Windows', 'System32', 'tar.exe') : 'bsdtar';

// Returns the directory that members of this source are resolved in. Archives are listed first (no absolute
// or ".." members) and unpacked into a fresh directory below tmp; plain files are used where they are.
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

// Filter graph for one job: resample, set channels, trim leading silence, cut the region, bake the loop
// crossfade or apply fades. A loop over [a, b) with crossfade C is acrossfade(A=[a+C, b), B=[a, a+C)):
// the result is b-a-C long and its end flows straight into its start.
function filterGraph(job, channels) {
    const { start = 0, end, crossfade: fade = 0 } = job;
    const head = ['aresample=44100', `aformat=channel_layouts=${channels === 1 ? 'mono' : 'stereo'}`];
    if (job.trimSilence !== false) head.push('silenceremove=start_periods=1:start_threshold=-50dB:start_silence=0.002:detection=peak');
    const cut = (from, to) => `atrim=start=${round(from)}${to === undefined ? '' : `:end=${round(to)}`},asetpts=PTS-STARTPTS`;
    if (fade > 0) {
        if (end === undefined) fail(`${job.path}: a crossfade needs an end`);
        return `[0:a]${head.join(',')},asplit=2[x][y];[x]${cut(start + fade, end)}[a];[y]${cut(start, start + fade)}[b];[a][b]acrossfade=d=${fade}:curve1=${CROSSFADE_CURVE}:curve2=${CROSSFADE_CURVE}[out]`;
    }
    const tail = [cut(start, end)];
    if (job.fadeIn) tail.push(`afade=t=in:st=0:d=${job.fadeIn}`);
    if (job.fadeOut) tail.push(`afade=t=out:st=${job.fadeOut.start}:d=${job.fadeOut.duration}`);
    return `[0:a]${head.concat(tail).join(',')}[out]`;
}

// Decodes and processes the input to a float WAV, measures it, then encodes the normalized Ogg Vorbis file.
function encode(job, input, dest, tmp) {
    const stage = path.join(tmp, 'stage.wav');
    run('ffmpeg', [...FFMPEG, '-i', input, '-filter_complex', filterGraph(job, job.kind === 'sfx' ? 1 : 2), '-map', '[out]', '-c:a', 'pcm_f32le', stage]);
    // astats reads the float samples (volumedetect would clip at 0 dBFS and miss hot sources).
    const meter = 'astats=measure_perchannel=none:measure_overall=Peak_level' + (job.kind === 'music' ? ',ebur128=framelog=quiet' : '');
    const stats = run('ffmpeg', ['-hide_banner', '-nostats', '-i', stage, '-af', meter, '-f', 'null', '-']);
    const peak = Number(/Peak level dB: (-?[\d.]+)/.exec(stats)?.[1]);
    if (!Number.isFinite(peak) || peak < -60) fail(`${job.path}: processed audio is silent or unreadable`);
    let gain = SFX_PEAK_DB - peak;
    if (job.kind === 'music') {
        const lufs = Number(/\bI:\s+(-?[\d.]+) LUFS/.exec(stats)?.[1]);
        if (!Number.isFinite(lufs)) fail(`${job.path}: cannot measure loudness`);
        gain = Math.min(MUSIC_LUFS - lufs, MUSIC_PEAK_DB - peak);
    }
    run('ffmpeg', [...FFMPEG, '-i', stage, '-af', `volume=${gain.toFixed(3)}dB`, '-map_metadata', '-1', '-fflags', '+bitexact', '-flags:a', '+bitexact',
        '-c:a', 'libvorbis', '-q:a', String(job.kind === 'sfx' ? SFX_QUALITY : job.quality), '-ar', '44100', dest]);
    return { peak, gain };
}

async function write(sourcesDir) {
    const { recipe, jobs } = await loadRecipe();
    const version = run('ffmpeg', ['-hide_banner', '-version']);
    if (Number(/ffmpeg version [nN]?(\d+)/.exec(version)?.[1]) < 8) fail('ffmpeg 8 or newer is required');
    if (!/\blibvorbis\b/.test(run('ffmpeg', ['-hide_banner', '-encoders']))) fail('this ffmpeg build lacks the libvorbis encoder');
    const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'aurago-minigolf-audio-'));
    try {
        const bases = new Map();
        for (const source of recipe.sources) if (jobs.some(job => job.source === source.id)) bases.set(source.id, await materialize(source, path.resolve(sourcesDir), tmp));
        const staged = path.join(tmp, 'out');
        await fs.mkdir(path.join(staged, 'sfx'), { recursive: true });
        await fs.mkdir(path.join(staged, 'music'));
        const manifest = { version: 1, sfx: {}, music: [], files: [], sources: recipe.sources };
        for (const job of jobs) {
            const source = recipe.sources.find(candidate => candidate.id === job.source);
            const input = await resolveMember(bases.get(source.id), source, job.member);
            const dest = path.join(staged, job.path);
            const { peak, gain } = encode(job, input, dest, tmp);
            const bytes = await fs.readFile(dest);
            const seconds = round(Number(run('ffprobe', ['-v', 'error', '-show_entries', 'format=duration', '-of', 'csv=p=0', dest]).trim()), 3);
            manifest.files.push({ path: job.path, bytes: bytes.length, sha256: sha256(bytes), seconds, source: job.source, member: job.member, license: LICENSE });
            if (job.kind === 'sfx') (manifest.sfx[job.group] ??= []).push(job.path);
            else manifest.music.push({ id: job.id, file: job.path, title: job.title, author: job.author, seconds });
            console.log(`${job.path.padEnd(34)} ${String(bytes.length).padStart(8)} B ${seconds.toFixed(3).padStart(8)} s  peak ${peak.toFixed(2)} dB, gain ${gain.toFixed(2)} dB`);
        }
        // Replace the shipped pack only after every file was produced.
        for (const folder of ['sfx', 'music']) {
            await fs.mkdir(path.join(outDir, folder), { recursive: true });
            for (const name of await fs.readdir(path.join(outDir, folder))) if (name.endsWith('.ogg')) await fs.rm(path.join(outDir, folder, name));
        }
        for (const file of manifest.files) await fs.copyFile(path.join(staged, file.path), path.join(outDir, file.path));
        await fs.writeFile(path.join(outDir, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n');
    } finally {
        await fs.rm(tmp, { recursive: true, force: true });
    }
    await check();
}

const args = process.argv.slice(2);
const sourcesAt = args.indexOf('--sources');
try {
    const unknown = args.filter((arg, i) => !['--check', '--write', '--sources'].includes(arg) && (sourcesAt < 0 || i !== sourcesAt + 1));
    if (unknown.length) fail(`unknown argument ${unknown[0]}; usage: [--check] | --write --sources <dir>`);
    if (args.includes('--write')) {
        if (args.includes('--check') || sourcesAt < 0 || !args[sourcesAt + 1]) fail('usage: --write --sources <dir> (without --check)');
        await write(args[sourcesAt + 1]);
    } else if (sourcesAt >= 0) fail('--sources is only used with --write');
    else await check();
} catch (error) {
    console.error(`AuraGolf audio: ${error.message}`);
    process.exitCode = 1;
}
