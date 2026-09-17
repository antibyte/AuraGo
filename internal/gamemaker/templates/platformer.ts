import { GameScene, start } from './common';
class Platformer extends GameScene {
  setup() { if (this.setupScene()) return;
    this.configureLevels([{id:'meadow',title:'Meadow trail'},{id:'ridge',title:'The high ridge'}]);
    this.state.lives=3;
    this.player = this.body(160, 450, 28, 40, 0x5eead4,false,"player");
    const width=this.levelIndex===0?1920:2880;this.configureWorld(width,720);
    const ground = this.body(width/2, 520, width, 40, 0x334155, true,"ground");
    this.physics.add.collider(this.player, ground);
    const platforms=this.levelIndex===0?[[350,420],[580,340],[760,260],[1150,420],[1380,340],[1600,260]]:[[350,420],[750,400],[1150,360],[1500,300],[1750,400],[2150,420],[2380,340],[2600,260]];
    for (const [x,y] of platforms) this.physics.add.collider(this.player, this.body(x,y,150,20,0x475569,true,"platform"));
    const coin = this.body(280,470,20,20,0xfacc15,true,"item");
    this.physics.add.overlap(this.player,coin,()=>{this.feedback('pickup',coin);coin.destroy();this.state.hits++;this.state.score++;});
    const danger = this.body(870,485,40,30,0xfb7185,true,"enemy");
    this.physics.add.overlap(this.player,danger,()=>this.damagePlayer());
    const checkpoint=this.body(width/2,460,24,60,0x38bdf8,true,'');
    this.physics.add.overlap(this.player,checkpoint,()=>{if(checkpoint.reached)return;checkpoint.reached=true;this.setCheckpoint(checkpoint.x,450);this.flow.message('Checkpoint');this.feedback('pickup',checkpoint);});
    const goal = this.body(width-220,210,30,60,0xa78bfa,true,"goal");
    this.physics.add.overlap(this.player,goal,()=>{this.state.score+=100;this.end(true);});
  }
  action() { if (this.builder) { super.action(); return; } if (this.player.body.blocked.down || this.player.body.touching.down) {this.player.body.setVelocityY(-500);this.state.actions++;this.feedback('jump');} }
  step() { if (this.builder) { super.step(0);return; } this.player.body.setVelocityX(this.inputKeys.vector().x * 240); }
}
start(Platformer,1000);
