/* Runenpfad shared simulation helpers: tile collision, event queue, fx wrappers and the seeded gameplay RNG. DOM-free. */
(function (root) {
    'use strict';

    function fxApi() {
        return root.RunenpfadFx || null;
    }

    function pushEvent(sim, type, detail) {
        sim.events.push({ type: type, detail: detail || null });
    }

    // Particle helpers keep every effect call site short and safe when fx.js is absent.
    function fxSpawn(sim, kind, x, y, opts) {
        if (!sim.fx) return null;
        return fxApi().spawn(sim.fx, kind, x, y, opts);
    }

    function fxBurst(sim, kind, x, y, count, opts) {
        if (!sim.fx) return;
        fxApi().burst(sim.fx, kind, x, y, count, opts);
    }

    function fxShake(sim, amount) {
        if (!sim.fx) return;
        fxApi().shake(sim.fx, amount);
    }

    function fxFlash(sim, color, alpha, duration) {
        if (!sim.fx) return;
        fxApi().flash(sim.fx, color, alpha, duration);
    }

    function fxHitstop(sim, seconds) {
        if (!sim.fx) return;
        fxApi().hitstop(sim.fx, seconds);
    }

    // Radial ring effect; safe when fx.js or the fx system is absent.
    function ring(sim, x, y, opts) {
        if (!sim.fx || !fxApi()) return null;
        return fxApi().ring(sim.fx, x, y, opts);
    }

    // Seeded gameplay RNG (LCG over sim.rngState). Never use fx.rng for gameplay decisions.
    function rand(sim) {
        let s = sim.rngState >>> 0;
        s = (Math.imul(s, 1664525) + 1013904223) >>> 0;
        sim.rngState = s;
        return s / 4294967296;
    }

    // Every jump shares the same kick-off dust so the feedback stays consistent.
    function pushJump(sim) {
        pushEvent(sim, 'jump', { x: sim.player.x, y: sim.player.y });
        fxBurst(sim, 'dust', sim.player.x, sim.player.y, 6, {
            color: '#d9c9a8',
            speed: 95,
            arc: Math.PI,
            angle: -Math.PI / 2,
            life: 0.36
        });
    }

    function rectOverlap(ax, ay, aw, ah, bx, by, bw, bh) {
        return ax < bx + bw && ax + aw > bx && ay < by + bh && ay + ah > by;
    }

    function isSolidTile(stage, tx, ty) {
        if (tx < 0 || tx >= stage.widthTiles || ty < 0) return true;
        if (ty >= stage.heightTiles) return false;
        return !!stage.solidMap[tx + ',' + ty];
    }

    function resolveHorizontalTiles(stage, entity, w, h) {
        const TILE = stage.tileSize;
        const top = entity.y - h + 2;
        const bottom = entity.y - 2;
        const tTop = Math.floor(top / TILE);
        const tBot = Math.floor(bottom / TILE);
        if (entity.vx > 0) {
            const right = Math.floor((entity.x + w / 2) / TILE);
            for (let ty = tTop; ty <= tBot; ty++) {
                if (isSolidTile(stage, right, ty)) {
                    entity.x = right * TILE - w / 2 - 0.01;
                    entity.vx = 0;
                    return true;
                }
            }
        } else if (entity.vx < 0) {
            const left = Math.floor((entity.x - w / 2) / TILE);
            for (let ty = tTop; ty <= tBot; ty++) {
                if (isSolidTile(stage, left, ty)) {
                    entity.x = (left + 1) * TILE + w / 2 + 0.01;
                    entity.vx = 0;
                    return true;
                }
            }
        }
        return false;
    }

    function resolveVerticalTiles(stage, entity, w, h, prevY, dropThrough) {
        const TILE = stage.tileSize;
        const left = entity.x - w / 2 + 2;
        const right = entity.x + w / 2 - 2;
        const tLeft = Math.floor(left / TILE);
        const tRight = Math.floor(right / TILE);
        entity.onGround = false;
        entity.onPlatformRide = null;

        if (entity.vy >= 0) {
            const tBot = Math.floor(entity.y / TILE);
            for (let tx = tLeft; tx <= tRight; tx++) {
                if (isSolidTile(stage, tx, tBot)) {
                    entity.y = tBot * TILE;
                    entity.vy = 0;
                    entity.onGround = true;
                    return;
                }
            }
            if (!dropThrough) {
                for (let i = 0; i < stage.platforms.length; i++) {
                    const p = stage.platforms[i];
                    if (!p.solid) continue;
                    if (entity.x + w / 2 > p.x + 2 && entity.x - w / 2 < p.x + p.w - 2) {
                        if (prevY <= p.y + 8 && entity.y >= p.y) {
                            entity.y = p.y;
                            if (p.kind === 'spring') {
                                entity.vy = -790;
                                entity.onGround = false;
                                return 'spring';
                            }
                            if (p.kind === 'trapdoor' && p.crumbleTimer === 0) {
                                p.crumbleTimer = p.crumbleTime || 0.45;
                            }
                            entity.vy = 0;
                            entity.onGround = true;
                            return;
                        }
                    }
                }
                for (let i = 0; i < stage.movingPlatforms.length; i++) {
                    const mp = stage.movingPlatforms[i];
                    if (entity.x + w / 2 > mp.x + 2 && entity.x - w / 2 < mp.x + mp.w - 2) {
                        if (prevY <= mp.y + 10 && entity.y >= mp.y) {
                            entity.y = mp.y;
                            entity.vy = 0;
                            entity.onGround = true;
                            entity.onPlatformRide = mp;
                            return;
                        }
                    }
                }
            }
        } else {
            const tTop = Math.floor((entity.y - h) / TILE);
            for (let tx = tLeft; tx <= tRight; tx++) {
                if (isSolidTile(stage, tx, tTop)) {
                    entity.y = (tTop + 1) * TILE + h + 0.01;
                    entity.vy = 0;
                    return;
                }
            }
        }
        return undefined;
    }

    root.RunenpfadPhysics = {
        pushEvent: pushEvent,
        pushJump: pushJump,
        fxSpawn: fxSpawn,
        fxBurst: fxBurst,
        fxShake: fxShake,
        fxFlash: fxFlash,
        fxHitstop: fxHitstop,
        ring: ring,
        rand: rand,
        rectOverlap: rectOverlap,
        isSolidTile: isSolidTile,
        resolveHorizontalTiles: resolveHorizontalTiles,
        resolveVerticalTiles: resolveVerticalTiles
    };
})(typeof window !== 'undefined' ? window : globalThis);
