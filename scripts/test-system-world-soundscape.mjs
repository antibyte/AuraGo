// Phase 5 soundscape: injection, the limiter chain, stored levels, gesture and owner rules (Task 1); zones, glass,
// gating, beds by zone and visitor steps (Task 10); layers, tiers and the voice and node budget (Task 11).
import assert from 'node:assert/strict';
import {createFakeAudio} from './system-world-audio-fake.mjs';
import {createCityAmbience,LIMITER,ceilingCurve,citySoundscapeEvents} from '../ui/js/desktop/apps/sysworld-audio.js';

const seeded=seed=>{let s=seed>>>0;return()=>(s=(Math.imul(s,1664525)+1013904223)>>>0)/4294967296;};
const memory=initial=>{const m=new Map(Object.entries(initial||{}));return {getItem:k=>m.has(k)?m.get(k):null,setItem:(k,v)=>m.set(k,String(v))};};
function city(extra={}){const fake=createFakeAudio(),storage=extra.storage||memory();
  const sound=createCityAmbience(()=>{},{context:fake.context,timers:fake.timers,random:seeded(5),voice:false,...extra,storage});return {fake,sound,storage};}
const frameOf=(o={})=>({time:0,seaTime:0,wind:{x:1,z:0,strength:.4},trams:[{x:0,z:0,heading:0,on:false},{x:0,z:0,heading:0,on:false}],
  companion:{x:0,z:0,phase:0,on:false},doorValue:()=>0,reduced:false,...o});
const ramps=fake=>fake.nodes.reduce((n,x)=>n+x.gain.ramp.length+x.frequency.ramp.length+x.pan.ramp.length+x.detune.ramp.length,0);

// --- Task 1: limiter, injection, stored levels ---------------------------------------------------------------------
{
  const curve=ceilingCurve();assert.ok(curve.every(v=>Math.abs(v)<=LIMITER.ceiling),'the ceiling never passes -1 dBFS');
  for(let i=0;i<curve.length;i++){const x=i*2/(curve.length-1)-1;if(Math.abs(x)<=LIMITER.linear)assert.ok(Math.abs(curve[i]-x)<1e-6,'linear below '+LIMITER.linear);if(i)assert.ok(curve[i]>=curve[i-1],'monotonic');}
  const {fake,sound}=city({storage:memory({'aurago.desktop.sysworld.sound':'true'})});
  assert.equal(sound.stats().state,'uninitialized','nothing before a gesture, even when switched on');
  assert.equal(fake.nodes.length,1,'no node before the gesture (the fake destination only)');
  sound.unlock();sound.setActive(true);
  const compressor=fake.nodes.find(n=>n.kind==='compressor');assert.ok(compressor,'a compressor guards the master');
  assert.deepEqual([compressor.threshold.value,compressor.knee.value,compressor.ratio.value,compressor.attack.value,compressor.release.value],
    [LIMITER.threshold,LIMITER.knee,LIMITER.ratio,LIMITER.attack,LIMITER.release]);
  const trim=compressor.links[0];assert.equal(trim.gain.value,LIMITER.trim,'the trim cancels the makeup gain');
  const shaper=trim.links[0];assert.equal(shaper.kind,'shaper');assert.equal(shaper.oversample,'4x');
  const analyser=fake.nodes.find(n=>n.kind==='analyser');assert.ok(shaper.links.includes(analyser)&&analyser.links.includes(fake.context.destination),'compressor -> trim -> ceiling -> analyser -> destination');
  const master=fake.nodes.find(n=>n.kind==='gain'&&n.links.includes(compressor));assert.ok(master&&master.links.length===1,'the master feeds only the limiter');
  assert.deepEqual(fake.nodes.filter(n=>n.links.includes(fake.context.destination)),[analyser],'nothing bypasses the limiter');
  assert.ok(sound.stats().limiter.trim===LIMITER.trim&&sound.stats().graph.nodes>20);
  sound.dispose();
}
// Stored levels: written on change, read at creation, clamped.
{
  const storage=memory(),{sound}=city({storage});sound.setVolume(.3);sound.setChannel('effects',.4);
  assert.equal(storage.getItem('aurago.desktop.sysworld.volume'),'0.3');
  assert.deepEqual(JSON.parse(storage.getItem('aurago.desktop.sysworld.channels')),{ambience:1,effects:.4,voice:1});
  const again=city({storage}).sound;assert.equal(again.stats().volume,.3);assert.equal(again.stats().channels.effects,.4);
  const bad=city({storage:memory({'aurago.desktop.sysworld.volume':'9','aurago.desktop.sysworld.channels':'{"voice":-2,"ambience":"x"}'})}).sound;
  assert.equal(bad.stats().volume,.35,'volume clamps to its maximum');assert.equal(bad.stats().channels.voice,0);assert.equal(bad.stats().channels.ambience,1);
  sound.dispose();again.dispose();bad.dispose();
}
// Injected context and timers: the owner closes the context; a muted city does no per-frame work.
{
  const {fake,sound}=city({enabled:true});let closed=false;fake.context.close=()=>{closed=true;return Promise.resolve();};
  sound.unlock();sound.setActive(true);assert.ok(fake.tasks.size>=2,'events and chords run on the injected timers');
  sound.setActive(false);assert.equal(sound.stats().soundscape.scheduled,false);
  const before=ramps(fake);
  for(let i=0;i<30;i++){sound.setListener(0,2.4,76,0,-1,0,76);sound.frame(frameOf());}
  assert.equal(ramps(fake),before,'a muted city schedules nothing per frame');
  sound.dispose();assert.equal(closed,false,'an injected context belongs to its owner');
}
// Determinism: the same seed builds the same noise.
{
  const a=city({enabled:true}),b=city({enabled:true});a.sound.unlock();b.sound.unlock();
  const buf=f=>f.nodes.find(n=>n.kind==='source'&&n.buffer)?.buffer.getChannelData(0);assert.deepEqual(buf(a.fake),buf(b.fake));
  a.sound.dispose();b.sound.dispose();
}
console.log('System World soundscape: limiter chain, stored levels, gesture and owner rules, deterministic injection verified');

// --- Task 10: zones, glass, gating, beds by zone, visitor steps ---------------------------------------------------
{
  const {fake,sound}=city({enabled:true});sound.unlock();sound.setActive(true);
  const run=(x,y,z,fx,fz,f,seconds=2)=>{for(let i=0;i<seconds*60;i++){fake.context.currentTime+=1/60;sound.setListener(x,y,z,fx,fz);sound.frame(f);}};
  run(-13.5,2.4,-12,0,-1,frameOf());let s=sound.stats().soundscape;
  assert.ok(s.zones.canyon>.5&&s.space.glass===0,'street: canyon, no glass '+JSON.stringify(s.zones));
  run(0,3.28,-12,0,1,frameOf());s=sound.stats().soundscape;
  assert.equal(s.zones.room,'lobby:agent');assert.ok(s.space.glass>.95&&s.space.cutoff<1000,'shut lobby '+JSON.stringify(s.space));
  const inside=citySoundscapeEvents({day:1,weather:'clear'},s.zones).map(e=>e.kind);
  assert.ok(inside.length&&inside.every(k=>['chime','horn','bell','siren'].includes(k)),'only glass-passing events indoors: '+inside);
  run(0,3.28,-12,0,1,frameOf({doorValue:()=>1}));s=sound.stats().soundscape;assert.ok(s.space.glass>.2&&s.space.glass<.4,'open doors let the city in '+s.space.glass);
  run(-43,3.28,36,0,1,frameOf());s=sound.stats().soundscape;assert.equal(s.zones.open,true);
  assert.ok(citySoundscapeEvents({day:1,weather:'clear'},s.zones).some(e=>e.kind==='birds'),'the open court keeps its birds');
  run(3,2.4,-1,0,-1,frameOf({doorValue:k=>k==='lobby:agent:0'?1:0}));s=sound.stats().soundscape;
  assert.ok(s.zones.leakRoom==='lobby:agent'&&s.zones.leak>.5&&s.zones.leakPan<0,'the open door on the left leaks');
  run(-20,2.48,77,0,1,frameOf());assert.ok(sound.stats().soundscape.beds.water>.04,'water by the quay proximity');
  sound.setListener(2,2.4,0,0,-1);   // the visitor stands where the step happens (a step beyond the foley's 48 m reach is dropped)
  const played=sound.stats().foley.played;sound.effect('step',2,2.4,0);
  assert.equal(sound.stats().foley.played,played+1,'the visitor step is a foley voice');assert.equal(sound.stats().foley.bySurface.stairs,1,'on the agent stairs');
  assert.deepEqual(citySoundscapeEvents({day:1,weather:'clear'}),citySoundscapeEvents({day:1,weather:'clear'},null),'without zones the table is unchanged');
  sound.dispose();
}
console.log('System World soundscape: zones, glass, indoor gate, open court, door leak, beds by zone and visitor steps verified');

// --- Task 11: layers, tiers and the budget -------------------------------------------------------------------------
const budget={};
for(const tier of['low','medium','high']){
  const {fake,sound}=city({enabled:true,tier});sound.unlock();sound.setActive(true);
  const trams=[{x:-18,z:-40,heading:0,on:true},{x:-18,z:20,heading:Math.PI,on:true}];
  for(let i=0;i<180;i++){fake.context.currentTime+=1/60;for(const t of trams)t.z+=(t.heading?-1:1)*11/60;
    for(let k=0;k<12;k++)sound.gait('patrol-'+k,-13.5+(k%2?1:-1)*(2+k*3),1.6,-12,true,false,'glide');
    sound.setListener(-13.5,2.4,-12,0,-1);sound.frame(frameOf({trams}));fake.flush();}
  const st=sound.stats(),live=fake.live();
  budget[tier]={nodes:st.graph.nodes,live:live.length,sources:live.filter(n=>n.kind==='source'||n.kind==='oscillator').length,reverbs:live.filter(n=>n.kind==='reverb').length,
    hovers:st.soundscape.layers.movers.hovers.length,trams:st.soundscape.layers.movers.trams.filter(t=>t.voice).length,foley:st.foley.voices,events:st.soundscape.transient};
  assert.ok(budget[tier].foley<=(tier==='low'?6:10)&&budget[tier].events<=12);
  sound.dispose();assert.equal(fake.live().filter(n=>n.kind!=='destination').length,0,tier+': dispose releases every node');
}
assert.deepEqual([budget.low.hovers,budget.medium.hovers,budget.high.hovers],[2,4,4],'hover voices per tier');
assert.deepEqual([budget.low.trams,budget.high.trams],[1,2],'tram voices per tier');
assert.equal(budget.low.reverbs,1,'low: the open-air verb only');assert.ok(budget.medium.reverbs<=2&&budget.high.reverbs<=3);
assert.ok(budget.low.live<budget.high.live&&budget.high.live<=320&&budget.low.live<=220,'node budget '+JSON.stringify(budget));
console.table(budget);
console.log('System World soundscape: layers, tiers, voice caps and node budget verified');
