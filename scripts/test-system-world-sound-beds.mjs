// Phase 5 beds: level laws, steam by vent distance, laps from the sea wave, the graph per tier, cost and dispose.
import assert from 'node:assert/strict';
import {createFakeAudio} from './system-world-audio-fake.mjs';
import * as B from '../ui/js/desktop/apps/sysworld-sound-beds.js';
import {steamVents} from '../ui/js/desktop/apps/sysworld-particles.js';
const zone=o=>({harbour:0,canyon:0,green:0,plaza:1,height:0,interior:0,open:false,quay:0,...o});
assert.equal(B.leafLevel(0,1),0);assert.ok(Math.abs(B.leafLevel(1,0)-B.BEDS.leaves*.12)<1e-12);assert.ok(Math.abs(B.leafLevel(1,1)-B.BEDS.leaves)<1e-12);
let prev=-1;for(let s=0;s<=1.0001;s+=.05){const v=B.leafLevel(.7,s);assert.ok(v>=prev);prev=v;}
assert.ok(B.trafficLevel(false,zone({canyon:1,plaza:0}),false)>B.trafficLevel(false,zone({}),false),'canyons carry more traffic than an open plaza');
assert.ok(B.trafficLevel(false,zone({plaza:0,harbour:1}),false)<B.trafficLevel(false,zone({}),false));
assert.ok(Math.abs(B.trafficLevel(true,zone({}),false)/B.trafficLevel(false,zone({}),false)-1.6)<1e-12,'busy');
assert.equal(B.rainLevel(false,zone({})),0);assert.ok(B.rainLevel(true,zone({harbour:1,plaza:0}))>B.rainLevel(true,zone({})));
assert.equal(B.patterLevel(true,zone({interior:1,open:true})),0,'no glass in the open court');assert.equal(B.patterLevel(true,zone({interior:1})),B.BEDS.patter);assert.equal(B.patterLevel(false,zone({interior:1})),0);
const tower=steamVents()[0],near=B.steamLevel(tower.x,2.4,tower.z+3,{}),far=B.steamLevel(0,2.4,300,{});
assert.ok(near.level>.5*B.BEDS.steam&&near.x===tower.x&&near.z===tower.z,'beside a cooling tower '+JSON.stringify(near));assert.ok(far.level<.02*B.BEDS.steam,'far out at sea '+far.level);
assert.equal(B.lapCrossing(.1,.2),true);assert.equal(B.lapCrossing(.2,.3),false);assert.equal(B.lapCrossing(.2,.1),false);
const seeded=s=>()=>(s=(Math.imul(s,1664525)+1013904223)>>>0)/4294967296;
const L=(x,z)=>({x,y:2.4,z,fx:0,fz:-1,origin:{x,z}}),fake=createFakeAudio(),c=fake.context,[outdoor,send,local]=['outdoor','send','local'].map(k=>fake.node(k));
const beds=B.createSoundBeds(c,{outdoor,send,local,noise:c.createBuffer(1,c.sampleRate*2),random:seeded(3),tier:'high'});
const sources=()=>fake.live('source').length+fake.live('oscillator').length,high=sources();
assert.ok(fake.downstream(fake.nodes.find(n=>n.kind==='source')).has(outdoor));
beds.setMood({busy:false,weather:'rain'});const f={time:0,wind:{strength:.5}},w=zone({green:.5,plaza:.5});
beds.update(f,w,L(-44,-88),0);const ramps=()=>fake.nodes.reduce((n,x)=>n+x.gain.ramp.length+x.frequency.ramp.length+x.pan.ramp.length,0),r0=ramps();
for(let i=0;i<120;i++)beds.update(f,w,L(-44,-88),0);assert.equal(ramps(),r0,'unchanged input schedules nothing');
assert.ok(beds.stats().levels.leaves>0&&beds.stats().levels.rain>0&&beds.stats().levels.patter===0);
let maxAlive=0;for(let k=0;k<60*25;k++){const t=k/25;c.currentTime=t;beds.update({time:t,wind:{strength:.3}},zone({harbour:.8,plaza:.2,quay:1}),L(-20,77),t);maxAlive=Math.max(maxAlive,beds.stats().laps);if(k%5===0)fake.flush();}
const played=beds.stats().played;assert.ok(played>=5&&played<=60,'laps in a minute at the quay: '+played);assert.ok(maxAlive<=B.LAP.max);
for(let k=0;k<30*25;k++){const t=60+k/25;c.currentTime=t;beds.update({time:t,wind:{strength:.3}},zone({}),L(0,0),t);}assert.equal(beds.stats().played,played,'no laps inland');
beds.setTier('low');assert.ok(sources()<high&&beds.stats().rich===false,'low: traffic and rain only');beds.setTier('high');assert.equal(sources(),high,'high rebuilds');
fake.flush();beds.dispose();assert.equal(fake.live().filter(n=>!['destination','outdoor','send','local'].includes(n.kind)).length,0,'dispose releases everything');
console.log(`System World sound beds: laws, steam, ${played} laps per minute at the quay, tiers and cost verified`);
