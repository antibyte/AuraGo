(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createEntities = function (ctx) {
        GC.createEntitiesCore(ctx);
        GC.createEntitiesSpawning(ctx);
        GC.createEntitiesCombat(ctx);
        GC.createEntitiesBehaviors(ctx);
        const WEAPON_EVOS = {
            vulcan: { name: 'VULCAN', desc: 'Ultra-fast stream', col: '#ff8844', fireRate: 0.6, spread: 0, dmgMult: 0.7 },
            cannon: { name: 'CANNON', desc: 'Slow massive shots', col: '#ff4444', fireRate: 2.5, spread: 0, dmgMult: 4 },
            beam: { name: 'BEAM', desc: 'Continuous laser', col: '#88ccff', fireRate: 0, spread: 0, dmgMult: 0, isBeam: true }
        };
        ctx.WEAPON_EVOS = WEAPON_EVOS;

        let evoSel = 0;
        function updateEvoChoice() {
            const u = ctx.G.inp.u && !ctx.G.inp.up;
            const d = ctx.G.inp.d && !ctx.G.inp.dp;
            const f = ctx.G.inp.f && !ctx.G.inp.fp;
            if (u) evoSel = Math.max(0, evoSel - 1);
            if (d) evoSel = Math.min(2, evoSel + 1);
            if (f) {
                const evos = ['vulcan', 'cannon', 'beam'];
                ctx.G.weaponEvo = evos[evoSel];
                ctx.G.evoChoiceOpen = false;
                ctx.SFX.puUpgrade(ctx.W / 2);
                ctx.G.upgradeBanner = { text: WEAPON_EVOS[ctx.G.weaponEvo].name + '!', type: 'evolution', t: 0, dur: 2000 };
            }
        }
        ctx.updateEvoChoice = updateEvoChoice;
        ctx.evoSel = function () { return evoSel; };
    };
})();
