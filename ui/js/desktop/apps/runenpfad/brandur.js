/* Runenpfad Brandur, the ember wyrm of the deeps: one large top-down guardian with seven telegraphed moves, three phases, scales that
   turn aside most blows and four sleep-rune pillars that make him drowsy. He is not killed: at hp 0 he is "beaten", the room clears
   and the story offers the choice (slay or seal). DOM-free; every mob and room field below is plain data for brandur-view.js.

   Renderer-facing mob state (room pixels; feet = m.x, m.y; centre = m.x, m.y - 7):
     m.state 'sleep'|'move'|'tell'|'attack'|'air'|'drowsy'|'dead' | m.phase 1..3 | m.face +-1 (sprite flips) | m.ax, m.ay aim | m.hurtT flash
     m.dr = { move: null|'breath'|'tail'|'stomp'|'takeoff'|'buffet'|'roar'|'vents', t, tellDur, alt 0..1, shadow {x, y}|null,
       breath {x, y, a, a0, a1, len, half, t, dur, live}|null (x, y = mouth), tail {x, y, a, r, half}|null, patches [{x, y, r, left, life}],
       marks [{x, y, r, t, at, kind: 'stalactite'|'rain', done}], vents {rows: [cy], t, at, dur}|null, ring {x, y, r, max, t}|null,
       drowsy (seconds left), pillars (0..4 lit), phase }
   Room state: it.lair = { fate: 'beaten'|'slain'|'sealed', x, y, face, t, asked } once he is beaten (feet position). */
(function (root) {
    'use strict';

    const CELL = 48;
    const HERO_R = 9;
    const HW = 45;
    const HH = 62;
    const WAKE_R = 300;
    const MIN_TELL = 0.7;
    const DROWSY_T = 5;
    const MUL_AWAKE = 0.25;
    const MUL_DROWSY = 1.5;
    const PILLAR_RE = /^pillar/;
    const CDS = [1.6, 1.3, 1.0];
    const FATES = [null, 'beaten', 'slain', 'sealed'];

    // Tuning knobs (see the report): hp scales the phase thresholds (2/3 and 1/3 of it).
    const DEF = { hp: 90, r: 56, speed: 46, tell: 1.0, atk: 0.4, cd: 1.6, range: 420, boss: true };

    // tell and attack in seconds; min = first phase that may use the move.
    const MOVES = {
        breath: { tell: 1.0, atk: 1.2, atk3: 2.4, min: 1 },
        tail: { tell: 0.8, atk: 0.3, min: 1 },
        stomp: { tell: 0.9, atk: 0.3, min: 1 },
        takeoff: { tell: 0.8, atk: 3.0, min: 2 },
        buffet: { tell: 0.8, atk: 0.4, min: 2 },
        roar: { tell: 0.9, atk: 0.5, min: 2 },
        vents: { tell: 1.0, atk: 0.6, min: 3 }
    };
    const AIR = { follow: 2.2, hold: 0.8, rain: 8, rainFirst: 0.3, rainEvery: 0.35, speed: 160, rise: 0.4 };
    const FALL = { stalactite: 1.1, rain: 1.0 };
    const RING = { max: 150, dur: 0.5 };

    function P() { return root.RunenpfadPhysics; }
    function I() { return root.RunenpfadInterior; }
    function M() { return root.RunenpfadInteriorMobs; }
    function N() { return root.RunenpfadNpcs || null; }
    function D() { return root.RunenpfadDialogue || null; }
    function E() { return root.RunenpfadEmber || null; }
    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }
    function rand(sim) { return P().rand(sim); }

    // ---- small helpers ----

    function heroC(it) { return { x: it.hero.x, y: it.hero.y - 7 }; }
    function flags(sim) { return (sim.profile && sim.profile.npcFlags) || {}; }
    function fateIndex(sim) {
        const v = flags(sim)['dr.fate'];
        return v >= 1 && v <= 3 ? v : 0;
    }
    function fateOf(sim) { return FATES[fateIndex(sim)]; }
    function mealOf(sim) { return E() && E().mealOf ? E().mealOf(sim) : (sim.meal || null); }
    function wrap(a) {
        while (a > Math.PI) a -= Math.PI * 2;
        while (a < -Math.PI) a += Math.PI * 2;
        return a;
    }
    function seen(sim, id) { return !!(sim.profile && sim.profile.seen && sim.profile.seen.indexOf(id) >= 0); }

    let scratch = null;
    // room.solid (walls, lava, pillars, blocks) plus the exit cells: he never stands on an exit.
    function gridOf(it) {
        const room = it.room;
        const n = room.cols * room.rows;
        if (!scratch || scratch.length !== n) scratch = new Uint8Array(n);
        scratch.set(room.solid);
        room.exits.forEach(function (e) {
            if (e.cx >= 0 && e.cy >= 0 && e.cx < room.cols && e.cy < room.rows) scratch[e.cy * room.cols + e.cx] = 1;
        });
        // The "talk" spot behind him (K) is only a prompt cell for later visits; his body may cover it.
        (it.things || []).forEach(function (t) {
            if (t.kind === 'use' && t.request === 'brandur') scratch[t.cy * room.cols + t.cx] = 0;
        });
        return scratch;
    }
    function bmove(it, grid, m, dx, dy) { return M().slide(it.room, grid, m, dx, dy, HW, HH); }
    function floorCell(it, x, y) {
        const room = it.room;
        const cx = Math.floor(x / CELL);
        const cy = Math.floor(y / CELL);
        return cx >= 0 && cy >= 0 && cx < room.cols && cy < room.rows && !room.solid[cy * room.cols + cx];
    }
    function pillarsOf(it) {
        return (it.things || []).filter(function (t) { return t.kind === 'brazier' && PILLAR_RE.test(t.bid || ''); });
    }
    function aim(m, it) {
        const h = heroC(it);
        const dx = h.x - m.x;
        const dy = h.y - (m.y - 7);
        const d = Math.hypot(dx, dy) || 1;
        m.ax = dx / d;
        m.ay = dy / d;
        if (Math.abs(m.ax) > 0.15) m.face = m.ax > 0 ? 1 : -1;
        return d;
    }

    function initDr(m) {
        m.dr = {
            move: null, t: 0, tellDur: 0, alt: 0, shadow: null, breath: null, tail: null, patches: [], marks: [], vents: null, ring: null,
            drowsy: 0, pillars: 0, phase: 1
        };
        m.state = 'sleep';
        m.t = 0;
        m.hist = [];
        m.pendMoves = [];
        m.sweep = 1;
        m.locked = false;
        m.waking = false;
        m.rainN = 0;
        m.hitDone = false;
    }

    function clearHazards(m) {
        const dr = m.dr;
        dr.move = null;
        dr.breath = null;
        dr.tail = null;
        dr.vents = null;
        dr.ring = null;
        dr.shadow = null;
        dr.alt = 0;
        dr.patches = [];
        dr.marks = [];
    }

    // ---- hazards that outlive a move (marks, fire patches, the landing ring) ----

    function hazards(sim, it, m, dt) {
        const dr = m.dr;
        const h = heroC(it);
        const I_ = I();
        let landed = false;
        dr.marks.forEach(function (k) {
            k.t += dt;
            if (!k.done && k.t >= k.at) {
                k.done = true;
                if (Math.hypot(h.x - k.x, h.y - k.y) < k.r + HERO_R) I_.hurtHero(sim, it, k.kind === 'rain' ? 'fire_rain' : 'stalactite', k.x, k.y, 1);
                if (k.kind === 'rain') pushEvent(sim, 'dr_impact', { kind: 'rain' });
                else if (!landed) { landed = true; pushEvent(sim, 'dr_impact', { kind: 'stalactite' }); }
            }
        });
        dr.marks = dr.marks.filter(function (k) { return !k.done || k.t < k.at + 0.5; });
        const tea = mealOf(sim) === 'tea';
        dr.patches.forEach(function (p) {
            p.left -= dt;
            if (!tea && p.left > 0 && Math.hypot(h.x - p.x, h.y - p.y) < p.r + HERO_R * 0.5) I_.hurtHero(sim, it, 'fire_patch', p.x, p.y, 1);
        });
        dr.patches = dr.patches.filter(function (p) { return p.left > 0; });
        if (dr.ring) {
            const g = dr.ring;
            g.t += dt;
            g.r = RING.max * Math.min(1, g.t / RING.dur);
            if (!g.hit && Math.abs(Math.hypot(h.x - g.x, h.y - g.y) - g.r) < 20 && I_.hurtHero(sim, it, 'shockwave', g.x, g.y, 1)) g.hit = true;
            if (g.t >= RING.dur + 0.15) dr.ring = null;
        }
    }

    // ---- moves: tell previews ----

    function lockAt(m) { return 0.6 * m.dr.tellDur; }

    function preview(sim, it, m, id) {
        const dr = m.dr;
        const cx = m.x;
        const cy = m.y - 7;
        const a = Math.atan2(m.ay, m.ax);
        if (id === 'breath') {
            const dir = m.sweep;
            dr.breath = {
                x: cx + m.ax * 60, y: cy + m.ay * 60, a: a - dir * 0.349, a0: a - dir * 0.349, a1: a + dir * 0.349,
                len: 260, half: 0.436, t: 0, dur: m.phase >= 3 ? MOVES.breath.atk3 : MOVES.breath.atk, live: false
            };
        } else if (id === 'tail') {
            dr.tail = { x: cx, y: cy, a: a, r: 150, half: 1.22 };
        } else if (id === 'vents') {
            const room = it.room;
            const hr = Math.max(1, Math.min(room.rows - 2, Math.floor(heroC(it).y / CELL)));
            if (!dr.vents) {
                const up = hr - 3 >= 1;
                const down = hr + 3 <= room.rows - 2;
                const dir = up && down ? (rand(sim) < 0.5 ? -1 : 1) : (up ? -1 : 1);
                m.ventDir = dir;
                dr.vents = { rows: [hr, hr + dir * 3], t: 0, at: dr.tellDur, dur: MOVES.vents.atk };
            } else {
                dr.vents.rows = [hr, hr + m.ventDir * 3];
            }
        }
    }

    function startTell(sim, it, m, id) {
        const mv = MOVES[id];
        const dr = m.dr;
        m.state = 'tell';
        m.t = 0;
        m.locked = false;
        m.waking = false;
        m.vx = 0;
        m.vy = 0;
        m.move = id;
        m.hist.unshift(id);
        if (m.hist.length > 3) m.hist.pop();
        dr.move = id;
        dr.t = 0;
        dr.tellDur = Math.max(MIN_TELL, mv.tell);
        if (id === 'tail') {
            const h = heroC(it);
            const d = Math.hypot(h.x - m.x, h.y - (m.y - 7)) || 1;
            m.ax = (h.x - m.x) / d;
            m.ay = (h.y - (m.y - 7)) / d;
        } else {
            aim(m, it);
        }
        if (id === 'breath') m.sweep = -m.sweep;
        preview(sim, it, m, id);
        pushEvent(sim, 'in_boss_tell', { kind: 'brandur', move: id, pattern: 'brandur_' + id });
    }

    // ---- moves: attacks ----

    function markAt(it, x, y) { return floorCell(it, x, y); }

    function addMark(m, x, y, r, kind) {
        m.dr.marks.push({ x: x, y: y, r: r, t: 0, at: FALL[kind], kind: kind, done: false });
    }

    function stompMarks(sim, it, m) {
        const h = heroC(it);
        addMark(m, h.x, h.y, 26, 'stalactite');
        for (let i = 0; i < 4; i++) {
            for (let tries = 0; tries < 8; tries++) {
                const ang = rand(sim) * Math.PI * 2;
                const dist = 70 + rand(sim) * 40;
                const x = h.x + Math.cos(ang) * dist;
                const y = h.y + Math.sin(ang) * dist;
                if (markAt(it, x, y)) {
                    addMark(m, x, y, 26, 'stalactite');
                    break;
                }
            }
        }
    }

    // Nearest cell where his box fits, searched around (x, y) feet; falls back to the take-off spot.
    function landingSpot(it, grid, x, y, fallback) {
        const room = it.room;
        let best = null;
        let bd = 1e9;
        for (let cy = 0; cy < room.rows; cy++) {
            for (let cx = 0; cx < room.cols; cx++) {
                const px = (cx + 0.5) * CELL;
                const py = (cy + 0.5) * CELL + 7;
                const dd = Math.hypot(px - x, py - y);
                if (dd >= bd || M().blocked(room, grid, px, py, HW, HH)) continue;
                bd = dd;
                best = { x: px, y: py };
            }
        }
        if (!M().blocked(room, grid, x, y, HW, HH)) return { x: x, y: y };
        return best || fallback;
    }

    function extinguish(sim, it) {
        pillarsOf(it).forEach(function (b) {
            if (!b.lit) return;
            b.lit = false;
            b.left = 0;
            pushEvent(sim, 'in_brazier', { lit: false, step: -1 });
        });
    }

    function beginAttack(sim, it, m, id) {
        const dr = m.dr;
        const h = heroC(it);
        m.state = 'attack';
        m.t = 0;
        m.hitDone = false;
        pushEvent(sim, 'in_boss_attack', { kind: 'brandur', move: id, pattern: 'brandur_' + id });
        if (id === 'breath') {
            dr.breath.live = true;
            dr.breath.t = 0;
        } else if (id === 'stomp') {
            stompMarks(sim, it, m);
        } else if (id === 'takeoff') {
            m.state = 'air';
            m.rainN = 0;
            dr.shadow = { x: m.x, y: m.y };
            dr.alt = 0;
            m.from = { x: m.x, y: m.y };
        } else if (id === 'buffet') {
            const hero = it.hero;
            const dx = h.x - m.x;
            const dy = h.y - (m.y - 7);
            const d = Math.hypot(dx, dy) || 1;
            if (d <= 320 && !(hero.dashT > 0)) {
                hero.kx = dx / d * 300;
                hero.ky = dy / d * 300;
                hero.kt = 0.35;
            }
            extinguish(sim, it);
        } else if (id === 'roar') {
            M().summon(sim, it, m, 'drakeling', 2, 2);
        } else if (id === 'vents') {
            pushEvent(sim, 'dr_impact', { kind: 'vent' });
        }
    }

    function breathDamage(sim) {
        return flags(sim)['dr.shield'] ? 1 : 2;
    }

    function attackStep(sim, it, m, id, dt) {
        const dr = m.dr;
        const h = heroC(it);
        const I_ = I();
        if (id === 'breath') {
            const b = dr.breath;
            b.t = m.t;
            const k = Math.min(1, m.t / b.dur);
            const kk = m.phase >= 3 ? (k < 0.5 ? k * 2 : 2 - k * 2) : k;
            b.a = b.a0 + (b.a1 - b.a0) * kk;
            const dx = h.x - b.x;
            const dy = h.y - b.y;
            const dist = Math.hypot(dx, dy);
            if (dist < b.len + HERO_R && Math.abs(wrap(Math.atan2(dy, dx) - b.a)) <= b.half + Math.asin(Math.min(1, HERO_R / Math.max(dist, HERO_R)))) {
                I_.hurtHero(sim, it, 'brandur_breath', b.x, b.y, breathDamage(sim));
            }
        } else if (id === 'tail') {
            const t = dr.tail;
            const dx = h.x - t.x;
            const dy = h.y - t.y;
            const dist = Math.hypot(dx, dy);
            if (!m.hitDone && dist < t.r + HERO_R && Math.abs(wrap(Math.atan2(dy, dx) - t.a)) <= t.half && Math.acos(Math.max(-1, Math.min(1, dx * m.face / (dist || 1)))) > 1.22) {
                if (I_.hurtHero(sim, it, 'brandur_tail', t.x, t.y, 1)) m.hitDone = true;
            }
        } else if (id === 'vents') {
            const v = dr.vents;
            v.t = v.at + m.t;
            if (v.rows.indexOf(Math.floor(h.y / CELL)) >= 0) I_.hurtHero(sim, it, 'vent', h.x, h.y + 10, 1);
        }
    }

    function endMove(sim, it, m, id) {
        const dr = m.dr;
        if (id === 'breath' && dr.breath) {
            const b = dr.breath;
            [0.45, 0.7, 0.95].forEach(function (f) {
                const x = b.x + Math.cos(b.a) * b.len * f;
                const y = b.y + Math.sin(b.a) * b.len * f;
                if (floorCell(it, x, y)) dr.patches.push({ x: x, y: y, r: 30, left: 4, life: 4 });
            });
        }
        dr.move = null;
        dr.breath = null;
        dr.tail = null;
        dr.vents = null;
        m.move = null;
        m.state = 'move';
        m.t = 0;
        m.cd = CDS[m.phase - 1];
    }

    // ---- the air move ----

    function airStep(sim, it, m, dt) {
        const dr = m.dr;
        const h = heroC(it);
        const total = AIR.follow + AIR.hold;
        const t = m.t;
        if (t < AIR.follow) {
            const dx = h.x - dr.shadow.x;
            const dy = (it.hero.y) - dr.shadow.y;
            const d = Math.hypot(dx, dy);
            const step = Math.min(d, AIR.speed * dt);
            if (d > 0.001) {
                dr.shadow.x += dx / d * step;
                dr.shadow.y += dy / d * step;
            }
        }
        m.x = dr.shadow.x;
        m.y = dr.shadow.y;
        dr.alt = t < AIR.rise ? t / AIR.rise : (t >= total - AIR.hold ? Math.max(0, (total - t) / AIR.hold) : 1);
        while (m.rainN < AIR.rain && t >= AIR.rainFirst + m.rainN * AIR.rainEvery) {
            m.rainN += 1;
            let x = h.x + (rand(sim) * 80 - 40);
            let y = h.y + (rand(sim) * 80 - 40);
            if (!floorCell(it, x, y)) { x = h.x; y = h.y; }
            addMark(m, x, y, 24, 'rain');
        }
        if (t >= total) land(sim, it, m);
    }

    function land(sim, it, m) {
        const dr = m.dr;
        const grid = gridOf(it);
        const spot = landingSpot(it, grid, dr.shadow.x, dr.shadow.y, m.from);
        m.x = spot.x;
        m.y = spot.y;
        dr.shadow = null;
        dr.alt = 0;
        dr.ring = { x: m.x, y: m.y - 7, r: 0, max: RING.max, t: 0, hit: false };
        pushEvent(sim, 'dr_impact', { kind: 'land' });
        endMove(sim, it, m, 'takeoff');
        m.cd = Math.max(m.cd, 0.8);
    }

    // ---- choosing a move ----

    function pickMove(sim, it, m, dist) {
        if (m.pendMoves.length) return m.pendMoves.shift();
        const h = heroC(it);
        const dx = h.x - m.x;
        const dy = h.y - (m.y - 7);
        const off = Math.acos(Math.max(-1, Math.min(1, dx * m.face / (Math.hypot(dx, dy) || 1))));
        const lit = pillarsOf(it).filter(function (b) { return b.lit; }).length;
        const drakes = it.mobs.filter(function (o) { return o.owner === m.id && o.state !== 'dead'; }).length;
        const w = {};
        if (dist <= 150 && off > 1.745) w.tail = 1000;
        w.breath = 3;
        w.stomp = 2;
        if (m.phase >= 2) {
            w.takeoff = 1.5;
            w.buffet = lit >= 2 ? 3 : 0.5;
            w.roar = drakes < 2 ? 1 : 0;
        }
        if (m.phase >= 3) w.vents = 2;
        const hist = m.hist;
        const ids = Object.keys(w).filter(function (id) {
            if (id === 'takeoff' && hist[0] === 'takeoff') return false;
            return !(hist.length >= 2 && hist[0] === id && hist[1] === id);
        });
        let total = 0;
        ids.forEach(function (id) { total += w[id]; });
        if (!total) return 'breath';
        let r = rand(sim) * total;
        for (let i = 0; i < ids.length; i++) {
            r -= w[ids[i]];
            if (r < 0) return ids[i];
        }
        return ids[ids.length - 1];
    }

    // ---- wake, drowsy, phases, defeat ----

    function wake(sim, it, m) {
        m.aggro = true;
        m.state = 'tell';
        m.t = 0;
        m.waking = true;
        m.dr.move = 'roar';
        m.dr.t = 0;
        m.dr.tellDur = 0.9;
        aim(m, it);
        pushEvent(sim, 'dr_wake', {});
    }

    function startDrowsy(sim, it, m) {
        const dr = m.dr;
        dr.move = null;
        dr.breath = null;
        dr.tail = null;
        dr.vents = null;
        dr.ring = null;
        m.move = null;
        m.state = 'drowsy';
        m.t = 0;
        dr.drowsy = DROWSY_T;
        pushEvent(sim, 'dr_drowsy', { on: true });
        sim.notice = 'runenpfad.notice.dr_drowsy';
    }

    function endDrowsy(sim, it, m) {
        m.dr.drowsy = 0;
        extinguish(sim, it);
        pushEvent(sim, 'dr_drowsy', { on: false });
        sim.notice = 'runenpfad.notice.dr_awake';
        m.state = 'move';
        m.t = 0;
        m.cd = 0.8;
    }

    function setFate(sim, fate) {
        if (E() && E().setFate) { E().setFate(sim, fate); return; }
        if (N()) N().setFlag(sim, 'dr.fate', 1);
    }

    function defeat(sim, it, m) {
        m.hp = 0;
        m.state = 'dead';
        m.t = 0;
        m.deadT = 0;
        m.waking = false;
        m.move = null;
        clearHazards(m);
        m.dr.drowsy = 0;
        pushEvent(sim, 'in_mob_die', { kind: m.kind, id: m.id });
        pushEvent(sim, 'in_boss_die', { kind: 'brandur' });
        it.mobs.forEach(function (o) {
            if (o.owner !== m.id || o.state === 'dead') return;
            o.hp = 0;
            o.state = 'dead';
            o.t = 0;
            o.deadT = 0;
            pushEvent(sim, 'in_mob_die', { kind: o.kind, id: o.id });
        });
        setFate(sim, 'beaten');
        it.lair = { fate: 'beaten', x: m.x, y: m.y, face: m.face, t: 0, asked: false };
    }

    function phaseCheck(sim, m) {
        if (m.phase < 2 && m.hp <= m.maxHp * 2 / 3) {
            m.phase = 2;
            m.dr.phase = 2;
            m.pendMoves.push('roar');
            pushEvent(sim, 'in_boss_phase', { kind: 'brandur', phase: 2 });
        }
        if (m.phase < 3 && m.hp <= m.maxHp / 3) {
            m.phase = 3;
            m.dr.phase = 3;
            m.pendMoves.push('vents');
            pushEvent(sim, 'in_boss_phase', { kind: 'brandur', phase: 3 });
        }
    }

    // Damage from the hero (interior-mobs.js hands it over through DEFS.brandur.ext). Returns true when he is beaten.
    function hit(sim, it, m, dmg, ax, ay) {
        if (!m.dr) initDr(m);
        if (m.state === 'dead' || m.state === 'air') return false;
        if (m.state === 'sleep') {
            wake(sim, it, m);
            return false;
        }
        const drowsy = m.state === 'drowsy';
        m.hp -= dmg * (drowsy ? MUL_DROWSY : MUL_AWAKE);
        m.hurtT = 0.28;
        pushEvent(sim, 'in_hit', { target: 'mob', kind: 'brandur' });
        if (!drowsy) {
            pushEvent(sim, 'dr_scale', {});
            if (!it.drScaleNote) {
                it.drScaleNote = true;
                sim.notice = 'runenpfad.notice.dr_scales';
            }
        }
        if (m.hp <= 0) {
            defeat(sim, it, m);
            return true;
        }
        phaseCheck(sim, m);
        return false;
    }

    // ---- the tick ----

    const TELL_STATES = { move: 1, tell: 1, attack: 1 };

    function step(sim, it, m, dt) {
        if (!m.dr) initDr(m);
        const dr = m.dr;
        const grid = gridOf(it);
        const h = heroC(it);
        const dx = h.x - m.x;
        const dy = h.y - (m.y - 7);
        const dist = Math.hypot(dx, dy);
        const pil = pillarsOf(it);
        const nLit = pil.filter(function (b) { return b.lit; }).length;
        dr.pillars = nLit;
        dr.phase = m.phase;
        hazards(sim, it, m, dt);
        if (m.state === 'sleep') {
            if (dist < WAKE_R) wake(sim, it, m);
            return;
        }
        if (TELL_STATES[m.state] && !m.waking && pil.length > 0 && nLit === pil.length) startDrowsy(sim, it, m);
        if (m.state === 'drowsy') {
            dr.drowsy -= dt;
            pil.forEach(function (b) { if (b.lit && b.left < dr.drowsy + 0.1) b.left = dr.drowsy + 0.1; });
            if (dr.drowsy <= 0) endDrowsy(sim, it, m);
            return;
        }
        if (m.state === 'air') {
            airStep(sim, it, m, dt);
            return;
        }
        if (m.state === 'tell') {
            dr.t = m.t;
            if (m.waking) {
                if (m.t >= dr.tellDur) {
                    m.waking = false;
                    dr.move = null;
                    m.state = 'move';
                    m.t = 0;
                    m.cd = 0.8;
                }
                return;
            }
            const id = m.move;
            if (!m.locked && m.t >= lockAt(m)) m.locked = true;
            if (!m.locked && id !== 'roar' && id !== 'takeoff' && id !== 'stomp' && id !== 'buffet') {
                if (id === 'tail') {
                    // The swing follows the hero until the lock, without turning the body.
                    const d = Math.hypot(dx, dy) || 1;
                    m.ax = dx / d;
                    m.ay = dy / d;
                } else {
                    aim(m, it);
                }
                preview(sim, it, m, id);
            }
            if (dr.vents) dr.vents.t = m.t;
            if (m.t >= dr.tellDur) beginAttack(sim, it, m, id);
        } else if (m.state === 'attack') {
            const id = m.move;
            dr.t = m.t;
            attackStep(sim, it, m, id, dt);
            const dur = id === 'breath' && m.phase >= 3 ? MOVES.breath.atk3 : MOVES[id].atk;
            if (m.t >= dur) endMove(sim, it, m, id);
        } else {
            if (m.cd > 0) m.cd -= dt;
            const speed = m.phase >= 3 ? 56 : 46;
            const k = dist > 220 ? 1 : (dist < 110 ? -1 : 0);
            const ux = dx / (dist || 1);
            const uy = dy / (dist || 1);
            let vx = ux * k * speed;
            let vy = uy * k * speed;
            if (k === 0) {
                vx = -uy * m.strafe * speed * 0.4;
                vy = ux * m.strafe * speed * 0.4;
            }
            if (!bmove(it, grid, m, vx * dt, vy * dt) && k === 0) m.strafe = -m.strafe;
            // He turns with a lag, so a hero circling behind him stays behind long enough for the tail.
            if (Math.abs(dx) > 30 && (dx > 0 ? 1 : -1) !== m.face) {
                m.turnT = (m.turnT || 0) + dt;
                if (m.turnT >= 0.6) { m.face = -m.face; m.turnT = 0; }
            } else {
                m.turnT = 0;
            }
            if (m.cd <= 0) startTell(sim, it, m, pickMove(sim, it, m, dist));
        }
        if (TELL_STATES[m.state] && !m.waking && Math.hypot(h.x - m.x, h.y - (m.y - 7)) < m.r + HERO_R) I().hurtHero(sim, it, 'brandur', m.x, m.y, 1);
    }

    // ---- room hook and the "talk" use ----

    function brandurMob(it) { return it.mobs.filter(function (o) { return o.kind === 'brandur'; })[0] || null; }

    function roomSetup(sim, it, lay) {
        if (!it.room || !it.room.def || it.room.def.guardian !== 'brandur') return;
        const fate = fateIndex(sim);
        const m = brandurMob(it);
        it.lair = null;
        if (fate >= 1) {
            if (m) it.mobs.splice(it.mobs.indexOf(m), 1);
            it.guardian = null;
            const g = (lay.markers.G || [])[0];
            const x = g ? (g.cx + 0.5) * CELL : it.room.w / 2;
            const y = g ? (g.cy + 0.5) * CELL + 7 : it.room.h / 2;
            it.lair = { fate: FATES[fate], x: x, y: y, face: 1, t: 0, asked: true };
            it.hadMobs = it.mobs.length > 0;
            const ps = I().placeState(sim, it.placeId);
            it.cleared = !!ps.cleared[it.roomId] || !it.hadMobs;
            if (fate === 3) sealPillars(it);
            return;
        }
        if (m) {
            initDr(m);
            if (!seen(sim, 'dr_wake') && !sim.brandurAsked && sim.uiRequest == null) {
                sim.brandurAsked = true;
                sim.uiRequest = 'scene:dr_wake';
            }
        }
    }

    function sealPillars(it) {
        pillarsOf(it).forEach(function (b) {
            b.lit = true;
            b.timed = 0;
            b.left = 0;
        });
    }

    function roomStep(sim, it, input, dt) {
        if (!it.room || !it.room.def || it.room.def.guardian !== 'brandur' || !it.lair) return;
        const L = it.lair;
        const fate = fateIndex(sim);
        L.t += dt;
        if (fate >= 1) L.fate = FATES[fate];
        if (L.fate === 'sealed') sealPillars(it);
        if (L.fate === 'beaten' && !L.asked && L.t >= 1.2 && !sim.encounter && D() && D().openVirtual(sim, 'brandur', 'beaten')) L.asked = true;
    }

    function talkNode(profile) {
        const v = profile && profile.npcFlags ? profile.npcFlags['dr.fate'] : 0;
        return v === 1 ? 'again' : (v === 3 ? 'sleeps' : (v === 2 ? 'bones' : null));
    }

    const USE = {
        prompt: 'talk',
        offer: function (sim, it) {
            const m = brandurMob(it);
            return fateIndex(sim) >= 1 && !(m && m.state !== 'dead');
        },
        use: function (sim) {
            const node = talkNode(sim.profile);
            if (node && D()) D().openVirtual(sim, 'brandur', node);
        }
    };

    function register() {
        const mobs = M();
        if (mobs && mobs.DEFS) mobs.DEFS.brandur = Object.assign({}, DEF, { ext: { step: step, hit: hit } });
        const T = root.RunenpfadInteriorThings;
        if (T && T.USES) T.USES.brandur = USE;
        const H = root.RunenpfadRoomHooks || (root.RunenpfadRoomHooks = []);
        if (!H.some(function (x) { return x.id === 'brandur'; })) H.push({ id: 'brandur', setup: roomSetup, step: roomStep });
    }
    register();

    root.RunenpfadBrandur = {
        DEF: DEF,
        MOVES: MOVES,
        MUL_AWAKE: MUL_AWAKE,
        MUL_DROWSY: MUL_DROWSY,
        DROWSY_T: DROWSY_T,
        PILLAR_RE: PILLAR_RE,
        AIR: AIR,
        FALL: FALL,
        step: step,
        hit: hit,
        setup: roomSetup,
        roomStep: roomStep,
        talkNode: talkNode,
        USE: USE
    };
})(typeof window !== 'undefined' ? window : globalThis);
