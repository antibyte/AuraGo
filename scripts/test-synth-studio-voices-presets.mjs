import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const runtime = { console };
runtime.globalThis = runtime;
runtime.window = runtime;

function loadClassic(file) {
    vm.runInNewContext(fs.readFileSync(file, 'utf8'), runtime, { filename: file });
}

loadClassic('ui/js/vendor/synth-studio/gm-data.js');
loadClassic('ui/js/desktop/apps/synth-studio-voices.js');

assert.equal(runtime.SynthStudioPresets, undefined, 'Minigolf loads voices without the lazy Synth Studio preset catalog');
const track = { instrument: 'late-custom-lead', params: {} };
const fallback = runtime.SynthStudioVoices.operatorsFor(track, 60);
assert.equal(fallback[0].w, 'sawtooth', 'the voice engine remains usable before presets load');

runtime.SynthStudioPresets = {
    get(id) {
        return id === 'late-custom-lead'
            ? { category: 'leads', patch: { wave: 'triangle', level: 0.73, attack: 0.02, decay: 0.2, sustain: 0.4, release: 0.3 } }
            : undefined;
    }
};

const latePatch = runtime.SynthStudioVoices.operatorsFor(track, 60);
assert.equal(latePatch[0].w, 'triangle');
assert.equal(latePatch[0].v, 0.73);
assert.equal(latePatch[0].a, 0.02);

// Every curated drum preset must own a real kit patch. A note without a patch
// falls back to a generic noise hit, which is not a drum at all.
loadClassic('ui/js/desktop/apps/synth-studio-presets.js');
const catalog = runtime.SynthStudioPresets;
assert.equal(catalog.drums.length, 24, 'the curated kit keeps twenty-four drum sounds');
const isFallback = operators => operators.length === 1 && operators[0].w === 'n0' && operators[0].f === 440 && operators[0].v === 0.25;
for (const drum of catalog.drums) {
    const operators = runtime.SynthStudioVoices.operatorsFor({ instrument: 'drums-kit' }, drum.drumNote);
    assert.ok(operators && operators.length, 'the kit resolves a patch for ' + drum.name);
    assert.ok(!isFallback(operators), drum.name + ' must not fall back to the generic noise hit');
}
assert.equal(runtime.SynthStudioVoices.operatorsFor({ instrument: 'drums-kit' }, catalog.get('drum-tambora').drumNote)[0].f, 96, 'the tambora is a tuned membrane');
assert.equal(runtime.SynthStudioVoices.operatorsFor({ instrument: 'drums-kit' }, catalog.get('drum-wood-block').drumNote)[0].f, 1750, 'the wood block plays the wood block patch');
assert.equal(runtime.SynthStudioVoices.operatorsFor({ instrument: 'drums-kit' }, 63)[0].f, 245, 'note 63 stays the electro conga');
assert.equal(runtime.SynthStudioVoices.operatorsFor({ instrument: 'drums-kit' }, 61)[0].f, 145, 'note 61 is the low timbale');
assert.equal(runtime.SynthStudioVoices.operatorsFor({ instrument: 'drums-kit' }, 62)[0].f, 250, 'note 62 is the high timbale');

// A duplicated case number would silently hide the second patch.
const voiceSource = fs.readFileSync('ui/js/desktop/apps/synth-studio-voices.js', 'utf8');
const drumBlock = voiceSource.slice(voiceSource.indexOf('function customDrumPatch'), voiceSource.indexOf('function customPatch'));
const caseNumbers = [...drumBlock.matchAll(/case\s+(\d+)\s*:/g)].map(match => Number(match[1]));
const duplicates = caseNumbers.filter((value, index) => caseNumbers.indexOf(value) !== index);
assert.deepEqual(duplicates, [], 'the drum kit has no duplicate note cases');

console.log('Synth Studio voices resolve custom patches when presets load after the voice engine');
