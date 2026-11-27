/* Runenpfad interior monsters: seven top-down kinds with telegraphed attacks, shots, drops and box collision. DOM-free. */
(function (root) {
    'use strict';

    const CELL = 48;
    const AGGRO = 180;
    const HERO_R = 9;
    const HURT_TIME = 0.28;
    const DEAD_TIME = 0.6;

    // tell = seconds of warning (>= 0.45), atk = attack duration, cd = pause afterwards, range = distance that starts a tell.
    const DEFS = {
        rat: { hp: 2, r: 9, speed: 95, tell: 0.5, atk: 0.22, cd: 1.2, range: 64, atkSpeed: 260 },
        slime: { hp: 3, r: 12, speed: 38, tell: 0.55, atk: 0.4, cd: 1.6, range: 100, atkSpeed: 170 },
        bat: { hp: 1, r: 8, speed: 105, tell: 0.5, atk: 0.5, cd: 2.0, range: 200, atkSpeed: 300, fly: true },
        spider: { hp: 3, r: 10, speed: 70, tell: 0.6, atk: 0.25, cd: 2.4, range: 280, shot: { speed: 140, r: 5, life: 2.6 } },
        mummy: { hp: 5, r: 12, speed: 38, tell: 0.7, atk: 0.3, cd: 1.8, range: 58, slam: { reach: 34, rad: 38 } },
        icewisp: { hp: 2, r: 9, speed: 48, tell: 0.7, atk: 0.2, cd: 2.6, range: 230, fly: true, ring: 6, shot: { speed: 130, r: 5, life: 2.2 } },
        clockbug: { hp: 3, r: 10, speed: 55, tell: 0.6, atk: 0.55, cd: 2.0, range: 230, atkSpeed: 330 },
        // Guardians (boss: true) are driven by interior-bosses.js; the plain fields keep them a legal mob when that file is absent.
        brood: { hp: 14, r: 26, speed: 40, tell: 0.95, atk: 0.35, cd: 1.8, range: 90, atkSpeed: 200, boss: true },
        weaver: { hp: 14, r: 24, speed: 70, tell: 0.8, atk: 0.25, cd: 1.5, range: 260, boss: true, shot: { speed: 150, r: 5, life: 2.6 } },
        tombward: { hp: 18, r: 24, speed: 45, tell: 0.95, atk: 0.3, cd: 1.7, range: 90, boss: true, slam: { reach: 50, rad: 52 } },
        frostheart: { hp: 14, r: 22, speed: 60, tell: 0.8, atk: 0.25, cd: 1.4, range: 260, fly: true, boss: true, ring: 8, shot: { speed: 120, r: 5, life: 2.4 } },
        boilerward: { hp: 20, r: 26, speed: 50, tell: 0.9, atk: 1.2, cd: 1.6, range: 200, atkSpeed: 300, boss: true },
        counter: { hp: 26, r: 24, speed: 45, tell: 0.9, atk: 0.3, cd: 1.4, range: 240, ring: 12, boss: true, shot: { speed: 120, r: 5, life: 2.6 } }
    };
    const KINDS = Object.keys(DEFS).filter(function (k) { return !DEFS[k].boss; });
    const BOSS_KINDS = Object.keys(DEFS).filter(function (k) { return DEFS[k].boss; });

    function P() { return root.RunenpfadPhysics; }
    function I() { return root.RunenpfadInterior; }
    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }

    // ---- box collision against a solid grid (room.solid or room.walls); the box is x +- hw, y - hh .. y ----

    function blocked(room, grid, x, y, hw, hh) {
        const x0 = Math.floor((x - hw) / CELL);
        const x1 = Math.floor((x + hw - 0.001) / CELL);
        const y0 = Math.floor((y - hh) / CELL);
        const y1 = Math.floor((y - 0.001) / CELL);
        for (let cy = y0; cy <= y1; cy++) {
            for (let cx = x0; cx <= x1; cx++) {
                if (cx < 0 || cy < 0 || cx >= room.cols || cy >= room.rows || grid[cy * room.cols + cx]) return true;
            }
        }
        return false;
    }

    function axisMove(room, grid, e, dx, dy, hw, hh) {
        let f = 1;
        for (let i = 0; i < 4; i++) {
            if (!blocked(room, grid, e.x + dx * f, e.y + dy * f, hw, hh)) {
                e.x += dx * f;
                e.y += dy * f;
                return f === 1;
            }
            f /= 2;
        }
        return false;
    }

    // Axis-separated move. Returns true when the whole step went through.
    function slide(room, grid, e, dx, dy, hw, hh) {
        const a = dx === 0 ? true : axisMove(room, grid, e, dx, 0, hw, hh);
        const b = dy === 0 ? true : axisMove(room, grid, e, 0, dy, hw, hh);
        return a && b;
    }

    function create(kind, id, x, y, n) {
        const d = DEFS[kind];
        const m = {
            id: id, kind: kind, x: x, y: y, vx: 0, vy: 0, hp: d.hp, maxHp: d.hp, state: 'move', t: 0, face: 1, hurtT: 0, deadT: 0, r: d.r,
            cd: 0.6 + (n % 4) * 0.25, aggro: false, ax: 0, ay: 1, ang: (n % 8) * 0.8, kx: 0, ky: 0, kt: 0, strafe: n % 2 ? 1 : -1
        };
        if (d.boss) {
            // Guardian state (documented in interior-bosses.js): flash-only hits, one phase change at half hp.
            m.boss = true;
            m.move = null;
            m.phase = 1;
            m.half = false;
            m.pend = false;
            m.last = null;
            m.tellDur = 0;
            m.zones = [];
            m.patches = [];
            m.line = null;
            m.cross = null;
            m.beam = null;
            m.blink = null;
        }
        return m;
    }

    // Spawns up to n adds of a kind for a guardian on free cells near it (never on exits or solid cells), keeping at most
    // cap of its adds alive. Ids are 'm_s' + n. Returns how many were created.
    function summon(sim, it, owner, kind, n, cap) {
        const d = DEFS[kind];
        if (!d) return 0;
        const room = it.room;
        const h = heroC(it);
        let living = it.mobs.filter(function (o) { return o.owner === owner.id && o.state !== 'dead'; }).length;
        let made = 0;
        if (!it.summonN) it.summonN = 0;
        while (made < n && living < cap) {
            const cells = [];
            for (let cy = 0; cy < room.rows; cy++) {
                for (let cx = 0; cx < room.cols; cx++) {
                    if (room.solid[cy * room.cols + cx] || room.exits.some(function (e) { return e.cx === cx && e.cy === cy; })) continue;
                    const x = (cx + 0.5) * CELL;
                    const y = (cy + 0.5) * CELL + 7;
                    if (Math.hypot(x - owner.x, y - owner.y) > 200 || Math.hypot(x - h.x, y - 7 - h.y) < 70) continue;
                    if (blocked(room, room.solid, x, y, d.r * 0.8, d.r * 1.1)) continue;
                    if (it.mobs.some(function (o) { return o.state !== 'dead' && Math.hypot(o.x - x, o.y - y) < o.r + d.r + 6; })) continue;
                    cells.push({ x: x, y: y });
                }
            }
            if (!cells.length) break;
            const c = cells[Math.min(cells.length - 1, Math.floor(P().rand(sim) * cells.length))];
            const mob = create(kind, 'm_s' + it.summonN, c.x, c.y, it.summonN);
            it.summonN += 1;
            mob.aggro = true;
            mob.owner = owner.id;
            it.mobs.push(mob);
            made += 1;
            living += 1;
        }
        return made;
    }

    function setState(m, s) { m.state = s; m.t = 0; }

    function heroC(it) { return { x: it.hero.x, y: it.hero.y - 7 }; }

    function aimAtHero(m, it) {
        const h = heroC(it);
        const dx = h.x - m.x;
        const dy = h.y - (m.y - 7);
        const d = Math.hypot(dx, dy) || 1;
        m.ax = dx / d;
        m.ay = dy / d;
    }

    function spawnShot(it, m, ang, d) {
        it.shots.push({ x: m.x, y: m.y - 7, vx: Math.cos(ang) * d.shot.speed, vy: Math.sin(ang) * d.shot.speed, r: d.shot.r, dmg: 1, life: d.shot.life, kind: m.kind });
    }

    function beginAttack(sim, it, m, d) {
        setState(m, 'attack');
        pushEvent(sim, 'in_mob_attack', { kind: m.kind, id: m.id });
        if (m.kind === 'spider') spawnShot(it, m, Math.atan2(m.ay, m.ax), d);
        if (m.kind === 'icewisp') {
            for (let i = 0; i < d.ring; i++) spawnShot(it, m, (i / d.ring) * Math.PI * 2 + 0.3, d);
        }
        if (m.kind === 'mummy') {
            const c = { x: m.x + m.ax * d.slam.reach, y: m.y - 7 + m.ay * d.slam.reach };
            const h = heroC(it);
            if (Math.hypot(h.x - c.x, h.y - c.y) < d.slam.rad + HERO_R) I().hurtHero(sim, it, m.kind, m.x, m.y);
        }
    }

    function startTell(sim, m, it) {
        setState(m, 'tell');
        aimAtHero(m, it);
        m.vx = 0;
        m.vy = 0;
        pushEvent(sim, 'in_mob_tell', { kind: m.kind, id: m.id });
    }

    function move(it, m, d, dt, vx, vy) {
        const grid = d.fly ? it.room.walls : it.room.solid;
        slide(it.room, grid, m, vx * dt, vy * dt, d.r * 0.8, d.r * 1.1);
    }

    function moveStep(sim, it, m, d, dt, dist, dx, dy) {
        const ux = dx / (dist || 1);
        const uy = dy / (dist || 1);
        let vx = 0;
        let vy = 0;
        if (m.kind === 'bat') {
            m.ang += 1.6 * dt;
            const h = heroC(it);
            const tx = h.x + Math.cos(m.ang) * 110 - m.x;
            const ty = h.y + Math.sin(m.ang) * 110 - (m.y - 7);
            const td = Math.hypot(tx, ty) || 1;
            vx = tx / td * d.speed;
            vy = ty / td * d.speed;
        } else if (m.kind === 'spider') {
            const k = dist < 140 ? -1 : (dist > 210 ? 1 : 0);
            vx = ux * k * d.speed - uy * m.strafe * d.speed * (k === 0 ? 0.5 : 0.2);
            vy = uy * k * d.speed + ux * m.strafe * d.speed * (k === 0 ? 0.5 : 0.2);
        } else if (m.kind === 'icewisp') {
            const k = dist > 180 ? 1 : (dist < 110 ? -1 : 0);
            const side = Math.sin(it.t * 1.3 + m.ang) * 0.6;
            vx = (ux * k - uy * side) * d.speed;
            vy = (uy * k + ux * side) * d.speed;
        } else if (dist > m.r + HERO_R + 5) {
            vx = ux * d.speed;
            vy = uy * d.speed;
        }
        if (vx || vy) {
            move(it, m, d, dt, vx, vy);
            if (Math.abs(vx) > 1) m.face = vx > 0 ? 1 : -1;
        }
        if (m.cd <= 0 && dist <= d.range) startTell(sim, m, it);
    }

    function attackStep(it, m, d, dt) {
        if (d.atkSpeed) {
            const ok = slide(it.room, d.fly ? it.room.walls : it.room.solid, m, m.ax * d.atkSpeed * dt, m.ay * d.atkSpeed * dt, d.r * 0.8, d.r * 1.1);
            if (!ok && m.kind === 'clockbug') m.t = d.atk;
        }
    }

    function step(sim, it, m, dt) {
        const d = DEFS[m.kind];
        m.t += dt;
        if (m.hurtT > 0) m.hurtT = Math.max(0, m.hurtT - dt);
        if (m.state === 'dead') {
            m.deadT += dt;
            return;
        }
        if (d && d.ext && typeof d.ext.step === 'function') {
            d.ext.step(sim, it, m, dt);
            return;
        }
        if (m.boss && root.RunenpfadInteriorBosses) {
            root.RunenpfadInteriorBosses.step(sim, it, m, dt);
            if (m.aggro && m.state !== 'stun' && m.state !== 'dead' && Math.hypot(it.hero.x - m.x, it.hero.y - m.y) < m.r + HERO_R) I().hurtHero(sim, it, m.kind, m.x, m.y);
            return;
        }
        if (m.cd > 0) m.cd -= dt;
        const h = heroC(it);
        const dx = h.x - m.x;
        const dy = h.y - (m.y - 7);
        const dist = Math.hypot(dx, dy);
        if (m.kt > 0) {
            m.kt -= dt;
            slide(it.room, d.fly ? it.room.walls : it.room.solid, m, m.kx * dt, m.ky * dt, d.r * 0.8, d.r * 1.1);
        }
        if (!m.aggro) {
            if (dist < AGGRO) m.aggro = true;
            else return;
        }
        if (m.state === 'hurt') {
            if (m.t >= HURT_TIME) setState(m, 'move');
            return;
        }
        if (m.state === 'tell') {
            if (m.t < d.tell * 0.5) aimAtHero(m, it);
            if (m.t >= d.tell) beginAttack(sim, it, m, d);
        } else if (m.state === 'attack') {
            attackStep(it, m, d, dt);
            if (m.t >= d.atk) {
                setState(m, 'move');
                m.cd = d.cd;
            }
        } else {
            moveStep(sim, it, m, d, dt, dist, dx, dy);
        }
        if (m.state !== 'hurt' && dist < m.r + HERO_R) I().hurtHero(sim, it, m.kind, m.x, m.y);
    }

    // Damage from the hero. Returns true when the mob died.
    function hit(sim, it, m, dmg, ax, ay) {
        const dd = DEFS[m.kind];
        if (dd && dd.ext && typeof dd.ext.hit === 'function') return dd.ext.hit(sim, it, m, dmg, ax, ay);
        if (m.state === 'dead') return false;
        m.hp -= dmg;
        if (!m.boss) m.aggro = true;
        pushEvent(sim, 'in_hit', { target: 'mob', kind: m.kind });
        if (m.hp <= 0) {
            m.hp = 0;
            m.state = 'dead';
            m.t = 0;
            m.deadT = 0;
            pushEvent(sim, 'in_mob_die', { kind: m.kind, id: m.id });
            if (m.boss) {
                m.move = null;
                m.zones = [];
                m.patches = [];
                m.line = m.cross = m.beam = m.blink = null;
                pushEvent(sim, 'in_boss_die', { kind: m.kind });
            }
            const roll = P().rand(sim);
            const drop = roll < 0.12 ? 'heart' : (roll < 0.47 ? 'coin' : null);
            if (drop) it.pickups.push({ x: m.x, y: m.y, kind: drop, taken: false });
            return true;
        }
        if (m.boss) {
            // A guardian only flashes: no state change, no knockback, so a tell is never cancelled.
            m.hurtT = HURT_TIME;
            if (!m.half && m.hp * 2 <= m.maxHp) {
                m.half = true;
                m.phase = 2;
                m.pend = true;
                pushEvent(sim, 'in_boss_phase', { kind: m.kind });
            }
            return false;
        }
        setState(m, 'hurt');
        m.hurtT = HURT_TIME;
        m.cd = Math.max(m.cd, 0.6);
        m.kx = ax * 140;
        m.ky = ay * 140;
        m.kt = 0.2;
        return false;
    }

    function stepShots(sim, it, dt) {
        const h = heroC(it);
        const keep = [];
        it.shots.forEach(function (s) {
            s.x += s.vx * dt;
            s.y += s.vy * dt;
            s.life -= dt;
            if (s.life <= 0) return;
            const cx = Math.floor(s.x / CELL);
            const cy = Math.floor(s.y / CELL);
            if (cx < 0 || cy < 0 || cx >= it.room.cols || cy >= it.room.rows || (it.room.solid[cy * it.room.cols + cx] && !(it.room.lava && it.room.lava[cy * it.room.cols + cx]))) return;
            if (Math.hypot(s.x - h.x, s.y - h.y) < s.r + HERO_R) {
                if (I().hurtHero(sim, it, s.kind, s.x, s.y, s.dmg)) return;
            }
            keep.push(s);
        });
        it.shots = keep;
    }

    root.RunenpfadInteriorMobs = {
        DEFS: DEFS,
        KINDS: KINDS,
        BOSS_KINDS: BOSS_KINDS,
        AGGRO: AGGRO,
        create: create,
        summon: summon,
        step: step,
        hit: hit,
        stepShots: stepShots,
        blocked: blocked,
        slide: slide
    };
})(typeof window !== 'undefined' ? window : globalThis);
