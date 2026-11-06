/* Compares the prefix blocks of the port with the numpy reference, layer by layer.
 * Development helper. */
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

const outDir = path.join(root, 'scripts/woof-reference/out');

/* A minimal .npy reader for the float32 dumps of the reference. */
function readNpy(file) {
    const buf = readFileSync(file);
    const headerLen = buf.readUInt16LE(8);
    const header = buf.toString('latin1', 10, 10 + headerLen);
    const m = /'shape'\s*:\s*\(([^)]*)\)/.exec(header);
    const shape = m[1].trim() === '' ? [] : m[1].split(',').map(s => parseInt(s.trim(), 10)).filter(v => Number.isFinite(v));
    const count = shape.reduce((a, b) => a * b, 1);
    const data = new Float32Array(count);
    for (let i = 0; i < count; i++) data[i] = buf.readFloatLE(10 + headerLen + i * 4);
    return { data, shape };
}

function diff(a, b) {
    const n = Math.min(a.length, b.length);
    let max = 0, at = 0, sum = 0;
    for (let i = 0; i < n; i++) {
        const d = Math.abs(a[i] - b[i]);
        sum += d * d;
        if (d > max) { max = d; at = i; }
    }
    return { max, at, rms: Math.sqrt(sum / n) };
}

const weights = readFileSync(path.join(root, 'ui/3d/aura-drift/voice/woof_int8.npz'));
const buffer = weights.buffer.slice(weights.byteOffset, weights.byteOffset + weights.byteLength);
const model = new R.Woof(buffer);
const speech = model.speech;

const prompt = readNpy(path.join(outDir, 'stage.prompt.npy'));
console.log('prompt shape', prompt.shape.join('x'), 'diff', JSON.stringify(diff(prompt.data, prompt.data)));

const d = speech.d;
const ids = R.encodeText("Wave goodbye! That's my exhaust, not the horizon.");
const voice = speech.voices.juno;
const voiceRows = voice.length / d;
const p0 = voiceRows + ids.length + 1;
const x = new Float32Array(p0 * d);
x.set(voice, 0);
for (let i = 0; i < ids.length; i++) x.set(speech.textEmb.subarray(ids[i] * d, (ids[i] + 1) * d), (voiceRows + i) * d);
x.set(speech.bos, (voiceRows + ids.length) * d);
console.log('prompt diff', JSON.stringify(diff(prompt.data, x)));

for (let layer = 0; layer < speech.L; layer++) {
    speech.prefixBlock(layer, x, p0);
    const ref = readNpy(path.join(outDir, `stage.layer${layer}.npy`));
    const r = diff(ref.data, x);
    console.log(`layer ${layer}: max ${r.max.toExponential(3)} at ${r.at}  rms ${r.rms.toExponential(3)}`);
    if (r.max > 1e-3) {
        console.log(`   ref[0..3] ${Array.from(ref.data.subarray(0, 4)).map(v => v.toFixed(5)).join(' ')}` +
            `   js[0..3] ${Array.from(x.subarray(0, 4)).map(v => v.toFixed(5)).join(' ')}`);
        break;
    }
}
