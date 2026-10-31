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
  setValueAtTime(value, time) { this.value = value; this.events.push(['set', value, time]); }
  setTargetAtTime(value, time, constant) { this.value = value; this.events.push(['target', value, time, constant]); }
  linearRampToValueAtTime(value, time) { this.value = value; this.events.push(['ramp', value, time]); }
  exponentialRampToValueAtTime(value, time) { this.value = value; this.events.push(['exp', value, time]); }
}

class FakeNode {
  constructor() {
    this.gain = new FakeParam();
    for (const name of ['threshold', 'knee', 'ratio', 'attack', 'release']) this[name] = new FakeParam();
    this.outputs = [];
  }
  // A real browser rejects a missing destination, so the fake does too.
  connect(destination) {
    if (!destination || typeof destination !== 'object') {
      throw new TypeError("Failed to execute 'connect' on 'AudioNode': Overload resolution failed.");
    }
    this.outputs.push(destination);
  }
  disconnect() { this.outputs.length = 0; }
}

// Walks the patched audio graph so the tests can prove that a signal really
// reaches the output. A builder that connects to a node the processor does not
// own yet silences the whole insert in the browser.
function reachable(from, to, seen = new Set()) {
  if (!from || !to) return false;
  if (from === to) return true;
  if (seen.has(from)) return false;
  seen.add(from);
  for (const next of from.outputs || []) if (reachable(next, to, seen)) return true;
  return false;
}

class FakeAudioContext {
  constructor() { this.currentTime = 0; this.sampleRate = 44100; this.state = 'suspended'; this.destination = new FakeNode(); this.nodes = []; }
  createGain() { const node = new FakeNode(); this.nodes.push(node); return node; }
  createDynamicsCompressor() { return new FakeNode(); }
  createOscillator() { const node = new FakeNode(); node.type = 'sine'; node.frequency = new FakeParam(440); node.detune = new FakeParam(0); node.start = () => {}; node.stop = () => {}; return node; }
  createDelay() { const node = new FakeNode(); node.delayTime = new FakeParam(0); return node; }
  createBiquadFilter() { const node = new FakeNode(); node.type = 'lowpass'; node.frequency = new FakeParam(350); node.Q = new FakeParam(1); node.detune = new FakeParam(0); return node; }
  createWaveShaper() { const node = new FakeNode(); node.curve = null; node.oversample = 'none'; return node; }
  createConvolver() { const node = new FakeNode(); node.buffer = null; return node; }
  createStereoPanner() { const node = new FakeNode(); node.pan = new FakeParam(0); return node; }
  createChannelSplitter() { return new FakeNode(); }
  createChannelMerger() { return new FakeNode(); }
  createPeriodicWave() { return {}; }
  createBufferSource() { const node = new FakeNode(); node.buffer = null; node.loop = false; node.start = () => {}; node.stop = () => {}; return node; }
  createAnalyser() {
    const node = new FakeNode();
    node.fftSize = 2048;
    node.frequencyBinCount = 1024;
    node.smoothingTimeConstant = 0.75;
    node.getFloatTimeDomainData = buffer => { for (let i = 0; i < buffer.length; i++) buffer[i] = Math.sin(i / 12); };
    node.getByteFrequencyData = buffer => { for (let i = 0; i < buffer.length; i++) buffer[i] = i % 255; };
    return node;
  }
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
loadClassic('ui/js/desktop/apps/synth-studio-fx.js');
loadClassic('ui/js/desktop/apps/synth-studio-audio.js');
const synth = audioSandbox.SynthStudioAudio.create();
await synth.unlock();
assert.equal(contexts[0].nodes[0].gain.value, 1, 'the app volume multiplier starts at unity');
assert.equal(contexts[0].nodes[1].gain.value, 0.78, 'the master strip keeps its default level');
assert.ok(Array.isArray(synth.spectrum(32)) && synth.spectrum(32).length === 32, 'the master analyser reports a spectrum');
synth.dispose();
const mutedSynth = audioSandbox.SynthStudioAudio.create();
assert.equal(mutedSynth.setVolume(0), 0);
await mutedSynth.unlock();
assert.equal(contexts[1].nodes[0].gain.value, 0, 'pre-unlock volume zero mutes the master bus');
mutedSynth.setVolume(0.4);
assert.ok(Math.abs(contexts[1].nodes[0].gain.value - 0.4) < 1e-9, 'volume changes ramp the existing master gain');
assert.equal(contexts[1].nodes[0].gain.events.at(-1)[0], 'target');
const mixTrack={id:'large-timeline',params:{tone:.4},get clips(){throw Error('Mixer must not read timeline events');}};
mutedSynth.updateTracks({tempo:120,ppq:480,tracks:[mixTrack],master:{volume:.6,echoSync:'1/8'}});
mutedSynth.dispose();

// Every insert has to route both the dry and the wet path from its input to its
// output. A builder that connects to a node the processor does not own yet
// silences the whole insert in a real browser.
const FX=audioSandbox.SynthStudioFX;
const insertTypes=FX.types();
assert.ok(insertTypes.length>=12,'the effect rack offers at least twelve insert types');
for (const type of insertTypes) {
  const ctx=new RecordingAudioContext();
  const processor=FX.createInsert(ctx,{type,mix:.5});
  assert.ok(processor,'the insert '+type+' builds a processor');
  assert.ok(reachable(processor.input,processor.output),'the insert '+type+' routes its dry path to the output');
  assert.ok(reachable(processor.input,processor.wet),'the insert '+type+' routes its wet path');
}
const stripContext=new RecordingAudioContext();
const strip=FX.createMaster(stripContext,undefined);
assert.ok(reachable(strip.input,strip.output),'the master strip routes the mix to its output');
const sends=FX.createSends(stripContext,undefined,.5);
assert.ok(reachable(sends.reverbIn,sends.plateGain),'the plate send reaches the master bus');
assert.ok(reachable(sends.delayIn,sends.echoGain),'the echo send reaches the master bus');

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
