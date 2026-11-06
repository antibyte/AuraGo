/* Aura Drift - loads the simulation modules (load-order 1 to 12 of the module
 * list) into a fresh vm context and returns window.AuraDriftCore.
 * The modules are plain browser IIFEs; the context only supplies `window`.
 * Usage: import { loadSim } from './aura-drift-sim-loader.mjs'; const AD = loadSim();
 */
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

export const SIM_MODULES = [
    'core', 'physics', 'track', 'style', 'conditions', 'tricks',
    'features', 'weapons', 'projectiles', 'modes', 'ai', 'race'
].map(name => 'ui/js/desktop/apps/aura-drift-' + name + '.js');

export function loadSim() {
    const sandbox = { window: {}, console };
    vm.createContext(sandbox);
    for (const file of SIM_MODULES) {
        vm.runInContext(readFileSync(path.join(root, file), 'utf8'), sandbox, { filename: file });
    }
    return sandbox.window.AuraDriftCore;
}
