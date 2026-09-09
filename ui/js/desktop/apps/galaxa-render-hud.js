(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createRenderHUD = function (ctx) {
        function label(text, x, y, size, color, align) {
            const c = ctx.c; c.fillStyle = color || '#d5e8f7'; c.textAlign = align || 'left';
            c.font = size + 'px "Share Tech Mono", monospace'; c.fillText(text, x, y);
        }
        function bar(x, y, width, fraction, color) {
            ctx.c.fillStyle = '#26354a'; ctx.c.fillRect(x, y, width, 4);
            ctx.c.fillStyle = color; ctx.c.fillRect(x, y, width * Math.max(0, Math.min(1, fraction)), 4);
        }
        function renderBeam(tb) {
            const c = ctx.c; c.save(); c.globalAlpha = 0.55;
            c.fillStyle = '#398bc7'; c.beginPath(); c.moveTo(tb.x - 10, tb.y); c.lineTo(tb.x + 10, tb.y);
            c.lineTo(tb.x + 22, tb.y + tb.h); c.lineTo(tb.x - 22, tb.y + tb.h); c.closePath(); c.fill(); c.restore();
        }
        function renderHUD() {
            const G = ctx.G, c = ctx.c, W = ctx.W, H = ctx.H;
            c.fillStyle = '#081322ed'; c.fillRect(0, 0, W, 43); c.fillRect(0, H - 40, W, 40);
            c.fillStyle = '#34516b'; c.fillRect(0, 42, W, 1); c.fillRect(0, H - 41, W, 1);
            label(ctx.t('galaxa.score'), 12, 14, 10, '#88abc6');
            label(String(Math.round(G.displayScore)).padStart(8, '0'), 12, 33, 17, '#e7d2a6');
            label(ctx.t('galaxa.stage') + ' ' + G.stage, W / 2, 15, 12, '#d5e8f7', 'center');
            label(G.chal ? ctx.t('galaxa.bonus_stage') + ' ' + Math.ceil(G.bonusStageT / 1000) : ctx.getModeLabel(), W / 2, 33, 10, '#88abc6', 'center');
            label(ctx.t('galaxa.high_score'), W - 12, 14, 10, '#88abc6', 'right');
            label(String(G.hi).padStart(8, '0'), W - 12, 33, 15, '#d5e8f7', 'right');
            ctx.drawAnimation(c, 'icon.' + ctx.settings.ship, 21, H - 20, 0, 24);
            label('× ' + G.lives, 38, H - 16, 14);
            label('W' + G.weaponLv + (G.weaponEvo ? ' · ' + ctx.WEAPON_EVOS[G.weaponEvo].name : ''), 92, H - 23, 10, '#b6d1e2');
            bar(92, H - 14, 92, G.weaponLv >= 4 ? 1 : G.weaponXP / (G.weaponLv * 10), '#77bbbe');
            const ready = G.superMeter >= 100 && G.superPhase === 'idle';
            label(ctx.t('galaxa.action_super') + ' C', 270, H - 23, 10, ready ? '#f6d38d' : '#88abc6', 'center');
            bar(217, H - 14, 106, G.superMeter / 100, ready ? '#f6d38d' : '#6a9fbd');
            if (G.combo > 0) {
                label('×' + G.comboMult + ' · ' + G.combo, 373, H - 23, 10, '#f6d38d', 'center');
                bar(339, H - 14, 68, G.comboTimer / ctx.COMBO_TIMEOUT, '#f6d38d');
            }
            if (G.activePU) {
                ctx.drawAnimation(c, 'pickup.' + G.activePU.type, W - 23, H - 20, G.animTime, 24);
                if (ctx.PU_DUR[G.activePU.type]) bar(W - 114, H - 14, 72, G.puTimer / ctx.PU_DUR[G.activePU.type], '#86c7c9');
            }
            if (G.achievementPopups.length) label(ctx.t('galaxa.achievement_unlocked') + ' · ' + G.achievementPopups[0].text, W / 2, 94, 11, '#f6d38d', 'center');
            if (G.evoChoiceOpen) {
                c.fillStyle = '#071222f5'; c.fillRect(28, 170, W - 56, 346);
                label(ctx.t('galaxa.weapon_evolution'), W / 2, 210, 22, '#e7d2a6', 'center');
                ['vulcan', 'cannon', 'beam'].forEach((id, i) => {
                    const y = 239 + i * 76, selected = ctx.evoSel() === i;
                    c.fillStyle = selected ? '#25405a' : '#101f31'; c.fillRect(48, y, W - 96, 63);
                    label(ctx.WEAPON_EVOS[id].name, W / 2, y + 25, 18, selected ? '#e7d2a6' : '#b6d1e2', 'center');
                    label(ctx.t('galaxa.evo_' + id), W / 2, y + 47, 12, '#88abc6', 'center');
                });
                label(ctx.t('galaxa.settings_hint'), W / 2, 494, 10, '#88abc6', 'center');
            }
        }
        Object.assign(ctx, { renderHUD, renderBeam });
    };
})();
