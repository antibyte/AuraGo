/* Runenpfad pure 60 Hz 2D platformer simulation, combat, boss AI and persistence. DOM-free. */
(function (root) {
    'use strict';

    const STEP = 1 / 60;
    const GRAVITY = 1480;
    const MAX_FALL = 760;

    function levelsApi() {
        return root.RunenpfadLevels;
    }

    // Profile handling lives in profile.js; the simulation re-exports it for callers.
    function profileApi() {
        return root.RunenpfadProfile || null;
    }

    function fxApi() {
        return root.RunenpfadFx || null;
    }

    function behaviorsApi() {
        return root.RunenpfadBehaviors || null;
    }

    function varietyApi() {
        return root.RunenpfadVariety || null;
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

    function createSimulation(options) {
        options = options || {};
        const levels = levelsApi();
        const profileRef = profileApi();
        const profile = profileRef ? profileRef.exportProfile(options.profile) : (options.profile || {});
        const mode = options.mode === 'endless' ? 'endless' : 'campaign';
        const levelId = Math.min(levels.CAMPAIGN_LEVELS.length, Math.max(1, Math.floor(Number(options.levelId) || 1)));
        const endlessFloor = Math.min(9999, Math.max(1, Math.floor(Number(options.endlessFloor) || 1)));
        const heroId = options.hero === 'ranger' ? 'ranger' : (options.hero === 'knight' ? 'knight' : profile.hero);
        const stage = levels.buildStage(mode === 'endless' ? { mode: 'endless', endlessFloor: endlessFloor, seed: options.seed || 9001 } : levelId);
        const stats = profileRef ? profileRef.heroStats(heroId, profile) : levels.HERO_DEFS[heroId];
        const fxApiRef = fxApi();

        return {
            mode: mode,
            levelId: mode === 'campaign' ? stage.id : endlessFloor,
            endlessFloor: mode === 'endless' ? endlessFloor : 0,
            seed: options.seed || 9001,
            heroId: heroId,
            profile: profile,
            stage: stage,
            stats: stats,
            state: 'playing',
            elapsed: 0,
            accum: 0,
            stepCount: 0,
            damageTaken: 0,
            stageCoins: 0,
            stageScore: 0,
            gemsCollected: [false, false, false],
            gemCount: 0,
            hasKey: false,
            earnedStars: 0,
            notice: 'runenpfad.notice.find_key',
            checkpoint: { x: stage.spawn.x, y: stage.spawn.y },
            checkpointUsed: false,
            player: {
                x: stage.spawn.x,
                y: stage.spawn.y,
                vx: 0,
                vy: 0,
                w: 28,
                h: 44,
                facing: 1,
                onGround: false,
                onLadder: false,
                onPlatformRide: null,
                coyoteTimer: 0,
                jumpBuffer: 0,
                airJumpsLeft: stats.maxAirJumps,
                hp: stats.maxHp,
                maxHp: stats.maxHp,
                potions: stats.potions + (stage.modEffects && stage.modEffects.potionBonus ? stage.modEffects.potionBonus : 0),
                invulnTimer: 0,
                attackTimer: 0,
                attackCooldown: 0,
                dashTimer: 0,
                dashCooldown: 0,
                slamActive: false,
                hurtTimer: 0,
                stepTimer: 0,
                animState: 'idle',
                animTime: 0
            },
            projectiles: [],
            effects: [],
            floatingTexts: [],
            events: [],
            blessing: varietyApi() ? varietyApi().createBlessing() : { runePower: 0, shrineCount: 0 },
            fx: fxApiRef ? fxApiRef.createSystem((mode === 'endless' ? endlessFloor : levelId) * 7919 + 13) : null
        };
    }

    function pushEvent(sim, type, detail) {
        sim.events.push({ type: type, detail: detail || null });
    }

    // Every jump shares the same kick-off dust so the feedback stays consistent.
    function pushJump(sim) {
        pushEvent(sim, 'jump', null);
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
                                p.crumbleTimer = 0.45;
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

    function hurtPlayer(sim, amount, sourceX, sound) {
        const p = sim.player;
        if (p.invulnTimer > 0 || sim.state !== 'playing') return;
        p.hp = Math.max(0, p.hp - amount);
        sim.damageTaken += amount;
        p.invulnTimer = 1.1;
        p.hurtTimer = 0.28;
        p.slamActive = false;
        p.vy = -300;
        p.vx = p.x >= sourceX ? 180 : -180;
        pushEvent(sim, 'player_hurt', { hp: p.hp, sound: sound || 'impact-flesh' });
        sim.floatingTexts.push({ x: p.x, y: p.y - 44, text: '-' + amount, color: '#ff6b7d', timer: 0.75 });
        fxFlash(sim, '#ff3b5c', 0.26, 0.22);
        fxShake(sim, 5.5);
        fxBurst(sim, 'spark', p.x, p.y - 22, 10, { color: '#ff6b7d', speed: 210, arc: Math.PI * 2, life: 0.34 });
        if (p.hp <= 0) {
            if (!sim.checkpointUsed && sim.checkpoint) {
                sim.checkpointUsed = true;
                p.hp = Math.max(2, Math.ceil(p.maxHp * 0.6));
                p.x = sim.checkpoint.x;
                p.y = sim.checkpoint.y;
                p.vx = 0;
                p.vy = 0;
                p.invulnTimer = 1.6;
                sim.notice = 'runenpfad.notice.respawn';
                fxFlash(sim, '#7ee8fa', 0.3, 0.3);
                fxBurst(sim, 'rune', p.x, p.y - 22, 12, { color: '#7ee8fa', speed: 150, life: 0.6 });
            } else {
                sim.state = 'lost';
                p.animState = 'death';
                p.animTime = 0;
                pushEvent(sim, 'player_defeat', null);
                fxFlash(sim, '#ff3b5c', 0.45, 0.5);
                fxShake(sim, 9);
                fxBurst(sim, 'shard', p.x, p.y - 22, 16, { color: '#ff6b7d', speed: 240, life: 0.7 });
            }
        }
    }

    function damageEnemy(sim, enemy, dmg, hitX) {
        if (enemy.dead) return;
        enemy.hp = Math.max(0, enemy.hp - dmg);
        enemy.hurtTimer = 0.22;
        if (!enemy.def.boss && enemy.vx !== undefined) {
            enemy.vx = (enemy.x >= hitX ? 1 : -1) * Math.max(70, enemy.def.speed);
        }
        sim.floatingTexts.push({ x: enemy.x, y: enemy.y - 46, text: String(dmg), color: '#ffe066', timer: 0.6 });
        pushEvent(sim, 'enemy_hit', { sound: enemy.def.hitSound, x: enemy.x, y: enemy.y });
        fxBurst(sim, 'spark', hitX, enemy.y - 24, 7, {
            color: enemy.def.hitSound === 'impact-metal' ? '#cfe8ff' : '#ffe066',
            speed: 190,
            arc: Math.PI * 2,
            life: 0.3
        });
        if (enemy.hp <= 0) {
            enemy.dead = true;
            enemy.deathTimer = 0.55;
            enemy.animState = 'death';
            enemy.animTime = 0;
            sim.stageScore += enemy.score || enemy.def.score;
            sim.effects.push({ animId: 'explosion', x: enemy.x, y: enemy.y - 24, timer: 0.5 });
            fxBurst(sim, 'shard', enemy.x, enemy.y - 24, enemy.def.boss ? 26 : 12, {
                color: enemy.def.hitSound === 'impact-metal' ? '#cfe8ff' : '#ffd08a',
                speed: enemy.def.boss ? 320 : 220,
                life: 0.8
            });
            fxBurst(sim, 'ember', enemy.x, enemy.y - 24, enemy.def.boss ? 22 : 8, {
                color: '#ffb347',
                speed: 90,
                life: 1.1
            });
            fxShake(sim, enemy.def.boss ? 11 : 3.6);
            fxHitstop(sim, enemy.def.boss ? 0.12 : 0.045);
            if (enemy.def.boss) {
                fxFlash(sim, '#fff3c4', 0.4, 0.4);
                sim.notice = 'runenpfad.notice.boss_defeated';
                pushEvent(sim, 'boss_defeat', { id: enemy.id });
                if (sim.stage.key && sim.stage.key.guardedByBoss && !sim.stage.key.collected) {
                    sim.stage.key.guardedByBoss = false;
                    sim.stage.key.x = enemy.x;
                    sim.stage.key.y = sim.stage.groundY - 24;
                }
            } else {
                pushEvent(sim, 'enemy_kill', { id: enemy.defId });
            }
            // Bomber drones detonate when killed, hurting everything close by.
            if (enemy.def.behavior === 'bomber' && !enemy.blastDone) {
                enemy.blastDone = true;
                pushEvent(sim, 'bomb_blast', { x: enemy.x, y: enemy.y });
                sim.effects.push({ animId: 'explosion', x: enemy.x, y: enemy.y - 20, timer: 0.5 });
                fxBurst(sim, 'shard', enemy.x, enemy.y - 20, 18, { color: '#fb923c', speed: 300, life: 0.8 });
                fxBurst(sim, 'ember', enemy.x, enemy.y - 20, 12, { color: '#fde68a', speed: 150, life: 1.0 });
                fxShake(sim, 7);
                fxFlash(sim, '#fb923c', 0.2, 0.24);
                stage.enemies.forEach(function (o) {
                    if (!o.dead && o !== enemy && Math.hypot(o.x - enemy.x, o.y - enemy.y) < 96) {
                        damageEnemy(sim, o, sim.stats.damage + 2, enemy.x);
                    }
                });
                const p = sim.player;
                if (Math.hypot(p.x - enemy.x, p.y - enemy.y) < 96) {
                    hurtPlayer(sim, 1, enemy.x, 'impact-metal');
                }
            }
        } else if (enemy.def.boss && enemy.hp <= enemy.maxHp * 0.5) {
            enemy.phase = 2;
            pushEvent(sim, 'boss_phase', { id: enemy.id });
            fxFlash(sim, '#fb7185', 0.32, 0.35);
            fxShake(sim, 8);
            fxBurst(sim, 'rune', enemy.x, enemy.y - 28, 18, { color: '#fb7185', speed: 210, life: 0.8 });
        }
    }

    function breakCrate(sim, crate) {
        if (crate.broken) return;
        crate.hp -= 1;
        pushEvent(sim, 'crate_break', { x: crate.x, y: crate.y });
        fxBurst(sim, 'dust', crate.x, crate.y + 20, 5, { color: '#c9a97a', speed: 90, life: 0.4 });
        if (crate.hp <= 0) {
            crate.broken = true;
            sim.effects.push({ animId: 'smoke', x: crate.x, y: crate.y + 18, timer: 0.45 });
            fxBurst(sim, 'shard', crate.x, crate.y + 20, 10, { color: '#c9a97a', speed: 200, life: 0.7 });
            fxShake(sim, 2.4);
            const dropKind = crate.drop || 'coin';
            const asset = dropKind === 'apple' ? 'apple' : dropKind === 'heart' ? 'heart' : dropKind === 'star' ? 'star' : 'coin';
            sim.stage.collectibles.push({
                id: 'drop_' + sim.stepCount + '_' + Math.floor(crate.x),
                kind: dropKind,
                assetId: asset,
                animated: asset === 'coin',
                animId: asset === 'coin' ? 'coin_spin' : '',
                x: crate.x,
                y: crate.y + 20,
                collected: false
            });
        }
    }

    function performPlayerAttack(sim, input) {
        const p = sim.player;
        const stage = sim.stage;
        const stats = sim.stats;
        const variety = varietyApi();
        const blessingMul = variety ? variety.damageMultiplier(sim.blessing) : 1;
        const attackDamage = stats.damage * blessingMul;
        if (p.attackCooldown > 0) return;

        // Mid-air knight ground slam
        if (stats.def.slamEnabled && input.down && !p.onGround && !p.onLadder) {
            p.slamActive = true;
            p.vy = Math.max(p.vy, 660);
            p.attackCooldown = stats.def.attackCooldown;
            p.attackTimer = stats.def.attackDuration;
            p.animState = 'attack';
            p.animTime = 0;
            pushEvent(sim, 'slam', null);
            pushEvent(sim, 'attack_melee', null);
            return;
        }

        p.attackCooldown = stats.def.attackCooldown;
        p.attackTimer = stats.def.attackDuration;
        p.animState = 'attack';
        p.animTime = 0;

        if (sim.heroId === 'knight') {
            pushEvent(sim, 'attack_melee', null);
            const reach = 64;
            const hx = p.facing > 0 ? p.x : p.x - reach;
            const hy = p.y - p.h - 6;
            const hw = reach;
            const hh = p.h + 10;
            fxBurst(sim, 'spark', p.x + p.facing * 30, p.y - 26, 5, {
                color: '#7ee8fa',
                speed: 230,
                arc: 1.1,
                angle: p.facing > 0 ? -0.35 : Math.PI + 0.35,
                life: 0.24
            });
            stage.enemies.forEach(function (e) {
                if (!e.dead) {
                    const ew = 40 * e.def.scale;
                    const eh = 48 * e.def.scale;
                    if (rectOverlap(hx, hy, hw, hh, e.x - ew / 2, e.y - eh, ew, eh)) {
                        damageEnemy(sim, e, attackDamage, p.x);
                    }
                }
            });
            stage.crates.forEach(function (c) {
                if (!c.broken && rectOverlap(hx, hy, hw, hh, c.x - c.w / 2, c.y + stage.tileSize - c.h, c.w, c.h)) {
                    breakCrate(sim, c);
                }
            });
            // Knight slash deflects enemy projectiles
            sim.projectiles.forEach(function (proj) {
                if (proj.fromEnemy && !proj.dead && rectOverlap(hx, hy, hw, hh, proj.x - 8, proj.y - 8, 16, 16)) {
                    proj.fromEnemy = false;
                    proj.vx = -proj.vx * 1.3;
                    proj.damage = attackDamage;
                    proj.color = '#7ee8fa';
                }
            });
        } else {
            pushEvent(sim, 'attack_ranged', null);
            fxBurst(sim, 'rune', p.x + p.facing * 18, p.y - 24, 4, {
                color: '#7ee8fa',
                speed: 110,
                life: 0.34
            });
            sim.projectiles.push({
                x: p.x + p.facing * 18,
                y: p.y - 24,
                vx: p.facing * 560,
                vy: 0,
                fromEnemy: false,
                damage: stats.damage,
                life: 1.15,
                color: '#7ee8fa',
                dead: false
            });
        }
    }

    function stepPlatformsAndCannons(sim, dt) {
        const stage = sim.stage;
        stage.movingPlatforms.forEach(function (mp) {
            mp.prevX = mp.x;
            mp.x += mp.speed * mp.dir * dt;
            if (mp.x >= mp.endX) {
                mp.x = mp.endX;
                mp.dir = -1;
            } else if (mp.x <= mp.startX) {
                mp.x = mp.startX;
                mp.dir = 1;
            }
        });
        stage.platforms.forEach(function (p) {
            if (p.kind !== 'trapdoor') return;
            if (p.crumbleTimer > 0) {
                p.crumbleTimer -= dt;
                if (p.crumbleTimer <= 0) {
                    p.crumbleTimer = 0;
                    p.solid = false;
                    p.respawnTimer = 2.6;
                }
            } else if (p.respawnTimer > 0) {
                p.respawnTimer -= dt;
                if (p.respawnTimer <= 0) {
                    p.respawnTimer = 0;
                    p.solid = true;
                }
            }
        });
        stage.cannons.forEach(function (c) {
            c.timer -= dt;
            if (c.timer <= 0) {
                c.timer = c.interval;
                if (Math.abs(sim.player.x - c.x) < 520) {
                    pushEvent(sim, 'cannon', { x: c.x, y: c.y });
                    fxBurst(sim, 'spark', c.x + c.dir * 22, c.y, 5, { color: '#fb923c', speed: 150, life: 0.26 });
                    sim.projectiles.push({
                        x: c.x + c.dir * 20,
                        y: c.y,
                        vx: c.dir * 250,
                        vy: 0,
                        fromEnemy: true,
                        damage: 1,
                        life: 2.2,
                        color: '#fb923c',
                        dead: false
                    });
                }
            }
        });

        // Falling stones: they drop when the hero enters their drop lane.
        (stage.boulders || []).forEach(function (b) {
            if (b.state === 'idle') {
                if (Math.abs(sim.player.x - (b.x + b.w / 2)) < 250) {
                    b.state = 'falling';
                    b.vy = 0;
                }
            } else if (b.state === 'falling') {
                b.vy = Math.min(MAX_FALL, b.vy + GRAVITY * dt);
                b.y += b.vy * dt;
                if (b.y >= stage.groundY - b.h) {
                    b.y = stage.groundY - b.h;
                    b.state = 'landed';
                    b.timer = 1.4;
                    pushEvent(sim, 'boulder_land', null);
                    fxBurst(sim, 'dust', b.x + b.w / 2, b.y + b.h, 9, { color: '#cfc3ae', speed: 180, arc: Math.PI, angle: Math.PI / 2, life: 0.5 });
                    fxShake(sim, 4);
                }
            } else if (b.state === 'landed') {
                b.timer -= dt;
                if (b.timer <= 0) {
                    b.state = 'idle';
                    b.y = b.spawnY;
                    b.vy = 0;
                }
            }
        });
    }

    // Tile helpers handed to the behaviour module so enemy AI stays out of the simulation core.
    const ENEMY_API = {
        GRAVITY: GRAVITY,
        MAX_FALL: MAX_FALL,
        isSolidTile: isSolidTile,
        resolveHorizontalTiles: resolveHorizontalTiles,
        resolveVerticalTiles: resolveVerticalTiles
    };

    function stepEnemies(sim, dt) {
        const stage = sim.stage;
        const p = sim.player;
        const behaviors = behaviorsApi();
        stage.enemies.forEach(function (e) {
            e.animTime += dt;
            if (e.dead) {
                if (e.deathTimer > 0) e.deathTimer -= dt;
                return;
            }
            if (e.hurtTimer > 0) e.hurtTimer -= dt;
            const speedMul = e.phase === 2 ? 1.38 : 1.0;

            if (behaviors) {
                behaviors.update(sim, e, dt, ENEMY_API);
            } else {
                e.x += e.vx * speedMul * dt;
            }

            if (e.def.ranged) {
                e.shootTimer -= dt * speedMul;
                if (e.shootTimer <= 0) {
                    e.shootTimer = e.def.boss ? 1.45 : 2.2;
                    if (Math.abs(p.x - e.x) < 420) {
                        pushEvent(sim, 'enemy_shot', { id: e.id });
                        const dx = p.x - e.x;
                        const dy = (p.y - 22) - (e.y - 24);
                        const len = Math.max(1, Math.hypot(dx, dy));
                        const spd = e.def.boss ? 260 : 210;
                        sim.projectiles.push({
                            x: e.x + (dx >= 0 ? 18 : -18),
                            y: e.y - 24,
                            vx: (dx / len) * spd,
                            vy: e.def.flying ? (dy / len) * spd * 0.65 : 0,
                            fromEnemy: true,
                            damage: 1,
                            life: 2.1,
                            color: '#ff6b7d',
                            dead: false
                        });
                    }
                }
            }

            e.animState = e.hurtTimer > 0 ? 'hurt' : (Math.abs(e.vx) < 1 ? 'idle' : 'move');
            const ew = 28 * e.def.scale;
            const eh = 38 * e.def.scale;
            if (rectOverlap(p.x - p.w / 2, p.y - p.h, p.w, p.h, e.x - ew / 2, e.y - eh, ew, eh)) {
                hurtPlayer(sim, e.def.damage, e.x, e.def.hitSound);
            }
        });
    }

    function stepProjectiles(sim, dt) {
        const stage = sim.stage;
        const p = sim.player;
        const TILE = stage.tileSize;
        sim.projectiles.forEach(function (proj) {
            if (proj.dead) return;
            proj.x += proj.vx * dt;
            proj.y += proj.vy * dt;
            proj.life -= dt;
            if (proj.life <= 0) {
                proj.dead = true;
                return;
            }
            const tx = Math.floor(proj.x / TILE);
            const ty = Math.floor(proj.y / TILE);
            if (isSolidTile(stage, tx, ty)) {
                proj.dead = true;
                return;
            }
            if (proj.fromEnemy) {
                if (rectOverlap(p.x - p.w / 2, p.y - p.h, p.w, p.h, proj.x - 6, proj.y - 6, 12, 12)) {
                    proj.dead = true;
                    hurtPlayer(sim, proj.damage, proj.x, 'impact-metal');
                }
            } else {
                stage.enemies.forEach(function (e) {
                    if (proj.dead || e.dead) return;
                    const ew = 34 * e.def.scale;
                    const eh = 42 * e.def.scale;
                    if (rectOverlap(proj.x - 8, proj.y - 8, 16, 16, e.x - ew / 2, e.y - eh, ew, eh)) {
                        proj.dead = true;
                        damageEnemy(sim, e, proj.damage, proj.x);
                    }
                });
                stage.crates.forEach(function (c) {
                    if (proj.dead || c.broken) return;
                    if (rectOverlap(proj.x - 6, proj.y - 6, 12, 12, c.x - c.w / 2, c.y + TILE - c.h, c.w, c.h)) {
                        proj.dead = true;
                        breakCrate(sim, c);
                    }
                });
            }
        });
        if (sim.projectiles.length > 48) {
            sim.projectiles = sim.projectiles.filter(function (pr) { return !pr.dead; });
        }
    }

    function finalizeVictory(sim) {
        const stage = sim.stage;
        let stars = 1;
        if (sim.gemCount >= 3) stars++;
        if (sim.elapsed <= stage.parTime || sim.damageTaken === 0) stars++;
        sim.earnedStars = stars;
        sim.state = 'won';
        sim.notice = 'runenpfad.notice.victory';
        if (sim.mode === 'campaign') {
            const prevStars = sim.profile.starsByLevel[stage.id] || 0;
            if (stars > prevStars) sim.profile.starsByLevel[stage.id] = stars;
            const prevBest = sim.profile.bestTimeByLevel[stage.id];
            const cleanElapsed = Math.round(sim.elapsed * 10) / 10;
            if (!prevBest || cleanElapsed < prevBest) sim.profile.bestTimeByLevel[stage.id] = cleanElapsed;
            if (stage.id >= sim.profile.unlockedLevel && stage.id < levelsApi().CAMPAIGN_LEVELS.length) {
                sim.profile.unlockedLevel = stage.id + 1;
            }
        } else {
            if (sim.endlessFloor > sim.profile.endlessBestFloor) {
                sim.profile.endlessBestFloor = sim.endlessFloor;
            }
        }
        sim.profile.totalScore += sim.stageScore + stars * 150;
        pushEvent(sim, 'portal_win', { stars: stars });
        fxFlash(sim, '#fff3c4', 0.35, 0.5);
        fxBurst(sim, 'rune', sim.player.x, sim.player.y - 24, 24, { color: '#7ee8fa', speed: 250, life: 1.1 });
        fxBurst(sim, 'ember', sim.player.x, sim.player.y - 24, 16, { color: '#ffe066', speed: 170, life: 1.2 });
    }

    function stepInteractables(sim) {
        const p = sim.player;
        const stage = sim.stage;
        const TILE = stage.tileSize;
        const px = p.x - p.w / 2;
        const py = p.y - p.h;
        const magnet = sim.stats.magnetRadius;

        stage.collectibles.forEach(function (item) {
            if (item.collected) return;
            const dx = item.x - p.x;
            const dy = item.y - (p.y - 22);
            const dist = Math.hypot(dx, dy);
            if (item.kind === 'coin' && dist < magnet + 24 && dist > 1) {
                item.x -= (dx / dist) * 4.5;
                item.y -= (dy / dist) * 4.5;
            }
            if (dist < 30) {
                item.collected = true;
                if (item.kind === 'coin') {
                    const value = Math.max(1, Math.round(stage.coinValue || 1));
                    sim.stageCoins += value;
                    sim.profile.coins += value;
                    sim.stageScore += 25 * value;
                    pushEvent(sim, 'pickup_coin', null);
                    fxBurst(sim, 'ember', item.x, item.y, 3, { color: '#ffd65a', speed: 70, life: 0.4 });
                } else if (item.kind === 'gem') {
                    if (!sim.gemsCollected[item.gemIndex]) {
                        sim.gemsCollected[item.gemIndex] = true;
                        sim.gemCount += 1;
                    }
                    sim.stageScore += 100;
                    pushEvent(sim, 'pickup_gem', { gemIndex: item.gemIndex });
                    sim.floatingTexts.push({ x: item.x, y: item.y - 12, text: '+GEM', color: '#7ee8fa', timer: 0.85 });
                    fxBurst(sim, 'rune', item.x, item.y, 12, { color: '#7ee8fa', speed: 165, life: 0.7 });
                    fxFlash(sim, '#7ee8fa', 0.16, 0.2);
                } else if (item.kind === 'star') {
                    sim.stageScore += 150;
                    pushEvent(sim, 'pickup_gem', null);
                    sim.floatingTexts.push({ x: item.x, y: item.y - 12, text: '+STAR', color: '#ffe066', timer: 0.85 });
                    fxBurst(sim, 'rune', item.x, item.y, 10, { color: '#ffe066', speed: 150, life: 0.7 });
                } else if (item.kind === 'heart' || item.kind === 'apple') {
                    p.hp = Math.min(p.maxHp, p.hp + 1);
                    sim.stageScore += 50;
                    pushEvent(sim, 'pickup_heart', null);
                    sim.floatingTexts.push({ x: item.x, y: item.y - 12, text: '+1 HP', color: '#86efac', timer: 0.75 });
                    fxBurst(sim, 'ember', item.x, item.y, 6, { color: '#86efac', speed: 85, life: 0.55 });
                } else if (item.kind === 'potion') {
                    p.potions += 1;
                    pushEvent(sim, 'pickup_potion', null);
                    sim.floatingTexts.push({ x: item.x, y: item.y - 12, text: '+POTION', color: '#c084fc', timer: 0.85 });
                    fxBurst(sim, 'rune', item.x, item.y, 8, { color: '#c084fc', speed: 120, life: 0.6 });
                } else if (item.kind === 'treasure') {
                    sim.stageCoins += 10;
                    sim.profile.coins += 10;
                    sim.stageScore += 250;
                    pushEvent(sim, 'pickup_gem', null);
                    sim.floatingTexts.push({ x: item.x, y: item.y - 12, text: '+10', color: '#ffd65a', timer: 0.9 });
                    fxBurst(sim, 'ember', item.x, item.y, 14, { color: '#ffd65a', speed: 130, life: 0.9 });
                    fxFlash(sim, '#ffd65a', 0.2, 0.25);
                }
            }
        });

        stage.checkpoints.forEach(function (cp) {
            if (!cp.activated && rectOverlap(px, py, p.w, p.h, cp.x - 22, cp.y, 44, TILE)) {
                cp.activated = true;
                sim.checkpoint = { x: cp.x, y: cp.y + TILE };
                p.hp = Math.min(p.maxHp, p.hp + 1);
                sim.notice = 'runenpfad.notice.checkpoint';
                pushEvent(sim, 'checkpoint', null);
                fxBurst(sim, 'ember', cp.x, cp.y + 8, 14, { color: '#ffb347', speed: 110, life: 0.9 });
                fxFlash(sim, '#ffb347', 0.14, 0.3);
            }
        });

        if (stage.key && !stage.key.collected && !stage.key.guardedByBoss) {
            if (Math.hypot(stage.key.x - p.x, stage.key.y - (p.y - 22)) < 34) {
                stage.key.collected = true;
                sim.hasKey = true;
                if (stage.door) stage.door.opened = true;
                sim.notice = 'runenpfad.notice.key_found';
                sim.stageScore += 200;
                pushEvent(sim, 'pickup_key', null);
                sim.floatingTexts.push({ x: stage.key.x, y: stage.key.y - 16, text: 'KEY!', color: '#ffe066', timer: 1.0 });
                fxBurst(sim, 'rune', stage.key.x, stage.key.y, 18, { color: '#ffe066', speed: 210, life: 0.9 });
                fxFlash(sim, '#ffe066', 0.24, 0.35);
                fxShake(sim, 3.5);
            }
        }

        stage.hazards.forEach(function (h) {
            const top = h.y + TILE - h.h + 4;
            if (rectOverlap(px + 2, py + 4, p.w - 4, p.h - 6, h.x + 4, top, h.w - 8, h.h - 4)) {
                hurtPlayer(sim, 1, h.x + h.w / 2, 'impact-stone');
            }
        });

        stage.boulders.forEach(function (b) {
            if (b.state !== 'falling') return;
            if (rectOverlap(px + 2, py + 4, p.w - 4, p.h - 6, b.x + 6, b.y + 6, b.w - 12, b.h - 12)) {
                hurtPlayer(sim, 1, b.x + b.w / 2, 'impact-stone');
            }
        });

        // Rune shrine: a short blessing that doubles the hero's damage.
        (stage.shrines || []).forEach(function (sh) {
            if (sh.taken) return;
            if (rectOverlap(px, py, p.w, p.h, sh.x - 24, sh.y - 22, 48, 48)) {
                sh.taken = true;
                sim.blessing.runePower = varietyApi() ? varietyApi().RUNE_POWER_DURATION : 12;
                sim.stageScore += 150;
                pushEvent(sim, 'pickup_shrine', null);
                sim.floatingTexts.push({ x: sh.x, y: sh.y - 26, text: 'RUNE POWER', color: '#c084fc', timer: 1.1 });
                fxBurst(sim, 'rune', sh.x, sh.y, 18, { color: '#c084fc', speed: 220, life: 1.0 });
                fxFlash(sim, '#c084fc', 0.28, 0.4);
                fxShake(sim, 4);
            }
        });

        if (stage.portal && rectOverlap(px, py, p.w, p.h, stage.portal.x - 24, stage.portal.y - 6, 48, TILE + 8)) {
            if (sim.hasKey) {
                finalizeVictory(sim);
            } else {
                sim.notice = 'runenpfad.notice.need_key';
            }
        }
    }

    // Potion heal is shared by the normal tick and the impact-freeze tick.
    function applyPotionHeal(sim) {
        const p = sim.player;
        if (p.potions <= 0 || p.hp >= p.maxHp) return;
        p.potions -= 1;
        p.hp = Math.min(p.maxHp, p.hp + 2);
        pushEvent(sim, 'heal', null);
        sim.floatingTexts.push({ x: p.x, y: p.y - 44, text: '+2 HP', color: '#c084fc', timer: 0.8 });
        fxBurst(sim, 'rune', p.x, p.y - 24, 12, { color: '#c084fc', speed: 150, life: 0.7 });
        fxFlash(sim, '#c084fc', 0.16, 0.24);
    }

    function stepOneTick(sim, input) {
        if (sim.state !== 'playing') return;
        const dt = STEP;
        const p = sim.player;
        const stage = sim.stage;
        const stats = sim.stats;
        const TILE = stage.tileSize;

        // Impact freeze: the world holds for a few frames, but the hero still reacts to input.
        if (sim.fx && sim.fx.hitstop > 0) {
            sim.fx.hitstop = Math.max(0, sim.fx.hitstop - dt);
            if (input.healPressed) applyPotionHeal(sim);
            if (input.attackPressed) performPlayerAttack(sim, input);
            if (p.y > stage.height + 64) hurtPlayer(sim, p.maxHp, p.x, 'impact-flesh');
            stepInteractables(sim);
            return;
        }

        sim.elapsed += dt;
        sim.stepCount += 1;

        if (p.invulnTimer > 0) p.invulnTimer = Math.max(0, p.invulnTimer - dt);
        if (p.attackTimer > 0) p.attackTimer = Math.max(0, p.attackTimer - dt);
        if (p.attackCooldown > 0) p.attackCooldown = Math.max(0, p.attackCooldown - dt);
        if (p.dashCooldown > 0) p.dashCooldown = Math.max(0, p.dashCooldown - dt);
        if (p.hurtTimer > 0) p.hurtTimer = Math.max(0, p.hurtTimer - dt);
        if (sim.blessing && sim.blessing.runePower > 0) {
            sim.blessing.runePower = Math.max(0, sim.blessing.runePower - dt);
        }

        stepPlatformsAndCannons(sim, dt);

        // Potion heal
        if (input.healPressed) applyPotionHeal(sim);

        // Dash roll (Ranger)
        if (stats.def.dashEnabled && input.dashPressed && p.dashCooldown <= 0 && p.dashTimer <= 0) {
            p.dashTimer = 0.18;
            p.dashCooldown = 0.72;
            p.invulnTimer = Math.max(p.invulnTimer, 0.24);
            p.vx = p.facing * 520;
            p.vy = 0;
            pushEvent(sim, 'dash', null);
            fxBurst(sim, 'trail', p.x, p.y - 22, 8, {
                color: '#a5f3fc',
                speed: 70,
                arc: Math.PI,
                angle: p.facing > 0 ? Math.PI : 0,
                life: 0.3
            });
        }

        // Carry standing player on moving platforms
        if (p.onGround && p.onPlatformRide) {
            p.x += p.onPlatformRide.x - p.onPlatformRide.prevX;
        }

        // Check ladder overlap
        let nearLadder = false;
        for (let i = 0; i < stage.ladders.length; i++) {
            const l = stage.ladders[i];
            if (Math.abs(p.x - (l.x + TILE / 2)) < 20 && p.y > l.y && p.y - p.h < l.y + TILE) {
                nearLadder = true;
                break;
            }
        }
        if (nearLadder && (input.up || input.down)) {
            p.onLadder = true;
            p.slamActive = false;
        } else if (!nearLadder) {
            p.onLadder = false;
        }

        const moveDir = (input.right ? 1 : 0) - (input.left ? 1 : 0);
        if (moveDir !== 0 && p.dashTimer <= 0) p.facing = moveDir;

        if (p.dashTimer > 0) {
            p.dashTimer -= dt;
        } else if (p.onLadder) {
            p.vx = moveDir * stats.speed * 0.65;
            p.vy = input.up ? -190 : input.down ? 190 : 0;
            if (input.jumpPressed) {
                p.onLadder = false;
                p.vy = -stats.jumpVel;
                pushJump(sim);
            }
        } else {
            const targetVx = moveDir * stats.speed + (stage.modEffects && stage.modEffects.wind ? stage.modEffects.wind : 0);
            const accel = p.onGround ? (stage.world.friction < 1 ? 650 : 2400) : 1400;
            const diff = targetVx - p.vx;
            const stepV = accel * dt;
            if (Math.abs(diff) <= stepV) p.vx = targetVx;
            else p.vx += Math.sign(diff) * stepV;

            if (p.onGround) {
                p.coyoteTimer = 0.11;
                p.airJumpsLeft = stats.maxAirJumps;
            } else {
                p.coyoteTimer = Math.max(0, p.coyoteTimer - dt);
            }
            if (input.jumpPressed) p.jumpBuffer = 0.11;
            else p.jumpBuffer = Math.max(0, p.jumpBuffer - dt);

            if (p.jumpBuffer > 0) {
                if (p.coyoteTimer > 0) {
                    p.vy = -stats.jumpVel;
                    p.onGround = false;
                    p.coyoteTimer = 0;
                    p.jumpBuffer = 0;
                    p.slamActive = false;
                    pushJump(sim);
                } else if (p.airJumpsLeft > 0) {
                    p.airJumpsLeft -= 1;
                    p.vy = -stats.jumpVel * 0.92;
                    p.jumpBuffer = 0;
                    p.slamActive = false;
                    pushJump(sim);
                }
            }
            p.vy = Math.min(MAX_FALL, p.vy + GRAVITY * dt);
        }

        if (input.attackPressed) {
            performPlayerAttack(sim, input);
        }

        const prevY = p.y;
        const wasOnGround = p.onGround;
        p.x += p.vx * dt;
        resolveHorizontalTiles(stage, p, p.w, p.h);
        p.y += p.vy * dt;
        const landResult = resolveVerticalTiles(stage, p, p.w, p.h, prevY, !!(input.down && !p.slamActive && !nearLadder));
        if (landResult === 'spring') {
            pushJump(sim);
            pushEvent(sim, 'spring', null);
            fxBurst(sim, 'rune', p.x, p.y, 10, { color: '#c4b5fd', speed: 190, life: 0.55 });
            fxShake(sim, 2.6);
        }

        if (!wasOnGround && p.onGround) {
            pushEvent(sim, 'land', null);
            fxBurst(sim, 'dust', p.x, p.y, 7, {
                color: '#d9c9a8',
                speed: 130,
                arc: Math.PI * 0.9,
                angle: Math.PI / 2,
                life: 0.38
            });
            if (p.slamActive) {
                p.slamActive = false;
                sim.effects.push({ animId: 'explosion', x: p.x, y: p.y - 16, timer: 0.45 });
                pushEvent(sim, 'slam_land', null);
                fxBurst(sim, 'shard', p.x, p.y - 8, 18, { color: '#ffd08a', speed: 300, life: 0.7 });
                fxBurst(sim, 'dust', p.x, p.y, 16, { color: '#e8d8b8', speed: 230, arc: Math.PI, angle: Math.PI / 2, life: 0.5 });
                if (sim.fx) {
                    fxApi().ring(sim.fx, p.x, p.y - 4, { color: '#ffd08a', size: 16, grow: 320, life: 0.42 });
                }
                fxShake(sim, 8);
                fxHitstop(sim, 0.06);
                fxFlash(sim, '#ffe9b0', 0.18, 0.18);
                stage.enemies.forEach(function (e) {
                    if (!e.dead && Math.hypot(e.x - p.x, e.y - p.y) < 120) {
                        const slamMul = varietyApi() ? varietyApi().damageMultiplier(sim.blessing) : 1;
                        damageEnemy(sim, e, (stats.damage + 1) * slamMul, p.x);
                    }
                });
                stage.crates.forEach(function (c) {
                    if (!c.broken && Math.hypot(c.x - p.x, c.y + TILE - p.y) < 110) {
                        breakCrate(sim, c);
                    }
                });
            }
        }

        // Fall out of bottom abyss
        if (p.y > stage.height + 64) {
            hurtPlayer(sim, p.maxHp, p.x);
        }

        // Footstep SFX pacing
        if (p.onGround && Math.abs(p.vx) > 40) {
            p.stepTimer -= dt;
            if (p.stepTimer <= 0) {
                p.stepTimer = 0.28;
                pushEvent(sim, 'step', { sound: stage.world.stepSound });
            }
        }

        stepEnemies(sim, dt);
        stepProjectiles(sim, dt);
        stepInteractables(sim);

        // Cleanup transient visual effects & floating text
        sim.effects.forEach(function (fx) { fx.timer -= dt; });
        sim.effects = sim.effects.filter(function (fx) { return fx.timer > 0; });
        sim.floatingTexts.forEach(function (ft) { ft.timer -= dt; ft.y -= 28 * dt; });
        sim.floatingTexts = sim.floatingTexts.filter(function (ft) { return ft.timer > 0; });
        if (sim.fx) fxApi().update(sim.fx, dt);

        // Animation state selection
        p.animTime += dt;
        if (p.hurtTimer > 0) p.animState = 'hurt';
        else if (p.attackTimer > 0) p.animState = 'attack';
        else if (!p.onGround) p.animState = 'jump';
        else if (Math.abs(p.vx) > 150) p.animState = 'run';
        else if (Math.abs(p.vx) > 18) p.animState = 'walk';
        else p.animState = 'idle';
    }

    function stepSimulation(sim, dt, input) {
        if (!sim) return [];
        const cleanInput = input || {};
        sim.events = [];
        const safeDt = Math.min(0.1, Math.max(0, Number(dt) || 0));
        sim.accum += safeDt;
        let substeps = 0;
        let edgeInput = Object.assign({}, cleanInput);
        while (sim.accum >= STEP && substeps < 6) {
            stepOneTick(sim, edgeInput);
            edgeInput.jumpPressed = false;
            edgeInput.attackPressed = false;
            edgeInput.dashPressed = false;
            edgeInput.healPressed = false;
            sim.accum -= STEP;
            substeps += 1;
        }
        return sim.events;
    }

    const profileRef = profileApi();
    root.RunenpfadSim = {
        STEP: STEP,
        STORAGE_KEY: profileRef ? profileRef.STORAGE_KEY : 'aurago.runenpfad.v1',
        SAVE_VERSION: profileRef ? profileRef.SAVE_VERSION : 1,
        createDefaultProfile: profileRef ? profileRef.createDefaultProfile : null,
        importProfile: profileRef ? profileRef.importProfile : null,
        exportProfile: profileRef ? profileRef.exportProfile : null,
        upgradeCost: profileRef ? profileRef.upgradeCost : null,
        buyUpgrade: profileRef ? profileRef.buyUpgrade : null,
        totalStars: profileRef ? profileRef.totalStars : null,
        heroStats: profileRef ? profileRef.heroStats : null,
        createSimulation: createSimulation,
        stepSimulation: stepSimulation
    };
})(typeof window !== 'undefined' ? window : globalThis);
