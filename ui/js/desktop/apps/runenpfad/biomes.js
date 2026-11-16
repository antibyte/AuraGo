/* Runenpfad per-biome presentation model: sky, parallax layers, lighting, weather, ambience. DOM-free. */
(function (root) {
    'use strict';

    // Layer fields (consumed by atmosphere.js, baked once into 2048 px periodic strips):
    //   style  prop painter  (peaks/castle/pines/oaks/trunks/crystals/rocks/mushrooms/ceiling/mesas/ruins/cacti/
    //                         snowpines/icespires/towers/chimneys/chains/spires/none)
    //   base   crest shape   (peaks/hills/dunes/flat)       color  silhouette colour (far layers sit near the sky colour)
    //   factor parallax      lift   baseline offset (fraction of view height, negative = above ground line)
    //   span   strip height  amp    crest amplitude px      cells  periodic noise cells per 2048 px
    //   anchor 'top' hangs from the screen top (cave ceilings)
    // Each biome drives the backdrop painter, the light map, the weather layer and the ambience mixer.
    const BIOMES = [
        {
            id: 'woodland',
            sky: ['#0b2438', '#1b4557', '#3c7a69'],
            celestial: { color: '#fff3c4', glow: '#ffd98a', radius: 42, x: 0.82, y: 0.2, alpha: 0.55, craters: true, rays: true },
            layers: [
                { style: 'peaks', base: 'peaks', color: '#2d5f68', factor: 0.04, lift: -0.3, span: 330, amp: 260, cells: 5, seed: 11, facets: true, mist: 0.5 },
                { style: 'castle', base: 'hills', color: '#27565c', factor: 0.08, lift: -0.22, span: 230, amp: 70, cells: 6, seed: 23, mist: 0.4 },
                { style: 'pines', base: 'hills', color: '#1f4b4d', factor: 0.15, lift: -0.145, span: 210, amp: 34, cells: 9, seed: 37, mist: 0.3 },
                { style: 'oaks', base: 'hills', color: '#1b4443', factor: 0.27, lift: -0.07, span: 240, amp: 24, cells: 11, seed: 47, mist: 0.25 },
                { style: 'trunks', base: 'flat', color: '#11302f', factor: 0.45, lift: -0.01, span: 360, amp: 8, cells: 13, seed: 59, mist: 0.15 }
            ],
            shafts: { color: '#ffe7a8', alpha: 0.11, factor: 0.12, count: 7 },
            stars: { count: 40, alpha: 0.5, color: '#e8f6ff' },
            clouds: { color: '#a9d8d0', alpha: 0.16, speed: 6, count: 8, y: 0.2, scale: 1 },
            foreground: { style: 'grass', color: '#04140f', alpha: 0.7, factor: 1.25 },
            surface: { grass: { color: '#6ec24f', color2: '#b9f27c', density: 0.55 } },
            fog: { color: '#8fd8c6', alpha: 0.12 },
            light: { ambient: 0.08, color: '#ffe9b0', radius: 210, player: 0.5 },
            weather: { kind: 'leaf', color: '#bef264', color2: '#86efac', density: 30, speed: 26, size: 2.8 },
            grade: { color: '#0d2a26', alpha: 0.1 },
            ambience: { primary: 'forest-day', secondary: 'stream' }
        },
        {
            id: 'cavern',
            sky: ['#070512', '#171029', '#2d234a'],
            celestial: { color: '#9be7ff', glow: '#3ba7c9', radius: 26, x: 0.26, y: 0.2, alpha: 0.3, rays: false },
            layers: [
                { style: 'ceiling', anchor: 'top', color: '#44387f', factor: 0.1, span: 320, seed: 41, len: [90, 270], mist: 0.35 },
                { style: 'crystals', base: 'peaks', color: '#1c1435', factor: 0.07, lift: -0.3, span: 300, amp: 150, cells: 6, seed: 53, facets: true, mist: 0.35, palette: ['#7ee8fa', '#a78bfa', '#f0abfc'] },
                { style: 'rocks', base: 'hills', color: '#251b43', factor: 0.2, lift: -0.17, span: 240, amp: 60, cells: 8, seed: 67, mist: 0.25 },
                { style: 'mushrooms', base: 'hills', color: '#2a2050', factor: 0.35, lift: -0.06, span: 200, amp: 26, cells: 10, seed: 79, mist: 0.2, palette: ['#5eead4', '#c4b5fd', '#7dd3fc'] },
                { style: 'ceiling', anchor: 'top', color: '#04030c', factor: 0.42, span: 300, seed: 89, len: [60, 250], drips: true }
            ],
            clouds: { color: '#6f5bd0', alpha: 0.1, speed: 4, count: 6, y: 0.34, scale: 1.3 },
            foreground: { style: 'fungi', color: '#05030d', alpha: 0.75, factor: 1.25 },
            surface: { grass: { color: '#4fd1c5', color2: '#a7f3d0', density: 0.4 } },
            fog: { color: '#6f5bd0', alpha: 0.16 },
            light: { ambient: 0.44, color: '#7ee8fa', radius: 300, player: 1 },
            weather: { kind: 'spore', color: '#a7f3d0', color2: '#c4b5fd', density: 34, speed: 12, size: 2.4 },
            grade: { color: '#150b33', alpha: 0.2 },
            ambience: { primary: 'forest-night', secondary: 'space-hum' }
        },
        {
            id: 'desert',
            sky: ['#3c1e34', '#8a3f45', '#c97141'],
            celestial: { color: '#ffe3a1', glow: '#ff9d5c', radius: 62, x: 0.74, y: 0.26, alpha: 0.7, rays: true, halo: 5 },
            layers: [
                { style: 'mesas', base: 'flat', color: '#a65b49', factor: 0.04, lift: -0.26, span: 260, amp: 8, cells: 6, seed: 71, mist: 0.5 },
                { style: 'ruins', base: 'dunes', color: '#8d4f45', factor: 0.09, lift: -0.2, span: 270, amp: 58, cells: 4, seed: 83, mist: 0.4 },
                { style: 'none', base: 'dunes', color: '#7a4140', factor: 0.17, lift: -0.13, span: 150, amp: 70, cells: 5, seed: 97, mist: 0.3 },
                { style: 'cacti', base: 'dunes', color: '#64343a', factor: 0.3, lift: -0.07, span: 200, amp: 50, cells: 7, seed: 107, mist: 0.2 },
                { style: 'rocks', base: 'hills', color: '#4a2832', factor: 0.46, lift: -0.012, span: 220, amp: 16, cells: 12, seed: 119, mist: 0.1, warm: true }
            ],
            clouds: { color: '#ffc08a', alpha: 0.17, speed: 9, count: 6, y: 0.28, scale: 1.4 },
            foreground: { style: 'dry', color: '#1a0a0c', alpha: 0.68, factor: 1.25 },
            surface: { grass: { color: '#d9b45a', color2: '#f3d98a', density: 0.4 } },
            fog: { color: '#f0b075', alpha: 0.16 },
            light: { ambient: 0.14, color: '#ffcf8a', radius: 240, player: 0.42 },
            weather: { kind: 'sand', color: '#f5cf87', color2: '#e0a56a', density: 40, speed: 104, size: 2.2 },
            grade: { color: '#4a2018', alpha: 0.14 },
            ambience: { primary: 'wind-strong', secondary: '' }
        },
        {
            id: 'glacier',
            sky: ['#0d2340', '#2a5b7e', '#8fc3d8'],
            celestial: { color: '#f2fbff', glow: '#bfe6ff', radius: 38, x: 0.2, y: 0.17, alpha: 0.5, craters: true, rays: true },
            layers: [
                { style: 'none', base: 'peaks', snow: true, falls: true, color: '#2b5578', factor: 0.04, lift: -0.3, span: 340, amp: 270, cells: 5, seed: 103, facets: true, mist: 0.5 },
                { style: 'none', base: 'peaks', snow: true, color: '#2a4f72', factor: 0.09, lift: -0.22, span: 260, amp: 150, cells: 7, seed: 113, facets: true, mist: 0.4 },
                { style: 'snowpines', base: 'hills', color: '#27506f', factor: 0.17, lift: -0.14, span: 220, amp: 40, cells: 9, seed: 127, mist: 0.3 },
                { style: 'snowpines', base: 'hills', color: '#1f4563', factor: 0.3, lift: -0.07, span: 240, amp: 26, cells: 11, seed: 137, mist: 0.2, big: true },
                { style: 'icespires', base: 'flat', color: '#17385a', factor: 0.46, lift: -0.012, span: 280, amp: 10, cells: 13, seed: 149, mist: 0.1 }
            ],
            stars: { count: 95, alpha: 0.7, color: '#ffffff' },
            aurora: { colors: ['#10e08a', '#12d6c8', '#9b6bff'], alpha: 0.4 },
            clouds: { color: '#d8efff', alpha: 0.14, speed: 5, count: 7, y: 0.3, scale: 1.2 },
            foreground: { style: 'ice', color: '#06182a', alpha: 0.7, factor: 1.25 },
            surface: { grass: { color: '#cfe9ff', color2: '#ffffff', density: 0.35 } },
            fog: { color: '#d8efff', alpha: 0.18 },
            light: { ambient: 0.16, color: '#d6f1ff', radius: 230, player: 0.5 },
            weather: { kind: 'snow', color: '#e8f7ff', color2: '#ffffff', density: 52, speed: 34, size: 2.4 },
            grade: { color: '#123049', alpha: 0.16 },
            ambience: { primary: 'wind-gentle', secondary: 'stream' }
        },
        {
            id: 'citadel',
            sky: ['#05050b', '#191226', '#4a2030'],
            celestial: { color: '#ff9d8a', glow: '#e0453c', radius: 34, x: 0.78, y: 0.18, alpha: 0.5, rays: true, craters: true },
            layers: [
                { style: 'towers', base: 'hills', color: '#2a1d2f', factor: 0.05, lift: -0.26, span: 330, amp: 60, cells: 6, seed: 131, gears: true, mist: 0.5 },
                { style: 'chimneys', base: 'hills', color: '#241b33', factor: 0.1, lift: -0.19, span: 270, amp: 50, cells: 8, seed: 139, smoke: true, mist: 0.4 },
                { style: 'chains', base: 'hills', color: '#1f1830', factor: 0.2, lift: -0.12, span: 270, amp: 40, cells: 9, seed: 149, mist: 0.3 },
                { style: 'towers', base: 'hills', color: '#19132a', factor: 0.33, lift: -0.06, span: 300, amp: 26, cells: 11, seed: 157, windows: true, mist: 0.2 },
                { style: 'spires', base: 'flat', color: '#0f0b19', factor: 0.48, lift: -0.01, span: 340, amp: 10, cells: 13, seed: 163, mist: 0.1 }
            ],
            stars: { count: 34, alpha: 0.4, color: '#ffb4a8' },
            clouds: { color: '#8b2f3d', alpha: 0.2, speed: 7, count: 7, y: 0.26, scale: 1.4 },
            foreground: { style: 'iron', color: '#07040d', alpha: 0.78, factor: 1.25 },
            surface: { grass: { color: '#7c5a63', color2: '#a1707a', density: 0.25 }, lava: { color: '#ff8a3d', glow: 'rgba(255, 110, 40, 0.6)' } },
            fog: { color: '#8b3a44', alpha: 0.15 },
            light: { ambient: 0.4, color: '#ffb347', radius: 290, player: 1 },
            weather: { kind: 'ember', color: '#fb7185', color2: '#fbbf24', density: 34, speed: 30, size: 2.6 },
            grade: { color: '#1a0a18', alpha: 0.22 },
            ambience: { primary: 'industrial', secondary: 'engine' }
        }
    ];

    // Used by the Storm rune trial (stage.weather === 'rain').
    const RAIN_WEATHER = { kind: 'rain', color: '#b9d6f0', color2: '#e0f0ff', density: 150, speed: 880, size: 1.2 };

    const DEFAULT_BIOME = BIOMES[0];

    function biomeFor(worldIndex) {
        const idx = Number(worldIndex);
        if (!Number.isFinite(idx)) return DEFAULT_BIOME;
        return BIOMES[Math.min(BIOMES.length - 1, Math.max(0, Math.floor(idx)))];
    }

    root.RunenpfadBiomes = {
        BIOMES: BIOMES,
        DEFAULT_BIOME: DEFAULT_BIOME,
        RAIN_WEATHER: RAIN_WEATHER,
        biomeFor: biomeFor
    };
})(typeof window !== 'undefined' ? window : globalThis);
