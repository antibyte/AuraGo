package ui

import "testing"

func TestVideoStudioEditLinks(t *testing.T) {
	runVideoStudioEditScript(t, `
// Link groups: the video and audio parts of one clip share an id; the edges on one frame belong to the group.
let p = project([
  track('v1','video',[clip('x',V,0,100,{link:'L'}), clip('y',V,100,100,{offset:200})]),
  track('a1','audio',[clip('ax',A,0,100,{link:'L'})])
]);
assert.deepEqual(E.linkGroup(p,'x').sort(), ['ax','x']);
assert.deepEqual(E.linkGroup(p,'y'), ['y']);
assert.deepEqual(E.linkedEdges(p,'x','end').sort(), ['ax','x']);
assert.deepEqual(E.withLinks(p,['y','x']).sort(), ['ax','x','y']);
assert.equal(E.rippleTrim(p, E.linkedEdges(p,'x','end'), 'end', -40), true);
assert.equal(at(p,'ax').duration, 60, 'the partner trims with the edge');
assert.equal(at(p,'y').start, 60, 'the later clip on the video track ripples');
valid(p);

// Blade: the linked partner is cut too; the right parts share one new id, unlinked clips get none.
p = project([track('v1','video',[clip('x',V,0,100,{link:'L'}), clip('z',V,100,50)]), track('a1','audio',[clip('ax',A,0,100,{link:'L'})])]);
const out = {};
assert.equal(E.blade(p, 50, {clipIds:['x'], linked:true}, out), true);
assert.equal(out.created.length, 2, 'the linked audio is cut with its video');
const rightV = at(p, out.created[0]), rightA = at(p, out.created[1]);
assert.equal(at(p,'x').link, 'L'); assert.equal(at(p,'ax').link, 'L');
assert.ok(rightV.link && rightV.link !== 'L' && rightV.link === rightA.link, 'the right parts share a new link id');
const plainCut = {};
assert.equal(E.blade(p, 120, {clipIds:['z']}, plainCut), true);
assert.ok(!at(p, plainCut.created[0]).link, 'an unlinked clip gets no link id');
valid(p);
p = project([track('v1','video',[clip('x',V,0,100,{link:'L'})]), track('a1','audio',[clip('ax',A,0,100,{link:'L'})])]);
assert.equal(E.blade(p, 50, {clipIds:['x']}, {}), true);
assert.equal(p.tracks[1].clips.length, 1, 'without linked the partner stays whole');
assert.ok(!p.tracks[0].clips.find(c => c.start === 50).link, 'a lone right part is unlinked');

// Speed: the default keeps the source range and ripples; keep duration changes the source range; partners follow.
p = project([track('v1','video',[clip('x',V,0,100), clip('y',V,100,100,{offset:100})])]);
assert.equal(E.setSpeed(p, ['x'], 2, false), true);
assert.deepEqual(spans(p,'v1'), [[0,50,0],[50,100,100]], 'the default halves the clip and ripples');
assert.equal(at(p,'x').speed, 2);
valid(p);
p = project([track('v1','video',[clip('x',V,0,100)])]);
assert.equal(E.setSpeed(p, ['x'], 0.5, true), true);
assert.deepEqual(spans(p,'v1'), [[0,100,0]], 'keep duration changes the source range only');
assert.equal(at(p,'x').speed, 0.5);
assert.equal(E.setSpeed(project([track('v1','video',[clip('x',V,0,100,{offset:550})])]), ['x'], 0.5, true), 'media', 'the source extent rule refuses');
assert.equal(E.setSpeed(project([track('o1','overlay',[clip('t',I,20,40)])]), ['t'], 2, false), 'media', 'stills have no speed');
p = project([track('v1','video',[clip('x',V,0,100,{link:'L'})]), track('a1','audio',[clip('ax',A,0,100,{link:'L'})])]);
assert.equal(E.setSpeed(p, E.withLinks(p, ['x']), 2, false), true);
assert.equal(at(p,'ax').speed, 2); assert.equal(at(p,'ax').duration, 50, 'partners get the same speed and length');
p = project([track('v1','video',[clip('x',V,0,600)])]);
assert.equal(E.setSpeed(p, ['x'], 0.5, false), true);
assert.equal(at(p,'x').duration, 1199, 'slowing a clip that uses all of its media keeps the last frame inside it');
assert.equal(E.setSpeed(pair(), ['a'], 16, false), 'bounds', 'a clip never gets shorter than its transition');

// Detach audio: the free audio track takes the sound; a busy one gets a new track; eight busy tracks refuse.
p = project([track('v1','video',[clip('x',V,50,100,{offset:10})]), track('a1','audio',[clip('m',A,0,40)])]);
assert.equal(E.detachAudio(p, 'x', {clip:'ax', link:'L', track:'a2', trackName:'Audio 2'}), true);
assert.deepEqual(spans(p,'a1'), [[0,40,0],[50,100,10]], 'the free audio track takes the sound');
assert.equal(at(p,'x').audio_detached, true); assert.equal(at(p,'x').link, 'L'); assert.equal(at(p,'ax').link, 'L');
assert.equal(E.audioPartner(p,'x'), 'ax');
valid(p);
assert.equal(E.reattachAudio(p,'x'), true);
assert.equal(at(p,'ax'), undefined, 'reattach deletes the audio clip');
assert.ok(!at(p,'x').audio_detached && !at(p,'x').link, 'reattach clears the flag and the lone link');
p = project([track('v1','video',[clip('x',V,50,100)]), track('a1','audio',[clip('m',A,60,40)])]);
assert.equal(E.detachAudio(p,'x',{clip:'ax',link:'L',track:'a2',trackName:'Audio 2'}), true);
assert.deepEqual(spans(p,'a2'), [[50,100,0]], 'a new audio track is added while fewer than eight exist');
const busy = project([track('v1','video',[clip('x',V,50,100)])].concat(Array.from({length:8}, (_, i) => track('a'+i,'audio',[clip('m'+i,A,60,40)]))));
assert.equal(E.detachAudio(busy,'x',{clip:'ax',link:'L',track:'a9',trackName:'Audio 9'}), 'tracks');

// Link (Ctrl+L): a shared group, unlinking, and the sixteen-clip limit.
p = project([track('v1','video',[clip('x',V,0,50), clip('y',V,50,50)])]);
assert.equal(E.setLink(p, ['x','y'], 'G'), true);
assert.deepEqual(E.linkGroup(p,'y').sort(), ['x','y']);
assert.equal(E.setLink(p, ['x'], ''), true);
assert.ok(!at(p,'x').link && !at(p,'y').link, 'a group of one is no group');
const many = project([track('v1','video', Array.from({length:17}, (_, i) => clip('c'+i,V,i*10,10)))]);
assert.equal(E.setLink(many, many.tracks[0].clips.map(c => c.id), 'G'), 'link');
p = project([track('v1','video',[clip('x',V,0,50,{link:'G'})]), track('v2','video',[clip('y',V,0,50,{link:'G'})])]);
p.tracks[1].locked = true;
assert.equal(E.setLink(p, ['x'], ''), true);
assert.equal(at(p,'y').link, 'G', 'a locked clip keeps its link');

// Labels and enable state.
p = project([track('v1','video',[clip('x',V,0,50)])]);
assert.equal(E.setLabel(p, ['x'], 'teal'), true);
assert.equal(at(p,'x').label, 'teal');
assert.equal(E.setDisabled(p, ['x'], true), true);
assert.equal(at(p,'x').disabled, true);
assert.equal(E.setDisabled(p, ['x'], false), true);
assert.ok(!at(p,'x').disabled);

// Paste: a pasted group gets its own link id; a lone copy is unlinked. Paste attributes: speed uses the timing rules.
p = project([track('v1','video',[clip('x',V,0,100,{link:'L'})]), track('a1','audio',[clip('ax',A,0,100,{link:'L'})])]);
const board = E.copyClips(p, ['x','ax']);
assert.equal(E.paste(p, board, 400, 'overwrite', {}), true);
const pasted = p.tracks[0].clips.find(c => c.start === 400), pastedAudio = p.tracks[1].clips.find(c => c.start === 400);
assert.ok(pasted.link && pasted.link !== 'L' && pasted.link === pastedAudio.link, 'a pasted group gets its own link id');
valid(p);
assert.equal(E.paste(p, E.copyClips(p, ['x']), 600, 'overwrite', {}), true);
assert.ok(!p.tracks[0].clips.find(c => c.start === 600).link, 'a lone copy is unlinked');
p = project([track('v1','video',[clip('x',V,0,100), clip('y',V,100,100,{offset:100})])]);
assert.equal(E.pasteAttributes(p, Object.assign({}, at(p,'x'), {speed: 2}), ['x'], ['speed']), true);
assert.deepEqual(spans(p,'v1'), [[0,50,0],[50,100,100]], 'pasted speed ripples like the speed command');
p = project([track('v1','video',[clip('x',V,0,100,{link:'L'}), clip('s',I,100,50)]), track('a1','audio',[clip('ax',A,0,100,{link:'L'})])]);
assert.equal(E.pasteAttributes(p, Object.assign({}, at(p,'x'), {speed: 2}), ['x','s'], ['speed']), true);
assert.equal(at(p,'ax').speed, 2, 'pasted speed reaches the link partners');
assert.ok(!at(p,'s').speed, 'pasted speed skips stills');
valid(p);

// Reverse: the clip and its same-asset partner play the reversed asset with mirrored offsets.
const reversed = {id:'r', name:'Clip (reversed).mp4', kind:'video', has_audio:true, duration_frames:100};
p = project([track('v1','video',[clip('x',V,50,100,{offset:200,link:'L'})]), track('a1','audio',[clip('ax',V,50,100,{offset:200,link:'L'})])]);
p.assets.push(reversed);
assert.equal(E.reverseClips(p, 'x', {assetId:'r', rangeStart:200, frames:100}), true);
assert.equal(at(p,'x').asset_id, 'r'); assert.equal(at(p,'x').offset, 0);
assert.equal(at(p,'ax').asset_id, 'r'); assert.equal(at(p,'ax').offset, 0);
p = project([track('v1','video',[clip('x',V,50,100,{offset:200,link:'L'})]), track('a1','audio',[clip('ax',V,50,100,{offset:100,link:'L'})])]);
p.assets.push(reversed);
assert.equal(E.reverseClips(p, 'x', {assetId:'r', rangeStart:200, frames:100}), 'media', 'a partner outside the range refuses');

// Freeze frame: the clip is cut at the playhead and a still is inserted on its own track.
p = project([track('v1','video',[clip('x',V,0,100), clip('y',V,100,100,{offset:100})])]);
p.assets.push({id:'s', name:'s.png', kind:'image', duration_frames:0, width:64, height:64});
assert.equal(E.freezeFrame(p, 'x', 40, {asset_id:'s', clip_id:'still', duration:60}), true);
assert.deepEqual(spans(p,'v1'), [[0,40,0],[40,60,0],[100,60,40],[160,100,100]]);
assert.equal(at(p,'still').asset_id, 's');
valid(p);
`)
}

// TestVideoStudioEditSplitsKeepLinkGroups: overwrite and insert splits inside linked clips never grow a link group.
func TestVideoStudioEditSplitsKeepLinkGroups(t *testing.T) {
	runVideoStudioEditScript(t, `
const groups = p => { const m = {}; p.tracks.forEach(t => t.clips.forEach(c => { if (c.link) m[c.link] = (m[c.link] || 0) + 1; })); return m; };

// Overwrite splits follow the blade rule: a lone right part is unlinked, so sixteen drops into a detached clip keep its
// group at two clips (it used to reach eighteen, which the server refuses).
let p = project([track('v1','video',[clip('x',V,0,600)])]);
assert.equal(E.detachAudio(p, 'x', {clip:'ax', link:'L', track:'a1', trackName:'A1'}), true);
for (let k = 0; k < 16; k++) assert.equal(E.overwrite(p, [{trackId:'v1', clips:[clip('i'+k,I,10+k*35,20)]}]), true);
assert.deepEqual(groups(p), {L:2}, 'overwrite drops never grow the group');
valid(p);
p = project([track('v1','video',[clip('x',V,0,300,{link:'L'})]), track('a1','audio',[clip('ax',V,0,300,{link:'L'})])]);
assert.equal(E.overwrite(p, [{trackId:'v1', clips:[clip('n',I,100,50)]}, {trackId:'a1', clips:[clip('na',A,100,50)]}]), true);
let rv = p.tracks[0].clips.find(c => c.start === 150), ra = p.tracks[1].clips.find(c => c.start === 150);
assert.ok(rv.link && rv.link !== 'L' && rv.link === ra.link, 'right parts of one group split together share a new link id');
assert.deepEqual(E.linkGroup(p,'x').sort(), ['ax','x']);
valid(p);
// Insert edits split the same way.
p = project([track('v1','video',[clip('x',V,0,300,{link:'L'})]), track('a1','audio',[clip('ax',V,0,300,{link:'L'})])]);
assert.equal(E.insertClips(p, 100, [{trackId:'v1', clips:[clip('n',I,100,50)]}]), true);
assert.ok(!p.tracks[0].clips.find(c => c.start === 150).link, 'an insert into one linked clip unlinks the lone right part');
assert.deepEqual(groups(p), {L:2});
p = project([track('v1','video',[clip('x',V,0,300,{link:'L'})]), track('a1','audio',[clip('ax',V,0,300,{link:'L'})])]);
assert.equal(E.insertClips(p, 100, [{trackId:'v1', clips:[clip('n',I,100,50)]}, {trackId:'a1', clips:[clip('na',A,100,50)]}]), true);
rv = p.tracks[0].clips.find(c => c.start === 150); ra = p.tracks[1].clips.find(c => c.start === 150);
assert.ok(rv.link && rv.link !== 'L' && rv.link === ra.link, 'an insert across the group gives the right parts a new shared id');
valid(p);
// Go's limit (MaxLinkClips): a group above sixteen clips is invalid and edits refuse to leave one behind.
const big = project([track('v1','video', Array.from({length:17}, (_, i) => clip('c'+i,V,i*10,10,{link:'G'})))]);
assert.equal(T.validTimeline(big), false, 'a link group holds at most sixteen clips');
assert.equal(E.setLabel(big, ['c0'], 'red'), 'link');
big.tracks[0].clips.pop();
valid(big);
`)
}

// TestVideoStudioEditLinkedCutInTransition: a linked blade or split never cuts only part of a group.
func TestVideoStudioEditLinkedCutInTransition(t *testing.T) {
	runVideoStudioEditScript(t, `
// A linked blade or split inside a transition cuts nothing when a member of the group would stay whole.
let p, out;
const faded = () => project([track('v1','video',[clip('a',V,0,100,{transition:{type:'dissolve',duration:20}}), clip('b',V,80,100,{offset:300})])]);
p = faded();
assert.equal(E.detachAudio(p, 'b', {clip:'ab', link:'L', track:'a1', trackName:'A1'}), true);
out = {};
assert.equal(E.blade(p, 90, {trackIds:['v1'], linked:true}, out), 'transition');
assert.deepEqual(spans(p,'a1'), [[80,100,300]], 'the detached audio stays whole');
assert.equal(E.blade(p, 90, {clipIds:['b'], linked:true}, {}), 'transition', 'S refuses the same way');
p = faded(); p.tracks.push(track('v2','video',[clip('w',V,50,100)]));
assert.equal(E.blade(p, 90, {trackIds:['v1','v2'], linked:true}, out = {}), true, 'unlinked clips beside a transition are still cut');
assert.deepEqual(spans(p,'v2'), [[50,40,0],[90,60,40]]);
assert.equal(p.tracks[0].clips.length, 2);
`)
}

// TestVideoStudioEditDetachedSound: copies and reattach never leave a detached clip silent without an audio clip.
func TestVideoStudioEditDetachedSound(t *testing.T) {
	runVideoStudioEditScript(t, `
let p, out;
// A detached clip copied without its audio clip plays its own sound; copied with it, it stays detached.
p = project([track('v1','video',[clip('x',V,100,200,{offset:50})])]);
assert.equal(E.detachAudio(p, 'x', {clip:'ax', link:'L', track:'a1', trackName:'A1'}), true);
assert.equal(E.paste(p, E.copyClips(p, ['x']), 400, 'overwrite', {out: out = {}}), true);
assert.ok(!at(p, out.created[0]).audio_detached, 'a lone copy is not silent');
assert.equal(E.paste(p, E.copyClips(p, ['x','ax']), 700, 'overwrite', {out: out = {}}), true);
const [copyV, copyA] = out.created.map(id => at(p, id));
assert.equal(copyV.audio_detached, true);
assert.equal(E.audioPartner(p, copyV.id), copyA.id, 'a copy with its audio clip stays detached');
assert.equal(E.duplicateClips(p, ['x'], out = {}), true);
assert.ok(!at(p, out.created[0]).audio_detached, 'duplicate follows paste');
valid(p);

// Reattach: with the audio clip gone (Alt-delete) it restores the clip's sound; while an audio clip of the same media
// still plays with it (J-cut, unlinked by an Alt blade) it is not offered.
p = project([track('v1','video',[clip('x',V,100,200,{offset:100})])]);
assert.equal(E.detachAudio(p, 'x', {clip:'ax', link:'L', track:'a1', trackName:'A1'}), true);
assert.equal(E.canReattach(p, 'x'), true);
assert.equal(E.removeClips(p, ['ax']), true);
assert.equal(E.canReattach(p, 'x'), true, 'an orphaned detached clip can get its sound back');
assert.equal(E.reattachAudio(p, 'x'), true);
assert.ok(!at(p,'x').audio_detached && !at(p,'x').link, 'the flag and the lone link are cleared');
valid(p);
p = project([track('v1','video',[clip('x',V,100,200,{offset:100})])]);
assert.equal(E.detachAudio(p, 'x', {clip:'ax', link:'L', track:'a1', trackName:'A1'}), true);
assert.equal(E.rippleTrim(p, ['ax'], 'start', -30), true);
assert.equal(E.canReattach(p, 'x'), false, 'a J-cut audio clip still holds the sound');
assert.equal(E.reattachAudio(p, 'x'), 'nothing');
p = project([track('v1','video',[clip('x',V,100,200,{offset:100})])]);
assert.equal(E.detachAudio(p, 'x', {clip:'ax', link:'L', track:'a1', trackName:'A1'}), true);
assert.equal(E.blade(p, 200, {clipIds:['x']}, out = {}), true);
assert.equal(E.canReattach(p, out.created[0]), false, 'the uncut audio clip still plays under the unlinked part');
assert.equal(E.reattachAudio(p, out.created[0]), 'nothing');
`)
}

// TestVideoStudioEditReverseUnionAndFreezeSource: reverse covers the union of the source ranges of the clip's same-asset
// group members and plays the group backwards around the reversed clip, so linked picture and sound stay in sync; a
// freeze applies only while the clip still shows the captured frame.
func TestVideoStudioEditReverseUnionAndFreezeSource(t *testing.T) {
	runVideoStudioEditScript(t, `
const copy = value => JSON.parse(JSON.stringify(value));
const sourceAt = (clip, f) => Math.floor(clip.offset + (f - clip.start) * (clip.speed || 1) + 1e-9);
// Reversed frame k is source frame rangeStart + frames - 1 - k.
const reversedSource = (range, clip, f) => range.rangeStart + range.frames - 1 - sourceAt(clip, f);
// The group plays backwards around the anchor: at timeline frame anchor.start + anchor.end - 1 - f every member shows
// the source frame it showed at f.
function mirrored(before, after, anchorId, range) {
  const anchor = at(before, anchorId), pivot = 2 * anchor.start + anchor.duration - 1;
  before.tracks.forEach(t => t.clips.forEach(old => {
    const now = at(after, old.id);
    assert.equal(now.asset_id, 'r', old.id + ' switches to the reversed asset');
    assert.equal(now.duration, old.duration, old.id + ' keeps its duration');
    for (let f = old.start; f < old.start + old.duration; f++) assert.equal(reversedSource(range, now, pivot - f), sourceAt(old, f), old.id + ' at frame ' + f);
  }));
}
// At every timeline frame both clips play, they show the same frame of their (same) asset.
function inSync(p, a, b) {
  const x = at(p, a), y = at(p, b);
  assert.equal(x.asset_id, y.asset_id);
  let shared = 0;
  for (let f = Math.max(x.start, y.start); f < Math.min(x.start + x.duration, y.start + y.duration); f++, shared++) assert.equal(sourceAt(x, f), sourceAt(y, f), a + ' and ' + b + ' at frame ' + f);
  assert.ok(shared > 0, a + ' and ' + b + ' play together');
}
const timing = (p, id) => [at(p,id).start, at(p,id).duration, at(p,id).offset];
const reversedAsset = frames => ({id:'r', name:'Clip (reversed).mp4', kind:'video', has_audio:true, duration_frames:frames});
// J-cut pair: the video [50,150) reads [100,200), its audio partner [30,150) was trimmed 20 frames earlier and reads [80,200).
const jcut = () => project([track('v1','video',[clip('x',V,50,100,{offset:100,link:'L',audio_detached:true})]), track('a1','audio',[clip('ax',V,30,120,{offset:80,link:'L'})])]);
let p = jcut();
valid(p); inSync(p, 'x', 'ax');
assert.deepEqual(E.reverseRange(p, 'x'), {rangeStart:80, frames:120});
assert.deepEqual(E.reverseRange(p, 'ax'), {rangeStart:80, frames:120}, 'either member gives the same range');
p.assets.push(reversedAsset(120));
let before = copy(p);
assert.equal(E.reverseClips(p, 'x', {assetId:'r', rangeStart:80, frames:120, sourceAssetId:'v'}), true);
assert.deepEqual(timing(p,'x'), [50,100,0], 'the reversed clip keeps its place');
assert.deepEqual(timing(p,'ax'), [50,120,0], 'the J-cut sound is mirrored around the clip and becomes an L-cut');
inSync(p, 'x', 'ax'); mirrored(before, p, 'x', {rangeStart:80, frames:120});
valid(p);
// Reversed from the audio clip, the group mirrors around that clip: the video moves instead.
p = copy(before);
assert.equal(E.reverseClips(p, 'ax', {assetId:'r', rangeStart:80, frames:120, sourceAssetId:'v'}), true);
assert.deepEqual(timing(p,'ax'), [30,120,0]); assert.deepEqual(timing(p,'x'), [30,100,0]);
inSync(p, 'x', 'ax'); mirrored(before, p, 'ax', {rangeStart:80, frames:120});
valid(p);
// L-cut pair: the audio [50,180) runs 30 frames past the video ([100,230) against [100,200)) and becomes a J-cut.
p = project([track('v1','video',[clip('x',V,50,100,{offset:100,link:'L',audio_detached:true})]), track('a1','audio',[clip('ax',V,50,130,{offset:100,link:'L'})])]);
assert.deepEqual(E.reverseRange(p, 'x'), {rangeStart:100, frames:130});
p.assets.push(reversedAsset(130));
before = copy(p);
assert.equal(E.reverseClips(p, 'x', {assetId:'r', rangeStart:100, frames:130, sourceAssetId:'v'}), true);
assert.deepEqual(timing(p,'x'), [50,100,30]); assert.deepEqual(timing(p,'ax'), [20,130,0]);
inSync(p, 'x', 'ax'); mirrored(before, p, 'x', {rangeStart:100, frames:130});
valid(p);
// A J-cut at 2× stays in sync too (the same floor rule as the renderer).
p = project([track('v1','video',[clip('x',V,50,50,{offset:100,speed:2,link:'L',audio_detached:true})]), track('a1','audio',[clip('ax',V,40,60,{offset:80,speed:2,link:'L'})])]);
valid(p); inSync(p, 'x', 'ax');
assert.deepEqual(E.reverseRange(p, 'x'), {rangeStart:80, frames:119});
p.assets.push(reversedAsset(119));
before = copy(p);
assert.equal(E.reverseClips(p, 'x', {assetId:'r', rangeStart:80, frames:119, sourceAssetId:'v'}), true);
assert.deepEqual(timing(p,'ax'), [50,60,0]);
inSync(p, 'x', 'ax'); mirrored(before, p, 'x', {rangeStart:80, frames:119});
valid(p);
// No room: another audio clip right after the video blocks the mirrored sound; nothing changes.
p = jcut(); p.tracks[1].clips.push(clip('n',A,150,50)); p.assets.push(reversedAsset(120));
valid(p);
before = copy(p);
assert.equal(E.reverseClips(p, 'x', {assetId:'r', rangeStart:80, frames:120, sourceAssetId:'v'}), 'room');
assert.deepEqual(p, before, 'a refused reverse changes nothing');
// A partner mirrored before the project start refuses as well.
p = project([track('v1','video',[clip('x',V,10,100,{offset:100,link:'L',audio_detached:true})]), track('a1','audio',[clip('ax',V,10,190,{offset:100,link:'L'})])]);
p.assets.push(reversedAsset(190));
before = copy(p);
assert.equal(E.reverseClips(p, 'x', {assetId:'r', rangeStart:100, frames:190, sourceAssetId:'v'}), 'room');
assert.deepEqual(p, before);
// A clip that no longer reads the reversed source refuses; so does a member the range does not cover.
p = project([track('v1','video',[clip('x',V,50,100,{offset:100})])]);
p.assets.push({id:'r', name:'r.mp4', kind:'video', has_audio:true, duration_frames:100});
assert.equal(E.reverseClips(p, 'x', {assetId:'r', rangeStart:100, frames:100, sourceAssetId:'other'}), 'media');
assert.equal(E.reverseClips(p, 'x', {assetId:'r', rangeStart:110, frames:90, sourceAssetId:'v'}), 'media');
// Speed: a 2× clip of 50 frames reads source frames 100..198, 99 of them.
p = project([track('v1','video',[clip('x',V,0,50,{offset:100,speed:2})])]);
assert.deepEqual(E.reverseRange(p, 'x'), {rangeStart:100, frames:99});
// At 0.5× a 4-frame clip shows source frames 100, 100, 101, 101; the range also holds 102, so the mirrored clip fits.
p = project([track('v1','video',[clip('x',V,0,4,{offset:100,speed:0.5})])]);
assert.deepEqual(E.reverseRange(p, 'x'), {rangeStart:100, frames:3});
p.assets.push(reversedAsset(3));
assert.equal(E.reverseClips(p, 'x', {assetId:'r', rangeStart:100, frames:3, sourceAssetId:'v'}), true);
valid(p);

// Freeze: the request remembers the asset and source frame (offset + (point - start) × speed, rounded down).
const still = {id:'s', name:'s.png', kind:'image', duration_frames:0, width:64, height:64};
const fresh = () => { const q = project([track('v1','video',[clip('x',V,0,100,{offset:30})])]); q.assets.push(still); return q; };
p = fresh();
assert.equal(E.sourceFrameAt(p, at(p,'x'), 40), 70);
const captured = {asset_id:'s', clip_id:'still', duration:60, source_asset_id:'v', source_frame:70};
let q = fresh(); assert.equal(E.slip(q, ['x'], 5), true);
assert.equal(E.freezeFrame(q, 'x', 40, captured), 'media', 'a slipped clip shows another frame at the cut');
q = fresh(); at(q,'x').start = 10;
assert.equal(E.freezeFrame(q, 'x', 40, captured), 'media', 'a moved clip shows another frame at the cut');
q = fresh(); at(q,'x').speed = 2;
assert.equal(E.freezeFrame(q, 'x', 40, captured), 'media', 'a re-timed clip shows another frame at the cut');
q = fresh(); q.assets.push({id:'r', name:'r.mp4', kind:'video', has_audio:true, duration_frames:600}); at(q,'x').asset_id = 'r';
assert.equal(E.freezeFrame(q, 'x', 40, captured), 'media', 'a reversed clip reads another asset');
q = fresh(); Object.assign(at(q,'x'), {start:10, offset:40});
assert.equal(E.freezeFrame(q, 'x', 40, captured), true, 'a moved clip that still shows the frame at the cut takes the still');
valid(q);
q = fresh();
assert.equal(E.freezeFrame(q, 'x', 40, captured), true);
assert.deepEqual(spans(q,'v1'), [[0,40,30],[40,60,0],[100,60,70]]);
valid(q);
`)
}
