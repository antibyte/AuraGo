package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Proxies (2a): a finished video import queues one preview job per project at a time, the state follows the job, a
// failed proxy and Full quality play the original, the job list adopts earlier proxies without backfilling, and the
// jobs list never shows proxy or range-render jobs.
func TestVideoStudioProxyQueue(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio proxy checks")
	}
	script := `
const assert = require('node:assert/strict');
if (!globalThis.crypto) globalThis.crypto = require('node:crypto').webcrypto;
global.window = {setTimeout: fn => setTimeout(fn, 1)};
global.location = {origin: 'http://studio.test'};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-core.js');
require('./js/desktop/apps/video-studio-jobs.js');
const P = window.VideoStudioParts;
const API = '/api/desktop/video-studio';
let renders = 0, projectReads = 0, nextJob = 1, listed = [];
const posts = [], server = new Map();
P.renderAssets = () => { renders++; };
P.renderUI = () => {};
const video = id => ({id, name: id + '.mp4', path: 'media/' + id, kind: 'video', duration_frames: 90, duration_us: 3000000, video_duration_us: 3000000, width: 1280, height: 720, has_audio: false});
const projectDoc = () => ({version: 3, name: 'Proxies', width: 1280, height: 720, fps: 30, tracks: [], assets: [video('a'), video('b'), video('c'), video('d'),
  {id: 'i', name: 'i.png', path: 'media/i', kind: 'image', duration_frames: 0, width: 64, height: 36, has_audio: false},
  {id: 'u', name: 'u.wav', path: 'media/u', kind: 'audio', duration_frames: 60, duration_us: 2000000, has_audio: true}]});
global.fetch = async (url, options = {}) => {
  const method = (options.method || 'GET').toUpperCase();
  let body = {};
  if (method === 'POST' && url === API + '/projects/p/jobs') {
    const request = JSON.parse(options.body);
    posts.push(request);
    const job = {id: 'px' + nextJob++, project_id: 'p', kind: request.kind, status: 'queued', progress: 0};
    server.set(job.id, job);
    body = {job: Object.assign({}, job)};
  } else if (url.startsWith(API + '/jobs/')) body = {job: Object.assign({}, server.get(url.split('/').pop()))};
  else if (url === API + '/jobs?project_id=p') body = {jobs: listed.map(job => Object.assign({}, job))};
  else if (url === API + '/projects/p') { projectReads++; body = {project: projectDoc()}; }
  return {ok: true, status: method === 'POST' ? 202 : 200, headers: {get: name => name.toLowerCase() === 'content-type' ? 'application/json' : null}, json: async () => body};
};
const s = {projectId: 'p', projectEpoch: 1, abort: {}, disposed: false, readonly: false, dirty: false, revision: 0, previewQuality: 'auto', jobs: [], jobLocalStops: new Set(), notifiedJobs: new Set(), q: () => null, ctx: {}};
s.project = P.hydrateProject(s, projectDoc());
s.savedProject = P.canonicalProject(s.project);
const assetOf = id => s.project.assets.find(item => item.id === id);
const finished = (kind, asset, status = 'succeeded') => ({id: kind + '-' + asset.id, project_id: 'p', kind, status, result: {asset}});
const succeed = (id, assetId) => Object.assign(server.get(id), {status: 'succeeded', progress: 1, result: {asset_id: assetId}, artifact: {name: 'preview.mp4', download_url: API + '/projects/p/previews/' + id, size: 10}});
const until = async (test, message) => {
  for (let i = 0; i < 500; i++) { if (test()) return; await new Promise(resolve => setTimeout(resolve, 2)); }
  throw new Error(message + ': ' + JSON.stringify({posts, states: ['a', 'b', 'c', 'd'].map(id => P.proxyState(s, id))}));
};
(async () => {
  // A finished video import queues its proxy; a second one waits in the editor while the first is in flight.
  assert.equal(P.noteFinishedJob(s, finished('probe', {id: 'a', kind: 'video'})), true);
  assert.equal(P.proxyState(s, 'a'), 'queued');
  await until(() => posts.length === 1, 'the first proxy was not requested');
  assert.deepEqual(posts[0], {kind: 'preview', asset_id: 'a'});
  assert.equal(P.noteFinishedJob(s, finished('generate', {id: 'b', kind: 'video'})), true);
  assert.equal(P.proxyState(s, 'b'), 'queued');
  await new Promise(resolve => setTimeout(resolve, 30));
  assert.equal(posts.length, 1, 'at most one preview job of the project is queued or running');
  // Images, audio, failed imports, other job kinds and duplicates never queue.
  assert.equal(P.noteFinishedJob(s, finished('probe', {id: 'i', kind: 'image'})), false);
  assert.equal(P.noteFinishedJob(s, finished('probe', {id: 'u', kind: 'audio'})), false);
  assert.equal(P.noteFinishedJob(s, finished('probe', {id: 'c', kind: 'video'}, 'failed')), false);
  assert.equal(P.noteFinishedJob(s, finished('still', {id: 'c', kind: 'video'})), false);
  assert.equal(P.queueProxy(s, 'i'), false, 'the item menu refuses an image');
  assert.equal(P.queueProxy(s, 'u'), false, 'the item menu refuses audio');
  assert.equal(P.queueProxy(s, 'a'), false, 'a queued proxy is not requested twice');
  assert.equal(P.proxyState(s, 'i'), '');
  // The state follows the job. Auto, Half and Quarter play a ready proxy; Full, images and unready proxies the original.
  server.get('px1').status = 'running';
  await until(() => P.proxyState(s, 'a') === 'running', 'running was not picked up');
  assert.equal(P.previewMediaURL(s, assetOf('a')), assetOf('a').media_url, 'a running proxy plays the original');
  succeed('px1', 'a');
  await until(() => P.proxyState(s, 'a') === 'ready', 'ready was not picked up');
  const proxyURL = 'http://studio.test' + API + '/projects/p/previews/px1';
  for (const quality of ['auto', 'half', 'quarter']) { s.previewQuality = quality; assert.equal(P.previewMediaURL(s, assetOf('a')), proxyURL, quality + ' plays the proxy'); }
  s.previewQuality = 'full';
  assert.equal(P.previewMediaURL(s, assetOf('a')), assetOf('a').media_url, 'full plays the original');
  s.previewQuality = 'auto';
  assert.equal(P.previewMediaURL(s, assetOf('i')), assetOf('i').media_url, 'an image plays its original');
  assert(renders > 0, 'state changes redraw the library');
  // The finished proxy frees the slot; the waiting one fails and plays the original; the item menu retries it.
  await until(() => posts.length === 2, 'the waiting proxy was not requested');
  assert.deepEqual(posts[1], {kind: 'preview', asset_id: 'b'});
  Object.assign(server.get('px2'), {status: 'failed', error: 'preview_failed'});
  await until(() => P.proxyState(s, 'b') === 'failed', 'the failure was not picked up');
  assert.equal(P.previewMediaURL(s, assetOf('b')), assetOf('b').media_url, 'a failed proxy plays the original');
  assert.equal(P.queueProxy(s, 'b'), true, 'Create preview proxy retries a failed one');
  await until(() => posts.length === 3 && posts[2].asset_id === 'b', 'the retry was not requested');
  succeed('px3', 'b');
  await until(() => P.proxyState(s, 'b') === 'ready', 'the retry did not finish');
  // The jobs list hides proxies and range renders.
  assert.deepEqual(P.visibleJobs({jobs: [{id: '1', kind: 'preview', status: 'running'}, {id: '2', kind: 'range_render', status: 'queued'}, {id: '3', kind: 'render', status: 'queued'}], jobLocalStops: new Set()}).map(job => job.kind), ['render']);
  // Job list: a finished preview job never reloads the project, and a ready proxy of an earlier session is adopted.
  const oldC = {id: 'old-c', project_id: 'p', kind: 'preview', status: 'succeeded', result: {asset_id: 'c'}, artifact: {download_url: API + '/projects/p/previews/old-c'}};
  listed = [oldC, server.get('px3'), server.get('px2'), server.get('px1')];
  await P.loadJobs(s);
  assert.equal(projectReads, 0, 'a finished preview job does not reload the project');
  assert.equal(P.proxyState(s, 'c'), 'ready');
  assert.equal(P.previewMediaURL(s, assetOf('c')), 'http://studio.test' + API + '/projects/p/previews/old-c');
  // A probe that was already done when the list first showed it gets no proxy (no backfill on open)...
  listed = [{id: 'probe-old', project_id: 'p', kind: 'probe', status: 'succeeded', result: {asset: {id: 'd', kind: 'video'}}}].concat(listed);
  await P.loadJobs(s);
  await until(() => projectReads === 1, 'the finished probe did not reload the project');
  assert.equal(P.proxyState(s, 'd'), '', 'media imported before this session gets no proxy on open');
  // ...one seen running and then done does, once another tab's preview job has left the worker.
  const external = {id: 'ext', project_id: 'p', kind: 'preview', status: 'running'};
  const probeNew = {id: 'probe-new', project_id: 'p', kind: 'probe', status: 'running'};
  listed = [external, probeNew].concat(listed);
  await P.loadJobs(s);
  Object.assign(probeNew, {status: 'succeeded', result: {asset: {id: 'd', kind: 'video'}}});
  await P.loadJobs(s);
  assert.equal(P.proxyState(s, 'd'), 'queued');
  await new Promise(resolve => setTimeout(resolve, 30));
  assert.equal(posts.length, 3, 'a preview job of another tab holds the queue');
  external.status = 'cancelled';
  await P.loadJobs(s);
  await until(() => posts.length === 4 && posts[3].asset_id === 'd', 'the queued proxy was not requested once the worker was free');
  // A replaced proxy (its job lost the artifact) is dropped: the original plays.
  Object.assign(oldC, {artifact: null, message: 'replaced'});
  await P.loadJobs(s);
  assert.equal(P.proxyState(s, 'c'), '', 'a replaced proxy is dropped');
  // Read-only editors request nothing; another project starts with an empty store.
  s.readonly = true;
  assert.equal(P.queueProxy(s, 'zz'), false);
  s.readonly = false;
  s.projectEpoch = 2;
  assert.equal(P.proxyState(s, 'a'), '', 'a new project epoch starts an empty proxy store');
  s.disposed = true;
})().catch(error => { console.error(error); process.exit(1); });
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("proxy checks: %v\n%s", err, output)
	}
}

// The stage plays proxies, the source monitor originals; both job pollers report finished jobs; the item menu and the
// library are wired; every locale has the proxy strings.
func TestVideoStudioProxyWiring(t *testing.T) {
	read := func(path string) string {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	apps := filepath.Join("js", "desktop", "apps")
	if !strings.Contains(read(filepath.Join(apps, "video-studio.js")), "mediaURL: asset => P.previewMediaURL(s, asset)") {
		t.Error("the stage does not play proxies")
	}
	if strings.Contains(read(filepath.Join(apps, "video-studio-source.js")), "previewMediaURL") {
		t.Error("the source monitor must play originals")
	}
	if n := strings.Count(read(filepath.Join(apps, "video-studio-jobs.js")), "noteFinishedJob(s, job)"); n < 2 {
		t.Errorf("noteFinishedJob is called %d times; pollJob and loadJobs both report finished jobs", n)
	}
	if !strings.Contains(read(filepath.Join(apps, "video-studio-mediabin.js")), "P.queueProxy(s, assetId)") {
		t.Error("the item menu cannot create a preview proxy")
	}
	if !strings.Contains(read(filepath.Join(apps, "video-studio-library.js")), `data-proxy-state="${state}"`) {
		t.Error("the library shows no proxy state")
	}
	files, err := filepath.Glob(filepath.Join("lang", "desktop", "*.json"))
	if err != nil || len(files) != 16 {
		t.Fatalf("desktop locales: %d files, %v", len(files), err)
	}
	for _, file := range files {
		data := read(file)
		for _, key := range []string{"proxyQueued", "proxyPreparing", "proxyReady", "proxyFailed", "createProxy"} {
			if !strings.Contains(data, `"videoStudio.`+key+`"`) {
				t.Errorf("%s lacks videoStudio.%s", file, key)
			}
		}
	}
}
