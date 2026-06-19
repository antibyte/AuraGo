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

            // Apply new biome at wipe complete
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
            const total = TRANSITION_DURATIONS.total;

            // Phase 1: Wipe (0-300ms)
            if (t < TRANSITION_DURATIONS.wipe) {
                // FIX: cap wipeY at GC.H/2 so top and bottom bars meet at the middle
                // and stop, leaving the middle band of the screen always visible.
                // Without the cap, the bars overlap after t=150ms and cover the entire
                // canvas with black, hiding the game until the lens-flare phase starts.
                const wipeY = Math.min(GC.H / 2, (t / TRANSITION_DURATIONS.wipe) * (GC.H / 2));
                c.fillStyle = '#000';
                c.fillRect(0, 0, GC.W, wipeY);
                c.fillRect(0, GC.H - wipeY, GC.W, wipeY);
            }
            // Phase 2: Lens Flare (300-800ms)
            else if (t < TRANSITION_DURATIONS.wipe + TRANSITION_DURATIONS.flare) {
                const flareT = (t - TRANSITION_DURATIONS.wipe) / TRANSITION_DURATIONS.flare;
                const intensity = 1 - flareT;
                const grad = c.createRadialGradient(GC.W / 2, GC.H / 2, 0, GC.W / 2, GC.H / 2, 300);
                grad.addColorStop(0, `rgba(255,255,255,${intensity * 0.9})`);
                grad.addColorStop(1, 'rgba(255,255,255,0)');
                c.fillStyle = grad;
                c.fillRect(0, 0, GC.W, GC.H);
            }
            // Phase 3: Biome name text (500-1000ms)
            else if (t < TRANSITION_DURATIONS.wipe + TRANSITION_DURATIONS.flare + TRANSITION_DURATIONS.text) {
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
