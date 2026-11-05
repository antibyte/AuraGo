# Part E1: The app for speech

Batch 2B. Run the tasks in order. `outline.md` holds the batch rules, the test matrices and the execution rules ("outline section N"). `interfaces.md` holds the frozen interfaces ("frozen section N"), and its section 10 and `reconcile.md` win over them. A group id such as `G-A-5` names a block of tasks inside a part; the index lists them.

<!-- G-E1-1 -->

### Task E1a: API client additions and error `data`
**Batch:** 2B  **Depends on:** N15, N16
**Files:**
- Modify: `ui/js/desktop/apps/storymaker-api.js`
- Modify: `ui/js/desktop/apps/storymaker-core.js`
- Test: `ui/desktop_storymaker_api_test.go`

- [ ] **Step 1: Impact**

Run this from the main checkout, once each for `normalizeError`, `errorFromResponse`, `describeError`, `createApi` and `events`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "normalizeError" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and treat UNKNOWN as unresolved until a text search confirms it (outline section 5, GitNexus).

- [ ] **Step 2: Write the failing test**

In `ui/desktop_storymaker_api_test.go`, replace this line inside `storymakerApiScript`:

```js
  console.log('storymaker app api ok');
```

with:

```js
  // the phase 2 calls, each on its route
  answers['GET ' + base + '/voices'] = json({ providers: [{ id: 'phonon' }] });
  eq((await api.voices()).providers[0].id, 'phonon', 'voices');
  call = calls.pop();
  eq([call.method, call.url], ['GET', base + '/voices'], 'voices route');
  answers['GET ' + base + '/projects/' + id + '/narration'] = json({ narration: {}, missing: [], chars: { total: 0, missing: 0 }, music: null });
  eq((await api.narration(id)).missing, [], 'narration');
  call = calls.pop();
  eq([call.method, call.url], ['GET', base + '/projects/' + id + '/narration'], 'narration route');
  answers['GET ' + base + '/projects/' + id + '/jobs'] = json({ jobs: [{ id: 'n1', kind: 'narrate' }] });
  eq((await api.listJobs(id)).map(j => j.id), ['n1'], 'listJobs gives the array');
  call = calls.pop();
  eq([call.method, call.url], ['GET', base + '/projects/' + id + '/jobs'], 'jobs route');
  answers['POST ' + base + '/projects/' + id + '/narrate'] = json({ job: { id: 'n2', kind: 'narrate' } }, 202);
  eq((await api.narrate(id)).job.id, 'n2', 'narrate gives {job}');
  call = calls.pop();
  eq([call.method, call.url, call.body === undefined], ['POST', base + '/projects/' + id + '/narrate', true], 'narrate route, empty body');
  answers['POST ' + base + '/projects/' + id + '/narrate'] = json({ job: null });
  eq((await api.narrate(id)).job, null, 'nothing missing answers job null');
  calls.pop();

  // audio addresses: lower-case hex hash and wav or mp3 only; an empty string means refused
  const hash = 'a'.repeat(64);
  eq(api.narrationUrl(id, hash, 'wav'), base + '/projects/' + id + '/audio/narration/' + hash + '.wav', 'narration url');
  eq(api.narrationUrl(id, 'A'.repeat(64), 'wav'), '', 'an upper-case hash is refused');
  eq(api.narrationUrl(id, hash, 'ogg'), '', 'ogg is refused');
  eq(api.narrationUrl(id, '../' + hash, 'wav'), '', 'a path is refused');
  eq(api.musicUrl(id, hash, 'mp3'), base + '/projects/' + id + '/audio/music/' + hash + '.mp3', 'music url');

  // the extra fields of an error body survive both error functions
  answers['POST ' + base + '/projects'] = json({ error: 'narration_unavailable', code: 'narration_unavailable', message: 'no voice', languages: ['de', 'en'] }, 422);
  await rejects(api.createProject({ title: 'x', language: 'xx' }), err => eq(err.data.languages, ['de', 'en'], 'errorFromResponse keeps languages'), 'refused create');
  eq(S.errors.normalize({ status: 422, body: { code: 'narration_unavailable', languages: ['de'] } }).data.languages, ['de'], 'normalizeError keeps languages');
  eq(S.errors.describe(t, { status: 422, code: 'narration_unavailable' }), 'storymaker.error.narration_unavailable', 'a 422 narration_unavailable has its own sentence');
  eq(S.errors.describe(t, { status: 409, code: 'narration_unavailable' }), 'storymaker.error.narration_unavailable', 'a 409 narration_unavailable has its own sentence');
  eq(S.errors.describe(t, { status: 409, code: 'voice_unknown' }), 'storymaker.error.voice_unknown', 'voice_unknown has its own sentence');

  // the event stream: the browser retries a dropped connection by itself, and a stream it closed for good comes back after a wait
  const sources = [], timers = [], states = [], jobIds = [];
  const fakeSource = url => {
    const heard = {};
    const source = {
      url, readyState: 0,
      addEventListener(name, fn) { (heard[name] = heard[name] || []).push(fn); },
      close() { source.readyState = 2; },
      fire(name, event) { (heard[name] || []).forEach(fn => fn(event || {})); }
    };
    sources.push(source);
    return source;
  };
  const clock = { setTimeout: context.setTimeout, clearTimeout: context.clearTimeout };
  context.setTimeout = (fn, ms) => { const timer = { fn, ms }; timers.push(timer); return timer; };
  context.clearTimeout = timer => { if (timers.indexOf(timer) >= 0) timers.splice(timers.indexOf(timer), 1); };
  const stream = S.createApi({ fetch: fetcher, EventSource: fakeSource }).events(id, seen => jobIds.push(seen.id), state => states.push(state));
  eq(sources.map(item => item.url), [base + '/projects/' + id + '/events'], 'one stream on its route');
  sources[0].fire('error');
  eq([timers.length, sources.length, states], [0, 1, ['error']], 'a connection the browser still retries stays with the browser');
  const closeForGood = () => { const last = sources[sources.length - 1]; last.readyState = 2; last.fire('error'); };
  const waits = [];
  for (let i = 0; i < 7; i++) {
    closeForGood();
    const timer = timers.shift();
    waits.push(timer.ms);
    timer.fn();
  }
  eq(waits, [1000, 2000, 4000, 8000, 16000, 30000, 30000], 'the wait doubles up to 30 s');
  eq(sources.length, 8, 'each wait opens a new stream');
  const reopened = sources[7];
  reopened.fire('job', { data: JSON.stringify({ job: { id: 'n9', kind: 'narrate', status: 'interrupted' } }) });
  eq(jobIds, ['n9'], 'a reopened stream hands its jobs on');
  reopened.fire('open');
  eq(states[states.length - 1], 'open', 'a reopened stream reports its state');
  closeForGood();
  eq(timers.map(timer => timer.ms), [1000], 'an open resets the wait');
  stream.close();
  reopened.fire('error');
  eq([timers.length, sources.length], [0, 8], 'close ends the wait and opens nothing more');
  Object.assign(context, clock);
  console.log('storymaker app api ok');
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^TestStorymakerAppApiAndAssetsInNode$' -count=1
```

Expected: FAIL with `node check failed` and `TypeError: api.voices is not a function`, because the client has no such call yet.

- [ ] **Step 4: Implement**

In `ui/js/desktop/apps/storymaker-api.js`, replace:

```js
    const MAX_JSON_BYTES = 16 * 1048576;
```

with:

```js
    const MAX_JSON_BYTES = 16 * 1048576;
    const AUDIO_KEY_RE = /^[0-9a-f]{64}$/;
    const AUDIO_EXTS = ['wav', 'mp3'];
    const EVENTS_WAIT_MIN_MS = 1000;
    const EVENTS_WAIT_MAX_MS = 30000;
    const EVENT_SOURCE_CLOSED = 2;

    // The only place that builds an audio address. '' means refused: a key that is not 64 lower-case hex digits, or an ext other
    // than wav and mp3.
    function audioUrl(id, folder, key, ext) {
        if (typeof key !== 'string' || !AUDIO_KEY_RE.test(key) || AUDIO_EXTS.indexOf(ext) < 0) return '';
        return BASE + '/projects/' + encodeURIComponent(id) + '/audio/' + folder + '/' + key + '.' + ext;
    }

    // An event stream that comes back. The browser retries a dropped connection by itself, but an error answer such as a 502 from a
    // proxy or a 401 closes an EventSource for good. This then opens a new one after a wait of 1 s, doubling up to 30 s, and an open
    // resets the wait. Every new connection starts with the project's unfinished and latest jobs (initialJobs on the server), so the
    // panels learn what ended meanwhile and need no poll of their own. A throw from the first open() goes to the caller.
    function reopeningSource(open) {
        const listeners = [];
        let source = null, timer = null, closed = false, wait = EVENTS_WAIT_MIN_MS;

        function retryLater() {
            timer = setTimeout(() => {
                timer = null;
                try { attach(open()); } catch (_) { retryLater(); }
            }, wait);
            wait = Math.min(wait * 2, EVENTS_WAIT_MAX_MS);
        }

        function attach(next) {
            source = next;
            listeners.forEach(([name, fn]) => next.addEventListener(name, fn));
            next.addEventListener('open', () => { wait = EVENTS_WAIT_MIN_MS; });
            next.addEventListener('error', () => {
                if (!closed && timer === null && next === source && next.readyState === EVENT_SOURCE_CLOSED) retryLater();
            });
        }

        attach(open());
        return {
            addEventListener(name, fn) {
                listeners.push([name, fn]);
                source.addEventListener(name, fn);
            },
            close() {
                closed = true;
                clearTimeout(timer);
                timer = null;
                source.close();
            }
        };
    }
```

In the same file, replace:

```js
            // events(projectId, onJob, onState) -> {close()}; onState('open' | 'error')
```

with:

```js
            // {providers}; works read-only
            voices() { return json('GET', '/voices'); },
            // the AudioAnswer of the stored story: {narration, missing, chars, music, music_issue?}
            narration(id) { return json('GET', '/projects/' + encodeURIComponent(id) + '/narration'); },
            // {job}: 202 a new job, 200 the job already running, 200 {job: null} when nothing is missing
            narrate(id) { return json('POST', '/projects/' + encodeURIComponent(id) + '/narrate'); },
            async listJobs(id) {
                const body = await json('GET', '/projects/' + encodeURIComponent(id) + '/jobs');
                return Array.isArray(body.jobs) ? body.jobs : [];
            },
            // audio addresses, only from values of the parent's own /narration answer; '' means refused
            narrationUrl(id, hash, ext) { return audioUrl(id, 'narration', hash, ext); },
            musicUrl(id, key, ext) { return audioUrl(id, 'music', key, ext); },

            // events(projectId, onJob, onState) -> {close()}; onState('open' | 'error')
```

In the same file, replace:

```js
                try { source = makeEventSource(BASE + '/projects/' + encodeURIComponent(projectId) + '/events'); } catch (_) { return { close() {} }; }
```

with:

```js
                try { source = reopeningSource(() => makeEventSource(BASE + '/projects/' + encodeURIComponent(projectId) + '/events')); } catch (_) { return { close() {} }; }
```

The other lines of `events` stay as they are. `source` is now the wrapper, so their `addEventListener` and `close` calls hold across connections.

In `ui/js/desktop/apps/storymaker-core.js`, replace:

```js
            network: !(err && err.status)
```

with:

```js
            data: body,
            network: !(err && err.status)
```

In the same file, replace:

```js
            retryAfter: Number(retryAfter) || 0,
```

with:

```js
            retryAfter: Number(retryAfter) || 0,
            data: b,
```

In the same file, replace:

```js
        else if (e.code === 'invalid_story' || e.status === 422) key = 'storymaker.error.invalid_story';
```

with:

```js
        else if (e.code === 'narration_unavailable') key = 'storymaker.error.narration_unavailable';
        else if (e.code === 'voice_unknown') key = 'storymaker.error.voice_unknown';
        else if (e.code === 'invalid_story' || e.status === 422) key = 'storymaker.error.invalid_story';
```

Do not touch `ui/lang/storymaker/*.json`. Task E1i owns those strings, and until then `TestStorymakerAppUsesOnlyKeysItHas` is red on the two new keys (allowed red state, outline section 3).

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^TestStorymakerAppApiAndAssetsInNode$' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run `detect_changes` as outline section 5 says. A PARTIAL answer on new files needs a grep confirmation in the commit body.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-api.js ui/js/desktop/apps/storymaker-core.js ui/desktop_storymaker_api_test.go
git commit -m "feat(storymaker): API calls for voices, narration and jobs, error data" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E1b: Message whitelists. Wire: parent and host
**Batch:** 2B  **Depends on:** G7, S3, S4
**Files:**
- Modify: `ui/js/desktop/apps/storymaker-player.js`
- Modify: `ui/js/storymaker/host-app.js`
- Modify: `ui/js/storymaker/host.bundle.js` (generated by `npm run build:ui`)
- Test: `ui/desktop_storymaker_model_test.go`
- Test: `scripts/storymaker-tests/08-host-app.mjs`

- [ ] **Step 1: Impact**

Run this from the main checkout, once each for `checkIncoming` and `checkMessage`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "checkIncoming" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and treat UNKNOWN as unresolved until a text search confirms it (outline section 5, GitNexus).

- [ ] **Step 2: Write the failing tests**

In `ui/desktop_storymaker_model_test.go`, replace this line inside `storymakerModelScript`:

```js
// the export size follows the aspect, as render.CanvasSize does (the preview compiles smaller)
```

with:

```js
// the compiled whitelist of phase 2: a malformed entry is dropped, the message stays
{
  const evHash = 'a'.repeat(64);
  const rich = P({ sm: 1, type: 'compiled', report: { frame_count: 90, fps: 30, cue_precision: 0.5,
    scenes: [{ id: 's1', start_frame: 0, end_frame: 90, kind: 'storyboard', cues: [{ marker: 1, at_ms: 1840, placed: 'span' }, { marker: 'x', at_ms: 1, placed: 'span' }, { marker: 2, at_ms: 5, placed: 'guess' }] }],
    audio_events: [
      { kind: 'narration', ref: evHash, start_ms: 500, scene_id: 's1', block_id: 's1:n', piece: 0, extra: 'x' },
      { kind: 'sfx', ref: 'sfx.pop', start_ms: 1836, scene_id: 's1', take: 2 },
      { kind: 'music', ref: evHash, start_ms: 1, scene_id: 's1' },
      { kind: 'narration', ref: 'r'.repeat(81), start_ms: 1, scene_id: 's1' },
      { kind: 'narration', ref: evHash, start_ms: -1, scene_id: 's1' },
      { kind: 'narration', ref: evHash, start_ms: 1, scene_id: 's1', piece: 'x' },
      'nonsense'
    ],
    captions: [
      { start_ms: 500, end_ms: 2600, lines: ['one', 'two', 'three'], scene_id: 's1', block_id: 's1:n' },
      { start_ms: 2600, end_ms: 3600, lines: ['x'.repeat(500)], scene_id: 's1', block_id: 's1:n' },
      { start_ms: 3600, end_ms: 4600, lines: [7], scene_id: 's1', block_id: 's1:n' },
      { start_ms: 4600, end_ms: 5600, lines: [], scene_id: 's1', block_id: 's1:n' }
    ]
  } });
  eq(rich.report.audio_events, [
    { kind: 'narration', ref: evHash, start_ms: 500, scene_id: 's1', block_id: 's1:n', piece: 0 },
    { kind: 'sfx', ref: 'sfx.pop', start_ms: 1836, scene_id: 's1', take: 2 }
  ], 'malformed events are dropped, unknown fields too, and the message stays');
  eq(rich.report.captions.map(c => c.lines.map(line => line.length)), [[3, 3], [400]], 'at most 2 lines, each clipped at 400 characters');
  eq(rich.report.scenes[0].cues, [{ marker: 1, at_ms: 1840, placed: 'span' }], 'only well-formed cues stay');
  eq(rich.report.cue_precision, 0.5, 'cue_precision');
  const bare = P(valid).report;
  eq([bare.audio_events, bare.captions, bare.cue_precision, bare.scenes[0].cues], [[], [], null, []], 'a report without the new fields');
  // the other side of the wire: a report from a real compile loses nothing (dir is ui/js/desktop/apps, four levels below the repo root)
  const measured = JSON.parse(fs.readFileSync(path.join(dir, '..', '..', '..', '..', 'internal', 'storymaker', 'render', 'testdata', 'report_measured.json'), 'utf8'));
  const wire = P({ sm: 1, type: 'compiled', report: measured }).report;
  eq(wire.audio_events.length, measured.audio_events.length, 'every event of a real compile passes');
  eq(wire.captions.length, (measured.captions || []).length, 'every caption of a real compile passes');
}

// the export size follows the aspect, as render.CanvasSize does (the preview compiles smaller)
```

In `scripts/storymaker-tests/08-host-app.mjs`, replace:

```js
import { t } from '../storymaker-test-context.mjs';
```

with:

```js
import { t } from '../storymaker-test-context.mjs';
import { readJSON as readRepoJSON } from '../storymaker-test-support.mjs';
```

In the same file, replace:

```js
check('host app: seek, play with frame dropping, pause and the end of the video', async () => {
```

with:

```js
check('host app: audio_info and glyph_data stay inside their caps', () => {
    const ok = loadHost().StorymakerHost.checkMessage;
    const load = extra => ({ sm: 1, type: 'load', story: firewall, catalog, ...extra });
    const block = (levels, pieces = 1) => ({ id: 's1:n', kind: 'narration', levels: new Array(levels).fill(0), pieces: new Array(pieces).fill({ hash: 'a', ext: 'wav' }) });
    const audio = narration => load({ audio_info: { narration } });
    const scene = blocks => ({ s1: { blocks } });
    assert.equal(ok(load({ audio_info: {} })), null, 'audio_info without narration passes');
    assert.equal(ok(audio({})), null, 'an empty answer passes');
    assert.match(ok(load({ audio_info: { narration: 5 } })), /audio_info/, 'narration must be an object');
    assert.equal(ok(audio(scene([block(27000)]))), null, 'one block at maxLevels passes');
    assert.match(ok(audio(scene([block(27001)]))), /levels/, 'maxLevels is per block');
    // 7 blocks of 27000 are 189000 levels, inside maxTotalLevels 200000. 8 blocks are 216000, over it, although each block fits.
    assert.equal(ok(audio(scene(new Array(7).fill(block(27000))))), null, 'seven full blocks pass');
    assert.match(ok(audio(scene(new Array(8).fill(block(27000))))), /levels/, 'maxTotalLevels is per message');
    assert.match(ok(audio(scene([block(1, 801)]))), /pieces/, 'maxAudioPieces 800 is per block');
    assert.match(ok(audio(scene(new Array(62).fill(block(1))))), /blocks/, 'maxAudioBlocks 61 is per scene');
    const many = {};
    for (let i = 0; i < 401; i++) many['s' + i] = { blocks: [] };
    assert.match(ok(audio(many)), /scenes/, 'maxAudioScenes is 400');

    const schema = 'storymaker-glyphs/1';
    const glyphs = data => load({ catalog: { ...catalog, glyph_data: data } });
    const face = d => ({ id: 'f', family: 'F', file: '/fonts/f.woff2', weight: '400', d });
    assert.match(ok(glyphs({ faces: [] })), /schema/, 'glyph_data needs the schema string');
    assert.match(ok(glyphs({ schema: 'storymaker-glyphs/2', faces: [] })), /schema/, 'another schema is refused');
    assert.equal(ok(glyphs({ schema, faces: [] })), null, 'the schema string with no faces passes');
    assert.match(ok(glyphs({ schema, faces: new Array(17).fill(face([])) })), /faces/, 'maxGlyphFaces is 16');
    // Each range is two values in d, so 120002 values are 60001 ranges, one over maxGlyphRanges.
    assert.match(ok(glyphs({ schema, faces: [face(new Array(120002).fill(1))] })), /ranges/, 'maxGlyphRanges is 60000');
    assert.equal(ok(glyphs(readRepoJSON('ui/img/storymaker/glyphs.json'))), null, 'the shipped coverage file fits the caps');
});

check('host app: seek, play with frame dropping, pause and the end of the video', async () => {
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^TestStorymakerAppModelInNode$' -count=1
npm run test:storymaker
```

Expected:
- The Go test fails with `malformed events are dropped, unknown fields too, and the message stays: got undefined`, because the whitelist copies no `audio_events`.
- The Node runner prints `FAIL host app: audio_info and glyph_data stay inside their caps`, because `checkMessage` answers null where `assert.match` wants a reason.

- [ ] **Step 4: Implement the parent side**

In `ui/js/desktop/apps/storymaker-player.js`, replace:

```js
    const LIMITS = { scenes: 400, issues: 2000, keys: 2000, keyLength: 240, text: 2000, id: 64, frame: 100000000, thumbWaitMs: 12000 };
```

with:

```js
    const LIMITS = { scenes: 400, issues: 2000, keys: 2000, keyLength: 240, text: 2000, id: 64, frame: 100000000, thumbWaitMs: 12000, events: 40000, captions: 8000, cues: 400, lineChars: 400 };
```

In the same file, replace:

```js
    // checkIncoming(msg) -> a cleaned copy of a valid message, or null. The copy holds only the fields the schema names.
```

with:

```js
    const EVENT_KINDS = ['narration', 'dialogue', 'sfx'];
    const CUE_PLACED = ['span', 'proportion'];

    function isMs(v) { return isInt(v, 0, 1e12); }
    function okSceneId(v) { return typeof v === 'string' && v.length <= LIMITS.id; }
    // A block id is the scene id plus ":n" or ":b<beat>", so 16 more characters than a scene id are enough.
    function okBlockId(v) { return typeof v === 'string' && v.length <= LIMITS.id + 16; }

    // cleanList(raw, max, clean): the entries that pass clean, at most max. A malformed entry is dropped, not the message.
    function cleanList(raw, max, clean) {
        const out = [];
        (Array.isArray(raw) ? raw.slice(0, max) : []).forEach(entry => { const item = clean(entry); if (item) out.push(item); });
        return out;
    }

    function cleanEvent(raw) {
        if (!isObject(raw) || EVENT_KINDS.indexOf(raw.kind) < 0 || typeof raw.ref !== 'string' || !raw.ref || raw.ref.length > 80) return null;
        if (!isMs(raw.start_ms) || !okSceneId(raw.scene_id)) return null;
        const out = { kind: raw.kind, ref: raw.ref, start_ms: raw.start_ms, scene_id: raw.scene_id };
        if (raw.block_id !== undefined) {
            if (!okBlockId(raw.block_id)) return null;
            out.block_id = raw.block_id;
        }
        if (raw.piece !== undefined) {
            if (!isInt(raw.piece, 0, 100000)) return null;
            out.piece = raw.piece;
        }
        if (raw.take !== undefined) {
            if (!isInt(raw.take, 0, 100000)) return null;
            out.take = raw.take;
        }
        return out;
    }

    function cleanCaption(raw) {
        if (!isObject(raw) || !isMs(raw.start_ms) || !isMs(raw.end_ms) || !Array.isArray(raw.lines)) return null;
        if (!okSceneId(raw.scene_id) || !okBlockId(raw.block_id)) return null;
        const lines = raw.lines.slice(0, 2);
        if (!lines.length || !lines.every(line => typeof line === 'string')) return null;
        return { start_ms: raw.start_ms, end_ms: raw.end_ms, lines: lines.map(line => line.slice(0, LIMITS.lineChars)), scene_id: raw.scene_id, block_id: raw.block_id };
    }

    function cleanCue(raw) {
        if (!isObject(raw) || !isInt(raw.marker, 0, 1e9) || !isMs(raw.at_ms) || CUE_PLACED.indexOf(raw.placed) < 0) return null;
        return { marker: raw.marker, at_ms: raw.at_ms, placed: raw.placed };
    }

    // checkIncoming(msg) -> a cleaned copy of a valid message, or null. The copy holds only the fields the schema names.
```

In the same file, replace:

```js
                    scenes.push({ id: s.id, start_frame: s.start_frame, end_frame: s.end_frame, kind: text(s.kind, 16) });
```

with:

```js
                    scenes.push({ id: s.id, start_frame: s.start_frame, end_frame: s.end_frame, kind: text(s.kind, 16), cues: cleanList(s.cues, LIMITS.cues, cleanCue) });
```

In the same file, replace:

```js
                        catalog_version: text(r.catalog_version, 64)
```

with:

```js
                        catalog_version: text(r.catalog_version, 64),
                        audio_events: cleanList(r.audio_events, LIMITS.events, cleanEvent),
                        captions: cleanList(r.captions, LIMITS.captions, cleanCaption),
                        cue_precision: typeof r.cue_precision === 'number' && isFinite(r.cue_precision) ? r.cue_precision : null
```

- [ ] **Step 5: Implement the host side**

Task G7 has already changed `checkMessage` in `ui/js/storymaker/host-app.js` to `checkMessage(msg, loaded)` and added the font checks. The three lines quoted below are ones that task leaves as they are.

G7 added `maxFallbackFontBytes` to `CAPS`. This step adds every other new cap.

Replace this fragment of the `CAPS` literal:

```js
        maxBundles: 400,
```

with:

```js
        maxAudioScenes: 400, maxAudioBlocks: 61, maxAudioPieces: 800, maxLevels: 27000, maxTotalLevels: 200000, maxGlyphFaces: 16, maxGlyphRanges: 60000,
        maxBundles: 400,
```

In the same file, replace:

```js
    function okReq(v) { return (typeof v === 'string' || typeof v === 'number') && String(v).length <= CAPS.maxReq; }
```

with:

```js
    function okReq(v) { return (typeof v === 'string' || typeof v === 'number') && String(v).length <= CAPS.maxReq; }

    // checkAudioInfo(info) = null when audio_info fits its caps, else a short reason. maxAudioBlocks counts per scene (one narration
    // plus 60 beats), maxAudioPieces and maxLevels per block, maxTotalLevels per message.
    function checkAudioInfo(info) {
        if (!isObject(info)) return 'load.audio_info is invalid';
        if (info.narration === undefined) return null;
        if (!isObject(info.narration)) return 'load.audio_info.narration is invalid';
        var ids = Object.keys(info.narration), total = 0, i, j;
        if (ids.length > CAPS.maxAudioScenes) return 'load.audio_info has too many scenes';
        for (i = 0; i < ids.length; i++) {
            var scene = info.narration[ids[i]];
            if (!isObject(scene) || !Array.isArray(scene.blocks)) return 'load.audio_info has a bad scene';
            if (scene.blocks.length > CAPS.maxAudioBlocks) return 'load.audio_info has too many blocks in a scene';
            for (j = 0; j < scene.blocks.length; j++) {
                var block = scene.blocks[j];
                if (!isObject(block)) return 'load.audio_info has a bad block';
                var pieces = block.pieces || [], levels = block.levels || [];
                if (!Array.isArray(pieces) || !Array.isArray(levels)) return 'load.audio_info has a bad block';
                if (pieces.length > CAPS.maxAudioPieces) return 'load.audio_info has too many pieces in a block';
                if (levels.length > CAPS.maxLevels) return 'load.audio_info has too many levels in a block';
                total += levels.length;
                if (total > CAPS.maxTotalLevels) return 'load.audio_info has too many levels';
            }
        }
        return null;
    }

    // checkGlyphData(data) = null when catalog.glyph_data has the schema string and fits its caps, else a short reason.
    // Each range takes two values of d. maxGlyphRanges counts across all faces.
    function checkGlyphData(data) {
        if (!isObject(data) || data.schema !== 'storymaker-glyphs/1') return 'load.catalog.glyph_data needs the schema string';
        if (!Array.isArray(data.faces) || data.faces.length > CAPS.maxGlyphFaces) return 'load.catalog.glyph_data has bad faces or too many faces';
        var ranges = 0;
        for (var i = 0; i < data.faces.length; i++) {
            var face = data.faces[i];
            if (!isObject(face) || !Array.isArray(face.d)) return 'load.catalog.glyph_data has bad faces or too many faces';
            ranges += face.d.length / 2;
            if (ranges > CAPS.maxGlyphRanges) return 'load.catalog.glyph_data has too many ranges';
        }
        return null;
    }
```

In the same file, replace:

```js
                if (msg.audio_info !== undefined && !isObject(msg.audio_info)) return 'load.audio_info is invalid';
```

with:

```js
                if (msg.catalog.glyph_data !== undefined) {
                    var badGlyphs = checkGlyphData(msg.catalog.glyph_data);
                    if (badGlyphs) return badGlyphs;
                }
                if (msg.audio_info !== undefined) {
                    var badAudio = checkAudioInfo(msg.audio_info);
                    if (badAudio) return badAudio;
                }
```

- [ ] **Step 6: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
npm run build:ui
npm run test:storymaker
go test ./ui -run '^(TestStorymakerAppModelInNode|TestStorymakerPlayerBridgeIsSandboxed)$' -count=1
```

Expected:
- `npm run build:ui` rewrites `ui/js/storymaker/host.bundle.js`.
- The Node runner reports no `FAIL` line.
- Both Go tests PASS.

- [ ] **Step 7: Commit**

Run `detect_changes` as outline section 5 says. A PARTIAL answer on new files needs a grep confirmation in the commit body.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-player.js ui/js/storymaker/host-app.js ui/js/storymaker/host.bundle.js ui/js/storymaker/player.bundle.js ui/desktop_storymaker_model_test.go scripts/storymaker-tests/08-host-app.mjs
git commit -m "feat(storymaker): whitelists for audio events, captions, audio_info and glyph_data" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E1c: `storymaker-audio.js`: narration, pure model
**Batch:** 2B  **Depends on:** E1a
**Files:**
- Create: `ui/js/desktop/apps/storymaker-audio.js`
- Modify: `ui/js/desktop/core/module-loader.js`
- Test: `ui/desktop_storymaker_registration_test.go`
- Test: `ui/desktop_storymaker_audio_test.go`

- [ ] **Step 1: Write the failing test**

In `ui/desktop_storymaker_registration_test.go`, replace:

```go
var storymakerModules = []string{"core", "model", "api", "assets", "player", "projects", "strip", "inspector", "code", "export"}
```

with:

```go
var storymakerModules = []string{"core", "model", "api", "assets", "player", "projects", "strip", "inspector", "code", "export", "audio"}
```

Create `ui/desktop_storymaker_audio_test.go`:

```go
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
```

- [ ] **Step 2: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^(TestStorymakerAudioModelInNode|TestDesktopStorymakerRegistration)$' -count=1
```

Expected:
- `TestStorymakerAudioModelInNode` fails with `ENOENT` on `storymaker-audio.js`, because the module does not exist.
- `TestDesktopStorymakerRegistration` fails with `the storymaker loader entry lists /js/desktop/apps/storymaker-audio.js' out of order or not at all`, because the loader has no such entry.

- [ ] **Step 3: Implement**

Create `ui/js/desktop/apps/storymaker-audio.js`:

```js
// Storymaker app preview audio: the parent side of the sound. The player iframe has no audio or connect access, so this module
// plays the narration pieces of the project's /narration answer next to it, on the schedule of report.audio_events. Pieces are
// decoded in a sliding window, 2 s behind to 20 s ahead of the playhead. The owner calls sync(frame) on each time message while
// playing, and a drift above 120 ms reschedules. Audio URLs come only from api.narrationUrl with values of the parent's own
// answer, never from message text. Classic script, publishes S.audioModel and S.createAudio.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const WINDOW_BACK_MS = 2000;
    const WINDOW_AHEAD_MS = 20000;
    const DRIFT_MS = 120;
    const SPOKEN_KINDS = ['narration', 'dialogue'];

    // eachBlock(narration, fn): every block of an answer's narration map, {<scene id>: {blocks}}.
    function eachBlock(narration, fn) {
        Object.keys(narration || {}).forEach(sceneId => {
            const scene = narration[sceneId];
            (scene && Array.isArray(scene.blocks) ? scene.blocks : []).forEach(fn);
        });
    }

    // windowFor(events, ms): the events that start from 2 s behind to 20 s ahead of the playhead.
    function windowFor(events, ms) {
        return (Array.isArray(events) ? events : []).filter(event => event.start_ms >= ms - WINDOW_BACK_MS && event.start_ms <= ms + WINDOW_AHEAD_MS);
    }

    // waveform(report, narration): one level per frame of the film. Block levels sit at the block's frames, everything else is 0.
    // An event's start_ms is the block start plus the piece's at_ms, so any event of a block gives the block start.
    function waveform(report, narration) {
        const r = report || {};
        const count = r.frame_count > 0 ? r.frame_count : 0;
        const fps = r.fps > 0 ? r.fps : 30;
        const wave = new Uint8Array(count);
        const blocks = new Map();
        eachBlock(narration, block => blocks.set(block.id, block));
        const done = new Set();
        (Array.isArray(r.audio_events) ? r.audio_events : []).forEach(event => {
            const block = blocks.get(event.block_id);
            if (!block || done.has(block.id)) return;
            const piece = Array.isArray(block.pieces) ? block.pieces[event.piece] : null;
            if (!piece) return;
            done.add(block.id);
            const first = Math.round((event.start_ms - piece.at_ms) * fps / 1000);
            (Array.isArray(block.levels) ? block.levels : []).forEach((level, k) => {
                const frame = first + k;
                if (frame >= 0 && frame < count && level > wave[frame]) wave[frame] = level;
            });
        });
        return wave;
    }

    // createAudio({api, projectId}) -> {load(info, events), play(frame), pause(), seek(frame), sync(frame), dispose()}.
    // info = {fps, frame_count, narration, music, volume, catalog}, events = report.audio_events. A piece's hash and ext come from
    // info.narration. An event whose ref is not in that answer is ignored.
    function createAudio(options) {
        const o = options || {};
        const api = o.api, projectId = o.projectId;
        const lifetime = new AbortController();
        const buffers = new Map();
        const nodes = new Set();
        let ctx = null, info = {}, pieces = new Map(), events = [], started = new Set();
        let playing = false, disposed = false, origin = 0, generation = 0;

        function msOf(frame) {
            return Math.max(0, frame) * 1000 / (info.fps > 0 ? info.fps : 30);
        }

        function piecesOf(narration) {
            const map = new Map();
            eachBlock(narration, block => (Array.isArray(block.pieces) ? block.pieces : []).forEach(piece => map.set(piece.hash, piece)));
            return map;
        }

        // Own copies with end_ms, the film time where the used part of the piece ends.
        function spokenEvents(list) {
            const out = [];
            (Array.isArray(list) ? list : []).forEach(event => {
                const piece = event && pieces.get(event.ref);
                if (!piece || SPOKEN_KINDS.indexOf(event.kind) < 0) return;
                out.push({ ref: event.ref, start_ms: event.start_ms, end_ms: event.start_ms + piece.out_ms - piece.in_ms });
            });
            return out;
        }

        function stopAll() {
            generation++;
            started = new Set();
            nodes.forEach(node => {
                node.onended = null;
                try { node.stop(); } catch (_) { /* already over */ }
                try { node.disconnect(); } catch (_) { /* already gone */ }
            });
            nodes.clear();
        }

        async function fetchAndDecode(hash) {
            const url = api.narrationUrl(projectId, hash, pieces.get(hash).ext);
            if (!url) throw new Error('refused');
            const response = await window.fetch(url, { credentials: 'same-origin', signal: lifetime.signal });
            if (!response.ok) throw new Error('audio answer ' + response.status);
            return ctx.decodeAudioData(await response.arrayBuffer());
        }

        // A piece that fails stays silent and leaves the cache, so a later schedule may try it again.
        function decode(hash) {
            if (!buffers.has(hash)) buffers.set(hash, fetchAndDecode(hash).catch(() => { buffers.delete(hash); return null; }));
            return buffers.get(hash);
        }

        function startPiece(event, buffer) {
            const piece = pieces.get(event.ref);
            const when = origin + event.start_ms / 1000;
            const now = ctx.currentTime;
            const late = Math.max(0, now - when);
            const length = (piece.out_ms - piece.in_ms) / 1000;
            if (late >= length) return;
            const source = ctx.createBufferSource();
            source.buffer = buffer;
            source.connect(ctx.destination);
            source.start(Math.max(when, now), piece.in_ms / 1000 + late, length - late);
            source.onended = () => nodes.delete(source);
            nodes.add(source);
        }

        // The window of windowFor, plus an older piece that is still sounding, so a seek into a long sentence plays its rest.
        // Buffers outside the result are dropped, which bounds the decoded memory.
        function fill(ms) {
            const wanted = windowFor(events, ms).concat(events.filter(event => event.start_ms < ms - WINDOW_BACK_MS && event.end_ms > ms));
            const keep = new Set(wanted.map(event => event.ref));
            Array.from(buffers.keys()).forEach(hash => { if (!keep.has(hash)) buffers.delete(hash); });
            const mine = generation;
            wanted.forEach(event => {
                if (started.has(event)) return;
                started.add(event);
                decode(event.ref).then(buffer => { if (buffer && playing && mine === generation) startPiece(event, buffer); });
            });
        }

        function reschedule(ms) {
            stopAll();
            origin = ctx.currentTime - ms / 1000;
            fill(ms);
        }

        return {
            load(nextInfo, nextEvents) {
                info = nextInfo || {};
                pieces = piecesOf(info.narration);
                events = spokenEvents(nextEvents);
                if (playing) reschedule((ctx.currentTime - origin) * 1000);
            },
            // Call this from the click that starts playback: the browser allows sound only after a user gesture.
            play(frame) {
                if (disposed) return;
                if (!ctx) {
                    const Context = window.AudioContext || window.webkitAudioContext;
                    if (!Context) return;
                    ctx = new Context();
                }
                if (ctx.state === 'suspended') ctx.resume().catch(() => { /* stays silent */ });
                playing = true;
                reschedule(msOf(frame));
            },
            pause() {
                playing = false;
                stopAll();
            },
            seek(frame) {
                if (playing) reschedule(msOf(frame));
            },
            sync(frame) {
                if (!playing) return;
                const ms = msOf(frame);
                if (Math.abs((ctx.currentTime - origin) * 1000 - ms) > DRIFT_MS) reschedule(ms);
                else fill(ms);
            },
            dispose() {
                if (disposed) return;
                disposed = true;
                playing = false;
                stopAll();
                lifetime.abort();
                buffers.clear();
                if (ctx) ctx.close().catch(() => { /* already closed */ });
                ctx = null;
            }
        };
    }

    S.audioModel = { waveform, windowFor };
    S.createAudio = createAudio;
})();
```

In `ui/js/desktop/core/module-loader.js`, replace:

```js
'/js/desktop/apps/storymaker-export.js', '/js/desktop/apps/storymaker.js']
```

with:

```js
'/js/desktop/apps/storymaker-export.js', '/js/desktop/apps/storymaker-audio.js', '/js/desktop/apps/storymaker.js']
```

- [ ] **Step 4: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^(TestStorymakerAudioModelInNode|TestDesktopStorymakerRegistration|TestStorymakerLoaderListsEveryScript|TestStorymakerSourcesAreSafeAndSmall)$' -count=1
```

Expected: PASS for all four. `TestStorymakerAppLoadsNothingExternalAndAvoidsFilteredClasses` is not affected, because the module has no classes and no addresses.

- [ ] **Step 5: Commit**

Run `detect_changes` as outline section 5 says. A PARTIAL answer on new files needs a grep confirmation in the commit body.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-audio.js ui/js/desktop/core/module-loader.js ui/desktop_storymaker_registration_test.go ui/desktop_storymaker_audio_test.go
git commit -m "feat(storymaker): preview audio with a sliding decode window and drift sync" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E1d: `bindAudio` and player reload
**Batch:** 2B  **Depends on:** E1b, E1c
**Files:**
- Modify: `ui/js/desktop/apps/storymaker-audio.js`
- Modify: `ui/js/desktop/apps/storymaker.js`
- Test: `ui/desktop_storymaker_fixture_test.go`
- Test: `ui/desktop_storymaker_audio_test.go`

- [ ] **Step 1: Impact**

Run this from the main checkout, once each for `reloadPlayer`, `buildEditor`, `afterStorySaved` and `storymakerFixture`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "reloadPlayer" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and treat UNKNOWN as unresolved until a text search confirms it (outline section 5, GitNexus).

- [ ] **Step 2: Write the failing test**

`ui/desktop_storymaker_audio_test.go` is the file task E1c left. The script runs the real `bindAudio` with the real `bindPlayer`, `reloadPlayer`, `schedulePreview`, `afterStorySaved` and `play`, cut out of `storymaker.js`, because the root module needs a whole DOM to load.

Replace:

```go
// The preview audio without a DOM: the decode window, the waveform, and scheduling on a fake AudioContext.
```

with:

```go
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
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^TestStorymakerBindAudioInNode$' -count=1
```

Expected: FAIL with `node check failed` and `storymaker.js lacks inst.parts.audio = S.bindAudio(inst);`, because the root does not bind the audio yet.

- [ ] **Step 4: Implement `bindAudio`**

`ui/js/desktop/apps/storymaker-audio.js` is the file task E1c left. Replace:

```js
// answer, never from message text. Classic script, publishes S.audioModel and S.createAudio.
```

with:

```js
// answer, never from message text. bindAudio joins it to the editor. Classic script, publishes S.audioModel, S.createAudio and
// S.bindAudio.
```

In the same file, replace:

```js
    S.createAudio = createAudio;
```

with:

```js
    const AUDIO_JOB_KINDS = ['narrate', 'music'];
    const JOB_DONE = ['succeeded', 'failed', 'cancelled', 'interrupted'];

    // bindAudio(inst) -> {refresh(), dispose()} joins the preview audio to the editor. It sets inst.audio, inst.audioInfo and
    // inst.refreshAudio, and emits bus event audio on each fresh answer. It asks for the /narration answer on project open, after a
    // successful save (afterStorySaved calls inst.refreshAudio), and once per narrate or music job on its terminal status. When the
    // narration of a fresh answer differs from the one last sent, the player compiles again and the next report feeds
    // createAudio.load. inst.loadSeq inside reloadPlayer makes the newest load win. An older answer, or one for another project,
    // is ignored. A failed request keeps the last answer and sets inst.audioError, and a fresh answer clears it. Only without a last
    // answer does the failure emit audio, because that event also clears the export's refusal and the music start issue.
    function bindAudio(inst) {
        const audio = S.createAudio({ api: inst.api, projectId: inst.projectId });
        const finished = new Set();
        let seq = 0, disposed = false, sent = '{}';

        async function refresh() {
            const projectId = inst.projectId;
            const mine = ++seq;
            let answer;
            try {
                answer = await inst.api.narration(projectId);
            } catch (err) {
                if (disposed || mine !== seq || inst.projectId !== projectId || (err && err.aborted)) return null;
                inst.audioError = err;
                if (!inst.audioInfo) inst.bus.emit('audio');
                return null; // the last answer stays, and the next trigger asks again
            }
            if (disposed || mine !== seq || inst.projectId !== projectId) return null;
            inst.audioInfo = answer;
            inst.audioError = null;
            inst.bus.emit('audio');
            const narration = JSON.stringify(answer && answer.narration ? answer.narration : {});
            if (narration !== sent) {
                sent = narration;
                inst.reloadPlayer();
            }
            return answer;
        }

        const off = [
            inst.bus.on('report', () => {
                const report = inst.report;
                if (!report) return;
                const answer = inst.audioInfo || {};
                // volume stays 1 until the story has music
                audio.load({ fps: report.fps, frame_count: report.frame_count, narration: answer.narration || {}, music: answer.music || null, volume: 1, catalog: inst.catalog }, report.audio_events);
            }),
            inst.bus.on('job', job => {
                if (!job || AUDIO_JOB_KINDS.indexOf(job.kind) < 0 || JOB_DONE.indexOf(job.status) < 0 || finished.has(job.id)) return;
                finished.add(job.id);
                refresh();
            }),
            inst.player.on('loading', () => audio.pause()),
            inst.player.on('time', ({ frame, playing }) => {
                if (playing) audio.sync(frame);
                else audio.pause();
            })
        ];

        inst.audio = audio;
        inst.audioError = null;
        inst.audioInfo = null;
        inst.refreshAudio = refresh;
        refresh();

        return {
            refresh,
            dispose() {
                if (disposed) return;
                disposed = true;
                off.forEach(fn => fn());
                audio.dispose();
                inst.audio = null;
                inst.audioInfo = null;
                inst.audioError = null;
                inst.refreshAudio = null;
            }
        };
    }

    S.createAudio = createAudio;
    S.bindAudio = bindAudio;
```

- [ ] **Step 5: Wire the root**

In `ui/js/desktop/apps/storymaker.js`, replace:

```js
            play: () => { if (inst.player) inst.player.play(); },
```

with:

```js
            play: () => { if (!inst.player || inst.compiling) return; inst.player.play(); if (inst.audio) inst.audio.play(inst.frame); },
```

The call stays in the click path because the browser allows sound only after a user gesture. During a compile the press does nothing: the host drops a play until it has a report again, and `inst.compiling` runs from `loading` to `compiled`, so the sound cannot start without the film.

In the same file, replace:

```js
            showIssues: () => toggleIssues(inst, true)
```

with:

```js
            showIssues: () => toggleIssues(inst, true),
            reloadPlayer: () => reloadPlayer(inst)
```

In the same file, replace:

```js
        bindPlayer(inst, stageNote);
```

with:

```js
        bindPlayer(inst, stageNote);
        inst.parts.audio = S.bindAudio(inst);
```

In the same file, replace:

```js
                story: inst.story, catalog: prepared.catalog, audio_info: {}, assets: prepared.assets, fonts: prepared.fonts,
```

with:

```js
                story: inst.story, catalog: prepared.catalog, audio_info: { narration: inst.audioInfo ? inst.audioInfo.narration : {} }, assets: prepared.assets, fonts: prepared.fonts,
```

In the same file, replace:

```js
        if (inst.parts.code) inst.parts.code.storyChanged();
```

with:

```js
        if (inst.parts.code) inst.parts.code.storyChanged();
        if (inst.refreshAudio) inst.refreshAudio();
```

- [ ] **Step 6: Give the fixture the route**

The app now asks for `/narration` on every open, so the stand-in server must answer it from the real store.

In `ui/desktop_storymaker_fixture_test.go`, replace:

```go
	case rest == "render" && r.Method == http.MethodPost:
```

with:

```go
	case rest == "narration" && r.Method == http.MethodGet:
		f.narration(w, ctx, id)
	case rest == "render" && r.Method == http.MethodPost:
```

In the same file, replace:

```go
func (f *storymakerFixture) render(w http.ResponseWriter, r *http.Request, id string) {
```

with:

```go
// narration answers GET /projects/{id}/narration through the real ProjectStore.AudioAnswer.
func (f *storymakerFixture) narration(w http.ResponseWriter, ctx context.Context, id string) {
	data, _, err := f.projects.LoadStory(ctx, id)
	if err != nil {
		f.notFound(w, err)
		return
	}
	story, _, err := storymaker.DecodeStory(data, f.limits)
	if err != nil {
		f.fail(w, http.StatusUnprocessableEntity, "invalid_story", err.Error(), nil)
		return
	}
	storymaker.ApplyDefaults(&story)
	answer, err := f.projects.AudioAnswer(ctx, id, &story, nil)
	if err != nil {
		f.fail(w, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	f.writeJSON(w, http.StatusOK, answer)
}

func (f *storymakerFixture) render(w http.ResponseWriter, r *http.Request, id string) {
```

- [ ] **Step 7: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^(TestStorymakerBindAudioInNode|TestStorymakerAudioModelInNode|TestStorymakerSourcesAreSafeAndSmall|TestStorymakerLoaderListsEveryScript)$' -count=1
```

Expected: PASS for all four. A compile of the `ui` test package also proves the fixture method.

- [ ] **Step 8: Commit**

Run `detect_changes` as outline section 5 says. A PARTIAL answer on new files needs a grep confirmation in the commit body.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-audio.js ui/js/desktop/apps/storymaker.js ui/desktop_storymaker_fixture_test.go ui/desktop_storymaker_audio_test.go
git commit -m "feat(storymaker): bind preview audio to the editor, reload the player on new narration" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

<!-- G-E1-2 -->

### Task E1e: Voice panel
**Batch:** 2B  **Depends on:** E1d, N14
**Files:**
- Create: `ui/js/desktop/apps/storymaker-voice.js`
- Modify: `ui/js/desktop/apps/storymaker.js`
- Modify: `ui/js/desktop/core/module-loader.js`
- Modify: `ui/css/desktop-app-storymaker.css`
- Test: `ui/desktop_storymaker_registration_test.go`
- Test: `ui/desktop_storymaker_audio_test.go`

- [ ] **Step 1: Impact**

Run this from the main checkout, because `.worktrees/*` is not registered:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildEditor" --direction upstream --repo .
```

Report callers, processes and risk. Handle HIGH, CRITICAL and UNKNOWN by the GitNexus rule in outline section 5: warn on the first two, and confirm UNKNOWN with a text search.

- [ ] **Step 2: Write the failing test**

In `ui/desktop_storymaker_registration_test.go`, on the `storymakerModules` line as E1c leaves it, replace:

```go
"export", "audio"}
```

with:

```go
"export", "audio", "voice"}
```

Append the code below to the end of `ui/desktop_storymaker_audio_test.go`. E1c created that file, and its import block already holds `os`, `os/exec`, `path/filepath`, `strings` and `testing`.

```go
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
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(TestStorymakerVoicePanelInNode|TestDesktopStorymakerRegistration)' -count=1
```

Expected: FAIL.
- `TestDesktopStorymakerRegistration` says the loader entry lists `storymaker-voice.js` out of order or not at all.
- `TestStorymakerVoicePanelInNode` says `storymaker.js lacks "S.createVoicePanel(inst)"`, and Node stops on ENOENT for `storymaker-voice.js`.

- [ ] **Step 4: Implement**

Create `ui/js/desktop/apps/storymaker-voice.js`:

```js
// Storymaker app voice panel: the side tab that gives a story its voice. It lists the providers of GET /voices, preselects the
// active one for a story with voice null, and offers a voice select or a free-text id, a language select limited to the
// provider's languages (a free-text field for a provider with "*"), the "not tested" mark and the facts of the provider's note.
// One button writes {provider, voice, language} into the story. The audition plays the first stored piece of the current
// /narration answer and never synthesises, so no paid call starts here. Server texts are set as text only. Classic script, publishes
// S.createVoicePanel.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const VOICE_ID = /^[A-Za-z0-9._-]{0,128}$/;

    function languageBase(tag) {
        return String(tag || '').trim().toLowerCase().split(/[-_]/)[0];
    }

    function providerId(id) {
        return String(id || '').trim().toLowerCase();
    }

    function desktopLanguage() {
        const lang = window.SYSTEM_LANG || (document.documentElement && document.documentElement.lang) || 'en';
        return String(lang).trim() || 'en';
    }

    function languagesOf(provider) {
        return provider && Array.isArray(provider.languages) ? provider.languages : [];
    }

    // "*" means not restricted, and an empty list restricts nothing either: the field takes free text.
    function languageIsFree(provider) {
        const list = languagesOf(provider);
        return !list.length || list.indexOf('*') >= 0;
    }

    function isTested(provider, language) {
        const tested = provider && Array.isArray(provider.tested_languages) ? provider.tested_languages : [];
        return tested.map(code => languageBase(code)).indexOf(languageBase(language)) >= 0;
    }

    // A Phonon voice that is not installed fails its job, because nothing downloads it on the way (phonon.VoicePath). The panel lists it only while
    // the draft holds it, the rule of the Settings select (ui/cfg/tts.js:421). Settings offers every Piper voice, so that flag decides nothing here.
    function voiceMissing(provider, item) {
        return !!provider && !!item && provider.id === 'phonon' && item.installed === false;
    }

    function voicesFor(provider, language, current) {
        const list = (provider && Array.isArray(provider.voices) ? provider.voices : []).filter(item => !voiceMissing(provider, item) || item.id === current);
        const base = languageBase(language);
        const fitting = list.filter(item => !item.language || languageBase(item.language) === base);
        return fitting.length ? fitting : list;
    }

    // The first piece of the first block, in the story's scene order: the answer's map has no order of its own.
    function firstPiece(story, answer) {
        const narration = answer && answer.narration ? answer.narration : {};
        for (const scene of S.model.scenesOf(story)) {
            const entry = scene ? narration[scene.id] : null;
            for (const block of entry && Array.isArray(entry.blocks) ? entry.blocks : []) {
                if (block && Array.isArray(block.pieces) && block.pieces.length) return block.pieces[0];
            }
        }
        return null;
    }

    // inst: {t, bus, api, story, selected, projectId, readonly, audioInfo, domId(suffix), notify(text, kind), editStory(fn, options)}
    function createVoicePanel(inst) {
        const { el, icon } = S;
        const t = inst.t;
        let providers = null;
        let loadError = '';
        let draft = { provider: '', voice: '', language: '' };
        let savedText = '';
        let replacedLanguage = '';
        let disposed = false;

        function field(labelKey, control, stm) {
            return el('div', { class: 'stm-field', dataset: { stm } },
                el('label', { class: 'stm-label', for: control.id, text: t(labelKey) }), control);
        }

        const providerSelect = el('select', { class: 'stm-select', id: inst.domId('voice-provider'), dataset: { stm: 'voice-provider' }, onChange: () => changeProvider() });
        const voiceSelect = el('select', { class: 'stm-select', id: inst.domId('voice-select'), dataset: { stm: 'voice-select' }, onChange: () => { draft.voice = voiceSelect.value; paintNote(); sync(); } });
        const voiceInput = el('input', { class: 'stm-input', type: 'text', id: inst.domId('voice-free'), maxlength: '128', autocomplete: 'off', spellcheck: 'false', dataset: { stm: 'voice-free' }, onInput: () => { draft.voice = voiceInput.value.trim(); sync(); } });
        const languageSelect = el('select', { class: 'stm-select', id: inst.domId('voice-language'), dataset: { stm: 'voice-language' }, onChange: () => { draft.language = languageSelect.value; replacedLanguage = ''; fitDraft(); fillLanguage(); fillVoice(); sync(); } });
        const languageInput = el('input', { class: 'stm-input', type: 'text', id: inst.domId('voice-language-free'), maxlength: '32', autocomplete: 'off', spellcheck: 'false', dataset: { stm: 'voice-language-free' }, onInput: () => { draft.language = languageInput.value.trim(); fitDraft(); fillVoice(); sync(); } });
        const voiceSelectField = field('storymaker.voice.voice', voiceSelect, 'voice-select-field');
        const voiceInputField = field('storymaker.voice.voice_free', voiceInput, 'voice-free-field');
        const languageSelectField = field('storymaker.voice.language', languageSelect, 'voice-language-field');
        const languageInputField = field('storymaker.voice.language_free', languageInput, 'voice-language-free-field');
        const notTested = el('span', { class: 'stm-badge', hidden: true, dataset: { stm: 'voice-not-tested' }, text: t('storymaker.voice.not_tested') });
        const note = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'voice-note' } });
        const apply = el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'voice-apply' }, onClick: () => writeVoice() }, icon('check'), el('span', { text: t('storymaker.voice.apply') }));
        const audition = el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'voice-audition' }, onClick: () => playSample() }, icon('play'), el('span', { text: t('storymaker.voice.audition') }));
        const player = el('audio', { preload: 'none', dataset: { stm: 'voice-audio' } });
        const form = el('div', { class: 'stm-voice-form', hidden: true, dataset: { stm: 'voice-form' } },
            field('storymaker.voice.provider', providerSelect, 'voice-provider-field'),
            languageSelectField, languageInputField, voiceSelectField, voiceInputField,
            el('div', { class: 'stm-actions' }, notTested),
            note,
            el('div', { class: 'stm-actions' }, apply, audition));
        const none = el('p', { class: 'stm-voice-none', role: 'status', hidden: true, dataset: { stm: 'voice-none' }, text: t('storymaker.voice.none_ready') });
        const errorLine = el('p', { class: 'stm-voice-error', role: 'alert', hidden: true, dataset: { stm: 'voice-error' } });
        const retry = el('button', { type: 'button', class: 'stm-btn', hidden: true, dataset: { stm: 'voice-retry' }, onClick: () => load() }, icon('refresh'), el('span', { text: t('storymaker.retry') }));
        const loading = el('p', { class: 'stm-muted', role: 'status', dataset: { stm: 'voice-loading' }, text: t('storymaker.loading') });
        const root = el('section', { class: 'stm-voice', ariaLabel: t('storymaker.voice.title'), dataset: { stm: 'voice' } }, loading, none, errorLine, retry, form, player);

        function currentProvider() {
            return (providers || []).find(item => item.id === draft.provider) || null;
        }

        // The draft always holds what the controls can show: a language the provider lists, a voice of its list.
        // A language it has to replace stays in replacedLanguage: the note names it, so a German voice never reads Japanese text unannounced.
        function fitDraft() {
            const provider = currentProvider();
            if (!provider) return;
            const listed = languagesOf(provider);
            if (!languageIsFree(provider) && !listed.some(code => languageBase(code) === languageBase(draft.language))) {
                replacedLanguage = draft.language;
                draft.language = listed[0];
            }
            if (provider.free_text) return;
            const voices = voicesFor(provider, draft.language, draft.voice);
            if (!voices.length) { draft.voice = provider.default_voice || ''; return; }
            if (voices.some(item => item.id === draft.voice)) return;
            draft.voice = voices.some(item => item.id === provider.default_voice) ? provider.default_voice : voices[0].id;
        }

        function startDraft() {
            const saved = inst.story && inst.story.voice;
            savedText = JSON.stringify(saved || null);
            replacedLanguage = '';
            if (saved && saved.provider) {
                draft = { provider: providerId(saved.provider), voice: saved.voice || '', language: saved.language || desktopLanguage() };
            } else {
                const list = providers || [];
                const first = list.find(item => item.active && item.ready) || list.find(item => item.ready) || list[0] || null;
                draft = { provider: first ? first.id : '', voice: first && first.default_voice ? first.default_voice : '', language: desktopLanguage() };
            }
            fitDraft();
        }

        function changeProvider() {
            draft.provider = providerSelect.value;
            const provider = currentProvider();
            draft.voice = provider && provider.default_voice ? provider.default_voice : '';
            // The language asked for gets another chance with this provider. fitDraft takes it out again if this one has no voice for it either.
            if (replacedLanguage) { draft.language = replacedLanguage; replacedLanguage = ''; }
            fitDraft();
            fillAll();
            sync();
        }

        function fillProviders() {
            S.clear(providerSelect);
            (providers || []).forEach(item => providerSelect.appendChild(el('option', {
                value: item.id, disabled: !item.ready || null,
                text: item.ready ? item.id : t('storymaker.voice.provider_not_ready', { name: item.id })
            })));
            if (draft.provider && !currentProvider()) providerSelect.appendChild(el('option', { value: draft.provider, text: t('storymaker.voice.provider_not_ready', { name: draft.provider }) }));
            providerSelect.value = draft.provider;
        }

        function fillLanguage() {
            const provider = currentProvider();
            const free = !provider || languageIsFree(provider);
            languageSelectField.hidden = free;
            languageInputField.hidden = !free;
            S.clear(languageSelect);
            if (free) {
                if (languageInput.value !== draft.language) languageInput.value = draft.language;
                return;
            }
            const base = languageBase(draft.language);
            languagesOf(provider).forEach(code => {
                const value = languageBase(code) === base ? draft.language : code;
                languageSelect.appendChild(el('option', { value, text: value }));
            });
            languageSelect.value = draft.language;
        }

        function paintNote() {
            const provider = currentProvider();
            const voice = provider && Array.isArray(provider.voices) ? provider.voices.find(item => item.id === draft.voice) : null;
            const replaced = replacedLanguage ? t('storymaker.voice.language_replaced', { language: replacedLanguage }) : '';
            const facts = [replaced, provider ? provider.note : '', voice ? voice.note : '', voiceMissing(provider, voice) ? t('storymaker.voice.not_installed') : ''].filter(Boolean).join(' ');
            note.textContent = facts;
            note.hidden = !facts;
        }

        function fillVoice() {
            const provider = currentProvider();
            const free = !!(provider && provider.free_text);
            const voices = provider && !free ? voicesFor(provider, draft.language, draft.voice) : [];
            voiceSelectField.hidden = !voices.length;
            voiceInputField.hidden = !free;
            S.clear(voiceSelect);
            voices.forEach(item => {
                const label = item.name ? item.name + ' (' + item.id + ')' : item.id;
                voiceSelect.appendChild(el('option', { value: item.id, text: voiceMissing(provider, item) ? label + ' - ' + t('storymaker.voice.not_installed') : label }));
            });
            if (voices.length) voiceSelect.value = draft.voice;
            if (free) {
                if (voiceInput.value !== draft.voice) voiceInput.value = draft.voice;
                voiceInput.placeholder = provider.default_voice || '';
            }
            paintNote();
        }

        function fillAll() {
            fillProviders();
            fillLanguage();
            fillVoice();
        }

        function sameAsStory() {
            const saved = inst.story && inst.story.voice;
            return !!saved && providerId(saved.provider) === draft.provider && (saved.voice || '') === draft.voice && (saved.language || '') === draft.language;
        }

        // The controls show a voice the story does not hold. Generate and the audition run the stored voice, so both wait until it is written. There is nothing to
        // write with no form on screen, in read-only mode or with a provider that is not ready, so the stored voice stays usable and the server's refusal says why.
        function draftDiffers() {
            const provider = currentProvider();
            return !inst.readonly && !form.hidden && !!provider && !!provider.ready && !sameAsStory();
        }

        function sync() {
            const provider = currentProvider();
            const loaded = providers !== null;
            const anyReady = (providers || []).some(item => item.ready);
            loading.hidden = loaded;
            errorLine.textContent = loadError;
            errorLine.hidden = !loadError;
            retry.hidden = !loadError;
            none.hidden = !loaded || !!loadError || anyReady;
            form.hidden = !anyReady;
            [providerSelect, voiceSelect, voiceInput, languageSelect, languageInput].forEach(control => { control.disabled = inst.readonly; });
            notTested.hidden = !provider || isTested(provider, draft.language);
            const listed = provider && Array.isArray(provider.voices) ? provider.voices.find(item => item.id === draft.voice) : null;
            const valid = !!provider && !!provider.ready && !!draft.language && VOICE_ID.test(draft.voice) && !voiceMissing(provider, listed);
            apply.disabled = inst.readonly || !valid || sameAsStory();
            const piece = firstPiece(inst.story, inst.audioInfo);
            const differs = draftDiffers();
            audition.disabled = !piece || differs;
            audition.title = !piece ? t('storymaker.voice.audition_none') : differs ? t('storymaker.voice.apply_first') : '';
        }

        function writeVoice() {
            if (inst.readonly || apply.disabled) return;
            const voice = { provider: draft.provider, voice: draft.voice, language: draft.language };
            inst.editStory(story => { story.voice = voice; }, { immediate: true });
        }

        function playSample() {
            if (audition.disabled) return;
            const piece = firstPiece(inst.story, inst.audioInfo);
            if (!piece) return;
            const url = inst.api.narrationUrl(inst.projectId, piece.hash, piece.ext);
            if (!url) { inst.notify(t('storymaker.voice.audition_failed'), 'error'); return; }
            player.src = url;
            const playing = player.play();
            if (playing && typeof playing.catch === 'function') playing.catch(() => inst.notify(t('storymaker.voice.audition_failed'), 'error'));
        }

        // A story changed elsewhere (the Code tab, a reload) restarts the draft. Anything else keeps the controls, so typing and an open select survive an autosave.
        function paint() {
            if (disposed || providers === null) return;
            const text = JSON.stringify((inst.story && inst.story.voice) || null);
            if (text !== savedText) { startDraft(); fillAll(); }
            sync();
        }

        async function load() {
            loadError = '';
            providers = null;
            sync();
            try {
                const answer = await inst.api.voices();
                if (disposed) return;
                providers = answer && Array.isArray(answer.providers) ? answer.providers : [];
            } catch (err) {
                if (disposed || (err && err.aborted)) return;
                providers = [];
                loadError = S.errors.describe(t, err);
            }
            startDraft();
            fillAll();
            sync();
        }

        const off = [
            inst.bus.on('story', paint),
            inst.bus.on('audio', paint),
            inst.bus.on('readonly', paint)
        ];
        load();
        return {
            el: root,
            paint,
            dispose() {
                disposed = true;
                off.forEach(fn => fn());
                player.pause();
                player.removeAttribute('src');
            }
        };
    }

    S.createVoicePanel = createVoicePanel;
})();
```

The other files use CRLF on Windows checkouts. Every old text below is one line, so it matches either way. Keep the file's line ending in the new lines.

`ui/js/desktop/apps/storymaker.js`, five edits. E1d left a comma and its `reloadPlayer` property after the first old text, and the old text stops before that comma.

Old:
```js
showIssues: () => toggleIssues(inst, true)
```
New:
```js
editStory: (fn, options) => editStory(inst, fn, options),
            showIssues: () => toggleIssues(inst, true)
```

Old:
```js
        const exporter = S.createExport(inst);
```
New:
```js
        const exporter = S.createExport(inst);
        const voice = S.createVoicePanel(inst);
```

Old:
```js
inst.parts.exporter = exporter;
```
New:
```js
inst.parts.exporter = exporter; inst.parts.voice = voice;
```

Old:
```js
['library', 'storymaker.side_library', 'box']]
```
New:
```js
['library', 'storymaker.side_library', 'box'], ['voice', 'storymaker.side_voice', 'user']]
```

Old:
```js
sideTabs), inspector.el, library.el);
```
New:
```js
sideTabs), inspector.el, library.el, voice.el);
```

One more edit in the same file adds the story-level edit. Old:
```js
    function commit(inst, draft, immediate) {
```
New:
```js
    // An edit of the story itself, not of one scene: the voice, the captions switch.
    function editStory(inst, fn, options) {
        if (inst.readonly || !inst.story) return false;
        const draft = S.model.cloneStory(inst.story);
        fn(draft);
        commit(inst, draft, !!(options && options.immediate));
        return true;
    }

    function commit(inst, draft, immediate) {
```

`ui/js/desktop/core/module-loader.js`. This file is dirty in the main checkout, so edit only the worktree copy. On the storymaker line as E1c leaves it:

Old:
```js
'/js/desktop/apps/storymaker-audio.js', '/js/desktop/apps/storymaker.js']
```
New:
```js
'/js/desktop/apps/storymaker-audio.js', '/js/desktop/apps/storymaker-voice.js', '/js/desktop/apps/storymaker.js']
```

`ui/css/desktop-app-storymaker.css`, two edits. The status and marker styles of Task E1f sit here too, because that task does not touch the stylesheet.

Old:
```css
.stm-app [data-sidetab="scene"] .stm-library, .stm-app [data-sidetab="library"] .stm-inspector { display: none; }
```
New:
```css
.stm-app [data-sidetab="scene"] :is(.stm-library, .stm-voice), .stm-app [data-sidetab="library"] :is(.stm-inspector, .stm-voice), .stm-app [data-sidetab="voice"] :is(.stm-inspector, .stm-library) { display: none; }
```

Old:
```css
/* Issues panel */
```
New:
```css
/* Voice tab, narration status, the "no audio" marker on the stage */
.stm-voice { display: grid; gap: var(--ds-space-3); padding: 0 var(--ds-space-3) var(--ds-space-3); }
.stm-voice .stm-field { margin: 0; }
.stm-voice-form { display: grid; gap: var(--ds-space-3); }
.stm-voice-none { margin: 0; padding: var(--ds-space-2) var(--ds-space-3); border: 1px dashed var(--stm-line); border-radius: var(--ds-radius-sm); }
.stm-voice-error { margin: 0; color: var(--stm-bad-text); }
.stm-voice-status { display: grid; gap: var(--ds-space-2); padding-top: var(--ds-space-3); border-top: 1px solid var(--stm-line); }
.stm-voice-status p { margin: 0; }
.stm-voice-check { display: flex; align-items: center; gap: var(--ds-space-2); cursor: pointer; }
.stm-no-audio { position: absolute; left: var(--ds-space-3); top: var(--ds-space-3); z-index: 2; display: inline-flex; align-items: center; gap: 4px; padding: 2px 8px; border: 1px solid var(--stm-warn); border-radius: var(--ds-radius-full); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); color: var(--stm-warn-text); font-size: 12px; pointer-events: none; }

/* Issues panel */
```

New string keys. E1i owns `ui/lang/storymaker/*.json`, so do not edit those files here. The English texts for E1i:

| Key | English |
| --- | --- |
| `storymaker.side_voice` | Voice |
| `storymaker.voice.title` | Voice and narration |
| `storymaker.voice.provider` | Provider |
| `storymaker.voice.provider_not_ready` | {name} (not ready) |
| `storymaker.voice.voice` | Voice |
| `storymaker.voice.voice_free` | Voice id |
| `storymaker.voice.language` | Language |
| `storymaker.voice.language_free` | Language tag |
| `storymaker.voice.not_tested` | not tested |
| `storymaker.voice.apply` | Use this voice |
| `storymaker.voice.audition` | Play a sample |
| `storymaker.voice.audition_none` | Generate narration first, then play a sample. |
| `storymaker.voice.audition_failed` | The sample could not be played. |
| `storymaker.voice.apply_first` | Choose Use this voice first. Generate and the sample use the story's saved voice. |
| `storymaker.voice.not_installed` | Not installed. Download it under More voices in the Phonon settings. |
| `storymaker.voice.language_replaced` | This provider has no voice for {language}. |
| `storymaker.voice.none_ready` | No text-to-speech provider is ready. Set one up in the configuration to give this film a voice. |

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(TestStorymakerVoicePanelInNode|TestDesktopStorymakerRegistration|TestStorymakerLoaderListsEveryScript|TestStorymakerSourcesAreSafeAndSmall|TestStorymakerAppLoadsNothingExternalAndAvoidsFilteredClasses|TestStorymakerStylesheetUsesOnlyTokens)' -count=1
```

Expected: PASS. `TestStorymakerAppUsesOnlyKeysItHas` stays red on the new keys until E1i, which is the allowed red state of outline section 3.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-voice.js ui/js/desktop/apps/storymaker.js ui/js/desktop/core/module-loader.js ui/css/desktop-app-storymaker.css ui/desktop_storymaker_registration_test.go ui/desktop_storymaker_audio_test.go
git commit -m "feat(storymaker): voice panel with provider, language and audition" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

On a PARTIAL answer for the new file, add a middle `-m` with the grep confirmation, as outline section 5 asks.

### Task E1f: Narration status, Generate, "no audio" marker, job filter, captions toggle
**Batch:** 2B  **Depends on:** E1e
**Files:**
- Modify: `ui/js/desktop/apps/storymaker-voice.js`
- Modify: `ui/js/desktop/apps/storymaker-export.js`
- Modify: `ui/js/desktop/apps/storymaker-model.js`
- Modify: `ui/js/desktop/apps/storymaker-code.js`
- Modify: `ui/js/desktop/apps/storymaker.js`
- Test: `ui/desktop_storymaker_audio_test.go`
- Test: `ui/desktop_storymaker_model_test.go`

- [ ] **Step 1: Impact**

Run these from the main checkout:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createExport" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildEditor" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createVoicePanel" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "visibleIssues" --direction upstream --repo .
```

Report callers, processes and risk. Handle HIGH, CRITICAL and UNKNOWN by the GitNexus rule in outline section 5. `createVoicePanel` is new on this branch, so expect UNKNOWN and confirm it with a text search. `visibleIssues` is called through `S.model`, so confirm its three callers with a text search too: `storymaker.js`, `storymaker-export.js` and `storymaker-code.js`.

- [ ] **Step 2: Write the failing test**

In `ui/desktop_storymaker_model_test.go`, replace this line inside `storymakerModelScript`:

```js
eq(M.visibleIssues([{ code: 'narration_missing' }, { code: 'x' }]).length, 1, 'narration_missing is hidden');
```

with:

```js
eq(M.visibleIssues([{ code: 'narration_missing' }, { code: 'x' }]).length, 2, 'narration_missing is shown');
```

Append the code below to the end of `ui/desktop_storymaker_audio_test.go`. Its import block, which E1c wrote, already holds `os`, `os/exec`, `path/filepath`, `strings` and `testing`.

```go
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
  console.log('storymaker narration status ok');
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(TestStorymakerNarrationStatusInNode|TestStorymakerAppModelInNode)' -count=1
```

Expected: FAIL. Go says `storymaker.js does not put the no-audio marker on the stage`. Node stops on `f('narration-summary')`, because it is null: the panel has no status yet. `TestStorymakerAppModelInNode` stops on `narration_missing is shown: got 1, want 2`.

- [ ] **Step 4: Implement**

The other files use CRLF on Windows checkouts. Every old text below is one line, so it matches either way. Keep the file's line ending in the new lines.

`ui/js/desktop/apps/storymaker-voice.js`, as Task E1e leaves it, six edits.

Old:
```js
// S.createVoicePanel.
```
New:
```js
// S.createVoicePanel. The narration half shows what still needs audio and the progress of the narrate job, starts it with Generate
// (the only paid path, and it runs in the job loop with its caps), stops it with Stop, switches the burned captions and owns the "no audio" marker.
```

Old:
```js
    // inst: {t, bus, api, story, selected, projectId, readonly, audioInfo, domId(suffix), notify(text, kind), editStory(fn, options)}
```
New:
```js
    const ACTIVE = ['queued', 'running'];

    function jobIsActive(job) {
        return !!job && ACTIVE.indexOf(job.status) >= 0;
    }

    // The narration half of the panel. Job events of kind narrate land here; the export popover takes kind render only.
    // voiceDiffers() says the panel shows a voice the story does not hold. Generate runs the stored voice, so it waits until that voice is written.
    function createNarrationStatus(inst, voiceDiffers) {
        const { el, icon } = S;
        const t = inst.t;
        let job = null;
        let starting = false;
        let refusal = '';
        let cancelling = false;

        const summary = el('p', { class: 'stm-hint', dataset: { stm: 'narration-summary' } });
        const meter = el('progress', { class: 'stm-progress', max: '1', value: '0', hidden: true, ariaLabel: t('storymaker.voice.status_title'), dataset: { stm: 'narration-progress' } });
        const pieces = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'narration-pieces' } });
        const jobLine = el('p', { role: 'status', hidden: true, dataset: { stm: 'narration-job' } });
        const jobDetail = el('code', { class: 'stm-cap-issue', hidden: true, dataset: { stm: 'narration-job-detail' } });
        const refusalLine = el('p', { class: 'stm-voice-error', role: 'alert', hidden: true, dataset: { stm: 'narration-refusal' } });
        const voiceHint = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'narration-voice-hint' }, text: t('storymaker.voice.apply_first') });
        const generateLabel = el('span', { text: t('storymaker.voice.generate') });
        const generate = el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'narration-generate' }, onClick: () => start() }, icon('sparkle'), generateLabel);
        const cancelButton = el('button', { type: 'button', class: 'stm-btn', hidden: true, dataset: { stm: 'narration-cancel' }, onClick: () => cancel() }, icon('stop'), el('span', { text: t('storymaker.voice.cancel') }));
        const captions = el('input', { type: 'checkbox', id: inst.domId('captions'), dataset: { stm: 'captions-toggle' }, onChange: () => toggleCaptions() });
        const marker = el('div', { class: 'stm-no-audio', role: 'status', hidden: true, dataset: { stm: 'no-audio' } }, icon('alert'), el('span', { text: t('storymaker.voice.no_audio') }));
        const readRetry = el('button', { type: 'button', class: 'stm-btn', hidden: true, dataset: { stm: 'narration-read-retry' }, onClick: () => inst.refreshAudio() }, el('span', { text: t('storymaker.retry') }));
        const root = el('div', { class: 'stm-voice-status', dataset: { stm: 'narration-status' } },
            el('span', { class: 'stm-label', text: t('storymaker.voice.status_title') }),
            summary, meter, pieces, jobLine, jobDetail, refusalLine, voiceHint,
            el('div', { class: 'stm-actions' }, generate, cancelButton, readRetry),
            el('label', { class: 'stm-voice-check', for: inst.domId('captions') }, captions, el('span', { text: t('storymaker.voice.captions') })));

        function jobText() {
            if (job.status === 'succeeded') return t('storymaker.voice.job_done');
            if (job.status === 'cancelled') return t('storymaker.voice.job_cancelled');
            if (job.status === 'interrupted') return t('storymaker.voice.job_interrupted');
            if (job.status === 'failed') return S.errors.describeJob(t, job.error);
            if (job.status === 'queued') return t('storymaker.voice.job_queued');
            return t('storymaker.voice.job_running');
        }

        function paintJob() {
            const active = jobIsActive(job);
            const total = job && Number(job.pieces_total) > 0 ? Number(job.pieces_total) : 0;
            const done = job ? Number(job.pieces_done) || 0 : 0;
            meter.hidden = !active;
            if (active) meter.value = total ? S.clamp(done / total, 0, 1) : S.clamp(job.progress, 0, 1);
            pieces.hidden = !active || !total;
            if (active && total) pieces.textContent = t('storymaker.voice.pieces', { done, total });
            jobLine.hidden = !job;
            jobLine.textContent = job ? jobText() : '';
            const detail = job && job.status === 'failed' && job.error && job.error.message ? String(job.error.message).slice(0, 600) : '';
            jobDetail.hidden = !detail;
            jobDetail.textContent = detail;
        }

        function paint() {
            const answer = inst.audioInfo;
            const chars = answer && answer.chars ? answer.chars : { total: 0, missing: 0 };
            const missing = answer && Array.isArray(answer.missing) ? answer.missing : [];
            // A failed first read must not look like loading forever: the reason and a way to read again.
            const readFailed = !answer && !!inst.audioError;
            readRetry.hidden = !readFailed;
            if (readFailed) summary.textContent = S.errors.describe(t, inst.audioError);
            else if (!answer) summary.textContent = t('storymaker.loading');
            else if (missing.length) summary.textContent = t('storymaker.voice.missing', { blocks: missing.length, chars: chars.missing, total: chars.total });
            else if (chars.total > 0) summary.textContent = t('storymaker.voice.complete', { total: chars.total });
            else summary.textContent = t('storymaker.voice.no_text');
            paintJob();
            refusalLine.textContent = refusal;
            refusalLine.hidden = !refusal;
            generateLabel.textContent = t(starting ? 'storymaker.voice.generating' : 'storymaker.voice.generate');
            const differs = voiceDiffers();
            voiceHint.hidden = !differs;
            generate.disabled = inst.readonly || starting || jobIsActive(job) || !missing.length || differs;
            cancelButton.hidden = inst.readonly || !jobIsActive(job);
            cancelButton.disabled = cancelling;
            const style = inst.story && inst.story.style;
            captions.checked = !style || style.captions !== false;
            captions.disabled = inst.readonly;
            const scene = S.model.scenesOf(inst.story)[inst.selected];
            marker.hidden = !scene || !missing.some(item => item.scene_id === scene.id);
        }

        // take(next, adopt): without a tracked job only an active one is taken up, because the event stream replays old jobs on every open.
        function take(next, adopt) {
            if (!next || typeof next !== 'object' || next.kind !== 'narrate') return;
            if (next.project_id && inst.projectId && next.project_id !== inst.projectId) return;
            if (job && job.id !== next.id && (jobIsActive(job) || !jobIsActive(next))) return;
            if (!job && !adopt && !jobIsActive(next)) return;
            job = next;
            paint();
        }

        function refusalText(err) {
            const e = S.errors.normalize(err);
            const data = e.data || {};
            const parts = [S.errors.describe(t, e)];
            if (Array.isArray(data.alternatives) && data.alternatives.length) parts.push(t('storymaker.voice.alternatives', { names: data.alternatives.join(', ') }));
            if (e.code === 'budget_exceeded' && data.limit) parts.push(t('storymaker.voice.budget_detail', { limit: data.limit, missing: data.missing }));
            return parts.join(' ');
        }

        async function start() {
            if (starting || inst.readonly || jobIsActive(job) || voiceDiffers()) return;
            starting = true;
            refusal = '';
            paint();
            try {
                if (!(await inst.flushEdits())) return;
                const answer = await inst.api.narrate(inst.projectId);
                if (answer && answer.job) {
                    // An event of this job may already have arrived and be newer than this answer.
                    if (!job || job.id !== answer.job.id) { job = null; take(answer.job, true); }
                } else {
                    job = null;
                    inst.refreshAudio();
                }
            } catch (err) {
                if (err && err.aborted) return;
                refusal = refusalText(err);
            } finally {
                starting = false;
                paint();
            }
        }

        async function cancel() {
            if (inst.readonly || cancelling || !jobIsActive(job)) return;
            cancelling = true;
            paint();
            try {
                take(await inst.api.cancelJob(job.id), false);
            } catch (err) {
                if (err && err.aborted) return;
                inst.notify(S.errors.describe(t, err), 'error');
            } finally {
                cancelling = false;
                paint();
            }
        }

        function toggleCaptions() {
            const on = captions.checked;
            inst.editStory(story => { story.style = Object.assign({}, story.style, { captions: on }); }, { immediate: true });
        }

        const off = [
            inst.bus.on('job', next => take(next, false)),
            inst.bus.on('audio', paint),
            inst.bus.on('story', paint),
            inst.bus.on('selection', paint),
            inst.bus.on('readonly', paint)
        ];
        paint();
        return { el: root, marker, paint, dispose() { off.forEach(fn => fn()); } };
    }

    // inst: {t, bus, api, story, selected, projectId, readonly, audioInfo, domId(suffix), notify(text, kind), editStory(fn, options), flushEdits(), refreshAudio()}
```

Old:
```js
        const root = el('section', { class: 'stm-voice', ariaLabel: t('storymaker.voice.title'), dataset: { stm: 'voice' } }, loading, none, errorLine, retry, form, player);
```
New:
```js
        const status = createNarrationStatus(inst, draftDiffers);
        const root = el('section', { class: 'stm-voice', ariaLabel: t('storymaker.voice.title'), dataset: { stm: 'voice' } }, loading, none, errorLine, retry, form, player, status.el);
```

Old:
```js
            paint,
```
New:
```js
            marker: status.marker,
            paint() { paint(); status.paint(); },
```

Old:
```js
                disposed = true;
```
New:
```js
                disposed = true;
                status.dispose();
```

The end of `sync()` repaints the status, because a changed draft opens or closes Generate. `status` is assigned before the first `sync()` runs, and `status.paint` does not call `sync`, so there is no loop.

Old:
```js
            audition.title = !piece ? t('storymaker.voice.audition_none') : differs ? t('storymaker.voice.apply_first') : '';
```
New:
```js
            audition.title = !piece ? t('storymaker.voice.audition_none') : differs ? t('storymaker.voice.apply_first') : '';
            status.paint();
```

`ui/js/desktop/apps/storymaker-code.js`. Audio that is not generated yet is no mistake in the story text, so the Code tab's problem list leaves `narration_missing` out, and its count stays 0 for a clean story.

Old:

```js
            const issues = S.model.visibleIssues(S.model.mergeIssues(inst.issues, inst.report ? inst.report.issues : []));
            issues.forEach(issue => {
```

New:

```js
            const issues = S.model.visibleIssues(S.model.mergeIssues(inst.issues, inst.report ? inst.report.issues : []));
            issues.filter(issue => issue.code !== 'narration_missing').forEach(issue => {
```

`ui/js/desktop/apps/storymaker-model.js`, two edits. The warning now shows in the Issues panel. Export stays as it is, because only `counts.error` blocks it.

Old:
```js
    // Phase 1 renders are silent, so a scene without narration audio is not worth showing.
```
New:
```js
    // Every issue shows, narration_missing too: this list is the one place that locates a scene without audio.
```

Old:
```js
        return (Array.isArray(list) ? list : []).filter(issue => issue.code !== 'narration_missing');
```
New:
```js
        return Array.isArray(list) ? list : [];
```

`ui/js/desktop/apps/storymaker-export.js`:

Old:
```js
            inst.bus.on('job', next => apply(next, false)),
```
New:
```js
            inst.bus.on('job', next => { if (next && next.kind === 'render') apply(next, false); }),
```

`ui/js/desktop/apps/storymaker.js`, as Task E1e leaves it. The stage is built earlier in the same function:

Old:
```js
        const voice = S.createVoicePanel(inst);
```
New:
```js
        const voice = S.createVoicePanel(inst);
        stage.appendChild(voice.marker);
```

New string keys for E1i. Do not edit `ui/lang/storymaker/*.json` here.

| Key | English |
| --- | --- |
| `storymaker.voice.status_title` | Narration |
| `storymaker.voice.no_text` | This story has no narration to speak. |
| `storymaker.voice.missing` | {blocks} blocks still need audio: {chars} of {total} characters. |
| `storymaker.voice.complete` | All narration has audio ({total} characters). |
| `storymaker.voice.generate` | Generate narration |
| `storymaker.voice.cancel` | Stop |
| `storymaker.voice.generating` | Starting… |
| `storymaker.voice.pieces` | {done} of {total} sentences |
| `storymaker.voice.job_queued` | Waiting in the queue. |
| `storymaker.voice.job_running` | Generating narration… |
| `storymaker.voice.job_done` | Narration is ready. |
| `storymaker.voice.job_cancelled` | Narration was cancelled. Finished sentences stay, and Generate continues with the rest. |
| `storymaker.voice.job_interrupted` | Narration was interrupted by a restart. Finished sentences stay, and Generate continues with the rest. |
| `storymaker.voice.alternatives` | Voices you can use: {names} |
| `storymaker.voice.budget_detail` | The limit is {limit} characters, and this story needs {missing} more. |
| `storymaker.voice.no_audio` | No audio |
| `storymaker.voice.captions` | Burn captions into the film |

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(TestStorymakerNarrationStatusInNode|TestStorymakerVoicePanelInNode|TestStorymakerAppModelInNode|TestStorymakerSourcesAreSafeAndSmall|TestStorymakerAppLoadsNothingExternalAndAvoidsFilteredClasses)' -count=1
```

Expected: PASS. `TestStorymakerAppUsesOnlyKeysItHas` stays red on the new keys until E1i, which is the allowed red state of outline section 3.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-voice.js ui/js/desktop/apps/storymaker-export.js ui/js/desktop/apps/storymaker-model.js ui/js/desktop/apps/storymaker-code.js ui/js/desktop/apps/storymaker.js ui/desktop_storymaker_audio_test.go ui/desktop_storymaker_model_test.go
git commit -m "feat(storymaker): narration status, Generate, no-audio marker and captions toggle" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

On a PARTIAL answer for `storymaker-voice.js`, add a middle `-m` with the grep confirmation, as outline section 5 asks.

### Task E1g: Scrubber waveform
**Batch:** 2B  **Depends on:** E1c
**Files:**
- Modify: `ui/js/desktop/apps/storymaker-strip.js`
- Modify: `ui/css/desktop-app-storymaker.css`
- Test: `ui/desktop_storymaker_css_test.go`
- Test: `ui/desktop_storymaker_audio_test.go`

- [ ] **Step 1: Impact**

Run this from the main checkout:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createTransport" --direction upstream --repo .
```

Report callers, processes and risk. Handle HIGH, CRITICAL and UNKNOWN by the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

Append to the end of `ui/desktop_storymaker_css_test.go`. Its imports already hold `regexp`, `strings` and `testing`.

```go
// The waveform is drawn over the scene blocks, and the pointer must reach the scrubber through it.
func TestStorymakerScrubWaveIgnoresThePointer(t *testing.T) {
	t.Parallel()
	css := readDesktopAssetText(t, "css/desktop-app-storymaker.css")
	rule := regexp.MustCompile(`(?m)^\.stm-scrub-wave \{[^}]*\}`).FindString(css)
	if rule == "" {
		t.Fatal("css/desktop-app-storymaker.css has no .stm-scrub-wave rule")
	}
	if !strings.Contains(rule, "pointer-events: none") {
		t.Errorf(".stm-scrub-wave must have pointer-events: none: %s", rule)
	}
}
```

Append the code below to the end of `ui/desktop_storymaker_audio_test.go`. Its import block, which E1c wrote, already holds `os`, `os/exec`, `path/filepath`, `strings` and `testing`.

```go
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
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(TestStorymakerScrubWaveInNode|TestStorymakerScrubWaveIgnoresThePointer)' -count=1
```

Expected: FAIL.
- `TestStorymakerScrubWaveIgnoresThePointer` says there is no `.stm-scrub-wave` rule.
- `TestStorymakerScrubWaveInNode` passes the waveform table, because E1c's `waveform` already places the levels. It stops at `the wave lies between the blocks and the playhead`, because the scrubber has no canvas yet.

- [ ] **Step 4: Implement**

The other files use CRLF on Windows checkouts. Every old text below is one line, so it matches either way. Keep the file's line ending in the new lines.

`ui/js/desktop/apps/storymaker-audio.js` stays as E1c and E1d left it. E1c's `waveform` already puts block levels at block frames and 0 everywhere else, which is what the table in Step 2 checks, and `TestStorymakerAudioModelInNode` pins that a block can be known by a later piece alone.

`ui/js/desktop/apps/storymaker-strip.js`, seven edits.

Old:
```js
// keys. The transport is play/pause, the time and the scrubber: one block per scene of the compile report, a playhead, pointer
```
New:
```js
// keys. The transport is play/pause, the time and the scrubber: one block per scene of the compile report, the narration waveform, a playhead, pointer
```

Old:
```js
    // inst: {t, bus, report, frame, playing, readonly, play(), pause(), seek(frame)}
```
New:
```js
    // inst: {t, bus, report, audioInfo, frame, playing, readonly, play(), pause(), seek(frame)}
```

Old:
```js
        const head = el('div', { class: 'stm-scrub-head' });
```
New:
```js
        const wave = el('canvas', { class: 'stm-scrub-wave', ariaHidden: 'true', dataset: { stm: 'scrub-wave', drawn: 'false' } });
        const head = el('div', { class: 'stm-scrub-head' });
```

Old:
```js
        }, blocks, head);
```
New:
```js
        }, blocks, wave, head);
```

Old:
```js
        function paintAll() {
```
New:
```js
        // One bar per column, as tall as the loudest frame under it. The colour comes from the stylesheet, so the wave follows the skin.
        function paintWave() {
            const report = inst.report;
            const count = frameCount();
            const narration = inst.audioInfo && inst.audioInfo.narration ? inst.audioInfo.narration : null;
            const width = Math.max(1, Math.round(wave.clientWidth || 300));
            const height = Math.max(1, Math.round(wave.clientHeight || 24));
            if (wave.width !== width) wave.width = width;
            if (wave.height !== height) wave.height = height;
            const ctx = wave.getContext('2d');
            if (!ctx) return;
            ctx.clearRect(0, 0, width, height);
            wave.dataset.drawn = 'false';
            if (!report || !count || !narration) return;
            const levels = S.audioModel.waveform(report, narration);
            ctx.fillStyle = window.getComputedStyle(wave).color;
            for (let x = 0; x < width; x++) {
                const from = Math.floor(x * count / width);
                const to = Math.max(from + 1, Math.floor((x + 1) * count / width));
                let peak = 0;
                for (let frame = from; frame < to && frame < count; frame++) peak = Math.max(peak, levels[frame]);
                if (!peak) continue;
                const bar = Math.max(1, Math.round(peak / 255 * height));
                ctx.fillRect(x, (height - bar) / 2, 1, bar);
                wave.dataset.drawn = 'true';
            }
        }

        function paintAll() {
```

Old:
```js
            paintBlocks();
```
New:
```js
            paintBlocks();
            paintWave();
```

Old:
```js
            inst.bus.on('report', paintAll),
```
New:
```js
            inst.bus.on('report', paintAll),
            inst.bus.on('audio', paintWave),
```

`ui/css/desktop-app-storymaker.css`:

Old:
```css
.stm-scrub-head { position: absolute;
```
New:
```css
.stm-scrub-wave { position: absolute; left: 2px; top: 2px; width: calc(100% - 4px); height: calc(100% - 4px); color: var(--vd-text); opacity: .55; pointer-events: none; }
.stm-scrub-head { position: absolute;
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(TestStorymakerScrubWaveInNode|TestStorymakerScrubWaveIgnoresThePointer|TestStorymakerStylesheetUsesOnlyTokens|TestStorymakerSourcesAreSafeAndSmall)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-strip.js ui/css/desktop-app-storymaker.css ui/desktop_storymaker_css_test.go ui/desktop_storymaker_audio_test.go
git commit -m "feat(storymaker): scrubber waveform from block levels" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

On a PARTIAL answer, add a middle `-m` with the grep confirmation, as outline section 5 asks.

### Task E1h: New project sends the language
**Batch:** 2B  **Depends on:** E1a, N17
**Files:**
- Modify: `ui/js/desktop/apps/storymaker-projects.js`
- Modify: `ui/js/desktop/apps/storymaker.js`
- Test: `ui/desktop_storymaker_model_test.go`

- [ ] **Step 1: Impact**

Run these from the main checkout:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "createProjects" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "showProjects" --direction upstream --repo .
```

Report callers, processes and risk. Handle HIGH, CRITICAL and UNKNOWN by the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

Append to the end of `ui/desktop_storymaker_model_test.go`. Its imports already hold everything this code uses.

```go
// A new project sends the desktop language. When no voice speaks it, the form shows the languages from the error and offers a
// create without a voice, and that second call sends no language.
func TestStorymakerNewProjectLanguageInNode(t *testing.T) {
	t.Parallel()
	if !strings.Contains(readDesktopAssetText(t, "js/desktop/apps/storymaker.js"), "get language() { return String(window.SYSTEM_LANG || document.documentElement.lang || 'en'); }") {
		t.Error("storymaker.js does not hand the desktop language to the project list")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-new-project.js")
	if err := os.WriteFile(script, []byte(storymakerNewProjectScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	out, err := exec.Command(node, script, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker new project ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerNewProjectScript = `
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
  get value() { return this.val !== undefined ? this.val : (this.attrs.value || ''); }
  set value(next) { this.val = String(next); }
  get textContent() { return this.kids.length ? this.kids.map(kid => kid.textContent).join('') : this.ownText; }
  set textContent(next) { this.kids = []; this.ownText = String(next); }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  fire(type) { (this.handlers[type] || []).slice().forEach(fn => fn({ type, target: this, preventDefault() {}, stopPropagation() {} })); }
  click() { if (!this.disabled) this.fire('click'); }
  focus() {}
  querySelectorAll() { return []; }
  find(name) { if (this.dataset.stm === name) return this; for (const kid of this.kids) { const hit = kid.find(name); if (hit) return hit; } return null; }
  findTag(tag) { if (this.tag === tag) return this; for (const kid of this.kids) { const hit = kid.findTag(tag); if (hit) return hit; } return null; }
}
const document = {
  createElement: tag => new FakeNode(tag),
  createElementNS: (ns, tag) => new FakeNode(tag),
  createTextNode: text => { const node = new FakeNode('#text'); node.ownText = String(text); return node; },
  documentElement: { lang: 'en' }
};
const context = { console, setTimeout, clearTimeout, document };
context.globalThis = context; context.window = context;
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-projects.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const flush = () => new Promise(resolve => setImmediate(resolve));
const t = (key, vars) => (vars ? key + ' ' + JSON.stringify(vars) : key);

function open(results) {
  const calls = [];
  const inst = {
    t, readonly: false, language: 'de', domId: suffix => 'stm-test-' + suffix,
    createProject(spec) { calls.push(spec); const next = results.shift(); return next ? Promise.reject(next) : Promise.resolve(); },
    openProject() {}, deleteProject() {}, confirm: async () => false
  };
  const view = S.createProjects(inst);
  view.setProjects([]);
  view.el.find('new-title').value = 'Film';
  return { calls, view, f: name => view.el.find(name) };
}

(async () => {
  const refused = S.errors.fromResponse(422, { error: 'narration_unavailable', code: 'narration_unavailable', message: 'No voice speaks ja.', languages: ['de', 'en'] }, 0);
  const first = open([refused, null]);
  eq(first.f('projects-offer').hidden, true, 'no offer before a refusal');
  first.view.el.findTag('form').fire('submit');
  await flush();
  eq(first.calls, [{ title: 'Film', aspect: '16:9', language: 'de' }], 'the first call sends the desktop language');
  const text = first.f('projects-error').textContent;
  eq(text.includes('storymaker.error.narration_unavailable'), true, 'the refusal has its own sentence');
  eq(text.includes('storymaker.projects.languages {"languages":"de, en"}'), true, 'err.data.languages shows');
  eq(first.f('projects-offer').hidden, false, 'the offer shows');

  first.f('create-without-voice').click();
  await flush();
  eq(first.calls[1], { title: 'Film', aspect: '16:9' }, 'the second call sends no language');
  eq([first.f('projects-offer').hidden, first.f('projects-error').hidden], [true, true], 'a success clears both');

  // Any other failure keeps its plain sentence and offers nothing.
  const plain = open([S.errors.fromResponse(500, { error: 'internal', code: 'internal', message: 'boom' }, 0)]);
  plain.view.el.findTag('form').fire('submit');
  await flush();
  eq(plain.f('projects-error').textContent, 'storymaker.error.generic boom', 'a plain failure');
  eq(plain.f('projects-offer').hidden, true, 'no offer for a plain failure');

  console.log('storymaker new project ok');
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^TestStorymakerNewProjectLanguageInNode$' -count=1
```

Expected: FAIL. Go says `storymaker.js does not hand the desktop language to the project list`. Node stops at `first.f('projects-offer').hidden`, because that element does not exist yet.

- [ ] **Step 4: Implement**

Both files use CRLF on Windows checkouts. Every old text below is one line, so it matches either way. Keep the file's line ending in the new lines.

`ui/js/desktop/apps/storymaker-projects.js`, six edits.

Old:
```js
    // inst: {t, readonly, createProject(spec), openProject(id), deleteProject(project), confirm(title, message)}
```
New:
```js
    // inst: {t, readonly, language, createProject(spec), openProject(id), deleteProject(project), confirm(title, message)}
```

Old:
```js
        const errorLine = el('p', { class: 'stm-form-error', role: 'alert', hidden: true, dataset: { stm: 'projects-error' } });
```
New:
```js
        const errorLine = el('p', { class: 'stm-form-error', role: 'alert', hidden: true, dataset: { stm: 'projects-error' } });
        const offerButton = el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'create-without-voice' }, onClick: () => submit(true) }, el('span', { text: t('storymaker.projects.create_without_voice') }));
        const offer = el('div', { class: 'stm-form-error stm-actions', hidden: true, dataset: { stm: 'projects-offer' } }, offerButton);
```

Old:
```js
            errorLine);
```
New:
```js
            errorLine, offer);
```

Old:
```js
            create.disabled = off;
```
New:
```js
            create.disabled = off;
            offerButton.disabled = off;
```

Old:
```js
        async function submit() {
```
New:
```js
        // No voice speaks the language: say which languages have one and offer a project without a voice. Other errors go to submit.
        async function send(value, withoutVoice) {
            const spec = { title: value, aspect };
            if (!withoutVoice && inst.language) spec.language = inst.language;
            try {
                await inst.createProject(spec);
            } catch (err) {
                const e = S.errors.normalize(err);
                if (e.code !== 'narration_unavailable') throw err;
                const languages = e.data && Array.isArray(e.data.languages) ? e.data.languages : [];
                setMessage([S.errors.describe(t, e), languages.length ? t('storymaker.projects.languages', { languages: languages.join(', ') }) : ''].filter(Boolean).join(' '));
                offer.hidden = false;
            }
        }

        async function submit(withoutVoice) {
```

Old:
```js
            setMessage('');
```
New:
```js
            setMessage('');
            offer.hidden = true;
```

One more edit in the same file. Old:
```js
                await inst.createProject({ title: value, aspect });
```
New:
```js
                await send(value, withoutVoice);
```

The form's `onSubmit` calls `submit()` with no argument, so `withoutVoice` is undefined there and the language goes out.

`ui/js/desktop/apps/storymaker.js`. The value is read the way `module-loader.js` reads it for `/api/i18n`:

Old:
```js
            t: inst.t, domId: inst.domId, get readonly() { return inst.readonly; },
```
New:
```js
            t: inst.t, domId: inst.domId, get readonly() { return inst.readonly; },
            get language() { return String(window.SYSTEM_LANG || document.documentElement.lang || 'en'); },
```

New string keys for E1i. Do not edit `ui/lang/storymaker/*.json` here.

| Key | English |
| --- | --- |
| `storymaker.projects.languages` | Languages with a voice: {languages} |
| `storymaker.projects.create_without_voice` | Create without a voice |

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '(?i)(TestStorymakerNewProjectLanguageInNode|TestStorymakerAppModelInNode|TestStorymakerSourcesAreSafeAndSmall|TestStorymakerAppLoadsNothingExternalAndAvoidsFilteredClasses)' -count=1
```

Expected: PASS. `TestStorymakerAppUsesOnlyKeysItHas` stays red on the new keys until E1i, which is the allowed red state of outline section 3.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-projects.js ui/js/desktop/apps/storymaker.js ui/desktop_storymaker_model_test.go
git commit -m "feat(storymaker): new project sends the desktop language and offers no voice" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

On a PARTIAL answer, add a middle `-m` with the grep confirmation, as outline section 5 asks.

<!-- G-E1-3 -->

### Task E1i: 2B strings
**Batch:** 2B  **Depends on:** E1a, E1b, E1c, E1d, E1e, E1f, E1g, E1h
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
Test: `ui/desktop_storymaker_guards_test.go`
Test: `ui/desktop_storymaker_i18n_test.go` (runs unchanged: no row of the table below equals its English text, so the exact list at lines 81-85 stays)

- [ ] **Step 1: Impact**

Run from the main checkout, once per symbol: `JOB_ERROR_CODES`, `ISSUE_CODES`, `TestStorymakerAppUsesOnlyKeysItHas`.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "JOB_ERROR_CODES" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "ISSUE_CODES" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "TestStorymakerAppUsesOnlyKeysItHas" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and confirm `risk: UNKNOWN` with a text search (outline section 5).

- [ ] **Step 2: Write the failing test**

In `ui/desktop_storymaker_guards_test.go`, make three replacements. The old text is as M15 leaves it.

Old:

```go
	// The list of issue codes is the contract's: twenty-seven codes, no more, no less.
```

New:

```go
	// The list of issue codes is the contract's: twenty-eight codes, no more, no less.
```

Old:

```go
transition_variety cue_late")
```

New:

```go
transition_variety cue_late glyph_missing")
```

Old:

```go
		t.Errorf("ISSUE_CODES = %v, want the 27 codes of the contract", families["storymaker.issue."])
	}
```

New:

```go
		t.Errorf("ISSUE_CODES = %v, want the 28 codes of the contract", families["storymaker.issue."])
	}
	// A narrate job ends with these codes, so the app needs a sentence for each. Batch 2C adds more, so this is no exact list.
	jobCodes := " " + strings.Join(families["storymaker.job_error."], " ") + " "
	for _, code := range strings.Fields("narration_failed narration_unavailable voice_required budget_exceeded") {
		if !strings.Contains(jobCodes, " "+code+" ") {
			t.Errorf("JOB_ERROR_CODES = %v lacks %s", families["storymaker.job_error."], code)
		}
	}
```

The count 28 is the 19 codes of phase 1, the 8 of 2A and `glyph_missing`.

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^TestStorymakerAppUsesOnlyKeysItHas$' -count=1
```

Expected: FAIL.
- `ISSUE_CODES = [...], want the 28 codes of the contract`, because `storymaker-core.js` still has 27.
- `JOB_ERROR_CODES = [...] lacks narration_failed`, and the same line for the other three codes.
- The allowed red state of this batch: `uses storymaker.voice.<name>, which lang/storymaker/en.json lacks`, for the keys of E1a-E1h.

- [ ] **Step 4: Implement**

In `ui/js/desktop/apps/storymaker-core.js`:

Old:

```js
    // The job error codes of the spec (Error handling) that a phase 1 render can end with.
    const JOB_ERROR_CODES = ['invalid_story', 'storymaker_disabled', 'browser_unavailable', 'ffmpeg_unavailable', 'render_failed', 'render_timeout', 'cancelled', 'busy', 'code_error'];
```

New:

```js
    // The job error codes of the spec (Error handling) that a render or a narrate job can end with.
    const JOB_ERROR_CODES = ['invalid_story', 'storymaker_disabled', 'browser_unavailable', 'ffmpeg_unavailable', 'render_failed', 'render_timeout', 'cancelled', 'busy', 'code_error', 'narration_failed', 'narration_unavailable', 'voice_required', 'budget_exceeded'];
```

Old (the tail of `ISSUE_CODES` as M15 leaves it):

```js
'transition_variety', 'cue_late'];
```

New:

```js
'transition_variety', 'cue_late', 'glyph_missing'];
```

Write this script to the session scratchpad as `e1i-locales.mjs`. It holds the strings, because they contain non-ASCII text and belong in one reviewed table. It refuses to write when a key used by a module has no row, and when a row is used by no module, so the table stays exactly the keys of E1a, E1e, E1f and E1h and the five keys the app builds from its code lists.

```js
// E1i: writes the 2B app strings into ui/lang/storymaker/*.json. Run from the worktree root.
import fs from 'node:fs';
import path from 'node:path';

const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const DIR = 'ui/lang/storymaker';
const APP = 'ui/js/desktop/apps';
// The app builds these keys from ISSUE_CODES and JOB_ERROR_CODES, so no module names them as a literal.
const BUILT = new Set([
  'storymaker.issue.glyph_missing',
  'storymaker.job_error.narration_failed',
  'storymaker.job_error.narration_unavailable',
  'storymaker.job_error.voice_required',
  'storymaker.job_error.budget_exceeded'
]);

const T = {
  'storymaker.issue.glyph_missing': {
    en: "Character not available", cs: "Znak není k dispozici", da: "Tegnet er ikke tilgængeligt", de: "Zeichen nicht verfügbar",
    el: "Ο χαρακτήρας δεν είναι διαθέσιμος", es: "Carácter no disponible", fr: "Caractère indisponible", hi: "अक्षर उपलब्ध नहीं है",
    it: "Carattere non disponibile", ja: "使用できない文字", nl: "Teken niet beschikbaar", no: "Tegnet er ikke tilgjengelig",
    pl: "Znak niedostępny", pt: "Carácter indisponível", sv: "Tecknet är inte tillgängligt", zh: "字符不可用"
  },
  'storymaker.job_error.narration_failed': {
    en: "The narration could not be generated.", cs: "Komentář se nepodařilo vytvořit.", da: "Lyden til fortællerteksten kunne ikke oprettes.", de: "Die Sprachausgabe konnte nicht erzeugt werden.",
    el: "Η αφήγηση δεν ήταν δυνατό να δημιουργηθεί.", es: "No se pudo generar la narración.", fr: "Le commentaire n'a pas pu être généré.", hi: "वर्णन नहीं बनाया जा सका।",
    it: "Non è stato possibile generare la narrazione.", ja: "ナレーションを生成できませんでした。", nl: "De voice-over kon niet worden gemaakt.", no: "Lyden til fortellerteksten kunne ikke lages.",
    pl: "Nie udało się wygenerować głosu lektora.", pt: "Não foi possível gerar a narração.", sv: "Berättarrösten kunde inte skapas.", zh: "无法生成旁白。"
  },
  'storymaker.job_error.narration_unavailable': {
    en: "The voice provider is not available.", cs: "Poskytovatel hlasu není k dispozici.", da: "Stemmeudbyderen er ikke tilgængelig.", de: "Der Stimmenanbieter ist nicht verfügbar.",
    el: "Ο πάροχος φωνής δεν είναι διαθέσιμος.", es: "El proveedor de voz no está disponible.", fr: "Le fournisseur de voix n'est pas disponible.", hi: "आवाज़ प्रदाता उपलब्ध नहीं है।",
    it: "Il fornitore della voce non è disponibile.", ja: "音声プロバイダーを利用できません。", nl: "De stemaanbieder is niet beschikbaar.", no: "Stemmeleverandøren er ikke tilgjengelig.",
    pl: "Dostawca głosu jest niedostępny.", pt: "O fornecedor de voz não está disponível.", sv: "Röstleverantören är inte tillgänglig.", zh: "语音服务不可用。"
  },
  'storymaker.job_error.voice_required': {
    en: "Choose a voice first.", cs: "Nejdřív vyber hlas.", da: "Vælg først en stemme.", de: "Wähle zuerst eine Stimme.",
    el: "Επιλέξτε πρώτα μια φωνή.", es: "Elige primero una voz.", fr: "Choisis d'abord une voix.", hi: "पहले कोई आवाज़ चुनें।",
    it: "Scegli prima una voce.", ja: "先に音声を選択してください。", nl: "Kies eerst een stem.", no: "Velg en stemme først.",
    pl: "Najpierw wybierz głos.", pt: "Escolhe primeiro uma voz.", sv: "Välj en röst först.", zh: "请先选择语音。"
  },
  'storymaker.job_error.budget_exceeded': {
    en: "The narration character limit has been reached.", cs: "Bylo dosaženo limitu znaků pro komentář.", da: "Tegngrænsen for fortællerteksten er nået.", de: "Das Zeichenlimit für die Sprachausgabe ist erreicht.",
    el: "Συμπληρώθηκε το όριο χαρακτήρων της αφήγησης.", es: "Se alcanzó el límite de caracteres de la narración.", fr: "La limite de caractères du commentaire est atteinte.", hi: "वर्णन के अक्षरों की सीमा पूरी हो गई है।",
    it: "È stato raggiunto il limite di caratteri della narrazione.", ja: "ナレーションの文字数の上限に達しました。", nl: "De tekenlimiet voor de voice-over is bereikt.", no: "Tegngrensen for fortellerteksten er nådd.",
    pl: "Osiągnięto limit znaków lektora.", pt: "O limite de caracteres da narração foi atingido.", sv: "Teckengränsen för berättarrösten är nådd.", zh: "已达到旁白字符数上限。"
  },
  'storymaker.error.narration_unavailable': {
    en: "No voice provider is available for this.", cs: "Pro tuto akci není k dispozici žádný poskytovatel hlasu.", da: "Ingen stemmeudbyder er tilgængelig til dette.", de: "Dafür ist kein Stimmenanbieter verfügbar.",
    el: "Δεν υπάρχει διαθέσιμος πάροχος φωνής για αυτό.", es: "No hay ningún proveedor de voz disponible para esto.", fr: "Aucun fournisseur de voix n'est disponible pour cela.", hi: "इसके लिए कोई आवाज़ प्रदाता उपलब्ध नहीं है।",
    it: "Nessun fornitore di voci disponibile per questa operazione.", ja: "これに使える音声プロバイダーがありません。", nl: "Hiervoor is geen stemaanbieder beschikbaar.", no: "Ingen stemmeleverandør er tilgjengelig for dette.",
    pl: "Brak dostawcy głosu dla tej operacji.", pt: "Nenhum fornecedor de voz está disponível para isto.", sv: "Ingen röstleverantör är tillgänglig för detta.", zh: "没有可用的语音服务。"
  },
  'storymaker.error.voice_unknown': {
    en: "The provider does not offer this voice. Choose another one.", cs: "Poskytovatel tento hlas nenabízí. Vyber jiný.", da: "Udbyderen tilbyder ikke denne stemme. Vælg en anden.", de: "Der Anbieter bietet diese Stimme nicht an. Wähle eine andere.",
    el: "Ο πάροχος δεν προσφέρει αυτή τη φωνή. Επιλέξτε άλλη.", es: "El proveedor no ofrece esta voz. Elige otra.", fr: "Le fournisseur ne propose pas cette voix. Choisis-en une autre.", hi: "प्रदाता यह आवाज़ नहीं देता। कोई दूसरी चुनें।",
    it: "Il fornitore non offre questa voce. Scegline un'altra.", ja: "プロバイダーはこの音声を提供していません。別の音声を選択してください。", nl: "De aanbieder biedt deze stem niet aan. Kies een andere.", no: "Leverandøren tilbyr ikke denne stemmen. Velg en annen.",
    pl: "Dostawca nie oferuje tego głosu. Wybierz inny.", pt: "O fornecedor não oferece esta voz. Escolhe outra.", sv: "Leverantören erbjuder inte den här rösten. Välj en annan.", zh: "该服务不提供此语音。请选择其他语音。"
  },
  'storymaker.side_voice': {
    en: "Voice", cs: "Hlas", da: "Stemme", de: "Stimme",
    el: "Φωνή", es: "Voz", fr: "Voix", hi: "आवाज़",
    it: "Voce", ja: "音声", nl: "Stem", no: "Stemme",
    pl: "Głos", pt: "Voz", sv: "Röst", zh: "语音"
  },
  'storymaker.voice.title': {
    en: "Voice and narration", cs: "Hlas a komentář", da: "Stemme og fortællertekst", de: "Stimme und Sprechtext",
    el: "Φωνή και αφήγηση", es: "Voz y narración", fr: "Voix et commentaire", hi: "आवाज़ और वर्णन",
    it: "Voce e narrazione", ja: "音声とナレーション", nl: "Stem en voice-over", no: "Stemme og fortellertekst",
    pl: "Głos i tekst lektora", pt: "Voz e narração", sv: "Röst och berättarröst", zh: "语音与旁白"
  },
  'storymaker.voice.provider': {
    en: "Provider", cs: "Poskytovatel", da: "Udbyder", de: "Anbieter",
    el: "Πάροχος", es: "Proveedor", fr: "Fournisseur", hi: "प्रदाता",
    it: "Fornitore", ja: "プロバイダー", nl: "Aanbieder", no: "Leverandør",
    pl: "Dostawca", pt: "Fornecedor", sv: "Leverantör", zh: "服务商"
  },
  'storymaker.voice.provider_not_ready': {
    en: "{name} (not ready)", cs: "{name} (není připraven)", da: "{name} (ikke klar)", de: "{name} (nicht bereit)",
    el: "{name} (δεν είναι έτοιμος)", es: "{name} (no está listo)", fr: "{name} (pas prêt)", hi: "{name} (तैयार नहीं)",
    it: "{name} (non pronto)", ja: "{name} (準備未完了)", nl: "{name} (niet gereed)", no: "{name} (ikke klar)",
    pl: "{name} (niegotowy)", pt: "{name} (não está pronto)", sv: "{name} (inte redo)", zh: "{name} (未就绪)"
  },
  'storymaker.voice.voice': {
    en: "Voice", cs: "Hlas", da: "Stemme", de: "Stimme",
    el: "Φωνή", es: "Voz", fr: "Voix", hi: "आवाज़",
    it: "Voce", ja: "音声", nl: "Stem", no: "Stemme",
    pl: "Głos", pt: "Voz", sv: "Röst", zh: "语音"
  },
  'storymaker.voice.voice_free': {
    en: "Voice id", cs: "ID hlasu", da: "Stemme-id", de: "Stimmen-ID",
    el: "Αναγνωριστικό φωνής", es: "ID de voz", fr: "ID de la voix", hi: "आवाज़ की आईडी",
    it: "ID della voce", ja: "音声 ID", nl: "Stem-ID", no: "Stemme-ID",
    pl: "ID głosu", pt: "ID da voz", sv: "Röst-id", zh: "语音 ID"
  },
  'storymaker.voice.language': {
    en: "Language", cs: "Jazyk", da: "Sprog", de: "Sprache",
    el: "Γλώσσα", es: "Idioma", fr: "Langue", hi: "भाषा",
    it: "Lingua", ja: "言語", nl: "Taal", no: "Språk",
    pl: "Język", pt: "Idioma", sv: "Språk", zh: "语言"
  },
  'storymaker.voice.language_free': {
    en: "Language tag", cs: "Značka jazyka", da: "Sprogkode", de: "Sprachkürzel",
    el: "Κωδικός γλώσσας", es: "Código de idioma", fr: "Code de langue", hi: "भाषा कोड",
    it: "Codice lingua", ja: "言語タグ", nl: "Taalcode", no: "Språkkode",
    pl: "Kod języka", pt: "Código de idioma", sv: "Språkkod", zh: "语言代码"
  },
  'storymaker.voice.language_replaced': {
    en: "This provider has no voice for {language}.", cs: "Tento poskytovatel nemá žádný hlas pro jazyk {language}.", da: "Denne udbyder har ingen stemme til sproget {language}.", de: "Dieser Anbieter hat keine Stimme für die Sprache {language}.",
    el: "Αυτός ο πάροχος δεν έχει φωνή για τη γλώσσα {language}.", es: "Este proveedor no tiene ninguna voz para el idioma {language}.", fr: "Ce fournisseur n'a aucune voix pour la langue {language}.", hi: "इस प्रदाता के पास {language} भाषा के लिए कोई आवाज़ नहीं है।",
    it: "Questo fornitore non ha alcuna voce per la lingua {language}.", ja: "このプロバイダーには言語 {language} の音声がありません。", nl: "Deze aanbieder heeft geen stem voor de taal {language}.", no: "Denne leverandøren har ingen stemme for språket {language}.",
    pl: "Ten dostawca nie ma głosu dla języka {language}.", pt: "Este fornecedor não tem nenhuma voz para o idioma {language}.", sv: "Den här leverantören har ingen röst för språket {language}.", zh: "此服务商没有语言 {language} 的语音。"
  },
  'storymaker.voice.apply': {
    en: "Use this voice", cs: "Použít tento hlas", da: "Brug denne stemme", de: "Diese Stimme verwenden",
    el: "Χρήση αυτής της φωνής", es: "Usar esta voz", fr: "Utiliser cette voix", hi: "यह आवाज़ इस्तेमाल करें",
    it: "Usa questa voce", ja: "この音声を使う", nl: "Deze stem gebruiken", no: "Bruk denne stemmen",
    pl: "Użyj tego głosu", pt: "Usar esta voz", sv: "Använd den här rösten", zh: "使用此语音"
  },
  'storymaker.voice.audition': {
    en: "Play a sample", cs: "Přehrát ukázku", da: "Afspil en prøve", de: "Hörprobe abspielen",
    el: "Αναπαραγωγή δείγματος", es: "Reproducir una muestra", fr: "Écouter un extrait", hi: "नमूना चलाएँ",
    it: "Riproduci un campione", ja: "サンプルを再生", nl: "Voorbeeld afspelen", no: "Spill av et eksempel",
    pl: "Odtwórz próbkę", pt: "Reproduzir uma amostra", sv: "Spela upp ett prov", zh: "播放样本"
  },
  'storymaker.voice.audition_none': {
    en: "Generate narration first, then play a sample.", cs: "Nejdřív vytvoř komentář, pak přehraj ukázku.", da: "Opret først lyden til fortællerteksten, og afspil derefter en prøve.", de: "Erzeuge zuerst die Sprachausgabe, dann spiele eine Hörprobe ab.",
    el: "Δημιουργήστε πρώτα την αφήγηση και μετά αναπαραγάγετε ένα δείγμα.", es: "Genera primero la narración y luego reproduce una muestra.", fr: "Génère d'abord le commentaire, puis écoute un extrait.", hi: "पहले वर्णन बनाएँ, फिर नमूना चलाएँ।",
    it: "Genera prima la narrazione, poi riproduci un campione.", ja: "先にナレーションを生成してから、サンプルを再生してください。", nl: "Maak eerst de voice-over en speel daarna een voorbeeld af.", no: "Lag lyden til fortellerteksten først, og spill deretter av et eksempel.",
    pl: "Najpierw wygeneruj głos lektora, a potem odtwórz próbkę.", pt: "Gera primeiro a narração e depois reproduz uma amostra.", sv: "Skapa berättarrösten först och spela sedan upp ett prov.", zh: "请先生成旁白，然后播放样本。"
  },
  'storymaker.voice.audition_failed': {
    en: "The sample could not be played.", cs: "Ukázku se nepodařilo přehrát.", da: "Prøven kunne ikke afspilles.", de: "Die Hörprobe konnte nicht abgespielt werden.",
    el: "Δεν ήταν δυνατή η αναπαραγωγή του δείγματος.", es: "No se pudo reproducir la muestra.", fr: "L'extrait n'a pas pu être lu.", hi: "नमूना नहीं चलाया जा सका।",
    it: "Non è stato possibile riprodurre il campione.", ja: "サンプルを再生できませんでした。", nl: "Het voorbeeld kon niet worden afgespeeld.", no: "Eksemplet kunne ikke spilles av.",
    pl: "Nie udało się odtworzyć próbki.", pt: "Não foi possível reproduzir a amostra.", sv: "Provet kunde inte spelas upp.", zh: "无法播放样本。"
  },
  'storymaker.voice.not_tested': {
    en: "not tested", cs: "netestováno", da: "ikke testet", de: "nicht getestet",
    el: "δεν έχει δοκιμαστεί", es: "sin probar", fr: "non testé", hi: "परीक्षण नहीं हुआ",
    it: "non testato", ja: "未検証", nl: "niet getest", no: "ikke testet",
    pl: "nieprzetestowane", pt: "não testado", sv: "inte testad", zh: "未测试"
  },
  'storymaker.voice.apply_first': {
    en: "Choose Use this voice first. Generate and the sample use the story's saved voice.", cs: "Nejdřív zvol „Použít tento hlas“. Vytvoření komentáře i ukázka používají hlas uložený v příběhu.", da: "Vælg først »Brug denne stemme«. Oprettelsen af lyd og prøven bruger historiens gemte stemme.", de: "Wähle zuerst „Diese Stimme verwenden“. Die Erzeugung und die Hörprobe nutzen die gespeicherte Stimme der Geschichte.",
    el: "Επιλέξτε πρώτα «Χρήση αυτής της φωνής». Η δημιουργία και το δείγμα χρησιμοποιούν την αποθηκευμένη φωνή της ιστορίας.", es: "Elige primero «Usar esta voz». La generación y la muestra usan la voz guardada de la historia.", fr: "Choisis d'abord « Utiliser cette voix ». La génération et l'extrait utilisent la voix enregistrée de l'histoire.", hi: "पहले “यह आवाज़ इस्तेमाल करें” चुनें। वर्णन बनाने और नमूने में कहानी की सहेजी गई आवाज़ का उपयोग होता है।",
    it: "Scegli prima «Usa questa voce». La generazione e il campione usano la voce salvata della storia.", ja: "先に「この音声を使う」を選択してください。生成とサンプルには、ストーリーに保存された音声が使われます。", nl: "Kies eerst 'Deze stem gebruiken'. Het maken en het voorbeeld gebruiken de opgeslagen stem van het verhaal.", no: "Velg først «Bruk denne stemmen». Genereringen og eksemplet bruker den lagrede stemmen til historien.",
    pl: "Najpierw wybierz „Użyj tego głosu”. Generowanie i próbka korzystają z głosu zapisanego w historii.", pt: "Escolhe primeiro «Usar esta voz». A geração e a amostra usam a voz guardada da história.", sv: "Välj först ”Använd den här rösten”. Skapandet och provet använder berättelsens sparade röst.", zh: "请先选择“使用此语音”。生成和样本都使用故事中保存的语音。"
  },
  'storymaker.voice.not_installed': {
    en: "Not installed. Download it under More voices in the Phonon settings.", cs: "Není nainstalováno. Stáhni ho v nastavení Phonon v části Další hlasy.", da: "Ikke installeret. Download den under Flere stemmer i Phonon-indstillingerne.", de: "Nicht installiert. Lade sie in den Phonon-Einstellungen unter Weitere Stimmen herunter.",
    el: "Δεν έχει εγκατασταθεί. Κατεβάστε την στις Περισσότερες φωνές, στις ρυθμίσεις του Phonon.", es: "No instalada. Descárgala en Más voces, en los ajustes de Phonon.", fr: "Non installée. Télécharge-la sous Autres voix dans les réglages de Phonon.", hi: "इंस्टॉल नहीं है। इसे Phonon सेटिंग में और आवाज़ें के अंतर्गत डाउनलोड करें।",
    it: "Non installata. Scaricala in Altre voci nelle impostazioni di Phonon.", ja: "インストールされていません。Phonon の設定の「追加の音声」からダウンロードしてください。", nl: "Niet geïnstalleerd. Download hem onder Meer stemmen in de Phonon-instellingen.", no: "Ikke installert. Last den ned under Flere stemmer i Phonon-innstillingene.",
    pl: "Nie zainstalowano. Pobierz go w sekcji Więcej głosów w ustawieniach Phonon.", pt: "Não instalada. Transfere-a em Mais vozes, nas definições do Phonon.", sv: "Inte installerad. Ladda ner den under Fler röster i Phonon-inställningarna.", zh: "未安装。请在 Phonon 设置的“更多语音”中下载。"
  },
  'storymaker.voice.none_ready': {
    en: "No text-to-speech provider is ready. Set one up in the configuration to give this film a voice.", cs: "Žádný poskytovatel převodu textu na řeč není připraven. Nastav některého v konfiguraci, aby tento film dostal hlas.", da: "Ingen udbyder af tekst til tale er klar. Konfigurer en i indstillingerne for at give denne film en stemme.", de: "Kein Anbieter für Sprachausgabe ist bereit. Richte in der Konfiguration einen ein, um diesem Film eine Stimme zu geben.",
    el: "Κανένας πάροχος μετατροπής κειμένου σε ομιλία δεν είναι έτοιμος. Ρυθμίστε έναν στις ρυθμίσεις για να δώσετε φωνή σε αυτή την ταινία.", es: "Ningún proveedor de texto a voz está listo. Configura uno en la configuración para dar voz a esta película.", fr: "Aucun fournisseur de synthèse vocale n'est prêt. Configure-en un dans la configuration pour donner une voix à ce film.", hi: "कोई टेक्स्ट-टू-स्पीच प्रदाता तैयार नहीं है। इस फ़िल्म को आवाज़ देने के लिए कॉन्फ़िगरेशन में एक सेट करें।",
    it: "Nessun fornitore di sintesi vocale è pronto. Configurane uno nella configurazione per dare una voce a questo film.", ja: "音声合成プロバイダーの準備ができていません。この動画に音声を付けるには、設定でプロバイダーを設定してください。", nl: "Er is geen tekst-naar-spraakaanbieder gereed. Stel er een in via de configuratie om deze film een stem te geven.", no: "Ingen leverandør av tekst-til-tale er klar. Sett opp en i konfigurasjonen for å gi denne filmen en stemme.",
    pl: "Żaden dostawca syntezy mowy nie jest gotowy. Skonfiguruj go w konfiguracji, aby nadać temu filmowi głos.", pt: "Nenhum fornecedor de texto para voz está pronto. Configura um na configuração para dar voz a este filme.", sv: "Ingen leverantör av text-till-tal är redo. Ställ in en i konfigurationen för att ge den här filmen en röst.", zh: "没有可用的文字转语音服务。请在配置中设置一个，为这部影片配音。"
  },
  'storymaker.voice.captions': {
    en: "Burn captions into the film", cs: "Vypálit titulky do filmu", da: "Brænd undertekster ind i filmen", de: "Untertitel in den Film einbrennen",
    el: "Ενσωμάτωση υποτίτλων στην ταινία", es: "Grabar los subtítulos en la película", fr: "Incruster les sous-titres dans le film", hi: "कैप्शन को फ़िल्म में जोड़ें",
    it: "Imprimi i sottotitoli nel film", ja: "字幕を動画に焼き込む", nl: "Ondertitels in de film branden", no: "Brenn inn undertekster i filmen",
    pl: "Wtop napisy w film", pt: "Gravar as legendas no filme", sv: "Bränn in undertexter i filmen", zh: "将字幕烧录到影片中"
  },
  'storymaker.voice.status_title': {
    en: "Narration", cs: "Komentář", da: "Fortællertekst", de: "Sprechtext",
    el: "Αφήγηση", es: "Narración", fr: "Commentaire", hi: "वर्णन",
    it: "Narrazione", ja: "ナレーション", nl: "Voice-over", no: "Fortellertekst",
    pl: "Narracja", pt: "Narração", sv: "Berättarröst", zh: "旁白"
  },
  'storymaker.voice.no_text': {
    en: "This story has no narration to speak.", cs: "Tento příběh nemá žádný komentář k namluvení.", da: "Denne historie har ingen fortællertekst, der skal læses op.", de: "Diese Geschichte hat keinen Sprechtext.",
    el: "Αυτή η ιστορία δεν έχει αφήγηση για εκφώνηση.", es: "Esta historia no tiene narración que leer.", fr: "Cette histoire n'a aucun commentaire à dire.", hi: "इस कहानी में बोलने के लिए कोई वर्णन नहीं है।",
    it: "Questa storia non ha narrazione da pronunciare.", ja: "このストーリーには読み上げるナレーションがありません。", nl: "Dit verhaal heeft geen voice-overtekst.", no: "Denne historien har ingen fortellertekst å lese opp.",
    pl: "Ta historia nie ma tekstu lektora do odczytania.", pt: "Esta história não tem narração para dizer.", sv: "Den här berättelsen har ingen berättartext att läsa upp.", zh: "此故事没有需要朗读的旁白。"
  },
  'storymaker.voice.missing': {
    en: "{blocks} blocks still need audio: {chars} of {total} characters.", cs: "Zvuk ještě potřebují bloky: {blocks} ({chars} z {total} znaků).", da: "{blocks} blokke mangler stadig lyd: {chars} af {total} tegn.", de: "{blocks} Blöcke brauchen noch Audio: {chars} von {total} Zeichen.",
    el: "{blocks} τμήματα χρειάζονται ακόμη ήχο: {chars} από {total} χαρακτήρες.", es: "{blocks} bloques aún necesitan audio: {chars} de {total} caracteres.", fr: "{blocks} blocs ont encore besoin d'audio : {chars} caractères sur {total}.", hi: "{blocks} खंडों को अभी ऑडियो चाहिए: {total} में से {chars} अक्षर।",
    it: "{blocks} blocchi devono ancora avere l'audio: {chars} caratteri su {total}.", ja: "{blocks} 個のブロックにまだ音声が必要です: {total} 文字中 {chars} 文字。", nl: "{blocks} blokken hebben nog audio nodig: {chars} van {total} tekens.", no: "{blocks} blokker mangler fortsatt lyd: {chars} av {total} tegn.",
    pl: "Bloki wymagające jeszcze dźwięku: {blocks} ({chars} z {total} znaków).", pt: "{blocks} blocos ainda precisam de áudio: {chars} de {total} caracteres.", sv: "{blocks} block behöver fortfarande ljud: {chars} av {total} tecken.", zh: "还有 {blocks} 个片段需要音频：{total} 个字符中的 {chars} 个。"
  },
  'storymaker.voice.complete': {
    en: "All narration has audio ({total} characters).", cs: "Celý komentář má zvuk (znaků: {total}).", da: "Hele fortællerteksten har lyd ({total} tegn).", de: "Der gesamte Sprechtext hat Audio ({total} Zeichen).",
    el: "Όλη η αφήγηση έχει ήχο ({total} χαρακτήρες).", es: "Toda la narración tiene audio ({total} caracteres).", fr: "Tout le commentaire a de l'audio ({total} caractères).", hi: "पूरे वर्णन की ऑडियो मौजूद है ({total} अक्षर)।",
    it: "Tutta la narrazione ha l'audio ({total} caratteri).", ja: "すべてのナレーションに音声があります（{total} 文字）。", nl: "De hele voice-over heeft audio ({total} tekens).", no: "Hele fortellerteksten har lyd ({total} tegn).",
    pl: "Cały tekst lektora ma dźwięk (znaków: {total}).", pt: "Toda a narração tem áudio ({total} caracteres).", sv: "Hela berättarrösten har ljud ({total} tecken).", zh: "所有旁白都有音频（共 {total} 个字符）。"
  },
  'storymaker.voice.generate': {
    en: "Generate narration", cs: "Vytvořit komentář", da: "Opret lyd til fortællerteksten", de: "Sprachausgabe erzeugen",
    el: "Δημιουργία αφήγησης", es: "Generar narración", fr: "Générer le commentaire", hi: "वर्णन बनाएँ",
    it: "Genera narrazione", ja: "ナレーションを生成", nl: "Voice-over maken", no: "Lag lyd til fortellerteksten",
    pl: "Wygeneruj głos lektora", pt: "Gerar narração", sv: "Skapa berättarröst", zh: "生成旁白"
  },
  'storymaker.voice.generating': {
    en: "Starting…", cs: "Spouštím…", da: "Starter…", de: "Wird gestartet…",
    el: "Έναρξη…", es: "Iniciando…", fr: "Démarrage…", hi: "शुरू हो रहा है…",
    it: "Avvio…", ja: "開始しています…", nl: "Starten…", no: "Starter…",
    pl: "Uruchamianie…", pt: "A iniciar…", sv: "Startar…", zh: "正在启动…"
  },
  'storymaker.voice.pieces': {
    en: "{done} of {total} sentences", cs: "Věty: {done} z {total}", da: "{done} af {total} sætninger", de: "{done} von {total} Sätzen",
    el: "{done} από {total} προτάσεις", es: "{done} de {total} frases", fr: "{done} phrases sur {total}", hi: "{total} में से {done} वाक्य",
    it: "{done} frasi su {total}", ja: "{total} 文中 {done} 文", nl: "{done} van {total} zinnen", no: "{done} av {total} setninger",
    pl: "Zdania: {done} z {total}", pt: "{done} de {total} frases", sv: "{done} av {total} meningar", zh: "第 {done} 句，共 {total} 句"
  },
  'storymaker.voice.no_audio': {
    en: "No audio", cs: "Bez zvuku", da: "Ingen lyd", de: "Kein Audio",
    el: "Χωρίς ήχο", es: "Sin audio", fr: "Pas d'audio", hi: "ऑडियो नहीं",
    it: "Nessun audio", ja: "音声なし", nl: "Geen audio", no: "Ingen lyd",
    pl: "Brak dźwięku", pt: "Sem áudio", sv: "Inget ljud", zh: "没有音频"
  },
  'storymaker.voice.job_queued': {
    en: "Waiting in the queue.", cs: "Čeká ve frontě.", da: "Venter i køen.", de: "Wartet in der Warteschlange.",
    el: "Σε αναμονή στην ουρά.", es: "Esperando en la cola.", fr: "En attente dans la file.", hi: "कतार में प्रतीक्षा कर रहा है।",
    it: "In attesa nella coda.", ja: "キューで待機中です。", nl: "Wacht in de wachtrij.", no: "Venter i køen.",
    pl: "Oczekuje w kolejce.", pt: "A aguardar na fila.", sv: "Väntar i kön.", zh: "正在排队等候。"
  },
  'storymaker.voice.job_running': {
    en: "Generating narration…", cs: "Vytvářím komentář…", da: "Opretter lyd til fortællerteksten…", de: "Sprachausgabe wird erzeugt…",
    el: "Δημιουργία αφήγησης…", es: "Generando la narración…", fr: "Génération du commentaire…", hi: "वर्णन बनाया जा रहा है…",
    it: "Generazione della narrazione…", ja: "ナレーションを生成しています…", nl: "Voice-over wordt gemaakt…", no: "Lager lyd til fortellerteksten…",
    pl: "Generowanie głosu lektora…", pt: "A gerar a narração…", sv: "Skapar berättarröst…", zh: "正在生成旁白…"
  },
  'storymaker.voice.job_done': {
    en: "Narration is ready.", cs: "Komentář je hotový.", da: "Lyden til fortællerteksten er klar.", de: "Die Sprachausgabe ist fertig.",
    el: "Η αφήγηση είναι έτοιμη.", es: "La narración está lista.", fr: "Le commentaire est prêt.", hi: "वर्णन तैयार है।",
    it: "La narrazione è pronta.", ja: "ナレーションの準備ができました。", nl: "De voice-over is klaar.", no: "Lyden til fortellerteksten er klar.",
    pl: "Głos lektora jest gotowy.", pt: "A narração está pronta.", sv: "Berättarrösten är klar.", zh: "旁白已就绪。"
  },
  'storymaker.voice.job_cancelled': {
    en: "Narration was cancelled. Finished sentences stay, and Generate continues with the rest.", cs: "Komentář byl zrušen. Hotové věty zůstanou a vytváření naváže na zbytek.", da: "Lyden til fortællerteksten blev annulleret. Færdige sætninger bevares, og oprettelsen fortsætter med resten.", de: "Die Sprachausgabe wurde abgebrochen. Fertige Sätze bleiben erhalten, und die Erzeugung macht mit dem Rest weiter.",
    el: "Η αφήγηση ακυρώθηκε. Οι έτοιμες προτάσεις παραμένουν και η δημιουργία συνεχίζει με τις υπόλοιπες.", es: "La narración se canceló. Las frases terminadas se conservan y la generación continúa con el resto.", fr: "Le commentaire a été annulé. Les phrases terminées sont conservées et la génération reprend avec le reste.", hi: "वर्णन रद्द कर दिया गया। पूरे हो चुके वाक्य बने रहते हैं, और निर्माण बाकी वाक्यों के साथ जारी रहता है।",
    it: "La narrazione è stata annullata. Le frasi finite restano e la generazione continua con il resto.", ja: "ナレーションをキャンセルしました。完成した文は残り、生成は残りの文から続行します。", nl: "De voice-over is geannuleerd. Klare zinnen blijven bewaard en het maken gaat verder met de rest.", no: "Lyden til fortellerteksten ble avbrutt. Ferdige setninger beholdes, og genereringen fortsetter med resten.",
    pl: "Głos lektora anulowano. Gotowe zdania zostają, a generowanie obejmie resztę.", pt: "A narração foi cancelada. As frases terminadas ficam, e a geração continua com o resto.", sv: "Berättarrösten avbröts. Färdiga meningar finns kvar, och skapandet fortsätter med resten.", zh: "旁白已取消。已完成的句子会保留，生成会从其余句子继续。"
  },
  'storymaker.voice.job_interrupted': {
    en: "Narration was interrupted by a restart. Finished sentences stay, and Generate continues with the rest.", cs: "Komentář přerušil restart. Hotové věty zůstanou a vytváření naváže na zbytek.", da: "Lyden til fortællerteksten blev afbrudt af en genstart. Færdige sætninger bevares, og oprettelsen fortsætter med resten.", de: "Die Sprachausgabe wurde durch einen Neustart unterbrochen. Fertige Sätze bleiben erhalten, und die Erzeugung macht mit dem Rest weiter.",
    el: "Η αφήγηση διακόπηκε από επανεκκίνηση. Οι έτοιμες προτάσεις παραμένουν και η δημιουργία συνεχίζει με τις υπόλοιπες.", es: "Un reinicio interrumpió la narración. Las frases terminadas se conservan y la generación continúa con el resto.", fr: "Le commentaire a été interrompu par un redémarrage. Les phrases terminées sont conservées et la génération reprend avec le reste.", hi: "पुनः आरंभ से वर्णन बाधित हुआ। पूरे हो चुके वाक्य बने रहते हैं, और निर्माण बाकी वाक्यों के साथ जारी रहता है।",
    it: "La narrazione è stata interrotta da un riavvio. Le frasi finite restano e la generazione continua con il resto.", ja: "再起動によりナレーションが中断されました。完成した文は残り、生成は残りの文から続行します。", nl: "De voice-over is onderbroken door een herstart. Klare zinnen blijven bewaard en het maken gaat verder met de rest.", no: "Lyden til fortellerteksten ble avbrutt av en omstart. Ferdige setninger beholdes, og genereringen fortsetter med resten.",
    pl: "Głos lektora przerwał restart. Gotowe zdania zostają, a generowanie obejmie resztę.", pt: "Um reinício interrompeu a narração. As frases terminadas ficam, e a geração continua com o resto.", sv: "Berättarrösten avbröts av en omstart. Färdiga meningar finns kvar, och skapandet fortsätter med resten.", zh: "旁白因重启而中断。已完成的句子会保留，生成会从其余句子继续。"
  },
  'storymaker.voice.alternatives': {
    en: "Voices you can use: {names}", cs: "Hlasy, které můžeš použít: {names}", da: "Stemmer, du kan bruge: {names}", de: "Stimmen, die du verwenden kannst: {names}",
    el: "Φωνές που μπορείτε να χρησιμοποιήσετε: {names}", es: "Voces que puedes usar: {names}", fr: "Voix que tu peux utiliser : {names}", hi: "जिन आवाज़ों का आप उपयोग कर सकते हैं: {names}",
    it: "Voci che puoi usare: {names}", ja: "使える音声: {names}", nl: "Stemmen die je kunt gebruiken: {names}", no: "Stemmer du kan bruke: {names}",
    pl: "Głosy, których możesz użyć: {names}", pt: "Vozes que podes usar: {names}", sv: "Röster du kan använda: {names}", zh: "可用的语音：{names}"
  },
  'storymaker.voice.budget_detail': {
    en: "The limit is {limit} characters, and this story needs {missing} more.", cs: "Limit je {limit} znaků a tento příběh potřebuje ještě {missing}.", da: "Grænsen er {limit} tegn, og denne historie mangler {missing} mere.", de: "Das Limit liegt bei {limit} Zeichen, und diese Geschichte braucht noch {missing}.",
    el: "Το όριο είναι {limit} χαρακτήρες και αυτή η ιστορία χρειάζεται {missing} ακόμη.", es: "El límite es de {limit} caracteres y esta historia necesita {missing} más.", fr: "La limite est de {limit} caractères et cette histoire en demande encore {missing}.", hi: "सीमा {limit} अक्षर है, और इस कहानी को {missing} और चाहिए।",
    it: "Il limite è di {limit} caratteri e questa storia ne richiede altri {missing}.", ja: "上限は {limit} 文字で、このストーリーにはさらに {missing} 文字必要です。", nl: "De limiet is {limit} tekens en dit verhaal heeft er nog {missing} nodig.", no: "Grensen er {limit} tegn, og denne historien trenger {missing} til.",
    pl: "Limit wynosi {limit} znaków, a ta historia potrzebuje jeszcze {missing}.", pt: "O limite é de {limit} caracteres e esta história precisa de mais {missing}.", sv: "Gränsen är {limit} tecken, och den här berättelsen behöver {missing} till.", zh: "上限为 {limit} 个字符，此故事还需要 {missing} 个。"
  },
  'storymaker.voice.cancel': {
    en: "Stop", cs: "Zastavit", da: "Stands", de: "Stoppen",
    el: "Διακοπή", es: "Detener", fr: "Arrêter", hi: "रोकें",
    it: "Interrompi", ja: "停止", nl: "Stoppen", no: "Stopp",
    pl: "Zatrzymaj", pt: "Parar", sv: "Stoppa", zh: "停止"
  },
  'storymaker.projects.languages': {
    en: "Languages with a voice: {languages}", cs: "Jazyky s hlasem: {languages}", da: "Sprog med stemme: {languages}", de: "Sprachen mit Stimme: {languages}",
    el: "Γλώσσες με φωνή: {languages}", es: "Idiomas con voz: {languages}", fr: "Langues avec une voix : {languages}", hi: "आवाज़ वाली भाषाएँ: {languages}",
    it: "Lingue con una voce: {languages}", ja: "音声のある言語: {languages}", nl: "Talen met een stem: {languages}", no: "Språk med stemme: {languages}",
    pl: "Języki z głosem: {languages}", pt: "Idiomas com voz: {languages}", sv: "Språk med röst: {languages}", zh: "有语音的语言：{languages}"
  },
  'storymaker.projects.create_without_voice': {
    en: "Create without a voice", cs: "Vytvořit bez hlasu", da: "Opret uden stemme", de: "Ohne Stimme erstellen",
    el: "Δημιουργία χωρίς φωνή", es: "Crear sin voz", fr: "Créer sans voix", hi: "बिना आवाज़ के बनाएँ",
    it: "Crea senza voce", ja: "音声なしで作成", nl: "Zonder stem maken", no: "Opprett uten stemme",
    pl: "Utwórz bez głosu", pt: "Criar sem voz", sv: "Skapa utan röst", zh: "不使用语音创建"
  }
};

const placeholders = text => (text.match(/\{\{?\w+\}\}?/g) || []).sort().join(' ');
for (const [key, row] of Object.entries(T)) {
  for (const lang of LANGS) {
    if (typeof row[lang] !== 'string' || !row[lang].trim()) throw new Error(key + ' has no ' + lang);
    if (placeholders(row[lang]) !== placeholders(row.en)) throw new Error(key + ' ' + lang + ' has other placeholders');
    if (lang !== 'en' && row[lang] === row.en) throw new Error(key + ' ' + lang + ' keeps the English text');
  }
}

const english = JSON.parse(fs.readFileSync(path.join(DIR, 'en.json'), 'utf8'));
const used = new Set();
for (const name of fs.readdirSync(APP).filter(n => /^storymaker.*\.js$/.test(n))) {
  const source = fs.readFileSync(path.join(APP, name), 'utf8');
  for (const match of source.matchAll(/'(storymaker\.[A-Za-z0-9_.]+)'/g)) {
    const key = match[1];
    if (key.endsWith('.') || key.endsWith('_') || key.startsWith('storymaker.catalog.')) continue;
    used.add(key);
  }
}
const missing = [...used].filter(key => !(key in english) && !(key in T));
if (missing.length) {
  console.error('These keys are used by the modules and have no row:\n' + missing.join('\n'));
  process.exit(1);
}
const unused = Object.keys(T).filter(key => !used.has(key) && !BUILT.has(key));
if (unused.length) {
  console.error('These rows are used by no module:\n' + unused.join('\n'));
  process.exit(1);
}
const write = Object.keys(T);

function anchorOf(key) {
  if (key.startsWith('storymaker.side_')) return { text: '"storymaker.side_library":', before: false };
  if (key.startsWith('storymaker.issue.')) return { text: '"storymaker.issue.unknown":', before: true };
  if (key.startsWith('storymaker.job_error.')) return { text: '"storymaker.job_error.unknown":', before: true };
  if (key.startsWith('storymaker.error.')) return { text: '"storymaker.error.generic":', before: true };
  if (key.startsWith('storymaker.projects.')) return { text: '"storymaker.projects.deleted":', before: false };
  if (key.startsWith('storymaker.voice.')) return { text: '"storymaker.job_error.invalid_story":', before: true, block: true };
  throw new Error('no place for ' + key);
}

for (const lang of LANGS) {
  const file = path.join(DIR, lang + '.json');
  const lines = fs.readFileSync(file, 'utf8').split('\n');
  const groups = new Map();
  for (const key of write) {
    if (lines.some(line => line.trimStart().startsWith(JSON.stringify(key) + ':'))) continue;
    const anchor = anchorOf(key);
    if (!groups.has(anchor.text)) groups.set(anchor.text, { anchor, added: [] });
    groups.get(anchor.text).added.push('  ' + JSON.stringify(key) + ': ' + JSON.stringify(T[key][lang]) + ',');
  }
  for (const { anchor, added } of groups.values()) {
    const at = lines.findIndex(line => line.trimStart().startsWith(anchor.text));
    if (at < 0) throw new Error(file + ' has no line ' + anchor.text);
    if (anchor.block) added.push('');
    lines.splice(anchor.before ? at : at + 1, 0, ...added);
  }
  const text = lines.join('\n');
  const parsed = JSON.parse(text);
  for (const key of write) if (parsed[key] !== T[key][lang]) throw new Error(file + ' has another text for ' + key);
  fs.writeFileSync(file, text);
}
console.log('wrote ' + write.length + ' keys in ' + LANGS.length + ' locales');
```

Run it from the worktree root:

```powershell
node "<session scratchpad>/e1i-locales.mjs"
```

Expected: `wrote 43 keys in 16 locales`. These are the 7 rows of the issue, the four job errors and the two API errors, the 33 keys of E1e and E1f, `storymaker.voice.cancel`, and the 2 keys of E1h.

If it stops with a list of keys, a module and the table disagree on a name. Make the row match the module's key, with all 16 languages, then run it again. No row equals its English text in any language, so the `same` map at lines 81-85 of `ui/desktop_storymaker_i18n_test.go` stays exact and unchanged.

- [ ] **Step 5: Run**

```powershell
go test ./ui -run '(?i)(TestStorymakerAppUsesOnlyKeysItHas|TestStorymakerLocalesCoverAllSixteenLanguages|TestStorymakerAppTranslationsAreTranslated)' -count=1 -v
go test ./ui -run Storymaker -count=1 -timeout 20m
```

Expected: the first line shows 3 PASS lines. The second line is green, so the batch's allowed i18n red state ends here. Gated browser tests skip.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-core.js ui/desktop_storymaker_guards_test.go ui/lang/storymaker/cs.json ui/lang/storymaker/da.json ui/lang/storymaker/de.json ui/lang/storymaker/el.json ui/lang/storymaker/en.json ui/lang/storymaker/es.json ui/lang/storymaker/fr.json ui/lang/storymaker/hi.json ui/lang/storymaker/it.json ui/lang/storymaker/ja.json ui/lang/storymaker/nl.json ui/lang/storymaker/no.json ui/lang/storymaker/pl.json ui/lang/storymaker/pt.json ui/lang/storymaker/sv.json ui/lang/storymaker/zh.json
git commit -m "feat(storymaker): strings for voice, narration and glyph errors in 16 locales" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

A PARTIAL answer from `detect-changes` needs a grep confirmation stated in the commit body (outline section 5).

### Task E1j: Contract slice, child docs, cue share
**Batch:** 2B  **Depends on:** E1i
**Files:**
Modify: `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Modify: `internal/storymaker/AGENTS.md`
Modify: `internal/storymaker/render/AGENTS.md`
Modify: `ui/js/storymaker/AGENTS.md`
Modify: `ui/js/desktop/apps/AGENTS.md`
Modify: `internal/server/AGENTS.md`
Modify: `documentation/storymaker.md`
Modify: `docs/superpowers/plans/2026-11-04-storymaker-phase2/measured.md`

Docs only: no failing test. The check is a path and text check. docs-agent does this task.

- [ ] **Step 1: Edit contract sections 2, 3, 3a and 3b**

The file is `docs/superpowers/specs/2026-11-01-storymaker-contract.md`. M16 changed some of these lines first, so each edit matches a line by its start, not by its number. The line start is unique in the file. Replace the whole line unless the edit says otherwise.

C1. Line starting ``- `voice` and `music` may be `null`.``

```text
- `voice` and `music` may be `null`. `POST /narrate` synthesises; validation still checks only the shape of `voice` and of the provider id.
```

C2. Line starting `| Beats the player plays |`

```text
| Beats the player plays | enter, exit, walk, move, pose, gesture, emote, look, say, show (with `item` or `text` only), hide, highlight, camera, wait |
```

C3. Line starting `| Beats accepted and ignored |`

```text
| Beats accepted and ignored | sfx, chart, and show with `icon` or `chart`. Each gets a minor `not_supported_yet` issue. |
```

C4. Delete the line starting ``| `speaker` other than the narrator |``.

C5. Line starting `| Narration audio |`

```text
| Narration audio | From `/narrate`; until the gate landed a narrated story exports silently; a scene without audio gets `narration_missing`. |
```

C6. Line starting ``| `limit_exceeded` | error |``

```text
| `limit_exceeded` | error | Go Validate: scenes, items, beats, narration, say text, scene duration, summed durations, chart entries |
```

C6a. Line starting ``| `invalid_value` | error or warning |``

```text
| `invalid_value` | error or warning | Go Validate: enums, ranges, slugs, positions, asset kind, voice and music shape, the voice provider `"active"`. Warning: a code scene with actors, props or beats (ignored); `with: "prev"` on the first beat; a variant the environment lacks. Player: a duration shorter than the speech, narration or a say block (warning). |
```

C7. After the line starting ``| `layout_unverified` | minor |``, insert:

```text
| `glyph_missing` | error | Go Validate: a character no face covers, at the text path |
```

C8. After the line starting ``| `internal/storymaker/testdata/scenes/s3.bundle.js` |``, insert:

```text
| `internal/storymaker/testdata/narration/cross_language.json` | rows `{input, plain, cues, chars_total, width, words, pieces}`, read by a Go test and a Node test |
| `internal/storymaker/testdata/stories/speech.json` | a story with CJK, RTL, Greek and Hindi text and bubbles |
```

C9. Line starting `catalog.json  {schema:`

```text
catalog.json  {schema:"storymaker-catalog/1", version, stage, rig, glyphs, fonts, palettes, entries}
```

C10. Replace these two lines:

```text
version       12 hex characters: sha256 over the LF-normalised bytes of every file below ui/img/storymaker/
              except catalog.json, in sorted path order. Current value: ab33e88a6b77.
```

with:

```text
version       12 hex characters: sha256 over the bytes of every file below ui/img/storymaker/
              except catalog.json, in sorted path order; text files LF-normalised.
```

C11. After the line starting `                licence:"OFL-1.1"}]`, insert:

```text
glyphs        {file:"glyphs.json"}
```

C12. Before the line starting `- The default font is Geist`, insert:

```text
- Fallback faces live only in `glyphs.json`. They are not `fonts` entries and `style.font` cannot name them.
```

C13. Line starting ``- `LoadCatalog` checks the manifest and the entry fields.``

```text
- `LoadCatalog` checks the manifest and the entry fields. `LoadCatalog` reads `glyphs.file` too. It does not read the other files named by entries. `VerifyFiles` checks that every referenced file exists (used by tests).
```

C14. Line starting `| text styles | 3 |`

```text
| text styles | 6 | `text.title` `text.caption` `text.label` `text.speech` `text.thought` `text.subtitle` |
```

C15. Line starting `The starter catalog has 76 entries`

```text
The catalog has 79 entries (4 + 23 + 4 + 10 + 9 + 10 + 7 + 2 + 6 + 4) and 2 palettes. Entries of other kinds (`sfx`) belong to phase 2.
```

C16. After the line starting `` `thumbs/<id>.svg`, one per entry. `` and before the following `---`, insert this subsection:

````text

### Glyphs

`ui/img/storymaker/glyphs.json` holds the coverage of each face. The manifest names it as `"glyphs":{"file":"glyphs.json"}`, and the file joins the `version` hash and `--check`.

```json
{"schema":"storymaker-glyphs/1","faces":[
  {"id":"sm-greek","family":"SMGreek","file":"/fonts/UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa1pL7SUc.woff2","weight":"300 700","unicode_range":"U+0370-03FF","d":[880,3]},
  {"id":"sm-greek-ext","family":"SMGreek","file":"/fonts/UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa2ZL7SUc.woff2","weight":"300 700","unicode_range":"U+1F00-1FFF","d":[7936,21]},
  {"id":"geist","family":"Geist","file":"/fonts/Geist-Regular.woff2","weight":"400","unicode_range":"","d":[32,94]},
  {"id":"sm-deva","family":"SMDeva","file":"/fonts/qr/NotoSansDevanagari.ttf","weight":"100 900","unicode_range":"","d":[2304,127]},
  {"id":"sm-arab","family":"SMArab","file":"/fonts/qr/NotoSansArabic.ttf","weight":"100 900","unicode_range":"","d":[1536,255]},
  {"id":"sm-cjk","family":"SMCJK","file":"/fonts/qr/NotoSansCJKsc.otf","weight":"400","unicode_range":"","max_bytes":17825792,"d":[32,94]}
]}
```

- The `d` values above show the format only; the build writes the real ones. Decode: the first `d` value is the first range start, then `gap = start - previous end`, `len = end - start`.
- A codepoint is covered if it is in the face's cmap and inside `unicode_range`. `geist` data excludes U+0370-03FF.
- Family order: `SMGreek`, the style family in place of `Geist`, `SMDeva`, `SMArab`, `SMCJK`.
- A face whose `family` equals a catalog font family adds no need. Its faces are loaded through `catalog.fonts`.
- The host attaches the parsed file as `catalog.glyph_data`. That key exists only in memory, never in `catalog.json`.
````

C17. Line starting `12. **Mouth.**`

```text
12. **Mouth.** The current speech shape (`rest ai e o u mbp fv`), else the mouth of the expression, else `rest`. Expression mouths (for example `smile`, `frown`) are variants of the same slot. Speech shapes come from the player's lip-sync track.
```

C18. Line starting `` `internal/storymaker/testdata/catalog-mini/catalog.json` is a small catalog ``

```text
`internal/storymaker/testdata/catalog-mini/catalog.json` is a small catalog for the tests. Its version is `mini00000000` and it has one palette (`palette.sunny`). Its 23 entries are `char.office-woman-1`, `prop.lock`, `prop.shield`, `env.plain-stage`, `env.office`, `env.abstract-tech`, `pose.stand`, `pose.typing`, `gesture.point`, `expr.neutral`, `expr.surprised`, `expr.happy`, `mouth.rest`, `cycle.walk`, `cycle.idle`, `text.title`, `text.speech`, `text.thought`, `text.subtitle`, `trans.cut`, `trans.fade`, `trans.slide-left`, `trans.wipe`. Its first 20 entries are the mini fixture text recorded by parts A and C; phase 2 added the three speech styles.
```

- [ ] **Step 2: Edit contract section 4**

C19. Line starting `// Code<CamelCase> constants for the`

```text
// Code<CamelCase> constants for the 28 issue codes of section 2
```

C20. Line starting `` `LoadCatalog` checks the manifest: schema, ``

```text
`LoadCatalog` checks the manifest: schema, `version` (1 to 64 characters), `stage`, `rig`, `glyphs` (read through `LoadGlyphs` when the manifest names it), the font named `default`, palettes with the seven hex tokens, entry prefixes, bare names, `name_key`, relative thumbs, `usage_example` objects and the fields of each kind.
```

C21. After the line `    MkdirAll(ctx context.Context, dir string) error`, insert:

```text
    Open(ctx context.Context, path string) (io.ReadSeekCloser, FileInfo, error)   // missing: fs.ErrNotExist
```

C22. After the line starting ``- `ReadFile` returns `ETag(data)`.``, insert:

```text
- `Open` returns a seekable file for the audio routes, which serve files over 50 MB through it. `FileInfo` is `{Name string; Size int64; ModTime time.Time; MIME string}`. `MemStore` and `storymakerStore` are the only implementers.
```

C23. Line starting ``- `Create` writes `story.json` ``

```text
- `Create` writes `story.json` (title, format, default style, one scene `s1` on `env.plain-stage`, defaults applied) and `project.json`. It creates `assets/`, `audio/narration/`, `audio/music/` and `exports/`. The folder `scenes/` appears with the first code scene. `CreateWithVoice` stores a resolved voice.
```

C24. Line starting ``- `Delete` removes the project folder``

```text
- `Delete` removes the project folder (`Store.Remove`, recursive). The `DELETE` endpoint also cancels the project's render, narrate and music jobs. Code files have no delete of their own: a code scene removed from the story leaves `scenes/<id>.js` in place.
```

C25. Before the line `### Configuration`, insert:

````text
### Narration, envelopes, audio info, glyphs and voices

```go
const BlockKindNarration, BlockKindDialogue    // "narration" "dialogue"
const JoinSentence, JoinSplit                  // "sentence" (350 ms pause after the piece), "split" (150 ms)

type NarrationBlock struct { ID, Kind, SceneID string; SceneIndex, Beat int; Speaker string; Voice Voice; Text string }
type Piece struct { Text string; Chars int; Join string }
func Blocks(s *Story) []NarrationBlock
func StripMarkers(text string) string
func SplitPieces(text string, limit int) []Piece
func NormalizeProvider(id string) string       // trimmed and lower-cased
func ChunkLimit(provider string) int           // runes: phonon 2000, sanotts 2000, google 200, others 400
func PieceKey(v Voice, pieceText string) string
func PiecesOf(b NarrationBlock) []Piece
func IsJSSpace(r rune) bool
func UTF16Len(s string) int
func TextWidth(s string) int                   // East Asian wide counts 2
func WordCount(s string) int

var ErrUnknownAudio error
type DecodedAudio struct { Rate, Channels int; Samples []float32 }
func SniffAudio(head []byte) string            // "wav", "mp3" or ""
func DecodeAudio(data []byte) (DecodedAudio, string, error)
func ProbeAudio(r io.ReadSeeker) (ext string, durationMS int, err error)

type Levels []uint8                            // marshals as a JSON array of integers 0..255
type Envelope struct { V, DurationMS, Rate int; Levels Levels }   // audio/narration/<hash>.env.json
func ComputeEnvelope(a DecodedAudio) Envelope
func (e Envelope) SpeechRuns() [][2]int
func ParseEnvelope(data []byte) (Envelope, error)
type AudioPieceInput struct { Hash, Ext string; Chars int; Join string; Env Envelope }
type AudioPiece struct { Hash, Ext string; AtMS, InMS, OutMS, Chars int }
type AudioBlock struct { ID, Kind string; Beat *int; Speaker string; DurationMS int; Spans [][2]int; Levels Levels; Pieces []AudioPiece; SpeechOK bool }
func AssembleBlock(b NarrationBlock, pieces []AudioPieceInput, fps int) AudioBlock

type SceneAudio struct { Blocks []AudioBlock }
type AudioInfo struct { Narration map[string]SceneAudio }
type MissingBlock struct { SceneID, BlockID string; Chars int }
type CharCounts struct { Total, Missing int }
type MusicRef struct { Key, Ext string; DurationMS int }
type AudioAnswer struct { Narration map[string]SceneAudio; Missing []MissingBlock; Chars CharCounts; Music *MusicRef; MusicIssue string }
func (a AudioAnswer) Info() AudioInfo
var ErrMusicSourceMissing error
type MusicKeyResolver func(ctx context.Context, m *Music) (string, error)
func (p *ProjectStore) AudioAnswer(ctx context.Context, id string, s *Story, resolve MusicKeyResolver) (AudioAnswer, error)
type PieceFile struct { Ext string; HasEnv bool }
func (p *ProjectStore) PieceFiles(ctx context.Context, id string) (map[string]PieceFile, error)
func (p *ProjectStore) AudioPath(id, folder, file string) (string, error)
func (p *ProjectStore) OpenAudio(ctx context.Context, id, folder, file string) (io.ReadSeekCloser, FileInfo, error)
func (p *ProjectStore) WritePiece(ctx context.Context, id, hash, ext string, data []byte, env Envelope) error
func (p *ProjectStore) WriteEnvelope(ctx context.Context, id, hash string, env Envelope) error
func (p *ProjectStore) RemovePiece(ctx context.Context, id, hash, ext string) error

const GlyphsSchema = "storymaker-glyphs/1"
type GlyphFace struct { ID, Family, File, Weight, UnicodeRange string; MaxBytes int64 }
type Glyphs struct { Faces []GlyphFace }
func LoadGlyphs(fsys fs.FS, path string) (*Glyphs, error)   // at most 1 MiB, schema, monotonic ranges
func (g *Glyphs) FaceOf(r rune) int
func (g *Glyphs) Covered(r rune) bool
func (g *Glyphs) Missing(text string) []rune
func IsDefaultIgnorable(r rune) bool
func (c *Catalog) Glyphs() *Glyphs                          // nil when the manifest names no glyphs file
func (c *Catalog) GlyphsFile() string

func (p *ProjectStore) CreateWithVoice(ctx context.Context, title, aspect string, fps int, voice *Voice) (Project, Story, string, error)
type VoiceOption struct { ID, Name, Language, Gender string; Installed *bool; Note string }
type VoiceProvider struct { ID string; Active, Ready bool; Voices []VoiceOption; FreeText bool; Languages, TestedLanguages []string; NeedsInstall bool; DefaultVoice string; ChunkLimit int; Note string }
var ErrNoVoiceForLanguage error
func ResolveProjectVoice(providers []VoiceProvider, language string, defaultVoiceFor func(provider, language string) string) (Voice, []string, error)
func LanguageBase(tag string) string
func LocalVoiceProvider(id string) bool                     // phonon, piper, sanotts, supertonic, speech_lab
func ApplyCaptionOverride(s *Story, burnIn *bool)
```

- A block enters `Narration` only when every piece has audio and a valid envelope. Otherwise it appears in `Missing`, and the player estimates it. `Missing` and `Narration` are never null.
- `ErrMusicSourceMissing` is not an error of `AudioAnswer`: the narration stays, `Music` is null and `MusicIssue` is `source_missing`.
- Layout: `audio/narration/<hash>.<ext>`, `audio/narration/<hash>.env.json`, `audio/music/<key>.<ext>`.

````

C26. After the line starting `    Render  StorymakerRenderConfig`, insert:

```text
    Narration StorymakerNarrationConfig `yaml:"narration" json:"narration"`
```

C27. Before the line starting `func NormalizeStorymakerConfig`, insert:

```text
type StorymakerNarrationConfig struct {
    MaxDailyChars int `yaml:"max_daily_chars" json:"max_daily_chars"` // default 100000, 1000..100000000
    MaxJobChars   int `yaml:"max_job_chars" json:"max_job_chars"`     // default 20000, 100..10000000
}
```

- [ ] **Step 3: Edit contract section 5**

C28. Line starting ``| `POST /projects` `{title, aspect?, fps?}` |``

```text
| `POST /projects` `{title, aspect?, fps?, language?}` | write | 201 `{project, story, etag}`, `ETag` and `Location` headers. Defaults 16:9 and 30 fps. With `language` the story has the resolved `voice`; without it `voice:null`. | 400 `invalid_request` (body), 400 `invalid_argument` (title 1 to 120 characters, aspect, fps 24, 25 or 30), 422 `narration_unavailable` with `languages` |
```

C29. Line starting ``| `GET /projects/{id}/jobs` |``

```text
| `GET /projects/{id}/jobs` | read | 200 `{jobs:[Job]}`, newest first, across the render and narrate queues | |
```

C30. Line starting ``| `GET /projects/{id}/events` |``

```text
| `GET /projects/{id}/events` | read | `text/event-stream` (see below), across the render and narrate queues | |
```

C31. Line starting ``| `GET /jobs/{id}` |``

```text
| `GET /jobs/{id}` | read | 200 `{job}`, across the render and narrate queues | 404 `job_not_found` |
```

C32. Line starting ``| `POST /jobs/{id}/cancel` |``

```text
| `POST /jobs/{id}/cancel` | write, `desktopStop` | 200 `{job}`, across the render and narrate queues. Also answers for a finished job. | 404 `job_not_found` |
```

C33. After the line starting ``| `GET /player` |``, insert:

```text
| `GET /voices` | read | 200 `{providers:[VoiceProvider]}`: the active provider first, then phonon, piper, sanotts, supertonic, speech_lab, then the cloud ids sorted. `no-store`, no secret. google is absent unless active. Works read-only. | none; a provider whose list fails has `ready:false` |
| `GET /projects/{id}/narration` | read | 200 `AudioAnswer`, `no-store`. Works read-only. | 404 `project_not_found`, 422 `invalid_story` |
| `POST /projects/{id}/narrate` (empty body) | write | 202 `{job}` new; 200 `{job}` existing queued or running job of this project; 200 `{job:null}` nothing missing. Coalesced per project and kind. | 404 `project_not_found`, 422 `invalid_story`, 409 `voice_required` (any block without a voice, message names the first), 409 `narration_unavailable` (provider not ready or not offered, message names it), 409 `voice_unknown` with `alternatives` (at most 5; a voice off the provider's list, or a Phonon voice that is not installed), 409 `budget_exceeded` with `{limit, missing}`, 429 `busy`, 403 `desktop_readonly` |
| `GET,HEAD /projects/{id}/audio/narration/{hash}.{ext}` | read | 200 or 206 through `http.ServeContent`, Range honoured. Works read-only. | 404 `audio_not_found`, 405 |
| `GET,HEAD /projects/{id}/audio/music/{key}.{ext}` | read | 200 or 206, Range honoured. Works read-only. | 404 `audio_not_found`, 405 |
```

C34. After the line starting `- Sixteen jobs may wait`, insert:

```text
- 128 narrate jobs may wait, in their own queue. `/narrate` coalesces per project and kind.

Audio routes:

- The folder segment must be exactly `narration` or `music`, else 404 `audio_not_found`. The file segment must match `^([0-9a-f]{64})\.(wav|mp3)$`.
- `.env.json`, `.meta.json` and `.pending.json` are never served.
- Headers: `Content-Type: audio/wav` or `audio/mpeg`, `X-Content-Type-Options: nosniff`, `Content-Security-Policy: sandbox; default-src 'none'`, `Cache-Control: private, max-age=86400, immutable`.

Narration answer:

```json
{
  "narration": {"s1": {"blocks": [{
      "id":"s1:n", "kind":"narration", "speaker":"narrator",
      "duration_ms":4210, "spans":[[40,1820],[2170,4110]], "speech_ok":true,
      "levels":[0,0,91,140,171],
      "pieces":[{"hash":"<64 hex>","ext":"wav","at_ms":0,"in_ms":52,"out_ms":1932,"chars":34}]
  }]}},
  "missing": [{"scene_id":"s2","block_id":"s2:n","chars":120}],
  "chars": {"total":640,"missing":120},
  "music": {"key":"<64 hex>","ext":"mp3","duration_ms":61000}
}
```

- `beat` appears on dialogue blocks only. `pieces[].chars` is UTF-16 units including joining whitespace. `missing[].chars` and `chars.*` are code points of the piece text, which is what cloud caps count.
- `music_issue` appears only as `"source_missing"`, together with `music:null`.
```

C35. Line starting ``| `limits` | object |``

```text
| `limits` | object | `max_duration_seconds`, `max_scenes`, `max_items_per_scene`, `max_beats_per_scene`, `max_narration_chars`, `max_story_bytes`, `max_code_bytes`, `max_queued_renders` (16), `max_queued_narrations` (128), `max_daily_chars`, `max_job_chars` |
```

C36. Replace these lines:

```text
Job {id, project_id, kind:"render", status, progress:0..1, phase?, frames_done, frames_total,
     output?:{path, size}, error?:{code, message}, created_at, updated_at, started_at?, finished_at?}
status  queued | running | succeeded | failed | cancelled | interrupted
phase   starting | compiling | rendering | encoding | writing
```

with:

```text
Job {id, project_id, kind:"render"|"narrate", status, progress:0..1, phase?, frames_done, frames_total,
     pieces_done?, pieces_total?,
     output?:{path, size}, error?:{code, message}, created_at, updated_at, started_at?, finished_at?}
status  queued | running | succeeded | failed | cancelled | interrupted
phase   starting | compiling | rendering | encoding | writing | synthesizing
```

C37. After the line starting `- A job that was running when AuraGo stopped`, insert:

```text
- A narrate job has no `output` key. `pieces_done` and `pieces_total` show its progress.
```

C38. Line starting `Job error codes:`

```text
Job error codes: `invalid_story`, `code_error`, `browser_unavailable`, `ffmpeg_unavailable`, `render_failed`, `render_timeout`, `cancelled`, `interrupted`, `storymaker_disabled`, `narration_failed`, `narration_unavailable`, `voice_required`, `budget_exceeded`.
```

C39. Line starting `2. The latest five jobs of the project`

```text
2. Every unfinished job across both queues and the latest five jobs across both queues, oldest first, as `event: job` with `data: {"job": {...}}`. A window opened during a render sees its progress at once.
```

- [ ] **Step 4: Edit contract section 6**

C40. Line starting ``- `ui/js/storymaker/player.bundle.js` (``

```text
- `ui/js/storymaker/player.bundle.js` (30 parts, in the order of `scripts/storymaker-bundle-parts.js`; phase 2 added `seed`, `ranges`, `motion`, `lint`, `glyphs`, `cuemap`, `captions`, `lipsync`, `bubble`) runs in workers (one blob per worker, prelude `self.__SM_ROLE__ = "player" | "scene"; importScripts(<base>/js/storymaker/player.bundle.js);`, plus the scene bundle text for a scene worker).
```

C41. Line starting ``- `audioInfo` is `{}` in phase 1.``

```text
- `audioInfo` is `{}` or `{narration:{<scene id>:{blocks:[Block]}}}`, the `narration` value of `GET /projects/{id}/narration`. A block's absence means the scene or say text is estimated (words ÷ 2.3 s, at least 1.0 s), and a scene's narration or a say's text then gets `narration_missing`.
```

C42. Replace these lines:

```text
{frame_count, fps, width, height, duration_ms, catalog_version,
 scenes:[{id, start_frame, end_frame, kind}], audio_events:[], captions:[], checks:[],
 unverified:[Issue], issues:[Issue]}
```

with:

```text
{frame_count, fps, width, height, duration_ms, catalog_version,
 scenes:[{id, start_frame, end_frame, kind, cues:[{marker, at_ms, placed}]}],
 audio_events:[{kind:"narration"|"dialogue", ref, start_ms, scene_id, block_id, piece}],
 captions:[{start_ms, end_ms, lines, scene_id, block_id}], cue_precision, checks:[],
 unverified:[Issue], issues:[Issue]}
```

C43. Line starting ``- Phase 1: `audio_events`, `captions` and `checks` are empty.``

```text
- `checks` is empty until phase 3. `unverified` holds one minor `layout_unverified` per code scene. `captions` is always filled, measured or estimated. Compile drops audio events at or past `duration_ms` and cuts captions at it.
```

C44. Line starting ``- Timing: `audio_events` carries one``

```text
- `audio_events` has one entry per piece; Go owns the gain, so there is no `gain` field. `cue_precision` is the share of cues placed by a span, or null without cues. `scenes[].cues[].at_ms` is scene-local.
```

C45. Line starting `- A cue time is the start plus`

```text
- A cue time comes from `cueMap(block, offset)`: inside the piece holding the offset, proportional over that piece's used audio, starting at its `at_ms`; `placed:proportion` over the block duration when pieces are unmeasured, their `chars` do not sum to the text length, or a piece had no detectable speech.
```

C46. Line starting `- Automatic scene length is`

```text
- Automatic scene length is the largest of: narration end plus 0.4 s, last dialogue block end plus 0.4 s, the end of the last beat, the end of the last text hold, and 1.0 s for a scene without speech. A code scene without `api.duration` takes its last motion plus 0.5 s; with narration, it takes the larger of that and this automatic length.
```

C46a. Line starting ``- An explicit `duration` is``

```text
- An explicit `duration` is the total local length, entry transition included. A code scene's `api.duration` counts as one. Narration longer than an explicit duration is a `warning` at `/scenes/<i>/duration`.
```

C46b. Line starting ``- `api.duration(seconds)`:``

```text
- `api.duration(seconds)`: the local length (an explicit story `duration` wins). The scene keeps it, and a longer narration is a warning. A scene that does not call it takes its last motion plus 0.5 s; with narration, the larger of that and the automatic scene length.
```

C46a. Line starting ``- An explicit `duration` is the total local length``

```text
- An explicit `duration` is the total local length, entry transition included. Narration or a say block that runs past an explicit duration is one `warning` at `/scenes/<i>/duration`: the speech overlaps the next scene or is cut at the film end.
```

C47. Line starting `- Beat defaults`

```text
- Beat defaults, scaled by the rhythm factor unless explicit: enter and exit 0.6 s (they walk); walk distance at `cycle.walk.speed`, at least 0.3 s; move 0.8 s; pose 0.3 s; gesture = catalog duration × `count`; emote 0.25 s; look 0.3 s; show 0.5 s (0 with `entry: "none"`); hide 0.35 s; camera 0.8 s (shake 0.5 s); highlight 0.8 s; say the block length. `wait` is `seconds` (default 1), never scaled.
```

C48. Before the line starting `- Camera framings: wide 1.0`, insert:

```text
- `say` with text starts at the later of its scheduled start and the previous block's end, sets the new block end, and emits one `dialogue` event per piece. An estimated say with text gets a warning `narration_missing` at `/scenes/<i>/beats/<j>/text`. It adds the bubble and, with measured levels, a `speech` interval on `who`. When the actor is off stage the events stay, and there is no bubble and no interval. `say` without text keeps its start, lasts 0 and leaves the block end, the cursor and the target's last end unchanged. Its bubble stays until the end of the scene narration, with the 0.12 s fades, and its lines come from `SM.captions.at` at each time. A start at or after that end draws no bubble, and a silent scene draws none.
```

C49. Line starting ``- `show` text uses the `title` style``

```text
- `show` text uses the `title` style at the upper third. Text is drawn in screen space; camera movement does not move it. Bubbles and burned captions are drawn by the player; bubbles follow the camera.
```

- [ ] **Step 5: Edit contract sections 7, 8 and 9**

C50. Line starting ``- Every response under `<base>` has``

```text
- Every response under `<base>` has `Access-Control-Allow-Origin: *`, `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`. The render page has an opaque origin, so every request from it is cross-origin with `Origin: null`; the prefix is the credential for reads, and the token of `boot.json` is the credential for `/report`, `/frame` and `/error`.
```

C51. Replace these three lines:

```text
| `POST <base>/report` | compile report (JSON, 1 MiB at most) |
| `POST <base>/frame?n=<n>` | one frame (see below) |
| `POST <base>/error` | a page failure (see below) |
```

with:

```text
| `POST <base>/report?t=<token>` | compile report (JSON, 8 MiB at most) |
| `POST <base>/frame?n=<n>&t=<token>` | one frame (see below) |
| `POST <base>/error?t=<token>` | a page failure (see below) |
```

C52. Before the line `The render page is served with this CSP:`, insert:

```text
`boot.json` carries `token` on its first GET only. `/report`, `/frame` and `/error` require it as `t`; otherwise they answer 403 `bad_token` without any effect.

```

C53. Replace these three lines:

```text
{mode:"render", fps, width, height, transport, frame_limit, frame_deadline_ms,
 story_url, catalog_url, assets_base, code_bundles:{<scene_id>: url}, fonts:[{name, family, weight, url}],
 audio_info:{}}
```

with:

```text
{"mode":"render","fps":30,"width":1920,"height":1080,"transport":"png","frame_limit":27000,"frame_deadline_ms":5000,
 "token":"<32 hex, first GET only>",
 "story_url":"<base>/story.json","catalog_url":"<base>/catalog/catalog.json","assets_base":"<base>/catalog/",
 "code_bundles":{"s3":"<base>/scenes/s3.js"},
 "fonts":[{"name":"default","family":"Geist","weight":400,"url":"<base>/fonts/Geist-Regular.woff2"},
          {"name":"sm-cjk","family":"SMCJK","weight":400,"url":"<base>/fonts/qr/NotoSansCJKsc.otf","fallback":true,"weight_range":"400","max_bytes":17825792},
          {"name":"sm-greek","family":"SMGreek","weight":400,"url":"<base>/fonts/UcC73...woff2","fallback":true,"weight_range":"300 700","unicode_range":"U+0370-03FF"}],
 "audio_info":{"narration":{}}}
```

C54. Line starting ``- `fonts` lists the four faces``

```text
- `fonts` lists the faces of the story's font and every fallback face (`fallback`, `unicode_range`, `weight_range`, `max_bytes`). The page fetches a face only when compile needs it, and takes font URLs only from this list.
```

C55. Line starting `1. The page loads boot.json, story.json,`

```text
1. The page loads boot.json, story.json, the catalog, `glyphs.json` and the bundles, then compiles, and fetches the fonts compile needs during `prepare`.
```

C56. Line starting `` `POST <base>/frame?n=<n>` with the body of one frame. ``

```text
`POST <base>/frame?n=<n>&t=<token>` with the body of one frame. The page sends `Content-Type: text/plain`, which is CORS-safelisted, so Chrome sends no preflight. The listener also accepts the number as the header `X-SM-Frame: <n>`; the query is used when the header is absent.
```

C57. Line starting `Refusal codes: `

```text
Refusal codes: `frame_header` (400, no number), `frame_too_large` (413), `frame_read` (400), `frame_limit` (413, past the frame count), `frame_order` (409, a frame other than the expected one), `not_running` (409), `encoder_failed` (500), `bad_token` (403).
```

C58. Line starting `` `POST <base>/error` with `{code, message, scene_id?}`. ``

```text
`POST <base>/error?t=<token>` with `{code, message, scene_id?}`. The code is clipped to 40 characters, the message to 600 and the scene id to 60; the body is at most 16 KiB. Codes sent by the page: `readiness`, `deadline`, `code_error`, `render_failed`. The answer is `{"ok":true}`; a wrong token answers 403 and does not end the job. Any accepted `/error` ends the job: a page that reports an error does not continue.
```

C59. Line starting `` `report_too_large` (413, ``

```text
`report_too_large` (413, over 8 MiB), `report_repeated`, `bad_token` (403).
```

C60. Line starting `Limit: the prefix is not a secret against scene code`

```text
Limit: the prefix is not a secret against scene code, but a scene cannot obtain the token, because the host takes it before any worker exists. Go validates every report field it uses against its own plan.
```

C61. Line starting ``| `compiled` | `report` |``

```text
| `compiled` | `report` | `report` (section 6), cleaned by the parent; `audio_events` at most 40000, `captions` at most 8000 |
```

C62. Line starting ``| `load` | `story`, `catalog` ``

```text
| `load` | `story`, `catalog` (parsed `catalog.json`; may carry `glyph_data`), `audio_info?` (section 6), `assets?` (`catalog:<path>` to parsed JSON), `fonts?` (`{name, family, weight, data: ArrayBuffer, weight_range?, unicode_range?}`), `code_bundles?` (`{<scene_id>: text}`), `format?` `{aspect, fps, width?}` | a new `load` or `reset` ends all workers |
```

C63. Line starting `| fonts | 16, each at most 8 MiB`

```text
| fonts | 16, each at most 8 MiB unless a named glyph face allows more, 32 MiB in total, checked on `load.fonts` and on `font:` values of `assets`, with the faces and the byte count of the last accepted load |
```

C64. After the line starting ``| format `fps` | 24, 25, 30 |``, insert:

```text
| audio scenes | 400 |
| audio blocks per scene | 61 |
| audio pieces per block | 800 |
| levels per block | 27000 |
| levels per message | 200000 |
| glyph faces | 16 |
| glyph range values | 60000 |
| fallback font | at most its `max_bytes` and at most 18 MiB |
```

C65. Line starting ``- Module loader (`ui/js/desktop/core/module-loader.js`):``

```text
- Module loader (`ui/js/desktop/core/module-loader.js`): the style `desktop-app-storymaker.css` and thirteen scripts in order: `storymaker-core.js`, `storymaker-model.js`, `storymaker-api.js`, `storymaker-assets.js`, `storymaker-player.js`, `storymaker-projects.js`, `storymaker-strip.js`, `storymaker-inspector.js`, `storymaker-code.js`, `storymaker-export.js`, `storymaker-audio.js`, `storymaker-voice.js`, `storymaker.js`. Each file is under 1100 lines.
```

C66. Line starting `` `ui/cfg/virtual_desktop.js` (`vdCfgStorymaker`): ``

```text
`ui/cfg/virtual_desktop.js` (`vdCfgStorymaker`): the toggle for `storymaker.enabled`; `storymaker.render.timeout_seconds` (60 to 14400), `storymaker.render.max_duration_seconds` (10 to 3600), `storymaker.render.browser_path`, `storymaker.render.transport` (automatic, png, webp, raw, screenshot), `storymaker.narration.max_daily_chars` (1000 to 100000000), and `storymaker.narration.max_job_chars` (100 to 10000000). The "Check export setup" action reads `GET /api/desktop/storymaker/status` for the saved settings only.
```

- [ ] **Step 6: Edit `internal/storymaker/AGENTS.md`**

Old:

```text
- `LoadCatalog` checks the manifest only; `VerifyFiles` checks that referenced files exist.
```

New:

```text
- `LoadCatalog` checks the manifest and reads `glyphs.file` through `LoadGlyphs`, nothing else; `VerifyFiles` checks that referenced files exist.
```

Old:

```text
`interrupted`, `storymaker_disabled`) are shared
```

New:

```text
`interrupted`, `storymaker_disabled`, `narration_failed`, `narration_unavailable`, `voice_required`, `budget_exceeded`) are shared
```

Before the line `## Render and server (part B)`, insert:

```markdown
## Speech

- `narration.go`: `Blocks` is the one list of what must be spoken: one narration block per scene that is not silent, and one dialogue block per `say` with text, also in a silent scene and also with its actor off stage. `StripMarkers` is byte for byte `SM.narration.parse(text).plain`, and `testdata/narration/cross_language.json` holds the two languages together. One sentence is one piece (`SplitPieces`). `PieceKey` hashes provider, voice, language and piece text, with provider and language trimmed and lower-cased.
- `audio_decode.go`: `DecodeAudio` and `ProbeAudio` detect the format from the bytes, never from an extension. WAV and MP3 only, at most 600 s. `ProbeAudio` reads a duration without keeping samples.
- `envelope.go`: `<hash>.env.json` is a 10 ms series, so it stays free of the frame rate. `AssembleBlock` is the one function behind `/narration` and `Plan.AudioInfo`. `ParseEnvelope` trusts nothing, because these are ordinary workspace files: an audio file without a valid envelope counts as missing.
- `glyphs.go`: reads the build-generated `ui/img/storymaker/glyphs.json`; Go never reads WOFF2. `Validate` adds `glyph_missing` for show text, say text and narration that is drawn. A silent scene's narration is never drawn, so it is not checked. Code-scene strings stay `layout_unverified`. The mini catalog has no `glyphs` key and skips the check.
- `voices.go`: `ResolveProjectVoice` takes providers in the order of `storymakerVoiceProviders`, so a paid provider never wins over a free one. `Validate` checks only the shape of a voice, because the package does not import `internal/tools`; `POST /narrate` checks the list.
- `Store.Open` returns a seekable file for the audio routes. `MemStore` and `storymakerStore` are its only implementers.

```

- [ ] **Step 7: Edit `internal/storymaker/render/AGENTS.md`**

Old:

```text
secret against scene code. What a scene could do with it (post frames, errors, a report) only affects its own job.
```

New:

```text
secret against scene code. The prefix is the credential for reads only; a scene cannot obtain the token that `/report`, `/frame` and `/error` need.
```

Before the line starting `- **Ingest** (`, insert:

```markdown
- **Narrate jobs** (`jobs.go`): the server runs a second `Manager` for narrate jobs (`MaxQueued` 128). `Kinds` names what a manager loads and accepts, and `Workers` other than 0 or 1 is refused. `EnqueueRequest.Coalesce` returns a queued or running job of the same project and kind with replayed true, under `m.mu` in the same step as the insert. `SetPieces` fills `pieces_done` and `pieces_total`. A narrate job has no `output` key.
- **Token** (`listener.go`): `boot.json` serves a 32-hex token from `crypto/rand` on its first GET only (`atomic.Bool` swap; HEAD consumes nothing). `/report`, `/frame` and `/error` compare `t` in constant time. A mismatch answers 403 `bad_token`, calls no hook and consumes nothing. `StartListener` refuses a shorter token.
- **Report** (`report.go`): at most 8 MiB. `parseReport` checks every field Go uses against its own plan, with `filmMS` from `frame_count` and `fps`, never the report's own value: known block, kind and scene, `Hashes[piece] == ref`, unique pairs, a piece prefix without a gap, at least piece 0 for each measured narration block, times inside the film, at most `MaxEvents` events and 8000 captions, and caption text that is a substring of its block (`captionKey`). An empty `audio_events` with measured blocks is refused. A `gain` from JS is ignored.
- **Audio info and fonts** (`render.go`, `listener.go`): `Plan.AudioJSON` is marshalled once and served in `boot.audio_info`, so it equals the `narration` of `GET /narration`; nil serves `{}`. `boot.fonts` lists the story's faces and every fallback face. The page fetches a face only when compile needs it, and a response over `max_bytes` is a readiness failure. `glyphs.json` is in the snapshot. Exports stay silent in batch 2B: the audio track is still `anullsrc`.
```

- [ ] **Step 8: Edit `ui/js/storymaker/AGENTS.md`**

Before the line `## Verify`, insert:

```markdown
## Speech

- New parts: `glyphs.js` (coverage, font needs, family chain), `cuemap.js` (`cueMap(block, offset)`, the one function behind cue and caption times), `captions.js` (2 lines, width 42, at least 1.0 s), `lipsync.js` (mouths from stored levels, seeded from the block id), `bubble.js` (speech and thought bubbles, 0.12 s fades).
- `say` is in the support matrix. With text it runs one `dialogue` block after the previous block ends. Without text it draws the active caption line of the scene narration until that narration ends, has no block and moves nothing. In a silent scene it draws nothing. An actor off stage keeps the audio events and gets no bubble and no lip-sync.
- `audio_events` has one entry per piece and no `gain`; Go computes every gain. `captions` are always filled, and `style.captions` decides drawing only. Compile drops events at or past `duration_ms` and cuts captions at it.
- Fonts are needed per character, so a Latin story fetches no CJK face. Compile and the wrap use the generated `ranges.js`, never `Intl.Segmenter`, Unicode property escapes or `normalize`, so an ICU version cannot change a layout.
- The render page keeps the boot token in a closure and adds it to `/report`, `/frame` and `/error` itself. Failure texts name only the route, so neither token nor prefix reaches `__smStatus`, the `/error` body or the job record. Font fetches are lazy through `boot.fonts`.

```

- [ ] **Step 9: Edit `ui/js/desktop/apps/AGENTS.md`**

Old:

```text
`storymaker*.js` are eleven classic scripts in this load order
```

New:

```text
`storymaker*.js` are thirteen classic scripts in this load order
```

Old:

```text
`-export.js`, then the composition root `storymaker.js`
```

New:

```text
`-export.js`, `-audio.js` (preview audio and the pure waveform model), `-voice.js` (voice panel, narration status, captions toggle), then the composition root `storymaker.js`
```

Before the line starting ``- Verify: `go test ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker'``, insert:

```markdown
- Speech: `S.bindAudio(inst)` owns `inst.audioInfo`, `inst.audio` and `inst.refreshAudio()`. It refreshes on project open, on a successful save and on every terminal status of a narrate job, and it ignores an answer for another project. `reloadPlayer` sends `audio_info: {narration}`, and `bindAudio` reloads the player when the narration JSON of a fresh answer differs from the one last sent, because the player loads before any answer exists. The iframe stays without audio or connect access: the parent plays the pieces, decodes a window of 2 s back to 20 s ahead, and reschedules above 120 ms drift. Audio URLs come only from `api.narrationUrl` with values from the parent's own `/narration` answer, never from player-message text. Audition plays a stored piece and never synthesises, so no paid route exists outside the job loop. Generate and the audition run the stored voice, so both wait while the panel shows a voice the story does not hold. A Phonon voice that is not installed is listed only while the draft holds it. The export listener takes `kind === 'render'` only; the voice panel takes `narrate` and shows its own text for `interrupted` and `cancelled`. `err.data` carries the extra fields of an error body (`languages`, `alternatives`, `limit`, `missing`).
```

- [ ] **Step 10: Edit `internal/server/AGENTS.md`**

Before the line `## Verification`, insert:

```markdown
## Storymaker audio

- `storymaker_service.go` runs two `render.Manager`s, `jobs` for renders and `narrate`. `jobByID`, `listJobs` and `cancelProject` route across both. `/events` sends every unfinished job of both plus the merged latest five, so newer narrate jobs cannot push a running export out of the list a reloaded app reads.
- `storymakerAudioRun` gets its runtime through the closure built in `newStorymakerRuntime`. It must not call `s.storymakerRuntime()`, which holds `s.storymakerMu` while the old manager waits for the running job. It starts with `beginDesktopRun`, so a config change that sets read-only cancels a provider call in flight, and every error it returns is a `*render.JobError`.
- Before every provider call the narrate loop re-reads `story.json` and the config, and checks read-only, voice, free disk and the character caps. Usage is written to `storymaker/narrate/usage.json` before the call, because a provider that answered has billed. An unreadable file becomes `usage.bad.json` and stops cloud calls for that date: a budget guard must not fail open.
- `/voices`, `POST /projects` and `POST /narrate` share `storymakerVoiceProviders`: active first, then local, then cloud, so resolution never picks a paid provider ahead of a free one. google is omitted unless active.
- The audio routes accept only the folder `narration` or `music` and `^([0-9a-f]{64})\.(wav|mp3)$`, and never serve `.env.json`, `.meta.json` or `.pending.json`.
- Test seams beside `storymakerRunOverride`: `storymakerAudioRunOverride` and `storymakerSynthOverride`. Run the tour with a stub synth through `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m`.

```

- [ ] **Step 11: Edit `documentation/storymaker.md`**

M16 edited this page first, but it only added the `## Motion` section ahead of `## Not in phase 1`, so every quoted text below is still there. Match each edit by its quoted words.

Replace the text from `This page describes` to the end of that paragraph with:

```text
This page describes
the current version: the editor, the preview, voices and narration, captions and a silent export. Music, sound in the export and
the agent come in later versions.
```

After the line starting ``| `storymaker.render.transport` |``, insert:

```text
| `storymaker.narration.max_daily_chars` | `100000` | Characters that cloud voices may speak per local day (1000 to 100000000). |
| `storymaker.narration.max_job_chars` | `20000` | Characters that cloud voices may speak in one narration job (100 to 10000000). |
```

After the line starting `scenes/<id>.js    code scenes`, insert:

```text
audio/narration/  one audio file per sentence (<hash>.wav or .mp3) with its level curve (<hash>.env.json)
```

Old:

```text
Phase 1 videos have no sound.
```

New:

```text
Videos still have no sound, even for a narrated story: the narration plays in the preview, and the export draws its captions.
```

Before the line `## Export`, insert:

```markdown
## Voice and narration

- **Voice panel.** Pick a provider, a voice and a language, then **Use this voice** writes them into the story. **Generate narration** and **Play a sample** work with the saved voice, so a voice you picked needs this click first. Phonon voices that are not installed are not listed: download them in the Phonon settings under More voices. `not tested` means nobody has listened to that provider in that language yet. A new project asks for a voice in the desktop language; when no voice speaks it, the app lists the languages that have one and offers to create the project without a voice.
- **Generate narration.** Audio is made one sentence at a time and only for sentences without audio, so an edit costs one sentence. The panel shows the characters, what has no audio and the progress. **Play a sample** plays a stored sentence and never calls a provider.
- **Limits.** Cloud voices count characters per job and per local day (settings above). Local voices (phonon, piper, sanotts, supertonic, speech_lab) are not counted.
- **Preview.** The narration plays with the preview, and the timeline shows its waveform. Speech bubbles, burned-in captions and lip-sync are drawn in the preview and in the export. A switch in the voice panel turns the captions off.
- **Languages.** Greek, Hindi, Arabic, Chinese and Japanese text uses bundled fallback fonts that load only when needed. A character no font covers, such as an emoji or Thai and Hebrew text, is the error `glyph_missing` at that text.

Projects from the first version change in two ways. Captions default to on, so every narrated older story starts drawing them. A `say` without text whose `who` is not the scene's speaker is now an error that blocks the export.

```

Old:

```text
Voices and narration audio, music and sound effects, captions, the agent panel
```

New:

```text
Music and sound effects, the agent panel
```

Replace the heading `## Not in phase 1` with `## Not yet`.

- [ ] **Step 12: Record the cue share**

Write this script to the session scratchpad as `e1j-cue-share.mjs`:

```js
// E1j: cue_precision on hand-built measured blocks in four languages. Run from the worktree root.
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const { t } = await import(pathToFileURL(path.resolve('scripts/storymaker-test-context.mjs')).href);
const core = t.core();
const TEXTS = {
  de: 'Anna arbeitet zu Hause. {1}Plötzlich klopft jemand an die Tür.',
  el: 'Η Άννα δουλεύει από το σπίτι. {1}Ξαφνικά κάποιος χτυπά την πόρτα.',
  hi: 'अन्ना घर से काम करती है। {1}अचानक कोई दरवाज़ा खटखटाता है।',
  ja: 'アンナは家で働いています。{1}突然、誰かがドアをノックします。'
};
const DURATION = 4000;

function audio(chars, speechOk) {
  return { narration: { s1: { blocks: [{
    id: 's1:n', kind: 'narration', speaker: 'narrator', duration_ms: DURATION,
    spans: speechOk ? [[40, DURATION - 100]] : [], speech_ok: speechOk, levels: [],
    pieces: [{ hash: 'a'.repeat(64), ext: 'wav', at_ms: 0, in_ms: 0, out_ms: DURATION, chars }]
  }] } } };
}

function share(text, info) {
  return core.compile({ scenes: [{ id: 's1', narration: text }] }, info, t.catalog).report.cue_precision;
}

for (const [lang, text] of Object.entries(TEXTS)) {
  const length = core.narration.parse(text).plain.length;
  console.log([lang, share(text, audio(length, true)), share(text, audio(length, false)), share(text, audio(length + 1, true)), share(text, {})].join(' | '));
}
```

Run it from the worktree root:

```powershell
node "<session scratchpad>/e1j-cue-share.mjs"
```

Append this section to `docs/superpowers/plans/2026-11-04-storymaker-phase2/measured.md`, which M16 created. In the table's body, paste the four lines the script printed, unchanged.

```markdown
## Cue share, batch 2B (spec line 853)

One scene per language, with the marker at the second sentence. The values are `cue_precision`.

language | measured | speech_ok false | chars off by one | no audio
--- | --- | --- | --- | ---

These blocks are hand-built, so the table proves the rule and its fallback on de, el, hi and ja. It does not measure a provider. A real voice's share can be read from `cue_precision` in the compile report.
```

- [ ] **Step 13: Run**

Write this script to the session scratchpad as `e1j-check.mjs`:

```js
// E1j: every path the 2B docs cite exists, and the key sentences of the slice are in the contract. Run from the worktree root.
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const CITED = [
  'internal/storymaker/narration.go', 'internal/storymaker/audio_decode.go', 'internal/storymaker/envelope.go',
  'internal/storymaker/glyphs.go', 'internal/storymaker/voices.go', 'internal/storymaker/render/report.go',
  'internal/storymaker/testdata/narration/cross_language.json', 'internal/storymaker/testdata/stories/speech.json',
  'internal/server/storymaker_audio_handlers.go', 'internal/server/storymaker_narrate.go',
  'ui/img/storymaker/glyphs.json', 'ui/fonts/qr/NotoSansCJKsc.otf', 'ui/fonts/qr/NotoSansDevanagari.ttf', 'ui/fonts/qr/NotoSansArabic.ttf',
  'ui/js/storymaker/glyphs.js', 'ui/js/storymaker/cuemap.js', 'ui/js/storymaker/captions.js', 'ui/js/storymaker/lipsync.js', 'ui/js/storymaker/bubble.js',
  'ui/js/desktop/apps/storymaker-audio.js', 'ui/js/desktop/apps/storymaker-voice.js',
  'docs/superpowers/plans/2026-11-04-storymaker-phase2/measured.md'
];
const contract = fs.readFileSync('docs/superpowers/specs/2026-11-01-storymaker-contract.md', 'utf8');
const MUST = ['the 28 issue codes', '`glyph_missing` | error', 'The catalog has 79 entries', 'Its 23 entries', '`bad_token` (403)', '`GET /voices`', '`POST /projects/{id}/narrate`', 'thirteen scripts', 'storymaker.narration.max_job_chars', '8 MiB at most', '### Glyphs', '`max_queued_narrations`', 'a duration shorter than the speech', 'a say block that runs past an explicit duration'];
const NEVER = ['Current value: ab33e88a6b77', 'Phase 1 validates the shape', 'a report over 1 MiB', 'GET <base>/start', '"pre_roll"'];
const problems = [];
for (const file of CITED) if (!fs.existsSync(file)) problems.push('missing path: ' + file);
for (const text of MUST) if (!contract.includes(text)) problems.push('contract lacks: ' + text);
for (const text of NEVER) if (contract.includes(text)) problems.push('contract still has: ' + text);
const parts = await import(pathToFileURL(path.resolve('scripts/storymaker-bundle-parts.js')).href);
if (parts.storymakerPlayerParts.length !== 30) problems.push('player parts: ' + parts.storymakerPlayerParts.length + ', the contract says 30');
if (problems.length) { console.error(problems.join('\n')); process.exit(1); }
console.log('ok');
```

```powershell
node "<session scratchpad>/e1j-check.mjs"
```

Expected: `ok`.

- [ ] **Step 14: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add -f docs/superpowers/specs/2026-11-01-storymaker-contract.md docs/superpowers/plans/2026-11-04-storymaker-phase2/measured.md
git add internal/storymaker/AGENTS.md internal/storymaker/render/AGENTS.md ui/js/storymaker/AGENTS.md ui/js/desktop/apps/AGENTS.md internal/server/AGENTS.md documentation/storymaker.md
git commit -m "docs(storymaker): contract slice and child docs for batch 2B, cue share recorded" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

A PARTIAL answer from `detect-changes` needs a grep confirmation stated in the commit body (outline section 5).

### Task E1k: Tour with voice and narration
**Batch:** 2B  **Depends on:** E1e, E1f, E1g, E1h, N11
**Files:**
Test: `internal/server/storymaker_app_browser_test.go`

- [ ] **Step 1: Impact**

Run from the main checkout:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "TestStorymakerAppRealServerBrowser" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and confirm `risk: UNKNOWN` with a text search (outline section 5).

- [ ] **Step 2: Write the failing test**

In `internal/server/storymaker_app_browser_test.go`, replace the import block.

Old:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)
```

New:

```go
import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/storymaker"
	"aurago/internal/tools"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)
```

Add the helpers and the seam.

Old:

```go
func TestStorymakerAppRealServerBrowser(t *testing.T) {
	s := storymakerRealServer(t)
```

New:

```go
// smappStubSpeech is the audio of the tour's stub synth: a 24 kHz mono s16 WAV with a 440 Hz sine. A peak of sqrt(2)*0.1
// is an RMS of 0.1, which is -20 dBFS and above the -40 dBFS speech threshold, so the envelope finds speech.
func smappStubSpeech(seconds float64) []byte {
	const rate = 24000
	frames := int(seconds * rate)
	data := make([]byte, 44+2*frames)
	copy(data[0:], "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+2*frames))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], 2*rate)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(2*frames))
	for i := 0; i < frames; i++ {
		sample := math.Sqrt2 * 0.1 * math.Sin(2*math.Pi*440*float64(i)/rate)
		binary.LittleEndian.PutUint16(data[44+2*i:], uint16(int16(sample*32767)))
	}
	return data
}

// smappEventually polls until ok returns true, for state that only the real API shows.
func smappEventually(t *testing.T, limit time.Duration, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func TestStorymakerAppRealServerBrowser(t *testing.T) {
	s := storymakerRealServer(t)
	// A stub synth behind the real narrate job: no provider call and no cost. The key only makes the provider ready.
	var synthCalls atomic.Int32
	s.Cfg.TTS.Provider = "openai"
	s.Cfg.TTS.OpenAI.APIKey = "tour-key"
	s.storymakerSynthOverride = func(ctx context.Context, cfg tools.TTSConfig, text string) ([]byte, string, error) {
		synthCalls.Add(1)
		return smappStubSpeech(0.8), "wav", nil
	}
```

Add the voice steps ahead of the export.

Old:

```go
	// Export: the real render runs (FFmpeg and Chrome), progress arrives on the project's event stream and the panel ends in
```

New:

```go
	// Voice: the panel preselects the active provider, and one action writes the voice into the story. openai lists its voices
	// and takes any language, so its language control is the free-text field.
	const setField = `(sel,value)=>{const e=document.querySelector(sel);e.value=value;e.dispatchEvent(new Event('input',{bubbles:true}));e.dispatchEvent(new Event('change',{bubbles:true}));}`
	smappClick(t, page, `[data-stm="side-tab"][data-tab="voice"]`)
	smappWait(t, page, `()=>!!document.querySelector('[data-stm="voice"]')`)
	smappWait(t, page, `()=>document.querySelector('[data-stm="voice-provider"]')?.value==='openai'`)
	smappWait(t, page, `()=>!!document.querySelector('[data-stm="voice-select"] option[value="nova"]')`)
	page.MustEval(setField, `[data-stm="voice-select"]`, "nova")
	page.MustEval(setField, `[data-stm="voice-language-free"]`, "en")
	smappClick(t, page, `[data-stm="voice-apply"]`)
	smappEventually(t, 30*time.Second, "the stored voice", func() bool {
		voice, _ := smappStored(t, origin, id)["voice"].(map[string]any)
		return voice["provider"] == "openai" && voice["voice"] == "nova" && voice["language"] == "en"
	})

	// Narration: the real job runs against the stub synth. The narration has two sentences and neither merges: "Anna works
	// from home." has width 21 (at least 20) and 4 words (more than 3), so SplitPieces gives two pieces and the job calls the
	// synth twice. Scene code2 has no narration. A drawn waveform shows that the app refreshed its answer and compiled again.
	const waveDrawn = `document.querySelector('[data-stm="scrub-wave"]')?.dataset.drawn`
	if drawn := page.MustEval(`()=>` + waveDrawn).String(); drawn != "false" {
		t.Fatalf("waveform before any audio: drawn = %q, want false", drawn)
	}
	smappClick(t, page, `[data-stm="narration-generate"]`)
	var answer storymaker.AudioAnswer
	smappEventually(t, 90*time.Second, "the narrate job", func() bool {
		answer = storymaker.AudioAnswer{}
		smappGet(t, origin, "/projects/"+id+"/narration", &answer)
		return len(answer.Missing) == 0 && len(answer.Narration["s1"].Blocks) == 1
	})
	if pieces := len(answer.Narration["s1"].Blocks[0].Pieces); pieces != 2 || synthCalls.Load() != 2 {
		t.Fatalf("pieces = %d, synth calls = %d, want 2 and 2", pieces, synthCalls.Load())
	}
	smappWait(t, page, `()=>`+waveDrawn+`==='true'`)
	smappNoErrors(t, page)

	// Export: the real render runs (FFmpeg and Chrome), progress arrives on the project's event stream and the panel ends in
```

- [ ] **Step 3: Run it**

This task adds no product code, because E1e-E1h and N11 already landed. So the test passes on its first correct run.

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go vet ./internal/server
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```

Run with the shell sandbox off. Test binaries stay under `D:\tmp\go-build`.

Expected: `go vet` is clean and the test passes.
- A timeout on a `voice-*` or `narration-generate` selector, or on the waveform, is a defect in E1e, E1f or E1g.
- `the narrate job` timing out is a defect in N11 or N15.
- An export failure after this point is a defect in the render wire: the report now carries real events and captions (N18-N21, S3, S4).

Send a failure to debug-agent, and send the fix to maker-agent in the owning file.

- [ ] **Step 4: Implement**

Nothing beyond Step 2. Any fix from Step 3 goes into the file that owns the defect, in its own commit with its own failing test.

- [ ] **Step 5: Run**

```powershell
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/server -run '^TestStorymakerAppRealServerBrowser$' -count=1 -timeout 15m -v; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
gofmt -l internal/server
```

Expected: one `--- PASS: TestStorymakerAppRealServerBrowser` line, with no `SKIP`, and `gofmt` prints nothing. The earlier lines of the tour still hold: the MP4 is 1920x1080 yuv420p.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/server/storymaker_app_browser_test.go
git commit -m "test(storymaker): real-server tour picks a voice, generates narration, sees the waveform" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

A PARTIAL answer from `detect-changes` needs a grep confirmation stated in the commit body (outline section 5).

