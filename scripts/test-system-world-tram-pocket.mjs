import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {createExperience} from '../ui/js/desktop/apps/sysworld-experience.js';
import {createCityLife} from '../ui/js/desktop/apps/sysworld-life.js';
import {createDrones} from '../ui/js/desktop/apps/sysworld-drones.js';
import {createTraffic,fixedSteps} from '../ui/js/desktop/apps/sysworld-traffic.js';
import {installCityColliders,kitCatalog} from '../ui/js/desktop/apps/sysworld-colliders.js';
import {districts,placements,kitPlacements,obstacles} from '../ui/js/desktop/apps/sysworld-scene.js';

// The tram pocket: with the street furniture as traffic solids, tram-1 and the patrol robot patrol-0 used to
// lock each other near (-67,-69) after about 115 s and both trams stood still for good. A patrol that cannot step
// aside for more than 3 s must move to a free spot outside the rail corridor, so the trams keep their circuit.
// No visitor takes part: it is parked away from the rails, as in the living test.
const original={fetch:globalThis.fetch,localStorage:globalThis.localStorage,self:globalThis.self,createImageBitmap:globalThis.createImageBitmap};
globalThis.self=globalThis;
globalThis.createImageBitmap=async()=>({width:1,height:1,close(){}});
globalThis.localStorage={getItem:()=>null,setItem:()=>{}};
globalThis.fetch=async url=>{
  const bytes=await fs.readFile(String(url));
  return {ok:true,json:async()=>JSON.parse(bytes),arrayBuffer:async()=>bytes.buffer.slice(bytes.byteOffset,bytes.byteOffset+bytes.byteLength)};
};
const root='ui/3d/system-world/',catalog=JSON.parse(await fs.readFile(root+'v1/manifest.json','utf8'));
const scene=new THREE.Scene(),camera=new THREE.PerspectiveCamera(),traffic=createTraffic(),errors=[];
camera.position.set(18,2.5,57);
const v3=JSON.parse(await fs.readFile(root+'v3/manifest.json','utf8'));
installCityColliders(traffic,kitCatalog(catalog,v3),districts,placements,kitPlacements);
const visitor=traffic.register('visitor',{circles:[{x:0,z:0,r:.38}],minY:-2.3,maxY:.4,reach:.38},camera.position,100);
assert.ok(visitor);
let ready=false,active=true;
const experience=createExperience(scene,{traffic,camera,districts,tier:'high',kit3:kitCatalog(v3),assetURL:x=>root+'v2/'+x,reduced:()=>false,active:()=>active,onError:e=>errors.push(e),onReady:()=>{ready=true;},onSociety:()=>{}});
const life=createCityLife(scene,districts,{traffic,society:experience.society,robotURL:root+'white-robot.glb',obstacles,active:()=>active,onError:e=>errors.push(e)});
const drones=createDrones(scene,{traffic});
const droneBytes=await fs.readFile(root+'v1/service-drone.lod0.glb');
const drone=await new GLTFLoader().parseAsync(droneBytes.buffer.slice(droneBytes.byteOffset,droneBytes.byteOffset+droneBytes.byteLength),'');
const wait=async check=>{for(let n=0;n<1000;n++){if(errors.length)throw errors[0];if(check())return;await new Promise(r=>setTimeout(r,10));}throw Error('Living city did not load');};
const seconds=Number(process.env.SYSTEM_WORLD_SIM_SECONDS||200);
const tramIds=['tram-0','tram-1'],patrolIds=Array.from({length:5},(_,i)=>'patrol-'+i);
const lastSample=new Map(),slowSince=new Map(),worstSlow=new Map(),lastPatrol=new Map(),relocations=[];
const pathLength=new Map();
let time=0,sampleAt=0;
const lateralTo=(p,t)=>{const dx=p.x-t.x,dz=p.z-t.z;return Math.abs(dx*Math.cos(t.heading)-dz*Math.sin(t.heading));};
const simulate=fixedSteps(dt=>{
  traffic.begin();time+=dt;
  experience.update(dt,true,'orbit');life.update(dt,true);drones.update(dt,time,true);
  traffic.propose(visitor,{...camera.position,heading:0});traffic.solve(dt);experience.settle(dt);
  // A patrol that jumps more than 1.9 m in one step (the rule searches 2 to 12 m away) used the boxed rule (nothing else moves a robot that far).
  for(const id of patrolIds){
    const b=traffic.body(id),before=lastPatrol.get(id);
    if(b&&before&&Math.hypot(b.x-before.x,b.z-before.z)>1.9){
      const tram=tramIds.map(t=>traffic.body(t)).filter(Boolean).sort((p,q)=>Math.hypot(before.x-p.x,before.z-p.z)-Math.hypot(before.x-q.x,before.z-q.z))[0];
      relocations.push({id,t:Math.round(time*10)/10,from:{x:Math.round(before.x*10)/10,z:Math.round(before.z*10)/10},to:{x:Math.round(b.x*10)/10,z:Math.round(b.z*10)/10},lateral:Math.round(lateralTo(b,tram)*10)/10,need:Math.round((tram.reach+b.reach+1)*10)/10,tram:tram.id,free:traffic.clear(b,b,b)});
    }
    if(b)lastPatrol.set(id,{x:b.x,z:b.z});
  }
  if(time>=sampleAt){
    sampleAt=time+.5;
    for(const id of tramIds){
      const b=traffic.body(id),before=lastSample.get(id);
      if(before&&time>100){
        const speed=Math.hypot(b.x-before.x,b.z-before.z)/(time-before.t);
        if(speed<.05){if(!slowSince.has(id))slowSince.set(id,before.t);worstSlow.set(id,Math.max(worstSlow.get(id)||0,time-slowSince.get(id)));}
        else slowSince.delete(id);
      }
      if(before&&before.t>=110)pathLength.set(id,(pathLength.get(id)||0)+Math.hypot(b.x-before.x,b.z-before.z));
      lastSample.set(id,{x:b.x,z:b.z,t:time});
    }
  }
});
try{
  await wait(()=>ready&&experience.stats().society.places.length===14&&life.stats().robots===5);
  drones.setTemplate(drone.scene);
  assert.ok(traffic.relocate(visitor,{x:80,y:2.5,z:50,heading:0}));camera.position.set(visitor.x,visitor.y,visitor.z);
  const intervals=[1/120,1/60,1/30,.08,.013,.2];
  for(let i=0;time<seconds;i++){simulate(intervals[i%intervals.length]);if(i%300===0)await new Promise(r=>setTimeout(r,0));}
  const slow=Object.fromEntries(tramIds.map(id=>[id,Math.round((worstSlow.get(id)||0)*10)/10]));
  for(const id of tramIds)assert.ok((worstSlow.get(id)||0)<=30,`${id} stood still for ${slow[id]} s after 100 s; patrol relocations: ${JSON.stringify(relocations)}; poses: ${JSON.stringify(traffic.stats().poses.filter(p=>/tram|patrol/.test(p.id)))}`);
  assert.ok(pathLength.get('tram-1')>=30,`tram-1 covered only ${pathLength.get('tram-1')} m between 110 s and 200 s`);
  for(const r of relocations){
    assert.ok(r.lateral>=r.need,`${r.id} left the rail corridor of ${r.tram} too short: ${JSON.stringify(r)}`);
    assert.ok(r.free,`${r.id} relocated onto an occupied spot: ${JSON.stringify(r)}`);
  }
  for(const id of patrolIds){const b=traffic.body(id);assert.ok(traffic.clear(b,b,b),id+' ends on an occupied spot');}
  console.log(JSON.stringify({seconds:Math.round(time),slowestStandSeconds:slow,tram1MetresFrom110:Math.round(pathLength.get('tram-1')),relocations}));
}finally{
  life.dispose();experience.dispose();drones.dispose();traffic.dispose();
  Object.assign(globalThis,original);
}
