(function (rootScope) {
    'use strict';

    // Web Audio 8-band equalizer engine, factory presets, custom profile store,
    // and RBJ frequency-response curve solver for the native AuraGo music player.

    const STORAGE_KEY = 'aurago.audio-player.eq.v1';
    const MIN_GAIN = -12;
    const MAX_GAIN = 12;
    const GAIN_STEP = 0.5;
    const MAX_PROFILES = 24;
    const MAX_PROFILE_NAME = 40;
    const CURVE_SAMPLE_RATE = 48000;

    const BANDS = Object.freeze([
        Object.freeze({ freq: 60, filterFreq: 92, label: '60', freqLabel: '60 Hz', type: 'lowshelf', q: 0.7 }),
        Object.freeze({ freq: 170, label: '170', freqLabel: '170 Hz', type: 'peaking', q: 1.15 }),
        Object.freeze({ freq: 350, label: '350', freqLabel: '350 Hz', type: 'peaking', q: 1.2 }),
        Object.freeze({ freq: 1000, label: '1k', freqLabel: '1 kHz', type: 'peaking', q: 1.2 }),
        Object.freeze({ freq: 2400, label: '2.4k', freqLabel: '2.4 kHz', type: 'peaking', q: 1.2 }),
        Object.freeze({ freq: 6000, label: '6k', freqLabel: '6 kHz', type: 'peaking', q: 1.2 }),
        Object.freeze({ freq: 12000, label: '12k', freqLabel: '12 kHz', type: 'peaking', q: 1.15 }),
        Object.freeze({ freq: 16000, filterFreq: 12500, label: '16k', freqLabel: '16 kHz', type: 'highshelf', q: 0.7 })
    ]);

    function makePreset(key, bands) {
        return Object.freeze({
            id: 'preset:' + key,
            key,
            labelKey: 'desktop.audio_player_eq_preset_' + key,
            builtin: true,
            bands: Object.freeze(bands.slice())
        });
    }

    const PRESETS = Object.freeze([
        makePreset('flat', [0, 0, 0, 0, 0, 0, 0, 0]),
        makePreset('bass_boost', [6.5, 5.0, 2.5, 0, -0.5, 0.5, 1.5, 2.0]),
        makePreset('electronic', [5.5, 4.0, 1.0, -1.5, 1.0, 3.5, 4.5, 3.5]),
        makePreset('rock', [4.5, 3.0, -1.0, -1.5, 1.5, 3.5, 4.5, 3.0]),
        makePreset('pop', [-1.0, 1.5, 3.5, 4.0, 2.5, 0.5, -0.5, -1.0]),
        makePreset('vocal', [-2.5, -1.5, 1.5, 4.5, 5.0, 3.0, 0.5, -1.5]),
        makePreset('acoustic', [2.0, 1.5, 1.0, 0.5, 2.0, 3.0, 2.5, 1.5]),
        makePreset('jazz', [2.5, 1.5, 1.0, 2.0, -1.0, 1.5, 2.5, 2.0]),
        makePreset('classical', [2.5, 1.5, 0, 0, 0.5, 1.5, 2.5, 3.0]),
        makePreset('late_night', [-4.0, -2.0, 1.0, 2.5, 2.0, 1.0, -1.5, -3.0])
    ]);

    const PRESET_MAP = new Map(PRESETS.map(p => [p.id, p]));

    function clamp(value, min, max) {
        return Math.max(min, Math.min(max, value));
    }

    function snapGain(value) {
        const num = Number(value);
        if (!Number.isFinite(num)) return 0;
        const clamped = clamp(num, MIN_GAIN, MAX_GAIN);
        return Math.round(clamped / GAIN_STEP) * GAIN_STEP;
    }

    function normalizeBands(input) {
        const out = new Array(BANDS.length).fill(0);
        if (!Array.isArray(input)) return out;
        for (let i = 0; i < BANDS.length; i++) {
            out[i] = snapGain(input[i]);
        }
        return out;
    }

    function sanitizeProfileName(raw) {
        const collapsed = String(raw == null ? '' : raw).replace(/\s+/g, ' ').trim();
        if (!collapsed) return '';
        return Array.from(collapsed).slice(0, MAX_PROFILE_NAME).join('').trim();
    }

    function slugify(name) {
        const ascii = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
        return ascii.slice(0, 20) || 'profile';
    }

    function bandsEqual(a, b) {
        if (!Array.isArray(a) || !Array.isArray(b)) return false;
        for (let i = 0; i < BANDS.length; i++) {
            if (Math.abs((Number(a[i]) || 0) - (Number(b[i]) || 0)) >= 0.05) return false;
        }
        return true;
    }

    function resolveStorage(explicit) {
        if (explicit) return explicit;
        try {
            if (typeof window !== 'undefined' && window.localStorage) return window.localStorage;
        } catch (_) { /* storage disabled */ }
        return null;
    }

    function sanitizeProfiles(rawProfiles) {
        if (!Array.isArray(rawProfiles)) return [];
        const list = [];
        const seenIds = new Set();
        const seenNames = new Set();
        for (let i = 0; i < rawProfiles.length && list.length < MAX_PROFILES; i++) {
            const entry = rawProfiles[i];
            if (!entry || typeof entry !== 'object') continue;
            const name = sanitizeProfileName(entry.name);
            if (!name) continue;
            const nameKey = name.toLowerCase();
            if (seenNames.has(nameKey)) continue;
            let id = typeof entry.id === 'string' && entry.id.startsWith('user:') ? entry.id.trim() : '';
            if (!id || seenIds.has(id)) {
                id = 'user:' + slugify(name) + '-' + (i + 1);
            }
            if (seenIds.has(id)) continue;
            seenIds.add(id);
            seenNames.add(nameKey);
            list.push({
                id,
                name,
                bands: normalizeBands(entry.bands)
            });
        }
        return list;
    }

    function defaultState() {
        return {
            version: 1,
            enabled: true,
            activeId: 'preset:flat',
            bands: new Array(BANDS.length).fill(0),
            profiles: []
        };
    }

    function findEntry(state, id) {
        if (!id || id === 'custom') return null;
        if (PRESET_MAP.has(id)) return PRESET_MAP.get(id);
        return (state.profiles || []).find(p => p.id === id) || null;
    }

    function loadState(storage) {
        const store = resolveStorage(storage);
        const fallback = defaultState();
        if (!store) return fallback;
        try {
            const raw = store.getItem(STORAGE_KEY);
            if (!raw) return fallback;
            const parsed = JSON.parse(raw);
            if (!parsed || typeof parsed !== 'object') return fallback;
            const profiles = sanitizeProfiles(parsed.profiles);
            const bands = normalizeBands(parsed.bands);
            const hasActiveFlag = typeof parsed.enabled === 'boolean';
            let activeId = typeof parsed.activeId === 'string' ? parsed.activeId : 'preset:flat';
            const candidate = PRESET_MAP.get(activeId) || profiles.find(p => p.id === activeId);
            if (!candidate) {
                activeId = bands.some(v => Math.abs(v) >= 0.05) ? 'custom' : 'preset:flat';
            } else if (!bandsEqual(candidate.bands, bands)) {
                activeId = 'custom';
            }
            const nonZero = bands.some(v => Math.abs(v) >= 0.05);
            return {
                version: 1,
                enabled: hasActiveFlag ? parsed.enabled : (nonZero || true),
                activeId,
                bands,
                profiles
            };
        } catch (_) {
            return fallback;
        }
    }

    function saveState(state, storage) {
        const store = resolveStorage(storage);
        if (!store) return false;
        try {
            const payload = {
                version: 1,
                enabled: !!state.enabled,
                activeId: state.activeId || 'preset:flat',
                bands: normalizeBands(state.bands),
                profiles: sanitizeProfiles(state.profiles)
            };
            store.setItem(STORAGE_KEY, JSON.stringify(payload));
            return true;
        } catch (_) {
            return false;
        }
    }

    function cloneState(state) {
        return {
            version: 1,
            enabled: !!state.enabled,
            activeId: state.activeId,
            bands: state.bands.slice(),
            profiles: state.profiles.map(p => ({ id: p.id, name: p.name, bands: p.bands.slice() }))
        };
    }

    function createStore(storage) {
        const resolved = resolveStorage(storage);
        const state = loadState(resolved);

        function persist() {
            saveState(state, resolved);
        }

        return {
            snapshot() {
                return cloneState(state);
            },
            isModified() {
                return state.activeId === 'custom';
            },
            isEffective() {
                return !!state.enabled && state.bands.some(v => Math.abs(v) >= 0.05);
            },
            activeProfile() {
                if (!state.activeId || !state.activeId.startsWith('user:')) return null;
                return state.profiles.find(p => p.id === state.activeId) || null;
            },
            setEnabled(enabled) {
                state.enabled = !!enabled;
                persist();
                return cloneState(state);
            },
            setBand(index, gainDb) {
                const idx = Number(index);
                if (!Number.isInteger(idx) || idx < 0 || idx >= BANDS.length) return cloneState(state);
                state.bands[idx] = snapGain(gainDb);
                const matchingPreset = PRESETS.find(p => bandsEqual(p.bands, state.bands));
                const currentEntry = findEntry(state, state.activeId);
                if (currentEntry && bandsEqual(currentEntry.bands, state.bands)) {
                    // unchanged against active profile
                } else if (matchingPreset && state.activeId === matchingPreset.id) {
                    state.activeId = matchingPreset.id;
                } else if (state.bands.every(v => v === 0) && (!state.activeId || state.activeId.startsWith('preset:'))) {
                    state.activeId = 'preset:flat';
                } else {
                    state.activeId = 'custom';
                }
                persist();
                return cloneState(state);
            },
            selectProfile(id) {
                const entry = findEntry(state, id);
                if (!entry) return false;
                state.bands = normalizeBands(entry.bands);
                state.activeId = entry.id;
                if (state.bands.some(v => Math.abs(v) >= 0.05)) {
                    state.enabled = true;
                }
                persist();
                return true;
            },
            saveProfile(rawName) {
                const name = sanitizeProfileName(rawName);
                if (!name) return { ok: false, reason: 'empty_name' };
                const nameKey = name.toLowerCase();
                const existing = state.profiles.find(p => p.name.toLowerCase() === nameKey);
                if (existing) {
                    existing.name = name;
                    existing.bands = normalizeBands(state.bands);
                    state.activeId = existing.id;
                    persist();
                    return { ok: true, updated: true, profile: { id: existing.id, name: existing.name, bands: existing.bands.slice() } };
                }
                if (state.profiles.length >= MAX_PROFILES) {
                    return { ok: false, reason: 'limit_reached' };
                }
                const uniqueId = 'user:' + slugify(name) + '-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 5);
                const profile = { id: uniqueId, name, bands: normalizeBands(state.bands) };
                state.profiles.push(profile);
                state.activeId = profile.id;
                persist();
                return { ok: true, updated: false, profile: { id: profile.id, name: profile.name, bands: profile.bands.slice() } };
            },
            deleteProfile(id) {
                const idx = state.profiles.findIndex(p => p.id === id);
                if (idx < 0) return { ok: false, reason: 'not_found' };
                const removed = state.profiles.splice(idx, 1)[0];
                if (state.activeId === removed.id) {
                    state.activeId = state.bands.every(v => v === 0) ? 'preset:flat' : 'custom';
                }
                persist();
                return { ok: true, profile: removed };
            },
            reset() {
                state.bands = new Array(BANDS.length).fill(0);
                state.activeId = 'preset:flat';
                persist();
                return cloneState(state);
            }
        };
    }

    function buildChain(audioContext) {
        if (!audioContext || typeof audioContext.createBiquadFilter !== 'function') return null;
        const filters = [];
        let prev = null;
        for (let i = 0; i < BANDS.length; i++) {
            const spec = BANDS[i];
            const filter = audioContext.createBiquadFilter();
            filter.type = spec.type;
            filter.frequency.value = spec.filterFreq || spec.freq;
            if (spec.type === 'peaking' && filter.Q) {
                filter.Q.value = spec.q;
            }
            filter.gain.value = 0;
            if (prev) prev.connect(filter);
            prev = filter;
            filters.push(filter);
        }
        let limiter = null;
        let output = filters[filters.length - 1];
        if (typeof audioContext.createDynamicsCompressor === 'function') {
            limiter = audioContext.createDynamicsCompressor();
            limiter.threshold.value = -1.5;
            limiter.knee.value = 10;
            limiter.ratio.value = 6;
            limiter.attack.value = 0.004;
            limiter.release.value = 0.1;
            output.connect(limiter);
            output = limiter;
        }
        return {
            input: filters[0],
            output,
            filters,
            limiter
        };
    }

    function applyToChain(chain, audioContext, eqState, immediate) {
        if (!chain || !Array.isArray(chain.filters)) return;
        const enabled = !!(eqState && eqState.enabled);
        const bands = (eqState && eqState.bands) || [];
        const now = audioContext && Number.isFinite(audioContext.currentTime) ? audioContext.currentTime : 0;
        for (let i = 0; i < chain.filters.length; i++) {
            const param = chain.filters[i] && chain.filters[i].gain;
            if (!param) continue;
            const target = enabled ? snapGain(bands[i]) : 0;
            if (typeof param.cancelScheduledValues === 'function') {
                param.cancelScheduledValues(now);
            }
            if (!immediate && typeof param.setTargetAtTime === 'function') {
                param.setTargetAtTime(target, now, 0.025);
            } else if (typeof param.setValueAtTime === 'function') {
                param.setValueAtTime(target, now);
            } else {
                param.value = target;
            }
        }
    }

    function biquadCoeffs(spec, gainDb, sampleRate) {
        const fs = sampleRate || CURVE_SAMPLE_RATE;
        const A = Math.pow(10, gainDb / 40);
        const f0 = Math.min(spec.filterFreq || spec.freq, fs * 0.48);
        const w0 = (2 * Math.PI * f0) / fs;
        const cosw = Math.cos(w0);
        const sinw = Math.sin(w0);
        if (spec.type === 'lowshelf') {
            const alpha = (sinw * 0.5) * Math.SQRT2;
            const sq2A = 2 * Math.sqrt(A) * alpha;
            return {
                b0: A * ((A + 1) - (A - 1) * cosw + sq2A),
                b1: 2 * A * ((A - 1) - (A + 1) * cosw),
                b2: A * ((A + 1) - (A - 1) * cosw - sq2A),
                a0: (A + 1) + (A - 1) * cosw + sq2A,
                a1: -2 * ((A - 1) + (A + 1) * cosw),
                a2: (A + 1) + (A - 1) * cosw - sq2A
            };
        }
        if (spec.type === 'highshelf') {
            const alpha = (sinw * 0.5) * Math.SQRT2;
            const sq2A = 2 * Math.sqrt(A) * alpha;
            return {
                b0: A * ((A + 1) + (A - 1) * cosw + sq2A),
                b1: -2 * A * ((A - 1) + (A + 1) * cosw),
                b2: A * ((A + 1) + (A - 1) * cosw - sq2A),
                a0: (A + 1) - (A - 1) * cosw + sq2A,
                a1: 2 * ((A - 1) - (A + 1) * cosw),
                a2: (A + 1) - (A - 1) * cosw - sq2A
            };
        }
        const alpha = sinw / (2 * (spec.q || 1.2));
        return {
            b0: 1 + alpha * A,
            b1: -2 * cosw,
            b2: 1 - alpha * A,
            a0: 1 + alpha / A,
            a1: -2 * cosw,
            a2: 1 - alpha / A
        };
    }

    function responseDbAt(freqHz, bands, enabled) {
        if (!enabled) return 0;
        const gains = normalizeBands(bands);
        const w = (2 * Math.PI * clamp(Number(freqHz) || 20, 10, CURVE_SAMPLE_RATE * 0.48)) / CURVE_SAMPLE_RATE;
        const cw = Math.cos(w);
        const sw = Math.sin(w);
        const c2w = Math.cos(2 * w);
        const s2w = Math.sin(2 * w);
        let totalDb = 0;
        for (let i = 0; i < BANDS.length; i++) {
            const g = gains[i];
            if (Math.abs(g) < 0.01) continue;
            const c = biquadCoeffs(BANDS[i], g, CURVE_SAMPLE_RATE);
            const nr = c.b0 + c.b1 * cw + c.b2 * c2w;
            const ni = -c.b1 * sw - c.b2 * s2w;
            const dr = c.a0 + c.a1 * cw + c.a2 * c2w;
            const di = -c.a1 * sw - c.a2 * s2w;
            const mag2 = (nr * nr + ni * ni) / Math.max(1e-24, dr * dr + di * di);
            totalDb += 10 * Math.log10(Math.max(1e-12, mag2));
        }
        return clamp(totalDb, -14, 14);
    }

    function freqToNorm(freqHz, minFreq, maxFreq) {
        const logMin = Math.log10(minFreq);
        const logMax = Math.log10(maxFreq);
        return clamp((Math.log10(clamp(freqHz, minFreq, maxFreq)) - logMin) / (logMax - logMin), 0, 1);
    }

    function computeCurve(bands, enabled, width, height, samples) {
        const w = Math.max(40, Number(width) || 480);
        const h = Math.max(24, Number(height) || 68);
        const count = Math.max(16, Math.round(Number(samples) || 88));
        const minFreq = 30;
        const maxFreq = 20000;
        const logMin = Math.log10(minFreq);
        const logSpan = Math.log10(maxFreq) - logMin;
        const padX = 12;
        const padY = 8;
        const innerW = w - padX * 2;
        const innerH = h - padY * 2;
        const zeroY = padY + innerH * 0.5;
        const dbToY = db => clamp(zeroY - (clamp(db, -14, 14) / 14) * (innerH * 0.5), padY * 0.5, h - padY * 0.5);

        const points = [];
        for (let i = 0; i <= count; i++) {
            const ratio = i / count;
            const freq = Math.pow(10, logMin + ratio * logSpan);
            const db = responseDbAt(freq, bands, enabled);
            points.push([
                Number((padX + ratio * innerW).toFixed(2)),
                Number(dbToY(db).toFixed(2))
            ]);
        }

        const normGains = normalizeBands(bands);
        const bandDots = BANDS.map((spec, idx) => {
            const ratio = freqToNorm(spec.freq, minFreq, maxFreq);
            const db = enabled ? responseDbAt(spec.freq, normGains, true) : 0;
            return {
                x: Number((padX + ratio * innerW).toFixed(2)),
                y: Number(dbToY(db).toFixed(2)),
                gain: enabled ? normGains[idx] : 0,
                freq: spec.freq
            };
        });

        return { points, bandDots, zeroY: Number(zeroY.toFixed(2)), width: w, height: h };
    }

    const api = Object.freeze({
        STORAGE_KEY,
        MIN_GAIN,
        MAX_GAIN,
        GAIN_STEP,
        MAX_PROFILES,
        MAX_PROFILE_NAME,
        BANDS,
        PRESETS,
        clamp,
        snapGain,
        normalizeBands,
        sanitizeProfileName,
        bandsEqual,
        loadState,
        saveState,
        createStore,
        buildChain,
        applyToChain,
        responseDbAt,
        computeCurve
    });

    if (typeof rootScope !== 'undefined' && rootScope) {
        rootScope.AudioPlayerEQ = api;
    }
    if (typeof module !== 'undefined' && module.exports) {
        module.exports = api;
    }
})(typeof window !== 'undefined' ? window : globalThis);
