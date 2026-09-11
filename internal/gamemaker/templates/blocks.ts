import { GameScene, start } from './common';
class Blocks extends GameScene {
  ball: any; blocks: any; lives=3;
  setup() {
    this.lives=3;this.player=this.body(480,490,120,18,0x5eead4,false,'player');this.player.body.setImmovable(true);
    this.ball=this.body(480,465,16,16,0xfacc15,false,'ball');this.ball.body.setBounce(1);
    this.blocks=this.physics.add.staticGroup();
    const roles=this.assetRoles('block');
    for(let row=0;row<3;row++)for(let col=0;col<8;col++){const b=this.body(130+col*100,110+row*34,90,24,0xa78bfa,true,roles[(row*8+col)%roles.length]||'');this.blocks.add(b);}
    this.physics.add.collider(this.ball,this.player,()=>this.ball.body.setVelocityY(-340));
    this.physics.add.collider(this.ball,this.blocks,(_:any,block:any)=>{this.feedback('hit',block,'stone');block.destroy();this.state.score++;this.state.hits++;if(!this.blocks.countActive())this.end(true);});
  }
  action() { if(this.ball.body.velocity.length()===0){this.ball.body.setVelocity(70,-340);this.state.actions++;} }
  step() {
    this.player.body.setVelocityX(this.inputKeys.vector().x*380);
    if(this.ball.body.velocity.length()===0)this.ball.setPosition(this.player.x,465);
    if(this.ball.y>515){this.lives--;if(!this.lives)this.end();else{this.ball.body.setVelocity(0);this.ball.setPosition(this.player.x,465);}}
  }
}
start(Blocks);
