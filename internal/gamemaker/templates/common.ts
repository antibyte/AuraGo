import { createInputs, bindGameTest, preloadPack, registerAnimations, createAsset, createAssembly, fitVisual, playAction, setFacing } from '../vendor/aurago-game-1.js';
import {createPresentation, createPhaserAdapter} from '../vendor/aurago-effects-2d-1.js';
import {createScene2D, hasSceneNodes} from '../vendor/scene-builder.js';
import {createIsometricWorld, projectIso} from '../vendor/isometric.js';
import {createGameFlow, levelChoices, sceneForLevel} from '../vendor/game-flow.js';
import presentationPlan from './presentation.json';
import scenePlan from './scene.json';
import mechanicsPlan from './mechanics.json';
declare const Phaser: any;
// PLAN_ASSET_IMPORTS
const plannedAssets: any = {};

// Keep this lifecycle when adapting a template. State belongs to each scene run.
export class GameScene extends Phaser.Scene {
  player: any; inputKeys: any; hud: any;
  state: any; elapsed = 0; presentation: any; builder: any; isometric: any; isoPlayerID = ''; sceneDebugGraphics: any; footstepAt = 0; wasGrounded = false;
  visuals: any[] = []; gameObjects:any[]=[]; gameEvents:any={}; gameTrace:any[]=[]; sceneSolids:any; sceneDebugLabels: any[] = []; manualPause = false; previewActive = true;
  flow:any; levels:any[]=[]; levelIndex=Math.max(0,(scenePlan as any)?.levels?.findIndex((l:any)=>l.active)??0); checkpoint:any; respawnAt=0; invulnerableUntil=0; stageCarry:any=null;
  constructor() { super('main'); }
  preload() {
    for (const meta of new Set(Object.values(plannedAssets).map((a:any)=>a.meta))) {
      preloadPack(this, meta, `assets/builtin/${meta.id}/${meta.version}/${meta.schema_version === 2 ? '' : 'sheet.png'}`);
    }
  }
  create() {
    if (this.update !== GameScene.prototype.update) {
      throw Error('GameScene.update must be inherited from common.ts: it handles ESC end, R restart, input, elapsed time and planned sprite following. Remove the update override; put continuous gameplay in step(deltaSeconds), primary input in action(), and HUD changes in paintHUD().');
    }
    this.physics.resume();
    this.elapsed = 0; this.manualPause=false;this.time.paused=false;this.respawnAt=this.invulnerableUntil=0;this.checkpoint=null;this.player=null;
    this.levels=levelChoices(scenePlan,this.levels);
    this.visuals = [];this.gameObjects=[];this.gameEvents={};this.gameTrace=[];
    for (const meta of new Set(Object.values(plannedAssets).map((a:any)=>a.meta))) registerAnimations(this, meta);
    this.state = { score: 0, actions: 0, hits: 0, spawns: 0, turns: 0, ended: 0, ticks: 0, lives: 0, goal_remaining: 0, outcome: 0, hit_events: 0, pickup_events: 0, win_events: 0, lose_events: 0 };
    this.inputKeys = createInputs(this);
    this.hud = this.add.text(18, 16, '', { fontFamily: 'monospace', fontSize: '20px', color: '#ffffff' }).setDepth(1000).setScrollFactor(0);
    this.footstepAt = 0; this.wasGrounded = false;
    this.presentation = createPresentation({config:{...presentationPlan,feedback:true},root:document.getElementById('game-root'),adapter:createPhaserAdapter({scene:this,view:this.physics.world.gravity.y?'side':'top'}),report:(message:any)=>console.warn(message)});
    const active=(value:boolean)=>{this.previewActive=value;this.time.paused=!value||this.manualPause;this.presentation?.setActive(value);if(!value)this.physics.pause();else if(!this.manualPause&&!this.state.ended&&!this.respawnAt)this.physics.resume();};
    const paused=()=>this.presentation?.setPaused(true), resumed=()=>this.presentation?.setPaused(false);
    this.game.events.on('hidden',paused); this.game.events.on('visible',resumed);
    this.events.on('pause',paused); this.events.on('resume',resumed);
    const activation=(event:MessageEvent)=>{if(event.source===parent&&event.data?.type==='aurago:game:active')active(event.data.active===true)};
    window.addEventListener('message',activation);
    this.events.once('shutdown',()=>{this.builder?.dispose();this.builder=null;this.sceneDebugGraphics=null;this.sceneDebugLabels=[];this.presentation?.dispose();this.presentation=null;this.game.events.off('hidden',paused);this.game.events.off('visible',resumed);window.removeEventListener('message',activation)});
    this.events.once('shutdown',()=>{this.isometric?.dispose();this.isometric=null;this.flow?.dispose();this.flow=null;});
    this.setup();
    if(this.stageCarry){this.state.score=this.stageCarry.score;this.state.lives=this.stageCarry.lives;this.stageCarry=null;}
    if(!this.checkpoint&&this.player)this.setCheckpoint(this.player.x,this.player.y);
    const root=document.getElementById('game-root')!;
    this.flow=createGameFlow({root,stage:this.levelIndex,stages:this.levels,restart:()=>this.restartGame(),advance:()=>this.nextLevel(),
      feedback:(name:string,point:any,normal:any,material:any)=>this.presentation.event(name,point||[this.player.x,this.player.y,0],normal,material),
      project:(point:any)=>{if(!point)return null;const c=this.cameras.main,r=this.game.canvas.getBoundingClientRect(),b=root.getBoundingClientRect();return {x:(r.left-b.left+(point[0]-c.worldView.x)*c.zoom*r.width/Number(this.game.config.width))/b.width,y:(r.top-b.top+(point[1]-c.worldView.y)*c.zoom*r.height/Number(this.game.config.height))/b.height};}});
    this.flow.update(0,this.state);
    if(this.levels.length>1)this.flow.message(this.levels[this.levelIndex].title,1.8);
    bindGameTest(this, this.state, this.player);
    this.time.addEvent({ delay: 1000, loop: true, callback: () => { if (!this.state.ended&&!this.respawnAt&&!this.manualPause&&this.previewActive) { this.state.ticks++; this.tick(); } } });
    this.paintHUD();
  }
  // Scene JSON is the data boundary. The source templates remain a compatible
  // fallback when an older project has no nodes in scene.json.
  setupScene() {
    if (!hasSceneNodes(scenePlan)) return false;
    if((scenePlan as any).projection?.kind==='isometric')return this.setupIsometricScene();
    this.sceneSolids=this.physics.add.staticGroup();
    this.builder = createScene2D(sceneForLevel(scenePlan,this.levels[this.levelIndex]?.id), {
      viewport: { width: this.game.config.width, height: this.game.config.height },
      state: this.state, mechanics: mechanicsPlan,
      createNode: (node:any) => {
        const size = Array.isArray(node.size) ? node.size : [32, 32];
        const moving = node.kind === 'player' || node.properties?.player === true || node.behaviors?.some((b:any)=>['movement','patrol','chase','keepdistance','projectile'].includes(b.type));
        const fixed = !!node.collider && !moving && node.properties?.dynamic !== true;
        const color = Number.isFinite(Number(node.properties?.color)) ? Number(node.properties.color) : 0x64748b;
        const object=this.body(node.at[0],node.at[1],size[0],size[1],color,fixed,node.role || '');
        const rotation=Number(node.rotation?.[2] || 0);
        object.setRotation(rotation).setScale(Number(node.scale?.[0]) || 1,Number(node.scale?.[1]) || 1);
        const c=node.collider;
        if(fixed)object.body.updateFromGameObject();
        if(c) {
          const w=c.extents[0]*2,h=c.extents[1]*2;
          if(c.shape==='circle'||c.shape==='sphere')object.body.setCircle(c.extents[0]);
          else object.body.setSize(w,h);
          object.body.setOffset(size[0]/2-w/2+(c.offset?.[0]||0),size[1]/2-h/2+(c.offset?.[1]||0));
        }
        if(!fixed && !moving)object.body.setAllowGravity(false);
        const visual=this.visuals.find((v:any)=>v.object===object);
        if(visual) { visual.art.setRotation(rotation); visual.art.setScale(visual.art.scaleX*object.scaleX,visual.art.scaleY*object.scaleY); }
        if(fixed)this.sceneSolids.add(object);
        return {object};
      },
      player: () => this.player ? { object: this.player } : null,
      overlap: (_node:any, object:any, player:any) => { const target=player?.object || player; return !!object?.active && !!target?.active && !!this.physics.overlap(target, object); },
      attachVisual: (node:any, placement:any, object:any) => {
        const first=placement.id===node.placements[0]?.id;
        if(!first&&plannedAssets[placement.asset_role])this.paintAsset(object,node.size[0],node.size[1],placement.asset_role);
        const visual=first?this.visuals.find((v:any)=>v.object===object):this.visuals[this.visuals.length-1];
        if(!visual||visual.object!==object)return;
        visual.role=placement.asset_role;
        visual.offset.x+=placement.position[0]-node.at[0];visual.offset.y+=placement.position[1]-node.at[1];
        visual.art.setRotation(Number(placement.rotation?.[2])||0);
        visual.art.setScale(visual.art.scaleX*(Number(placement.scale?.[0])||1),visual.art.scaleY*(Number(placement.scale?.[1])||1));
      },
      actualAssets: (_node:any, object:any)=>this.visuals.filter((v:any)=>v.object===object).map((v:any)=>({role:v.role,asset_id:v.spec.id})),
      actualRole: (node:any) => node.role || '',
      actualAssetID: (_node:any,object:any) => this.visuals.find((v:any)=>v.object===object)?.spec.id || '',
      onNode: (node:any, object:any) => { if (node.kind === 'player' || node.properties?.player === true) this.player = object; },
      event: (name:string, node:any) => this.feedback(name, node ? this.builder?.nodes.get(node.id)?.object : this.player),
      end: (won:boolean) => this.end(won),
      presentation: this.presentation,
      resourceCounts: ()=>({objects:this.children.list.length,bodies:this.physics.world.bodies.entries.length,static_bodies:this.physics.world.staticBodies.entries.length,planned_assets:Object.keys(plannedAssets).length,visuals:this.visuals.length}),
      removeNode: (_node:any, object:any) => { object.setActive(false).setVisible(false); for(const visual of this.visuals.filter((item:any)=>item.object===object))visual.art?.setVisible(false); if (object.body) object.body.enable = false; },
      resetNode: (_node:any, object:any, at:any) => { object.setPosition(at[0], at[1]); object.setActive(true); const visuals=this.visuals.filter((item:any)=>item.object===object); object.setVisible(visuals.length===0);for(const visual of visuals)visual.art?.setVisible(true); if (object.body) object.body.enable = true; },
      move: (_node:any, object:any, delta:any) => this.moveSceneObject(object,delta),
      destroyNode: (_node:any,object:any)=>object.destroy(),
      debugChanged: (enabled:boolean)=>{this.sceneDebugGraphics?.setVisible(enabled);for(const label of this.sceneDebugLabels)label.setVisible(enabled);},
      renderDebug: (builder:any) => this.renderSceneDebug(builder),
    });
    const world = this.builder.scene.world_bounds;
    const cameraWorld = this.builder.scene.camera_bounds || world;
    const width = Math.max(1, world.max[0] - world.min[0]);
    const height = Math.max(1, world.max[1] - world.min[1]);
    this.physics.world.setBounds(world.min[0], world.min[1], width, height);
    this.cameras.main.setBounds(cameraWorld.min[0], cameraWorld.min[1], Math.max(1, cameraWorld.max[0] - cameraWorld.min[0]), Math.max(1, cameraWorld.max[1] - cameraWorld.min[1]));
    if (!this.player) this.player = this.body(world.min[0], world.min[1], 24, 24, 0x5eead4, false, '');
    this.physics.add.collider(this.player,this.sceneSolids);
    const movement=this.builder.scene.nodes.find((n:any)=>n.kind==='player')?.behaviors.find((b:any)=>b.type==='movement');
    if(movement?.mode==='platformer')this.physics.world.gravity.y=Number(movement.gravity)||900;
    const cameraBlock=(mechanicsPlan as any)?.blocks?.find((b:any)=>b.kind==='camera'&&b.enabled!==false),camera=typeof cameraBlock?.params==='string'?JSON.parse(cameraBlock.params||'{}'):(cameraBlock?.params||{});
    this.cameras.main.setZoom(Phaser.Math.Clamp(Number(camera.zoom)||1,.1,4));
    if(camera.follow!==false&&(width>Number(this.game.config.width)||height>Number(this.game.config.height)))this.cameras.main.startFollow(this.player,true,.12,.12);
    for(const zone of (scenePlan as any)?.zones||[])if(zone.kind==='loss') {
      if(zone.bounds.min[1]>=world.max[1])this.physics.world.checkCollision.down=false;
      if(zone.bounds.max[1]<=world.min[1])this.physics.world.checkCollision.up=false;
      if(zone.bounds.min[0]>=world.max[0])this.physics.world.checkCollision.right=false;
      if(zone.bounds.max[0]<=world.min[0])this.physics.world.checkCollision.left=false;
    }
    return true;
  }
  moveSceneObject(object:any,delta:any) {
    const bounds=object.getBounds();
    for(const axis of [0,1]) {
      const next={x:bounds.x+(axis===0?delta[0]:0),y:bounds.y+(axis===1?delta[1]:0),width:bounds.width,height:bounds.height};
      const blocked=[...this.builder.nodes.values()].some((r:any)=>r.object!==object&&r.active&&r.node.collider&&r.object.body?.physicsType===Phaser.Physics.Arcade.STATIC_BODY&&Phaser.Geom.Intersects.RectangleToRectangle(next,r.object.getBounds()));
      if(!blocked) { if(axis===0)object.x+=delta[0];else object.y+=delta[1]; }
    }
    object.body?.updateFromGameObject();
  }
  auditGame() {
    if(this.builder)return this.builder.audit();
    const roles:any={};
    for(const object of this.gameObjects)for(const asset of object.__gmAssets||[]){
      const key=asset.role+'\u0000'+asset.asset_id;roles[key]??={...asset,count:0};roles[key].count++;
    }
    return {outcome:this.state.outcome===1?'won':this.state.outcome===2?'lost':'playing',
      roles:Object.values(roles).slice(0,64),events:{...this.gameEvents},event_trace:this.gameTrace.slice(-32),presentation:this.presentation?.audit(),
      nodes:this.gameObjects.slice(0,128).map((object:any)=>({id:object.__auragoBrickID||object.__gmID,active:object.active===true,x:Number(object.x)||0,y:Number(object.y)||0,z:0}))};
  }
  renderSceneDebug(builder:any) {
    if (!this.sceneDebugGraphics) this.sceneDebugGraphics = this.add.graphics().setDepth(3000);
    this.sceneDebugGraphics.setVisible(true).clear();
    for(const label of this.sceneDebugLabels)label.destroy();this.sceneDebugLabels=[];
    this.sceneDebugGraphics.lineStyle(1, 0x58a6ff, .75);
    const labelSize=Math.max(11,11*Number(this.game.config.width)/Math.max(1,this.game.canvas.getBoundingClientRect().width)/this.cameras.main.zoom);
    const world = builder.scene.world_bounds;
    this.sceneDebugGraphics.strokeRect(world.min[0], world.min[1], world.max[0] - world.min[0], world.max[1] - world.min[1]);
    const camera=builder.scene.camera_bounds;
    if(camera) {this.sceneDebugGraphics.lineStyle(1,0xa78bfa,.8);this.sceneDebugGraphics.strokeRect(camera.min[0],camera.min[1],camera.max[0]-camera.min[0],camera.max[1]-camera.min[1]);}
    for(const zone of (scenePlan as any)?.zones||[]) {
      this.sceneDebugGraphics.lineStyle(1,zone.kind==='loss'?0xf87171:0x5eead4,.85);
      this.sceneDebugGraphics.strokeRect(zone.bounds.min[0],zone.bounds.min[1],zone.bounds.max[0]-zone.bounds.min[0],zone.bounds.max[1]-zone.bounds.min[1]);
    }
    this.sceneDebugGraphics.lineStyle(1,0xffb454,.8);
    for(const route of Object.values(builder.scene.routes) as any[])if(route.waypoints?.length>1) {
      this.sceneDebugGraphics.beginPath();this.sceneDebugGraphics.moveTo(route.waypoints[0][0],route.waypoints[0][1]);
      for(const point of route.waypoints.slice(1))this.sceneDebugGraphics.lineTo(point[0],point[1]);this.sceneDebugGraphics.strokePath();
    }
    for (const record of [...builder.nodes.values()].slice(0, 128)) {
      if (!record.object?.active) continue;
      const bounds = record.object.getBounds?.();
      if (!bounds) continue;
      this.sceneDebugGraphics.strokeRect(bounds.x, bounds.y, bounds.width, bounds.height);
      this.sceneDebugLabels.push(this.add.text(bounds.x,bounds.y-labelSize-2,record.node.id,{fontSize:labelSize+'px',color:'#fbbf24',backgroundColor:'#081018'}).setDepth(3001));
    }
  }
  setupIsometricScene() {
    const raw:any=sceneForLevel(scenePlan,this.levels[this.levelIndex]?.id);this.physics.world.gravity.y=0;
    this.isometric=createIsometricWorld(raw,{
      origin:{x:Number(this.game.config.width)/2,y:80},
      create:(node:any)=>{
        const placement=raw.placements?.find((p:any)=>p.node_id===node.id),spec=plannedAssets[placement?.asset_role];
        const art=spec?createAsset(this,spec.meta,spec.id,0,0):this.add.rectangle(0,0,node.kind==='player'?16:8,node.kind==='player'?24:8,0x5eead4);
        if(spec&&placement?.rotation?.[2]){const angle=placement.rotation[2],p=projectIso([Math.cos(angle),Math.sin(angle),0]);setFacing(art,p.x,p.y);}
        art.__gmID=node.id;art.__gmRole=placement?.asset_role||'';art.__gmAssets=spec?[{role:placement.asset_role,pack_id:spec.meta.id,asset_id:spec.id}]:[];
        if(!spec&&node.properties?.walkable)art.setVisible(false);
        this.gameObjects.push(art);
        if(node.kind==='player'||node.properties?.player){this.player=art;this.isoPlayerID=node.id;this.physics.add.existing(art);art.body.setAllowGravity(false);art.body.moves=false;}
        return art;
      },
      position:(art:any,p:any,depth:number)=>{art.setPosition(p.x,p.y).setDepth(depth);art.body?.updateFromGameObject();},
      visible:(art:any,value:boolean)=>art.setVisible(value),destroy:(art:any)=>art.destroy(),
      moved:(record:any)=>{if(record.node.id===this.isoPlayerID)this.checkIsometricContacts(record);},
      renderDebug:()=>this.renderIsometricDebug(),
      debugChanged:(on:boolean)=>{if(!on){this.sceneDebugGraphics?.clear();for(const label of this.sceneDebugLabels)label.destroy();this.sceneDebugLabels=[];}}
    });
    if(!this.player)throw Error('Isometric scene needs a player node');
    this.hud.setDepth(1e8);this.cameras.main.startFollow(this.player,true,.12,.12);
    return true;
  }
  checkIsometricContacts(player:any) {
    for(const r of [...this.isometric.records.values()] as any[]) {
      if(this.isometric.records.get(player.node.id)!==player)break;
      if(r===player||!r.object.active||Math.abs(player.at[2]-r.at[2])>.2||Math.hypot(player.at[0]-r.at[0],player.at[1]-r.at[1])>.4)continue;
      if(r.node.properties?.pickup){this.state.score++;this.state.pickup_events++;this.feedback('pickup',r.object);this.isometric.remove(r.node.id);}
      if(r.node.properties?.goal)this.end(true);
      this.isometricContact(r.node,player.node);
    }
  }
  // Hooks for authored rules. The helpers never infer behavior from an asset ID.
  isometricContact(_node:any,_player:any) {}
  isometricAction() {}
  renderIsometricDebug(){
    if(!this.sceneDebugGraphics)this.sceneDebugGraphics=this.add.graphics().setDepth(1e7);
    const g=this.sceneDebugGraphics.clear();for(const label of this.sceneDebugLabels)label.destroy();this.sceneDebugLabels=[];
    for(const r of [...this.isometric.records.values()].slice(0,256) as any[]){
      const n=r.node,size=n.properties?.footprint||[1,1],x=r.at[0],y=r.at[1],z=r.at[2];
      const corners=[[x,y,z],[x+size[0],y,z],[x+size[0],y+size[1],z],[x,y+size[1],z]].map(p=>this.isometric.project(p));
      g.lineStyle(1,n.properties?.solid?0xf87171:n.properties?.goal?0xfbbf24:0x58a6ff,.8);g.beginPath();g.moveTo(corners[0].x,corners[0].y);for(const c of corners.slice(1))g.lineTo(c.x,c.y);g.closePath();g.strokePath();
      if(!n.properties?.walkable)this.sceneDebugLabels.push(this.add.text(corners[0].x,corners[0].y,n.id+' z='+z,{fontSize:'11px',color:'#fff',backgroundColor:'#081018'}).setDepth(1e7+1));
    }
  }
  changeIsometricLevel(id:string) {this.isometric.level(id);this.gameObjects=this.gameObjects.filter((o:any)=>o.active);bindGameTest(this,this.state,this.player);this.cameras.main.startFollow(this.player,true,.12,.12);}
  setup() { if (this.setupScene()) return; this.player = this.body(240, 270, 28, 28, 0x5eead4,false,"player"); }
  // Authored stages rebuild through the existing Scene lifecycle. Carry only campaign progress.
  configureLevels(levels:any[]) {this.levels=levelChoices(null,levels);if(this.levelIndex>=this.levels.length)this.levelIndex=0;}
  nextLevel() {if(this.state.outcome!==1||this.levelIndex+1>=this.levels.length)return;this.stageCarry={score:this.state.score,lives:this.state.lives};this.levelIndex++;this.scene.restart();}
  restartGame() {this.levelIndex=0;this.stageCarry=null;this.scene.restart();}
  configureWorld(width:number,height:number,follow=true) {
    if(!Number.isFinite(width)||!Number.isFinite(height)||width<=0||height<=0)throw Error('World dimensions must be positive');
    this.physics.world.setBounds(0,0,width,height);this.cameras.main.setBounds(0,0,width,height);
    if(follow&&this.player)this.cameras.main.startFollow(this.player,true,.12,.12);
  }
  setCheckpoint(x:number,y:number) {if(!Number.isFinite(x)||!Number.isFinite(y))throw Error('Invalid checkpoint');this.checkpoint={x,y};}
  damagePlayer() {
    if(this.state.ended||this.manualPause||!this.previewActive||this.elapsed<this.invulnerableUntil||this.respawnAt)return false;
    this.state.lives=Math.max(0,Number(this.state.lives||1)-1);this.feedback('death');this.invulnerableUntil=this.elapsed+1700;
    if(this.state.lives<=0){this.end(false);return true;}
    this.respawnAt=this.elapsed+650;this.player.body.stop();this.physics.pause();return true;
  }
  assetRoles(prefix: string) { return Object.keys(plannedAssets).filter(role=>role===prefix||role.startsWith(prefix+'_')); }
  body(x: number, y: number, w: number, h: number, color: number, fixed = false, role = '') {
    const object = this.add.rectangle(x, y, w, h, color);
    object.__gmRole=role;
    object.__gmID='body-'+this.gameObjects.length;if(this.gameObjects.length<4096)this.gameObjects.push(object);
    this.physics.add.existing(object, fixed);
    if (!fixed) object.body.setCollideWorldBounds(true);
    this.paintAsset(object,w,h,role);
    if(fixed&&['ground','platform','building','obstacle'].includes(role))this.presentation?.registerSurface(object,{kind:role==='ground'?'ground':'roof'});
    return object;
  }
  paintAsset(object:any,w:number,h:number,role:string) {
    const choices=this.assetRoles(role);
    const spec = plannedAssets[role] || plannedAssets[choices[this.visuals.length % choices.length]];
    if (spec) {
      const art = (spec.assembly ? createAssembly : createAsset)(this, spec.meta, spec.id, object.x, object.y);
      const offset = fitVisual(art, w, h);
      if (spec.direction && spec.direction !== "none") setFacing(art, spec.direction === "right" ? 1 : spec.direction === "left" ? -1 : 0, spec.direction === "down" ? 1 : spec.direction === "up" ? -1 : 0);
      const asset=spec.meta.assets.find((a:any)=>a.id===spec.id);
      if (!spec.assembly && asset?.action && spec.meta.animations.some((a:any)=>(a.entity?a.entity===asset.entity:a.asset_id===asset.id)&&a.action===asset.action&&a.direction===asset.direction)) playAction(art,asset.action);
      object.setVisible(false);
      this.visuals.push({object, art, offset, spec, asset, role});
      (object.__gmAssets??=[]).push({role,asset_id:spec.id});
      object.once('destroy', ()=>{art.destroy();this.visuals=this.visuals.filter((v:any)=>v.art!==art);});
    }
    return object;
  }
  tick() {}
  action() { this.state.actions++;if(this.isometric){this.isometricAction();return;}if(this.builder&&this.physics.world.gravity.y&&(this.player.body.blocked.down||this.player.body.touching.down)){this.player.body.setVelocityY(-420);this.feedback('jump');}this.builder?.action?.(); }
  step(_delta: number) {
    if(this.isometric){
      const v=this.inputKeys.vector(),length=Math.max(1,Math.hypot(v.x,v.y)),node=this.isometric.records.get(this.isoPlayerID)?.node;
      const speed=Number(node?.properties?.speed)||2.5,dx=(v.x+v.y)/2/length*speed*_delta,dy=(v.y-v.x)/2/length*speed*_delta;
      const moved=this.isometric.move(this.isoPlayerID,dx,dy),placement=(scenePlan as any).placements?.find((p:any)=>p.node_id===this.isoPlayerID),spec=plannedAssets[placement?.asset_role];
      if(spec){const vector=projectIso([dx,dy,0]);setFacing(this.player,vector.x,vector.y);const action=moved?'walk':'idle';if(spec.meta.animations.some((a:any)=>a.asset_id===spec.id&&a.action===action))playAction(this.player,action);}
      return;
    }
    const v=this.inputKeys.vector(),record=this.builder?.scene.nodes.find((n:any)=>n.kind==='player'||n.properties?.player);
    const control=record?.behaviors.find((b:any)=>b.type==='movement')||{};
    const speed=Number(control.speed)||240, length=Math.max(1,Math.hypot(v.x,v.y));
    if(control.mode==='platformer'||this.physics.world.gravity.y>0) {
      this.player.body.setVelocityX(v.x*speed);
      if(v.y<0&&(this.player.body.blocked.down||this.player.body.touching.down))this.player.body.setVelocityY(-(Number(control.jump_speed)||420));
    } else this.player.body.setVelocity(v.x/length*speed,v.y/length*speed);
  }
  feedback(name:string,object:any=this.player,material='flesh') {
    if(!this.builder&&(Object.hasOwn(this.gameEvents,name)||Object.keys(this.gameEvents).length<24)) {
      this.gameEvents[name]=Math.min(1e6,(this.gameEvents[name]||0)+1);
      this.gameTrace.push({type:name,id:object?.__auragoBrickID||object?.__gmID||'',at:this.elapsed});
      if(this.gameTrace.length>32)this.gameTrace.shift();
    }
    if(object){const point=[object.x,object.y,0];if(this.flow)this.flow.event(name,point,[0,-1,0],material);else this.presentation?.event(name,point,[0,-1,0],material);}
  }
  end(won=false) { if(this.state.ended)return; if(this.builder) { if(this.state.outcome===0) { (won ? this.builder.win : this.builder.lose).call(this.builder); return; } this.state.ended=1; this.physics.pause(); this.paintHUD(); return; } this.state.outcome=won?1:2;this.state[won?'win_events':'lose_events']++;this.feedback(won?'win':'lose'); this.state.ended = 1; this.physics.pause(); this.paintHUD(); }
  paintHUD() { if(this.builder){const s=this.state,width=this.game.canvas.getBoundingClientRect().width||Number(this.game.config.width);this.hud.setFontSize(Math.max(20,Math.round(14*Number(this.game.config.width)/width))).setWordWrapWidth(Number(this.game.config.width)-36);this.hud.setText([s.ended?(s.outcome===1?'COMPLETE':'GAME OVER'):this.manualPause?'PAUSED':[s.goal_remaining?`Goals ${s.goal_remaining}`:'',s.lives>0?`Lives ${s.lives}`:'',s.score?`Score ${s.score}`:''].filter(Boolean).join(' · '),s.dialogue||'',width<650?'Touch controls':'Arrows/WASD · Space: action · P: pause · R: restart'].filter(Boolean).join('\n'));return;}this.hud.setText(`Score ${this.state.score} · Time ${this.state.ticks}s · Arrows/WASD · Space: action · R: restart\n${this.state.ended ? (this.state.outcome===1?'COMPLETE':'GAME OVER')+' — R to restart' : 'Esc: end game'}`); }
  update(_time: number, delta: number) {
    if (this.inputKeys.pressed('R')) { this.restartGame(); return; }
    if(this.inputKeys.pressed('P')) {this.manualPause=!this.manualPause;this.time.paused=this.manualPause||!this.previewActive;if(this.manualPause)this.physics.pause();else if(!this.state.ended&&!this.respawnAt)this.physics.resume();this.presentation?.setPaused(this.manualPause);}
    if(this.manualPause||!this.previewActive){this.paintHUD();return;}
    this.flow?.update(Math.min(delta,50)/1000,this.state);
    if (this.state.ended) {this.presentation?.update(Math.min(delta,50)/1000);return;}
    if (this.inputKeys.pressed('ESC')) { this.end(); return; }
    this.elapsed += delta;
    this.player?.setAlpha(this.elapsed<this.invulnerableUntil?.55:1);
    if(this.respawnAt){
      if(this.elapsed<this.respawnAt){this.presentation?.update(Math.min(delta,50)/1000);return;}
      this.respawnAt=0;this.player.body.reset(this.checkpoint.x,this.checkpoint.y);this.physics.resume();this.feedback('respawn');
    }
    if (this.inputKeys.pressed('SPACE')) this.action();
    this.step(Math.min(delta, 50) / 1000); if (this.builder) this.builder.step(Math.min(delta, 50));
    this.visuals = this.visuals.filter(({object,art,offset,spec,asset})=>{
       if (!object.active) { art?.setVisible(false); return true; }
      const velocity=object.body?.velocity;
      if(asset && velocity){
        const moving=Math.abs(velocity.x)+Math.abs(velocity.y)>1;
        if(spec.meta.schema_version===2){
          if(moving)setFacing(art,velocity.x,velocity.y);
          const available=spec.meta.animations.filter((a:any)=>a.asset_id===asset.id);
          const preferred=moving?['walk','move','swim','sailing','run']:['idle','tread_water','sailing'];
          const action=preferred.find(name=>available.some((a:any)=>a.action===name));
          if(action)playAction(art,action);
        }else{
        const direction=Math.abs(velocity.x)>=Math.abs(velocity.y)?(velocity.x<0?'left':'right'):(velocity.y<0?'up':'down');
        const supported=asset.transform.mode==='rotate'||asset.transform.flip_x&&['left','right'].includes(direction)||spec.meta.assets.some((a:any)=>a.entity===asset.entity&&a.direction===direction);
        if(moving&&supported)setFacing(art,velocity.x,velocity.y);
        const action=spec.meta.animations.find((a:any)=>(a.entity?a.entity===asset.entity:a.asset_id===asset.id)&&a.direction===(moving&&supported?direction:asset.direction)&&(moving?['walk','move'].includes(a.action):a.action==='idle'));
        if(action)playAction(art,action.action);
        }
      }
      art.setPosition(object.x+offset.x, object.y+offset.y).setDepth(object.depth);
      if(object===this.player)art.setAlpha(this.elapsed<this.invulnerableUntil?.55:1);
      return true;
    });
    if(this.presentation){
      const grounded=!this.physics.world.gravity.y||this.player?.body?.blocked.down||this.player?.body?.touching.down;
      const velocity=this.player?.body?.velocity;
      if(grounded&&velocity&&Math.abs(velocity.x)+(!this.physics.world.gravity.y?Math.abs(velocity.y):0)>10&&this.elapsed-this.footstepAt>380){this.footstepAt=this.elapsed;this.feedback('step')}
      if(this.physics.world.gravity.y&&grounded&&!this.wasGrounded&&this.elapsed>200)this.feedback('land');
      this.wasGrounded=!!grounded;
      if(this.player)this.presentation.audio.listener([this.player.x,this.player.y,0]);
      this.presentation.update(Math.min(delta,50)/1000);
    }
    this.builder?.render();
    this.isometric?.render();
    this.paintHUD();
  }
}
export function start(Scene: any, gravity = 0) {
  new Phaser.Game({ type: Phaser.AUTO, parent: 'game-root', width: 960, height: 540,
    pixelArt: true, backgroundColor: '#081018',
    physics: { default: 'arcade', arcade: { gravity: { y: gravity }, debug: false } },
    scene: [Scene], scale: { mode: Phaser.Scale.FIT, autoCenter: Phaser.Scale.CENTER_BOTH } });
}
