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

  // a missing bundle is null, not an error
  answers['GET ' + base + '/projects/' + id + '/scenes/s3/bundle'] = json({ error: 'not_found' }, 404);
  eq(await api.getBundle(id, 's3'), null, 'no bundle');

  // the asset resolver: the catalog files go out, everything else is missing
  const staticFiles = { '/img/storymaker/catalog.json': catalogText };
  const served = [];
  function staticFetch(url, init) {
    const hit = url.indexOf('/img/storymaker/') === 0 && url !== '/img/storymaker/catalog.json';
    served.push(url);
    if (staticFiles[url]) return Promise.resolve(new Response(staticFiles[url], { status: 200, headers: { 'Content-Type': 'application/json' } }));
    if (hit) return Promise.resolve(new Response(JSON.stringify({ file: url }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
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
  console.log('storymaker app api ok');
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`
