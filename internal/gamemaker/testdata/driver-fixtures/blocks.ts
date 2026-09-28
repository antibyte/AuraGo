import { GameScene, start } from './common';

class Blocks extends GameScene {
  ball: any; blocks: any; lives=3; brickRecords: any[]=[]; boardWidth=960; boardHeight=540; launchY=465;

  setup() {
    if (this.setupScene()) return;
    this.boardWidth=Number(this.game?.config?.width)||Number(this.scale?.width)||960;
    this.boardHeight=Number(this.game?.config?.height)||Number(this.scale?.height)||540;
    const columns=this.boardWidth>=700?8:this.boardWidth>=500?6:4;
    const rows=this.boardHeight>=400?3:2;
    const margin=Math.max(20,Math.min(80,this.boardWidth*.08));
    const gap=Math.max(4,Math.min(12,this.boardWidth*.012));
    const brickWidth=Math.max(24,(this.boardWidth-margin*2-gap*(columns-1))/columns);
    const brickHeight=Math.max(14,Math.min(28,this.boardHeight*.045));
    const top=Math.max(32,this.boardHeight*.16);
    const rowGap=Math.max(4,Math.min(12,this.boardHeight*.018));
    const playerWidth=Math.min(260,Math.max(220,this.boardWidth*.22));
    const playerHeight=Math.max(12,Math.min(22,this.boardHeight*.035));
    const playerY=this.boardHeight-Math.max(30,this.boardHeight*.08);
    const ballSize=Math.max(10,Math.min(20,this.boardWidth*.018));
    this.launchY=playerY-playerHeight/2-ballSize/2-4;
    this.lives=3; this.brickRecords=[]; this.state.lives=this.lives;
    this.player=this.body(this.boardWidth/2,playerY,playerWidth,playerHeight,0x5eead4,false,'player');
    this.player.body.setImmovable(true);
    this.ball=this.body(this.boardWidth/2,this.launchY,ballSize,ballSize,0xfacc15,false,'ball');
    this.ball.body.setBounce(1);
    // The bottom edge is the loss zone. Keeping side and top bounds makes the
    // ball playable while allowing a genuine miss to consume a life.
    this.physics.world.checkCollision.down=false;
    this.blocks=this.physics.add.staticGroup();
    const roles=this.assetRoles('block');
    for(let row=0;row<rows;row++)for(let col=0;col<columns;col++){
      const index=row*columns+col, role=roles.length?roles[index%roles.length]:'';
      const x=margin+brickWidth/2+col*(brickWidth+gap), y=top+brickHeight/2+row*(brickHeight+rowGap);
      const block=this.body(x,y,brickWidth,brickHeight,0xa78bfa,true,role);
      block.__auragoBrickID='brick-'+row+'-'+col; block.__auragoRole=role; this.brickRecords.push(block); this.blocks.add(block);
    }
    this.state.goal_remaining=this.brickRecords.length;
    this.physics.add.collider(this.ball,this.player,()=>this.ball.body.setVelocityY(-Math.abs(this.ball.body.velocity.y||this.launchVelocityY())));
    this.physics.add.collider(this.ball,this.blocks,(_:any,block:any)=>{
      if(!block?.active)return;
      this.feedback('hit',block,'stone'); block.destroy(); this.state.score++; this.state.hits++; this.state.hit_events++;
      this.state.goal_remaining=this.remainingBricks(); if(this.state.goal_remaining===0)this.end(true);
    });
  }

  launchVelocityY() { return Math.max(220,this.boardHeight*.68); }
  remainingBricks() { return this.brickRecords.reduce((count,block)=>count+(block?.active!==false?1:0),0); }
  launchBall(force=false) {
    if(force||this.ball.body.velocity.length()===0){this.ball.body.setVelocity(Math.max(90,this.boardWidth*.24),-this.launchVelocityY());this.state.actions++;}
  }

  auditGame() {
    const legacy=super.auditGame?.();
    if(this.builder)return legacy;
    const nodes=this.brickRecords.slice(0,128).map((block:any,index:number)=>({id:block.__auragoBrickID||('brick-'+index),active:block.active!==false,x:Number(block.x)||0,y:Number(block.y)||0,z:0}));
    const roleCounts:any={}, brickRoleKeys:any={};
    for(const block of this.brickRecords){
      if(!block?.__auragoRole)continue;
      const visual=this.visuals?.find((item:any)=>item.object===block), assetID=visual?.spec?.id||'';
      if(!assetID)continue;
      const key=block.__auragoRole+'\u0000'+assetID; brickRoleKeys[key]=true;
      if(block.active===false)continue;
      if(!roleCounts[key])roleCounts[key]={role:block.__auragoRole,asset_id:assetID,count:0};
      roleCounts[key].count++;
    }
    const audit:any=legacy&&typeof legacy==='object'?{...legacy}:{};
    const baseNodes=Array.isArray(audit.nodes)?audit.nodes:[];
    audit.nodes=[...baseNodes.filter((node:any)=>!String(node?.id||'').startsWith('brick-')), ...nodes];
    const roles=Array.isArray(audit.roles)?audit.roles.map((role:any)=>({...role})).filter((role:any)=>!brickRoleKeys[role?.role+'\u0000'+role?.asset_id]):[];
    for(const value of Object.values(roleCounts) as any[]){
      const existing=roles.find((role:any)=>role?.role===value.role&&role?.asset_id===value.asset_id);
      if(existing)existing.count=value.count;else roles.push(value);
    }
    audit.roles=roles.slice(0,128);
    audit.outcome=this.state?.outcome===1?'won':this.state?.outcome===2?'lost':'playing';
    audit.lives=Number(this.lives)||0;
    audit.goal_remaining=this.remainingBricks(); audit.remaining_bricks=this.remainingBricks();
    audit.events={...(audit.events&&typeof audit.events==='object'?audit.events:{}),hit:Number(this.state?.hit_events||0),win:Number(this.state?.win_events||0),lose:Number(this.state?.lose_events||0)};
    return audit;
  }

  action() {
    if (this.builder) { super.action(); return; }
    this.launchBall();
  }

  step(delta: number) {
    if (this.builder) { super.step(delta); return; }
    this.player.body.setVelocityX(this.inputKeys.vector().x*Math.max(240,this.boardWidth*.4));
    if(this.ball.body.velocity.length()===0){
      if(this.inputKeys.keys.SPACE?.isDown)this.launchBall();
      if(this.ball.body.velocity.length()===0)this.ball.setPosition(this.player.x,this.launchY);
    }
    if(this.ball.y>this.boardHeight+this.ball.height){
      this.lives--; this.state.lives=this.lives;
      if(this.lives<=0){this.ball.body.setVelocity(0,0);this.end();}
      else {
        this.ball.body.setVelocity(0,0);this.ball.setPosition(this.player.x,this.launchY);
        this.time.delayedCall(180,()=>{if(!this.state.ended&&this.ball?.active)this.launchBall(true);});
      }
    }
  }
}

start(Blocks);
