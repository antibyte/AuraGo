// SPDX-License-Identifier: GPL-3.0-only
// Builds the RB-338 demo songs (scripts/rb338-demos/<slug>.mjs) into ui/js/desktop/apps/rb338-demos/
// with the vendored OpenRB-338 wasm. Deterministic: the same specs always give the same bytes.
//   node scripts/build-rb338-demos.mjs                  build, verify and write the .rbs files and demos.json
//   node scripts/build-rb338-demos.mjs --check          rebuild in memory and compare with the files on disk
//   node scripts/build-rb338-demos.mjs --wav <dir>      also write the song-mode WAV of every song into <dir>
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { ROOT, openEngine, buildSong, analysePcm, integratedLufs, truePeakDb } from './rb338-demos/lib.mjs';

// The order of the songs in demos.json and in the demo menu.
export const SONGS = ['acid-sunrise', 'midnight-platform', 'plastic-circuits', 'iron-voltage', 'subwoofer-sunset', 'dub-station-zero'];
export const OUT_DIR = path.join(ROOT, 'ui/js/desktop/apps/rb338-demos');

// A built song is checked before it is written: it reloads and saves to the same bytes, plays
// the length the arrangement promises, is not silent, and stays clear of clipping.
const MAX_PEAK = 0.95;
// Loudness target of every demo (BS.1770 integrated, LUFS) and the true-peak ceiling (dBTP).
const TARGET_LUFS = -16;
const LUFS_TOLERANCE = 1;
const MAX_TRUE_PEAK_DB = -1;

export async function buildAll({ wavDir } = {}) {
    const engine = await openEngine();
    const files = new Map();
    const manifest = [];
    const report = [];
    try {
        for (const slug of SONGS) {
            const spec = (await import(pathToFileURL(path.join(ROOT, 'scripts/rb338-demos', slug + '.mjs')).href)).default;
            if (spec.slug !== slug) throw new Error(slug + ': spec.slug is ' + spec.slug);
            const built = await buildSong(spec, engine);

            // 1. a reload and a second save give the same bytes
            const code = engine.load(built.bytes);
            if (code !== 0) throw new Error(slug + ': orb_load_song returned ' + code);
            if (Buffer.compare(engine.save(), built.bytes) !== 0) throw new Error(slug + ': save after load differs');
            const info = engine.info();
            if (info.title !== spec.title) throw new Error(slug + ': song title is "' + info.title + '"');

            // 2. the whole song in song mode
            const { wav, pcm } = engine.exportWav(built.bytes);
            const a = analysePcm(pcm);
            if (Math.abs(a.seconds - built.seconds) > 0.1) throw new Error(slug + ': renders ' + a.seconds.toFixed(2) + ' s, expected ' + built.seconds.toFixed(2));
            if (a.peak >= MAX_PEAK) throw new Error(slug + ': peak ' + a.peak.toFixed(3) + ' leaves too little headroom');
            if (a.rmsDb < -40) throw new Error(slug + ': nearly silent, ' + a.rmsDb.toFixed(1) + ' dBFS RMS');
            const lufs = integratedLufs(pcm);
            const tp = truePeakDb(pcm);
            if (tp > MAX_TRUE_PEAK_DB) throw new Error(slug + ': true peak ' + tp.toFixed(2) + ' dBTP is above ' + MAX_TRUE_PEAK_DB);
            if (Math.abs(lufs - TARGET_LUFS) > LUFS_TOLERANCE) throw new Error(slug + ': ' + lufs.toFixed(2) + ' LUFS is not within ' + LUFS_TOLERANCE + ' LU of ' + TARGET_LUFS);
            if (wavDir) {
                fs.mkdirSync(wavDir, { recursive: true });
                fs.writeFileSync(path.join(wavDir, slug + '.wav'), wav);
            }

            files.set(slug + '.rbs', built.bytes);
            manifest.push({ id: slug, title: spec.title, style: spec.style, bpm: spec.bpm, bars: built.bars, file: slug + '.rbs', seconds: Math.round(built.seconds * 10) / 10 });
            report.push({ slug, bytes: built.bytes.length, seconds: a.seconds, peakDb: a.peakDb, rmsDb: a.rmsDb, lufs, tp });
        }
    } finally {
        engine.destroy();
    }
    files.set('demos.json', Buffer.from(JSON.stringify(manifest, null, 2) + '\n'));
    return { files, manifest, report };
}

async function main() {
    const args = process.argv.slice(2);
    const check = args.includes('--check');
    const wavAt = args.indexOf('--wav');
    const wavDir = wavAt >= 0 ? path.resolve(args[wavAt + 1]) : null;
    const { files, report } = await buildAll({ wavDir });
    if (check) {
        const bad = [];
        for (const [name, bytes] of files) {
            const file = path.join(OUT_DIR, name);
            if (!fs.existsSync(file) || Buffer.compare(fs.readFileSync(file), bytes) !== 0) bad.push(name);
        }
        const known = new Set(files.keys());
        if (fs.existsSync(OUT_DIR)) for (const name of fs.readdirSync(OUT_DIR)) if (!known.has(name)) bad.push(name + ' (unexpected)');
        if (bad.length) {
            console.error('rb338 demos out of date: ' + bad.join(', ') + '\nrun: node scripts/build-rb338-demos.mjs');
            process.exit(1);
        }
        console.log('rb338 demos: up to date (' + files.size + ' files)');
        return;
    }
    fs.mkdirSync(OUT_DIR, { recursive: true });
    for (const [name, bytes] of files) fs.writeFileSync(path.join(OUT_DIR, name), bytes);
    for (const r of report) {
        console.log(r.slug.padEnd(18) + String(r.bytes).padStart(6) + ' bytes  ' + r.seconds.toFixed(1).padStart(6) + ' s  peak ' + r.peakDb.toFixed(2) + ' dBFS  rms ' + r.rmsDb.toFixed(2) + ' dBFS  ' + r.lufs.toFixed(2) + ' LUFS  ' + r.tp.toFixed(2) + ' dBTP');
    }
    console.log('rb338 demos: wrote ' + files.size + ' files to ' + path.relative(ROOT, OUT_DIR));
}

if (process.argv[1] && pathToFileURL(path.resolve(process.argv[1])).href === import.meta.url) {
    main().catch((e) => { console.error(e); process.exit(1); });
}
