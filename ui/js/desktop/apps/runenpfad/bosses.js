/* Runenpfad boss attack patterns: telegraphed state machine with a real phase 2 per world boss. DOM-free, seeded via Physics.rand. */
(function (root) {
    'use strict';

    const INTRO_RANGE = 520;
    const INTRO_IDLE = 1.2;
    const SHOCK_COLOR = '#fbbf24';

    function P() {
        return root.RunenpfadPhysics;
    }

    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }
    function fxBurst(sim, kind, x, y, count, opts) { P().fxBurst(sim, kind, x, y, count, opts); }
    function fxShake(sim, amount) { P().fxShake(sim, amount); }
    function rand(sim) { return P().rand(sim); }

    function clamp(v, lo, hi) {
        return v < lo ? lo : v > hi ? hi : v;
    }

    function groundTop(sim) {
        return sim.stage.groundY;
    }

    function spawnProj(sim, props) {
        sim.projectiles.push(Object.assign({ fromEnemy: true, damage: 1, vy: 0, life: 3, dead: false, noDeflect: true }, props));
    }

    // Ballistic launch velocity that lands on (tx, ty) after `flight` seconds under `gravity`.
    function aimArc(x0, y0, tx, ty, flight, gravity) {
        return { vx: (tx - x0) / flight, vy: (ty - y0 - 0.5 * gravity * flight * flight) / flight };
    }

    // Ground boss movement with tile collision; returns true when a wall or the arena edge stopped it.
    function moveGround(e, dt, api, stage, vx) {
        const bw = 30 * e.def.scale;
        const bh = 42 * e.def.scale;
        const prevY = e.y;
        e.vx = vx;
        e.x += vx * dt;
        const hitWall = vx !== 0 && api.resolveHorizontalTiles(stage, e, bw, bh);
        let bound = false;
        if (e.x <= e.minX) { e.x = e.minX; bound = vx < 0; }
        if (e.x >= e.maxX) { e.x = e.maxX; bound = vx > 0; }
        e.vy = Math.min(api.MAX_FALL, e.vy + api.GRAVITY * dt);
        e.y += e.vy * dt;
        api.resolveVerticalTiles(stage, e, bw, bh, prevY, false);
        return !!(hitWall || bound);
    }

    function flyTo(e, tx, ty, speed, dt) {
        const dx = tx - e.x;
        const dy = ty - e.y;
        const len = Math.hypot(dx, dy);
        if (len > 1) {
            const step = Math.min(len, speed * dt);
            e.x += (dx / len) * step;
            e.y += (dy / len) * step;
        }
        e.x = clamp(e.x, e.minX, e.maxX);
    }

    function hoverY(e) {
        return e.spawnY - (e.def.dragon ? 72 : 96) + Math.sin(e.animTime * 2.2) * 12;
    }

    function summonedAlive(sim, defId) {
        return sim.stage.enemies.filter(function (o) { return o.summoned && !o.dead && o.defId === defId; }).length;
    }

    function summon(sim, e, ai, defId, x) {
        const L = root.RunenpfadLevels;
        if (!L || typeof L.createEnemy !== 'function') return;
        ai.n += 1;
        const flying = !!(L.ENEMY_DEFS[defId] && L.ENEMY_DEFS[defId].flying);
        const m = L.createEnemy(defId, x, groundTop(sim), e.minX, e.maxX, { id: 'sum_' + ai.n + '_' + e.id, dir: x >= e.x ? 1 : -1, shootTimer: 1.8 });
        m.summoned = true;
        if (flying) m.y = groundTop(sim) - 90;
        sim.stage.enemies.push(m);
        fxBurst(sim, 'rune', x, m.y - 30, 12, { color: '#c084fc', speed: 160, life: 0.6 });
    }

    function shockPair(sim, x, speeds) {
        const y = groundTop(sim) - 14;
        speeds.forEach(function (spd) {
            [-1, 1].forEach(function (dir) {
                spawnProj(sim, { x: x + dir * 30, y: y, vx: dir * spd, color: SHOCK_COLOR, kind: 'shock', radius: 14, groundHug: true, life: 2.6 });
            });
        });
    }

    function landSlam(sim, e, speeds, hurtRadius) {
        const x = e.x;
        const y = groundTop(sim);
        pushEvent(sim, 'boss_slam', { x: x, y: y });
        fxBurst(sim, 'dust', x, y, 16, { color: '#e8d8b8', speed: 240, arc: Math.PI, angle: Math.PI / 2, life: 0.55 });
        fxBurst(sim, 'shard', x, y - 6, 12, { color: '#ffd08a', speed: 280, life: 0.7 });
        P().ring(sim, x, y - 4, { color: SHOCK_COLOR, size: 18, grow: 360, life: 0.5 });
        fxShake(sim, 9);
        shockPair(sim, x, speeds);
        const p = sim.player;
        if (hurtRadius && Math.abs(p.x - x) < hurtRadius && p.y > y - 70) {
            root.RunenpfadCombat.hurtPlayer(sim, 1, x, 'impact-stone');
        }
    }

    function telegraph(e, kind, x, y, dur, extra) {
        e.telegraph = Object.assign({ kind: kind, x: x, y: y, t: 0, dur: dur }, extra || {});
    }

    // ---- attacks: tele(sim,e,ai) sets markers, begin() fires, run() returns true when finished ----
    const ATTACKS = {
        // Boar: lane marker, then a straight charge that stuns it against the arena wall.
        charge: {
            dur: 0.85,
            recover: 0.5,
            tele: function (sim, e, ai) {
                ai.dir = sim.player.x >= e.x ? 1 : -1;
                const len = ai.dir > 0 ? e.maxX - e.x : e.x - e.minX;
                telegraph(e, 'charge', e.x, groundTop(sim), ATTACKS.charge.dur, { dir: ai.dir, len: Math.max(60, len), h: 54 });
                e.facing = ai.dir;
            },
            begin: function (sim, e, ai) {
                ai.passes = e.phase === 2 ? 2 : 1;
                ai.sub = 'run';
                ai.speed = e.phase === 2 ? 540 : 430;
                ai.s = 0;
            },
            run: function (sim, e, dt, api, ai) {
                ai.s += dt;
                if (ai.sub === 'run') {
                    const hit = moveGround(e, dt, api, sim.stage, ai.dir * ai.speed);
                    e.facing = ai.dir;
                    if (hit || ai.s > 2.4) {
                        ai.sub = 'stun';
                        ai.s = 0;
                        pushEvent(sim, 'boss_slam', { x: e.x + ai.dir * 30, y: groundTop(sim) });
                        fxBurst(sim, 'dust', e.x + ai.dir * 30, groundTop(sim), 12, { color: '#e8d8b8', speed: 200, arc: Math.PI, angle: Math.PI / 2, life: 0.5 });
                        fxShake(sim, 8);
                    }
                    return false;
                }
                moveGround(e, dt, api, sim.stage, 0);
                if (ai.s < 0.8) return false;
                ai.passes -= 1;
                if (ai.passes > 0) {
                    ai.dir = -ai.dir;
                    ai.sub = 'run';
                    ai.s = 0;
                    return false;
                }
                return true;
            }
        },
        // Boar: leaps onto the marked spot and sends shock waves along the floor.
        ground_pound: {
            dur: 0.8,
            recover: 0.7,
            tele: function (sim, e, ai) {
                ai.tx = clamp(sim.player.x, e.minX + 20, e.maxX - 20);
                telegraph(e, 'pound', ai.tx, groundTop(sim), ATTACKS.ground_pound.dur, { w: 100 });
                e.facing = ai.tx >= e.x ? 1 : -1;
            },
            begin: function (sim, e, ai) {
                e.vy = -620;
                e.onGround = false;
                ai.hv = clamp((ai.tx - e.x) / 0.84, -460, 460);
                ai.s = 0;
            },
            run: function (sim, e, dt, api, ai) {
                ai.s += dt;
                moveGround(e, dt, api, sim.stage, ai.hv);
                if (ai.s > 0.3 && e.onGround) {
                    landSlam(sim, e, e.phase === 2 ? [250, 360] : [270], 0);
                    return true;
                }
                return ai.s > 2;
            }
        },
        // Goblin: parabolic leap onto the marked spot.
        leap: {
            dur: 0.7,
            recover: 0.6,
            tele: function (sim, e, ai) {
                ai.tx = clamp(sim.player.x, e.minX + 20, e.maxX - 20);
                telegraph(e, 'leap', ai.tx, groundTop(sim), ATTACKS.leap.dur, { w: 80 });
                e.facing = ai.tx >= e.x ? 1 : -1;
            },
            begin: function (sim, e, ai) {
                e.vy = -650;
                e.onGround = false;
                ai.hv = clamp((ai.tx - e.x) / 0.88, -480, 480);
                ai.s = 0;
            },
            run: function (sim, e, dt, api, ai) {
                ai.s += dt;
                moveGround(e, dt, api, sim.stage, ai.hv);
                if (ai.s > 0.3 && e.onGround) {
                    fxBurst(sim, 'dust', e.x, groundTop(sim), 10, { color: '#d9c9a8', speed: 180, arc: Math.PI, angle: Math.PI / 2, life: 0.45 });
                    fxShake(sim, 5);
                    return true;
                }
                return ai.s > 2;
            }
        },
        // Goblin: arcing spear volley that lands on marked spots around the hero.
        volley: {
            dur: 0.75,
            recover: 0.7,
            tele: function (sim, e, ai) {
                const n = e.phase === 2 ? 5 : 3;
                const marks = [];
                for (let i = 0; i < n; i++) marks.push({ x: clamp(sim.player.x + (i - (n - 1) / 2) * 70, 100, sim.stage.width - 100), y: groundTop(sim) });
                ai.marks = marks;
                telegraph(e, 'volley', e.x, groundTop(sim), ATTACKS.volley.dur, { marks: marks });
                e.facing = sim.player.x >= e.x ? 1 : -1;
            },
            begin: function (sim, e, ai) {
                ai.s = 0;
                ai.fired = 0;
                e.telegraph = null;
            },
            run: function (sim, e, dt, api, ai) {
                ai.s += dt;
                moveGround(e, dt, api, sim.stage, 0);
                while (ai.fired < ai.marks.length && ai.s >= ai.fired * 0.14) {
                    const m = ai.marks[ai.fired];
                    const x0 = e.x + e.facing * 24;
                    const y0 = e.y - 70;
                    const v = aimArc(x0, y0, m.x, m.y - 6, 0.9, 900);
                    spawnProj(sim, { x: x0, y: y0, vx: v.vx, vy: v.vy, gravity: 900, color: '#fde68a', kind: 'spear', radius: 7, life: 2.4 });
                    pushEvent(sim, 'enemy_attack', { id: e.id, defId: e.defId, kind: 'shot', x: e.x, y: e.y });
                    e.attackTimer = 0.45;
                    e.attackKind = 'shot';
                    ai.fired += 1;
                }
                return ai.fired >= ai.marks.length && ai.s > ai.marks.length * 0.14 + 0.2;
            }
        },
        // Goblin / quadcopter: call reinforcements to the marked spots.
        summon: {
            dur: 0.8,
            recover: 0.6,
            tele: function (sim, e, ai) {
                const marks = [-130, 130].map(function (off) { return { x: clamp(e.x + off, e.minX + 10, e.maxX - 10), y: groundTop(sim) }; });
                ai.marks = marks;
                telegraph(e, 'summon', e.x, groundTop(sim), ATTACKS.summon.dur, { marks: marks });
            },
            begin: function (sim, e, ai) {
                const isQuad = e.defId === 'boss_quad';
                const defId = isQuad ? 'quadcopter' : 'goblin';
                const cap = isQuad ? 3 : 2;
                ai.marks.forEach(function (m) {
                    if (summonedAlive(sim, defId) < cap) summon(sim, e, ai, defId, m.x);
                });
                e.telegraph = null;
            },
            run: function () { return true; }
        },
        // Orb: bullet ring with a gap; the gap rotates between waves in phase 2.
        ring: {
            dur: 0.75,
            recover: 0.8,
            tele: function (sim, e, ai) {
                const p = sim.player;
                ai.gap = Math.atan2((p.y - 22) - (e.y - 30), p.x - e.x);
                telegraph(e, 'ring', e.x, e.y - 30, ATTACKS.ring.dur, { gap: ai.gap });
            },
            begin: function (sim, e, ai) {
                ai.waves = e.phase === 2 ? 3 : 1;
                ai.wave = 0;
                ai.s = 0;
                e.telegraph = null;
            },
            run: function (sim, e, dt, api, ai) {
                ai.s += dt;
                if (ai.wave < ai.waves && ai.s >= ai.wave * 0.5) {
                    const base = ai.gap + ai.wave * 0.95;
                    const spd = e.phase === 2 ? 205 : 175;
                    for (let i = 0; i < 12; i++) {
                        if (i === 0 || i === 1 || i === 11) continue;
                        const a = base + (i * Math.PI * 2) / 12;
                        spawnProj(sim, { x: e.x, y: e.y - 30, vx: Math.cos(a) * spd, vy: Math.sin(a) * spd, color: '#a78bfa', kind: 'orb', radius: 7, life: 3.2 });
                    }
                    fxBurst(sim, 'rune', e.x, e.y - 30, 14, { color: '#a78bfa', speed: 170, life: 0.5 });
                    P().ring(sim, e.x, e.y - 30, { color: '#a78bfa', size: 14, grow: 260, life: 0.45 });
                    ai.wave += 1;
                }
                return ai.wave >= ai.waves && ai.s > (ai.waves - 1) * 0.5 + 0.3;
            }
        },
        // Orb: a vertical laser beam locked onto where the hero stood (two beams in phase 2).
        laser: {
            dur: 0.9,
            recover: 0.9,
            tele: function (sim, e, ai) {
                const xs = [sim.player.x];
                if (e.phase === 2) xs.push(clamp(sim.player.x + (rand(sim) < 0.5 ? -1 : 1) * 170, 80, sim.stage.width - 80));
                ai.xs = xs;
                telegraph(e, 'beam', xs[0], groundTop(sim), ATTACKS.laser.dur, { xs: xs, y0: e.y - 20, w: 44 });
            },
            begin: function (sim, e, ai) {
                ai.xs.forEach(function (bx) {
                    sim.tempHazards.push({ kind: 'laser', x: bx - 22, y: e.y - 20, w: 44, h: groundTop(sim) - (e.y - 20) + 8, t: 0.5, dur: 0.5, damage: 1 });
                    fxBurst(sim, 'spark', bx, groundTop(sim), 10, { color: '#f0abfc', speed: 200, life: 0.4 });
                });
                fxShake(sim, 4);
                ai.s = 0;
                e.telegraph = null;
            },
            run: function (sim, e, dt, api, ai) {
                ai.s += dt;
                return ai.s >= 0.5;
            }
        },
        // Quadcopter: marked dive that ends in a floor shock wave; phase 2 dives twice.
        dive: {
            dur: 0.9,
            recover: 0.4,
            tele: function (sim, e, ai) {
                ai.tx = clamp(sim.player.x, e.minX + 20, e.maxX - 20);
                telegraph(e, 'dive', ai.tx, groundTop(sim), ATTACKS.dive.dur, { w: 110 });
            },
            begin: function (sim, e, ai) {
                ai.sub = 'drop';
                ai.passes = e.phase === 2 ? 2 : 1;
                ai.s = 0;
                e.telegraph = null;
            },
            run: function (sim, e, dt, api, ai) {
                if (ai.sub === 'drop') {
                    e.x += clamp(ai.tx - e.x, -300 * dt, 300 * dt);
                    e.y += (e.phase === 2 ? 900 : 760) * dt;
                    if (e.y >= groundTop(sim)) {
                        e.y = groundTop(sim);
                        landSlam(sim, e, e.phase === 2 ? [280, 380] : [300], 80);
                        ai.sub = 'ground';
                        ai.s = 0.8;
                    }
                } else if (ai.sub === 'ground') {
                    ai.s -= dt;
                    if (ai.s <= 0) {
                        ai.passes -= 1;
                        ai.sub = ai.passes > 0 ? 'rise' : 'done';
                    }
                } else if (ai.sub === 'rise') {
                    const hy = hoverY(e);
                    flyTo(e, e.x, hy, 340, dt);
                    if (Math.abs(e.y - hy) < 8) {
                        ai.tx = clamp(sim.player.x, e.minX + 20, e.maxX - 20);
                        telegraph(e, 'dive', ai.tx, groundTop(sim), 0.6, { w: 110 });
                        pushEvent(sim, 'boss_telegraph', { kind: 'dive', x: ai.tx, y: groundTop(sim) });
                        ai.sub = 'mark';
                        ai.s = 0;
                    }
                } else if (ai.sub === 'mark') {
                    ai.s += dt;
                    e.telegraph.t = ai.s;
                    flyTo(e, ai.tx, hoverY(e), 300, dt);
                    if (ai.s >= 0.6) {
                        e.telegraph = null;
                        ai.sub = 'drop';
                    }
                }
                return ai.sub === 'done';
            }
        },
        // Mech: arcing missiles that blast the marked spots on the floor.
        missiles: {
            dur: 0.95,
            recover: 0.8,
            tele: function (sim, e, ai) {
                const offs = e.phase === 2 ? [-250, -150, -50, 50, 150, 250] : [-150, -50, 50, 150];
                const marks = offs.map(function (o) { return { x: clamp(sim.player.x + o, 100, sim.stage.width - 100), y: groundTop(sim) }; });
                ai.marks = marks;
                telegraph(e, 'missiles', e.x, groundTop(sim), ATTACKS.missiles.dur, { marks: marks });
                e.facing = sim.player.x >= e.x ? 1 : -1;
            },
            begin: function (sim, e, ai) {
                ai.s = 0;
                ai.fired = 0;
            },
            run: function (sim, e, dt, api, ai) {
                ai.s += dt;
                moveGround(e, dt, api, sim.stage, 0);
                if (e.telegraph) e.telegraph.t = e.telegraph.dur;
                while (ai.fired < ai.marks.length && ai.s >= ai.fired * 0.16) {
                    const m = ai.marks[ai.fired];
                    const x0 = e.x + e.facing * 18;
                    const y0 = e.y - 96;
                    const v = aimArc(x0, y0, m.x, m.y - 6, 1.1, 700);
                    spawnProj(sim, { x: x0, y: y0, vx: v.vx, vy: v.vy, gravity: 700, color: '#fb923c', kind: 'missile', radius: 9, blast: 62, life: 3 });
                    pushEvent(sim, 'enemy_attack', { id: e.id, defId: e.defId, kind: 'shot', x: e.x, y: e.y });
                    e.attackTimer = 0.45;
                    e.attackKind = 'shot';
                    ai.fired += 1;
                }
                return ai.fired >= ai.marks.length && ai.s > ai.marks.length * 0.16 + 1.2;
            }
        },
        // Mech: flamethrower that leaves a burning floor strip for a while.
        fire_strip: {
            dur: 0.7,
            recover: 0.8,
            tele: function (sim, e, ai) {
                ai.dir = sim.player.x >= e.x ? 1 : -1;
                e.facing = ai.dir;
                const len = e.phase === 2 ? 380 : 260;
                const x = ai.dir > 0 ? e.x + 40 : e.x - 40 - len;
                ai.rect = { x: x, y: groundTop(sim) - 40, w: len, h: 40 };
                telegraph(e, 'fire', x, ai.rect.y, ATTACKS.fire_strip.dur, { w: len, h: 40 });
            },
            begin: function (sim, e, ai) {
                sim.tempHazards.push({ kind: 'fire', x: ai.rect.x, y: ai.rect.y, w: ai.rect.w, h: ai.rect.h, t: 2.5, dur: 2.5, damage: 1 });
                fxBurst(sim, 'ember', ai.rect.x + ai.rect.w / 2, ai.rect.y + 20, 14, { color: '#fb923c', speed: 140, life: 0.8 });
                fxShake(sim, 4);
                e.telegraph = null;
            },
            run: function () { return true; }
        }
    };


    // ---- dragon boss (endless tower): low fire passes, arcing fireball volleys and a roar that calls drakelings ----
    const ARENA_PAD_L = 144;       // the arena floor reaches 3 tiles left of the boss range and 5 tiles right of it
    const ARENA_PAD_R = 240;
    const STRAFE_FIRE_H = 58;

    // Free flight toward a point without the arena clamp (the dragon enters and leaves from outside the arena).
    function flyFree(e, tx, ty, speed, dt) {
        const dx = tx - e.x;
        const dy = ty - e.y;
        const len = Math.hypot(dx, dy);
        if (len > 1) {
            const step = Math.min(len, speed * dt);
            e.x += (dx / len) * step;
            e.y += (dy / len) * step;
        }
    }

    function dragonKind(e) {
        return e.def.dragon || 'ember';
    }

    ATTACKS.strafe = {
        dur: 1.1,
        recover: 0.9,
        tele: function (sim, e, ai) {
            ai.dir = rand(sim) < 0.5 ? -1 : 1;
            ai.startX = ai.dir > 0 ? e.minX : e.maxX;
            ai.x1 = e.minX - ARENA_PAD_L;
            ai.x2 = e.maxX + ARENA_PAD_R;
            e.facing = ai.dir;
            telegraph(e, 'strafe', ai.startX, groundTop(sim) - STRAFE_FIRE_H, ATTACKS.strafe.dur, { dir: ai.dir, x1: ai.x1, x2: ai.x2, h: STRAFE_FIRE_H, variant: dragonKind(e) });
        },
        begin: function (sim, e, ai) {
            ai.passes = e.phase === 2 ? 2 : 1;
            ai.speed = e.phase === 2 ? 640 : 520;
            ai.sub = 'pass';
            ai.s = 0;
            ai.fireT = 0;
            e.telegraph = null;
            pushEvent(sim, 'dragon_breath', { x: e.x, y: groundTop(sim) - 30, kind: dragonKind(e) });
            fxShake(sim, 4);
        },
        run: function (sim, e, dt, api, ai) {
            const ground = groundTop(sim);
            e.facing = ai.dir;
            if (ai.sub === 'pass') {
                e.x += ai.dir * ai.speed * dt;
                e.y += clamp(ground - 72 - e.y, -560 * dt, 560 * dt);
                ai.fireT -= dt;
                if (ai.fireT <= 0) {
                    ai.fireT = 0.07;
                    const head = e.x + ai.dir * 80;
                    sim.tempHazards.push({ kind: 'dragonfire', variant: dragonKind(e), x: head - 50, y: ground - STRAFE_FIRE_H, w: 100, h: STRAFE_FIRE_H, t: 0.55, dur: 0.55, damage: 1 });
                    fxBurst(sim, dragonKind(e) === 'frost' ? 'dust' : 'ember', head, ground - 20, 3, { color: dragonKind(e) === 'frost' ? '#a5f3fc' : '#fb923c', speed: 120, arc: Math.PI, angle: -Math.PI / 2, life: 0.5 });
                }
                const exitX = ai.dir > 0 ? ai.x2 + 160 : ai.x1 - 160;
                if ((e.x - exitX) * ai.dir >= 0) {
                    ai.passes -= 1;
                    ai.sub = ai.passes > 0 ? 'turn' : 'return';
                    ai.s = 0.45;
                }
            } else if (ai.sub === 'turn') {
                ai.s -= dt;
                e.y += clamp(hoverY(e) - e.y, -400 * dt, 400 * dt);
                if (ai.s <= 0) {
                    ai.dir = -ai.dir;
                    ai.sub = 'pass';
                    pushEvent(sim, 'dragon_breath', { x: e.x, y: ground - 30, kind: dragonKind(e) });
                }
            } else {
                flyFree(e, clamp(e.x, e.minX, e.maxX), hoverY(e), 460, dt);
                if (Math.abs(e.x - clamp(e.x, e.minX, e.maxX)) < 6 && Math.abs(e.y - hoverY(e)) < 8) return true;
            }
            return false;
        }
    };

    ATTACKS.fireballs = {
        dur: 0.85,
        recover: 0.9,
        tele: function (sim, e, ai) {
            const n = e.phase === 2 ? 5 : 3;
            const marks = [];
            for (let i = 0; i < n; i++) marks.push({ x: clamp(sim.player.x + (i - (n - 1) / 2) * 80, 100, sim.stage.width - 100), y: groundTop(sim) });
            ai.marks = marks;
            telegraph(e, 'fireballs', e.x, groundTop(sim), ATTACKS.fireballs.dur, { marks: marks, variant: dragonKind(e) });
            e.facing = sim.player.x >= e.x ? 1 : -1;
        },
        begin: function (sim, e, ai) {
            ai.s = 0;
            ai.fired = 0;
            e.telegraph = null;
        },
        run: function (sim, e, dt, api, ai) {
            ai.s += dt;
            while (ai.fired < ai.marks.length && ai.s >= ai.fired * 0.2) {
                const m = ai.marks[ai.fired];
                const x0 = e.x + e.facing * 40;
                const y0 = e.y - 50;
                const v = aimArc(x0, y0, m.x, m.y - 6, 0.95, 800);
                spawnProj(sim, { x: x0, y: y0, vx: v.vx, vy: v.vy, gravity: 800, color: dragonKind(e) === 'frost' ? '#7dd3fc' : '#fb923c', kind: 'fireball', variant: dragonKind(e), radius: 10, blast: 56, life: 2.6 });
                pushEvent(sim, 'enemy_attack', { id: e.id, defId: e.defId, kind: 'fireball', x: e.x, y: e.y });
                e.attackTimer = 0.45;
                e.attackKind = 'shot';
                ai.fired += 1;
            }
            return ai.fired >= ai.marks.length && ai.s > ai.marks.length * 0.2 + 0.6;
        }
    };

    ATTACKS.roar = {
        dur: 0.9,
        recover: 1.0,
        tele: function (sim, e, ai) {
            ai.marks = [-140, 140].map(function (off) { return { x: clamp(e.x + off, e.minX + 10, e.maxX - 10), y: groundTop(sim) }; });
            telegraph(e, 'roar', e.x, groundTop(sim), ATTACKS.roar.dur, { marks: ai.marks, variant: dragonKind(e) });
        },
        begin: function (sim, e, ai) {
            pushEvent(sim, 'dragon_roar', { x: e.x, y: e.y, kind: dragonKind(e) });
            ai.marks.forEach(function (m) {
                if (summonedAlive(sim, 'drakeling') < 2) summon(sim, e, ai, 'drakeling', m.x);
            });
            P().ring(sim, e.x, e.y - 40, { color: '#fb923c', size: 18, grow: 300, life: 0.5 });
            fxShake(sim, 6);
            e.telegraph = null;
            ai.s = 0;
        },
        run: function (sim, e, dt, api, ai) {
            ai.s += dt;
            return ai.s >= 0.4;
        }
    };

    const TABLE = {
        boss_boar: { p1: ['charge', 'ground_pound'], p2: ['charge', 'ground_pound', 'charge'], keep: 190, speed: 0.45 },
        boss_goblin: { p1: ['leap', 'volley'], p2: ['leap', 'volley', 'summon'], keep: 300, speed: 0.5 },
        boss_orb: { p1: ['ring', 'laser'], p2: ['ring', 'laser', 'ring'], flying: true },
        boss_quad: { p1: ['dive', 'summon'], p2: ['dive', 'summon', 'dive'], flying: true },
        boss_mech: { p1: ['missiles', 'fire_strip'], p2: ['missiles', 'fire_strip', 'missiles'], keep: 300, speed: 0.4 },
        boss_dragon_ember: { p1: ['strafe', 'fireballs', 'roar', 'dive'], p2: ['strafe', 'fireballs', 'dive', 'roar', 'strafe'], flying: true },
        boss_dragon_frost: { p1: ['strafe', 'fireballs', 'roar', 'dive'], p2: ['strafe', 'fireballs', 'dive', 'roar', 'strafe'], flying: true }
    };

    function pickPattern(sim, e, ai, cfg) {
        let list = (e.phase === 2 ? cfg.p2 : cfg.p1).filter(function (name) {
            if (name === 'roar') return summonedAlive(sim, 'drakeling') < 2;
            if (name === 'summon') return summonedAlive(sim, e.defId === 'boss_quad' ? 'quadcopter' : 'goblin') < (e.defId === 'boss_quad' ? 3 : 2);
            return true;
        });
        const other = list.filter(function (name) { return name !== ai.last; });
        if (other.length) list = other;
        return list[Math.floor(rand(sim) * list.length)];
    }

    function idleMove(sim, e, dt, api, cfg, ai, active) {
        const p = sim.player;
        if (cfg.flying) {
            const hy = hoverY(e);
            const tx = active ? clamp(p.x + Math.sin(e.animTime * 0.9) * 140, e.minX, e.maxX) : e.spawnX;
            flyTo(e, tx, hy, e.def.speed * (e.phase === 2 ? 1.1 : 0.8), dt);
            e.facing = p.x >= e.x ? 1 : -1;
            return;
        }
        let vx = 0;
        if (active) {
            const dx = p.x - e.x;
            const dist = Math.abs(dx);
            const speed = e.def.speed * cfg.speed * (e.phase === 2 ? 1.3 : 1);
            if (dist > cfg.keep + 40) vx = Math.sign(dx) * speed;
            else if (cfg.keep > 220 && dist < cfg.keep - 70) vx = -Math.sign(dx) * speed;
            e.facing = dx >= 0 ? 1 : -1;
        }
        moveGround(e, dt, api, sim.stage, vx);
    }

    function update(sim, e, dt, api) {
        const cfg = TABLE[e.defId];
        if (!cfg) return false;
        const ai = e.ai || (e.ai = { state: 'wait', t: 0, pattern: null, step: 0, cool: 0, last: null, enraged: false, n: 0 });
        const p = sim.player;
        e.shootTimer = 99;
        if (e.phase === 2 && !ai.enraged) {
            ai.enraged = true;
            sim.notice = 'runenpfad.notice.boss_enraged';
        }
        const dist = Math.abs(p.x - e.x);

        if (ai.state === 'wait') {
            idleMove(sim, e, dt, api, cfg, ai, false);
            if (dist < INTRO_RANGE) {
                ai.state = 'intro';
                ai.t = 0;
                sim.bossIntro = { id: e.id, nameKey: e.def.bossNameKey, timer: 2.2 };
                pushEvent(sim, 'boss_intro', { id: e.id, nameKey: e.def.bossNameKey, defId: e.defId, x: e.x, y: e.y });
            }
        } else if (ai.state === 'intro') {
            idleMove(sim, e, dt, api, cfg, ai, false);
            ai.t += dt;
            if (ai.t >= INTRO_IDLE) {
                ai.state = 'idle';
                ai.cool = 0.5;
            }
        } else if (ai.state === 'idle') {
            idleMove(sim, e, dt, api, cfg, ai, true);
            ai.cool -= dt;
            if (ai.cool <= 0 && dist < 780) {
                ai.pattern = pickPattern(sim, e, ai, cfg);
                ai.state = 'telegraph';
                ai.t = 0;
                ATTACKS[ai.pattern].tele(sim, e, ai);
                const t = e.telegraph;
                pushEvent(sim, 'boss_telegraph', { kind: t ? t.kind : ai.pattern, x: t ? t.x : e.x, y: t ? t.y : e.y });
            }
        } else if (ai.state === 'telegraph') {
            const atk = ATTACKS[ai.pattern];
            ai.t += dt;
            if (e.telegraph) e.telegraph.t = ai.t;
            if (cfg.flying) {
                const tx = ai.tx != null && ai.pattern === 'dive' ? ai.tx : (ai.pattern === 'laser' ? ai.xs[0] : e.x);
                if (ai.pattern === 'strafe') flyTo(e, ai.startX, hoverY(e), 560, dt);
                else flyTo(e, tx, hoverY(e), 260, dt);
            } else {
                moveGround(e, dt, api, sim.stage, 0);
            }
            if (ai.t >= atk.dur) {
                ai.state = 'attack';
                ai.t = 0;
                atk.begin(sim, e, ai);
                pushEvent(sim, 'boss_attack', { pattern: ai.pattern, x: e.x, y: e.y });
                if (ai.pattern === 'missiles') pushEvent(sim, 'enemy_attack', { id: e.id, defId: e.defId, kind: 'shot', x: e.x, y: e.y });
            }
        } else if (ai.state === 'attack') {
            if (ATTACKS[ai.pattern].run(sim, e, dt, api, ai)) {
                ai.state = 'recover';
                ai.t = 0;
                ai.last = ai.pattern;
                e.telegraph = null;
                e.vx = 0;
            }
        } else if (ai.state === 'recover') {
            ai.t += dt;
            idleMove(sim, e, dt, api, cfg, ai, false);
            if (ai.t >= ATTACKS[ai.last].recover) {
                ai.state = 'idle';
                ai.cool = e.phase === 2 ? 0.55 : 0.9;
            }
        }

        if (!cfg.flying && ai.state !== 'attack' && ai.state !== 'telegraph') e.vx = 0;
        e.animState = e.hurtTimer > 0 ? 'hurt' : (Math.abs(e.vx) > 1 || ai.state === 'attack' ? 'move' : 'idle');
        return true;
    }

    // Defeat cleanup: reinforcements fall with the boss, markers and bullets vanish, the world slows down briefly.
    function onDefeat(sim, e) {
        e.telegraph = null;
        sim.bossIntro = null;
        sim.tempHazards = [];
        sim.projectiles.forEach(function (pr) { if (pr.fromEnemy) pr.dead = true; });
        sim.stage.enemies.forEach(function (o) {
            if (o.summoned && !o.dead) {
                o.dead = true;
                o.deathTimer = 0.4;
                o.animState = 'death';
                o.animTime = 0;
                fxBurst(sim, 'shard', o.x, o.y - 24, 8, { color: '#cfe8ff', speed: 200, life: 0.6 });
            }
        });
        const Fx = root.RunenpfadFx;
        if (Fx && sim.fx && typeof Fx.slowmo === 'function') Fx.slowmo(sim.fx, 1.1);
    }

    root.RunenpfadBosses = {
        PATTERNS: TABLE,
        update: update,
        onDefeat: onDefeat
    };
})(typeof window !== 'undefined' ? window : globalThis);
