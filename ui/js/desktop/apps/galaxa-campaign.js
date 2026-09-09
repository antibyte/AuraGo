(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createCampaign = function (ctx) {
        let rng = GC.makeRunRandom(Date.now());
        const scheduled = [];
        ctx.random = () => rng();
        ctx.seedRun = seed => { rng = GC.makeRunRandom(seed); ctx.G.runSeed = seed; };
        ctx.scheduleGame = (fn, delay) => { scheduled.push({ fn, left: delay, generation: ctx.G.runGeneration || 0 }); };
        ctx.clearGameSchedule = () => { scheduled.length = 0; };
        ctx.tickGameSchedule = ms => {
            const due = [];
            for (let i = scheduled.length - 1; i >= 0; i--) {
                const task = scheduled[i]; task.left -= ms;
                if (task.left <= 0) { scheduled.splice(i, 1); due.unshift(task); }
            }
            for (const task of due) if (!ctx.state.disposed && task.generation === (ctx.G.runGeneration || 0)) task.fn();
        };
        ctx.isCampaign = () => ['classic', 'mirror', 'daily'].includes(ctx.settings.mode);
        ctx.stagePlan = () => GC.getStagePlan(ctx.G.stage, ctx.settings.mode);
        ctx.hasPendingWaves = () => !!ctx.G.wavePlan && (ctx.G.waveIndex < ctx.G.wavePlan.length || (ctx.G.chal && ctx.G.bonusStageT > 0));
        ctx.newEnemy = function (type, x, y, index, hp) {
            const core = type === 'boss' || type === 'miniboss';
            return { type, x: x < ctx.W / 2 ? -30 : ctx.W + 30, y: -40 - index * 15, fx: x, fy: y,
                r: Math.floor(index / 8), col: index % 8, hp: hp || (core ? 4 : ['hunter','shield_bee','carrier'].includes(type) ? 3 : 1),
                maxHp: hp || (core ? 4 : ['hunter','shield_bee','carrier'].includes(type) ? 3 : 1),
                st: 'ENTER', eTmr: index * 100, dTmr: 3000 + index * 250, sTmr: 1800 + index * 120,
                fr: 0, frT: 0, animFrame: 0, animTimer: 0, animTime: 0, animSpeed: 85, animFrames: 8,
                hitF: 0, shootPh: 0, dPath: null, bossPhase: 0, bossPhaseTransition: 0,
                spawnAnim: 0, spawnDur: 400, rowPhase: index * 0.7, bobAmp: 2, elite: type === 'hunter' };
        };
        ctx.spawnCampaignWave = function () {
            const G = ctx.G, plan = G.wavePlan[G.waveIndex++];
            if (!plan) return;
            const sector = ctx.stagePlan().sector;
            for (let i = 0; i < plan.length; i++) {
                const col = i % 8, row = Math.floor(i / 8);
                const x = 62 + col * 59, y = 95 + row * 57 + (G.waveIndex % 2 ? Math.abs(3.5 - col) * 8 : 0);
                const e = ctx.newEnemy(plan[i], x, y, i);
                if (ctx.stagePlan().loop > 1) e.hp = e.maxHp = Math.min(6, e.hp + Math.floor((ctx.stagePlan().loop - 1) / 2));
                G.enemies.push(e);
            }
            G.chalTot += plan.length;
            G.nextWaveAt = G.stageElapsed + (G.chal ? 5000 : 16000);
            if (G.waveIndex > 1 && ctx.SFX.waveArrive) ctx.SFX.waveArrive(sector);
        };
        ctx.mkCampaignFormation = function () {
            const G = ctx.G, plan = ctx.stagePlan(), sector = GC.SECTORS[plan.sector];
            G.enemies = []; G.stageElapsed = 0; G.waveIndex = 0; G.chalHits = 0; G.chalTot = 0;
            G.wavePlan = null; G.encounterBoss = null; G.bossPhase = 0;
            G.campaignWarnings = []; G.bossLasers = []; G.bossDeath = null;
            G.fX = 0; G.dTmr = 2400; G.chal = plan.kind === 'bonus';
            G.bonusStage = G.chal; G.bonusStageT = G.chal ? 25000 : 0;
            if (plan.kind === 'boss') {
                const hp = Math.round([650, 700, 750, 780, 850, 850][plan.sector] * Math.min(1.8, 1 + (plan.loop - 1) * 0.15));
                const e = ctx.newEnemy('miniboss', ctx.W / 2, 155, 0, hp);
                Object.assign(e, { x: ctx.W / 2, y: -90, sectorBoss: sector.id, bossPhase: 1, bossPhaseHP: [0.65, 0.30],
                    phaseClock: 0, attackClock: 0, bossState: 'enter', invulnerable: true, st: 'FORM',
                    weakPoint: { x: 0, y: 24, angle: 0 }, armor: sector.id === 'asteroid' ? [40, 40] : null });
                G.enemies.push(e); G.encounterBoss = e; G.chalTot = 1;
                G.bossWarningT = 1400;
                if (ctx.SFX.bossWarning) ctx.SFX.bossWarning();
            } else {
                const roles = sector.enemies;
                const count = plan.kind === 'bonus' ? 16 : plan.kind === 'elite' ? 16 : 12;
                const waves = plan.kind === 'bonus' ? 5 : 3;
                G.wavePlan = Array.from({ length: waves }, (_, wave) => Array.from({ length: count }, (_, i) => {
                    if (plan.kind === 'bonus') return i % 3 ? 'bee' : 'butterfly';
                    if (plan.kind === 'intro') return i % 4 ? roles[0] : roles[1];
                    if (plan.kind === 'escort') return i % 5 === 0 ? 'boss' : roles[(i + wave) % roles.length];
                    return i % 6 === 0 ? 'hunter' : roles[(i + wave) % roles.length];
                }));
                ctx.spawnCampaignWave();
            }
            ctx.mkNebula(); ctx.initBG();
        };
        ctx.updateCampaign = function (dt) {
            const G = ctx.G;
            if (G.st !== 'PLAYING') return;
            G.stageElapsed = (G.stageElapsed || 0) + dt * 1000;
            for (const laser of G.bossLasers || []) {
                laser.left -= dt * 1000;
                if (laser.left > 0 && G.p.alive && G.p.inv <= 0 && ctx.hit({ x: laser.x - laser.w / 2, y: laser.y, w: laser.w, h: laser.h }, ctx.getShipHitbox())) ctx.killP();
            }
            G.bossLasers = (G.bossLasers || []).filter(laser => laser.left > 0);
            if (G.wavePlan && G.waveIndex < G.wavePlan.length && G.stageElapsed >= G.nextWaveAt) ctx.spawnCampaignWave();
            if (G.encounterBoss && (G.encounterBoss.hp <= 0 || G.encounterBoss.st === 'DEAD') && !G.bossDeath) {
                const e = G.encounterBoss;
                ctx.addScore(5000 + ctx.stagePlan().sector * 1000, e.x, e.y); ctx.registerKill(e.x, e.y); ctx.dropPU(e);
                G.killCount++; G.stageKills++; G.credits += Math.floor(10 * (ctx.relic_getRelicBonuses ? ctx.relic_getRelicBonuses().creditMult : 1));
                try { localStorage.setItem('galaxa_credits', String(G.credits)); } catch (_) {}
                G.bossDeath = { id: e.sectorBoss, x: e.x, y: e.y, t: 0 };
                G.ebul.length = 0; G.campaignWarnings.length = 0; G.bossLasers.length = 0;
                G.bossKillTotal++; try { localStorage.setItem('galaxa_boss_kills', String(G.bossKillTotal)); } catch (_) {}
                if (G.bossKillTotal >= 10) ctx.unlockAchievement('boss_slayer');
                ctx.SFX.bossDeathStinger(); G.hitstopT = 100; G.shkT = 350; G.shkM = 4;
                G.p.inv = Math.max(G.p.inv, 1800);
                for (const other of G.enemies) other.st = 'DEAD';
                G.stageClearLock = 1400;
                ctx.scheduleGame(() => { G.stageClearLock = 0; ctx.advanceToNextStage(false); }, 1450);
            }
            if (G.bossDeath) G.bossDeath.t += dt * 1000;
        };
        function emit(e, angle, speed, kind, x, y) {
            if (ctx.G.ebul.length >= 220) return;
            const mirrored = ctx.modesIsMirrorPermanent && ctx.modesIsMirrorPermanent();
            ctx.G.ebul.push({ x: mirrored ? ctx.W - (x ?? e.x) : (x ?? e.x), y: y ?? e.y + 38,
                vx: Math.cos(angle) * speed * (mirrored ? -1 : 1), vy: Math.sin(angle) * speed, w: 7, h: 7, kind: kind || 'sector' });
        }
        function fireBoss(e) {
            const G = ctx.G, phase = e.bossPhase, plan = ctx.stagePlan();
            const speed = (105 + phase * 14) * ctx.diffMod('ebSpd') * Math.min(1.5, 1 + (plan.loop - 1) * 0.08);
            const aimed = Math.atan2(G.p.y - e.y, G.p.x - e.x);
            const fan = (angle, n, spread) => { for (let i = 0; i < n; i++) emit(e, angle + (i - (n - 1) / 2) * spread, speed); };
            const pattern = e.sectorBoss === 'void' ? ['nebula', 'crystal', 'storm'][(e.volley || 0) % 3] : e.sectorBoss;
            if (pattern === 'nebula') {
                fan(aimed, 5 + phase * 2, 0.17);
                e.coreOpen = 900;
                if (phase > 1 && (e.volley || 0) % 3 === 0 && G.enemies.filter(a => a.st !== 'DEAD').length < 7) {
                    for (let i = 0; i < 4; i++) G.enemies.push(ctx.newEnemy('bee', 90 + i * 120, 245, i));
                }
            } else if (pattern === 'asteroid') {
                fan(Math.PI / 2, 5, 0.29);
                if (phase > 1) { e.chargeX = e.targetX; e.chargeT = 1000; }
            } else if (pattern === 'crystal') {
                e.reflectT = 750;
                for (let lane = 0; lane < 8; lane++) {
                    if (Math.abs(lane - e.safeLane) <= 1) continue;
                    G.bossLasers.push({ x: 40 + lane * 65, y: 230, w: 18, h: ctx.H - 260, left: 550 });
                }
                if (phase > 1) fan(aimed, 3, 0.24);
            } else if (pattern === 'storm') {
                for (let lane = 0; lane < 8; lane++) {
                    if (Math.abs(lane - e.safeLane) <= 1) continue;
                    for (let row = 0; row < phase + 1; row++) emit(e, Math.PI / 2, speed, 'ion', 40 + lane * 65, 140 - row * 24);
                }
            } else {
                for (let i = 0; i < 14 + phase * 2; i++) {
                    const angle = i * Math.PI * 2 / (14 + phase * 2) + (e.volley || 0) * 0.15;
                    emit(e, angle, speed * 0.75, 'gravity');
                    const bullet = G.ebul[G.ebul.length - 1];
                    if (bullet) { bullet.orbitX = e.x; bullet.orbitY = e.y; bullet.orbitAngle = angle; bullet.orbitT = 650; }
                }
                e.pullT = 1400; e.coreOpen = 1400;
            }
            e.volley = (e.volley || 0) + 1; e.attackAnim = 800;
            if (ctx.SFX.sectorAttack) ctx.SFX.sectorAttack(e.sectorBoss, e.x);
        }
        ctx.updateSectorBoss = function (e, dt) {
            const G = ctx.G, ms = dt * 1000;
            e.animTime = (e.animTime || 0) + ms; e.hitF = Math.max(0, e.hitF - ms);
            e.attackAnim = Math.max(0, (e.attackAnim || 0) - ms);
            if (e.bossState === 'enter') {
                e.y = Math.min(155, e.y + dt * 130);
                if (e.y >= 155) { e.bossState = 'recover'; e.attackClock = 900; e.invulnerable = false; }
                return;
            }
            const desiredPhase = e.hp <= e.maxHp * 0.30 ? 3 : e.hp <= e.maxHp * 0.65 ? 2 : 1;
            if (desiredPhase > e.bossPhase) {
                e.bossPhase = desiredPhase; e.bossState = 'transition'; e.attackClock = 1000; e.invulnerable = true;
                G.ebul.length = 0; G.campaignWarnings.length = 0; G.bossLasers.length = 0; G.p.inv = Math.max(G.p.inv, 1400);
                e.chargeT = 0; e.pullT = 0; e.reflectT = 0;
                e.telegraph = null; ctx.SFX.bossPhaseTransition();
            }
            G.bossPhase = e.bossPhase;
            e.phaseClock += ms; e.attackClock -= ms;
            if (e.bossState === 'transition') {
                if (e.attackClock <= 0) { e.bossState = 'recover'; e.attackClock = 700; e.invulnerable = false; }
                return;
            }
            if (G.freezeT > 0 || G.st !== 'PLAYING') return;
            if (e.chargeT > 0) {
                e.chargeT -= ms;
                const t = 1 - Math.max(0, e.chargeT) / 1000;
                e.x = e.chargeX; e.y = 155 + Math.sin(t * Math.PI) * 280;
            } else { e.x = ctx.W / 2 + Math.sin(e.phaseClock / 1700) * 135; e.y = 155 + Math.sin(e.phaseClock / 2300) * 20; }
            if (G.p.alive && G.p.inv <= 0 && ctx.hit({ x: e.x - 38, y: e.y - 34, w: 76, h: 68 }, ctx.getShipHitbox())) ctx.killP();
            if (e.pullT > 0) {
                e.pullT -= ms;
                if (G.p.alive) G.p.x += Math.max(-35, Math.min(35, (e.x - G.p.x) * 0.18)) * dt;
            }
            e.coreOpen = Math.max(0, (e.coreOpen || 0) - ms);
            e.reflectT = Math.max(0, (e.reflectT || 0) - ms);
            if (e.bossState === 'windup') {
                if (e.attackClock <= 0) { e.telegraph = null; fireBoss(e); e.bossState = 'recover'; e.attackClock = 1500 - e.bossPhase * 150; }
            } else if (e.attackClock <= 0) {
                e.bossState = 'windup'; e.attackClock = ctx.settings.diff === 'easy' ? 800 : ctx.settings.diff === 'hard' ? 450 : 600;
                e.targetX = G.p.x; e.safeLane = Math.max(1, Math.min(5, Math.floor(G.p.x / 65)));
                e.telegraph = { x: e.targetX, left: e.attackClock };
                if (ctx.SFX.attackWarning) ctx.SFX.attackWarning(e.x);
            }
        };
        ctx.damageEnemy = function (e, amount) {
            if (!e || e.st === 'DEAD' || e.invulnerable) return 0;
            let damage = amount;
            if (e.sectorBoss) {
                damage = Math.min(amount, e.maxHp * 0.12);
                if (e.armor && e.armor.some(hp => hp > 0)) {
                    const side = ctx.G.p.x < e.x ? 0 : 1;
                    if (e.armor[side] > 0) { e.armor[side] = Math.max(0, e.armor[side] - damage); damage *= 0.35; }
                }
                if (e.coreOpen > 0) damage *= 1.5;
                const floor = e.bossPhase === 1 ? e.maxHp * 0.65 : e.bossPhase === 2 ? e.maxHp * 0.30 : 0;
                e.hp = Math.max(floor, e.hp - damage);
                if (floor && e.hp === floor) e.hp = floor - 0.01;
            } else e.hp -= damage;
            e.hitF = 90; return damage;
        };
        ctx.reflectPlayerShot = function (bullet) {
            const e = ctx.G.encounterBoss;
            if (!e || e.st === 'DEAD' || e.reflectT <= 0) return false;
            for (const side of [-1, 1]) {
                const x = e.x + side * 88, y = e.y + 45;
                if (!ctx.hit(bullet, { x: x - 9, y: y - 24, w: 18, h: 48 })) continue;
                emit(e, Math.PI / 2 + side * 0.25, 170, 'crystal', x, y);
                ctx.bulletImpact(x, y, '#b6f3ed'); return true;
            }
            return false;
        };
    };
})();
