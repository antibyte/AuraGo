/* Runenpfad variety zones: bounce groves, updraft shafts, conveyor works, pendulum halls, secret vaults and lantern paths. DOM-free. */
(function (root) {
    'use strict';

    const ZONE_IDS = ['bounce_grove', 'updraft_shaft', 'conveyor_works', 'pendulum_hall', 'secret_vault', 'lantern_path', 'rope_chasm'];

    // Pendulum geometry: the saw bob hurts through a stage hazard rect that follows it every tick.
    const PEND_LEN = 480;
    const PEND_PIVOT_Y = 120;
    const PEND_AMP = 1.05;

    function P() {
        return root.RunenpfadPhysics;
    }

    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }
    function fxBurst(sim, kind, x, y, count, opts) { P().fxBurst(sim, kind, x, y, count, opts); }
    function fxShake(sim, amount) { P().fxShake(sim, amount); }

    function has(zone) {
        return ZONE_IDS.indexOf(zone) >= 0;
    }

    // Spike pit: a solid bottom row with spikes on top, as the classic chasm zones use.
    function spikePit(ctx, fromX, count) {
        for (let x = fromX; x < fromX + count; x++) {
            ctx.setTile(x, ctx.rows - 1, ctx.world.groundFill, 'solid');
            ctx.hazards.push({ kind: 'spikes', assetId: 'spikes', x: x * ctx.TILE, y: (ctx.rows - 2) * ctx.TILE, w: ctx.TILE, h: ctx.TILE });
        }
    }

    function addCollectible(ctx, kind, assetId, x, y) {
        ctx.collectibles.push({ id: kind.slice(0, 3) + '_' + ctx.collectibles.length, kind: kind, assetId: assetId, animated: false, x: x, y: y, collected: false });
    }

    function pendulumAt(pd, t) {
        const ang = PEND_AMP * Math.sin((t * Math.PI * 2) / pd.period + pd.phase);
        pd.angle = ang;
        pd.bx = pd.x + Math.sin(ang) * pd.len;
        pd.by = pd.y + Math.cos(ang) * pd.len;
        pd.hazard.x = pd.bx - 23;
        pd.hazard.y = pd.by - 29;
    }

    // Mushroom meadow: two bounce mushrooms launch the hero to high ledges.
    function buildBounceGrove(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        ctx.fillGround(cx, 15, g);
        let from = ctx.platforms.length;
        ctx.addPlatformRow(cx + 2, 2, g - 1, 'spring');
        for (let i = from; i < ctx.platforms.length; i++) ctx.platforms[i].bounce = 900;
        ctx.addPlatformRow(cx + 5, 4, g - 6, 'platform');
        ctx.addCoinLine(cx + 5, 4, g - 7, true);
        if (ctx.gemCount() < 2) ctx.placeGem(cx + 7, g - 7);
        from = ctx.platforms.length;
        ctx.addPlatformRow(cx + 11, 2, g - 1, 'spring');
        for (let i = from; i < ctx.platforms.length; i++) ctx.platforms[i].bounce = 1100;
        ctx.addPlatformRow(cx + 8, 3, g - 8, 'platform');
        addCollectible(ctx, 'star', 'star', (cx + 9.5) * ctx.TILE, (g - 8.6) * ctx.TILE);
        ctx.addCoinLine(cx + 8, 3, g - 9, false);
        ctx.addEnemy(ctx.nextEnemyId(), cx + 7, g, cx + 6, cx + 10);
        ctx.addProp(cx + 1, g);
        ctx.addProp(cx + 13, g);
        return 15;
    }

    // Wind column over a spiked pit: the lift carries the hero across.
    function buildUpdraftShaft(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        ctx.fillGround(cx, 4, g);
        spikePit(ctx, cx + 4, 6);
        ctx.windZones.push({
            id: 'wz_' + ctx.windZones.length,
            x: (cx + 5) * T,
            y: (g - 6) * T,
            w: 4 * T,
            h: (ctx.rows - (g - 6)) * T,
            lift: 2900,
            maxUp: 380
        });
        ctx.addPlatformRow(cx + 9, 3, g - 4, 'platform');
        ctx.addCoinLine(cx + 5, 4, g - 3, true);
        if (ctx.gemCount() < 3) ctx.placeGem(cx + 6, g - 6);
        ctx.fillGround(cx + 10, 5, g);
        ctx.addEnemy(ctx.nextEnemyId(), cx + 13, g, cx + 11, cx + 14);
        ctx.addProp(cx + 1, g);
        return 15;
    }

    // Conveyor platforms with opposing belts over spikes.
    function buildConveyorWorks(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        ctx.fillGround(cx, 3, g);
        spikePit(ctx, cx + 3, 11);
        const belts = [
            { x: cx + 3, row: g - 1, speed: -110 },
            { x: cx + 7, row: g - 2, speed: 140 },
            { x: cx + 11, row: g - 1, speed: -120 }
        ];
        belts.forEach(function (b) {
            const from = ctx.platforms.length;
            ctx.addPlatformRow(b.x, 3, b.row, 'conveyor');
            for (let i = from; i < ctx.platforms.length; i++) {
                ctx.platforms[i].conveyor = b.speed;
                ctx.platforms[i].assetId = ctx.world.platform;
            }
            ctx.addCoinLine(b.x, 3, b.row - 2, false);
        });
        if (ctx.gemCount() < 3) ctx.placeGem(cx + 8, g - 5);
        ctx.fillGround(cx + 14, 3, g);
        ctx.addEnemy(ctx.nextEnemyId(), cx + 15, g, cx + 14, cx + 16);
        ctx.addCheckpoint(cx + 14);
        void T;
        return 17;
    }

    // Swinging saw pendulums hang from the ceiling; timing the gaps is the whole puzzle.
    function buildPendulumHall(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        ctx.fillGround(cx, 19, g);
        ctx.addCheckpoint(cx + 1);
        for (let i = 0; i < 3; i++) {
            const hazard = { kind: 'pendulum', assetId: '', x: 0, y: 0, w: 46, h: 42 };
            ctx.hazards.push(hazard);
            const pd = {
                id: 'pd_' + ctx.pendulums.length,
                x: (cx + 4.5 + i * 5.5) * T,
                y: PEND_PIVOT_Y,
                len: PEND_LEN,
                amp: PEND_AMP,
                period: 3.6,
                phase: i * 2.0944,
                angle: 0,
                bx: 0,
                by: 0,
                hazard: hazard
            };
            pendulumAt(pd, 0);
            ctx.pendulums.push(pd);
        }
        ctx.addCoinLine(cx + 3, 14, g - 1, false);
        if (ctx.gemCount() < 2) ctx.placeGem(cx + 10, g - 2);
        ctx.addEnemy(ctx.nextEnemyId(), cx + 17, g, cx + 16, cx + 18);
        ctx.addProp(cx + 2, g);
        return 19;
    }

    // A cracked wall hides a chamber with treasure; three hits break it open.
    function buildSecretVault(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        ctx.fillGround(cx, 15, g);
        ctx.addPlatformRow(cx + 1, 2, g - 1, 'platform');
        ctx.addPlatformRow(cx + 3, 1, g - 2, 'platform');
        for (let x = cx + 4; x <= cx + 9; x++) ctx.setTile(x, g - 3, ctx.world.groundTop, 'solid');
        ctx.setTile(cx + 9, g - 2, ctx.world.groundFill, 'solid');
        ctx.setTile(cx + 9, g - 1, ctx.world.groundFill, 'solid');
        const wallTiles = [];
        for (let y = g - 2; y <= g - 1; y++) {
            ctx.setTile(cx + 4, y, 'stone_cracked', 'solid');
            wallTiles.push({ tx: cx + 4, ty: y, tile: ctx.tiles[ctx.tiles.length - 1] });
        }
        ctx.secretWalls.push({
            id: 'sw_' + ctx.secretWalls.length,
            x: (cx + 4) * T,
            y: (g - 2) * T,
            w: T,
            h: 2 * T,
            hp: 3,
            maxHp: 3,
            broken: false,
            hitFlash: 0,
            tiles: wallTiles,
            chamber: { x: (cx + 5) * T, y: (g - 2) * T, w: 4 * T, h: 2 * T }
        });
        ctx.addCoinLine(cx + 5, 4, g - 2, false);
        addCollectible(ctx, 'treasure', 'treasure', (cx + 6.5) * T, (g - 0.5) * T);
        addCollectible(ctx, 'potion', 'potion', (cx + 7.6) * T, (g - 0.5) * T);
        ctx.addCoinLine(cx + 5, 3, g - 4, true);
        ctx.addEnemy(ctx.nextEnemyId(), cx + 12, g, cx + 10, cx + 14);
        ctx.addProp(cx + 11, g);
        return 15;
    }

    // Lanterns light up on touch and reveal the spikes in the dark; lighting all three drops a heart.
    function buildLanternPath(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        ctx.fillGround(cx, 15, g);
        [2.5, 8, 12.5].forEach(function (off) {
            ctx.lanterns.push({ id: 'ln_' + ctx.lanterns.length, group: ctx.zIdx, x: (cx + off) * T, y: g * T - 44, lit: false, litAt: 0 });
        });
        [cx + 5, cx + 10].forEach(function (x) {
            ctx.hazards.push({ kind: 'spikes', assetId: ctx.cfg.worldIndex === 0 ? 'thorn' : 'spikes', x: x * T, y: (g - 1) * T, w: T, h: 32 });
        });
        ctx.addPlatformRow(cx + 3, 3, g - 3, 'platform');
        ctx.addCoinLine(cx + 3, 3, g - 4, true);
        ctx.addCoinLine(cx + 9, 3, g - 2, false);
        ctx.addEnemy(ctx.cfg.worldIndex >= 1 ? 'stalker' : ctx.nextEnemyId(), cx + 7, g, cx + 6, cx + 12);
        ctx.addCheckpoint(cx + 14);
        return 15;
    }

    // Hanging ropes cross a spiked chasm; the beam above them is drawn by stage art.
    function buildRopeChasm(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        ctx.fillGround(cx, 3, g);
        spikePit(ctx, cx + 3, 11);
        const beam = { x: (cx + 3) * T, y: (g - 8) * T, w: 11 * T };
        [5, 8, 11].forEach(function (off) {
            const x = (cx + off) * T;
            ctx.ropes.push({ id: 'rp_' + ctx.ropes.length, x: x, top: beam.y + 10, bottom: g * T - 20, beam: beam });
            ctx.addCoinLine(cx + off - 0.5, 1, g - 4, false);
            ctx.addCoinLine(cx + off - 0.5, 1, g - 6, false);
        });
        if (ctx.gemCount() < 3) ctx.placeGem(cx + 8, g - 7);
        ctx.fillGround(cx + 14, 4, g);
        ctx.addEnemy(ctx.nextEnemyId(), cx + 15, g, cx + 14, cx + 17);
        ctx.addCheckpoint(cx + 14);
        return 18;
    }

    const BUILDERS = {
        rope_chasm: buildRopeChasm,
        bounce_grove: buildBounceGrove,
        updraft_shaft: buildUpdraftShaft,
        conveyor_works: buildConveyorWorks,
        pendulum_hall: buildPendulumHall,
        secret_vault: buildSecretVault,
        lantern_path: buildLanternPath
    };

    function build(zone, ctx) {
        const fn = BUILDERS[zone];
        return fn ? fn(ctx) : 0;
    }

    // Pendulum bobs follow the clock; bounce mushrooms announce themselves one tick after the launch.
    function step(sim, dt) {
        const stage = sim.stage;
        (stage.pendulums || []).forEach(function (pd) { pendulumAt(pd, sim.elapsed); });
        (stage.secretWalls || []).forEach(function (w) {
            if (w.hitFlash > 0) w.hitFlash = Math.max(0, w.hitFlash - dt);
        });
        const p = sim.player;
        if (p.bouncePlat) {
            const plat = p.bouncePlat;
            p.bouncePlat = null;
            pushEvent(sim, 'bounce', { x: plat.x + plat.w / 2, y: plat.y, power: plat.bounce });
            fxBurst(sim, 'rune', plat.x + plat.w / 2, plat.y, 12, { color: '#fda4af', speed: 210, life: 0.6 });
            P().ring(sim, plat.x + plat.w / 2, plat.y, { color: '#fda4af', size: 14, grow: 220, life: 0.4 });
            fxShake(sim, 2.2);
        }
    }

    // Wind columns lift the hero; conveyor belts drag a standing hero along.
    function applyForces(sim, p, dt) {
        const zones = sim.stage.windZones || [];
        for (let i = 0; i < zones.length; i++) {
            const wz = zones[i];
            if (P().rectOverlap(p.x - p.w / 2, p.y - p.h, p.w, p.h, wz.x, wz.y, wz.w, wz.h)) {
                p.vy = Math.max(-wz.maxUp, p.vy - wz.lift * dt);
                p.slamActive = false;
            }
        }
        if (p.conveyorPush && p.onGround) p.x += p.conveyorPush * dt;
    }

    function lightLantern(sim, ln) {
        ln.lit = true;
        ln.litAt = sim.elapsed;
        sim.notice = 'runenpfad.notice.lantern_lit';
        sim.stageScore += 40;
        pushEvent(sim, 'lantern_lit', { x: ln.x, y: ln.y });
        fxBurst(sim, 'ember', ln.x, ln.y, 14, { color: '#fcd34d', speed: 120, life: 0.9 });
        P().ring(sim, ln.x, ln.y, { color: '#fcd34d', size: 12, grow: 160, life: 0.45 });
        const group = sim.stage.lanterns.filter(function (o) { return o.group === ln.group; });
        if (group.every(function (o) { return o.lit; })) {
            sim.stage.collectibles.push({ id: 'lh_' + sim.stage.collectibles.length, kind: 'heart', assetId: 'heart', animated: false, x: ln.x, y: ln.y - 70, collected: false });
            sim.floatingTexts.push({ x: ln.x, y: ln.y - 40, text: '+', color: '#86efac', timer: 0.9 });
        }
    }

    function interact(sim) {
        const p = sim.player;
        (sim.stage.lanterns || []).forEach(function (ln) {
            if (!ln.lit && Math.hypot(p.x - ln.x, (p.y - 22) - ln.y) < 46) lightLantern(sim, ln);
        });
    }

    // Attack hitboxes and rune bolts chip at cracked walls. Returns true when a wall was hit.
    function hitRect(sim, x, y, w, h, dmg) {
        let hit = false;
        (sim.stage.secretWalls || []).forEach(function (wall) {
            if (wall.broken || !P().rectOverlap(x, y, w, h, wall.x - 6, wall.y, wall.w + 12, wall.h)) return;
            hit = true;
            wall.hp -= dmg || 1;
            wall.hitFlash = 0.22;
            pushEvent(sim, 'enemy_hit', { sound: 'impact-stone', x: wall.x + wall.w / 2, y: wall.y + wall.h / 2 });
            fxBurst(sim, 'dust', wall.x + wall.w / 2, wall.y + wall.h / 2, 6, { color: '#cfc3ae', speed: 110, life: 0.45 });
            if (wall.hp <= 0) breakWall(sim, wall);
        });
        // Crystal switches (zones-extra) and rune chests (runes) react to the same hits.
        const extra = root.RunenpfadZonesExtra;
        const runes = root.RunenpfadRunes;
        if (extra && extra.hitRect(sim, x, y, w, h, dmg)) hit = true;
        if (runes && runes.hitRect(sim, x, y, w, h, dmg)) hit = true;
        // Round 4 puzzles: glyph stones, oracles, levers and braziers.
        const puzzles = root.RunenpfadPuzzles;
        if (puzzles && puzzles.hitRect(sim, x, y, w, h, dmg)) hit = true;
        return hit;
    }

    function breakWall(sim, wall) {
        wall.broken = true;
        wall.hp = 0;
        wall.tiles.forEach(function (t) {
            delete sim.stage.solidMap[t.tx + ',' + t.ty];
            t.tile.broken = true;
        });
        sim.notice = 'runenpfad.notice.secret_found';
        pushEvent(sim, 'secret_break', { x: wall.x + wall.w / 2, y: wall.y + wall.h / 2 });
        fxBurst(sim, 'shard', wall.x + wall.w / 2, wall.y + wall.h / 2, 16, { color: '#cfc3ae', speed: 230, life: 0.8 });
        fxBurst(sim, 'dust', wall.x + wall.w / 2, wall.y + wall.h, 12, { color: '#e8d8b8', speed: 160, life: 0.6 });
        fxShake(sim, 5);
        sim.stageScore += 100;
    }

    // Lantern light sources in world space (stage art feeds them into the light map).
    function lights(sim) {
        const out = [];
        (sim.stage.lanterns || []).forEach(function (ln) {
            out.push(ln.lit
                ? { x: ln.x, y: ln.y, radius: 210, intensity: 0.95, flicker: true }
                : { x: ln.x, y: ln.y, radius: 54, intensity: 0.22, flicker: false });
        });
        const extra = root.RunenpfadZonesExtra;
        const more = [root.RunenpfadPuzzles, root.RunenpfadDragons];
        let all = extra ? out.concat(extra.lights(sim)) : out;
        more.forEach(function (m) { if (m && m.lights) all = all.concat(m.lights(sim)); });
        return all;
    }

    root.RunenpfadZones = {
        ZONE_IDS: ZONE_IDS,
        has: has,
        build: build,
        step: step,
        applyForces: applyForces,
        interact: interact,
        hitRect: hitRect,
        lights: lights
    };
})(typeof window !== 'undefined' ? window : globalThis);
