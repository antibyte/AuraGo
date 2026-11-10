// Phase 3 walk: with the real experience, colliders and kit v3 interiors, the visitor climbs every flight,
// waits at a door whose interior is not shown, enters through every door once it is, and leaves again.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {createExperience} from '../ui/js/desktop/apps/sysworld-experience.js';
import {createTraffic} from '../ui/js/desktop/apps/sysworld-traffic.js';
import {installCityColliders,kitCatalog} from '../ui/js/desktop/apps/sysworld-colliders.js';
import {districts,placements,kitPlacements} from '../ui/js/desktop/apps/sysworld-scene.js';
import {LOBBIES,PODIUMS,PODIUM,FLIGHT,WALL,WALL_OUT} from '../ui/js/desktop/apps/sysworld-lobbies.js';

const original={fetch:globalThis.fetch,localStorage:globalThis.localStorage,self:globalThis.self,createImageBitmap:globalThis.createImageBitmap};
globalThis.self=globalThis;globalThis.createImageBitmap=async()=>({width:1,height:1,close(){}});globalThis.localStorage={getItem:()=>null,setItem:()=>{}};
globalThis.fetch=async url=>{const bytes=await fs.readFile(String(url));return {ok:true,json:async()=>JSON.parse(bytes),arrayBuffer:async()=>bytes.buffer.slice(bytes.byteOffset,bytes.byteOffset+bytes.byteLength)};};
const root='ui/3d/system-world/',read=f=>fs.readFile(root+f,'utf8').then(JSON.parse),v1=await read('v1/manifest.json'),v3=await read('v3/manifest.json');
const scene=new THREE.Scene(),camera=new THREE.PerspectiveCamera(),traffic=createTraffic(),errors=[],sounds=[];
installCityColliders(traffic,kitCatalog(v1,v3),districts,placements,kitPlacements);camera.position.set(18,2.4,57);
const visitor=traffic.register('visitor',{circles:[{x:0,z:0,r:.24}],minY:-2.3,maxY:.4,reach:.24},camera.position,100);
let ready=false;
const experience=createExperience(scene,{traffic,camera,districts,tier:'high',kit3:kitCatalog(v3),assetURL:x=>root+'v2/'+x,reduced:()=>false,active:()=>false,
  onError:e=>errors.push(e),onReady:()=>{ready=true;},onSound:k=>sounds.push(k)});
const dt=1/60,pose=(x,z)=>({x,y:2.4+experience.floor(x,z),z,heading:0});
const tick=()=>{traffic.begin();experience.update(dt,true,'street');traffic.propose(visitor,{...camera.position,heading:0},(_,p)=>camera.position.set(p.x,p.y,p.z));traffic.solve(dt);experience.settle(dt);};
const put=(x,z)=>{const p=pose(x,z);camera.position.set(p.x,p.y,p.z);Object.assign(visitor,p);};
// One step as sysworld-scene.js move(): the pure contract, then the colliders.
const stepTo=(x,z)=>{const p=pose(x,z);if(!experience.move(x,z,camera.position)||!traffic.clear(visitor,visitor,p))return;camera.position.set(p.x,p.y,p.z);};
const walk=(x,z,limit=600)=>{for(let n=0;n<limit;n++){const dx=x-camera.position.x,dz=z-camera.position.z,d=Math.hypot(dx,dz);if(d<.05)return true;const s=Math.min(d,.15);stepTo(camera.position.x+dx/d*s,camera.position.z+dz/d*s);tick();}return false;};
try{
  for(let n=0;n<1500&&!ready;n++){if(errors.length)throw errors[0];await new Promise(r=>setTimeout(r,10));}
  assert.ok(ready,'world did not load');
  let climbs=0;
  for(const l of LOBBIES)for(const d of l.doors){
    const key='lobby:'+l.id+':'+d.n,p=PODIUMS.find(p=>p.id===l.district),foot={x:d.x,z:p.z1+FLIGHT+.9},porch={x:d.x,z:l.front+WALL_OUT+1.1},inside={x:d.x,z:l.front-WALL-1.1};
    if(traffic.clear(visitor,pose(foot.x,foot.z),pose(foot.x,foot.z),false)){put(foot.x,foot.z);tick();assert.ok(walk(porch.x,porch.z),key+': cannot climb the stairs');climbs++;}
    else{put(porch.x,porch.z);tick();}
    assert.ok(Math.abs(camera.position.y-(2.4+PODIUM))<1e-6,key+': not on the podium');
    assert.equal(walk(inside.x,inside.z,80),false,key+': entered although the interior is not shown');
    assert.ok(camera.position.z>l.leaf,key+': crossed the closed leaf line');
    experience.lobbies.setReady(l.id,true);
    assert.ok(walk(inside.x,inside.z),key+': cannot walk in through the open door');
    const s=experience.stats();assert.equal(s.lobby,l.id);assert.equal(s.inside,l.open?null:l.id,key+': inside flag');
    assert.ok(walk(porch.x,porch.z),key+': cannot walk back out');assert.equal(experience.stats().lobby,null);
    experience.lobbies.setReady(l.id,false);for(let n=0;n<60;n++)tick();
  }
  assert.ok(climbs>=8,'too few stair feet are free: '+climbs);
  // Low tier: the doors stay shut for the visitor even with the interior flagged as shown.
  const agent=LOBBIES[0];experience.lobbies.setReady(agent.id,true);await experience.setTier('low');put(0,agent.front+WALL_OUT+1.1);tick();
  assert.equal(walk(0,agent.front-WALL-1.1,120),false,'low tier must keep the doors shut');await experience.setTier('high');
  assert.ok(sounds.includes('door'),'door sounds');assert.equal(errors.length,0);
  console.log(`System World lobby walk: ${LOBBIES.reduce((n,l)=>n+l.doors.length,0)} doors entered and left, ${climbs} flights climbed, low tier shut.`);
}finally{experience.dispose();traffic.dispose();Object.assign(globalThis,original);}
