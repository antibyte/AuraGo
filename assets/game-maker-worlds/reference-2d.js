import {preloadPack,registerAnimations,createAsset,playAction,setFacing} from '../vendor/aurago-game-1.js';
import config from './reference.json';
import {referenceInput} from './reference-input';
const diving=config.mode==='diving',pack='aurago-pirates-'+(diving?'side':'topdown'),base='assets/builtin/'+pack+'/1.0.0/';
const state={ready:false,score:0,health:3,outcome:'running',contacts:0,shots:0,restarts:0,frames:0,errors:[],disposed:false};globalThis.referenceState=state;
document.querySelector('#title').textContent=diving?'The Sunken Brass Key':'Archipelago Patrol';
const abort=new AbortController();let game,paused=false;
const input=referenceInput(key=>{if(key==='KeyP')paused=!paused;else {paused=false;game?.scene.getScene('maritime').scene.restart();state.restarts++;}});
async function metadata(id){const r=await fetch(base+'assets/'+id+'.json',{signal:abort.signal});if(!r.ok)throw Error(id+': '+r.status);return r.json()}
async function start(){try{
 const playerID=diving?'people-diver-brass':'ships-sloop',targetID=diving?'equipment-treasure-chest':'ships-dinghy',hazardID=diving?'animals-reef-shark':'ships-frigate';
 const [playerMeta,targetMeta,hazardMeta]=await Promise.all([playerID,targetID,hazardID].map(metadata));
 class Maritime extends Phaser.Scene {
  constructor(){super('maritime')}
  preload(){for(const m of [playerMeta,targetMeta,hazardMeta])preloadPack(this,m,base)}
  create(){
   Object.assign(state,{score:0,health:3,outcome:'running',contacts:0,shots:0});this.elapsed=0;this.cooldown=0;this.damageAt=0;this.shots=[];
   for(const m of [playerMeta,targetMeta,hazardMeta])registerAnimations(this,m);
   const floor=this.add.graphics();floor.fillStyle(diving?0x164f67:0x227e94);floor.fillRect(0,0,960,540);
   floor.fillStyle(0xb2a572);if(diving)floor.fillRect(0,470,960,70);else for(const [x,y]of [[220,100],[700,420],[830,90]])floor.fillEllipse(x,y,120,70);
   this.player=createAsset(this,playerMeta,playerID,110,270).setScale(diving?2:1);setFacing(this.player,1,0);playAction(this.player,diving?'swim':'sailing');
   this.targets=[320,530,740].map(x=>createAsset(this,targetMeta,targetID,x,270).setScale(diving?1:1));
   this.hazard=createAsset(this,hazardMeta,hazardID,560,diving?300:360);playAction(this.hazard,diving?'swim':'sailing');
   state.ready=true;
  }
  update(_,delta){
   state.frames++;if(paused||document.hidden){this.anims.pauseAll();return}this.anims.resumeAll();if(state.outcome!=='running')return;
   const dt=Math.min(.05,delta/1000);this.elapsed+=dt;this.cooldown-=dt;
   const [dx,dy]=input.axis();this.player.x=Phaser.Math.Clamp(this.player.x+dx*160*dt,35,925);this.player.y=Phaser.Math.Clamp(this.player.y+dy*160*dt,80,diving?445:500);if(dx||dy)setFacing(this.player,dx,dy);
   if(!diving&&input.keys.has('Space')&&this.cooldown<=0){this.shots.push(this.add.circle(this.player.x+35,this.player.y,4,0x252c34));state.shots++;this.cooldown=.3;}
   for(const t of this.targets)if(t.active&&diving&&Phaser.Math.Distance.Between(this.player.x,this.player.y,t.x,t.y)<30){t.destroy();state.score++;state.contacts++;}
   for(let i=this.shots.length-1;i>=0;i--){const shot=this.shots[i],old=shot.x;shot.x+=dt*420;
    for(const t of this.targets)if(t.active&&t.x>=old-25&&t.x<=shot.x+25&&Math.abs(t.y-shot.y)<28){t.destroy();state.score++;state.contacts++;shot.x=1000;break;}
    if(shot.x>980){shot.destroy();this.shots.splice(i,1)}
   }
   this.hazard.x=560+Math.sin(this.elapsed)*120;setFacing(this.hazard,Math.cos(this.elapsed),0);
   if(Phaser.Math.Distance.Between(this.player.x,this.player.y,this.hazard.x,this.hazard.y)<(diving?30:55)&&this.elapsed>this.damageAt){state.health--;state.contacts++;this.damageAt=this.elapsed+1.2;}
   if(state.score===3)state.outcome='won';else if(state.health<=0)state.outcome='lost';
   document.querySelector('#status').textContent=`${state.score}/3 · Health ${state.health} · ${state.outcome}`;
  }
 }
 game=new Phaser.Game({type:Phaser.AUTO,parent:'game-root',width:960,height:540,pixelArt:true,backgroundColor:'#164f67',scene:Maritime,audio:{noAudio:true},scale:{mode:Phaser.Scale.FIT,autoCenter:Phaser.Scale.CENTER_BOTH}});
}catch(e){state.errors.push(String(e));document.querySelector('#status').textContent=String(e)}}
start();
globalThis.disposeReference=()=>{if(state.disposed)return;abort.abort();input.dispose();game?.destroy(true);state.disposed=true};
window.addEventListener('pagehide',globalThis.disposeReference,{signal:abort.signal});
