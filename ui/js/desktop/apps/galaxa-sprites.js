(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    const ROOT = '/img/galaxa/';
    GC.ART_VERSION = window.BUILD_VERSION || window.AURAGO_BUILD_VERSION || 'dev';
    let pendingAssets;
    function validateManifest(data) {
        if (!data || data.version !== 1 || !data.sheets || !data.animations) throw Error('Invalid Galaxa atlas manifest');
        for (const sheet of Object.values(data.sheets)) {
            if (!/^[a-z0-9-]+\.png$/.test(sheet.file) || !Number.isInteger(sheet.width) || !Number.isInteger(sheet.height) || sheet.width <= 0 || sheet.height <= 0) throw Error('Invalid atlas sheet');
        }
        for (const scene of Object.values(data.scenes || {})) {
            const s = data.sheets[scene.sheet], f = scene.frame;
            if (!s || !Array.isArray(f) || f.length !== 4 || !f.every(Number.isFinite) || f[0] < 0 || f[1] < 0 || f[2] <= 0 || f[3] <= 0 || f[0] + f[2] > s.width || f[1] + f[3] > s.height) throw Error('Invalid background frame');
        }
        for (const [key, a] of Object.entries(data.animations)) {
            const sheet = data.sheets[a.sheet];
            if (!sheet || !/^[a-z0-9-]+\.png$/.test(sheet.file) || !Number.isFinite(a.ms) || a.ms <= 0 ||
                !Number.isFinite(a.size) || a.size <= 0 || !Array.isArray(a.pivot) || a.pivot.length !== 2 ||
                !a.pivot.every(n => Number.isFinite(n) && n >= 0 && n <= 1) || !Array.isArray(a.frames) || !a.frames.length) throw Error('Invalid animation: ' + key);
            if (a.sourceAnchors && (!Number.isFinite(a.sourceSize) || a.sourceSize <= 0 || !Array.isArray(a.sourceAnchors) || a.sourceAnchors.length !== a.frames.length || a.sourceAnchors.some((p, i) => !Array.isArray(p) || p.length !== 2 || !p.every(Number.isFinite) || p[0] < 0 || p[1] < 0 || p[0] > a.frames[i][2] || p[1] > a.frames[i][3]))) throw Error('Invalid source anchors: ' + key);
            for (const f of a.frames) {
                if (!Array.isArray(f) || f.length !== 4 || !f.every(Number.isFinite) ||
                    f[0] < 0 || f[1] < 0 || f[2] <= 0 || f[3] <= 0 ||
                    f[0] + f[2] > sheet.width || f[1] + f[3] > sheet.height) throw Error('Invalid frame: ' + key);
            }
        }
        function require(key, count) { if (!data.animations[key] || data.animations[key].frames.length < count) throw Error('Incomplete animation: ' + key); }
        for (const ship of Object.keys(GC.SHIP_TYPES)) {
            for (const state of ['idle', 'left', 'right', 'fire', 'boost', 'super']) require('player.' + ship + '.' + state, state === 'idle' || state === 'super' ? 8 : 4);
            require('icon.' + ship, 1);
        }
        for (const type of new Set(GC.SECTORS.flatMap(s => s.enemies).concat(['boss', 'miniboss']))) {
            require(type + '.idle', 8); require(type + '.attack', 6); require(type + '.damage', 2);
        }
        for (const sector of GC.SECTORS) {
            if (!data.scenes || !data.scenes[sector.id]) throw Error('Missing sector scene: ' + sector.id);
            for (let phase = 1; phase <= 3; phase++) require('sector.' + sector.id + '.phase' + phase, 8);
            require('sector.' + sector.id + '.death', 16);
        }
        for (const size of ['small', 'medium', 'large']) require('fx.explosion.' + size, 16);
        for (const name of ['shield', 'parry', 'plasma']) require('fx.' + name, 8);
        for (const type of ['normal', 'laser', 'rocket', 'plasma', 'ion', 'crystal', 'gravity', 'mine', 'bolt']) require('projectile.' + type, 2);
        for (const type of [...GC.PU_TYPES, ...Object.values(GC.PU_UPGRADE)]) require('pickup.' + type, 2);
        return data;
    }
    function loadAssets() {
        if (pendingAssets) return pendingAssets;
        pendingAssets = (async () => {
            const response = await fetch(ROOT + 'atlas.json?v=' + GC.ART_VERSION);
            if (!response.ok) throw Error('Galaxa atlas HTTP ' + response.status);
            const manifest = validateManifest(await response.json()), images = {};
            await Promise.all(Object.entries(manifest.sheets).map(([id, sheet]) => new Promise((resolve, reject) => {
                const img = new Image();
                img.onload = () => {
                    if (img.naturalWidth !== sheet.width || img.naturalHeight !== sheet.height) return reject(Error('Atlas dimensions: ' + id));
                    images[id] = img; resolve();
                };
                img.onerror = () => reject(Error('Atlas load: ' + id));
                img.src = ROOT + sheet.file + '?v=' + GC.ART_VERSION;
            })));
            return { manifest, images };
        })().catch(err => { pendingAssets = null; throw err; });
        return pendingAssets;
    }
    GC.validateAtlasManifest = validateManifest;
    GC.loadAtlasAssets = loadAssets;
    GC.createSprites = function (ctx) {
        const radialGradientCache = new Map(), spriteAtlasCache = new Map();
        let assets;
        const palette = { 1: '#f1f8ff', 2: '#74d6ff', 3: '#3684da', 4: '#243d72' };
        const SP = { player: 'player.classic.idle', playerIcon: 'icon.classic', pC: palette,
            playerFrames: ['player.classic.idle'], PLAYER_FRAME: { idleA: 0 }, pwShield: 'fx.shield', pwC: palette };
        function animation(key, time, loopOverride) {
            if (!assets) return null;
            const a = assets.manifest.animations[key];
            if (!a) throw Error('Missing Galaxa animation: ' + key);
            const t = Math.max(0, time || 0), loop = loopOverride === undefined ? a.loop : loopOverride;
            const index = loop ? Math.floor(t / a.ms) % a.frames.length : Math.min(a.frames.length - 1, Math.floor(t / a.ms));
            return { key, a, index, frame: a.frames[index] };
        }
        function drawAnimation(cv, key, x, y, time, size, tint, loop) {
            const entry = animation(key, time, loop);
            if (!entry) return;
            const { a, frame, index } = entry, width = size || a.size;
            const cacheKey = key + ':' + index + ':' + width + ':' + (tint || '');
            let sprite = spriteAtlasCache.get(cacheKey);
            if (!sprite) {
                const ratio = a.sourceAnchors ? width / a.sourceSize : 1, anchor = a.sourceAnchors && a.sourceAnchors[index];
                const extent = anchor ? Math.ceil(Math.max(width / 2, anchor[0] * ratio, anchor[1] * ratio, (frame[2] - anchor[0]) * ratio, (frame[3] - anchor[1]) * ratio)) * 2 : width;
                sprite = document.createElement('canvas'); sprite.width = sprite.height = extent;
                const c = sprite.getContext('2d'); c.imageSmoothingEnabled = false;
                if (anchor) {
                    c.drawImage(assets.images[a.sheet], ...frame, extent / 2 - anchor[0] * ratio, extent / 2 - anchor[1] * ratio, frame[2] * ratio, frame[3] * ratio);
                } else c.drawImage(assets.images[a.sheet], ...frame, 0, 0, width, width);
                if (tint) { c.globalCompositeOperation = 'source-atop'; c.fillStyle = tint; c.fillRect(0, 0, extent, extent); }
                // ponytail: bounded FIFO cache; revisit only if measured atlas churn matters.
                if (spriteAtlasCache.size >= 768) spriteAtlasCache.delete(spriteAtlasCache.keys().next().value);
                spriteAtlasCache.set(cacheKey, sprite);
            }
            const padding = (sprite.width - width) / 2;
            cv.drawImage(sprite, Math.round(x - width * a.pivot[0] - padding), Math.round(y - width * a.pivot[1] - padding));
        }
        function drawSp(cv, sp, cols, x, y, flash, noCache) {
            if (!assets) return;
            const ref = typeof sp === 'string' ? { key: sp, time: ctx.G.animTime || 0 } : sp;
            if (!ref || !ref.key) return;
            const a = assets.manifest.animations[ref.key];
            if (!a) throw Error('Missing sprite: ' + ref.key);
            const origin = a.origin || a.size;
            drawAnimation(cv, ref.key, x + origin / 2, y + origin / 2, ref.time, ref.size,
                flash ? '#edfaff' : noCache ? ((cols && cols[1]) || '#6dccff') : null);
        }
        function getPlayerSpriteFrame() {
            const g = ctx.G, ship = ctx.settings.ship || 'classic';
            let state = 'idle';
            if (['charge', 'burst', 'aftermath'].includes(g.superPhase)) state = 'super';
            else if (g.shipTilt < -0.04) state = 'left';
            else if (g.shipTilt > 0.04) state = 'right';
            else if (g.muzzleT > 0) state = 'fire';
            else if (g.activePU && /speed/.test(g.activePU.type)) state = 'boost';
            return { key: 'player.' + ship + '.' + state, time: state === 'fire' ? Math.max(0, 160 - g.muzzleT) : g.animTime || 0 };
        }
        function enemySpriteFor(e) {
            const type = e ? (e.sectorBoss ? 'sector.' + e.sectorBoss : e.type) : 'boss';
            const attack = e && ((e.attackAnim || 0) > 0 || e.telegraph);
            const state = e && e.sectorBoss ? 'phase' + (e.bossPhase || 1) : e && e.hitF > 0 ? 'damage' : attack ? 'attack' : 'idle';
            const time = e && e.sectorBoss ? e.attackAnim > 0 ? 800 - e.attackAnim : e.bossState === 'windup' ? Math.min(90, Math.max(0, 600 - e.attackClock)) : 0 : attack ? Math.max(0, 600 - (e.attackAnim || 600)) : (e && e.animTime) || 0;
            return { sp: { key: type + '.' + state, time }, cols: palette };
        }
        function cachedRadialGradient(c, key, x, y, inner, outer, stops) {
            const id = [key, Math.round(x), Math.round(y), Math.round(inner), Math.round(outer), JSON.stringify(stops)].join(':');
            if (radialGradientCache.has(id)) return radialGradientCache.get(id);
            const gradient = c.createRadialGradient(x, y, inner, x, y, Math.max(inner + 0.01, outer));
            stops.forEach(([at, col]) => gradient.addColorStop(at, col));
            if (radialGradientCache.size >= 128) radialGradientCache.delete(radialGradientCache.keys().next().value);
            radialGradientCache.set(id, gradient); return gradient;
        }
        ctx.loadSprites = async function () {
            assets = await loadAssets();
            if (ctx.state.disposed) return;
            SP.playerIcon = 'icon.' + ctx.settings.ship; ctx.spriteAssets = assets;
        };
        Object.assign(ctx, { SP, drawSp, drawAnimation, enemySpriteFor, getPlayerSpriteFrame,
            radialGradientCache, spriteAtlasCache, cachedRadialGradient,
            clearSpriteAtlasCache: () => spriteAtlasCache.clear(), rainbowPC: () => palette, spriteAnimation: animation });
    };
})();
