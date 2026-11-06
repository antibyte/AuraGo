/* Finds where the model load spends its time. Development helper, not part of the game. */
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import vm from 'node:vm';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
globalThis.self = globalThis;

const runtimePath = path.join(root, 'ui/js/desktop/apps/aura-drift-woof-runtime.js');
let source = readFileSync(runtimePath, 'utf8');

/* Wrap the expensive constructors with timers. */
source = source
    .replace('function Speech(st, store, voices) {', 'function Speech(st, store, voices) { const __t0 = Date.now();')
    .replace('this.scale = Math.pow(Dh, -0.5);', 'this.scale = Math.pow(Dh, -0.5); console.log("  speech weights", Date.now() - __t0, "ms");')
    .replace('function Decoder(st, store) {', 'function Decoder(st, store) { const __t0 = Date.now();')
    .replace('this.ops = [];', 'console.log("  decoder tf", Date.now() - __t0, "ms"); const __t1 = Date.now(); this.ops = [];')
    .replace('for (let idx = 0; idx < st.seanet.length; idx++) {', 'for (let idx = 0; idx < st.seanet.length; idx++) {')
    .replace('    }\n        }\n    }\n\n    Decoder.prototype.reset', '    }\n        }\n        console.log("  decoder seanet", Date.now() - __t1, "ms");\n    }\n\n    Decoder.prototype.reset');

vm.runInThisContext(readFileSync(path.join(root, 'ui/js/desktop/apps/aura-drift-woof-text.js'), 'utf8'), { filename: 'aura-drift-woof-text.js' });
vm.runInThisContext(source, { filename: 'aura-drift-woof-runtime.js' });
vm.runInThisContext(readFileSync(path.join(root, 'ui/js/desktop/apps/aura-drift-woof-worker.js'), 'utf8'), { filename: 'aura-drift-woof-worker.js' });
const R = globalThis.AuraDriftWoofRuntime;

const weights = readFileSync(path.join(root, 'ui/3d/aura-drift/voice/woof_int8.npz'));
const buffer = weights.buffer.slice(weights.byteOffset, weights.byteOffset + weights.byteLength);

const t0 = Date.now();
const store = R.makeStore(buffer);
console.log('zip read', Date.now() - t0, 'ms');
const t1 = Date.now();
const model = new R.Woof(buffer);
console.log('total load', Date.now() - t1, 'ms');
