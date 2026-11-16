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
        let camX = 0;
        let camY = 0;
        let shake = 0;
        const particles = [];
        for (let i = 0; i < 42; i++) {
            particles.push({
                x: (i * 197) % 1400,
                y: (i * 131) % 800,
                speed: 22 + (i % 5) * 11,
                size: 2 + (i % 3) * 1.5,
                phase: i * 0.7
            });
        }

        function triggerShake(amount) {
            if (!prefersReducedMotion()) {
                shake = Math.min(10, Math.max(shake, amount));
            }
        }

        function drawFrameIndex(packId, frameIdx, dx, dy, dw, dh, flipX, alpha) {
            const sheet = sheets[packId];
            if (!sheet) return;
            const frame = sheet.pack.framesByIndex[frameIdx];
            if (!frame) return;
            ctx.save();
            if (alpha !== undefined && alpha < 1) ctx.globalAlpha = Math.max(0, alpha);
            if (flipX) {
                ctx.translate(dx + dw / 2, dy + dh / 2);
                ctx.scale(-1, 1);
                ctx.drawImage(sheet.image, frame.x, frame.y, frame.w, frame.h, -dw / 2, -dh / 2, dw, dh);
            } else {
                ctx.drawImage(sheet.image, frame.x, frame.y, frame.w, frame.h, dx, dy, dw, dh);
            }
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

        function renderBackdrop(sim, viewW, viewH, nowSec) {
            const w = sim.stage.world;
            const sky = ctx.createLinearGradient(0, 0, 0, viewH);
            sky.addColorStop(0, w.skyTop);
            sky.addColorStop(1, w.skyBottom);
            ctx.fillStyle = sky;
            ctx.fillRect(0, 0, viewW, viewH);

            // Celestial orb
            ctx.save();
            ctx.globalAlpha = 0.24;
            ctx.fillStyle = '#fff5cc';
            ctx.beginPath();
            ctx.arc(viewW * 0.82 - camX * 0.04, viewH * 0.22, 46, 0, Math.PI * 2);
            ctx.fill();
            ctx.restore();

            // Far & near parallax hills
            [
                { color: w.hillBack, factor: 0.16, base: viewH * 0.62, amp: 58, freq: 0.0045 },
                { color: w.hillFront, factor: 0.34, base: viewH * 0.74, amp: 42, freq: 0.0072 }
            ].forEach(function (layer) {
                ctx.fillStyle = layer.color;
                ctx.beginPath();
                ctx.moveTo(0, viewH);
                for (let x = 0; x <= viewW + 32; x += 32) {
                    const wx = x + camX * layer.factor;
                    const y = layer.base + Math.sin(wx * layer.freq) * layer.amp + Math.cos(wx * layer.freq * 0.5) * 18;
                    ctx.lineTo(x, y);
                }
                ctx.lineTo(viewW, viewH);
                ctx.closePath();
                ctx.fill();
            });

            // Biome weather particles
            const reduced = prefersReducedMotion();
            ctx.save();
            ctx.fillStyle = w.weather === 'snow' ? '#e8f7ff'
                : w.weather === 'sand' ? '#f5cf87'
                : w.weather === 'ember' ? '#fb7185'
                : w.weather === 'spore' ? '#a7f3d0'
                : '#bef264';
            particles.forEach(function (pt) {
                const driftX = reduced ? 0 : nowSec * (w.weather === 'sand' ? 95 : 22);
                const driftY = reduced ? 0 : nowSec * (w.weather === 'ember' ? -pt.speed : pt.speed);
                const sx = ((pt.x + driftX) % (viewW + 40) + viewW + 40) % (viewW + 40) - 20;
                const sy = ((pt.y + driftY) % (viewH + 40) + viewH + 40) % (viewH + 40) - 20;
                ctx.globalAlpha = 0.52;
                ctx.beginPath();
                ctx.arc(sx, sy, pt.size, 0, Math.PI * 2);
                ctx.fill();
            });
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

            // Camera target follows player smoothly
            const targetX = Math.max(0, Math.min(stage.width - viewW, sim.player.x - viewW * 0.38));
            const targetY = Math.max(0, Math.min(stage.height - viewH, sim.player.y - viewH * 0.62));
            camX += (targetX - camX) * 0.16;
            camY += (targetY - camY) * 0.14;
            let sx = camX;
            let sy = camY;
            if (shake > 0.1) {
                sx += (Math.sin(nowSec * 61) * shake);
                sy += (Math.cos(nowSec * 53) * shake * 0.6);
                shake *= 0.84;
            } else {
                shake = 0;
            }

            renderBackdrop(sim, viewW, viewH, nowSec);

            ctx.save();
            ctx.translate(-Math.round(sx), -Math.round(sy));

            // Scenery (nature-assets) anchored at feet/ground
            stage.scenery.forEach(function (prop) {
                if (prop.x + 80 < sx || prop.x - 80 > sx + viewW) return;
                drawStaticAsset('nature-assets', prop.assetId, prop.x - 36, prop.y - 72, 72, 72, false, 0.92);
            });

            // Ladders
            stage.ladders.forEach(function (l) {
                drawStaticAsset('jump-and-run', 'ladder', l.x, l.y, TILE, TILE);
            });

            // Solid terrain tiles
            stage.tiles.forEach(function (tile) {
                const tx = tile.tx * TILE;
                const ty = tile.ty * TILE;
                if (tx + TILE < sx || tx > sx + viewW) return;
                drawStaticAsset('jump-and-run', tile.assetId, tx, ty, TILE, TILE);
            });

            // Static / crumbling / spring platforms
            stage.platforms.forEach(function (p) {
                if (!p.solid) return;
                if (p.x + p.w < sx || p.x > sx + viewW) return;
                const jitter = p.crumbleTimer > 0 && !prefersReducedMotion() ? Math.sin(nowSec * 55) * 2.5 : 0;
                drawStaticAsset('jump-and-run', p.assetId, p.x + jitter, p.y - 6, TILE, TILE);
            });

            // Moving magic platforms
            stage.movingPlatforms.forEach(function (mp) {
                drawAnimation('jump-and-run', 'magic_platform', nowSec, mp.x, mp.y - 10, TILE, TILE);
                drawAnimation('jump-and-run', 'magic_platform', nowSec + 0.25, mp.x + TILE, mp.y - 10, TILE, TILE);
            });

            // Checkpoints (campfire)
            stage.checkpoints.forEach(function (cp) {
                if (cp.activated) {
                    drawAnimation('assets-animated', 'campfire', nowSec, cp.x - 28, cp.y - 4, 56, 56);
                } else {
                    drawStaticAsset('assets-animated', 'campfire', cp.x - 28, cp.y - 4, 56, 56, false, 0.55);
                }
            });

            // Locked gate (door_open)
            if (stage.door) {
                if (stage.door.opened) {
                    drawAnimation('assets-animated', 'door_open', 10, stage.door.x - 28, stage.door.y - 14, 56, 62, false, 0.9, true);
                } else {
                    drawStaticAsset('assets-animated', 'door_open', stage.door.x - 28, stage.door.y - 14, 56, 62, false, 1.0);
                }
            }

            // Exit Portal
            if (stage.portal) {
                const unlocked = sim.hasKey;
                drawAnimation('assets-animated', 'portal', nowSec, stage.portal.x - 32, stage.portal.y - 16, 64, 64, false, unlocked ? 1.0 : 0.45);
            }

            // Hazards
            stage.hazards.forEach(function (h) {
                if (h.x + h.w < sx || h.x > sx + viewW) return;
                if (h.kind === 'saw') {
                    drawStaticAsset('jump-and-run', 'saw', h.x, h.y + (Math.sin(nowSec * 6) * 2), TILE, TILE);
                } else {
                    drawStaticAsset('jump-and-run', h.assetId, h.x, h.y, TILE, TILE);
                }
            });

            // Cannons
            stage.cannons.forEach(function (c) {
                drawStaticAsset('jump-and-run', 'cannon', c.x - 24, c.y - 24, 48, 48, c.dir > 0);
            });

            // Breakable crates & barrels
            stage.crates.forEach(function (c) {
                if (!c.broken) {
                    drawStaticAsset('jump-and-run', c.assetId, c.x - 22, c.y + 4, 44, 44);
                }
            });

            // Collectibles
            const bob = prefersReducedMotion() ? 0 : Math.sin(nowSec * 4) * 3.5;
            stage.collectibles.forEach(function (item) {
                if (item.collected) return;
                if (item.x + 32 < sx || item.x - 32 > sx + viewW) return;
                if (item.animated) {
                    drawAnimation('assets-animated', item.animId, nowSec, item.x - 16, item.y - 16 + bob * 0.5, 32, 32);
                } else {
                    drawStaticAsset('jump-and-run', item.assetId, item.x - 18, item.y - 18 + bob, 36, 36);
                }
            });

            // Runic Key
            if (stage.key && !stage.key.collected) {
                const alpha = stage.key.guardedByBoss ? 0.35 : 1.0;
                drawStaticAsset('jump-and-run', 'key', stage.key.x - 20, stage.key.y - 20 + bob, 40, 40, false, alpha);
            }

            // Enemies & Bosses
            stage.enemies.forEach(function (e) {
                if (e.dead && e.deathTimer <= 0) return;
                if (e.x + 64 < sx || e.x - 64 > sx + viewW) return;
                const size = Math.round(58 * e.def.scale);
                const animName = e.def.prefix + (e.dead ? 'death' : e.hurtTimer > 0 ? 'hurt' : e.animState === 'idle' ? 'idle' : 'move');
                if (e.def.boss && !e.dead) {
                    ctx.save();
                    ctx.strokeStyle = e.phase === 2 ? '#fb7185' : '#f7c948';
                    ctx.lineWidth = 2.5;
                    ctx.beginPath();
                    ctx.arc(e.x, e.y - size * 0.42, size * 0.46, 0, Math.PI * 2);
                    ctx.stroke();
                    ctx.restore();
                }
                drawAnimation(e.def.pack, animName, e.animTime, e.x - size / 2, e.y - size + 4, size, size, e.facing < 0, e.hurtTimer > 0 ? 0.65 : 1.0, e.dead);
                if (!e.dead && !e.def.boss && e.hp < e.maxHp) {
                    ctx.fillStyle = 'rgba(0, 0, 0, 0.55)';
                    ctx.fillRect(e.x - 18, e.y - size - 6, 36, 5);
                    ctx.fillStyle = '#fb7185';
                    ctx.fillRect(e.x - 18, e.y - size - 6, 36 * (e.hp / e.maxHp), 5);
                }
            });

            // Hero sprite
            const p = sim.player;
            const blinkAlpha = p.invulnTimer > 0 && Math.floor(nowSec * 16) % 2 === 0 ? 0.45 : 1.0;
            const heroAnim = sim.stats.def.prefix + p.animState;
            drawAnimation(sim.stats.def.pack, heroAnim, p.animTime, p.x - 32, p.y - 60, 64, 64, p.facing < 0, blinkAlpha, p.animState === 'death');

            // Knight melee cleave crescent visual
            if (sim.heroId === 'knight' && p.attackTimer > 0) {
                ctx.save();
                ctx.strokeStyle = '#7ee8fa';
                ctx.lineWidth = 4;
                ctx.beginPath();
                if (p.facing > 0) {
                    ctx.arc(p.x + 18, p.y - 24, 28, -Math.PI * 0.45, Math.PI * 0.45);
                } else {
                    ctx.arc(p.x - 18, p.y - 24, 28, Math.PI * 0.55, Math.PI * 1.45);
                }
                ctx.stroke();
                ctx.restore();
            }

            // Projectiles
            sim.projectiles.forEach(function (proj) {
                if (proj.dead) return;
                ctx.save();
                ctx.fillStyle = proj.color || '#7ee8fa';
                ctx.beginPath();
                ctx.arc(proj.x, proj.y, proj.fromEnemy ? 6 : 5, 0, Math.PI * 2);
                ctx.fill();
                ctx.restore();
            });

            // Transient animated explosions/smoke
            sim.effects.forEach(function (fx) {
                drawAnimation('assets-animated', fx.animId, 0.5 - fx.timer, fx.x - 28, fx.y - 28, 56, 56, false, 1.0, true);
            });

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
