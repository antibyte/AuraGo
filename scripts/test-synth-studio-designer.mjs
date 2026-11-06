import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

// Programmable synth designer: browser-local patch bank, catalog registration
// and the seam to the voice engine. DOM-free module surface only; the editor
// DOM is exercised by the browser and UI fixtures.

const store = new Map();
const localStorage = {
    getItem: key => (store.has(key) ? store.get(key) : null),
    setItem: (key, value) => store.set(key, String(value)),
    removeItem: key => store.delete(key)
};
const runtime = { console, localStorage, crypto: globalThis.crypto };
runtime.globalThis = runtime;
runtime.window = runtime;

function loadClassic(file) {
    vm.runInNewContext(fs.readFileSync(file, 'utf8'), runtime, { filename: file });
}

loadClassic('ui/js/vendor/synth-studio/gm-data.js');
loadClassic('ui/js/desktop/apps/synth-studio-presets.js');
loadClassic('ui/js/desktop/apps/synth-studio-designer.js');
const Designer = runtime.SynthStudioDesigner;
const Presets = runtime.SynthStudioPresets;

assert.ok(Designer, 'the designer module exports its namespace');
assert.equal(Designer.PREVIEW_ID, 'user-preview');

// --- storage hygiene: junk is dropped, bounds are enforced ----------------
const junk = [
    { id: 'user-hold-me', name: '\u0007Fette  Wave ', patch: { wave: 'supersaw', level: 99, unison: 5, detune: 5000, subOctave: 2, lfo: { target: 'nonsense', rate: -5, depth: 4 } } },
    { id: 'not-user', name: 'bad prefix', patch: {} },
    { id: 'user-evil-' + 'x'.repeat(200), name: 'overlong id', patch: {} },
    { id: 'user-keep', name: 'Keep', patch: { wave: 'triangle', mod: 'sine', ratio: 2.5, depth: 3, second: 0.5, lfo: { depth: 0.3, rate: 7 } } },
    null,
    'stale string',
    ...Array.from({ length: 40 }, (_, index) => ({ id: `user-fill-${index}`, name: `Fill ${index}`, patch: { wave: 'square' } }))
];
store.set(Designer.PREVIEW_ID === 'user-preview' ? 'aurago:synth-studio:patches:1' : '', JSON.stringify(junk));

const installed = Designer.install();
assert.equal(installed.length, Designer.MAX_PATCHES, 'the bank is capped at the maximum');
Designer.install();
assert.equal(Presets.user.length, installed.length, 'reinstalling the stored bank replaces its catalog entries');
assert.equal(new Set(Presets.user.map(entry => entry.id)).size, installed.length, 'repeated registration does not retain stale user entries');
assert.ok(installed.every(entry => entry.id.startsWith('user-')), 'every id keeps the user- prefix');
const held = Designer.userPatches().find(entry => entry.id === 'user-hold-me');
assert.ok(held, 'a well-formed entry survives');
assert.equal(held.name, 'Fette  Wave', 'control characters are stripped but umlauts stay');
assert.equal(held.patch.wave, 'supersaw');
assert.equal(held.patch.level, 1.5, 'levels clamp at the engine bound');
assert.equal(held.patch.unison, 5);
assert.equal(held.patch.detune, 1200, 'detune clamps at the engine bound');
assert.ok(!held.patch.mod2);
assert.equal(held.patch.subOctave, 2);
assert.equal(held.patch.lfo.target, 'pitch', 'unknown LFO targets fall back');
assert.equal(held.patch.lfo.rate, 0.05, 'negative LFO rates clamp to the lower bound');
assert.equal(held.patch.lfo.depth, 1, 'LFO depth clamps to 0..1');

// --- catalog registration -------------------------------------------------
assert.ok(Presets.presets.some(preset => preset.id === 'user-hold-me'), 'stored synths appear in the catalog list');
assert.equal(Presets.get('user-hold-me').patch.wave, 'supersaw');
assert.equal(Presets.get('user-hold-me').category, 'user');
const melodic = Presets.presets.filter(preset => preset.category !== 'drums');
assert.ok(melodic.some(preset => preset.id === 'user-hold-me'), 'the instrument select sees user synths');

// --- normalization is total ------------------------------------------------
const worst = Designer.normalize({ wave: 42, level: 'nope', unison: 9, mod: 'noise-burst', lfo: 'still not an object' });
assert.equal(worst.wave, 'sawtooth', 'unknown waves fall back to sawtooth');
assert.equal(worst.level, 0.22, 'non numeric levels fall back');
assert.equal(worst.unison, 1, 'unison is either 1, 3 or 5');
assert.equal(worst.mod, '', 'unknown modulators disarm');
assert.equal(worst.lfo.target, 'pitch');
assert.ok(Number.isFinite(worst.attack) && Number.isFinite(worst.cutoff) && Number.isFinite(worst.release));
assert.equal(Designer.normalize(undefined).wave, 'sawtooth', 'a missing patch normalizes to the neutral synth');

// --- compact strips disarmed sections without touching armed ones -----------
const compact = Designer.compact({ wave: 'sine', mod: '', mod2: 'sine', sub: 0, lfo: { depth: 0.4, rate: 5 } });
assert.ok(!('mod' in compact), 'an empty modulator is dropped');
assert.equal(compact.mod2, 'sine', 'an armed modulator stays');
assert.ok(!('sub' in compact), 'a silent sub oscillator is dropped');
assert.equal(compact.lfo.depth, 0.4);
assert.ok(!Designer.compact({ wave: 'sine', lfo: { depth: 0 } }).lfo, 'a depthless LFO is dropped');
assert.equal(JSON.parse(JSON.stringify(compact)).mod2, 'sine', 'compact output stays JSON clean');

// --- describe: factory, GM, drum kits, missing ------------------------------
const factory = Designer.describe('lead-supersaw');
assert.equal(factory.isUser, false);
assert.ok(factory.canEdit);
assert.equal(factory.patch.wave, 'supersaw', 'factory patches start the draft');
factory.patch.wave = 'zapped';
assert.notEqual(Presets.get('lead-supersaw').patch.wave, 'zapped', 'describe returns a clone');
assert.equal(Designer.describe('drums-kit').canEdit, false, 'drum kits cannot be edited as one synth');
assert.equal(Designer.describe('gm-drums').canEdit, false);
assert.equal(Designer.describe('gm-5').canEdit, true, 'GM programs start from the neutral synth');
assert.equal(Designer.describe('user-not-stored-here').canEdit, true, 'a missing user synth can be designed again');
assert.equal(Designer.describe('user-not-stored-here').patch.wave, 'sawtooth');

// --- create / update / remove round-trip ------------------------------------
store.clear();
for (const { id } of Designer.userPatches()) Designer.remove(id);
assert.equal(Designer.userPatches().length, 0, 'the test bank is cleared before the round-trip');
const entry = Designer.create('Leadsäge', { wave: 'supersaw', lfo: { depth: 0.2, target: 'amp' }, mod2: 'square', depth2: 4 });
assert.ok(entry && entry.id.startsWith('user-'), 'create mints a user- id');
assert.equal(Presets.user.length, Designer.userPatches().length, 'creating a synth refreshes the exported user bank without duplicates');
assert.equal(entry.name, 'Leadsäge');
const persisted = JSON.parse(store.get('aurago:synth-studio:patches:1'));
assert.equal(persisted.length, 1);
assert.equal(persisted[0].patch.wave, 'supersaw');
assert.ok(!('mod' in persisted[0].patch), 'disarmed modulators are not stored');
let count = Designer.userPatches().length;
while (count < Designer.MAX_PATCHES) { Designer.create(`Pad ${count}`, {}); count++; }
assert.equal(Designer.create('one too many', {}), null, 'the full bank refuses new synths');

const updated = Designer.update(entry.id, '  Retro Serge  ', { wave: 'triangle', cutoff: 0.2 });
assert.equal(updated.name, 'Retro Serge', 'names trim and keep their length bound');
assert.equal(Presets.get(entry.id).patch.wave, 'triangle', 'updates reach the catalog');
const storedUpdated = JSON.parse(store.get('aurago:synth-studio:patches:1')).find(item => item.id === entry.id);
assert.equal(storedUpdated.patch.cutoff, 0.2);
assert.equal(Designer.update('user-never-saved', 'X', {}), null, 'updating an unknown id fails safely');

assert.equal(Designer.remove(entry.id), true);
assert.equal(Presets.get(entry.id), undefined, 'a removed synth leaves the catalog');
assert.equal(Designer.remove(entry.id), false, 'deleting twice is safe');
const afterDelete = JSON.parse(store.get('aurago:synth-studio:patches:1'));
assert.ok(!afterDelete.some(item => item.id === entry.id), 'deletion persists');
installPreviewChecks();

function installPreviewChecks() {
    // --- runtime preview entries never list --------------------------------
    const before = Presets.presets.length;
    Designer.setPreview({ wave: 'formant', cutoff: 0.4 });
    assert.equal(Presets.get(Designer.PREVIEW_ID).patch.wave, 'formant', 'the preview instrument resolves');
    assert.equal(Presets.presets.length, before, 'runtime previews stay out of the library list');
    assert.equal(Presets.get(Designer.PREVIEW_ID).runtime, true);
    Designer.setPreview(null);
    assert.equal(Presets.get(Designer.PREVIEW_ID), undefined, 'the preview is cleared on demand');

    // --- live patch refresh for a stored user synth -------------------------
    const live = Designer.create('Live Edit', { wave: 'square' });
    Designer.live(live.id, { wave: 'bell', cutoff: 0.3 });
    assert.equal(Presets.get(live.id).patch.wave, 'bell', 'draft edits reach a track that plays this synth');
    assert.equal(JSON.parse(store.get('aurago:synth-studio:patches:1')).find(item => item.id === live.id).patch.wave, 'square', 'live drafts are not persisted');
    Designer.live('user-unstored', { wave: 'bell' });
    assert.equal(Presets.get('user-unstored'), undefined, 'live ignores unknown ids');
}

// --- model seam: user- ids validate, others still throw ----------------------
loadClassic('ui/js/desktop/apps/synth-studio-model.js');
const Model = runtime.SynthStudioModel;
const project = Model.create('T');
Model.addTrack(project, 'user-anything-designed');
Model.validate(project);
assert.throws(() => Model.addTrack(Model.create('T'), 'no-such-instrument'), /Unknown Synth Studio instrument/, 'unknown ids still fail');

// --- voice engine seam: operators follow the stored patch --------------------
loadClassic('ui/js/desktop/apps/synth-studio-fx.js');
loadClassic('ui/js/desktop/apps/synth-studio-voices.js');
const stored = Designer.userPatches()[0];
const operators = runtime.SynthStudioVoices.operatorsFor({ instrument: stored.id, params: {} }, 60);
assert.ok(operators && operators.length, 'a stored synth compiles to operators');
assert.ok(operators.every(operator => typeof operator.w === 'string' && Number.isFinite(operator.v)), 'operators stay numeric');
assert.equal(runtime.SynthStudioVoices.operatorsFor({ instrument: stored.id, params: {} }, 60)[0].w, stored.patch.wave, 'the stored wave drives the voice');
const missing = runtime.SynthStudioVoices.operatorsFor({ instrument: 'user-gone-with-the-wind', params: {} }, 60);
assert.equal(missing[0].w, 'sawtooth', 'a missing user synth falls back to a plain wave');

console.log('Synth Studio designer checks passed.');
