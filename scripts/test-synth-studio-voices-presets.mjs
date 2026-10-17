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

console.log('Synth Studio voices resolve custom patches when presets load after the voice engine');
