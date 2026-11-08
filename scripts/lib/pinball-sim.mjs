// Headless helpers for the pinball physics tests and shot tools (Node, no DOM).
import { BALL_M, G_SLOPE, STEP } from '../../ui/js/desktop/apps/pinball/physics/constants.js';
import { segIntersect } from '../../ui/js/desktop/apps/pinball/tables/kit/geom.js';

// Steps `world` for `ticks` ticks. `inputFn(tick, world)` returns the controls ({left, right, plunger}) for that tick
// (tick counts from 0 within this run); `onTick(tick, world)` runs after each step. Returns every event in emit order.
export function run(world, ticks, inputFn, onTick) {
    const events = [];
    for (let t = 0; t < ticks; t++) {
        world.step(inputFn ? inputFn(t, world) : undefined);
        for (const ev of world.events()) events.push(ev);
        if (onTick) onTick(t, world);
    }
    return events;
}

// Mechanical energy (J) of a ball: kinetic plus slope potential. Accepts a world ball or a snapshot ball.
export function energy(ball) {
    return 0.5 * BALL_M * (ball.vx * ball.vx + ball.vy * ball.vy) + BALL_M * G_SLOPE * ball.y;
}

// Collision outline of a layout: every wall segment (closed walls include the closing edge) and a containment test
// against the polygon of the wall named `containerId` (default 'outer', else the first closed wall, else the bounding box).
export function outline(layout, containerId) {
    const segments = [];
    for (const w of layout.walls || []) {
        const pts = w.points || [], n = w.closed ? pts.length : pts.length - 1;
        for (let i = 0; i < n; i++) {
            const a = pts[i], b = pts[(i + 1) % pts.length];
            segments.push({ id: w.id, a: [a[0], a[1]], b: [b[0], b[1]] });
        }
    }
    const box = (layout.walls || []).find(w => w.id === (containerId || 'outer')) || (layout.walls || []).find(w => w.closed);
    const poly = box ? box.points : null;
    let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
    for (const s of segments) for (const p of [s.a, s.b]) { minX = Math.min(minX, p[0]); maxX = Math.max(maxX, p[0]); minY = Math.min(minY, p[1]); maxY = Math.max(maxY, p[1]); }
    const contains = (x, y) => {
        if (!poly) return x >= minX && x <= maxX && y >= minY && y <= maxY;
        let inside = false;
        for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
            const [xi, yi] = poly[i], [xj, yj] = poly[j];
            if ((yi > y) !== (yj > y) && x < (xj - xi) * (y - yi) / (yj - yi) + xi) inside = !inside;
        }
        return inside;
    };
    return { segments, bounds: { minX, maxX, minY, maxY }, contains };
}

// The id of the first wall segment crossed by the path (ax, ay) -> (bx, by), or null.
export function crossesWall(ax, ay, bx, by, out) {
    for (const s of out.segments) if (segIntersect([ax, ay], [bx, by], s.a, s.b)) return s.id;
    return null;
}

// Frame accumulator driver: advances `world` by frames of 1/hz seconds until `ticks` ticks were stepped exactly.
export function runFrames(world, hz, ticks, inputFn) {
    const events = [];
    let acc = 0, t = 0;
    while (t < ticks) {
        acc += 1 / hz;
        while (acc >= STEP - 1e-12 && t < ticks) {
            world.step(inputFn ? inputFn(t, world) : undefined);
            for (const ev of world.events()) events.push(ev);
            acc -= STEP; t++;
        }
    }
    return events;
}
