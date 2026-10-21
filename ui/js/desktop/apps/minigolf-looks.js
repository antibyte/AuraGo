// Per-look rendering data for AuraGolf (P2). Pure data and math without
// three.js, so the scene modules and the Node tests share it.
const deepFreeze = value => {
    if (value && typeof value === 'object' && !Object.isFrozen(value)) { Object.freeze(value); Object.values(value).forEach(deepFreeze); }
    return value;
};

export const LOOK_IDS = Object.freeze(['pirate', 'temple', 'moon']);

// The world id of a manifest v2 course is its look.
export function lookOf(course) {
    return LOOK_IDS.includes(course?.world) ? course.world : 'pirate';
}

export const LOOKS = deepFreeze({
    pirate: {
        exposure: 1.0,
        background: 0x9ad8e6,
        sky: { zenith: 0x3fa7e0, horizon: 0xc4f0f2, below: 0x2a9fb0, sun: 0xfff3c9, clouds: .6, cloudColor: 0xffffff, haze: 0, silhouette: 0, silhouetteColor: 0x000000, stars: 0, earth: false },
        fog: { color: 0xa9e2ea, density: .010 },
        sun: { color: 0xfff1d6, intensity: 2.6, direction: [-.42, .84, .34] },
        rim: null,
        environment: .85,
        bloom: { strength: .14, radius: .4, threshold: .95 },
        water: { mode: 'lagoon', shallow: 0x62e3d6, deep: 0x0f6f93, foam: 0xf5fffb, level: -1.15 },
        fx: { grass: 0x9fd86a, sand: 0xf0d49a, wood: 0x9a6038, stone: 0xb7ab98, wall: 'chips', splash: 0xd4f7f4, ambient: 'glint', ambientColor: 0xfff8d8, gold: 0xffd35a, confetti: [0xd9483b, 0xf2b33d, 0x2ec4c9, 0xf4e9d0, 0x5dbb63] }
    },
    temple: {
        exposure: 1.0,
        background: 0xe8b98a,
        sky: { zenith: 0x3f8f9a, horizon: 0xffc58a, below: 0x2d7f74, sun: 0xffb061, clouds: .25, cloudColor: 0xffd9b0, haze: .55, silhouette: .85, silhouetteColor: 0x24453a, stars: 0, earth: false },
        fog: { color: 0xe7b98c, density: .014 },
        sun: { color: 0xffbf7a, intensity: 2.2, direction: [.78, .36, -.5] },
        rim: null,
        environment: .7,
        bloom: { strength: .22, radius: .5, threshold: .85 },
        water: { mode: 'jungle', shallow: 0x3cc2a8, deep: 0x125f5c, foam: 0xeefcf0, level: -1.15 },
        fx: { grass: 0x7fbf4a, sand: 0xd9b27a, wood: 0x8a5a32, stone: 0xc9b089, wall: 'dust', splash: 0xc7efe2, ambient: 'fireflies', ambientColor: 0xd9ff8a, gold: 0xffc94a, confetti: [0xe8b33a, 0x8cc04a, 0xff8a2a, 0x2aa39a, 0xf4e2c0] }
    },
    moon: {
        exposure: 1.05,
        background: 0x050816,
        sky: { zenith: 0x02040d, horizon: 0x0b1030, below: 0x02040d, sun: 0xcfe0ff, clouds: 0, cloudColor: 0x000000, haze: 0, silhouette: 0, silhouetteColor: 0x000000, stars: 1, earth: true },
        fog: null,
        sun: { color: 0xd6e4ff, intensity: 2.4, direction: [-.86, .42, .28] },
        rim: { color: 0x7fb4ff, intensity: .9, direction: [.6, .35, -.72] },
        environment: .55,
        bloom: { strength: .42, radius: .55, threshold: .75 },
        water: { mode: 'void', level: -1.15 },
        fx: { grass: 0x9a98b0, sand: 0xb7b4c8, wood: 0xffd27a, stone: 0xc0bcd6, wall: 'sparks', splash: 0xc9c6dd, ambient: 'dust', ambientColor: 0xb8c4ff, gold: 0x38f2ff, confetti: [0x38f2ff, 0xff4fd8, 0xff8a3d, 0xe8ecf2, 0x7fb4ff] }
    }
});

export const TIERS = deepFreeze({
    low: { dpr: 1, shadows: 0, bloom: false, clouds: false, twinkle: false, waterMotion: false, glints: false, scatter: false, motion: false, particles: 60, trail: false },
    medium: { dpr: 1.5, shadows: 1024, bloom: true, clouds: true, twinkle: false, waterMotion: true, glints: false, scatter: true, motion: true, particles: 160, trail: true },
    high: { dpr: 2, shadows: 2048, bloom: true, clouds: true, twinkle: true, waterMotion: true, glints: true, scatter: true, motion: true, particles: 320, trail: true }
});

export const tierConfig = tier => TIERS[tier] || TIERS.medium;

// Column-major 4x4 for a manifest placement [name, x, y, z, yaw, scale]; yaw turns about +Y.
export function placementMatrix(entry, out = new Array(16)) {
    const x = entry[1], y = entry[2], z = entry[3], yaw = entry[4] || 0, scale = entry[5] || 1;
    const c = Math.cos(yaw) * scale, s = Math.sin(yaw) * scale;
    out[0] = c; out[1] = 0; out[2] = -s; out[3] = 0;
    out[4] = 0; out[5] = scale; out[6] = 0; out[7] = 0;
    out[8] = s; out[9] = 0; out[10] = c; out[11] = 0;
    out[12] = x; out[13] = y; out[14] = z; out[15] = 1;
    return out;
}
