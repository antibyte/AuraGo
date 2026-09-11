import { createInputs, bindGameTest, preloadPack, registerAnimations, createAsset, createAssembly, fitVisual, playAction, setFacing } from '../vendor/aurago-game-1.js';
declare const Phaser: any;
// PLAN_ASSET_IMPORTS
const plannedAssets: any = {};

// Keep this lifecycle when adapting a template. State belongs to each scene run.
export class GameScene extends Phaser.Scene {
  player: any; inputKeys: any; hud: any;
  state: any; elapsed = 0;
  visuals: any[] = [];
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
    this.elapsed = 0;
    this.visuals = [];
    for (const meta of new Set(Object.values(plannedAssets).map((a:any)=>a.meta))) registerAnimations(this, meta);
    this.state = { score: 0, actions: 0, hits: 0, spawns: 0, turns: 0, ended: 0, ticks: 0 };
    this.inputKeys = createInputs(this);
    this.hud = this.add.text(18, 16, '', { fontFamily: 'monospace', fontSize: '20px', color: '#ffffff' }).setDepth(1000).setScrollFactor(0);
    this.setup();
    bindGameTest(this, this.state, this.player);
    this.time.addEvent({ delay: 1000, loop: true, callback: () => { if (!this.state.ended) { this.state.ticks++; this.tick(); } } });
    this.paintHUD();
  }
  setup() { this.player = this.body(240, 270, 28, 28, 0x5eead4,false,"player"); }
  assetRoles(prefix: string) { return Object.keys(plannedAssets).filter(role=>role===prefix||role.startsWith(prefix+'_')); }
  body(x: number, y: number, w: number, h: number, color: number, fixed = false, role = '') {
    const object = this.add.rectangle(x, y, w, h, color);
    this.physics.add.existing(object, fixed);
    if (!fixed) object.body.setCollideWorldBounds(true);
    this.paintAsset(object,w,h,role);
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
      this.visuals.push({object, art, offset, spec, asset});
      object.once('destroy', ()=>art.destroy());
    }
    return object;
  }
  tick() {}
  action() { this.state.actions++; }
  step(_delta: number) { const v = this.inputKeys.vector(); this.player.body.setVelocity(v.x * 240, v.y * 240); }
  end() { this.state.ended = 1; this.physics.pause(); this.paintHUD(); }
  paintHUD() { this.hud.setText(`Score ${this.state.score} · Time ${this.state.ticks}s · Arrows/WASD · Space: action · R: restart\n${this.state.ended ? 'GAME OVER — R to restart' : 'Esc: end game'}`); }
  update(_time: number, delta: number) {
    if (this.inputKeys.pressed('R')) { this.scene.restart(); return; }
    if (this.state.ended) return;
    if (this.inputKeys.pressed('ESC')) { this.end(); return; }
    this.elapsed += delta;
    if (this.inputKeys.pressed('SPACE')) this.action();
    this.step(Math.min(delta, 50) / 1000);
    this.visuals = this.visuals.filter(({object,art,offset,spec,asset})=>{
      if (!object.active) return false;
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
    this.paintHUD();
  }
}
export function start(Scene: any, gravity = 0) {
  new Phaser.Game({ type: Phaser.AUTO, parent: 'game-root', width: 960, height: 540,
    pixelArt: true, backgroundColor: '#081018',
    physics: { default: 'arcade', arcade: { gravity: { y: gravity }, debug: false } },
    scene: [Scene], scale: { mode: Phaser.Scale.FIT, autoCenter: Phaser.Scale.CENTER_BOTH } });
}
