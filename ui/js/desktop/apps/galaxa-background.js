(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createBackground = function (ctx) {
        const fields = [], scenes = new Map();
        let nebulaCv = null, time = 0;
        for (let layer = 0; layer < 3; layer++) {
            const canvas = document.createElement('canvas'); canvas.width = ctx.W; canvas.height = ctx.H;
            const c = canvas.getContext('2d');
            for (let star = 0; star < 35 - layer * 8; star++) {
                c.fillStyle = ['#596487', '#7887a5', '#d2d3c9'][layer];
                c.fillRect(Math.floor(Math.random() * ctx.W), Math.floor(Math.random() * ctx.H), layer === 2 ? 2 : 1, 1);
            }
            fields.push(canvas);
        }
        function ensureNebulaCanvas() {
            if (!nebulaCv) { nebulaCv = document.createElement('canvas'); nebulaCv.width = ctx.W; nebulaCv.height = ctx.H; }
            return nebulaCv;
        }
        function initBG() { ctx.G.bgTheme = GC.getBiomeForStage(ctx.G.stage).id; }
        function mkNebula() { ensureNebulaCanvas(); initBG(); }
        function updateBackground(dt) { if (!ctx.settings.reducedMotion) time += dt; }
        function drawNebula(c) {
            const id = ctx.G.biome || 'nebula', assets = ctx.spriteAssets;
            if (!assets) return;
            let canvas = scenes.get(id);
            if (!canvas) {
                const scene = assets.manifest.scenes[id]; if (!scene) return;
                canvas = document.createElement('canvas'); canvas.width = ctx.W; canvas.height = ctx.H;
                const dest = canvas.getContext('2d'); dest.imageSmoothingEnabled = false;
                dest.drawImage(assets.images[scene.sheet], ...scene.frame, 0, 0, ctx.W, ctx.H);
                scenes.set(id, canvas);
            }
            c.globalAlpha = 0.52; c.drawImage(canvas, 0, 0); c.globalAlpha = 1;
        }
        function drawStars(c) {
            for (let layer = 0; layer < fields.length; layer++) {
                const offset = Math.floor(time * (4 + layer * 9)) % ctx.H;
                c.globalAlpha = 0.45 + layer * 0.15; c.drawImage(fields[layer], 0, offset); c.drawImage(fields[layer], 0, offset - ctx.H);
            }
            c.globalAlpha = 1;
        }
        Object.assign(ctx, { initBG, mkNebula, ensureNebulaCanvas, updateBackground, drawNebula, drawStars });
    };
})();
