import assert from 'node:assert/strict';
import { EVENT_TYPES, CUES } from '../ui/js/desktop/apps/pinball/contracts.js';
import { soundFor, cueSoundFor, createVoiceLimiter, panFor, SILENT_EVENTS, MAX_VOICES } from '../ui/js/desktop/apps/pinball/audio/sfx.js';
import { RECIPES } from '../ui/js/desktop/apps/pinball/audio/recipes.js';
import { pickVoice } from '../ui/js/desktop/apps/pinball/audio/voice.js';

const sample = { impact: { speed: 2, material: 'wood' }, magnet: { state: 'grab' }, flipper: { up: true }, launch: { speed: 5 }, spinner: { rate: 20 } };
for (const type of EVENT_TYPES) {
    const snd = soundFor({ type, id: 'x', ball: 0, tick: 7, ...(sample[type] || {}) });
    if (SILENT_EVENTS.includes(type)) { assert.equal(snd, null, type + ' is silent'); continue; }
    assert.ok(snd, type + ' maps to a recipe'); assert.ok(RECIPES[snd.recipe], type + ' -> unknown recipe ' + snd.recipe);
    assert.ok(snd.gain > 0 && snd.gain <= 1 && Number.isFinite(snd.rate) && Math.abs(snd.pan) <= .8);
}
for (const cue of CUES) { const snd = cueSoundFor(cue); assert.ok(snd && RECIPES[snd.recipe], 'cue ' + cue); }
assert.equal(cueSoundFor('nope'), null);

assert.equal(soundFor({ type: 'impact', material: 'metal', speed: 0.14 }), null, 'impact below 0.15 is silent');
assert.ok(soundFor({ type: 'impact', material: 'metal', speed: 0.16 }));
assert.ok(soundFor({ type: 'impact', material: 'wood', speed: 6 }).gain > soundFor({ type: 'impact', material: 'wood', speed: 1 }).gain, 'impacts scale with speed');

assert.equal(panFor(10), .8); assert.equal(panFor(-10), -.8); assert.ok(Math.abs(panFor(.13) - .4) < 1e-9); assert.equal(panFor(undefined), 0);
assert.equal(soundFor({ type: 'bumper', id: 'b1', x: 0.5 }).pan, .8);

const lim = createVoiceLimiter(3);
const a = lim.acquire(5, 10, 0), b = lim.acquire(1, 10, 0), c = lim.acquire(3, 10, 0);
let stopped = 0; b.stop = () => stopped++;
assert.ok(a && b && c);
assert.equal(lim.acquire(0, 10, 0), null, 'lowest priority newcomer is dropped');
assert.ok(lim.acquire(4, 10, 0)); assert.equal(stopped, 1, 'lowest-priority voice was stolen');
assert.equal(lim.size(0), 3); assert.equal(lim.size(11), 0, 'ended voices free their slot');
assert.equal(MAX_VOICES, 24);

const voices = [{ lang: 'de-DE', localService: false }, { lang: 'de-DE', localService: true, name: 'a' }, { lang: 'en-US', localService: true }];
assert.equal(pickVoice(voices, 'de').name, 'a'); assert.equal(pickVoice(voices.slice(0, 1), 'de'), null, 'remote voices are never used'); assert.equal(pickVoice(voices, 'fr'), null);
console.log('pinball audio ok');
