package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The projectId launch of Video Studio in Node. video-studio.js runs against stubbed parts and fake instances; the reuse branch of the
// window shell and the launch merge of the routing are cut from their files and run on their own.
func TestVideoStudioOpenProjectInNode(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "video-studio-open-project.js")
	if err := os.WriteFile(script, []byte(videoStudioOpenProjectScript), 0o600); err != nil {
		t.Fatal(err)
	}
	apps, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	core, _ := filepath.Abs(filepath.Join("js", "desktop", "core"))
	out, err := exec.Command(node, script, apps, core).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "video studio open project ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const videoStudioOpenProjectScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const [appsDir, coreDir] = process.argv.slice(2);
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const read = (dir, name) => fs.readFileSync(path.join(dir, name), 'utf8').replace(/\r\n/g, '\n');
const settle = async () => { for (let i = 0; i < 50; i++) await new Promise(resolve => setImmediate(resolve)); };

const API = '/api/desktop/video-studio';
const log = [];
let saveWorks = true;
let holdLoad = null;
const P = {
  API,
  request: async url => {
    if (url === API + '/status') return { body: { enabled: true, desktop_enabled: true, ffmpeg_ready: true }, etag: '' };
    if (url === API + '/projects') return { body: { projects: [{ id: 'first' }, { id: 'wanted' }] }, etag: '' };
    const id = decodeURIComponent(url.slice((API + '/projects/').length));
    log.push('load ' + id);
    if (holdLoad) await holdLoad;
    return { body: { project: { name: id, tracks: [{ id: 'track-1', clips: [] }] } }, etag: 'etag-' + id };
  },
  hydrateProject: (s, project) => project,
  canonicalProject: project => project,
  cancelTextApplies() {},
  recoverDraft: async () => false,
  loadJobs: async () => {},
  closePopovers() {},
  flushArtwork: async () => {},
  saveUntilClean: async s => { log.push('save'); if (saveWorks) s.dirty = false; return saveWorks; },
  clearNotice() {},
  showNotice() {},
  renderAIPanel() {}
};
const context = { console, VideoStudioParts: P, VideoStudioEffects: { adopt() {} }, setInterval: () => 0 };
context.window = context; context.globalThis = context;
vm.createContext(context);
vm.runInContext(read(appsDir, 'video-studio.js'), context, { filename: 'video-studio.js' });
const App = context.VideoStudioApp;

// renderUI and populateCanvas return early without their nodes, so only the popover lookup of loadProjects needs an answer.
function instance(id, fields) {
  const s = Object.assign({
    id, ctx: {}, abort: { signal: null }, disposed: false, project: null, projectId: '', projects: [], dirty: false, readonly: false, studioReady: false,
    app: { classList: { toggle() {} } }, stickerPicker: { setDisabled() {} },
    q: selector => (selector === '[data-project-popover]' ? { hidden: true } : null)
  }, fields || {});
  App.instances.set(id, s);
  return s;
}
const open = { project: { name: 'first', tracks: [{ id: 'track-1', clips: [] }] } };

(async () => {
  const preferred = instance('w1', { ctx: { projectId: 'wanted' } });
  await P.initialize(preferred);
  await settle();
  eq([preferred.projectId, preferred.studioReady], ['wanted', true], 'initialize opens a listed id');

  const gone = instance('w2', { ctx: { projectId: 'gone' } });
  await P.initialize(gone);
  await settle();
  eq(gone.projectId, 'first', 'an id that is not listed falls back to the first project');

  log.length = 0;
  const early = instance('w3');
  await App.openProject('w3', 'wanted');
  eq([early.ctx.projectId, early.projectId, log], ['wanted', '', []], 'before ready the id only waits in the context');

  // s.projects starts empty here, so a switch proves that openProject read the list again.
  const dirty = instance('w4', Object.assign({ studioReady: true, dirty: true, projectId: 'first' }, open));
  await App.openProject('w4', 'wanted');
  eq([log, dirty.projectId], [['save', 'load wanted'], 'wanted'], 'dirty work is saved before the switch');

  log.length = 0;
  saveWorks = false;
  const stays = instance('w5', Object.assign({ studioReady: true, dirty: true, projectId: 'first' }, open));
  await App.openProject('w5', 'wanted');
  saveWorks = true;
  eq([log, stays.projectId], [['save'], 'first'], 'a failed save keeps the open project');

  log.length = 0;
  const unlisted = instance('w6', Object.assign({ studioReady: true, projectId: 'first' }, open));
  await App.openProject('w6', 'gone');
  eq([log, unlisted.projectId], [[], 'first'], 'an id that is not listed changes nothing');

  // An id that arrives while the first load runs opens when that load ends.
  let release = null;
  holdLoad = new Promise(resolve => { release = resolve; });
  const racing = instance('w7');
  const first = P.initialize(racing);
  await settle();
  await App.openProject('w7', 'wanted');
  holdLoad = null;
  release();
  await first;
  await settle();
  eq(racing.projectId, 'wanted', 'an id stored during the first load opens afterwards');

  // The reuse branch of the window shell.
  const shell = read(coreDir, 'window-shell-runtime.js');
  const mark = "if (appId === 'video-studio' && context && !context.sessionRestore && typeof context.projectId === 'string' && context.projectId) {";
  const start = shell.indexOf(mark);
  if (start < 0) throw new Error('window-shell-runtime.js has no projectId branch in the reuse path');
  const closing = '\n            }';
  const reuse = new Function('appId', 'context', 'existing', 'window', shell.slice(start, shell.indexOf(closing + '\n', start) + closing.length));
  const opened = [];
  const live = { VideoStudioApp: { instances: new Map([['win', {}]]), openProject: (id, projectId) => opened.push([id, projectId]) } };
  const win = { id: 'win' };
  reuse('video-studio', { projectId: 'p1' }, win, live);
  eq([opened, 'pendingProjectId' in win], [[['win', 'p1']], false], 'a live window opens the project');
  const lazy = { id: 'lazy' };
  reuse('video-studio', { projectId: 'p2' }, lazy, {});
  eq(lazy.pendingProjectId, 'p2', 'a window that still loads keeps the id');
  const restored = { id: 'win' };
  reuse('video-studio', { projectId: 'p3', sessionRestore: {} }, restored, live);
  reuse('video-studio', { projectId: '' }, restored, live);
  eq([opened.length, 'pendingProjectId' in restored], [1, false], 'a session restore and an empty id do nothing');

  // The launch merge of the routing.
  const routing = read(coreDir, 'menus-and-routing.js');
  const from = routing.indexOf('const launch = Object.assign({}, context || {}, win?.pendingCameraImport');
  const endMark = 'delete win.pendingProjectId; }';
  const to = routing.indexOf(endMark, from);
  if (from < 0 || to < 0) throw new Error('menus-and-routing.js does not merge pendingProjectId');
  const merge = new Function('context', 'win', routing.slice(from, to + endMark.length) + '\nreturn launch;');
  const pending = { pendingProjectId: 'p4', pendingCameraImport: { importPath: 'Videos/a.webm' } };
  eq(merge({ a: 1 }, pending), { a: 1, importPath: 'Videos/a.webm', projectId: 'p4' }, 'launch takes the pending id');
  eq(Object.keys(pending), [], 'the pending values are used once');
  eq(merge({ a: 1 }, {}), { a: 1 }, 'no pending id adds nothing');

  console.log('video studio open project ok');
})().catch(err => { console.error(err && err.stack || err); process.exit(1); });
`
