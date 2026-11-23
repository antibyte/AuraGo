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
        // Adjacent conveyor tiles with the same row and speed form one run (cached per platform list).
        const runCache = new WeakMap();
        function conveyorRuns(platforms) {
            let runs = runCache.get(platforms);
            if (runs) return runs;
            runs = [];
            platforms.filter(function (p) { return p.kind === 'conveyor'; }).sort(function (a, b) { return a.y - b.y || a.x - b.x; }).forEach(function (p) {
                const last = runs[runs.length - 1];
                if (last && last.y === p.y && last.speed === p.conveyor && Math.abs(last.x + last.w - p.x) < 1) last.w += p.w;
                else runs.push({ x: p.x, y: p.y, w: p.w, speed: p.conveyor });
            });
            runCache.set(platforms, runs);
            return runs;
        }

        function drawConveyors(sim, view) {
            const pa = root.RunenpfadPropsArt;
            if (!pa) return;
            conveyorRuns(sim.stage.platforms).forEach(function (run) {
                if (visible(view, run.x, run.x + run.w)) pa.drawConveyor(ctx, run, view.nowSec, view.reducedMotion);
            });
        }

        // Faint rails along the travel path of each moving platform (sky rays bob freely and have none).
        function drawTracks(sim, view) {
            const pa = root.RunenpfadPropsArt;
            const rays = sim.stage.skyRays || [];
            if (!pa) return;
            (sim.stage.movingPlatforms || []).forEach(function (mp) {
                if (rays.indexOf(mp) >= 0 || !visible(view, mp.startX, mp.endX + mp.w)) return;
                pa.drawRail(ctx, mp.startX + mp.w / 2, mp.y + 9, mp.endX + mp.w / 2, mp.y + 9, 0.38);
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
                if (root.RunenpfadPropsArt) root.RunenpfadPropsArt.drawChain(ctx, pd.x, pd.y, pd.bx, pd.by, { shackle: true, trim: 18 });
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
                if (!visible(view, r.x - 100, r.x + 100)) return;
                const pts = (r.nodes && r.nodes.length >= 2) ? r.nodes : [{ x: r.x, y: r.top }, { x: r.x, y: r.bottom }];
                ctx.save();
                ctx.lineCap = 'round';
                ctx.lineJoin = 'round';
                ctx.strokeStyle = 'rgba(26, 17, 10, 0.45)';
                ctx.lineWidth = 7.5;
                ctx.beginPath();
                ctx.moveTo(pts[0].x, pts[0].y);
                for (let k = 1; k < pts.length - 1; k++) {
                    ctx.quadraticCurveTo(pts[k].x, pts[k].y, (pts[k].x + pts[k + 1].x) * 0.5, (pts[k].y + pts[k + 1].y) * 0.5);
                }
                ctx.lineTo(pts[pts.length - 1].x, pts[pts.length - 1].y);
                ctx.stroke();
                let strandIdx = 0;
                for (let k = 0; k < pts.length - 1; k++) {
                    const a = pts[k], b = pts[k + 1];
                    const dx = b.x - a.x, dy = b.y - a.y, sl = Math.hypot(dx, dy) || 1;
                    const ux = dx / sl, uy = dy / sl, nx = -uy, ny = ux;
                    const steps = Math.max(1, Math.round(sl / 7));
                    for (let j = 0; j < steps; j++, strandIdx++) {
                        const t0 = j / steps, t1 = (j + 1) / steps;
                        const x0 = a.x + dx * t0, y0 = a.y + dy * t0;
                        const x1 = a.x + dx * t1, y1 = a.y + dy * t1;
                        ctx.strokeStyle = strandIdx % 2 ? '#a77d47' : '#7b5a30';
                        ctx.lineWidth = 5.5;
                        ctx.beginPath();
                        ctx.moveTo(x0 - nx * 2.2, y0 - ny * 2.2);
                        ctx.lineTo(x1 + nx * 2.2, y1 + ny * 2.2);
                        ctx.stroke();
                        ctx.strokeStyle = 'rgba(255, 230, 170, 0.35)';
                        ctx.lineWidth = 1.4;
                        ctx.beginPath();
                        ctx.moveTo(x0 - nx * 0.8, y0 - ny * 0.8);
                        ctx.lineTo(x1 + nx * 2.6, y1 + ny * 2.6);
                        ctx.stroke();
                    }
                }
                ctx.save();
                ctx.translate(r.x, r.top);
                ctx.rotate(Math.atan2(-(pts[1].x - pts[0].x), pts[1].y - pts[0].y) * 0.4);
                ctx.strokeStyle = '#9aa3b5';
                ctx.lineWidth = 3;
                ctx.beginPath();
                ctx.arc(0, -6, 7, 0.1 * Math.PI, 1.9 * Math.PI);
                ctx.stroke();
                ctx.fillStyle = '#5a6275';
                ctx.fillRect(-5, -2, 10, 8);
                ctx.restore();
                const last = pts[pts.length - 1], prev = pts[pts.length - 2];
                ctx.save();
                ctx.translate(last.x, last.y);
                ctx.rotate(Math.atan2(-(last.x - prev.x), last.y - prev.y));
                ctx.fillStyle = '#5e4424';
                ctx.beginPath();
                ctx.ellipse(0, 4, 7, 8, 0, 0, TAU);
                ctx.fill();
                ctx.strokeStyle = '#a77d47';
                ctx.lineWidth = 1.6;
                ctx.beginPath();
                ctx.moveTo(-3, 11); ctx.lineTo(-4.5, 18);
                ctx.moveTo(0, 12); ctx.lineTo(0, 20);
                ctx.moveTo(3, 11); ctx.lineTo(4.5, 18);
                ctx.stroke();
                ctx.restore();
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
            if (!p) return false;
            if (!visible(view, p.x - 70, p.x + 70)) return true;
            const feet = p.y + (sim.stage.tileSize || 48);
            const open = !!sim.hasKey;
            const time = open && !view.reducedMotion ? view.nowSec : 0;
            if (open) h.drawGlow(p.x, feet - 52, 150, 'rgba(126, 232, 250, 0.55)', 0.75);
            const painted = art('prop-portal.loop');
            if (painted) put('prop-portal.loop', time, p.x, feet, 112, open ? 1 : 0.62);
            else h.drawAnimation('assets-animated', 'portal', time, p.x - 32, p.y - 10, 64, 64, false, open ? 1 : 0.62);
            ctx.save();
            ctx.translate(p.x, painted ? feet - 52 : p.y + 20);
            if (open) {
                // Inward-moving lights draw the eye through the open arch.
                for (let i = 0; i < 8; i++) {
                    const depth = (i / 8 + time * 0.28) % 1;
                    const angle = i * 2.4 + depth * 0.7;
                    const radius = painted ? 29 : 18;
                    const x = Math.cos(angle) * radius * (1 - depth);
                    const y = Math.sin(angle) * radius * 1.3 * (1 - depth);
                    ctx.fillStyle = i % 2 ? '#b8fff9' : '#fff3bd';
                    ctx.globalAlpha = Math.sin(depth * Math.PI) * 0.92;
                    ctx.fillRect(x - 1, y - 2, 3, 4);
                }
            } else {
                ctx.lineWidth = 3;
                ctx.strokeStyle = '#ffdc92';
                ctx.beginPath();
                ctx.arc(0, -5, 6, Math.PI, Math.PI * 2);
                ctx.lineTo(6, 0);
                ctx.stroke();
                ctx.fillStyle = '#493529';
                ctx.fillRect(-9, -3, 18, 14);
                ctx.strokeRect(-9, -3, 18, 14);
                ctx.fillStyle = '#ffdc92';
                ctx.fillRect(-2, 1, 4, 5);
            }
            ctx.restore();
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
            if (root.RunenpfadPropsArt) root.RunenpfadPropsArt.drawTelegraphRing(ctx, x, y, radius, radius * 0.28, t, color, view.reducedMotion);
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

        function drawMothSignal(e, view) {
            const m = e.moth;
            const y = e.y - 19 * (e.def.scale || 1);
            h.drawGlow(e.x, y + 2, m.state === 'tell' ? 64 : 34, 'rgba(255,168,59,0.66)', 0.6);
            if (m.state !== 'tell' && m.state !== 'dive') return;
            ctx.save();
            if (m.state === 'tell') {
                const targetY = m.targetY - (e.y - y);
                ctx.setLineDash([6, 8]);
                ctx.lineDashOffset = view.reducedMotion ? 0 : -view.nowSec * 28;
                ctx.beginPath();
                ctx.moveTo(e.x, y);
                ctx.lineTo(m.targetX, targetY);
                ctx.strokeStyle = '#523947';
                ctx.lineWidth = 4;
                ctx.stroke();
                ctx.strokeStyle = '#ffcf72';
                ctx.lineWidth = 2;
                ctx.stroke();
                ctx.setLineDash([]);
                ctx.translate(m.targetX, targetY);
                ctx.rotate(Math.PI / 4);
                ctx.fillStyle = '#553441';
                ctx.fillRect(-5, -5, 10, 10);
                ctx.strokeRect(-5, -5, 10, 10);
            } else if (!view.reducedMotion) {
                const angle = Math.atan2(m.targetY - m.sy, m.targetX - m.sx);
                ctx.translate(e.x, y);
                ctx.rotate(angle);
                const streak = ctx.createLinearGradient(-58, 0, 0, 0);
                streak.addColorStop(0, 'rgba(239,135,64,0)');
                streak.addColorStop(1, 'rgba(255,209,113,0.85)');
                ctx.fillStyle = streak;
                ctx.beginPath();
                ctx.moveTo(-58, 0);
                ctx.lineTo(-3, -5);
                ctx.lineTo(0, 5);
                ctx.fill();
            }
            ctx.restore();
        }

        function drawBellArcs(x, y, dir, size, travelling) {
            ctx.save();
            ctx.translate(x, y);
            ctx.scale(dir, 0.83);
            ctx.lineCap = 'round';
            for (let i = 0; i < 3; i++) {
                const r = size - i * 3;
                ctx.beginPath();
                ctx.arc(-i * 6, 0, r, -0.98, 0.98);
                ctx.strokeStyle = '#4d354a';
                ctx.lineWidth = 4.5;
                ctx.stroke();
                ctx.strokeStyle = i === 0 ? '#fff2b1' : '#e6a75d';
                ctx.lineWidth = 2;
                ctx.stroke();
            }
            if (travelling) {
                ctx.fillStyle = '#ffe7a3';
                ctx.fillRect(-7, 6, 5, 2);
                ctx.fillRect(-16, 7, 3, 2);
            }
            ctx.restore();
        }

        const OB = { height: 0, rotation: 0, alpha: 1, flip: false, composite: '', pixel: true };

        function drawThornRollers(sim, view) {
            for (const r of sim.stage.thornRollers || []) {
                if (!visible(view, r.startX - r.r, r.endX + r.r)) continue;
                const floor = r.y + r.r;
                ctx.save();
                ctx.strokeStyle = 'rgba(31, 28, 45, 0.82)';
                ctx.lineWidth = 10;
                ctx.beginPath(); ctx.moveTo(r.startX - r.r, floor - 2); ctx.lineTo(r.endX + r.r, floor - 2); ctx.stroke();
                ctx.strokeStyle = 'rgba(213, 156, 97, 0.78)';
                ctx.lineWidth = 2;
                ctx.beginPath(); ctx.moveTo(r.startX - r.r, floor - 5); ctx.lineTo(r.endX + r.r, floor - 5); ctx.stroke();
                [r.startX, r.endX].forEach(function (x) {
                    ctx.strokeStyle = '#f4c889'; ctx.lineWidth = 3;
                    ctx.beginPath(); ctx.moveTo(x, floor - 22); ctx.lineTo(x, floor + 2); ctx.stroke();
                    ctx.fillStyle = '#793a58'; ctx.beginPath(); ctx.moveTo(x, floor - 28); ctx.lineTo(x - 7, floor - 20); ctx.lineTo(x + 7, floor - 20); ctx.fill();
                });
                if (!view.reducedMotion) {
                    const trail = ctx.createLinearGradient(r.x - r.dir * 54, 0, r.x - r.dir * 14, 0);
                    trail.addColorStop(0, 'rgba(190, 221, 204, 0)'); trail.addColorStop(1, 'rgba(190, 221, 204, 0.52)');
                    ctx.strokeStyle = trail; ctx.lineWidth = 2; ctx.lineCap = 'round';
                    for (let i = 0; i < 3; i++) {
                        const y = r.y - 12 + i * 12;
                        ctx.beginPath(); ctx.moveTo(r.x - r.dir * 54, y); ctx.lineTo(r.x - r.dir * 16, y - 2); ctx.stroke();
                    }
                }
                if (art('prop-moon-thorns.roller')) {
                    OB.height = 56; OB.rotation = view.reducedMotion ? 0 : r.angle; OB.alpha = 1;
                    Art.draw(ctx, h.art, 'prop-moon-thorns.roller', 0, r.x, r.y, OB);
                } else {
                    ctx.translate(r.x, r.y); ctx.rotate(view.reducedMotion ? 0 : r.angle);
                    const ball = ctx.createRadialGradient(-9, -10, 3, 0, 0, r.r);
                    ball.addColorStop(0, '#65805b'); ball.addColorStop(0.62, '#384632'); ball.addColorStop(1, '#542c48');
                    ctx.fillStyle = ball; ctx.strokeStyle = '#211d31'; ctx.lineWidth = 3;
                    ctx.beginPath(); ctx.arc(0, 0, r.r - 4, 0, Math.PI * 2); ctx.fill(); ctx.stroke();
                    for (let i = 0; i < 10; i++) {
                        const a = i * Math.PI / 5;
                        ctx.fillStyle = i % 2 ? '#d5d1ad' : '#e8b56e';
                        ctx.beginPath(); ctx.moveTo(Math.cos(a) * (r.r - 2), Math.sin(a) * (r.r - 2));
                        ctx.lineTo(Math.cos(a + 0.24) * 12, Math.sin(a + 0.24) * 12);
                        ctx.lineTo(Math.cos(a - 0.24) * 12, Math.sin(a - 0.24) * 12); ctx.closePath(); ctx.fill(); ctx.stroke();
                    }
                    ctx.fillStyle = '#e9899d'; ctx.beginPath(); ctx.arc(-5, -4, 3, 0, Math.PI * 2); ctx.fill();
                    ctx.restore();
                }
                ctx.restore();
            }
        }

        function drawMoonSteps(sim, view) {
            for (const p of sim.stage.moonSteps || []) {
                if (!visible(view, p.x, p.x + p.w)) continue;
                const alpha = p.state === 'hidden' ? 0.18 + 0.12 * p.phase01
                    : p.state === 'rising' ? 0.22 + 0.3 * p.phase01
                    : p.state === 'warning' ? (view.reducedMotion ? 0.76 : 0.68 + 0.15 * Math.sin(view.nowSec * 8)) : 1;
                if (art('prop-moon-thorns.moon')) {
                    const height = p.w * 199 / 241;
                    OB.height = height; OB.rotation = 0; OB.alpha = alpha;
                    Art.draw(ctx, h.art, 'prop-moon-thorns.moon', 0, p.x + p.w / 2, p.y + height * 28 / 199, OB);
                } else {
                    ctx.save(); ctx.globalAlpha = alpha;
                    ctx.fillStyle = p.state === 'warning' ? '#745b76' : '#403d69';
                    ctx.strokeStyle = p.state === 'warning' ? '#ffd27c' : '#b7b2e5'; ctx.lineWidth = 2;
                    ctx.beginPath(); ctx.moveTo(p.x + 7, p.y); ctx.lineTo(p.x + p.w - 7, p.y);
                    ctx.lineTo(p.x + p.w, p.y + 9); ctx.lineTo(p.x + p.w - 7, p.y + 47);
                    ctx.lineTo(p.x + 7, p.y + 47); ctx.lineTo(p.x, p.y + 9); ctx.closePath(); ctx.fill(); ctx.stroke();
                    ctx.fillStyle = p.state === 'warning' ? '#ffd27c' : '#e3dcff';
                    ctx.fillRect(p.x + 3, p.y, p.w - 6, 4);
                    const mx = p.x + p.w / 2; const my = p.y + 25;
                    ctx.beginPath(); ctx.moveTo(mx + 8, my - 11); ctx.quadraticCurveTo(mx - 12, my - 5, mx - 5, my + 9);
                    ctx.quadraticCurveTo(mx - 1, my + 16, mx + 8, my + 12); ctx.quadraticCurveTo(mx - 3, my + 5, mx - 2, my);
                    ctx.quadraticCurveTo(mx - 1, my - 6, mx + 8, my - 11); ctx.fill();
                    ctx.fillStyle = '#ffe4a2'; ctx.fillRect(mx + 18, my - 4, 3, 3); ctx.fillRect(mx - 19, my + 8, 2, 2);
                    ctx.restore();
                }
                ctx.save();
                ctx.globalAlpha = p.state === 'warning' ? 1 : alpha;
                ctx.lineWidth = 3; ctx.lineCap = 'square';
                if (p.state === 'solid') {
                    ctx.strokeStyle = '#f4f2ff'; ctx.beginPath(); ctx.moveTo(p.x + 7, p.y + 1); ctx.lineTo(p.x + p.w - 7, p.y + 1); ctx.stroke();
                    ctx.strokeStyle = '#88edff'; ctx.lineWidth = 1; ctx.beginPath(); ctx.moveTo(p.x + 9, p.y + 3); ctx.lineTo(p.x + p.w - 9, p.y + 3); ctx.stroke();
                } else if (p.state === 'warning') {
                    ctx.strokeStyle = '#ffd27c'; ctx.beginPath(); ctx.moveTo(p.x + 7, p.y + 1); ctx.lineTo(p.x + 61, p.y + 1); ctx.moveTo(p.x + 76, p.y + 1); ctx.lineTo(p.x + p.w - 7, p.y + 1); ctx.stroke();
                    ctx.beginPath(); ctx.moveTo(p.x + 61, p.y - 2); ctx.lineTo(p.x + 68, p.y + 5); ctx.lineTo(p.x + 73, p.y - 1); ctx.stroke();
                } else if (p.state === 'rising') {
                    ctx.strokeStyle = '#a8a0dc'; ctx.setLineDash([8, 9]); ctx.beginPath(); ctx.moveTo(p.x + 8, p.y + 1); ctx.lineTo(p.x + p.w - 8, p.y + 1); ctx.stroke(); ctx.setLineDash([]);
                }
                ctx.restore();
            }
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
            drawThornRollers(sim, view);
            drawMoonSteps(sim, view);
            drawSwitches(sim, view);
            drawChests(sim, view);
            sim.stage.enemies.forEach(function (e) {
                if (e.def.boss && !e.dead && e.telegraph) drawTelegraph(sim, e, view);
                if (e.dead || !visible(view, e.x, e.x)) return;
                if (e.moth) drawMothSignal(e, view);
                if (e.toad && e.toad.state === 'tell') {
                    drawBellArcs(e.x + e.toad.dir * 24, e.y - 16, e.toad.dir, 11 + 4 * pulseK(view, 7), false);
                }
            });
            for (const pr of sim.projectiles || []) {
                if (!pr.dead && pr.kind === 'toad_wave' && visible(view, pr.x, pr.x)) {
                    drawBellArcs(pr.x, pr.y, Math.sign(pr.vx) || 1, pr.radius + 3, true);
                }
            }
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
            for (const e of sim.stage.enemies) {
                if (e.moth && !e.dead && visible(view, e.x, e.x)) {
                    lights.push({ x: e.x, y: e.y - 18, radius: e.moth.state === 'tell' ? 100 : 58, intensity: 0.52, flicker: false, color: '#ffb060' });
                }
            }
            const Z = root.RunenpfadZones;
            if (Z && typeof Z.lights === 'function') {
                // forwarded zone lights: warm by default, cyan for glyph stones and switches, flame orange for dragon fire
                const cool = new Set();
                (sim.stage.glyphStones || []).forEach(function (g) { cool.add(g.x + ',' + (g.y - 30)); });
                (sim.stage.switches || []).forEach(function (sw) { cool.add(sw.x + ',' + sw.y); });
                Z.lights(sim).forEach(function (l) {
                    l.color = cool.has(l.x + ',' + l.y) ? '#7ee8fa' : (l.radius === 230 ? '#ff7a2a' : '#ffb060');
                    lights.push(l);
                });
            }
            (sim.stage.chests || []).forEach(function (c) {
                if (!c.opened) lights.push({ x: c.x, y: c.y - 22, radius: 70, intensity: 0.4, flicker: true, color: '#ffb060' });
            });
            sim.stage.collectibles.forEach(function (it) {
                if (it.type === 'rune_orb' && !it.collected) lights.push({ x: it.x, y: it.y, radius: 90, intensity: 0.55, flicker: false, color: '#7ee8fa' });
            });
            (sim.tempHazards || []).forEach(function (th) {
                if (th.kind === 'dragonfire') return;
                if (th.kind === 'fire') {
                    for (let x = th.x + 30; x < th.x + th.w; x += 90) lights.push({ x: x, y: th.y + th.h / 2, radius: 120, intensity: 0.7, flicker: true, color: '#ff7a2a' });
                } else if (th.kind === 'poison') {
                    lights.push({ x: th.x + th.w / 2, y: th.y + th.h * 0.6, radius: 90, intensity: 0.3, flicker: false, color: '#9be7a8' });
                } else {
                    lights.push({ x: th.x + th.w / 2, y: th.y + th.h * 0.6, radius: 200, intensity: 0.9, flicker: true, color: '#ff6aa8' });
                }
            });
        }

        return {
            drawWorld: drawWorld,
            drawTracks: drawTracks,
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
