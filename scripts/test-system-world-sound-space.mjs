// Phase 5 spaces: glass, send and room laws; impulses; the graph per tier; swaps only at a silent send; no automation
// while nothing changes; dispose.
import assert from 'node:assert/strict';
import {createFakeAudio} from './system-world-audio-fake.mjs';
import * as S from '../ui/js/desktop/apps/sysworld-sound-space.js';
const open=S.glassLaw(0,false,0);assert.deepEqual([open.amount,open.cutoff,open.gain],[0,S.GLASS.open,1]);
const shut=S.glassLaw(1,false,0);assert.ok(Math.abs(shut.cutoff-900)<1e-6&&Math.abs(20*Math.log10(shut.gain)+12)<1e-9,'shut glass: 900 Hz at -12 dB');
const ajar=S.glassLaw(1,false,1);assert.ok(ajar.amount>.25&&ajar.amount<.35&&ajar.cutoff>5000,'an open door lets the city in');
assert.equal(S.glassLaw(1,true,0).amount,0,'the open court has no glass');
let prev=-1;for(let i=0;i<=20;i++){const a=S.glassLaw(i/20,false,0).amount;assert.ok(a>=prev);prev=a;}
const sl=S.sendLaw({canyon:.3,interior:.4,space:'hall'});assert.ok(Math.abs(sl.open+sl.canyon-1)<1e-12&&Math.abs(sl.interior+sl.outdoor-1)<1e-12&&sl.interior===.4);
assert.equal(S.sendLaw({canyon:0,interior:1,space:null}).interior,0,'no room space, no interior send');
assert.equal(S.roomLaw(1,0),1);assert.equal(S.roomLaw(0,0),0);assert.ok(Math.abs(S.roomLaw(0,1)-S.RETURNS.leak)<1e-12);
const energy=a=>a.reduce((s,v)=>s+v*v,0);
for(const name of Object.keys(S.ROOM_SPACES)){const [l,r]=S.impulseRoom(name,8000),[l2]=S.impulseRoom(name,8000),sp=S.ROOM_SPACES[name];
  assert.equal(l.length,Math.ceil(8000*sp.seconds));assert.deepEqual(l,l2,name+' deterministic');assert.ok(l.every(Number.isFinite));
  assert.ok(Math.abs(energy(l)-.25)<1e-4&&Math.abs(energy(r)-.25)<1e-4,name+' equal energy');assert.ok(l.slice(0,Math.round(8000*sp.predelay)).every(v=>v===0),name+' predelay');}
const [cl]=S.impulseCanyon(48000);for(const [t]of S.CANYON_TAPS){const i=Math.round(t*48000);assert.ok(cl.slice(i,i+24).reduce((m,v)=>Math.max(m,Math.abs(v)),0)>.01,'tap at '+t);}
const fake=createFakeAudio(),ambience=fake.node('ambience'),verb=fake.node('verb-open'),space=S.createSoundSpace(fake.context,{ambience,verb,tier:'high'});
const reaches=(from,to)=>fake.downstream(from).has(to),reverbs=()=>fake.live('reverb').length;
assert.ok(reaches(space.outdoor,ambience)&&reaches(space.sendOutdoor,verb),'outdoor reaches the ambience through the glass; its send reaches the open air');
assert.equal(reverbs(),1,'high: the canyon at start; the interior loads with the first room');
const w={harbour:0,canyon:0,green:0,plaza:1,height:0,quay:0,interior:0,room:null,space:null,tone:null,open:false,door:0,leak:0,leakRoom:null,leakTone:null,leakSpace:null,leakPan:0};
const ramps=()=>fake.nodes.reduce((n,x)=>n+x.gain.ramp.length+x.frequency.ramp.length+x.pan.ramp.length,0);
space.update(w,1/60);const r0=ramps();for(let i=0;i<100;i++)space.update(w,1/60);assert.equal(ramps(),r0,'unchanged weights schedule nothing');
Object.assign(w,{interior:1,room:'lobby:agent',space:'lobby',tone:'agent'});space.update(w,1/60);
assert.equal(space.stats().interior,'lobby');assert.equal(reverbs(),2);assert.ok(reaches(space.sendLocal,space.room)&&reaches(space.roomSend,space.room),'local and room sends reach the room bus');
Object.assign(w,{space:'machine',room:'lobby:infra'});space.update(w,1/60);assert.equal(space.stats().interior,'lobby','no swap at full send');
w.interior=.01;space.update(w,1/60);assert.equal(space.stats().interior,'machine');assert.equal(reverbs(),2,'the old interior convolver is released');
space.setTier('low');assert.equal(reverbs(),0,'low: no extra convolver');assert.ok(reaches(space.sendLocal,verb)&&reaches(space.roomSend,verb),'low: room and local sends use the open-air verb');
space.setTier('medium');space.update(w,1/60);assert.equal(reverbs(),1,'medium: interior only');assert.equal(space.stats().canyon,false);
space.dispose();assert.equal(space.stats().nodes,0);
console.log('System World sound spaces: glass, sends, room bus, impulses, tiers and swaps verified');
