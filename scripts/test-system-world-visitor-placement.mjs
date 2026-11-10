// Leaving a ride (here the drone) seats the visitor body on a free spot, never on top of a resident.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {createExperience} from '../ui/js/desktop/apps/sysworld-experience.js';
import {createTraffic} from '../ui/js/desktop/apps/sysworld-traffic.js';
import {installCityColliders,kitCatalog} from '../ui/js/desktop/apps/sysworld-colliders.js';
import {districts,placements,kitPlacements} from '../ui/js/desktop/apps/sysworld-scene.js';
import {dronePad} from '../ui/js/desktop/apps/sysworld-exploration.js';

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
  put(dronePad.x,dronePad.z+3);
  for(let n=0;n<120&&experience.interaction()?.kind!=='drone';n++)tick();
  assert.equal(experience.interaction()?.kind,'drone','the drone pad offers a ride');
  const originY=camera.position.y;experience.interact();assert.equal(experience.stats().ride,'drone');
  for(let n=0;n<30;n++)tick();
  const origin={x:dronePad.x,y:originY,z:dronePad.z+3,heading:0};
  const blocker=traffic.register('blocker',{circles:[{x:0,z:0,r:.38}],minY:-2.3,maxY:.4,reach:.38},origin,1);
  assert.ok(blocker,'a blocker fits at the ride origin');
  experience.endRide();for(let n=0;n<5;n++)tick();
  assert.ok(camera.position.y<5,'the visitor is back on the ground: '+camera.position.y);
  assert.ok(traffic.clear(visitor,visitor,visitor),'the visitor overlaps '+traffic.obstruction(visitor,visitor,visitor));
  assert.ok(Math.hypot(camera.position.x-visitor.x,camera.position.y-visitor.y,camera.position.z-visitor.z)<1e-6,'the camera follows the visitor body');
  assert.ok(Math.hypot(visitor.x-origin.x,visitor.z-origin.z)<3,'the visitor is set down near the ride origin');
  console.log('System World visitor placement: leaving the drone ride seats the visitor on a free spot.');
}finally{experience.dispose();traffic.dispose();Object.assign(globalThis,original);}
