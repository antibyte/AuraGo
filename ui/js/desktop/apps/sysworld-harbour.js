import * as THREE from 'three';
import {seaHeight} from './sysworld-atmosphere.js';
import {createPartInstances} from './sysworld-parts.js';

// Harbour traffic (phase 4): a launch loops off the south quay, a barge crawls a long stadium 100-150 m out, three
// skiffs circle east of the island with the rig swinging to leeward, six buoys mark the channel and blink at night.
// Positions are closed-form functions of the atmosphere clock (craftPoses), so reduced motion freezes them and a test
// can replay ten minutes. The sea is flat geometry with shaded waves: hulls heave HEAVE x the shader height and tilt
// with its full slope. Boats keep KEEP_OUT from the island, the tidal turbine and each other. Low tier: LOD 2, travel stops.
export const SEA_LEVEL=-3.2,HEAVE=.25,KEEP_OUT=12;
export const CRAFT=Object.freeze([
  {id:'launch',asset:'harbour-launch',length:9,beam:2.9,speed:4.5,route:{kind:'ellipse',x:-45,z:100,rx:28,rz:6}},
  {id:'barge',asset:'harbour-barge',length:22,beam:6.2,speed:2.6,route:{kind:'stadium',x:0,z:205,half:200,radius:25}},
  {id:'skiff-0',asset:'sail-skiff',length:5.4,beam:1.9,speed:2.2,route:{kind:'circle',x:112,z:-40,r:9}},
  {id:'skiff-1',asset:'sail-skiff',length:5.4,beam:1.9,speed:-2.6,route:{kind:'circle',x:125,z:5,r:11}},
  {id:'skiff-2',asset:'sail-skiff',length:5.4,beam:1.9,speed:2,route:{kind:'circle',x:110,z:50,r:9}},
].map(c=>Object.freeze(c)));
export const BUOYS=Object.freeze([[25,100],[39,100],[25,122],[39,122],[25,144],[39,144]]);
const TAU=Math.PI*2,SLOT=CRAFT.map((c,i)=>CRAFT.slice(0,i).filter(o=>o.asset===c.asset).length);
const routeLength=r=>r.kind==='circle'?TAU*r.r:r.kind==='ellipse'?Math.PI*(3*(r.rx+r.rz)-Math.sqrt((3*r.rx+r.rz)*(r.rx+3*r.rz))):4*r.half+TAU*r.radius;
// Pose at arc length s on a closed route: x, z and heading (bow +X; heading h points the bow to (cos h, -sin h)).
export function routePose(r,s,out){
  const L=routeLength(r);let u=((s%L)+L)%L,a;
  if(r.kind!=='stadium'){
    const rx=r.kind==='circle'?r.r:r.rx,rz=r.kind==='circle'?r.r:r.rz;a=u/L*TAU;
    out.x=r.x+rx*Math.cos(a);out.z=r.z+rz*Math.sin(a);out.heading=Math.atan2(-rz*Math.cos(a),-rx*Math.sin(a));return out;
  }
  const leg=2*r.half,turn=Math.PI*r.radius;
  if(u<leg){out.x=r.x-r.half+u;out.z=r.z-r.radius;out.heading=0;return out;}
  u-=leg;
  if(u<turn){a=-Math.PI/2+u/r.radius;out.x=r.x+r.half+r.radius*Math.cos(a);}
  else{u-=turn;if(u<leg){out.x=r.x+r.half-u;out.z=r.z+r.radius;out.heading=Math.PI;return out;}u-=leg;a=Math.PI/2+u/r.radius;out.x=r.x-r.half+r.radius*Math.cos(a);}
  out.z=r.z+r.radius*Math.sin(a);out.heading=Math.atan2(-Math.cos(a),-Math.sin(a));return out;
}
export function craftPoses(t,out=CRAFT.map(()=>({x:0,z:0,heading:0}))){
  for(let i=0;i<CRAFT.length;i++){routePose(CRAFT[i].route,CRAFT[i].speed*t,out[i]);if(CRAFT[i].speed<0)out[i].heading+=Math.PI;}
  return out;
}
const e=new THREE.Euler(),q=new THREE.Quaternion(),v=new THREE.Vector3(),one=new THREE.Vector3(1,1,1);
// Hull matrix on the sea: heave from the height, pitch along the keel, roll across the beam (all from seaHeight).
function ride(m,x,z,heading,length,beam,t){
  const c=Math.cos(heading),s=Math.sin(heading),fx=c*length*.4,fz=-s*length*.4,sx=s*beam*.5,sz=c*beam*.5;
  const pitch=Math.atan2(seaHeight(x+fx,z+fz,t)-seaHeight(x-fx,z-fz,t),length*.8),roll=-Math.atan2(seaHeight(x+sx,z+sz,t)-seaHeight(x-sx,z-sz,t),beam);
  e.set(roll,heading,pitch,'YZX');q.setFromEuler(e);v.set(x,SEA_LEVEL+HEAVE*seaHeight(x,z,t),z);return m.compose(v,q,one);
}
export function createHarbour({group,load}){
  const holder=new THREE.Group();holder.name='harbour';group.add(holder);
  const poses=craftPoses(0),m=new THREE.Matrix4(),turn=new THREE.Matrix4(),zero=new THREE.Matrix4().makeScale(0,0,0);
  let tier='high',lod=null,parts=new Map(),travel=0,started=false,disposed=false,rigAngle=0,lampOn=false;
  const rigTurn=name=>name==='rig'?turn.makeRotationY(rigAngle):null,lampTurn=name=>name==='lamp'&&!lampOn?zero:null;
  async function build(level){
    lod=level;const ids=[...new Set([...CRAFT.map(c=>c.asset),'harbour-buoy'])],loaded=await Promise.allSettled(ids.map(id=>load(id,level)));
    if(disposed||lod!==level)return;
    for(const p of parts.values())p.dispose();holder.clear();parts=new Map();
    ids.forEach((id,i)=>{if(loaded[i].status!=='fulfilled')return;
      const p=createPartInstances(loaded[i].value,id==='sail-skiff'?3:id==='harbour-buoy'?BUOYS.length:1,{movable:['rig','lamp']});
      p.meshes.forEach(x=>{x.castShadow=false;holder.add(x);});parts.set(id,p);});
  }
  return {
    start(){if(started)return;started=true;void build(tier==='low'?2:1);},
    setTier(value){tier=value;const level=tier==='low'?2:1;if(started&&level!==lod)void build(level);},
    // time: the atmosphere clock (frozen under reduced motion); day: 1 day .. 0 night; wind: atmosphere.wind().
    update(time,day,wind,animated=true){
      if(tier!=='low')travel=time;craftPoses(travel,poses);
      for(let i=0;i<CRAFT.length;i++){const c=CRAFT[i],p=parts.get(c.asset);if(!p)continue;const pose=poses[i];
        // The boom swings to leeward: towards the side the wind blows to, further in gusts.
        const across=wind.x*Math.sin(pose.heading)+wind.z*Math.cos(pose.heading);rigAngle=(across>=0?1:-1)*(.35+.4*wind.strength)+.06*Math.sin(time*1.3+i);
        p.set(SLOT[i],ride(m,pose.x,pose.z,pose.heading,c.length,c.beam,time),rigTurn);}
      const buoys=parts.get('harbour-buoy');
      if(buoys)for(let i=0;i<BUOYS.length;i++){lampOn=day<.5&&(!animated||(time+i*.5)%3<.45);buoys.set(i,ride(m,BUOYS[i][0],BUOYS[i][1],i*.9,1.6,1.6,time),lampTurn);}
      for(const p of parts.values())p.commit();
    },
    barge:()=>poses[1],
    meshes:id=>parts.get(id)?.meshes||[],
    stats:()=>({craft:CRAFT.filter(c=>parts.has(c.asset)).length,buoys:parts.has('harbour-buoy')?BUOYS.length:0,lod,travel:+travel.toFixed(1)}),
    dispose(){disposed=true;for(const p of parts.values())p.dispose();parts.clear();holder.removeFromParent();},
  };
}
