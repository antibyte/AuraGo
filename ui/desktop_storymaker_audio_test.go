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

// The voice panel in Node, on a small stand-in for the DOM: what it preselects, what each kind of provider shows, the one action
// that writes the voice, the audition of a stored piece and the state with no ready provider.
func TestStorymakerVoicePanelInNode(t *testing.T) {
	t.Parallel()
	root := readDesktopAssetText(t, "js/desktop/apps/storymaker.js")
	for _, marker := range []string{"S.createVoicePanel(inst)", "inst.parts.voice = voice", "'storymaker.side_voice'", "editStory: (fn, options) => editStory(inst, fn, options)"} {
		if !strings.Contains(root, marker) {
			t.Errorf("storymaker.js lacks %q", marker)
		}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-voice-panel.js")
	if err := os.WriteFile(script, []byte(storymakerVoicePanelScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	out, err := exec.Command(node, script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker voice panel ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerVoicePanelScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const [dir] = process.argv.slice(2);

class FakeNode {
  constructor(tag) { this.tag = tag; this.kids = []; this.parent = null; this.attrs = {}; this.dataset = {}; this.handlers = {}; this.style = { setProperty() {} }; this.ownText = ''; this.className = ''; }
  get firstChild() { return this.kids[0] || null; }
  appendChild(kid) { if (kid.parent) kid.parent.removeChild(kid); kid.parent = this; this.kids.push(kid); return kid; }
  removeChild(kid) { this.kids.splice(this.kids.indexOf(kid), 1); kid.parent = null; return kid; }
  setAttribute(name, value) { this.attrs[name] = String(value); }
  getAttribute(name) { return name in this.attrs ? this.attrs[name] : null; }
  removeAttribute(name) { delete this.attrs[name]; }
  get hidden() { return 'hidden' in this.attrs; }
  set hidden(on) { if (on) this.attrs.hidden = ''; else delete this.attrs.hidden; }
  get disabled() { return 'disabled' in this.attrs; }
  set disabled(on) { if (on) this.attrs.disabled = ''; else delete this.attrs.disabled; }
  get value() {
    if (this.tag === 'select') return this.kids.map(kid => kid.attrs.value).indexOf(this.val) >= 0 ? this.val : '';
    return this.val !== undefined ? this.val : (this.attrs.value || '');
  }
  set value(next) { this.val = String(next); }
  get textContent() { return this.kids.length ? this.kids.map(kid => kid.textContent).join('') : this.ownText; }
  set textContent(next) { this.kids = []; this.ownText = String(next); }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  removeEventListener() {}
  fire(type) { (this.handlers[type] || []).slice().forEach(fn => fn({ type, target: this, preventDefault() {}, stopPropagation() {} })); }
  click() { if (!this.disabled) this.fire('click'); }
  focus() {}
  find(name) { if (this.dataset.stm === name) return this; for (const kid of this.kids) { const hit = kid.find(name); if (hit) return hit; } return null; }
  play() { this.played = this.src; return Promise.resolve(); }
  pause() {}
}
const document = {
  createElement: tag => new FakeNode(tag),
  createElementNS: (ns, tag) => new FakeNode(tag),
  createTextNode: text => { const node = new FakeNode('#text'); node.ownText = String(text); return node; },
  addEventListener() {}, removeEventListener() {},
  documentElement: { lang: 'en' }
};
const context = { console, setTimeout, clearTimeout, document, SYSTEM_LANG: 'de' };
context.globalThis = context; context.window = context;
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-voice.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const flush = () => new Promise(resolve => setImmediate(resolve));
const options = node => node.kids.filter(kid => kid.tag === 'option').map(kid => kid.attrs.value);
const nullStory = () => ({ schema: 'storymaker/1', title: 'T', voice: null, scenes: [{ id: 's1' }, { id: 's2' }] });

const providers = [
  { id: 'piper', active: true, ready: true, voices: [{ id: 'de_DE-thorsten-high', name: 'Thorsten', language: 'de_DE' }, { id: 'en_US-amy', name: 'Amy', language: 'en_US' }], languages: ['de', 'en'], tested_languages: [], needs_install: false, default_voice: 'de_DE-thorsten-high', chunk_limit: 400, note: 'Local voices.' },
  { id: 'sanotts', active: false, ready: true, languages: ['ar', 'de', 'en'], tested_languages: [], needs_install: false, chunk_limit: 2000 },
  { id: 'elevenlabs', active: false, ready: true, free_text: true, languages: ['*'], tested_languages: [], needs_install: false, chunk_limit: 400 },
  { id: 'phonon', active: false, ready: false, languages: ['de', 'en'], tested_languages: [], needs_install: true, chunk_limit: 2000 }
];

async function open(story, list, answer) {
  const inst = {
    t: (key, vars) => (vars ? key + ' ' + JSON.stringify(vars) : key),
    bus: S.createBus(), story, selected: 0, projectId: 'p1', readonly: false, audioInfo: answer || null, edits: 0,
    domId: suffix => 'stm-test-' + suffix,
    notify() {},
    editStory(fn) { if (inst.readonly) return false; const draft = S.model.cloneStory(inst.story); fn(draft); inst.story = draft; inst.edits++; inst.bus.emit('story'); return true; },
    api: {
      voices: () => (list instanceof Error ? Promise.reject(list) : Promise.resolve({ providers: list })),
      narrationUrl: (id, hash, ext) => '/audio/' + id + '/' + hash + '.' + ext
    }
  };
  const panel = S.createVoicePanel(inst);
  await flush();
  return { inst, panel, f: name => panel.el.find(name) };
}
function pick(view, id) { view.f('voice-provider').value = id; view.f('voice-provider').fire('change'); }

(async () => {
  // A story with voice null preselects the active provider, and the language starts from the desktop language.
  const first = await open(nullStory(), providers);
  eq(first.f('voice-provider').value, 'piper', 'the active provider is preselected');
  eq(first.f('voice-provider').kids.find(kid => kid.attrs.value === 'phonon').disabled, true, 'a provider that is not ready cannot be picked');
  eq([first.f('voice-language').value, options(first.f('voice-language'))], ['de', ['de', 'en']], 'the language select is limited to the provider');
  eq([first.f('voice-language-field').hidden, first.f('voice-language-free-field').hidden], [false, true], 'a list provider has no free language field');
  eq(options(first.f('voice-select')), ['de_DE-thorsten-high'], 'the voices of the language');
  eq(first.f('voice-not-tested').hidden, false, 'not tested shows for a list provider');
  eq(first.f('voice-note').textContent, 'Local voices.', 'the facts of the note');

  // One action writes the voice, and then there is nothing left to write.
  eq(first.f('voice-apply').disabled, false, 'the action is open for a voice null story');
  first.f('voice-apply').click();
  eq(first.inst.story.voice, { provider: 'piper', voice: 'de_DE-thorsten-high', language: 'de' }, 'one action writes the voice');
  eq(first.inst.edits, 1, 'one edit');
  eq(first.f('voice-apply').disabled, true, 'the stored voice needs no second write');

  // sanotts has neither a list nor free text.
  const second = await open(nullStory(), providers);
  pick(second, 'sanotts');
  eq([second.f('voice-select-field').hidden, second.f('voice-free-field').hidden], [true, true], 'sanotts has no voice control');
  eq([second.f('voice-language').value, options(second.f('voice-language'))], ['de', ['ar', 'de', 'en']], 'the sanotts languages');
  eq(second.f('voice-not-tested').hidden, false, 'not tested shows for sanotts');
  second.f('voice-apply').click();
  eq(second.inst.story.voice, { provider: 'sanotts', voice: '', language: 'de' }, 'sanotts writes no voice id');

  // A free-text provider takes the voice id and the language as text.
  pick(second, 'elevenlabs');
  eq([second.f('voice-free-field').hidden, second.f('voice-select-field').hidden], [false, true], 'a free-text field for the voice');
  eq([second.f('voice-language-free-field').hidden, second.f('voice-language-field').hidden], [false, true], 'a free-text field for ["*"]');
  eq(second.f('voice-language-free').value, 'de', 'the field holds the same language');
  eq(second.f('voice-not-tested').hidden, false, 'not tested shows for a free-text provider');
  second.f('voice-free').value = 'a b'; second.f('voice-free').fire('input');
  eq(second.f('voice-apply').disabled, true, 'a voice id with a space would be refused by the save');
  second.f('voice-free').value = 'abc_123'; second.f('voice-free').fire('input');
  second.f('voice-apply').click();
  eq(second.inst.story.voice, { provider: 'elevenlabs', voice: 'abc_123', language: 'de' }, 'free text is written');

  // A tested language hides the mark.
  const tested = providers.map(item => (item.id === 'piper' ? Object.assign({}, item, { tested_languages: ['de'] }) : item));
  eq((await open(nullStory(), tested)).f('voice-not-tested').hidden, true, 'a tested language has no mark');

  // voice.language wins over the desktop language, and a stored voice shows as stored.
  const story = nullStory();
  story.voice = { provider: 'Piper', voice: 'en_US-amy', language: 'en-US' };
  const stored = await open(story, providers);
  eq(stored.f('voice-provider').value, 'piper', 'provider case is ignored');
  eq([stored.f('voice-language').value, options(stored.f('voice-language'))], ['en-US', ['de', 'en-US']], 'the stored language stays as written');
  eq(stored.f('voice-select').value, 'en_US-amy', 'the stored voice');
  eq(stored.f('voice-apply').disabled, true, 'nothing to write');

  // A desktop language the provider does not list falls to its first language, and the note names the language it replaced.
  context.SYSTEM_LANG = 'ja';
  const foreign = await open(nullStory(), providers);
  const foreignWritten = await open(nullStory(), providers);
  context.SYSTEM_LANG = 'de';
  eq(foreign.f('voice-language').value, 'de', 'the language stays inside the provider list');
  eq(foreign.f('voice-note').textContent, 'storymaker.voice.language_replaced {"language":"ja"} Local voices.', 'the note names the replaced language');
  // The language asked for comes back with a provider that can take it, and the note goes with it.
  pick(foreign, 'elevenlabs');
  eq([foreign.f('voice-language-free').value, foreign.f('voice-note').textContent], ['ja', ''], 'a free-text provider takes the language back');
  pick(foreign, 'sanotts');
  eq([foreign.f('voice-language').value, foreign.f('voice-note').textContent], ['ar', 'storymaker.voice.language_replaced {"language":"ja"}'], 'another provider without it says so again');
  foreign.f('voice-language').value = 'de'; foreign.f('voice-language').fire('change');
  eq(foreign.f('voice-note').textContent, '', 'a language the user picks replaces nothing');
  foreignWritten.f('voice-apply').click();
  eq([foreignWritten.inst.story.voice.language, foreignWritten.f('voice-note').textContent], ['de', 'Local voices.'], 'a written voice holds no replacement');

  // The audition plays the first piece of the first block in scene order, and is disabled with none.
  const heardStory = nullStory();
  heardStory.voice = { provider: 'piper', voice: 'de_DE-thorsten-high', language: 'de' };
  const heard = await open(heardStory, providers);
  eq(heard.f('voice-audition').disabled, true, 'no stored piece, no audition');
  heard.inst.audioInfo = { narration: {
    s2: { blocks: [{ id: 's2:n', pieces: [{ hash: 'b'.repeat(64), ext: 'mp3', at_ms: 0 }] }] },
    s1: { blocks: [{ id: 's1:n', pieces: [{ hash: 'a'.repeat(64), ext: 'wav', at_ms: 0 }, { hash: 'c'.repeat(64), ext: 'wav', at_ms: 900 }] }] }
  }, missing: [], chars: { total: 10, missing: 0 }, music: null };
  heard.inst.bus.emit('audio');
  eq(heard.f('voice-audition').disabled, false, 'a stored piece opens the audition');
  heard.f('voice-audition').click();
  eq(heard.f('voice-audio').played, '/audio/p1/' + 'a'.repeat(64) + '.wav', 'the first stored piece plays');

  // The pieces belong to the stored voice, so a picked voice closes the audition until it is written.
  heard.f('voice-language').value = 'en'; heard.f('voice-language').fire('change');
  eq([heard.f('voice-audition').disabled, heard.f('voice-audition').title], [true, 'storymaker.voice.apply_first'], 'a differing draft has no sample');
  heard.f('voice-language').value = 'de'; heard.f('voice-language').fire('change');
  eq([heard.f('voice-audition').disabled, heard.f('voice-audition').title], [false, ''], 'the stored voice has it back');

  // A Phonon voice that is not installed fails its job, so the list leaves it out. A stored one stays visible and marked and cannot be written again.
  const catalog = [{ id: 'phonon', active: true, ready: true, languages: ['fr', 'de'], tested_languages: [], needs_install: false, default_voice: 'fr-there', chunk_limit: 2000, voices: [
    { id: 'fr-absent', name: 'Absent', language: 'fr', installed: false },
    { id: 'fr-there', name: 'There', language: 'fr', installed: true },
    { id: 'de-absent', name: 'Fehlt', language: 'de', installed: false },
    { id: 'de-there', name: 'Da', language: 'de', installed: true }
  ] }];
  const offered = await open(nullStory(), catalog);
  eq(options(offered.f('voice-select')), ['de-there'], 'an uninstalled voice is not listed');
  offered.f('voice-language').value = 'fr'; offered.f('voice-language').fire('change');
  eq([offered.f('voice-select').value, options(offered.f('voice-select'))], ['fr-there', ['fr-there']], 'a language change lands on an installed voice');
  const heldStory = nullStory();
  heldStory.voice = { provider: 'phonon', voice: 'fr-absent', language: 'fr' };
  const held = await open(heldStory, catalog);
  eq(options(held.f('voice-select')), ['fr-absent', 'fr-there'], 'the stored voice stays listed');
  eq(held.f('voice-select').kids.map(kid => kid.textContent.includes('storymaker.voice.not_installed')), [true, false], 'only the uninstalled voice is marked');
  eq(held.f('voice-note').textContent, 'storymaker.voice.not_installed', 'the note says what to do');
  const heldFree = await open(heldStory, [Object.assign({}, catalog[0], { languages: ['*'] })]);
  heldFree.f('voice-language-free').value = 'fr-CA'; heldFree.f('voice-language-free').fire('input');
  eq(heldFree.f('voice-apply').disabled, true, 'an uninstalled voice cannot be written');
  // Settings offers every Piper voice, so that flag decides nothing.
  const flagged = await open(nullStory(), [{ id: 'piper', active: true, ready: true, languages: ['de'], tested_languages: [], needs_install: false, default_voice: 'de-flagged', chunk_limit: 400, voices: [{ id: 'de-flagged', language: 'de', installed: false }] }]);
  eq(options(flagged.f('voice-select')), ['de-flagged'], 'a Piper voice stays listed');

  // No ready provider: the state says so and there is no form.
  const silent = await open(nullStory(), providers.map(item => Object.assign({}, item, { ready: false })));
  eq([silent.f('voice-none').hidden, silent.f('voice-form').hidden], [false, true], 'no TTS ready');

  // A failed list is an error, not the claim that nothing is ready.
  const down = await open(nullStory(), new Error('down'));
  eq([down.f('voice-none').hidden, down.f('voice-error').hidden, down.f('voice-retry').hidden], [true, false, false], 'a failed list offers a retry');

  console.log('storymaker voice panel ok');
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`
