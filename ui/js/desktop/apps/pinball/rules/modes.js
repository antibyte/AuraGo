// Rules framework, part 3: ctx, shot dispatch, modes and mode select, locks, multiball, jackpots, wizard. DOM-free.
import { matchEvent } from '../contracts.js';
import { rngNext } from '../config.js';
import {
    cur, pack, layoutOf, cmd, dmd, dmdClear, show, flash, gi, cue, say, shake, big, addBall, dropBall, armSave, ejectHole,
    addScore, addBonus, advanceBonusX, lightKickback, lightExtraBall, lightModeStart, comboHit
} from './features.js';
import { lampPos } from './lamps.js';

const SELECT_SECONDS = 4;
export const idleMode = () => ({ id: null, until: 0, count: 0, target: 0, moveAt: 0, total: 0 });
export const idleMb = () => ({ state: 'off', jackpots: 0, value: 0, litShots: [], superUntil: 0, addABallUsed: false });
const modeDef = (g, id) => (pack(g).modes || []).find(m => m.id === id);
const mbActive = g => g.mb.state !== 'off';

// ---------------------------------------------------------------- ctx handed to pack hooks
export function makeCtx(g) {
    return {
        game: g, player: cur(g), now: g.time, table: g._t,
        score: (n, o) => addScore(g, n, o),
        bonus: (id, n = 1) => addBonus(g, id, n),
        cmd: c => cmd(g, c),
        dmd: (scene, params, ms, prio) => dmd(g, scene, params, ms, prio),
        show: (name, o) => show(g, name, o),
        flash: (id, ms) => flash(g, id, ms),
        gi: (id, mode, ms) => gi(g, id, mode, ms),
        cue: name => cue(g, name),
        say: key => say(g, key),
        shake: s => shake(g, s),
        lightLock: () => lightLock(g),
        lightExtraBall: () => lightExtraBall(g),
        lightKickback: () => lightKickback(g),
        lightModeStart: () => lightModeStart(g),
        advanceBonusX: () => advanceBonusX(g),
        modeActive: () => !!g.mode.id,
        mbActive: () => mbActive(g),
        rng: () => rngNext(g)
    };
}

// ---------------------------------------------------------------- shot dispatch (framework step 7); returns true when the event's hole keeps its ball
export function dispatchShots(g, ev) {
    const pk = pack(g), shots = (layoutOf(g).shots || []).filter(s => matchEvent(s.on, ev));
    let held = false;
    for (const s of shots) {
        const v = pk.shotPoints?.[s.id];
        if (v) addScore(g, v);
        if (s.bonus) addBonus(g, s.bonus, 1);
        comboHit(g, s);
    }
    modeProgress(g, ev, shots);
    const ctx = makeCtx(g);
    for (const s of shots) {
        jackpotShot(g, s);
        if (lockShot(g, s, ev)) held = true;
        extraBallShot(g, s);
        if (modeStartShot(g, s, ev)) held = true;
        pk.hooks?.onShot?.(ctx, s.id);
    }
    return held;
}

// ---------------------------------------------------------------- modes
export function startMode(g, id) {
    const m = modeDef(g, id), p = cur(g);
    if (!m) return;
    p.modeReady = false;
    if (!p.modesPlayed.includes(id)) p.modesPlayed.push(id);
    const n = (m.targets || []).length;
    g.mode = { id, until: g.time + (m.duration ?? 40), count: 0, target: m.kind === 'roving' && n ? Math.floor(rngNext(g) * n) : 0, moveAt: g.time + (m.moveEvery ?? 4), total: 0 };
    g.select = null;
    dmd(g, 'modeStart', { id, name: m.name, hint: m.hint }, 2200, 5);
    cue(g, 'modeStart'); show(g, 'spiral', { ms: 1600 });
    if (m.show) show(g, m.show, { ms: 2000 });
    if (m.gi === 'dim') gi(g, 'all', 'dim', 0);
}

export function endMode(g, completed, quiet = false) {
    const id = g.mode.id, m = modeDef(g, id), p = cur(g);
    if (!m) return;
    if (completed) {
        const v = addScore(g, m.completeValue ?? 0);
        if (!p.modesCompleted.includes(id)) p.modesCompleted.push(id);
        addBonus(g, 'modes', 1);
        dmd(g, 'modeComplete', { name: m.name, score: v }, 2000, 5);
        cue(g, 'modeComplete'); show(g, 'radial', { ms: 1400 }); flash(g, 'all', 300);
    }
    if (!quiet) dmd(g, 'modeTotal', { score: g.mode.total }, 1800, 5);
    g.mode = idleMode();
    if (m.gi) gi(g, 'all', 'on', 0);
    const all = pack(g).modes || [];
    if (all.length && all.every(x => p.modesPlayed.includes(x.id)) && !p.wizardReady) {
        p.wizardReady = true;
        if (!quiet) big(g, 'wizard_lit', {}, 1800);
    }
}

function rove(g, m) {
    const n = (m.targets || []).length;
    if (n > 1) {
        let j = Math.floor(rngNext(g) * (n - 1));
        if (j >= g.mode.target) j++;
        g.mode.target = j;
    }
    g.mode.moveAt = g.time + (m.moveEvery ?? 4);
}

function modeProgress(g, ev, hits) {
    const m = g.mode.id && modeDef(g, g.mode.id);
    if (!m) return;
    const t = m.targets || [], ids = new Set((layoutOf(g).shots || []).map(s => s.id));
    const hit = x => x !== undefined && (hits.some(s => s.id === x) || (!ids.has(x) && matchEvent(x, ev)));
    const ok = m.kind === 'count' ? t.some(hit) : hit(t[g.mode.target]);
    if (!ok) return;
    g.mode.count++;
    if (m.kind === 'sequence') g.mode.target = (g.mode.target + 1) % t.length;
    else if (m.kind === 'roving') rove(g, m);
    addScore(g, (m.shotValue ?? 0) + (m.step ?? 0) * (g.mode.count - 1));
    const goal = m.count ?? t.length;
    if (g.mode.count >= goal) endMode(g, true);
    else big(g, 'to_go', { n: goal - g.mode.count }, 700, 3);
}

// ---------------------------------------------------------------- mode select
function selectScene(g) {
    const s = g.select;
    const played = cur(g).modesPlayed, done = cur(g).modesCompleted;
    const items = (pack(g).modes || []).map(m => ({ name: m.name, played: played.includes(m.id) && !s.options.includes(m.id), done: done.includes(m.id) }));
    dmd(g, 'select', { items, index: (pack(g).modes || []).findIndex(m => m.id === s.options[s.index]), remaining: Math.max(0, s.until - g.time), total: SELECT_SECONDS }, 0, 5);
}
function openSelect(g, hole) {
    const options = (pack(g).modes || []).filter(m => !cur(g).modesPlayed.includes(m.id)).map(m => m.id);
    if (!options.length) return false;
    if (options.length === 1) { startMode(g, options[0]); return false; }
    g.select = { options, index: 0, until: g.time + SELECT_SECONDS, hole: hole || null };
    selectScene(g);
    return !!hole;
}
export function cycleSelect(g, dir) {
    const s = g.select, n = s.options.length;
    s.index = (s.index + dir + n) % n;
    selectScene(g);
}
export function confirmSelect(g) {
    const s = g.select;
    dmdClear(g);
    startMode(g, s.options[s.index]);
    if (s.hole) ejectHole(g, s.hole);
}

function modeStartShot(g, shot, ev) {
    const ms = pack(g).modeStart, p = cur(g);
    if (!ms || shot.id !== ms.shot || mbActive(g) || g.mode.id || g.select) return false;
    if (p.wizardReady) { startWizard(g); return false; }
    if (!p.modeReady) return false;
    return openSelect(g, ms.hole && ev.type === 'hole' ? ev.id : null);
}

// ---------------------------------------------------------------- extra ball
function extraBallShot(g, shot) {
    const eb = pack(g).extraBall, p = cur(g);
    if (!eb || shot.id !== eb.shot || !p.ebLit) return;
    p.ebLit = false; p.extraBalls++;
    dmd(g, 'extraBall', { lit: false }, 2000, 5);
    cue(g, 'extraBall'); show(g, 'sparkle', { ms: 2000 }); say(g, 'extra_ball');
}

// ---------------------------------------------------------------- lock
export function lightLock(g) {
    const p = cur(g), L = pack(g).lock;
    if (!L || p.lockLit || mbActive(g) || p.locks >= 3) return;
    p.lockLit = true;
    if (L.diverter) cmd(g, { type: 'diverter', id: L.diverter.id, on: true });
    dmd(g, 'lock', { n: p.locks, lit: true }, 1500, 5);
    cue(g, 'lockLit');
}

function lockShot(g, shot, ev) {
    const L = pack(g).lock, p = cur(g), b = g.balls;
    if (!L || shot.id !== L.shot || !p.lockLit || mbActive(g)) return false;
    p.locks++; p.lockLit = false;
    addBonus(g, 'locks', 1);
    const physical = !!(L.physical && L.hole);
    if (physical) {
        const id = ev.ball ?? b.ids[0];
        if (dropBall(g, id)) b.lockedIds.push(id);
        b.locked = b.lockedIds.length;
        cmd(g, { type: 'lockBall', id: L.hole });
    }
    if (L.diverter) cmd(g, { type: 'diverter', id: L.diverter.id, on: false });
    dmd(g, 'lock', { n: p.locks, lit: false }, 1600, 5);
    cue(g, 'lock'); show(g, 'chase', { ms: 1000 });
    const ctx = makeCtx(g);
    pack(g).hooks?.onLock?.(ctx, p.locks);
    if (p.locks >= 3) { startMultiball(g, physical ? L.hole : null); return physical; }
    if (physical && b.ids.length === 0) addBall(g, true);
    return physical;
}

// ---------------------------------------------------------------- multiball and wizard
function startMultiball(g, hole) {
    const M = pack(g).multiball, p = cur(g), b = g.balls;
    p.locks = 0; p.lockLit = false;
    if (!M) return;
    g.mb = { state: 'running', jackpots: 0, value: M.jackpotBase ?? 1000000, litShots: [...(M.jackpotShots || [])], superUntil: 0, addABallUsed: false };
    armSave(g, M.ballSave ?? 10, g.time);
    releaseLocked(g, hole);
    for (let n = (M.balls ?? 3) - b.ids.length; n > 0; n--) addBall(g, true);
    dmd(g, 'multiball', { text: M.name }, 2500, 7);
    cue(g, 'multiball'); show(g, 'sweepUp', { ms: 2400, repeat: 3 }); shake(g, 0.003);
    pack(g).hooks?.onMultiball?.(makeCtx(g));
}
function releaseLocked(g, hole) {
    const b = g.balls;
    if (hole && b.lockedIds.length) cmd(g, { type: 'eject', id: hole, count: 'all', delay: 0.6 });
    b.ids.push(...b.lockedIds);
    b.lockedIds = []; b.locked = 0; b.active = b.ids.length;
}

export function startWizard(g) {
    const W = pack(g).wizard, p = cur(g), M = pack(g).multiball || {};
    if (!W) return;
    p.wizardReady = false;
    g.mb = { state: 'wizard', jackpots: 0, value: W.jackpotValue ?? 5000000, litShots: [...(M.jackpotShots || [])], superUntil: 0, addABallUsed: true };
    armSave(g, W.ballSave ?? 20, g.time);
    for (let n = (W.balls ?? 4) - g.balls.ids.length; n > 0; n--) addBall(g, true);
    dmd(g, 'wizardStart', { name: W.name }, 3000, 7);
    cue(g, 'wizard'); show(g, 'rainbow', { ms: 3000 }); g.loopAt = g.time + 3; shake(g, 0.003);
}

export function endMultiball(g) {
    const p = cur(g);
    if (g.mb.state === 'wizard') { p.modesPlayed = []; p.modesCompleted = []; p.wizardReady = false; }
    g.mb = idleMb();
    gi(g, 'all', 'on', 0);
}

function jackpotShot(g, shot) {
    const mb = g.mb;
    if (!mbActive(g)) return;
    const wiz = mb.state === 'wizard', M = pack(g).multiball || {}, W = pack(g).wizard || {};
    const superShot = wiz ? W.superShot : M.superShot, step = wiz ? (W.step ?? 1000000) : (M.jackpotStep ?? 250000);
    const at = lampPos(layoutOf(g), 'jp_' + shot.id, 'arrow_' + shot.id) || {};
    if (shot.id === superShot && mb.superUntil > g.time) {
        const v = addScore(g, mb.value * 5);
        mb.superUntil = 0; mb.jackpots = 0;
        dmd(g, 'superJackpot', { value: v, text: 'dmd_super_jackpot' }, 2400, 7);
        cue(g, 'superJackpot'); show(g, 'strobe', { ms: 800, priority: 3 }); show(g, 'radial', { ms: 1500, priority: 3, ...at }); shake(g, 0.002);
        return;
    }
    if (mb.litShots.includes(shot.id)) {
        const v = addScore(g, mb.value);
        mb.value += step; mb.jackpots++;
        mb.litShots = mb.litShots.filter(id => id !== shot.id);
        if (!mb.litShots.length) mb.litShots = [...(M.jackpotShots || [])];
        dmd(g, 'jackpot', { value: v, text: 'dmd_jackpot' }, 1800, 7);
        cue(g, 'jackpot'); show(g, 'radial', { ms: 1200, priority: 2, ...at }); flash(g, 'all', 500, { count: 2 }); shake(g, 0.002);
        if (mb.jackpots >= (wiz ? (W.superAfter ?? 6) : (M.superAfter ?? 4)) && mb.superUntil <= g.time) {
            mb.superUntil = g.time + (M.superWindow ?? 20);
            big(g, 'jackpots_lit', {}, 1200);
        }
        return;
    }
    if (!wiz && shot.id === M.addABallShot && !mb.addABallUsed) {
        mb.addABallUsed = true;
        addBall(g, true);
        big(g, 'add_a_ball', {}, 1500);
    }
}

// ---------------------------------------------------------------- timers
export function tickModes(g) {
    pack(g).hooks?.onTick?.(makeCtx(g));
    if (g.select && g.time >= g.select.until) confirmSelect(g);
    if (g.mode.id) {
        const m = modeDef(g, g.mode.id);
        if (g.time >= g.mode.until) endMode(g, false);
        else if (m.kind === 'roving' && g.time >= g.mode.moveAt) rove(g, m);
    }
    if (g.mb.superUntil && g.time > g.mb.superUntil) { g.mb.superUntil = 0; g.mb.jackpots = 0; }
    if (g.mb.state === 'wizard' && g.time >= g.loopAt) { show(g, 'rainbow', { ms: 3000 }); g.loopAt = g.time + 3; }
}

