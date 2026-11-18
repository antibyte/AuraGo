/* Runenpfad painted art atlas: sheet loader, clip/frame lookup, sprite drawing and the sprite-FX layer. Canvas-facing, optional at runtime. */
(function (root) {
    'use strict';

    const MAX_ATLAS_BYTES = 1024 * 1024;
    const MAX_FX = 56;
    const RUNE_COLORS = { fire: '#fb923c', frost: '#7dd3fc', storm: '#c4b5fd', magnet: '#fde047' };
    const MACHINES = { service_robot: 1, spider_drone: 1, quadcopter: 1, security_orb: 1, combat_mech: 1, sentry_bot: 1, bomber_drone: 1, clock_knight: 1, boss_orb: 1, boss_quad: 1, boss_mech: 1 };

    function loadImage(url, signal) {
        return new Promise(function (resolve, reject) {
            if (signal && signal.aborted) { reject(new Error('aborted')); return; }
            const img = new root.Image();
            img.onload = function () { resolve(img); };
            img.onerror = function () { reject(new Error('image')); };
            img.src = url;
        });
    }

    // Resolves to the decoded atlas or null. Never rejects and never throws.
    async function load(signal) {
        try {
            const ver = encodeURIComponent(root.BUILD_VERSION || 'dev');
            const resp = await root.fetch('/img/runenpfad/atlas.json?v=' + ver, { signal: signal, credentials: 'same-origin' });
            if (!resp.ok) return null;
            const text = await resp.text();
            if (text.length > MAX_ATLAS_BYTES) return null;
            const json = JSON.parse(text);
            if (!json || json.version !== 1 || !json.sheets || typeof json.sheets !== 'object') return null;
            const all = Object.keys(json.sheets);
            // Painted parallax sheets (bg-*) are large and biome specific: they load lazily through loadSheet.
            const ids = all.filter(function (id) { return !LAZY.test(id); });
            const images = await Promise.all(ids.map(function (id) {
                const file = String(json.sheets[id].file || '');
                if (!/^[a-z0-9-]+\.webp$/.test(file)) return Promise.resolve(null);
                return loadImage('/img/runenpfad/' + file + '?v=' + ver, signal).catch(function () { return null; });
            }));
            const atlas = { sheets: {}, clips: {}, ids: [], lazy: {}, ver: ver };
            all.forEach(function (id) { if (LAZY.test(id)) atlas.lazy[id] = json.sheets[id]; });
            ids.forEach(function (id, i) { register(atlas, id, json.sheets[id], images[i]); });
            if (!atlas.ids.length) return null;
            current = atlas;
            return atlas;
        } catch (err) {
            return null;
        }
    }

    const LAZY = /^bg-/;
    let current = null;

    function register(atlas, id, sh, img) {
        if (!img || !sh || !Array.isArray(sh.frames) || !sh.clips || !(sh.refH > 0)) return false;
        const sheet = { id: id, img: img, frames: sh.frames, refH: sh.refH, def: sh };
        atlas.sheets[id] = sheet;
        if (atlas.ids.indexOf(id) < 0) atlas.ids.push(id);
        Object.keys(sh.clips).forEach(function (name) {
            const c = sh.clips[name];
            if (!c || !Array.isArray(c.frames) || !c.frames.length) return;
            const clip = { sheet: sheet, name: name, frames: c.frames, ms: Math.max(16, +c.ms || 100), loop: !!c.loop };
            atlas.clips[id + '.' + name] = clip;
            if (name === sh.default) atlas.clips[id] = clip;
        });
        return true;
    }

    // Loads one lazy (bg-*) sheet into the atlas. Resolves true or false and never rejects.
    function loadSheet(atlas, id, signal) {
        try {
            if (!atlas) return Promise.resolve(false);
            if (atlas.sheets[id]) return Promise.resolve(true);
            const sh = atlas.lazy && atlas.lazy[id];
            const file = sh ? String(sh.file || '') : '';
            if (!/^[a-z0-9-]+\.webp$/.test(file)) return Promise.resolve(false);
            return loadImage('/img/runenpfad/' + file + '?v=' + atlas.ver, signal).then(function (img) {
                return register(atlas, id, sh, img);
            }).catch(function () { return false; });
        } catch (err) {
            return Promise.resolve(false);
        }
    }

    function has(atlas, clipId) {
        return !!(atlas && atlas.clips[clipId]);
    }

    // Seconds one pass of a clip takes.
    function duration(atlas, clipId) {
        const c = atlas && atlas.clips[clipId];
        return c ? (c.frames.length * c.ms) / 1000 : 0;
    }

    // Sheet frame index for a clip at `timeSec` (loops or holds the last frame); -1 when unknown.
    function frameAt(atlas, clipId, timeSec) {
        const c = atlas && atlas.clips[clipId];
        if (!c) return -1;
        const n = c.frames.length;
        let i = Math.floor((timeSec > 0 ? timeSec : 0) * 1000 / c.ms);
        i = c.loop ? i % n : (i >= n ? n - 1 : i);
        return c.frames[i];
    }

    let scratch = null;
    let scratchCtx = null;

    function scratchFor(w, h) {
        if (!scratch) {
            if (!root.document) return null;
            scratch = root.document.createElement('canvas');
            scratchCtx = scratch.getContext('2d');
        }
        if (!scratchCtx) return null;
        if (scratch.width < w || scratch.height < h) {
            scratch.width = Math.max(scratch.width, w);
            scratch.height = Math.max(scratch.height, h);
        }
        return scratchCtx;
    }

    // Draws clip frame at `timeSec` with the clip pivot at (x, y). opts: height | scale, flip, alpha, composite, rotation,
    // frame (explicit clip frame number instead of time), flash (0..1 white), tint (css colour) + tintAmount.
    function draw(ctx, atlas, clipId, timeSec, x, y, opts) {
        const c = atlas && atlas.clips[clipId];
        if (!c) return false;
        const o = opts || {};
        let idx;
        if (typeof o.frame === 'number') idx = c.frames[Math.max(0, Math.min(c.frames.length - 1, o.frame | 0))];
        else idx = frameAt(atlas, clipId, timeSec);
        const f = c.sheet.frames[idx];
        if (!f || !(f[2] > 0)) return false;
        const sc = o.scale > 0 ? o.scale : (o.height > 0 ? o.height / c.sheet.refH : 1);
        const alpha = o.alpha == null ? 1 : o.alpha;
        if (alpha <= 0.01) return false;
        ctx.save();
        ctx.translate(x, y);
        if (o.rotation) ctx.rotate(o.rotation);
        if (o.flip) ctx.scale(-1, 1);
        ctx.imageSmoothingEnabled = true;
        ctx.imageSmoothingQuality = 'high';
        if (alpha < 1) ctx.globalAlpha = alpha;
        if (o.composite) ctx.globalCompositeOperation = o.composite;
        const dx = f[4] * sc;
        const dy = f[5] * sc;
        const dw = f[2] * sc;
        const dh = f[3] * sc;
        const tinted = o.tint || o.flash > 0.02;
        const sctx = tinted ? scratchFor(f[2], f[3]) : null;
        if (!sctx) {
            ctx.drawImage(c.sheet.img, f[0], f[1], f[2], f[3], dx, dy, dw, dh);
        } else {
            sctx.setTransform(1, 0, 0, 1, 0, 0);
            sctx.globalCompositeOperation = 'source-over';
            sctx.globalAlpha = 1;
            sctx.clearRect(0, 0, f[2], f[3]);
            sctx.drawImage(c.sheet.img, f[0], f[1], f[2], f[3], 0, 0, f[2], f[3]);
            sctx.globalCompositeOperation = 'source-atop';
            sctx.fillStyle = o.tint || '#ffffff';
            sctx.globalAlpha = o.tint ? (o.tintAmount == null ? 0.55 : o.tintAmount) : 1;
            sctx.fillRect(0, 0, f[2], f[3]);
            if (o.tint) {
                ctx.drawImage(scratch, 0, 0, f[2], f[3], dx, dy, dw, dh);
            } else {
                ctx.drawImage(c.sheet.img, f[0], f[1], f[2], f[3], dx, dy, dw, dh);
                ctx.globalAlpha = alpha * Math.min(1, o.flash);
                ctx.drawImage(scratch, 0, 0, f[2], f[3], dx, dy, dw, dh);
            }
        }
        ctx.restore();
        return true;
    }

    // ---- sprite FX layer (renderer-owned, driven by simulation events, never writes simulation state) ----
    function createFx() {
        const list = [];
        const O = { height: 0, flip: false, alpha: 1, composite: '', rotation: 0, tint: '', tintAmount: 0.55, frame: undefined };
        let seed = 7;
        let fallVy = 0;
        let prevProj = [];
        let nextProj = [];
        let stormClock = 3;
        let cine = 0;

        function rnd() {
            seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
            return seed / 4294967296;
        }

        function add(clip, x, y, h, o) {
            const p = o || {};
            if (list.length >= MAX_FX) list.shift();
            list.push({
                clip: clip, x: x, y: y, h: h, t: -(p.delay || 0), flip: !!p.flip, rot: p.rot || 0, comp: p.comp === undefined ? 'lighter' : p.comp,
                alpha: p.alpha == null ? 1 : p.alpha, tint: p.tint || '', life: p.life || 0, loop: !!p.loop, dur: 0
            });
        }

        // Body centre + box height of the enemy closest to a point (falls back to a small default).
        function body(sim, x, y) {
            let best = null;
            let bd = 90 * 90;
            const list2 = sim && sim.stage ? sim.stage.enemies : null;
            if (list2) {
                for (let i = 0; i < list2.length; i++) {
                    const e = list2[i];
                    const d = (e.x - x) * (e.x - x) + (e.y - y) * (e.y - y);
                    if (d < bd) { bd = d; best = e; }
                }
            }
            if (!best) return { cx: x, cy: y - 18, h: 40, e: null };
            const h = (best.def && best.def.boss ? 42 : 38) * (best.def ? best.def.scale || 1 : 1);
            return { cx: best.x, cy: best.y - h / 2, h: h, e: best };
        }

        function onEvent(ev, sim) {
            const d = ev.detail || {};
            const p = sim && sim.player;
            const x = typeof d.x === 'number' ? d.x : (p ? p.x : 0);
            const y = typeof d.y === 'number' ? d.y : (p ? p.y : 0);
            const face = p && p.facing < 0 ? -1 : 1;
            switch (ev.type) {
                case 'attack_melee':
                    if (p && sim.heroId !== 'ranger') add('fx-slash', p.x + face * 34, p.y - 28, 84, { flip: face < 0, tint: p.rune && p.rune.kind === 'fire' && p.rune.timeLeft > 0 ? '#ff7a2a' : '', alpha: 1 });
                    break;
                case 'enemy_hit': {
                    const b = body(sim, x, y);
                    const hx = p ? b.cx + (p.x - b.cx) * 0.18 : b.cx;
                    add('fx-hit', hx, b.cy, b.e && b.e.def.boss ? 74 : 46, { rot: rnd() * 6.28 });
                    break;
                }
                case 'enemy_kill': {
                    if (d.boss) break;
                    const b = body(sim, x, y);
                    if (MACHINES[d.id]) {
                        add('fx-explosion', b.cx, b.cy, Math.max(80, b.h * 2.1), { comp: 'source-over' });
                        add('fx-explosion', b.cx, b.cy, Math.max(80, b.h * 2.1), { comp: 'lighter', alpha: 0.35 });
                    } else {
                        add('fx-rune-burst', b.cx, b.cy, Math.max(54, b.h * 1.4), {});
                        add('fx-dust', b.cx, y, 54, { comp: 'source-over', alpha: 0.8 });
                    }
                    if (d.id === 'frost_wraith') add('fx-frost', b.cx, b.cy, 90, { comp: 'source-over' });
                    break;
                }
                case 'land':
                    if (fallVy > 380 && p) {
                        add('fx-dust', p.x - 12, p.y, 56, { comp: 'source-over', alpha: 0.85, flip: true });
                        add('fx-dust', p.x + 12, p.y, 56, { comp: 'source-over', alpha: 0.85 });
                    }
                    fallVy = 0;
                    break;
                case 'dash':
                    if (p) add('fx-dust', p.x - face * 12, p.y, 50, { comp: 'source-over', alpha: 0.8, flip: face > 0 });
                    break;
                case 'stomp':
                    add('fx-dust', x, y, 64, { comp: 'source-over', alpha: 0.9 });
                    add('fx-hit', x, y - 10, 50, {});
                    break;
                case 'heal':
                case 'pickup_heart':
                case 'pickup_potion':
                    add('fx-sparkle', x, typeof d.y === 'number' && ev.type !== 'heal' ? y : y - 28, 80, { tint: ev.type === 'pickup_potion' ? '#a78bfa' : '#fda4af' });
                    break;
                case 'chest_open':
                    add('fx-sparkle', x, y - 40, 96, { tint: RUNE_COLORS[d.kind] || '' });
                    add('fx-dust', x, y, 60, { comp: 'source-over', alpha: 0.7 });
                    break;
                case 'bomb_blast':
                    add('fx-explosion', x, y, 126, { comp: 'source-over' });
                    add('fx-explosion', x, y, 126, { alpha: 0.4 });
                    add('fx-dust', x, y + 16, 70, { comp: 'source-over', alpha: 0.7 });
                    break;
                case 'boss_slam':
                    add('fx-dust', x - 20, y, 110, { comp: 'source-over', alpha: 0.9, flip: true });
                    add('fx-dust', x + 20, y, 110, { comp: 'source-over', alpha: 0.9 });
                    add('fx-explosion', x, y - 26, 90, { comp: 'source-over', alpha: 0.9 });
                    break;
                case 'slam_land':
                    add('fx-dust', x - 14, y, 72, { comp: 'source-over', alpha: 0.9, flip: true });
                    add('fx-dust', x + 14, y, 72, { comp: 'source-over', alpha: 0.9 });
                    add('fx-hit', x, y - 14, 70, {});
                    break;
                case 'enemy_slam':
                    add('fx-dust', x - 14, y, 66, { comp: 'source-over', alpha: 0.85, flip: true });
                    add('fx-dust', x + 14, y, 66, { comp: 'source-over', alpha: 0.85 });
                    break;
                case 'boss_attack':
                    if (d.pattern === 'fire_strip') fireStrip(sim);
                    break;
                case 'rune_zap':
                    if (typeof d.x1 === 'number' && typeof d.x2 === 'number') zap(d.x1, d.y1, d.x2, d.y2);
                    break;
                case 'rune_shield_break':
                    add('fx-frost', x, y, 96, { comp: 'source-over' });
                    break;
                case 'spore_burst':
                    add('fx-poison', x, y - 10, 84, { comp: 'source-over', alpha: 0.9 });
                    break;
                case 'geyser_burst':
                    add('fx-steam', x, y, 170, { comp: 'source-over', alpha: 0.85 });
                    break;
                case 'jet_fire':
                    add('fx-fire-pillar', x, y, 130, { comp: 'lighter', alpha: 0.9 });
                    break;
                case 'rune_pickup':
                    add('fx-rune-burst', x, y, 110, { tint: RUNE_COLORS[d.kind] || '#ffffff', tintAmount: 0.7 });
                    add('fx-sparkle', x, y, 80, { tint: RUNE_COLORS[d.kind] || '' });
                    break;
                case 'boss_defeat':
                    defeat(sim, x, y);
                    break;
                default:
            }
        }

        function fireStrip(sim) {
            const list2 = sim.stage.enemies;
            for (let i = 0; i < list2.length; i++) {
                const r = list2[i].ai && list2[i].ai.rect;
                if (!list2[i].def.boss || !r || list2[i].dead) continue;
                const n = Math.max(3, Math.round(r.w / 52));
                for (let k = 0; k < n; k++) add('fx-fire-pillar', r.x + (k + 0.5) * (r.w / n), r.y + r.h, 112, { delay: k * 0.05, alpha: 0.95 });
                return;
            }
        }

        // Lightning stretched from (x1, y1) towards (x2, y2); the sheet bolt stands up from its base.
        function zap(x1, y1, x2, y2) {
            const dx = x2 - x1;
            const dy = y2 - y1;
            const dist = Math.max(40, Math.hypot(dx, dy));
            add('fx-lightning', x1, y1, dist, { rot: Math.atan2(dy, dx) + Math.PI / 2 });
            add('fx-hit', x2, y2, 56, { tint: '#c4b5fd' });
        }

        // Chained explosions across the boss body over ~1.5 s.
        function defeat(sim, x, y) {
            const b = body(sim, x, y);
            const bh = Math.max(84, b.h * 1.3);
            cine = 1.6;
            for (let i = 0; i < 9; i++) {
                const ex = b.cx + (rnd() - 0.5) * bh * 1.3;
                const ey = b.cy + (rnd() - 0.5) * bh * 0.9;
                add('fx-explosion', ex, ey, bh * (0.9 + rnd() * 0.7), { delay: i * 0.17, comp: i % 2 ? 'lighter' : 'source-over' });
            }
            add('fx-explosion', b.cx, b.cy, bh * 2.4, { delay: 1.5, comp: 'lighter' });
            add('fx-dust', b.cx, y, bh * 1.4, { delay: 0.4, comp: 'source-over', alpha: 0.8 });
        }

        // Tracks falls, ranger bolt impacts and storm-trial lightning from read-only state.
        function step(sim, dt, view, atlas) {
            const p = sim.player;
            if (p) {
                if (!p.onGround) { if (p.vy > fallVy) fallVy = p.vy; } else if (fallVy > 0 && fallVy < 380) fallVy = 0;
            }
            if (cine > 0) cine = Math.max(0, cine - dt);
            const pl = sim.projectiles;
            nextProj.length = 0;
            for (let i = 0; i < prevProj.length; i += 3) {
                const ref = prevProj[i];
                if (ref.dead || pl.indexOf(ref) < 0) {
                    if (!ref.fromEnemy && ref.life > 0.04 && !ref.kind) add('fx-rune-burst', prevProj[i + 1], prevProj[i + 2], 40, { tint: '#7ee8fa', alpha: 0.9 });
                }
            }
            for (let i = 0; i < pl.length; i++) {
                if (!pl[i].dead && !pl[i].fromEnemy) nextProj.push(pl[i], pl[i].x, pl[i].y);
            }
            const swap = prevProj;
            prevProj = nextProj;
            nextProj = swap;
            const st = sim.stage;
            if (st && st.weather === 'rain' && st.ambience && st.ambience.secondary === 'rain-heavy' && !view.reducedMotion) {
                stormClock -= dt;
                if (stormClock <= 0) {
                    stormClock = 4 + rnd() * 5;
                    const lx = view.sx + (0.1 + rnd() * 0.8) * view.viewW;
                    add('fx-lightning', lx, view.sy + view.viewH * 0.62, view.viewH * 0.75, { alpha: 0.75, flip: rnd() < 0.5 });
                }
            }
            for (let i = list.length - 1; i >= 0; i--) {
                const f = list[i];
                f.t += dt;
                if (f.dur === 0) f.dur = f.life || duration(atlas, f.clip) + 0.05;
                if (f.t > f.dur || (atlas && !atlas.clips[f.clip])) list.splice(i, 1);
            }
        }

        function drawAll(ctx, atlas, view) {
            if (!atlas) return;
            const x0 = view.sx - 160;
            const x1 = view.sx + view.viewW + 160;
            for (let i = 0; i < list.length; i++) {
                const f = list[i];
                if (f.t < 0 || f.x < x0 || f.x > x1) continue;
                O.height = f.h;
                O.flip = f.flip;
                O.rotation = f.rot;
                O.composite = f.comp;
                O.tint = f.tint;
                const left = f.dur - f.t;
                O.alpha = f.alpha * (left < 0.12 ? left / 0.12 : 1);
                draw(ctx, atlas, f.clip, f.t, f.x, f.y, O);
            }
        }

        return {
            onEvent: onEvent,
            step: step,
            draw: drawAll,
            // True while the boss defeat cinematic runs (renderer adds shake and a white flash from it).
            cinematic: function () { return cine; },
            count: function () { return list.length; },
            clear: function () { list.length = 0; prevProj.length = 0; nextProj.length = 0; fallVy = 0; cine = 0; }
        };
    }

    root.RunenpfadArt = {
        RUNE_COLORS: RUNE_COLORS,
        load: load,
        loadSheet: loadSheet,
        current: function () { return current; },
        has: has,
        duration: duration,
        frameAt: frameAt,
        draw: draw,
        createFx: createFx
    };
})(typeof window !== 'undefined' ? window : globalThis);
