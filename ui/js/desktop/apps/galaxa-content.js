(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createExpansionContent = function (ctx) {
        ctx.expansionSector = () => ctx.settings.mode === 'gauntlet'
            ? GC.SECTORS[Math.min(5, Math.floor((ctx.G.stage - 1) / 2))].id
            : GC.SECTORS[ctx.stagePlan().sector].id;
        ctx.expansionAlien = function (type, slot, wave) {
            if (ctx.G.chal || ctx.G.bonusStage || type === 'boss' || type === 'miniboss' || ctx.settings.mode === 'boss_rush') return null;
            const pair = ctx.EXPANSION_ALIENS[ctx.expansionSector()];
            const firstWave = (ctx.G.stage - 1) % 5 === 0 && wave < 2;
            if (!pair || (firstWave && ctx.settings.mode !== 'gauntlet') || slot % 3 !== 2) return null;
            return pair[Math.floor(slot / 3) % 2];
        };
    };
})();
