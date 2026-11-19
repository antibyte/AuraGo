/* Runenpfad swinging and climbing ropes: multi-link Verlet physics, grab / pump / release and swing_gorge. DOM-free. */
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

    const SWING_SEGMENTS = 12;
    const CLIMB_SEGMENTS = 10;
    const SOLVER_ITERS = 8;
    const CLIMB_ITERS = 6;
    const MAX_NODE_STEP = 26;
    const SIM_DT = 1 / 60;

    const MAT = {
        vine: { drag: 0.992, breeze: 220, wind: 680, bend: 0.045, tipInvMass: 0.55, gravMul: 0.96, dust: '#86efac' },
        rope: { drag: 0.990, breeze: 185, wind: 540, bend: 0.018, tipInvMass: 0.45, gravMul: 1.00, dust: '#d9c9a8' },
        chain: { drag: 0.986, breeze: 105, wind: 300, bend: 0.000, tipInvMass: 0.35, gravMul: 1.14, dust: '#cbd5e1' }
    };

    function P() {
        return root.RunenpfadPhysics;
    }

    function clamp(v, lo, hi) {
        return v < lo ? lo : v > hi ? hi : v;
    }

    function matFor(style) {
        return MAT[style] || MAT.rope;
    }

    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }
    function fxBurst(sim, kind, x, y, count, opts) { P().fxBurst(sim, kind, x, y, count, opts); }

    function has(zone) {
        return ZONE_IDS.indexOf(zone) >= 0;
    }

    function styleFor(worldIndex) {
        return worldIndex === 0 ? 'vine' : (worldIndex === 4 ? 'chain' : 'rope');
    }

    function initSwingNodes(s) {
        const len = s.len || 270;
        const segLen = len / SWING_SEGMENTS;
        const ang = s.angle || 0;
        const om = s.omega || 0;
        const sin = Math.sin(ang);
        const cos = Math.cos(ang);
        const nodes = [];
        for (let k = 0; k <= SWING_SEGMENTS; k++) {
            const d = k * segLen;
            const x = s.ax + d * sin;
            const y = s.ay + d * cos;
            const vx = om * d * cos;
            const vy = -om * d * sin;
            nodes.push({ x: x, y: y, px: x - vx * SIM_DT, py: y - vy * SIM_DT });
        }
        s.segLen = segLen;
        s.nodes = nodes;
        s.flex = s.flex || 0;
        s.flexV = s.flexV || 0;
        s.grabPulse = s.grabPulse || 0;
        s._la = ang;
        s._lo = om;
    }

    function ensureSwingNodes(s) {
        if (!s.nodes || s.nodes.length !== SWING_SEGMENTS + 1 ||
            Math.abs((s.angle || 0) - (s._la || 0)) > 1e-6 ||
            Math.abs((s.omega || 0) - (s._lo || 0)) > 1e-6) {
            initSwingNodes(s);
            s.flex = 0;
            s.flexV = 0;
        }
    }

    function initClimbNodes(r) {
        const top = r.top;
        const len = Math.max(24, (r.bottom || top + 120) - top);
        const segLen = len / CLIMB_SEGMENTS;
        const nodes = [];
        for (let k = 0; k <= CLIMB_SEGMENTS; k++) {
            const y = top + k * segLen;
            nodes.push({ x: r.x, y: y, px: r.x, py: y });
        }
        r.segLen = segLen;
        r.nodes = nodes;
    }

    function ensureClimbNodes(r) {
        if (!r.nodes || r.nodes.length !== CLIMB_SEGMENTS + 1) initClimbNodes(r);
    }

    function applyImpulse(nodes, idx, dvx, dvy, dt, spread) {
        const n = nodes.length - 1;
        for (let k = 1; k <= n; k++) {
            const dist = Math.abs(k - idx);
            if (dist > spread) continue;
            const w = 1 - dist / spread;
            nodes[k].px -= dvx * dt * w;
            nodes[k].py -= dvy * dt * w;
        }
    }

    function integrateNodes(sim, nodes, ax, phase, mat, dt, baseGrav, breezeScale) {
        const nCount = nodes.length - 1;
        const t = sim.elapsed || 0;
        const wind = sim.wind;
        const runs = sim.stage && sim.stage.dragonRuns;
        let dragonGust = 0;
        if (runs) {
            for (let i = 0; i < runs.length; i++) {
                const dr = runs[i];
                if (dr && dr.state === 'strafe' && isFinite(dr.dx) && Math.abs(dr.dx - ax) < 360) {
                    dragonGust += (dr.dir || 1) * (1 - Math.abs(dr.dx - ax) / 360) * 520;
                }
            }
        }
        const fy = baseGrav * mat.gravMul;
        const dt2 = dt * dt;
        for (let k = 1; k <= nCount; k++) {
            const node = nodes[k];
            let vx = (node.x - node.px) * mat.drag;
            let vy = (node.y - node.py) * mat.drag;
            const sp = Math.hypot(vx, vy);
            if (sp > MAX_NODE_STEP) {
                const sc = MAX_NODE_STEP / sp;
                vx *= sc;
                vy *= sc;
            }
            const u = k / nCount;
            // Off-resonance travelling breeze keeps idle sway gentle and continuous.
            let fx = (Math.sin(t * 1.15 + phase * 1.7 - u * 2.2) * 0.72 +
                Math.sin(t * 4.6 + phase * 0.9 - u * 4.2) * 0.28) * mat.breeze * breezeScale * (0.25 + 0.75 * u);
            if (wind) {
                if (wind.state === 'gust' && wind.strength > 0) {
                    fx += (wind.dir || -1) * wind.strength * mat.wind * (0.35 + 0.65 * u);
                } else if (wind.state === 'warn') {
                    fx += Math.sin(t * 14 + phase + u * 4) * 0.18 * mat.wind * (0.3 + 0.7 * u);
                }
            }
            if (dragonGust !== 0) fx += dragonGust * u;
            node.px = node.x;
            node.py = node.y;
            node.x += vx + fx * dt2;
            node.y += vy + fy * dt2;
        }
    }

    function applyBodyInteractions(sim, nodes, ax, dt) {
        const p = sim.player;
        const nCount = nodes.length - 1;
        if (p) {
            const speed = Math.hypot(p.vx, p.vy);
            if (speed > 40) {
                const bx = p.x;
                const by = p.y - 24;
                for (let k = 2; k <= nCount; k++) {
                    const node = nodes[k];
                    const d = Math.hypot(node.x - bx, node.y - by);
                    if (d < 24) {
                        const w = 1 - d / 24;
                        node.px -= p.vx * dt * 0.16 * w;
                        node.py -= p.vy * dt * 0.08 * w;
                    }
                }
            }
            if (p.attackTimer > 0 && Math.abs(p.x - ax) < 90) {
                const sx = p.x + p.facing * 24;
                const sy = p.y - 24;
                for (let k = 1; k <= nCount; k++) {
                    const node = nodes[k];
                    if (Math.abs(node.x - sx) < 52 && Math.abs(node.y - sy) < 38) {
                        node.px -= p.facing * 1800 * dt * dt;
                        node.py += 320 * dt * dt;
                    }
                }
            }
        }
        const projs = sim.projectiles;
        if (projs && projs.length) {
            for (let i = 0; i < projs.length; i++) {
                const pr = projs[i];
                if (!pr || pr.dead) continue;
                for (let k = 1; k <= nCount; k++) {
                    const node = nodes[k];
                    const d = Math.hypot(node.x - pr.x, node.y - pr.y);
                    if (d < 20) {
                        const w = 1 - d / 20;
                        node.px -= clamp(pr.vx || 0, -500, 500) * dt * 0.18 * w;
                        node.py -= clamp(pr.vy || 0, -300, 300) * dt * 0.10 * w;
                    }
                }
            }
        }
    }

    function applyPin(nodes, pin) {
        if (!pin) return;
        if (pin.g0 === pin.g1) {
            if (pin.g0 > 0) {
                nodes[pin.g0].x = pin.x;
                nodes[pin.g0].y = pin.y;
            }
            return;
        }
        const a = nodes[pin.g0];
        const b = nodes[pin.g1];
        const w0 = 1 - pin.f;
        const w1 = pin.f;
        if (pin.g0 === 0) {
            b.x = (pin.x - a.x * w0) / w1;
            b.y = (pin.y - a.y * w0) / w1;
            return;
        }
        const den = w0 * w0 + w1 * w1;
        const dx = pin.x - (a.x * w0 + b.x * w1);
        const dy = pin.y - (a.y * w0 + b.y * w1);
        a.x += dx * w0 / den;
        a.y += dy * w0 / den;
        b.x += dx * w1 / den;
        b.y += dy * w1 / den;
    }

    function solveChain(sim, nodes, ax, ay, segLen, mat, iters, pin) {
        const nCount = nodes.length - 1;
        const stage = sim.stage;
        const T = stage.tileSize || 48;
        const bend = mat.bend || 0;
        const phys = P();
        for (let it = 0; it < iters; it++) {
            nodes[0].x = ax;
            nodes[0].y = ay;
            applyPin(nodes, pin);
            for (let k = 0; k < nCount; k++) {
                const a = nodes[k];
                const b = nodes[k + 1];
                let ia = k === 0 ? 0 : 1;
                let ib = k + 1 === nCount ? mat.tipInvMass : 1;
                if (pin && pin.g0 === pin.g1) {
                    if (k === pin.g0) ia = 0;
                    if (k + 1 === pin.g0) ib = 0;
                }
                const sum = ia + ib;
                if (sum <= 0) continue;
                const dx = b.x - a.x;
                const dy = b.y - a.y;
                const d = Math.hypot(dx, dy);
                if (d < 1e-5) continue;
                const diff = (d - segLen) / (d * sum);
                if (ia > 0) { a.x += dx * diff * ia; a.y += dy * diff * ia; }
                if (ib > 0) { b.x -= dx * diff * ib; b.y -= dy * diff * ib; }
            }
            if (bend > 0 && it < 2) {
                for (let k = 1; k < nCount; k++) {
                    if (pin && (k === pin.g0 || k === pin.g1)) continue;
                    const prev = nodes[k - 1];
                    const cur = nodes[k];
                    const next = nodes[k + 1];
                    cur.x += ((prev.x + next.x) * 0.5 - cur.x) * bend;
                    cur.y += ((prev.y + next.y) * 0.5 - cur.y) * bend;
                }
            }
            if (phys && (it === iters - 1 || it === Math.floor(iters / 2))) {
                for (let k = 1; k <= nCount; k++) {
                    if (pin && (k === pin.g0 || k === pin.g1)) continue;
                    const node = nodes[k];
                    const tx = Math.floor(node.x / T);
                    const ty = Math.floor(node.y / T);
                    if (phys.isSolidTile(stage, tx, ty)) {
                        const cx = tx * T + T * 0.5;
                        const dir = node.px < cx ? -1 : node.px > cx ? 1 : (ax < cx ? -1 : 1);
                        node.x = dir < 0 ? tx * T - 1 : (tx + 1) * T + 1;
                    }
                }
            }
        }
        nodes[0].x = ax;
        nodes[0].y = ay;
        applyPin(nodes, pin);
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
            const sw = {
                id: 'sw_' + ctx.extras.swings.length,
                group: group,
                style: style,
                ax: rx * T,
                ay: ay,
                len: len,
                angle: 0.09 * (i % 2 === 0 ? 1 : -1),
                omega: 0,
                phase: i
            };
            initSwingNodes(sw);
            ctx.extras.swings.push(sw);
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

    // One held pendulum tick. Pumping only helps on the swing toward the held direction, so holding one key builds amplitude.
    function integrateHeld(sim, s, grip, dir, dt) {
        const effLen = s.len * 0.65 + grip * 0.35;
        let acc = -(GRAVITY / effLen) * Math.sin(s.angle);
        if (dir !== 0 && s.omega * dir >= -0.1) acc += dir * PUMP * (0.5 + 0.5 * Math.cos(s.angle));
        if (sim.wind && sim.wind.state === 'gust' && sim.wind.strength > 0) {
            acc += (sim.wind.dir || -1) * sim.wind.strength * 0.65 * Math.cos(s.angle);
        }
        s.omega += acc * dt;
        s.omega *= 1 - 0.12 * dt;
        s.angle += s.omega * dt;
        if (s.angle > MAX_ANGLE || s.angle < -MAX_ANGLE) {
            s.angle = Math.sign(s.angle) * MAX_ANGLE;
            if (s.omega * s.angle > 0) s.omega = 0;
        }
        s.flexV = (s.flexV || 0) + (-420 * (s.flex || 0) - 24 * (s.flexV || 0)) * dt;
        s.flex = clamp((s.flex || 0) + s.flexV * dt, -2.0, 2.0);
    }

    function stepFreeSwing(sim, s, dt, skipBody) {
        const mat = matFor(s.style);
        s.grabPulse = Math.max(0, (s.grabPulse || 0) - dt * 2.5);
        s.flex = 0;
        s.flexV = 0;
        integrateNodes(sim, s.nodes, s.ax, s.phase || 0, mat, dt, GRAVITY, 1.0);
        if (!skipBody) applyBodyInteractions(sim, s.nodes, s.ax, dt);
        solveChain(sim, s.nodes, s.ax, s.ay, s.segLen, mat, SOLVER_ITERS, null);
        const tip = s.nodes[s.nodes.length - 1];
        const prevA = s.angle;
        const nextA = clamp(Math.atan2(tip.x - s.ax, tip.y - s.ay), -MAX_ANGLE, MAX_ANGLE);
        s.omega = clamp((nextA - prevA) / dt, -5, 5);
        s.angle = nextA;
        s._la = s.angle;
        s._lo = s.omega;
    }

    function stepHeldSwing(sim, s, sw, dt) {
        const mat = matFor(s.style);
        s.grabPulse = Math.max(0, (s.grabPulse || 0) - dt * 2.5);
        integrateNodes(sim, s.nodes, s.ax, s.phase || 0, mat, dt, GRAVITY, 0.35);
        const gripIdx = clamp(sw.grip / (s.segLen || (s.len / SWING_SEGMENTS)), 0.5, SWING_SEGMENTS);
        let g0 = Math.floor(gripIdx);
        let g1 = Math.min(SWING_SEGMENTS, g0 + 1);
        let f = gripIdx - g0;
        if (f < 1e-4) { g1 = g0; f = 0; }
        else if (f > 1 - 1e-4) { g0 = g1; f = 0; }
        const p = sim.player;
        solveChain(sim, s.nodes, s.ax, s.ay, s.segLen, mat, SOLVER_ITERS, { g0: g0, g1: g1, f: f, x: p.x, y: p.y - HANDS });
        s._la = s.angle;
        s._lo = s.omega;
    }

    function stepBodyAngle(s, sw, dir, climbing, dt) {
        const angAcc = (s.omega - (s.po != null ? s.po : s.omega)) / dt;
        if (climbing) sw.climbPhase = (sw.climbPhase || 0) + dt * 11;
        const target = s.angle * 0.88 - clamp(angAcc * 0.032, -0.28, 0.28) + dir * 0.12 +
            (climbing ? Math.sin(sw.climbPhase || 0) * 0.08 : 0);
        sw.bodyOmega = (sw.bodyOmega || 0) + ((target - (sw.bodyAngle || 0)) * 78 - (sw.bodyOmega || 0) * 9.5) * dt;
        sw.bodyAngle = clamp((sw.bodyAngle || 0) + sw.bodyOmega * dt, -1.45, 1.45);
    }

    function endOf(s) {
        if (s.nodes && s.nodes.length) {
            const last = s.nodes[s.nodes.length - 1];
            return { x: last.x, y: last.y };
        }
        return { x: s.ax + s.len * Math.sin(s.angle), y: s.ay + s.len * Math.cos(s.angle) };
    }

    function distToRope(s, hx, hy) {
        const nodes = s.nodes;
        if (!nodes || nodes.length < 2) {
            const dx = Math.sin(s.angle);
            const dy = Math.cos(s.angle);
            const along = (hx - s.ax) * dx + (hy - s.ay) * dy;
            const t = Math.min(s.len, Math.max(s.len * GRAB_FROM, along));
            const px = s.ax + dx * t;
            const py = s.ay + dy * t;
            return { dist: Math.hypot(hx - px, hy - py), along: along, px: px, py: py };
        }
        const minAlong = s.len * GRAB_FROM;
        let bestD = 1e9;
        let bestAlong = s.len;
        let bestPx = nodes[nodes.length - 1].x;
        let bestPy = nodes[nodes.length - 1].y;
        let cum = 0;
        for (let k = 0; k < nodes.length - 1; k++) {
            const a = nodes[k];
            const b = nodes[k + 1];
            const sx = b.x - a.x;
            const sy = b.y - a.y;
            const sl2 = sx * sx + sy * sy;
            const sl = Math.sqrt(sl2) || 1;
            let u = sl2 > 1e-6 ? ((hx - a.x) * sx + (hy - a.y) * sy) / sl2 : 0;
            u = clamp(u, 0, 1);
            let along = cum + u * sl;
            if (along < minAlong) {
                const need = minAlong - cum;
                if (need > sl) { cum += sl; continue; }
                u = clamp(need / sl, 0, 1);
                along = minAlong;
            }
            const px = a.x + sx * u;
            const py = a.y + sy * u;
            const d = Math.hypot(hx - px, hy - py);
            if (d < bestD) {
                bestD = d;
                bestAlong = along;
                bestPx = px;
                bestPy = py;
            }
            cum += sl;
        }
        return { dist: bestD, along: bestAlong, px: bestPx, py: bestPy };
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
        const effGrip = gp + (s.flex || 0);
        p.x = s.ax + effGrip * Math.sin(s.angle);
        p.y = s.ay + effGrip * Math.cos(s.angle) + HANDS;
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

    function grab(sim, s, along, px, py) {
        const p = sim.player;
        const mat = matFor(s.style);
        const gp = Math.min(s.len, Math.max(s.len * GRAB_FROM, along));
        const grabAng = (px != null && py != null) ? clamp(Math.atan2(px - s.ax, py - s.ay), -MAX_ANGLE, MAX_ANGLE) : s.angle;
        const vTan = p.vx * Math.cos(grabAng) - p.vy * Math.sin(grabAng);
        const vRad = p.vx * Math.sin(grabAng) + p.vy * Math.cos(grabAng);
        s.angle = grabAng;
        s.omega = clamp(s.omega * 0.25 + (vTan / Math.max(90, gp)) * 0.78, -3.4, 3.4);
        s.flex = 0;
        s.flexV = clamp(vRad * 0.055, -35, 55);
        s.grabPulse = clamp(Math.hypot(p.vx, p.vy) / 360, 0.3, 1);
        if (s.nodes && s.nodes.length) {
            const idx = clamp(gp / (s.segLen || (s.len / SWING_SEGMENTS)), 0.5, SWING_SEGMENTS);
            applyImpulse(s.nodes, idx, p.vx * 0.35, p.vy * 0.2, SIM_DT, 4);
            for (let k = Math.ceil(idx); k <= SWING_SEGMENTS; k++) {
                s.nodes[k].px -= p.vx * 0.18 * SIM_DT;
            }
        }
        p.swingCoolId = null;
        p.swing = {
            id: s.id, angle: s.angle, omega: s.omega, grip: gp, t: 0, endT: 0,
            bodyAngle: grabAng * 0.85, bodyOmega: s.omega * 0.35, climbPhase: 0
        };
        const runes = root.RunenpfadRunes;
        p.airJumpsLeft = sim.stats.maxAirJumps + (runes ? runes.extraAirJumps(sim) : 0);
        seat(sim, s);
        s._la = s.angle;
        s._lo = s.omega;
        pushEvent(sim, 'swing_grab', { id: s.id, x: p.x, y: p.y - HANDS });
        fxBurst(sim, 'dust', p.x, p.y - HANDS, 5, { color: mat.dust, speed: 70, life: 0.3 });
    }

    // Launch velocity is the tangential speed of the grip plus a small upward and forward boost when jumping off.
    function release(sim, s, boost, input) {
        const p = sim.player;
        const mat = matFor(s.style);
        const sgn = s.omega !== 0 ? Math.sign(s.omega) : (p.facing || 1);
        const gp = p.swing ? p.swing.grip : s.len;
        let vx = s.omega * gp * Math.cos(s.angle);
        let vy = -s.omega * gp * Math.sin(s.angle);
        if (boost) {
            vy -= JUMP_BOOST;
            vx += sgn * FWD_BOOST;
        }
        if (s.nodes && s.nodes.length) {
            const idx = clamp(gp / (s.segLen || (s.len / SWING_SEGMENTS)), 0.5, SWING_SEGMENTS);
            const kickX = boost ? -sgn * 240 : -sgn * 90;
            const kickY = boost ? 95 : 25;
            applyImpulse(s.nodes, idx, kickX, kickY, SIM_DT, 4.5);
            for (let k = Math.ceil(idx); k <= SWING_SEGMENTS; k++) {
                s.nodes[k].px -= kickX * 0.45 * SIM_DT;
            }
            s.grabPulse = boost ? 1 : 0.4;
            s._la = s.angle;
            s._lo = s.omega;
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
        fxBurst(sim, 'trail', p.x, p.y - HANDS, 5, { color: mat.dust, speed: 60, life: 0.3 });
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
        stepBodyAngle(s, sw, dir, sw.grip !== og, dt);
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
        let bestPx = hx;
        let bestPy = hy;
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
                ok = ((r.px - hx) * p.vx + (r.py - hy) * p.vy) > 0;
            }
            if (ok && r.dist < bestD) {
                best = s;
                bestD = r.dist;
                bestAlong = r.along;
                bestPx = r.px;
                bestPy = r.py;
            }
        });
        if (!best) return false;
        input.jumpPressed = false;
        grab(sim, best, bestAlong, bestPx, bestPy);
        return true;
    }

    function stepClimbRopes(sim, dt) {
        const ropes = sim.stage && sim.stage.ropes;
        if (!ropes || !ropes.length) return;
        const p = sim.player;
        const mat = MAT.rope;
        for (let i = 0; i < ropes.length; i++) {
            const r = ropes[i];
            ensureClimbNodes(r);
            integrateNodes(sim, r.nodes, r.x, i * 1.3 + r.x * 0.01, mat, dt, 1750, 0.84);
            if (!(p && p.onLadder && p.climbKind === 'rope' && p.climbRopeId === r.id)) {
                applyBodyInteractions(sim, r.nodes, r.x, dt);
            }
            solveChain(sim, r.nodes, r.x, r.top, r.segLen, mat, CLIMB_ITERS, null);
        }
    }

    function ropeX(r, y) {
        if (!r || !r.nodes || r.nodes.length < 2) return r ? r.x : 0;
        const nodes = r.nodes;
        for (let k = 0; k < nodes.length - 1; k++) {
            const a = nodes[k];
            const b = nodes[k + 1];
            if (y <= b.y || k === nodes.length - 2) {
                const dy = b.y - a.y;
                const u = Math.abs(dy) > 1e-4 ? clamp((y - a.y) / dy, 0, 1) : 0;
                return a.x + (b.x - a.x) * u;
            }
        }
        return r.x;
    }

    function onClimbGrab(sim, r, handY, vx, vy) {
        if (!r) return;
        ensureClimbNodes(r);
        const idx = clamp((handY - r.top) / r.segLen, 0.5, CLIMB_SEGMENTS);
        const impX = clamp(vx * 0.65, -240, 240);
        const impY = clamp(vy * 0.15, -60, 60);
        applyImpulse(r.nodes, idx, impX, impY, SIM_DT, 4.5);
        for (let k = Math.ceil(idx); k <= CLIMB_SEGMENTS; k++) {
            r.nodes[k].px -= impX * 0.35 * SIM_DT;
        }
        fxBurst(sim, 'dust', ropeX(r, handY), handY, 4, { color: MAT.rope.dust, speed: 55, life: 0.25 });
    }

    function holdClimbRope(sim, r, p, moveDir, climbDir, dt) {
        if (!r) return p.x;
        ensureClimbNodes(r);
        const nodes = r.nodes;
        const handY = clamp(p.y - 26, r.top + 8, r.bottom - 8);
        const gripIdx = clamp((handY - r.top) / r.segLen, 0.5, CLIMB_SEGMENTS - 0.5);
        let g0 = Math.floor(gripIdx);
        let g1 = Math.min(CLIMB_SEGMENTS, g0 + 1);
        let f = gripIdx - g0;
        if (f < 1e-4) { g1 = g0; f = 0; }
        else if (f > 1 - 1e-4) { g0 = g1; f = 0; }
        const climbWiggle = climbDir !== 0 ? Math.sin((sim.elapsed || 0) * 9) * 140 : 0;
        const pullX = (moveDir * 320 + climbWiggle) * dt * dt;
        const restore = Math.min(1, 3.2 * dt);
        nodes[g0].x += pullX + (r.x - nodes[g0].x) * restore;
        if (g1 !== g0) nodes[g1].x += pullX + (r.x - nodes[g1].x) * restore;
        const handX = clamp(ropeX(r, handY), r.x - 10, r.x + 10);
        solveChain(sim, nodes, r.x, r.top, r.segLen, MAT.rope, 3, { g0: g0, g1: g1, f: f, x: handX, y: handY });
        const a = nodes[Math.max(0, g0 - 1)];
        const b = nodes[Math.min(CLIMB_SEGMENTS, g1 + 1)];
        const ang = Math.atan2(b.x - a.x, Math.max(8, b.y - a.y));
        p.climbTilt = clamp(ang * 0.55, -0.2, 0.2);
        return handX;
    }

    function onClimbRelease(sim, r, handY, dir, jumped) {
        if (!r) return;
        ensureClimbNodes(r);
        const idx = clamp((handY - r.top) / r.segLen, 0.5, CLIMB_SEGMENTS);
        const kick = -(dir || 1) * (jumped ? 310 : 120);
        applyImpulse(r.nodes, idx, kick, jumped ? 85 : 20, SIM_DT, 4.5);
        for (let k = Math.ceil(idx); k <= CLIMB_SEGMENTS; k++) {
            r.nodes[k].px -= kick * 0.45 * SIM_DT;
        }
        if (jumped) fxBurst(sim, 'dust', ropeX(r, handY), handY, 4, { color: MAT.rope.dust, speed: 60, life: 0.25 });
    }

    // Per-tick entry called by sim.js before the normal movement: returns true while the hero hangs on a rope.
    function update(sim, dt, input) {
        stepClimbRopes(sim, dt);
        const stage = sim.stage;
        const list = stage.swings;
        if (!list || !list.length) return false;
        const p = sim.player;
        if (p.swingCool > 0) p.swingCool = Math.max(0, p.swingCool - dt);
        const held = p.swing ? find(list, p.swing.id) : null;
        if (p.swing && !held) p.swing = null;
        const dir = (input.right ? 1 : 0) - (input.left ? 1 : 0);
        for (let i = 0; i < list.length; i++) {
            const s = list[i];
            ensureSwingNodes(s);
            if (s === held) {
                s.pa = s.angle;
                s.po = s.omega;
                integrateHeld(sim, s, p.swing.grip, dir, dt);
            } else {
                stepFreeSwing(sim, s, dt, p.swingCool > 0 && p.swingCoolId === s.id);
            }
        }
        if (held) {
            const stillHeld = carry(sim, held, dt, input);
            if (stillHeld) stepHeldSwing(sim, held, p.swing, dt);
            return stillHeld;
        }
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
        ropeX: ropeX,
        onClimbGrab: onClimbGrab,
        holdClimbRope: holdClimbRope,
        onClimbRelease: onClimbRelease,
        GRAVITY: GRAVITY,
        MAX_ANGLE: MAX_ANGLE
    };
})(typeof window !== 'undefined' ? window : globalThis);
