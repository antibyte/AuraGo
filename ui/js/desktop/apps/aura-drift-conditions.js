/* Aura Drift - dynamic race conditions: weather schedule, grip, gusts, lightning and ambience.
 *
 * Everything here runs in the fixed 120 Hz simulation. Randomness comes only from the seeded
 * race rng at construction time; the per-step values are closed-form functions of simulation
 * time, so an input tape replays the same wind, the same wet road and the same lightning.
 * Presentation reads `state`; it never writes it. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const TAU = Math.PI * 2;
    /* Which conditions carry a storm, and how bright a strike can be. */
    const STORM = { rain: 1, snow: 0.55, mist: 0, fog: 0, haze: 0, heat: 0, glare: 0, clear: 0 };

    /* Every biome can drift through a small set of conditions. The profile
       also carries the ambience mix the audio engine needs: wind, water,
       machinery, birds and crowd. `storm` biases how often lightning is placed. */
    AD.CONDITIONS = {
        alpine: {
            kinds: ['clear', 'fog', 'snow', 'clear'],
            grip: { clear: 1, fog: 0.97, snow: 0.82 },
            wind: 0.55, storm: 0.5, ambience: { wind: 0.6, birds: 0.25, machinery: 0, water: 0, crowd: 0.05, reverb: 0.15 }
        },
        desert: {
            kinds: ['clear', 'heat', 'clear', 'glare'],
            grip: { clear: 1, heat: 0.95, glare: 0.94 },
            wind: 0.7, storm: 0.2, ambience: { wind: 0.75, birds: 0.05, machinery: 0, water: 0, crowd: 0, reverb: 0.05 }
        },
        harbour: {
            kinds: ['rain', 'rain', 'clear', 'rain'],
            grip: { clear: 1, rain: 0.88, fog: 0.94 },
            wind: 0.5, storm: 1, ambience: { wind: 0.45, birds: 0.12, machinery: 0.18, water: 0.6, crowd: 0, reverb: 0.2 }
        },
        salt: {
            kinds: ['glare', 'clear', 'glare', 'clear'],
            grip: { clear: 1, glare: 0.95, heat: 0.96 },
            wind: 0.8, storm: 0.2, ambience: { wind: 0.85, birds: 0.06, machinery: 0, water: 0, crowd: 0, reverb: 0.04 }
        },
        forest: {
            kinds: ['mist', 'clear', 'rain', 'mist'],
            grip: { clear: 1, mist: 0.95, rain: 0.9 },
            wind: 0.35, storm: 0.7, ambience: { wind: 0.35, birds: 0.55, machinery: 0, water: 0.15, crowd: 0, reverb: 0.12 }
        },
        industry: {
            kinds: ['haze', 'clear', 'haze', 'fog'],
            grip: { clear: 1, haze: 0.96, fog: 0.94 },
            wind: 0.4, storm: 0.35, ambience: { wind: 0.4, birds: 0.05, machinery: 0.75, water: 0.08, crowd: 0, reverb: 0.35 }
        },
        glacier: {
            kinds: ['snow', 'clear', 'snow', 'fog'],
            grip: { clear: 0.97, snow: 0.8, fog: 0.93 },
            wind: 0.75, storm: 0.65, ambience: { wind: 0.8, birds: 0.04, machinery: 0.05, water: 0.1, crowd: 0, reverb: 0.22 }
        },
        metro: {
            kinds: ['clear', 'rain', 'clear', 'rain'],
            grip: { clear: 1, rain: 0.9, fog: 0.95 },
            wind: 0.3, storm: 0.9, ambience: { wind: 0.28, birds: 0.02, machinery: 0.3, water: 0.22, crowd: 0.3, reverb: 0.3 }
        }
    };

    AD.conditionProfile = function (biomeId) {
        return AD.CONDITIONS[biomeId] || AD.CONDITIONS.alpine;
    };

    /* A strike is two short pulses: the main flash and a dimmer echo 170 ms later. */
    function flashLevel(since, strength) {
        if (since < 0 || since > 1.25) return 0;
        const first = Math.exp(-since * 9);
        const delta = (since - 0.17) / 0.055;
        const echo = 0.72 * Math.exp(-delta * delta);
        return clamp((first + echo) * strength, 0, 1.1);
    }

    /* The schedule is a short, seeded list of conditions. It is generated once
       per race from the track seed, so a replay always sees the same weather
       in the same order. Strikes are placed inside storm segments, never in clear sky. */
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

        const strikes = [];
        for (let i = 0; i < schedule.length; i++) {
            const seg = schedule[i];
            const storm = (STORM[seg.kind] || 0) * (profile.storm || 0.5);
            if (storm < 0.2) continue;
            const span = (i + 1 < schedule.length ? schedule[i + 1].at : seg.at + 46) - seg.at;
            const count = 1 + Math.floor(rng() * 3);
            for (let k = 0; k < count; k++) {
                strikes.push({
                    at: seg.at + 3 + rng() * Math.max(4, span - 8),
                    strength: clamp((0.5 + rng() * 0.5) * storm, 0.2, 1),
                    thunder: 0.35 + rng() * 1.1
                });
            }
        }
        strikes.sort((a, b) => a.at - b.at);
        const phase = [rng() * TAU, rng() * TAU, rng() * TAU];

        const state = {
            kind: 'clear', prevKind: 'clear', blend: 1,
            intensity: 0, target: 'clear', targetIntensity: 0,
            grip: 1, wind: profile.wind, gust: 0.5, wetness: 0, visibility: 1, flash: 0,
            index: 0, strikeIndex: 0, time: 0,
            schedule: schedule, strikes: strikes,
            ambience: profile.ambience, profile: profile,
            events: [], lastGustAt: -99, gustHigh: false, lastStrike: null
        };

        /* D1: the scheduled intensity is the blend target. Overwriting it with
           the current intensity pinned weather at zero for the whole race. */
        function applyTarget(next) {
            if (next.kind !== state.kind) { state.prevKind = state.kind; state.blend = 0; }
            state.kind = next.kind;
            state.targetIntensity = next.intensity;
        }

        return {
            state: state,
            /* Called from the fixed-step simulation so replays stay identical. */
            update(dt) {
                state.time += dt;
                state.events.length = 0;
                while (state.index < schedule.length && state.time >= schedule[state.index].at) {
                    const next = schedule[state.index++];
                    state.target = next.kind;
                    applyTarget(next);
                }
                /* Kinds crossfade over eight seconds instead of snapping. */
                state.blend = Math.min(1, state.blend + dt / 8);
                const wanted = state.targetIntensity;
                state.intensity += (wanted - state.intensity) * clamp(dt * 0.35, 0, 1);

                /* Wind arrives in slow deterministic waves, never a per-frame random roll. */
                const t = state.time;
                state.gust = clamp(0.5 + 0.29 * Math.sin(t * 0.41 + phase[0]) +
                    0.15 * Math.sin(t * 0.97 + phase[1]) + 0.06 * Math.sin(t * 2.3 + phase[2]), 0, 1);
                if (state.gust > 0.82 && !state.gustHigh && t - state.lastGustAt > 7) {
                    state.gustHigh = true; state.lastGustAt = t;
                    state.events.push({ type: 'gust', strength: state.gust, wind: state.wind });
                } else if (state.gust < 0.7) state.gustHigh = false;

                /* Wet roads stay slippery for a while after the last drop falls. */
                const wetTarget = state.kind === 'rain' ? 0.35 + state.intensity * 0.65 :
                    (state.kind === 'snow' ? 0.15 + state.intensity * 0.25 : 0);
                state.wetness += (wetTarget - state.wetness) *
                    clamp(dt * (wetTarget > state.wetness ? 0.24 : 0.055), 0, 1);

                const table = profile.grip || {};
                const base = table[state.kind] !== undefined ? table[state.kind] : 1;
                /* Rain and snow bite deeper the harder they fall; gusts and puddles
                   shave off a little more, identically for every driver. */
                const depth = (state.kind === 'rain' || state.kind === 'snow') ? state.intensity : 0;
                state.wind = profile.wind * (0.7 + state.intensity * 0.6) * (0.85 + state.gust * 0.3);
                state.grip = clamp(base - depth * 0.12 - state.wetness * 0.025 -
                    state.gust * state.wind * 0.006, 0.6, 1.05);
                const murk = state.kind === 'fog' || state.kind === 'mist' ? 0.55 :
                    state.kind === 'snow' ? 0.35 : state.kind === 'rain' ? 0.2 : 0;
                state.visibility = clamp(1 - murk * state.intensity, 0.35, 1);

                /* Strikes are queued by simulation time, so a replay flashes identically. */
                while (state.strikeIndex < strikes.length && state.time >= strikes[state.strikeIndex].at) {
                    const s = strikes[state.strikeIndex++];
                    state.lastStrike = s;
                    state.events.push({ type: 'lightning', strength: s.strength, thunder: s.thunder });
                }
                state.flash = state.lastStrike ? flashLevel(state.time - state.lastStrike.at, state.lastStrike.strength) : 0;
                return state;
            },
            /* Presentation helpers: the renderers map the kind to their own look. */
            isWet() { return state.wetness > 0.2; },
            label() { return state.kind; }
        };
    };
})();
