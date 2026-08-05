(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createEntitiesCore = function (ctx) {
        const particlePool = [];
        function getParticle(props) {
            const p = particlePool.length > 0 ? particlePool.pop() : {};
            Object.assign(p, props);
            return p;
        }
        function recycleParticles(arr) {
            for (let i = 0; i < arr.length; i++) {
                if (particlePool.length < 300) { const p = arr[i]; for (const k in p) delete p[k]; particlePool.push(p); }
            }
            arr.length = 0;
        }

        function rectsOverlap(a, b) {
            return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;
        }
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

        ctx.getParticle = getParticle;
        ctx.recycleParticles = recycleParticles;
        ctx.entsRectOverlap = rectsOverlap;
        ctx.entsDist = dist;
        ctx.entsDistSq = distSq;
        ctx.entsClamp = clamp;
        ctx.entsLerp = lerp;
    };
})();
