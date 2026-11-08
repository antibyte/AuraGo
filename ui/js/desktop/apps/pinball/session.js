// AuraPinball session: physics world + rules game behind one fixed-step tick. DOM-free and deterministic.
//   createSession(table, {seed}) -> { world, game, startGame, endGame, tick, nudge, onEvent, onPresentation, lampModes,
//     dmdState, toyState, snapshotPair, resetTraining, setScores, stress, dispose, ... }
// One tick = one physics STEP: input edges -> rules.button, world.step, events -> rules.handle + listeners, rules.tick,
// then every queued command is routed (physics -> world.command, everything else -> onPresentation).
import { STEP } from './physics/constants.js';
import { createWorld } from './physics/world.js';
import { PHYSICS_COMMANDS } from './contracts.js';
import { copy } from './config.js';
import * as rules from './rules/core.js';

const PHYSICS = new Set(PHYSICS_COMMANDS);
const NUDGE_FULL = 0.2;          // m/s of ball delta-v that counts as a full-strength nudge for the tilt energy

export function createSession(table, { seed = 1 } = {}) {
    const layout = table.layout;
    let world = createWorld(layout, { seed }), game = null, baseSeed = (seed >>> 0) || 1, disposed = false;
    const eventListeners = new Set(), presentationListeners = new Set();
    const prevControls = { left: false, right: false, plunger: 0, start: false };
    let snapPrev = world.snapshot(), snapCurr = world.snapshot(), ticks = 0;
    const attract = { scores: [], last: [] };

    function route() {
        if (!game) return;
        for (const command of rules.take(game)) {
            if (PHYSICS.has(command.type)) world.command(command);
            else for (const fn of presentationListeners) fn(command);
        }
    }

    function resetEdges() { prevControls.left = false; prevControls.right = false; prevControls.plunger = 0; prevControls.start = false; }
    function resetSnapshots() { snapPrev = world.snapshot(snapPrev); snapCurr = world.snapshot(snapCurr); ticks = 0; }

    function setScores(list) { attract.scores = Array.isArray(list) ? list.map(row => ({ name: row.name, score: row.score })) : []; }

    function startGame(options = {}) {
        world.dispose();
        baseSeed = ((options.seed ?? seed) >>> 0) || 1;
        world = createWorld(layout, { seed: baseSeed });
        if (Array.isArray(options.scores)) setScores(options.scores);
        game = rules.createGame(table, {
            players: options.players, training: !!options.training, seed: baseSeed, scores: options.scores || attract.scores
        });
        rules.startGame(game);
        resetEdges(); resetSnapshots();
        route();
        return game;
    }

    function endGame() {
        if (game && !game.training && game.phase === 'over') attract.last = game.players.map(p => ({ name: p.name, score: p.score }));
        game = null;
        world.dispose();
        world = createWorld(layout, { seed: baseSeed });
        resetEdges(); resetSnapshots();
    }

    function tick(controls) {
        const c = controls || {};
        const left = !!c.left, right = !!c.right, plunger = Number(c.plunger) || 0, start = !!c.start;
        if (game) {
            if (left !== prevControls.left) rules.button(game, 'left', left);
            if (right !== prevControls.right) rules.button(game, 'right', right);
            if (start && !prevControls.start) rules.button(game, 'start', true);
            // A plunger release launches the ball physically; with no ball in the lane it is the launch button
            // (mode select and initials entry confirm with it).
            if (prevControls.plunger > 0.05 && plunger < 0.02 && !snapCurr.balls.some(b => b.mode === 'shooter')) rules.button(game, 'launch', true);
        }
        prevControls.left = left; prevControls.right = right; prevControls.plunger = plunger; prevControls.start = start;

        world.step({ left, right, plunger });
        ticks++;
        const events = world.events();
        for (const ev of events) {
            if (game) rules.handle(game, ev);
            for (const fn of eventListeners) fn(ev);
        }
        if (game) { rules.tick(game, STEP); route(); }
        const spare = snapPrev; snapPrev = snapCurr; snapCurr = world.snapshot(spare);
        return events.length;
    }

    // Testing hook: push one synthetic physics event through the rules and the listeners.
    function inject(ev) {
        if (game) rules.handle(game, ev);
        for (const fn of eventListeners) fn(ev);
        route();
    }

    function nudge(dx, dy) {
        if (!game) return false;
        const strength = Math.min(1, Math.hypot(dx, dy) / NUDGE_FULL);
        const allowed = rules.nudge(game, strength);
        if (allowed) world.command({ type: 'nudge', dx, dy });
        route();
        return allowed;
    }

    function lampModes(out) {
        if (game) return rules.lampModes(game, out);
        out.fill(0);
        return out;
    }

    function dmdState() {
        if (game) return rules.dmdState(game);
        return { scene: 'attract', params: { title: layout.meta && layout.meta.title, scores: attract.scores, replayAt: (table.rules && table.rules.replay) || 0, last: attract.last, free: true } };
    }

    // Roll-sound input: one entry per live ball (speed, mode, kind of path the ball rides, x for the pan).
    const pathKinds = new Map((layout.paths || []).map(path => [path.id, path.kind]));
    const rolls = [];
    function rollBalls() {
        rolls.length = 0;
        for (const b of world.ctx.balls.values()) {
            const onPath = b.mode === 'path' && b.path;
            rolls.push({ speed: onPath ? Math.abs(b.path.v) : Math.hypot(b.vx, b.vy), mode: b.mode, pathKind: onPath ? pathKinds.get(b.path.id) : undefined, x: b.x, hidden: !!b.hidden });
        }
        return rolls;
    }

    function resetTraining() { if (!game) return; rules.resetTraining(game); route(); }

    // Testing hook: a training game with `n` balls moving on the field.
    function stress(n = 4) {
        if (!game) throw Error('game');
        game.training = true; game.ballSave.unlimited = true;
        world.command({ type: 'clearBalls' });
        const ids = [];
        for (let i = 0; i < n; i++) {
            const id = game.balls.nextId + i;
            ids.push(id);
            world.placeBall(id, -0.13 + (i % 4) * 0.09, 0.42 + Math.floor(i / 4) * 0.05, (i % 3 - 1) * 0.8, 1.2);
        }
        rules.debugBalls(game, ids);
        game.phase = 'play'; game.launched = true;
        return ids;
    }

    return {
        get world() { return world; },
        get game() { return game; },
        get ticks() { return ticks; },
        table, layout, startGame, endGame, tick, nudge, inject, setScores, lampModes, dmdState, resetTraining, stress, rollBalls,
        toyState: () => (game ? rules.toyState(game) : {}),
        onEvent(fn) { eventListeners.add(fn); return () => eventListeners.delete(fn); },
        onPresentation(fn) { presentationListeners.add(fn); return () => presentationListeners.delete(fn); },
        // The two newest tick snapshots (before and after the last tick), reused buffers: the render blends between them.
        snapshotPair() { return { prev: snapPrev, curr: snapCurr }; },
        snapshot: () => copy({ tick: snapCurr.tick, balls: snapCurr.balls, flippers: Array.from(snapCurr.flippers), plunger: snapCurr.plunger }),
        dispose() { if (disposed) return; disposed = true; world.dispose(); eventListeners.clear(); presentationListeners.clear(); game = null; }
    };
}

export default createSession;
