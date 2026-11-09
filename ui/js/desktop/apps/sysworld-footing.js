// Pure footing for the resident robots: a critically damped height spring, an analytic two-bone leg
// solve, a stance weight and the foot adaptation that keeps both soles on the ground profile.
// Three.js maths only; no scene state. A rig that is not complete disables the feet, never the walker.
import * as THREE from 'three';

// Rig of assets/system-world/robot_assets.py as glTF nodes: structure (pelvis) -> leg_x (hip) ->
// shin_x (knee, a pure local-X hinge) -> foot_x (ankle). Sole contact points in the foot frame from
// HEEL/TOE (forward, up) = (-.085, -.13) and (.27, -.13); glTF axes are (x, up, forward).
export const LEGS=[['leg_l','shin_l','foot_l'],['leg_r','shin_r','foot_r']];
export const SOLE=[[0,-.13,-.085],[0,-.13,.27]];

// Exact critically damped spring step; frame-rate independent. dt <= 0 (reduced motion), an unset
// state or a jump above `snap` (teleport, stairs) snaps.
export function springStep(state,target,dt,omega=14,snap=.6){
  if(!(dt>0)||!Number.isFinite(state.y)||Math.abs(target-state.y)>snap){state.y=target;state.v=0;return target;}
  const x=state.y-target,e=Math.exp(-omega*dt),t=(state.v+omega*x)*dt;
  state.v=(state.v-omega*t)*e;state.y=target+(x+t)*e;return state.y;
}

// 1 while the animated sole is on its ground plane (lift <= 2.5 cm), 0 in swing (lift >= 7 cm).
export function stanceWeight(lift,low=.025,high=.07){
  const t=Math.min(1,Math.max(0,(lift-low)/(high-low)));return 1-t*t*(3-2*t);
}

const _u=new THREE.Vector3(),_v=new THREE.Vector3(),_n=new THREE.Vector3(),_c=new THREE.Vector3(),_m=new THREE.Vector3(),_t=new THREE.Vector3();
const solved={hip:new THREE.Quaternion(),knee:new THREE.Quaternion()};
// World-space two-bone solve. Returns {hip, knee}: rotations to pre-multiply onto the thigh's and the
// shin's world orientation (knee first, then hip). The returned object is reused by the next call.
export function solveLeg(hip,knee,ankle,target,hinge){
  _u.subVectors(knee,hip);_v.subVectors(ankle,knee);
  const l1=_u.length(),l2=_v.length();
  if(l1<1e-6||l2<1e-6||hinge.lengthSq()<1e-12)return null;
  _n.copy(hinge).normalize();
  const reach=Math.min(l1+l2-1e-4,Math.max(Math.abs(l1-l2)+1e-4,target.distanceTo(hip)));
  const current=Math.atan2(_c.crossVectors(_u,_v).dot(_n),_u.dot(_v));   // signed knee bend about the hinge
  const inner=Math.acos(Math.max(-1,Math.min(1,(l1*l1+l2*l2-reach*reach)/(2*l1*l2))));
  const desired=(Math.PI-inner)*(current<0?-1:1);                          // straight legs bend forward
  solved.knee.setFromAxisAngle(_n,desired-current);
  _m.copy(_v).applyQuaternion(solved.knee).add(_u);_t.subVectors(target,hip);
  if(_m.lengthSq()<1e-12||_t.lengthSq()<1e-12)return null;
  solved.hip.setFromUnitVectors(_m.normalize(),_t.normalize());
  return solved;
}

const _a=new THREE.Vector3(),_b=new THREE.Vector3(),_hip=new THREE.Vector3(),_knee=new THREE.Vector3(),_ankle=new THREE.Vector3(),_target=new THREE.Vector3(),_hinge=new THREE.Vector3();
const _p=new THREE.Quaternion(),_q=new THREE.Quaternion(),_f=new THREE.Quaternion();
function contacts(foot){_a.fromArray(SOLE[0]).applyMatrix4(foot.matrixWorld);_b.fromArray(SOLE[1]).applyMatrix4(foot.matrixWorld);}
// Height of the lower sole contact above the body origin (matrices must be current).
export function soleLift(foot,root){contacts(foot);return Math.min(_a.y,_b.y)-root.matrixWorld.elements[13];}
function penetration(foot,ground){contacts(foot);return Math.max(ground(_a.x,_a.z)-_a.y,ground(_b.x,_b.z)-_b.y);}
function rotateWorld(node,q){node.parent.getWorldQuaternion(_p);node.quaternion.premultiply(_p).premultiply(q).premultiply(_p.invert());}

// {structure, legs:[{leg,shin,foot}]} or null when anything is missing or the rest soles do not
// stand on the body origin (then these contact points do not describe the model).
export function findRig(root){
  const structure=root.getObjectByName('structure');if(!structure)return null;
  const legs=[];
  for(const names of LEGS){
    const [leg,shin,foot]=names.map(n=>root.getObjectByName(n));
    if(!leg||!shin||!foot||shin.parent!==leg||foot.parent!==shin)return null;
    legs.push({leg,shin,foot});
  }
  root.updateMatrixWorld(true);
  for(const l of legs)if(Math.abs(soleLift(l.foot,root))>.04)return null;
  return {structure,legs};
}

// Runs after the mixer has posed the clip and the body has its final position for this step.
// Lowers the body until the lowest stance foot reaches the ground, then bends each knee so no
// sole stays below the ground. Writes root.position.y and the leg/shin/foot quaternions only.
export function adaptFeet(rig,root,ground,{maxDrop=.25,maxRaise=.35}={}){
  root.updateMatrixWorld(true);
  const feet=rig.legs.map(l=>({l,lift:soleLift(l.foot,root),pen:penetration(l.foot,ground)}));
  let drop=0;for(const f of feet)drop=Math.max(drop,stanceWeight(f.lift)*Math.max(0,-f.pen));
  drop=Math.min(drop,maxDrop);if(drop<1e-4)drop=0;
  if(drop){root.position.y-=drop;root.updateMatrixWorld(true);}
  const raised=[];
  for(const f of feet){
    const raise=Math.min(maxRaise,f.pen+drop);raised.push(Math.max(0,raise));
    if(raise<=5e-4)continue;
    const {leg,shin,foot}=f.l;
    leg.getWorldPosition(_hip);shin.getWorldPosition(_knee);foot.getWorldPosition(_ankle);
    _target.copy(_ankle);_target.y+=raise;_hinge.setFromMatrixColumn(leg.matrixWorld,0);
    const s=solveLeg(_hip,_knee,_ankle,_target,_hinge);if(!s)continue;
    foot.getWorldQuaternion(_f);
    rotateWorld(shin,s.knee);rotateWorld(leg,s.hip);leg.updateMatrixWorld(true);
    shin.getWorldQuaternion(_q);foot.quaternion.copy(_q.invert().multiply(_f));foot.updateMatrixWorld(true);
  }
  return {drop,raised};
}
