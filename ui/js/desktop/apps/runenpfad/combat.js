/* Runenpfad combat: player/enemy damage, crates, hero attacks, projectiles and the combo tick. DOM-free. */
(function (root) {
    'use strict';

    // Dependencies are read from the shared global scope at call time (load order: physics before combat).
    function P() {
        return root.RunenpfadPhysics;
    }

    function varietyApi() {
        return root.RunenpfadVariety || null;
    }

    function pushEvent(sim, type, detail) { P().pushEvent(sim, type, detail); }
    function fxBurst(sim, kind, x, y, count, opts) { P().fxBurst(sim, kind, x, y, count, opts); }
    function fxShake(sim, amount) { P().fxShake(sim, amount); }
    function fxFlash(sim, color, alpha, duration) { P().fxFlash(sim, color, alpha, duration); }
    function fxHitstop(sim, seconds) { P().fxHitstop(sim, seconds); }
    function rectOverlap(ax, ay, aw, ah, bx, by, bw, bh) { return P().rectOverlap(ax, ay, aw, ah, bx, by, bw, bh); }
    function isSolidTile(stage, tx, ty) { return P().isSolidTile(stage, tx, ty); }

    function zonesApi() {
        return root.RunenpfadZones || null;
    }

    function runesApi() {
        return root.RunenpfadRunes || null;
    }

    // Kills chain into a combo: every kill inside the 2.5 s window raises the score multiplier.
    function registerKill(sim, enemy) {
        const c = sim.combo;
        if (!c) return 1;
        c.count += 1;
        c.timer = 2.5;
        const mul = 1 + 0.5 * Math.min(4, Math.floor(c.count / 3));
        const rose = mul > c.mul;
        c.mul = mul;
        if (c.count >= 2) pushEvent(sim, 'combo', { count: c.count, mul: mul, x: enemy.x, y: enemy.y });
        if (rose) sim.floatingTexts.push({ x: enemy.x, y: enemy.y - 70, text: 'x' + mul, color: '#fde68a', timer: 0.9 });
        return mul;
    }

    function hurtPlayer(sim, amount, sourceX, sound) {
        const p = sim.player;
        if (p.invulnTimer > 0 || sim.state !== 'playing') return;
        const runes = runesApi();
        if (runes && amount < p.maxHp && runes.absorb(sim)) return;
        p.hp = Math.max(0, p.hp - amount);
        if (p.swing) {
            p.swing = null;
            p.swingCool = 0.3;
        }
        sim.damageTaken += amount;
        p.invulnTimer = 1.1;
        p.hurtTimer = 0.28;
        p.slamActive = false;
        p.vy = -300;
        p.vx = p.x >= sourceX ? 180 : -180;
        pushEvent(sim, 'player_hurt', { hp: p.hp, sound: sound || 'impact-flesh', x: p.x, y: p.y });
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

    // Scarabs block melee and bolts from the side they face; pierce (stomp, slam, blasts, zaps) ignores the shield.
    function damageEnemy(sim, enemy, dmg, hitX, pierce) {
        if (enemy.dead || enemy.phased) return;
        if (!pierce && enemy.def.behavior === 'shielded' && (hitX - enemy.x) * (enemy.facing || 1) > 0) {
            enemy.blockFlash = 0.25;
            pushEvent(sim, 'enemy_block', { id: enemy.id, defId: enemy.defId, x: enemy.x, y: enemy.y });
            sim.floatingTexts.push({ x: enemy.x, y: enemy.y - 46, text: 'BLOCK', color: '#cfe8ff', timer: 0.5 });
            fxBurst(sim, 'spark', enemy.x + (hitX < enemy.x ? -18 : 18), enemy.y - 24, 6, { color: '#cfe8ff', speed: 170, arc: Math.PI * 2, life: 0.26 });
            return;
        }
        enemy.hp = Math.max(0, enemy.hp - dmg);
        enemy.hurtTimer = 0.22;
        if (!enemy.def.boss && enemy.vx !== undefined) {
            enemy.vx = (enemy.x >= hitX ? 1 : -1) * Math.max(70, enemy.def.speed);
        }
        sim.floatingTexts.push({ x: enemy.x, y: enemy.y - 46, text: String(dmg), color: '#ffe066', timer: 0.6 });
        pushEvent(sim, 'enemy_hit', { defId: enemy.defId, sound: enemy.def.hitSound, x: enemy.x, y: enemy.y });
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
            sim.stageScore += Math.round((enemy.score || enemy.def.score) * registerKill(sim, enemy));
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
            pushEvent(sim, 'enemy_kill', { id: enemy.defId, defId: enemy.defId, boss: !!enemy.def.boss, x: enemy.x, y: enemy.y });
            const runes = runesApi();
            if (runes) runes.onKill(sim, enemy);
            if (enemy.def.boss) {
                fxFlash(sim, '#fff3c4', 0.4, 0.4);
                sim.notice = 'runenpfad.notice.boss_defeated';
                pushEvent(sim, 'boss_defeat', { id: enemy.id, defId: enemy.defId, x: enemy.x, y: enemy.y });
                if (sim.stage.key && sim.stage.key.guardedByBoss && !sim.stage.key.collected) {
                    sim.stage.key.guardedByBoss = false;
                    sim.stage.key.x = enemy.x;
                    sim.stage.key.y = sim.stage.groundY - 24;
                }
                const Bosses = root.RunenpfadBosses;
                if (Bosses && typeof Bosses.onDefeat === 'function') Bosses.onDefeat(sim, enemy);
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
                sim.stage.enemies.forEach(function (o) {
                    if (!o.dead && o !== enemy && Math.hypot(o.x - enemy.x, o.y - enemy.y) < 96) {
                        damageEnemy(sim, o, sim.stats.damage + 2, enemy.x, true);
                    }
                });
                const p = sim.player;
                if (Math.hypot(p.x - enemy.x, p.y - enemy.y) < 96) {
                    hurtPlayer(sim, 1, enemy.x, 'impact-metal');
                }
            }
        } else if (enemy.def.boss && enemy.phase !== 2 && enemy.hp <= enemy.maxHp * 0.5) {
            enemy.phase = 2;
            pushEvent(sim, 'boss_phase', { id: enemy.id, defId: enemy.defId, x: enemy.x, y: enemy.y });
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
            const runes = runesApi();
            if (runes) runes.maybeDrop(sim, crate.x, crate.y - 8);
        }
    }

    function performPlayerAttack(sim, input) {
        const p = sim.player;
        const stage = sim.stage;
        const stats = sim.stats;
        const variety = varietyApi();
        const blessingMul = variety ? variety.damageMultiplier(sim.blessing) : 1;
        const runes = runesApi();
        const attackDamage = stats.damage * blessingMul + (runes ? runes.damageBonus(sim) : 0);
        if (p.attackCooldown > 0) return;

        // Mid-air knight ground slam
        if (stats.def.slamEnabled && input.down && !p.onGround && !p.onLadder) {
            p.slamActive = true;
            p.vy = Math.max(p.vy, 660);
            p.attackCooldown = stats.def.attackCooldown;
            p.attackTimer = stats.def.attackDuration;
            p.animState = 'attack';
            p.animTime = 0;
            pushEvent(sim, 'slam', { x: p.x, y: p.y });
            pushEvent(sim, 'attack_melee', { x: p.x, y: p.y });
            return;
        }

        p.attackCooldown = stats.def.attackCooldown;
        p.attackTimer = stats.def.attackDuration;
        p.animState = 'attack';
        p.animTime = 0;
        if (runes && runes.active(sim, 'fire')) fxBurst(sim, 'ember', p.x + p.facing * 34, p.y - 26, 6, { color: '#fb923c', speed: 90, life: 0.5 });

        if (sim.heroId === 'knight') {
            pushEvent(sim, 'attack_melee', { x: p.x, y: p.y });
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
            const zones = zonesApi();
            if (zones && typeof zones.hitRect === 'function') zones.hitRect(sim, hx, hy, hw, hh, 1);
            // Knight slash deflects enemy projectiles (boss missiles and shock waves are too heavy)
            sim.projectiles.forEach(function (proj) {
                if (proj.fromEnemy && !proj.dead && !proj.noDeflect && rectOverlap(hx, hy, hw, hh, proj.x - 8, proj.y - 8, 16, 16)) {
                    proj.fromEnemy = false;
                    proj.vx = -proj.vx * 1.3;
                    proj.damage = attackDamage;
                    proj.color = '#7ee8fa';
                }
            });
        } else {
            pushEvent(sim, 'attack_ranged', { x: p.x, y: p.y });
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
                damage: attackDamage,
                life: 1.15,
                color: '#7ee8fa',
                dead: false
            });
        }
    }

    // Missile impact: a small blast that hurts the hero when close.
    function projectileBlast(sim, proj) {
        const p = sim.player;
        const radius = proj.blast;
        pushEvent(sim, proj.kind === 'fireball' ? 'fireball_hit' : 'bomb_blast', { x: proj.x, y: proj.y });
        sim.effects.push({ animId: 'explosion', x: proj.x, y: proj.y - 8, timer: 0.5 });
        fxBurst(sim, 'shard', proj.x, proj.y, 10, { color: '#fb923c', speed: 230, life: 0.6 });
        fxShake(sim, 4);
        if (Math.hypot(p.x - proj.x, (p.y - 22) - proj.y) < radius) hurtPlayer(sim, proj.damage, proj.x, 'impact-metal');
    }

    // A spore bursts into a poison cloud (64x44, 1.6 s) whose bottom edge rests on bottomY.
    function sporeBurst(sim, x, bottomY) {
        sim.tempHazards.push({ kind: 'poison', x: x - 32, y: bottomY - 44, w: 64, h: 44, t: 1.6, dur: 1.6, damage: 1 });
        pushEvent(sim, 'spore_burst', { defId: 'sporecap', x: x, y: bottomY });
        fxBurst(sim, 'dust', x, bottomY - 10, 10, { color: '#a3e635', speed: 80, arc: Math.PI, angle: -Math.PI / 2, life: 0.6 });
    }

    function stepProjectiles(sim, dt) {
        const stage = sim.stage;
        const p = sim.player;
        const TILE = stage.tileSize;
        const zones = zonesApi();
        sim.projectiles.forEach(function (proj) {
            if (proj.dead) return;
            if (proj.gravity) proj.vy += proj.gravity * dt;
            proj.x += proj.vx * dt;
            proj.y += proj.vy * dt;
            proj.life -= dt;
            if (proj.life <= 0) {
                proj.dead = true;
                return;
            }
            const r = proj.radius || 6;
            if (!proj.fromEnemy && zones && typeof zones.hitRect === 'function' && zones.hitRect(sim, proj.x - r, proj.y - r, r * 2, r * 2, 1)) {
                proj.dead = true;
                return;
            }
            const tx = Math.floor(proj.x / TILE);
            const ty = Math.floor(proj.y / TILE);
            if (isSolidTile(stage, tx, ty)) {
                proj.dead = true;
                if (proj.blast) projectileBlast(sim, proj);
                if (proj.spore) sporeBurst(sim, proj.x, proj.vy > 0 ? ty * TILE : proj.y + 20);
                return;
            }
            // Shock waves hug the floor and fade out where the ground ends.
            if (proj.groundHug && !isSolidTile(stage, tx, Math.floor((proj.y + r + 6) / TILE))) {
                proj.dead = true;
                return;
            }
            if (proj.fromEnemy) {
                if (rectOverlap(p.x - p.w / 2, p.y - p.h, p.w, p.h, proj.x - r, proj.y - r, r * 2, r * 2)) {
                    proj.dead = true;
                    if (proj.blast) projectileBlast(sim, proj);
                    else if (proj.spore) sporeBurst(sim, proj.x, p.y);
                    else hurtPlayer(sim, proj.damage, proj.x, 'impact-metal');
                }
            } else {
                stage.enemies.forEach(function (e) {
                    if (proj.dead || e.dead || e.phased) return;
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
        if (sim.projectiles.length > 64) {
            sim.projectiles = sim.projectiles.filter(function (pr) { return !pr.dead; });
        }
    }

    // Falling onto an enemy's top: 1 damage (+1 fire, +1 knight slam), a bounce, no contact damage. Returns true when it stomped.
    // Bosses take the damage at most once per 0.6 s; inside that window the bounce still happens.
    function stompEnemy(sim, e, eh, jumpHeld) {
        const p = sim.player;
        if (e.phased || e.dead || p.vy <= 120 || sim.state !== 'playing') return false;
        const top = e.y - eh;
        if (p.y < top || p.y > top + eh * 0.45) return false;
        if (!e.def.boss || !(e.stompCool > 0)) {
            const runes = runesApi();
            const dmg = 1 + (runes ? runes.damageBonus(sim) : 0) + (p.slamActive ? 1 : 0);
            if (e.def.boss) e.stompCool = 0.6;
            damageEnemy(sim, e, dmg, p.x, true);
        }
        p.vy = jumpHeld ? -600 : -470;
        p.onGround = false;
        p.slamActive = false;
        pushEvent(sim, 'stomp', { x: p.x, y: p.y, enemyId: e.id });
        fxBurst(sim, 'spark', p.x, p.y, 8, { color: '#ffe066', speed: 170, arc: Math.PI, angle: -Math.PI / 2, life: 0.3 });
        return true;
    }

    // Timed hazards (boss lasers, fire strips) burn the hero while they are active.
    function stepTempHazards(sim, dt) {
        const list = sim.tempHazards;
        if (!list || !list.length) return;
        const p = sim.player;
        list.forEach(function (th) {
            th.t -= dt;
            if (th.t <= 0 || p.invulnTimer > 0 || sim.state !== 'playing') return;
            if (rectOverlap(p.x - p.w / 2 + 2, p.y - p.h + 4, p.w - 4, p.h - 6, th.x, th.y, th.w, th.h)) {
                pushEvent(sim, 'hazard', { kind: th.kind, x: th.x + th.w / 2, y: th.y + th.h / 2 });
                hurtPlayer(sim, th.damage || 1, th.x + th.w / 2, 'impact-metal');
            }
        });
        sim.tempHazards = list.filter(function (th) { return th.t > 0; });
    }

    // Per-tick combat bookkeeping: combo window, boss intro banner and timed hazards.
    function tick(sim, dt) {
        const c = sim.combo;
        if (c && c.timer > 0) {
            c.timer = Math.max(0, c.timer - dt);
            if (c.timer === 0) {
                c.count = 0;
                c.mul = 1;
            }
        }
        if (sim.bossIntro) {
            sim.bossIntro.timer -= dt;
            if (sim.bossIntro.timer <= 0) sim.bossIntro = null;
        }
        stepTempHazards(sim, dt);
    }

    root.RunenpfadCombat = {
        hurtPlayer: hurtPlayer,
        damageEnemy: damageEnemy,
        stompEnemy: stompEnemy,
        breakCrate: breakCrate,
        performPlayerAttack: performPlayerAttack,
        stepProjectiles: stepProjectiles,
        tick: tick
    };
})(typeof window !== 'undefined' ? window : globalThis);
