/* Checks the port's random numbers against numpy's mt19937 and torch.randn. */
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

const rng = new R.MT19937(12345);
const words = [];
for (let i = 0; i < 8; i++) words.push(rng.next());
console.log('js  mt19937 words:', words.join(','));

const noise = R.randn(new R.MT19937(12345), 32);
console.log('js  randn[0..5]:', Array.from(noise.subarray(0, 6)).map(v => v.toFixed(8)).join(' '));
console.log('js  randn[16..21]:', Array.from(noise.subarray(16, 22)).map(v => v.toFixed(8)).join(' '));
