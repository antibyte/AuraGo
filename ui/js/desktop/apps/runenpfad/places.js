/* Runenpfad places: door zones in the level, entering and leaving interiors, loot and the tick hooks. DOM-free. */
(function (root) {
    'use strict';

    const ZONE_IDS = ['place'];
    const ZONE_TILES = 9;
    const NEAR_DX = 40;
    const HINT = 'runenpfad.notice.place_hint';
    const STEP = 1 / 60;

    function P() { return root.RunenpfadPhysics; }
    function R() { return root.RunenpfadPlaceRooms; }
    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }

    function has(zone) { return ZONE_IDS.indexOf(zone) >= 0; }

    // Lays flat ground and one door in the middle of the zone. Unknown places build nothing.
    function build(zone, ctx) {
        const def = R() && R().PLACES[ctx.variant];
        if (!def) return 0;
        const T = ctx.TILE;
        const cx = ctx.cursorX;
        ctx.fillGround(cx, ZONE_TILES, ctx.groundY);
        if (!ctx.extras.doors) ctx.extras.doors = [];
        ctx.extras.doors.push({
            id: 'door_' + ctx.extras.doors.length, placeId: def.id, kind: def.kind, x: (cx + ZONE_TILES / 2) * T, y: ctx.groundY * T, w: 56,
            zoneX0: cx * T, zoneX1: (cx + ZONE_TILES) * T, near: false, cool: 0,
            facade: { clip: def.facade.clip, h: def.facade.h, dx: def.facade.dx }
        });
        return ZONE_TILES;
    }

    function doorsOf(stage) { return (stage && stage.doors) || []; }

    // Once per stage: ensure the arrays and move enemies out of the door zones (like camps do).
    function resolve(sim) {
        const stage = sim.stage;
        if (!stage || stage.placesReady) return;
        stage.placesReady = true;
        if (!stage.doors) stage.doors = [];
        if (!stage.placeState) stage.placeState = {};
        const T = stage.tileSize;
        stage.doors.forEach(function (door) {
            (stage.enemies || []).forEach(function (e) {
                if (e.def && e.def.boss) return;
                if (e.spawnX > door.zoneX0 && e.spawnX < door.zoneX1) {
                    const nx = door.zoneX1 + 2 * T;
                    e.x = nx;
                    e.spawnX = nx;
                    e.minX = Math.max(e.minX, door.zoneX1 + T);
                    e.maxX = Math.max(e.maxX, e.minX + 4 * T);
                } else if (e.spawnX >= door.zoneX1) {
                    e.minX = Math.max(e.minX, door.zoneX1);
                } else {
                    e.maxX = Math.min(e.maxX, door.zoneX0);
                }
            });
        });
    }

    function hint(sim, on) {
        if (on) {
            if (sim.notice === HINT) return;
            sim.placePrevNotice = sim.notice;
            sim.notice = HINT;
        } else if (sim.notice === HINT) {
            sim.notice = sim.placePrevNotice || '';
        }
    }

    // Called every world tick (after the NPC hold): door proximity, hint and the enter key.
    function tryEnter(sim, input) {
        const stage = sim.stage;
        if (!stage) return false;
        if (!stage.placesReady) resolve(sim);
        const doors = stage.doors;
        if (!doors.length) return false;
        if (root.RunenpfadVillage && root.RunenpfadVillage.stepBarks) root.RunenpfadVillage.stepBarks(sim, STEP);
        const p = sim.player;
        let target = null;
        doors.forEach(function (door) {
            if (door.cool > 0) door.cool = Math.max(0, door.cool - STEP);
            const near = Math.abs(p.x - door.x) <= NEAR_DX && !!p.onGround && Math.abs(p.y - door.y) <= 8;
            if (near && !door.near) pushEvent(sim, 'place_near', { placeId: door.placeId, kind: door.kind });
            door.near = near;
            if (near && !target) target = door;
        });
        hint(sim, !!target);
        if (!target || target.cool > 0) return false;
        const talk = input.talkPressed;
        const ok = talk === 'e' || (talk === 'up' && p.onGround && !p.swing && !p.onLadder && Math.abs(p.vx) < 60);
        if (!ok) return false;
        input.jumpPressed = false;
        input.attackPressed = false;
        p.vx = 0;
        if (target.kind === 'arch') {
            sim.uiRequest = 'worldmap';
            return true;
        }
        const it = root.RunenpfadInterior && root.RunenpfadInterior.create(sim, target.placeId, target.id);
        if (!it) return false;
        hint(sim, false);
        sim.interior = it;
        pushEvent(sim, 'place_enter', { placeId: target.placeId, kind: target.kind });
        return true;
    }

    function leave(sim, reason) {
        const it = sim.interior;
        if (!it) return;
        const door = doorsOf(sim.stage).filter(function (d) { return d.id === it.doorId; })[0];
        const p = sim.player;
        sim.interior = null;
        if (door) {
            p.x = door.x;
            door.cool = 0.6;
        }
        p.vx = 0;
        p.vy = 0;
        sim.notice = it.prevNotice || '';
        pushEvent(sim, 'place_leave', { placeId: it.placeId, reason: reason });
        if (reason === 'ko') {
            p.hp = 1;
            p.invulnTimer = 2;
            sim.notice = 'runenpfad.notice.place_ko';
            pushEvent(sim, 'place_ko');
        }
    }

    function step(sim, input, dt) {
        const it = sim.interior;
        const res = root.RunenpfadInterior.step(sim, it, input, dt);
        if (res === 'exit' || res === 'ko') leave(sim, res);
    }

    function placeOf(sim) { return sim.interior ? sim.interior.place : null; }
    function locked(sim) { return !!(sim && sim.interior); }

    function lootTaken(sim, lootId) {
        return !!(sim.profile && sim.profile.loot && sim.profile.loot.indexOf(lootId) >= 0);
    }

    // Pays a chest once. Campaign chests are remembered in profile.loot, repeat and endless chests only per stage run.
    function takeLoot(sim, chest) {
        const N = root.RunenpfadNpcs;
        const D = root.RunenpfadDialogue;
        const loot = chest.loot || {};
        const p = sim.player;
        if (loot.coins) {
            N.addCoins(sim, loot.coins);
            sim.stageCoins += loot.coins;
            sim.floatingTexts.push({ x: p.x, y: p.y - 44, text: '+' + loot.coins, icon: 'coin', color: '#fcd34d', timer: 1.2 });
        }
        if (loot.give && D) D.applyEffects(sim, null, [{ give: loot.give }]);
        if (loot.item) Object.keys(loot.item).forEach(function (id) { N.addItem(sim, id, loot.item[id]); });
        if (!chest.repeat && sim.profile) {
            const list = sim.profile.loot || (sim.profile.loot = []);
            if (list.indexOf(chest.lootId) < 0 && list.length < 48) {
                list.push(chest.lootId);
                list.sort();
            }
        }
        sim.storyDirty = true;
    }

    root.RunenpfadPlaces = {
        ZONE_IDS: ZONE_IDS,
        ZONE_TILES: ZONE_TILES,
        has: has,
        build: build,
        resolve: resolve,
        tryEnter: tryEnter,
        step: step,
        leave: leave,
        placeOf: placeOf,
        doorsOf: doorsOf,
        locked: locked,
        lootTaken: lootTaken,
        takeLoot: takeLoot
    };
})(typeof window !== 'undefined' ? window : globalThis);
