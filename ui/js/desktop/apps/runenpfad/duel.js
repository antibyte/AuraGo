/* Runenpfad close-up duel simulation: hero controller, NPC AI scheduler, shots, zones, minions, damage and events. DOM-free, seeded. */
(function (root) {
    'use strict';

    const P = root.RunenpfadDuelPatterns;
    const ARENA = P.ARENA;
    const G = P.G;
    const HW = P.HERO_HW;
    const HH = P.HERO_H;
    const INTRO = 0.8;
    const OUTRO_POSE = 1.4;
    const OUTRO_WIPE = 0.6;

    function pushEvent(sim, type, detail) { root.RunenpfadPhysics.pushEvent(sim, type, detail); }
    function rnd(sim) { return root.RunenpfadPhysics.rand(sim); }
    function sgn(v) { return v < 0 ? -1 : 1; }
    function fxApi() { return root.RunenpfadFx || null; }
    function burst(d, kind, x, y, n, o) { if (d.fx && fxApi()) fxApi().burst(d.fx, kind, x, y, n, o); }
    function shake(d, amount) { if (d.fx && fxApi()) fxApi().shake(d.fx, amount); }
    function ov(a, b) { return a.l < b.r && a.r > b.l && a.t < b.b && a.b > b.t; }

    function heroBox(d) { const h = d.hero; return { l: h.x - HW, r: h.x + HW, t: h.y - HH, b: h.y }; }
    function npcBox(d) {
        const n = d.npc;
        const T = P.TABLE[d.npcId];
        return { l: n.x - T.hw, r: n.x + T.hw, t: n.y - (T.height - 30), b: n.y };
    }

    function create(sim, npcId, opts) {
        const T = P.TABLE[npcId];
        if (!T) return null;
        const o = opts || {};
        const stats = sim.stats;
        const rune = o.rune || (sim.player.rune && sim.player.rune.timeLeft > 0 ? sim.player.rune.kind : null);
        const maxHp = 8 + 2 * stats.maxHp;
        const seed = (sim.rngState >>> 0) ^ 0x5bd1e995;
        return {
            npcId: npcId, heroId: sim.heroId, phase: 'intro', phaseT: 0, t: 0, result: null, provoked: !!o.provoked,
            hero: {
                x: 260, y: ARENA.floorY, vx: 0, vy: 0, facing: 1, onGround: true, hp: maxHp, maxHp: maxHp, hit: 2 + stats.damage,
                state: 'idle', stateT: 0, combo: 0, comboT: 0, attackCd: 0, rollCd: 0, invuln: 0, rune: rune, specialUsed: false,
                airJumps: stats.maxAirJumps, lock: 0, queued: 0, swung: true, swingMul: 1,
                speed: stats.speed * 1.15, jumpVel: stats.jumpVel * 1.25, ranged: sim.heroId === 'ranger'
            },
            npc: {
                x: 700, y: ARENA.floorY, vx: 0, vy: 0, facing: -1, hp: T.hp, maxHp: T.hp, phase: 1, invuln: 0, shield: 0, frozen: 0, hidden: 0, fly: false, clip: 'idle',
                ai: { state: 'idle', pattern: null, t: 0, gap: 1.1, last: [], used: {}, poise: 0, hitT: [], poiseImm: 0, mem: {}, hurtDur: 0, tellDur: 0, actDur: 0 }
            },
            shots: [], zones: [], minions: [],
            fx: fxApi() ? fxApi().createSystem(seed) : null,
            freeze: 0, introTime: INTRO, outroTime: OUTRO_POSE + OUTRO_WIPE
        };
    }

    function done(d) { return d.phase === 'outro' && d.phaseT >= OUTRO_POSE + OUTRO_WIPE; }

    // Iris state for the renderer: which side is drawn and how far the wipe covers the screen (0 clear .. 1 closed).
    function wipe(d) {
        if (d.phase === 'intro') return d.phaseT < 0.4 ? { side: 'level', cover: d.phaseT / 0.4 } : { side: 'arena', cover: Math.max(0, 1 - (d.phaseT - 0.4) / 0.4) };
        if (d.phase === 'outro' && d.phaseT >= OUTRO_POSE) {
            const k = (d.phaseT - OUTRO_POSE) / OUTRO_WIPE;
            return k < 0.5 ? { side: 'arena', cover: k * 2 } : { side: 'level', cover: Math.max(0, 2 - k * 2) };
        }
        return { side: 'arena', cover: 0 };
    }

    function setState(h, s) { if (h.state !== s) { h.state = s; h.stateT = 0; } }

    // ---- damage ----

    function endDuel(sim, d, result) {
        if (d.phase === 'outro') return;
        d.result = result;
        d.phase = 'outro';
        d.phaseT = 0;
        d.freeze = 0;
        d.shots = [];
        d.zones = [];
        d.minions = [];
        const n = d.npc;
        n.hidden = 0;
        n.shield = 0;
        n.fly = false;
        n.vx = 0;
        n.ai.state = result === 'won' ? 'defeat' : 'idle';
        n.ai.mem = {};
        setState(d.hero, result === 'won' ? 'victory' : 'defeat');
        d.hero.vx = 0;
        shake(d, 10);
    }

    // Returns 'hit' | 'block' | 'ignored'. Never touches sim.player.
    function hurtHero(sim, d, dmg, o) {
        const h = d.hero;
        const op = o || {};
        if (d.phase !== 'fight' || h.hp <= 0 || h.invuln > 0) return 'ignored';
        const from = op.from == null ? d.npc.x : op.from;
        const dir = from >= h.x ? 1 : -1;
        const kb = op.kb == null ? 140 : op.kb;
        if (op.blockable && h.state === 'block' && (h.facing === dir || Math.abs(from - h.x) < 12)) {
            h.invuln = Math.max(h.invuln, 0.12);
            h.vx = -dir * 60 * 3.2;
            pushEvent(sim, 'duel_block', { by: 'hero' });
            burst(d, 'spark', h.x + dir * 30, h.y - 90, 6, { color: '#93c5fd', speed: 200 });
            return 'block';
        }
        h.hp = Math.max(0, h.hp - dmg);
        pushEvent(sim, 'duel_hit', { target: 'hero', dmg: dmg });
        burst(d, 'spark', h.x, h.y - 100, 8, { color: '#fca5a5', speed: 220 });
        if (op.soft) {
            h.invuln = Math.max(h.invuln, op.invuln == null ? 0.2 : op.invuln);
        } else {
            h.invuln = Math.max(h.invuln, op.invuln == null ? 0.6 : op.invuln);
            h.lock = 0.28;
            h.queued = 0;
            h.swung = true;
            setState(h, 'hurt');
            h.vx = -dir * kb * 3.2;
            d.freeze = Math.max(d.freeze, 0.05);
            shake(d, 5);
        }
        if (h.hp <= 0) endDuel(sim, d, 'lost');
        return 'hit';
    }

    function interrupt(d) {
        const n = d.npc;
        d.zones = d.zones.filter(function (z) { return !(z.tellOf && z.warn > 0); });
        n.fly = false;
        n.ai.mem = {};
        n.ai.pattern = null;
    }

    function stagger(d, secs) {
        const n = d.npc;
        interrupt(d);
        n.ai.state = 'hurt';
        n.ai.t = 0;
        n.ai.hurtDur = secs;
        n.vx = 0;
    }

    // Returns 'hit' | 'block' | 'ignored'. `o.bypass`: specials ignore blocks and the rune shield.
    function hurtNpc(sim, d, dmg, o) {
        const n = d.npc;
        const ai = n.ai;
        const op = o || {};
        if (d.phase !== 'fight' || n.hp <= 0 || n.hidden > 0 || n.invuln > 0) return 'ignored';
        if (n.shield > 0 && !op.bypass) {
            pushEvent(sim, 'duel_block', { by: 'npc' });
            burst(d, 'rune', n.x, n.y - 110, 5, { color: '#93c5fd', speed: 150 });
            return 'block';
        }
        if (!op.bypass && n.frozen <= 0 && (ai.state === 'idle' || ai.state === 'walk') && ((op.from == null ? d.hero.x : op.from) - n.x) * n.facing > 0 && rnd(sim) < (n.phase === 2 ? 0.35 : 0.25)) {
            interrupt(d);
            ai.state = 'block';
            ai.t = 0;
            ai.hurtDur = 0.3;
            n.vx = 0;
            pushEvent(sim, 'duel_block', { by: 'npc' });
            burst(d, 'spark', n.x + n.facing * 30, n.y - 110, 6, { color: '#fde68a', speed: 200 });
            return 'block';
        }
        n.hp = Math.max(0, n.hp - dmg);
        pushEvent(sim, 'duel_hit', { target: 'npc', dmg: dmg });
        burst(d, 'spark', n.x, n.y - 110, 8, { color: '#fde68a', speed: 240 });
        d.freeze = Math.max(d.freeze, op.freeze == null ? 0.045 : op.freeze);
        shake(d, 3);
        if (n.hp <= 0) { endDuel(sim, d, 'won'); return 'hit'; }
        ai.hitT.push(d.t);
        while (ai.hitT.length && d.t - ai.hitT[0] > 1.6) ai.hitT.shift();
        ai.poise = ai.hitT.length;
        if (ai.poise >= 3 && ai.poiseImm <= 0 && (ai.state === 'idle' || ai.state === 'walk' || ai.state === 'tell' || ai.state === 'recover')) {
            stagger(d, 0.5);
            ai.hitT.length = 0;
            ai.poise = 0;
            ai.poiseImm = 2.0;
        }
        return 'hit';
    }

    function hurtMinion(d, m, dmg) {
        m.hp -= dmg;
        burst(d, 'spark', m.x, m.y - 40, 5, { color: '#fde68a', speed: 160 });
    }

    // ---- hero ----

    function special(sim, d) {
        const h = d.hero;
        const n = d.npc;
        if (!h.rune || h.specialUsed) return;
        h.specialUsed = true;
        h.lock = 0.35;
        h.vx = 0;
        setState(h, 'special');
        pushEvent(sim, 'duel_special', { kind: h.rune });
        if (h.rune === 'fire') {
            P.addShot(d, { kind: 'fire', owner: 'hero', x: h.x + h.facing * 30, y: h.y - 100, vx: h.facing * 700, dmg: 10, blockable: false, pierce: true, life: 1.6, r: 24 });
        } else if (h.rune === 'frost') {
            if (n.hidden <= 0) n.frozen = 1.6;
            burst(d, 'shard', n.x, n.y - 110, 14, { color: '#7ee8fa', speed: 200 });
        } else if (h.rune === 'storm') {
            if (hurtNpc(sim, d, 8, { bypass: true, from: h.x }) === 'hit') burst(d, 'spark', n.x, n.y - 120, 18, { color: '#fde047', speed: 300 });
        } else if (h.rune === 'magnet') {
            d.shots = d.shots.filter(function (s) { return s.owner !== 'npc'; });
            burst(d, 'rune', h.x, h.y - 100, 14, { color: '#f0abfc', speed: 220 });
        }
    }

    function attack(sim, d) {
        const h = d.hero;
        setState(h, 'attack');
        h.queued = 0;
        h.stateT = 0;
        if (h.ranged) {
            h.lock = 0.16;
            h.attackCd = 0.24;
            h.swung = true;
            P.addShot(d, { kind: 'bolt', owner: 'hero', x: h.x + h.facing * 30, y: h.y - 90, vx: h.facing * 760, dmg: h.hit, blockable: false, life: 1.4, r: 10 });
        } else {
            h.combo = h.comboT > 0 ? h.combo % 3 + 1 : 1;
            h.comboT = 0.65;
            h.lock = 0.28;
            h.attackCd = h.combo === 3 ? 0.55 : 0.28;
            h.swung = false;
            h.swingMul = h.combo === 3 ? 1.5 : 1;
        }
    }

    function meleeHit(sim, d) {
        const h = d.hero;
        const f = h.facing;
        const box = { l: Math.min(h.x - f * 12, h.x + f * 130), r: Math.max(h.x - f * 12, h.x + f * 130), t: h.y - HH, b: h.y };
        const dmg = Math.round(h.hit * h.swingMul);
        d.minions.forEach(function (m) { if (ov(box, { l: m.x - 18, r: m.x + 18, t: m.y - 100, b: m.y })) hurtMinion(d, m, dmg); });
        d.minions = d.minions.filter(function (m) { return m.hp > 0; });
        burst(d, 'trail', h.x + f * 80, h.y - 100, 4, { color: '#e5e7eb', speed: 120 });
        if (ov(box, npcBox(d))) hurtNpc(sim, d, dmg, { from: h.x });
    }

    function stepHero(sim, d, dt, input, live) {
        const h = d.hero;
        h.stateT += dt;
        h.attackCd = Math.max(0, h.attackCd - dt);
        h.rollCd = Math.max(0, h.rollCd - dt);
        h.invuln = Math.max(0, h.invuln - dt);
        h.comboT = Math.max(0, h.comboT - dt);
        h.queued = Math.max(0, h.queued - dt);
        if (h.lock > 0) h.lock = Math.max(0, h.lock - dt);
        const dir = (input.right ? 1 : 0) - (input.left ? 1 : 0);
        const ended = h.state === 'victory' || h.state === 'defeat';
        if (live && !ended) {
            if (input.attackPressed) h.queued = 0.25;
            if (h.lock <= 0 && (h.state === 'attack' || h.state === 'hurt' || h.state === 'roll' || h.state === 'special')) setState(h, 'idle');
            if (h.lock <= 0) {
                if (input.talkPressed && input.talkPressed !== 'up' && h.rune && !h.specialUsed) {
                    special(sim, d);
                } else if (input.dashPressed && h.rollCd <= 0 && h.onGround) {
                    h.rollDir = dir || h.facing;
                    h.facing = h.rollDir;
                    h.lock = 0.34;
                    h.rollCd = 0.75;
                    h.invuln = Math.max(h.invuln, 0.30);
                    setState(h, 'roll');
                    pushEvent(sim, 'duel_roll');
                    burst(d, 'dust', h.x, h.y, 6, { color: '#d9c9a8', speed: 120 });
                } else if (h.queued > 0 && h.attackCd <= 0) {
                    if (dir) h.facing = dir;
                    attack(sim, d);
                }
            }
            if (h.state === 'attack' && !h.swung && h.stateT >= 0.09) {
                h.swung = true;
                meleeHit(sim, d);
            }
            const free = h.lock <= 0;
            if (free) {
                if (input.down && h.onGround) {
                    setState(h, 'block');
                    if (dir) h.facing = dir;
                    h.vx -= sgn(h.vx) * Math.min(Math.abs(h.vx), 1100 * dt);
                } else {
                    if (dir) { h.facing = dir; h.vx = dir * h.speed; } else { h.vx = 0; }
                    if (input.jumpPressed) {
                        if (h.onGround) { h.vy = -h.jumpVel; h.onGround = false; } else if (h.airJumps > 0) { h.airJumps--; h.vy = -h.jumpVel * 0.92; }
                    }
                    setState(h, !h.onGround ? 'jump' : (dir ? 'run' : 'idle'));
                }
                if (!(input.up || input.jump || input.jumpPressed) && h.vy < -240) h.vy = -240;
            } else if (h.state === 'roll') {
                h.vx = h.rollDir * 560;
            } else if (h.state === 'attack') {
                h.vx = dir * h.speed * 0.35;
            } else if (h.state === 'special') {
                h.vx = 0;
            } else if (h.state === 'hurt') {
                h.vx -= sgn(h.vx) * Math.min(Math.abs(h.vx), 1100 * dt);
            }
        } else {
            h.vx -= sgn(h.vx) * Math.min(Math.abs(h.vx), 1100 * dt);
        }
        h.vy += G * dt;
        h.y += h.vy * dt;
        h.x = Math.max(ARENA.left + HW, Math.min(ARENA.right - HW, h.x + h.vx * dt));
        if (h.y >= ARENA.floorY) {
            h.y = ARENA.floorY;
            h.vy = 0;
            if (!h.onGround) burst(d, 'dust', h.x, h.y, 4, { color: '#d9c9a8', speed: 90 });
            h.onGround = true;
            h.airJumps = sim.stats.maxAirJumps;
        } else {
            h.onGround = false;
        }
    }

    // ---- NPC ----

    function pickPattern(sim, d) {
        const T = P.TABLE[d.npcId];
        const n = d.npc;
        const ai = n.ai;
        const dist = Math.abs(d.hero.x - n.x);
        const keys = ['a', 'b', 'c'];
        const ws = keys.map(function (k) {
            const range = T.patterns[k].range;
            if (!P.usable(d, k, n.phase)) return 0;
            let w = range === 'melee' ? (dist <= 260 ? 3 : 1) : range === 'ranged' ? (dist >= 220 ? 3 : 1.2) : 2;
            if (!ai.used[k]) w *= 3;
            const l = ai.last;
            if (l.length >= 2 && l[0] === k && l[1] === k) return 0;
            if (l.length && l[l.length - 1] === k) w *= 0.5;
            return w;
        });
        let tot = ws.reduce(function (a, b) { return a + b; }, 0);
        if (tot <= 0) { ws[0] = 1; ws[1] = 1; ws[2] = 1; tot = 3; }
        let r = rnd(sim) * tot;
        let pick = keys.length - 1;
        for (let i = 0; i < ws.length; i++) {
            if (r < ws[i]) { pick = i; break; }
            r -= ws[i];
        }
        const k = keys[pick];
        ai.used[k] = true;
        ai.last.push(k);
        if (ai.last.length > 2) ai.last.shift();
        return k;
    }

    function beginTell(sim, d) {
        const n = d.npc;
        const ai = n.ai;
        const key = ai.pattern;
        const dur = P.tellTime(d.npcId, key, n.phase);
        ai.state = 'tell';
        ai.t = 0;
        ai.tellDur = dur;
        ai.mem = {};
        n.vx = 0;
        P.faceHero(d);
        const sp = P.SPAWN[d.npcId + '_' + key];
        if (sp.tell) sp.tell(sim, d, n.phase, dur);
        pushEvent(sim, 'duel_tell', { npcId: d.npcId, pattern: d.npcId + '_' + key });
    }

    function beginAct(sim, d) {
        const n = d.npc;
        const ai = n.ai;
        const key = ai.pattern;
        ai.state = 'act';
        ai.t = 0;
        ai.actDur = P.actTime(d.npcId, key, n.phase);
        d.zones.forEach(function (z) { z.tellOf = false; });
        const sp = P.SPAWN[d.npcId + '_' + key];
        if (sp.act) sp.act(sim, d, n.phase);
        pushEvent(sim, 'duel_attack', { npcId: d.npcId, pattern: d.npcId + '_' + key });
    }

    function toRecover(d) {
        const n = d.npc;
        n.ai.state = 'recover';
        n.ai.t = 0;
        n.vx = 0;
        n.fly = false;
    }

    function toIdle(d, gap) {
        const ai = d.npc.ai;
        ai.state = 'idle';
        ai.t = 0;
        ai.gap = gap;
        ai.pattern = null;
        d.npc.vx = 0;
    }

    function stepNpc(sim, d, dt, live) {
        const n = d.npc;
        const ai = n.ai;
        const T = P.TABLE[d.npcId];
        const h = d.hero;
        n.invuln = Math.max(0, n.invuln - dt);
        n.shield = Math.max(0, n.shield - dt);
        ai.poiseImm = Math.max(0, ai.poiseImm - dt);
        if (live && n.frozen > 0) {
            n.frozen = Math.max(0, n.frozen - dt);
            n.vx = 0;
            n.clip = 'hurt';
        } else if (live && n.phase === 1 && n.hp <= n.maxHp * 0.5 && ai.state !== 'act') {
            interrupt(d);
            n.phase = 2;
            ai.state = 'enraged';
            ai.t = 0;
            n.invuln = 1.0;
            n.vx = 0;
            pushEvent(sim, 'duel_phase', { npcId: d.npcId });
            if (d.fx && fxApi()) fxApi().ring(d.fx, n.x, n.y - 100, { color: '#f87171', grow: 320 });
            shake(d, 8);
        } else if (live) {
            ai.t += dt;
            const mul = n.phase === 2 ? 1.2 : 1;
            const dx = h.x - n.x;
            if (ai.state === 'idle') {
                n.vx = 0;
                P.faceHero(d);
                if (ai.t >= ai.gap) { ai.pattern = pickPattern(sim, d); ai.state = 'walk'; ai.t = 0; }
            } else if (ai.state === 'walk') {
                const pat = T.patterns[ai.pattern];
                const dist = Math.abs(dx);
                let want = 0;
                if (pat.range === 'melee') want = dist > 130 ? sgn(dx) : 0;
                else if (pat.range === 'ranged') want = dist < 260 ? -sgn(dx) : dist > 420 ? sgn(dx) : 0;
                if ((want < 0 && n.x <= ARENA.left + 40) || (want > 0 && n.x >= ARENA.right - 40)) want = 0;
                P.faceHero(d);
                if (want === 0 || ai.t > 1.6) {
                    beginTell(sim, d);
                } else {
                    n.vx = want * T.speed * mul;
                    n.x = P.clampX(n.x + n.vx * dt);
                }
            } else if (ai.state === 'tell') {
                P.faceHero(d);
                const sp = P.SPAWN[d.npcId + '_' + ai.pattern];
                if (sp.tellTick) sp.tellTick(sim, d, dt, n.phase, ai.t);
                if (ai.t >= ai.tellDur) beginAct(sim, d);
            } else if (ai.state === 'act') {
                const sp = P.SPAWN[d.npcId + '_' + ai.pattern];
                const end = sp.tick ? sp.tick(sim, d, dt, n.phase, ai.t) : false;
                if (ai.state === 'act' && (end === true || ai.t >= ai.actDur)) toRecover(d);
            } else if (ai.state === 'recover') {
                n.vx = 0;
                if (ai.t >= P.TABLE[d.npcId].patterns[ai.pattern].rec) toIdle(d, n.phase === 2 ? 0.75 : 1.1);
            } else if (ai.state === 'hurt' || ai.state === 'block') {
                if (ai.t >= ai.hurtDur) toIdle(d, ai.state === 'block' ? 0.3 : 0.6);
            } else if (ai.state === 'enraged') {
                if (ai.t >= 1.0) toIdle(d, 0.6);
            }
            n.clip = ai.state === 'tell' ? ai.pattern + '_tell' : ai.state === 'act' ? ai.pattern : ai.state === 'recover' ? 'idle' : ai.state;
        } else {
            n.clip = ai.state === 'defeat' ? 'defeat' : (d.result === 'lost' ? 'taunt' : 'idle');
        }
        if (!n.fly) {
            n.vy += G * dt;
            n.y += n.vy * dt;
            if (n.y >= ARENA.floorY) { n.y = ARENA.floorY; n.vy = 0; }
        }
        if (!(live && n.frozen > 0) && ai.state === 'idle') P.faceHero(d);
    }

    // ---- shots, zones, minions ----

    function stepShots(sim, d, dt) {
        const h = d.hero;
        const hb = heroBox(d);
        d.shots = d.shots.filter(function (s) {
            s.life -= dt;
            if (s.homing && s.owner === 'npc') {
                let diff = Math.atan2(h.y - 100 - s.y, h.x - s.x) - Math.atan2(s.vy, s.vx);
                while (diff > Math.PI) diff -= Math.PI * 2;
                while (diff < -Math.PI) diff += Math.PI * 2;
                const a = Math.atan2(s.vy, s.vx) + Math.max(-s.turn * dt, Math.min(s.turn * dt, diff));
                s.vx = Math.cos(a) * s.homing;
                s.vy = Math.sin(a) * s.homing;
            }
            s.vy += s.g * dt;
            s.x += s.vx * dt;
            s.y += s.vy * dt;
            if (s.life <= 0 || s.x < ARENA.left - 40 || s.x > ARENA.right + 40 || s.y < -80) return false;
            if (s.y >= ARENA.floorY) {
                if (s.land) { s.y = ARENA.floorY; P.land(sim, d, s); }
                return false;
            }
            if (s.owner === 'npc') {
                if (s.dmg > 0 && ov({ l: s.x - s.r, r: s.x + s.r, t: s.y - s.r, b: s.y + s.r }, hb)) {
                    const r = d.hero.hp > 0 ? Duel.hurtHero(sim, d, s.dmg, { from: h.x - sgn(s.vx) * 40, blockable: s.blockable, kb: 90 }) : 'ignored';
                    if (r !== 'ignored') {
                        if (s.land) { s.spent = true; P.land(sim, d, s); }
                        return false;
                    }
                }
                return true;
            }
            const box = { l: s.x - s.r, r: s.x + s.r, t: s.y - s.r, b: s.y + s.r };
            const seen = s.seen || (s.seen = []);
            let alive = true;
            d.minions.forEach(function (m) {
                if (alive && seen.indexOf(m) < 0 && ov(box, { l: m.x - 18, r: m.x + 18, t: m.y - 100, b: m.y })) {
                    seen.push(m);
                    hurtMinion(d, m, s.dmg);
                    if (!s.pierce) alive = false;
                }
            });
            d.minions = d.minions.filter(function (m) { return m.hp > 0; });
            if (alive && seen.indexOf(d.npc) < 0 && ov(box, npcBox(d))) {
                if (d.npc.shield > 0 && !s.pierce) {
                    s.owner = 'npc';
                    s.vx = -s.vx;
                    s.dmg = 1;
                    s.blockable = true;
                    s.kind = 'hex';
                    pushEvent(sim, 'duel_block', { by: 'npc' });
                    return true;
                }
                const r = hurtNpc(sim, d, s.dmg, { from: s.x - sgn(s.vx) * 40, bypass: s.pierce });
                if (r !== 'ignored') {
                    if (!s.pierce) return false;
                    seen.push(d.npc);
                }
            }
            return alive;
        });
    }

    function zoneOverlap(z, hb) {
        return hb.r > z.x - z.w / 2 && hb.l < z.x + z.w / 2 && hb.b > ARENA.floorY - z.h;
    }

    function stepZones(sim, d, dt) {
        const h = d.hero;
        d.zones = d.zones.filter(function (z) {
            if (z.warn > 0) {
                z.warn = Math.max(0, z.warn - dt);
                if (z.warn > 0) return true;
                if (z.kind === 'marker') return false;
                if (z.kind === 'blast') {
                    z.hit = true;
                    if (Math.abs(h.x - z.x) <= z.w / 2 && h.y > ARENA.floorY - z.h) Duel.hurtHero(sim, d, z.dmg, { from: z.x, blockable: false, kb: 160 });
                    burst(d, 'spark', z.x, ARENA.floorY - 30, 16, { color: '#fb923c', speed: 280 });
                    shake(d, 6);
                }
                return true;
            }
            z.t -= dt;
            z.x += z.vx * dt;
            if (z.kind === 'shock' && (z.x < ARENA.left - 80 || z.x > ARENA.right + 80)) return false;
            if (z.kind === 'vortex' && ((z.x < ARENA.left + 60 && z.vx < 0) || (z.x > ARENA.right - 60 && z.vx > 0))) z.vx = -z.vx;
            const hb = heroBox(d);
            if (z.pull && Math.abs(h.x - z.x) < z.w / 2 + 90 && h.state !== 'roll' && d.phase === 'fight') h.x = Math.max(ARENA.left + HW, Math.min(ARENA.right - HW, h.x + sgn(z.x - h.x) * z.pull * dt));
            if (z.dmg > 0 && !z.hit && zoneOverlap(z, hb)) {
                if (z.tick > 0) {
                    z.tickT -= dt;
                    if (z.tickT <= 0 && Duel.hurtHero(sim, d, z.dmg, { from: z.x, blockable: false, soft: true, invuln: 0.2 }) !== 'ignored') z.tickT = z.tick;
                } else if (Duel.hurtHero(sim, d, z.dmg, { from: z.x, blockable: z.blockable, kb: 120 }) !== 'ignored') {
                    z.hit = true;
                }
            }
            return z.t > 0;
        });
    }

    function stepMinions(sim, d, dt) {
        d.minions = d.minions.filter(function (m) { return m.hp > 0 && P.stepMinion(sim, d, m, dt); });
    }

    function step(sim, d, dt, input) {
        const inp = input || {};
        if (d.freeze > 0) {
            d.freeze = Math.max(0, d.freeze - dt);
            if (d.fx && fxApi()) fxApi().update(d.fx, dt);
            return;
        }
        d.t += dt;
        d.phaseT += dt;
        if (d.fx && fxApi()) fxApi().update(d.fx, dt);
        if (d.phase === 'intro') {
            stepHero(sim, d, dt, inp, false);
            stepNpc(sim, d, dt, false);
            if (d.phaseT >= INTRO) { d.phase = 'fight'; d.phaseT = 0; }
            return;
        }
        if (d.phase === 'fight') {
            stepHero(sim, d, dt, inp, true);
            if (d.phase === 'fight') stepNpc(sim, d, dt, true);
            if (d.phase === 'fight') stepShots(sim, d, dt);
            if (d.phase === 'fight') stepZones(sim, d, dt);
            if (d.phase === 'fight') stepMinions(sim, d, dt);
            return;
        }
        stepHero(sim, d, dt, inp, false);
        stepNpc(sim, d, dt, false);
    }

    const Duel = {
        ARENA: ARENA,
        INTRO: INTRO,
        OUTRO_POSE: OUTRO_POSE,
        OUTRO_WIPE: OUTRO_WIPE,
        create: create,
        step: step,
        done: done,
        wipe: wipe,
        heroBox: heroBox,
        npcBox: npcBox,
        hurtHero: hurtHero,
        hurtNpc: hurtNpc,
        stagger: stagger,
        shake: shake
    };
    root.RunenpfadDuel = Duel;
})(typeof window !== 'undefined' ? window : globalThis);
