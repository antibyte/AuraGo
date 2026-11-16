/* Runenpfad 2D decor layer: torches, lanterns, ledge caps, mid-ground floaters, grass sway, lava shimmer and ripples. */
(function (root) {
    'use strict';

    const TAU = Math.PI * 2;
    const LEDGES = {
        woodland: { left: 'grass_ledge_left', right: 'grass_ledge_right' },
        desert: { left: 'sand_ledge', right: 'sand_ledge', flipLeft: true },
        glacier: { left: 'snow_ledge', right: 'ice_ledge', flipLeft: true }
    };
    // Mid-ground floaters were dropped: anything behind the gameplay layer must never read as a platform.
    const FLOATERS = {};

    function hash(n, seed) {
        let h = Math.imul(n ^ seed, 0x27d4eb2d);
        h ^= h >>> 15;
        h = Math.imul(h, 0x85ebca6b);
        h ^= h >>> 13;
        return (h >>> 0) / 4294967296;
    }

    function create(h) {
        if (!h || !h.ctx) return null;
        const ctx = h.ctx;
        let prepared = null;
        let data = null;
        let lavaGrad = null;

        function isLava(hz) {
            return hz.assetId === 'lava' || hz.assetId === 'fire';
        }

        function prepare(stage) {
            if (prepared === stage) return data;
            prepared = stage;
            const seed = (Number(stage.id) || Number(stage.levelId) || Math.round(stage.width) || 1) * 2654435 + 17;
            const T = stage.tileSize || 48;
            const biomeId = (stage.biome && stage.biome.id) || 'woodland';
            const solid = stage.solidMap || {};
            const d = { T: T, biomeId: biomeId, seed: seed, torches: [], lanterns: [], chests: [], caps: [], tufts: [], floaters: [], ripples: [], tops: {}, lava: [], shadows: [] };
            const tops = d.tops;
            (stage.tiles || []).forEach(function (t) {
                if (t.kind && t.kind !== 'solid') return;
                if (tops[t.tx] === undefined || t.ty < tops[t.tx]) tops[t.tx] = t.ty;
            });
            const groundTopAt = function (x) {
                const ty = tops[Math.floor(x / T)];
                return ty === undefined ? null : ty * T;
            };
            // torches: beside checkpoints, the key door and the exit portal
            const onHazard = function (x, gy) {
                return (stage.hazards || []).some(function (hz) {
                    return x >= hz.x - 14 && x <= hz.x + hz.w + 14 && hz.y < gy + T && hz.y + hz.h > gy - T;
                });
            };
            const addTorch = function (x, n) {
                const gy = groundTopAt(x);
                if (gy !== null && !onHazard(x, gy)) d.torches.push({ x: x, y: gy, n: n });
            };
            (stage.checkpoints || []).forEach(function (cp, i) { addTorch(cp.x - 38, i * 3); addTorch(cp.x + 38, i * 3 + 1); });
            if (stage.door) { addTorch(stage.door.x - 44, 21); }
            if (stage.portal) { addTorch(stage.portal.x + 46, 22); addTorch(stage.portal.x - 46, 23); }
            // lanterns: sparse along the ground in dark biomes
            if (biomeId === 'cavern' || biomeId === 'citadel') {
                const cols = Object.keys(tops).map(Number).sort(function (a, b) { return a - b; });
                let last = -99;
                cols.forEach(function (tx) {
                    if (tx - last < 9 || tx < 4) return;
                    if (hash(tx, seed) > 0.28) return;
                    const left = tops[tx - 1];
                    const right = tops[tx + 1];
                    if (left !== tops[tx] || right !== tops[tx]) return;
                    // never inside a pit: no higher ground within two tiles, no hazard under the post
                    for (let k = -2; k <= 2; k++) if (tops[tx + k] === undefined || tops[tx + k] < tops[tx]) return;
                    if (onHazard((tx + 0.5) * T, tops[tx] * T)) return;
                    last = tx;
                    d.lanterns.push({ x: (tx + 0.5) * T, y: tops[tx] * T, n: tx });
                });
            }
            (stage.collectibles || []).forEach(function (c) {
                if (c.kind === 'treasure') {
                    const gy = groundTopAt(c.x);
                    if (gy !== null) d.chests.push({ x: c.x + 38, y: gy });
                }
            });
            // ledge caps and grass tufts on exposed tile corners / tops
            const ledge = LEDGES[biomeId];
            const grass = stage.biome && stage.biome.surface && stage.biome.surface.grass;
            (stage.tiles || []).forEach(function (t) {
                if (t.broken || (t.kind && t.kind !== 'solid')) return;
                if (solid[t.tx + ',' + (t.ty - 1)]) return;
                const left = !solid[(t.tx - 1) + ',' + t.ty];
                const right = !solid[(t.tx + 1) + ',' + t.ty];
                if (ledge && left) d.caps.push({ x: t.tx * T, y: t.ty * T, side: 'left' });
                if (ledge && right) d.caps.push({ x: t.tx * T, y: t.ty * T, side: 'right' });
                if (grass && hash(t.tx * 13 + t.ty, seed) < grass.density) {
                    d.tufts.push({ x: t.tx * T + 6 + hash(t.tx, seed + 5) * (T - 12), y: t.ty * T + 3, n: t.tx * 7 + t.ty, hh: 7 + hash(t.tx, seed + 9) * 9 });
                }
            });
            // pits: water ripples at the bottom of tile-free columns (woodland / glacier)
            if (biomeId === 'woodland' || biomeId === 'glacier') {
                const haz = {};
                (stage.hazards || []).forEach(function (hz) { for (let x = Math.floor(hz.x / T); x <= Math.floor((hz.x + hz.w - 1) / T); x++) haz[x] = true; });
                const maxTx = Math.floor(stage.width / T);
                for (let tx = 1; tx < maxTx; tx++) {
                    if (tops[tx] === undefined && !haz[tx]) d.ripples.push({ x: tx * T, y: stage.height - 44 });
                }
            }
            (stage.hazards || []).forEach(function (hz) { if (isLava(hz)) d.lava.push(hz); });
            // soft shadows below platform decks
            (stage.platforms || []).forEach(function (p) { if (p.solid) d.shadows.push(p); });
            // mid-ground floaters at parallax 0.7
            const pool = FLOATERS[biomeId];
            if (pool) {
                const n = Math.max(3, Math.round(stage.width / 520));
                for (let i = 0; i < n; i++) {
                    d.floaters.push({
                        x: (i + hash(i, seed + 3) * 0.8) * (stage.width / n),
                        y: 70 + hash(i, seed + 4) * Math.max(60, (stage.height || 760) * 0.3),
                        s: 64 + hash(i, seed + 5) * 40,
                        id: pool[i % pool.length],
                        ph: hash(i, seed + 6) * TAU
                    });
                }
            }
            data = d;
            return d;
        }

        function inView(x, view, pad) {
            return x + pad >= view.sx && x - pad <= view.sx + view.viewW;
        }

        function drawBehind(sim, view) {
            const d = prepare(sim.stage);
            const t = view.nowSec;
            const rm = view.reducedMotion;
            ctx.save();
            for (let i = 0; i < d.floaters.length; i++) {
                const f = d.floaters[i];
                const px = f.x + view.sx * 0.3;
                if (px + 120 < view.sx || px - 120 > view.sx + view.viewW) continue;
                const bob = rm ? 0 : Math.sin(t * 0.7 + f.ph) * 4;
                const py = f.y + view.sy * 0.3 + bob;
                h.drawStaticAsset('jump-and-run', f.id, px - f.s / 2, py, f.s, f.s, false, 0.5);
            }
            ctx.restore();
            for (let i = 0; i < d.ripples.length; i++) {
                const r = d.ripples[i];
                if (!inView(r.x, view, d.T)) continue;
                h.drawAnimation('assets-animated', 'water_ripple', t + i * 0.37, r.x, r.y, d.T, 44, false, 0.85);
            }
        }

        function drawTorch(x, gy, n, t) {
            ctx.fillStyle = '#3a2a1e';
            ctx.fillRect(x - 2, gy - 46, 4, 46);
            ctx.fillStyle = '#5b4332';
            ctx.fillRect(x - 5, gy - 48, 10, 4);
            h.drawGlow(x, gy - 62, 54, 'rgba(255, 170, 70, 0.7)', 0.65 + 0.15 * Math.sin(t * 9 + n));
            h.drawAnimation('assets-animated', 'torch', t + n * 0.13, x - 18, gy - 90, 36, 44);
        }

        function drawSurface(sim, view) {
            const d = prepare(sim.stage);
            const t = view.nowSec;
            const rm = view.reducedMotion;
            const T = d.T;
            const ledge = LEDGES[d.biomeId];
            // ledge caps: rounded grass / sand / snow lips on exposed corners
            if (ledge) {
                for (let i = 0; i < d.caps.length; i++) {
                    const c = d.caps[i];
                    if (c.x + T < view.sx || c.x > view.sx + view.viewW) continue;
                    const left = c.side === 'left';
                    const flip = left ? !!ledge.flipLeft : false;
                    h.drawCroppedAsset('jump-and-run', left ? ledge.left : ledge.right, 0, 0, 64, 24,
                        c.x + (left ? -4 : 0), c.y - 2, T + 4, 24 * ((T + 4) / 64), flip, 0.95);
                }
            }
            // sunlit rim on platform tops is drawn by the view; here: soft shadow beneath platform decks
            ctx.save();
            for (let i = 0; i < d.shadows.length; i++) {
                const p = d.shadows[i];
                if (p.x + p.w < view.sx || p.x > view.sx + view.viewW) continue;
                ctx.fillStyle = 'rgba(0, 0, 0, 0.16)';
                ctx.fillRect(p.x + 3, p.y + 30, p.w - 6, 5);
                ctx.fillStyle = 'rgba(0, 0, 0, 0.09)';
                ctx.fillRect(p.x + 6, p.y + 35, p.w - 12, 5);
            }
            ctx.restore();
            // swaying grass tufts
            const grass = sim.stage.biome && sim.stage.biome.surface && sim.stage.biome.surface.grass;
            if (grass) {
                const batches = [[grass.color, []], [grass.color2, []]];
                for (let i = 0; i < d.tufts.length; i++) {
                    const tf = d.tufts[i];
                    if (tf.x + 14 < view.sx || tf.x - 14 > view.sx + view.viewW) continue;
                    const sway = rm ? 0 : Math.sin(t * 1.7 + tf.x * 0.02 + tf.n) * 2.6;
                    batches[tf.n % 2][1].push(tf, sway);
                }
                ctx.save();
                ctx.lineCap = 'round';
                ctx.lineWidth = 2;
                for (let b = 0; b < 2; b++) {
                    const arr = batches[b][1];
                    if (!arr.length) continue;
                    ctx.strokeStyle = batches[b][0];
                    ctx.beginPath();
                    for (let i = 0; i < arr.length; i += 2) {
                        const tf = arr[i];
                        const sw = arr[i + 1];
                        for (let k = -1; k <= 1; k++) {
                            ctx.moveTo(tf.x + k * 3, tf.y);
                            ctx.quadraticCurveTo(tf.x + k * 3 + sw * 0.4, tf.y - tf.hh * 0.6, tf.x + k * 4.5 + sw, tf.y - tf.hh - (k === 0 ? 3 : 0));
                        }
                    }
                    ctx.stroke();
                }
                ctx.restore();
            }
            // torches
            for (let i = 0; i < d.torches.length; i++) {
                const tc = d.torches[i];
                if (inView(tc.x, view, 60)) drawTorch(tc.x, tc.y, tc.n, t);
            }
            // lanterns on posts
            for (let i = 0; i < d.lanterns.length; i++) {
                const l = d.lanterns[i];
                if (!inView(l.x, view, 60)) continue;
                h.drawGlow(l.x + 14, l.y - 34, 70, 'rgba(255, 190, 90, 0.6)', 0.6 + (rm ? 0 : 0.12 * Math.sin(t * 6 + l.n)));
                h.drawStaticAsset('jump-and-run', 'lantern', l.x - 28, l.y - 56, 56, 56);
            }
            // empty open chests beside treasures
            for (let i = 0; i < d.chests.length; i++) {
                const c = d.chests[i];
                if (inView(c.x, view, 40)) h.drawAnimation('assets-animated', 'chest_open', 5, c.x - 20, c.y - 36, 40, 40, false, 0.95, true);
            }
            // lava: shimmer line, upward glow and bubbles
            if (d.lava.length) drawLava(d, view, t, rm);
        }

        function drawLava(d, view, t, rm) {
            const sf = (sim0 && sim0.stage.biome && sim0.stage.biome.surface && sim0.stage.biome.surface.lava) || { color: '#ff8a3d', glow: 'rgba(255, 110, 40, 0.6)' };
            if (!lavaGrad) {
                lavaGrad = ctx.createLinearGradient(0, 0, 0, -64);
                lavaGrad.addColorStop(0, 'rgba(255, 120, 40, 0.5)');
                lavaGrad.addColorStop(1, 'rgba(255, 120, 40, 0)');
            }
            ctx.save();
            for (let i = 0; i < d.lava.length; i++) {
                const hz = d.lava[i];
                if (hz.x + hz.w < view.sx || hz.x > view.sx + view.viewW) continue;
                const top = hz.y + 5;
                ctx.save();
                ctx.translate(hz.x, top);
                ctx.globalCompositeOperation = 'lighter';
                ctx.fillStyle = lavaGrad;
                ctx.fillRect(0, -64, hz.w, 64);
                ctx.restore();
                ctx.globalCompositeOperation = 'lighter';
                ctx.strokeStyle = '#ffd27a';
                ctx.lineWidth = 2;
                ctx.globalAlpha = 0.85;
                ctx.beginPath();
                for (let x = 0; x <= hz.w; x += 6) {
                    const y = top + 1 + (rm ? 0 : Math.sin(t * 3.2 + (hz.x + x) * 0.09) * 1.8);
                    if (x === 0) ctx.moveTo(hz.x + x, y);
                    else ctx.lineTo(hz.x + x, y);
                }
                ctx.stroke();
                ctx.fillStyle = sf.color;
                for (let b = 0; b < 2 && !rm; b++) {
                    const ph = (t * 0.55 + hash(i * 3 + b, d.seed)) % 1;
                    ctx.globalAlpha = (1 - ph) * 0.9;
                    ctx.beginPath();
                    ctx.arc(hz.x + 8 + hash(i * 3 + b, d.seed + 1) * (hz.w - 16), top + 4 - ph * 14, 1.5 + hash(b, i) * 2.2, 0, TAU);
                    ctx.fill();
                }
                ctx.globalAlpha = 1;
                ctx.globalCompositeOperation = 'source-over';
            }
            ctx.restore();
        }

        let sim0 = null;
        function collectLights(sim, view, lights) {
            const d = prepare(sim.stage);
            const t = view.nowSec;
            for (let i = 0; i < d.torches.length; i++) {
                const tc = d.torches[i];
                if (!inView(tc.x, view, 200)) continue;
                lights.push({ x: tc.x, y: tc.y - 62, radius: 120, intensity: 0.62 + 0.06 * Math.sin(t * 8 + tc.n), flicker: true });
            }
            for (let i = 0; i < d.lanterns.length; i++) {
                const l = d.lanterns[i];
                if (inView(l.x, view, 200)) lights.push({ x: l.x + 14, y: l.y - 34, radius: 130, intensity: 0.58, flicker: true });
            }
        }

        function wrapSim(fn) {
            return function (sim, view, extra) {
                sim0 = sim;
                return fn(sim, view, extra);
            };
        }

        return {
            prepare: prepare,
            drawBehind: wrapSim(drawBehind),
            drawSurface: wrapSim(drawSurface),
            collectLights: wrapSim(collectLights)
        };
    }

    root.RunenpfadDecor = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
