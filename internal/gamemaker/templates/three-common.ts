// Original MIT game bases, adapted from AuraGo's playable low-poly references.
import * as A from '../vendor/aurago-three-assets-1.js';
import {createPresentation,createThreeAdapter} from '../vendor/aurago-effects-3d-1.js';
import {createScene3D,hasSceneNodes} from '../vendor/scene-builder.js';
import presentationPlan from './presentation.json';
import scenePlan from './scene.json';
import mechanicsPlan from './mechanics.json';
// PLAN_MODEL_IMPORTS
const roles: any = {};
const T = A.THREE;

export function startGame(config: any) {
  const movement=(mechanicsPlan as any)?.blocks?.find((b:any)=>b.kind==='movement'&&b.enabled!==false);
  if(movement) { const params=JSON.parse(movement.params||'{}'); config={...config,mode:params.mode||config.mode,speed:params.speed??config.speed}; }
  const cameraBlock=(mechanicsPlan as any)?.blocks?.find((b:any)=>b.kind==='camera'&&b.enabled!==false);
  const cameraOptions=cameraBlock?JSON.parse(cameraBlock.params||'{}'):{};
  const root = document.getElementById('game-root')!;
  const controller = new AbortController(), signal = controller.signal;
  const renderer = new T.WebGLRenderer({antialias:true, preserveDrawingBuffer:true});
  renderer.setPixelRatio(Math.min(devicePixelRatio,1.5));
  renderer.toneMapping = T.ACESFilmicToneMapping;
  renderer.shadowMap.enabled = true; renderer.shadowMap.type = T.PCFSoftShadowMap;
  root.append(renderer.domElement);
  const scene = new T.Scene(), camera = new T.PerspectiveCamera(T.MathUtils.clamp(Number(cameraOptions.fov)||65,20,110),1,.015,240);
  scene.background = new T.Color(config.mode === 'space' ? 0x090e21 : 0x9db9bc);
  scene.fog = new T.Fog(scene.background,45,150);
  const sun = new T.DirectionalLight(0xffe9c2,2.8); sun.position.set(-20,35,-10);sun.castShadow=true;
  sun.shadow.mapSize.set(1024,1024);Object.assign(sun.shadow.camera,{left:-35,right:35,top:35,bottom:-35});sun.shadow.normalBias=.05;
  const ambient=new T.HemisphereLight(0xcfe8ff,0x51633a,2.2);scene.add(sun,ambient,camera);
  const presentation=presentationPlan?createPresentation({config:presentationPlan,root,adapter:createThreeAdapter({scene,camera,renderer,sun,ambient}),report:(message:any)=>console.warn(message)}):null;
  let footstepAt=0,finishReported=false;
  const owned:any[] = [], live:any[] = [], objects:any[] = [], loaded = new Map();
  const position = new T.Vector3(), vector = new T.Vector3(), ray = new T.Raycaster();
  const player = new T.Group();scene.add(player);
  const floor = new T.Mesh(presentationPlan?.environment==='coast'?new T.CircleGeometry(12,48):new T.PlaneGeometry(200,200),new T.MeshStandardMaterial({color:0x526b4d,roughness:1}));
  floor.rotation.x=-Math.PI/2;floor.receiveShadow=true;floor.visible=config.mode!=='space';scene.add(floor);owned.push(floor);if(floor.visible)presentation?.registerSurface(floor,{kind:'ground'});
  const hud = document.createElement('div'), crosshair = document.createElement('div'), controls=document.createElement('div');
  hud.style.cssText='position:absolute;left:18px;top:18px;padding:14px 18px;border-radius:12px;background:#101e2bdd;color:#eef8ff;font:14px/1.6 system-ui;pointer-events:none;white-space:pre-line;max-width:70%';
  crosshair.style.cssText='position:absolute;left:50%;top:50%;transform:translate(-50%,-50%);color:#fff;pointer-events:none;font:24px monospace';crosshair.textContent=config.mode==='fps'?'+':'';
  controls.style.cssText='position:absolute;left:12px;right:12px;bottom:12px;display:flex;gap:6px;flex-wrap:wrap;pointer-events:none';
  root.append(hud,crosshair,controls);
  const keys=new Set<string>();let listeners=0, dragging=false;
  const on=(target:any,name:string,fn:any)=>{target.addEventListener(name,fn,{signal});listeners++};
  let time=0,score=0,hits=0,actions=0,health=100,ammo=8,reloads=0,ended=false,won=false,carrying=false,aim=0,pitch=0,shotAt=-1,reloadAt=0,boostUntil=0,wheelAngle=0;
  let frame=0,last=0,disposed=false,ready=false,arms:any,weapon:any,avatar:any,paused=false;
  let builder:any=null, debugGroup:any=null, cameraBounds:any=null,active=true, sceneBounds:any={min:[-45,0,-20],max:[45,30,170]};
  const sceneState:any={score:0,hits:0,actions:0,lives:0,goal_remaining:0,outcome:0,hit_events:0,pickup_events:0,win_events:0,lose_events:0};
  const clip=(unit:any,name:string,restart=false)=>{
    if(!unit)return;
    if(unit.asset.model.animations.some((c:any)=>c.id===name))A.playAction(unit,name,{fade:.12,restart});
  };
  const weaponAction=(name:string)=>{const prefix=weapon?.asset.model.fps_binding?.arm_action_prefix||'';clip(arms,prefix+name,true);clip(weapon,name,true)};
  const resize=()=>{renderer.setSize(root.clientWidth||innerWidth,root.clientHeight||innerHeight);camera.aspect=renderer.domElement.width/renderer.domElement.height;camera.updateProjectionMatrix();presentation?.resize()};
  function reset(){
    presentation?.reset();presentation?.setPaused(false);footstepAt=0;finishReported=false;
    time=score=hits=actions=reloads=0;sceneState.score=sceneState.hits=sceneState.actions=0;health=100;ammo=8;ended=won=carrying=paused=false;aim=pitch=reloadAt=boostUntil=wheelAngle=0;shotAt=-1;keys.clear();
    player.position.set(0,config.mode==='flight'?5:config.mode==='space'?4:0,0);player.rotation.set(0,0,0);
    for(const o of objects){o.unit.root.position.copy(o.start);o.unit.root.visible=true}
    if(avatar)clip(avatar,'idle');weaponAction('idle');builder?.reset();cameraUpdate(1);config.reset?.(api);
  }
  function fire(){
    if(ended||!ready||paused||!active||time-shotAt<.22||time<reloadAt)return;
    if(config.mode==='fps'&&!ammo){reload();return}
    shotAt=time;actions++;if(config.mode==='fps'){ammo--;weaponAction('fire')}
    if(config.mode==='fps'){camera.updateMatrixWorld();ray.setFromCamera({x:0,y:0},camera)}
    else {vector.set(0,0,1);ray.set(player.position,vector)}
    presentation?.event('shot',camera.position.clone().addScaledVector(ray.ray.direction,.8).toArray());
    let closest:any=null,distance=80,hitPoint=new T.Vector3(),hitNormal=new T.Vector3();
    for(const o of objects){
      if(!o.unit.root.visible)continue;
      if(builder&&!o.node?.collider&&!o.node?.behaviors?.some((b:any)=>['destroy','health'].includes(b.type)))continue;
      if(!builder&&o.role!=='enemy')continue;
      o.unit.root.updateMatrixWorld(true);const contact=ray.intersectObject(o.unit.root,true)[0];
      if(contact&&contact.distance<distance){distance=contact.distance;closest=o;hitPoint.copy(contact.point);hitNormal.copy(contact.face?.normal||ray.ray.direction.clone().negate()).transformDirection(contact.object.matrixWorld)}
    }
    if(closest){if(builder) { if(closest.node.behaviors.some((b:any)=>['destroy','health'].includes(b.type)))builder.hit(closest.nodeID,{point:hitPoint.toArray(),normal:hitNormal.toArray()}); }else{presentation?.event('hit',hitPoint.toArray(),hitNormal.toArray(),config.mode==='space'?'metal':'flesh');closest.unit.root.visible=false;score++;hits++;if(score>=config.goal){ended=won=true}}}
  }
  function reload(){
    if(config.mode!=='fps'||ended||reloadAt>time||ammo===8)return;
    presentation?.event('reload',camera.position.toArray());
    const name=ammo?'reload':'reload_empty';weaponAction(name);
    reloadAt=time+(weapon?.asset.model.animations.find((c:any)=>c.id===name)?.duration||1.5);
  }
  function primary(){if(ended||!ready||paused||!active)return;if(config.action?.(api)===false)return;if(builder?.action?.()>0){actions++;return;}if(config.mode==='fps'||config.mode==='space')fire();else if(!ended&&ready&&!paused&&active){boostUntil=time+1.5;actions++}}
  function key(event:KeyboardEvent,down:boolean){
    const name=event.key.toLowerCase();if([' ','arrowup','arrowdown','arrowleft','arrowright'].includes(name))event.preventDefault();
    if(down)keys.add(name);else keys.delete(name);
    if(down&&!event.repeat){if(name==='r')reset();if(name==='f')reload();if(name==='escape'){ended=true;document.exitPointerLock?.()}if(name===' ')primary();if(name==='p')paused=!paused}
  }
  on(window,'keydown',(e:KeyboardEvent)=>key(e,true));on(window,'keyup',(e:KeyboardEvent)=>key(e,false));
  on(window,'message',(e:MessageEvent)=>{if(e.source===parent&&e.data?.type==='aurago:game:active'){active=e.data.active===true;keys.clear();last=0;presentation?.setActive(active)}});
  on(window,'blur',()=>{keys.clear();dragging=false;document.exitPointerLock?.()});on(window,'resize',resize);
  on(renderer.domElement,'pointerdown',(e:PointerEvent)=>{dragging=true;if(e.isTrusted)renderer.domElement.setPointerCapture?.(e.pointerId);if(config.mode==='fps'){primary();if(e.isTrusted&&e.pointerType==='mouse')renderer.domElement.requestPointerLock?.()?.catch?.(()=>{})}});
  on(renderer.domElement,'pointerup',()=>dragging=false);
  on(renderer.domElement,'pointermove',(e:PointerEvent)=>{if(config.mode==='fps'&&(dragging||document.pointerLockElement===renderer.domElement)){aim-=e.movementX*.003;pitch=T.MathUtils.clamp(pitch-e.movementY*.003,-1.1,1.1)}});
  for(const [label,keyName] of [['◀','a'],['▲','w'],['▼','s'],['▶','d'],['Action',' '],['Pause','p'],['Reload','f'],['Restart','r'],['Descend','q'],['Ascend','e']]){
    if(keyName==='f'&&config.mode!=='fps'||['q','e'].includes(keyName)&&!['flight','space'].includes(config.mode))continue;
    const b=document.createElement('button');b.textContent=label;b.setAttribute('aria-label',label);b.style.cssText='pointer-events:auto;touch-action:none;border:1px solid #8eafc666;border-radius:9px;background:#152635dc;color:white;padding:12px 16px;min-height:44px';
    on(b,'pointerdown',(e:PointerEvent)=>{e.preventDefault();b.setPointerCapture(e.pointerId);keys.add(keyName);if(keyName===' ')primary();if(keyName==='f')reload();if(keyName==='r')reset();if(keyName==='p')paused=!paused});
    on(b,'pointerup',()=>keys.delete(keyName));on(b,'pointercancel',()=>keys.delete(keyName));controls.append(b);
  }
  function instantiate(role:string,at:number[]){
    const asset=loaded.get(role);if(!asset)throw Error('Unknown role '+role+'. Available: '+Object.keys(roles).join(', '));
    const unit=A.createInstance(asset);unit.root.scale.setScalar(roles[role].scale);unit.root.position.fromArray(at);unit.root.userData.assetID=roles[role].id;scene.add(unit.root);live.push(unit);return unit;
  }
  function primitiveNode(node:any){
    const size=Array.isArray(node.size)?node.size:[1,1,1];
    const geometry=node.kind==='circle'?new T.SphereGeometry(Math.max(size[0],size[1],size[2])*.5,16,10):new T.BoxGeometry(Math.max(.05,size[0]),Math.max(.05,size[1]),Math.max(.05,size[2]));
    const material=new T.MeshStandardMaterial({color:Number.isFinite(Number(node.properties?.color))?Number(node.properties.color):0x64748b,roughness:.85});
    const rootNode=new T.Mesh(geometry,material);rootNode.position.fromArray(node.at);scene.add(rootNode);
    const unit:any={root:rootNode,procedural:true,asset:{model:{animations:[],moving_parts:[]}}};live.push(unit);owned.push(rootNode);return unit;
  }
  function nodeBox(node:any,object:any) {
    const c=node?.collider,half=c?.extents||node?.size?.map((v:number)=>v/2)||[.5,.5,.5],offset=c?.offset||[0,0,0];
    const center=object.position.clone().add(new T.Vector3(...offset));
    return new T.Box3(center.clone().sub(new T.Vector3(...half)),center.clone().add(new T.Vector3(...half)));
  }
  function playerBox(at=player.position) {
    const node=builder?.scene.nodes.find((n:any)=>n.kind==='player'||n.properties?.player);
    if(node?.collider)return nodeBox(node,{position:at});
    return new T.Box3(at.clone().add(new T.Vector3(-.3,.02,-.3)),at.clone().add(new T.Vector3(.3,1.7,.3)));
  }
  function moveBody(object:any,delta:any,node?:any) {
    // ponytail: bounded AABB scan; add a spatial grid if dense scenes show measured cost.
    for(const axis of ['x','y','z']) {
      const before=object.position[axis];object.position[axis]+=delta[axis];
      const moving=object===player?playerBox():nodeBox(node,object);
      const blocked=objects.some(o=>o.unit.root!==object&&o.unit.root.visible&&o.node?.collider&&
        !o.node.behaviors.some((b:any)=>['projectile','collect','reach'].includes(b.type))&&moving.intersectsBox(nodeBox(o.node,o.unit.root)));
      if(blocked)object.position[axis]=before;
    }
  }
  function destroySceneObject(object:any) {
    const entry=objects.find(o=>o.unit.root===object);
    if(!entry)return;
    for(const detach of object.userData.sceneAttachments||[])detach();
    for(const visual of object.userData.sceneVisuals||[]){live.splice(live.indexOf(visual.unit),1);A.disposeInstance(visual.unit);}
    objects.splice(objects.indexOf(entry),1);live.splice(live.indexOf(entry.unit),1);
    if(entry.unit.procedural) {object.removeFromParent();object.geometry?.dispose();object.material?.dispose();const index=owned.indexOf(object);if(index>=0)owned.splice(index,1)}
    else A.disposeInstance(entry.unit);
  }
  function clearSceneDebug() {
    debugGroup?.traverse((o:any)=>{o.geometry?.dispose();o.material?.map?.dispose();o.material?.dispose()});
    debugGroup?.removeFromParent();debugGroup=null;
  }
  function renderSceneDebug(current:any) {
    if(!debugGroup) {
      debugGroup=new T.Group();scene.add(debugGroup);
      const box=(b:any,color:number)=>debugGroup.add(new T.Box3Helper(new T.Box3(new T.Vector3(...b.min),new T.Vector3(...b.max)),color));
      box(current.scene.world_bounds,0x58a6ff);if(current.scene.camera_bounds)box(current.scene.camera_bounds,0xa78bfa);
      for(const zone of (scenePlan as any)?.zones||[])box(zone.bounds,zone.kind==='loss'?0xf87171:0x5eead4);
      for(const route of Object.values(current.scene.routes) as any[])if(route.waypoints?.length>1) {
        const line=new T.Line(new T.BufferGeometry().setFromPoints(route.waypoints.map((v:any)=>new T.Vector3(...v))),new T.LineBasicMaterial({color:0xffb454}));debugGroup.add(line);
      }
      for(const record of [...current.nodes.values()].slice(0,128) as any[]) {
        const helper=new T.Box3Helper(nodeBox(record.node,record.object),0xfbbf24);helper.userData.record=record;debugGroup.add(helper);
        const canvas=document.createElement('canvas');canvas.width=256;canvas.height=32;const c=canvas.getContext('2d')!;c.fillStyle='#081018';c.fillRect(0,0,256,32);c.fillStyle='#fbbf24';c.font='18px monospace';c.fillText(record.node.id,5,23);
        const label=new T.Sprite(new T.SpriteMaterial({map:new T.CanvasTexture(canvas),depthTest:false}));label.scale.set(3,.375,1);label.userData.label=record;debugGroup.add(label);
      }
    }
    debugGroup.visible=true;
    for(const child of debugGroup.children) {
      const record=child.userData.record||child.userData.label;if(!record)continue;child.visible=record.active;
      if(child.userData.record)child.box.copy(nodeBox(record.node,record.object));
      else child.position.copy(record.object.position).add(new T.Vector3(0,2,0));
    }
  }
  async function setup(){
    for(const [role,spec] of Object.entries(roles) as any){
      const asset=await A.loadAsset(spec.manifest,spec.id,spec.base,{signal});
      if(disposed){A.releaseAsset(asset);return}loaded.set(role,asset);
    }
    if(loaded.has('player')){avatar=instantiate('player',[0,0,0]);player.add(avatar.root);clip(avatar,'idle')}
    if(hasSceneNodes(scenePlan)){
      builder=createScene3D(scenePlan,{
        state:sceneState, mechanics:mechanicsPlan,
        createNode:(node:any)=>{
          if(node.kind==='player'||node.properties?.player===true) {
            player.position.fromArray(node.at);player.rotation.set(...(node.rotation||[0,0,0]));
            if(!avatar&&config.mode!=='fps') { avatar=loaded.has(node.role)?instantiate(node.role,[0,0,0]):primitiveNode({...node,at:[0,0,0]});player.add(avatar.root); }
            if(avatar) {player.userData.assetID=avatar.root.userData.assetID||'';avatar.root.scale.multiply(new T.Vector3(...(node.scale||[1,1,1]).map((v:number)=>v||1)));}
            return {object:player};
          }
          const unit=loaded.has(node.role)?instantiate(node.role,node.at):primitiveNode(node);if(!unit.procedural)clip(unit,'idle');
          if(Array.isArray(node.rotation))unit.root.rotation.set(node.rotation[0],node.rotation[1],node.rotation[2]);
          unit.root.scale.multiply(new T.Vector3(...(node.scale||[1,1,1]).map((v:number)=>v||1)));
          const entry:any={role:node.role,nodeID:node.id,node,unit,start:unit.root.position.clone(),box:new T.Box3()};
          objects.push(entry);return {object:unit.root};
        },
        player:()=>({object:player}),
        attachVisual:(node:any,placement:any,object:any)=>{
          const first=placement.id===node.placements[0]?.id;
          let unit=first?(object===player?avatar:objects.find(o=>o.unit.root===object)?.unit):null;
          if(!first&&loaded.has(placement.asset_role)){unit=instantiate(placement.asset_role,[0,0,0]);object.add(unit.root);(object.userData.sceneVisuals??=[]).push({role:placement.asset_role,unit});}
          if(!unit)return;
          if(first)unit.root.userData.sceneRole=placement.asset_role;
          const visuals=first&&object!==player&&!unit.procedural?unit.levels:[unit.root];
          for(const visual of visuals) {
            if(!unit.procedural)visual.position.set(...placement.position.map((v:number,i:number)=>v-node.at[i]));
            visual.rotation.set(...(placement.rotation||[0,0,0]));
            visual.scale.multiply(new T.Vector3(...placement.scale));
          }
        },
        resolveAttachment:(attachment:any,object:any)=>{
          const cache=object.userData.sceneAnchors??={};if(cache[attachment.id])return cache[attachment.id];
          const candidates=[object===player?avatar:objects.find(o=>o.unit.root===object)?.unit,...(object.userData.sceneVisuals||[]).map((v:any)=>v.unit)].filter(Boolean);
          const unit=candidates.find((u:any)=>!attachment.asset_id||u.root.userData.assetID===attachment.asset_id);
          if(!unit||unit.procedural)throw Error('Attachment requires an imported model: '+attachment.id);
          const anchor=new T.Group();anchor.position.fromArray(attachment.position||[0,0,0]);anchor.rotation.set(...(attachment.rotation||[0,0,0]));
          const detach=A.attachToSocket(unit,attachment.socket,anchor);
          (object.userData.sceneAttachments??=[]).push(detach);cache[attachment.id]=anchor;return anchor;
        },
        actualAssets:(node:any,object:any)=>[
          {role:node.role,asset_id:object.userData.assetID||''},
          ...(object.userData.sceneVisuals||[]).map((v:any)=>({role:v.role,asset_id:v.unit.root.userData.assetID||''}))
        ].filter((v:any)=>v.asset_id),
        actualRole:(node:any)=>node.role||'',
        resourceCounts:()=>{let count=0;scene.traverse(()=>count++);return{objects:count,geometries:renderer.info.memory.geometries,textures:renderer.info.memory.textures,loaded_assets:loaded.size,instances:live.length}},
        actualAssetID:(_node:any,object:any)=>object.userData.assetID||'',
        event:(name:string,node:any,detail:any={})=>{
          const point=builder?.nodes.get(node?.id)?.object?.position||player.position;
          presentation?.event(name,detail.point||point.toArray(),detail.normal);
        },
        presentation,
        end:(complete:boolean)=>{ended=true;won=complete},
        removeNode:(_node:any,object:any)=>{object.visible=false},
        resetNode:(_node:any,object:any,at:any)=>{object.position.fromArray(at);object.visible=true},
        move:(node:any,object:any,delta:any)=>moveBody(object,new T.Vector3(...delta),node),
        overlap:(node:any,object:any)=>nodeBox(node,object).intersectsBox(playerBox()),
        destroyNode:(_node:any,object:any)=>destroySceneObject(object),
        renderDebug:(_builder:any)=>renderSceneDebug(_builder),
        debugChanged:(enabled:boolean)=>{if(debugGroup)debugGroup.visible=enabled},
      });
      sceneBounds=builder.scene.world_bounds;cameraBounds=builder.scene.camera_bounds;
      camera.near=.015;camera.far=Math.max(80,Math.hypot(...sceneBounds.max.map((v:number,i:number)=>v-sceneBounds.min[i]))*2);camera.updateProjectionMatrix();
    } else {
      for(const spec of config.objects){
        if(!Array.isArray(spec.at)||spec.at.length!==3||!spec.at.every(Number.isFinite))throw Error('Each object requires at:[x,y,z]');
        const unit=instantiate(spec.role,spec.at);clip(unit,'idle');
        if(['building','terrain','obstacle'].includes(spec.role))presentation?.registerSurface(unit.root,{kind:spec.role==='terrain'?'ground':'roof'});
        objects.push({role:spec.role,unit,start:unit.root.position.clone(),box:new T.Box3()});
      }
    }
    if(config.mode==='fps'&&loaded.has('arms')&&loaded.has('weapon')){
      arms=await instantiate('arms',[0,0,0]);weapon=await instantiate('weapon',[0,0,0]);
      const view=new T.Group();view.rotation.y=Math.PI;view.position.set(.25,-.22,-.35);camera.add(view);view.add(arms.root,weapon.root);
      weapon.root.position.fromArray(weapon.asset.model.fps_binding.weapon_translation);
      for(const unit of [arms,weapon])unit.root.traverse((n:any)=>n.castShadow=false);weaponAction('idle');
    }
    ready=true;reset();resize();config.setup?.(api);
    const binding={kind:'three',canvas:renderer.domElement,alive:()=>ready&&!disposed,snapshot:()=>snapshot(),audit:()=>builder?.audit?.()||null};
    (window as any).__AURAGO_GAME_TEST__=binding;
    frame=requestAnimationFrame(draw);
  }
  function cameraUpdate(dt:number){
    if(config.mode==='fps'){camera.position.copy(player.position).add(vector.set(0,1.65,0));camera.rotation.set(pitch,Math.PI+aim,0,'YXZ')}
    else {position.copy(player.position).add(vector.fromArray([cameraOptions.offset?.[0]??10,cameraOptions.offset?.[1]??12,cameraOptions.offset?.[2]??-17]));camera.position.lerp(position,1-Math.exp(-dt*(Number(cameraOptions.smoothing)||8)));camera.lookAt(player.position.x,player.position.y+1,player.position.z+3)}
    if(cameraBounds)camera.position.clamp(new T.Vector3(...cameraBounds.min),new T.Vector3(...cameraBounds.max));
  }
  function update(dt:number){
    time+=dt;if(reloadAt&&time>=reloadAt){reloadAt=0;ammo=8;reloads++;weaponAction('idle')}
    const has=(...names:string[])=>names.some(n=>keys.has(n));
    const x=Number(has('d',...(config.mode==='fps'?[]:['arrowright'])))-Number(has('a',...(config.mode==='fps'?[]:['arrowleft'])));
    const z=Number(has('w',...(config.mode==='fps'?[]:['arrowup'])))-Number(has('s',...(config.mode==='fps'?[]:['arrowdown'])));
    if(config.mode==='fps'){aim+=(Number(has('arrowleft'))-Number(has('arrowright')))*dt;pitch=T.MathUtils.clamp(pitch+(Number(has('arrowup'))-Number(has('arrowdown')))*dt,-1.1,1.1)}
    vector.set(x,0,z);if(vector.lengthSq()>1)vector.normalize();if(config.mode==='fps'){vector.x=-vector.x;vector.applyAxisAngle(T.Object3D.DEFAULT_UP,aim)}
    const before=player.position.clone();const displacement=vector.clone().multiplyScalar(dt*config.speed*(time<boostUntil?1.7:1));if(builder)moveBody(player,displacement);else player.position.add(displacement);
    player.position.x=T.MathUtils.clamp(player.position.x,sceneBounds.min[0],sceneBounds.max[0]);player.position.z=T.MathUtils.clamp(player.position.z,sceneBounds.min[2],sceneBounds.max[2]);
    if(config.mode==='space'||config.mode==='flight')player.position.y=T.MathUtils.clamp(player.position.y+(Number(has('e'))-Number(has('q')))*dt*config.speed,Math.max(0,sceneBounds.min[1]),sceneBounds.max[1]);
    if(avatar&&(x||z)&&['exploration','transport'].includes(config.mode))avatar.root.rotation.y=Math.atan2(x,z);
    if(x||z)wheelAngle+=dt*config.speed/0.35;
    if(avatar){clip(avatar,x||z?'walk':'idle');for(const part of avatar.asset.model.moving_parts){if(part.kind==='wheel'||part.kind==='propeller')A.setPart(avatar,part.node,part.kind==='wheel'?wheelAngle:time*28)}}
    cameraUpdate(dt);if(has(' ')&&(config.mode==='fps'||config.mode==='space'))fire();
    if(builder)builder.step(dt*1000);else{
    for(const o of objects){
      const mesh=o.unit.root;if(!mesh.visible)continue;
      const d=mesh.position.distanceTo(player.position);
      if(o.role==='tree'||o.role==='obstacle'||o.role==='building'){
        o.box.setFromObject(mesh);if(o.box.clone().expandByScalar(.3).containsPoint(player.position))player.position.copy(before);
      }
      if(o.role==='enemy'){
        if(config.mode==='exploration'){mesh.position.x=o.start.x+Math.sin(time*.7)*3;clip(o.unit,'walk')}
        if(d<2 || config.mode==='fps'&&d<18&&time>5)health-=dt*(config.mode==='fps'?3:18);
      }
      if(o.role==='item'&&d<1.5 || o.role==='cargo'&&d<2){presentation?.event('pickup',mesh.position.toArray());mesh.visible=false;score++;hits++;if(o.role==='cargo')carrying=true;else if(score>=config.goal){ended=won=true}}
      if(o.role==='goal'&&d<(config.mode==='flight'?3:2.5)){
        if(config.mode==='flight'){presentation?.event('pickup',mesh.position.toArray());mesh.visible=false;score++;hits++;if(score>=config.goal){ended=won=true}}
        else if(carrying){score++;hits++;ended=won=true}
      }
    }
    }
    const moving=player.position.distanceToSquared(before)>.0001;
    if(moving&&['fps','exploration'].includes(config.mode)&&time-footstepAt>.38){footstepAt=time;presentation?.event('step',player.position.toArray())}
    if(['flight','transport','space'].includes(config.mode)){
      if(moving)presentation?.event('engine',player.position.toArray());else presentation?.audio.stop('engine');
      if(presentationPlan?.effects.some((e:any)=>e.id==='engine-trail'))presentation?.set('engine-trail',{position:player.position.toArray(),intensity:moving?1:0});
    }
    if(!builder&&(health<=0||time>=config.duration)){ended=true;won=false}
    for(const unit of live)if(!unit.procedural)A.updateInstance(unit,dt,camera);
  }
  function snapshot(){return {player_x:player.position.x,player_y:player.position.z,aim,ammo,reloads,health:builder?(sceneState.health??0):health,actions,score:builder?sceneState.score:score,hits:builder?sceneState.hits:hits,lives:builder?sceneState.lives:0,goal_remaining:builder?sceneState.goal_remaining:0,outcome:builder?sceneState.outcome:(ended?(won?1:2):0),hit_events:builder?sceneState.hit_events:hits,pickup_events:builder?sceneState.pickup_events:0,win_events:builder?sceneState.win_events:0,lose_events:builder?sceneState.lose_events:0,turns:0,spawns:objects.length,ticks:Math.floor(time),ended:Number(ended),object_count:live.length,timer_count:0,listener_count:listeners,invalid_assets:live.filter(u=>!u.root.parent).length,assets_used:live.filter(u=>u.root.visible).length,elapsed_ms:time*1000}}
  function draw(now:number){
    frame=0;if(disposed||document.hidden)return;
    const dt=last?Math.min(.05,(now-last)/1000):0;last=now;
    if(ready&&active&&!ended&&!paused){update(dt);config.step?.(dt,api);}
    presentation?.setPaused(paused||!active);
    if(ended&&!finishReported){finishReported=true;presentation?.audio.stop('engine');if(!builder)presentation?.event(won?'win':'lose',player.position.toArray())}
    builder?.render();
    if(presentation){presentation.audio.listener(camera.position.toArray(),camera.getWorldDirection(vector).toArray());presentation.update(dt);presentation.render()}else renderer.render(scene,camera);
    hud.textContent=config.objective+'\n'+(ended?(won?'COMPLETE':'GAME OVER'):paused?'PAUSED':builder?[sceneState.goal_remaining?`Goals ${sceneState.goal_remaining}`:'',sceneState.lives>0?`Lives ${sceneState.lives}`:'',sceneState.score?`Score ${sceneState.score}`:''].filter(Boolean).join(' · '):`Score ${score} · Health ${Math.ceil(health)} · ${Math.ceil(config.duration-time)}s`)+(config.mode==='fps'?` · Ammo ${ammo}`:time<boostUntil?' · BOOST':'')+'\nWASD · '+(config.mode==='fps'?'Drag / arrows: aim · Space: fire · F: reload':config.mode==='space'?'Space: fire · Q/E: altitude':config.mode==='flight'?'Space: boost · Q/E: altitude':'Space: boost')+' · R: restart';
    if(config.hud)hud.textContent=String(config.hud(api));
    frame=requestAnimationFrame(draw);
  }
  on(document,'visibilitychange',()=>{keys.clear();last=0;if(document.hidden){cancelAnimationFrame(frame);frame=0}else if(ready&&!frame&&!disposed)frame=requestAnimationFrame(draw)});
  function dispose(){
    if(disposed)return;disposed=true;ready=false;config.dispose?.(api);controller.abort();cancelAnimationFrame(frame);keys.clear();document.exitPointerLock?.();
    builder?.dispose();for(const record of [player,...objects.map(o=>o.unit.root)])for(const detach of record.userData.sceneAttachments||[])detach();clearSceneDebug();presentation?.dispose();live.forEach(unit=>{if(!unit.procedural)A.disposeInstance(unit)});loaded.forEach(A.releaseAsset);owned.forEach(o=>{o.geometry.dispose();o.material.dispose()});sun.shadow.dispose();renderer.dispose();renderer.forceContextLoss();root.replaceChildren();
    delete (window as any).__AURAGO_GAME_TEST__;
  }
  const api={scene,camera,player,renderer,get builder(){return builder},get state(){return builder?sceneState:snapshot()},event:(name:string,point?:number[])=>presentation?.event(name,point||player.position.toArray()),win:()=>builder?builder.win():(ended=won=true),lose:()=>builder?builder.lose():(ended=true,won=false),reset,dispose};
  on(window,'pagehide',dispose);
  setup().catch(error=>{if(!disposed){hud.textContent='Cannot load game: '+error.message;console.error(error);dispose()}});
  return api;
}
