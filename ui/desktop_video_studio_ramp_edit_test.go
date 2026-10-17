package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVideoStudioRampEdits checks the ramp-aware timing edits of plan 2b Task 9 (index §4.18):
// - splits and head edits bake the passed source into a floored offset and shift the ramp points;
// - slip and reverse refuse with reason ramp, and a constant speed replaces the ramp;
// - detached sound plays at 1x without ramp or keyframes;
// - still requests send the mean speed, and a rate change scales the points.
// Every result is a valid timeline.
func TestVideoStudioRampEdits(t *testing.T) {
	editing, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "video-studio-editing.js"))
	if err != nil {
		t.Fatalf("editing part: %v", err)
	}
	if !strings.Contains(string(editing), "ramp: ['editRamp', ") {
		t.Error("REASON_NOTICES has no ramp entry with the editRamp key")
	}
	files, err := filepath.Glob(filepath.Join("lang", "desktop", "*.json"))
	if err != nil || len(files) != 16 {
		t.Fatalf("desktop locales: %d files, %v", len(files), err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"videoStudio.editRamp": "`) {
			t.Errorf("%s lacks videoStudio.editRamp", filepath.Base(file))
		}
	}
	runVideoStudioEditScript(t, `
const FX = window.VideoStudioEffects, Time = window.VideoStudioTime;
const near = (got, want, message) => assert(Math.abs(got - want) < 1e-6, message + ': got ' + got + ', want ' + want);
// Ramp 1x -> 3x over clip frames 0..100, one linear piece: speed(t) = 1 + t/50, S(t) = t + t*t/100; 3x after frame
// 100, 1x before frame 0. S(10) = 11, S(20) = 24, S(25) = 31.25, S(30) = 39, S(50) = 75, S(99) = 197.01, S(100) = 200.
const R = () => [{f: 0, speed: 1, e: 'linear'}, {f: 100, speed: 3, e: 'linear'}];
const shiftedR = frames => R().map(point => Object.assign(point, {f: point.f - frames}));
const ramped = () => project([track('v1','video',[clip('r',V,0,100,{ramp: R()}), clip('n',V,100,100,{offset: 300})]), track('a1','audio',[])]);
assert.equal(E.REASON.ramp, 'ramp', 'a new refusal reason');
valid(ramped());

// headShift: the source the head passes goes into the floored offset; ramp points (clip time) move with the head.
let r = at(ramped(), 'r');
assert.deepEqual(FX.headShift(r, 30), {offset: 39, ramp: shiftedR(30)}, 'S(30) = 39 joins the offset');
assert.equal(FX.headShift(r, 25).offset, 31, 'S(25) = 31.25 rounds down');
assert.deepEqual(FX.headShift(Object.assign({}, r, FX.headShift(r, 30)), -30), {offset: 0, ramp: R()}, 'growing the head back restores offset and points');
assert.deepEqual(FX.headShift({offset: 10, speed: 1.5}, 7), {offset: 20, ramp: null}, 'without a ramp: phase 1 floor(offset + frames x speed)');
assert.deepEqual(FX.headShift({offset: 3}, -10), {offset: 0, ramp: null}, 'never below 0');
// meanSpeed: the constant speed that consumes the same source; still requests send it.
near(FX.meanSpeed(r, 50), 1.5, 'S(50) / 50');
assert.equal(FX.meanSpeed(r, 0), 1, 'the first frame');
assert.equal(FX.meanSpeed({offset: 0, speed: 2}, 10), 2, 'a constant speed is its own mean');
const flat16 = {offset: 0, ramp: [{f: 0, speed: 16, e: 'linear'}, {f: 10, speed: 16, e: 'linear'}]};
assert(FX.meanSpeed(flat16, 7) <= 16 && FX.meanSpeed(flat16, 7) > 15.999999, 'kept inside the speed range');

// Splits: the left part keeps the curve; the right part bakes S(cut) into its offset and shifts the points.
let p = ramped();
const whole = JSON.parse(JSON.stringify(at(p, 'r'))), out = {};
assert.equal(E.blade(p, 30, {clipIds: ['r']}, out), true);
const left = at(p, 'r'), right = at(p, out.created[0]);
assert.deepEqual([left.start, left.duration, left.offset, left.ramp], [0, 30, 0, R()], 'the left part keeps the curve');
assert.deepEqual([right.start, right.duration, right.offset, right.ramp], [30, 70, 39, shiftedR(30)], 'the right part bakes S(30) into its offset');
for (let frame = 0; frame < 100; frame++) near(FX.sourceAt(frame < 30 ? left : right, frame), FX.sourceAt(whole, frame), 'the cut keeps the content at frame ' + frame);
valid(p);
assert.equal(E.overwrite(ramped(), [{trackId: 'v1', clips: [clip('q', V, 300, 300, {ramp: R()})]}]), 'media', 'a placed ramped clip must fit its media (234 frames here)');

// Ripple trims along the curve.
p = ramped();
assert.equal(E.rippleTrim(p, ['r'], 'start', 30), true);
assert.deepEqual([at(p,'r').start, at(p,'r').duration, at(p,'r').offset, at(p,'r').ramp], [0, 70, 39, shiftedR(30)], 'a start trim shows later frames along the curve');
assert.equal(at(p,'n').start, 70, 'later clips ripple');
valid(p);
assert.equal(E.rippleTrim(p, ['r'], 'start', -500), true, 'the head grows back along the curve');
assert.deepEqual([at(p,'r').duration, at(p,'r').offset, at(p,'r').ramp], [100, 0, R()]);
assert.equal(E.rippleTrim(p, ['r'], 'start', -1), 'bounds', 'no source before frame 0');
assert.equal(E.rippleTrim(p, ['r'], 'end', 500), true);
assert.equal(at(p,'r').duration, 234, 'the tail stops where offset + S(duration - 1) reaches source frame 599 (200 + 3 x 133)');
valid(p);

// Roll and slide move the ramped head the same way.
p = project([track('v1','video',[clip('a',V,0,100), clip('r',V,100,100,{ramp: R()})])]);
assert.equal(E.roll(p, [['a','r']], 20), true);
assert.deepEqual([at(p,'a').duration, at(p,'r').start, at(p,'r').duration, at(p,'r').offset, at(p,'r').ramp], [120, 120, 80, 24, shiftedR(20)], 'a roll moves the ramped head by S(20) = 24');
valid(p);
assert.equal(E.roll(p, [['a','r']], -20), true);
assert.deepEqual([at(p,'r').start, at(p,'r').offset, at(p,'r').ramp], [100, 0, R()], 'rolling back restores the curve');
assert.equal(E.roll(p, [['a','r']], -5), 'bounds', 'the ramped clip has no source before its first frame');
p = project([track('v1','video',[clip('a',V,0,100), clip('b',V,100,50,{offset: 200}), clip('r',V,150,100,{ramp: R()})])]);
assert.equal(E.slide(p, 'b', 10), true);
assert.deepEqual([at(p,'a').duration, at(p,'b').start, at(p,'r').start, at(p,'r').duration, at(p,'r').offset, at(p,'r').ramp], [110, 110, 160, 90, 11, shiftedR(10)], 'a slide moves the ramped neighbour head by S(10) = 11');
valid(p);

// Slip and reverse refuse ramped clips.
p = ramped();
assert.equal(E.slip(p, ['r'], 5), 'ramp', 'slip refuses a ramped clip');
assert.equal(E.slip(p, ['r', 'n'], 5), 'ramp', 'and a selection that holds one');
assert.equal(E.slip(p, ['n'], 5), true, 'constant-speed clips still slip');
assert.equal(at(p, 'n').offset, 305);
p = ramped();
assert.deepEqual(E.reverseRange(p, 'r'), {rangeStart: 0, frames: 199}, 'the reverse range covers the ramped extent: ceil(S(99) = 197.01) + 1');
p.assets.push({id: 'rev', name: 'rev', kind: 'video', has_audio: true, duration_frames: 199});
assert.equal(E.reverseClips(p, 'r', {assetId: 'rev', rangeStart: 0, frames: 199, sourceAssetId: 'v'}), 'ramp', 'reverse refuses a ramped clip');

// A constant speed replaces the ramp; by default it keeps the source range S(99) = 197.01 (index §4.18): round(197.01 / speed) + 1 frames.
p = ramped();
assert.equal(E.setSpeed(p, ['r'], 4, false), true);
assert.deepEqual([at(p,'r').duration, at(p,'r').speed, 'ramp' in at(p,'r')], [50, 4, false], 'round(49.25) + 1 frames at 4x');
assert.equal(at(p,'n').start, 50, 'later clips ripple');
valid(p);
p = ramped();
assert.equal(E.setSpeed(p, ['r'], 4, true), true);
assert.deepEqual([at(p,'r').duration, at(p,'r').speed, 'ramp' in at(p,'r')], [100, 4, false], 'keep duration');
valid(p);
p = ramped();
assert.equal(E.setSpeed(p, ['r'], 1, false), true);
assert.deepEqual([at(p,'r').duration, 'speed' in at(p,'r'), 'ramp' in at(p,'r')], [198, false, false], '1x keeps the source range: round(197.01) + 1 frames');
valid(p);

// Detached sound plays at 1x without ramp or keyframes; a range that does not fit at 1x refuses.
p = ramped();
at(p, 'r').keyframes = {opacity: [{f: 0, v: 0, e: 'linear'}, {f: 10, v: 1, e: 'linear'}]};
valid(p);
assert.equal(E.detachAudio(p, 'r', {clip: 'ra', link: 'L', track: 'a2', trackName: 'Audio 2'}), true);
const sound = at(p, 'ra');
assert.deepEqual([sound.start, sound.duration, sound.offset, 'speed' in sound, 'ramp' in sound, 'keyframes' in sound], [0, 100, 0, false, false, false], 'the sound plays at 1x without ramp or keyframes');
assert(at(p, 'r').audio_detached && at(p, 'r').ramp && at(p, 'r').keyframes, 'the picture keeps its ramp and keyframes');
valid(p);
p = project([track('v1','video',[clip('s',V,0,60,{offset: 560, ramp: [{f: 0, speed: 0.5, e: 'linear'}, {f: 10, speed: 0.5, e: 'linear'}]})]), track('a1','audio',[])]);
valid(p);
assert.equal(E.detachAudio(p, 's', {clip: 'sa', link: 'L', track: 'a2', trackName: 'Audio 2'}), 'media', 'at 1x the sound would pass the media end (560 + 60 > 600)');

// Still frames: the source frame shown is floor(offset + S(t)), computed exactly as the server checks a request.
p = ramped();
r = at(p, 'r');
assert.equal(E.sourceFrameAt(p, r, 50), 75, 'floor(offset + S(50))');
assert.equal(E.sourceFrameAt(p, r, 99), 197, 'floor(S(99) = 197.01)');
for (const frame of [1, 7, 33, 50, 99]) {
  const t = frame - r.start, speed = FX.meanSpeed(r, t);
  assert(speed >= 0.25 && speed <= 16, 'a speed the server accepts at ' + frame);
  assert.equal(Math.floor(r.offset + t * speed + 1e-9), E.sourceFrameAt(p, r, frame), 'the server rule floor(offset + clip_frame x speed) names the same frame at ' + frame);
  assert.equal(E.sourceFrameAt(p, r, frame), Math.floor(FX.sourceAt(r, frame) + 1e-6), 'the frame the export shows, floor(offset + S(t)), at ' + frame);
}
const requests = [];
window.VideoStudioParts = {T: () => T, showNotice() {}, showReason() {}, applyEdit() {}, rateOf: () => 30, tr: (s, key, fallback) => fallback,
  clone: value => JSON.parse(JSON.stringify(value)), createJob: async (s, payload) => { requests.push(payload); return null; }};
require('./js/desktop/apps/video-studio-derive.js');
const P = window.VideoStudioParts;
assert.equal(P.exportFrame({project: ramped(), frame: 50, readonly: false}, 'r'), true);
const still = requests.pop();
near(still.speed, 1.5, 'a ramped still request sends the mean speed S(50) / 50');
assert.deepEqual(Object.assign({}, still, {speed: 1.5}), {kind: 'still', asset_id: 'v', frame: 75, offset: 0, clip_frame: 50, speed: 1.5, purpose: 'export'});
assert.equal(Math.floor(still.offset + still.clip_frame * still.speed + 1e-9), still.frame, 'the unchanged server check accepts it');
assert.equal(P.exportFrame({project: base(), frame: 150, readonly: false}, 'y'), true);
assert.deepEqual(requests.pop(), {kind: 'still', asset_id: 'v', frame: 250, offset: 200, clip_frame: 50, speed: 1, purpose: 'export'}, 'constant-speed requests are unchanged');

// Timeline edge drags follow the same rules.
p = ramped();
let dragged = T.dragClip(p, 'r', 'start', 30, null, 0, [], []);
assert(dragged, 'a valid head drag');
assert.deepEqual([at(dragged,'r').start, at(dragged,'r').duration, at(dragged,'r').offset, at(dragged,'r').ramp], [30, 70, 39, shiftedR(30)], 'a head drag bakes S(30) into the offset and shifts the points');
assert.deepEqual(at(T.dragClip(dragged, 'r', 'start', -100, null, 0, [], []), 'r').ramp, R(), 'dragging the head back restores the curve');
dragged = T.dragClip(p, 'r', 'start', -5, null, 0, [], []);
assert.deepEqual([at(dragged,'r').start, at(dragged,'r').offset], [0, 0], 'no source before the first frame');
dragged = T.dragClip(project([track('v1','video',[clip('r',V,0,100,{ramp: R()})])]), 'r', 'end', 500, null, 0, [], []);
assert.equal(at(dragged,'r').duration, 234, 'the tail drag stops at the media end along the curve');

// A rate change scales ramp points like durations. A point that rounds onto an earlier one is dropped, and a single
// point left becomes a constant speed. The projects are cloned because rescaleProject rewrites asset lengths and the
// prelude's assets are shared.
p = JSON.parse(JSON.stringify(ramped()));
Time.rescaleProject(p, 60, false);
assert.deepEqual([at(p,'r').start, at(p,'r').duration, at(p,'r').offset, at(p,'r').ramp], [0, 200, 0, [{f: 0, speed: 1, e: 'linear'}, {f: 200, speed: 3, e: 'linear'}]], '30 -> 60 fps doubles the points; speeds stay');
valid(p);
const at60 = points => JSON.parse(JSON.stringify(Object.assign(project([track('v1','video',[clip('r',V,0,100,{ramp: points})])]), {fps: 60})));
p = at60([{f: 0, speed: 1, e: 'linear'}, {f: 1, speed: 2, e: 'ease_in'}, {f: 100, speed: 3, e: 'linear'}]);
valid(p);
Time.rescaleProject(p, 24, false);
assert.deepEqual(at(p,'r').ramp, [{f: 0, speed: 1, e: 'linear'}, {f: 40, speed: 3, e: 'linear'}], '60 -> 24 fps: frame 1 rounds onto frame 0 and is dropped');
valid(p);
p = at60([{f: 0, speed: 2, e: 'linear'}, {f: 1, speed: 3, e: 'linear'}]);
Time.rescaleProject(p, 24, false);
assert.deepEqual(['ramp' in at(p,'r'), at(p,'r').speed], [false, 2], 'one point left becomes its constant speed');
valid(p);
`)
}

// TestVideoStudioReverseIsOfferedOnlyWhereItRuns checks that the Reverse button and the clip-menu item (both read
// P.clipCapabilities().reversible) are off for a ramped clip and for a clip whose same-asset link partner is ramped.
// reverseClips refuses those with REASON.ramp, so offering them would be a button that always fails.
func TestVideoStudioReverseIsOfferedOnlyWhereItRuns(t *testing.T) {
	runVideoStudioKeyframesScript(t, videoStudioKeyframePartsPrelude+`
require('./js/desktop/apps/video-studio-derive.js');
const ramp = [{f: 0, speed: 1, e: 'linear'}, {f: 50, speed: 2, e: 'linear'}];
const caps = (p, id, frame) => P.clipCapabilities({project: p, selectedClipId: id, frame: frame == null ? 10 : frame, readonly: false});
let p = project([
  track('v1','video',[clip('plain',V,0,100), clip('r',V,100,100,{ramp})]),
  track('v2','video',[clip('lp',V,0,100,{link:'L'})]),
  track('v3','video',[clip('lr',V,0,100,{link:'L', ramp})]),
  track('a1','audio',[])
]);
valid(p);
assert.equal(caps(p, 'plain').reversible, true, 'a plain clip can be reversed');
assert.equal(caps(p, 'r', 110).reversible, false, 'a ramped clip cannot: reverseClips refuses it with reason ramp');
assert.equal(caps(p, 'lp').reversible, false, 'nor can a clip whose same-asset link partner is ramped');
assert.equal(caps(p, 'lr').reversible, false);
assert.equal(P.reverseFits({project: p, frame: 0}, 'r'), false, 'the shared gate says no for a ramped clip');
assert.equal(P.reverseFits({project: p, frame: 0}, 'plain'), true);
// The edit layer keeps its own refusal (the model rule does not depend on the UI gate).
p.assets.push({id: 'rev', name: 'rev', kind: 'video', has_audio: true, duration_frames: 600});
assert.equal(E.reverseClips(p, 'r', {assetId: 'rev', rangeStart: 0, frames: 600, sourceAssetId: 'v'}), 'ramp');
// Once the ramp is gone the clip is offered again.
delete at(p, 'r').ramp;
assert.equal(caps(p, 'r', 110).reversible, true, 'without a ramp reverse is offered again');
`)
}
