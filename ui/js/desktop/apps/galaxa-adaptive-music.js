(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    const LAYER_GAIN_LIMITS = { max: 0.04 };

    const BIOME_LAYERS = {
        nebula: { wave: 'sine', vol: 0.03, notes: [{ f: 220, d: 4 }, { f: 247, d: 4 }, { f: 196, d: 4 }, { f: 220, d: 4 }] },
        asteroid: { wave: 'sawtooth', vol: 0.025, noise: true, freq: 8000, notes: [] },
        crystal: { wave: 'triangle', vol: 0.03, notes: [{ f: 1318, d: 0.5 }, { f: 1568, d: 0.5 }, { f: 1760, d: 0.5 }, { f: 2093, d: 0.5 }] },
        storm: { wave: 'sawtooth', vol: 0.04, notes: [{ f: 55, d: 4 }, { f: 73, d: 4 }, { f: 65, d: 4 }, { f: 55, d: 4 }] },
        blackhole: { wave: 'sine', vol: 0.04, notes: [{ f: 40, d: 4 }, { f: 55, d: 4 }, { f: 49, d: 4 }, { f: 40, d: 4 }] },
        void: { wave: 'triangle', vol: 0.02, notes: [{ f: 0, d: 8 }, { f: 1047, d: 0.5 }, { f: 0, d: 8 }] }
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
            if (type === 'combo') {
                if (value >= 20) ctx.MusicEngine.setTempo && ctx.MusicEngine.setTempo(1.5);
                else if (value >= 10) ctx.MusicEngine.setTempo && ctx.MusicEngine.setTempo(1.25);
                else ctx.MusicEngine.setTempo && ctx.MusicEngine.setTempo(1.0);
            } else if (type === 'bossPhase') {
                if (value >= 2 && ctx.MusicEngine.transpose) ctx.MusicEngine.transpose(2);
            } else if (type === 'health') {
                if (value <= 1 && ctx.MusicEngine.setIntensity) ctx.MusicEngine.setIntensity(8);
            }
        }

        function applyBiomeAmbient(biomeId) {
            for (const layerId of Object.keys(activeLayers)) {
                removeLayer('gameplay', layerId);
            }
            if (biomeId && BIOME_LAYERS[biomeId]) {
                addLayer('gameplay', biomeId);
            }
        }

        ctx.addMusicLayer = addLayer;
        ctx.removeMusicLayer = removeLayer;
        ctx.modulateMusic = modulate;
        ctx.applyBiomeAmbient = applyBiomeAmbient;
    };
})();
