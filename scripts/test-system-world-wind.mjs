// Phase 4 wind: bounded, turning, gusty, weather-dependent and deterministic; owned by the atmosphere, still on low and
// without motion; banner parts turn about their pivot.
import assert from 'node:assert/strict';
import * as THREE from 'three';
import {windAt,createAtmosphere} from '../ui/js/desktop/apps/sysworld-atmosphere.js';
import {createPartInstances} from '../ui/js/desktop/apps/sysworld-parts.js';
let lo=1,hi=0;const out={};
for(let t=0;t<3600;t+=.5){windAt(t,'clear',out);lo=Math.min(lo,out.strength);hi=Math.max(hi,out.strength);assert.ok(Math.abs(Math.hypot(out.x,out.z)-1)<1e-5,'unit direction');}
assert.ok(lo>=.15&&hi<=1&&hi-lo>.3,'wind range '+lo.toFixed(2)+'..'+hi.toFixed(2));
assert.ok(windAt(100,'rain').strength>windAt(100,'clear').strength&&windAt(100,'fog').strength<windAt(100,'clear').strength,'weather');
assert.deepEqual(windAt(42.5),windAt(42.5));assert.equal(windAt(1,'clear',out),out,'writes into out');
const atmosphere=createAtmosphere(new THREE.Scene(),{}),camera=new THREE.PerspectiveCamera();
atmosphere.setTier('high');atmosphere.update(1,1,camera,true);assert.ok(atmosphere.wind().strength>0,'wind blows');
atmosphere.update(1,2,camera,false);assert.equal(atmosphere.wind().strength,0,'reduced motion stills the wind');
atmosphere.setTier('low');atmosphere.update(1,3,camera,true);assert.equal(atmosphere.wind().strength,0,'no wind on low');atmosphere.dispose();
// Parts: a mesh under `banner` turns about the banner node's origin; the rest is skipped with `only`.
const root=new THREE.Group(),mast=new THREE.Mesh(new THREE.BoxGeometry(),new THREE.MeshBasicMaterial()),banner=new THREE.Group(),cloth=new THREE.Mesh(mast.geometry,mast.material);
banner.name='banner';banner.position.set(.14,6.55,0);cloth.position.set(.5,-1.6,0);banner.add(cloth);root.add(mast,banner);
const parts=createPartInstances(root,2,{only:['banner'],movable:['banner']});assert.equal(parts.meshes.length,1,'only the cloth');
const m=new THREE.Matrix4(),p=new THREE.Vector3(),turn=new THREE.Matrix4().makeRotationY(Math.PI/2);
parts.set(0,new THREE.Matrix4(),()=>null);parts.meshes[0].getMatrixAt(0,m);p.setFromMatrixPosition(m);assert.ok(p.distanceTo(new THREE.Vector3(.64,4.95,0))<1e-5,'rest pose');
parts.set(0,new THREE.Matrix4(),()=>turn);parts.meshes[0].getMatrixAt(0,m);p.setFromMatrixPosition(m);assert.ok(p.distanceTo(new THREE.Vector3(.14,4.95,-.5))<1e-5,'turns about the pivot');
parts.hide(1);parts.meshes[0].getMatrixAt(1,m);assert.equal(m.determinant(),0,'hidden instance');parts.dispose();
console.log(`System World wind: ${lo.toFixed(2)}..${hi.toFixed(2)} strength, weather, low/reduced stillness and pivoted parts verified.`);
