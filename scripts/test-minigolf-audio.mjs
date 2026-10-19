import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath, pathToFileURL } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const audioDir = path.join(root, 'ui/3d/minigolf/audio');
const appsDir = path.join(root, 'ui/js/desktop/apps');
const load = name => import(pathToFileURL(path.join(appsDir, name)).href);

// The committed pack must verify before anything else is trusted.
const check = spawnSync(process.execPath, [path.join(root, 'scripts/build-minigolf-audio.mjs'), '--check'], { cwd: root, encoding: 'utf8' });
assert.equal(check.status, 0, check.stderr || check.stdout);
const manifest = JSON.parse(fs.readFileSync(path.join(audioDir, 'manifest.json'), 'utf8'));
const sampleCount = Object.values(manifest.sfx).flat().length;
const fileBySha = new Map(manifest.files.map(file => [file.sha256, file]));

const { AUDIO_CREDITS, AUDIO_MANIFEST_PATH, REQUIRED_GROUPS, WORLD_ALIAS, createAudio } = await load('minigolf-audio.js');
const { REVERB_PRESETS, createImpulse, mulberry32 } = await load('minigolf-audio-synth.js');
assert.equal(AUDIO_MANIFEST_PATH, '/3d/minigolf/audio/manifest.json');
assert.deepEqual(WORLD_ALIAS, { garden: 'temple', coast: 'pirate', night: 'moon' });
assert.deepEqual([...REQUIRED_GROUPS].sort(), Object.keys(manifest.sfx).sort(), 'REQUIRED_GROUPS are exactly the manifest groups');
for (const group of REQUIRED_GROUPS) assert.ok(manifest.sfx[group].length > 0, `${group} has samples`);
assert.deepEqual(manifest.music.map(track => track.id), ['menu', 'pirate', 'temple', 'moon']);
assert.deepEqual(new Set(AUDIO_CREDITS.map(credit => credit.author)), new Set(manifest.sources.map(source => source.author)));
for (const source of manifest.sources) {
  assert.ok(AUDIO_CREDITS.some(credit => credit.author === source.author && credit.title.includes(source.title)), `${source.id} is credited`);
}
for (const file of ['minigolf-audio.js', 'minigolf-audio-engine.js', 'minigolf-audio-music.js', 'minigolf-audio-synth.js']) {
  assert.doesNotMatch(fs.readFileSync(path.join(appsDir, file), 'utf8'), /synth-?studio/i, `${file} does not depend on Synth Studio`);
}

// ---- Fake Web Audio ----
class Param {
  constructor(value = 0) { this.value = value; this.events = []; }
  setValueAtTime(value, time) { this.value = value; this.events.push(['set', value, time]); return this; }
  setTargetAtTime(value, time, constant) { this.value = value; this.events.push(['target', value, time, constant]); return this; }
  linearRampToValueAtTime(value, time) { this.value = value; this.events.push(['ramp', value, time]); return this; }
  cancelScheduledValues(time) { this.events.push(['cancel', time]); return this; }
  last(kind) { return this.events.filter(event => event[0] === kind).at(-1); }
}

class Node {
  constructor(ctx, kind, params = {}) {
    this.ctx = ctx; this.kind = kind; this.out = []; this.disconnects = 0;
    for (const [name, value] of Object.entries(params)) this[name] = new Param(value);
    ctx.nodes.push(this);
  }
  connect(target) { this.out.push(target); return target; }
  disconnect() { this.disconnects++; }
}

class Source extends Node {
  constructor(ctx, kind, params) { super(ctx, kind, params); this.starts = []; this.stops = []; this.onended = null; }
  start(when = 0) { this.starts.push(when); }
  stop(when = 0) { this.stops.push(when); }
}

class FakeContext {
  static instances = [];
  static refuse = false;
  constructor(options) {
    this.options = options; this.state = 'suspended'; this.currentTime = 0; this.sampleRate = 44100; this.nodes = [];
    this.calls = { resume: 0, suspend: 0, close: 0 };
    this.destination = new Node(this, 'destination');
    FakeContext.instances.push(this);
  }
  get sources() { return this.nodes.filter(node => node.kind === 'buffer'); }
  createGain() { return new Node(this, 'gain', { gain: 1 }); }
  createStereoPanner() { return new Node(this, 'pan', { pan: 0 }); }
  createBiquadFilter() { return new Node(this, 'filter', { frequency: 350, Q: 1 }); }
  createConvolver() { return new Node(this, 'convolver'); }
  createDynamicsCompressor() { return new Node(this, 'limiter', { threshold: 0, knee: 0, ratio: 0, attack: 0, release: 0 }); }
  createBufferSource() { return new Source(this, 'buffer', { playbackRate: 1 }); }
  createOscillator() { return new Source(this, 'osc', { frequency: 440 }); }
  createBuffer(channels, length, sampleRate) {
    const data = Array.from({ length: channels }, () => new Float32Array(length));
    return { numberOfChannels: channels, length, sampleRate, duration: length / sampleRate, getChannelData: index => data[index] };
  }
  async decodeAudioData(bytes) {
    const file = fileBySha.get(createHash('sha256').update(Buffer.from(bytes)).digest('hex'));
    if (!file) throw new Error('unknown audio bytes');
    return { path: file.path, duration: file.seconds, numberOfChannels: 1, length: Math.round(file.seconds * 44100), sampleRate: 44100, getChannelData: () => new Float32Array(1) };
  }
  async resume() { this.calls.resume++; if (!FakeContext.refuse) this.state = 'running'; }
  async suspend() { this.calls.suspend++; this.state = 'suspended'; }
  async close() { this.calls.close++; this.state = 'closed'; }
}
globalThis.AudioContext = FakeContext;

const requests = [];
const toBytes = file => {
  const bytes = fs.readFileSync(file);
  return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
};
function fakeFetch(url, init) {
  requests.push({ url: String(url), init });
  const prefix = '/versioned/3d/minigolf/audio/';
  const file = String(url).startsWith(prefix) ? path.join(audioDir, String(url).slice(prefix.length)) : '';
  if (!file || !fs.existsSync(file)) return Promise.resolve({ ok: false, status: 404 });
  return Promise.resolve({ ok: true, status: 200, text: async () => fs.readFileSync(file, 'utf8'), arrayBuffer: async () => toBytes(file) });
}
globalThis.fetch = fakeFetch;

const settle = (ms = 25) => new Promise(resolve => setTimeout(resolve, ms));
async function until(condition, label) {
  for (let i = 0; i < 2000; i++) {
    if (condition()) return;
    await settle(1);
  }
  assert.fail(`timed out waiting for ${label}`);
}

function make(seed = 1) {
  const errors = [];
  const tracks = [];
  const audio = createAudio({ assetURL: value => `/versioned${value}`, onTrack: track => tracks.push(track), onError: error => errors.push(error), random: mulberry32(seed) });
  return { audio, errors, tracks };
}

// ---- Impulse responses are deterministic and decay ----
const impulseContext = new FakeContext();
const energy = (data, from, to) => { let sum = 0; for (let i = from; i < to; i++) sum += data[i] * data[i]; return sum; };
const impulses = {};
for (const id of ['menu', 'pirate', 'temple', 'moon']) {
  const first = createImpulse(impulseContext, id);
  const second = createImpulse(impulseContext, id);
  impulses[id] = first;
  assert.equal(first.numberOfChannels, 2);
  assert.equal(first.length, Math.round(REVERB_PRESETS[id].seconds * 44100));
  for (const channel of [0, 1]) {
    const data = first.getChannelData(channel);
    assert.deepEqual(data, second.getChannelData(channel), `${id} impulse is deterministic`);
    const tenth = Math.floor(data.length / 10);
    assert.ok(energy(data, data.length - tenth, data.length) < 0.01 * energy(data, 0, tenth), `${id} impulse decays`);
  }
}
assert.notDeepEqual(impulses.menu.getChannelData(0).slice(0, 2000), impulses.temple.getChannelData(0).slice(0, 2000));
FakeContext.instances.length = 0;

// ---- Lifecycle ----
const main = make();
const { audio } = main;
assert.equal(FakeContext.instances.length, 0, 'no AudioContext before a user gesture');
assert.equal(requests.length, 0, 'no request before a user gesture');
assert.equal(await audio.resume(), false, 'resume cannot unlock audio');
for (const result of [audio.ui('click'), audio.rolling(5, 'grass')]) assert.equal(result, false);
assert.equal(FakeContext.instances.length, 0);
assert.equal(requests.length, 0);

FakeContext.refuse = true;
assert.equal(await audio.unlock(), false, 'a context that stays suspended is not unlocked');
assert.equal(requests.length, 0, 'a failed unlock fetches nothing');
assert.equal(main.errors.length, 1);
assert.equal(audio.getState().unlocked, false);
FakeContext.refuse = false;
assert.equal(await audio.unlock(), true, 'the next gesture can retry');
assert.equal(FakeContext.instances.length, 1);
const ctx = FakeContext.instances[0];
assert.deepEqual(ctx.options, { latencyHint: 'balanced' });
assert.equal(requests[0].url, '/versioned/3d/minigolf/audio/manifest.json');
assert.equal(requests[0].init.cache, 'force-cache');
const limiter = ctx.nodes.find(node => node.kind === 'limiter');
assert.deepEqual(['threshold', 'knee', 'ratio', 'attack', 'release'].map(name => limiter[name].last('set')[1]), [-3, 0, 20, 0.003, 0.12]);

const pathOf = source => source.buffer?.path ?? '';
const musicSources = () => ctx.sources.filter(source => source.loop && pathOf(source).startsWith('music/'));
const rollLoops = name => ctx.sources.filter(source => source.loop && pathOf(source) === `sfx/roll-${name}-1.ogg`);
const voices = () => ctx.sources.filter(source => !source.loop && pathOf(source).startsWith('sfx/'));
const liveVoices = () => voices().filter(source => !source.disconnects);
const endAll = () => liveVoices().forEach(source => source.onended());
const oscillators = hz => ctx.nodes.filter(node => node.kind === 'osc' && node.frequency.events.some(event => event[1] === hz));
const down = (node, kind) => { let next = node.out[0]; while (next && next.kind !== kind) next = next.out[0]; return next; };
const rateOf = source => source.playbackRate.last('set')[1];
const gainOf = source => down(source, 'gain').gain.last('set')[1];
const fired = action => { const before = voices().length; const result = action(); return { result, made: voices().slice(before) }; };

await until(() => musicSources().length === 1, 'menu music');
assert.equal(musicSources()[0].buffer.path, 'music/menu.ogg');
assert.deepEqual(main.tracks.at(-1), { id: 'menu', title: 'Happy Clappy Loop', author: 'OwlishMedia' });
await until(() => audio.getState().samples.loaded + audio.getState().samples.failed === sampleCount, 'samples');
assert.deepEqual(audio.getState().samples, { loaded: sampleCount, failed: 0 });
assert.equal(main.errors.length, 1, 'only the refused unlock was reported');

// Music crossfade, aliases and manual override
ctx.currentTime = 10;
audio.setMusic('coast');
await until(() => musicSources().length === 2, 'pirate crossfade');
const [menuSource, pirateSource] = musicSources();
assert.equal(pirateSource.buffer.path, 'music/pirate.ogg');
assert.ok(Math.abs(menuSource.stops.at(-1) - 11.9) < 1e-9, 'old track stops 1.9 s later');
const fadeIn = down(pirateSource, 'gain').gain.last('ramp');
assert.ok(fadeIn[1] === 1 && Math.abs(fadeIn[2] - 11.8) < 1e-9, 'new track fades in over 1.8 s');
assert.equal(main.tracks.at(-1).title, 'Pirate Adventure');
assert.equal(ctx.nodes.filter(node => node.kind === 'convolver').length, 2, 'reverb swaps to a new convolver');
assert.equal(ctx.nodes.filter(node => node.kind === 'convolver')[1].buffer.getChannelData(0).length, Math.round(REVERB_PRESETS.pirate.seconds * 44100));
audio.setMusic('coast');
await settle();
assert.equal(musicSources().length, 2, 'the same context does not restart music');
ctx.currentTime = 15;
assert.deepEqual([audio.getState().trackId, audio.getState().music, audio.getState().track, audio.getState().trackIndex], ['pirate', 'pirate', 'Pirate Adventure', 1]);
assert.ok(Math.abs(audio.getState().position - 5) < 1e-9);
assert.equal(audio.nextTrack(), true);
await until(() => musicSources().length === 3, 'next track');
assert.equal(musicSources()[2].buffer.path, 'music/temple.ogg');
audio.setMusic('coast');
await settle();
assert.equal(musicSources().length, 3, 'a repeated context keeps the manual choice');
audio.setMusic('night');
await until(() => musicSources().length === 4, 'moon');
assert.equal(musicSources()[3].buffer.path, 'music/moon.ogg');
audio.setMusic('nowhere');
assert.equal(audio.getState().music, 'menu', 'unknown contexts select the menu');
audio.setMusic('night');
await settle();
assert.equal(musicSources().length, 4, 'returning to the playing track cancels the pending switch');

// Impacts: material, tier, jitter bounds and variant rotation
ctx.currentTime = 20;
const impact = fired(() => audio.effect('impact', { speed: 8, surface: 'wood' }));
assert.equal(impact.result, true);
assert.equal(impact.made.length, 1);
assert.match(impact.made[0].buffer.path, /^sfx\/impact-wood-heavy-\d\.ogg$/);
assert.ok(Math.abs(rateOf(impact.made[0]) - 1) <= 0.06 + 1e-9, 'rate stays within 6 %');
assert.ok(Math.abs(gainOf(impact.made[0]) - 1) <= 0.15 + 1e-9, 'gain stays within 15 %');
assert.equal(fired(() => audio.effect('impact', { speed: 8, surface: 'wood' })).result, false, 'the per-group cooldown skips a doubled hit');
for (const [options, expected] of [
  [{ speed: 3, surface: 'grass' }, 'impact-soft-light'], [{ speed: 7, surface: 'sand' }, 'impact-soft-heavy'],
  [{ speed: 4, surface: 'metal' }, 'impact-metal-medium'], [{ speed: 8, surface: 'stone' }, 'impact-stone'], [{ speed: 1, surface: 'mystery' }, 'impact-wood-light']
]) {
  ctx.currentTime += 0.1;
  assert.match(fired(() => audio.effect('impact', options)).made[0].buffer.path, new RegExp(`^sfx/${expected}-\\d\\.ogg$`), `${options.surface} ${options.speed}`);
}
ctx.currentTime += 0.1;
const stone = fired(() => audio.effect('impact', { speed: 8, surface: 'stone' })).made[0];
assert.ok(Math.abs(rateOf(stone) - 0.86) <= 0.86 * 0.06 + 1e-9, 'heavy stone is pitched down');
assert.equal(fired(() => audio.effect('impact', { speed: 0.5, surface: 'grass' })).result, false, 'tiny soft landings are silent');
endAll();
const order = [];
for (let i = 0; i < 40; i++) {
  ctx.currentTime += 0.05;
  const [voice] = fired(() => audio.effect('impact', { speed: 3, surface: 'wood' })).made;
  order.push(voice.buffer.path);
  voice.onended();
}
order.forEach((name, i) => { if (i) assert.notEqual(name, order[i - 1], 'no immediate variant repeat'); });
assert.equal(audio.getState().activeEffects, 0, 'ended voices are released');

// Other effects
for (const [type, options, expected] of [
  ['strike', { speed: 12 }, ['strike', 'cup-rim']], ['splash', { speed: 3 }, ['splash']], ['cannon', {}, ['cannon', 'cannon-boom']],
  ['boost', { speed: 10 }, ['boost']], ['teleport', {}, ['teleport']]
]) {
  ctx.currentTime += 1;
  const { made } = fired(() => audio.effect(type, options));
  assert.equal(made.length, expected.length, type);
  expected.forEach(name => assert.ok(made.some(source => new RegExp(`^sfx/${name}-\\d+\\.ogg$`).test(source.buffer.path)), `${type} plays ${name}`));
  for (const source of made) source.onended();
}
ctx.currentTime += 1;
const cup = fired(() => audio.effect('cup')).made;
const rattles = cup.filter(source => source.buffer.path.startsWith('sfx/cup-rattle'));
assert.ok(rattles.length >= 2 && rattles.length <= 4);
rattles.forEach((source, i) => { if (i) assert.ok(source.starts[0] - rattles[i - 1].starts[0] >= 0.07 - 1e-9, 'rattles are spaced'); });
assert.ok(cup.some(source => source.buffer.path.startsWith('sfx/impact-soft-light') && source.starts[0] > rattles.at(-1).starts[0]), 'settles after the last rattle');
const duckTargets = level => ctx.nodes.some(node => node.kind === 'gain' && node.gain.events.some(event => event[0] === 'target' && event[1] === level));
assert.ok(duckTargets(0.35), 'cup ducks the music');
for (const source of cup) source.onended();

// Voice cap
endAll();
for (let i = 0; i < 30; i++) { ctx.currentTime += 0.05; audio.effect('impact', { speed: 3, surface: 'wood' }); }
assert.equal(audio.getState().activeEffects, 20, 'at most 20 simultaneous voices');
assert.ok(voices().filter(source => source.stops.length).length >= 10, 'the oldest voices were stolen');

// Rolling
assert.equal(audio.rolling(0.05, 'grass'), false);
assert.equal(rollLoops('soft').length, 0);
assert.equal(audio.rolling(3, 'grass'), true);
const soft = rollLoops('soft').at(-1);
const hard = rollLoops('hard').at(-1);
const target = (source, kind) => (kind === 'gain' ? down(source, 'gain').gain : kind === 'cutoff' ? down(source, 'filter').frequency : source.playbackRate).last('target')[1];
assert.ok(target(soft, 'gain') > 0 && target(hard, 'gain') === 0, 'grass uses the soft loop');
const slow = [target(soft, 'rate'), target(soft, 'gain'), target(soft, 'cutoff')];
assert.equal(audio.rolling(12, 'grass'), true);
assert.ok(target(soft, 'rate') > slow[0] && target(soft, 'gain') > slow[1] && target(soft, 'cutoff') > slow[2], 'speed raises rate, gain and cutoff');
assert.equal(audio.rolling(12, 'wood'), true);
assert.ok(target(hard, 'gain') > 0 && target(soft, 'gain') === 0, 'wood switches to the hard loop');
assert.equal(rollLoops('soft').length, 1, 'loops are reused while rolling');
assert.equal(audio.rolling(0, 'wood'), false);
assert.ok(soft.stops.length && hard.stops.length, 'rolling stops both loops');
assert.equal(audio.getState().rollingSpeed, 0);
assert.equal(audio.rolling(6, 'sand'), true);
assert.equal(rollLoops('soft').length, 2, 'rolling restarts with fresh loops');

// Ambience, UI sounds, win jingle
audio.setAmbient('night');
assert.equal(audio.getState().ambient, 'moon');
assert.equal(oscillators(55).length, 1);
assert.equal(oscillators(55)[0].starts.length, 1);
ctx.currentTime += 1;
const oscillatorsBefore = ctx.nodes.filter(node => node.kind === 'osc').length;
assert.equal(audio.ui('start'), true);
const blips = ctx.nodes.filter(node => node.kind === 'osc').slice(oscillatorsBefore);
assert.deepEqual(blips.map(node => node.frequency.events[0][1]), [660, 990]);
assert.ok(blips.every(node => node.starts.length === 1));
assert.equal(audio.ui('click'), false, 'UI sounds are throttled');
ctx.currentTime += 1;
const win = fired(() => audio.effect('win')).made;
assert.equal(win[0].buffer.path, 'sfx/win-moon-1.ogg');
assert.ok(duckTargets(0.25), 'win ducks the music');
audio.setMusic('menu');
await until(() => musicSources().length === 5, 'menu music');
ctx.currentTime += 5;
assert.equal(fired(() => audio.effect('win')).made[0].buffer.path, 'sfx/win-moon-1.ogg', 'the win jingle keeps the last world');

// Pause and resume
ctx.currentTime = 100;
const heldPosition = audio.getState().position;
assert.equal(audio.pause(), heldPosition);
await until(() => ctx.calls.suspend === 1, 'suspend');
assert.ok(liveVoices().every(source => source.stops.length), 'pause stops every voice');
assert.ok(rollLoops('soft').every(source => source.stops.length) && rollLoops('hard').every(source => source.stops.length), 'pause stops the rolling loops');
assert.ok(oscillators(55)[0].stops.length && oscillators(55)[0].disconnects > 0, 'pause tears the ambience down');
assert.equal(audio.effect('strike', { speed: 5 }), false);
assert.equal(audio.rolling(5, 'grass'), false);
assert.equal(audio.ui('click'), false);
ctx.currentTime = 130;
assert.equal(audio.getState().position, heldPosition, 'position is frozen while paused');
assert.equal(audio.getState().playing, false);
assert.equal(await audio.resume(), true);
assert.equal(ctx.calls.resume, 3, 'resume wakes the context');
assert.equal(audio.getState().paused, false);
assert.equal(oscillators(55).length, 2, 'ambience restarts after resume');
assert.equal(audio.setEffectsVolume(0), 0);
await until(() => oscillators(55)[1].stops.length, 'ambience fade-out');
assert.equal(audio.effect('strike'), false);
audio.setEffectsVolume(0.5);
assert.equal(oscillators(55).length, 3);
assert.equal(audio.setMusicVolume(2), 1);
audio.setMusicVolume(0.2);

// Effect before the first gesture unlocks audio and plays afterwards
const late = make(3);
assert.equal(late.audio.effect('strike', { speed: 6 }), true);
const lateContext = FakeContext.instances.at(-1);
await until(() => lateContext.sources.some(source => !source.loop && pathOf(source).startsWith('sfx/strike-')), 'pre-unlock effect');
late.audio.dispose();

// Failing samples are reported once per kind
const originalFetch = globalThis.fetch;
globalThis.fetch = (url, init) => {
  if (String(url).endsWith('/sfx/strike-1.ogg')) return Promise.resolve({ ok: true, status: 200, arrayBuffer: async () => new ArrayBuffer(600 * 1024) });
  if (String(url).endsWith('/sfx/cup-rim-1.ogg')) return Promise.resolve({ ok: false, status: 404 });
  return originalFetch(url, init);
};
const flaky = make(4);
assert.equal(await flaky.audio.unlock(), true);
await until(() => flaky.audio.getState().samples.loaded + flaky.audio.getState().samples.failed === sampleCount, 'flaky samples');
assert.deepEqual(flaky.audio.getState().samples, { loaded: sampleCount - 2, failed: 2 });
assert.equal(flaky.errors.length, 1, 'sample failures are reported once');
flaky.audio.dispose();
globalThis.fetch = originalFetch;

// Disposal
audio.dispose();
assert.equal(ctx.state, 'closed');
assert.equal(ctx.calls.close, 1);
assert.equal(await audio.unlock(), false);
assert.equal(await audio.resume(), false);
assert.equal(audio.nextTrack(), false);
assert.equal(audio.effect('win'), false);
assert.equal(audio.getState().disposed, true);
audio.dispose();
assert.equal(ctx.calls.close, 1, 'dispose is idempotent');

// Disposal while the manifest is still loading starts nothing
const manifestResponse = await fakeFetch('/versioned/3d/minigolf/audio/manifest.json');
let finishManifest;
globalThis.fetch = (url, init) => String(url).endsWith('manifest.json') ? new Promise(resolve => { finishManifest = () => resolve(manifestResponse); }) : originalFetch(url, init);
const fenced = make(5);
const pending = fenced.audio.unlock();
await until(() => finishManifest, 'manifest request');
fenced.audio.dispose();
finishManifest();
assert.equal(await pending, false);
const fencedContext = FakeContext.instances.at(-1);
assert.equal(fencedContext.sources.length, 0, 'a late manifest starts no sound');
assert.equal(fencedContext.calls.close, 1);
globalThis.fetch = originalFetch;

console.log('Minigolf audio pack, engine, synthesis, music crossfade, effects, pause and disposal checks passed.');
