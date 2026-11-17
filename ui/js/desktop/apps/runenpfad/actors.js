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
        const heroVis = { climbClock: 100, sx: 1, sy: 1, vsx: 0, vsy: 0, fallVy: 0, zapT: 0, wasGround: true, ghosts: [], ghostClock: 0, tilt: 0 };
        const ent = {}; // per-enemy visual state keyed by id
        const parts = [];
        let lastDt = 1 / 60;
        let fireBolt = false;
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
                s = ent[e.id] = { sx: 1, sy: 1, vsx: 0, vsy: 0, prevHurt: 0, prevAtk: 0, burst: false, deadT: 0, stateT: 0, sKey: '', clock: 0, hurtT: 0, prevPhase: 1, enrageT: 0 };
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

        // ---- painted atlas actors: enemies and the five world bosses ----
        const Art = root.RunenpfadArt;
        const BOSS_ART = { boss_boar: 'boss-boar', boss_goblin: 'boss-goblin', boss_orb: 'boss-orb', boss_quad: 'boss-quad', boss_mech: 'boss-mech' };
        const BOSS_GLOW = { boss_boar: '110,231,120', boss_goblin: '250,204,21', boss_orb: '167,139,250', boss_quad: '125,211,252', boss_mech: '251,146,60' };
        const AO = { height: 0, flip: false, alpha: 1, composite: '', rotation: 0, scale: 0, frame: undefined, flash: 0, tint: '', tintAmount: 0.55 };
        const BOSS_DEFEAT_HOLD = 2.2;

        function artIdFor(e) {
            if (!h.art || !Art) return '';
            const id = e.def.art || BOSS_ART[e.defId] || '';
            return id && Art.has(h.art, id + '.idle') ? id : '';
        }

        function aoReset(height, flip) {
            AO.height = height;
            AO.flip = flip;
            AO.alpha = 1;
            AO.composite = '';
            AO.rotation = 0;
            AO.scale = 0;
            AO.frame = undefined;
            AO.flash = 0;
            AO.tint = '';
            AO.tintAmount = 0.55;
        }

        function drawArtEnemy(e, st, view, artId, rm) {
            const boss = !!e.def.boss;
            const scale = e.def.scale || 1;
            const boxH = (boss ? 42 : 38) * scale;
            const target = boxH * (boss ? 1.3 : 1.25) * (e.def.dragon ? 1.15 : 1);
            const sheet = h.art.sheets[artId];
            const center = !!sheet && sheet.def.pivot === 'center';
            const py = center ? e.y - boxH * 0.5 : e.y;
            const bodyY = center ? py : e.y - boxH * 0.5;
            const ai = boss ? e.ai : null;
            const aiState = ai ? ai.state : '';
            const sKey = e.dead ? 'dead' : (boss ? aiState : (e.windup > 0 ? 'windup' : e.attackTimer > 0 ? 'attack' : 'idle'));
            if (sKey !== st.sKey) { st.sKey = sKey; st.stateT = 0; } else st.stateT += lastDt;
            st.clock += lastDt * (1 + Math.min(1.2, Math.abs(e.vx || 0) / 220));
            st.hurtT = e.hurtTimer > 0 ? st.hurtT + lastDt : 0;
            if (boss && e.phase === 2 && st.prevPhase !== 2) st.enrageT = 1.1;
            if (boss) st.prevPhase = e.phase;
            if (st.enrageT > 0) st.enrageT -= lastDt;
            const flip = e.facing < 0;
            let clip = artId + '.idle';
            let time = st.clock;
            let frame;
            let alpha = 1;
            let tint = '';
            if (e.dead) {
                clip = artId + (boss ? '.defeat' : '.death');
                time = st.deadT;
                alpha = boss ? (st.deadT > BOSS_DEFEAT_HOLD ? clamp((BOSS_DEFEAT_HOLD + 0.5 - st.deadT) / 0.5, 0, 1) : 1) : clamp(e.deathTimer / 0.2, 0, 1);
            } else if (boss) {
                if (aiState === 'telegraph') { clip = artId + '.telegraph'; frame = st.stateT < 0.32 ? 0 : 1; }
                else if (ai.pattern === 'roar' && (aiState === 'attack' || aiState === 'recover') && st.stateT < 0.7) { clip = artId + '.roar'; frame = 0; }
                else if (aiState === 'attack') { clip = artId + '.attack'; frame = 0; }
                else if (aiState === 'recover' && st.stateT < 0.4) { clip = artId + '.attack'; frame = 1; }
                else if (st.enrageT > 0) { clip = artId + '.enraged'; frame = 0; }
            } else if (e.hurtTimer > 0) {
                clip = artId + '.hurt';
                time = st.hurtT;
            } else if (e.windup > 0) {
                clip = artId + '.telegraph';
                frame = e.windup > 0.3 ? 0 : 1;
            } else if (e.attackTimer > 0) {
                clip = artId + '.attack';
                time = Math.max(0, 0.45 - e.attackTimer);
            }
            if (!Art.has(h.art, clip)) { clip = artId + '.idle'; frame = undefined; time = st.clock; }
            if (!e.dead && e.phased) alpha = 0.34;
            if (!e.dead && e.blockFlash > 0) tint = '#aee3ff';
            const flash = !e.dead && e.hurtTimer > 0 ? clamp(e.hurtTimer * (boss ? 6 : 7), 0, boss ? 0.6 : 0.85) : 0;
            if (!e.dead && !center && e.onGround) h.drawContactShadow(e.x, e.y, target * 0.62);
            if (boss && !e.dead) drawBossArtGlow(e, bodyY, target, view, st, aiState, rm);
            ctx.save();
            ctx.translate(e.x, py);
            if (!rm) ctx.scale(st.sx, st.sy);
            aoReset(target, flip);
            AO.alpha = alpha;
            AO.frame = frame;
            AO.flash = flash;
            AO.tint = tint;
            Art.draw(ctx, h.art, clip, time, 0, 0, AO);
            if (boss && aiState === 'telegraph' && !e.dead) {
                aoReset(target, flip);
                AO.frame = frame;
                AO.composite = 'lighter';
                AO.tint = '#ff7a30';
                AO.tintAmount = 0.85;
                AO.alpha = 0.14 + 0.26 * (0.5 + 0.5 * Math.sin(view.nowSec * 20));
                Art.draw(ctx, h.art, clip, time, 0, 0, AO);
            }
            ctx.restore();
            if (!boss && !e.dead && e.hp < e.maxHp) {
                const top = (center ? py - target * 0.5 : e.y - target) - 6;
                ctx.fillStyle = 'rgba(0, 0, 0, 0.6)';
                ctx.fillRect(e.x - 19, top - 1, 38, 7);
                ctx.fillStyle = '#fb923c';
                ctx.fillRect(e.x - 18, top, 36 * clamp(e.hp / e.maxHp, 0, 1), 5);
                ctx.fillStyle = 'rgba(255, 255, 255, 0.35)';
                ctx.fillRect(e.x - 18, top, 36 * clamp(e.hp / e.maxHp, 0, 1), 1.5);
            }
        }

        // Subtle painted-boss glow: a soft body halo, a pulsing rune warning while telegraphing and an enraged ember aura in phase two.
        function drawBossArtGlow(e, cy, target, view, st, aiState, rm) {
            const rgb = BOSS_GLOW[e.defId] || '247,201,72';
            const p2 = e.phase === 2;
            const pulse = 0.5 + 0.5 * Math.sin(view.nowSec * (p2 ? 6 : 2.4));
            if (aiState === 'telegraph') {
                glow(e.x, cy, target * (0.75 + 0.2 * pulse), 'rgba(255, 96, 48, 0.7)', 0.4 + 0.3 * pulse);
            } else {
                glow(e.x, cy, target * 0.8, p2 ? 'rgba(251, 113, 133, 0.5)' : 'rgba(' + rgb + ', 0.4)', p2 ? 0.3 + 0.15 * pulse : 0.14 + 0.06 * pulse);
            }
            if (p2 && !rm && rnd() < lastDt * 14) {
                addPart('bit', e.x + (rnd() - 0.5) * target * 0.6, cy + (rnd() - 0.2) * target * 0.5, (rnd() - 0.5) * 36, -40 - rnd() * 60, 0.75, 2 + rnd() * 2.2, rnd() < 0.5 ? '#ff8a5c' : '#ffd0a0');
            }
        }

        function drawEnemies(sim, view) {
            step(view);
            const stage = sim.stage;
            const sx0 = view.sx;
            const dt = lastDt;
            const rm = view.reducedMotion;
            for (let i = 0; i < stage.enemies.length; i++) {
                const e = stage.enemies[i];
                const artId = artIdFor(e);
                const st = stateFor(e);
                if (e.dead) st.deadT += dt; else st.deadT = 0;
                if (e.dead && e.deathTimer <= 0 && !(artId && e.def.boss && st.deadT < BOSS_DEFEAT_HOLD + 0.5)) continue;
                const margin = artId ? 200 : 64;
                if (e.x + margin < sx0 || e.x - margin > sx0 + view.viewW) continue;
                if (!e.dead) st.burst = false;
                if (!rm) {
                    if (e.hurtTimer > 0 && st.prevHurt <= 0 && !e.dead) { st.sx = 1.2; st.sy = 0.82; }
                    if (e.attackTimer > 0 && st.prevAtk <= 0 && !e.dead) { st.sx = 0.88; st.sy = 1.14; }
                    spring(st, dt);
                }
                st.prevHurt = e.hurtTimer;
                st.prevAtk = e.attackTimer;
                if (artId) {
                    drawArtEnemy(e, st, view, artId, rm);
                    continue;
                }
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
            const fireRune = activeRune(sim) && activeRune(sim).kind === 'fire';
            crescent(12, 0, 40, head - 1.25, head, 26, fireRune ? '#ff6a1a' : '#3ccfe8', 0.5 * fade);
            crescent(12, 0, 40, head - 0.8, head, 18, fireRune ? '#ffb55a' : '#9af3ff', 0.8 * fade);
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

        // ---- rune power-up visuals on the hero ----
        function activeRune(sim) {
            const r = sim.player.rune;
            return r && r.timeLeft > 0 ? r : null;
        }

        // Blink during the last two seconds so the end of a rune is readable.
        function runeFade(r, nowSec) {
            return r.timeLeft < 2 ? (Math.floor(nowSec * 9) % 2 ? 0.3 : 0.85) : 1;
        }

        function drawRuneAura(sim, view, p, r) {
            const kind = r.kind;
            const k = runeFade(r, view.nowSec);
            const cy = p.y - 26;
            const t = view.nowSec;
            const rm = view.reducedMotion;
            if (kind === 'fire') {
                glow(p.x, cy, 54 + (rm ? 0 : Math.sin(t * 9) * 4), 'rgba(255, 140, 50, 0.7)', 0.5 * k);
                if (!rm && rnd() < lastDt * 22 * k) addPart('bit', p.x + (rnd() - 0.5) * 18, p.y - 6 - rnd() * 24, (rnd() - 0.5) * 24 - p.vx * 0.05, -30 - rnd() * 50, 0.6, 2 + rnd() * 2, rnd() < 0.5 ? '#ff9a4a' : '#ffd37a');
            } else if (kind === 'frost') {
                if (r.charges > 0) {
                    ctx.save();
                    const g = ctx.createRadialGradient(p.x, cy, 8, p.x, cy, 34);
                    g.addColorStop(0, 'rgba(170, 230, 255, 0.03)');
                    g.addColorStop(0.8, 'rgba(150, 220, 255, 0.2)');
                    g.addColorStop(1, 'rgba(210, 245, 255, 0.45)');
                    ctx.globalAlpha = k;
                    ctx.fillStyle = g;
                    ctx.beginPath();
                    ctx.arc(p.x, cy, 34, 0, TAU);
                    ctx.fill();
                    ctx.strokeStyle = 'rgba(220, 247, 255, 0.75)';
                    ctx.lineWidth = 1.6;
                    ctx.stroke();
                    ctx.globalCompositeOperation = 'lighter';
                    ctx.strokeStyle = 'rgba(255, 255, 255, 0.5)';
                    ctx.beginPath();
                    ctx.arc(p.x, cy, 28, 3.7, 4.6);
                    ctx.stroke();
                    ctx.restore();
                }
            } else if (kind === 'storm') {
                ctx.save();
                ctx.globalCompositeOperation = 'lighter';
                ctx.strokeStyle = '#d9ccff';
                ctx.lineWidth = 1.4;
                ctx.globalAlpha = k;
                const slot = Math.floor(t * 18);
                for (let i = 0; i < 3; i++) {
                    const a = hash(slot * 7 + i) * TAU;
                    const r0 = 18 + hash(slot * 13 + i) * 8;
                    let x = p.x + Math.cos(a) * r0;
                    let y = cy + Math.sin(a) * r0 * 1.25;
                    ctx.beginPath();
                    ctx.moveTo(x, y);
                    for (let s = 1; s <= 3; s++) {
                        x += Math.cos(a + (hash(slot + s * 5 + i) - 0.5) * 2.4) * 6;
                        y += Math.sin(a + (hash(slot + s * 3 + i) - 0.5) * 2.4) * 6;
                        ctx.lineTo(x, y);
                    }
                    ctx.stroke();
                }
                ctx.restore();
                glow(p.x, cy, 44, 'rgba(180, 160, 255, 0.6)', 0.3 * k);
                if (heroVis.zapT > 0) {
                    heroVis.zapT -= lastDt;
                    ctx.save();
                    ctx.globalCompositeOperation = 'lighter';
                    ctx.strokeStyle = '#ffffff';
                    ctx.lineWidth = 2.4;
                    ctx.globalAlpha = Math.max(0, heroVis.zapT / 0.16);
                    ctx.beginPath();
                    let zx = p.x;
                    let zy = p.y + 4;
                    ctx.moveTo(zx, zy);
                    for (let s = 1; s <= 6; s++) {
                        zx += (hash(slot + s) - 0.5) * 22;
                        zy -= 16;
                        ctx.lineTo(zx, zy);
                    }
                    ctx.stroke();
                    ctx.restore();
                }
            } else if (kind === 'magnet') {
                const ph = ((t % 1.4) / 1.4);
                ctx.save();
                ctx.globalCompositeOperation = 'lighter';
                ctx.strokeStyle = '#c084fc';
                ctx.lineWidth = 2.2 * (1 - ph) + 0.6;
                ctx.globalAlpha = (1 - ph) * 0.55 * k;
                ctx.beginPath();
                ctx.arc(p.x, cy, 16 + ph * 74, 0, TAU);
                ctx.stroke();
                ctx.restore();
                glow(p.x, cy, 40, 'rgba(192, 132, 252, 0.6)', 0.28 * k);
            }
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
            const rune = activeRune(sim);
            if (rune) drawRuneAura(sim, view, p, rune);
            const blink = p.invulnTimer > 0 && Math.floor(view.nowSec * 16) % 2 === 0 ? 0.45 : 1;
            const flash = p.hurtTimer > 0 ? clamp(p.hurtTimer * 5, 0, 0.8) : 0;
            if (p.swing) {
                // Hanging on a rope: the jump pose pivots around the hands (feet are 40 px below) and leans with the rope.
                sprite(def.pack, clipFor(def.pack, def.prefix, 'jump'), p.animTime, false, p.x, p.y - 40, 64, -19, flipHero, blink, 1, 1, flash, -(p.swing.angle || 0) * 0.8);
            } else sprite(def.pack, anim, clock, hold, drawX, p.y, 64, -59, flipHero, blink, heroVis.sx, heroVis.sy, flash, heroVis.tilt);
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

        // ---- projectile kinds: each type gets its own silhouette ----
        function rgba(color, a) {
            if (color && color.charAt(0) === '#' && color.length >= 7) return 'rgba(' + parseInt(color.slice(1, 3), 16) + ',' + parseInt(color.slice(3, 5), 16) + ',' + parseInt(color.slice(5, 7), 16) + ',' + a + ')';
            return 'rgba(126, 232, 250, ' + a + ')';
        }

        function drawSpear(pr, ang, view) {
            ctx.save();
            ctx.translate(pr.x, pr.y);
            ctx.rotate(ang);
            glow(0, 0, 26, rgba(pr.color || '#fde68a', 0.6), 0.5);
            ctx.fillStyle = '#6b4a2f';
            ctx.fillRect(-24, -1.6, 34, 3.2);
            ctx.fillStyle = '#a77d47';
            ctx.fillRect(-24, -1.6, 34, 1.1);
            ctx.fillStyle = '#e8efe9';
            ctx.beginPath();
            ctx.moveTo(-26, 0);
            ctx.lineTo(-19, -4.5);
            ctx.lineTo(-17, 0);
            ctx.lineTo(-19, 4.5);
            ctx.closePath();
            ctx.fill();
            ctx.fillStyle = '#7ee8fa';
            ctx.beginPath();
            ctx.moveTo(18, 0);
            ctx.lineTo(9, -5.5);
            ctx.lineTo(5, 0);
            ctx.lineTo(9, 5.5);
            ctx.closePath();
            ctx.fill();
            ctx.fillStyle = '#ffffff';
            ctx.beginPath();
            ctx.moveTo(15, 0);
            ctx.lineTo(9, -2.4);
            ctx.lineTo(7, 0);
            ctx.closePath();
            ctx.fill();
            ctx.restore();
        }

        function drawMissile(pr, ang, view) {
            const fl = view.reducedMotion ? 1 : 0.7 + 0.3 * Math.sin(view.nowSec * 50 + pr.x);
            ctx.save();
            ctx.translate(pr.x, pr.y);
            ctx.rotate(ang);
            ctx.globalCompositeOperation = 'lighter';
            const g = ctx.createLinearGradient(-6, 0, -34 * fl, 0);
            g.addColorStop(0, 'rgba(255, 210, 120, 0.95)');
            g.addColorStop(0.5, 'rgba(251, 120, 40, 0.6)');
            g.addColorStop(1, 'rgba(251, 120, 40, 0)');
            ctx.fillStyle = g;
            ctx.beginPath();
            ctx.moveTo(-5, -4);
            ctx.lineTo(-34 * fl, 0);
            ctx.lineTo(-5, 4);
            ctx.closePath();
            ctx.fill();
            ctx.globalCompositeOperation = 'source-over';
            ctx.fillStyle = '#7b8498';
            ctx.fillRect(-9, -4, 20, 8);
            ctx.fillStyle = '#c9d0dd';
            ctx.fillRect(-9, -4, 20, 2.4);
            ctx.fillStyle = '#d9483b';
            ctx.beginPath();
            ctx.moveTo(11, -4);
            ctx.lineTo(19, 0);
            ctx.lineTo(11, 4);
            ctx.closePath();
            ctx.fill();
            ctx.fillStyle = '#4a5263';
            ctx.fillRect(-12, -7, 6, 4);
            ctx.fillRect(-12, 3, 6, 4);
            ctx.restore();
            if (!view.reducedMotion && rnd() < 0.5) addPart('dust', pr.x - Math.cos(ang) * 14, pr.y - Math.sin(ang) * 14, (rnd() - 0.5) * 20, (rnd() - 0.5) * 20, 0.5, 3.5 + rnd() * 3, '#aaa69c');
        }

        function drawOrbBullet(pr, view) {
            const r = 8 + (pr.radius ? pr.radius * 0.3 : 0);
            const col = pr.color || '#a78bfa';
            glow(pr.x, pr.y, r * 3.6, rgba(col, 0.7), 0.85);
            ctx.save();
            ctx.translate(pr.x, pr.y);
            const g = ctx.createRadialGradient(0, 0, 0, 0, 0, r);
            g.addColorStop(0, '#ffffff');
            g.addColorStop(0.45, col);
            g.addColorStop(1, rgba(col, 0.15));
            ctx.fillStyle = g;
            ctx.beginPath();
            ctx.arc(0, 0, r, 0, TAU);
            ctx.fill();
            ctx.globalCompositeOperation = 'lighter';
            ctx.strokeStyle = rgba(col, 0.95);
            ctx.lineWidth = 1.8;
            ctx.rotate(view.reducedMotion ? 0 : view.nowSec * 4 + pr.x * 0.02);
            ctx.beginPath();
            ctx.ellipse(0, 0, r * 1.45, r * 0.62, 0, 0, TAU);
            ctx.stroke();
            ctx.rotate(1.5);
            ctx.beginPath();
            ctx.ellipse(0, 0, r * 1.45, r * 0.62, 0, 0, TAU);
            ctx.stroke();
            ctx.restore();
        }

        function drawShockCrest(pr, view) {
            const dir = pr.vx >= 0 ? 1 : -1;
            const k = view.reducedMotion ? 1 : 0.9 + 0.1 * Math.sin(view.nowSec * 34 + pr.x * 0.1);
            const hgt = 30 * k;
            ctx.save();
            ctx.translate(pr.x, pr.y + 12);
            ctx.scale(dir, 1);
            ctx.globalCompositeOperation = 'lighter';
            const g = ctx.createLinearGradient(0, 0, 0, -hgt);
            g.addColorStop(0, 'rgba(255, 190, 90, 0.95)');
            g.addColorStop(1, 'rgba(255, 240, 200, 0.15)');
            ctx.fillStyle = g;
            ctx.beginPath();
            ctx.moveTo(-34, 0);
            ctx.quadraticCurveTo(-18, -hgt * 0.25, -4, -hgt * 0.7);
            ctx.quadraticCurveTo(10, -hgt * 1.05, 22, -hgt * 0.35);
            ctx.quadraticCurveTo(16, -hgt * 0.12, 26, 0);
            ctx.closePath();
            ctx.fill();
            ctx.strokeStyle = 'rgba(255, 255, 255, 0.8)';
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.moveTo(-14, -hgt * 0.35);
            ctx.quadraticCurveTo(-2, -hgt * 0.8, 14, -hgt * 0.55);
            ctx.stroke();
            ctx.restore();
            if (!view.reducedMotion && rnd() < 0.5) addPart('dust', pr.x - dir * 24, pr.y + 10, -dir * 30, -20 - rnd() * 20, 0.45, 4 + rnd() * 4, '#e8d8b8');
        }

        function drawSpore(pr, view) {
            const w = view.reducedMotion ? 0 : Math.sin(view.nowSec * 14 + pr.x * 0.05) * 0.14;
            glow(pr.x, pr.y, 30, 'rgba(134, 239, 172, 0.6)', 0.55);
            ctx.save();
            ctx.translate(pr.x, pr.y);
            ctx.scale(1 + w, 1 - w);
            ctx.fillStyle = '#3f9a52';
            ctx.beginPath();
            ctx.arc(0, 0, 9, 0, TAU);
            ctx.fill();
            ctx.fillStyle = '#86efac';
            ctx.beginPath();
            ctx.arc(-1.5, -2, 6.4, 0, TAU);
            ctx.fill();
            ctx.fillStyle = '#d9ffd9';
            ctx.beginPath();
            ctx.arc(-3.4, -4, 2.2, 0, TAU);
            ctx.fill();
            ctx.fillStyle = 'rgba(40, 90, 50, 0.75)';
            ctx.beginPath();
            ctx.arc(3, 2, 1.8, 0, TAU);
            ctx.arc(-2, 4, 1.3, 0, TAU);
            ctx.fill();
            ctx.restore();
            if (!view.reducedMotion && rnd() < 0.3) addPart('bit', pr.x, pr.y, (rnd() - 0.5) * 30, -10 - rnd() * 24, 0.5, 2, '#86efac');
        }

        function drawCannonBolt(pr, ang, view) {
            glow(pr.x, pr.y, 34, 'rgba(251, 146, 60, 0.6)', 0.6);
            ctx.save();
            ctx.translate(pr.x, pr.y);
            ctx.rotate(ang);
            const g = ctx.createRadialGradient(-3, -3, 1, 0, 0, 10);
            g.addColorStop(0, '#7b8498');
            g.addColorStop(1, '#1d2230');
            ctx.fillStyle = g;
            ctx.beginPath();
            ctx.arc(0, 0, 8.5, 0, TAU);
            ctx.fill();
            ctx.strokeStyle = 'rgba(251, 146, 60, 0.9)';
            ctx.lineWidth = 1.6;
            ctx.beginPath();
            ctx.arc(0, 0, 8.5, -0.9, 0.9);
            ctx.stroke();
            ctx.restore();
            if (!view.reducedMotion && rnd() < 0.5) addPart('dust', pr.x - Math.cos(ang) * 9, pr.y, (rnd() - 0.5) * 20, -10, 0.4, 3 + rnd() * 3, '#8d8576');
        }

        // Ranger bolt: slim rune arrow with trailing glyphs.
        function drawRangerBolt(pr, ang, view) {
            const col = fireBolt ? '#ff9a4a' : (pr.color || '#7ee8fa');
            glow(pr.x, pr.y, 30, rgba(col, 0.6), 0.7);
            ctx.save();
            ctx.translate(pr.x, pr.y);
            ctx.rotate(ang);
            ctx.globalCompositeOperation = 'lighter';
            const g = ctx.createLinearGradient(0, 0, -44, 0);
            g.addColorStop(0, rgba(col, 0.95));
            g.addColorStop(1, rgba(col, 0));
            ctx.fillStyle = g;
            ctx.beginPath();
            ctx.moveTo(10, 0);
            ctx.lineTo(-44, -2.4);
            ctx.lineTo(-44, 2.4);
            ctx.closePath();
            ctx.fill();
            ctx.fillStyle = '#ffffff';
            ctx.beginPath();
            ctx.moveTo(14, 0);
            ctx.lineTo(3, -3.2);
            ctx.lineTo(-6, 0);
            ctx.lineTo(3, 3.2);
            ctx.closePath();
            ctx.fill();
            ctx.fillStyle = rgba(col, 0.9);
            for (let i = 1; i <= 3; i++) {
                const gx = -i * 14 - ((view.nowSec * 40) % 14);
                const s = 3.2 - i * 0.6;
                ctx.globalAlpha = 0.9 - i * 0.22;
                ctx.beginPath();
                ctx.moveTo(gx, -s);
                ctx.lineTo(gx + s, 0);
                ctx.lineTo(gx, s);
                ctx.lineTo(gx - s, 0);
                ctx.closePath();
                ctx.fill();
            }
            ctx.restore();
        }

        // Returns true when the projectile type has its own look.
        function drawKind(pr, view) {
            const ang = Math.atan2(pr.vy, pr.vx || 0.001);
            if (pr.kind === 'spear') drawSpear(pr, ang, view);
            else if (pr.kind === 'missile') drawMissile(pr, ang, view);
            else if (pr.kind === 'orb') drawOrbBullet(pr, view);
            else if (pr.kind === 'shock') drawShockCrest(pr, view);
            else if (pr.kind === 'spore') drawSpore(pr, view);
            else if (pr.kind === 'fireball') return true; // painted by set-pieces.js
            else if (!pr.kind && pr.fromEnemy && pr.color === '#fb923c') drawCannonBolt(pr, ang, view);
            else if (!pr.kind && !pr.fromEnemy) drawRangerBolt(pr, ang, view);
            else return false;
            return true;
        }

        function drawProjectiles(sim, view) {
            const list = sim.projectiles;
            const pr0 = activeRune(sim);
            fireBolt = !!(pr0 && pr0.kind === 'fire');
            if (!list.length) {
                drawParts();
                return;
            }
            for (let i = 0; i < list.length; i++) {
                const pr = list[i];
                if (pr.dead) continue;
                if (drawKind(pr, view)) continue;
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
                glow(pr.x, pr.y, rad * 5, rgba(color, 0.6), 0.75);
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
                if (p && !p.onGround && p.rune && p.rune.kind === 'storm') heroVis.zapT = 0.16;
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
            } else if (ev.type === 'rune_shield_break' && p) {
                for (let i = 0; i < 12; i++) addPart('bit', p.x + (rnd() - 0.5) * 20, p.y - 26 + (rnd() - 0.5) * 30, (rnd() - 0.5) * 240, (rnd() - 0.5) * 240 - 40, 0.5, 2 + rnd() * 2.5, '#bfeaff');
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
