// Goldfish Adventure room songs: YuE2 Studio FLAC masters -> seamless looping Ogg Vorbis.
//
//   node assets/goldfish-adventure/build-songs.mjs --sources <dir containing R01.flac .. R16.flac>
//
// The FLAC masters are NOT committed (hundreds of MB); only the encoded Ogg files are. Steps per room:
// decode to float stereo 48 kHz, trim digital silence/near-silent decay at both ends, bake an equal-power
// loop crossfade (the last 3 s fade out against the first 3 s fading in, so the file wraps without a click),
// apply ONE gain so the decoded Ogg peaks at -8 dBFS (like the Earshot loops), encode at ~104 kbps, then
// record id, path, duration, sha256, source hash, origin and style prompt in audio-manifest.json.
// Requires ffmpeg on PATH. Runtime needs only the resulting Ogg files.
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync, existsSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REPO = path.resolve(HERE, '../..');
const OUT = path.join(REPO, 'ui/img/goldfish-adventure/audio');
const RATE = 48000;
const FADE = 3 * RATE;
const PEAK_DB = -8;
const TRIM_DB = -48;
const BITRATE = '104k';
const WINDOW = 480;
const MARGIN = Math.round(RATE * 0.02);
const ROOMS = Array.from({ length: 16 }, (_, i) => 'R' + String(i + 1).padStart(2, '0'));

function run(command, args, binary = false) {
    const result = spawnSync(command, args, { encoding: binary ? null : 'utf8', maxBuffer: 1 << 30, windowsHide: true });
    if (result.error) throw new Error(`Cannot run ${command}: ${result.error.message}`);
    if (result.status !== 0) throw new Error(`${command} failed: ${args.join(' ')}\n${String(result.stderr || '').slice(-600)}`);
    return binary ? result.stdout : `${result.stderr}\n${result.stdout}`;
}
const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');
const toDb = value => 20 * Math.log10(Math.max(value, 1e-12));

function decode(file) {
    const raw = run('ffmpeg', ['-hide_banner', '-nostdin', '-loglevel', 'error', '-i', file, '-f', 'f32le', '-ac', '2', '-ar', String(RATE), '-'], true);
    const all = new Float32Array(raw.buffer.slice(raw.byteOffset, raw.byteOffset + raw.length - (raw.length % 8)));
    const n = all.length / 2, left = new Float32Array(n), right = new Float32Array(n);
    for (let i = 0; i < n; i++) { left[i] = all[2 * i]; right[i] = all[2 * i + 1]; }
    return [left, right];
}

function audibleRange(chans, thresholdDb) {
    const limit = 10 ** (thresholdDb / 20), n = chans[0].length;
    const loud = from => { let p = 0; for (const ch of chans) for (let i = from; i < Math.min(n, from + WINDOW); i++) p = Math.max(p, Math.abs(ch[i])); return p > limit; };
    let first = 0, last = n;
    while (first < n && !loud(first)) first += WINDOW;
    while (last > first && !loud(last - WINDOW)) last -= WINDOW;
    return [Math.max(0, first - MARGIN), Math.min(n, last + MARGIN)];
}

// Loop of length N - F: body [F, N-F) then the tail [N-F, N) fading out against the head [0, F) fading in.
// The wrap runs from x[F-1] (end of the blend) into x[F] (start of the file): continuous.
function bakeLoop(chans, fade) {
    const n = chans[0].length;
    if (n < fade * 4) throw new Error(`song of ${n} samples is too short for a ${fade} sample crossfade`);
    return chans.map(x => {
        const out = new Float32Array(n - fade), body = n - 2 * fade;
        out.set(x.subarray(fade, fade + body));
        for (let k = 0; k < fade; k++) {
            const w = (Math.PI / 2) * (k + 0.5) / fade;
            out[body + k] = x[n - fade + k] * Math.cos(w) + x[k] * Math.sin(w);
        }
        return out;
    });
}

function f32le(chans, gain) {
    const n = chans[0].length, buf = Buffer.alloc(n * 8);
    for (let i = 0; i < n; i++) { buf.writeFloatLE(chans[0][i] * gain, i * 8); buf.writeFloatLE(chans[1][i] * gain, i * 8 + 4); }
    return buf;
}

function encode(chans, gain, target, tmp) {
    const raw = path.join(tmp, 'in.f32');
    writeFileSync(raw, f32le(chans, gain));
    run('ffmpeg', ['-hide_banner', '-nostdin', '-loglevel', 'error', '-y', '-f', 'f32le', '-ar', String(RATE), '-ac', '2', '-i', raw,
        '-c:a', 'libvorbis', '-b:a', BITRATE, target]);
}

function decodedPeakDb(file) {
    const raw = run('ffmpeg', ['-hide_banner', '-nostdin', '-loglevel', 'error', '-i', file, '-f', 'f32le', '-ac', '2', '-ar', String(RATE), '-'], true);
    const all = new Float32Array(raw.buffer.slice(raw.byteOffset, raw.byteOffset + raw.length - (raw.length % 4)));
    let p = 0;
    for (let i = 0; i < all.length; i++) { const a = Math.abs(all[i]); if (a > p) p = a; }
    return toDb(p);
}

function loudness(file) {
    const text = run('ffmpeg', ['-hide_banner', '-nostdin', '-i', file, '-af', 'ebur128=peak=true', '-f', 'null', '-']);
    const part = text.slice(text.lastIndexOf('Summary:'));
    return Number(/I:\s+(-?[\d.]+) LUFS/.exec(part)?.[1]);
}

// Title and "Style (instrumental)" per room from the YuE2 prompt records.
function prompts() {
    const found = {};
    for (let act = 1; act <= 4; act++) {
        const text = readFileSync(path.join(HERE, `music-templates/yue2-act${act}.md`), 'utf8');
        for (const block of text.split(/^## /m).slice(1)) {
            const id = /^(R\d\d)/.exec(block)?.[1];
            if (!id) continue;
            const fence = label => new RegExp('^' + label + '\\s*\\n```text\\n([^\\n]*)\\n```', 'm').exec(block)?.[1]?.trim();
            found[id] = { title: fence('Title'), style: fence('Style \\(instrumental\\)') };
        }
    }
    return found;
}

function main() {
    const argv = process.argv.slice(2);
    const at = argv.indexOf('--sources');
    if (at < 0 || !argv[at + 1]) { console.error('usage: node build-songs.mjs --sources <dir with R01.flac .. R16.flac>'); process.exit(2); }
    const sources = path.resolve(argv[at + 1]);
    const texts = prompts();
    const tmp = mkdtempSync(path.join(tmpdir(), 'goldfish-songs-'));
    mkdirSync(OUT, { recursive: true });
    const songs = [];
    try {
        for (const room of ROOMS) {
            const flac = path.join(sources, room + '.flac');
            if (!existsSync(flac)) throw new Error('missing source ' + flac);
            if (!texts[room]?.style) throw new Error('missing instrumental style prompt for ' + room);
            const full = decode(flac);
            const [from, to] = audibleRange(full, TRIM_DB);
            const trimmed = full.map(ch => ch.subarray(from, to));
            const looped = bakeLoop(trimmed, FADE);
            let peak = 0;
            for (const ch of looped) for (let i = 0; i < ch.length; i++) peak = Math.max(peak, Math.abs(ch[i]));
            let gain = 10 ** (PEAK_DB / 20) / peak;
            const id = 'song-' + room.toLowerCase(), target = path.join(OUT, id + '.ogg');
            let measured = 0;
            for (let attempt = 0; attempt < 6; attempt++) {
                encode(looped, gain, target, tmp);
                measured = decodedPeakDb(target);
                if (measured <= PEAK_DB + 0.02 && measured >= PEAK_DB - 0.4) break;
                gain *= 10 ** ((PEAK_DB - 0.1 - measured) / 20);
            }
            if (measured > PEAK_DB + 0.02) throw new Error(`${room} peak ${measured} dB above target`);
            const bytes = readFileSync(target);
            const duration = Number((looped[0].length / RATE).toFixed(3));
            const lufs = loudness(target);
            songs.push({
                id, path: 'audio/' + id + '.ogg', room, title: texts[room].title, duration, peak_db: Number(measured.toFixed(2)), lufs,
                loop_crossfade_s: FADE / RATE, trimmed_from_s: Number((from / RATE).toFixed(3)), trimmed_to_s: Number((to / RATE).toFixed(3)),
                source: 'YuE2 Studio (local generation)', style_prompt: texts[room].style,
                source_flac_sha256: sha256(readFileSync(flac)), sha256: sha256(bytes),
            });
            console.log(room, 'dur', duration, 's  peak', measured.toFixed(2), 'dB  LUFS', lufs, ' ', (bytes.length / 1024).toFixed(0), 'KiB');
        }
    } finally { rmSync(tmp, { recursive: true, force: true }); }

    const manifestPath = path.join(HERE, 'audio-manifest.json');
    const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
    // Keep the effect entries; the superseded Earshot loops (bpm/bars entries) and older songs are replaced.
    manifest.assets = manifest.assets.filter(a => !a.id.startsWith('song-') && a.bars === undefined);
    manifest.assets.splice(0, 0, ...songs);
    manifest.songs = { codec: 'Ogg Vorbis', bitrate: BITRATE, stereo: true, sample_rate: RATE, loop_crossfade_s: FADE / RATE, trim_threshold_db: TRIM_DB };
    writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + '\n', { encoding: 'utf8' });
    console.log('Wrote', songs.length, 'songs to audio-manifest.json');
}
main();
