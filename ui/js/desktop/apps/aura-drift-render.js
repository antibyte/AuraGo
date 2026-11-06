/* Aura Drift - Canvas 2D world renderer with streamed chunks and pooled effects. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    AD.createCamera = function () {
        return {
            x: 0, y: 0, scale: 5.2, shake: 0, lookX: 0, lookY: 0, punch: 0,
            update(v, dt, quality, reducedMotion) {
                const speed = Math.max(0, v.vx);
                const boost = v.boost ? 1 : 0;
                let targetScale = quality.lowZoom ? 5.0 : clamp(7.0 - speed * 0.02, 5.2, 7.0);
                /* Nitro punches the field of view in: the speed reads in the
                   frame, not only on the dial. */
                if (!quality.lowZoom) targetScale *= 1 - boost * 0.06;
                this.scale = AD.smooth(this.scale, targetScale, boost ? 7 : 2.4, dt);
                const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
                const lead = reducedMotion ? 0 : clamp(speed * 0.55, 0, 42);
                /* Drift lean: the camera looks into the slide so the apex of
                   the corner stays in frame while the car is sideways. */
                const lateral = reducedMotion ? 0 : clamp(-v.vy * 0.55, -16, 16);
                this.lookX = AD.smooth(this.lookX, cos * lead - sin * lateral, 3, dt);
                this.lookY = AD.smooth(this.lookY, sin * lead + cos * lateral, 3, dt);
                this.punch = AD.smooth(this.punch, boost, 9, dt);
                this.x = v.x + this.lookX;
                this.y = v.y + this.lookY;
                this.shake = Math.max(0, this.shake - dt * 2.4);
            }
        };
    };

    AD.createRenderer = function (canvas, settings) {
        const c = canvas.getContext('2d', { alpha: false });
        let width = 960, height = 540, dpr = 1;
        let quality = AD.QUALITY.high;
        const chunks = new Map();
        const decals = [];
        const particles = [];
        const lights = [];
        const furniture = [];
        let frame = 0;
        let camPrevX = 0, camPrevY = 0, camVX = 0, camVY = 0;

        function getChunk(track, cx, cy) {
            const key = track.id + '/' + track.seed + '/' + cx + ',' + cy;
            if (chunks.has(key)) {
                const hit = chunks.get(key);
                chunks.delete(key); chunks.set(key, hit);
                return hit;
            }
            const entry = { canvas: AD.makeChunkCanvas(track, cx, cy), props: AD.chunkProps(track, cx, cy) };
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
            const cols = 12, rows = 3;
            const cell = (w * 2) / cols;
            const along = 1.4;
            const total = rows * along;
            /* Dark frame grounds the line on pale salt/sand. */
            c.fillStyle = 'rgba(8,10,14,0.65)';
            c.beginPath();
            c.moveTo(...worldToScreen(p.x - p.nx * (w + 0.6) + p.tx * (-total / 2 - 0.3), p.y - p.ny * (w + 0.6) + p.ty * (-total / 2 - 0.3), cam));
            c.lineTo(...worldToScreen(p.x + p.nx * (w + 0.6) + p.tx * (-total / 2 - 0.3), p.y + p.ny * (w + 0.6) + p.ty * (-total / 2 - 0.3), cam));
            c.lineTo(...worldToScreen(p.x + p.nx * (w + 0.6) + p.tx * (total / 2 + 0.3), p.y + p.ny * (w + 0.6) + p.ty * (total / 2 + 0.3), cam));
            c.lineTo(...worldToScreen(p.x - p.nx * (w + 0.6) + p.tx * (total / 2 + 0.3), p.y - p.ny * (w + 0.6) + p.ty * (total / 2 + 0.3), cam));
            c.closePath(); c.fill();
            /* White base so the dark squares read on night asphalt too. */
            for (let row = 0; row < rows; row++) {
                const a0 = (row - rows / 2) * along, a1 = (row - rows / 2 + 1) * along;
                c.fillStyle = 'rgba(240,244,248,0.94)';
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
            /* Start gantry: two posts and an accent beam across the line. */
            const postL = worldToScreen(p.x - p.nx * (w + 2.2), p.y - p.ny * (w + 2.2), cam);
            const postR = worldToScreen(p.x + p.nx * (w + 2.2), p.y + p.ny * (w + 2.2), cam);
            const beamW = Math.hypot(postR[0] - postL[0], postR[1] - postL[1]);
            if (beamW < 2000) {
                c.save();
                c.strokeStyle = 'rgba(8,10,14,0.7)';
                c.lineWidth = Math.max(2, 0.5 * cam.scale);
                c.beginPath(); c.moveTo(postL[0], postL[1]); c.lineTo(postL[0], postL[1] - 7 * cam.scale); c.stroke();
                c.beginPath(); c.moveTo(postR[0], postR[1]); c.lineTo(postR[0], postR[1] - 7 * cam.scale); c.stroke();
                c.strokeStyle = track.biome.accent;
                c.globalAlpha = 0.9;
                c.lineWidth = Math.max(2, 0.7 * cam.scale);
                c.beginPath();
                c.moveTo(postL[0], postL[1] - 7 * cam.scale);
                c.lineTo(postR[0], postR[1] - 7 * cam.scale);
                c.stroke();
                c.restore();
            }
        }

        /* Ribbon fill: the road is a true offset polygon, not a stroked
           centreline, so the surface edge always meets the kerbs exactly -
           even in hairpins where a round-join stroke would pinch and bulge. */
        function ribbonPath(cam, track, grow) {
            const samples = track.samples;
            const n = samples.length;
            c.beginPath();
            for (let i = 0; i < n; i++) {
                const p = samples[i];
                const hw = p.halfWidth + grow;
                const sx = width * 0.5 + (p.x + p.nx * hw - cam.x) * cam.scale;
                const sy = height * 0.5 + (p.y + p.ny * hw - cam.y) * cam.scale;
                if (i === 0) c.moveTo(sx, sy); else c.lineTo(sx, sy);
            }
            for (let i = n - 1; i >= 0; i--) {
                const p = samples[i];
                const hw = p.halfWidth + grow;
                const sx = width * 0.5 + (p.x - p.nx * hw - cam.x) * cam.scale;
                const sy = height * 0.5 + (p.y - p.ny * hw - cam.y) * cam.scale;
                c.lineTo(sx, sy);
            }
            c.closePath();
        }

        function drawRoadSurface(track, cam, race) {
            /* Dark shoulder grounds the kerbs, then the asphalt ribbon. */
            c.fillStyle = track.biome.edge;
            ribbonPath(cam, track, 1.35);
            c.fill();
            c.fillStyle = track.biome.road;
            ribbonPath(cam, track, 0);
            c.fill();

            /* Subtle asphalt tone: a lighter inset ribbon, world shaped. */
            c.fillStyle = track.biome.night ? 'rgba(120,170,210,0.05)' : 'rgba(255,255,255,0.028)';
            ribbonPath(cam, track, -Math.max(0.8, track.width * 0.19));
            c.fill();
            /* Two worn racing-line bands: tyres polish the asphalt over time.
               They follow the track normal, so they stay on the road in corners. */
            c.strokeStyle = track.biome.night ? 'rgba(16,20,28,0.30)' : 'rgba(20,22,26,0.16)';
            c.lineWidth = track.width * cam.scale * 0.16;
            c.lineCap = 'round';
            const lane = track.width * 0.22;
            offsetPath(cam, track, lane); c.stroke();
            offsetPath(cam, track, -lane); c.stroke();
            /* Rain darkens and cools the asphalt before the post pass adds
               the reflections on top. */
            const conditions = race && race.conditions ? race.conditions.state : null;
            if (conditions && conditions.kind === 'rain' && conditions.intensity > 0.08) {
                c.fillStyle = 'rgba(72,112,152,' + (0.08 + conditions.intensity * 0.18).toFixed(3) + ')';
                ribbonPath(cam, track, 0);
                c.fill();
            }
        }

        /* Faint transverse joints every 55 m: true world quads, so the eye can
           see the anchoring is exact - they never drift, crawl or stretch. */
        function drawSeams(track, cam) {
            const step = 55;
            const count = Math.floor(track.length / step);
            c.fillStyle = track.biome.night ? 'rgba(10,14,22,0.10)' : 'rgba(10,12,16,0.055)';
            for (let i = 0; i < count; i++) {
                const s = i * step;
                const p = AD.pointAt(track, s);
                const hw = p.halfWidth - 0.3;
                const a0 = -0.22, a1 = 0.22;
                const x0 = p.x - p.nx * hw + p.tx * a0, y0 = p.y - p.ny * hw + p.ty * a0;
                const x1 = p.x + p.nx * hw + p.tx * a0, y1 = p.y + p.ny * hw + p.ty * a0;
                const x2 = p.x + p.nx * hw + p.tx * a1, y2 = p.y + p.ny * hw + p.ty * a1;
                const x3 = p.x - p.nx * hw + p.tx * a1, y3 = p.y - p.ny * hw + p.ty * a1;
                const q0 = worldToScreen(x0, y0, cam), q1 = worldToScreen(x1, y1, cam);
                if ((q0[0] < -40 && q1[0] < -40) || (q0[0] > width + 40 && q1[0] > width + 40) ||
                    (q0[1] < -40 && q1[1] < -40) || (q0[1] > height + 40 && q1[1] > height + 40)) continue;
                c.beginPath();
                c.moveTo(q0[0], q0[1]);
                c.lineTo(q1[0], q1[1]);
                c.lineTo(...worldToScreen(x2, y2, cam));
                c.lineTo(...worldToScreen(x3, y3, cam));
                c.closePath(); c.fill();
            }
        }

        /* Repair patches and tar snakes: the asphalt is old, and the eye
           reads a used surface far better than a flat fill. The placement is
           derived from the track seed, so it is stable for the whole race. */
        function drawRoadPatches(track, cam) {
            const rng = AD.rng(AD.hashString(track.id + '/patches/' + track.seed));
            const count = 34;
            for (let i = 0; i < count; i++) {
                const s = rng() * track.length;
                const p = AD.pointAt(track, s);
                const off = (rng() - 0.5) * p.halfWidth * 1.2;
                const len = 3 + rng() * 11;
                const wide = 1.4 + rng() * 3.4;
                const x = p.x + p.nx * off;
                const y = p.y + p.ny * off;
                const q = [
                    worldToScreen(x - p.tx * len * 0.5 - p.nx * wide * 0.5, y - p.ty * len * 0.5 - p.ny * wide * 0.5, cam),
                    worldToScreen(x + p.tx * len * 0.5 - p.nx * wide * 0.5, y + p.ty * len * 0.5 - p.ny * wide * 0.5, cam),
                    worldToScreen(x + p.tx * len * 0.5 + p.nx * wide * 0.5, y + p.ty * len * 0.5 + p.ny * wide * 0.5, cam),
                    worldToScreen(x - p.tx * len * 0.5 + p.nx * wide * 0.5, y - p.ty * len * 0.5 + p.ny * wide * 0.5, cam)
                ];
                if (q.every(v => v[0] < -30 || v[0] > width + 30)) continue;
                if (q.every(v => v[1] < -30 || v[1] > height + 30)) continue;
                c.fillStyle = track.biome.night ? 'rgba(8,12,18,0.30)' : 'rgba(24,26,30,0.17)';
                c.beginPath();
                c.moveTo(q[0][0], q[0][1]);
                c.lineTo(q[1][0], q[1][1]);
                c.lineTo(q[2][0], q[2][1]);
                c.lineTo(q[3][0], q[3][1]);
                c.closePath(); c.fill();
                /* The sealed edge of the repair. */
                c.strokeStyle = track.biome.night ? 'rgba(150,175,205,0.10)' : 'rgba(255,255,255,0.07)';
                c.lineWidth = 1;
                c.stroke();
            }
        }

        function drawCurbs(track, cam) {
            const scale = cam.scale;
            const dash = worldDash(track, 11, 0.5);
            const period = (dash[0] + dash[1]) * scale;
            const signs = [-1, 1];
            for (let k = 0; k < signs.length; k++) {
                const sign = signs[k];
                /* Dark grounding so the kerb never floats over grass. */
                offsetPath(cam, track, p => sign * (p.halfWidth + 0.25));
                c.lineCap = 'butt';
                c.strokeStyle = 'rgba(8,10,14,0.38)';
                c.lineWidth = 1.7 * scale;
                c.setLineDash([]);
                c.stroke();
                c.strokeStyle = 'rgba(232,236,240,0.9)';
                c.lineWidth = 1.15 * scale;
                c.stroke();
                c.strokeStyle = '#c22f2f';
                c.setLineDash([dash[0] * scale, dash[1] * scale]);
                /* Stagger the two sides by half a period: classic alternating kerbs. */
                c.lineDashOffset = k === 0 ? 0 : -period / 2;
                c.stroke();
            }
            c.setLineDash([]);
            c.lineDashOffset = 0;
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
            /* Dashed centre line, anchored to world arc length. A soft dark
               halo keeps it readable on pale salt and desert asphalt too. */
            centrePath(cam, track);
            c.globalAlpha = 0.5;
            c.strokeStyle = 'rgba(8,10,14,0.55)';
            c.lineWidth = Math.max(2, 0.42 * scale);
            c.setLineDash([centreDash[0] * scale, centreDash[1] * scale]);
            c.stroke();
            c.globalAlpha = 0.7;
            c.strokeStyle = '#f4f6f2';
            c.lineWidth = Math.max(1.2, 0.2 * scale);
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
                const fade = clamp((d.life === undefined ? 26 : d.life) / 8, 0, 1);
                const alpha = d.alpha * 0.42 * (0.25 + 0.75 * fade);
                if (alpha <= 0.01) continue;
                c.strokeStyle = 'rgba(12,12,14,' + alpha.toFixed(3) + ')';
                c.lineWidth = Math.max(1, 1.1 * cam.scale);
                c.beginPath();
                c.moveTo(sx - Math.cos(d.angle) * 2.2, sy - Math.sin(d.angle) * 2.2);
                c.lineTo(sx + Math.cos(d.angle) * 2.2, sy + Math.sin(d.angle) * 2.2);
                c.stroke();
            }
        }

        /* Car art lives in aura-drift-cars.js: one pre-rendered silhouette
           per car class, with wheels, lights and damage drawn on top. */
        function drawCar(car, cam, ghost) {
            const v = car.v || car;
            const [sx, sy] = worldToScreen(v.x, v.y, cam);
            AD.drawCar(c, car, sx, sy, cam.scale, {
                ghost: ghost,
                night: !!carNight,
                lighting: quality.lighting,
                bloom: quality.bloom,
                frame: frame,
                width: width,
                height: height
            });
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
            for (let i = decals.length - 1; i >= 0; i--) {
                const d = decals[i];
                if (d.life === undefined) d.life = 26;
                d.life -= dt;
                if (d.life <= 0) decals.splice(i, 1);
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
            const biome = race.track.biome;
            /* Live conditions win over the biome default: a race can start in
               the sun and end in a downpour. */
            const conditions = race.conditions ? race.conditions.state : null;
            const weather = conditions ? conditions.kind : biome.weather;
            const intensity = conditions ? conditions.intensity : 0.65;
            if (!weather || reducedMotion || quality.weather <= 0) return;
            const amount = Math.floor(quality.weather * intensity *
                (weather === 'rain' || weather === 'snow' ? 90 : 45));
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
                /* Splash rings where drops hit: cheap, bright, unmistakable wet. */
                c.strokeStyle = 'rgba(190,220,245,0.22)';
                for (let i = 0; i < amount / 6; i++) {
                    const x = (i * 211 + frame * 2.1) % (width + 40) - 20;
                    const y = (i * 157 + frame * 5.3) % (height + 40) - 20;
                    const r = 2 + ((frame * 0.6 + i * 2.4) % 5);
                    c.globalAlpha = clamp(1 - r / 7, 0, 1) * 0.5;
                    c.beginPath(); c.ellipse(x, y, r * 1.6, r * 0.6, 0, 0, Math.PI * 2); c.stroke();
                }
                c.globalAlpha = 1;
            } else if (weather === 'snow') {
                /* Slow flakes with a sideways drift: the difference between
                   snow and rain has to be readable at a glance. */
                c.fillStyle = 'rgba(238,246,255,0.55)';
                for (let i = 0; i < amount; i++) {
                    const x = (i * 89 + frame * 1.9 + Math.sin(i) * 22) % (width + 60) - 30;
                    const y = (i * 47 + frame * 2.6) % (height + 60) - 30;
                    const r = 1 + (i % 3) * 0.6;
                    c.beginPath(); c.arc(x, y, r, 0, Math.PI * 2); c.fill();
                }
                c.fillStyle = 'rgba(220,234,250,0.06)';
                c.fillRect(0, 0, width, height);
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
            if (weather === 'fog' || weather === 'mist' || weather === 'glare' || weather === 'haze') {
                const veil = weather === 'glare' ? 'rgba(255,255,240,0.12)' :
                    (weather === 'haze' ? 'rgba(198,168,120,0.10)' : 'rgba(200,215,205,0.12)');
                c.fillStyle = veil;
                c.globalAlpha = clamp(0.35 + intensity * 0.75, 0, 1);
                c.fillRect(0, 0, width, height);
                c.globalAlpha = 1;
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
                lights.length = 0; furniture.length = 0;
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
                /* Trackside furniture: grandstands, hoardings, tyre walls,
                   marshal posts, braking boards and the biome set pieces. */
                if (typeof AD.trackFurniture === 'function') {
                    const items = AD.trackFurniture(track);
                    for (const item of items) furniture.push(item);
                }
            },
            addDecal(x, y, angle, alpha) {
                decals.push({ x, y, angle, alpha, life: 26 });
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
                            AD.drawProp(c, prop, px, py, cam.scale);
                        }
                    }
                }
                drawRoadSurface(track, cam, race);
                if (quality.body !== 0) drawSeams(track, cam);
                drawRoadPatches(track, cam);
                drawDecals(cam);
                drawCurbs(track, cam);
                drawRoadMarkings(track, cam);
                /* Trackside furniture frames the road: grandstands, hoardings,
                   tyre walls, marshal posts and the biome set pieces. */
                for (const item of furniture) {
                    const [sx, sy] = worldToScreen(item.x, item.y, cam);
                    if (sx < -220 || sx > width + 220 || sy < -220 || sy > height + 220) continue;
                    AD.drawFurniture(c, item, sx, sy, cam.scale, frame);
                }

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
