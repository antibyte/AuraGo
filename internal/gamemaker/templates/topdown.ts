import { GameScene, start } from './common';
class Adventure extends GameScene {
  chest: any;
  setup() { if (this.setupScene()) return;
    this.configureLevels([{id:'grove',title:'The old grove'},{id:'ruins',title:'Ruins beyond the grove'}]);
    this.state.lives=3;
    this.player = this.body(240,270,28,32,0x5eead4,false,"player");
    this.configureWorld(this.levelIndex===0?1920:2400,this.levelIndex===0?1080:1440);
    this.physics.add.collider(this.player,this.body(500,270,70,140,0x475569,true,"obstacle"));
    const item = this.body(360,270,18,18,0xfacc15,true,"item");
    this.physics.add.overlap(this.player,item,()=>{this.feedback('pickup',item);item.destroy();this.state.hits++;this.state.score++;});
    this.chest = this.body(this.levelIndex===0?850:1600,this.levelIndex===0?310:1000,32,24,0xd97706,true,"goal");
    if(this.levelIndex===1)for(const [x,y] of [[700,500],[1100,800],[1500,500]])this.physics.add.collider(this.player,this.body(x,y,180,100,0x475569,true,'obstacle'));
  }
  action() { if (this.builder) { super.action(); return; }
    if (Math.hypot(this.player.x-this.chest.x,this.player.y-this.chest.y)<100 && !this.chest.opened) {
      this.feedback('interact',this.chest);this.chest.opened=true;this.chest.setFillStyle(0xfacc15);this.state.actions++;this.state.score+=5;
      this.end(true);
    }
  }
  step(delta: number) { if (this.builder) { super.step(delta);return; } super.step(delta);this.player.setDepth(this.player.y);this.chest.setDepth(this.chest.y); }
}
start(Adventure);
