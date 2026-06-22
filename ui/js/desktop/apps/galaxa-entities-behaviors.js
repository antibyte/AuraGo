(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    // AI behaviors: dive patterns, attack waves, formation updates
    GC.createEntitiesBehaviors = function (ctx) {
        function updateEnemyAI(e, dt, p) {
            // Default behavior: stay in formation, occasionally dive
            if (e.state === 'formation') {
                e.diveTmr = (e.diveTmr || 0) - dt;
                if (e.diveTmr <= 0 && Math.random() < 0.001 * dt) {
                    e.state = 'diving';
                    e.targetX = p.x;
                    e.targetY = p.y;
                }
            } else if (e.state === 'diving') {
                e.x += (e.targetX - e.x) * 0.02 * dt;
                e.y += GC.DIVE_SPD * dt * 0.06;
                if (e.y > GC.H + 50) e.state = 'returning';
            } else if (e.state === 'returning') {
                e.y -= GC.DIVE_SPD * dt * 0.04;
                if (e.y < e.row * 32 + GC.ESP_Y) e.state = 'formation';
            }
        }

        ctx.updateEnemyAI = updateEnemyAI;
    };
})();
