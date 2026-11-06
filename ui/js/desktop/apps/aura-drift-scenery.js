/* Aura Drift - world building: terrain chunks, scenery props and trackside furniture. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const CHUNK_PX = 320;

    /* --- terrain -------------------------------------------------------- */
    AD.makeChunkCanvas = function (track, cx, cy) {
        const canvas = document.createElement('canvas');
        canvas.width = CHUNK_PX; canvas.height = CHUNK_PX;
        const c = canvas.getContext('2d');
        const biome = track.biome;
        const rng = AD.rng(AD.hashString(track.id + '/' + track.seed + '/' + cx + '/' + cy));
        c.fillStyle = biome.ground;
        c.fillRect(0, 0, CHUNK_PX, CHUNK_PX);

        /* Broad tonal patches break the flat fill. */
        for (let i = 0; i < 26; i++) {
            const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
            const r = 16 + rng() * 72;
            const g = c.createRadialGradient(x, y, 0, x, y, r);
            const tint = rng() > 0.5 ? biome.groundAlt : biome.edge;
            g.addColorStop(0, tint);
            g.addColorStop(1, 'rgba(0,0,0,0)');
            c.globalAlpha = 0.05 + rng() * 0.08;
            c.fillStyle = g;
            c.beginPath(); c.arc(x, y, r, 0, Math.PI * 2); c.fill();
        }
        c.globalAlpha = 1;

        /* Fine grain: stones, grit and grass blades. */
        for (let i = 0; i < 520; i++) {
            const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
            c.fillStyle = rng() > 0.55 ? biome.edge : biome.groundAlt;
            c.globalAlpha = 0.05 + rng() * 0.13;
            c.fillRect(x, y, 1 + rng() * 1.4, 1 + rng() * 1.4);
        }
        c.globalAlpha = 1;
        for (let i = 0; i < 90; i++) {
            const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
            c.strokeStyle = rng() > 0.5 ? 'rgba(255,255,255,0.05)' : 'rgba(0,0,0,0.08)';
            c.lineWidth = 1;
            c.beginPath(); c.moveTo(x, y); c.lineTo(x + (rng() - 0.5) * 4, y - 1 - rng() * 3); c.stroke();
        }

        const id = track.biomeId;
        if (id === 'salt') {
            /* Polygonal salt crust. */
            c.strokeStyle = 'rgba(120,130,125,0.16)'; c.lineWidth = 1;
            for (let i = 0; i < 16; i++) {
                c.beginPath();
                let x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
                c.moveTo(x, y);
                for (let j = 0; j < 4; j++) { x += (rng() - 0.5) * 48; y += (rng() - 0.5) * 48; c.lineTo(x, y); }
                c.stroke();
            }
        } else if (id === 'desert') {
            /* Wind-blown sand ripples. */
            c.strokeStyle = 'rgba(228,190,130,0.10)'; c.lineWidth = 1.5;
            for (let i = 0; i < 10; i++) {
                const y = rng() * CHUNK_PX;
                c.beginPath();
                c.moveTo(0, y);
                for (let x = 0; x <= CHUNK_PX; x += 24) c.lineTo(x, y + Math.sin(x * 0.05 + rng() * 2) * 5);
                c.stroke();
            }
        } else if (id === 'harbour' || id === 'metro') {
            /* Poured concrete slabs with expansion joints. */
            c.strokeStyle = 'rgba(140,170,200,0.10)'; c.lineWidth = 2;
            for (let i = 0; i < 6; i++) {
                const x = rng() * CHUNK_PX;
                c.beginPath(); c.moveTo(x, 0); c.lineTo(x, CHUNK_PX); c.stroke();
            }
            c.strokeStyle = 'rgba(0,0,0,0.12)';
            for (let i = 0; i < 4; i++) {
                const y = rng() * CHUNK_PX;
                c.beginPath(); c.moveTo(0, y); c.lineTo(CHUNK_PX, y); c.stroke();
            }
        } else if (id === 'industry') {
            /* Steel decking with oil staining. */
            c.strokeStyle = 'rgba(150,165,180,0.10)'; c.lineWidth = 2;
            for (let i = 0; i < 6; i++) {
                const x = rng() * CHUNK_PX;
                c.beginPath(); c.moveTo(x, 0); c.lineTo(x, CHUNK_PX); c.stroke();
            }
            for (let i = 0; i < 5; i++) {
                const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX, r = 10 + rng() * 26;
                const g = c.createRadialGradient(x, y, 0, x, y, r);
                g.addColorStop(0, 'rgba(20,14,10,0.30)');
                g.addColorStop(1, 'rgba(20,14,10,0)');
                c.fillStyle = g;
                c.beginPath(); c.arc(x, y, r, 0, Math.PI * 2); c.fill();
            }
        } else if (id === 'forest') {
            /* Leaf litter and moss. */
            for (let i = 0; i < 160; i++) {
                const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
                c.fillStyle = rng() > 0.5 ? 'rgba(126,158,90,0.16)' : 'rgba(58,44,26,0.18)';
                c.beginPath();
                c.ellipse(x, y, 1.5 + rng() * 3, 1 + rng() * 2, rng() * 3, 0, Math.PI * 2);
                c.fill();
            }
        } else if (id === 'alpine') {
            /* Scree and old snow patches. */
            for (let i = 0; i < 90; i++) {
                const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX, r = 8 + rng() * 30;
                const g = c.createRadialGradient(x, y, 0, x, y, r);
                g.addColorStop(0, 'rgba(232,240,246,0.14)');
                g.addColorStop(1, 'rgba(232,240,246,0)');
                c.fillStyle = g;
                c.beginPath(); c.arc(x, y, r, 0, Math.PI * 2); c.fill();
            }
        } else if (id === 'glacier') {
            /* Blue ice veins and drift snow. */
            for (let i = 0; i < 70; i++) {
                const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
                const g = c.createRadialGradient(x, y, 0, x, y, 12 + rng() * 40);
                g.addColorStop(0, 'rgba(198,230,255,0.18)');
                g.addColorStop(1, 'rgba(198,230,255,0)');
                c.fillStyle = g;
                c.beginPath(); c.arc(x, y, 12 + rng() * 40, 0, Math.PI * 2); c.fill();
            }
            c.strokeStyle = 'rgba(214,238,255,0.18)'; c.lineWidth = 1.4;
            for (let i = 0; i < 10; i++) {
                c.beginPath();
                let x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
                c.moveTo(x, y);
                for (let j = 0; j < 5; j++) { x += (rng() - 0.5) * 60; y += (rng() - 0.5) * 60; c.lineTo(x, y); }
                c.stroke();
            }
        }
        return canvas;
    };

    /* --- scenery props --------------------------------------------------- */
    function propShape(c, prop, size) {
        switch (prop.kind) {
            case 'alpine': {
                c.fillStyle = '#4a3b2c';
                c.fillRect(-size * 0.07, size * 0.1, size * 0.14, size * 0.38);
                c.fillStyle = '#1c3323';
                c.beginPath(); c.moveTo(-size * 0.52, size * 0.22); c.lineTo(0, -size * 0.72); c.lineTo(size * 0.52, size * 0.22); c.closePath(); c.fill();
                c.fillStyle = '#2c5236';
                c.beginPath(); c.moveTo(-size * 0.38, -size * 0.02); c.lineTo(0, -size * 0.62); c.lineTo(size * 0.38, -size * 0.02); c.closePath(); c.fill();
                c.fillStyle = 'rgba(226,238,246,0.5)';
                c.beginPath(); c.moveTo(-size * 0.16, -size * 0.36); c.lineTo(0, -size * 0.62); c.lineTo(size * 0.16, -size * 0.36); c.closePath(); c.fill();
                break;
            }
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
            case 'glacier':
                c.fillStyle = 'rgba(188,222,255,0.55)';
                c.beginPath();
                c.moveTo(-size * 0.5, size * 0.4); c.lineTo(-size * 0.3, -size * 0.5);
                c.lineTo(size * 0.1, -size * 0.66); c.lineTo(size * 0.52, size * 0.36);
                c.closePath(); c.fill();
                c.fillStyle = 'rgba(240,250,255,0.6)';
                c.beginPath();
                c.moveTo(-size * 0.16, size * 0.38); c.lineTo(size * 0.02, -size * 0.42);
                c.lineTo(size * 0.2, -size * 0.36); c.lineTo(size * 0.08, size * 0.38);
                c.closePath(); c.fill();
                break;
            case 'metro': {
                c.fillStyle = '#3c4148';
                c.fillRect(-size * 0.5, -size * 0.34, size * 1.0, size * 0.68);
                c.fillStyle = '#5b636d';
                c.fillRect(-size * 0.5, -size * 0.34, size * 1.0, size * 0.12);
                c.fillStyle = 'rgba(120,220,255,0.5)';
                c.fillRect(-size * 0.34, -size * 0.16, size * 0.68, size * 0.1);
                break;
            }
            default:
                c.fillStyle = '#8f9a86';
                c.beginPath(); c.arc(0, 0, size * 0.4, 0, Math.PI * 2); c.fill();
        }
    }

    AD.drawProp = function (c, prop, sx, sy, scale) {
        const size = prop.size * 3.3 * scale;
        c.save(); c.translate(sx, sy); c.rotate(prop.angle);
        c.fillStyle = 'rgba(6,10,16,0.30)';
        c.beginPath(); c.ellipse(size * 0.16, size * 0.2, size * 0.62, size * 0.4, 0, 0, Math.PI * 2); c.fill();
        propShape(c, prop, size);
        /* Rim light from the top-left grounds every prop in sun and neon. */
        c.fillStyle = 'rgba(255,255,255,0.08)';
        c.beginPath(); c.arc(-size * 0.15, -size * 0.2, size * 0.3, 0, Math.PI * 2); c.fill();
        c.restore();
    };

    /* --- trackside furniture --------------------------------------------- */
    /* Every item is world anchored, deterministic per track and kept clear of
       the drivable corridor so it never becomes an invisible wall. */
    AD.trackFurniture = function (track) {
        const rng = AD.rng(AD.hashString(track.id + '/furniture/' + track.seed));
        const items = [];
        const biomeId = track.biomeId;

        function clear(x, y, margin) {
            const sample = AD.sampleTrack(track, x, y);
            return Math.abs(sample.offset) > sample.halfWidth + (margin || 3.5);
        }

        function place(kind, s, side, extra) {
            const p = AD.pointAt(track, s);
            const offset = p.halfWidth + (extra && extra.offset !== undefined ? extra.offset : 4.2);
            const x = p.x + p.nx * offset * side;
            const y = p.y + p.ny * offset * side;
            if (!clear(x, y, (extra && extra.margin) || 2.5)) return null;
            const item = Object.assign({
                kind: kind, x: x, y: y,
                angle: Math.atan2(p.ny * side, p.nx * side),
                size: 1, seed: Math.floor(rng() * 100000)
            }, extra || {});
            items.push(item);
            return item;
        }

        /* Grandstands and pit structures frame the start straight. */
        const standOffsets = [18, 34, -26];
        for (const off of standOffsets) {
            place('grandstand', track.length * 0.02 + off, off < 0 ? -1 : 1, { offset: 9 + rng() * 6, width: 26 + rng() * 10 });
        }
        place('paddock', track.length * 0.94, -1, { offset: 11, width: 22 });

        /* Sponsor hoardings every 70 m, alternating sides. */
        for (let s = 0, i = 0; s < track.length; s += 70, i++) {
            place('board', s, i % 2 === 0 ? 1 : -1, { width: 12 + rng() * 5, accent: i % 3 });
        }

        /* Marshal posts and braking boards at the tight corners. */
        const samples = track.samples;
        for (let i = 0; i < samples.length; i += 11) {
            const a = samples[i];
            const b = samples[(i + 16) % samples.length];
            const cross = Math.abs(a.tx * b.ty - a.ty * b.tx);
            if (cross < 0.2 || cross > 0.85) continue;
            const turn = a.tx * b.ny - a.ty * b.nx;
            const side = turn > 0 ? 1 : -1;
            place('marshal', a.s + 12, side, {});
            place('marker100', a.s + 46, side, { label: '100' });
            place('marker50', a.s + 66, side, { label: '50' });
            place('tyrewall', a.s + 8, side, { width: 16 });
            if (items.length > 260) break;
        }

        /* Safety fencing runs beside the fastest sections. */
        for (let s = 0; s < track.length; s += 46) {
            if (rng() > 0.55) continue;
            place('fence', s, rng() > 0.5 ? 1 : -1, { width: 22 });
        }

        /* Biome set pieces: the things that make a venue recognisable. */
        const setPieces = {
            alpine: ['pine', 'pine', 'rock', 'cabin'],
            desert: ['cactus', 'rock', 'billboard', 'dune'],
            harbour: ['crane', 'container', 'container', 'lightmast'],
            salt: ['beacon', 'sign', 'rock'],
            forest: ['pine', 'pine', 'stump', 'hut'],
            industry: ['silo', 'pipe', 'pylon', 'lightmast'],
            glacier: ['iceblock', 'iceblock', 'mast', 'aurora'],
            metro: ['neon', 'neon', 'lightmast', 'barrier']
        };
        const kinds = setPieces[biomeId] || ['rock'];
        for (let i = 0; i < 90; i++) {
            const s = rng() * track.length;
            const p = AD.pointAt(track, s);
            const side = rng() > 0.5 ? 1 : -1;
            const offset = p.halfWidth + 12 + rng() * 90;
            const x = p.x + p.nx * offset * side;
            const y = p.y + p.ny * offset * side;
            if (!clear(x, y, 4)) continue;
            items.push({
                kind: kinds[Math.floor(rng() * kinds.length)],
                x: x, y: y, angle: rng() * Math.PI * 2, size: 0.8 + rng() * 0.9,
                seed: Math.floor(rng() * 100000)
            });
        }
        return items;
    };

    AD.drawFurniture = function (c, item, sx, sy, scale, frame) {
        const s = scale;
        const size = (item.size || 1) * s;
        c.save();
        c.translate(sx, sy);
        c.rotate(item.angle);
        const accent = item.accent === 1 ? '#5eead4' : (item.accent === 2 ? '#ffd166' : '#ff5a4e');

        switch (item.kind) {
            case 'grandstand': {
                const w = (item.width || 24) * s;
                const d = 8 * s;
                c.fillStyle = 'rgba(6,10,16,0.32)';
                c.fillRect(-d / 2 + 2 * s, -w / 2 + 2 * s, d, w);
                /* Tiers of seating with a dense crowd: the stand has to read
                   as a building full of people, not a grey slab. */
                const tiers = 4;
                const seatDepth = d * 0.55;
                for (let i = 0; i < tiers; i++) {
                    const x = -d / 2 + (i / tiers) * seatDepth;
                    const band = seatDepth / tiers;
                    c.fillStyle = i % 2 ? '#2b313a' : '#242a33';
                    c.fillRect(x, -w / 2, band, w);
                    for (let j = 0; j < 30; j++) {
                        const cy = -w / 2 + ((j * 37 + i * 13) % w);
                        const cx = x + band * (0.22 + ((j * 17) % 52) / 100);
                        c.fillStyle = ['rgba(226,234,246,0.55)', 'rgba(160,190,220,0.45)',
                            'rgba(236,198,162,0.42)', 'rgba(200,214,236,0.38)'][j % 4];
                        c.fillRect(cx, cy, 1.1 * s, 1.7 * s);
                    }
                }
                /* Roof over the outer half, carried on pillars. */
                c.fillStyle = '#39404a';
                c.fillRect(d * 0.04, -w / 2 - 1.2 * s, d * 0.46, w + 2.4 * s);
                c.fillStyle = '#4d5764';
                c.fillRect(d * 0.04, -w / 2 - 1.2 * s, d * 0.46, 1.5 * s);
                c.fillStyle = 'rgba(16,20,26,0.85)';
                for (let i = 0; i <= 6; i++) {
                    const cy = -w / 2 + (i / 6) * w;
                    c.fillRect(d * 0.02, cy - 0.5 * s, d * 0.5, 1 * s);
                }
                /* Safety fence between the stand and the track. */
                c.fillStyle = 'rgba(190,205,225,0.20)';
                c.fillRect(-d / 2 - 0.7 * s, -w / 2, 0.5 * s, w);
                break;
            }
            case 'paddock': {
                const w = (item.width || 22) * s;
                c.fillStyle = 'rgba(6,10,16,0.3)';
                c.fillRect(-4 * s + 2 * s, -w / 2 + 2 * s, 8 * s, w);
                c.fillStyle = '#2f353e';
                c.fillRect(-4 * s, -w / 2, 8 * s, w);
                /* Garage bays with lit doorways. */
                for (let i = 0; i < 5; i++) {
                    const y = -w / 2 + (i + 0.5) * (w / 5);
                    c.fillStyle = '#171b21';
                    c.fillRect(-3.2 * s, y - 1.8 * s, 5.4 * s, 3.6 * s);
                    c.fillStyle = 'rgba(130,160,190,0.20)';
                    c.fillRect(-3.2 * s, y - 1.8 * s, 5.4 * s, 0.8 * s);
                }
                /* Roof band in the team colour. */
                c.fillStyle = '#454e5a';
                c.fillRect(-4.2 * s, -w / 2, 8.4 * s, 1.8 * s);
                c.fillStyle = accent;
                c.globalAlpha = 0.85;
                c.fillRect(-4.2 * s, -w / 2 + 1.2 * s, 8.4 * s, 0.6 * s);
                c.globalAlpha = 1;
                break;
            }
            case 'board': {
                const w = (item.width || 12) * s;
                c.fillStyle = 'rgba(6,10,16,0.3)';
                c.fillRect(-1.4 * s + 1.4 * s, -w / 2 + 1.4 * s, 2.8 * s, w);
                c.fillStyle = '#e8ebef';
                c.fillRect(-1.4 * s, -w / 2, 2.8 * s, w);
                c.fillStyle = accent;
                for (let i = 0; i < 3; i++) {
                    const y = -w / 2 + (i + 0.6) * (w / 3.4);
                    c.fillRect(-1.1 * s, y, 2.2 * s, w / 8);
                }
                c.fillStyle = 'rgba(20,24,30,0.55)';
                c.fillRect(-1.4 * s, -w / 2, 2.8 * s, 0.7 * s);
                break;
            }
            case 'tyrewall': {
                const w = (item.width || 16) * s;
                const r = 1.5 * s;
                for (let i = 0; i < w / (r * 2.1); i++) {
                    const y = -w / 2 + i * r * 2.1;
                    c.fillStyle = 'rgba(8,10,14,0.55)';
                    c.beginPath(); c.arc(0, y, r, 0, Math.PI * 2); c.fill();
                    c.fillStyle = i % 2 ? '#e8ecef' : '#c22f2f';
                    c.beginPath(); c.arc(0, y, r * 0.8, 0, Math.PI * 2); c.fill();
                    c.fillStyle = 'rgba(8,10,14,0.55)';
                    c.beginPath(); c.arc(0, y, r * 0.34, 0, Math.PI * 2); c.fill();
                }
                break;
            }
            case 'fence': {
                const w = (item.width || 22) * s;
                c.strokeStyle = 'rgba(190,205,225,0.22)';
                c.lineWidth = 0.7 * s;
                c.beginPath(); c.moveTo(0, -w / 2); c.lineTo(0, w / 2); c.stroke();
                for (let i = 0; i <= 8; i++) {
                    const y = -w / 2 + (i / 8) * w;
                    c.fillStyle = 'rgba(120,132,148,0.5)';
                    c.fillRect(-0.5 * s, y - 0.5 * s, 1.1 * s, 1.1 * s);
                }
                break;
            }
            case 'marshal': {
                c.fillStyle = 'rgba(6,10,16,0.32)';
                c.beginPath(); c.ellipse(0.6 * s, 1.4 * s, 2.6 * s, 1.8 * s, 0, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#e8ebef';
                c.fillRect(-1.6 * s, -1.6 * s, 3.2 * s, 3.2 * s);
                c.fillStyle = '#ffcf6b';
                c.fillRect(-1.6 * s, -1.6 * s, 3.2 * s, 1.1 * s);
                /* Waving flag. */
                const wave = Math.sin((frame || 0) * 0.12) * 0.3;
                c.save();
                c.rotate(wave);
                c.fillStyle = '#ff5a4e';
                c.beginPath();
                c.moveTo(0, 0); c.lineTo(3.6 * s, -0.8 * s); c.lineTo(0, -1.7 * s);
                c.closePath(); c.fill();
                c.restore();
                break;
            }
            case 'marker100':
            case 'marker50': {
                c.fillStyle = 'rgba(6,10,16,0.3)';
                c.beginPath(); c.ellipse(0.5 * s, 1.2 * s, 1.9 * s, 1.2 * s, 0, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#f4f6f2';
                c.fillRect(-1.3 * s, -1.3 * s, 2.6 * s, 2.6 * s);
                c.fillStyle = '#12141a';
                c.font = 'bold ' + (1.5 * s) + 'px system-ui, sans-serif';
                c.textAlign = 'center';
                c.textBaseline = 'middle';
                c.fillText(item.label || '50', 0, 0.1 * s);
                break;
            }
            case 'crane': {
                c.fillStyle = 'rgba(6,10,16,0.3)';
                c.fillRect(-1 * s + 3 * s, -2 * s + 3 * s, 2 * s, 4 * s);
                c.fillStyle = '#6b747d';
                c.fillRect(-1 * s, -2 * s, 2 * s, 4 * s);
                c.fillStyle = '#8b959f';
                c.fillRect(-1 * s, -30 * s, 2 * s, 30 * s);
                c.fillRect(-1 * s, -30 * s, 26 * s, 2 * s);
                c.fillStyle = '#ffd166';
                c.fillRect(-1 * s, -28.6 * s, 26 * s, 0.8 * s);
                break;
            }
            case 'container': {
                const hue = ['#2c4055', '#5a3b46', '#3b5546', '#57492e'][(item.seed || 0) % 4];
                c.fillStyle = 'rgba(6,10,16,0.3)';
                c.fillRect(-5 * s + 1.6 * s, -3.2 * s + 1.6 * s, 10 * s, 6.4 * s);
                c.fillStyle = hue;
                c.fillRect(-5 * s, -3.2 * s, 10 * s, 6.4 * s);
                c.strokeStyle = 'rgba(190,215,235,0.28)'; c.lineWidth = 0.8 * s;
                for (let i = -2; i <= 2; i++) {
                    c.beginPath();
                    c.moveTo(i * 2 * s, -3.2 * s); c.lineTo(i * 2 * s, 3.2 * s);
                    c.stroke();
                }
                break;
            }
            case 'silo': {
                c.fillStyle = 'rgba(6,10,16,0.3)';
                c.beginPath(); c.ellipse(1.4 * s, 1.8 * s, 4 * s, 3.4 * s, 0, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#6a7079';
                c.beginPath(); c.arc(0, 0, 4 * s, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#8d959f';
                c.beginPath(); c.arc(-0.8 * s, -0.8 * s, 2.2 * s, 0, Math.PI * 2); c.fill();
                break;
            }
            case 'pipe': {
                c.fillStyle = 'rgba(6,10,16,0.28)';
                c.fillRect(-1.4 * s + 1.2 * s, -12 * s + 1.2 * s, 2.8 * s, 24 * s);
                c.fillStyle = '#5a6169';
                c.fillRect(-1.4 * s, -12 * s, 2.8 * s, 24 * s);
                c.fillStyle = 'rgba(255,255,255,0.12)';
                c.fillRect(-1.4 * s, -12 * s, 0.8 * s, 24 * s);
                break;
            }
            case 'pylon': {
                c.strokeStyle = 'rgba(120,132,148,0.55)';
                c.lineWidth = 0.9 * s;
                c.beginPath();
                c.moveTo(-2.4 * s, 5 * s); c.lineTo(0, -6 * s); c.lineTo(2.4 * s, 5 * s);
                c.moveTo(-1.6 * s, 1 * s); c.lineTo(1.6 * s, 1 * s);
                c.moveTo(-0.8 * s, -3 * s); c.lineTo(0.8 * s, -3 * s);
                c.stroke();
                break;
            }
            case 'lightmast': {
                c.fillStyle = 'rgba(6,10,16,0.26)';
                c.beginPath(); c.ellipse(0.8 * s, 4 * s, 1.2 * s, 1.2 * s, 0, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#3a414a';
                c.fillRect(-0.5 * s, -7 * s, 1 * s, 11 * s);
                c.fillStyle = '#20252c';
                c.fillRect(-1.8 * s, -7.6 * s, 3.6 * s, 1.4 * s);
                c.fillStyle = 'rgba(255,214,150,0.9)';
                c.fillRect(-1.4 * s, -6.4 * s, 2.8 * s, 0.7 * s);
                break;
            }
            case 'mast': {
                c.strokeStyle = 'rgba(190,205,225,0.5)';
                c.lineWidth = 1 * s;
                c.beginPath(); c.moveTo(0, 4 * s); c.lineTo(0, -9 * s); c.stroke();
                c.beginPath();
                c.moveTo(-3 * s, 4 * s); c.lineTo(0, -2 * s); c.lineTo(3 * s, 4 * s);
                c.stroke();
                const blink = ((frame || 0) + (item.seed || 0)) % 90 < 45;
                c.fillStyle = blink ? '#ff4d4d' : 'rgba(255,77,77,0.25)';
                c.beginPath(); c.arc(0, -9 * s, 0.9 * s, 0, Math.PI * 2); c.fill();
                break;
            }
            case 'aurora': {
                /* A slow ribbon of northern light over the glacier. */
                const g = c.createLinearGradient(-14 * s, 0, 14 * s, 0);
                g.addColorStop(0, 'rgba(94,234,212,0)');
                g.addColorStop(0.4, 'rgba(94,234,212,0.30)');
                g.addColorStop(0.7, 'rgba(159,122,255,0.22)');
                g.addColorStop(1, 'rgba(94,234,212,0)');
                c.fillStyle = g;
                c.beginPath();
                c.moveTo(-14 * s, -2 * s);
                for (let i = 0; i <= 8; i++) {
                    const x = -14 * s + i * 3.5 * s;
                    c.lineTo(x, -2 * s + Math.sin(i * 0.9 + (frame || 0) * 0.02) * 2.4 * s);
                }
                c.lineTo(14 * s, 3 * s);
                for (let i = 8; i >= 0; i--) {
                    const x = -14 * s + i * 3.5 * s;
                    c.lineTo(x, 3 * s + Math.sin(i * 0.9 + (frame || 0) * 0.02) * 2.4 * s);
                }
                c.closePath(); c.fill();
                break;
            }
            case 'iceblock': {
                c.fillStyle = 'rgba(188,222,255,0.28)';
                c.beginPath();
                c.moveTo(-4 * s, 3 * s); c.lineTo(-2.6 * s, -3.4 * s);
                c.lineTo(1 * s, -4.4 * s); c.lineTo(4.2 * s, 2.8 * s);
                c.closePath(); c.fill();
                c.strokeStyle = 'rgba(230,246,255,0.45)'; c.lineWidth = 0.8 * s;
                c.stroke();
                break;
            }
            case 'snowdrift': {
                c.fillStyle = 'rgba(236,244,252,0.4)';
                c.beginPath(); c.ellipse(0, 0, 6 * s, 3 * s, 0.3, 0, Math.PI * 2); c.fill();
                break;
            }
            case 'neon': {
                const pulse = 0.7 + 0.3 * Math.sin((frame || 0) * 0.08 + (item.seed || 0));
                c.fillStyle = 'rgba(8,10,16,0.4)';
                c.fillRect(-1.6 * s + 1.4 * s, -5 * s + 1.4 * s, 3.2 * s, 10 * s);
                c.fillStyle = 'rgba(18,22,30,0.95)';
                c.fillRect(-1.6 * s, -5 * s, 3.2 * s, 10 * s);
                c.fillStyle = accent;
                c.globalAlpha = 0.55 + pulse * 0.45;
                c.fillRect(-1.2 * s, -4.4 * s, 2.4 * s, 8.8 * s);
                c.globalAlpha = 1;
                c.fillStyle = 'rgba(255,255,255,0.5)';
                c.fillRect(-1.2 * s, -1.2 * s, 2.4 * s, 2.4 * s);
                break;
            }
            case 'barrier': {
                c.fillStyle = 'rgba(6,10,16,0.3)';
                c.fillRect(-0.8 * s + 1.2 * s, -5 * s + 1.2 * s, 1.6 * s, 10 * s);
                c.fillStyle = '#c9ced6';
                c.fillRect(-0.8 * s, -5 * s, 1.6 * s, 10 * s);
                c.fillStyle = '#ffcf6b';
                for (let i = 0; i < 4; i++) c.fillRect(-0.8 * s, -5 * s + i * 2.5 * s, 1.6 * s, 1.2 * s);
                break;
            }
            case 'pine':
            case 'rock':
            case 'cactus':
            case 'billboard':
            case 'dune':
            case 'beacon':
            case 'sign':
            case 'stump':
            case 'hut':
            case 'cabin': {
                /* Landscape props reuse the chunk prop vocabulary, mapped to a
                   readable silhouette per biome. */
                const map = {
                    pine: 'forest', rock: 'salt', cactus: 'desert', billboard: 'salt',
                    dune: 'desert', beacon: 'salt', sign: 'salt', stump: 'forest',
                    hut: 'alpine', cabin: 'alpine'
                };
                AD.drawProp(c, {
                    kind: map[item.kind] || 'alpine',
                    size: 1.5, angle: (item.seed || 0) % 7
                }, 0, 0, s * 0.85);
                break;
            }
            default:
                break;
        }
        c.restore();
    };
})();
