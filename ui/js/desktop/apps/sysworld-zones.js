// Sound zones (System World beauty pass, phase 5). Pure: where the listener stands (harbour, street canyon, green, open
// plaza, height above the roofs), the room it is in (lobbies of sysworld-lobbies.js, waterfront pavilions of
// sysworld-layout.js), the nearest door and its openness, and the surface under a foot. World metres, Y up. Every
// sound layer reads these values; none measures its own distances. Outdoor values are shares (harbour + canyon +
// green + plaza + height = 1); `quay` is the raw harbour proximity; `interior` is 0 or 1 (smoothed by the tracker).
// Green and canyon come from fields sampled every ZONE.cell metres, built once on first use, so zoneWeights,
// doorState and the tracker never allocate. scripts/test-system-world-zones.mjs fixes the weights at known points.
import {streets,interiors,stations,towers} from './sysworld-layout.js';
import {streetProfile} from './sysworld-ground.js';
import {LOBBIES,DOORS,SOLID_TOP,lobbyAt,lobbyGround,podiumAt} from './sysworld-lobbies.js';
import {SURFACES,furnishing} from './sysworld-furnishing.js';
import {roomAt} from './sysworld-exploration.js';

export const ZONE=Object.freeze({quayZ:79,quayHalf:80,harbourNear:6,harbourReach:36,canyonHeight:24,canyonNear:3,canyonReach:24,canyonPair:1.6,
  plotStreet:.6,lawnReach:14,gardenNear:3,gardenReach:12,courtReach:12,treeRadius:10,treeCount:6,treeShare:.7,heightFrom:14,heightTo:50,
  pavilionTop:12,cell:4,outdoorTau:.8,interiorTau:.3,leakReach:12});
export const TOWER_HEIGHT=38.79;
// The seven districts of sysworld-scene.js `districts` (x, z, height, radius; checked by the test) and the data towers.
export const BUILDINGS=Object.freeze([[0,-12,87,13],[-43,-53,24,18],[43,-10,34,14],[43,36,23,15],[-43,-10,37,14],[-43,36,28,17],[0,36,22,6],
  ...towers.map(t=>[t.x,t.z,TOWER_HEIGHT*t.scale,7.5])].map(([x,z,height,radius])=>Object.freeze({x,z,height,radius})));
export const GARDENS=Object.freeze([[-79,68],[-79,-67],[55,4],[8,40],[45,-23]]);
export const SURFACE_KINDS=Object.freeze(['road','pavement','kerb','stairs','quay','lobby','grating','garden']);
// Rooms: tone (sysworld-sound-rooms.js ROOM_TONES), reverb space (sysworld-sound-space.js ROOM_SPACES), open to the sky.
export const ROOMS=Object.freeze({
  'lobby:agent':{tone:'agent',space:'lobby',open:false},'lobby:memory':{tone:'archive',space:'hall',open:false},
  'lobby:integrations-west':{tone:'gate',space:'small',open:false},'lobby:integrations-east':{tone:'gate',space:'small',open:false},
  'lobby:missions':{tone:'departure',space:'hall',open:false},'lobby:infra':{tone:'machine',space:'machine',open:false},
  'lobby:graph':{tone:'court',space:'lobby',open:true},
  'pavilion:agent':{tone:'pavilion',space:'small',open:false},'pavilion:memory':{tone:'pavilion',space:'small',open:false},
  'pavilion:missions':{tone:'pavilion',space:'small',open:false},
});
const LOBBY_ROOM=new Map(LOBBIES.map(l=>[l.id,'lobby:'+l.id])),PAVILION_ROOM=new Map(interiors.map(r=>[r.id,'pavilion:'+r.id]));
const COURT=LOBBIES.find(l=>l.open).circle,LAWNS=SURFACES.filter(s=>s.kind==='lawn'),BRIDGE=[-82,28,-75,52];
const DOOR_LIST=Object.freeze([...DOORS.map(d=>({key:d.key,room:'lobby:'+d.lobby,x:d.x,z:d.z})),...interiors.map(r=>({key:'pavilion:'+r.id,room:'pavilion:'+r.id,x:r.x,z:r.doorZ}))]);
const ROOM_DOORS=new Map(Object.keys(ROOMS).map(room=>[room,DOOR_LIST.filter(d=>d.room===room)]));
const SHARES=['harbour','canyon','green','plaza','height'];
const clamp01=v=>v<0?0:v>1?1:v;
// Stereo position of a source at (dx, dz) from the camera facing (fx, fz): the formula of sysworld-audio.js reach().
export function bearingPan(dx,dz,fx,fz){const d=Math.hypot(dx,dz);return d<1e-6?0:Math.max(-1,Math.min(1,(dx*-fz+dz*fx)/Math.max(1,d)));}
export function interiorAt(x,y,z){
  if(y<SOLID_TOP){const l=lobbyAt(x,z);if(l)return LOBBY_ROOM.get(l.id);}
  if(y<ZONE.pavilionTop){const r=roomAt(x,z);if(r)return PAVILION_ROOM.get(r.id);}
  return null;
}
function harbourRaw(x,z){const dx=Math.max(0,Math.abs(x)-ZONE.quayHalf),dz=Math.max(0,ZONE.quayZ-z),v=clamp01(1-(Math.hypot(dx,dz)-ZONE.harbourNear)/ZONE.harbourReach);return v*Math.sqrt(v);}
function onLawn(x,z){for(let i=0;i<LAWNS.length;i++){const s=LAWNS[i];if(x>=s.x0&&x<=s.x1&&z>=s.z0&&z<=s.z1)return true;}return false;}
function streetShare(x,z){if(streetProfile(x,z)||lobbyGround(x,z)!==null)return 1;return z>streets.quayZ||onLawn(x,z)?0:ZONE.plotStreet;}
let treeCache=null;const trees=()=>treeCache||(treeCache=furnishing().filter(f=>f.asset.startsWith('tree-')));
// Exact values (the fields sample these): lawns, gardens, the garden court and street-tree density; tall buildings on
// both sides of a street.
export function greenAt(x,z){
  let g=0;
  for(const s of LAWNS){const dx=Math.max(s.x0-x,0,x-s.x1),dz=Math.max(s.z0-z,0,z-s.z1);g=Math.max(g,clamp01(1-Math.hypot(dx,dz)/ZONE.lawnReach));}
  for(const [gx,gz]of GARDENS)g=Math.max(g,clamp01(1-Math.max(0,Math.hypot(x-gx,z-gz)-ZONE.gardenNear)/ZONE.gardenReach));
  g=Math.max(g,clamp01(1-Math.max(0,Math.hypot(x-COURT.x,z-COURT.z)-COURT.radius)/ZONE.courtReach));
  let n=0;for(const t of trees())if(Math.hypot(t.x-x,t.z-z)<ZONE.treeRadius)n++;
  return Math.max(g,clamp01(n/ZONE.treeCount)*ZONE.treeShare);
}
export function canyonAt(x,z){
  let e1=0,e2=0;
  for(const b of BUILDINGS){const edge=Math.max(0,Math.hypot(x-b.x,z-b.z)-b.radius),e=clamp01(b.height/ZONE.canyonHeight)*clamp01(1-Math.max(0,edge-ZONE.canyonNear)/ZONE.canyonReach);
    if(e>e1){e2=e1;e1=e;}else if(e>e2)e2=e;}
  return clamp01((e1+e2)/ZONE.canyonPair)*streetShare(x,z);
}
const GX0=-120,GZ0=-130,GNX=61,GNZ=63;let fields=null;
function buildFields(){const green=new Float32Array(GNX*GNZ),canyon=new Float32Array(GNX*GNZ);
  for(let j=0;j<GNZ;j++)for(let i=0;i<GNX;i++){const x=GX0+i*ZONE.cell,z=GZ0+j*ZONE.cell;green[j*GNX+i]=greenAt(x,z);canyon[j*GNX+i]=canyonAt(x,z);}
  return fields={green,canyon};}
function sample(field,x,z){const fx=(x-GX0)/ZONE.cell,fz=(z-GZ0)/ZONE.cell;if(!(fx>=0&&fz>=0&&fx<GNX-1&&fz<GNZ-1))return 0;
  const i=Math.floor(fx),j=Math.floor(fz),u=fx-i,v=fz-j,k=j*GNX+i;return (field[k]*(1-u)+field[k+1]*u)*(1-v)+(field[k+GNX]*(1-u)+field[k+GNX+1]*u)*v;}
// Listener at (x, y, z) (camera), resting on (ox, oz) (the acoustic origin); writes into `out`.
export function zoneWeights(x,y,z,ox=x,oz=z,out={}){
  const f=fields||buildFields(),h=clamp01((y-ZONE.heightFrom)/(ZONE.heightTo-ZONE.heightFrom)),quay=harbourRaw(ox,oz);
  let harbour=quay,canyon=sample(f.canyon,ox,oz),green=sample(f.green,ox,oz),sum=harbour+canyon+green;
  if(sum>1){harbour/=sum;canyon/=sum;green/=sum;sum=1;}
  const ground=1-h;out.harbour=harbour*ground;out.canyon=canyon*ground;out.green=green*ground;out.plaza=(1-sum)*ground;out.height=h;out.quay=quay;
  const room=interiorAt(x,y,z),r=room?ROOMS[room]:null;out.room=room;out.interior=room?1:0;out.open=r?r.open:false;out.space=r?r.space:null;out.tone=r?r.tone:null;
  out.x=ox;out.z=oz;return out;
}
// Openness of the listener's room (largest of its doors) or, outside, of the nearest door within ZONE.leakReach.
// valueOf(key) -> 0..1 for 'lobby:<id>:<n>' and 'pavilion:<id>' keys.
export function doorState(x,y,z,valueOf,out={}){
  out.open=0;out.leakRoom=null;out.leakDistance=Infinity;out.leakX=0;out.leakZ=0;
  const room=interiorAt(x,y,z);out.room=room;if(typeof valueOf!=='function')return out;
  if(room){const doors=ROOM_DOORS.get(room);for(let i=0;i<doors.length;i++)out.open=Math.max(out.open,clamp01(Number(valueOf(doors[i].key))||0));return out;}
  if(y>=SOLID_TOP)return out;
  let best=null;for(let i=0;i<DOOR_LIST.length;i++){const d=DOOR_LIST[i],dist=Math.hypot(x-d.x,z-d.z);if(dist<ZONE.leakReach&&dist<out.leakDistance){out.leakDistance=dist;best=d;}}
  if(best){out.leakRoom=best.room;out.leakX=best.x;out.leakZ=best.z;out.open=clamp01(Number(valueOf(best.key))||0);}
  return out;
}
// The surface under a foot at floor height y (called once per footfall).
export function surfaceAt(x,y,z){
  if(y>40)return 'grating';
  if(y<SOLID_TOP){const l=lobbyAt(x,z);if(l)return l.id==='infra'?'grating':l.open?'garden':'lobby';}
  if(roomAt(x,z))return 'lobby';
  if(x>=BRIDGE[0]&&x<=BRIDGE[2]&&z>=BRIDGE[1]&&z<=BRIDGE[3])return 'grating';
  if(lobbyGround(x,z)!==null)return podiumAt(x,z)?'lobby':'stairs';
  for(const s of stations){const turned=!!s.angle&&Math.abs(s.angle)!==Math.PI;if(Math.abs(x-s.platformX)<(turned?2:4.5)&&Math.abs(z-s.platformZ)<(turned?4.5:2))return 'pavement';}
  const p=streetProfile(x,z);if(p)return p.kind==='road'?'road':p.kind==='kerb'?'kerb':'pavement';
  if(z>=streets.quayZ&&z<=80.5&&Math.abs(x)<=80)return 'quay';
  if(onLawn(x,z))return 'garden';
  for(const [gx,gz]of GARDENS)if(Math.hypot(x-gx,z-gz)<2.2)return 'garden';
  return 'pavement';
}
// Smoothed weights for the reverb sends, the glass and the event table. Shares settle with ZONE.outdoorTau, the room
// with ZONE.interiorTau; the room id stays until the interior weight has faded. Also: door openness, the leak of the
// nearest open door outside (tone, space, pan). `update` returns the same object every time.
export function createZoneTracker(){
  const w={harbour:0,canyon:0,green:0,plaza:1,height:0,quay:0,interior:0,room:null,space:null,tone:null,open:false,door:0,leak:0,leakRoom:null,leakTone:null,leakSpace:null,leakPan:0};
  let first=true;
  return {weights:w,
    update(raw,door,dt,listener){
      const ko=first?1:1-Math.exp(-Math.max(0,dt)/ZONE.outdoorTau),ki=first?1:1-Math.exp(-Math.max(0,dt)/ZONE.interiorTau);first=false;
      for(let i=0;i<SHARES.length;i++){const k=SHARES[i];w[k]+=(raw[k]-w[k])*ko;}
      w.quay+=(raw.quay-w.quay)*ko;w.interior+=(raw.interior-w.interior)*ki;
      if(raw.room){w.room=raw.room;w.space=raw.space;w.tone=raw.tone;w.open=raw.open;}else if(w.interior<.02){w.room=null;w.space=null;w.tone=null;w.open=false;}
      w.door=door?door.open:0;
      const leak=door&&door.leakRoom&&!raw.room?door.open*clamp01(1-door.leakDistance/ZONE.leakReach):0,r=leak>0?ROOMS[door.leakRoom]:null;
      w.leak=r?leak:0;w.leakRoom=r?door.leakRoom:null;w.leakTone=r?r.tone:null;w.leakSpace=r?r.space:null;
      w.leakPan=r&&listener?bearingPan(door.leakX-listener.x,door.leakZ-listener.z,listener.fx,listener.fz):0;
      return w;
    },
    reset(){first=true;},
  };
}
