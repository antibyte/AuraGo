/* Runenpfad set pieces: dragon runs (far / warn / strafe / leave), swinging ropes, the three puzzles, fireballs and dragon fire.
 * Canvas-facing and read-only: every state array is optional and skipped quietly when absent. The atlas is optional too. */
(function (root) {
    'use strict';

    const TAU = Math.PI * 2;
    const Art = root.RunenpfadArt;
    const GLYPH_RGB = ['251,191,36', '196,181,253', '56,189,248', '74,222,128'];
    const KIND_RGB = { ember: '255,120,40', frost: '120,220,255' };
    const MAGIC = { sx: 8, sy: 35, sw: 48, sh: 24 };
    const MAX_EMBERS = 90;
    const MAX_PINGS = 28;
    const DRAGON_PX = 335; // content width of a dragon sheet frame at scale 1

    function clamp(v, lo, hi) {
        return v < lo ? lo : v > hi ? hi : v;
    }

    function create(h) {
        if (!h || !h.ctx) return null;
        const ctx = h.ctx;
        const O = { height: 0, scale: 0, flip: false, alpha: 1, composite: '', rotation: 0, tint: '', tintAmount: 0.55, frame: undefined, flash: 0 };
        const seen = new WeakMap();
        const runVis = Object.create(null);
        const embers = [];
        const pings = [];
        const trail = [];
        let seed = 11;
        let stepped = -1;
        let heatNow = 0;
        let prevFb = [];
        let nextFb = [];
        const impacts = [];

        function rnd() {
            seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
            return seed / 4294967296;
        }

        function vis(view, x0, x1) {
            return x1 >= view.sx - 200 && x0 <= view.sx + view.viewW + 200;
        }

        function has(clip) {
            return !!(h.art && Art && Art.has(h.art, clip));
        }

        function pulse(view, rate) {
            return view.reducedMotion ? 0.6 : 0.5 + 0.5 * Math.sin(view.nowSec * rate);
        }

        // Draws a clip with the pivot at (x, y); o: height | scale, flip, alpha, comp, rot, frame, tint, tintAmount, flash.
        function put(clip, time, x, y, o) {
            if (!has(clip)) return false;
            O.height = o.height || 0;
            O.scale = o.scale || 0;
            O.flip = !!o.flip;
            O.alpha = o.alpha == null ? 1 : o.alpha;
            O.composite = o.comp || '';
            O.rotation = o.rot || 0;
            O.tint = o.tint || '';
            O.tintAmount = o.amount == null ? 0.55 : o.amount;
            O.frame = o.frame;
            O.flash = o.flash || 0;
            return Art.draw(ctx, h.art, clip, time, x, y, O);
        }

        // Seconds since an object entered its "on" state (objects found already on skip their intro).
        function stamp(obj, on, nowSec) {
            let s = seen.get(obj);
            if (!s) { s = { on: on, t0: on ? -100 : nowSec }; seen.set(obj, s); }
            if (on !== s.on) { s.t0 = nowSec; s.on = on; }
            return nowSec - s.t0;
        }

        function ping(x, y, rgb, size) {
            if (pings.length >= MAX_PINGS) pings.shift();
            pings.push({ x: x, y: y, rgb: rgb, size: size || 46, age: 0 });
        }

        function ember(x, y, vx, vy, life, size, rgb) {
            if (embers.length >= MAX_EMBERS) embers.shift();
            embers.push({ x: x, y: y, vx: vx, vy: vy, life: life, max: life, size: size, rgb: rgb });
        }

        function glow(x, y, r, rgb, a) {
            h.drawGlow(x, y, r, 'rgba(' + rgb + ', 0.6)', a);
        }

        // ---- procedural glyph (also the fallback when the stones sheet is absent) ----
        function glyphShape(g, x, y, r, color) {
            ctx.save();
            ctx.strokeStyle = color;
            ctx.fillStyle = color;
            ctx.lineWidth = Math.max(1.6, r * 0.16);
            ctx.lineCap = 'round';
            ctx.beginPath();
            if (g === 0) {
                ctx.arc(x, y, r * 0.36, 0, TAU);
                for (let i = 0; i < 8; i++) {
                    const a = (i * TAU) / 8;
                    ctx.moveTo(x + Math.cos(a) * r * 0.55, y + Math.sin(a) * r * 0.55);
                    ctx.lineTo(x + Math.cos(a) * r * 0.95, y + Math.sin(a) * r * 0.95);
                }
            } else if (g === 1) {
                ctx.arc(x, y, r * 0.8, 0.9, TAU - 0.9);
                ctx.arc(x + r * 0.34, y, r * 0.62, TAU - 1.05, 1.05, true);
            } else if (g === 2) {
                for (let k = -1; k <= 1; k++) {
                    ctx.moveTo(x - r * 0.9, y + k * r * 0.55);
                    ctx.bezierCurveTo(x - r * 0.4, y + k * r * 0.55 - r * 0.45, x + r * 0.1, y + k * r * 0.55 + r * 0.45, x + r * 0.9, y + k * r * 0.55);
                }
            } else {
                ctx.moveTo(x, y + r * 0.95);
                ctx.lineTo(x, y - r * 0.2);
                ctx.moveTo(x, y + r * 0.35);
                ctx.lineTo(x - r * 0.6, y - r * 0.35);
                ctx.moveTo(x, y + r * 0.05);
                ctx.lineTo(x + r * 0.65, y - r * 0.55);
                ctx.moveTo(x, y - r * 0.2);
                ctx.lineTo(x - r * 0.25, y - r * 0.9);
                ctx.moveTo(x, y - r * 0.2);
                ctx.lineTo(x + r * 0.3, y - r * 0.85);
            }
            ctx.stroke();
            ctx.restore();
        }

        // ---- swinging ropes ----
        const STYLE_TINT = { chain: ['#8b93a3', 0.8], vine: ['#4f7d3a', 0.55] };

        function swingEnd(s) {
            if (s.nodes && s.nodes.length) return s.nodes[s.nodes.length - 1];
            return { x: s.ax + s.len * Math.sin(s.angle || 0), y: s.ay + s.len * Math.cos(s.angle || 0) };
        }

        function strokeSpline(pts, color, width) {
            ctx.strokeStyle = color;
            ctx.lineWidth = width;
            ctx.beginPath();
            ctx.moveTo(pts[0].x, pts[0].y);
            for (let k = 1; k < pts.length - 1; k++) {
                ctx.quadraticCurveTo(pts[k].x, pts[k].y, (pts[k].x + pts[k + 1].x) * 0.5, (pts[k].y + pts[k + 1].y) * 0.5);
            }
            ctx.lineTo(pts[pts.length - 1].x, pts[pts.length - 1].y);
            ctx.stroke();
        }

        function drawSwings(sim, view) {
            const list = sim.stage.swings;
            if (!list || !list.length) return;
            const p = sim.player;
            let held = null;
            for (let i = 0; i < list.length; i++) {
                const s = list[i];
                if (!(s.len > 0)) continue;
                const pts = (s.nodes && s.nodes.length >= 2) ? s.nodes : [{ x: s.ax, y: s.ay }, swingEnd(s)];
                let minX = s.ax - 24, maxX = s.ax + 24;
                for (let k = 0; k < pts.length; k++) {
                    if (pts[k].x < minX) minX = pts[k].x;
                    if (pts[k].x > maxX) maxX = pts[k].x;
                }
                if (!vis(view, minX - 20, maxX + 20)) continue;
                const style = s.style || 'rope';
                const tint = STYLE_TINT[style];
                ctx.save();
                ctx.lineCap = 'round';
                ctx.lineJoin = 'round';
                strokeSpline(pts, 'rgba(16, 12, 10, 0.35)', 6.5);
                if (!has('prop-swing.rope')) {
                    strokeSpline(pts, style === 'chain' ? '#8b93a3' : style === 'vine' ? '#4f7d3a' : '#8a6440', 5);
                    strokeSpline(pts, style === 'chain' ? 'rgba(220,230,245,0.4)' : style === 'vine' ? 'rgba(160,220,120,0.3)' : 'rgba(255,225,170,0.3)', 1.5);
                }
                ctx.restore();
                if (has('prop-swing.rope')) {
                    for (let k = 0; k < pts.length - 1; k++) {
                        const a = pts[k], b = pts[k + 1];
                        const dx = b.x - a.x, dy = b.y - a.y;
                        const sl = Math.hypot(dx, dy) || 1;
                        ctx.save();
                        ctx.translate((a.x + b.x) * 0.5, (a.y + b.y) * 0.5);
                        ctx.rotate(Math.atan2(-dx, dy));
                        ctx.scale(1, (sl + 3) / (179 * 0.2));
                        put('prop-swing.rope', 0, 0, 0, { scale: 0.2, tint: tint ? tint[0] : '', amount: tint ? tint[1] : 0 });
                        ctx.restore();
                    }
                }
                if (style === 'chain') {
                    ctx.save();
                    for (let k = 0; k < pts.length - 1; k++) {
                        const a = pts[k], b = pts[k + 1], ang = Math.atan2(b.y - a.y, b.x - a.x);
                        for (let m = 0; m < 2; m++) {
                            const u = (m + 0.5) / 2, lx = a.x + (b.x - a.x) * u, ly = a.y + (b.y - a.y) * u;
                            const flat = (k * 2 + m) % 2 === 0;
                            ctx.strokeStyle = 'rgba(25, 28, 38, 0.8)';
                            ctx.lineWidth = 1.6;
                            ctx.beginPath(); ctx.ellipse(lx, ly, 4.6, flat ? 2.6 : 1.3, ang, 0, TAU); ctx.stroke();
                            ctx.strokeStyle = 'rgba(215, 225, 240, 0.55)';
                            ctx.lineWidth = 1;
                            ctx.beginPath(); ctx.ellipse(lx - 0.5, ly - 0.5, 3.2, flat ? 1.5 : 0.7, ang, 0, Math.PI); ctx.stroke();
                        }
                    }
                    ctx.restore();
                } else if (style === 'vine') {
                    ctx.save();
                    for (let k = 2; k < pts.length - 1; k += 2) {
                        const a = pts[k - 1], b = pts[k + 1], n = pts[k], side = (k / 2) % 2 === 0 ? 1 : -1;
                        ctx.save();
                        ctx.translate(n.x, n.y);
                        ctx.rotate(Math.atan2(-(b.x - a.x), b.y - a.y) + side * 0.85);
                        ctx.fillStyle = '#3f6e2d';
                        ctx.beginPath(); ctx.ellipse(side * 6, 0, 6.5, 2.8, side * 0.2, 0, TAU); ctx.fill();
                        ctx.fillStyle = '#74b84d';
                        ctx.beginPath(); ctx.ellipse(side * 5.5, -0.6, 4.2, 1.4, side * 0.2, 0, TAU); ctx.fill();
                        ctx.restore();
                    }
                    ctx.restore();
                }
                // anchor (ring under a beam, branch for vines) painted over the rope start
                const anchorClip = style === 'vine' ? 'prop-swing.branch' : 'prop-swing.ring';
                ctx.save();
                ctx.translate(s.ax, s.ay);
                if (style !== 'vine') ctx.rotate(Math.atan2(-(pts[1].x - pts[0].x), pts[1].y - pts[0].y) * 0.35);
                if (!put(anchorClip, 0, 0, -11, { scale: style === 'vine' ? 0.4 : 0.34 })) {
                    ctx.fillStyle = '#5a6275'; ctx.fillRect(-22, -14, 44, 10);
                    ctx.strokeStyle = '#9aa3b5'; ctx.lineWidth = 3;
                    ctx.beginPath(); ctx.arc(0, 0, 7, 0, TAU); ctx.stroke();
                }
                ctx.restore();
                // grip at the rope end aligned with the final segment tangent
                const last = pts[pts.length - 1], prev = pts[pts.length - 2];
                ctx.save();
                ctx.translate(last.x, last.y);
                ctx.rotate(Math.atan2(-(last.x - prev.x), last.y - prev.y));
                if (!put('prop-swing.grip', 0, 0, 86 * 0.2 - 4, { scale: 0.2, tint: tint ? tint[0] : '', amount: tint ? 0.35 : 0 })) {
                    ctx.fillStyle = '#6b4a2f'; ctx.fillRect(-12, -2, 24, 8);
                }
                ctx.restore();
                if (p && p.swing && p.swing.id === s.id) held = s;
            }
            // motion trail of the hanging hero's hands at speed
            if (held && !view.reducedMotion) {
                const e = { x: p.x, y: p.y - 40 };
                const last = trail[trail.length - 1];
                if (!last || last.t !== view.nowSec) trail.push({ x: e.x, y: e.y, t: view.nowSec });
                if (trail.length > 12) trail.shift();
                const sp = Math.abs((p.swing.omega || held.omega || 0) * held.len);
                if (sp > 180 && trail.length > 2) {
                    ctx.save();
                    ctx.globalCompositeOperation = 'lighter';
                    ctx.lineCap = 'round';
                    for (let i = 1; i < trail.length; i++) {
                        const k = i / trail.length;
                        ctx.strokeStyle = 'rgba(190, 235, 255, ' + (k * clamp((sp - 180) / 360, 0.15, 0.55)) + ')';
                        ctx.lineWidth = 2 + k * 9;
                        ctx.beginPath();
                        ctx.moveTo(trail[i - 1].x, trail[i - 1].y + 24);
                        ctx.lineTo(trail[i].x, trail[i].y + 24);
                        ctx.stroke();
                    }
                    ctx.restore();
                }
            } else trail.length = 0;
        }

        // ---- puzzles ----
        function drawPip(x, y, on, rgb) {
            ctx.save();
            ctx.fillStyle = on ? 'rgba(' + rgb + ', 0.95)' : 'rgba(20, 22, 34, 0.7)';
            ctx.strokeStyle = 'rgba(' + rgb + ', ' + (on ? 1 : 0.5) + ')';
            ctx.lineWidth = 1.5;
            ctx.beginPath();
            ctx.moveTo(x, y - 5);
            ctx.lineTo(x + 4, y);
            ctx.lineTo(x, y + 5);
            ctx.lineTo(x - 4, y);
            ctx.closePath();
            ctx.fill();
            ctx.stroke();
            ctx.restore();
        }

        function drawOracles(sim, view) {
            const list = sim.stage.oracles;
            if (!list) return;
            for (let i = 0; i < list.length; i++) {
                const o = list[i];
                if (!vis(view, o.x, o.x)) continue;
                const st = o.state || 'idle';
                const age = stamp(o, st, view.nowSec);
                const active = st === 'showing' || st === 'input';
                const cur = st === 'showing' && o.showLit && o.seq && o.showIdx >= 0 ? o.seq[o.showIdx] : -1;
                const speaking = st === 'showing' && cur >= 0;
                const clip = st === 'solved' ? 'prop-oracle.calm' : speaking ? 'prop-oracle.speaking' : active ? 'prop-oracle.eyes' : 'prop-oracle.dormant';
                const feet = o.y;
                const failFlash = st === 'fail' ? clamp(1 - age / 0.6, 0, 1) : 0;
                const goldFlash = st === 'solved' ? clamp(1 - age / 1.2, 0, 1) : 0;
                if (active || st === 'solved') glow(o.x, feet - 62, 100, st === 'solved' ? '250,204,21' : '90,220,240', 0.4 + 0.2 * pulse(view, 5));
                if (!put(clip, 0, o.x, feet, { height: 104, tint: failFlash > 0 ? '#ff4a4a' : goldFlash > 0 ? '#ffd24a' : '', amount: Math.max(failFlash, goldFlash) * 0.6 })) {
                    ctx.fillStyle = active ? '#5d7280' : '#46505e';
                    ctx.fillRect(o.x - 18, feet - 100, 36, 100);
                    ctx.fillStyle = active ? '#7ee8fa' : '#222a35';
                    ctx.fillRect(o.x - 12, feet - 80, 8, 6);
                    ctx.fillRect(o.x + 4, feet - 80, 8, 6);
                }
                if (cur >= 0) {
                    const bob = view.reducedMotion ? 0 : Math.sin(view.nowSec * 3.1) * 4;
                    const gy = feet - 132 + bob;
                    glow(o.x, gy, 62, GLYPH_RGB[cur] || '255,255,255', 0.8);
                    glyphShape(cur, o.x, gy, 20, 'rgb(' + (GLYPH_RGB[cur] || '255,255,255') + ')');
                }
                const n = o.seq ? o.seq.length : 0;
                if (active && n) {
                    const prog = st === 'showing' ? Math.max(0, (o.showIdx | 0) + (o.showLit ? 1 : 0)) : (o.step | 0);
                    for (let k = 0; k < n; k++) drawPip(o.x + (k - (n - 1) / 2) * 14, feet - 112, k < prog, st === 'input' ? '250,204,21' : '90,220,240');
                }
            }
        }

        function drawStones(sim, view) {
            const list = sim.stage.glyphStones;
            if (!list) return;
            const oracles = sim.stage.oracles || [];
            for (let i = 0; i < list.length; i++) {
                const g = list[i];
                if (!vis(view, g.x, g.x)) continue;
                const gl = clamp(g.glyph | 0, 0, 3);
                const age = stamp(g, !!g.lit, view.nowSec);
                let flashRed = 0;
                let flashGold = 0;
                for (let k = 0; k < oracles.length; k++) {
                    if (oracles[k].id !== g.oracleId) continue;
                    const oa = stamp(oracles[k], oracles[k].state || 'idle', view.nowSec);
                    if (oracles[k].state === 'fail') flashRed = clamp(1 - oa / 0.6, 0, 1);
                    if (oracles[k].state === 'solved') flashGold = clamp(1 - oa / 1.2, 0, 1);
                }
                const lit = !!g.lit || flashGold > 0;
                if (lit) glow(g.x, g.y - 32, 56 + (age < 0.3 ? (0.3 - age) * 120 : 0), GLYPH_RGB[gl], 0.55 + 0.2 * pulse(view, 7));
                const clip = (lit ? 'prop-glyphs.lit_' : 'prop-glyphs.dormant_') + gl;
                const sq = age < 0.18 && g.lit ? 1 + (0.18 - age) * 0.5 : 1;
                ctx.save();
                ctx.translate(g.x, g.y);
                ctx.scale(sq, sq);
                if (!put(clip, 0, 0, 0, { height: 64, tint: flashRed > 0 ? '#ff3b3b' : flashGold > 0 ? '#ffd24a' : '', amount: Math.max(flashRed, flashGold) * 0.65 })) {
                    ctx.fillStyle = '#6b7280';
                    ctx.fillRect(-18, -56, 36, 56);
                    glyphShape(gl, 0, -30, 12, lit ? 'rgb(' + GLYPH_RGB[gl] + ')' : '#2b303b');
                }
                ctx.restore();
            }
        }

        function maskList(mask, count) {
            if (Array.isArray(mask)) return mask;
            const out = [];
            if (typeof mask === 'number') for (let i = 0; i < count; i++) if (mask & (1 << i)) out.push(i);
            return out;
        }

        function drawBridge(sim, view) {
            const segs = sim.stage.bridgeSegs;
            if (!segs || !segs.length) return;
            const T = sim.stage.tileSize || 48;
            for (let i = 0; i < segs.length; i++) {
                const sg = segs[i];
                if (!vis(view, sg.x, sg.x + sg.w)) continue;
                const age = stamp(sg, !!sg.on, view.nowSec);
                const k = sg.on ? clamp(age / 0.35, 0, 1) : 1 - clamp(age / 0.35, 0, 1);
                if (k < 1) {
                    // ghost rune outline
                    const gp = 0.28 + 0.18 * pulse(view, 4 + (i % 3));
                    ctx.save();
                    ctx.globalAlpha = (1 - k) * gp * 1.6;
                    ctx.fillStyle = 'rgba(126, 232, 250, 0.1)';
                    ctx.fillRect(sg.x, sg.y, sg.w, 16);
                    ctx.setLineDash([7, 5]);
                    ctx.strokeStyle = 'rgba(160, 240, 255, 0.9)';
                    ctx.lineWidth = 1.6;
                    ctx.strokeRect(sg.x + 1, sg.y + 1, sg.w - 2, 14);
                    ctx.setLineDash([]);
                    ctx.beginPath();
                    const cx = sg.x + sg.w / 2;
                    ctx.moveTo(cx, sg.y + 3);
                    ctx.lineTo(cx + 5, sg.y + 8);
                    ctx.lineTo(cx, sg.y + 13);
                    ctx.lineTo(cx - 5, sg.y + 8);
                    ctx.closePath();
                    ctx.stroke();
                    ctx.restore();
                }
                if (k > 0.02) {
                    const n = Math.max(1, Math.round(sg.w / T));
                    const tw = sg.w / n;
                    const dh = Math.round(MAGIC.sh * (T / MAGIC.sw));
                    ctx.save();
                    ctx.globalAlpha = k;
                    for (let t = 0; t < n; t++) h.drawCroppedAnimation('jump-and-run', 'magic_platform', view.nowSec + t * 0.25, MAGIC.sx, MAGIC.sy, MAGIC.sw, MAGIC.sh, sg.x + t * tw, sg.y - 1 - (1 - k) * 8, tw + 1, dh);
                    ctx.globalCompositeOperation = 'lighter';
                    ctx.fillStyle = 'rgba(126, 232, 250, ' + (0.1 + 0.08 * pulse(view, 5)) + ')';
                    ctx.fillRect(sg.x, sg.y - 2, sg.w, 5);
                    ctx.restore();
                    if (age < 0.5 && sg.on) glow(sg.x + sg.w / 2, sg.y, 80, '126,232,250', (0.5 - age) * 1.4);
                }
            }
        }

        function drawLevers(sim, view) {
            const list = sim.stage.levers;
            if (!list) return;
            const p = sim.player;
            const segs = sim.stage.bridgeSegs || [];
            for (let i = 0; i < list.length; i++) {
                const lv = list[i];
                if (!vis(view, lv.x, lv.x)) continue;
                const age = stamp(lv, !!lv.on, view.nowSec);
                // faint rune links to the segments this lever toggles while the hero is close
                if (p && Math.abs(p.x - lv.x) < 150 && Math.abs(p.y - lv.y) < 110) {
                    const mates = [];
                    for (let k = 0; k < segs.length; k++) if (segs[k].puzzleId === lv.puzzleId) mates.push(segs[k]);
                    const ids = maskList(lv.mask, mates.length);
                    ctx.save();
                    ctx.setLineDash([3, 7]);
                    ctx.lineDashOffset = view.reducedMotion ? 0 : -view.nowSec * 22;
                    ctx.lineWidth = 1.8;
                    for (let k = 0; k < ids.length; k++) {
                        const sg = mates[ids[k]];
                        if (!sg) continue;
                        ctx.strokeStyle = 'rgba(160, 240, 255, ' + (0.38 + 0.2 * pulse(view, 6)) + ')';
                        ctx.beginPath();
                        ctx.moveTo(lv.x, lv.y - 46);
                        ctx.quadraticCurveTo((lv.x + sg.x + sg.w / 2) / 2, Math.min(lv.y, sg.y) - 90, sg.x + sg.w / 2, sg.y - 6);
                        ctx.stroke();
                    }
                    ctx.restore();
                }
                if (lv.on) glow(lv.x, lv.y - 24, 56, '90,230,255', 0.45 + 0.15 * pulse(view, 6));
                const clip = lv.on ? (age > 0.4 && Math.floor(view.nowSec * 0.8) % 3 === 0 ? 'prop-lever.glint' : 'prop-lever.on') : 'prop-lever.off';
                const mid = age < 0.12 ? 'prop-lever.mid' : clip;
                if (!put(mid, view.nowSec, lv.x, lv.y, { height: 66, flip: !lv.on && mid === 'prop-lever.off' })) {
                    ctx.fillStyle = '#4b5563';
                    ctx.fillRect(lv.x - 18, lv.y - 20, 36, 20);
                    ctx.strokeStyle = lv.on ? '#7ee8fa' : '#9ca3af';
                    ctx.lineWidth = 5;
                    ctx.beginPath();
                    ctx.moveTo(lv.x, lv.y - 18);
                    ctx.lineTo(lv.x + (lv.on ? 16 : -16), lv.y - 46);
                    ctx.stroke();
                }
            }
        }

        // 0..1 progress of a rising / sinking visual (eases towards the target and keeps its own value).
        function easeVis(obj, target, dt) {
            let s = seen.get(obj);
            if (!s || s.v === undefined) { s = { v: target, on: target > 0.5 }; seen.set(obj, s); }
            s.v += clamp(target - s.v, -4.5 * dt, 4.5 * dt);
            return s.v;
        }

        function remaining(r) {
            return clamp((+r.timeLimit || 0) - (+r.t || 0), 0, +r.timeLimit || 0); // r.t counts up
        }

        function drawRituals(sim, view) {
            const list = sim.stage.rituals;
            if (!list) return;
            const T = sim.stage.tileSize || 48;
            for (let i = 0; i < list.length; i++) {
                const r = list[i];
                const active = r.state === 'active';
                const done = r.state === 'done';
                (r.stairs || []).forEach(function (st) {
                    if (!vis(view, st.x, st.x + st.w)) return;
                    const v = easeVis(st, st.up ? 1 : 0, view.dt);
                    const n = Math.max(1, Math.round(st.w / T));
                    const tw = st.w / n;
                    if (v < 0.98) {
                        ctx.save();
                        ctx.globalAlpha = (1 - v) * (0.25 + 0.15 * pulse(view, 4));
                        ctx.setLineDash([6, 5]);
                        ctx.strokeStyle = 'rgba(250, 204, 21, 0.9)';
                        ctx.lineWidth = 1.5;
                        ctx.strokeRect(st.x + 1, st.y + 1, st.w - 2, 16);
                        ctx.restore();
                    }
                    if (v > 0.02) {
                        const rise = (1 - v) * 26;
                        ctx.save();
                        ctx.globalAlpha = Math.min(1, v * 1.3);
                        const dh = Math.round(MAGIC.sh * (T / MAGIC.sw));
                        for (let t = 0; t < n; t++) h.drawCroppedAnimation('jump-and-run', 'magic_platform', view.nowSec + t * 0.3, MAGIC.sx, MAGIC.sy, MAGIC.sw, MAGIC.sh, st.x + t * tw, st.y - 1 + rise, tw + 1, dh);
                        ctx.globalCompositeOperation = 'lighter';
                        ctx.fillStyle = 'rgba(250, 204, 21, ' + (0.12 + 0.1 * pulse(view, 5)) + ')';
                        ctx.fillRect(st.x, st.y - 2 + rise, st.w, 5);
                        ctx.restore();
                        if (v < 0.98 && !view.reducedMotion && rnd() < 0.5) ember(st.x + rnd() * st.w, st.y + rise + 10, (rnd() - 0.5) * 20, -30 - rnd() * 30, 0.5, 2, '250,204,21');
                    }
                });
                const pl = r.plate;
                if (pl && vis(view, pl.x, pl.x)) {
                    const clip = done ? 'prop-plate.lit' : active ? 'prop-plate.pulse' : pl.down ? 'prop-plate.down' : 'prop-plate.up';
                    if (active) glow(pl.x, pl.y - 6, 60, '250,204,21', 0.4);
                    if (!put(clip, view.nowSec, pl.x, pl.y + 2, { height: 44 })) {
                        ctx.fillStyle = pl.down ? '#6b7280' : '#8b93a3';
                        ctx.fillRect(pl.x - 26, pl.y - (pl.down ? 4 : 10), 52, pl.down ? 6 : 12);
                    }
                    if (active && r.timeLimit > 0) {
                        const f = remaining(r) / r.timeLimit;
                        const cy = pl.y - 40;
                        const urgent = f < 0.3;
                        ctx.save();
                        ctx.lineCap = 'round';
                        ctx.strokeStyle = 'rgba(8, 10, 18, 0.75)';
                        ctx.lineWidth = 7;
                        ctx.beginPath();
                        ctx.arc(pl.x, cy, 15, 0, TAU);
                        ctx.stroke();
                        ctx.strokeStyle = urgent ? (Math.floor(view.nowSec * 8) % 2 ? '#ff6b4a' : '#ffd0a0') : '#fbbf24';
                        ctx.lineWidth = 4.5;
                        ctx.beginPath();
                        ctx.arc(pl.x, cy, 15, -Math.PI / 2, -Math.PI / 2 + TAU * f);
                        ctx.stroke();
                        ctx.restore();
                    }
                }
                const bz = r.braziers || [];
                let minOrder = 99;
                bz.forEach(function (b) { if (b.order < minOrder) minOrder = b.order; });
                bz.forEach(function (b) {
                    if (!vis(view, b.x, b.x)) return;
                    const age = stamp(b, !!b.lit, view.nowSec);
                    const clip = !b.lit ? 'prop-brazier.unlit' : age < 0.18 ? 'prop-brazier.ignite' : 'prop-brazier.burn';
                    if (b.lit) glow(b.x, b.y - 50, 96, '255,150,50', 0.55 + 0.15 * pulse(view, 9));
                    if (!put(clip, view.nowSec + b.x * 0.01, b.x, b.y, { height: 112 })) {
                        ctx.fillStyle = '#4b5563';
                        ctx.fillRect(b.x - 16, b.y - 30, 32, 30);
                        if (b.lit) { ctx.fillStyle = '#fb923c'; ctx.fillRect(b.x - 10, b.y - 52, 20, 22); }
                    }
                    if (!b.lit && !done) {
                        const n = b.order - minOrder + 1;
                        for (let k = 0; k < n; k++) drawPip(b.x + (k - (n - 1) / 2) * 11, b.y - 90, false, active ? '250,204,21' : '170,170,190');
                    }
                    if (b.lit && age < 0.6 && !view.reducedMotion) ember(b.x + (rnd() - 0.5) * 18, b.y - 50, (rnd() - 0.5) * 60, -60 - rnd() * 60, 0.7, 2.4, '255,170,70');
                });
            }
        }

        // ---- dragon set piece ----
        function vsFor(run, view) {
            const key = run.id == null ? 'r' : String(run.id);
            let s = runVis[key];
            if (!s) s = runVis[key] = { state: '', age: 0, fireAge: 0, lastFire: false, px: 0, py: 0 };
            if (s.state !== run.state) { s.state = run.state; s.age = 0; }
            if (s.stepped !== view.nowSec) {
                s.stepped = view.nowSec;
                s.age += view.dt;
                if (run.fire) { s.fireAge += view.dt; s.lastFire = true; } else { s.fireAge = 0; s.lastFire = false; }
            }
            return s;
        }

        function sheetFor(run) {
            return run.kind === 'frost' ? 'dragon-frost' : 'dragon-ember';
        }

        function runScale(T) {
            return (T * 8) / DRAGON_PX;
        }

        function mouthOf(run, x, y, sc) {
            const d = run.dir < 0 ? -1 : 1;
            return { x: x + d * 150 * sc, y: y - 20 * sc };
        }

        function dragonClip(run, s) {
            const id = sheetFor(run);
            if (run.state === 'strafe' && run.fire) return { clip: id + '.attack', time: s.fireAge };
            if (run.state === 'strafe' && s.age < 0.32) return { clip: id + '.telegraph', time: 0, frame: s.age < 0.16 ? 0 : 1 };
            return { clip: id + '.idle', time: 0 };
        }

        function drawDragon(run, x, y, sc, view, extra) {
            const s = runVis[String(run.id == null ? 'r' : run.id)] || { age: 0, fireAge: 0 };
            const c = dragonClip(run, s);
            const o = extra || {};
            const frame = c.frame;
            const clock = c.clip.indexOf('.idle') > 0 ? view.nowSec * (o.fast || 1) : c.time;
            return put(c.clip, clock, x, y, { scale: sc, flip: run.dir < 0, alpha: o.alpha, rot: o.rot, tint: o.tint, amount: o.amount, frame: frame });
        }

        // A hazard stream from (x, y) (its left edge or, mirrored, its right edge, mid-height) along w with thickness hgt.
        function stream(x, y, w, hgt, age, dir, kind, alpha) {
            if (!has('fx-dragonfire.loop') || w < 12) return false;
            const n = Math.max(1, Math.round(w / 300));
            const seg = w / n;
            const ysc = clamp(hgt / 56, 0.5, 3.5) / (seg / 256);
            const frost = kind === 'frost';
            for (let pass = 0; pass < 2; pass++) {
                for (let i = 0; i < n; i++) {
                    const x0 = dir < 0 ? x + w - i * seg : x + i * seg;
                    const clip = i === 0 && age < 0.14 ? 'fx-dragonfire.start' : 'fx-dragonfire.loop';
                    ctx.save();
                    ctx.translate(x0, y);
                    ctx.scale(1, ysc);
                    put(clip, age + i * 0.09, 0, 0, {
                        scale: seg / 256,
                        flip: dir < 0,
                        alpha: (pass ? 0.4 : 1) * alpha,
                        comp: pass ? 'lighter' : '',
                        tint: frost ? (pass ? '#5cc8ff' : '#a9e8ff') : '',
                        amount: frost ? (pass ? 0.8 : 0.6) : 0
                    });
                    ctx.restore();
                }
            }
            return true;
        }

        function streamRect(rect, age, dir, kind, alpha) {
            if (!stream(rect.x, rect.y + rect.h / 2, rect.w, rect.h * 1.35, age, dir, kind, alpha)) {
                ctx.save();
                const g = ctx.createLinearGradient(rect.x, 0, rect.x + rect.w, 0);
                const c = kind === 'frost' ? '140,230,255' : '255,140,40';
                g.addColorStop(0, 'rgba(' + c + ', 0.85)');
                g.addColorStop(1, 'rgba(' + c + ', 0.15)');
                ctx.globalAlpha = alpha;
                ctx.fillStyle = g;
                ctx.fillRect(rect.x, rect.y, rect.w, rect.h);
                ctx.restore();
            }
        }

        function drawWarn(run, view, s) {
            const T = 48;
            const k = clamp(s.age / 1.2, 0, 1);
            const pu = 0.5 + 0.5 * Math.sin(s.age * 16);
            const rgb = KIND_RGB[run.kind] || KIND_RGB.ember;
            const ly = run.laneY != null ? run.laneY : 0;
            const lh = run.laneH > 0 ? run.laneH : 70;
            const x0 = run.x1;
            const w = run.x2 - run.x1;
            ctx.save();
            ctx.beginPath();
            ctx.rect(x0, ly, w, lh);
            ctx.clip();
            ctx.fillStyle = 'rgba(' + rgb + ', ' + (0.1 + 0.12 * pu) + ')';
            ctx.fillRect(x0, ly, w, lh);
            ctx.strokeStyle = 'rgba(' + rgb + ', ' + (0.3 + 0.2 * pu) + ')';
            ctx.lineWidth = 9;
            const off = ((view.nowSec * 90 * (run.dir < 0 ? -1 : 1)) % 44 + 44) % 44;
            for (let x = x0 - lh - 44; x < x0 + w + 44; x += 44) {
                ctx.beginPath();
                ctx.moveTo(x + off, ly + lh);
                ctx.lineTo(x + off + lh * 0.7, ly);
                ctx.stroke();
            }
            ctx.restore();
            ctx.save();
            ctx.strokeStyle = 'rgba(' + rgb + ', ' + (0.55 + 0.35 * pu) + ')';
            ctx.lineWidth = 2;
            ctx.strokeRect(x0 + 1, ly + 1, w - 2, lh - 2);
            ctx.restore();
            // the big shadow gliding in from the dragon's side
            const d = run.dir < 0 ? -1 : 1;
            const sx = d > 0 ? x0 - T * 5 + (w * 0.35 + T * 5) * k : run.x2 + T * 5 - (w * 0.35 + T * 5) * k;
            drawShadow(sx, ly + lh, T * 8, 0.35 * k);
        }

        function drawShadow(x, groundY, width, alpha) {
            ctx.save();
            ctx.globalAlpha = clamp(alpha, 0, 0.6);
            ctx.fillStyle = 'rgba(6, 8, 16, 0.9)';
            ctx.beginPath();
            ctx.ellipse(x, groundY + 4, width * 0.5, width * 0.06, 0, 0, TAU);
            ctx.fill();
            ctx.restore();
        }

        function drawRunWorld(run, sim, view) {
            if (!run || run.state === 'off') return;
            const s = vsFor(run, view);
            const T = sim.stage.tileSize || 48;
            const sc = runScale(T);
            if (run.state === 'warn') {
                if (run.x2 > run.x1 && vis(view, run.x1, run.x2)) drawWarn(run, view, s);
                // the dragon rears at the zone edge and roars
                if (isFinite(run.dx) && isFinite(run.dy) && vis(view, run.dx - T * 5, run.dx + T * 5)) {
                    put(sheetFor(run) + '.roar', 0, run.dx, run.dy, { scale: sc, flip: run.dir < 0, alpha: clamp(s.age * 3, 0, 1) });
                }
                return;
            }
            if ((run.state !== 'strafe' && run.state !== 'leave') || !(isFinite(run.dx) && isFinite(run.dy))) return;
            if (!vis(view, run.dx - T * 6, run.dx + T * 6)) return;
            const leaving = run.state === 'leave';
            const alpha = leaving ? clamp(1.2 - s.age * 0.45, 0.25, 1) : clamp(s.age * 3, 0, 1);
            const ground = run.laneY != null ? run.laneY + (run.laneH || 70) : run.dy + T * 4;
            drawShadow(run.dx, ground, T * 8 * (0.8 + 0.2 * clamp((ground - run.dy) / 300, 0, 1)), 0.45 * alpha * clamp(1 - (ground - run.dy) / 700, 0.3, 1));
            const fast = run.state === 'strafe' ? 1.25 : 1;
            if (!drawDragon(run, run.dx, run.dy, sc, view, { alpha: alpha, rot: leaving ? -0.22 * (run.dir < 0 ? -1 : 1) : 0, fast: fast })) {
                ctx.save();
                ctx.globalAlpha = alpha;
                ctx.fillStyle = run.kind === 'frost' ? '#7aa7c7' : '#7a2a1c';
                ctx.beginPath();
                ctx.ellipse(run.dx, run.dy, T * 3.4, T * 0.9, 0, 0, TAU);
                ctx.fill();
                ctx.restore();
            }
            if (!view.reducedMotion) {
                const rgb = KIND_RGB[run.kind] || KIND_RGB.ember;
                if (rnd() < view.dt * 30) ember(run.dx + (rnd() - 0.5) * T * 6, run.dy + T * 0.6, (rnd() - 0.5) * 40, 30 + rnd() * 50, 1.3 + rnd(), 2 + rnd() * 2, rgb);
                if (rnd() < view.dt * 20) ember(view.sx + rnd() * view.viewW, view.sy - 10, (rnd() - 0.5) * 30, 90 + rnd() * 90, 1.6 + rnd(), 1.5 + rnd() * 2, rgb);
            }
        }

        // Dragon fire in front of the hero (the hazard stays readable even when the hero stands in it).
        function drawRunFire(run, sim, view) {
            if (!run || run.state !== 'strafe' || !run.fire) return;
            const f = run.fire;
            if (!vis(view, f.x, f.x + f.w)) return;
            const s = vsFor(run, view);
            const d = run.dir < 0 ? -1 : 1;
            const sc = runScale(sim.stage.tileSize || 48);
            const rgb = KIND_RGB[run.kind] || KIND_RGB.ember;
            if (isFinite(run.dx) && isFinite(run.dy)) {
                const m = mouthOf(run, run.dx, run.dy, sc);
                const near = d > 0 ? f.x : f.x + f.w;
                const dist = Math.hypot(near - m.x, f.y + f.h / 2 - m.y);
                if (dist > 36 && has('fx-dragonfire.loop')) {
                    const ang = Math.atan2(f.y + f.h / 2 - m.y, Math.abs(near - m.x));
                    ctx.save();
                    ctx.translate(m.x, m.y);
                    ctx.scale(d, 1);
                    ctx.rotate(ang);
                    stream(0, 0, dist, Math.max(40, f.h * 0.7), s.fireAge, 1, run.kind, 1);
                    ctx.restore();
                }
                glow(m.x, m.y, 90, rgb, 0.8);
            }
            streamRect(f, s.fireAge, d, run.kind, 1);
            glow(f.x + f.w / 2, f.y + f.h / 2, Math.max(120, f.w * 0.45), rgb, 0.3);
            if (!view.reducedMotion && rnd() < view.dt * 40) ember(f.x + rnd() * f.w, f.y + f.h * rnd(), (rnd() - 0.5) * 50, -40 - rnd() * 70, 0.8, 2 + rnd() * 2, rgb);
        }

        // Boss dragon fire (temp hazard `dragonfire`): the whole rect is a tiled stream.
        function drawBossFire(sim, view) {
            const list = sim.tempHazards;
            if (!list) return;
            for (let i = 0; i < list.length; i++) {
                const th = list[i];
                if (th.kind !== 'dragonfire' || !vis(view, th.x, th.x + th.w)) continue;
                const life = clamp(th.t / (th.dur || 1), 0, 1);
                const age = (th.dur || 1) - th.t;
                const frost = th.variant === 'frost';
                const alpha = clamp(life * 4, 0, 1);
                const kind = frost ? 'frost' : 'ember';
                const rgb = KIND_RGB[kind];
                streamRect(th, 0.2 + age, th.dir < 0 ? -1 : 1, kind, alpha);
                glow(th.x + th.w / 2, th.y + th.h / 2, Math.max(140, th.w * 0.4), rgb, 0.3 * alpha);
                if (!view.reducedMotion && rnd() < view.dt * 40) ember(th.x + rnd() * th.w, th.y + th.h * rnd(), (rnd() - 0.5) * 50, -40 - rnd() * 70, 0.8, 2 + rnd() * 2, rgb);
            }
        }

        function activeRun(sim, states) {
            const list = sim.stage.dragonRuns;
            if (!list) return null;
            const p = sim.player;
            let best = null;
            let bd = 1e9;
            for (let i = 0; i < list.length; i++) {
                const r = list[i];
                if (!r || states.indexOf(r.state) < 0) continue;
                const d = p ? Math.abs((r.x1 + r.x2) / 2 - p.x) : 0;
                if (d < bd) { bd = d; best = r; }
            }
            return best;
        }

        // Far dragon: small, darkened, circling in the background (screen space, after the parallax layers).
        function drawFar(sim, view, W, Hh) {
            const run = activeRun(sim, ['far', 'warn']);
            if (!run || !has(sheetFor(run) + '.idle')) return;
            const p = sim.player;
            if (p && (p.x < run.x1 - 600 || p.x > run.x2 + 600)) return;
            const s = vsFor(run, view);
            const fade = run.state === 'far' ? clamp(s.age / 1.2, 0, 1) : clamp(1 - s.age / 1.2, 0, 1);
            if (fade <= 0.02) return;
            const t = view.nowSec * 0.33;
            const cx = W * 0.58 + Math.cos(t) * W * 0.26;
            const cy = Hh * 0.24 + Math.sin(t * 1.7) * Hh * 0.05;
            const dir = -Math.sin(t) >= 0 ? 1 : -1;
            const size = clamp(W * 0.12, 70, 150) / 335;
            ctx.save();
            put(sheetFor(run) + '.idle', view.nowSec, cx, cy, { scale: size * (0.92 + 0.08 * Math.sin(t * 1.7)), flip: dir < 0, alpha: 0.85 * fade, tint: run.kind === 'frost' ? '#10202e' : '#1a0f16', amount: 0.55, rot: Math.cos(t * 1.7) * 0.06 * dir });
            ctx.restore();
        }

        // Screen-edge glow on the side the dragon will come from, plus a faint fire wash while it strafes.
        function drawScreen(sim, view, W, Hh) {
            const run = activeRun(sim, ['warn', 'strafe']);
            if (!run) return;
            const s = vsFor(run, view);
            const rgb = KIND_RGB[run.kind] || KIND_RGB.ember;
            const pu = 0.5 + 0.5 * Math.sin(s.age * 14);
            ctx.save();
            if (run.state === 'warn') {
                const left = run.dir >= 0;
                const g = ctx.createLinearGradient(left ? 0 : W, 0, left ? W * 0.28 : W * 0.72, 0);
                g.addColorStop(0, 'rgba(' + rgb + ', ' + (0.3 + 0.3 * pu) + ')');
                g.addColorStop(1, 'rgba(' + rgb + ', 0)');
                ctx.fillStyle = g;
                ctx.fillRect(0, 0, W, Hh);
            } else if (run.fire) {
                ctx.globalCompositeOperation = 'lighter';
                ctx.fillStyle = 'rgba(' + rgb + ', ' + (0.05 + 0.03 * pu) + ')';
                ctx.fillRect(0, 0, W, Hh);
            }
            ctx.restore();
        }

        // ---- fireballs ----
        function drawFireballs(sim, view) {
            const pl = sim.projectiles;
            nextFb.length = 0;
            if (pl) {
                for (let i = 0; i < pl.length; i++) {
                    const pr = pl[i];
                    if (pr.dead || pr.kind !== 'fireball') continue;
                    nextFb.push(pr, pr.x, pr.y);
                    if (!vis(view, pr.x, pr.x)) continue;
                    const ang = Math.atan2(pr.vy, pr.vx || 0.001);
                    glow(pr.x, pr.y, 64, pr.variant === 'frost' ? '110,210,255' : '255,130,40', 0.7);
                    ctx.save();
                    ctx.translate(pr.x, pr.y);
                    ctx.rotate(ang);
                    const ok = put('fx-fireball.loop', view.nowSec + i * 0.05, -15, 0, { scale: pr.radius > 8 ? 0.36 : 0.3, tint: pr.variant === 'frost' ? '#6fd3ff' : '', amount: 0.75 });
                    if (!ok) {
                        ctx.fillStyle = '#fb923c';
                        ctx.beginPath();
                        ctx.arc(0, 0, 9, 0, TAU);
                        ctx.fill();
                    }
                    ctx.restore();
                    if (!view.reducedMotion && rnd() < 0.45) ember(pr.x, pr.y, (rnd() - 0.5) * 40, (rnd() - 0.5) * 40 + 20, 0.5, 2, '255,160,60');
                }
            }
            for (let i = 0; i < prevFb.length; i += 3) {
                const ref = prevFb[i];
                if (!pl || ref.dead || pl.indexOf(ref) < 0) impacts.push({ x: prevFb[i + 1], y: prevFb[i + 2], age: 0 });
            }
            const swap = prevFb;
            prevFb = nextFb;
            nextFb = swap;
            for (let i = impacts.length - 1; i >= 0; i--) {
                const im = impacts[i];
                im.age += view.dt;
                if (im.age > 0.3) { impacts.splice(i, 1); continue; }
                glow(im.x, im.y, 90, '255,140,50', 0.8 * (1 - im.age / 0.3));
                put('fx-fireball.impact', im.age, im.x, im.y, { scale: 0.42, comp: 'source-over' });
            }
        }

        function drawFx(view) {
            for (let i = pings.length - 1; i >= 0; i--) {
                const pg = pings[i];
                pg.age += view.dt;
                if (pg.age > 0.7) { pings.splice(i, 1); continue; }
                const k = pg.age / 0.7;
                ctx.save();
                ctx.globalCompositeOperation = 'lighter';
                ctx.globalAlpha = (1 - k) * 0.85;
                ctx.strokeStyle = 'rgb(' + pg.rgb + ')';
                ctx.lineWidth = 3 * (1 - k) + 1;
                ctx.beginPath();
                ctx.arc(pg.x, pg.y, pg.size * (0.3 + k), 0, TAU);
                ctx.stroke();
                ctx.restore();
            }
            if (!embers.length) return;
            ctx.save();
            ctx.globalCompositeOperation = 'lighter';
            for (let i = embers.length - 1; i >= 0; i--) {
                const e = embers[i];
                e.life -= view.dt;
                if (e.life <= 0) { embers.splice(i, 1); continue; }
                e.x += e.vx * view.dt + Math.sin(e.life * 7 + i) * 8 * view.dt;
                e.y += e.vy * view.dt;
                ctx.globalAlpha = clamp(e.life / e.max * 1.5, 0, 1);
                ctx.fillStyle = 'rgb(' + e.rgb + ')';
                ctx.fillRect(e.x - e.size / 2, e.y - e.size / 2, e.size, e.size);
            }
            ctx.restore();
        }

        // ---- frame hooks ----
        function step(sim, view) {
            if (stepped === view.nowSec) return;
            stepped = view.nowSec;
            let heat = 0;
            const list = sim.stage.dragonRuns;
            if (list) {
                for (let i = 0; i < list.length; i++) {
                    const r = list[i];
                    if (r.state === 'strafe' && r.fire) heat = 0.7;
                    else if (r.state === 'strafe' && heat < 0.35) heat = 0.35;
                    if (r.state === 'strafe' && sim.player && isFinite(r.dx)) {
                        const near = clamp(1 - Math.abs(r.dx - sim.player.x) / 720, 0, 1);
                        if (near > 0 && h.shake) h.shake(1.5 + near * 3.5);
                    }
                }
            }
            heatNow = heat;
        }

        function drawResonancePath(path, view) {
            if (!path) return;
            const runes = path.runes;
            if (!vis(view, runes[0].x, runes[runes.length - 1].x)) return;
            put('prop-waystones.' + view.biome.id, 0, runes[0].x - 24, path.groundY + 1, { height: 93 });
            if (path.complete) return;
            const first = path.nextRune;
            const time = view.reducedMotion ? 0 : view.nowSec;
            ctx.save();
            // The thread leads the eye to the next seal even against bright snow or sand.
            for (let i = Math.max(1, first); i < runes.length; i++) {
                const a = runes[i - 1], b = runes[i];
                const bright = i === first || (first === 0 && i === 1);
                ctx.beginPath();
                ctx.moveTo(a.x, a.y);
                ctx.quadraticCurveTo((a.x + b.x) / 2, Math.min(a.y, b.y) - 16, b.x, b.y);
                ctx.setLineDash([]);
                ctx.strokeStyle = bright ? 'rgba(8,37,51,.72)' : 'rgba(8,37,51,.38)';
                ctx.lineWidth = 3;
                ctx.stroke();
                ctx.setLineDash([3, 9]);
                ctx.lineDashOffset = -time * 15;
                ctx.strokeStyle = bright ? '#9bfbe3' : '#73a9ac';
                ctx.lineWidth = 1.5;
                ctx.stroke();
            }
            ctx.setLineDash([]);
            for (let i = first; i < runes.length; i++) {
                const rune = runes[i];
                const lit = i === first;
                const radius = lit ? 15 : 10;
                const y = rune.y + Math.sin(time * 2.5) * 2;
                if (lit) glow(rune.x, y, 44, '106,255,225', 0.55);
                ctx.save();
                ctx.translate(rune.x, y);
                // Chiseled brass bevel around a dark enamel face and a different engraved sign per stop.
                ctx.beginPath();
                for (let side = 0; side < 8; side++) {
                    const angle = side * TAU / 8 - Math.PI / 2;
                    const x = Math.cos(angle) * radius, dy = Math.sin(angle) * radius * 1.18;
                    if (side === 0) ctx.moveTo(x, dy);
                    else ctx.lineTo(x, dy);
                }
                ctx.closePath();
                ctx.lineWidth = 4;
                ctx.strokeStyle = '#17272c';
                ctx.stroke();
                ctx.fillStyle = lit ? '#194e50' : '#26383c';
                ctx.fill();
                ctx.lineWidth = 1.5;
                ctx.strokeStyle = lit ? '#f8dfa2' : '#96ad9f';
                ctx.stroke();
                glyphShape(i % 4, 0, 0, radius - 4, lit ? '#c3ffec' : '#9ebeb5');
                if (lit) {
                    ctx.beginPath();
                    const fraction = first === 0 ? 1 : clamp(path.timeLeft / path.timeLimit, 0, 1);
                    ctx.arc(0, 0, radius + 7, -Math.PI / 2, -Math.PI / 2 + TAU * fraction);
                    ctx.strokeStyle = fraction < 0.25 ? '#ffd48b' : '#9bfbe3';
                    ctx.lineWidth = 2;
                    ctx.stroke();
                }
                ctx.restore();
            }
            ctx.restore();
        }

        function drawWorld(sim, view) {
            step(sim, view);
            drawBridge(sim, view);
            drawRituals(sim, view);
            drawSwings(sim, view);
            drawLevers(sim, view);
            drawStones(sim, view);
            drawOracles(sim, view);
            drawResonancePath(sim.stage.resonancePath, view);
            const runs = sim.stage.dragonRuns;
            if (runs) for (let i = 0; i < runs.length; i++) drawRunWorld(runs[i], sim, view);
            // fire stays behind the hero so the hero never disappears in the flames
            if (runs) for (let i = 0; i < runs.length; i++) drawRunFire(runs[i], sim, view);
            drawBossFire(sim, view);
        }

        function drawOverlay(sim, view) {
            drawFireballs(sim, view);
            drawFx(view);
        }

        function collectLights(sim, view, lights) {
            const st = sim.stage;
            const path = st.resonancePath;
            if (path && !path.complete) {
                const next = path.runes[path.nextRune];
                lights.push({ x: next.x, y: next.y, radius: 86, intensity: 0.72, flicker: false });
            }
            (st.rituals || []).forEach(function (r) {
                (r.braziers || []).forEach(function (b) {
                    if (b.lit) lights.push({ x: b.x, y: b.y - 50, radius: 140, intensity: 0.75, flicker: true });
                });
            });
            (st.glyphStones || []).forEach(function (g) {
                if (g.lit) lights.push({ x: g.x, y: g.y - 32, radius: 70, intensity: 0.45, flicker: false });
            });
            (st.oracles || []).forEach(function (o) {
                if (o.state === 'showing' || o.state === 'input') lights.push({ x: o.x, y: o.y - 70, radius: 110, intensity: 0.5, flicker: false });
            });
            (st.dragonRuns || []).forEach(function (r) {
                if (r.state !== 'strafe' || !r.fire) return;
                const f = r.fire;
                for (let x = f.x + 30; x < f.x + f.w; x += 100) lights.push({ x: x, y: f.y + f.h / 2, radius: 150, intensity: 0.8, flicker: true });
            });
            (sim.tempHazards || []).forEach(function (th) {
                if (th.kind !== 'dragonfire') return;
                for (let x = th.x + 30; x < th.x + th.w; x += 110) lights.push({ x: x, y: th.y + th.h / 2, radius: 150, intensity: 0.8, flicker: true });
            });
            (sim.projectiles || []).forEach(function (pr) {
                if (!pr.dead && pr.kind === 'fireball') lights.push({ x: pr.x, y: pr.y, radius: 90, intensity: 0.6, flicker: true });
            });
        }

        function onEvent(ev, sim) {
            const d = ev.detail || {};
            const p = sim && sim.player;
            const x = typeof d.x === 'number' ? d.x : (p ? p.x : 0);
            const y = typeof d.y === 'number' ? d.y : (p ? p.y : 0);
            switch (ev.type) {
                case 'dragon_roar':
                case 'dragon_breath':
                case 'dragon_pass':
                    if (h.shake) h.shake(ev.type === 'dragon_roar' ? 6 : 4.5);
                    break;
                case 'seq_solved': ping(x, y, '250,204,21', 80); break;
                case 'seq_fail': ping(x, y, '255,70,70', 60); break;
                case 'seq_hit': ping(x, y, GLYPH_RGB[clamp(d.glyph | 0, 0, 3)], 40); break;
                case 'lever_pull': ping(x, y - 30, '120,230,255', 44); break;
                case 'bridge_toggle': ping(x, y, '120,230,255', 50); break;
                case 'bridge_done': ping(x, y, '250,204,21', 90); break;
                case 'plate_down': ping(x, y, '250,204,21', 60); break;
                case 'brazier_lit': ping(x, y - 40, '255,150,50', 54); break;
                case 'ritual_done': ping(x, y, '250,204,21', 100); break;
                case 'ritual_fail': ping(x, y, '255,70,70', 70); break;
                case 'swing_grab':
                case 'swing_release': ping(x, y, '200,230,255', 34); break;
                default:
            }
        }

        function dispose() {
            embers.length = 0;
            pings.length = 0;
            impacts.length = 0;
            trail.length = 0;
            prevFb.length = 0;
            nextFb.length = 0;
        }

        return {
            drawFar: drawFar,
            drawWorld: drawWorld,
            drawOverlay: drawOverlay,
            drawScreen: drawScreen,
            collectLights: collectLights,
            onEvent: onEvent,
            heat: function () { return heatNow; },
            dispose: dispose
        };
    }

    root.RunenpfadSetPieces = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
