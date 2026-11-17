/* Runenpfad rune power-ups: fire, frost, storm and magnet runes, rune orbs, rune chests and chest placement. DOM-free. */
(function (root) {
    'use strict';

    const KINDS = ['fire', 'frost', 'storm', 'magnet'];
    const DURATIONS = { fire: 15, frost: 20, storm: 15, magnet: 18 };
    const COLORS = { fire: '#fb923c', frost: '#7dd3fc', storm: '#c4b5fd', magnet: '#fde047' };
    const MAGNET_RADIUS = 340;
    const FROST_RADIUS = 130;
    const ZAP_RADIUS = 200;
    const DROP_CHANCE = 0.06;
    const PULLED = { coin: true, heart: true, apple: true, potion: true, star: true, rune_orb: true };

    function P() {
        return root.RunenpfadPhysics;
    }

    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }
    function fxBurst(sim, kind, x, y, count, opts) { P().fxBurst(sim, kind, x, y, count, opts); }

    function active(sim, kind) {
        const r = sim.player.rune;
        return r && r.timeLeft > 0 && (!kind || r.kind === kind) ? r : null;
    }

    // A new rune replaces the active one. Storm grants its extra air jump at once.
    function grant(sim, kind, x, y) {
        const p = sim.player;
        if (KINDS.indexOf(kind) < 0) return;
        const bonusBefore = extraAirJumps(sim);
        p.rune = { kind: kind, timeLeft: DURATIONS[kind], duration: DURATIONS[kind], charges: kind === 'frost' ? 1 : 0 };
        p.airJumpsLeft = Math.max(0, p.airJumpsLeft + extraAirJumps(sim) - bonusBefore);
        sim.stageScore += 100;
        pushEvent(sim, 'rune_pickup', { kind: kind, x: x, y: y });
        sim.floatingTexts.push({ x: x, y: y - 20, text: kind.toUpperCase() + ' RUNE', color: COLORS[kind], timer: 1.1 });
        fxBurst(sim, 'rune', x, y, 16, { color: COLORS[kind], speed: 200, life: 0.8 });
        P().fxFlash(sim, COLORS[kind], 0.2, 0.3);
    }

    function expire(sim) {
        const r = sim.player.rune;
        if (!r) return;
        const was = r.kind;
        const bonus = r.kind === 'storm' ? 1 : 0;
        sim.player.rune = null;
        sim.player.airJumpsLeft = Math.max(0, sim.player.airJumpsLeft - bonus);
        pushEvent(sim, 'rune_expire', { kind: was });
    }

    function extraAirJumps(sim) {
        return active(sim, 'storm') ? 1 : 0;
    }

    function speedMul(sim) {
        return active(sim, 'storm') ? 1.12 : 1;
    }

    function damageBonus(sim) {
        return active(sim, 'fire') ? 1 : 0;
    }

    function coinBonus(sim) {
        return active(sim, 'magnet') ? 1 : 0;
    }

    // Frost: enemies close to the hero crawl.
    function enemySlow(sim, e) {
        if (!active(sim, 'frost') || e.def.boss) return 1;
        return Math.hypot(e.x - sim.player.x, e.y - sim.player.y) <= FROST_RADIUS ? 0.6 : 1;
    }

    // Frost shield: swallows the next hit. Returns true when the hit was absorbed.
    function absorb(sim) {
        const r = active(sim, 'frost');
        if (!r || r.charges <= 0) return false;
        const p = sim.player;
        r.charges = 0;
        sim.player.rune = null;
        p.invulnTimer = 0.9;
        pushEvent(sim, 'rune_shield_break', { x: p.x, y: p.y - 22 });
        pushEvent(sim, 'rune_expire', { kind: 'frost' });
        fxBurst(sim, 'shard', p.x, p.y - 22, 14, { color: COLORS.frost, speed: 230, life: 0.6 });
        P().fxShake(sim, 3);
        return true;
    }

    // Storm: each kill zaps the nearest other living enemy within range for 1 damage (zaps never chain).
    function onKill(sim, enemy) {
        const r = active(sim, 'storm');
        if (!r || r.zapping || enemy.def.boss) return;
        let best = null;
        let bestD = ZAP_RADIUS;
        sim.stage.enemies.forEach(function (o) {
            if (o.dead || o === enemy || o.phased) return;
            const d = Math.hypot(o.x - enemy.x, o.y - enemy.y);
            if (d <= bestD) { best = o; bestD = d; }
        });
        if (!best) return;
        r.zapping = true;
        pushEvent(sim, 'rune_zap', { x1: enemy.x, y1: enemy.y - 24, x2: best.x, y2: best.y - 24 });
        root.RunenpfadCombat.damageEnemy(sim, best, 1, enemy.x, true);
        r.zapping = false;
    }

    function orb(sim, kind, x, y) {
        sim.stage.collectibles.push({ id: 'rune_' + sim.stepCount + '_' + Math.floor(x), type: 'rune_orb', kind: kind, assetId: '', animated: false, x: x, y: y, collected: false });
    }

    // Crates and barrels sometimes hold a rune orb.
    function maybeDrop(sim, x, y) {
        if (P().rand(sim) >= DROP_CHANCE) return false;
        orb(sim, KINDS[Math.floor(P().rand(sim) * KINDS.length)], x, y);
        return true;
    }

    function chestBox(c) {
        return { x: c.x - 28, y: c.y - 46, w: 56, h: 46 };
    }

    function openChest(sim, c) {
        if (c.opened) return;
        c.opened = true;
        pushEvent(sim, 'chest_open', { x: c.x, y: c.y, kind: c.kind });
        sim.effects.push({ animId: 'chest_open', x: c.x, y: c.y - 8, timer: 0.6 });
        orb(sim, c.kind, c.x, c.y - 74);
        sim.stageScore += 60;
        fxBurst(sim, 'rune', c.x, c.y - 30, 14, { color: COLORS[c.kind], speed: 170, life: 0.7 });
        P().fxShake(sim, 2.5);
    }

    // Attack hitboxes and rune bolts open chests. Returns true when a chest was hit.
    function hitRect(sim, x, y, w, h) {
        let hit = false;
        (sim.stage.chests || []).forEach(function (c) {
            const b = chestBox(c);
            if (!c.opened && P().rectOverlap(x, y, w, h, b.x, b.y, b.w, b.h)) {
                openChest(sim, c);
                hit = true;
            }
        });
        return hit;
    }

    // Collects a rune orb; returns true when the item was a rune orb (handled either way).
    function collect(sim, item, dist) {
        if (item.type !== 'rune_orb') return false;
        if (dist < 30) {
            item.collected = true;
            grant(sim, item.kind, item.x, item.y);
        }
        return true;
    }

    // Per-tick: touching a chest opens it.
    function interact(sim) {
        const p = sim.player;
        (sim.stage.chests || []).forEach(function (c) {
            const b = chestBox(c);
            if (!c.opened && P().rectOverlap(p.x - p.w / 2, p.y - p.h, p.w, p.h, b.x, b.y, b.w, b.h)) openChest(sim, c);
        });
    }

    // Per-tick: rune timers and the magnet pull.
    function tick(sim, dt) {
        const r = sim.player.rune;
        if (!r) return;
        r.timeLeft -= dt;
        if (r.timeLeft <= 0) {
            expire(sim);
            return;
        }
        if (r.kind !== 'magnet') return;
        const p = sim.player;
        sim.stage.collectibles.forEach(function (item) {
            if (item.collected || !(item.type === 'rune_orb' || PULLED[item.kind])) return;
            const dx = p.x - item.x;
            const dy = (p.y - 22) - item.y;
            const dist = Math.hypot(dx, dy);
            if (dist < MAGNET_RADIUS && dist > 1) {
                const step = Math.min(dist, 8);
                item.x += (dx / dist) * step;
                item.y += (dy / dist) * step;
            }
        });
    }

    // Guarantees a chest on the ground of a stage that has none (campaign level 3 on, every endless floor).
    function placeChest(stage, cfg) {
        if (stage.chests.length) return;
        const T = stage.tileSize;
        const g = Math.round(stage.groundY / T);
        const solid = function (tx, ty) { return !!stage.solidMap[tx + ',' + ty]; };
        const target = stage.widthTiles * (0.4 + 0.1 * ((cfg.id || 1) % 4));
        let best = -1;
        for (let tx = 6; tx < stage.widthTiles - 6; tx++) {
            if (!(solid(tx - 1, g) && solid(tx, g) && solid(tx + 1, g)) || solid(tx - 1, g - 1) || solid(tx, g - 1) || solid(tx + 1, g - 1)) continue;
            const x0 = (tx - 1) * T;
            const x1 = (tx + 2) * T;
            const clear = !stage.hazards.some(function (h) { return h.x < x1 && h.x + h.w > x0; })
                && !stage.platforms.some(function (pl) { return pl.ty >= g - 1 && pl.x < x1 && pl.x + pl.w > x0; })
                && !stage.crates.some(function (c) { return Math.abs(c.x - (tx + 0.5) * T) < 70; })
                && !stage.ladders.some(function (l) { return l.x < x1 && l.x + l.w > x0; })
                && !stage.checkpoints.some(function (c) { return Math.abs(c.x - (tx + 0.5) * T) < 70; })
                && !stage.boulders.some(function (b) { return b.x < x1 && b.x + b.w > x0; })
                && !stage.cannons.some(function (c) { return Math.abs(c.x - (tx + 0.5) * T) < 100; })
                && ![stage.key, stage.door, stage.portal].some(function (o) { return o && Math.abs(o.x - (tx + 0.5) * T) < 90; });
            if (clear && (best < 0 || Math.abs(tx - target) < Math.abs(best - target))) best = tx;
        }
        if (best < 0) return;
        stage.chests.push({ x: (best + 0.5) * T, y: stage.groundY, opened: false, kind: KINDS[((cfg.id || 1) * 3 + 1) % KINDS.length] });
    }

    root.RunenpfadRunes = {
        KINDS: KINDS,
        DURATIONS: DURATIONS,
        MAGNET_RADIUS: MAGNET_RADIUS,
        grant: grant,
        active: active,
        extraAirJumps: extraAirJumps,
        speedMul: speedMul,
        damageBonus: damageBonus,
        coinBonus: coinBonus,
        enemySlow: enemySlow,
        absorb: absorb,
        onKill: onKill,
        orb: orb,
        maybeDrop: maybeDrop,
        openChest: openChest,
        hitRect: hitRect,
        collect: collect,
        interact: interact,
        tick: tick,
        placeChest: placeChest
    };
})(typeof window !== 'undefined' ? window : globalThis);
