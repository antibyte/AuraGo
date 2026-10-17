(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createEnemyRewards = function (ctx) {
        ctx.finishEnemyKill = function (e, b = {}) {
            if (!e || e.hp > 0 || e.st === 'DEAD' || e.st === 'SMOLDER' || e.sectorBoss || e.rewarded) return false;
            if (!b.noSmolder && ctx.maybeSmolder(e, b)) return false;
            const wasDiving = e.st === 'DIVING' || e.smolderIgnitedAir === true;
            const pts = ctx.PTS[e.type] ? ctx.PTS[e.type][wasDiving ? 1 : 0] : 200;
            // Claim before combo effects can kill another enemy in this explosion.
            e.rewarded = true; e.st = 'DEAD';
            ctx.registerKill(e.x, e.y);
            ctx.addScore(pts, e.x, e.y, e.type === 'bee' ? '#ffcc00' : e.type === 'butterfly' ? '#ff3366' : '#44cc44');
            if (wasDiving) {
                ctx.G.scorePopups.push({ x: e.x, y: e.y - 16, text: 'HEADSHOT!', t: 0, dur: 800, col: '#ff8844', big: true });
                ctx.addScore(100, e.x, e.y - 10, '#ff8844');
                if (ctx.SFX.sparkleTwinkle) ctx.SFX.sparkleTwinkle(e.x);
            }
            if (ctx.G.combo > 0 && ctx.G.combo % 5 === 0) {
                ctx.G.scorePopups.push({ x: e.x + 15, y: e.y - 16, text: 'CHAIN x' + ctx.G.combo + '!', t: 0, dur: 800, col: '#44ff88', big: false });
            }
            ctx.boom(e.x, e.y, !!e.sectorBoss, e.type, b.vx || 0, b.vy || -ctx.PB_SPEED); ctx.dropPU(e);
            const creditGain = (e.type === 'boss' ? 10 : e.type === 'miniboss' ? 7 : 1) + (ctx.G.comboMult >= 4 ? 5 : 0);
            const creditMult = ctx.relic_getRelicBonuses ? ctx.relic_getRelicBonuses().creditMult : 1;
            const earned = (ctx.G.creditRemainder || 0) + creditGain * creditMult;
            ctx.G.credits += Math.floor(earned); ctx.G.creditRemainder = earned % 1;
            try { localStorage.setItem('galaxa_credits', String(ctx.G.credits)); } catch (e2) {}
            if (e.sectorBoss) { ctx.G.hitstopT = 100; }
            if (e.hasCap) ctx.G.p.cap = { x: e.x, y: e.y };
              if (ctx.G.chal) ctx.G.chalHits++; e.st = 'DEAD';
            // NEW: Splitter splits into 2 mini enemies on death
            if (e.type === 'splitter') {
                const _splitType = ctx.G.stage >= 15 && !(e._chained) ? 'splitter' : 'bee';
                const _splitHP = _splitType === 'splitter' ? 1 : 1;
                for (let _si = 0; _si < 2; _si++) {
                    const sx = e.x + (_si === 0 ? -15 : 15);
                    const sy = e.y - 10;
                    ctx.G.enemies.push({ type: _splitType, r: 0, col: 0, x: sx, y: sy, fx: sx, fy: sy, hp: _splitHP, maxHp: _splitHP, st: 'DIVING', eTmr: 0, fr: 0, frT: 0, dTmr: 2000, dPath: { ph: 0, amp: 20, vx: (_si === 0 ? -40 : 40) }, sTmr: 500, shootPh: 0, hasCap: false, hitF: 0, elite: false, bossPhase: 0, bossPhaseTransition: 0, bossPhaseHP: [0,0,0], animFrame: 0, animTimer: 0, animSpeed: 120, animFrames: 8, spawnAnim: 0, spawnDur: 300, rowPhase: ctx.random() * 3, bobAmp: 2, _chained: _splitType === 'splitter' });
                }
            }
            // NEW: Carrier releases 3 bees on death
            if (e.type === 'carrier') {
                for (let _ci = 0; _ci < 3; _ci++) {
                    const ca = (_ci / 3) * Math.PI * 2;
                    ctx.G.enemies.push({ type: 'bee', r: 0, col: 0, x: e.x, y: e.y, fx: e.x + Math.cos(ca) * 40, fy: e.y + Math.sin(ca) * 40, hp: 1, maxHp: 1, st: 'ENTER', eTmr: 300 + _ci * 100, fr: 0, frT: 0, dTmr: 1500, dPath: null, sTmr: 800, shootPh: 0, hasCap: false, hitF: 0, elite: false, bossPhase: 0, bossPhaseTransition: 0, bossPhaseHP: [0,0,0], animFrame: 0, animTimer: 0, animSpeed: 120, animFrames: 8, spawnAnim: 0, spawnDur: 300 });
                }
            }
            ctx.G.killCount++;
            ctx.G.stageKills = (ctx.G.stageKills || 0) + 1;
            if (ctx.G.killCount === 1) ctx.unlockAchievement('first_blood');
            if (ctx.G.activePU && ctx.G.activePU.type === 'chain_lightning') ctx.G._chainLightningTarget = e;
              ctx.G.weaponXP += (e.type === 'boss' ? 3 : e.type === 'miniboss' ? 2 : wasDiving ? 1.5 : 1);
            ctx.grantWeaponLevel();
            // NEW: Floating combat text — damage on hit, crit on headshot/weakpoint
            if (e.weakPoint && Math.hypot(b.x - (e.x + e.weakPoint.x), b.y - (e.y + e.weakPoint.y)) < 6) {
                ctx.G.combatText.push({ x: e.x, y: e.y - 12, text: 'CRIT!', t: 0, dur: 600, col: '#ff4444', big: true });
                if (ctx.SFX && ctx.SFX.critHit) ctx.SFX.critHit(e.x);
            }
            ctx.checkWeaponEvo();
            if (!e.sectorBoss && (e.type === 'boss' || e.type === 'miniboss')) { ctx.G.bossKillTotal++; if (ctx.G.bossKillTotal >= 10) ctx.unlockAchievement('boss_slayer'); try { localStorage.setItem('galaxa_boss_kills', String(ctx.G.bossKillTotal)); } catch(e2) {} }
            if (ctx.G.killCount % 10 === 0 && ctx.G.weaponLv < 4) { ctx.G.weaponLv++; ctx.SFX.weaponUp(); }
            if (e.artId) ctx.expansionEffect('alien-plasma', e.x, e.y, 64);
            return true;
        };
    };
})();
