/* Aura Drift - the Underdog Woof port against the numpy reference.
 *
 * The model (ui/3d/aura-drift/voice/woof_int8.npz) and the reference dumps
 * (scripts/woof-reference/out/probe.*.npy.bin) are not in the repository, so the
 * check is skipped when they are missing.
 *
 * Run: node scripts/test-aura-drift-woof-model.mjs
 */
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import vm from 'node:vm';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const out = path.join(root, 'scripts/woof-reference/out');
const modelPath = path.join(root, 'ui/3d/aura-drift/voice/woof_int8.npz');
const latentsPath = path.join(out, 'probe.latents.npy.bin');
const audioPath = path.join(out, 'probe.audio.npy.bin');

if (!existsSync(modelPath) || !existsSync(latentsPath) || !existsSync(audioPath)) {
    console.log('skip the model check: the weights or the reference dumps are not here');
    process.exit(0);
}

/* The line the reference was made with. */
const TEXT = "Wave goodbye! That's my exhaust, not the horizon.";
const VOICE = 'juno';
const SEED = 1;

globalThis.self = globalThis;
/* The text side, the model and the worker entry, in that order. */
for (const file of ['aura-drift-woof-text.js', 'aura-drift-woof-runtime.js', 'aura-drift-woof-worker.js']) {
    vm.runInThisContext(readFileSync(path.join(root, 'ui/js/desktop/apps', file), 'utf8'), { filename: file });
}
const R = globalThis.AuraDriftWoofRuntime;

function readFloats(file) {
    const buf = readFileSync(file);
    const out = new Float32Array(buf.length / 4);
    for (let i = 0; i < out.length; i++) out[i] = buf.readFloatLE(i * 4);
    return out;
}

function correlation(a, b) {
    const n = Math.min(a.length, b.length);
    let dot = 0, aa = 0, bb = 0, max = 0;
    for (let i = 0; i < n; i++) {
        dot += a[i] * b[i];
        aa += a[i] * a[i];
        bb += b[i] * b[i];
        const d = Math.abs(a[i] - b[i]);
        if (d > max) max = d;
    }
    return { corr: dot / (Math.sqrt(aa) * Math.sqrt(bb) || 1), max: max, n: n };
}

const weights = readFileSync(modelPath);
const buffer = weights.buffer.slice(weights.byteOffset, weights.byteOffset + weights.byteLength);
const model = new R.Woof(buffer);
assert.deepEqual(Object.keys(model.speech.voices).sort(), ['juno', 'mira'], 'the two voices of the model');

model.seed(SEED);
const latents = model.latents(TEXT, VOICE, 0.5, 0.9);
const audio = model.decode(latents);

const latentsCheck = correlation(readFloats(latentsPath), latents);
console.log(`latents: ${latentsCheck.n} values | max abs diff ${latentsCheck.max.toExponential(3)} | correlation ${latentsCheck.corr.toFixed(6)}`);
assert.ok(latentsCheck.corr > 0.9999, 'the latents are the same as the numpy reference');

const audioCheck = correlation(readFloats(audioPath), audio);
console.log(`audio: ${audioCheck.n} values | max abs diff ${audioCheck.max.toExponential(3)} | correlation ${audioCheck.corr.toFixed(6)}`);
assert.ok(audioCheck.corr > 0.9999, 'the audio is the same as the numpy reference');

/* The audio has to be real sound, not silence or noise. */
let peak = 0, energy = 0;
for (let i = 0; i < audio.length; i++) {
    const v = Math.abs(audio[i]);
    if (v > peak) peak = v;
    energy += audio[i] * audio[i];
}
const rms = Math.sqrt(energy / audio.length);
console.log(`audio: ${(audio.length / R.SR).toFixed(2)} s | peak ${peak.toFixed(4)} | rms ${rms.toFixed(4)}`);
assert.ok(audio.length > R.SR, 'the line is longer than a second');
assert.ok(peak > 0.05 && peak < 1.0, 'the line has a sane level');
assert.ok(rms > 0.005, 'the line is not silent');

console.log('\nok   the port matches the numpy reference');
