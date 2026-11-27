/* Runenpfad interior renderer for Brandur, the ember wyrm: the body (ground and air), his breath jet, floor warnings (breath cone, tail, stomp marks, vents, landing), fire patches, the lava glow and lights, the sleeper after the fight and the boss bar. Canvas only, read-only on the sim, every sprite has a primitive fallback, never throws. */
(function (root) {
    'use strict';

    // ---- tuning (sizes, offsets, colours: adjust here) ----
    const CELL = 48;
    const BODY_SCALE = 300 / 272; // atlas px -> room px: the side view (refH 272) reads about 300 px tall
    const SHEET_K = { 'td-boss-brandur': 1, 'td-brandur-b': 0.9, 'td-brandur-air': 0.95 }; // the front view and the air view hold more height per cell
    const AIR_BOOST = 1.1;
    const FOOT_DY = 30; // feet pivot = (m.x, m.y + 30)
    const ALT_PX = 140; // air height at alt 1
    const BREATH_LEN = 260;
    const BREATH_CONTENT_W = 487; // content width of a loop frame of td-fx-breath
    const BREATH_CONTENT_H = 209;
    const BREATH_HALF = 0.436;
    const DROWSY_T = 5;
    const BAR_W = 380;
    const BAR_H = 12;
    const BAR_GAP = 8;
    const PHASE_COLOR = { 1: '#d9892f', 2: '#e0603c', 3: '#c0262c' };
    const WARN = '255, 84, 64';
    const WARN_EDGE = '255, 170, 140';
    const SHEETS = ['td-boss-brandur', 'td-brandur-b', 'td-brandur-air', 'td-fx-breath', 'td-fx-hazards', 'tdp-lair'];
    const GROUND_CLIP = {
        sleep: 'td-brandur-b.asleep', move: 'td-boss-brandur.walk', drowsy: 'td-brandur-b.drowsy', dead: 'td-boss-brandur.down'
    };
    const MOVE_CLIP = {
        breath: ['td-boss-brandur.breath_tell', 'td-boss-brandur.breath'],
        tail: ['td-brandur-b.tail_tell', 'td-brandur-b.tail'],
        stomp: ['td-brandur-b.stomp_tell', 'td-brandur-b.stomp'],
        roar: ['td-brandur-b.roar', 'td-brandur-b.roar'],
        vents: ['td-brandur-b.roar', 'td-brandur-b.roar'],
        buffet: ['td-brandur-b.buffet', 'td-brandur-b.buffet'],
        takeoff: ['td-brandur-air.takeoff', 'td-brandur-air.takeoff']
    };

    function Art() { return root.RunenpfadArt || null; }
    function clamp(v, lo, hi) { return v < lo ? lo : v > hi ? hi : v; }
    function list(v) { return Array.isArray(v) ? v : []; }
    function num(v, d) { return typeof v === 'number' && isFinite(v) ? v : d; }
    function owns(m) { return !!m && m.kind === 'brandur'; }
    function sheetOf(clip) { return String(clip).split('.')[0]; }

    function create(h) {
        const ctx = h.ctx;
        const lavaCache = new WeakMap(); // room -> { cells: [[cx, cy]], lights: [{x, y}] }

        function hasClip(clip) {
            const A = Art();
            return !!(A && h.art && A.has(h.art, clip));
        }

        function tr(key, fallback) {
            const f = h.t;
            if (typeof f !== 'function' || !key) return fallback;
            const w = f(key);
            return w && w !== key ? w : fallback;
        }

        function fontFor(px) {
            const fam = h.font || '"Segoe UI", sans-serif';
            return /Press Start/.test(fam) ? Math.max(7, Math.round(px * 0.6)) + 'px ' + fam : 'bold ' + px + 'px ' + fam;
        }

        function textAt(str, x, y, px, color) {
            ctx.save();
            ctx.font = fontFor(px);
            ctx.textAlign = 'center';
            ctx.lineJoin = 'round';
            ctx.lineWidth = 3;
            ctx.strokeStyle = '#09111a';
            ctx.strokeText(str, x, y);
            ctx.fillStyle = color || '#fff6dc';
            ctx.fillText(str, x, y);
            ctx.restore();
        }

        function glow(x, y, r, rgb, alpha) {
            if (!(alpha > 0.01) || typeof ctx.createRadialGradient !== 'function') return;
            const g = ctx.createRadialGradient(x, y, 0, x, y, r);
            g.addColorStop(0, 'rgba(' + rgb + ', ' + clamp(alpha, 0, 1).toFixed(3) + ')');
            g.addColorStop(1, 'rgba(' + rgb + ', 0)');
            ctx.save();
            ctx.globalCompositeOperation = 'lighter';
            ctx.fillStyle = g;
            ctx.fillRect(x - r, y - r, r * 2, r * 2);
            ctx.restore();
        }

        function shadow(x, y, w, alpha) {
            ctx.save();
            ctx.fillStyle = 'rgba(4, 6, 12, ' + (alpha == null ? 0.34 : alpha) + ')';
            ctx.beginPath();
            ctx.ellipse(x, y - 1, w, Math.max(4, w * 0.36), 0, 0, Math.PI * 2);
            ctx.fill();
            ctx.restore();
        }

        function pulse(f, speed, lo) {
            return f.reduced ? 0.8 : lo + (1 - lo) * (0.5 + 0.5 * Math.sin(f.t * speed));
        }

        // Sprite with a clip-specific scale; the dragon art is painted, so it stays smooth.
        function sprite(clip, time, x, y, scale, f, o) {
            const A = Art();
            if (!A || !h.art || !A.has(h.art, clip)) return false;
            const opt = o || {};
            opt.scale = scale;
            opt.pixel = false;
            if (f && f.reduced && opt.frame == null) opt.frame = 0;
            return !!A.draw(ctx, h.art, clip, time, x, y, opt);
        }

        function bodyScale(clip) { return BODY_SCALE * (SHEET_K[sheetOf(clip)] || 1); }

        // ---- sheets ----

        function hasBrandur(iv) {
            if (!iv) return false;
            if (iv.lair) return true;
            if (iv.room && iv.room.def && iv.room.def.guardian === 'brandur') return true;
            return list(iv.mobs).some(owns);
        }

        function sheets(iv) {
            const out = hasBrandur(iv) ? SHEETS.slice() : [];
            if (iv && iv.room && iv.room.lava) out.push('tex-glut');
            return out;
        }

        // ---- lava ----

        function lavaOf(room) {
            if (!room || !room.lava) return null;
            let c = lavaCache.get(room);
            if (c) return c;
            c = { cells: [], lights: [] };
            const cols = room.cols | 0;
            const rows = room.rows | 0;
            const at = function (cx, cy) { return cx >= 0 && cy >= 0 && cx < cols && cy < rows && room.lava[cy * cols + cx] ? 1 : 0; };
            for (let cy = 0; cy < rows; cy++) for (let cx = 0; cx < cols; cx++) if (at(cx, cy)) c.cells.push([cx, cy]);
            for (let cy = 0; cy < rows; cy += 2) {
                for (let cx = 0; cx < cols; cx += 2) {
                    if (at(cx, cy) || at(cx + 1, cy) || at(cx, cy + 1) || at(cx + 1, cy + 1)) c.lights.push({ x: (cx + 1) * CELL, y: (cy + 1) * CELL });
                }
            }
            lavaCache.set(room, c);
            return c;
        }

        function drawLava(iv, f) {
            const lv = lavaOf(iv.room);
            if (!lv) return;
            ctx.save();
            ctx.globalCompositeOperation = 'lighter';
            for (let i = 0; i < lv.cells.length; i++) {
                const c = lv.cells[i];
                const p = f.reduced ? 0.5 : 0.5 + 0.5 * Math.sin(f.t * 1.7 + c[0] * 0.9 + c[1] * 1.3);
                ctx.fillStyle = 'rgba(255, 110, 30, ' + (0.1 + 0.12 * p).toFixed(3) + ')';
                ctx.fillRect(c[0] * CELL, c[1] * CELL, CELL, CELL);
            }
            ctx.restore();
        }

        // ---- floor warnings ----

        function warnStyle(f, k) {
            const p = f.reduced ? 0.8 : 0.55 + 0.45 * Math.sin(f.t * 22);
            const prog = clamp(k == null ? 0 : k, 0, 1);
            return {
                fill: 'rgba(' + WARN + ', ' + (0.2 + 0.14 * p + 0.2 * prog).toFixed(3) + ')',
                edge: 'rgba(' + WARN_EDGE + ', ' + (0.7 + 0.3 * p).toFixed(3) + ')'
            };
        }

        function wedge(x, y, a0, a1, len, st, lw) {
            ctx.beginPath();
            ctx.moveTo(x, y);
            ctx.arc(x, y, len, a0, a1);
            ctx.closePath();
            ctx.fillStyle = st.fill;
            ctx.fill();
            ctx.strokeStyle = st.edge;
            ctx.lineWidth = lw || 3;
            ctx.lineJoin = 'round';
            ctx.stroke();
        }

        function disc(x, y, r, st, lw) {
            ctx.beginPath();
            ctx.arc(x, y, Math.max(1, r), 0, Math.PI * 2);
            ctx.fillStyle = st.fill;
            ctx.fill();
            ctx.strokeStyle = st.edge;
            ctx.lineWidth = lw || 3;
            ctx.stroke();
        }

        function drawMark(mk, f) {
            const at = Math.max(0.05, num(mk.at, 1));
            const t = num(mk.t, 0);
            const x = mk.x;
            const y = mk.y;
            const r = Math.max(8, num(mk.r, 26));
            const rain = mk.kind === 'rain';
            if (mk.done) {
                const age = t - at;
                if (age < 0.3 && age >= 0) {
                    const k = 1 - age / 0.3;
                    ctx.save();
                    ctx.globalAlpha = k;
                    if (!rain && !sprite('td-fx-hazards.shards', 0, x, y - 10, 0.6, f)) disc(x, y, r * 0.8, { fill: 'rgba(120, 100, 90, 0.5)', edge: 'rgba(60, 50, 44, 0.8)' }, 2);
                    if (rain) glow(x, y - 8, r * 2.2, '255, 130, 40', 0.5 * k);
                    ctx.restore();
                }
                return;
            }
            const k = clamp(t / at, 0, 1);
            const st = warnStyle(f, k);
            if (rain) st.fill = 'rgba(255, 120, 30, ' + (0.2 + 0.2 * k).toFixed(3) + ')';
            ctx.save();
            disc(x, y, r, st, 3);
            ctx.beginPath(); // shrinking inner circle: it closes in until the hit
            ctx.arc(x, y, Math.max(2, r * (1 - k)), 0, Math.PI * 2);
            ctx.strokeStyle = 'rgba(255, 244, 220, 0.9)';
            ctx.lineWidth = 2;
            ctx.stroke();
            if (hasClip('td-fx-hazards.ring')) sprite('td-fx-hazards.ring', 0, x, y, (r * 2.1) / 202, f, { alpha: 0.55 });
            ctx.restore();
            const rem = at - t;
            if (rem < 0.3 && rem > -0.01) { // the rock (or ember) is on its way down
                const drop = clamp(rem / 0.3, 0, 1);
                if (rain) {
                    glow(x, y - 20 - drop * 170, r * 1.5, '255, 150, 60', 0.8);
                    ctx.save();
                    ctx.fillStyle = '#ffb060';
                    ctx.beginPath();
                    ctx.arc(x, y - 20 - drop * 170, 7, 0, Math.PI * 2);
                    ctx.fill();
                    ctx.restore();
                } else if (!sprite('td-fx-hazards.stalactite', 0, x, y - 4 - drop * 190, 0.75, f)) {
                    ctx.save();
                    ctx.fillStyle = '#7a6a60';
                    ctx.strokeStyle = 'rgba(0,0,0,0.6)';
                    ctx.lineWidth = 2;
                    const yy = y - drop * 190;
                    ctx.beginPath();
                    ctx.moveTo(x - 14, yy - 70);
                    ctx.lineTo(x + 14, yy - 70);
                    ctx.lineTo(x, yy);
                    ctx.closePath();
                    ctx.fill();
                    ctx.stroke();
                    ctx.restore();
                }
            }
        }

        function drawPatch(p, f) {
            const r = Math.max(10, num(p.r, 30));
            const fade = p.left != null && p.left < 1 ? clamp(p.left, 0, 1) : 1;
            ctx.save();
            ctx.globalAlpha = 0.35 + 0.65 * fade;
            disc(p.x, p.y, r, { fill: 'rgba(230, 90, 20, 0.34)', edge: 'rgba(255, 170, 70, 0.85)' }, 2);
            ctx.restore();
            const art = sprite('td-fx-hazards.flames', f.t + p.x * 0.01, p.x, p.y - r * 0.5, (r * 2.4) / 103, f, { alpha: 0.35 + 0.65 * fade });
            glow(p.x, p.y - 8, r * 1.8, '255, 120, 30', (art ? 0.32 : 0.42) * fade);
        }

        function floorRow(room, cy) {
            const out = [];
            const row = room && room.def && room.def.map ? room.def.map[cy] : null;
            if (!row) return out;
            for (let cx = 0; cx < row.length; cx++) {
                const ch = row.charAt(cx);
                if (ch !== '#' && ch !== ' ' && ch !== '~') out.push(cx);
            }
            return out;
        }

        function hatch(x, y, w, hh, a) {
            ctx.save();
            ctx.beginPath();
            ctx.rect(x, y, w, hh);
            ctx.clip();
            ctx.strokeStyle = 'rgba(' + WARN_EDGE + ', ' + a.toFixed(3) + ')';
            ctx.lineWidth = 3;
            ctx.beginPath();
            for (let i = -hh; i < w; i += 12) {
                ctx.moveTo(x + i, y + hh);
                ctx.lineTo(x + i + hh, y);
            }
            ctx.stroke();
            ctx.restore();
        }

        function drawVents(iv, v, f) {
            const t = num(v.t, 0);
            const at = num(v.at, 1);
            const dur = num(v.dur, 0.6);
            const live = t >= at && t < at + dur + 0.2;
            const st = warnStyle(f, t / Math.max(0.1, at));
            list(v.rows).forEach(function (cy) {
                const cells = floorRow(iv.room, cy);
                for (let i = 0; i < cells.length; i++) {
                    const x = cells[i] * CELL;
                    const y = cy * CELL;
                    if (!live) {
                        ctx.fillStyle = st.fill;
                        ctx.fillRect(x, y, CELL, CELL);
                        hatch(x, y, CELL, CELL, 0.75);
                    } else {
                        ctx.fillStyle = 'rgba(255, 130, 40, 0.34)';
                        ctx.fillRect(x, y, CELL, CELL);
                        if (!sprite('td-fx-hazards.vent', 0, x + CELL / 2, y + CELL * 0.8, 0.62, f)) {
                            glow(x + CELL / 2, y + CELL / 2, CELL * 0.9, '255, 150, 60', 0.6);
                        }
                    }
                }
                if (cells.length) {
                    ctx.strokeStyle = st.edge;
                    ctx.lineWidth = 3;
                    ctx.strokeRect(cells[0] * CELL, cy * CELL, (cells[cells.length - 1] - cells[0] + 1) * CELL, CELL);
                }
            });
        }

        function mouthOf(m, br) {
            if (br && isFinite(br.x) && isFinite(br.y)) return { x: br.x, y: br.y };
            return { x: m.x + num(m.ax, 1) * 60, y: m.y - 7 + num(m.ay, 0) * 60 };
        }

        function drawMobFloor(m, f) {
            const dr = m.dr;
            if (!dr || m.state === 'dead') return;
            const warn = m.state === 'tell';
            const live = warn || m.state === 'attack';
            if (!live) return;
            const k = warn ? clamp(num(dr.t, 0) / Math.max(0.1, num(dr.tellDur, 1)), 0, 1) : 1;
            const st = warnStyle(f, k);
            ctx.save();
            if (dr.move === 'breath') {
                const br = dr.breath;
                const mo = mouthOf(m, br);
                const a = br && isFinite(br.a) ? br.a : Math.atan2(num(m.ay, 0), num(m.ax, 1));
                const half = br && br.half > 0 ? br.half : BREATH_HALF;
                const len = br && br.len > 0 ? br.len : BREATH_LEN;
                if (br && isFinite(br.a0) && isFinite(br.a1) && Math.abs(br.a1 - br.a0) > 0.02) {
                    const sweep = { fill: 'rgba(' + WARN + ', ' + (warn ? 0.1 : 0.06) + ')', edge: 'rgba(' + WARN_EDGE + ', 0.45)' };
                    wedge(mo.x, mo.y, Math.min(br.a0, br.a1) - half, Math.max(br.a0, br.a1) + half, len, sweep, 2);
                }
                wedge(mo.x, mo.y, a - half, a + half, len, warn ? st : { fill: 'rgba(' + WARN + ', 0.12)', edge: 'rgba(' + WARN_EDGE + ', 0.5)' }, warn ? 4 : 2);
            } else if (dr.move === 'tail') {
                const tl = dr.tail;
                const a = tl && isFinite(tl.a) ? tl.a : (m.face < 0 ? 0 : Math.PI);
                const r = tl && tl.r > 0 ? tl.r : 150;
                const half = tl && tl.half > 0 ? tl.half : 1.22;
                wedge(tl ? tl.x : m.x, tl ? tl.y : m.y - 7, a - half, a + half, r, st, 4);
            } else if (dr.move === 'stomp' && warn) {
                disc(m.x, m.y + 8, 118, st, 4);
                disc(m.x, m.y + 8, 118 * (1 - k), { fill: 'rgba(0,0,0,0)', edge: 'rgba(255, 244, 220, 0.85)' }, 2);
            } else if (warn && dr.move !== 'vents') {
                disc(m.x, m.y + 8, dr.move === 'buffet' ? 130 : 96, st, 3);
            }
            ctx.restore();
        }

        function drawFloor(iv, sim, f) {
            try {
                drawLava(iv, f);
                const mobs = list(iv.mobs).filter(owns);
                mobs.forEach(function (m) {
                    const dr = m.dr;
                    if (!dr) return;
                    try {
                        if (dr.vents) drawVents(iv, dr.vents, f);
                        list(dr.marks).forEach(function (mk) { try { drawMark(mk, f); } catch (err) { /* ignore */ } });
                        list(dr.patches).forEach(function (p) { try { drawPatch(p, f); } catch (err) { /* ignore */ } });
                        if (dr.shadow) {
                            const hold = num(dr.t, 0) > 2.2;
                            ctx.save();
                            ctx.fillStyle = 'rgba(2, 3, 8, ' + (0.42 + 0.1 * (1 - num(dr.alt, 0))).toFixed(3) + ')';
                            ctx.beginPath();
                            ctx.ellipse(dr.shadow.x, dr.shadow.y, 92, 52, 0, 0, Math.PI * 2);
                            ctx.fill();
                            ctx.strokeStyle = hold ? 'rgba(' + WARN_EDGE + ', ' + (0.7 + 0.3 * pulse(f, 20, 0.3)).toFixed(3) + ')' : 'rgba(' + WARN_EDGE + ', 0.35)';
                            ctx.lineWidth = hold ? 4 : 2;
                            ctx.stroke();
                            ctx.restore();
                        }
                        if (dr.ring && num(dr.ring.max, 150) > 0) {
                            const rg = dr.ring;
                            const k = clamp(num(rg.r, 0) / num(rg.max, 150), 0, 1);
                            ctx.save();
                            ctx.globalAlpha = clamp(1.15 - k * 0.7, 0.2, 1);
                            ctx.beginPath();
                            ctx.arc(rg.x, rg.y, Math.max(2, num(rg.r, 0)), 0, Math.PI * 2);
                            ctx.fillStyle = 'rgba(255, 140, 50, 0.14)';
                            ctx.fill();
                            ctx.strokeStyle = 'rgba(255, 190, 110, 0.95)';
                            ctx.lineWidth = 7;
                            ctx.stroke();
                            ctx.restore();
                            if (hasClip('td-fx-hazards.ring')) sprite('td-fx-hazards.ring', 0, rg.x, rg.y, (num(rg.r, 0) * 2) / 202, f, { alpha: 0.6 });
                        }
                    } catch (err) { /* one bad hazard must not stop the room */ }
                    drawMobFloor(m, f);
                });
            } catch (err) { /* ignore */ }
            void sim;
        }

        // ---- the body ----

        function fbBody(x, y, scale, flash, alpha, air, face) {
            const w = 150 * scale;
            const hh = (air ? 120 : 90) * scale;
            ctx.save();
            ctx.globalAlpha = alpha;
            ctx.lineJoin = 'round';
            ctx.fillStyle = flash > 0.3 ? '#ffffff' : '#7a2a22';
            ctx.strokeStyle = 'rgba(0,0,0,0.65)';
            ctx.lineWidth = 3;
            const cy = air ? y : y - hh * 0.9;
            ctx.beginPath();
            ctx.ellipse(x, cy, w, hh, 0, 0, Math.PI * 2);
            ctx.fill();
            ctx.stroke();
            ctx.fillStyle = flash > 0.3 ? '#ffffff' : '#5a1c18';
            [-1, 1].forEach(function (sd) {
                ctx.beginPath();
                ctx.moveTo(x + sd * w * 0.5, cy - hh * 0.2);
                ctx.lineTo(x + sd * w * 1.7, cy - hh * (air ? 0.3 : 1.1));
                ctx.lineTo(x + sd * w * 1.3, cy + hh * 0.5);
                ctx.closePath();
                ctx.fill();
                ctx.stroke();
            });
            const hx = x + (air ? 0 : (face < 0 ? -1 : 1) * w * 0.95);
            const hy = air ? cy + hh * 1.0 : cy - hh * 0.2;
            ctx.fillStyle = flash > 0.3 ? '#ffffff' : '#8a3226';
            ctx.beginPath();
            ctx.ellipse(hx, hy, w * 0.38, hh * 0.46, 0, 0, Math.PI * 2);
            ctx.fill();
            ctx.stroke();
            ctx.fillStyle = '#ffd27a';
            ctx.fillRect(hx - 6, hy - 4, 4, 4);
            ctx.fillRect(hx + 4, hy - 4, 4, 4);
            ctx.restore();
        }

        function groundClip(m) {
            const dr = m.dr || {};
            if (m.state === 'tell' || m.state === 'attack') {
                const mv = MOVE_CLIP[dr.move];
                if (mv) return mv[m.state === 'tell' ? 0 : 1];
                return GROUND_CLIP.move;
            }
            if (m.state === 'move' && num(m.hurtT, 0) > 0.3) return 'td-boss-brandur.hurt';
            return GROUND_CLIP[m.state] || GROUND_CLIP.move;
        }

        function drawZ(x, y, f) {
            ctx.save();
            ctx.fillStyle = '#9fe8ff';
            ctx.strokeStyle = '#09111a';
            ctx.lineWidth = 3;
            ctx.textAlign = 'center';
            for (let i = 0; i < 3; i++) {
                const ph = f.reduced ? i * 0.3 : ((f.t * 0.5 + i / 3) % 1);
                const px = 16 + i * 6;
                ctx.font = fontFor(px);
                ctx.globalAlpha = 1 - ph * 0.7;
                ctx.strokeText('Z', x + i * 14 + ph * 10, y - ph * 46);
                ctx.fillText('Z', x + i * 14 + ph * 10, y - ph * 46);
            }
            ctx.restore();
        }

        // One dragon on the ground: pivot at the feet.
        function drawBody(clip, x, y, face, f, o) {
            const sc = bodyScale(clip);
            const opt = o || {};
            opt.flip = face < 0;
            if (!sprite(clip, opt.time || 0, x, y, sc, f, opt)) {
                fbBody(x, y, sc, opt.flash || 0, opt.alpha == null ? 1 : opt.alpha, false, face);
            }
        }

        function drawMobObject(m, iv, f) {
            const dr = m.dr || {};
            const x = m.x;
            const y = m.y + FOOT_DY;
            if (m.state === 'air') return;
            if (dr.ring && num(dr.ring.t, 9) < 0.3 && m.state !== 'dead') return; // the landing pose is drawn on top (drawTop)
            const flash = num(m.hurtT, 0) > 0 && m.state !== 'dead' ? Math.min(0.85, m.hurtT * 6) : 0;
            let alpha = 1;
            if (m.state === 'dead') alpha = clamp(1 - (num(m.deadT, 0) - 0.3) / 0.3, 0, 1);
            if (alpha <= 0.01) return;
            shadow(x, y, 100, 0.34 * alpha);
            const clip = groundClip(m);
            const time = m.state === 'move' ? f.t + m.x * 0.01 : num(dr.t, 0);
            if (sheetOf(clip) === 'td-brandur-air') { // the takeoff pose: the air sheet pivots at the centre of the body
                const sc = bodyScale(clip) * AIR_BOOST;
                const ay = y - 37 - clamp(num(dr.alt, 0), 0, 1) * ALT_PX;
                if (!sprite(clip, 0, x, ay, sc, f, { flash: flash, alpha: alpha, flip: m.face < 0 })) fbBody(x, ay, sc, flash, alpha, true, m.face);
            } else drawBody(clip, x, y, m.face, f, { flash: flash, alpha: alpha, time: time, frame: m.state === 'move' ? undefined : 0 });
            if (m.state === 'drowsy') drawZ(x + (m.face < 0 ? -1 : 1) * 70, y - 250, f);
            if (m.state === 'tell' && dr.move === 'stomp' && !f.reduced) glow(x, y - 20, 90, WARN, 0.1);
            void iv;
        }

        function drawLairObject(lair, f) {
            const x = lair.x;
            const y = lair.y + FOOT_DY;
            const slain = lair.fate === 'slain';
            shadow(x, y, 100, 0.34);
            if (lair.fate === 'sealed') glow(x, y - 130, 230, '110, 170, 255', 0.2 * pulse(f, 1.6, 0.7));
            drawBody(slain ? 'td-boss-brandur.down' : 'td-brandur-b.asleep', x, y, lair.face, f, slain ? { tint: '#10121c', tintAmount: 0.45 } : { time: f.t });
            if (lair.fate === 'sealed') glow(x, y - 150, 120, '140, 190, 255', 0.1);
        }

        // Entries { y, o } for the y-sorted list: Brandur on the ground and the sleeper.
        function sorted(iv) {
            const out = [];
            try {
                const mobs = list(iv.mobs).filter(owns);
                mobs.forEach(function (m) { if (m.state !== 'air') out.push({ y: m.y, o: { mob: m } }); });
                const lair = iv.lair;
                if (lair && lair.fate && lair.fate !== 'none' && !mobs.some(function (m) { return m.state === 'dead'; })) out.push({ y: num(lair.y, 0), o: { lair: lair } });
            } catch (err) { /* ignore */ }
            return out;
        }

        function drawObject(o, iv, sim, f) {
            try {
                if (o.mob) drawMobObject(o.mob, iv, f);
                else if (o.lair) drawLairObject(o.lair, f);
            } catch (err) { /* ignore */ }
            void sim;
        }

        // ---- top layer: breath jet and the airborne body ----

        function drawJet(br, f) {
            const A = Art();
            const dur = Math.max(0.2, num(br.dur, 1.2));
            const t = num(br.t, 0);
            const len = br.len > 0 ? br.len : BREATH_LEN;
            const half = br.half > 0 ? br.half : BREATH_HALF;
            let clip = 'td-fx-breath.loop';
            let time = t;
            if (t < 0.12) { clip = 'td-fx-breath.ignite'; time = t; } else if (t > dur - 0.16) { clip = 'td-fx-breath.fade'; time = t - (dur - 0.16); }
            const sx = len / BREATH_CONTENT_W;
            const sy = (2 * len * Math.tan(25 * Math.PI / 180) * 0.9) / BREATH_CONTENT_H;
            ctx.save();
            ctx.translate(br.x, br.y);
            ctx.rotate(num(br.a, 0));
            ctx.scale(sx, sy);
            let ok = false;
            if (A && h.art && A.has(h.art, clip)) {
                ctx.globalCompositeOperation = 'lighter';
                ok = !!A.draw(ctx, h.art, clip, time, 0, 0, { scale: 1, pixel: false, frame: f.reduced && clip === 'td-fx-breath.loop' ? 1 : undefined });
            }
            ctx.restore();
            if (!ok) { // polygon cone
                ctx.save();
                ctx.translate(br.x, br.y);
                ctx.rotate(num(br.a, 0));
                const g = ctx.createLinearGradient(0, 0, len, 0);
                g.addColorStop(0, 'rgba(255, 240, 160, 0.95)');
                g.addColorStop(0.5, 'rgba(255, 150, 40, 0.75)');
                g.addColorStop(1, 'rgba(220, 60, 20, 0.15)');
                ctx.fillStyle = g;
                ctx.beginPath();
                ctx.moveTo(0, 0);
                ctx.lineTo(len, -Math.tan(half) * len * 0.8);
                ctx.lineTo(len, Math.tan(half) * len * 0.8);
                ctx.closePath();
                ctx.fill();
                ctx.restore();
            }
            glow(br.x + Math.cos(num(br.a, 0)) * 40, br.y + Math.sin(num(br.a, 0)) * 40, 70, '255, 200, 90', 0.4);
        }

        function airClip(m) {
            const dr = m.dr || {};
            if (num(dr.t, 0) >= 2.2) return 'td-brandur-air.dive';
            return num(dr.alt, 0) > 0.85 ? 'td-brandur-air.glide' : 'td-brandur-air.fly';
        }

        function drawAir(m, f) {
            const dr = m.dr || {};
            const alt = clamp(num(dr.alt, 1), 0, 1);
            const base = dr.shadow || { x: m.x, y: m.y + FOOT_DY };
            const x = base.x;
            const y = base.y - 37 - alt * ALT_PX;
            const clip = airClip(m);
            const sc = bodyScale(clip) * AIR_BOOST;
            const flash = num(m.hurtT, 0) > 0 ? Math.min(0.85, m.hurtT * 6) : 0;
            if (!sprite(clip, f.t, x, y, sc, f, { flash: flash })) fbBody(x, y, sc, flash, 1, true, m.face);
        }

        function drawTop(iv, sim, f) {
            try {
                list(iv.mobs).filter(owns).forEach(function (m) {
                    const dr = m.dr;
                    if (!dr) return;
                    const br = dr.breath;
                    if (br && br.live) { try { drawJet(br, f); } catch (err) { /* ignore */ } }
                    try {
                        if (m.state === 'air') drawAir(m, f);
                        else if (dr.ring && num(dr.ring.t, 9) < 0.3 && m.state !== 'dead') {
                            const sc = bodyScale('td-brandur-air.land') * AIR_BOOST;
                            const y = m.y + FOOT_DY - 37;
                            if (!sprite('td-brandur-air.land', 0, m.x, y, sc, f)) fbBody(m.x, y, sc, 0, 1, true, m.face);
                        }
                    } catch (err) { /* ignore */ }
                });
            } catch (err) { /* ignore */ }
            void sim;
        }

        // ---- lights ----

        function lights(iv) {
            const out = [];
            try {
                const lv = lavaOf(iv.room);
                if (lv) lv.lights.forEach(function (l) { out.push({ x: l.x, y: l.y, r: 110, color: '#ff7a2a', flicker: true }); });
                list(iv.mobs).filter(owns).forEach(function (m) {
                    const dr = m.dr;
                    if (!dr) return;
                    const br = dr.breath;
                    if (br && br.live) out.push({ x: br.x + Math.cos(num(br.a, 0)) * 120, y: br.y + Math.sin(num(br.a, 0)) * 120, r: 260, color: '#ffb060', flicker: true });
                    list(dr.patches).forEach(function (p) { out.push({ x: p.x, y: p.y, r: 70, color: '#ff7a2a', flicker: true }); });
                });
            } catch (err) { /* ignore */ }
            return out;
        }

        // ---- boss bar (screen space) ----

        function pillarsLit(iv, m) {
            if (m && m.dr && typeof m.dr.pillars === 'number') return clamp(m.dr.pillars | 0, 0, 4);
            return list(iv.things).filter(function (t) { return t && t.kind === 'brazier' && /^pillar/.test(String(t.bid || t.id || '')) && t.lit; }).length;
        }

        function drawBossBar(iv, f, vw, inset) {
            try {
                const g = iv && iv.guardian && iv.guardian.kind === 'brandur' && iv.guardian.state !== 'dead' ? iv.guardian : null;
                if (!g) return false;
                const w = Math.min(BAR_W, vw * 0.7);
                const x = (vw - w) / 2;
                const y = inset + BAR_GAP + 16;
                const maxHp = Math.max(1, num(g.maxHp, num(g.hp, 1)));
                const frac = clamp(num(g.hp, maxHp) / maxHp, 0, 1);
                const phase = g.dr && g.dr.phase ? g.dr.phase : (g.phase || 1);
                textAt(tr('runenpfad.inboss.brandur', 'Brandur'), vw / 2, y - 5, 14, '#ffe9c0');
                ctx.save();
                ctx.fillStyle = 'rgba(8, 6, 14, 0.82)';
                ctx.strokeStyle = 'rgba(244, 200, 116, 0.9)';
                ctx.lineWidth = 2;
                ctx.fillRect(x - 2, y - 1, w + 4, BAR_H + 2);
                ctx.strokeRect(x - 2, y - 1, w + 4, BAR_H + 2);
                if (frac > 0) {
                    ctx.fillStyle = PHASE_COLOR[phase] || PHASE_COLOR[1];
                    ctx.fillRect(x, y + 1, Math.max(3, w * frac), BAR_H - 2);
                }
                ctx.fillStyle = 'rgba(255, 246, 220, 0.85)';
                ctx.fillRect(x + w * (2 / 3) - 1, y, 2, BAR_H);
                ctx.fillRect(x + w * (1 / 3) - 1, y, 2, BAR_H);
                const lit = pillarsLit(iv, g);
                for (let i = 0; i < 4; i++) {
                    const px = vw / 2 + (i - 1.5) * 20;
                    ctx.beginPath();
                    ctx.arc(px, y + BAR_H + 12, 6, 0, Math.PI * 2);
                    ctx.fillStyle = i < lit ? '#7ec8ff' : 'rgba(40, 46, 66, 0.9)';
                    ctx.fill();
                    ctx.strokeStyle = i < lit ? '#e6f6ff' : 'rgba(150, 160, 190, 0.7)';
                    ctx.lineWidth = 2;
                    ctx.stroke();
                }
                if (g.state === 'drowsy') {
                    const left = clamp(num(g.dr && g.dr.drowsy, 0) / DROWSY_T, 0, 1);
                    const by = y + BAR_H + 24;
                    ctx.fillStyle = 'rgba(8, 6, 14, 0.82)';
                    ctx.fillRect(x + w * 0.2, by, w * 0.6, 7);
                    ctx.fillStyle = '#9fe8ff';
                    ctx.fillRect(x + w * 0.2 + 1, by + 1, Math.max(2, (w * 0.6 - 2) * left), 5);
                    ctx.strokeStyle = 'rgba(159, 232, 255, 0.9)';
                    ctx.lineWidth = 1.5;
                    ctx.strokeRect(x + w * 0.2, by, w * 0.6, 7);
                }
                ctx.restore();
                return true;
            } catch (err) { return false; }
            void f;
        }

        function dispose() { /* only a WeakMap of derived lava cells */ }

        return { owns: owns, sheets: sheets, drawFloor: drawFloor, sorted: sorted, drawObject: drawObject, drawTop: drawTop, lights: lights, drawBossBar: drawBossBar, dispose: dispose };
    }

    root.RunenpfadBrandurView = { create: create, owns: owns };
})(typeof window !== 'undefined' ? window : globalThis);
