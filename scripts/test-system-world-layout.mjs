import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {createExperience} from '../ui/js/desktop/apps/sysworld-experience.js';
import {districts} from '../ui/js/desktop/apps/sysworld-scene.js';
import {streetCurve} from '../ui/js/desktop/apps/sysworld-navigation.js';
import {interiors,streets,stations,tramWaypoints,surfaces,streetAt} from '../ui/js/desktop/apps/sysworld-layout.js';

// Check the complete moving vehicle footprint, including its overhang at bends,
// against the road actually used by the renderer, not a second expected route.
const route=streetCurve(tramWaypoints),p=new THREE.Vector3(),tangent=new THREE.Vector3();
const pack=JSON.parse(await fs.readFile('ui/3d/system-world/v2/manifest.json','utf8'));
const tramBounds=pack.assets.find(a=>a.id==='tram').lods[0].bounds;
for(let n=0;n<4096;n++){
  route.getPointAt(n/4096,p);route.getTangentAt(n/4096,tangent);
  for(const forward of[tramBounds.min[2],-2,0,2,tramBounds.max[2]])for(const side of[tramBounds.min[0],0,tramBounds.max[0]]){
    const x=p.x+tangent.x*forward-tangent.z*side,z=p.z+tangent.z*forward+tangent.x*side;
    assert.ok(streetAt(x,z,streets.laneHalfWidth),`Tram leaves carriageway at ${x}, ${z}`);
  }
}
for(const s of stations){
  let distance=Infinity;for(let n=0;n<4096;n++){route.getPointAt(n/4096,p);distance=Math.min(distance,Math.hypot(p.x-s.x,p.z-s.z));}
  assert.ok(distance<.15,'Station is on the driven route: '+s.id);
}
for(const r of interiors){
  for(let x=r.x-6.2;x<=r.x+6.2;x+=.2)for(let z=r.z-6.2;z<=r.z+6.2;z+=.2)
    assert.ok(!streetAt(x,z),'Pavilion intersects road/pavement: '+r.id);
}
assert.ok(surfaces.room-surfaces.quay>=.06&&surfaces.quay-surfaces.ground>=.06,'Separate exposed ground surfaces, no coplanar overlay');

const originalFetch=globalThis.fetch,originalStorage=globalThis.localStorage;
globalThis.localStorage={getItem:()=>null,setItem:()=>{}};
globalThis.fetch=async name=>{
  const file=String(name).split('/').at(-1);assert.match(file,/^[\w.-]+$/);
  const bytes=await fs.readFile('ui/3d/system-world/v2/'+file);
  return {ok:true,json:async()=>JSON.parse(bytes),arrayBuffer:async()=>bytes.buffer.slice(bytes.byteOffset,bytes.byteOffset+bytes.byteLength)};
};
const scene=new THREE.Scene(),camera=new THREE.PerspectiveCamera(),errors=[];
camera.position.set(0,2.56,66);
const experience=createExperience(scene,{camera,districts,tier:'high',assetURL:x=>x,reduced:()=>false,active:()=>true,onError:e=>errors.push(e)});
const wait=async check=>{for(let n=0;n<400;n++){if(errors.length)throw errors[0];if(check())return;await new Promise(r=>setTimeout(r,10));}throw Error('World did not load');};
const ray=new THREE.Raycaster(),down=new THREE.Vector3(0,-1,0);
function floorAt(x,z,height){
  scene.updateMatrixWorld(true);ray.set(new THREE.Vector3(x,height+.1,z),down);
  const hits=ray.intersectObjects(scene.children,true).filter(h=>['floor','lift','quay'].includes(h.object.userData.worldAsset));
  return hits[0]?.point.y;
}
try{
  await wait(()=>experience.stats().trams===2);
  assert.ok(Math.abs(floorAt(75,77,surfaces.quay)-surfaces.quay)<.005,'Exported quay is raised clear of the island ground');
  // Actual exported meshes, including instance transforms, must leave the
  // carriageway free. Decorations and architecture cannot occupy a traffic lane.
  const roadObstacles=new Set(['station','garden','bench','arcade','pad','charger','cooler','service-cart','wall','window','door','lift']);
  scene.updateMatrixWorld(true);
  scene.traverse(node=>{
    if(!node.isMesh||!roadObstacles.has(node.userData.worldAsset))return;
    node.geometry.computeBoundingBox();
    const count=node.isInstancedMesh?node.count:1;
    for(let i=0;i<count;i++){
      const matrix=new THREE.Matrix4();if(node.isInstancedMesh)node.getMatrixAt(i,matrix);
      matrix.premultiply(node.matrixWorld);const box=node.geometry.boundingBox.clone().applyMatrix4(matrix);
      if(box.min.y>3.5)continue;
      for(let x=box.min.x;x<=box.max.x;x+=.2)for(let z=box.min.z;z<=box.max.z;z+=.2)
        assert.ok(!streetAt(x,z,streets.laneHalfWidth),`${node.userData.worldAsset} blocks traffic at ${x}, ${z}`);
    }
  });
  for(const r of interiors){
    camera.position.set(r.x,2.4+surfaces.room,r.doorZ+2);experience.update(0,false,'street');
    await wait(()=>experience.stats().rooms.includes(r.id));
    assert.ok(Math.abs(floorAt(r.x,r.z-3,surfaces.room)-surfaces.room)<.005,'Ground floor matches rendered GLB: '+floorAt(r.x,r.z-3,surfaces.room));
    camera.position.set(r.liftX,2.4+surfaces.room,r.liftZ);experience.update(0,false,'street');
    assert.equal(experience.interaction()?.kind,'lift');experience.interact();
    for(let i=0;i<200;i++)experience.update(.05,true,'street');
    assert.equal(experience.stats().ride,null);
    assert.ok(Math.abs(camera.position.y-(surfaces.gallery+2.4))<.001);
    assert.ok(Math.abs(floorAt(r.liftX,r.liftZ,surfaces.gallery)-surfaces.gallery)<.005,'Rendered lift meets landing height');
    assert.ok(floorAt(r.liftX,r.liftZ,surfaces.room)<surfaces.room-.05,'No ground-floor mesh coplanar with the parked lift');
    for(let x=r.liftX;x>=r.x-4;x-=.1){
      assert.ok(experience.move(x,r.liftZ,camera.position),'Can exit lift and traverse upper floor: '+r.id);
      camera.position.x=x;
      assert.ok(Math.abs(floorAt(x,r.liftZ,surfaces.gallery)-surfaces.gallery)<.03,`No invisible floor/unsupported gap: x=${x}, actual=${floorAt(x,r.liftZ,surfaces.gallery)}`);
      assert.equal(experience.floor(x,r.liftZ),surfaces.gallery);
    }
    assert.equal(experience.move(r.x,r.z-1,camera.position),false,'Atrium has no imaginary upper floor');
    camera.position.set(r.liftX,2.4+surfaces.gallery,r.liftZ);experience.update(0,false,'street');experience.interact();
    for(let i=0;i<200;i++)experience.update(.05,true,'street');
    assert.ok(Math.abs(camera.position.y-(surfaces.room+2.4))<.001);
    assert.ok(Math.abs(floorAt(r.liftX,r.liftZ,surfaces.room)-surfaces.room)<.005);
  }
  for(const quality of ['medium','low']){
    await experience.setTier(quality);
    for(const r of interiors)assert.ok(Math.abs(floorAt(r.x,r.liftZ,surfaces.gallery)-surfaces.gallery)<.005,'Gallery stays aligned after LOD change');
  }
  experience.visit('infra');experience.update(0,false,'street');experience.interact();
  experience.visit('infra');experience.update(0,false,'street');experience.interact();
  for(let i=0;i<2400&&experience.stats().ride!=='tram';i++)experience.update(.05,true,'street');
  assert.equal(experience.stats().ride,'tram','The station platform must allow boarding');
  scene.updateMatrixWorld(true);ray.set(camera.position,camera.getWorldDirection(new THREE.Vector3()));
  assert.ok(!ray.intersectObjects(scene.children,true).some(h=>h.distance<4.3&&h.object.userData.worldAsset==='tram'),'The passenger must see through the actual end window');
  experience.endRide();
  assert.equal(errors.length,0);
  console.log('World layout: 4096 swept tram poses, 7 stops, rendered object footprints, 3 galleries and GLB lift/floor alignment passed.');
}finally{experience.dispose();globalThis.fetch=originalFetch;globalThis.localStorage=originalStorage;}
