/* Aura Drift - dynamic race conditions: weather schedule, grip and ambience. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    /* Every biome can drift through a small set of conditions. The profile
       also carries the ambience mix the audio engine needs: wind, water,
       machinery, birds and crowd. */
    AD.CONDITIONS = {
        alpine: {
            kinds: ['clear', 'fog', 'snow', 'clear'],
            grip: { clear: 1, fog: 0.97, snow: 0.82 },
            wind: 0.55, ambience: { wind: 0.6, birds: 0.25, machinery: 0, water: 0, crowd: 0.05, reverb: 0.15 }
        },
        desert: {
            kinds: ['clear', 'heat', 'clear', 'glare'],
            grip: { clear: 1, heat: 0.95, glare: 0.94 },
            wind: 0.7, ambience: { wind: 0.75, birds: 0.05, machinery: 0, water: 0, crowd: 0, reverb: 0.05 }
        },
        harbour: {
            kinds: ['rain', 'rain', 'clear', 'rain'],
            grip: { clear: 1, rain: 0.88, fog: 0.94 },
            wind: 0.5, ambience: { wind: 0.45, birds: 0.12, machinery: 0.18, water: 0.6, crowd: 0, reverb: 0.2 }
        },
        salt: {
            kinds: ['glare', 'clear', 'glare', 'clear'],
            grip: { clear: 1, glare: 0.95, heat: 0.96 },
            wind: 0.8, ambience: { wind: 0.85, birds: 0.06, machinery: 0, water: 0, crowd: 0, reverb: 0.04 }
        },
        forest: {
            kinds: ['mist', 'clear', 'rain', 'mist'],
            grip: { clear: 1, mist: 0.95, rain: 0.9 },
            wind: 0.35, ambience: { wind: 0.35, birds: 0.55, machinery: 0, water: 0.15, crowd: 0, reverb: 0.12 }
        },
        industry: {
            kinds: ['haze', 'clear', 'haze', 'fog'],
            grip: { clear: 1, haze: 0.96, fog: 0.94 },
            wind: 0.4, ambience: { wind: 0.4, birds: 0.05, machinery: 0.75, water: 0.08, crowd: 0, reverb: 0.35 }
        },
        glacier: {
            kinds: ['snow', 'clear', 'snow', 'fog'],
            grip: { clear: 0.97, snow: 0.8, fog: 0.93 },
            wind: 0.75, ambience: { wind: 0.8, birds: 0.04, machinery: 0.05, water: 0.1, crowd: 0, reverb: 0.22 }
        },
        metro: {
            kinds: ['clear', 'rain', 'clear', 'rain'],
            grip: { clear: 1, rain: 0.9, fog: 0.95 },
            wind: 0.3, ambience: { wind: 0.28, birds: 0.02, machinery: 0.3, water: 0.22, crowd: 0.3, reverb: 0.3 }
        }
    };

    AD.conditionProfile = function (biomeId) {
        return AD.CONDITIONS[biomeId] || AD.CONDITIONS.alpine;
    };

    /* The schedule is a short, seeded list of conditions. It is generated once
       per race from the track seed, so a replay always sees the same weather
       in the same order. */
    AD.createConditions = function (track, seed) {
        const profile = AD.conditionProfile(track.biomeId);
        const rng = AD.rng(((seed >>> 0) ^ 0x9e3779b9) >>> 0 || 1);
        const schedule = [];
        let at = 12 + rng() * 14;
        for (let i = 0; i < 6; i++) {
            const kind = profile.kinds[Math.floor(rng() * profile.kinds.length)];
            const intensity = 0.35 + rng() * 0.65;
            schedule.push({ at: at, kind: kind, intensity: intensity });
            at += 24 + rng() * 26;
        }

        const state = {
            kind: 'clear',
            intensity: 0,
            target: 'clear',
            targetIntensity: 0,
            grip: 1,
            wind: profile.wind,
            index: 0,
            time: 0,
            schedule: schedule,
            ambience: profile.ambience,
            profile: profile
        };

        function applyTarget() {
            state.kind = state.target;
            state.targetIntensity = state.intensity;
        }

        return {
            state: state,
            /* Called from the fixed-step simulation so replays stay identical. */
            update(dt) {
                state.time += dt;
                while (state.index < schedule.length && state.time >= schedule[state.index].at) {
                    const next = schedule[state.index++];
                    state.target = next.kind;
                    state.targetIntensity = next.intensity;
                    applyTarget();
                }
                /* Blend the intensity so weather arrives instead of snapping. */
                const wanted = state.targetIntensity;
                state.intensity += (wanted - state.intensity) * clamp(dt * 0.35, 0, 1);
                const table = profile.grip || {};
                const base = table[state.kind] !== undefined ? table[state.kind] : 1;
                /* Rain and snow bite deeper the harder they fall. */
                const depth = (state.kind === 'rain' || state.kind === 'snow') ? state.intensity : 0;
                state.grip = clamp(base - depth * 0.12, 0.6, 1.05);
                state.wind = profile.wind * (0.7 + state.intensity * 0.6);
                return state;
            },
            /* Presentation helper: the renderer maps the kind to its own look. */
            isWet() { return state.kind === 'rain' && state.intensity > 0.2; },
            label() { return state.kind; }
        };
    };
})();
