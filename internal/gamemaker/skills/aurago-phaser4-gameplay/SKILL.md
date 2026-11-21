---
name: aurago-phaser4-gameplay
description: Build responsive 2D gameplay with the pinned Phaser 4 runtime.
license: MIT
compatibility: Phaser 4.2.1
metadata:
  managed_by: aurago
  source: phaserjs/phaser skills
  commit: 41be1e462bc600064e498cba370bfa8c5c055a22
allowed-tools: game_maker_project, game_maker_file, game_maker_asset, game_maker_preview
---

# Optional Phaser guidance

Use the project's local Phaser runtime when it suits the request. `game_maker_project`
`add_library` (`library: "phaser"`) installs Phaser and AuraGo helpers into `vendor/`
without overwriting your code; `reference` (`framework: "phaser"`) returns the
`GameScene` contract, and `use_template` installs a starter (`canvas` for framework-free
2D or `minimal`, `shooter`, `platformer`, `topdown`, `blocks`, `board`).

## 2D game feel & polish

- **Hitstop & screen shake:** Pause simulation briefly (35–60 ms) or trigger `this.cameras.main.shake(90, 0.008)` on heavy hits and explosions.
- **Squash & stretch + hit flash:** Tween `scaleX`/`scaleY` on jumps/pickups (`ease: 'Back.easeOut'`) and tint sprites briefly (`0xffffff` or `0xfb7185` for 100 ms) on impact.
- **Forgiving platformer controls:** Track `coyoteTimer` (~100 ms after walking off a ledge) and `jumpBufferTimer` (~120 ms before touching down), plus variable jump height when releasing jump early.
- **Audio & particles:** Use `this.feedback(...)`, `vendor/zzfx.js` (`playSfx`), or `generate kind="sfx"` so jumps, pickups, shots and hits sound crisp.
- **Restart discipline:** Phaser scene restart reuses the scene instance, so reset custom cooldowns, spawn clocks, score and per-run flags in `setup()`/`create()`.

## Built-in sprite sheets & lifecycle

Import pack JSON in `src/main.ts`, call `preloadPack` in `preload()`, register animations in `setup()`, and keep physics objects/groups distinct from wrapper records:

```typescript
import { GameScene, start } from './common';
import { preloadPack, registerAnimations, createAsset, setFacing, playAction } from '../vendor/aurago-game-1.js';
import meta from '../assets/builtin/human-characters-animated/2/sheet.json';
class RangerGame extends GameScene {
  art: any; attackUntil=0;
  preload() { super.preload(); preloadPack(this,meta,'assets/builtin/human-characters-animated/2/sheet.png'); }
  setup() {
    this.player=this.body(240,270,28,28,0x5eead4,false,'');
    this.attackUntil=0;this.player.setVisible(false);
    registerAnimations(this,meta);
    this.art=createAsset(this,meta,'ranger_idle',this.player.x,this.player.y).setScale(2);
    const coin=this.body(360,270,20,20,0xfacc15,true);
    this.physics.add.overlap(this.player,coin,()=>{this.feedback('pickup',coin);coin.destroy();this.state.score++;this.state.hits++;});
  }
  action() {this.state.actions++;this.attackUntil=this.elapsed+650;playAction(this.art,'attack');}
  step(delta: number) {
    super.step(delta);
    this.art.setPosition(this.player.x,this.player.y);
    const x=this.inputKeys.vector().x;
    setFacing(this.art,x,0);
    if(this.elapsed>=this.attackUntil)playAction(this.art,x?'walk':'idle');
  }
}
start(RangerGame);
```

## Arcade physics pitfalls

- Use dynamic Arcade bodies (`fixed = false`) for any moving object (`player`, moving paddles, projectiles); `StaticBody` has no `setVelocity` or `setImmovable`.
- Arcade bodies have no `setPosition`: use `obj.setPosition(x, y)` + `obj.body.reset(x, y)`, or `obj.body.updateFromGameObject()` after moving static rectangles.
- Clear Phaser Groups with `group.clear(true, true)` (`removeAll()` is for Containers).

## Spawned collision objects

Pass physics GameObjects or persistent Groups to `physics.add.collider/overlap`, never wrapper `{body, art}` records or one-time array snapshots:

```typescript
import { GameScene, start } from './common';
class Multiball extends GameScene {
  balls: any; blocks: any;
  setup() {
    this.player=this.body(480,490,120,18,0x5eead4,false,'player');
    this.player.body.setImmovable(true);
    this.balls=this.add.group();this.blocks=this.add.group();
    const roles=this.assetRoles('block');
    for(let col=0;col<8;col++)this.blocks.add(this.body(130+col*100,200,90,24,0xa78bfa,true,roles[col%roles.length]||''));
    this.physics.add.collider(this.balls,this.player,(ball:any)=>ball.body.setVelocityY(-340));
    this.physics.add.collider(this.balls,this.blocks,(_:any,block:any)=>{
      block.destroy();this.state.hits++;this.state.score++;
      if(!this.blocks.countActive())this.end();
    });
  }
  spawnBall(x: number) {
    const ball=this.body(x,465,16,16,0xfacc15,false,'ball');
    ball.body.setBounce(1).setVelocity(0,-340);
    this.balls.add(ball);this.state.spawns++;
  }
  action() {
    if(this.balls.countActive()>=4)return;
    this.spawnBall(this.player.x);this.spawnBall(this.player.x+40);
    this.state.actions++;
  }
  step() { this.player.body.setVelocityX(this.inputKeys.vector().x*380); }
}
start(Multiball);
```

File writes only save. Use `build` for compiler diagnostics and `preview` (`open`/`reload`,
`input` with `capture`, `probe`, `logs`) for visual verification. `finish` publishes
after the current build succeeds.
