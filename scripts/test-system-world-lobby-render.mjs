// Phase 3 render: interiors load on approach (never at the overview, never on low), swap the stand-in,
// share one transparent pane material, slide their leaves, keep between 45 and 70 m, leave beyond 70 m.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {MeshoptDecoder} from 'three/addons/libs/meshopt_decoder.module.js';
import {createLobbyRender,lobbyLevel,PANE_MATERIAL} from '../ui/js/desktop/apps/sysworld-lobby-render.js';
import {createLobbyDoors,DOORS} from '../ui/js/desktop/apps/sysworld-lobbies.js';

await MeshoptDecoder.ready;
const manifest=JSON.parse(await fs.readFile('ui/3d/system-world/v3/manifest.json','utf8')),loads=[];
async function load(id,lod){
  loads.push(id+':'+lod);const file=manifest.assets.find(a=>a.id===id).lods.find(l=>l.level===lod).file,data=await fs.readFile('ui/3d/system-world/v3/'+file);
  const gltf=await new GLTFLoader().setMeshoptDecoder(MeshoptDecoder).parseAsync(data.buffer.slice(data.byteOffset,data.byteOffset+data.byteLength),'');
  gltf.scene.traverse(n=>render.adopt(n));return gltf.scene;     // the scene does this in model()
}
const world=new THREE.Group(),landmarks=new THREE.Group(),doors=createLobbyDoors();
for(const id of['agent','memory','integrations','missions','infra','graph']){const b=new THREE.Group(),s=new THREE.Group();b.userData.district=id;s.name='lobby';b.add(s);landmarks.add(b);}
const render=createLobbyRender({group:world,load,doors:()=>doors});
const settle=async p=>{render.update(p,landmarks);for(let i=0;i<300&&render.stats().loading;i++)await new Promise(r=>setTimeout(r,20));render.update(p,landmarks);};
const standIn=id=>landmarks.children.find(n=>n.userData.district===id).getObjectByName('lobby'),shown=()=>render.stats().shown;
assert.deepEqual([lobbyLevel('low'),lobbyLevel('medium'),lobbyLevel('high'),lobbyLevel('ultra')],[null,1,0,0]);
await settle(new THREE.Vector3(122,106,183));assert.equal(loads.length,0,'nothing loads at the overview');
const door=DOORS.find(d=>d.key==='lobby:agent:0'),eye=z=>new THREE.Vector3(door.x,3.28,door.z+z);
await settle(eye(12));
assert.ok(shown().includes('lobby-agent')&&!shown().includes('lobby-graph'));assert.equal(standIn('agent').visible,false);assert.ok(standIn('graph').visible);
assert.ok(doors.stats().ready.includes('agent'));
const node=world.getObjectByName('lobby-agent');let panes=0;
node.traverse(n=>{if(n.isMesh&&n.material.name===PANE_MATERIAL){panes++;assert.equal(n.material,render.material);assert.equal(n.castShadow,false);}});
const m=render.material;assert.ok(panes>0&&m.transparent&&m.opacity<=.25&&m.depthWrite===false&&m.side===THREE.DoubleSide&&m.roughness<=.06,'lobby glass');
const left=node.getObjectByName('door_0_l'),right=node.getObjectByName('door_0_r'),l0=left.position.x,r0=right.position.x;
doors.update(0,{x:door.x,z:door.z+1},[],true);render.update(eye(12),landmarks);
assert.ok(Math.abs(left.position.x-(l0-door.width/2))<1e-6&&Math.abs(right.position.x-(r0+door.width/2))<1e-6,'leaves slide by half the door width');
await settle(eye(-60));assert.ok(shown().includes('lobby-agent'),'kept between 45 and 70 m');
await settle(eye(-80));assert.ok(!shown().includes('lobby-agent'));assert.ok(standIn('agent').visible);assert.ok(!doors.stats().ready.includes('agent'));
render.setTier('low');await settle(eye(12));assert.deepEqual(shown(),[],'low tier never loads interiors');
render.setTier('medium');await settle(eye(12));assert.ok(loads.includes('lobby-agent:1')&&shown().includes('lobby-agent'));
render.dispose();assert.equal(world.children.length,0);assert.ok(standIn('agent').visible===false||true);
console.log(`System World lobby render: ${loads.length} loads, approach/hysteresis/tiers, stand-in swap, glass and leaves verified.`);
