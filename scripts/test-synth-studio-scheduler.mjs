import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const scheduledVoices = [];
const timers = [];

class FakeParam {
  constructor(value = 0) { this.value = value; }
  cancelScheduledValues() {}
  setValueAtTime(value) { this.value = value; }
  setTargetAtTime(value) { this.value = value; }
}

class FakeNode {
  constructor() { this.gain = new FakeParam(); this.stopped = []; }
  connect() { return this; }
  disconnect() {}
  stop(at) { this.stopped.push(at); }
}

class FakeAudioContext {
  constructor() {
    this.currentTime = 0;
    this.state = 'suspended';
    this.destination = new FakeNode();
    sandbox.__lastAudioContext = this;
  }
  resume() { this.state = 'running'; return Promise.resolve(); }
  close() { this.state = 'closed'; return Promise.resolve(); }
}

const voices = {
  DEFAULT_PARAMS: { tone: 0.5, release: 0 },
  clamp(value, min, max, fallback) {
    const number = Number(value);
    return Number.isFinite(number) ? Math.max(min, Math.min(max, number)) : fallback;
  },
  operatorsFor() { return []; },
  voiceReleaseSeconds() { return 0.008; },
  makeMix() { return { master: { gain: new FakeParam(0.78) } }; },
  createTrackBus() { return { input: new FakeNode() }; },
  updateTrackBus() {},
  scheduleMetronome() {},
  createVoice(context, _mix, _bus, _state, _track, pitch, _velocity, at) {
    const source = new FakeNode();
    const voice = { sources: [source], voiceGain: new FakeNode(), endTime: at + 0.025 };
    scheduledVoices.push({ pitch, at, observedAt: context.currentTime, voice });
    return voice;
  },
  disposeVoice() {},
  releaseVoice(voice, at) { voice.endTime = at + 0.008; },
  makeTrackState() { return { voices: new Set(), pending: [], sustain: false, bend: 0, mod: 0 }; },
  applyController() {},
  releaseTrackState() {}
};

const sandbox = {
  console,
  AudioContext: FakeAudioContext,
  setInterval(callback, delay) {
    const timer = { callback, delay, cleared: false };
    timers.push(timer);
    return timer;
  },
  clearInterval(timer) { if (timer) timer.cleared = true; }
};
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
const context = vm.createContext(sandbox);
context.SynthStudioVoices = voices;
vm.runInContext(fs.readFileSync(path.join(root, 'ui/js/desktop/apps/synth-studio-audio.js'), 'utf8'), context);

const project = {
  tempo: 120,
  ppq: 480,
  tracks: [{
    id: 'scheduler-test',
    instrument: 'lead-saw',
    volume: 0.8,
    params: { release: 0 },
    clips: [{
      id: 'long-line',
      start: 0,
      length: 2200,
      notes: Array.from({ length: 35 }, (_, index) => ({
        id: `note-${index}`,
        start: index * 60,
        duration: 24,
        pitch: 60 + (index % 12),
        velocity: 0.7
      }))
    }]
  }]
};

async function startPlayer(player, sourceProject = project) {
  const previousTimerCount = timers.length;
  const pending = player.play(sourceProject);
  for (let attempt = 0; attempt < 12 && timers.length === previousTimerCount; attempt++) await Promise.resolve();
  const timer = timers.at(-1);
  assert.ok(timer, 'play installs its scheduler timer');
  return { pending, timer };
}

const defaultPlayer = context.SynthStudioAudio.create();
assert.equal(defaultPlayer.setVolume(0), 0, 'scheduler test keeps its output muted');
await defaultPlayer.unlock();
const defaultRun = await startPlayer(defaultPlayer);
// A delayed UI-thread callback arrives after the established 120 ms buffer.
scheduledVoices.length = 0;
// The fake context is captured by the engine instance; advance it without an
// actual audio device, then invoke the exact callback installed by setInterval.
const firstTimer = timers.at(-1);
const activeContext = sandbox.__lastAudioContext;
// The engine does not expose its context, so the test context constructor
// records it for this silent clock simulation.
assert.ok(activeContext, 'fake audio context was created');
activeContext.currentTime = 0.25;
firstTimer.callback();
const lateDefaultNotes = scheduledVoices.filter(event => event.observedAt === 0.25 && event.at < 0.25);
assert.ok(lateDefaultNotes.length > 0, 'the old 120 ms reserve reproduces late note scheduling after a 250 ms stall');
defaultPlayer.stop();
await defaultRun.pending;
defaultPlayer.dispose();
assert.equal(firstTimer.cleared, true, 'stop clears the scheduler');

scheduledVoices.length = 0;
const denseProject = {
  tempo: 120,
  ppq: 480,
  tracks: [{
    id: 'dense-sequential-test',
    instrument: 'lead-saw',
    volume: 0.8,
    params: { release: 0 },
    clips: [{
      id: 'dense-line',
      start: 0,
      length: 800,
      notes: Array.from({ length: 800 }, (_, index) => ({
        id: `dense-${index}`,
        start: index,
        duration: 1,
        pitch: 60 + (index % 12),
        velocity: 0.4
      }))
    }]
  }]
};
const resilientPlayer = context.SynthStudioAudio.create({ lookAhead: 0.75 });
assert.equal(resilientPlayer.setVolume(0), 0, 'extended lookahead remains silent in this regression');
await resilientPlayer.unlock();
const resilientRun = await startPlayer(resilientPlayer, denseProject);
const resilientAudioContext = sandbox.__lastAudioContext;
const resilientTimer = timers.at(-1);
for (let now = 0.25; now <= 1.5; now += 0.25) {
  resilientAudioContext.currentTime = now;
  resilientTimer.callback();
}
const lateResilientNotes = scheduledVoices.filter(event => event.observedAt > 0 && event.at < event.observedAt);
assert.equal(lateResilientNotes.length, 0, '750 ms reserve covers sustained 250 ms scheduler delays without a note starting in the past');
assert.ok(scheduledVoices.length > 256, 'the extended queue can hold many sequential notes without treating queued notes as overlapping voices');
resilientPlayer.stop();
await resilientRun.pending;
assert.equal(resilientTimer.cleared, true);
assert.ok(scheduledVoices.every(event => event.voice.sources[0].stopped.length > 0), 'stop cancels future queued voices');
resilientPlayer.dispose();

console.log('Synth Studio scheduler reserve, delayed callback, sequential voice budget, and stop checks passed.');

const changing = context.SynthStudioAudio.create();
const loopProject = structuredClone(project); loopProject.loop={enabled:true,start:0,end:1920};
const changingRun=await startPlayer(changing,loopProject);
const clock=sandbox.__lastAudioContext;
clock.currentTime=.25;changingRun.timer.callback();
const positionBefore=changing.currentTick();
const changedProject=structuredClone(loopProject);changedProject.tracks[0].params.tone=.9;
changing.updateTracks(changedProject);
assert.equal(changing.currentTick(),positionBefore,'mix changes preserve the audio clock');
assert.equal(changingRun.timer.cleared,false,'mix changes preserve the transport');
changedProject.tracks[0].clips[0].notes=[{id:'replacement',start:0,duration:120,pitch:91,velocity:.8}];
let applied=0, obsolete=0;
changing.queueProject(changedProject,0,()=>obsolete++);
const boundary=changing.queueProject(changedProject,0,()=>applied++).at;
clock.currentTime=boundary-.05;changingRun.timer.callback();
assert.equal(applied,0,'queued notification waits for audible boundary');
const future=scheduledVoices.filter(v=>v.pitch===91);
assert.equal(future.length,1,'replacement is scheduled once');
assert.ok(Math.abs(future[0].at-boundary)<1e-8,'replacement starts exactly at the queued bar');
clock.currentTime=boundary+.01;changingRun.timer.callback();
assert.equal(applied,1);assert.equal(obsolete,0,'latest queued request wins');
changing.queueProject(loopProject,0,()=>obsolete++);
changing.stop();clock.currentTime+=4;
assert.equal(changingRun.timer.cleared,true);assert.equal(changing.pendingChange(),null);
assert.equal(obsolete,0,'stop discards queued callbacks');
await changingRun.pending;changing.dispose();
console.log('Synth Studio live mix and quantized replacement checks passed.');
