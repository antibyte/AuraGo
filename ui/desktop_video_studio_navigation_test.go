package ui

import (
	"os/exec"
	"testing"
)

const videoStudioNavPrelude = `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-timeline.js');
const T = window.VideoStudioTimeline, Time = window.VideoStudioTime;
const box = {x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1, fit: 'contain'};
`

func runVideoStudioNavScript(t *testing.T, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio navigation checks")
	}
	if output, err := exec.Command(node, "-e", videoStudioNavPrelude+script).CombinedOutput(); err != nil {
		t.Fatalf("navigation checks: %v\n%s", err, output)
	}
}

func TestVideoStudioRescaleKeepsMarkersAndInOut(t *testing.T) {
	runVideoStudioNavScript(t, `
assert.equal(Time.frameAtRate(1000, {fps: 30}, {fps: 25}), 833, 'the same time at the new rate, rounded');
assert.equal(Time.frameAtRate(833, {fps: 25}, {fps: 30}), 1000, 'undo maps the playhead back');
assert.equal(Time.frameAtRate(100, {fps: 24}, {fps: 24, ntsc: true}), 100);
assert.equal(Time.clockSeconds(3725), '1:02:05', 'clock text carries hours');
assert.equal(Time.clockSeconds(75), '1:15');
// Markers and in/out follow the clips when the rate changes.
const project = {fps: 30, assets: [{id: 'v', kind: 'video', duration_frames: 300}], tracks: [{id: 't', kind: 'video', clips: [
  {...box, id: 'c', asset_id: 'v', start: 0, offset: 0, duration: 300, transition: null}]}],
  markers: [{id: 'm', frame: 31}, {id: 'r', frame: 10, duration: 15, name: 'range'}], in_point: 1, out_point: 31};
Time.rescaleProject(project, 25, false);
assert.deepEqual(project.markers.map(m => [m.id, m.frame, m.duration || 0]), [['m', 26, 0], ['r', 8, 13]], 'the array keeps its order; the app sorts on save');
assert.deepEqual([project.in_point, project.out_point], [1, 26]);
// A range keeps at least one frame; an out point that rounds onto the in point is dropped.
const tiny = {fps: 60, assets: [], tracks: [], markers: [{id: 'z', frame: 2, duration: 1}], in_point: 2, out_point: 3};
Time.rescaleProject(tiny, 24, false);
assert.equal(tiny.markers[0].duration, 1);
assert.equal(tiny.in_point, 1);
assert.equal('out_point' in tiny, false, 'a collapsed out point is dropped');
// A 60-minute project cannot change to 23.976 fps: its last frame would be past the limit. Refuse, do not clamp.
const long = {fps: 24, assets: [{id: 'img', kind: 'image'}], tracks: [{id: 't', kind: 'video', clips: [
  {...box, id: 'c', asset_id: 'img', start: 0, offset: 0, duration: 86400, transition: null}]}]};
assert.equal(Time.rescaleFits(long, 24, true), false, 'one frame past the 60-minute limit');
assert.equal(Time.rescaleFits(long, 24, false), true, 'the same rate always fits');
assert.equal(Time.rescaleFits(long, 25, false), true, '3600 s is exactly 90000 frames at 25 fps and fits');
`)
}
