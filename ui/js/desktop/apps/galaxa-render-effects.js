(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createRenderEffects = function (ctx) {
        function drawParticle(c, p) {
            const lifeRatio = p.life > 0 ? Math.max(0, Math.min(1, p.t / p.life)) : 0;
            const alpha = p.a != null ? p.a : Math.max(0, 1 - lifeRatio) * (p.smoke ? 0.42 : 0.85);
            if (alpha <= 0.01) return;
            const col = p.col || '#fff';
            const r = p.r || p.size || 2;
            const isAdditive = !!(p.spark || p.fireTrail);

            if (isAdditive) c.globalCompositeOperation = 'lighter';
            c.globalAlpha = alpha;
            c.fillStyle = col;

            if (p.smoke) {
                const sr = r * (0.8 + lifeRatio * 0.9);
                c.beginPath();
                c.arc(p.x, p.y, sr, 0, Math.PI * 2);
                c.fill();
            } else if (p.debris) {
                c.save();
                c.translate(p.x, p.y);
                c.rotate(p.rot || (p.t * 0.01));
                c.fillRect(-r, -r * 0.6, r * 2, r * 1.2);
                c.restore();
            } else if (p.shape === 'diamond') {
                const dr = r + (p.spark ? 0.6 : 0);
                c.beginPath();
                c.moveTo(p.x, p.y - dr);
                c.lineTo(p.x + dr, p.y);
                c.lineTo(p.x, p.y + dr);
                c.lineTo(p.x - dr, p.y);
                c.closePath();
                c.fill();
            } else if (p.shape === 'circle') {
                const cr = p.fireTrail ? r * (1 - lifeRatio * 0.45) : r;
                c.beginPath();
                c.arc(p.x, p.y, Math.max(0.5, cr), 0, Math.PI * 2);
                c.fill();
                if (p.fireTrail && alpha > 0.35) {
                    c.fillStyle = '#ffffff';
                    c.globalAlpha = alpha * 0.6;
                    c.beginPath();
                    c.arc(p.x, p.y, Math.max(0.4, cr * 0.45), 0, Math.PI * 2);
                    c.fill();
                }
            } else if (p.shape === 'square') {
                c.fillRect(p.x - r, p.y - r, r * 2, r * 2);
            } else {
                // Default rect/spark: velocity-aligned streak for fast sparks, crisp pixel otherwise
                const spdSq = (p.vx || 0) * (p.vx || 0) + (p.vy || 0) * (p.vy || 0);
                if (p.spark && spdSq > 2500) {
                    c.strokeStyle = col;
                    c.lineWidth = Math.max(1, p.size || 1.2);
                    c.beginPath();
                    c.moveTo(p.x, p.y);
                    c.lineTo(p.x - (p.vx || 0) * 0.028, p.y - (p.vy || 0) * 0.028);
                    c.stroke();
                } else {
                    const sz = p.size || 2;
                    c.fillRect(Math.round(p.x), Math.round(p.y), sz, sz);
                }
            }

            if (isAdditive) c.globalCompositeOperation = 'source-over';
            c.globalAlpha = 1;
        }

        function drawShockwave(c, x, y, r, col) {
            c.save();
            c.globalCompositeOperation = 'lighter';
            c.strokeStyle = col || '#fff';
            c.lineWidth = 2.2;
            c.globalAlpha = 0.65;
            c.beginPath();
            c.arc(x, y, r, 0, Math.PI * 2);
            c.stroke();
            if (r > 6) {
                c.strokeStyle = '#ffffff';
                c.lineWidth = 1;
                c.globalAlpha = 0.35;
                c.beginPath();
                c.arc(x, y, r * 0.88, 0, Math.PI * 2);
                c.stroke();
            }
            c.restore();
        }

        ctx.drawParticle = drawParticle;
        ctx.drawShockwave = drawShockwave;
    };
})();
