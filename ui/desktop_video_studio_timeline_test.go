package ui

import (
	"os/exec"
	"testing"
)

func TestVideoStudioTimelineEdits(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for timeline checks")
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-timeline.js');
const T = window.VideoStudioTimeline;
const asset = {id:'a',kind:'video',has_audio:true,duration_frames:300};
const clip = Object.assign(T.makeClip(asset, 30, 'c'), {duration:90,offset:30});
const track = (id, kind, clips=[]) => ({id,kind,clips});
const project = {assets:[asset],tracks:[track('v1','video',[clip]),track('v2','video'),track('audio','audio')]};
assert(T.validTimeline(project));
const trimmed = T.dragClip(project,'c','start',-100,'v1',0);
assert.equal(trimmed.tracks[0].clips[0].start,0);
assert.equal(trimmed.tracks[0].clips[0].offset,0);
assert.equal(trimmed.tracks[0].clips[0].duration,120);
assert.equal(project.tracks[0].clips[0].offset,30,'drag mutates the undo snapshot');
const end = T.dragClip(project,'c','end',1000,'v1',0);
assert.equal(end.tracks[0].clips[0].duration,270,'trim extends past the source');
const moved = T.dragClip(project,'c','move',25,'v2',0);
assert.equal(moved.tracks[0].clips.length,0);
assert.equal(moved.tracks[1].clips[0].start,55);
assert(T.dragClip(project,'c','move',0,'audio',0),'video audio cannot be placed on an audio track');
project.tracks[1].locked = true;
assert.equal(T.dragClip(project,'c','move',0,'v2',0),null);
project.tracks[0].clips.push(Object.assign(T.makeClip(asset,150,'d'),{duration:90}));
assert.equal(T.dragClip(project,'c','move',70,'v1',0),null,'overlap without transition allowed');
project.tracks[0].clips[0].transition = {type:'dissolve',duration:15};
project.tracks[0].clips[1].start = 105;
assert(T.validTimeline(project));
project.tracks[0].clips[1].start++;
assert(!T.validTimeline(project),'transition duration need not match overlap');
project.tracks[0].clips[0].start = Number.MAX_SAFE_INTEGER;
assert(!T.validTimeline(project));
assert.equal(T.makeClip(asset, 108000, 'end', {fps:30}).duration,1);
assert.equal(T.makeClip(asset, 0, 'pal', {fps:25}).duration,300,'a clip keeps its asset frames');
assert(T.MIN_ZOOM * 52 * 3600 <= 1000,'sixty minutes fit the timeline');
const eight = {fps:30,assets:[asset],tracks:Array.from({length:8},(_,i)=>track('v'+i,'video'))};
assert(T.validTimeline(eight) && !T.trackCapacity(eight,'video'));
eight.tracks.push(track('v8','video'));
assert(!T.validTimeline(eight),'a ninth video track is rejected');
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("timeline checks: %v\n%s", err, output)
	}
}
