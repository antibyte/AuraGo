/* Compares the first latents of the port with the numpy reference, so the step that
 * breaks can be found. Development helper. */
import { readFileSync } from 'node:fs';
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

const weights = readFileSync(path.join(root, 'ui/3d/aura-drift/voice/woof_int8.npz'));
const buffer = weights.buffer.slice(weights.byteOffset, weights.byteOffset + weights.byteLength);
const model = new R.Woof(buffer);
const speech = model.speech;

const refBuf = readFileSync(path.join(root, 'scripts/woof-reference/out/probe.latents.npy.bin'));
const ref = new Float32Array(refBuf.length / 4);
for (let i = 0; i < ref.length; i++) ref[i] = refBuf.readFloatLE(i * 4);
console.log('reference latents:', ref.length / 32, 'frames');

const text = "Wave goodbye! That's my exhaust, not the horizon.";
const ids = R.encodeText(text);
const d = speech.d;
const voice = speech.voices.juno;
const voiceRows = voice.length / d;
const p0 = voiceRows + ids.length + 1;

model.seed(1);
const rng = model.rng;
const x = new Float32Array(p0 * d);
x.set(voice, 0);
for (let i = 0; i < ids.length; i++) {
    x.set(speech.textEmb.subarray(ids[i] * d, (ids[i] + 1) * d), (voiceRows + i) * d);
}
x.set(speech.bos, (voiceRows + ids.length) * d);
for (let layer = 0; layer < speech.L; layer++) speech.prefixBlock(layer, x, p0);

const last = new Float32Array(d);
for (let i = 0; i < d; i++) last[i] = x[(p0 - 1) * d + i];
let cond = speech.cond(last);
console.log('first cond: e =', cond.e.toFixed(6), ' s[0..3] =', Array.from(cond.s.subarray(0, 4)).map(v => v.toFixed(5)).join(' '));

const step = new Float32Array(d);
for (let f = 0; f < 4; f++) {
    const noise = R.randn(rng, speech.latent);
    for (let i = 0; i < speech.latent; i++) noise[i] *= 0.5;
    const z = speech.head(cond.s, noise);
    for (let i = 0; i < speech.latent; i++) z[i] += noise[i];
    let maxd = 0, at = 0;
    for (let i = 0; i < speech.latent; i++) {
        const dd = Math.abs(z[i] - ref[f * speech.latent + i]);
        if (dd > maxd) { maxd = dd; at = i; }
    }
    console.log(`frame ${f}: js[0..3]=${Array.from(z.subarray(0, 4)).map(v => v.toFixed(5)).join(' ')}` +
        `  ref[0..3]=${Array.from(ref.subarray(f * speech.latent, f * speech.latent + 4)).map(v => v.toFixed(5)).join(' ')}` +
        `  maxdiff ${maxd.toExponential(3)} at ${at}`);
    R.matvecDot(z, speech.latIn.w, d, speech.latent, step);
    if (speech.latIn.b) for (let i = 0; i < d; i++) step[i] += speech.latIn.b[i];
    for (let layer = 0; layer < speech.L; layer++) speech.blockStep(layer, step, p0 + f);
    cond = speech.cond(step);
    console.log(`         cond.e = ${cond.e.toFixed(6)}`);
}
