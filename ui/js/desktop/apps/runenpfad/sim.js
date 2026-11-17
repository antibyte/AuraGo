/* Runenpfad pure 60 Hz 2D platformer simulation, boss hooks, collectibles and persistence re-export. DOM-free. */
(function (root) {
    'use strict';

    const STEP = 1 / 60;
    const GRAVITY = 1480;
    const MAX_FALL = 760;

    // Dependencies are read from the shared global scope at call time.
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

    function physicsApi() {
        return root.RunenpfadPhysics;
    }

    function combatApi() {
        return root.RunenpfadCombat;
    }

    function runesApi() {
        return root.RunenpfadRunes || null;
    }

    function pushEvent(sim, type, detail) { physicsApi().pushEvent(sim, type, detail); }
    function pushJump(sim) { physicsApi().pushJump(sim); }
    function fxBurst(sim, kind, x, y, count, opts) { physicsApi().fxBurst(sim, kind, x, y, count, opts); }
    function fxShake(sim, amount) { physicsApi().fxShake(sim, amount); }
    function fxFlash(sim, color, alpha, duration) { physicsApi().fxFlash(sim, color, alpha, duration); }
    function fxHitstop(sim, seconds) { physicsApi().fxHitstop(sim, seconds); }
    function rectOverlap(ax, ay, aw, ah, bx, by, bw, bh) { return physicsApi().rectOverlap(ax, ay, aw, ah, bx, by, bw, bh); }
    function hurtPlayer(sim, amount, sourceX, sound) { combatApi().hurtPlayer(sim, amount, sourceX, sound); }
    function damageEnemy(sim, enemy, dmg, hitX, pierce) { combatApi().damageEnemy(sim, enemy, dmg, hitX, pierce); }
    function breakCrate(sim, crate) { combatApi().breakCrate(sim, crate); }
    function performPlayerAttack(sim, input) { combatApi().performPlayerAttack(sim, input); }

    // Optional zone hooks: every zone module takes part when loaded.
    function zoneHook(name, a, b, c) {
        [root.RunenpfadZones, root.RunenpfadZonesExtra, root.RunenpfadSwing, root.RunenpfadPuzzles, root.RunenpfadDragons].forEach(function (z) {
            if (z && typeof z[name] === 'function') z[name](a, b, c);
        });
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
                climbKind: null,
                climbDir: 0,
                onPlatformRide: null,
                coyoteTimer: 0,
                jumpBuffer: 0,
                airJumpsLeft: stats.maxAirJumps,
                rune: null,
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
                swing: null,
                swingCool: 0,
                stepTimer: 0,
                animState: 'idle',
                animTime: 0
            },
            projectiles: [],
            effects: [],
            floatingTexts: [],
            events: [],
            blessing: varietyApi() ? varietyApi().createBlessing() : { runePower: 0, shrineCount: 0 },
            rngState: (((mode === 'endless' ? endlessFloor : levelId) * 7919) + 101) >>> 0,
            combo: { count: 0, timer: 0, mul: 1 },
            bossIntro: null,
            tempHazards: [],
            fx: fxApiRef ? fxApiRef.createSystem((mode === 'endless' ? endlessFloor : levelId) * 7919 + 13) : null
        };
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
        // Spike traps cycle on a timer: hidden, rising, then armed for a short window.
        stage.hazards.forEach(function (h, index) {
            if (h.kind !== 'spike_trap') return;
            if (!h.period) {
                h.period = 2.2;
                h.offset = index * 0.35;
            }
            h.phase01 = ((sim.elapsed + h.offset) % h.period) / h.period;
            const wasActive = !!h.active;
            h.active = h.phase01 >= 0.55 && h.phase01 <= 0.93;
            if (h.active && !wasActive && Math.abs(sim.player.x - (h.x + h.w / 2)) < 520) {
                pushEvent(sim, 'spike_rise', { x: h.x + h.w / 2, y: h.y });
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
                    pushEvent(sim, 'boulder_land', { x: b.x + b.w / 2, y: b.y + b.h });
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
    let enemyApiCache = null;
    function enemyApi() {
        if (!enemyApiCache) {
            const ph = physicsApi();
            enemyApiCache = {
                GRAVITY: GRAVITY,
                MAX_FALL: MAX_FALL,
                isSolidTile: ph.isSolidTile,
                resolveHorizontalTiles: ph.resolveHorizontalTiles,
                resolveVerticalTiles: ph.resolveVerticalTiles,
                rand: ph.rand,
                pushEvent: ph.pushEvent
            };
        }
        return enemyApiCache;
    }

    function enemyShot(sim, e) {
        const p = sim.player;
        pushEvent(sim, 'enemy_shot', { id: e.id, defId: e.defId, x: e.x, y: e.y });
        pushEvent(sim, 'enemy_attack', { id: e.id, defId: e.defId, kind: 'shot', x: e.x, y: e.y });
        e.attackTimer = 0.45;
        e.attackKind = 'shot';
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

    function stepEnemies(sim, dt, input) {
        const stage = sim.stage;
        const p = sim.player;
        const behaviors = behaviorsApi();
        const runes = runesApi();
        const jumpHeld = !!(input && (input.jump || input.up || input.jumpPressed));
        stage.enemies.forEach(function (e) {
            e.animTime += dt;
            if (e.dead) {
                if (e.deathTimer > 0) e.deathTimer -= dt;
                return;
            }
            if (e.hurtTimer > 0) e.hurtTimer -= dt;
            if (e.blockFlash > 0) e.blockFlash = Math.max(0, e.blockFlash - dt);
            if (e.stompCool > 0) e.stompCool -= dt;
            if (e.attackTimer > 0) e.attackTimer = Math.max(0, e.attackTimer - dt);
            const speedMul = e.phase === 2 ? 1.38 : 1.0;

            if (behaviors) {
                behaviors.update(sim, e, dt * (runes ? runes.enemySlow(sim, e) : 1), enemyApi());
            } else {
                e.x += e.vx * speedMul * dt;
            }

            if (e.def.ranged) {
                const burst = e.def.behavior === 'shooter';
                e.shootTimer -= dt * speedMul;
                if (e.shootTimer <= 0) {
                    e.shootTimer = e.def.boss ? 1.45 : 2.2;
                    if (Math.abs(p.x - e.x) < 420) {
                        enemyShot(sim, e);
                        if (burst) {
                            // Three-shot burst: the first round is out, two more follow 0.18 s apart.
                            e.burstLeft = e.burstLeft > 0 ? e.burstLeft - 1 : 2;
                            if (e.burstLeft > 0) e.shootTimer = 0.18;
                        }
                    } else if (burst) {
                        e.burstLeft = 0;
                    }
                }
            }

            e.animState = e.hurtTimer > 0 ? 'hurt' : (Math.abs(e.vx) < 1 ? 'idle' : 'move');
            const ew = 28 * e.def.scale;
            const eh = 38 * e.def.scale;
            if (!e.phased && rectOverlap(p.x - p.w / 2, p.y - p.h, p.w, p.h, e.x - ew / 2, e.y - eh, ew, eh)) {
                if (combatApi().stompEnemy(sim, e, eh, jumpHeld)) return;
                if (p.invulnTimer <= 0 && sim.state === 'playing') {
                    e.attackKind = 'lunge';
                    e.attackTimer = 0.45;
                    pushEvent(sim, 'enemy_attack', { id: e.id, defId: e.defId, kind: 'lunge', x: e.x, y: e.y });
                }
                hurtPlayer(sim, e.def.damage, e.x, e.def.hitSound);
            }
        });
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
            sim.newBest = !!prevBest && cleanElapsed < prevBest; // a first clear sets the time, it does not beat one
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

    // Hazard and boulder hits announce themselves only when the hit actually lands.
    function hazardHurt(sim, kind, srcX, srcY) {
        const p = sim.player;
        if (p.invulnTimer > 0 || sim.state !== 'playing') return;
        pushEvent(sim, 'hazard', { kind: kind, x: srcX, y: srcY });
        hurtPlayer(sim, 1, srcX, 'impact-stone');
    }

    // Falling out of the world is announced once, right before the lethal hit.
    function abyssCheck(sim, sound) {
        const p = sim.player;
        if (p.y <= sim.stage.height + 64) return;
        if (p.invulnTimer <= 0 && sim.state === 'playing') {
            pushEvent(sim, 'player_fall', { x: p.x, y: p.y });
        }
        hurtPlayer(sim, p.maxHp, p.x, sound);
    }

    function stepInteractables(sim) {
        const p = sim.player;
        const stage = sim.stage;
        const TILE = stage.tileSize;
        const px = p.x - p.w / 2;
        const py = p.y - p.h;
        const magnet = sim.stats.magnetRadius;
        const runes = runesApi();
        if (runes) runes.interact(sim);

        stage.collectibles.forEach(function (item) {
            if (item.collected) return;
            const dx = item.x - p.x;
            const dy = item.y - (p.y - 22);
            const dist = Math.hypot(dx, dy);
            if (runes && runes.collect(sim, item, dist)) return;
            if (item.kind === 'coin' && dist < magnet + 24 && dist > 1) {
                item.x -= (dx / dist) * 4.5;
                item.y -= (dy / dist) * 4.5;
            }
            if (dist < 30) {
                item.collected = true;
                if (item.kind === 'coin') {
                    const value = Math.max(1, Math.round(stage.coinValue || 1)) + (runes ? runes.coinBonus(sim) : 0);
                    sim.stageCoins += value;
                    sim.profile.coins += value;
                    sim.stageScore += 25 * value;
                    pushEvent(sim, 'pickup_coin', { x: item.x, y: item.y });
                    fxBurst(sim, 'ember', item.x, item.y, 3, { color: '#ffd65a', speed: 70, life: 0.4 });
                } else if (item.kind === 'gem') {
                    if (!sim.gemsCollected[item.gemIndex]) {
                        sim.gemsCollected[item.gemIndex] = true;
                        sim.gemCount += 1;
                    }
                    sim.stageScore += 100;
                    pushEvent(sim, 'pickup_gem', { gemIndex: item.gemIndex, x: item.x, y: item.y });
                    sim.floatingTexts.push({ x: item.x, y: item.y - 12, text: '+GEM', key: 'runenpfad.hud_gems', suffix: ' ' + sim.gemCount + '/3', color: '#7ee8fa', timer: 1.0 });
                    fxBurst(sim, 'rune', item.x, item.y, 12, { color: '#7ee8fa', speed: 165, life: 0.7 });
                    fxFlash(sim, '#7ee8fa', 0.16, 0.2);
                } else if (item.kind === 'star') {
                    sim.stageScore += 150;
                    pushEvent(sim, 'pickup_star', { x: item.x, y: item.y });
                    sim.floatingTexts.push({ x: item.x, y: item.y - 12, text: '+150', icon: 'star', color: '#ffe066', timer: 0.85 });
                    fxBurst(sim, 'rune', item.x, item.y, 10, { color: '#ffe066', speed: 150, life: 0.7 });
                } else if (item.kind === 'heart' || item.kind === 'apple') {
                    p.hp = Math.min(p.maxHp, p.hp + 1);
                    sim.stageScore += 50;
                    pushEvent(sim, 'pickup_heart', { x: item.x, y: item.y });
                    sim.floatingTexts.push({ x: item.x, y: item.y - 12, text: '+1', icon: 'heart', color: '#86efac', timer: 0.75 });
                    fxBurst(sim, 'ember', item.x, item.y, 6, { color: '#86efac', speed: 85, life: 0.55 });
                } else if (item.kind === 'potion') {
                    p.potions += 1;
                    pushEvent(sim, 'pickup_potion', { x: item.x, y: item.y });
                    sim.floatingTexts.push({ x: item.x, y: item.y - 12, text: '+1', icon: 'potion', color: '#c084fc', timer: 0.85 });
                    fxBurst(sim, 'rune', item.x, item.y, 8, { color: '#c084fc', speed: 120, life: 0.6 });
                } else if (item.kind === 'treasure') {
                    sim.stageCoins += 10;
                    sim.profile.coins += 10;
                    sim.stageScore += 250;
                    pushEvent(sim, 'pickup_treasure', { x: item.x, y: item.y });
                    sim.floatingTexts.push({ x: item.x, y: item.y - 12, text: '+10', icon: 'coin', color: '#ffd65a', timer: 0.9 });
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
                pushEvent(sim, 'checkpoint', { x: cp.x, y: cp.y });
                fxBurst(sim, 'ember', cp.x, cp.y + 8, 14, { color: '#ffb347', speed: 110, life: 0.9 });
                fxFlash(sim, '#ffb347', 0.14, 0.3);
            }
        });

        if (stage.key && !stage.key.collected && !stage.key.guardedByBoss) {
            if (Math.hypot(stage.key.x - p.x, stage.key.y - (p.y - 22)) < 34) {
                stage.key.collected = true;
                sim.hasKey = true;
                if (stage.door) {
                    stage.door.opened = true;
                    pushEvent(sim, 'door_open', { material: stage.worldIndex >= 3 ? 'metal' : 'wood', x: stage.door.x, y: stage.door.y });
                }
                sim.notice = 'runenpfad.notice.key_found';
                sim.stageScore += 200;
                pushEvent(sim, 'pickup_key', { x: stage.key.x, y: stage.key.y });
                sim.floatingTexts.push({ x: stage.key.x, y: stage.key.y - 16, text: 'KEY!', key: 'runenpfad.hud_key_on', color: '#ffe066', timer: 1.2 });
                fxBurst(sim, 'rune', stage.key.x, stage.key.y, 18, { color: '#ffe066', speed: 210, life: 0.9 });
                fxFlash(sim, '#ffe066', 0.24, 0.35);
                fxShake(sim, 3.5);
            }
        }

        stage.hazards.forEach(function (h) {
            const top = h.y + TILE - h.h + 4;
            if (h.kind === 'spike_trap' && !h.active) return;
            if (rectOverlap(px + 2, py + 4, p.w - 4, p.h - 6, h.x + 4, top, h.w - 8, h.h - 4)) {
                hazardHurt(sim, h.kind, h.x + h.w / 2, h.y);
            }
        });

        stage.boulders.forEach(function (b) {
            if (b.state !== 'falling') return;
            if (rectOverlap(px + 2, py + 4, p.w - 4, p.h - 6, b.x + 6, b.y + 6, b.w - 12, b.h - 12)) {
                hazardHurt(sim, 'boulder', b.x + b.w / 2, b.y);
            }
        });

        // Rune shrine: a short blessing that doubles the hero's damage.
        (stage.shrines || []).forEach(function (sh) {
            if (sh.taken) return;
            if (rectOverlap(px, py, p.w, p.h, sh.x - 24, sh.y - 22, 48, 48)) {
                sh.taken = true;
                sim.blessing.runePower = varietyApi() ? varietyApi().RUNE_POWER_DURATION : 12;
                sim.stageScore += 150;
                pushEvent(sim, 'pickup_shrine', { x: sh.x, y: sh.y });
                sim.floatingTexts.push({ x: sh.x, y: sh.y - 26, text: 'RUNE POWER', key: 'runenpfad.hud_rune', color: '#c084fc', timer: 1.1 });
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

        zoneHook('interact', sim);
    }

    // Potion heal is shared by the normal tick and the impact-freeze tick.
    function applyPotionHeal(sim) {
        const p = sim.player;
        if (p.potions <= 0 || p.hp >= p.maxHp) return;
        p.potions -= 1;
        p.hp = Math.min(p.maxHp, p.hp + 2);
        pushEvent(sim, 'heal', { x: p.x, y: p.y });
        sim.floatingTexts.push({ x: p.x, y: p.y - 44, text: '+2', icon: 'heart', color: '#c084fc', timer: 0.8 });
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
            abyssCheck(sim, 'impact-flesh');
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
        zoneHook('step', sim, dt);
        if (runesApi()) runesApi().tick(sim, dt);

        // Potion heal
        if (input.healPressed) applyPotionHeal(sim);

        // Swinging ropes own the hero's position while hanging.
        const swung = !!(root.RunenpfadSwing && root.RunenpfadSwing.update(sim, dt, input));

        // Dash roll (Ranger)
        if (!swung && stats.def.dashEnabled && input.dashPressed && p.dashCooldown <= 0 && p.dashTimer <= 0) {
            p.dashTimer = 0.18;
            p.dashCooldown = 0.72;
            p.invulnTimer = Math.max(p.invulnTimer, 0.24);
            p.vx = p.facing * 520;
            p.vy = 0;
            pushEvent(sim, 'dash', { x: p.x, y: p.y });
            fxBurst(sim, 'trail', p.x, p.y - 22, 8, {
                color: '#a5f3fc',
                speed: 70,
                arc: Math.PI,
                angle: p.facing > 0 ? Math.PI : 0,
                life: 0.3
            });
        }

        // Carry standing player on moving platforms
        if (!swung && p.onGround && p.onPlatformRide) {
            p.x += p.onPlatformRide.x - p.onPlatformRide.prevX;
        }

        // Check ladder overlap
        let nearLadder = false;
        let nearKind = null;
        let nearRope = null;
        let ladderBelow = false; // a ladder tile continues under the feet, so one-way platforms at the column are passable
        for (let i = 0; i < stage.ladders.length; i++) {
            const l = stage.ladders[i];
            if (Math.abs(p.x - (l.x + TILE / 2)) >= 20) continue;
            // Standing on the platform at the top of a ladder (feet on the top tile's edge) + Down grabs it.
            const atTopEdge = input.down && p.y > l.y - 3;
            if ((p.y > l.y || atTopEdge) && p.y - p.h < l.y + TILE) {
                nearLadder = true;
                nearKind = 'ladder';
                if (p.y + 2 >= l.y && p.y + 2 < l.y + TILE) ladderBelow = true;
            }
        }
        if (swung) {
            nearLadder = false;
        } else if (!nearLadder && stage.ropes) {
            for (let i = 0; i < stage.ropes.length; i++) {
                const r = stage.ropes[i];
                if (Math.abs(p.x - r.x) <= 14 && p.y > r.top && p.y - p.h < r.bottom) {
                    nearLadder = true;
                    nearKind = 'rope';
                    nearRope = r;
                    break;
                }
            }
        }
        if (nearLadder && (input.up || input.down)) {
            p.onLadder = true;
            p.slamActive = false;
        } else if (!nearLadder) {
            p.onLadder = false;
        }
        if (swung) p.onLadder = false;

        p.climbKind = p.onLadder ? (nearKind || p.climbKind) : null;
        p.climbDir = 0;
        const moveDir = (input.right ? 1 : 0) - (input.left ? 1 : 0);
        if (moveDir !== 0 && p.dashTimer <= 0) p.facing = moveDir;

        if (swung) {
            p.climbDir = 0;
        } else if (p.dashTimer > 0) {
            p.dashTimer -= dt;
        } else if (p.onLadder) {
            p.vx = moveDir * stats.speed * (p.climbKind === 'rope' ? 0.4 : 0.65);
            p.vy = input.up ? -190 : input.down ? 190 : 0;
            p.climbDir = input.up ? -1 : input.down ? 1 : 0;
            if (nearRope && moveDir === 0) p.x += (nearRope.x - p.x) * 0.3;
            if (input.jumpPressed) {
                p.onLadder = false;
                p.vy = -stats.jumpVel;
                pushJump(sim);
            }
        } else {
            const targetVx = moveDir * stats.speed * (runesApi() ? runesApi().speedMul(sim) : 1) + (stage.modEffects && stage.modEffects.wind ? stage.modEffects.wind : 0);
            const accel = p.onGround ? (stage.world.friction < 1 ? 650 : 2400) : 1400;
            const diff = targetVx - p.vx;
            const stepV = accel * dt;
            if (Math.abs(diff) <= stepV) p.vx = targetVx;
            else p.vx += Math.sign(diff) * stepV;

            if (p.onGround) {
                p.coyoteTimer = 0.11;
                p.airJumpsLeft = stats.maxAirJumps + (runesApi() ? runesApi().extraAirJumps(sim) : 0);
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

        zoneHook('applyForces', sim, p, dt);
        const prevY = p.y;
        const wasOnGround = p.onGround;
        let landResult;
        if (!swung) {
            p.x += p.vx * dt;
            physicsApi().resolveHorizontalTiles(stage, p, p.w, p.h);
            p.y += p.vy * dt;
            landResult = physicsApi().resolveVerticalTiles(stage, p, p.w, p.h, prevY, !!(input.down && !p.slamActive && (!nearLadder || ladderBelow)));
        }
        if (landResult === 'spring') {
            pushJump(sim);
            pushEvent(sim, 'spring', { x: p.x, y: p.y });
            fxBurst(sim, 'rune', p.x, p.y, 10, { color: '#c4b5fd', speed: 190, life: 0.55 });
            fxShake(sim, 2.6);
        }

        if (!wasOnGround && p.onGround) {
            pushEvent(sim, 'land', { x: p.x, y: p.y });
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
                pushEvent(sim, 'slam_land', { x: p.x, y: p.y });
                fxBurst(sim, 'shard', p.x, p.y - 8, 18, { color: '#ffd08a', speed: 300, life: 0.7 });
                fxBurst(sim, 'dust', p.x, p.y, 16, { color: '#e8d8b8', speed: 230, arc: Math.PI, angle: Math.PI / 2, life: 0.5 });
                if (sim.fx) {
                    physicsApi().ring(sim, p.x, p.y - 4, { color: '#ffd08a', size: 16, grow: 320, life: 0.42 });
                }
                fxShake(sim, 8);
                fxHitstop(sim, 0.06);
                fxFlash(sim, '#ffe9b0', 0.18, 0.18);
                zoneHook('slamLand', sim, p.x, p.y);
                stage.enemies.forEach(function (e) {
                    if (!e.dead && Math.hypot(e.x - p.x, e.y - p.y) < 120) {
                        const slamMul = varietyApi() ? varietyApi().damageMultiplier(sim.blessing) : 1;
                        damageEnemy(sim, e, (stats.damage + 1) * slamMul, p.x, true);
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
        abyssCheck(sim);

        // Footstep SFX pacing
        if (p.onGround && Math.abs(p.vx) > 40) {
            p.stepTimer -= dt;
            if (p.stepTimer <= 0) {
                p.stepTimer = 0.28;
                pushEvent(sim, 'step', { sound: stage.world.stepSound });
            }
        }

        stepEnemies(sim, dt, input);
        combatApi().stepProjectiles(sim, dt);
        combatApi().tick(sim, dt);
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
        else if (p.swing) p.animState = 'swing';
        else if (p.onLadder) p.animState = 'climb';
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
