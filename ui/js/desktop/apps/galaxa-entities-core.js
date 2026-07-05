(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createEntitiesCore = function (ctx) {
        // Shared utilities used by entities-core, entities-spawning, entities-behaviors
        function rectsOverlap(a, b) {
            return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;
        }

        // OPTIMIZATION: Math.hypot handles overflow/underflow gracefully but is
        // ~3-4x slower than Math.sqrt with squared deltas. Since all in-game
        // distances are bounded by the canvas size (well within float range),
        // use the squared form when only a comparison is needed (distSq < r*r)
        // and the sqrt form otherwise. This is a hot path during bullet/enemy
        // collision checks that runs hundreds of times per frame.
        function dist(x1, y1, x2, y2) {
            const dx = x2 - x1, dy = y2 - y1;
            return Math.sqrt(dx * dx + dy * dy);
        }
        function distSq(x1, y1, x2, y2) {
            const dx = x2 - x1, dy = y2 - y1;
            return dx * dx + dy * dy;
        }

        function clamp(v, lo, hi) { return Math.max(lo, Math.min(hi, v)); }

        function lerp(a, b, t) { return a + (b - a) * t; }

        ctx.entsRectOverlap = rectsOverlap;
        ctx.entsDist = dist;
        ctx.entsDistSq = distSq;
        ctx.entsClamp = clamp;
        ctx.entsLerp = lerp;
    };
})();
