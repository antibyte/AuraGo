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
    };
})();
