/* Runenpfad atmosphere painter: cached parallax strips, clouds, weather, 2D light map and colour grade. */
(function (root) {
    'use strict';


    const B = root.RunenpfadBackdrops || {};
    const P = B.P || 2048;
    const TAU = Math.PI * 2;
    const hash1 = B.hash1;
    const noise1 = B.noise1;
    const clamp = B.clamp;
    const mix = B.mix;
    const rgba = B.rgba;
    const wrapDraw = B.wrapDraw;
    const buildCrest = B.buildCrest;
    const PROPS = B.PROPS || {};
    const decoratePeaks = B.decoratePeaks;
    const drawBody = B.drawBody;
    const propsCeiling = B.propsCeiling;


    function lightning(t) {
        const E = 9;
        const ep = Math.floor(t / E);
        const rr = hash1(ep, 77);
        if (rr < 0.4) return null;
        const dt = t - (ep * E + 2 + rr * 4.5);
        if (dt < 0 || dt > 0.7) return null;
        const a = dt < 0.07 ? 1 : dt < 0.14 ? 0.15 : dt < 0.22 ? 0.9 : Math.max(0, 0.5 * (1 - (dt - 0.22) / 0.48));
        return { a: a, bolt: dt < 0.24, x: hash1(ep, 91), seed: ep };
    }

    function createAtmosphere(ctx) {
        let lightCanvas = null;
        let lightCtx = null;
        let weatherSeed = 1;
        let stripSet = null; // { key, layers[] }
        let parallax = null; // painted parallax strips (parallax-art.js), created on first use
        let sky = { key: '', canvas: null };
        const small = {}; // cached helper canvases (clouds, shafts, aurora, foreground, fog)

        function dpr() {
            return Math.min(2, root.devicePixelRatio || 1);
        }

        function makeCanvas(w, h) {
            const doc = ctx.canvas && ctx.canvas.ownerDocument;
            if (!doc || typeof doc.createElement !== 'function') return null;
            const cv = doc.createElement('canvas');
            cv.width = Math.max(1, Math.round(w));
            cv.height = Math.max(1, Math.round(h));
            return cv;
        }

        function release(cv) {
            if (cv) { cv.width = 1; cv.height = 1; }
        }

        function ensureLightCanvas(w, h) {
            if (!lightCanvas) {
                lightCanvas = makeCanvas(w, h);
                lightCtx = lightCanvas ? lightCanvas.getContext('2d') : null;
            }
            if (!lightCtx) return null;
            if (lightCanvas.width !== w || lightCanvas.height !== h) {
                lightCanvas.width = w;
                lightCanvas.height = h;
            }
            return lightCtx;
        }

        function bakeStrip(w, h, d, draw) {
            const cv = makeCanvas(w * d, h * d);
            if (!cv) return null;
            const g = cv.getContext('2d');
            g.scale(d, d);
            draw(g);
            return cv;
        }

        function bakeLayer(biome, L, d) {
            const top = L.anchor === 'top';
            const span = L.span || 200;
            const H = top ? span : span + 24;
            const entry = { L: L, canvas: null, d: d, h: H, solid: L.color, tips: [], smoke: [], top: top };
            entry.canvas = bakeStrip(P, H, d, function (g) {
                const env = {
                    c: g, L: L, biome: biome, entry: entry, span: span,
                    hs: top ? null : buildCrest(L),
                    r: function (i, s) { return hash1(i * 977 + (s || 0) * 7919 + 1, L.seed); }
                };
                if (!top) {
                    drawBody(env);
                    if (L.base === 'peaks') decoratePeaks(env);
                }
                const painter = PROPS[L.style];
                if (painter) painter(env);
                if (L.style === 'ceiling') propsCeiling(env);
            });
            return entry;
        }

        function getLayers(biome) {
            const d = dpr();
            const key = biome.id + '|' + d;
            if (stripSet && stripSet.key === key) return stripSet.layers;
            purge();
            const layers = [];
            (biome.layers || []).forEach(function (L) { layers.push(bakeLayer(biome, L, d)); });
            stripSet = { key: key, layers: layers };
            return layers;
        }

        function painted() {
            if (!parallax && root.RunenpfadParallaxArt && root.RunenpfadParallaxArt.createParallax) parallax = root.RunenpfadParallaxArt.createParallax(ctx, makeCanvas);
            return parallax;
        }

        function purge() {
            if (stripSet) stripSet.layers.forEach(function (e) { release(e.canvas); });
            stripSet = null;
            Object.keys(small).forEach(function (k) { release(small[k].canvas); delete small[k]; });
        }

        function cached(key, w, h, d, draw) {
            let e = small[key];
            if (!e) {
                e = { canvas: bakeStrip(w, h, d, draw), w: w, h: h };
                small[key] = e;
            }
            return e;
        }

        function blit(cv, d, offCss, viewW, dy, alpha, flipY) {
            if (!cv) return;
            const W = cv.width;
            const H = cv.height;
            const need = Math.ceil(viewW * d);
            let srcX = Math.floor((((offCss % P) + P) % P) * d);
            let dx = 0;
            if (alpha !== undefined && alpha !== 1) ctx.globalAlpha = alpha;
            while (dx < need) {
                const w = Math.min(W - srcX, need - dx);
                if (w <= 0) break;
                ctx.drawImage(cv, srcX, 0, w, H, dx / d, dy, w / d, H / d);
                dx += w;
                srcX = 0;
            }
            if (alpha !== undefined && alpha !== 1) ctx.globalAlpha = 1;
        }

        function getSky(biome, viewW, viewH, d) {
            const key = biome.id + '|' + viewW + '|' + viewH + '|' + d;
            if (sky.key === key) return sky.canvas;
            release(sky.canvas);
            const cv = makeCanvas(viewW * d, viewH * d);
            sky = { key: key, canvas: cv };
            if (!cv) return null;
            const g = cv.getContext('2d');
            g.scale(d, d);
            const stops = biome.sky || ['#101018', '#202030'];
            const grad = g.createLinearGradient(0, 0, 0, viewH);
            for (let i = 0; i < stops.length; i++) grad.addColorStop(stops.length === 1 ? 0 : i / (stops.length - 1), stops[i]);
            g.fillStyle = grad;
            g.fillRect(0, 0, viewW, viewH);
            const hz = stops[stops.length - 1];
            const c = biome.celestial;
            // horizon glow
            g.save();
            g.translate(viewW * (c ? c.x : 0.5), viewH * 0.68);
            g.scale(1, 0.42);
            const hg = g.createRadialGradient(0, 0, 0, 0, 0, viewW * 0.75);
            hg.addColorStop(0, rgba(biome.fog ? biome.fog.color : hz, 0.4));
            hg.addColorStop(1, rgba(biome.fog ? biome.fog.color : hz, 0));
            g.fillStyle = hg;
            g.fillRect(-viewW, -viewH * 2, viewW * 2, viewH * 4);
            g.restore();
            if (biome.stars) {
                for (let i = 18; i < biome.stars.count; i++) {
                    const sx = hash1(i, 501) * viewW;
                    const sy = hash1(i, 502) * viewH * 0.55;
                    g.fillStyle = rgba(biome.stars.color || '#ffffff', biome.stars.alpha * (0.25 + hash1(i, 503) * 0.6));
                    const sz = 0.7 + hash1(i, 504) * 1.1;
                    g.fillRect(sx, sy, sz, sz);
                }
            }
            if (c) {
                const cx = viewW * c.x;
                const cy = viewH * c.y;
                const hr = c.radius * (c.halo || 3.2);
                const halo = g.createRadialGradient(cx, cy, c.radius * 0.5, cx, cy, hr);
                halo.addColorStop(0, rgba(c.glow, 0.75 * c.alpha));
                halo.addColorStop(0.3, rgba(c.glow, 0.28 * c.alpha));
                halo.addColorStop(1, rgba(c.glow, 0));
                g.fillStyle = halo;
                g.fillRect(cx - hr, cy - hr, hr * 2, hr * 2);
                if (c.halo) {
                    g.strokeStyle = rgba(c.glow, 0.16);
                    g.lineWidth = 2;
                    g.beginPath();
                    g.arc(cx, cy, c.radius * 1.7, 0, TAU);
                    g.stroke();
                    g.beginPath();
                    g.arc(cx, cy, c.radius * 2.4, 0, TAU);
                    g.stroke();
                }
                const disk = g.createRadialGradient(cx - c.radius * 0.3, cy - c.radius * 0.3, 1, cx, cy, c.radius);
                disk.addColorStop(0, '#ffffff');
                disk.addColorStop(1, c.color);
                g.globalAlpha = Math.min(1, c.alpha + 0.4);
                g.fillStyle = disk;
                g.beginPath();
                g.arc(cx, cy, c.radius, 0, TAU);
                g.fill();
                if (c.craters) {
                    g.globalAlpha = 0.14;
                    g.fillStyle = '#3a4a66';
                    for (let k = 0; k < 6; k++) {
                        const a = hash1(k, 61) * TAU;
                        const rr = hash1(k, 62) * c.radius * 0.6;
                        g.beginPath();
                        g.arc(cx + Math.cos(a) * rr, cy + Math.sin(a) * rr, c.radius * (0.08 + hash1(k, 63) * 0.14), 0, TAU);
                        g.fill();
                    }
                }
                g.globalAlpha = 1;
            }
            return cv;
        }

        function sunPosition(biome, camX, viewW, viewH) {
            const c = biome.celestial;
            if (!c || !c.rays) return null;
            return { x: viewW * c.x, y: viewH * c.y, strength: clamp(c.alpha * 0.42, 0.08, 0.36) };
        }

        function paintTwinkle(biome, viewW, viewH, t) {
            const s = biome.stars;
            if (!s) return;
            ctx.save();
            ctx.fillStyle = s.color || '#ffffff';
            const n = Math.min(18, s.count);
            for (let i = 0; i < n; i++) {
                const x = hash1(i, 501) * viewW;
                const y = hash1(i, 502) * viewH * 0.55;
                const tw = 0.5 + 0.5 * Math.sin(t * (1.2 + hash1(i, 7) * 2.2) + i * 1.7);
                ctx.globalAlpha = s.alpha * (0.25 + 0.75 * tw);
                const sz = 1 + hash1(i, 504) * 1.2;
                ctx.fillRect(x, y, sz, sz);
                if (tw > 0.85) {
                    ctx.fillRect(x - 3, y + sz / 2 - 0.3, 6 + sz, 0.7);
                    ctx.fillRect(x + sz / 2 - 0.3, y - 3, 0.7, 6 + sz);
                }
            }
            ctx.restore();
        }

        function paintClouds(biome, camX, viewW, viewH, t, d, groundY) {
            const cl = biome.clouds;
            if (!cl) return;
            const e = cached('clouds|' + biome.id, P, 150, d, function (g) {
                for (let i = 0; i < cl.count; i++) {
                    const x = ((i + hash1(i, 31) * 0.8) * P) / cl.count;
                    const y = 40 + hash1(i, 32) * 60;
                    const sc = (0.7 + hash1(i, 33) * 0.8) * (cl.scale || 1);
                    wrapDraw(x, 160 * sc, function (xx) {
                        for (let k = 0; k < 9; k++) {
                            const ox = (k - 4) * 26 * sc + (hash1(i * 9 + k, 34) - 0.5) * 18;
                            const oy = Math.abs(k - 4) * 3 * sc - hash1(i * 9 + k, 35) * 10 * sc;
                            const rx = (38 + hash1(i * 9 + k, 36) * 40) * sc;
                            const ry = (9 + hash1(i * 9 + k, 37) * 12) * sc;
                            const gr = g.createRadialGradient(xx + ox, y + oy, 1, xx + ox, y + oy, rx);
                            gr.addColorStop(0, rgba(cl.color, 0.6));
                            gr.addColorStop(1, rgba(cl.color, 0));
                            g.fillStyle = gr;
                            g.save();
                            g.translate(xx + ox, y + oy);
                            g.scale(1, ry / rx);
                            g.translate(-(xx + ox), -(y + oy));
                            g.fillRect(xx + ox - rx, y + oy - rx, rx * 2, rx * 2);
                            g.restore();
                        }
                    });
                }
            });
            const y0 = Math.round(cl.y * viewH - 75);
            blit(e.canvas, d, camX * 0.02 + t * cl.speed, viewW, y0, cl.alpha);
            blit(e.canvas, d, camX * 0.035 + P * 0.5 + t * cl.speed * 0.6, viewW, y0 + 34, cl.alpha * 0.7);
        }

        function paintAurora(biome, camX, viewW, viewH, t, d) {
            const au = biome.aurora;
            if (!au) return;
            const e = cached('aurora|' + biome.id, P, 240, d, function (g) {
                // translucent curtains: thin vertical streaks, bright lower hem fading upward
                for (let rb = 0; rb < 3; rb++) {
                    const col = au.colors[rb % au.colors.length];
                    const f1 = 2 + rb;
                    const ph = hash1(rb, 71) * TAU;
                    for (let x = 0; x < P; x += 3) {
                        const yc = 120 + rb * 14 + Math.sin((x / P) * TAU * f1 + ph) * 30 + Math.sin((x / P) * TAU * (f1 * 2 + 1) + ph * 2) * 10;
                        const ht = 100 + 44 * Math.sin((x / P) * TAU * (f1 + 2) + ph) + rb * 12;
                        const streak = 0.35 + 0.65 * hash1(Math.floor(x / 3) + rb * 977, 72);
                        const gr = g.createLinearGradient(0, yc - ht, 0, yc + 10);
                        gr.addColorStop(0, rgba(col, 0));
                        gr.addColorStop(0.75, rgba(col, 0.7 * streak));
                        gr.addColorStop(0.92, rgba(col, 0.88 * streak));
                        gr.addColorStop(1, rgba(col, 0));
                        g.fillStyle = gr;
                        g.fillRect(x, yc - ht, 3, ht + 10);
                    }
                }
            });
            ctx.save();
            ctx.globalCompositeOperation = 'lighter';
            blit(e.canvas, d, camX * 0.015 + t * 7, viewW, Math.round(viewH * 0.1), au.alpha * (0.75 + 0.25 * Math.sin(t * 0.5)));
            ctx.restore();
        }

        function paintShafts(biome, camX, viewW, viewH, t, d) {
            const sh = biome.shafts;
            if (!sh) return;
            const e = cached('shafts|' + biome.id, P, 520, d, function (g) {
                g.save();
                g.filter = 'blur(14px)';
                for (let i = 0; i < sh.count; i++) {
                    const x = ((i + 0.2 + hash1(i, 81) * 0.6) * P) / sh.count;
                    const w0 = 16 + hash1(i, 82) * 26;
                    const w1 = w0 * 2.6;
                    const sk = 120 + hash1(i, 83) * 100;
                    wrapDraw(x, w1 + sk, function (xx) {
                        const gr = g.createLinearGradient(xx, 0, xx + sk, 520);
                        gr.addColorStop(0, rgba(sh.color, 1));
                        gr.addColorStop(1, rgba(sh.color, 0));
                        g.fillStyle = gr;
                        g.beginPath();
                        g.moveTo(xx - w0, 0);
                        g.lineTo(xx + w0, 0);
                        g.lineTo(xx + sk + w1, 520);
                        g.lineTo(xx + sk - w1, 520);
                        g.closePath();
                        g.fill();
                    });
                }
                g.restore();
            });
            blit(e.canvas, d, camX * sh.factor, viewW, 0, sh.alpha * (0.8 + 0.2 * Math.sin(t * 0.35)));
        }

        function copiesAt(px, off, viewW, fn) {
            const base = (((px - off) % P) + P) % P;
            for (let sx = base - P; sx < viewW + 30; sx += P) if (sx > -30) fn(sx);
        }

        function paintDynamic(entry, off, viewW, baseY, t) {
            if (entry.tips.length) {
                ctx.save();
                ctx.fillStyle = '#c9ecff';
                for (let i = 0; i < entry.tips.length; i++) {
                    const tp = entry.tips[i];
                    const T = 2.4 + hash1(tp.n, 3) * 2.6;
                    const ph = (t / T + hash1(tp.n, 4)) % 1;
                    if (ph > 0.6) continue;
                    const f = ph / 0.6;
                    copiesAt(tp.x, off, viewW, function (sx) {
                        ctx.globalAlpha = 0.85 * (1 - f * 0.5);
                        ctx.fillRect(sx - 0.8, tp.y + f * f * 180, 1.6, 4 + f * 4);
                    });
                    copiesAt(tp.x, off, viewW, function (sx) {
                        ctx.globalAlpha = 0.5 * (1 - ph) + 0.15;
                        ctx.beginPath();
                        ctx.arc(sx, tp.y, 2.1, 0, TAU);
                        ctx.fill();
                    });
                }
                ctx.restore();
            }
            if (entry.smoke.length) {
                ctx.save();
                for (let i = 0; i < entry.smoke.length; i++) {
                    const sm = entry.smoke[i];
                    copiesAt(sm.x, off, viewW, function (sx) {
                        for (let k = 0; k < 5; k++) {
                            const ph = (t * 0.1 + k / 5 + hash1(sm.n, 6)) % 1;
                            ctx.globalAlpha = 0.34 * (1 - ph);
                            ctx.fillStyle = '#3d2c40';
                            ctx.beginPath();
                            ctx.arc(sx + Math.sin(ph * 5 + sm.n) * 9 + ph * 34, baseY - sm.h - ph * 130, 5 + ph * 24, 0, TAU);
                            ctx.fill();
                        }
                    });
                }
                ctx.restore();
            }
        }

        function paintBackdrop(biome, camX, viewW, viewH, groundY, timeSec, reducedMotion) {
            const d = dpr();
            const t = timeSec || 0;
            const motionTime = reducedMotion ? 0 : t;
            const skyCv = getSky(biome, viewW, viewH, d);
            if (skyCv) ctx.drawImage(skyCv, 0, 0, viewW, viewH);
            else {
                ctx.fillStyle = biome.sky[0];
                ctx.fillRect(0, 0, viewW, viewH);
            }
            paintTwinkle(biome, viewW, viewH, motionTime);
            paintClouds(biome, camX, viewW, viewH, motionTime, d, groundY);
            // Painted parallax (bg-* sheets) fades in over the procedural silhouettes, which then step aside.
            const pa = biome.art ? painted() : null;
            const fade = pa ? pa.prepare(biome, viewH, d, t) : 0;
            if (fade > 0) paintAurora(biome, camX, viewW, viewH, motionTime, d);
            if (fade < 1) {
                const layers = getLayers(biome);
                for (let i = 0; i < layers.length; i++) {
                    const e = layers[i];
                    if (!e.canvas) continue;
                    const off = camX * e.L.factor;
                    if (e.top) {
                        blit(e.canvas, d, off, viewW, 0);
                    } else {
                        const base = Math.round(groundY + (e.L.lift || 0) * viewH);
                        const y0 = base - e.L.span;
                        blit(e.canvas, d, off, viewW, y0);
                        const fillTop = y0 + e.h;
                        if (fillTop < viewH) {
                            ctx.fillStyle = e.solid;
                            ctx.fillRect(0, fillTop - 1, viewW, viewH - fillTop + 1);
                        }
                        if (e.smoke.length) paintDynamic(e, off, viewW, base, motionTime);
                    }
                    if (e.tips.length) paintDynamic(e, off, viewW, 0, motionTime);
                    if (i === 1 && fade <= 0) paintAurora(biome, camX, viewW, viewH, motionTime, d);
                    if (i === 2 && fade <= 0) paintShafts(biome, camX, viewW, viewH, motionTime, d);
                }
            }
            if (fade > 0) {
                pa.paint(camX, viewW, viewH, groundY, fade, function () { paintShafts(biome, camX, viewW, viewH, motionTime, d); });
            }
            if (biome.fog && biome.fog.alpha > 0) {
                const fogTop = Math.max(0, groundY - viewH * 0.75);
                const fe = cached('fog|' + biome.id, 1, 256, 1, function (g) {
                    const fog = g.createLinearGradient(0, 0, 0, 256);
                    fog.addColorStop(0, rgba(biome.fog.color, 0));
                    fog.addColorStop(1, rgba(biome.fog.color, 1));
                    g.fillStyle = fog;
                    g.fillRect(0, 0, 1, 256);
                });
                if (fe.canvas) {
                    ctx.globalAlpha = biome.fog.alpha;
                    ctx.drawImage(fe.canvas, 0, fogTop, viewW, viewH - fogTop);
                    ctx.globalAlpha = 1;
                }
            }
        }

        function paintRainStreaks(w, viewW, viewH, t, frac, front) {
            const n = Math.round(w.density * frac);
            const slant = 0.2;
            ctx.save();
            ctx.strokeStyle = w.color;
            ctx.globalAlpha = front ? 0.36 : 0.22;
            ctx.lineWidth = front ? 1.5 : 1;
            ctx.beginPath();
            for (let i = 0; i < n; i++) {
                const sp = w.speed * (0.8 + hash1(i, 5) * 0.4) * (front ? 1.15 : 0.9);
                const y = ((hash1(i, 7 + (front ? 50 : 0)) * (viewH + 80) + t * sp) % (viewH + 80)) - 40;
                const x = hash1(i, 3 + (front ? 50 : 0)) * (viewW + 200) - 100 - y * slant;
                const len = (front ? 20 : 13) + hash1(i, 9) * 12;
                ctx.moveTo(x, y);
                ctx.lineTo(x + slant * len, y - len);
            }
            ctx.stroke();
            ctx.restore();
        }

        function paintWeather(biome, viewW, viewH, timeSec, densityScale, groundY, override) {
            const w = override || biome.weather;
            if (!w || w.density <= 0) return;
            const scale = densityScale || 1;
            if (w.kind === 'rain') {
                ctx.fillStyle = 'rgba(6, 14, 30, 0.3)';
                ctx.fillRect(0, 0, viewW, viewH);
                const lg = scale >= 0.9 ? lightning(timeSec) : null;
                if (lg && lg.bolt) {
                    const bx = viewW * (0.15 + lg.x * 0.7);
                    ctx.save();
                    ctx.globalAlpha = lg.a;
                    ctx.strokeStyle = '#e8f2ff';
                    ctx.lineWidth = 2.5;
                    ctx.beginPath();
                    let x = bx;
                    let y = 0;
                    ctx.moveTo(x, y);
                    for (let k = 0; k < 9; k++) {
                        x += (hash1(lg.seed * 13 + k, 5) - 0.5) * 60;
                        y += (groundY || viewH * 0.7) * 0.11;
                        ctx.lineTo(x, y);
                    }
                    ctx.stroke();
                    ctx.restore();
                }
                paintRainStreaks(w, viewW, viewH, timeSec, 0.6 * scale, false);
                return;
            }
            const count = Math.max(6, Math.round(w.density * scale));
            const kind = w.kind;
            ctx.save();
            for (let i = 0; i < count; i++) {
                const seed = i * 37 + weatherSeed;
                const speed = w.speed * (0.55 + hash1(seed, 5) * 0.9);
                let sx = hash1(seed, 11) * (viewW + 60) - 30;
                let sy = hash1(seed, 17) * (viewH + 60) - 30;
                if (kind === 'sand') {
                    sx = ((sx + timeSec * speed * 2.4) % (viewW + 60)) - 30;
                    sy += Math.sin(timeSec * 2 + i) * 6;
                } else if (kind === 'ember') {
                    sy = ((sy - timeSec * speed) % (viewH + 60) + viewH + 60) % (viewH + 60) - 30;
                    sx += Math.sin(timeSec * 1.7 + i) * 9;
                } else if (kind === 'snow') {
                    sy = ((sy + timeSec * speed) % (viewH + 60) + viewH + 60) % (viewH + 60) - 30;
                    sx += Math.sin(timeSec * 0.9 + i) * 13;
                } else if (kind === 'leaf') {
                    sy = ((sy + timeSec * speed * 0.5) % (viewH + 60) + viewH + 60) % (viewH + 60) - 30;
                    sx += Math.sin(timeSec * 1.1 + i) * 16;
                } else {
                    sy = ((sy - timeSec * speed * 0.42) % (viewH + 60) + viewH + 60) % (viewH + 60) - 30;
                    sx += Math.cos(timeSec * 0.8 + i) * 10;
                }
                const size = w.size * (0.6 + hash1(seed, 23) * 0.9);
                ctx.globalAlpha = 0.28 + hash1(seed, 29) * 0.42;
                ctx.fillStyle = hash1(seed, 31) > 0.7 ? (w.color2 || w.color) : w.color;
                if (kind === 'leaf') {
                    ctx.save();
                    ctx.translate(sx, sy);
                    ctx.rotate(timeSec * 1.4 + i);
                    ctx.beginPath();
                    ctx.ellipse(0, 0, size * 1.7, size * 0.7, 0, 0, TAU);
                    ctx.fill();
                    ctx.restore();
                } else if (kind === 'ember') {
                    ctx.shadowBlur = 8;
                    ctx.shadowColor = w.color;
                    ctx.beginPath();
                    ctx.arc(sx, sy, size * 0.8, 0, TAU);
                    ctx.fill();
                    ctx.shadowBlur = 0;
                } else if (kind === 'sand') {
                    ctx.fillRect(sx, sy, size * 3.4, size * 0.7);
                } else {
                    ctx.beginPath();
                    ctx.arc(sx, sy, size, 0, TAU);
                    ctx.fill();
                }
            }
            ctx.restore();
        }

        // Rain in front of the world: streaks, ground splashes and the lightning flash.
        function paintRainFront(w, viewW, viewH, timeSec, groundY, densityScale) {
            const scale = densityScale || 1;
            paintRainStreaks(w, viewW, viewH, timeSec, 0.4 * scale, true);
            ctx.save();
            ctx.strokeStyle = '#d8ebff';
            ctx.lineWidth = 1;
            const gy = Math.min(groundY, viewH - 44);
            const ns = Math.round(26 * scale);
            for (let i = 0; i < ns; i++) {
                const ph = (timeSec * (1.6 + hash1(i, 41)) + hash1(i, 42)) % 1;
                const x = hash1(i + Math.floor(timeSec * 1.6 + hash1(i, 42)) * 31, 43) * viewW;
                ctx.globalAlpha = (1 - ph) * 0.5;
                ctx.beginPath();
                ctx.ellipse(x, gy + 3, 2 + ph * 8, 1 + ph * 2.6, 0, 0, TAU);
                ctx.stroke();
            }
            ctx.restore();
            const lg = scale >= 0.9 ? lightning(timeSec) : null;
            if (lg) {
                ctx.save();
                ctx.globalAlpha = lg.a * 0.3;
                ctx.fillStyle = '#d8e8ff';
                ctx.fillRect(0, 0, viewW, viewH);
                ctx.restore();
            }
        }

        // Real 2D light map: darkness is painted, then light sources punch holes out of it.
        function paintLighting(biome, camX, camY, viewW, viewH, lights, timeSec, ambientOverride) {
            const amb = typeof ambientOverride === 'number' ? ambientOverride : (biome.light ? biome.light.ambient : 0);
            if (amb <= 0.02) return;
            const scale = dpr();
            const lc = ensureLightCanvas(Math.max(1, Math.round(viewW * scale)), Math.max(1, Math.round(viewH * scale)));
            if (!lc) return;
            lc.setTransform(scale, 0, 0, scale, 0, 0);
            lc.globalCompositeOperation = 'source-over';
            lc.clearRect(0, 0, viewW, viewH);
            lc.fillStyle = 'rgba(4, 7, 18, ' + amb + ')';
            lc.fillRect(0, 0, viewW, viewH);
            lc.globalCompositeOperation = 'destination-out';
            const list = lights || [];
            for (let i = 0; i < list.length; i++) {
                const lt = list[i];
                const sx = lt.x - camX;
                const sy = lt.y - camY;
                const r = Math.max(8, lt.radius || 100);
                if (sx < -r || sx > viewW + r || sy < -r || sy > viewH + r) continue;
                const flicker = lt.flicker ? 0.86 + Math.sin(timeSec * 9 + i * 2.1) * 0.09 + Math.sin(timeSec * 23 + i) * 0.05 : 1;
                const a = clamp((lt.intensity == null ? 0.8 : lt.intensity) * flicker, 0, 1);
                const g = lc.createRadialGradient(sx, sy, 0, sx, sy, r);
                g.addColorStop(0, 'rgba(0,0,0,' + a + ')');
                g.addColorStop(0.42, 'rgba(0,0,0,' + (a * 0.52) + ')');
                g.addColorStop(1, 'rgba(0,0,0,0)');
                lc.fillStyle = g;
                lc.beginPath();
                lc.arc(sx, sy, r, 0, TAU);
                lc.fill();
            }
            lc.globalCompositeOperation = 'source-over';
            ctx.drawImage(lightCanvas, 0, 0, viewW, viewH);
        }

        // Warm/cool colour grade plus a soft vignette to focus the play field.
        function paintGrade(biome, viewW, viewH) {
            if (biome.grade && biome.grade.alpha > 0) {
                ctx.save();
                ctx.globalAlpha = biome.grade.alpha;
                ctx.fillStyle = biome.grade.color;
                ctx.fillRect(0, 0, viewW, viewH);
                ctx.restore();
            }
            const g = ctx.createRadialGradient(
                viewW * 0.5, viewH * 0.48, Math.min(viewW, viewH) * 0.32,
                viewW * 0.5, viewH * 0.48, Math.max(viewW, viewH) * 0.78
            );
            g.addColorStop(0, 'rgba(0,0,0,0)');
            g.addColorStop(1, 'rgba(2, 4, 10, 0.52)');
            ctx.fillStyle = g;
            ctx.fillRect(0, 0, viewW, viewH);
        }

        function resetWeather() {
            weatherSeed = (weatherSeed + 1) % 9973;
        }

        function dispose() {
            purge();
            if (parallax) parallax.dispose();
            release(sky.canvas);
            sky = { key: '', canvas: null };
            release(lightCanvas);
            lightCanvas = null;
            lightCtx = null;
        }

        return {
            paintBackdrop: paintBackdrop,
            paintWeather: paintWeather,
            paintRainFront: paintRainFront,
            paintLighting: paintLighting,
            paintGrade: paintGrade,
            sunPosition: sunPosition,
            resetWeather: resetWeather,
            dispose: dispose
        };
    }

    root.RunenpfadAtmosphere = {
        createAtmosphere: createAtmosphere,
        noise1: noise1,
        hash1: hash1
    };
})(typeof window !== 'undefined' ? window : globalThis);
