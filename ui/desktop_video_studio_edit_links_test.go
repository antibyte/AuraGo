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
