package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The API client and the asset resolver of the Storymaker app in Node, against a fake fetch: the headers of every write (If-Match,
// If-None-Match, Idempotency-Key), the flat error shape and what the player may be given (only what the catalog lists, only the
// bundles of the story's code scenes, unknown keys reported as missing).
func TestStorymakerAppApiAndAssetsInNode(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-app-api.js")
	if err := os.WriteFile(script, []byte(storymakerApiScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	catalog, _ := filepath.Abs(filepath.Join("img", "storymaker", "catalog.json"))
	out, err := exec.Command(node, script, dir, catalog).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker app api ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerApiScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const [dir, catalogFile] = process.argv.slice(2);
const context = { console, AbortController, Response, Headers, setTimeout, clearTimeout, JSON, Promise, Map, Set, Array, Object, String, Number, Math, URL };
context.globalThis = context; context.window = context;
context.document = { documentElement: { lang: 'en' } };
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-api.js', 'storymaker-assets.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const catalogText = fs.readFileSync(catalogFile, 'utf8');
const catalog = JSON.parse(catalogText);

// A fake server: records every request, answers from a table of routes.
const calls = [];
let answers = {};
function fetcher(url, init) {
  calls.push({ url, method: init.method, headers: init.headers, body: init.body, credentials: init.credentials });
  const key = init.method + ' ' + url;
  const answer = answers[key];
  if (!answer) return Promise.resolve(new Response('{"error":"not_found","code":"not_found","message":"no route"}', { status: 404, headers: { 'Content-Type': 'application/json' } }));
  return Promise.resolve(typeof answer === 'function' ? answer() : answer);
}
// every answer is a factory: a Response body can be read once
const json = (value, status, headers) => () => new Response(JSON.stringify(value), { status: status || 200, headers: Object.assign({ 'Content-Type': 'application/json' }, headers || {}) });
const api = S.createApi({ fetch: fetcher });
const base = '/api/desktop/storymaker';
const id = '11111111-2222-4333-8444-555555555555';

async function rejects(promise, check, what) {
  try { await promise; } catch (err) { check(err); return; }
  throw new Error(what + ': did not reject');
}

(async () => {
  // writes carry their preconditions
  answers['PUT ' + base + '/projects/' + id + '/story'] = json({ etag: '"b"', issues: [], story: {} });
  await api.saveStory(id, '{"schema":"storymaker/1"}', '"a"');
  let call = calls.pop();
  eq([call.method, call.headers['If-Match'], call.headers['Content-Type'], call.body, call.credentials], ['PUT', '"a"', 'application/json', '{"schema":"storymaker/1"}', 'same-origin'], 'story save');

  answers['PUT ' + base + '/projects/' + id + '/scenes/s3/code'] = json({ etag: '"c"', ok: true, errors: [] });
  await api.putCode(id, 's3', 'export default function scene(api) {}', null);
  call = calls.pop();
  eq([call.headers['If-None-Match'], 'If-Match' in call.headers, call.headers['Content-Type']], ['*', false, 'text/javascript'], 'a new code scene is created, not replaced');
  await api.putCode(id, 's3', 'x', '"c"');
  call = calls.pop();
  eq([call.headers['If-Match'], 'If-None-Match' in call.headers], ['"c"', false], 'an existing code scene is replaced with its etag');

  answers['POST ' + base + '/projects/' + id + '/render'] = json({ job: { id: 'j1', status: 'queued' } }, 202);
  const job = await api.startRender(id, '"b"', 'key-1');
  call = calls.pop();
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

  // the flat error shape
  answers['PUT ' + base + '/projects/' + id + '/story'] = json({ error: 'invalid_story', code: 'invalid_story', message: 'The story has errors.', issues: [{ severity: 'error', code: 'invalid_json', path: '', message: 'm' }] }, 422);
  await rejects(api.saveStory(id, '{', '"a"'), err => eq([err.status, err.code, err.issues.length, err.network], [422, 'invalid_story', 1, false], 'a 422 carries its issues'), 'invalid story');
  answers['PUT ' + base + '/projects/' + id + '/story'] = json({ error: 'precondition_failed', code: 'precondition_failed', message: 'stale' }, 412);
  await rejects(api.saveStory(id, '{}', '"old"'), err => eq(err.status, 412, 'a stale etag'), 'conflict');
  answers['GET ' + base + '/status'] = json({ error: 'storymaker_disabled', code: 'storymaker_disabled', message: 'off' }, 403);
  await rejects(api.status(), err => eq([err.status, err.code], [403, 'storymaker_disabled'], 'disabled'), 'disabled');
  await rejects(S.createApi({ fetch: () => Promise.reject(new TypeError('Failed to fetch')) }).status(), err => eq([err.network, err.status], [true, 0], 'a network failure'), 'network');
  const t = key => key;
  eq(S.errors.describe(t, { status: 412, code: 'precondition_failed' }), 'storymaker.error.conflict', 'text for a conflict');
  eq(S.errors.describe(t, { status: 428 }), 'storymaker.error.required', 'text for a missing precondition');
  eq(S.errors.describe(t, { status: 401, code: 'unauthorized' }), 'storymaker.error.unauthorized', 'text for signed out');
  eq(S.errors.describe(t, { status: 403, code: 'desktop_readonly' }), 'storymaker.error.readonly', 'text for read-only');
  eq(S.errors.describe(t, { status: 500, code: 'internal', message: 'boom' }), 'storymaker.error.generic boom', 'text for the rest keeps the server detail');
  eq(S.errors.describe(t, { status: 503, code: 'ffmpeg_unavailable' }), 'storymaker.job_error.ffmpeg_unavailable', 'a refused render start names the missing part');
  eq(S.errors.describe(t, { status: 429, code: 'busy' }), 'storymaker.error.busy', 'busy');
  eq(S.errors.describe(t, { status: 422, code: 'invalid_story' }), 'storymaker.error.invalid_story', 'a refused save names the story');
  eq(S.errors.describe(t, { status: 422, code: 'invalid_story' }, 'render'), 'storymaker.job_error.invalid_story', 'a refused export names the story');
  eq(S.errors.describe(t, { status: 422, code: 'code_error' }, 'render'), 'storymaker.job_error.code_error', 'a refused export names the code scene');
  eq(S.errors.describe(t, { status: 422, code: 'invalid_music_file' }), 'storymaker.error.invalid_music_file', 'a refused music file has its own sentence');
  eq(S.errors.describe(t, { status: 409, code: 'music_unavailable' }), 'storymaker.error.music_unavailable', 'missing music generation has its own sentence');

  // a missing bundle is null, not an error
  answers['GET ' + base + '/projects/' + id + '/scenes/s3/bundle'] = json({ error: 'not_found' }, 404);
  eq(await api.getBundle(id, 's3'), null, 'no bundle');

  // the asset resolver: the catalog files go out, everything else is missing
  const staticFiles = { '/img/storymaker/catalog.json': catalogText, '/img/storymaker/glyphs.json': fs.readFileSync(path.join(path.dirname(catalogFile), 'glyphs.json'), 'utf8') };
  const served = [];
  function staticFetch(url, init) {
    const hit = url.indexOf('/img/storymaker/') === 0 && url !== '/img/storymaker/catalog.json';
    served.push(url);
    if (staticFiles[url]) return Promise.resolve(new Response(staticFiles[url], { status: 200, headers: { 'Content-Type': 'application/json' } }));
    if (hit) return Promise.resolve(new Response(JSON.stringify({ file: url }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    if (url === '/fonts/qr/NotoSansCJKsc.otf') return Promise.resolve(new Response(new Uint8Array(9 * 1048576), { status: 200 }));
    if (url.indexOf('/fonts/') === 0) return Promise.resolve(new Response(new Uint8Array([1, 2, 3, 4]), { status: 200 }));
    return fetcher(url, init);
  }
  const assets = S.createAssets(S.createApi({ fetch: staticFetch }));
  const story = { style: { font: 'default' }, scenes: [{ id: 's1' }, { id: 's3', kind: 'code' }] };
  answers['GET ' + base + '/projects/' + id + '/scenes/s3/bundle'] = () => new Response('self.__smScene = function () {};', { status: 200, headers: { 'Content-Type': 'text/javascript' } });
  const prepared = await assets.prepare(story, id);
  eq([prepared.catalog.version, Object.keys(prepared.assets), prepared.fonts.map(f => f.weight), prepared.fonts[0].family, Object.keys(prepared.code_bundles)],
    [catalog.version, ['catalog:' + catalog.rig.file], [400, 500, 600, 700], 'Geist', ['s3']], 'the first load');
  eq(prepared.fonts[0].data.byteLength, 4, 'font bytes arrive as an ArrayBuffer');

  const props = catalog.entries.find(e => e.kind === 'prop');
  answers['GET ' + base + '/projects/' + id + '/scenes/s3/bundle'] = () => new Response('self.__smScene = 2;', { status: 200, headers: { 'Content-Type': 'text/javascript' } });
  const result = await assets.resolve([
    'catalog:' + props.file, 'catalog:../../etc/passwd', 'catalog:rig/other.json', 'font:default/400', 'font:default/450', 'font:nope/400',
    'bundle:s3', 'bundle:s1', 'bundle:s9', 'nonsense', 'catalog:' + 'x'.repeat(300)
  ], story, id);
  eq(Object.keys(result.assets).sort(), ['bundle:s3', 'catalog:' + props.file, 'font:default/400'].sort(), 'what is served');
  eq(result.missing.length, 8, 'everything else is reported missing, none silently');
  eq(served.some(u => u.indexOf('passwd') >= 0 || u.indexOf('other.json') >= 0), false, 'a path the catalog does not list is never requested');
  eq(result.assets['bundle:s3'], 'self.__smScene = 2;', 'the bundle is fetched fresh');
  // fallback faces: the coverage file travels on the catalog, and a face resolves with its own byte limit
  eq(prepared.catalog.glyph_data.schema, 'storymaker-glyphs/1', 'the coverage file is attached to the catalog');
  const faces = await assets.resolve(['font:sm-cjk/400', 'font:sm-greek/400', 'font:sm-cjk/700', 'font:sm-nope/400'], story, id);
  eq(Object.keys(faces.assets).sort(), ['font:sm-cjk/400', 'font:sm-greek/400'], 'both faces resolve');
  eq(faces.assets['font:sm-cjk/400'].byteLength, 9 * 1048576, 'the CJK face passes over 8 MiB through its max_bytes');
  eq(faces.missing.sort(), ['font:sm-cjk/700', 'font:sm-nope/400'], 'a face answers only at weight 400');
  await rejects(S.createApi({ fetch: staticFetch }).fontBuffer('/fonts/qr/NotoSansCJKsc.otf'), err => eq(err.message, 'The font is too large.', 'the default cap stays 8 MiB'), 'font cap');
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
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`
