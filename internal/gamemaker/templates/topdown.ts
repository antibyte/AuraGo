import { GameScene, start } from './common';
class Adventure extends GameScene {
  chest: any;
  setup() {
    this.player = this.body(240,270,28,32,0x5eead4,false,"player");
    this.physics.add.collider(this.player,this.body(500,270,70,140,0x475569,true,"obstacle"));
    const item = this.body(360,270,18,18,0xfacc15,true,"item");
    this.physics.add.overlap(this.player,item,()=>{this.feedback('pickup',item);item.destroy();this.state.hits++;this.state.score++;});
    this.chest = this.body(260,310,32,24,0xd97706,true,"goal");
  }
  action() {
    if (Math.hypot(this.player.x-this.chest.x,this.player.y-this.chest.y)<100 && !this.chest.opened) {
      this.feedback('interact',this.chest);this.chest.opened=true;this.chest.setFillStyle(0xfacc15);this.state.actions++;this.state.score+=5;
    }
  }
  step(delta: number) { super.step(delta);this.player.setDepth(this.player.y);this.chest.setDepth(this.chest.y); }
}
start(Adventure);
