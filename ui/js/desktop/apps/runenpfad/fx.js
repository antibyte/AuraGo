/* Runenpfad particle and screen-effect simulation. DOM-free, deterministic. */
(function (root) {
    'use strict';

    const MAX_PARTICLES = 460;

    // Deterministic RNG: identical spawn sequences across runs and tests.
    function makeRng(seed) {
        let s = (Number(seed) || 1) >>> 0;
        if (s === 0) s = 0x9e3779b9;
        return function () {
            s = (Math.imul(1664525, s) + 1013904223) >>> 0;
            return s / 4294967296;
        };
    }

    // Per-kind defaults: size, lifetime, gravity, drag, shape and glow.
    const KINDS = {
        dust: { size: 4.5, life: 0.45, gravity: 70, drag: 3.4, shape: 'soft', glow: 0 },
        spark: { size: 3.2, life: 0.32, gravity: 360, drag: 0.9, shape: 'streak', glow: 1 },
        shard: { size: 5, life: 0.78, gravity: 640, drag: 0.4, shape: 'square', glow: 0 },
        ember: { size: 3, life: 1.15, gravity: -46, drag: 1.5, shape: 'soft', glow: 1 },
        smoke: { size: 13, life: 0.72, gravity: -78, drag: 2.1, shape: 'soft', glow: 0 },
        ring: { size: 12, life: 0.38, gravity: 0, drag: 0, shape: 'ring', glow: 1 },
        trail: { size: 5.5, life: 0.28, gravity: 0, drag: 4.2, shape: 'soft', glow: 1 },
        rune: { size: 6, life: 0.95, gravity: -24, drag: 1.2, shape: 'rune', glow: 1 },
        bubble: { size: 4, life: 0.9, gravity: -140, drag: 1.0, shape: 'soft', glow: 0 }
    };

    function createSystem(seed) {
        return {
            particles: [],
            rng: makeRng(seed || 20240516),
            shake: 0,
            flashColor: '',
            flashAlpha: 0,
            flashTimer: 0,
            flashMax: 0,
            hitstop: 0,
            time: 0
        };
    }

    function clamp(v, lo, hi) {
        return v < lo ? lo : v > hi ? hi : v;
    }

    // Spawns a single particle. `opts` may override any default.
    function spawn(sys, kind, x, y, opts) {
        if (!sys) return null;
        const def = KINDS[kind] || KINDS.dust;
        const o = opts || {};
        const rng = sys.rng;
        const angle = o.angle == null ? rng() * Math.PI * 2 : o.angle;
        const arc = o.arc == null ? Math.PI * 2 : o.arc;
        const a = angle + (rng() - 0.5) * arc;
        const speed = (o.speed == null ? 120 : o.speed) * (0.55 + rng() * 0.8);
        const life = (o.life == null ? def.life : o.life) * (0.75 + rng() * 0.5);
        const p = {
            kind: kind,
            x: x + (o.offsetX || 0),
            y: y + (o.offsetY || 0),
            vx: Math.cos(a) * speed + (o.vx || 0),
            vy: Math.sin(a) * speed + (o.vy || 0),
            life: life,
            maxLife: life,
            size: (o.size == null ? def.size : o.size) * (0.7 + rng() * 0.6),
            color: o.color || '#ffffff',
            gravity: o.gravity == null ? def.gravity : o.gravity,
            drag: o.drag == null ? def.drag : o.drag,
            shape: o.shape || def.shape,
            glow: o.glow == null ? def.glow : o.glow,
            spin: o.spin == null ? (rng() - 0.5) * 9 : o.spin,
            rot: rng() * Math.PI * 2,
            fade: o.fade == null ? 1 : o.fade,
            grow: o.grow == null ? 0 : o.grow,
            additive: o.additive == null ? def.glow === 1 : o.additive
        };
        if (sys.particles.length >= MAX_PARTICLES) sys.particles.shift();
        sys.particles.push(p);
        return p;
    }

    // Spawns `count` particles in a cone/arc around (x, y).
    function burst(sys, kind, x, y, count, opts) {
        const o = opts || {};
        const n = Math.max(1, Math.floor(count || 1));
        for (let i = 0; i < n; i++) {
            spawn(sys, kind, x, y, o);
        }
    }

    // Radial ring used for impacts, ground slams and boss phase changes.
    function ring(sys, x, y, opts) {
        const o = opts || {};
        return spawn(sys, 'ring', x, y, {
            angle: 0,
            arc: 0,
            speed: 0,
            size: o.size == null ? 12 : o.size,
            grow: o.grow == null ? 210 : o.grow,
            life: o.life == null ? 0.38 : o.life,
            color: o.color || '#7ee8fa',
            vx: o.vx || 0,
            vy: o.vy || 0,
            additive: o.additive !== false
        });
    }

    function flash(sys, color, alpha, duration) {
        if (!sys) return;
        sys.flashColor = color || '#ffffff';
        sys.flashAlpha = clamp(alpha == null ? 0.35 : alpha, 0, 1);
        sys.flashTimer = duration == null ? 0.18 : duration;
        sys.flashMax = sys.flashTimer;
    }

    function shake(sys, amount, duration) {
        if (!sys) return;
        const next = clamp(amount == null ? 4 : amount, 0, 14);
        sys.shake = Math.max(sys.shake, next);
        if (duration) sys.shake = Math.max(sys.shake, next * clamp(duration / 0.3, 0.2, 1));
    }

    // Brief simulation freeze that reads as impact weight (interactables still resolve).
    function hitstop(sys, seconds) {
        if (!sys) return;
        sys.hitstop = Math.max(sys.hitstop, clamp(seconds == null ? 0.055 : seconds, 0, 0.2));
    }

    function clear(sys) {
        if (!sys) return;
        sys.particles.length = 0;
        sys.shake = 0;
        sys.hitstop = 0;
        sys.flashTimer = 0;
        sys.flashAlpha = 0;
    }

    function update(sys, dt) {
        if (!sys) return;
        const step = Math.min(0.05, Math.max(0, Number(dt) || 0));
        sys.time += step;
        if (sys.shake > 0) {
            sys.shake = Math.max(0, sys.shake - step * 26);
        }
        if (sys.flashTimer > 0) {
            sys.flashTimer = Math.max(0, sys.flashTimer - step);
            if (sys.flashTimer === 0) sys.flashAlpha = 0;
        }
        if (sys.hitstop > 0) {
            sys.hitstop = Math.max(0, sys.hitstop - step);
        }
        const list = sys.particles;
        for (let i = list.length - 1; i >= 0; i--) {
            const p = list[i];
            p.life -= step;
            if (p.life <= 0) {
                list.splice(i, 1);
                continue;
            }
            const drag = Math.max(0, 1 - p.drag * step);
            p.vx *= drag;
            p.vy = p.vy * drag + p.gravity * step;
            p.x += p.vx * step;
            p.y += p.vy * step;
            p.rot += p.spin * step;
            if (p.grow) p.size += p.grow * step;
        }
    }

    root.RunenpfadFx = {
        MAX_PARTICLES: MAX_PARTICLES,
        KINDS: KINDS,
        createSystem: createSystem,
        spawn: spawn,
        burst: burst,
        ring: ring,
        flash: flash,
        shake: shake,
        hitstop: hitstop,
        clear: clear,
        update: update
    };
})(typeof window !== 'undefined' ? window : globalThis);
