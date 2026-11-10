// Phase 4 gulls: counts per tier, paths over the harbour and around the barge, flapping wings, hidden extras.
import assert from 'node:assert/strict';
import * as THREE from 'three';
import {FLOCK,HARBOUR_SKY,gullPose,createGulls} from '../ui/js/desktop/apps/sysworld-gulls.js';
const barge={x:60,z:180},pose={};
for(let i=0;i<12;i++)for(let t=0;t<120;t+=.5){gullPose(i,t,barge,pose);const c=i%2?barge:HARBOUR_SKY,d=Math.hypot(pose.x-c.x,pose.z-c.z);
  assert.ok(d>=11.9&&d<=33.1&&pose.y>=11.9&&pose.y<=25.2&&Math.abs(pose.flap)<=.56,'gull '+i+' off its circle: '+JSON.stringify(pose));}
const box=new THREE.BoxGeometry(),mat=new THREE.MeshBasicMaterial(),root=new THREE.Group();root.add(new THREE.Mesh(box,mat));
for(const [name,z]of[['wing_l',-.06],['wing_r',.06]]){const w=new THREE.Group();w.name=name;w.position.set(0,.03,z);const mesh=new THREE.Mesh(box,mat);mesh.position.z=z*6;w.add(mesh);root.add(w);}
const group=new THREE.Group(),gulls=createGulls({group,load:()=>Promise.resolve(root)});gulls.start();await new Promise(r=>setTimeout(r,0));
const meshes=group.getObjectByName('gulls').children,m=new THREE.Matrix4(),det=(mesh,i)=>{mesh.getMatrixAt(i,m);return m.determinant();};
for(const [tier,count]of Object.entries(FLOCK)){gulls.setTier(tier);gulls.update(10,barge);assert.equal(gulls.stats().count,count,tier);
  for(let i=0;i<12;i++)assert.equal(det(meshes[0],i)!==0,i<count,tier+' gull '+i);}
gulls.setTier('high');gulls.update(0,barge);const a=new THREE.Matrix4();meshes[1].getMatrixAt(0,a);gulls.update(.2,barge);meshes[1].getMatrixAt(0,m);assert.ok(!a.equals(m),'wings move');
gulls.dispose();assert.equal(group.children.length,0);
console.log('System World gulls: tiers 0/6/12/12, harbour and barge circles, wing beats verified.');
