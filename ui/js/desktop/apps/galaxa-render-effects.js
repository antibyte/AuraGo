(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createRenderEffects = function (ctx) {
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
    };
})();
