import * as THREE from 'three';
import {createPartInstances} from './sysworld-parts.js';
// A small flock (phase 4): even gulls circle the harbour mouth, odd ones circle the barge. Body and wings are instanced;
// matrices come from closed-form paths of the atmosphere clock (frozen under reduced motion). 12 on high/ultra, 6 on
// medium, none on low. The gull flies towards +X; wing_l (glTF -Z) and wing_r hinge about X at the shoulders.
export const FLOCK=Object.freeze({low:0,medium:6,high:12,ultra:12}),HARBOUR_SKY=Object.freeze({x:0,z:104});
export function gullPose(i,t,barge,out){
  const follow=i%2===1,cx=follow?barge.x:HARBOUR_SKY.x,cz=follow?barge.z:HARBOUR_SKY.z,dir=i%3===0?-1:1;
  const r=(follow?12:24)+(i*3.7)%9,w=dir*(follow?.42:.26)*(1+(i%4)*.06),a=w*t+i*1.9;
  out.x=cx+r*Math.cos(a);out.z=cz+r*Math.sin(a);out.y=(follow?13:17)+(i*2.3)%7+1.1*Math.sin(t*.55+i);
  out.heading=Math.atan2(-r*Math.cos(a)*w,-r*Math.sin(a)*w);out.bank=-Math.sign(w)*.32;
  // Three wing beats, then a glide with the wings raised a little.
  const cycle=(t*.21+i*.37)%1;out.flap=cycle<.3?.55*Math.sin(cycle/.3*Math.PI*6):.08;
  return out;
}
export function createGulls({group,load}){
  const holder=new THREE.Group();holder.name='gulls';group.add(holder);
  const pose={x:0,y:0,z:0,heading:0,bank:0,flap:0},m=new THREE.Matrix4(),wing=new THREE.Matrix4(),e=new THREE.Euler(),q=new THREE.Quaternion(),v=new THREE.Vector3(),one=new THREE.Vector3(1,1,1);
  let parts=null,tier='high',started=false,disposed=false,flap=0;
  const turn=name=>name==='wing_l'?wing.makeRotationX(flap):name==='wing_r'?wing.makeRotationX(-flap):null;
  return {
    start(){if(started)return;started=true;load('gull',1).then(t=>{if(disposed)return;parts=createPartInstances(t,FLOCK.ultra,{movable:['wing_l','wing_r']});parts.meshes.forEach(x=>{x.castShadow=false;holder.add(x);});},()=>{});},
    setTier(value){tier=value;},
    update(time,barge){
      if(!parts)return;const count=FLOCK[tier]??0;
      for(let i=0;i<FLOCK.ultra;i++){
        if(i>=count){parts.hide(i);continue;}
        gullPose(i,time,barge,pose);flap=pose.flap;e.set(pose.bank,pose.heading,0,'YZX');q.setFromEuler(e);v.set(pose.x,pose.y,pose.z);parts.set(i,m.compose(v,q,one),turn);
      }
      parts.commit();
    },
    stats:()=>({count:parts?FLOCK[tier]??0:0,loaded:!!parts}),
    dispose(){disposed=true;parts?.dispose();holder.removeFromParent();},
  };
}
