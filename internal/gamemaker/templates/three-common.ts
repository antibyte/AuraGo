// Original MIT game bases, adapted from AuraGo's playable low-poly references.
import * as A from '../vendor/aurago-three-assets-1.js';
import {createPresentation,createThreeAdapter} from '../vendor/aurago-effects-3d-1.js';
import presentationPlan from './presentation.json';
// PLAN_MODEL_IMPORTS
const roles: any = {};
const T = A.THREE;

export function startGame(config: any) {
  const root = document.getElementById('game-root')!;
  const controller = new AbortController(), signal = controller.signal;
  const renderer = new T.WebGLRenderer({antialias:true, preserveDrawingBuffer:true});
  renderer.setPixelRatio(Math.min(devicePixelRatio,1.5));
  renderer.toneMapping = T.ACESFilmicToneMapping;
  renderer.shadowMap.enabled = true; renderer.shadowMap.type = T.PCFSoftShadowMap;
  root.append(renderer.domElement);
  const scene = new T.Scene(), camera = new T.PerspectiveCamera(65,1,.015,240);
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
  const clip=(unit:any,name:string,restart=false)=>{
    if(!unit)return;
    if(unit.asset.model.animations.some((c:any)=>c.id===name))A.playAction(unit,name,{fade:.12,restart});
  };
  const weaponAction=(name:string)=>{const prefix=weapon?.asset.model.fps_binding?.arm_action_prefix||'';clip(arms,prefix+name,true);clip(weapon,name,true)};
  const resize=()=>{renderer.setSize(root.clientWidth||innerWidth,root.clientHeight||innerHeight);camera.aspect=renderer.domElement.width/renderer.domElement.height;camera.updateProjectionMatrix();presentation?.resize()};
  function reset(){
    presentation?.reset();presentation?.setPaused(false);footstepAt=0;finishReported=false;
    time=score=hits=actions=reloads=0;health=100;ammo=8;ended=won=carrying=paused=false;aim=pitch=reloadAt=boostUntil=wheelAngle=0;shotAt=-1;keys.clear();
    player.position.set(0,config.mode==='flight'?5:config.mode==='space'?4:0,0);player.rotation.set(0,0,0);
    for(const o of objects){o.unit.root.position.copy(o.start);o.unit.root.visible=true}
    if(avatar)clip(avatar,'idle');weaponAction('idle');cameraUpdate(1);
  }
  function fire(){
    if(ended||!ready||time-shotAt<.22||time<reloadAt)return;
    if(config.mode==='fps'&&!ammo){reload();return}
    shotAt=time;actions++;if(config.mode==='fps'){ammo--;weaponAction('fire')}
    if(config.mode==='fps'){camera.updateMatrixWorld();ray.setFromCamera({x:0,y:0},camera)}
    else {vector.set(0,0,1);ray.set(player.position,vector)}
    presentation?.event('shot',camera.position.clone().addScaledVector(ray.ray.direction,.8).toArray());
    let closest:any=null,distance=80,hitPoint=new T.Vector3(),hitNormal=new T.Vector3();
    for(const o of objects){
      if(o.role!=='enemy'||!o.unit.root.visible)continue;
      o.unit.root.updateMatrixWorld(true);const contact=ray.intersectObject(o.unit.root,true)[0];
      if(contact&&contact.distance<distance){distance=contact.distance;closest=o;hitPoint.copy(contact.point);hitNormal.copy(contact.face?.normal||ray.ray.direction.clone().negate()).transformDirection(contact.object.matrixWorld)}
    }
    if(closest){presentation?.event('hit',hitPoint.toArray(),hitNormal.toArray(),config.mode==='space'?'metal':'flesh');closest.unit.root.visible=false;score++;hits++;if(score>=config.goal){ended=won=true}}
  }
  function reload(){
    if(config.mode!=='fps'||ended||reloadAt>time||ammo===8)return;
    presentation?.event('reload',camera.position.toArray());
    const name=ammo?'reload':'reload_empty';weaponAction(name);
    reloadAt=time+(weapon?.asset.model.animations.find((c:any)=>c.id===name)?.duration||1.5);
  }
  function primary(){if(config.mode==='fps'||config.mode==='space')fire();else if(!ended&&ready){boostUntil=time+1.5;actions++}}
  function key(event:KeyboardEvent,down:boolean){
    const name=event.key.toLowerCase();if([' ','arrowup','arrowdown','arrowleft','arrowright'].includes(name))event.preventDefault();
    if(down)keys.add(name);else keys.delete(name);
    if(down&&!event.repeat){if(name==='r')reset();if(name==='f')reload();if(name==='escape'){ended=true;document.exitPointerLock?.()}if(name===' ')primary();if(name==='p')paused=!paused}
  }
  on(window,'keydown',(e:KeyboardEvent)=>key(e,true));on(window,'keyup',(e:KeyboardEvent)=>key(e,false));
  on(window,'message',(e:MessageEvent)=>{if(e.source===parent&&e.data?.type==='aurago:game:active')presentation?.setActive(e.data.active===true)});
  on(window,'blur',()=>{keys.clear();dragging=false;document.exitPointerLock?.()});on(window,'resize',resize);
  on(renderer.domElement,'pointerdown',(e:PointerEvent)=>{dragging=true;if(e.isTrusted)renderer.domElement.setPointerCapture?.(e.pointerId);if(config.mode==='fps'){primary();if(e.isTrusted&&e.pointerType==='mouse')renderer.domElement.requestPointerLock?.()?.catch?.(()=>{})}});
  on(renderer.domElement,'pointerup',()=>dragging=false);
  on(renderer.domElement,'pointermove',(e:PointerEvent)=>{if(config.mode==='fps'&&(dragging||document.pointerLockElement===renderer.domElement)){aim-=e.movementX*.003;pitch=T.MathUtils.clamp(pitch-e.movementY*.003,-1.1,1.1)}});
  for(const [label,keyName] of [['◀','a'],['▲','w'],['▼','s'],['▶','d'],['Action',' '],['Reload','f'],['Restart','r'],['Descend','q'],['Ascend','e']]){
    if(keyName==='f'&&config.mode!=='fps'||['q','e'].includes(keyName)&&!['flight','space'].includes(config.mode))continue;
    const b=document.createElement('button');b.textContent=label;b.setAttribute('aria-label',label);b.style.cssText='pointer-events:auto;touch-action:none;border:1px solid #8eafc666;border-radius:9px;background:#152635dc;color:white;padding:12px 16px;min-height:44px';
    on(b,'pointerdown',(e:PointerEvent)=>{e.preventDefault();b.setPointerCapture(e.pointerId);keys.add(keyName);if(keyName===' ')primary();if(keyName==='f')reload();if(keyName==='r')reset()});
    on(b,'pointerup',()=>keys.delete(keyName));on(b,'pointercancel',()=>keys.delete(keyName));controls.append(b);
  }
  async function instantiate(role:string,at:number[]){
    const asset=loaded.get(role);if(!asset)throw Error('Unknown role '+role+'. Available: '+Object.keys(roles).join(', '));
    const unit=A.createInstance(asset);unit.root.scale.setScalar(roles[role].scale);unit.root.position.fromArray(at);scene.add(unit.root);live.push(unit);return unit;
  }
  async function setup(){
    for(const [role,spec] of Object.entries(roles) as any){
      const asset=await A.loadAsset(spec.manifest,spec.id,spec.base,{signal});
      if(disposed){A.releaseAsset(asset);return}loaded.set(role,asset);
    }
    if(loaded.has('player')){avatar=await instantiate('player',[0,0,0]);player.add(avatar.root);clip(avatar,'idle')}
    for(const spec of config.objects){
      if(!Array.isArray(spec.at)||spec.at.length!==3||!spec.at.every(Number.isFinite))throw Error('Each object requires at:[x,y,z]');
      const unit=await instantiate(spec.role,spec.at);clip(unit,'idle');
      if(['building','terrain','obstacle'].includes(spec.role))presentation?.registerSurface(unit.root,{kind:spec.role==='terrain'?'ground':'roof'});
      objects.push({role:spec.role,unit,start:unit.root.position.clone(),box:new T.Box3()});
    }
    if(config.mode==='fps'){
      arms=await instantiate('arms',[0,0,0]);weapon=await instantiate('weapon',[0,0,0]);
      const view=new T.Group();view.rotation.y=Math.PI;view.position.set(.25,-.22,-.35);camera.add(view);view.add(arms.root,weapon.root);
      weapon.root.position.fromArray(weapon.asset.model.fps_binding.weapon_translation);
      for(const unit of [arms,weapon])unit.root.traverse((n:any)=>n.castShadow=false);weaponAction('idle');
    }
    ready=true;reset();resize();
    const binding={kind:'three',canvas:renderer.domElement,alive:()=>ready&&!disposed,snapshot:()=>snapshot()};
    (window as any).__AURAGO_GAME_TEST__=binding;
    frame=requestAnimationFrame(draw);
  }
  function cameraUpdate(dt:number){
    if(config.mode==='fps'){camera.position.copy(player.position).add(vector.set(0,1.65,0));camera.rotation.set(pitch,Math.PI+aim,0,'YXZ')}
    else {position.copy(player.position).add(vector.set(10,12,-17));camera.position.lerp(position,1-Math.exp(-dt*8));camera.lookAt(player.position.x,player.position.y+1,player.position.z+3)}
  }
  function update(dt:number){
    time+=dt;if(reloadAt&&time>=reloadAt){reloadAt=0;ammo=8;reloads++;weaponAction('idle')}
    const has=(...names:string[])=>names.some(n=>keys.has(n));
    const x=Number(has('d',...(config.mode==='fps'?[]:['arrowright'])))-Number(has('a',...(config.mode==='fps'?[]:['arrowleft'])));
    const z=Number(has('w',...(config.mode==='fps'?[]:['arrowup'])))-Number(has('s',...(config.mode==='fps'?[]:['arrowdown'])));
    if(config.mode==='fps'){aim+=(Number(has('arrowleft'))-Number(has('arrowright')))*dt;pitch=T.MathUtils.clamp(pitch+(Number(has('arrowup'))-Number(has('arrowdown')))*dt,-1.1,1.1)}
    vector.set(x,0,z);if(vector.lengthSq()>1)vector.normalize();if(config.mode==='fps'){vector.x=-vector.x;vector.applyAxisAngle(T.Object3D.DEFAULT_UP,aim)}
    const before=player.position.clone();player.position.addScaledVector(vector,dt*config.speed*(time<boostUntil?1.7:1));
    player.position.x=T.MathUtils.clamp(player.position.x,-45,45);player.position.z=T.MathUtils.clamp(player.position.z,-20,170);
    if(config.mode==='space'||config.mode==='flight')player.position.y=T.MathUtils.clamp(player.position.y+(Number(has('e'))-Number(has('q')))*dt*config.speed,1,30);
    if(avatar&&(x||z)&&['exploration','transport'].includes(config.mode))avatar.root.rotation.y=Math.atan2(x,z);
    if(x||z)wheelAngle+=dt*config.speed/0.35;
    if(avatar){clip(avatar,x||z?'walk':'idle');for(const part of avatar.asset.model.moving_parts){if(part.kind==='wheel'||part.kind==='propeller')A.setPart(avatar,part.node,part.kind==='wheel'?wheelAngle:time*28)}}
    cameraUpdate(dt);if(has(' ')&&(config.mode==='fps'||config.mode==='space'))fire();
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
    const moving=player.position.distanceToSquared(before)>.0001;
    if(moving&&['fps','exploration'].includes(config.mode)&&time-footstepAt>.38){footstepAt=time;presentation?.event('step',player.position.toArray())}
    if(['flight','transport','space'].includes(config.mode)){
      if(moving)presentation?.event('engine',player.position.toArray());else presentation?.audio.stop('engine');
      if(presentationPlan?.effects.some((e:any)=>e.id==='engine-trail'))presentation?.set('engine-trail',{position:player.position.toArray(),intensity:moving?1:0});
    }
    if(health<=0||time>=config.duration){ended=true;won=false}
    for(const unit of live)A.updateInstance(unit,dt,camera);
  }
  function snapshot(){return {player_x:player.position.x,player_y:player.position.z,aim,ammo,reloads,health,actions,score,hits,turns:0,spawns:objects.length,ticks:Math.floor(time),ended:Number(ended),object_count:live.length,timer_count:0,listener_count:listeners,invalid_assets:live.filter(u=>!u.root.parent).length,assets_used:live.filter(u=>u.root.visible).length,elapsed_ms:time*1000}}
  function draw(now:number){
    frame=0;if(disposed||document.hidden)return;
    const dt=last?Math.min(.05,(now-last)/1000):0;last=now;
    if(ready&&!ended&&!paused)update(dt);
    presentation?.setPaused(paused);
    if(ended&&!finishReported){finishReported=true;presentation?.audio.stop('engine');presentation?.event(won?'win':'lose',player.position.toArray())}
    if(presentation){presentation.audio.listener(camera.position.toArray(),camera.getWorldDirection(vector).toArray());presentation.update(dt);presentation.render()}else renderer.render(scene,camera);
    hud.textContent=config.objective+'\n'+(ended?(won?'COMPLETE':'GAME OVER'):paused?'PAUSED':`Score ${score} · Health ${Math.ceil(health)} · ${Math.ceil(config.duration-time)}s`)+(config.mode==='fps'?` · Ammo ${ammo}`:time<boostUntil?' · BOOST':'')+'\nWASD · '+(config.mode==='fps'?'Drag / arrows: aim · Space: fire · F: reload':config.mode==='space'?'Space: fire · Q/E: altitude':config.mode==='flight'?'Space: boost · Q/E: altitude':'Space: boost')+' · R: restart';
    frame=requestAnimationFrame(draw);
  }
  on(document,'visibilitychange',()=>{keys.clear();last=0;if(document.hidden){cancelAnimationFrame(frame);frame=0}else if(ready&&!frame&&!disposed)frame=requestAnimationFrame(draw)});
  function dispose(){
    if(disposed)return;disposed=true;ready=false;controller.abort();cancelAnimationFrame(frame);keys.clear();document.exitPointerLock?.();
    presentation?.dispose();live.forEach(A.disposeInstance);loaded.forEach(A.releaseAsset);owned.forEach(o=>{o.geometry.dispose();o.material.dispose()});sun.shadow.dispose();renderer.dispose();renderer.forceContextLoss();root.replaceChildren();
    delete (window as any).__AURAGO_GAME_TEST__;
  }
  on(window,'pagehide',dispose);
  setup().catch(error=>{if(!disposed){hud.textContent='Cannot load game: '+error.message;console.error(error);dispose()}});
  return {dispose};
}
