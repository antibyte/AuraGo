import { GameScene, start } from './common';
class Shooter extends GameScene {
  shots: any; enemies: any; lastShot = 0;
  setup() {
    this.player = this.body(480, 450, 30, 36, 0x5eead4, false, "player");
    this.shots = this.physics.add.group(); this.enemies = this.physics.add.group(); this.lastShot = -1000;
    this.spawn();
    this.physics.add.overlap(this.shots, this.enemies, (shot: any, enemy: any) => {
      this.feedback('hit',enemy,'metal');shot.destroy(); enemy.destroy(); this.state.hits++; this.state.score += 10;
    });
    this.physics.add.overlap(this.player, this.enemies, () => this.end());
  }
  spawn() {
    const enemy = this.body(this.player.x, 100, 32, 32, 0xfb7185, false, "enemy");
    this.enemies.add(enemy); enemy.body.setCollideWorldBounds(false).setVelocityY(70); this.state.spawns++;
  }
  tick() { this.spawn(); }
  action() {
    if (this.elapsed - this.lastShot < 180) return;
    this.lastShot = this.elapsed;
    const shot = this.body(this.player.x, this.player.y - 24, 6, 18, 0xfacc15, false, "projectile");
    this.shots.add(shot); shot.body.setCollideWorldBounds(false).setVelocityY(-500); this.state.actions++;this.feedback('shot',shot);
  }
  step(delta: number) {
    super.step(delta); if (this.inputKeys.keys.SPACE.isDown) this.action();
    for (const object of [...this.shots.getChildren(), ...this.enemies.getChildren()]) {
      if (object.y < -30 || object.y > 570) object.destroy();
    }
  }
}
start(Shooter);
