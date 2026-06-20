(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createDemo = function (ctx) {
        let decisionTmr = 0;
        let menuPhase = 'idle';
        let menuTmr = 0;
        let menuStep = 0;

        function clearAI() {
            const ai = ctx.G.ai;
            ai.l = false; ai.r = false; ai.u = false; ai.d = false;
            ai.f = false; ai.s = false; ai.p = false;
            ai.parry = false; ai.super = false;
        }

        function startDemo() {
            ctx.G.score = 0;
            ctx.G.lives = ctx.diffMod('lives');
            ctx.G.stage = 1;
            ctx.G.p.dual = false;
            ctx.G.p.cap = null;
            ctx.G.weaponLv = 1;
            ctx.G.killCount = 0;
            ctx.G.displayScore = 0;
            ctx.G.deathParts = [];
            ctx.G.collectedPU = new Set();
            ctx.G.perfectCount = 0;
            ctx.G.demoMode = true;
            clearAI();
            ctx.startStage();
            ctx.MusicEngine.play('gameplay');
            decisionTmr = 0;
            menuPhase = 'idle';
            menuTmr = 0;
            menuStep = 0;
        }

        function findNearestEnemy() {
            let best = null, bestD2 = Infinity;
            const px = ctx.G.p.x, py = ctx.G.p.y;
            for (let i = 0; i < ctx.G.enemies.length; i++) {
                const e = ctx.G.enemies[i];
                if (e.st === 'DEAD') continue;
                const d2 = (e.x - px) * (e.x - px) + (e.y - py) * (e.y - py);
                if (d2 < bestD2) { bestD2 = d2; best = e; }
            }
            return best;
        }

        function findNearestPowerup() {
            let best = null, bestD2 = Infinity;
            const px = ctx.G.p.x, py = ctx.G.p.y;
            for (let i = 0; i < ctx.G.powerups.length; i++) {
                const pu = ctx.G.powerups[i];
                const d2 = (pu.x - px) * (pu.x - px) + (pu.y - py) * (pu.y - py);
                if (d2 < bestD2) { bestD2 = d2; best = pu; }
            }
            return best;
        }

        function bulletThreat() {
            const px = ctx.G.p.x, py = ctx.G.p.y;
            const la = ctx.DEMO_DODGE_LOOKAHEAD;
            const dr = ctx.DEMO_DODGE_RADIUS;
            for (let i = 0; i < ctx.G.ebul.length; i++) {
                const b = ctx.G.ebul[i];
                const vx = b.vx || 0, vy = b.vy || ctx.EB_SPEED * 0.5;
                const fx = b.x + vx * la * (1 / 60), fy = b.y + vy * la * (1 / 60);
                const dx = Math.min(Math.abs(fx - px), Math.abs(b.x - px));
                const dy = Math.min(Math.abs(fy - py), Math.abs(b.y - py));
                if (dx < dr && dy < dr * 2) {
                    return { x: b.x, y: b.y, vx: vx, vy: vy };
                }
            }
            return null;
        }

        function updateDemo(dt) {
            if (!ctx.G.demoMode) return;
            const ai = ctx.G.ai;
            const st = ctx.G.st;

            if (st === 'STAGE_INTRO' || st === 'STAGE_CLEAR') {
                clearAI();
                return;
            }

            if (st === 'SHOP') {
                updateDemoShop(dt);
                return;
            }

            if (ctx.G.evoChoiceOpen) {
                updateDemoEvo(dt);
                return;
            }

            if (st === 'GAME_OVER') {
                clearAI();
                return;
            }

            if (st !== 'PLAYING') {
                clearAI();
                return;
            }

            decisionTmr -= dt * 1000;
            const shouldDecide = decisionTmr <= 0;
            if (shouldDecide) decisionTmr = ctx.DEMO_DECISION_MS;

            ai.f = true;

            const threat = shouldDecide ? bulletThreat() : null;
            const prevThreat = bulletThreat();

            if (threat || prevThreat) {
                const t = threat || prevThreat;
                if (t.vx > 0) { ai.l = true; ai.r = false; }
                else if (t.vx < 0) { ai.r = true; ai.l = false; }
                else {
                    ai.l = ctx.G.p.x > ctx.W / 2;
                    ai.r = !ai.l;
                }
                ai.u = false; ai.d = false;
            } else {
                const pu = shouldDecide ? findNearestPowerup() : null;
                if (pu && pu.y < ctx.G.p.y - 10 && Math.abs(pu.x - ctx.G.p.x) < 80) {
                    ai.l = pu.x < ctx.G.p.x - ctx.DEMO_AIM_DEADZONE;
                    ai.r = pu.x > ctx.G.p.x + ctx.DEMO_AIM_DEADZONE;
                    ai.u = pu.y < ctx.G.p.y - 30;
                    ai.d = false;
                } else {
                    const target = shouldDecide ? findNearestEnemy() : null;
                    if (target) {
                        ai.l = target.x < ctx.G.p.x - ctx.DEMO_AIM_DEADZONE;
                        ai.r = target.x > ctx.G.p.x + ctx.DEMO_AIM_DEADZONE;
                        ai.u = false; ai.d = false;
                    } else {
                        ai.l = false; ai.r = false; ai.u = false; ai.d = false;
                    }
                }
            }

            ai.s = false; ai.p = false;
            ai.parry = false; ai.super = false;
        }

        function updateDemoShop(dt) {
            const ai = ctx.G.ai;
            menuTmr -= dt * 1000;
            if (menuTmr > 0) { clearAI(); return; }
            menuTmr = ctx.DEMO_MENU_TAP_MS;

            const total = ctx.shopItemCount ? ctx.shopItemCount() : 5;
            const leaveIdx = total;

            if (menuPhase === 'idle') {
                menuPhase = 'navigate';
                menuStep = 0;
                clearAI();
                return;
            }

            if (menuPhase === 'navigate') {
                ai.f = false;
                if (menuStep < leaveIdx) {
                    ai.d = true; ai.u = false;
                    menuStep++;
                    if (menuStep >= leaveIdx) menuPhase = 'confirm_leave';
                } else {
                    menuPhase = 'confirm_leave';
                }
                return;
            }

            if (menuPhase === 'confirm_leave') {
                ai.d = false; ai.u = false; ai.f = true;
                menuPhase = 'release';
                return;
            }

            if (menuPhase === 'release') {
                clearAI();
                menuPhase = 'idle';
                menuStep = 0;
                return;
            }
        }

        function updateDemoEvo(dt) {
            const ai = ctx.G.ai;
            menuTmr -= dt * 1000;
            if (menuTmr > 0) { clearAI(); return; }
            menuTmr = ctx.DEMO_MENU_TAP_MS;

            if (menuPhase === 'idle') {
                menuPhase = 'navigate';
                menuStep = 0;
                clearAI();
                return;
            }

            if (menuPhase === 'navigate') {
                const target = 1;
                if (menuStep < target) { ai.d = true; ai.u = false; menuStep++; }
                else if (menuStep > target) { ai.u = true; ai.d = false; menuStep--; }
                else { menuPhase = 'confirm'; }
                return;
            }

            if (menuPhase === 'confirm') {
                ai.u = false; ai.d = false; ai.f = true;
                menuPhase = 'release';
                return;
            }

            if (menuPhase === 'release') {
                clearAI();
                menuPhase = 'idle';
                menuStep = 0;
                return;
            }
        }

        ctx.startDemo = startDemo;
        ctx.updateDemo = updateDemo;
    };
})();
