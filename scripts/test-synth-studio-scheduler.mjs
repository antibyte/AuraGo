import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const scheduledVoices = [];
const scheduledAutomation = [];
const releasedRanges=[];
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
  DEFAULT_PARAMS: { tone: 0.5, reverb: 0, release: 0 },
  clamp(value, min, max, fallback) {
    const number = Number(value);
    return Number.isFinite(number) ? Math.max(min, Math.min(max, number)) : fallback;
  },
  operatorsFor() { return []; },
  voiceReleaseSeconds() { return 0.008; },
  makeMix() { return { master: { gain: new FakeParam(0.78) } }; },
  createTrackBus() { return { input: new FakeNode() }; },
  updateTrackBus() {},
  updateTrackParameter(_bus,track,parameter,value,at,cancel){scheduledAutomation.push({id:track.id,parameter,value,at,cancel});},
  scheduleMetronome() {},
  createVoice(context, _mix, _bus, _state, _track, pitch, _velocity, at) {
    const source = new FakeNode();
    const voice = { sources: [source], voiceGain: new FakeNode(), startTime:at, endTime: at + 0.025 };
    scheduledVoices.push({ pitch, at, observedAt: context.currentTime, voice });
    return voice;
  },
  disposeVoice() {},
  releaseVoice(voice, at) { voice.endTime = at + 0.008; },
  makeTrackState() { return { voices: new Set(), pending: [], sustain: false, bend: 0, mod: 0 }; },
  applyController() {},
  releaseTrackState(_context,state,_track,at){releasedRanges.push(at);state.sustain=false;state.pending=[];}
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

const automated=structuredClone(loopProject);automated.tracks[0].automation=[{tick:240,parameter:'tone',value:.2},{tick:480,parameter:'tone',value:.8}];
const automPlayer=context.SynthStudioAudio.create(),automRun=await startPlayer(automPlayer,automated),automClock=sandbox.__lastAudioContext;
scheduledAutomation.length=0;automClock.currentTime=.2;automRun.timer.callback();
assert.ok(scheduledAutomation.some(p=>p.value===.2&&Math.abs(p.at-.295)<1e-9),'automation uses the musical audio clock');
automPlayer.setManual(automated.tracks[0].id,'tone',true);automClock.currentTime=.45;automRun.timer.callback();
assert.ok(!scheduledAutomation.some(p=>p.value===.8),'manual gesture suppresses queued automation');
automClock.currentTime=.55;automPlayer.setManual(automated.tracks[0].id,'tone',false);
assert.ok(scheduledAutomation.some(p=>p.value===.8&&p.cancel),'automation resumes at its current value after release');
const replacement=structuredClone(automated);replacement.tracks[0].automation=[{tick:720,parameter:'tone',value:.4}];automPlayer.setAutomation(replacement);automClock.currentTime=.7;automRun.timer.callback();
assert.ok(scheduledAutomation.some(p=>p.value===.4&&Math.abs(p.at-.795)<1e-9),'recorded automation updates without restarting transport');
const beforeBound=scheduledVoices.length;automPlayer.setStopAfter(1);automClock.currentTime=.95;automRun.timer.callback();automClock.currentTime=1.06;automRun.timer.callback();assert.equal(automPlayer.running(),false,'hard recording bound completes playback');assert.ok(scheduledVoices.slice(beforeBound).every(v=>v.at<1.045),'recording deadline never schedules a new onset at or after the limit');await automRun.pending;automPlayer.dispose();

const late=context.SynthStudioAudio.create(),lateRun=await startPlayer(late,loopProject),lateClock=sandbox.__lastAudioContext;
let oldCallback=0,newCallback=0;const at=late.queueProject(changedProject,0,()=>oldCallback++).at;
lateClock.currentTime=at-.06;lateRun.timer.callback();
const earlier=scheduledVoices.at(-1);assert.equal(earlier.pitch,91);
const newest=structuredClone(changedProject);newest.tracks[0].clips[0].notes[0].pitch=92;
const sameBoundary=late.queueProject(newest,0,()=>newCallback++).at;assert.equal(sameBoundary,at,'last-moment scene replacement keeps the requested bar');
assert.ok(earlier.voice.sources[0].stopped.some(t=>t<=at),'replaced future voice is cancelled before sounding');
lateClock.currentTime=at+.01;lateRun.timer.callback();assert.equal(oldCallback,0);assert.equal(newCallback,1);
assert.equal(scheduledVoices.filter(v=>v.pitch===92&&Math.abs(v.at-at)<1e-9).length,1,'replacement has a single note onset');late.stop();await lateRun.pending;late.dispose();
const firstLate=context.SynthStudioAudio.create(),firstRun=await startPlayer(firstLate,loopProject),firstClock=sandbox.__lastAudioContext;
const nextBar=2.045;firstClock.currentTime=nextBar-.06;firstRun.timer.callback();
assert.ok(releasedRanges.some(at=>Math.abs(at-nextBar)<1e-9),'scene repetition releases sustain before the next cycle');
const reserved=scheduledVoices.filter(v=>v.observedAt===firstClock.currentTime&&v.at>=nextBar);
assert.ok(reserved.length,'the old loop has already reserved the next bar');
const firstRequest=firstLate.queueProject(newest,0);
assert.ok(Math.abs(firstRequest.at-nextBar)<1e-9,'a first late request still uses the immediately next bar');
assert.ok(reserved.every(v=>v.voice.sources[0].stopped.some(t=>t<=nextBar)),'all obsolete reserved notes are cancelled');
firstLate.stop();await firstRun.pending;firstLate.dispose();

const bounded=context.SynthStudioAudio.create(),boundedRun=await startPlayer(bounded,loopProject),boundedClock=sandbox.__lastAudioContext;
let beyondLimit=0;boundedClock.currentTime=.1;bounded.setStopAfter(2);bounded.queueProject(newest,0,()=>beyondLimit++);
const beforeBoundary=scheduledVoices.length;boundedClock.currentTime=1.99;boundedRun.timer.callback();boundedClock.currentTime=2.06;boundedRun.timer.callback();
assert.equal(beyondLimit,0,'scene at the recording deadline never activates');
assert.ok(!scheduledVoices.slice(beforeBoundary).some(v=>v.pitch===92),'recording deadline takes priority over a scene');
await boundedRun.pending;bounded.dispose();
console.log('Synth Studio automation override, recording bound and last-moment scene checks passed.');
