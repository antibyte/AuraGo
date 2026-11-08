// AuraPinball rules framework, part 1: lifecycle, players, drains, ball save, tilt, extra ball, replay, bonus, match,
// training and DMD state. Pure, DOM-free and deterministic (rngNext only). Game state stays plain JSON.
import { SCORING_EVENTS } from '../contracts.js';
import { rngNext, copy } from '../config.js';
import {
    cur, pack, cmd, dmd, dmdClear, show, gi, cue, music, big, addBall, dropBall, saveActive, addScore, bonusLines, bonusSum,
    featureStep, rotateLanes, ejectHole
} from './features.js';
import {
    makeCtx, dispatchShots, tickModes, endMode, endMultiball, cycleSelect, confirmSelect, idleMode, idleMb
} from './modes.js';
import { rankOf, nextInitials, initialsButton, tickInitials } from './highscore.js';
import { lampModes } from './lamps.js';

export { lampModes };
const BALLS = 3;
const SCORING = new Set(SCORING_EVENTS);
const live = g => g.started && (g.phase === 'ballStart' || g.phase === 'play');

// ---------------------------------------------------------------- creation
function newPlayer(g, name) {
    const pk = pack(g);
    return {
        name, score: 0, ball: 0, extraBalls: 0, ebLit: false, bonusX: 1, counters: {},
        lanesTop: new Array(pk.topLanes?.switches?.length || 0).fill(false),
        lanesLower: new Array(pk.lowerLanes?.switches?.length || 0).fill(false),
        kickbackLit: !!(pk.kickback && pk.kickback.litAtStart),
        lockLit: false, locks: 0, modeReady: true, modesPlayed: [], modesCompleted: [], wizardReady: false,
        replayAwarded: false, tilt: false, warnings: 0, table: pk.hooks?.init ? pk.hooks.init() : {}
    };
}

export function createGame(table, o = {}) {
    const training = !!o.training;
    const names = (Array.isArray(o.players) && o.players.length ? o.players : ['P1']).slice(0, training ? 1 : 4).map((n, i) => String(n || 'P' + (i + 1)));
    const g = {
        v: 1, table: table.layout.id, seed: ((o.seed ?? 1) >>> 0) || 1, training, started: false, phase: 'ballStart', time: 0,
        players: [], turn: 0,
        balls: { active: 0, nextId: 1, locked: 0, ids: [], lockedIds: [] },
        ballSave: { armed: false, until: null, soft: null, unlimited: training, seconds: 8 },
        tiltEnergy: 0, nextWarning: 0, coilsOff: false,
        skill: { lane: 0, live: false, until: 0, superUntil: 0 }, combo: { last: null, until: 0, count: 0 },
        mb: idleMb(), mode: idleMode(), select: null, bonusCount: null, match: null,
        initials: { queue: [], player: -1, letters: '', cur: 0, rank: 0, until: 0 },
        results: [], queue: [], scores: copy(Array.isArray(o.scores) ? o.scores : []),
        btn: { left: false, right: false, leftAt: -1, rightAt: -1 },
        music: 'attract', base: '', lastBig: -1, loopAt: 0, launched: false
    };
    Object.defineProperty(g, '_t', { value: table, enumerable: false });
    g.players = names.map(n => newPlayer(g, n));
    return g;
}

export function startGame(g) {
    g.started = true;
    cue(g, 'startGame');
    startBall(g, false);
    sync(g);
}

// ---------------------------------------------------------------- ball lifecycle
function startBall(g, again) {
    const p = cur(g), pk = pack(g);
    if (!again) p.ball++;
    p.counters = {}; p.bonusX = 1; p.warnings = 0; p.tilt = false;
    p.lanesTop.fill(false); p.lanesLower.fill(false);
    if (g.coilsOff) { g.coilsOff = false; cmd(g, { type: 'coils', on: true }); gi(g, 'all', 'on', 0); }
    Object.assign(g, { tiltEnergy: 0, nextWarning: 0, select: null, bonusCount: null, phase: 'ballStart', launched: false, base: '' });
    g.mode = idleMode(); g.mb = idleMb(); g.combo = { last: null, until: 0, count: 0 };
    g.ballSave = { armed: !!pk.ballSave, until: null, soft: null, unlimited: g.training, seconds: pk.ballSave?.seconds ?? 8 };
    const n = pk.topLanes?.switches?.length || 0;
    g.skill = { lane: n ? Math.floor(rngNext(g) * n) : -1, live: true, until: 1e9, superUntil: 0 };
    if (pk.kickback) cmd(g, { type: 'kickback', id: pk.kickback.device, armed: p.kickbackLit });
    if (pk.lock?.diverter) cmd(g, { type: 'diverter', id: pk.lock.diverter.id, on: p.lockLit });
    pk.hooks?.onBallStart?.(makeCtx(g));
    addBall(g, false);
    if (again) { dmd(g, 'shootAgain', {}, 1600, 5); cue(g, 'shootAgain'); }
}

function onLaunch(g) {
    if (g.phase !== 'ballStart') return;
    g.phase = 'play'; g.launched = true;
    g.skill.until = g.time + 10;
    g.skill.superUntil = g.time + (pack(g).skill?.super?.window ?? 6);
}

function reserve(g) {
    g.phase = 'ballStart'; g.launched = false;
    g.skill = { ...g.skill, live: true, until: 1e9, superUntil: 0 };
    addBall(g, false);
}

function onDrain(g, ev) {
    if (!live(g) || !dropBall(g, ev.ball)) return;
    const p = cur(g);
    if (!p.tilt) { cue(g, 'drain'); show(g, 'sweepDown', { ms: 700 }); gi(g, 'all', 'flicker', 400); }
    if (g.training) { if (!g.balls.ids.length) reserve(g); return; }
    if (!p.tilt && saveActive(g)) {
        addBall(g, true);
        if (g.mb.state === 'off') g.ballSave.armed = false;
        dmd(g, 'ballSaved', {}, 1500, 5);
        cue(g, 'ballSaved'); show(g, 'sweepUp', { ms: 800 });
        return;
    }
    if (g.mb.state !== 'off' && g.balls.ids.length <= 1) endMultiball(g);
    if (!g.balls.ids.length) endBall(g);
}

function onSearchFail(g, ev) {
    if (!live(g)) return;
    const ids = g.balls.ids;
    if (ev.ball === undefined ? !ids.length : !ids.includes(ev.ball)) return;
    if (ev.ball !== undefined) cmd(g, { type: 'removeBall', id: ev.ball });
    dropBall(g, ev.ball);
    addBall(g, g.phase === 'play');
}

function endBall(g) {
    const p = cur(g), pk = pack(g), b = g.balls;
    if (b.lockedIds.length && pk.lock?.hole) cmd(g, { type: 'releaseToTrough', id: pk.lock.hole });
    b.lockedIds = []; b.locked = 0;
    if (g.mode.id) endMode(g, false, true);
    if (pk.lock?.diverter) cmd(g, { type: 'diverter', id: pk.lock.diverter.id, on: false });
    g.mb = idleMb(); g.select = null; g.skill.live = false;
    if (p.tilt) { afterBonus(g); return; }
    g.phase = 'bonus';
    const lines = bonusLines(g), sum = bonusSum(g);
    g.bonusCount = { lines, x: p.bonusX, sum, total: sum * p.bonusX, k: -1, until: g.time, paid: false, startedAt: g.time };
    bonusEnter(g, 0);
}

// ---------------------------------------------------------------- end-of-ball bonus count
function payBonus(g) {
    const b = g.bonusCount;
    if (b.paid) return;
    b.paid = true;
    addScore(g, b.total);
    checkReplay(g);
}
function bonusEnter(g, k) {
    const b = g.bonusCount, n = b.lines.length;
    const t0 = k === 0 ? g.time : b.until; // chain from the previous deadline so a long dt still walks every step
    b.k = k;
    if (k < n) {
        const l = b.lines[k];
        dmd(g, 'bonus', { phase: 'line', key: l.key, count: l.n, value: l.value, x: b.x, subtotal: b.sum }, 0, 5);
        cue(g, 'bonusTick'); b.until = t0 + 0.45;
    } else if (k === n) {
        dmd(g, 'bonus', { phase: 'mult', x: b.x, subtotal: b.sum, total: b.total }, 0, 5);
        b.until = t0 + 0.6;
    } else if (k === n + 1) {
        payBonus(g);
        dmd(g, 'bonus', { phase: 'total', x: b.x, subtotal: b.sum, total: b.total }, 0, 5);
        cue(g, 'bonusTotal'); b.until = t0 + 1.0;
    } else { dmdClear(g); g.bonusCount = null; afterBonus(g); }
}
function trySkip(g) {
    const b = g.bonusCount, s = g.btn;
    if (g.phase !== 'bonus' || !b || !s.left || !s.right || Math.min(s.leftAt, s.rightAt) < b.startedAt) return;
    payBonus(g);
    dmdClear(g);
    g.bonusCount = null;
    afterBonus(g);
}

function afterBonus(g) {
    const p = cur(g), n = g.players.length;
    if (p.extraBalls > 0) { p.extraBalls--; startBall(g, true); return; }
    for (let i = 1; i <= n; i++) {
        const j = (g.turn + i) % n;
        if (g.players[j].ball < BALLS) { g.turn = j; startBall(g, false); return; }
    }
    enterMatch(g);
}

function enterMatch(g) {
    const digit = Math.floor(rngNext(g) * 10) * 10;
    const matched = g.players.map((p, i) => p.score % 100 === digit ? i : -1).filter(i => i >= 0);
    g.phase = 'match';
    g.match = { digit, matched, until: g.time + 3.5 };
    const last = g.players[g.players.length - 1].score % 100;
    dmd(g, 'match', { match: digit, last, hit: matched.length > 0 }, 3200, 5);
    cue(g, 'match');
    if (matched.length) cue(g, 'knocker');
}
function gameOver(g) {
    g.phase = 'over';
    dmdClear(g);
    dmd(g, 'gameOver', {}, 3000, 5);
    cue(g, 'gameOver');
}
function afterMatch(g) {
    g.initials.queue = g.players.map((p, i) => i);
    if (nextInitials(g)) g.phase = 'initials'; else gameOver(g);
}

// ---------------------------------------------------------------- replay and tilt
function checkReplay(g) {
    const p = cur(g), r = pack(g).replay;
    if (g.training || !r || p.replayAwarded || p.score < r) return;
    p.replayAwarded = true; p.extraBalls++;
    dmd(g, 'replay', { score: r }, 2000, 5);
    cue(g, 'replay'); cue(g, 'knocker');
}

function doTilt(g) {
    const p = cur(g);
    p.tilt = true; p.counters = {}; p.bonusX = 1; g.select = null; g.ballSave.armed = false; g.coilsOff = true;
    cmd(g, { type: 'coils', on: false });
    gi(g, 'all', 'off', 0);
    dmd(g, 'tilt', {}, 3000, 9);
    cue(g, 'tilt'); show(g, 'blackout', { ms: 3000, priority: 9 });
    cmd(g, { type: 'shake', strength: 0.010 });
}

export function nudge(g, strength = 1) {
    if (!live(g) || cur(g).tilt) return false;
    if (g.training) return true;
    const p = cur(g);
    g.tiltEnergy += 0.9 * strength;
    if (g.tiltEnergy > 1.5 && g.time >= g.nextWarning) {
        g.nextWarning = g.time + 1;
        if (++p.warnings >= 3) { doTilt(g); sync(g); return false; }
        dmd(g, 'tiltWarning', { n: p.warnings }, 1500, 7);
        cue(g, 'tiltWarning');
    }
    return true;
}

// ---------------------------------------------------------------- events
export function handle(g, ev) {
    if (!ev || !g.started || g.phase === 'over') return;
    if (ev.type === 'drain') { onDrain(g, ev); sync(g); return; }
    if (ev.type === 'searchFail') { onSearchFail(g, ev); sync(g); return; }
    if (ev.type === 'launch') onLaunch(g);
    if (!live(g)) return;
    const p = cur(g);
    if (p.tilt) { if (ev.type === 'hole') ejectHole(g, ev.id); return; }
    if (ev.type === 'kickback') p.kickbackLit = false;
    featureStep(g, ev, SCORING.has(ev.type));
    pack(g).hooks?.onEvent?.(makeCtx(g), ev);
    const held = dispatchShots(g, ev);
    checkReplay(g);
    if (ev.type === 'hole' && !held) ejectHole(g, ev.id);
    sync(g);
}

export function button(g, name, down) {
    if (name === 'left' || name === 'right') { g.btn[name] = !!down; if (down) g.btn[name + 'At'] = g.time; }
    if (!down || !g.started) return;
    if (g.phase === 'bonus') trySkip(g);
    else if (g.phase === 'initials') { if (!initialsButton(g, name)) gameOver(g); }
    else if (live(g) && !cur(g).tilt) {
        const dir = name === 'right' ? 1 : name === 'left' ? -1 : 0;
        if (g.select) { if (dir) cycleSelect(g, dir); else if (name === 'start' || name === 'launch') confirmSelect(g); }
        else if (dir) rotateLanes(g, dir);
    }
    sync(g);
}

export function tick(g, dt) {
    if (!g.started) return;
    g.time += dt;
    g.tiltEnergy = Math.max(0, g.tiltEnergy - 0.7 * dt);
    if (live(g)) {
        const s = g.ballSave;
        if (s.armed && s.until !== null && g.time > s.until && !s.unlimited) s.armed = false;
        if (g.skill.live && g.launched && g.time > g.skill.until) g.skill.live = false;
        if (!cur(g).tilt) tickModes(g);
    } else if (g.phase === 'bonus') {
        while (g.bonusCount && g.time >= g.bonusCount.until) bonusEnter(g, g.bonusCount.k + 1);
        trySkip(g);
    } else if (g.phase === 'match') {
        if (g.time >= g.match.until) afterMatch(g);
    } else if (g.phase === 'initials') {
        if (!tickInitials(g)) gameOver(g);
    }
    sync(g);
}

// ---------------------------------------------------------------- presentation sync (music section, base DMD scene)
function musicFor(g) {
    if (!g.started || g.phase === 'over') return 'attract';
    if (cur(g).tilt) return 'off';
    if (g.phase === 'bonus') return 'bonus';
    if (g.phase === 'match' || g.phase === 'initials') return 'hiscore';
    if (g.mb.state === 'wizard') return 'wizard';
    if (g.mb.state === 'running') return 'multiball';
    return g.mode.id ? 'mode' : 'main';
}
function sync(g) {
    const m = musicFor(g);
    if (m !== g.music) { g.music = m; music(g, m); }
    if (!live(g)) return;
    const want = g.mode.id ? 'modeStatus' : 'score';
    if (want !== g.base) { g.base = want; cmd(g, { type: 'dmdBase', scene: want, params: {} }); }
}

// ---------------------------------------------------------------- outputs
export function take(g) {
    const q = g.queue;
    g.queue = [];
    return q;
}

export function dmdState(g) {
    const p = cur(g), pk = pack(g), md = g.mode.id ? (pk.modes || []).find(m => m.id === g.mode.id) : null;
    if (md && live(g)) {
        return {
            scene: 'modeStatus',
            params: {
                name: md.name, hint: md.hint, remaining: Math.max(0, g.mode.until - g.time), total: md.duration ?? 40,
                count: g.mode.count, target: md.count ?? (md.targets || []).length, score: g.mode.total
            }
        };
    }
    return {
        scene: 'score',
        params: {
            players: g.players.map(x => ({ name: x.name, score: x.score })), turn: g.turn, ball: p.ball, ballsTotal: BALLS, training: g.training
        }
    };
}

export function toyState(g) {
    const hook = pack(g).hooks?.toy;
    return hook ? hook(g) : {};
}

// ---------------------------------------------------------------- training and testing helpers
export function resetTraining(g) {
    if (!g.training) return;
    cmd(g, { type: 'clearBalls' });
    Object.assign(g.balls, { active: 0, locked: 0, ids: [], lockedIds: [] });
    g.players[0] = newPlayer(g, g.players[0].name);
    g.turn = 0;
    startBall(g, false);
    sync(g);
}

export function debugBalls(g, ids) {
    g.balls.ids = [...ids];
    g.balls.active = ids.length;
    g.balls.nextId = Math.max(g.balls.nextId, ...ids.map(i => i + 1));
}

export { rankOf };
