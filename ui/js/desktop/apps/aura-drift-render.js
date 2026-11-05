/* Aura Drift - Canvas 2D world renderer with streamed chunks and pooled effects. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const CHUNK_PX = 320;

    AD.createCamera = function () {
        return {
            x: 0, y: 0, scale: 5.2, shake: 0, lookX: 0, lookY: 0,
            update(v, dt, quality, reducedMotion) {
                const targetScale = quality.lowZoom ? 5.0 : clamp(7.0 - Math.max(0, v.vx) * 0.02, 5.2, 7.0);
                this.scale = AD.smooth(this.scale, targetScale, 2.4, dt);
                const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
                const lead = reducedMotion ? 0 : clamp(v.vx * 0.55, 0, 42);
                this.lookX = AD.smooth(this.lookX, cos * lead, 3, dt);
                this.lookY = AD.smooth(this.lookY, sin * lead, 3, dt);
                this.x = v.x + this.lookX;
                this.y = v.y + this.lookY;
                this.shake = Math.max(0, this.shake - dt * 2.4);
            }
        };
    };

    function makeChunk(track, cx, cy) {
        const canvas = document.createElement('canvas');
        canvas.width = CHUNK_PX; canvas.height = CHUNK_PX;
        const c = canvas.getContext('2d');
        const biome = track.biome;
        const rng = AD.rng(AD.hashString(track.id + '/' + track.seed + '/' + cx + '/' + cy));
        c.fillStyle = biome.ground;
        c.fillRect(0, 0, CHUNK_PX, CHUNK_PX);

        const patches = 18;
        for (let i = 0; i < patches; i++) {
            const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
            const r = 18 + rng() * 70;
            const g = c.createRadialGradient(x, y, 0, x, y, r);
            const tint = rng() > 0.5 ? biome.groundAlt : biome.edge;
            const a = 0.05 + rng() * 0.07;
            g.addColorStop(0, tint);
            g.addColorStop(1, 'rgba(0,0,0,0)');
            c.globalAlpha = a;
            c.fillStyle = g;
            c.beginPath(); c.arc(x, y, r, 0, Math.PI * 2); c.fill();
        }
        c.globalAlpha = 1;
        const speckles = 420;
        for (let i = 0; i < speckles; i++) {
            const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
            c.fillStyle = rng() > 0.55 ? biome.edge : biome.groundAlt;
            c.globalAlpha = 0.05 + rng() * 0.12;
            c.fillRect(x, y, 1 + rng() * 1.3, 1 + rng() * 1.3);
        }
        c.globalAlpha = 1;
        if (track.biomeId === 'salt') {
            c.strokeStyle = 'rgba(120,130,125,0.16)'; c.lineWidth = 1;
            for (let i = 0; i < 16; i++) {
                c.beginPath();
                let x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
                c.moveTo(x, y);
                for (let j = 0; j < 4; j++) { x += (rng() - 0.5) * 48; y += (rng() - 0.5) * 48; c.lineTo(x, y); }
                c.stroke();
            }
        }
        if (track.biomeId === 'harbour' || track.biomeId === 'industry') {
            c.strokeStyle = 'rgba(140,170,200,0.10)'; c.lineWidth = 2;
            for (let i = 0; i < 6; i++) {
                const x = rng() * CHUNK_PX;
                c.beginPath(); c.moveTo(x, 0); c.lineTo(x, CHUNK_PX); c.stroke();
            }
        }
        if (track.biomeId === 'desert') {
            c.strokeStyle = 'rgba(228,190,130,0.10)'; c.lineWidth = 1.5;
            for (let i = 0; i < 10; i++) {
                const y = rng() * CHUNK_PX;
                c.beginPath();
                c.moveTo(0, y);
                for (let x = 0; x <= CHUNK_PX; x += 24) c.lineTo(x, y + Math.sin(x * 0.05 + rng() * 2) * 5);
                c.stroke();
            }
        }
        return canvas;
    }

    function drawProp(c, prop, sx, sy, scale) {
        const size = prop.size * 3.3 * scale;
        c.save(); c.translate(sx, sy); c.rotate(prop.angle);
        c.fillStyle = 'rgba(6,10,16,0.30)';
        c.beginPath(); c.ellipse(size * 0.16, size * 0.2, size * 0.62, size * 0.4, 0, 0, Math.PI * 2); c.fill();
        switch (prop.kind) {
            case 'alpine':
                c.fillStyle = '#4a3b2c';
                c.fillRect(-size * 0.07, size * 0.1, size * 0.14, size * 0.38);
                c.fillStyle = '#1c3323';
                c.beginPath(); c.moveTo(-size * 0.52, size * 0.22); c.lineTo(0, -size * 0.72); c.lineTo(size * 0.52, size * 0.22); c.closePath(); c.fill();
                c.fillStyle = '#2c5236';
                c.beginPath(); c.moveTo(-size * 0.38, -size * 0.02); c.lineTo(0, -size * 0.62); c.lineTo(size * 0.38, -size * 0.02); c.closePath(); c.fill();
                c.fillStyle = 'rgba(226,238,246,0.5)';
                c.beginPath(); c.moveTo(-size * 0.16, -size * 0.36); c.lineTo(0, -size * 0.62); c.lineTo(size * 0.16, -size * 0.36); c.closePath(); c.fill();
                break;
            case 'desert':
                if (prop.size > 1.1) {
                    c.fillStyle = '#7d5a34';
                    c.beginPath(); c.moveTo(-size * 0.55, size * 0.42); c.lineTo(-size * 0.1, -size * 0.62); c.lineTo(size * 0.58, size * 0.42); c.closePath(); c.fill();
                    c.fillStyle = '#b98a4c';
                    c.beginPath(); c.moveTo(-size * 0.24, size * 0.42); c.lineTo(-size * 0.06, -size * 0.42); c.lineTo(size * 0.16, size * 0.42); c.closePath(); c.fill();
                } else {
                    c.fillStyle = '#4e7c46';
                    c.fillRect(-size * 0.12, -size * 0.5, size * 0.24, size * 0.95);
                    c.fillRect(-size * 0.42, -size * 0.22, size * 0.3, size * 0.16);
                    c.fillRect(size * 0.12, -size * 0.38, size * 0.3, size * 0.16);
                    c.fillStyle = '#6d9c5c';
                    c.fillRect(-size * 0.07, -size * 0.5, size * 0.08, size * 0.95);
                }
                break;
            case 'harbour': {
                const hue = ['#2c4055', '#5a3b46', '#3b5546', '#57492e'][Math.floor(prop.size * 3) % 4];
                c.fillStyle = hue;
                c.fillRect(-size * 0.62, -size * 0.36, size * 1.24, size * 0.72);
                c.strokeStyle = 'rgba(190,215,235,0.32)'; c.lineWidth = 1;
                for (let i = -2; i <= 2; i++) {
                    c.beginPath();
                    c.moveTo(i * size * 0.22, -size * 0.36); c.lineTo(i * size * 0.22, size * 0.36);
                    c.stroke();
                }
                c.strokeRect(-size * 0.62, -size * 0.36, size * 1.24, size * 0.72);
                break;
            }
            case 'salt':
                c.fillStyle = '#8d959c';
                c.fillRect(-size * 0.05, -size * 0.7, size * 0.1, size * 1.05);
                c.fillStyle = '#e8564a';
                c.fillRect(-size * 0.08, -size * 0.78, size * 0.16, size * 0.18);
                c.fillStyle = 'rgba(255,255,255,0.55)';
                c.fillRect(-size * 0.03, -size * 0.5, size * 0.06, size * 0.3);
                break;
            case 'forest':
                c.fillStyle = '#4a3b2c';
                c.fillRect(-size * 0.08, size * 0.08, size * 0.16, size * 0.4);
                c.fillStyle = '#1d3a26';
                c.beginPath(); c.arc(0, -size * 0.1, size * 0.58, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#315c39';
                c.beginPath(); c.arc(-size * 0.16, -size * 0.22, size * 0.38, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#4a7c4c';
                c.beginPath(); c.arc(-size * 0.22, -size * 0.34, size * 0.2, 0, Math.PI * 2); c.fill();
                break;
            case 'industry':
                if (prop.size > 1.15) {
                    c.fillStyle = '#4a5057';
                    c.beginPath(); c.arc(0, 0, size * 0.52, 0, Math.PI * 2); c.fill();
                    c.strokeStyle = '#6b747d'; c.lineWidth = 2;
                    c.beginPath(); c.arc(0, 0, size * 0.52, 0, Math.PI * 2); c.stroke();
                    c.fillStyle = '#77828c';
                    c.beginPath(); c.arc(0, 0, size * 0.16, 0, Math.PI * 2); c.fill();
                } else {
                    c.fillStyle = '#4a5057'; c.fillRect(-size * 0.4, -size * 0.55, size * 0.8, size * 1.1);
                    c.fillStyle = '#ff9d4a'; c.globalAlpha = 0.55;
                    c.fillRect(-size * 0.25, -size * 0.3, size * 0.5, size * 0.14);
                    c.fillRect(-size * 0.25, -size * 0.02, size * 0.5, size * 0.14);
                    c.globalAlpha = 1;
                    c.fillStyle = '#2f343a'; c.fillRect(-size * 0.4, -size * 0.55, size * 0.8, size * 0.12);
                }
                break;
            default:
                c.fillStyle = '#8f9a86';
                c.beginPath(); c.arc(0, 0, size * 0.4, 0, Math.PI * 2); c.fill();
        }
        c.restore();
    }

    AD.createRenderer = function (canvas, settings) {
        const c = canvas.getContext('2d', { alpha: false });
        let width = 960, height = 540, dpr = 1;
        let quality = AD.QUALITY.high;
        const chunks = new Map();
        const decals = [];
        const particles = [];
        const lights = [];
        let frame = 0;
        let camPrevX = 0, camPrevY = 0, camVX = 0, camVY = 0;

        function getChunk(track, cx, cy) {
            const key = track.id + '/' + track.seed + '/' + cx + ',' + cy;
            if (chunks.has(key)) {
                const hit = chunks.get(key);
                chunks.delete(key); chunks.set(key, hit);
                return hit;
            }
            const entry = { canvas: makeChunk(track, cx, cy), props: AD.chunkProps(track, cx, cy) };
            chunks.set(key, entry);
            while (chunks.size > quality.chunkCache) chunks.delete(chunks.keys().next().value);
            return entry;
        }

        function worldToScreen(x, y, cam) {
            return [width * 0.5 + (x - cam.x) * cam.scale, height * 0.5 + (y - cam.y) * cam.scale];
        }

        /* Screen polylines of the centreline and constant lateral offsets.
           Offsets follow the track normal, never the screen axes, so bands on
           the racing line stay glued to the road through corners. */
        function centrePath(cam, track) {
            const samples = track.samples;
            c.beginPath();
            for (let i = 0; i < samples.length; i++) {
                const p = samples[i];
                const sx = width * 0.5 + (p.x - cam.x) * cam.scale;
                const sy = height * 0.5 + (p.y - cam.y) * cam.scale;
                if (i === 0) c.moveTo(sx, sy); else c.lineTo(sx, sy);
            }
            c.closePath();
        }

        function offsetPath(cam, track, metres) {
            const samples = track.samples;
            c.beginPath();
            for (let i = 0; i < samples.length; i++) {
                const p = samples[i];
                const off = typeof metres === 'function' ? metres(p) : metres;
                const sx = width * 0.5 + (p.x + p.nx * off - cam.x) * cam.scale;
                const sy = height * 0.5 + (p.y + p.ny * off - cam.y) * cam.scale;
                if (i === 0) c.moveTo(sx, sy); else c.lineTo(sx, sy);
            }
            c.closePath();
        }

        /* World-anchored dashes: the pattern is a fixed length of real track, so
           markings never crawl when standing still and never stretch with zoom.
           The period divides the lap length exactly, so the seam closes cleanly. */
        function worldDash(track, metres, duty) {
            const length = track.length || 1;
            const periods = Math.max(1, Math.round(length / metres));
            const period = length / periods;
            return [Math.max(0.4, period * duty), Math.max(0.4, period * (1 - duty))];
        }

        function drawStartLine(cam, track) {
            const samples = track.samples;
            if (!samples.length) return;
            const p = samples[0];
            const w = p.halfWidth;
            const cols = 10, rows = 2;
            const cell = (w * 2) / cols;
            const along = 1.15;
            /* White base so the dark squares read on night asphalt too. */
            for (let row = 0; row < rows; row++) {
                const a0 = (row - rows / 2) * along, a1 = (row - rows / 2 + 1) * along;
                c.fillStyle = 'rgba(240,244,248,0.92)';
                c.beginPath();
                c.moveTo(...worldToScreen(p.x - p.nx * w + p.tx * a0, p.y - p.ny * w + p.ty * a0, cam));
                c.lineTo(...worldToScreen(p.x + p.nx * w + p.tx * a0, p.y + p.ny * w + p.ty * a0, cam));
                c.lineTo(...worldToScreen(p.x + p.nx * w + p.tx * a1, p.y + p.ny * w + p.ty * a1, cam));
                c.lineTo(...worldToScreen(p.x - p.nx * w + p.tx * a1, p.y - p.ny * w + p.ty * a1, cam));
                c.closePath(); c.fill();
            }
            for (let row = 0; row < rows; row++) {
                for (let col = 0; col < cols; col++) {
                    if ((row + col) % 2 === 0) continue;
                    const l = -w + col * cell;
                    const a0 = (row - rows / 2) * along;
                    const a1 = (row - rows / 2 + 1) * along;
                    const x0 = p.x + p.nx * l + p.tx * a0, y0 = p.y + p.ny * l + p.ty * a0;
                    const x1 = p.x + p.nx * (l + cell) + p.tx * a0, y1 = p.y + p.ny * (l + cell) + p.ty * a0;
                    const x2 = p.x + p.nx * (l + cell) + p.tx * a1, y2 = p.y + p.ny * (l + cell) + p.ty * a1;
                    const x3 = p.x + p.nx * l + p.tx * a1, y3 = p.y + p.ny * l + p.ty * a1;
                    c.fillStyle = 'rgba(16,18,22,0.95)';
                    c.beginPath();
                    c.moveTo(...worldToScreen(x0, y0, cam));
                    c.lineTo(...worldToScreen(x1, y1, cam));
                    c.lineTo(...worldToScreen(x2, y2, cam));
                    c.lineTo(...worldToScreen(x3, y3, cam));
                    c.closePath(); c.fill();
                }
            }
        }

        function drawRoadSurface(track, cam) {
            const w = track.width * cam.scale;
            c.lineCap = 'round'; c.lineJoin = 'round';
            centrePath(cam, track);
            c.strokeStyle = track.biome.edge; c.lineWidth = w + 3.4 * cam.scale; c.stroke();
            c.strokeStyle = track.biome.road; c.lineWidth = w; c.stroke();

            /* Subtle asphalt grain and a wet sheen for night biomes. */
            if (quality.lighting) {
                c.strokeStyle = track.biome.night ? 'rgba(120,170,210,0.07)' : 'rgba(255,255,255,0.035)';
                c.lineWidth = w * 0.62; c.stroke();
            }
            /* Two worn racing-line bands: tyres polish the asphalt over time.
               They follow the track normal, so they stay on the road in corners. */
            c.strokeStyle = track.biome.night ? 'rgba(16,20,28,0.30)' : 'rgba(20,22,26,0.16)';
            c.lineWidth = w * 0.16;
            const lane = track.width * 0.22;
            offsetPath(cam, track, lane); c.stroke();
            offsetPath(cam, track, -lane); c.stroke();
        }

        function drawCurbs(track, cam) {
            const scale = cam.scale;
            const dash = worldDash(track, 11, 0.5);
            for (const sign of [-1, 1]) {
                offsetPath(cam, track, p => sign * (p.halfWidth + 0.25));
                c.lineCap = 'butt';
                c.strokeStyle = 'rgba(232,236,240,0.85)';
                c.lineWidth = 1.15 * scale;
                c.setLineDash([]);
                c.stroke();
                c.strokeStyle = '#c22f2f';
                c.setLineDash([dash[0] * scale, dash[1] * scale]);
                c.stroke();
            }
            c.setLineDash([]);
            c.lineCap = 'round';
        }

        function drawRoadMarkings(track, cam) {
            const scale = cam.scale;
            const centreDash = worldDash(track, 9, 0.42);
            c.lineCap = 'butt';
            /* Solid white edge lines just inside the kerbs. */
            c.strokeStyle = track.biome.line;
            c.globalAlpha = 0.85;
            c.lineWidth = Math.max(1, 0.16 * scale);
            offsetPath(cam, track, p => p.halfWidth - 0.4); c.stroke();
            offsetPath(cam, track, p => -(p.halfWidth - 0.4)); c.stroke();
            /* Dashed centre line, anchored to world arc length. */
            c.globalAlpha = 0.62;
            c.lineWidth = Math.max(1.2, 0.22 * scale);
            centrePath(cam, track);
            c.setLineDash([centreDash[0] * scale, centreDash[1] * scale]);
            c.stroke();
            c.setLineDash([]);
            c.globalAlpha = 1;
            c.lineCap = 'round';
            drawStartLine(cam, track);
        }

        function drawDecals(cam) {
            c.lineCap = 'round';
            for (const d of decals) {
                const [sx, sy] = worldToScreen(d.x, d.y, cam);
                if (sx < -20 || sx > width + 20 || sy < -20 || sy > height + 20) continue;
                c.strokeStyle = 'rgba(12,12,14,' + (d.alpha * 0.42).toFixed(3) + ')';
                c.lineWidth = Math.max(1, 1.1 * cam.scale);
                c.beginPath();
                c.moveTo(sx - Math.cos(d.angle) * 2.2, sy - Math.sin(d.angle) * 2.2);
                c.lineTo(sx + Math.cos(d.angle) * 2.2, sy + Math.sin(d.angle) * 2.2);
                c.stroke();
            }
        }

        function drawCar(car, cam, ghost) {
            const v = car.v || car;
            const [sx, sy] = worldToScreen(v.x, v.y, cam);
            if (sx < -80 || sx > width + 80 || sy < -80 || sy > height + 80) return;
            const len = 4.5 * cam.scale, wid = 2.2 * cam.scale;
            const night = !!carNight;
            c.save(); c.translate(sx, sy); c.rotate(v.psi);
            if (ghost) c.globalAlpha = 0.32;

            c.fillStyle = 'rgba(0,0,0,0.38)';
            c.beginPath(); c.ellipse(1.5, 2.2, len * 0.55, wid * 0.52, 0, 0, Math.PI * 2); c.fill();

            /* Wheels: the front pair visibly follows the steering angle. */
            const steerAngle = ghost ? 0 : (v.steer || 0);
            c.fillStyle = '#15181c';
            for (const [wx, wy, steer] of [
                [-len * 0.34, -wid * 0.58, 0], [-len * 0.34, wid * 0.58, 0],
                [len * 0.30, -wid * 0.6, steerAngle], [len * 0.30, wid * 0.6, steerAngle]
            ]) {
                c.save(); c.translate(wx, wy); c.rotate(steer);
                c.fillRect(-len * 0.11, -wid * 0.09, len * 0.22, wid * 0.18);
                c.restore();
            }

            const body = c.createLinearGradient(0, -wid / 2, 0, wid / 2);
            const colour = ghost ? '#9fd6ff' : (v.car ? v.car.colour : '#ff5a4e');
            body.addColorStop(0, shade(colour, 0.18));
            body.addColorStop(0.45, colour);
            body.addColorStop(0.55, shade(colour, -0.28));
            body.addColorStop(1, shade(colour, -0.55));
            c.fillStyle = body;
            roundRect(c, -len / 2, -wid / 2, len, wid, wid * 0.28); c.fill();

            /* Racing stripe down the nose and a subtle side highlight. */
            c.fillStyle = 'rgba(255,255,255,0.16)';
            c.fillRect(-len * 0.06, -wid * 0.1, len * 0.52, wid * 0.2);
            c.fillStyle = 'rgba(255,255,255,0.10)';
            c.fillRect(-len * 0.42, -wid * 0.5, len * 0.84, wid * 0.08);

            c.fillStyle = 'rgba(18,28,40,0.82)';
            roundRect(c, -len * 0.12, -wid * 0.38, len * 0.34, wid * 0.76, wid * 0.12); c.fill();
            c.fillStyle = 'rgba(190,225,255,0.35)';
            c.fillRect(-len * 0.1, -wid * 0.32, len * 0.1, wid * 0.64);
            c.fillStyle = 'rgba(150,195,235,0.28)';
            c.fillRect(-len * 0.22, -wid * 0.3, len * 0.08, wid * 0.6);
            c.fillStyle = ghost ? '#9fd6ff' : (v.car ? v.car.accent : '#ffd7c2');
            c.fillRect(len * 0.44, -wid * 0.42, len * 0.05, wid * 0.84);

            if (!ghost) {
                const braking = v.brake > 0.05;
                c.fillStyle = braking ? '#ff3b30' : '#8a231c';
                c.fillRect(-len * 0.5, -wid * 0.42, len * 0.05, wid * 0.22);
                c.fillRect(-len * 0.5, wid * 0.2, len * 0.05, wid * 0.22);
                if (braking && quality.bloom) {
                    const glow = c.createRadialGradient(-len * 0.56, 0, 0, -len * 0.56, 0, len * 0.32);
                    glow.addColorStop(0, 'rgba(255,60,40,0.4)');
                    glow.addColorStop(1, 'rgba(255,60,40,0)');
                    c.fillStyle = glow;
                    c.fillRect(-len * 0.92, -wid * 0.7, len * 0.62, wid * 1.4);
                }
                if (night && quality.lighting) {
                    for (const side of [-1, 1]) {
                        const hx = len * 0.5, hy = side * wid * 0.3;
                        const beam = c.createLinearGradient(hx, 0, hx + len * 5, 0);
                        beam.addColorStop(0, 'rgba(255,236,190,0.11)');
                        beam.addColorStop(1, 'rgba(255,236,190,0)');
                        c.fillStyle = beam;
                        c.beginPath();
                        c.moveTo(hx, hy - wid * 0.08);
                        c.lineTo(hx + len * 5, hy - wid * 0.85);
                        c.lineTo(hx + len * 5, hy + wid * 0.85);
                        c.lineTo(hx, hy + wid * 0.08);
                        c.closePath(); c.fill();
                        c.fillStyle = 'rgba(255,244,214,0.9)';
                        c.beginPath(); c.arc(hx, hy, wid * 0.08, 0, Math.PI * 2); c.fill();
                    }
                }
                if (v.boost) {
                    const glow = c.createLinearGradient(-len * 0.8, 0, -len * 0.45, 0);
                    glow.addColorStop(0, 'rgba(90,200,255,0)');
                    glow.addColorStop(1, 'rgba(160,235,255,0.9)');
                    c.fillStyle = glow;
                    c.fillRect(-len * 0.8, -wid * 0.2, len * 0.35, wid * 0.4);
                    c.fillStyle = 'rgba(230,250,255,0.85)';
                    c.beginPath(); c.ellipse(-len * 0.52, 0, len * 0.1, wid * 0.13, 0, 0, Math.PI * 2); c.fill();
                }
                if (v.shield > 0) {
                    c.strokeStyle = 'rgba(94,234,212,0.85)'; c.lineWidth = 2;
                    c.beginPath(); c.ellipse(0, 0, len * 0.62, wid * 0.9, 0, 0, Math.PI * 2); c.stroke();
                }
                if (v.disabled > 0) {
                    c.strokeStyle = 'rgba(56,189,248,0.8)'; c.lineWidth = 1.5;
                    c.beginPath(); c.arc(0, 0, len * 0.6, 0, Math.PI * 2); c.stroke();
                }
                if (v.damage > 55) {
                    c.fillStyle = 'rgba(30,26,24,0.4)';
                    c.beginPath(); c.arc(len * 0.2, wid * 0.18, wid * 0.22, 0, Math.PI * 2); c.fill();
                }
            }
            c.restore();
        }

        function shade(hex, amount) {
            const n = parseInt(hex.slice(1), 16);
            const r = clamp(((n >> 16) & 255) * (1 + amount), 0, 255) | 0;
            const g = clamp(((n >> 8) & 255) * (1 + amount), 0, 255) | 0;
            const b = clamp((n & 255) * (1 + amount), 0, 255) | 0;
            return 'rgb(' + r + ',' + g + ',' + b + ')';
        }

        function roundRect(context, x, y, w, h, r) {
            context.beginPath();
            context.moveTo(x + r, y);
            context.arcTo(x + w, y, x + w, y + h, r);
            context.arcTo(x + w, y + h, x, y + h, r);
            context.arcTo(x, y + h, x, y, r);
            context.arcTo(x, y, x + w, y, r);
            context.closePath();
        }

        function spawn(kind, x, y, opts) {
            if (particles.length >= quality.particles) particles.shift();
            const rng = Math.random;
            particles.push({
                kind: kind, x: x, y: y,
                vx: (opts && opts.vx) || (rng() - 0.5) * 8,
                vy: (opts && opts.vy) || (rng() - 0.5) * 8,
                life: (opts && opts.life) || 0.7, max: (opts && opts.life) || 0.7,
                size: (opts && opts.size) || 1.6, colour: (opts && opts.colour) || '#ffffff'
            });
        }

        function updateParticles(dt) {
            for (let i = particles.length - 1; i >= 0; i--) {
                const p = particles[i];
                p.life -= dt;
                if (p.life <= 0) { particles.splice(i, 1); continue; }
                p.x += p.vx * dt; p.y += p.vy * dt;
                p.vx *= 0.96; p.vy *= 0.96;
            }
        }

        function drawParticles(cam) {
            for (const p of particles) {
                const [sx, sy] = worldToScreen(p.x, p.y, cam);
                if (sx < 0 || sx > width || sy < 0 || sy > height) continue;
                const fade = clamp(p.life / p.max, 0, 1);
                const smoke = p.kind === 'smoke';
                c.globalAlpha = fade * (smoke ? 0.24 : 0.8);
                c.fillStyle = p.colour;
                c.beginPath(); c.arc(sx, sy, Math.max(0.6, p.size * cam.scale * fade * (smoke ? 0.7 : 1)), 0, Math.PI * 2); c.fill();
            }
            c.globalAlpha = 1;
        }

        function drawLighting(race, cam, reducedMotion) {
            const biome = race.track.biome;
            if (!biome.night || !quality.lighting) return;
            c.save();
            c.globalCompositeOperation = 'lighter';
            for (const car of race.cars) {
                const v = car.v;
                const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
                const [sx, sy] = worldToScreen(v.x + cos * 12, v.y + sin * 12, cam);
                const radius = 30 * cam.scale;
                const glow = c.createRadialGradient(sx, sy, 0, sx, sy, radius);
                glow.addColorStop(0, 'rgba(210,235,255,0.12)');
                glow.addColorStop(1, 'rgba(120,180,255,0)');
                c.fillStyle = glow;
                c.beginPath(); c.arc(sx, sy, radius, 0, Math.PI * 2); c.fill();
            }
            if (!reducedMotion) {
                for (const light of lights) {
                    const [sx, sy] = worldToScreen(light.x, light.y, cam);
                    if (sx < -100 || sx > width + 100 || sy < -100 || sy > height + 100) continue;
                    const glow = c.createRadialGradient(sx, sy, 0, sx, sy, 18 * cam.scale);
                    glow.addColorStop(0, 'rgba(255,196,120,0.30)');
                    glow.addColorStop(0.4, 'rgba(255,178,92,0.12)');
                    glow.addColorStop(1, 'rgba(255,170,80,0)');
                    c.fillStyle = glow;
                    c.beginPath(); c.arc(sx, sy, 18 * cam.scale, 0, Math.PI * 2); c.fill();
                }
            }
            c.restore();
            /* Lamp poles give the pools a physical anchor. */
            c.save();
            for (const light of lights) {
                const [sx, sy] = worldToScreen(light.x, light.y, cam);
                if (sx < -60 || sx > width + 60 || sy < -60 || sy > height + 60) continue;
                c.fillStyle = 'rgba(12,16,22,0.85)';
                c.beginPath(); c.arc(sx, sy, 1.1 * cam.scale, 0, Math.PI * 2); c.fill();
                c.fillStyle = 'rgba(255,214,150,0.95)';
                c.beginPath(); c.arc(sx, sy, 0.55 * cam.scale, 0, Math.PI * 2); c.fill();
            }
            c.restore();
        }

        function drawWeather(race, cam, dt, reducedMotion) {
            const weather = race.track.biome.weather;
            if (!weather || reducedMotion || quality.weather <= 0) return;
            const amount = Math.floor(quality.weather * (weather === 'rain' ? 90 : 45));
            c.save();
            if (weather === 'rain') {
                /* Streaks lean with camera motion so the rain feels alive. */
                const drift = clamp(-camVX * 0.06, -6, 6);
                c.strokeStyle = 'rgba(170,205,240,0.32)';
                c.lineWidth = 1;
                for (let i = 0; i < amount; i++) {
                    const x = (i * 97 + frame * 7) % (width + 80) - 40;
                    const y = (i * 53 + frame * 16) % (height + 80) - 40;
                    c.beginPath(); c.moveTo(x, y);
                    c.lineTo(x - 5 + drift, y + 15);
                    c.stroke();
                }
                c.strokeStyle = 'rgba(170,205,240,0.14)';
                for (let i = 0; i < amount / 3; i++) {
                    const x = (i * 131 + frame * 3.2) % (width + 40) - 20;
                    const y = (i * 67 + frame * 7) % (height + 40) - 20;
                    c.beginPath(); c.moveTo(x, y); c.lineTo(x - 2 + drift, y + 8); c.stroke();
                }
            } else if (weather === 'sparks') {
                c.strokeStyle = 'rgba(255,170,80,0.28)';
                c.lineWidth = 1;
                for (let i = 0; i < amount; i++) {
                    const x = (i * 97 + frame * 1.4) % (width + 80) - 40;
                    const y = (i * 53 + frame * 2) % (height + 80) - 40;
                    c.beginPath(); c.moveTo(x, y); c.lineTo(x - 1, y + 3); c.stroke();
                }
            } else {
                c.strokeStyle = 'rgba(230,230,215,0.16)';
                c.lineWidth = 1;
                for (let i = 0; i < amount; i++) {
                    const x = (i * 97 + frame * 1.4) % (width + 80) - 40;
                    const y = (i * 53 + frame * 2) % (height + 80) - 40;
                    c.beginPath(); c.moveTo(x, y); c.lineTo(x - 1, y + 3); c.stroke();
                }
            }
            if (weather === 'fog' || weather === 'mist' || weather === 'glare') {
                c.fillStyle = weather === 'glare' ? 'rgba(255,255,240,0.10)' : 'rgba(200,215,205,0.10)';
                c.fillRect(0, 0, width, height);
                c.fillStyle = weather === 'glare' ? 'rgba(255,252,235,0.05)' : 'rgba(210,222,212,0.05)';
                const band = (frame * 0.18) % (height + 240) - 120;
                c.fillRect(0, band, width, 120);
            }
            if (weather === 'heat') {
                c.strokeStyle = 'rgba(255,214,150,0.07)';
                c.lineWidth = 6;
                for (let i = 0; i < 5; i++) {
                    const y = (i * 97 + frame * 0.35) % height;
                    c.beginPath(); c.moveTo(0, y);
                    for (let x = 0; x <= width; x += 48) c.lineTo(x, y + Math.sin(x * 0.02 + frame * 0.05 + i) * 5);
                    c.stroke();
                }
            }
            c.restore();
        }

        function drawVignette(reducedMotion, speed) {
            const g = c.createRadialGradient(width / 2, height / 2, height * 0.28, width / 2, height / 2, height * 0.86);
            g.addColorStop(0, 'rgba(0,0,0,0)');
            g.addColorStop(1, 'rgba(6,8,14,0.55)');
            c.fillStyle = g; c.fillRect(0, 0, width, height);
            if (!reducedMotion && speed > 45 && quality.bloom) {
                /* Speed streaks radiate outward from the focus, not sideways. */
                c.save();
                c.translate(width / 2, height / 2);
                const stretch = clamp((speed - 45) / 45, 0, 1);
                c.strokeStyle = 'rgba(255,255,255,' + (0.03 + stretch * 0.05).toFixed(3) + ')';
                c.lineWidth = 1.2;
                for (let i = 0; i < 16; i++) {
                    const angle = (i * 2.39996) % (Math.PI * 2);
                    const inner = height * (0.32 + (i % 3) * 0.06);
                    const outer = inner + height * (0.1 + stretch * 0.16);
                    c.beginPath();
                    c.moveTo(Math.cos(angle) * inner, Math.sin(angle) * inner);
                    c.lineTo(Math.cos(angle) * outer, Math.sin(angle) * outer);
                    c.stroke();
                }
                c.restore();
            }
        }

        let carNight = false;

        return {
            resize(w, h, ratio) {
                width = Math.max(320, Math.round(w));
                height = Math.max(240, Math.round(h));
                dpr = clamp(ratio || 1, 1, 2);
                canvas.width = Math.round(width * dpr);
                canvas.height = Math.round(height * dpr);
                canvas.style.width = width + 'px';
                canvas.style.height = height + 'px';
                c.setTransform(dpr, 0, 0, dpr, 0, 0);
            },
            setQuality(q) { quality = AD.QUALITY[q] || AD.QUALITY.high; },
            setTrack(track) {
                chunks.clear(); decals.length = 0; particles.length = 0;
                lights.length = 0;
                /* Lamp posts every ~110 m alternate sides of the road. */
                const spacing = 110;
                for (let s = 0, i = 0; s < track.length; s += spacing, i++) {
                    const p = AD.pointAt(track, s);
                    const side = i % 2 === 0 ? 1 : -1;
                    lights.push({
                        x: p.x + p.nx * (p.halfWidth + 4.5) * side,
                        y: p.y + p.ny * (p.halfWidth + 4.5) * side
                    });
                }
            },
            addDecal(x, y, angle, alpha) {
                decals.push({ x, y, angle, alpha });
                if (decals.length > 900) decals.splice(0, decals.length - 900);
            },
            burst(kind, x, y, count, colour) {
                for (let i = 0; i < count; i++) {
                    spawn(kind, x, y, {
                        vx: (Math.random() - 0.5) * 22, vy: (Math.random() - 0.5) * 22,
                        life: 0.4 + Math.random() * 0.6, size: 1 + Math.random() * 2.4, colour: colour
                    });
                }
            },
            smoke(x, y, colour) {
                spawn('smoke', x, y, { vx: (Math.random() - 0.5) * 5, vy: (Math.random() - 0.5) * 5, life: 0.75, size: 1.5, colour: colour || '#c9cbd0' });
            },
            render(race, cam, dt, reducedMotion, alpha) {
                frame += 1;
                camVX = (cam.x - camPrevX) / Math.max(dt, 1e-4);
                camVY = (cam.y - camPrevY) / Math.max(dt, 1e-4);
                camPrevX = cam.x; camPrevY = cam.y;
                const track = race.track;
                carNight = !!track.biome.night;
                const shake = reducedMotion ? 0 : cam.shake;
                const ox = shake ? (Math.random() - 0.5) * shake * 8 : 0;
                const oy = shake ? (Math.random() - 0.5) * shake * 8 : 0;
                c.save();
                c.translate(ox, oy);
                c.fillStyle = track.biome.ground;
                c.fillRect(-10, -10, width + 20, height + 20);

                const halfW = width / (2 * cam.scale), halfH = height / (2 * cam.scale);
                const minCx = Math.floor((cam.x - halfW - AD.CHUNK_METRES) / AD.CHUNK_METRES);
                const maxCx = Math.floor((cam.x + halfW + AD.CHUNK_METRES) / AD.CHUNK_METRES);
                const minCy = Math.floor((cam.y - halfH - AD.CHUNK_METRES) / AD.CHUNK_METRES);
                const maxCy = Math.floor((cam.y + halfH + AD.CHUNK_METRES) / AD.CHUNK_METRES);
                for (let cx = minCx; cx <= maxCx; cx++) {
                    for (let cy = minCy; cy <= maxCy; cy++) {
                        const chunk = getChunk(track, cx, cy);
                        const [sx, sy] = worldToScreen(cx * AD.CHUNK_METRES, cy * AD.CHUNK_METRES, cam);
                        c.drawImage(chunk.canvas, sx, sy, AD.CHUNK_METRES * cam.scale, AD.CHUNK_METRES * cam.scale);
                        for (const prop of chunk.props) {
                            const [px, py] = worldToScreen(cx * AD.CHUNK_METRES + (prop.x - cx * AD.CHUNK_METRES),
                                cy * AD.CHUNK_METRES + (prop.y - cy * AD.CHUNK_METRES), cam);
                            drawProp(c, prop, px, py, cam.scale);
                        }
                    }
                }
                drawRoadSurface(track, cam);
                drawDecals(cam);
                drawCurbs(track, cam);
                drawRoadMarkings(track, cam);

                /* Pickups, hazards and projectiles live in world space. */
                for (const pad of race.pickups) {
                    if (race.time - pad.taken < 8) continue;
                    const [sx, sy] = worldToScreen(pad.x, pad.y, cam);
                    if (sx < -20 || sx > width + 20 || sy < -20 || sy > height + 20) continue;
                    c.save(); c.translate(sx, sy); c.rotate(frame * 0.02);
                    const halo = c.createRadialGradient(0, 0, 0, 0, 0, 4 * cam.scale);
                    halo.addColorStop(0, 'rgba(120,230,255,0.35)');
                    halo.addColorStop(1, 'rgba(120,230,255,0)');
                    c.fillStyle = halo;
                    c.beginPath(); c.arc(0, 0, 4 * cam.scale, 0, Math.PI * 2); c.fill();
                    c.strokeStyle = 'rgba(140,235,255,0.8)'; c.lineWidth = 1.5;
                    c.beginPath(); c.arc(0, 0, 2.6 * cam.scale, 0, Math.PI * 2); c.stroke();
                    c.fillStyle = '#8ce9ff';
                    c.fillRect(-1.1 * cam.scale, -1.1 * cam.scale, 2.2 * cam.scale, 2.2 * cam.scale);
                    c.restore();
                }
                for (const h of race.hazards) {
                    const [sx, sy] = worldToScreen(h.x, h.y, cam);
                    if (sx < -20 || sx > width + 20 || sy < -20 || sy > height + 20) continue;
                    if (h.type === 'oil') {
                        const oil = c.createRadialGradient(sx, sy, 0, sx, sy, h.r * cam.scale);
                        oil.addColorStop(0, 'rgba(30,26,44,0.8)');
                        oil.addColorStop(0.7, 'rgba(10,10,14,0.66)');
                        oil.addColorStop(1, 'rgba(10,10,14,0)');
                        c.fillStyle = oil;
                        c.beginPath(); c.ellipse(sx, sy, h.r * cam.scale, h.r * cam.scale * 0.7, 0, 0, Math.PI * 2); c.fill();
                    } else {
                        const armed = h.arm > 0;
                        c.strokeStyle = armed ? '#f97316' : '#ffd166';
                        c.lineWidth = 2;
                        c.beginPath(); c.arc(sx, sy, h.r * cam.scale, 0, Math.PI * 2); c.stroke();
                        c.fillStyle = armed ? 'rgba(249,115,22,0.25)' : 'rgba(255,209,102,0.3)';
                        c.beginPath(); c.arc(sx, sy, h.r * cam.scale * 0.55, 0, Math.PI * 2); c.fill();
                        if (!armed && frame % 30 < 15) {
                            c.fillStyle = 'rgba(255,120,60,0.9)';
                            c.beginPath(); c.arc(sx, sy, 1.2 * cam.scale, 0, Math.PI * 2); c.fill();
                        }
                    }
                }
                for (const p of race.projectiles) {
                    const [sx, sy] = worldToScreen(p.x, p.y, cam);
                    if (p.type === 'rocket') {
                        const tail = c.createLinearGradient(
                            sx - Math.cos(p.psi) * 8 * cam.scale, sy - Math.sin(p.psi) * 8 * cam.scale, sx, sy);
                        tail.addColorStop(0, 'rgba(255,140,80,0)');
                        tail.addColorStop(1, 'rgba(255,190,120,0.7)');
                        c.strokeStyle = tail; c.lineWidth = 2.2 * cam.scale;
                        c.beginPath();
                        c.moveTo(sx - Math.cos(p.psi) * 8 * cam.scale, sy - Math.sin(p.psi) * 8 * cam.scale);
                        c.lineTo(sx, sy); c.stroke();
                        c.fillStyle = '#fb7185';
                        c.beginPath(); c.arc(sx, sy, 1.5 * cam.scale, 0, Math.PI * 2); c.fill();
                    } else {
                        c.strokeStyle = 'rgba(250,204,21,0.85)'; c.lineWidth = 1.2 * cam.scale;
                        c.beginPath();
                        c.moveTo(sx, sy);
                        c.lineTo(sx - Math.cos(p.psi) * 3.4 * cam.scale, sy - Math.sin(p.psi) * 3.4 * cam.scale);
                        c.stroke();
                    }
                }

                if (race.ghost && race.ghost.active) drawCar(race.ghost, cam, true);
                for (const car of race.cars) drawCar(car, cam, false);

                updateParticles(dt);
                drawParticles(cam);
                drawLighting(race, cam, reducedMotion);
                drawWeather(race, cam, dt, reducedMotion);
                if (track.biome.night) {
                    c.fillStyle = 'rgba(6,12,26,0.20)';
                    c.fillRect(0, 0, width, height);
                }
                drawVignette(reducedMotion, race.cars[0].v.vx);
                c.restore();
            },
            dispose() {
                chunks.clear(); decals.length = 0; particles.length = 0; lights.length = 0;
            },
            stats() { return { chunks: chunks.size, decals: decals.length, particles: particles.length }; }
        };
    };
})();
