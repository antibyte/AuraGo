package ui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

// runVideoStudioLibraryScript runs a self-contained Node check (the script sets up window itself).
func runVideoStudioLibraryScript(t *testing.T, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio library checks")
	}
	var output bytes.Buffer
	cmd := exec.Command(node, "-e", script)
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("library checks: %v\n%s", err, output.Bytes())
	}
}

// The effects library tab (spec §8.4): categories in registry order with the eight looks after the lut effect, item
// lookup by key only, availability from the server's unavailable list, the drag MIME and the LUT limit shared with Go,
// the LUT reference helpers, the tab's markup, and the wiring in the app shell and the library.
func TestVideoStudioEffectsLibraryModel(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("js", "desktop", "apps", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	app, library := read("video-studio.js"), read("video-studio-library.js")
	for _, want := range []string{"tab('effects', 'effects', 'effectsTab', 'Effects')", `data-panel="effects"`, "<div data-effects-library></div>", "P.wireEffectsLibrary(s);", "P.renderEffectsLibrary(s);", "'shortcutEffectMove'", `accept="video/*,audio/*,image/png,image/webp,.cube"`, "if (asset.kind === 'lut') return P.dropLUTAsset(s, asset, trackId, at);"} {
		if !strings.Contains(app, want) {
			t.Errorf("video-studio.js lacks %s", want)
		}
	}
	if wire, attach := strings.Index(app, "P.wireEffectsLibrary(s);"), strings.Index(app, "s.detachTimeline = T().attach("); wire < attach {
		t.Error("the effects library must wire its timeline drop after the timeline's own listeners")
	}
	for _, want := range []string{"window.VideoStudioEffectsLibrary.isLUTFile(file.name)", "'.cube'", "P.lutUploadNotice(s, error.body)"} {
		if !strings.Contains(library, want) {
			t.Errorf("video-studio-library.js lacks %s", want)
		}
	}
	runVideoStudioLibraryScript(t, fmt.Sprintf(`
const assert = require('node:assert/strict');
global.window = {};
global.document = {activeElement: null};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-picture.js');
require('./js/desktop/apps/video-studio-effects-library.js');
const FX = window.VideoStudioEffects, L = window.VideoStudioEffectsLibrary, P = window.VideoStudioParts;
assert.equal(L.MIME, 'application/x-aurago-vs-effect');
assert.equal(L.MAX_LUT_BYTES, %d, 'MAX_LUT_BYTES mirrors Go videostudio.MaxLUTBytes');

const groups = L.model();
assert.deepEqual(groups.map(group => group.category), ['color', 'detail', 'stylize', 'key']);
assert.deepEqual(groups.map(group => group.label_key), ['effectCategory_color', 'effectCategory_detail', 'effectCategory_stylize', 'effectCategory_key']);
const looks = ['warm', 'cool', 'teal_orange', 'mono', 'faded_film', 'high_contrast', 'vintage', 'bleach_bypass'];
assert.deepEqual(groups[0].items.map(entry => entry.key), ['color', 'lut'].concat(looks.map(id => 'look:' + id)));
assert.deepEqual(groups[1].items.map(entry => entry.key), ['blur', 'sharpen']);
assert.deepEqual(groups[2].items.map(entry => entry.key), ['vignette']);
assert.deepEqual(groups[3].items.map(entry => entry.key), ['chroma_key'], 'chroma_key opens the key category (2d)');
const look = L.item('look:teal_orange');
assert.deepEqual([look.effect, look.params, look.label_key, look.look, look.category], ['lut', {lut: 'builtin:teal_orange'}, 'look_teal_orange', 'teal_orange', 'color']);
assert.deepEqual(L.item('lut').params, {}, 'the plain LUT item starts without a table (lut.lut defaults to "")');
assert.equal(L.item('color').label_key, 'effect.color.name');
for (const bad of ['', 'look:', 'look:nope', 'chroma', '__proto__', 'constructor', 'look:__proto__', null, undefined]) assert.equal(L.item(bad), null, String(bad));
L.item('look:warm').params.lut = 'changed';
assert.equal(L.item('look:warm').params.lut, 'builtin:warm', 'items are fresh copies');
assert(groups.every(group => group.items.every(entry => entry.available === true)), 'everything is available with the built-in registry');
// The server's unavailable list greys out the lut effect and every look with it.
FX.adopt(FX.registry(), {effects: ['lut'], transitions: [], blend_modes: [], anim_presets: []});
assert.deepEqual(L.model()[0].items.filter(entry => !entry.available).map(entry => entry.key), ['lut'].concat(looks.map(id => 'look:' + id)));
FX.adopt(FX.registry(), {effects: [], transitions: [], blend_modes: [], anim_presets: []});

for (const name of ['a.cube', 'LOOK.CUBE', 'x.Cube']) assert.equal(L.isLUTFile(name), true, name);
for (const name of ['a.cube.png', 'cube', 'a.cub', '', null]) assert.equal(L.isLUTFile(name), false, String(name));

// LUT references: every lut entry, on or off; removal keeps other entries and drops an emptied list.
const p = {tracks: [{id: 'v1', kind: 'video', locked: false, clips: [
  {id: 'a', asset_id: 'v', effects: [{id: 'lut', on: true, params: {lut: 'L1'}}, {id: 'color', on: true, params: {}}, {id: 'lut', on: false, params: {lut: 'L1'}}]},
  {id: 'b', asset_id: 'v', effects: [{id: 'lut', on: true, params: {lut: 'L1'}}]},
  {id: 'c', asset_id: 'v', effects: [{id: 'lut', on: true, params: {lut: 'builtin:warm'}}]}]}]};
assert.deepEqual(L.lutRefs(p, ['L1']).map(ref => ref.clip.id + ':' + ref.index), ['a:0', 'a:2', 'b:0']);
assert.equal(L.removeLUTRefs(p, new Set(['L1'])), 3);
assert.deepEqual(p.tracks[0].clips.map(clip => clip.effects ? clip.effects.map(entry => entry.id + ':' + (entry.params.lut || '')) : null), [['color:'], null, ['lut:builtin:warm']]);
assert.equal('effects' in p.tracks[0].clips[1], false, 'a clip left without effects loses the field');

// The tab's markup: groups, items as buttons with aria-disabled, draggable only while editing is possible.
Object.assign(P, {esc: (s, value) => String(value == null ? '' : value).replace(/[&<>"']/g, ch => '&#' + ch.charCodeAt(0) + ';'), icon: name => '<i data-icon="' + name + '"></i>', tr: (s, key) => '[' + key + ']'});
const host = {innerHTML: '', contains: () => false, querySelector: () => null};
const s = {id: 'w', project: {tracks: []}, readonly: false, q: selector => (selector === '[data-effects-library]' ? host : null)};
P.renderEffectsLibrary(s);
const tag = key => (host.innerHTML.match(new RegExp('<button[^>]*data-effect-item="' + key + '"[^>]*>')) || [''])[0];
assert(tag('look:mono').includes('draggable="true"') && tag('look:mono').includes('aria-disabled="false"'), tag('look:mono'));
assert(host.innerHTML.includes('[effectCategory_detail]') && host.innerHTML.includes('[look_mono]') && host.innerHTML.includes('[effectsLibraryIntro]'));
assert(host.innerHTML.includes('data-icon="lut"') && host.innerHTML.includes('data-icon="sliders"'));
s.readonly = true;
P.renderEffectsLibrary(s);
assert(tag('color').includes('aria-disabled="true"') && tag('color').includes('draggable="false"'), 'read-only items are disabled');
`, videostudio.MaxLUTBytes))
}

// Removing a LUT asset (spec §5.7): effects that use it count as usage, the confirm names how many go, and the effects
// leave with the media in one undo step before the file is deleted. Once the file is gone no undo step refers to it. A
// lut effect on a locked track refuses the removal; declining the confirm changes nothing.
func TestVideoStudioRemoveLUTRemovesEffects(t *testing.T) {
	runVideoStudioLibraryScript(t, `
const assert = require('node:assert/strict');
global.window = {};
global.crypto = global.crypto || require('node:crypto').webcrypto;
global.localStorage = {setItem() {}, getItem() { return null; }, removeItem() {}};
for (const name of ['time', 'effects', 'picture', 'timeline', 'edit', 'folders', 'inspector', 'core', 'editing', 'mediabin', 'effects-library']) require('./js/desktop/apps/video-studio-' + name + '.js');
window.setTimeout = () => 0;
const P = window.VideoStudioParts, F = window.VideoStudioFolders, L = window.VideoStudioEffectsLibrary, T = window.VideoStudioTimeline;
const notices = [], deletes = [];
let releaseDelete = null;
Object.assign(P, {
  renderUI() {}, renderAssets() {}, renderInspector() {}, renderToolbar() {}, renderTimeline() {}, syncInspectorLock() {}, updateTransform() {},
  showNotice: (s, key) => { notices.push(key); }, saveUntilClean: async () => true,
  request: (url, options) => { deletes.push([url, options && options.method]); return new Promise(resolve => { releaseDelete = () => resolve({body: {}, etag: ''}); }); }
});
const box = {x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1, fade_in: 0, fade_out: 0, fit: 'contain', text: '', text_style: null, transition: null};
const V = {id: 'v', name: 'Clip.mp4', path: 'media/v.mp4', kind: 'video', duration_frames: 600, width: 1280, height: 720, has_audio: false};
const I = {id: 'i', name: 'Logo.png', path: 'media/i.png', kind: 'image', duration_frames: 18000, width: 400, height: 200, has_audio: false};
const lut = id => ({id, name: id + '.cube', path: 'media/' + id + '.cube', kind: 'lut', duration_frames: 0, has_audio: false, lut_size: 17, size_bytes: 120000});
const fixture = lockedOverlay => ({version: 3, name: 'LUT fixture', width: 1280, height: 720, fps: 30, assets: [V, I, lut('lut1'), lut('lut2')], tracks: [
  {id: 'v1', kind: 'video', name: 'Video 1', muted: false, hidden: false, locked: false, clips: [Object.assign({id: 'c1', asset_id: 'v', start: 0, offset: 0, duration: 60}, box, {effects: [
    {id: 'color', on: true, params: {exposure: 0.5}}, {id: 'lut', on: true, params: {intensity: 1, lut: 'lut1'}}, {id: 'lut', on: true, params: {intensity: 1, lut: 'builtin:warm'}}]})]},
  {id: 'o1', kind: 'overlay', name: 'Overlay 1', muted: false, hidden: false, locked: !!lockedOverlay, clips: [Object.assign({id: 'c2', asset_id: 'i', start: 0, offset: 0, duration: 60}, box, {effects: [
    {id: 'lut', on: false, params: {intensity: 0.5, lut: 'lut1'}, keyframes: {intensity: [{f: 0, v: 0, e: 'linear'}, {f: 30, v: 1, e: 'linear'}]}}]})]}]});
const session = (project, confirm) => {
  const s = {id: 'w', projectId: 'p', projectEpoch: 1, disposed: false, readonly: false, history: [], redo: [], jobs: [], revision: 0, dirty: false,
    selectedClipId: '', selectionSet: new Set(), selectionPrimary: '', selectionRevision: 0, selectedAssetId: '', abort: {signal: null},
    q: () => null, host: {querySelectorAll: () => []}, ctx: {t: key => key, confirmDialog: confirm}};
  s.project = P.hydrateProject(s, project);
  s.savedProject = P.canonicalProject(s.project);
  return s;
};
const effectsOf = (s, id) => { const clip = T.clipFor(s.project, id).clip; return clip.effects ? clip.effects.map(entry => entry.id + ':' + (entry.params.lut || '')) : null; };
const tick = () => new Promise(resolve => setImmediate(resolve));
(async () => {
  let s = session(fixture(false), async () => true);
  assert(T.validTimeline(s.project), 'the fixture is valid');
  const usage = F.usageCounts(s.project);
  assert.equal(usage.get('lut1'), 2, 'each effect entry that uses a LUT counts, on or off');
  assert.equal(usage.get('lut2'), undefined, 'an unreferenced LUT is unused');
  assert.equal(usage.has('builtin:warm'), false, 'built-in looks are no media');
  assert.equal(usage.get('v'), 1, 'clips still count');

  // Removal: the confirm names the effects; effects and media leave in one undo step before the DELETE.
  const asked = [];
  s = session(fixture(false), async (title, copy) => { asked.push([title, copy]); return true; });
  const pending = P.removeAssets(s, ['lut1'], false);
  for (let i = 0; i < 50 && !deletes.length; i++) await tick();
  assert.deepEqual(asked, [['Remove media used in the timeline?', '2 effects that use this LUT are removed in one undo step. Then the file is deleted. This cannot be undone.']]);
  assert.deepEqual(effectsOf(s, 'c1'), ['color:', 'lut:builtin:warm']);
  assert.equal(effectsOf(s, 'c2'), null, 'a clip left without effects loses the field');
  assert.deepEqual(s.project.assets.map(asset => asset.id), ['v', 'i', 'lut2']);
  assert(T.validTimeline(s.project), 'no effect refers to the removed LUT');
  assert.equal(s.history.length, 1, 'one undo step');
  assert.equal(s.history[0].assets.some(asset => asset.id === 'lut1'), true, 'undo would bring back the LUT ...');
  assert.equal(L.lutRefs(s.history[0], ['lut1']).length, 2, '... and both effects');
  assert.deepEqual(deletes.map(([url, method]) => [url.replace(/^.*\/projects\//, ''), method]), [['p/media/lut1', 'DELETE']]);
  releaseDelete();
  assert.equal(await pending, true);
  assert.equal(JSON.stringify(s.history).includes('lut1'), false, 'no undo step can bring back an effect of the deleted LUT');
  assert.equal(s.history.length, 0, 'the removal step itself leaves the history');
  assert(notices.includes('mediaRemoved'));

  // An unused LUT gets the plain confirm.
  asked.length = 0;
  s = session(fixture(false), async (title, copy) => { asked.push([title, copy]); return false; });
  assert.equal(await P.removeAssets(s, ['lut2'], false), false);
  assert.deepEqual(asked, [['Remove this media?', 'The file is deleted from the project. This cannot be undone.']]);

  // A lut effect on a locked track refuses the removal; nothing changes and nothing is deleted.
  notices.length = 0; deletes.length = 0;
  s = session(fixture(true), async () => true);
  const untouched = JSON.stringify(P.canonicalProject(s.project));
  assert.equal(await P.removeAssets(s, ['lut1'], false), false);
  assert.equal(JSON.stringify(P.canonicalProject(s.project)), untouched);
  assert.deepEqual(deletes, []);
  assert(notices.includes('editLocked'), JSON.stringify(notices));

  // Declining the confirm changes nothing.
  s = session(fixture(false), async () => false);
  assert.equal(await P.removeAssets(s, ['lut1'], false), false);
  assert.equal(L.lutRefs(s.project, ['lut1']).length, 2);
})().catch(error => { console.error(error); process.exit(1); });
`)
}
