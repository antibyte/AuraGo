(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createGame = function (ctx) {
        function resize() {
            if (!ctx.wrapEl) return;
            ctx.scale = Math.max(1, Math.min(Math.floor(ctx.wrapEl.clientWidth / ctx.W) || 1, Math.floor(ctx.wrapEl.clientHeight / ctx.H) || 1));
            ctx.canvas.width = ctx.W * ctx.scale; ctx.canvas.height = ctx.H * ctx.scale;
            ctx.canvas.style.width = (ctx.W * ctx.scale) + 'px'; ctx.canvas.style.height = (ctx.H * ctx.scale) + 'px';
            ctx.c.imageSmoothingEnabled = false;
        }

        function isChal(s) { return ctx.settings.mode !== 'endless' && s >= 3 && (s - 3) % 4 === 0; }

        function isMiniBossStage() { return ctx.G.stage >= 5 && ctx.G.stage % 5 === 0; }

        function dailySeed() {
            const d = new Date();
            return d.getFullYear() * 10000 + (d.getMonth() + 1) * 100 + d.getDate();
        }

        function seededRandom(seed) {
            let s = seed;
            return function() { s = (s * 16807 + 0) % 2147483647; return (s - 1) / 2147483646; };
        }

        function startDailyChallenge() {
            const seed = dailySeed();
            const rng = seededRandom(seed);
            ctx.G.dailySeed = seed;
            ctx.G.score = 0; ctx.G.lives = ctx.diffMod('lives'); ctx.G.stage = 1;
            ctx.G.p.dual = false; ctx.G.p.cap = null; ctx.G.weaponLv = 1; ctx.G.killCount = 0;
            ctx.G.displayScore = 0; ctx.G.deathParts = []; ctx.G.collectedPU = new Set();
            ctx.G.perfectCount = 0; ctx.G.chal = true;
            const mods = ctx.STAGE_MODIFIERS;
            const mod1 = mods[Math.floor(rng() * mods.length)];
            let mod2 = mods[Math.floor(rng() * mods.length)];
            while (mod2.id === mod1.id && mods.length > 1) mod2 = mods[Math.floor(rng() * mods.length)];
            mod1.apply(ctx.G); mod2.apply(ctx.G);
            ctx.G.stageModifier = [mod1, mod2];
            ctx.settings.mode = 'daily';
            ctx.startStage();
            ctx.MusicEngine.play('challenge');
            ctx.G.scorePopups.push({ x: ctx.W / 2, y: ctx.H / 2 - 40, text: 'DAILY CHALLENGE', t: 0, dur: 2000, col: '#ff88ff', big: true });
        }

        function checkDailyStreak() {
            const today = dailySeed();
            const lastDaily = parseInt(localStorage.getItem('galaxa_last_daily') || '0');
            const streak = ctx.G.dailyStreak;
            if (lastDaily === today) return;
            const yesterday = dailySeed() - 1;
            if (lastDaily === yesterday || lastDaily === today - 1) {
                ctx.G.dailyStreak = streak + 1;
            } else if (lastDaily < today - 1) {
                ctx.G.dailyStreak = 1;
            }
            try {
                localStorage.setItem('galaxa_daily_streak', String(ctx.G.dailyStreak));
                localStorage.setItem('galaxa_last_daily', String(today));
            } catch (e) {}
            if (ctx.G.dailyStreak >= 7) ctx.unlockAchievement('daily_warrior');
            if (ctx.G.dailyStreak >= 3) { ctx.G.credits += 50; try { localStorage.setItem('galaxa_credits', String(ctx.G.credits)); } catch(e2) {} }
            if (ctx.G.dailyStreak >= 7) { ctx.G.credits += 100; try { localStorage.setItem('galaxa_credits', String(ctx.G.credits)); } catch(e3) {} }
        }

        function advanceToNextStage(fromSkip) {
            if (ctx.G.stageClearLock > 0 || ctx.G.st === 'STAGE_INTRO' || ctx.G.st === 'GAME_OVER') return;
            ctx.G.stageClearLock = 600;
            ctx.G.stageEmptyT = 0;
            ctx.G.transitionType = Math.floor(Math.random() * 4);
            if (ctx.G.transitionType === 0) { ctx.G.warpT = 1500; ctx.G.warpFlash = 50; }
            else if (ctx.G.transitionType === 1) { ctx.G.swipeT = 1200; ctx.G.swipeDir = Math.random() > 0.5 ? 1 : -1; }
            else if (ctx.G.transitionType === 2) { ctx.G.portalT = 1400; ctx.G.portalR = 0; }
            else { ctx.G.glitchT = 1000; ctx.G.glitchStrips = []; for (let _gi = 0; _gi < 12; _gi++) ctx.G.glitchStrips.push({ y: _gi * (ctx.H / 12), offset: 0, targetOffset: (Math.random() - 0.5) * 60 }); }
            ctx.G.stage++;
            if (ctx.G.stage >= 10) ctx.unlockAchievement('survivor');
            if (ctx.G.stage >= 20) ctx.unlockAchievement('legend');
            if (ctx.G.score >= 1000000) ctx.unlockAchievement('millionaire');
            const stageTime = (performance.now ? performance.now() : Date.now()) - ctx.G.stageStartTime;
            if (stageTime < 30000 && ctx.G.stage > 2) ctx.unlockAchievement('speed_demon');
            // Intensity Director
            const _accuracy = (ctx.G.stageAccuracyShots || 0) > 0 ? (ctx.G.stageAccuracyHits || 0) / ctx.G.stageAccuracyShots : 0.5;
            const _killSpeed = (ctx.G.stageKills || 0) / Math.max(1, stageTime / 1000);
            let _intAdj = 0;
            if (_accuracy > 0.7) _intAdj--; if (_accuracy < 0.3) _intAdj++;
            if (_killSpeed > 3) _intAdj--; if (_killSpeed < 1) _intAdj++;
            if ((ctx.G.stageDamageTaken || 0) === 0) _intAdj--; if ((ctx.G.stageDamageTaken || 0) >= 3) _intAdj++;
            ctx.G.intensityScore = Math.max(0, Math.min(10, (ctx.G.intensityScore || 5) + _intAdj));
            ctx.MusicEngine.setIntensity(ctx.G.intensityScore);
            ctx.SFX.warpJump();
            if (!ctx.G.chal && !fromSkip) {
                ctx.MusicEngine.play('victory');
                setTimeout(() => { if (!ctx.state.disposed && ctx.MusicEngine.playing === 'victory') ctx.MusicEngine.play('gameplay'); }, 3500);
            }
            if (ctx.G.stage % 3 === 0 && !fromSkip && ctx.openShop) {
                ctx.openShop();
            } else {
                ctx.startStage();
            }
        }

        function startStage() {
            ctx.G.enemies = [];
            ctx.G.chal = ctx.isChal(ctx.G.stage);
            ctx.G.stageWipeT = 400;
            ctx.G.st = 'STAGE_INTRO';
            ctx.G.introTmr = 1200;
            ctx.G.stageStartTime = performance.now ? performance.now() : Date.now();
            // NEW: Biome progression + reveal cinematic
            const _biome = ctx.getBiomeForStage ? ctx.getBiomeForStage(ctx.G.stage) : null;
            if (_biome) {
                const prevBiomeId = ctx.G.biome;
                ctx.G.biome = _biome.id; ctx.G.biomeName = _biome.name;
                if (prevBiomeId !== _biome.id && ctx.G.stage > 1) { ctx.G.biomeRevealT = 2200; if (ctx.SFX.biomeReveal) ctx.SFX.biomeReveal(); ctx.duckMusic(0.3, 600); }
            }
            // NEW: Bonus sub-stage scheduling (every BONUS_STAGE_EVERY stages, no death penalty)
            ctx.G.bonusStage = (ctx.G.stage > 1) && (ctx.G.stage % ctx.BONUS_STAGE_EVERY === 0) && !ctx.G.chal && !ctx.isMiniBossStage();
            if (ctx.G.bonusStage) { ctx.G.bonusStageT = ctx.BONUS_STAGE_DURATION; if (ctx.SFX.bonusStart) ctx.SFX.bonusStart(); }
            ctx.G.bul = []; ctx.G.ebul = []; ctx.G.exp = []; ctx.G.part = []; ctx.G.pendingBooms = []; ctx.G.levelSkipTimer = 0;
            ctx.G.beam = null; ctx.G.powerups = []; ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.G.shieldHits = 0;
            ctx.G.scorePopups = []; ctx.G.warpT = 0; ctx.G.warpFlash = 0; ctx.G.perfectT = 0;
            ctx.G.combo = 0; ctx.G.comboTimer = 0; ctx.G.comboMult = 1; ctx.G.comboBanner = null;
            ctx.G.trails = []; ctx.G.timeScale = 1; ctx.G.timeSlowTimer = 0; ctx.G.freezeT = 0; ctx.G.damageVignetteT = 0;
            ctx.G.bossWarningT = 0; ctx.G.bossWarningShown = false;
            ctx.G.weaponLv = Math.max(1, ctx.G.weaponLv); ctx.G.puUpgrade = null; ctx.G.upgradeBanner = null; ctx.G.killCount = 0; ctx.G.slowMoT = 0;
            ctx.G.swipeT = 0; ctx.G.portalT = 0; ctx.G.glitchT = 0; ctx.G.glitchStrips = [];
            ctx.G._closeCallCooldown = 0; ctx.G._synergyChecked = null; ctx.G.shieldReflect = false; ctx.G.laserSlow = false; ctx.G.droneRicochet = false;
            ctx.G.scoreMult = 1; ctx.G.glassCannon = false; ctx.G.bulletStorm = false; ctx.G.powerSurge = false; ctx.G.darkness = false; ctx.G.turbo = false;
            ctx.G.mirrorField = false; ctx.G.gravityWell = false; ctx.G.phasing = false; ctx.G.ricochetWorld = false;
            ctx.G.orbitalShields = null; ctx.G.orbitalShieldTimer = 0;
            ctx.G.stageKills = 0; ctx.G.stageDamageTaken = 0; ctx.G.stageAccuracyShots = 0; ctx.G.stageAccuracyHits = 0;
            ctx.G.pacifistStage = true; ctx.G.overcharge = 0; ctx.G.overchargeTimer = 0;
            ctx.G.p.x = ctx.W / 2; ctx.G.p.y = ctx.H - 50; ctx.G.p.alive = true; ctx.G.p.inv = 2000; ctx.G.p.cap = null; ctx.G.p.dual = false; ctx.G.p.reviveTimer = 0;
            ctx.G.stageEmptyT = 0;
            ctx.setPUClass(null);
            ctx.G.chal ? ctx.SFX.challenge() : ctx.SFX.stageClear();
            ctx.MusicEngine.setTempo(1 + ctx.G.stage * 0.05);
            ctx.MusicEngine.play(ctx.G.chal ? 'challenge' : 'gameplay');
            ctx.mkFormation();
            if (ctx.spawnHazards) ctx.spawnHazards();
            if (ctx.relic_applyRelics) ctx.relic_applyRelics(ctx.G);
            // Mutation start notification
            if (ctx.G.stageModifier) { for (const _m of (Array.isArray(ctx.G.stageModifier) ? ctx.G.stageModifier : [ctx.G.stageModifier])) { if (_m && (_m.id === 'mirror_field' || _m.id === 'gravity_well' || _m.id === 'phasing' || _m.id === 'ricochet_world')) { ctx.SFX.mutationStart(); ctx.G.mutationStages = (ctx.G.mutationStages || 0) + 1; if (ctx.G.mutationStages >= 5) ctx.unlockAchievement('mutation_master'); } } }
        }

        function updateExp(dt) {
            const dtMs = dt * 1000;
            let elen = 0;
            for (let i = 0; i < ctx.G.exp.length; i++) { const ex = ctx.G.exp[i]; ex.t += dtMs; if (ex.t < ex.dur) ctx.G.exp[elen++] = ex; }
            ctx.G.exp.length = elen;
            const _partCap = ctx.settings.particles === 'low' ? 60 : ctx.settings.particles === 'medium' ? 100 : 150;
            if (ctx.G.part.length > _partCap) ctx.G.part.length = _partCap;
            let plen = 0;
            for (let i = 0; i < ctx.G.part.length; i++) {
                const p = ctx.G.part[i];
                p.x += p.vx * dt; p.y += p.vy * dt;
                if (ctx.G.bgTheme === 'blackhole') { const _bhDx = ctx.W/2 - p.x, _bhDy = ctx.H/3 - p.y, _bhDist = Math.hypot(_bhDx, _bhDy); if (_bhDist > 10 && _bhDist < 150) { const _bhF = 60 / _bhDist; p.vx += (_bhDx / _bhDist) * _bhF * dt; p.vy += (_bhDy / _bhDist) * _bhF * dt; } }
                if (p.debris) { p.vy += 60 * dt; p.vx *= 0.98; p.rot += dt * 3; }
                else if (p.smoke) { p.vy -= 8 * dt; p.size += dt * 2; }
                else if (p.bloom) { const _bp = p.t / p.life; if (_bp > 0.5) { p.vx *= 1 + dt * 3; p.vy *= 1 + dt * 3; } else { p.vx *= 1 - dt * 2; p.vy *= 1 - dt * 2; } }
                else if (p.spark) { p.vx *= 0.95; p.vy *= 0.95; }
                p.t += dtMs; if (p.t < p.life) ctx.G.part[plen++] = p;
            }
            if (ctx.G.part.length > plen && ctx.recycleParticles) {
                const dead = ctx.G.part.splice(plen);
                ctx.recycleParticles(dead);
            } else {
                ctx.G.part.length = plen;
            }
            let tlen = 0;
            for (let i = 0; i < ctx.G.trails.length; i++) { const tr = ctx.G.trails[i]; tr.x += tr.vx * dt; tr.y += tr.vy * dt; tr.t += dtMs; if (tr.t < tr.life) ctx.G.trails[tlen++] = tr; }
            ctx.G.trails.length = tlen;
            let slen = 0;
            for (let i = 0; i < ctx.G.scorePopups.length; i++) { const sp = ctx.G.scorePopups[i]; sp.y -= 40 * dt; sp.t += dtMs; if (sp.t < sp.dur) ctx.G.scorePopups[slen++] = sp; }
            ctx.G.scorePopups.length = slen;
            if (ctx.G.flashT > 0) ctx.G.flashT -= dtMs;
            // NEW: Hitstop countdown — freezes gameplay timeScale briefly for impact weight
            if (ctx.G.hitstopT > 0) { ctx.G.hitstopT -= dtMs; if (ctx.G.hitstopT <= 0) { ctx.G.hitstopT = 0; ctx.G.timeScale = 1; } else ctx.G.timeScale = 0.001; }
            // NEW: Biome reveal timer + bonus sub-stage timer
            if (ctx.G.biomeRevealT > 0) ctx.G.biomeRevealT -= dtMs;
            if (ctx.G.bonusStage && ctx.G.bonusStageT > 0) { ctx.G.bonusStageT -= dtMs; if (ctx.G.bonusStageT <= 0) ctx.G.bonusStageT = 0; }
            // NEW: Super timer countdown
            if (ctx.G.superActive > 0) { ctx.G.superActive -= dtMs; if (ctx.G.superActive <= 0) { ctx.G.superActive = 0; ctx.G.superType = null; ctx.G.superTimer = 0; } }
            if (ctx.G.superCooldown > 0) ctx.G.superCooldown -= dtMs;
            // NEW: Parry timers
            if (ctx.G.parryActive > 0) { ctx.G.parryActive -= dtMs; if (ctx.G.parryActive <= 0) { ctx.G.parryActive = 0; ctx.G.parryCooldown = ctx.PARRY_COOLDOWN; } }
            if (ctx.G.parryCooldown > 0) ctx.G.parryCooldown -= dtMs;
            if (ctx.G.parrySuccessFlash > 0) ctx.G.parrySuccessFlash -= dtMs;
            // NEW: Combat text float update
            let _ctlen = 0; for (let _ci = 0; _ci < ctx.G.combatText.length; _ci++) { const _ct = ctx.G.combatText[_ci]; _ct.y -= 30 * dt; _ct.t += dtMs; if (_ct.t < _ct.dur) ctx.G.combatText[_ctlen++] = _ct; } ctx.G.combatText.length = _ctlen;
            if (ctx.G.stageWipeT > 0) ctx.G.stageWipeT -= dtMs;
            if (ctx.G.damageVignetteT > 0) ctx.G.damageVignetteT -= dtMs;
            if (ctx.G.freezeT > 0) { ctx.G.freezeT -= dtMs; ctx.wrapEl.classList.add('galaxa-freeze'); if (ctx.G.freezeT <= 0) { ctx.G.freezeT = 0; ctx.wrapEl.classList.remove('galaxa-freeze'); if (ctx.G.activePU && ctx.G.activePU.type === 'freeze') { ctx.G.activePU = null; ctx.G.puTimer = 0; ctx.setPUClass(null); } } }
            if (ctx.G.warpT > 0) ctx.G.warpT -= dtMs;
            if (ctx.G.swipeT > 0) ctx.G.swipeT -= dtMs;
            if (ctx.G.portalT > 0) ctx.G.portalT -= dtMs;
            if (ctx.G.glitchT > 0) ctx.G.glitchT -= dtMs;
            if (ctx.G._closeCallCooldown > 0) ctx.G._closeCallCooldown -= dtMs;
            if (ctx.G.warpFlash > 0) ctx.G.warpFlash -= dtMs;
            if (ctx.G.perfectT > 0) ctx.G.perfectT -= dtMs;
            if (ctx.G.bossWarningT > 0) ctx.G.bossWarningT -= dtMs;
            if (ctx.G.comboBanner) { ctx.G.comboBanner.t += dtMs; if (ctx.G.comboBanner.t >= ctx.G.comboBanner.dur) ctx.G.comboBanner = null; }
            if (ctx.G.upgradeBanner) { ctx.G.upgradeBanner.t += dtMs; if (ctx.G.upgradeBanner.t >= ctx.G.upgradeBanner.dur) ctx.G.upgradeBanner = null; }
            if (ctx.G.slowMoT > 0) { ctx.G.slowMoT -= dtMs; if (ctx.G.slowMoT <= 0) ctx.G.timeScale = 1; }
            if (ctx.G.chromAb > 0) ctx.G.chromAb -= dtMs;
            if (ctx.G.muzzleT > 0) ctx.G.muzzleT -= dtMs;
            if (ctx.G.displayScore < ctx.G.score) { ctx.G.displayScore += Math.max(1, Math.ceil((ctx.G.score - ctx.G.displayScore) * 0.1)); if (ctx.G.displayScore > ctx.G.score) ctx.G.displayScore = ctx.G.score; }
            ctx.G.beatT += dt; const _bpm = (ctx.MusicEngine.themes[ctx.MusicEngine.playing] || {}).bpm || 120; ctx.G.beatPhase = (ctx.G.beatT % (60 / (_bpm * ctx.MusicEngine.tempoMult))) / (60 / (_bpm * ctx.MusicEngine.tempoMult));
            let prlen = 0;
            for (let i = 0; i < ctx.G.plasmaRings.length; i++) { const _pr = ctx.G.plasmaRings[i]; _pr.t += dtMs; _pr.r = (_pr.t / _pr.dur) * _pr.maxR; if (_pr.t < _pr.dur) ctx.G.plasmaRings[prlen++] = _pr; }
            ctx.G.plasmaRings.length = prlen;
            let bmlen = 0;
            for (let i = 0; i < ctx.G.pendingBooms.length; i++) {
                const bm = ctx.G.pendingBooms[i]; bm.delay -= dtMs;
                if (bm.delay <= 0) { ctx.boom(bm.x, bm.y, bm.isBoss); } else { ctx.G.pendingBooms[bmlen++] = bm; }
            }
            ctx.G.pendingBooms.length = bmlen;
            if (ctx.G.stageClearLock > 0) ctx.G.stageClearLock -= dtMs;
            if (ctx.G.levelSkipTimer > 0) {
                ctx.G.levelSkipTimer -= dtMs;
                if (ctx.G.levelSkipTimer <= 0 && ctx.G.st === 'PLAYING' && ctx.G.stageClearLock <= 0) {
                    ctx.G.levelSkipTimer = 0;
                    ctx.advanceToNextStage(true);
                }
            }
            const inp2 = ctx.G.inp;
            // NEW: Eased ship banking toward target tilt (smoother than instant)
            ctx.G.shipTiltTarget = (inp2.l ? -0.18 : 0) + (inp2.r ? 0.18 : 0);
            ctx.G.shipPitchTarget = (inp2.u ? -0.16 : 0) + (inp2.d ? 0.16 : 0);
            ctx.G.shipTilt += (ctx.G.shipTiltTarget - ctx.G.shipTilt) * Math.min(1, dt * 8);
            ctx.G.shipPitch += (ctx.G.shipPitchTarget - ctx.G.shipPitch) * Math.min(1, dt * 8);
            if (!inp2.l && !inp2.r) ctx.G.shipTilt *= Math.max(0, 1 - dt * 4);
            if (!inp2.u && !inp2.d) ctx.G.shipPitch *= Math.max(0, 1 - dt * 4);
            let dlen = 0;
            for (let i = 0; i < ctx.G.deathParts.length; i++) {
                const dp = ctx.G.deathParts[i]; dp.x += dp.vx * dt; dp.y += dp.vy * dt; dp.vy += 40 * dt; dp.rot += dt * 4; dp.t += dtMs;
                if (dp.t < dp.life) ctx.G.deathParts[dlen++] = dp;
            }
            ctx.G.deathParts.length = dlen;
            if (Math.random() < 0.008) {
                ctx.G.trails.push({ x: Math.random() * ctx.W, y: 0, vx: -30 - Math.random() * 50, vy: 100 + Math.random() * 80, life: 400, t: 0, col: '#ffffff', size: 1, spark: true });
            }
        }

        function update(dt, now) {
            if (dt > 0.1) dt = 0.1;
            const dtMs = dt * 1000;
            ctx.updateBackground(dt);
            if (ctx.updateDuck) ctx.updateDuck(dtMs);
            if (ctx.updateTweens) ctx.updateTweens(dtMs);
            ctx.updateCombo(dtMs);
            if (ctx.G.inp.p && !ctx.G.inp.pp) {
                if (ctx.G.st === 'PAUSED') { ctx.G.st = ctx.G._prevSt; } else if (ctx.G.st === 'PLAYING') { ctx.G._prevSt = ctx.G.st; ctx.G.st = 'PAUSED'; ctx.G.pauseSel = 0; }
                else if (ctx.G.st === 'SETTINGS') { ctx.G.st = 'TITLE'; }
            }
            if (ctx.G.st === 'PAUSED') { ctx.updatePauseMenu(); return; }
            if (ctx.G.st === 'SETTINGS') { ctx.updateSettingsMenu(); return; }
            if (ctx.G.st === 'SHOP') { ctx.updateShop(); return; }
            if (ctx.G.evoChoiceOpen) { ctx.updateEvoChoice(); return; }
            if (ctx.G.st === 'TITLE') {
                ctx.G.tIdle += dt * 1000;
                if (ctx.G.tIdle > ctx.TITLE_IDLE && !ctx.G.attract) { ctx.G.attract = true; ctx.G.aTmr = 0; ctx.G.score = 0; ctx.G.lives = ctx.diffMod('lives'); ctx.G.stage = 1; ctx.G.p.x = ctx.W / 2; ctx.G.p.y = ctx.H - 50; ctx.G.p.alive = true; ctx.G.p.inv = 0; ctx.G.bul = []; ctx.G.ebul = []; ctx.G.exp = []; ctx.G.part = []; ctx.G.trails = []; ctx.mkFormation(); ctx.MusicEngine.play('title'); }
                if (ctx.G.attract) { ctx.updateAttract(dt); ctx.updateP(dt, now); ctx.updateBul(dt); ctx.updateE(dt); ctx.updateExp(dt); if (ctx.G.inp.s && !ctx.G.inp.sp) { ctx.G.attract = false; ctx.G.tIdle = 0; ctx.G.score = 0; ctx.G.lives = ctx.diffMod('lives'); ctx.G.stage = 1; ctx.G.p.dual = false; ctx.G.p.cap = null; ctx.G.weaponLv = 1; ctx.G.killCount = 0; ctx.G.displayScore = 0; ctx.G.deathParts = []; ctx.startStage(); ctx.MusicEngine.play('gameplay'); } }
                else if (ctx.G.inp.s && !ctx.G.inp.sp) { ctx.SFX.coinInsert(); ctx.G.titleParts = []; ctx.G.score = 0; ctx.G.lives = ctx.diffMod('lives'); ctx.G.stage = 1; ctx.G.p.dual = false; ctx.G.p.cap = null; ctx.G.weaponLv = 1; ctx.G.killCount = 0; ctx.G.displayScore = 0; ctx.G.deathParts = []; ctx.G.collectedPU = new Set(); ctx.G.perfectCount = 0; ctx.G.bossKillTotal = 0; ctx.startStage(); ctx.MusicEngine.play('gameplay'); }
                if (!ctx.G.attract) {
                    if (Math.random() < 0.04) { const _tc = ['#4488ff','#ffcc00','#ff4444','#00ffcc','#ff88aa']; ctx.G.titleParts.push({ x: Math.random() * ctx.W, y: ctx.H + 5, vx: (Math.random()-0.5)*20, vy: -30 - Math.random()*40, life: 2500, t: 0, col: _tc[Math.floor(Math.random()*_tc.length)], size: 1 + Math.floor(Math.random()*2) }); }
                    let _tplen = 0; for (let _ti = 0; _ti < ctx.G.titleParts.length; _ti++) { const _tp = ctx.G.titleParts[_ti]; _tp.x += _tp.vx * dt; _tp.y += _tp.vy * dt; _tp.t += dt * 1000; if (_tp.t < _tp.life && _tp.y >= -10) ctx.G.titleParts[_tplen++] = _tp; } ctx.G.titleParts.length = _tplen;
                }
                return;
            }
            if (ctx.G.st === 'STAGE_INTRO') {
                ctx.G.introTmr -= dt * 1000;
                ctx.updateP(dt, now);
                ctx.updateBul(dt);
                ctx.updateE(dt);
                ctx.updateExp(dt);
                if (ctx.G.introTmr <= 0) { ctx.G.st = 'PLAYING'; ctx.G.introTmr = 0; }
                return;
            }
            if (ctx.G.st === 'GAME_OVER') {
                ctx.G.sTmr -= dt * 1000; ctx.updateExp(dt);
                if (ctx.G.contTmr > 0) { ctx.G.contTmr -= dt; ctx.G.contCnt = Math.ceil(ctx.G.contTmr); }
                if (ctx.G.contTmr > 0 && ctx.G.inp.s && !ctx.G.inp.sp) { ctx.G.lives = ctx.diffMod('lives'); ctx.G.st = 'PLAYING'; ctx.G.p.alive = true; ctx.G.p.x = ctx.W / 2; ctx.G.p.y = ctx.H - 50; ctx.G.p.inv = 3000; ctx.G.activePU = null; ctx.G.shieldHits = 0; ctx.G.powerups = []; ctx.G.timeScale = 1; ctx.G.freezeT = 0; ctx.G.damageVignetteT = 0; ctx.G.combo = 0; ctx.G.comboMult = 1; ctx.mkFormation(); ctx.MusicEngine.play('gameplay'); }
                if (ctx.G.sTmr <= 0 && ctx.G.contTmr <= 0) {
                    if (ctx.relic_earnShards) ctx.relic_earnShards(ctx.G.score, ctx.G.stage);
                    if (ctx.G.score > 0 && ctx.isHS(ctx.G.score)) { ctx.G.st = 'HIGH_SCORE'; ctx.G.ne = { ch: [65, 65, 65], pos: 0, done: false }; ctx.showHSOverlay(); }
                    else { ctx.G.st = 'TITLE'; ctx.G.tIdle = 0; ctx.showTitle(); ctx.MusicEngine.play('title'); }
                }
                return;
            }
            if (ctx.G.st === 'HIGH_SCORE') { ctx.handleName(); return; }
            if (ctx.G.st === 'PLAYING') {
                ctx.updateP(dt, now); ctx.updateBul(dt); ctx.updateE(dt); ctx.updateExp(dt);
                if (ctx.updateHazards) ctx.updateHazards(dt);
                // Super activation check (C / left shoulder / Y button)
                if (ctx.G.inp.super && !ctx.G.inp.superp && ctx.G.superPhase === 'idle' && ctx.G.superMeter >= 100) {
                    if (ctx.startSuper) ctx.startSuper();
                }
                if (ctx.updateSupers) ctx.updateSupers(dt);
                // NEW: Bonus sub-stage auto-advance when timer hits zero (no death penalty)
                if (ctx.G.bonusStage && ctx.G.bonusStageT <= 0 && ctx.G.stageClearLock <= 0) {
                    ctx.G.bonusStage = false;
                    const _rating = ctx.G.stageKills >= 30 ? 'S' : ctx.G.stageKills >= 20 ? 'A' : ctx.G.stageKills >= 10 ? 'B' : 'C';
                    ctx.G.bonusRating = _rating;
                    if (ctx.SFX.bonusEnd) ctx.SFX.bonusEnd(_rating);
                    ctx.G.scorePopups.push({ x: ctx.W / 2, y: ctx.H / 2, text: 'BONUS RANK: ' + _rating, t: 0, dur: 2000, col: '#ffcc00', big: true });
                    ctx.unlockAchievement('bonus_hunter');
                    ctx.advanceToNextStage(true);
                }
                if (ctx.G.shkT > 0) ctx.G.shkT -= dt * 1000;
                if (ctx.G.p.cap) { ctx.G.p.cap.y -= 100 * dt; if (ctx.G.p.cap.y < ctx.G.p.y - 20) { ctx.G.p.dual = true; ctx.G.p.cap = null; ctx.SFX.rescue(); ctx.unlockAchievement('dual_wielder'); } }
                let bossAlive = false, minibossAlive = false, _aliveN = 0;
                for (let _ai = 0; _ai < ctx.G.enemies.length; _ai++) { const _ae = ctx.G.enemies[_ai]; if (_ae.st === 'DEAD') continue; _aliveN++; if (_ae.type === 'boss') bossAlive = true; else if (_ae.type === 'miniboss') { bossAlive = true; minibossAlive = true; } }
                if (_aliveN === 0 && ctx.G.levelSkipTimer <= 0 && ctx.G.stageClearLock <= 0) {
                    ctx.G.stageEmptyT += dtMs;
                    if (ctx.G.stageEmptyT > 350) {
                        ctx.G.stageEmptyT = 0;
                        ctx.mkFormation();
                        let _recovered = 0;
                        for (let _ri = 0; _ri < ctx.G.enemies.length; _ri++) { if (ctx.G.enemies[_ri].st !== 'DEAD') _recovered++; }
                        if (_recovered === 0) ctx.advanceToNextStage(false);
                    }
                } else {
                    ctx.G.stageEmptyT = 0;
                }
                const baseTheme = ctx.G.chal ? 'challenge' : 'gameplay';
                const bossTheme = minibossAlive ? 'miniboss' : 'boss';
                const effectiveBossTheme = ctx.G.stage >= 15 ? 'deep_boss' : bossTheme;
                if (bossAlive && ctx.MusicEngine.playing !== effectiveBossTheme) { ctx.SFX.bossJingle(); ctx.MusicEngine.play(effectiveBossTheme); }
                else if (!bossAlive && (ctx.MusicEngine.playing === 'boss' || ctx.MusicEngine.playing === 'miniboss' || ctx.MusicEngine.playing === 'deep_boss')) ctx.MusicEngine.play(baseTheme);
                else if (!bossAlive && ctx.MusicEngine.playing !== baseTheme && ctx.MusicEngine.playing !== 'challenge' && ctx.MusicEngine.playing !== 'victory') ctx.MusicEngine.play(baseTheme);
                if (_aliveN !== ctx.MusicEngine._lastIntensity) { ctx.MusicEngine.setIntensity(_aliveN); ctx.MusicEngine._lastIntensity = _aliveN; }
            }
        }

        function updateAttract(dt) {
            ctx.G.aTmr += dt * 1000;
            if (ctx.G.aTmr > 300) {
                ctx.G.aTmr = 0;
                const ne = ctx.G.enemies.filter(e => e.st === 'FORM');
                if (ne.length) { const tgt = ne[0]; ctx.G.inp.l = ctx.G.p.x > tgt.x + 5; ctx.G.inp.r = ctx.G.p.x < tgt.x - 5; ctx.G.inp.f = Math.abs(ctx.G.p.x - tgt.x) < 20; }
                else { ctx.G.inp.l = Math.random() < 0.2; ctx.G.inp.r = !ctx.G.inp.l && Math.random() < 0.2; ctx.G.inp.f = Math.random() < 0.2; }
            }
        }

        function updatePauseMenu() {
            const u = ctx.G.inp.u && !ctx.G.inp.up, d = ctx.G.inp.d && !ctx.G.inp.dp, f = ctx.G.inp.f && !ctx.G.inp.fp;
            if (u) ctx.G.pauseSel = (ctx.G.pauseSel + 2) % 3;
            if (d) ctx.G.pauseSel = (ctx.G.pauseSel + 1) % 3;
            if (f) {
                if (ctx.G.pauseSel === 0) { ctx.G.st = ctx.G._prevSt; }
                else if (ctx.G.pauseSel === 1) { ctx.G.st = 'TITLE'; ctx.G.tIdle = 0; ctx.G.score = 0; ctx.G.lives = ctx.diffMod('lives'); ctx.G.stage = 1; ctx.G.p.dual = false; ctx.G.p.cap = null; ctx.G.activePU = null; ctx.G.shieldHits = 0; ctx.G.timeScale = 1; ctx.G.combo = 0; ctx.G.comboMult = 1; ctx.G.weaponLv = 1; ctx.G.killCount = 0; ctx.G.puUpgrade = null; ctx.G.displayScore = 0; ctx.G.deathParts = []; ctx.setPUClass(null); ctx.showTitle(); ctx.MusicEngine.play('title'); }
                else if (ctx.G.pauseSel === 2) { ctx.G.st = 'TITLE'; ctx.G.tIdle = 0; ctx.showTitle(); ctx.MusicEngine.play('title'); }
            }
        }

        function updateSettingsMenu() {
            const u = ctx.G.inp.u && !ctx.G.inp.up, d = ctx.G.inp.d && !ctx.G.inp.dp, f = ctx.G.inp.f && !ctx.G.inp.fp, l = ctx.G.inp.l && !ctx.G.inp.lp, r = ctx.G.inp.r && !ctx.G.inp.rp;
            if (u) ctx.G.settingsSel = Math.max(0, ctx.G.settingsSel - 1);
            if (d) ctx.G.settingsSel = Math.min(7, ctx.G.settingsSel + 1);
            if (f) {
                if (ctx.G.settingsSel === 7) { ctx.G.st = 'TITLE'; }
                else if (ctx.G.settingsSel === 0) { ctx.G.muted = !ctx.G.muted; ctx.settings.mute = ctx.G.muted; ctx.MusicEngine.setMuted(ctx.G.muted); ctx.saveSettings(); }
                else if (ctx.G.settingsSel === 4) { ctx.settings.crt = !ctx.settings.crt; if (ctx.settings.crt) ctx.wrapEl.classList.add('galaxa-crt'); else ctx.wrapEl.classList.remove('galaxa-crt'); ctx.saveSettings(); }
            }
            if (l || r) {
                if (ctx.G.settingsSel === 1) { ctx.settings.diff = l ? (ctx.settings.diff === 'hard' ? 'normal' : ctx.settings.diff === 'normal' ? 'easy' : 'easy') : (ctx.settings.diff === 'easy' ? 'normal' : ctx.settings.diff === 'normal' ? 'hard' : 'hard'); ctx.saveSettings(); }
                if (ctx.G.settingsSel === 2) { ctx.settings.vol = Math.max(0, Math.min(100, ctx.settings.vol + (l ? -10 : 10))); ctx.G.vol = ctx.settings.vol / 100; if (ctx.MusicEngine.masterGain) ctx.MusicEngine.masterGain.gain.value = ctx.G.muted ? 0 : ctx.G.vol * 0.35; ctx.saveSettings(); }
                if (ctx.G.settingsSel === 3) { const ships = Object.keys(ctx.SHIP_TYPES); const idx = ships.indexOf(ctx.settings.ship); ctx.settings.ship = l ? ships[(idx + ships.length - 1) % ships.length] : ships[(idx + 1) % ships.length]; ctx.saveSettings(); }
                if (ctx.G.settingsSel === 5) { const modes = ['high', 'medium', 'low']; const idx = modes.indexOf(ctx.settings.particles); ctx.settings.particles = l ? modes[(idx + modes.length - 1) % modes.length] : modes[(idx + 1) % modes.length]; ctx.saveSettings(); }
                if (ctx.G.settingsSel === 6) { ctx.settings.shake = Math.max(0, Math.min(1, ctx.settings.shake + (l ? -0.25 : 0.25))); ctx.saveSettings(); }
            }
        }

        function showTitle() { ctx.overlayEl.classList.remove('active'); ctx.overlayEl.innerHTML = ''; ctx.MusicEngine.play('title'); }
        function showHSOverlay() {
            ctx.overlayEl.classList.add('active');
            let h = '<div class="galaxa-overlay-box"><h2>' + ctx.esc(ctx.t('galaxa.game_over', 'GAME OVER')) + '</h2>';
            h += '<p>' + ctx.esc(ctx.t('galaxa.score', 'SCORE')) + ': ' + ctx.G.score + '</p><p>' + ctx.esc(ctx.t('galaxa.stage', 'STAGE')) + ': ' + ctx.G.stage + '</p>';
            h += '<p style="margin-top:12px">' + ctx.esc(ctx.t('galaxa.enter_name', 'ENTER YOUR NAME')) + '</p>';
            h += '<div class="galaxa-name-entry" data-ne>';
            for (let i = 0; i < 3; i++) h += '<div class="galaxa-name-char' + (i === 0 ? ' active' : '') + '" data-ci="' + i + '">A</div>';
            h += '</div><p style="font-size:10px;color:#666">\u2191\u2193 change  \u2190\u2192 select  ENTER confirm</p></div>';
            ctx.overlayEl.innerHTML = h;
        }

        function handleName() {
            const ne = ctx.G.ne; if (ne.done) return;
            const u = ctx.G.inp.u && !ctx.G.inp.up, d = ctx.G.inp.d && !ctx.G.inp.dp, l = ctx.G.inp.l && !ctx.G.inp.lp, f = ctx.G.inp.f && !ctx.G.inp.fp, r = ctx.G.inp.r && !ctx.G.inp.rp;
            if (u) ne.ch[ne.pos] = ne.ch[ne.pos] >= 90 ? 65 : ne.ch[ne.pos] + 1;
            if (d) ne.ch[ne.pos] = ne.ch[ne.pos] <= 65 ? 90 : ne.ch[ne.pos] - 1;
            if (l) ne.pos = Math.max(0, ne.pos - 1);
            if (r) ne.pos = Math.min(2, ne.pos + 1);
            ctx.overlayEl.querySelectorAll('[data-ci]').forEach((el, i) => { el.textContent = String.fromCharCode(ne.ch[i]); el.classList.toggle('active', i === ne.pos); });
            if (f) { if (ne.pos < 2) ne.pos++; else { ne.done = true; ctx.submitHS(String.fromCharCode(ne.ch[0], ne.ch[1], ne.ch[2]), ctx.G.score, ctx.G.stage); } }
        }

        function isHS(s) { return ctx.G.hiScores.length < 10 || s > ctx.G.hiScores[ctx.G.hiScores.length - 1].score; }
        async function loadHS() { try { const d = await ctx.api('/api/desktop/galaxa/highscore'); ctx.G.hiScores = Array.isArray(d) ? d : []; if (ctx.G.hiScores.length) ctx.G.hi = ctx.G.hiScores[0].score; } catch (e) {} }
        async function submitHS(name, score, stage) {
            try { const d = await ctx.api('/api/desktop/galaxa/highscore/submit', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, score, stage }) }); ctx.G.hiScores = Array.isArray(d) ? d : []; if (ctx.G.hiScores.length) ctx.G.hi = ctx.G.hiScores[0].score; } catch (e) {}
            ctx.overlayEl.classList.remove('active'); ctx.overlayEl.innerHTML = ''; ctx.G.st = 'TITLE'; ctx.G.tIdle = 0; ctx.showTitle();
        }

        function pollGP() {
            ctx.G.gp.l = false; ctx.G.gp.r = false; ctx.G.gp.u = false; ctx.G.gp.d = false; ctx.G.gp.f = false; ctx.G.gp.s = false; ctx.G.gp.p = false;
            try {
                const gps = navigator.getGamepads ? navigator.getGamepads() : [];
                for (const gp of gps) {
                    if (!gp) continue; const dz = 0.3, ax0 = gp.axes[0] || 0, ax1 = gp.axes[1] || 0;
                    if (ax0 < -dz || !!gp.buttons[14]?.pressed) ctx.G.gp.l = true;
                    if (ax0 > dz || !!gp.buttons[15]?.pressed) ctx.G.gp.r = true;
                    if (ax1 < -dz || !!gp.buttons[12]?.pressed) ctx.G.gp.u = true;
                    if (ax1 > dz || !!gp.buttons[13]?.pressed) ctx.G.gp.d = true;
                    if (!!gp.buttons[0]?.pressed) ctx.G.gp.f = true;
                    if (!!gp.buttons[9]?.pressed) ctx.G.gp.s = true;
                    if (!!gp.buttons[8]?.pressed) ctx.G.gp.p = true;
                    // NEW: parry on right shoulder (button 5) or X face button (button 2), super on Y face button (button 3) or left shoulder (button 4)
                    if (!!gp.buttons[5]?.pressed || !!gp.buttons[2]?.pressed) ctx.G.gp.parry = true;
                    if (!!gp.buttons[3]?.pressed || !!gp.buttons[4]?.pressed) ctx.G.gp.super = true;
                    break;
                }
            } catch (e) {}
        }

        function mergeInput() {
            ctx.G.inp.l = ctx.G.kb.l || ctx.G.gp.l; ctx.G.inp.r = ctx.G.kb.r || ctx.G.gp.r;
            ctx.G.inp.u = ctx.G.kb.u || ctx.G.gp.u; ctx.G.inp.d = ctx.G.kb.d || ctx.G.gp.d;
            ctx.G.inp.f = ctx.G.kb.f || ctx.G.gp.f; ctx.G.inp.s = ctx.G.kb.s || ctx.G.gp.s; ctx.G.inp.p = ctx.G.kb.p || ctx.G.gp.p;
            ctx.G.inp.parry = ctx.G.kb.parry || ctx.G.gp.parry;
            ctx.G.inp.super = ctx.G.kb.super || ctx.G.gp.super;
        }

        function onKey(e) {
            if (ctx.state.disposed) return; const k = e.key;
            if (k === 'ArrowLeft' || k === 'a') { ctx.G.kb.l = true; e.preventDefault(); }
            if (k === 'ArrowRight' || k === 'd') { ctx.G.kb.r = true; e.preventDefault(); }
            if (k === 'ArrowUp' || k === 'w') { ctx.G.kb.u = true; e.preventDefault(); }
            if (k === 'ArrowDown' || k === 's') { ctx.G.kb.d = true; e.preventDefault(); }
            if (k === ' ' || k === 'Enter') { ctx.G.kb.f = true; ctx.G.kb.s = true; e.preventDefault(); }
            if (k === 'Escape') { ctx.G.kb.p = true; e.preventDefault(); }
            if (k === 'm' || k === 'M') { ctx.G.muted = !ctx.G.muted; ctx.settings.mute = ctx.G.muted; ctx.MusicEngine.setMuted(ctx.G.muted); ctx.saveSettings(); }
            if (k === 'x' || k === 'X') { ctx.G.kb.parry = true; e.preventDefault(); }
            if (k === 'c' || k === 'C') { ctx.G.kb.super = true; e.preventDefault(); }
            if ((k === 'S' || k === 's') && ctx.G.st === 'TITLE' && !ctx.G.attract && !ctx.G.kb.d) { ctx.G.st = 'SETTINGS'; ctx.G.settingsSel = 0; }
            if ((k === 'D' || k === 'd') && ctx.G.st === 'TITLE' && !ctx.G.attract && !ctx.G.kb.r) { ctx.SFX.coinInsert(); ctx.startDailyChallenge(); }
        }
        function onKeyUp(e) {
            const k = e.key;
            if (k === 'ArrowLeft' || k === 'a') ctx.G.kb.l = false;
            if (k === 'ArrowRight' || k === 'd') ctx.G.kb.r = false;
            if (k === 'ArrowUp' || k === 'w') ctx.G.kb.u = false;
            if (k === 'ArrowDown' || k === 's') ctx.G.kb.d = false;
            if (k === ' ' || k === 'Enter') { ctx.G.kb.f = false; ctx.G.kb.s = false; }
            if (k === 'Escape') ctx.G.kb.p = false;
            if (k === 'x' || k === 'X') ctx.G.kb.parry = false;
            if (k === 'c' || k === 'C') ctx.G.kb.super = false;
        }

        function savePrev() { ctx.G.inp.fp = ctx.G.inp.f; ctx.G.inp.sp = ctx.G.inp.s; ctx.G.inp.pp = ctx.G.inp.p; ctx.G.inp.lp = ctx.G.inp.l; ctx.G.inp.rp = ctx.G.inp.r; ctx.G.inp.up = ctx.G.inp.u; ctx.G.inp.dp = ctx.G.inp.d; ctx.G.inp.parryp = ctx.G.inp.parry; ctx.G.inp.superp = ctx.G.inp.super; }

        let touchJoystick = null;
        let touchFire = false;
        let touchStartY = 0;
        let touchStartX = 0;
        const isTouchDevice = ('ontouchstart' in window) || (navigator.maxTouchPoints > 0);

        function onTouchStart(e) {
            if (ctx.state.disposed) return;
            e.preventDefault();
            for (const touch of e.changedTouches) {
                const rect = ctx.canvas.getBoundingClientRect();
                const tx = (touch.clientX - rect.left) / ctx.scale;
                const ty = (touch.clientY - rect.top) / ctx.scale;
                if (tx < ctx.W / 2) {
                    touchJoystick = { id: touch.identifier, startX: tx, startY: ty, curX: tx, curY: ty };
                    touchStartY = ty;
                    touchStartX = tx;
                } else {
                    touchFire = true;
                    ctx.G.kb.f = true; ctx.G.kb.s = true;
                }
            }
        }

        function onTouchMove(e) {
            if (ctx.state.disposed) return;
            e.preventDefault();
            for (const touch of e.changedTouches) {
                if (touchJoystick && touch.identifier === touchJoystick.id) {
                    const rect = ctx.canvas.getBoundingClientRect();
                    touchJoystick.curX = (touch.clientX - rect.left) / ctx.scale;
                    touchJoystick.curY = (touch.clientY - rect.top) / ctx.scale;
                    const dx = touchJoystick.curX - touchJoystick.startX;
                    const dy = touchJoystick.curY - touchJoystick.startY;
                    ctx.G.kb.l = dx < -8;
                    ctx.G.kb.r = dx > 8;
                    ctx.G.kb.u = dy < -8;
                    ctx.G.kb.d = dy > 8;
                }
            }
        }

        function onTouchEnd(e) {
            if (ctx.state.disposed) return;
            e.preventDefault();
            for (const touch of e.changedTouches) {
                if (touchJoystick && touch.identifier === touchJoystick.id) {
                    const dy = touchJoystick.curY - touchStartY;
                    const dx = touchJoystick.curX - touchStartX;
                    if (dy < -50 && Math.abs(dx) < 40) {
                        ctx.G.kb.s = true;
                        setTimeout(() => { ctx.G.kb.s = false; }, 100);
                    }
                    touchJoystick = null;
                    ctx.G.kb.l = false;
                    ctx.G.kb.r = false;
                    ctx.G.kb.u = false;
                    ctx.G.kb.d = false;
                } else {
                    touchFire = false;
                    ctx.G.kb.f = false;
                    ctx.G.kb.s = false;
                }
            }
        }

        function setupTouch() {
            if (!isTouchDevice) return;
            ctx.canvas.addEventListener('touchstart', onTouchStart, { passive: false });
            ctx.canvas.addEventListener('touchmove', onTouchMove, { passive: false });
            ctx.canvas.addEventListener('touchend', onTouchEnd, { passive: false });
            ctx.canvas.addEventListener('touchcancel', onTouchEnd, { passive: false });
        }

        let _frameBudgetSkip = 0;
        function loop() {
            if (ctx.state.disposed) return;
            const dt = ctx.frameDelta();
            ctx.savePrev(); ctx.pollGP(); ctx.mergeInput();
            ctx.update(dt, performance.now());
            ctx.tick++;
            ctx.renderFrame(dt);
            if (dt > 0.018) { _frameBudgetSkip = Math.min(3, _frameBudgetSkip + 1); } else if (_frameBudgetSkip > 0) _frameBudgetSkip--;
            ctx.rafId = requestAnimationFrame(ctx.loop);
        }

        ctx.resize = resize;
        ctx.isChal = isChal;
        ctx.isMiniBossStage = isMiniBossStage;
        ctx.getBiomeForStage = GC.getBiomeForStage;
        ctx.advanceToNextStage = advanceToNextStage;
        ctx.startStage = startStage;
        ctx.updateExp = updateExp;
        ctx.update = update;
        ctx.updateAttract = updateAttract;
        ctx.startDailyChallenge = startDailyChallenge;
        ctx.checkDailyStreak = checkDailyStreak;
        ctx.updatePauseMenu = updatePauseMenu;
        ctx.updateSettingsMenu = updateSettingsMenu;
        ctx.showTitle = showTitle;
        ctx.showHSOverlay = showHSOverlay;
        ctx.handleName = handleName;
        ctx.isHS = isHS;
        ctx.loadHS = loadHS;
        ctx.submitHS = submitHS;
        ctx.pollGP = pollGP;
        ctx.mergeInput = mergeInput;
        ctx.onKey = onKey;
        ctx.onKeyUp = onKeyUp;
        ctx.savePrev = savePrev;
        ctx.loop = loop;
        ctx.setupTouch = setupTouch;
        ctx.isTouchDevice = function() { return isTouchDevice; };
    };
})();
