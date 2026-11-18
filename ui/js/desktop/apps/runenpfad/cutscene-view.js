/* Runenpfad cutscene renderer (global `RunenpfadCutsceneView`): paints one full-screen story panel on a canvas.
 * Cover-fit nearest-neighbour image, Ken Burns pan, a foreground layer at 1.6x the pan, visual-RNG particles,
 * additive glow lights, small sprite actors, crossfade between panels, cinematic bars, vignette and a dark
 * gradient under the subtitle. Canvas only: it reads the player state from cutscenes.js and never mutates it.
 * Every part degrades quietly: without art the panel is a dark gradient with the scene title. Never throws. */
(function (root) {
    'use strict';

    const BAR = 0.045;          // cinematic bar height as a share of the canvas height
    const FG_PARALLAX = 1.6;
    const FALLBACK = {
        story_dawn: ['#2c2140', '#d28a52'],
        story_dark: ['#070b16', '#26364d'],
        story_triumph: ['#16254a', '#e0aa48']
    };

    function Art() { return root.RunenpfadArt || null; }
    function Cut() { return root.RunenpfadCutscenes || null; }
    function clamp(v, lo, hi) { return v < lo ? lo : v > hi ? hi : v; }
    function lerp(a, b, t) { return a + (b - a) * t; }
    function ease(t) { return t * t * (3 - 2 * t); }
    function frac(v) { return v - Math.floor(v); }

    // Small seeded generator: the particle layout of a panel is the same on every visit.
    function rngFor(str) {
        let h = 2166136261;
        for (let i = 0; i < str.length; i++) h = Math.imul(h ^ str.charCodeAt(i), 16777619) >>> 0;
        return function () {
            h = (Math.imul(h, 1664525) + 1013904223) >>> 0;
            return h / 4294967296;
        };
    }

    function hexRgb(hex) {
        const m = /^#([0-9a-f]{6})$/i.exec(String(hex || ''));
        const n = m ? parseInt(m[1], 16) : 0xffd56b;
        return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
    }

    function create(canvas, opts) {
        const o = opts || {};
        const ctx = canvas && typeof canvas.getContext === 'function' ? canvas.getContext('2d') : null;
        const requested = {};
        const glows = {};
        const layouts = {};
        let requestedFor = null;
        let mistSprite = null;
        let disposed = false;
        let lastKey = '';
        let last = null;      // { scene, panel, key } of the panel drawn last
        let lastT = 0;
        let prev = null;      // { scene, panel, key, t0, at } while a crossfade runs
        let fadeAt = 0;

        // ---- loading ----
        function request(id) {
            const A = Art();
            const atlas = A && A.current ? A.current() : null;
            if (!id || !atlas || typeof A.loadSheet !== 'function') return;
            if (requestedFor !== atlas) {
                requestedFor = atlas;
                Object.keys(requested).forEach(function (k) { delete requested[k]; });
            }
            if (requested[id]) return;
            requested[id] = true;
            A.loadSheet(atlas, id);
        }

        function requestPanel(p) {
            if (!p) return;
            request(p.art);
            if (p.fg) request(p.fg);
        }

        function requestScene(id) {
            const C = Cut();
            const s = C && C.SCENES[id];
            if (s) s.panels.forEach(requestPanel);
        }

        // Warms the sheets of the first scene (all panels) and the opening panel of the next one.
        function prime(sceneIds) {
            try {
                const C = Cut();
                const ids = Array.isArray(sceneIds) ? sceneIds : [];
                if (ids.length) requestScene(ids[0]);
                const next = ids.length > 1 && C ? C.SCENES[ids[1]] : null;
                if (next) requestPanel(next.panels[0]);
            } catch (err) { /* art is optional */ }
        }

        function image(id) {
            const A = Art();
            const atlas = A && A.current ? A.current() : null;
            const clip = atlas && id ? atlas.clips[id + '.full'] : null;
            if (!clip || !clip.sheet || !clip.sheet.img) return null;
            const f = clip.sheet.frames[clip.frames[0]];
            return f && f[2] > 0 && f[3] > 0 ? { img: clip.sheet.img, x: f[0], y: f[1], w: f[2], h: f[3] } : null;
        }

        // ---- camera: visible window of an iw x ih image on a W x H canvas ----
        function camera(W, H, iw, ih, cx, cy, zoom) {
            const s = Math.max(W / iw, H / ih) * Math.max(1, zoom);
            const vw = W / s;
            const vh = H / s;
            return {
                s: s,
                sx: clamp(cx * iw - vw / 2, 0, Math.max(0, iw - vw)),
                sy: clamp(cy * ih - vh / 2, 0, Math.max(0, ih - vh)),
                iw: iw,
                ih: ih
            };
        }

        function panCenter(panel, p, reduced) {
            const a = panel.pan.from;
            const b = panel.pan.to;
            const e = reduced ? 0 : ease(p);
            return [lerp(a[0], b[0], e), lerp(a[1], b[1], e), lerp(a[2], b[2], e)];
        }

        // ---- glow sprite ----
        function glow(color) {
            if (glows[color]) return glows[color];
            if (!root.document) return null;
            const c = root.document.createElement('canvas');
            c.width = c.height = 64;
            const g = c.getContext('2d');
            if (!g) return null;
            const rgb = hexRgb(color).join(',');
            const rg = g.createRadialGradient(32, 32, 0, 32, 32, 32);
            rg.addColorStop(0, 'rgba(' + rgb + ',1)');
            rg.addColorStop(0.35, 'rgba(' + rgb + ',0.4)');
            rg.addColorStop(1, 'rgba(' + rgb + ',0)');
            g.fillStyle = rg;
            g.fillRect(0, 0, 64, 64);
            glows[color] = c;
            return c;
        }

        function mist() {
            if (mistSprite || !root.document) return mistSprite;
            const c = root.document.createElement('canvas');
            c.width = c.height = 64;
            const g = c.getContext('2d');
            if (!g) return null;
            const rg = g.createRadialGradient(32, 32, 0, 32, 32, 32);
            rg.addColorStop(0, 'rgba(210,222,240,0.5)');
            rg.addColorStop(1, 'rgba(210,222,240,0)');
            g.fillStyle = rg;
            g.fillRect(0, 0, 64, 64);
            mistSprite = c;
            return c;
        }

        // ---- particles: fixed seeded layout, position is a pure function of time ----
        const COUNTS = { motes: 36, embers: 44, snow: 90, mist: 7, sand: 70, sparks: 26, rain: 90 };

        function layoutFor(key, kind) {
            const id = key + '|' + kind;
            if (layouts[id]) return layouts[id];
            const rnd = rngFor(id);
            const list = [];
            const n = COUNTS[kind] || 0;
            for (let i = 0; i < n; i++) list.push({ x: rnd(), y: rnd(), v: rnd(), a: rnd(), s: rnd(), ph: rnd() * 6.283 });
            layouts[id] = list;
            return list;
        }

        function dot(x, y, size, color, alpha) {
            ctx.globalAlpha = alpha;
            ctx.fillStyle = color;
            ctx.fillRect(Math.round(x), Math.round(y), size, size);
        }

        function particles(key, kinds, W, H, T) {
            const u = Math.max(1, Math.round(H / 540));
            for (let k = 0; k < kinds.length; k++) {
                const kind = kinds[k];
                const list = layoutFor(key, kind);
                ctx.save();
                if (kind === 'motes' || kind === 'embers' || kind === 'sparks') ctx.globalCompositeOperation = 'lighter';
                for (let i = 0; i < list.length; i++) {
                    const p = list[i];
                    if (kind === 'motes') {
                        const x = frac(p.x + Math.sin(T * 0.25 + p.ph) * 0.02) * W;
                        const y = frac(p.y - T * (0.01 + p.v * 0.02)) * H;
                        dot(x, y, Math.max(2, Math.round((1.5 + p.s * 2) * u)), '#ffe9a8', 0.25 + 0.5 * (0.5 + 0.5 * Math.sin(T * 1.6 + p.ph)));
                    } else if (kind === 'embers') {
                        const x = frac(p.x + Math.sin(T * 1.1 + p.ph) * 0.03 + T * 0.01) * W;
                        const y = frac(p.y - T * (0.04 + p.v * 0.06)) * H;
                        dot(x, y, Math.max(2, Math.round((1.5 + p.s * 1.8) * u)), p.a > 0.5 ? '#ff9a3a' : '#ffd27a', 0.35 + 0.5 * p.a);
                    } else if (kind === 'snow') {
                        const x = frac(p.x + Math.sin(T * 0.7 + p.ph) * 0.03 + T * 0.012) * W;
                        const y = frac(p.y + T * (0.035 + p.v * 0.055)) * H;
                        dot(x, y, Math.max(2, Math.round((1.5 + p.s * 2.2) * u)), '#f2f8ff', 0.45 + 0.5 * p.a);
                    } else if (kind === 'sand') {
                        const x = frac(p.x + T * (0.08 + p.v * 0.12)) * W;
                        const y = (p.y + Math.sin(T * 0.8 + p.ph) * 0.01) * H;
                        ctx.globalAlpha = 0.2 + 0.4 * p.a;
                        ctx.fillStyle = '#f0ce8c';
                        ctx.fillRect(Math.round(x), Math.round(y), Math.round((6 + p.s * 12) * u), u);
                    } else if (kind === 'sparks') {
                        const period = 0.8 + p.v * 1.4;
                        const local = frac(T / period + p.ph);
                        const x = (p.x + (p.a - 0.5) * local * 0.12) * W;
                        const y = (p.y - local * 0.12) * H;
                        dot(x, y, Math.max(2, 2 * u), local < 0.5 ? '#fff2a0' : '#ffb040', 1 - local);
                    } else if (kind === 'rain') {
                        const x = frac(p.x - T * 0.12) * W;
                        const y = frac(p.y + T * (0.9 + p.v * 0.5)) * H;
                        ctx.globalAlpha = 0.25 + 0.3 * p.a;
                        ctx.fillStyle = '#b4cdff';
                        ctx.fillRect(Math.round(x), Math.round(y), u, Math.round((9 + p.s * 10) * u));
                    } else if (kind === 'mist') {
                        const sp = mist();
                        if (!sp) continue;
                        const r = (0.22 + p.s * 0.18) * W;
                        const x = (frac(p.x + T * (0.006 + p.v * 0.012)) * 1.6 - 0.3) * W;
                        const y = (0.5 + p.y * 0.5) * H;
                        ctx.globalAlpha = 0.07 + 0.09 * p.a;
                        ctx.drawImage(sp, x - r, y - r * 0.55, r * 2, r * 1.1);
                    }
                }
                ctx.restore();
            }
        }

        // ---- one panel ----
        function drawFallback(scene, W, H) {
            const cols = FALLBACK[scene && scene.music] || FALLBACK.story_dark;
            const g = ctx.createLinearGradient(0, 0, 0, H);
            g.addColorStop(0, cols[0]);
            g.addColorStop(1, cols[1]);
            ctx.fillStyle = g;
            ctx.fillRect(0, 0, W, H);
            const f = o.t;
            const title = scene ? (typeof f === 'function' ? f(scene.titleKey) : '') : '';
            const text = title && title !== scene.titleKey ? title : (scene ? scene.id : '');
            ctx.save();
            ctx.globalAlpha = 0.85;
            ctx.fillStyle = '#fff6dc';
            ctx.textAlign = 'center';
            ctx.textBaseline = 'middle';
            ctx.font = 'bold ' + Math.max(14, Math.round(H * 0.06)) + 'px ' + (o.font || '"Segoe UI", system-ui, sans-serif');
            ctx.fillText(text, W / 2, H * 0.4);
            ctx.restore();
        }

        function drawPanel(scene, panel, key, panelT, alpha, nowS, reduced, W, H) {
            ctx.save();
            ctx.globalAlpha = alpha;
            const img = image(panel.art);
            const iw = img ? img.w : W;
            const ih = img ? img.h : H;
            const p = clamp(panelT / (Cut() ? Cut().PANEL_SECONDS : 9), 0, 1);
            const c = panCenter(panel, p, reduced);
            const cam = camera(W, H, iw, ih, c[0], c[1], c[2]);
            if (img) {
                ctx.imageSmoothingEnabled = cam.s < 1;
                ctx.drawImage(img.img, img.x + cam.sx, img.y + cam.sy, W / cam.s, H / cam.s, 0, 0, W, H);
            } else {
                drawFallback(scene, W, H);
            }
            const A = Art();
            const atlas = A && A.current ? A.current() : null;
            const X = function (u) { return (u * iw - cam.sx) * cam.s; };
            const Y = function (v) { return (v * ih - cam.sy) * cam.s; };
            // sprite actors
            if (atlas && panel.sprites) {
                const lp = reduced ? 0.5 : p;
                for (let i = 0; i < panel.sprites.length; i++) {
                    const sp = panel.sprites[i];
                    if (!A.has(atlas, sp.clip)) continue;
                    const dur = A.duration(atlas, sp.clip);
                    A.draw(ctx, atlas, sp.clip, reduced || !(dur > 0) ? 0 : nowS % dur,
                        X(lerp(sp.from[0], sp.to[0], lp)), Y(lerp(sp.from[1], sp.to[1], lp)),
                        { height: sp.h * ih * cam.s, flip: sp.flip, pixel: true });
                }
            }
            // glowing lights
            if (panel.lights && panel.lights.length) {
                ctx.save();
                ctx.globalCompositeOperation = 'lighter';
                for (let i = 0; i < panel.lights.length; i++) {
                    const l = panel.lights[i];
                    const sprite = glow(l.color);
                    if (!sprite) continue;
                    const pulse = l.pulse && !reduced ? 0.2 * Math.sin(nowS * 2.2 + i * 1.7) : 0;
                    const r = l.r * ih * cam.s;
                    ctx.globalAlpha = clamp((reduced ? 0.55 : 0.55 + pulse) * alpha, 0, 1);
                    ctx.drawImage(sprite, X(l.x) - r, Y(l.y) - r, r * 2, r * 2);
                    ctx.globalAlpha = clamp((0.4 + pulse) * alpha, 0, 1);
                    ctx.drawImage(sprite, X(l.x) - r * 0.3, Y(l.y) - r * 0.3, r * 0.6, r * 0.6);
                }
                ctx.restore();
            }
            // foreground layer, parallax 1.6x
            const fg = panel.fg ? image(panel.fg) : null;
            if (fg) {
                const fc = [0.5 + (c[0] - 0.5) * FG_PARALLAX, 0.5 + (c[1] - 0.5) * FG_PARALLAX, 1 + (c[2] - 1) * FG_PARALLAX];
                const fcam = camera(W, H, fg.w, fg.h, fc[0], fc[1], fc[2]);
                ctx.imageSmoothingEnabled = fcam.s < 1;
                ctx.drawImage(fg.img, fg.x + fcam.sx, fg.y + fcam.sy, W / fcam.s, H / fcam.s, 0, 0, W, H);
            }
            if (!reduced && panel.fx && panel.fx.length) particles(key, panel.fx, W, H, nowS);
            ctx.restore();
        }

        // ---- frame ----
        function finish(W, H) {
            // dark gradient under the subtitle
            const g = ctx.createLinearGradient(0, H * 0.55, 0, H);
            g.addColorStop(0, 'rgba(4,3,8,0)');
            g.addColorStop(1, 'rgba(4,3,8,0.82)');
            ctx.fillStyle = g;
            ctx.fillRect(0, H * 0.55, W, H * 0.45);
            // soft vignette
            const v = ctx.createRadialGradient(W / 2, H / 2, Math.min(W, H) * 0.35, W / 2, H / 2, Math.hypot(W, H) * 0.55);
            v.addColorStop(0, 'rgba(0,0,0,0)');
            v.addColorStop(1, 'rgba(0,0,0,0.5)');
            ctx.fillStyle = v;
            ctx.fillRect(0, 0, W, H);
            // cinematic bars
            ctx.fillStyle = '#000';
            ctx.fillRect(0, 0, W, Math.round(H * BAR));
            ctx.fillRect(0, H - Math.round(H * BAR), W, Math.round(H * BAR));
        }

        function render(state, nowMs, reducedMotion) {
            try {
                if (disposed || !ctx) return;
                const W = canvas.width;
                const H = canvas.height;
                if (!(W > 0 && H > 0)) return;
                const C = Cut();
                const cur = C ? C.current(state) : null;
                ctx.setTransform(1, 0, 0, 1, 0, 0);
                ctx.globalAlpha = 1;
                ctx.globalCompositeOperation = 'source-over';
                ctx.fillStyle = '#000';
                ctx.fillRect(0, 0, W, H);
                if (!cur) return;
                const reduced = !!reducedMotion;
                const now = +nowMs || 0;
                const nowS = now / 1000;
                const key = cur.scene.id + ':' + state.panelIndex;
                // keep the sheets of the current and the following panels coming
                requestPanel(cur.panel);
                const nextPanel = cur.scene.panels[state.panelIndex + 1];
                if (nextPanel) requestPanel(nextPanel);
                else if (C && C.SCENES[state.ids[state.sceneIndex + 1]]) requestPanel(C.SCENES[state.ids[state.sceneIndex + 1]].panels[0]);
                if (key !== lastKey) {
                    prev = last && !reduced ? { scene: last.scene, panel: last.panel, key: last.key, t0: lastT, at: now } : null;
                    fadeAt = now;
                    lastKey = key;
                }
                last = { scene: cur.scene, panel: cur.panel, key: key };
                lastT = state.panelT;
                const fade = prev ? (now - fadeAt) / (C.FADE_SECONDS * 1000) : 1;
                if (prev && fade < 1 && fade >= 0) {
                    drawPanel(prev.scene, prev.panel, prev.key, prev.t0 + (now - prev.at) / 1000, 1, nowS, reduced, W, H);
                    drawPanel(cur.scene, cur.panel, key, state.panelT, ease(fade), nowS, reduced, W, H);
                } else {
                    prev = null;
                    drawPanel(cur.scene, cur.panel, key, state.panelT, 1, nowS, reduced, W, H);
                }
                finish(W, H);
            } catch (err) { /* a broken frame must never break the game */ }
        }

        function dispose() {
            disposed = true;
            prev = null;
            last = null;
            [glows, layouts, requested].forEach(function (m) { Object.keys(m).forEach(function (k) { delete m[k]; }); });
            mistSprite = null;
        }

        return { prime: prime, render: render, dispose: dispose };
    }

    root.RunenpfadCutsceneView = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
