// Phase 4 harbour: the JS wave is the shader's, ten minutes inside the keep-out, wave ride, rigs, lanterns, tiers,
// no per-frame allocation, dispose.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import * as THREE from 'three';
import {seaHeight} from '../ui/js/desktop/apps/sysworld-atmosphere.js';
import {CRAFT,KEEP_OUT,SEA_LEVEL,HEAVE,craftPoses,createHarbour} from '../ui/js/desktop/apps/sysworld-harbour.js';
import {streets} from '../ui/js/desktop/apps/sysworld-layout.js';
import {landmarkPlaces} from '../ui/js/desktop/apps/sysworld-places.js';
const source=await fs.readFile('ui/js/desktop/apps/sysworld-atmosphere.js','utf8');
const glsl=/float height\(vec2 p\)\{return ([\s\S]*?);\}/.exec(source)[1].replace(/\s+/g,'').replace(/p\.x/g,'x').replace(/p\.y/g,'z').replace(/time/g,'t').replace(/sin\(/g,'Math.sin(');
const shader=new Function('x','z','t','return '+glsl);
for(let i=0;i<500;i++){const x=(Math.random()-.5)*600,z=(Math.random()-.5)*600,t=Math.random()*900;assert.ok(Math.abs(shader(x,z,t)-seaHeight(x,z,t))<1e-9,'seaHeight differs from the SEA shader');}
const turbine=landmarkPlaces.find(l=>l.id==='tidal-turbine'),poses=craftPoses(0);
const corners=(c,p)=>[[1,1],[1,-1],[-1,1],[-1,-1]].map(([a,b])=>{const lx=a*c.length/2,lz=b*c.beam/2,co=Math.cos(p.heading),si=Math.sin(p.heading);return [p.x+lx*co+lz*si,p.z-lx*si+lz*co];});
const island=([x,z])=>Math.hypot(Math.max(0,streets.minX-x,x-streets.maxX),Math.max(0,streets.minZ-z,z-streets.maxZ));
let worst={island:Infinity,turbine:Infinity,boats:Infinity},previous=null;
for(let t=0;t<=600;t+=.1){
  craftPoses(t,poses);const cs=CRAFT.map((c,i)=>corners(c,poses[i]));
  for(const p of cs.flat()){worst.island=Math.min(worst.island,island(p));worst.turbine=Math.min(worst.turbine,Math.hypot(p[0]-turbine.x,p[1]-turbine.z));}
  for(let i=0;i<cs.length;i++)for(let j=i+1;j<cs.length;j++)for(const a of cs[i])for(const b of cs[j])worst.boats=Math.min(worst.boats,Math.hypot(a[0]-b[0],a[1]-b[1]));
  if(previous)poses.forEach((p,i)=>{const dx=p.x-previous[i].x,dz=p.z-previous[i].z,d=Math.hypot(dx,dz);if(d>1e-6)assert.ok(Math.cos(p.heading)*dx/d-Math.sin(p.heading)*dz/d>.9,CRAFT[i].id+' sails bow first at '+t.toFixed(1));});
  previous=poses.map(p=>({...p}));
}
assert.ok(worst.island>=KEEP_OUT&&worst.turbine>=KEEP_OUT&&worst.boats>=KEEP_OUT,'keep-out over ten minutes: '+JSON.stringify(worst));
const box=new THREE.BoxGeometry(),mat=new THREE.MeshBasicMaterial(),loads=[];
const template=()=>{const g=new THREE.Group();g.add(new THREE.Mesh(box,mat));for(const name of['rig','lamp']){const p=new THREE.Group();p.name=name;p.position.set(.5,1,0);p.add(new THREE.Mesh(box,mat));g.add(p);}return g;};
const group=new THREE.Group(),harbour=createHarbour({group,load:(id,lod)=>{loads.push(id+':'+lod);return Promise.resolve(template());}});
harbour.setTier('high');harbour.start();await new Promise(r=>setTimeout(r,0));
assert.deepEqual(loads.sort(),['harbour-barge:1','harbour-buoy:1','harbour-launch:1','sail-skiff:1']);
const m=new THREE.Matrix4(),p=new THREE.Vector3(),at=(id,row,i=0)=>{harbour.meshes(id)[row].getMatrixAt(i,m);return m;};
// Instance matrices are float32: positions in the hundreds compare at 1e-3.
harbour.update(100,1,{x:1,z:0,strength:.5},true);const pose=craftPoses(100)[0];p.setFromMatrixPosition(at('harbour-launch',0));
assert.ok(Math.abs(p.x-pose.x)<1e-3&&Math.abs(p.z-pose.z)<1e-3&&Math.abs(p.y-(SEA_LEVEL+HEAVE*seaHeight(pose.x,pose.z,100)))<1e-3,'the launch rides the wave');
const hull=at('sail-skiff',0).clone(),rigA=at('sail-skiff',1).clone();harbour.update(100,1,{x:-1,z:0,strength:.5},true);
assert.ok(at('sail-skiff',0).equals(hull)&&!at('sail-skiff',1).equals(rigA),'the rig follows the wind, the hull does not');
assert.equal(at('harbour-buoy',2).determinant(),0,'lanterns dark by day');harbour.update(100,0,{x:1,z:0,strength:.5},false);assert.notEqual(at('harbour-buoy',2).determinant(),0,'lanterns lit at night');
let lit=0;for(let t=0;t<3;t+=.05){harbour.update(t,0,{x:1,z:0,strength:.5},true);if(at('harbour-buoy',2).determinant()!==0)lit++;}assert.ok(lit>=3&&lit<=15,'lanterns blink: '+lit);
const clones=[THREE.Matrix4,THREE.Vector3,THREE.Quaternion].map(C=>{const o=C.prototype.clone;let n=0;C.prototype.clone=function(){n++;return o.call(this);};return {C,o,count:()=>n};});
const reuse=craftPoses(5);assert.equal(craftPoses(6,reuse),reuse);for(let i=0;i<1000;i++)harbour.update(200+i/60,.3,{x:1,z:0,strength:.4},true);
for(const c of clones){assert.equal(c.count(),0,'update clones '+c.C.name);c.C.prototype.clone=c.o;}
harbour.update(300,1,{x:1,z:0,strength:.5},true);harbour.setTier('low');await new Promise(r=>setTimeout(r,0));harbour.update(400,1,{x:1,z:0,strength:.5},true);
assert.ok(loads.includes('harbour-launch:2')&&harbour.stats().lod===2,'low tier loads LOD 2');p.setFromMatrixPosition(at('harbour-launch',0));
assert.ok(Math.abs(p.x-craftPoses(300)[0].x)<1e-3,'low tier: the boats keep their place');assert.equal(harbour.stats().craft,5);
harbour.dispose();assert.equal(group.children.length,0);
console.log(`System World harbour: keep-out ${Object.entries(worst).map(([k,v])=>k+' '+v.toFixed(1)).join(', ')} m; wave ride, rig, lanterns, tiers and allocation verified.`);
