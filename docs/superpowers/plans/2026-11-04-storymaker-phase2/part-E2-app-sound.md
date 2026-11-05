# Part E2: The app for sound

Batch 2C. Run the tasks in order. `outline.md` holds the batch rules, the test matrices and the execution rules ("outline section N"). `interfaces.md` holds the frozen interfaces ("frozen section N"), and its section 10 and `reconcile.md` win over them. A group id such as `G-A-5` names a block of tasks inside a part; the index lists them.

<!-- G-E2-1 -->

### Task E2a: API: `music`, `noisemakerTracks`, `startRender` options, music errors
**Batch:** 2C  **Depends on:** A21, A25a
**Files:**
Modify: `ui/js/desktop/apps/storymaker-api.js`
Modify: `ui/js/desktop/apps/storymaker-core.js`
Test: `ui/desktop_storymaker_api_test.go`

- [ ] **Step 1: Impact**

Run these from the main checkout. Warn on HIGH or CRITICAL and confirm UNKNOWN with a text search, per outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createApi" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "describeError" --direction upstream --repo .
```

- [ ] **Step 2: Write the failing test**

Edit `ui/desktop_storymaker_api_test.go`.

Old text:

```js
  eq([job.id, call.headers['If-Match'], call.headers['Idempotency-Key']], ['j1', '"b"', 'key-1'], 'render start');
```

New text:

```js
  eq([job.id, call.headers['If-Match'], call.headers['Idempotency-Key']], ['j1', '"b"', 'key-1'], 'render start');
  eq([call.body === undefined, 'Content-Type' in call.headers], [true, false], 'a render without options sends no body');
  const renderOptions = { burn_in_captions: true, srt: true, vtt: false, stems: true, quality: 'high' };
  await api.startRender(id, '"b"', 'key-2', renderOptions);
  call = calls.pop();
  eq([call.headers['If-Match'], call.headers['Idempotency-Key'], call.headers['Content-Type'], JSON.parse(call.body)], ['"b"', 'key-2', 'application/json', renderOptions], 'a render with options sends them as JSON');

  // music: the duration goes out as JSON, the answer stays {job}
  answers['POST ' + base + '/projects/' + id + '/music'] = json({ job: { id: 'm1', kind: 'music', status: 'queued' } }, 202);
  const musicAnswer = await api.music(id, 61000);
  call = calls.pop();
  eq([musicAnswer.job.id, call.method, call.headers['Content-Type'], JSON.parse(call.body)], ['m1', 'POST', 'application/json', { duration_ms: 61000 }], 'music start');
  await api.music(id, 61000, true);
  call = calls.pop();
  eq(JSON.parse(call.body), { duration_ms: 61000, generate: true }, 'only the Generate button asks for a generation');
  answers['POST ' + base + '/projects/' + id + '/music'] = json({ job: null });
  eq(await api.music(id, 61000), { job: null }, 'nothing to do answers job null');

  // Noisemaker tracks come from their own route, outside the Storymaker base
  answers['GET /api/desktop/noisemaker/tracks'] = json({ status: 'ok', items: [{ id: 7, title: 'Rain' }], total: 1 });
  const tracks = await api.noisemakerTracks();
  call = calls.pop();
  eq([call.url, call.method, tracks.items.length], ['/api/desktop/noisemaker/tracks', 'GET', 1], 'tracks');

  // musicUrl, which E1a added, refuses everything but its one URL form with an empty string
  const musicKey = 'a'.repeat(64);
  for (const [badKey, badExt] of [[musicKey.toUpperCase(), 'mp3'], ['abc', 'mp3'], ['../' + musicKey, 'mp3'], [musicKey, 'ogg']]) {
    eq(api.musicUrl(id, badKey, badExt), '', 'musicUrl refuses ' + badKey + '.' + badExt);
  }
```

Old text:

```js
  eq(S.errors.describe(t, { status: 422, code: 'code_error' }, 'render'), 'storymaker.job_error.code_error', 'a refused export names the code scene');
```

New text:

```js
  eq(S.errors.describe(t, { status: 422, code: 'code_error' }, 'render'), 'storymaker.job_error.code_error', 'a refused export names the code scene');
  eq(S.errors.describe(t, { status: 422, code: 'invalid_music_file' }), 'storymaker.error.invalid_music_file', 'a refused music file has its own sentence');
  eq(S.errors.describe(t, { status: 409, code: 'music_unavailable' }), 'storymaker.error.music_unavailable', 'missing music generation has its own sentence');
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^TestStorymakerAppApiAndAssetsInNode$' -count=1
```

Expected: FAIL. The output shows a `SyntaxError` from `JSON.parse(undefined)` at the render with options, because `startRender` ignores its fourth argument.

- [ ] **Step 4: Implement**

`musicUrl` is E1a's, so this step adds only `music` and `noisemakerTracks`.

Edit `ui/js/desktop/apps/storymaker-api.js`.

Old text:

```js
            async startRender(id, etag, key) {
```

New text:

```js
            // generate is true only from the music panel's Generate button. Without it the server starts no paid generation.
            music(id, durationMs, generate) {
                const body = { duration_ms: durationMs };
                if (generate) body.generate = true;
                return json('POST', '/projects/' + encodeURIComponent(id) + '/music', { headers: jsonHeaders(), body: JSON.stringify(body) });
            },
            noisemakerTracks() { return json('GET', '/api/desktop/noisemaker/tracks', { raw: true }); },

            async startRender(id, etag, key, options) {
```

Old text:

```js
                const body = await json('POST', '/projects/' + encodeURIComponent(id) + '/render', { headers: { 'If-Match': etag, 'Idempotency-Key': key || S.uuid() } });
```

New text:

```js
                const spec = { headers: { 'If-Match': etag, 'Idempotency-Key': key || S.uuid() } };
                if (options) {
                    spec.headers['Content-Type'] = 'application/json';
                    spec.body = JSON.stringify(options);
                }
                const body = await json('POST', '/projects/' + encodeURIComponent(id) + '/render', spec);
```

Edit `ui/js/desktop/apps/storymaker-core.js`.

Old text:

```js
        else if (e.code === 'invalid_story' || e.status === 422) key = 'storymaker.error.invalid_story';
```

New text:

```js
        else if (e.code === 'invalid_music_file') key = 'storymaker.error.invalid_music_file';
        else if (e.code === 'music_unavailable') key = 'storymaker.error.music_unavailable';
        else if (e.code === 'invalid_story' || e.status === 422) key = 'storymaker.error.invalid_story';
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^TestStorymakerAppApiAndAssetsInNode$' -count=1
```

Expected: PASS. `TestStorymakerAppUsesOnlyKeysItHas` stays red on the two new error keys until E2e, which outline section 3 allows.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-api.js ui/js/desktop/apps/storymaker-core.js ui/desktop_storymaker_api_test.go
git commit -m "feat(storymaker): music and track calls and render options in the app API" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E2b: Music panel
**Batch:** 2C  **Depends on:** E2a
**Files:**
Create: `ui/js/desktop/apps/storymaker-music.js`
Modify: `ui/js/desktop/apps/storymaker.js`
Modify: `ui/js/desktop/core/module-loader.js`
Modify: `ui/css/desktop-app-storymaker.css`
Test: `ui/desktop_storymaker_registration_test.go`
Test: `ui/desktop_storymaker_fixture_test.go`
Test: `ui/desktop_storymaker_audio_test.go`

- [ ] **Step 1: Impact**

Run these from the main checkout. Warn on HIGH or CRITICAL and confirm UNKNOWN with a text search, per outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildEditor" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "storymakerFixture.status" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "storymakerFixture.project" --direction upstream --repo .
```

- [ ] **Step 2: Write the failing test**

Append this to the end of `ui/desktop_storymaker_audio_test.go`, which E1c created. The new code uses only `os`, `os/exec`, `path/filepath`, `strings` and `testing`, and E1c's import block holds exactly these.

```go
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
  console.log('storymaker music panel ok');
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`
```

Edit `ui/desktop_storymaker_registration_test.go`, in the text E1e leaves.

Old text:

```go
"audio", "voice"}
```

New text:

```go
"audio", "voice", "music"}
```

The stand-in tours still compile against the new routes. Edit `ui/desktop_storymaker_fixture_test.go`.

Old text:

```go
		"enabled": true, "read_only": f.readOnly, "ffmpeg": ffmpeg, "browser": browser, "transport": "png", "catalog_version": f.catalog.Version(),
```

New text:

```go
		"enabled": true, "read_only": f.readOnly, "ffmpeg": ffmpeg, "browser": browser, "transport": "png", "catalog_version": f.catalog.Version(), "music_configured": false,
```

Old text:

```go
	case rest == "events" && r.Method == http.MethodGet:
```

New text:

```go
	case rest == "music" && r.Method == http.MethodPost:
		f.writeJSON(w, http.StatusOK, map[string]any{"job": nil})
	case rest == "events" && r.Method == http.MethodGet:
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^TestStorymakerMusicPanelInNode$' -count=1
```

Expected: FAIL with `ENOENT` for `storymaker-music.js`, because the module does not exist yet.

- [ ] **Step 4: Implement**

Create `ui/js/desktop/apps/storymaker-music.js`:

```js
// Storymaker app music panel: mode, mood and prompt, a file or a Noisemaker track, the volume and the state of the music job.
// Generated music costs money and its key changes at every pause in typing, so it starts only from the button that shows the mood
// and the prompt. A file or a track costs nothing, so one job per source starts without a click. Classic script, publishes
// S.createMusicPanel.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const ACTIVE = ['queued', 'running'];
    const DEFAULT_VOLUME = 0.25;
    const MIN_MUSIC_MS = 1000;
    const MODE_KEYS = { none: 'storymaker.music.mode_none', generated: 'storymaker.music.mode_generated', file: 'storymaker.music.mode_file' };
    // MusicMoods of internal/storymaker/music.go, each with a literal key so that the guard test sees it.
    const MOODS = [
        ['upbeat-corporate', 'storymaker.music.mood_upbeat_corporate'],
        ['calm-ambient', 'storymaker.music.mood_calm_ambient'],
        ['inspiring', 'storymaker.music.mood_inspiring'],
        ['playful', 'storymaker.music.mood_playful'],
        ['tech-pulse', 'storymaker.music.mood_tech_pulse'],
        ['dramatic', 'storymaker.music.mood_dramatic'],
        ['lo-fi', 'storymaker.music.mood_lo_fi'],
        ['acoustic', 'storymaker.music.mood_acoustic']
    ];

    function option(value, label) {
        return S.el('option', { value, text: label });
    }

    // inst: {t, bus, api, story, status, report, audioInfo, readonly, dirty, projectId, domId(name), notify(text, kind), editStory(fn, options), flushEdits(), refreshAudio()}
    function createMusicPanel(inst) {
        const { el, icon } = S;
        const t = inst.t;
        let job = null;
        let requestedFor = '';     // the file or track source that already got its one job
        let starting = false;
        let cancelling = false;
        let startError = '';
        let startIssue = '';
        let startDetail = '';      // the server's own reason for a refused start, shown under startError
        let answerDue = false;     // a finished job asked for a fresh answer, so the old one says nothing yet
        let tracksAsked = false;
        let tracks = [];
        let modesKey = '';
        let tracksKey = '';
        let disposed = false;

        function field(labelKey, id, control) {
            return el('div', { class: 'stm-field' }, el('label', { class: 'stm-label', for: id, text: t(labelKey) }), control);
        }

        const modeSelect = el('select', { class: 'stm-select', id: inst.domId('music-mode'), dataset: { stm: 'music-mode' } });
        const moodSelect = el('select', { class: 'stm-select', id: inst.domId('music-mood'), dataset: { stm: 'music-mood' } },
            option('', t('storymaker.music.mood_none')), MOODS.map(([name, key]) => option(name, t(key))));
        const promptBox = el('textarea', { class: 'stm-textarea', id: inst.domId('music-prompt'), rows: '3', dataset: { stm: 'music-prompt' }, placeholder: t('storymaker.music.prompt_placeholder') });
        const moodField = field('storymaker.music.mood', inst.domId('music-mood'), moodSelect);
        const promptField = field('storymaker.music.prompt', inst.domId('music-prompt'), promptBox);
        const notConfigured = el('p', { class: 'stm-hint', dataset: { stm: 'music-not-configured' }, text: t('storymaker.music.not_configured') });
        const summary = el('p', { class: 'stm-hint', dataset: { stm: 'music-summary' } });
        const tooLong = el('p', { class: 'stm-hint stm-music-bad', hidden: true, dataset: { stm: 'music-too-long' } });
        const generateButton = el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'music-generate' }, onClick: () => start(true) }, icon('sparkle'), el('span', { text: t('storymaker.music.generate') }));
        const chooseLabel = el('span', { text: t('storymaker.music.choose_file') });
        const chooseButton = el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'music-choose' }, onClick: () => pickFile() }, icon('folder'), chooseLabel);
        const fileName = el('code', { class: 'stm-path', dataset: { stm: 'music-file' } });
        const fileField = el('div', { class: 'stm-field' }, fileName, chooseButton);
        const trackSelect = el('select', { class: 'stm-select', id: inst.domId('music-track'), dataset: { stm: 'music-track' } });
        const trackField = field('storymaker.music.track', inst.domId('music-track'), trackSelect);
        const volumeInput = el('input', { class: 'stm-input', type: 'range', min: '0', max: '1', step: '0.05', id: inst.domId('music-volume'), dataset: { stm: 'music-volume' } });
        const volumeField = field('storymaker.music.volume', inst.domId('music-volume'), volumeInput);
        const status = el('div', { class: 'stm-music-status', role: 'status', dataset: { stm: 'music-status' } });
        const cancelButton = el('button', { type: 'button', class: 'stm-btn', hidden: true, dataset: { stm: 'music-cancel' }, onClick: () => cancel() }, icon('stop'), el('span', { text: t('storymaker.music.cancel') }));
        const root = el('section', { class: 'stm-music', ariaLabel: t('storymaker.side_music'), dataset: { stm: 'music' } },
            field('storymaker.music.mode', inst.domId('music-mode'), modeSelect), notConfigured, moodField, promptField, summary, tooLong, generateButton, fileField, trackField, volumeField, status, cancelButton);

        function music() {
            const value = inst.story && inst.story.music;
            return value && typeof value === 'object' ? value : { mode: 'none' };
        }
        function isActive() { return !!job && ACTIVE.indexOf(job.status) >= 0; }
        function volumeOf(m) { return typeof m.volume === 'number' ? m.volume : DEFAULT_VOLUME; }
        function sourceOf(m) { return m.track ? 'track|' + m.track : m.file ? 'file|' + m.file : ''; }
        function durationMs() {
            const ms = inst.report ? Number(inst.report.duration_ms) : 0;
            return ms > 0 ? Math.max(MIN_MUSIC_MS, Math.round(ms)) : 0;
        }
        // The /music route refuses a generation for a film over max_duration_seconds. This answers that limit for such a film, else 0.
        function overLimit() {
            const max = Number(inst.status && inst.status.limits && inst.status.limits.max_duration_seconds) || 0;
            return max > 0 && inst.report && inst.report.duration_ms / 1000 > max ? max : 0;
        }
        function moodLabel(name) {
            const hit = MOODS.find(item => item[0] === name);
            return hit ? t(hit[1]) : name ? String(name) : t('storymaker.music.mood_none');
        }
        function refreshAnswer() {
            answerDue = true;
            if (typeof inst.refreshAudio === 'function') inst.refreshAudio();
        }

        // story.music belongs to the story, not to a scene, so it goes through editStory, which refuses read-only itself.
        function editMusic(fn, immediate) {
            inst.editStory(draft => {
                const next = draft.music && typeof draft.music === 'object' ? draft.music : { mode: 'none' };
                fn(next);
                draft.music = next;
            }, { immediate: !!immediate });
        }

        async function pickFile() {
            if (inst.readonly) return;
            const dialogs = window.AuraDesktopFileDialogs;
            if (!dialogs || typeof dialogs.open !== 'function') return;
            const m = music();
            const options = { title: t('storymaker.music.choose_file'), filters: [{ label: t('storymaker.music.file_filter'), extensions: ['.wav', '.mp3'] }] };
            if (m.file) options.initialFile = m.file;
            const result = await dialogs.open(options);
            if (disposed || !result || result.canceled || !result.path) return;
            startError = '';
            startIssue = '';
            editMusic(next => { next.file = String(result.path); delete next.track; }, true);
        }

        async function loadTracks() {
            if (tracksAsked) return;
            tracksAsked = true;
            try {
                const body = await inst.api.noisemakerTracks();
                tracks = body && Array.isArray(body.items) ? body.items : [];
            } catch (err) {
                if (err && err.aborted) return;
                tracks = [];
            }
            paint();
        }

        // generate is true only from the Generate button, the one call that may start a paid generation.
        async function start(generate) {
            if (starting || inst.readonly || disposed) return;
            const ms = durationMs();
            if (!ms) return;
            starting = true;
            startError = '';
            startIssue = '';
            paint();
            try {
                if (!(await inst.flushEdits())) return;
                const answer = await inst.api.music(inst.projectId, ms, generate);
                if (disposed) return;
                if (answer && answer.job) applyJob(answer.job, true);
                else refreshAnswer();
            } catch (err) {
                if (err && err.aborted) return;
                // The music budget shares its code with the narration's character limit, so this panel has its own sentence.
                const refusal = S.errors.normalize(err);
                startError = refusal.code === 'budget_exceeded' ? t('storymaker.music.budget_exceeded') : S.errors.describe(t, err);
                // The server's own reason, as a job failure shows it: the sentence for a refused file cannot say why this file was refused.
                startDetail = !refusal.network && refusal.message && startError.indexOf(refusal.message) < 0 ? refusal.message : '';
                startIssue = err && err.data && err.data.music_issue ? String(err.data.music_issue) : '';
            } finally {
                starting = false;
                paint();
            }
        }

        // The stop control of a queued or running job, on the existing cancel route.
        async function cancel() {
            if (!isActive() || inst.readonly || cancelling) return;
            cancelling = true;
            paint();
            try {
                applyJob(await inst.api.cancelJob(job.id), true);
            } catch (err) {
                if (!(err && err.aborted)) inst.notify(S.errors.describe(t, err), 'error');
            } finally {
                cancelling = false;
                paint();
            }
        }

        // A job event: an old finished job of an earlier session says nothing, a started or running one is taken up.
        function applyJob(next, mine) {
            if (!next || typeof next !== 'object' || next.kind !== 'music') return;
            if (next.project_id && inst.projectId && next.project_id !== inst.projectId) return;
            const same = !!job && job.id === next.id;
            if (!same && !mine && ACTIVE.indexOf(next.status) < 0) return;
            const was = same ? job.status : '';
            job = next;
            if (ACTIVE.indexOf(next.status) < 0 && was && was !== next.status) refreshAnswer();
            paint();
        }

        // One job per source, without a click: a file or a track costs nothing.
        function maybeAutoStart(m) {
            if (m.mode !== 'file' || inst.readonly || inst.dirty || starting || isActive()) return;
            const source = sourceOf(m);
            const answer = inst.audioInfo || null;
            if (!source || source === requestedFor || !answer || answer.music || answer.music_issue || volumeOf(m) <= 0 || !durationMs()) return;
            requestedFor = source;
            start();
        }

        function paintModes(configured) {
            const offered = configured ? ['none', 'generated', 'file'] : ['none', 'file'];
            const key = offered.join();
            if (key === modesKey) return;
            modesKey = key;
            S.clear(modeSelect);
            offered.forEach(name => modeSelect.appendChild(option(name, t(MODE_KEYS[name]))));
        }

        function paintTracks(m) {
            const wanted = String(m.track || '');
            const key = tracks.map(item => item.id).join() + '|' + wanted;
            if (key !== tracksKey) {
                tracksKey = key;
                S.clear(trackSelect);
                trackSelect.appendChild(option('', t('storymaker.music.track_none')));
                tracks.forEach(item => {
                    const id = String(item.id);
                    trackSelect.appendChild(option(id, String(item.title || id).slice(0, 80)));
                });
                if (wanted && !tracks.some(item => String(item.id) === wanted)) trackSelect.appendChild(option(wanted, wanted));
            }
            trackSelect.value = wanted;
        }

        function jobFailureText() {
            if (job.status === 'interrupted') return t('storymaker.music.interrupted');
            if (job.status === 'cancelled') return t('storymaker.music.cancelled');
            if (job.error && job.error.code === 'budget_exceeded') return t('storymaker.music.budget_exceeded');
            return S.errors.describeJob(t, job.error);
        }

        function retryButton() {
            return el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'music-retry' }, disabled: inst.readonly || null, onClick: () => { job = null; requestedFor = ''; startError = ''; paint(); } }, icon('refresh'), el('span', { text: t('storymaker.retry') }));
        }

        function paintStatus(m, answer, issue) {
            S.clear(status);
            if (isActive()) {
                const fraction = Number.isFinite(job.progress) ? S.clamp(job.progress, 0, 1) : 0;
                status.appendChild(el('p', { class: 'stm-hint', text: t('storymaker.music.running') }));
                status.appendChild(el('progress', { class: 'stm-progress', max: '1', value: String(fraction), ariaLabel: t('storymaker.music.running') }));
                return;
            }
            if (m.mode === 'none') return;
            // A failed first read must not look like an empty answer: the reason and a way to read again.
            if (!answer && inst.audioError) {
                status.appendChild(el('p', { class: 'stm-hint stm-music-bad', dataset: { stm: 'music-read-error' }, text: S.errors.describe(t, inst.audioError) }));
                status.appendChild(el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'music-read-retry' }, onClick: () => inst.refreshAudio() }, el('span', { text: t('storymaker.retry') })));
                return;
            }
            if (issue === 'source_missing' && m.mode === 'file') {
                status.appendChild(el('p', { class: 'stm-hint stm-music-bad', dataset: { stm: 'music-missing' }, text: t('storymaker.music.source_missing', { source: String(m.file || m.track || '') }) }));
                return;
            }
            if (job && job.status !== 'succeeded') {
                status.appendChild(el('p', { class: 'stm-hint stm-music-bad', text: jobFailureText() }));
                if (job.status === 'failed' && job.error && job.error.message) status.appendChild(el('code', { class: 'stm-cap-issue', text: String(job.error.message).slice(0, 600) }));
                if (m.mode === 'file') status.appendChild(retryButton());
                return;
            }
            if (startError) {
                status.appendChild(el('p', { class: 'stm-hint stm-music-bad', text: startError }));
                if (startDetail) status.appendChild(el('code', { class: 'stm-cap-issue', dataset: { stm: 'music-refusal-detail' }, text: startDetail.slice(0, 600) }));
                if (m.mode === 'file') status.appendChild(retryButton());
                return;
            }
            // A file replaced on disk has a new key: the answer names no music and no issue, and this source already had its one job, so only a click starts another.
            if (m.mode === 'file' && requestedFor && requestedFor === sourceOf(m) && !starting && !answerDue && answer && !answer.music && !issue && volumeOf(m) > 0) {
                status.appendChild(el('p', { class: 'stm-hint', dataset: { stm: 'music-unprepared' }, text: t('storymaker.export.missing_music') }));
                status.appendChild(retryButton());
                return;
            }
            if (answer && answer.music) status.appendChild(el('p', { class: 'stm-hint', text: t('storymaker.music.ready', { seconds: S.format.seconds(answer.music.duration_ms / 1000) }) }));
        }

        function paint() {
            if (disposed) return;
            const m = music();
            const configured = !!(inst.status && inst.status.music_configured);
            const answer = inst.audioInfo || null;
            const generated = m.mode === 'generated';
            const fromFile = m.mode === 'file';
            const issue = startIssue || (answer && answer.music_issue) || '';
            paintModes(configured);
            modeSelect.value = m.mode || 'none';
            modeSelect.disabled = inst.readonly;
            moodSelect.value = m.mood || '';
            moodSelect.disabled = inst.readonly;
            if (document.activeElement !== promptBox) promptBox.value = m.prompt || '';
            promptBox.disabled = inst.readonly;
            if (document.activeElement !== volumeInput) volumeInput.value = String(volumeOf(m));
            volumeInput.disabled = inst.readonly;
            notConfigured.hidden = !generated || configured;
            moodField.hidden = !generated || !configured;
            promptField.hidden = !generated || !configured;
            const offer = generated && configured && !!answer && !answer.music && volumeOf(m) > 0 && !isActive();
            summary.hidden = !offer;
            summary.textContent = t('storymaker.music.generate_summary', { mood: moodLabel(m.mood), prompt: m.prompt || '-' });
            const over = overLimit();
            tooLong.hidden = !offer || !over;
            tooLong.textContent = over ? t('storymaker.export.blocked_too_long', { max: S.format.seconds(over) }) : '';
            generateButton.hidden = !offer;
            generateButton.disabled = inst.readonly || starting || !durationMs() || over > 0;
            fileField.hidden = !fromFile;
            fileName.hidden = !m.file;
            fileName.textContent = String(m.file || '');
            chooseButton.disabled = inst.readonly;
            chooseLabel.textContent = issue === 'source_missing' ? t('storymaker.music.choose_other') : t('storymaker.music.choose_file');
            trackField.hidden = !fromFile;
            trackSelect.disabled = inst.readonly;
            volumeField.hidden = m.mode === 'none' || !m.mode;
            if (fromFile) {
                loadTracks();
                paintTracks(m);
            }
            cancelButton.hidden = !isActive() || inst.readonly;
            cancelButton.disabled = cancelling;
            paintStatus(m, answer, issue);
            maybeAutoStart(m);
        }

        modeSelect.addEventListener('change', () => editMusic(next => {
            next.mode = modeSelect.value;
            if (next.mode === 'generated' && !next.mood && !next.prompt) next.mood = MOODS[0][0];
        }, true));
        moodSelect.addEventListener('change', () => editMusic(next => {
            if (moodSelect.value) next.mood = moodSelect.value; else delete next.mood;
        }, true));
        promptBox.addEventListener('input', () => editMusic(next => {
            if (promptBox.value) next.prompt = promptBox.value; else delete next.prompt;
        }, false));
        trackSelect.addEventListener('change', () => editMusic(next => {
            if (trackSelect.value) { next.track = trackSelect.value; delete next.file; } else delete next.track;
        }, true));
        volumeInput.addEventListener('change', () => editMusic(next => { next.volume = S.clamp(volumeInput.value, 0, 1); }, true));

        const off = [
            inst.bus.on('story', () => paint()),
            inst.bus.on('report', () => paint()),
            inst.bus.on('status', () => paint()),
            inst.bus.on('readonly', () => paint()),
            inst.bus.on('audio', () => { startIssue = ''; answerDue = false; paint(); }),
            inst.bus.on('job', next => applyJob(next, false))
        ];
        paint();

        return {
            el: root,
            paint,
            dispose() {
                disposed = true;
                off.forEach(fn => fn());
            }
        };
    }

    S.createMusicPanel = createMusicPanel;
})();
```

Edit `ui/js/desktop/core/module-loader.js`, in the text E1e leaves. Make the edit only inside the worktree, because the main checkout holds another session's edits to this file.

Old text:

```js
'/js/desktop/apps/storymaker-voice.js', '/js/desktop/apps/storymaker.js'
```

New text:

```js
'/js/desktop/apps/storymaker-voice.js', '/js/desktop/apps/storymaker-music.js', '/js/desktop/apps/storymaker.js'
```

Edit `ui/js/desktop/apps/storymaker.js`, in the text E1d, E1e and E1f leave. Every old text below is part of one line, and E1e extends some of these lines, so match only the quoted part.

Old text:

```js
const exporter = S.createExport(inst);
```

New text:

```js
const exporter = S.createExport(inst);
        const music = S.createMusicPanel(inst);
```

Old text:

```js
inst.parts.exporter = exporter;
```

New text:

```js
inst.parts.exporter = exporter; inst.parts.music = music;
```

Old text:

```js
const side = el('aside', { class: 'stm-side', dataset: { stm: 'side' } },
```

New text:

```js
sideTabs.push(el('button', {
            type: 'button', class: 'stm-tab', role: 'tab', dataset: { stm: 'side-tab', tab: 'music' }, ariaSelected: String(inst.sideTab === 'music'),
            onClick: () => setSideTab(inst, 'music')
        }, icon('sparkle'), el('span', { text: t('storymaker.side_music') })));
        const side = el('aside', { class: 'stm-side', dataset: { stm: 'side' } },
```

Old text:

```js
const center = el('main', { class: 'stm-center' },
```

New text:

```js
side.appendChild(music.el);
        const center = el('main', { class: 'stm-center' },
```

Old text:

```js
showIssues: () => toggleIssues(inst, true)
```

New text:

```js
showIssues: () => toggleIssues(inst, true),
            showPanel: name => showPanel(inst, name)
```

Old text:

```js
    function allIssues(inst) {
```

New text:

```js
    // An action elsewhere (the export refusals) opens a side panel: the storyboard tab, then the side tab of that name (voice from E1e, music from this task).
    function showPanel(inst, name) {
        if (inst.tab === 'code') setTab(inst, 'stage');
        setSideTab(inst, name);
    }

    function allIssues(inst) {
```

Edit `ui/css/desktop-app-storymaker.css`.

Old text:

```css
/* The code tab has the whole body: the strip and the inspector belong to the storyboard */
```

New text:

```css
/* Music panel: a side tab of its own. With a fourth tab the strip wraps instead of overflowing the side panel. */
.stm-tabs-small { flex-wrap: wrap; }
.stm-music { display: none; padding: 0 var(--ds-space-3) var(--ds-space-3); }
.stm-app [data-sidetab="music"] .stm-music { display: block; }
.stm-app [data-sidetab="music"] .stm-side > :not(.stm-tabs, .stm-music) { display: none; }
.stm-music-status { display: grid; gap: var(--ds-space-2); margin-top: var(--ds-space-2); }
.stm-music-status p { margin: 0; }
.stm-music-bad { color: var(--stm-bad-text); }

/* The code tab has the whole body: the strip and the inspector belong to the storyboard */
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^(TestStorymakerMusicPanelInNode|TestStorymakerLoaderListsEveryScript|TestDesktopStorymakerRegistration|TestStorymakerSourcesAreSafeAndSmall|TestStorymakerStylesheetUsesOnlyTokens)$' -count=1
go vet ./ui
```

Expected: PASS. `TestStorymakerAppUsesOnlyKeysItHas` stays red on the `storymaker.music.*` keys and `storymaker.export.missing_music` until E2e, which outline section 3 allows.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-music.js ui/js/desktop/apps/storymaker.js ui/js/desktop/core/module-loader.js ui/css/desktop-app-storymaker.css ui/desktop_storymaker_registration_test.go ui/desktop_storymaker_fixture_test.go ui/desktop_storymaker_audio_test.go
git commit -m "feat(storymaker): music panel" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E2c: Preview with music and SFX
**Batch:** 2C  **Depends on:** E2b, A6, A25c
**Files:**
Modify: `ui/js/desktop/apps/storymaker-audio.js`
Modify: `ui/js/desktop/apps/storymaker-music.js`
Modify: `ui/js/desktop/apps/storymaker-voice.js`
Modify: `internal/storymaker/music.go`
Modify: `internal/storymaker/narration.go`
Modify: `internal/storymaker/render/meter.go`
Modify: `internal/server/storymaker_narrate.go`
Create: `internal/server/storymaker_music_level.go`
Test: `ui/desktop_storymaker_audio_test.go`
Test: `internal/storymaker/music_test.go`
Test: `internal/storymaker/render/meter_test.go`
Test: `internal/server/storymaker_audio_test.go`

- [ ] **Step 1: Impact**

Run these from the main checkout. Warn on HIGH or CRITICAL and confirm UNKNOWN with a text search, per outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createAudio" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "bindAudio" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "storymakerFinishMusic" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "MusicMeta" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "MusicRef" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "validMusicMeta" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createVoicePanel" --direction upstream --repo .
```

- [ ] **Step 2: Write the failing test**

Append this to the end of `ui/desktop_storymaker_audio_test.go`. The new code uses only `os`, `os/exec`, `path/filepath`, `strings` and `testing`, and E1c's import block holds exactly these.

```go
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
```

A fresh answer with only new music starts no compile, so `refresh` feeds the engine itself (Step 4). One assertion in E1d's `TestStorymakerBindAudioInNode`, in the same file, pins that.

Old text:

```js
  eq(a.loads.length, 2, 'the same narration does not reload');
```

New text:

```js
  eq(a.loads.length, 2, 'the same narration does not reload');
  eq(a.spy.fed.length, 2, 'a fresh answer feeds the audio, because music alone starts no compile');
```

The audition player of the voice panel lets its file go on dispose too. Edit the fake DOM and the end of `TestStorymakerNarrationStatusInNode` in `ui/desktop_storymaker_audio_test.go`.

Old text:

```js
  play() { return Promise.resolve(); }
```

New text:

```js
  play() { return Promise.resolve(); }
  load() { this.loaded = true; }
```

Old text:

```js
  quiet.dispose();
```

New text:

```js
  quiet.dispose();
  eq(f('voice-audio').loaded, true, 'dispose lets the audition file go');
```

The music panel says when the server has no measured level. Edit `storymakerMusicPanelScript` in the same file.

Old text:

```js
  console.log('storymaker music panel ok');
```

New text:

```js
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
```

The store keeps the measured level and refuses one that no meter can read. Append this to the end of `internal/storymaker/music_test.go`. It uses only `context` and `testing`, which the file imports.

```go
func TestMusicMetaKeepsTheMeasuredLevel(t *testing.T) {
	ctx := context.Background()
	mem := NewMemStore()
	ps := NewProjectStore(mem, "Documents/Storymaker")
	proj, _, _, err := ps.Create(ctx, "T", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	key := MusicKeyGenerated("lo-fi", "")
	if err := ps.WriteMusic(ctx, proj.ID, key, "mp3", []byte("ID3")); err != nil {
		t.Fatal(err)
	}
	level := -14.25
	if err := ps.WriteMusicMeta(ctx, proj.ID, key, MusicMeta{V: 1, Ext: "mp3", DurationMS: 61000, LoudnessI: &level}); err != nil {
		t.Fatal(err)
	}
	resolve := func(context.Context, *Music) (string, error) { return key, nil }
	answer, err := ps.AudioAnswer(ctx, proj.ID, &Story{}, resolve)
	if err != nil || answer.Music == nil || answer.Music.LoudnessI == nil || *answer.Music.LoudnessI != level {
		t.Fatalf("answer = %+v, %v", answer.Music, err)
	}
	if _, err := mem.WriteFile(ctx, ps.musicFile(proj.ID, key+".meta.json"), []byte(`{"v":1,"ext":"mp3","duration_ms":1000,"loudness_i":999}`), ""); err != nil {
		t.Fatal(err)
	}
	if meta, _, err := ps.MusicInfo(ctx, proj.ID, key); err != nil || meta.V != 0 {
		t.Fatalf("a level of 999: %+v, %v", meta, err)
	}
}
```

The meter reads a stored file from its input. Edit `internal/storymaker/render/meter_test.go`.

Old text:

```go
import (
	"context"
	"math"
```

New text:

```go
import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
```

Append this to the end of the same file.

```go
// meterToneWAV is a 48 kHz stereo PCM16 WAV of a 1 kHz sine at amplitude 0.1.
func meterToneWAV(seconds int) []byte {
	const rate = 48000
	frames := seconds * rate
	data := make([]byte, 44+frames*4)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+frames*4))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 2)
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], rate*4)
	binary.LittleEndian.PutUint16(data[32:], 4)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(frames*4))
	for i := 0; i < frames; i++ {
		v := int16(0.1 * 32767 * math.Sin(2*math.Pi*1000*float64(i)/rate))
		binary.LittleEndian.PutUint16(data[44+i*4:], uint16(v))
		binary.LittleEndian.PutUint16(data[46+i*4:], uint16(v))
	}
	return data
}

func TestMeterStoredReadsAFileFromItsInput(t *testing.T) {
	ffmpeg := testBinary(t, "ffmpeg")
	// The same reading as TestMeterRealGraph: amplitude 0.1 reads -20 LUFS.
	reading, err := MeterStored(context.Background(), ffmpeg, bytes.NewReader(meterToneWAV(5)))
	if err != nil {
		t.Fatalf("MeterStored: %v", err)
	}
	if reading.Silent || math.Abs(reading.I+20) > 0.5 {
		t.Fatalf("reading = %+v, want about -20 LUFS", reading)
	}
	if _, err := MeterStored(context.Background(), ffmpeg, bytes.NewReader([]byte("this is not audio"))); err == nil {
		t.Fatal("bytes that are no audio were measured")
	}
}
```

The music job measures the stored copy and never fails for it. Edit `internal/server/storymaker_audio_test.go`, whose silent test file now passes the meter too.

Old text:

```go
// musicRouteWAV builds a silent 8 kHz mono PCM16 WAV. The route and the job read its header only.
```

New text:

```go
// musicRouteWAV builds a silent 8 kHz mono PCM16 WAV. The route reads its header only. The job's meter reads silence, so
// the meta keeps no level, and the tests that compare a whole MusicMeta hold with or without FFmpeg.
```

Append this to the end of `internal/server/storymaker_audio_test.go`. It uses `musicKeyAnswer` from `storymaker_music_key_test.go` and only imports the file already has.

```go
// musicToneWAV is musicRouteWAV with a 1 kHz square wave in it, so that the meter reads a level.
func musicToneWAV(seconds float64) []byte {
	data := musicRouteWAV(seconds)
	for i := 44; i+1 < len(data); i += 2 {
		sample := int16(8192)
		if (i-44)/2%8 >= 4 {
			sample = -8192
		}
		binary.LittleEndian.PutUint16(data[i:], uint16(sample))
	}
	return data
}

// The music job measures the stored copy for the preview, and the /narration answer carries the level.
func TestStorymakerMusicJobMeasuresTheStoredCopy(t *testing.T) {
	s, readToken, token := storymakerAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = storymakerTestFFmpeg(t)
	p := createStorymakerProject(t, s, token)
	writeMusicSource(t, s, "Documents/theme.wav", musicToneWAV(2))
	musicRouteStory(t, s, token, &p, `{"mode":"file","file":"Documents/theme.wav"}`)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	done := waitStorymakerMusicJob(t, rt, queueMusicJob(t, rt, p.Project.ID).ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	answer := musicKeyAnswer(t, s, readToken, p.Project.ID)
	if answer.Music == nil || answer.Music.LoudnessI == nil || *answer.Music.LoudnessI > -3 || *answer.Music.LoudnessI < -40 {
		t.Fatalf("music = %+v, want a measured level", answer.Music)
	}
}

// The export measures for itself, so a missing FFmpeg costs the preview its level and never the job.
func TestStorymakerMusicJobSurvivesAMissingMeter(t *testing.T) {
	s, readToken, token := storymakerAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = filepath.Join(t.TempDir(), "no-ffmpeg")
	p := createStorymakerProject(t, s, token)
	writeMusicSource(t, s, "Documents/theme.wav", musicToneWAV(2))
	musicRouteStory(t, s, token, &p, `{"mode":"file","file":"Documents/theme.wav"}`)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	done := waitStorymakerMusicJob(t, rt, queueMusicJob(t, rt, p.Project.ID).ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	answer := musicKeyAnswer(t, s, readToken, p.Project.ID)
	if answer.Music == nil || answer.Music.LoudnessI != nil {
		t.Fatalf("music = %+v, want stored music without a level", answer.Music)
	}
}
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^(TestStorymakerPreviewMusicAndSFXInNode|TestStorymakerBindAudioInNode|TestStorymakerMusicPanelInNode|TestStorymakerNarrationStatusInNode)$' -count=1
go test ./internal/storymaker/... -run '(?i)(MusicMetaKeeps|MeterStored)' -count=1
go test ./internal/server -run '^TestStorymakerMusicJob(Measures|Survives)' -count=1
```

Expected: FAIL. The Go check reports that `S.audioModel.infoFor(inst)` is missing, Node reports `M.speechFrames is not a function`, and `TestStorymakerBindAudioInNode` stops on `a fresh answer feeds the audio, because music alone starts no compile: got 1, want 2`. `TestStorymakerMusicPanelInNode` stops on `no measured level: the note shows: got false, want true`. `TestStorymakerNarrationStatusInNode` stops on `dispose lets the audition file go: got undefined, want true`. The Go packages do not build: `internal/storymaker` and `internal/server` report `LoudnessI` as unknown, and `internal/storymaker/render` reports `undefined: MeterStored`.

- [ ] **Step 4: Implement**

The narration engine of E1c stays as it is. A layer at the end of the module adds the bed and the SFX. The layer sits in a nested function, so its names cannot collide with E1c's.

Edit `ui/js/desktop/apps/storymaker-audio.js`. The old text is the module's closing line, the only line that starts at column 0 with `})`.

Old text:

```js
})();
```

New text:

```js
    // ---- music and SFX: a layer around the narration engine -------------------------------------------------------------------
    (function addMusicAndSfx() {
        const SPEECH_LEVEL = 64;      // -45 dBFS on the 0..255 level scale: (-45 + 60) * 255 / 60 = 63.75, the threshold of mix.go
        const DUCK_DB = 6;
        const LOOK_MS = 50;
        const ATTACK_MS = 50;
        const RELEASE_MS = 400;
        const DRIFT_S = 0.12;
        const LATE_S = 0.05;          // a sound this little behind the clock still plays, at once, as startPiece does for a late piece
        const DEFAULT_VOLUME = 0.25;
        const BED_BELOW_NARRATION_LU = -12;   // bedBelowNarrLU of mix.go
        const BED_NO_NARRATION_LUFS = -18;    // bedNoNarrLUFS of mix.go
        const BED_MAX_GAIN_DB = 24;           // bedMaxGainDB of mix.go
        const SFX_BASE = '/img/storymaker/';
        const SFX_FILE = /^sfx\/[A-Za-z0-9._-]+\.wav$/;

        // Frames in which speech sounds above the threshold, from the stored per-frame levels of each block.
        function speechFrames(events, narration, fps, frameCount) {
            const speech = new Uint8Array(Math.max(0, Math.floor(frameCount) || 0));
            const blocks = {};
            Object.keys(narration || {}).forEach(sceneId => {
                const scene = narration[sceneId];
                (scene && Array.isArray(scene.blocks) ? scene.blocks : []).forEach(block => { if (block && block.id) blocks[block.id] = block; });
            });
            const done = new Set();
            (Array.isArray(events) ? events : []).forEach(event => {
                const block = event ? blocks[event.block_id] : null;
                if (!block || done.has(block.id) || !Array.isArray(block.pieces) || !Array.isArray(block.levels)) return;
                const piece = block.pieces[Number(event.piece) || 0];
                if (!piece) return;
                done.add(block.id);
                const first = Math.round((event.start_ms - piece.at_ms) / 1000 * fps);
                block.levels.forEach((level, k) => {
                    const frame = first + k;
                    if (frame >= 0 && frame < speech.length && level >= SPEECH_LEVEL) speech[frame] = 1;
                });
            });
            return speech;
        }

        // The bed's linear gain at a frame: full depth from 50 ms before speech, a 50 ms attack ahead of that, a 400 ms release after it.
        function duckAt(speech, frame, fps) {
            const msPerFrame = 1000 / fps;
            let depth = 0;
            const ahead = Math.ceil((LOOK_MS + ATTACK_MS) / msPerFrame);
            for (let k = 0; k <= ahead; k++) {
                if (!speech[frame + k]) continue;
                const ms = k * msPerFrame;
                depth = Math.max(depth, ms <= LOOK_MS ? 1 : Math.max(0, 1 - (ms - LOOK_MS) / ATTACK_MS));
                break;
            }
            const behind = Math.ceil(RELEASE_MS / msPerFrame);
            for (let k = 0; k <= behind; k++) {
                if (!speech[frame - k]) continue;
                depth = Math.max(depth, Math.max(0, 1 - k * msPerFrame / RELEASE_MS));
                break;
            }
            return Math.pow(10, -DUCK_DB * depth / 20);
        }

        // The narration level of computeMixLevels in mix.go, from the stored levels: the power mean of the speech frames on the byte
        // scale of the envelopes (level * 60 / 255 - 60 dB). null without speech.
        function narrationLufs(narration) {
            let sum = 0;
            let count = 0;
            Object.keys(narration || {}).forEach(sceneId => {
                const scene = narration[sceneId];
                (scene && Array.isArray(scene.blocks) ? scene.blocks : []).forEach(block => {
                    (block && Array.isArray(block.levels) ? block.levels : []).forEach(level => {
                        if (!(level >= SPEECH_LEVEL)) return;
                        sum += Math.pow(10, (level * 60 / 255 - 60) / 10);
                        count++;
                    });
                });
            });
            return count ? 10 * Math.log10(sum / count) : null;
        }

        // The level that bed and SFX aim at in the export: narration minus 12 LU, or -18 LUFS without narration, moved by the
        // volume of a bed relative to 0.25.
        function referenceLevel(info) {
            const narration = narrationLufs(info.narration);
            const base = narration === null ? BED_NO_NARRATION_LUFS : narration + BED_BELOW_NARRATION_LU;
            const volume = Number(info.volume);
            return info.music && volume > 0 ? base + 20 * Math.log10(volume / DEFAULT_VOLUME) : base;
        }

        // The info of createAudio.load from the instance. The volume is the story's, the music comes from the /narration answer.
        function infoFor(inst) {
            const answer = inst.audioInfo || {};
            const report = inst.report || {};
            const music = inst.story && inst.story.music;
            return {
                fps: report.fps,
                frame_count: report.frame_count,
                narration: answer.narration || {},
                music: answer.music || null,
                volume: music && typeof music.volume === 'number' ? music.volume : DEFAULT_VOLUME,
                catalog: inst.catalog || null
            };
        }

        const narrationEngine = S.createAudio;

        S.createAudio = function createAudioWithSound(options) {
            const engine = narrationEngine(options);
            const api = options.api;
            const projectId = options.projectId;
            let info = {};
            let sfxEvents = [];
            let speech = new Uint8Array(0);
            let reference = BED_NO_NARRATION_LUFS;   // what referenceLevel gave for the current info
            let ctx = null;
            let playing = false;
            let last = 0;
            let anchor = 0;       // the context time of film time 0
            let generation = 0;   // a decode that ends after a new schedule is dropped
            let bed = null;
            let disposed = false;
            const buffers = new Map();   // take file -> Promise of its AudioBuffer
            const live = new Map();      // event key -> the source node

            function fps() { return Number(info.fps) > 0 ? Number(info.fps) : 30; }

            function bedUrl() {
                const music = info.music;
                if (!music || !(Number(info.volume) > 0)) return '';
                return api.musicUrl(projectId, music.key, music.ext);
            }

            // Only what can really sound opens a second AudioContext: a bed, or an sfx event whose take is in the catalog. E1c's test feeds an
            // sfx event without a catalog and counts one context.
            function needsSound() { return !!bedUrl() || sfxEvents.some(event => takeOf(event)); }

            function openContext() {
                if (ctx || disposed) return ctx;
                const Ctor = window.AudioContext || window.webkitAudioContext;
                if (!Ctor) return null;
                ctx = new Ctor();
                return ctx;
            }

            // One <audio> element for the whole session: a new key only changes its src. Taking the src away needs load() after it, or the
            // element keeps its connection and the file handle open.
            function syncBed() {
                const url = bedUrl();
                if (!url) {
                    if (bed && bed.url) { bed.url = ''; bed.element.pause(); bed.element.removeAttribute('src'); bed.element.load(); }
                    return;
                }
                if (!ctx) return;
                if (!bed) {
                    const element = document.createElement('audio');
                    element.preload = 'auto';
                    element.loop = true;
                    const source = ctx.createMediaElementSource(element);
                    const gain = ctx.createGain();
                    source.connect(gain);
                    gain.connect(ctx.destination);
                    bed = { element, source, gain, url: '' };
                }
                if (bed.url !== url) {
                    bed.url = url;
                    bed.element.src = url;
                    bed.element.load();
                }
            }

            function placeBed(frame) {
                if (!bed || !bed.url || !info.music) return;
                const length = Number(info.music.duration_ms) / 1000;
                const seconds = frame / fps();
                const want = length > 0 ? seconds % length : seconds;
                if (Math.abs(bed.element.currentTime - want) > DRIFT_S) bed.element.currentTime = want;
            }

            function applyDuck(frame) {
                if (!bed || !bed.url || !ctx) return;
                // The export's bed gain when the server measured the track. Otherwise the raw file at the story volume, and the music panel says so.
                const measured = info.music ? info.music.loudness_i : null;
                const level = Number.isFinite(measured) ? Math.pow(10, Math.min(reference - measured, BED_MAX_GAIN_DB) / 20) : S.clamp(info.volume, 0, 1);
                bed.gain.gain.setTargetAtTime(level * duckAt(speech, frame, fps()), ctx.currentTime, 0.02);
            }

            // Only a take of a catalog sfx entry gives a file: ref and take are lookups, never parts of a URL.
            function takeOf(event) {
                const catalog = info.catalog;
                const entry = catalog && catalog.byId ? catalog.byId[event.ref] : null;
                if (!entry || entry.kind !== 'sfx' || !Array.isArray(entry.takes) || !Number.isInteger(event.take)) return null;
                const take = entry.takes[event.take];
                return take && typeof take.file === 'string' && SFX_FILE.test(take.file) ? { entry, take } : null;
            }

            // The gain of an effect in the export: the reference plus the entry's over_db minus the take's L50 (Mix in mix.go).
            function sfxGain(found) {
                const over = found.entry.hit && Number.isFinite(found.entry.hit.over_db) ? found.entry.hit.over_db : 0;
                return Number.isFinite(found.take.l50_lufs) ? Math.pow(10, (reference + over - found.take.l50_lufs) / 20) : 1;
            }

            function decode(file) {
                if (!buffers.has(file)) {
                    buffers.set(file, window.fetch(SFX_BASE + file, { credentials: 'same-origin' })
                        .then(response => {
                            if (!response.ok) throw new Error('sound effect not found');
                            return response.arrayBuffer();
                        })
                        .then(data => ctx.decodeAudioData(data)));
                }
                return buffers.get(file);
            }

            function scheduleWindow(frame) {
                if (!ctx || !playing) return;
                S.audioModel.windowFor(sfxEvents, frame / fps() * 1000).forEach(event => {
                    const eventKey = event.start_ms + '|' + event.ref + '|' + event.take;
                    if (live.has(eventKey)) return;
                    const when = anchor + event.start_ms / 1000;
                    if (when < ctx.currentTime - LATE_S) return;
                    const found = takeOf(event);
                    if (!found) return;
                    const level = sfxGain(found);
                    live.set(eventKey, null);
                    const seen = generation;
                    decode(found.take.file).then(buffer => {
                        if (disposed || seen !== generation || !playing || !buffer) return;
                        const at = anchor + event.start_ms / 1000;
                        const now = ctx.currentTime;
                        if (at < now - LATE_S) return;
                        const node = ctx.createBufferSource();
                        const gain = ctx.createGain();
                        node.buffer = buffer;
                        gain.gain.value = level;
                        node.connect(gain);
                        gain.connect(ctx.destination);
                        node.onended = () => gain.disconnect();
                        node.start(Math.max(at, now));
                        live.set(eventKey, { node, gain });
                    }, () => { /* a sound effect that cannot load stays silent in the preview */ });
                });
            }

            function stopSfx() {
                generation++;
                live.forEach(item => {
                    if (!item) return;
                    try { item.node.stop(); } catch (_) { /* already over */ }
                    item.gain.disconnect();
                });
                live.clear();
            }

            function begin(frame) {
                anchor = ctx.currentTime - frame / fps();   // the anchor of the narration engine, so a sound lands on the word
                stopSfx();
                scheduleWindow(frame);
                if (bed && bed.url) {
                    placeBed(frame);
                    applyDuck(frame);
                    const started = bed.element.play();
                    if (started && typeof started.catch === 'function') started.catch(() => { /* the owner shows the transport state */ });
                }
            }

            return {
                load(nextInfo, events) {
                    info = nextInfo || {};
                    const all = Array.isArray(events) ? events : [];
                    sfxEvents = all.filter(event => event && event.kind === 'sfx');
                    speech = speechFrames(all, info.narration, fps(), Number(info.frame_count) || 0);
                    reference = referenceLevel(info);
                    engine.load(nextInfo, all.filter(event => event && event.kind !== 'sfx'));
                    syncBed();
                    if (playing && needsSound() && openContext()) {
                        syncBed();
                        begin(last);
                    }
                },
                play(frame) {
                    engine.play(frame);
                    last = frame;
                    playing = true;
                    if (!needsSound() || !openContext()) return;
                    if (typeof ctx.resume === 'function') ctx.resume().catch(() => { /* the next play tries again */ });
                    syncBed();
                    begin(frame);
                },
                pause() {
                    engine.pause();
                    playing = false;
                    stopSfx();
                    if (bed) bed.element.pause();
                },
                seek(frame) {
                    engine.seek(frame);
                    last = frame;
                    if (!ctx) return;
                    if (playing) { begin(frame); return; }
                    stopSfx();
                    placeBed(frame);
                    applyDuck(frame);
                },
                sync(frame) {
                    engine.sync(frame);
                    last = frame;
                    if (!playing || !ctx) return;
                    if (Math.abs(ctx.currentTime - (anchor + frame / fps())) > DRIFT_S) begin(frame);
                    else scheduleWindow(frame);
                    placeBed(frame);
                    applyDuck(frame);
                },
                dispose() {
                    disposed = true;
                    engine.dispose();
                    stopSfx();
                    if (bed) {
                        bed.element.pause();
                        bed.element.removeAttribute('src');
                        bed.element.load();
                        bed.source.disconnect();
                        bed.gain.disconnect();
                        bed = null;
                    }
                    if (ctx && typeof ctx.close === 'function') ctx.close().catch(() => { /* gone */ });
                    ctx = null;
                }
            };
        };

        S.audioModel.speechFrames = speechFrames;
        S.audioModel.duckAt = duckAt;
        S.audioModel.infoFor = infoFor;
    })();
})();
```

Then make two edits inside E1d's `bindAudio`, in the same file. E1d already creates its engine through `S.createAudio(`, so it gets the layer without an edit.

The report handler feeds the engine with the info of the layer.

Old text:

```js
                const answer = inst.audioInfo || {};
                // volume stays 1 until the story has music
                audio.load({ fps: report.fps, frame_count: report.frame_count, narration: answer.narration || {}, music: answer.music || null, volume: 1, catalog: inst.catalog }, report.audio_events);
```

New text:

```js
                audio.load(S.audioModel.infoFor(inst), report.audio_events);
```

A fresh answer feeds the engine too.

Old text:

```js
            inst.bus.emit('audio');
```

New text:

```js
            // An answer can change only the music, and then no new compile follows, so the answer feeds the engine here.
            if (inst.report) audio.load(S.audioModel.infoFor(inst), inst.report.audio_events);
            inst.bus.emit('audio');
```

The music panel says when the bed level is only approximate. Edit `ui/js/desktop/apps/storymaker-music.js`.

Old text:

```js
            if (answer && answer.music) status.appendChild(el('p', { class: 'stm-hint', text: t('storymaker.music.ready', { seconds: S.format.seconds(answer.music.duration_ms / 1000) }) }));
```

New text:

```js
            if (answer && answer.music) status.appendChild(el('p', { class: 'stm-hint', text: t('storymaker.music.ready', { seconds: S.format.seconds(answer.music.duration_ms / 1000) }) }));
            if (answer && answer.music && !Number.isFinite(answer.music.loudness_i)) status.appendChild(el('p', { class: 'stm-hint', dataset: { stm: 'music-level-note' }, text: t('storymaker.music.preview_level_note') }));
```

The audition player of the voice panel lets its file go the same way. Edit `ui/js/desktop/apps/storymaker-voice.js`, in the text E1e leaves.

Old text:

```js
                player.removeAttribute('src');
```

New text:

```js
                player.removeAttribute('src');
                player.load();
```

The bed gain needs the level of the track, which only the server can measure. The music job measures the stored copy, the meta keeps the value and the `/narration` answer reads it from the meta, so that answer still decodes nothing. Edit `internal/storymaker/music.go`.

Old text:

```go
	DurationMS int    `json:"duration_ms"` // 1..3600000
}
```

New text:

```go
	DurationMS int    `json:"duration_ms"` // 1..3600000
	// LoudnessI is the integrated loudness of the file in LUFS, for the preview's bed gain. The music job measures it. It is
	// nil without FFmpeg, after a failed meter or for a silent file. The mix measures for itself.
	LoudnessI *float64 `json:"loudness_i,omitempty"`
}
```

Old text:

```go
	return m.V == 1 && validMusicExt(m.Ext) && m.DurationMS >= 1 && m.DurationMS <= 3600000
```

New text:

```go
	if m.LoudnessI != nil && (*m.LoudnessI < -100 || *m.LoudnessI > 20) {
		return false
	}
	return m.V == 1 && validMusicExt(m.Ext) && m.DurationMS >= 1 && m.DurationMS <= 3600000
```

Edit `internal/storymaker/narration.go`.

Old text:

```go
	DurationMS int    `json:"duration_ms"`
}

// AudioAnswer is the body of GET /projects/{id}/narration.
```

New text:

```go
	DurationMS int    `json:"duration_ms"`
	// LoudnessI is MusicMeta.LoudnessI, for the preview's bed gain. Absent without a measurement.
	LoudnessI *float64 `json:"loudness_i,omitempty"`
}

// AudioAnswer is the body of GET /projects/{id}/narration.
```

Old text:

```go
		answer.Music = &MusicRef{Key: key, Ext: audioExt, DurationMS: meta.DurationMS}
```

New text:

```go
		answer.Music = &MusicRef{Key: key, Ext: audioExt, DurationMS: meta.DurationMS, LoudnessI: meta.LoudnessI}
```

Edit `internal/storymaker/render/meter.go`. The new function has the filter and the parser of `Meter`, and FFmpeg reads the whole file from its stdin.

Old text:

```go
// parseMeterOutput reads I from the last I: line of ebur128's summary and TP from the loudnorm JSON
```

New text:

```go
// MeterStored measures a stored WAV or MP3 with the graph of Meter. FFmpeg reads the file from src on its stdin, so it sees
// only these bytes. ctx ends it.
func MeterStored(ctx context.Context, ffmpegPath string, src io.Reader) (MeterReading, error) {
	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-hide_banner", "-nostdin", "-nostats", "-loglevel", "info",
		"-protocol_whitelist", "file,pipe", "-format_whitelist", "wav,mp3", "-i", "pipe:0",
		"-map", "0:a:0", "-ar", "48000", "-ac", "2", "-t", "3600",
		"-af", meterFilter, "-f", "null", "-")
	cmd.Stdin = src
	stderr := &tailBuffer{limit: ffmpegStderrKeep}
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return MeterReading{}, fmt.Errorf("storymaker: the FFmpeg meter failed: %w", err)
	}
	return parseMeterOutput(stderr.String())
}

// parseMeterOutput reads I from the last I: line of ebur128's summary and TP from the loudnorm JSON
```

Edit `internal/server/storymaker_narrate.go`, in the text A25c leaves.

Old text:

```go
	return storymakerFinishMusic(ctx, rt, p.ProjectID, key, ext)
```

New text:

```go
	return storymakerFinishMusic(ctx, rt, cfg, p.ProjectID, key, ext)
```

Old text:

```go
func storymakerFinishMusic(ctx context.Context, rt *storymakerRuntime, id, key, ext string) error {
```

New text:

```go
func storymakerFinishMusic(ctx context.Context, rt *storymakerRuntime, cfg *config.Config, id, key, ext string) error {
```

A cancel during the measurement must not store a meta without a level, or a rerun would answer `{job:null}` and never measure.

Old text:

```go
	if err := rt.projects.WriteMusicMeta(ctx, id, key, storymaker.MusicMeta{V: 1, Ext: ext, DurationMS: durationMS}); err != nil {
```

New text:

```go
	loudness := storymakerMeasureMusic(ctx, rt, cfg, id, file)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := rt.projects.WriteMusicMeta(ctx, id, key, storymaker.MusicMeta{V: 1, Ext: ext, DurationMS: durationMS, LoudnessI: loudness}); err != nil {
```

Create `internal/server/storymaker_music_level.go`:

```go
package server

import (
	"context"

	"aurago/internal/config"
	"aurago/internal/storymaker/render"
	"aurago/internal/videostudio"
)

// storymakerMeasureMusic measures a stored music file, so that the preview can play the bed at the level of the export. The
// export measures again for itself, so this is best effort: no FFmpeg, a failed meter or a silent file gives nil and never
// fails the job. The music panel then says that the preview's level is approximate.
func storymakerMeasureMusic(ctx context.Context, rt *storymakerRuntime, cfg *config.Config, id, file string) *float64 {
	ffmpeg, err := videostudio.ResolveFFmpeg(cfg.VideoStudio.FFmpegPath)
	if err != nil {
		return nil
	}
	r, _, err := rt.projects.OpenAudio(ctx, id, "music", file)
	if err != nil {
		return nil
	}
	defer r.Close()
	reading, err := render.MeterStored(ctx, ffmpeg, r)
	if err != nil || reading.Silent || reading.I < -100 || reading.I > 20 {
		return nil
	}
	return &reading.I
}
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
gofmt -w internal/storymaker/music.go internal/storymaker/narration.go
go test ./ui -run '(?i)(audio|voice|music|NarrationStatus)' -count=1
go test ./ui -run '^TestStorymakerSourcesAreSafeAndSmall$' -count=1
go test ./internal/storymaker/... -count=1
go test ./internal/server -run 'TestStorymaker(Music|Narration)' -count=1 -timeout 20m
gofmt -l internal/storymaker internal/server
go vet ./internal/storymaker/... ./internal/server
```

Expected: PASS. This includes the tests of E1c to E1g, which must not notice the layer, because it adds nothing without music or a catalog take. `gofmt -l` prints nothing. `TestStorymakerAppUsesOnlyKeysItHas` stays red on `storymaker.music.preview_level_note` until E2e, which outline section 3 allows.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-audio.js ui/js/desktop/apps/storymaker-music.js ui/js/desktop/apps/storymaker-voice.js internal/storymaker/music.go internal/storymaker/narration.go internal/storymaker/render/meter.go internal/server/storymaker_narrate.go internal/server/storymaker_music_level.go ui/desktop_storymaker_audio_test.go internal/storymaker/music_test.go internal/storymaker/render/meter_test.go internal/server/storymaker_audio_test.go
git commit -m "feat(storymaker): preview with music bed and SFX" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E2d: Export errors, missing audio actions, silent note
**Batch:** 2C  **Depends on:** A24
**Files:**
Modify: `ui/js/desktop/apps/storymaker-export.js`
Modify: `ui/js/desktop/apps/storymaker-core.js`
Modify: `ui/js/desktop/apps/storymaker-inspector.js`
Test: `ui/desktop_storymaker_model_test.go`

- [ ] **Step 1: Impact**

Run these from the main checkout. Warn on HIGH or CRITICAL and confirm UNKNOWN with a text search, per outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createExport" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createInspector" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "describeError" --direction upstream --repo .
```

- [ ] **Step 2: Write the failing test**

Edit `ui/desktop_storymaker_model_test.go`. The file already imports `os`, `os/exec`, `path/filepath`, `regexp`, `strings` and `testing`.

Old text:

```go
const storymakerModelScript = `
```

New text:

```go
// The export panel in Node, on a small fake DOM: a refused start shows what is missing and leads to the panel that fixes it, and the
// silent note appears only for a film that really has no sound.
func TestStorymakerExportRefusalsInNode(t *testing.T) {
	t.Parallel()
	if strings.Contains(readDesktopAssetText(t, "js/desktop/apps/storymaker-export.js"), "silent_note") {
		t.Error("storymaker-export.js still names storymaker.export.silent_note")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-export-refusals.js")
	if err := os.WriteFile(script, []byte(storymakerExportRefusalsScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	out, err := exec.Command(node, script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker export refusals ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerExportRefusalsScript = `
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
  querySelector() { return null; }
  contains() { return true; }
}
const document = {
  createElement: tag => new FakeNode(tag),
  createElementNS: (ns, tag) => new FakeNode(tag),
  createTextNode: text => { const node = new FakeNode('#text'); node.own = String(text); return node; },
  addEventListener() {},
  removeEventListener() {},
  documentElement: { lang: 'en' },
  activeElement: null
};
const context = { console, document, setTimeout, clearTimeout };
context.globalThis = context; context.window = context;
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-export.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const find = (node, stm) => { if (node.dataset.stm === stm) return node; for (const child of node.children) { const hit = find(child, stm); if (hit) return hit; } return null; };
const flush = () => new Promise(resolve => setImmediate(resolve));
const t = (key, vars) => (vars ? key + ' ' + JSON.stringify(vars) : key);
const quietAnswer = () => ({ narration: {}, missing: [], chars: { total: 0, missing: 0 }, music: null });

function makeInst(refusal, overrides) {
  const inst = Object.assign({
    t,
    bus: S.createBus(),
    status: { ffmpeg: { ready: true }, browser: { ready: true, source: 'system' } },
    readonly: false,
    projectId: 'p1',
    etag: '"e"',
    story: { format: { aspect: '16:9' }, scenes: [{ id: 's1' }] },
    report: { frame_count: 300, fps: 30, duration_ms: 10000, scenes: [{ id: 's1', start_frame: 0, end_frame: 300 }], audio_events: [], issues: [] },
    audioInfo: quietAnswer(),
    issues: [],
    ctx: {},
    notes: [],
    shown: [],
    flushEdits: () => Promise.resolve(true),
    showIssues() {},
    notify(text) { inst.notes.push(text); },
    showPanel(name) { inst.shown.push(name); },
    api: { startRender() { return Promise.reject(refusal); } }
  }, overrides);
  return inst;
}

(async () => {
  // narration_incomplete names the missing scenes and characters and leads to the voice panel
  let refusal = S.errors.fromResponse(409, { error: 'narration_incomplete', code: 'narration_incomplete', message: 'm', missing: [{ scene_id: 's2', block_id: 's2:n', chars: 120 }, { scene_id: 's2', block_id: 's2:b1', chars: 30 }, { scene_id: 's4', block_id: 's4:n', chars: 10 }], chars: { total: 640, missing: 160 } }, null);
  let inst = makeInst(refusal);
  let exporter = S.createExport(inst);
  exporter.open();
  find(exporter.el, 'export-start').click();
  await flush();
  eq(find(exporter.el, 'export-refused').dataset.code, 'narration_incomplete', 'the refusal is shown');
  const missing = find(exporter.el, 'export-missing').textContent;
  eq([missing.includes('s2, s4'), missing.includes('160')], [true, true], 'the missing scenes and characters are named');
  eq(inst.notes, ['storymaker.job_error.narration_incomplete'], 'the sentence of the refusal');
  find(exporter.el, 'export-go-voice').click();
  eq(inst.shown, ['voice'], 'the action leads to the voice panel');
  exporter.dispose();

  // music_incomplete leads to the music panel
  refusal = S.errors.fromResponse(409, { error: 'music_incomplete', code: 'music_incomplete', message: 'm', key: 'a'.repeat(64) }, null);
  inst = makeInst(refusal);
  exporter = S.createExport(inst);
  exporter.open();
  find(exporter.el, 'export-start').click();
  await flush();
  eq(find(exporter.el, 'export-missing').textContent, 'storymaker.export.missing_music', 'a music file that is not ready');
  find(exporter.el, 'export-go-music').click();
  eq(inst.shown, ['music'], 'the action leads to the music panel');
  exporter.dispose();

  // a moved source says so
  refusal = S.errors.fromResponse(409, { error: 'music_incomplete', code: 'music_incomplete', message: 'm', key: '', music_issue: 'source_missing' }, null);
  inst = makeInst(refusal);
  exporter = S.createExport(inst);
  exporter.open();
  find(exporter.el, 'export-start').click();
  await flush();
  eq(find(exporter.el, 'export-missing').textContent, 'storymaker.export.missing_music_source', 'a moved source');
  exporter.dispose();

  // silent_film only with no narration, no music and empty audio_events
  const report = { audio_events: [] };
  eq(S.isSilentFilm(quietAnswer(), report), true, 'a film without any sound');
  eq(S.isSilentFilm(Object.assign(quietAnswer(), { narration: { s1: { blocks: [{ id: 's1:n' }] } } }), report), false, 'narration');
  eq(S.isSilentFilm(Object.assign(quietAnswer(), { music: { key: 'k', ext: 'mp3', duration_ms: 1000 } }), report), false, 'music');
  eq(S.isSilentFilm(Object.assign(quietAnswer(), { missing: [{ scene_id: 's1', block_id: 's1:n', chars: 5 }] }), report), false, 'narration text without audio');
  eq(S.isSilentFilm(quietAnswer(), { audio_events: [{ kind: 'sfx', ref: 'sfx.pop', start_ms: 1, scene_id: 's1', take: 0 }] }), false, 'an sfx event');
  eq(S.isSilentFilm(null, report), false, 'an answer that has not arrived');
  inst = makeInst(null);
  exporter = S.createExport(inst);
  exporter.open();
  eq(find(exporter.el, 'export-silent').textContent, 'storymaker.export.silent_film', 'the note shows for a silent film');
  exporter.dispose();
  inst = makeInst(null, { audioInfo: Object.assign(quietAnswer(), { music: { key: 'k', ext: 'mp3', duration_ms: 1000 } }) });
  exporter = S.createExport(inst);
  exporter.open();
  eq(find(exporter.el, 'export-silent'), null, 'no note with music');
  exporter.dispose();
  // A panel that is gone is done with its job: a poll that was due asks for nothing, and an answer that arrives later starts no poll.
  const running = { id: 'r1', project_id: 'p1', kind: 'render', status: 'running', progress: 0 };
  const timers = [];
  const realSetTimeout = context.setTimeout;
  context.setTimeout = fn => { timers.push(fn); return timers.length; };
  let polled = 0;
  inst = makeInst(null, { api: { getJob() { polled++; return Promise.resolve(running); } } });
  exporter = S.createExport(inst);
  exporter.apply(running, true);
  eq(timers.length, 1, 'a running job polls');
  exporter.dispose();
  await timers[0]();
  eq(polled, 0, 'a poll that was due asks for nothing');
  exporter.apply(running, true);
  eq(timers.length, 1, 'an answer after dispose starts no poll');
  context.setTimeout = realSetTimeout;
  console.log('storymaker export refusals ok');
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`

// The Silent switch of the inspector in Node: it writes and clears scene.silent with an immediate save, for storyboard and code
// scenes alike, follows the story and locks when read-only. The narration text stays, because silent only keeps it out of the film.
func TestStorymakerInspectorSilentInNode(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-inspector-silent.js")
	if err := os.WriteFile(script, []byte(storymakerInspectorSilentScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	out, err := exec.Command(node, script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker inspector silent ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerInspectorSilentScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const [dir] = process.argv.slice(2);

class FakeNode {
  constructor(tag) { this.tag = String(tag); this.children = []; this.parent = null; this.dataset = {}; this.attrs = {}; this.listeners = {}; this.own = ''; this.value = ''; this.hidden = false; this.disabled = false; this.readOnly = false; this.checked = false; this.className = ''; this.style = { setProperty() {} }; }
  appendChild(child) { child.parent = this; this.children.push(child); return child; }
  removeChild(child) { this.children = this.children.filter(item => item !== child); child.parent = null; return child; }
  replaceWith(next) { const list = this.parent.children; list[list.indexOf(this)] = next; next.parent = this.parent; }
  get firstChild() { return this.children[0] || null; }
  get textContent() { return this.own + this.children.map(child => child.textContent).join(''); }
  set textContent(text) { this.children = []; this.own = String(text); }
  setAttribute(name, value) { this.attrs[name] = String(value); if (name === 'value') this.value = String(value); if (name === 'disabled') this.disabled = true; }
  removeAttribute(name) { delete this.attrs[name]; }
  addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
  removeEventListener() {}
  fire(type) { (this.listeners[type] || []).forEach(fn => fn({ target: this })); }
  focus() {}
  contains() { return false; }
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
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-inspector.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const find = (node, stm) => { if (node.dataset.stm === stm) return node; for (const child of node.children) { const hit = find(child, stm); if (hit) return hit; } return null; };

const saves = [];
const inst = {
  t: key => key,
  bus: S.createBus(),
  domId: name => 'x-' + name,
  story: { scenes: [{ id: 's1', narration: 'Hello there.' }, { id: 'c1', kind: 'code', narration: 'A code scene.' }] },
  selected: 0,
  catalog: null,
  status: null,
  readonly: false,
  moveScene() {},
  deleteScene() {},
  openCode() {},
  editScene(id, fn, options) {
    const draft = JSON.parse(JSON.stringify(inst.story));
    fn(draft.scenes.find(scene => scene.id === id), draft);
    inst.story = draft;
    saves.push(!!(options && options.immediate));
    inst.bus.emit('story');
    return true;
  }
};
const panel = S.createInspector(inst);
const box = () => find(panel.el, 'scene-silent');

eq(box().checked, false, 'a normal scene is not silent');
box().checked = true;
box().fire('change');
eq(inst.story.scenes[0].silent, true, 'checking writes silent');
eq(inst.story.scenes[0].narration, 'Hello there.', 'the text stays');
eq(saves, [true], 'the save is immediate, so the audio answer and the blocker follow');
eq(box().checked, true, 'the switch stays on');
box().checked = false;
box().fire('change');
eq('silent' in inst.story.scenes[0], false, 'unchecking deletes the field');

// the story moves the switch
inst.story = JSON.parse(JSON.stringify(inst.story));
inst.story.scenes[0].silent = true;
inst.bus.emit('story');
eq(box().checked, true, 'the story moves the switch');

// a code scene has it too
inst.selected = 1;
inst.bus.emit('selection');
eq(box().checked, false, 'the code scene is not silent');
box().checked = true;
box().fire('change');
eq(inst.story.scenes[1].silent, true, 'a code scene can be silent');

// read-only locks it
inst.readonly = true;
inst.bus.emit('readonly');
eq(box().disabled, true, 'read-only disables the switch');
console.log('storymaker inspector silent ok');
`

const storymakerModelScript = `
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^(TestStorymakerExportRefusalsInNode|TestStorymakerInspectorSilentInNode)$' -count=1
```

Expected: FAIL. The Go check reports that `storymaker-export.js` still names `silent_note`, and Node ends with `Cannot read properties of null (reading 'dataset')`, because no `export-refused` element exists. `TestStorymakerInspectorSilentInNode` ends with `Cannot read properties of null (reading 'checked')`, because no `scene-silent` control exists.

- [ ] **Step 4: Implement**

Edit `ui/js/desktop/apps/storymaker-core.js`.

Old text:

```js
        else if (context === 'render' && (e.code === 'invalid_story' || e.code === 'code_error')) key = 'storymaker.job_error.' + e.code;
```

New text:

```js
        else if (context === 'render' && (e.code === 'invalid_story' || e.code === 'code_error' || e.code === 'narration_incomplete' || e.code === 'music_incomplete')) key = 'storymaker.job_error.' + e.code;
```

Edit `ui/js/desktop/apps/storymaker-export.js`.

Old text:

```js
    function dirOf(path) {
```

New text:

```js
    // A film is silent when the /narration answer holds no block, no music and no missing text and the report has no audio event.
    function isSilentFilm(audioInfo, report) {
        if (!audioInfo) return false;
        const narration = audioInfo.narration && typeof audioInfo.narration === 'object' ? audioInfo.narration : {};
        const spoken = Object.keys(narration).some(id => narration[id] && Array.isArray(narration[id].blocks) && narration[id].blocks.length > 0);
        const waiting = Array.isArray(audioInfo.missing) && audioInfo.missing.length > 0;
        const events = report && Array.isArray(report.audio_events) ? report.audio_events : [];
        return !spoken && !waiting && !audioInfo.music && events.length === 0;
    }

    function dirOf(path) {
```

Old text:

```js
        let copied = 0;
```

New text:

```js
        let copied = 0;
        let refused = null;   // the last gate answer of a start: {code, data}
        let disposed = false;   // a panel that is gone takes up no job and polls nothing
```

Old text:

```js
        function startView() {
```

New text:

```js
        function goPanel(name) {
            close(false);
            inst.showPanel(name);
        }

        // A refused start says what is missing and offers the panel that fixes it.
        function refusedView() {
            const data = refused.data && typeof refused.data === 'object' ? refused.data : {};
            let text = '';
            let action = null;
            if (refused.code === 'narration_incomplete') {
                const missing = Array.isArray(data.missing) ? data.missing : [];
                const scenes = Array.from(new Set(missing.map(item => String(item && item.scene_id || '')))).filter(Boolean).slice(0, 5).join(', ');
                const chars = data.chars && Number(data.chars.missing) > 0 ? Number(data.chars.missing) : 0;
                text = t('storymaker.export.missing_narration', { scenes, chars });
                action = el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-go-voice' }, onClick: () => goPanel('voice') }, icon('user'), el('span', { text: t('storymaker.export.go_voice') }));
            } else {
                text = data.music_issue === 'source_missing' ? t('storymaker.export.missing_music_source') : t('storymaker.export.missing_music');
                action = el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-go-music' }, onClick: () => goPanel('music') }, icon('sparkle'), el('span', { text: t('storymaker.export.go_music') }));
            }
            return el('div', { class: 'stm-job', dataset: { stm: 'export-refused', code: refused.code } },
                el('p', { class: 'stm-job-title stm-job-bad' }, icon('alert'), el('strong', { text: refused.code === 'narration_incomplete' ? t('storymaker.job_error.narration_incomplete') : t('storymaker.job_error.music_incomplete') })),
                el('p', { class: 'stm-hint', dataset: { stm: 'export-missing' }, text }),
                el('div', { class: 'stm-actions' }, action));
        }

        function startView() {
```

Old text:

```js
                if (seconds > 0) view.appendChild(el('p', { class: 'stm-hint', text: t('storymaker.export.silent_note') }));
```

New text:

```js
                if (seconds > 0 && isSilentFilm(inst.audioInfo, inst.report)) view.appendChild(el('p', { class: 'stm-hint', dataset: { stm: 'export-silent' }, text: t('storymaker.export.silent_film') }));
```

Old text:

```js
            view.appendChild(el('div', { class: 'stm-actions' },
```

New text:

```js
            if (refused) view.appendChild(refusedView());
            view.appendChild(el('div', { class: 'stm-actions' },
```

Old text:

```js
                const created = await inst.api.startRender(inst.projectId, inst.etag, S.uuid());
```

New text:

```js
                refused = null;
                const created = await inst.api.startRender(inst.projectId, inst.etag, S.uuid());
```

Old text:

```js
                if (err && err.status === 412) inst.bus.emit('conflict', err);
```

New text:

```js
                if (err && err.status === 412) inst.bus.emit('conflict', err);
                if (err && (err.code === 'narration_incomplete' || err.code === 'music_incomplete')) refused = { code: err.code, data: err.data || {} };
```

Old text:

```js
            inst.bus.on('report', () => { if (open) paint(); }),
```

New text:

```js
            inst.bus.on('report', () => { if (open) paint(); }),
            inst.bus.on('audio', () => { refused = null; if (open) paint(); }),
```

Old text:

```js
        function paint() {
```

New text:

```js
        function paint() {
            if (disposed) return;
```

A project switch disposes the panel while POST /render may still be in flight. Its answer must not start a poll that runs for the whole render, repaints detached DOM and notifies on the project list.

Old text:

```js
            if (!isActive()) return;
```

New text:

```js
            if (disposed || !isActive()) return;
```

Old text:

```js
                if (!job) return;
```

New text:

```js
                if (disposed || !job) return;
```

Old text:

```js
            if (!next || typeof next !== 'object' || typeof next.id !== 'string') return;
```

New text:

```js
            if (disposed || !next || typeof next !== 'object' || typeof next.id !== 'string') return;
```

Old text:

```js
                document.removeEventListener('pointerdown', onDocumentPointer, true);
```

New text:

```js
                disposed = true;
                document.removeEventListener('pointerdown', onDocumentPointer, true);
```

Old text:

```js
    S.createExport = createExport;
```

New text:

```js
    S.createExport = createExport;
    S.isSilentFilm = isSilentFilm;
```

The gate lets a silent scene pass (spec line 504), so the inspector offers that switch for storyboard and code scenes alike. Edit `ui/js/desktop/apps/storymaker-inspector.js`.

Old text:

```js
            body.appendChild(field('storymaker.inspector.narration', inst.domId('narration'), controls.narration, t('storymaker.inspector.narration_hint')));
```

New text:

```js
            body.appendChild(field('storymaker.inspector.narration', inst.domId('narration'), controls.narration, t('storymaker.inspector.narration_hint')));
            const silent = el('input', { type: 'checkbox', id: inst.domId('silent'), dataset: { stm: 'scene-silent' } });
            silent.checked = scene.silent === true;
            silent.addEventListener('change', () => {
                const on = silent.checked;
                inst.editScene(scene.id, draft => { if (on) draft.silent = true; else delete draft.silent; }, { immediate: true });
            });
            controls.silent = silent;
            body.appendChild(el('div', { class: 'stm-field' }, el('label', { class: 'stm-voice-check', for: inst.domId('silent') }, silent, el('span', { text: t('storymaker.inspector.silent') }))));
```

Old text:

```js
            ['environment', 'variant', 'narration'].forEach(name => { if (controls[name]) controls[name].disabled = inst.readonly; });
```

New text:

```js
            ['environment', 'variant', 'narration', 'silent'].forEach(name => { if (controls[name]) controls[name].disabled = inst.readonly; });
```

Old text:

```js
            set(controls.narration, scene.narration || '');
```

New text:

```js
            set(controls.narration, scene.narration || '');
            // set compares .value, and a checkbox holds its state in .checked.
            if (controls.silent) controls.silent.checked = scene.silent === true;
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^(TestStorymakerExportRefusalsInNode|TestStorymakerInspectorSilentInNode|TestStorymakerAppApiAndAssetsInNode|TestStorymakerSourcesAreSafeAndSmall)$' -count=1
```

Expected: PASS. The i18n guards stay red on the new export keys until E2e.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-export.js ui/js/desktop/apps/storymaker-core.js ui/js/desktop/apps/storymaker-inspector.js ui/desktop_storymaker_model_test.go
git commit -m "feat(storymaker): export refusals with actions, silent film note" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E2e: 2C strings
**Batch:** 2C  **Depends on:** E2a, E2b, E2c, E2d
**Files:**
Modify: `ui/js/desktop/apps/storymaker-core.js`
Modify: `ui/lang/storymaker/cs.json`
Modify: `ui/lang/storymaker/da.json`
Modify: `ui/lang/storymaker/de.json`
Modify: `ui/lang/storymaker/el.json`
Modify: `ui/lang/storymaker/en.json`
Modify: `ui/lang/storymaker/es.json`
Modify: `ui/lang/storymaker/fr.json`
Modify: `ui/lang/storymaker/hi.json`
Modify: `ui/lang/storymaker/it.json`
Modify: `ui/lang/storymaker/ja.json`
Modify: `ui/lang/storymaker/nl.json`
Modify: `ui/lang/storymaker/no.json`
Modify: `ui/lang/storymaker/pl.json`
Modify: `ui/lang/storymaker/pt.json`
Modify: `ui/lang/storymaker/sv.json`
Modify: `ui/lang/storymaker/zh.json`
Test: `ui/desktop_storymaker_i18n_test.go`

- [ ] **Step 1: Impact**

Run this from the main checkout. Warn on HIGH or CRITICAL and confirm UNKNOWN with a text search, per outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "describeJobError" --direction upstream --repo .
```

- [ ] **Step 2: Write the failing test**

The guards exist already. Two edits make them demand the new keys. The second exempts `Volume` and `Lo-fi`, which are correct as they stand in several languages.

Edit `ui/js/desktop/apps/storymaker-core.js`, in the text E1i leaves.

Old text:

```js
'budget_exceeded'];
```

New text:

```js
'budget_exceeded', 'narration_incomplete', 'music_failed', 'music_incomplete'];
```

Edit `ui/desktop_storymaker_i18n_test.go`.

Old text:

```go
"storymaker.title": true, "storymaker.tab_stage": true, "storymaker.strip.duration_s": true, "storymaker.export.cap_ffmpeg": true,
```

New text:

```go
"storymaker.music.volume": true, "storymaker.music.mood_lo_fi": true,
		"storymaker.title": true, "storymaker.tab_stage": true, "storymaker.strip.duration_s": true, "storymaker.export.cap_ffmpeg": true,
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^(TestStorymakerAppUsesOnlyKeysItHas|TestStorymakerLocalesCoverAllSixteenLanguages|TestStorymakerAppTranslationsAreTranslated)$' -count=1
```

Expected: FAIL. `TestStorymakerAppUsesOnlyKeysItHas` names the `storymaker.music.*` keys, `storymaker.side_music`, the two `storymaker.error.*` sentences, the new `storymaker.export.*` keys, `storymaker.inspector.silent` and `lang/storymaker/en.json lacks storymaker.job_error.narration_incomplete`.

- [ ] **Step 4: Implement**

Write this script with the Write tool as `<scratchpad>/e2e-strings.mjs`, where `<scratchpad>` is this session's scratchpad folder. Then run it from the worktree root with `node <scratchpad>/e2e-strings.mjs`.

```js
// Adds the 2C strings to the 16 Storymaker locale files and removes the silent note. Run from the worktree root.
import fs from 'node:fs';
import path from 'node:path';

const KEYS = [
  'music.cancel',
  'side_music',
  'music.mode', 'music.mode_none', 'music.mode_generated', 'music.mode_file',
  'music.mood', 'music.mood_none',
  'music.mood_upbeat_corporate', 'music.mood_calm_ambient', 'music.mood_inspiring', 'music.mood_playful',
  'music.mood_tech_pulse', 'music.mood_dramatic', 'music.mood_lo_fi', 'music.mood_acoustic',
  'music.prompt', 'music.prompt_placeholder', 'music.generate', 'music.generate_summary', 'music.not_configured',
  'music.choose_file', 'music.choose_other', 'music.file_filter', 'music.track', 'music.track_none', 'music.volume',
  'music.running', 'music.ready', 'music.source_missing', 'music.interrupted', 'music.cancelled',
  'error.invalid_music_file', 'error.music_unavailable',
  'job_error.narration_incomplete', 'job_error.music_failed', 'job_error.music_incomplete',
  'export.silent_film', 'export.missing_narration', 'export.missing_music', 'export.missing_music_source',
  'export.go_voice', 'export.go_music'
];

const TEXT = {
  en: [
    "Stop",
    "Music",
    "Music source", "No music", "Generated by AI", "File or track",
    "Mood", "No preset",
    "Upbeat corporate", "Calm ambient", "Inspiring", "Playful",
    "Tech pulse", "Dramatic", "Lo-fi", "Acoustic",
    "What it should sound like", "Soft piano, slow and warm", "Generate music", "Mood: {mood}. Description: {prompt}.",
    "Music generation is not set up on this server. Choose a file or a track.",
    "Choose a file", "Choose another file", "Audio files (WAV, MP3)", "Noisemaker track", "No track", "Volume",
    "Preparing the music", "The music is ready ({seconds} s).", "The music source is gone: {source}",
    "The music job was interrupted by a restart.", "The music job was cancelled.",
    "This file cannot be used as music. Use a WAV or MP3 file of at most 1 GiB.",
    "Music generation is not set up on this server.",
    "The narration audio is not complete. Generate it first.", "The music could not be prepared.", "The music is not ready yet.",
    "This film has no narration and no music, so the video stays silent.",
    "Narration audio is missing in {scenes} ({chars} characters).",
    "The music file has not been prepared yet.", "The music source is gone. Choose another file.",
    "Open the voice panel", "Open the music panel"
  ],
  de: [
    "Stopp",
    "Musik",
    "Musikquelle", "Keine Musik", "Von KI erzeugt", "Datei oder Titel",
    "Stimmung", "Keine Vorgabe",
    "Schwungvoll, geschäftlich", "Ruhig, flächig", "Inspirierend", "Verspielt",
    "Technik-Puls", "Dramatisch", "Lo-fi", "Akustisch",
    "So soll es klingen", "Weiches Klavier, langsam und warm", "Musik erzeugen", "Stimmung: {mood}. Beschreibung: {prompt}.",
    "Die Musikerzeugung ist auf diesem Server nicht eingerichtet. Wähle eine Datei oder einen Titel.",
    "Datei wählen", "Andere Datei wählen", "Audiodateien (WAV, MP3)", "Noisemaker-Titel", "Kein Titel", "Lautstärke",
    "Musik wird vorbereitet", "Die Musik ist bereit ({seconds} s).", "Die Musikquelle fehlt: {source}",
    "Der Musikauftrag wurde durch einen Neustart unterbrochen.", "Der Musikauftrag wurde abgebrochen.",
    "Diese Datei kann nicht als Musik verwendet werden. Nimm eine WAV- oder MP3-Datei mit höchstens 1 GiB.",
    "Die Musikerzeugung ist auf diesem Server nicht eingerichtet.",
    "Der Erzählton ist nicht vollständig. Erzeuge ihn zuerst.", "Die Musik konnte nicht vorbereitet werden.", "Die Musik ist noch nicht bereit.",
    "Dieser Film hat keine Erzählung und keine Musik, das Video bleibt also stumm.",
    "Der Erzählton fehlt in {scenes} ({chars} Zeichen).",
    "Die Musikdatei ist noch nicht vorbereitet.", "Die Musikquelle fehlt. Wähle eine andere Datei.",
    "Stimmen-Bereich öffnen", "Musik-Bereich öffnen"
  ],
  fr: [
    "Arrêter",
    "Musique",
    "Source de la musique", "Pas de musique", "Générée par l'IA", "Fichier ou morceau",
    "Ambiance", "Aucun préréglage",
    "Entreprise entraînant", "Ambiance calme", "Inspirant", "Ludique",
    "Pulsation tech", "Dramatique", "Lo-fi", "Acoustique",
    "Le son souhaité", "Piano doux, lent et chaleureux", "Générer la musique", "Ambiance : {mood}. Description : {prompt}.",
    "La génération de musique n'est pas configurée sur ce serveur. Choisissez un fichier ou un morceau.",
    "Choisir un fichier", "Choisir un autre fichier", "Fichiers audio (WAV, MP3)", "Morceau Noisemaker", "Aucun morceau", "Volume",
    "Préparation de la musique", "La musique est prête ({seconds} s).", "La source de la musique a disparu : {source}",
    "La tâche musicale a été interrompue par un redémarrage.", "La tâche musicale a été annulée.",
    "Ce fichier ne peut pas servir de musique. Utilisez un fichier WAV ou MP3 de 1 Gio au maximum.",
    "La génération de musique n'est pas configurée sur ce serveur.",
    "L'audio de la narration est incomplet. Générez-le d'abord.", "La musique n'a pas pu être préparée.", "La musique n'est pas encore prête.",
    "Ce film n'a ni narration ni musique : la vidéo reste muette.",
    "L'audio de la narration manque dans {scenes} ({chars} caractères).",
    "Le fichier de musique n'a pas encore été préparé.", "La source de la musique a disparu. Choisissez un autre fichier.",
    "Ouvrir le panneau des voix", "Ouvrir le panneau de la musique"
  ],
  es: [
    "Detener",
    "Música",
    "Fuente de la música", "Sin música", "Generada por IA", "Archivo o pista",
    "Ambiente", "Sin ajuste previo",
    "Corporativo animado", "Ambiente tranquilo", "Inspirador", "Juguetón",
    "Pulso tecnológico", "Dramático", "Lo-fi", "Acústico",
    "Cómo debería sonar", "Piano suave, lento y cálido", "Generar música", "Ambiente: {mood}. Descripción: {prompt}.",
    "La generación de música no está configurada en este servidor. Elige un archivo o una pista.",
    "Elegir un archivo", "Elegir otro archivo", "Archivos de audio (WAV, MP3)", "Pista de Noisemaker", "Sin pista", "Volumen",
    "Preparando la música", "La música está lista ({seconds} s).", "Falta la fuente de la música: {source}",
    "Un reinicio interrumpió el trabajo de música.", "El trabajo de música se canceló.",
    "Este archivo no se puede usar como música. Usa un archivo WAV o MP3 de 1 GiB como máximo.",
    "La generación de música no está configurada en este servidor.",
    "El audio de la narración está incompleto. Genéralo primero.", "No se pudo preparar la música.", "La música aún no está lista.",
    "Esta película no tiene narración ni música, así que el vídeo queda en silencio.",
    "Falta el audio de la narración en {scenes} ({chars} caracteres).",
    "El archivo de música aún no se ha preparado.", "Falta la fuente de la música. Elige otro archivo.",
    "Abrir el panel de voz", "Abrir el panel de música"
  ],
  it: [
    "Ferma",
    "Musica",
    "Fonte della musica", "Nessuna musica", "Generata dall'IA", "File o traccia",
    "Atmosfera", "Nessuna preimpostazione",
    "Aziendale vivace", "Ambient tranquillo", "Ispirante", "Giocoso",
    "Impulso tech", "Drammatico", "Lo-fi", "Acustico",
    "Come dovrebbe suonare", "Pianoforte morbido, lento e caldo", "Genera la musica", "Atmosfera: {mood}. Descrizione: {prompt}.",
    "La generazione di musica non è configurata su questo server. Scegli un file o una traccia.",
    "Scegli un file", "Scegli un altro file", "File audio (WAV, MP3)", "Traccia Noisemaker", "Nessuna traccia", "Volume",
    "Preparazione della musica", "La musica è pronta ({seconds} s).", "La fonte della musica non c'è più: {source}",
    "Il processo musicale è stato interrotto da un riavvio.", "Il processo musicale è stato annullato.",
    "Questo file non può essere usato come musica. Usa un file WAV o MP3 di al massimo 1 GiB.",
    "La generazione di musica non è configurata su questo server.",
    "L'audio della narrazione è incompleto. Generalo prima.", "Non è stato possibile preparare la musica.", "La musica non è ancora pronta.",
    "Questo film non ha narrazione né musica, quindi il video resta muto.",
    "Manca l'audio della narrazione in {scenes} ({chars} caratteri).",
    "Il file musicale non è ancora stato preparato.", "La fonte della musica non c'è più. Scegli un altro file.",
    "Apri il pannello voce", "Apri il pannello musica"
  ],
  nl: [
    "Stoppen",
    "Muziek",
    "Muziekbron", "Geen muziek", "Gemaakt door AI", "Bestand of nummer",
    "Sfeer", "Geen voorinstelling",
    "Vlot zakelijk", "Rustig en ruimtelijk", "Inspirerend", "Speels",
    "Tech-puls", "Dramatisch", "Lo-fi", "Akoestisch",
    "Zo moet het klinken", "Zachte piano, langzaam en warm", "Muziek maken", "Sfeer: {mood}. Omschrijving: {prompt}.",
    "Muziek maken is op deze server niet ingesteld. Kies een bestand of een nummer.",
    "Kies een bestand", "Kies een ander bestand", "Audiobestanden (WAV, MP3)", "Noisemaker-nummer", "Geen nummer", "Volume",
    "Muziek wordt voorbereid", "De muziek is klaar ({seconds} s).", "De muziekbron ontbreekt: {source}",
    "De muziektaak is door een herstart onderbroken.", "De muziektaak is geannuleerd.",
    "Dit bestand kan niet als muziek worden gebruikt. Gebruik een WAV- of MP3-bestand van maximaal 1 GiB.",
    "Muziek maken is op deze server niet ingesteld.",
    "De audio van de vertelling is niet compleet. Maak die eerst.", "De muziek kon niet worden voorbereid.", "De muziek is nog niet klaar.",
    "Deze film heeft geen vertelling en geen muziek, dus de video blijft stil.",
    "De audio van de vertelling ontbreekt in {scenes} ({chars} tekens).",
    "Het muziekbestand is nog niet voorbereid.", "De muziekbron ontbreekt. Kies een ander bestand.",
    "Stempaneel openen", "Muziekpaneel openen"
  ],
  sv: [
    "Stoppa",
    "Musik",
    "Musikkälla", "Ingen musik", "Skapad av AI", "Fil eller spår",
    "Stämning", "Ingen förinställning",
    "Pigg företagsmusik", "Lugn ambient", "Inspirerande", "Lekfull",
    "Tech-puls", "Dramatisk", "Lo-fi", "Akustisk",
    "Så här ska det låta", "Mjukt piano, långsamt och varmt", "Skapa musik", "Stämning: {mood}. Beskrivning: {prompt}.",
    "Musikgenerering är inte inställd på den här servern. Välj en fil eller ett spår.",
    "Välj en fil", "Välj en annan fil", "Ljudfiler (WAV, MP3)", "Noisemaker-spår", "Inget spår", "Volym",
    "Musiken förbereds", "Musiken är klar ({seconds} s).", "Musikkällan saknas: {source}",
    "Musikjobbet avbröts av en omstart.", "Musikjobbet avbröts.",
    "Den här filen kan inte användas som musik. Använd en WAV- eller MP3-fil på högst 1 GiB.",
    "Musikgenerering är inte inställd på den här servern.",
    "Berättarljudet är inte komplett. Skapa det först.", "Musiken kunde inte förberedas.", "Musiken är inte klar än.",
    "Den här filmen har varken berättarröst eller musik, så videon blir tyst.",
    "Berättarljud saknas i {scenes} ({chars} tecken).",
    "Musikfilen har inte förberetts än.", "Musikkällan saknas. Välj en annan fil.",
    "Öppna röstpanelen", "Öppna musikpanelen"
  ],
  da: [
    "Stands",
    "Musik",
    "Musikkilde", "Ingen musik", "Lavet af AI", "Fil eller nummer",
    "Stemning", "Ingen forudindstilling",
    "Frisk virksomhedsmusik", "Rolig ambient", "Inspirerende", "Legende",
    "Tech-puls", "Dramatisk", "Lo-fi", "Akustisk",
    "Sådan skal det lyde", "Blødt klaver, langsomt og varmt", "Lav musik", "Stemning: {mood}. Beskrivelse: {prompt}.",
    "Musikgenerering er ikke sat op på denne server. Vælg en fil eller et nummer.",
    "Vælg en fil", "Vælg en anden fil", "Lydfiler (WAV, MP3)", "Noisemaker-nummer", "Intet nummer", "Lydstyrke",
    "Musikken forberedes", "Musikken er klar ({seconds} s).", "Musikkilden mangler: {source}",
    "Musikjobbet blev afbrudt af en genstart.", "Musikjobbet blev annulleret.",
    "Denne fil kan ikke bruges som musik. Brug en WAV- eller MP3-fil på højst 1 GiB.",
    "Musikgenerering er ikke sat op på denne server.",
    "Fortællerlyden er ikke komplet. Lav den først.", "Musikken kunne ikke forberedes.", "Musikken er ikke klar endnu.",
    "Denne film har hverken fortælling eller musik, så videoen forbliver tavs.",
    "Fortællerlyd mangler i {scenes} ({chars} tegn).",
    "Musikfilen er ikke forberedt endnu.", "Musikkilden mangler. Vælg en anden fil.",
    "Åbn stemmepanelet", "Åbn musikpanelet"
  ],
  no: [
    "Stopp",
    "Musikk",
    "Musikkkilde", "Ingen musikk", "Laget av KI", "Fil eller spor",
    "Stemning", "Ingen forhåndsinnstilling",
    "Frisk bedriftsmusikk", "Rolig ambient", "Inspirerende", "Lekende",
    "Tech-puls", "Dramatisk", "Lo-fi", "Akustisk",
    "Slik skal det høres ut", "Mykt piano, sakte og varmt", "Lag musikk", "Stemning: {mood}. Beskrivelse: {prompt}.",
    "Musikkgenerering er ikke satt opp på denne serveren. Velg en fil eller et spor.",
    "Velg en fil", "Velg en annen fil", "Lydfiler (WAV, MP3)", "Noisemaker-spor", "Ingen spor", "Lydstyrke",
    "Musikken forberedes", "Musikken er klar ({seconds} s).", "Musikkkilden mangler: {source}",
    "Musikkjobben ble avbrutt av en omstart.", "Musikkjobben ble avbrutt.",
    "Denne filen kan ikke brukes som musikk. Bruk en WAV- eller MP3-fil på høyst 1 GiB.",
    "Musikkgenerering er ikke satt opp på denne serveren.",
    "Fortellerlyden er ikke fullstendig. Lag den først.", "Musikken kunne ikke forberedes.", "Musikken er ikke klar ennå.",
    "Denne filmen har verken fortelling eller musikk, så videoen forblir stille.",
    "Fortellerlyd mangler i {scenes} ({chars} tegn).",
    "Musikkfilen er ikke forberedt ennå.", "Musikkkilden mangler. Velg en annen fil.",
    "Åpne stemmepanelet", "Åpne musikkpanelet"
  ],
  pl: [
    "Zatrzymaj",
    "Muzyka",
    "Źródło muzyki", "Bez muzyki", "Wygenerowana przez AI", "Plik lub utwór",
    "Nastrój", "Bez ustawienia",
    "Energiczny firmowy", "Spokojny ambient", "Inspirujący", "Figlarny",
    "Puls technologii", "Dramatyczny", "Lo-fi", "Akustyczny",
    "Jak ma to brzmieć", "Miękki fortepian, wolno i ciepło", "Wygeneruj muzykę", "Nastrój: {mood}. Opis: {prompt}.",
    "Generowanie muzyki nie jest skonfigurowane na tym serwerze. Wybierz plik lub utwór.",
    "Wybierz plik", "Wybierz inny plik", "Pliki audio (WAV, MP3)", "Utwór Noisemaker", "Brak utworu", "Głośność",
    "Przygotowywanie muzyki", "Muzyka jest gotowa ({seconds} s).", "Brak źródła muzyki: {source}",
    "Zadanie muzyczne przerwał restart.", "Zadanie muzyczne anulowano.",
    "Tego pliku nie można użyć jako muzyki. Użyj pliku WAV lub MP3 o rozmiarze do 1 GiB.",
    "Generowanie muzyki nie jest skonfigurowane na tym serwerze.",
    "Dźwięk narracji jest niekompletny. Najpierw go wygeneruj.", "Nie udało się przygotować muzyki.", "Muzyka nie jest jeszcze gotowa.",
    "Ten film nie ma narracji ani muzyki, więc wideo pozostaje ciche.",
    "Brak dźwięku narracji w {scenes} (znaki: {chars}).",
    "Plik muzyczny nie został jeszcze przygotowany.", "Brak źródła muzyki. Wybierz inny plik.",
    "Otwórz panel głosu", "Otwórz panel muzyki"
  ],
  pt: [
    "Parar",
    "Música",
    "Fonte da música", "Sem música", "Gerada por IA", "Ficheiro ou faixa",
    "Ambiente", "Sem predefinição",
    "Corporativo animado", "Ambiente calmo", "Inspirador", "Brincalhão",
    "Pulso tecnológico", "Dramático", "Lo-fi", "Acústico",
    "Como deve soar", "Piano suave, lento e quente", "Gerar música", "Ambiente: {mood}. Descrição: {prompt}.",
    "A geração de música não está configurada neste servidor. Escolhe um ficheiro ou uma faixa.",
    "Escolher um ficheiro", "Escolher outro ficheiro", "Ficheiros de áudio (WAV, MP3)", "Faixa do Noisemaker", "Sem faixa", "Volume",
    "A preparar a música", "A música está pronta ({seconds} s).", "A fonte da música desapareceu: {source}",
    "A tarefa de música foi interrompida por um reinício.", "A tarefa de música foi cancelada.",
    "Este ficheiro não pode ser usado como música. Usa um ficheiro WAV ou MP3 de 1 GiB no máximo.",
    "A geração de música não está configurada neste servidor.",
    "O áudio da narração está incompleto. Gera-o primeiro.", "Não foi possível preparar a música.", "A música ainda não está pronta.",
    "Este filme não tem narração nem música, por isso o vídeo fica em silêncio.",
    "Falta o áudio da narração em {scenes} ({chars} caracteres).",
    "O ficheiro de música ainda não foi preparado.", "A fonte da música desapareceu. Escolhe outro ficheiro.",
    "Abrir o painel de voz", "Abrir o painel de música"
  ],
  cs: [
    "Zastavit",
    "Hudba",
    "Zdroj hudby", "Bez hudby", "Vytvořeno umělou inteligencí", "Soubor nebo skladba",
    "Nálada", "Bez předvolby",
    "Svižná firemní", "Klidný ambient", "Inspirující", "Hravá",
    "Technologický puls", "Dramatická", "Lo-fi", "Akustická",
    "Jak to má znít", "Jemný klavír, pomalý a hřejivý", "Vytvořit hudbu", "Nálada: {mood}. Popis: {prompt}.",
    "Tvorba hudby není na tomto serveru nastavena. Vyberte soubor nebo skladbu.",
    "Vybrat soubor", "Vybrat jiný soubor", "Zvukové soubory (WAV, MP3)", "Skladba z Noisemakeru", "Žádná skladba", "Hlasitost",
    "Příprava hudby", "Hudba je připravena ({seconds} s).", "Zdroj hudby chybí: {source}",
    "Hudební úlohu přerušil restart.", "Hudební úloha byla zrušena.",
    "Tento soubor nelze použít jako hudbu. Použijte soubor WAV nebo MP3 o velikosti nejvýše 1 GiB.",
    "Tvorba hudby není na tomto serveru nastavena.",
    "Zvuk vyprávění není úplný. Nejprve ho vytvořte.", "Hudbu se nepodařilo připravit.", "Hudba ještě není připravena.",
    "Tento film nemá vyprávění ani hudbu, takže video zůstane němé.",
    "Zvuk vyprávění chybí v {scenes} (znaků: {chars}).",
    "Hudební soubor ještě není připraven.", "Zdroj hudby chybí. Vyberte jiný soubor.",
    "Otevřít panel hlasu", "Otevřít panel hudby"
  ],
  el: [
    "Διακοπή",
    "Μουσική",
    "Πηγή μουσικής", "Χωρίς μουσική", "Δημιουργία με ΤΝ", "Αρχείο ή κομμάτι",
    "Διάθεση", "Χωρίς προεπιλογή",
    "Ζωηρή εταιρική", "Ήρεμη ατμοσφαιρική", "Εμπνευστική", "Παιχνιδιάρικη",
    "Τεχνολογικός παλμός", "Δραματική", "Lo-fi", "Ακουστική",
    "Πώς πρέπει να ακούγεται", "Απαλό πιάνο, αργό και ζεστό", "Δημιουργία μουσικής", "Διάθεση: {mood}. Περιγραφή: {prompt}.",
    "Η δημιουργία μουσικής δεν έχει ρυθμιστεί σε αυτόν τον διακομιστή. Επιλέξτε αρχείο ή κομμάτι.",
    "Επιλογή αρχείου", "Επιλογή άλλου αρχείου", "Αρχεία ήχου (WAV, MP3)", "Κομμάτι Noisemaker", "Κανένα κομμάτι", "Ένταση",
    "Προετοιμασία μουσικής", "Η μουσική είναι έτοιμη ({seconds} s).", "Η πηγή της μουσικής λείπει: {source}",
    "Η εργασία μουσικής διακόπηκε από επανεκκίνηση.", "Η εργασία μουσικής ακυρώθηκε.",
    "Αυτό το αρχείο δεν μπορεί να χρησιμοποιηθεί ως μουσική. Χρησιμοποιήστε αρχείο WAV ή MP3 έως 1 GiB.",
    "Η δημιουργία μουσικής δεν έχει ρυθμιστεί σε αυτόν τον διακομιστή.",
    "Ο ήχος της αφήγησης δεν είναι πλήρης. Δημιουργήστε τον πρώτα.", "Η μουσική δεν μπόρεσε να προετοιμαστεί.", "Η μουσική δεν είναι ακόμη έτοιμη.",
    "Αυτή η ταινία δεν έχει αφήγηση ούτε μουσική, οπότε το βίντεο μένει βουβό.",
    "Λείπει ο ήχος της αφήγησης στο {scenes} ({chars} χαρακτήρες).",
    "Το αρχείο μουσικής δεν έχει προετοιμαστεί ακόμη.", "Η πηγή της μουσικής λείπει. Επιλέξτε άλλο αρχείο.",
    "Άνοιγμα πίνακα φωνής", "Άνοιγμα πίνακα μουσικής"
  ],
  hi: [
    "रोकें",
    "संगीत",
    "संगीत का स्रोत", "कोई संगीत नहीं", "AI से बना", "फ़ाइल या ट्रैक",
    "मूड", "कोई प्रीसेट नहीं",
    "जोशीला कॉर्पोरेट", "शांत एंबिएंट", "प्रेरक", "चंचल",
    "टेक पल्स", "नाटकीय", "Lo-fi", "अकूस्टिक",
    "यह कैसा सुनाई दे", "नरम पियानो, धीमा और गर्म", "संगीत बनाएँ", "मूड: {mood}. विवरण: {prompt}.",
    "इस सर्वर पर संगीत बनाना सेट नहीं है। कोई फ़ाइल या ट्रैक चुनें।",
    "फ़ाइल चुनें", "दूसरी फ़ाइल चुनें", "ऑडियो फ़ाइलें (WAV, MP3)", "Noisemaker ट्रैक", "कोई ट्रैक नहीं", "आवाज़ का स्तर",
    "संगीत तैयार हो रहा है", "संगीत तैयार है ({seconds} s)।", "संगीत का स्रोत नहीं मिला: {source}",
    "पुनरारंभ के कारण संगीत का काम रुक गया।", "संगीत का काम रद्द कर दिया गया।",
    "इस फ़ाइल को संगीत के रूप में उपयोग नहीं किया जा सकता। अधिकतम 1 GiB की WAV या MP3 फ़ाइल का उपयोग करें।",
    "इस सर्वर पर संगीत बनाना सेट नहीं है।",
    "वर्णन का ऑडियो पूरा नहीं है। पहले उसे बनाएँ।", "संगीत तैयार नहीं हो सका।", "संगीत अभी तैयार नहीं है।",
    "इस फ़िल्म में न वर्णन है न संगीत, इसलिए वीडियो बिना आवाज़ के रहेगा।",
    "{scenes} में वर्णन का ऑडियो नहीं है ({chars} अक्षर)।",
    "संगीत फ़ाइल अभी तैयार नहीं है।", "संगीत का स्रोत नहीं मिला। दूसरी फ़ाइल चुनें।",
    "आवाज़ पैनल खोलें", "संगीत पैनल खोलें"
  ],
  ja: [
    "停止",
    "音楽",
    "音楽のソース", "音楽なし", "AI で生成", "ファイルまたはトラック",
    "雰囲気", "プリセットなし",
    "明るいコーポレート", "穏やかなアンビエント", "感動的", "遊び心のある",
    "テックパルス", "ドラマチック", "Lo-fi", "アコースティック",
    "どんな音にするか", "柔らかいピアノ、ゆっくりで温かく", "音楽を生成", "雰囲気：{mood}。説明：{prompt}。",
    "このサーバーでは音楽生成が設定されていません。ファイルまたはトラックを選んでください。",
    "ファイルを選択", "別のファイルを選択", "音声ファイル（WAV、MP3）", "Noisemaker のトラック", "トラックなし", "音量",
    "音楽を準備しています", "音楽の準備ができました（{seconds} 秒）。", "音楽のソースがありません：{source}",
    "再起動により音楽ジョブが中断されました。", "音楽ジョブはキャンセルされました。",
    "このファイルは音楽として使えません。1 GiB 以下の WAV または MP3 ファイルを使ってください。",
    "このサーバーでは音楽生成が設定されていません。",
    "ナレーション音声が不完全です。先に生成してください。", "音楽を準備できませんでした。", "音楽はまだ準備できていません。",
    "この作品にはナレーションも音楽もないため、動画は無音のままです。",
    "{scenes} のナレーション音声がありません（{chars} 文字）。",
    "音楽ファイルはまだ準備されていません。", "音楽のソースがありません。別のファイルを選んでください。",
    "音声パネルを開く", "音楽パネルを開く"
  ],
  zh: [
    "停止",
    "音乐",
    "音乐来源", "无音乐", "由 AI 生成", "文件或曲目",
    "氛围", "无预设",
    "轻快商务", "平静氛围", "鼓舞人心", "活泼俏皮",
    "科技脉动", "戏剧性", "Lo-fi", "原声",
    "希望的听感", "柔和的钢琴，缓慢而温暖", "生成音乐", "氛围：{mood}。描述：{prompt}。",
    "此服务器未设置音乐生成。请选择文件或曲目。",
    "选择文件", "选择其他文件", "音频文件（WAV、MP3）", "Noisemaker 曲目", "无曲目", "音量",
    "正在准备音乐", "音乐已就绪（{seconds} 秒）。", "音乐来源已丢失：{source}",
    "音乐任务因重启而中断。", "音乐任务已取消。",
    "此文件不能用作音乐。请使用不超过 1 GiB 的 WAV 或 MP3 文件。",
    "此服务器未设置音乐生成。",
    "旁白音频不完整。请先生成。", "无法准备音乐。", "音乐尚未就绪。",
    "这部影片没有旁白和音乐，因此视频保持无声。",
    "{scenes} 缺少旁白音频（{chars} 个字符）。",
    "音乐文件尚未准备好。", "音乐来源已丢失。请选择其他文件。",
    "打开语音面板", "打开音乐面板"
  ]
};

// Three more keys, with one text per language.
const EXTRA = {
  'music.budget_exceeded': {
    en: "The daily budget is used up, so no music can be generated.", de: "Das Tagesbudget ist aufgebraucht, daher kann keine Musik erzeugt werden.", fr: "Le budget quotidien est épuisé : aucune musique ne peut être générée.", es: "El presupuesto diario se ha agotado, así que no se puede generar música.",
    it: "Il budget giornaliero è esaurito, quindi non è possibile generare musica.", nl: "Het dagbudget is op, dus er kan geen muziek worden gemaakt.", sv: "Dagsbudgeten är slut, så ingen musik kan skapas.", da: "Dagsbudgettet er brugt op, så der kan ikke laves musik.",
    no: "Dagsbudsjettet er brukt opp, så det kan ikke lages musikk.", pl: "Dzienny budżet został wyczerpany, więc nie można wygenerować muzyki.", pt: "O orçamento diário esgotou-se, por isso não é possível gerar música.", cs: "Denní rozpočet je vyčerpán, takže hudbu nelze vytvořit.",
    el: "Ο ημερήσιος προϋπολογισμός εξαντλήθηκε, οπότε δεν μπορεί να δημιουργηθεί μουσική.", hi: "दैनिक बजट समाप्त हो गया है, इसलिए संगीत नहीं बनाया जा सकता।", ja: "1 日の予算を使い切ったため、音楽を生成できません。", zh: "每日预算已用完，因此无法生成音乐。"
  },
  'music.preview_level_note': {
    en: "The preview's music level is approximate; the export sets the final balance.", de: "Die Musiklautstärke der Vorschau ist nur ungefähr; der Export legt die endgültige Balance fest.", fr: "Le niveau de la musique dans l'aperçu est approximatif ; l'export fixe l'équilibre final.", es: "El nivel de la música en la vista previa es aproximado; la exportación fija el equilibrio final.",
    it: "Il livello della musica nell'anteprima è approssimativo; l'esportazione stabilisce il bilanciamento finale.", nl: "Het muziekniveau in het voorbeeld is bij benadering; de export bepaalt de uiteindelijke balans.", sv: "Musiknivån i förhandsvisningen är ungefärlig; exporten bestämmer den slutliga balansen.", da: "Musikniveauet i forhåndsvisningen er omtrentligt; eksporten fastlægger den endelige balance.",
    no: "Musikknivået i forhåndsvisningen er omtrentlig; eksporten fastsetter den endelige balansen.", pl: "Poziom muzyki w podglądzie jest przybliżony; ostateczne proporcje ustala eksport.", pt: "O nível da música na pré-visualização é aproximado; a exportação define o equilíbrio final.", cs: "Hlasitost hudby v náhledu je přibližná; konečné vyvážení určí export.",
    el: "Η στάθμη της μουσικής στην προεπισκόπηση είναι κατά προσέγγιση· η εξαγωγή ορίζει την τελική ισορροπία.", hi: "पूर्वावलोकन में संगीत का स्तर अनुमानित है; अंतिम संतुलन एक्सपोर्ट तय करता है।", ja: "プレビューの音楽の音量はおおよそのものです。最終的なバランスは書き出しで決まります。", zh: "预览中的音乐音量仅为近似值，最终平衡由导出决定。"
  },
  'inspector.silent': {
    en: "Silent scene: this narration is not spoken or shown", de: "Stumme Szene: Dieser Sprechtext wird weder gesprochen noch angezeigt", fr: "Scène muette : ce commentaire n'est ni lu ni affiché", es: "Escena muda: esta narración no se locuta ni se muestra",
    it: "Scena muta: questa narrazione non viene letta né mostrata", nl: "Stille scène: deze voice-over wordt niet uitgesproken of getoond", sv: "Tyst scen: den här berättarrösten varken läses upp eller visas", da: "Tavs scene: denne fortællertekst bliver hverken læst op eller vist",
    no: "Stille scene: denne fortellerteksten blir verken lest opp eller vist", pl: "Scena bez głosu: ten tekst lektora nie jest czytany ani pokazywany", pt: "Cena muda: esta narração não é falada nem mostrada", cs: "Němá scéna: tento komentář se nečte ani nezobrazuje",
    el: "Βουβή σκηνή: αυτή η αφήγηση δεν εκφωνείται ούτε εμφανίζεται", hi: "मौन दृश्य: यह वर्णन न बोला जाता है, न दिखाया जाता है", ja: "無音のシーン：このナレーションは読み上げも表示もされません", zh: "静音场景：此旁白不会朗读，也不会显示"
  }
};

// The narration refusal also names the other way out. The tab is called Scene in the app, and the word inspector appears nowhere on
// screen. Each text brings its own separator, because Japanese and Chinese join sentences without a space.
const SILENT_WAY_OUT = {
  en: " Or mark the scene as silent on the Scene tab.", de: " Oder markiere die Szene im Tab Szene als stumm.", fr: " Ou marquez la scène comme muette dans l'onglet Scène.", es: " O marca la escena como muda en la pestaña Escena.",
  it: " Oppure segna la scena come muta nella scheda Scena.", nl: " Of markeer de scène als stil op het tabblad Scène.", sv: " Eller markera scenen som tyst på fliken Scen.", da: " Eller markér scenen som tavs på fanen Scene.",
  no: " Eller merk scenen som stille på fanen Scene.", pl: " Albo oznacz scenę jako bez głosu na karcie Scena.", pt: " Ou marca a cena como muda no separador Cena.", cs: " Nebo označte scénu jako němou na kartě Scéna.",
  el: " Ή σημειώστε τη σκηνή ως βουβή στην καρτέλα Σκηνή.", hi: " या दृश्य टैब में दृश्य को मौन चिह्नित करें।", ja: "または、シーンタブでシーンを無音に設定してください。", zh: "或在“场景”选项卡中将该场景设为静音。"
};

const languages = Object.keys(TEXT);
if (languages.length !== 16) throw new Error('want 16 languages, have ' + languages.length);
for (const lang of languages) {
  if (TEXT[lang].length !== KEYS.length) throw new Error(lang + ' has ' + TEXT[lang].length + ' texts, want ' + KEYS.length);
  const file = path.join(process.cwd(), 'ui', 'lang', 'storymaker', lang + '.json');
  const data = JSON.parse(fs.readFileSync(file, 'utf8'));
  if (!('storymaker.export.silent_note' in data)) throw new Error(lang + ' has no silent note to remove');
  delete data['storymaker.export.silent_note'];
  KEYS.forEach((key, index) => { data['storymaker.' + key] = TEXT[lang][index]; });
  Object.keys(EXTRA).forEach(key => {
    if (!EXTRA[key][lang]) throw new Error(lang + ' has no text for ' + key);
    data['storymaker.' + key] = EXTRA[key][lang];
  });
  if (!SILENT_WAY_OUT[lang]) throw new Error(lang + ' has no silent-scene sentence');
  data['storymaker.job_error.narration_incomplete'] += SILENT_WAY_OUT[lang];
  fs.writeFileSync(file, JSON.stringify(data, null, 2) + '\n');
}
console.log('2C strings written to ' + languages.length + ' locales');
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
Select-String -Path ui/lang/storymaker/*.json -Pattern 'silent_note'
go test ./ui -run Storymaker -count=1 -timeout 20m
gofmt -l ui
```

Expected: `Select-String` prints nothing. `go test` passes, including `TestStorymakerAppUsesOnlyKeysItHas`, `TestStorymakerLocalesCoverAllSixteenLanguages` and `TestStorymakerAppTranslationsAreTranslated`. `gofmt -l` prints nothing.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-core.js ui/desktop_storymaker_i18n_test.go ui/lang/storymaker/cs.json ui/lang/storymaker/da.json ui/lang/storymaker/de.json ui/lang/storymaker/el.json ui/lang/storymaker/en.json ui/lang/storymaker/es.json ui/lang/storymaker/fr.json ui/lang/storymaker/hi.json ui/lang/storymaker/it.json ui/lang/storymaker/ja.json ui/lang/storymaker/nl.json ui/lang/storymaker/no.json ui/lang/storymaker/pl.json ui/lang/storymaker/pt.json ui/lang/storymaker/sv.json ui/lang/storymaker/zh.json
git commit -m "feat(storymaker): 2C strings in 16 locales, silent note removed" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E2f: Contract slice and docs
**Batch:** 2C  **Depends on:** E2e, A26
**Files:**
Modify: `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Modify: `internal/storymaker/render/AGENTS.md`
Modify: `internal/storymaker/AGENTS.md`
Modify: `ui/js/desktop/apps/AGENTS.md`
Modify: `internal/server/AGENTS.md`
Modify: `internal/desktop/AGENTS.md`
Modify: `documentation/storymaker.md`

No failing test: this task changes docs only. Earlier slices shift lines, so match every edit by the quoted text, not by a line number.

- [ ] **Step 1: Contract section 2**

Edit `docs/superpowers/specs/2026-11-01-storymaker-contract.md`. Each old text below is one line or part of one line.

1. Old, as M16 leaves it: ``rhythm?:"calm"|"standard"|"lively"}``
   New: ``rhythm?:"calm"|"standard"|"lively", auto_sfx?:bool}``
2. Old: ``- `kind` is not defaulted. An absent kind is a storyboard scene.``
   New: the same line, then this new line: ``- `music.track` is a positive decimal media id and excludes `music.file`. `style.auto_sfx`, like `style.rhythm`, is not written when absent.``
3. In the table under "Phase 2 support", in the row ``| Beats the player plays |``:
   Old: ``camera, wait |``
   New: ``camera, wait, sfx |``
4. In the row ``| Beats accepted and ignored |``:
   Old: ``sfx, chart, and show``
   New: ``chart, and show``
5. Remove this whole row: ``| `music` other than `none` | Accepted, ignored, minor `not_supported_yet`. |``
6. In the row of `missing_field`:
   Old: ``voice provider and language, music file |``
   New: ``voice provider and language, music file, `/music/mood` |``

- [ ] **Step 2: Contract sections 3 and 3b**

1. The `version` entry, as E1j leaves it.

Old:

```
version       12 hex characters: sha256 over the bytes of every file below ui/img/storymaker/
              except catalog.json, in sorted path order; text files LF-normalised.
```

New:

```
version       12 hex characters: sha256 over the bytes of every file below ui/img/storymaker/ except catalog.json,
              in sorted path order; text files LF-normalised, WAV files raw.
```

2. Old: ``- `thumb` is `thumbs/<id>.svg`. Every thumbnail is a deterministic SVG composition (no PNG).``
   New: ``- `thumb` is `thumbs/<id>.svg`. Every entry except `sfx` has a thumb. Every thumbnail is a deterministic SVG composition (no PNG).``
3. Old: ``| `sfx` | `file` (phase 2) |``
   New: ``| `sfx` | `file` (take 1), `takes[{file, dur_ms, onset_ms, peak_ms, impact_ms, l50_lufs, true_peak_dbtp}]`, `impact_ms`, `align` (`peak`, `onset`), `hit?{over_db, dip_db}`, `pattern?{spacing_ms:[min,max], default_duration_ms}`, `source{kind, basis, url}` |``
4. Old: ``| palettes | 2 | `palette.sunny` `palette.calm` |``
   New: the same line, then this new row: ``| sfx | 7 | `sfx.whoosh` `sfx.pop` `sfx.click` `sfx.ding` `sfx.success` `sfx.error` `sfx.typing` |``
5. Old, as E1j leaves it: ``The catalog has 79 entries (4 + 23 + 4 + 10 + 9 + 10 + 7 + 2 + 6 + 4) and 2 palettes. Entries of other kinds (`sfx`) belong to phase 2.``
   New: ``The catalog has 86 entries (4 + 23 + 4 + 10 + 9 + 10 + 7 + 2 + 6 + 4 + 7) and 2 palettes.``
6. Old: `` `thumbs/<id>.svg`, one per entry. ``
   New: `` `thumbs/<id>.svg`, one per entry except `sfx`. ``
7. Section 3b, as E1j leaves it. A6 put three `sfx` entries at the head of the `catalog-mini` list.
   Old: ``Its 23 entries are `char.office-woman-1`,``
   New: ``Its 26 entries are `sfx.whoosh`, `sfx.pop`, `sfx.typing`, `char.office-woman-1`,``
8. In the same line:
   Old: ``Its first 20 entries are the mini fixture text recorded by parts A and C; phase 2 added the three speech styles.``
   New: ``Twenty of them are the mini fixture text recorded by parts A and C; phase 2 added the three speech styles and the three `sfx` entries.``

- [ ] **Step 3: Contract section 4**

1. Old: ``Params map[string]string; Spots map[string]Spot; Floor *Floor; Variants []string; Aspects map[string]AspectOverride }``
   New: ``Params map[string]string; Spots map[string]Spot; Floor *Floor; Variants []string; Aspects map[string]AspectOverride;``, then this new line: ``                   Takes []SFXTake; ImpactMS float64; Align string; Hit *SFXHit; Pattern *SFXPattern; Source *SFXSource }``
2. After the paragraph that starts ``` `LoadCatalog` checks the manifest: schema```, add this paragraph: ``` `LoadCatalog` also checks the sfx fields: `file` equals `takes[0].file`, at least 2 takes, `align` set, `source` set, and the thumb check is skipped for this kind. `VerifyFiles` checks every take and skips an empty thumb. ```
3. Old: ``### Code scenes``
   New: this subsection, then the same heading:

````markdown
### Music API

```go
type SFXTake struct{ File string; DurMS, OnsetMS, PeakMS, ImpactMS, L50LUFS, TruePeakDBTP float64 }
type SFXHit struct{ OverDB, DipDB float64 }
type SFXPattern struct{ SpacingMS [2]float64; DefaultDurationMS float64 }
type SFXSource struct{ Kind, Basis, URL string }

var MusicMoods = []string{"upbeat-corporate", "calm-ambient", "inspiring", "playful", "tech-pulse", "dramatic", "lo-fi", "acoustic"}
func MusicPrompt(mood, prompt string) string
func MusicKeyGenerated(mood, prompt string) string
func MusicKeyFile(path string, size int64, mtime time.Time) string
func MusicKeyTrack(id string, size int64, mtime time.Time) string
func MusicVolume(m *Music) float64                // 0.25 when absent
type MusicMeta struct{ V int; Ext string; DurationMS int; LoudnessI *float64 }   // JSON: v, ext, duration_ms 1..3600000, loudness_i? in LUFS, -100..20
func (p *ProjectStore) WriteMusic(ctx context.Context, id, key, ext string, data []byte) error
func (p *ProjectStore) WriteMusicMeta(ctx context.Context, id, key string, m MusicMeta) error
func (p *ProjectStore) MusicInfo(ctx context.Context, id, key string) (meta MusicMeta, audioExt string, err error)
```

Keys are lower-case hex sha256 of `generated|mood|prompt`, `file|path|size|mtime` or `track|id|size|mtime`, with `mtime` as decimal `UnixNano`. Length is excluded. `meta.json` holds the duration, so `GET /narration` never decodes a track. The helper package `internal/storymaker/storymakertest` seeds narration and music for tests (`SeedNarration`, `SeedMusic`).

### Code scenes
````

4. Old: ``type MusicRef struct { Key, Ext string; DurationMS int }``
   New: ``type MusicRef struct { Key, Ext string; DurationMS int; LoudnessI *float64 }``

- [ ] **Step 4: Contract section 5**

1. Replace the whole row that starts ``| `POST /projects/{id}/render` | write | 202 `{job}` |`` with these two rows:

```
| `POST /projects/{id}/render` with optional body `{burn_in_captions?, srt, vtt, stems, quality}` | write | 202 `{job}` | 428, 400 `precondition_invalid`, 412 `precondition_failed`, 400 `invalid_idempotency_key`, 400 `invalid_request`, 409 `idempotency_conflict`, 422 `invalid_story` (with `issues`), 422 `code_error`, 409 `ffmpeg_unavailable`, 409 `browser_unavailable` (message is the status issue), 409 `narration_incomplete` (`missing`, `chars`), 409 `music_incomplete` (`key`, `music_issue?`), 429 `busy` |
| `POST /projects/{id}/music` `{duration_ms, generate?}` | write | 202 `{job}`; 200 `{job}` existing; 200 `{job:null}` when no music is needed, the key's file and meta exist, or mode generated has no stored file and the call lacks `generate:true`. `duration_ms` is required for mode generated, 1000 to `max_duration_seconds*1000`. `generate:true` is the user's click, and the job calls a provider only for the key this call read. Coalesced per project and kind. | 400 `invalid_request`, 400 `invalid_argument`, 404 `project_not_found`, 409 `music_unavailable` (mode generated while `cfg.MusicConfigured()` is false), 409 `budget_exceeded`, 422 `invalid_music_file` (source not wav or mp3 by its bytes, over 1 GiB, or missing, then with `music_issue:"source_missing"`), 429 `busy` |
```

2. Old: ``The same key with the same story etag returns the same job, again with 202. The same key with another etag gives 409 `idempotency_conflict`.``
   New: ``The same key, etag and options return the same job, again with 202; another etag or other options give 409 `idempotency_conflict`.``
3. Old, as E1j leaves it: ``- 128 narrate jobs may wait, in their own queue. `/narrate` coalesces per project and kind.``
   New: ``- 128 narrate and music jobs may wait, in their own queue. `/narrate` and `/music` coalesce per project and kind.``
4. Old: the row that starts ``| `catalog_version` | string |``
   New: the same row, then this new row: ``| `music_configured` | bool | `cfg.MusicConfigured()`: music generation can run. The music panel offers mode `generated` only then. |``
5. The code block under ``### Job object``, as E1j leaves it.

Old:

```
Job {id, project_id, kind:"render"|"narrate", status, progress:0..1, phase?, frames_done, frames_total,
     pieces_done?, pieces_total?,
     output?:{path, size}, error?:{code, message}, created_at, updated_at, started_at?, finished_at?}
status  queued | running | succeeded | failed | cancelled | interrupted
phase   starting | compiling | rendering | encoding | writing | synthesizing
```

New:

```
Job {id, project_id, kind:"render"|"narrate"|"music", status, progress:0..1, phase?, frames_done, frames_total,
     pieces_done?, pieces_total?,
     output?:{path, size, stems?:{narration?, music?, sfx?}, srt?, vtt?, library?},
     warnings?:[{code, message}], error?:{code, message}, created_at, updated_at, started_at?, finished_at?}
status  queued | running | succeeded | failed | cancelled | interrupted
phase   starting | compiling | mixing | rendering | encoding | writing | synthesizing | generating
```

6. Old: ``- `phase` is omitted while it is empty (a queued job). `starting` covers the browser start and a first-time download. `mixing` is defined in the code but phase 1 never sets it.``
   New: ``- `phase` is omitted while empty. `starting` covers the browser start and a first-time download. `mixing` runs between the report and the first frame.``
7. In the line that starts ``Job error codes:``, add ``, `narration_incomplete`, `music_failed`, `music_incomplete` `` before its closing period.
8. Old: ``with the job id, title, output path, frames, fps, size, encoder, transport, catalog version, render seconds and creation time.``
   New: ``with the job id, title, output path, frames, fps, size, encoder, transport, catalog version, render seconds, creation time, `stems`, `srt`, `vtt`, `library`, `warnings`, `loudness:{i, tp, master_gain_db}` and `cue_precision`.``
9. Old: ``The library copy in `Videos/Storymaker/` is phase 2. `Videos` is a read-only media mount for desktop writes.``
   New: ``The library copy is `Videos/Storymaker/<slug>.mp4`, with `-2`, `-3` on collision. It goes through `desktop.Service.WriteStorymakerVideo`. Stems and subtitles live in `exports/<slug>-<job8>/`. A failed extra leaves the MP4 and adds a warning.``
10. In the narration answer, as E1j leaves it:
   Old: ``"music": {"key":"<64 hex>","ext":"mp3","duration_ms":61000}``
   New: ``"music": {"key":"<64 hex>","ext":"mp3","duration_ms":61000,"loudness_i":-14.2}``
11. Directly under the code block of the narration answer, add this bullet: ``- `music.loudness_i` is the music job's measurement of the stored track in LUFS, for the preview's bed gain. It is absent without FFmpeg, after a failed meter and for a silent track. The mix measures for itself. A story in mode `file` with no source yet has no `music_issue`: `music` is null, `POST /music` answers `{job:null}` and the validator reports `/music/file`.``

- [ ] **Step 5: Contract section 6**

1. In the line that starts ``- Beat defaults``, as E1j leaves it:
   Old: ``highlight 0.8 s; say the block length.``
   New: ``highlight 0.8 s; say the block length; sfx 0 (a pattern `default_duration_ms`).``
2. M16's numbered scheduling rules end with the item that starts ``  9. After the loop, `holdsEnd` is the largest``. Directly after that line, add:

```markdown
- Impact (`sfx`): `impact = chosen take impact_ms/1000`, 0 when the catalog has no such entry. The take comes from `seed.takeIndex` with `lastTake.get(sound)`, and `lastTake` is updated. A cued sfx uses `t = cueTime + delay - lead - impact`. An sfx with `with:"prev"` uses `(prev.landing ?? prev.start) + delay - impact`.
- Typing: `prng = seed.random(sceneId+':'+j+':'+sound)`; `x` starts at the beat's start plus the first take's impact; key events sit at event time `x - impact` of that key's take, with `x` advancing by `90+50*prng()` ms while `x < start+dur`; the take comes from `seed.takeIndex`.
- Auto-SFX, when `style.auto_sfx`: each show with entry `pop` gets a `sfx.pop` event with its impact on the landing, unless an sfx beat has the same `cue` or runs `with:"prev"` on it.
```

3. In the code block under ``### Compile report``:
   Old: ``audio_events:[{kind:"narration"|"dialogue", ref, start_ms, scene_id, block_id, piece}],``
   New: ``audio_events:[{kind:"narration"|"dialogue", ref, start_ms, scene_id, block_id, piece} | {kind:"sfx", ref, start_ms, scene_id, take}],``
4. In the bullet under that block, as E1j leaves it:
   Old: ``- `audio_events` has one entry per piece; Go owns the gain, so there is no `gain` field.``
   New: ``- `audio_events` has one entry per piece and one per sound; Go owns the gain, so there is no `gain` field. For `sfx`, `ref` is the catalog id and `take` is a 0-based index into its `takes`. `start_ms` is always film-global: for `sfx` it is `Math.round((sceneBase + eventTime)*1000)`, where `eventTime` is scene-local, the `start` of scheduling step 7 for a plain sfx and the key's event time for a typing key.``

- [ ] **Step 6: Contract section 7**

1. Old: ``(`listener.go`, `ingest.go`, `render.go`, `browser.go`, `session.go`, `egress.go`, `ffmpeg.go`, `jobs.go`)``
   New: ``(`listener.go`, `ingest.go`, `render.go`, `browser.go`, `session.go`, `egress.go`, `ffmpeg.go`, `jobs.go`, `mix.go`, `mixinput.go`, `meter.go`, `gate.go`, `subtitles.go`, `options.go`)``
2. Old: ``(`catalog.json` and every file it names)``
   New: ``(`catalog.json` and every file it names, except the sfx WAVs)``
3. Before the row that starts ``| `POST <base>/frame``, add: ``| `GET <base>/start?t=<token>` | long poll after the report: held at most 20 s, answers `{"pending":true}`, `{"start":true}` or `{"error", "message"}`; 403 `bad_token`, 404 before an accepted report |``
4. Old, as E1j leaves it: ``` `/report`, `/frame` and `/error` require it as `t` ```
   New: ``` `/report`, `/frame`, `/error` and `/start` require it as `t` ```
5. Replace the item that starts ``2. `POST /report` with the compile report.`` with: ``2. `POST /report?t=` validates and answers `{"start":false,"pending":true}` at once, or `{"start":false, "error":"<code>", "message":"..."}`. Go then mixes in the background. 2a. The page long-polls `GET /start?t=`; each call holds at most 20 s and answers `{"pending":true}`, `{"start":true}` or `{"error", "message"}`, and 404 before an accepted report. The 10 s no-frame watchdog starts with FFmpeg, after the mix.``
6. Old: ``- A silent AAC track from `anullsrc` (48 kHz, stereo) is always added, so the MP4 has the same stream layout in every phase.``
   New: ``- The audio is the mix WAV as input 1 with `-format_whitelist wav`. A film with no audio events and no music keeps `anullsrc` with `-t`. AAC 192k.``
7. Old: ``plus the encoder's filter tail, `-shortest`, `-movflags +faststart`.``
   New: ``plus the encoder's filter tail, `-movflags +faststart`.``
8. Old: ``- The encoder comes from `video_studio.hardware_encoding` and `hardware_device`, else libx264. FFmpeg comes from `video_studio.ffmpeg_path` or PATH (`videostudio.CheckRuntime`).``
   New: the same line, then:

```markdown

### Frame gate

After FFmpeg ends and before anything is published, `finish` probes the output with ffprobe. The probes of one check share one timeout, the larger of 120 s and the file size at 10 MiB/s, and each has an 8 MiB output cap.

- `ffprobe -v error -protocol_whitelist file -format_whitelist mov -count_packets -select_streams v:0 -show_entries stream=nb_read_packets -of csv=p=0 -i FILE` must equal the frame count.
- `ffprobe -v error -protocol_whitelist file -format_whitelist mov -select_streams <s> -show_entries stream=duration -of csv=p=0 -i FILE`, once for `v:0` and once for `a:0`: the two durations must differ by under 25 ms.

A failure is `render_failed`, and nothing is published. The order in `finish` is `ff.Wait`, stat, size, `ProbeOutput` and `CheckOutput`, phase writing, MP4, `publishExtras`, record. An empty `FFprobePath` fails the job with `ffmpeg_unavailable`.
```

- [ ] **Step 7: Contract section 9**

1. Old: ``thirteen scripts in order``
   New: ``fourteen scripts in order``
2. Old: `` `storymaker-voice.js`, `storymaker.js` ``
   New: `` `storymaker-voice.js`, `storymaker-music.js`, `storymaker.js` ``

- [ ] **Step 8: Child docs**

Edit `internal/storymaker/render/AGENTS.md`.

1. Old: ``and a silent AAC track from `anullsrc` ``
   New: ``and AAC 192k from the mix WAV (input 1, `-format_whitelist wav`), or from `anullsrc` with `-t` for a film without sound ``
2. Old: ``(so the stream layout is video plus audio for later phases).``
   New: ``(`-shortest` is gone; the frame gate checks the audio length).``
3. Old: ``no frame within 10 s after `start` kills the job``
   New: ``no frame within 10 s after FFmpeg started (after the mix, not when the report is accepted) kills the job``
4. Old: ``- **Egress** (`egress.go`,``
   New: these bullets, then the same text:

```markdown
- **Mix** (`mix.go`, `mixinput.go`, `meter.go`): a pure Go stream mixer at 48 kHz in blocks of 4800 frames. FFmpeg decodes pieces and music to s16 in job staging (`-protocol_whitelist file -format_whitelist wav,mp3 -map 0:a:0`, `-t` and `-fs` against decode bombs), each hash once. Order: voice match (cap +6 dB), bed gain to the reference (narration minus 12 LU, -18 LUFS without narration, relative to volume 0.25, cap +24 dB), ducking (6 dB, attack 50 ms, release 400 ms, look-ahead 50 ms, threshold -45 dBFS), carve (2200 Hz, Q 1.0, -5 dB, 20 ms ramps, only inside spans), impact dips, master gain `min(-16-I, -1-TP, -0.1-max stem peak)`. No limiter. Mix minus the three stems is 0 at every sample before encoding, and only there. The meter is one FFmpeg process on a float pipe: I from ebur128, TP from loudnorm. `MeterStored` runs the same graph over a stored file on stdin, for the music job. A silent bed is dropped with the warning `music_silent`.
- **Deferred start** (`render.go`, `listener.go`): `POST /report` validates and answers `pending` at once, a goroutine mixes on `mixCtx`, starts FFmpeg and the watchdog, and closes `startCh`. The page long-polls `GET /start` (held 20 s). `run.audioPath` survives `recoverEncoder`, so a hardware-encoder fallback keeps the mix. `execute` cancels the mix and waits for it before staging is removed.
- **Frame gate** (`gate.go`): packet count equal to the frame count and audio against video under 25 ms, after FFmpeg and before anything is published. See contract section 7.
- **Options and extras** (`options.go`, `subtitles.go`, `publishExtras`): `RenderOptions` joins the idempotency fingerprint, `high` sets CRF 18, `burn_in_captions` overrides `style.captions` in the snapshot before `Validate`. Stems and subtitles go to `exports/<slug>-<job8>/`, the library copy to `Videos/Storymaker/`. A failed extra leaves the MP4 and adds `stem_failed`, `subtitle_failed` or `library_failed`. A silent stem is omitted. The snapshot holds no sfx WAV.
```

5. In the **Audio info and fonts** bullet that E1j wrote, delete the last sentence, because exports now have sound:
   Old: `` `glyphs.json` is in the snapshot. Exports stay silent in batch 2B: the audio track is still `anullsrc`. ``
   New: `` `glyphs.json` is in the snapshot. ``

Edit `internal/storymaker/AGENTS.md`.

Old: ``## Work Guidance``
New: these bullets, then the same heading:

```markdown
- Music (`music.go`): `MusicMoods`, `MusicPrompt`, the keys `generated|mood|prompt`, `file|path|size|mtime` and `track|id|size|mtime`, with length excluded. `MusicMeta` is written after the audio file, and a file without valid meta counts as missing. Its optional `loudness_i` is the job's best-effort measurement for the preview's bed gain; the mix measures for itself. `auto_sfx` and the `sfx` beat left "not supported yet" in 2C, and a mood outside the presets is an error.
- Catalog `sfx` entries carry `takes`, `impact_ms`, `align`, `hit?`, `pattern?` and `source`, and have no thumb. `render.snapshot` skips them, because the page never reads WAVs.
- `storymakertest/seed.go` (`SeedNarration`, `SeedMusic`) gives fixtures audio, because the `narration_incomplete` and `music_incomplete` gates refuse a story without it.

## Work Guidance
```

Edit `ui/js/desktop/apps/AGENTS.md`.

1. In the first bullet of the Storymaker section, as E1j leaves it:
   Old: ``thirteen classic scripts``
   New: ``fourteen classic scripts``
   Old: ``-voice.js` (voice panel, narration status, captions toggle), then``
   New: ``-voice.js` (voice panel, narration status, captions toggle), `-music.js` (the music panel), then``
2. Old: ``- Verify: `go test ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker' -count=1` ``
   New: these bullets, then the same text:

```markdown
- Music (`S.createMusicPanel`, side tab `music`): generated music starts only from the button that shows the mood and the prompt, because autosave changes the key at every pause in typing and the call is paid. A file or a track starts one job per source without a click. `file` and `track` stay exclusive. `music_issue: "source_missing"` names the source and offers another file. A refused start shows the server's reason under the sentence, a film over `max_duration_seconds` closes Generate, and a file replaced on disk offers Try again, because its source already had its one job. Story-level edits go through `inst.editStory`.
- Preview sound (`storymaker-audio.js`): a layer around the narration engine adds one `<audio>` bed through `MediaElementSource`, a WebAudio duck from stored levels (no carve) and SFX from `audio_events`. Bed and SFX take the gains of `computeMixLevels` from the stored speech levels, `music.loudness_i` and the catalog take, so the balance stays close to the export. Without `loudness_i` the bed plays the raw file at the story volume and the music panel says the level is approximate. SFX share the narration's anchor with no lead, and a sound up to 50 ms late still plays at once. An element that loses its `src` also calls `load()`, or it keeps its connection and the file handle. SFX URLs come only from `catalog.byId[ref].takes[take].file`, never from message text.
- Export refusals: 409 `narration_incomplete` and `music_incomplete` show what is missing and lead to the panel through `inst.showPanel(name)`. `storymaker.export.silent_film` shows only without narration, music and audio events.
- Silent switch (`data-stm="scene-silent"`, inspector): writes `scene.silent` or removes it, with an immediate save. It is the way past `narration_incomplete` that needs no audio, and the refusal names it.
```

Edit `internal/server/AGENTS.md`.

Old: ``### ECalc catalog API``
New: this subsection, then the same heading:

```markdown
### Storymaker sound

`POST /api/desktop/storymaker/projects/{id}/music` is a job in the narrate queue, kind `music`, coalesced per project and kind. Generated music needs `generate:true` on the route and the same mood and prompt when the job runs, needs `cfg.MusicConfigured()`, passes `IsBlocked("music_generation")` and a read-only re-check before the call, and writes `audio/music/<key>.pending.json`, so a rerun after a failed copy makes no provider call. A file opens only through `Store.Open`. A track must be media type `music` with a local path under `DataDir/audio`, opened as `Music/<rel>`. Sources are wav or mp3 by their bytes, at most 1 GiB, 1 s to 3600 s. A moved source reports `music_issue: source_missing` and keeps the narration. `POST /render` refuses with 409 `narration_incomplete` and 409 `music_incomplete` after validation. The job measures the stored copy best effort into `meta.loudness_i` (`storymakerMeasureMusic`); no FFmpeg or a silent file leaves it empty and never fails the job. `/status` reports `music_configured`. Test seam: `storymakerMusicOverride`.

### ECalc catalog API
```

Edit `internal/desktop/AGENTS.md`.

Old: ``- Desktop cloud transfers use `StatStoragePath` for fresh rooted metadata,``
New: this bullet, a blank line, then the same text:

```markdown
- `WriteStorymakerVideo` is the only desktop write into the read-only `Videos` mount. It takes a slug (`^[A-Za-z0-9-]{1,80}$`) and builds the path itself, `Videos/Storymaker/<slug>.mp4` with `-2`, `-3` on collision. It refuses read-only, checks read-only again under the lock, and needs free space of size plus 64 MiB. It creates `Storymaker` on the first copy and skips `RegisterMedia`. Verify `go test ./internal/desktop -run WriteStorymakerVideo`.
```

- [ ] **Step 9: User page**

Edit `documentation/storymaker.md`.

1. Old: ``- **Code tab.** `story.json` and every code scene in an editor.``
   New: this bullet, then the same text:

```markdown
- **Music (right).** The source of the background music: none, generated by AI (only when music generation is set up), or a WAV or MP3 file or a Noisemaker track. Generated music starts only when you click the button, because a generation may cost money. A file or a track is copied into the project by itself. A moved file is named here, with an offer to choose another one.
```

2. Old: ``## For developers``
   New: this section, then the same heading:

```markdown
## Sound

The export mixes narration, music and sound effects in AuraGo itself and writes AAC 192k, about -16 LUFS at -1 dBTP at most. The music ducks under speech. An `sfx` beat plays a sound from the catalog (`sfx.whoosh`, `sfx.pop`, `sfx.click`, `sfx.ding`, `sfx.success`, `sfx.error`, `sfx.typing`) on its landing, and `style.auto_sfx` adds a pop to each show with entry `pop`.

An export is refused while narration audio or the music file is missing. The export panel says what is missing and opens the panel that fixes it. A scene marked as silent on the Scene tab exports without its narration and keeps the text, while a `say` with its own text still needs audio. A film without narration, music and sound effects exports with a silent track, and the panel says so before you start.

`POST /api/desktop/storymaker/projects/{id}/render` takes an optional body `{burn_in_captions, srt, vtt, stems, quality}`. Stems and subtitles land in `exports/<title>-<job>/`, and a copy of the MP4 lands in `Videos/Storymaker/`. A failed extra leaves the MP4 and adds a warning to the job. The dialog for these options comes with the next update.

## For developers
```

3. The opening paragraph. Replace the text E1j leaves, from `This page describes` to `the agent come in later versions.`, with:

```text
This page describes
the current version: the editor, the preview, voices and narration, captions, music, sound effects and an export with sound. The
agent comes in a later version.
```

4. The other sentences E1j leaves about what is still missing:
   Old: ``Videos still have no sound, even for a narrated story: the narration plays in the preview, and the export draws its captions.``
   New: ``Videos carry the narration, the music and the sound effects.``
   Old: ``Music and sound effects, the agent panel``
   New: ``The agent panel``
5. The **Preview** bullet that E1j wrote:
   Old: ``A switch in the voice panel turns the captions off.``
   New: ``A switch in the voice panel turns the captions off. Music and sound effects play along at about their export level. The preview has no presence carve, voice match or impact dips, so its balance is close, but the export is final.``

- [ ] **Step 10: Check**

```powershell
node -e "for (const p of ['docs/superpowers/specs/2026-11-01-storymaker-contract.md','internal/storymaker/music.go','internal/storymaker/storymakertest/seed.go','internal/storymaker/render/mix.go','internal/storymaker/render/mixinput.go','internal/storymaker/render/meter.go','internal/storymaker/render/gate.go','internal/storymaker/render/subtitles.go','internal/storymaker/render/options.go','internal/desktop/service_media.go','internal/server/storymaker_narrate.go','ui/js/desktop/apps/storymaker-music.js','ui/js/desktop/apps/storymaker-audio.js','assets/storymaker/README.md']) if (!require('fs').existsSync(p)) { console.error('missing', p); process.exit(1); } console.log('paths ok')"
Select-String -Path docs/superpowers/specs/2026-11-01-storymaker-contract.md -Pattern '-shortest','The catalog has 79','storymaker-voice.js`, `storymaker.js','block_id, piece}\],'
```

Expected: `paths ok`, and `Select-String` prints nothing.

- [ ] **Step 11: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add -f docs/superpowers/specs/2026-11-01-storymaker-contract.md
git add internal/storymaker/render/AGENTS.md internal/storymaker/AGENTS.md ui/js/desktop/apps/AGENTS.md internal/server/AGENTS.md internal/desktop/AGENTS.md documentation/storymaker.md
git commit -m "docs(storymaker): 2C contract slice and child docs" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E2g: Tour with sound
**Batch:** 2C  **Depends on:** E2e, A26
**Files:**
Test: `internal/server/storymaker_app_browser_test.go`

- [ ] **Step 1: Impact**

Run this from the main checkout. Warn on HIGH or CRITICAL and confirm UNKNOWN with a text search, per outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "TestStorymakerAppRealServerBrowser" --direction upstream --repo .
```

- [ ] **Step 2: Write the test**

Edit `internal/server/storymaker_app_browser_test.go`. E1k's import block has `encoding/json`, `io`, `net/http`, `regexp`, `strings`, `testing` and `time`, but neither `os/exec` nor `strconv`. Add those two.

Old text:

```go
	"path/filepath"
```

New text:

```go
	"os/exec"
	"path/filepath"
```

Old text:

```go
	"regexp"
```

New text:

```go
	"regexp"
	"strconv"
```

Old text:

```go
func TestStorymakerAppRealServerBrowser(t *testing.T) {
```

New text:

```go
// smappNarrateByRoute starts the narrate job through the real route and waits until the project has no missing block.
func smappNarrateByRoute(t *testing.T, origin, id string) {
	t.Helper()
	res, err := http.Post(origin+storymakerAPIBase+"/projects/"+id+"/narrate", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusAccepted && res.StatusCode != http.StatusOK {
		t.Fatalf("narrate = %d %s", res.StatusCode, body)
	}
	deadline := time.Now().Add(2 * time.Minute)
	for {
		var answer struct {
			Missing []json.RawMessage `json:"missing"`
		}
		smappGet(t, origin, "/projects/"+id+"/narration", &answer)
		if len(answer.Missing) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d narration blocks are still missing after 2 minutes", len(answer.Missing))
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// smappMaxVolume runs FFmpeg's volumedetect over the first audio stream of a file and returns its max_volume in dB.
func smappMaxVolume(t *testing.T, ffmpeg, file string) float64 {
	t.Helper()
	out, err := exec.Command(ffmpeg, "-hide_banner", "-nostats", "-i", file, "-map", "0:a:0", "-af", "volumedetect", "-f", "null", "-").CombinedOutput()
	if err != nil {
		t.Fatalf("volumedetect: %v\n%s", err, out)
	}
	match := regexp.MustCompile(`max_volume: (-?[0-9.]+) dB`).FindSubmatch(out)
	if match == nil {
		t.Fatalf("volumedetect printed no max_volume:\n%s", out)
	}
	value, err := strconv.ParseFloat(string(match[1]), 64)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestStorymakerAppRealServerBrowser(t *testing.T) {
```

A24 leaves this tour without a seed, so the export still follows E1k's narration step directly. E1k's narrate job has stored the audio of scene s1 by now, and this step runs the job for code2. Add the step ahead of the export.

Old text:

```go
	// Export: the real render runs (FFmpeg and Chrome), progress arrives on the project's event stream and the panel ends in
```

New text:

```go
	// Four side tabs stay inside the side panel: the strip wraps instead of clipping the last tab.
	if !page.MustEval(`()=>{const r=document.querySelector('[data-stm="side"]').getBoundingClientRect().right;return [...document.querySelectorAll('[data-stm="side-tab"]')].every(b=>b.getBoundingClientRect().right<=r);}`).Bool() {
		t.Fatal("a side tab overflows the side panel")
	}

	// The gate: narration text without audio stops the export. The panel names the scene and offers the voice panel, and the same
	// export passes once the narrate job has stored the audio. The code scene code2 is still selected.
	smappClick(t, page, `[data-stm="side-tab"][data-tab="scene"]`)
	page.MustElement(`[data-stm="narration"]`).MustInput("Every packet gets checked.")
	smappWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	smappClick(t, page, `[data-stm="export"]`)
	smappWait(t, page, `()=>!!document.querySelector('[data-stm="export-start"]') && !document.querySelector('[data-stm="export-start"]').disabled`)
	smappClick(t, page, `[data-stm="export-start"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="export-refused"]')?.dataset.code==='narration_incomplete'`)
	if missing := smappText(page, `[data-stm="export-missing"]`); !strings.Contains(missing, "code2") {
		t.Fatalf("the refusal does not name the scene: %q", missing)
	}
	smappClick(t, page, `[data-stm="export-go-voice"]`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="export-panel"]').hidden`)
	smappNarrateByRoute(t, origin, id)
	smappNoErrors(t, page)

	// Export: the real render runs (FFmpeg and Chrome), progress arrives on the project's event stream and the panel ends in
```

Old text:

```go
	windows := page.MustEval(`()=>auditTest.state.windows.size`).Int()
```

New text:

```go
	// The film has a real AAC track. Silence reads -91 dB and the mix peaks near its ceiling of -1 dBTP, so -30 dB separates them.
	var aac bool
	for _, st := range doc.Streams {
		aac = aac || st.CodecType == "audio" && st.CodecName == "aac"
	}
	if !aac {
		t.Fatalf("the MP4 has no AAC track: %+v", doc.Streams)
	}
	if peak := smappMaxVolume(t, s.Cfg.VideoStudio.FFmpegPath, mp4); peak < -30 {
		t.Fatalf("the export has no audible track: max_volume %.1f dB", peak)
	}
	windows := page.MustEval(`()=>auditTest.state.windows.size`).Int()
```

- [ ] **Step 3: Run it**

Run with the shell sandbox off, per outline section 4.

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
$env:AURAGO_RUN_BROWSER_SMOKE = '1'; go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```

This task adds test code only, and A20, A24 and E2a to E2e are already in, so a pass on the first run is possible and is valid evidence. Any failure is a real defect in those tasks and goes to debug-agent:
- no `export-refused`: E2d or A24
- `narrate` answers other than 200 or 202: E1k's voice step
- `the export failed` with `narration_failed`: E1k's stub synth returns silence, and it must return speech above -40 dBFS, because A13 fails a narration stem that measures `-inf`
- `no audible track`: A20
- `a side tab overflows the side panel`: E2b's `.stm-tabs-small` rule

- [ ] **Step 4: Implement**

No product code in this task. If Step 3 failed, debug-agent finds the cause and maker-agent fixes the task that owns it. Each fix adds one test, per outline section 4.

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
$env:AURAGO_RUN_BROWSER_SMOKE = '1'; go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
gofmt -l internal/server
go vet ./internal/server
```

Expected: PASS, one `--- PASS: TestStorymakerAppRealServerBrowser` line. `gofmt -l` prints nothing.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/server/storymaker_app_browser_test.go
git commit -m "test(storymaker): app tour passes the narration gate and exports an audible film" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

