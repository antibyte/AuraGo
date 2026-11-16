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
            const biome = (Biomes && Biomes.biomeFor(stage.worldIndex)) || { layers: [], light: { ambient: 0 }, weather: null, sky: ['#101018'] };
            const fx = sim.fx;
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
                camX += (targetX - camX) * 0.16;
                camY = viewH >= stage.height ? targetY : camY + (targetY - camY) * 0.14;
            }
            let sx = camX;
            let sy = camY;
            const fxShake = fx ? fx.shake : 0;
            const totalShake = Math.max(shake, fxShake);
            if (totalShake > 0.1) {
                sx += (Math.sin(nowSec * 61) * totalShake);
                sy += (Math.cos(nowSec * 53) * totalShake * 0.6);
                if (shake > 0.1) shake *= 0.84;
                if (fx) fx.shake = Math.max(0, fx.shake - 0.02);
            } else {
                shake = 0;
            }

            if (atmosphere) {
                const groundScreenY = Math.min(viewH * 1.15, Math.max(viewH * 0.5, stage.groundY - sy));
                atmosphere.paintBackdrop(biome, sx, viewW, viewH, groundScreenY);
                atmosphere.paintWeather(biome, viewW, viewH, nowSec, prefersReducedMotion() ? 0.35 : 1);
            } else {
                ctx.fillStyle = biome.sky[0];
                ctx.fillRect(0, 0, viewW, viewH);
            }

            ctx.save();
            ctx.translate(-Math.round(sx), -Math.round(sy));

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
                    drawAnimation('jump-and-run', 'spike_trap', nowSec, h.x, h.y + 4, TILE, TILE, false, 1.0, false);
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

            // Enemies & Bosses
            stage.enemies.forEach(function (e) {
                if (e.dead && e.deathTimer <= 0) return;
                if (e.x + 64 < sx || e.x - 64 > sx + viewW) return;
                const size = Math.round(58 * e.def.scale);
                const drawTop = e.y - Math.round(size * (57 / 64));
                const animName = e.def.prefix + (e.dead ? 'death' : e.hurtTimer > 0 ? 'hurt' : e.animState === 'idle' ? 'idle' : 'move');
                if (!e.dead && !e.def.flying && e.onGround) {
                    drawContactShadow(e.x, e.y, size * 0.56);
                }
                if (e.def.boss && !e.dead) {
                    ctx.save();
                    ctx.strokeStyle = e.phase === 2 ? '#fb7185' : '#f7c948';
                    ctx.lineWidth = 2.5;
                    ctx.beginPath();
                    ctx.arc(e.x, e.y - size * 0.42, size * 0.46, 0, Math.PI * 2);
                    ctx.stroke();
                    ctx.restore();
                    drawGlow(e.x, e.y - size * 0.42, size * 0.9, e.phase === 2 ? 'rgba(251, 113, 133, 0.45)' : 'rgba(247, 201, 72, 0.4)', 0.6);
                }
                drawAnimation(e.def.pack, animName, e.animTime, e.x - size / 2, drawTop, size, size, e.facing < 0, e.hurtTimer > 0 ? 0.75 : 1.0, e.dead);
                if (e.hurtTimer > 0 && !e.dead) {
                    ctx.save();
                    ctx.globalCompositeOperation = 'lighter';
                    ctx.globalAlpha = 0.35;
                    drawAnimation(e.def.pack, animName, e.animTime, e.x - size / 2, drawTop, size, size, e.facing < 0, 1.0, false);
                    ctx.restore();
                }
                if (!e.dead && !e.def.boss && e.hp < e.maxHp) {
                    ctx.fillStyle = 'rgba(0, 0, 0, 0.55)';
                    ctx.fillRect(e.x - 18, drawTop - 4, 36, 5);
                    ctx.fillStyle = '#fb7185';
                    ctx.fillRect(e.x - 18, drawTop - 4, 36 * (e.hp / e.maxHp), 5);
                }
            });

            // Hero sprite (feet pixel y=59 inside 64x64 frame aligns squarely on p.y + 2)
            const p = sim.player;
            if (p.onGround) {
                drawContactShadow(p.x, p.y, 28);
            }
            if (sim.blessing && sim.blessing.runePower > 0) {
                drawGlow(p.x, p.y - 24, 62, 'rgba(192, 132, 252, 0.7)', 0.8);
            }
            const blinkAlpha = p.invulnTimer > 0 && Math.floor(nowSec * 16) % 2 === 0 ? 0.45 : 1.0;
            const heroAnim = sim.stats.def.prefix + p.animState;
            drawAnimation(sim.stats.def.pack, heroAnim, p.animTime, p.x - 32, p.y - 57, 64, 64, p.facing < 0, blinkAlpha, p.animState === 'death');

            // Knight melee cleave crescent visual
            if (sim.heroId === 'knight' && p.attackTimer > 0) {
                const progress = 1 - p.attackTimer / Math.max(0.01, sim.stats.def.attackDuration);
                ctx.save();
                ctx.globalCompositeOperation = 'lighter';
                ctx.lineCap = 'round';
                ctx.lineWidth = 9 - progress * 4;
                ctx.strokeStyle = 'rgba(126, 232, 250, ' + (0.85 * (1 - progress * 0.7)).toFixed(3) + ')';
                ctx.beginPath();
                if (p.facing > 0) {
                    ctx.arc(p.x + 18, p.y - 24, 28, -Math.PI * 0.45, Math.PI * 0.45);
                } else {
                    ctx.arc(p.x - 18, p.y - 24, 28, Math.PI * 0.55, Math.PI * 1.45);
                }
                ctx.stroke();
                ctx.lineWidth = 2.5;
                ctx.strokeStyle = 'rgba(255, 255, 255, ' + (0.8 * (1 - progress)).toFixed(3) + ')';
                ctx.stroke();
                ctx.restore();
            }

            // Ranger rune bolt glow
            if (sim.heroId === 'ranger' && p.attackTimer > 0) {
                drawGlow(p.x + p.facing * 20, p.y - 24, 60, 'rgba(126, 232, 250, 0.5)', 0.7);
            }

            // Projectiles with motion trail
            sim.projectiles.forEach(function (proj) {
                if (proj.dead) return;
                const color = proj.color || '#7ee8fa';
                ctx.save();
                ctx.globalCompositeOperation = 'lighter';
                const len = Math.hypot(proj.vx, proj.vy) * 0.028;
                const nx = proj.vx === 0 && proj.vy === 0 ? 0 : proj.vx / Math.max(0.001, Math.hypot(proj.vx, proj.vy));
                const ny = proj.vx === 0 && proj.vy === 0 ? 0 : proj.vy / Math.max(0.001, Math.hypot(proj.vx, proj.vy));
                const grad = ctx.createLinearGradient(proj.x, proj.y, proj.x - nx * len * 2, proj.y - ny * len * 2);
                grad.addColorStop(0, color);
                grad.addColorStop(1, 'rgba(0,0,0,0)');
                ctx.strokeStyle = grad;
                ctx.lineWidth = proj.fromEnemy ? 7 : 5;
                ctx.lineCap = 'round';
                ctx.beginPath();
                ctx.moveTo(proj.x, proj.y);
                ctx.lineTo(proj.x - nx * len * 2, proj.y - ny * len * 2);
                ctx.stroke();
                ctx.restore();
                ctx.save();
                ctx.fillStyle = color;
                ctx.beginPath();
                ctx.arc(proj.x, proj.y, proj.fromEnemy ? 6 : 5, 0, Math.PI * 2);
                ctx.fill();
                ctx.fillStyle = 'rgba(255, 255, 255, 0.85)';
                ctx.beginPath();
                ctx.arc(proj.x - 1, proj.y - 1, proj.fromEnemy ? 2.4 : 2, 0, Math.PI * 2);
                ctx.fill();
                ctx.restore();
            });

            // Transient animated explosions/smoke
            sim.effects.forEach(function (fx2) {
                drawAnimation('assets-animated', fx2.animId, 0.5 - fx2.timer, fx2.x - 28, fx2.y - 28, 56, 56, false, 1.0, true);
            });

            // Particle layer (sparks, dust, shards, rings)
            drawParticles(sim);

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

            // Dynamic light map above the world plane
            if (atmosphere && ambient > 0.02) {
                atmosphere.paintLighting(biome, sx, sy, viewW, viewH, collectLights(sim, stage, biome, nowSec, ambient), nowSec);
            }
            if (atmosphere) {
                atmosphere.paintGrade(biome, viewW, viewH);
            }

            // Screen flash (damage, phase change, rune surge)
            if (fx && fx.flashAlpha > 0) {
                ctx.save();
                ctx.globalAlpha = Math.max(0, Math.min(1, fx.flashAlpha));
                ctx.fillStyle = fx.flashColor || '#ffffff';
                ctx.fillRect(0, 0, viewW, viewH);
                ctx.restore();
            }
        }

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
            triggerShake: triggerShake
        };
    }

    root.RunenpfadView = {
        loadSpriteSheets: loadSpriteSheets,
        createRenderer: createRenderer
    };
})(typeof window !== 'undefined' ? window : globalThis);
