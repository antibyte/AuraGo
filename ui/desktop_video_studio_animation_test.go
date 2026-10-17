package ui

import "testing"

// validTimeline refuses what Go's Validate refuses for keyframes and speed ramps (plan index §4.18), through
// VideoStudioEffects.validAnimation. The cases mirror TestValidateKeyframes and TestValidateRampShape
// (internal/videostudio/validate_animation_test.go).
func TestVideoStudioValidTimelineAnimation(t *testing.T) {
	runVideoStudioEffectsScript(t, `
require('./js/desktop/apps/video-studio-timeline.js');
const T = window.VideoStudioTimeline;
assert.deepEqual(E.EASINGS, ['linear', 'ease_in', 'ease_out', 'ease_in_out', 'hold']);
assert.deepEqual(E.LIMITS, {keyframesPerParam: 64, keyframesPerClip: 512, rampPoints: 16, rampPieces: 64, rampEaseSteps: 4, effectsPerClip: 8});
const box = {x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1, fade_in: 0, fade_out: 0, fit: 'contain', text: '', text_style: null, transition: null};
const k = (f, v, e = 'linear') => ({f, v, e});
const pt = (f, speed, e = 'linear') => ({f, speed, e});
const run = (n, v) => Array.from({length: n}, (_, i) => k(i, v));
// Mirrors animationProject: "v" a 300-frame video on the video track (offset 10), "o" an image on the overlay track,
// "a" a song on the audio track; 60 frames each at 30 fps (maxFrames 108000).
function project(edit) {
  const clip = (id, asset, extra) => Object.assign({id, asset_id: asset, start: 0, offset: 0, duration: 60}, box, extra || {});
  const clips = {v: clip('v', 'video', {offset: 10}), o: clip('o', 'logo', {x: 0.1, y: 0.1, width: 0.2, height: 0.2}), a: clip('a', 'song')};
  edit(clips);
  return {version: 3, name: 'Animation', width: 1280, height: 720, fps: 30,
    assets: [{id: 'video', kind: 'video', duration_frames: 300, width: 640, height: 360}, {id: 'logo', kind: 'image', duration_frames: 0, width: 64, height: 64}, {id: 'song', kind: 'audio', duration_frames: 600, has_audio: true}],
    tracks: [{id: 'video', kind: 'video', name: 'Video', clips: [clips.v]}, {id: 'overlay', kind: 'overlay', name: 'Overlay', clips: [clips.o]}, {id: 'audio', kind: 'audio', name: 'Audio', clips: [clips.a]}]};
}
const ok = (label, edit) => assert.equal(T.validTimeline(project(edit)), true, label);
const bad = (label, edit) => assert.equal(T.validTimeline(project(edit)), false, label);
ok('no animation', () => {});
// Keyframes.
ok('keyframes on the video clip', c => { c.v.keyframes = {x: [k(0, 0), k(30, 0.5, 'ease_in_out')], opacity: [k(10, 1, 'hold')]}; });
ok('every easing on the overlay clip, frames before the source start', c => { c.o.keyframes = {rotation: [k(-20, -360, 'ease_in'), k(0, 0, 'ease_out'), k(5, 45.5, 'ease_in_out'), k(9, 360, 'hold'), k(12, 0)]}; });
ok('no cross-parameter check (x + width may pass 1)', c => { c.v.keyframes = {x: [k(0, 0.9)], width: [k(0, 1)]}; });
ok('64 keys on one path', c => { c.v.keyframes = {opacity: run(64, 1)}; });
ok('frames at the bound', c => { c.v.keyframes = {x: [k(-1728000, 0), k(1728000, 1)]}; });
ok('an empty map', c => { c.v.keyframes = {}; });
bad('keyframes on an audio clip', c => { c.a.keyframes = {opacity: [k(0, 1)]}; });
bad('an unknown path', c => { c.v.keyframes = {volume: [k(0, 1)]}; });
bad('a choice parameter', c => { c.v.keyframes = {fit: [k(0, 1)]}; });
bad('an empty list', c => { c.v.keyframes = {x: []}; });
bad('65 keys on one path', c => { c.v.keyframes = {opacity: run(65, 1)}; });
bad('more than 512 keys per clip', c => { c.v.keyframes = Object.fromEntries(Array.from({length: 9}, (_, i) => ['p' + i, run(64, 1)])); });
bad('unsorted frames', c => { c.v.keyframes = {x: [k(5, 0), k(3, 1)]}; });
bad('a repeated frame', c => { c.v.keyframes = {x: [k(5, 0), k(5, 1)]}; });
bad('a fractional frame', c => { c.v.keyframes = {x: [k(1.5, 0)]}; });
bad('a frame beyond the bound', c => { c.v.keyframes = {x: [k(1728001, 0)]}; });
bad('a value outside the range', c => { c.v.keyframes = {x: [k(0, 1.5)]}; });
bad('width 0', c => { c.v.keyframes = {width: [k(0, 0)]}; });
// Boxes are at least 1 % wide and tall (Go MinBoxSize): a sliver would make a cover bound of many canvas widths.
ok('width at the 1 % minimum', c => { c.v.keyframes = {width: [k(0, 0.01), k(10, 1)]}; });
ok('height at the 1 % minimum', c => { c.v.keyframes = {height: [k(0, 1), k(10, 0.01)]}; });
bad('width below 1 %', c => { c.v.keyframes = {width: [k(0, 0.009)]}; });
bad('height below 1 %', c => { c.v.keyframes = {height: [k(0, 1), k(10, 0.0001)]}; });
ok('a small opacity is no box', c => { c.v.keyframes = {opacity: [k(0, 0.001)]}; });
bad('rotation beyond 360', c => { c.o.keyframes = {rotation: [k(0, 400)]}; });
bad('a number given as text', c => { c.v.keyframes = {opacity: [k(0, '1')]}; });
bad('an unknown easing', c => { c.v.keyframes = {x: [k(0, 0, 'bounce')]}; });
bad('a missing easing', c => { c.v.keyframes = {x: [k(0, 0, '')]}; });
// Ramps: shape only here; the source extent rule follows the ramp from plan 2b Task 3 on.
const two = [pt(0, 1, 'ease_in_out'), pt(30, 2)];
ok('a two-point ramp', c => { c.v.ramp = two; });
ok('sixteen points with every easing, outside the clip too', c => { c.v.ramp = Array.from({length: 16}, (_, i) => pt(-20 + 10 * i, [0.25, 0.5, 1, 2][i % 4], E.EASINGS[i % 5])); });
ok('speed 1 beside a ramp', c => { c.v.speed = 1; c.v.ramp = two; });
ok('bounds', c => { c.v.ramp = [pt(-108000, 0.25), pt(0, 0.25, 'hold'), pt(108000, 16)]; });
bad('a ramp on the overlay image', c => { c.o.ramp = two; });
bad('a ramp on a still on the video track', c => { c.v.asset_id = 'logo'; c.v.offset = 0; c.v.ramp = two; });
bad('a ramp on the audio clip', c => { c.a.ramp = two; });
bad('one point', c => { c.v.ramp = [pt(0, 2)]; });
bad('seventeen points', c => { c.v.ramp = Array.from({length: 17}, (_, i) => pt(i, 1)); });
bad('a constant speed beside a ramp', c => { c.v.speed = 2; c.v.ramp = two; });
bad('unsorted points', c => { c.v.ramp = [pt(10, 1), pt(5, 2)]; });
bad('a point beyond the timeline limit', c => { c.v.ramp = [pt(0, 1), pt(108001, 2)]; });
bad('a speed below 0.25', c => { c.v.ramp = [pt(0, 0.2), pt(30, 2)]; });
bad('a speed above 16', c => { c.v.ramp = [pt(0, 1), pt(30, 16.5)]; });
bad('an unknown ramp easing', c => { c.v.ramp = [pt(0, 1, ''), pt(30, 2)]; });
bad('a ramp that is not a list', c => { c.v.ramp = {f: 0}; });
// The source extent rule follows the ramp: offset + S(duration - 1) <= duration_frames - 1. With 1 -> 3 over 20 frames,
// S(59) = 40 + 39 * 3 = 157.
const up = [pt(0, 1), pt(20, 3)];
ok('a ramp that ends on the last source frame', c => { c.v.offset = 142; c.v.ramp = up; });
bad('a ramp one frame past the media', c => { c.v.offset = 143; c.v.ramp = up; });
ok('a slow ramp holds a clip longer than the media at 1x', c => { c.v.offset = 0; c.v.duration = 400; c.v.ramp = [pt(0, 0.5), pt(10, 0.5)]; });
bad('the same clip without the ramp', c => { c.v.offset = 0; c.v.duration = 400; });
console.log('ok');
`)
}
