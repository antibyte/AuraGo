(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    const SUPER_DURATIONS = {
        charge: 400,
        classic_burst: 800,
        interceptor_burst: 600,
        heavy_burst: 1200,
        stealth_burst: 1000,
        aftermath: 500,
        cooldown: 8000
    };

    GC.createSupers = function (ctx) {
        function startSuper() {
            const G = ctx.G;
            if (G.superPhase !== 'idle' || G.superMeter < 100) return false;
            G.superPhase = 'charge';
            G.superPhaseT = 0;
            G.superBurstFired = false;
            G.superFreezeWorld = false;
            G.superType = ctx.settings.ship || 'classic';
            G.superActive = SUPER_DURATIONS[G.superType + '_burst'] || SUPER_DURATIONS.classic_burst;
            G.p.alive && (G.p.inv = SUPER_DURATIONS.charge + SUPER_DURATIONS.classic_burst + SUPER_DURATIONS.aftermath + 200);
            ctx.SFX.superChargeStart();
            return true;
        }

        function updateSupers(dt) {
            const G = ctx.G;
            if (G.superPhase === 'idle') {
                if (G.superMeter < 100) G.superMeter = Math.min(100, G.superMeter + dt * 0.001);
                return;
            }

            G.superPhaseT += dt;
            const ship = ctx.settings.ship || 'classic';
            let phaseDur = 0;
            if (G.superPhase === 'charge') phaseDur = SUPER_DURATIONS.charge;
            else if (G.superPhase === 'burst') {
                phaseDur = SUPER_DURATIONS[ship + '_burst'] || SUPER_DURATIONS.classic_burst;
                if (!G.superBurstFired) {
                    triggerBurst(ship);
                    G.superBurstFired = true;
                    G.superFreezeWorld = true;
                }
            }
            else if (G.superPhase === 'aftermath') phaseDur = SUPER_DURATIONS.aftermath;
            else if (G.superPhase === 'cooldown') phaseDur = SUPER_DURATIONS.cooldown;

            if (G.superPhaseT >= phaseDur) {
                G.superPhaseT = 0;
                if (G.superPhase === 'charge') {
                    G.superPhase = 'burst';
                } else if (G.superPhase === 'burst') {
                    G.superPhase = 'aftermath';
                    G.superFreezeWorld = false;
                    G.comboMult = Math.min(16, G.comboMult + 2);
                    setTimeout(() => ctx.duckMusic(0.3, 1000), 100);
                } else if (G.superPhase === 'aftermath') {
                    G.superPhase = 'cooldown';
                    G.camZoom = 1;
                } else if (G.superPhase === 'cooldown') {
                    G.superPhase = 'idle';
                    G.superMeter = 0;
                }
            }

            // Camera tween during charge
            if (G.superPhase === 'charge') {
                G.camZoom = 1 + 0.15 * (G.superPhaseT / SUPER_DURATIONS.charge);
            }
        }

        function triggerBurst(shipType) {
            // Each ship fires its specific burst
            if (shipType === 'classic') burstClassic();
            else if (shipType === 'interceptor') burstInterceptor();
            else if (shipType === 'heavy') burstHeavy();
            else if (shipType === 'stealth') burstStealth();
        }

        function burstClassic() {
            const G = ctx.G;
            const p = G.p;
            // 24 bullets in 360° spread
            for (let i = 0; i < 24; i++) {
                const a = (i / 24) * Math.PI * 2;
                G.bul.push({ x: p.x, y: p.y - 8, w: 3, h: 6, vx: Math.cos(a) * 300, vy: Math.sin(a) * 300, kind: 'nova' });
            }
            ctx.SFX.superNovaBarrage();
        }

        function burstInterceptor() {
            const G = ctx.G;
            const p = G.p;
            // Teleport to opposite side + spawn trailing blades
            p.x = GC.W - p.x;
            for (let i = 0; i < 8; i++) {
                setTimeout(() => {
                    G.bul.push({ x: p.x, y: p.y - 8, w: 2, h: 10, vx: 0, vy: -500, kind: 'phase_blade', dmg: 3 });
                }, i * 50);
            }
            ctx.SFX.superPhaseDash();
        }

        function burstHeavy() {
            const G = ctx.G;
            const p = G.p;
            // Wide front beam for 1.2s (handled via updateSupers timer)
            G.beam = { x: p.x, y: p.y - 8, w: 200, h: 20, life: 1200, dmg: 5, kind: 'aegis' };
            ctx.SFX.superAegisCannon();
        }

        function burstStealth() {
            const G = ctx.G;
            const p = G.p;
            // 3 clones fire synchronously for 1s
            G.clones = [];
            for (let i = 0; i < 3; i++) {
                const offset = (i - 1) * 60;
                G.clones.push({ x: p.x + offset, y: p.y, life: 1000, fireT: 0 });
            }
            ctx.SFX.superShadowClone();
        }

        ctx.startSuper = startSuper;
        ctx.updateSupers = updateSupers;
    };
})();
