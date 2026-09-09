(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createRenderer = function (ctx) {
        GC.createRenderEffects(ctx); GC.createRenderStage(ctx); GC.createRenderHUD(ctx); GC.createRenderWorld(ctx);
        function text(value, y, size, color) {
            ctx.c.fillStyle = color || '#e5edf7'; ctx.c.textAlign = 'center'; ctx.c.font = 'bold ' + size + 'px monospace';
            ctx.c.fillText(value, ctx.W / 2, y, ctx.W - 48);
        }
        function renderFrame() {
            const G = ctx.G, c = ctx.c, W = ctx.W, H = ctx.H;
            if (ctx.touchActions) ctx.touchActions.hidden = !['PLAYING', 'STAGE_INTRO'].includes(G.st);
            c.save(); c.setTransform(ctx.scale, 0, 0, ctx.scale, 0, 0); c.imageSmoothingEnabled = false;
            c.fillStyle = '#050a17'; c.fillRect(0, 0, W, H);
            ctx.drawNebula(c); ctx.drawStars(c);
            const world = !['TITLE', 'SETTINGS', 'SHOP', 'LOOP_CLEAR', 'RUN_CLEAR', 'LOADING'].includes(G.st);
            if (world) {
                c.save();
                if (G.shkT > 0 && !ctx.settings.reducedMotion) {
                    const strength = Math.min(4, G.shkM || 0) * ctx.settings.shake * Math.min(1, G.shkT / 200);
                    c.translate(Math.round(Math.sin(ctx.tick * 2.3) * strength), Math.round(Math.cos(ctx.tick * 1.7) * strength));
                }
                if (!ctx.settings.reducedMotion && ctx.fxDrawBack) ctx.fxDrawBack(c);
                ctx.renderGame();
                ctx.renderDangers();
                c.restore();
                // HUD has its own fixed transform; no camera movement or particles cover it.
                ctx.renderHUD();
                const boss = G.encounterBoss;
                if (boss && boss.st !== 'DEAD') {
                    c.fillStyle = '#111d32'; c.fillRect(90, 67, W - 180, 9);
                    c.fillStyle = boss.invulnerable ? '#7293af' : '#eaaa6b'; c.fillRect(90, 67, (W - 180) * Math.max(0, boss.hp / boss.maxHp), 9);
                    for (const threshold of [0.30, 0.65]) { c.fillStyle = '#050a17'; c.fillRect(90 + (W - 180) * threshold, 66, 2, 11); }
                    text(ctx.t('galaxa.boss_' + boss.sectorBoss) + ' · ' + boss.bossPhase + '/3', 61, 11);
                }
                if (G.st === 'STAGE_INTRO') ctx.renderStageIntro();
                if (G.st === 'PAUSED') ctx.renderPause();
                if (G.st === 'GAME_OVER') {
                    c.fillStyle = 'rgba(5,10,23,0.9)'; c.fillRect(32, H / 2 - 70, W - 64, 145);
                    text(ctx.t('galaxa.game_over'), H / 2 - 22, 26, '#f59680');
                    text(ctx.t('galaxa.score') + ' ' + G.score, H / 2 + 10, 16);
                    if (G.contTmr > 0) text(ctx.t('galaxa.continue_prompt') + ' ' + G.contCnt, H / 2 + 45, 14, '#ead19b');
                }
                if (G.demoMode) text(ctx.t('galaxa.demo_hint'), H - 62, 12, '#ead19b');
            } else if (G.st === 'TITLE') ctx.renderTitle();
            else if (G.st === 'SETTINGS') ctx.renderSettings();
            else if (G.st === 'SHOP') ctx.renderShop();
            else if (G.st === 'RUN_CLEAR') {
                text(ctx.t(ctx.settings.mode === 'gauntlet' ? 'galaxa.gauntlet_clear' : 'galaxa.boss_rush_clear'), 280, 26, '#ead19b');
                text(ctx.t('galaxa.score') + ' ' + G.score, 330, 20);
                text(ctx.t('galaxa.continue_hint'), 400, 12);
            } else if (G.st === 'LOOP_CLEAR') {
                text(ctx.t('galaxa.loop_complete'), 260, 26, '#ead19b');
                text(ctx.t('galaxa.score') + ' ' + G.score, 310, 20);
                text(ctx.t('galaxa.loop_next') + ' ' + ctx.stagePlan().loop, 350, 16);
                text(ctx.t('galaxa.continue_hint'), 410, 12);
            }
            c.restore();
        }
        ctx.renderFrame = renderFrame;
    };
})();
