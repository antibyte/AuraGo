// Table validator: ids, references, bounds, lamp conventions, i18n keys and collider clearance. DOM-free.
import { EVENT_TYPES, PHYSICS_COMMANDS, parsePattern } from '../contracts.js';
import { FIELD } from '../physics/constants.js';
import { distPointSeg, distSegSeg } from './kit/geom.js';

const MAX_LAMPS = 200, MAX_FLASHERS = 6;
const TOUCH = 0.002, CLEAR = 0.030;
const KEY_RE = /^(dmd|mode|lamp|card|help)_[a-z0-9_]+$/;
const EPS = 1e-6;
// Commands whose id names a ball rather than a table device.
const BALL_COMMANDS = new Set(['addBall', 'lockBall', 'releaseToTrough', 'removeBall']);
const DEVICE_COMMANDS = new Set(['eject', 'holeEnable', 'resetBank', 'dropBank', 'diverter', 'gate', 'door', 'kickback', 'magnet', 'kick']);

const list = value => Array.isArray(value) ? value : [];

export function validateTable(table) {
    const errors = [], warnings = [], keys = new Set();
    const err = (code, msg) => errors.push(code + ': ' + msg);
    const layout = table?.layout, rules = table?.rules || {};
    if (!layout) return { ok: false, errors: ['missing-layout: table.layout is required'], warnings, keys: [] };

    const ids = collectIds(layout, err);
    checkBounds(layout, err);
    checkCounts(layout, err);
    checkReferences(layout, rules, ids, err);
    checkConventions(layout, rules, ids, err);
    collectKeys(layout, rules, keys);
    checkClearance(layout, err);
    return { ok: errors.length === 0, errors, warnings, keys: [...keys].sort() };
}

// ---------------------------------------------------------------- ids
function collectIds(layout, err) {
    const spaces = {
        flipper: layout.flippers, geometry: [...list(layout.walls), ...list(layout.posts)],
        path: layout.paths, shot: layout.shots, lamp: layout.lamps, flasher: layout.flashers, gi: layout.gi, toy: layout.toys
    };
    const deviceItems = [];
    for (const d of list(layout.devices)) {
        deviceItems.push(d);
        for (const t of list(d.targets)) deviceItems.push(t);
    }
    spaces.device = deviceItems;
    const sets = {};
    for (const [name, items] of Object.entries(spaces)) {
        sets[name] = new Set();
        for (const item of list(items)) {
            if (typeof item?.id !== 'string' || !item.id) { err('missing-id', name + ' entry without an id'); continue; }
            if (sets[name].has(item.id)) err('duplicate-id', name + ' "' + item.id + '"');
            sets[name].add(item.id);
        }
    }
    const byType = type => new Set(list(layout.devices).filter(d => d.type === type).map(d => d.id));
    const events = new Set([...sets.device, ...sets.path, ...sets.flipper]);
    for (const p of list(layout.paths)) for (const s of list(p.switches)) if (s?.id) events.add(s.id);
    for (const d of list(layout.devices)) if (d.endSwitch) events.add(d.endSwitch);
    return { ...sets, holes: byType('hole'), diverters: byType('diverter'), events };
}

// ---------------------------------------------------------------- bounds and counts
function inField(x, y) {
    return Number.isFinite(x) && Number.isFinite(y) && x >= FIELD.xMin - EPS && x <= FIELD.xMax + EPS && y >= FIELD.yMin - EPS && y <= FIELD.yMax + EPS;
}

function checkBounds(layout, err) {
    const at = (what, id, x, y) => { if (!inField(x, y)) err('out-of-bounds', what + ' "' + id + '" at (' + x + ', ' + y + ')'); };
    for (const w of list(layout.walls)) {
        if (!Array.isArray(w.points) || w.points.length < 2) err('short-wall', 'wall "' + w.id + '" needs at least 2 points');
        for (const p of list(w.points)) at('wall', w.id, p[0], p[1]);
    }
    for (const p of list(layout.posts)) at('post', p.id, p.x, p.y);
    for (const f of list(layout.flippers)) at('flipper', f.id, f.x, f.y);
    for (const l of list(layout.lamps)) at('lamp', l.id, l.x, l.y);
    for (const f of list(layout.flashers)) at('flasher', f.id, f.x, f.y);
    for (const t of list(layout.toys)) at('toy', t.id, t.x, t.y);
    for (const l of list(layout.labels)) at('label', l.key, l.x, l.y);
    for (const g of list(layout.gi)) for (const b of list(g.bulbs)) at('gi', g.id, b[0], b[1]);
    for (const d of list(layout.devices)) {
        if (typeof d.x === 'number') at(d.type, d.id, d.x, d.y);
        if (d.a) at(d.type, d.id, d.a[0], d.a[1]);
        if (d.b) at(d.type, d.id, d.b[0], d.b[1]);
        if (d.at) at(d.type, d.id, d.at[0], d.at[1]);
        for (const t of list(d.targets)) at('target', t.id, t.x, t.y);
    }
    for (const p of list(layout.paths)) {
        if (!Array.isArray(p.points) || p.points.length < 2) err('short-path', 'path "' + p.id + '" needs at least 2 points');
        for (const q of list(p.points)) at('path', p.id, q[0], q[1]);
    }
}

function checkCounts(layout, err) {
    if (list(layout.lamps).length > MAX_LAMPS) err('too-many-lamps', list(layout.lamps).length + ' lamps (max ' + MAX_LAMPS + ')');
    if (list(layout.flashers).length > MAX_FLASHERS) err('too-many-flashers', list(layout.flashers).length + ' flashers (max ' + MAX_FLASHERS + ')');
}

// ---------------------------------------------------------------- references
function checkReferences(layout, rules, ids, err) {
    const ref = (what, set, id) => { if (!set.has(id)) err('dangling-ref', what + ' -> "' + id + '"'); };
    const exitRef = (owner, exit, depth = 0) => {
        if (!exit || depth > 3) return;
        if (exit.kind === 'hole') ref(owner + ' exit hole', ids.holes, exit.hole);
        else if (exit.kind === 'path') ref(owner + ' exit path', ids.path, exit.path);
        else if (exit.kind === 'switch') {
            ref(owner + ' exit diverter', ids.diverters, exit.diverter);
            exitRef(owner, exit.on, depth + 1); exitRef(owner, exit.off, depth + 1);
        }
    };
    for (const p of list(layout.paths)) exitRef('path "' + p.id + '"', p.exit);
    for (const d of list(layout.devices)) {
        if (d.type === 'sling') ref('sling "' + d.id + '" face', new Set(list(layout.walls).map(w => w.id)), d.face);
        if (d.type === 'hole' && d.eject?.path) ref('hole "' + d.id + '" eject path', ids.path, d.eject.path);
        if (d.type === 'captive' && d.endSwitch === undefined) err('dangling-ref', 'captive "' + d.id + '" has no endSwitch');
    }
    for (const l of list(layout.lamps)) if (l.device) ref('lamp "' + l.id + '" device', ids.device, l.device);
    const shotIds = new Set(list(layout.shots).map(s => s.id));
    for (const s of list(layout.shots)) {
        patternRef('shot "' + s.id + '"', s.on, ids, err);
        for (const cmd of list(s.setup)) {
            if (!PHYSICS_COMMANDS.includes(cmd?.type)) { err('bad-command', 'shot "' + s.id + '" setup ' + cmd?.type); continue; }
            if (DEVICE_COMMANDS.has(cmd.type) && cmd.id !== undefined) ref('shot "' + s.id + '" setup ' + cmd.type, ids.device, cmd.id);
        }
    }
    checkPack(layout, rules, ids, shotIds, ref, err);
}

function patternRef(owner, pattern, ids, err) {
    const p = parsePattern(pattern);
    if (!p) { err('bad-pattern', owner + ' pattern "' + pattern + '"'); return; }
    if (!EVENT_TYPES.includes(p.type)) err('bad-pattern', owner + ' unknown event type "' + p.type + '"');
    else if (p.id !== '*' && !ids.events.has(p.id)) err('dangling-ref', owner + ' pattern id "' + p.id + '"');
}

function checkPack(layout, rules, ids, shotIds, ref, err) {
    const lampIds = ids.lamp;
    const shot = (what, id) => { if (id !== undefined && id !== null) ref(what, shotIds, id); };
    const lamp = (what, id) => { if (id !== undefined && id !== null) ref(what, lampIds, id); };
    for (const key of Object.keys(rules.points || {})) if (key !== 'default') patternRef('points', key, ids, err);
    for (const id of Object.keys(rules.shotPoints || {})) shot('shotPoints', id);
    const counters = new Set(list(rules.bonusCounters).map(c => c.id));
    for (const [pattern, id] of Object.entries(rules.bonusFrom || {})) { patternRef('bonusFrom', pattern, ids, err); ref('bonusFrom "' + pattern + '"', counters, id); }
    for (const name of ['topLanes', 'lowerLanes']) {
        const lanes = rules[name];
        if (!lanes) continue;
        for (const id of list(lanes.switches)) ref(name + ' switch', ids.device, id);
        for (const id of list(lanes.lamps)) lamp(name + ' lamp', id);
    }
    for (const id of list(rules.bonusX?.lamps)) lamp('bonusX lamp', id);
    lamp('ballSave lamp', rules.ballSave?.lamp);
    if (rules.kickback) { ref('kickback device', ids.device, rules.kickback.device); lamp('kickback lamp', rules.kickback.lamp); }
    if (rules.extraBall) { shot('extraBall shot', rules.extraBall.shot); lamp('extraBall lamp', rules.extraBall.lamp); }
    if (rules.lock) {
        shot('lock shot', rules.lock.shot);
        if (rules.lock.hole) ref('lock hole', ids.holes, rules.lock.hole);
        if (rules.lock.diverter) { ref('lock diverter', ids.diverters, rules.lock.diverter.id); shot('lock diverter shot', rules.lock.diverter.shot); }
    }
    const mb = rules.multiball;
    if (mb) { for (const id of list(mb.jackpotShots)) shot('jackpot shot', id); shot('superShot', mb.superShot); shot('addABallShot', mb.addABallShot); }
    if (rules.modeStart) { shot('modeStart shot', rules.modeStart.shot); if (rules.modeStart.hole) ref('modeStart hole', ids.holes, rules.modeStart.hole); lamp('modeStart lamp', rules.modeStart.lamp); }
    const modes = list(rules.modes);
    if (modes.length && modes.length !== 4) err('bad-modes', 'a pack declares exactly 4 modes (found ' + modes.length + ')');
    const modeIds = new Set();
    for (const m of modes) {
        if (modeIds.has(m.id)) err('duplicate-id', 'mode "' + m.id + '"');
        modeIds.add(m.id);
        for (const t of list(m.targets)) if (!shotIds.has(t)) patternRef('mode "' + m.id + '" target', t, ids, err);
    }
    if (rules.wizard) shot('wizard superShot', rules.wizard.superShot);
}

// ---------------------------------------------------------------- lamp conventions
function checkConventions(layout, rules, ids, err) {
    const need = (id, why) => { if (!ids.lamp.has(id)) err('missing-lamp', '"' + id + '" (' + why + ')'); };
    need('shootAgain', 'ball save'); ['bonus2x', 'bonus3x', 'bonus4x', 'bonus5x'].forEach(id => need(id, 'bonus multiplier'));
    for (const d of list(layout.devices)) if (d.type === 'kickback') need(d.id, 'kickback');
    const shots = list(layout.shots), modes = list(rules.modes);
    for (const s of shots) {
        need('arrow_' + s.id, 'shot arrow');
        if (s.combo) need('combo_' + s.id, 'combo shot');
    }
    for (const id of list(rules.multiball?.jackpotShots)) need('jp_' + id, 'jackpot shot');
    if (!shots.length && !modes.length) return;
    if (rules.extraBall) need('extraBall', 'extra ball');
    if (rules.lock) { need('lockLit', 'lock'); [1, 2, 3].forEach(n => need('lock' + n, 'lock')); }
    if (rules.modeStart || modes.length) need('modeStart', 'mode start');
    for (const m of modes) need('mode_' + m.id, 'mode');
    if (rules.wizard) need('wizard', 'wizard');
    if (rules.multiball) need('superJackpot', 'multiball');
}

// ---------------------------------------------------------------- i18n keys
function collectKeys(layout, rules, keys) {
    const add = value => { if (typeof value === 'string' && KEY_RE.test(value)) keys.add(value); };
    for (const l of list(layout.lamps)) add(l.label);
    for (const l of list(layout.labels)) add(l.key);
    for (const c of list(rules.bonusCounters)) add(c.key);
    for (const m of list(rules.modes)) { add(m.name); add(m.hint); }
    add(rules.multiball?.name); add(rules.wizard?.name);
    add(layout.meta?.title);
}

// ---------------------------------------------------------------- clearance
function colliders(layout) {
    const out = [];
    for (const w of list(layout.walls)) {
        const pts = w.points || [], n = w.closed ? pts.length : pts.length - 1;
        for (let i = 0; i < n; i++) out.push({ kind: 'seg', id: w.id, group: w.id, a: pts[i], b: pts[(i + 1) % pts.length] });
    }
    for (const p of list(layout.posts)) out.push({ kind: 'circle', id: p.id, group: p.id, x: p.x, y: p.y, r: p.r });
    const face = (id, group, x, y, angle, width) => {
        const tx = -Math.sin(angle) * width / 2, ty = Math.cos(angle) * width / 2;
        out.push({ kind: 'seg', id, group, a: [x - tx, y - ty], b: [x + tx, y + ty] });
    };
    for (const d of list(layout.devices)) {
        if (d.type === 'bumper') out.push({ kind: 'circle', id: d.id, group: d.id, x: d.x, y: d.y, r: d.r ?? 0.0215 });
        else if (d.type === 'standup') face(d.id, d.id, d.x, d.y, d.angle || 0, d.width ?? 0.020);
        else if (d.type === 'dropBank') for (const t of list(d.targets)) face(t.id, d.id, t.x, t.y, d.angle || 0, d.width ?? 0.024);
        else if (d.type === 'door' && !d.open) out.push({ kind: 'seg', id: d.id, group: d.id, a: d.a, b: d.b });
    }
    return out;
}

function dist(p, q) {
    if (p.kind === 'seg' && q.kind === 'seg') return distSegSeg(p.a, p.b, q.a, q.b);
    if (p.kind === 'circle' && q.kind === 'circle') return Math.hypot(p.x - q.x, p.y - q.y) - p.r - q.r;
    const [s, c] = p.kind === 'seg' ? [p, q] : [q, p];
    return distPointSeg(c.x, c.y, s.a[0], s.a[1], s.b[0], s.b[1]) - c.r;
}

// Gates, triggers, rollovers, holes and paths are not solid, so they take no part in the check.
function checkClearance(layout, err) {
    const cs = colliders(layout), n = cs.length;
    const allowed = new Set();
    for (const [a, b] of list(layout.allowGaps)) { allowed.add(a + '|' + b); allowed.add(b + '|' + a); }
    const d = Array.from({ length: n }, () => new Float64Array(n));
    const touch = Array.from({ length: n }, () => new Set());
    for (let i = 0; i < n; i++) for (let j = i + 1; j < n; j++) {
        const v = dist(cs[i], cs[j]);
        d[i][j] = d[j][i] = v;
        if (v <= TOUCH) { touch[i].add(j); touch[j].add(i); }
    }
    const len = cs.map(c => c.kind === 'seg' ? Math.hypot(c.b[0] - c.a[0], c.b[1] - c.a[1]) : 2 * c.r);
    const worst = new Map();
    for (let i = 0; i < n; i++) {
        const chain = chainLengths(i, touch, len);
        for (let j = i + 1; j < n; j++) {
            const v = d[i][j];
            if (v <= TOUCH || v >= CLEAR || cs[i].group === cs[j].group) continue;
            // Colliders joined within 30 mm of chain length form one joint (short arc segments, corners).
            if (chain[j] <= CLEAR) continue;
            const a = cs[i].group, b = cs[j].group, key = a < b ? a + '|' + b : b + '|' + a;
            if (allowed.has(key)) continue;
            if (!worst.has(key) || worst.get(key) > v) worst.set(key, v);
        }
    }
    for (const [key, v] of worst) err('clearance', key.replace('|', ' / ') + ' are ' + (v * 1000).toFixed(1) + ' mm apart (allowed: touching or at least 30 mm)');
}

// Shortest chain length from collider src to every other collider over the touching graph;
// each intermediate collider adds its own length.
function chainLengths(src, touch, len) {
    const n = touch.length, dist = new Float64Array(n).fill(Infinity), done = new Uint8Array(n);
    dist[src] = 0;
    for (;;) {
        let u = -1;
        for (let k = 0; k < n; k++) if (!done[k] && dist[k] < Infinity && (u < 0 || dist[k] < dist[u])) u = k;
        if (u < 0) return dist;
        done[u] = 1;
        const step = u === src ? 0 : len[u];
        for (const v of touch[u]) if (dist[u] + step < dist[v]) dist[v] = dist[u] + step;
    }
}
