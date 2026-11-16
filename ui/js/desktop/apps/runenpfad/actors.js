/* Runenpfad actor layer: enemies, hero and projectiles with squash & stretch, afterimages, hit flash and dissolve. */
(function (root) {
    'use strict';

    const TAU = Math.PI * 2;
    const GHOSTS = 5;
    const MAX_PARTS = 140;
    const DEATH_TIME = 0.55;

    function clamp(v, lo, hi) {
        return v < lo ? lo : v > hi ? hi : v;
    }

    function hash(n) {
        let h = Math.imul(n ^ 0x9e3779b9, 0x27d4eb2d);
        h ^= h >>> 15;
        h = Math.imul(h, 0x85ebca6b);
        h ^= h >>> 13;
        return (h >>> 0) / 4294967296;
    }

    function create(h) {
        if (!h || !h.ctx) return null;
        const ctx = h.ctx;
        const doc = ctx.canvas && ctx.canvas.ownerDocument;
        let scratch = null;
        let scratchCtx = null;
        let seq = 1;
        const heroVis = { climbClock: 100, sx: 1, sy: 1, vsx: 0, vsy: 0, fallVy: 0, wasGround: true, ghosts: [], ghostClock: 0, tilt: 0 };
        const ent = {}; // per-enemy visual state keyed by id
        const parts = [];
        let lastDt = 1 / 60;
        let stepped = -1;

        function rnd() {
            seq = (Math.imul(seq, 1664525) + 1013904223) >>> 0;
            return seq / 4294967296;
        }

        function reduced() {
            return !!h.reducedMotion;
        }

        function spring(s, dt) {
            const n = Math.max(1, Math.ceil(dt / 0.008));
            const step = dt / n;
            for (let i = 0; i < n; i++) {
                s.vsx += (-300 * (s.sx - 1) - 17 * s.vsx) * step;
                s.vsy += (-300 * (s.sy - 1) - 17 * s.vsy) * step;
                s.sx += s.vsx * step;
                s.sy += s.vsy * step;
            }
            s.sx = clamp(s.sx, 0.6, 1.5);
            s.sy = clamp(s.sy, 0.6, 1.5);
        }

        function addPart(kind, x, y, vx, vy, life, size, color) {
            if (parts.length >= MAX_PARTS) parts.shift();
            parts.push({ kind: kind, x: x, y: y, vx: vx, vy: vy, life: life, max: life, size: size, color: color });
        }

        function stepParts(dt) {
            for (let i = parts.length - 1; i >= 0; i--) {
                const p = parts[i];
                p.life -= dt;
                if (p.life <= 0) { parts.splice(i, 1); continue; }
                const drag = Math.max(0, 1 - 3 * dt);
                p.vx *= drag;
                p.vy = p.vy * drag + (p.kind === 'bit' ? 380 * dt : p.kind === 'dust' ? -26 * dt : 0);
                p.x += p.vx * dt;
                p.y += p.vy * dt;
            }
        }

        function drawParts() {
            if (!parts.length) return;
            ctx.save();
            for (let i = 0; i < parts.length; i++) {
                const p = parts[i];
                const t = p.life / p.max;
                if (p.kind === 'dust') {
                    ctx.globalAlpha = t * 0.5;
                    ctx.fillStyle = p.color;
                    ctx.beginPath();
                    ctx.arc(p.x, p.y, p.size * (1.6 - t * 0.7), 0, TAU);
                    ctx.fill();
                } else {
                    ctx.globalAlpha = Math.min(1, t * 1.6);
                    ctx.globalCompositeOperation = 'lighter';
                    ctx.fillStyle = p.color;
                    ctx.fillRect(p.x - p.size / 2, p.y - p.size / 2, p.size, p.size);
                    ctx.globalCompositeOperation = 'source-over';
                }
            }
            ctx.restore();
        }

        function step(view) {
            const key = view.nowSec;
            if (stepped === key) return;
            stepped = key;
            lastDt = clamp(view.dt || 1 / 60, 0, 0.05);
            spring(heroVis, lastDt);
            stepParts(lastDt);
        }

        // A missing prefix+state clip falls back to idle so a sprite can never vanish.
        function clipFor(pack, prefix, state) {
            const name = prefix + state;
            return h.hasAnim(pack, name) ? name : prefix + 'idle';
        }

        function flashCanvas() {
            if (!scratch && doc) {
                scratch = doc.createElement('canvas');
                scratch.width = 128;
                scratch.height = 128;
                scratchCtx = scratch.getContext('2d');
            }
            return scratchCtx;
        }

        // Draws one animation frame with pivot at (px, py): squash/stretch, tilt, flip, alpha and white hit-flash.
        function sprite(pack, anim, clock, hold, px, py, size, topLocal, flip, alpha, sx, sy, flash, tilt) {
            const fr = h.getFrame(pack, anim, clock, hold);
            if (!fr) return;
            ctx.save();
            ctx.translate(px, py);
            if (tilt) ctx.rotate(tilt);
            ctx.scale(flip ? -sx : sx, sy);
            if (alpha < 1) ctx.globalAlpha = Math.max(0, alpha);
            ctx.drawImage(fr.image, fr.sx, fr.sy, fr.sw, fr.sh, -size / 2, topLocal, size, size);
            if (flash > 0.02) {
                const sc = flashCanvas();
                if (sc) {
                    sc.setTransform(1, 0, 0, 1, 0, 0);
                    sc.globalCompositeOperation = 'source-over';
                    sc.clearRect(0, 0, 128, 128);
                    sc.drawImage(fr.image, fr.sx, fr.sy, fr.sw, fr.sh, 0, 0, fr.sw, fr.sh);
                    sc.globalCompositeOperation = 'source-in';
                    sc.fillStyle = '#ffffff';
                    sc.fillRect(0, 0, 128, 128);
                    ctx.globalAlpha = Math.min(1, flash) * alpha;
                    ctx.drawImage(scratch, 0, 0, fr.sw, fr.sh, -size / 2, topLocal, size, size);
                }
            }
            ctx.restore();
        }

        function dissolve(fr, px, py, size, topLocal, flip, progress, seed) {
            const bands = 14;
            const bh = fr.sh / bands;
            ctx.save();
            ctx.translate(px, py);
            if (flip) ctx.scale(-1, 1);
            for (let i = 0; i < bands; i++) {
                const r = hash(seed * 31 + i);
                const a = 1 - clamp(progress * 1.7 - r * 0.7, 0, 1);
                if (a <= 0.01) continue;
                ctx.globalAlpha = a;
                const off = (r - 0.5) * 34 * progress;
                const rise = -progress * (10 + r * 26);
                ctx.drawImage(fr.image, fr.sx, fr.sy + i * bh, fr.sw, bh + 0.5, -size / 2 + off, topLocal + (size / bands) * i + rise, size, size / bands + 0.5);
            }
            ctx.restore();
        }

        function glow(x, y, r, color, a) {
            h.drawGlow(x, y, r, color, a);
        }

        function stateFor(e) {
            let s = ent[e.id];
            if (!s) {
                s = ent[e.id] = { sx: 1, sy: 1, vsx: 0, vsy: 0, prevHurt: 0, prevAtk: 0, burst: false };
            }
            return s;
        }

        function drawBossAura(e, size, nowSec) {
            const phase2 = e.phase === 2;
            const cy = e.y - size * 0.42;
            const col = phase2 ? '#fb7185' : '#f7c948';
            const pulse = 0.5 + 0.5 * Math.sin(nowSec * (phase2 ? 7 : 3.4));
            glow(e.x, cy, size * (0.95 + pulse * 0.18), phase2 ? 'rgba(251, 113, 133, 0.55)' : 'rgba(247, 201, 72, 0.5)', 0.6 + pulse * 0.2);
            ctx.save();
            ctx.globalCompositeOperation = 'lighter';
            ctx.strokeStyle = col;
            ctx.lineWidth = 2;
            ctx.globalAlpha = 0.55 + pulse * 0.35;
            ctx.beginPath();
            ctx.arc(e.x, cy, size * 0.46 + pulse * 3, 0, TAU);
            ctx.stroke();
            ctx.fillStyle = phase2 ? '#ffd3da' : '#fff1b0';
            const n = phase2 ? 7 : 5;
            for (let i = 0; i < n; i++) {
                const a = nowSec * (phase2 ? 1.8 : 1.1) + (i * TAU) / n;
                const rx = e.x + Math.cos(a) * size * 0.58;
                const ry = cy + Math.sin(a) * size * 0.36;
                ctx.globalAlpha = 0.9;
                ctx.beginPath();
                ctx.moveTo(rx, ry - 4);
                ctx.lineTo(rx + 3, ry);
                ctx.lineTo(rx, ry + 4);
                ctx.lineTo(rx - 3, ry);
                ctx.closePath();
                ctx.fill();
            }
            ctx.restore();
        }

        function drawEnemies(sim, view) {
            step(view);
            const stage = sim.stage;
            const sx0 = view.sx;
            const dt = lastDt;
            const rm = view.reducedMotion;
            for (let i = 0; i < stage.enemies.length; i++) {
                const e = stage.enemies[i];
                if (e.dead && e.deathTimer <= 0) continue;
                if (e.x + 64 < sx0 || e.x - 64 > sx0 + view.viewW) continue;
                const st = stateFor(e);
                if (!e.dead) st.burst = false;
                if (!rm) {
                    if (e.hurtTimer > 0 && st.prevHurt <= 0 && !e.dead) { st.sx = 1.2; st.sy = 0.82; }
                    if (e.attackTimer > 0 && st.prevAtk <= 0 && !e.dead) { st.sx = 0.88; st.sy = 1.14; }
                    spring(st, dt);
                }
                st.prevHurt = e.hurtTimer;
                st.prevAtk = e.attackTimer;
                const size = Math.round(58 * e.def.scale);
                const topLocal = -Math.round(size * (59 / 64)); // v3 sheets: feet end on cell row 59 (origin y 0.9375)
                const attacking = !e.dead && e.attackTimer > 0 && h.hasAnim(e.def.pack, e.def.prefix + 'attack');
                const animName = clipFor(e.def.pack, e.def.prefix, e.dead ? 'death' : e.hurtTimer > 0 ? 'hurt' : attacking ? 'attack' : e.animState === 'idle' ? 'idle' : 'move');
                const clock = attacking ? Math.max(0, 0.45 - e.attackTimer) : e.animTime;
                if (!e.dead && !e.def.flying && e.onGround) h.drawContactShadow(e.x, e.y, size * 0.56);
                if (e.def.boss && !e.dead) drawBossAura(e, size, view.nowSec);
                if (e.dead) {
                    const prog = clamp(1 - e.deathTimer / DEATH_TIME, 0, 1);
                    const fr = h.getFrame(e.def.pack, animName, clock, true);
                    if (fr) dissolve(fr, e.x, e.y, size, topLocal, e.facing < 0, rm ? 0 : prog, i + 1);
                    if (!st.burst && !rm) {
                        st.burst = true;
                        const col = e.def.boss ? '#ffe28a' : '#e8f3ff';
                        for (let k = 0; k < (e.def.boss ? 26 : 12); k++) {
                            addPart('bit', e.x + (rnd() - 0.5) * size * 0.5, e.y - size * (0.15 + rnd() * 0.5), (rnd() - 0.5) * 220, -60 - rnd() * 200, 0.5 + rnd() * 0.4, 2 + rnd() * 3, col);
                        }
                    }
                    continue;
                }
                const flash = e.hurtTimer > 0 ? clamp(e.hurtTimer * 7, 0, 0.85) : 0;
                sprite(e.def.pack, animName, clock, attacking, e.x, e.y, size, topLocal, e.facing < 0, 1, st.sx, st.sy, flash, 0);
                if (!e.def.boss && e.hp < e.maxHp) {
                    const top = e.y + topLocal - 4;
                    ctx.fillStyle = 'rgba(0, 0, 0, 0.6)';
                    ctx.fillRect(e.x - 19, top - 1, 38, 7);
                    ctx.fillStyle = '#fb7185';
                    ctx.fillRect(e.x - 18, top, 36 * clamp(e.hp / e.maxHp, 0, 1), 5);
                    ctx.fillStyle = 'rgba(255, 255, 255, 0.35)';
                    ctx.fillRect(e.x - 18, top, 36 * clamp(e.hp / e.maxHp, 0, 1), 1.5);
                }
            }
        }

        // Tapered glowing crescent for the knight swing.
        function crescent(cx, cy, R, a0, a1, width, color, alpha) {
            ctx.fillStyle = color;
            ctx.globalAlpha = alpha;
            ctx.beginPath();
            const n = 14;
            for (let i = 0; i <= n; i++) {
                const t = i / n;
                const a = a0 + (a1 - a0) * t;
                const w = width * Math.pow(Math.sin(Math.PI * (0.12 + 0.88 * t)), 0.9) * t;
                ctx.lineTo(cx + Math.cos(a) * (R + w * 0.5), cy + Math.sin(a) * (R + w * 0.5));
            }
            for (let i = n; i >= 0; i--) {
                const t = i / n;
                const a = a0 + (a1 - a0) * t;
                const w = width * Math.pow(Math.sin(Math.PI * (0.12 + 0.88 * t)), 0.9) * t;
                ctx.lineTo(cx + Math.cos(a) * (R - w * 0.5), cy + Math.sin(a) * (R - w * 0.5));
            }
            ctx.closePath();
            ctx.fill();
        }

        function drawSwoosh(sim) {
            const p = sim.player;
            const dur = Math.max(0.01, sim.stats.def.attackDuration);
            const pr = clamp(1 - p.attackTimer / dur, 0, 1);
            const ease = 1 - Math.pow(1 - pr, 2.2);
            const head = -1.3 + ease * 2.4;
            const fade = pr < 0.55 ? 1 : 1 - (pr - 0.55) / 0.45;
            ctx.save();
            ctx.translate(p.x, p.y - 24);
            ctx.scale(p.facing < 0 ? -1 : 1, 1);
            ctx.globalCompositeOperation = 'lighter';
            crescent(12, 0, 40, head - 1.25, head, 26, '#3ccfe8', 0.5 * fade);
            crescent(12, 0, 40, head - 0.8, head, 18, '#9af3ff', 0.8 * fade);
            crescent(12, 0, 40, head - 0.45, head, 9, '#ffffff', 0.95 * fade);
            const tx = 12 + Math.cos(head) * 38;
            const ty = Math.sin(head) * 38;
            const g = ctx.createRadialGradient(tx, ty, 0, tx, ty, 14);
            g.addColorStop(0, 'rgba(255,255,255,0.95)');
            g.addColorStop(1, 'rgba(126,232,250,0)');
            ctx.globalAlpha = fade;
            ctx.fillStyle = g;
            ctx.fillRect(tx - 14, ty - 14, 28, 28);
            ctx.restore();
        }

        // Visual-only snap of the rear-view climber onto the ladder / rope centre.
        function climbCentre(sim, p) {
            const T = sim.stage.tileSize || 48;
            const ropes = sim.stage.ropes || [];
            const ladders = sim.stage.ladders || [];
            for (let i = 0; i < ropes.length; i++) if (Math.abs(p.x - ropes[i].x) <= 14) return ropes[i].x;
            for (let i = 0; i < ladders.length; i++) if (Math.abs(p.x - (ladders[i].x + T / 2)) < 20) return ladders[i].x + T / 2;
            return p.x;
        }

        function drawHero(sim, view) {
            step(view);
            const p = sim.player;
            const rm = view.reducedMotion;
            const def = sim.stats.def;
            let anim = clipFor(def.pack, def.prefix, p.animState);
            let clock = p.animTime;
            let flipHero = p.facing < 0;
            let drawX = p.x;
            const hold = p.animState === 'death';
            if (p.animState === 'climb') {
                // Rear-view climb clips never flip; the clock only runs while moving and reverses when descending.
                const ck = def.prefix + 'climb_' + (p.climbKind || 'ladder');
                if (h.hasAnim(def.pack, ck)) {
                    anim = ck;
                    flipHero = false;
                    drawX = climbCentre(sim, p);
                    if (p.climbDir !== 0) {
                        heroVis.climbClock += (p.climbDir === 1 ? -1 : 1) * lastDt;
                        if (heroVis.climbClock < 0) heroVis.climbClock += 100;
                    }
                    clock = heroVis.climbClock;
                } else {
                    anim = def.prefix + 'idle';
                }
            }
            if (!rm) {
                if (!p.onGround && p.vy > heroVis.fallVy) heroVis.fallVy = p.vy;
                if (p.onGround) heroVis.fallVy = 0;
                const tilt = p.onGround ? clamp(p.vx / 420, -1, 1) * 0.07 : clamp(p.vx / 600, -1, 1) * 0.04;
                heroVis.tilt += (tilt - heroVis.tilt) * Math.min(1, lastDt * 14);
                if (p.dashTimer > 0) {
                    heroVis.ghostClock -= lastDt;
                    if (heroVis.ghostClock <= 0) {
                        heroVis.ghostClock = 0.035;
                        heroVis.ghosts.push({ x: p.x, y: p.y, f: p.facing, anim: anim, clock: p.animTime, life: 0.26 });
                        if (heroVis.ghosts.length > GHOSTS) heroVis.ghosts.shift();
                    }
                }
                for (let i = heroVis.ghosts.length - 1; i >= 0; i--) {
                    const g = heroVis.ghosts[i];
                    g.life -= lastDt;
                    if (g.life <= 0) { heroVis.ghosts.splice(i, 1); continue; }
                    ctx.save();
                    ctx.globalCompositeOperation = 'lighter';
                    sprite(def.pack, g.anim, g.clock, false, g.x, g.y, 64, -59, g.f < 0, g.life * 1.3, 1.08, 0.94, 0.6, 0);
                    ctx.restore();
                }
            } else {
                heroVis.sx = 1;
                heroVis.sy = 1;
                heroVis.ghosts.length = 0;
            }
            if (p.onGround) h.drawContactShadow(p.x, p.y, 28 * (2 - heroVis.sy));
            if (sim.blessing && sim.blessing.runePower > 0) {
                glow(p.x, p.y - 24, 62, 'rgba(192, 132, 252, 0.7)', 0.8);
                ctx.save();
                ctx.globalCompositeOperation = 'lighter';
                ctx.fillStyle = '#e9d5ff';
                for (let i = 0; i < 3; i++) {
                    const a = view.nowSec * 2.2 + (i * TAU) / 3;
                    ctx.globalAlpha = 0.9;
                    ctx.fillRect(p.x + Math.cos(a) * 20 - 1.5, p.y - 24 + Math.sin(a) * 26 - 1.5, 3, 3);
                }
                ctx.restore();
            }
            const blink = p.invulnTimer > 0 && Math.floor(view.nowSec * 16) % 2 === 0 ? 0.45 : 1;
            const flash = p.hurtTimer > 0 ? clamp(p.hurtTimer * 5, 0, 0.8) : 0;
            sprite(def.pack, anim, clock, hold, drawX, p.y, 64, -59, flipHero, blink, heroVis.sx, heroVis.sy, flash, heroVis.tilt);
            if (sim.heroId === 'knight' && p.attackTimer > 0) drawSwoosh(sim);
            if (sim.heroId === 'ranger' && p.attackTimer > 0) {
                const mx = p.x + p.facing * 22;
                const my = p.y - 24;
                glow(mx, my, 56, 'rgba(126, 232, 250, 0.6)', 0.8);
                ctx.save();
                ctx.globalCompositeOperation = 'lighter';
                ctx.fillStyle = '#ffffff';
                ctx.globalAlpha = 0.9;
                ctx.beginPath();
                ctx.moveTo(mx + p.facing * 12, my);
                ctx.lineTo(mx, my - 3);
                ctx.lineTo(mx - p.facing * 5, my);
                ctx.lineTo(mx, my + 3);
                ctx.closePath();
                ctx.fill();
                ctx.restore();
            }
        }

        function drawProjectiles(sim, view) {
            const list = sim.projectiles;
            if (!list.length) {
                drawParts();
                return;
            }
            for (let i = 0; i < list.length; i++) {
                const pr = list[i];
                if (pr.dead) continue;
                const color = pr.color || (pr.fromEnemy ? '#fb7185' : '#7ee8fa');
                const sp = Math.hypot(pr.vx, pr.vy) || 1;
                const nx = pr.vx / sp;
                const ny = pr.vy / sp;
                const rad = pr.radius ? Math.max(4, Math.min(10, pr.radius * 0.7)) : (pr.fromEnemy ? 6 : 5);
                const len = Math.min(70, 16 + sp * 0.05);
                ctx.save();
                ctx.globalCompositeOperation = 'lighter';
                const g = ctx.createLinearGradient(pr.x, pr.y, pr.x - nx * len, pr.y - ny * len);
                g.addColorStop(0, color);
                g.addColorStop(1, 'rgba(0,0,0,0)');
                ctx.strokeStyle = g;
                ctx.lineCap = 'round';
                ctx.lineWidth = rad * 1.5;
                ctx.globalAlpha = 0.9;
                ctx.beginPath();
                ctx.moveTo(pr.x, pr.y);
                ctx.lineTo(pr.x - nx * len, pr.y - ny * len);
                ctx.stroke();
                ctx.lineWidth = rad * 0.55;
                ctx.strokeStyle = 'rgba(255,255,255,0.85)';
                ctx.beginPath();
                ctx.moveTo(pr.x, pr.y);
                ctx.lineTo(pr.x - nx * len * 0.5, pr.y - ny * len * 0.5);
                ctx.stroke();
                ctx.restore();
                glow(pr.x, pr.y, rad * 5, color.charAt(0) === '#' ? 'rgba(' + parseInt(color.slice(1, 3), 16) + ',' + parseInt(color.slice(3, 5), 16) + ',' + parseInt(color.slice(5, 7), 16) + ',0.6)' : 'rgba(126, 232, 250, 0.6)', 0.75);
                ctx.save();
                ctx.translate(pr.x, pr.y);
                ctx.rotate(Math.atan2(ny, nx));
                ctx.fillStyle = color;
                ctx.beginPath();
                ctx.ellipse(0, 0, rad * 1.25, rad * 0.85, 0, 0, TAU);
                ctx.fill();
                ctx.fillStyle = '#ffffff';
                ctx.beginPath();
                ctx.ellipse(rad * 0.15, 0, rad * 0.7, rad * 0.4, 0, 0, TAU);
                ctx.fill();
                ctx.restore();
                if (!view.reducedMotion && rnd() < 0.35) {
                    addPart('bit', pr.x - nx * 6, pr.y - ny * 6, (rnd() - 0.5) * 40, (rnd() - 0.5) * 40, 0.25, 2, color);
                }
            }
            drawParts();
        }

        function onEvent(ev, sim) {
            if (reduced()) return;
            const d = ev.detail || {};
            const p = sim && sim.player;
            if (ev.type === 'jump') {
                heroVis.sx = 0.82;
                heroVis.sy = 1.22;
                heroVis.vsx = 0;
                heroVis.vsy = 0;
                if (p) for (let i = 0; i < 4; i++) addPart('dust', p.x + (rnd() - 0.5) * 16, p.y, (rnd() - 0.5) * 80, -10 - rnd() * 20, 0.4, 4 + rnd() * 3, '#d8cdb6');
            } else if (ev.type === 'land' && p) {
                const k = clamp(heroVis.fallVy / 620, 0.35, 1);
                heroVis.sx = 1 + 0.15 * k;
                heroVis.sy = 1 - 0.18 * k;
                heroVis.vsx = 0;
                heroVis.vsy = 0;
                for (let i = 0; i < 8; i++) {
                    const dir = i % 2 ? 1 : -1;
                    addPart('dust', p.x + dir * 6, p.y - 2, dir * (50 + rnd() * 110), -8 - rnd() * 22, 0.45 + rnd() * 0.2, 4 + rnd() * 4, '#d8cdb6');
                }
            } else if (ev.type === 'dash') {
                heroVis.ghostClock = 0;
                heroVis.sx = 1.22;
                heroVis.sy = 0.84;
            } else if (ev.type === 'enemy_kill' && typeof d.x === 'number') {
                for (let i = 0; i < 5; i++) addPart('dust', d.x + (rnd() - 0.5) * 24, d.y - 10 - rnd() * 20, (rnd() - 0.5) * 100, -20 - rnd() * 40, 0.5, 6 + rnd() * 6, '#cfd8e4');
            }
        }

        function dispose() {
            parts.length = 0;
            scratch = null;
            scratchCtx = null;
        }

        return { drawEnemies: drawEnemies, drawHero: drawHero, drawProjectiles: drawProjectiles, onEvent: onEvent, dispose: dispose };
    }

    root.RunenpfadActors = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
