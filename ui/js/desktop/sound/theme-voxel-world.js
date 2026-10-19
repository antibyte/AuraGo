(function () {
    'use strict';
    const S = window.DesktopSoundSynth;
    const L = S.layer;
    const materials = ['wood', 'stone', 'earth'];

    function impact(eventId, material) {
        const fracture = eventId === 'window.fracture';
        const spec = {
            wood: { midi: 43, cutoff: 950, noise: 0.11, tone: 0.18, periods: 9 },
            stone: { midi: 55, cutoff: 1850, noise: 0.2, tone: 0.15, periods: 5 },
            earth: { midi: 38, cutoff: 620, noise: 0.23, tone: 0.11, periods: 12 }
        }[material];
        const layers = [
            L('pluck', { midi: spec.midi - (fracture ? 5 : 0), duration: fracture ? 0.22 : 0.15,
                gain: fracture ? 0.3 : 0.34, periods: spec.periods, decay: fracture ? 2.1 : 2.8,
                filter: { type: 'lowpass', freq: spec.cutoff, q: 0.9 }, reverb: 'short' }),
            L('noise', { at: 0.004, duration: fracture ? 0.21 : 0.12, gain: spec.noise,
                color: 'pink', decayPower: fracture ? 2.6 : 3.1,
                filter: { type: material === 'stone' ? 'bandpass' : 'lowpass', freq: spec.cutoff, q: 0.9 }, reverb: 'short' }),
            L('tone', { at: 0.015, midi: spec.midi - (fracture ? 12 : 7), duration: 0.18,
                gain: spec.tone, attack: 0.002, decay: 0.07, sustain: 0, release: 0.12,
                type: 'triangle', filter: { type: 'lowpass', freq: spec.cutoff * 1.4, q: 0.7 }, reverb: 'short' })
        ];
        if (fracture) layers.push(L('noise', { at: 0.075, duration: 0.1, gain: spec.noise * 0.42,
            color: 'pink', decayPower: 3.6,
            filter: { type: 'highpass', freq: material === 'stone' ? 2200 : 1100 }, reverb: 'short' }));
        return layers;
    }

    const specialFx = {};
    ['window.fracture', 'window.impact'].forEach(eventId => {
        specialFx[eventId] = Object.fromEntries(materials.map(material => [material, impact(eventId, material)]));
    });

    window.DesktopSoundThemes.voxel_world = {
        id: 'voxel_world',
        events: S.composeDesktopEvents({ voice: 'voxel', root: 48, rise: 3, fall: -5, color: 2,
            step: 0.064, gain: 0.22, filter: 1380, noiseCutoff: 1150, periods: 7 }),
        specialFx
    };
})();
