import test from 'node:test';
import assert from 'node:assert/strict';
import { createRng, createSchool } from '../ui/js/chat/ocean-life-boids.js';

const bounds = { x: [-5, 5], y: [-3, 3], z: [-13, -9] };
function school(seed) {
    return createSchool({ count: 24, rng: createRng(seed), bounds, speed: [0.4, 0.8], size: [0.8, 1.2] });
}
const quiet = { laneHalfWidth: () => 0, laneStrength: 0, repulsors: [], speedMul: 1, cohesionMul: 1 };

test('same seed gives the same school', () => {
    const a = school(7), b = school(7);
    for (let i = 0; i < 200; i++) { a.step(1 / 60, quiet); b.step(1 / 60, quiet); }
    assert.deepEqual(Array.from(a.pos), Array.from(b.pos));
});

test('fish stay inside the bounds', () => {
    const s = school(3);
    for (let i = 0; i < 1800; i++) s.step(1 / 60, quiet);
    for (let i = 0; i < s.count; i++) {
        assert.ok(s.pos[i * 3] > bounds.x[0] - 1 && s.pos[i * 3] < bounds.x[1] + 1, 'x ' + s.pos[i * 3]);
        assert.ok(s.pos[i * 3 + 1] > bounds.y[0] - 1 && s.pos[i * 3 + 1] < bounds.y[1] + 1, 'y');
        assert.ok(s.pos[i * 3 + 2] > bounds.z[0] - 1 && s.pos[i * 3 + 2] < bounds.z[1] + 1, 'z');
    }
});

test('the reading lane stays mostly empty', () => {
    const s = school(5);
    const env = { ...quiet, laneHalfWidth: () => 1.5, laneStrength: 1 };
    for (let i = 0; i < 1800; i++) s.step(1 / 60, env);
    let inside = 0;
    for (let i = 0; i < s.count; i++) if (Math.abs(s.pos[i * 3]) < 1.0) inside++;
    assert.ok(inside <= 3, inside + ' fish inside the lane');
});

test('a repulsor scatters the fish and the burst decays', () => {
    const s = school(9);
    for (let i = 0; i < 300; i++) s.step(1 / 60, quiet);
    const target = { x: s.pos[0], y: s.pos[1], z: s.pos[2], radius: 2, strength: 6 };
    const before = Math.hypot(s.pos[0] - target.x, s.pos[1] - target.y, s.pos[2] - target.z);
    for (let i = 0; i < 60; i++) s.step(1 / 60, { ...quiet, repulsors: [target] });
    const after = Math.hypot(s.pos[0] - target.x, s.pos[1] - target.y, s.pos[2] - target.z);
    assert.ok(after > before + 0.5, 'fish moved away: ' + before + ' -> ' + after);
    assert.ok(s.burst[0] > 0.1);
    for (let i = 0; i < 240; i++) s.step(1 / 60, quiet);
    assert.ok(s.burst[0] < 0.05, 'burst decayed: ' + s.burst[0]);
});
