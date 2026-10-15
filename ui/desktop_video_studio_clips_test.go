package ui

import (
	"os/exec"
	"testing"
)

func TestVideoStudioClipModelChecks(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio clip model checks")
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-core.js');
const T = window.VideoStudioTimeline, P = window.VideoStudioParts;
const V = {id:'v',kind:'video',has_audio:true,duration_frames:300}, A = {id:'a',kind:'audio',has_audio:true,duration_frames:300};
const I = {id:'i',kind:'image',duration_frames:18000,width:1,height:1,has_audio:false};
const clip = (id, asset, start, duration, extra) => Object.assign(T.makeClip(asset, start, id, {fps:30}), {duration}, extra || {});
const track = (id, kind, clips) => ({id, kind, name: id, muted:false, hidden:false, locked:false, clips: clips || []});
const project = tracks => ({version:2, fps:30, width:1280, height:720, assets:[V,A,I], tracks});
assert(T.accepts(track('v1','video'), I), 'images play on video tracks');
assert(!T.accepts(track('o1','overlay'), V), 'overlays still take images only');
assert(T.validTimeline(project([track('v1','video',[clip('x',V,0,150,{speed:2})])])), 'speed 2 with 149 source steps fits');
assert(!T.validTimeline(project([track('v1','video',[clip('x',V,0,151,{speed:2})])])), 'the extent rule uses speed');
assert(!T.validTimeline(project([track('v1','video',[clip('x',V,0,10,{speed:0.1})])])), 'speed below 0.25 is refused');
assert(!T.validTimeline(project([track('v1','video',[clip('x',I,0,10,{speed:2})])])), 'images keep speed 1');
assert(T.validTimeline(project([track('v1','video',[clip('x',I,0,300)])])), 'an image on a video track is valid');
assert(!T.validTimeline(project([track('a1','audio',[clip('x',V,0,10,{audio_detached:true})])])), 'detached audio only on video tracks');
const linked = project([track('v1','video',[clip('x',V,20,100,{offset:50,speed:2})]), track('a1','audio',[clip('ax',V,20,100,{offset:50,speed:2})])]);
const trimmed = T.dragClip(linked,'x','start',-10,'v1',0,[],['ax']);
assert.deepEqual([trimmed.tracks[0].clips[0].start, trimmed.tracks[0].clips[0].offset, trimmed.tracks[0].clips[0].duration], [10, 30, 110], 'a speed-2 head moves two source frames per frame');
assert.deepEqual([trimmed.tracks[1].clips[0].start, trimmed.tracks[1].clips[0].offset], [10, 30], 'the linked partner moves with the edge');
const ended = T.dragClip(linked,'x','end',10,'v1',0,[],['ax']);
assert.equal(ended.tracks[1].clips[0].duration, 110, 'the linked end follows the dragged end');
const plain = T.dragClip(project([track('v1','video',[clip('x',V,20,100,{offset:50,speed:2})])]),'x','start',-10,'v1',0);
assert.equal(plain.tracks[0].clips[0].offset, 30, 'without partners the edge still moves by speed');
const canon = P.canonicalProject({version:2,name:'x',width:1280,height:720,fps:30,assets:[V],tracks:[track('v1','video',[clip('x',V,0,30,{speed:1, link:'L', audio_detached:true, disabled:true, label:'teal'})])]});
assert.deepEqual(Object.keys(canon.tracks[0].clips[0]).slice(-5), ['transition','link','audio_detached','disabled','label'], 'new clip keys follow Go order and speed 1 is absent');
const slow = P.canonicalProject({version:2,name:'x',width:1280,height:720,fps:30,assets:[V],tracks:[track('v1','video',[clip('x',V,0,30,{speed:0.5})])]});
assert.equal(slow.tracks[0].clips[0].speed, 0.5);
assert(!('link' in P.canonicalProject({version:2,name:'x',width:1280,height:720,fps:30,assets:[V],tracks:[track('v1','video',[clip('x',V,0,30,{link:''})])]}).tracks[0].clips[0]), 'an empty link is omitted');
assert.deepEqual(P.LABEL_COLORS, ['red','orange','yellow','green','teal','blue','purple','pink']);
`
	if output, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("clip model checks: %v\n%s", err, output)
	}
}
