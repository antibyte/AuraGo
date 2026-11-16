/* Runenpfad per-biome presentation model: sky, parallax layers, lighting, weather, ambience. DOM-free. */
(function (root) {
    'use strict';

    // Each biome drives the backdrop painter, the light map, the weather layer and the ambience mixer.
    const BIOMES = [
        {
            id: 'woodland',
            sky: ['#0b2438', '#1b4557', '#3c7a69'],
            celestial: { color: '#fff3c4', glow: '#ffd98a', radius: 42, x: 0.82, y: 0.2, alpha: 0.5 },
            layers: [
                { style: 'ridge', color: '#14323f', factor: 0.07, lift: -0.34, amp: 46, freq: 0.0031, seed: 11 },
                { style: 'forest', color: '#1c4648', factor: 0.19, lift: -0.20, amp: 34, freq: 0.0052, seed: 23 },
                { style: 'forest', color: '#255c4d', factor: 0.35, lift: -0.08, amp: 24, freq: 0.0081, seed: 37 }
            ],
            fog: { color: '#8fd8c6', alpha: 0.12 },
            light: { ambient: 0.08, color: '#ffe9b0', radius: 210, player: 0.5 },
            weather: { kind: 'leaf', color: '#bef264', color2: '#86efac', density: 30, speed: 26, size: 2.8 },
            grade: { color: '#0d2a26', alpha: 0.1 },
            ambience: { primary: 'forest-day', secondary: 'stream' }
        },
        {
            id: 'cavern',
            sky: ['#070512', '#171029', '#2d234a'],
            celestial: { color: '#9be7ff', glow: '#3ba7c9', radius: 30, x: 0.26, y: 0.16, alpha: 0.34 },
            layers: [
                { style: 'crystal', color: '#160f28', factor: 0.08, lift: -0.40, amp: 58, freq: 0.0026, seed: 41 },
                { style: 'ridge', color: '#20193b', factor: 0.2, lift: -0.22, amp: 40, freq: 0.0048, seed: 53 },
                { style: 'crystal', color: '#2f2752', factor: 0.38, lift: -0.09, amp: 30, freq: 0.0074, seed: 67 }
            ],
            fog: { color: '#6f5bd0', alpha: 0.16 },
            light: { ambient: 0.44, color: '#7ee8fa', radius: 300, player: 1 },
            weather: { kind: 'spore', color: '#a7f3d0', color2: '#c4b5fd', density: 34, speed: 12, size: 2.4 },
            grade: { color: '#150b33', alpha: 0.2 },
            ambience: { primary: 'forest-night', secondary: 'space-hum' }
        },
        {
            id: 'desert',
            sky: ['#3c1e34', '#8a3f45', '#c97141'],
            celestial: { color: '#ffd98a', glow: '#ff9d5c', radius: 58, x: 0.74, y: 0.24, alpha: 0.55 },
            layers: [
                { style: 'dune', color: '#5c3436', factor: 0.06, lift: -0.32, amp: 34, freq: 0.0022, seed: 71 },
                { style: 'dune', color: '#683938', factor: 0.2, lift: -0.18, amp: 30, freq: 0.0041, seed: 83 },
                { style: 'spire', color: '#8c533c', factor: 0.38, lift: -0.07, amp: 28, freq: 0.0069, seed: 97 }
            ],
            fog: { color: '#f0b075', alpha: 0.16 },
            light: { ambient: 0.14, color: '#ffcf8a', radius: 240, player: 0.42 },
            weather: { kind: 'sand', color: '#f5cf87', color2: '#e0a56a', density: 40, speed: 104, size: 2.2 },
            grade: { color: '#4a2018', alpha: 0.14 },
            ambience: { primary: 'wind-strong', secondary: '' }
        },
        {
            id: 'glacier',
            sky: ['#0d2340', '#2a5b7e', '#8fc3d8'],
            celestial: { color: '#f2fbff', glow: '#bfe6ff', radius: 38, x: 0.2, y: 0.17, alpha: 0.44 },
            layers: [
                { style: 'spire', color: '#1d3f5e', factor: 0.07, lift: -0.36, amp: 54, freq: 0.0029, seed: 103 },
                { style: 'ice', color: '#264b6e', factor: 0.19, lift: -0.21, amp: 40, freq: 0.0053, seed: 113 },
                { style: 'ice', color: '#38688f', factor: 0.36, lift: -0.08, amp: 30, freq: 0.0077, seed: 127 }
            ],
            fog: { color: '#d8efff', alpha: 0.18 },
            light: { ambient: 0.16, color: '#d6f1ff', radius: 230, player: 0.5 },
            weather: { kind: 'snow', color: '#e8f7ff', color2: '#ffffff', density: 52, speed: 34, size: 2.4 },
            grade: { color: '#123049', alpha: 0.16 },
            ambience: { primary: 'wind-gentle', secondary: 'stream' }
        },
        {
            id: 'citadel',
            sky: ['#05050b', '#191226', '#3e1e2b'],
            celestial: { color: '#ff9d8a', glow: '#e0453c', radius: 34, x: 0.78, y: 0.18, alpha: 0.4 },
            layers: [
                { style: 'gear', color: '#14121f', factor: 0.08, lift: -0.34, amp: 40, freq: 0.0032, seed: 131 },
                { style: 'ridge', color: '#1f1b2c', factor: 0.21, lift: -0.19, amp: 34, freq: 0.0057, seed: 139 },
                { style: 'gear', color: '#2e2538', factor: 0.38, lift: -0.07, amp: 26, freq: 0.0083, seed: 149 }
            ],
            fog: { color: '#8b3a44', alpha: 0.15 },
            light: { ambient: 0.4, color: '#ffb347', radius: 290, player: 1 },
            weather: { kind: 'ember', color: '#fb7185', color2: '#fbbf24', density: 34, speed: 30, size: 2.6 },
            grade: { color: '#1a0a18', alpha: 0.22 },
            ambience: { primary: 'industrial', secondary: 'engine' }
        }
    ];

    const DEFAULT_BIOME = BIOMES[0];

    function biomeFor(worldIndex) {
        const idx = Number(worldIndex);
        if (!Number.isFinite(idx)) return DEFAULT_BIOME;
        return BIOMES[Math.min(BIOMES.length - 1, Math.max(0, Math.floor(idx)))];
    }

    root.RunenpfadBiomes = {
        BIOMES: BIOMES,
        DEFAULT_BIOME: DEFAULT_BIOME,
        biomeFor: biomeFor
    };
})(typeof window !== 'undefined' ? window : globalThis);
