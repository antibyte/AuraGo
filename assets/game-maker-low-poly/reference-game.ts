// Original playable acceptance scenes. The Go fixture supplies only selected metadata.
import * as A from '../vendor/aurago-three-assets-1.js';
// MODEL_IMPORTS
const MODE = 'transport';
const started=performance.now();
const T=A.THREE, base=new URL('assets/builtin/aurago-low-poly/1.0.0/',document.baseURI);
const manifest={kind:'model3d',assets:MODEL_DATA.flatMap(m=>m.assets)};
const root=document.getElementById('game-root'),abort=new AbortController();
const renderer=new T.WebGLRenderer({antialias:true});
renderer.setPixelRatio(Math.min(devicePixelRatio,1.5));renderer.setSize(innerWidth,innerHeight);
renderer.toneMapping=T.ACESFilmicToneMapping;renderer.toneMappingExposure=1.1;
renderer.shadowMap.enabled=true;renderer.shadowMap.type=T.PCFSoftShadowMap;root.append(renderer.domElement);
const scene=new T.Scene();scene.background=new T.Color(MODE==='space'?0x101b30:0x9eb9be);
scene.fog=new T.Fog(scene.background,65,160);
const camera=new T.PerspectiveCamera(50,innerWidth/innerHeight,.03,400);
const sun=new T.DirectionalLight(0xffebce,3);sun.position.set(-25,45,-20);sun.castShadow=true;
sun.shadow.mapSize.set(2048,2048);sun.shadow.normalBias=.055;sun.shadow.bias=-.00015;
Object.assign(sun.shadow.camera,{left:-42,right:42,top:42,bottom:-42,near:1,far:120});
scene.add(sun,new T.HemisphereLight(0xd8eaff,0x57634b,2.5));
const floor=new T.Mesh(new T.PlaneGeometry(260,260),new T.MeshStandardMaterial({color:0x738573,roughness:1}));
floor.rotation.x=-Math.PI/2;floor.receiveShadow=true;floor.visible=MODE!=='space';scene.add(floor);
const hud=document.createElement('div');hud.style.cssText='position:fixed;inset:18px auto auto 18px;background:#172c35ed;color:#e9dfc6;padding:18px 22px;border:1px solid #b4c4c94d;border-radius:12px;font:14px system-ui;line-height:1.65;max-width:430px;pointer-events:none';
root.append(hud);
const help={transport:'WASD / arrows: drive · collect cargo, then reach the glowing depot',
 flight:'WASD: steer · Q/E: altitude · fly through the numbered course',
 space:'WASD + Q/E: manoeuvre · Space: fire · clear the asteroid field',
 exploration:'WASD / arrows: walk · collect crystals · avoid the patrolling wolf',
 fps:'WASD: move · arrows: aim · Space: fire · F: reload',
 performance:'200 instanced objects · 20 independent animated rigs · 8 vehicles'};
const keys=new Set(),loaded=new Map(),live=[],targets=[],vehicles=[],walkers=[],shots=[];
let player,arms,weapon,yaw=0,pitch=0,loadMS=0,gateIndex=0,score=0,health=100,ammo=8,time=0,shotAt=-10,reloadUntil=0,carrying=false,paused=false,ended=false,frame=0,last=0,disposed=false,draws=0,spawnCount=0;
const samples=[],direction=new T.Vector3(),toward=new T.Vector3(),matrix=new T.Matrix4(),ray=new T.Raycaster();
function key(e,down){if(['ArrowUp','ArrowDown','ArrowLeft','ArrowRight',' '].includes(e.key))e.preventDefault();if(down)keys.add(e.key.toLowerCase());else keys.delete(e.key.toLowerCase());if(down&&!e.repeat){if(e.key.toLowerCase()==='r')reset();if(e.key.toLowerCase()==='p')paused=!paused;if(e.key.toLowerCase()==='f')reload();}}
addEventListener('keydown',e=>key(e,true),{signal:abort.signal});addEventListener('keyup',e=>key(e,false),{signal:abort.signal});
addEventListener('blur',()=>keys.clear(),{signal:abort.signal});
function resize(){renderer.setSize(innerWidth,innerHeight);camera.aspect=innerWidth/innerHeight;camera.updateProjectionMatrix()}
addEventListener('resize',resize,{signal:abort.signal});
async function add(id,at=[0,0,0],clip=null){
 let asset=loaded.get(id);if(!asset){asset=await A.loadAsset(manifest,id,base,{signal:abort.signal});loaded.set(id,asset)}
 const instance=A.createInstance(asset);instance.root.position.fromArray(at);scene.add(instance.root);live.push(instance);
 if(clip)A.playAction(instance,clip,{fade:0});return instance;
}
function action(i,name){if(i)A.playAction(i,name,{fade:.12})}
function projectile(){const mesh=new T.Mesh(new T.SphereGeometry(.07,6,4),new T.MeshBasicMaterial({color:0xffdf86}));scene.add(mesh);mesh.visible=false;return{mesh,velocity:new T.Vector3(),life:0}}
function fire(){
 if(time-shotAt<.23||time<reloadUntil||!ammo)return;shotAt=time;ammo--;spawnCount++;
 action(arms,'fire');action(weapon,'fire');
 const shot=shots.find(s=>!s.life);if(!shot)return;
 if(MODE==='fps'){shot.mesh.position.copy(camera.position);camera.getWorldDirection(direction)}
 else {shot.mesh.position.copy(player.root.position);direction.set(0,0,1)}
 shot.mesh.position.addScaledVector(direction,1.1);shot.velocity.copy(direction).multiplyScalar(42);shot.life=2;shot.mesh.visible=true;
 if(MODE==='space')ammo=8;
}
function reload(){if(!weapon||time<reloadUntil||ammo===8)return;const clip=ammo?'reload':'reload_empty';action(arms,clip);action(weapon,clip);reloadUntil=time+weapon.current.duration;}
function reset(){
 score=0;health=100;ammo=8;time=0;shotAt=-10;reloadUntil=0;gateIndex=0;yaw=0;pitch=0;carrying=false;ended=false;paused=false;spawnCount=0;keys.clear();
 if(player){player.root.position.set(0,MODE==='flight'?5:MODE==='space'?4:0,MODE==='fps'?-10:0);player.root.rotation.set(0,0,0)}
 for(const target of targets){target.instance.root.position.copy(target.start);target.instance.root.visible=true}
 for(const s of shots){s.life=0;s.mesh.visible=false}
 action(arms,'idle');action(weapon,'idle');
}
async function setup(){
 if(MODE==='transport'){
  player=await add('road-pickup');await add('architecture-warehouse',[12,0,22]);
  for(let z=-16;z<=40;z+=8)await add('landscape-road-straight',[0,.02,z]);
  const cargo=await add('props-crate-wood',[0,0,6]),depot=await add('props-checkpoint',[0,0,24]);
  targets.push({instance:cargo,start:cargo.root.position.clone(),kind:'cargo'},{instance:depot,start:depot.root.position.clone(),kind:'depot'});vehicles.push(player);
  await add('humans-mechanic-a',[5,0,22],'wave');
 }else if(MODE==='flight'){
  player=await add('aircraft-prop-plane',[0,5,0]);
  for(let i=0;i<5;i++){const gate=await add('props-checkpoint',[Math.sin(i*.9)*9,5+i%2*3,12+i*12]);gate.root.scale.setScalar(1.8);targets.push({instance:gate,start:gate.root.position.clone(),kind:'gate'})}
  await add('landscape-island',[18,-2,36]);await add('architecture-hangar',[-20,0,30]);
 }else if(MODE==='space'){
  player=await add('space-scout',[0,4,0]);sun.intensity=2;
  const planet=await add('space-planet-earth',[-30,5,65]);planet.root.scale.setScalar(2.5);
  for(let i=0;i<6;i++){const rock=await add(i%2?'space-asteroid-split':'space-asteroid-round',[(i%3-1)*7,4+Math.floor(i/3)*3,13+i*5]);targets.push({instance:rock,start:rock.root.position.clone(),kind:'rock'})}
 }else if(MODE==='exploration'){
  player=await add('humans-explorer-a',[0,0,0],'idle');await add('architecture-house',[12,0,20]);
  for(let i=0;i<16;i++)await add(i%2?'vegetation-oak':'vegetation-pine',[(i%4-1.5)*13,0,Math.floor(i/4)*13-8]);
  for(let i=0;i<5;i++){const gem=await add('props-crystal',[(i%2?1:-1)*3,0,5+i*6]);targets.push({instance:gem,start:gem.root.position.clone(),kind:'gem'})}
  const wolf=await add('animals-wolf',[8,0,15],'walk');walkers.push(wolf);
 }else if(MODE==='fps'){
  player=await add('fps-medkit',[0,0,-10]);player.root.visible=false;
  for(let z=0;z<30;z+=2)for(let x=-4;x<=4;x+=2)await add('architecture-floor',[x,0,z]);
  for(let z=0;z<30;z+=2){await add('architecture-wall',[-6,0,z]);await add('architecture-wall',[6,0,z])}
  for(let i=0;i<5;i++){const target=await add('humans-trooper-b',[(i%3-1)*2.5,0,8+i*4],'rifle_idle');targets.push({instance:target,start:target.root.position.clone(),kind:'target'})}
  arms=await add('fps-arms-modern');weapon=await add('fps-rifle');
  const view=new T.Group();view.rotation.y=Math.PI;view.position.set(.3,-.2,-.35);camera.add(view);scene.add(camera);
  view.add(arms.root,weapon.root);weapon.root.position.set(.12,.01,.145);
  arms.root.traverse(n=>{n.castShadow=false});weapon.root.traverse(n=>{n.castShadow=false});
  action(arms,'idle');action(weapon,'idle');
 }else{
  for(const [id,count] of [['vegetation-pine',100],['props-barrel-metal',100]]){
   const asset=await A.loadAsset(manifest,id,base,{signal:abort.signal});loaded.set(id,asset);
   const transforms=[];for(let i=0;i<count;i++)transforms.push(new T.Matrix4().makeTranslation((i%10-4.5)*7,0,Math.floor(i/10)*7-28+(id.includes('crate')?3:0)));
   const batch=A.createInstances(asset,transforms,1);live.push(batch);scene.add(batch.root);
  }
  for(let i=0;i<20;i++){const unit=await add(i%2?'animals-dog':'humans-civilian-a',[(i%10-4.5)*4,0,Math.floor(i/10)*10],'walk');walkers.push(unit)}
  for(let i=0;i<8;i++)vehicles.push(await add('road-sedan',[(i-3.5)*5,0,-14]));
 }
 for(let i=0;i<16;i++)shots.push(projectile());
 reset();window.__referenceReady=true;
}
function update(dt){
 time+=dt;
 if(reloadUntil&&time>=reloadUntil){ammo=8;reloadUntil=0;action(arms,'idle');action(weapon,'idle')}
 const x=(keys.has('d')||MODE!=='fps'&&keys.has('arrowright')?1:0)-(keys.has('a')||MODE!=='fps'&&keys.has('arrowleft')?1:0);
 const z=(keys.has('w')||MODE!=='fps'&&keys.has('arrowup')?1:0)-(keys.has('s')||MODE!=='fps'&&keys.has('arrowdown')?1:0);
 const y=(keys.has('e')?1:0)-(keys.has('q')?1:0);
 if(player){
  const speed=MODE==='transport'?9:MODE==='flight'?12:MODE==='space'?10:MODE==='exploration'?3:4;
  player.root.position.x=T.MathUtils.clamp(player.root.position.x+x*dt*speed,-35,35);
  player.root.position.z=T.MathUtils.clamp(player.root.position.z+z*dt*speed,-20,90);
  if(MODE==='flight'||MODE==='space')player.root.position.y=T.MathUtils.clamp(player.root.position.y+y*dt*speed,2,22);
  if((x||z)&&MODE!=='fps'&&MODE!=='space'&&MODE!=='flight')player.root.rotation.y=Math.atan2(x,z);
  if(MODE==='exploration')action(player,x||z?'walk':'idle');
  if(MODE==='fps'){
   yaw+=((keys.has('arrowleft')?1:0)-(keys.has('arrowright')?1:0))*dt*.85;
   pitch=T.MathUtils.clamp(pitch+((keys.has('arrowup')?1:0)-(keys.has('arrowdown')?1:0))*dt*.65,-.7,.7);
   camera.position.copy(player.root.position).add(toward.set(0,1.65,0));
   camera.rotation.set(pitch,Math.PI+yaw,0,'YXZ');
  }else{
   toward.copy(player.root.position).add(direction.set(12,MODE==='flight'?9:14,-19));camera.position.lerp(toward,1-Math.exp(-dt*6));
   camera.lookAt(player.root.position.x,player.root.position.y+1,player.root.position.z+4);
  }
  if(keys.has(' ')&&(MODE==='space'||MODE==='fps'))fire();
  for(const target of targets){
   if(!target.instance.root.visible)continue;
   const distance=target.instance.root.position.distanceTo(player.root.position);
   if(target.kind==='cargo'&&distance<2){carrying=true;target.instance.root.visible=false}
   if(target.kind==='depot'&&carrying&&distance<3){score=1;ended=true}
   if(target.kind==='gate'&&target===targets[gateIndex]&&distance<4){target.instance.root.visible=false;gateIndex++;score++;if(score===targets.length)ended=true}
   if(target.kind==='gem'&&distance<1.4){target.instance.root.visible=false;score++;if(score===targets.length)ended=true}
   if(target.kind==='rock'){target.instance.root.rotation.y+=dt*.15;if(distance<2){health-=dt*30}}
   if(target.kind==='target'){target.instance.root.position.x=target.start.x+Math.sin(time*.5+target.start.z)*.6;if(time>8)health-=dt*.45}
  }
  if(time>90||health<=0)ended=true;
 }else{camera.position.set(40,32,-42);camera.lookAt(0,0,4)}
 for(const walker of walkers){walker.root.position.z=Math.sin(time*.25)*9;walker.root.rotation.y=Math.cos(time*.25)>0?0:Math.PI;if(MODE==='exploration'&&walker.root.position.distanceTo(player.root.position)<1.5)health-=dt*25}
 for(const vehicle of vehicles){
  for(const part of vehicle.asset.model.moving_parts){if(part.kind==='wheel')A.setPart(vehicle,part.node,time*4)}
  if(MODE==='performance')vehicle.root.position.z=-14+Math.sin(time*.3)*12;
 }
 if(MODE==='flight')for(const part of player.asset.model.moving_parts)if(part.kind==='propeller')A.setPart(player,part.node,time*28);
 for(const shot of shots){
  if(!shot.life)continue;shot.life=Math.max(0,shot.life-dt);shot.mesh.position.addScaledVector(shot.velocity,dt);shot.mesh.visible=!!shot.life;
  for(const target of targets){if(!target.instance.root.visible||!['target','rock'].includes(target.kind))continue;
   toward.copy(target.instance.root.position);if(target.kind==='target')toward.y+=1;
   if(shot.mesh.position.distanceTo(toward)<(target.kind==='target'?1:2)){
    shot.life=0;shot.mesh.visible=false;target.instance.root.visible=false;score++;if(score===targets.length)ended=true;break;
   }
  }
 }
 for(const instance of live)A.updateInstance(instance,dt,camera);
}
function draw(now){
 frame=0;if(disposed||document.hidden)return;
 const dt=last?Math.min(.05,(now-last)/1000):0;if(last&&MODE==='performance'&&time>2)samples.push(now-last);last=now;
 if(!paused&&!ended)update(dt);
 renderer.render(scene,camera);draws++;
 // Benchmark only: wait for GPU completion rather than measuring queued commands.
 if(MODE==='performance')renderer.getContext().finish();
 if(!loadMS)loadMS=performance.now()-started;
 hud.innerHTML='<b>AURAGO LOW POLY / '+MODE.toUpperCase()+'</b><br>'+help[MODE]+'<br>'+
  (ended?(health>0&&time<=90?'COMPLETE':'TIME / HEALTH DEPLETED'):paused?'PAUSED':'Score '+score+' · Health '+Math.ceil(health)+' · '+Math.ceil(90-time)+'s'+(MODE==='fps'?' · Ammo '+ammo:''))+
  '<br>R: restart · P: pause'+(carrying?' · Cargo loaded':'');
 frame=requestAnimationFrame(draw);
}
function visibility(){last=0;keys.clear();if(document.hidden){cancelAnimationFrame(frame);frame=0}else if(!frame)frame=requestAnimationFrame(draw)}
document.addEventListener('visibilitychange',visibility,{signal:abort.signal});
function dispose(){
 if(disposed)return;disposed=true;abort.abort();cancelAnimationFrame(frame);
 live.forEach(A.disposeInstance);loaded.forEach(A.releaseAsset);shots.forEach(s=>{s.mesh.geometry.dispose();s.mesh.material.dispose()});
 floor.geometry.dispose();floor.material.dispose();sun.shadow.dispose();renderer.dispose();renderer.forceContextLoss();root.replaceChildren();
}
addEventListener('pagehide',dispose,{once:true});
window.__reference={snapshot:()=>({mode:MODE,score,health,ammo,time,ended,carrying,draws,spawnCount,position:player?.root.position.toArray(),objects:live.length,cache:A.assetCacheSize()}),reset,dispose,
 performance:()=>{const sorted=samples.slice().sort((a,b)=>a-b),gl=renderer.getContext(),debug=gl.getExtension('WEBGL_debug_renderer_info');return{samples:sorted.length,loadMS,p50:sorted[Math.floor(sorted.length*.5)],p95:sorted[Math.floor(sorted.length*.95)],fps:1000/(samples.reduce((a,b)=>a+b,0)/samples.length),gpuCompletion:true,viewport:[innerWidth,innerHeight],renderer:gl.getParameter(debug?debug.UNMASKED_RENDERER_WEBGL:gl.RENDERER),triangles:renderer.info.render.triangles,calls:renderer.info.render.calls}}};
setup().then(()=>{frame=requestAnimationFrame(draw)}).catch(error=>{window.__referenceError=error.message;hud.textContent='Model loading failed: '+error.message;console.error(error)});
