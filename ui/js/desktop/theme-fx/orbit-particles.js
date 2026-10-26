(function () {
    'use strict';

    // Orbit particles (global AuraOrbitParticles). A DOM-free, bounded particle pool with the emitters of the theme and
    // a Canvas 2D additive renderer, so a burst of window events never grows memory and the Node regression suite can
    // drive the simulation. The pool is struct-of-arrays (Float32Array per field) with a ring write index: when it is
    // full the oldest slot is overwritten. Rings (the sonar ping) live in a separate list of at most four. No Math.random,
    // no clock: every emitter draws from the seeded generator of its system, and update(dt) is the only thing that
    // moves time.
    //  - createSystem({ capacity = 900, seed = 7 }) -> { emitMaterialize, emitDissolve, emitStream, emitThruster, emitPing,
    //    emitStardust, update, forEach, rings, stats, clear },
    //  - drawCanvas2D(ctx, system, dpr) clears the 2D context and paints the system with additive blending; it only
    //    touches the context it is given (sprite canvases come from the context's own canvas/document) and returns at
    //    once when the context cannot do radial gradients.
    // Coordinates are CSS px, sizes are the px diameter of the bright core. The sprite of a particle spans 3.2x its size.
    // window.AuraOrbitParticles = { createSystem, drawCanvas2D, KINDS }.

    const TWO_PI = Math.PI * 2;
    const KINDS = Object.freeze(['materialize', 'dissolve', 'stream', 'thruster', 'spark', 'stardust']);
    const MATERIALIZE = 0, DISSOLVE = 1, STREAM = 2, THRUSTER = 3, SPARK = 4, STARDUST = 5;
    const MAX_DT = 0.05;
    const FADE_IN = 0.15;
    const FADE_OUT = 0.35;
    const MAX_RINGS = 4;
    const RING_SPEED = 520;
    const RING_LIFE = 1.1;
    const PING_SPARKS = 24;
    const SPREAD = 20 * Math.PI / 180;
    // Core colours of the three hues (0 cyan, 1 violet, 2 amber) and the share of each.
    const HUE_COLORS = Object.freeze([[92, 225, 255], [138, 107, 255], [255, 181, 71]]);
    const HUE_CUT_VIOLET = 0.6;
    const HUE_CUT_AMBER = 0.88;
    const SPRITE_CSS = 32;
    const SPRITE_SCALE = 1.6;

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }
    function finite(value, fallback) { const n = Number(value); return Number.isFinite(n) ? n : fallback; }

    function createRng(seed) {
        let a = finite(seed, 0) >>> 0;
        return function () {
            a = (a + 0x6D2B79F5) >>> 0;
            let t = a;
            t = Math.imul(t ^ (t >>> 15), t | 1);
            t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    // -------------------------------------------------------------------------------------------------
    // The pool

    function createSystem(options) {
        const opts = options || {};
        const capacity = Math.max(1, Math.floor(finite(opts.capacity, 900)));
        const rng = createRng(finite(opts.seed, 7));
        const x = new Float32Array(capacity), y = new Float32Array(capacity);
        const vx = new Float32Array(capacity), vy = new Float32Array(capacity);
        // life counts down to 0 (a slot is alive while it is > 0). A particle that waits for its staggered start has
        // life above maxLife, so age = maxLife - life is negative until it starts.
        const life = new Float32Array(capacity), maxLife = new Float32Array(capacity);
        const size = new Float32Array(capacity), hue = new Float32Array(capacity), kind = new Float32Array(capacity);
        // Start (sx, sy) and target (tx, ty) of the particles that follow a path; a stream keeps its Bezier control
        // point in (vx, vy).
        const tx = new Float32Array(capacity), ty = new Float32Array(capacity);
        const sx = new Float32Array(capacity), sy = new Float32Array(capacity);
        const ringList = [];
        const view = { x: 0, y: 0, size: 0, alpha: 0, hue: 0, kind: '' };
        let head = 0, live = 0, emitted = 0;

        function between(low, high) { return low + (high - low) * rng(); }
        function pickHue() { const r = rng(); return r < HUE_CUT_VIOLET ? 0 : r < HUE_CUT_AMBER ? 1 : 2; }
        function count(value, fallback) { return clamp(Math.floor(finite(value, fallback)), 0, capacity); }
        function seconds(value, fallback) { const n = finite(value, fallback); return n > 0.02 ? n : fallback; }

        // Takes the next slot of the ring; the caller fills the fields and must leave life above 0.
        function spawn(k) {
            const i = head;
            head = (head + 1) % capacity;
            if (!(life[i] > 0)) live++;
            emitted++;
            kind[i] = k;
            hue[i] = pickHue();
            return i;
        }

        // Window opens: the particles gather from outside onto the frame, evenly spread along its perimeter.
        function emitMaterialize(rect, o) {
            if (!rect || !Number.isFinite(rect.x + rect.y + rect.width + rect.height)) return;
            const n = count(o && o.count, 160), duration = seconds(o && o.duration, 0.52);
            const w = Math.max(0, rect.width), h = Math.max(0, rect.height), perimeter = 2 * (w + h);
            for (let k = 0; k < n; k++) {
                const s = k / n * perimeter;
                let px, py, nx, ny;
                if (s < w) { px = rect.x + s; py = rect.y; nx = 0; ny = -1; }
                else if (s < w + h) { px = rect.x + w; py = rect.y + (s - w); nx = 1; ny = 0; }
                else if (s < 2 * w + h) { px = rect.x + w - (s - w - h); py = rect.y + h; nx = 0; ny = 1; }
                else { px = rect.x; py = rect.y + h - (s - 2 * w - h); nx = -1; ny = 0; }
                const out = between(40, 140), slide = between(-30, 30);
                const i = spawn(MATERIALIZE);
                sx[i] = px + nx * out - ny * slide;
                sy[i] = py + ny * out + nx * slide;
                tx[i] = px; ty[i] = py;
                x[i] = sx[i]; y[i] = sy[i];
                vx[i] = 0; vy[i] = 0;
                size[i] = between(1.5, 3.5);
                maxLife[i] = life[i] = duration * between(0.9, 1.1);
            }
        }

        // Window closes: the pixels of the frame come loose from a jittered grid and drift outward, shrinking away.
        function emitDissolve(rect, o) {
            if (!rect || !Number.isFinite(rect.x + rect.y + rect.width + rect.height)) return;
            const n = count(o && o.count, 260), duration = seconds(o && o.duration, 0.95);
            const w = Math.max(0, rect.width), h = Math.max(0, rect.height);
            const cols = Math.max(1, Math.round(Math.sqrt(n * w / Math.max(h, 1)))), rows = Math.max(1, Math.ceil(n / cols));
            const cx = rect.x + w / 2, cy = rect.y + h / 2, shortest = Math.min(0.55, duration);
            for (let k = 0; k < n; k++) {
                const cell = Math.floor(k * cols * rows / n);
                const px = rect.x + ((cell % cols) + rng()) / cols * w;
                const py = rect.y + (Math.floor(cell / cols) + rng()) / rows * h;
                let dx = px - cx, dy = py - cy, length = Math.hypot(dx, dy);
                if (length < 1e-3) { const angle = rng() * TWO_PI; dx = Math.cos(angle); dy = Math.sin(angle); length = 1; }
                const speed = between(60, 220);
                const i = spawn(DISSOLVE);
                x[i] = px; y[i] = py;
                vx[i] = dx / length * speed;
                vy[i] = dy / length * speed - 30;
                size[i] = between(1.5, 4);
                maxLife[i] = life[i] = between(shortest, duration);
            }
        }

        // Window minimizes: the particles leave random points of the frame on a curve that bows upward and gather at
        // `to` (the taskbar slot); the start times are staggered over the first 30 % of the duration.
        function emitStream(rect, to, o) {
            if (!rect || !to || !Number.isFinite(rect.x + rect.y + rect.width + rect.height + to.x + to.y)) return;
            const n = count(o && o.count, 80), duration = seconds(o && o.duration, 0.5);
            for (let k = 0; k < n; k++) {
                const px = rect.x + rng() * Math.max(0, rect.width), py = rect.y + rng() * Math.max(0, rect.height);
                const distance = Math.hypot(to.x - px, to.y - py);
                const i = spawn(STREAM);
                sx[i] = px; sy[i] = py;
                tx[i] = to.x; ty[i] = to.y;
                vx[i] = (px + to.x) / 2;
                vy[i] = (py + to.y) / 2 - distance * 0.25;
                x[i] = px; y[i] = py;
                size[i] = between(1.5, 3.5);
                maxLife[i] = duration;
                life[i] = duration + rng() * 0.3 * duration;
            }
        }

        // Window drag: up to three exhaust particles per call, scaled by intensity (0..1), thrown along `dir`.
        function emitThruster(point, dir, intensity) {
            if (!point || !dir || !Number.isFinite(point.x + point.y + dir.x + dir.y)) return;
            const n = Math.min(3, Math.round(3 * clamp(finite(intensity, 0), 0, 1)));
            const base = Math.hypot(dir.x, dir.y) > 1e-6 ? Math.atan2(dir.y, dir.x) : -Math.PI / 2;
            for (let k = 0; k < n; k++) {
                const angle = base + between(-SPREAD, SPREAD), speed = between(120, 260);
                const i = spawn(THRUSTER);
                x[i] = point.x; y[i] = point.y;
                vx[i] = Math.cos(angle) * speed;
                vy[i] = Math.sin(angle) * speed;
                size[i] = between(1.5, 3);
                maxLife[i] = life[i] = between(0.25, 0.45);
            }
        }

        // Click on the empty desktop: an expanding ring (the oldest of four is replaced) and a burst of sparks.
        function emitPing(point) {
            if (!point || !Number.isFinite(point.x + point.y)) return;
            const ring = ringList.length >= MAX_RINGS ? ringList.shift() : {};
            ring.x = point.x; ring.y = point.y; ring.radius = 0; ring.alpha = 1;
            ring.speed = RING_SPEED; ring.life = RING_LIFE; ring.maxLife = RING_LIFE;
            ringList.push(ring);
            for (let k = 0; k < PING_SPARKS; k++) {
                const angle = (k + rng()) / PING_SPARKS * TWO_PI, speed = between(160, 320);
                const i = spawn(SPARK);
                x[i] = point.x; y[i] = point.y;
                vx[i] = Math.cos(angle) * speed;
                vy[i] = Math.sin(angle) * speed;
                size[i] = between(1.5, 3);
                maxLife[i] = life[i] = between(0.4, 0.7);
            }
        }

        // Pointer trail on the empty desktop: one slow mote that keeps a little of the pointer's velocity.
        function emitStardust(point, velocity) {
            if (!point || !Number.isFinite(point.x + point.y)) return;
            const i = spawn(STARDUST);
            x[i] = point.x; y[i] = point.y;
            vx[i] = finite(velocity && velocity.x, 0) * 0.05;
            vy[i] = finite(velocity && velocity.y, 0) * 0.05;
            size[i] = between(1, 2);
            maxLife[i] = life[i] = between(0.8, 1.4);
        }

        // Advances time by dt (clamped to 0.05 s) and frees what expired.
        function update(dt) {
            const h = clamp(finite(dt, 0), 0, MAX_DT);
            if (h <= 0) return;
            for (let i = 0; i < capacity; i++) {
                if (!(life[i] > 0)) continue;
                const remaining = life[i] - h;
                if (remaining <= 0) { life[i] = 0; live--; continue; }
                life[i] = remaining;
                const age = maxLife[i] - remaining;
                if (age < 0) continue;
                const k = kind[i];
                if (k === MATERIALIZE) {
                    const rest = 1 - age / maxLife[i], gain = 1 - rest * rest * rest;
                    x[i] = sx[i] + (tx[i] - sx[i]) * gain;
                    y[i] = sy[i] + (ty[i] - sy[i]) * gain;
                } else if (k === STREAM) {
                    const u = age / maxLife[i], t = u * u * (3 - 2 * u), a = (1 - t) * (1 - t), b = 2 * (1 - t) * t, c = t * t;
                    x[i] = a * sx[i] + b * vx[i] + c * tx[i];
                    y[i] = a * sy[i] + b * vy[i] + c * ty[i];
                } else {
                    x[i] += vx[i] * h;
                    y[i] += vy[i] * h;
                }
            }
            let kept = 0;
            for (let r = 0; r < ringList.length; r++) {
                const ring = ringList[r];
                ring.life -= h;
                if (ring.life <= 0) continue;
                const rest = ring.life / ring.maxLife;
                ring.radius += ring.speed * h;
                ring.alpha = rest * rest;
                ringList[kept++] = ring;
            }
            ringList.length = kept;
        }

        // Calls fn(p) for every visible particle with the one reused object { x, y, size, alpha, hue, kind } (copy what
        // you keep). Alpha follows the life envelope (15 % fade in, 35 % fade out); a dissolving particle shrinks to 0.
        function forEach(fn) {
            for (let i = 0; i < capacity; i++) {
                const remaining = life[i];
                if (!(remaining > 0)) continue;
                const age = maxLife[i] - remaining;
                if (age < 0) continue;
                const u = age / maxLife[i];
                let alpha = 1;
                if (u < FADE_IN) alpha = u / FADE_IN;
                if (u > 1 - FADE_OUT) alpha = Math.min(alpha, (1 - u) / FADE_OUT);
                view.x = x[i]; view.y = y[i];
                view.size = kind[i] === DISSOLVE ? size[i] * (1 - u) : size[i];
                view.alpha = clamp(alpha, 0, 1);
                view.hue = hue[i];
                view.kind = KINDS[kind[i]];
                fn(view);
            }
        }

        // The live rings, oldest first: { x, y, radius, alpha, speed, life, maxLife }. The array is the system's own.
        function rings() { return ringList; }

        // `emitted` counts every particle since creation, `live` the ones in the pool (waiting streamers included).
        function stats() { return { live, emitted, capacity, rings: ringList.length }; }

        function clear() {
            life.fill(0);
            live = 0;
            head = 0;
            ringList.length = 0;
        }

        return Object.freeze({ emitMaterialize, emitDissolve, emitStream, emitThruster, emitPing, emitStardust,
            update, forEach, rings, stats, clear });
    }

    // -------------------------------------------------------------------------------------------------
    // Canvas 2D additive renderer

    // Per device pixel ratio: one canvas per hue with a radial glow (bright core, soft falloff), created once.
    const spriteCache = new Map();
    const paint = { ctx: null, sprites: null };

    function makeSpriteCanvas(ctx, pixels) {
        let canvas = null;
        if (typeof OffscreenCanvas === 'function') canvas = new OffscreenCanvas(pixels, pixels);
        else if (ctx.canvas && ctx.canvas.ownerDocument) canvas = ctx.canvas.ownerDocument.createElement('canvas');
        if (!canvas) return null;
        canvas.width = pixels;
        canvas.height = pixels;
        return canvas;
    }

    function buildSprites(ctx, ratio) {
        const pixels = Math.max(16, Math.round(SPRITE_CSS * ratio)), half = pixels / 2;
        const sprites = [];
        for (let h = 0; h < HUE_COLORS.length; h++) {
            const canvas = makeSpriteCanvas(ctx, pixels);
            const g = canvas && canvas.getContext && canvas.getContext('2d');
            if (!g) return null;
            const [r, gr, b] = HUE_COLORS[h], core = 'rgba(' + [r, gr, b].map(v => Math.round(v + (255 - v) * 0.6)).join(',') + ',1)';
            const tint = 'rgba(' + r + ',' + gr + ',' + b + ',';
            const glow = g.createRadialGradient(half, half, 0, half, half, half);
            glow.addColorStop(0, core);
            glow.addColorStop(0.2, tint + '0.85)');
            glow.addColorStop(0.55, tint + '0.22)');
            glow.addColorStop(1, tint + '0)');
            g.fillStyle = glow;
            g.fillRect(0, 0, pixels, pixels);
            sprites.push(canvas);
        }
        return sprites;
    }

    function spritesFor(ctx, ratio) {
        const key = Math.round(ratio * 100);
        if (spriteCache.has(key)) return spriteCache.get(key);
        // A context that cannot make sprite canvases is remembered as null: the rings still draw, particles are skipped.
        const sprites = buildSprites(ctx, ratio);
        if (spriteCache.size >= 4) spriteCache.delete(spriteCache.keys().next().value);
        spriteCache.set(key, sprites);
        return sprites;
    }

    // forEach callback: one sprite per particle, no allocation.
    function drawParticle(p) {
        const r = p.size * SPRITE_SCALE;
        if (p.alpha <= 0.004 || r <= 0.05) return;
        paint.ctx.globalAlpha = p.alpha;
        paint.ctx.drawImage(paint.sprites[p.hue | 0], p.x - r, p.y - r, r * 2, r * 2);
    }

    function drawRing(ctx, ring) {
        if (ring.radius < 0.5 || ring.alpha <= 0.004) return;
        ctx.beginPath();
        ctx.arc(ring.x, ring.y, ring.radius, 0, TWO_PI);
        ctx.globalAlpha = ring.alpha * 0.28;
        ctx.lineWidth = 6;
        ctx.stroke();
        ctx.globalAlpha = ring.alpha;
        ctx.lineWidth = 1.5;
        ctx.stroke();
    }

    // Clears the context and paints the system in CSS px (the context is scaled by dpr). Does nothing on a context
    // without radial gradients, which is how the Node suite calls it.
    function drawCanvas2D(ctx, system, dpr) {
        if (!ctx || !system || typeof ctx.createRadialGradient !== 'function') return;
        const ratio = clamp(finite(dpr, 1), 0.5, 4);
        const canvas = ctx.canvas;
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.globalAlpha = 1;
        ctx.globalCompositeOperation = 'source-over';
        ctx.clearRect(0, 0, canvas ? canvas.width : 16384, canvas ? canvas.height : 16384);
        ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
        ctx.globalCompositeOperation = 'lighter';
        const sprites = spritesFor(ctx, ratio);
        if (sprites) {
            paint.ctx = ctx;
            paint.sprites = sprites;
            system.forEach(drawParticle);
            paint.ctx = null;
            paint.sprites = null;
        }
        const rings = system.rings();
        if (rings.length) {
            ctx.strokeStyle = 'rgb(92, 225, 255)';
            for (let i = 0; i < rings.length; i++) drawRing(ctx, rings[i]);
        }
        ctx.globalAlpha = 1;
        ctx.globalCompositeOperation = 'source-over';
        ctx.setTransform(1, 0, 0, 1, 0, 0);
    }

    window.AuraOrbitParticles = Object.freeze({ createSystem, drawCanvas2D, KINDS });
}());
