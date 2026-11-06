/* Aura Drift - per-biome look, terrain rules and scatter tables (data only, no THREE, no DOM).
 *
 * LOOK[biome]   light, sky, fog and grade for the 3D renderer (render3d, sky, road, terrain).
 *   sunEl/sunAz  degrees; the light comes FROM that direction (az 0 = +x, 90 = +z in three space)
 *   sun/skyTop/skyHorizon/fog  hex colours; fogFar metres; fogNear is a fraction of fogFar
 *   hemiSky/hemiGround/hemi    ambient dome light colours and intensity
 *   sunPower     directional light intensity; env = image based light intensity
 *   exposure/bloom/grade       tone mapping exposure, bloom strength, named grade tint
 *   ground       colour of the flat stand-in ground and the far terrain blend
 *   terrain      vertex colour palette { low, mid, high, rock } for T3.3
 *   headlights   headlamp emission for cars (night biomes burn bright)
 *   wet          base road wetness 0..1
 *   glow         optional multiplier on the emissive intensity of prop windows and lamps (default 1.5 at night, 1.1 by day)
 *   roadLift     base road albedo multiplier, roadCoat clearcoat multiplier (both optional, default 1)
 * TERRAIN[biome] height rules (metres), interpreted by terrain.js.
 * SCATTER[biome] prop name -> { min, max, rate } : band from the road edge, count per 100 m per side. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;

    function look(o) {
        return Object.assign({ fogNear: 0.18, sunAz: 205, hemi: 1.0, sunPower: 3.0, env: 1.0, wet: 0, night: false, headlights: 0.6 }, o);
    }

    AD.LOOK = {
        alpine: look({
            time: 'golden morning', sunEl: 24, sunAz: 212, sun: '#ffd9a0', skyTop: '#5c8fd6', skyHorizon: '#ffd7ad', fog: '#d9c8b4', fogFar: 900,
            exposure: 1.05, bloom: 0.35, grade: 'warm', hemiSky: '#a9c8f0', hemiGround: '#6d5a44', hemi: 1.1, sunPower: 3.4, ground: '#4f6a45',
            terrain: { low: '#4c6a42', mid: '#6b8a52', high: '#a8a58f', rock: '#7e7a72' }
        }),
        desert: look({
            time: 'late afternoon', sunEl: 18, sunAz: 150, sun: '#ffb26b', skyTop: '#4a7bc4', skyHorizon: '#ffb07a', fog: '#e8b98a', fogFar: 1100,
            exposure: 1.1, bloom: 0.35, grade: 'orange', hemiSky: '#9cb8e8', hemiGround: '#a8703c', hemi: 1.1, sunPower: 3.6, ground: '#c28a4c',
            terrain: { low: '#c58d4e', mid: '#d9a661', high: '#b9703f', rock: '#9a5a36' }
        }),
        harbour: look({
            time: 'blue hour neon', sunEl: 6, sunAz: 250, sun: '#8fb4ff', skyTop: '#141c44', skyHorizon: '#d36a7a', fog: '#2a3050', fogFar: 650,
            exposure: 1.0, bloom: 0.8, grade: 'teal-magenta', hemiSky: '#4a5a9c', hemiGround: '#2a2438', hemi: 1.3, sunPower: 2.4, env: 1.1,
            ground: '#252d3d', night: true, headlights: 1.6, wet: 0.7, roadCoat: 0.3,
            terrain: { low: '#222c3c', mid: '#2c3a50', high: '#3d4c62', rock: '#4a5568' }
        }),
        salt: look({
            time: 'low white sun', sunEl: 14, sunAz: 170, sun: '#fff0d8', skyTop: '#6aa6e8', skyHorizon: '#ffd0c8', fog: '#f0e4dc', fogFar: 1400,
            exposure: 1.15, bloom: 0.45, grade: 'pastel', hemiSky: '#b6d4f4', hemiGround: '#e8dcd2', hemi: 1.3, sunPower: 3.2, ground: '#e0ddd2',
            terrain: { low: '#dcd9cd', mid: '#e9e5d9', high: '#f3efe4', rock: '#b9a7a0' }
        }),
        forest: look({
            time: 'dawn mist', sunEl: 20, sunAz: 235, sun: '#ffd28a', skyTop: '#7aa890', skyHorizon: '#f4d9a8', fog: '#b8c8a4', fogFar: 420, fogNear: 0.08,
            exposure: 1.0, bloom: 0.3, grade: 'green-gold', hemiSky: '#a8c8a8', hemiGround: '#4a4a2a', hemi: 1.2, sunPower: 3.0, ground: '#34522f', wet: 0.25,
            terrain: { low: '#2f4a2a', mid: '#46683a', high: '#7a8450', rock: '#6b6556' }
        }),
        industry: look({
            time: 'sulphur dusk', sunEl: 8, sunAz: 195, sun: '#ff9a55', skyTop: '#2c2a3c', skyHorizon: '#d9703c', fog: '#55413a', fogFar: 600,
            exposure: 1.0, bloom: 0.7, grade: 'sodium', hemiSky: '#5a4a5a', hemiGround: '#3a2a22', hemi: 1.2, sunPower: 2.4, ground: '#3a3d41',
            night: true, headlights: 1.3,
            terrain: { low: '#383b3e', mid: '#46494d', high: '#5a5d60', rock: '#63564d' }
        }),
        glacier: look({
            time: 'polar night, aurora', sunEl: 10, sunAz: 225, sun: '#a8d0ff', skyTop: '#0a1630', skyHorizon: '#3a6a8a', fog: '#203248', fogFar: 750,
            exposure: 1.05, bloom: 0.7, grade: 'ice', hemiSky: '#5a86c0', hemiGround: '#2a3a52', hemi: 1.4, sunPower: 2.5, env: 1.1, ground: '#4a5c72',
            night: true, headlights: 1.5,
            terrain: { low: '#48607a', mid: '#8fb0cc', high: '#dcecf8', rock: '#5c6f86' }
        }),
        metro: look({
            time: 'midnight neon', sunEl: 4, sunAz: 270, sun: '#7f8cff', skyTop: '#0c0a22', skyHorizon: '#5a2a7a', fog: '#1a1436', fogFar: 520,
            exposure: 1.0, bloom: 0.95, grade: 'magenta', hemiSky: '#4a3a8a', hemiGround: '#241a38', hemi: 1.4, sunPower: 2.2, env: 1.1, ground: '#2a2d36',
            night: true, headlights: 1.8, wet: 0.6, roadLift: 1.45, glow: 0.72,
            terrain: { low: '#262a32', mid: '#31353f', high: '#444956', rock: '#3a3a48' }
        })
    };

    /* Height rules; every amplitude and wavelength is in metres. */
    AD.TERRAIN = {
        alpine: { kind: 'hills', amp: 90, wave: 900, ridged: { amp: 220, from: 400 }, flat: true },
        desert: { kind: 'dunes', amp: 18, wave: 240, mesas: { from: 180, quant: 30, amp: 90 } },
        harbour: { kind: 'flat', base: -0.4, water: { depth: -3, side: 'outer' } },
        salt: { kind: 'flat', base: -0.3, ring: { from: 900, amp: 120 } },
        forest: { kind: 'rolling', amp: 28, wave: 500 },
        industry: { kind: 'embankments', amp: 6, heaps: { from: 200, amp: 30 } },
        glacier: { kind: 'ridged', amp: 70, wave: 520 },
        metro: { kind: 'flat', base: -0.2 }
    };

    function band(min, max, rate) { return { min: min, max: max, rate: rate }; }
    AD.SCATTER = {
        alpine: { pine_a: band(10, 160, 3), pine_b: band(14, 180, 2), rock_a: band(8, 120, 1.2), rock_b: band(20, 200, 0.8), flower_meadow: band(8, 95, 1.4), chalet: band(40, 180, 0.08), hay_barn: band(28, 160, 0.09), cablecar_pylon: band(80, 260, 0.05) },
        desert: { cactus_a: band(8, 140, 1.6), cactus_b: band(12, 160, 1), mesa_rock: band(60, 400, 0.2), rock_arch: band(45, 320, 0.14), palm_oasis: band(26, 170, 0.1), water_tower: band(40, 200, 0.05), windmill_pump: band(30, 200, 0.08) },
        harbour: { container_stack: band(12, 140, 0.8), crane: band(60, 240, 0.08), lighthouse: band(200, 300, 0.02), fishing_boat: band(40, 200, 0.15), tugboat: band(36, 190, 0.12), buoy_cluster: band(18, 140, 0.35), neon_sign: band(8, 60, 0.2) },
        salt: { salt_crystal: band(8, 200, 1.2), geyser_pool: band(22, 160, 0.12), radar_dome: band(45, 240, 0.06), rover: band(30, 200, 0.03), flag: band(15, 100, 0.1) },
        forest: { oak: band(8, 120, 3), birch: band(10, 150, 2.5), fern_glade: band(7, 85, 1.6), mushroom_cluster: band(6, 40, 1), log_pile: band(10, 70, 0.2), hunting_hide: band(20, 80, 0.06), watermill: band(26, 130, 0.08) },
        industry: { silo: band(20, 200, 0.2), smokestack: band(60, 300, 0.1), cooling_tower: band(55, 280, 0.09), chem_tank: band(18, 130, 0.16), pipes: band(10, 80, 0.4), forklift: band(12, 60, 0.1), pylon: band(40, 260, 0.1) },
        glacier: { ice_block: band(8, 120, 1), ice_spire: band(12, 150, 0.85), ice_arch: band(30, 160, 0.1), snow_pine: band(10, 150, 1.8), igloo: band(20, 120, 0.06), snow_cat: band(16, 95, 0.09), research_station: band(80, 260, 0.03) },
        metro: {
            skyscraper_a: band(16, 90, 1.2), skyscraper_b: band(16, 90, 1), skyscraper_c: band(30, 160, 1), billboard: band(8, 30, 0.2),
            street_lamp: band(6, 8, 1), food_truck: band(8, 30, 0.1), taxi: band(8, 30, 0.2), metro_entrance: band(10, 42, 0.14), arcade_kiosk: band(9, 36, 0.16)
        }
    };

    AD.ANIMAL_FOR_BIOME = {
        alpine: 'cow', desert: 'llama', harbour: 'crab', salt: 'cow',
        forest: 'sheep', industry: 'sheep', glacier: 'penguin', metro: 'llama'
    };

    /* Scene light direction (towards the sun) in three space for a biome. */
    AD.sunDirection = function (look, minEl) {
        const el = Math.max(minEl || 0, look.sunEl) * Math.PI / 180, az = look.sunAz * Math.PI / 180;
        return [Math.cos(el) * Math.cos(az), Math.sin(el), Math.cos(el) * Math.sin(az)];
    };
})();
