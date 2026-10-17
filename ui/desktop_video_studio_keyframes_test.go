package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// videoStudioKeyframesPrelude adds keyframe helpers to videoStudioEditPrelude (30 fps; V and A are 600-frame video and
// audio sources, I a still; clip, track, project, at and valid build and check projects).
const videoStudioKeyframesPrelude = `
require('./js/desktop/apps/video-studio-keyframe-edit.js');
const FX = window.VideoStudioEffects, K = window.VideoStudioKeyframeEdit;
const kf = (f, v, e) => ({f, v, e: e || 'linear'});
const near = (a, b, message) => assert(Math.abs(a - b) < 1e-9, (message || 'values differ') + ': ' + a + ' vs ' + b);
`

// runVideoStudioKeyframesScript runs a Node check after the edit and keyframe preludes.
func runVideoStudioKeyframesScript(t *testing.T, script string) {
	t.Helper()
	runVideoStudioEditScript(t, videoStudioKeyframesPrelude+script)
}

// Keyframe edit semantics (index §4.18): keys stay on their content through split, trims, slip and speed changes;
// reverse mirrors them (ease_in <-> ease_out, holds shifted by one source frame, limit on overflow); freeze takes the
// evaluated box; paste attributes re-anchors keys and copies a ramp; a rate change rescales and drops collisions.
func TestVideoStudioKeyframeEditSemantics(t *testing.T) {
	editing, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio-editing.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(editing), `value="keyframes"`) || !strings.Contains(string(editing), "label('attrKeyframes'") {
		t.Fatal("the paste attributes dialog has no keyframes group")
	}
	runVideoStudioKeyframesScript(t, `
const keys = () => ({
  opacity: [kf(110, 1, 'hold'), kf(140, 0.2), kf(141, 0.6, 'hold'), kf(160, 0.9), kf(170, 0.3, 'hold'), kf(171, 0.8)],
  x: [kf(90, 0.1, 'ease_in'), kf(120, 0.5), kf(150, 0.2, 'ease_in_out'), kf(210, 0.7, 'ease_out')]
});
const animated = () => project([track('v1','video',[clip('c',V,0,100,{offset:100, keyframes: keys()})]), track('a1','audio',[clip('m',A,0,100)])]);
const value = (p, id, path, frame) => FX.valueAt(at(p, id), path, frame, p);
const original = animated();
let p = animated();
valid(p);

// Split: both parts keep every key, and the values stay continuous across the cut.
assert.equal(E.blade(p, 40, {clipIds: ['c'], linked: false}, {}), true);
const right = p.tracks[0].clips.find(c => c.id !== 'c');
assert.deepEqual(at(p,'c').keyframes, keys());
assert.deepEqual(right.keyframes, keys());
for (const frame of [39, 40, 41, 70]) for (const path of ['x', 'opacity']) near(FX.valueAt(frame < 40 ? at(p,'c') : right, path, frame, p), value(original, 'c', path, frame), 'split ' + path + '@' + frame);
valid(p);

// Start trim, slip and speed: no remapping; the picture content keeps its values.
p = animated();
assert.equal(E.rippleTrim(p, ['c'], 'start', 10), true);
assert.deepEqual(at(p,'c').keyframes, keys());
for (const frame of [0, 25, 89]) for (const path of ['x', 'opacity']) near(value(p, 'c', path, frame), value(original, 'c', path, frame + 10), 'trim ' + path + '@' + frame);
p = animated();
assert.equal(E.slip(p, ['c'], 5), true);
for (const frame of [0, 50, 94]) near(value(p, 'c', 'x', frame), value(original, 'c', 'x', frame + 5), 'slip @' + frame);
p = animated();
assert.equal(E.setSpeed(p, ['c'], 2, true), true);
assert.deepEqual(at(p,'c').keyframes, keys());
for (const frame of [0, 10, 40]) near(value(p, 'c', 'x', frame), value(original, 'c', 'x', 2 * frame), 'speed 2 @' + frame);
valid(p);

// Reverse: keys mirror around the reversed asset's last frame (pivot = rangeStart + frames - 1 = 199), ease_in and
// ease_out swap, and every hold longer than one frame gets an extra key one source frame after its mirrored end.
p = animated();
assert.deepEqual(E.reverseRange(p, 'c'), {rangeStart: 100, frames: 100});
p.assets.push({id:'r', kind:'video', has_audio:true, duration_frames:100});
assert.equal(E.reverseClips(p, 'c', {assetId:'r', rangeStart:100, frames:100, sourceAssetId:'v'}), true);
assert.deepEqual(at(p,'c').keyframes, {
  opacity: [kf(28, 0.8, 'hold'), kf(29, 0.3), kf(39, 0.9, 'hold'), kf(40, 0.6), kf(58, 0.6), kf(59, 0.2, 'hold'), kf(60, 1), kf(89, 1)],
  x: [kf(-11, 0.7, 'ease_in_out'), kf(49, 0.2), kf(79, 0.5, 'ease_out'), kf(109, 0.1, 'ease_in')]
});
valid(p);
// The reversed clip shows at timeline frame 99 - T what the original showed at T (whole source frames at speed 1).
for (let frame = 0; frame < 100; frame++) for (const path of ['x', 'opacity']) near(value(p, 'c', path, 99 - frame), value(original, 'c', path, frame), 'mirror ' + path + '@' + frame);
assert.deepEqual(FX.mirrorKeyframes(FX.mirrorKeyframes({x: keys().x}, 199), 199), {x: keys().x}, 'mirroring twice gives the keys back');
assert.equal(FX.mirrorKeyframes(undefined, 199), undefined);
// Extra hold keys that would pass 64 keys per value refuse the reverse and leave the project as it was.
p = project([track('v1','video',[clip('c',V,0,100,{offset:100, keyframes: {opacity: Array.from({length: 40}, (_, i) => kf(100 + 2 * i, i % 2, 'hold'))}})])]);
valid(p);
p.assets.push({id:'r', kind:'video', has_audio:true, duration_frames:100});
const untouched = JSON.stringify(p);
assert.equal(E.reverseClips(p, 'c', {assetId:'r', rangeStart:100, frames:100, sourceAssetId:'v'}), 'limit');
assert.equal(JSON.stringify(p), untouched);

// Freeze frame: the still takes the values shown at the frozen frame, its box made valid as fields, and no keys.
p = project([track('v1','video',[clip('c',V,0,100,{offset:100, keyframes: {
  x: [kf(100, 0.5), kf(200, 0.9)], width: [kf(100, 0.4), kf(200, 0.8)], rotation: [kf(100, 0), kf(200, 90)], opacity: [kf(100, 1, 'hold'), kf(160, 0.5)]
}})])]);
valid(p);
assert.equal(E.freezeFrame(p, 'c', 50, {asset_id: 'i', clip_id: 'fz', duration: 30}), true);
const still = at(p, 'fz');
assert.equal('keyframes' in still, false);
near(still.width, 0.6, 'width at source 150');
near(still.x, 0.4, 'x (0.7 at source 150) is pulled in so x + width <= 1');
near(still.rotation, 45, 'rotation');
near(still.opacity, 1, 'the hold keeps opacity 1');
assert(still.x + still.width <= 1);
assert.deepEqual(at(p,'c').keyframes.x, [kf(100, 0.5), kf(200, 0.9)], 'the cut parts keep their keys');
valid(p);

// Paste attributes. The keyframes group keeps each key's timing from the clip start on a clip with another speed and
// offset (re-anchored through both time maps). Audio clips are skipped, and a source without keys clears them.
p = project([
  track('v1','video',[clip('c',V,0,100,{offset:100, keyframes: {x: [kf(100, 0), kf(150, 0.5)], opacity: [kf(120, 0.2, 'ease_out')]}})]),
  track('v2','video',[clip('d',V,200,100,{offset:300, speed:2})]),
  track('o1','overlay',[clip('t',I,0,60)]),
  track('a1','audio',[clip('m',A,0,100)])
]);
valid(p);
assert.equal(E.pasteAttributes(p, at(p,'c'), ['d','t','m'], ['keyframes']), true);
assert.deepEqual(at(p,'d').keyframes, {opacity: [kf(340, 0.2, 'ease_out')], x: [kf(300, 0), kf(400, 0.5)]});
assert.deepEqual(at(p,'t').keyframes, {opacity: [kf(20, 0.2, 'ease_out')], x: [kf(0, 0), kf(50, 0.5)]}, 'a still: source time is clip time');
assert.equal('keyframes' in at(p,'m'), false, 'audio clips take no keyframes');
for (let k = 0; k <= 50; k += 5) near(value(p, 'd', 'x', 200 + k), value(p, 'c', 'x', k), 're-anchored @' + k);
valid(p);
assert.equal(E.pasteAttributes(p, at(p,'m'), ['d'], ['keyframes']), true);
assert.equal('keyframes' in at(p,'d'), false, 'pasting from a clip without keys clears them');
// The speed group copies a ramp (the clip keeps its duration); stills keep their timing.
const ramped = Object.assign(JSON.parse(JSON.stringify(at(p,'c'))), {ramp: [{f: 0, speed: 1, e: 'linear'}, {f: 49, speed: 2, e: 'linear'}]});
assert.equal(E.pasteAttributes(p, ramped, ['d', 't'], ['speed']), true);
assert.deepEqual(at(p,'d').ramp, ramped.ramp);
assert.equal('speed' in at(p,'d'), false, 'a ramp replaces the constant speed');
assert.equal(at(p,'d').duration, 100);
assert.equal('ramp' in at(p,'t'), false, 'stills take no ramp');
valid(p);
p = project([track('v1','video',[clip('e',V,0,5,{offset:595})])]);
assert.equal(E.pasteAttributes(p, ramped, ['e'], ['speed']), 'media', 'the media ends before the ramped clip does');

// Rate change: key frames scale like the offset (rounded); keys that land on the frame before them keep the first.
p = project([track('v1','video',[clip('c',V,0,100,{offset:10, keyframes: {x: [kf(10, 0.1), kf(11, 0.2), kf(12, 0.3), kf(40, 0.4)], opacity: [kf(-3, 0.5)]}})])]);
p.assets = p.assets.map(asset => Object.assign({}, asset));
p.fps = 60;
window.VideoStudioTime.rescaleProject(p, 24, false);
assert.equal(at(p,'c').offset, 4);
assert.deepEqual(at(p,'c').keyframes, {opacity: [kf(-1, 0.5)], x: [kf(4, 0.1), kf(5, 0.3), kf(16, 0.4)]});
valid(p);
`)
}

// Keyframe operations (index §4.18): the stopwatch on and off (the field keeps the value at the playhead, clamped to a
// valid box), the diamond (removing the last key ends the animation), auto-key on animated and static values, ◀▶
// navigation inside the clip, moves that collide, easing, copy and paste across speeds, and the per-value limit.
func TestVideoStudioKeyframeOps(t *testing.T) {
	runVideoStudioKeyframesScript(t, `
const fresh = () => project([
  track('v1','video',[clip('c',V,0,100,{offset:100, x:0.2, width:0.5})]),
  track('v2','video',[clip('d',V,200,100,{speed:2})]),
  track('a1','audio',[clip('m',A,0,100)]),
  track('o1','overlay',[clip('t',I,0,60)])
]);
let p = fresh();
valid(p);
const c = () => at(p, 'c');

// Keys sit at round(sourceAt(clip, frame)).
assert.equal(K.keyFrame(c(), 10), 110);
assert.equal(K.keyFrame(at(p,'d'), 210), 20, 'speed 2 from offset 0');

// Stopwatch on: one key at the playhead holding the value shown there.
assert.equal(K.setAnimated(p, 'c', 'opacity', true, 10), true);
assert.deepEqual(c().keyframes, {opacity: [kf(110, 1)]});
assert.deepEqual(K.atPlayhead(c(), 'opacity', 10), kf(110, 1));
assert.equal(K.atPlayhead(c(), 'opacity', 11), null);
assert.equal(K.setAnimated(p, 'c', 'opacity', true, 20), 'nothing', 'already animated');
assert.equal(K.setAnimated(p, 'c', 'x', true, 150), 'nothing', 'the playhead is outside the clip');
assert.equal(K.setAnimated(p, 'c', 'fit', true, 10), 'nothing', 'fit is not keyframable');
assert.equal(K.setAnimated(p, 'm', 'opacity', true, 10), 'kind', 'audio clips have no picture');
p.tracks[1].locked = true;
assert.equal(K.setAnimated(p, 'd', 'x', true, 210), 'locked');
p.tracks[1].locked = false;
valid(p);

// Auto-key: an animated value gets a key at the playhead; a new key takes the easing of the key before it, and an
// existing key takes the value and keeps its easing.
assert.equal(K.setValues(p, 'c', {opacity: 0.4}, 40), true);
assert.deepEqual(c().keyframes.opacity, [kf(110, 1), kf(140, 0.4)]);
assert.equal(K.setEasing(p, 'c', [{path: 'opacity', f: 110}], 'ease_in'), true);
assert.equal(K.setValues(p, 'c', {opacity: 0.9}, 20), true);
assert.deepEqual(c().keyframes.opacity, [kf(110, 1, 'ease_in'), kf(120, 0.9, 'ease_in'), kf(140, 0.4)]);
assert.equal(K.setValues(p, 'c', {opacity: 0.8}, 20), true);
assert.deepEqual(c().keyframes.opacity[1], kf(120, 0.8, 'ease_in'));
valid(p);

// Easing: registry names only; no change is no edit.
assert.equal(K.setEasing(p, 'c', [{path: 'opacity', f: 120}], 'bounce'), 'nothing');
assert.equal(K.setEasing(p, 'c', [{path: 'opacity', f: 120}], 'ease_in'), 'nothing');
assert.equal(K.setEasing(p, 'c', [{path: 'opacity', f: 120}, {path: 'opacity', f: 140}], 'hold'), true);
assert.deepEqual(c().keyframes.opacity.map(key => key.e), ['ease_in', 'hold', 'hold']);

// Stopwatch off: the keys go and the field takes the value at the playhead.
assert.equal(K.setAnimated(p, 'c', 'opacity', false, 30), true);
assert.equal('keyframes' in c(), false, 'the last animated value leaves no empty map');
near(c().opacity, 0.8, 'the held value at source 130');

// The fields stay a valid box: an animated x leaves its field alone, and a static width next to it makes x's unused
// field give way. Turning x off writes its value at the playhead, clamped against the static width.
assert.equal(K.setAnimated(p, 'c', 'x', true, 0), true);
assert.deepEqual(c().keyframes, {x: [kf(100, 0.2)]});
assert.equal(K.setValues(p, 'c', {x: 0.9}, 0), true);
assert.equal(c().x, 0.2, 'an animated value leaves its field alone');
assert.equal(K.setValues(p, 'c', {width: 0.9}, 0), true);
near(c().width, 0.9, 'a static value changes its field');
near(c().x, 0.1, 'the field of the animated x gives way');
valid(p);
assert.equal(K.setAnimated(p, 'c', 'x', false, 0), true);
near(c().x, 0.1, 'x off at 0.9 next to width 0.9 is clamped to 0.1');
valid(p);

// One auto-key call with an animated and a static value.
assert.equal(K.setAnimated(p, 'c', 'rotation', true, 0), true);
assert.equal(K.setValues(p, 'c', {rotation: 30, y: 0.05}, 50), true);
assert.deepEqual(c().keyframes.rotation, [kf(100, 0), kf(150, 30)]);
assert.equal(c().y, 0, 'a static y of 0.05 next to the static height 1 is clamped to a valid box');
assert.equal('y' in c().keyframes, false);
assert.equal(K.setValues(p, 'c', {rotation: 30, opacity: 0.6}, 50), true, 'an animated and a static value in one call');
near(c().opacity, 0.6, 'the static value changes its field');
assert.deepEqual(c().keyframes.rotation, [kf(100, 0), kf(150, 30)]);
assert.equal('opacity' in c().keyframes, false);
assert.equal(K.setValues(p, 'c', {rotation: 10}, 150), 'nothing', 'no key outside the clip');
assert.equal(K.setValues(p, 'c', {fit: 'cover'}, 10), 'nothing', 'only keyframable values');
valid(p);

// The diamond adds a key holding the value shown and removes the key at the playhead. Removing the last key ends the
// animation and leaves that value in the field; on a static value the diamond starts the animation.
assert.equal(K.toggleKeyframe(p, 'c', 'rotation', 25), true);
near(K.atPlayhead(c(), 'rotation', 25).v, 15, 'the value shown at frame 25');
assert.equal(K.toggleKeyframe(p, 'c', 'rotation', 25), true);
assert.equal(K.atPlayhead(c(), 'rotation', 25), null);
assert.equal(K.toggleKeyframe(p, 'c', 'rotation', 50), true);
assert.equal(K.toggleKeyframe(p, 'c', 'rotation', 0), true, 'removes the last key');
assert.equal('rotation' in (c().keyframes || {}), false);
assert.equal(c().rotation, 0);
assert.equal(K.toggleKeyframe(p, 'c', 'height', 10), true);
assert.deepEqual(c().keyframes.height, [kf(110, 1)]);
valid(p);

// ◀▶: whole timeline frames of keys inside the clip; keys outside it are skipped.
p = fresh();
at(p,'c').keyframes = {x: [kf(90, 0), kf(120, 0.2), kf(170, 0.4), kf(250, 0.6)], opacity: [kf(130, 0.5)]};
at(p,'d').keyframes = {x: [kf(20, 0), kf(41, 1)]};
valid(p);
assert.equal(K.neighbor(c(), ['x'], 50, -1), 20);
assert.equal(K.neighbor(c(), ['x'], 50, 1), 70);
assert.equal(K.neighbor(c(), ['x'], 70, 1), null, 'the key at timeline 150 is past the clip end');
assert.equal(K.neighbor(c(), ['x'], 20, -1), null, 'the key at timeline -10 is before the clip start');
assert.equal(K.neighbor(c(), ['x', 'opacity'], 20, 1), 30, 'the nearest key of any listed value');
assert.equal(K.neighbor(at(p,'d'), ['x'], 200, 1), 210, 'speed 2: source 20 is 10 timeline frames in');
assert.equal(K.neighbor(at(p,'d'), ['x'], 210, 1), 221, 'source 41 sits at 220.5 and rounds to 221');

// Moves go by whole timeline frames through the time map; landing on another key of the value refuses.
assert.equal(K.moveKeys(p, 'c', [{path: 'x', f: 120}], 50), 'bounds', 'onto the key at 170');
assert.equal(K.moveKeys(p, 'c', [{path: 'x', f: 120}], 5), true);
assert.deepEqual(c().keyframes.x.map(key => key.f), [90, 125, 170, 250]);
assert.equal(K.moveKeys(p, 'c', [{path: 'x', f: 125}, {path: 'x', f: 170}], 100), true, 'a key may pass another one');
assert.deepEqual(c().keyframes.x.map(key => key.f), [90, 225, 250, 270]);
assert.equal(K.moveKeys(p, 'd', [{path: 'x', f: 20}], 3), true);
assert.deepEqual(at(p,'d').keyframes.x.map(key => key.f), [26, 41], 'speed 2: three timeline frames are six source frames');
assert.equal(K.moveKeys(p, 'c', [{path: 'x', f: 90}], 0), 'nothing');
valid(p);

// Removing keys; a value left without keys ends its animation with the value at the clip's first frame in its field.
assert.equal(K.removeKeys(p, 'c', [{path: 'opacity', f: 130}]), true);
assert.equal('opacity' in c().keyframes, false);
near(c().opacity, 0.5, 'the removed key held 0.5 everywhere');
assert.equal(K.removeKeys(p, 'c', [{path: 'opacity', f: 130}]), 'nothing');
valid(p);

// Copy and paste between clips of different speeds keeps the timeline timing relative to the given frames.
p = fresh();
at(p,'c').keyframes = {x: [kf(110, 0.1), kf(130, 0.5, 'ease_out')], opacity: [kf(120, 0.3)]};
valid(p);
const board = K.copyKeys(c(), [{path: 'x', f: 110}, {path: 'x', f: 130}, {path: 'opacity', f: 120}], 10);
assert.deepEqual(board, {keys: [{path: 'x', dt: 0, v: 0.1, e: 'linear'}, {path: 'opacity', dt: 10, v: 0.3, e: 'linear'}, {path: 'x', dt: 20, v: 0.5, e: 'ease_out'}]});
assert.equal(K.copyKeys(c(), [{path: 'y', f: 0}], 10), null);
assert.equal(K.pasteKeys(p, 'd', board, 220), true);
assert.deepEqual(at(p,'d').keyframes, {x: [kf(40, 0.1), kf(80, 0.5, 'ease_out')], opacity: [kf(60, 0.3)]});
for (let k = 0; k <= 20; k++) near(FX.valueAt(at(p,'d'), 'x', 220 + k, p), FX.valueAt(c(), 'x', 10 + k, p), 'pasted timing @' + k);
assert.equal(K.pasteKeys(p, 'm', board, 0), 'kind');
assert.equal(K.pasteKeys(p, 'd', {keys: []}, 220), 'nothing');
valid(p);

// 64 keys per value. The 512 per clip cannot be reached with the six base values of 2b.
p = fresh();
at(p,'c').keyframes = {x: Array.from({length: 64}, (_, i) => kf(100 + i, 0.5))};
valid(p);
assert.equal(K.toggleKeyframe(p, 'c', 'x', 80), 'limit');
assert.equal(K.setValues(p, 'c', {x: 0.2}, 90), 'limit');
assert.equal(K.setValues(p, 'c', {x: 0.2}, 10), true, 'an existing key still takes a value');
assert.equal(K.pasteKeys(p, 'c', {keys: [{path: 'x', dt: 0, v: 0.3, e: 'linear'}]}, 70), 'limit');
`)
}

// videoStudioKeyframePartsPrelude loads the app parts the keyframe UI talks to (inspector, core, editing, source
// monitor, transport) and video-studio-keyframes.js on top of the keyframe prelude; none of them touches the DOM at load.
const videoStudioKeyframePartsPrelude = `
require('./js/desktop/apps/video-studio-inspector.js');
require('./js/desktop/apps/video-studio-core.js');
require('./js/desktop/apps/video-studio-editing.js');
require('./js/desktop/apps/video-studio-source.js');
require('./js/desktop/apps/video-studio-transport.js');
require('./js/desktop/apps/video-studio-keyframes.js');
const P = window.VideoStudioParts, KF = window.VideoStudioKeyframes, IN = window.VideoStudioInspector;
`

// Keyframe shortcuts (index §4.18): Alt+K, Alt+ArrowUp and Alt+ArrowDown are matched by event.code (macOS Option changes
// event.key) and collide with no edit, navigation or source shortcut; handleKeys tries them before the edit keys and the
// shortcut overview lists them.
func TestVideoStudioKeyframeShortcuts(t *testing.T) {
	app, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio.js"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	keys, edit := strings.Index(text, "P.handleKeyframeKeys(s, event)"), strings.Index(text, "P.handleEditKeys(s, event, focusedClip)")
	if keys < 0 || edit < keys {
		t.Fatal("handleKeys must try P.handleKeyframeKeys before the edit shortcuts")
	}
	if !strings.Contains(text, "P.isKeyframeShortcut(s, event)") {
		t.Fatal("the modifier guard in handleKeys must let the keyframe shortcuts through")
	}
	for _, key := range []string{"'shortcutKeyframe'", "'shortcutKeyframeNav'"} {
		if !strings.Contains(text, key) {
			t.Errorf("the shortcut overview does not list %s", key)
		}
	}
	runVideoStudioKeyframesScript(t, videoStudioKeyframePartsPrelude+`
const timeline = {stageMode: 'timeline', sourceAssetId: 'v', q: () => null};
const source = Object.assign({}, timeline, {stageMode: 'source'});
const ev = (code, key, mods) => Object.assign({code, key, altKey: false, ctrlKey: false, metaKey: false, shiftKey: false}, mods || {});
// Matched by code whatever key says; other modifier sets do not count.
for (const [code, keys] of [['KeyK', ['k', 'K', '˚']], ['ArrowUp', ['ArrowUp']], ['ArrowDown', ['ArrowDown']]]) {
  for (const key of keys) assert(P.isKeyframeShortcut(timeline, ev(code, key, {altKey: true})), code + ' with key ' + key);
  for (const mods of [{}, {altKey: true, shiftKey: true}, {altKey: true, ctrlKey: true}, {altKey: true, metaKey: true}, {ctrlKey: true}]) assert(!P.isKeyframeShortcut(timeline, ev(code, keys[0], mods)), code + ' ' + JSON.stringify(mods));
  assert(!P.isKeyframeShortcut(source, ev(code, keys[0], {altKey: true})), 'the source monitor keeps its keys');
}
// Every key with every Alt/Ctrl/Shift combination: the keyframe shortcuts are exactly three and collide with nothing.
const codes = [];
for (let c = 65; c <= 90; c++) codes.push(['Key' + String.fromCharCode(c), String.fromCharCode(c + 32)]);
for (let d = 0; d <= 9; d++) codes.push(['Digit' + d, String(d)]);
codes.push(['ArrowUp', 'ArrowUp'], ['ArrowDown', 'ArrowDown'], ['ArrowLeft', 'ArrowLeft'], ['ArrowRight', 'ArrowRight'], ['Delete', 'Delete'], ['Backspace', 'Backspace'], ['Comma', ','], ['Period', '.'], ['Space', ' '], ['Home', 'Home'], ['End', 'End']);
let found = 0;
for (const [code, key] of codes) for (const altKey of [false, true]) for (const ctrlKey of [false, true]) for (const shiftKey of [false, true]) {
  const event = ev(code, key, {altKey, ctrlKey, shiftKey});
  if (!P.isKeyframeShortcut(timeline, event)) continue;
  found++;
  const name = code + ' ' + JSON.stringify({altKey, ctrlKey, shiftKey});
  assert(!P.isEditShortcut(timeline, event), 'edit collision: ' + name);
  assert(!P.isNavShortcut(timeline, event), 'navigation collision: ' + name);
  assert(!P.isSourceShortcut(timeline, event) && !P.isSourceShortcut(source, event), 'source collision: ' + name);
}
assert.equal(found, 3, 'Alt+K, Alt+ArrowUp and Alt+ArrowDown');
`)
}

// Keyframe controls: controlState per value, P.keyframeView for the selected clip, and the inspector's Picture section
// built from it (values at the playhead; stopwatch, diamond and ◀▶ per value; frozen while the playhead is outside).
func TestVideoStudioKeyframeControls(t *testing.T) {
	runVideoStudioKeyframesScript(t, videoStudioKeyframePartsPrelude+`
let p = project([track('v1','video',[clip('c',V,0,100,{offset:100, keyframes:{x:[kf(100, 0), kf(130, 0.6), kf(260, 1)]}})])]);
valid(p);
const c = at(p, 'c');
assert.deepEqual(KF.controlState(p, c, 'x', 0), {animated: true, onKey: true, prev: null, next: 30, value: 0});
const mid = KF.controlState(p, c, 'x', 15);
assert.deepEqual([mid.animated, mid.onKey, mid.prev, mid.next], [true, false, 0, 30]);
near(mid.value, 0.3, 'the value shown at frame 15');
assert.equal(KF.controlState(p, c, 'x', 40).next, null, 'the key at timeline 160 lies past the clip end');
assert.deepEqual(KF.controlState(p, c, 'opacity', 15), {animated: false, onKey: false, prev: null, next: null, value: 1});
c.speed = 2; c.duration = 60;
valid(p);
assert.deepEqual(KF.controlState(p, c, 'x', 15), {animated: true, onKey: true, prev: 0, next: null, value: 0.6}, 'speed 2: source 130 is frame 15');
c.speed = 1; c.duration = 100;

// P.keyframeView: the selected visual clip at the playhead.
const s = {project: p, selectedClipId: 'c', frame: 15, readonly: false};
let view = P.keyframeView(s);
assert.deepEqual(Object.keys(view.controls).sort(), ['height', 'opacity', 'rotation', 'width', 'x', 'y']);
assert.equal(view.inside, true);
assert.equal(view.locked, false);
assert.equal(view.boxAnimated, true);
near(view.state.x, 0.3, 'state at the playhead');
assert.equal(P.keyframeView({project: p, selectedClipId: '', frame: 0}), null);

// The inspector draws the controls from the view.
const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, ch => '&#' + ch.charCodeAt(0) + ';');
const render = keyframes => {
  const host = {innerHTML: ''}, track0 = p.tracks[0];
  IN.render(host, {selected: {clip: track0.clips[0], track: track0}, asset: V, project: p, locale: 'en', caps: {}, transitionInfo: {next: null},
    esc, icon: name => '<i data-icon="' + name + '"></i>', tr: key => '[' + key + ':{{name}}]', keyframes, fineOpen: true});
  return host.innerHTML;
};
let html = render(view);
const tag = (path, action) => (html.match(new RegExp('<button[^>]*data-kf-path="' + path + '" data-kf-action="' + action + '"[^>]*>')) || [''])[0];
for (const path of ['x', 'y', 'width', 'height', 'rotation', 'opacity']) for (const action of ['stopwatch', 'key', 'prev', 'next']) assert(tag(path, action), 'missing ' + path + ' ' + action);
assert(tag('x', 'stopwatch').includes('aria-pressed="true"'), 'x is animated');
assert(tag('x', 'key').includes('aria-pressed="false"'), 'no x key at frame 15');
assert(!/ disabled/.test(tag('x', 'prev')) && !/ disabled/.test(tag('x', 'next')));
assert(tag('opacity', 'stopwatch').includes('aria-pressed="false"') && / disabled/.test(tag('opacity', 'prev')), 'a static value has no neighbours');
assert(html.includes('[keyframeAdd:[box_x:{{name}}]]'), 'labels come from the locale keys with the value name filled in');
assert(/<input type="number" data-field="x"[^>]*value="30"/.test(html), 'fields show the value at the playhead');
assert(html.includes('data-icon="diamond"') && !html.includes('data-icon="diamondFilled"'));
s.frame = 0;
html = render(P.keyframeView(s));
assert(tag('x', 'key').includes('aria-pressed="true"') && html.includes('data-icon="diamondFilled"'), 'on a key the diamond is filled');
s.frame = 120;
view = P.keyframeView(s);
html = render(view);
assert.equal(view.inside, false);
assert(/ disabled/.test(tag('x', 'stopwatch')) && /title="\[keyframeOutside/.test(html), 'outside the clip the controls are disabled with a title');
assert(/<input type="number" data-field="x"[^>]*disabled/.test(html), 'an animated value is frozen outside the clip');
assert(!/<input type="number" data-field="y"[^>]*disabled/.test(html), 'a static value stays editable');
assert(/data-position="tl"[^>]*disabled/.test(html), 'the position grid moves x and y');
assert.equal(P.keyframeView({project: p, selectedClipId: 'c', frame: 15, readonly: true}).locked, true);
`)
}

// The keyframe strip's geometry (index §4.18 content anchor): every diamond sits at timelineAt(clip, f) in lane pixels,
// through a constant speed or a ramp; keys outside the clip's frames stay in the model, flagged; the summary row has one
// diamond per source frame and lists the paths keyed there.
func TestVideoStudioKeyframeStripModel(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio keyframe strip checks")
	}
	script := `
const assert = require('node:assert/strict');
global.window = { VideoStudioParts: {} };
global.crypto = global.crypto || require('node:crypto').webcrypto;
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-edit.js');
require('./js/desktop/apps/video-studio-keyframe-edit.js');
require('./js/desktop/apps/video-studio-keyframes.js');
const K = window.VideoStudioKeyframes, Fx = window.VideoStudioEffects;
const project = {fps: 30};
const near = (a, b, message) => assert(Math.abs(a - b) < 1e-9, message + ': ' + a + ' vs ' + b);
// Speed 1.5 from source frame 15: f 45 shows at timeline frame 30 + 30 / 1.5 = 50.
const fast = {id: 'c', asset_id: 'v', start: 30, offset: 15, duration: 60, speed: 1.5, keyframes: {
  opacity: [{f: 15, v: 0, e: 'linear'}, {f: 45, v: 1, e: 'ease_in'}, {f: 200, v: 1, e: 'linear'}],
  x: [{f: 0, v: 0, e: 'linear'}, {f: 45, v: 0.5, e: 'hold'}]}};
let model = K.stripModel(project, fast, 52, {expanded: false});
assert.equal(model.clipId, 'c');
near(model.left, 52, 'the clip starts at one second');
near(model.width, 104, 'sixty frames are two seconds');
assert.equal(model.expanded, false);
assert.deepEqual(model.rows.map(row => row.id), ['summary'], 'collapsed: the summary row only');
const summary = model.rows[0].keys;
assert.deepEqual(summary.map(key => [key.f, key.paths, key.e, key.outside]),
  [[0, ['x'], 'linear', true], [15, ['opacity'], 'linear', false], [45, ['x', 'opacity'], 'mixed', false], [200, ['opacity'], 'linear', true]]);
near(summary[1].frame, 30, 'source frame 15 is the first clip frame');
near(summary[2].frame, 50, 'f 45 at speed 1.5');
near(summary[2].x, 50 / 30 * 52, 'lane pixels follow pps');
near(summary[0].frame, 20, 'a key before the offset maps before the clip');
model = K.stripModel(project, fast, 104, {expanded: true});
assert.deepEqual(model.rows.map(row => row.id), ['summary', 'x', 'opacity'], 'expanded: one row per animated path in registry order');
const opacity = model.rows[2].keys;
assert.deepEqual(opacity.map(key => [key.path, key.f, key.v, key.e, key.outside]),
  [['opacity', 15, 0, 'linear', false], ['opacity', 45, 1, 'ease_in', false], ['opacity', 200, 1, 'linear', true]]);
near(opacity[1].x, 50 / 30 * 104, 'zoom doubles the pixels');
// A ramp places diamonds through the ramp's inverse: speed 1 to 2 over 49 frames reaches source 30 at t = 24.08.
const ramped = {id: 'r', asset_id: 'v', start: 0, offset: 0, duration: 50,
  ramp: [{f: 0, speed: 1, e: 'linear'}, {f: 49, speed: 2, e: 'linear'}], keyframes: {opacity: [{f: 30, v: 1, e: 'linear'}]}};
const key = K.stripModel(project, ramped, 52, {expanded: true}).rows[1].keys[0];
near(key.frame, Fx.timelineAt(ramped, 30), 'diamonds use the shared time map');
assert(Math.abs(key.frame - 24.0821) < 1e-3, 'ramp inverse: ' + key.frame);
assert(Math.abs(key.frame - 30) > 5, 'a ramp does not place keys at the constant-speed position');
// Stills: offset 0 and speed 1, so source frames are clip frames.
const still = {id: 's', asset_id: 'i', start: 10, offset: 0, duration: 20, keyframes: {rotation: [{f: 5, v: 90, e: 'linear'}]}};
near(K.stripModel(project, still, 52, null).rows[0].keys[0].frame, 15, 'a still keys at start + f');
assert.deepEqual(K.stripModel(project, {id: 'n', start: 0, offset: 0, duration: 10}, 52, {expanded: true}).rows.map(row => [row.id, row.keys.length]),
  [['summary', 0]], 'no keyframes: an empty summary');
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("keyframe strip checks: %v\n%s", err, output)
	}
}

// Every desktop locale carries the keyframe strip strings, and the sentences are translated.
func TestVideoStudioKeyframeStripLocales(t *testing.T) {
	keys := []string{"keyframes", "keyframeSummary", "keyframeExpand", "keyframeCollapse", "easing_linear", "easing_ease_in", "easing_ease_out", "easing_ease_in_out", "easing_hold", "keyframeCopy", "keyframePaste", "keyframeDelete"}
	english := readDesktopLocale(t, "en")
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := readDesktopLocale(t, lang)
		for _, key := range keys {
			if strings.TrimSpace(values["videoStudio."+key]) == "" {
				t.Errorf("%s lacks videoStudio.%s", lang, key)
			}
		}
		if lang == "en" {
			continue
		}
		for _, key := range []string{"keyframeSummary", "keyframeExpand", "keyframeCollapse", "keyframeCopy", "keyframePaste", "keyframeDelete"} {
			if values["videoStudio."+key] == english["videoStudio."+key] {
				t.Errorf("%s copies the English videoStudio.%s", lang, key)
			}
		}
	}
}

// Ramp presets and lane operations (index §4.18).
// - Presets start from the clip's speed. By default they keep the source range and move later clips on the track;
//   Keep duration keeps the length, and the media must hold the new range.
// - 'none' is setSpeed(1).
// - Lane edits keep the duration; an invalid fit is refused.
// - The strip model gains the speed row.
func TestVideoStudioRampOps(t *testing.T) {
	runVideoStudioEditScript(t, `
window.VideoStudioParts = window.VideoStudioParts || {};
require('./js/desktop/apps/video-studio-keyframe-edit.js');
require('./js/desktop/apps/video-studio-keyframes.js');
const KE = window.VideoStudioKeyframeEdit, Fx = window.VideoStudioEffects, K = window.VideoStudioKeyframes;
const near = (a, b, message) => assert(Math.abs(a - b) < 1e-9, message + ': ' + a + ' vs ' + b);
// Preset shapes.
assert.deepEqual(KE.rampPreset('ramp_up', 100, 1), [{f:0,speed:1,e:'ease_in_out'},{f:99,speed:2,e:'linear'}]);
assert.deepEqual(KE.rampPreset('ramp_down', 100, 1.5), [{f:0,speed:3,e:'ease_in_out'},{f:99,speed:1.5,e:'linear'}]);
assert.deepEqual(KE.rampPreset('slow_middle', 181, 1), [{f:0,speed:1,e:'ease_in_out'},{f:72,speed:0.25,e:'linear'},{f:108,speed:0.25,e:'ease_in_out'},{f:180,speed:1,e:'linear'}]);
assert.deepEqual(KE.rampPreset('custom', 2, 0.5), [{f:0,speed:0.5,e:'linear'},{f:1,speed:0.5,e:'linear'}]);
assert.deepEqual(KE.rampPreset('slow_middle', 1, 1).map(p => p.f), [0, 1, 2, 3], 'tiny clips keep strictly increasing points');
assert.equal(KE.rampPreset('ramp_up', 100, 12)[1].speed, 16, 'speeds are clamped to 16');
assert.equal(KE.rampPreset('none', 100, 1), null);
// Default: keep the source range. 100 frames at 1x consume 99 source frames; 1x to 2x eased in and out averages 1.5x.
let p = base();
assert.equal(KE.applyRampPreset(p, ['x'], 'ramp_up', false), true);
assert.equal(at(p,'x').duration, 67);
near(Fx.sourceExtent(at(p,'x')), 99, 'the source range is kept');
assert.equal(at(p,'x').speed, undefined);
assert.deepEqual(at(p,'x').ramp, [{f:0,speed:1,e:'ease_in_out'},{f:66,speed:2,e:'linear'}]);
assert.deepEqual(spans(p,'v1'), [[0,67,0],[67,100,200],[167,100,400]], 'later clips on the track move up');
assert.deepEqual(spans(p,'v2'), [[50,60,0]], 'other tracks stay');
valid(p);
// Slow middle: the same 99 source frames take 181 frames (0.625 x 72 + 0.25 x 36 + 0.625 x 72).
p = base();
assert.equal(KE.applyRampPreset(p, ['x'], 'slow_middle', false), true);
assert.equal(at(p,'x').duration, 181);
near(Fx.sourceExtent(at(p,'x')), 99, 'slow middle keeps the source range');
assert.deepEqual(spans(p,'v1').map(s => s[0]), [0, 181, 281]);
valid(p);
// Keep duration: the length stays and the ramp consumes more source.
p = base();
assert.equal(KE.applyRampPreset(p, ['x'], 'ramp_up', true), true);
assert.equal(at(p,'x').duration, 100);
assert.deepEqual(at(p,'x').ramp, [{f:0,speed:1,e:'ease_in_out'},{f:99,speed:2,e:'linear'}]);
near(Fx.sourceExtent(at(p,'x')), 148.5, 'keep duration uses more source');
assert.deepEqual(spans(p,'v1').map(s => s[0]), [0, 100, 200], 'keep duration moves nothing');
// The media must hold the new range: 500 + 147 > 599 with Keep duration; without it the clip gets shorter.
p = project([track('v1','video',[clip('late',V,0,99,{offset:500})])]);
assert.equal(KE.applyRampPreset(p, ['late'], 'ramp_up', true), E.REASON.media);
assert.equal(KE.applyRampPreset(p, ['late'], 'ramp_up', false), true);
assert.equal(at(p,'late').duration, 66);
// 'none' goes back to 1x with the same source range.
p = base();
assert.equal(KE.applyRampPreset(p, ['x'], 'ramp_up', false), true);
assert.equal(KE.applyRampPreset(p, ['x'], 'none', false), true);
assert.equal(at(p,'x').ramp, undefined);
assert.equal(at(p,'x').duration, 100);
assert.deepEqual(spans(p,'v1').map(s => s[0]), [0, 100, 200]);
valid(p);
// Refusals.
p = base();
assert.equal(KE.applyRampPreset(p, ['m'], 'ramp_up', false), E.REASON.media, 'audio clips take no ramp');
assert.equal(KE.applyRampPreset(p, ['t'], 'ramp_up', false), E.REASON.media, 'an overlay still takes no ramp');
p.tracks[0].locked = true;
assert.equal(KE.applyRampPreset(p, ['x'], 'ramp_up', false), E.REASON.locked);
// Lane edits keep the duration; refusals leave the project as it was.
p = base();
assert.equal(KE.addRampPoint(p, 'x', 50, 2), true, 'the first point turns the constant speed into a ramp');
assert.deepEqual(at(p,'x').ramp, [{f:0,speed:1,e:'linear'},{f:50,speed:2,e:'linear'},{f:99,speed:1,e:'linear'}]);
assert.equal(at(p,'x').duration, 100);
assert.equal(KE.moveRampPoint(p, 'x', 1, 99, 2), E.REASON.bounds, 'a point cannot pass its neighbour');
assert.equal(KE.moveRampPoint(p, 'x', 1, 40, 3), true);
assert.deepEqual(at(p,'x').ramp[1], {f:40,speed:3,e:'linear'});
let q = JSON.parse(JSON.stringify(p));
assert.equal(KE.moveRampPoint(q, 'x', 1, 40, 16), E.REASON.media, 'the media must hold the source range');
assert.equal(KE.setRamp(p, 'x', Array.from({length: 17}, (_, i) => ({f: i, speed: 1, e: 'linear'}))), E.REASON.limit);
assert.equal(KE.removeRampPoint(p, 'x', 1), true);
assert.deepEqual(at(p,'x').ramp, [{f:0,speed:1,e:'linear'},{f:99,speed:1,e:'linear'}]);
assert.equal(KE.removeRampPoint(p, 'x', 0), true, 'one point left: the clip plays at that speed');
assert.equal(at(p,'x').ramp, undefined);
assert.equal(at(p,'x').speed, undefined, 'a remaining 1x point leaves no speed field');
valid(p);
assert.equal(KE.setRamp(p, 'x', [{f:0,speed:2,e:'bogus'},{f:10,speed:0.1,e:'linear'}]), true, 'speeds are clamped and unknown easings become linear');
assert.deepEqual(at(p,'x').ramp, [{f:0,speed:2,e:'linear'},{f:10,speed:0.25,e:'linear'}]);
valid(p);
// The strip model's speed row (expanded only): points in clip time and the curve's piece nodes.
p = base();
KE.applyRampPreset(p, ['x'], 'ramp_up', false);
const lane = K.stripModel(p, at(p,'x'), 52, {expanded: true}).rows.find(row => row.id === 'speed');
assert.deepEqual(lane.points.map(point => [point.index, point.f, point.speed, point.outside]), [[0,0,1,false],[1,66,2,false]]);
near(lane.points[1].x, 66 / 30 * 52, 'points sit at start + f');
assert.deepEqual([lane.curve[0].t, lane.curve[0].speed], [0, 1]);
assert.equal(K.stripModel(p, at(p,'x'), 52, {expanded: false}).rows.some(row => row.id === 'speed'), false, 'collapsed strips show the summary only');
`)
}

// Every desktop locale carries the speed ramp strings with their placeholders, and the sentences are translated.
func TestVideoStudioRampLocales(t *testing.T) {
	keys := []string{"animation", "speedRamp", "ramp_none", "ramp_up", "ramp_down", "ramp_slow_middle", "ramp_custom", "rampEdit", "rampMuted", "rampBadge", "speedLane", "speedRampActive", "rampPointSpeed"}
	english := readDesktopLocale(t, "en")
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := readDesktopLocale(t, lang)
		for _, key := range keys {
			if strings.TrimSpace(values["videoStudio."+key]) == "" {
				t.Errorf("%s lacks videoStudio.%s", lang, key)
			}
		}
		if text := values["videoStudio.rampPointSpeed"]; !strings.Contains(text, "{{speed}}") || !strings.Contains(text, "{{time}}") {
			t.Errorf("%s videoStudio.rampPointSpeed lost a placeholder: %q", lang, text)
		}
		if lang == "en" {
			continue
		}
		for _, key := range []string{"speedRamp", "ramp_up", "ramp_down", "ramp_slow_middle", "rampEdit", "rampMuted", "speedLane", "rampPointSpeed"} {
			if values["videoStudio."+key] == english["videoStudio."+key] {
				t.Errorf("%s copies the English videoStudio.%s", lang, key)
			}
		}
	}
}

// Paste attributes, Picture group on animated values (index §4.18): the clip shows the keys of an animated value and
// ignores its field, so the paste writes a key at the playhead there (auto-key, like the inspector) and the field only
// where the value is static. An animated value is not edited while the playhead is outside the clip, and with the
// keyframes group in the same paste the target takes the source's keys and static fields as before.
func TestVideoStudioPasteAttributesPictureKeysAnimatedValues(t *testing.T) {
	runVideoStudioKeyframesScript(t, `
const picture = {x: 0.5, y: 0.25, width: 0.5, height: 0.5, rotation: 15, opacity: 0.7, fit: 'cover'};
const source = () => Object.assign(JSON.parse(JSON.stringify(at(project([track('v1','video',[clip('s',V,0,50)])]), 's'))), picture);
const target = () => project([
  track('v1','video',[clip('d',V,200,100,{offset:300, x:0.1, width:0.6, keyframes: {x: [kf(300, 0.1), kf(400, 0.3)], rotation: [kf(320, 0), kf(380, 45, 'ease_out')]}})]),
  track('a1','audio',[clip('m',A,0,100)])
]);
const d = p => at(p, 'd');
let p = target();
valid(p);

// At timeline 250 (source 350, inside the clip): animated x and rotation get a key there; the fields of the animated
// values stay as they were; the static values take the source's field values.
assert.equal(E.pasteAttributes(p, source(), ['d', 'm'], ['picture'], 250), true);
assert.equal(d(p).x, 0.1, 'the static field of the animated x is untouched');
assert.equal(d(p).rotation, 0, 'and so is the one of the animated rotation');
assert.deepEqual(d(p).keyframes.x, [kf(300, 0.1), kf(350, 0.5), kf(400, 0.3)], 'a key at the playhead holds the pasted x');
assert.deepEqual(d(p).keyframes.rotation, [kf(320, 0), kf(350, 15), kf(380, 45, 'ease_out')], 'a new key takes the easing of the key before it (linear here)');
near(FX.valueAt(d(p), 'x', 250, p), 0.5, 'the clip now shows the pasted x at the playhead');
near(FX.valueAt(d(p), 'rotation', 250, p), 15, 'and the pasted rotation');
assert.deepEqual([d(p).y, d(p).width, d(p).height, d(p).opacity, d(p).fit], [0.25, 0.5, 0.5, 0.7, 'cover'], 'static values take the field');
for (const path of ['y', 'width', 'height', 'opacity']) assert.equal(path in d(p).keyframes, false, path + ' stays static');
assert.equal(at(p, 'm').x, 0, 'audio clips take no picture attributes');
valid(p);

// A key already at the playhead takes the value and keeps its easing.
p = target();
d(p).keyframes.x[0].e = 'ease_in';
assert.equal(E.pasteAttributes(p, source(), ['d'], ['picture'], 200), true);
assert.deepEqual(d(p).keyframes.x[0], kf(300, 0.5, 'ease_in'));
assert.equal(d(p).keyframes.x.length, 2);

// Outside the clip (or without a frame) an animated value is left alone; the static values still change.
for (const frame of [150, 300, undefined]) {
  p = target();
  const before = JSON.stringify(d(p).keyframes);
  assert.equal(E.pasteAttributes(p, source(), ['d'], ['picture'], frame), true, 'the static values still paste at ' + frame);
  assert.equal(JSON.stringify(d(p).keyframes), before, 'no key outside the clip at ' + frame);
  assert.equal(d(p).x, 0.1);
  assert.deepEqual([d(p).y, d(p).width, d(p).opacity], [0.25, 0.5, 0.7]);
  valid(p);
}
p = project([track('v1','video',[clip('d',V,200,100,{offset:300, keyframes: {x: [kf(300, 0.1)]}})])]);
assert.equal(E.pasteAttributes(p, {x: 0.5}, ['d'], ['picture'], 10), 'nothing', 'only an animated value was asked for and the playhead is outside');

// The key limit refuses the whole paste.
p = project([track('v1','video',[clip('d',V,0,100,{keyframes: {x: Array.from({length: 64}, (_, i) => kf(10 + i, 0.2))}})])]);
assert.equal(E.pasteAttributes(p, source(), ['d'], ['picture'], 5), 'limit');

// With the keyframes group too, the target ends up with the source's keys and static fields: nothing is keyed at the
// playhead, and the source without keys leaves the target static with the source's fields.
p = target();
assert.equal(E.pasteAttributes(p, source(), ['d'], ['picture', 'keyframes'], 250), true);
assert.equal('keyframes' in d(p), false, 'the source has no keys, so the target loses its own');
assert.deepEqual([d(p).x, d(p).rotation, d(p).y, d(p).width, d(p).opacity, d(p).fit], [0.5, 15, 0.25, 0.5, 0.7, 'cover']);
valid(p);
const keyed = Object.assign(source(), {keyframes: {opacity: [kf(0, 0), kf(40, 1)]}});
p = target();
assert.equal(E.pasteAttributes(p, keyed, ['d'], ['picture', 'keyframes'], 250), true);
assert.deepEqual(Object.keys(d(p).keyframes), ['opacity'], 'only the source keys remain');
assert.deepEqual([d(p).x, d(p).rotation], [0.5, 15]);
valid(p);
`)
}

// A re-rendered title or caption (index §4.18): the new PNG is sized and centred from the box the clip shows at the
// playhead (keyframes evaluated), because the keys drive the picture and ignore the fields of animated values. The new
// box is written like an inspector edit: a key at the playhead for every animated value, the fields only for static ones.
func TestVideoStudioArtworkCommitUsesTheAnimatedBox(t *testing.T) {
	runVideoStudioKeyframesScript(t, videoStudioKeyframePartsPrelude+`
require('./js/desktop/apps/video-studio-artwork.js');
// Boxes are rounded to 6 decimals (clampBox).
const close = (a, b, message) => assert(Math.abs(a - b) < 1e-5, message + ': ' + a + ' vs ' + b);
const realTimeline = P.T();
P.T = () => new Proxy(realTimeline, {get: (target, key) => key === 'render' ? () => {} : target[key]});
let saved = 0;
Object.assign(P, {scheduleSave: () => { saved++; }, renderInspector() {}, updateTransform() {}});
const OLD = {id: 'art1', kind: 'image', width: 400, height: 100, duration_frames: 18000};
const NEW = {id: 'art2', kind: 'image', width: 600, height: 100, duration_frames: 18000};
const fixture = (extra, keyframes) => {
  const p = project([track('o1', 'overlay', [clip('t', OLD, 0, 90, Object.assign({text: 'Hi', text_style: {}, x: 0.3, y: 0.4, width: 0.3125, height: 100 / 720}, extra, keyframes ? {keyframes} : {}))])]);
  p.assets.push(OLD, NEW);
  return p;
};
const session = (p, frame) => {
  const s = {disposed: false, project: p, frame, artworkRevision: 0, clipArtRevisions: new Map([['t', 1]]), staleArtwork: new Set(['t']),
    q: () => null, timelineOptions: {}, preview: {seek() {}}};
  return {s, result: {clipId: 't', clipRevision: 1, global: 0, originalAssetId: 'art1', text: 'Hi', styleKey: P.textStyleKey({}), asset: NEW, width: 600, height: 100}};
};
const commit = (p, frame) => { const {s, result} = session(p, frame); P.commitArtwork(s, result); return s; };
const shown = (p, frame) => FX.clipState(at(p, 't'), frame, p);

// A static title: the fields take the new box, centred on the old one, at the same scale; nothing is keyed.
let p = fixture();
valid(p);
let s = commit(p, 30);
assert.equal(at(p, 't').asset_id, 'art2');
close(at(p, 't').width, 600 / 1280, 'the natural width of the new PNG at scale 1');
close(at(p, 't').x + at(p, 't').width / 2, 0.3 + 0.3125 / 2, 'the centre stays');
close(at(p, 't').y + at(p, 't').height / 2, 0.4 + 100 / 720 / 2, 'the vertical centre stays');
assert.equal('keyframes' in at(p, 't'), false, 'a static title is not keyed');
assert(s.staleArtwork.size === 0 && saved === 1, 'the stale mark clears and the project saves');
valid(p);

// An animated width (0.3125 -> 0.625 over 60 frames) shows 0.46875 at frame 30: that is 1.5 times the natural width of
// the old PNG, so the new PNG is 1.5 times its natural size, centred where the picture was.
p = fixture({}, {width: [kf(0, 0.3125), kf(60, 0.625)]});
valid(p);
const before = shown(p, 30);
close(before.width, 0.46875, 'the box shown at frame 30');
commit(p, 30);
let t = at(p, 't');
assert.equal(t.asset_id, 'art2');
assert.equal(t.width, 0.3125, 'the static field of the animated width is untouched');
assert.deepEqual(t.keyframes.width.map(key => key.f), [0, 30, 60], 'a key at the playhead carries the new width');
close(t.keyframes.width[1].v, 600 / 1280 * 1.5, 'new natural width times the scale shown at the playhead');
close(t.keyframes.width[0].v, 0.3125, 'the other keys are untouched');
const after = shown(p, 30);
close(after.width, 600 / 1280 * 1.5, 'the box shown at the playhead');
close(after.height, 100 / 720 * 1.5, 'the static height follows the same scale');
close(after.x + after.width / 2, before.x + before.width / 2, 'the horizontal centre shown at the playhead stays');
close(after.y + after.height / 2, before.y + before.height / 2, 'the vertical centre shown at the playhead stays');
assert.equal(t.height, after.height, 'the static height takes its field');
valid(p);

// Position and size animated together: every animated value gets its key, centre and scale taken from the animated state.
p = fixture({}, {x: [kf(0, 0.1), kf(60, 0.3)], width: [kf(0, 0.3125), kf(60, 0.3125)], y: [kf(0, 0.2), kf(60, 0.4)], height: [kf(0, 100 / 720), kf(60, 100 / 720)]});
valid(p);
const was = shown(p, 45);
commit(p, 45);
t = at(p, 't');
close(t.x, 0.3, 'static fields stay for every animated value');
for (const path of ['x', 'y', 'width', 'height']) assert.deepEqual(t.keyframes[path].map(key => key.f), [0, 45, 60], path + ' is keyed at the playhead');
const now = shown(p, 45);
close(now.width, 600 / 1280, 'scale 1 at the playhead');
close(now.x + now.width / 2, was.x + was.width / 2, 'centre x');
close(now.y + now.height / 2, was.y + was.height / 2, 'centre y');
valid(p);

// A playhead outside the clip evaluates and keys at the nearest clip frame (here the last one, 89).
p = fixture({}, {width: [kf(0, 0.3125), kf(60, 0.625)]});
commit(p, 500);
t = at(p, 't');
assert.deepEqual(t.keyframes.width.map(key => key.f), [0, 60, 89]);
close(t.keyframes.width[2].v, 600 / 1280 * 2, 'scale 2 = 0.625 / 0.3125 held after the last key');
valid(p);

// A value at its 64-key limit cannot take another key: nothing changes and the title stays stale.
p = fixture({}, {width: Array.from({length: 64}, (_, i) => kf(2 * i + 100, 0.3125))});
const untouched = JSON.stringify(p);
s = commit(p, 31);
assert.equal(JSON.stringify(p), untouched, 'the clip keeps its asset and keys');
assert(s.staleArtwork.has('t'), 'and stays marked stale');
`)
}
