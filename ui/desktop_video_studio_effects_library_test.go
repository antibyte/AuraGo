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
assert.deepEqual(groups.map(group => group.category), ['color', 'detail', 'stylize', 'key', 'basic', 'wipe', 'motion', 'shape'], 'the transition groups follow the effect groups (2e)');
assert.deepEqual(groups.map(group => group.label_key), ['effectCategory_color', 'effectCategory_detail', 'effectCategory_stylize', 'effectCategory_key', 'transitionCategory_basic', 'transitionCategory_wipe', 'transitionCategory_motion', 'transitionCategory_shape']);
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

// Transitions in the effects library (2e, plan index §4.9). The test covers:
//   - the model: one group per transition category after the effect groups, keys 'transition:<id>', availability from
//     the unavailable list, the drag MIME of its own;
//   - cutAt: both halves of a clip, an existing transition, a gap, the track ends, overlay and audio tracks, bad input;
//   - addLibraryItem and addTransitionTo through P.applyTransition: the default length, short clips, an existing
//     transition's length, the following-clip and length rules, locked, read-only, audio, no-clip and unavailable
//     refusals, one undo step per edit and the outgoing clip selected; applyTransition without the 4th argument keeps
//     the 2a behaviour;
//   - the tab's markup: transition items with their generated icons and the unavailable title.
func TestVideoStudioEffectsLibraryTransitions(t *testing.T) {
	stage, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio-stage.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stage), "function applyTransition(s, type, frames, clipId) {") {
		t.Error("applyTransition must take the outgoing clip as its 4th parameter")
	}
	runVideoStudioKeyframesScript(t, videoStudioKeyframePartsPrelude+`
require('./js/desktop/apps/video-studio-picture.js');
require('./js/desktop/apps/video-studio-anim.js');
require('./js/desktop/apps/video-studio-icons.js');
require('./js/desktop/apps/video-studio-effects-library.js');
require('./js/desktop/apps/video-studio-stage.js');
const L = window.VideoStudioEffectsLibrary;
FX.adopt(undefined, undefined);

// The model: the four transition groups follow the effect groups; their items are fresh 'transition:<id>' entries.
const groups = L.model(), tx = groups.filter(group => group.kind === 'transition');
assert.deepEqual(groups.filter(group => !group.kind).map(group => group.category), ['color', 'detail', 'stylize', 'key']);
assert.deepEqual(tx.map(group => [group.category, group.label_key, group.items.length]), [['basic', 'transitionCategory_basic', 5], ['wipe', 'transitionCategory_wipe', 9], ['motion', 'transitionCategory_motion', 6], ['shape', 'transitionCategory_shape', 4]]);
assert.deepEqual(tx.flatMap(group => group.items.map(entry => entry.key)), FX.transitionGroups().flatMap(group => group.items.map(spec => 'transition:' + spec.id)));
assert.deepEqual(L.item('transition:wipeup'), {key: 'transition:wipeup', transition: 'wipeup', label_key: 'transition_wipeup', category: 'wipe', available: true});
assert.equal(L.item('transition:dissolve').label_key, 'dissolve', 'the 2a label keys');
for (const bad of ['transition:', 'transition:nope', 'transition:__proto__', 'transition:none', 'wipeup']) assert.equal(L.item(bad), null, bad);
L.item('transition:radial').transition = 'changed';
assert.equal(L.item('transition:radial').transition, 'radial', 'items are fresh copies');
FX.adopt(FX.registry(), {effects: [], transitions: ['fadegrays'], blend_modes: [], anim_presets: []});
assert.equal(L.item('transition:fadegrays').available, false);
assert.equal(L.item('transition:fadewhite').available, true);
FX.adopt(FX.registry(), {effects: [], transitions: [], blend_modes: [], anim_presets: []});
assert.equal(L.TRANSITION_MIME, 'application/x-aurago-vs-transition');
assert.notEqual(L.TRANSITION_MIME, L.MIME, 'a transition drag has a type of its own');

// cutAt: the hovered clip's halves name the cut before and after it. The clips are listed out of order on purpose.
const cuts = project([
  track('v1', 'video', [clip('d', V, 200, 60), clip('a', V, 0, 60), clip('c', V, 110, 60), clip('b', V, 60, 60, {transition: {type: 'dissolve', duration: 10}})]),
  track('o1', 'overlay', [clip('p', I, 0, 30), clip('q', I, 30, 30)]),
  track('a1', 'audio', [clip('m', A, 0, 30), clip('n', A, 30, 30)])
]);
valid(cuts);
const pairOf = (trackId, clipId, side) => { const cut = L.cutAt(cuts, trackId, clipId, side); return cut ? [cut.out.id, cut.inc.id] : null; };
assert.deepEqual(pairOf('v1', 'a', 'after'), ['a', 'b'], 'the right half of a: the cut where b starts');
assert.deepEqual(pairOf('v1', 'b', 'before'), ['a', 'b'], 'the left half of b: the same cut');
assert.deepEqual(pairOf('v1', 'b', 'after'), ['b', 'c'], 'clips joined by a transition share a cut');
assert.deepEqual(pairOf('v1', 'c', 'before'), ['b', 'c']);
assert.equal(pairOf('v1', 'c', 'after'), null, 'a gap is no cut');
assert.equal(pairOf('v1', 'd', 'before'), null, 'nor from the other side');
assert.equal(pairOf('v1', 'a', 'before'), null, 'the track start');
assert.equal(pairOf('v1', 'd', 'after'), null, 'the track end');
assert.deepEqual(pairOf('o1', 'p', 'after'), ['p', 'q'], 'overlay tracks take transitions');
assert.equal(pairOf('o1', 'p', 'before'), null);
assert.equal(pairOf('a1', 'm', 'after'), null, 'audio tracks take none');
assert.equal(pairOf('v1', 'nope', 'after'), null);
assert.equal(pairOf('nope', 'a', 'after'), null);
assert.equal(pairOf('v1', 'a', 'middle'), null);
assert.equal(L.cutAt(null, 'v1', 'a', 'after'), null);

// The edits: one undo step each through P.applyTransition, notices instead of changes when refused.
P.renderUI = () => {}; P.renderToolbar = () => {}; P.renderInspector = () => {}; P.resyncArtwork = () => {};
const notices = [];
P.showNotice = (s, key) => { notices.push(key); };
const session = p => ({project: p, projectId: '', readonly: false, conflict: true, history: [], redo: [], frame: 0, dirty: false, revision: 0,
  selectedClipId: '', selectionRevision: 0, selectionSet: new Set(), selectionPrimary: '', ctx: {}, q: () => null, preview: {seek() {}}});
const lane = () => project([track('v1', 'video', [clip('a', V, 0, 60), clip('b', V, 60, 60), clip('c', V, 150, 60)]), track('a1', 'audio', [clip('m', A, 0, 60)])]);
const entry = id => L.item('transition:' + id);
let s = session(lane());
// A drop on the cut a|b: the default length (half a second at 30 fps), the later clips move with the overlap like the
// inspector's, one undo step, and a is selected.
assert.equal(P.addLibraryItem(s, 'a', entry('wipeup')), true);
assert.deepEqual(at(s.project, 'a').transition, {type: 'wipeup', duration: 15});
assert.deepEqual([at(s.project, 'b').start, at(s.project, 'c').start], [45, 135]);
assert.equal(s.history.length, 1);
assert.deepEqual([s.selectedClipId, s.selectionPrimary, Array.from(s.selectionSet)], ['a', 'a', ['a']]);
valid(s.project);
// Another type on the same cut keeps the existing length.
assert.equal(P.applyTransition(s, 'wipeup', 20, 'a'), true);
assert.equal(P.addLibraryItem(s, 'a', entry('circleopen')), true);
assert.deepEqual(at(s.project, 'a').transition, {type: 'circleopen', duration: 20}, 'an existing transition keeps its length');
assert.equal(s.history.length, 3);
valid(s.project);
// Short clips: the default stays below both clips (transitionLength).
s = session(project([track('v1', 'video', [clip('a', V, 0, 8), clip('b', V, 8, 6)])]));
assert.equal(P.addTransitionTo(s, 'a', 'dissolve'), true);
assert.deepEqual(at(s.project, 'a').transition, {type: 'dissolve', duration: 5});
// No following clip: the inspector's notice, and nothing changes.
s = session(lane());
notices.length = 0;
assert.equal(P.addLibraryItem(s, 'c', entry('fadewhite')), false);
assert.deepEqual(notices, ['transitionNeedsNext']);
assert.equal(s.history.length, 0);
// Clips too short for any length: the length rule's notice.
s = session(project([track('v1', 'video', [clip('a', V, 0, 2), clip('b', V, 2, 2)])]));
notices.length = 0;
assert.equal(P.addTransitionTo(s, 'a', 'dissolve'), false);
assert.deepEqual(notices, ['transitionTooLong']);
// Refusals: a locked track, read-only, an audio clip, no clip, an unavailable transition.
s = session(lane()); s.project.tracks[0].locked = true; notices.length = 0;
assert.equal(P.addLibraryItem(s, 'a', entry('wipeup')), false);
assert.deepEqual(notices, ['editLocked']);
s = session(lane()); s.readonly = true; notices.length = 0;
assert.equal(P.addLibraryItem(s, 'a', entry('wipeup')), false);
assert.deepEqual(notices, ['readonly']);
s = session(lane()); notices.length = 0;
assert.equal(P.addLibraryItem(s, 'm', entry('wipeup')), false);
assert.equal(P.addLibraryItem(s, '', entry('wipeup')), false);
assert.deepEqual(notices, ['effectsLibraryTransitionNeedsClip', 'effectsLibraryTransitionNeedsClip'], 'a transition without a picture clip names the cut (2f F1)');
notices.length = 0;
assert.equal(P.addLibraryItem(s, 'm', L.item('blur')), false);
assert.deepEqual(notices, ['effectsLibraryNeedsClip'], 'effects keep their own notice');
FX.adopt(FX.registry(), {effects: [], transitions: ['radial'], blend_modes: [], anim_presets: []});
notices.length = 0;
assert.equal(P.addLibraryItem(s, 'a', L.item('transition:radial')), false);
assert.deepEqual(notices, ['transitionUnavailable']);
FX.adopt(FX.registry(), {effects: [], transitions: [], blend_modes: [], anim_presets: []});
assert.equal(s.history.length, 0);
// Without the 4th argument applyTransition acts on the selected clip and leaves the selection alone (2a).
s = session(lane()); s.selectedClipId = 'b';
assert.equal(P.applyTransition(s, 'dissolve', 10), true);
assert.deepEqual(at(s.project, 'b').transition, {type: 'dissolve', duration: 10});
assert.equal(s.selectionSet.size, 0);

// The tab's markup: transition items carry their generated icon; an unavailable one names the transition reason.
Object.assign(P, {esc: (st, value) => String(value == null ? '' : value).replace(/[&<>"']/g, ch => '&#' + ch.charCodeAt(0) + ';'), icon: name => '<i data-icon="' + name + '"></i>', tr: (st, key) => '[' + key + ']'});
const host = {innerHTML: '', contains: () => false, querySelector: () => null};
const tab = {id: 'w', project: lane(), readonly: false, q: selector => (selector === '[data-effects-library]' ? host : null)};
P.renderEffectsLibrary(tab);
assert(host.innerHTML.includes('data-effect-item="transition:circleopen" draggable="true" aria-disabled="false" title="[transition_circleopen]">' + window.VideoStudioIcons.transition('circleopen', 30) + '<span>[transition_circleopen]</span>'));
assert(host.innerHTML.includes('<h3 id="vs-fxlib-w-tx-basic">[transitionCategory_basic]</h3>'));
assert(host.innerHTML.indexOf('[effectCategory_key]') < host.innerHTML.indexOf('[transitionCategory_basic]'), 'the transitions follow the effects');
FX.adopt(FX.registry(), {effects: [], transitions: ['radial'], blend_modes: [], anim_presets: []});
P.renderEffectsLibrary(tab);
assert(/data-effect-item="transition:radial" draggable="false" aria-disabled="true" title="\[transition_radial\] \(\[transitionUnavailable\]\)"/.test(host.innerHTML), 'an unavailable transition says why');
FX.adopt(FX.registry(), {effects: [], transitions: [], blend_modes: [], anim_presets: []});
console.log('ok');
`)
}

// The library strings of 2e Task 17 and 2f F1 in every desktop locale: the intro, the cut notice and the transition
// notice without a picture clip. en and de are the plans' texts; every other locale translates them.
func TestVideoStudioEffectsLibraryTransitionLocales(t *testing.T) {
	want := map[string][2]string{
		"effectsLibraryIntro":               {"Drag an effect onto a clip, or a transition onto the cut between two clips. Double-click adds it to the selected clip.", "Ziehe einen Effekt auf einen Clip oder einen Übergang auf den Schnitt zwischen zwei Clips. Ein Doppelklick fügt ihn dem ausgewählten Clip hinzu."},
		"effectsLibraryDropOnCut":           {"Drop a transition on the cut between two clips on one track.", "Lege einen Übergang auf den Schnitt zwischen zwei Clips einer Spur."},
		"effectsLibraryTransitionNeedsClip": {"Select the picture clip before the cut first, or drop the transition on a cut.", "Wähle zuerst den Bildclip vor dem Schnitt aus oder lege den Übergang auf einen Schnitt."},
	}
	english := readDesktopLocale(t, "en")
	for _, lang := range videoStudioLocales {
		values := readDesktopLocale(t, lang)
		for key, texts := range want {
			got := values["videoStudio."+key]
			switch {
			case strings.TrimSpace(got) == "":
				t.Errorf("%s lacks videoStudio.%s", lang, key)
			case lang == "en" && got != texts[0]:
				t.Errorf("en videoStudio.%s = %q, want %q", key, got, texts[0])
			case lang == "de" && got != texts[1]:
				t.Errorf("de videoStudio.%s = %q, want %q", key, got, texts[1])
			case lang != "en" && got == english["videoStudio."+key]:
				t.Errorf("%s copies the English videoStudio.%s", lang, key)
			}
		}
	}
}
