// Aviation lights stand on the centre of each tower's roof: roofTop finds the highest vertices' centre, aviationPoints turns
// and scales it with the placement; on the shipped LOD 2 towers every light lies over its roof.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {MeshoptDecoder} from 'three/addons/libs/meshopt_decoder.module.js';
import {roofTop,aviationPoints} from '../ui/js/desktop/apps/sysworld-atmosphere.js';
import {placements} from '../ui/js/desktop/apps/sysworld-scene.js';

{ // a 10 m block with a 2 m mast off-centre: the top is the mast
  const g=new THREE.Group(),block=new THREE.Mesh(new THREE.BoxGeometry(8,10,8));block.position.y=5;
  const mast=new THREE.Mesh(new THREE.BoxGeometry(.4,2,.4));mast.position.set(3,11,-1);g.add(block,mast);
  const r=roofTop(g);assert.ok(Math.abs(r.y-12)<1e-6&&Math.abs(r.x-3)<1e-6&&Math.abs(r.z+1)<1e-6,JSON.stringify(r));
  const [p]=aviationPoints([{asset:'t',x:10,y:-3,z:-150,angle:Math.PI/2,scale:[2,1.5,2]}],new Map([['t',r]]));
  // local (3,-1) scaled to (6,-2), turned 90 degrees: x' = -2, z' = -6
  assert.ok(Math.abs(p.x-8)<1e-9&&Math.abs(p.z+156)<1e-9&&Math.abs(p.y-(-3+12*1.5+.6))<1e-9,JSON.stringify(p));
  assert.equal(aviationPoints([{asset:'t',x:0,y:0,z:0,angle:0,scale:[1,1,1]}],new Map([['t',r]])).length,0,'near, unscaled towers carry no light');
}
await MeshoptDecoder.ready;
const loader=new GLTFLoader().setMeshoptDecoder(MeshoptDecoder),kit=JSON.parse(await fs.readFile('ui/3d/system-world/v3/manifest.json','utf8')),roofs=new Map(),bands=new Map();
for(const id of ['data-tower-a','data-tower-b']){
  const lod=kit.assets.find(a=>a.id===id).lods.find(l=>l.level===2),data=await fs.readFile('ui/3d/system-world/v3/'+lod.file);
  const gltf=await loader.parseAsync(data.buffer.slice(data.byteOffset,data.byteOffset+data.byteLength),'');const r=roofTop(gltf.scene);roofs.set(id,r);
  const v=new THREE.Vector3(),box=new THREE.Box2();box.makeEmpty();gltf.scene.updateMatrixWorld(true);
  gltf.scene.traverse(n=>{const p=n.isMesh&&n.geometry.getAttribute('position');if(!p)return;for(let i=0;i<p.count;i++){v.fromBufferAttribute(p,i).applyMatrix4(n.matrixWorld);if(v.y>=r.y-.5)box.expandByPoint(new THREE.Vector2(v.x,v.z));}});
  bands.set(id,box);assert.ok(box.containsPoint(new THREE.Vector2(r.x,r.z)),id+': roof centre lies inside its top band');
}
const lit=placements.filter(p=>roofs.has(p.asset)&&(p.z<-100||p.scale[1]>1.1)),points=aviationPoints(placements,roofs);
assert.equal(points.length,lit.length,'one light per tall or skyline tower');
points.forEach((q,i)=>{const p=lit[i],c=Math.cos(p.angle||0),s=Math.sin(p.angle||0),dx=q.x-p.x,dz=q.z-p.z,lx=(dx*c-dz*s)/p.scale[0],lz=(dx*s+dz*c)/p.scale[2];
  assert.ok(bands.get(p.asset).clone().expandByScalar(.05).containsPoint(new THREE.Vector2(lx,lz)),'light '+i+' floats beside its roof');
  assert.ok(Math.abs(q.y-(p.y+roofs.get(p.asset).y*p.scale[1]+.6))<1e-9,'0.6 m above the roof');});
console.log(`System World aviation: ${points.length} lights on roof tops (offsets from the origin ${[...roofs].map(([id,r])=>id+' '+Math.hypot(r.x,r.z).toFixed(2)+' m').join(', ')}).`);
