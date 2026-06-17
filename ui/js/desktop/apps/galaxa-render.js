(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createRenderer = function (ctx) {
        function renderFlame(cv, fx, fy, intensity, tk) {
            const f1 = Math.abs(Math.sin(tk * 0.35 + fx * 0.08)) * 4;
            const f2 = Math.abs(Math.sin(tk * 0.55 + fx * 0.12)) * 3;
            const f3 = Math.abs(Math.sin(tk * 0.7 + fx * 0.2)) * 2;
            cv.fillStyle = 'rgba(255,255,240,' + intensity + ')';
            cv.fillRect(Math.floor(fx), Math.floor(fy), 2, 3);
            cv.fillStyle = 'rgba(255,230,60,' + (intensity * 0.95) + ')';
            cv.fillRect(Math.floor(fx - 1), Math.floor(fy + 2), 4, 2 + Math.ceil(f1 * 0.5));
            cv.fillStyle = 'rgba(255,140,20,' + (intensity * 0.85) + ')';
            cv.fillRect(Math.floor(fx - 1), Math.floor(fy + 4), 4, 3 + Math.ceil(f1));
            cv.fillStyle = 'rgba(255,60,10,' + (intensity * 0.6) + ')';
            cv.fillRect(Math.floor(fx), Math.floor(fy + 7), 3, 2 + Math.ceil(f2));
            cv.fillStyle = 'rgba(200,40,10,' + (intensity * 0.35) + ')';
            cv.fillRect(Math.floor(fx), Math.floor(fy + 9), 2, 2 + Math.ceil(f3));
            cv.fillStyle = 'rgba(160,20,10,' + (intensity * 0.15) + ')';
            cv.fillRect(Math.floor(fx + 0.5), Math.floor(fy + 11), 1, 1 + Math.ceil(f3 * 0.5));
        }

        function renderFrame(dt) {
            ctx.c.save(); ctx.c.setTransform(ctx.scale, 0, 0, ctx.scale, 0, 0);
            let sx = 0, sy = 0; if (ctx.G.shkT > 0 && ctx.settings.shake > 0) { const _decay = Math.min(1, ctx.G.shkT / 200); const _sm = ctx.settings.shake || 1; sx = (Math.random() - 0.5) * ctx.G.shkM * _decay * _sm; sy = (Math.random() - 0.5) * ctx.G.shkM * _decay * _sm; }
            ctx.c.translate(sx, sy); ctx.c.fillStyle = '#000'; ctx.c.fillRect(-5, -5, ctx.W + 10, ctx.H + 10);
            ctx.drawNebula(ctx.c); ctx.drawStars(ctx.c);
            if (ctx.G.chromAb > 0) {
                const ca = Math.min(1, ctx.G.chromAb / 200);
                const offset = Math.round(ca * 3);
                ctx.c.globalCompositeOperation = 'lighter';
                ctx.c.globalAlpha = ca * 0.08;
                ctx.c.fillStyle = '#ff0000'; ctx.c.fillRect(offset, 0, ctx.W, ctx.H);
                ctx.c.fillStyle = '#0000ff'; ctx.c.fillRect(-offset, 0, ctx.W, ctx.H);
                ctx.c.globalAlpha = ca * 0.04;
                ctx.c.fillStyle = '#00ff00'; ctx.c.fillRect(0, offset, ctx.W, ctx.H);
                ctx.c.globalCompositeOperation = 'source-over';
                ctx.c.globalAlpha = 1;
            }
            if (ctx.G.damageVignetteT > 0) {
                const _dv = Math.min(1, ctx.G.damageVignetteT / 400) * 0.65;
                ctx.c.save();
                const _dvg = ctx.c.createRadialGradient(ctx.W * 0.5, ctx.H * 0.5, ctx.H * 0.2, ctx.W * 0.5, ctx.H * 0.5, ctx.H * 0.85);
                _dvg.addColorStop(0, 'rgba(180,0,0,0)');
                _dvg.addColorStop(1, 'rgba(220,0,0,' + _dv + ')');
                ctx.c.fillStyle = _dvg; ctx.c.fillRect(0, 0, ctx.W, ctx.H);
                ctx.c.restore();
            }
            if (ctx.G.activePU && ctx.G.activePU.type !== 'shield' && ctx.G.p && ctx.G.p.alive) {
                const egCol = ctx.PU_COL[ctx.G.activePU.type] || '#ffffff';
                const egGrad = ctx.cachedRadialGradient(ctx.c, 'powerup-edge:' + egCol, ctx.W / 2, ctx.H / 2, ctx.W * 0.25, ctx.W * 0.75, [
                    [0, 'rgba(0,0,0,0)'],
                    [1, egCol + '55']
                ]);
                ctx.c.globalAlpha = 0.5 + Math.sin(ctx.tick * 0.05) * 0.2;
                ctx.c.fillStyle = egGrad; ctx.c.fillRect(0, 0, ctx.W, ctx.H);
                ctx.c.globalAlpha = 1;
            }
            if (ctx.G.warpFlash > 0) { ctx.c.fillStyle = 'rgba(255,255,255,' + (ctx.G.warpFlash / 50) + ')'; ctx.c.fillRect(0, 0, ctx.W, ctx.H); }
            // NEW: Alternative screen transitions
            if (ctx.G.swipeT > 0) {
                const progress = 1 - (ctx.G.swipeT / 1200);
                const wipeX = ctx.G.swipeDir > 0 ? ctx.W * progress : ctx.W * (1 - progress);
                ctx.c.fillStyle = '#000';
                if (ctx.G.swipeDir > 0) ctx.c.fillRect(0, 0, wipeX, ctx.H);
                else ctx.c.fillRect(wipeX, 0, ctx.W - wipeX, ctx.H);
                ctx.c.strokeStyle = '#4488ff'; ctx.c.lineWidth = 2;
                ctx.c.beginPath(); ctx.c.moveTo(wipeX, 0); ctx.c.lineTo(wipeX, ctx.H); ctx.c.stroke();
            }
            if (ctx.G.portalT > 0) {
                const progress = 1 - (ctx.G.portalT / 1400);
                ctx.G.portalR = progress * Math.max(ctx.W, ctx.H) * 0.8;
                ctx.c.save();
                ctx.c.beginPath(); ctx.c.arc(ctx.W / 2, ctx.H / 2, ctx.G.portalR, 0, Math.PI * 2); ctx.c.clip();
                ctx.c.fillStyle = '#000'; ctx.c.fillRect(0, 0, ctx.W, ctx.H);
                ctx.c.restore();
                ctx.c.strokeStyle = '#ffcc00'; ctx.c.lineWidth = 3; ctx.c.shadowBlur = 15; ctx.c.shadowColor = '#ffcc00';
                ctx.c.beginPath(); ctx.c.arc(ctx.W / 2, ctx.H / 2, ctx.G.portalR, 0, Math.PI * 2); ctx.c.stroke();
                ctx.c.shadowBlur = 0;
            }
            if (ctx.G.glitchT > 0) {
                const progress = 1 - (ctx.G.glitchT / 1000);
                ctx.c.save();
                for (const strip of ctx.G.glitchStrips) {
                    strip.offset += (strip.targetOffset - strip.offset) * 0.1;
                    const alpha = Math.abs(Math.sin(progress * Math.PI * 3 + strip.y * 0.1)) * 0.8;
                    ctx.c.globalAlpha = alpha;
                    ctx.c.fillStyle = '#000';
                    ctx.c.fillRect(0, strip.y, ctx.W, ctx.H / 12);
                    if (Math.random() < 0.3) {
                        ctx.c.fillStyle = ['#ff0000', '#00ff00', '#0000ff', '#ffffff'][Math.floor(Math.random() * 4)];
                        ctx.c.fillRect(strip.offset, strip.y, 2, ctx.H / 12);
                    }
                }
                ctx.c.restore();
            }
            if (ctx.G.flashT > 0) { ctx.c.fillStyle = 'rgba(255,255,255,' + (ctx.G.flashT > 30 ? 0.5 : ctx.G.flashT / 60) + ')'; ctx.c.fillRect(0, 0, ctx.W, ctx.H); }
            if (ctx.G.stageWipeT > 0) { const wp = ctx.G.stageWipeT / 400; ctx.c.fillStyle = '#000'; ctx.c.fillRect(0, 0, ctx.W, ctx.H * wp); ctx.c.fillRect(0, ctx.H * (1 - wp), ctx.W, ctx.H * wp); }
            if (ctx.G.levelSkipTimer > 0) {
                const _lsA = Math.min(1, ctx.G.levelSkipTimer / 500) * 0.15;
                ctx.c.fillStyle = 'rgba(255,136,255,' + _lsA + ')'; ctx.c.fillRect(0, 0, ctx.W, ctx.H);
            }
            if (ctx.G.st === 'TITLE' && !ctx.G.attract) ctx.renderTitle();
            else if (ctx.G.st === 'STAGE_INTRO') { ctx.renderGame(); ctx.renderStageIntro(); }
            else if (ctx.G.st === 'SETTINGS') ctx.renderSettings();
            else if (ctx.G.st === 'PAUSED') { ctx.renderGame(); ctx.renderPause(); }
            else ctx.renderGame();
            ctx.c.restore();
        }

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
            if (Math.sin(ctx.tick * 0.08) > 0) { ctx.c.fillStyle = '#fff'; ctx.c.font = '14px "Courier New",monospace'; ctx.c.fillText(ctx.t('galaxa.insert_coin', 'PRESS START'), ctx.W / 2, 320); }
            ctx.c.fillStyle = '#4488ff'; ctx.c.font = '12px "Courier New",monospace'; ctx.c.fillText(ctx.t('galaxa.high_score', 'HIGH SCORE'), ctx.W / 2, 260);
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
        }

        function renderStageIntro() {
            ctx.c.textAlign = 'center';
            const sc = Math.max(1, 3 - (ctx.G.introTmr / 1200) * 2);
            ctx.c.save(); ctx.c.translate(ctx.W / 2, ctx.H / 2 - 20); ctx.c.scale(sc, sc);
            ctx.c.shadowBlur = 12; ctx.c.shadowColor = '#ffcc00';
            ctx.c.fillStyle = '#ffcc00'; ctx.c.font = 'bold 24px "Courier New",monospace';
            ctx.c.fillText(ctx.G.chal ? ctx.t('galaxa.challenge_stage', 'CHALLENGE STAGE') : ctx.t('galaxa.stage', 'STAGE') + ' ' + ctx.G.stage, 0, 0);
            ctx.c.shadowBlur = 0; ctx.c.restore();
            ctx.c.fillStyle = '#fff'; ctx.c.font = '14px "Courier New",monospace'; ctx.c.fillText('READY', ctx.W / 2, ctx.H / 2 + 20);
        }

        function renderGame() {
            const p = ctx.G.p;

            // Beat-synced background pulse
            if (ctx.G.beatPhase > 0.88 && ctx.nebulaCv) {
                const _bp = (ctx.G.beatPhase - 0.88) * 8.33 * 0.06;
                ctx.c.globalAlpha = _bp; ctx.c.fillStyle = '#1a0033'; ctx.c.fillRect(0, 0, ctx.W, ctx.H); ctx.c.globalAlpha = 1;
            }

            if (ctx.G.bossWarningT > 0) {
                const flash = Math.sin(ctx.G.bossWarningT * 0.01) > 0;
                ctx.c.fillStyle = flash ? 'rgba(255,0,0,0.15)' : 'rgba(255,0,0,0.05)';
                ctx.c.fillRect(0, 0, ctx.W, ctx.H);
                ctx.c.shadowBlur = 10; ctx.c.shadowColor = '#ff0000';
                ctx.c.fillStyle = '#ff4444'; ctx.c.font = 'bold 28px "Courier New",monospace'; ctx.c.textAlign = 'center';
                ctx.c.fillText('WARNING', ctx.W / 2, ctx.H / 2 - 20);
                ctx.c.shadowBlur = 0;
                ctx.wrapEl.classList.add('galaxa-boss-warning');
            } else {
                ctx.wrapEl.classList.remove('galaxa-boss-warning');
            }

            for (const tr of ctx.G.trails) {
                if (tr.col.startsWith('rgba')) { ctx.c.fillStyle = tr.col; }
                else {
                    const alpha = Math.max(0, 1 - tr.t / tr.life);
                    ctx.c.fillStyle = tr.col; ctx.c.globalAlpha = alpha * 0.6;
                }
                ctx.c.fillRect(Math.floor(tr.x), Math.floor(tr.y), tr.size || 2, tr.size || 2);
            }
            ctx.c.globalAlpha = 1;

            for (const dp of ctx.G.deathParts) {
                const alpha = Math.max(0, 1 - dp.t / dp.life);
                ctx.c.globalAlpha = alpha;
                ctx.c.save(); ctx.c.translate(dp.x, dp.y); ctx.c.rotate(dp.rot);
                ctx.c.fillStyle = dp.col;
                ctx.c.fillRect(-dp.sz / 2, -dp.sz / 2, dp.sz, dp.sz);
                ctx.c.restore();
            }
            ctx.c.globalAlpha = 1;

            if (ctx.G.muzzleT > 0 && p.alive) {
                ctx.c.globalAlpha = ctx.G.muzzleT / 50;
                ctx.c.fillStyle = '#ffff88';
                ctx.c.fillRect(Math.floor(p.x - 2), Math.floor(p.y - 14), 4, 4);
                ctx.c.fillRect(Math.floor(p.x - 1), Math.floor(p.y - 16), 2, 2);
                ctx.c.globalAlpha = 1;
            }

            if (p.alive) {
                ctx.c.save(); ctx.c.translate(p.x, p.y); ctx.c.rotate(ctx.G.shipTilt); ctx.c.translate(-p.x, -p.y);
                const _egGlow = ctx.G.activePU && ctx.PU_COL[ctx.G.activePU.type] ? ctx.PU_COL[ctx.G.activePU.type] : '#ff6600';
                const _egInt = 0.25 + Math.sin(ctx.tick * 0.15) * 0.15;
                const _eglG = ctx.cachedRadialGradient(ctx.c, 'engGlow:' + _egGlow, p.x, p.y + 14, 0, 18, [[0, _egGlow + '88'], [0.5, _egGlow + '22'], [1, 'transparent']]);
                ctx.c.globalAlpha = _egInt; ctx.c.fillStyle = _eglG; ctx.c.fillRect(p.x - 20, p.y - 4, 40, 36); ctx.c.globalAlpha = 1;
                if (p.inv > 0) {
                    const rpc = ctx.rainbowPC();
                    ctx.drawSp(ctx.c, ctx.SP.player, rpc, p.x - 12, p.y - 12, false, true);
                    if (p.dual) ctx.drawSp(ctx.c, ctx.SP.player, rpc, p.x + 28, p.y - 12, false, true);
                } else {
                    ctx.drawSp(ctx.c, ctx.SP.player, ctx.SP.pC, p.x - 12, p.y - 12, false);
                    if (p.dual) ctx.drawSp(ctx.c, ctx.SP.player, ctx.SP.pC, p.x + 28, p.y - 12, false);
                }
                if (p.alive) {
                    const eg = 0.5 + Math.sin(ctx.tick * 0.15) * 0.3;
                    const flameGlowCol = ctx.G.activePU && ctx.PU_COL[ctx.G.activePU.type] ? ctx.PU_COL[ctx.G.activePU.type] : '#ff6600';
                    renderFlame(ctx.c, p.x - 6, p.y + 11, eg, ctx.tick);
                    renderFlame(ctx.c, p.x + 3, p.y + 11, eg, ctx.tick);
                    if (p.dual) {
                        renderFlame(ctx.c, p.x + 28, p.y + 11, eg, ctx.tick);
                        renderFlame(ctx.c, p.x + 34, p.y + 11, eg, ctx.tick);
                    }
                }
                ctx.c.restore();
            }
            if (p.cap) ctx.drawSp(ctx.c, ctx.SP.player, ctx.SP.pC, p.cap.x - 12, p.cap.y - 12, false);
            if (ctx.G.shieldHits > 0 && p.alive) {
                ctx.c.strokeStyle = '#4488ff'; ctx.c.lineWidth = 1.5; ctx.c.globalAlpha = 0.5 + Math.sin(ctx.tick * 0.1) * 0.2;
                ctx.c.shadowBlur = 10; ctx.c.shadowColor = '#4488ff';
                ctx.c.beginPath(); ctx.c.arc(p.x, p.y, 18, 0, Math.PI * 2); ctx.c.stroke(); ctx.c.shadowBlur = 0; ctx.c.globalAlpha = 1;
                for (let i = 0; i < ctx.G.shieldHits; i++) {
                    const a = ctx.tick * 0.05 + i * 2.1;
                    ctx.c.fillStyle = '#4488ff'; ctx.c.fillRect(Math.floor(p.x + Math.cos(a) * 18 - 1), Math.floor(p.y + Math.sin(a) * 18 - 1), 3, 3);
                }
            }
            if (ctx.G.activePU && ctx.G.activePU.type !== 'shield' && p.alive) {
                const auraCol = ctx.PU_COL[ctx.G.activePU.type];
                const auraPulse = 0.15 + Math.sin(ctx.tick * 0.08) * 0.1;
                ctx.c.shadowBlur = 12; ctx.c.shadowColor = auraCol;
                ctx.c.strokeStyle = auraCol; ctx.c.lineWidth = 1; ctx.c.globalAlpha = auraPulse;
                ctx.c.beginPath(); ctx.c.arc(p.x, p.y, 20 + Math.sin(ctx.tick * 0.12) * 3, 0, Math.PI * 2); ctx.c.stroke();
                ctx.c.shadowBlur = 0; ctx.c.globalAlpha = 1;
            }

            if (ctx.G.activePU && (ctx.G.activePU.type === 'laser' || ctx.G.activePU.type === 'mega_laser') && ctx.G.p.alive && ctx.G.muzzleT > 0) {
                const _lAlpha = ctx.G.muzzleT / 50;
                let _nearE = null, _nearD2 = Infinity;
                for (let _li = 0; _li < ctx.G.enemies.length; _li++) { const _le = ctx.G.enemies[_li]; if (_le.st === 'DEAD' || _le.y <= 0 || _le.y >= ctx.H) continue; const _ld = (_le.x-ctx.G.p.x)*(_le.x-ctx.G.p.x)+(_le.y-ctx.G.p.y)*(_le.y-ctx.G.p.y); if (_ld < _nearD2) { _nearD2 = _ld; _nearE = _le; } }
                if (_nearE && _nearD2 < 220 * 220) {
                    const _lx1 = ctx.G.p.x, _ly1 = ctx.G.p.y - 8, _lx2 = _nearE.x, _ly2 = _nearE.y;
                    ctx.c.globalAlpha = _lAlpha * 0.7;
                    ctx.c.strokeStyle = ctx.G.activePU.type === 'mega_laser' ? '#ffffff' : '#aaccff';
                    ctx.c.lineWidth = ctx.G.activePU.type === 'mega_laser' ? 2 : 1;
                    ctx.c.shadowBlur = 8; ctx.c.shadowColor = '#4488ff';
                    ctx.c.beginPath(); ctx.c.moveTo(_lx1, _ly1);
                    for (let _li = 1; _li < 6; _li++) { const _lt = _li / 6; ctx.c.lineTo(_lx1 + (_lx2-_lx1)*_lt + (Math.random()-0.5)*16, _ly1 + (_ly2-_ly1)*_lt + (Math.random()-0.5)*16); }
                    ctx.c.lineTo(_lx2, _ly2); ctx.c.stroke();
                    ctx.c.shadowBlur = 0; ctx.c.globalAlpha = 1;
                }
            }
            for (const _pr of ctx.G.plasmaRings) {
                const _prAlpha = Math.max(0, 1 - _pr.t / _pr.dur) * 0.75;
                ctx.c.globalAlpha = _prAlpha;
                ctx.c.strokeStyle = _pr.col;
                ctx.c.lineWidth = Math.max(1, 3 * (1 - _pr.t / _pr.dur));
                ctx.c.shadowBlur = 14; ctx.c.shadowColor = _pr.col;
                ctx.c.beginPath(); ctx.c.arc(_pr.x, _pr.y, _pr.r, 0, Math.PI * 2); ctx.c.stroke();
                ctx.c.shadowBlur = 0;
            }
            ctx.c.globalAlpha = 1;
            // bullet trails (no shadow)
            for (const b of ctx.G.bul) {
                if (!b.laser) {
                    ctx.c.fillStyle = 'rgba(255,255,136,0.3)';
                    ctx.c.fillRect(Math.floor(b.x - 1), Math.floor(b.y + 3), 2, 4);
                }
            }
            // player bullets — shadow set once for the whole batch
            ctx.c.shadowColor = '#ffff88'; ctx.c.shadowBlur = 6;
            for (const b of ctx.G.bul) {
                if (!b.laser) {
                    ctx.c.fillStyle = '#ffff88';
                    ctx.c.fillRect(Math.floor(b.x - 1), Math.floor(b.y - 3), 2, 6);
                    ctx.c.globalAlpha = 0.4;
                    ctx.c.fillStyle = '#ffff44';
                    ctx.c.fillRect(Math.floor(b.x - 2), Math.floor(b.y - 4), 4, 8);
                    ctx.c.globalAlpha = 1;
                }
            }
            ctx.c.shadowBlur = 0;
            // laser bullets — shadow set once for the whole batch
            ctx.c.shadowColor = '#aaccff'; ctx.c.shadowBlur = 14;
            for (const b of ctx.G.bul) {
                if (b.laser) {
                    ctx.c.fillStyle = '#ffffff'; ctx.c.fillRect(Math.floor(b.x - 2), Math.floor(b.y - 7), 4, 14);
                    ctx.c.fillStyle = 'rgba(170,200,255,0.5)'; ctx.c.fillRect(Math.floor(b.x - 3), Math.floor(b.y - 9), 6, 18);
                    ctx.c.fillStyle = 'rgba(100,150,255,0.25)'; ctx.c.fillRect(Math.floor(b.x - 4), Math.floor(b.y - 11), 8, 22);
                }
            }
            ctx.c.shadowBlur = 0;
            // enemy bullet trails (no shadow)
            for (const b of ctx.G.ebul) {
                const trCol = b.kind === 'plasma' ? 'rgba(68,255,136,0.3)' : b.kind === 'spiral' ? 'rgba(68,255,255,0.28)' : b.kind === 'mine' ? 'rgba(204,102,255,0.35)' : b.kind === 'hunter' ? 'rgba(255,136,68,0.3)' : 'rgba(255,68,68,0.25)';
                ctx.c.fillStyle = trCol;
                const tw = b.w || 2, th = b.h || 4;
                ctx.c.fillRect(Math.floor(b.x - tw / 2), Math.floor(b.y + 2), tw, th);
            }
            // enemy bullets — batched by kind
            const ebKinds = [
                { kinds: ['plasma'], fill: '#66ffaa', glow: '#44ff88', shadow: '#00cc66' },
                { kinds: ['spiral', 'sniper'], fill: '#66ffff', glow: '#44dddd', shadow: '#00cccc' },
                { kinds: ['mine'], fill: '#dd88ff', glow: '#cc66ff', shadow: '#aa44cc' },
                { kinds: ['hunter'], fill: '#ff8844', glow: '#ff6622', shadow: '#ff4400' },
                { kinds: ['bolt'], fill: '#ff6666', glow: '#ff4444', shadow: '#ff4444' }
            ];
            for (const batch of ebKinds) {
                ctx.c.shadowColor = batch.shadow; ctx.c.shadowBlur = 6;
                for (const b of ctx.G.ebul) {
                    if (!batch.kinds.includes(b.kind || 'bolt')) continue;
                    const bw = b.w || 2, bh = b.h || 6;
                    if (b.kind === 'mine') {
                        const pulse = 0.7 + Math.sin(ctx.tick * 0.2 + b.t * 0.01) * 0.3;
                        ctx.c.globalAlpha = pulse;
                        ctx.c.fillStyle = batch.fill;
                        ctx.c.beginPath(); ctx.c.arc(b.x, b.y, 4, 0, Math.PI * 2); ctx.c.fill();
                        ctx.c.globalAlpha = 1;
                        continue;
                    }
                    ctx.c.fillStyle = batch.fill;
                    ctx.c.fillRect(Math.floor(b.x - bw / 2), Math.floor(b.y - bh / 2), bw, bh);
                    ctx.c.globalAlpha = 0.35;
                    ctx.c.fillStyle = batch.glow;
                    ctx.c.fillRect(Math.floor(b.x - bw / 2 - 1), Math.floor(b.y - bh / 2 - 1), bw + 2, bh + 2);
                    ctx.c.globalAlpha = 1;
                }
            }
            ctx.c.shadowBlur = 0;

            // Boss telegraph lines — show dive path before attack
            if (ctx.G.p && ctx.G.p.alive) {
                ctx.c.setLineDash([2, 4]);
                for (const _te of ctx.G.enemies) {
                    if (_te.st !== 'DIVING' || _te.type === 'bee' || ctx.G.freezeT > 0) continue;
                    const _showTel = _te.type === 'hunter' || _te.type === 'sniper' || _te.type === 'lasher' || (_te.sTmr !== undefined && _te.sTmr <= 250 && _te.sTmr >= 0);
                    if (!_showTel) continue;
                    const _ta = (1 - _te.sTmr / 250) * 0.5;
                    ctx.c.globalAlpha = _ta; ctx.c.strokeStyle = '#ff4444'; ctx.c.lineWidth = 1;
                    ctx.c.beginPath(); ctx.c.moveTo(_te.x, _te.y + 8); ctx.c.lineTo(ctx.G.p.x, ctx.G.p.y - 8); ctx.c.stroke();
                }
                ctx.c.setLineDash([]); ctx.c.globalAlpha = 1;
            }

            for (const e of ctx.G.enemies) {
                if (e.st === 'DEAD') continue;
                if (e.st === 'DIVING') {
                    ctx.c.globalAlpha = 0.12;
                    let sp, cols;
                    const _bossVar = (ctx.G.stage - 1) % 3;
                    const _bossCols = _bossVar === 1 ? ctx.SP.bossRedC : _bossVar === 2 ? ctx.SP.bossBlueC : ctx.SP.bossC;
                    if (e.type === 'bee') { sp = ctx.SP.bee[e.fr]; cols = ctx.SP.bC; } else if (e.type === 'butterfly') { sp = ctx.SP.bf[e.fr]; cols = ctx.SP.bfC; } else if (e.type === 'stalker') { sp = ctx.SP.stalker[e.fr]; cols = ctx.SP.stalkerC; } else if (e.type === 'sniper') { sp = ctx.SP.sniper[e.fr]; cols = ctx.SP.sniperC; } else if (e.type === 'hunter') { sp = ctx.SP.hunter[e.fr]; cols = ctx.SP.hunterC; } else if (e.type === 'spinner') { sp = ctx.SP.spinner[e.fr]; cols = ctx.SP.spinnerC; } else if (e.type === 'bomber') { sp = ctx.SP.bomber[e.fr]; cols = ctx.SP.bomberC; } else if (e.type === 'lasher') { sp = ctx.SP.lasher[e.fr]; cols = ctx.SP.lasherC; } else if (e.type === 'weaver') { sp = ctx.SP.weaver[e.fr]; cols = ctx.SP.weaverC; } else if (e.type === 'splitter') { sp = ctx.SP.splitter[e.fr]; cols = ctx.SP.splitterC; } else if (e.type === 'shield_bee') { sp = ctx.SP.shield_bee[e.fr]; cols = ctx.SP.shield_beeC; } else if (e.type === 'kamikaze') { sp = ctx.SP.kamikaze[e.fr]; cols = ctx.SP.kamikazeC; } else if (e.type === 'carrier') { sp = ctx.SP.carrier[e.fr]; cols = ctx.SP.carrierC; } else if (e.type === 'teleporter') { sp = ctx.SP.teleporter[e.fr]; cols = ctx.SP.teleporterC; } else if (e.type === 'miniboss') { sp = e.hp <= 1 ? ctx.SP.bossCrit : e.hp <= Math.ceil(e.maxHp / 2) ? ctx.SP.bossHit : ctx.SP.boss; cols = _bossCols; } else { sp = e.hp <= 1 ? ctx.SP.bossCrit : e.hp <= Math.ceil(e.maxHp / 2) ? ctx.SP.bossHit : ctx.SP.boss; cols = _bossCols; }
                    ctx.drawSp(ctx.c, sp, cols, e.x - 12, e.y - 18, false);
                    ctx.drawSp(ctx.c, sp, cols, e.x - 12, e.y - 10, false);
                    ctx.c.globalAlpha = 1;
                }
                const fl = e.hitF > 0; let sp, cols;
                const bossVariant = (ctx.G.stage - 1) % 3;
                const bossCols = bossVariant === 1 ? ctx.SP.bossRedC : bossVariant === 2 ? ctx.SP.bossBlueC : ctx.SP.bossC;
                if (e.type === 'bee') { sp = ctx.SP.bee[e.fr]; cols = ctx.SP.bC; } else if (e.type === 'butterfly') { sp = ctx.SP.bf[e.fr]; cols = ctx.SP.bfC; } else if (e.type === 'stalker') { sp = ctx.SP.stalker[e.fr]; cols = ctx.SP.stalkerC; } else if (e.type === 'sniper') { sp = ctx.SP.sniper[e.fr]; cols = ctx.SP.sniperC; } else if (e.type === 'hunter') { sp = ctx.SP.hunter[e.fr]; cols = ctx.SP.hunterC; } else if (e.type === 'spinner') { sp = ctx.SP.spinner[e.fr]; cols = ctx.SP.spinnerC; } else if (e.type === 'bomber') { sp = ctx.SP.bomber[e.fr]; cols = ctx.SP.bomberC; } else if (e.type === 'lasher') { sp = ctx.SP.lasher[e.fr]; cols = ctx.SP.lasherC; } else if (e.type === 'weaver') { sp = ctx.SP.weaver[e.fr]; cols = ctx.SP.weaverC; } else if (e.type === 'splitter') { sp = ctx.SP.splitter[e.fr]; cols = ctx.SP.splitterC; } else if (e.type === 'shield_bee') { sp = ctx.SP.shield_bee[e.fr]; cols = ctx.SP.shield_beeC; } else if (e.type === 'kamikaze') { sp = ctx.SP.kamikaze[e.fr]; cols = ctx.SP.kamikazeC; } else if (e.type === 'carrier') { sp = ctx.SP.carrier[e.fr]; cols = ctx.SP.carrierC; } else if (e.type === 'teleporter') { sp = ctx.SP.teleporter[e.fr]; cols = ctx.SP.teleporterC; } else if (e.type === 'miniboss') { sp = e.hp <= 1 ? ctx.SP.bossCrit : e.hp <= Math.ceil(e.maxHp / 2) ? ctx.SP.bossHit : ctx.SP.boss; cols = bossCols; } else { sp = e.hp <= 1 ? ctx.SP.bossCrit : e.hp <= Math.ceil(e.maxHp / 2) ? ctx.SP.bossHit : ctx.SP.boss; cols = bossCols; }
                if (e.type === 'hunter' && e.st !== 'DEAD') {
                    ctx.c.globalAlpha = 0.25 + Math.sin(ctx.tick * 0.12) * 0.1;
                    ctx.c.shadowBlur = 10; ctx.c.shadowColor = '#ff6600';
                    ctx.drawSp(ctx.c, sp, cols, e.x - 12, e.y - 12, false);
                    ctx.c.shadowBlur = 0; ctx.c.globalAlpha = 1;
                }
                ctx.drawSp(ctx.c, sp, cols, e.x - 12, e.y - 12, fl);
                if (!fl && ctx.G.beatPhase > 0.82 && (e.type === 'bee' || e.type === 'butterfly')) {
                    // beat glow drawn in batched pass below to avoid per-enemy shadowBlur changes
                }
            }

            // batched beat-glow pass — one shadow setup per color type instead of per enemy
            if (ctx.G.beatPhase > 0.82) {
                const _ba = (ctx.G.beatPhase - 0.82) * 5.5 * 0.25;
                ctx.c.globalAlpha = _ba;
                ctx.c.shadowBlur = 5; ctx.c.shadowColor = '#8899ff';
                for (const e of ctx.G.enemies) {
                    if (e.st === 'DEAD' || e.hitF > 0 || e.type !== 'bee') continue;
                    ctx.drawSp(ctx.c, ctx.SP.bee[e.fr], ctx.SP.bC, e.x - 12, e.y - 12, false);
                }
                ctx.c.shadowColor = '#88ffaa';
                for (const e of ctx.G.enemies) {
                    if (e.st === 'DEAD' || e.hitF > 0 || e.type !== 'butterfly') continue;
                    ctx.drawSp(ctx.c, ctx.SP.bf[e.fr], ctx.SP.bfC, e.x - 12, e.y - 12, false);
                }
                ctx.c.shadowColor = '#aa66ee';
                for (const e of ctx.G.enemies) {
                    if (e.st === 'DEAD' || e.hitF > 0 || e.type !== 'stalker') continue;
                    ctx.drawSp(ctx.c, ctx.SP.stalker[e.fr], ctx.SP.stalkerC, e.x - 12, e.y - 12, false);
                }
                ctx.c.shadowColor = '#ffff44';
                for (const e of ctx.G.enemies) {
                    if (e.st === 'DEAD' || e.hitF > 0 || e.type !== 'sniper') continue;
                    ctx.drawSp(ctx.c, ctx.SP.sniper[e.fr], ctx.SP.sniperC, e.x - 12, e.y - 12, false);
                }
                ctx.c.shadowColor = '#ff6600';
                for (const e of ctx.G.enemies) {
                    if (e.st === 'DEAD' || e.hitF > 0 || e.type !== 'hunter') continue;
                    ctx.drawSp(ctx.c, ctx.SP.hunter[e.fr], ctx.SP.hunterC, e.x - 12, e.y - 12, false);
                }
                ctx.c.shadowBlur = 0; ctx.c.globalAlpha = 1;
            }

            // Enemy HP bars
            for (const _he of ctx.G.enemies) {
                if (_he.st === 'DEAD' || _he.maxHp <= 1) continue;
                const _bw = 20, _bh = 2, _bx = _he.x - 10, _by = _he.y - 18;
                ctx.c.fillStyle = '#111'; ctx.c.fillRect(_bx - 1, _by - 1, _bw + 2, _bh + 2);
                ctx.c.fillStyle = '#333'; ctx.c.fillRect(_bx, _by, _bw, _bh);
                const _hr = _he.hp / _he.maxHp;
                ctx.c.fillStyle = _hr > 0.5 ? '#44cc44' : _hr > 0.25 ? '#ffcc00' : '#ff4444';
                ctx.c.fillRect(_bx, _by, Math.ceil(_bw * _hr), _bh);
            }
            // Frozen enemy overlay
            if (ctx.G.freezeT > 0) {
                const _iceAlpha = Math.min(1, ctx.G.freezeT / 400) * 0.42;
                ctx.c.globalAlpha = _iceAlpha; ctx.c.fillStyle = '#88eeff';
                for (const _ie of ctx.G.enemies) { if (_ie.st === 'DEAD') continue; ctx.c.fillRect(Math.floor(_ie.x - 13), Math.floor(_ie.y - 13), 26, 26); }
                ctx.c.globalAlpha = 1;
                if (Math.random() < 0.08 && ctx.G.enemies.length > 0) {
                    const _fe = ctx.G.enemies[Math.floor(Math.random() * ctx.G.enemies.length)];
                    if (_fe.st !== 'DEAD') ctx.G.part.push({ x: _fe.x + (Math.random()-0.5)*18, y: _fe.y + (Math.random()-0.5)*18, vx: (Math.random()-0.5)*12, vy: -8 - Math.random()*15, life: 280, t: 0, col: '#ccf4ff', size: 1, spark: true });
                }
            }

            for (const pu of ctx.G.powerups) {
                const glow = 0.3 + Math.sin(ctx.tick * 0.1 + pu.t * 0.01) * 0.2;
                const pulse = 1 + Math.sin(ctx.tick * 0.06 + pu.t * 0.005) * 0.15;
                ctx.c.shadowBlur = 8; ctx.c.shadowColor = ctx.PU_COL[pu.type];
                ctx.c.globalAlpha = glow * 0.7; ctx.c.fillStyle = ctx.PU_COL[pu.type];
                ctx.c.beginPath(); ctx.c.arc(pu.x, pu.y, 10 * pulse, 0, Math.PI * 2); ctx.c.fill(); ctx.c.globalAlpha = 1;
                ctx.c.save(); ctx.c.translate(pu.x, pu.y); ctx.c.rotate(ctx.tick * 0.02 + pu.t * 0.001);
                ctx.c.fillStyle = ctx.PU_COL[pu.type]; ctx.c.font = 'bold 8px monospace'; ctx.c.textAlign = 'center';
                if (pu.type === 'rapid') { ctx.c.fillRect(-1, -4, 2, 8); ctx.c.fillRect(-3, -1, 6, 2); }
                else if (pu.type === 'spread') { for (let a2 = -1; a2 <= 1; a2++) ctx.c.fillRect(a2 * 3, Math.abs(a2) * 2 - 2, 2, 4); }
                else if (pu.type === 'shield') { ctx.c.strokeStyle = ctx.PU_COL.shield; ctx.c.lineWidth = 1; ctx.c.beginPath(); ctx.c.arc(0, 0, 4, 0, Math.PI * 2); ctx.c.stroke(); }
                else if (pu.type === 'speed') { ctx.c.fillRect(-3, 0, 6, 2); ctx.c.fillRect(1, -3, 2, 3); ctx.c.fillRect(1, 2, 2, 3); }
                else if (pu.type === 'magnet') { ctx.c.beginPath(); ctx.c.arc(0, 0, 3, 0, Math.PI * 2); ctx.c.stroke(); ctx.c.fillRect(-1, -4, 2, 2); }
                else if (pu.type === 'laser') { ctx.c.fillRect(-1, -5, 2, 10); }
                else if (pu.type === 'multibomb') { for (let i2 = 0; i2 < 6; i2++) { const a2 = i2 * 1.05; ctx.c.fillRect(Math.floor(Math.cos(a2) * 4), Math.floor(Math.sin(a2) * 4), 2, 2); } }
                else if (pu.type === 'timeslow') { ctx.c.beginPath(); ctx.c.arc(0, 0, 4, -Math.PI / 2, Math.PI / 2); ctx.c.stroke(); ctx.c.fillRect(0, -4, 1, 4); }
                else if (pu.type === 'pierce') { ctx.c.fillRect(-1, -5, 2, 10); ctx.c.fillRect(-3, 0, 6, 1); }
                else if (pu.type === 'homing') { ctx.c.beginPath(); ctx.c.moveTo(0, -4); ctx.c.lineTo(3, 2); ctx.c.lineTo(-3, 2); ctx.c.closePath(); ctx.c.stroke(); }
                else if (pu.type === 'supernova') { for (let i2 = 0; i2 < 8; i2++) { const a2 = i2 * 0.785; ctx.c.fillRect(Math.floor(Math.cos(a2) * 5), Math.floor(Math.sin(a2) * 5), 2, 2); } }
                else if (pu.type === 'freeze') {
                    ctx.c.strokeStyle = ctx.PU_COL.freeze; ctx.c.lineWidth = 1;
                    ctx.c.beginPath();
                    for (let i2 = 0; i2 < 6; i2++) { const a2 = i2 * Math.PI / 3; ctx.c.moveTo(0, 0); ctx.c.lineTo(Math.round(Math.cos(a2) * 5), Math.round(Math.sin(a2) * 5)); }
                    ctx.c.stroke();
                }
                else if (pu.type === 'levelskip') {
                    ctx.c.strokeStyle = ctx.PU_COL.levelskip; ctx.c.fillStyle = ctx.PU_COL.levelskip; ctx.c.lineWidth = 1;
                    ctx.c.beginPath(); ctx.c.moveTo(-3, -4); ctx.c.lineTo(1, 0); ctx.c.lineTo(-3, 4); ctx.c.closePath(); ctx.c.fill();
                    ctx.c.beginPath(); ctx.c.moveTo(1, -4); ctx.c.lineTo(5, 0); ctx.c.lineTo(1, 4); ctx.c.closePath(); ctx.c.fill();
                }
                else { for (let i2 = 0; i2 < 5; i2++) { const a2 = i2 * 1.26; ctx.c.fillRect(Math.floor(Math.cos(a2) * 4), Math.floor(Math.sin(a2) * 4), 2, 2); } }
                ctx.c.restore();
                ctx.c.shadowBlur = 0;
            }
            for (const dr of ctx.G.drones) {
                ctx.c.fillStyle = '#44ffaa'; ctx.c.shadowBlur = 4; ctx.c.shadowColor = '#44ffaa';
                ctx.c.fillRect(Math.floor(dr.x - 3), Math.floor(dr.y - 3), 6, 6);
                ctx.c.fillStyle = '#88ffcc'; ctx.c.fillRect(Math.floor(dr.x - 1), Math.floor(dr.y - 1), 2, 2);
                ctx.c.shadowBlur = 0;
            }
            if (ctx.G.blackhole) {
                const bha = Math.min(1, ctx.G.blackhole.t / 500);
                const bhr = 15 + Math.sin(ctx.tick * 0.1) * 5;
                ctx.c.globalAlpha = 0.6;
                const bhGr = ctx.c.createRadialGradient(ctx.G.blackhole.x, ctx.G.blackhole.y, 0, ctx.G.blackhole.x, ctx.G.blackhole.y, bhr + 20);
                bhGr.addColorStop(0, '#000'); bhGr.addColorStop(0.4, '#220044'); bhGr.addColorStop(0.7, '#440088'); bhGr.addColorStop(1, 'transparent');
                ctx.c.fillStyle = bhGr; ctx.c.fillRect(ctx.G.blackhole.x - bhr - 20, ctx.G.blackhole.y - bhr - 20, (bhr + 20) * 2, (bhr + 20) * 2);
                ctx.c.globalAlpha = 0.8;
                ctx.c.strokeStyle = '#8844ff'; ctx.c.lineWidth = 1.5;
                ctx.c.shadowBlur = 8; ctx.c.shadowColor = '#8844ff';
                ctx.c.beginPath(); ctx.c.arc(ctx.G.blackhole.x, ctx.G.blackhole.y, bhr, 0, Math.PI * 2); ctx.c.stroke();
                ctx.c.shadowBlur = 0; ctx.c.globalAlpha = 1;
            }

            if (ctx.G.activePU && ctx.G.p.alive) {
                ctx.c.fillStyle = ctx.PU_COL[ctx.G.activePU.type]; ctx.c.font = '9px "Courier New",monospace'; ctx.c.textAlign = 'center';
                const labels = { rapid: 'RAPID FIRE', spread: 'SPREAD SHOT', shield: 'SHIELD', speed: 'SPEED BOOST', magnet: 'MAGNET', laser: 'LASER', timeslow: 'TIME SLOW' };
                const label = labels[ctx.G.activePU.type];
                if (label) ctx.c.fillText(label, p.x, p.y + 22);
                if (ctx.G.activePU.type !== 'shield' && ctx.PU_DUR[ctx.G.activePU.type]) {
                    const bw = 40, bh = 3, bx = p.x - bw / 2, by = p.y + 24;
                    ctx.c.fillStyle = '#333'; ctx.c.fillRect(bx, by, bw, bh);
                    ctx.c.fillStyle = ctx.PU_COL[ctx.G.activePU.type]; ctx.c.fillRect(bx, by, bw * (ctx.G.puTimer / ctx.PU_DUR[ctx.G.activePU.type]), bh);
                }
            }

            if (ctx.G.beam && ctx.G.beam.active) ctx.renderBeam(ctx.G.beam);

            for (const ex of ctx.G.exp) {
                const pr = ex.t / ex.dur;
                if (ex.flash) {
                    ctx.c.globalAlpha = Math.max(0, 1 - pr);
                    ctx.c.fillStyle = '#fff';
                    const fr = ex.isBoss ? 25 : 12;
                    ctx.c.beginPath(); ctx.c.arc(ex.x, ex.y, fr * (1 - pr * 0.5), 0, Math.PI * 2); ctx.c.fill();
                    ctx.c.globalAlpha = 1;
                } else if (ex.shockwave) {
                    ctx.c.globalAlpha = Math.max(0, 1 - pr) * 0.5;
                    ctx.c.strokeStyle = '#ffcc00'; ctx.c.lineWidth = Math.max(1, 3 - pr * 3);
                    ctx.c.shadowBlur = 8; ctx.c.shadowColor = '#ff8800';
                    ctx.c.beginPath(); ctx.c.arc(ex.x, ex.y, pr * 50, 0, Math.PI * 2); ctx.c.stroke();
                    ctx.c.shadowBlur = 0; ctx.c.globalAlpha = 1;
                } else {
                    const sz = ex.isBoss ? 10 + Math.floor(pr * 14) : 4 + Math.floor(pr * 4) * 3;
                    ctx.c.globalAlpha = Math.max(0, 1 - pr);
                    ctx.c.shadowBlur = ex.isBoss ? 12 : 6; ctx.c.shadowColor = '#ff8800';
                    for (let i = 0; i < sz; i++) {
                        const a = (i / sz) * Math.PI * 2 + ex.seed, d = (ex.isBoss ? 8 : 3) * (1 + pr * 2.5);
                        const ci = Math.floor(pr * 3); ctx.c.fillStyle = ['#ffcc00', '#ff8800', '#ff4444'][ci < 3 ? ci : 2];
                        ctx.c.fillRect(Math.floor(ex.x + Math.cos(a) * d), Math.floor(ex.y + Math.sin(a) * d), ex.isBoss ? 3 : 2, ex.isBoss ? 3 : 2);
                    }
                    ctx.c.shadowBlur = 0; ctx.c.globalAlpha = 1;
                }
            }
            for (const pt of ctx.G.part) {
                const alpha = Math.max(0, 1 - pt.t / pt.life);
                if (pt.spark) {
                    ctx.c.globalAlpha = alpha; ctx.c.fillStyle = pt.col; ctx.c.fillRect(Math.floor(pt.x), Math.floor(pt.y), 1, 1);
                } else if (pt.smoke) {
                    ctx.c.globalAlpha = alpha * 0.35; ctx.c.fillStyle = pt.col;
                    ctx.c.fillRect(Math.floor(pt.x), Math.floor(pt.y), pt.size || 3, pt.size || 3);
                } else if (pt.debris) {
                    ctx.c.globalAlpha = alpha;
                    ctx.c.save(); ctx.c.translate(pt.x, pt.y); ctx.c.rotate(pt.rot);
                    ctx.c.fillStyle = pt.col; ctx.c.fillRect(-pt.size / 2, -pt.size / 2, pt.size, pt.size);
                    ctx.c.restore();
                } else {
                    ctx.c.globalAlpha = alpha; ctx.c.fillStyle = pt.col;
                    if (pt.size >= 3) { ctx.c.shadowBlur = 6; ctx.c.shadowColor = pt.col; ctx.c.fillRect(Math.floor(pt.x), Math.floor(pt.y), pt.size || 2, pt.size || 2); ctx.c.shadowBlur = 0; }
                    else ctx.c.fillRect(Math.floor(pt.x), Math.floor(pt.y), pt.size || 2, pt.size || 2);
                }
            } ctx.c.globalAlpha = 1;
            for (const sp of ctx.G.scorePopups) {
                const _spAlpha = Math.max(0, 1 - sp.t / sp.dur);
                const _spScale = sp.big ? (1 + Math.max(0, 1 - sp.t / 200) * 0.7) : 1;
                ctx.c.globalAlpha = _spAlpha;
                ctx.c.save(); ctx.c.translate(Math.floor(sp.x), Math.floor(sp.y)); ctx.c.scale(_spScale, _spScale);
                if (sp.big) { ctx.c.shadowBlur = 8; ctx.c.shadowColor = sp.col; }
                ctx.c.fillStyle = sp.col;
                ctx.c.font = (sp.big ? 'bold 13px' : 'bold 10px') + ' "Courier New",monospace';
                ctx.c.textAlign = 'center'; ctx.c.fillText(sp.text, 0, 0);
                if (sp.big) ctx.c.shadowBlur = 0;
                ctx.c.restore();
            } ctx.c.globalAlpha = 1;

            if (ctx.G.perfectT > 0) {
                ctx.c.shadowBlur = 8; ctx.c.shadowColor = '#00ffcc';
                ctx.c.fillStyle = '#00ffcc'; ctx.c.font = 'bold 22px "Courier New",monospace'; ctx.c.textAlign = 'center';
                ctx.c.fillText(ctx.t('galaxa.perfect_bonus', 'PERFECT BONUS') + ' +5000', ctx.W / 2, ctx.H / 2 - 40);
                ctx.c.shadowBlur = 0;
            }

            if (ctx.G.comboBanner) {
                const alpha = Math.max(0, 1 - ctx.G.comboBanner.t / ctx.G.comboBanner.dur);
                const sc = 1 + (ctx.G.comboBanner.t < 200 ? (200 - ctx.G.comboBanner.t) / 200 * 0.5 : 0);
                ctx.c.save(); ctx.c.globalAlpha = alpha;
                ctx.c.translate(ctx.W / 2, ctx.H / 2 + 30); ctx.c.scale(sc, sc);
                ctx.c.shadowBlur = 12; ctx.c.shadowColor = '#ffcc00';
                ctx.c.fillStyle = '#ffcc00'; ctx.c.font = 'bold 20px "Courier New",monospace'; ctx.c.textAlign = 'center';
                ctx.c.fillText(ctx.G.comboBanner.text, 0, 0);
                ctx.c.fillStyle = '#fff'; ctx.c.font = 'bold 14px "Courier New",monospace';
                ctx.c.fillText('x' + ctx.G.comboBanner.mult, 0, 20);
                ctx.c.shadowBlur = 0; ctx.c.restore();
            }
            for (let _api = 0; _api < ctx.G.achievementPopups.length; _api++) {
                const ap = ctx.G.achievementPopups[_api];
                const apAlpha = ap.t < 300 ? ap.t / 300 : ap.t > ap.dur - 500 ? (ap.dur - ap.t) / 500 : 1;
                const apY = ctx.H - 60 - _api * 30;
                ctx.c.globalAlpha = apAlpha;
                ctx.c.fillStyle = 'rgba(0,0,0,0.7)'; ctx.c.fillRect(ctx.W / 2 - 100, apY - 10, 200, 22);
                ctx.c.strokeStyle = '#ffcc00'; ctx.c.lineWidth = 1; ctx.c.strokeRect(ctx.W / 2 - 100, apY - 10, 200, 22);
                ctx.c.shadowBlur = 6; ctx.c.shadowColor = '#ffcc00';
                ctx.c.fillStyle = '#ffcc00'; ctx.c.font = 'bold 10px "Courier New",monospace'; ctx.c.textAlign = 'center';
                ctx.c.fillText('ACHIEVEMENT: ' + ap.text, ctx.W / 2, apY + 4);
                ctx.c.shadowBlur = 0;
            }
            ctx.c.globalAlpha = 1;
            let _apLen = 0;
            for (let _api = 0; _api < ctx.G.achievementPopups.length; _api++) { ctx.G.achievementPopups[_api].t += 16; if (ctx.G.achievementPopups[_api].t < ctx.G.achievementPopups[_api].dur) ctx.G.achievementPopups[_apLen++] = ctx.G.achievementPopups[_api]; }
            ctx.G.achievementPopups.length = _apLen;

            if (ctx.G.upgradeBanner) {
                const alpha = Math.max(0, 1 - ctx.G.upgradeBanner.t / ctx.G.upgradeBanner.dur);
                const sc = 1 + (ctx.G.upgradeBanner.t < 300 ? (300 - ctx.G.upgradeBanner.t) / 300 * 0.8 : 0);
                ctx.c.save(); ctx.c.globalAlpha = alpha;
                ctx.c.translate(ctx.W / 2, ctx.H / 2 + 60); ctx.c.scale(sc, sc);
                ctx.c.shadowBlur = 15; ctx.c.shadowColor = ctx.PU_UPGRADE_COL[ctx.G.upgradeBanner.type] || '#fff';
                ctx.c.fillStyle = ctx.PU_UPGRADE_COL[ctx.G.upgradeBanner.type] || '#fff'; ctx.c.font = 'bold 18px "Courier New",monospace'; ctx.c.textAlign = 'center';
                ctx.c.fillText(ctx.G.upgradeBanner.text, 0, 0);
                ctx.c.shadowBlur = 0; ctx.c.restore();
            }

            if (ctx.G.slowMoT > 0) {
                ctx.c.fillStyle = 'rgba(255,255,255,0.03)'; ctx.c.fillRect(0, 0, ctx.W, ctx.H);
            }

            let boss = null; for (let _bi = 0; _bi < ctx.G.enemies.length; _bi++) { const _be = ctx.G.enemies[_bi]; if ((_be.type === 'boss' || _be.type === 'miniboss') && _be.st !== 'DEAD') { boss = _be; break; } }
            if (boss) {
                const barW = 220, barH = 8, barX = ctx.W / 2 - barW / 2, barY = 40;
                ctx.c.fillStyle = '#222'; ctx.c.fillRect(barX - 1, barY - 1, barW + 2, barH + 2);
                ctx.c.fillStyle = '#333'; ctx.c.fillRect(barX, barY, barW, barH);
                const hpRatio = boss.hp / boss.maxHp;
                const grad = ctx.c.createLinearGradient(barX, barY, barX + barW * hpRatio, barY);
                grad.addColorStop(0, hpRatio > 0.5 ? '#ff4444' : '#ff2222'); grad.addColorStop(1, hpRatio > 0.5 ? '#ff8844' : '#ff4444');
                ctx.c.shadowBlur = 8; ctx.c.shadowColor = hpRatio > 0.3 ? '#ff4444' : '#ff0000';
                ctx.c.fillStyle = grad; ctx.c.fillRect(barX, barY, barW * hpRatio, barH);
                if (hpRatio <= 0.3 && Math.sin(ctx.tick * 0.15) > 0) {
                    ctx.c.strokeStyle = '#ff0000'; ctx.c.lineWidth = 1; ctx.c.strokeRect(barX - 2, barY - 2, barW + 4, barH + 4);
                }
                ctx.c.shadowBlur = 0;
                ctx.c.fillStyle = '#fff'; ctx.c.font = 'bold 11px "Courier New",monospace'; ctx.c.textAlign = 'center';
                ctx.c.fillText(boss.type === 'miniboss' ? 'MINI-BOSS' : 'BOSS', ctx.W / 2, barY - 4);
            }

            if (ctx.G.timeScale < 1) {
                ctx.c.fillStyle = 'rgba(170,68,255,0.08)'; ctx.c.fillRect(0, 0, ctx.W, ctx.H);
                ctx.wrapEl.classList.add('galaxa-timeslow');
            } else {
                ctx.wrapEl.classList.remove('galaxa-timeslow');
            }

            ctx.renderHUD();
            if (ctx.G.st === 'GAME_OVER') {
                ctx.c.fillStyle = 'rgba(0,0,0,0.5)'; ctx.c.fillRect(0, ctx.H / 2 - 40, ctx.W, 80);
                ctx.c.fillStyle = '#ff4444'; ctx.c.font = 'bold 24px "Courier New",monospace'; ctx.c.textAlign = 'center'; ctx.c.fillText(ctx.t('galaxa.game_over', 'GAME OVER'), ctx.W / 2, ctx.H / 2 - 10);
                if (ctx.G.contTmr > 0) {
                    ctx.c.fillStyle = '#ffcc00'; ctx.c.font = '16px "Courier New",monospace';
                    ctx.c.fillText(ctx.t('galaxa.continue_prompt', 'CONTINUE?') + ' ' + ctx.G.contCnt, ctx.W / 2, ctx.H / 2 + 20);
                    const _cAngle = (ctx.G.contTmr / 10) * Math.PI * 2 - Math.PI / 2;
                    ctx.c.strokeStyle = '#ffcc00'; ctx.c.lineWidth = 3; ctx.c.globalAlpha = 0.7;
                    ctx.c.shadowBlur = 6; ctx.c.shadowColor = '#ffcc00';
                    ctx.c.beginPath(); ctx.c.arc(ctx.W / 2, ctx.H / 2 + 46, 16, -Math.PI / 2, _cAngle); ctx.c.stroke();
                    ctx.c.shadowBlur = 0; ctx.c.lineWidth = 1; ctx.c.globalAlpha = 1;
                }
            }
        }

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
            ctx.c.fillStyle = '#4488ff'; ctx.c.font = '12px "Courier New",monospace'; ctx.c.textAlign = 'left'; ctx.c.fillText(ctx.t('galaxa.score', 'SCORE'), 10, 16);
            ctx.c.fillStyle = '#fff'; ctx.c.fillText(String(ctx.G.displayScore).padStart(8, '0'), 10, 32);
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
            ctx.c.fillStyle = '#4488ff'; ctx.c.textAlign = 'right'; ctx.c.fillText(ctx.t('galaxa.high_score', 'HIGH SCORE'), ctx.W - 10, 16);
            ctx.c.fillStyle = '#ffcc00'; ctx.c.fillText(String(ctx.G.hi).padStart(8, '0'), ctx.W - 10, 32);
            const stagePulse = ctx.G.warpT > 0 ? 1 + Math.sin(ctx.tick * 0.15) * 0.3 : 1;
            ctx.c.save(); ctx.c.translate(ctx.W / 2, 16); ctx.c.scale(stagePulse, stagePulse);
            ctx.c.fillStyle = '#4488ff'; ctx.c.font = 'bold 12px "Courier New",monospace'; ctx.c.textAlign = 'center';
            ctx.c.fillText(ctx.t('galaxa.stage', 'STAGE') + ' ' + ctx.G.stage, 0, 0);
            ctx.c.restore();
            if (ctx.G.chal) {
                let _cr = 0; for (let _ci = 0; _ci < ctx.G.enemies.length; _ci++) if (ctx.G.enemies[_ci].st !== 'DEAD') _cr++;
                ctx.c.fillStyle = '#ff8800'; ctx.c.font = 'bold 10px "Courier New",monospace'; ctx.c.textAlign = 'center';
                ctx.c.fillText(ctx.t('galaxa.challenge_stage', 'CHALLENGE') + ' ' + _cr + '/' + ctx.G.chalTot, ctx.W / 2, 28);
            }
            let alive2cnt = 0; for (let _hi = 0; _hi < ctx.G.enemies.length; _hi++) { const _hh = ctx.G.enemies[_hi]; if (_hh.st !== 'DEAD' && _hh.type !== 'boss' && _hh.type !== 'miniboss') alive2cnt++; }
            if (alive2cnt > 0 && alive2cnt <= 5) {
                ctx.c.fillStyle = '#888'; ctx.c.font = '10px "Courier New",monospace'; ctx.c.textAlign = 'center';
                ctx.c.fillText(alive2cnt + ' LEFT', ctx.W / 2, ctx.G.chal ? 38 : 28);
            }
            if (ctx.G.weaponLv > 1) {
                ctx.c.fillStyle = '#44cc88'; ctx.c.font = '9px "Courier New",monospace'; ctx.c.textAlign = 'left';
                ctx.c.fillText('W' + ctx.G.weaponLv, 10, 54);
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
                    const cx = ctx.G.p.x, cy = ctx.G.p.y, r = 24;
                    const startA = -Math.PI / 2, endA = startA + ratio * Math.PI * 2;
                    ctx.c.strokeStyle = puCol; ctx.c.lineWidth = 2; ctx.c.globalAlpha = 0.5;
                    ctx.c.shadowBlur = 4; ctx.c.shadowColor = puCol;
                    ctx.c.beginPath(); ctx.c.arc(cx, cy, r, startA, endA); ctx.c.stroke();
                    ctx.c.shadowBlur = 0; ctx.c.globalAlpha = 1;
                }
            }
            for (let i = 0; i < Math.min(ctx.G.lives, 5); i++) ctx.drawSp(ctx.c, ctx.SP.player, ctx.SP.pC, 10 + i * 26, ctx.H - 24, false);
            if (ctx.G.activePU) {
                const puIconX = ctx.W - 20, puIconY = ctx.H - 20;
                const expiring = ctx.G.activePU.type !== 'shield' && ctx.PU_DUR[ctx.G.activePU.type] && ctx.G.puTimer < 2000;
                if (!expiring || Math.sin(ctx.tick * 0.2) > 0) {
                    ctx.c.fillStyle = ctx.PU_COL[ctx.G.activePU.type] || '#fff'; ctx.c.font = 'bold 9px monospace'; ctx.c.textAlign = 'right';
                    ctx.c.fillText(ctx.G.activePU.type.toUpperCase().substring(0, 4), puIconX, puIconY);
                }
            }
        }

        function renderPause() {
            if (ctx.G.st !== 'PAUSED') return;
            ctx.c.fillStyle = 'rgba(0,0,0,0.75)'; ctx.c.fillRect(0, 0, ctx.W, ctx.H);
            ctx.c.textAlign = 'center'; ctx.c.fillStyle = '#ffcc00'; ctx.c.font = 'bold 26px "Courier New",monospace';
            ctx.c.shadowBlur = 10; ctx.c.shadowColor = '#ffcc00';
            ctx.c.fillText(ctx.t('galaxa.paused', 'PAUSED'), ctx.W / 2, ctx.H / 2 - 60);
            ctx.c.shadowBlur = 0;
            ctx.c.fillStyle = '#aaccee'; ctx.c.font = '12px "Courier New",monospace';
            ctx.c.fillText(ctx.t('galaxa.score', 'SCORE') + ': ' + ctx.G.score + '  ' + ctx.t('galaxa.stage', 'STAGE') + ': ' + ctx.G.stage, ctx.W / 2, ctx.H / 2 - 35);
            const items = [ctx.t('galaxa.resume', 'RESUME'), ctx.t('galaxa.restart', 'RESTART'), ctx.t('galaxa.quit', 'QUIT')];
            items.forEach((it, i) => {
                ctx.c.fillStyle = i === ctx.G.pauseSel ? '#ffcc00' : '#888'; ctx.c.font = i === ctx.G.pauseSel ? 'bold 16px "Courier New",monospace' : '14px "Courier New",monospace';
                if (i === ctx.G.pauseSel) { ctx.c.shadowBlur = 6; ctx.c.shadowColor = '#ffcc00'; }
                ctx.c.fillText(it, ctx.W / 2, ctx.H / 2 + i * 30);
                ctx.c.shadowBlur = 0;
            });
        }

        function renderSettings() {
            ctx.c.fillStyle = 'rgba(0,0,0,0.88)'; ctx.c.fillRect(0, 0, ctx.W, ctx.H);
            ctx.c.textAlign = 'center'; ctx.c.fillStyle = '#ffcc00'; ctx.c.font = 'bold 22px "Courier New",monospace';
            ctx.c.shadowBlur = 10; ctx.c.shadowColor = '#ffcc00';
            ctx.c.fillText(ctx.t('galaxa.settings', 'SETTINGS'), ctx.W / 2, 80);
            ctx.c.shadowBlur = 0;
            const shipName = ctx.t('galaxa.' + ctx.settings.ship, (ctx.SHIP_TYPES[ctx.settings.ship] || ctx.SHIP_TYPES.classic).name);
            const shakeLabel = ctx.settings.shake === 0 ? 'OFF' : ctx.settings.shake === 0.25 ? 'LOW' : ctx.settings.shake === 0.5 ? 'MED' : ctx.settings.shake === 0.75 ? 'HIGH' : 'MAX';
            const items = [
                { label: ctx.t('galaxa.sound', 'SOUND'), val: ctx.G.muted ? 'OFF' : 'ON' },
                { label: ctx.t('galaxa.difficulty', 'DIFFICULTY'), val: ctx.t('galaxa.' + ctx.settings.diff, ctx.settings.diff.toUpperCase()) },
                { label: ctx.t('galaxa.volume', 'VOLUME'), val: ctx.settings.vol + '%' },
                { label: ctx.t('galaxa.ship_select', 'SHIP'), val: shipName },
                { label: ctx.t('galaxa.crt_effect', 'CRT EFFECT'), val: ctx.settings.crt ? 'ON' : 'OFF' },
                { label: ctx.t('galaxa.particle_density', 'PARTICLES'), val: ctx.t('galaxa.' + ctx.settings.particles, ctx.settings.particles.toUpperCase()) },
                { label: ctx.t('galaxa.shake_intensity', 'SHAKE'), val: shakeLabel },
                { label: ctx.t('galaxa.quit', 'QUIT'), val: '' }
            ];
            items.forEach((it, i) => {
                const sel = i === ctx.G.settingsSel;
                ctx.c.fillStyle = sel ? '#ffcc00' : '#888'; ctx.c.font = sel ? 'bold 14px "Courier New",monospace' : '12px "Courier New",monospace';
                if (sel) { ctx.c.shadowBlur = 6; ctx.c.shadowColor = '#ffcc00'; }
                ctx.c.fillText(it.label + (it.val ? ': ' + it.val : ''), ctx.W / 2, 130 + i * 36);
                ctx.c.shadowBlur = 0;
                if (i === 2) {
                    const bw = 200, bh = 8, bx = ctx.W / 2 - bw / 2, by = 138 + i * 36;
                    ctx.c.fillStyle = '#222'; ctx.c.fillRect(bx, by, bw, bh);
                    ctx.c.fillStyle = '#4488ff'; ctx.c.fillRect(bx, by, bw * ctx.settings.vol / 100, bh);
                    if (sel) { ctx.c.strokeStyle = '#4488ff'; ctx.c.lineWidth = 1; ctx.c.strokeRect(bx - 1, by - 1, bw + 2, bh + 2); }
                }
            });
            ctx.c.fillStyle = '#666'; ctx.c.font = '10px "Courier New",monospace';
            ctx.c.fillText('\u2191\u2193 select  \u2190\u2192 change  ENTER confirm', ctx.W / 2, 430);
            ctx.c.fillText('ARROWS+SPACE  GAMEPAD D-PAD+A', ctx.W / 2, 450);
        }

        ctx.renderFlame = renderFlame;
        ctx.renderFrame = renderFrame;
        ctx.renderTitle = renderTitle;
        ctx.renderStageIntro = renderStageIntro;
        ctx.renderGame = renderGame;
        ctx.renderBeam = renderBeam;
        ctx.renderHUD = renderHUD;
        ctx.renderPause = renderPause;
        ctx.renderSettings = renderSettings;
    };
})();
