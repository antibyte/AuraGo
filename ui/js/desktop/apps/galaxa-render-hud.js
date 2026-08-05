(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createRenderHUD = function (ctx) {

        function drawSuperMeter(c, G) {
            if (G.superMeter == null) return;
            const x = GC.W / 2 - 50;
            const y = GC.H - 14;
            c.fillStyle = 'rgba(0,0,0,0.5)';
            c.fillRect(x - 2, y - 2, 104, 9);
            c.fillStyle = '#333';
            c.fillRect(x, y, 100, 5);
            const fill = (G.superMeter / 100) * 100;
            c.fillStyle = G.superPhase && G.superPhase !== 'idle' ? '#ffcc00' : '#888';
            c.fillRect(x, y, fill, 5);
        }

        function drawArchetypeHUD(c, G) {
            if (!G.archetype) return;
            const arch = GC.ARCHETYPES[G.archetype];
            if (!arch) return;
            c.fillStyle = 'rgba(0,0,0,0.5)';
            c.fillRect(GC.W - 110, 8, 102, 18);
            c.fillStyle = arch.hue;
            c.font = 'bold 10px monospace';
            c.textAlign = 'center';
            c.fillText(arch.name, GC.W - 59, 20);
        }

        function drawRankBanner(c, G) {
            if (!G.stageRank || !(G.fxRankSlamT > 0)) return;
            const colors = { 'S+': '#ffcc00', 'S': '#cccccc', 'A': '#44ccff', 'B': '#44ff44', 'C': '#888888' };
            const col = colors[G.stageRank] || '#fff';
            const y = GC.H * 0.4;
            const scale = Math.min(1, (G.tick % 60) / 30);
            c.save();
            c.translate(GC.W / 2, y);
            c.scale(scale, scale);
            c.fillStyle = col;
            c.font = 'bold 64px monospace';
            c.textAlign = 'center';
            c.shadowColor = col;
            c.shadowBlur = 16;
            c.fillText(G.stageRank, 0, 0);
            c.shadowBlur = 0;
            // Bonus icons
            c.font = '12px monospace';
            c.fillStyle = '#fff';
            const boni = G.stageBoni || {};
            c.fillText(`NO DAMAGE: ${boni.noDamageRun ? '✓' : '✗'}`, 0, 30);
            c.fillText(`SPEED: ${boni.speedDemon ? '✓' : '✗'}`, 0, 50);
            c.fillText(`PACIFIST: ${boni.pacifist ? '✓' : '✗'}`, 0, 70);
            c.restore();
        }

        ctx.drawSuperMeterHUD = drawSuperMeter;
        ctx.drawArchetypeHUD = drawArchetypeHUD;
        ctx.drawRankBanner = drawRankBanner;

                function renderBeam(tb) {
            ctx.c.shadowBlur = 8; ctx.c.shadowColor = '#4488ff';
            ctx.c.strokeStyle = '#4488ff'; ctx.c.lineWidth = 2; ctx.c.globalAlpha = 0.55;
            const w = 20 + Math.sin(ctx.tick * 0.15) * 8;
            ctx.c.beginPath();
            for (let i = 0; i < 8; i++) { const t2 = i / 8, y1 = tb.y + t2 * tb.h, y2 = tb.y + (t2 + 0.125) * tb.h, ww = w * (1 - t2 * 0.3); ctx.c.moveTo(tb.x - ww / 2, y1); ctx.c.lineTo(tb.x - ww * 0.4, y2); ctx.c.moveTo(tb.x + ww / 2, y1); ctx.c.lineTo(tb.x + ww * 0.4, y2); }
            ctx.c.stroke();
            ctx.c.globalAlpha = 1; ctx.c.shadowBlur = 0;
        }

        function renderHUD() {
            ctx.c.fillStyle = '#4488ff'; ctx.c.font = '12px "Courier New",monospace'; ctx.c.textAlign = 'left'; ctx.c.fillText(ctx.t('galaxa.score'), 10, 16);
            const _scoreText = ctx.formatScore ? ctx.formatScore(ctx.G.displayScore | 0) : String(ctx.G.displayScore | 0);
            const _isHigh = ctx.G.displayScore > ctx.G.hi;
            ctx.c.fillStyle = _isHigh ? '#ffcc00' : '#fff';
            ctx.c.font = 'bold 14px monospace';
            ctx.c.textAlign = 'left';
            if (_isHigh) { ctx.c.shadowColor = '#ffcc00'; ctx.c.shadowBlur = 8 + Math.sin(ctx.tick * 0.1) * 4; }
            ctx.c.fillText(_scoreText, 10, 34);
            ctx.c.shadowBlur = 0;
            if (ctx.G.comboMult > 1) {
                ctx.c.fillStyle = '#ffcc00'; ctx.c.font = 'bold 11px "Courier New",monospace';
                ctx.c.fillText('x' + ctx.G.comboMult, 10, 44);
            }
            if (ctx.G.combo > 0) {
                const comboRatio = Math.min(1, ctx.G.combo / 20);
                const cmx = ctx.W - 28, cmy = 54, cmr = 14;
                ctx.c.strokeStyle = '#333'; ctx.c.lineWidth = 2; ctx.c.beginPath(); ctx.c.arc(cmx, cmy, cmr, -Math.PI * 0.75, Math.PI * 0.75); ctx.c.stroke();
                const cmCol = ctx.G.combo >= 10 ? '#ff4444' : ctx.G.combo >= 5 ? '#ffcc00' : '#4488ff';
                ctx.c.strokeStyle = cmCol; ctx.c.lineWidth = 2;
                ctx.c.shadowBlur = 4; ctx.c.shadowColor = cmCol;
                ctx.c.beginPath(); ctx.c.arc(cmx, cmy, cmr, -Math.PI * 0.75, -Math.PI * 0.75 + comboRatio * Math.PI * 1.5); ctx.c.stroke();
                ctx.c.shadowBlur = 0;
                ctx.c.fillStyle = cmCol; ctx.c.font = 'bold 8px monospace'; ctx.c.textAlign = 'center';
                ctx.c.fillText(ctx.G.combo, cmx, cmy + 3);
            }
            ctx.c.fillStyle = '#4488ff'; ctx.c.textAlign = 'right'; ctx.c.fillText(ctx.t('galaxa.high_score'), ctx.W - 10, 16);
            ctx.c.fillStyle = '#ffcc00'; ctx.c.fillText(String(ctx.G.hi).padStart(8, '0'), ctx.W - 10, 32);
            const stagePulse = ctx.G.warpT > 0 ? 1 + Math.sin(ctx.tick * 0.15) * 0.3 : 1;
            ctx.c.save(); ctx.c.translate(ctx.W / 2, 16); ctx.c.scale(stagePulse, stagePulse);
            ctx.c.fillStyle = '#4488ff'; ctx.c.font = 'bold 12px "Courier New",monospace'; ctx.c.textAlign = 'center';
            ctx.c.fillText(ctx.t('galaxa.stage') + ' ' + ctx.G.stage, 0, 0);
            ctx.c.restore();
            if (ctx.getModeLabel && ctx.isGameMode && !ctx.isGameMode('classic')) {
                ctx.c.fillStyle = '#88ddff'; ctx.c.font = '9px "Courier New",monospace'; ctx.c.textAlign = 'center';
                ctx.c.fillText(ctx.getModeLabel(), ctx.W / 2, 30);
            }
            if (ctx.isGameMode && ctx.isGameMode('gauntlet') && ctx.G.gauntletWave) {
                ctx.c.fillStyle = '#ff8844'; ctx.c.font = 'bold 9px "Courier New",monospace';
                ctx.c.fillText('WAVE ' + ctx.G.gauntletWave + '/' + (ctx.GAUNTLET_WAVES ? ctx.GAUNTLET_WAVES.length : 12), ctx.W / 2, 42);
            }
            if (ctx.G.chal) {
                let _cr = 0; for (let _ci = 0; _ci < ctx.G.enemies.length; _ci++) if (ctx.G.enemies[_ci].st !== 'DEAD') _cr++;
                ctx.c.fillStyle = '#ff8800'; ctx.c.font = 'bold 10px "Courier New",monospace'; ctx.c.textAlign = 'center';
                ctx.c.fillText(ctx.t('galaxa.challenge_stage') + ' ' + _cr + '/' + ctx.G.chalTot, ctx.W / 2, 28);
            }
            let alive2cnt = 0; for (let _hi = 0; _hi < ctx.G.enemies.length; _hi++) { const _hh = ctx.G.enemies[_hi]; if (_hh.st !== 'DEAD' && _hh.type !== 'boss' && _hh.type !== 'miniboss') alive2cnt++; }
            if (alive2cnt > 0 && alive2cnt <= 5) {
                ctx.c.fillStyle = '#888'; ctx.c.font = '10px "Courier New",monospace'; ctx.c.textAlign = 'center';
                ctx.c.fillText(alive2cnt + ' LEFT', ctx.W / 2, ctx.G.chal ? 38 : 28);
            }
            if (ctx.G.weaponLv > 1) {
                ctx.c.fillStyle = '#44cc88'; ctx.c.font = '9px "Courier New",monospace'; ctx.c.textAlign = 'left';
                ctx.c.fillText('W' + ctx.G.weaponLv + (ctx.G.weaponEvo ? ' ' + (ctx.WEAPON_EVOS[ctx.G.weaponEvo] || {}).name : ''), 10, 54);
            }
            if (ctx.G.weaponLv < 4 && ctx.G.st === 'PLAYING') {
                const _xpNeed = ctx.G.weaponLv * 10;
                const _xpR = Math.min(1, ctx.G.weaponXP / _xpNeed);
                const _xpW = 40, _xpH = 2, _xpX = 10, _xpY = 58;
                ctx.c.fillStyle = '#222'; ctx.c.fillRect(_xpX, _xpY, _xpW, _xpH);
                ctx.c.fillStyle = '#44cc88'; ctx.c.fillRect(_xpX, _xpY, _xpW * _xpR, _xpH);
            }
            if (ctx.G.evoChoiceOpen) {
                ctx.c.fillStyle = 'rgba(0,0,0,0.85)'; ctx.c.fillRect(0, 0, ctx.W, ctx.H);
                ctx.c.textAlign = 'center';
                ctx.c.fillStyle = '#ffcc00'; ctx.c.font = 'bold 20px "Courier New",monospace';
                ctx.c.shadowBlur = 10; ctx.c.shadowColor = '#ffcc00';
                ctx.c.fillText('WEAPON EVOLUTION', ctx.W / 2, 120);
                ctx.c.shadowBlur = 0;
                const evos = ['vulcan', 'cannon', 'beam'];
                const _evoSel = ctx.evoSel ? ctx.evoSel() : 0;
                for (let i = 0; i < evos.length; i++) {
                    const evo = ctx.WEAPON_EVOS[evos[i]];
                    const y = 200 + i * 80;
                    const sel = i === _evoSel;
                    if (sel) {
                        ctx.c.fillStyle = 'rgba(68,136,255,0.15)';
                        ctx.c.fillRect(40, y - 20, ctx.W - 80, 60);
                        ctx.c.strokeStyle = '#4488ff'; ctx.c.lineWidth = 1;
                        ctx.c.strokeRect(40, y - 20, ctx.W - 80, 60);
                    }
                    ctx.c.fillStyle = sel ? evo.col : '#888';
                    ctx.c.font = sel ? 'bold 16px "Courier New",monospace' : '14px "Courier New",monospace';
                    if (sel) { ctx.c.shadowBlur = 6; ctx.c.shadowColor = evo.col; }
                    ctx.c.fillText(evo.name, ctx.W / 2, y);
                    ctx.c.shadowBlur = 0;
                    ctx.c.fillStyle = '#aaccee'; ctx.c.font = '11px "Courier New",monospace';
                    ctx.c.fillText(evo.desc, ctx.W / 2, y + 18);
                }
                ctx.c.fillStyle = '#666'; ctx.c.font = '10px "Courier New",monospace';
                ctx.c.fillText('\u2191\u2193 select  ENTER confirm', ctx.W / 2, ctx.H - 40);
            }
            if (ctx.G.activePU && ctx.G.activePU.type !== 'shield' && ctx.PU_DUR[ctx.G.activePU.type]) {
                const ratio = ctx.G.puTimer / ctx.PU_DUR[ctx.G.activePU.type];
                const isExpiringSoon = ctx.G.puTimer < 2000 && ctx.G.puTimer > 0;
                const puCol = ctx.PU_COL[ctx.G.activePU.type];
                const barW = ctx.W * 0.6, barH = 3, barX = ctx.W / 2 - barW / 2, barY = 4;
                ctx.c.fillStyle = '#222'; ctx.c.fillRect(barX, barY, barW, barH);
                ctx.c.fillStyle = puCol; ctx.c.fillRect(barX, barY, barW * ratio, barH);
                if ((ratio < 0.3 || isExpiringSoon) && Math.sin(ctx.tick * (isExpiringSoon ? 0.4 : 0.2)) > 0) { ctx.c.fillStyle = '#fff'; ctx.c.fillRect(barX, barY, barW * ratio, barH); }
                if (ctx.G.p && ctx.G.p.alive) {
                    const cx = ctx.G.p.x, cy = ctx.G.p.y, r = 32;
                    const startA = -Math.PI / 2, endA = startA + ratio * Math.PI * 2;
                    ctx.c.strokeStyle = puCol; ctx.c.lineWidth = 2; ctx.c.globalAlpha = 0.5;
                    ctx.c.shadowBlur = 4; ctx.c.shadowColor = puCol;
                    ctx.c.beginPath(); ctx.c.arc(cx, cy, r, startA, endA); ctx.c.stroke();
                    ctx.c.shadowBlur = 0; ctx.c.globalAlpha = 1;
                }
            }
            // NEW: Biome reveal cinematic — letterbox + sliding name plate
            if (ctx.G.biomeRevealT > 0) {
                const _br = ctx.G.biomeRevealT;
                const _phase = _br > 2000 ? 0 : _br > 1800 ? (_br - 1800) / 200 : _br < 400 ? _br / 400 : 1;
                const _ease = ctx.Easing.easeOutCubic(Math.min(1, _phase));
                // name plate
                if (_phase > 0) {
                    ctx.c.save();
                    const _py = ctx.H / 2;
                    const _plateW = ctx.W * 0.8, _plateX = ctx.W / 2 - _plateW / 2;
                    const _slide = (1 - _ease) * ctx.W;
                    ctx.c.globalAlpha = _ease;
                    ctx.c.fillStyle = 'rgba(0,0,0,0.6)'; ctx.c.fillRect(_plateX - _slide, _py - 24, _plateW, 48);
                    const _biomeDef = ctx.getBiomeForStage ? ctx.getBiomeForStage(ctx.G.stage) : null;
                    const _accent = _biomeDef ? _biomeDef.palette[1] : '#4488ff';
                    ctx.c.strokeStyle = _accent; ctx.c.lineWidth = 1; ctx.c.strokeRect(_plateX - _slide, _py - 24, _plateW, 48);
                    ctx.c.shadowBlur = 12; ctx.c.shadowColor = _accent;
                    ctx.c.fillStyle = _accent; ctx.c.font = 'bold 22px "Courier New",monospace'; ctx.c.textAlign = 'center';
                    ctx.c.fillText(ctx.G.biomeName || 'BIOME', ctx.W / 2 - _slide, _py);
                    ctx.c.shadowBlur = 0;
                    ctx.c.fillStyle = '#aaccee'; ctx.c.font = '10px "Courier New",monospace';
                    const _desc = _biomeDef ? _biomeDef.desc : '';
                    if (_desc) ctx.c.fillText(_desc, ctx.W / 2 - _slide, _py + 18);
                    ctx.c.restore(); ctx.c.globalAlpha = 1;
                }
            }
            // NEW: Bonus sub-stage banner
            if (ctx.G.bonusStage && ctx.G.bonusStageT > 0) {
                const _btA = ctx.G.bonusStageT < 1500 ? Math.min(1, (ctx.G.bonusStageT) / 300) : Math.min(1, (ctx.BONUS_STAGE_DURATION - ctx.G.bonusStageT) / 300);
                const _blink = Math.sin(ctx.tick * 0.15) > 0;
                ctx.c.save(); ctx.c.globalAlpha = Math.min(0.9, _btA + 0.3);
                ctx.c.fillStyle = _blink ? '#ffcc00' : '#ff8844'; ctx.c.font = 'bold 12px "Courier New",monospace'; ctx.c.textAlign = 'center';
                ctx.c.fillText('BONUS STAGE', ctx.W / 2, ctx.H - 70);
                ctx.c.fillStyle = '#fff'; ctx.c.font = '10px "Courier New",monospace';
                const _secs = Math.ceil(ctx.G.bonusStageT / 1000);
                ctx.c.fillText(_secs + 's', ctx.W / 2, ctx.H - 56);
                ctx.c.restore();
            }
            for (let i = 0; i < Math.min(ctx.G.lives, 5); i++) ctx.drawSp(ctx.c, ctx.SP.playerIcon || ctx.SP.player, ctx.SP.pC, 10 + i * 34, ctx.H - 32, false);
            if (ctx.G.activePU) {
                const puIconX = ctx.W - 20, puIconY = ctx.H - 20;
                const expiring = ctx.G.activePU.type !== 'shield' && ctx.PU_DUR[ctx.G.activePU.type] && ctx.G.puTimer < 2000;
                if (!expiring || Math.sin(ctx.tick * 0.2) > 0) {
                    ctx.c.fillStyle = ctx.PU_COL[ctx.G.activePU.type] || '#fff'; ctx.c.font = 'bold 9px monospace'; ctx.c.textAlign = 'right';
                    ctx.c.fillText(ctx.G.activePU.type.toUpperCase().substring(0, 4), puIconX, puIconY);
                }
            }
        }

        ctx.renderBeam = renderBeam;
        ctx.renderHUD = renderHUD;
    };
})();
