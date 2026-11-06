/* Runs the browser port of Underdog Woof in node and prints what it produced, so the
 * numbers can be compared with the numpy reference. Not part of the game. */
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

const t0 = performance.now();
const model = new R.Woof(buffer);
const t1 = performance.now();
console.log(`load: ${(t1 - t0).toFixed(0)} ms`);
console.log(`voices: ${Object.keys(model.speech.voices).join(', ')}  rate ${R.SR}  maxFrames ${model.maxFrames}  fps ${model.fps}`);

const text = process.argv[2] || 'Wave goodbye! That is my exhaust, not the horizon.';
const voice = process.argv[3] || 'juno';
const seed = process.argv[4] !== undefined ? parseInt(process.argv[4], 10) : 1;

const t2 = performance.now();
const audio = model.speak(text, voice, seed);
const t3 = performance.now();
const seconds = audio.length / R.SR;
console.log(`render: ${(t3 - t2).toFixed(2)} s for ${seconds.toFixed(2)} s of audio (${(seconds / ((t3 - t2) / 1000)).toFixed(2)}x realtime)`);

let peak = 0, energy = 0;
for (let i = 0; i < audio.length; i++) { const v = Math.abs(audio[i]); if (v > peak) peak = v; energy += audio[i] * audio[i]; }
console.log(`peak ${peak.toFixed(4)}  rms ${Math.sqrt(energy / audio.length).toFixed(4)}`);

if (process.argv[5]) {
    const out = new Float32Array(audio);
    console.log(`first 12 samples: ${Array.from(out.subarray(0, 12)).map(v => v.toFixed(6)).join(' ')}`);
}
