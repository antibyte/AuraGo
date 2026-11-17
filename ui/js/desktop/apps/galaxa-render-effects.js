(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createRenderEffects = function (ctx) {
        function drawParticle(c, p) {
            const lifeRatio = p.life > 0 ? Math.max(0, Math.min(1, p.t / p.life)) : 0;
            const easeFade = (1 - lifeRatio) * (1 - lifeRatio * 0.32);
            const alpha = p.a != null ? p.a : Math.max(0, easeFade) * (p.smoke ? 0.44 : 0.88);
            if (alpha <= 0.01) return;
            const col = p.col || '#fff';
            const r = p.r || p.size || 2;
            const isAdditive = !!(p.spark || p.fireTrail || p.shape === 'ringlet');

            if (isAdditive) c.globalCompositeOperation = 'lighter';
            c.globalAlpha = alpha;
            c.fillStyle = col;

            if (p.smoke) {
                const sr = r * (0.72 + lifeRatio * 1.35);
                c.beginPath();
                c.arc(p.x, p.y, sr, 0, Math.PI * 2);
                c.fill();
                if (lifeRatio < 0.3) {
                    c.fillStyle = p.coreCol || '#ff9944';
                    c.globalAlpha = alpha * (1 - lifeRatio / 0.3) * 0.48;
                    c.beginPath();
                    c.arc(p.x, p.y, sr * 0.46, 0, Math.PI * 2);
                    c.fill();
                }
            } else if (p.debris) {
                c.save();
                c.translate(p.x, p.y);
                c.rotate((p.rot || 0) + p.t * (p.rotSpd ? p.rotSpd * 0.001 : 0.01));
                c.fillRect(-r, -r * 0.6, r * 2, r * 1.2);
                if (lifeRatio < 0.55) {
                    c.fillStyle = '#fff0b8';
                    c.globalAlpha = alpha * (1 - lifeRatio / 0.55) * 0.72;
                    c.fillRect(-r, -r * 0.6, r * 1.4, Math.max(0.8, r * 0.45));
                }
                c.restore();
            } else if (p.shape === 'ringlet') {
                c.strokeStyle = col;
                c.lineWidth = Math.max(0.9, (1 - lifeRatio * 0.5) * 1.5);
                c.beginPath();
                c.arc(p.x, p.y, Math.max(1, r * (0.5 + lifeRatio * 1.4)), 0, Math.PI * 2);
                c.stroke();
            } else if (p.shape === 'diamond') {
                const dr = (r + (p.spark ? 0.65 : 0)) * (1 - lifeRatio * 0.25);
                c.beginPath();
                c.moveTo(p.x, p.y - dr);
                c.lineTo(p.x + dr, p.y);
                c.lineTo(p.x, p.y + dr);
                c.lineTo(p.x - dr, p.y);
                c.closePath();
                c.fill();
                if (p.spark && lifeRatio < 0.45) {
                    c.fillStyle = '#ffffff';
                    c.globalAlpha = alpha * 0.7;
                    c.fillRect(p.x - 0.6, p.y - 0.6, 1.2, 1.2);
                }
            } else if (p.shape === 'circle') {
                const cr = p.fireTrail ? r * (1 - lifeRatio * 0.45) : r;
                c.beginPath();
                c.arc(p.x, p.y, Math.max(0.5, cr), 0, Math.PI * 2);
                c.fill();
                if (p.fireTrail && alpha > 0.35) {
                    c.fillStyle = '#ffffff';
                    c.globalAlpha = alpha * 0.65;
                    c.beginPath();
                    c.arc(p.x, p.y, Math.max(0.4, cr * 0.45), 0, Math.PI * 2);
                    c.fill();
                }
            } else if (p.shape === 'square') {
                c.fillRect(p.x - r, p.y - r, r * 2, r * 2);
            } else {
                // Default rect/spark: velocity-aligned streak with white-hot tip for fast sparks, crisp pixel otherwise
                const spdSq = (p.vx || 0) * (p.vx || 0) + (p.vy || 0) * (p.vy || 0);
                if (p.spark && spdSq > 2500) {
                    c.strokeStyle = col;
                    c.lineWidth = Math.max(1, p.size || 1.2);
                    c.beginPath();
                    c.moveTo(p.x, p.y);
                    c.lineTo(p.x - (p.vx || 0) * 0.032, p.y - (p.vy || 0) * 0.032);
                    c.stroke();
                    if (lifeRatio < 0.6) {
                        c.fillStyle = '#ffffff';
                        c.globalAlpha = alpha * 0.85;
                        c.fillRect(Math.round(p.x) - 0.5, Math.round(p.y) - 0.5, 1.5, 1.5);
                    }
                } else {
                    const sz = p.size || 2;
                    c.fillRect(Math.round(p.x), Math.round(p.y), sz, sz);
                }
            }

            if (isAdditive) c.globalCompositeOperation = 'source-over';
            c.globalAlpha = 1;
        }

        function drawShockwave(c, x, y, r, col, progress) {
            if (r <= 0.5) return;
            const prog = progress != null ? Math.max(0, Math.min(1, progress)) : 0.35;
            const fade = Math.max(0, 1 - prog);
            c.save();
            c.globalCompositeOperation = 'lighter';
            if (prog < 0.35 && ctx.cachedRadialGradient && r > 4) {
                const glowR = Math.max(6, Math.round(r * 0.75));
                c.globalAlpha = (1 - prog / 0.35) * 0.28;
                c.fillStyle = ctx.cachedRadialGradient(c, 'shock:' + (col || '#fff'), 0, 0, 0, glowR, [[0, '#ffffff'], [0.45, col || '#ffd88a'], [1, 'rgba(0,0,0,0)']]);
                c.save();
                c.translate(Math.round(x), Math.round(y));
                c.beginPath();
                c.arc(0, 0, glowR, 0, Math.PI * 2);
                c.fill();
                c.restore();
            }
            c.strokeStyle = col || '#fff';
            c.lineWidth = Math.max(0.9, 2.8 * (1 - prog * 0.55));
            c.globalAlpha = 0.72 * fade;
            c.beginPath();
            c.arc(x, y, r, 0, Math.PI * 2);
            c.stroke();
            if (r > 6) {
                c.strokeStyle = '#ffffff';
                c.lineWidth = Math.max(0.7, 1.2 * (1 - prog * 0.4));
                c.globalAlpha = 0.42 * fade;
                c.beginPath();
                c.arc(x, y, r * 0.87, 0, Math.PI * 2);
                c.stroke();
            }
            c.restore();
        }

        ctx.drawParticle = drawParticle;
        ctx.drawShockwave = drawShockwave;
    };
})();
