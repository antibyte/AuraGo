import assert from 'node:assert/strict';
import { webcrypto } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

class FakeParam {
  constructor(value = 0) { this.value = value; this.events = []; }
  cancelScheduledValues(time) { this.events.push(['cancel', time]); }
  setTargetAtTime(value, time, constant) { this.value = value; this.events.push(['target', value, time, constant]); }
}

class FakeNode {
  constructor() {
    this.gain = new FakeParam();
    for (const name of ['threshold', 'knee', 'ratio', 'attack', 'release']) this[name] = new FakeParam();
  }
  connect() {}
  disconnect() {}
}

class FakeAudioContext {
  constructor() { this.currentTime = 0; this.sampleRate = 44100; this.state = 'suspended'; this.destination = new FakeNode(); this.nodes = []; }
  createGain() { const node = new FakeNode(); this.nodes.push(node); return node; }
  createDynamicsCompressor() { return new FakeNode(); }
  createBuffer(channels, length) {
    const data = Array.from({ length: channels }, () => new Float32Array(length));
    return { getChannelData: index => data[index] };
  }
  resume() { this.state = 'running'; return Promise.resolve(); }
  close() { this.state = 'closed'; return Promise.resolve(); }
}

const contexts = [];
class RecordingAudioContext extends FakeAudioContext {
  constructor() { super(); contexts.push(this); }
}
const audioSandbox = { console, crypto: webcrypto, TextEncoder, performance, AudioContext: RecordingAudioContext, setTimeout, clearTimeout };
audioSandbox.globalThis = audioSandbox;
audioSandbox.window = audioSandbox;
audioSandbox.self = audioSandbox;
const audioContext = vm.createContext(audioSandbox);
const loadClassic = file => vm.runInContext(fs.readFileSync(path.join(root, file), 'utf8'), audioContext, { filename: file });
loadClassic('ui/js/vendor/synth-studio/gm-data.js');
loadClassic('ui/js/desktop/apps/synth-studio-presets.js');
loadClassic('ui/js/desktop/apps/synth-studio-voices.js');
loadClassic('ui/js/desktop/apps/synth-studio-audio.js');
const synth = audioSandbox.SynthStudioAudio.create();
await synth.unlock();
assert.equal(contexts[0].nodes[0].gain.value, 0.78, 'default synth master level is unchanged');
synth.dispose();
const mutedSynth = audioSandbox.SynthStudioAudio.create();
assert.equal(mutedSynth.setVolume(0), 0);
await mutedSynth.unlock();
assert.equal(contexts[1].nodes[0].gain.value, 0, 'pre-unlock volume zero mutes the master bus');
mutedSynth.setVolume(0.4);
assert.ok(Math.abs(contexts[1].nodes[0].gain.value - 0.312) < 1e-9, 'volume changes ramp the existing master gain');
assert.equal(contexts[1].nodes[0].gain.events.at(-1)[0], 'target');
const mixTrack={id:'large-timeline',params:{tone:.4},get clips(){throw Error('Mixer must not read timeline events');}};
mutedSynth.updateTracks({tempo:120,ppq:480,tracks:[mixTrack]});
mutedSynth.dispose();

const stopped=[],detune=new FakeParam(),voice={sources:[{stop:at=>stopped.push(at),detune}],envelopes:[],releaseSeconds:.3,startTime:0,endTime:Infinity};
audioSandbox.SynthStudioVoices.releaseVoice(voice,4);
audioSandbox.SynthStudioVoices.releaseVoice(voice,2);
assert.equal(voice.releaseAt,2,'a scene can shorten an already scheduled note release');
assert.ok(stopped.at(-1)<2.35,'native source stop is moved to the earlier scene boundary');
audioSandbox.SynthStudioVoices.releaseVoice(voice,3);
assert.equal(voice.releaseAt,2,'a later note-off cannot extend the released voice');
const releasedState=audioSandbox.SynthStudioVoices.makeTrackState();releasedState.voices.add(voice);
audioSandbox.SynthStudioVoices.applyController({},releasedState,null,{},'bend',1,2.1);
assert.equal(detune.events.length,0,'a new scene does not bend a previous release tail');

await import('./test-synth-studio-voices-presets.mjs');
await import('./test-synth-studio-scheduler.mjs');
console.log('Synth Studio audio master level, voices, presets and scheduler checks passed.');
