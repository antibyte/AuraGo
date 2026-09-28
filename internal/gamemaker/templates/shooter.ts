import { GameScene, start } from './common';
// Three stages; each adds one element. First wave: divers fall straight. Crossfire:
// weavers zigzag. Mothership: gunships fire back and a boss must be worn down.
// Every fifth kill drops a rapid-fire pickup. Stage data drives spawn(); no timers.
const STAGES = [
  { goal: 6, kinds: ['diver'], speed: 70, boss: false },
  { goal: 9, kinds: ['diver','weaver'], speed: 85, boss: false },
  { goal: 8, kinds: ['weaver','gunship'], speed: 95, boss: true },
];
class Shooter extends GameScene {
  shots: any; enemies: any; enemyShots: any; drops: any; boss: any = null; lastShot = 0; rapidUntil = 0; kills = 0;
  setup() { if (this.setupScene()) return;
    this.configureLevels([{id:'approach',title:'First wave'},{id:'crossfire',title:'Crossfire'},{id:'mothership',title:'Mothership'}]);this.state.lives=3;
    this.player = this.body(480, 450, 30, 36, 0x5eead4, false, "player");
    this.shots = this.physics.add.group(); this.enemies = this.physics.add.group(); this.enemyShots = this.physics.add.group(); this.drops = this.physics.add.group();
    this.lastShot = -1000; this.rapidUntil = 0; this.kills = 0; this.boss = null;
    this.spawn();
    this.physics.add.overlap(this.shots, this.enemies, (shot: any, enemy: any) => {
      this.feedback('hit',enemy,'metal');shot.destroy(); enemy.destroy(); this.state.hits++; this.state.score += 10; this.kills++;
      if (this.kills%5===0) this.drop(enemy.x, enemy.y);
      if (!STAGES[this.levelIndex].boss && this.state.hits>=STAGES[this.levelIndex].goal) this.end(true);
      else if (STAGES[this.levelIndex].boss && !this.boss && this.kills>=STAGES[this.levelIndex].goal) this.summonBoss();
    });
    this.physics.add.overlap(this.player, this.enemies, () => this.damagePlayer());
    this.physics.add.overlap(this.player, this.enemyShots, (_: any, bolt: any) => { bolt.destroy(); this.damagePlayer(); });
    this.physics.add.overlap(this.player, this.drops, (_: any, drop: any) => { this.feedback('pickup',drop); drop.destroy(); this.rapidUntil = this.elapsed+6000; this.flow.message('Rapid fire!',1.4); });
  }
  spawn() {
    if(this.enemies.countActive(true)>=10||this.boss)return;
    const stage = STAGES[this.levelIndex], kind = stage.kinds[this.state.spawns%stage.kinds.length];
    const x=this.levelIndex===0?this.player.x:160+(this.state.spawns%3)*320;
    const enemy = this.body(x, 100, 32, 32, kind==='gunship'?0xf97316:kind==='weaver'?0xc084fc:0xfb7185, false, "enemy");
    this.enemies.add(enemy); enemy.body.setCollideWorldBounds(false).setVelocityY(stage.speed); this.state.spawns++;
    enemy.kind = kind; enemy.baseX = x; enemy.fireAt = this.elapsed+1500;
  }
  summonBoss() {
    this.boss = this.body(480, 90, 140, 44, 0xef4444, false, "boss"); this.boss.hp = 12; this.boss.body.setImmovable(true).setVelocityX(120);
    this.flow.message('The mothership!',1.8);
    this.physics.add.overlap(this.boss, this.shots, (_: any, shot: any) => {
      shot.destroy(); this.feedback('hit',this.boss,'metal'); this.boss.hp--; this.state.hits++; this.state.score += 25;
      this.boss.setAlpha(.4+.6*this.boss.hp/12); if (this.boss.hp<=0) { this.boss.destroy(); this.end(true); }
    });
  }
  drop(x: number, y: number) { const drop = this.body(x, y, 18, 18, 0x38bdf8, false, "powerup"); this.drops.add(drop); drop.body.setCollideWorldBounds(false).setVelocityY(90); }
  bolt(x: number, y: number) {
    const bolt = this.body(x, y, 8, 14, 0xfde047, false, "enemy_projectile"); this.enemyShots.add(bolt);
    const dx = this.player.x-x, dy = this.player.y-y, length = Math.max(1,Math.hypot(dx,dy));
    bolt.body.setCollideWorldBounds(false).setVelocity(dx/length*220, dy/length*220);
  }
  tick() { if (this.builder) return; this.spawn(); }
  action() { if (this.builder) { super.action(); return; }
    if (this.elapsed - this.lastShot < (this.elapsed<this.rapidUntil?90:180)) return;
    this.lastShot = this.elapsed;
    const shot = this.body(this.player.x, this.player.y - 24, 6, 18, 0xfacc15, false, "projectile");
    this.shots.add(shot); shot.body.setCollideWorldBounds(false).setVelocityY(-500); this.state.actions++;this.feedback('shot',shot);
  }
  step(delta: number) { if (this.builder) { super.step(delta);return; }
    super.step(delta); if (this.inputKeys.down('SPACE')) this.action();
    for (const enemy of this.enemies.getChildren() as any[]) {
      if (enemy.kind==='weaver') enemy.body.setVelocityX(Math.cos((enemy.y-100)/45)*150);
      if (enemy.kind==='gunship' && this.elapsed>=enemy.fireAt) { enemy.fireAt = this.elapsed+1800; this.bolt(enemy.x, enemy.y+18); }
    }
    if (this.boss?.active) {
      if (this.boss.x<120&&this.boss.body.velocity.x<0 || this.boss.x>840&&this.boss.body.velocity.x>0) this.boss.body.setVelocityX(-this.boss.body.velocity.x);
      if (this.elapsed>=(this.boss.fireAt||0)) { this.boss.fireAt = this.elapsed+900; this.bolt(this.boss.x-40, this.boss.y+24); this.bolt(this.boss.x+40, this.boss.y+24); }
    }
    for (const object of [...this.shots.getChildren(), ...this.enemies.getChildren(), ...this.enemyShots.getChildren(), ...this.drops.getChildren()]) {
      if (object.y < -30 || object.y > 570 || object.x < -30 || object.x > 990) object.destroy();
    }
  }
}
start(Shooter);
