import {GameScene,start} from './common';
class Journey extends GameScene {
  setup() {
    this.configureLevels([{id:'trail',title:'The trail'},{id:'fort',title:'The fort'}]);
    this.state.lives=2;
    this.player=this.body(100,270,28,32,0x5eead4);
    this.configureWorld(1920,540);this.setCheckpoint(100,270);
    const hazard=this.body(300,270,32,32,0xf87171,true,'enemy');
    this.physics.add.overlap(this.player,hazard,()=>{
      if(this.damagePlayer()) {this.feedback('hit',hazard);if(this.levelIndex===0)hazard.destroy();}
    });
    const goal=this.body(this.levelIndex===0?1400:1800,270,40,60,0xa78bfa,true,'goal');
    this.physics.add.overlap(this.player,goal,()=>{if(!this.state.ended){this.state.score+=10;this.end(true);}});
    (window as any).fixture=this;
  }
}
start(Journey);
