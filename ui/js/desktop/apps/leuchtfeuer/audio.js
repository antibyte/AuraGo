/* Leuchtfeuer presentation config. The coastal environment owns the water. */
(function (root) {
    const EFFECT_IDS = ['coast', 'sky-clear', 'water-ocean', 'color-grade', 'fog-ground', 'water-splash', 'water-ripple', 'fire', 'magic', 'pickup-glow'];
    const SOUND_IDS = ['ocean', 'wind-gentle', 'step-wood', 'step-gravel', 'pickup', 'water-splash', 'door-wood', 'victory', 'fire'];
    const BASE = '/api/game-maker/asset-packs/aurago-sounds/';

    function take(manifest, ids, label) {
        const assets = manifest && manifest.assets;
        if (!assets) throw new Error('leuchtfeuer ' + label);
        const picked = [];
        ids.forEach(function (id) {
            const found = assets.find(function (asset) { return asset.id === id; });
            if (!found) throw new Error('leuchtfeuer ' + label + ' ' + id);
            picked.push(found);
        });
        return picked;
    }

    function presentationConfig(effectsManifest, soundsManifest) {
        return {
            environment: 'coast',
            effects: take(effectsManifest, EFFECT_IDS, 'effect'),
            sounds: take(soundsManifest, SOUND_IDS, 'sound'),
            bindings: [
                { event: 'step', sound: 'step-wood' },
                { event: 'pickup', sound: 'pickup' },
                { event: 'splash', sound: 'water-splash' },
                { event: 'interact', sound: 'door-wood' },
                { event: 'win', sound: 'victory' }
            ],
            quality: 'medium',
            version: '1.0.0',
            base: BASE
        };
    }

    root.LeuchtfeuerAudio = {
        EFFECT_IDS: EFFECT_IDS,
        SOUND_IDS: SOUND_IDS,
        BASE: BASE,
        presentationConfig: presentationConfig
    };
})(typeof window !== 'undefined' ? window : globalThis);
