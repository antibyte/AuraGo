import { createInputs, bindGameTest, preloadPack, registerAnimations, createAsset, createAssembly, fitVisual, playAction, setFacing } from '../vendor/aurago-game-1.js';
import {createPresentation, createPhaserAdapter} from '../vendor/aurago-effects-2d-1.js';
import {createScene2D, hasSceneNodes} from '../vendor/scene-builder.js';
import presentationPlan from './presentation.json';
import scenePlan from './scene.json';
import mechanicsPlan from './mechanics.json';
declare const Phaser: any;
// PLAN_ASSET_IMPORTS
const plannedAssets: any = {};

// Keep this lifecycle when adapting a template. State belongs to each scene run.
export class GameScene extends Phaser.Scene {
  player: any; inputKeys: any; hud: any;
  state: any; elapsed = 0; presentation: any; builder: any; sceneDebugGraphics: any; footstepAt = 0; wasGrounded = false;
  visuals: any[] = []; gameObjects:any[]=[]; gameEvents:any={}; gameTrace:any[]=[]; sceneSolids:any; sceneDebugLabels: any[] = []; manualPause = false; previewActive = true;
  constructor() { super('main'); }
  preload() {
    for (const meta of new Set(Object.values(plannedAssets).map((a:any)=>a.meta))) {
      preloadPack(this, meta, `assets/builtin/${meta.id}/${meta.version}/sheet.png`);
    }
  }
  create() {
    if (this.update !== GameScene.prototype.update) {
      throw Error('GameScene.update must be inherited from common.ts: it handles ESC end, R restart, input, elapsed time and planned sprite following. Remove the update override; put continuous gameplay in step(deltaSeconds), primary input in action(), and HUD changes in paintHUD().');
    }
    this.physics.resume();
    this.elapsed = 0; this.manualPause=false;this.time.paused=false;
    this.visuals = [];this.gameObjects=[];this.gameEvents={};this.gameTrace=[];
    for (const meta of new Set(Object.values(plannedAssets).map((a:any)=>a.meta))) registerAnimations(this, meta);
    this.state = { score: 0, actions: 0, hits: 0, spawns: 0, turns: 0, ended: 0, ticks: 0, lives: 0, goal_remaining: 0, outcome: 0, hit_events: 0, pickup_events: 0, win_events: 0, lose_events: 0 };
    this.inputKeys = createInputs(this);
    this.hud = this.add.text(18, 16, '', { fontFamily: 'monospace', fontSize: '20px', color: '#ffffff' }).setDepth(1000).setScrollFactor(0);
    this.footstepAt = 0; this.wasGrounded = false;
    this.presentation = presentationPlan ? createPresentation({config:presentationPlan,root:document.getElementById('game-root'),adapter:createPhaserAdapter({scene:this,view:this.physics.world.gravity.y?'side':'top'}),report:(message:any)=>console.warn(message)}) : null;
    const active=(value:boolean)=>{this.previewActive=value;this.time.paused=!value||this.manualPause;this.presentation?.setActive(value);if(!value)this.physics.pause();else if(!this.manualPause&&!this.state.ended)this.physics.resume();};
    const paused=()=>this.presentation?.setPaused(true), resumed=()=>this.presentation?.setPaused(false);
    this.game.events.on('hidden',paused); this.game.events.on('visible',resumed);
    this.events.on('pause',paused); this.events.on('resume',resumed);
    const activation=(event:MessageEvent)=>{if(event.source===parent&&event.data?.type==='aurago:game:active')active(event.data.active===true)};
    window.addEventListener('message',activation);
    this.events.once('shutdown',()=>{this.builder?.dispose();this.builder=null;this.sceneDebugGraphics=null;this.sceneDebugLabels=[];this.presentation?.dispose();this.presentation=null;this.game.events.off('hidden',paused);this.game.events.off('visible',resumed);window.removeEventListener('message',activation)});
    this.setup();
    bindGameTest(this, this.state, this.player);
    this.time.addEvent({ delay: 1000, loop: true, callback: () => { if (!this.state.ended&&!this.manualPause&&this.previewActive) { this.state.ticks++; this.tick(); } } });
    this.paintHUD();
  }
  // Scene JSON is the data boundary. The source templates remain a compatible
  // fallback when an older project has no nodes in scene.json.
  setupScene() {
    if (!hasSceneNodes(scenePlan)) return false;
    this.sceneSolids=this.physics.add.staticGroup();
    this.builder = createScene2D(scenePlan, {
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
  setup() { if (this.setupScene()) return; this.player = this.body(240, 270, 28, 28, 0x5eead4,false,"player"); }
  assetRoles(prefix: string) { return Object.keys(plannedAssets).filter(role=>role===prefix||role.startsWith(prefix+'_')); }
  body(x: number, y: number, w: number, h: number, color: number, fixed = false, role = '') {
    const object = this.add.rectangle(x, y, w, h, color);
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
  action() { this.state.actions++;if(this.builder&&this.physics.world.gravity.y&&(this.player.body.blocked.down||this.player.body.touching.down)){this.player.body.setVelocityY(-420);this.feedback('jump');}this.builder?.action?.(); }
  step(_delta: number) {
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
    if(object)this.presentation?.event(name,[object.x,object.y,0],[0,-1,0],material);
  }
  end(won=false) { if(this.state.ended)return; if(this.builder) { if(this.state.outcome===0) { (won ? this.builder.win : this.builder.lose).call(this.builder); return; } this.state.ended=1; this.physics.pause(); this.paintHUD(); return; } this.state.outcome=won?1:2;this.state[won?'win_events':'lose_events']++;this.feedback(won?'win':'lose'); this.state.ended = 1; this.physics.pause(); this.paintHUD(); }
  paintHUD() { if(this.builder){const s=this.state,width=this.game.canvas.getBoundingClientRect().width||Number(this.game.config.width);this.hud.setFontSize(Math.max(20,Math.round(14*Number(this.game.config.width)/width))).setWordWrapWidth(Number(this.game.config.width)-36);this.hud.setText([s.ended?(s.outcome===1?'COMPLETE':'GAME OVER'):this.manualPause?'PAUSED':[s.goal_remaining?`Goals ${s.goal_remaining}`:'',s.lives>0?`Lives ${s.lives}`:'',s.score?`Score ${s.score}`:''].filter(Boolean).join(' · '),s.dialogue||'',width<650?'Touch controls':'Arrows/WASD · Space: action · P: pause · R: restart'].filter(Boolean).join('\n'));return;}this.hud.setText(`Score ${this.state.score} · Time ${this.state.ticks}s · Arrows/WASD · Space: action · R: restart\n${this.state.ended ? (this.state.outcome===1?'COMPLETE':'GAME OVER')+' — R to restart' : 'Esc: end game'}`); }
  update(_time: number, delta: number) {
    if (this.inputKeys.pressed('R')) { this.scene.restart(); return; }
    if(this.inputKeys.pressed('P')) {this.manualPause=!this.manualPause;this.time.paused=this.manualPause||!this.previewActive;if(this.manualPause)this.physics.pause();else if(!this.state.ended)this.physics.resume();this.presentation?.setPaused(this.manualPause);}
    if(this.manualPause||!this.previewActive){this.paintHUD();return;}
    if (this.state.ended) {this.presentation?.update(Math.min(delta,50)/1000);return;}
    if (this.inputKeys.pressed('ESC')) { this.end(); return; }
    this.elapsed += delta;
    if (this.inputKeys.pressed('SPACE')) this.action();
    this.step(Math.min(delta, 50) / 1000); if (this.builder) this.builder.step(Math.min(delta, 50));
    this.visuals = this.visuals.filter(({object,art,offset,spec,asset})=>{
       if (!object.active) { art?.setVisible(false); return true; }
      const velocity=object.body?.velocity;
      if(asset && velocity){
        const moving=Math.abs(velocity.x)+Math.abs(velocity.y)>1;
        const direction=Math.abs(velocity.x)>=Math.abs(velocity.y)?(velocity.x<0?'left':'right'):(velocity.y<0?'up':'down');
        const supported=asset.transform.mode==='rotate'||asset.transform.flip_x&&['left','right'].includes(direction)||spec.meta.assets.some((a:any)=>a.entity===asset.entity&&a.direction===direction);
        if(moving&&supported)setFacing(art,velocity.x,velocity.y);
        const action=spec.meta.animations.find((a:any)=>(a.entity?a.entity===asset.entity:a.asset_id===asset.id)&&a.direction===(moving&&supported?direction:asset.direction)&&(moving?['walk','move'].includes(a.action):a.action==='idle'));
        if(action)playAction(art,action.action);
      }
      art.setPosition(object.x+offset.x, object.y+offset.y).setDepth(object.depth);
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
    this.paintHUD();
  }
}
export function start(Scene: any, gravity = 0) {
  new Phaser.Game({ type: Phaser.AUTO, parent: 'game-root', width: 960, height: 540,
    pixelArt: true, backgroundColor: '#081018',
    physics: { default: 'arcade', arcade: { gravity: { y: gravity }, debug: false } },
    scene: [Scene], scale: { mode: Phaser.Scale.FIT, autoCenter: Phaser.Scale.CENTER_BOTH } });
}
