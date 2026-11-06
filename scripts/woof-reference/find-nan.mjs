/* Finds the first stage that produces a NaN in the port. Development helper. */
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

function firstNaN(arr, limit) {
    const n = Math.min(arr.length, limit || arr.length);
    for (let i = 0; i < n; i++) if (!Number.isFinite(arr[i])) return i;
    return -1;
}
function report(label, arr, limit) {
    const at = firstNaN(arr, limit);
    let min = Infinity, max = -Infinity;
    for (let i = 0; i < Math.min(arr.length, 5000); i++) {
        if (!Number.isFinite(arr[i])) continue;
        if (arr[i] < min) min = arr[i];
        if (arr[i] > max) max = arr[i];
    }
    console.log(`${label.padEnd(34)} len ${String(arr.length).padStart(8)}  nan@ ${at}  range [${min.toFixed(4)}, ${max.toFixed(4)}]`);
}

/* the weights themselves */
report('ada.w', speech.ada.w);
report('ce.w', speech.ce.w);
report('layers[0].0.w', speech.layers[0][0].w);
report('textEmb', speech.textEmb);
report('voices.juno', speech.voices.juno);
report('bos', speech.bos);

/* the prompt */
const ids = R.encodeText("Wave goodbye! That's my exhaust, not the horizon.");
console.log('ids', ids.length, 'first', ids.slice(0, 8).join(','), 'max', Math.max(...ids));
const d = speech.d;
const voice = speech.voices.juno;
const p0 = voice.length / d + ids.length + 1;
console.log('p0', p0);
const x = new Float32Array(p0 * d);
x.set(voice, 0);
for (let i = 0; i < ids.length; i++) {
    const id = ids[i];
    if (id * d + d > speech.textEmb.length) console.log('BAD id', id);
    x.set(speech.textEmb.subarray(id * d, (id + 1) * d), (voice.length / d + i) * d);
}
x.set(speech.bos, (voice.length / d + ids.length) * d);
report('prompt x', x);

for (let layer = 0; layer < speech.L; layer++) {
    speech.prefixBlock(layer, x, p0);
    report(`after prefixBlock ${layer}`, x);
    if (firstNaN(x) >= 0) break;
}
