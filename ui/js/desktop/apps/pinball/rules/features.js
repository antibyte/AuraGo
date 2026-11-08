// Rules framework, part 2: shared emitters, scoring, bonus, lanes, skill shot, ball save, kickback, combos. DOM-free.
import { parsePattern, matchEvent } from '../contracts.js';

export const GRACE = 1.5;
export const HOLD = 0.9;
export const HOLD_BIG = 1.8;

export const cur = g => g.players[g.turn];
export const pack = g => g._t.rules;
export const layoutOf = g => g._t.layout;
const round10 = n => Math.round(n / 10) * 10;
const LANE_EVENTS = new Set(['rollover', 'trigger']);
const NO_DEFAULT = new Set(['drain', 'bankReset', 'pathFail', 'pathSwitch', 'searchFail', 'kickback']);

// ---------------------------------------------------------------- emitters (everything goes through game.queue)
export function cmd(g, c) { g.queue.push(c); return c; }
export function dmd(g, scene, params = {}, ms = 1500, priority = 5, transition) {
    if (priority >= 5) g.lastBig = g.time;
    const c = { type: 'dmd', scene, params, ms, priority };
    if (transition) c.transition = transition;
    return cmd(g, c);
}
export const show = (g, name, o = {}) => cmd(g, { type: 'show', name, ms: 1000, priority: 1, ...o });
export const flash = (g, id, ms = 250, extra = {}) => cmd(g, { type: 'flash', id, ms, ...extra });
export const gi = (g, id, mode, ms = 0) => cmd(g, { type: 'gi', id, mode, ms });
export const cue = (g, name) => cmd(g, { type: 'cue', name });
export const say = (g, key) => cmd(g, { type: 'say', key });
export const shake = (g, strength) => cmd(g, { type: 'shake', strength });
export const music = (g, section) => cmd(g, { type: 'music', section });
export const big = (g, key, vars = {}, ms = 1400, priority = 7) => dmd(g, 'big', { text: 'dmd_' + key, vars }, ms, priority);
// Live scenes (select, bonus, initials) run until cleared: this is {type:'dmd', scene:null}.
export const dmdClear = g => cmd(g, { type: 'dmd', scene: null });

export function holdTime(g) { return g.lastBig === g.time ? HOLD_BIG : HOLD; }
export function ejectHole(g, id) { cmd(g, { type: 'eject', id, count: 1, delay: holdTime(g) }); }

// ---------------------------------------------------------------- balls and ball save
export function addBall(g, auto = false) {
    const id = g.balls.nextId++;
    g.balls.ids.push(id);
    g.balls.active = g.balls.ids.length;
    cmd(g, { type: 'addBall', id, auto });
    return id;
}
export function dropBall(g, id) {
    const ids = g.balls.ids;
    const i = id === undefined || id === null ? 0 : ids.indexOf(id);
    if (i < 0 || !ids.length) return false;
    ids.splice(i, 1);
    g.balls.active = ids.length;
    return true;
}
// from = null: the timer has not started yet (it starts at the first non-shooter switch).
export function armSave(g, seconds, from = null) {
    const s = g.ballSave;
    s.armed = true; s.seconds = seconds;
    s.soft = from === null ? null : from + seconds;
    s.until = from === null ? null : from + seconds + GRACE;
}
export function saveActive(g) {
    const s = g.ballSave;
    return s.unlimited || (s.armed && (s.until === null || g.time <= s.until));
}
function startSaveTimer(g, ev) {
    const s = g.ballSave;
    if (!s.armed || s.until !== null || s.unlimited || ev.type === 'drain' || ev.type === 'gate') return;
    armSave(g, s.seconds, g.time);
}

// ---------------------------------------------------------------- scoring and bonus
export function addScore(g, n, o = {}) {
    const p = cur(g);
    if (p.tilt || !(n > 0)) return 0;
    n = round10(n);
    p.score += n;
    if (g.mode.id) g.mode.total += n;
    if (typeof o.bonus === 'string') addBonus(g, o.bonus, 1);
    return n;
}
export function addBonus(g, id, n = 1) {
    const c = cur(g).counters;
    c[id] = (c[id] || 0) + n;
}
export function bonusLines(g) {
    const p = cur(g);
    return (pack(g).bonusCounters || []).filter(c => (p.counters[c.id] || 0) > 0)
        .map(c => ({ id: c.id, key: c.key, n: p.counters[c.id], value: c.value }));
}
export function bonusSum(g) { return bonusLines(g).reduce((s, l) => s + l.n * l.value, 0); }

function pointsFor(pts, ev) {
    let best = null, bestSpec = -1;
    for (const key of Object.keys(pts)) {
        if (key === 'default') continue;
        const p = parsePattern(key);
        if (!p || !matchEvent(p, ev)) continue;
        const spec = (p.id === '*' ? 0 : 2) + (p.dir !== null ? 1 : 0);
        if (spec > bestSpec) { best = key; bestSpec = spec; }
    }
    if (best !== null) return pts[best];
    return NO_DEFAULT.has(ev.type) ? 0 : (pts.default ?? 1010);
}
export function awardPoints(g, ev) { addScore(g, pointsFor(pack(g).points || {}, ev)); }
export function bonusFrom(g, ev) {
    for (const [pattern, id] of Object.entries(pack(g).bonusFrom || {})) if (matchEvent(pattern, ev)) addBonus(g, id, 1);
}

export function advanceBonusX(g) {
    const p = cur(g), max = pack(g).bonusX?.max ?? 5;
    if (p.bonusX >= max) return false;
    p.bonusX++;
    big(g, 'bonus_x', { n: p.bonusX }, 1200, 7);
    return true;
}

// ---------------------------------------------------------------- lights for awards
export function lightKickback(g) {
    const k = pack(g).kickback;
    if (!k) return;
    cur(g).kickbackLit = true;
    cmd(g, { type: 'kickback', id: k.device, armed: true });
    big(g, 'kickback_lit', {}, 1300);
}
export function lightExtraBall(g) {
    const p = cur(g);
    if (p.ebLit || !pack(g).extraBall) return;
    p.ebLit = true;
    dmd(g, 'extraBall', { lit: true }, 1500, 5);
}
export function lightModeStart(g) { cur(g).modeReady = true; }

// ---------------------------------------------------------------- lanes
function laneSets(g) {
    const pk = pack(g), p = cur(g);
    return [['lanesTop', pk.topLanes], ['lanesLower', pk.lowerLanes]].filter(([, l]) => l && l.switches?.length).map(([key, l]) => [p[key], l]);
}
export function laneHit(g, ev) {
    if (!LANE_EVENTS.has(ev.type)) return;
    for (const [lit, def] of laneSets(g)) {
        const i = def.switches.indexOf(ev.id);
        if (i < 0 || lit[i]) continue;
        lit[i] = true;
        if (!lit.every(Boolean)) continue;
        lit.fill(false);
        if (def.award === 'bonusX') advanceBonusX(g);
        else if (def.award === 'kickback') lightKickback(g);
        else if (def.award === 'extraBall') lightExtraBall(g);
        show(g, 'sweepUp', { ms: 700 });
    }
}
export function rotateLanes(g, dir) {
    for (const [lit] of laneSets(g)) {
        if (dir > 0) lit.unshift(lit.pop()); else lit.push(lit.shift());
    }
}

// ---------------------------------------------------------------- skill shot
export function skillCheck(g, ev) {
    const sk = g.skill, pk = pack(g), sup = pk.skill?.super;
    if (sup && sk.superUntil > g.time && matchEvent(sup.on, ev)) {
        sk.superUntil = 0; sk.live = false;
        const v = addScore(g, sup.value ?? 1000000);
        dmd(g, 'skillShot', { value: v, super: true }, 1800, 5);
        cue(g, 'skillShot'); show(g, 'ring', { ms: 1200 }); say(g, 'super_skill_shot');
        return;
    }
    if (!sk.live) return;
    const lanes = pk.topLanes?.switches || [];
    const i = LANE_EVENTS.has(ev.type) ? lanes.indexOf(ev.id) : -1;
    if (i >= 0) {
        if (i === sk.lane) {
            sk.live = false;
            const v = addScore(g, pk.skill?.value ?? 250000);
            dmd(g, 'skillShot', { value: v, super: false }, 1500, 5);
            cue(g, 'skillShot'); show(g, 'ring', { ms: 1000 });
        }
        return;
    }
    if (ev.type !== 'bumper' && ev.type !== 'gate' && ev.type !== 'shooter') sk.live = false;
}

// ---------------------------------------------------------------- combos
export function comboHit(g, shot) {
    if (!shot.combo) return;
    const cb = g.combo, w = pack(g).combos?.window ?? 3;
    const chained = cb.count > 0 && g.time <= cb.until;
    if (!chained) cb.count = 1;
    else if (shot.id !== cb.last) cb.count++;
    cb.last = shot.id; cb.until = g.time + w;
    if (cb.count < 2) return;
    const v = addScore(g, (pack(g).combos?.base ?? 100000) * (cb.count - 1));
    addBonus(g, 'combos', 1);
    dmd(g, 'combo', { n: cb.count, value: v }, 1100, 3);
    cue(g, 'combo');
}

// ---------------------------------------------------------------- the per-event feature pipeline (steps 2-5)
export function featureStep(g, ev, scoring) {
    if (!scoring) return;
    startSaveTimer(g, ev);
    awardPoints(g, ev);
    bonusFrom(g, ev);
    laneHit(g, ev);
    skillCheck(g, ev);
}
