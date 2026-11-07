// Aura Drift race songs: FLAC master -> seamless looping Ogg Vorbis (used by build-aura-drift-audio.mjs --songs).
// Steps: decode to float stereo, trim digital silence at both ends, bake an equal-power loop crossfade (the tail is blended into
// the head, so the file wraps without a click), apply ONE gain per song (-16 LUFS integrated, peak -1 dBFS), encode, then lower
// the gain until the decoded file honours the peak target.
import { spawnSync } from 'node:child_process';
import { writeFileSync, rmSync } from 'node:fs';
import path from 'node:path';

export const SONG_RATE = 48000;
const LUFS_TARGET = -16, PEAK_TARGET_DB = -1;
const WINDOW = 480;                               // 10 ms analysis window for the silence trim
const MARGIN = Math.round(SONG_RATE * 0.02);      // keep 20 ms around the audible part

function run(command, args, binary = false) {
    const result = spawnSync(command, args, { encoding: binary ? null : 'utf8', maxBuffer: 1024 * 1024 * 1024, windowsHide: true });
    if (result.error) throw new Error(`Cannot run ${command}: ${result.error.message}`);
    if (result.status !== 0) throw new Error(`${command} failed (exit ${result.status}): ${args.join(' ')}\n${String(result.stderr || '').trim().split('\n').slice(-6).join('\n')}`);
    return binary ? result.stdout : `${result.stderr}\n${result.stdout}`;
}
const toDb = value => 20 * Math.log10(Math.max(value, 1e-12));

export function decodeFlac(file) {
    const raw = run('ffmpeg', ['-hide_banner', '-nostdin', '-loglevel', 'error', '-i', file, '-f', 'f32le', '-ac', '2', '-ar', String(SONG_RATE), '-'], true);
    const all = new Float32Array(raw.buffer.slice(raw.byteOffset, raw.byteOffset + raw.length - (raw.length % 8)));
    const n = all.length / 2, left = new Float32Array(n), right = new Float32Array(n);
    for (let i = 0; i < n; i++) { left[i] = all[2 * i]; right[i] = all[2 * i + 1]; }
    return [left, right];
}

// First and last audible sample (10 ms windows louder than threshold dBFS), with a small margin.
export function audibleRange(chans, thresholdDb) {
    const limit = 10 ** (thresholdDb / 20), n = chans[0].length;
    const loud = from => { let p = 0; for (const ch of chans) for (let i = from; i < Math.min(n, from + WINDOW); i++) p = Math.max(p, Math.abs(ch[i])); return p > limit; };
    let first = 0, last = n;
    while (first < n && !loud(first)) first += WINDOW;
    while (last > first && !loud(last - WINDOW)) last -= WINDOW;
    return [Math.max(0, first - MARGIN), Math.min(n, last + MARGIN)];
}

// Loop file of length N - F: the original [F, N) with its last F samples replaced by the tail [N-F, N) fading out against the
// head [0, F) fading in (equal power). The wrap point then runs from x[F-1] (blend end) straight into x[F] (file start).
export function bakeLoop(chans, fade) {
    const n = chans[0].length;
    if (n < fade * 4) throw new Error(`song of ${n} samples is too short for a ${fade} sample loop crossfade`);
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

export function wavF32(chans, gain, rate = SONG_RATE) {
    const n = chans[0].length, c = chans.length, buf = Buffer.alloc(44 + n * c * 4);
    buf.write('RIFF', 0); buf.writeUInt32LE(36 + n * c * 4, 4); buf.write('WAVEfmt ', 8); buf.writeUInt32LE(16, 16); buf.writeUInt16LE(3, 20); buf.writeUInt16LE(c, 22);
    buf.writeUInt32LE(rate, 24); buf.writeUInt32LE(rate * c * 4, 28); buf.writeUInt16LE(c * 4, 32); buf.writeUInt16LE(32, 34); buf.write('data', 36); buf.writeUInt32LE(n * c * 4, 40);
    let off = 44;
    for (let i = 0; i < n; i++) for (let k = 0; k < c; k++) { buf.writeFloatLE(Math.fround(chans[k][i] * gain), off); off += 4; }
    return buf;
}
const peakOf = chans => { let p = 0; for (const ch of chans) for (let i = 0; i < ch.length; i++) { const a = Math.abs(ch[i]); if (a > p) p = a; } return p; };

export function measureLufs(file) {
    const lufs = Number(/\bI:\s+(-?[\d.]+) LUFS/.exec(run('ffmpeg', ['-hide_banner', '-nostats', '-i', file, '-af', 'ebur128=framelog=quiet', '-f', 'null', '-']))?.[1]);
    if (!Number.isFinite(lufs)) throw new Error(`cannot measure loudness of ${file}`);
    return lufs;
}
export function measureDecodedPeakDb(file) {
    return Number(/Peak level dB: (-?[\d.]+)/.exec(run('ffmpeg', ['-hide_banner', '-nostats', '-i', file, '-af', 'astats=measure_perchannel=none:measure_overall=Peak_level', '-f', 'null', '-']))?.[1]);
}

// settings: { loopFade (s), kbps, trimThresholdDb }. Returns { gainDb, lufs (decoded result), peakDb (decoded), sourceSeconds, trimmed: [from, to] seconds }.
export function encodeSong(flac, dest, tmp, settings) {
    const source = decodeFlac(flac), sourceSeconds = source[0].length / SONG_RATE;
    const [from, to] = audibleRange(source, settings.trimThresholdDb);
    const trimmed = source.map(ch => ch.subarray(from, to));
    const looped = bakeLoop(trimmed, Math.round(settings.loopFade * SONG_RATE));
    const wav = path.join(tmp, `song-${process.hrtime.bigint()}.wav`);
    writeFileSync(wav, wavF32(looped, 1));
    const lufs = measureLufs(wav);
    rmSync(wav);
    let gainDb = Math.min(LUFS_TARGET - lufs, PEAK_TARGET_DB - toDb(peakOf(looped))), decodedPeak = NaN;
    for (let attempt = 0; attempt < 6; attempt++) {
        writeFileSync(wav, wavF32(looped, 10 ** (gainDb / 20)));
        run('ffmpeg', ['-hide_banner', '-nostdin', '-loglevel', 'error', '-y', '-i', wav, '-map_metadata', '-1', '-fflags', '+bitexact', '-flags:a', '+bitexact', '-c:a', 'libvorbis', '-b:a', `${settings.kbps}k`, '-ar', String(SONG_RATE), dest]);
        decodedPeak = measureDecodedPeakDb(dest);
        if (!Number.isFinite(decodedPeak) || decodedPeak <= PEAK_TARGET_DB + 0.15) break;
        gainDb -= decodedPeak - PEAK_TARGET_DB + 0.05;
    }
    rmSync(wav);
    return { gainDb: +gainDb.toFixed(3), lufs: +measureLufs(dest).toFixed(2), peakDb: +decodedPeak.toFixed(2), sourceSeconds: +sourceSeconds.toFixed(3), trimmed: [+(from / SONG_RATE).toFixed(3), +(to / SONG_RATE).toFixed(3)] };
}
