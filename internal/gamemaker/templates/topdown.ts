import { GameScene, start } from './common';
// Three areas; each adds one element. Grove: gems and a patrolling guard. Ruins: walls,
// a wisp that chases on sight and a heart. Crypt: the chest is locked until the guarded
// key is found. Layouts are data; setup() builds the area chosen by this.levelIndex.
const AREAS = [
  { w: 1920, h: 1080, walls: [[500,270,70,140]], gems: [[360,270],[700,640],[1250,300],[1550,820]],
    guards: [[900,760,1400,760]], wisps: [], chest: [850,310], key: null, heart: null },
  { w: 2400, h: 1440, walls: [[500,270,70,140],[760,560,200,90],[1180,860,200,90],[1560,520,90,320],[1960,960,260,90]],
    gems: [[360,270],[980,300],[1320,700],[1760,1180],[2140,420]], guards: [[800,1100,1450,1100],[1800,300,1800,820]],
    wisps: [[1500,1150]], chest: [2120,1200], key: null, heart: [1000,860] },
  { w: 2800, h: 1600, walls: [[500,270,70,140],[900,420,90,420],[1300,900,420,90],[1750,420,90,520],[2200,1000,90,500],[1500,1350,500,90]],
    gems: [[360,270],[1100,250],[1250,1150],[2000,300],[2500,1400]], guards: [[1050,700,1600,700],[2000,800,2000,1400],[600,1300,1200,1300]],
    wisps: [[1450,420],[2450,650]], chest: [2550,300], key: [1500,1180], heart: [700,900] },
];
class Adventure extends GameScene {
  chest: any; enemies: any[] = []; hasKey = false;
  setup() { if (this.setupScene()) return;
    this.configureLevels([{id:'grove',title:'The old grove'},{id:'ruins',title:'Ruins beyond the grove'},{id:'crypt',title:'The sealed crypt'}]);
    const area = AREAS[this.levelIndex];
    this.state.lives=3; this.enemies=[]; this.hasKey=!area.key;
    this.player = this.body(240,270,28,32,0x5eead4,false,"player");
    this.configureWorld(area.w,area.h);
    for (const [x,y,w,h] of area.walls) this.physics.add.collider(this.player,this.body(x,y,w,h,0x475569,true,"obstacle"));
    for (const [x,y] of area.gems) {
      const item = this.body(x,y,18,18,0xfacc15,true,"item");
      this.physics.add.overlap(this.player,item,()=>{this.feedback('pickup',item);item.destroy();this.state.hits++;this.state.score++;});
    }
    for (const [x1,y1,x2,y2] of area.guards) this.enemy(x1,y1,{x1,y1,x2,y2});
    for (const [x,y] of area.wisps) this.enemy(x,y,{homeX:x,homeY:y});
    if (area.key) {
      const key = this.body(area.key[0],area.key[1],20,14,0x38bdf8,true,"key");
      this.physics.add.overlap(this.player,key,()=>{this.feedback('pickup',key);key.destroy();this.hasKey=true;this.flow.message('Key found!',1.6);});
    }
    if (area.heart) {
      const heart = this.body(area.heart[0],area.heart[1],18,18,0xf472b6,true,"powerup");
      this.physics.add.overlap(this.player,heart,()=>{this.feedback('pickup',heart);heart.destroy();this.state.lives=Math.min(5,this.state.lives+1);});
    }
    this.chest = this.body(area.chest[0],area.chest[1],32,24,0xd97706,true,"goal");
  }
  // Guards walk between two points; wisps (no route) chase the player within sight.
  enemy(x: number, y: number, motion: any) {
    const foe = this.body(x,y,26,26,motion.x2===undefined?0xc084fc:0xf97316,false,"enemy");
    foe.body.setImmovable(true); foe.motion=motion; foe.leg=1;
    this.physics.add.overlap(this.player,foe,()=>{if(foe.active)this.damagePlayer();});
    this.enemies.push(foe);
  }
  action() { if (this.builder) { super.action(); return; }
    if (Math.hypot(this.player.x-this.chest.x,this.player.y-this.chest.y)<100 && !this.chest.opened) {
      if (!this.hasKey) {this.flow.message('Locked. Find the key.',1.6);return;}
      this.feedback('interact',this.chest);this.chest.opened=true;this.chest.setFillStyle(0xfacc15);this.state.actions++;this.state.score+=5;
      this.end(true);
    }
  }
  step(delta: number) { if (this.builder) { super.step(delta);return; } super.step(delta);this.player.setDepth(this.player.y);this.chest.setDepth(this.chest.y);
    const speed = 80+this.levelIndex*25;
    for (const foe of this.enemies) {
      if (!foe.active) continue;
      const m = foe.motion; let tx: number, ty: number;
      if (m.x2 !== undefined) { tx = foe.leg ? m.x2 : m.x1; ty = foe.leg ? m.y2 : m.y1; if (Math.hypot(tx-foe.x,ty-foe.y)<6) foe.leg = 1-foe.leg; }
      else { const near = Math.hypot(this.player.x-foe.x,this.player.y-foe.y)<280; tx = near ? this.player.x : m.homeX; ty = near ? this.player.y : m.homeY; }
      const dx = tx-foe.x, dy = ty-foe.y, length = Math.hypot(dx,dy);
      foe.body.setVelocity(length>4?dx/length*speed:0, length>4?dy/length*speed:0); foe.setDepth(foe.y);
    }
  }
}
start(Adventure);
