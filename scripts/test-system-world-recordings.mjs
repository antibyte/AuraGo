// Phase 5 recordings layer: level laws, lazy single load, failure isolation, two offset sources per recording, routing,
// the low tier, the engine wiring (events thin out, stats) and dispose; plus the shipped audio assets against their manifest.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import crypto from 'node:crypto';
import {fileURLToPath} from 'node:url';
import {createFakeAudio} from './system-world-audio-fake.mjs';
import * as R from '../ui/js/desktop/apps/sysworld-sound-recordings.js';
import {createCityAmbience} from '../ui/js/desktop/apps/sysworld-audio.js';
const zone=o=>({harbour:0,canyon:0,green:0,plaza:0,height:0,interior:0,open:false,quay:0,...o});
const mood=o=>({day:1,weather:'clear',busy:false,...o}),wind=s=>({wind:{strength:s}});
const near=(a,b,m)=>assert.ok(Math.abs(a-b)<1e-9,m+': '+a+' vs '+b);
const G=R.RECORDINGS,gw=G.water.gain,gg=G.gulls.gain,gc=G.city.gain,gr=G.rain.gain;
assert.deepEqual(Object.keys(G),['water','leaves','birds','gulls','city','rain']);
assert.deepEqual(Object.fromEntries(Object.entries(G).map(([k,v])=>[k,v.gain])),{water:1.95,leaves:1.2,birds:.9,gulls:.9,city:1.5,rain:1.25});
// --- level laws (the level includes the recording's gain) ---
near(R.recordingLevel('water',zone({quay:1}),wind(0),mood()),gw,'quay');near(R.recordingLevel('water',zone({quay:.5}),wind(0),mood()),gw*Math.pow(.5,1.3),'quay^1.3');
assert.equal(R.recordingLevel('water',zone({plaza:1}),wind(0),mood()),0,'no water on the plaza');
near(R.recordingLevel('leaves',zone({green:1}),wind(0),mood()),1.2*.25,'park, no wind');near(R.recordingLevel('leaves',zone({green:1}),wind(1),mood()),1.2,'park, full wind');
near(R.recordingLevel('leaves',zone({green:.5}),wind(.5),mood()),1.2*.5*(.25+.75*.25),'park in wind');
near(R.recordingLevel('birds',zone({green:1}),wind(0),mood()),.9,'birds by day');near(R.recordingLevel('birds',zone({green:.2,plaza:1}),wind(0),mood()),.9*.55,'plaza adds .35');
assert.equal(R.recordingLevel('birds',zone({green:1}),wind(0),mood({day:0})),0,'no birds at night');assert.equal(R.recordingLevel('birds',zone({green:1}),wind(0),mood({weather:'rain'})),0,'none in rain');
near(R.recordingLevel('birds',zone({green:1}),wind(0),mood({weather:'fog'})),.9*.6,'fog');
near(R.recordingLevel('gulls',zone({harbour:1}),wind(0),mood()),gg,'gulls by day');near(R.recordingLevel('gulls',zone({harbour:1}),wind(0),mood({day:0})),gg*.35,'gulls at night');
near(R.recordingLevel('gulls',zone({harbour:1}),wind(0),mood({weather:'rain'})),gg*.4,'gulls in rain');
// street by day, canyon share 1 (never above the cap), a busy open plaza, a roof
near(R.recordingLevel('city',zone({canyon:1}),wind(0),mood()),gc,'street by day');near(R.recordingLevel('city',zone({}),wind(0),mood({day:0})),.35*.7*gc,'empty night');
near(R.recordingLevel('city',zone({canyon:1}),wind(0),mood({busy:true})),1.25*gc,'busy');near(R.recordingLevel('city',zone({plaza:1}),wind(0),mood()),(.35+.65*.6)*gc,'plaza');
near(R.recordingLevel('city',zone({height:1}),wind(0),mood()),(.35+.65*.5)*gc,'roof');
// rain at night in the street, on a roof, and no rain when dry
near(R.recordingLevel('rain',zone({canyon:1}),wind(0),mood({day:0,weather:'rain'})),gr,'rain at night');near(R.recordingLevel('rain',zone({height:1}),wind(0),mood({weather:'rain'})),gr*.5,'rain on a roof');
assert.equal(R.recordingLevel('rain',zone({canyon:1}),wind(0),mood()),0);assert.equal(R.recordingLevel('nothing',zone({}),wind(0),mood()),0);
assert.equal(R.recordingLevel('leaves',zone({green:1}),null,mood()),1.2*.25,'a missing frame means no wind');

// --- the layer ---
const bytes=new ArrayBuffer(8);
function rig(opts={}){
  const fake=createFakeAudio(),c=fake.context,outdoor=fake.node('outdoor'),send=fake.node('send'),calls=[];
  const manifest=new TextEncoder().encode(JSON.stringify({recordings:R.IDS.map(id=>({id,file:id+'.ogg'}))})).buffer;
  const load=async path=>{calls.push(path);if(opts.fail?.includes(path))throw Error('missing '+path);return path==='manifest.json'?manifest:bytes;};
  if(opts.decodeFail)c.decodeAudioData=async()=>{throw Error('no vorbis');};else c.decodeAudioData=async()=>({duration:10,numberOfChannels:1,sampleRate:8000,getChannelData:()=>new Float32Array(4)});
  return {fake,c,outdoor,send,calls,layer:R.createSoundRecordings(c,{outdoor,send,load,tier:opts.tier||'high'})};
}
const settle=()=>new Promise(r=>setTimeout(r,5));
const W=zone({quay:1,harbour:1,green:.5,canyon:.3});
{
  const {fake,c,outdoor,send,calls,layer}=rig();
  assert.equal(calls.length,0,'nothing loads at creation');layer.setMood(mood());
  layer.update(wind(.3),W);layer.update(wind(.3),W);await settle();
  assert.deepEqual(calls,['manifest.json',...R.IDS.map(id=>id+'.ogg')],'one manifest and one load per file');
  for(let i=0;i<5;i++)layer.update(wind(.3),W);await settle();assert.equal(calls.length,7,'never reloaded or retried');
  assert.equal(layer.stats().failed,0);assert.deepEqual(layer.stats().loaded.slice().sort(),[...R.IDS].sort());
  const sources=fake.live('source');assert.equal(sources.length,12,'two per recording');
  const water=layer.stats().loaded.indexOf('water');assert.ok(water>=0);
  // find the water voice's level gain: the gain whose links hold two chains and which feeds outdoor
  const levelGains=fake.nodes.filter(n=>n.kind==='gain'&&n.links.includes(outdoor));assert.equal(levelGains.length,6,'six level gains feed the outdoor bus');
  for(const lg of levelGains)assert.equal(fake.live('pan').filter(p=>p.links.some(n=>n===lg)).length,2,'two panners per recording');
  const starts=sources.map(s=>s.startArgs);assert.equal(starts.filter(a=>a[1]===0).length,6);assert.equal(starts.filter(a=>a[1]===5).length,6,'the second source starts half a loop (10 s / 2) in');
  assert.ok(sources.every(s=>s.loop===true&&s.buffer.duration===10));
  const pans=fake.nodes.filter(n=>n.kind==='pan').map(n=>n.pan.value).sort();assert.deepEqual([pans[0],pans[11]],[-.6,.6]);
  assert.ok(fake.nodes.filter(n=>n.kind==='gain'&&n.gain.value===.707).length===12);
  // sends: only birds and gulls reach the reverb send, at .2
  const sent=levelGains.filter(lg=>fake.downstream(lg).has(send));assert.equal(sent.length,2,'birds and gulls only');
  assert.ok(fake.nodes.filter(n=>n.kind==='gain'&&n.links.includes(send)).every(n=>n.gain.value===.2));
  // levels: set once per quantized change, none while nothing changes
  c.currentTime=1;layer.update(wind(.3),W);const count=()=>fake.nodes.reduce((n,x)=>n+x.gain.ramp.length,0),before=count();
  for(let i=0;i<200;i++)layer.update(wind(.3),W);assert.equal(count(),before,'unchanged input schedules nothing');
  const wl=layer.stats().levels;near(wl.water,+(R.recordingLevel('water',W,wind(.3),mood())).toFixed(3),'water level in stats');
  layer.update(wind(.3),zone({}));const t=levelGains.map(lg=>lg.gain.ramp.at(-1));assert.ok(t.every(r=>r[0]==='target'&&r[3]===R.LEVEL_TAU),'time constant '+R.LEVEL_TAU);
  assert.equal(layer.stats().levels.water,0,'a recording at level 0 keeps running');assert.equal(fake.live('source').length,12);assert.ok(fake.live('source').every(s=>!s.stopped));
  layer.dispose();assert.equal(fake.live().filter(n=>!['destination','outdoor','send'].includes(n.kind)).length,0,'dispose releases everything');
  assert.ok(sources.every(s=>s.stopped),'dispose stops every source');
}
{ // A missing file and a failing decode leave the rest working; no retry.
  const {fake,calls,layer}=rig({fail:['birds.ogg']});layer.update(wind(0),W);await settle();
  assert.equal(layer.stats().failed,1);assert.ok(!layer.stats().loaded.includes('birds')&&layer.stats().loaded.length===5);assert.equal(fake.live('source').length,10);
  layer.update(wind(0),W);await settle();assert.equal(calls.filter(p=>p==='birds.ogg').length,1);layer.dispose();
  const all=rig({decodeFail:true});all.layer.update(wind(0),W);await settle();assert.deepEqual([all.layer.stats().failed,all.layer.stats().loaded.length],[6,0],'no Vorbis: silent, counted');all.layer.dispose();
  const none=rig({fail:['manifest.json']});none.layer.update(wind(0),W);await settle();assert.equal(none.layer.stats().failed,6,'no manifest: all counted');assert.equal(none.layer.stats().loaded.length,0);
  none.layer.setTier('low');none.layer.setTier('high');await settle();assert.equal(none.calls.length,1,'no retry after tier changes');none.layer.dispose();
}
{ // Low tier loads three; raising the tier loads the rest once.
  const {fake,calls,layer}=rig({tier:'low'});layer.update(wind(0),W);await settle();
  assert.deepEqual(calls,['manifest.json','water.ogg','city.ogg','rain.ogg']);assert.equal(fake.live('source').length,6);
  layer.setTier('high');await settle();assert.equal(layer.stats().loaded.length,6);assert.equal(calls.length,7);
  layer.setTier('low');assert.deepEqual(layer.stats().loaded.slice().sort(),['city','rain','water']);assert.equal(fake.live('source').length,6,'lowering releases the extra voices');
  layer.dispose();
}
{ // Disposed while the load is in flight: nothing is built.
  const {fake,layer}=rig();layer.update(wind(0),W);layer.dispose();await settle();assert.equal(fake.live('source').length,0);
}
// --- engine wiring ---
{
  const fake=createFakeAudio(),calls=[];fake.context.decodeAudioData=async()=>({duration:10,numberOfChannels:1,sampleRate:8000,getChannelData:()=>new Float32Array(4)});
  const manifest=new TextEncoder().encode(JSON.stringify({recordings:R.IDS.map(id=>({id,file:id+'.ogg'}))})).buffer;
  let s=7;const random=()=>(s=(Math.imul(s,1664525)+1013904223)>>>0)/4294967296;
  const sound=createCityAmbience(()=>{},{context:fake.context,timers:fake.timers,random,voice:false,storage:null,enabled:true,load:async p=>{calls.push(p);return p==='manifest.json'?manifest:bytes;}});
  const frame={time:0,seaTime:0,wind:{x:1,z:0,strength:.4},trams:[{on:false},{on:false}],companion:{x:0,z:0,phase:0,on:false},doorValue:()=>0,reduced:false};
  sound.setMood({day:1,evening:0,busy:false,weather:'clear'});
  sound.unlock();assert.equal(calls.length,0,'nothing loads at unlock');sound.frame(frame);assert.equal(calls.length,0,'nothing loads while not active');
  sound.setActive(true);assert.equal(calls.length,0,'nothing loads before a frame');
  const run=(x,z,n=120)=>{for(let i=0;i<n;i++){fake.context.currentTime+=1/60;sound.setListener(x,2.48,z,0,1,x,z);sound.frame(frame);}};
  run(-20,77,2);await settle();assert.equal(calls.length,7,'the first frame loads the manifest and six files');run(-20,77);
  const st=sound.stats().soundscape.layers.recordings;assert.equal(st.loaded.length,6);assert.equal(st.failed,0);assert.ok(st.levels.water>.5&&st.levels.gulls>.3,'quay: '+JSON.stringify(st.levels));
  const before=sound.stats().graph.nodes;assert.ok(before>0);
  run(-13.5,-12);assert.ok(sound.stats().soundscape.layers.recordings.levels.water<.09,'street: no water '+sound.stats().soundscape.layers.recordings.levels.water);
  sound.dispose();assert.equal(fake.live().filter(n=>n.kind!=='destination').length,0,'engine dispose releases the recordings too');
}
{ // The event hook: with an audible gull recording, the synthesised gull is chosen about .35 as often.
  const picks=recordingsOn=>{
    const fake=createFakeAudio();fake.context.decodeAudioData=async()=>({duration:10,numberOfChannels:1,sampleRate:8000,getChannelData:()=>new Float32Array(4)});
    const manifest=new TextEncoder().encode(JSON.stringify({recordings:R.IDS.map(id=>({id,file:id+'.ogg'}))})).buffer;
    let s=3;const random=()=>(s=(Math.imul(s,1664525)+1013904223)>>>0)/4294967296,counts={gull:0,birds:0};
    const sound=createCityAmbience(()=>{},{context:fake.context,timers:fake.timers,random,voice:false,storage:null,enabled:true,load:async p=>{if(!recordingsOn)throw Error('off');return p==='manifest.json'?manifest:bytes;}});
    sound.setMood({day:1,evening:0,busy:false,weather:'clear'});sound.unlock();sound.setActive(true);
    return {fake,sound,counts};
  };
  const frame={time:0,seaTime:0,wind:{x:1,z:0,strength:.4},trams:[{on:false},{on:false}],companion:{x:0,z:0,phase:0,on:false},doorValue:()=>0,reduced:false};
  const gulls=async on=>{const {fake,sound}=picks(on);sound.setListener(-20,2.48,77,0,1,-20,77);sound.frame(frame);await settle();
    for(let i=0;i<120;i++){fake.context.currentTime+=1/60;sound.setListener(-20,2.48,77,0,1,-20,77);sound.frame(frame);}
    const seen=new Set();
    for(let k=0;k<600;k++){fake.timers.run(fake.context.currentTime+4+k*13);fake.flush();for(const o of fake.nodes)if(o.type==='sawtooth'&&o.frequency.ramp[0]?.[0]==='set'&&o.frequency.ramp[1]?.[0]==='exp'&&!seen.has(o)){seen.add(o);}}
    const n=seen.size;sound.dispose();return n;};
  const without=await gulls(false),withRec=await gulls(true);
  assert.ok(without>0&&withRec<without*0.7,'synthesised calls thin out with the recordings: '+without+' -> '+withRec);
}

// --- shipped assets ---
{
  const dir=fileURLToPath(new URL('../ui/3d/system-world/audio/',import.meta.url)),text=fs.readFileSync(dir+'manifest.json','utf8'),man=JSON.parse(text);
  assert.deepEqual(man.recordings.map(r=>r.id).sort(),[...R.IDS].sort(),'the manifest lists exactly the recordings');
  let total=0;
  for(const r of man.recordings){const data=fs.readFileSync(dir+r.file);total+=data.length;assert.equal(data.length,r.bytes,r.id+' bytes');
    assert.equal(crypto.createHash('sha256').update(data).digest('hex'),r.sha256,r.id+' sha256');assert.equal(r.licence,'CC0-1.0',r.id+' licence');}
  assert.ok(total<=1572864,'budget '+total);assert.ok(!/http/i.test(text),'no URL in the manifest');
}
console.log('System World recordings: level laws, lazy single load, failure isolation, two offset sources, routing, low tier, event thinning, dispose and assets verified');
