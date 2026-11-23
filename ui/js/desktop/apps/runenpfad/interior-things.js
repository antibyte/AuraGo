/* Runenpfad interior things: breakables, push blocks, plates, levers, gates, traps, braziers, cracks, lecterns, signs. DOM-free. */
(function (root) {
    'use strict';

    const CELL = 48;
    const HW = 11;
    const HH = 14;
    const PUSH_T = 0.22;
    const MOVE_T = 0.18;
    const WARN = 0.5;
    const GATE_T = 0.25;
    const HIT_R = 18;
    const KINDS = ['pot', 'block', 'plate', 'lever', 'gate', 'spikes', 'dart', 'brazier', 'crack', 'page', 'sign', 'use'];
    // Kinds that occupy their cell in room.solid while they are intact.
    const SOLID_KINDS = ['pot', 'block', 'lever', 'dart', 'brazier', 'crack', 'page', 'sign', 'use'];
    const DIRS = { left: [-1, 0], right: [1, 0], up: [0, -1], down: [0, 1] };

    function P() { return root.RunenpfadPhysics; }
    function M() { return root.RunenpfadInteriorMobs; }
    function I() { return root.RunenpfadInterior; }
    function D() { return root.RunenpfadDialogue || null; }
    function N() { return root.RunenpfadNpcs || null; }
    function C() { return root.RunenpfadChronicle || null; }
    function ev(sim, type, detail) { P().pushEvent(sim, type, detail); }
    function cc(cx, cy) { return { x: (cx + 0.5) * CELL, y: (cy + 0.5) * CELL + 7 }; }
    function heroCell(h) { return { cx: Math.floor(h.x / CELL), cy: Math.floor((h.y - 7) / CELL) }; }
    function alive(m) { return m.state !== 'dead'; }

    // ---- profile and flag reads ----

    function pageKey(n) { return 'page_' + (n < 10 ? '0' : '') + n; }

    function pageHave(sim, n) {
        const c = C();
        if (c && c.has) return !!c.has(sim.profile, n);
        return ((sim.profile && sim.profile.loot) || []).indexOf(pageKey(n)) >= 0;
    }

    function pageCount(sim) {
        const c = C();
        if (c && c.count) return c.count(sim.profile);
        let n = 0;
        for (let i = 1; i <= 10; i++) if (pageHave(sim, i)) n++;
        return n;
    }

    function flagSet(sim, f) { return !!(N() && N().hasFlag(sim, f)); }

    // ---- state helpers ----

    function placeSt(sim, it) { return I().placeState(sim, it.placeId); }

    // Per room persistence inside the stage run; read defensively (older literals lack the fields).
    function roomSt(sim, it) {
        const ps = placeSt(sim, it);
        const all = ps.things || (ps.things = {});
        const rs = all[it.roomId] || (all[it.roomId] = {});
        if (!rs.broken) rs.broken = {};
        if (!rs.cracks) rs.cracks = {};
        if (!rs.keysUsed) rs.keysUsed = {};
        return rs;
    }

    function setSolid(it, t, on) { it.room.solid[t.cy * it.room.cols + t.cx] = on ? 1 : 0; }

    function setGateCell(it, g, closed) {
        const i = g.cy * it.room.cols + g.cx;
        it.room.solid[i] = closed ? 1 : 0;
        it.room.walls[i] = closed ? 1 : 0;
    }

    function ofKind(it, kind) { return (it.things || []).filter(function (t) { return t.kind === kind; }); }

    function thingAt(it, kind, cx, cy) {
        return (it.things || []).filter(function (t) { return t.kind === kind && t.cx === cx && t.cy === cy; })[0] || null;
    }

    function boxOverlapsCell(x, y, hw, hh, cx, cy) {
        return x + hw > cx * CELL && x - hw < (cx + 1) * CELL && y > cy * CELL && y - hh < (cy + 1) * CELL;
    }

    // ---- conditions ----

    function idOn(it, id) {
        const hits = (it.things || []).filter(function (t) { return t.pid === id || t.lid === id || t.bid === id; });
        return hits.length > 0 && hits.every(function (t) { return t.kind === 'brazier' ? t.lit : t.on; });
    }

    function leverOff(it, id) {
        const hits = ofKind(it, 'lever').filter(function (t) { return t.lid === id; });
        return hits.length > 0 && hits.every(function (t) { return !t.on; });
    }

    function setLit(it, set) {
        const hits = ofKind(it, 'brazier').filter(function (t) { return t.set === set; });
        return hits.length > 0 && hits.every(function (t) { return t.lit; });
    }

    // One condition object; every given part must hold. `g` is the gate that asks (for the key part).
    function cond(sim, it, c, g) {
        if (!c) return true;
        if (c.on && !c.on.every(function (k) { return idOn(it, k); })) return false;
        if (c.off && !c.off.every(function (k) { return leverOff(it, k); })) return false;
        if (c.lit != null && !setLit(it, c.lit)) return false;
        if (c.order != null && !(it.tx && it.tx.orderDone[c.order])) return false;
        if (c.clear && it.mobs.some(alive)) return false;
        if (c.pages != null && pageCount(sim) < c.pages) return false;
        if (c.flag != null && !flagSet(sim, c.flag)) return false;
        if (c.key && !(g && roomSt(sim, it).keysUsed[g.gid])) return false;
        return true;
    }

    // ---- setup ----

    function make(kind, t, c, id, sim, rs) {
        const p = cc(c.cx, c.cy);
        const e = { id: id, kind: kind, cx: c.cx, cy: c.cy, x: p.x, y: p.y };
        if (kind === 'pot') {
            e.look = t.pot;
            e.broken = !!rs.broken[id];
            e.drop = t.drop || null;
        } else if (kind === 'block') {
            e.look = t.block;
            e.moving = null;
        } else if (kind === 'plate') {
            e.pid = t.plate;
            e.on = false;
            e.latch = !!t.latch;
        } else if (kind === 'lever') {
            e.lid = t.lever;
            e.on = !!t.on;
            e.flips = (t.flips || []).slice();
        } else if (kind === 'gate') {
            e.gid = t.gate;
            e.open = false;
            e.k = 0;
            e.cond = t.open || {};
            e.stay = !!t.stay || !!e.cond.key;
        } else if (kind === 'spikes') {
            e.look = t.look || 'spikes';
            e.phase = 'down';
            e.t = 0;
            e.sp = t.spikes;
            e.when = t.when || null;
        } else if (kind === 'dart') {
            e.dir = t.dart;
            e.phase = 'idle';
            e.t = 0;
            e.every = t.every;
            e.offset = t.offset || 0;
            e.when = t.when || null;
            e.pos = e.offset % e.every;
        } else if (kind === 'brazier') {
            e.bid = t.brazier;
            e.set = t.set || null;
            e.lit = false;
            e.left = 0;
            e.timed = t.timed || 0;
        } else if (kind === 'crack') {
            e.broken = !!rs.cracks[id];
            e.hp = e.broken ? 0 : 2;
        } else if (kind === 'page') {
            e.n = t.page;
            e.have = pageHave(sim, t.page);
        } else if (kind === 'sign') {
            e.node = t.sign;
            e.look = t.look || 'plaque';
        } else if (kind === 'use') {
            e.request = t.use;
            e.prompt = t.prompt || (t.use === 'board' ? 'board' : (t.use === 'core' ? 'read' : 'play'));
        }
        return e;
    }

    // Builds it.things for the room that enterRoom has just laid out. Safe to call once per room entry.
    function setup(sim, it, lay) {
        const room = it.room;
        const rs = roomSt(sim, it);
        const ps = placeSt(sim, it);
        const endless = sim.mode === 'endless';
        const counts = {};
        it.things = [];
        it.puzzle = null;
        it.tx = { clock: 0, slip: null, push: null, orderPos: {}, orderDone: {}, setDone: {} };
        lay.def.things.forEach(function (t) {
            const kind = KINDS.filter(function (k) { return t[k] != null; })[0];
            if (!kind) return;
            if (kind === 'page' && endless) return;
            (lay.markers[t.at] || []).forEach(function (c) {
                const n = counts[kind] || 0;
                counts[kind] = n + 1;
                it.things.push(make(kind, t, c, kind + '_' + n, sim, rs));
            });
        });
        it.things.forEach(function (e) {
            if (SOLID_KINDS.indexOf(e.kind) >= 0 && !e.broken) setSolid(it, e, true);
            if (e.kind === 'pot' && e.broken && e.drop === 'key') {
                const key = 'pk_' + it.roomId + '_' + e.id;
                if (!ps.used[key]) it.pickups.push({ x: e.x, y: e.y, kind: 'key', taken: false, key: key });
            }
            if (e.kind === 'brazier' && rs.solved && e.set) e.lit = true;
        });
        const gates = ofKind(it, 'gate');
        if (rs.solved) {
            ofKind(it, 'brazier').forEach(function (b) { if (b.set) { it.tx.setDone[b.set] = true; if (room.def.orders && room.def.orders[b.set]) it.tx.orderDone[b.set] = true; } });
        }
        gates.forEach(function (g) {
            g.open = (rs.solved && g.stay) || cond(sim, it, g.cond, g);
            g.k = g.open ? 1 : 0;
            setGateCell(it, g, !g.open);
        });
        if (gates.length) {
            it.puzzle = rs.solved ? 'solved' : 'open';
            it.tx.everClosed = gates.some(function (g) { return !g.open; });
        }
    }

    // ---- hero floor: slippery ground ----

    function slipCell(it, cx, cy) {
        if (!it.room.def.slippery) return false;
        const row = it.room.def.map[cy];
        return !!row && row.charAt(cx) !== ',' && row.charAt(cx) !== '#' && row.charAt(cx) !== ' ' && row.charAt(cx) !== '';
    }

    function heroMove(sim, it, input, vx, vy, dt) {
        const h = it.hero;
        if (!it.tx || !it.room.def.slippery) return { vx: vx, vy: vy };
        const tx = it.tx;
        if (h.dashT > 0 || h.kt > 0) {
            tx.slip = null;
            return { vx: vx, vy: vy };
        }
        const c = heroCell(h);
        if (!slipCell(it, c.cx, c.cy)) {
            tx.slip = null;
            return { vx: vx, vy: vy };
        }
        if (tx.slip) return { vx: tx.slip.vx, vy: tx.slip.vy };
        if (vx !== 0 || vy !== 0) {
            tx.slip = { vx: vx, vy: vy, announced: false };
        }
        return { vx: vx, vy: vy };
    }

    // ---- blocks ----

    function blockFree(it, cx, cy) {
        const room = it.room;
        if (cx < 0 || cy < 0 || cx >= room.cols || cy >= room.rows) return false;
        if (room.solid[cy * room.cols + cx]) return false;
        if (room.exits.some(function (e) { return e.cx === cx && e.cy === cy; })) return false;
        if (it.mobs.some(function (m) { return alive(m) && boxOverlapsCell(m.x, m.y, m.r * 0.8, m.r * 1.1, cx, cy); })) return false;
        if (it.pickups.some(function (pk) { return !pk.taken && Math.floor(pk.x / CELL) === cx && Math.floor((pk.y - 7) / CELL) === cy; })) return false;
        if (boxOverlapsCell(it.hero.x, it.hero.y, HW, HH, cx, cy)) return false;
        return true;
    }

    function startMove(sim, it, b, dx, dy, slid) {
        const tx = b.cx + dx;
        const ty = b.cy + dy;
        if (!blockFree(it, tx, ty)) return false;
        b.moving = { fx: b.cx, fy: b.cy, tx: tx, ty: ty, t: 0, dx: dx, dy: dy };
        it.room.solid[ty * it.room.cols + tx] = 1;
        ev(sim, slid ? 'in_slide' : 'in_push');
        return true;
    }

    function stepBlock(sim, it, b, dt) {
        const m = b.moving;
        if (!m) return;
        m.t += dt;
        if (m.t < MOVE_T) {
            const f = m.t / MOVE_T;
            b.x = (m.fx + 0.5 + (m.tx - m.fx) * f) * CELL;
            b.y = (m.fy + 0.5 + (m.ty - m.fy) * f) * CELL + 7;
            return;
        }
        it.room.solid[m.fy * it.room.cols + m.fx] = 0;
        b.cx = m.tx;
        b.cy = m.ty;
        const p = cc(b.cx, b.cy);
        b.x = p.x;
        b.y = p.y;
        b.moving = null;
        if (it.room.def.slippery && slipCell(it, b.cx, b.cy)) startMove(sim, it, b, m.dx, m.dy, true);
    }

    // Runs right after the hero's slide: stops a slide at a wall and times the push against a block.
    function afterMove(sim, it, input, moved, dt) {
        if (!it.tx) return;
        const tx = it.tx;
        const h = it.hero;
        if (tx.slip && !moved) tx.slip = null;
        else if (tx.slip && !tx.slip.announced) {
            tx.slip.announced = true;
            ev(sim, 'in_slide');
        }
        const mx = (input.right ? 1 : 0) - (input.left ? 1 : 0);
        const my = (input.down ? 1 : 0) - (input.up ? 1 : 0);
        if (moved || h.dashT > 0 || (mx === 0) === (my === 0)) {
            tx.push = null;
            return;
        }
        const px = mx !== 0 ? h.x + mx * (HW + 2) : h.x;
        const py = my < 0 ? h.y - HH - 2 : (my > 0 ? h.y + 2 : h.y - 7);
        const cx = Math.floor(px / CELL);
        const cy = Math.floor(py / CELL);
        const b = thingAt(it, 'block', cx, cy);
        const within = mx !== 0 ? (h.y - HH >= cy * CELL && h.y <= (cy + 1) * CELL) : (h.x - HW >= cx * CELL && h.x + HW <= (cx + 1) * CELL);
        if (!b || b.moving || !within) {
            tx.push = null;
            return;
        }
        const key = b.id + ':' + mx + ':' + my;
        if (!tx.push || tx.push.key !== key) tx.push = { key: key, t: 0 };
        if (!it.pushHinted) {
            it.pushHinted = true;
            sim.notice = 'runenpfad.notice.push_hint';
        }
        tx.push.t += dt;
        if (tx.push.t >= PUSH_T) {
            tx.push = null;
            startMove(sim, it, b, mx, my, false);
        }
    }

    // ---- plates, braziers, gates ----

    function stepPlates(sim, it) {
        const hc = heroCell(it.hero);
        ofKind(it, 'plate').forEach(function (p) {
            const stood = (hc.cx === p.cx && hc.cy === p.cy) || !!thingAt(it, 'block', p.cx, p.cy);
            const on = p.latch && p.on ? true : stood;
            if (on !== p.on) {
                p.on = on;
                ev(sim, 'in_plate', { on: on });
            }
        });
    }

    function setComplete(it, set) {
        if (setLit(it, set)) it.tx.setDone[set] = true;
        return !!it.tx.setDone[set];
    }

    function lightBrazier(sim, it, b) {
        if (b.lit) return;
        const tx = it.tx;
        const order = b.set && it.room.def.orders && it.room.def.orders[b.set];
        if (order && !tx.orderDone[b.set]) {
            const pos = tx.orderPos[b.set] || 0;
            if (order[pos] !== b.bid) {
                ofKind(it, 'brazier').forEach(function (o) { if (o.set === b.set) { o.lit = false; o.left = 0; } });
                tx.orderPos[b.set] = 0;
                ev(sim, 'in_puzzle_fail');
                sim.notice = 'runenpfad.notice.puzzle_fail';
                return;
            }
            b.lit = true;
            b.left = b.timed;
            tx.orderPos[b.set] = pos + 1;
            ev(sim, 'in_brazier', { lit: true, step: pos });
            if (pos + 1 >= order.length) tx.orderDone[b.set] = true;
        } else {
            b.lit = true;
            b.left = b.timed;
            ev(sim, 'in_brazier', { lit: true, step: -1 });
        }
        if (b.set) setComplete(it, b.set);
    }

    function stepBraziers(sim, it, dt) {
        ofKind(it, 'brazier').forEach(function (b) {
            if (!b.lit || !b.timed || (b.set && it.tx.setDone[b.set])) return;
            b.left -= dt;
            if (b.left <= 0) {
                b.lit = false;
                b.left = 0;
                ev(sim, 'in_brazier', { lit: false, step: -1 });
                sim.notice = 'runenpfad.notice.lamps_out';
            }
        });
    }

    function openGate(sim, it, g, open) {
        g.open = open;
        setGateCell(it, g, !open);
        ev(sim, 'in_gate', { open: open });
    }

    function stepGates(sim, it, dt) {
        const gates = ofKind(it, 'gate');
        if (!gates.length) return;
        const rs = roomSt(sim, it);
        gates.forEach(function (g) {
            if (!(g.open && g.stay)) {
                const want = cond(sim, it, g.cond, g);
                if (want && !g.open) openGate(sim, it, g, true);
                else if (!want && g.open && !boxOverlapsCell(it.hero.x, it.hero.y, HW, HH, g.cx, g.cy)) {
                    g.open = false;
                    setGateCell(it, g, true);
                    ev(sim, 'in_gate', { open: false });
                }
            }
            g.k = Math.max(0, Math.min(1, g.k + (g.open ? 1 : -1) * dt / GATE_T));
        });
        if (it.puzzle === 'open' && it.tx.everClosed && gates.every(function (g) { return g.open; })) {
            it.puzzle = 'solved';
            rs.solved = true;
            ev(sim, 'in_puzzle_done');
            if (gates.some(function (g) { const c = g.cond; return c.on || c.off || c.lit != null || c.order != null; })) sim.notice = 'runenpfad.notice.puzzle_done';
        }
    }

    // ---- traps ----

    function stepSpikes(sim, it, s) {
        const sp = s.sp;
        let phase = 'off';
        let t = 0;
        if (!s.when || cond(sim, it, s.when)) {
            const pos = (it.tx.clock + (sp.offset || 0)) % sp.period;
            const r = sp.period - WARN - sp.up;
            if (pos < r) { phase = 'down'; t = pos; } else if (pos < r + WARN) { phase = 'warn'; t = pos - r; } else { phase = 'up'; t = pos - r - WARN; }
        }
        if (phase !== s.phase) {
            if (phase === 'warn') ev(sim, 'in_trap_warn', { kind: s.look });
            if (phase === 'up') ev(sim, 'in_trap', { kind: s.look });
        }
        s.phase = phase;
        s.t = t;
        if (phase === 'up') {
            const hc = heroCell(it.hero);
            if (hc.cx === s.cx && hc.cy === s.cy) I().hurtHero(sim, it, s.look, s.x, s.y);
        }
    }

    function stepDart(sim, it, d) {
        const pos = (it.tx.clock + d.offset) % d.every;
        const wrapped = pos < d.pos;
        d.pos = pos;
        if (d.when && !cond(sim, it, d.when)) {
            d.phase = 'idle';
            d.t = 0;
            return;
        }
        if (wrapped && d.phase === 'warn') {
            const v = DIRS[d.dir];
            it.shots.push({ x: (d.cx + 0.5) * CELL + v[0] * (CELL / 2 + 1), y: (d.cy + 0.5) * CELL + v[1] * (CELL / 2 + 1), vx: v[0] * 150, vy: v[1] * 150, r: 4, dmg: 1, life: 4, kind: 'dart' });
            ev(sim, 'in_trap', { kind: 'dart' });
        }
        const warn = pos >= d.every - WARN;
        if (warn && d.phase !== 'warn') ev(sim, 'in_trap_warn', { kind: 'dart' });
        d.phase = warn ? 'warn' : 'idle';
        d.t = warn ? pos - (d.every - WARN) : 0;
    }

    // Once per interior tick, after the mob shots.
    function step(sim, it, input, dt) {
        if (!it.things || !it.tx) return;
        it.tx.clock += dt;
        it.things.forEach(function (t) {
            if (t.kind === 'block') stepBlock(sim, it, t, dt);
            else if (t.kind === 'page') t.have = pageHave(sim, t.n);
        });
        stepPlates(sim, it);
        stepBraziers(sim, it, dt);
        stepGates(sim, it, dt);
        it.things.forEach(function (t) {
            if (t.kind === 'spikes') stepSpikes(sim, it, t);
            else if (t.kind === 'dart') stepDart(sim, it, t);
        });
    }

    // ---- hero swing ----

    function inArc(cx, cy, ax, ay, cfg, tx, ty) {
        const dx = tx - cx;
        const dy = ty - cy;
        const dist = Math.hypot(dx, dy);
        if (dist - HIT_R > cfg.reach) return false;
        if (dist > 8) {
            const cos = Math.max(-1, Math.min(1, (ax * dx + ay * dy) / dist));
            if (Math.acos(cos) > cfg.half + Math.asin(Math.min(1, HIT_R / dist))) return false;
        }
        return true;
    }

    function breakPot(sim, it, p) {
        const rs = roomSt(sim, it);
        const ps = placeSt(sim, it);
        p.broken = true;
        setSolid(it, p, false);
        rs.broken[p.id] = 1;
        ev(sim, 'in_break', { kind: p.look });
        let drop = p.drop;
        if (!drop) {
            const roll = P().rand(sim);
            drop = roll < 0.45 ? 'coin' : (roll < 0.55 ? 'heart' : 'none');
        }
        if (drop === 'key') {
            const key = 'pk_' + it.roomId + '_' + p.id;
            if (!ps.used[key]) it.pickups.push({ x: p.x, y: p.y, kind: 'key', taken: false, key: key });
        } else if (drop === 'coin' || drop === 'heart') {
            it.pickups.push({ x: p.x, y: p.y, kind: drop, taken: false });
        }
    }

    // Called from the hero swing with the swing centre, aim and arc; each target is visited once per swing.
    function swing(sim, it, cx, cy, ax, ay, cfg) {
        if (!it.things) return;
        it.things.slice().forEach(function (t) {
            if (t.kind !== 'pot' && t.kind !== 'crack' && t.kind !== 'brazier') return;
            if ((t.kind === 'pot' || t.kind === 'crack') && t.broken) return;
            if (!inArc(cx, cy, ax, ay, cfg, t.x, t.y - 7)) return;
            if (t.kind === 'pot') {
                breakPot(sim, it, t);
            } else if (t.kind === 'brazier') {
                lightBrazier(sim, it, t);
            } else {
                t.hp -= 1;
                if (t.hp <= 0) {
                    t.hp = 0;
                    t.broken = true;
                    setSolid(it, t, false);
                    roomSt(sim, it).cracks[t.id] = 1;
                }
                ev(sim, 'in_crack', { broken: t.broken });
            }
        });
    }

    // ---- interaction ----

    // Offers the things within reach to interact()'s nearest-target search.
    function consider(sim, it, offer) {
        if (!it.things) return;
        const keys = placeSt(sim, it).keys || 0;
        it.things.forEach(function (t) {
            if (t.kind === 'lever') offer(t.x, t.y, 'pull', t);
            else if (t.kind === 'brazier') { if (!t.lit) offer(t.x, t.y, 'light', t); }
            else if (t.kind === 'page' || t.kind === 'sign') offer(t.x, t.y, 'read', t);
            else if (t.kind === 'use') { if (!(t.request === 'core' && flagSet(sim, 'obs.done'))) offer(t.x, t.y, t.prompt, t); }
            else if (t.kind === 'gate' && !t.open) offer(t.x, t.y, t.cond.key && keys > 0 ? 'unlock' : 'shut', t);
        });
    }

    function openVirtual(sim, npc, node) {
        const d = D();
        if (d && d.openVirtual) d.openVirtual(sim, npc, node);
    }

    function shutNotice(sim, it, g) {
        sim.notice = g.cond.key ? 'runenpfad.notice.smallkey_need' : (g.cond.pages != null && pageCount(sim) < g.cond.pages ? 'runenpfad.notice.need_pages' : 'runenpfad.notice.gate_shut');
        ev(sim, 'in_locked');
    }

    function useGate(sim, it, g) {
        const ps = placeSt(sim, it);
        const rest = Object.assign({}, g.cond);
        delete rest.key;
        if (!g.cond.key || (ps.keys || 0) < 1 || !cond(sim, it, rest, g)) {
            shutNotice(sim, it, g);
            return;
        }
        ps.keys -= 1;
        roomSt(sim, it).keysUsed[g.gid] = 1;
        g.stay = true;
        ev(sim, 'in_unlock');
        sim.notice = 'runenpfad.notice.unlocked';
        openGate(sim, it, g, true);
    }

    // Handles the interact key for the prompt kinds of this module. Returns true when `best` was one of them.
    function use(sim, it, best, input) {
        const t = best && best.ref;
        if (!t || KINDS.indexOf(t.kind) < 0) return false;
        if (t.kind === 'lever') {
            t.on = !t.on;
            ofKind(it, 'lever').forEach(function (o) { if (o !== t && t.flips.indexOf(o.lid) >= 0) o.on = !o.on; });
            ev(sim, 'in_lever', { on: t.on });
        } else if (t.kind === 'brazier') {
            lightBrazier(sim, it, t);
        } else if (t.kind === 'page') {
            const c = C();
            const first = !!(c && c.give && c.give(sim, t.n));
            t.have = pageHave(sim, t.n);
            if (!first) ev(sim, 'in_read');
            openVirtual(sim, 'journal', 'p' + t.n + 'a');
        } else if (t.kind === 'sign') {
            ev(sim, 'in_read');
            openVirtual(sim, 'lore', t.node);
        } else if (t.kind === 'use') {
            if (t.request === 'core') openVirtual(sim, 'core', 'query');
            else sim.uiRequest = t.request;
        } else if (t.kind === 'gate') {
            useGate(sim, it, t);
        } else {
            return false;
        }
        return true;
    }

    // Dynamic lights for the view: lit braziers and lecterns whose page the profile still lacks.
    function lights(it) {
        const out = [];
        if (!it || !it.things) return out;
        const wood = it.room && it.room.def && it.room.def.theme === 'wood';
        it.things.forEach(function (t) {
            if (t.kind === 'brazier' && t.lit) out.push({ x: t.x, y: t.y - 7, r: wood ? 150 : 170, color: wood ? '#ffd27a' : '#ffb060', flicker: true });
            else if (t.kind === 'page' && !t.have) out.push({ x: t.x, y: t.y - 7, r: 90, color: '#ffe9a8', flicker: false });
        });
        return out;
    }

    root.RunenpfadInteriorThings = {
        KINDS: KINDS,
        setup: setup,
        heroMove: heroMove,
        afterMove: afterMove,
        step: step,
        swing: swing,
        consider: consider,
        use: use,
        lights: lights,
        cond: cond
    };
})(typeof window !== 'undefined' ? window : globalThis);
