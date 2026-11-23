/* Runenpfad interior things, guardians and prompts, canvas side: pots, blocks, plates, levers, gates, traps, braziers, cracks, lecterns, signs, board, dice table, core gear, guardian drawing, boss bar and prompt icons. Read-only on the sim, every sprite has a primitive fallback, never throws. */
(function (root) {
    'use strict';

    // ---- tuning (sizes, offsets, colours: adjust here) ----
    const CELL = 48;
    const FOOT = 7; // things stand at the cell centre + 7 px, like chests
    const PROP_SHEETS = ['tdp-mech', 'tdp-lore', 'tdp-stars'];
    const BOSS_IDS = ['brood', 'weaver', 'tombward', 'frostheart', 'boilerward', 'counter'];
    const BOSS_SCALE = 1.5; // guardian height = normal mob height (34..84) times this
    const BOSS_COLOR = { brood: '#7bbf5a', weaver: '#7a4fa0', tombward: '#c9a85a', frostheart: '#8ad8ff', boilerward: '#c06a30', counter: '#d8b24a' };
    const BAR_W = 340;
    const BAR_H = 12;
    const BAR_GAP = 8; // below the top-bar inset
    const COL_WARN = '255, 96, 80';
    const COL_FLAME = 'rgba(255, 176, 96, 0.9)';
    const COL_SEAL = 'rgba(255, 210, 120, 0.9)';
    const SHOT_COLOR = { dart: '#e8e0cc', weaver_venom: '#8df08a', frostheart_bolt: '#9ad8ff', frostheart_shard: '#bfe9ff', counter_clock: '#ffd27a', counter_beam: '#ffb347' };
    const NEW_KEYS = ['pot', 'block', 'plate', 'lever', 'gate', 'spikes', 'dart', 'brazier', 'crack', 'page', 'sign', 'use'];
    const POT_CLIP = { pot: 'tdp-mech.p0', crate: 'tdp-mech.p1', barrel: 'tdp-mech.p2' };
    const BLOCK_CLIP = { crate: 'tdp-mech.p1', stone: 'tdp-mech.p4', crystal: 'tdp-mech.p5', ice: 'tdp-stars.p13' };
    const BLOCK_COLOR = { crate: '#9a6a30', stone: '#8a8f99', crystal: '#7fd0e8', ice: '#b8e4f4' };
    const USE_SIZE = { board: [2, 1], dice: [2, 1], core: [2, 2] };
    const PUSH_TIME = 0.18;
    const LAMP_SCALE = 0.8; // storm lamps (brazier in wood rooms) read as table-height

    function Art() { return root.RunenpfadArt || null; }
    function Bake() { return root.RunenpfadInteriorBake || null; }
    function clamp(v, lo, hi) { return v < lo ? lo : v > hi ? hi : v; }
    function list(v) { return Array.isArray(v) ? v : []; }
    function num(v, d) { return typeof v === 'number' && isFinite(v) ? v : d; }
    function isBoss(m) { return !!m && BOSS_IDS.indexOf(m.kind) >= 0; }

    function create(h) {
        const ctx = h.ctx;

        // ---- small helpers (own copies: the module stands on its own) ----

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

        function shadow(x, y, w, alpha) {
            ctx.save();
            ctx.fillStyle = 'rgba(4, 6, 12, ' + (alpha == null ? 0.3 : alpha) + ')';
            ctx.beginPath();
            ctx.ellipse(x, y - 1, w, Math.max(4, w * 0.36), 0, 0, Math.PI * 2);
            ctx.fill();
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

        function rbox(x, y, w, hh, r, fill, stroke) {
            ctx.beginPath();
            const B = Bake();
            if (B) B.roundRect(ctx, x, y, w, hh, r); else ctx.rect(x, y, w, hh);
            if (fill) { ctx.fillStyle = fill; ctx.fill(); }
            if (stroke) { ctx.strokeStyle = stroke; ctx.lineWidth = 2; ctx.stroke(); }
        }

        function textAt(str, x, y, px, color, align) {
            ctx.save();
            ctx.font = fontFor(px);
            ctx.textAlign = align || 'center';
            ctx.lineJoin = 'round';
            ctx.lineWidth = 3;
            ctx.strokeStyle = '#09111a';
            ctx.strokeText(str, x, y);
            ctx.fillStyle = color || '#fff6dc';
            ctx.fillText(str, x, y);
            ctx.restore();
        }

        // Atlas prop with its feet at (x, y), width from the footprint; false while the sheet is missing.
        function sprite(clip, x, y, wCells, hCells, f, o) {
            const B = Bake();
            if (!B || !hasClip(clip)) return false;
            const opt = o || {};
            opt.screen = f.s;
            return !!B.drawProp(ctx, h.art, { clip: clip, x: x, y: y, wCells: wCells || 1, hCells: hCells || 1, layer: 'obj' }, opt);
        }

        function pulse(f, speed, lo) {
            return f.reduced ? 0.8 : lo + (1 - lo) * (0.5 + 0.5 * Math.sin(f.t * speed));
        }

        function themeOf(iv) {
            return iv && iv.room && iv.room.def ? iv.room.def.theme : '';
        }

        function flagOf(sim, key) {
            const N = root.RunenpfadNpcs;
            try { return !!(N && typeof N.getFlag === 'function' && N.getFlag(sim, key)); } catch (err) { return false; }
        }

        // ---- sheet requests ----

        function sheetsOfDef(def) {
            const ids = [];
            if (!def) return ids;
            const things = list(def.things);
            let uses = false;
            for (let i = 0; i < things.length && !uses; i++) {
                for (let k = 0; k < NEW_KEYS.length; k++) if (things[i] && things[i][NEW_KEYS[k]] != null) { uses = true; break; }
            }
            if (uses) PROP_SHEETS.forEach(function (s) { ids.push(s); });
            if (def.guardian) ids.push('td-boss-' + def.guardian);
            things.forEach(function (t) { if (t && t.mob && BOSS_IDS.indexOf(t.mob) >= 0) ids.push('td-boss-' + t.mob); });
            return ids;
        }

        function mobSheet(kind) { return (BOSS_IDS.indexOf(kind) >= 0 ? 'td-boss-' : 'td-') + kind; }

        // ---- thing geometry ----

        function posOf(th) { return { x: num(th.x, (num(th.cx, 0) + 0.5) * CELL), y: num(th.y, (num(th.cy, 0) + 0.5) * CELL + FOOT) }; }

        // The engine interpolates a moving block itself: x, y are the current position.
        function blockPos(th) { return posOf(th); }

        function brazierClip(th, iv) {
            const wood = themeOf(iv) === 'wood';
            return th.lit ? (wood ? 'tdp-lore.p14' : 'tdp-mech.p15') : (wood ? 'tdp-lore.p13' : 'tdp-mech.p14');
        }

        // ---- fallbacks ----

        function fbThing(th, x, y, f, iv) {
            ctx.save();
            ctx.lineJoin = 'round';
            const kind = th.kind;
            if (kind === 'pot') {
                const crate = th.look === 'crate';
                rbox(x - 15, y - 28, 30, 28, crate ? 3 : 12, crate ? '#9a6a30' : th.look === 'barrel' ? '#7a5030' : '#b0643c', 'rgba(0,0,0,0.6)');
            } else if (kind === 'block') {
                const lk = th.look || th.block || 'stone';
                rbox(x - 20, y - 36, 40, 38, lk === 'stone' || lk === 'crate' ? 4 : 9, BLOCK_COLOR[lk] || BLOCK_COLOR.stone, 'rgba(0,0,0,0.6)');
                ctx.fillStyle = 'rgba(255,255,255,0.22)';
                ctx.fillRect(x - 16, y - 33, 32, 6);
            } else if (kind === 'lever') {
                rbox(x - 11, y - 12, 22, 12, 3, '#6a6a72', 'rgba(0,0,0,0.6)');
                ctx.strokeStyle = th.on ? '#ffd27a' : '#c9ced8';
                ctx.lineWidth = 5;
                ctx.lineCap = 'round';
                ctx.beginPath();
                ctx.moveTo(x, y - 10);
                ctx.lineTo(x + (th.on ? 14 : -14), y - 32);
                ctx.stroke();
            } else if (kind === 'gate') {
                const k = clamp(num(th.k, th.open ? 1 : 0), 0, 1);
                ctx.fillStyle = '#4a4e58';
                ctx.strokeStyle = 'rgba(0,0,0,0.6)';
                ctx.lineWidth = 2;
                for (let i = 0; i < 4; i++) {
                    const bx = x - 19 + i * 12.6;
                    ctx.fillRect(bx, y - 40 + k * 36, 6, 40 - k * 36 + 2);
                }
            } else if (kind === 'spikes') {
                for (let i = 0; i < 3; i++) {
                    ctx.fillStyle = '#c9ced8';
                    ctx.beginPath();
                    ctx.moveTo(x - 16 + i * 14, y);
                    ctx.lineTo(x - 10 + i * 14, y - 22);
                    ctx.lineTo(x - 4 + i * 14, y);
                    ctx.closePath();
                    ctx.fill();
                }
            } else if (kind === 'dart') {
                rbox(x - 17, y - 30, 34, 30, 4, '#5a5660', 'rgba(0,0,0,0.6)');
                ctx.fillStyle = '#16121a';
                const d = th.dir;
                ctx.fillRect(d === 'left' ? x - 17 : d === 'right' ? x + 9 : x - 5, d === 'up' ? y - 30 : d === 'down' ? y - 8 : y - 20, d === 'up' || d === 'down' ? 10 : 8, d === 'up' || d === 'down' ? 8 : 10);
            } else if (kind === 'brazier') {
                rbox(x - 14, y - 20, 28, 20, 5, '#3a3640', 'rgba(0,0,0,0.6)');
                if (th.lit) {
                    ctx.fillStyle = '#ffb060';
                    ctx.beginPath();
                    ctx.moveTo(x - 9, y - 20);
                    ctx.quadraticCurveTo(x - 4, y - 38, x, y - 46);
                    ctx.quadraticCurveTo(x + 5, y - 36, x + 9, y - 20);
                    ctx.closePath();
                    ctx.fill();
                }
            } else if (kind === 'crack') {
                rbox(x - 22, y - 44, 44, 46, 3, '#5a4a3c', 'rgba(0,0,0,0.6)');
                ctx.strokeStyle = '#1a120c';
                ctx.lineWidth = 2;
                ctx.beginPath();
                ctx.moveTo(x - 4, y - 44);
                ctx.lineTo(x + 2, y - 28);
                ctx.lineTo(x - 6, y - 16);
                ctx.lineTo(x + 3, y);
                ctx.stroke();
            } else if (kind === 'page') {
                rbox(x - 7, y - 32, 14, 32, 2, '#6a4a2c', 'rgba(0,0,0,0.6)');
                rbox(x - 17, y - 42, 34, 12, 3, th.have ? '#b8a888' : '#f4e6b8', 'rgba(0,0,0,0.6)');
            } else if (kind === 'sign' && th.look === 'scroll') {
                ctx.save();
                ctx.translate(x, y - 8);
                ctx.rotate(-0.12);
                rbox(-16, -10, 32, 20, 3, '#ecdcae', 'rgba(0,0,0,0.6)');
                ctx.fillStyle = '#7a5a30';
                ctx.fillRect(-11, -5, 22, 2);
                ctx.fillRect(-11, 0, 16, 2);
                ctx.restore();
            } else if (kind === 'sign') {
                rbox(x - 15, y - 38, 30, 28, 3, '#8a7a5a', 'rgba(0,0,0,0.6)');
                ctx.fillStyle = '#2a1d10';
                ctx.fillRect(x - 9, y - 31, 18, 2);
                ctx.fillRect(x - 9, y - 25, 14, 2);
                ctx.fillRect(x - 3, y - 10, 6, 10);
            } else if (kind === 'plate') {
                rbox(x - 18, y - 22, 36, 28, 4, th.on ? '#6e7a58' : '#7a7568', 'rgba(0,0,0,0.5)');
            } else {
                rbox(x - 14, y - 24, 28, 24, 5, '#7a6040', 'rgba(0,0,0,0.55)');
            }
            ctx.restore();
            void f;
            void iv;
        }

        function fbUse(g, f, sim) {
            const w = g.w * CELL * 0.9;
            const hh = g.h * CELL * 0.62;
            ctx.save();
            if (g.use === 'core') {
                const done = flagOf(sim, 'obs.done');
                ctx.translate(g.x, g.y - hh * 0.5 - 6);
                ctx.fillStyle = done ? '#7a7066' : '#d8a040';
                ctx.strokeStyle = 'rgba(0,0,0,0.65)';
                ctx.lineWidth = 3;
                const rot = done || f.reduced ? 0 : f.t * 0.6;
                ctx.beginPath();
                for (let i = 0; i < 24; i++) {
                    const a = rot + i * Math.PI / 12;
                    const rr = i % 2 ? 30 : 40;
                    ctx.lineTo(Math.cos(a) * rr, Math.sin(a) * rr);
                }
                ctx.closePath();
                ctx.fill();
                ctx.stroke();
            } else {
                rbox(g.x - w / 2, g.y - hh, w, hh, 5, g.use === 'board' ? '#8a6a3c' : '#2f6a48', 'rgba(0,0,0,0.65)');
                ctx.fillStyle = g.use === 'board' ? '#f2e6c0' : '#f4f4f0';
                if (g.use === 'board') { ctx.fillRect(g.x - 28, g.y - hh + 6, 16, 14); ctx.fillRect(g.x - 6, g.y - hh + 8, 16, 14); ctx.fillRect(g.x + 14, g.y - hh + 5, 14, 14); } else { ctx.fillRect(g.x - 16, g.y - hh + 8, 12, 12); ctx.fillRect(g.x + 6, g.y - hh + 10, 12, 12); }
            }
            ctx.restore();
        }

        // ---- floor pass (under the actors) ----

        function floorClip(th, iv) {
            const kind = th.kind;
            if (kind === 'plate') return th.on ? 'tdp-mech.p7' : 'tdp-mech.p6';
            if (kind === 'pot' && th.broken) return 'tdp-mech.p3';
            if (kind === 'crack' && th.broken) return 'tdp-lore.p5';
            if (kind === 'spikes' && th.phase !== 'up') return th.look === 'vent' ? 'tdp-lore.p7' : 'tdp-mech.p12';
            if (kind === 'gate' && (th.open || num(th.k, 0) > 0.02)) return 'tdp-mech.p11';
            void iv;
            return '';
        }

        function drawFloorThing(th, iv, f) {
            const kind = th.kind;
            const p = posOf(th);
            const clip = floorClip(th, iv);
            if (!clip) return;
            if (kind === 'gate' && !(th.open && num(th.k, 1) >= 0.99) && !hasClip(clip)) return;
            if (!sprite(clip, p.x, p.y, 1, 1, f)) {
                if (kind === 'plate') fbThing(th, p.x, p.y, f);
                else if (kind === 'spikes') {
                    ctx.save();
                    ctx.fillStyle = 'rgba(20, 16, 24, 0.55)';
                    for (let i = 0; i < 3; i++) ctx.fillRect(p.x - 14 + i * 12, p.y - 8, 5, 5);
                    ctx.restore();
                } else if (kind === 'pot' || kind === 'crack') {
                    ctx.save();
                    ctx.fillStyle = kind === 'pot' ? 'rgba(150, 90, 56, 0.8)' : 'rgba(96, 82, 70, 0.85)';
                    for (let i = 0; i < 5; i++) ctx.fillRect(p.x - 16 + i * 7, p.y - 10 - (i % 2) * 6, 6, 5);
                    ctx.restore();
                }
            }
            if (kind === 'plate' && th.on) glow(p.x, p.y - 10, 30, '150, 255, 160', 0.28);
            if (kind === 'spikes' && th.phase === 'warn') {
                const a = 0.25 + 0.45 * pulse(f, 30, 0.2);
                ctx.save();
                ctx.strokeStyle = 'rgba(' + COL_WARN + ', ' + a.toFixed(3) + ')';
                ctx.fillStyle = 'rgba(' + COL_WARN + ', ' + (a * 0.35).toFixed(3) + ')';
                ctx.lineWidth = 3;
                ctx.beginPath();
                ctx.ellipse(p.x, p.y - 8, 20, 13, 0, 0, Math.PI * 2);
                ctx.fill();
                ctx.stroke();
                ctx.restore();
            }
        }

        // Guardian floor visuals from the mob state of interior-bosses.js: web patches, warning zones, charge line, steam cross, beam, blink target.
        function ellipseAt(x, y, r, fill, stroke, lw) {
            ctx.beginPath();
            ctx.ellipse(x, y, r, r * 0.66, 0, 0, Math.PI * 2);
            ctx.fillStyle = fill;
            ctx.fill();
            ctx.strokeStyle = stroke;
            ctx.lineWidth = lw;
            ctx.stroke();
        }

        function drawGuardianFloor(m, f) {
            list(m.patches).forEach(function (w) {
                if (!(num(w.r, 0) > 0)) return;
                ctx.save();
                ctx.globalAlpha = w.left > 0 && w.left < 1 ? 0.25 + 0.3 * w.left : 0.55;
                ellipseAt(w.x, w.y, w.r, 'rgba(232, 228, 216, 0.14)', '#e8e4d8', 1.5);
                ctx.strokeStyle = '#e8e4d8';
                for (let i = 0; i < 8; i++) {
                    const a = i * Math.PI / 4;
                    ctx.beginPath();
                    ctx.moveTo(w.x, w.y);
                    ctx.lineTo(w.x + Math.cos(a) * w.r, w.y + Math.sin(a) * w.r * 0.66);
                    ctx.stroke();
                }
                ctx.restore();
            });
            const warn = m.state === 'tell';
            const live = warn || m.state === 'attack';
            if (!live) return;
            const a = warn ? 0.3 + 0.4 * pulse(f, 24, 0.3) : 0.7;
            const edge = 'rgba(' + COL_WARN + ', ' + a.toFixed(3) + ')';
            const body = 'rgba(' + COL_WARN + ', ' + (a * 0.22).toFixed(3) + ')';
            ctx.save();
            list(m.zones).forEach(function (z) {
                if (z.done || !(num(z.r, 0) > 0)) return;
                ellipseAt(z.x, z.y, z.r, body, edge, 3);
            });
            const ln = m.line;
            if (ln && typeof ln === 'object') {
                ctx.strokeStyle = edge;
                ctx.lineWidth = 12;
                ctx.lineCap = 'round';
                ctx.globalAlpha = 0.45;
                ctx.beginPath();
                ctx.moveTo(ln.x0, ln.y0);
                ctx.lineTo(ln.x1, ln.y1);
                ctx.stroke();
                ctx.globalAlpha = 1;
                ctx.lineWidth = 2;
                ctx.stroke();
            }
            const cr = m.cross;
            if (cr && typeof cr === 'object') {
                ctx.fillStyle = warn ? body : 'rgba(232, 244, 255, 0.4)';
                ctx.strokeStyle = edge;
                ctx.lineWidth = 2;
                ctx.fillRect(cr.x - cr.half, cr.y0, cr.half * 2, cr.y1 - cr.y0);
                ctx.fillRect(cr.x0, cr.y - cr.half, cr.x1 - cr.x0, cr.half * 2);
                ctx.strokeRect(cr.x - cr.half, cr.y0, cr.half * 2, cr.y1 - cr.y0);
                ctx.strokeRect(cr.x0, cr.y - cr.half, cr.x1 - cr.x0, cr.half * 2);
            }
            const bl = m.blink;
            if (bl && typeof bl === 'object') ellipseAt(bl.x, bl.y - 2, 22, 'rgba(138, 216, 255, 0.18)', 'rgba(138, 216, 255, ' + a.toFixed(3) + ')', 2);
            ctx.restore();
            const beam = m.beam;
            if (beam && typeof beam === 'object' && num(beam.len, 0) > 0) {
                const ang = num(beam.a, 0);
                ctx.save();
                ctx.lineCap = 'round';
                if (num(beam.a0, ang) !== num(beam.a1, ang)) {
                    ctx.strokeStyle = 'rgba(' + COL_WARN + ', 0.35)';
                    ctx.lineWidth = 2;
                    [beam.a0, beam.a1].forEach(function (aa) {
                        ctx.beginPath();
                        ctx.moveTo(beam.x, beam.y);
                        ctx.lineTo(beam.x + Math.cos(aa) * beam.len, beam.y + Math.sin(aa) * beam.len);
                        ctx.stroke();
                    });
                }
                ctx.globalCompositeOperation = 'lighter';
                ctx.strokeStyle = warn ? 'rgba(' + COL_WARN + ', 0.55)' : 'rgba(255, 190, 90, 0.85)';
                ctx.lineWidth = warn ? 4 : 11;
                ctx.beginPath();
                ctx.moveTo(beam.x, beam.y);
                ctx.lineTo(beam.x + Math.cos(ang) * beam.len, beam.y + Math.sin(ang) * beam.len);
                ctx.stroke();
                ctx.restore();
            }
        }

        function drawFloor(iv, sim, f) {
            try {
                const things = list(iv.things);
                for (let i = 0; i < things.length; i++) {
                    try { drawFloorThing(things[i], iv, f); } catch (err) { /* one bad thing must not stop the room */ }
                }
                list(iv.mobs).forEach(function (m) { if (isBoss(m) && m.state !== 'dead') { try { drawGuardianFloor(m, f); } catch (err) { /* ignore */ } } });
                const hero = iv.hero;
                if (hero && hero.slowT > 0) {
                    ctx.save();
                    ctx.strokeStyle = 'rgba(232, 228, 216, 0.55)';
                    ctx.lineWidth = 1.5;
                    ctx.beginPath();
                    ctx.ellipse(hero.x, hero.y - 2, 24, 15, 0, 0, Math.PI * 2);
                    ctx.moveTo(hero.x - 24, hero.y - 2);
                    ctx.lineTo(hero.x + 24, hero.y - 2);
                    ctx.moveTo(hero.x, hero.y - 17);
                    ctx.lineTo(hero.x, hero.y + 13);
                    ctx.stroke();
                    ctx.restore();
                }
            } catch (err) { /* ignore */ }
            void sim;
        }

        // ---- y-sorted objects ----

        function useGroups(things) {
            const uses = things.filter(function (t) { return t && t.kind === 'use'; });
            const out = [];
            const seen = {};
            uses.forEach(function (t) {
                if (seen[t.id]) return;
                const req = String(t.request || t.use || '');
                const group = [t];
                seen[t.id] = true;
                for (let gi = 0; gi < group.length; gi++) {
                    uses.forEach(function (o) {
                        if (seen[o.id] || String(o.request || o.use || '') !== req) return;
                        if (Math.abs(o.cx - group[gi].cx) + Math.abs(o.cy - group[gi].cy) === 1) { seen[o.id] = true; group.push(o); }
                    });
                }
                const x0 = Math.min.apply(null, group.map(function (g) { return num(g.cx, 0); }));
                const x1 = Math.max.apply(null, group.map(function (g) { return num(g.cx, 0); }));
                const y0 = Math.min.apply(null, group.map(function (g) { return num(g.cy, 0); }));
                const y1 = Math.max.apply(null, group.map(function (g) { return num(g.cy, 0); }));
                const key = req.split(':')[0];
                const nominal = USE_SIZE[key] || [1, 1];
                out.push({ use: key, w: Math.max(x1 - x0 + 1, nominal[0]), h: Math.max(y1 - y0 + 1, nominal[1]), x: (x0 + x1 + 1) * CELL / 2, y: (y1 + 1) * CELL - 17 + FOOT, th: t });
            });
            return out;
        }

        // Entries { y, o } for the y-sorted list; floor-level things are drawn by drawFloor.
        function sorted(iv) {
            const out = [];
            try {
                const things = list(iv.things);
                for (let i = 0; i < things.length; i++) {
                    const th = things[i];
                    if (!th || th.kind === 'plate' || th.kind === 'use') continue;
                    if (th.kind === 'pot' && th.broken) continue;
                    if (th.kind === 'crack' && th.broken) continue;
                    if (th.kind === 'spikes' && th.phase !== 'up') continue;
                    if (th.kind === 'gate' && th.open && num(th.k, 1) >= 0.99) continue;
                    const p = th.kind === 'block' ? blockPos(th) : posOf(th);
                    out.push({ y: p.y, o: { th: th, x: p.x, y: p.y } });
                }
                useGroups(things).forEach(function (g) { out.push({ y: g.y, o: { group: g, th: g.th, x: g.x, y: g.y } }); });
            } catch (err) { /* ignore */ }
            return out;
        }

        function drawObject(e, iv, sim, f) {
            const th = e.th;
            const x = e.x;
            const y = e.y;
            if (e.group) {
                const g = e.group;
                const done = g.use === 'core' && flagOf(sim, 'obs.done');
                const clip = g.use === 'board' ? 'tdp-lore.p9' : g.use === 'dice' ? 'tdp-lore.p10' : g.use === 'core' ? (done ? 'tdp-stars.p7' : 'tdp-stars.p6') : '';
                shadow(g.x, g.y, g.w * CELL * 0.42, 0.3);
                if (!clip || !sprite(clip, g.x, g.y, g.w, g.h, f)) fbUse(g, f, sim);
                if (g.use === 'core' && !done) glow(g.x, g.y - g.h * CELL * 0.5, 80, '255, 179, 71', 0.22 * pulse(f, 3, 0.6));
                return;
            }
            const kind = th.kind;
            if (kind === 'block') { drawBlock(th, x, y, f); return; }
            if (kind === 'gate') { drawGate(th, x, y, f); return; }
            let clip = '';
            let flash = 0;
            let wc = 1;
            if (kind === 'pot') clip = POT_CLIP[th.look] || POT_CLIP.pot;
            else if (kind === 'lever') clip = th.on ? 'tdp-mech.p9' : 'tdp-mech.p8';
            else if (kind === 'spikes') clip = th.look === 'vent' ? 'tdp-lore.p8' : 'tdp-mech.p13';
            else if (kind === 'dart') { clip = 'tdp-lore.p6'; if (th.phase === 'warn') flash = 0.25 + 0.4 * pulse(f, 34, 0.1); }
            else if (kind === 'brazier') { clip = brazierClip(th, iv); if (themeOf(iv) === 'wood') wc = LAMP_SCALE; }
            else if (kind === 'crack') clip = 'tdp-lore.p4';
            else if (kind === 'page') clip = pageRead(th, sim) ? 'tdp-lore.p1' : 'tdp-lore.p0';
            else if (kind === 'sign') { if (th.look === 'scroll') { clip = 'tdp-lore.p2'; wc = 0.9; } else clip = 'tdp-lore.p12'; }
            else return;
            const flat = kind === 'sign' && th.look === 'scroll';
            if (kind !== 'spikes' && !flat) shadow(x, y, 20 * wc, 0.3);
            if (kind === 'spikes' && th.look === 'vent') glow(x, y - 16, 38, '232, 244, 255', 0.2);
            const art = sprite(clip, x, y, wc, wc, f, flash > 0 ? { flash: flash } : null);
            if (!art) fbThing(th, x, y, f, iv);
            if (kind === 'dart' && th.phase === 'warn') drawDartWarn(th, x, y, f);
            if (kind === 'brazier' && th.lit) drawFlame(th, x, y, f, art, wc);
            if (kind === 'page' && !pageRead(th, sim)) glow(x, y - 30, 40, '255, 233, 168', 0.3 * pulse(f, 3, 0.6));
            if (kind === 'spikes' && th.phase === 'up' && th.look !== 'vent') glow(x, y - 8, 22, COL_WARN, 0.12);
        }

        function pageRead(th, sim) {
            if (typeof th.have === 'boolean') return th.have;
            const loot = sim && sim.profile && Array.isArray(sim.profile.loot) ? sim.profile.loot : [];
            const n = num(th.n, num(th.page, 0));
            return loot.indexOf('page_' + (n < 10 ? '0' : '') + n) >= 0;
        }

        function drawDartWarn(th, x, y, f) {
            const d = th.dir;
            const dx = d === 'left' ? -1 : d === 'right' ? 1 : 0;
            const dy = d === 'up' ? -1 : d === 'down' ? 1 : 0;
            const a = 0.35 + 0.5 * pulse(f, 34, 0.1);
            ctx.save();
            ctx.fillStyle = 'rgba(' + COL_WARN + ', ' + a.toFixed(3) + ')';
            ctx.beginPath();
            const cx = x + dx * 24;
            const cy = y - 16 + dy * 22;
            ctx.moveTo(cx + dx * 9, cy + dy * 9);
            ctx.lineTo(cx - dy * 8, cy + dx * 8);
            ctx.lineTo(cx + dy * 8, cy - dx * 8);
            ctx.closePath();
            ctx.fill();
            ctx.restore();
            glow(x, y - 16, 34, COL_WARN, 0.25 * a);
        }

        function drawFlame(th, x, y, f, art, sc) {
            const left = num(th.left, 0);
            const low = num(th.timed, 0) > 0 && left > 0 && left < 2 ? left / 2 : 1; // a timed brazier shrinks before it goes out
            const sway = f.reduced ? 0 : Math.sin(f.t * 11 + x * 0.1) * 2;
            glow(x, y - 38 * sc, 44 * low * sc, '255, 176, 96', 0.42 * low * pulse(f, 9, 0.75));
            if (art) return;
            ctx.save();
            ctx.fillStyle = COL_FLAME;
            ctx.beginPath();
            ctx.moveTo(x - 7, y - 24);
            ctx.quadraticCurveTo(x - 3 + sway, y - 36 * low - 6, x + sway, y - 44 * low);
            ctx.quadraticCurveTo(x + 4 + sway, y - 34 * low, x + 7, y - 24);
            ctx.closePath();
            ctx.fill();
            ctx.restore();
        }

        function drawBlock(th, x, y, f) {
            const lk = th.look || th.block || 'stone';
            shadow(x, y, 21, 0.34);
            const clip = BLOCK_CLIP[lk] || BLOCK_CLIP.stone;
            if (!sprite(clip, x, y, 1, 1, f)) fbThing(th, x, y, f);
            if (lk === 'ice') glow(x, y - 20, 30, '190, 235, 255', 0.1);
            if (th.moving && !f.reduced) { // scrape dust behind the block
                const mv = th.moving;
                ctx.save();
                ctx.fillStyle = 'rgba(220, 210, 190, 0.35)';
                for (let i = 0; i < 3; i++) {
                    ctx.beginPath();
                    ctx.arc(x - num(mv.dx, 0) * (14 + i * 7), y - 3 - num(mv.dy, 0) * (14 + i * 7), 4 - i, 0, Math.PI * 2);
                    ctx.fill();
                }
                ctx.restore();
            }
        }

        // Gate: bars sink into the floor by k; the open frame lies under it (drawFloorThing).
        function drawGate(th, x, y, f) {
            const k = clamp(num(th.k, th.open ? 1 : 0), 0, 1);
            if (k >= 0.99) return;
            shadow(x, y, 21, 0.3 * (1 - k));
            ctx.save();
            ctx.beginPath();
            ctx.rect(x - CELL / 2, y - 120, CELL, 120 + 15);
            ctx.clip();
            ctx.translate(0, k * 58);
            if (!sprite('tdp-mech.p10', x, y, 1, 1, f)) fbThing(th, x, y, f);
            ctx.restore();
        }

        // ---- guardians ----

        function bossHeight(height) { return height * BOSS_SCALE; }

        function bossColor(kind) { return BOSS_COLOR[kind] || null; }

        function drawBossOverlay(m, f) {
            if (!isBoss(m) || m.state === 'dead') return;
            if (m.state === 'stun') {
                const top = bossHeight(clamp((m.r > 0 ? m.r : 24) * 4.6, 34, 84)) * 0.92; // just above the head
                ctx.save();
                ctx.fillStyle = '#ffe066';
                for (let i = 0; i < 3; i++) {
                    const a = (f.reduced ? 0 : f.t * 4) + i * 2.094;
                    ctx.beginPath();
                    ctx.arc(m.x + Math.cos(a) * 18, m.y - top + Math.sin(a) * 5, 4, 0, Math.PI * 2);
                    ctx.fill();
                }
                ctx.restore();
            }
        }

        // Name plate and health bar below the top-bar inset; screen space.
        function drawBossBar(iv, f, vw, inset) {
            try {
                let g = iv.guardian && iv.guardian.state !== 'dead' ? iv.guardian : null;
                if (!g) g = list(iv.mobs).filter(function (m) { return isBoss(m) && m.state !== 'dead'; })[0] || null;
                if (!g) return;
                const w = Math.min(BAR_W, vw * 0.7);
                const x = (vw - w) / 2;
                const y = inset + BAR_GAP + 16;
                const frac = clamp(num(g.hp, 1) / Math.max(1, num(g.maxHp, num(g.hp, 1))), 0, 1);
                textAt(tr('runenpfad.inboss.' + g.kind, g.kind), vw / 2, y - 5, 14, '#ffe9c0');
                ctx.save();
                rbox(x - 2, y - 1, w + 4, BAR_H + 2, 5, 'rgba(8, 6, 14, 0.82)', 'rgba(244, 200, 116, 0.9)');
                if (frac > 0) rbox(x, y + 1, Math.max(4, w * frac), BAR_H - 2, 3, g.phase >= 2 || frac <= 0.5 ? '#e0603c' : '#c8403c', null);
                ctx.fillStyle = 'rgba(255, 246, 220, 0.8)';
                ctx.fillRect(x + w / 2 - 1, y, 2, BAR_H);
                ctx.restore();
            } catch (err) { /* ignore */ }
            void f;
        }

        // ---- shots, key pickup, prompts ----

        function shotColor(kind) { return SHOT_COLOR[kind] || null; }

        function drawShot(s) {
            if (s.kind !== 'dart') return false;
            const sp = Math.hypot(s.vx || 0, s.vy || 0) || 1;
            const tx = (s.vx || 0) / sp;
            const ty = (s.vy || 0) / sp;
            const y = s.y - 12;
            shadow(s.x, s.y + 2, 6, 0.22);
            ctx.save();
            ctx.lineCap = 'round';
            ctx.strokeStyle = SHOT_COLOR.dart;
            ctx.lineWidth = 3;
            ctx.beginPath();
            ctx.moveTo(s.x - tx * 12, y - ty * 12);
            ctx.lineTo(s.x + tx * 6, y + ty * 6);
            ctx.stroke();
            ctx.fillStyle = '#c85a4a';
            ctx.beginPath();
            ctx.moveTo(s.x + tx * 12, y + ty * 12);
            ctx.lineTo(s.x + tx * 4 - ty * 4, y + ty * 4 + tx * 4);
            ctx.lineTo(s.x + tx * 4 + ty * 4, y + ty * 4 - tx * 4);
            ctx.closePath();
            ctx.fill();
            ctx.restore();
            return true;
        }

        function drawKey(p, f) {
            const bob = f.reduced ? 0 : Math.sin(f.t * 4 + p.x * 0.05) * 3;
            shadow(p.x, p.y, 10, 0.3);
            glow(p.x, p.y - 14 + bob, 26, '255, 224, 102', 0.4);
            if (sprite('tdp-lore.p3', p.x, p.y + bob, 0.5, 0.5, f)) return true;
            ctx.save();
            ctx.strokeStyle = '#ffd36a';
            ctx.fillStyle = '#ffd36a';
            ctx.lineWidth = 3;
            ctx.beginPath();
            ctx.arc(p.x - 5, p.y - 16 + bob, 5, 0, Math.PI * 2);
            ctx.stroke();
            ctx.fillRect(p.x - 1, p.y - 17 + bob, 12, 3);
            ctx.fillRect(p.x + 7, p.y - 14 + bob, 3, 5);
            ctx.restore();
            return true;
        }

        const PROMPT_KINDS = { pull: 1, light: 1, read: 1, unlock: 1, board: 1, play: 1, shut: 1 };

        // Small drawn icon above the key cap for the new prompt kinds; false for the old ones.
        function promptIcon(kind, x, y) {
            if (!PROMPT_KINDS[kind]) return false;
            ctx.save();
            ctx.translate(x, y);
            ctx.lineJoin = 'round';
            ctx.lineCap = 'round';
            ctx.strokeStyle = '#09111a';
            ctx.lineWidth = 3;
            if (kind === 'pull') {
                ctx.fillStyle = '#c9ced8';
                rbox(-9, 2, 18, 8, 3, '#8a8f99', '#09111a');
                ctx.beginPath();
                ctx.moveTo(0, 4);
                ctx.lineTo(8, -12);
                ctx.stroke();
                ctx.strokeStyle = '#ffd27a';
                ctx.lineWidth = 2;
                ctx.stroke();
                ctx.fillStyle = '#ffd27a';
                ctx.beginPath();
                ctx.arc(8, -12, 3.5, 0, Math.PI * 2);
                ctx.fill();
            } else if (kind === 'light') {
                ctx.fillStyle = '#ffb060';
                ctx.beginPath();
                ctx.moveTo(-7, 8);
                ctx.quadraticCurveTo(-8, -4, 0, -13);
                ctx.quadraticCurveTo(8, -4, 7, 8);
                ctx.closePath();
                ctx.stroke();
                ctx.fill();
                ctx.fillStyle = '#fff0b0';
                ctx.beginPath();
                ctx.ellipse(0, 4, 3, 5, 0, 0, Math.PI * 2);
                ctx.fill();
            } else if (kind === 'read') {
                ctx.fillStyle = '#f4e6b8';
                ctx.beginPath();
                ctx.moveTo(0, -6);
                ctx.lineTo(-12, -9);
                ctx.lineTo(-12, 7);
                ctx.lineTo(0, 10);
                ctx.lineTo(12, 7);
                ctx.lineTo(12, -9);
                ctx.closePath();
                ctx.stroke();
                ctx.fill();
                ctx.strokeStyle = '#7a5a30';
                ctx.lineWidth = 1.5;
                ctx.beginPath();
                ctx.moveTo(0, -6);
                ctx.lineTo(0, 10);
                ctx.stroke();
            } else if (kind === 'unlock') {
                ctx.strokeStyle = '#09111a';
                ctx.lineWidth = 5;
                ctx.beginPath();
                ctx.arc(-5, -3, 6, 0, Math.PI * 2);
                ctx.moveTo(0, -3);
                ctx.lineTo(12, -3);
                ctx.stroke();
                ctx.strokeStyle = '#ffd36a';
                ctx.lineWidth = 2.5;
                ctx.stroke();
            } else if (kind === 'board') {
                rbox(-11, -11, 22, 22, 3, '#f2e6c0', '#09111a');
                ctx.fillStyle = '#7a5a30';
                ctx.fillRect(-7, -6, 14, 2);
                ctx.fillRect(-7, -1, 14, 2);
                ctx.fillRect(-7, 4, 9, 2);
            } else if (kind === 'play') {
                rbox(-10, -10, 20, 20, 4, '#fff6dc', '#09111a');
                ctx.fillStyle = '#2a1d10';
                [[-4, -4], [4, 4], [0, 0], [4, -4], [-4, 4]].forEach(function (d) { ctx.beginPath(); ctx.arc(d[0], d[1], 1.8, 0, Math.PI * 2); ctx.fill(); });
            } else {
                ctx.fillStyle = '#ff9a8a';
                rbox(-9, -3, 18, 14, 4, '#ff9a8a', '#09111a');
                ctx.beginPath();
                ctx.arc(0, -3, 6, Math.PI, 0);
                ctx.stroke();
            }
            ctx.restore();
            return true;
        }

        function dispose() { /* nothing cached */ }

        return {
            sheetsOfDef: sheetsOfDef,
            mobSheet: mobSheet,
            isBoss: isBoss,
            bossHeight: bossHeight,
            bossColor: bossColor,
            drawFloor: drawFloor,
            sorted: sorted,
            drawObject: function (e, iv, sim, f) { try { drawObject(e, iv, sim, f); } catch (err) { /* ignore */ } },
            drawBossOverlay: drawBossOverlay,
            drawBossBar: drawBossBar,
            shotColor: shotColor,
            drawShot: drawShot,
            drawKey: drawKey,
            promptIcon: promptIcon,
            dispose: dispose
        };
    }

    root.RunenpfadInteriorThingsView = { create: create, BOSS_IDS: BOSS_IDS };
})(typeof window !== 'undefined' ? window : globalThis);
