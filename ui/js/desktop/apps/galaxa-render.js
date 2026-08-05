(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createRenderer = function (ctx) {
        GC.createRenderEffects(ctx);
        GC.createRenderStage(ctx);
        GC.createRenderHUD(ctx);
        GC.createRenderWorld(ctx);
                function renderFrame(dt) {
            // NEW: Cinematic camera transform during super (zoom-in around canvas center)
            const G = ctx.G;
            // OPTIMIZATION: cache hot property lookups once per frame.
            // ctx.G / ctx.tick / ctx.W / ctx.H are accessed many times in this
            // function and its callees — V8 caches hidden classes well but each
            // repeated property access still costs a hash lookup. Local aliases
            // give measurable wins in heavy frames (boss fights, bullet hell).
            const c = ctx.c;
            const W = ctx.W, H = ctx.H;
            const scale = ctx.scale;
            const tick = ctx.tick;
            const zoom = G.camZoom || 1;
            const camX = G.camX || 0;
            const camY = G.camY || 0;
            if (zoom !== 1 || camX !== 0 || camY !== 0) {
                c.save();
                c.translate(W / 2, H / 2);
                c.scale(zoom, zoom);
                c.translate(-W / 2 + camX, -H / 2 + camY);
            }
            c.save(); c.setTransform(scale, 0, 0, scale, 0, 0);
            let sx = 0, sy = 0; if (G.shkT > 0 && ctx.settings.shake > 0) {
                const _decay = Math.pow(Math.min(1, G.shkT / 200), 1.5);
                const _sm = ctx.settings.shake || 1;
                if (G.shkX !== undefined && G.shkX !== 0) {
                    const _sdx = (W / 2 - G.shkX) / (W / 2);
                    const _sdy = (H / 2 - G.shkY) / (H / 2);
                    sx = (_sdx * 0.5 + (Math.random() - 0.5) * 0.5) * G.shkM * _decay * _sm;
                    sy = (_sdy * 0.5 + (Math.random() - 0.5) * 0.5) * G.shkM * _decay * _sm;
                } else {
                    sx = (Math.random() - 0.5) * G.shkM * _decay * _sm;
                    sy = (Math.random() - 0.5) * G.shkM * _decay * _sm;
                }
            }
            c.translate(sx, sy); c.fillStyle = '#000'; c.fillRect(-5, -5, W + 10, H + 10);
            ctx.drawNebula(c); ctx.drawStars(c);
            // NEW: Warp speed-line streaks behind the game layer (galaxa-fx)
            if (ctx.fxDrawBack) ctx.fxDrawBack(c);
            // NEW: Boss entrance shockwave (galaxa-fx)
            if (ctx.fxDrawBossEnter) ctx.fxDrawBossEnter(c);
            // NEW: Foreground parallax layer (debris, dust, ice shards)
            const _fgOff = (tick * 1.5) % H;
            c.globalAlpha = 0.4;
            const _fgCol = G.biome === 'crystal' ? '#88ccff' : G.biome === 'void' ? '#ffffff' : '#666';
            for (let _fi = 0; _fi < 8; _fi++) {
                const _fx = (_fi * 73 + _fgOff * 0.3) % W;
                const _fy = ((_fi * 91) + _fgOff) % H;
                c.fillStyle = _fgCol;
                c.fillRect(_fx, _fy, 2, 2);
            }
            c.globalAlpha = 1;
            if (G.chromAb > 0) {
                const ca = Math.min(1, G.chromAb / 200);
                const offset = Math.round(ca * 3);
                c.globalCompositeOperation = 'lighter';
                c.globalAlpha = ca * 0.08;
                c.fillStyle = '#ff0000'; c.fillRect(offset, 0, W, H);
                c.fillStyle = '#0000ff'; c.fillRect(-offset, 0, W, H);
                c.globalAlpha = ca * 0.04;
                c.fillStyle = '#00ff00'; c.fillRect(0, offset, W, H);
                c.globalCompositeOperation = 'source-over';
                c.globalAlpha = 1;
            }
            // NEW: Hitstop chromab spike — brief red/blue split during hitstop freeze
            if (G.hitstopT > 0) {
                const _hsa = Math.min(1, G.hitstopT / 120) * 0.12;
                c.globalCompositeOperation = 'lighter';
                c.globalAlpha = _hsa;
                c.fillStyle = '#ff0000'; c.fillRect(2, 0, W, H);
                c.fillStyle = '#00ffff'; c.fillRect(-2, 0, W, H);
                c.globalCompositeOperation = 'source-over';
                c.globalAlpha = 1;
            }
            if (G.damageVignetteT > 0) {
                const _dv = Math.min(1, G.damageVignetteT / 400) * 0.65;
                c.save();
                const _dvg = c.createRadialGradient(W * 0.5, H * 0.5, H * 0.2, W * 0.5, H * 0.5, H * 0.85);
                _dvg.addColorStop(0, 'rgba(180,0,0,0)');
                _dvg.addColorStop(1, 'rgba(220,0,0,' + _dv + ')');
                c.fillStyle = _dvg; c.fillRect(0, 0, W, H);
                c.restore();
            }
            if (G.st === 'PLAYING' && G.p.alive && G.lives <= 2) {
                const _hvInt = G.lives <= 0 ? 0.35 : G.lives === 1 ? 0.22 : 0.1;
                const _hvPulse = 1 + Math.sin(tick * 0.06) * 0.3;
                c.save();
                const _hvg = c.createRadialGradient(W * 0.5, H * 0.5, H * 0.3, W * 0.5, H * 0.5, H * 0.9);
                _hvg.addColorStop(0, 'rgba(0,0,0,0)');
                _hvg.addColorStop(1, 'rgba(0,0,0,' + (_hvInt * _hvPulse) + ')');
                c.fillStyle = _hvg; c.fillRect(0, 0, W, H);
                c.restore();
            }
            if (G.slowMoT > 0 && G.slowMoT > 1000) {
                const _rbInt = Math.min(1, (G.slowMoT - 1000) / 500) * 0.15;
                c.save();
                const _rbg = c.createRadialGradient(W * 0.5, H * 0.5, 0, W * 0.5, H * 0.5, W * 0.6);
                _rbg.addColorStop(0, 'rgba(255,255,255,0)');
                _rbg.addColorStop(0.5, 'rgba(200,220,255,' + (_rbInt * 0.3) + ')');
                _rbg.addColorStop(1, 'rgba(255,255,255,' + _rbInt + ')');
                c.fillStyle = _rbg; c.fillRect(0, 0, W, H);
                c.restore();
            }
            if (G.activePU && G.activePU.type !== 'shield' && G.p && G.p.alive) {
                const egCol = ctx.PU_COL[G.activePU.type] || '#ffffff';
                const egGrad = ctx.cachedRadialGradient(c, 'powerup-edge:' + egCol, W / 2, H / 2, W * 0.25, W * 0.75, [
                    [0, 'rgba(0,0,0,0)'],
                    [1, egCol + '55']
                ]);
                c.globalAlpha = 0.5 + Math.sin(tick * 0.05) * 0.2;
                c.fillStyle = egGrad; c.fillRect(0, 0, W, H);
                c.globalAlpha = 1;
            }
            if (G.warpFlash > 0) { c.fillStyle = 'rgba(255,255,255,' + (G.warpFlash / 50) + ')'; c.fillRect(0, 0, W, H); }
            if (false && G.swipeT > 0) {
                const progress = 1 - (G.swipeT / 1200);
                const wipeX = G.swipeDir > 0 ? W * progress : W * (1 - progress);
                c.fillStyle = '#000';
                if (G.swipeDir > 0) c.fillRect(0, 0, wipeX, H);
                else c.fillRect(wipeX, 0, W - wipeX, H);
                c.strokeStyle = '#4488ff'; c.lineWidth = 2;
                c.beginPath(); c.moveTo(wipeX, 0); c.lineTo(wipeX, H); c.stroke();
            }
            if (G.portalT > 0) {
                const progress = 1 - (G.portalT / 1400);
                G.portalR = progress * Math.max(W, H) * 0.8;
                c.save();
                c.beginPath(); c.arc(W / 2, H / 2, G.portalR, 0, Math.PI * 2); c.clip();
                c.fillStyle = '#000'; c.fillRect(0, 0, W, H);
                c.restore();
                c.strokeStyle = '#ffcc00'; c.lineWidth = 3; c.shadowBlur = 15; c.shadowColor = '#ffcc00';
                c.beginPath(); c.arc(W / 2, H / 2, G.portalR, 0, Math.PI * 2); c.stroke();
                c.shadowBlur = 0;
            }
            if (G.glitchT > 0) {
                const progress = 1 - (G.glitchT / 1000);
                c.save();
                const stripH = H / 12;
                for (const strip of G.glitchStrips) {
                    strip.offset += (strip.targetOffset - strip.offset) * 0.1;
                    const alpha = Math.abs(Math.sin(progress * Math.PI * 3 + strip.y * 0.1)) * 0.8;
                    c.globalAlpha = alpha;
                    c.fillStyle = '#000';
                    c.fillRect(0, strip.y, W, stripH);
                    if (Math.random() < 0.3) {
                        c.fillStyle = ['#ff0000', '#00ff00', '#0000ff', '#ffffff'][Math.floor(Math.random() * 4)];
                        c.fillRect(strip.offset, strip.y, 2, stripH);
                    }
                }
                c.restore();
            }
            if (G.flashT > 0) { c.fillStyle = 'rgba(255,255,255,' + (G.flashT > 30 ? 0.5 : G.flashT / 60) + ')'; c.fillRect(0, 0, W, H); }

            if (G.levelSkipTimer > 0) {
                const _lsA = Math.min(1, G.levelSkipTimer / 500) * 0.15;
                c.fillStyle = 'rgba(255,136,255,' + _lsA + ')'; c.fillRect(0, 0, W, H);
            }
            if (G.st === 'TITLE' && !G.demoMode) ctx.renderTitle();
            else if (G.st === 'STAGE_INTRO') { ctx.renderGame(); ctx.renderStageIntro(); }
            else if (G.st === 'SETTINGS') ctx.renderSettings();
            else if (G.st === 'PAUSED') { ctx.renderGame(); ctx.renderPause(); }
            else if (G.st === 'SHOP') ctx.renderShop();
            else ctx.renderGame();
            // NEW: Combo screen-edge pulse over the game layer (galaxa-fx)
            if (ctx.fxDrawOverlay) ctx.fxDrawOverlay(c);
            // NEW: Confetti over the game layer (galaxa-fx)
            if (ctx.fxDrawConfetti) ctx.fxDrawConfetti(c);
            // NEW: Magnet pull-lines (galaxa-fx)
            if (ctx.fxDrawPullLines) ctx.fxDrawPullLines(c);
            // NEW: Boss death rumble overlay (galaxa-fx)
            if (ctx.fxDrawRumbleOverlay) ctx.fxDrawRumbleOverlay(c);
            if (G.demoMode) {
                c.save();
                c.fillStyle = 'rgba(0,0,0,0.55)';
                c.fillRect(W / 2 - 90, 8, 180, 36);
                c.strokeStyle = '#4488ff';
                c.lineWidth = 1;
                c.strokeRect(W / 2 - 90, 8, 180, 36);
                c.textAlign = 'center';
                c.fillStyle = '#ffcc00';
                c.font = 'bold 13px "Courier New",monospace';
                c.fillText('DEMO MODE', W / 2, 22);
                const _blink = Math.sin(tick * 0.08) > 0;
                if (_blink) {
                    c.fillStyle = '#aaccee';
                    c.font = '9px "Courier New",monospace';
                    c.fillText('PRESS ANY KEY TO PLAY', W / 2, 37);
                }
                c.restore();
            }
            if (ctx.drawBiomeTransition) ctx.drawBiomeTransition(c, G);
            c.restore();
            if (zoom !== 1 || camX !== 0 || camY !== 0) {
                c.restore();
            }
        }

        ctx.renderFrame = renderFrame;
    };
})();
