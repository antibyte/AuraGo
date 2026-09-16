import * as A from '../vendor/aurago-three-assets-1.js';
import config from './reference.json';
import {referenceInput} from './reference-input';
const T=A.THREE,diving=config.mode==='diving',root=document.querySelector('#game-root');
document.querySelector('#title').textContent=diving?'Below the Brass Reef':'Letters of Marque';
const state={ready:false,score:0,health:3,outcome:'running',contacts:0,shots:0,restarts:0,frames:0,errors:[],disposed:false};globalThis.referenceState=state;
const controller=new AbortController(),handles=[],actors=[],owned=[];
let renderer,loop=0,previous=0,elapsed=0,cooldown=0,damageAt=0,paused=false,disposed=false;
const scene=new T.Scene();scene.background=new T.Color(diving?0x0d637b:0x83c4d3);scene.fog=new T.Fog(diving?0x0d637b:0x83c4d3,diving?15:60,diving?65:180);
const camera=new T.PerspectiveCamera(48,innerWidth/innerHeight,.05,220);
scene.add(new T.HemisphereLight(0xd6faff,0x37482d,3));const sun=new T.DirectionalLight(0xffe4b2,3);sun.position.set(15,40,10);scene.add(sun);
const floor=new T.Mesh(new T.PlaneGeometry(220,220,40,40),new T.MeshStandardMaterial({color:diving?0xb69b68:0x1b8498,roughness:diving?.9:.3,metalness:diving?0:.25}));floor.rotation.x=-Math.PI/2;floor.position.y=diving?-5:0;scene.add(floor);owned.push(floor);
const input=referenceInput(key=>{if(key==='KeyP')paused=!paused;else reset()});
let player;const targets=[],shots=[];
function reset(){if(!player)return;for(const a of actors){a.root.position.copy(a.spawn);a.root.visible=true;}for(const shot of shots){scene.remove(shot);shot.geometry.dispose();shot.material.dispose()}shots.length=0;Object.assign(state,{score:0,health:3,outcome:'running',contacts:0,shots:0});state.restarts++;elapsed=0;damageAt=0;cooldown=0;paused=false;}
async function model(id,at,action){
 const base='assets/builtin/aurago-pirates-3d/1.0.0/',res=await fetch(base+'assets/'+id+'.json',{signal:controller.signal});if(!res.ok)throw Error('Model metadata '+res.status);
 const meta=await res.json(),handle=await A.loadAsset(meta,id,base,{signal:controller.signal});if(disposed){A.releaseAsset(handle);throw Error('Closed')}
 handles.push(handle);const instance=A.createInstance(handle);instance.spawn=new T.Vector3(...at);instance.root.position.copy(instance.spawn);scene.add(instance.root);actors.push(instance);
 if(action&&meta.assets[0].animations.some(c=>c.id===action))A.playAction(instance,action);return instance;
}
function fire(){const shot=new T.Mesh(new T.SphereGeometry(.18,6,4),new T.MeshStandardMaterial({color:0x252c34}));shot.position.copy(player.root.position).add(new T.Vector3(1,1,0));shot.ttl=3;scene.add(shot);shots.push(shot);state.shots++;}
function update(dt){
 elapsed+=dt;cooldown-=dt;const [dx,dy]=input.axis();player.root.position.x=T.MathUtils.clamp(player.root.position.x+dx*6*dt,-12,50);
 if(diving){player.root.position.y=T.MathUtils.clamp(player.root.position.y-dy*3*dt,-3,6);player.root.rotation.y=dx<0?-Math.PI/2:Math.PI/2;}
 else {player.root.position.z=T.MathUtils.clamp(player.root.position.z+dy*6*dt,-30,30);if(dx||dy)player.root.rotation.y=Math.atan2(dx,dy);if(input.keys.has('Space')&&cooldown<=0){fire();cooldown=.4;}}
 for(const target of targets){
  if(!target.root.visible)continue;
  if(diving&&player.root.position.distanceTo(target.root.position)<1.4){target.root.visible=false;state.score++;state.contacts++;}
 }
 for(let i=shots.length-1;i>=0;i--){const s=shots[i],before=s.position.clone();s.position.x+=dt*30;s.ttl-=dt;
  for(const target of targets)if(target.root.visible){const line=new T.Line3(before,s.position),near=line.closestPointToPoint(target.root.position.clone().add(new T.Vector3(0,1,0)),true,new T.Vector3());if(near.distanceTo(target.root.position.clone().add(new T.Vector3(0,1,0)))<2.5){target.root.visible=false;state.score++;state.contacts++;s.ttl=0;break}}
  if(s.ttl<=0){scene.remove(s);s.geometry.dispose();s.material.dispose();shots.splice(i,1)}
 }
 const hazard=actors.at(-1);hazard.root.position.x=12+Math.sin(elapsed*.6)*7;hazard.root.position.z=diving?0:9+Math.cos(elapsed*.6)*7;
 if(player.root.position.distanceTo(hazard.root.position)<(diving?2:5)&&elapsed>damageAt){state.health--;state.contacts++;damageAt=elapsed+1.5;}
 if(state.score===targets.length)state.outcome='won';else if(state.health<=0)state.outcome='lost';
}
function draw(now){loop=0;if(disposed)return;const dt=previous?Math.min(.05,(now-previous)/1000):0;previous=now;
 if(!document.hidden&&!paused&&state.outcome==='running'){update(dt);for(const a of actors)A.updateInstance(a,dt,camera);}
 if(!document.hidden){camera.position.copy(player.root.position).add(new T.Vector3(diving?0:16,diving?5:25,diving?16:26));camera.lookAt(player.root.position);renderer.render(scene,camera);state.frames++;}
 document.querySelector('#status').textContent=`${state.score}/${targets.length} · Hull ${state.health} · ${paused?'Paused':state.outcome}`;loop=requestAnimationFrame(draw);
}
function resize(){if(renderer){renderer.setSize(innerWidth,innerHeight);camera.aspect=innerWidth/innerHeight;camera.updateProjectionMatrix()}}
function dispose(){if(disposed)return;disposed=true;controller.abort();input.dispose();cancelAnimationFrame(loop);for(const a of actors)A.disposeInstance(a);for(const h of handles)A.releaseAsset(h);for(const o of [...owned,...shots]){o.geometry.dispose();o.material.dispose()}renderer?.dispose();renderer?.forceContextLoss();state.disposed=true;state.cache=A.assetCacheSize()}
globalThis.disposeReference=dispose;window.addEventListener('pagehide',dispose,{signal:controller.signal});window.addEventListener('resize',resize,{signal:controller.signal});
async function start(){try{
 renderer=new T.WebGLRenderer({antialias:true});renderer.setPixelRatio(Math.min(devicePixelRatio,1.5));root.append(renderer.domElement);resize();
 player=await model(diving?'people-diver-brass':'ships-sloop',[0,diving?0:.05,0],diving?'swim':'sailing');
 for(let i=0;i<3;i++)targets.push(await model(diving?'equipment-treasure-chest':'ships-dinghy',[6+i*6,0,0],diving?null:'sailing'));
 await model(diving?'animals-reef-shark':'ships-frigate',[12,0,diving?0:12],diving?'swim':'sailing');
 state.ready=true;loop=requestAnimationFrame(draw);
}catch(e){if(!disposed){state.errors.push(String(e));document.querySelector('#status').textContent=String(e);dispose();}}}
start();
