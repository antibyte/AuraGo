package ui

import (
	"os/exec"
	"testing"
)

// Server assets carry duration_frames at the rate stored on the server. When one is merged into a project whose
// rate change is still unsaved it must be expressed in the project's rate, or the timeline becomes invalid.
func TestVideoStudioMergedServerAssetsFollowTheProjectRate(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio merge checks")
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-core.js');
require('./js/desktop/apps/video-studio-jobs.js');
const P = window.VideoStudioParts, T = window.VideoStudioTimeline;
P.renderUI = () => {};
const stored = {};
global.localStorage = {setItem: (key, value) => { stored[key] = value; }, getItem: key => stored[key] || null, removeItem: key => { delete stored[key]; }};
function respond(project, etag) {
  global.fetch = async () => ({ok: true, headers: {get: name => name.toLowerCase() === 'content-type' ? 'application/json' : name === 'ETag' ? etag : null}, json: async () => ({project})});
}
const box = {x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1, fit: 'contain'};
const asset = (id, frames, us) => ({id, name: id + '.mp4', path: 'media/' + id, kind: 'video', duration_frames: frames, duration_us: us, video_duration_us: us, width: 64, height: 36, has_audio: true});
// The saved server copy: one 10 s asset used by one clip at 30 fps.
function saved() {
  return {version: 2, name: 'x', width: 1280, height: 720, fps: 30, assets: [asset('a', 300, 10000000)], tracks: [{id: 't', kind: 'video', name: 'Video 1', muted: false, hidden: false, locked: false,
    clips: [Object.assign({}, box, {id: 'k', asset_id: 'a', start: 0, offset: 0, duration: 300, fade_in: 0, fade_out: 0, text: '', transition: null})]}]};
}
// The server copy after imports: asset b is new, asset c finished probing (5 s). Both are counted at 30 fps.
function latest() {
  const project = saved();
  project.assets.push(asset('b', 300, 10000000), asset('c', 150, 5000000));
  return project;
}
// The editor with an unsaved switch to 60 fps and a still-unprobed asset c.
function session() {
  const s = {projectId: 'p', projectEpoch: 1, abort: {}, disposed: false, dirty: true, revision: 0, etag: '"1"', ctx: {confirmDialog: async () => true}};
  s.savedProject = P.canonicalProject(P.hydrateProject(s, saved()));
  s.project = P.hydrateProject(s, saved());
  s.project.assets.push({id: 'c', name: 'c.mp4', path: 'media/c', kind: '', duration_frames: 0});
  window.VideoStudioTime.rescaleProject(s.project, 60, false);
  return s;
}
const byId = (s, id) => s.project.assets.find(item => item.id === id);
function check(s, label) {
  assert.equal(byId(s, 'a').duration_frames, 600, label + ': the used asset keeps its 60 fps length');
  assert.equal(byId(s, 'b').duration_frames, 600, label + ': a new 10 s asset counts 600 frames at 60 fps');
  assert.equal(byId(s, 'c').duration_frames, 300, label + ': a probed 5 s asset counts 300 frames at 60 fps');
  assert.equal(byId(s, 'c').kind, 'video', label + ': probe results arrive');
  assert(T.validTimeline(s.project), label + ': the project stays a valid timeline');
}
(async () => {
  const sane = session();
  assert.equal(sane.project.fps, 60);
  assert.equal(sane.project.tracks[0].clips[0].duration, 600);
  assert(T.validTimeline(sane.project), 'the fixture is valid at 60 fps');
  assert.equal(P.assetAtProjectRate(asset('x', 1, 1000000), {fps: 60}).duration_frames, 60);
  assert.equal(P.assetAtProjectRate({id: 'i', kind: 'image', duration_frames: 0, duration_us: 3000000}, {fps: 60}).duration_frames, 0, 'images keep their frame count');
  assert.equal(P.assetAtProjectRate({id: 'p', kind: '', duration_frames: 0}, {fps: 60}).duration_frames, 0, 'assets without a duration stay unchanged');

  // 412 adoption.
  let s = session();
  respond(latest(), '"2"');
  assert.equal(await P.adoptServerMedia(s, 'p', 1), true, 'only the media list differs, so it is adopted');
  check(s, 'adoptServerMedia');
  assert.equal(s.etag, '"2"');
  assert.equal(s.savedProject.assets.find(item => item.id === 'b').duration_frames, 300, 'the saved copy keeps the server rate');

  // Refresh with unsaved edits.
  s = session();
  respond(latest(), '"2"');
  await P.refreshProject(s);
  assert.equal(s.conflict, undefined, 'no conflict for a media-only difference');
  check(s, 'refreshProject');
  assert.equal(s.project.fps, 60, 'the unsaved rate change survives');

  // A recovered draft of the 60 fps project learns about the media the server received meanwhile.
  s = session();
  P.persistDraft(s);
  s.project = P.hydrateProject(s, latest());
  s.dirty = false;
  await P.recoverDraft(s, 'p', 1);
  assert.equal(s.dirty, true);
  assert.equal(s.project.fps, 60, 'the draft rate is restored');
  assert.equal(byId(s, 'b').duration_frames, 600, 'recoverDraft: server assets follow the draft rate');
  assert(T.validTimeline(s.project), 'recoverDraft: the recovered project stays valid');
})().catch(error => { console.error(error); process.exit(1); });
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("merge checks: %v\n%s", err, output)
	}
}
