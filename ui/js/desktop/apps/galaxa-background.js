(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createBackground = function (ctx) {
        const fields = [], twinkles = [], scenes = new Map();
        let nebulaCv = null, time = 0;
        for (let layer = 0; layer < 3; layer++) {
            const canvas = document.createElement('canvas'); canvas.width = ctx.W; canvas.height = ctx.H;
            const c = canvas.getContext('2d');
            for (let star = 0; star < 38 - layer * 8; star++) {
                c.fillStyle = ['#596487', '#7887a5', '#d2d3c9'][layer];
                const sx = Math.floor(Math.random() * ctx.W), sy = Math.floor(Math.random() * ctx.H);
                c.fillRect(sx, sy, layer === 2 ? 2 : 1, layer === 2 ? 2 : 1);
            }
            fields.push(canvas);
        }
        const twinkleCols = ['#ffffff', '#aaddff', '#ffe8aa', '#ccbbff'];
        for (let i = 0; i < 16; i++) {
            twinkles.push({
                x: Math.random() * ctx.W,
                y: Math.random() * ctx.H,
                spd: 14 + (i % 3) * 11,
                ph: Math.random() * Math.PI * 2,
                col: twinkleCols[i % twinkleCols.length],
                layer: (i % 2) + 1
            });
        }
        function ensureNebulaCanvas() {
            if (!nebulaCv) { nebulaCv = document.createElement('canvas'); nebulaCv.width = ctx.W; nebulaCv.height = ctx.H; }
            return nebulaCv;
        }
        function initBG() { ctx.G.bgTheme = GC.getBiomeForStage(ctx.G.stage).id; }
        function mkNebula() { ensureNebulaCanvas(); initBG(); }
        function updateBackground(dt) {
            if (ctx.settings.reducedMotion) return;
            const G = ctx.G;
            const warpBoost = (G && (G.fxWarpT > 0 || G.fxHyperTunnelT > 0)) ? 3.2 : 1;
            time += dt * warpBoost;
        }
        function drawNebula(c) {
            const G = ctx.G;
            const id = G.biome || 'nebula', assets = ctx.spriteAssets;
            if (!assets) return;
            let canvas = scenes.get(id);
            if (!canvas) {
                const scene = assets.manifest.scenes[id]; if (!scene) return;
                canvas = document.createElement('canvas'); canvas.width = ctx.W; canvas.height = ctx.H;
                const dest = canvas.getContext('2d'); dest.imageSmoothingEnabled = false;
                dest.drawImage(assets.images[scene.sheet], ...scene.frame, 0, 0, ctx.W, ctx.H);
                scenes.set(id, canvas);
            }
            const beatPulse = (!ctx.settings.reducedMotion && G && G.beatPhase != null)
                ? Math.sin(G.beatPhase * Math.PI * 2) * 0.04 : 0;
            c.globalAlpha = 0.52 + beatPulse;
            c.drawImage(canvas, 0, 0);
            c.globalAlpha = 1;
        }
        function drawStars(c) {
            const G = ctx.G;
            const reduced = !!ctx.settings.reducedMotion;
            const px = (!reduced && G && G.p) ? ((G.p.x - ctx.W / 2) / (ctx.W / 2)) : 0;
            const warping = !reduced && G && (G.fxWarpT > 0 || G.fxHyperTunnelT > 0);
            for (let layer = 0; layer < fields.length; layer++) {
                const offset = Math.floor(time * (4 + layer * 9)) % ctx.H;
                const dx = Math.round(-px * (layer + 1) * 3);
                c.globalAlpha = 0.45 + layer * 0.16;
                c.drawImage(fields[layer], dx, offset);
                c.drawImage(fields[layer], dx, offset - ctx.H);
            }
            for (let i = 0; i < twinkles.length; i++) {
                const st = twinkles[i];
                const sy = (st.y + time * st.spd) % ctx.H;
                const sx = (st.x - px * st.layer * 5 + ctx.W) % ctx.W;
                const tw = reduced ? 0.65 : (0.35 + 0.65 * (0.5 + 0.5 * Math.sin(time * 3.8 + st.ph)));
                c.globalAlpha = tw * 0.85;
                c.fillStyle = st.col;
                if (warping) {
                    const streakLen = 8 + st.layer * 6;
                    c.fillRect(Math.round(sx), Math.round(sy - streakLen), 1, streakLen);
                } else if (tw > 0.72) {
                    const rx = Math.round(sx), ry = Math.round(sy);
                    c.fillRect(rx - 2, ry, 5, 1);
                    c.fillRect(rx, ry - 2, 1, 5);
                } else {
                    c.fillRect(Math.round(sx), Math.round(sy), 1, 1);
                }
            }
            c.globalAlpha = 1;
        }
        Object.assign(ctx, { initBG, mkNebula, ensureNebulaCanvas, updateBackground, drawNebula, drawStars });
    };
})();
