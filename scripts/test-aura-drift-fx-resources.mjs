import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { loadSim } from './aura-drift-sim-loader.mjs';

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

const AD = loadSim(['fx-resources']), track = AD.buildTrack(AD.TRACKS[0]);
const pool = track.puddles.find(p => p.permanent);
const v = {car: AD.CARS[0], x: pool.x, y: pool.y, psi: Math.atan2(pool.ty,pool.tx), vx: 34, planing: .72};
const race = {track, cars: [{v}], time: 4, state: 'running'}, prints = AD.createSurfacePrints();
const advance = dt => { race.time += dt; prints.update(race, false); };
prints.update(race, false);
const crest = prints.marks.find(m => m.kind === 'wake' && m.alpha > 0);
assert.ok(crest);
const {x, y, width, alpha} = crest;
v.x += 80; v.y += 80; advance(.14);
assert.ok(crest.x === x && crest.y === y && crest.width > width && crest.alpha < alpha, 'A crest spreads at the tyre contact point');
assert.equal(prints.marks.filter(m => m.alpha > 0).length, 1, 'Residual aquaplaning on dry asphalt makes no fresh waves');
advance(.5); assert.ok(prints.marks.every(m => !m.alpha));
v.boost = true; advance(.1);
const lamp = prints.marks[0];
assert.ok(lamp.alpha > .4 && lamp.kind === 'cyan');
v.overdrive = 1; advance(.05); assert.equal(lamp.kind,'amber');
const held = lamp.alpha; advance(0); assert.equal(lamp.alpha,held);
v.boost = false; v.overdrive = 0; advance(.05); assert.ok(lamp.alpha > 0 && lamp.alpha < held);
v.airborne = true; v.boost = true; advance(.1); assert.equal(lamp.alpha,0);
v.airborne = false; advance(.1); prints.update(race,true); assert.ok(prints.marks.every(m => !m.alpha));
console.log('Grounded boost light, water contact, fading and reduced motion passed.');
