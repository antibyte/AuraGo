(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    const TRANSITION_DURATIONS = {
        wipe: 300,
        flare: 500,
        text: 500,
        total: 1500
    };

    GC.createBiomeTransitions = function (ctx) {
        function startTransition(newBiome) {
            const G = ctx.G;
            if (G.transitionActive) return;
            G.transitionActive = true;
            G.transitionT = 0;
            G.transitionType = 'wipe';
            G.pendingBiome = newBiome;
            ctx.duckMusic(0.5, 1500);
            setTimeout(() => ctx.SFX.biomeReveal && ctx.SFX.biomeReveal(), 300);
        }

        function updateTransitions(dt) {
            const G = ctx.G;
            if (!G.transitionActive) return;
            G.transitionT += dt;

            if (G.transitionT === dt && G.pendingBiome) {
                G.biome = G.pendingBiome.id;
                G.biomeName = G.pendingBiome.name;
                G.pendingBiome = null;
            }

            if (G.transitionT >= TRANSITION_DURATIONS.total) {
                G.transitionActive = false;
                G.transitionT = 0;
            }
        }

        function drawTransition(c, G) {
            if (!G.transitionActive) return;
            const t = G.transitionT;

            // Phase 1: letterbox wipe removed — no top/bottom bars shrinking play area
            if (t < TRANSITION_DURATIONS.wipe) {
                return;
            }
            if (t < TRANSITION_DURATIONS.wipe + TRANSITION_DURATIONS.flare) {
                const flareT = (t - TRANSITION_DURATIONS.wipe) / TRANSITION_DURATIONS.flare;
                const intensity = 1 - flareT;
                const grad = c.createRadialGradient(GC.W / 2, GC.H / 2, 0, GC.W / 2, GC.H / 2, 300);
                grad.addColorStop(0, `rgba(255,255,255,${intensity * 0.9})`);
                grad.addColorStop(1, 'rgba(255,255,255,0)');
                c.fillStyle = grad;
                c.fillRect(0, 0, GC.W, GC.H);
            } else if (t < TRANSITION_DURATIONS.wipe + TRANSITION_DURATIONS.flare + TRANSITION_DURATIONS.text) {
                const textT = (t - TRANSITION_DURATIONS.wipe - TRANSITION_DURATIONS.flare) / TRANSITION_DURATIONS.text;
                const text = G.biomeName || '';
                c.fillStyle = '#fff';
                c.font = 'bold 24px monospace';
                c.textAlign = 'center';
                c.shadowColor = '#88ccff';
                c.shadowBlur = 12 * textT;
                c.fillText(text, GC.W / 2, GC.H / 2);
                c.shadowBlur = 0;
            }
        }

        ctx.startBiomeTransition = startTransition;
        ctx.updateBiomeTransitions = updateTransitions;
        ctx.drawBiomeTransition = drawTransition;
    };
})();