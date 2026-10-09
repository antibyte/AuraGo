// MIT. One clock owns simulation, presentation and incremental chunk geometry.
import * as T from './three-0.186.1.module.min.js';
import {VoxelGame} from './voxel-rules.js';
import {noise} from './voxel-world.js';
import {createVoxelUI,voxelLabels} from './voxel-ui.js';
import {createVoxelPersistence} from './voxel-save.js';
import {createPlayerUI} from './player-ui.js';
import {createGameFlow} from './game-flow.js';
import {createPresentation,createThreeAdapter} from './aurago-effects-3d-1.js';

export function startVoxelGame(definition,hooks={}) {
  window.__AURAGO_VOXEL_DISPOSE__?.();
  // Definitions were validated at file-write and build boundaries. Copy them so hooks cannot mutate rules.
  definition=JSON.parse(JSON.stringify(definition));
  let game=new VoxelGame(definition),disposed=false,frame=0,last=0,active=true,inventory=false,loading=true,ui,playerUI,persistence;
  let lastEvent=0,lastPersist=-1,persistAt=0,finished=false,fps=60,look=null,jumpUntil=0;
  const root=document.getElementById('game-root'),controller=new AbortController(),signal=controller.signal,keys=new Set(),inputs=new Map(),labels=voxelLabels();
  root.style.position='relative';root.querySelector('#hud')?.remove();const oldHUD=document.getElementById('hud');if(oldHUD)oldHUD.hidden=true;
  const touch=matchMedia('(pointer:coarse)').matches,view=touch?40:88;
  const renderer=new T.WebGLRenderer({antialias:!touch,preserveDrawingBuffer:true});renderer.setPixelRatio(Math.min(devicePixelRatio,touch?1:1.5));
  renderer.domElement.tabIndex=0;renderer.domElement.style.touchAction='none';root.append(renderer.domElement);
  const scene=new T.Scene(),camera=new T.PerspectiveCamera(75,1,.05,160);scene.background=new T.Color('#9fc8d7');scene.fog=new T.Fog(scene.background,view*.65,view);
  const sun=new T.DirectionalLight(0xffebcb,2.3);sun.position.set(-25,60,20);const ambient=new T.HemisphereLight(0xdaf0ff,0x526747,2);scene.add(sun,ambient,camera);
  const presentation=createPresentation({config:{...(hooks.presentation||{}),feedback:true},root,adapter:createThreeAdapter({scene,camera,renderer,sun,ambient}),report:message=>console.warn(message)});
  const flow=createGameFlow({root,feedback:(name,point,normal,material)=>presentation?.event?.(name,point,normal,material),project:point=>point?project(point):{x:.5,y:.5},restart:()=>respawn(),labels:{restart:labels.respawn,lost:labels.respawn,damage:labels.health,death:labels.respawn,respawn:labels.respawn,score:labels.goals}});
  const atlas=new Uint8Array(64*64*4);
  for(const block of definition.blocks){const color=new T.Color(block.color),tile=block.id-1;
    for(let y=0;y<8;y++)for(let x=0;x<8;x++){
      const grain=noise(block.id,x,y),stripe=block.material==='wood'?x%3===0:block.material==='planks'?y===0:block.material==='brick'?(y%4===0||(x+(y<4?0:4))%8===0):false;
      const factor=stripe?.62:.8+grain*.35,i=(((tile>>3)*8+y)*64+(tile%8)*8+x)*4;
      atlas[i]=Math.min(255,color.r*255*factor);atlas[i+1]=Math.min(255,color.g*255*factor);atlas[i+2]=Math.min(255,color.b*255*factor);atlas[i+3]=255;
    }
  }
  const texture=new T.DataTexture(atlas,64,64);texture.magFilter=texture.minFilter=T.NearestFilter;texture.needsUpdate=true;
  const material=new T.MeshLambertMaterial({map:texture}),chunks=new Map();
  const enemyGeometry=new T.BoxGeometry(.7,1.4,.7),enemyMaterials={melee:new T.MeshLambertMaterial({color:'#b65c4a'}),ranged:new T.MeshLambertMaterial({color:'#7861b2'})},enemyMeshes=new Map();
  const bulletGeometry=new T.BoxGeometry(.16,.16,.16),bulletMaterial=new T.MeshBasicMaterial({color:'#ffb261'}),bulletMeshes=[];
  const outlineBox=new T.BoxGeometry(1.006,1.006,1.006),outline=new T.LineSegments(new T.EdgesGeometry(outlineBox),new T.LineBasicMaterial({color:0xffffff,transparent:true,opacity:.7}));outlineBox.dispose();scene.add(outline);
  function clear(){inputs.clear();keys.clear();look=null;jumpUntil=0;ui?.release();game.mining=null;}
  function blocked(){return loading||!active||inventory||playerUI?.blocked||!game.alive;}
  function resize(){const width=root.clientWidth||innerWidth,height=root.clientHeight||innerHeight;renderer.setSize(width,height,false);camera.aspect=width/height;camera.updateProjectionMatrix();presentation?.resize();}
  const sizeObserver=new ResizeObserver(resize);sizeObserver.observe(root);resize();
  function project(point){const p=new T.Vector3(...point).project(camera);return {x:(p.x+1)/2,y:(1-p.y)/2,visible:p.z>=-1&&p.z<=1&&Math.abs(p.x)<1&&Math.abs(p.y)<1};}
  function rebuild(){
    const started=performance.now(),world=game.world,p=game.player;
    const distance=key=>{const c=key.split(',').map(Number);return Math.hypot(c[0]*16+8-p.x,c[2]*16+8-p.z);};
    const pending=[...world.dirty].sort((a,b)=>distance(a)-distance(b));
    for(const key of pending){
      if(distance(key)>view+24)continue;const data=world.mesh(key),old=chunks.get(key);if(old){scene.remove(old);old.geometry.dispose();chunks.delete(key);}
      if(data.indices.length){const geometry=new T.BufferGeometry();geometry.setAttribute('position',new T.Float32BufferAttribute(data.positions,3));geometry.setAttribute('normal',new T.Float32BufferAttribute(data.normals,3));geometry.setAttribute('uv',new T.Float32BufferAttribute(data.uv,2));geometry.setIndex(data.indices);geometry.computeBoundingSphere();const mesh=new T.Mesh(geometry,material);scene.add(mesh);chunks.set(key,mesh);}
      world.dirty.delete(key);if(performance.now()-started>5)break;
    }
    for(const [key,mesh] of chunks)mesh.visible=distance(key)<view+16;
  }
  function refreshActors(){
    for(const e of game.enemies){let mesh=enemyMeshes.get(e.id);if(!mesh){mesh=new T.Mesh(enemyGeometry,enemyMaterials[e.type.behavior]);enemyMeshes.set(e.id,mesh);scene.add(mesh);}mesh.position.set(e.position.x,e.position.y+.7,e.position.z);mesh.visible=e.health>0;}
    while(bulletMeshes.length<game.bullets.length){const mesh=new T.Mesh(bulletGeometry,bulletMaterial);bulletMeshes.push(mesh);scene.add(mesh);}
    bulletMeshes.forEach((mesh,i)=>{const b=game.bullets[i];mesh.visible=Boolean(b);if(b)mesh.position.set(b.position.x,b.position.y,b.position.z);});
  }
  function freeze(){clear();last=0;document.exitPointerLock?.();persistence?.save(true);}
  function toggleInventory(){if(loading||!playerUI.started||playerUI.blocked||!game.alive)return;inventory=!inventory;ui.open(inventory);freeze();}
  function action(name,down,source='voxel-ui'){
    if(name==='inventory'){if(down)toggleInventory();return;}
    if(blocked()&&down)return;
    if(name==='place'){if(down)game.place();return;}
    if(name==='jump'&&down)jumpUntil=performance.now()+180;
    // A released pointer must not clear another held touch or keyboard input.
    const held=inputs.get(name)||new Set();
    if(down)held.add(source);else held.delete(source);
    if(held.size){inputs.set(name,held);keys.add(name);}else{inputs.delete(name);keys.delete(name);}
  }
  function respawn(){if(game.respawn()){finished=false;flow.reset();playerUI.reset();clear();}}
  function replaceWorld(state){
    const next=new VoxelGame(definition);if(state)next.restore(state);hooks.dispose?.(api);game=next;
    for(const mesh of chunks.values()){scene.remove(mesh);mesh.geometry.dispose();}chunks.clear();
    clear();flow.reset();finished=false;lastEvent=0;inventory=false;ui?.open(false);playerUI?.reset();hooks.setup?.(api);
  }
  ui=createVoxelUI({root,getGame:()=>game,key:action,toggle:toggleInventory,craft:id=>{if(inventory)game.craft(id);},select:i=>{if(!blocked()){game.selected=i;game.changed();}},swap:i=>{if(inventory)game.swap(i);},
    supply:id=>{if(inventory)game.supply(id);},
    reload:async()=>{loading=true;clear();const state=await persistence.reload();if(persistence.loaded)replaceWorld(state);loading=false;},
    reset:async()=>{if(await persistence.reset()||persistence.temporary){replaceWorld(null);lastPersist=-1;}}});
  playerUI=createPlayerUI({root,mode:'voxel',movement:'full',action:false,objective:hooks.objective||'',instructions:labels.help,
    key:(name,down)=>action(({LEFT:'left',RIGHT:'right',UP:'forward',DOWN:'backward'})[name]||name,down,'player-ui'),clear,restart:()=>{if(!game.alive)respawn();else {Object.assign(game.player,game.world.spawn(),{vy:0});game.changed();}},change:isBlocked=>{if(isBlocked)freeze();}});
  const api=Object.freeze({scene,camera,renderer,
    getBlock:(x,y,z)=>game.world.get(x,y,z),
    setBlock:(x,y,z,id)=>{if(!game.world.inside(x,y,z)||!Number.isInteger(id)||(id!==0&&!game.world.blocks.has(id))||y===0)return false;const old=game.world.get(x,y,z);if(!game.world.set(x,y,z,id))return false;if(id&&game.world.collides(game.player)){game.world.set(x,y,z,old);return false;}game.changed();return true;},
    transaction:(take,give)=>game.transaction(take,give),count:id=>game.count(id),craft:id=>game.craft(id),
    damagePlayer:amount=>game.damage(amount),get player(){return Object.freeze({...game.player});},get progress(){return Object.freeze({...game.progress});},
    event:(name,point)=>flow.event(name,point),dispose:()=>dispose(),
  });
  persistence=createVoxelPersistence({definition,snapshot:()=>game.snapshot(),restore:state=>game.restore(state),status:kind=>ui.saveStatus(kind)});
  persistence.ready.finally(()=>{if(disposed)return;loading=false;lastPersist=game.revision;hooks.setup?.(api);});
  window.addEventListener('keydown',e=>{
    if(e.target instanceof HTMLInputElement||e.target instanceof HTMLTextAreaElement)return;
    const key=e.key.toLowerCase();
    if([' ','arrowup','arrowdown','arrowleft','arrowright','escape','i'].includes(key))e.preventDefault();
    if(e.repeat&&['i','escape','p'].includes(key))return;
    if(key==='escape'||key==='p'){if(inventory){inventory=false;ui.open(false);freeze();}else {playerUI.togglePause();freeze();}return;}
    if(key==='i'){toggleInventory();return;}
    if(!game.alive&&(key==='r'||key==='enter')){respawn();return;}
    if(/^[1-9]$/.test(key)&&!blocked()){game.selected=Number(key)-1;game.changed();return;}
    action(({w:'forward',s:'backward',a:'left',d:'right',' ':'jump',arrowup:'lookUp',arrowdown:'lookDown',arrowleft:'lookLeft',arrowright:'lookRight',e:'primary',f:'place'})[key]||key,true,'keyboard:'+e.code);
  },{signal});
  window.addEventListener('keyup',e=>action(({w:'forward',s:'backward',a:'left',d:'right',' ':'jump',arrowup:'lookUp',arrowdown:'lookDown',arrowleft:'lookLeft',arrowright:'lookRight',e:'primary',f:'place'})[e.key.toLowerCase()]||e.key.toLowerCase(),false,'keyboard:'+e.code),{signal});
  const canvas=renderer.domElement;
  canvas.addEventListener('contextmenu',e=>e.preventDefault(),{signal});
  canvas.addEventListener('pointerdown',e=>{
    if(blocked())return;canvas.focus();
    // Pointer capture is forbidden while pointer lock already owns mouse input.
    if(!document.pointerLockElement)canvas.setPointerCapture(e.pointerId);
    if(e.pointerType==='touch'||e.pointerType==='pen'){look={id:e.pointerId,x:e.clientX,y:e.clientY};return;}
    if(e.button===2){game.place();return;}if(e.button!==0)return;
    action('primary',true,'pointer:'+e.pointerId);look={id:e.pointerId,x:e.clientX,y:e.clientY};
    if(!document.pointerLockElement)try{canvas.requestPointerLock?.()?.catch?.(()=>{});}catch{}
  },{signal});
  function turn(x,y){game.player.yaw=((game.player.yaw-x*.003+Math.PI*3)%(Math.PI*2))-Math.PI;game.player.pitch=Math.max(-1.5,Math.min(1.5,game.player.pitch-y*.003));game.changed();}
  window.addEventListener('pointermove',e=>{if(blocked())return;if(document.pointerLockElement===canvas){turn(e.movementX,e.movementY);return;}if(look?.id===e.pointerId){turn(e.clientX-look.x,e.clientY-look.y);look.x=e.clientX;look.y=e.clientY;}},{signal});
  for(const event of ['pointerup','pointercancel','lostpointercapture'])canvas.addEventListener(event,e=>{if(look?.id===e.pointerId)look=null;action('primary',false,'pointer:'+e.pointerId);},{signal});
  document.addEventListener('pointerlockchange',()=>{if(document.pointerLockElement!==canvas)clear();},{signal});
  window.addEventListener('blur',()=>{if(playerUI.started&&!playerUI.blocked)playerUI.togglePause();freeze();},{signal});
  document.addEventListener('visibilitychange',()=>{active=!document.hidden;playerUI.sync(active);if(!active)freeze();last=0;},{signal});
  window.addEventListener('pagehide',()=>{persistence.save(true).finally(dispose);},{signal});
  window.addEventListener('message',event=>{
    const channel=new URLSearchParams(location.hash.slice(1)).get('gm-channel');
    if(event.source===parent&&channel&&event.data?.source==='aurago-voxel-host'&&event.data.channel===channel&&event.data.type==='flush'){
      clear();if(playerUI.started&&!playerUI.blocked)playerUI.togglePause();persistence.save(true).then(ok=>parent.postMessage({source:'aurago-voxel',type:'flushed',channel,ok},'*'));return;
    }
    if(event.source===parent&&event.data?.type==='aurago:game:active'&&typeof event.data.active==='boolean'){
      active=event.data.active;playerUI.sync(active);if(!active)freeze();last=0;return;
    }
    if(event.source!==parent||!channel||event.data?.source!=='aurago-studio'||event.data.channel!==channel)return;
    if(event.data.type==='preview-active'){active=event.data.active===true;playerUI.sync(active);if(!active)freeze();last=0;}
  },{signal});
  function observation(cells=[]){
    const p=game.player,target=game.aim(),g=game.metrics;
    const requested=Array.isArray(cells)?cells.slice(0,62).filter(c=>Array.isArray(c)&&c.length===3&&game.world.inside(...c)):[];
    if(target)requested.push(target.cell,target.previous);
    const sampled=new Map(requested.map(c=>[c.join(','),{cell:[...c],id:game.world.get(...c)}]));
    const targets=[];
    for(let y=Math.max(1,Math.floor(p.y)-2);y<=Math.min(game.world.size[1]-1,Math.ceil(p.y)+4);y++)for(let z=Math.floor(p.z)-5;z<=Math.floor(p.z)+5;z++)for(let x=Math.floor(p.x)-5;x<=Math.floor(p.x)+5;x++){
      const id=game.world.get(x,y,z);if(!id)continue;const point=[x+.5,y+.5,z+.5],eye=game.eye(),delta=[point[0]-eye.x,point[1]-eye.y,point[2]-eye.z],distance=Math.hypot(...delta);
      if(distance>6)continue;const hit=game.world.ray(eye,delta,6);if(hit?.cell.join(',')!==[x,y,z].join(','))continue;
      const block=game.world.blocks.get(id);targets.push({cell:[x,y,z],point,id,item:block.drop,tier:block.tier,hardness:block.hardness,distance});
    }
    targets.sort((a,b)=>a.distance-b.distance);
    return {kind:'voxel',version:1,mode:definition.mode,player:{position:[p.x,p.y,p.z],yaw:p.yaw,pitch:p.pitch,health:p.health},target:target?{...target,block:game.world.get(...target.cell)}:null,
      inventory:game.inventory.map(s=>s?{...s}:null),selected:game.selected,events:JSON.parse(JSON.stringify(game.events)),world_revision:game.world.revision,
      enemies:game.enemies.map(e=>({id:e.id,position:[e.position.x,e.position.y+.7,e.position.z],health:e.health,...project([e.position.x,e.position.y+.7,e.position.z])})),
      blocks:[...sampled.values()],targets:targets.slice(0,64),recipes:definition.recipes||[],items:definition.items,
      metrics:{...g},paused:blocked(),ready:!loading,grounded:p.grounded};
  }
  const binding={kind:'voxel',canvas,alive:()=>!disposed&&!loading,snapshot:()=>({ready:loading?0:1,player_x:game.player.x,player_y:game.player.z,player_distance:game.metrics.distance,actions:game.metrics.mined+game.metrics.placed+game.metrics.hits,hits:game.metrics.hits,pickup_events:game.metrics.pickups,voxel_mined:game.metrics.mined,voxel_placed:game.metrics.placed,voxel_crafted:game.metrics.crafted,voxel_jumps:game.metrics.jumps,voxel_respawns:game.metrics.respawns,health:game.player.health,ended:game.alive?0:1,ticks:game.time,invalid_assets:0,assets_used:definition.blocks.length,fps}),observeVoxel:observation};
  window.__AURAGO_GAME_TEST__=binding;
  function draw(now){
    if(disposed)return;frame=requestAnimationFrame(draw);const elapsed=last?(now-last)/1000:0,dt=Math.min(.05,elapsed);last=now;if(elapsed>0)fps=fps*.95+.05/elapsed;
    if(!active)return;
    if(!blocked()){
      if(keys.has('lookLeft'))turn(-dt*400,0);if(keys.has('lookRight'))turn(dt*400,0);if(keys.has('lookUp'))turn(0,-dt*400);if(keys.has('lookDown'))turn(0,dt*400);
      const input=Object.fromEntries([...keys].map(k=>[k,true]));input.jump=Boolean(input.jump)||now<jumpUntil;if(input.jump&&game.player.grounded)jumpUntil=0;if(input.primary&&hooks.action?.(api,game.aim())===false)input.primary=false;
      game.tick(dt,input);hooks.step?.(api,dt);
      if(!finished&&definition.goals?.length&&definition.goals.every(g=>(game.progress[g.kind+(g.item?':'+g.item:'')]||0)>=g.count)){finished=true;flow.message(labels.complete,3);}
    }
    const p=game.player;camera.position.set(p.x,p.y+1.62,p.z);camera.rotation.set(p.pitch,p.yaw,0,'YXZ');camera.updateMatrixWorld();
    rebuild();refreshActors();const target=game.aim();outline.visible=Boolean(target)&&!blocked();if(target)outline.position.set(...target.cell.map(n=>n+.5));
    for(const event of game.events.filter(e=>e.sequence>lastEvent)){flow.event(({mine:'pickup',place:'impact',craft:'pickup'})[event.kind]||event.kind);}
    lastEvent=game.eventSequence;
    const paused=loading||inventory||playerUI.blocked;flow.update(dt,{health:p.health,score:game.metrics.mined,ended:!game.alive,outcome:game.alive?0:2},!paused);
    playerUI.sync(active,!game.alive);ui.update(paused||!game.alive);presentation?.setPaused(paused);presentation?.update(dt);if(presentation?.render)presentation.render();else renderer.render(scene,camera);
    if(!loading&&game.revision!==lastPersist&&now-persistAt>=5000){persistAt=now;const revision=game.revision;persistence.save().then(ok=>{if(ok)lastPersist=revision;});}
  }
  function dispose(){if(disposed)return;disposed=true;cancelAnimationFrame(frame);clear();controller.abort();sizeObserver.disconnect();playerUI.dispose();ui.dispose();persistence.dispose();flow.dispose();presentation?.dispose();
    for(const mesh of chunks.values())mesh.geometry.dispose();enemyGeometry.dispose();bulletGeometry.dispose();material.dispose();texture.dispose();for(const m of Object.values(enemyMaterials))m.dispose();bulletMaterial.dispose();outline.geometry.dispose();outline.material.dispose();renderer.dispose();canvas.remove();
    hooks.dispose?.(api);if(window.__AURAGO_GAME_TEST__===binding)delete window.__AURAGO_GAME_TEST__;if(window.__AURAGO_VOXEL_DISPOSE__===dispose)delete window.__AURAGO_VOXEL_DISPOSE__;
  }
  window.__AURAGO_VOXEL_DISPOSE__=dispose;frame=requestAnimationFrame(draw);return api;
}
