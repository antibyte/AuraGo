// Ocean chat theme: species table of the sea life layer.
// kind: 'school' (boids, many instances) | 'cruiser' (one animal on a path) | 'drifter' (jelly) | 'rare' (whale)
// depth: z band in metres (negative = away from the camera); counts per quality tier
// length: nose-to-tail extent in metres; the runtime scales every GLB so that its bounding-box depth equals it
// base: 'abyss' = model comes from the Tiefsee screensaver kit (env.abyssBase) instead of env.modelBase
// fog (optional, default 1): multiplies the distance inside the ALPHA fog exp(-(d * 0.052 * fog)^2) only; the colour
//   still mixes toward the water tint by the plain distance, so far animals read as blue-grey silhouettes while the
//   big ones stay visible (alpha at the band centre: whale ~0.5, manta ~0.5, sharks ~0.65, turtle ~0.7, schools ~0.7)
// haze (optional, default 1): scales the colour fog; the whale (0.15) and the manta (0.35) keep most of their dark body (a dark shape, not a water-tinted ghost)
// dive (optional, cruisers): while crossing the reading lane the animal eases to the deep end of its depth band
export const SPECIES = [
    { id: 'fish_sardine', file: 'fish-sardine.glb', kind: 'school', length: 0.15, shader: 'fish', variant: 3,
      counts: { high: 40, medium: 24, low: 16 }, depth: [-9, -13], speed: [0.45, 0.8], swim: { freq: [5, 7], amp: 0.14, k: 2.8 } },
    { id: 'fish_tang', file: 'fish-tang.glb', kind: 'school', length: 0.3, shader: 'fish', variant: 0,
      counts: { high: 14, medium: 10, low: 6 }, depth: [-8, -12], speed: [0.3, 0.55], swim: { freq: [3.5, 5], amp: 0.1, k: 2.4 } },
    { id: 'fish_clown', file: 'fish-clown.glb', kind: 'school', length: 0.12, shader: 'fish', variant: 1,
      counts: { high: 8, medium: 6, low: 4 }, depth: [-7, -10], speed: [0.2, 0.4], swim: { freq: [4, 6], amp: 0.12, k: 2.2 } },
    { id: 'fish_butterfly', file: 'fish-butterfly.glb', kind: 'school', length: 0.2, shader: 'fish', variant: 2,
      counts: { high: 8, medium: 6, low: 4 }, depth: [-8, -11], speed: [0.25, 0.45], swim: { freq: [3, 4.5], amp: 0.09, k: 2.2 } },
    { id: 'shark_reef', file: 'shark-reef.glb', kind: 'cruiser', length: 1.8, shader: 'shark', variant: 0, fog: 0.79, dive: true,
      counts: { high: 1, medium: 1, low: 1 }, depth: [-12, -20], speed: [0.9, 1.3], swim: { freq: [1.6, 2.2], amp: 0.08, k: 1.8 } },
    { id: 'shark_hammer', file: 'shark-hammer.glb', kind: 'cruiser', length: 2.6, shader: 'shark', variant: 1, fog: 0.7, dive: true,
      counts: { high: 1, medium: 0, low: 0 }, depth: [-14, -22], speed: [0.8, 1.1], swim: { freq: [1.4, 1.9], amp: 0.07, k: 1.8 } },
    { id: 'turtle', file: 'turtle.glb', kind: 'cruiser', length: 1.0, shader: 'turtle', variant: 0, fog: 0.9, dive: true,
      counts: { high: 1, medium: 1, low: 0 }, depth: [-9, -16], speed: [0.25, 0.4], swim: { freq: [0.7, 0.9], amp: 0.3, k: 1.0 } },
    { id: 'manta', file: 'manta.glb', base: 'abyss', kind: 'cruiser', length: 3.5, shader: 'manta', variant: 0, fog: 0.6, haze: 0.35,
      counts: { high: 1, medium: 1, low: 0 }, depth: [-22, -32], speed: [0.6, 0.9], swim: { freq: [1.0, 1.3], amp: 0.42, k: 1.4 } },
    { id: 'jelly_moon', file: 'jelly-moon.glb', base: 'abyss', kind: 'drifter', length: 0.35, shader: 'jelly', variant: 0, fog: 1.1,
      counts: { high: 5, medium: 3, low: 2 }, depth: [-7, -18], speed: [0.08, 0.16], swim: { freq: [0.32, 0.5], amp: 1, k: 1 } },
    { id: 'whale', file: 'whale.glb', kind: 'rare', length: 12, shader: 'whale', variant: 0, fog: 0.42, haze: 0.15,
      counts: { high: 1, medium: 1, low: 0 }, depth: [-36, -36], speed: [1.6, 1.6], swim: { freq: [0.35, 0.35], amp: 0.6, k: 1.2 } }
];
export const QUALITY_TIERS = { high: 1.2e6, medium: 0.5e6 }; // canvas pixels (after DPR) needed for the tier
