(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createRenderEffects = function (ctx) {
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

        function drawParticle(c, p) {
            c.globalAlpha = p.a != null ? p.a : 1;
            c.fillStyle = p.col || '#fff';
            if (p.shape === 'circle') {
                c.beginPath();
                c.arc(p.x, p.y, p.r || 2, 0, Math.PI * 2);
                c.fill();
            } else if (p.shape === 'square') {
                c.fillRect(p.x - (p.r || 2), p.y - (p.r || 2), (p.r || 2) * 2, (p.r || 2) * 2);
            } else {
                // default: small square
                c.fillRect(p.x - 1, p.y - 1, 2, 2);
            }
            c.globalAlpha = 1;
        }

        function drawShockwave(c, x, y, r, col) {
            c.strokeStyle = col || '#fff';
            c.lineWidth = 2;
            c.globalAlpha = 0.6;
            c.beginPath();
            c.arc(x, y, r, 0, Math.PI * 2);
            c.stroke();
            c.globalAlpha = 1;
        }

        ctx.drawParticle = drawParticle;
        ctx.drawShockwave = drawShockwave;

        ctx.renderFlame = renderFlame;
    };
})();
