// Route search for AuraGolf courses (developer tooling, Node only).
//
// Finds shots that play a course to the cup against the real Rapier physics, scores positions by
// the distance left along the course's corridors (not the straight line to the cup) and replays a
// found route the way the game plays it. Used by scripts/minigolf-route-search.mjs and by
// scripts/test-minigolf-courses.mjs.
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { readColliders } from '../../ui/js/desktop/apps/minigolf-collision.js';

const FRAME = 1 / 60;
// Cannon flights and long gate waits take longer than a plain putt: 30 s of simulated time per shot.
export const MAX_FRAMES = 1800;
export const TERMINAL = ['rest', 'cup', 'water', 'out'];
export const ANGLE_OFFSETS = [-60, -30, -15, -7, 0, 7, 15, 30, 60];
const OFFSET_ORDER = [...ANGLE_OFFSETS].sort((a, b) => Math.abs(a) - Math.abs(b));
export const POWERS = [0.15, 0.25, 0.35, 0.45, 0.55, 0.65, 0.75, 0.85, 0.925, 1];
export const LOOKAHEAD = [2, 4, 7, 11];
export const BEAM = 6;
const SHOT_PENALTY = 0.8;
const CELL = 0.28;
const MAX_SHOTS = 12;

const toArrayBuffer = buffer => buffer.buffer.slice(buffer.byteOffset, buffer.byteOffset + buffer.byteLength);

// Reads <dir>/manifest.json. Version 2 courses get their colliders from the course GLB; version 1 keeps
// the colliders that the manifest lists. Returns { manifest, courses }.
export async function loadCourses(dir) {
    const manifest = JSON.parse(await readFile(path.join(dir, 'manifest.json'), 'utf8'));
    if (manifest.version !== 1 && manifest.version !== 2) throw new Error(`unsupported manifest version ${manifest.version}`);
    const courses = [];
    for (const entry of manifest.courses) {
        if (manifest.version === 1) { courses.push(entry); continue; }
        const glb = await readFile(path.join(dir, entry.model));
        courses.push({ ...entry, colliders: readColliders(toArrayBuffer(glb)) });
    }
    return { manifest, courses };
}

// Plays one shot from `position` and returns the terminal event (rest, cup, water, out) or null after MAX_FRAMES.
export function runShot(physics, position, shot, events, { resetTime = true } = {}) {
    events.length = 0;
    physics.setBall(position);
    if (resetTime) physics.resetTime();
    if (!physics.shoot([Math.sin(shot.angle), Math.cos(shot.angle)], shot.power)) throw new Error('the physics rejected the shot');
    for (let frame = 0; frame < MAX_FRAMES; frame++) {
        physics.step(FRAME);
        const terminal = events.find(event => TERMINAL.includes(event.type));
        if (terminal) return terminal;
    }
    return null;
}

// ---- Progress along the corridors -------------------------------------------------------------

function makeLine(points) {
    const cumulative = [0];
    for (let i = 1; i < points.length; i++) cumulative.push(cumulative[i - 1] + Math.hypot(...points[i].map((value, axis) => value - points[i - 1][axis])));
    return { points, cumulative, length: cumulative[cumulative.length - 1] };
}

// Nearest point of a polyline to p: { distance, s (arc length), index (segment start), point }.
function project(line, p) {
    let best = null;
    for (let i = 0; i + 1 < line.points.length; i++) {
        const a = line.points[i], b = line.points[i + 1];
        const d = [b[0] - a[0], b[1] - a[1], b[2] - a[2]], lengthSquared = d[0] * d[0] + d[1] * d[1] + d[2] * d[2];
        const t = lengthSquared > 0 ? Math.max(0, Math.min(1, ((p[0] - a[0]) * d[0] + (p[1] - a[1]) * d[1] + (p[2] - a[2]) * d[2]) / lengthSquared)) : 0;
        const point = [a[0] + d[0] * t, a[1] + d[1] * t, a[2] + d[2] * t];
        const distance = Math.hypot(p[0] - point[0], p[1] - point[1], p[2] - point[2]);
        if (!best || distance < best.distance) best = { distance, s: line.cumulative[i] + Math.sqrt(lengthSquared) * t, index: i, point };
    }
    return best;
}

const models = new WeakMap();
// The main route is `path` (else the first corridor); every other corridor is a branch that rejoins the main route
// where it ends. Distances to the cup are measured along the main route up to the cup's own projection.
function modelOf(course) {
    let model = models.get(course);
    if (model) return model;
    const usable = list => Array.isArray(list) && list.length >= 2 && list.every(point => Array.isArray(point) && point.length === 3);
    const lines = (Array.isArray(course.corridors) && course.corridors.length ? course.corridors : [course.path]).filter(usable).map(makeLine);
    const main = usable(course.path) ? makeLine(course.path) : lines[0];
    if (!main) throw new Error(`${course.id}: course has neither path nor corridors`);
    const cupS = project(main, course.cup).s;
    const branches = lines.filter(line => line.points !== main.points).map(line => {
        const join = project(main, line.points[line.points.length - 1]);
        return { ...line, join, endRemaining: Math.abs(cupS - join.s) };
    });
    model = { main, branches, cupS, total: Number.isFinite(course.length) && course.length > 0 ? course.length : cupS };
    models.set(course, model);
    return model;
}

// Nearest route line to p and the distance still to travel to the cup along it. The main route wins near-ties.
function locate(course, p) {
    const model = modelOf(course);
    let best = { line: model.main, hit: project(model.main, p), branch: null };
    best.score = best.hit.distance - 0.05;
    best.left = Math.abs(model.cupS - best.hit.s);
    for (const branch of model.branches) {
        const hit = project(branch, p);
        if (hit.distance < best.score) best = { line: branch, hit, branch, score: hit.distance, left: branch.length - hit.s + branch.endRemaining };
    }
    const dx = p[0] - course.cup[0], dz = p[2] - course.cup[2];
    if (Math.hypot(dx, dz) < (Number.isFinite(course.cupRadius) ? course.cupRadius : 0.32)) best.left = 0;
    return { ...best, model };
}

// Metres travelled along the main route (through a branch, the length its detour is worth). Sits at the tee near 0.
export function progress(course, position) {
    const { left, model } = locate(course, position);
    return model.total - left;
}

// Metres left to the cup along the route graph; 0 inside the cup opening.
export function remaining(course, position) {
    return Math.max(0, modelOf(course).total - progress(course, position));
}

// Points `distance` metres ahead of p along the nearest corridor (a branch continues on the main route after its end).
function pointsAhead(course, position, distances) {
    const { line, hit, branch, model, left } = locate(course, position);
    const forward = [hit.point, ...line.points.slice(hit.index + 1)];
    if (branch) forward.push(branch.join.point, ...model.main.points.slice(branch.join.index + 1));
    const cumulative = [0];
    for (let i = 1; i < forward.length; i++) cumulative.push(cumulative[i - 1] + Math.hypot(...forward[i].map((value, axis) => value - forward[i - 1][axis])));
    return distances.map(distance => {
        const want = Math.min(distance, left, cumulative[cumulative.length - 1]);
        let i = 1;
        while (i < forward.length - 1 && cumulative[i] < want) i++;
        const span = cumulative[i] - cumulative[i - 1] || 1, t = Math.max(0, Math.min(1, (want - cumulative[i - 1]) / span));
        return forward[i - 1].map((value, axis) => value + (forward[i][axis] - value) * t);
    });
}

// ---- Search -----------------------------------------------------------------------------------

const angleTo = (from, to) => Math.atan2(to[0] - from[0], to[2] - from[2]);
const keyOf = position => position.map(value => Math.round(value / CELL)).join(',');

// Aim angles from a resting position: straight at the cup and at points 2, 4, 7 and 11 m along the corridor,
// each with the angle offsets. Duplicates (a straight lane aims every target the same way) are dropped.
function aimAngles(course, position) {
    const targets = [course.cup, ...pointsAhead(course, position, LOOKAHEAD)].filter(target => Math.hypot(target[0] - position[0], target[2] - position[2]) >= 0.25);
    const seen = new Set(), angles = [];
    // Smallest offsets first so the first route found is the straightest one.
    for (const offset of OFFSET_ORDER) for (const target of targets) {
        const angle = angleTo(position, target) + offset * Math.PI / 180, cell = Math.round(angle / 0.002);
        if (!seen.has(cell)) { seen.add(cell); angles.push(angle); }
    }
    return angles;
}

// Beam search over resting positions. Returns the shots [{ angle, power }] that reach the cup or null.
// Every shot starts at obstacle time 0; timedRoute() adds the waits that line the shots up again.
// `trace` (optional array) receives { depth, states, best: { position, remaining, shots } } for each depth.
export function findRoute(physics, course, events, { beam = BEAM, extraShots = 2, trace } = {}) {
    const limit = Math.min(MAX_SHOTS, (Number.isFinite(course.par) ? course.par : 3) + extraShots);
    let states = [{ position: course.tee, shots: [], remaining: remaining(course, course.tee) }];
    const seen = new Set([keyOf(course.tee)]);
    for (let depth = 0; depth < limit && states.length; depth++) {
        const next = [];
        for (const state of states) {
            for (const angle of aimAngles(course, state.position)) for (const power of POWERS) {
                const result = runShot(physics, state.position, { angle, power }, events);
                if (!result) continue;
                const shots = [...state.shots, { angle, power }];
                if (result.type === 'cup') return shots;
                if (result.type !== 'rest') continue;
                const cell = keyOf(result.position);
                if (seen.has(cell)) continue;
                seen.add(cell);
                next.push({ position: result.position, shots, remaining: remaining(course, result.position) });
            }
        }
        const score = state => state.remaining + SHOT_PENALTY * state.shots.length;
        next.sort((a, b) => score(a) - score(b));
        states = next.slice(0, beam);
        if (trace) trace.push({ depth: depth + 1, states: next.length, best: next[0] && { position: next[0].position, remaining: next[0].remaining, shots: next[0].shots.length } });
    }
    return null;
}

// Replays a route with continuous obstacle time, like the real game. With `period`, every shot first waits until the
// obstacles return to phase 0 (the state the search assumed) and records that wait on the shot.
function play(physics, course, route, events, period = 0) {
    let position = course.tee;
    const positions = [];
    physics.resetTime();
    for (let index = 0; index < route.length; index++) {
        if (period) {
            const wait = Math.round((period - physics.getSnapshot().time % period) % period * 120) / 120;
            if (wait > 0) route[index].wait = wait; else delete route[index].wait;
        }
        for (let tick = 0; tick < Math.round((route[index].wait || 0) * 120); tick++) physics.step(1 / 120);
        const terminal = runShot(physics, position, route[index], events, { resetTime: false });
        const wanted = index === route.length - 1 ? 'cup' : 'rest';
        if (!terminal || terminal.type !== wanted) return { ok: false, shots: index + 1, positions, failed: { index, expected: wanted, got: terminal?.type ?? null } };
        position = terminal.position;
        positions.push(position);
    }
    return { ok: true, shots: route.length, positions };
}

// Plays the route as the game does (waits included) and reports { ok, shots, positions }.
export function replayRoute(physics, course, route, events) {
    return play(physics, course, route.map(shot => ({ ...shot })), events);
}

// The search starts every shot at obstacle time 0. Routes behind moving parts therefore get waits that realign
// the obstacles to phase 0: whole `course.cycle` seconds when set, else the single obstacle period 2π / speed.
export function timedRoute(physics, course, route, events) {
    if (!route) return null;
    const timed = route.map(shot => ({ ...shot }));
    if (play(physics, course, timed, events).ok) return timed;
    const periods = course.cycle ? [course.cycle] : [...new Set((course.obstacles || []).map(obstacle => 2 * Math.PI / obstacle.speed))];
    if (periods.length !== 1 || !play(physics, course, timed, events, periods[0]).ok) return null;
    return play(physics, course, timed, events).ok ? timed : null;
}
