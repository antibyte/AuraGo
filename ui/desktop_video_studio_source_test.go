package ui

import (
	"os/exec"
	"testing"
)

// Fixtures for the source-monitor rules: 30 fps; a 600-frame video with sound, a 300-frame silent video, an audio
// file and a still (5 s = 150 frames). The base project has empty video, audio and overlay tracks.
const videoStudioSourcePrelude = `
const assert = require('node:assert/strict');
global.window = {};
global.crypto = global.crypto || require('node:crypto').webcrypto;
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-edit.js');
require('./js/desktop/apps/video-studio-source-edit.js');
const T = window.VideoStudioTimeline, E = window.VideoStudioEdit, S = window.VideoStudioSourceEdit;
const V = {id:'v',kind:'video',has_audio:true,duration_frames:600,name:'v'};
const Q = {id:'q',kind:'video',has_audio:false,duration_frames:300,name:'q'};
const A = {id:'a',kind:'audio',has_audio:true,duration_frames:600,name:'a'};
const I = {id:'i',kind:'image',name:'i'};
const track = (id, kind) => ({id, kind, name:id, muted:false, hidden:false, locked:false, clips:[]});
const base = () => ({version:2, fps:30, width:1280, height:720, assets:[V,Q,A,I], tracks:[track('v1','video'), track('v2','video'), track('a1','audio'), track('o1','overlay')]});
const R = (asset, marks) => S.sourceRange(base(), asset, marks || {});
`

func runVideoStudioSourceScript(t *testing.T, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio source checks")
	}
	if output, err := exec.Command(node, "-e", videoStudioSourcePrelude+script).CombinedOutput(); err != nil {
		t.Fatalf("source checks: %v\n%s", err, output)
	}
}

func TestVideoStudioSourceThreePoint(t *testing.T) {
	runVideoStudioSourceScript(t, `
const plan = input => S.plan(Object.assign({timelineIn:null, timelineOut:null, playhead:0, still:false}, input));
// Timeline in and out: placed at in with out - in; the source in decides the start and the source out is ignored.
assert.deepEqual(plan({timelineIn:30, timelineOut:90, range:R(V,{in:100,out:200})}), {start:30, duration:60, offset:100});
// No fit-to-fill: a source that cannot cover the range is refused.
assert.equal(plan({timelineIn:0, timelineOut:500, range:R(Q)}).reason, 'media');
assert.equal(plan({timelineIn:0, timelineOut:60, range:R(Q,{in:250})}).reason, 'media');
// Only in: the source range's length. Only out: backtimed so the clip ends at out.
assert.deepEqual(plan({timelineIn:30, range:R(V)}), {start:30, duration:600, offset:0});
assert.deepEqual(plan({timelineIn:30, range:R(V,{in:100,out:160})}), {start:30, duration:60, offset:100});
assert.deepEqual(plan({timelineOut:200, range:R(V,{in:100,out:160})}), {start:140, duration:60, offset:100});
assert.equal(plan({timelineOut:200, range:R(V)}).reason, 'start', 'backtiming before the timeline start is refused');
// Neither: at the playhead.
assert.deepEqual(plan({playhead:45, range:R(V)}), {start:45, duration:600, offset:0});
assert.deepEqual(plan({playhead:45, range:R(V,{in:100,out:160})}), {start:45, duration:60, offset:100});
// Stills: 5 s unless the timeline in and out define the length; their offset stays 0.
assert.equal(S.sourceFrames(base(), I), 150);
assert.deepEqual(plan({playhead:10, still:true, range:R(I)}), {start:10, duration:150, offset:0});
assert.deepEqual(plan({timelineIn:30, timelineOut:90, still:true, range:R(I)}), {start:30, duration:60, offset:0});
assert.deepEqual(plan({timelineOut:200, still:true, range:R(I)}), {start:50, duration:150, offset:0});
assert.equal(plan({timelineOut:100, still:true, range:R(I)}).reason, 'start');
`)
}

func TestVideoStudioSourcePlacements(t *testing.T) {
	runVideoStudioSourceScript(t, `
const place = (asset, options) => S.placements(base(), asset, Object.assign({start:30, duration:60, offset:100, targets:{}, part:'both'}, options));
// A video with sound and an audio target: a linked pair, the video detached from its embedded sound.
let built = place(V, {targets:{video:'v1', audio:'a1'}});
assert.equal(built.placements.length, 2);
const video = built.placements[0].clips[0], audio = built.placements[1].clips[0];
assert.deepEqual([built.placements[0].trackId, built.placements[1].trackId], ['v1', 'a1']);
assert.equal(video.audio_detached, true);
assert.equal(typeof video.link, 'string');
assert.equal(audio.link, video.link);
assert.equal(audio.audio_detached, undefined);
assert.deepEqual([video.start, video.duration, video.offset, audio.start, audio.duration, audio.offset], [30, 60, 100, 30, 60, 100]);
assert.equal(built.created.length, 2);
// Without an audio target: one video clip with its embedded sound.
built = place(V, {targets:{video:'v1'}});
assert.equal(built.placements.length, 1);
assert.equal(built.placements[0].clips[0].link, undefined);
assert.equal(built.placements[0].clips[0].audio_detached, undefined);
// Picture only detaches the embedded sound; sound only is an audio clip made from the video.
built = place(V, {targets:{video:'v1', audio:'a1'}, part:'picture'});
assert.equal(built.placements[0].clips[0].audio_detached, true);
assert.equal(built.placements[0].clips[0].link, undefined);
built = place(V, {targets:{audio:'a1'}, part:'sound'});
assert.deepEqual(built.placements.map(item => item.trackId), ['a1']);
assert.equal(built.placements[0].clips[0].asset_id, 'v');
// Media without the requested part, or without a target for it, is refused.
assert.equal(place(Q, {targets:{audio:'a1'}, part:'sound'}).reason, 'media');
assert.equal(place(V, {targets:{}}).reason, 'target');
assert.equal(place(A, {targets:{video:'v1'}, part:'both'}).reason, 'target');
// A still takes the overlay when no video target accepts it.
built = place(I, {start:0, duration:150, offset:0, targets:{overlay:'o1'}});
assert.equal(built.placements[0].trackId, 'o1');
assert.equal(built.placements[0].clips[0].offset, 0);
// The placements go through the edit module like any other edit.
const p = base();
built = place(V, {targets:{video:'v1', audio:'a1'}});
assert.equal(E.insertClips(p, 30, built.placements), true);
assert.deepEqual(p.tracks[0].clips.map(clip => [clip.start, clip.duration]), [[30, 60]]);
// Marks: stored in seconds, read back in frames; invalid values read as unset; the oldest entries go first.
const entry = S.entryFor(base(), {in:45, out:90}, 7);
assert.deepEqual(entry, {in:1.5, out:3, at:7});
assert.deepEqual(S.marksFrom(entry, base(), 600), {in:45, out:90});
assert.deepEqual(S.marksFrom(undefined, base(), 600), {in:null, out:null});
assert.deepEqual(S.marksFrom({in:700, out:900}, base(), 600), {in:null, out:null});
assert.deepEqual(Object.keys(S.pruneMarks({a:{at:1}, b:{at:3}, c:{at:2}}, 2)).sort(), ['b', 'c']);
`)
}
