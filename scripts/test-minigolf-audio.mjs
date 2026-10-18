import assert from 'node:assert/strict';
import { createHash, webcrypto } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import vm from 'node:vm';
import { pathToFileURL, fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const script = path.join(root, 'scripts/build-minigolf-music.mjs');
const sourceDir = path.join(root, 'assets/minigolf/music');
const outputDir = path.join(root, 'ui/3d/minigolf/music');

function runBuild() {
  const result = spawnSync(process.execPath, [script, '--write'], { cwd: root, encoding: 'utf8' });
  assert.equal(result.status, 0, result.stderr || result.stdout);
}

runBuild();
const firstOutput = fs.readdirSync(outputDir).sort().map(file => [file, fs.readFileSync(path.join(outputDir, file))]);
runBuild();
for (const [file, bytes] of firstOutput) assert.deepEqual(fs.readFileSync(path.join(outputDir, file)), bytes, `${file} is deterministic`);

const manifest = JSON.parse(fs.readFileSync(path.join(outputDir, 'manifest.json'), 'utf8'));
assert.equal(manifest.version, 1);
assert.deepEqual(manifest.tracks.map(track => [track.id, track.tempo, track.trackCount, track.noteCount]), [
  ['a-simple-trifle', 170.0000850000425, 2, 152],
  ['piano-emotional-solo-139', 120, 1, 341],
  ['crystal-tune-bright-1', 120, 12, 706]
]);
for (const track of manifest.tracks) {
  const bytes = fs.readFileSync(path.join(sourceDir, track.original));
  assert.equal(createHash('sha256').update(bytes).digest('hex'), track.sha256, `${track.id} source hash`);
  assert.equal(track.license, 'CC0');
  const project = JSON.parse(fs.readFileSync(path.join(outputDir, track.project), 'utf8'));
  assert.ok(project.tracks.length >= 1 && project.tracks.length <= 16, `${track.id} remains inside the MIDI track limit`);
  const ids = [
    ...project.tracks.map(item => item.id),
    ...project.tracks.flatMap(item => item.clips.flatMap(clip => [clip.id, ...clip.notes.map(note => note.id)]))
  ];
  assert.equal(new Set(ids).size, ids.length, `${track.id} IDs are stable and unique`);
  assert.ok(project.tracks.flatMap(item => item.clips).reduce((sum, clip) => sum + clip.notes.length + clip.controllers.length, 0) <= 50_000,
    `${track.id} remains inside the MIDI event limit`);
}

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
mutedSynth.dispose();

const oldSynthStudioAudio = globalThis.SynthStudioAudio;
const oldSynthStudioVoices = globalThis.SynthStudioVoices;
const oldFetch = globalThis.fetch;
const players = [];
const playerOptions = [];
let fakeContexts = 0;
function fakeAudioEngine(options) {
  playerOptions.push(options);
  const player = {
    volume: null, tick: 0, playCalls: [], stopCalls: 0, noteCalls: 0, releaseCalls: 0, unlockCalls: 0, disposed: false,
    setVolume(value) { this.volume = value; },
    async unlock() { this.unlockCalls++; fakeContexts++; return true; },
    currentTick() { return this.tick; },
    play(project, tick, _onTick, onEnd) { this.playCalls.push({ project, tick, onEnd }); this.tick = tick; return Promise.resolve(); },
    stop() { this.stopCalls++; return this.tick; },
    noteOn() { this.noteCalls++; return () => { this.releaseCalls++; }; },
    dispose() { this.disposed = true; }
  };
  players.push(player);
  return player;
}
globalThis.SynthStudioAudio = { create: fakeAudioEngine };
globalThis.SynthStudioVoices = { DEFAULT_PARAMS: { tone: 0.5, attack: 0.01, release: 0.3, reverb: 0, delay: 0 } };
const demoProject = { version: 1, tempo: 120, ppq: 480, tracks: [] };
const demoManifest = { version: 1, tracks: [
  { title: 'One', project: 'one.project.json' },
  { title: 'Two', project: 'two.project.json' }
] };
const successfulFetch = async url => ({
  ok: true, status: 200,
  async text() { return JSON.stringify(String(url).endsWith('manifest.json') ? demoManifest : demoProject); }
});
globalThis.fetch = successfulFetch;
const { createAudio } = await import(`${pathToFileURL(path.join(root, 'ui/js/desktop/apps/minigolf-audio.js')).href}?test=${Date.now()}`);
const requested = [];
let selectedTrack;
let preGestureFetches = 0;
globalThis.fetch = async () => { preGestureFetches++; throw new Error('visibility resume must not load music'); };
const preGestureAudio = createAudio();
const preGesturePlayers = players.slice(-2);
assert.equal(await preGestureAudio.resume(), false, 'visibility resume cannot unlock audio before a user gesture');
assert.equal(preGestureAudio.getState().unlocked, false);
assert.equal(preGestureFetches, 0, 'visibility resume does not fetch the playlist');
assert.equal(fakeContexts, 0, 'visibility resume does not create/resume an audio context');
assert.deepEqual(preGesturePlayers.map(player => player.unlockCalls), [0, 0]);
preGestureAudio.dispose();
const workingEngine = globalThis.SynthStudioAudio;
globalThis.SynthStudioAudio = { create: () => ({ setVolume() {}, async unlock() { return false; }, dispose() {} }) };
let falseUnlockFetches = 0;
globalThis.fetch = async () => { falseUnlockFetches++; throw new Error('a false unlock result must not load music'); };
const falseUnlockAudio = createAudio();
assert.equal(await falseUnlockAudio.unlock(), false, 'fulfilled false audio unlock is not readiness');
assert.equal(falseUnlockAudio.getState().unlocked, false);
assert.equal(falseUnlockFetches, 0);
falseUnlockAudio.dispose();
globalThis.SynthStudioAudio = workingEngine;
globalThis.fetch = successfulFetch;
const controller = createAudio({
  assetURL: value => { requested.push(value); return `/versioned${value}`; },
  onTrack: track => { selectedTrack = track; }
});
const [musicPlayer, effectsPlayer] = players.slice(-2);
assert.deepEqual(playerOptions.slice(-2), [{ lookAhead: 0.75 }, undefined], 'only Minigolf music opts into the extended scheduler reserve');
assert.equal(musicPlayer.volume, 0.2);
assert.equal(effectsPlayer.volume, 0.65);
assert.equal(await controller.unlock(), true);
assert.deepEqual(requested, [
  '/3d/minigolf/music/manifest.json',
  '/3d/minigolf/music/one.project.json',
  '/3d/minigolf/music/two.project.json'
]);
assert.equal(musicPlayer.playCalls.length, 1, 'playlist starts after the first gesture unlock');
assert.equal(selectedTrack.title, 'One');
assert.equal('projectData' in selectedTrack, false, 'track callbacks do not expose the large imported project');
musicPlayer.tick = 345;
controller.effect('win');
assert.equal(controller.pause(), 345, 'pause preserves the current MIDI tick');
await new Promise(resolve => setTimeout(resolve, 15));
assert.equal(effectsPlayer.noteCalls, 0, 'pause clears scheduled effects before they can sound');
assert.equal(musicPlayer.playCalls.length, 1, 'pause leaves no automatic music scheduler');
assert.equal(controller.effect('strike'), false, 'paused audio suppresses effects');
assert.equal(await controller.resume(), true);
assert.equal(musicPlayer.playCalls.at(-1).tick, 345, 'resume continues at the saved position');
controller.effect('strike');
await new Promise(resolve => setTimeout(resolve, 10));
assert.equal(effectsPlayer.noteCalls, 2, 'strike layers locally synthesized notes');
const realClearTimeout = globalThis.clearTimeout;
let clearedEffectTimers = 0;
globalThis.clearTimeout = timer => { clearedEffectTimers++; return realClearTimeout(timer); };
for (let index = 0; index < 12; index++) controller.effect('strike');
await new Promise(resolve => setTimeout(resolve, 10));
globalThis.clearTimeout = realClearTimeout;
assert.ok(clearedEffectTimers >= 2, 'evicting the oldest captured effect clears its timer');
const noteCountBeforeStoppedRolling = effectsPlayer.noteCalls;
assert.equal(controller.rolling(0), false, 'zero speed stops rolling pulses');
assert.equal(effectsPlayer.noteCalls, noteCountBeforeStoppedRolling);
assert.equal(controller.getState().rollingSpeed, 0);
musicPlayer.playCalls.at(-1).onEnd();
assert.equal(controller.getState().trackIndex, 1, 'finishing a track advances the playlist');
assert.equal(controller.getState().track, 'Two');
assert.equal(controller.getState().playing, true, 'the next playlist track is playing');
musicPlayer.playCalls.at(-1).onEnd();
assert.equal(controller.getState().trackIndex, 0, 'playlist loops after its last track');
assert.equal(controller.getState().playing, true, 'the looped track is reported as playing');
const activeBeforePause = controller.getState().activeEffects;
const releasesBeforePause = effectsPlayer.releaseCalls;
controller.pause();
assert.equal(effectsPlayer.releaseCalls - releasesBeforePause, activeBeforePause, 'pause releases every active effect voice');
assert.equal(controller.getState().activeEffects, 0);
const priorPlayCount = musicPlayer.playCalls.length;
controller.dispose();
assert.equal(musicPlayer.disposed, true);
assert.equal(effectsPlayer.disposed, true);
assert.equal(await controller.unlock(), false);
assert.equal(await controller.resume(), false);
assert.equal(controller.nextTrack(), false);
assert.equal(controller.effect('win'), false);
assert.equal(musicPlayer.playCalls.length, priorPlayCount, 'disposed audio cannot restart playback');

let finishManifest;
globalThis.fetch = url => String(url).endsWith('manifest.json')
  ? new Promise(resolve => { finishManifest = resolve; })
  : Promise.resolve({ ok: true, status: 200, async text() { return JSON.stringify(demoProject); } });
const disposedDuringFetch = createAudio();
const pendingUnlock = disposedDuringFetch.unlock();
while (!finishManifest) await new Promise(resolve => setImmediate(resolve));
disposedDuringFetch.dispose();
finishManifest({ ok: true, status: 200, async text() { return JSON.stringify(demoManifest); } });
assert.equal(await pendingUnlock, false);
assert.equal(players.at(-2).playCalls.length, 0, 'disposal fences a late playlist response');

globalThis.SynthStudioAudio = oldSynthStudioAudio;
globalThis.SynthStudioVoices = oldSynthStudioVoices;
globalThis.fetch = oldFetch;
await import('./test-synth-studio-voices-presets.mjs');
await import('./test-synth-studio-scheduler.mjs');
console.log('Minigolf audio source, limits, gain, playlist lifecycle, effects and disposal checks passed.');
