(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createPlanets = function (ctx) {
        const cache = new Map();
        let generation = 0, current = null, previous = null, transition = 0, feedback = null, blendCanvas = null;
        function clearSlot(id) {
            const slot = cache.get(id);
            if (slot) { slot.image.onload = slot.image.onerror = null; slot.image.src = ''; cache.delete(id); }
        }
        function resetPlanets() {
            generation++; for (const id of cache.keys()) clearSlot(id);
            current = previous = null; transition = 0; blendCanvas = null;
            if (feedback) { feedback.remove(); feedback = null; }
        }
        function showError() {
            if (feedback || !ctx.wrapEl || ctx.state.disposed) return;
            feedback = document.createElement('div'); feedback.className = 'galaxa-planet-status'; feedback.setAttribute('role', 'status');
            const label = document.createElement('span'); label.textContent = ctx.t('galaxa.planet_load_failed');
            const retry = document.createElement('button'); retry.type = 'button'; retry.textContent = ctx.t('galaxa.retry');
            retry.addEventListener('keydown', event => event.stopPropagation());
            retry.addEventListener('click', () => {
                for (const [id, slot] of cache) if (slot.failed) clearSlot(id);
                feedback.remove(); feedback = null;
            });
            feedback.append(label, retry); ctx.wrapEl.appendChild(feedback);
        }
        function ensureSheet(id, wanted) {
            if (cache.has(id)) return cache.get(id);
            for (const key of cache.keys()) if (!wanted.has(key)) clearSlot(key);
            if (cache.size >= ctx.PLANET_SHEET_LIMIT) return null;
            const sheet = ctx.spriteAssets.manifest.sheets[id];
            if (!sheet || sheet.optional !== 'planet') return null;
            const image = new Image(), token = generation, run = ctx.G.runGeneration;
            const slot = { image, ready: false, failed: false }; cache.set(id, slot);
            const live = () => token === generation && run === ctx.G.runGeneration && !ctx.state.disposed && cache.get(id) === slot;
            image.onload = () => {
                if (!live()) return;
                if (image.naturalWidth !== sheet.width || image.naturalHeight !== sheet.height) { slot.failed = true; showError(); return; }
                slot.ready = true;
                if (![...cache.values()].some(s => s.failed) && feedback) { feedback.remove(); feedback = null; }
            };
            image.onerror = () => { if (live()) { slot.failed = true; showError(); } };
            image.src = '/img/galaxa/' + sheet.file + '?v=' + GC.ART_VERSION;
            return slot;
        }
        function entry(planet) { return planet && ctx.spriteAssets.manifest.animations['planet.' + planet.id]; }
        function indices(planet) {
            const a = entry(planet), time = ctx.settings.reducedMotion ? 0 : planet.time;
            const frame = (time / a.ms) % a.frames.length;
            return { a, index: Math.floor(frame), next: (Math.floor(frame) + 1) % a.frames.length, mix: frame % 1 };
        }
        function sheetFor(a, index) { return a.frameSheets ? a.frameSheets[index] : a.sheet; }
        function updatePlanets(dt) {
            if (!ctx.spriteAssets || ctx.state.disposed) return;
            const biome = ctx.expansionSector(), pool = ctx.PLANET_ART[biome] || ctx.PLANET_ART.nebula;
            const id = pool[Math.max(0, ctx.G.stage - 1) % pool.length];
            if (!ctx.spriteAssets.manifest.animations['planet.' + id]) return;
            if (!current || current.id !== id) {
                if (current) {
                    const old = indices(current), slot = cache.get(sheetFor(old.a, old.index));
                    previous = slot && slot.ready ? { ...current, frozen: old.index } : null;
                }
                const n = Object.values(ctx.PLANET_ART).flat().indexOf(id);
                current = { id, time: 0, diameter: 180 + n % 5 * 40, x: n % 2 ? ctx.W - 28 : 28, speed: 0.8 + n % 5 * 0.2 };
                transition = 0;
            }
            const next = indices(current);
            const wanted = new Set([sheetFor(next.a, next.index)]);
            if (previous) wanted.add(sheetFor(entry(previous), previous.frozen));
            else wanted.add(sheetFor(next.a, next.next));
            for (const key of cache.keys()) if (!wanted.has(key)) clearSlot(key);
            for (const key of wanted) ensureSheet(key, wanted);
            const ready = cache.get(sheetFor(next.a, next.index))?.ready;
            if (previous && ready) {
                transition += dt * 1000;
                if (ctx.settings.reducedMotion || transition >= 3000) previous = null;
            } else if (!previous && ready && !ctx.settings.reducedMotion) current.time += dt * 1000;
        }
        function paint(c, planet, index, alpha) {
            if (alpha <= 0) return;
            const a = entry(planet), slot = cache.get(sheetFor(a, index));
            if (!slot || !slot.ready) return;
            const f = a.frames[index], anchor = a.sourceAnchors ? a.sourceAnchors[index] : [f[2] / 2, f[3] / 2];
            const ratio = planet.diameter / (a.sourceSize || f[2]);
            const parallax = ctx.settings.reducedMotion ? 0 : Math.max(-2, Math.min(2, (ctx.G.p.x - ctx.W / 2) / (ctx.W / 2) * 2));
            const y = ((planet.time / 1000 * planet.speed + planet.diameter + 100) % (ctx.H + planet.diameter * 2)) - planet.diameter;
            c.globalAlpha = alpha;
            c.drawImage(slot.image, ...f, planet.x - parallax - anchor[0] * ratio, y - anchor[1] * ratio, f[2] * ratio, f[3] * ratio);
        }
        function drawPlanets(c) {
            if (!current || !ctx.spriteAssets) return;
            if (!blendCanvas) { blendCanvas = document.createElement('canvas'); blendCanvas.width = ctx.W; blendCanvas.height = ctx.H; }
            const layer = blendCanvas.getContext('2d');
            layer.clearRect(0, 0, ctx.W, ctx.H); layer.imageSmoothingEnabled = true;
            // Blend premultiplied frames before compositing so the planet body stays opaque.
            layer.globalCompositeOperation = 'lighter';
            const f = indices(current), nextReady = cache.get(sheetFor(f.a, f.next))?.ready;
            if (previous) {
                const blend = Math.min(1, transition / 3000);
                paint(layer, previous, previous.frozen, 1 - blend); paint(layer, current, f.index, blend);
            } else {
                const mix = ctx.settings.reducedMotion || !nextReady ? 0 : f.mix;
                paint(layer, current, f.index, 1 - mix); if (mix) paint(layer, current, f.next, mix);
            }
            // Keep distant planets subdued by tinting their color, not reducing opacity.
            layer.globalCompositeOperation = 'source-atop'; layer.globalAlpha = 0.32;
            layer.fillStyle = '#050a17'; layer.fillRect(0, 0, ctx.W, ctx.H);
            c.save(); c.imageSmoothingEnabled = true; c.globalAlpha = 1;
            c.drawImage(blendCanvas, 0, 0);
            c.restore();
        }
        Object.assign(ctx, { planetCache: cache, updatePlanets, drawPlanets, resetPlanets, disposePlanets: resetPlanets });
    };
})();
