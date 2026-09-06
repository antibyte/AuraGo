import { createInputs, bindGameTest } from '../vendor/aurago-game-1.js';
declare const Phaser: any;

// Keep this lifecycle when adapting a template. State belongs to each scene run.
export class GameScene extends Phaser.Scene {
  player: any; inputKeys: any; hud: any;
  state: any; elapsed = 0;
  constructor() { super('main'); }
  create() {
    this.physics.resume();
    this.elapsed = 0;
    this.state = { score: 0, actions: 0, hits: 0, spawns: 0, turns: 0, ended: 0, ticks: 0 };
    this.inputKeys = createInputs(this);
    this.hud = this.add.text(18, 16, '', { fontFamily: 'monospace', fontSize: '20px', color: '#ffffff' }).setDepth(1000).setScrollFactor(0);
    this.setup();
    bindGameTest(this, this.state, this.player);
    this.time.addEvent({ delay: 1000, loop: true, callback: () => { if (!this.state.ended) { this.state.ticks++; this.tick(); } } });
    this.paintHUD();
  }
  setup() { this.player = this.body(240, 270, 28, 28, 0x5eead4); }
  body(x: number, y: number, w: number, h: number, color: number, fixed = false) {
    const object = this.add.rectangle(x, y, w, h, color);
    this.physics.add.existing(object, fixed);
    if (!fixed) object.body.setCollideWorldBounds(true);
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
    this.paintHUD();
  }
}
export function start(Scene: any, gravity = 0) {
  new Phaser.Game({ type: Phaser.AUTO, parent: 'game-root', width: 960, height: 540,
    pixelArt: true, backgroundColor: '#081018',
    physics: { default: 'arcade', arcade: { gravity: { y: gravity }, debug: false } },
    scene: [Scene], scale: { mode: Phaser.Scale.FIT, autoCenter: Phaser.Scale.CENTER_BOTH } });
}
