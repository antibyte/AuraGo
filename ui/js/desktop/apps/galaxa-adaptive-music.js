(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    const LAYER_GAIN_LIMITS = { max: 0.04 };

    const BIOME_LAYERS = {
        nebula: { wave: 'sine', vol: 0.032, notes: [{ f: 220, d: 4 }, { f: 349.23, d: 4 }, { f: 293.66, d: 4 }, { f: 329.63, d: 4 }] },
        asteroid: { wave: 'sawtooth', vol: 0.026, noise: true, freq: 8000, notes: [{ f: 146.83, d: 4 }, { f: 174.61, d: 4 }, { f: 196.00, d: 4 }, { f: 155.56, d: 4 }] },
        crystal: { wave: 'triangle', vol: 0.03, notes: [{ f: 1174.66, d: 1 }, { f: 1479.98, d: 1 }, { f: 1760, d: 1 }, { f: 2093, d: 1 }] },
        storm: { wave: 'sawtooth', vol: 0.036, notes: [{ f: 82.41, d: 4 }, { f: 146.83, d: 4 }, { f: 130.81, d: 4 }, { f: 123.47, d: 4 }] },
        blackhole: { wave: 'sine', vol: 0.038, notes: [{ f: 65.41, d: 4 }, { f: 69.30, d: 4 }, { f: 103.83, d: 4 }, { f: 98.00, d: 4 }] },
        void: { wave: 'triangle', vol: 0.028, notes: [{ f: 110, d: 4 }, { f: 880, d: 1 }, { f: 174.61, d: 4 }, { f: 1046.50, d: 1 }] }
    };

    GC.createAdaptiveMusic = function (ctx) {
        let activeLayers = {};
        let modulation = { combo: 0, bossPhase: 0, health: 3 };

        function addLayer(themeId, layerId) {
            const layerDef = BIOME_LAYERS[layerId];
            if (!layerDef) return null;
            const gain = Math.min(LAYER_GAIN_LIMITS.max, layerDef.vol || 0.03);
            if (!ctx.MusicEngine) return null;
            const layer = ctx.MusicEngine.addLayer && ctx.MusicEngine.addLayer(layerId, layerDef);
            if (layer && ctx.MusicEngine.setLayerGain) {
                ctx.MusicEngine.setLayerGain(themeId, layerId, gain);
            }
            activeLayers[layerId] = { gain, themeId };
            return layer;
        }

        function removeLayer(themeId, layerId) {
            if (ctx.MusicEngine && ctx.MusicEngine.removeLayer) {
                ctx.MusicEngine.removeLayer(themeId, layerId);
            }
            delete activeLayers[layerId];
        }

        function modulate(type, value) {
            modulation[type] = value;
            if (!ctx.MusicEngine) return;
            if (!ctx.settings.adaptiveMusic) { ctx.MusicEngine.setTempo(1); ctx.MusicEngine.transpose(0); return; }
            if (type === 'combo') {
                if (value >= 20) {
                    ctx.MusicEngine.setTempo && ctx.MusicEngine.setTempo(1.5);
                    ctx.MusicEngine.setIntensity && ctx.MusicEngine.setIntensity(9);
                } else if (value >= 10) {
                    ctx.MusicEngine.setTempo && ctx.MusicEngine.setTempo(1.25);
                    ctx.MusicEngine.setIntensity && ctx.MusicEngine.setIntensity(7);
                } else {
                    ctx.MusicEngine.setTempo && ctx.MusicEngine.setTempo(1.0);
                    if (modulation.health > 1) ctx.MusicEngine.setIntensity && ctx.MusicEngine.setIntensity(4);
                }
            } else if (type === 'bossPhase') {
                if (ctx.MusicEngine.transpose) ctx.MusicEngine.transpose(value >= 2 ? 2 : 0);
                if (value >= 2 && ctx.MusicEngine.setIntensity) ctx.MusicEngine.setIntensity(8);
            } else if (type === 'health') {
                if (value <= 1 && ctx.MusicEngine.setIntensity) ctx.MusicEngine.setIntensity(8);
                else if (modulation.combo < 10 && ctx.MusicEngine.setIntensity) ctx.MusicEngine.setIntensity(4);
            }
        }

        function activeBaseTheme() {
            if (ctx.modesGetBaseMusicTheme && ctx.G) return ctx.modesGetBaseMusicTheme(!!ctx.G.chal);
            return ctx.G && ctx.G.chal ? 'challenge' : 'gameplay';
        }

        function applyBiomeAmbient(biomeId) {
            for (const layerId of Object.keys(activeLayers)) {
                removeLayer(activeLayers[layerId].themeId, layerId);
            }
            if (biomeId && BIOME_LAYERS[biomeId]) {
                addLayer(activeBaseTheme(), biomeId);
            }
        }

        ctx.addMusicLayer = addLayer;
        ctx.removeMusicLayer = removeLayer;
        ctx.modulateMusic = modulate;
        ctx.applyBiomeAmbient = applyBiomeAmbient;
    };
})();
