(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createRenderHUD = function (ctx) {
        function label(text, x, y, size, color, align) {
            const c = ctx.c; c.fillStyle = color || '#d5e8f7'; c.textAlign = align || 'left';
            c.font = size + 'px "Share Tech Mono", monospace'; c.fillText(text, x, y);
        }
        function bar(x, y, width, fraction, color, glow) {
            const c = ctx.c;
            const fillW = width * Math.max(0, Math.min(1, fraction));
            c.fillStyle = '#26354a'; c.fillRect(x, y, width, 4);
            c.fillStyle = color; c.fillRect(x, y, fillW, 4);
            if (fillW > 2) {
                c.fillStyle = 'rgba(255,255,255,0.28)';
                c.fillRect(x, y, fillW, 1);
            }
            if (glow) {
                c.strokeStyle = color;
                c.lineWidth = 1;
                c.strokeRect(x - 1, y - 1, width + 2, 6);
            }
        }
        function renderBeam(tb) {
            const c = ctx.c, time = (ctx.G && ctx.G.animTime) || 0;
            c.save();
            c.globalCompositeOperation = 'lighter';
            const grad = c.createLinearGradient(tb.x, tb.y, tb.x, tb.y + tb.h);
            grad.addColorStop(0, 'rgba(120, 220, 255, 0.72)');
            grad.addColorStop(0.6, 'rgba(57, 139, 199, 0.42)');
            grad.addColorStop(1, 'rgba(30, 90, 170, 0.12)');
            c.fillStyle = grad;
            c.beginPath();
            c.moveTo(tb.x - 10, tb.y);
            c.lineTo(tb.x + 10, tb.y);
            c.lineTo(tb.x + 26, tb.y + tb.h);
            c.lineTo(tb.x - 26, tb.y + tb.h);
            c.closePath();
            c.fill();
            // Descending concentric tractor energy arcs
            c.strokeStyle = '#99eeff';
            c.lineWidth = 1.5;
            const ringCount = 5;
            for (let i = 0; i < ringCount; i++) {
                const p = ((time * 0.0018 + i / ringCount) % 1);
                const ry = tb.y + p * tb.h;
                const rw = 10 + p * 16;
                c.globalAlpha = (1 - p * 0.65) * 0.75;
                c.beginPath();
                c.moveTo(tb.x - rw, ry);
                c.quadraticCurveTo(tb.x, ry + 6, tb.x + rw, ry);
                c.stroke();
            }
            c.restore();
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
            bar(92, H - 14, 92, G.weaponLv >= 4 ? 1 : G.weaponXP / (G.weaponLv * 10), '#77bbbe', G.overcharge > 0);
            const ready = G.superMeter >= 100 && G.superPhase === 'idle';
            const readyBlink = ready && Math.floor((G.animTime || 0) / 220) % 2 === 0;
            label(ctx.t('galaxa.action_super') + ' C', 270, H - 23, 10, ready ? (readyBlink ? '#fff4cc' : '#f6d38d') : '#88abc6', 'center');
            bar(217, H - 14, 106, G.superMeter / 100, ready ? '#f6d38d' : '#6a9fbd', ready);
            if (G.combo > 0) {
                label('×' + G.comboMult + ' · ' + G.combo, 373, H - 23, 10, '#f6d38d', 'center');
                bar(339, H - 14, 68, G.comboTimer / ctx.COMBO_TIMEOUT, '#f6d38d', G.comboMult >= 8);
            }
            if (G.activePU) {
                c.fillStyle = '#d5e8f7'; c.textAlign = 'right'; c.font = '11px monospace';
                c.fillText(ctx.t('galaxa.' + G.activePU.type), W - 12, H - 50, 220);
                ctx.drawAnimation(c, 'pickup.' + G.activePU.type, W - 23, H - 20, G.animTime, 24);
                if (ctx.PU_DUR[G.activePU.type]) bar(W - 114, H - 14, 72, G.puTimer / ctx.PU_DUR[G.activePU.type], '#86c7c9');
            }
            if (G.achievementPopups.length) label(ctx.t('galaxa.achievement_unlocked') + ' · ' + G.achievementPopups[0].text, W / 2, 94, 11, '#f6d38d', 'center');
            if (G.evoChoiceOpen) {
                c.fillStyle = '#071222f5'; c.fillRect(28, 170, W - 56, 376);
                label(ctx.t('galaxa.weapon_evolution'), W / 2, 210, 22, '#e7d2a6', 'center');
                ctx.WEAPON_EVO_IDS.forEach((id, i) => {
                    const y = 239 + i * 46, selected = ctx.evoSel() === i;
                    c.fillStyle = selected ? '#25405a' : '#101f31'; c.fillRect(48, y, W - 96, 40); c.fillStyle = ctx.WEAPON_EVOS[id].col; c.fillRect(48, y, 3, 40);
                    label(ctx.WEAPON_EVOS[id].name, W / 2, y + 17, 15, selected ? '#e7d2a6' : '#b6d1e2', 'center');
                    label(ctx.t('galaxa.evo_' + id), W / 2, y + 32, 10, '#88abc6', 'center');
                });
                label(ctx.t('galaxa.settings_hint'), W / 2, 239 + ctx.WEAPON_EVO_IDS.length * 46 + 14, 10, '#88abc6', 'center');
            }
        }
        Object.assign(ctx, { renderHUD, renderBeam });
    };
})();
