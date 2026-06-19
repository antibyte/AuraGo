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

        ctx.drawSuperMeterHUD = drawSuperMeter;
        ctx.drawArchetypeHUD = drawArchetypeHUD;
    };
})();