package ui

import (
	"os/exec"
	"testing"
)

func TestVideoStudioHelpers(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio helper checks")
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-icons.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-media.js');
require('./js/desktop/apps/video-studio-inspector.js');
const I = window.VideoStudioInspector, M = window.VideoStudioMedia, Icons = window.VideoStudioIcons;

assert(Icons.svg('play', 16).startsWith('<svg'), 'icons render inline SVG');
assert(Icons.svg('does-not-exist').includes('<svg'), 'unknown icons fall back');

// Times are shown as m:ss.cc and accept seconds with comma or dot.
assert.equal(I.formatTime(0), '0:00.00');
assert.equal(I.formatTime(132), '0:04.40');
assert.equal(I.formatTime(30 * 75 + 15), '1:15.50');
assert.equal(I.formatTime(30 * 3600), '1:00:00.00');
assert.equal(I.parseTime('62,5'), 1875);
assert.equal(I.parseTime('62.5'), 1875);
assert.equal(I.parseTime('1:02.5'), 1875);
assert.equal(I.parseTime('0:04.40'), 132);
assert.equal(I.parseTime(' 45 '), 1350);
assert.equal(I.parseTime('1:00:00'), 108000);
assert.equal(I.parseTime(''), null);
assert.equal(I.parseTime('abc'), null);
assert.equal(I.parseTime('-3'), null);
assert.equal(I.parseTime('1:75'), null, 'seconds above 59 inside m:ss are rejected');
assert.equal(I.formatSeconds(45), '1.5');
assert.equal(I.formatSeconds(30), '1');

// Position presets keep the box inside the canvas.
const box = {x: 0.1, y: 0.1, width: 0.3, height: 0.2};
assert.deepEqual(I.presetBox('tl', box), {x: 0.04, y: 0.04});
assert.deepEqual(I.presetBox('mc', box), {x: 0.35, y: 0.4});
const br = I.presetBox('br', box);
assert(Math.abs(br.x - 0.66) < 1e-9 && Math.abs(br.y - 0.76) < 1e-9);
assert.deepEqual(I.presetBox('br', {x: 0, y: 0, width: 1, height: 1}), {x: 0, y: 0});
const scaled = I.scaleBox({x: 0.4, y: 0.4, width: 0.2, height: 0.2}, 0.4);
assert(Math.abs(scaled.x - 0.3) < 1e-9 && Math.abs(scaled.width - 0.4) < 1e-9 && Math.abs(scaled.height - 0.4) < 1e-9, 'scale keeps the center and aspect');
const edge = I.scaleBox({x: 0.8, y: 0.8, width: 0.2, height: 0.2}, 0.5);
assert(edge.x + edge.width <= 1 + 1e-9 && edge.y + edge.height <= 1 + 1e-9, 'scaled boxes stay inside the canvas');
const clamped = I.clampBox({x: 0.9, y: -0.2, width: 0.5, height: 1.4});
assert.deepEqual(clamped, {x: 0.5, y: 0, width: 0.5, height: 1});
for (let px = 1; px <= 1280; px++) {
  for (const canvas of [640, 720, 1280, 1920]) {
    const b = I.clampBox({x: 2, y: 2, width: px / canvas, height: px / canvas});
    assert(b.x + b.width <= 1 && b.y + b.height <= 1, 'rounded box crosses the frame edge: ' + JSON.stringify(b));
  }
}

// Overlay slots: a free unlocked overlay track, a new track, or nothing.
const clip = (start, duration) => ({id: 'c' + start, asset_id: 'a', start, duration});
const project = {assets: [], tracks: [
  {id: 'v1', kind: 'video', clips: [clip(0, 300)]},
  {id: 'o1', kind: 'overlay', clips: [clip(0, 150)]},
  {id: 'a1', kind: 'audio', clips: []}
]};
assert.deepEqual(I.findOverlaySlot(project, 60, 150), {create: true});
assert.deepEqual(I.findOverlaySlot(project, 150, 150), {trackId: 'o1'});
project.tracks.push({id: 'o2', kind: 'overlay', clips: []});
assert.deepEqual(I.findOverlaySlot(project, 60, 150), {trackId: 'o2'});
project.tracks.find(t => t.id === 'o2').locked = true;
assert.deepEqual(I.findOverlaySlot(project, 60, 150), {create: true});
project.tracks.push({id: 'o3', kind: 'overlay', clips: [clip(0, 900)]}, {id: 'o4', kind: 'overlay', clips: [clip(0, 900)]});
assert.equal(I.findOverlaySlot(project, 60, 150), null);

// Display order: front-most visual track first, audio last.
const order = I.displayTracks([{id: 'v1', kind: 'video'}, {id: 'v2', kind: 'video'}, {id: 'a1', kind: 'audio'}, {id: 'o1', kind: 'overlay'}, {id: 'a2', kind: 'audio'}]).map(t => t.id);
assert.deepEqual(order, ['o1', 'v2', 'v1', 'a1', 'a2']);
const tracks = [{kind: 'video'}, {kind: 'overlay'}, {kind: 'audio'}];
assert.equal(I.insertIndexFor(tracks, 'video'), 1);
assert.equal(I.insertIndexFor(tracks, 'overlay'), 3);
assert.equal(I.insertIndexFor([{kind: 'overlay'}], 'video'), 0);

// Transitions: setting, changing and removing one shifts the following clips together; repairs follow edits.
const T = window.VideoStudioTimeline;
const vid = {id: 'v', kind: 'video', duration_frames: 900};
const seq = () => ({assets: [vid], tracks: [{id: 't', kind: 'video', clips: [
  {id: 'a', asset_id: 'v', start: 0, offset: 0, duration: 60, fade_in: 0, fade_out: 0, volume: 1, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, fit: 'contain', transition: null},
  {id: 'b', asset_id: 'v', start: 60, offset: 0, duration: 60, fade_in: 0, fade_out: 0, volume: 1, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, fit: 'contain', transition: null},
  {id: 'c', asset_id: 'v', start: 120, offset: 0, duration: 60, fade_in: 0, fade_out: 0, volume: 1, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, fit: 'contain', transition: null}
]}]});
const starts = p => p.tracks[0].clips.slice().sort((x, y) => x.start - y.start).map(c => c.id + '@' + c.start);
let p = seq();
assert(T.setTransition(p, 't', 'a', 'dissolve', 15));
assert.deepEqual(starts(p), ['a@0', 'b@45', 'c@105'], 'a transition pulls the following clips together');
assert(T.validTimeline(p));
assert(T.setTransition(p, 't', 'a', 'dissolve', 6));
assert.deepEqual(starts(p), ['a@0', 'b@54', 'c@114'], 'a shorter transition pushes them back without colliding');
assert(T.validTimeline(p));
assert(T.setTransition(p, 't', 'a', 'none', 0));
assert.deepEqual(starts(p), ['a@0', 'b@60', 'c@120'], 'removing a transition restores the cut');
assert(T.validTimeline(p) && p.tracks[0].clips[0].transition === null);
p = seq(); p.tracks[0].clips[1].start = 80; p.tracks[0].clips[2].start = 150;
assert(T.setTransition(p, 't', 'a', 'black', 10));
assert.deepEqual(starts(p), ['a@0', 'b@50', 'c@120'], 'a transition across a gap closes the gap');
assert(!T.setTransition(p, 't', 'c', 'dissolve', 10), 'the last clip has no following clip');
// Deleting the following clip drops the dangling transition.
p = seq(); T.setTransition(p, 't', 'a', 'dissolve', 15); p.tracks[0].clips = p.tracks[0].clips.filter(c => c.id !== 'b');
T.repairTransitions(p);
assert(p.tracks[0].clips.find(c => c.id === 'a').transition === null && T.validTimeline(p));
// Moving the incoming clip changes the overlap: the transition follows it, or disappears without overlap.
p = seq(); T.setTransition(p, 't', 'a', 'dissolve', 15); p.tracks[0].clips.find(c => c.id === 'b').start = 40;
T.repairTransitions(p);
assert.equal(p.tracks[0].clips.find(c => c.id === 'a').transition.duration, 20);
assert(T.validTimeline(p));
p.tracks[0].clips.find(c => c.id === 'a').duration = 40;
T.repairTransitions(p);
assert(p.tracks[0].clips.find(c => c.id === 'a').transition === null && T.validTimeline(p));
const dragged = T.dragClip((() => { const q = seq(); T.setTransition(q, 't', 'a', 'dissolve', 15); return q; })(), 'a', 'end', -5, 't', 0);
assert(dragged && dragged.tracks[0].clips.find(c => c.id === 'a').transition.duration === 10, 'trimming the outgoing clip shortens its transition');

// Media helpers.
assert.deepEqual(M.thumbnailTimes(300, 4), [9, 81, 153, 225]);
assert.deepEqual(M.thumbnailTimes(0, 4), [0]);
assert.deepEqual(M.thumbnailTimes(2, 8), [0, 1]);
const peaks = M.peaksFromChannels([new Float32Array([0, 0.5, -1, 0.25]), new Float32Array([0.75, 0, 0, 0])], 4, 2, 100);
assert.deepEqual(Array.from(peaks), [0.75, 1]);
assert.equal(M.peaksFromChannels([new Float32Array(1000)], 100, 50, 7).length, 7, 'peak count is capped');
assert.equal(M.pickThumb([{frame: 0, url: 'a'}, {frame: 90, url: 'b'}], 60).url, 'b');
assert.equal(M.pickThumb([], 60), null);
assert.equal(M.isArtwork({name: 'title-1791446437502.png', kind: 'image'}), true);
assert.equal(M.isArtwork({name: 'sticker-heart.png', kind: 'image'}), true);
assert.equal(M.isArtwork({name: 'Holiday title-card.png', kind: 'image'}), false);
assert.equal(M.isArtwork({name: 'title-urlaub.png', kind: 'image'}), false, 'a user file named like a title stays visible');
assert.equal(M.isArtwork({name: 'sticker-logo.png', kind: 'image'}), false, 'only the built-in sticker names are artwork');
`
	cmd := exec.Command(node, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper checks: %v\n%s", err, output)
	}
}
