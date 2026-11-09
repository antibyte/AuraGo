// System World footing: spring response, stance weight, two-bone solve and foot adaptation on a
// synthetic rig with the resident proportions (robot_assets.py, scaled 1.15 like the runtime) and on
// the shipped resident models.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import * as THREE from 'three';
import {GLTFLoader} from 'three/addons/loaders/GLTFLoader.js';
import {springStep,stanceWeight,solveLeg,findRig,adaptFeet,soleLift,SOLE} from '../ui/js/desktop/apps/sysworld-footing.js';

// --- Spring ---
{
  const dt=1/60,s={y:0,v:0};let peak=0;
  for(let i=0;i<36;i++){springStep(s,.14,dt);peak=Math.max(peak,s.y);}
  assert.ok(peak<=.14+1e-12,'critically damped: no overshoot');assert.ok(Math.abs(s.y-.14)<.001,'settles within 0.6 s: '+s.y);
  const a={y:0,v:0},b={y:0,v:0};for(let i=0;i<6;i++)springStep(a,.14,dt);springStep(b,.14,.1);
  assert.ok(Math.abs(a.y-b.y)<1e-9&&Math.abs(a.v-b.v)<1e-9,'frame-rate independent');
  const mid={y:0,v:0};springStep(mid,.14,.05);assert.ok(mid.y>.01&&mid.y<.13,'eases instead of snapping: '+mid.y);
  const jump={y:0,v:0};springStep(jump,2,dt);assert.equal(jump.y,2,'teleports snap');
  const still={y:0,v:0};springStep(still,.14,0);assert.equal(still.y,.14,'reduced motion snaps');
  const unset={y:NaN,v:0};springStep(unset,.3,dt);assert.equal(unset.y,.3,'an unset spring starts at its target');
}
// --- Stance weight ---
assert.equal(stanceWeight(0),1);assert.equal(stanceWeight(.01),1);assert.equal(stanceWeight(.1),0);
assert.ok(stanceWeight(.04)>stanceWeight(.05)&&stanceWeight(.05)>stanceWeight(.06),'monotonic');
// --- Two-bone solve ---
{
  const hip=new THREE.Vector3(0,1,0),knee=new THREE.Vector3(0,.5,0),ankle=new THREE.Vector3(0,0,0),target=new THREE.Vector3(0,.14,0);
  const r=solveLeg(hip,knee,ankle,target,new THREE.Vector3(1,0,0));assert.ok(r);
  const u=knee.clone().sub(hip),v=ankle.clone().sub(knee);
  const k2=hip.clone().add(u.clone().applyQuaternion(r.hip)),a2=k2.clone().add(v.clone().applyQuaternion(r.knee).applyQuaternion(r.hip));
  assert.ok(a2.distanceTo(target)<1e-6,'ankle reaches the target');assert.ok(k2.z>.05,'the knee bends forward (+Z)');
  assert.ok(Math.abs(k2.distanceTo(hip)-.5)<1e-9&&Math.abs(a2.distanceTo(k2)-.5)<1e-9,'bone lengths kept');
  assert.equal(solveLeg(hip,hip.clone(),ankle,target,new THREE.Vector3(1,0,0)),null,'degenerate leg');
}
// --- Synthetic rig: structure at the hip, legs straight down, ankles 0.13 m above the soles ---
function rig(){
  const root=new THREE.Group();root.scale.setScalar(1.15);
  const structure=new THREE.Object3D();structure.name='structure';structure.position.set(0,1.07,0);root.add(structure);
  for(const [tag,s]of[['l',-1],['r',1]]){
    const leg=new THREE.Object3D();leg.name='leg_'+tag;leg.position.set(s*.165,0,0);structure.add(leg);
    const shin=new THREE.Object3D();shin.name='shin_'+tag;shin.position.set(0,-.47,0);leg.add(shin);
    const foot=new THREE.Object3D();foot.name='foot_'+tag;foot.position.set(0,-.47,0);shin.add(foot);
  }
  return root;
}
const soles=foot=>SOLE.map(p=>new THREE.Vector3(...p).applyMatrix4(foot.matrixWorld));
{
  const root=rig(),r=findRig(root);assert.ok(r&&r.legs.length===2,'complete rig');
  assert.ok(Math.abs(soleLift(r.legs[0].foot,root))<1e-9,'rest soles on the body origin');
  const before=r.legs.map(l=>l.leg.quaternion.clone()),res=adaptFeet(r,root,()=>0);
  assert.equal(res.drop,0);assert.ok(r.legs.every((l,i)=>l.leg.quaternion.equals(before[i])),'flat ground changes nothing');
}
{ // a kerb under the left foot only
  const root=rig(),r=findRig(root),ground=x=>x<0?.14:0;adaptFeet(r,root,ground);root.updateMatrixWorld(true);
  assert.ok(Math.abs(Math.min(...soles(r.legs[0].foot).map(p=>p.y))-.14)<.005,'left foot planted on the kerb');
  assert.ok(Math.abs(Math.min(...soles(r.legs[1].foot).map(p=>p.y)))<.005,'right foot stays on the road');
  assert.ok(r.legs[0].shin.getWorldPosition(new THREE.Vector3()).z>.02,'left knee bends forward');
}
{ // stepping down: the spring still holds the body 14 cm up
  const root=rig();root.position.y=.14;const r=findRig(root),res=adaptFeet(r,root,()=>0);root.updateMatrixWorld(true);
  assert.ok(Math.abs(res.drop-.14)<1e-6,'the pelvis lowers');
  for(const l of r.legs)assert.ok(Math.abs(Math.min(...soles(l.foot).map(p=>p.y)))<.005,'both soles reach the road');
}
{ // a 1:10 ramp rising forward
  const root=rig(),r=findRig(root),ground=(x,z)=>.1*z;adaptFeet(r,root,ground);root.updateMatrixWorld(true);
  for(const l of r.legs){const gap=soles(l.foot).map(p=>p.y-ground(p.x,p.z));
    assert.ok(Math.min(...gap)>-.003,'no contact below the ramp');assert.ok(Math.min(...gap)<.005,'the foot touches the ramp');}
}
{ // random walk-like poses over road and kerb: no sole ends below the ground
  let seed=7,clamped=0;const rnd=()=>(seed=(Math.imul(seed,1664525)+1013904223)>>>0)/4294967296;
  for(let n=0;n<300;n++){
    const root=rig(),r=findRig(root);root.position.y=-.05+rnd()*.25;
    for(const l of r.legs){const hip=(rnd()-.5)*.8,flex=rnd()*.9;l.leg.rotation.x=hip;l.shin.rotation.x=flex;l.foot.rotation.x=-(hip+flex);}
    const gl=rnd()<.5?0:.14,gr=rnd()<.5?0:.14,ground=x=>x<0?gl:gr,res=adaptFeet(r,root,ground);root.updateMatrixWorld(true);
    if(res.raised.some(v=>v>=.35-1e-9)||res.drop>=.25-1e-9){clamped++;continue;}
    for(const l of r.legs)for(const p of soles(l.foot))assert.ok(p.y>=ground(p.x,p.z)-.003,'a sole below the ground: '+JSON.stringify({n,p}));
  }
  assert.ok(clamped<15,'limits rarely reached: '+clamped);
}
{ // incomplete or foreign rigs disable the feet
  const missing=rig();missing.getObjectByName('foot_r').removeFromParent();assert.equal(findRig(missing),null);
  assert.equal(findRig(new THREE.Group()),null);
  const odd=rig();odd.getObjectByName('foot_l').position.y=-.3;assert.equal(findRig(odd),null,'soles away from the origin are rejected');
}
// --- The shipped resident models ---
const loader=new GLTFLoader();
for(const name of['courier','technician','archivist']){
  const data=fs.readFileSync(new URL(`../ui/3d/system-world/v2/robot-${name}.lod0.glb`,import.meta.url));
  const gltf=await loader.parseAsync(data.buffer.slice(data.byteOffset,data.byteOffset+data.byteLength),'');
  const root=new THREE.Group();root.add(gltf.scene);root.scale.setScalar(1.15);root.updateMatrixWorld(true);
  const r=findRig(root);assert.ok(r,name+': findRig accepts the real model');
  const rest=r.legs.flatMap(l=>[l.leg,l.shin,l.foot]).map(n=>n.quaternion.clone());
  const flat=adaptFeet(r,root,()=>0);root.updateMatrixWorld(true);
  assert.ok(flat.drop===0&&flat.raised.every(v=>v<5e-4),name+': flat ground changes nothing');
  r.legs.flatMap(l=>[l.leg,l.shin,l.foot]).forEach((n,i)=>assert.ok(n.quaternion.angleTo(rest[i])<1e-6,name+': pose unchanged on flat ground'));
  for(const l of r.legs)for(const p of soles(l.foot))assert.ok(Math.abs(p.y)<.001,name+': soles within 1 mm of the road');
  // a 14 cm kerb under the left foot only (left foot is at x < 0)
  const ground=x=>x<0?.14:0;
  adaptFeet(r,root,ground);root.updateMatrixWorld(true);
  const left=Math.min(...soles(r.legs[0].foot).map(p=>p.y)),right=Math.min(...soles(r.legs[1].foot).map(p=>p.y));
  assert.ok(left>=.14-.005,name+': left sole not below the kerb: '+left);
  assert.ok(left<=.14+.01,name+': left sole on the kerb: '+left);
  assert.ok(Math.abs(right)<.005,name+': right foot still reaches its ground: '+right);
}
console.log('System World footing: spring, stance weight, two-bone solve, kerb, step-down, ramp, 300 random poses, rig detection and 3 shipped residents passed.');
