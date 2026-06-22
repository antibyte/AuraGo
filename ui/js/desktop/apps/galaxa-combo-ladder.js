(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    const RANK_THRESHOLDS = {
        'S+': { noDamageRun: true, speedDemon: true, pacifist: true },
        'S': { combos: [['noDamageRun', 'speedDemon'], ['noDamageRun', 'pacifist'], ['speedDemon', 'pacifist']] },
        'A': { minBonuses: 1 },
        'B': { minCleared: true },
        'C': { minDamage: 2 }
    };

    GC.createComboLadder = function (ctx) {
        function calculateRank(boni, damagePoints) {
            if (boni.noDamageRun && boni.speedDemon && boni.pacifist) return 'S+';
            const bonusCount = (boni.noDamageRun ? 1 : 0) + (boni.speedDemon ? 1 : 0) + (boni.pacifist ? 1 : 0);
            if (bonusCount >= 2) return 'S';
            if (bonusCount >= 1) return 'A';
            if (damagePoints > 2) return 'C';
            return 'B';
        }

        function applyRiskItMultiplier(combo) {
            // Risk-It mode: exponential instead of linear
            return 1 + Math.pow(combo, 1.5) * 0.4;
        }

        function applyRiskItPenalty(stageScore) {
            return Math.floor(stageScore * 0.25);
        }

        function trackBoni(G) {
            // Called at stage end
            const stageTime = (G.stageStartTime ? performance.now() - G.stageStartTime : 30000) / 1000;
            G.stageBoni = {
                noDamageRun: G.stageDamageTaken === 0,
                speedDemon: stageTime < 30,
                pacifist: G.stageAccuracyShots === 0
            };
            G.stageRank = calculateRank(G.stageBoni, G.stageDamageTaken || 0);
        }

        function formatScore(n) {
            return n.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ',');
        }

        function clusterPopups(popups) {
            // Group overlapping popups vertically
            const clustered = [];
            for (const p of popups) {
                let stacked = false;
                for (const c of clustered) {
                    if (Math.abs(c.x - p.x) < 40 && Math.abs(c.y - p.y) < 20) {
                        c.y -= 16;
                        stacked = true;
                        break;
                    }
                }
                if (!stacked) clustered.push({ ...p });
            }
            return clustered;
        }

        ctx.calculateRank = calculateRank;
        ctx.applyRiskItMultiplier = applyRiskItMultiplier;
        ctx.applyRiskItPenalty = applyRiskItPenalty;
        ctx.trackStageBoni = trackBoni;
        ctx.formatScore = formatScore;
        ctx.clusterPopups = clusterPopups;
    };
})();
