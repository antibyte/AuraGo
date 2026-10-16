package ui

import (
	"os/exec"
	"testing"
)

func TestVideoStudioTimeModule(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio time checks")
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-effects.js');
require('./js/desktop/apps/video-studio-timeline.js');
const V = window.VideoStudioTime;
const p30 = {fps: 30}, ntsc = {fps: 30, ntsc: true}, p25 = {fps: 25};
assert.equal(V.rate(p30), 30);
assert(Math.abs(V.rate(ntsc) - 30000 / 1001) < 1e-9);
assert.equal(V.rate(undefined), 30, 'a missing project means the v1 default');
assert.equal(V.formatTime(132, p30), '0:04.40');
assert.equal(V.formatTime(108000, p30), '1:00:00.00');
assert.equal(V.formatTime(25, p25), '0:01.00');
assert.equal(V.formatTime(29, p30), '0:00.97');
assert.equal(V.formatTime(959, {fps: 60, ntsc: true}), '0:16.00', 'centiseconds carry into the seconds instead of clamping at 99');
V.FRAME_RATES.forEach(rate => {
  const project = {fps: rate.fps, ntsc: rate.ntsc};
  for (let f = 0; f <= 3000; f++) assert.equal(V.parseTime(V.formatTime(f, project), project), f, 'frame ' + f + ' at ' + V.rateLabel(project) + ' reads back from its display');
});
assert.equal(V.parseTime('1:02,5', p30), 1875);
assert.equal(V.parseTime('2', p25), 50);
assert.equal(V.parseTime('1:75', p30), null);
assert.equal(V.parseTime('abc', p30), null);
assert.equal(V.shortClock(3600 * 25, p25), '1:00:00');
assert.equal(V.shortClock(75 * 30, p30), '1:15');
assert.equal(V.formatSeconds(45, p30), '1.5');
assert.equal(V.framesAt(1033333, p30), 31);
assert.equal(V.framesAt(1001000, ntsc), 30);
assert.equal(V.framesAt(0, p30), 0);
assert.equal(V.maxFrames(p30), 108000);
assert.equal(V.maxFrames(ntsc), 107892);
assert.equal(V.maxFrames({fps: 60}), 216000);
assert.equal(V.rateLabel(ntsc), '29.97 fps');
assert.equal(V.rateLabel({fps: 24, ntsc: true}), '23.976 fps');
assert.equal(V.rateLabel(p25), '25 fps');
assert.equal(V.FRAME_RATES.length, 8);

// 30 → 25 fps: edges scale, neighbours stay gapless, transitions follow the overlap, clips stay in their media.
const box = {x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1, fit: 'contain'};
const project = {fps: 30, assets: [{id: 'v', kind: 'video', duration_frames: 300, duration_us: 10000000}], tracks: [{id: 't', kind: 'video', clips: [
  {...box, id: 'a', asset_id: 'v', start: 0, offset: 0, duration: 31, fade_in: 0, fade_out: 0, transition: {type: 'dissolve', duration: 7}},
  {...box, id: 'b', asset_id: 'v', start: 24, offset: 0, duration: 31, fade_in: 6, fade_out: 0, transition: null},
  {...box, id: 'c', asset_id: 'v', start: 55, offset: 270, duration: 30, fade_in: 0, fade_out: 0, transition: null}]}]};
assert(window.VideoStudioTimeline.validTimeline(project), 'the 30 fps fixture is valid');
V.rescaleProject(project, 25, false);
const [a, b, c] = project.tracks[0].clips;
assert.equal(project.fps, 25);
assert.equal(project.assets[0].duration_frames, 250);
assert.deepEqual([a.start, a.duration, b.start, b.start + b.duration, c.start], [0, 26, 20, 46, 46]);
assert.equal(a.transition.duration, a.start + a.duration - b.start, 'the transition equals the rounded overlap');
assert.equal(b.fade_in, 5);
assert(c.offset + c.duration <= 250, 'clips stay inside their media');
assert(window.VideoStudioTimeline.validTimeline(project), 'a rescaled project stays valid');
// A speed clip that reaches the end of its media stays inside it at the new rate (source extent rule at speed 2).
const sped = {fps: 24, assets: [{id: 'v', kind: 'video', duration_frames: 119, duration_us: Math.round(119 / 24 * 1e6)}], tracks: [{id: 't', kind: 'video', clips: [
  {...box, id: 's', asset_id: 'v', start: 0, offset: 0, duration: 60, speed: 2, fade_in: 0, fade_out: 0, transition: null}]}]};
assert(window.VideoStudioTimeline.validTimeline(sped), 'the 24 fps speed fixture is valid');
V.rescaleProject(sped, 25, false);
assert.equal(sped.assets[0].duration_frames, 124);
assert.equal(sped.tracks[0].clips[0].duration, 62, 'the duration is clamped by offset + (duration - 1) * speed');
assert(window.VideoStudioTimeline.validTimeline(sped), 'a rescaled speed clip stays valid');
V.rescaleProject(project, 30, true);
assert.equal(project.ntsc, true);
V.rescaleProject(project, 25, true);
assert.equal('ntsc' in project, false, 'NTSC timing exists only at 24, 30 and 60 fps');
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("time module checks: %v\n%s", err, output)
	}
}
