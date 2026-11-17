/* Runenpfad dragons: the dragon_strafe set piece (warned fire passes over an open stretch with stone shelters). DOM-free. */
(function (root) {
    'use strict';

    const ZONE_IDS = ['dragon_strafe'];

    const ZONE_TILES = 24;
    const LANE_TILES = 2.2;          // height of the fire lane above the ground, in tiles
    const FAR_MIN = 2;               // first far phase 2-4 s, later cooldown 5-7 s
    const FAR_SPAN = 2;
    const COOL_MIN = 5;
    const COOL_SPAN = 2;
    const WARN_TIME = 1.4;
    const STRAFE_SPEED = 560;        // px/s of the dragon body during the pass
    const ENTRY_PAD = 260;           // the dragon starts and ends this far outside the zone
    const HEAD_OFFSET = 90;          // head leads the body centre
    const FIRE_LENGTH = 200;         // flame stream ahead of the head
    const LEAVE_TIME = 1.2;
    const SHELTER_TILES = 3;         // a solid tile this close above the head shelters the hero

    function P() {
        return root.RunenpfadPhysics;
    }

    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }
    function fxBurst(sim, kind, x, y, count, opts) { P().fxBurst(sim, kind, x, y, count, opts); }
    function fxShake(sim, amount) { P().fxShake(sim, amount); }
    function rand(sim) { return P().rand(sim); }

    function has(zone) {
        return ZONE_IDS.indexOf(zone) >= 0;
    }

    // Open stretch with four floating stone roofs (clear height two tiles); every second roof has a step up.
    function buildDragonStrafe(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        const kind = ctx.cfg.worldIndex === 3 ? 'frost' : 'ember';
        ctx.fillGround(cx, ZONE_TILES, g);
        [1, 7, 13, 19].forEach(function (o, i) {
            for (let x = cx + o; x < cx + o + 3; x++) ctx.setTile(x, g - 3, ctx.world.groundTop, 'solid');
            ctx.addCoinLine(cx + o, 3, g - 1, false);
            if (i % 2 === 1) {
                ctx.addPlatformRow(cx + o - 2, 2, g - 2, 'platform');
                ctx.addCoinLine(cx + o, 3, g - 4, false);
                if (i === 1 && ctx.gemCount() < 3) ctx.placeGem(cx + o + 1, g - 4);
            }
        });
        ctx.extras.dragonRuns.push({
            id: 'dr_' + ctx.extras.dragonRuns.length,
            kind: kind,
            x1: cx * T,
            x2: (cx + ZONE_TILES) * T,
            state: 'off',
            t: 0,
            dx: 0,
            dy: 0,
            dir: 1,
            laneY: g * T - Math.round(LANE_TILES * T),
            laneH: Math.round(LANE_TILES * T),
            fire: null,
            passed: false,
            fxT: 0
        });
        ctx.addCheckpoint(cx + 0.5);
        ctx.addProp(cx + 23, g);
        return ZONE_TILES;
    }

    function build(zone, ctx) {
        return zone === 'dragon_strafe' ? buildDragonStrafe(ctx) : 0;
    }

    // True when a solid tile lies within three tiles above the hero's head.
    function sheltered(sim) {
        const p = sim.player;
        const stage = sim.stage;
        const T = stage.tileSize;
        const top = Math.floor((p.y - p.h) / T);
        for (let tx = Math.floor((p.x - 10) / T); tx <= Math.floor((p.x + 10) / T); tx++) {
            for (let k = 0; k <= SHELTER_TILES; k++) {
                const ty = top - k;
                if (ty >= 0 && P().isSolidTile(stage, tx, ty)) return true;
            }
        }
        return false;
    }

    function clampNear(sim, x) {
        return Math.max(sim.player.x - 420, Math.min(sim.player.x + 420, x));
    }

    function busy(runs, self) {
        return runs.some(function (o) { return o !== self && (o.state === 'warn' || o.state === 'strafe' || o.state === 'leave'); });
    }

    function startWarn(sim, r) {
        r.state = 'warn';
        r.t = WARN_TIME;
        r.dir = rand(sim) < 0.5 ? -1 : 1;
        r.dx = r.dir > 0 ? r.x1 - ENTRY_PAD : r.x2 + ENTRY_PAD;
        r.passed = false;
        pushEvent(sim, 'dragon_roar', { x: clampNear(sim, r.dx), y: r.laneY - 200, kind: r.kind });
        pushEvent(sim, 'dragon_warn', { x1: r.x1, x2: r.x2, laneY: r.laneY, laneH: r.laneH, kind: r.kind, dir: r.dir });
        if (!sim.stage.dragonHinted) {
            sim.stage.dragonHinted = true;
            sim.notice = 'runenpfad.notice.dragon_warn';
        }
        fxShake(sim, 3);
    }

    function startStrafe(sim, r) {
        r.state = 'strafe';
        r.t = 0;
        pushEvent(sim, 'dragon_breath', { x: clampNear(sim, r.dx), y: r.laneY, kind: r.kind });
        pushEvent(sim, 'dragon_pass', { x: clampNear(sim, r.dx), y: r.dy, kind: r.kind });
        fxShake(sim, 4);
    }

    function stepRun(sim, r, dt, runs) {
        const p = sim.player;
        const inside = p.x >= r.x1 && p.x <= r.x2;
        const mid = (r.x1 + r.x2) / 2;
        const span = r.x2 - r.x1;
        if (r.state === 'off') {
            if (inside) {
                r.state = 'far';
                r.t = FAR_MIN + rand(sim) * FAR_SPAN;
            }
            return;
        }
        if (r.state === 'far') {
            if (!inside) {
                r.state = 'off';
                return;
            }
            r.dx = mid + Math.sin(sim.elapsed * 0.45 + r.x1) * span * 0.55;
            r.dy = r.laneY - 420 + Math.sin(sim.elapsed * 0.9) * 40;
            r.dir = Math.cos(sim.elapsed * 0.45 + r.x1) >= 0 ? 1 : -1;
            r.t -= dt;
            if (r.t <= 0 && !busy(runs, r)) startWarn(sim, r);
            return;
        }
        if (r.state === 'warn') {
            r.t -= dt;
            const u = 1 - Math.max(0, r.t) / WARN_TIME;
            r.dy = r.laneY - 420 + u * 300;
            if (r.t <= 0) startStrafe(sim, r);
            return;
        }
        if (r.state === 'strafe') {
            r.t += dt;
            r.dx += r.dir * STRAFE_SPEED * dt;
            r.dy = r.laneY - 110 + Math.max(0, 1 - r.t / 0.4) * 190;
            const head = r.dx + r.dir * HEAD_OFFSET;
            r.fire = { dir: r.dir, x: r.dir > 0 ? head : head - FIRE_LENGTH, y: r.laneY, w: FIRE_LENGTH, h: r.laneH };
            r.fxT -= dt;
            if (r.fxT <= 0) {
                r.fxT = 0.09;
                fxBurst(sim, r.kind === 'frost' ? 'dust' : 'ember', r.fire.x + r.fire.w / 2, r.laneY + r.laneH - 10, 4,
                    { color: r.kind === 'frost' ? '#a5f3fc' : '#fb923c', speed: 120, arc: Math.PI, angle: -Math.PI / 2, life: 0.5 });
            }
            if (!r.passed && (r.dx - p.x) * r.dir >= 0) {
                r.passed = true;
                pushEvent(sim, 'dragon_pass', { x: p.x, y: r.dy, kind: r.kind });
            }
            if (p.invulnTimer <= 0 && sim.state === 'playing' && !sheltered(sim)
                && P().rectOverlap(p.x - p.w / 2 + 2, p.y - p.h + 4, p.w - 4, p.h - 6, r.fire.x, r.fire.y, r.fire.w, r.fire.h)) {
                pushEvent(sim, 'hazard', { kind: 'dragonfire', frost: r.kind === 'frost', x: p.x, y: p.y - 20 });
                root.RunenpfadCombat.hurtPlayer(sim, 1, r.fire.x + r.fire.w / 2, 'impact-metal');
            }
            const endX = r.dir > 0 ? r.x2 + ENTRY_PAD : r.x1 - ENTRY_PAD;
            if ((r.dx - endX) * r.dir >= 0) {
                r.state = 'leave';
                r.t = LEAVE_TIME;
                r.fire = null;
            }
            return;
        }
        if (r.state === 'leave') {
            r.t -= dt;
            r.dx += r.dir * STRAFE_SPEED * dt;
            r.dy -= 260 * dt;
            if (r.t <= 0) {
                r.state = 'far';
                r.t = COOL_MIN + rand(sim) * COOL_SPAN;
            }
        }
    }

    function step(sim, dt) {
        const runs = sim.stage.dragonRuns;
        if (!runs || !runs.length) return;
        runs.forEach(function (r) { stepRun(sim, r, dt, runs); });
    }

    function lights(sim) {
        const out = [];
        (sim.stage.dragonRuns || []).forEach(function (r) {
            if (r.state === 'strafe' && r.fire) {
                out.push({ x: r.fire.x + r.fire.w / 2, y: r.laneY + r.laneH / 2, radius: 230, intensity: 0.95, flicker: true });
            }
        });
        return out;
    }

    root.RunenpfadDragons = {
        ZONE_IDS: ZONE_IDS,
        has: has,
        build: build,
        step: step,
        lights: lights,
        sheltered: sheltered,
        WARN_TIME: WARN_TIME
    };
})(typeof window !== 'undefined' ? window : globalThis);
