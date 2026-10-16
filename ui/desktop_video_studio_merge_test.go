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

  // Name and folder belong to the project document: a server copy refreshes every other field (the poster flag
  // included) but never overwrites a rename or move the editor holds.
  const importedCopy = (id, extra) => { const project = latest(); Object.assign(project.assets.find(item => item.id === id), {name: 'imported.png', folder: 'Upload', has_poster: true}, extra || {}); return project; };
  s = session();
  Object.assign(byId(s, 'a'), {name: 'Hero', folder: 'Trips'});
  respond(importedCopy('a'), '"3"');
  assert.equal(await P.adoptServerMedia(s, 'p', 1), true, 'a rename does not make the copies differ');
  assert.equal(byId(s, 'a').name, 'Hero', 'adoptServerMedia keeps the local name');
  assert.equal(byId(s, 'a').folder, 'Trips', 'adoptServerMedia keeps the local folder');
  assert.equal(byId(s, 'a').has_poster, true, 'adoptServerMedia takes the server poster flag');
  assert.equal(s.savedProject.assets.find(item => item.id === 'a').name, 'imported.png', 'the saved copy mirrors the server');
  check(s, 'owned fields');

  s = session();
  Object.assign(byId(s, 'c'), {name: 'Pending'});
  respond(importedCopy('c'), '"3"');
  await P.refreshProject(s);
  assert.equal(s.conflict, undefined);
  assert.equal(byId(s, 'c').name, 'Pending', 'refreshProject keeps the local name of a probed asset');
  assert.ok(!byId(s, 'c').folder, 'a local top-level asset does not take the upload folder');
  assert.equal(byId(s, 'c').has_poster, true, 'refreshProject takes the server poster flag');
  assert.equal(byId(s, 'c').kind, 'video', 'the probe result still arrives');
  assert.equal(P.keepOwnedAssetFields({name: '', folder: ''}, {name: 'n', folder: 'F', kind: 'video'}).name, 'n', 'a blank local name falls back to the server name');
  assert.equal('folder' in P.keepOwnedAssetFields({name: 'x'}, {name: 'n', folder: 'F'}), false, 'no local folder, no folder key');
})().catch(error => { console.error(error); process.exit(1); });
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("merge checks: %v\n%s", err, output)
	}
}

// Another window removed media this editor still lists. Unused, it leaves the project (412 adoption, a 400
// project_asset_invalid save, a recovered draft) and the save goes through once; used by a clip, the editor stops
// retrying and surfaces the conflict.
func TestVideoStudioMediaRemovedElsewhereClientPaths(t *testing.T) {
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
const P = window.VideoStudioParts;
let timers = 0;
window.setTimeout = () => { timers++; return 0; };
P.renderUI = () => {}; P.renderToolbar = () => {};
const stored = {};
global.localStorage = {setItem: (key, value) => { stored[key] = value; }, getItem: key => stored[key] || null, removeItem: key => { delete stored[key]; }};
// The server: the saved document lists asset a only; media x was removed in another window.
let puts = [], putReplies = [];
global.fetch = async (url, options = {}) => {
  const method = (options.method || 'GET').toUpperCase();
  let status = 200, body, etag = null;
  if (method === 'PUT') { puts.push(JSON.parse(options.body)); const reply = putReplies.shift() || {status: 200}; status = reply.status; body = reply.body || {project: JSON.parse(options.body)}; etag = '"3"'; }
  else if (url.endsWith('/media')) body = {media: [{asset: {id: 'a'}, state: 'ready'}]};
  else { body = {project: saved()}; etag = '"2"'; }
  return {ok: status < 400, status, headers: {get: name => name.toLowerCase() === 'content-type' ? 'application/json' : name === 'ETag' ? etag : null}, json: async () => body};
};
const box = {x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1, fit: 'contain'};
const asset = id => ({id, name: id + '.png', path: 'media/' + id, kind: 'image', duration_frames: 150, width: 64, height: 36, has_audio: false});
const clip = (id, assetId, start) => Object.assign({}, box, {id, asset_id: assetId, start, offset: 0, duration: 150, fade_in: 0, fade_out: 0, text: '', transition: null});
function saved() {
  return {version: 2, name: 'x', width: 1280, height: 720, fps: 30, assets: [asset('a')], tracks: [{id: 't', kind: 'video', name: 'Video 1', muted: false, hidden: false, locked: false, clips: [clip('k', 'a', 0)]}]};
}
const withX = used => { const project = saved(); project.assets.push(asset('x')); if (used) project.tracks[0].clips.push(clip('kx', 'x', 150)); return project; };
// The stale window: its last save still listed x; usedLocally adds an unsaved clip of x.
function session(usedLocally) {
  const s = {projectId: 'p', projectEpoch: 1, abort: {}, disposed: false, dirty: true, revision: 1, etag: '"1"', ctx: {confirmDialog: async () => true}, q: () => null, history: [], redo: []};
  s.savedProject = P.canonicalProject(P.hydrateProject(s, withX(false)));
  s.project = P.hydrateProject(s, withX(usedLocally));
  s.history = [P.hydrateProject(s, withX(false))];
  return s;
}
const ids = project => project.assets.map(item => item.id).join(',');
(async () => {
  // 412: unused removed media leaves the project and its history; the observed ETag is adopted.
  let s = session(false);
  assert.equal(await P.adoptServerMedia(s, 'p', 1), true, 'unused removed media does not block the adoption');
  assert.equal(ids(s.project), 'a', 'adoptServerMedia drops the removed media');
  assert.equal(ids(s.history[0]), 'a', 'the undo history drops the unused removed media');
  assert.equal(s.etag, '"2"');
  // 412: a clip that still uses the removed media is a conflict; nothing changes.
  s = session(true);
  assert.equal(await P.adoptServerMedia(s, 'p', 1), false, 'a clip of removed media is a conflict');
  assert.equal(ids(s.project), 'a,x', 'the conflict leaves the project as it was');

  // A save that meets the 412 retries once without the removed media.
  s = session(false); puts = []; putReplies = [{status: 412, body: {code: 'file_conflict', error: 'file_conflict'}}];
  assert.equal(await P.saveProject(s), true, 'the stale save goes through after the adoption');
  assert.equal(puts.length, 2);
  assert.equal(ids(puts[1]), 'a', 'the retry no longer lists the removed media');
  assert.ok(!s.conflict && !s.dirty);

  // 400 project_asset_invalid with unused removed media: dropped, one retry, saved.
  s = session(false); s.etag = '"2"'; puts = []; putReplies = [{status: 400, body: {code: 'project_asset_invalid', error: 'project_asset_invalid'}}];
  assert.equal(await P.saveProject(s), true, 'the save goes through without the removed media');
  assert.deepEqual(puts.map(ids), ['a,x', 'a']);
  assert.ok(!s.conflict && !s.dirty);

  // 400 project_asset_invalid with a clip of the removed media: no retry loop, the conflict is surfaced.
  s = session(true); s.etag = '"2"'; puts = []; timers = 0; putReplies = [{status: 400, body: {code: 'project_asset_invalid', error: 'project_asset_invalid'}}];
  assert.equal(await P.saveProject(s), false);
  assert.equal(puts.length, 1, 'a clip of removed media is not retried');
  assert.equal(s.conflict, true, 'the conflict choices take over');
  assert.equal(timers, 0, 'no autosave is scheduled while the conflict is open');

  // A recovered draft drops unused removed media; a draft clip of it surfaces the conflict.
  for (const used of [false, true]) {
    s = session(false); s.project = P.hydrateProject(s, saved()); s.dirty = false; s.conflict = false;
    stored['video-studio-draft:p'] = JSON.stringify({saved_at: Date.now(), project: P.canonicalProject(P.hydrateProject(s, withX(used)))});
    const conflict = await P.recoverDraft(s, 'p', 1);
    assert.equal(s.dirty, true);
    assert.equal(conflict, used, 'recoverDraft reports a draft that uses removed media');
    assert.equal(!!s.conflict, used);
    assert.equal(ids(s.project), used ? 'a,x' : 'a', used ? 'the draft is kept for the user to resolve' : 'recoverDraft drops unused removed media');
  }
})().catch(error => { console.error(error); process.exit(1); });
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("removed media checks: %v\n%s", err, output)
	}
}
