package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runStorymakerAudioScript runs a Node script against the real app modules. The script gets the modules folder as its first
// argument and prints okLine when every check passed.
func runStorymakerAudioScript(t *testing.T, script, okLine string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	file := filepath.Join(t.TempDir(), "storymaker-audio-check.js")
	if err := os.WriteFile(file, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	out, err := exec.Command(node, file, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), okLine) {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

// bindAudio with the real reload and save code of storymaker.js: what refreshes the answer, what reloads the player, what feeds the audio.
func TestStorymakerBindAudioInNode(t *testing.T) {
	t.Parallel()
	runStorymakerAudioScript(t, storymakerBindAudioScript, "storymaker bind audio ok")
}

const storymakerBindAudioScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const dir = process.argv[2];
const context = { console, AbortController, setTimeout, clearTimeout };
context.globalThis = context; context.window = context;
context.document = { documentElement: { lang: 'en' } };
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-audio.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const flush = () => new Promise(resolve => setTimeout(resolve, 0));

const root = fs.readFileSync(path.join(dir, 'storymaker.js'), 'utf8');
const between = (from, to) => {
  const a = root.indexOf(from), b = root.indexOf(to, a);
  if (a < 0 || b < 0) throw new Error('storymaker.js lost ' + from);
  return root.slice(a, b);
};
for (const marker of ['inst.parts.audio = S.bindAudio(inst);', 'reloadPlayer: () => reloadPlayer(inst)', 'if (inst.audio) inst.audio.play(inst.frame);']) {
  if (root.indexOf(marker) < 0) throw new Error('storymaker.js lacks ' + marker);
}
const wiring = vm.runInContext('(function () { const S = window.StorymakerParts; const PREVIEW_DELAY_MS = 250; const paintSave = () => {};\n'
  + between('function bindPlayer(inst, note) {', 'async function requestThumbs(inst) {')
  + between('function afterStorySaved(inst) {', 'function handleSaveError(inst, err) {')
  + 'const controls = inst => ({ ' + between('play: () => {', 'pause: () => {') + '});'
  + '\nreturn { bindPlayer, reloadPlayer, afterStorySaved, controls };\n})()', context);

function harness(projectId) {
  const spy = { fed: [], synced: [], paused: 0, disposed: 0, audioEvents: 0, audioPlays: [], hostPlays: 0 };
  const handlers = {}, loads = [], asked = [], pending = [];
  S.createAudio = () => ({
    load(info, events) { spy.fed.push({ info, events }); },
    play(frame) { spy.audioPlays.push(frame); }, seek() {},
    pause() { spy.paused++; },
    sync(frame) { spy.synced.push(frame); },
    dispose() { spy.disposed++; }
  });
  const inst = {
    projectId, disposed: false, loadSeq: 0, selected: 0, frame: 0, catalog: {}, parts: {}, report: null,
    story: { schema: 'storymaker/1', format: { aspect: '16:9', fps: 30 }, scenes: [{ id: 's1' }] },
    bus: S.createBus(),
    api: { narration(id) {
      asked.push(id);
      const d = {};
      d.promise = new Promise((resolve, reject) => { d.resolve = resolve; d.reject = reject; });
      pending.push(d);
      return d.promise;
    } },
    assets: { prepare: async () => ({ catalog: { entries: [] }, assets: {}, fonts: [], code_bundles: {} }), index: () => ({}) },
    player: {
      load(payload) { loads.push(payload); },
      play() { spy.hostPlays++; },
      on(name, fn) { (handlers[name] = handlers[name] || []).push(fn); return () => {}; }
    }
  };
  inst.reloadPlayer = () => wiring.reloadPlayer(inst);
  inst.bus.on('audio', () => { spy.audioEvents++; });
  return { inst, spy, handlers, loads, asked, pending };
}

const hash = 'a'.repeat(64);
const blocks = { s1: { blocks: [{ id: 's1:n', kind: 'narration', speaker: 'narrator', duration_ms: 1000, spans: [[0, 900]], speech_ok: true, levels: [0, 90], pieces: [{ hash, ext: 'wav', at_ms: 0, in_ms: 0, out_ms: 1000, chars: 5 }] }] } };
const answerWith = narration => ({ narration, missing: [], chars: { total: 5, missing: 0 }, music: null });

(async () => {
  // Open with stored audio, in the order of openProject: buildEditor binds, then the player loads, then the answer arrives.
  const a = harness('p1');
  const bound = S.bindAudio(a.inst);
  wiring.reloadPlayer(a.inst);
  eq(a.asked, ['p1'], 'open refreshes once');
  await flush();
  eq(a.loads.length, 1, 'the first load does not wait for the answer');
  eq(a.loads[0].audio_info, { narration: {} }, 'the first load has no audio yet');
  a.pending[0].resolve(answerWith(blocks));
  await flush();
  eq(a.inst.audioInfo.narration, blocks, 'bindAudio sets inst.audioInfo');
  eq(a.spy.audioEvents, 1, 'the bus hears audio');
  eq(a.loads.length, 2, 'an answer with other narration reloads the player');
  eq(a.loads[1].audio_info, { narration: blocks }, 'the second load carries the blocks');

  // The next compiled report feeds createAudio.load (bindPlayer sets inst.report and emits report).
  a.inst.report = { fps: 30, frame_count: 90, scenes: [], audio_events: [{ kind: 'narration', ref: hash, start_ms: 0, scene_id: 's1', block_id: 's1:n', piece: 0 }] };
  a.inst.bus.emit('report');
  eq(a.spy.fed.length, 1, 'compiled feeds the audio once');
  eq([a.spy.fed[0].info.fps, a.spy.fed[0].info.frame_count, a.spy.fed[0].info.narration], [30, 90, blocks], 'the info of the answer and the report');
  eq(a.spy.fed[0].events, a.inst.report.audio_events, 'the events of the report');

  // A successful save refreshes once. The same narration does not reload the player.
  wiring.afterStorySaved(a.inst);
  clearTimeout(a.inst.previewTimer);
  eq(a.asked.length, 2, 'a save refreshes once');
  a.pending[1].resolve(answerWith(blocks));
  await flush();
  eq(a.loads.length, 2, 'the same narration does not reload');

  // A narrate job on every terminal status refreshes once. A running job, a repeat of the same event and a render job do not.
  ['succeeded', 'failed', 'cancelled', 'interrupted'].forEach((status, i) => {
    const before = a.asked.length;
    a.inst.bus.emit('job', { id: 'n' + i, kind: 'narrate', status: 'running' });
    eq(a.asked.length, before, 'a running job asks nothing');
    a.inst.bus.emit('job', { id: 'n' + i, kind: 'narrate', status });
    a.inst.bus.emit('job', { id: 'n' + i, kind: 'narrate', status });
    eq(a.asked.length, before + 1, status + ' refreshes once');
  });
  const afterJobs = a.asked.length;
  a.inst.bus.emit('job', { id: 'r1', kind: 'render', status: 'succeeded' });
  eq(a.asked.length, afterJobs, 'a render job asks nothing');

  // The owner side of the time messages.
  const pausedBefore = a.spy.paused;
  a.handlers.time[0]({ frame: 12, playing: true });
  eq(a.spy.synced, [12], 'a time message while playing syncs');
  a.handlers.time[0]({ frame: 13, playing: false });
  eq([a.spy.synced, a.spy.paused], [[12], pausedBefore + 1], 'a paused host pauses the audio');
  a.handlers.loading[0]();
  eq(a.spy.paused, pausedBefore + 2, 'a new load pauses the audio');

  bound.dispose();
  eq([a.spy.disposed, a.inst.audio, a.inst.audioInfo, a.inst.refreshAudio], [1, null, null, null], 'dispose clears the instance');

  // A project without audio stays at one load.
  const c = harness('p3');
  S.bindAudio(c.inst);
  wiring.reloadPlayer(c.inst);
  await flush();
  c.pending[0].resolve(answerWith({}));
  await flush();
  eq(c.loads.length, 1, 'empty narration does not reload');

  // A stale answer for another project is ignored.
  const b = harness('p1');
  S.bindAudio(b.inst);
  b.inst.projectId = 'p2';
  b.pending[0].resolve(answerWith(blocks));
  await flush();
  eq([b.inst.audioInfo, b.spy.audioEvents, b.loads.length], [null, 0, 0], 'an answer for another project changes nothing');

  // Play follows the host. During a compile the host drops a play, so a press starts neither the film nor the sound. After compiled it starts both.
  const p = harness('p4');
  p.inst.t = key => key;
  wiring.bindPlayer(p.inst, {});
  S.bindAudio(p.inst);
  const controls = wiring.controls(p.inst);
  p.handlers.loading.forEach(fn => fn());
  controls.play();
  eq([p.spy.hostPlays, p.spy.audioPlays], [0, []], 'a press during a compile starts nothing');
  p.handlers.compiled[0]({ fps: 30, frame_count: 90, scenes: [], audio_events: [] });
  p.inst.frame = 7;
  controls.play();
  eq([p.spy.hostPlays, p.spy.audioPlays], [1, [7]], 'a press after compiled starts both');

  // A failed answer is kept for the panels. A fresh answer clears it, and a failure after an answer keeps that answer and stays quiet.
  const f = harness('p5');
  const boundFailed = S.bindAudio(f.inst);
  const failure = { stm: true, status: 500, code: 'storage_failed' };
  f.pending[0].reject(failure);
  await flush();
  eq([f.inst.audioError.code, f.inst.audioInfo, f.spy.audioEvents], ['storage_failed', null, 1], 'a failure without an answer is told');
  f.inst.refreshAudio();
  f.pending[1].resolve(answerWith({}));
  await flush();
  eq([f.inst.audioError, f.spy.audioEvents], [null, 2], 'a fresh answer clears the error');
  f.inst.refreshAudio();
  f.pending[2].reject(failure);
  await flush();
  eq([f.inst.audioError.code, f.inst.audioInfo.missing, f.spy.audioEvents], ['storage_failed', [], 2], 'a failure after an answer keeps it and stays quiet');
  boundFailed.dispose();
  eq(f.inst.audioError, null, 'dispose clears the error');

  // A request cut off by closing the window says nothing.
  const q = harness('p6');
  S.bindAudio(q.inst);
  q.pending[0].reject({ aborted: true });
  await flush();
  eq([q.inst.audioError, q.spy.audioEvents], [null, 0], 'an aborted request says nothing');

  console.log('storymaker bind audio ok');
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`

// The preview audio without a DOM: the decode window, the waveform, and scheduling on a fake AudioContext.
func TestStorymakerAudioModelInNode(t *testing.T) {
	t.Parallel()
	runStorymakerAudioScript(t, storymakerAudioModelScript, "storymaker audio model ok")
}

const storymakerAudioModelScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const dir = process.argv[2];

const contexts = [], fetched = [], decoded = [];
class FakeSource {
  constructor(owner) { this.owner = owner; this.buffer = null; this.started = null; this.stopped = false; this.onended = null; }
  connect() {}
  disconnect() {}
  start(when, offset, duration) { this.started = { when, offset, duration }; this.owner.sources.push(this); }
  stop() { this.stopped = true; }
}
class FakeAudioContext {
  constructor() { this.currentTime = 0; this.state = 'running'; this.destination = {}; this.sources = []; this.closed = false; contexts.push(this); }
  createBufferSource() { return new FakeSource(this); }
  decodeAudioData(data) { decoded.push(data.byteLength); return Promise.resolve({ duration: 60 }); }
  resume() { return Promise.resolve(); }
  close() { this.closed = true; return Promise.resolve(); }
}
const context = { console, AbortController, setTimeout, clearTimeout, AudioContext: FakeAudioContext };
context.fetch = url => { fetched.push(url); return Promise.resolve({ ok: true, arrayBuffer: () => Promise.resolve(new ArrayBuffer(8)) }); };
context.globalThis = context; context.window = context;
context.document = { documentElement: { lang: 'en' } };
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-api.js', 'storymaker-audio.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts, M = S.audioModel;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const startedAt = (source, when, offset, duration, what) => {
  const s = source.started;
  if (Math.abs(s.when - when) > 1e-6 || Math.abs(s.offset - offset) > 1e-6 || Math.abs(s.duration - duration) > 1e-6) throw new Error(what + ': got ' + JSON.stringify(s));
};
const flush = () => new Promise(resolve => setTimeout(resolve, 0));

(async () => {
  // windowFor. Playhead 5000 ms: 2 s back is 3000 ms and 20 s ahead is 25000 ms, both ends are kept.
  const at = ms => ({ kind: 'narration', ref: 'r' + ms, start_ms: ms, scene_id: 's1' });
  eq(M.windowFor([2999, 3000, 10000, 25000, 25001].map(at), 5000).map(e => e.start_ms), [3000, 10000, 25000], 'the window keeps 2 s back to 20 s ahead');

  // waveform at 30 fps. A frame is 100/3 ms, so 100 ms is frame 3, 200 ms is frame 6 and 300 ms is frame 9.
  // Block s1:n starts at 100 ms. Block s1:b1 is known only by its second piece: 400 ms minus at_ms 200 is 200 ms.
  // Block s2:n starts at 300 ms and runs past frame_count 10, so its second level is cut.
  const wave = M.waveform({ fps: 30, frame_count: 10, audio_events: [
    { kind: 'narration', ref: 'x', start_ms: 100, scene_id: 's1', block_id: 's1:n', piece: 0 },
    { kind: 'dialogue', ref: 'y', start_ms: 400, scene_id: 's1', block_id: 's1:b1', piece: 1 },
    { kind: 'narration', ref: 'z', start_ms: 300, scene_id: 's2', block_id: 's2:n', piece: 0 },
    { kind: 'sfx', ref: 'sfx.pop', start_ms: 0, scene_id: 's1', take: 0 }
  ] }, {
    s1: { blocks: [
      { id: 's1:n', levels: [10, 20, 30], pieces: [{ at_ms: 0 }] },
      { id: 's1:b1', levels: [40, 50], pieces: [{ at_ms: 0 }, { at_ms: 200 }] }
    ] },
    s2: { blocks: [{ id: 's2:n', levels: [60, 70], pieces: [{ at_ms: 0 }] }] }
  });
  eq(wave.length, 10, 'one value per frame');
  eq(Array.from(wave), [0, 0, 0, 10, 20, 30, 40, 50, 0, 60], 'block levels at their frames, 0 elsewhere');

  // scheduling
  const id = '11111111-2222-4333-8444-555555555555';
  const h1 = 'a'.repeat(64), h2 = 'b'.repeat(64), unknown = 'c'.repeat(64);
  const url = hash => '/api/desktop/storymaker/projects/' + id + '/audio/narration/' + hash + '.wav';
  const narration = { s1: { blocks: [{ id: 's1:n', kind: 'narration', levels: [], pieces: [
    { hash: h1, ext: 'wav', at_ms: 0, in_ms: 50, out_ms: 5050, chars: 10 },
    { hash: h2, ext: 'wav', at_ms: 30000, in_ms: 0, out_ms: 1000, chars: 10 }
  ] }] } };
  const audio = S.createAudio({ api: S.createApi({ fetch: () => Promise.reject(new Error('unused')) }), projectId: id });
  audio.load({ fps: 30, frame_count: 1800, narration, music: null, volume: 1, catalog: null }, [
    { kind: 'narration', ref: h1, start_ms: 0, scene_id: 's1', block_id: 's1:n', piece: 0 },
    { kind: 'sfx', ref: 'sfx.pop', start_ms: 0, scene_id: 's1', take: 0 },
    { kind: 'narration', ref: unknown, start_ms: 0, scene_id: 's1', block_id: 's1:n', piece: 0 },
    { kind: 'narration', ref: h2, start_ms: 30000, scene_id: 's1', block_id: 's1:n', piece: 1 }
  ]);
  eq(contexts.length, 0, 'no AudioContext before the first play');

  audio.play(0);
  await flush();
  eq(contexts.length, 1, 'play creates the AudioContext');
  const ctx = contexts[0];
  eq(fetched, [url(h1)], 'only the piece in the window is fetched; an unknown ref and sfx never are');
  eq(ctx.sources.length, 1, 'one piece is scheduled');
  // Film time 0 is context time 0. The used part of the file starts at in_ms 50 and lasts out_ms-in_ms = 5000 ms.
  startedAt(ctx.sources[0], 0, 0.05, 5, 'the first start');

  // 30 fps: frame 30 is 1000 ms. The context says 1100 ms, a drift of 100 ms, under the 120 ms limit.
  ctx.currentTime = 1.1;
  audio.sync(30);
  await flush();
  eq([ctx.sources.length, ctx.sources[0].stopped], [1, false], 'a drift of 100 ms changes nothing');

  // The context says 1250 ms, a drift of 250 ms, so the new origin is 1.25-1 = 0.25 s. The piece began 1 s ago:
  // it starts now, 1 s further into the file (0.05+1) and 1 s shorter (5-1).
  ctx.currentTime = 1.25;
  audio.sync(30);
  await flush();
  eq([ctx.sources[0].stopped, ctx.sources.length], [true, 2], 'a drift of 250 ms stops and reschedules');
  startedAt(ctx.sources[1], 1.25, 1.05, 4, 'the rescheduled start');
  eq(decoded.length, 1, 'a piece in the window is not decoded twice');

  // Frame 300 is 10000 ms, and the context says (10.25-0.25) s, so no drift. The window now reaches 30000 ms.
  ctx.currentTime = 10.25;
  audio.sync(300);
  await flush();
  eq(fetched, [url(h1), url(h2)], 'the window slides forward');
  startedAt(ctx.sources[2], 30.25, 0, 1, 'the second piece waits for its time');

  // Frame 90 is 3000 ms. The first piece began 3 s back, outside the window, but sounds until 5000 ms: its last 2 s play.
  // It left the buffers when the window slid, so it is fetched again.
  audio.seek(90);
  await flush();
  eq(fetched, [url(h1), url(h2), url(h1)], 'a seek into a long piece fetches it again');
  startedAt(ctx.sources[3], 10.25, 3.05, 2, 'a seek into a long piece plays its rest');

  audio.pause();
  eq(ctx.sources.every(source => source.stopped), true, 'pause stops everything');
  audio.sync(120);
  await flush();
  eq(ctx.sources.length, 4, 'sync while paused schedules nothing');
  audio.dispose();
  eq(ctx.closed, true, 'dispose closes the context');
  console.log('storymaker audio model ok');
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`
