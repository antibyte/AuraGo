/* Aura Drift - shared core: deterministic math, data tables, input and state. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore = window.AuraDriftCore || {};
    AD.VERSION = '1.0.0';
    AD.STEP = 1 / 120;
    AD.MAX_SUBSTEPS = 5;
    AD.CHUNK_METRES = 256;

    AD.clamp = (v, lo, hi) => v < lo ? lo : (v > hi ? hi : v);
    AD.lerp = (a, b, t) => a + (b - a) * t;
    AD.smooth = (a, b, rate, dt) => a + (b - a) * (1 - Math.exp(-rate * dt));
    AD.wrapAngle = a => Math.atan2(Math.sin(a), Math.cos(a));
    AD.distance = (ax, ay, bx, by) => Math.hypot(ax - bx, ay - by);
    AD.round2 = v => Math.round(v * 100) / 100;

    /* mulberry32: small, fast, reproducible. Never used for particles. */
    AD.rng = function (seed) {
        let s = (seed >>> 0) || 1;
        return function () {
            s = (s + 0x6D2B79F5) >>> 0;
            let t = s;
            t = Math.imul(t ^ (t >>> 15), t | 1);
            t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    };

    AD.hashString = function (text) {
        let h = 2166136261 >>> 0;
        for (let i = 0; i < text.length; i++) {
            h ^= text.charCodeAt(i);
            h = Math.imul(h, 16777619) >>> 0;
        }
        return h >>> 0;
    };

    /* Presentation text for surfaces lives in locale keys (aura_drift.surface.<type>). */
    AD.SURFACES = {
        asphalt: { grip: 1.00, roll: 0.016 },
        salt: { grip: 1.14, roll: 0.012 },
        wet: { grip: 0.72, roll: 0.020 },
        gravel: { grip: 0.62, roll: 0.034 },
        mud: { grip: 0.50, roll: 0.048 },
        ice: { grip: 0.34, roll: 0.010 },
        oil: { grip: 0.20, roll: 0.010 },
        sand: { grip: 0.55, roll: 0.055 },
        snow: { grip: 0.42, roll: 0.030 }
    };

    /* Event bus of the simulation. Presentation drains race.events; the ghost
       steps with race.muteEvents so it never makes noise. */
    AD.emit = function (race, type, data) {
        if (race.muteEvents) return;
        race.events.push(Object.assign({ type: type, t: race.time }, data || {}));
    };

    /* "{name}" placeholders. Locale strings never receive arguments in `t`. */
    AD.fmt = function (text, values) {
        return String(text === undefined || text === null ? '' : text).replace(/\{(\w+)\}/g, (m, key) =>
            values && Object.prototype.hasOwnProperty.call(values, key) ? String(values[key]) : m);
    };

    /* Order is fixed and `id` is the stable key. `shape` picks the 2D silhouette
       (unknown shapes fall back to the GT outline). `quirk` names a rule that the
       owning module reads; `quirkK` is its strength. */
    function car(id, name, shape, power, mass, grip, dragK, steerK, airControl, looseRoll, radius, scale, colour, accent, liveries, quirk, quirkK, extra) {
        return Object.assign({
            id: id, name: name, shape: shape, power: power, mass: mass, grip: grip, dragK: dragK, steerK: steerK,
            airControl: airControl, looseRoll: looseRoll, radius: radius, visualScale: scale,
            colour: colour, accent: accent, liveries: liveries, engine: id,
            quirk: quirk, quirkK: quirkK, driftNitro: 1
        }, extra || {});
    }
    AD.CARS = [
        car('vektor', 'Vektor', 'wedge', 1.00, 1.00, 1.00, 1.00, 1.00, 1.00, 1.00, 2.05, 1.25, '#ff5a4e', '#ffd7c2', ['#ff5a4e', '#ffd166', '#38bdf8', '#f5f5f5'], 'slipstream', 1.25),
        car('kobalt', 'Kobalt', 'gt', 0.96, 1.02, 1.06, 0.98, 1.02, 1.00, 1.00, 2.05, 1.25, '#4f8cff', '#cfe2ff', ['#4f8cff', '#a78bfa', '#34d399', '#1e293b'], 'wet_grip', 1.06),
        car('lumen', 'Lumen', 'muscle', 1.08, 1.12, 0.95, 1.02, 0.96, 0.95, 1.00, 2.05, 1.25, '#f6c445', '#fff7e6', ['#f6c445', '#fb7185', '#2dd4bf', '#111827'], 'turbo_time', 1.2),
        car('tundra', 'Tundra', 'rally', 0.92, 0.92, 1.04, 1.05, 1.05, 1.05, 0.75, 2.05, 1.25, '#4ade80', '#ffe066', ['#4ade80', '#f97316', '#e0f2fe', '#7c3aed'], 'loose_grip', 1.18),
        car('softeis', 'Softeis', 'van', 1.10, 1.35, 0.92, 1.18, 0.90, 0.85, 1.00, 2.20, 1.2, '#ffb3d1', '#f3e3c3', ['#ffb3d1', '#fde68a', '#a7f3d0', '#fef3c7'], 'pad_time', 1.4),
        car('maehkoenig', 'Mähkönig', 'mower', 0.72, 0.50, 1.12, 1.30, 1.15, 1.15, 0.45, 1.80, 1.3, '#7ac74f', '#f3e3c3', ['#7ac74f', '#f59e0b', '#ef4444', '#3b82f6'], 'sturdy', 1),
        car('couch', 'Couch-Potato', 'sofa', 0.90, 0.72, 0.88, 1.10, 1.12, 1.35, 1.00, 1.90, 1.25, '#b5764a', '#2a2a2a', ['#b5764a', '#6d28d9', '#0f766e', '#be185d'], 'trick_points', 1.25, { driftNitro: 1.3 }),
        car('bertha', 'Big Bertha', 'truck', 1.25, 1.60, 0.98, 1.20, 0.86, 0.90, 0.60, 2.40, 1.15, '#8a5cf6', '#2a2a2a', ['#8a5cf6', '#f97316', '#10b981', '#94a3b8'], 'heavy', 0.6)
    ];
    AD.carIndex = function (id) {
        const i = AD.CARS.findIndex(c => c.id === id);
        return i < 0 ? 0 : i;
    };
    AD.carById = id => AD.CARS[AD.carIndex(id)];

    /* Opponent personalities. Names are proper nouns and are not translated. */
    function driver(id, name, carId, skill, aggression, drift, helmet, visor, hair, moustache) {
        return {
            id: id, name: name, carId: carId, skill: skill, aggression: aggression, drift: drift,
            portrait: { helmet: helmet, visor: visor, hair: hair, moustache: moustache }
        };
    }
    AD.DRIVERS = [
        driver('viktor', 'Viktor Vollgas', 'vektor', 0.97, 1.05, 0.5, '#ff5a4e', '#1f2937', '#3b2a20', true),
        driver('rosi', 'Rosi Rückspiegel', 'kobalt', 0.99, 0.90, 0.3, '#4f8cff', '#0f172a', '#c2410c', false),
        driver('bruno', 'Bruno Bleifuß', 'lumen', 0.92, 1.12, 0.6, '#f6c445', '#1f2937', '#111827', true),
        driver('heidi', 'Heidi Handbremse', 'tundra', 0.94, 1.00, 1.0, '#4ade80', '#052e16', '#fde68a', false),
        driver('ilse', 'Ilse Eistüte', 'softeis', 0.90, 0.95, 0.4, '#ffb3d1', '#4a044e', '#e5e7eb', false),
        driver('gustav', 'Gustav Gartenzwerg', 'maehkoenig', 0.92, 0.88, 0.4, '#7ac74f', '#1c1917', '#f5f5f4', true),
        driver('kurt', 'Kurt Kanapee', 'couch', 0.88, 0.82, 0.7, '#b5764a', '#292524', '#78350f', true),
        driver('dolores', 'Dolores Dampfwalze', 'bertha', 0.91, 1.15, 0.2, '#8a5cf6', '#1e1b4b', '#9a3412', false)
    ];
    AD.driverField = function (playerCarId, count, rng) {
        const pool = AD.DRIVERS.filter(d => d.carId !== playerCarId);
        for (let i = pool.length - 1; i > 0; i--) {
            const j = Math.floor(rng() * (i + 1));
            const swap = pool[i]; pool[i] = pool[j]; pool[j] = swap;
        }
        const picked = [];
        for (let i = 0; i < count; i++) picked.push(pool[i % pool.length]);
        return picked;
    };

    /* Style scoring: reward sideways momentum, chain it into a multiplier and
       pay out for clean exits, close passes and air time. */
    AD.STYLE = {
        driftAngleMin: 0.17,
        driftSpeedMin: 9,
        driftRate: 3.5,
        comboWindow: 1.5,
        multiplierStep: 0.5,
        multiplierMax: 8,
        nearMissDistance: 2.7,
        nearMissWindow: 0.85,
        bankRate: 0.75
    };

    /* Old 2D fields stay; `gl` is the budget table of the three.js renderer. */
    AD.QUALITY = {
        low: {
            particles: 70, weather: 0.25, lighting: false, bloom: false, chunkCache: 24, detail: 0.35,
            gl: {
                dpr: 1, shadows: 0, shadowBox: 0, post: false, bloom: 'off', aa: 'none', msaa: 0, gtao: false,
                particles: 300, skid: 800, props: 300, propRadius: 220, terrainCell: 16, lights: 0,
                clearcoat: false, envSize: 64, weather: 0.4, calls: 120, triangles: 250000
            }
        },
        medium: {
            particles: 180, weather: 0.6, lighting: true, bloom: false, chunkCache: 40, detail: 0.7,
            gl: {
                dpr: 1.25, shadows: 1024, shadowBox: 90, post: true, bloom: 'half', aa: 'fxaa', msaa: 0, gtao: false,
                particles: 900, skid: 2000, props: 700, propRadius: 320, terrainCell: 8, lights: 2,
                clearcoat: false, envSize: 128, weather: 0.7, calls: 180, triangles: 500000
            }
        },
        high: {
            particles: 380, weather: 1, lighting: true, bloom: true, chunkCache: 64, detail: 1,
            gl: {
                dpr: 1.5, shadows: 2048, shadowBox: 140, post: true, bloom: 'full', aa: 'smaa', msaa: 0, gtao: false,
                particles: 2000, skid: 3000, props: 1400, propRadius: 480, terrainCell: 8, lights: 4,
                clearcoat: true, envSize: 256, weather: 1, calls: 260, triangles: 900000
            }
        },
        ultra: {
            particles: 620, weather: 1.4, lighting: true, bloom: true, chunkCache: 96, detail: 1.35,
            gl: {
                dpr: 2, shadows: 2048, shadowBox: 180, post: true, bloom: 'full', aa: 'smaa', msaa: 4, gtao: true,
                particles: 3500, skid: 4000, props: 2200, propRadius: 700, terrainCell: 8, lights: 6,
                clearcoat: true, envSize: 256, weather: 1.3, calls: 350, triangles: 1500000
            }
        }
    };

    /* Frame-time governor (fixes D2: dt is added exactly once per sample).
       2 s windows; down after 2 slow windows, up after 3 fast ones, 12 s
       cooldown, failed tiers blocked for 60 s doubling. Time is the sum of the
       samples, so the governor needs no clock. */
    const TIER_ORDER = ['low', 'medium', 'high', 'ultra'];
    AD.createQualityGovernor = function (startTier) {
        const gov = { tier: TIER_ORDER.includes(startTier) ? startTier : 'medium' };
        let frames = [], windowTime = 0, slow = 0, fast = 0, cooldown = 0, locked = false, clock = 0;
        const blocked = {}, penalty = {};

        function change(next) {
            gov.tier = next;
            cooldown = 12; slow = 0; fast = 0;
            return next;
        }
        gov.sample = function (dt) {
            clock += dt;
            if (locked) return null;
            if (cooldown > 0) cooldown = Math.max(0, cooldown - dt);
            frames.push(dt); windowTime += dt;
            if (windowTime < 2) return null;
            const avg = windowTime / frames.length;
            const sorted = frames.slice().sort((a, b) => a - b);
            const p95 = sorted[Math.min(sorted.length - 1, Math.floor(sorted.length * 0.95))];
            frames = []; windowTime = 0;
            if (cooldown > 0) { slow = 0; fast = 0; return null; }
            const index = TIER_ORDER.indexOf(gov.tier);
            if (avg > 0.022) { slow++; fast = 0; } else if (avg < 0.0176 && p95 < 0.02) { fast++; slow = 0; } else { slow = 0; fast = 0; }
            if (slow >= 2 && index > 0) return change(TIER_ORDER[index - 1]);
            if (fast >= 3 && index < TIER_ORDER.length - 1) {
                const up = TIER_ORDER[index + 1];
                if ((blocked[up] || 0) > clock) { fast = 0; return null; }
                return change(up);
            }
            return null;
        };
        /* A tier that could not hold the budget is blocked for 60 s, doubling. */
        gov.fail = function (tier) {
            penalty[tier] = Math.min(960, (penalty[tier] || 30) * 2);
            blocked[tier] = clock + penalty[tier];
        };
        gov.lock = function (tier) {
            if (TIER_ORDER.includes(tier)) gov.tier = tier;
            locked = true;
        };
        gov.reset = function () {
            frames = []; windowTime = 0; slow = 0; fast = 0; cooldown = 0; locked = false;
        };
        return gov;
    };

    AD.defaultSettings = function () {
        return {
            quality: 'auto', volume: 0.65, muted: false,
            reducedMotion: false, weapons: true, showGhost: true,
            car: 'vektor', livery: 0, camera: 'chase'
        };
    };

    AD.SETTINGS_KEY = 'aurago.desktop.aura-drift.v1';
    AD.loadSettings = function () {
        const base = AD.defaultSettings();
        try {
            const raw = window.localStorage && window.localStorage.getItem(AD.SETTINGS_KEY);
            if (!raw) return base;
            const saved = JSON.parse(raw);
            for (const key of Object.keys(base)) {
                if (Object.prototype.hasOwnProperty.call(saved, key) &&
                    typeof saved[key] === typeof base[key]) base[key] = saved[key];
            }
        } catch (_) { /* private mode or corrupt value: use defaults */ }
        return base;
    };
    AD.saveSettings = function (settings, readonly) {
        if (readonly) return;
        try {
            if (window.localStorage) window.localStorage.setItem(AD.SETTINGS_KEY, JSON.stringify(settings));
        } catch (_) { /* storage unavailable */ }
    };

    /* Run-length pairs in base 36, comma separated: value,count,value,count...
       A raw JSON integer tape would fill localStorage within a few runs. */
    AD.encodeTape = function (ints) {
        const parts = [];
        let i = 0;
        while (i < ints.length) {
            let j = i + 1;
            while (j < ints.length && ints[j] === ints[i]) j++;
            parts.push(ints[i].toString(36), (j - i).toString(36));
            i = j;
        }
        return parts.join(',');
    };
    AD.decodeTape = function (text) {
        if (Array.isArray(text)) return text;
        if (typeof text !== 'string' || !text) return [];
        const parts = text.split(',');
        const out = [];
        for (let i = 0; i + 1 < parts.length; i += 2) {
            const value = parseInt(parts[i], 36), count = parseInt(parts[i + 1], 36);
            if (!isFinite(value) || !(count > 0)) return [];
            for (let k = 0; k < count; k++) out.push(value);
        }
        return out;
    };

    /* v2: tapes now include the countdown steps, so v1 ghosts cannot replay.
       The old key is removed (no migration) unless the desktop is readonly. */
    AD.BEST_KEY = 'aurago.desktop.aura-drift.best.v2';
    AD.OLD_BEST_KEY = 'aurago.desktop.aura-drift.best.v1';
    AD.loadBests = function (readonly) {
        try {
            if (!readonly && window.localStorage) window.localStorage.removeItem(AD.OLD_BEST_KEY);
            const raw = window.localStorage && window.localStorage.getItem(AD.BEST_KEY);
            const parsed = raw ? JSON.parse(raw) : {};
            return parsed && typeof parsed === 'object' ? parsed : {};
        } catch (_) { return {}; }
    };
    /* Entry shape: { time, tape: string, carId, score? }. Array tapes handed in
       by callers are encoded on the way out. */
    AD.saveBests = function (bests, readonly) {
        if (readonly) return;
        try {
            const out = {};
            for (const key of Object.keys(bests)) {
                const entry = bests[key];
                out[key] = entry && Array.isArray(entry.tape)
                    ? Object.assign({}, entry, { tape: AD.encodeTape(entry.tape) }) : entry;
            }
            if (window.localStorage) window.localStorage.setItem(AD.BEST_KEY, JSON.stringify(out));
        } catch (_) { /* storage unavailable */ }
    };

    AD.createInput = function () {
        return { throttle: 0, brake: 0, steer: 0, handbrake: 0, fire: 0, boost: 0 };
    };
    AD.copyInput = function (input) {
        return {
            throttle: input.throttle, brake: input.brake, steer: input.steer,
            handbrake: input.handbrake, fire: input.fire, boost: input.boost
        };
    };
    /* Input tapes are compact integers: one byte per axis, one bit per button. */
    AD.packInput = function (i) {
        return (Math.round(AD.clamp(i.throttle, 0, 1) * 255)) |
            (Math.round(AD.clamp(i.brake, 0, 1) * 255) << 8) |
            ((Math.round(AD.clamp(i.steer, -1, 1) * 127) & 255) << 16) |
            ((i.handbrake ? 1 : 0) << 24) | ((i.fire ? 1 : 0) << 25) | ((i.boost ? 1 : 0) << 26);
    };
    AD.unpackInput = function (packed) {
        const steer = (packed >>> 16) & 255;
        return {
            throttle: (packed & 255) / 255,
            brake: ((packed >>> 8) & 255) / 255,
            steer: (steer > 127 ? steer - 256 : steer) / 127,
            handbrake: (packed >>> 24) & 1 ? 1 : 0,
            fire: (packed >>> 25) & 1 ? 1 : 0,
            boost: (packed >>> 26) & 1 ? 1 : 0
        };
    };

    AD.createScheduler = function () {
        let accumulator = 0;
        return {
            reset() { accumulator = 0; },
            /* Runs the simulation in fixed steps; returns the number of steps taken. */
            advance(dt, step) {
                accumulator += Math.min(dt, 0.25);
                let count = 0;
                /* A small tolerance prevents floating-point drift from dropping a
                   step at 144 Hz, which would desynchronise replays and ghosts. */
                while (accumulator >= AD.STEP - 1e-9 && count < AD.MAX_SUBSTEPS) {
                    step(AD.STEP);
                    accumulator -= AD.STEP;
                    count++;
                }
                if (accumulator < 0) accumulator = 0;
                if (count === AD.MAX_SUBSTEPS) accumulator = 0;
                return count;
            },
            get pending() { return accumulator; }
        };
    };

    /* State hash for replay determinism checks. Uses only simulation state. */
    AD.hashValues = function (values) {
        let h = 2166136261 >>> 0;
        for (let i = 0; i < values.length; i++) {
            const scaled = Math.round(values[i] * 10000) | 0;
            h ^= scaled & 255; h = Math.imul(h, 16777619) >>> 0;
            h ^= (scaled >>> 8) & 255; h = Math.imul(h, 16777619) >>> 0;
            h ^= (scaled >>> 16) & 255; h = Math.imul(h, 16777619) >>> 0;
            h ^= (scaled >>> 24) & 255; h = Math.imul(h, 16777619) >>> 0;
        }
        return h >>> 0;
    };

    AD.formatTime = function (seconds) {
        if (!isFinite(seconds) || seconds < 0) return '--:--.---';
        const m = Math.floor(seconds / 60);
        const s = Math.floor(seconds % 60);
        const ms = Math.floor((seconds - Math.floor(seconds)) * 1000);
        return String(m).padStart(2, '0') + ':' + String(s).padStart(2, '0') + '.' + String(ms).padStart(3, '0');
    };
})();
