import * as A from '../vendor/aurago-three-assets-1.js';
import {preloadPack,registerAnimations,createAsset,playAction,setFacing,inspectAssets,setAssetLayerVisible,fitVisual,getAssetSocket} from '../vendor/aurago-game-1.js';
import {projectIso,createIsometricWorld} from '../vendor/isometric.js';
const result={errors:[],models:[],frames:0,pixels:null,disposed:false};window.worldReview=result;
window.addEventListener('error',event=>result.errors.push(event.message));
window.addEventListener('unhandledrejection',event=>result.errors.push(String(event.reason)));
const {THREE}=A,scene=new THREE.Scene();scene.background=new THREE.Color('#213948');
const camera=new THREE.PerspectiveCamera(42,1,0.01,300);camera.position.set(15,13,22);camera.lookAt(0,2,0);
const renderer=new THREE.WebGLRenderer({antialias:true});renderer.setSize(800,700);document.querySelector('#models').append(renderer.domElement);
scene.add(new THREE.HemisphereLight(0xffffff,0x536879,3));const sun=new THREE.DirectionalLight(0xffe5c1,4);sun.position.set(10,20,10);scene.add(sun);
const handles=[],instances=[];const abort=new AbortController();
try {
 const base='assets/builtin/aurago-pirates-3d/1.0.0/';
 for(const [id,at,action] of [['ships-sloop',[-3,0,0],'sailing'],['people-diver-brass',[3,0,3],'walk'],['people-diver-brass',[5,0,3],'swim'],['animals-reef-shark',[3,2,-3],'swim']]) {
  const meta=await fetch(base+'assets/'+id+'.json').then(r=>{if(!r.ok)throw Error(r.status);return r.json()});
  const handle=await A.loadAsset(meta,id,base,{signal:abort.signal}),instance=A.createInstance(handle);
  handles.push(handle);instances.push(instance);instance.root.position.set(...at);scene.add(instance.root);A.playAction(instance,action);
  result.models.push({id,action,meshes:0});instance.root.traverse(o=>{if(o.isMesh)result.models.at(-1).meshes++});
 }
 const [a,b]=instances.slice(1,3),sa=[],sb=[];a.root.traverse(o=>{if(o.isSkinnedMesh)sa.push(o.skeleton)});b.root.traverse(o=>{if(o.isSkinnedMesh)sb.push(o.skeleton)});
 result.independent=sa.length>0&&sb.length>0&&sa[0]!==sb[0]&&sa[0].bones[0]!==sb[0].bones[0];
 const base2='assets/builtin/aurago-pirates-side/1.0.0/';const meta=await fetch(base2+'assets/people-diver-brass.json').then(r=>r.json());
 const house=await fetch(base2+'assets/harbor-beach-hut.json').then(r=>r.json());
 class Review extends Phaser.Scene {
  preload(){preloadPack(this,meta,base2);preloadPack(this,house,base2)}
  create(){registerAnimations(this,meta);registerAnimations(this,house);this.a=createAsset(this,meta,'people-diver-brass',200,260).setScale(3);this.b=createAsset(this,meta,'people-diver-brass',560,260).setScale(3);setFacing(this.b,-1,0);playAction(this.a,'walk');playAction(this.b,'swim');this.seen=new Set();
    this.listenersBefore=this.events.listenerCount('postupdate');this.hut=createAsset(this,house,'harbor-beach-hut',420,550);fitVisual(this.hut,270,200);
    const entrance=getAssetSocket(this.hut,'entrance');setFacing(this.hut,-1,0);const reverse=getAssetSocket(this.hut,'entrance');setFacing(this.hut,1,0);
    const head=getAssetSocket(this.a,'head');
    result.sockets=Number.isFinite(entrance.x)&&entrance.x>this.hut.x&&reverse.x<this.hut.x&&Math.abs(entrance.y-reverse.y)<0.01&&head.y<this.a.y;
    window.reviewRoof=()=>{const before=this.children.list.filter(s=>s.visible).length;setAssetLayerVisible(this.hut,'roof',false);return {hidden:before-this.children.list.filter(s=>s.visible).length,scale:this.hut.scaleX};};
    window.reviewLayerCleanup=()=>{const before=this.children.list.length;this.hut.destroy();return {removed:before-this.children.list.length,listeners:this.events.listenerCount('postupdate')-this.listenersBefore};};
  }
  update(time,delta){this.seen.add(this.a.frame.name);result.pixels={...inspectAssets(this),seen_frames:this.seen.size};for(const instance of instances)A.updateInstance(instance,Math.min(delta,50)/1000,camera);renderer.render(scene,camera);result.frames++;if(result.frames>60){result.ready=true;document.querySelector('#result').textContent=JSON.stringify(result,null,2)}}
 }
 const game=new Phaser.Game({type:Phaser.AUTO,width:800,height:700,parent:'pixels',backgroundColor:'#314854',pixelArt:true,scene:Review,audio:{noAudio:true}});
 window.disposeWorldReview=()=>{abort.abort();game.destroy(true);for(const i of instances)A.disposeInstance(i);for(const h of handles)A.releaseAsset(h);renderer.dispose();renderer.forceContextLoss();result.disposed=true;result.cache=A.assetCacheSize()};
}catch(error){result.errors.push(String(error));document.querySelector('#result').textContent=JSON.stringify(result)}
