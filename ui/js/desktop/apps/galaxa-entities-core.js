(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createEntitiesCore = function (ctx) {
        // Shared utilities used by entities-core, entities-spawning, entities-behaviors
        function rectsOverlap(a, b) {
            return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;
        }

        function dist(x1, y1, x2, y2) { return Math.hypot(x2 - x1, y2 - y1); }

        function clamp(v, lo, hi) { return Math.max(lo, Math.min(hi, v)); }

        function lerp(a, b, t) { return a + (b - a) * t; }

        ctx.entsRectOverlap = rectsOverlap;
        ctx.entsDist = dist;
        ctx.entsClamp = clamp;
        ctx.entsLerp = lerp;
    };
})();
