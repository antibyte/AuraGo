import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const modelSandbox = { console };
modelSandbox.window = modelSandbox;
modelSandbox.globalThis = modelSandbox;
vm.runInNewContext(fs.readFileSync(path.join(root, 'ui/js/desktop/apps/synth-studio-model.js'), 'utf8'), modelSandbox);
const Model = modelSandbox.SynthStudioModel;
assert.ok(Model, 'model loads');

// ---- model ---------------------------------------------------------------
function audioProject(version = 4) {
  const project = Model.create('Clips');
  project.version = version;
  const track = Model.addTrack(project, 'audio');
  return { project, track };
}
const audio = { kind: 'rb338', song: Buffer.from('rb338-song').toString('base64'), mode: 'song', start: 0, end: 1920, frames: 88200 };
const clip = (extra = {}) => ({ id: 'ac1', name: 'RB', start: 0, length: 1920, notes: [], controllers: [], audio: { ...audio }, ...extra });

{
  const { project, track } = audioProject();
  assert.equal(project.version, 4, 'adding an audio track lifts the project to v4');
  track.clips.push(clip());
  const valid = Model.validate(project);
  assert.equal(valid.tracks[0].clips[0].audio.kind, 'rb338', 'v4 accepts a valid audio clip and keeps clip.audio');
  assert.equal(Model.upgrade(valid).version, 4, 'upgrade of a v4 project is a no-op');
  assert.equal(Model.clone(valid).tracks[0].clips[0].audio.frames, 88200, 'clone keeps clip.audio');

  const withNotes = structuredClone(project);
  withNotes.tracks[0].clips[0] = clip({ notes: [{ id: 'n1', pitch: 60, start: 0, duration: 120, velocity: 0.8 }] });
  assert.throws(() => Model.validate(withNotes), /audio clip/i, 'an audio clip cannot hold notes');

  const withController = structuredClone(project);
  withController.tracks[0].clips[0] = clip({ controllers: [{ tick: 0, type: 'sustain', value: 1 }] });
  assert.throws(() => Model.validate(withController), /audio clip/i, 'an audio clip cannot hold controllers');

  const plain = structuredClone(project);
  plain.tracks[0].clips[0] = { id: 'p1', name: 'Plain', start: 0, length: 1920, notes: [], controllers: [] };
  assert.throws(() => Model.validate(plain), /audio/i, 'an audio track holds only audio clips');

  const bigSong = structuredClone(project);
  bigSong.tracks[0].clips[0].audio.song = Buffer.alloc(512 * 1024 + 1).toString('base64');
  assert.throws(() => Model.validate(bigSong), /512/, 'the song is limited to 512 KiB');

  const badMode = structuredClone(project);
  badMode.tracks[0].clips[0].audio.mode = 'loop';
  assert.throws(() => Model.validate(badMode), /mode/, 'the mode must be song or pattern');
}
{
  const v3 = Model.create('V3');
  assert.equal(v3.version, 3, 'a new project stays v3');
  const bad = structuredClone(v3);
  const t = Model.addTrack(bad, 'user-test');
  t.instrument = 'audio';
  assert.throws(() => Model.validate(bad), /instrument/i, 'instrument audio is rejected in v3');
  const withAudio = structuredClone(v3);
  Model.addTrack(withAudio, 'user-test').clips.push(clip());
  assert.throws(() => Model.validate(withAudio), /audio|unknown|key/i, 'clip.audio is rejected in v3');
  const full = structuredClone(v3);
  for (let i = 0; i < 15; i++) Model.addTrack(full, 'user-test');
  assert.equal(full.tracks.length, 15, 'the v3 project holds 15 melodic tracks');
  assert.throws(() => Model.addTrack(full, 'audio'), /15 melodic/, 'a project with 15 melodic tracks rejects an audio track');
  assert.equal(full.version, 3, 'a rejected audio addTrack leaves the v3 project at version 3');
}

// ---- scheduling ----------------------------------------------------------
const created = [];
const timers = [];
class FakeParam { constructor(v = 0) { this.value = v; } cancelScheduledValues() {} setValueAtTime(v) { this.value = v; } setTargetAtTime(v) { this.value = v; } }
class FakeNode {
  constructor() { this.gain = new FakeParam(); this.calls = []; }
  connect(target) { this.target = target; return this; }
  disconnect() {}
  start(...args) { this.calls.push(['start', ...args]); }
  stop(...args) { this.calls.push(['stop', ...args]); }
}
class FakeAudioContext {
  constructor() { this.currentTime = 0; this.state = 'suspended'; this.destination = new FakeNode(); sandbox.__ctx = this; }
  resume() { this.state = 'running'; return Promise.resolve(); }
  close() { return Promise.resolve(); }
  createBufferSource() { const node = new FakeNode(); created.push(node); return node; }
}
const buses = new Map();
const voices = {
  DEFAULT_PARAMS: { tone: 0.5, reverb: 0, release: 0 },
  clamp(value, min, max, fallback) { const n = Number(value); return Number.isFinite(n) ? Math.max(min, Math.min(max, n)) : fallback; },
  operatorsFor() { return []; },
  voiceReleaseSeconds() { return 0.008; },
  makeMix() { return { master: { gain: new FakeParam(0.78) } }; },
  createTrackBus(_c, _m, track) { const bus = { input: new FakeNode() }; buses.set(track.id, bus); return bus; },
  updateTrackBus() {},
  updateTrackParameter() {},
  scheduleMetronome() {},
  createVoice() { throw new Error('audio clips must not create voices'); },
  disposeVoice() {},
  releaseVoice() {},
  makeTrackState() { return { voices: new Set(), pending: [], sustain: false, bend: 0, mod: 0 }; },
  applyController() {},
  releaseTrackState() {}
};
const sandbox = {
  console,
  AudioContext: FakeAudioContext,
  setInterval(callback, delay) { const timer = { callback, delay, cleared: false }; timers.push(timer); return timer; },
  clearInterval(timer) { if (timer) timer.cleared = true; }
};
sandbox.window = sandbox;
sandbox.globalThis = sandbox;
const ctx = vm.createContext(sandbox);
ctx.SynthStudioVoices = voices;
vm.runInContext(fs.readFileSync(path.join(root, 'ui/js/desktop/apps/synth-studio-audio.js'), 'utf8'), ctx);
const Audio = ctx.SynthStudioAudio;

const scheduled = audioProject().project;
scheduled.tempo = 120; // 0.5 s per beat, 1 ms per 1 tick = 60/120/480 s
scheduled.tracks[0].clips.push(clip({ start: 1920, length: 1920 }));
const secPerTick = 60 / 120 / 480;

// prepareRender sees exactly one audio event and emits no voice for it.
const prep = Audio.prepareRender(scheduled);
const audioEvents = prep.events.filter(event => event.type === 'audio');
assert.equal(audioEvents.length, 1, 'allEvents emits one audio event per audio clip');
assert.equal(audioEvents[0].tick, 1920, 'the audio event sits at clip.start');
assert.equal(audioEvents[0].track.id, scheduled.tracks[0].id);
assert.equal(prep.info.noteCount, 0, 'audio clips do not count toward the note limits');

const resolverCalls = [];
const buffer = { duration: 10 };
const player = Audio.create({ audioBuffer: c => { resolverCalls.push(c.id); return buffer; } });
player.setVolume(0);

async function run(project, startTick) {
  const before = timers.length;
  const pending = player.play(project, startTick);
  for (let i = 0; i < 12 && timers.length === before; i++) await Promise.resolve();
  const timer = timers.at(-1);
  assert.ok(timer, 'play installs its timer');
  return { pending, timer };
}

created.length = 0;
{
  const { pending, timer } = await run(scheduled, 0);
  sandbox.__ctx.currentTime = 4.5;
  timer.callback();
  assert.equal(created.length, 1, 'one buffer source for the clip');
  const source = created[0];
  const start = source.calls.find(c => c[0] === 'start');
  const stop = source.calls.find(c => c[0] === 'stop');
  assert.ok(start && stop, 'source is started and stopped');
  assert.equal(start[2], 0, 'a clip starting at its own tick has no offset');
  assert.ok(Math.abs((stop[1] - start[1]) - 1920 * secPerTick) < 1e-9, 'stop is clamped to the clip length');
  assert.equal(source.target, [...buses.values()][0].input, 'source feeds the track bus input');
  player.stop();
  await pending;
}

created.length = 0;
{
  // Start in the middle of the clip: it plays with an offset.
  const { pending, timer } = await run(scheduled, 1920 + 480);
  sandbox.__ctx.currentTime = 0.5;
  timer.callback();
  assert.equal(created.length, 1, 'a clip straddling the start tick still plays');
  const start = created[0].calls.find(c => c[0] === 'start');
  const stop = created[0].calls.find(c => c[0] === 'stop');
  assert.ok(Math.abs(start[2] - 480 * secPerTick) < 1e-9, 'the offset equals the elapsed part of the clip');
  assert.ok(Math.abs((stop[1] - start[1]) - 1440 * secPerTick) < 1e-9, 'stop covers only the remaining clip');
  player.stop();
  await pending;
}

created.length = 0;
{
  // Looping: the straddling clip restarts at the loop start with an offset.
  const looped = structuredClone(scheduled);
  looped.loop = { enabled: true, start: 2400, end: 3840 };
  const { pending, timer } = await run(looped, 2400);
  sandbox.__ctx.currentTime = 0.3;
  timer.callback();
  assert.equal(created.length, 1, 'the first loop cycle starts the straddling clip');
  const start = created[0].calls.find(c => c[0] === 'start');
  const stop = created[0].calls.find(c => c[0] === 'stop');
  assert.ok(Math.abs(start[2] - 480 * secPerTick) < 1e-9, 'loop start offset is measured from the clip start');
  assert.ok(Math.abs((stop[1] - start[1]) - 1440 * secPerTick) < 1e-9, 'stop is capped by the loop end');
  player.stop();
  await pending;
}

// ---- stems and MIDI -------------------------------------------------------
{
  const exportCtx = vm.createContext({ console, window: null, TextEncoder, Blob, Uint8Array, Date });
  exportCtx.window = exportCtx;
  exportCtx.globalThis = exportCtx;
  const calls = [];
  exportCtx.SynthStudioModel = Model;
  exportCtx.SynthStudioAudio = { renderWav: async (_p, opts) => { calls.push(opts); return new Blob(['x']); } };
  vm.runInContext(fs.readFileSync(path.join(root, 'ui/js/desktop/apps/synth-studio-export.js'), 'utf8'), exportCtx);
  const resolver = () => null;
  const files = await exportCtx.SynthStudioExport.renderStems(scheduled, { audioBuffer: resolver });
  assert.equal(files.length, 1, 'stems include a track that only holds audio clips');
  assert.equal(calls[0].audioBuffer, resolver, 'the resolver reaches renderWav');
}

console.log('synth studio audio clips ok');
