// MIT. Logical grid coordinates and elevation remain independent of screen pixels.
export const ISO = Object.freeze({tile_width:128,tile_height:64,height_step:32});
function finite(v) { if (!Number.isFinite(v)) throw new Error('Isometric coordinates must be finite'); return v; }
export function projectIso([x,y,z=0], origin={x:0,y:0}) {
  return {x:finite((x-y)*64)+origin.x,y:finite((x+y)*32-z*32)+origin.y};
}
export function unprojectIso(x,y,z=0,origin={x:0,y:0}) {
  const a=finite((x-origin.x)/64), b=finite((y-origin.y+z*32)/32);
  return [(a+b)/2,(b-a)/2,z];
}
export function isoDirection(dx,dy) {
  if (!dx&&!dy) return null;
  const screen=projectIso([dx,dy,0]);
  return ['right','down-right','down','down-left','left','up-left','up','up-right'][(Math.round(Math.atan2(screen.y,screen.x)*4/Math.PI)+8)%8];
}
export function isoDepth([x,y,z=0], layer=0) { return finite((x+y)*1024+z*32+layer); }
const cellKey=(x,y)=>`${Math.floor(x)},${Math.floor(y)}`;

// No RAF, timers, physics engine or independent input owner. Call update from
// the existing game loop; the host owns sprites, camera, pause and input.
export function createIsometricWorld(scene, host={}) {
  if (scene.schema_version!==2 || scene.dimension!=='2d' || scene.projection?.kind!=='isometric') throw new Error('Expected scene schema 2 with isometric projection');
  const levels=scene.levels.filter(l=>l.active);
  if(levels.length!==1) throw new Error('Exactly one active isometric level is required');
  let level=levels[0].id,disposed=false,debug=false,debugListener;
  const records=new Map(), floors=new Map(), blocked=new Map(), links=new Map(),solidCells=new Map();
  const block=(key,id)=>{if(!blocked.has(key))blocked.set(key,new Set());blocked.get(key).add(id);if(!solidCells.has(id))solidCells.set(id,new Set());solidCells.get(id).add(key);};
  const unblock=id=>{for(const [key,owners] of blocked){owners.delete(id);if(!owners.size)blocked.delete(key);}};
  const active=n=>!n.level_id||n.level_id===level;
  const origin=host.origin||{x:0,y:0};
  function rebuild() {
    floors.clear();blocked.clear();links.clear();solidCells.clear();
    let cells=0;
    for(const n of scene.nodes.filter(active)) {
      const [x,y,z=0]=n.position, size=n.properties?.footprint||[1,1];
      if(!n.position.every(Number.isFinite)||!size.every(v=>Number.isFinite(v)&&v>0&&v<=256))throw new Error('Invalid isometric footprint');
      cells+=Math.ceil(size[0])*Math.ceil(size[1]);if(cells>65536)throw new Error('Isometric grid exceeds 65536 cells');
      for(let i=0;i<Math.ceil(size[0]);i++)for(let j=0;j<Math.ceil(size[1]);j++) {
        const key=cellKey(x+i,y+j);
        if(n.properties?.walkable) {
          if(floors.has(key)&&floors.get(key)!==z) throw new Error('Overlapping walkable elevations are unsupported');
          floors.set(key,z);
        }
        if(n.properties?.solid)block(key,n.id);
      }
    }
    const byID=new Map(scene.nodes.filter(active).map(n=>[n.id,n]));
    for(const r of scene.routes||[]) {
      if(!['stairs','ramp'].includes(r.kind)||r.level_id&&r.level_id!==level)continue;
      const a=byID.get(r.from),b=byID.get(r.to);if(!a?.properties?.walkable||!b?.properties?.walkable)throw new Error('Elevation links require walkable floor endpoints');
      const ak=cellKey(...a.position),bk=cellKey(...b.position);
      if(Math.abs(Math.floor(a.position[0])-Math.floor(b.position[0]))+Math.abs(Math.floor(a.position[1])-Math.floor(b.position[1]))!==1)throw new Error('Elevation links must connect adjacent floor cells');
      links.set(ak+'>'+bk,true);links.set(bk+'>'+ak,true);
    }
    for(const collider of scene.colliders||[]) {
      const node=byID.get(collider.node_id);if(!node||node.kind==='player'||node.properties?.dynamic||node.properties?.walkable)continue;
      const p=node.position,e=collider.extents,o=collider.offset||[0,0,0];
      for(let x=Math.floor(p[0]+o[0]-e[0]);x<Math.ceil(p[0]+o[0]+e[0]);x++)for(let y=Math.floor(p[1]+o[1]-e[1]);y<Math.ceil(p[1]+o[1]+e[1]);y++) {
        if(++cells>65536)throw new Error('Isometric colliders exceed grid limit');block(cellKey(x,y),node.id);
      }
    }
  }
  function canMove(a,b) {
    const ak=cellKey(...a),bk=cellKey(...b);
    if(blocked.has(bk)||!floors.has(bk))return false;
    if(ak===bk)return true;
    const ax=Math.floor(a[0]),ay=Math.floor(a[1]),bx=Math.floor(b[0]),by=Math.floor(b[1]);
    if(Math.abs(ax-bx)+Math.abs(ay-by)!==1)return false;
    return floors.get(ak)===floors.get(bk)||links.has(ak+'>'+bk);
  }
  function paint(record) {
    const size=record.node.properties?.footprint||[1,1],point=record.node.properties?.walkable?[record.at[0]+size[0]/2,record.at[1]+size[1]/2,record.at[2]]:record.at;
    const at=projectIso(point,origin);
    host.position?.(record.object,at,isoDepth(record.at,record.node.properties?.layer||0));
  }
  function loadLevel(id) {
    if(!scene.levels.some(l=>l.id===id))throw new Error('Unknown level '+id);
    for(const r of records.values())host.destroy?.(r.object);
    records.clear();level=id;rebuild();
    for(const node of scene.nodes.filter(active)) {
      const record={node,at:[...node.position],object:host.create?.(node)};
      records.set(node.id,record);paint(record);
    }
  }
  loadLevel(level);
  // Preview diagnostics require the Studio parent and its per-frame channel.
  // Standalone exports never enable this through query parameters alone.
  if(typeof window!=='undefined'&&window.parent!==window){
    const channel=()=>new URLSearchParams(window.location.hash.slice(1)).get('gm-channel')||String(globalThis.__AURAGO_STUDIO_CHANNEL__||'');
    debug=Boolean(channel())&&new URLSearchParams(window.location.hash.slice(1)).get('gm-debug')==='1';
    debugListener=event=>{const d=event.data,origin=globalThis.__AURAGO_STUDIO_ORIGIN__;if(event.source!==window.parent||!channel()||d?.source!=='aurago-studio'||d?.type!=='scene-debug'||d.channel!==channel()||typeof d.enabled!=='boolean'||origin&&event.origin!==origin)return;debug=d.enabled;host.debugChanged?.(debug)};
    window.addEventListener('message',debugListener);
  }
  return {
    records,project:at=>projectIso(at,origin),unproject:(x,y,z)=>unprojectIso(x,y,z,origin),canMove,
    remove(id) {const r=records.get(id);if(!r)return;if(r.node.properties?.walkable)throw new Error('Change walkable floors through a validated scene revision');host.destroy?.(r.object);records.delete(id);unblock(id);solidCells.delete(id);},
    setSolid(id,solid) {const r=records.get(id);if(!r)throw new Error('Unknown node '+id);unblock(id);if(solid){if(solidCells.has(id)){for(const key of solidCells.get(id))block(key,id);}else{const size=r.node.properties?.footprint||[1,1];for(let x=0;x<Math.ceil(size[0]);x++)for(let y=0;y<Math.ceil(size[1]);y++)block(cellKey(r.at[0]+x,r.at[1]+y),id);}}},
    level:id=>{if(!disposed)loadLevel(id)},
    move(id,dx,dy) {
      if(disposed)return false;
      const r=records.get(id);if(!r)throw new Error('Unknown node '+id);
      finite(dx);finite(dy);
      // Bounded substeps stop tunnelling and diagonal corner cutting.
      const steps=Math.ceil(Math.max(Math.abs(dx),Math.abs(dy))*8)||1;
      if(steps>128)throw new Error('Movement exceeds per-update bound');
      let moved=false;
      for(let i=0;i<steps;i++)for(const [axis,delta] of [[0,dx/steps],[1,dy/steps]]) {
        if(!delta)continue;const next=[...r.at];next[axis]+=delta;
        if(canMove(r.at,next)){next[2]=floors.get(cellKey(...next));r.at=next;moved=true;}
      }
      paint(r);if(moved)host.moved?.(r);return moved;
    },
    pick(x,y) {
      // Highest visible candidate first; inverse projection must use its height.
      const candidates=[...records.values()].filter(r=>r.node.properties?.walkable).sort((a,b)=>isoDepth(b.at)-isoDepth(a.at));
      return candidates.find(r=>{const at=unprojectIso(x,y,r.at[2],origin),size=r.node.properties?.footprint||[1,1];return at[0]>=r.at[0]&&at[0]<r.at[0]+size[0]&&at[1]>=r.at[1]&&at[1]<r.at[1]+size[1]})?.node.id||null;
    },
    update() {
      if(disposed)return;
      for(const r of records.values())paint(r);
    },
    render() {if(debug&&!disposed)host.renderDebug?.(records);},
    hideRoof(region,hidden=true) { for(const r of records.values())if(r.node.region_id===region&&r.node.properties?.roof)host.visible?.(r.object,!hidden); },
    inspect() {return {level,objects:records.size,floors:floors.size,blocked:blocked.size,elevation_links:links.size/2};},
    dispose() {if(disposed)return;disposed=true;if(debugListener)window.removeEventListener('message',debugListener);for(const r of records.values())host.destroy?.(r.object);records.clear();floors.clear();blocked.clear();links.clear();solidCells.clear();}
  };
}
