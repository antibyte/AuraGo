/* Runenpfad close-up duel renderer: letterboxed arena, big sprites, telegraphs, shots, zones, duel HUD and the iris wipe. Canvas only, read-only on the sim. */
(function (root) {
    'use strict';

    const AR_FALLBACK = { w: 960, h: 540, floorY: 450, left: 80, right: 880 };
    const BACKING = ['#3b2313', '#271e38', '#6e4720', '#2c4a63', '#252c3d'];
    const RUNE_COLOR = { fire: '#fb923c', frost: '#7ee8fa', storm: '#fde047', magnet: '#f0abfc' };
    const RUNE_GLOW = { fire: 'rgba(251, 146, 60, 0.9)', frost: 'rgba(126, 232, 250, 0.9)', storm: 'rgba(253, 224, 71, 0.9)', magnet: 'rgba(240, 171, 252, 0.9)' };
    const AMBER = 'rgba(251, 191, 36, ';
    const HERO_H = 200;
    const IRIS_RIM = '#c4a8ff';

    function Art() { return root.RunenpfadArt || null; }
    function Duel() { return root.RunenpfadDuel || null; }
    function arena() { const D = Duel(); return D && D.ARENA ? D.ARENA : AR_FALLBACK; }
    function clamp(v, lo, hi) { return v < lo ? lo : v > hi ? hi : v; }
    function duelOf(sim) {
        const enc = sim && sim.encounter;
        return enc && enc.kind === 'duel' && enc.duel ? enc.duel : null;
    }

    function create(h) {
        const ctx = h.ctx;
        const seen = new WeakMap(); // zone/shot -> arena time it was first drawn (visual clock only)
        const glows = {};
        const requested = {};
        let requestedFor = null;
        let vig = null;
        let vigKey = '';
        let curS = 1;
        const st = { d: null, heroHp: 0, npcHp: 0, hf: 0, nf: 0, hDisp: 0, nDisp: 0 };

        // ---- loading ----

        function ensureSet(npcId, heroId) {
            const A = Art();
            const art = h.art;
            if (!A || !art || typeof A.loadSheet !== 'function') return;
            if (requestedFor !== art) { requestedFor = art; Object.keys(requested).forEach(function (k) { delete requested[k]; }); }
            ['duel-' + npcId, 'portrait-' + npcId, 'duel-' + heroId, 'fx-duel-spells'].forEach(function (id) {
                if (requested[id]) return;
                requested[id] = true;
                A.loadSheet(art, id);
            });
        }

        // Called every level frame: warms the lazy duel sheets once an NPC is approached or a duel starts.
        function prime(sim) {
            if (!h.art || !sim) return;
            const d = duelOf(sim);
            if (d) { ensureSet(d.npcId, d.heroId || sim.heroId); return; }
            const list = sim.stage && sim.stage.npcs;
            if (!list) return;
            for (let i = 0; i < list.length; i++) {
                const n = list[i];
                if (n.state !== 'gone' && (n.near || n.provoked || n.state === 'alert')) ensureSet(n.npcId, sim.heroId);
            }
        }

        function wantsArena(sim) {
            const d = duelOf(sim);
            const D = Duel();
            return !!(d && D && typeof D.wipe === 'function' && D.wipe(d).side === 'arena');
        }

        // ---- helpers ----

        function layout(vw, vh) {
            const A = arena();
            const s = Math.min(vw / A.w, vh / A.h);
            return { s: s, ox: (vw - A.w * s) / 2, oy: (vh - A.h * s) / 2 };
        }

        function tr(key, fallback) {
            const f = h.t;
            if (typeof f !== 'function') return fallback;
            const w = f(key);
            return w && w !== key ? w : fallback;
        }

        function fontFor(px) {
            const fam = h.font || '"Segoe UI", sans-serif';
            return /Press Start/.test(fam) ? Math.max(7, Math.round(px * 0.6)) + 'px ' + fam : 'bold ' + px + 'px ' + fam;
        }

        function text(str, x, y, px, color, align, alpha) {
            ctx.save();
            if (alpha != null) ctx.globalAlpha = alpha;
            ctx.font = fontFor(px);
            ctx.textAlign = align || 'left';
            ctx.lineJoin = 'round';
            ctx.lineWidth = Math.max(3, px / 5);
            ctx.strokeStyle = '#09060f';
            ctx.strokeText(str, x, y);
            ctx.fillStyle = color || '#fff6dc';
            ctx.fillText(str, x, y);
            ctx.restore();
        }

        // Cached soft round glow sprite (one canvas per colour, never rebuilt per frame).
        function glowCanvas(color) {
            if (glows[color]) return glows[color];
            if (!root.document) return null;
            const c = root.document.createElement('canvas');
            c.width = c.height = 64;
            const g = c.getContext('2d');
            const rg = g.createRadialGradient(32, 32, 0, 32, 32, 32);
            rg.addColorStop(0, color);
            rg.addColorStop(0.4, color.replace(/[\d.]+\)$/, '0.35)'));
            rg.addColorStop(1, 'rgba(0,0,0,0)');
            g.fillStyle = rg;
            g.fillRect(0, 0, 64, 64);
            glows[color] = c;
            return c;
        }

        function glow(x, y, r, color, alpha, add) {
            const c = glowCanvas(color);
            if (!c) return;
            ctx.save();
            ctx.globalAlpha = alpha;
            if (add !== false) ctx.globalCompositeOperation = 'lighter';
            ctx.drawImage(c, x - r, y - r, r * 2, r * 2);
            ctx.restore();
        }

        // Atlas sprite with the clip pivot (feet) at x, y and a screen-aware crisp flag.
        function spr(clip, time, x, y, height, o) {
            const A = Art();
            const art = h.art;
            if (!A || !art || !A.has(art, clip)) return false;
            const opt = o || {};
            const sheet = art.clips[clip].sheet;
            opt.height = height;
            opt.pixel = (height / sheet.refH) * curS >= 0.95;
            return A.draw(ctx, art, clip, time, x, y, opt);
        }

        function hasClip(clip) {
            const A = Art();
            return !!(A && h.art && A.has(h.art, clip));
        }

        function ageOf(obj, d) {
            let t0 = seen.get(obj);
            if (t0 == null) { t0 = d.t; seen.set(obj, t0); }
            return d.t - t0;
        }

        function track(d, dt) {
            if (st.d !== d) {
                st.d = d;
                st.heroHp = d.hero.hp; st.npcHp = d.npc.hp; st.hf = 0; st.nf = 0; st.hDisp = d.hero.hp; st.nDisp = d.npc.hp;
            }
            if (d.hero.hp < st.heroHp) st.hf = 0.16;
            if (d.npc.hp < st.npcHp) st.nf = 0.14;
            st.heroHp = d.hero.hp;
            st.npcHp = d.npc.hp;
            st.hf = Math.max(0, st.hf - dt);
            st.nf = Math.max(0, st.nf - dt);
            st.hDisp = d.hero.hp >= st.hDisp ? d.hero.hp : Math.max(d.hero.hp, st.hDisp - d.hero.maxHp * 0.4 * dt);
            st.nDisp = d.npc.hp >= st.nDisp ? d.npc.hp : Math.max(d.npc.hp, st.nDisp - d.npc.maxHp * 0.3 * dt);
        }

        // ---- backdrop and floor ----

        function paintBackdrop(frame, vw, vh, L, d) {
            const A = arena();
            const atm = h.atmosphere;
            const biome = frame.biome;
            const floorScreen = L.oy + A.floorY * L.s;
            ctx.imageSmoothingEnabled = true;
            if (!atm || !biome) {
                ctx.fillStyle = '#12101c';
                ctx.fillRect(0, 0, vw, vh);
                return;
            }
            const k = 1.15;
            ctx.save();
            ctx.translate(vw / 2, vh);
            ctx.scale(k, k);
            ctx.translate(-vw / 2, -vh);
            atm.paintBackdrop(biome, frame.reducedMotion ? 0 : d.t * 14, vw, vh, vh + (floorScreen - vh) / k, frame.nowSec, frame.reducedMotion);
            ctx.restore();
            ctx.fillStyle = 'rgba(8, 6, 18, 0.26)';
            ctx.fillRect(0, 0, vw, vh);
        }

        function paintFloor(frame, vw, vh, L) {
            const A = arena();
            const FY = A.floorY;
            const TS = 64;
            const x0 = -L.ox / L.s - TS;
            const x1 = (vw - L.ox) / L.s + TS;
            const yEnd = (vh - L.oy) / L.s + TS;
            const Lv = root.RunenpfadLevels;
            const world = Lv && Lv.WORLDS ? Lv.WORLDS[frame.worldIndex] : null;
            ctx.fillStyle = BACKING[frame.worldIndex] || '#2c2438';
            ctx.fillRect(x0, FY, x1 - x0, yEnd - FY);
            if (world && h.sheets && h.sheets['jump-and-run']) {
                ctx.imageSmoothingEnabled = false;
                const fills = [world.groundFill, world.altFill || world.groundFill];
                for (let x = Math.floor(x0 / TS) * TS; x < x1; x += TS) {
                    h.drawCroppedAsset('jump-and-run', world.groundTop, 4, 2, 56, 57, x, FY - 2, TS + 1, TS + 1);
                    let row = 0;
                    for (let y = FY + TS - 2; y < yEnd; y += TS, row++) {
                        h.drawCroppedAsset('jump-and-run', fills[(Math.floor(x / TS) + row) & 1], 4, 4, 56, 55, x, y, TS + 1, TS + 1);
                    }
                }
                ctx.imageSmoothingEnabled = true;
            } else {
                ctx.fillStyle = '#3a3150';
                for (let x = Math.floor(x0 / TS) * TS; x < x1; x += TS) {
                    ctx.fillRect(x + 2, FY + 2, TS - 4, 28);
                    ctx.fillRect(x + 2 + (TS >> 1), FY + 34, TS - 4, 28);
                }
            }
            ctx.fillStyle = 'rgba(8, 6, 18, 0.30)';
            ctx.fillRect(x0, FY + 6, x1 - x0, yEnd - FY);
            ctx.fillStyle = 'rgba(255, 246, 214, 0.16)';
            ctx.fillRect(x0, FY - 2, x1 - x0, 3);
        }

        // ---- characters ----

        function shadow(x, floorY, w, lift) {
            ctx.save();
            ctx.fillStyle = 'rgba(5, 8, 14, ' + (0.38 * clamp(1 - lift / 260, 0.15, 1)).toFixed(3) + ')';
            ctx.beginPath();
            ctx.ellipse(x, floorY + 3, w * clamp(1 - lift / 500, 0.5, 1), 9, 0, 0, Math.PI * 2);
            ctx.fill();
            ctx.restore();
        }

        function smoke(x, y, t) {
            for (let i = 0; i < 6; i++) {
                const a = t * 3 + i * 1.05;
                glow(x + Math.cos(a) * 26, y + Math.sin(a * 1.3) * 34, 40, 'rgba(180, 170, 200, 0.8)', 0.5, false);
            }
        }

        function drawNpc(d, T, t, reduced) {
            const n = d.npc;
            const A = arena();
            const def = root.RunenpfadNpcs && root.RunenpfadNpcs.NPCS ? root.RunenpfadNpcs.NPCS[d.npcId] : null;
            const body = T ? T.height : 220;
            if (n.hidden > 0) { smoke(n.x, n.y - 90, t); return; }
            shadow(n.x, A.floorY, (T ? T.hw : 36) * 1.5, Math.max(0, A.floorY - n.y));
            if (n.phase === 2 && n.hp > 0) {
                const pulse = 0.4 + (reduced ? 0 : Math.sin(t * 7) * 0.12);
                glow(n.x, n.y - body * 0.5, body * 1.0, 'rgba(239, 68, 68, 0.9)', pulse + 0.3);
                glow(n.x, n.y - 6, 120, 'rgba(239, 68, 68, 0.9)', 0.5);
                if (!reduced) {
                    for (let i = 0; i < 5; i++) {
                        const k = (t * 0.7 + i * 0.2) % 1;
                        ctx.fillStyle = 'rgba(254, 202, 120, ' + (0.7 * (1 - k)).toFixed(2) + ')';
                        ctx.fillRect(n.x + Math.sin(i * 2.4 + t) * 40 - 2, n.y - 20 - k * (body * 0.9), 4, 4);
                    }
                }
            }
            let clip = d.phase === 'intro' && hasClip('duel-' + d.npcId + '.taunt') ? 'taunt' : (n.clip || 'idle');
            if (!hasClip('duel-' + d.npcId + '.' + clip)) clip = 'idle';
            const o = { flip: n.facing > 0, flash: st.nf > 0 ? Math.min(0.85, st.nf * 7) : 0 };
            if (n.frozen > 0) { o.tint = '#8fe8ff'; o.tintAmount = 0.5; o.flash = 0; }
            const time = clip === 'defeat' ? (d.phase === 'outro' ? d.phaseT : 0) : d.t;
            const drawn = spr('duel-' + d.npcId + '.' + clip, time, n.x, n.y, body, o);
            if (!drawn) {
                // Level sprite fallback while the duel sheet is still loading (npc sheets face right).
                const lo = { flip: n.facing < 0 };
                if (!spr('npc-' + d.npcId + '.idle', d.t, n.x, n.y, 215, lo)) {
                    ctx.fillStyle = (def && def.color) || '#a78bfa';
                    ctx.fillRect(n.x - 30, n.y - 180, 60, 180);
                }
            }
            if (n.shield > 0) shieldRing(n, body, T, t);
            if (n.frozen > 0) iceShards(n, body, T);
        }

        function shieldRing(n, body, T, t) {
            const cy = n.y - body * 0.5;
            const rx = (T ? T.hw : 36) + 40;
            const ry = body * 0.58;
            ctx.save();
            ctx.globalCompositeOperation = 'lighter';
            ctx.fillStyle = 'rgba(147, 197, 253, 0.12)';
            ctx.beginPath();
            ctx.ellipse(n.x, cy, rx, ry, 0, 0, Math.PI * 2);
            ctx.fill();
            ctx.strokeStyle = 'rgba(191, 219, 254, 0.85)';
            ctx.lineWidth = 3;
            ctx.setLineDash([14, 9]);
            ctx.lineDashOffset = -t * 40;
            ctx.stroke();
            ctx.setLineDash([]);
            ctx.fillStyle = 'rgba(224, 242, 254, 0.9)';
            for (let i = 0; i < 6; i++) {
                const a = t * 1.4 + i * Math.PI / 3;
                const px = n.x + Math.cos(a) * rx;
                const py = cy + Math.sin(a) * ry;
                ctx.beginPath();
                ctx.moveTo(px, py - 6); ctx.lineTo(px + 4, py); ctx.lineTo(px, py + 6); ctx.lineTo(px - 4, py);
                ctx.closePath();
                ctx.fill();
            }
            ctx.restore();
        }

        function iceShards(n, body, T) {
            const hw = T ? T.hw : 36;
            ctx.save();
            ctx.globalCompositeOperation = 'lighter';
            ctx.fillStyle = 'rgba(186, 240, 255, 0.28)';
            ctx.strokeStyle = 'rgba(224, 250, 255, 0.8)';
            ctx.lineWidth = 2;
            [[-1, 0.0, 0.5], [1, 0.0, 0.62], [0, 0.0, 0.85]].forEach(function (c) {
                const bx = n.x + c[0] * hw * 0.85;
                ctx.beginPath();
                ctx.moveTo(bx - 12, n.y);
                ctx.lineTo(bx, n.y - body * c[2]);
                ctx.lineTo(bx + 12, n.y);
                ctx.closePath();
                ctx.fill();
                ctx.stroke();
            });
            ctx.restore();
        }

        function drawHero(d, t, reduced) {
            const hh = d.hero;
            const A = arena();
            shadow(hh.x, A.floorY, 40, Math.max(0, A.floorY - hh.y));
            if (hh.rune && !hh.specialUsed && d.phase === 'fight') glow(hh.x, hh.y - 100, 90, RUNE_GLOW[hh.rune] || RUNE_GLOW.magnet, 0.22 + (reduced ? 0 : Math.sin(t * 4) * 0.06));
            const id = 'duel-' + d.heroId + '.';
            let state = hh.state === 'attack' ? 'combo' : hh.state;
            if (!hasClip(id + state)) state = 'idle';
            let alpha = 1;
            if (hh.invuln > 0 && hh.state !== 'roll' && !reduced && Math.floor(t * 22) % 2 === 0) alpha = 0.35;
            const o = { flip: hh.facing < 0, alpha: alpha, flash: st.hf > 0 ? Math.min(0.85, st.hf * 7) : 0 };
            if (state === 'combo') o.frame = clamp((hh.combo || 1) - 1, 0, 2);
            const time = hh.state === 'defeat' || hh.state === 'victory' ? hh.stateT : d.t;
            const drawn = spr(id + state, time, hh.x, hh.y, HERO_H, o);
            if (!drawn) {
                // Level pack sprite at 2.6x until the duel sheet decodes.
                const Lv = root.RunenpfadLevels;
                const hd = Lv && Lv.HERO_DEFS ? Lv.HERO_DEFS[d.heroId] : null;
                ctx.save();
                ctx.imageSmoothingEnabled = false;
                if (hd) h.drawAnimation(hd.pack, hd.prefix + 'idle', reduced ? 0 : t, hh.x - 83, hh.y - 153, 166, 166, hh.facing < 0, alpha);
                ctx.restore();
            }
            if (hh.state === 'block') {
                ctx.save();
                ctx.strokeStyle = 'rgba(186, 230, 253, 0.85)';
                ctx.lineWidth = 4;
                ctx.globalCompositeOperation = 'lighter';
                ctx.beginPath();
                ctx.arc(hh.x, hh.y - 95, 70, hh.facing > 0 ? -1.0 : Math.PI - 1.0, hh.facing > 0 ? 1.0 : Math.PI + 1.0);
                ctx.stroke();
                ctx.restore();
            }
        }

        // ---- shots ----

        function drawShot(s, d) {
            const ang = Math.atan2(s.vy, Math.abs(s.vx) < 1 ? 1 : Math.abs(s.vx)) * (s.vx < 0 ? -1 : 1);
            const frame = Math.floor(d.t * 14 + s.x * 0.01) % 4;
            const o = { flip: s.vx < 0, rotation: s.vx < 0 ? -ang : ang, composite: 'lighter', frame: frame };
            if (s.kind === 'hex' || s.kind === 'arcane' || s.kind === 'acid') {
                const size = s.kind === 'hex' ? 110 : s.kind === 'arcane' ? 78 : 96;
                const col = s.kind === 'hex' ? 'rgba(167, 139, 250, 0.9)' : s.kind === 'arcane' ? 'rgba(125, 211, 252, 0.9)' : 'rgba(163, 230, 53, 0.9)';
                glow(s.x, s.y, 44, col, 0.5);
                if (!spr('fx-duel-spells.' + s.kind, d.t, s.x, s.y, size, o)) glow(s.x, s.y, 20, col, 0.9);
                return;
            }
            if (s.kind === 'fire') {
                glow(s.x, s.y, 70, 'rgba(251, 146, 60, 0.9)', 0.6);
                if (!spr('fx-fireball.loop', d.t, s.x, s.y, 96, o)) glow(s.x, s.y, 26, 'rgba(255, 200, 80, 0.95)', 1);
                return;
            }
            ctx.save();
            ctx.translate(s.x, s.y);
            if (s.kind === 'dagger') {
                ctx.rotate(Math.atan2(s.vy, s.vx));
                ctx.fillStyle = '#e2e8f0';
                ctx.beginPath();
                ctx.moveTo(-14, -3); ctx.lineTo(16, -4); ctx.lineTo(28, 0); ctx.lineTo(16, 4); ctx.lineTo(-14, 3);
                ctx.closePath();
                ctx.fill();
                ctx.fillStyle = '#7c4a21';
                ctx.fillRect(-26, -3, 12, 6);
                ctx.fillStyle = '#facc15';
                ctx.fillRect(-15, -8, 3, 16);
            } else if (s.kind === 'bomb') {
                ctx.fillStyle = '#1e1b26';
                ctx.beginPath();
                ctx.arc(0, 0, 15, 0, Math.PI * 2);
                ctx.fill();
                ctx.fillStyle = 'rgba(255,255,255,0.25)';
                ctx.fillRect(-8, -9, 5, 5);
                ctx.fillStyle = '#d6b87a';
                ctx.fillRect(-1, -22, 3, 8);
                glow(0, -24, 14, 'rgba(255, 190, 70, 0.95)', 0.5 + 0.4 * Math.sin(d.t * 40));
            } else {
                ctx.rotate(Math.atan2(s.vy, s.vx));
                ctx.globalCompositeOperation = 'lighter';
                ctx.strokeStyle = 'rgba(125, 235, 255, 0.9)';
                ctx.lineWidth = 5;
                ctx.lineCap = 'round';
                ctx.beginPath();
                ctx.moveTo(-42, 0);
                ctx.lineTo(10, 0);
                ctx.stroke();
                ctx.strokeStyle = '#ffffff';
                ctx.lineWidth = 2;
                ctx.beginPath();
                ctx.moveTo(-20, 0);
                ctx.lineTo(12, 0);
                ctx.stroke();
                glow(0, 0, 22, 'rgba(125, 235, 255, 0.9)', 0.6);
            }
            ctx.restore();
        }

        // ---- zones ----

        function warnZone(z, t) {
            const A = arena();
            const pulse = 0.5 + 0.5 * Math.sin(t * 16);
            const x = z.x - z.w / 2;
            ctx.save();
            ctx.fillStyle = AMBER + (0.07 + 0.06 * pulse).toFixed(3) + ')';
            ctx.fillRect(x, A.floorY - z.h, z.w, z.h);
            ctx.fillStyle = AMBER + (0.35 + 0.3 * pulse).toFixed(3) + ')';
            ctx.fillRect(x, A.floorY - 3, z.w, 9);
            ctx.strokeStyle = AMBER + '0.7)';
            ctx.lineWidth = 2;
            ctx.setLineDash([9, 7]);
            ctx.beginPath();
            ctx.moveTo(x, A.floorY); ctx.lineTo(x, A.floorY - z.h);
            ctx.moveTo(x + z.w, A.floorY); ctx.lineTo(x + z.w, A.floorY - z.h);
            ctx.stroke();
            ctx.restore();
        }

        function drawZone(z, d, t) {
            const A = arena();
            const FY = A.floorY;
            const age = ageOf(z, d);
            if (z.kind === 'blast') {
                if (z.warn > 0) {
                    ctx.save();
                    ctx.strokeStyle = AMBER + (0.45 + 0.4 * Math.sin(t * 20)).toFixed(2) + ')';
                    ctx.lineWidth = 3;
                    ctx.setLineDash([10, 8]);
                    ctx.beginPath();
                    ctx.ellipse(z.x, FY, z.w / 2, 24, 0, 0, Math.PI * 2);
                    ctx.stroke();
                    ctx.restore();
                    drawShot({ kind: 'bomb', x: z.x, y: FY - 15, vx: 0, vy: 0 }, d);
                } else {
                    glow(z.x, FY - 50, z.w * 0.7, 'rgba(255, 170, 70, 0.95)', 0.7);
                    spr('fx-explosion.play', clamp(age, 0, 0.5), z.x, FY - 8, z.w * 1.15, { composite: 'source-over' });
                }
                return;
            }
            if (z.warn > 0) {
                if (z.kind === 'marker') {
                    ctx.save();
                    ctx.strokeStyle = AMBER + (0.5 + 0.4 * Math.sin(t * 18)).toFixed(2) + ')';
                    ctx.lineWidth = 4;
                    ctx.beginPath();
                    ctx.ellipse(z.x, FY, z.w * 0.6, 14, 0, 0, Math.PI * 2);
                    ctx.stroke();
                    ctx.restore();
                    warnZone({ x: z.x, w: z.w, h: Math.min(z.h, 120) }, t);
                } else {
                    warnZone(z, t);
                }
                return;
            }
            const fade = clamp(z.t / 0.3, 0, 1);
            if (z.kind === 'cloud') {
                glow(z.x, FY - 14, z.w * 0.55, 'rgba(120, 200, 70, 0.9)', 0.35 * fade, false);
                for (let i = 0; i < 4; i++) {
                    const bx = z.x + (i - 1.5) * z.w * 0.3;
                    const by = FY - 6 - (i & 1) * 22 + Math.sin(t * 3 + i) * 6;
                    spr('fx-poison.loop', t + i * 0.13, bx, by, 190, { alpha: 0.85 * fade, composite: 'source-over' });
                }
            } else if (z.kind === 'vortex') {
                glow(z.x, FY - z.h * 0.4, z.w, 'rgba(230, 190, 110, 0.9)', 0.3 * fade);
                const cy = FY - z.h * 0.5;
                if (!spr('fx-duel-spells.vortex', t, z.x, cy, z.h * 1.1, { alpha: 0.95 * fade, frame: Math.floor(t * 11) % 4 })) {
                    ctx.fillStyle = 'rgba(214, 190, 130, ' + (0.4 * fade).toFixed(2) + ')';
                    ctx.fillRect(z.x - z.w / 2, FY - z.h, z.w, z.h);
                }
                ctx.fillStyle = 'rgba(214, 190, 130, ' + (0.5 * fade).toFixed(2) + ')';
                for (let i = 0; i < 7; i++) {
                    const k = (t * 1.4 + i / 7) % 1;
                    ctx.fillRect(z.x + Math.cos(t * 6 + i * 2.1) * z.w * 0.45 * (0.4 + k * 0.6) - 3, FY - k * z.h, 6, 6);
                }
            } else if (z.kind === 'beam') {
                ctx.save();
                ctx.globalCompositeOperation = 'lighter';
                ctx.globalAlpha = fade;
                const top = FY - z.h;
                ctx.fillStyle = 'rgba(255, 214, 120, 0.22)';
                ctx.fillRect(z.x - z.w / 2, top, z.w, z.h);
                ctx.fillStyle = 'rgba(255, 238, 170, 0.45)';
                ctx.fillRect(z.x - z.w * 0.3, top, z.w * 0.6, z.h);
                ctx.fillStyle = 'rgba(255, 255, 240, 0.95)';
                ctx.fillRect(z.x - z.w * 0.12, top, z.w * 0.24, z.h);
                ctx.restore();
                glow(z.x, FY - 4, z.w * 1.3, 'rgba(255, 220, 130, 0.95)', 0.8 * fade);
            } else if (z.kind === 'steam') {
                ctx.save();
                ctx.globalAlpha = 0.18 * fade;
                ctx.fillStyle = '#e2e8f0';
                ctx.fillRect(z.x - z.w * 0.4, FY - z.h, z.w * 0.8, z.h);
                ctx.restore();
                for (let k = 0; k < 4; k++) {
                    const y = FY - 20 - k * (z.h / 4);
                    if (!spr('fx-steam.sustain', t + k * 0.1, z.x, y, 160, { alpha: 0.9 * fade, composite: 'source-over' })) glow(z.x, y, 44, 'rgba(226, 232, 240, 0.9)', 0.5, false);
                }
            } else if (z.kind === 'puddle') {
                ctx.save();
                ctx.globalAlpha = clamp(z.t / 0.5, 0, 1);
                ctx.fillStyle = 'rgba(110, 200, 60, 0.6)';
                ctx.beginPath();
                ctx.ellipse(z.x, FY + 2, z.w / 2, Math.max(8, z.h * 0.45), 0, 0, Math.PI * 2);
                ctx.fill();
                ctx.strokeStyle = 'rgba(214, 250, 140, 0.8)';
                ctx.lineWidth = 2;
                ctx.stroke();
                ctx.fillStyle = 'rgba(224, 255, 160, 0.8)';
                for (let i = 0; i < 4; i++) {
                    const k = (t * 0.9 + i * 0.27) % 1;
                    ctx.beginPath();
                    ctx.arc(z.x + Math.sin(i * 3.1) * z.w * 0.3, FY - k * 24, 3 * (1 - k) + 1, 0, Math.PI * 2);
                    ctx.fill();
                }
                ctx.restore();
            } else if (z.kind === 'shock') {
                const dir = z.vx < 0 ? -1 : 1;
                ctx.save();
                for (let i = 0; i < 5; i++) {
                    const bx = z.x - dir * i * (z.w * 0.28);
                    const hh = z.h * (1 - i * 0.14);
                    ctx.fillStyle = i === 0 ? '#e3d3a8' : '#a99874';
                    ctx.beginPath();
                    ctx.moveTo(bx - 12, FY);
                    ctx.lineTo(bx + dir * 4, FY - hh);
                    ctx.lineTo(bx + 12, FY);
                    ctx.closePath();
                    ctx.fill();
                }
                ctx.restore();
                spr('fx-dust.loop', t, z.x - dir * 10, FY, 70, { alpha: 0.8, composite: 'source-over', flip: dir < 0 });
            } else if (z.kind === 'marker') {
                glow(z.x, FY - 8, 50, 'rgba(251, 191, 36, 0.9)', 0.3);
            }
        }

        // ---- minions ----

        function drawMinion(m, d, t) {
            const A = arena();
            const FY = A.floorY;
            shadow(m.x, FY, m.kind === 'clone' ? 38 : 26, 0);
            if (m.kind === 'kobold') {
                const o = { flip: m.vx < 0, tint: '#7c4a1e', tintAmount: 0.25 };
                if (!spr('npc-chief.idle', t * 2 + m.x * 0.01, m.x, FY, 84, o)) { ctx.fillStyle = '#a16207'; ctx.fillRect(m.x - 14, FY - 56, 28, 56); }
            } else if (m.kind === 'clone') {
                const toward = d.hero.x < m.x ? -1 : 1;
                const o = { flip: toward > 0, alpha: 0.55 + (Math.sin(t * 6 + m.x) * 0.1), tint: '#c4b5fd', tintAmount: 0.3 };
                if (!spr('duel-seer.idle', d.t, m.x, FY, 220, o)) { ctx.fillStyle = 'rgba(196,181,253,0.4)'; ctx.fillRect(m.x - 26, FY - 190, 52, 190); }
            } else {
                drawClockwork(m, t);
            }
        }

        function drawClockwork(m, t) {
            const A = arena();
            const x = m.x;
            const y = A.floorY;
            const bob = Math.sin(t * 22 + x) * 2;
            const arming = m.fuse >= 0;
            const blink = arming && Math.floor(t * (8 + (0.6 - Math.min(0.6, m.fuse)) * 30)) % 2 === 0;
            ctx.save();
            ctx.fillStyle = '#3b3a4a';
            ctx.fillRect(x - 18, y - 30 + bob, 36, 22);
            ctx.fillStyle = blink ? '#ef4444' : '#94a3b8';
            ctx.fillRect(x - 12, y - 38 + bob, 24, 10);
            ctx.fillStyle = '#d4a85a';
            ctx.beginPath();
            ctx.arc(x - 12, y - 8, 8, 0, Math.PI * 2);
            ctx.arc(x + 12, y - 8, 8, 0, Math.PI * 2);
            ctx.fill();
            ctx.strokeStyle = '#7c5a22';
            ctx.lineWidth = 2;
            for (let i = 0; i < 2; i++) {
                const cx = x + (i ? 12 : -12);
                const a = t * 9 * (i ? -1 : 1);
                ctx.beginPath();
                ctx.moveTo(cx - Math.cos(a) * 8, y - 8 - Math.sin(a) * 8);
                ctx.lineTo(cx + Math.cos(a) * 8, y - 8 + Math.sin(a) * 8);
                ctx.stroke();
            }
            ctx.fillStyle = '#e2e8f0';
            ctx.fillRect(x - 3, y - 50 + bob, 2, 12);
            if (blink) glow(x, y - 28, 46, 'rgba(255, 80, 60, 0.95)', 0.55);
            ctx.restore();
        }

        // ---- telegraph ----

        function drawTell(d, T, t) {
            const n = d.npc;
            const ai = n.ai;
            if (ai.state !== 'tell' || n.hidden > 0) return;
            const x = n.x;
            const y = n.y - (T ? T.height : 220) - 40;
            const prog = ai.tellDur > 0 ? clamp(ai.t / ai.tellDur, 0, 1) : 0;
            const blink = 0.65 + 0.35 * Math.sin(t * 24);
            ctx.save();
            ctx.fillStyle = 'rgba(12, 8, 20, 0.7)';
            ctx.beginPath();
            ctx.arc(x, y, 30, 0, Math.PI * 2);
            ctx.fill();
            ctx.strokeStyle = 'rgba(251, 191, 36, 0.3)';
            ctx.lineWidth = 5;
            ctx.beginPath();
            ctx.arc(x, y, 30, 0, Math.PI * 2);
            ctx.stroke();
            ctx.strokeStyle = '#fbbf24';
            ctx.beginPath();
            ctx.arc(x, y, 30, -Math.PI / 2, -Math.PI / 2 + prog * Math.PI * 2);
            ctx.stroke();
            ctx.restore();
            if (!spr('ui-npc-icons.alert', 0, x, y + 22, 46, { alpha: blink })) text('!', x, y + 10, 32, '#fbbf24', 'center', blink);
        }

        // ---- HUD ----

        function hpBar(x, y, w, hh, frac, trail, color, dir) {
            ctx.fillStyle = '#0d0a14';
            ctx.fillRect(x - 3, y - 3, w + 6, hh + 6);
            ctx.strokeStyle = '#5b4a78';
            ctx.lineWidth = 2;
            ctx.strokeRect(x - 3, y - 3, w + 6, hh + 6);
            const fw = w * clamp(frac, 0, 1);
            const tw = w * clamp(trail, 0, 1);
            const fx = dir < 0 ? x + w - fw : x;
            const tx = dir < 0 ? x + w - tw : x;
            ctx.fillStyle = 'rgba(248, 250, 252, 0.85)';
            ctx.fillRect(tx, y, tw, hh);
            ctx.fillStyle = color;
            ctx.fillRect(fx, y, fw, hh);
            ctx.fillStyle = 'rgba(255, 255, 255, 0.25)';
            ctx.fillRect(fx, y, fw, hh * 0.38);
        }

        function badge(cx, cy, r, ring, paint) {
            ctx.save();
            ctx.fillStyle = '#0d0a14';
            ctx.beginPath();
            ctx.arc(cx, cy, r + 4, 0, Math.PI * 2);
            ctx.fill();
            ctx.save();
            ctx.beginPath();
            ctx.arc(cx, cy, r, 0, Math.PI * 2);
            ctx.clip();
            ctx.fillStyle = '#241c36';
            ctx.fillRect(cx - r, cy - r, r * 2, r * 2);
            paint();
            ctx.restore();
            ctx.strokeStyle = ring;
            ctx.lineWidth = 3;
            ctx.beginPath();
            ctx.arc(cx, cy, r + 2, 0, Math.PI * 2);
            ctx.stroke();
            ctx.restore();
        }

        function drawHud(d, T, t, reduced, top) {
            const A = arena();
            const n = d.npc;
            const hh = d.hero;
            const def = root.RunenpfadNpcs && root.RunenpfadNpcs.NPCS ? root.RunenpfadNpcs.NPCS[d.npcId] : null;
            const R = 30;
            const y = 30 + top;
            // hero (left)
            badge(54, 56 + top, R, '#86efac', function () {
                spr('duel-' + d.heroId + '.idle', 0, 54, 56 + top + 150 * 0.86, 150, {});
            });
            hpBar(96, y + 12, 300, 20, hh.hp / hh.maxHp, st.hDisp / hh.maxHp, '#4ade80', 1);
            text(Math.ceil(hh.hp) + ' / ' + hh.maxHp, 100, y + 60, 15, '#d1fae5', 'left');
            // npc (right)
            const nx = A.w - 54;
            badge(nx, 56 + top, R, def && def.color ? def.color : '#fca5a5', function () {
                const clip = 'portrait-' + d.npcId + (n.phase === 2 ? '.angry' : '.neutral');
                if (!spr(clip, 0, nx, 56 + top + 38, 84, {})) spr('portrait-' + d.npcId + '.neutral', 0, nx, 56 + top + 38, 84, {});
            });
            hpBar(A.w - 396, y + 12, 300, 20, n.hp / n.maxHp, st.nDisp / n.maxHp, '#ef4444', -1);
            const name = def ? tr(def.nameKey, d.npcId.charAt(0).toUpperCase() + d.npcId.slice(1)) : d.npcId;
            text(name, A.w - 100, y + 60, 16, (def && def.color) || '#fecaca', 'right');
            // phase 2 marker at the half-way notch
            const mx = A.w - 396 + 150;
            ctx.save();
            ctx.fillStyle = n.phase === 2 ? '#fecaca' : 'rgba(252, 165, 165, 0.55)';
            ctx.fillRect(mx - 1, y + 8, 2, 28);
            ctx.translate(mx, y + 2);
            ctx.fillStyle = n.phase === 2 ? '#f87171' : '#4b3b66';
            ctx.beginPath();
            ctx.moveTo(0, -6); ctx.lineTo(6, 0); ctx.lineTo(0, 6); ctx.lineTo(-6, 0);
            ctx.closePath();
            ctx.fill();
            ctx.restore();
            if (n.phase === 2 && !reduced) glow(mx, y + 2, 18, 'rgba(248, 113, 113, 0.95)', 0.6);
            // rune special hint
            if (hh.rune && !hh.specialUsed && d.phase === 'fight') {
                const label = '[E] ' + tr('runenpfad.npc.duel.special', 'Rune');
                text(label, 100, y + 82, 14, RUNE_COLOR[hh.rune] || '#fde68a', 'left', 0.7 + (reduced ? 0 : Math.sin(t * 5) * 0.3));
            }
            // controls hint, first seconds
            if (d.phase === 'fight' && d.t > 0.9 && d.t < 6) {
                const hint = tr('runenpfad.npc.duel.controls', '');
                if (hint) text(hint, A.w / 2, A.h - 22, 15, '#e9d5ff', 'center', clamp(Math.min(d.t - 0.9, 6 - d.t), 0, 1));
            }
            // intro title card
            if (d.t < 1.9) {
                const a = clamp((d.t - 0.4) / 0.25, 0, 1) * clamp((1.9 - d.t) / 0.4, 0, 1);
                if (a > 0.01) {
                    text(tr('runenpfad.npc.duel.title', 'Duel!'), A.w / 2, 190, 64, '#fde68a', 'center', a);
                    text(name, A.w / 2, 232, 28, '#fecaca', 'center', a);
                }
            }
            // result banner during the outro pose
            if (d.phase === 'outro' && d.result && d.phaseT > 0.25) {
                const k = clamp((d.phaseT - 0.25) / 0.3, 0, 1);
                const pop = 1 + (1 - k) * 0.6;
                const won = d.result === 'won';
                ctx.save();
                ctx.translate(A.w / 2, 215);
                ctx.scale(pop, pop);
                ctx.fillStyle = 'rgba(10, 6, 18, ' + (0.62 * k).toFixed(2) + ')';
                ctx.fillRect(-300, -62, 600, 96);
                ctx.fillStyle = won ? 'rgba(250, 204, 21, ' + k.toFixed(2) + ')' : 'rgba(248, 113, 113, ' + k.toFixed(2) + ')';
                ctx.fillRect(-300, -62, 600, 3);
                ctx.fillRect(-300, 31, 600, 3);
                text(won ? tr('runenpfad.npc.duel.won', 'Victory!') : tr('runenpfad.npc.duel.lost', 'Defeated'), 0, 0, 58, won ? '#fde68a' : '#fecaca', 'center', k);
                ctx.restore();
            }
        }

        // ---- letterbox + vignette ----

        function paintBars(L, vw, vh) {
            const A = arena();
            const x0 = L.ox;
            const y0 = L.oy;
            const x1 = L.ox + A.w * L.s;
            const y1 = L.oy + A.h * L.s;
            ctx.save();
            ctx.fillStyle = 'rgba(9, 7, 16, 0.88)';
            if (y0 > 0.5) { ctx.fillRect(0, 0, vw, y0); ctx.fillRect(0, y1, vw, vh - y1); }
            if (x0 > 0.5) { ctx.fillRect(0, 0, x0, vh); ctx.fillRect(x1, 0, vw - x1, vh); }
            if (y0 > 0.5 || x0 > 0.5) {
                ctx.strokeStyle = '#4b3b66';
                ctx.lineWidth = 2;
                ctx.strokeRect(x0, y0, x1 - x0, y1 - y0);
                ctx.fillStyle = '#6d56a0';
                const step = 48 * L.s;
                for (let x = x0 + step / 2; x < x1; x += step) {
                    if (y0 > 6) { ctx.fillRect(x - 2, y0 - 7, 4, 4); ctx.fillRect(x - 2, y1 + 3, 4, 4); }
                }
                for (let y = y0 + step / 2; y < y1; y += step) {
                    if (x0 > 6) { ctx.fillRect(x0 - 7, y - 2, 4, 4); ctx.fillRect(x1 + 3, y - 2, 4, 4); }
                }
            }
            ctx.restore();
            const key = vw + 'x' + vh;
            if (!vig || vigKey !== key) {
                if (root.document) {
                    vig = root.document.createElement('canvas');
                    vig.width = 256;
                    vig.height = Math.max(64, Math.round(256 * vh / vw));
                    const g = vig.getContext('2d');
                    const rg = g.createRadialGradient(vig.width / 2, vig.height / 2, vig.height * 0.35, vig.width / 2, vig.height / 2, vig.width * 0.62);
                    rg.addColorStop(0, 'rgba(0,0,0,0)');
                    rg.addColorStop(1, 'rgba(4,2,10,0.55)');
                    g.fillStyle = rg;
                    g.fillRect(0, 0, vig.width, vig.height);
                    vigKey = key;
                }
            }
            if (vig) ctx.drawImage(vig, 0, 0, vw, vh);
        }

        // ---- render ----

        function render(sim, frame) {
            const d = duelOf(sim);
            if (!d || !frame) return;
            const vw = frame.viewW;
            const vh = frame.viewH;
            const L = layout(vw, vh);
            const A = arena();
            const T = root.RunenpfadDuelPatterns && root.RunenpfadDuelPatterns.TABLE ? root.RunenpfadDuelPatterns.TABLE[d.npcId] : null;
            const reduced = !!frame.reducedMotion;
            const t = frame.nowSec;
            curS = L.s;
            ensureSet(d.npcId, d.heroId || sim.heroId);
            track(d, frame.dt || 0.016);
            paintBackdrop(frame, vw, vh, L, d);

            const shk = reduced || !d.fx ? 0 : Math.min(10, d.fx.shake || 0);
            ctx.save();
            ctx.translate(L.ox + Math.sin(t * 61) * shk * L.s, L.oy + Math.cos(t * 53) * shk * 0.6 * L.s);
            ctx.scale(L.s, L.s);
            paintFloor(frame, vw, vh, L);
            const zones = d.zones || [];
            for (let i = 0; i < zones.length; i++) if (zones[i].kind === 'puddle' || zones[i].kind === 'marker') drawZone(zones[i], d, t);
            const minions = d.minions || [];
            for (let i = 0; i < minions.length; i++) drawMinion(minions[i], d, t);
            drawNpc(d, T, t, reduced);
            drawHero(d, t, reduced);
            for (let i = 0; i < zones.length; i++) if (zones[i].kind !== 'puddle' && zones[i].kind !== 'marker') drawZone(zones[i], d, t);
            const shots = d.shots || [];
            for (let i = 0; i < shots.length; i++) drawShot(shots[i], d);
            if (typeof h.drawParticles === 'function' && d.fx) h.drawParticles({ fx: d.fx });
            drawTell(d, T, t);
            ctx.restore();

            ctx.save();
            ctx.translate(L.ox, L.oy);
            ctx.scale(L.s, L.s);
            // The window's own DOM top bar covers the first ~64 css px of the canvas: keep the duel bars below it.
            drawHud(d, T, t, reduced, Math.max(0, ((frame.topInset > 0 ? frame.topInset : 60) + 8 - L.oy) / L.s - 22));
            ctx.restore();
            paintBars(L, vw, vh);
            void A;
        }

        // ---- wipe ----

        function drawWipe(sim, vw, vh, frame) {
            const d = duelOf(sim);
            const D = Duel();
            if (!d || !D) return;
            const w = D.wipe(d);
            if (!(w.cover > 0.001)) return;
            if (frame && frame.reducedMotion) {
                ctx.fillStyle = 'rgba(8, 6, 14, ' + clamp(w.cover, 0, 1).toFixed(3) + ')';
                ctx.fillRect(0, 0, vw, vh);
                return;
            }
            let fx = vw / 2;
            let fy = vh / 2;
            if (w.side === 'arena') {
                const L = layout(vw, vh);
                fx = L.ox + d.npc.x * L.s;
                fy = L.oy + (d.npc.y - 110) * L.s;
            } else if (frame && frame.zoom) {
                const list = (sim.stage && sim.stage.npcs) || [];
                const enc = sim.encounter;
                for (let i = 0; i < list.length; i++) {
                    if (list[i].ref === enc.npcRef) {
                        fx = (list[i].x - frame.sx) * frame.zoom;
                        fy = (list[i].y - 70 - frame.sy) * frame.zoom;
                        break;
                    }
                }
            }
            fx = clamp(fx, 0, vw);
            fy = clamp(fy, 0, vh);
            const maxR = Math.hypot(Math.max(fx, vw - fx), Math.max(fy, vh - fy)) + 8;
            const r = (1 - clamp(w.cover, 0, 1)) * maxR;
            ctx.save();
            ctx.fillStyle = '#07050d';
            ctx.beginPath();
            ctx.rect(0, 0, vw, vh);
            if (r > 0.5) ctx.arc(fx, fy, r, 0, Math.PI * 2, true);
            ctx.fill('evenodd');
            if (r > 2 && r < maxR - 4) {
                ctx.strokeStyle = IRIS_RIM;
                ctx.globalAlpha = 0.85;
                ctx.lineWidth = 3;
                ctx.beginPath();
                ctx.arc(fx, fy, r, 0, Math.PI * 2);
                ctx.stroke();
                ctx.fillStyle = IRIS_RIM;
                const spin = d.t * 3;
                for (let i = 0; i < 16; i++) {
                    const a = spin + (i * Math.PI * 2) / 16;
                    ctx.beginPath();
                    ctx.arc(fx + Math.cos(a) * (r + 9), fy + Math.sin(a) * (r + 9), i % 2 ? 2 : 3.5, 0, Math.PI * 2);
                    ctx.fill();
                }
            }
            ctx.restore();
        }

        function dispose() {
            Object.keys(glows).forEach(function (k) { delete glows[k]; });
            Object.keys(requested).forEach(function (k) { delete requested[k]; });
            vig = null;
            st.d = null;
        }

        return { prime: prime, wantsArena: wantsArena, render: render, drawWipe: drawWipe, dispose: dispose };
    }

    root.RunenpfadDuelView = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
