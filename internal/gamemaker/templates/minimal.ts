import { GameScene, start } from './common';
// Replace these rules with the requested special-case game; retain the lifecycle
// and bindGameTest contract. No template is a substitute for the accepted plan.
class CustomGame extends GameScene {
  target: any;
  setup() { if (this.setupScene()) return;
    super.setup();this.target=this.body(360,270,24,24,0xfacc15,true,"item");
    this.physics.add.overlap(this.player,this.target,()=>{this.feedback('pickup',this.target);this.target.destroy();this.state.hits++;this.state.score++;});
  }
  action() { if (this.builder) { super.action(); return; } super.action();this.player.setFillStyle(this.state.actions%2?0xa78bfa:0x5eead4); }
}
start(CustomGame);
