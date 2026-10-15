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

func TestVideoStudioEditTrims(t *testing.T) {
	runVideoStudioEditScript(t, `
// Ripple delete closes the removed time on the clips' own tracks; other tracks stay.
let p = base();
assert.equal(E.rippleDelete(p, ['y']), true);
assert.deepEqual(spans(p,'v1'), [[0,100,0],[100,100,400]]);
assert.deepEqual(spans(p,'a1'), [[0,300,0]]);
// Only time that is empty on every affected track closes (J-cut: audio starts 20 frames earlier).
p = project([
  track('v1','video',[clip('x',V,0,100), clip('y',V,100,100,{offset:100}), clip('z',V,200,100,{offset:200})]),
  track('a1','audio',[clip('ax',A,0,80), clip('ay',A,80,120,{offset:80}), clip('az',A,200,100,{offset:200})])
]);
assert.equal(E.rippleDelete(p, ['y','ay']), true);
assert.deepEqual(spans(p,'v1'), [[0,100,0],[100,100,200]]);
assert.deepEqual(spans(p,'a1'), [[0,80,0],[100,100,200]]);
valid(p);
// A gap closes on its own track.
p = base(); p.tracks[1].clips.push(clip('w2',V,200,50));
assert.deepEqual(E.gapAt(p,'v2',150), {trackId:'v2', start:110, end:200});
assert.equal(E.rippleDeleteGap(p,'v2',150), true);
assert.equal(at(p,'w2').start, 110);
assert.equal(E.gapAt(p,'v2',300), null, 'space after the last clip is not a gap');

// Ripple trim: the end edge moves later clips; the start edge keeps the clip start and shows later frames.
p = base();
assert.equal(E.rippleTrim(p, ['x'], 'end', -40), true);
assert.deepEqual(spans(p,'v1'), [[0,60,0],[60,100,200],[160,100,400]]);
assert.equal(E.rippleTrim(p, ['z'], 'end', 500), true, 'clamped at the media end');
assert.equal(at(p,'z').duration, 200);
assert.equal(E.rippleTrim(p, ['z'], 'end', 5), 'bounds');
p = base();
assert.equal(E.rippleTrim(p, ['y'], 'start', 30), true);
assert.deepEqual(spans(p,'v1'), [[0,100,0],[100,70,230],[170,100,400]]);
assert.equal(E.rippleTrim(p, ['x'], 'start', -10), 'bounds', 'no media before the first frame');
p = base(); p.tracks[0].locked = true;
assert.equal(E.rippleTrim(p, ['x'], 'end', -10), 'locked');

// Roll moves the cut between touching clips, bounded by both clips' media.
p = base();
assert.equal(E.roll(p, [['x','y']], 20), true);
assert.deepEqual(spans(p,'v1'), [[0,120,0],[120,80,220],[200,100,400]]);
assert.equal(E.roll(p, [['x','y']], -500), true);
assert.deepEqual(spans(p,'v1'), [[0,1,0],[1,199,101],[200,100,400]]);
assert.equal(E.roll(p, [['x','z']], 5), 'nothing', 'only neighbors roll');
p = pair();
assert.equal(E.roll(p, [['a','b']], 15), true);
assert.equal(at(p,'a').transition.duration, 10, 'a transition keeps its overlap');
assert.deepEqual(spans(p,'v1'), [[0,115,0],[105,85,115]]);
valid(p);

// Slip changes the source frames in place; stills have nothing to slip.
p = base();
assert.equal(E.slip(p, ['y'], 50), true);
assert.equal(at(p,'y').offset, 250);
assert.equal(E.slip(p, ['y'], 1000), true);
assert.equal(at(p,'y').offset, 500);
assert.equal(E.slip(p, ['y'], 1), 'bounds');
assert.equal(E.slip(base(), ['t'], 10), 'media');

// Slide moves a clip between its neighbors; they absorb the change.
p = base();
assert.equal(E.slide(p, 'y', 30), true);
assert.deepEqual(spans(p,'v1'), [[0,130,0],[130,100,200],[230,70,430]]);
assert.equal(E.slide(p, 'y', -1000), true);
assert.deepEqual(spans(p,'v1'), [[0,1,0],[1,100,200],[101,199,301]]);
valid(p);
p = base();
assert.equal(E.slide(p, 'w', -80), true, 'without neighbors the clip uses the gap');
assert.equal(at(p,'w').start, 0);
`)
}

func TestVideoStudioEditCutsAndClipboard(t *testing.T) {
	runVideoStudioEditScript(t, `
// Blade: the left part keeps id and fade in; the right part gets fade out and the outgoing transition.
let p = base();
at(p,'y').fade_in = 10; at(p,'y').fade_out = 12;
const out = {};
assert.equal(E.blade(p, 150, {trackIds:['v1','a1']}, out), true);
assert.equal(out.created.length, 2);
assert.deepEqual(spans(p,'v1'), [[0,100,0],[100,50,200],[150,50,250],[200,100,400]]);
assert.deepEqual([at(p,'y').fade_in, at(p,'y').fade_out], [10, 0]);
assert.deepEqual([at(p,out.created[0]).fade_in, at(p,out.created[0]).fade_out], [0, 12]);
assert.deepEqual(spans(p,'a1'), [[0,150,0],[150,150,150]]);
p = pair();
assert.equal(E.blade(p, 50, {trackIds:['v1']}, {}), true);
const pieces = p.tracks[0].clips.slice().sort((a, b) => a.start - b.start);
assert.equal(pieces[0].transition, null);
assert.deepEqual(pieces[1].transition, {type:'dissolve', duration:10});
valid(p);
assert.equal(E.blade(p, 95, {trackIds:['v1']}, {}), 'transition');
p = base();
assert.equal(E.blade(p, 150, {clipIds:['m']}, {}), true);
assert.equal(p.tracks[0].clips.length, 3, 'only the listed clip is cut');
assert.equal(p.tracks[2].clips.length, 2);
assert.equal(E.blade(p, 5000, {trackIds:['v1']}, {}), 'nothing');

// Moves overwrite by default, insert on request, change tracks within a kind and close gaps when magnetic.
p = base();
assert.equal(E.moveClips(p, ['x'], 'x', 150, 0, 'overwrite', {}), true);
assert.deepEqual(spans(p,'v1'), [[100,50,200],[150,100,0],[250,50,450]]);
p = base();
assert.equal(E.moveClips(p, ['x'], 'x', 150, 0, 'insert', {}), true);
assert.deepEqual(spans(p,'v1'), [[100,50,200],[150,100,0],[250,50,250],[300,100,400]]);
valid(p);
p = base();
assert.equal(E.moveClips(p, ['x','m'], 'x', 0, 1, 'overwrite', {}), true);
assert.deepEqual(spans(p,'v2'), [[0,100,0],[100,10,50]]);
assert.equal(at(p,'m').start, 0, 'clips of another kind keep their track');
assert.equal(E.moveClips(base(), ['x'], 'x', 0, 2, 'overwrite', {}), 'bounds', 'no third video track');
p = base();
assert.equal(E.moveClips(p, ['y'], 'y', 0, 1, 'overwrite', {magnetic:true}), true);
assert.deepEqual(spans(p,'v1'), [[0,100,0],[100,100,400]]);
assert.deepEqual(spans(p,'v2'), [[50,50,0],[100,100,200]]);
valid(p);

// Clipboard: relative times, original tracks, overwrite or insert, optional target tracks.
p = base();
const board = E.copyClips(p, ['y','m']);
assert.equal(board.span, 300);
assert.deepEqual(board.entries.map(e => [e.trackId, e.kind, e.kindIndex, e.clip.start]), [['v1','video',0,100],['a1','audio',0,0]]);
assert.equal(E.paste(p, board, 400, 'overwrite', {}), true);
assert.deepEqual(spans(p,'v1').at(-1), [500,100,200]);
assert.deepEqual(spans(p,'a1').at(-1), [400,300,0]);
const inserted = {};
assert.equal(E.paste(p, board, 50, 'insert', {out: inserted}), true);
assert.equal(inserted.created.length, 2);
assert.equal(at(p, inserted.created[0]).start, 150);
assert.equal(at(p,'w').start, 350);
valid(p);
p = base(); p.tracks[0].locked = true;
assert.equal(E.paste(p, E.copyClips(p, ['x']), 500, 'overwrite', {}), 'locked');
p = base();
assert.equal(E.paste(p, E.copyClips(p, ['x']), 400, 'overwrite', {targets:{video:'v2'}}), true);
assert.deepEqual(spans(p,'v2'), [[50,60,0],[400,100,0]]);

// Duplicate lands on the first place after the clips where every copy fits.
p = base();
const dup = {};
assert.equal(E.duplicateClips(p, ['w'], dup), true);
assert.equal(at(p, dup.created[0]).start, 110);
p = base();
assert.equal(E.duplicateClips(p, ['x'], {}), true);
assert.deepEqual(spans(p,'v1').at(-1), [300,100,0]);

// Paste attributes: picture skips audio clips, fades are clamped to the clip.
p = base();
const source = Object.assign({}, at(p,'x'), {x:0.1, y:0.2, width:0.5, height:0.5, opacity:0.4, volume:2, fade_in:150, fade_out:0});
assert.equal(E.pasteAttributes(p, source, ['t','m'], ['picture','sound']), true);
assert.deepEqual([at(p,'t').opacity, at(p,'t').fade_in, at(p,'t').volume], [0.4, 40, 2]);
assert.deepEqual([at(p,'m').x, at(p,'m').volume, at(p,'m').fade_in], [0, 2, 150]);
valid(p);

// Helpers for the timeline.
assert.equal(E.snapDelta([100, 200], 47, [0, 150, 300], 5), 50);
assert.equal(E.snapDelta([100], 47, [0, 300], 5), 47);
assert.deepEqual(E.clipsFrom(base(), 'v1', 100), ['y','z']);
assert.deepEqual(E.neighborAt(base(), 'x', 'end'), ['x','y']);
assert.equal(E.neighborAt(base(), 'w', 'start'), null);
`)
}
