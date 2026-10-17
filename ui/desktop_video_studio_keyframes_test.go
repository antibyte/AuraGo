package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// videoStudioKeyframesPrelude adds keyframe helpers to videoStudioEditPrelude (30 fps; V and A are 600-frame video and
// audio sources, I a still; clip, track, project, at and valid build and check projects).
const videoStudioKeyframesPrelude = `
const FX = window.VideoStudioEffects;
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
