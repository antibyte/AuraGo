(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createRenderStage = function (ctx) {

        function drawStageOverlay(c, G) {
            // Per-biome foreground layer (placeholder)
            const biome = G.biome || 'nebula';
            if (biome === 'storm') {
                c.strokeStyle = 'rgba(255,255,68,0.3)';
                c.lineWidth = 1;
                const t = G.tick * 0.01;
                for (let i = 0; i < 3; i++) {
                    c.beginPath();
                    const x = ((t * 100 + i * 160) % GC.W);
                    c.moveTo(x, 0);
                    c.lineTo(x + 20, 30);
                    c.stroke();
                }
            }
        }

        ctx.drawStageOverlay = drawStageOverlay;

                function renderTitle() {
            for (const _tp of ctx.G.titleParts) { const _ta = Math.max(0, 1 - _tp.t / _tp.life); ctx.c.globalAlpha = _ta; ctx.c.fillStyle = _tp.col; ctx.c.shadowBlur = 6; ctx.c.shadowColor = _tp.col; ctx.c.fillRect(Math.floor(_tp.x), Math.floor(_tp.y), _tp.size, _tp.size); } ctx.c.globalAlpha = 1; ctx.c.shadowBlur = 0;
            ctx.c.textAlign = 'center';
            // Glowing title
            const titlePulse = 1 + Math.sin(ctx.tick * 0.04) * 0.03;
            ctx.c.save(); ctx.c.translate(ctx.W / 2, 180); ctx.c.scale(titlePulse, titlePulse);
            ctx.c.shadowBlur = 15; ctx.c.shadowColor = '#4488ff';
            ctx.c.fillStyle = '#4488ff'; ctx.c.font = 'bold 36px "Courier New",monospace'; ctx.c.fillText('GALAXA', 0, 0);
            ctx.c.shadowBlur = 0; ctx.c.restore();
            ctx.c.save(); ctx.c.translate(ctx.W / 2, 210); ctx.c.scale(titlePulse, titlePulse);
            ctx.c.shadowBlur = 10; ctx.c.shadowColor = '#ffcc00';
            ctx.c.fillStyle = '#ffcc00'; ctx.c.font = 'bold 20px "Courier New",monospace'; ctx.c.fillText('DELUXE', 0, 0);
            ctx.c.shadowBlur = 0; ctx.c.restore();
            if (Math.sin(ctx.tick * 0.08) > 0) { ctx.c.fillStyle = '#fff'; ctx.c.font = '14px "Courier New",monospace'; ctx.c.fillText(ctx.t('galaxa.insert_coin'), ctx.W / 2, 320); }
            ctx.c.fillStyle = '#4488ff'; ctx.c.font = '12px "Courier New",monospace'; ctx.c.fillText(ctx.t('galaxa.high_score'), ctx.W / 2, 260);
            ctx.c.fillStyle = '#ffcc00'; ctx.c.fillText(String(ctx.G.hi).padStart(8, '0'), ctx.W / 2, 280);
            if (ctx.G.hiScores.length) { ctx.c.fillStyle = '#aaccee'; ctx.c.font = '11px "Courier New",monospace'; let y = 380; ctx.c.fillText('RANK   NAME    SCORE    STAGE', ctx.W / 2, y); y += 18; ctx.G.hiScores.forEach((h, i) => { ctx.c.fillText((i + 1) + '    ' + h.name.padEnd(3) + '   ' + String(h.score).padStart(8) + '   ' + String(h.stage).padStart(3), ctx.W / 2, y); y += 16; }); }
            const achKeys = Object.keys(ctx.G.achievements).filter(k => ctx.G.achievements[k]);
            if (achKeys.length > 0) {
                ctx.c.fillStyle = '#ffcc00'; ctx.c.font = 'bold 10px "Courier New",monospace'; ctx.c.textAlign = 'center';
                ctx.c.fillText('ACHIEVEMENTS: ' + achKeys.length + '/' + Object.keys(ctx.ACHIEVEMENTS).length, ctx.W / 2, ctx.H - 70);
                ctx.c.fillStyle = '#888'; ctx.c.font = '9px "Courier New",monospace';
                const achNames = achKeys.slice(0, 4).map(k => ctx.ACHIEVEMENTS[k] ? ctx.ACHIEVEMENTS[k].name : k);
                ctx.c.fillText(achNames.join(' | '), ctx.W / 2, ctx.H - 55);
            }
            ctx.c.fillStyle = '#666'; ctx.c.font = '10px "Courier New",monospace'; ctx.c.fillText('ARROWS+SPACE  GAMEPAD  SHIFT+S=SETTINGS  M=MUTE', ctx.W / 2, ctx.H - 40);
            if (ctx.G.dailyStreak > 0) {
                ctx.c.fillStyle = '#ff88ff'; ctx.c.font = 'bold 10px "Courier New",monospace';
                ctx.c.fillText('DAILY STREAK: ' + ctx.G.dailyStreak + ' DAYS', ctx.W / 2, ctx.H - 55);
            }
            ctx.c.fillStyle = '#888'; ctx.c.font = '9px "Courier New",monospace';
            ctx.c.fillText('D=Daily Challenge', ctx.W / 2, ctx.H - 25);
            if (ctx.getModeLabel) {
                ctx.c.fillStyle = '#66ddff'; ctx.c.font = 'bold 11px "Courier New",monospace';
                ctx.c.fillText(ctx.t('galaxa.mode_label') + ': ' + ctx.getModeLabel(), ctx.W / 2, 300);
            }
        }

        function renderStageIntro() {
            ctx.c.textAlign = 'center';
            const sc = Math.max(1, 3 - (ctx.G.introTmr / 1200) * 2);
            ctx.c.save(); ctx.c.translate(ctx.W / 2, ctx.H / 2 - 20); ctx.c.scale(sc, sc);
            ctx.c.shadowBlur = 12; ctx.c.shadowColor = '#ffcc00';
            ctx.c.fillStyle = '#ffcc00'; ctx.c.font = 'bold 24px "Courier New",monospace';
            ctx.c.fillText(ctx.G.chal ? ctx.t('galaxa.challenge_stage') : ctx.t('galaxa.stage') + ' ' + ctx.G.stage, 0, 0);
            ctx.c.shadowBlur = 0; ctx.c.restore();
            ctx.c.fillStyle = '#fff'; ctx.c.font = '14px "Courier New",monospace'; ctx.c.fillText('READY', ctx.W / 2, ctx.H / 2 + 20);
        }

                function renderPause() {
            if (ctx.G.st !== 'PAUSED') return;
            ctx.c.fillStyle = 'rgba(0,0,0,0.75)'; ctx.c.fillRect(0, 0, ctx.W, ctx.H);
            ctx.c.textAlign = 'center'; ctx.c.fillStyle = '#ffcc00'; ctx.c.font = 'bold 26px "Courier New",monospace';
            ctx.c.shadowBlur = 10; ctx.c.shadowColor = '#ffcc00';
            ctx.c.fillText(ctx.t('galaxa.paused'), ctx.W / 2, ctx.H / 2 - 60);
            ctx.c.shadowBlur = 0;
            ctx.c.fillStyle = '#aaccee'; ctx.c.font = '12px "Courier New",monospace';
            ctx.c.fillText(ctx.t('galaxa.score') + ': ' + ctx.G.score + '  ' + ctx.t('galaxa.stage') + ': ' + ctx.G.stage, ctx.W / 2, ctx.H / 2 - 35);
            const items = [ctx.t('galaxa.resume'), ctx.t('galaxa.restart'), ctx.t('galaxa.quit')];
            items.forEach((it, i) => {
                ctx.c.fillStyle = i === ctx.G.pauseSel ? '#ffcc00' : '#888'; ctx.c.font = i === ctx.G.pauseSel ? 'bold 16px "Courier New",monospace' : '14px "Courier New",monospace';
                if (i === ctx.G.pauseSel) { ctx.c.shadowBlur = 6; ctx.c.shadowColor = '#ffcc00'; }
                ctx.c.fillText(it, ctx.W / 2, ctx.H / 2 + i * 30);
                ctx.c.shadowBlur = 0;
            });
        }

        function settingsItemLabel(item) {
            const keyMap = {
                sound: 'galaxa.sound', difficulty: 'galaxa.difficulty', volume: 'galaxa.volume',
                ship: 'galaxa.ship_select', mode: 'galaxa.mode_label', crt: 'galaxa.crt_effect',
                particles: 'galaxa.particle_density', shake: 'galaxa.shake_intensity', riskIt: 'galaxa.risk_it',
                adaptiveMusic: 'galaxa.adaptive_music', quit: 'galaxa.quit'
            };
            return ctx.t(keyMap[item.id] || item.label, item.label);
        }

        function settingsItemValue(item) {
            const s = ctx.settings;
            if (item.type === 'toggle') {
                if (item.key === 'mute') return ctx.G.muted ? 'OFF' : 'ON';
                return s[item.key] ? 'ON' : 'OFF';
            }
            if (item.type === 'slider') return s[item.key] + '%';
            if (item.type === 'action') return '';
            if (item.key === 'diff') return ctx.t('galaxa.' + s.diff, s.diff.toUpperCase());
            if (item.key === 'ship') return ctx.t('galaxa.' + s.ship, (ctx.SHIP_TYPES[s.ship] || ctx.SHIP_TYPES.classic).name);
            if (item.key === 'mode') return ctx.getModeLabel ? ctx.getModeLabel() : s.mode;
            if (item.key === 'particles') return ctx.t('galaxa.' + s.particles, s.particles.toUpperCase());
            if (item.key === 'shake') {
                return s.shake === 0 ? 'OFF' : s.shake === 0.25 ? 'LOW' : s.shake === 0.5 ? 'MED' : s.shake === 0.75 ? 'HIGH' : 'MAX';
            }
            return String(s[item.key] || '');
        }

        function renderSettings() {
            ctx.c.fillStyle = 'rgba(0,0,0,0.88)'; ctx.c.fillRect(0, 0, ctx.W, ctx.H);
            ctx.c.textAlign = 'center'; ctx.c.fillStyle = '#ffcc00'; ctx.c.font = 'bold 22px "Courier New",monospace';
            ctx.c.shadowBlur = 10; ctx.c.shadowColor = '#ffcc00';
            ctx.c.fillText(ctx.t('galaxa.settings'), ctx.W / 2, 80);
            ctx.c.shadowBlur = 0;
            const items = ctx.SETTINGS_ITEMS || [];
            items.forEach((it, i) => {
                const sel = i === ctx.G.settingsSel;
                const label = settingsItemLabel(it);
                const val = settingsItemValue(it);
                ctx.c.fillStyle = sel ? '#ffcc00' : '#888'; ctx.c.font = sel ? 'bold 14px "Courier New",monospace' : '12px "Courier New",monospace';
                if (sel) { ctx.c.shadowBlur = 6; ctx.c.shadowColor = '#ffcc00'; }
                ctx.c.fillText(label + (val ? ': ' + val : ''), ctx.W / 2, 130 + i * 36);
                ctx.c.shadowBlur = 0;
                if (it.type === 'slider' && sel) {
                    const bw = 200, bh = 8, bx = ctx.W / 2 - bw / 2, by = 138 + i * 36;
                    ctx.c.fillStyle = '#222'; ctx.c.fillRect(bx, by, bw, bh);
                    ctx.c.fillStyle = '#4488ff'; ctx.c.fillRect(bx, by, bw * ctx.settings.vol / 100, bh);
                    ctx.c.strokeStyle = '#4488ff'; ctx.c.lineWidth = 1; ctx.c.strokeRect(bx - 1, by - 1, bw + 2, bh + 2);
                }
            });
            ctx.c.fillStyle = '#666'; ctx.c.font = '10px "Courier New",monospace';
            ctx.c.fillText('\u2191\u2193 select  \u2190\u2192 change  ENTER confirm', ctx.W / 2, 430);
            ctx.c.fillText('ARROWS+SPACE  GAMEPAD D-PAD+A', ctx.W / 2, 450);
        }
        ctx.renderTitle = renderTitle;
        ctx.renderStageIntro = renderStageIntro;
        ctx.renderPause = renderPause;
        ctx.renderSettings = renderSettings;
    };
})();
