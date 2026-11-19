/* Runenpfad painted parallax: bakes the lazily loaded bg-* atlas sheets (far band, ceiling band, mid and near props)
   into cached periodic strips with depth haze. Canvas-facing, optional: until the sheets are decoded (or if they fail)
   the procedural silhouette layers of atmosphere.js stay in charge. Per frame it only blits. */
(function (root) {
    'use strict';

    const B = root.RunenpfadBackdrops || {};
    const P = B.P || 2048; // prop strips repeat every P css px (far/ceiling strips use their own mirrored period)
    const hash1 = B.hash1;
    const mix = B.mix;
    const rgba = B.rgba;
    const buildCrest = B.buildCrest;
    const crestH = B.crestH;
    const BUCKET = 48; // view heights are quantised so a resize does not rebake every frame
    const FADE_SEC = 0.7;
    const MAX_PERIOD = 6200;

    function clamp(v, lo, hi) {
        return v < lo ? lo : v > hi ? hi : v;
    }

    function createParallax(ctx, makeCanvas) {
        let set = null; // { key, layers[] }
        const loads = {}; // biome id -> { atlas, ready, failed }
        const readyAt = {};

        function atlas() {
            const A = root.RunenpfadArt;
            return A && typeof A.current === 'function' ? A.current() : null;
        }

        function release(cv) {
            if (cv) { cv.width = 1; cv.height = 1; }
        }

        // Starts the lazy sheet load for the biome once; true when the painted layers can be baked.
        function request(biome) {
            const spec = biome.art;
            const a = atlas();
            const A = root.RunenpfadArt;
            if (!spec || !a || !A || typeof A.loadSheet !== 'function') return false;
            let st = loads[biome.id];
            if (!st || st.atlas !== a) {
                st = { atlas: a, ready: false, failed: false };
                loads[biome.id] = st;
                const names = Object.keys(spec.sheets).map(function (k) { return spec.sheets[k]; });
                Promise.all(names.map(function (id) { return A.loadSheet(a, id); })).then(function (res) {
                    st.ready = res.some(Boolean);
                    st.failed = !st.ready;
                }).catch(function () { st.failed = true; });
            }
            return st.ready && !st.failed;
        }

        // Dark and haze veil over the already painted pixels only (source-atop), heavier towards the bottom edge.
        function veil(g, w, h, cfg) {
            g.save();
            g.globalCompositeOperation = 'source-atop';
            if (cfg.dark) {
                g.fillStyle = 'rgba(4, 8, 16, ' + cfg.dark + ')';
                g.fillRect(0, 0, w, h);
            }
            if (cfg.haze) {
                const gr = g.createLinearGradient(0, 0, 0, h);
                gr.addColorStop(0, rgba(cfg.color, clamp(cfg.haze * 0.8, 0, 1)));
                gr.addColorStop(1, rgba(cfg.color, clamp(cfg.haze * 1.2, 0, 1)));
                g.fillStyle = gr;
                g.fillRect(0, 0, w, h);
            }
            g.restore();
        }

        function bandFrame(a, id) {
            const clip = a.clips[id + '.band'];
            if (!clip) return null;
            const f = clip.sheet.frames[clip.frames[0]];
            return f && f[2] > 0 ? { img: clip.sheet.img, f: f } : null;
        }

        // Mirror-tiled band: A | mirror(A), so the periodic strip always joins edge to edge.
        function bakeBand(a, sheetId, cfg, vh, d, top) {
            const b = bandFrame(a, sheetId);
            if (!b) return null;
            const f = b.f;
            let w = f[2] * ((cfg.frac * vh) / f[3]);
            if (w * 2 > MAX_PERIOD) w = MAX_PERIOD / 2;
            const period = Math.max(2, Math.round(w * 2));
            w = period / 2;
            const h = Math.max(2, Math.round(f[3] * (w / f[2])));
            const cv = makeCanvas(period * d, h * d);
            if (!cv) return null;
            const g = cv.getContext('2d');
            g.scale(d, d);
            g.imageSmoothingEnabled = true;
            g.imageSmoothingQuality = 'high';
            g.drawImage(b.img, f[0], f[1], f[2], f[3], 0, 0, w, h);
            g.save();
            g.translate(period, 0);
            g.scale(-1, 1);
            g.drawImage(b.img, f[0], f[1], f[2], f[3], 0, 0, w, h);
            g.restore();
            veil(g, period, h, cfg);
            const solid = mix(cfg.color, '#06080e', 0.35);
            if (!top) {
                // Dissolve the painted ridge into the same fill used below the band.
                const fadeY = Math.floor(h * 0.7);
                const fade = g.createLinearGradient(0, fadeY, 0, h);
                fade.addColorStop(0, rgba(solid, 0));
                fade.addColorStop(0.45, rgba(solid, 0.34));
                fade.addColorStop(0.82, rgba(solid, 0.88));
                fade.addColorStop(1, solid);
                g.fillStyle = fade;
                g.fillRect(0, fadeY, period, h - fadeY);
            }
            return {
                canvas: cv, d: d, period: period, h: h, span: h, factor: cfg.factor, lift: cfg.lift || 0, top: !!top,
                solid: solid, fill: !top
            };
        }

        // Seeded prop layout over one period: varied spacing and scale, no repeated neighbour, occasional flip.
        function layout(cfg, frames, vs, capH) {
            const n = frames.length;
            const out = [];
            const seed = cfg.seed || 1;
            const g0 = cfg.gap[0];
            const g1 = cfg.gap[1];
            let x = hash1(1, seed) * g0 * vs;
            let prev = -1;
            let prev2 = -1;
            for (let k = 0; x < P && k < 64; k++) {
                let idx = -1;
                for (let t = 0; t < 8; t++) {
                    const c = Math.floor(hash1(k * 11 + t, seed + 5) * n);
                    if (c !== prev && c !== prev2) { idx = c; break; }
                }
                if (idx < 0) idx = (prev + 1) % n;
                const f = frames[idx];
                const sc = Math.min(cfg.scale * vs * (0.75 + 0.45 * hash1(k, seed + 9)), capH / f[3]);
                out.push({ x: x, idx: idx, sc: sc, flip: hash1(k, seed + 13) > 0.62 });
                prev2 = prev;
                prev = idx;
                x += (g0 + (g1 - g0) * hash1(k, seed + 17)) * vs;
            }
            const last = out.length - 1;
            if (last > 0 && out[0].idx === out[last].idx) out[last].idx = (out[last].idx + 1) % n;
            return out;
        }

        function drawProp(g, img, f, x, y, sc, flip) {
            g.save();
            g.translate(x, y);
            if (flip) g.scale(-1, 1);
            g.drawImage(img, f[0], f[1], f[2], f[3], f[4] * sc, f[5] * sc, f[2] * sc, f[3] * sc);
            g.restore();
        }

        function bakeProps(a, sheetId, cfg, vh, d, near) {
            const frames = [];
            const clips = [];
            for (let i = 0; i < 8; i++) {
                const clip = a.clips[sheetId + '.p' + i];
                if (!clip || (cfg.skip && cfg.skip.indexOf(i) >= 0)) continue;
                const f = clip.sheet.frames[clip.frames[0]];
                if (f && f[2] > 0) { frames.push(f); clips.push(clip); }
            }
            if (!frames.length) return null;
            const vs = clamp(vh / 720, 0.8, 1.7);
            const items = layout(cfg, frames, vs, (near ? 0.3 : 0.46) * vh);
            let maxH = 0;
            items.forEach(function (it) { maxH = Math.max(maxH, frames[it.idx][3] * it.sc); });
            const amp = cfg.amp || 0;
            const span = Math.ceil(maxH + amp + 10);
            const H = span + 24;
            const img = clips[0].sheet.img;
            const hs = amp > 0 ? buildCrest({ base: 'hills', cells: cfg.cells || 8, amp: amp, seed: cfg.seed || 1 }) : null;
            const cv = makeCanvas(P * d, H * d);
            const pc = makeCanvas(P * d, H * d);
            if (!cv || !pc) { release(cv); release(pc); return null; }
            const g = cv.getContext('2d');
            const pg = pc.getContext('2d');
            g.scale(d, d);
            pg.scale(d, d);
            pg.imageSmoothingEnabled = true;
            pg.imageSmoothingQuality = 'high';
            const bottom = mix(cfg.body, cfg.color, 0.35);
            const grad = g.createLinearGradient(0, Math.max(0, span - amp - 10), 0, span + 24);
            // Let the distant painting show through the crest beneath the grounded props.
            grad.addColorStop(0, rgba(cfg.body, 0));
            grad.addColorStop(0.6, rgba(cfg.body, 0.48));
            grad.addColorStop(1, bottom);
            g.fillStyle = grad;
            g.beginPath();
            g.moveTo(0, H);
            if (hs) for (let i = 0; i <= P / 4; i++) g.lineTo(i * 4, span - hs[i]);
            else g.lineTo(0, span);
            g.lineTo(P, hs ? span - hs[P / 4] : span);
            g.lineTo(P, H);
            g.closePath();
            g.fill();
            items.slice().sort(function (p, q) { return p.sc - q.sc; }).forEach(function (it) {
                const f = frames[it.idx];
                const hw = (f[2] * it.sc) / 2;
                [it.x, it.x - P, it.x + P].forEach(function (wx) {
                    if (wx + hw < 0 || wx - hw > P) return;
                    const feet = hs ? span - crestH(hs, wx) + 4 : span + 1;
                    drawProp(pg, img, f, wx, feet, it.sc, it.flip);
                });
            });
            veil(pg, P, H, cfg);
            g.drawImage(pc, 0, 0, P * d, H * d, 0, 0, P, H);
            release(pc);
            return { canvas: cv, d: d, period: P, h: H, span: span, factor: cfg.factor, lift: cfg.lift || 0, top: false, solid: bottom, fill: true };
        }

        function bake(biome, vh, d) {
            const spec = biome.art;
            const a = atlas();
            const layers = [];
            if (!spec || !a) return layers;
            const add = function (l) { if (l) layers.push(l); };
            if (spec.far) add(bakeBand(a, spec.sheets.far, spec.far, vh, d, false));
            if (spec.ceiling) add(bakeBand(a, spec.sheets.ceiling, spec.ceiling, vh, d, true));
            if (spec.mid) add(bakeProps(a, spec.sheets.mid, spec.mid, vh, d, false));
            if (spec.near) add(bakeProps(a, spec.sheets.near, spec.near, vh, d, true));
            layers.sort(function (p, q) { return p.factor - q.factor; });
            return layers;
        }

        function purge() {
            if (set) set.layers.forEach(function (l) { release(l.canvas); });
            set = null;
        }

        // Painted layers fade in over FADE_SEC once ready; returns 0 while the procedural fallback must carry the scene.
        function prepare(biome, viewH, d, t) {
            try {
                if (!request(biome)) return 0;
                const bucket = Math.max(8, Math.round(viewH / BUCKET));
                const key = biome.id + '|' + d + '|' + bucket;
                if (!set || set.key !== key) {
                    purge();
                    set = { key: key, layers: bake(biome, bucket * BUCKET, d) };
                }
                if (!set.layers.length) return 0;
                if (readyAt[biome.id] === undefined || t < readyAt[biome.id]) readyAt[biome.id] = t;
                return clamp((t - readyAt[biome.id]) / FADE_SEC, 0, 1) || 0.001;
            } catch (err) {
                purge();
                const st = loads[biome.id];
                if (st) st.failed = true;
                return 0;
            }
        }

        function blit(l, off, viewW, y0, alpha) {
            const cv = l.canvas;
            const d = l.d;
            const W = cv.width;
            const need = Math.ceil(viewW * d);
            let srcX = Math.floor((((off % l.period) + l.period) % l.period) * d);
            let dx = 0;
            if (alpha < 1) ctx.globalAlpha = alpha;
            while (dx < need) {
                const w = Math.min(W - srcX, need - dx);
                if (w <= 0) break;
                ctx.drawImage(cv, srcX, 0, w, cv.height, dx / d, y0, w / d, cv.height / d);
                dx += w;
                srcX = 0;
            }
            if (alpha < 1) ctx.globalAlpha = 1;
        }

        // Blit-only per frame. `afterFirst` runs once the first (farthest) layer is down (light shafts go there).
        function paint(camX, viewW, viewH, groundY, alpha, afterFirst) {
            if (!set) return;
            for (let i = 0; i < set.layers.length; i++) {
                const l = set.layers[i];
                if (!l.canvas) continue;
                const off = camX * l.factor;
                if (l.top) {
                    blit(l, off, viewW, 0, alpha);
                } else {
                    const base = Math.round(groundY + l.lift * viewH);
                    const y0 = base - l.span;
                    const fillTop = y0 + l.h;
                    if (l.fill && fillTop < viewH) {
                        ctx.globalAlpha = alpha;
                        ctx.fillStyle = l.solid;
                        ctx.fillRect(0, fillTop - 1, viewW, viewH - fillTop + 1);
                        ctx.globalAlpha = 1;
                    }
                    blit(l, off, viewW, y0, alpha);
                }
                if (i === 0 && afterFirst) afterFirst();
            }
        }

        function dispose() {
            purge();
        }

        return { prepare: prepare, paint: paint, dispose: dispose };
    }

    root.RunenpfadParallaxArt = { createParallax: createParallax };
})(typeof window !== 'undefined' ? window : globalThis);
