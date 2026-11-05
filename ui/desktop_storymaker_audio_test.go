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
