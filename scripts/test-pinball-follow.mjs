// Ball camera maths: target choice, clamps, spring convergence, multiball zoom, resting framing, no NaN.
import assert from 'node:assert/strict';
import { FOLLOW, followGoal, createFollow, stepFollow } from '../ui/js/desktop/apps/pinball/render/follow.js';

const ball = (y, extra = {}) => ({ id: 1, x: 0, y, h: 0, vx: 0, vy: 0, mode: 'field', hidden: false, ...extra });
const half = FOLLOW.depth / 2;

// The lowest visible in-play ball wins; hidden and off balls are ignored.
let g = followGoal([ball(0.9, { id: 1 }), ball(0.5, { id: 2 }), ball(0.2, { id: 3, hidden: true }), ball(0.1, { id: 4, mode: 'off' }), ball(0.05, { id: 'pearl_ball', mode: 'captive' })]);
assert.ok(g.balls === 2 && g.y > 0.45 && g.y < 0.8, 'follows the lower of the two live balls: ' + JSON.stringify(g));

// Clamp: a low ball leaves the flippers view at the bottom, a high ball the top arc at the upper limit.
g = followGoal([ball(0.04)]);
assert.ok(Math.abs(g.y - (FOLLOW.yMin + half)) < 1e-9, 'bottom clamp keeps the flippers in frame');
g = followGoal([ball(1.05)]);
assert.ok(Math.abs(g.y - (FOLLOW.yMax - half)) < 1e-9, 'top clamp stops at the top arc');
g = followGoal([ball(0.05, { vy: -3 })]);
assert.ok(g.y - half >= FOLLOW.yMin - 1e-9, 'lookahead cannot push the window below the table');

// Lookahead follows the velocity (bounded); held balls do not look ahead; a raised ball shifts the target up the table.
const base = followGoal([ball(0.5)]).y;
assert.ok(followGoal([ball(0.5, { vy: 2 })]).y > base && followGoal([ball(0.5, { vy: 2 })]).y - base <= FOLLOW.lookMax + 1e-9);
assert.equal(followGoal([ball(0.5, { vy: 2, mode: 'held' })]).y, base);
assert.ok(followGoal([ball(0.5, { h: 0.08 })]).y > base, 'ramp height raises the target');
assert.equal(followGoal([ball(0.15, { x: 0.24, mode: 'shooter' })]).y, FOLLOW.yMin + half, 'shooter lane stays framed at the bottom');

// Lateral follow is limited to the range the window leaves (0 when the full width fits).
assert.equal(followGoal([ball(0.5, { x: 0.2 })]).x, 0);
const gx = followGoal([ball(0.5, { x: 0.25 })], { xRange: 0.05 }).x;
assert.ok(gx > 0 && gx <= 0.05);

// Multiball: close balls keep the base depth, far apart balls zoom out (never beyond the whole table) and the lowest stays in view.
assert.equal(followGoal([ball(0.4, { id: 1 }), ball(0.5, { id: 2 })]).depth, FOLLOW.depth);
g = followGoal([ball(0.15, { id: 1 }), ball(0.95, { id: 2 })]);
assert.ok(g.depth > FOLLOW.depth && g.depth <= FOLLOW.maxDepth, 'spread zooms out');
assert.ok(g.y - g.depth / 2 <= 0.15 && g.y + g.depth / 2 >= 0.95 - 0.01 || g.depth === FOLLOW.maxDepth, 'both lowest balls framed');

// No balls: resting framing around the flippers, finite everywhere (including garbage input).
for (const none of [[], null, undefined, [ball(0.3, { hidden: true })], [{ x: NaN, y: NaN, mode: 'field' }]]) {
    g = followGoal(none);
    assert.deepEqual([g.y, g.x, g.depth, g.balls], [FOLLOW.yMin + half, 0, FOLLOW.depth, 0].map((v, i) => i === 0 ? Math.max(v, Math.min(FOLLOW.restY, FOLLOW.yMax - half)) : v));
}
const odd = followGoal([ball(0.5, { vy: NaN, vx: Infinity, h: undefined })]);
assert.ok(Object.values(odd).every(Number.isFinite), 'NaN input stays finite');

// Spring: converges to the goal, dt 0 changes nothing, and 60 Hz equals 144 Hz for the same goal and time.
const run = (hz, secs, goalOf) => {
    const s = createFollow(); stepFollow(s, goalOf(0), 0);
    const n = Math.round(secs * hz);
    for (let i = 1; i <= n; i++) stepFollow(s, goalOf(i / hz), 1 / hz);
    return s;
};
const still = () => ({ y: 0.7, x: 0, depth: FOLLOW.depth });
let a = run(60, 3, still); assert.ok(Math.abs(a.y - 0.7) < 1e-3, 'converges');
const s0 = createFollow(); stepFollow(s0, { y: 0.3, x: 0, depth: FOLLOW.depth }, 0); const y0 = s0.y;
stepFollow(s0, { y: 0.6, x: 0, depth: FOLLOW.depth }, 0); assert.equal(s0.y, y0, 'dt 0 is a no-op');
const sine = t => ({ y: 0.5 + 0.2 * Math.sin(t * 2), x: 0, depth: FOLLOW.depth });
const b60 = run(60, 1.2, t => t < 0.5 ? { y: 0.3, x: 0, depth: FOLLOW.depth } : { y: 0.6, x: 0, depth: FOLLOW.depth });
const b144 = run(144, 1.2, t => t < 0.5 ? { y: 0.3, x: 0, depth: FOLLOW.depth } : { y: 0.6, x: 0, depth: FOLLOW.depth });
assert.ok(Math.abs(b60.y - b144.y) < 0.01, 'frame-rate independent step response: ' + b60.y + ' vs ' + b144.y);
const c60 = run(60, 2, sine), c144 = run(144, 2, sine);
assert.ok(Math.abs(c60.y - c144.y) < 0.02, 'frame-rate independent tracking');
// A big step never overshoots past the table; the depth spring stays in range.
const w = createFollow(); stepFollow(w, followGoal([ball(1.0)]), 0);
for (let i = 0; i < 600; i++) { stepFollow(w, followGoal([ball(0.02)]), 1 / 60); assert.ok(w.y - w.depth / 2 >= FOLLOW.yMin - 1e-9 && w.y + w.depth / 2 <= FOLLOW.yMax + 1e-9); }
assert.ok(Math.abs(w.y - (FOLLOW.yMin + half)) < 1e-3);
console.log('pinball follow ok');
