// Rules framework, part 4: lamp modes derived from game state by the lamp conventions (plan 3.9). DOM-free.
import { LAMP_MODES } from '../contracts.js';
import { cur, pack, layoutOf } from './features.js';

const { off: OFF, on: ON, slow: SLOW, fast: FAST, pulse: PULSE } = LAMP_MODES;
const indexCache = new WeakMap();

// lamp id -> array index (lamp texel index is that plus one).
export function lampIndex(layout) {
    let map = indexCache.get(layout);
    if (!map) {
        map = new Map();
        (layout.lamps || []).forEach((l, i) => map.set(l.id, i));
        indexCache.set(layout, map);
    }
    return map;
}
export function lampPos(layout, ...ids) {
    const map = lampIndex(layout);
    for (const id of ids) if (map.has(id)) { const l = layout.lamps[map.get(id)]; return { x: l.x, y: l.y }; }
    return null;
}

const code = m => typeof m === 'number' ? m : (LAMP_MODES[m] ?? OFF);

// Writes one mode code per layout lamp: out[i] belongs to layout.lamps[i]. Returns out.
export function lampModes(g, out) {
    out.fill(OFF);
    const layout = layoutOf(g), pk = pack(g), p = cur(g);
    const map = lampIndex(layout);
    if (!p || p.tilt || g.phase === 'over' || !layout.lamps) return out;
    const set = (id, mode) => { const i = map.get(id); if (i !== undefined) out[i] = code(mode); };
    const t = g.time, mb = g.mb, mbOn = mb.state !== 'off';

    // ball save / extra ball
    const s = g.ballSave;
    if (pk.ballSave?.lamp) {
        let m = OFF;
        if (p.extraBalls > 0) m = ON;
        else if (s.unlimited) m = SLOW;
        else if (s.armed && (s.until === null || t <= s.until)) m = s.soft !== null && t > s.soft - 2 ? FAST : SLOW;
        set(pk.ballSave.lamp, m);
    }
    // bonus multiplier, lanes, kickback
    (pk.bonusX?.lamps || []).forEach((id, k) => set(id, p.bonusX >= k + 2 ? ON : OFF));
    (pk.topLanes?.lamps || []).forEach((id, i) => set(id, p.lanesTop[i] ? ON : (g.skill.live && g.skill.lane === i ? FAST : OFF)));
    (pk.lowerLanes?.lamps || []).forEach((id, i) => set(id, p.lanesLower[i] ? ON : OFF));
    if (pk.kickback?.lamp) set(pk.kickback.lamp, p.kickbackLit ? ON : OFF);

    // lock, extra ball, modes, wizard
    if (pk.extraBall) set(pk.extraBall.lamp || 'extraBall', p.ebLit ? SLOW : OFF);
    if (pk.lock) {
        set('lockLit', p.lockLit ? SLOW : OFF);
        for (let n = 1; n <= 3; n++) set('lock' + n, mbOn ? FAST : (p.locks >= n ? ON : OFF));
    }
    set(pk.modeStart?.lamp || 'modeStart', p.wizardReady ? FAST : (p.modeReady ? SLOW : OFF));
    const options = g.select ? g.select.options : [];
    for (const m of pk.modes || []) {
        let mode = p.modesCompleted.includes(m.id) ? ON : (p.modesPlayed.includes(m.id) ? PULSE : OFF);
        if (g.mode.id === m.id) mode = FAST;
        else if (options.includes(m.id)) mode = SLOW;
        set('mode_' + m.id, mode);
    }
    set('wizard', mb.state === 'wizard' ? FAST : (p.wizardReady ? SLOW : OFF));
    set('superJackpot', mb.superUntil > t ? FAST : OFF);

    // shot arrows (priority: jackpot > mode target > lock > extra ball > mode start > combo > off), jackpot and combo lamps
    const modeDef = g.mode.id ? (pk.modes || []).find(m => m.id === g.mode.id) : null;
    const superShot = mb.state === 'wizard' ? pk.wizard?.superShot : pk.multiball?.superShot;
    const comboLive = g.combo.count > 0 && t <= g.combo.until;
    const mbShots = new Set(mbOn ? (mb.litShots || []) : []);
    for (const shot of layout.shots || []) {
        let mode = OFF;
        const targeted = modeDef && modeTargets(g, modeDef).includes(shot.id);
        if (mbOn && (mbShots.has(shot.id) || (shot.id === superShot && mb.superUntil > t))) mode = FAST;
        else if (targeted) mode = modeDef.kind === 'roving' ? FAST : SLOW;
        else if (!mbOn && p.lockLit && pk.lock?.shot === shot.id) mode = ON;
        else if (p.ebLit && pk.extraBall?.shot === shot.id) mode = PULSE;
        else if ((p.modeReady || p.wizardReady) && !g.mode.id && !mbOn && pk.modeStart?.shot === shot.id) mode = SLOW;
        else if (shot.combo && comboLive && g.combo.last !== shot.id) mode = PULSE;
        set('arrow_' + shot.id, mode);
        set('jp_' + shot.id, mbShots.has(shot.id) ? FAST : OFF);
        set('combo_' + shot.id, shot.combo && comboLive && g.combo.last !== shot.id ? PULSE : OFF);
    }

    // pack overrides come last
    if (pk.hooks?.lamps) pk.hooks.lamps({ game: g, player: p, now: t, table: g._t }, set);
    return out;
}

// Targets of the running mode that currently light an arrow (all for count, the current one for roving/sequence).
export function modeTargets(g, m) {
    const shotIds = new Set((layoutOf(g).shots || []).map(s => s.id));
    const t = m.targets || [];
    const list = m.kind === 'count' ? t : [t[g.mode.target]].filter(x => x !== undefined);
    return list.filter(x => shotIds.has(x));
}
