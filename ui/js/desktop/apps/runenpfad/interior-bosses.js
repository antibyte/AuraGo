/* Runenpfad guardians: six room bosses as data plus a small move interpreter (lunge, hop and area slams, shot fan and ring,
   summon, blink, charge, steam cross, web patch, sweeping beam). DOM-free; the mob state it keeps is plain data for the renderer.

   Renderer-facing mob state (all positions in room pixels; "centre" = body centre, the same space as it.shots; "feet" = m.x, m.y):
     m.boss true | m.kind | m.state 'move'|'tell'|'attack'|'stun'|'dead' | m.move move id while telling or attacking, else null
     m.phase 1, or 2 below half hp | m.t seconds in the state | m.tellDur length of the current tell | m.ax, m.ay aim (unit)
     m.zones [{x, y, r, at, done, kind: 'hop'|'slam'|'web'}] (centre; shown in tell and attack; done once it has landed)
     m.line {x0, y0, x1, y1} (centre; path of a charge or lunge) | m.cross {x, y, half, x0, x1, y0, y1} (steam bands, centre)
     m.beam {x, y, a, a0, a1, len} (centre; a = current angle) | m.blink {x, y} (feet; where it will appear)
     m.patches [{x, y, r, left}] (centre; web patches on the floor) | m.hurtT flash timer | m.deadT death pose timer. */
(function (root) {
    'use strict';

    const CELL = 48;
    const WAKE = 220;
    const HERO_R = 9;
    const MIN_TELL = 0.65;
    const STUN = 1;

    // keep = [nearest, farthest] distance it likes to hold; cd = pause after a move; fast = quicker below half hp.
    const SPECS = {
        brood: {
            speed: 40, keep: [0, 50], cd: 1.8, moves: [
                { id: 'slam', type: 'hop', tell: 0.95, atk: 0.35, rad: 46, max: 300, lock: 0.3 },
                { id: 'spawn', type: 'summon', tell: 1.0, atk: 0.3, add: 'slime', n: 3, cap: 3, max: 9999 }
            ], phase: { id: 'split', type: 'summon', tell: 1.0, atk: 0.3, add: 'slime', n: 2, cap: 5, max: 9999 }
        },
        weaver: {
            speed: 70, keep: [130, 220], cd: 1.5, moves: [
                { id: 'venom', type: 'fan', tell: 0.8, atk: 0.25, n: 3, spread: 0.3, speed: 150, r: 5, life: 2.6, shot: 'weaver_venom', max: 440 },
                { id: 'lunge', type: 'lunge', tell: 0.75, atk: 0.35, speed: 340, max: 280, lock: 0.4 },
                { id: 'web', type: 'web', tell: 0.9, atk: 0.25, rad: 46, life: 6, max: 440, lock: 0.4 }
            ]
        },
        tombward: {
            speed: 45, keep: [0, 48], cd: 1.7, moves: [
                { id: 'slam', type: 'slam', tell: 0.95, atk: 0.3, reach: 56, rad: 46, max: 110, lock: 0.3 },
                { id: 'march', type: 'march', tell: 0.95, atk: 0.6, n: 3, first: 64, step: 54, rad: 32, max: 300, lock: 0.3 }
            ], phase: { id: 'raise', type: 'summon', tell: 1.0, atk: 0.3, add: 'mummy', n: 2, cap: 2, max: 9999 }
        },
        frostheart: {
            speed: 45, keep: [60, 120], cd: 1.6, moves: [
                { id: 'ring', type: 'ring', tell: 0.8, atk: 0.25, n: 8, speed: 120, r: 5, life: 2.4, shot: 'frostheart_shard', max: 440 },
                { id: 'triple', type: 'fan', tell: 0.7, atk: 0.25, n: 3, spread: 0.26, speed: 180, r: 5, life: 2.2, shot: 'frostheart_bolt', max: 440 },
                { id: 'blink', type: 'blink', tell: 0.7, atk: 0.2, max: 170 }
            ]
        },
        boilerward: {
            speed: 50, keep: [0, 52], cd: 1.6, moves: [
                { id: 'charge', type: 'charge', tell: 0.9, atk: 2.0, speed: 300, min: 80, max: 400, lock: 0.45 },
                { id: 'cross', type: 'cross', tell: 0.9, atk: 0.5, half: 22, max: 9999 }
            ]
        },
        counter: {
            speed: 45, keep: [0, 64], cd: 2.2, fast: true, moves: [
                { id: 'clock', type: 'clock', tell: 0.9, atk: 0.3, slots: 14, speed: 120, r: 5, life: 2.6, shot: 'counter_clock', max: 9999 },
                { id: 'beam', type: 'beam', tell: 1.0, atk: 1.2, sweep: Math.PI / 2, emit: 0.1, speeds: [130, 180, 230], r: 5, life: 2.2, len: 340, shot: 'counter_beam', max: 440, lock: 0.45 },
                { id: 'summon', type: 'summon', tell: 1.0, atk: 0.3, add: 'clockbug', n: 2, cap: 2, max: 9999 }
            ]
        }
    };

    function P() { return root.RunenpfadPhysics; }
    function I() { return root.RunenpfadInterior; }
    function M() { return root.RunenpfadInteriorMobs; }
    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }

    let scratch = null;

    // room.solid plus the exit cells: a guardian never walks onto an exit.
    function gridOf(it) {
        const room = it.room;
        const n = room.cols * room.rows;
        if (!scratch || scratch.length !== n) scratch = new Uint8Array(n);
        scratch.set(room.solid);
        room.exits.forEach(function (e) {
            if (e.cx >= 0 && e.cy >= 0 && e.cx < room.cols && e.cy < room.rows) scratch[e.cy * room.cols + e.cx] = 1;
        });
        return scratch;
    }

    function heroC(it) { return { x: it.hero.x, y: it.hero.y - 7 }; }

    function gateBetween(it, a, b) {
        const gates = (it.things || []).filter(function (t) { return t.kind === 'gate' && !t.open; });
        if (!gates.length) return false;
        const n = Math.max(1, Math.ceil(Math.hypot(b.x - a.x, b.y - a.y) / 12));
        for (let i = 0; i <= n; i++) {
            const cx = Math.floor((a.x + (b.x - a.x) * i / n) / CELL);
            const cy = Math.floor((a.y + (b.y - a.y) * i / n) / CELL);
            if (gates.some(function (g) { return g.cx === cx && g.cy === cy; })) return true;
        }
        return false;
    }

    function aim(m, it) {
        const h = heroC(it);
        const dx = h.x - m.x;
        const dy = h.y - (m.y - 7);
        const d = Math.hypot(dx, dy) || 1;
        m.ax = dx / d;
        m.ay = dy / d;
    }

    function bmove(it, grid, m, dx, dy) {
        return M().slide(it.room, grid, m, dx, dy, m.r * 0.8, m.r * 1.1);
    }

    // Length of a ray from the centre along (ax, ay) up to the first solid cell (capped).
    function rayEnd(it, grid, cx, cy, ax, ay, cap) {
        const room = it.room;
        let d = 0;
        while (d < cap) {
            const x = Math.floor((cx + ax * (d + 6)) / CELL);
            const y = Math.floor((cy + ay * (d + 6)) / CELL);
            if (x < 0 || y < 0 || x >= room.cols || y >= room.rows || grid[y * room.cols + x]) break;
            d += 6;
        }
        return { x: cx + ax * d, y: cy + ay * d };
    }

    function findMove(spec, id) {
        for (let i = 0; i < spec.moves.length; i++) if (spec.moves[i].id === id) return spec.moves[i];
        return spec.phase && spec.phase.id === id ? spec.phase : null;
    }

    function livingAdds(it, m) {
        return it.mobs.filter(function (o) { return o.owner === m.id && o.state !== 'dead'; }).length;
    }

    function pick(sim, it, m, spec, dist) {
        if (m.pend) {
            m.pend = false;
            if (spec.phase) return spec.phase;
        }
        const ok = function (mv) {
            return dist <= mv.max && dist >= (mv.min || 0) && (!mv.add || livingAdds(it, m) < mv.cap);
        };
        let list = spec.moves.filter(function (mv) { return ok(mv) && mv.id !== m.last; });
        if (!list.length) list = spec.moves.filter(ok);
        if (!list.length) return null;
        return list.length === 1 ? list[0] : list[Math.min(list.length - 1, Math.floor(P().rand(sim) * list.length))];
    }

    // ---- tell: previews of what the move will cover; aimed ones follow the hero until the lock ----

    function preview(it, grid, m, mv) {
        const h = heroC(it);
        const cx = m.x;
        const cy = m.y - 7;
        if (mv.type === 'hop') {
            m.zones = [{ x: h.x, y: h.y, r: mv.rad, at: mv.atk, done: false, kind: 'hop' }];
        } else if (mv.type === 'web') {
            m.zones = [{ x: h.x, y: h.y, r: mv.rad, at: 0, done: false, kind: 'web' }];
        } else if (mv.type === 'slam') {
            m.zones = [{ x: cx + m.ax * mv.reach, y: cy + m.ay * mv.reach, r: mv.rad, at: 0, done: false, kind: 'slam' }];
        } else if (mv.type === 'march') {
            m.zones = [];
            for (let i = 0; i < mv.n; i++) {
                const d = mv.first + mv.step * i;
                m.zones.push({ x: cx + m.ax * d, y: cy + m.ay * d, r: mv.rad, at: 0.05 + i * 0.22, done: false, kind: 'slam' });
            }
        } else if (mv.type === 'charge' || mv.type === 'lunge') {
            const e = mv.type === 'charge' ? rayEnd(it, grid, cx, cy, m.ax, m.ay, 600) : { x: cx + m.ax * mv.speed * mv.atk, y: cy + m.ay * mv.speed * mv.atk };
            m.line = { x0: cx, y0: cy, x1: e.x, y1: e.y };
        } else if (mv.type === 'beam') {
            const a = Math.atan2(m.ay, m.ax);
            m.beam = { x: cx, y: cy, a: a, a0: a, a1: a + mv.sweep * (m.strafe < 0 ? -1 : 1), len: mv.len, next: 0 };
        }
    }

    function freeSpot(it, grid, m, x, y) {
        return !M().blocked(it.room, grid, x, y, m.r * 0.8, m.r * 1.1);
    }

    function startTell(sim, it, m, spec, mv, grid) {
        m.state = 'tell';
        m.t = 0;
        m.move = mv.id;
        m.last = mv.id;
        m.tellDur = Math.max(MIN_TELL, mv.tell * (m.phase > 1 && spec.fast ? 0.85 : 1));
        m.locked = false;
        m.vx = 0;
        m.vy = 0;
        aim(m, it);
        if (mv.type === 'cross') {
            const cx = m.x;
            const cy = m.y - 7;
            const a = rayEnd(it, grid, cx, cy, -1, 0, 900);
            const b = rayEnd(it, grid, cx, cy, 1, 0, 900);
            const c = rayEnd(it, grid, cx, cy, 0, -1, 900);
            const d = rayEnd(it, grid, cx, cy, 0, 1, 900);
            m.cross = { x: cx, y: cy, half: mv.half, x0: a.x, x1: b.x, y0: c.y, y1: d.y };
        } else if (mv.type === 'blink') {
            const room = it.room;
            const h = heroC(it);
            const cells = [];
            for (let cyy = 0; cyy < room.rows; cyy++) {
                for (let cxx = 0; cxx < room.cols; cxx++) {
                    const x = (cxx + 0.5) * CELL;
                    const y = (cyy + 0.5) * CELL + 7;
                    const dm = Math.hypot(x - m.x, y - m.y);
                    if (dm < 100 || dm > 260 || Math.hypot(x - h.x, y - 7 - h.y) < 100) continue;
                    if (freeSpot(it, grid, m, x, y)) cells.push({ x: x, y: y });
                }
            }
            m.blink = cells.length ? cells[Math.min(cells.length - 1, Math.floor(P().rand(sim) * cells.length))] : null;
        } else {
            preview(it, grid, m, mv);
        }
        pushEvent(sim, 'in_boss_tell', { kind: m.kind, move: mv.id, pattern: m.kind + '_' + mv.id });
    }

    // ---- attack ----

    function shot(it, m, ang, mv, speed) {
        it.shots.push({ x: m.x, y: m.y - 7, vx: Math.cos(ang) * speed, vy: Math.sin(ang) * speed, r: mv.r, dmg: 1, life: mv.life, kind: mv.shot });
    }

    function beginAttack(sim, it, m, spec, mv, grid) {
        m.state = 'attack';
        m.t = 0;
        pushEvent(sim, 'in_boss_attack', { kind: m.kind, move: mv.id, pattern: m.kind + '_' + mv.id });
        const a = Math.atan2(m.ay, m.ax);
        if (mv.type === 'fan') {
            for (let i = 0; i < mv.n; i++) shot(it, m, a + (i - (mv.n - 1) / 2) * mv.spread, mv, mv.speed);
        } else if (mv.type === 'ring') {
            for (let i = 0; i < mv.n; i++) shot(it, m, a + Math.PI / mv.n + (i / mv.n) * Math.PI * 2, mv, mv.speed);
        } else if (mv.type === 'clock') {
            // 14 slots around the aim; slot 0 (the aim itself) and slot 7 (opposite) stay empty: twelve shots, two gaps.
            for (let i = 1; i < mv.slots; i++) {
                if (i !== mv.slots / 2) shot(it, m, a + (i / mv.slots) * Math.PI * 2, mv, mv.speed);
            }
        } else if (mv.type === 'summon') {
            M().summon(sim, it, m, mv.add, mv.n, mv.cap);
        } else if (mv.type === 'web') {
            const z = m.zones[0];
            if (z) m.patches.push({ x: z.x, y: z.y, r: z.r, left: mv.life });
            while (m.patches.length > 2) m.patches.shift();
            m.zones = [];
        } else if (mv.type === 'blink') {
            if (m.blink && freeSpot(it, grid, m, m.blink.x, m.blink.y)) {
                m.x = m.blink.x;
                m.y = m.blink.y;
            }
        } else if (mv.type === 'hop') {
            const z = m.zones[0];
            m.hop = { sx: m.x, sy: m.y, tx: z ? z.x : m.x, ty: z ? z.y + 7 : m.y };
        }
    }

    function resolveZones(sim, it, m) {
        const h = heroC(it);
        (m.zones || []).forEach(function (z) {
            if (z.done || m.t < z.at) return;
            z.done = true;
            if (Math.hypot(h.x - z.x, h.y - z.y) < z.r + HERO_R) I().hurtHero(sim, it, m.kind, z.x, z.y);
        });
    }

    function attackStep(sim, it, m, mv, grid, dt) {
        const t = mv.type;
        if (t === 'hop') {
            const k = Math.min(1, m.t / mv.atk);
            bmove(it, grid, m, m.hop.sx + (m.hop.tx - m.hop.sx) * k - m.x, m.hop.sy + (m.hop.ty - m.hop.sy) * k - m.y);
        } else if (t === 'lunge' || t === 'charge') {
            const ok = bmove(it, grid, m, m.ax * mv.speed * dt, m.ay * mv.speed * dt);
            if (!ok) {
                if (t === 'charge') {
                    m.state = 'stun';
                    m.t = 0;
                    m.move = null;
                    m.line = null;
                    return true;
                }
                m.t = mv.atk;
            }
        } else if (t === 'cross') {
            const c = m.cross;
            const h = heroC(it);
            const inCol = Math.abs(h.x - c.x) < c.half + HERO_R && h.y > c.y0 - HERO_R && h.y < c.y1 + HERO_R;
            const inRow = Math.abs(h.y - c.y) < c.half + HERO_R && h.x > c.x0 - HERO_R && h.x < c.x1 + HERO_R;
            if (inCol || inRow) I().hurtHero(sim, it, m.kind, c.x, c.y);
        } else if (t === 'beam') {
            const b = m.beam;
            b.a = b.a0 + (b.a1 - b.a0) * Math.min(1, m.t / mv.atk);
            b.next -= dt;
            while (b.next <= 0) {
                b.next += mv.emit;
                mv.speeds.forEach(function (s) { shot(it, m, b.a, mv, s); });
            }
        }
        resolveZones(sim, it, m);
        return false;
    }

    function clearMove(m) {
        m.move = null;
        m.zones = [];
        m.line = null;
        m.cross = null;
        m.beam = null;
        m.blink = null;
        m.hop = null;
    }

    function approach(it, grid, m, spec, dt, dist, dx, dy) {
        const ux = dx / (dist || 1);
        const uy = dy / (dist || 1);
        const k = dist > spec.keep[1] ? 1 : (dist < spec.keep[0] ? -1 : 0);
        let vx = ux * k * spec.speed;
        let vy = uy * k * spec.speed;
        if (k === 0 && spec.keep[0] > 0) {
            vx = -uy * m.strafe * spec.speed * 0.5;
            vy = ux * m.strafe * spec.speed * 0.5;
        }
        if (vx || vy) {
            if (!bmove(it, grid, m, vx * dt, vy * dt) && k === 0) m.strafe = -m.strafe;
        }
        if (Math.abs(dx) > 1) m.face = dx > 0 ? 1 : -1;
    }

    // Called from interior-mobs.js `step` for every guardian that is not dead (the dead pose is handled there).
    function step(sim, it, m, dt) {
        const spec = SPECS[m.kind];
        if (!spec) return;
        const h = heroC(it);
        const dx = h.x - m.x;
        const dy = h.y - (m.y - 7);
        const dist = Math.hypot(dx, dy);
        if (m.patches && m.patches.length) {
            m.patches = m.patches.filter(function (p) { p.left -= dt; return p.left > 0; });
            m.patches.forEach(function (p) {
                if (Math.hypot(h.x - p.x, h.y - p.y) < p.r + HERO_R * 0.5) it.hero.slowT = 2;
            });
        }
        if (!m.aggro) {
            if (dist < WAKE && !gateBetween(it, { x: m.x, y: m.y - 7 }, h)) m.aggro = true;
            else return;
        }
        const grid = gridOf(it);
        if (m.state === 'stun') {
            if (m.t >= STUN) {
                m.state = 'move';
                m.t = 0;
                m.cd = 0.6;
            }
        } else if (m.state === 'tell') {
            const mv = findMove(spec, m.move);
            if (!mv) { clearMove(m); m.state = 'move'; m.t = 0; return; }
            if (!m.locked && m.t >= (mv.lock || 0.5) * m.tellDur) m.locked = true;
            if (!m.locked && mv.type !== 'cross' && mv.type !== 'blink' && mv.type !== 'summon') {
                aim(m, it);
                preview(it, grid, m, mv);
            }
            if (m.t >= m.tellDur) beginAttack(sim, it, m, spec, mv, grid);
        } else if (m.state === 'attack') {
            const mv = findMove(spec, m.move);
            if (!mv) { clearMove(m); m.state = 'move'; m.t = 0; return; }
            const stunned = attackStep(sim, it, m, mv, grid, dt);
            if (!stunned && m.t >= mv.atk) {
                clearMove(m);
                m.state = 'move';
                m.t = 0;
                m.cd = spec.cd * (m.phase > 1 && spec.fast ? 0.7 : 1);
            }
        } else {
            if (m.cd > 0) m.cd -= dt;
            approach(it, grid, m, spec, dt, dist, dx, dy);
            if (m.cd <= 0) {
                const mv = pick(sim, it, m, spec, dist);
                if (mv) startTell(sim, it, m, spec, mv, grid);
            }
        }
    }

    root.RunenpfadInteriorBosses = {
        SPECS: SPECS,
        WAKE: WAKE,
        MIN_TELL: MIN_TELL,
        step: step
    };
})(typeof window !== 'undefined' ? window : globalThis);
