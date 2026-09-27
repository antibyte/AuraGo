import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {createExperience} from '../ui/js/desktop/apps/sysworld-experience.js';
import {createCityLife} from '../ui/js/desktop/apps/sysworld-life.js';
import {createDrones} from '../ui/js/desktop/apps/sysworld-drones.js';
import {createTraffic,fixedSteps} from '../ui/js/desktop/apps/sysworld-traffic.js';
import {installCityColliders} from '../ui/js/desktop/apps/sysworld-colliders.js';
import {districts,placements,obstacles} from '../ui/js/desktop/apps/sysworld-scene.js';

// Run the real controllers and exported model dimensions, without a WebGL context.
// Pixels, material appearance and real image decoding belong to the browser check.
const original={fetch:globalThis.fetch,localStorage:globalThis.localStorage,self:globalThis.self,createImageBitmap:globalThis.createImageBitmap};
globalThis.self=globalThis;
globalThis.createImageBitmap=async()=>({width:1,height:1,close(){}});
globalThis.localStorage={getItem:()=>null,setItem:()=>{}};
globalThis.fetch=async url=>{
  if(String(url).startsWith('blob:'))return original.fetch(url);
  const bytes=await fs.readFile(String(url));
  return {ok:true,json:async()=>JSON.parse(bytes),arrayBuffer:async()=>bytes.buffer.slice(bytes.byteOffset,bytes.byteOffset+bytes.byteLength)};
};
const root='ui/3d/system-world/',catalog=JSON.parse(await fs.readFile(root+'v1/manifest.json','utf8'));
const scene=new THREE.Scene(),camera=new THREE.PerspectiveCamera(),traffic=createTraffic(),errors=[];
camera.position.set(18,2.5,57);
installCityColliders(traffic,new Map(catalog.assets.map(a=>[a.id,a])),districts,placements);
const visitor=traffic.register('visitor',{circles:[{x:0,z:0,r:.38}],minY:-2.3,maxY:.4,reach:.38},camera.position,100);
assert.ok(visitor);
let ready=false,active=true,selected=null;
const experience=createExperience(scene,{traffic,camera,districts,tier:'high',assetURL:x=>root+'v2/'+x,reduced:()=>false,active:()=>active,onError:e=>errors.push(e),onReady:()=>{ready=true;},onSociety:s=>{selected=s;}});
const life=createCityLife(scene,districts,{traffic,society:experience.society,robotURL:root+'white-robot.glb',obstacles,active:()=>active,onError:e=>errors.push(e)});
const drones=createDrones(scene,{traffic});
const droneBytes=await fs.readFile(root+'v1/service-drone.lod0.glb');
const drone=await new GLTFLoader().parseAsync(droneBytes.buffer.slice(droneBytes.byteOffset,droneBytes.byteOffset+droneBytes.byteLength),'');
const wait=async check=>{for(let n=0;n<1000;n++){if(errors.length)throw errors[0];if(check())return;await new Promise(r=>setTimeout(r,10));}throw Error('Living city did not load');};
let time=0,frames=0;
const starts=new Map(),travel=new Map(),stalled=new Map();
const seconds=Number(process.env.SYSTEM_WORLD_SIM_SECONDS||120);
const simulate=fixedSteps(dt=>{
  traffic.begin();time+=dt;
  experience.update(dt,true,'orbit');life.update(dt,true);drones.update(dt,time,true);
  traffic.propose(visitor,{...camera.position,heading:0});traffic.solve(dt);
  if(++frames%30===0)for(const p of traffic.stats().poses){
    const b=traffic.body(p.id);assert.ok(traffic.clear(b,b,b),`Overlap at ${time.toFixed(1)}s: ${JSON.stringify(p)}`);
    const before=starts.get(p.id);if(before){
      const distance=Math.hypot(p.x-before.x,p.z-before.z);travel.set(p.id,(travel.get(p.id)||0)+distance);
      stalled.set(p.id,distance>.05?time:stalled.get(p.id)||time);
      if(p.id.startsWith('tram-')&&time>seconds/3+90)assert.ok(time-stalled.get(p.id)<60,'Tram deadlock after visitor release: '+JSON.stringify(traffic.stats().poses));
    }starts.set(p.id,p);
  }
});
try{
  await wait(()=>ready&&experience.stats().society.places.length===6&&life.stats().robots===5);
  drones.setTemplate(drone.scene);
  assert.equal(experience.stats().society.residents.length,24,'All five patrols and nineteen residents have safe spawn positions');
  assert.equal(drones.stats().drones,6);
  const intervals=[1/120,1/60,1/30,.08,.013,.2];
  for(let i=0;time<seconds;i++){
    if(time>seconds/3&&camera.position.x===18){assert.ok(traffic.relocate(visitor,{x:80,y:2.5,z:50,heading:0}));camera.position.set(visitor.x,visitor.y,visitor.z);}
    simulate(intervals[i%intervals.length]);if(i%300===0)await new Promise(r=>setTimeout(r,0));
  }
  const progress=Object.fromEntries([...travel].map(([id,distance])=>[id,Math.round(distance)]));
  if(process.env.SYSTEM_WORLD_DIAGNOSTICS)console.log(JSON.stringify({bodies:traffic.stats().poses,progress,society:experience.stats().society,drones:drones.stats()}));
  assert.ok(experience.stats().society.completed>0,'Robots must reach workstations, not just avoid collisions');
  assert.ok(experience.stats().society.signals.sent>0,'Ambient meetings must complete directed exchanges');
  for(const id of ['tram-0','tram-1',...Array.from({length:6},(_,i)=>'drone-'+i),...Array.from({length:5},(_,i)=>'patrol-'+i)])assert.ok(travel.get(id)>15,`${id} stuck: ${JSON.stringify(progress)}`);
  if(seconds>=300)for(const id of ['tram-0','tram-1'])assert.ok(travel.get(id)>seconds*2,`${id} cannot resume its circuit: ${JSON.stringify(progress)}`);
  assert.ok([...travel].filter(([id,d])=>id.startsWith('resident')&&d>8).length>=15,'Most residents should make progress');
  const resident=experience.stats().society.residents.find(a=>a.role==='courier');
  experience.society.inspect(resident.id);experience.socialAction('greet');
  assert.equal(selected.state,'greet');assert.equal(selected.source,'ambient');
  const still={...traffic.body(resident.id)};for(let n=0;n<60;n++)simulate(1/60);
  assert.ok(Math.hypot(still.x-traffic.body(resident.id).x,still.z-traffic.body(resident.id).z)<.01,'A greeting stops the selected robot');
  experience.socialAction('guide','infra');assert.equal(selected.guided,true,'A courier can guide to an existing station');
  const waiting={...traffic.body(resident.id)};for(let n=0;n<60;n++)simulate(1/60);
  assert.ok(Math.hypot(waiting.x-traffic.body(resident.id).x,waiting.z-traffic.body(resident.id).z)<.01,'The guide waits for a distant visitor');
  experience.socialAction('cancel');assert.equal(selected.guided,false);
  experience.society.suspend();
  assert.equal(experience.stats().society.reservations,0);assert.equal(experience.stats().society.meetings,0);assert.equal(experience.stats().society.signals.active,0);
  await experience.setTier('low');drones.setTier('low');simulate(1/60);
  assert.equal(experience.stats().residents,3);assert.equal(drones.stats().drones,2);
  await experience.setTier('high');drones.setTier('high');simulate(1/60);
  assert.equal(experience.stats().residents,19);assert.equal(drones.stats().drones,6);
  const now=Date.now();experience.society.event({state:'failed',from:'agent',to:'infra',at:now});
  assert.equal(experience.stats().machinery.places.find(p=>p.id==='repair-bay').source,'live');
  experience.setWorld(null,true);experience.society.event({state:'succeeded',from:'agent',to:'infra',at:now});
  assert.ok(experience.stats().machinery.places.every(p=>p.source==='ambient'),'Replay cannot start live demonstrations');
  experience.setWorld(null,false);active=false;experience.society.event({state:'succeeded',from:'agent',to:'infra',at:now});
  active=true;experience.society.event({state:'succeeded',from:'agent',to:'infra',at:now-30000});
  assert.ok(experience.stats().machinery.places.every(p=>p.source==='ambient'),'Hidden/stale events cannot replay on resume');
  console.log(JSON.stringify({seconds:Math.round(time),traffic:traffic.stats().bodies,completed:experience.stats().society.completed,signals:experience.stats().society.signals.sent,travel:progress}));
}finally{
  life.dispose();experience.dispose();drones.dispose();traffic.dispose();
  Object.assign(globalThis,original);
}
