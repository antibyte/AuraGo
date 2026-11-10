// Phase 3 contract: the lobby registry (sysworld-lobbies.js) against the packed kit v3 GLBs. Exteriors are
// measured now (podiums, treads by ray casts, the `lobby` stand-in, its door glass); interiors once shipped.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {MeshoptDecoder} from 'three/addons/libs/meshopt_decoder.module.js';
import {districts} from '../ui/js/desktop/apps/sysworld-scene.js';
import {PODIUMS,LOBBIES,DOORS,PODIUM,TREAD,FLIGHT,FLOOR,lobbyGround,lobbyAt,lobbyBounds} from '../ui/js/desktop/apps/sysworld-lobbies.js';

const base='ui/3d/system-world/v3',manifest=JSON.parse(await fs.readFile(base+'/manifest.json','utf8')),catalog=new Map(manifest.assets.map(a=>[a.id,a]));
await MeshoptDecoder.ready;
const loader=new GLTFLoader().setMeshoptDecoder(MeshoptDecoder),docOf=data=>JSON.parse(data.subarray(20,20+data.readUInt32LE(12)).toString('utf8'));
async function open(id,level=0){
  const file=catalog.get(id).lods.find(l=>l.level===level).file,data=await fs.readFile(base+'/'+file);
  const gltf=await loader.parseAsync(data.buffer.slice(data.byteOffset,data.byteOffset+data.byteLength),'');gltf.scene.updateMatrixWorld(true);
  return {gltf,doc:docOf(data),file};
}
const near=(a,b,eps,what)=>assert.ok(Math.abs(a-b)<=eps,`${what}: ${Number(a).toFixed(3)} instead of ${Number(b).toFixed(3)}`);
const district=id=>districts.find(d=>d.id===id);
for(const l of LOBBIES){const d=district(l.district);assert.deepEqual(l.origin,[d.x,d.z],l.id+' is authored in its building frame');}
// Podiums and stairs at LOD 0 (LOD 2 draws no stairs): rays from 2.5 m, below every canopy.
const ray=new THREE.Raycaster(),down=new THREE.Vector3(0,-1,0);
const top=(scene,x,z)=>{ray.set(new THREE.Vector3(x,2.5,z),down);ray.far=3;const hit=ray.intersectObject(scene,true)[0];return hit?2.5-hit.distance:null;};
let treads=0;
for(const p of PODIUMS){
  const d=district(p.id),{gltf,file}=await open(d.asset),at=(x,z)=>top(gltf.scene,x-d.x,z-d.z);
  for(const [x,z,ox,oz]of[[p.x0,p.z0,1,1],[p.x1,p.z0,-1,1],[p.x0,p.z1,1,-1],[p.x1,p.z1,-1,-1]]){
    assert.ok((at(x+ox*.4,z+oz*.4)??0)>=PODIUM-.02,file+' podium corner');const out=at(x-ox*.15,z-oz*.15);assert.ok(out===null||out<.02,file+' ground outside the podium');
  }
  for(const f of p.stairs){
    for(const dx of[-f.width/2+.35,0,f.width/2-.35])for(let i=0;i<5;i++){const x=f.x+dx,z=p.z1+(i+.5)*TREAD;near(at(x,z)??-1,lobbyGround(x,z),.02,`${file} tread ${i} at x ${x}`);treads++;}
    near(at(f.x,p.z1-.5)??-1,PODIUM,.02,file+' landing');
    const below=at(f.x,p.z1+FLIGHT+.25);assert.ok(below===null||below<.02,file+' ground beyond the flight');assert.equal(lobbyGround(f.x,p.z1+FLIGHT+.25),null);
  }
}
// Stand-ins: the `lobby` part spans the ground-floor glazing; its closed door glass sits on the registry doors.
const v=new THREE.Vector3();
for(const id of new Set(LOBBIES.map(l=>l.district))){
  const d=district(id),halls=LOBBIES.filter(l=>l.district===id),{gltf,file}=await open(d.asset),part=gltf.scene.getObjectByName('lobby');
  assert.ok(part,file+' has the lobby stand-in part');
  const box=new THREE.Box3().setFromObject(part),rects=halls.map(lobbyBounds);
  near(box.min.y,PODIUM,.02,file+' stand-in bottom');near(box.max.y,Math.max(...halls.map(l=>l.ceiling??l.circle.wall)),.03,file+' stand-in top');
  for(const [x0,z0,x1,z1]of rects)assert.ok(box.min.x<=x0-d.x+.7&&box.max.x>=x1-d.x-.7&&box.min.z<=z0-d.z+.7&&box.max.z>=z1-d.z-.7,file+' stand-in covers its hall');
  assert.ok(box.min.x>=Math.min(...rects.map(r=>r[0]))-d.x-.05&&box.max.x<=Math.max(...rects.map(r=>r[2]))-d.x+.05&&
    box.min.z>=Math.min(...rects.map(r=>r[1]))-d.z-.05&&box.max.z<=Math.max(...rects.map(r=>r[3]))-d.z+.05,file+' stand-in stays inside the lobby');
  const glass=[];part.traverse(m=>{if(!m.isMesh||m.material.name!=='city.glass')return;const pos=m.geometry.getAttribute('position');for(let i=0;i<pos.count;i++)glass.push(v.fromBufferAttribute(pos,i).applyMatrix4(m.matrixWorld).clone());});
  const doors=DOORS.filter(x=>x.district===id);
  for(const door of doors){
    const g=glass.filter(p=>Math.abs(p.x-(door.x-d.x))<=door.width/2+.05);assert.ok(g.length,door.key+' door glass');
    const xs=g.map(p=>p.x),ys=g.map(p=>p.y),zs=g.map(p=>p.z);
    near((Math.min(...xs)+Math.max(...xs))/2,door.x-d.x,.02,door.key+' centre');near(Math.max(...xs)-Math.min(...xs),door.width,.02,door.key+' width');
    near(Math.min(...ys),PODIUM,.02,door.key+' sill');near(Math.max(...ys)-Math.min(...ys),door.height,.02,door.key+' height');
    near((Math.min(...zs)+Math.max(...zs))/2,door.z-d.z,.03,door.key+' leaf line');
  }
  assert.equal(glass.filter(p=>!doors.some(door=>Math.abs(p.x-(door.x-d.x))<=door.width/2+.05)).length,0,file+' door glass without a registry door');
}
// Interiors (Task 0 payload): ship together, frame, leaves, workpoints, fixtures, glazing, size.
const interiorIds=[...new Set(LOBBIES.map(l=>l.asset))],shipped=interiorIds.filter(id=>catalog.has(id));
assert.ok(shipped.length===0||shipped.length===interiorIds.length,'Interiors ship together: '+shipped);
let interiorBytes=0;
for(const id of shipped){
  const a=catalog.get(id),halls=LOBBIES.filter(l=>l.asset===id),[ox,oz]=halls[0].origin,doors=DOORS.filter(d=>d.asset===id);
  assert.equal(a.category,'interior',id+' category');
  for(const door of doors)for(const side of['l','r'])assert.ok(a.components.includes(`door_${door.n}_${side}`),`${id} door_${door.n}_${side}`);
  const points=a.navigation.workpoints||[];assert.ok(points.length>=3,id+' workpoints');
  for(const w of points){near(w.position[1],FLOOR,.01,id+' '+w.id+' height');assert.ok(halls.some(l=>lobbyAt(ox+w.position[0],oz+w.position[2])?.id===l.id),id+' '+w.id+' outside the clear room');}
  for(const b of a.navigation.colliders||[]){
    assert.ok(b[1]>=PODIUM-.01&&b[4]>b[1],id+' collider height '+b);
    assert.ok(halls.some(l=>{const [x0,z0,x1,z1]=lobbyBounds(l);return ox+b[0]>=x0-.05&&ox+b[3]<=x1+.05&&oz+b[2]>=z0-.05&&oz+b[5]<=z1+.05;}),id+' collider outside the lobby '+b);
  }
  assert.ok(a.lods[0].triangles<=12000,id+' LOD 0 triangles '+a.lods[0].triangles);interiorBytes+=a.lods.reduce((s,l)=>s+l.bytes,0);
  const {gltf,doc,file}=await open(id);
  assert.ok(doc.materials.some(m=>m.name==='city.pane')&&!doc.materials.some(m=>m.name==='city.window'),file+' glazing');
  for(const door of doors)for(const [side,sign]of[['l',-1],['r',1]]){
    const leaf=gltf.scene.getObjectByName(`door_${door.n}_${side}`);assert.ok(leaf,file+' leaf node');
    near(leaf.position.x,door.x-ox+sign*door.width/4,.02,door.key+side+' pivot x');near(leaf.position.y,PODIUM,.02,door.key+side+' pivot y');
    near(leaf.position.z,door.z-oz,.02,door.key+side+' pivot z');
    const box=new THREE.Box3().setFromObject(leaf);near(box.max.x-box.min.x,door.width/2,.08,door.key+side+' leaf width');
  }
}
for(const a of manifest.assets)if(!shipped.includes(a.id))for(const lod of a.lods)
  assert.ok(!docOf(await fs.readFile(base+'/'+lod.file)).materials.some(m=>m.name==='city.pane'),lod.file+' must not carry lobby glazing');
assert.ok(interiorBytes<2.5*1024*1024,'Interiors exceed 2.5 MiB: '+interiorBytes);
console.log(`System World lobby contract: ${PODIUMS.length} podiums, ${treads} treads, ${DOORS.length} doors verified; interiors: `+(shipped.length?`${shipped.length} designs, ${interiorBytes} bytes verified`:'none shipped yet'));
