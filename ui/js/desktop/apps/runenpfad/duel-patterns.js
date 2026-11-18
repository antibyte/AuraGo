/* Runenpfad duel pattern table: the numbers (plan 1.6) and the shot/zone/minion spawners per NPC. DOM-free. */
(function (root) {
    'use strict';

    const ARENA = { w: 960, h: 540, floorY: 450, left: 80, right: 880 };
    const G = 1700;
    const MIN_TELL = 0.45;
    const HERO_HW = 26;
    const HERO_H = 190;

    // hp/speed/height from the plan; `hw` is the half width of the NPC hurt box. Per pattern: tell/act/rec seconds,
    // `range` (melee <= 150, ranged 260-420, any), `blk` blockable, `act2` the phase 2 act length. `p2` holds the phase 2 extras.
    const TABLE = {
        witch: { hp: 126, speed: 120, height: 220, hw: 36, p2: { fan: 3, clouds: 2 }, patterns: {
            a: { tell: 0.55, act: 0.25, rec: 0.55, range: 'ranged', blk: true },
            b: { tell: 0.70, act: 0.2, rec: 0.6, range: 'any', blk: false },
            c: { tell: 0.60, act: 1.4, rec: 0.9, range: 'any', blk: true } } },
        wizard: { hp: 162, speed: 100, height: 220, hw: 36, p2: { missiles: 5, ring: true }, patterns: {
            a: { tell: 0.50, act: 0.45, act2: 0.7, rec: 0.6, range: 'ranged', blk: true },
            b: { tell: 0.60, act: 0.35, rec: 0.8, range: 'any', blk: true },
            c: { tell: 0.45, act: 2.5, act2: 2.6, rec: 0.5, range: 'any', blk: false } } },
        merchant: { hp: 132, speed: 170, height: 220, hw: 34, p2: { bombs: 2, dagger: true }, patterns: {
            a: { tell: 0.45, act: 0.3, rec: 0.6, range: 'melee', blk: true },
            b: { tell: 0.55, act: 0.15, act2: 0.5, rec: 0.6, range: 'ranged', blk: false },
            c: { tell: 0.50, act: 1.0, act2: 1.55, rec: 0.5, range: 'any', blk: false } } },
        chief: { hp: 180, speed: 110, height: 240, hw: 46, p2: { shock: true, kobolds: 2, cap: 3 }, patterns: {
            a: { tell: 0.65, act: 0.3, rec: 0.9, range: 'melee', blk: true },
            b: { tell: 0.70, act: 0.35, rec: 0.7, range: 'any', blk: false },
            c: { tell: 0.60, act: 1.6, rec: 0.8, range: 'any', blk: true } } },
        seer: { hp: 138, speed: 110, height: 220, hw: 36, p2: { clones: 3, beam2: true }, patterns: {
            a: { tell: 0.60, act: 0.2, rec: 0.6, range: 'any', blk: false },
            b: { tell: 0.50, act: 2.05, rec: 0.6, range: 'any', blk: false },
            c: { tell: 0.80, act: 0.45, act2: 1.35, rec: 0.7, range: 'any', blk: false } } },
        alchemist: { hp: 156, speed: 95, height: 220, hw: 36, p2: { vents: 3 }, patterns: {
            a: { tell: 0.50, act: 0.2, rec: 0.6, range: 'ranged', blk: true },
            b: { tell: 0.60, act: 0.35, rec: 0.6, range: 'any', blk: false },
            c: { tell: 0.70, act: 0.2, rec: 0.8, range: 'any', blk: false } } }
    };

    function Dm() { return root.RunenpfadDuel; }
    function rnd(sim) { return root.RunenpfadPhysics.rand(sim); }
    function clampX(x) { return Math.max(ARENA.left + 20, Math.min(ARENA.right - 20, x)); }
    function sgn(v) { return v < 0 ? -1 : 1; }

    function tellTime(npcId, key, phase) {
        const t = TABLE[npcId].patterns[key].tell;
        return phase === 2 ? Math.max(MIN_TELL, t * 0.85) : t;
    }

    function actTime(npcId, key, phase) {
        const p = TABLE[npcId].patterns[key];
        return phase === 2 && p.act2 ? p.act2 : p.act;
    }

    function addShot(d, o) {
        const s = Object.assign({ kind: 'hex', x: 0, y: 0, vx: 0, vy: 0, dmg: 1, owner: 'npc', blockable: true, homing: 0, turn: 0, life: 3, g: 0, r: 14, pierce: false }, o);
        d.shots.push(s);
        return s;
    }

    function addZone(d, o) {
        const z = Object.assign({ kind: 'cloud', x: 0, w: 100, h: 100, t: 1, warn: 0, dmg: 1, tick: 0, tickT: 0, vx: 0, pull: 0, blockable: false, tellOf: false, hit: false }, o);
        d.zones.push(z);
        return z;
    }

    function addMinion(d, o) {
        const m = Object.assign({ kind: 'kobold', x: 0, y: ARENA.floorY, vx: 0, hp: 4, fuse: -1, t: 0, cd: 0.3, speed: 160 }, o);
        d.minions.push(m);
        return m;
    }

    function count(d, kind) { return d.minions.filter(function (m) { return m.kind === kind; }).length; }
    function chest(n) { return n.y - 120; }
    function aimAt(n, h) { return Math.atan2(h.y - 100 - chest(n), h.x - n.x); }
    function burst(d, kind, x, y, n, o) { if (d.fx && root.RunenpfadFx) root.RunenpfadFx.burst(d.fx, kind, x, y, n, o); }
    function faceHero(d) { d.npc.facing = sgn(d.hero.x - d.npc.x); }

    // A lobbed projectile that lands at floor level on `tx` after `T` seconds.
    function lob(d, o, tx, T) {
        const n = d.npc;
        const x0 = n.x + n.facing * 30;
        const y0 = chest(n);
        return addShot(d, Object.assign({ x: x0, y: y0, vx: (tx - x0) / T, vy: (ARENA.floorY - y0 - 0.5 * G * T * T) / T, g: G, life: T + 0.5 }, o));
    }

    // Wall-limited strike box check in front of the NPC (reach from the NPC centre).
    function inFront(d, reach) {
        const dx = d.hero.x - d.npc.x;
        return dx * d.npc.facing > -10 && Math.abs(dx) <= reach && Math.abs(d.hero.y - d.npc.y) < 120;
    }

    function hurt(sim, d, dmg, o) { return Dm().hurtHero(sim, d, dmg, o); }

    function bolts(sim, d, n, spread, o) {
        const base = aimAt(d.npc, d.hero);
        for (let k = 0; k < n; k++) {
            const a = n === 1 ? base : base + (k / (n - 1) * 2 - 1) * spread;
            addShot(d, Object.assign({ x: d.npc.x + d.npc.facing * 34, y: chest(d.npc), vx: Math.cos(a) * 430, vy: Math.sin(a) * 430 }, o));
        }
    }

    function stagger(d, secs) { Dm().stagger(d, secs); }

    // Pattern spawners. tell: at tell start; tellTick: every tell tick; act: at act start; tick: every act tick (true ends the act early);
    // `m` is the per-pattern scratch (d.npc.ai.mem), reset before every pattern.
    const SPAWN = {
        witch_a: { act: function (sim, d, ph) { bolts(sim, d, ph === 2 ? TABLE.witch.p2.fan : 1, 0.18, { kind: 'hex', dmg: 2, life: 3.5 }); } },
        witch_b: {
            tell: function (sim, d, ph, dur) {
                const xs = [d.hero.x];
                if (ph === 2) xs.push(d.hero.x + (rnd(sim) < 0.5 ? -220 : 220));
                xs.forEach(function (x) { addZone(d, { kind: 'cloud', x: clampX(x), w: 180, h: 150, warn: dur, t: 1.6, dmg: 1, tick: 0.5, tellOf: true }); });
            }
        },
        witch_c: {
            tell: function (sim, d, ph, dur) { d.npc.fly = true; d.npc.ai.mem.dur = dur; },
            tellTick: function (sim, d, dt, ph, u) { const k = Math.min(1, u / d.npc.ai.mem.dur); d.npc.y = ARENA.floorY - 160 * k; d.npc.vy = 0; },
            act: function (sim, d) { const m = d.npc.ai.mem; m.tx = d.hero.x; m.hit = false; },
            tick: function (sim, d, dt) {
                const n = d.npc;
                const m = n.ai.mem;
                const dx = m.tx - n.x;
                const dy = ARENA.floorY - n.y;
                const len = Math.hypot(dx, dy) || 1;
                const step = Math.min(len, 640 * dt);
                n.x = clampX(n.x + dx / len * step);
                n.y += dy / len * step;
                n.vx = dx / len * 640;
                n.facing = sgn(dx || n.facing);
                if (!m.hit && Math.abs(d.hero.x - n.x) < 60 && n.y > d.hero.y - HERO_H - 30 && n.y < d.hero.y + 30) {
                    const r = hurt(sim, d, 3, { from: n.x, blockable: true, kb: 150 });
                    if (r !== 'ignored') m.hit = true;
                    if (r === 'block') { stagger(d, 0.8); return true; }
                }
                if (len - step < 6) { n.y = ARENA.floorY; n.fly = false; n.vx = 0; burst(d, 'dust', n.x, n.y, 10, { color: '#c4b5fd', speed: 200 }); return true; }
                return false;
            }
        },
        wizard_a: {
            tick: function (sim, d, dt, ph, u) {
                const m = d.npc.ai.mem;
                const total = ph === 2 ? TABLE.wizard.p2.missiles : 3;
                while ((m.n || 0) < total && u >= (m.n || 0) * 0.12) {
                    const k = m.n || 0;
                    const a = -Math.PI / 2 + (k / (total - 1) - 0.5) * 1.1 + d.npc.facing * 0.4;
                    addShot(d, { kind: 'arcane', x: d.npc.x + d.npc.facing * 20, y: chest(d.npc) - 30, vx: Math.cos(a) * 340, vy: Math.sin(a) * 340, homing: 340, turn: 2.0, life: 2.4, dmg: 1 });
                    m.n = k + 1;
                }
                return false;
            }
        },
        wizard_b: {
            tell: function (sim, d, ph, dur) {
                const x = clampX(d.hero.x - d.hero.facing * 110);
                d.npc.ai.mem.x = x;
                addZone(d, { kind: 'marker', x: x, w: 70, h: 200, warn: dur, t: 0, dmg: 0, tellOf: true });
            },
            act: function (sim, d) {
                const n = d.npc;
                burst(d, 'rune', n.x, n.y - 100, 10, { color: '#60a5fa', speed: 160 });
                n.x = d.npc.ai.mem.x;
                faceHero(d);
                burst(d, 'rune', n.x, n.y - 100, 10, { color: '#60a5fa', speed: 160 });
            },
            tick: function (sim, d, dt, ph, u) {
                const m = d.npc.ai.mem;
                if (!m.done && u >= 0.12) {
                    m.done = true;
                    faceHero(d);
                    if (inFront(d, 140)) hurt(sim, d, 3, { from: d.npc.x, blockable: true, kb: 140 });
                }
                return false;
            }
        },
        wizard_c: {
            can: function (d) { return !(d.npc.shield > 0); },
            act: function (sim, d) { d.npc.shield = 2.5; burst(d, 'rune', d.npc.x, d.npc.y - 110, 14, { color: '#93c5fd', speed: 180 }); },
            tick: function (sim, d, dt, ph, u) {
                const m = d.npc.ai.mem;
                if (ph === 2 && !m.ring && u >= 2.5) {
                    m.ring = true;
                    [-1, 1].forEach(function (s) { addZone(d, { kind: 'shock', x: d.npc.x, w: 60, h: 40, t: 3, dmg: 2, vx: s * 520 }); });
                }
                return false;
            }
        },
        merchant_a: {
            act: function (sim, d) {
                const m = d.npc.ai.mem;
                faceHero(d);
                m.dist = Math.max(0, Math.min(200, Math.abs(d.hero.x - d.npc.x) - 60));
                m.sp = m.dist / 0.3;
                m.hits = 0;
            },
            tick: function (sim, d, dt, ph, u) {
                const n = d.npc;
                const m = n.ai.mem;
                if (u < 0.3) {
                    const mv = Math.min(m.dist, m.sp * dt);
                    m.dist -= mv;
                    n.x = clampX(n.x + n.facing * mv);
                    n.vx = n.facing * m.sp;
                } else { n.vx = 0; }
                while (m.hits < 3 && u >= 0.05 + m.hits * 0.1) {
                    m.hits++;
                    if (inFront(d, 90)) hurt(sim, d, 1, { from: n.x, blockable: true, kb: 60, invuln: 0.08 });
                }
                return false;
            }
        },
        merchant_b: {
            act: function (sim, d) { d.npc.ai.mem.n = 0; },
            tick: function (sim, d, dt, ph, u) {
                const m = d.npc.ai.mem;
                const total = ph === 2 ? TABLE.merchant.p2.bombs : 1;
                while (m.n < total && u >= m.n * 0.35) {
                    m.n++;
                    lob(d, { kind: 'bomb', dmg: 0, blockable: false, land: 'blast' }, clampX(d.hero.x + (rnd(sim) - 0.5) * 80), 0.7);
                }
                return false;
            }
        },
        merchant_c: {
            act: function (sim, d) { const n = d.npc; n.hidden = 1.0; burst(d, 'smoke', n.x, n.y - 90, 16, { color: '#d1d5db', speed: 140 }); },
            tick: function (sim, d, dt, ph, u) {
                const n = d.npc;
                const m = n.ai.mem;
                if (!m.back && u >= 1.0) {
                    m.back = true;
                    n.hidden = 0;
                    n.x = clampX(2 * d.hero.x - n.x);
                    faceHero(d);
                    burst(d, 'smoke', n.x, n.y - 90, 16, { color: '#d1d5db', speed: 140 });
                }
                if (ph === 2 && m.back && !m.threw && u >= 1.45) {
                    m.threw = true;
                    faceHero(d);
                    const a = aimAt(n, d.hero);
                    addShot(d, { kind: 'dagger', x: n.x + n.facing * 30, y: chest(n), vx: Math.cos(a) * 520, vy: Math.sin(a) * 520, dmg: 1, life: 2 });
                }
                return false;
            }
        },
        chief_a: {
            tick: function (sim, d, dt, ph, u) {
                const m = d.npc.ai.mem;
                if (!m.hit && u >= 0.12) {
                    m.hit = true;
                    if (inFront(d, 150)) hurt(sim, d, 3, { from: d.npc.x, blockable: true, kb: 120 });
                    burst(d, 'dust', d.npc.x + d.npc.facing * 120, d.npc.y, 10, { color: '#d9c9a8', speed: 220 });
                    if (ph === 2) [-1, 1].forEach(function (s) { addZone(d, { kind: 'shock', x: d.npc.x + d.npc.facing * 100, w: 50, h: 36, t: 3, dmg: 2, vx: s * 420 }); });
                }
                return false;
            }
        },
        chief_b: {
            can: function (d, ph) { return count(d, 'kobold') < (ph === 2 ? TABLE.chief.p2.cap : 2); },
            act: function (sim, d, ph) {
                const cap = ph === 2 ? TABLE.chief.p2.cap : 2;
                const n = ph === 2 ? TABLE.chief.p2.kobolds : 1;
                for (let i = 0; i < n && count(d, 'kobold') < cap; i++) addMinion(d, { kind: 'kobold', x: clampX(d.npc.x + d.npc.facing * (70 + i * 50)), hp: 4, speed: 160 });
                burst(d, 'dust', d.npc.x, d.npc.y - 160, 12, { color: '#f87171', speed: 180 });
            }
        },
        chief_c: {
            act: function (sim, d) { const m = d.npc.ai.mem; faceHero(d); m.dir = d.npc.facing; m.hit = false; },
            tick: function (sim, d, dt) {
                const n = d.npc;
                const m = n.ai.mem;
                n.x += m.dir * 540 * dt;
                n.vx = m.dir * 540;
                if (!m.hit && Math.abs(d.hero.x - n.x) < 46 + HERO_HW && d.hero.y > n.y - 150) {
                    const r = hurt(sim, d, 3, { from: n.x, blockable: true, kb: 160 });
                    if (r !== 'ignored') m.hit = true;
                    if (r === 'block') { n.vx = 0; stagger(d, 1.0); return true; }
                }
                if (n.x <= ARENA.left + 30 || n.x >= ARENA.right - 30) {
                    n.x = clampX(n.x);
                    n.vx = 0;
                    Dm().shake(d, 8);
                    burst(d, 'shard', n.x, n.y - 120, 12, { color: '#d9c9a8', speed: 240 });
                    stagger(d, 0.8);
                    return true;
                }
                return false;
            }
        },
        seer_a: {
            act: function (sim, d) { addZone(d, { kind: 'vortex', x: d.npc.x, w: 120, h: 200, t: 2.0, dmg: 1, tick: 0.4, vx: 120 * sgn(d.hero.x - d.npc.x), pull: 140 }); }
        },
        seer_b: {
            act: function (sim, d, ph) {
                const n = ph === 2 ? TABLE.seer.p2.clones : 2;
                const offs = [-170, 170, 300];
                for (let i = 0; i < n; i++) addMinion(d, { kind: 'clone', x: clampX(d.npc.x + offs[i]), hp: 1, t: 4.0, speed: 100 });
                burst(d, 'rune', d.npc.x, d.npc.y - 100, 14, { color: '#fbbf24', speed: 170 });
            },
            tick: function (sim, d, dt, ph, u) {
                const m = d.npc.ai.mem;
                if (!m.swap && u >= 2.0) {
                    m.swap = true;
                    const cl = d.minions.filter(function (c) { return c.kind === 'clone'; });
                    if (cl.length) {
                        const c = cl[Math.floor(rnd(sim) * cl.length) % cl.length];
                        const x = c.x;
                        c.x = d.npc.x;
                        d.npc.x = x;
                        faceHero(d);
                        burst(d, 'rune', x, d.npc.y - 100, 12, { color: '#fbbf24', speed: 170 });
                    }
                }
                return false;
            }
        },
        seer_c: {
            tell: function (sim, d, ph, dur) { addZone(d, { kind: 'beam', x: clampX(d.hero.x), w: 80, h: 540, warn: dur, t: 0.45, dmg: 3, tellOf: true }); },
            tick: function (sim, d, dt, ph, u) {
                const m = d.npc.ai.mem;
                if (ph === 2 && !m.b2 && u >= 0.45) {
                    m.b2 = true;
                    addZone(d, { kind: 'beam', x: clampX(d.hero.x), w: 80, h: 540, warn: 0.45, t: 0.45, dmg: 3 });
                }
                return false;
            }
        },
        alchemist_a: {
            act: function (sim, d) {
                const dist = Math.abs(d.hero.x - d.npc.x);
                lob(d, { kind: 'acid', dmg: 2, land: 'puddle' }, clampX(d.hero.x), Math.max(0.45, Math.min(0.9, dist / 420)));
            }
        },
        alchemist_b: {
            can: function (d) { return count(d, 'clockwork') < 2; },
            act: function (sim, d) { addMinion(d, { kind: 'clockwork', x: clampX(d.npc.x + d.npc.facing * 60), hp: 3, speed: 130, fuse: -1 }); }
        },
        alchemist_c: {
            tell: function (sim, d, ph, dur) {
                const xs = ph === 2 ? [d.hero.x, d.hero.x - 200, d.hero.x + 200] : [d.hero.x, d.hero.x + (rnd(sim) < 0.5 ? -200 : 200)];
                xs.forEach(function (x) { addZone(d, { kind: 'steam', x: clampX(x), w: 70, h: 300, warn: dur, t: 0.6, dmg: 2, tellOf: true }); });
            }
        }
    };

    function usable(d, key, ph) {
        const s = SPAWN[d.npcId + '_' + key];
        return !s || !s.can || s.can(d, ph);
    }

    // A landed shot: bombs start a fuse blast (r100), acid leaves a puddle (and bites when it lands on the hero).
    function land(sim, d, s) {
        if (s.land === 'blast') {
            addZone(d, { kind: 'blast', x: s.x, w: 200, h: 140, warn: 0.6, t: 0.25, dmg: 3 });
        } else if (s.land === 'puddle') {
            if (!s.spent && Math.abs(d.hero.x - s.x) < 55 && d.hero.y > ARENA.floorY - 80) hurt(sim, d, s.dmg, { from: s.x, blockable: true, kb: 80 });
            addZone(d, { kind: 'puddle', x: s.x, w: 130, h: 30, t: 2.5, dmg: 1, tick: 0.5 });
            burst(d, 'smoke', s.x, ARENA.floorY - 10, 8, { color: '#84cc16', speed: 90 });
        }
    }

    // Minion AI. Returns false when the minion is gone.
    function stepMinion(sim, d, m, dt) {
        const h = d.hero;
        m.cd = Math.max(0, m.cd - dt);
        if (m.kind === 'clone') {
            m.t -= dt;
            if (m.t <= 0) { burst(d, 'rune', m.x, m.y - 80, 8, { color: '#fbbf24', speed: 120 }); return false; }
        }
        const dx = h.x - m.x;
        if (m.kind === 'clockwork' && m.fuse >= 0) {
            m.vx = 0;
            m.fuse -= dt;
            if (m.fuse <= 0) {
                if (Math.abs(h.x - m.x) <= 70 + HERO_HW && h.y > ARENA.floorY - 100) hurt(sim, d, 2, { from: m.x, blockable: false, kb: 160 });
                burst(d, 'spark', m.x, m.y - 30, 14, { color: '#fb923c', speed: 260 });
                Dm().shake(d, 6);
                return false;
            }
            return true;
        }
        m.vx = sgn(dx) * m.speed;
        m.x = clampX(m.x + m.vx * dt);
        if (m.kind === 'clockwork') {
            if (Math.abs(dx) < 70) m.fuse = 0.5;
        } else if (m.cd <= 0 && Math.abs(dx) < 16 + HERO_HW && h.y > m.y - 60) {
            const r = hurt(sim, d, 1, { from: m.x, blockable: true, kb: 100 });
            if (r !== 'ignored') m.cd = 0.8;
        }
        return true;
    }

    root.RunenpfadDuelPatterns = {
        ARENA: ARENA,
        G: G,
        MIN_TELL: MIN_TELL,
        HERO_HW: HERO_HW,
        HERO_H: HERO_H,
        TABLE: TABLE,
        SPAWN: SPAWN,
        tellTime: tellTime,
        actTime: actTime,
        usable: usable,
        land: land,
        stepMinion: stepMinion,
        addShot: addShot,
        clampX: clampX,
        faceHero: faceHero
    };
})(typeof window !== 'undefined' ? window : globalThis);
