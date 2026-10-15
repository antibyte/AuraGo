package ui

import (
	"os/exec"
	"testing"
)

// Shared fixtures for the edit checks: 30 fps, a 600-frame video and audio source and a still image.
const videoStudioEditPrelude = `
const assert = require('node:assert/strict');
global.window = {};
global.crypto = global.crypto || require('node:crypto').webcrypto;
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-edit.js');
const T = window.VideoStudioTimeline, E = window.VideoStudioEdit;
const V = {id:'v',kind:'video',has_audio:true,duration_frames:600};
const A = {id:'a',kind:'audio',has_audio:true,duration_frames:600};
const I = {id:'i',kind:'image',duration_frames:18000};
const clip = (id, asset, start, duration, extra) => Object.assign(T.makeClip(asset, start, id, {fps:30}), {duration}, extra || {});
const track = (id, kind, clips) => ({id, kind, name:id, muted:false, hidden:false, locked:false, clips: clips || []});
const project = tracks => ({version:2, fps:30, width:1280, height:720, assets:[V,A,I], tracks});
const base = () => project([
  track('v1','video',[clip('x',V,0,100), clip('y',V,100,100,{offset:200}), clip('z',V,200,100,{offset:400})]),
  track('v2','video',[clip('w',V,50,60)]),
  track('a1','audio',[clip('m',A,0,300)]),
  track('o1','overlay',[clip('t',I,20,40)])
]);
const pair = () => project([track('v1','video',[clip('a',V,0,100,{transition:{type:'dissolve',duration:10}}), clip('b',V,90,100,{offset:100})])]);
const at = (p, id) => (T.clipFor(p, id) || {}).clip;
const spans = (p, id) => p.tracks.find(t => t.id === id).clips.slice().sort((a, b) => a.start - b.start).map(c => [c.start, c.duration, c.offset]);
const valid = p => assert(T.validTimeline(p), 'invalid timeline: ' + JSON.stringify(p.tracks));
assert(T.validTimeline(base()) && T.validTimeline(pair()));
`

func runVideoStudioEditScript(t *testing.T, script string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio edit checks")
	}
	if output, err := exec.Command(node, "-e", videoStudioEditPrelude+script).CombinedOutput(); err != nil {
		t.Fatalf("edit checks: %v\n%s", err, output)
	}
}

func TestVideoStudioEditPlacement(t *testing.T) {
	runVideoStudioEditScript(t, `
// Overwrite trims, splits or removes what lies under the new clip.
let p = base();
assert.equal(E.overwrite(p, [{trackId:'v1', clips:[clip('n',V,150,100,{offset:10})]}]), true);
assert.deepEqual(spans(p,'v1'), [[0,100,0],[100,50,200],[150,100,10],[250,50,450]]);
assert.equal(at(p,'z').start, 250, 'a clip that loses its head keeps its id');
valid(p);
p = base();
assert.equal(E.overwrite(p, [{trackId:'a1', clips:[clip('n',A,100,50)]}]), true);
assert.deepEqual(spans(p,'a1'), [[0,100,0],[100,50,0],[150,150,150]]);
assert.equal(at(p,'m').duration, 100, 'the left part keeps the id');
p = base(); p.tracks[0].locked = true;
assert.equal(E.overwrite(p, [{trackId:'v1', clips:[clip('n',V,0,10)]}]), 'locked');
assert.equal(E.overwrite(base(), [{trackId:'v1', clips:[clip('n',A,0,10)]}]), 'kind');
assert.equal(E.overwrite(base(), [{trackId:'v1', clips:[clip('n',V,107990,20)]}]), 'limit');
// Clearing through a transition never leaves a clip inside its neighbor.
p = pair();
assert.equal(E.overwrite(p, [{trackId:'v1', clips:[clip('n',V,50,42)]}]), true);
assert.deepEqual(spans(p,'v1'), [[0,50,0],[50,42,0],[92,98,102]]);
assert(p.tracks[0].clips.every(c => !c.transition));
valid(p);

// Insert splits on target tracks, moves later clips on every unlocked track and leaves locked tracks alone.
p = base(); p.tracks[3].locked = true;
assert.equal(E.insertClips(p, 150, [{trackId:'v1', clips:[clip('n',V,150,30)]}]), true);
assert.deepEqual(spans(p,'v1'), [[0,100,0],[100,50,200],[150,30,0],[180,50,250],[230,100,400]]);
assert.deepEqual(spans(p,'a1'), [[0,300,0]], 'a clip spanning the point on another track stays whole');
assert.deepEqual(spans(p,'o1'), [[20,40,0]]);
valid(p);
p = base();
assert.equal(E.insertClips(p, 40, [{trackId:'a1', clips:[clip('n',A,40,20)]}]), true);
assert.deepEqual(spans(p,'a1'), [[0,40,0],[40,20,0],[60,260,40]]);
assert.deepEqual(spans(p,'v1'), [[0,100,0],[120,100,200],[220,100,400]]);
assert.equal(at(p,'w').start, 70);
assert.equal(at(p,'t').start, 20);
valid(p);
assert.equal(E.insertClips(pair(), 95, [{trackId:'v1', clips:[clip('n',V,95,10)]}]), 'transition');

// Lift delete removes clips on unlocked tracks only.
p = base(); p.tracks[3].locked = true;
assert.equal(E.removeClips(p, ['x','t']), true);
assert.equal(at(p,'x'), undefined);
assert.equal(at(p,'t').start, 20);
assert.equal(E.removeClips(p, ['t']), 'locked');
assert.equal(E.removeClips(p, ['nope']), 'nothing');
`)
}
