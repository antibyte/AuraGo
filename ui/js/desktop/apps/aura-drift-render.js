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
                const targetScale = quality.lowZoom ? 4.4 : clamp(5.6 - Math.max(0, v.vx) * 0.018, 4.1, 5.6);
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

        const patches = 26;
        for (let i = 0; i < patches; i++) {
            const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
            const r = 18 + rng() * 70;
            c.fillStyle = rng() > 0.5 ? biome.groundAlt : biome.edge;
            c.globalAlpha = 0.05 + rng() * 0.09;
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
        return canvas;
    }

    function drawProp(c, prop, sx, sy, scale) {
        const size = prop.size * 2.6 * scale;
        c.save(); c.translate(sx, sy); c.rotate(prop.angle);
        switch (prop.kind) {
            case 'forest':
                c.fillStyle = 'rgba(8,20,12,0.35)';
                c.beginPath(); c.ellipse(2, 2, size * 0.7, size * 0.45, 0, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#1d3a26';
                c.beginPath(); c.arc(0, 0, size * 0.62, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#315c39';
                c.beginPath(); c.arc(-size * 0.14, -size * 0.14, size * 0.42, 0, Math.PI * 2); c.fill();
                break;
            case 'desert':
                c.fillStyle = '#7d5a34';
                c.beginPath(); c.moveTo(-size * 0.5, size * 0.4); c.lineTo(0, -size * 0.7); c.lineTo(size * 0.5, size * 0.4); c.closePath(); c.fill();
                c.fillStyle = '#b98a4c';
                c.beginPath(); c.moveTo(-size * 0.22, size * 0.4); c.lineTo(0, -size * 0.7); c.lineTo(size * 0.1, size * 0.4); c.closePath(); c.fill();
                break;
            case 'harbour':
                c.fillStyle = '#2c4055'; c.fillRect(-size * 0.6, -size * 0.4, size * 1.2, size * 0.8);
                c.strokeStyle = '#4f7691'; c.lineWidth = 1;
                c.strokeRect(-size * 0.6, -size * 0.4, size * 1.2, size * 0.8);
                break;
            case 'industry':
                c.fillStyle = '#4a5057'; c.fillRect(-size * 0.4, -size * 0.55, size * 0.8, size * 1.1);
                c.fillStyle = '#ff9d4a'; c.globalAlpha = 0.5;
                c.fillRect(-size * 0.25, -size * 0.3, size * 0.5, size * 0.15);
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

        function drawRoad(track, cam) {
            const samples = track.samples;
            const w = track.width * cam.scale;
            c.lineCap = 'round'; c.lineJoin = 'round';
            c.beginPath();
            for (let i = 0; i < samples.length; i++) {
                const p = samples[i];
                const sx = width * 0.5 + (p.x - cam.x) * cam.scale;
                const sy = height * 0.5 + (p.y - cam.y) * cam.scale;
                if (i === 0) c.moveTo(sx, sy); else c.lineTo(sx, sy);
            }
            c.closePath();
            c.strokeStyle = track.biome.edge; c.lineWidth = w + 7; c.stroke();
            c.strokeStyle = '#b83232'; c.lineWidth = w + 3;
            c.setLineDash([14, 14]); c.lineDashOffset = -(frame * 0.2) % 28; c.stroke();
            c.setLineDash([]);
            c.strokeStyle = track.biome.road; c.lineWidth = w; c.stroke();

            /* Subtle asphalt grain and a wet sheen for night biomes. */
            if (quality.lighting) {
                c.strokeStyle = track.biome.night ? 'rgba(120,170,210,0.07)' : 'rgba(255,255,255,0.035)';
                c.lineWidth = w * 0.62; c.stroke();
            }
            c.strokeStyle = track.biome.line; c.globalAlpha = 0.55;
            c.lineWidth = Math.max(1.5, cam.scale * 0.35);
            c.setLineDash([18, 22]); c.lineDashOffset = -(frame * 0.15) % 40; c.stroke();
            c.setLineDash([]); c.globalAlpha = 1;
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
            c.save(); c.translate(sx, sy); c.rotate(v.psi);
            if (ghost) c.globalAlpha = 0.32;

            c.fillStyle = 'rgba(0,0,0,0.38)';
            c.beginPath(); c.ellipse(1.5, 2.2, len * 0.55, wid * 0.52, 0, 0, Math.PI * 2); c.fill();

            c.fillStyle = '#15181c';
            c.fillRect(-len * 0.48, -wid * 0.62, len * 0.22, wid * 0.18);
            c.fillRect(-len * 0.48, wid * 0.44, len * 0.22, wid * 0.18);
            c.fillRect(len * 0.28, -wid * 0.62, len * 0.22, wid * 0.18);
            c.fillRect(len * 0.28, wid * 0.44, len * 0.22, wid * 0.18);

            const body = c.createLinearGradient(0, -wid / 2, 0, wid / 2);
            const colour = ghost ? '#9fd6ff' : (v.car ? v.car.colour : '#ff5a4e');
            body.addColorStop(0, colour);
            body.addColorStop(0.5, ghost ? '#6ea8d8' : shade(colour, -0.28));
            body.addColorStop(1, shade(colour, -0.55));
            c.fillStyle = body;
            roundRect(c, -len / 2, -wid / 2, len, wid, wid * 0.28); c.fill();

            c.fillStyle = 'rgba(18,28,40,0.82)';
            roundRect(c, -len * 0.12, -wid * 0.38, len * 0.34, wid * 0.76, wid * 0.12); c.fill();
            c.fillStyle = 'rgba(190,225,255,0.35)';
            c.fillRect(-len * 0.1, -wid * 0.32, len * 0.1, wid * 0.64);
            c.fillStyle = ghost ? '#9fd6ff' : (v.car ? v.car.accent : '#ffd7c2');
            c.fillRect(len * 0.44, -wid * 0.42, len * 0.05, wid * 0.84);

            if (!ghost) {
                c.fillStyle = v.brake > 0.05 ? '#ff3b30' : '#8a231c';
                c.fillRect(-len * 0.5, -wid * 0.42, len * 0.05, wid * 0.22);
                c.fillRect(-len * 0.5, wid * 0.2, len * 0.05, wid * 0.22);
                if (v.boost) {
                    const glow = c.createLinearGradient(-len * 0.8, 0, -len * 0.45, 0);
                    glow.addColorStop(0, 'rgba(90,200,255,0)');
                    glow.addColorStop(1, 'rgba(160,235,255,0.9)');
                    c.fillStyle = glow;
                    c.fillRect(-len * 0.8, -wid * 0.2, len * 0.35, wid * 0.4);
                }
                if (v.shield > 0) {
                    c.strokeStyle = 'rgba(94,234,212,0.85)'; c.lineWidth = 2;
                    c.beginPath(); c.ellipse(0, 0, len * 0.62, wid * 0.9, 0, 0, Math.PI * 2); c.stroke();
                }
                if (v.disabled > 0) {
                    c.strokeStyle = 'rgba(56,189,248,0.8)'; c.lineWidth = 1.5;
                    c.beginPath(); c.arc(0, 0, len * 0.6, 0, Math.PI * 2); c.stroke();
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
                const radius = 42 * cam.scale;
                const glow = c.createRadialGradient(sx, sy, 0, sx, sy, radius);
                glow.addColorStop(0, 'rgba(210,235,255,0.22)');
                glow.addColorStop(1, 'rgba(120,180,255,0)');
                c.fillStyle = glow;
                c.beginPath(); c.arc(sx, sy, radius, 0, Math.PI * 2); c.fill();
            }
            if (!reducedMotion) {
                for (const light of lights) {
                    const [sx, sy] = worldToScreen(light.x, light.y, cam);
                    if (sx < -100 || sx > width + 100 || sy < -100 || sy > height + 100) continue;
                    const glow = c.createRadialGradient(sx, sy, 0, sx, sy, 16 * cam.scale);
                    glow.addColorStop(0, 'rgba(255,190,110,0.20)');
                    glow.addColorStop(1, 'rgba(255,170,80,0)');
                    c.fillStyle = glow;
                    c.beginPath(); c.arc(sx, sy, 16 * cam.scale, 0, Math.PI * 2); c.fill();
                }
            }
            c.restore();
        }

        function drawWeather(race, cam, dt, reducedMotion) {
            const weather = race.track.biome.weather;
            if (!weather || reducedMotion || quality.weather <= 0) return;
            const amount = Math.floor(quality.weather * (weather === 'rain' ? 90 : 45));
            c.save();
            c.strokeStyle = weather === 'rain' ? 'rgba(170,205,240,0.32)' :
                (weather === 'sparks' ? 'rgba(255,170,80,0.28)' : 'rgba(230,230,215,0.16)');
            c.lineWidth = 1;
            for (let i = 0; i < amount; i++) {
                const x = (i * 97 + frame * (weather === 'rain' ? 7 : 1.4)) % (width + 80) - 40;
                const y = (i * 53 + frame * (weather === 'rain' ? 16 : 2)) % (height + 80) - 40;
                c.beginPath(); c.moveTo(x, y);
                c.lineTo(x - (weather === 'rain' ? 5 : 1), y + (weather === 'rain' ? 15 : 3));
                c.stroke();
            }
            if (weather === 'fog' || weather === 'mist' || weather === 'glare') {
                c.fillStyle = weather === 'glare' ? 'rgba(255,255,240,0.10)' : 'rgba(200,215,205,0.10)';
                c.fillRect(0, 0, width, height);
            }
            c.restore();
        }

        function drawVignette(reducedMotion, speed) {
            const g = c.createRadialGradient(width / 2, height / 2, height * 0.28, width / 2, height / 2, height * 0.86);
            g.addColorStop(0, 'rgba(0,0,0,0)');
            g.addColorStop(1, 'rgba(6,8,14,0.55)');
            c.fillStyle = g; c.fillRect(0, 0, width, height);
            if (!reducedMotion && speed > 45 && quality.bloom) {
                c.strokeStyle = 'rgba(255,255,255,0.05)';
                c.lineWidth = 1;
                for (let i = 0; i < 12; i++) {
                    const y = (i * 71 + frame * 3) % height;
                    c.beginPath(); c.moveTo(0, y); c.lineTo(width * 0.16, y - 6); c.stroke();
                    c.beginPath(); c.moveTo(width, y); c.lineTo(width * 0.84, y - 6); c.stroke();
                }
            }
        }

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
                for (let i = 0; i < track.gates.length; i += 2) {
                    const p = AD.pointAt(track, track.gates[i]);
                    lights.push({ x: p.x + p.nx * (track.width * 0.5 + 6), y: p.y + p.ny * (track.width * 0.5 + 6) });
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
                const track = race.track;
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
                drawDecals(cam);
                drawRoad(track, cam);

                /* Pickups, hazards and projectiles live in world space. */
                for (const pad of race.pickups) {
                    if (race.time - pad.taken < 8) continue;
                    const [sx, sy] = worldToScreen(pad.x, pad.y, cam);
                    if (sx < -20 || sx > width + 20 || sy < -20 || sy > height + 20) continue;
                    c.save(); c.translate(sx, sy); c.rotate(frame * 0.02);
                    c.fillStyle = 'rgba(120,230,255,0.22)';
                    c.beginPath(); c.arc(0, 0, 3.2 * cam.scale, 0, Math.PI * 2); c.fill();
                    c.fillStyle = '#8ce9ff';
                    c.fillRect(-1.1 * cam.scale, -1.1 * cam.scale, 2.2 * cam.scale, 2.2 * cam.scale);
                    c.restore();
                }
                for (const h of race.hazards) {
                    const [sx, sy] = worldToScreen(h.x, h.y, cam);
                    if (h.type === 'oil') {
                        c.fillStyle = 'rgba(10,10,14,0.72)';
                        c.beginPath(); c.ellipse(sx, sy, h.r * cam.scale, h.r * cam.scale * 0.7, 0, 0, Math.PI * 2); c.fill();
                    } else {
                        c.strokeStyle = h.arm > 0 ? '#f97316' : '#ffd166';
                        c.lineWidth = 2;
                        c.beginPath(); c.arc(sx, sy, h.r * cam.scale, 0, Math.PI * 2); c.stroke();
                    }
                }
                for (const p of race.projectiles) {
                    const [sx, sy] = worldToScreen(p.x, p.y, cam);
                    c.fillStyle = p.type === 'rocket' ? '#fb7185' : '#facc15';
                    c.beginPath(); c.arc(sx, sy, (p.type === 'rocket' ? 1.5 : 0.7) * cam.scale, 0, Math.PI * 2); c.fill();
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
