/* Runenpfad extra zones: geyser fields, rune gates with crystal switches and flame jet corridors. DOM-free. */
(function (root) {
    'use strict';

    const ZONE_IDS = ['geyser_field', 'rune_gate', 'flame_jets'];
    const RUNE_KINDS = ['fire', 'frost', 'storm', 'magnet'];

    const GEYSER_PERIOD = 2.6;
    const GEYSER_WARN_FROM = 0.4;
    const GEYSER_ERUPT_FROM = 0.6;
    const GEYSER_ERUPT_TO = 0.85;
    const GEYSER_LAUNCH = -980;
    const EVENT_RANGE = 700;

    function P() {
        return root.RunenpfadPhysics;
    }

    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }
    function fxBurst(sim, kind, x, y, count, opts) { P().fxBurst(sim, kind, x, y, count, opts); }
    function fxShake(sim, amount) { P().fxShake(sim, amount); }

    function has(zone) {
        return ZONE_IDS.indexOf(zone) >= 0;
    }

    function near(sim, x) {
        return Math.abs(sim.player.x - x) < EVENT_RANGE;
    }

    // Two steam vents under high ledges: stepping into an eruption launches the hero up to the ledge treasure.
    function buildGeyserField(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        ctx.fillGround(cx, 18, g);
        [{ at: 4, offset: 0 }, { at: 12, offset: GEYSER_PERIOD / 2 }].forEach(function (v, i) {
            ctx.geysers.push({
                id: 'gy_' + ctx.geysers.length,
                x: (cx + v.at) * T,
                y: g * T,
                w: T,
                h: Math.round(6.5 * T),
                period: GEYSER_PERIOD,
                offset: v.offset,
                phase01: 0,
                active: false,
                erupting: false,
                launched: false
            });
            ctx.addPlatformRow(cx + v.at - 1, 4, g - 6, 'platform');
            ctx.addCoinLine(cx + v.at - 1, 4, g - 7, true);
            if (i === 0 && ctx.gemCount() < 3) ctx.placeGem(cx + v.at, g - 7);
            ctx.addProp(cx + v.at + 2, g);
        });
        ctx.collectibles.push({ id: 'gys_' + ctx.collectibles.length, kind: 'star', assetId: 'star', animated: false, x: (cx + 13.5) * T, y: (g - 7.2) * T, collected: false });
        ctx.addEnemy(ctx.nextEnemyId(), cx + 8, g, cx + 7, cx + 10);
        ctx.addEnemy(ctx.nextEnemyId(), cx + 16, g, cx + 15, cx + 17);
        ctx.addCheckpoint(cx + 1);
        return 18;
    }

    // Three crystal switches (ground, mid ledge, high ledge) open a sealed alcove that holds a rune chest.
    function buildRuneGate(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        ctx.fillGround(cx, 20, g);
        ctx.addPlatformRow(cx + 4, 2, g - 2, 'platform');
        ctx.addPlatformRow(cx + 7, 2, g - 4, 'platform');
        ctx.addPlatformRow(cx + 10, 2, g - 6, 'platform');
        ctx.addPlatformRow(cx + 11, 2, g - 2, 'platform');
        const switchIds = [];
        [{ x: cx + 2.5, y: g * T - 26 }, { x: cx + 7.5, y: (g - 4) * T - 26 }, { x: cx + 10.5, y: (g - 6) * T - 26 }].forEach(function (s) {
            ctx.switches.push({ id: 'sw_' + ctx.switches.length, x: s.x * T, y: s.y, on: false, hitFlash: 0 });
            switchIds.push('sw_' + (ctx.switches.length - 1));
        });
        // Sealed alcove: roof on top (also the walkable way over), gate column at the front, back wall behind.
        for (let x = cx + 13; x <= cx + 17; x++) ctx.setTile(x, g - 3, ctx.world.groundTop, 'solid');
        const gateTiles = [];
        for (let y = g - 2; y <= g - 1; y++) {
            ctx.setTile(cx + 13, y, ctx.world.groundFill, 'solid');
            gateTiles.push({ tx: cx + 13, ty: y, tile: ctx.tiles[ctx.tiles.length - 1] });
            ctx.setTile(cx + 17, y, ctx.world.groundFill, 'solid');
        }
        ctx.gates.push({ id: 'gt_' + ctx.gates.length, x: (cx + 13) * T, y: (g - 2) * T, w: T, h: 2 * T, open: false, tiles: gateTiles, switchIds: switchIds });
        ctx.chests.push({ x: (cx + 15.5) * T, y: g * T, opened: false, kind: RUNE_KINDS[(ctx.cfg.id || 1) % RUNE_KINDS.length] });
        ctx.addCoinLine(cx + 4, 2, g - 3, false);
        ctx.addCoinLine(cx + 7, 2, g - 5, false);
        if (ctx.gemCount() < 3) ctx.placeGem(cx + 10, g - 7);
        ctx.addEnemy(ctx.nextEnemyId(), cx + 9, g, cx + 8, cx + 11);
        ctx.addProp(cx + 1, g);
        return 20;
    }

    // Three flame jets with staggered timing: a short warning, then a burst you wait out.
    function buildFlameJets(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        const period = 3.4;
        ctx.fillGround(cx, 17, g);
        [4, 8, 12].forEach(function (at, i) {
            ctx.flameJets.push({
                id: 'fj_' + ctx.flameJets.length,
                x: (cx + at + 0.5) * T,
                y: g * T,
                w: 40,
                h: 132,
                period: period,
                warn: 0.8,
                fireDur: 1.0,
                offset: i * (period / 3),
                active: false,
                state: 'idle'
            });
        });
        ctx.addCoinLine(cx + 3, 3, g - 1, false);
        ctx.addCoinLine(cx + 10, 3, g - 1, false);
        if (ctx.gemCount() < 3) ctx.placeGem(cx + 8, g - 3);
        ctx.addEnemy(ctx.nextEnemyId(), cx + 15, g, cx + 14, cx + 16);
        ctx.addCheckpoint(cx + 1);
        ctx.addProp(cx + 6, g);
        return 17;
    }

    const BUILDERS = { geyser_field: buildGeyserField, rune_gate: buildRuneGate, flame_jets: buildFlameJets };

    function build(zone, ctx) {
        const fn = BUILDERS[zone];
        return fn ? fn(ctx) : 0;
    }

    function stepGeysers(sim) {
        (sim.stage.geysers || []).forEach(function (gy) {
            const ph = ((sim.elapsed + gy.offset) % gy.period) / gy.period;
            const was = gy.erupting;
            gy.phase01 = ph;
            gy.erupting = ph >= GEYSER_ERUPT_FROM && ph < GEYSER_ERUPT_TO;
            gy.active = gy.erupting || (ph >= GEYSER_WARN_FROM && ph < GEYSER_ERUPT_FROM);
            if (gy.erupting && !was) {
                gy.launched = false;
                if (near(sim, gy.x)) {
                    pushEvent(sim, 'geyser_burst', { x: gy.x + gy.w / 2, y: gy.y });
                    fxBurst(sim, 'dust', gy.x + gy.w / 2, gy.y, 10, { color: '#e0f2fe', speed: 200, arc: Math.PI, angle: -Math.PI / 2, life: 0.6 });
                    fxShake(sim, 2);
                }
            }
        });
    }

    function stepJets(sim) {
        const p = sim.player;
        (sim.stage.flameJets || []).forEach(function (j) {
            const t = (sim.elapsed + j.offset) % j.period;
            const fireFrom = j.period - j.fireDur;
            const prev = j.state;
            j.state = t >= fireFrom ? 'fire' : (t >= fireFrom - j.warn ? 'warn' : 'idle');
            j.active = j.state === 'fire';
            if (j.state !== prev && near(sim, j.x)) {
                if (j.state === 'warn') pushEvent(sim, 'jet_warn', { x: j.x, y: j.y });
                else if (j.state === 'fire') {
                    pushEvent(sim, 'jet_fire', { x: j.x, y: j.y });
                    fxBurst(sim, 'ember', j.x, j.y - 20, 12, { color: '#fb923c', speed: 150, arc: Math.PI, angle: -Math.PI / 2, life: 0.7 });
                }
            }
            if (j.state === 'fire' && p.invulnTimer <= 0 && sim.state === 'playing'
                && P().rectOverlap(p.x - p.w / 2 + 2, p.y - p.h + 4, p.w - 4, p.h - 6, j.x - j.w / 2, j.y - j.h, j.w, j.h)) {
                pushEvent(sim, 'hazard', { kind: 'flame_jet', x: j.x, y: j.y - j.h / 2 });
                root.RunenpfadCombat.hurtPlayer(sim, 1, j.x, 'impact-metal');
            }
        });
    }

    function step(sim, dt) {
        (sim.stage.switches || []).forEach(function (s) {
            if (s.hitFlash > 0) s.hitFlash = Math.max(0, s.hitFlash - dt);
        });
        stepGeysers(sim);
        stepJets(sim);
    }

    // Eruptions throw the hero skyward, once per eruption.
    function applyForces(sim, p) {
        (sim.stage.geysers || []).forEach(function (gy) {
            if (!gy.erupting || gy.launched) return;
            if (P().rectOverlap(p.x - p.w / 2, p.y - p.h, p.w, p.h + 4, gy.x, gy.y - gy.h, gy.w, gy.h + 4)) {
                gy.launched = true;
                p.vy = Math.min(p.vy, GEYSER_LAUNCH);
                p.onGround = false;
                p.onLadder = false;
                p.slamActive = false;
                fxBurst(sim, 'rune', p.x, p.y, 10, { color: '#bae6fd', speed: 180, life: 0.5 });
            }
        });
    }

    function openGate(sim, gate) {
        gate.open = true;
        gate.tiles.forEach(function (t) {
            delete sim.stage.solidMap[t.tx + ',' + t.ty];
            t.tile.broken = true;
        });
        pushEvent(sim, 'gate_open', { x: gate.x + gate.w / 2, y: gate.y + gate.h / 2 });
        sim.floatingTexts.push({ x: gate.x + gate.w / 2, y: gate.y - 10, text: 'OPEN', key: 'runenpfad.float_open', color: '#7ee8fa', timer: 1 });
        fxBurst(sim, 'shard', gate.x + gate.w / 2, gate.y + gate.h / 2, 14, { color: '#cfc3ae', speed: 200, life: 0.7 });
        fxShake(sim, 4);
        sim.stageScore += 120;
    }

    // Attack hitboxes and rune bolts flip crystal switches. Returns true when a switch was hit.
    function hitRect(sim, x, y, w, h) {
        let hit = false;
        const stage = sim.stage;
        (stage.switches || []).forEach(function (s) {
            if (!P().rectOverlap(x, y, w, h, s.x - 24, s.y - 28, 48, 56)) return;
            hit = true;
            if (s.on) return;
            s.on = true;
            s.hitFlash = 0.3;
            pushEvent(sim, 'switch_on', { x: s.x, y: s.y });
            fxBurst(sim, 'rune', s.x, s.y, 12, { color: '#7ee8fa', speed: 150, life: 0.6 });
            // Each gate opens once its own switches are all on.
            (stage.gates || []).forEach(function (gate) {
                if (gate.open || !gate.switchIds || gate.switchIds.indexOf(s.id) < 0) return;
                if (stage.switches.every(function (o) { return gate.switchIds.indexOf(o.id) < 0 || o.on; })) openGate(sim, gate);
            });
        });
        return hit;
    }

    function lights(sim) {
        const out = [];
        (sim.stage.flameJets || []).forEach(function (j) {
            out.push({ x: j.x, y: j.y - 50, radius: j.state === 'fire' ? 170 : 60, intensity: j.state === 'fire' ? 0.9 : (j.state === 'warn' ? 0.4 : 0.15), flicker: true });
        });
        (sim.stage.switches || []).forEach(function (s) {
            out.push({ x: s.x, y: s.y, radius: s.on ? 120 : 50, intensity: s.on ? 0.7 : 0.2, flicker: false });
        });
        return out;
    }

    root.RunenpfadZonesExtra = {
        ZONE_IDS: ZONE_IDS,
        has: has,
        build: build,
        step: step,
        applyForces: applyForces,
        hitRect: hitRect,
        lights: lights,
        openGate: openGate
    };
})(typeof window !== 'undefined' ? window : globalThis);
