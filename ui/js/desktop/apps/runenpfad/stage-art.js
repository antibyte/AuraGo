/* Runenpfad stage art: variety zones (conveyors, pendulums, wind, secret walls, lanterns, ropes) and boss telegraphs. Canvas-facing. */
(function (root) {
    'use strict';

    const TAU = Math.PI * 2;
    const CRACKS = [
        [[24, 0], [18, 22], [28, 40], [20, 60]],
        [[48, 30], [34, 38], [30, 52], [10, 70]],
        [[0, 56], [14, 60], [22, 80], [34, 96]]
    ];

    function create(h) {
        const ctx = h.ctx;

        function visible(view, x0, x1) {
            return x1 >= view.sx - 80 && x0 <= view.sx + view.viewW + 80;
        }

        function pulse(view, t, rate) {
            return 0.5 + 0.5 * Math.sin((view.reducedMotion ? 0.6 : t) * (rate || 16));
        }

        // ---- zones ----
        function drawConveyors(sim, view) {
            sim.stage.platforms.forEach(function (p) {
                if (p.kind !== 'conveyor' || !visible(view, p.x, p.x + p.w)) return;
                ctx.save();
                ctx.beginPath();
                ctx.rect(p.x, p.y - 2, p.w, 14);
                ctx.clip();
                ctx.fillStyle = 'rgba(20, 24, 34, 0.55)';
                ctx.fillRect(p.x, p.y - 2, p.w, 12);
                const dir = p.conveyor >= 0 ? 1 : -1;
                const off = view.reducedMotion ? 0 : (view.nowSec * Math.abs(p.conveyor) * 0.5) % 16;
                ctx.strokeStyle = dir > 0 ? 'rgba(253, 224, 71, 0.9)' : 'rgba(125, 211, 252, 0.9)';
                ctx.lineWidth = 2.5;
                for (let x = p.x - 16 + off * dir + (dir < 0 ? 16 : 0); x < p.x + p.w + 16; x += 16) {
                    ctx.beginPath();
                    ctx.moveTo(x - 4 * dir, p.y);
                    ctx.lineTo(x + 3 * dir, p.y + 4);
                    ctx.lineTo(x - 4 * dir, p.y + 8);
                    ctx.stroke();
                }
                ctx.restore();
            });
        }

        function drawPendulums(sim, view) {
            (sim.stage.pendulums || []).forEach(function (pd) {
                if (!visible(view, pd.x - 440, pd.x + 440)) return;
                ctx.save();
                ctx.fillStyle = '#2a2f3d';
                ctx.fillRect(pd.x - 18, 0, 36, 12);
                ctx.fillStyle = '#4b5366';
                ctx.fillRect(pd.x - 3, 0, 6, pd.y);
                ctx.strokeStyle = '#8b93a7';
                ctx.lineWidth = 4;
                ctx.setLineDash([9, 4]);
                ctx.beginPath();
                ctx.moveTo(pd.x, pd.y);
                ctx.lineTo(pd.bx, pd.by);
                ctx.stroke();
                ctx.setLineDash([]);
                ctx.fillStyle = '#c7ccd9';
                ctx.beginPath();
                ctx.arc(pd.x, pd.y, 7, 0, TAU);
                ctx.fill();
                ctx.translate(pd.bx, pd.by);
                ctx.rotate(view.reducedMotion ? 0 : view.nowSec * 9 * (pd.angle >= 0 ? 1 : -1));
                h.drawGlow(0, 0, 60, 'rgba(251, 113, 133, 0.55)', 0.45);
                if (!view.reducedMotion) {
                    ctx.save();
                    ctx.rotate(-0.3 * (pd.angle >= 0 ? 1 : -1));
                    h.drawStaticAsset('jump-and-run', 'saw', -26, -26, 52, 52, false, 0.28);
                    ctx.restore();
                }
                h.drawStaticAsset('jump-and-run', 'saw', -26, -26, 52, 52);
                ctx.restore();
            });
        }

        function drawWind(sim, view) {
            (sim.stage.windZones || []).forEach(function (wz) {
                if (!visible(view, wz.x, wz.x + wz.w)) return;
                ctx.save();
                const g = ctx.createLinearGradient(0, wz.y, 0, wz.y + wz.h);
                g.addColorStop(0, 'rgba(186, 230, 253, 0.04)');
                g.addColorStop(1, 'rgba(186, 230, 253, 0.2)');
                ctx.fillStyle = g;
                ctx.fillRect(wz.x, wz.y, wz.w, wz.h);
                ctx.strokeStyle = 'rgba(224, 242, 254, 0.7)';
                ctx.lineWidth = 2;
                ctx.lineCap = 'round';
                for (let i = 0; i < 16; i++) {
                    const lx = wz.x + 10 + ((i * 53) % Math.max(20, wz.w - 20));
                    const speed = 150 + (i % 4) * 40;
                    const len = 22 + (i % 3) * 10;
                    const cyc = wz.h + len;
                    const yy = wz.y + wz.h - ((view.reducedMotion ? i * 31 : view.nowSec * speed + i * 67) % cyc);
                    ctx.globalAlpha = 0.25 + 0.5 * ((i % 5) / 5);
                    ctx.beginPath();
                    ctx.moveTo(lx + Math.sin(yy * 0.05 + i) * 4, yy);
                    ctx.lineTo(lx + Math.sin((yy + len) * 0.05 + i) * 4, yy + len);
                    ctx.stroke();
                }
                ctx.restore();
            });
        }

        function drawSecretWalls(sim, view) {
            (sim.stage.secretWalls || []).forEach(function (w) {
                if (!visible(view, w.x, w.x + w.w + 200)) return;
                ctx.save();
                if (!w.broken) {
                    const c = w.chamber;
                    ctx.fillStyle = 'rgba(12, 8, 26, 0.5)';
                    ctx.fillRect(c.x, c.y, c.w, c.h);
                    ctx.fillStyle = 'rgba(253, 224, 71, ' + (0.12 + 0.1 * pulse(view, view.nowSec, 3)) + ')';
                    ctx.fillRect(c.x, c.y, c.w, c.h);
                    const dmg = w.maxHp - w.hp;
                    ctx.strokeStyle = 'rgba(10, 8, 16, 0.85)';
                    ctx.lineWidth = 2 + dmg * 0.6;
                    ctx.lineJoin = 'round';
                    for (let i = 0; i <= Math.min(2, dmg); i++) {
                        ctx.beginPath();
                        CRACKS[i].forEach(function (pt, k) {
                            if (k === 0) ctx.moveTo(w.x + pt[0], w.y + pt[1]);
                            else ctx.lineTo(w.x + pt[0], w.y + pt[1]);
                        });
                        ctx.stroke();
                    }
                    if (w.hitFlash > 0) {
                        ctx.fillStyle = 'rgba(255, 255, 255, ' + Math.min(0.55, w.hitFlash * 3) + ')';
                        ctx.fillRect(w.x, w.y, w.w, w.h);
                    }
                } else {
                    for (let i = 0; i < 9; i++) {
                        const dx = ((i * 17) % 44) + 2;
                        const sz = 5 + (i % 4) * 2;
                        ctx.fillStyle = i % 2 ? '#8a8171' : '#a69d8b';
                        ctx.fillRect(w.x + dx - 12, w.y + w.h - sz, sz + 3, sz);
                    }
                }
                ctx.restore();
            });
        }

        function drawLanterns(sim, view) {
            (sim.stage.lanterns || []).forEach(function (ln) {
                if (!visible(view, ln.x, ln.x)) return;
                ctx.save();
                ctx.fillStyle = '#3b2f24';
                ctx.fillRect(ln.x - 3, ln.y - 26, 6, sim.stage.groundY - ln.y + 26);
                ctx.fillRect(ln.x - 3, ln.y - 28, 22, 5);
                if (ln.lit) {
                    const f = view.reducedMotion ? 1 : 0.85 + 0.15 * Math.sin(view.nowSec * 11 + ln.x);
                    h.drawGlow(ln.x, ln.y, 120 * f, 'rgba(252, 211, 77, 0.65)', 0.8);
                    h.drawStaticAsset('jump-and-run', 'lantern', ln.x - 20, ln.y - 24, 40, 40);
                } else {
                    h.drawStaticAsset('jump-and-run', 'lantern', ln.x - 20, ln.y - 24, 40, 40, false, 0.55);
                    h.drawGlow(ln.x, ln.y, 36, 'rgba(252, 211, 77, 0.4)', 0.25 + 0.15 * pulse(view, view.nowSec, 3));
                }
                ctx.restore();
            });
        }

        function drawRopes(sim, view) {
            const ropes = sim.stage.ropes || [];
            const beams = [];
            ropes.forEach(function (r) { if (r.beam && beams.indexOf(r.beam) < 0) beams.push(r.beam); });
            beams.forEach(function (b) {
                if (!visible(view, b.x, b.x + b.w)) return;
                ctx.fillStyle = '#4a3322';
                ctx.fillRect(b.x - 14, b.y, 14, sim.stage.groundY - b.y);
                ctx.fillRect(b.x + b.w, b.y, 14, sim.stage.groundY - b.y);
                ctx.fillStyle = '#6b4a2f';
                ctx.fillRect(b.x - 20, b.y - 4, b.w + 40, 16);
                ctx.fillStyle = '#8a6440';
                ctx.fillRect(b.x - 20, b.y - 4, b.w + 40, 4);
                ctx.fillStyle = '#3a281a';
                ctx.fillRect(b.x - 20, b.y + 8, b.w + 40, 4);
            });
            ropes.forEach(function (r) {
                if (!visible(view, r.x, r.x)) return;
                const len = r.bottom - r.top;
                const swayAmp = view.reducedMotion ? 0 : 4;
                function swayAt(y) {
                    const k = (y - r.top) / len;
                    return Math.sin(view.nowSec * 1.4 + r.x * 0.013) * swayAmp * k * k;
                }
                ctx.save();
                ctx.lineCap = 'round';
                for (let y = r.top, i = 0; y < r.bottom; y += 7, i++) {
                    const x0 = r.x + swayAt(y);
                    const x1 = r.x + swayAt(y + 7);
                    ctx.strokeStyle = i % 2 ? '#a77d47' : '#7b5a30';
                    ctx.lineWidth = 6;
                    ctx.beginPath();
                    ctx.moveTo(x0 - 2.4, y);
                    ctx.lineTo(x1 + 2.4, y + 7);
                    ctx.stroke();
                    ctx.strokeStyle = 'rgba(255, 230, 170, 0.35)';
                    ctx.lineWidth = 1.5;
                    ctx.beginPath();
                    ctx.moveTo(x0 - 1, y);
                    ctx.lineTo(x1 + 3, y + 7);
                    ctx.stroke();
                }
                // hook at the beam
                ctx.strokeStyle = '#9aa3b5';
                ctx.lineWidth = 3;
                ctx.beginPath();
                ctx.arc(r.x, r.top - 6, 7, 0.1 * Math.PI, 1.9 * Math.PI);
                ctx.stroke();
                ctx.fillStyle = '#5a6275';
                ctx.fillRect(r.x - 5, r.top - 2, 10, 8);
                // knot at the bottom
                ctx.fillStyle = '#5e4424';
                ctx.beginPath();
                ctx.ellipse(r.x + swayAt(r.bottom), r.bottom + 4, 7, 8, 0, 0, TAU);
                ctx.fill();
                ctx.restore();
            });
        }

        // ---- painted props (atlas) with plain fallbacks so gameplay objects never vanish ----
        const Art = root.RunenpfadArt;
        const O = { height: 0, flip: false, alpha: 1, composite: '', rotation: 0, scale: 0 };
        const seen = new WeakMap();
        const RUNE_RGB = { fire: '251,146,60', frost: '125,211,252', storm: '196,181,253', magnet: '253,224,71' };

        function art(clip) {
            return h.art && Art && Art.has(h.art, clip) ? h.art : null;
        }

        function put(clip, time, x, y, height, alpha, comp) {
            O.height = height;
            O.alpha = alpha == null ? 1 : alpha;
            O.composite = comp || '';
            O.flip = false;
            O.rotation = 0;
            return Art.draw(ctx, h.art, clip, time, x, y, O);
        }

        // Seconds since an object entered its "on" state; objects found already on skip their intro clip.
        function stamp(obj, on, nowSec) {
            let s = seen.get(obj);
            if (!s) { s = { on: on, t0: on ? -100 : 0 }; seen.set(obj, s); }
            if (on && !s.on) s.t0 = nowSec;
            s.on = on;
            return nowSec - s.t0;
        }

        function pulseK(view, rate) {
            return view.reducedMotion ? 0.6 : 0.5 + 0.5 * Math.sin(view.nowSec * rate);
        }

        function drawPortal(sim, view) {
            const p = sim.stage.portal;
            if (!p || !art('prop-portal.loop')) return false;
            const feet = p.y + (sim.stage.tileSize || 48);
            const open = !!sim.hasKey;
            if (open) h.drawGlow(p.x, feet - 52, 150, 'rgba(126, 232, 250, 0.55)', 0.75);
            put('prop-portal.loop', view.nowSec, p.x, feet, 112, open ? 1 : 0.5);
            if (open) put('prop-portal.loop', view.nowSec + 0.3, p.x, feet, 112, 0.28, 'lighter');
            return true;
        }

        function drawCheckpoints(sim, view) {
            const list = sim.stage.checkpoints;
            if (!list || !art('prop-checkpoint.active')) return false;
            const T = sim.stage.tileSize || 48;
            for (let i = 0; i < list.length; i++) {
                const cp = list[i];
                if (!visible(view, cp.x, cp.x)) continue;
                const t = stamp(cp, !!cp.activated, view.nowSec);
                const feet = cp.y + T;
                if (!cp.activated) {
                    put('prop-checkpoint.dormant', 0, cp.x, feet, 84, 0.9);
                } else if (t < Art.duration(h.art, 'prop-checkpoint.activate')) {
                    put('prop-checkpoint.activate', t, cp.x, feet, 84);
                    h.drawGlow(cp.x, feet - 40, 120, 'rgba(255, 190, 90, 0.6)', 0.5 + t);
                } else {
                    h.drawGlow(cp.x, feet - 40, 112, 'rgba(255, 190, 90, 0.55)', 0.6 + 0.15 * pulseK(view, 6));
                    put('prop-checkpoint.active', view.nowSec, cp.x, feet, 84);
                }
            }
            return true;
        }

        function drawRuneOrb(item, view, bob) {
            const kind = item.kind || 'fire';
            const rgb = RUNE_RGB[kind] || RUNE_RGB.fire;
            const y = item.y + (bob || 0);
            const k = pulseK(view, 5 + (item.x % 7));
            h.drawGlow(item.x, y, 62 + 10 * k, 'rgba(' + rgb + ',0.65)', 0.55 + 0.25 * k);
            if (!art('powerups.' + kind)) {
                ctx.save();
                ctx.fillStyle = 'rgba(' + rgb + ',0.9)';
                ctx.beginPath();
                ctx.arc(item.x, y, 11, 0, TAU);
                ctx.fill();
                ctx.restore();
                return;
            }
            put('powerups.' + kind, view.nowSec + item.x * 0.01, item.x, y, 50 + 3 * k, 1);
            put('powerups.' + kind, view.nowSec + item.x * 0.01, item.x, y, 54, 0.22 + 0.2 * k, 'lighter');
        }

        function drawChests(sim, view) {
            const list = sim.stage.chests;
            if (!list) return;
            for (let i = 0; i < list.length; i++) {
                const c = list[i];
                if (!visible(view, c.x, c.x)) continue;
                const t = stamp(c, !!c.opened, view.nowSec);
                const rgb = RUNE_RGB[c.kind] || RUNE_RGB.fire;
                if (!c.opened) h.drawGlow(c.x, c.y - 22, 54 + 8 * pulseK(view, 3), 'rgba(' + rgb + ',0.55)', 0.4 + 0.25 * pulseK(view, 3));
                if (art('prop-chest.closed')) {
                    put(c.opened ? 'prop-chest.open' : 'prop-chest.closed', t, c.x, c.y + 2, 58);
                } else {
                    ctx.save();
                    ctx.fillStyle = c.opened ? '#6b4a2f' : '#8a6440';
                    ctx.fillRect(c.x - 20, c.y - 28, 40, 28);
                    ctx.fillStyle = 'rgba(' + rgb + ',0.9)';
                    ctx.fillRect(c.x - 4, c.y - 20, 8, 8);
                    ctx.restore();
                }
            }
        }

        function drawSwitches(sim, view) {
            const list = sim.stage.switches;
            if (!list) return;
            for (let i = 0; i < list.length; i++) {
                const s = list[i];
                if (!visible(view, s.x, s.x)) continue;
                if (s.on) h.drawGlow(s.x, s.y - 4, 74, 'rgba(126, 232, 250, 0.6)', 0.55 + 0.2 * pulseK(view, 6));
                if (art('prop-switch.off')) {
                    put(s.hitFlash > 0 ? 'prop-switch.flash' : (s.on ? 'prop-switch.on' : 'prop-switch.off'), view.nowSec, s.x, s.y + 28, 58);
                } else {
                    ctx.save();
                    ctx.fillStyle = s.on ? '#7ee8fa' : '#5b6a8f';
                    ctx.beginPath();
                    ctx.moveTo(s.x, s.y - 22);
                    ctx.lineTo(s.x + 14, s.y);
                    ctx.lineTo(s.x, s.y + 22);
                    ctx.lineTo(s.x - 14, s.y);
                    ctx.closePath();
                    ctx.fill();
                    ctx.restore();
                }
            }
        }

        // Sealed gate column: glowing rune seal while closed (the open gate simply disappears with its tiles).
        function drawGates(sim, view) {
            const list = sim.stage.gates;
            if (!list) return;
            for (let i = 0; i < list.length; i++) {
                const g = list[i];
                if (g.open || !visible(view, g.x, g.x + g.w)) continue;
                const k = pulseK(view, 3.2);
                const cx = g.x + g.w / 2;
                const cy = g.y + g.h / 2;
                ctx.save();
                ctx.globalCompositeOperation = 'lighter';
                const grad = ctx.createLinearGradient(g.x, 0, g.x + g.w, 0);
                grad.addColorStop(0, 'rgba(126, 232, 250, 0)');
                grad.addColorStop(0.5, 'rgba(126, 232, 250, ' + (0.16 + 0.14 * k) + ')');
                grad.addColorStop(1, 'rgba(126, 232, 250, 0)');
                ctx.fillStyle = grad;
                ctx.fillRect(g.x - 8, g.y, g.w + 16, g.h);
                ctx.strokeStyle = 'rgba(190, 245, 255, ' + (0.5 + 0.4 * k) + ')';
                ctx.lineWidth = 2;
                ctx.beginPath();
                ctx.moveTo(cx, cy - 20);
                ctx.lineTo(cx + 9, cy);
                ctx.lineTo(cx, cy + 20);
                ctx.lineTo(cx - 9, cy);
                ctx.closePath();
                ctx.moveTo(cx, cy - 20);
                ctx.lineTo(cx, cy + 20);
                ctx.stroke();
                ctx.restore();
            }
        }

        function drawGeysers(sim, view) {
            const list = sim.stage.geysers;
            if (!list) return;
            for (let i = 0; i < list.length; i++) {
                const g = list[i];
                const cx = g.x + g.w / 2;
                if (!visible(view, g.x, g.x + g.w)) continue;
                ctx.save();
                ctx.fillStyle = 'rgba(30, 36, 50, 0.7)';
                ctx.beginPath();
                ctx.ellipse(cx, g.y + 1, g.w * 0.42, 6, 0, 0, TAU);
                ctx.fill();
                ctx.restore();
                const warn = g.active && !g.erupting;
                if (art('fx-steam.sustain')) {
                    if (g.erupting) {
                        h.drawGlow(cx, g.y - 40, 90, 'rgba(186, 230, 253, 0.5)', 0.4);
                        put('fx-steam.sustain', view.nowSec, cx, g.y + 2, Math.min(g.h || 300, 320), 0.92);
                    } else {
                        put('fx-steam.warn', view.nowSec + i * 0.37, cx, g.y + 2, warn ? 90 : 52, warn ? 0.75 : 0.3);
                    }
                } else if (g.erupting) {
                    ctx.save();
                    ctx.fillStyle = 'rgba(224, 242, 254, 0.35)';
                    ctx.fillRect(cx - 16, g.y - (g.h || 300), 32, g.h || 300);
                    ctx.restore();
                }
            }
        }

        function drawFlameJets(sim, view) {
            const list = sim.stage.flameJets;
            if (!list) return;
            for (let i = 0; i < list.length; i++) {
                const j = list[i];
                if (!visible(view, j.x, j.x)) continue;
                ctx.save();
                ctx.fillStyle = '#2a2f3d';
                ctx.fillRect(j.x - 15, j.y - 8, 30, 10);
                ctx.fillStyle = '#4b5366';
                ctx.fillRect(j.x - 12, j.y - 11, 24, 4);
                ctx.restore();
                if (j.state === 'warn') {
                    const k = view.reducedMotion ? 0.7 : 0.5 + 0.5 * Math.sin(view.nowSec * 30);
                    h.drawGlow(j.x, j.y - 8, 40 + 20 * k, 'rgba(255, 120, 30, 0.7)', 0.45 + 0.4 * k);
                    if (art('fx-fire-pillar.warn')) put('fx-fire-pillar.warn', view.nowSec, j.x, j.y - 4, 56, 0.75);
                } else if (j.state === 'fire') {
                    h.drawGlow(j.x, j.y - j.h * 0.45, 130, 'rgba(255, 140, 40, 0.65)', 0.7);
                    if (art('fx-fire-pillar.sustain')) {
                        put('fx-fire-pillar.sustain', view.nowSec + i * 0.21, j.x, j.y - 2, j.h * 1.1, 1);
                        put('fx-fire-pillar.sustain', view.nowSec + i * 0.21 + 0.1, j.x, j.y - 2, j.h * 1.1, 0.3, 'lighter');
                    } else {
                        ctx.save();
                        ctx.fillStyle = 'rgba(255, 140, 40, 0.8)';
                        ctx.fillRect(j.x - j.w / 2, j.y - j.h, j.w, j.h);
                        ctx.restore();
                    }
                }
            }
        }

        // ---- boss telegraphs ----
        function markRing(view, x, y, radius, t, color) {
            ctx.save();
            ctx.globalAlpha = 0.62 + 0.3 * pulse(view, t, 18);
            ctx.fillStyle = color;
            ctx.beginPath();
            ctx.ellipse(x, y, radius, radius * 0.28, 0, 0, TAU);
            ctx.fill();
            ctx.globalAlpha = 1;
            ctx.lineJoin = 'round';
            ctx.strokeStyle = '#450a0a';
            ctx.lineWidth = 7;
            markRingPath(x, y, radius);
            ctx.strokeStyle = '#ffd23f';
            ctx.lineWidth = 3;
            markRingPath(x, y, radius);
            ctx.restore();
        }

        function markRingPath(x, y, radius) {
            ctx.beginPath();
            ctx.ellipse(x, y, radius, radius * 0.28, 0, 0, TAU);
            ctx.stroke();
            ctx.beginPath();
            ctx.moveTo(x - radius * 0.5, y - 4);
            ctx.lineTo(x + radius * 0.5, y + 4);
            ctx.moveTo(x + radius * 0.5, y - 4);
            ctx.lineTo(x - radius * 0.5, y + 4);
            ctx.stroke();
        }

        function drawTelegraph(sim, e, view) {
            const tg = e.telegraph;
            const prog = Math.max(0, Math.min(1, tg.t / tg.dur));
            const warn = '#e11d1d';
            const ground = sim.stage.groundY;
            ctx.save();
            if (tg.kind === 'charge') {
                const x0 = tg.dir > 0 ? tg.x : tg.x - tg.len;
                const a = 0.34 + 0.26 * pulse(view, tg.t, 18);
                ctx.fillStyle = 'rgba(225, 29, 29, ' + a + ')';
                ctx.fillRect(x0, ground - tg.h, tg.len, tg.h);
                ctx.strokeStyle = '#ffd23f';
                ctx.lineWidth = 3;
                ctx.strokeRect(x0, ground - tg.h, tg.len, tg.h);
                ctx.lineWidth = 4;
                const shift = view.reducedMotion ? 0 : (tg.t * 160) % 40;
                for (let x = x0 + shift; x < x0 + tg.len - 12; x += 40) {
                    const cx = tg.dir > 0 ? x : x0 + tg.len - (x - x0);
                    ctx.beginPath();
                    ctx.moveTo(cx - 8 * tg.dir, ground - tg.h + 10);
                    ctx.lineTo(cx + 4 * tg.dir, ground - tg.h / 2);
                    ctx.lineTo(cx - 8 * tg.dir, ground - 10);
                    ctx.stroke();
                }
            } else if (tg.kind === 'pound' || tg.kind === 'leap') {
                markRing(view, tg.x, ground - 3, tg.w * (0.5 + 0.5 * (1 - prog)) + 24, tg.t, warn);
                ctx.strokeStyle = '#ffd23f';
                ctx.lineWidth = 3;
                ctx.beginPath();
                ctx.moveTo(tg.x, ground - 120);
                ctx.lineTo(tg.x, ground - 26 + prog * 20);
                ctx.stroke();
            } else if (tg.kind === 'dive') {
                const a = 0.3 + 0.25 * pulse(view, tg.t, 18);
                ctx.fillStyle = 'rgba(225, 29, 29, ' + a + ')';
                ctx.fillRect(tg.x - tg.w / 2, 0, tg.w, ground);
                ctx.strokeStyle = '#ffd23f';
                ctx.setLineDash([14, 8]);
                ctx.lineWidth = 3;
                ctx.strokeRect(tg.x - tg.w / 2, 0, tg.w, ground);
                ctx.setLineDash([]);
                markRing(view, tg.x, ground - 3, tg.w * 0.5, tg.t, warn);
            } else if (tg.kind === 'volley' || tg.kind === 'missiles' || tg.kind === 'summon' || tg.kind === 'fireballs' || tg.kind === 'roar') {
                const color = tg.kind === 'summon' || tg.kind === 'roar' ? '#a855f7' : (tg.kind === 'missiles' ? '#ff7a00' : warn);
                (tg.marks || []).forEach(function (m) {
                    markRing(view, m.x, ground - 3, tg.kind === 'missiles' ? 46 : 34, tg.t, color);
                });
            } else if (tg.kind === 'strafe') {
                // dragon low pass: a pulsing lane over the arena floor with chevrons running the way it will fly
                const fc = tg.variant === 'frost' ? '110, 210, 255' : '255, 110, 30';
                const lx = Math.min(tg.x1, tg.x2);
                const lw = Math.abs(tg.x2 - tg.x1);
                ctx.fillStyle = 'rgba(' + fc + ', ' + (0.14 + 0.16 * pulse(view, tg.t, 16)) + ')';
                ctx.fillRect(lx, ground - tg.h, lw, tg.h);
                ctx.strokeStyle = 'rgba(' + fc + ', 0.85)';
                ctx.lineWidth = 3;
                ctx.strokeRect(lx, ground - tg.h, lw, tg.h);
                const sh = view.reducedMotion ? 0 : (tg.t * 220 * tg.dir) % 48;
                for (let x = lx - 48; x < lx + lw + 48; x += 48) {
                    const cx = x + sh;
                    if (cx < lx || cx > lx + lw) continue;
                    ctx.beginPath();
                    ctx.moveTo(cx - 8 * tg.dir, ground - tg.h + 8);
                    ctx.lineTo(cx + 5 * tg.dir, ground - tg.h / 2);
                    ctx.lineTo(cx - 8 * tg.dir, ground - 8);
                    ctx.stroke();
                }
            } else if (tg.kind === 'ring') {
                const r = 46 + prog * 40;
                ctx.lineWidth = 5;
                ctx.strokeStyle = 'rgba(239, 35, 35, ' + (0.7 + 0.3 * pulse(view, tg.t, 18)) + ')';
                ctx.beginPath();
                ctx.arc(tg.x, tg.y, r, tg.gap + Math.PI / 3, tg.gap + TAU - Math.PI / 3);
                ctx.stroke();
                ctx.strokeStyle = 'rgba(134, 239, 172, 0.95)';
                ctx.beginPath();
                ctx.arc(tg.x, tg.y, r, tg.gap - Math.PI / 3, tg.gap + Math.PI / 3);
                ctx.stroke();
            } else if (tg.kind === 'beam') {
                (tg.xs || [tg.x]).forEach(function (bx) {
                    ctx.fillStyle = 'rgba(236, 40, 140, ' + (0.2 + 0.35 * prog) + ')';
                    ctx.fillRect(bx - tg.w / 2, tg.y0, tg.w, ground - tg.y0);
                    ctx.strokeStyle = 'rgba(255, 224, 80, ' + (0.75 + 0.25 * pulse(view, tg.t, 20)) + ')';
                    ctx.lineWidth = 3;
                    ctx.setLineDash([12, 8]);
                    ctx.strokeRect(bx - tg.w / 2, tg.y0, tg.w, ground - tg.y0);
                    ctx.setLineDash([]);
                    markRing(view, bx, ground - 3, tg.w * 0.7, tg.t, '#ec288c');
                });
            } else if (tg.kind === 'fire') {
                ctx.fillStyle = 'rgba(255, 110, 10, ' + (0.35 + 0.3 * pulse(view, tg.t, 18)) + ')';
                ctx.fillRect(tg.x, tg.y, tg.w, tg.h);
                ctx.strokeStyle = '#ffd23f';
                ctx.lineWidth = 3;
                ctx.setLineDash([10, 6]);
                ctx.strokeRect(tg.x, tg.y, tg.w, tg.h);
                ctx.setLineDash([]);
            }
            // warning glyph above the boss
            const bx = e.x;
            const by = e.y - 48 * e.def.scale - 22;
            ctx.globalAlpha = 0.6 + 0.4 * pulse(view, tg.t, 22);
            ctx.fillStyle = '#fde047';
            ctx.beginPath();
            ctx.moveTo(bx, by - 18);
            ctx.lineTo(bx + 16, by + 10);
            ctx.lineTo(bx - 16, by + 10);
            ctx.closePath();
            ctx.fill();
            ctx.fillStyle = '#7f1d1d';
            ctx.fillRect(bx - 2, by - 8, 4, 10);
            ctx.fillRect(bx - 2, by + 4, 4, 4);
            ctx.restore();
        }

        function drawWorld(sim, view) {
            drawConveyors(sim, view);
            drawWind(sim, view);
            drawSecretWalls(sim, view);
            drawLanterns(sim, view);
            drawRopes(sim, view);
            drawPendulums(sim, view);
            drawGates(sim, view);
            drawGeysers(sim, view);
            drawFlameJets(sim, view);
            drawSwitches(sim, view);
            drawChests(sim, view);
            sim.stage.enemies.forEach(function (e) {
                if (e.def.boss && !e.dead && e.telegraph) drawTelegraph(sim, e, view);
            });
        }

        // ---- grounded hazards: poison cloud and ground fire (the bottom edge always sits on the damage rect's bottom) ----
        const GO = { height: 0, alpha: 1, composite: '', tint: '', tintAmount: 0.5, flip: false, rotation: 0, scale: 0 };

        function putOpt(clip, time, x, y, height, alpha, comp, tint, amount, flip) {
            GO.height = height;
            GO.alpha = alpha;
            GO.composite = comp || '';
            GO.tint = tint || '';
            GO.tintAmount = amount == null ? 0.5 : amount;
            GO.flip = !!flip;
            return Art.draw(ctx, h.art, clip, time, x, y, GO);
        }

        function drawPoison(th, life, view) {
            const bottom = th.y + th.h;
            const cx = th.x + th.w / 2;
            const fade = Math.min(1, life * 3);
            const t = view.reducedMotion ? 0.3 : view.nowSec;
            ctx.save();
            ctx.beginPath();
            ctx.rect(th.x - 10, th.y - 34, th.w + 20, th.h + 34); // nothing paints below the ground line
            ctx.clip();
            const g = ctx.createLinearGradient(0, bottom, 0, th.y - 6);
            g.addColorStop(0, 'rgba(132, 204, 22, ' + 0.5 * fade + ')');
            g.addColorStop(1, 'rgba(34, 120, 40, 0)');
            ctx.fillStyle = g;
            ctx.fillRect(th.x, th.y - 6, th.w, th.h + 6);
            if (art('fx-poison.loop')) {
                const n = Math.max(2, Math.round(th.w / 30));
                for (let i = 0; i < n; i++) {
                    const px = th.x + (i + 0.5) * (th.w / n);
                    const ph = t + i * 0.37;
                    putOpt('fx-poison.loop', ph, px, bottom - th.h * 0.5, th.h * 1.7, 0.85 * fade, 'source-over', '#65a30d', 0.5, i & 1);
                    putOpt('fx-poison.loop', ph + 0.21, px, bottom - th.h * 0.5, th.h * 1.7, 0.4 * fade, 'lighter', '#a3e635', 0.7, !(i & 1));
                }
            } else {
                ctx.fillStyle = 'rgba(101, 163, 13, ' + 0.45 * fade + ')';
                ctx.fillRect(th.x, th.y, th.w, th.h);
            }
            ctx.restore();
            h.drawGlow(cx, bottom - th.h * 0.4, 70, 'rgba(163, 230, 53, 0.55)', 0.5 * fade);
        }

        // Ground fire tiled across [x, x + w) with its bottom exactly on groundY. o: age (s since lit), left (s of life left, for
        // the fade; omit for permanent fires), view, seed (per-fire phase). Falls back to the pack flame sprite (still grounded, no bob).
        function drawGroundFire(x, groundY, w, hgt, o) {
            const view = o.view;
            const n = Math.max(1, Math.round(w / 48));
            const seg = w / n;
            const t = view.reducedMotion ? 0.4 : view.nowSec;
            const age = o.age == null ? 99 : o.age;
            const fadeLen = 0.45;
            const hasFx = !!art('fx-groundfire.loop');
            const ign = hasFx ? Art.duration(h.art, 'fx-groundfire.ignite') : 0;
            const fad = hasFx ? Art.duration(h.art, 'fx-groundfire.fade') : 0;
            const fading = o.left != null && o.left < fadeLen;
            const base = Math.min(1, Math.max(0, o.left == null ? 1 : o.left / fadeLen));
            ctx.save();
            for (let i = 0; i < n; i++) {
                const cx = x + (i + 0.5) * seg;
                const ph = ((o.seed || 0) * 0.013 + i * 0.173) % 1;
                if (hasFx) {
                    const sa = age - i * 0.04;
                    let clip = 'fx-groundfire.loop';
                    let time = t + ph;
                    if (fading) { clip = 'fx-groundfire.fade'; time = (1 - base) * fad; } else if (sa < ign) { clip = 'fx-groundfire.ignite'; time = Math.max(0, sa); }
                    putOpt(clip, time, cx, groundY, hgt, 1, 'source-over');
                    if (!fading && sa >= ign) putOpt(clip, time + 0.11, cx, groundY, hgt, 0.35, 'lighter');
                } else {
                    ctx.globalAlpha = fading ? base : 1;
                    h.drawStaticAsset('jump-and-run', 'fire', cx - seg / 2 - 2, groundY - hgt, seg + 4, hgt);
                    ctx.globalAlpha = 1;
                }
            }
            ctx.restore();
            h.drawGlow(x + w / 2, groundY - hgt * 0.35, Math.max(60, w * 0.6), 'rgba(251, 146, 60, 0.6)', 0.55 * (fading ? base : 1));
            return true;
        }

        // ---- Storm gusts (read-only): faint streaks while warning, strong streaks plus biome debris and a wrap around the hero while gusting ----
        const GUST_TINT = ['226,240,255', '200,235,230', '255,232,190', '235,248,255', '255,214,180'];
        const DEBRIS = [
            { rgb: '134,180,70', rgb2: '190,160,60', kind: 'leaf', lum: false },
            { rgb: '120,255,200', rgb2: '170,200,255', kind: 'dot', lum: true },
            { rgb: '226,190,120', rgb2: '244,214,150', kind: 'grain', lum: false },
            { rgb: '255,255,255', rgb2: '210,235,255', kind: 'dot', lum: false },
            { rgb: '255,150,60', rgb2: '90,80,80', kind: 'dot', lum: true }
        ];

        function gustHash(i, k) {
            const s = Math.sin(i * 127.1 + k * 311.7) * 43758.5453;
            return s - Math.floor(s);
        }

        function drawGusts(sim, view) {
            const w = sim.wind;
            if (!w || (w.state !== 'warn' && w.state !== 'gust')) return;
            const gusting = w.state === 'gust';
            const level = gusting ? w.strength : Math.max(0, Math.min(1, 1 - w.t / (w.dur || 1))) * 0.35;
            if (level <= 0.01) return;
            const reduced = !!view.reducedMotion;
            const wi = Math.max(0, Math.min(4, sim.stage.worldIndex | 0));
            const z = view.zoom || 1;
            const x0 = view.sx;
            const W = view.viewW;
            const y0 = view.sy + 8 / z;
            const H = Math.max(40, view.viewH - 56 / z); // nothing in the bottom 40 canvas px
            const t = reduced ? 1.7 : view.nowSec;
            const n = gusting ? (reduced ? 10 : 34) : (reduced ? 3 : 8);
            ctx.save();
            ctx.beginPath();
            ctx.rect(x0, y0, W, H);
            ctx.clip();
            ctx.lineCap = 'round';
            for (let i = 0; i < n; i++) {
                const len = (gusting ? 90 : 60) + gustHash(i, 1) * (gusting ? 170 : 80);
                const speed = reduced ? 120 : (gusting ? 1100 : 420) + gustHash(i, 2) * (gusting ? 700 : 260);
                const span = W + len * 2;
                const head = x0 + W + len - ((t * speed + gustHash(i, 3) * span) % span);
                const y = y0 + gustHash(i, 4) * H;
                const wob = Math.sin(t * 3 + i * 1.7) * (reduced ? 0 : 3);
                ctx.strokeStyle = 'rgba(' + GUST_TINT[wi] + ',' + (level * (0.3 + gustHash(i, 5) * 0.45)).toFixed(3) + ')';
                ctx.lineWidth = gusting ? 2 + gustHash(i, 6) * 2.5 : 1.5;
                ctx.beginPath();
                ctx.moveTo(head, y);
                ctx.quadraticCurveTo(head + len * 0.5, y + wob + 6, head + len, y + wob);
                ctx.stroke();
            }
            if (gusting) {
                const D = DEBRIS[wi];
                const m = reduced ? 6 : 24;
                if (D.lum) ctx.globalCompositeOperation = 'lighter';
                for (let i = 0; i < m; i++) {
                    const speed = reduced ? 90 : 520 + gustHash(i, 12) * 520;
                    const span = W + 80;
                    const px = x0 + W + 40 - ((t * speed + gustHash(i, 13) * span) % span);
                    const py = y0 + gustHash(i, 14) * H + (reduced ? 0 : Math.sin(t * 4 + i * 2.3) * 14);
                    const a = w.strength * (0.7 + gustHash(i, 15) * 0.3);
                    ctx.fillStyle = 'rgba(' + (i & 1 ? D.rgb : D.rgb2) + ',' + a.toFixed(3) + ')';
                    if (D.kind === 'leaf') {
                        ctx.save();
                        ctx.translate(px, py);
                        ctx.rotate(t * 6 + i);
                        ctx.fillRect(-6, -3, 12, 5);
                        ctx.restore();
                    } else if (D.kind === 'grain') {
                        ctx.fillRect(px, py, 3, 3);
                        ctx.fillRect(px + 8, py + 1, 3, 3);
                    } else {
                        const r = 2.2 + gustHash(i, 16) * 2.2;
                        ctx.beginPath();
                        ctx.arc(px, py, r, 0, TAU);
                        ctx.fill();
                    }
                }
                ctx.globalCompositeOperation = 'source-over';
                // Streaks bend around the hero and trail off to the left.
                const p = sim.player;
                const by = p.y - 22;
                ctx.lineWidth = 2;
                for (let k = 0; k < 5; k++) {
                    const off = (k - 2) * 13;
                    const flow = reduced ? 0 : (t * 7 + k * 0.9) % 6.28;
                    const lean = Math.sin(flow) * (reduced ? 0 : 3);
                    ctx.strokeStyle = 'rgba(' + GUST_TINT[wi] + ',' + (w.strength * (0.5 - Math.abs(k - 2) * 0.08)).toFixed(3) + ')';
                    ctx.beginPath();
                    ctx.moveTo(p.x + 74, by + off * 0.7);
                    ctx.bezierCurveTo(p.x + 30, by + off * 0.7, p.x + 20, by + off + lean, p.x, by + off * 1.25 + lean);
                    ctx.bezierCurveTo(p.x - 22, by + off * 1.3 + lean, p.x - 34, by + off * 0.8, p.x - 90, by + off * 0.9 + lean);
                    ctx.stroke();
                }
            }
            ctx.restore();
        }

        function drawOverlay(sim, view) {
            drawGusts(sim, view);
            (sim.tempHazards || []).forEach(function (th) {
                if (th.kind === 'dragonfire' || !visible(view, th.x, th.x + th.w)) return; // dragon fire is drawn by set-pieces.js
                const life = Math.max(0, Math.min(1, th.t / th.dur));
                ctx.save();
                if (th.kind === 'laser') {
                    const flick = view.reducedMotion ? 1 : 0.8 + 0.2 * Math.sin(view.nowSec * 60);
                    ctx.globalAlpha = Math.min(1, life * 3) * flick;
                    const cx = th.x + th.w / 2;
                    const g = ctx.createLinearGradient(th.x, 0, th.x + th.w, 0);
                    g.addColorStop(0, 'rgba(236, 72, 153, 0)');
                    g.addColorStop(0.3, 'rgba(244, 114, 182, 0.9)');
                    g.addColorStop(0.5, 'rgba(255, 255, 255, 1)');
                    g.addColorStop(0.7, 'rgba(244, 114, 182, 0.9)');
                    g.addColorStop(1, 'rgba(236, 72, 153, 0)');
                    ctx.fillStyle = g;
                    ctx.fillRect(th.x - 6, th.y, th.w + 12, th.h);
                    h.drawGlow(cx, th.y + th.h, 90, 'rgba(244, 114, 182, 0.7)', 0.8);
                } else if (th.kind === 'poison') {
                    drawPoison(th, life, view);
                } else {
                    drawGroundFire(th.x, th.y + th.h, th.w, th.h * 1.35, { age: th.dur - th.t, left: th.t, view: view, seed: th.x });
                }
                ctx.restore();
            });
        }

        function collectLights(sim, view, lights) {
            const Z = root.RunenpfadZones;
            if (Z && typeof Z.lights === 'function') Z.lights(sim).forEach(function (l) { lights.push(l); });
            (sim.stage.chests || []).forEach(function (c) {
                if (!c.opened) lights.push({ x: c.x, y: c.y - 22, radius: 70, intensity: 0.4, flicker: true });
            });
            sim.stage.collectibles.forEach(function (it) {
                if (it.type === 'rune_orb' && !it.collected) lights.push({ x: it.x, y: it.y, radius: 90, intensity: 0.55, flicker: false });
            });
            (sim.tempHazards || []).forEach(function (th) {
                if (th.kind === 'dragonfire') return;
                if (th.kind === 'fire') {
                    for (let x = th.x + 30; x < th.x + th.w; x += 90) lights.push({ x: x, y: th.y + th.h / 2, radius: 120, intensity: 0.7, flicker: true });
                } else if (th.kind === 'poison') {
                    lights.push({ x: th.x + th.w / 2, y: th.y + th.h * 0.6, radius: 90, intensity: 0.3, flicker: false });
                } else {
                    lights.push({ x: th.x + th.w / 2, y: th.y + th.h * 0.6, radius: 200, intensity: 0.9, flicker: true });
                }
            });
        }

        return {
            drawWorld: drawWorld,
            drawOverlay: drawOverlay,
            drawGroundFire: drawGroundFire,
            collectLights: collectLights,
            drawCheckpoints: drawCheckpoints,
            drawPortal: drawPortal,
            drawRuneOrb: drawRuneOrb
        };
    }

    root.RunenpfadStageArt = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
