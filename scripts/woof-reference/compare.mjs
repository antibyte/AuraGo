/* Compares the browser port with the numpy reference for one utterance.
 *
 *   node scripts/woof-reference/compare.mjs "text" juno 1
 *
 * The reference dumps come from scripts/woof-reference/dump_reference.py. The script
 * reports where the two runs first differ, so a bug in the port points at the stage
 * that broke: the latents are the speech model, the audio is the decoder. */
import { readFileSync, existsSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import vm from 'node:vm';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
globalThis.self = globalThis;

/* The text side, the model and the worker entry, in that order. */
for (const file of ['aura-drift-woof-text.js', 'aura-drift-woof-runtime.js', 'aura-drift-woof-worker.js']) {
    vm.runInThisContext(readFileSync(path.join(root, 'ui/js/desktop/apps', file), 'utf8'), { filename: file });
}
const R = globalThis.AuraDriftWoofRuntime;

const text = process.argv[2] || "Wave goodbye! That's my exhaust, not the horizon.";
const voice = process.argv[3] || 'juno';
const seed = parseInt(process.argv[4] || '1', 10);

const weights = readFileSync(path.join(root, 'ui/3d/aura-drift/voice/woof_int8.npz'));
const buffer = weights.buffer.slice(weights.byteOffset, weights.byteOffset + weights.byteLength);
const model = new R.Woof(buffer);

model.seed(seed);
const lat = model.latents(text, voice, 0.5, 0.9);
console.log(`js latents: ${lat.length / 32} frames`);
writeFileSync(path.join(root, 'scripts/woof-reference/out/js.latents.bin'), Buffer.from(lat.buffer, lat.byteOffset, lat.byteLength));

const audio = model.decode(lat);
console.log(`js audio: ${audio.length} samples (${(audio.length / 24000).toFixed(2)} s)`);
writeFileSync(path.join(root, 'scripts/woof-reference/out/js.audio.bin'), Buffer.from(audio.buffer, audio.byteOffset, audio.byteLength));

/* Compare with the reference dumps when they are there. */
const outDir = path.join(root, 'scripts/woof-reference/out');
function readFloats(file) {
    const buf = readFileSync(file);
    const copy = new Float32Array(buf.length / 4);
    for (let i = 0; i < copy.length; i++) copy[i] = buf.readFloatLE(i * 4);
    return copy;
}

function compare(name, jsBuf, refPath, shape) {
    if (!existsSync(refPath)) {
        console.log(`${name}: no reference dump, skipped`);
        return;
    }
    const ref = readFloats(refPath);
    const js = new Float32Array(jsBuf.buffer.slice(jsBuf.byteOffset, jsBuf.byteOffset + jsBuf.byteLength));
    const n = Math.min(ref.length, js.length);
    let maxAbs = 0, at = 0;
    for (let i = 0; i < n; i++) { const d = Math.abs(ref[i] - js[i]); if (d > maxAbs) { maxAbs = d; at = i; } }
    let dot = 0, rr = 0, jj = 0;
    for (let i = 0; i < n; i++) { dot += ref[i] * js[i]; rr += ref[i] * ref[i]; jj += js[i] * js[i]; }
    const corr = dot / (Math.sqrt(rr) * Math.sqrt(jj) || 1);
    console.log(`${name}: ref ${ref.length} vs js ${js.length} samples | max abs diff ${maxAbs.toExponential(3)} at ${at} | correlation ${corr.toFixed(6)}`);
    if (shape) console.log(`   first ref: ${Array.from(ref.subarray(0, 6)).map(v => v.toFixed(5)).join(' ')}`);
    if (shape) console.log(`   first js : ${Array.from(js.subarray(0, 6)).map(v => v.toFixed(5)).join(' ')}`);
}

compare('latents', lat, path.join(outDir, 'probe.latents.npy.bin'), true);
compare('audio', audio, path.join(outDir, 'probe.audio.npy.bin'), true);
