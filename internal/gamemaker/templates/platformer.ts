import { GameScene, start } from './common';
class Platformer extends GameScene {
  setup() {
    this.player = this.body(160, 450, 28, 40, 0x5eead4,false,"player");
    const ground = this.body(480, 520, 960, 40, 0x334155, true,"ground");
    this.physics.add.collider(this.player, ground);
    for (const [x,y] of [[350,420],[580,340],[760,260]]) this.physics.add.collider(this.player, this.body(x,y,150,20,0x475569,true,"platform"));
    const coin = this.body(280,470,20,20,0xfacc15,true,"item");
    this.physics.add.overlap(this.player,coin,()=>{coin.destroy();this.state.hits++;this.state.score++;});
    const danger = this.body(870,485,40,30,0xfb7185,true,"enemy");
    this.physics.add.overlap(this.player,danger,()=>this.end());
    const goal = this.body(820,210,30,60,0xa78bfa,true,"goal");
    this.physics.add.overlap(this.player,goal,()=>{this.state.score+=100;this.end();});
  }
  action() { if (this.player.body.blocked.down || this.player.body.touching.down) {this.player.body.setVelocityY(-500);this.state.actions++;} }
  step() { this.player.body.setVelocityX(this.inputKeys.vector().x * 240); }
}
start(Platformer,1000);
