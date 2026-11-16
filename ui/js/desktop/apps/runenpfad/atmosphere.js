/* Runenpfad atmosphere painter: parallax backdrops, weather, 2D light map, colour grade and vignette. */
(function (root) {
    'use strict';

    // Deterministic value noise so a biome silhouette is stable across frames.
    function hash1(n, seed) {
        let h = Math.imul(n ^ seed, 0x27d4eb2d);
        h ^= h >>> 15;
        h = Math.imul(h, 0x85ebca6b);
        h ^= h >>> 13;
        return (h >>> 0) / 4294967296;
    }

    function noise1(x, seed) {
        const i = Math.floor(x);
        const f = x - i;
        const u = f * f * (3 - 2 * f);
        return hash1(i, seed) * (1 - u) + hash1(i + 1, seed) * u;
    }

    function clamp(v, lo, hi) {
        return v < lo ? lo : v > hi ? hi : v;
    }

    // Darkens a hex colour so crest props read as silhouettes against their own band.
    function shade(hex, amount) {
        const m = /^#([0-9a-f]{6})$/i.exec(String(hex || ''));
        if (!m) return hex;
        const n = parseInt(m[1], 16);
        const r = Math.max(0, Math.round(((n >> 16) & 255) * (1 - amount)));
        const g = Math.max(0, Math.round(((n >> 8) & 255) * (1 - amount)));
        const b = Math.max(0, Math.round((n & 255) * (1 - amount)));
        return '#' + (((r << 16) | (g << 8) | b).toString(16)).padStart(6, '0');
    }

    // Crest height (screen px) of a parallax silhouette at world x.
    // `lift` is the offset above the ground line (negative = above), `amp` the crest amplitude in px.
    function crestAt(style, wx, layer, groundY, viewH) {
        const freq = layer.freq;
        const amp = layer.amp;
        const base = groundY + (layer.lift || 0) * viewH;
        const seed = layer.seed;
        const n = noise1(wx * freq, seed) * 0.66 + noise1(wx * freq * 2.7 + 13.7, seed + 7) * 0.34;
        if (style === 'spire' || style === 'ice') {
            const sharp = Math.max(0, (n - 0.4) / 0.6);
            return base - amp * sharp * sharp;
        }
        if (style === 'crystal') {
            return base - amp * (Math.floor(n * 4) / 4);
        }
        if (style === 'gear') {
            return base - amp * (Math.floor(n * 5) / 5);
        }
        if (style === 'dune') {
            const wave = Math.sin(wx * freq * 0.55 + seed) * 0.5 + 0.5;
            return base - amp * (wave * 0.55 + n * 0.45);
        }
        return base - amp * n;
    }

    function createAtmosphere(ctx) {
        let lightCanvas = null;
        let lightCtx = null;
        let weatherSeed = 1;

        function dpr() {
            return Math.min(2, root.devicePixelRatio || 1);
        }

        function ensureLightCanvas(w, h) {
            const doc = ctx.canvas && ctx.canvas.ownerDocument;
            if (!doc || typeof doc.createElement !== 'function') return null;
            if (!lightCanvas) {
                lightCanvas = doc.createElement('canvas');
                lightCtx = lightCanvas.getContext('2d');
            }
            if (!lightCtx) return null;
            if (lightCanvas.width !== w || lightCanvas.height !== h) {
                lightCanvas.width = w;
                lightCanvas.height = h;
            }
            return lightCtx;
        }

        function paintCelestial(biome, viewW, viewH) {
            const c = biome.celestial;
            if (!c) return;
            const cx = viewW * c.x;
            const cy = viewH * c.y;
            ctx.save();
            const halo = ctx.createRadialGradient(cx, cy, c.radius * 0.4, cx, cy, c.radius * 3.2);
            halo.addColorStop(0, c.glow);
            halo.addColorStop(1, 'rgba(0,0,0,0)');
            ctx.globalAlpha = c.alpha * 0.5;
            ctx.fillStyle = halo;
            ctx.beginPath();
            ctx.arc(cx, cy, c.radius * 3.2, 0, Math.PI * 2);
            ctx.fill();
            ctx.globalAlpha = c.alpha;
            ctx.fillStyle = c.color;
            ctx.beginPath();
            ctx.arc(cx, cy, c.radius, 0, Math.PI * 2);
            ctx.fill();
            ctx.restore();
        }

        // Silhouette decorations that sit on the crest of a parallax band.
        function paintCrestProps(style, layer, camX, viewW, viewH, groundY) {
            if (style === 'forest') {
                const step = 34;
                const start = Math.floor(camX * layer.factor / step) * step;
                ctx.fillStyle = shade(layer.color, 0.28);
                for (let wx = start; wx < camX * layer.factor + viewW + step; wx += step) {
                    const jitter = hash1(wx | 0, layer.seed) * 16;
                    const treeH = 16 + jitter;
                    const treeW = 6 + hash1(wx | 0, layer.seed + 3) * 5;
                    const sx = wx - camX * layer.factor;
                    const sy = crestAt(style, wx, layer, groundY, viewH);
                    ctx.beginPath();
                    ctx.moveTo(sx, sy + 2);
                    ctx.lineTo(sx - treeW, sy + treeH);
                    ctx.lineTo(sx + treeW, sy + treeH);
                    ctx.closePath();
                    ctx.fill();
                }
            } else if (style === 'gear') {
                const step = 96;
                const start = Math.floor(camX * layer.factor / step) * step;
                ctx.fillStyle = shade(layer.color, 0.28);
                for (let wx = start; wx < camX * layer.factor + viewW + step; wx += step) {
                    const r = 14 + hash1(wx | 0, layer.seed) * 12;
                    const sx = wx - camX * layer.factor;
                    const sy = crestAt(style, wx, layer, groundY, viewH) + r * 0.4;
                    ctx.beginPath();
                    ctx.arc(sx, sy, r, 0, Math.PI * 2);
                    ctx.fill();
                    ctx.fillRect(sx - 2, sy - r * 0.5, 4, r * 1.5);
                }
            } else if (style === 'crystal') {
                const step = 58;
                const start = Math.floor(camX * layer.factor / step) * step;
                ctx.fillStyle = shade(layer.color, 0.28);
                for (let wx = start; wx < camX * layer.factor + viewW + step; wx += step) {
                    const h = 10 + hash1(wx | 0, layer.seed) * 22;
                    const w = 3 + hash1(wx | 0, layer.seed + 5) * 4;
                    const sx = wx - camX * layer.factor;
                    const sy = crestAt(style, wx, layer, groundY, viewH);
                    ctx.beginPath();
                    ctx.moveTo(sx, sy - h);
                    ctx.lineTo(sx + w, sy + 2);
                    ctx.lineTo(sx - w, sy + 2);
                    ctx.closePath();
                    ctx.fill();
                }
            }
        }

        function paintLayer(layer, camX, viewW, viewH, groundY) {
            const step = 12;
            const offset = camX * layer.factor;
            ctx.fillStyle = layer.color;
            ctx.beginPath();
            ctx.moveTo(0, viewH);
            for (let x = 0; x <= viewW + step; x += step) {
                const wx = x + offset;
                const y = crestAt(layer.style, wx, layer, groundY, viewH);
                ctx.lineTo(x, y);
            }
            ctx.lineTo(viewW, viewH);
            ctx.closePath();
            ctx.fill();
            paintCrestProps(layer.style, layer, camX, viewW, viewH, groundY);
        }

        function paintBackdrop(biome, camX, viewW, viewH, groundY) {
            const sky = ctx.createLinearGradient(0, 0, 0, viewH);
            const stops = biome.sky || ['#101018', '#202030'];
            for (let i = 0; i < stops.length; i++) {
                sky.addColorStop(stops.length === 1 ? 0 : i / (stops.length - 1), stops[i]);
            }
            ctx.fillStyle = sky;
            ctx.fillRect(0, 0, viewW, viewH);

            paintCelestial(biome, viewW, viewH);

            const layers = biome.layers || [];
            for (let i = 0; i < layers.length; i++) {
                paintLayer(layers[i], camX, viewW, viewH, groundY);
            }

            // Horizon haze that separates backdrop from gameplay plane.
            if (biome.fog && biome.fog.alpha > 0) {
                const fogTop = Math.max(0, groundY - viewH * 0.75);
                const fog = ctx.createLinearGradient(0, fogTop, 0, viewH);
                fog.addColorStop(0, 'rgba(0,0,0,0)');
                fog.addColorStop(1, biome.fog.color);
                ctx.save();
                ctx.globalAlpha = biome.fog.alpha;
                ctx.fillStyle = fog;
                ctx.fillRect(0, fogTop, viewW, viewH - fogTop);
                ctx.restore();
            }
        }

        function paintWeather(biome, viewW, viewH, timeSec, densityScale) {
            const w = biome.weather;
            if (!w || w.density <= 0) return;
            const count = Math.max(6, Math.round(w.density * (densityScale || 1)));
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
                    ctx.ellipse(0, 0, size * 1.7, size * 0.7, 0, 0, Math.PI * 2);
                    ctx.fill();
                    ctx.restore();
                } else if (kind === 'ember') {
                    ctx.shadowBlur = 8;
                    ctx.shadowColor = w.color;
                    ctx.beginPath();
                    ctx.arc(sx, sy, size * 0.8, 0, Math.PI * 2);
                    ctx.fill();
                    ctx.shadowBlur = 0;
                } else if (kind === 'sand') {
                    ctx.fillRect(sx, sy, size * 3.4, size * 0.7);
                } else {
                    ctx.beginPath();
                    ctx.arc(sx, sy, size, 0, Math.PI * 2);
                    ctx.fill();
                }
            }
            ctx.restore();
        }

        // Real 2D light map: darkness is painted, then light sources punch holes out of it.
        function paintLighting(biome, camX, camY, viewW, viewH, lights, timeSec) {
            const amb = biome.light ? biome.light.ambient : 0;
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
                lc.arc(sx, sy, r, 0, Math.PI * 2);
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

        return {
            paintBackdrop: paintBackdrop,
            paintWeather: paintWeather,
            paintLighting: paintLighting,
            paintGrade: paintGrade,
            resetWeather: resetWeather
        };
    }

    root.RunenpfadAtmosphere = {
        createAtmosphere: createAtmosphere,
        noise1: noise1,
        hash1: hash1
    };
})(typeof window !== 'undefined' ? window : globalThis);
