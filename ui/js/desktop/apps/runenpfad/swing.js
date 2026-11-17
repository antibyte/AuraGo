/* Runenpfad swinging ropes: pendulum physics, grab / pump / release and the swing_gorge zone. DOM-free. */
(function (root) {
    'use strict';

    const ZONE_IDS = ['swing_gorge'];

    const GRAVITY = 1900;       // pendulum gravity in px/s^2 (a bit snappier than the world gravity)
    const MAX_ANGLE = 1.31;     // ~75 degrees
    const PUMP = 7.5;           // rad/s^2 added while left/right is held on the forward swing
    const HANDS = 40;           // hands sit this far above the hero's feet
    const GRAB_RADIUS = 32;     // forgiving grab radius to the rope line
    const MAGNET_RADIUS = 48;   // an airborne hero moving toward a rope this close gets caught
    const GRAB_FROM = 0.12;     // everything below the top eighth of the rope can be grabbed
    const RELEASE_COOL = 0.25;  // seconds after a release before re-grabbing THAT rope
    const JUMP_BOOST = 360;     // upward boost when releasing with jump (feels like a jump)
    const FWD_BOOST = 140;      // forward boost in the swing direction
    const CLIMB_SPEED = 140;    // px/s along the rope with Up / Down
    const CLIMB_TOP = 0.18;     // highest grip as a fraction of the rope length
    const LET_GO = 0.25;        // seconds of holding Down at the rope end before letting go
    const HINT_RANGE = 420;

    function P() {
        return root.RunenpfadPhysics;
    }

    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }
    function fxBurst(sim, kind, x, y, count, opts) { P().fxBurst(sim, kind, x, y, count, opts); }

    function has(zone) {
        return ZONE_IDS.indexOf(zone) >= 0;
    }

    function styleFor(worldIndex) {
        return worldIndex === 0 ? 'vine' : (worldIndex === 4 ? 'chain' : 'rope');
    }

    function spikePit(ctx, fromX, count) {
        for (let x = fromX; x < fromX + count; x++) {
            ctx.setTile(x, ctx.rows - 1, ctx.world.groundFill, 'solid');
            ctx.hazards.push({ kind: 'spikes', assetId: 'spikes', x: x * ctx.TILE, y: (ctx.rows - 2) * ctx.TILE, w: ctx.TILE, h: ctx.TILE });
        }
    }

    // A wide spiked gorge crossed on 3-4 swinging ropes hung from a beam (vines, iron rings or chains by world).
    function buildSwingGorge(ctx) {
        const cx = ctx.cursorX;
        const g = ctx.groundY;
        const T = ctx.TILE;
        const n = ctx.cfg.worldIndex >= 2 ? 4 : 3;
        const spacing = 3.6;
        const first = 6; // the arc reaches ~5.4 tiles to each side, so every rope's corridor stays inside the zone
        const pitWidth = n === 4 ? 15 : 11; // the far ledge starts ~3 tiles past the last rope
        const style = styleFor(ctx.cfg.worldIndex);
        const group = 'sg_' + ctx.extras.swings.length;
        ctx.fillGround(cx, 5, g);
        spikePit(ctx, cx + 5, pitWidth);
        ctx.fillGround(cx + 5 + pitWidth, 4, g);
        const ay = (g - 8) * T;
        const len = 270;
        for (let i = 0; i < n; i++) {
            const rx = cx + first + i * spacing;
            ctx.extras.swings.push({
                id: 'sw_' + ctx.extras.swings.length,
                group: group,
                style: style,
                ax: rx * T,
                ay: ay,
                len: len,
                angle: 0.09 * (i % 2 === 0 ? 1 : -1),
                omega: 0,
                phase: i
            });
            if (i < n - 1) ctx.addCoinLine(rx - 0.5 + spacing / 2, 1, g - 3.2 + (i % 2) * 0.3, false);
        }
        // Bonus gem hanging in the arc of the middle rope: reachable only by swinging out.
        if (ctx.gemCount() < 3) {
            const mid = ctx.extras.swings[ctx.extras.swings.length - Math.ceil(n / 2)];
            ctx.placeGem((mid.ax + Math.sin(0.62) * len) / T - 0.5, (mid.ay + Math.cos(0.62) * len + HANDS - 22) / T - 0.5);
        }
        ctx.addCheckpoint(cx + 1);
        ctx.addProp(cx + 1, g);
        ctx.addProp(cx + 5 + pitWidth + 2, g);
        return 5 + pitWidth + 4;
    }

    function build(zone, ctx) {
        return zone === 'swing_gorge' ? buildSwingGorge(ctx) : 0;
    }

    function find(list, id) {
        for (let i = 0; i < list.length; i++) if (list[i].id === id) return list[i];
        return null;
    }

    // One pendulum tick. Pumping only helps on the swing toward the held direction, so holding one key builds amplitude.
    function integrate(s, dir, occupied, dt) {
        let acc = -(GRAVITY / s.len) * Math.sin(s.angle);
        if (occupied && dir !== 0 && s.omega * dir >= -0.1) acc += dir * PUMP * (0.5 + 0.5 * Math.cos(s.angle));
        s.omega += acc * dt;
        s.omega *= 1 - (occupied ? 0.12 : 0.3) * dt;
        if (!occupied && Math.abs(s.angle) < 0.15 && Math.abs(s.omega) < 0.45) {
            s.omega += (s.omega !== 0 ? Math.sign(s.omega) : (s.angle >= 0 ? 1 : -1)) * 0.1 * dt;
        }
        s.angle += s.omega * dt;
        if (s.angle > MAX_ANGLE || s.angle < -MAX_ANGLE) {
            s.angle = Math.sign(s.angle) * MAX_ANGLE;
            if (s.omega * s.angle > 0) s.omega = 0;
        }
    }

    function endOf(s) {
        return { x: s.ax + s.len * Math.sin(s.angle), y: s.ay + s.len * Math.cos(s.angle) };
    }

    function distToRope(s, hx, hy) {
        const dx = Math.sin(s.angle);
        const dy = Math.cos(s.angle);
        const along = (hx - s.ax) * dx + (hy - s.ay) * dy;
        const t = Math.min(s.len, Math.max(s.len * GRAB_FROM, along));
        return { dist: Math.hypot(hx - (s.ax + dx * t), hy - (s.ay + dy * t)), along: along };
    }

    // True when the hero rect at (x, y) overlaps a solid tile, or a solid one-way platform while moving down.
    function blocked(sim, x, y, prevY) {
        const stage = sim.stage;
        const T = stage.tileSize;
        for (let tx = Math.floor((x - 14) / T); tx <= Math.floor((x + 14) / T); tx++) {
            for (let ty = Math.floor((y - 44) / T); ty <= Math.floor((y - 1) / T); ty++) {
                if (P().isSolidTile(stage, tx, ty)) return true;
            }
        }
        if (y > prevY) {
            for (let i = 0; i < stage.platforms.length; i++) {
                const pl = stage.platforms[i];
                if (pl.solid && x + 14 > pl.x && x - 14 < pl.x + pl.w && prevY <= pl.y + 2 && y > pl.y) return true;
            }
        }
        return false;
    }

    function seat(sim, s) {
        const p = sim.player;
        const sw = p.swing;
        const gp = sw.grip;
        p.x = s.ax + gp * Math.sin(s.angle);
        p.y = s.ay + gp * Math.cos(s.angle) + HANDS;
        p.vx = s.omega * gp * Math.cos(s.angle);
        p.vy = -s.omega * gp * Math.sin(s.angle);
        sw.angle = s.angle;
        sw.omega = s.omega;
        p.onGround = false;
        p.onLadder = false;
        p.climbKind = null;
        p.slamActive = false;
        p.dashTimer = 0;
        p.coyoteTimer = 0;
        p.jumpBuffer = 0;
    }

    function grab(sim, s, along) {
        const p = sim.player;
        p.swingCoolId = null;
        p.swing = { id: s.id, angle: s.angle, omega: s.omega, grip: Math.min(s.len, Math.max(s.len * GRAB_FROM, along)), t: 0, endT: 0 };
        const runes = root.RunenpfadRunes;
        p.airJumpsLeft = sim.stats.maxAirJumps + (runes ? runes.extraAirJumps(sim) : 0);
        seat(sim, s);
        pushEvent(sim, 'swing_grab', { id: s.id, x: p.x, y: p.y - HANDS });
        fxBurst(sim, 'dust', p.x, p.y - HANDS, 5, { color: '#d9c9a8', speed: 70, life: 0.3 });
    }

    // Launch velocity is the tangential speed of the grip plus a small upward and forward boost when jumping off.
    function release(sim, s, boost, input) {
        const p = sim.player;
        const sgn = s.omega !== 0 ? Math.sign(s.omega) : (p.facing || 1);
        const gp = p.swing ? p.swing.grip : s.len;
        let vx = s.omega * gp * Math.cos(s.angle);
        let vy = -s.omega * gp * Math.sin(s.angle);
        if (boost) {
            vy -= JUMP_BOOST;
            vx += sgn * FWD_BOOST;
        }
        p.vx = vx;
        p.vy = vy;
        p.swing = null;
        p.swingCool = RELEASE_COOL;
        p.swingCoolId = s.id;
        p.swingAway = false;
        p.onGround = false;
        if (input) input.jumpPressed = false;
        pushEvent(sim, 'swing_release', { id: s.id, x: p.x, y: p.y - HANDS, speed: Math.hypot(vx, vy) });
        fxBurst(sim, 'trail', p.x, p.y - HANDS, 5, { color: '#d9c9a8', speed: 60, life: 0.3 });
    }

    // Hanging hero: a real jump press releases with a boost (Up only climbs), Down climbs down and lets go at the end.
    function carry(sim, s, dt, input) {
        const p = sim.player;
        const sw = p.swing;
        sw.t += dt;
        if (sw.t > 0.08 && input.jumpPressed && (input.jumpKey || !input.up)) {
            release(sim, s, true, input);
            return false;
        }
        const og = sw.grip;
        if (input.up && !input.down) sw.grip = Math.max(s.len * CLIMB_TOP, sw.grip - CLIMB_SPEED * dt);
        else if (input.down) {
            if (sw.grip < s.len) {
                sw.grip = Math.min(s.len, sw.grip + CLIMB_SPEED * dt);
                sw.endT = 0;
            } else {
                sw.endT = (sw.endT || 0) + dt;
                if (sw.endT >= LET_GO) {
                    release(sim, s, false, input);
                    return false;
                }
            }
        } else sw.endT = 0;
        const dir = (input.right ? 1 : 0) - (input.left ? 1 : 0);
        if (dir !== 0) p.facing = dir;
        else if (Math.abs(s.omega) > 0.5) p.facing = s.omega > 0 ? 1 : -1;
        const ox = p.x;
        const oy = p.y;
        seat(sim, s);
        if (sw.grip !== og && blocked(sim, p.x, p.y, oy)) {
            // Climbing into a ceiling or beam: stay at the old grip instead of embedding.
            sw.grip = og;
            seat(sim, s);
        }
        if (blocked(sim, p.x, p.y, oy)) {
            // Bonk: the arc runs into a solid tile or platform, so the swing stops dead and never seats the hero inside it.
            s.angle = s.pa;
            s.omega = 0;
            seat(sim, s);
            if (blocked(sim, p.x, p.y, oy)) {
                p.x = ox;
                p.y = oy;
                release(sim, s, false, null);
                return false;
            }
        }
        return true;
    }

    function tryGrab(sim, input) {
        const p = sim.player;
        if (p.onGround || p.onLadder || p.swingCool > 0 || p.hurtTimer > 0 || p.dashTimer > 0 || p.slamActive) return false;
        const hx = p.x;
        const hy = p.y - HANDS;
        let best = null;
        let bestD = 1e9;
        let bestAlong = 0;
        const speed = Math.hypot(p.vx, p.vy);
        sim.stage.swings.forEach(function (s) {
            const r = distToRope(s, hx, hy);
            const mine = p.swingCoolId === s.id;
            if (mine && r.dist > MAGNET_RADIUS + 16) p.swingAway = true;
            // the rope just left stays untouchable for the cool-down and, at speed, until the hero has flown clear of it
            if (p.swingCool > 0 && (!p.swingCoolId || mine)) return;
            if (mine && !p.swingAway && speed > 250) return;
            let ok = r.dist < GRAB_RADIUS;
            if (!ok && r.dist < MAGNET_RADIUS && speed > 20) {
                // magnet: the closest rope point lies in the direction of travel
                const dx = Math.sin(s.angle);
                const dy = Math.cos(s.angle);
                const t = Math.min(s.len, Math.max(s.len * GRAB_FROM, r.along));
                ok = ((s.ax + dx * t - hx) * p.vx + (s.ay + dy * t - hy) * p.vy) > 0;
            }
            if (ok && r.dist < bestD) {
                best = s;
                bestD = r.dist;
                bestAlong = r.along;
            }
        });
        if (!best) return false;
        input.jumpPressed = false;
        grab(sim, best, bestAlong);
        return true;
    }

    // Per-tick entry called by sim.js before the normal movement: returns true while the hero hangs on a rope.
    function update(sim, dt, input) {
        const stage = sim.stage;
        const list = stage.swings;
        if (!list || !list.length) return false;
        const p = sim.player;
        if (p.swingCool > 0) p.swingCool = Math.max(0, p.swingCool - dt);
        const held = p.swing ? find(list, p.swing.id) : null;
        if (p.swing && !held) p.swing = null;
        const dir = (input.right ? 1 : 0) - (input.left ? 1 : 0);
        if (held) {
            held.pa = held.angle;
            held.po = held.omega;
        }
        list.forEach(function (s) { integrate(s, s === held ? dir : 0, s === held, dt); });
        if (held) return carry(sim, held, dt, input);
        if (!stage.swingHinted && list.some(function (s) { return Math.abs(p.x - s.ax) < HINT_RANGE; })) {
            stage.swingHinted = true;
            sim.notice = 'runenpfad.notice.swing_hint';
        }
        return tryGrab(sim, input);
    }

    root.RunenpfadSwing = {
        ZONE_IDS: ZONE_IDS,
        has: has,
        build: build,
        update: update,
        endOf: endOf,
        GRAVITY: GRAVITY,
        MAX_ANGLE: MAX_ANGLE
    };
})(typeof window !== 'undefined' ? window : globalThis);
