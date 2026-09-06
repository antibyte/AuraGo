---
name: aurago-phaser4-gameplay
description: Build responsive 2D gameplay with the pinned Phaser 4 runtime.
license: MIT
compatibility: Phaser 4.2.1
metadata:
  managed_by: aurago
  source: phaserjs/phaser skills
  commit: 41be1e462bc600064e498cba370bfa8c5c055a22
allowed-tools: game_maker_project, game_maker_file, game_maker_asset, game_maker_validate
---

# Phaser 4 Gameplay

Use the provided global `Phaser` runtime. Keep a scene-first architecture:
bootstrap configuration, one focused gameplay scene, and separate helpers only
when complexity justifies them.

- Create game objects and physics relationships in `create`; update continuous
  input and simulation in `update`.
- Use Arcade Physics for simple movement, overlap, collision, bounds, and
  velocities. Avoid Matter unless the design genuinely needs it.
- Pick a fixed logical resolution and `Phaser.Scale.FIT` with auto-centering
  so the canvas adapts to the preview without stretching gameplay.
- Set `parent: 'game-root'` in the Phaser game configuration when using the
  scaffold. A canvas appended after the full-height root is clipped offscreen.
- Make keyboard controls explicit and include touch or pointer controls when
  the game concept is likely to be used on mobile.
- Reference project assets with relative paths (`assets/...`); load them in
  `preload` and confirm the exact paths returned by `game_maker_asset`.
- Unlock audio only after a player gesture; keep music opt-in with a mute.
- Pool frequently spawned objects and never allocate objects or arrays in the
  per-frame `update` path.
- Communicate objectives, score, health, cooldowns, and game-over state through
  readable in-game UI with a fixed HUD (`setScrollFactor(0)`).
- Release scene resources on shutdown so restarts stay leak-free.
- Preserve `window.__AURAGO_GAME_DIAGNOSTICS__` and emit scene readiness.
- Use `Phaser.Utils.Array.GetRandom(values)` or `Phaser.Math.RND.pick(values)`
  for a random array element. `Phaser.Math.pick` does not exist. Keep spawning,
  shooting, collision and restart paths executable during testing; errors in
  a delayed callback are runtime failures too. Scale per-frame movement by
  `delta / 1000` and reset score/game-over flags when restarting a scene.

Validate after scene wiring, after gameplay rules, and after final polish.

## Built-in sprite sheets

Import a pack first. Import its JSON in `src/main.ts` so esbuild bundles metadata
into the offline game. The import response includes a pack-specific
`phaser_example`. Never load the sheet with `load.image`, or its AuraGo metadata
with `load.atlas`.
Complete `src/main.ts` example after importing human-characters-animated version 2.
Keep the installed `src/common.ts` template lifecycle:

```typescript
import { GameScene, start } from './common';
import { preloadPack, registerAnimations, createAsset, setFacing, playAction } from '../vendor/aurago-game-1.js';
import meta from '../assets/builtin/human-characters-animated/2/sheet.json';
class RangerGame extends GameScene {
  art: any; attackUntil=0;
  preload() { preloadPack(this,meta,'assets/builtin/human-characters-animated/2/sheet.png'); }
  setup() {
    super.setup();this.attackUntil=0;this.player.setVisible(false);
    registerAnimations(this,meta);
    this.art=createAsset(this,meta,'ranger_idle',this.player.x,this.player.y).setScale(2);
    const coin=this.body(360,270,20,20,0xfacc15,true);
    this.physics.add.overlap(this.player,coin,()=>{coin.destroy();this.state.score++;this.state.hits++;});
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

Keep pixel art sharp with `pixelArt: true`. Preserve ordered frame lists with
`sortFrames: false`, including holds. Reference:
https://docs.phaser.io/phaser/concepts/animations

## Direction, physics, and restart

Use setFacing(object,dx,dy): zero vector retains the last facing; four-view art
selects the dominant axis (ties horizontal). Side-view art uses only horizontal
facing. Rotatable art uses atan2(dy,dx) minus metadata.forward_radians. Phaser
`rotation`/setRotation are radians; `angle` is degrees. Do not mix them. Flip is
around the texture center; the helper mirrors the normalized anchor too.
Buildings, cards, signs and terrain cannot be flipped/rotated unless metadata
explicitly permits it. Do not rotate a side-view person into a top-down person.
playAction ignores a repeated request for the currently running animation;
call it when the action changes and let one-shot attacks/deaths finish.

For an assembly, createAssembly returns one container with original part offsets.
Scale the container uniformly. Move a separate hidden rectangular/circular
physics proxy and copy its position to the art. Never attach physics to the
parts: container transforms do not correctly transform child Arcade bodies.
Choose a conservative explicit collider; visual rotation is not collider rotation.
Reset counters/timers/inputs in the provided scene lifecycle. A restarted scene
must resume physics if the previous game ended with physics.pause().
Phaser API references:
https://docs.phaser.io/phaser/concepts/gameobjects/components
https://docs.phaser.io/phaser/concepts/gameobjects/container
