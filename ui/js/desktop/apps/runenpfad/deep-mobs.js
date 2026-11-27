/* Runenpfad deeps monsters: the emberling (a crawling spark that hops at the hero) and the drakeling (a small flyer that keeps its
   distance and spits aimed fireballs). Both register in RunenpfadInteriorMobs.DEFS with an ext.step; hits use the default path. DOM-free.

   Renderer-facing extras: m.hop = { fx, fy, tx, ty, k 0..1 } | null (emberling: take-off and landing feet, k = progress, set from the
   moment the landing point locks until the hop is over). Shots of a drakeling are plain it.shots entries of kind 'fireball'. */
(function (root) {
    'use strict';

    const HERO_R = 9;
    const HURT_TIME = 0.28;
    const LOCK = 0.4;

    function P() { return root.RunenpfadPhysics; }
    function I() { return root.RunenpfadInterior; }
    function M() { return root.RunenpfadInteriorMobs; }
    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }

    const EMBERLING = { hp: 2, r: 10, speed: 60, tell: 0.7, atk: 0.35, cd: 1.8, range: 150, hop: { dist: 120, rad: 26 } };
    const DRAKELING = { hp: 3, r: 11, speed: 90, tell: 0.7, atk: 0.2, cd: 2.4, range: 300, fly: true, keep: [150, 220], shot: { speed: 160, r: 7, life: 2.4 } };

    function heroC(it) { return { x: it.hero.x, y: it.hero.y - 7 }; }

    function aimAt(m, it) {
        const h = heroC(it);
        const dx = h.x - m.x;
        const dy = h.y - (m.y - 7);
        const d = Math.hypot(dx, dy) || 1;
        m.ax = dx / d;
        m.ay = dy / d;
        return d;
    }

    function setState(m, s) { m.state = s; m.t = 0; }

    // Knockback and the shared cooldown; returns the grid the mob walks on.
    function begin(it, m, d, dt) {
        if (m.cd > 0) m.cd -= dt;
        const grid = d.fly ? it.room.walls : it.room.solid;
        if (m.kt > 0) {
            m.kt -= dt;
            M().slide(it.room, grid, m, m.kx * dt, m.ky * dt, d.r * 0.8, d.r * 1.1);
        }
        return grid;
    }

    function wakes(it, m) {
        if (m.aggro) return true;
        const h = heroC(it);
        if (Math.hypot(h.x - m.x, h.y - (m.y - 7)) < M().AGGRO) m.aggro = true;
        return m.aggro;
    }

    // ---- emberling ----

    function lockHop(m, it, d) {
        const h = heroC(it);
        const dx = h.x - m.x;
        const dy = h.y - (m.y - 7);
        const dist = Math.hypot(dx, dy) || 1;
        const run = Math.min(dist, d.hop.dist);
        m.hop = { fx: m.x, fy: m.y, tx: m.x + dx / dist * run, ty: m.y + dy / dist * run, k: 0 };
    }

    function emberlingStep(sim, it, m, dt) {
        const d = M().DEFS.emberling;
        const grid = begin(it, m, d, dt);
        if (!wakes(it, m)) return;
        const h = heroC(it);
        const dist = aimAt(m, it);
        if (m.state === 'hurt') {
            m.hop = null;
            if (m.t >= HURT_TIME) setState(m, 'move');
            return;
        }
        if (m.state === 'tell') {
            if (m.t < LOCK) {
                aimAt(m, it);
                lockHop(m, it, d);
            }
            if (m.t >= d.tell) {
                setState(m, 'attack');
                pushEvent(sim, 'in_mob_attack', { kind: m.kind, id: m.id });
            }
        } else if (m.state === 'attack') {
            const hp = m.hop;
            if (hp) {
                hp.k = Math.min(1, m.t / d.atk);
                M().slide(it.room, grid, m, hp.fx + (hp.tx - hp.fx) * hp.k - m.x, hp.fy + (hp.ty - hp.fy) * hp.k - m.y, d.r * 0.8, d.r * 1.1);
            }
            if (m.t >= d.atk) {
                if (Math.hypot(h.x - m.x, h.y - (m.y - 7)) < d.hop.rad + HERO_R) I().hurtHero(sim, it, m.kind, m.x, m.y, 1);
                m.hop = null;
                setState(m, 'move');
                m.cd = d.cd;
            }
        } else {
            if (dist > m.r + HERO_R + 4) {
                M().slide(it.room, grid, m, m.ax * d.speed * dt, m.ay * d.speed * dt, d.r * 0.8, d.r * 1.1);
                if (Math.abs(m.ax) > 0.05) m.face = m.ax > 0 ? 1 : -1;
            }
            if (m.cd <= 0 && dist <= d.range) {
                setState(m, 'tell');
                m.vx = 0;
                m.vy = 0;
                lockHop(m, it, d);
                pushEvent(sim, 'in_mob_tell', { kind: m.kind, id: m.id });
            }
        }
        if (m.state !== 'hurt' && Math.hypot(h.x - m.x, h.y - (m.y - 7)) < m.r + HERO_R) I().hurtHero(sim, it, m.kind, m.x, m.y, 1);
    }

    // ---- drakeling ----

    function drakelingStep(sim, it, m, dt) {
        const d = M().DEFS.drakeling;
        const grid = begin(it, m, d, dt);
        if (!wakes(it, m)) return;
        const dist = aimAt(m, it);
        if (m.state === 'hurt') {
            if (m.t >= HURT_TIME) setState(m, 'move');
            return;
        }
        if (m.state === 'tell') {
            if (m.t < d.tell * 0.6) aimAt(m, it);
            if (m.t >= d.tell) {
                setState(m, 'attack');
                pushEvent(sim, 'in_mob_attack', { kind: m.kind, id: m.id });
                const ang = Math.atan2(m.ay, m.ax);
                it.shots.push({ x: m.x, y: m.y - 7, vx: Math.cos(ang) * d.shot.speed, vy: Math.sin(ang) * d.shot.speed, r: d.shot.r, dmg: 1, life: d.shot.life, kind: 'fireball' });
            }
        } else if (m.state === 'attack') {
            if (m.t >= d.atk) {
                setState(m, 'move');
                m.cd = d.cd;
            }
        } else {
            const k = dist > d.keep[1] ? 1 : (dist < d.keep[0] ? -1 : 0);
            const side = k === 0 ? 0.55 : 0.25;
            const vx = (m.ax * k - m.ay * m.strafe * side) * d.speed;
            const vy = (m.ay * k + m.ax * m.strafe * side) * d.speed;
            if (!M().slide(it.room, grid, m, vx * dt, vy * dt, d.r * 0.8, d.r * 1.1) && k === 0) m.strafe = -m.strafe;
            if (Math.abs(m.ax) > 0.05) m.face = m.ax > 0 ? 1 : -1;
            if (m.cd <= 0 && dist <= d.range) {
                setState(m, 'tell');
                m.vx = 0;
                m.vy = 0;
                pushEvent(sim, 'in_mob_tell', { kind: m.kind, id: m.id });
            }
        }
    }

    function register() {
        const mobs = M();
        if (!mobs || !mobs.DEFS) return;
        mobs.DEFS.emberling = Object.assign({}, EMBERLING, { ext: { step: emberlingStep } });
        mobs.DEFS.drakeling = Object.assign({}, DRAKELING, { ext: { step: drakelingStep } });
    }
    register();

    root.RunenpfadDeepMobs = {
        EMBERLING: EMBERLING,
        DRAKELING: DRAKELING,
        emberlingStep: emberlingStep,
        drakelingStep: drakelingStep
    };
})(typeof window !== 'undefined' ? window : globalThis);
