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
            if (!G.stageRank) return;
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
    };
})();