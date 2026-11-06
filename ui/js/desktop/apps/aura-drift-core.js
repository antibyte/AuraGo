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

    AD.SURFACES = {
        asphalt: { grip: 1.00, roll: 0.016, label: 'Asphalt' },
        salt: { grip: 1.14, roll: 0.012, label: 'Salz' },
        wet: { grip: 0.72, roll: 0.020, label: 'Nass' },
        gravel: { grip: 0.62, roll: 0.034, label: 'Schotter' },
        mud: { grip: 0.50, roll: 0.048, label: 'Matsch' },
        ice: { grip: 0.34, roll: 0.010, label: 'Eis' },
        oil: { grip: 0.20, roll: 0.010, label: 'Öl' },
        sand: { grip: 0.55, roll: 0.055, label: 'Sand' },
        snow: { grip: 0.42, roll: 0.030, label: 'Schnee' }
    };

    AD.WEAPONS = {
        oil: { id: 'oil', name: 'Ölspur', ammo: 1, cooldown: 0.9, colour: '#1b1b22' },
        mine: { id: 'mine', name: 'Mine', ammo: 1, cooldown: 1.1, colour: '#f97316' },
        emp: { id: 'emp', name: 'EMP', ammo: 1, cooldown: 1.4, colour: '#38bdf8' },
        rocket: { id: 'rocket', name: 'Rakete', ammo: 1, cooldown: 1.6, colour: '#fb7185' },
        mg: { id: 'mg', name: 'MG', ammo: 6, cooldown: 0.16, colour: '#facc15' },
        shield: { id: 'shield', name: 'Schild', ammo: 1, cooldown: 1.2, colour: '#5eead4' }
    };
    AD.WEAPON_IDS = Object.keys(AD.WEAPONS);

    /* shape picks the pre-rendered silhouette: the prototype, the GT coupe,
       the muscle car and the rally hatch all read differently at a glance. */
    AD.CARS = [
        { name: 'Vektor', colour: '#ff5a4e', accent: '#ffd7c2', grip: 1.00, power: 1.00, shape: 'wedge' },
        { name: 'Kobalt', colour: '#4f8cff', accent: '#cfe2ff', grip: 1.04, power: 0.96, shape: 'gt' },
        { name: 'Lumen', colour: '#f6c445', accent: '#fff0bd', grip: 0.97, power: 1.05, shape: 'muscle' },
        { name: 'Tundra', colour: '#4ade80', accent: '#d6ffe4', grip: 1.08, power: 0.92, shape: 'rally' }
    ];

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

    AD.QUALITY = {
        low: { particles: 70, weather: 0.25, lighting: false, bloom: false, chunkCache: 24, detail: 0.35 },
        medium: { particles: 180, weather: 0.6, lighting: true, bloom: false, chunkCache: 40, detail: 0.7 },
        high: { particles: 380, weather: 1, lighting: true, bloom: true, chunkCache: 64, detail: 1 },
        ultra: { particles: 620, weather: 1.4, lighting: true, bloom: true, chunkCache: 96, detail: 1.35 }
    };

    AD.defaultSettings = function () {
        return {
            quality: 'auto', volume: 0.65, muted: false,
            reducedMotion: false, weapons: true, showGhost: true
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

    AD.BEST_KEY = 'aurago.desktop.aura-drift.best.v1';
    AD.loadBests = function () {
        try {
            const raw = window.localStorage && window.localStorage.getItem(AD.BEST_KEY);
            const parsed = raw ? JSON.parse(raw) : {};
            return parsed && typeof parsed === 'object' ? parsed : {};
        } catch (_) { return {}; }
    };
    AD.saveBests = function (bests, readonly) {
        if (readonly) return;
        try {
            if (window.localStorage) window.localStorage.setItem(AD.BEST_KEY, JSON.stringify(bests));
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
