// Phase 5 moving sources: laws, nearest slots, a tram pass (reach, Doppler, clacks, release), a curve's squeal, hover
// voices per tier, boats in reach, gull positions, no automation while nothing moves, dispose.
import assert from 'node:assert/strict';
import {createFakeAudio} from './system-world-audio-fake.mjs';
import * as M from '../ui/js/desktop/apps/sysworld-sound-movers.js';
import {HARBOUR_SKY} from '../ui/js/desktop/apps/sysworld-gulls.js';
assert.equal(M.distanceGain(3,6,1),1);let prev=2;for(let d=6;d<200;d+=7){const g=M.distanceGain(d,6,1);assert.ok(g<=prev&&g>0);prev=g;}
assert.ok(M.dopplerRatio(50,49,.1)>1&&M.dopplerRatio(49,50,.1)<1,'approaching sounds higher');assert.equal(M.dopplerRatio(0,1000,.1),.94);assert.equal(M.dopplerRatio(NaN,5,.1),1);
assert.equal(M.tramLaw(0,0,5,{}).hum,0);assert.equal(M.tramLaw(11,0,5,{}).hum,M.MOVERS.tram);assert.equal(M.tramLaw(11,4,5,{}).squeal,M.MOVERS.squeal);
assert.equal(M.tramLaw(11,0,95,{}).hum,0,'beyond reach');assert.equal(M.tramLaw(11,.5,5,{}).squeal,0,'a straight run does not squeal');
const items=[{x:10,z:0,on:true},{x:2,z:0,on:true},{x:5,z:0,on:false},{x:40,z:0,on:true},{x:3,z:0,on:true}],out=new Int32Array(3);
assert.deepEqual([...M.nearestSlots(items,3,0,0,30,out)],[1,4,0]);assert.deepEqual([...M.nearestSlots(items,3,0,0,4,out)],[1,4,-1]);
function rig(tier='high'){const fake=createFakeAudio(),c=fake.context,outdoor=fake.node('outdoor'),send=fake.node('send');return {fake,c,movers:M.createSoundMovers(c,{outdoor,send,noise:c.createBuffer(1,c.sampleRate*2),tier})};}
const L=(x,z,fx=0,fz=-1)=>({x,y:2.4,z,fx,fz,origin:{x,z}}),off={x:0,z:0,heading:0,on:false},frame=(trams=[],seaTime=0)=>({seaTime,trams:[trams[0]||off,trams[1]||off]});
{ // A tram passes along x at 11 m/s, 2 m from the listener.
  const {fake,c,movers}=rig(),tram={x:-150,z:2,heading:Math.PI/2,on:true};let built=null,approach=0,recede=0,maxClacks=0;
  const motor=()=>fake.nodes.find(n=>n.kind==='oscillator'&&n.type==='sawtooth'&&!n.disconnected&&n.frequency.ramp.length>0);
  for(let k=0;k<30*60;k++){const t=k/60;c.currentTime=t;tram.x=-150+11*t;movers.update(frame([tram]),L(0,0),t,1/60);
    const s=movers.stats().trams[0];if(s.voice&&built===null)built=tram.x;if(Math.abs(tram.x+30)<.1)approach=motor().frequency.value;if(Math.abs(tram.x-30)<.1)recede=motor().frequency.value;
    maxClacks=Math.max(maxClacks,movers.stats().clacks);if(k%6===0)fake.flush();}
  assert.ok(built>-91&&built<-85,'the voice starts within 90 m: '+built);assert.ok(approach>recede&&approach>92&&recede<92,'Doppler '+approach+' / '+recede);
  assert.ok(movers.stats().clacksPlayed>4&&maxClacks<=M.MOVERS.maxClacks,'rail joints');assert.equal(movers.stats().trams[0].voice,false,'released after it left');
  assert.equal(movers.stats().trams[0].squeal,0);movers.dispose();
}
{ // A curve squeals.
  const {c,movers}=rig(),tram={x:0,z:0,heading:0,on:true};let squeal=0;
  for(let k=0;k<6*60;k++){const t=k/60,a=t*8/5.3;c.currentTime=t;tram.x=20+5.3*Math.cos(a);tram.z=5.3*Math.sin(a);tram.heading=Math.atan2(-Math.sin(a),Math.cos(a));
    movers.update(frame([tram]),L(10,0),t,1/60);squeal=Math.max(squeal,movers.stats().trams[0].squeal);}
  assert.ok(squeal>.5*M.MOVERS.squeal,'a 5.3 m curve at 8 m/s squeals: '+squeal);movers.dispose();
}
for(const [tier,n]of[['low',2],['medium',4],['high',4]]){ // The nearest hover robots win a voice.
  const {c,movers}=rig(tier);for(let i=0;i<12;i++)movers.gait('patrol-'+i,2+i*3,1.6,0,true,'glide');movers.gait('resident-0',1,0,0,true,'');
  for(let k=0;k<30;k++){c.currentTime=k/60;movers.update(frame(),L(0,0),k/60,1/60);}
  assert.deepEqual(movers.stats().hovers,['patrol-0','patrol-1','patrol-2','patrol-3'].slice(0,n),tier);assert.equal(movers.stats().gliders,12,'walking residents are not gliders');movers.dispose();
}
{ // Boats.
  const high=rig('high');high.movers.update(frame([],0),L(-20,77,0,1),0,1/60);assert.deepEqual([high.movers.stats().launch,high.movers.stats().barge],[true,true]);high.movers.dispose();
  const low=rig('low');low.movers.update(frame([],0),L(-20,77,0,1),0,1/60);assert.equal(low.movers.stats().barge,false,'low: no barge');
  const inland=rig('high');inland.movers.update(frame([],0),L(0,-60),0,1/60);assert.equal(inland.movers.stats().launch,false,'inland: no engines');
}
{ // Gulls call from the flock.
  const {movers}=rig('high');let s=1;const r=()=>(s=(Math.imul(s,1664525)+1013904223)>>>0)/4294967296,spot={};
  for(let i=0;i<50;i++){const p=movers.gull({seaTime:i*3},r,spot);assert.ok(p===spot&&p.y>=11.5&&Math.hypot(p.x-HARBOUR_SKY.x,p.z-HARBOUR_SKY.z)<300,'over the harbour or the barge');}
  assert.equal(rig('low').movers.gull({seaTime:0},r,spot),null);
}
{ // Nothing moves: nothing is scheduled; dispose releases everything.
  const {fake,c,movers}=rig(),tram={x:-30,z:2,heading:0,on:true};for(let k=0;k<120;k++){c.currentTime=k/60;movers.update(frame([tram]),L(0,0),k/60,1/60);}
  const ramps=()=>fake.nodes.reduce((n,x)=>n+x.gain.ramp.length+x.frequency.ramp.length+x.pan.ramp.length,0),r0=ramps();
  for(let k=120;k<240;k++){c.currentTime=k/60;movers.update(frame([tram]),L(0,0),k/60,1/60);}assert.equal(ramps(),r0,'a standing tram and listener schedule nothing');
  movers.dispose();assert.equal(fake.live().filter(n=>!['destination','outdoor','send'].includes(n.kind)).length,0);
}
console.log('System World sound movers: tram pass with Doppler, clacks and release, curve squeal, hover voices per tier, boats and gulls verified');
