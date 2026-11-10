// Pure navigation contract, shared by the renderer and deterministic tests.
import {interiors,stations,towers,surfaces} from './sysworld-layout.js';
import {streetProfile} from './sysworld-ground.js';
import {PODIUM,podiumAt,lobbyGround,stepFree,lobbyPassage} from './sysworld-lobbies.js';
export {interiors,stations,tramWaypoints,dronePad} from './sysworld-layout.js';
// Ground-level solids of what is not enterable. Phase 3 replaced the six enterable buildings by lobby walls
// with door gaps (sysworld-lobbies.js); the integration gate keeps the two posts of its central passage.
export const footprints={integrations:[[-4.8,-.8,-3.2,.8],[3.2,-.8,4.8,.8]]};
// Stepped upper solids [half width, from, to] following build_city.py's agent spire
// (tiers plus corner struts), so the crown terrace and the east lift shaft stay free.
export const upperSolids={agent:[[5.9,8,13],[5.4,13,29],[4.7,29,42.5],[4.2,42.5,61.5],[3.6,61.5,72.2],[2.6,72.2,87]]};
export function buildingFloor(x,z){return podiumAt(x,z)?PODIUM:0;}
function solidBuilding(x,z,districts){
  for(const d of districts){const dx=x-d.x,dz=z-d.z;if(d.id==='operations'&&Math.hypot(dx,dz)<3)return true;
    if((footprints[d.id]||[]).some(([x0,z0,x1,z1])=>dx>x0&&dx<x1&&dz>z0&&dz<z1))return true;}
  return towers.some(t=>Math.abs(x-t.x)<6&&Math.abs(z-t.z)<5);
}
export function roomAt(x,z){return interiors.find(r=>Math.abs(x-r.x)<r.width/2&&Math.abs(z-r.z)<r.depth/2);}
export function groundHeight(x,z){
  // Promenade ramp rises gently to the raised footbridge, with a stair route beside it.
  if(x>=-81.5&&x<=-78.5&&z>=46&&z<=52)return (52-z)/3;
  if(x>=-78.3&&x<=-75.3&&z>=46&&z<=52)return Math.min(2,Math.ceil((52-z)*2)/6);
  if(x>=-82&&x<=-75&&z>=34&&z<46)return 2;
  if(x>=-80&&x<=-77&&z>=28&&z<34)return (z-28)/3;
  if(roomAt(x,z))return surfaces.room;
  const lifted=lobbyGround(x,z);if(lifted!==null)return lifted;
  for(const s of stations){const rotated=!!s.angle&&Math.abs(s.angle)!==Math.PI;
    if(Math.abs(x-s.platformX)<(rotated?2:4.5)&&Math.abs(z-s.platformZ)<(rotated?4.5:2))return surfaces.pavement+.3;}
  const street=streetProfile(x,z);if(street)return street.height;
  return z>=74&&z<=80&&x>=-80&&x<=80?surfaces.quay:surfaces.ground;
}
export function canWalk(x,z,from,doorOpen,districts){
  if(Math.abs(x)>84||z < -90||z > 79)return false;
  const next=roomAt(x,z),prev=roomAt(from.x,from.z);
  if(next||prev){
    const r=next||prev;
    if(next?.id!==prev?.id){
      // Only the 3.2 m door opening connects the outside and the room.
      if(Math.abs(x-r.x)>1.15||Math.abs(from.x-r.x)>1.15||Math.min((z-r.doorZ)*r.front,(from.z-r.doorZ)*r.front)<-.9||!doorOpen(r.id))return false;
    }
    if(next && (Math.abs(x-r.x)>5.55||(z-r.z)*r.front < -5.55))return false;
    return true;
  }
  // No step above 0.35 m (podium edges close everywhere but at the stairs), no lobby wall, doors only when open.
  if(!stepFree(from,{x,z},groundHeight)||!lobbyPassage(from,{x,z},doorOpen))return false;
  return !solidBuilding(x,z,districts);
}
export function daylight(mode,date=new Date()){
  const hour=mode==='day'?12:mode==='evening'?18.5:mode==='night'?0:date.getHours()+date.getMinutes()/60;
  return {hour,amount:Math.max(0,Math.min(1,Math.sin((hour-6)/12*Math.PI)*1.5)),evening:Math.max(0,1-Math.abs(hour-18.5)/2)};
}
