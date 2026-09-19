// Pure navigation contract, shared by the renderer and deterministic tests.
import {interiors,stations,towers,surfaces,streetHeight} from './sysworld-layout.js';
export {interiors,stations,tramWaypoints,dronePad} from './sysworld-layout.js';
// Ground-level solids authored from build_city.py, not the skyline or picking radius.
// Integration gate's central passage and space between towers remain walkable.
const footprints={agent:[[-8,-8,8,8]],memory:[[-11.4,-10.4,11.4,8.4]],
  integrations:[[-11.4,-4.9,-4.6,4.9],[4.6,-4.9,11.4,4.9],[-4.8,-.8,-3.2,.8],[3.2,-.8,4.8,.8]],
  missions:[[-13.4,-8.9,13.4,6.9]],infra:[[-10.7,-9.4,10.7,3.4],[-10,-2.6,10,8.5]]};
const podiums={agent:[23,23],memory:[26,22],integrations:[26,16],missions:[30,21],infra:[25,23],graph:[24,24],operations:[9,9]};
export function buildingFloor(x,z,districts){for(const d of districts){const s=podiums[d.id];if(s&&Math.abs(x-d.x)<s[0]/2&&Math.abs(z-d.z)<s[1]/2)return .88;}return 0;}
function solidBuilding(x,z,districts){
  for(const d of districts){const dx=x-d.x,dz=z-d.z;if(d.id==='graph'&&Math.hypot(dx,dz)<10.2)return true;if(d.id==='operations'&&Math.hypot(dx,dz)<3)return true;
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
  for(const s of stations){const rotated=!!s.angle&&Math.abs(s.angle)!==Math.PI;
    if(Math.abs(x-s.platformX)<(rotated?2:4.5)&&Math.abs(z-s.platformZ)<(rotated?4.5:2))return surfaces.pavement+.3;}
  return Math.max(streetHeight(x,z),z>=74&&z<=80&&x>=-80&&x<=80?surfaces.quay:surfaces.ground);
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
  return !solidBuilding(x,z,districts);
}
export function daylight(mode,date=new Date()){
  const hour=mode==='day'?12:mode==='evening'?18.5:mode==='night'?0:date.getHours()+date.getMinutes()/60;
  return {hour,amount:Math.max(0,Math.min(1,Math.sin((hour-6)/12*Math.PI)*1.5)),evening:Math.max(0,1-Math.abs(hour-18.5)/2)};
}
