// System World furniture near and far sets: hysteresis, the re-sort gate (twice a second, 4 m), instance matrices of both
// sets, counts and bounding spheres, NEAR flags, dispose; the shipped trees keep their leaf part at the far level.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {MeshoptDecoder} from 'three/addons/libs/meshopt_decoder.module.js';
import {FURNITURE_LOD,nearFlag,worthSets,setDesigns,createFurnitureSets} from '../ui/js/desktop/apps/sysworld-furniture-lod.js';
import {furnishing} from '../ui/js/desktop/apps/sysworld-furnishing.js';

assert.equal(nearFlag(44.9,false),true);assert.equal(nearFlag(45,false),false);assert.equal(nearFlag(59.9,true),true);assert.equal(nearFlag(60.1,true),false);
assert.deepEqual({...FURNITURE_LOD},{near:45,far:60,interval:.5,move:4,level:2,worth:8000});
assert.equal(worthSets(100,100,21),false,'7 900 triangles saved');assert.equal(worthSets(100,100,20),true,'exactly 8 000 saved');
const template=parts=>{const g=new THREE.Group();parts.forEach(([y,name])=>{const m=new THREE.Mesh(new THREE.BoxGeometry(.5,.5,.5),new THREE.MeshStandardMaterial({name}));m.position.y=y;g.add(m);});return g;};
const near=new Map([['bench',template([[0,'city.bronze'],[1,'city.leaf']])],['bin',template([[0,'city.iron']])]]);
const far=new Map([['bench',template([[0,'city.bronze']])],['bin',template([[0,'city.iron']])]]);
const pieces=[{asset:'bench',x:10,y:0,z:0,angle:Math.PI/2,scale:[1,1,1]},{asset:'bench',x:50,y:0,z:0,angle:0,scale:[1,1,1]},{asset:'bin',x:100,y:.2,z:0,angle:0,scale:[1,1,1]},{asset:'kiosk',x:0,y:0,z:0,angle:0,scale:[1,1,1]}];
const sets=createFurnitureSets({pieces,near,far,isNear:id=>id==='bin'});
const of=(asset,kind)=>sets.meshes.filter(m=>m.userData.furniture===kind&&m.userData.asset===asset);
assert.equal(sets.meshes.length,5,'bench: 2 near parts + 1 far part; bin: 1 + 1; the kiosk has no templates');
const at=(x,z)=>new THREE.Vector3(x,1.7,z);
assert.equal(sets.sync(at(0,0),0,true),true,'a forced sort writes');
assert.deepEqual(of('bench','near').map(m=>m.count),[1,1]);assert.equal(of('bench','far')[0].count,1);assert.equal(of('bin','far')[0].count,1);assert.equal(of('bin','near')[0].count,0);
assert.ok(of('bin','near')[0].userData.near&&!of('bench','near')[0].userData.near,'NEAR flag per design');
{ const m=new THREE.Matrix4(),p=new THREE.Vector3(),q=new THREE.Quaternion(),s=new THREE.Vector3();of('bench','near')[1].getMatrixAt(0,m);m.decompose(p,q,s);
  assert.ok(p.distanceTo(new THREE.Vector3(10,1,0))<1e-3&&q.angleTo(new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0,1,0),Math.PI/2))<1e-3,'near matrix (placement times part)');
  const f=new THREE.Matrix4();of('bench','far')[0].getMatrixAt(0,f);assert.ok(new THREE.Vector3().setFromMatrixPosition(f).distanceTo(new THREE.Vector3(50,0,0))<1e-3,'far matrix of the second bench');
  assert.ok(of('bench','far')[0].boundingSphere.center.distanceTo(new THREE.Vector3(50,0,0))<1,'bounding sphere follows the set'); }
assert.equal(sets.sync(at(30,0),.3),false,'too soon');
assert.equal(sets.sync(at(2,0),1),false,'moved less than 4 m');
assert.equal(sets.sync(at(30,0),1),true,'the second bench is 20 m away now');assert.equal(of('bench','near')[0].count,2);assert.equal(of('bench','far')[0].count,0);
assert.equal(sets.sync(at(-2,0),2),false,'52 m: the second bench stays near (hysteresis), nothing to rewrite');assert.equal(of('bench','near')[0].count,2);
assert.equal(sets.sync(at(-12,0),3),true,'62 m: it leaves');assert.equal(of('bench','near')[0].count,1);
assert.deepEqual(sets.stats(),{designs:2,pieces:3,near:1,far:2,sorts:3,meshes:5});
{ const parent=new THREE.Group(),meshes=[...sets.meshes];meshes.forEach(m=>parent.add(m));let disposed=0;meshes.forEach(m=>m.addEventListener('dispose',()=>disposed++));
  sets.dispose();assert.equal(disposed,5);assert.equal(parent.children.length,0);assert.ok(near.get('bench').children[0].geometry.getAttribute('position').count>0,'templates stay');sets.dispose(); }
await MeshoptDecoder.ready;
const loader=new GLTFLoader().setMeshoptDecoder(MeshoptDecoder),kit=JSON.parse(await fs.readFile('ui/3d/system-world/v3/manifest.json','utf8'));
for(const id of ['tree-round','tree-column'])for(const level of [0,FURNITURE_LOD.level]){
  const lod=kit.assets.find(a=>a.id===id).lods.find(l=>l.level===level),data=await fs.readFile('ui/3d/system-world/v3/'+lod.file);
  const gltf=await loader.parseAsync(data.buffer.slice(data.byteOffset,data.byteOffset+data.byteLength),'');let leaf=0;
  gltf.scene.traverse(n=>{if(n.isMesh&&[].concat(n.material).some(m=>m.name==='city.leaf'))leaf++;});assert.ok(leaf>0,id+' LOD '+level+' has no city.leaf part (no sway)');}
{ const counts=new Map();for(const f of furnishing())if(f.asset!=='banner-mast')counts.set(f.asset,(counts.get(f.asset)||0)+1);
  const catalog=new Map(kit.assets.map(a=>[a.id,a]));
  const names=level=>[...setDesigns(counts,catalog,level)].sort();
  assert.deepEqual(names(0),['bollard','flower-bed','litter-bin','tree-column','tree-round','zebra-sign'],'designs with sets on high');
  assert.deepEqual(names(1),['bollard','flower-bed','litter-bin','tree-column','tree-round'],'designs with sets on medium');
  assert.equal(setDesigns(counts,catalog,2).size,0,'none on low'); }
console.log('System World furniture sets: hysteresis 45/60 m, gate 0.5 s and 4 m, both sets written and bounded, trees keep their leaves at LOD 2.');
