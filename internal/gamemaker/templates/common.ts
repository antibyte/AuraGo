import { createInputs, bindGameTest, preloadPack, registerAnimations, createAsset, createAssembly, fitVisual } from '../vendor/aurago-game-1.js';
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
  setup() { this.player = this.body(240, 270, 28, 28, 0x5eead4); }
  assetRoles(prefix: string) { return Object.keys(plannedAssets).filter(role=>role===prefix||role.startsWith(prefix+'_')); }
  body(x: number, y: number, w: number, h: number, color: number, fixed = false, role = '') {
    const object = this.add.rectangle(x, y, w, h, color);
    this.physics.add.existing(object, fixed);
    if (!fixed) object.body.setCollideWorldBounds(true);
    const spec = Object.prototype.hasOwnProperty.call(plannedAssets, role) ? plannedAssets[role] : null;
    if (spec) {
      const art = (spec.assembly ? createAssembly : createAsset)(this, spec.meta, spec.id, x, y);
      const offset = fitVisual(art, w, h);
      object.setSize(offset.width, offset.height);
      object.body.setSize(offset.width, offset.height);
      if (fixed) object.body.updateFromGameObject();
      object.setVisible(false);
      this.visuals.push({object, art, offset});
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
    this.visuals = this.visuals.filter(({object,art,offset})=>{
      if (!object.active) return false;
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
