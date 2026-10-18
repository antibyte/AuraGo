import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

const root = new URL('../ui/', import.meta.url);
const source = await readFile(new URL('js/desktop/apps/galaxa-constants.js', root), 'utf8');
const context = vm.createContext({ window: {} });
vm.runInContext(source, context, { filename: 'galaxa-constants.js' });
const GC = context.window.GalaxaCore;
const P = GC.ATTACK_PATTERNS;

let s = 1;
const random = () => (s = s * 16807 % 2147483647) / 2147483647;
const ebSpd = 260;
const center = bullets => bullets.reduce((sum, b) => sum + Math.atan2(b.vy, b.vx), 0) / bullets.length;

// fan_sweep: always downward, and the center swings between volleys.
const sweeper = { x: 270, y: 100 };
const v1 = P.fan_sweep(sweeper, 7, 0.42, 1.9, ebSpd), v2 = P.fan_sweep(sweeper, 7, 0.42, 1.9, ebSpd);
assert.equal(v1.length, 7);
assert.ok(v1.every(b => b.vy > 0) && v2.every(b => b.vy > 0), 'fan_sweep bullets must travel downward');
assert.notEqual(center(v1).toFixed(4), center(v2).toFixed(4), 'fan_sweep center must change between volleys');

// pincer: mirrored sideways launch, steering toward the target column.
const pin = P.pincer({ x: 270, y: 100 }, 0.5, ebSpd, 200);
assert.equal(pin.length, 2);
assert.ok(pin[0].vx * pin[1].vx < 0, 'pincer shots leave in opposite directions');
assert.ok(pin.every(b => b.steerX === 200 && b.steer > 0));

// lob: pops upward, gravity pulls it down.
const lob = P.lob({ x: 270, y: 100 }, 3, 0.5, ebSpd, random);
assert.equal(lob.length, 3);
assert.ok(lob.every(b => b.vy < 0 && b.ay > 0), 'lob shells rise first and carry gravity');
{
    const b = { ...lob[0] };
    const startY = b.y;
    let minY = b.y;
    const dt = 1 / 60;
    for (let i = 0; i < 180; i++) { b.vy += b.ay * dt; b.y += b.vy * dt; minY = Math.min(minY, b.y); }
    assert.ok(startY - minY >= 40, 'lob shells must rise at least 40px, rose ' + (startY - minY));
    assert.ok(b.y > startY, 'lob shells must end below their start');
}

// rain: a wide curtain.
const rain = P.rain({ x: 270, y: 100 }, 6, 0.42, 110, ebSpd, random);
assert.equal(rain.length, 6);
const xs = rain.map(b => b.x);
assert.ok(Math.max(...xs) - Math.min(...xs) >= 80, 'rain must span at least 80px');

// predictive: leads a moving target.
const still = P.predictive({ x: 270, y: 100 }, 1, 0.62, ebSpd, { x: 270, y: 600, vx: 0, vy: 0 });
const moving = P.predictive({ x: 270, y: 100 }, 1, 0.62, ebSpd, { x: 270, y: 600, vx: 200, vy: 0 });
assert.equal(still.length, 1);
assert.ok(moving[0].vx > still[0].vx + 1, 'predictive must aim further right at a target moving right');

for (const k of ['loop', 'strafe', 'hover']) assert.ok(GC.DIVE_PATHS[k] && GC.DIVE_PATHS[k].stageMin > 0, 'DIVE_PATHS.' + k);
assert.ok(GC.ENEMY_HEADING.default && GC.ENEMY_HEADING.hunter.face && GC.ENEMY_HEADING.spinner.spin, 'ENEMY_HEADING profiles');

console.log('galaxa attack pattern checks passed');
