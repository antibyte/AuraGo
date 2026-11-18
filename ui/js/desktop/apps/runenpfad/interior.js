/* Runenpfad interiors: top-down rooms, hero controller, interaction, chests, rest spots, room exits and KO. DOM-free. */
(function (root) {
    'use strict';

    const CELL = 48;
    const HW = 11;
    const HH = 14;
    const WIPE = 0.35;
    const TALK_R = 44;
    const HERO = {
        knight: { half: Math.PI / 3, reach: 52, dashT: 0.16, dashSpeed: 320, dashCool: 1.1 },
        ranger: { half: (35 * Math.PI) / 180, reach: 46, dashT: 0.18, dashSpeed: 420, dashCool: 0.7 }
    };

    function P() { return root.RunenpfadPhysics; }
    function R() { return root.RunenpfadPlaceRooms; }
    function M() { return root.RunenpfadInteriorMobs; }
    function N() { return root.RunenpfadNpcs; }
    function D() { return root.RunenpfadDialogue || null; }
    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }

    function placeState(sim, placeId) {
        const stage = sim.stage;
        if (!stage.placeState) stage.placeState = {};
        return stage.placeState[placeId] || (stage.placeState[placeId] = { cleared: {}, sold: {}, opened: {}, used: {} });
    }

    function cc(cx, cy) { return { x: (cx + 0.5) * CELL, y: (cy + 0.5) * CELL + 7 }; }

    function floatText(sim, text, color, icon) {
        const f = { x: sim.player.x, y: sim.player.y - 44, text: text, color: color, timer: 1 };
        if (icon) f.icon = icon;
        sim.floatingTexts.push(f);
    }

    // ---- room construction ----

    function npcBase(place, roomId) {
        let n = 0;
        for (let i = 0; i < place.rooms.length && place.rooms[i] !== roomId; i++) {
            n += R().ROOMS[place.rooms[i]].things.filter(function (t) { return t.npc; }).length;
        }
        return n;
    }

    function enterRoom(sim, it, roomId, atMarker) {
        const rooms = R();
        const lay = rooms.layout(roomId, sim);
        const d = lay.def;
        const ps = placeState(sim, it.placeId);
        const cleared = !!ps.cleared[roomId];
        const endless = sim.mode === 'endless';
        const loot = (sim.profile && sim.profile.loot) || [];
        const room = {
            id: roomId, def: d, cols: lay.cols, rows: lay.rows, w: lay.cols * CELL, h: lay.rows * CELL,
            solid: lay.solid, walls: lay.walls, props: lay.props, lights: lay.lights, exits: [], rests: []
        };
        it.room = room;
        it.roomId = roomId;
        it.mobs = [];
        it.shots = [];
        it.npcs = [];
        it.chests = [];
        it.pickups = [];
        it.prompt = null;
        it.leaving = null;
        it.exitArmed = false;
        it.cleared = false;
        const cells = function (m) { return lay.markers[m] || []; };
        Object.keys(d.exits || {}).forEach(function (m) {
            cells(m).forEach(function (c) { room.exits.push({ cx: c.cx, cy: c.cy, to: d.exits[m].to, at: d.exits[m].at || null }); });
        });
        let npcN = npcBase(it.place, roomId);
        let mobN = 0;
        let pickN = 0;
        let restN = 0;
        d.things.forEach(function (t) {
            if (t.npc) {
                const def = N() && N().NPCS[t.npc];
                if (!def) return;
                cells(t.at).forEach(function (c) {
                    if (t.if && D() && !D().evalCond(t.if, sim)) return;
                    const p = cc(c.cx, c.cy);
                    const ref = 'in_' + npcN++;
                    const npc = {
                        ref: ref, npcId: t.npc, x: p.x, y: p.y, meetingId: t.meeting || null, facing: 1, state: 'idle', mood: 'neutral',
                        near: false, marker: null, provoked: false, sold: ps.sold[ref] || (ps.sold[ref] = {}), hurtT: 0
                    };
                    room.solid[c.cy * room.cols + c.cx] = 1;
                    it.npcs.push(npc);
                    N().refresh(sim, npc);
                });
            } else if (t.mob && !cleared) {
                cells(t.at).forEach(function (c) {
                    const p = cc(c.cx, c.cy);
                    it.mobs.push(M().create(t.mob, 'm_' + mobN, p.x, p.y, mobN));
                    mobN++;
                });
            } else if (t.chest) {
                cells(t.at).forEach(function (c) {
                    const p = cc(c.cx, c.cy);
                    const repeat = !!t.repeat || endless;
                    const lootDef = endless ? { coins: 15 } : (t.loot || {});
                    const opened = !!ps.opened[t.chest] || (!repeat && loot.indexOf(t.chest) >= 0);
                    room.solid[c.cy * room.cols + c.cx] = 1;
                    it.chests.push({ id: t.chest, x: p.x, y: p.y, lootId: t.chest, loot: lootDef, locked: t.locked === 'clear' && !cleared, opened: opened, repeat: repeat });
                });
            } else if (t.pickup) {
                cells(t.at).forEach(function (c) {
                    const key = 'pk_' + roomId + '_' + pickN++;
                    if (ps.used[key]) return;
                    const p = cc(c.cx, c.cy);
                    it.pickups.push({ x: p.x, y: p.y, kind: t.pickup, taken: false, key: key });
                });
            } else if (t.rest) {
                cells(t.at).forEach(function (c) {
                    const p = cc(c.cx, c.cy);
                    room.rests.push({ x: p.x, y: p.y, key: 'rest_' + roomId + '_' + restN++ });
                });
            }
        });
        it.hadMobs = it.mobs.length > 0;
        it.cleared = cleared || !it.hadMobs;
        // The hero appears at the arrival marker (or the room spawn) and must step off any exit cell first.
        const marker = atMarker || d.spawn;
        const start = (marker && cells(marker)[0]) || { cx: 1, cy: 1 };
        const p = cc(start.cx, start.cy);
        const h = it.hero;
        h.x = p.x;
        h.y = p.y;
        h.vx = 0;
        h.vy = 0;
        h.moving = false;
        it.wipe = { t: 0, dir: 1 };
        pushEvent(sim, 'place_room', { roomId: roomId });
    }

    function create(sim, placeId, doorId) {
        const place = R().PLACES[placeId];
        if (!place) return null;
        const it = {
            placeId: placeId, place: place, roomId: null, room: null, doorId: doorId || null, t: 0,
            hero: { x: 0, y: 0, vx: 0, vy: 0, dir: 'up', moving: false, walkT: 0, attackT: 0, attackCool: 0, dashT: 0, dashCool: 0, invuln: 0, hurtT: 0, ax: 0, ay: -1, dx: 0, dy: 0, kx: 0, ky: 0, kt: 0 },
            mobs: [], shots: [], npcs: [], chests: [], pickups: [], floats: [], prompt: null, wipe: { t: 0, dir: 1 },
            cleared: false, result: null, leaving: null, exitArmed: false, hadMobs: false, prevNotice: sim.notice
        };
        enterRoom(sim, it, place.entry, null);
        return it;
    }

    // ---- hero damage and healing ----

    // Returns true when the hit landed (not while invulnerable, dashing or leaving).
    function hurtHero(sim, it, kind, fromX, fromY) {
        const h = it.hero;
        if (h.invuln > 0 || h.dashT > 0 || it.leaving) return false;
        sim.player.hp -= 1;
        h.invuln = 1;
        h.hurtT = 0.3;
        const dx = h.x - fromX;
        const dy = h.y - fromY;
        const dd = Math.hypot(dx, dy) || 1;
        h.kx = dx / dd * 180;
        h.ky = dy / dd * 180;
        h.kt = 0.15;
        pushEvent(sim, 'in_hit', { target: 'hero', kind: kind });
        if (sim.player.hp <= 0) startLeave(it, 'ko');
        return true;
    }

    function startLeave(it, result, to, at) {
        if (it.leaving) return;
        it.leaving = { result: result, to: to || null, at: at || null, t: 0 };
        it.wipe = { t: 0, dir: -1 };
    }

    function heal(sim, amount, text, icon) {
        const p = sim.player;
        p.hp = Math.min(p.maxHp, p.hp + amount);
        floatText(sim, text, '#f87171', icon);
    }

    // ---- hero controller ----

    function swing(sim, it, cfg) {
        const h = it.hero;
        const def = sim.stats.def;
        h.attackT = def.attackDuration + 0.06;
        h.attackCool = def.attackCooldown;
        pushEvent(sim, 'in_attack', { hero: sim.heroId });
        const Runes = root.RunenpfadRunes;
        const dmg = Math.max(1, sim.stats.damage + (Runes ? Runes.damageBonus(sim) : 0));
        const cx = h.x;
        const cy = h.y - 7;
        it.mobs.slice().forEach(function (m) {
            if (m.state === 'dead') return;
            const dx = m.x - cx;
            const dy = (m.y - 7) - cy;
            const dist = Math.hypot(dx, dy);
            if (dist - m.r > cfg.reach) return;
            if (dist > 8) {
                const cos = Math.max(-1, Math.min(1, (h.ax * dx + h.ay * dy) / dist));
                if (Math.acos(cos) > cfg.half + Math.asin(Math.min(1, m.r / dist))) return;
            }
            M().hit(sim, it, m, dmg, dist > 1 ? dx / dist : h.ax, dist > 1 ? dy / dist : h.ay);
        });
    }

    function stepHero(sim, it, input, dt) {
        const h = it.hero;
        const cfg = HERO[sim.heroId === 'ranger' ? 'ranger' : 'knight'];
        ['attackT', 'attackCool', 'dashT', 'dashCool', 'invuln', 'hurtT'].forEach(function (k) { if (h[k] > 0) h[k] = Math.max(0, h[k] - dt); });
        let mx = (input.right ? 1 : 0) - (input.left ? 1 : 0);
        let my = (input.down ? 1 : 0) - (input.up ? 1 : 0);
        const len = Math.hypot(mx, my);
        if (len > 0) { mx /= len; my /= len; }
        h.moving = len > 0;
        if ((input.dashPressed || input.jumpPressed) && h.dashCool <= 0 && h.dashT <= 0) {
            h.dashT = cfg.dashT;
            h.dashCool = cfg.dashCool;
            h.dx = len > 0 ? mx : h.ax;
            h.dy = len > 0 ? my : h.ay;
            pushEvent(sim, 'in_dash');
        }
        let vx = mx * sim.stats.speed * 0.64;
        let vy = my * sim.stats.speed * 0.64;
        if (h.dashT > 0) {
            vx = h.dx * cfg.dashSpeed;
            vy = h.dy * cfg.dashSpeed;
        }
        if (h.kt > 0) {
            h.kt -= dt;
            vx += h.kx;
            vy += h.ky;
        }
        M().slide(it.room, it.room.solid, h, vx * dt, vy * dt, HW, HH);
        h.vx = vx;
        h.vy = vy;
        if (len > 0 && h.dashT <= 0) {
            h.ax = mx;
            h.ay = my;
            h.dir = Math.abs(mx) > Math.abs(my) ? (mx > 0 ? 'right' : 'left') : (my > 0 ? 'down' : 'up');
        }
        if (h.moving) {
            const before = Math.floor(h.walkT / 0.3);
            h.walkT += dt;
            if (Math.floor(h.walkT / 0.3) !== before) pushEvent(sim, 'in_step');
        }
        if (input.attackPressed && h.attackCool <= 0) swing(sim, it, cfg);
        if (input.healPressed) {
            const p = sim.player;
            if (p.potions > 0 && p.hp < p.maxHp) {
                p.potions -= 1;
                heal(sim, 2, '+2', 'heart');
            }
        }
    }

    // ---- loot, pickups, interaction ----

    function openChest(sim, it, chest) {
        const Places = root.RunenpfadPlaces;
        const ps = placeState(sim, it.placeId);
        if (chest.opened) {
            pushEvent(sim, 'in_chest', { lootId: chest.lootId, empty: true });
            sim.notice = 'runenpfad.notice.place_empty';
            return;
        }
        if (chest.locked) {
            pushEvent(sim, 'in_locked');
            sim.notice = 'runenpfad.notice.place_locked';
            return;
        }
        chest.opened = true;
        ps.opened[chest.lootId] = true;
        pushEvent(sim, 'in_chest', { lootId: chest.lootId, empty: false });
        if (Places) Places.takeLoot(sim, chest);
    }

    function collectPickups(sim, it) {
        const h = it.hero;
        const p = sim.player;
        const ps = placeState(sim, it.placeId);
        it.pickups.forEach(function (pk) {
            if (pk.taken || Math.hypot(pk.x - h.x, pk.y - (h.y - 7)) > 20) return;
            if (pk.kind === 'heart') {
                if (p.hp >= p.maxHp) return;
                heal(sim, 1, '+1', 'heart');
            } else if (pk.kind === 'coin') {
                N().addCoins(sim, 1);
                sim.stageCoins += 1;
                floatText(sim, '+1', '#fcd34d', 'coin');
            } else if (pk.kind === 'potion') {
                p.potions += 1;
                floatText(sim, '+1', '#c084fc', 'potion');
            }
            pk.taken = true;
            if (pk.key) ps.used[pk.key] = true;
            pushEvent(sim, 'in_pickup', { kind: pk.kind });
        });
    }

    function interact(sim, it, input) {
        const h = it.hero;
        const hx = h.x;
        const hy = h.y - 7;
        const ps = placeState(sim, it.placeId);
        let best = null;
        let bestD = TALK_R;
        const consider = function (x, y, kind, ref) {
            const dd = Math.hypot(x - hx, (y - 7) - hy);
            if (dd <= bestD) { bestD = dd; best = { kind: kind, x: x, y: y, ref: ref }; }
        };
        it.npcs.forEach(function (n) {
            const dd = Math.abs(n.x - hx);
            if (dd < 200) n.facing = n.x > hx ? -1 : 1;
            n.near = Math.hypot(n.x - hx, (n.y - 7) - hy) <= TALK_R;
            if (n.near && D() && D().startNode(sim, n)) consider(n.x, n.y, 'talk', n);
        });
        it.chests.forEach(function (c) { if (!c.opened) consider(c.x, c.y, c.locked ? 'locked' : 'open', c); });
        it.room.rests.forEach(function (r) { if (!ps.used[r.key]) consider(r.x, r.y, 'rest', r); });
        it.prompt = best ? { kind: best.kind, x: best.x, y: best.y } : null;
        if (input.talkPressed !== 'e') return;
        if (!best) {
            const near = it.chests.filter(function (c) { return c.opened && Math.hypot(c.x - hx, (c.y - 7) - hy) <= TALK_R; })[0];
            if (near) openChest(sim, it, near);
            return;
        }
        input.attackPressed = false;
        if (best.kind === 'talk') {
            if (D().open(sim, best.ref)) input.jumpPressed = false;
        } else if (best.kind === 'rest') {
            ps.used[best.ref.key] = true;
            heal(sim, 1, '+1', 'heart');
            sim.notice = 'runenpfad.notice.place_rest';
            pushEvent(sim, 'in_pickup', { kind: 'heart' });
        } else {
            openChest(sim, it, best.ref);
        }
    }

    function checkExits(sim, it) {
        const h = it.hero;
        const cx = Math.floor(h.x / CELL);
        const cy = Math.floor((h.y - 7) / CELL);
        const ex = it.room.exits.filter(function (e) { return e.cx === cx && e.cy === cy; })[0];
        if (!ex) { it.exitArmed = true; return; }
        if (!it.exitArmed) return;
        startLeave(it, ex.to === 'out' ? 'exit' : 'room', ex.to, ex.at);
    }

    function stepFloats(sim, it, dt) {
        sim.floatingTexts.forEach(function (f, i) {
            it.floats.push({ x: it.hero.x, y: it.hero.y - 30 - i * 12, text: f.text, color: f.color, timer: f.timer, key: f.key, suffix: f.suffix, icon: f.icon });
        });
        sim.floatingTexts = [];
        it.floats.forEach(function (f) { f.timer -= dt; f.y -= 28 * dt; });
        it.floats = it.floats.filter(function (f) { return f.timer > 0; });
    }

    // Returns null while the visit goes on, otherwise 'exit' or 'ko' (the iris has closed).
    function step(sim, it, input, dt) {
        it.t += dt;
        if (it.wipe.t < WIPE) it.wipe.t = Math.min(WIPE, it.wipe.t + dt);
        stepFloats(sim, it, dt);
        if (it.leaving) {
            const lv = it.leaving;
            lv.t += dt;
            if (lv.t >= WIPE) {
                if (lv.result === 'room') enterRoom(sim, it, lv.to, lv.at);
                else { it.result = lv.result; return lv.result; }
            }
            return null;
        }
        stepHero(sim, it, input, dt);
        it.mobs.forEach(function (m) { M().step(sim, it, m, dt); });
        it.mobs = it.mobs.filter(function (m) { return m.state !== 'dead' || m.deadT < 0.6; });
        M().stepShots(sim, it, dt);
        if (it.hadMobs && !it.cleared && !it.mobs.some(function (m) { return m.state !== 'dead'; })) {
            it.cleared = true;
            placeState(sim, it.placeId).cleared[it.roomId] = true;
            it.chests.forEach(function (c) { c.locked = false; });
            pushEvent(sim, 'in_clear', { roomId: it.roomId });
            sim.notice = 'runenpfad.notice.place_clear';
        }
        collectPickups(sim, it);
        interact(sim, it, input);
        if (!it.leaving) checkExits(sim, it);
        if (!it.leaving && sim.player.hp <= 0) startLeave(it, 'ko');
        return null;
    }

    root.RunenpfadInterior = {
        CELL: CELL,
        WIPE: WIPE,
        create: create,
        enterRoom: enterRoom,
        step: step,
        hurtHero: hurtHero,
        placeState: placeState
    };
})(typeof window !== 'undefined' ? window : globalThis);
