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
  eq(a.spy.fed.length, 2, 'a fresh answer feeds the audio, because music alone starts no compile');

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

// The narration half of the voice panel beside the real export popover, on a small stand-in for the DOM: job events go to the right
// owner, the status follows pieces_done, a missing block shows the marker, and the terminal states have the panel's own text.
func TestStorymakerNarrationStatusInNode(t *testing.T) {
	t.Parallel()
	if !strings.Contains(readDesktopAssetText(t, "js/desktop/apps/storymaker.js"), "stage.appendChild(voice.marker)") {
		t.Error("storymaker.js does not put the no-audio marker on the stage")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-narration-status.js")
	if err := os.WriteFile(script, []byte(storymakerNarrationStatusScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	out, err := exec.Command(node, script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker narration status ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerNarrationStatusScript = `
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
  play() { return Promise.resolve(); }
  load() { this.loaded = true; }
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
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-export.js', 'storymaker-voice.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const flush = () => new Promise(resolve => setImmediate(resolve));
const t = (key, vars) => (vars ? key + ' ' + JSON.stringify(vars) : key);
const providers = [{ id: 'piper', active: true, ready: true, voices: [{ id: 'de_DE-thorsten-high', language: 'de_DE' }, { id: 'de_DE-karlsson-low', language: 'de_DE' }], languages: ['de'], tested_languages: [], needs_install: false, chunk_limit: 400 }];
const job = extra => Object.assign({ id: 'n1', project_id: 'p1', kind: 'narrate', status: 'running', pieces_done: 0, pieces_total: 4, progress: 0 }, extra);

function makeInst(answer) {
  const inst = {
    t, bus: S.createBus(), selected: 0, projectId: 'p1', readonly: false, status: {}, issues: [], report: null, etag: '', ctx: {},
    audioInfo: answer,
    story: { schema: 'storymaker/1', voice: { provider: 'piper', voice: 'de_DE-thorsten-high', language: 'de' }, style: { palette: 'sunny', font: 'default', captions: true }, scenes: [{ id: 's1' }, { id: 's2' }] },
    domId: suffix => 'stm-test-' + suffix,
    notify() {}, showIssues() {},
    flushed: 0, refreshed: 0, narrated: [], answers: [], cancelled: [],
    flushEdits: async () => { inst.flushed++; return true; },
    refreshAudio() { inst.refreshed++; },
    editStory(fn) { if (inst.readonly) return false; const draft = S.model.cloneStory(inst.story); fn(draft); inst.story = draft; inst.bus.emit('story'); return true; },
    api: {
      voices: async () => ({ providers }),
      narrationUrl: () => '',
      narrate(id) { inst.narrated.push(id); const next = inst.answers.shift(); return next && next.stm ? Promise.reject(next) : Promise.resolve(next); },
      getJob: async () => { throw new Error('nothing polls in this test'); },
      cancelJob: async id => { inst.cancelled.push(id); return job({ id, pieces_done: 3 }); }
    }
  };
  return inst;
}

(async () => {
  const inst = makeInst({ narration: {}, missing: [{ scene_id: 's1', block_id: 's1:n', chars: 120 }], chars: { total: 640, missing: 120 }, music: null });
  const panel = S.createVoicePanel(inst);
  const exporter = S.createExport(inst);
  await flush();
  const f = name => panel.el.find(name);
  const button = exporter.el.find('export');

  // What is missing, and the marker for the selected scene.
  eq(f('narration-summary').textContent, 'storymaker.voice.missing {"blocks":1,"chars":120,"total":640}', 'what is missing');
  eq(panel.marker.hidden, false, 'a missing block in the selected scene shows the marker');
  inst.selected = 1; inst.bus.emit('selection');
  eq(panel.marker.hidden, true, 'the other scene misses nothing');
  inst.selected = 0; inst.bus.emit('selection');

  // Read-only.
  eq(f('narration-cancel').hidden, true, 'no job, no Stop');
  eq(f('narration-generate').disabled, false, 'Generate is open');
  inst.readonly = true; inst.bus.emit('readonly');
  eq([f('narration-generate').disabled, f('captions-toggle').disabled], [true, true], 'read-only disables Generate and the toggle');
  inst.readonly = false; inst.bus.emit('readonly');

  // A failed first read says why and offers Retry, instead of loading forever.
  const answered = inst.audioInfo;
  inst.audioInfo = null; inst.audioError = S.errors.fromResponse(500, { error: 'internal', code: 'internal', message: 'm' }, 0); inst.bus.emit('audio');
  eq([f('narration-summary').textContent === 'storymaker.loading', f('narration-read-retry').hidden], [false, false], 'a failed read shows the reason and Retry');
  const readsBefore = inst.refreshed;
  f('narration-read-retry').click();
  eq(inst.refreshed, readsBefore + 1, 'Retry reads again');
  inst.refreshed = readsBefore;
  inst.audioInfo = answered; inst.audioError = null; inst.bus.emit('audio');
  eq(f('narration-read-retry').hidden, true, 'Retry goes with the error');

  // Refusals say why, with the fields the sentence alone cannot carry.
  inst.answers.push(S.errors.fromResponse(409, { error: 'voice_unknown', code: 'voice_unknown', message: 'm', alternatives: ['a', 'b'] }, 0));
  f('narration-generate').click(); await flush();
  eq([inst.flushed, inst.narrated], [1, ['p1']], 'edits are saved first, then the route runs once');
  eq(f('narration-refusal').textContent.includes('storymaker.voice.alternatives {"names":"a, b"}'), true, 'the alternatives show');
  inst.answers.push(S.errors.fromResponse(409, { error: 'budget_exceeded', code: 'budget_exceeded', message: 'm', limit: 20000, missing: 30000 }, 0));
  f('narration-generate').click(); await flush();
  eq(f('narration-refusal').textContent.includes('storymaker.voice.budget_detail {"limit":20000,"missing":30000}'), true, 'the budget numbers show');

  // Nothing missing on the server: no job, and the status is read again.
  inst.answers.push({ job: null });
  f('narration-generate').click(); await flush();
  eq([f('narration-job').hidden, inst.refreshed, f('narration-refusal').hidden], [true, 1, true], 'job null refreshes');

  // A new job.
  inst.answers.push({ job: job({ status: 'queued' }) });
  f('narration-generate').click(); await flush();
  eq(f('narration-job').textContent, 'storymaker.voice.job_queued', 'queued');
  eq(f('narration-cancel').hidden, false, 'a queued job can be stopped');
  eq(f('narration-generate').disabled, true, 'one job at a time');

  // A narrate event leaves the popover idle and advances the pieces.
  inst.bus.emit('job', job({ pieces_done: 1 }));
  eq([exporter.job(), button.dataset.state], [null, 'idle'], 'a narrate job leaves the export popover idle');
  eq(f('narration-job').textContent, 'storymaker.voice.job_running', 'running');
  eq([f('narration-pieces').textContent, f('narration-progress').value], ['storymaker.voice.pieces {"done":1,"total":4}', '0.25'], 'one of four');
  inst.bus.emit('job', job({ pieces_done: 3 }));
  eq([f('narration-pieces').textContent, f('narration-progress').value], ['storymaker.voice.pieces {"done":3,"total":4}', '0.75'], 'three of four');

  // Stop shows while the job runs, is hidden when read-only and goes through the existing cancel call.
  eq(f('narration-cancel').hidden, false, 'a running job can be stopped');
  inst.readonly = true; inst.bus.emit('readonly');
  eq(f('narration-cancel').hidden, true, 'read-only hides Stop');
  inst.readonly = false; inst.bus.emit('readonly');
  f('narration-cancel').click(); await flush();
  eq(inst.cancelled, ['n1'], 'Stop calls cancelJob with the job id');

  // A render event goes to the popover only, and another project is ignored.
  inst.bus.emit('job', { id: 'r1', project_id: 'p1', kind: 'render', status: 'running', progress: 0.5 });
  eq([exporter.job().id, button.dataset.state], ['r1', 'running'], 'a render job reaches the popover');
  inst.bus.emit('job', job({ id: 'n9', project_id: 'p2', pieces_done: 9, pieces_total: 9 }));
  eq(f('narration-pieces').textContent, 'storymaker.voice.pieces {"done":3,"total":4}', 'the panel kept its own job');

  // Terminal states have the panel's own text. describeJobError would answer with the unknown text for these two.
  inst.bus.emit('job', job({ status: 'interrupted' }));
  eq(f('narration-job').textContent, 'storymaker.voice.job_interrupted', 'interrupted');
  eq([f('narration-progress').hidden, f('narration-generate').disabled, f('narration-cancel').hidden], [true, false, true], 'a finished job frees Generate and has no Stop');
  inst.bus.emit('job', job({ status: 'cancelled' }));
  eq(f('narration-job').textContent, 'storymaker.voice.job_cancelled', 'cancelled');
  inst.bus.emit('job', job({ status: 'failed', error: { code: 'narration_failed', message: 'piper answered 500' } }));
  eq(f('narration-job-detail').textContent, 'piper answered 500', 'the failure detail');

  // Everything has audio.
  inst.audioInfo = { narration: {}, missing: [], chars: { total: 640, missing: 0 }, music: null };
  inst.bus.emit('audio');
  eq(f('narration-summary').textContent, 'storymaker.voice.complete {"total":640}', 'complete');
  eq([panel.marker.hidden, f('narration-generate').disabled], [true, true], 'nothing to generate');

  // The toggle writes style.captions and leaves the rest of the style alone.
  eq(f('captions-toggle').checked, true, 'captions start on');
  f('captions-toggle').checked = false; f('captions-toggle').fire('change');
  eq(inst.story.style, { palette: 'sunny', font: 'default', captions: false }, 'the toggle writes style.captions');
  eq(f('captions-toggle').checked, false, 'the toggle follows the story');

  // Generate runs the stored voice, so a voice the panel shows but the story does not hold keeps it closed and says why.
  const unsaved = () => ({ narration: {}, missing: [{ scene_id: 's1', block_id: 's1:n', chars: 120 }], chars: { total: 120, missing: 120 }, music: null });
  const fresh = makeInst(unsaved());
  fresh.story.voice = null;
  const freshPanel = S.createVoicePanel(fresh);
  await flush();
  const gate = () => [freshPanel.el.find('narration-generate').disabled, freshPanel.el.find('narration-voice-hint').hidden];
  eq(gate(), [true, false], 'voice null: the preselected voice is not stored yet');
  freshPanel.el.find('voice-apply').click();
  eq(gate(), [false, true], 'the one action opens Generate');
  const picker = freshPanel.el.find('voice-select');
  picker.value = 'de_DE-karlsson-low'; picker.fire('change');
  eq(gate(), [true, false], 'a picked voice that is not written closes it again');
  picker.value = 'de_DE-thorsten-high'; picker.fire('change');
  eq(gate(), [false, true], 'the stored voice opens it');
  freshPanel.dispose();

  // With no voice to write on screen the server's refusal still speaks.
  const bare = makeInst(unsaved());
  bare.story.voice = null;
  bare.api.voices = async () => ({ providers: [] });
  const barePanel = S.createVoicePanel(bare);
  await flush();
  eq([barePanel.el.find('narration-generate').disabled, barePanel.el.find('narration-voice-hint').hidden], [false, true], 'no form, no hold');
  barePanel.dispose();

  // An idle panel does not take up an old finished job, which the event stream replays on every open.
  const other = makeInst(null);
  const quiet = S.createVoicePanel(other);
  await flush();
  other.bus.emit('job', job({ id: 'old', status: 'failed', error: { code: 'narration_failed', message: 'x' } }));
  eq(quiet.el.find('narration-job').hidden, true, 'an old failure stays out');

  exporter.dispose();
  panel.dispose();
  quiet.dispose();
  eq(f('voice-audio').loaded, true, 'dispose lets the audition file go');
  console.log('storymaker narration status ok');
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`

// The scrubber waveform in Node: block levels at block frames, 0 in pauses and between blocks, and a canvas inside the scrubber that
// paints those levels.
func TestStorymakerScrubWaveInNode(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-scrub-wave.js")
	if err := os.WriteFile(script, []byte(storymakerScrubWaveScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	out, err := exec.Command(node, script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker scrub wave ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerScrubWaveScript = `
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
  get hidden() { return 'hidden' in this.attrs; }
  set hidden(on) { if (on) this.attrs.hidden = ''; else delete this.attrs.hidden; }
  get disabled() { return 'disabled' in this.attrs; }
  set disabled(on) { if (on) this.attrs.disabled = ''; else delete this.attrs.disabled; }
  get textContent() { return this.kids.length ? this.kids.map(kid => kid.textContent).join('') : this.ownText; }
  set textContent(next) { this.kids = []; this.ownText = String(next); }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  find(name) { if (this.dataset.stm === name) return this; for (const kid of this.kids) { const hit = kid.find(name); if (hit) return hit; } return null; }
  getContext() {
    if (!this.ctx) this.ctx = { bars: [], fillStyle: '', clearRect() { this.bars = []; }, fillRect(x, y, w, h) { this.bars.push({ x, y, w, h }); } };
    return this.ctx;
  }
}
const document = {
  createElement: tag => new FakeNode(tag),
  createElementNS: (ns, tag) => new FakeNode(tag),
  createTextNode: text => { const node = new FakeNode('#text'); node.ownText = String(text); return node; },
  addEventListener() {}, removeEventListener() {},
  documentElement: { lang: 'en' }
};
const context = { console, setTimeout, clearTimeout, document, getComputedStyle: () => ({ color: 'ink' }) };
context.globalThis = context; context.window = context;
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-strip.js', 'storymaker-audio.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };

// 30 fps: a block starts at frame round(start_ms*30/1000). 100 ms is frame 3, 300 ms is frame 9, 366 ms is 10.98, so frame 11.
const report = {
  frame_count: 12, fps: 30,
  scenes: [{ id: 's1', start_frame: 0, end_frame: 12, kind: 'storyboard' }],
  audio_events: [
    { kind: 'narration', ref: 'a', start_ms: 100, scene_id: 's1', block_id: 's1:n', piece: 0 },
    { kind: 'narration', ref: 'b', start_ms: 200, scene_id: 's1', block_id: 's1:n', piece: 1 },
    { kind: 'dialogue', ref: 'c', start_ms: 300, scene_id: 's1', block_id: 's1:b2', piece: 0 },
    { kind: 'narration', ref: 'd', start_ms: 366, scene_id: 's1', block_id: 's1:x', piece: 0 },
    { kind: 'narration', ref: 'e', start_ms: 0, scene_id: 's1', block_id: 'gone', piece: 0 },
    { kind: 'sfx', ref: 'sfx.pop', start_ms: 0, scene_id: 's1', take: 0 }
  ]
};
const narration = { s1: { blocks: [
  { id: 's1:n', kind: 'narration', levels: [10, 0, 30], pieces: [{ hash: 'a', at_ms: 0 }, { hash: 'b', at_ms: 100 }] },
  { id: 's1:b2', kind: 'dialogue', levels: [40, 50], pieces: [{ hash: 'c', at_ms: 0 }] },
  { id: 's1:x', kind: 'narration', levels: [60, 70], pieces: [{ hash: 'd', at_ms: 0 }] }
] } };

// s1:n fills frames 3-5 with its inner pause at 4. Its piece 1 event repeats nothing. s1:b2 fills 9-10. s1:x fills 11, and its second level is past the film.
// An event of a block that is not in the answer and an sfx event add nothing.
const levels = S.audioModel.waveform(report, narration);
eq(Array.from(levels), [0, 0, 0, 10, 0, 30, 0, 0, 0, 40, 50, 60], 'block levels at block frames, 0 in pauses');
eq(Array.from(S.audioModel.waveform(report, {})), new Array(12).fill(0), 'no answer gives one 0 per frame');

const inst = { t: key => key, bus: S.createBus(), report, frame: 0, playing: false, readonly: false, selected: 0, audioInfo: { narration }, play() {}, pause() {}, seek() {} };
const transport = S.createTransport(inst);
const scrub = transport.el.find('scrubber');
const wave = scrub.find('scrub-wave');
eq(scrub.kids.map(kid => kid.className), ['stm-scrub-blocks', 'stm-scrub-wave', 'stm-scrub-head'], 'the wave lies between the blocks and the playhead');
const ctx = wave.getContext('2d');
// The stand-in has no layout, so the canvas takes its fallback of 300 x 24. 300 columns over 12 frames is 25 columns per frame.
// Frames 3, 5, 9, 10 and 11 have a level, so 5 * 25 = 125 bars.
eq(ctx.bars.length, 125, 'one bar per column of a frame with a level');
eq(ctx.fillStyle, 'ink', 'the colour comes from the stylesheet');
eq(wave.dataset.drawn, 'true', 'drawn');
// Column 275 is frame 11 (275*12/300). Level 60 is round(60/255*24) = 6 pixels, centred: (24-6)/2 = 9.
eq(ctx.bars.find(bar => bar.x === 275), { x: 275, y: 9, w: 1, h: 6 }, 'the height follows the level');

inst.audioInfo = { narration: {} };
inst.bus.emit('audio');
eq([ctx.bars.length, wave.dataset.drawn], [0, 'false'], 'a fresh answer repaints');
inst.audioInfo = { narration };
inst.bus.emit('report');
eq(ctx.bars.length, 125, 'a new report repaints');

transport.dispose();
console.log('storymaker scrub wave ok');
`

// The music panel in Node, on a small fake DOM and a fake instance: when generated music is offered, that a paid call needs a
// click, that a file or a track starts one job per source, and what the panel says about a missing source and a stopped job.
func TestStorymakerMusicPanelInNode(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-music-panel.js")
	if err := os.WriteFile(script, []byte(storymakerMusicPanelScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	out, err := exec.Command(node, script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker music panel ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerMusicPanelScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const [dir] = process.argv.slice(2);

class FakeNode {
  constructor(tag) { this.tag = String(tag); this.children = []; this.parent = null; this.dataset = {}; this.attrs = {}; this.listeners = {}; this.own = ''; this.value = ''; this.hidden = false; this.disabled = false; this.className = ''; this.style = { setProperty() {} }; }
  appendChild(child) { child.parent = this; this.children.push(child); return child; }
  removeChild(child) { this.children = this.children.filter(item => item !== child); child.parent = null; return child; }
  get firstChild() { return this.children[0] || null; }
  get textContent() { return this.own + this.children.map(child => child.textContent).join(''); }
  set textContent(text) { this.children = []; this.own = String(text); }
  setAttribute(name, value) { this.attrs[name] = String(value); if (name === 'value') this.value = String(value); if (name === 'hidden') this.hidden = true; if (name === 'disabled') this.disabled = true; }
  removeAttribute(name) { delete this.attrs[name]; }
  addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
  removeEventListener() {}
  fire(type) { (this.listeners[type] || []).forEach(fn => fn({ target: this })); }
  click() { if (!this.disabled) this.fire('click'); }
  focus() {}
}
const document = {
  createElement: tag => new FakeNode(tag),
  createElementNS: (ns, tag) => new FakeNode(tag),
  createTextNode: text => { const node = new FakeNode('#text'); node.own = String(text); return node; },
  documentElement: { lang: 'en' },
  activeElement: null
};
const context = { console, document, setTimeout, clearTimeout };
context.globalThis = context; context.window = context;
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-music.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const find = (node, stm) => { if (node.dataset.stm === stm) return node; for (const child of node.children) { const hit = find(child, stm); if (hit) return hit; } return null; };
const flush = () => new Promise(resolve => setImmediate(resolve));
const emptyAnswer = () => ({ narration: {}, missing: [], chars: { total: 0, missing: 0 }, music: null });

function makeInst(overrides) {
  const calls = { music: [], tracks: 0, refresh: 0, cancel: [] };
  const inst = Object.assign({
    t: (key, vars) => (vars ? key + ' ' + JSON.stringify(vars) : key),
    bus: S.createBus(),
    domId: name => 'x-' + name,
    projectId: 'p1',
    status: { music_configured: true },
    readonly: false,
    dirty: false,
    story: { scenes: [{ id: 's1' }], music: null },
    report: { duration_ms: 61000, fps: 30, frame_count: 1830 },
    audioInfo: emptyAnswer(),
    flushEdits: () => Promise.resolve(true),
    refreshAudio() { calls.refresh++; },
    api: {
      music(id, ms, generate) { calls.music.push(generate ? [id, ms, generate] : [id, ms]); return Promise.resolve({ job: { id: 'j' + calls.music.length, kind: 'music', status: 'queued', project_id: 'p1' } }); },
      noisemakerTracks() { calls.tracks++; return Promise.resolve({ items: [{ id: 7, title: 'Rain' }] }); },
      cancelJob(id) { calls.cancel.push(id); return Promise.resolve({ id, kind: 'music', status: 'running', project_id: 'p1' }); }
    },
    editStory(fn) {
      if (inst.readonly) return false;
      const draft = JSON.parse(JSON.stringify(inst.story));
      fn(draft);
      inst.story = draft;
      inst.bus.emit('story');
      return true;
    }
  }, overrides);
  inst.calls = calls;
  return inst;
}

(async () => {
  // generated mode hides without music_configured
  let inst = makeInst({ status: { music_configured: false } });
  let panel = S.createMusicPanel(inst);
  eq(find(panel.el, 'music-mode').children.map(item => item.value), ['none', 'file'], 'generated hides without music_configured');
  inst = makeInst();
  panel = S.createMusicPanel(inst);
  eq(find(panel.el, 'music-mode').children.map(item => item.value), ['none', 'generated', 'file'], 'generated shows when configured');

  // generated music waits for the click, also after a prompt edit and a fresh answer
  inst = makeInst({ story: { scenes: [{ id: 's1' }], music: { mode: 'generated', mood: 'calm-ambient', prompt: 'soft rain' } } });
  panel = S.createMusicPanel(inst);
  await flush();
  eq(inst.calls.music, [], 'generated sends nothing by itself');
  const button = find(panel.el, 'music-generate');
  eq(button.hidden, false, 'the button shows while audioInfo.music is null');
  const summary = find(panel.el, 'music-summary').textContent;
  eq([summary.includes('storymaker.music.mood_calm_ambient'), summary.includes('soft rain')], [true, true], 'the button shows the mood and the prompt');
  const box = find(panel.el, 'music-prompt');
  box.value = 'harder rain';
  box.fire('input');
  inst.bus.emit('audio');
  inst.bus.emit('report');
  await flush();
  eq(inst.calls.music, [], 'a prompt edit and a fresh answer send nothing');
  eq(find(panel.el, 'music-summary').textContent.includes('harder rain'), true, 'the summary follows the edit');
  button.click();
  await flush();
  eq(inst.calls.music, [['p1', 61000, true]], 'the click sends the report duration and asks for the generation');
  eq(button.hidden, true, 'no second click while the job runs');

  // the stop control: only while the job is queued or running, never read-only, and it calls the existing cancel route
  const stop = find(panel.el, 'music-cancel');
  eq(stop.hidden, false, 'stop shows while the job is queued');
  inst.readonly = true;
  inst.bus.emit('readonly');
  eq(stop.hidden, true, 'stop hides when read-only');
  inst.readonly = false;
  inst.bus.emit('readonly');
  stop.click();
  await flush();
  eq(inst.calls.cancel, ['j1'], 'stop cancels the panel job');

  // the panel has its own text for interrupted and cancelled, and ignores other kinds
  inst.bus.emit('job', { id: 'j1', kind: 'music', status: 'interrupted', project_id: 'p1', error: { code: 'interrupted', message: 'm' } });
  eq(find(panel.el, 'music-status').textContent.includes('storymaker.music.interrupted'), true, 'interrupted');
  eq(inst.calls.refresh, 1, 'a finished job refreshes the answer');
  eq(stop.hidden, true, 'stop hides once the job is over');
  button.click();
  await flush();
  inst.bus.emit('job', { id: 'j2', kind: 'music', status: 'cancelled', project_id: 'p1' });
  eq(find(panel.el, 'music-status').textContent.includes('storymaker.music.cancelled'), true, 'cancelled');
  inst.bus.emit('job', { id: 'n1', kind: 'narrate', status: 'running', project_id: 'p1' });
  eq(find(panel.el, 'music-status').textContent.includes('storymaker.music.cancelled'), true, 'a narrate job is not the panel job');

  // a file starts one job per source, with the report's duration and no click
  const opened = [];
  context.AuraDesktopFileDialogs = { open(options) { opened.push(options); return Promise.resolve({ canceled: false, path: 'Documents/Music/other.wav' }); } };
  inst = makeInst({ story: { scenes: [{ id: 's1' }], music: { mode: 'file', file: 'Documents/Music/theme.mp3' } } });
  panel = S.createMusicPanel(inst);
  await flush();
  eq(inst.calls.music, [['p1', 61000]], 'a file starts one job without a click and does not ask for a generation');
  inst.bus.emit('audio');
  inst.bus.emit('report');
  await flush();
  eq(inst.calls.music.length, 1, 'no second call while the job runs');
  inst.bus.emit('job', { id: 'j1', kind: 'music', status: 'succeeded', project_id: 'p1' });
  await flush();
  eq(inst.calls.music.length, 1, 'the same source does not start again');

  // the picker: the desktop file dialog with a filter; a new source is a new job
  find(panel.el, 'music-choose').click();
  await flush();
  eq(opened[0].filters[0].extensions, ['.wav', '.mp3'], 'the picker filters wav and mp3');
  eq(inst.story.music.file, 'Documents/Music/other.wav', 'the picked file is stored');
  eq(inst.calls.music.length, 2, 'a new source starts its own job');

  // file and track stay exclusive
  const track = find(panel.el, 'music-track');
  eq([inst.calls.tracks, track.children.map(item => item.value)], [1, ['', '7']], 'tracks come from Noisemaker once');
  track.value = '7';
  track.fire('change');
  eq(['file' in inst.story.music, inst.story.music.track], [false, '7'], 'a track removes the file');
  find(panel.el, 'music-choose').click();
  await flush();
  eq(['track' in inst.story.music, inst.story.music.file], [false, 'Documents/Music/other.wav'], 'a file removes the track');

  // a missing source is named, starts nothing and offers another file
  const answer = emptyAnswer();
  answer.music_issue = 'source_missing';
  inst = makeInst({ story: { scenes: [{ id: 's1' }], music: { mode: 'file', file: 'Documents/Music/gone.mp3' } }, audioInfo: answer });
  panel = S.createMusicPanel(inst);
  await flush();
  eq(inst.calls.music, [], 'a missing source starts nothing');
  eq(find(panel.el, 'music-missing').textContent.includes('Documents/Music/gone.mp3'), true, 'the missing source is named');
  eq(find(panel.el, 'music-choose').textContent, 'storymaker.music.choose_other', 'another file is offered');

  // a refused start and a failed job name the music budget, not the narration's character limit
  inst = makeInst({ story: { scenes: [{ id: 's1' }], music: { mode: 'generated', mood: 'calm-ambient' } } });
  inst.api.music = () => Promise.reject(S.errors.fromResponse(409, { error: 'budget_exceeded', code: 'budget_exceeded', message: 'm' }, null));
  panel = S.createMusicPanel(inst);
  await flush();
  find(panel.el, 'music-generate').click();
  await flush();
  let said = find(panel.el, 'music-status').textContent;
  eq([said.includes('storymaker.music.budget_exceeded'), said.includes('storymaker.job_error.budget_exceeded')], [true, false], 'a refused start names the music budget');
  inst.bus.emit('job', { id: 'b1', kind: 'music', status: 'queued', project_id: 'p1' });
  inst.bus.emit('job', { id: 'b1', kind: 'music', status: 'failed', project_id: 'p1', error: { code: 'budget_exceeded', message: 'm' } });
  said = find(panel.el, 'music-status').textContent;
  eq([said.includes('storymaker.music.budget_exceeded'), said.includes('storymaker.job_error.budget_exceeded')], [true, false], 'a failed job names the music budget');

  // a refused file keeps the server's reason next to the sentence; a sentence that already holds the message does not repeat it
  const refusedFile = 'The music file must last between 1 second and 1 hour.';
  inst = makeInst({ story: { scenes: [{ id: 's1' }], music: { mode: 'file', file: 'Documents/Music/long.wav' } } });
  inst.api.music = () => Promise.reject(S.errors.fromResponse(422, { error: 'invalid_music_file', code: 'invalid_music_file', message: refusedFile }, null));
  panel = S.createMusicPanel(inst);
  await flush();
  eq([find(panel.el, 'music-status').textContent.includes('storymaker.error.invalid_music_file'), find(panel.el, 'music-refusal-detail').textContent], [true, refusedFile], 'the reason of the server shows');
  inst = makeInst({ story: { scenes: [{ id: 's1' }], music: { mode: 'file', file: 'Documents/Music/long.wav' } } });
  inst.api.music = () => Promise.reject(S.errors.fromResponse(500, { error: 'job_failed', code: 'job_failed', message: 'The music job could not be queued.' }, null));
  panel = S.createMusicPanel(inst);
  await flush();
  eq(find(panel.el, 'music-refusal-detail'), null, 'the generic sentence already holds the message');

  // a film over the limit closes Generate and says why, in the sentence of the export panel
  inst = makeInst({ status: { music_configured: true, limits: { max_duration_seconds: 60 } }, story: { scenes: [{ id: 's1' }], music: { mode: 'generated', mood: 'calm-ambient' } } });
  panel = S.createMusicPanel(inst);
  eq([find(panel.el, 'music-generate').disabled, find(panel.el, 'music-too-long').textContent], [true, 'storymaker.export.blocked_too_long {"max":"60"}'], 'a film of 61 s over a limit of 60 s');
  inst.report = { duration_ms: 60000, fps: 30, frame_count: 1800 };
  inst.bus.emit('report');
  eq([find(panel.el, 'music-generate').disabled, find(panel.el, 'music-too-long').hidden], [false, true], 'a film that fits opens it');

  // a file replaced on disk has a new key: no music, no issue, nothing running and its one job used, so the panel says so and offers to prepare it again
  inst = makeInst({ story: { scenes: [{ id: 's1' }], music: { mode: 'file', file: 'Documents/Music/theme.mp3' } } });
  panel = S.createMusicPanel(inst);
  await flush();
  eq(find(panel.el, 'music-retry'), null, 'no retry while the job runs');
  inst.bus.emit('job', { id: 'j1', kind: 'music', status: 'succeeded', project_id: 'p1' });
  eq(find(panel.el, 'music-retry'), null, 'no retry while the fresh answer is on its way');
  inst.bus.emit('audio');
  eq(find(panel.el, 'music-unprepared').textContent, 'storymaker.export.missing_music', 'the panel says the file is not prepared');
  eq(inst.calls.music.length, 1, 'nothing starts by itself');
  find(panel.el, 'music-retry').click();
  await flush();
  eq(inst.calls.music.length, 2, 'retry prepares it again');
  // the level note: only for stored music that the server could not measure
  const ready = emptyAnswer();
  ready.music = { key: 'k', ext: 'mp3', duration_ms: 61000 };
  inst = makeInst({ story: { scenes: [{ id: 's1' }], music: { mode: 'generated', mood: 'calm-ambient' } }, audioInfo: ready });
  panel = S.createMusicPanel(inst);
  eq(!!find(panel.el, 'music-level-note'), true, 'no measured level: the note shows');
  ready.music.loudness_i = -14;
  inst.bus.emit('audio');
  eq(!!find(panel.el, 'music-level-note'), false, 'a measured level: no note');
  console.log('storymaker music panel ok');
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`

// The music bed and the SFX of the preview in Node, on a fake AudioContext: one <audio> element through MediaElementSource, a
// duck that follows the stored levels, SFX scheduled from audio_events, and URLs that come only from the catalog and the answer.
func TestStorymakerPreviewMusicAndSFXInNode(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/storymaker-audio.js")
	if !strings.Contains(source, "S.createAudio(") {
		t.Error("bindAudio must create its engine through S.createAudio, or it misses the music and SFX layer")
	}
	if strings.Count(source, "S.audioModel.infoFor(inst)") < 2 {
		t.Error("bindAudio must feed createAudio.load with S.audioModel.infoFor(inst)")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-preview-sound.js")
	if err := os.WriteFile(script, []byte(storymakerPreviewSoundScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	out, err := exec.Command(node, script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker preview sound ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerPreviewSoundScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const [dir] = process.argv.slice(2);

const created = [];
const mediaSources = [];
const sources = [];
const fetched = [];
const urlOf = new WeakMap();

class FakeParam {
  constructor() { this.value = 1; this.targets = []; }
  setTargetAtTime(value) { this.targets.push(value); this.value = value; }
  setValueAtTime(value) { this.value = value; }
  linearRampToValueAtTime(value) { this.value = value; }
  cancelScheduledValues() {}
}
class FakeAudioContext {
  constructor() { this.currentTime = 0; this.destination = { name: 'destination' }; this.state = 'running'; }
  createGain() { return { gain: new FakeParam(), connect(target) { return target; }, disconnect() {} }; }
  createBufferSource() {
    return { buffer: null, target: null, connect(target) { this.target = target; return target; }, disconnect() {}, start(when) { this.when = when; sources.push(this); }, stop() { this.stopped = true; } };
  }
  createMediaElementSource(element) {
    const node = { element, target: null, connect(target) { node.target = target; return target; }, disconnect() {} };
    mediaSources.push(node);
    return node;
  }
  decodeAudioData(data) { return Promise.resolve({ url: urlOf.get(data) || '', duration: 1, numberOfChannels: 1, sampleRate: 48000 }); }
  resume() { return Promise.resolve(); }
  suspend() { return Promise.resolve(); }
  close() { return Promise.resolve(); }
}
const document = {
  documentElement: { lang: 'en' },
  createElement(tag) {
    const element = { tag, src: '', currentTime: 0, paused: true, play() { this.paused = false; return Promise.resolve(); }, pause() { this.paused = true; }, load() { this.emptied = !this.src; }, removeAttribute(name) { if (name === 'src') this.src = ''; }, setAttribute(name, value) { this[name] = value; }, addEventListener() {}, removeEventListener() {} };
    created.push(element);
    return element;
  }
};
function fakeFetch(url) {
  fetched.push(String(url));
  return Promise.resolve({
    ok: true, status: 200, headers: { get() { return null; } },
    arrayBuffer() { const data = new ArrayBuffer(8); urlOf.set(data, String(url)); return Promise.resolve(data); },
    json() { return Promise.resolve({}); },
    text() { return Promise.resolve(''); }
  });
}
const context = { console, document, setTimeout, clearTimeout, AbortController, AudioContext: FakeAudioContext, fetch: fakeFetch };
context.globalThis = context; context.window = context;
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-api.js', 'storymaker-audio.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const near = (a, b, tolerance, what) => { if (Math.abs(a - b) > tolerance) throw new Error(what + ': got ' + a + ', want ' + b); };
const flush = () => new Promise(resolve => setImmediate(resolve));

const id = '11111111-2222-4333-8444-555555555555';
const key = 'a'.repeat(64);
const otherKey = 'b'.repeat(64);
const hash = 'c'.repeat(64);
const catalog = { byId: {
  'sfx.pop': { id: 'sfx.pop', kind: 'sfx', hit: { over_db: 4.5, dip_db: 3 }, takes: [{ file: 'sfx/pop_1.wav', l50_lufs: -18 }, { file: 'sfx/pop_2.wav', l50_lufs: -18 }, { file: 'sfx/pop_3.wav', l50_lufs: -20 }] },
  'sfx.evil': { id: 'sfx.evil', kind: 'sfx', takes: [{ file: '../../secret.wav' }] },
  'prop.lock': { id: 'prop.lock', kind: 'prop', file: 'props/lock.json' }
} };
// A block of 30 frames at level byte 140 whose first piece starts at 1000 ms: at 30 fps it covers frames 30 to 59.
const narration = { s1: { blocks: [{ id: 's1:n', kind: 'narration', speaker: 'narrator', duration_ms: 1000, spans: [[0, 1000]], speech_ok: true, levels: new Array(30).fill(140), pieces: [{ hash, ext: 'wav', at_ms: 0, in_ms: 0, out_ms: 1000, chars: 10 }] }] } };
const events = [
  { kind: 'narration', ref: hash, start_ms: 1000, scene_id: 's1', block_id: 's1:n', piece: 0 },
  { kind: 'sfx', ref: 'sfx.pop', start_ms: 1836, scene_id: 's1', take: 2 },
  { kind: 'sfx', ref: '/etc/passwd', start_ms: 1900, scene_id: 's1', take: 0 },
  { kind: 'sfx', ref: 'sfx.pop', start_ms: 1950, scene_id: 's1', take: 9 },
  { kind: 'sfx', ref: 'prop.lock', start_ms: 1960, scene_id: 's1', take: 0 },
  { kind: 'sfx', ref: 'sfx.evil', start_ms: 1970, scene_id: 's1', take: 0 }
];
const info = { fps: 30, frame_count: 300, narration, music: { key, ext: 'mp3', duration_ms: 61000, loudness_i: -14 }, volume: 0.5, catalog };
// The levels of computeMixLevels in mix.go: level byte 140 is -27.06 dB, and the reference is that minus 12 LU plus 6.02 dB for volume 0.5.
const speechDb = 140 * 60 / 255 - 60;
const referenceDb = speechDb - 12 + 20 * Math.log10(0.5 / 0.25);

(async () => {
  // The duck rule: -6 dB from 50 ms before speech, a 50 ms attack ahead of that, a 400 ms release after it. Frames are 33.33 ms at 30 fps.
  const M = S.audioModel;
  const speech = M.speechFrames(events, narration, 30, 300);
  eq([speech[29], speech[30], speech[59], speech[60]], [0, 1, 1, 0], 'frames 30 to 59 speak');
  const full = Math.pow(10, -6 / 20);
  near(M.duckAt(speech, 40, 30), full, 1e-9, 'inside speech');
  near(M.duckAt(speech, 29, 30), full, 1e-9, 'frame 29 is 33 ms before speech, inside the 50 ms look-ahead');
  near(M.duckAt(speech, 28, 30), Math.pow(10, -4 / 20), 1e-9, 'frame 28 is 66.7 ms before speech: depth (100-66.7)/50 = 2/3 of 6 dB');
  near(M.duckAt(speech, 20, 30), 1, 1e-9, 'frame 20 is 333 ms before speech: no duck');
  near(M.duckAt(speech, 65, 30), Math.pow(10, -3 / 20), 1e-9, 'frame 65 is 200 ms after frame 59: half of the 400 ms release');
  near(M.duckAt(speech, 72, 30), 1, 1e-9, 'frame 72 is 433 ms after frame 59: released');

  // Nothing exists before the first play; then one <audio> through MediaElementSource.
  const api = S.createApi({ fetch: fakeFetch });
  const audio = S.createAudio({ api, projectId: id });
  audio.load(info, events);
  eq([created.length, mediaSources.length], [0, 0], 'nothing before the first play');
  audio.play(0);
  await flush();
  const beds = created.filter(item => item.tag === 'audio');
  eq([beds.length, mediaSources.length], [1, 1], 'the bed is one audio element through MediaElementSource');
  eq(beds[0].src, '/api/desktop/storymaker/projects/' + id + '/audio/music/' + key + '.mp3', 'the bed plays the key of the answer');
  eq(beds[0].paused, false, 'the bed plays');

  // SFX: take 2 is the third take, at the event time; the anchor is currentTime 0 minus frame 0, the anchor of the narration.
  const sfx = sources.filter(node => node.buffer && String(node.buffer.url).indexOf('/img/storymaker/') === 0);
  eq(sfx.map(node => node.buffer.url), ['/img/storymaker/sfx/pop_3.wav'], 'one sound, from the catalog take');
  near(sfx[0].when, 1.836, 1e-9, 'the sound starts at its event time');
  const spoken = sources.find(node => node.buffer && String(node.buffer.url).includes(hash));
  near(spoken.when, 1, 1e-9, 'the narration piece at 1000 ms has the same anchor');
  // The level of the export: reference + over_db - the L50 of the take, through a gain node on the way to the destination.
  near(sfx[0].target.gain.value, Math.pow(10, (referenceDb + 4.5 + 20) / 20), 1e-9, 'the sound plays at the level of the export');

  // No URL comes from message text: an unknown ref, a take out of range, a non-sfx entry and a bad file name fetch nothing.
  eq(fetched.filter(url => url.indexOf('/img/storymaker/') === 0), ['/img/storymaker/sfx/pop_3.wav'], 'only the catalog take is fetched');
  eq(fetched.some(url => url.includes('passwd') || url.includes('secret') || url.includes('lock.json')), false, 'nothing else is requested');

  // The duck follows the levels, on top of the export's bed gain: the reference minus the measured level of the track.
  const bedGain = Math.pow(10, (referenceDb + 14) / 20);
  const targets = mediaSources[0].target.gain.targets;
  audio.sync(40);
  near(targets[targets.length - 1], bedGain * full, 1e-9, 'ducked during speech');
  audio.sync(72);
  near(targets[targets.length - 1], bedGain, 1e-9, 'released after speech');

  // A very quiet track stops at the +24 dB cap of the export.
  audio.load(Object.assign({}, info, { music: Object.assign({}, info.music, { loudness_i: -90 }) }), events);
  audio.sync(72);
  near(targets[targets.length - 1], Math.pow(10, 24 / 20), 1e-9, 'the cap of the export');

  // Without a measured level the bed plays the raw file at the story volume.
  audio.load(Object.assign({}, info, { music: { key, ext: 'mp3', duration_ms: 61000 } }), events);
  audio.sync(72);
  near(targets[targets.length - 1], 0.5, 1e-9, 'no measured level');

  // A new key keeps the one element.
  audio.load(Object.assign({}, info, { music: { key: otherKey, ext: 'wav', duration_ms: 1000 } }), events);
  eq(created.filter(item => item.tag === 'audio').length, 1, 'still one audio element');
  eq(beds[0].src, '/api/desktop/storymaker/projects/' + id + '/audio/music/' + otherKey + '.wav', 'the element takes the new key');
  // Removing src ends the download only with a load() after it, for a bed that loses its source and on dispose.
  eq(beds[0].emptied, false, 'the bed holds its file');
  audio.load(Object.assign({}, info, { volume: 0 }), events);
  eq([beds[0].src, beds[0].emptied], ['', true], 'without a source the element lets the file go');
  audio.load(Object.assign({}, info, { music: { key: otherKey, ext: 'wav', duration_ms: 1000 } }), events);
  eq(beds[0].emptied, false, 'the next source loads');
  audio.dispose();
  eq([beds[0].src, beds[0].emptied], ['', true], 'dispose lets the file go');

  // A sound a little behind the clock plays at once, like a late narration piece. One further behind stays silent.
  const heard = sources.length;
  const lateAudio = S.createAudio({ api, projectId: id });
  lateAudio.load({ fps: 30, frame_count: 300, narration: {}, music: null, volume: 0.25, catalog }, [
    { kind: 'sfx', ref: 'sfx.pop', start_ms: 1000, scene_id: 's1', take: 0 },
    { kind: 'sfx', ref: 'sfx.pop', start_ms: 500, scene_id: 's1', take: 1 }
  ]);
  lateAudio.play(31);
  await flush();
  const late = sources.slice(heard);
  eq(late.map(node => node.buffer.url), ['/img/storymaker/sfx/pop_1.wav'], 'frame 31 is 33 ms after the first sound and 533 ms after the second');
  near(late[0].when, 0, 1e-9, 'a late sound starts at once');
  lateAudio.dispose();
  process.stdout.write('storymaker preview sound ok\n', () => process.exit(0));
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`
