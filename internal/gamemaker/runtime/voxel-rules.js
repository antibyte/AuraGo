// MIT. Authoritative single-player transactions; no renderer or test-only shortcuts.
import { VoxelWorld, noise } from './voxel-world.js';
export class VoxelGame {
  constructor(definition) {
    this.definition=definition;this.world=new VoxelWorld(definition);
    this.items=new Map(definition.items.map(i=>[i.id,i]));this.recipes=new Map((definition.recipes||[]).map(r=>[r.id,r]));
    this.inventory=Array(36).fill(null);this.selected=0;this.progress={};this.revision=0;this.time=0;
    this.player={...this.world.spawn(),yaw:0,pitch:0,health:100,vy:0,grounded:false};
    this.spawn={x:this.player.x,y:this.player.y,z:this.player.z};this.mining=null;this.attackClock=0;this.bullets=[];this.events=[];this.eventSequence=0;
    this.metrics={mined:0,placed:0,crafted:0,pickups:0,hits:0,defeated:0,jumps:0,respawns:0,distance:0};
    this.enemies=[];
    for(const type of definition.enemies||[])for(let i=0;i<type.count;i++){
      const index=this.enemies.length,angle=index*2.399,range=10+noise(this.world.seed+41,index,0)*12;
      const x=Math.max(1.5,Math.min(definition.size[0]-1.5,this.spawn.x+Math.sin(angle)*range)),z=Math.max(1.5,Math.min(definition.size[2]-1.5,this.spawn.z+Math.cos(angle)*range));
      this.enemies.push({id:`${type.id}:${i}`,type,position:{x,y:this.world.height(Math.floor(x),Math.floor(z))+1.02,z},health:type.health,vy:0,cooldown:1});
    }
    if(definition.mode==='creative')for(const item of definition.items.filter(i=>i.block).slice(0,9))this.credit(item.id,item.stack);
  }
  get alive(){return this.player.health>0;}
  get creative(){return this.definition.mode==='creative';}
  held(){return this.items.get(this.inventory[this.selected]?.item);}
  eye(){return {x:this.player.x,y:this.player.y+1.62,z:this.player.z};}
  direction(){const p=this.player;return [-Math.sin(p.yaw)*Math.cos(p.pitch),Math.sin(p.pitch),-Math.cos(p.yaw)*Math.cos(p.pitch)];}
  aim(){return this.world.ray(this.eye(),this.direction(),6);}
  changed(){this.revision++;}
  count(id){return this.inventory.reduce((n,s)=>n+(s?.item===id?s.count:0),0);}
  // Preflight on a copy, then commit all ingredients and outputs together.
  transaction(take={},give={}) {
    const next=this.inventory.map(s=>s?{...s}:null);
    if(Object.keys(take).length>64||Object.keys(give).length>64)return false;
    for(const [id,count] of Object.entries(take)){
      if(!this.items.has(id)||!Number.isInteger(count)||count<1||count>35964)return false;
      let left=count;
      for(let i=0;i<next.length&&left;i++)if(next[i]?.item===id){const n=Math.min(left,next[i].count);next[i].count-=n;left-=n;if(!next[i].count)next[i]=null;}
      if(left)return false;
    }
    for(const [id,count] of Object.entries(give)){
      const item=this.items.get(id);if(!item||!Number.isInteger(count)||count<1||count>35964)return false;
      let left=count;
      for(let i=0;i<next.length&&left;i++)if(next[i]?.item===id){const n=Math.min(left,item.stack-next[i].count);next[i].count+=n;left-=n;}
      for(let i=0;i<next.length&&left;i++)if(!next[i]){const n=Math.min(left,item.stack);next[i]={item:id,count:n};left-=n;}
      if(left)return false;
    }
    this.inventory=next;this.changed();return true;
  }
  credit(id,count=1){return this.transaction({}, {[id]:count});}
  supply(id){const item=this.items.get(id);if(!this.creative||!item)return false;this.inventory[this.selected]={item:id,count:item.stack};this.changed();return true;}
  record(kind,item,count=1){const key=kind+(item?':'+item:'');this.progress[key]=Math.min(1000000,(this.progress[key]||0)+count);this.changed();}
  event(kind,data={}){this.events.push({kind,...data,time:this.time,sequence:++this.eventSequence});if(this.events.length>48)this.events.shift();}
  craft(id) {
    if(!this.alive)return false;const r=this.recipes.get(id);if(!r)return false;
    const before=this.inventory.map(s=>s?{...s}:null);
    if(!this.transaction(this.creative?{}:r.ingredients,{[r.item]:r.count}))return false;
    this.metrics.crafted++;this.record('craft',r.item,r.count);this.event('craft',{recipe:id,before,after:this.inventory.map(s=>s?{...s}:null)});return true;
  }
  swap(slot){if(!Number.isInteger(slot)||slot<0||slot>=36)return false;[this.inventory[this.selected],this.inventory[slot]]=[this.inventory[slot],this.inventory[this.selected]];this.changed();return true;}
  place() {
    if(!this.alive)return false;const target=this.aim(),item=this.held();if(!target||!item?.block)return false;
    const cell=target.previous,old=this.world.get(...cell);if(old||!this.world.inside(...cell)||cell[1]===0)return false;
    const overlap=(p,width=.6,height=1.8)=>p.x+width/2>cell[0]&&p.x-width/2<cell[0]+1&&p.y+height>cell[1]&&p.y<cell[1]+1&&p.z+width/2>cell[2]&&p.z-width/2<cell[2]+1;
    if(overlap(this.player)||this.enemies.some(e=>e.health>0&&overlap(e.position,.7,1.4)))return false;
    const before=this.count(item.id);if(!this.creative&&!this.transaction({[item.id]:1},{}))return false;
    if(!this.world.set(...cell,item.block)){if(!this.creative)this.credit(item.id);return false;}
    this.metrics.placed++;this.record('place',item.id);this.event('place',{cell,old:0,block:item.block,item:item.id,before,after:this.count(item.id)});return true;
  }
  enemyTarget(range=4) {
    const origin=this.eye(),dir=this.direction(),wall=this.world.ray(origin,dir,range);let result=null,best=wall?wall.distance:range;
    for(const e of this.enemies){if(e.health<=0)continue;const p=e.position;let near=0,far=best;
      for(let axis=0;axis<3;axis++){
        const o=[origin.x,origin.y,origin.z][axis],lo=[p.x-.4,p.y,p.z-.4][axis],hi=[p.x+.4,p.y+1.5,p.z+.4][axis];
        if(Math.abs(dir[axis])<1e-8){if(o<lo||o>hi){near=Infinity;break;}continue;}
        let a=(lo-o)/dir[axis],b=(hi-o)/dir[axis];if(a>b)[a,b]=[b,a];near=Math.max(near,a);far=Math.min(far,b);
      }
      if(near<=far&&near<best){best=near;result=e;}
    }
    return result;
  }
  primary(dt) {
    if(!this.alive)return;
    const enemy=this.enemyTarget();
    if(enemy){this.mining=null;if(this.attackClock>0)return;this.attackClock=.35;
      const before=enemy.health;enemy.health=Math.max(0,enemy.health-(this.held()?.damage||2));this.metrics.hits++;this.changed();this.event('hit',{id:enemy.id,before,after:enemy.health});
      if(!enemy.health){this.metrics.defeated++;this.record('defeat','');if(this.credit(enemy.type.drop)){this.metrics.pickups++;this.record('collect',enemy.type.drop);}this.event('defeat',{id:enemy.id});}return;
    }
    const target=this.aim();if(!target||target.cell[1]===0){this.mining=null;return;}
    const block=this.world.blocks.get(target.id);if(!this.creative&&(this.held()?.tier||0)<block.tier){this.mining=null;return;}
    const key=target.cell.join(',');if(this.mining?.key!==key)this.mining={key,elapsed:0};this.mining.elapsed+=dt;
    if(this.mining.elapsed<(this.creative ? .12 : block.hardness))return;
    const before=this.count(block.drop);
    if(!this.credit(block.drop)){this.mining=null;return;}
    if(!this.world.set(...target.cell,0))throw new Error('Voxel mining transaction lost its target');
    this.mining=null;this.metrics.mined++;this.metrics.pickups++;this.record('collect',block.drop);
    this.event('mine',{cell:target.cell,block:target.id,afterBlock:0,item:block.drop,before,after:this.count(block.drop)});
  }
  damage(amount) {
    if(this.creative||!this.alive||!Number.isFinite(amount)||amount<0)return;
    const before=this.player.health;this.player.health=Math.max(0,before-amount);this.changed();this.event('damage',{before,after:this.player.health});
    if(!this.alive){this.mining=null;this.event('death');}
  }
  respawn() {
    if(this.alive)return false;
    Object.assign(this.player,this.world.spawn(),{health:100,vy:0});this.metrics.respawns++;this.changed();this.event('respawn');return true;
  }
  tick(dt,input={}) {
    dt=Math.min(.05,Math.max(0,dt));this.time+=dt;this.attackClock=Math.max(0,this.attackClock-dt);
    if(!this.alive)return;
    const p=this.player,old={x:p.x,y:p.y,z:p.z},yaw=p.yaw;
    let x=(input.right||0)-(input.left||0),z=(input.backward||0)-(input.forward||0),length=Math.max(1,Math.hypot(x,z));
    x/=length;z/=length;
    const dx=(x*Math.cos(yaw)+z*Math.sin(yaw))*5*dt,dz=(-x*Math.sin(yaw)+z*Math.cos(yaw))*5*dt;
    if(input.jump&&p.grounded){p.vy=7;this.metrics.jumps++;this.event('jump',{from:p.y});}
    p.vy=Math.max(-25,p.vy-20*dt);const blocked=this.world.move(p,{x:dx,y:p.vy*dt,z:dz});
    p.grounded=this.world.collides({...p,y:p.y-.04});if(blocked.y)p.vy=0;
    const travelled=Math.hypot(p.x-old.x,p.y-old.y,p.z-old.z);this.metrics.distance+=travelled;if(travelled>.00001)this.changed();
    if(input.primary)this.primary(dt);else this.mining=null;
    this.tickEnemies(dt);
  }
  tickEnemies(dt) {
    if(this.creative)return;
    const p=this.player;
    for(const e of this.enemies){if(e.health<=0)continue;const pos=e.position,old={...pos},dx=p.x-pos.x,dz=p.z-pos.z,distance=Math.hypot(dx,dz);e.cooldown=Math.max(0,e.cooldown-dt);
      e.vy=Math.max(-25,e.vy-20*dt);
      const chase=e.type.behavior==='melee'&&distance<20&&distance>1.2;
      const blocked=this.world.move(pos,{x:chase?dx/distance*2.2*dt:0,y:e.vy*dt,z:chase?dz/distance*2.2*dt:0},.7,1.4);
      if(Math.hypot(pos.x-old.x,pos.y-old.y,pos.z-old.z)>.00001)this.changed();
      if(blocked.y)e.vy=0;if(chase&&(blocked.x||blocked.z)&&this.world.collides({...pos,y:pos.y-.04},.7,1.4))e.vy=6;
      if(e.cooldown||distance> (e.type.behavior==='melee'?1.7:14))continue;
      const origin={x:pos.x,y:pos.y+1.1,z:pos.z},delta=[dx,p.y+1-origin.y,dz],length=Math.hypot(...delta);
      if(this.world.ray(origin,delta,Math.min(8,length)))continue;
      if(e.type.behavior==='melee'){if(Math.abs(p.y-pos.y)<1.6)this.damage(e.type.damage);e.cooldown=1;}
      else if(this.bullets.length<48){this.bullets.push({position:{...origin},velocity:delta.map(v=>v/length*8),damage:e.type.damage,age:0});e.cooldown=2;}
    }
    this.bullets=this.bullets.filter(b=>{
      const old={...b.position},step=b.velocity.map(v=>v*dt);b.age+=dt;
      if(this.world.ray(old,step,Math.hypot(...step)))return false;
      b.position.x+=step[0];b.position.y+=step[1];b.position.z+=step[2];
      if(Math.hypot(b.position.x-p.x,b.position.z-p.z)<.45&&b.position.y>p.y&&b.position.y<p.y+1.8){this.damage(b.damage);return false;}
      return b.age<3;
    });
  }
  snapshot() {
    const p=this.player;
    return {format:1,chunks:this.world.snapshotChunks(),player:{position:[p.x,p.y,p.z],yaw:p.yaw,pitch:p.pitch,health:p.health},inventory:this.inventory.map(s=>s?{...s}:null),selected:this.selected,progress:{...this.progress},enemies:this.enemies.map(e=>({id:e.id,position:[e.position.x,e.position.y,e.position.z],health:e.health}))};
  }
  restore(state) {
    validateVoxelState(this.definition,state);
    this.world.restoreChunks(state.chunks);const p=state.player;
    Object.assign(this.player,{x:p.position[0],y:p.position[1],z:p.position[2],yaw:p.yaw,pitch:p.pitch,health:p.health,vy:0});
    if(this.world.collides(this.player))Object.assign(this.player,this.world.spawn());
    this.inventory=state.inventory.map(s=>s?{...s}:null);this.selected=state.selected;this.progress={...state.progress};
    for(const saved of state.enemies){const e=this.enemies.find(e=>e.id===saved.id);if(e){e.position={x:saved.position[0],y:saved.position[1],z:saved.position[2]};e.health=Math.min(e.type.health,saved.health);}}
    this.changed();
  }
}

export function validateVoxelState(def,s) {
  const fail=()=>{throw new Error('Invalid voxel save');};
  if(!s||s.format!==1||!Array.isArray(s.chunks)||s.chunks.length>def.size.reduce((a,n)=>a*n/16,1)||!Array.isArray(s.inventory)||s.inventory.length!==36||!Number.isInteger(s.selected)||s.selected<0||s.selected>8)fail();
  const blocks=new Set([0,...def.blocks.map(b=>b.id)]),items=new Map(def.items.map(i=>[i.id,i])),chunks=new Set();
  const position=p=>Array.isArray(p)&&p.length===3&&p.every((n,i)=>Number.isFinite(n)&&n>=0&&n<=def.size[i]);
  if(!s.player||!position(s.player.position)||!Number.isFinite(s.player.yaw)||Math.abs(s.player.yaw)>Math.PI*2||!Number.isFinite(s.player.pitch)||Math.abs(s.player.pitch)>1.55||!Number.isFinite(s.player.health)||s.player.health<0||s.player.health>100)fail();
  for(const c of s.chunks){
    if(!c||!Array.isArray(c.position)||c.position.length!==3||!c.position.every((n,i)=>Number.isInteger(n)&&n>=0&&n<def.size[i]/16)||chunks.has(c.position.join(','))||!Array.isArray(c.runs)||c.runs.length<2||c.runs.length>8192||c.runs.length%2)fail();
    chunks.add(c.position.join(','));let count=0;
    for(let i=0;i<c.runs.length;i+=2){if(!blocks.has(c.runs[i])||!Number.isInteger(c.runs[i+1])||c.runs[i+1]<1||c.runs[i+1]>4096)fail();count+=c.runs[i+1];}if(count!==4096)fail();
  }
  for(const slot of s.inventory)if(slot&&(!items.has(slot.item)||!Number.isInteger(slot.count)||slot.count<1||slot.count>items.get(slot.item).stack))fail();
  const progress=new Set(['defeat',...def.items.flatMap(i=>['collect:'+i.id,'craft:'+i.id,'place:'+i.id])]);
  if(!s.progress||typeof s.progress!=='object'||Array.isArray(s.progress)||Object.entries(s.progress).some(([k,n])=>!progress.has(k)||!Number.isInteger(n)||n<0||n>1000000))fail();
  const enemies=new Map((def.enemies||[]).flatMap(e=>Array.from({length:e.count},(_,i)=>[`${e.id}:${i}`,e]))),seen=new Set();
  if(!Array.isArray(s.enemies)||s.enemies.length>24)fail();
  for(const e of s.enemies){if(!enemies.has(e.id)||seen.has(e.id)||!position(e.position)||!Number.isFinite(e.health)||e.health<0||e.health>1000)fail();seen.add(e.id);}
  return true;
}
