/* Compares every step of one prefix block against the numpy reference. */
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
    let max = 0, at = 0;
    for (let i = 0; i < n; i++) { const d = Math.abs(a[i] - b[i]); if (d > max) { max = d; at = i; } }
    return { max, at };
}
function cmp(name, file, js) {
    const ref = readNpy(path.join(outDir, file));
    const r = diff(ref.data, js);
    console.log(`${name.padEnd(12)} max ${r.max.toExponential(3)} at ${r.at}` +
        (r.max < 1e-4 ? '' : `   ref ${Array.from(ref.data.subarray(0, 3)).map(v => v.toFixed(5)).join(' ')}` +
            `  js ${Array.from(js.subarray(0, 3)).map(v => v.toFixed(5)).join(' ')}`));
}

const weights = readFileSync(path.join(root, 'ui/3d/aura-drift/voice/woof_int8.npz'));
const buffer = weights.buffer.slice(weights.byteOffset, weights.byteOffset + weights.byteLength);
const model = new R.Woof(buffer);
const speech = model.speech;

const text = "Wave goodbye! That's my exhaust, not the horizon.";
const ids = R.encodeText(text);
const d = speech.d, H = speech.H, Dh = speech.Dh, half = Dh >> 1;
const voice = speech.voices.juno;
const voiceRows = voice.length / d;
const n = voiceRows + ids.length + 1;

const x = new Float32Array(n * d);
x.set(voice, 0);
for (let i = 0; i < ids.length; i++) x.set(speech.textEmb.subarray(ids[i] * d, (ids[i] + 1) * d), (voiceRows + i) * d);
x.set(speech.bos, (voiceRows + ids.length) * d);

const l0 = speech.layers[0][0];
const xn = new Float32Array(n * d);
for (let t = 0; t < n; t++) {
    const o = t * d;
    let ss = 0;
    for (let i = 0; i < d; i++) ss += x[o + i] * x[o + i];
    const inv = 1 / Math.sqrt(ss / d + 1.1920929e-7);
    for (let i = 0; i < d; i++) xn[o + i] = x[o + i] * inv;
}
cmp('xn', 'blk.xn.npy', xn);

const qkv = R.matmulDot(xn, l0.w, n, d, 3 * d);
cmp('qkv', 'blk.qkv.npy', qkv);

/* the rope rotation of q and k */
const q = new Float32Array(n * H * Dh), k = new Float32Array(n * H * Dh);
for (let t = 0; t < n; t++) {
    for (let h = 0; h < H; h++) {
        for (let w = 0; w < 2; w++) {
            const src = ((t * 3 + w) * H + h) * Dh;
            const dst = t * H * Dh + h * Dh;
            const target = w === 0 ? q : k;
            for (let i = 0; i < Dh; i++) {
                const rot = i < half ? qkv[src + i + half] : qkv[src + i - half];
                target[dst + i] = qkv[src + i] * speech.cos[t * Dh + i] + rot * speech.sin[t * Dh + i];
            }
        }
    }
}
cmp('q', 'blk.q.npy', q);
cmp('k', 'blk.k.npy', k);

/* the attention matrix */
const kvStride = speech.kvLen * Dh;
const kBase = 0;
for (let t = 0; t < n; t++) {
    for (let h = 0; h < H; h++) {
        const src = t * H * Dh + h * Dh;
        const dst = kBase + h * kvStride + t * Dh;
        for (let i = 0; i < Dh; i++) speech.kv[dst + i] = k[src + i];
    }
}
const att = new Float32Array(H * n * n);
for (let h = 0; h < H; h++) {
    for (let a = 0; a < n; a++) {
        const qo = (a * H + h) * Dh, ko = kBase + h * kvStride;
        let max = -Infinity;
        const scores = new Float32Array(a + 1);
        for (let b = 0; b <= a; b++) {
            let s = 0;
            for (let i = 0; i < Dh; i++) s += q[qo + i] * speech.kv[ko + b * Dh + i];
            s *= speech.scale;
            scores[b] = s;
            if (s > max) max = s;
        }
        let sum = 0;
        for (let b = 0; b <= a; b++) { const e = Math.exp(scores[b] - max); scores[b] = e; sum += e; }
        const inv = 1 / sum;
        for (let b = 0; b <= a; b++) att[h * n * n + a * n + b] = scores[b] * inv;
    }
}
const attFlat = new Float32Array(n * H * n);
for (let h = 0; h < H; h++) for (let a = 0; a < n; a++) for (let b = 0; b < n; b++) attFlat[a * H * n + h * n + b] = att[h * n * n + a * n + b];
cmp('att', 'blk.att.npy', attFlat);
