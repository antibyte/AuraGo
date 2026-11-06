package ui

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

// storymakerHandoffPrelude loads the app core, the model and the handoff module into one context. The core gives S.uuid and
// S.errors, the model gives S.model.exportSize, and no module touches the DOM while it loads.
const storymakerHandoffPrelude = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const dir = process.argv[2];
const context = { console };
context.globalThis = context; context.window = context;
context.document = { documentElement: { lang: 'en' } };
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-handoff.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
`

// runStorymakerHandoffNode runs the prelude plus body in Node and returns what the script printed.
func runStorymakerHandoffNode(t *testing.T, body string) []byte {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-handoff-check.js")
	if err := os.WriteFile(script, []byte(storymakerHandoffPrelude+body), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(node, script, dir)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("node check failed: %v\n%s", err, stderr.String())
	}
	return stdout.Bytes()
}

// decodeStorymakerHandoffProject reads a project the way the PUT handler does (unknown fields refused) and runs the Go validator.
func decodeStorymakerHandoffProject(t *testing.T, raw json.RawMessage) videostudio.Project {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var project videostudio.Project
	if err := decoder.Decode(&project); err != nil {
		t.Fatalf("the project does not decode strictly: %v\n%s", err, raw)
	}
	if err := videostudio.Validate(project); err != nil {
		t.Fatalf("videostudio.Validate refuses the project: %v\n%s", err, raw)
	}
	return project
}

// The tracks from Node pass the strict decoder and the Go validator; durations never pass the asset or the film; a silent stem has
// no track.
func TestStorymakerHandoffTracksInNode(t *testing.T) {
	t.Parallel()
	out := runStorymakerHandoffNode(t, storymakerHandoffTracksScript)
	var printed struct {
		Full   json.RawMessage `json:"full"`
		Silent json.RawMessage `json:"silent"`
	}
	if err := json.Unmarshal(out, &printed); err != nil {
		t.Fatalf("node output is not JSON: %v\n%s", err, out)
	}
	full := decodeStorymakerHandoffProject(t, printed.Full)
	silent := decodeStorymakerHandoffProject(t, printed.Silent)

	// The film has 250 frames, so each duration is min(asset frames, 250): video 300 -> 250, narration 180 -> 180,
	// music 400 -> 250, sfx 250 -> 250.
	want := []struct {
		kind     videostudio.TrackKind
		name     string
		asset    string
		duration int
	}{
		{videostudio.TrackVideo, "Video track", "a-video", 250},
		{videostudio.TrackAudio, "Narration track", "a-narration", 180},
		{videostudio.TrackAudio, "Music track", "a-music", 250},
		{videostudio.TrackAudio, "Effects track", "a-sfx", 250},
	}
	if len(full.Tracks) != len(want) {
		t.Fatalf("got %d tracks, want %d", len(full.Tracks), len(want))
	}
	for i, w := range want {
		track := full.Tracks[i]
		if track.Kind != w.kind || track.Name != w.name || len(track.Clips) != 1 || !strings.HasPrefix(track.ID, "track-") {
			t.Fatalf("track %d = %+v", i, track)
		}
		clip := track.Clips[0]
		if clip.AssetID != w.asset || clip.Duration != w.duration || clip.Start != 0 || clip.Offset != 0 || !strings.HasPrefix(clip.ID, "clip-") {
			t.Errorf("track %d clip = %+v", i, clip)
		}
		// Only the picture detaches its audio: the stems carry the sound.
		if clip.AudioDetached != (w.kind == videostudio.TrackVideo) {
			t.Errorf("track %d audio_detached = %v", i, clip.AudioDetached)
		}
	}

	// Music and sfx were silent, so their stems do not exist and they get no track.
	if len(silent.Tracks) != 2 || silent.Tracks[0].Kind != videostudio.TrackVideo || silent.Tracks[1].Clips[0].AssetID != "a-narration" {
		t.Fatalf("tracks without silent stems = %+v", silent.Tracks)
	}
}

// The whole handoff against a fake fetch and a fake timer: the order of the calls, the import keys, the poll, the PUT with the fresh
// ETag and the launch. canOffer decides on the job and the Video Studio status.
func TestStorymakerHandoffRunInNode(t *testing.T) {
	t.Parallel()
	out := runStorymakerHandoffNode(t, storymakerHandoffRunScript)
	if !strings.Contains(string(out), "storymaker handoff ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

// The export panel offers the handoff through the module and honours the wish from the dialog. The app tour runs it for real.
func TestStorymakerExportOffersHandoff(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/storymaker-export.js")
	for _, marker := range []string{
		"S.handoff.canOffer(job, vsStatus)",
		"S.handoff.status()",
		"S.handoff.run(inst, job, handoffProjects)",
		"inst.handoffWish.delete(id)",
		"dataset: { stm: 'export-handoff' }",
	} {
		if !strings.Contains(source, marker) {
			t.Errorf("storymaker-export.js lacks %q", marker)
		}
	}
}

// An export that ended while no panel watched it: a replay alone changes nothing, a wish from this session runs the handoff, and the
// job list gives the way back to the result with its paths, warnings and handoff.
func TestStorymakerExportKeepsLastJobInNode(t *testing.T) {
	t.Parallel()
	out := runStorymakerExportScript(t, "storymaker-export-last.js", storymakerExportLastJobScript)
	if !strings.Contains(out, "storymaker export last job ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerExportLastJobScript = `
load(['storymaker-core.js', 'storymaker-model.js', 'storymaker-handoff.js', 'storymaker-export.js']);
const S = context.StorymakerParts;
// The real canOffer decides. Only the two calls that leave the page are stand-ins.
const runs = [];
S.handoff.status = () => Promise.resolve({ enabled: true, desktop_enabled: true, ffmpeg_ready: true, read_only: false });
S.handoff.run = (inst, job) => { runs.push(job.id); return Promise.resolve({ projectId: 'vs1' }); };
const mp4 = 'Documents/Storymaker/p1/exports/film-12345678.mp4';
const finished = id => ({ id, project_id: 'p1', kind: 'render', status: 'succeeded', progress: 1, frames_total: 90, output: { path: mp4, size: 10, stems: { narration: 'Documents/Storymaker/p1/exports/film-12345678/narration.wav' } }, warnings: [{ code: 'library_failed', message: 'Videos is read-only' }] });
const ended = (id, status) => ({ id, project_id: 'p1', kind: 'render', status });
const answering = jobs => () => Promise.resolve(jobs);
function setup(listJobs) {
  return {
    t: key => key, domId: name => 'test-' + name, bus: S.createBus(), parts: {}, readonly: false, projectId: 'p1', etag: '"e1"', issues: [], handoffWish: new Set(),
    ctx: { openApp() {} },
    status: { ffmpeg: { ready: true }, browser: { ready: true, source: 'system' }, limits: {} },
    story: { format: { aspect: '16:9' }, style: { captions: true }, music: null },
    report: { frame_count: 90, fps: 30, duration_ms: 3000, issues: [], audio_events: [] },
    audioInfo: { narration: {}, missing: [], chars: { total: 0, missing: 0 }, music: null },
    flushEdits: () => Promise.resolve(true), showIssues() {}, notify() {},
    api: { listJobs }
  };
}
(async () => {
  // A finished job that the stream replays without a wish leaves the panel idle, and an empty list offers nothing.
  let inst = setup(answering([]));
  let exporter = S.createExport(inst);
  let root = exporter.el;
  await settle();
  inst.bus.emit('job', finished('old'));
  await settle();
  eq([exporter.job(), one(root, 'export').dataset.state], [null, 'idle'], 'a replayed job without a wish stays out');
  exporter.open();
  one(root, 'export-ready');
  eq(byStm(root, 'export-last').length, 0, 'an empty list offers nothing');
  eq(runs, [], 'nothing runs');
  exporter.dispose();

  // A replayed finished job with a wish runs the handoff once, and the wish is used up.
  inst = setup(answering([]));
  inst.handoffWish.add('wished');
  exporter = S.createExport(inst);
  await settle();
  inst.bus.emit('job', finished('wished'));
  await settle();
  eq(runs, ['wished'], 'a wish from this session runs on the way back');
  eq(inst.handoffWish.size, 0, 'the wish is used up');
  inst.bus.emit('job', finished('wished'));
  await settle();
  eq(runs, ['wished'], 'a second replay runs nothing');
  exporter.dispose();

  // The replay can miss a wished job, the list cannot.
  runs.length = 0;
  inst = setup(answering([finished('listed')]));
  inst.handoffWish.add('listed');
  exporter = S.createExport(inst);
  await settle();
  eq(runs, ['listed'], 'the list also runs a wish');
  exporter.dispose();

  // The way back: the newest render that left a result. Narrate jobs and renders that ended without one do not count.
  runs.length = 0;
  inst = setup(answering([{ id: 'n1', project_id: 'p1', kind: 'narrate', status: 'succeeded' }, ended('r4', 'failed'), ended('r3', 'cancelled'), ended('r2', 'running'), finished('job-1'), finished('job-0')]));
  exporter = S.createExport(inst);
  root = exporter.el;
  exporter.open();
  eq(byStm(root, 'export-last').length, 0, 'the list has not answered yet');
  await settle();
  one(root, 'export-ready');
  one(root, 'export-last').click();
  eq(exporter.job().id, 'job-1', 'the newest one');
  one(root, 'export-done');
  eq(one(root, 'export-path').textContent, mp4, 'its path');
  one(root, 'export-extras');
  one(root, 'export-warnings');
  eq(runs, [], 'the way back starts nothing by itself');
  await settle();
  one(root, 'export-handoff').click();
  await settle();
  eq(runs, ['job-1'], 'the handoff is one click away');
  one(root, 'export-again').click();
  eq(exporter.job(), null, 'back at the start view');
  one(root, 'export-last');
  exporter.dispose();

  // An interrupted export says so on the way back.
  inst = setup(answering([ended('cut', 'interrupted'), finished('older')]));
  exporter = S.createExport(inst);
  root = exporter.el;
  await settle();
  exporter.open();
  one(root, 'export-last').click();
  eq(one(root, 'export-failed').dataset.status, 'interrupted', 'the interrupted text shows');
  exporter.dispose();

  // A list that fails offers nothing and breaks nothing.
  inst = setup(() => Promise.reject(new Error('offline')));
  exporter = S.createExport(inst);
  root = exporter.el;
  await settle();
  exporter.open();
  one(root, 'export-ready');
  eq(byStm(root, 'export-last').length, 0, 'a failed list offers nothing');
  exporter.dispose();

  // A late answer does nothing for a panel that is gone or a project that was left.
  runs.length = 0;
  let release = null;
  inst = setup(() => new Promise(resolve => { release = resolve; }));
  inst.handoffWish.add('late');
  exporter = S.createExport(inst);
  exporter.dispose();
  release([finished('late')]);
  await settle();
  eq(runs, [], 'a disposed panel starts nothing');
  eq(inst.handoffWish.size, 1, 'the wish stays for the next panel');
  inst = setup(() => new Promise(resolve => { release = resolve; }));
  exporter = S.createExport(inst);
  root = exporter.el;
  exporter.open();
  inst.projectId = 'p2';
  release([finished('late')]);
  await settle();
  eq(byStm(root, 'export-last').length, 0, 'an answer for the project before stays out');
  exporter.dispose();

  // A wished job that ended without a film shows once and gives up the wish, so the next open of the project is clean.
  inst = setup(answering([]));
  inst.handoffWish.add('stopped');
  exporter = S.createExport(inst);
  await settle();
  inst.bus.emit('job', ended('stopped', 'cancelled'));
  eq(exporter.job().id, 'stopped', 'the ending of a wished job shows');
  eq(inst.handoffWish.size, 0, 'the wish ends with the job');
  exporter.dispose();
  exporter = S.createExport(inst);
  await settle();
  inst.bus.emit('job', ended('stopped', 'cancelled'));
  eq(exporter.job(), null, 'a later replay stays out');
  exporter.dispose();

  // A failed stem: the wish runs nothing, no action is offered, and the panel says why.
  runs.length = 0;
  inst = setup(answering([]));
  const notices = [];
  inst.notify = (text, kind) => notices.push([text, kind]);
  inst.handoffWish.add('holed');
  exporter = S.createExport(inst);
  root = exporter.el;
  await settle();
  inst.bus.emit('job', Object.assign(finished('holed'), { warnings: [{ code: 'stem_failed', message: 'music.wav could not be written' }] }));
  await settle();
  exporter.open();
  eq(runs, [], 'a failed stem starts no handoff');
  eq(notices, [['storymaker.handoff.unavailable', 'error']], 'the wish learns why');
  eq(byStm(root, 'export-handoff').length, 0, 'no action');
  eq(one(root, 'export-handoff-unavailable').textContent, 'storymaker.handoff.unavailable', 'the panel says why');
  exporter.dispose();

  // A failed run keeps its project for the next click.
  const kept = [];
  S.handoff.run = (_, job, projects) => { kept.push(projects.get(job.id) || ''); projects.set(job.id, 'vs1'); return Promise.reject(new Error('timed out')); };
  inst = setup(answering([finished('retry')]));
  exporter = S.createExport(inst);
  root = exporter.el;
  await settle();
  exporter.open();
  one(root, 'export-last').click();
  await settle();
  one(root, 'export-handoff').click();
  await settle();
  eq(one(root, 'export-handoff-error').textContent, 'timed out', 'the failure shows');
  one(root, 'export-handoff').click();
  await settle();
  eq(kept, ['', 'vs1'], 'the retry finds the project of the first try');
  exporter.dispose();

  console.log('storymaker export last job ok');
})().catch(fail);
`

const storymakerHandoffRunScript = `
const API = '/api/desktop/video-studio';
const calls = [];
const delays = [];
const opened = [];
const polled = {};
let stuck = false;
let broken = false;
// The timer fires at once and records its delay, so 120 s of polling takes no real time.
context.setTimeout = (fn, ms) => { delays.push(ms); fn(); return 0; };
const answer = (value, status, headers) => new Response(JSON.stringify(value), { status: status || 200, headers: Object.assign({ 'Content-Type': 'application/json' }, headers || {}) });
const stored = {
  version: 3, name: 'Firewall', width: 1080, height: 1920, fps: 25,
  assets: [
    { id: 'a-video', name: 'film-job1.mp4', path: 'media/a-video_film-job1.mp4', kind: 'video', duration_frames: 300, width: 1080, height: 1920, has_audio: true },
    { id: 'a-narration', name: 'narration.wav', folder: 'Sound', path: 'media/a-narration_narration.wav', kind: 'audio', duration_frames: 180, has_audio: true },
    { id: 'a-sfx', name: 'sfx.wav', path: 'media/a-sfx_sfx.wav', kind: 'audio', duration_frames: 400, has_audio: true }
  ],
  tracks: []
};
context.fetch = (url, init) => {
  calls.push({ url, method: init.method, headers: init.headers, body: init.body, credentials: init.credentials });
  const route = init.method + ' ' + url;
  if (route === 'POST ' + API + '/projects') return Promise.resolve(answer({ id: 'vs1', project: stored, etag: '"created"' }, 201, { ETag: '"created"' }));
  if (route === 'POST ' + API + '/projects/vs1/media') {
    const name = path.basename(JSON.parse(init.body).source_path).replace(/\.wav$/, '').replace(/^film.*$/, 'video');
    return Promise.resolve(answer({ asset: { id: 'a-' + name }, media_url: 'x', job: { id: 'probe-' + name, status: 'queued' } }, 202));
  }
  if (init.method === 'GET' && url.startsWith(API + '/jobs/')) {
    const id = url.slice((API + '/jobs/').length);
    polled[id] = (polled[id] || 0) + 1;
    // The first job answers running once, so the poll has to repeat.
    const running = stuck || (id === 'probe-video' && polled[id] === 1);
    return Promise.resolve(answer({ id, status: broken ? 'failed' : running ? 'running' : 'succeeded' }));
  }
  if (route === 'GET ' + API + '/projects/vs1') return Promise.resolve(answer({ id: 'vs1', project: stored, etag: '"fresh"' }, 200, { ETag: '"fresh"' }));
  if (route === 'PUT ' + API + '/projects/vs1') return Promise.resolve(answer({ id: 'vs1', project: stored, etag: '"saved"' }, 200, { ETag: '"saved"' }));
  return Promise.resolve(answer({ code: 'not_found', message: 'no route ' + route }, 404));
};
const inst = { t: key => key, disposed: false, story: { title: 'Firewall', format: { aspect: '9:16', fps: 25 } }, project: { title: 'Firewall' }, ctx: { openApp: (app, launch) => opened.push([app, launch]) } };
const exportsDir = 'Documents/Storymaker/p1/exports/';
// The music stem was silent, so job.output.stems has no music.
const job = { id: 'job1', status: 'succeeded', frames_total: 250, output: { path: exportsDir + 'film-job1.mp4', size: 9, stems: { narration: exportsDir + 'film-job1/narration.wav', sfx: exportsDir + 'film-job1/sfx.wav' } } };

(async () => {
  const result = await S.handoff.run(inst, job);
  eq(result, { projectId: 'vs1' }, 'the answer');
  eq(calls.map(call => call.method + ' ' + call.url.slice(API.length)), [
    'POST /projects',
    'POST /projects/vs1/media', 'POST /projects/vs1/media', 'POST /projects/vs1/media',
    'GET /jobs/probe-video', 'GET /jobs/probe-video', 'GET /jobs/probe-narration', 'GET /jobs/probe-sfx',
    'GET /projects/vs1',
    'PUT /projects/vs1'
  ], 'the order of the calls');
  eq(calls[0].credentials, 'same-origin', 'the session goes along');
  eq(JSON.parse(calls[0].body), { name: 'Firewall', width: 1080, height: 1920, fps: 25, ntsc: false }, 'canvas and rate follow the story');

  const imports = calls.slice(1, 4);
  eq(imports.map(call => call.headers['Idempotency-Key']), ['storymaker-job1-video', 'storymaker-job1-narration', 'storymaker-job1-sfx'], 'the import keys');
  eq(imports.map(call => JSON.parse(call.body)), [
    { source_path: exportsDir + 'film-job1.mp4', folder: '' },
    { source_path: exportsDir + 'film-job1/narration.wav', folder: '' },
    { source_path: exportsDir + 'film-job1/sfx.wav', folder: '' }
  ], 'the import bodies');
  eq(imports[0].headers['Content-Type'], 'application/json', 'without JSON the server answers 415');

  // Four polls: video twice (running, then succeeded), narration once, sfx once.
  eq(delays, [500, 500, 500, 500], 'one 500 ms wait before each poll');

  const put = calls[calls.length - 1];
  eq(put.headers['If-Match'], '"fresh"', 'If-Match carries the ETag of the last GET');
  const sent = JSON.parse(put.body);
  eq([sent.version, sent.name, sent.width, sent.height, sent.fps], [3, 'Firewall', 1080, 1920, 25], 'the whole project');
  eq(sent.assets, [{ id: 'a-video', name: 'film-job1.mp4' }, { id: 'a-narration', name: 'narration.wav', folder: 'Sound' }, { id: 'a-sfx', name: 'sfx.wav' }], 'only the asset fields the client owns');
  // frames_total is 250: video min(300, 250) = 250, narration min(180, 250) = 180, sfx min(400, 250) = 250.
  eq(sent.tracks.map(track => [track.kind, track.name, track.clips[0].asset_id, track.clips[0].duration]), [
    ['video', 'storymaker.handoff.track_video', 'a-video', 250],
    ['audio', 'storymaker.handoff.track_narration', 'a-narration', 180],
    ['audio', 'storymaker.handoff.track_sfx', 'a-sfx', 250]
  ], 'the tracks');
  eq(opened, [['video-studio', { projectId: 'vs1' }]], 'the launch');

  // A probe that never ends: 120 s at one poll per 500 ms is 120000 / 500 = 240 polls, all on the first job.
  calls.length = 0; delays.length = 0; opened.length = 0;
  Object.keys(polled).forEach(key => delete polled[key]);
  stuck = true;
  let refused = null;
  try { await S.handoff.run(inst, job); } catch (err) { refused = err; }
  if (!refused) throw new Error('a probe that never ends must reject');
  eq(polled, { 'probe-video': 240 }, 'the poll stops at 120 s');
  eq(delays.length, 240, 'one wait per poll');
  eq(delays.every(ms => ms === 500), true, 'every wait is 500 ms');
  eq(calls.some(call => call.method === 'PUT'), false, 'no PUT after a timeout');
  eq(opened, [], 'no launch after a timeout');

  // The panel's Map keeps the project of a failed run. The retry creates none, repeats the imports with the same keys and fills it.
  const projects = new Map();
  refused = null;
  try { await S.handoff.run(inst, job, projects); } catch (err) { refused = err; }
  if (!refused) throw new Error('the failed run must reject');
  eq(Array.from(projects), [['job1', 'vs1']], 'the failed run keeps its project');
  calls.length = 0; opened.length = 0;
  stuck = false;
  eq(await S.handoff.run(inst, job, projects), { projectId: 'vs1' }, 'the retry fills that project');
  eq(calls.map(call => call.method + ' ' + call.url.slice(API.length)).filter(route => !route.startsWith('GET /jobs/')), [
    'POST /projects/vs1/media', 'POST /projects/vs1/media', 'POST /projects/vs1/media',
    'GET /projects/vs1',
    'PUT /projects/vs1'
  ], 'the retry creates no project');
  eq(calls.slice(0, 3).map(call => call.headers['Idempotency-Key']), ['storymaker-job1-video', 'storymaker-job1-narration', 'storymaker-job1-sfx'], 'the same import keys');
  eq(opened, [['video-studio', { projectId: 'vs1' }]], 'the retry opens it');
  eq(projects.size, 0, 'a finished handoff forgets the project, so a later click cannot overwrite edits made in Video Studio');

  // A kept project that was deleted in Video Studio is given up once.
  calls.length = 0;
  projects.set('job1', 'gone');
  eq(await S.handoff.run(inst, job, projects), { projectId: 'vs1' }, 'a deleted project gets a new one');
  eq(calls.slice(0, 2).map(call => call.method + ' ' + call.url.slice(API.length)), ['POST /projects/gone/media', 'POST /projects'], 'the order after the 404');

  // A failed probe replays as failed for the same key, so only a new project can try the file again.
  broken = true;
  refused = null;
  try { await S.handoff.run(inst, job, projects); } catch (err) { refused = err; }
  eq(refused && refused.message, 'storymaker.handoff.import_failed', 'a failed probe says so');
  eq(projects.size, 0, 'a failed probe gives its project up');
  broken = false;

  const ready = { enabled: true, desktop_enabled: true, ffmpeg_ready: true, read_only: false };
  eq(S.handoff.canOffer(job, ready), true, 'a finished job with stems');
  eq(S.handoff.canOffer({ id: 'x', status: 'succeeded', output: { path: 'a', stems: {} } }, ready), true, 'stems asked for, all silent: the film still goes over');
  eq(S.handoff.canOffer(Object.assign({}, job, { status: 'failed' }), ready), false, 'a failed job');
  eq(S.handoff.canOffer({ id: 'x', status: 'succeeded', output: { path: 'a' } }, ready), false, 'no stems asked for');
  for (const field of ['enabled', 'desktop_enabled', 'ffmpeg_ready']) eq(S.handoff.canOffer(job, Object.assign({}, ready, { [field]: false })), false, field);
  eq(S.handoff.canOffer(job, Object.assign({}, ready, { read_only: true })), false, 'read-only');
  eq(S.handoff.canOffer(job, null), false, 'no status yet');
  eq(S.handoff.canOffer(Object.assign({}, job, { warnings: [{ code: 'stem_failed', message: 'narration.wav could not be written' }] }), ready), false, 'a failed stem would leave that sound out of the project');
  eq(S.handoff.canOffer(Object.assign({}, job, { warnings: [{ code: 'library_failed', message: 'Videos is read-only' }] }), ready), true, 'another warning does not stop the offer');
  eq([S.handoff.studioReady(ready), S.handoff.studioReady(Object.assign({}, ready, { read_only: true })), S.handoff.studioReady(null)], [true, false, false], 'the status half of canOffer');
  console.log('storymaker handoff ok');
})().catch(err => { console.error(err && err.stack || err); process.exit(1); });
`

const storymakerHandoffTracksScript = `
const audio = (id, frames) => ({ id, name: id + '.wav', path: 'media/' + id + '.wav', kind: 'audio', duration_frames: frames, duration_us: Math.round(frames / 30 * 1e6), has_audio: true, codec: 'pcm_s16le', size_bytes: 1000 });
const project = {
  version: 3, name: 'Firewall', width: 1920, height: 1080, fps: 30,
  assets: [
    { id: 'a-video', name: 'film.mp4', path: 'media/a-video.mp4', kind: 'video', duration_frames: 300, duration_us: 10000000, video_duration_us: 10000000, width: 1920, height: 1080, has_audio: true, source_fps: '30/1', codec: 'h264', size_bytes: 1000 },
    audio('a-narration', 180), audio('a-music', 400), audio('a-sfx', 250)
  ],
  tracks: []
};
const names = { video: 'Video track', narration: 'Narration track', music: 'Music track', sfx: 'Effects track' };
const full = S.handoff.buildTracks(project, { video: 'a-video', narration: 'a-narration', music: 'a-music', sfx: 'a-sfx' }, 250, names);
const silent = S.handoff.buildTracks(project, { video: 'a-video', narration: 'a-narration' }, 250, names);
console.log(JSON.stringify({ full: Object.assign({}, project, { tracks: full }), silent: Object.assign({}, project, { tracks: silent }) }));
`
