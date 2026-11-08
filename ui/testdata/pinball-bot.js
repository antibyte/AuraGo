// A simple flipping bot for the pinball soak and harness (plan T14). No DOM; deterministic (seeded mulberry via rngNext).
//   createBot(layout, seed) -> { controls(balls, tick) -> {left, right, plunger} }
// `balls` is any array of {id, x, y, vx, vy, mode} (world snapshot balls or world.ctx.balls values).
import { rngNext } from '../js/desktop/apps/pinball/config.js';

const STEP = 1 / 480;
const PULL_RATE = 1.6;

export function createBot(layout, seed = 1) {
    const rng = { seed: seed >>> 0 };
    const rand = (a, b) => a + (b - a) * rngNext(rng);
    const flips = (layout.flippers || []).map(f => ({ f, on: false, until: 0, freeAt: 0, watch: new Map() }));
    const plunger = { phase: 'idle', until: 0, level: 0, wait: 0 };

    function updateFlipper(s, balls, tick) {
        const { f } = s;
        if (s.on) { if (tick >= s.until) { s.on = false; s.freeAt = tick + Math.round(0.08 / STEP); } return; }
        if (tick < s.freeAt) return;
        const reach = (f.length || 0.059) + 0.016;
        for (const b of balls) {
            if (b.mode !== 'field' && b.mode !== undefined) continue;
            const dx = f.side * (b.x - f.x), dy = b.y - f.y;
            if (dx < -0.012 || dx > reach || dy < -0.02 || dy > 0.09 || b.vy > 0.5) { s.watch.delete(b.id); continue; }
            let thr = s.watch.get(b.id);
            if (thr === undefined) { thr = rand(0.02, 0.06); s.watch.set(b.id, thr); }
            if (dy <= thr + dx * 0.15) {
                s.on = true; s.until = tick + Math.round(rand(0.10, 0.45) / STEP);
                s.watch.delete(b.id);
                return;
            }
        }
    }

    function updatePlunger(balls, tick) {
        const seated = balls.some(b => b.mode === 'shooter' && Math.abs(b.vy) < 0.05);
        const p = plunger;
        if (p.phase === 'idle') {
            if (seated && tick >= p.wait) { p.level = rand(0.55, 1); p.phase = 'pull'; p.until = tick + Math.round(p.level / PULL_RATE / STEP) + 24; }
        } else if (p.phase === 'pull') {
            if (tick >= p.until) { p.phase = 'release'; p.until = tick + 30; }
        } else if (tick >= p.until) { p.phase = 'idle'; p.wait = tick + Math.round(0.8 / STEP); }
        return p.phase === 'pull' ? p.level : 0;
    }

    return {
        controls(balls, tick) {
            const out = { left: false, right: false, plunger: updatePlunger(balls, tick) };
            for (const s of flips) {
                updateFlipper(s, balls, tick);
                if (s.on) out[s.f.key] = true;
            }
            return out;
        }
    };
}

export default createBot;
