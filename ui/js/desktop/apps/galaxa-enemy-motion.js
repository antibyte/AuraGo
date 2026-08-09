(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createEnemyMotion = function (ctx) {
        function trailCap() {
            const p = ctx.settings.particles || 'high';
            if (p === 'low') return 3;
            if (p === 'medium') return 5;
            return 7;
        }

        function fxDef(e) {
            return GC.ENEMY_MOTION_FX[e.type] || GC.ENEMY_MOTION_FX.default;
        }

        function blinkAlpha(ph, offMs) {
            const t = ph / offMs;
            if (t < 0.1) return 1 - t / 0.1 * 0.96;
            if (t > 0.9) return (1 - t) / 0.1 * 0.08;
            return 0.04;
        }

        function trailTint(e, cols) {
            if (e.type === 'hunter') return '#ff7722';
            if (e.type === 'kamikaze') return '#ff4444';
            if (e.type === 'stalker') return '#aa66ee';
            if (cols && (cols[2] || cols['2'])) return cols[2] || cols['2'];
            return '#88ccff';
        }

        function updateEnemyMotionFx(e, dtMs) {
            if (e.st === 'DEAD') return;
            const fx = fxDef(e);
            if (e.motionPhase == null) e.motionPhase = Math.random() * Math.PI * 2;

            let scale = 1;
            let alpha = 1;

            if (fx.pulse) {
                scale *= 1 + Math.sin(ctx.G.fTmr * fx.pulse.speed + e.motionPhase) * fx.pulse.amp;
            }

            if (fx.blink) {
                if (fx.blinkAim && e.sTmr != null && e.sTmr < 320) {
                    alpha = 0.18 + Math.sin(ctx.tick * 0.45) * 0.12;
                } else {
                    e.blinkT = (e.blinkT || 0) + dtMs;
                    const cycle = fx.blink.on + fx.blink.off;
                    const ph = e.blinkT % cycle;
                    if (ph >= fx.blink.on) alpha = blinkAlpha(ph - fx.blink.on, fx.blink.off);
                }
            }

            if (fx.diveScale && e.st === 'DIVING') {
                const prog = Math.min(1, Math.max(0, (e.y - ctx.FTOP) / Math.max(1, ctx.H * 0.45)));
                scale *= fx.diveScale.min + (fx.diveScale.max - fx.diveScale.min) * prog;
            }

            if (fx.teleportBlink && e.type === 'teleporter' && e.teleportTimer != null && e.teleportTimer < 220) {
                alpha = Math.min(alpha, Math.max(0.02, e.teleportTimer / 220 * 0.12));
            }

            if (e.st === 'ENTER') {
                const enterProg = 1 - Math.max(0, e.eTmr) / Math.max(1, e.eTmr + 400);
                scale *= 0.65 + enterProg * 0.35;
            }

            e.mScale = scale;
            e.mAlpha = alpha;
            e.mSkipDraw = alpha < 0.06 && e.st !== 'DIVING';

            if (fx.diveTrail && e.st === 'DIVING') {
                e.trailAcc = (e.trailAcc || 0) + dtMs;
                const interval = e.type === 'kamikaze' ? 16 : e.type === 'hunter' ? 22 : 30;
                if (e.trailAcc >= interval) {
                    e.trailAcc -= interval;
                    if (!e.trail) e.trail = [];
                    e.trail.unshift({ x: e.x, y: e.y, rot: e.rot || 0 });
                    while (e.trail.length > trailCap()) e.trail.pop();
                }
            } else if (e.trail && e.trail.length) {
                e.trailFade = (e.trailFade || 0) + dtMs;
                if (e.trailFade > 45) {
                    e.trail.pop();
                    e.trailFade = 0;
                }
            } else {
                e.trail = null;
                e.trailAcc = 0;
                e.trailFade = 0;
            }
        }

        function drawEnemyMotionTrail(c, e, sp, cols, off) {
            if (!e.trail || !e.trail.length) return;
            const baseA = e.mAlpha != null ? e.mAlpha : 1;
            const tint = trailTint(e, cols);
            const ghostCols = { 1: tint, 2: tint, 3: tint, 4: tint, 5: tint, 6: tint, 7: tint, a: tint };

            c.save();
            c.globalCompositeOperation = 'lighter';
            c.strokeStyle = tint;
            c.lineWidth = 2;
            c.beginPath();
            c.moveTo(e.x, e.y);
            for (let i = 0; i < e.trail.length; i++) {
                const t = e.trail[i];
                c.globalAlpha = (1 - i / e.trail.length) * 0.85 * baseA;
                c.lineTo(t.x, t.y);
            }
            c.stroke();
            c.restore();

            for (let i = 0; i < e.trail.length; i++) {
                const t = e.trail[i];
                const fade = 1 - i / e.trail.length;
                c.globalAlpha = fade * 0.72 * baseA;
                c.save();
                c.translate(t.x, t.y);
                c.rotate(t.rot);
                const ts = 0.82 + fade * 0.18;
                c.scale(ts, ts);
                c.translate(-t.x, -t.y);
                ctx.drawSp(c, sp, ghostCols, t.x - off, t.y - off, false, true);
                c.restore();
            }
            c.globalAlpha = 1;
        }

        ctx.updateEnemyMotionFx = updateEnemyMotionFx;
        ctx.drawEnemyMotionTrail = drawEnemyMotionTrail;
    };
})();
