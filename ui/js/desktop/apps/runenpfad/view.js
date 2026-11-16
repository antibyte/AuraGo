/* Runenpfad Canvas 2D sprite renderer powered by Game Maker 2D asset packs. */
(function (root) {
    'use strict';

    const PACK_BASE = '/api/game-maker/asset-packs/';

    function prefersReducedMotion() {
        if (typeof document !== 'undefined' && document.body && document.body.dataset && document.body.dataset.animations === 'false') {
            return true;
        }
        return !!(root.matchMedia && root.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function loadImage(url, signal) {
        return new Promise(function (resolve, reject) {
            if (signal && signal.aborted) {
                reject(new DOMException('Aborted', 'AbortError'));
                return;
            }
            const img = new Image();
            let done = false;
            function cleanup() {
                if (signal) signal.removeEventListener('abort', onAbort);
                img.onload = null;
                img.onerror = null;
            }
            function onAbort() {
                if (done) return;
                done = true;
                cleanup();
                reject(new DOMException('Aborted', 'AbortError'));
            }
            img.onload = function () {
                if (done) return;
                done = true;
                cleanup();
                resolve(img);
            };
            img.onerror = function () {
                if (done) return;
                done = true;
                cleanup();
                reject(new Error('image load failed: ' + url));
            };
            if (signal) signal.addEventListener('abort', onAbort);
            img.src = url;
        });
    }

    async function loadSpriteSheets(indexedPacks, signal) {
        const sheets = {};
        const ids = Object.keys(indexedPacks);
        const images = await Promise.all(ids.map(function (id) {
            return loadImage(PACK_BASE + id + '/' + indexedPacks[id].manifest.image, signal);
        }));
        ids.forEach(function (id, index) {
            sheets[id] = {
                image: images[index],
                pack: indexedPacks[id]
            };
        });
        return sheets;
    }

    function createRenderer(canvas, sheets) {
        const ctx = canvas.getContext('2d');
        const Atmosphere = root.RunenpfadAtmosphere;
        const Biomes = root.RunenpfadBiomes;
        const atmosphere = Atmosphere && Atmosphere.createAtmosphere ? Atmosphere.createAtmosphere(ctx) : null;
        let camX = 0;
        let camY = 0;
        let camInitialized = false;
        let shake = 0;
        let lastNowMs = 0;
        let hurtPulse = 0;
        let postfx = null;
        let disposed = false;
        let lastCam = { x: 0, y: 0, w: 960, h: 540, dpr: 1 };
        const SHOCKWAVE_EVENTS = { bomb_blast: 1.0, slam_land: 0.8, boss_phase: 1.2, boss_defeat: 1.4, boss_slam: 1.1, portal_win: 1.0 };

        function triggerShake(amount) {
            if (!prefersReducedMotion()) {
                shake = Math.min(10, Math.max(shake, amount));
            }
        }

        const PLATFORM_CROPS = {
            wood_platform: { sx: 8, sy: 30, sw: 48, sh: 29 },
            rope_platform: { sx: 9, sy: 41, sw: 45, sh: 18 },
            stone_platform: { sx: 8, sy: 32, sw: 48, sh: 27 },
            chain_platform: { sx: 9, sy: 40, sw: 45, sh: 19 },
            earth_platform: { sx: 8, sy: 25, sw: 47, sh: 31 },
            log_platform: { sx: 8, sy: 24, sw: 48, sh: 29 },
            grass_platform: { sx: 10, sy: 19, sw: 43, sh: 38 },
            ice_platform: { sx: 9, sy: 18, sw: 46, sh: 35 },
            red_mushroom_platform: { sx: 8, sy: 24, sw: 48, sh: 35 },
            violet_mushroom_platform: { sx: 10, sy: 22, sw: 44, sh: 30 },
            trapdoor: { sx: 8, sy: 22, sw: 48, sh: 37 },
            magic_platform: { sx: 8, sy: 35, sw: 48, sh: 24 }
        };

        // Fill tiles pick a neighbouring texture so long walls stop looking like wallpaper.
        const TILE_VARIANTS = {
            earth_block: ['earth_block', 'earth_light', 'earth_stones'],
            earth_light: ['earth_light', 'earth_block', 'earth_stones'],
            earth_stones: ['earth_stones', 'earth_block', 'earth_light'],
            earth_dark: ['earth_dark', 'earth_roots', 'earth_block'],
            earth_roots: ['earth_roots', 'earth_dark', 'earth_block'],
            stone_round: ['stone_round', 'stone_moss', 'stone_cracked'],
            stone_cracked: ['stone_cracked', 'stone_dark', 'stone_round'],
            stone_dark: ['stone_dark', 'stone_round', 'stone_moss'],
            stone_moss: ['stone_moss', 'stone_round', 'stone_dark'],
            sand_block: ['sand_block', 'sand_pebbles', 'sand_waves'],
            sand_pebbles: ['sand_pebbles', 'sand_block', 'sand_waves'],
            sand_waves: ['sand_waves', 'sand_block', 'sand_pebbles'],
            sand_bricks: ['sand_bricks', 'sand_block', 'sand_cracked'],
            sand_cracked: ['sand_cracked', 'sand_block', 'sand_bricks'],
            snow_block: ['snow_block', 'ice_block', 'ice_stone'],
            ice_stone: ['ice_stone', 'ice_block', 'snow_block'],
            ice_block: ['ice_block', 'ice_stone', 'snow_block']
        };

        const WORLD_BACKING = ['#3b2313', '#271e38', '#6e4720', '#2c4a63', '#252c3d'];

        function tileVariant(assetId, tx, ty, seed) {
            const list = TILE_VARIANTS[assetId];
            if (!list || list.length < 2) return assetId;
            const h = (tx * 73856093) ^ (ty * 19349663) ^ ((seed || 0) * 83492791);
            return list[Math.abs(h) % list.length];
        }

        function drawCroppedFrame(packId, frameIdx, sx, sy, sw, sh, dx, dy, dw, dh, flipX, alpha) {
            const sheet = sheets[packId];
            if (!sheet) return;
            const frame = sheet.pack.framesByIndex[frameIdx];
            if (!frame) return;
            ctx.save();
            if (alpha !== undefined && alpha < 1) ctx.globalAlpha = Math.max(0, alpha);
            if (flipX) {
                ctx.translate(dx + dw / 2, dy + dh / 2);
                ctx.scale(-1, 1);
                ctx.drawImage(sheet.image, frame.x + sx, frame.y + sy, sw, sh, -dw / 2, -dh / 2, dw, dh);
            } else {
                ctx.drawImage(sheet.image, frame.x + sx, frame.y + sy, sw, sh, dx, dy, dw, dh);
            }
            ctx.restore();
        }

        function drawFrameIndex(packId, frameIdx, dx, dy, dw, dh, flipX, alpha) {
            const sheet = sheets[packId];
            if (!sheet) return;
            const frame = sheet.pack.framesByIndex[frameIdx];
            if (!frame) return;
            drawCroppedFrame(packId, frameIdx, 0, 0, frame.w, frame.h, dx, dy, dw, dh, flipX, alpha);
        }

        function drawCroppedAsset(packId, assetId, sx, sy, sw, sh, dx, dy, dw, dh, flipX, alpha) {
            const sheet = sheets[packId];
            if (!sheet) return;
            const asset = sheet.pack.assetsById[assetId];
            if (!asset || !asset.frames.length) return;
            drawCroppedFrame(packId, asset.frames[0], sx, sy, sw, sh, dx, dy, dw, dh, flipX, alpha);
        }

        function drawCroppedAnimation(packId, animId, elapsedSec, sx, sy, sw, sh, dx, dy, dw, dh, flipX, alpha, holdOnce) {
            const sheet = sheets[packId];
            if (!sheet) return;
            const anim = sheet.pack.animationsById[animId];
            if (!anim || !anim.frames.length) {
                drawCroppedAsset(packId, animId, sx, sy, sw, sh, dx, dy, dw, dh, flipX, alpha);
                return;
            }
            const fps = Number(anim.frame_rate) || 10;
            const total = anim.frames.length;
            let idx = Math.floor(Math.max(0, elapsedSec) * fps);
            if (holdOnce || anim.repeat === 0) idx = Math.min(total - 1, idx);
            else idx = idx % total;
            drawCroppedFrame(packId, anim.frames[idx], sx, sy, sw, sh, dx, dy, dw, dh, flipX, alpha);
        }

        function drawContactShadow(cx, footY, width) {
            ctx.save();
            ctx.fillStyle = 'rgba(8, 12, 20, 0.30)';
            ctx.beginPath();
            ctx.ellipse(cx, footY + 2, width * 0.5, 4.5, 0, 0, Math.PI * 2);
            ctx.fill();
            ctx.restore();
        }

        function drawStaticAsset(packId, assetId, dx, dy, dw, dh, flipX, alpha) {
            const sheet = sheets[packId];
            if (!sheet) return;
            const asset = sheet.pack.assetsById[assetId];
            if (!asset || !asset.frames.length) return;
            drawFrameIndex(packId, asset.frames[0], dx, dy, dw, dh, flipX, alpha);
        }

        function drawAnimation(packId, animId, elapsedSec, dx, dy, dw, dh, flipX, alpha, holdOnce) {
            const sheet = sheets[packId];
            if (!sheet) return;
            const anim = sheet.pack.animationsById[animId];
            if (!anim || !anim.frames.length) {
                drawStaticAsset(packId, animId, dx, dy, dw, dh, flipX, alpha);
                return;
            }
            const fps = Number(anim.frame_rate) || 10;
            const total = anim.frames.length;
            let idx = Math.floor(Math.max(0, elapsedSec) * fps);
            if (holdOnce || anim.repeat === 0) idx = Math.min(total - 1, idx);
            else idx = idx % total;
            drawFrameIndex(packId, anim.frames[idx], dx, dy, dw, dh, flipX, alpha);
        }

        // Soft emissive halo behind light sources and collectibles.
        function drawGlow(cx, cy, radius, color, alpha) {
            if (!radius || radius <= 0) return;
            ctx.save();
            const g = ctx.createRadialGradient(cx, cy, 0, cx, cy, radius);
            g.addColorStop(0, color);
            g.addColorStop(0.5, color.replace(/[\d.]+\)$/, '0.18)'));
            g.addColorStop(1, 'rgba(0,0,0,0)');
            ctx.globalAlpha = alpha == null ? 0.6 : alpha;
            ctx.fillStyle = g;
            ctx.beginPath();
            ctx.arc(cx, cy, radius, 0, Math.PI * 2);
            ctx.fill();
            ctx.restore();
        }

        function drawParticles(sim) {
            const fx = sim.fx;
            if (!fx || !fx.particles.length) return;
            const list = fx.particles;
            ctx.save();
            for (let i = 0; i < list.length; i++) {
                const p = list[i];
                const t = Math.max(0, Math.min(1, p.life / p.maxLife));
                const alpha = t * (p.fade == null ? 1 : p.fade);
                if (alpha <= 0.01) continue;
                ctx.globalAlpha = alpha;
                ctx.globalCompositeOperation = p.additive ? 'lighter' : 'source-over';
                ctx.fillStyle = p.color;
                ctx.strokeStyle = p.color;
                if (p.shape === 'streak') {
                    const len = Math.min(18, Math.hypot(p.vx, p.vy) * 0.05);
                    const nx = p.vx === 0 && p.vy === 0 ? 0 : p.vx / Math.max(0.001, Math.hypot(p.vx, p.vy));
                    const ny = p.vx === 0 && p.vy === 0 ? 0 : p.vy / Math.max(0.001, Math.hypot(p.vx, p.vy));
                    ctx.lineWidth = p.size;
                    ctx.lineCap = 'round';
                    ctx.beginPath();
                    ctx.moveTo(p.x, p.y);
                    ctx.lineTo(p.x - nx * len, p.y - ny * len);
                    ctx.stroke();
                } else if (p.shape === 'square') {
                    ctx.save();
                    ctx.translate(p.x, p.y);
                    ctx.rotate(p.rot);
                    ctx.fillRect(-p.size / 2, -p.size / 2, p.size, p.size);
                    ctx.restore();
                } else if (p.shape === 'ring') {
                    ctx.lineWidth = Math.max(1.5, p.size * 0.18);
                    ctx.beginPath();
                    ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2);
                    ctx.stroke();
                } else if (p.shape === 'rune') {
                    ctx.save();
                    ctx.translate(p.x, p.y);
                    ctx.rotate(p.rot);
                    ctx.beginPath();
                    ctx.moveTo(0, -p.size);
                    ctx.lineTo(p.size, 0);
                    ctx.lineTo(0, p.size);
                    ctx.lineTo(-p.size, 0);
                    ctx.closePath();
                    ctx.fill();
                    ctx.restore();
                } else {
                    ctx.beginPath();
                    ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2);
                    ctx.fill();
                }
            }
            ctx.restore();
        }

        function render(sim, nowMs) {
            if (!sim || !ctx) return;
            const dpr = Math.min(2, root.devicePixelRatio || 1);
            const rect = canvas.getBoundingClientRect();
            const viewW = Math.max(320, Math.floor(rect.width || canvas.width || 960));
            const viewH = Math.max(240, Math.floor(rect.height || canvas.height || 540));
            if (canvas.width !== Math.floor(viewW * dpr) || canvas.height !== Math.floor(viewH * dpr)) {
                canvas.width = Math.floor(viewW * dpr);
                canvas.height = Math.floor(viewH * dpr);
            }

            ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
            const stage = sim.stage;
            const TILE = stage.tileSize;
            const nowSec = (nowMs || 0) * 0.001;
            const dt = lastNowMs ? Math.min(0.1, Math.max(0, (nowMs - lastNowMs) * 0.001)) : 1 / 60;
            lastNowMs = nowMs || 0;
            const reduced = prefersReducedMotion();
            const biome = (Biomes && Biomes.biomeFor(stage.worldIndex)) || { layers: [], light: { ambient: 0 }, weather: null, sky: ['#101018'] };
            const fx = sim.fx;
            const rainWeather = stage.weather === 'rain' && Biomes && Biomes.RAIN_WEATHER ? Biomes.RAIN_WEATHER : null;
            const ambient = Math.min(0.92, (biome.light ? biome.light.ambient : 0) + (stage.darkness || 0));

            // Camera anchors stage bottom flush on tall screens and follows player smoothly
            const maxX = Math.max(0, stage.width - viewW);
            const targetX = Math.max(0, Math.min(maxX, sim.player.x - viewW * 0.38));
            const targetY = viewH >= stage.height
                ? stage.height - viewH
                : Math.max(0, Math.min(stage.height - viewH, sim.player.y - viewH * 0.62));
            if (!camInitialized) {
                camX = targetX;
                camY = targetY;
                camInitialized = true;
            } else {
                camX += (targetX - camX) * (1 - Math.exp(-dt * 10.5));
                camY = viewH >= stage.height ? targetY : camY + (targetY - camY) * (1 - Math.exp(-dt * 9));
            }
            let sx = camX;
            let sy = camY;
            // The renderer only reads sim.fx.shake; the simulation owns its decay.
            const totalShake = reduced ? 0 : Math.max(shake, fx ? fx.shake : 0);
            if (totalShake > 0.1) {
                sx += (Math.sin(nowSec * 61) * totalShake);
                sy += (Math.cos(nowSec * 53) * totalShake * 0.6);
            }
            if (shake > 0.1) shake *= Math.exp(-dt * 10.5);
            else shake = 0;
            hurtPulse = hurtPulse > 0.01 ? hurtPulse * Math.exp(-dt * 4) : 0;
            lastCam = { x: sx, y: sy, w: viewW, h: viewH, dpr: dpr };
            const frameView = { sx: sx, sy: sy, viewW: viewW, viewH: viewH, nowSec: nowSec, dt: dt, biome: biome, reducedMotion: reduced, ambient: ambient };
            let sunScreen = null;

            if (atmosphere) {
                const groundScreenY = Math.min(viewH * 1.15, Math.max(viewH * 0.5, stage.groundY - sy));
                atmosphere.paintBackdrop(biome, sx, viewW, viewH, groundScreenY, nowSec);
                if (typeof atmosphere.sunPosition === 'function') sunScreen = atmosphere.sunPosition(biome, sx, viewW, viewH, groundScreenY) || null;
                atmosphere.paintWeather(biome, viewW, viewH, nowSec, reduced ? 0.35 : 1, groundScreenY, rainWeather);
            } else {
                ctx.fillStyle = biome.sky[0];
                ctx.fillRect(0, 0, viewW, viewH);
            }

            ctx.save();
            ctx.translate(-Math.round(sx), -Math.round(sy));

            callHook(decor, 'drawBehind', sim, frameView);

            // Scenery (nature-assets) grounded directly on terrain
            stage.scenery.forEach(function (prop) {
                if (prop.x + 80 < sx || prop.x - 80 > sx + viewW) return;
                drawStaticAsset('nature-assets', prop.assetId, prop.x - 36, prop.y - 64, 72, 72, false, 0.94);
            });

            // Ladders (cropped horizontally & stacked seamlessly)
            stage.ladders.forEach(function (l) {
                drawCroppedAsset('jump-and-run', 'ladder', 14, 6, 36, 54, l.x + 6, l.y, TILE - 12, TILE + 1);
            });

            // Solid terrain tiles (subterranean fill + cropped interior so zero sky seams appear)
            const backingColor = WORLD_BACKING[stage.worldIndex] || '#3b2313';
            const solidMap = stage.solidMap || {};
            stage.tiles.forEach(function (tile) {
                if (tile.broken) return;
                const tx = tile.tx * TILE;
                const ty = tile.ty * TILE;
                if (tx + TILE < sx || tx > sx + viewW) return;
                const hasAbove = !!solidMap[tile.tx + ',' + (tile.ty - 1)];
                ctx.fillStyle = backingColor;
                ctx.fillRect(tx, ty + (hasAbove ? 0 : 4), TILE + 1, TILE - (hasAbove ? -1 : 3));
                const cropY = hasAbove ? 4 : 2;
                const cropH = hasAbove ? 55 : 57;
                const assetId = hasAbove ? tile.assetId : tileVariant(tile.assetId, tile.tx, tile.ty, stage.worldIndex);
                drawCroppedAsset('jump-and-run', assetId, 4, cropY, 56, cropH, tx, ty, TILE + 1, TILE + 1);
            });

            // Edge definition: sunlit rim on exposed tops, shaded flanks on exposed sides
            stage.tiles.forEach(function (tile) {
                if (tile.broken) return;
                const tx = tile.tx * TILE;
                const ty = tile.ty * TILE;
                if (tx + TILE < sx || tx > sx + viewW) return;
                const hasAbove = !!solidMap[tile.tx + ',' + (tile.ty - 1)];
                if (!hasAbove) {
                    ctx.fillStyle = 'rgba(255, 246, 214, 0.16)';
                    ctx.fillRect(tx, ty, TILE + 1, 4);
                }
                if (!solidMap[(tile.tx - 1) + ',' + tile.ty]) {
                    ctx.fillStyle = 'rgba(6, 8, 16, 0.22)';
                    ctx.fillRect(tx, ty, 4, TILE);
                }
                if (!solidMap[(tile.tx + 1) + ',' + tile.ty]) {
                    ctx.fillStyle = 'rgba(6, 8, 16, 0.22)';
                    ctx.fillRect(tx + TILE - 3, ty, 4, TILE);
                }
            });

            callHook(decor, 'drawSurface', sim, frameView);

            // Static / crumbling / spring platforms (deck top sits flush at p.y, edge-to-edge tiles)
            stage.platforms.forEach(function (p) {
                if (!p.solid) return;
                if (p.x + p.w < sx || p.x > sx + viewW) return;
                const jitter = p.crumbleTimer > 0 && !prefersReducedMotion() ? Math.sin(nowSec * 55) * 2.5 : 0;
                const crop = PLATFORM_CROPS[p.assetId];
                if (crop) {
                    const drawH = Math.round(crop.sh * (TILE / crop.sw));
                    drawCroppedAsset('jump-and-run', p.assetId, crop.sx, crop.sy, crop.sw, crop.sh, p.x + jitter, p.y - 1, TILE + 1, drawH);
                    ctx.fillStyle = 'rgba(0, 0, 0, 0.22)';
                    ctx.fillRect(p.x + jitter, p.y + Math.round(drawH) - 4, TILE + 1, 4);
                } else {
                    drawStaticAsset('jump-and-run', p.assetId, p.x + jitter, p.y - 20, TILE, TILE);
                }
            });

            // Moving magic platforms
            const magicCrop = PLATFORM_CROPS.magic_platform;
            const magicH = Math.round(magicCrop.sh * (TILE / magicCrop.sw));
            stage.movingPlatforms.forEach(function (mp) {
                drawCroppedAnimation('jump-and-run', 'magic_platform', nowSec, magicCrop.sx, magicCrop.sy, magicCrop.sw, magicCrop.sh, mp.x, mp.y - 1, TILE + 1, magicH);
                drawCroppedAnimation('jump-and-run', 'magic_platform', nowSec + 0.25, magicCrop.sx, magicCrop.sy, magicCrop.sw, magicCrop.sh, mp.x + TILE, mp.y - 1, TILE + 1, magicH);
            });

            // Checkpoints (campfire grounded at surface)
            stage.checkpoints.forEach(function (cp) {
                if (cp.activated) {
                    drawGlow(cp.x, cp.y + 4, 108, 'rgba(255, 168, 76, 0.55)', 0.75);
                    drawAnimation('assets-animated', 'campfire', nowSec, cp.x - 28, cp.y + 2, 56, 56);
                } else {
                    drawStaticAsset('assets-animated', 'campfire', cp.x - 28, cp.y + 2, 56, 56, false, 0.55);
                }
            });

            // Locked gate (door_open)
            if (stage.door) {
                if (stage.door.opened) {
                    drawAnimation('assets-animated', 'door_open', 10, stage.door.x - 28, stage.door.y - 8, 56, 62, false, 0.9, true);
                } else {
                    drawStaticAsset('assets-animated', 'door_open', stage.door.x - 28, stage.door.y - 8, 56, 62, false, 1.0);
                }
            }

            // Exit Portal
            if (stage.portal) {
                const unlocked = sim.hasKey;
                if (unlocked) drawGlow(stage.portal.x, stage.portal.y + 14, 132, 'rgba(126, 232, 250, 0.55)', 0.8);
                drawAnimation('assets-animated', 'portal', nowSec, stage.portal.x - 32, stage.portal.y - 10, 64, 64, false, unlocked ? 1.0 : 0.45);
            }

            // Hazards
            stage.hazards.forEach(function (h) {
                if (h.x + h.w < sx || h.x > sx + viewW) return;
                if (h.kind === 'saw') {
                    drawStaticAsset('jump-and-run', 'saw', h.x, h.y + (Math.sin(nowSec * 6) * 2), TILE, TILE);
                } else if (h.kind === 'spike_trap') {
                    drawAnimation('jump-and-run', 'spike_trap', spikeElapsed(h, nowSec), h.x, h.y + 4, TILE, TILE, false, 1.0, typeof h.phase01 === 'number');
                } else if (h.assetId === 'stalactite') {
                    drawStaticAsset('jump-and-run', h.assetId, h.x, h.y - 4, TILE, TILE);
                } else if (h.kind === 'lava' || h.assetId === 'lava' || h.assetId === 'fire') {
                    drawGlow(h.x + TILE / 2, h.y + TILE / 2, 86, 'rgba(251, 113, 133, 0.55)', 0.7);
                    drawStaticAsset('jump-and-run', h.assetId, h.x, h.y + 4, TILE, TILE);
                } else {
                    drawStaticAsset('jump-and-run', h.assetId, h.x, h.y + 4, TILE, TILE);
                }
            });

            // Falling stones
            (stage.boulders || []).forEach(function (b) {
                if (b.x + b.w < sx || b.x > sx + viewW) return;
                if (b.state === 'falling') {
                    drawGlow(b.x + b.w / 2, b.y + b.h / 2, 70, 'rgba(207, 195, 174, 0.5)', 0.55);
                }
                drawStaticAsset('jump-and-run', 'stone_weight', b.x, b.y, b.w, b.h);
            });

            // Rune shrines
            (stage.shrines || []).forEach(function (sh) {
                if (sh.x + 60 < sx || sh.x - 60 > sx + viewW) return;
                if (sh.taken) {
                    drawStaticAsset('nature-assets', 'blue_crystals', sh.x - 20, sh.y - 24, 40, 40, false, 0.4);
                    return;
                }
                drawGlow(sh.x, sh.y - 4, 118, 'rgba(192, 132, 252, 0.6)', 0.8);
                drawStaticAsset('nature-assets', 'blue_crystals', sh.x - 24, sh.y - 30, 48, 48);
            });

            // Cannons
            stage.cannons.forEach(function (c) {
                drawStaticAsset('jump-and-run', 'cannon', c.x - 24, c.y - 20, 48, 48, c.dir > 0);
            });

            // Breakable crates & barrels
            stage.crates.forEach(function (c) {
                if (!c.broken) {
                    drawStaticAsset('jump-and-run', c.assetId, c.x - 22, c.y + TILE - 40, 44, 44);
                }
            });

            // Collectibles
            const bob = prefersReducedMotion() ? 0 : Math.sin(nowSec * 4) * 3.5;
            stage.collectibles.forEach(function (item) {
                if (item.collected) return;
                if (item.x + 32 < sx || item.x - 32 > sx + viewW) return;
                if (item.kind === 'gem') {
                    drawGlow(item.x, item.y, 78, 'rgba(126, 232, 250, 0.6)', 0.7);
                } else if (item.kind === 'star') {
                    drawGlow(item.x, item.y, 70, 'rgba(255, 224, 102, 0.6)', 0.65);
                }
                if (item.animated) {
                    drawAnimation('assets-animated', item.animId, nowSec, item.x - 16, item.y - 16 + bob * 0.5, 32, 32);
                } else {
                    drawStaticAsset('jump-and-run', item.assetId, item.x - 18, item.y - 18 + bob, 36, 36);
                }
            });

            // Runic Key
            if (stage.key && !stage.key.collected) {
                const alpha = stage.key.guardedByBoss ? 0.35 : 1.0;
                if (!stage.key.guardedByBoss) drawGlow(stage.key.x, stage.key.y, 96, 'rgba(255, 224, 102, 0.6)', 0.7);
                drawStaticAsset('jump-and-run', 'key', stage.key.x - 20, stage.key.y - 20 + bob, 40, 40, false, alpha);
            }

            callHook(stageArt, 'drawWorld', sim, frameView);
            callHook(actors, 'drawEnemies', sim, frameView);
            callHook(actors, 'drawHero', sim, frameView);
            callHook(actors, 'drawProjectiles', sim, frameView);

            // Transient animated explosions/smoke
            sim.effects.forEach(function (fx2) {
                drawAnimation('assets-animated', fx2.animId, 0.5 - fx2.timer, fx2.x - 28, fx2.y - 28, 56, 56, false, 1.0, true);
            });

            // Particle layer (sparks, dust, shards, rings)
            drawParticles(sim);

            callHook(stageArt, 'drawOverlay', sim, frameView);

            // Floating feedback numbers
            ctx.save();
            ctx.font = 'bold 14px "Segoe UI", sans-serif';
            ctx.textAlign = 'center';
            sim.floatingTexts.forEach(function (ft) {
                ctx.globalAlpha = Math.min(1, ft.timer * 1.6);
                ctx.fillStyle = '#09111a';
                ctx.fillText(ft.text, ft.x + 1, ft.y + 1);
                ctx.fillStyle = ft.color || '#ffe066';
                ctx.fillText(ft.text, ft.x, ft.y);
            });
            ctx.restore();

            ctx.restore();

            if (atmosphere) {
                const fgGround = Math.min(viewH * 1.15, Math.max(viewH * 0.5, stage.groundY - sy));
                if (typeof atmosphere.paintForeground === 'function') atmosphere.paintForeground(biome, sx, viewW, viewH, fgGround);
                if (rainWeather) atmosphere.paintRainFront(rainWeather, viewW, viewH, nowSec, fgGround, reduced ? 0.35 : 1);
            }

            // Dynamic light map above the world plane
            if (atmosphere && ambient > 0.02) {
                const lights = collectLights(sim, stage, biome, nowSec, ambient);
                callHook(decor, 'collectLights', sim, frameView, lights);
                callHook(stageArt, 'collectLights', sim, frameView, lights);
                atmosphere.paintLighting(biome, sx, sy, viewW, viewH, lights, nowSec, ambient);
            }
            const gpu = !!(postfx && postfx.isEnabled());
            if (atmosphere && !gpu) {
                atmosphere.paintGrade(biome, viewW, viewH);
            }

            // Screen flash (damage, phase change, rune surge)
            if (fx && fx.flashAlpha > 0) {
                const fade = fx.flashMax > 0 ? Math.max(0, Math.min(1, fx.flashTimer / fx.flashMax)) : 1;
                ctx.save();
                ctx.globalAlpha = Math.max(0, Math.min(1, fx.flashAlpha * fade));
                ctx.fillStyle = fx.flashColor || '#ffffff';
                ctx.fillRect(0, 0, viewW, viewH);
                ctx.restore();
            }

            if (gpu) {
                const pl = sim.player;
                postfx.render({
                    t: nowSec,
                    dt: dt,
                    biomeId: biome.id || 'woodland',
                    sun: sunScreen,
                    hurt: hurtPulse,
                    lowHp: pl && pl.maxHp > 0 && pl.hp > 0 && pl.hp / pl.maxHp <= 0.34 ? 1 : 0,
                    heat: biome.id === 'desert' ? 1 : biome.id === 'citadel' ? 0.6 : 0
                });
            }
        }

        // Spike frames: retracted, warning, rising, full extension, retracting (10-frame strip).
        function spikeElapsed(h, nowSec) {
            if (typeof h.phase01 !== 'number') return nowSec;
            const ph = h.phase01;
            let idx = 0;
            if (h.active) {
                const q = (ph - 0.55) / 0.38;
                idx = q < 0.15 ? 2 : q < 0.3 ? 4 : q < 0.8 ? 5 : q < 0.9 ? 3 : 1;
            } else if (ph >= 0.45 && ph < 0.55) {
                idx = 1;
            }
            return (idx + 0.5) / 12;
        }

        function hasAnim(packId, animId) {
            const sheet = sheets[packId];
            return !!(sheet && sheet.pack.animationsById && sheet.pack.animationsById[animId]);
        }

        // Resolves an animation (or static asset) to a source rectangle for custom drawing (flash, dissolve).
        function getFrame(packId, animId, elapsedSec, holdOnce) {
            const sheet = sheets[packId];
            if (!sheet) return null;
            const anim = sheet.pack.animationsById[animId];
            let idx;
            if (anim && anim.frames.length) {
                const total = anim.frames.length;
                const i = Math.floor(Math.max(0, elapsedSec) * (Number(anim.frame_rate) || 10));
                idx = anim.frames[holdOnce || anim.repeat === 0 ? Math.min(total - 1, i) : i % total];
            } else {
                const asset = sheet.pack.assetsById[animId];
                if (!asset || !asset.frames.length) return null;
                idx = asset.frames[0];
            }
            const fr = sheet.pack.framesByIndex[idx];
            return fr ? { image: sheet.image, sx: fr.x, sy: fr.y, sw: fr.w, sh: fr.h } : null;
        }

        function callHook(mod, name, a, b, c) {
            if (mod && typeof mod[name] === 'function') mod[name](a, b, c);
        }

        function worldToScreen(x, y) {
            return { x: (x - lastCam.x) , y: (y - lastCam.y) };
        }

        function handleEvents(events, sim) {
            if (disposed || !events || !events.length) return;
            const pl = sim && sim.player;
            for (let i = 0; i < events.length; i++) {
                const ev = events[i];
                const d = ev.detail || {};
                const wx = typeof d.x === 'number' ? d.x : (pl ? pl.x : 0);
                const wy = typeof d.y === 'number' ? d.y : (pl ? pl.y : 0);
                if (ev.type === 'player_hurt') hurtPulse = 1;
                if (ev.type === 'player_hurt' || ev.type === 'boss_defeat') triggerShake(7);
                if (SHOCKWAVE_EVENTS[ev.type] && postfx) {
                    const sp = worldToScreen(wx, wy);
                    postfx.pushShockwave(sp.x, sp.y, SHOCKWAVE_EVENTS[ev.type]);
                }
                callHook(actors, 'onEvent', ev, sim, lastCam);
            }
        }

        function attachPostfx(p) {
            if (disposed || !p) {
                if (p && typeof p.dispose === 'function') p.dispose();
                return;
            }
            postfx = p;
            if (typeof p.resize === 'function') p.resize(Math.round(lastCam.w * lastCam.dpr), Math.round(lastCam.h * lastCam.dpr));
        }

        function getCamera() {
            return { x: lastCam.x, y: lastCam.y, w: lastCam.w, h: lastCam.h, dpr: lastCam.dpr };
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            // The app owns the post-processing object and disposes it itself.
            [decor, actors, stageArt].forEach(function (m) {
                if (m && typeof m.dispose === 'function') {
                    try { m.dispose(); } catch (err) { /* ignore */ }
                }
            });
            if (atmosphere && typeof atmosphere.dispose === 'function') atmosphere.dispose();
            postfx = null;
        }

        const helper = {
            ctx: ctx,
            sheets: sheets,
            drawAnimation: drawAnimation,
            drawStaticAsset: drawStaticAsset,
            drawCroppedAsset: drawCroppedAsset,
            drawCroppedAnimation: drawCroppedAnimation,
            drawGlow: drawGlow,
            drawContactShadow: drawContactShadow,
            getFrame: getFrame,
            hasAnim: hasAnim,
            get reducedMotion() { return prefersReducedMotion(); }
        };
        function makeHook(mod) {
            return mod && typeof mod.create === 'function' ? mod.create(helper) : null;
        }
        const decor = makeHook(root.RunenpfadDecor);
        const actors = makeHook(root.RunenpfadActors);
        const stageArt = makeHook(root.RunenpfadStageArt);

        // Light sources in world space for the 2D light map.
        function collectLights(sim, stage, biome, nowSec, ambient) {
            const lights = [];
            const p = sim.player;
            if (biome.light && biome.light.player > 0) {
                lights.push({
                    x: p.x,
                    y: p.y - 22,
                    radius: biome.light.radius,
                    intensity: biome.light.player * 0.92,
                    flicker: false
                });
            }
            if (sim.blessing && sim.blessing.runePower > 0) {
                lights.push({
                    x: p.x,
                    y: p.y - 24,
                    radius: biome.light ? biome.light.radius * 0.8 : 160,
                    intensity: 0.7,
                    flicker: false
                });
            }
            stage.checkpoints.forEach(function (cp) {
                if (cp.activated) {
                    lights.push({ x: cp.x, y: cp.y + 6, radius: 132, intensity: 0.78, flicker: true });
                }
            });
            const half = (stage.tileSize || 48) / 2;
            stage.hazards.forEach(function (h) {
                if (h.kind === 'lava' || h.assetId === 'lava' || h.assetId === 'fire') {
                    lights.push({ x: h.x + half, y: h.y + half, radius: 104, intensity: 0.6, flicker: true });
                }
            });
            stage.collectibles.forEach(function (item) {
                if (item.collected) return;
                if (item.kind === 'gem') lights.push({ x: item.x, y: item.y, radius: 86, intensity: 0.42, flicker: false });
            });
            if (stage.key && !stage.key.collected && !stage.key.guardedByBoss) {
                lights.push({ x: stage.key.x, y: stage.key.y, radius: 112, intensity: 0.6, flicker: true });
            }
            if (stage.portal && sim.hasKey) {
                lights.push({ x: stage.portal.x, y: stage.portal.y + 14, radius: 150, intensity: 0.7, flicker: true });
            }
            stage.enemies.forEach(function (e) {
                if (e.dead) return;
                if (e.def.boss) {
                    lights.push({ x: e.x, y: e.y - 26, radius: 150, intensity: e.phase === 2 ? 0.55 : 0.42, flicker: e.phase === 2 });
                }
            });
            sim.projectiles.forEach(function (proj) {
                if (!proj.dead) lights.push({ x: proj.x, y: proj.y, radius: 62, intensity: 0.4, flicker: false });
            });
            return lights;
        }

        return {
            render: render,
            triggerShake: triggerShake,
            handleEvents: handleEvents,
            attachPostfx: attachPostfx,
            getCamera: getCamera,
            dispose: dispose
        };
    }

    root.RunenpfadView = {
        loadSpriteSheets: loadSpriteSheets,
        createRenderer: createRenderer
    };
})(typeof window !== 'undefined' ? window : globalThis);
