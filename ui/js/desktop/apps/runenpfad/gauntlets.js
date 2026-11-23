/* Runenpfad gauntlets: one clearly harder, timing-based section on the main route of every campaign level. DOM-free. */
(function (root) {
    'use strict';

    const ZONE_IDS = ['gauntlet'];
    // Width in tiles and the par-time bonus in seconds (about 0.25 s per tile plus the expected waiting).
    const WIDTHS = {
        thorn_hop: 12, crumble_steps: 13, spike_tunnel: 13, pendulum_bridge: 14, trap_isle: 13, guarded_isle: 14,
        crumble_chain: 15, ferry: 16, cannon_islands: 16, cannon_crossfire: 16, jet_islands: 15, conveyor_gauntlet: 15
    };
    const PAR = {
        thorn_hop: 4, crumble_steps: 4, spike_tunnel: 8, pendulum_bridge: 8, trap_isle: 6, guarded_isle: 8,
        crumble_chain: 6, ferry: 10, cannon_islands: 8, cannon_crossfire: 10, jet_islands: 8, conveyor_gauntlet: 9
    };
    const VARIANTS = Object.keys(WIDTHS);
    // Pit hazards by world index: thorns, spikes, lava, spikes, lava.
    const PITS = [{ kind: 'spikes', asset: 'thorn' }, { kind: 'spikes', asset: 'spikes' }, { kind: 'lava', asset: 'lava' }, { kind: 'spikes', asset: 'spikes' }, { kind: 'lava', asset: 'lava' }];
    const TUNNEL_PERIOD = [1.8, 1.7, 1.6, 1.6, 1.5];
    // Must equal zones.js PEND_AMP so the swept arc matches the pendulum hall tests.
    const PEND_AMP = 1.05;

    function has(zone) {
        return ZONE_IDS.indexOf(zone) >= 0;
    }

    // Layout helpers: offsets `o` are tiles from the zone start. Nothing here draws from ctx.rng, so the
    // enemy and prop stream of the other zones stays identical with or without a gauntlet.
    function kit(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        const R = ctx.rows;
        const wi = ctx.cfg.worldIndex || 0;
        const pit = PITS[wi] || PITS[0];
        let seq = 0;
        const k = { cx: cx, g: g, T: T, wi: wi };

        k.ground = function (o, n, top) { ctx.fillGround(cx + o, n, top == null ? g : top); };
        k.pit = function (o, n) {
            for (let x = cx + o; x < cx + o + n; x++) {
                ctx.setTile(x, R - 1, ctx.world.groundFill, 'solid');
                ctx.hazards.push({ kind: pit.kind, assetId: pit.asset, x: x * T, y: (R - 2) * T, w: T, h: T, gauntlet: true });
            }
        };
        k.plat = function (o, n, row, kind, extra) {
            const from = ctx.platforms.length;
            ctx.addPlatformRow(cx + o, n, row, kind);
            for (let i = from; i < ctx.platforms.length; i++) {
                const p = ctx.platforms[i];
                Object.assign(p, extra || {}, { gauntlet: true });
                if (kind === 'conveyor') p.assetId = ctx.world.platform;
            }
        };
        // A low ceiling slab: the top row shows grass, the rows below are fill.
        k.roof = function (o, n, rowTop, rowBottom) {
            for (let x = cx + o; x < cx + o + n; x++) {
                for (let y = rowTop; y <= rowBottom; y++) ctx.setTile(x, y, y === rowTop ? ctx.world.groundTop : ctx.world.groundFill, 'solid');
            }
        };
        k.trap = function (o, surfaceRow, period, offset) {
            ctx.hazards.push({ kind: 'spike_trap', assetId: 'spike_trap', x: (cx + o) * T, y: (surfaceRow - 1) * T, w: T, h: 30, period: period, offset: offset, gauntlet: true });
        };
        k.pendulum = function (oc, pivotY, len, period, phase) {
            const hazard = { kind: 'pendulum', assetId: '', x: 0, y: 0, w: 46, h: 42, gauntlet: true };
            ctx.hazards.push(hazard);
            const pd = { id: 'pd_' + ctx.pendulums.length, x: (cx + oc) * T, y: pivotY, len: len, amp: PEND_AMP, period: period, phase: phase, angle: 0, bx: 0, by: 0, hazard: hazard, gauntlet: true };
            const a = PEND_AMP * Math.sin(phase);
            pd.angle = a;
            pd.bx = pd.x + Math.sin(a) * len;
            pd.by = pd.y + Math.cos(a) * len;
            hazard.x = pd.bx - 23;
            hazard.y = pd.by - 29;
            ctx.pendulums.push(pd);
        };
        k.mover = function (startPx, endPx, row, speed) {
            const sx = cx * T + startPx;
            ctx.movingPlatforms.push({ id: 'mp_' + ctx.movingPlatforms.length, startX: sx, endX: cx * T + endPx, x: sx, prevX: sx, y: row * T, prevY: row * T, w: T * 2, h: 18, speed: speed, dir: 1, gauntlet: true });
        };
        k.cannon = function (oc, y, timer, interval) {
            ctx.cannons.push({ id: 'can_' + ctx.cannons.length, x: (cx + oc) * T, y: y, dir: -1, timer: timer, interval: interval, gauntlet: true });
        };
        k.jet = function (o, surfaceRow, period, warn, fireDur, offset) {
            ctx.flameJets.push({ id: 'fj_' + ctx.flameJets.length, x: (cx + o + 0.5) * T, y: surfaceRow * T, w: 40, h: 132, period: period, warn: warn, fireDur: fireDur, offset: offset, active: false, state: 'idle', gauntlet: true });
        };
        // Enemies are built directly: ctx.addEnemy would draw the shoot timer from ctx.rng.
        k.guard = function (defId, oc, omin, omax, dir) {
            const endless = ctx.cfg.mode === 'endless';
            const e = root.RunenpfadLevels.createEnemy(defId, (cx + oc) * T, g * T, (cx + omin) * T, (cx + omax) * T, {
                id: 'e_' + ctx.enemies.length,
                hpScale: endless ? 1 + Math.floor((ctx.cfg.endlessFloor - 1) / 4) : 1,
                dir: dir || 1,
                shootTimer: 1.6
            });
            e.gauntlet = true;
            ctx.enemies.push(e);
        };
        k.barrel = function (oc, drop) {
            ctx.crates.push({ id: 'c_' + ctx.crates.length, assetId: 'barrel', x: (cx + oc) * T, y: (g - 1) * T, w: 38, h: 38, hp: 1, broken: false, drop: drop, gauntlet: true });
        };
        k.heart = function (oc, rowC) {
            seq++;
            ctx.collectibles.push({ id: 'gh_' + seq + '_' + ctx.collectibles.length, kind: 'heart', assetId: 'heart', animated: false, x: (cx + oc) * T, y: rowC * T, collected: false });
        };
        k.coins = function (o, n, row, arc) { ctx.addCoinLine(cx + o, n, row, arc); };
        return k;
    }

    // Each builder lays out one variant; the comments name the hard moment.
    const BUILDERS = {
        // Two thorn pits; the second jump has to start from the last stretch of a two-tile island.
        thorn_hop: function (k) {
            k.ground(0, 2); k.pit(2, 2); k.ground(4, 2); k.pit(6, 3); k.ground(9, 3);
            k.coins(2, 2, k.g - 3, true); k.coins(6, 3, k.g - 3, true);
        },
        // Rising jumps between crumbling ledges.
        crumble_steps: function (k) {
            k.ground(0, 3); k.pit(3, 7);
            k.plat(4, 2, k.g - 1, 'trapdoor'); k.plat(7, 2, k.g - 2, 'trapdoor');
            k.ground(10, 3);
            k.coins(4, 2, k.g - 2, false); k.coins(7, 2, k.g - 3, false);
        },
        // A low ceiling forbids jumping: stop and go between three spike traps.
        spike_tunnel: function (k) {
            const P = TUNNEL_PERIOD[k.wi] || TUNNEL_PERIOD[0];
            k.ground(0, 13); k.roof(2, 9, k.g - 5, k.g - 2);
            k.trap(3, k.g, P, 0); k.trap(6, k.g, P, P / 2); k.trap(9, k.g, P, 0);
            k.coins(4, 2, k.g - 1, false); k.coins(7, 2, k.g - 1, false);
        },
        // A saw swings over a crumbling bridge: once started, the hero must keep running.
        pendulum_bridge: function (k) {
            k.ground(0, 4); k.pit(4, 6); k.plat(4, 6, k.g, 'trapdoor'); k.ground(10, 4);
            k.pendulum(7.5, (k.g - 4) * k.T, 168, 3.6, 0);
            k.coins(4, 6, k.g - 1, false);
        },
        // Land on a short island, wait for the spike trap, then jump on.
        trap_isle: function (k) {
            k.ground(0, 3); k.pit(3, 2); k.ground(5, 4); k.pit(9, 2); k.ground(11, 2);
            k.trap(7, k.g, 1.8, 0);
            k.coins(5, 2, k.g - 1, false);
        },
        // Land next to two guards patrolling the island in opposite phase and beat them there.
        guarded_isle: function (k) {
            k.ground(0, 3); k.pit(3, 2); k.ground(5, 4); k.pit(9, 2); k.ground(11, 3);
            k.guard(k.wi <= 1 ? 'goblin' : 'service_robot', 7.5, 5.5, 8.5, 1);
            k.guard(k.wi <= 1 ? 'goblin' : 'service_robot', 7, 6, 8.5, -1);
            k.barrel(6.5, 'heart');
            k.coins(3, 2, k.g - 3, true); k.coins(9, 2, k.g - 3, true);
        },
        // Four single-tile slabs that crumble quickly, alternating up and down.
        crumble_chain: function (k) {
            k.ground(0, 3); k.pit(3, 9); k.ground(12, 3);
            k.plat(4, 1, k.g - 1, 'trapdoor', { crumbleTime: 0.4 }); k.plat(6, 1, k.g - 2, 'trapdoor', { crumbleTime: 0.4 });
            k.plat(8, 1, k.g - 1, 'trapdoor', { crumbleTime: 0.4 }); k.plat(10, 1, k.g - 2, 'trapdoor', { crumbleTime: 0.4 });
            k.coins(4, 1, k.g - 2, false); k.coins(6, 1, k.g - 3, false); k.coins(8, 1, k.g - 2, false); k.coins(10, 1, k.g - 3, false);
            k.heart(13.5, k.g - 1.5);
        },
        // Board a moving platform, then jump onto a ledge whose near half is a spike trap.
        ferry: function (k) {
            k.ground(0, 3); k.pit(3, 10); k.ground(13, 3);
            k.mover(168, 312, k.g - 1, 80);
            k.plat(10, 2, k.g - 1, 'platform');
            k.trap(10, k.g - 1, 1.6, 0);
            k.coins(4, 4, k.g - 2, false); k.heart(11.5, k.g - 1.5);
        },
        // Island hops that double as dodges over cannonballs.
        cannon_islands: function (k) {
            k.ground(0, 3); k.pit(3, 2); k.ground(5, 2); k.pit(7, 2); k.ground(9, 2); k.pit(11, 2); k.ground(13, 3);
            k.cannon(15.5, (k.g - 0.5) * k.T, 1.0, 1.7);
            k.coins(3, 2, k.g - 3, true); k.coins(7, 2, k.g - 3, true); k.coins(11, 2, k.g - 3, true);
        },
        // The same islands plus a second, head-high cannon on a different rhythm: low balls want a jump, high ones do not.
        cannon_crossfire: function (k) {
            BUILDERS.cannon_islands(k);
            k.plat(14, 1, k.g - 1, 'platform');
            k.cannon(14.5, (k.g - 1.2) * k.T, 0.35, 2.3);
        },
        // Flame-jet gates on narrow islands between pits.
        jet_islands: function (k) {
            k.ground(0, 3); k.pit(3, 2); k.ground(5, 3); k.pit(8, 2); k.ground(10, 5);
            k.jet(7, k.g, 2.2, 0.5, 1.2, 0); k.jet(13, k.g, 2.2, 0.5, 1.2, 1.1);
            k.coins(5, 2, k.g - 1, false); k.coins(10, 3, k.g - 1, false);
        },
        // A committed run against two belts through two fire gates.
        conveyor_gauntlet: function (k) {
            k.ground(0, 3); k.pit(3, 10); k.ground(13, 2);
            k.plat(3, 5, k.g, 'conveyor', { conveyor: -140 }); k.plat(10, 3, k.g - 1, 'conveyor', { conveyor: -140 });
            k.jet(6, k.g, 2.6, 0.6, 1.1, 0); k.jet(12, k.g - 1, 2.6, 0.6, 1.1, 0.53);
            k.coins(4, 2, k.g - 1, false); k.coins(10, 2, k.g - 2, false);
        }
    };

    function build(zone, ctx) {
        const builder = BUILDERS[ctx.variant];
        if (zone !== 'gauntlet' || !builder) return 0;
        builder(kit(ctx), ctx);
        const width = WIDTHS[ctx.variant];
        if (!ctx.extras.gauntlets) ctx.extras.gauntlets = [];
        ctx.extras.gauntlets.push({ variant: ctx.variant, x0: ctx.cursorX * ctx.TILE, x1: (ctx.cursorX + width) * ctx.TILE, width: width });
        return width;
    }

    // Walkable spans (tiles, [from, to)) a checkpoint must not sit in: camps, place zones, thorn rollers, flame jets, gates.
    function blockedSpans(ctx) {
        const T = ctx.TILE;
        const spans = [];
        (ctx.extras.npcs || []).forEach(function (n) { spans.push([n.campX0 / T, n.campX1 / T]); });
        (ctx.extras.doors || []).forEach(function (d) { spans.push([d.zoneX0 / T, d.zoneX1 / T]); });
        (ctx.thornRollers || []).forEach(function (r) { spans.push([(r.startX - r.r) / T, (r.endX + r.r) / T]); });
        (ctx.flameJets || []).forEach(function (j) { spans.push([j.x / T - 1.5, j.x / T + 1.5]); });
        (ctx.gates || []).forEach(function (g) { spans.push([g.x / T - 1, (g.x + g.w) / T + 1]); });
        return spans;
    }

    // Row of the topmost solid tile of column x, or -1.
    function topSolid(ctx, x) {
        for (let y = 0; y < ctx.rows; y++) if (ctx.solidMap[x + ',' + y]) return y;
        return -1;
    }

    // Called by levels.js right before a gauntlet is built: makes sure a checkpoint lies within 8 tiles before it, so the
    // revive is re-armed. Reuses an existing checkpoint, else a flat tile within 6 tiles, else lays a 2-tile pad.
    // Returns the number of tiles the cursor advances (0 or 2). Makes no rng draw. Campaign levels only.
    function checkpointBefore(ctx, info) {
        const mode = (ctx.cfg && ctx.cfg.mode) || 'campaign';
        if (mode !== 'campaign' || (ctx.cfg && (ctx.cfg.hub || ctx.cfg.id === 0)) || !ctx.checkpoints) return 0;
        const T = ctx.TILE;
        const cx = ctx.cursorX;
        const near = ctx.checkpoints.some(function (cp) { return cp.x >= (cx - 8) * T && cp.x <= cx * T; });
        if (near) return 0;
        const blocked = blockedSpans(ctx);
        for (let x = cx - 1; x >= cx - 6 && x >= 1; x--) {
            const g = topSolid(ctx, x);
            if (g < 2 || topSolid(ctx, x - 1) !== g || (x + 1 < cx && topSolid(ctx, x + 1) !== g)) continue;
            if (blocked.some(function (s) { return x + 1 > s[0] && x < s[1]; })) continue;
            if (ctx.platforms.some(function (p) { return p.tx === x && p.ty >= g - 3 && p.ty < g; })) continue;
            if (ctx.hazards.some(function (h) { return h.x < (x + 1) * T && h.x + h.w > x * T && h.y < g * T + T && h.y + h.h > (g - 2) * T; })) continue;
            ctx.checkpoints.push({ id: 'cp_' + ctx.checkpoints.length, x: (x + 0.5) * T, y: (g - 1) * T, activated: false });
            return 0;
        }
        ctx.fillGround(cx, 2, ctx.groundY);
        ctx.checkpoints.push({ id: 'cp_' + ctx.checkpoints.length, x: (cx + 0.5) * T, y: (ctx.groundY - 1) * T, activated: false });
        return 2;
    }

    function parBonus(variant) {
        return PAR[variant] || 0;
    }

    // Par bonus of a level config: every 'gauntlet:<variant>' slot adds its variant's bonus.
    function parBonusFor(zones) {
        return (zones || []).reduce(function (sum, z) {
            return typeof z === 'string' && z.indexOf('gauntlet:') === 0 ? sum + parBonus(z.slice(9)) : sum;
        }, 0);
    }

    root.RunenpfadGauntlets = { ZONE_IDS: ZONE_IDS, VARIANTS: VARIANTS, WIDTHS: WIDTHS, PAR: PAR, has: has, build: build, checkpointBefore: checkpointBefore, parBonus: parBonus, parBonusFor: parBonusFor };
})(typeof window !== 'undefined' ? window : globalThis);
