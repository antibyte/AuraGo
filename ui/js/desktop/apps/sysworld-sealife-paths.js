import {createRng} from '../../chat/ocean-life-boids.js';
import {streets} from './sysworld-layout.js';
import {SEA_LEVEL} from './sysworld-harbour.js';
// Sea life of the System World (data and closed-form motion; no three.js, no DOM). Every pose is a pure function of the
// clock, so reduced motion freezes the animals, a test can replay minutes of it and the survey can aim at an event.
// World metres, Y up. Depth is metres below SEA_LEVEL. The animals' frame is the chat's: +Z nose, +Y up, +X the left.
export const SEA=SEA_LEVEL,TAU=Math.PI*2,KEEP_FROM_ISLAND=2;
export const ISLAND=Object.freeze({x0:streets.minX,x1:streets.maxX,z0:streets.minZ,z1:streets.maxZ});
// scale multiplies the species length of the chat table: the city is stylised and its camera far away.
export const SCALE=Object.freeze({fish:3,shark:1.3,hammer:1.3,turtle:1.6,whale:1.3,manta:1.2,jelly:2});
export const TIER_COUNTS=Object.freeze({
  low:Object.freeze({sharks:0,hammers:0,turtles:0,whale:0,mantas:0,jellies:0}),
  medium:Object.freeze({sharks:2,hammers:1,turtles:2,whale:1,mantas:1,jellies:12}),
  high:Object.freeze({sharks:3,hammers:2,turtles:4,whale:1,mantas:2,jellies:36}),
  ultra:Object.freeze({sharks:3,hammers:2,turtles:4,whale:1,mantas:2,jellies:36}),
});
// Draw distances (metres): schools 90, big animals 320; the moon jellies are bells of well under a metre.
export const DRAW=Object.freeze({school:90,big:320,jelly:60});
// Fish are drawn by distance to the school's centre. From the water's edge (a low camera outside the island or within 12 m of its
// edge) every fish up to 60 m, every second fish up to 90 m. From inland or from high above the fish are a few pixels: every fish
// up to 25 m, every second fish up to 45 m, none beyond. Returns the stride (1 = all, 2 = every second) or 0 for none.
export const SCHOOL_LOD=Object.freeze({near:Object.freeze([60,90]),far:Object.freeze([25,45])});
export const BOIDS_STEP=.05;   // boids run at a fixed 20 Hz (10 Hz for every-second-fish schools) with the poses extrapolated in between
export function schoolStride(distance,nearWater){const r=nearWater?SCHOOL_LOD.near:SCHOOL_LOD.far;return distance<r[0]?1:distance<r[1]?2:0;}
// Fish schools: a box habitat each (x and z ranges, depth range below the surface), boids inside it. Along the quay the
// tangs, clowns and butterflies keep to the wall (z 83..92). `medium` marks the half that remains on the medium tier.
export const FISH_COUNT=Object.freeze({fish_sardine:48,fish_tang:14,fish_clown:10,fish_butterfly:10});
const habitat=(species,x,z,medium=false,depth=[.25,2.2])=>Object.freeze({species,count:FISH_COUNT[species],x:Object.freeze(x),z:Object.freeze(z),depth:Object.freeze(depth),medium});
export const HABITATS=Object.freeze([
  habitat('fish_sardine',[-72,-48],[90,106],true),habitat('fish_sardine',[-40,-16],[88,104]),habitat('fish_sardine',[-12,12],[92,110],true),habitat('fish_sardine',[16,40],[88,104]),
  habitat('fish_sardine',[44,72],[90,108],true),habitat('fish_sardine',[92,112],[-70,-40]),habitat('fish_sardine',[-118,-94],[-40,-10],true),habitat('fish_sardine',[-30,0],[-122,-100]),
  habitat('fish_tang',[-66,-50],[83,92],true),habitat('fish_tang',[-30,-14],[83,92]),habitat('fish_tang',[-2,14],[83,92],true),habitat('fish_tang',[30,46],[83,92]),habitat('fish_tang',[56,72],[83,92],true),
  habitat('fish_clown',[-58,-44],[83.5,91],true),habitat('fish_clown',[-22,-8],[83.5,91]),habitat('fish_clown',[10,24],[83.5,91],true),habitat('fish_clown',[48,62],[83.5,91]),
  habitat('fish_butterfly',[-46,-32],[83,92],true),habitat('fish_butterfly',[-6,8],[83,92]),habitat('fish_butterfly',[22,36],[83,92],true),habitat('fish_butterfly',[61,75],[83,92]),
]);
export function habitatsFor(tier){return tier==='low'?[]:tier==='medium'?HABITATS.filter(h=>h.medium):HABITATS.slice();}
// A box clear of the island footprint plus KEEP_FROM_ISLAND metres on every side.
export function clearOfIsland(x,z,margin=KEEP_FROM_ISLAND){return x[1]<ISLAND.x0-margin||x[0]>ISLAND.x1+margin||z[1]<ISLAND.z0-margin||z[0]>ISLAND.z1+margin;}
// Sea life is drawn when the camera can see water: outside the island, high above it, or near its edge. A visitor deep in
// a street sees only buildings, so nothing is simulated or submitted there (the street views gain no triangles).
export const WATER_VIEW=Object.freeze({height:15,edge:40});
export function nearWater(x,y,z){return y<WATER_VIEW.height&&Math.min(x-ISLAND.x0,ISLAND.x1-x,z-ISLAND.z0,ISLAND.z1-z)<12;}
export function waterInView(x,y,z){
  if(y>=WATER_VIEW.height)return true;
  const inside=Math.min(x-ISLAND.x0,ISLAND.x1-x,z-ISLAND.z0,ISLAND.z1-z);
  return inside<WATER_VIEW.edge;
}
// ---- Surface remap: JS twin of the vertex shader (sysworld-sealife-shaders.js, seaClip) ----
// A point below the sea plane is drawn at the place where the view ray through it crosses the plane, moved a little towards
// the camera (same screen position, surface depth), so the opaque sea never hides it but everything in front of the sea does.
export function remapBias(below,distance){return (.03+.02*Math.min(6,Math.max(0,6-below)))*(1+.01*distance);}
export function seaRemap(cx,cy,cz,px,py,pz,out){
  const below=SEA-py;
  if(below<=0){out.x=px;out.y=py;out.z=pz;return out;}
  const h=Math.max(cy-SEA,.02),k=h/Math.max(cy-py,h);
  out.x=cx+(px-cx)*k;out.y=cy+(py-cy)*k;out.z=cz+(pz-cz)*k;
  const dx=cx-out.x,dy=cy-out.y,dz=cz-out.z,len=Math.max(Math.hypot(dx,dy,dz),.001),b=remapBias(below,len)/len;
  out.x+=dx*b;out.y+=dy*b;out.z+=dz*b;return out;
}
// ---- Fish leaps ----
export const LEAP=Object.freeze({duration:.9,minGap:4,maxGap:9,apex:Object.freeze([.6,1.1]),speed:Object.freeze([2.2,3.2]),ring:1.2});
function cumulative(seed,first,lo,hi,count){const rng=createRng(seed),out=[];let t=first;for(let i=0;i<count;i++){t+=lo+(hi-lo)*rng();out.push(t);}return Object.freeze(out);}
export const LEAP_STARTS=cumulative(0x1ea95,3,LEAP.minGap,LEAP.maxGap,2400);
export const WHALE_EVENTS=cumulative(0xa11e,24,40,60,400),TURTLE_EVENTS=cumulative(0x7081e,14,30,50,700);
// Index of the first start >= t (binary search).
export function firstAtOrAfter(list,t){let lo=0,hi=list.length;while(lo<hi){const mid=(lo+hi)>>1;if(list[mid]<t)lo=mid+1;else hi=mid;}return lo;}
// The event whose window [start-before, start+after] holds t, as {k,u: t-start}, or null.
export function eventAt(list,t,before,after,out={k:0,u:0}){
  const k=firstAtOrAfter(list,t-after);
  if(k>=list.length||list[k]-before>t)return null;
  out.k=k;out.u=t-list[k];return out;
}
export function leapParams(k,out={apex:0,speed:0}){const rng=createRng((k+1)*2654435761);out.apex=LEAP.apex[0]+(LEAP.apex[1]-LEAP.apex[0])*rng();out.speed=LEAP.speed[0]+(LEAP.speed[1]-LEAP.speed[0])*rng();return out;}
// Pose at s in [0,1] of a leap that leaves the water at the origin heading (dx,dz) (unit): a parabola with its apex above the
// surface at s = .5; the nose (fx,fy,fz) follows the arc. h is metres above the surface.
export function leapPose(apex,speed,dx,dz,s,out){
  const dist=speed*LEAP.duration,u=Math.min(1,Math.max(0,s)),slope=4*apex*(1-2*u)/dist,c=1/Math.hypot(1,slope);
  out.x=dx*dist*u;out.z=dz*dist*u;out.h=4*apex*u*(1-u);out.fx=dx*c;out.fy=slope*c;out.fz=dz*c;return out;
}
// ---- Closed loops ----
// cx,cz,rx,rz: the ellipse (turned by rot); speed: nominal m/s along it; dir: +1/-1; wob: radius wobble; depth: body centre
// range below the surface; period: seconds of the depth cycle; ph: its phase. Every loop keeps clear of the island.
const loop=(o)=>Object.freeze({rot:0,wob:.04,ph:0,dir:1,period:40,...o});
export const LOOPS=Object.freeze({
  // Reef sharks patrol around the buoys and the launch route; the dorsal fin cuts the surface when the depth cycle is shallow.
  shark:Object.freeze([
    loop({cx:32,cz:122,rx:16.5,rz:30,speed:1.45,phase:.4,wob:.05,depth:[.2,1.4],period:44}),
    loop({cx:-45,cz:100,rx:38,rz:15,speed:1.65,phase:2.5,dir:-1,depth:[.2,1.4],period:49,ph:2.1}),
    loop({cx:32,cz:122,rx:31,rz:36,speed:1.75,phase:4,wob:.03,depth:[.2,1.4],period:54,ph:4.2}),
  ]),
  // Hammerheads cruise wide, 45-85 m off the shore, 1.3-3.3 m down.
  hammer:Object.freeze([
    loop({cx:0,cz:-7,rx:168,rz:168,wob:0,speed:1.5,phase:1,depth:[1.3,3.3],period:70}),
    loop({cx:0,cz:-7,rx:170,rz:168,wob:0,speed:1.7,phase:3.7,dir:-1,depth:[1.3,3.3],period:83,ph:1.7}),
  ]),
  // Turtles potter near the shores at 0.3-0.45 m/s and come up every 30-50 s (TURTLE_EVENTS, one after the other).
  turtle:Object.freeze([
    loop({cx:-104,cz:-20,rx:8,rz:18,speed:.4,phase:.3,wob:.06,depth:[.9,1.3],period:31}),
    loop({cx:26,cz:-111,rx:16,rz:8,speed:.45,phase:4.1,wob:.06,depth:[.9,1.3],period:37,ph:1.3}),
    loop({cx:101,cz:-78,rx:8,rz:13,speed:.35,phase:2.2,dir:-1,wob:.06,depth:[.9,1.3],period:29,ph:2.6}),
    loop({cx:-102,cz:38,rx:7,rz:14,speed:.3,phase:5.2,dir:-1,wob:.06,depth:[.9,1.3],period:33,ph:3.9}),
  ]),
  // The whale circles about 210 m from the island centre and surfaces every 40-60 s (WHALE_EVENTS).
  whale:Object.freeze([loop({cx:0,cz:-7,rx:250,rz:178,speed:1.5,phase:.8,wob:.02,depth:[5,6],period:90})]),
  // Mantas glide 0.6-1.6 m down, 60-120 m off the east and west shores.
  manta:Object.freeze([
    loop({cx:-190,cz:0,rx:32,rz:62,speed:1.4,phase:.5,wob:.03,depth:[.6,1.6],period:36}),
    loop({cx:190,cz:-12,rx:36,rz:55,speed:1.2,phase:3,dir:-1,wob:.03,depth:[.6,1.6],period:41,ph:1.9}),
  ]),
});
export const SURFACING=Object.freeze({
  turtle:Object.freeze({rise:1.5,hold:4,fall:1.5,depth:.12}),   // shell and head break the surface for about 4 s
  whale:Object.freeze({rise:4,hold:3,fall:3.5,depth:.9,cruise:5.5}),   // back out of the water, the blow, then the dive
});
export function loopPoint(L,t,out){
  const mean=(L.rx+L.rz)/2,th=L.phase+L.dir*L.speed*t/mean,w=1+L.wob*Math.sin(3*th+L.phase*2),c=Math.cos(L.rot),s=Math.sin(L.rot),x=L.rx*Math.cos(th)*w,z=L.rz*Math.sin(th)*w;
  out.x=L.cx+x*c-z*s;out.z=L.cz+x*s+z*c;return out;
}
// Mean speed (m/s) along a loop: Ramanujan perimeter over the period of one turn.
export function loopSpeed(L){const a=L.rx,b=L.rz,perimeter=Math.PI*(3*(a+b)-Math.sqrt((3*a+b)*(a+3*b)));return perimeter/(TAU*((a+b)/2)/L.speed);}
const smooth=(a,b,x)=>{const u=Math.min(1,Math.max(0,(x-a)/(b-a)));return u*u*(3-2*u);};
// Depth of the body centre below the surface for animal i of a kind at time t. n: how many of the kind exist (the
// surfacing turtle is event k modulo n).
const hit={k:0,u:0};
export function depthAt(kind,i,t,n=1){
  const L=LOOPS[kind][i],base=(L.depth[0]+L.depth[1])/2+(L.depth[1]-L.depth[0])/2*Math.sin(TAU*t/L.period+L.ph);
  if(kind==='turtle'){const S=SURFACING.turtle,e=eventAt(TURTLE_EVENTS,t,S.rise,S.hold+S.fall,hit);
    if(!e||e.k%n!==i)return base;
    const up=smooth(-S.rise,0,e.u)*(1-smooth(S.hold,S.hold+S.fall,e.u));return base+(S.depth-base)*up;}
  if(kind==='whale'){const S=SURFACING.whale,e=eventAt(WHALE_EVENTS,t,S.rise,S.hold+S.fall,hit);
    if(!e)return S.cruise;
    const up=smooth(-S.rise,0,e.u)*(1-smooth(S.hold,S.hold+S.fall,e.u));return S.cruise+(S.depth-S.cruise)*up;}
  return base;
}
// Position (world) of animal i at time t.
export function seaPosition(kind,i,t,n,out){
  loopPoint(LOOPS[kind][i],t,out);out.y=SEA-depthAt(kind,i,t,n);return out;
}
const pa={x:0,y:0,z:0},pb={x:0,y:0,z:0},pc={x:0,y:0,z:0};
// Full pose: position, forward (the heading of the motion, so a dive tilts the nose down) and roll into the turn.
export function seaPose(kind,i,t,n,out){
  seaPosition(kind,i,t-.1,n,pa);seaPosition(kind,i,t,n,pb);seaPosition(kind,i,t+.1,n,pc);
  out.x=pb.x;out.y=pb.y;out.z=pb.z;
  let fx=pc.x-pa.x,fy=pc.y-pa.y,fz=pc.z-pa.z;const l=Math.hypot(fx,fy,fz)||1;out.fx=fx/l;out.fy=fy/l;out.fz=fz/l;
  const turn=Math.atan2(pc.x-pb.x,pc.z-pb.z)-Math.atan2(pb.x-pa.x,pb.z-pa.z),wrapped=Math.atan2(Math.sin(turn),Math.cos(turn));
  out.roll=Math.max(-.35,Math.min(.35,-wrapped*5));return out;
}
// Blow of the surfaced whale: five soft puffs from t0+.8 s, each rising about 4 m and fading within 2.5 s. Returns the puff's
// age (0..1) and fills out with its position, or -1 when it is not alive.
export const BLOW=Object.freeze({count:5,first:.8,gap:.35,life:2.5,rise:4,length:15.6});
const pose={x:0,y:0,z:0,fx:0,fy:0,fz:1,roll:0},bhit={k:0,u:0};
export function blowPuff(t,i,out){
  const e=eventAt(WHALE_EVENTS,t,0,BLOW.first+BLOW.gap*BLOW.count+BLOW.life,bhit);
  if(!e)return -1;
  const elapsed=e.u-(BLOW.first+BLOW.gap*i),age=elapsed/BLOW.life;
  if(age<0||age>1)return -1;
  seaPose('whale',0,t-elapsed,1,pose);
  const h=Math.hypot(pose.fx,pose.fz)||1,r=createRng(e.k*131+i*17+5),sway=(r()-.5)*1.4;
  out.x=pose.x+pose.fx/h*BLOW.length*.2+sway*(.3+age);out.z=pose.z+pose.fz/h*BLOW.length*.2+(r()-.5)*1.4*(.3+age);
  out.y=SEA+.15+BLOW.rise*Math.pow(age,.75);out.size=1.2+3.2*age;
  return age;
}
// Moon jellies drift in the quay band and about the buoys, 0.3-1.5 m deep; everything seeded.
export const JELLY_HOMES=(()=>{const rng=createRng(0x7e11f),out=[];for(let i=0;i<36;i++){
  const quay=i%3!==2,x=quay?-70+140*rng():18+28*rng(),z=quay?84+24*rng():96+54*rng();
  out.push(Object.freeze({x,z,phase:rng()*TAU,rate:.03+.04*rng(),reach:2.5+3*rng(),pulse:rng(),freq:.32+.18*rng(),size:.8+.45*rng(),tilt:rng()*TAU}));}return Object.freeze(out);})();
export function jellyPosition(i,t,out){
  const J=JELLY_HOMES[i];out.x=J.x+J.reach*Math.sin(t*J.rate+J.phase)+.8*Math.sin(t*J.rate*2.7+J.phase*2);
  out.z=Math.max(ISLAND.z1+1.5,J.z+J.reach*.8*Math.cos(t*J.rate*.8+J.phase)+.6*Math.sin(t*J.rate*3.1));   // never nearer than 1.5 m to the quay wall
  out.y=SEA-(.9+.6*Math.sin(t*.07+J.phase*1.7));return out;
}
