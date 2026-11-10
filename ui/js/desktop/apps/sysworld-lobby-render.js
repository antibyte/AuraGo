import * as THREE from 'three';
import {LOBBIES,DOORS,PODIUM,LOAD_DISTANCE,HIDE_DISTANCE} from './sysworld-lobbies.js';

// Lobby interiors of the enterable buildings (kit v3 `lobby-*`, category interior, authored in the building
// frame). One loads when the camera comes within LOAD_DISTANCE of one of its doors (3D, at eye height),
// hides the building's `lobby` stand-in part and leaves again beyond HIDE_DISTANCE; the decoded template
// stays in the scene's model cache. The low tier never loads interiors. The see-through glazing
// (`city.pane`) shares one transparent material owned here; leaves `door_<n>_<l|r>` slide along X by half
// the door width, following the door states of sysworld-lobbies.js (experience.lobbies).
export const PANE_MATERIAL='city.pane';
// Interior light fixtures (ceiling panels, ring lights, column slots, screens) get their own material instances so
// that their glow can follow daylight without touching the exterior's shared kit materials: full by day, a modest
// fraction at night, when they are seen through the glass at close range. The glass tint is a little denser at night.
export const FIXTURES=new Set(['city.ivory','city.warm','city.cyan','city.jade']);
export const NIGHT={glow:.28,opacity:.26},DAY={glow:1,opacity:.2};
const LEVELS={medium:1,high:0,ultra:0},LEAF=/^door_(\d+)_([lr])$/;
export const lobbyLevel=tier=>LEVELS[tier]??null;
export function createLobbyRender({group,load,doors=()=>null}){
  const holder=new THREE.Group();holder.name='lobby-interiors';group.add(holder);
  const material=new THREE.MeshStandardMaterial({name:PANE_MATERIAL,color:new THREE.Color(.03,.09,.1),roughness:.05,metalness:0,
    envMapIntensity:2.4,transparent:true,opacity:DAY.opacity,depthWrite:false,side:THREE.DoubleSide});
  const assets=[...new Set(LOBBIES.map(l=>l.asset))].map(asset=>{const halls=LOBBIES.filter(l=>l.asset===asset);
    return {asset,halls,district:halls[0].district,origin:halls[0].origin,doors:DOORS.filter(d=>d.asset===asset),node:null,level:null,pending:null,failed:false,leaves:[],kit:[]};});
  let level=0,disposed=false,glow=DAY.glow;
  // The instance keeps the shader patch of the shared material (night/day brightness of the fixtures).
  const own=m=>{const c=m.clone();c.onBeforeCompile=m.onBeforeCompile;c.customProgramCacheKey=m.customProgramCacheKey;c.userData.glow=m.emissiveIntensity;
    c.emissiveIntensity=m.emissiveIntensity*glow;return c;};
  const ready=(a,value)=>{for(const h of a.halls)doors()?.setReady(h.id,value);};
  function show(a,template,lod){
    const node=template.clone(true);node.name=a.asset;node.position.set(a.origin[0],0,a.origin[1]);
    node.traverse(n=>{const m=LEAF.exec(n.name),door=m&&a.doors.find(d=>d.n===Number(m[1]));
      if(door)a.leaves.push({node:n,key:door.key,base:n.position.x,shift:(m[2]==='l'?-1:1)*door.width/2});});
    const copies=new Map();
    node.traverse(n=>{if(!n.isMesh)return;const list=Array.isArray(n.material)?n.material:[n.material];
      const next=list.map(m=>{if(!FIXTURES.has(m?.name))return m;if(!copies.has(m)){const c=own(m);copies.set(m,c);a.kit.push(c);}return copies.get(m);});
      n.material=Array.isArray(n.material)?next:next[0];});
    holder.add(node);a.node=node;a.level=lod;ready(a,true);
  }
  function hide(a){if(!a.node)return;holder.remove(a.node);a.node=null;a.level=null;a.leaves=[];for(const m of a.kit)m.dispose();a.kit=[];ready(a,false);}
  return {
    material,owns:m=>m===material,
    // Swaps every city.pane slot for the shared glass, disposes the loaded material; glass casts no shadow.
    adopt(mesh){
      if(!mesh?.isMesh||disposed)return false;
      const list=Array.isArray(mesh.material)?mesh.material:[mesh.material];let hit=false;
      const next=list.map(m=>{if(m===material||m?.name!==PANE_MATERIAL)return m;hit=true;m.dispose();return material;});
      if(!hit)return false;mesh.material=Array.isArray(mesh.material)?next:next[0];mesh.castShadow=false;return true;
    },
    // Daylight 0 (night) .. 1 (day): glass tint and fixture glow of the loaded interiors.
    setLighting(day){
      const k=THREE.MathUtils.smoothstep(Number.isFinite(day)?day:1,0,1),mix=(a,b,t=k)=>a+(b-a)*t;
      material.opacity=mix(NIGHT.opacity,DAY.opacity);
      glow=mix(NIGHT.glow,DAY.glow,k*k);for(const a of assets)for(const m of a.kit)m.emissiveIntensity=m.userData.glow*glow;
    },
    setTier(tier){level=lobbyLevel(tier);for(const a of assets)a.failed=false;},
    // Returns true when the shadow map should update (an interior or a stand-in changed).
    update(position,landmarks){
      if(disposed)return false;let changed=false;const states=doors();
      for(const a of assets){
        const distance=Math.min(...a.doors.map(d=>Math.hypot(position.x-d.x,position.y-PODIUM-1.5,position.z-d.z)));
        const want=level!==null&&(distance<=LOAD_DISTANCE||(!!a.node&&distance<=HIDE_DISTANCE));
        if(a.node&&(!want||a.level!==level)){hide(a);changed=true;}
        if(want&&!a.node&&!a.failed&&a.pending!==level){
          const lod=level;a.pending=lod;
          load(a.asset,lod).then(template=>{if(a.pending!==lod)return;a.pending=null;if(!disposed&&level===lod&&!a.node)show(a,template,lod);},
            ()=>{if(a.pending===lod){a.pending=null;a.failed=true;}});
        }
        const standIn=landmarks?.children.find(n=>n.userData.district===a.district)?.getObjectByName('lobby');
        if(standIn&&standIn.visible===!!a.node){standIn.visible=!a.node;changed=true;}
        if(states)for(const leaf of a.leaves)leaf.node.position.x=leaf.base+leaf.shift*states.value(leaf.key);
      }
      return changed;
    },
    stats:()=>({level,shown:assets.filter(a=>a.node).map(a=>a.asset),loading:assets.filter(a=>a.pending!==null).length,failed:assets.filter(a=>a.failed).map(a=>a.asset)}),
    dispose(){if(disposed)return;for(const a of assets)hide(a);disposed=true;holder.removeFromParent();material.dispose();},
  };
}
