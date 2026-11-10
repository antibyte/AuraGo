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
