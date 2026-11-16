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

        // ---- boss telegraphs ----
        function markRing(view, x, y, radius, t, color) {
            const a = 0.35 + 0.45 * pulse(view, t, 18);
            ctx.save();
            ctx.globalAlpha = a;
            ctx.fillStyle = color;
            ctx.beginPath();
            ctx.ellipse(x, y, radius, radius * 0.28, 0, 0, TAU);
            ctx.fill();
            ctx.globalAlpha = 0.95;
            ctx.strokeStyle = '#fff1f2';
            ctx.lineWidth = 3;
            ctx.beginPath();
            ctx.ellipse(x, y, radius, radius * 0.28, 0, 0, TAU);
            ctx.stroke();
            ctx.beginPath();
            ctx.moveTo(x - radius * 0.5, y - 4);
            ctx.lineTo(x + radius * 0.5, y + 4);
            ctx.moveTo(x + radius * 0.5, y - 4);
            ctx.lineTo(x - radius * 0.5, y + 4);
            ctx.stroke();
            ctx.restore();
        }

        function drawTelegraph(sim, e, view) {
            const tg = e.telegraph;
            const prog = Math.max(0, Math.min(1, tg.t / tg.dur));
            const warn = 'rgba(239, 68, 68, 0.55)';
            const ground = sim.stage.groundY;
            ctx.save();
            if (tg.kind === 'charge') {
                const x0 = tg.dir > 0 ? tg.x : tg.x - tg.len;
                const a = 0.18 + 0.3 * pulse(view, tg.t, 18);
                ctx.fillStyle = 'rgba(239, 68, 68, ' + a + ')';
                ctx.fillRect(x0, ground - tg.h, tg.len, tg.h);
                ctx.strokeStyle = 'rgba(254, 226, 226, 0.9)';
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
                ctx.strokeStyle = 'rgba(254, 226, 226, 0.7)';
                ctx.lineWidth = 3;
                ctx.beginPath();
                ctx.moveTo(tg.x, ground - 120);
                ctx.lineTo(tg.x, ground - 26 + prog * 20);
                ctx.stroke();
            } else if (tg.kind === 'dive') {
                const a = 0.12 + 0.28 * pulse(view, tg.t, 18);
                ctx.fillStyle = 'rgba(239, 68, 68, ' + a + ')';
                ctx.fillRect(tg.x - tg.w / 2, 0, tg.w, ground);
                ctx.strokeStyle = 'rgba(254, 226, 226, 0.85)';
                ctx.setLineDash([14, 8]);
                ctx.lineWidth = 3;
                ctx.strokeRect(tg.x - tg.w / 2, 0, tg.w, ground);
                ctx.setLineDash([]);
                markRing(view, tg.x, ground - 3, tg.w * 0.5, tg.t, warn);
            } else if (tg.kind === 'volley' || tg.kind === 'missiles' || tg.kind === 'summon') {
                const color = tg.kind === 'summon' ? 'rgba(192, 132, 252, 0.6)' : (tg.kind === 'missiles' ? 'rgba(249, 115, 22, 0.6)' : warn);
                (tg.marks || []).forEach(function (m) {
                    markRing(view, m.x, ground - 3, tg.kind === 'missiles' ? 46 : 34, tg.t, color);
                });
            } else if (tg.kind === 'ring') {
                const r = 46 + prog * 40;
                ctx.lineWidth = 5;
                ctx.strokeStyle = 'rgba(248, 113, 113, ' + (0.45 + 0.5 * pulse(view, tg.t, 18)) + ')';
                ctx.beginPath();
                ctx.arc(tg.x, tg.y, r, tg.gap + Math.PI / 3, tg.gap + TAU - Math.PI / 3);
                ctx.stroke();
                ctx.strokeStyle = 'rgba(134, 239, 172, 0.95)';
                ctx.beginPath();
                ctx.arc(tg.x, tg.y, r, tg.gap - Math.PI / 3, tg.gap + Math.PI / 3);
                ctx.stroke();
            } else if (tg.kind === 'beam') {
                (tg.xs || [tg.x]).forEach(function (bx) {
                    ctx.fillStyle = 'rgba(244, 114, 182, ' + (0.08 + 0.3 * prog) + ')';
                    ctx.fillRect(bx - tg.w / 2, tg.y0, tg.w, ground - tg.y0);
                    ctx.strokeStyle = 'rgba(253, 242, 248, ' + (0.5 + 0.45 * pulse(view, tg.t, 20)) + ')';
                    ctx.lineWidth = 3;
                    ctx.setLineDash([12, 8]);
                    ctx.strokeRect(bx - tg.w / 2, tg.y0, tg.w, ground - tg.y0);
                    ctx.setLineDash([]);
                    markRing(view, bx, ground - 3, tg.w * 0.7, tg.t, 'rgba(244, 114, 182, 0.6)');
                });
            } else if (tg.kind === 'fire') {
                ctx.fillStyle = 'rgba(249, 115, 22, ' + (0.15 + 0.3 * pulse(view, tg.t, 18)) + ')';
                ctx.fillRect(tg.x, tg.y, tg.w, tg.h);
                ctx.strokeStyle = 'rgba(254, 215, 170, 0.95)';
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
            sim.stage.enemies.forEach(function (e) {
                if (e.def.boss && !e.dead && e.telegraph) drawTelegraph(sim, e, view);
            });
        }

        function drawOverlay(sim, view) {
            (sim.tempHazards || []).forEach(function (th) {
                if (!visible(view, th.x, th.x + th.w)) return;
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
                } else {
                    ctx.globalAlpha = Math.min(1, life * 4) * 0.9;
                    const g = ctx.createLinearGradient(0, th.y + th.h, 0, th.y - 30);
                    g.addColorStop(0, 'rgba(234, 88, 12, 0.85)');
                    g.addColorStop(1, 'rgba(253, 224, 71, 0)');
                    ctx.fillStyle = g;
                    ctx.fillRect(th.x, th.y - 30, th.w, th.h + 30);
                    for (let x = th.x; x < th.x + th.w - 8; x += 44) {
                        const bob = view.reducedMotion ? 0 : Math.sin(view.nowSec * 12 + x) * 3;
                        h.drawStaticAsset('jump-and-run', 'fire', x - 2, th.y - 10 + bob, 48, 52);
                    }
                }
                ctx.restore();
            });
        }

        function collectLights(sim, view, lights) {
            const Z = root.RunenpfadZones;
            if (Z && typeof Z.lights === 'function') Z.lights(sim).forEach(function (l) { lights.push(l); });
            (sim.tempHazards || []).forEach(function (th) {
                if (th.kind === 'fire') {
                    for (let x = th.x + 30; x < th.x + th.w; x += 90) lights.push({ x: x, y: th.y + th.h / 2, radius: 120, intensity: 0.7, flicker: true });
                } else {
                    lights.push({ x: th.x + th.w / 2, y: th.y + th.h * 0.6, radius: 200, intensity: 0.9, flicker: true });
                }
            });
        }

        return { drawWorld: drawWorld, drawOverlay: drawOverlay, collectLights: collectLights };
    }

    root.RunenpfadStageArt = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
