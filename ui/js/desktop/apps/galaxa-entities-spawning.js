(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    // NOTE: For this task, only the scaffold is created.
    // Functions will be moved in subsequent tasks. This avoids a single huge commit.
    GC.createEntitiesSpawning = function (ctx) {
        // Spawn patterns and formation generation
        function mkFormation(stage, count) {
            const out = [];
            for (let r = 0; r < GC.FROWS; r++) {
                for (let c = 0; c < GC.FCOLS; c++) {
                    if (out.length >= count) break;
                    out.push({ x: GC.ESP_X + c * 36, y: GC.ESP_Y + r * 32, row: r, col: c, state: 'entering' });
                }
            }
            return out;
        }

        ctx.mkFormation = mkFormation;
    };
})();
