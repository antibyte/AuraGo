import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';

const apps = path.resolve(import.meta.dirname, '../ui/js/desktop/apps');
const resources = readFileSync(path.join(apps, 'aura-drift-fx-resources.js'), 'utf8');
const effects = readFileSync(path.join(apps, 'aura-drift-fx.js'), 'utf8');
for (const available of [false, true]) {
    const core = {};
    const context = vm.createContext({ window: { AuraDriftCore: core } });
    if (available) vm.runInContext(resources, context);
    vm.runInContext(effects, context);
    assert.equal(typeof core.createFx, available ? 'function' : 'undefined',
        'A stale loader must safely omit optional FX when their resources are missing');
    if (available) assert.equal(typeof core.createFxResources, 'function');
}
console.log('Aura Drift FX resource loading and optional fallback passed.');
