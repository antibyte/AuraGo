import { GameScene, start } from './common';
// Three stages; each adds one element. Meadow: a patrol and spikes. Ridge: a hovering
// bat and spring boots. Peak: a gauntlet of everything. Layouts are data; setup()
// builds the stage chosen by this.levelIndex. Landing on an enemy defeats it.
const STAGES = [
  { width: 1920, platforms: [[350,420],[580,340],[760,260],[1150,420],[1380,340],[1600,260]],
    coins: [[280,470],[580,300],[760,220],[1380,300],[1600,220]], spikes: [870], patrols: [[1050,1350]], bats: [], boots: null },
  { width: 2880, platforms: [[350,420],[750,400],[1150,360],[1500,300],[1750,400],[2150,420],[2380,340],[2600,260]],
    coins: [[280,470],[750,360],[1500,260],[1750,360],[2380,300],[2600,220]], spikes: [870,1950], patrols: [[1000,1400],[2000,2300]], bats: [[1500,230]], boots: [1150,330] },
  { width: 3360, platforms: [[350,420],[600,340],[850,260],[1150,420],[1400,330],[1650,250],[1950,420],[2250,340],[2550,260],[2850,340],[3100,260]],
    coins: [[280,470],[600,300],[850,220],[1400,290],[1650,210],[2250,300],[2550,220],[3100,220]], spikes: [1000,1800,2700], patrols: [[1100,1500],[2000,2400],[2600,2950]], bats: [[1300,280],[2100,250],[2800,230]], boots: [850,230] },
];
class Platformer extends GameScene {
  enemies: any[] = []; bootsUntil = 0;
  setup() { if (this.setupScene()) return;
    this.configureLevels([{id:'meadow',title:'Meadow trail'},{id:'ridge',title:'The high ridge'},{id:'peak',title:'Storm peak'}]);
    const stage = STAGES[this.levelIndex];
    this.state.lives=3; this.enemies=[]; this.bootsUntil=0;
    this.player = this.body(160, 450, 28, 40, 0x5eead4,false,"player");
    this.configureWorld(stage.width,720);
    const ground = this.body(stage.width/2, 520, stage.width, 40, 0x334155, true,"ground");
    this.physics.add.collider(this.player, ground);
    for (const [x,y] of stage.platforms) this.physics.add.collider(this.player, this.body(x,y,150,20,0x475569,true,"platform"));
    for (const [x,y] of stage.coins) {
      const coin = this.body(x,y,20,20,0xfacc15,true,"item");
      this.physics.add.overlap(this.player,coin,()=>{this.feedback('pickup',coin);coin.destroy();this.state.hits++;this.state.score++;});
    }
    for (const x of stage.spikes) this.physics.add.overlap(this.player,this.body(x,485,40,30,0xfb7185,true,"hazard"),()=>this.damagePlayer());
    for (const [from,to] of stage.patrols) this.enemy(from,482,{from,to,speed:70+this.levelIndex*25});
    for (const [x,y] of stage.bats) this.enemy(x,y,{phase:x/300});
    if (stage.boots) {
      const boots = this.body(stage.boots[0],stage.boots[1],22,22,0x38bdf8,true,"powerup");
      this.physics.add.overlap(this.player,boots,()=>{this.feedback('pickup',boots);boots.destroy();this.bootsUntil=this.elapsed+10000;this.flow.message('Spring boots!',1.6);});
    }
    const checkpoint=this.body(stage.width/2,460,24,60,0x38bdf8,true,'');
    this.physics.add.overlap(this.player,checkpoint,()=>{if(checkpoint.reached)return;checkpoint.reached=true;this.setCheckpoint(checkpoint.x,450);this.flow.message('Checkpoint');this.feedback('pickup',checkpoint);});
    const goal = this.body(stage.width-220,210,30,60,0xa78bfa,true,"goal");
    this.physics.add.overlap(this.player,goal,()=>{this.state.score+=100;this.end(true);});
  }
  // Patrols walk between two x positions; bats (no range) hover on a sine wave.
  enemy(x: number, y: number, motion: any) {
    const foe = this.body(x,y,30,28,motion.speed?0xf97316:0xc084fc,false,"enemy");
    foe.body.setAllowGravity(false).setImmovable(true); foe.motion=motion;
    if (motion.speed) foe.body.setVelocityX(motion.speed);
    this.physics.add.overlap(this.player,foe,()=>{
      if (!foe.active) return;
      if (this.player.body.velocity.y>0 && this.player.y<foe.y-12) {this.feedback('hit',foe);foe.destroy();this.player.body.setVelocityY(-340);this.state.hits++;this.state.score+=5;}
      else this.damagePlayer();
    });
    this.enemies.push(foe);
  }
  action() { if (this.builder) { super.action(); return; } if (this.player.body.blocked.down || this.player.body.touching.down) {this.player.body.setVelocityY(this.elapsed<this.bootsUntil?-650:-500);this.state.actions++;this.feedback('jump');} }
  step() { if (this.builder) { super.step(0);return; }
    this.player.body.setVelocityX(this.inputKeys.vector().x * 240);
    const t = this.elapsed/1000;
    for (const foe of this.enemies) {
      if (!foe.active) continue;
      const m = foe.motion;
      if (m.speed) { if (foe.x<m.from&&foe.body.velocity.x<0 || foe.x>m.to&&foe.body.velocity.x>0) foe.body.setVelocityX(-foe.body.velocity.x); }
      else foe.body.setVelocity(Math.cos(t*1.1+m.phase)*60, Math.cos(t*2.4+m.phase)*120);
    }
  }
}
start(Platformer,1000);
