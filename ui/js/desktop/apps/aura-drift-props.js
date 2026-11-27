/* Aura Drift - 3D props: deterministic scatter and trackside furniture, instanced streaming, brand
 * hoardings, silly road signs, the start gantry and the salt flat UFO.
 *
 * Pure part (node tested): AD.scatterProps, AD.furnitureProps, AD.propIndex, AD.filterProps.
 * Three part: AD.createProps(T, scene, track, biomeId, q, t) -> { update(pose, race), rebuild(), setQuality(q), dispose(), stats() }.
 * Placements are in sim coordinates (x, y); three space is [x, up, y_sim]. A placement is
 * { kind, name, biome, x, y, yaw, sx, sy, sz, tint, r, h, reach, ... }; yaw is the three-space Y rotation. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const SEG = 100;
    const CELL = 128;
    const DEF_META = { radius: 3, height: 6, tags: [] };
    const NATURAL = /^(pine|snow_pine|oak|birch|rock|cactus|salt_crystal|ice_block|ice_spire|mushroom|log_pile|mesa|flower_meadow|fern_glade|palm_oasis|geyser_pool)/;
    const STRUCT_SET = new Set(['chalet', 'hay_barn', 'water_tower', 'windmill_pump', 'sunshade', 'container_stack', 'crane', 'radar_dome', 'rover', 'watermill', 'hunting_hide', 'silo', 'smokestack', 'cooling_tower', 'chem_tank', 'pipes', 'forklift', 'igloo', 'snow_cat', 'research_station', 'snow_fence', 'skyscraper_a', 'skyscraper_b', 'skyscraper_c', 'food_truck', 'taxi', 'metro_entrance', 'arcade_kiosk', 'neon_sign', 'storm_lantern', 'signal_mast', 'holo_arrow']);
    const FULL_SEP = new Set(['mesa_rock', 'rock_arch', 'ice_arch', 'chalet', 'hay_barn', 'palm_oasis', 'water_tower', 'windmill_pump', 'container_stack', 'crane', 'silo', 'cooling_tower', 'smokestack', 'research_station', 'igloo', 'skyscraper_a', 'skyscraper_b', 'skyscraper_c', 'radar_dome', 'watermill', 'lighthouse']);
    const HUG_SET = new Set(['flower_meadow', 'fern_glade', 'mushroom_cluster', 'log_pile', 'salt_crystal', 'rock_a', 'ice_block', 'geyser_pool']);
    const ARCH_ROCK = new Set(['rock_arch', 'ice_arch', 'mesa_rock', 'palm_oasis', 'geyser_pool', 'cablecar_pylon', 'pylon']);
    const WATER_DRAFT = { tugboat: -0.26, fishing_boat: -0.88, buoy_cluster: -0.06, lighthouse: -0.35 };

    function band(min, max, rate, scale) { return { min: min, max: max, rate: rate, scale: scale || 1 }; }
    /* Extra far silhouettes beyond the SCATTER bands: tall heroes on the horizon (drawn out to 1.8 x propRadius). */
    const BACKDROP = {
        alpine: { pine_a: band(170, 460, 1.4, 1.7), rock_b: band(220, 520, 0.7, 2.6), hay_barn: band(180, 420, 0.06, 1.5), cablecar_pylon: band(300, 700, 0.05, 1.4) },
        desert: { mesa_rock: band(280, 900, 0.3, 2.4), rock_arch: band(220, 760, 0.18, 2.1), palm_oasis: band(180, 460, 0.06, 1.6), windmill_pump: band(160, 500, 0.03, 1.4) },
        harbour: { crane: band(220, 600, 0.1, 1.4), lighthouse: band(300, 800, 0.02, 1.5), container_stack: band(150, 420, 0.3, 1.6), tugboat: band(160, 480, 0.08, 1.4) },
        salt: { salt_crystal: band(220, 700, 0.5, 3.0), radar_dome: band(220, 680, 0.06, 1.8), flag: band(150, 500, 0.04, 2.0) },
        forest: { oak: band(130, 300, 1.8, 1.6), birch: band(140, 320, 1.2, 1.5), watermill: band(150, 340, 0.05, 1.5) },
        industry: { smokestack: band(280, 800, 0.14, 1.5), cooling_tower: band(240, 760, 0.12, 1.6), silo: band(200, 520, 0.2, 1.6), pylon: band(260, 800, 0.12, 1.5) },
        glacier: { ice_arch: band(200, 520, 0.06, 2.2), ice_spire: band(180, 520, 0.35, 2.0), snow_pine: band(150, 420, 1.0, 1.6), research_station: band(320, 640, 0.02, 1.6) },
        metro: { skyscraper_a: band(150, 520, 1.3, 1.5), skyscraper_b: band(150, 520, 1.1, 1.5), skyscraper_c: band(170, 560, 1.1, 1.5) }
    };
    /* Glowing things the water and the salt mirror reflect as vertical streaks: name -> [colour, height above ground]. */
    const EMIT = {
        neon_sign: ['#ff4fa3', 9], crane: ['#ff6fb4', 19], lighthouse: ['#ffcf7a', 18], fishing_boat: ['#ffcf7a', 6], tugboat: ['#7af4ff', 6.5], buoy_cluster: ['#ffcf7a', 4.5],
        ufo: ['#7dffb0', 40], rover: ['#fff2c0', 3], radar_dome: ['#7af4ff', 9], cooling_tower: ['#ff5a52', 19], metro_entrance: ['#ff7ac0', 4.6], arcade_kiosk: ['#5ff4ff', 5.2],
        signal_mast: ['#ff6fb4', 6.4], holo_arrow: ['#5ff4ff', 4.2]
    };
    const WATER_ONLY = { fishing_boat: true, tugboat: true, buoy_cluster: true };
    /* The water surface of the harbour. It is the height of the water plane in the
       sky module, so a boat floats on the sea instead of hovering above it. */
    const WATER_Y = typeof AD.HARBOUR_WATER_Y === 'number' ? AD.HARBOUR_WATER_Y : -2.2;
    /* The SCATTER rates keep their proportions; this lifts them so each biome reads dense and framed. */
    const DENSITY = { alpine: 2.2, desert: 3.2, harbour: 3.2, salt: 3.4, forest: 1.8, industry: 4, glacier: 2.0, metro: 1.3 };

    /* ---- deterministic helpers ----------------------------------------------------- */
    function ihash(a, b) {
        let h = Math.imul(a ^ 0x9e3779b9, 0x85ebca6b) ^ Math.imul((b | 0) + 0x7f4a7c15, 0xc2b2ae35);
        h ^= h >>> 15; h = Math.imul(h, 0x2c1b3c6d); h ^= h >>> 12;
        return (h >>> 0) / 4294967296;
    }
    function vnoise(x, seed) {
        const i = Math.floor(x), f = x - i, t = f * f * (3 - 2 * f);
        return ihash(i, seed) * (1 - t) + ihash(i + 1, seed) * t;
    }
    function wrapDist(a, b, length) { const d = Math.abs(a - b) % length; return Math.min(d, length - d); }

    const featureCache = new WeakMap();
    function featuresFor(track) {
        if (featureCache.has(track)) return featureCache.get(track);
        let fs = null;
        try { if (typeof AD.createFeatures === 'function') fs = AD.createFeatures(track, track.seed, { crates: false }); } catch (_) { fs = null; }
        featureCache.set(track, fs);
        return fs;
    }
    const centres = new WeakMap();
    function centroid(track) {
        let c = centres.get(track);
        if (c) return c;
        let cx = 0, cy = 0;
        for (const s of track.samples) { cx += s.x; cy += s.y; }
        c = { x: cx / track.samples.length, y: cy / track.samples.length };
        centres.set(track, c);
        return c;
    }
    /* +1 when the point lies on the inner side (towards the middle of the loop) of its nearest sample. */
    function innerSign(track, x, y, smp) {
        const c = centroid(track);
        return Math.sign((c.x - x) * smp.nx + (c.y - y) * smp.ny) || 1;
    }
    function yawToward(dx, dy) { return Math.atan2(-dy, dx); }
    function slopeOk(track, biomeId, x, y, radius, name, back) {
        if (!AD.terrainSlopeAt) return true;
        const s = AD.terrainSlopeAt(track, biomeId, x, y, Math.max(2.5, radius * 0.65));
        if (back) return s.slope <= 0.50;
        if (STRUCT_SET.has(name)) return s.slope <= 0.24 && s.span <= 2.4;
        if (ARCH_ROCK.has(name)) return s.slope <= 0.36;
        if (/^(pine|snow_pine|oak|birch|cactus)/.test(name)) return s.slope <= 0.55;
        return s.slope <= 0.60 && s.span <= 4.5;
    }

    /* ---- scatter ---------------------------------------------------------------------- */
    AD.scatterProps = function (track, biomeId, meta) {
        const table = AD.SCATTER && AD.SCATTER[biomeId];
        if (!table) return [];
        meta = meta || {};
        const rng = AD.rng(AD.hashString(track.id + '/props/' + track.seed));
        const entries = [];
        for (const name of Object.keys(table)) { const b = table[name]; entries.push({ name: name, min: b.min, max: b.max, rate: b.rate, scale: 1, back: false }); }
        const back = BACKDROP[biomeId] || {};
        for (const name of Object.keys(back)) { const b = back[name]; entries.push({ name: name, min: b.min, max: b.max, rate: b.rate, scale: b.scale, back: true }); }
        const furniture = AD.furnitureProps(track).concat(typeof AD.petDecorPlacements === 'function' ? AD.petDecorPlacements(track) : []);
        if (AD.roadsideSites) furniture.push(...AD.roadsideSites(track));
        const reservedSpaces = furniture.filter(p => p.folder || p.stage);
        const cells = new Map();
        const cellKey = (x, y) => Math.floor(x / 16) + ',' + Math.floor(y / 16);
        for (const f of furniture) { const k = cellKey(f.x, f.y); (cells.get(k) || cells.set(k, []).get(k)).push(f); }
        const out = [], n = track.samples.length, segs = Math.ceil(track.length / SEG);

        function tryPlace(name, s, side, dist, x, y, opts) {
            const o = opts || {};
            const m = meta[name] || DEF_META, tags = m.tags || [];
            const backFlag = !!o.back, sc0 = o.scale || 1;
            const smp = AD.sampleTrack(track, x, y);
            const clear = clamp(o.minDist || (table[name] ? table[name].min : 8), 6, 8);
            const dRoad = Math.hypot(x - smp.x, y - smp.y);
            if (dRoad < smp.halfWidth + clear) return null;
            if (biomeId === 'harbour') {
                const hSide = AD.propSide(track, biomeId, x, y);
                const wantWater = !!(WATER_ONLY[name] || name === 'lighthouse');
                if (wantWater !== (hSide === 'water')) return null;
            }
            const height = (m.height || 6) * sc0, radius = (m.radius || 3) * sc0;
            if (!backFlag && height > 3 && dRoad < smp.halfWidth + 18 && track.curvature[smp.index] > 0.02) {
                const a = track.samples[smp.index], b = track.samples[(smp.index + 4) % n];
                const cross = a.tx * b.ty - a.ty * b.tx;
                if ((cross > 0 ? 1 : -1) === innerSign(track, x, y, smp)) return null;
            }
            if (!slopeOk(track, biomeId, x, y, radius, name, backFlag)) return null;
            const natural = NATURAL.test(name) || (tags.indexOf('scatter') >= 0 && !backFlag && name !== 'street_lamp' && name !== 'road_sign');
            const sep = FULL_SEP.has(name) ? 0.96 : (natural ? 0.52 : 0.85);
            const cx = Math.floor(x / 16), cy = Math.floor(y / 16), reachCells = Math.ceil((radius + 18) / 16);
            for (let ox = -reachCells; ox <= reachCells; ox++) {
                for (let oy = -reachCells; oy <= reachCells; oy++) {
                    const list = cells.get((cx + ox) + ',' + (cy + oy));
                    if (!list) continue;
                    for (const prev of list) {
                        const lim = prev.furniture ? prev.r + radius * 0.6 + 3 : (prev.r + radius) * (FULL_SEP.has(prev.name) ? Math.max(sep, 0.95) : sep);
                        if (Math.hypot(prev.x - x, prev.y - y) < lim) return null;
                    }
                }
            }
            const sc = o.fixedScale || (natural ? (0.8 + (o.scR ?? 0.4) * 0.7) * sc0 : (0.94 + (o.scR ?? 0.3) * 0.22) * sc0);
            const rFinal = (m.radius || 3) * sc;
            if (reservedSpaces.some(rp => Math.hypot(rp.x - x, rp.y - y) < rp.r + rFinal + 2)) return null;
            const toRoad = [-side * smp.nx, -side * smp.ny];
            const aligned = !natural && (biomeId === 'metro' || biomeId === 'harbour' || biomeId === 'industry');
            const yaw = o.yaw !== undefined ? o.yaw : (natural ? (o.yawR ?? 0.5) * Math.PI * 2 : yawToward(toRoad[0], toRoad[1]) + ((o.yawR ?? 0.5) - 0.5) * (aligned ? 0.08 : 0.44));
            const pl = {
                kind: 'scatter', name: name, biome: biomeId, s: s, side: side, dist: dist, x: x, y: y, yaw: yaw, sx: sc, sy: sc, sz: sc,
                tint: o.tint ?? 0.5, r: rFinal, h: (m.height || 6) * sc, back: backFlag,
                reach: backFlag || tags.indexOf('far') >= 0 ? 1.8 : (tags.indexOf('scatter') >= 0 ? 0.7 : 1),
                tags: tags.indexOf('hero') >= 0 ? 'hero' : ''
            };
            if (o.chain !== undefined) { pl.chain = o.chain; pl.chainIdx = o.chainIdx; }
            out.push(pl);
            (cells.get(cellKey(x, y)) || cells.set(cellKey(x, y), []).get(cellKey(x, y))).push(pl);
            return pl;
        }

        /* Curated architectural colonnades & landmark chains before filling natural scatter. */
        if (biomeId === 'metro') {
            for (let s = 26; s < track.length - 16; s += 34) { const p = AD.pointAt(track, s); for (const sd of [-1, 1]) tryPlace('street_lamp', s, sd, 6.8, p.x + p.nx * sd * (p.halfWidth + 6.8), p.y + p.ny * sd * (p.halfWidth + 6.8), { yaw: yawToward(-sd * p.nx, -sd * p.ny), fixedScale: 1, tint: rng(), minDist: 6.2 }); }
            for (let s = 55; s < track.length - 40; s += 56) { const p = AD.pointAt(track, s); for (const sd of [-1, 1]) { const d = 24 + rng() * 14, off = sd * (p.halfWidth + d); tryPlace(rng() < 0.5 ? 'skyscraper_a' : 'skyscraper_b', s, sd, d, p.x + p.nx * off, p.y + p.ny * off, { yaw: yawToward(-sd * p.nx, -sd * p.ny) + (rng() - 0.5) * 0.06, scR: rng(), tint: rng(), minDist: 16 }); } }
        } else if (biomeId === 'harbour') {
            for (let s = 30; s < track.length - 20; s += 42) { const p = AD.pointAt(track, s); for (const sd of [-1, 1]) { const off = sd * (p.halfWidth + 7.4), x = p.x + p.nx * off, y = p.y + p.ny * off; if (AD.propSide(track, biomeId, x, y) === 'quay') tryPlace('storm_lantern', s, sd, 7.4, x, y, { yaw: yawToward(-sd * p.nx, -sd * p.ny), fixedScale: 1, tint: rng(), minDist: 6.5 }); } }
        } else if (biomeId === 'alpine' || biomeId === 'industry') {
            const pName = biomeId === 'alpine' ? 'cablecar_pylon' : 'pylon', d0 = biomeId === 'alpine' ? 95 : 72, step = biomeId === 'alpine' ? 115 : 122, cnt = biomeId === 'alpine' ? 3 : 4;
            [0.2, 0.65].forEach((frac, ci) => {
                const p0 = AD.pointAt(track, track.length * frac), sd = ci % 2 ? 1 : -1, bx = p0.x + p0.nx * sd * (p0.halfWidth + d0), by = p0.y + p0.ny * sd * (p0.halfWidth + d0), cy = yawToward(p0.tx, p0.ty);
                for (let idx = 0; idx < cnt; idx++) { const px = bx + p0.tx * (idx - 1) * step, py = by + p0.ty * (idx - 1) * step, smp = AD.sampleTrack(track, px, py); tryPlace(pName, smp.s, sd, Math.hypot(px - smp.x, py - smp.y) - smp.halfWidth, px, py, { yaw: cy, fixedScale: 1, tint: 0.5, minDist: 28, chain: ci, chainIdx: idx }); }
            });
        }
        for (let k = 0; k < segs; k++) for (const side of [-1, 1]) for (const e of entries) {
            const m = meta[e.name] || DEF_META, tags = m.tags || [];
            const natural = NATURAL.test(e.name) || (tags.indexOf('scatter') >= 0 && !e.back && e.name !== 'street_lamp' && e.name !== 'road_sign');
            const grove = 0.32 + 1.35 * vnoise((k * SEG) / 240 + (side > 0 ? 17 : 0), AD.hashString(e.name));
            const count = Math.floor(e.rate * (DENSITY[biomeId] || 1) * (natural ? grove : 1) + rng());
            for (let i = 0; i < count; i++) {
                const s = k * SEG + rng() * SEG, dist = e.min + (e.max - e.min) * Math.pow(rng(), e.back ? 1 : 1.45), p = AD.pointAt(track, s), off = side * (p.halfWidth + dist);
                tryPlace(e.name, s, side, dist, p.x + p.nx * off, p.y + p.ny * off, { scale: e.scale, back: e.back, minDist: e.min, yawR: rng(), scR: rng(), tint: rng() });
            }
        }
        return out;
    };

    /* ---- trackside furniture ------------------------------------------------------------------ */
    const FURN_META = {
        grandstand: { r: 13.4, h: 8.9 }, fan_camp: { r: 3.5, h: 4.3 }, hoarding: { r: 4.9, h: 5.5 }, marshal_post: { r: 1.6, h: 5.1 }, gnome_statue: { r: 1.5, h: 5.4 },
        hotdog_stand: { r: 1.9, h: 5 }, camper_van: { r: 2.9, h: 3.6 }, inflatable_arch: { r: 7.7, h: 10.9 }, road_sign: { r: 1, h: 3.9 },
        balloon_bunch: { r: 1.1, h: 5.6 }, photo_tower: { r: 1.4, h: 6.9 },
        windsock: { r: 1.2, h: 6.5 }, rain_shelter: { r: 2.7, h: 3.3 }
    };
    AD.BRAND_COUNT = 8;
    AD.SIGN_COUNT = 16;
    AD.furnitureProps = function (track) {
        const out = [];
        const length = track.length;
        const fs = featuresFor(track);
        const add = (name, s, off, opts) => {
            const o = opts || {};
            const p = AD.pointAt(track, s);
            const x = p.x + p.nx * off, y = p.y + p.ny * off;
            let dx, dy;
            if (o.dir) { dx = o.dir[0]; dy = o.dir[1]; }
            else if (o.across) { dx = -p.tx; dy = -p.ty; }
            else {
                const sd = off >= 0 ? 1 : -1, a = o.angle === undefined ? 0 : o.angle;
                dx = -sd * p.nx * Math.cos(a) - p.tx * Math.sin(a); dy = -sd * p.ny * Math.cos(a) - p.ty * Math.sin(a);
            }
            const m = FURN_META[name] || { r: 3, h: 5 };
            const sz = o.across ? (2 * (p.halfWidth + 2.6)) / (o.span || 16) : 1;
            out.push(Object.assign({
                kind: 'furniture', name: name, biome: 'shared', s: s, side: off >= 0 ? 1 : -1, x: x, y: y, yaw: yawToward(dx, dy) + (o.jitter || 0),
                sx: 1, sy: 1, sz: sz, tint: ihash(Math.round(s), name.length), r: m.r * (o.across ? Math.max(1, sz) : 1), h: m.h, reach: 1, furniture: true, tags: 'hero'
            }, o.extra || {}));
        };
        const hw = s => AD.pointAt(track, s).halfWidth;
        const biomeId = track.biomeId;
        const quaySide = (p) => biomeId !== 'harbour' ? 1 : (AD.propSide(track, 'harbour', p.x + p.nx * (p.halfWidth + 10), p.y + p.ny * (p.halfWidth + 10)) === 'quay' ? 1 : -1);
        const flatS = (baseS, preferSide, baseOff, maxSlope, rad) => {
            let best = { s: ((baseS % length) + length) % length, side: preferSide, score: 1e9 };
            for (const ds of [0, 12, -12, 24, -24, 38, -38]) {
                const s = (((baseS + ds) % length) + length) % length;
                const p = AD.pointAt(track, s);
                for (const side of [preferSide, -preferSide]) {
                    const x = p.x + p.nx * side * (p.halfWidth + baseOff), y = p.y + p.ny * side * (p.halfWidth + baseOff);
                    if (biomeId === 'harbour' && AD.propSide(track, 'harbour', x, y) !== 'quay') continue;
                    const near = AD.sampleTrack(track, x, y);
                    if (Math.hypot(x - near.x, y - near.y) < near.halfWidth + Math.min(baseOff, 4)) continue;
                    if (out.some(o => Math.hypot(o.x - x, o.y - y) < o.r + rad + 2.5)) continue;
                    const sl = AD.terrainSlopeAt ? AD.terrainSlopeAt(track, biomeId, x, y, rad).slope : 0;
                    const score = sl + (side !== preferSide ? 0.08 : 0) + Math.abs(ds) * 0.002;
                    if (sl <= maxSlope) return { s, side };
                    if (score < best.score) best = { s, side, score };
                }
            }
            return best;
        };
        /* Start straight: gantry over the line, grandstands on paddock terrace, inflatable arches at sector lines. */
        out.push(Object.assign({}, (() => {
            const p = AD.pointAt(track, 0);
            return {
                kind: 'furniture', name: 'gantry', biome: 'track', special: 'gantry', s: 0, side: 0, x: p.x, y: p.y, yaw: yawToward(-p.tx, -p.ty),
                sx: 1, sy: 1, sz: (2 * (p.halfWidth + 2.6)) / 16, tint: 0, r: 9, h: 7.3, reach: 1, furniture: true, tags: 'hero'
            };
        })()));
        if (biomeId === 'harbour') {
            for (const s of [length - 54, 54, length - 104, 104]) { const p = AD.pointAt(track, s); add('grandstand', s, quaySide(p) * (p.halfWidth + 12.8), { extra: { tint: 0.5 } }); }
        } else {
            for (const s of [length - 52, 52]) for (const side of [-1, 1]) add('grandstand', s, side * (hw(s) + 12.8), { extra: { tint: 0.5 } });
        }
        track.sectors.forEach((s, i) => add('inflatable_arch', s, 0, { across: true, span: 15.4, extra: { special: 'arch', tint: i } }));
        for (const s of [length - 92, 92]) for (const side of [-1, 1]) add('balloon_bunch', s, (biomeId === 'harbour' ? quaySide(AD.pointAt(track, s)) : side) * (hw(s) + (biomeId === 'harbour' ? 7.2 + (side > 0 ? 3.5 : 0) : 7.2)), { angle: 0.5 });
        track.sectors.forEach((s, i) => { const ts = s + 14; add('photo_tower', ts, (biomeId === 'harbour' ? quaySide(AD.pointAt(track, ts)) : (i % 2 ? 1 : -1)) * (hw(ts) + 10.5), { angle: 0.6 }); });
        let slot = 0;
        for (let s = 130; s < length - 110; s += 200) {
            const smp = AD.pointAt(track, s), idx = AD.sampleTrack(track, smp.x, smp.y).index;
            if (track.curvature[idx] > 0.009 || wrapDist(s, 0, length) < 110) continue;
            add('hoarding', s, (biomeId === 'harbour' ? quaySide(smp) : (slot++ % 2 ? 1 : -1)) * (hw(s) + 9.5), { angle: 0.75, extra: { brand: Math.floor(ihash(Math.round(s), 7) * AD.BRAND_COUNT) } });
        }
        const marshal = (s, side) => add('marshal_post', s, side * (hw(s) + 3.4), { angle: 0.3 });
        if (fs) fs.crossings.forEach((c, i) => marshal(c.s - 6, biomeId === 'harbour' ? quaySide(AD.pointAt(track, c.s - 6)) : (i % 2 ? 1 : -1)));
        for (let k = 0; k < 4; k++) { const ms = length * (k + 0.5) / 4 + 30; marshal(ms, biomeId === 'harbour' ? quaySide(AD.pointAt(track, ms)) : (k % 2 ? -1 : 1)); }
        for (let s = 250; s < length - 150; s += 500) add('gnome_statue', s, (biomeId === 'harbour' ? quaySide(AD.pointAt(track, s)) : ((s / 500) % 2 ? 1 : -1)) * (hw(s) + 5.5), { angle: 0.4 });
        /* Cohesive Fan Zones & Festival Camps: group fan_camp, hotdog_stand & camper_van on flat pockets. */
        const campSpots = [0.24, 0.48, 0.74].map((f, i) => { const h = flatS(length * f, i % 2 ? -1 : 1, 8.5, 0.25, 4); add('fan_camp', h.s, h.side * (hw(h.s) + 8.5), { angle: 0.15 }); return h; });
        [campSpots[0].s + 13, campSpots[1].s + 13].forEach((s0, i) => { const h = flatS(s0, campSpots[i].side, 7.2, 0.25, 3); add('hotdog_stand', h.s, h.side * (hw(h.s) + 7.2), { angle: 0.25 }); });
        [campSpots[0].s - 15, 0.36 * length, campSpots[1].s - 15, campSpots[2].s + 15].forEach((s0, i) => { const h = flatS(s0, i < 3 ? campSpots[i % 3].side : (i % 2 ? 1 : -1), 15 + (i % 2) * 3, 0.24, 3.2); add('camper_van', h.s, h.side * (hw(h.s) + 15 + (i % 2) * 3), { jitter: 0.22 * (i - 1.5) }); });
        track.sectors.forEach((s, i) => { const ws = ((s - 70) % length + length) % length; add('windsock', ws, (biomeId === 'harbour' ? quaySide(AD.pointAt(track, ws)) : (i % 2 ? -1 : 1)) * (hw(ws) + 6.4), { angle: 0.35 }); });
        [0.15, 0.42, 0.68, 0.91].forEach((f, i) => { const h = flatS(length * f, i % 2 ? 1 : -1, 5.4, 0.3, 3); add('rain_shelter', h.s, h.side * (hw(h.s) + 5.4), { angle: 0.2 }); });
        const signSpots = fs ? fs.crossings.map(c => c.signS !== undefined ? c.signS : c.s - 120).concat(fs.ramps.map(r => r.s - 120)) : [];
        signSpots.forEach((s0, i) => {
            const s = ((s0 % length) + length) % length;
            if (wrapDist(s, 0, length) >= 60) add('road_sign', s, (biomeId === 'harbour' ? quaySide(AD.pointAt(track, s)) : (i % 2 ? 1 : -1)) * (hw(s) + 4.6), { angle: 1.05, extra: { sign: Math.floor(ihash(i + 3, AD.hashString(track.id)) * AD.SIGN_COUNT) } });
        });
        return out;
    };

    /* ---- spatial index and the streaming filter ------------------------------------------------- */
    AD.propIndex = function (placements) {
        const cells = new Map();
        placements.forEach((p, i) => {
            const k = Math.floor(p.x / CELL) + ',' + Math.floor(p.y / CELL);
            const list = cells.get(k);
            if (list) list.push(i); else cells.set(k, [i]);
        });
        return cells;
    };
    /* view: { x, y, fx, fy } in sim units (forward unit vector). limits: { radius, cap, cone (cos), near }.
       Returns placement indices, nearest first, at most cap long. */
    AD.filterProps = function (placements, cells, view, limits) {
        const R = limits.radius, rmax = R * 1.8, near = limits.near === undefined ? 70 : limits.near, cone = limits.cone === undefined ? 0.42 : limits.cone;
        const reach = Math.ceil(rmax / CELL);
        const cx = Math.floor(view.x / CELL), cy = Math.floor(view.y / CELL);
        const hits = [];
        for (let ox = -reach; ox <= reach; ox++) {
            for (let oy = -reach; oy <= reach; oy++) {
                const list = cells.get((cx + ox) + ',' + (cy + oy));
                if (!list) continue;
                for (const i of list) {
                    const p = placements[i];
                    const dx = p.x - view.x, dy = p.y - view.y, d = Math.hypot(dx, dy);
                    if (d > R * p.reach) continue;
                    if (d > near && (dx * view.fx + dy * view.fy) / d < cone) continue;
                    hits.push([d, i]);
                }
            }
        }
        hits.sort((a, b) => a[0] - b[0]);
        const cap = Math.max(0, limits.cap | 0);
        const out = [];
        for (let i = 0; i < hits.length && i < cap; i++) out.push(hits[i][1]);
        return out;
    };

    /* ---- ground of a placement -------------------------------------------------------------------- */
    /* The ground the player sees comes from the terrain module, which reads the same
       distance field the terrain mesh is built from. Without it a prop hovers over
       the verge, because the road surface is 0.4 m above it. */
    AD.propGroundAt = function (track, biomeId, x, y) {
        if (typeof AD.groundHeight === 'function') {
            try {
                const h = AD.groundHeight(track, biomeId, x, y);
                if (isFinite(h)) return h;
            } catch (_) { /* fall through to the flat rule */ }
        }
        if (biomeId === 'harbour') {
            const smp = AD.sampleTrack(track, x, y);
            const outer = innerSign(track, x, y, smp) !== Math.sign(smp.offset);
            return outer && Math.abs(smp.offset) > smp.halfWidth + 14 ? -3 : -0.4;
        }
        return biomeId === 'salt' ? -0.3 : biomeId === 'metro' ? -0.2 : 0;
    };

    /* Which side of the loop a point lies on. The harbour has two: inside is the
       quay, outside is the sea. */
    AD.propSide = function (track, biomeId, x, y) {
        if (biomeId !== 'harbour') return 'land';
        const field = typeof AD.trackField === 'function' ? AD.trackField(track, 8) : null;
        if (!field) return 'quay';
        const info = {};
        field.info(x, y, info);
        const inner = info.inner !== null && info.inner !== undefined ? info.inner : field.inside(x, y);
        return inner ? 'quay' : 'water';
    };

    /* Height of a placement, or null when the prop does not belong there.
       Every prop stands on the ground it is placed on; nothing is lifted to road
       level. The harbour adds its own rule: the boats float on the water, the quay
       carries the cranes and the container stacks. */
    AD.propHeight = function (track, biomeId, p) {
        const gy = AD.propGroundAt(track, biomeId, p.x, p.y);
        if (biomeId !== 'harbour') return gy;
        const side = AD.propSide(track, biomeId, p.x, p.y);
        if (p.kind === 'scatter') {
            if (WATER_ONLY[p.name]) {
                /* A boat needs water under it: the ground has to be below the water
                   line, otherwise the hull would hang over the quay. */
                if (side !== 'water' || gy > WATER_Y - 0.2) return null;
                return WATER_Y;
            }
            if (p.name === 'lighthouse') {
                if (side !== 'water' || gy > WATER_Y) return null;
                return Math.max(gy, WATER_Y - 1);   /* its rock stands in the sea */
            }
            /* The quay carries the cranes and the container stacks: the ground has to
               be clearly above the water line. */
            if (side !== 'quay' || gy < WATER_Y + 0.8) return null;
        } else if (p.name !== 'gantry' && (side !== 'quay' || gy < WATER_Y + 0.8)) {
            return null;                            /* no grandstand on the sea */
        }
        return gy;
    };

    /* Exact downward grounding offset (<= 0) so every corner touches the rendered terrain triangle. */
    AD.propGroundOffset = function (track, biomeId, p, nearCell, farCell) {
        if (!p || !track || typeof AD.terrainMeshHeight !== 'function') return 0;
        if (biomeId === 'harbour' && WATER_ONLY[p.name]) return 0;
        const nc = nearCell || 8, fc = farCell || 16, gy = p.gy !== undefined ? p.gy : AD.propGroundAt(track, biomeId, p.x, p.y);
        if (p.name === 'gantry' || p.name === 'inflatable_arch') {
            const smp = AD.pointAt(track, p.s || 0), span = (smp.halfWidth || 12) + 2.6;
            const lh = AD.terrainMeshHeight(track, biomeId, p.x + smp.nx * span, p.y + smp.ny * span, nc);
            const rh = AD.terrainMeshHeight(track, biomeId, p.x - smp.nx * span, p.y - smp.ny * span, nc);
            return Math.max(-2.4, Math.min(0, Math.min(lh, rh) - gy) - 0.18);
        }
        const useFar = p.back || (p.dist || 0) > 140;
        let low = Math.min(gy, AD.terrainMeshHeight(track, biomeId, p.x, p.y, nc));
        if (useFar) low = Math.min(low, AD.terrainMeshHeight(track, biomeId, p.x, p.y, fc));
        const footR = clamp((p.r || 2) * 0.65, 0.8, 7.5), steps = (p.r || 2) >= 3.5 ? 8 : 4;
        for (let i = 0; i < steps; i++) {
            const a = (i / steps) * Math.PI * 2, sx = p.x + Math.cos(a) * footR, sy = p.y + Math.sin(a) * footR;
            low = Math.min(low, AD.terrainMeshHeight(track, biomeId, sx, sy, nc));
            if (useFar) low = Math.min(low, AD.terrainMeshHeight(track, biomeId, sx, sy, fc));
        }
        const embed = NATURAL.test(p.name) ? 0.18 : 0.10;
        const maxSink = Math.max(2.6, (p.h || 6) * 0.48);
        return Math.max(-maxSink, Math.min(0, low - gy) - embed);
    };

    /* ---- brand and sign canvases ------------------------------------------------------------------- */
    const BRANDS = [
        { id: 'wurstwerk', name: 'WURSTWERK', tag: 'Seit gestern traditionell', a: '#d62f2f', b: '#ffd23f', icon: 'sausage' },
        { id: 'muhmilch', name: 'MUH-MILCH', tag: 'Jetzt mit mehr Kuh', a: '#3a7bd5', b: '#ffffff', icon: 'cow' },
        { id: 'turbotoast', name: 'TURBO-TOAST', tag: 'Heiß, schnell, krümelig', a: '#ff8a1f', b: '#2b1a10', icon: 'toast' },
        { id: 'sofaspeed', name: 'SOFA-SPEED', tag: 'Sitzen ist das neue Rasen', a: '#7b4bd6', b: '#ffe0f2', icon: 'sofa' },
        { id: 'gnombau', name: 'GNOMBAU AG', tag: 'Wir zwergen für Sie', a: '#2e9d4f', b: '#fff2a8', icon: 'gnome' },
        { id: 'nebelhorn', name: 'NEBELHORN TEE', tag: 'Klingt nach Pause', a: '#1fb5b0', b: '#08323a', icon: 'cup' },
        { id: 'entenbank', name: 'ENTENBANK', tag: 'Zinsen mit Quietsch', a: '#ffcf2a', b: '#2a2a2a', icon: 'duck' },
        { id: 'plueschreifen', name: 'PLÜSCH-REIFEN', tag: 'Bremsen? Nee.', a: '#e8368f', b: '#1a1030', icon: 'tyre' }
    ];
    const SIGNS = [
        ['cow_priority', 'Kühe haben Vorfahrt', '#ffd23f', 'cow'], ['brake_is_cowardly', 'Bremsen ist für Feiglinge', '#e03a3a', 'ban'],
        ['sofa_lane', 'Sofa-Spur', '#3f8fe0', 'arrow'], ['gnomes_watching', 'Zwerge sehen alles', '#ffd23f', 'eye'],
        ['no_wheelies', 'Wheelies nur nach Feierabend', '#e03a3a', 'ban'], ['speed_limit_ish', 'Tempolimit: ja', '#ffffff', 'warn'],
        ['caution_ducks', 'Achtung: Quietscheenten', '#ffd23f', 'warn'], ['free_hugs_ahead', 'Gratis-Umarmungen 200 m', '#ff7ab8', 'heart'],
        ['last_exit_pizza', 'Letzte Ausfahrt vor Pizza', '#3fb86a', 'arrow'], ['mind_the_gap', 'Vorsicht: Lücke', '#ffd23f', 'warn'],
        ['drift_zone', 'Driftzone: Reifen bitte mitbringen', '#3f8fe0', 'bolt'], ['you_are_here', 'Sie sind hier', '#ffffff', 'pin'],
        ['turn_left_maybe', 'Links abbiegen (vielleicht)', '#3fb86a', 'arrow'], ['wrong_way_fun', 'Falsch ist auch ein Weg', '#e03a3a', 'ban'],
        ['hold_my_nitro', 'Halt mal mein Nitro', '#ffd23f', 'bolt'], ['honk_for_cow', 'Hupen erlaubt, Kuh grüßt', '#ffffff', 'cow']
    ];
    function icon(g, kind, cx, cy, s, ink) {
        g.save(); g.translate(cx, cy); g.scale(s, s); g.fillStyle = ink; g.strokeStyle = ink; g.lineWidth = 0.09; g.lineCap = 'round'; g.lineJoin = 'round';
        const ell = (x, y, rx, ry) => { g.beginPath(); g.ellipse(x, y, rx, ry, 0, 0, 7); g.fill(); };
        if (kind === 'sausage') { g.lineWidth = 0.5; g.beginPath(); g.moveTo(-0.7, 0.2); g.quadraticCurveTo(0, -0.5, 0.7, 0.2); g.stroke(); }
        else if (kind === 'cow') { ell(0, 0.05, 0.6, 0.5); g.fillStyle = '#fff'; ell(-0.5, -0.25, 0.28, 0.22); ell(0.5, -0.25, 0.28, 0.22); g.fillStyle = ink; g.fillRect(-0.35, 0.1, 0.7, 0.3); g.fillStyle = '#fff'; ell(-0.2, -0.05, 0.1, 0.1); ell(0.2, -0.05, 0.1, 0.1); }
        else if (kind === 'toast') { g.beginPath(); g.moveTo(-0.6, 0.6); g.lineTo(-0.6, -0.2); g.quadraticCurveTo(-0.7, -0.7, -0.2, -0.65); g.lineTo(0.2, -0.65); g.quadraticCurveTo(0.7, -0.7, 0.6, -0.2); g.lineTo(0.6, 0.6); g.closePath(); g.fill(); }
        else if (kind === 'sofa') { g.fillRect(-0.7, -0.2, 1.4, 0.55); g.fillRect(-0.55, -0.55, 1.1, 0.4); g.fillRect(-0.62, 0.35, 0.12, 0.2); g.fillRect(0.5, 0.35, 0.12, 0.2); }
        else if (kind === 'gnome') { g.beginPath(); g.moveTo(0, -0.8); g.lineTo(0.45, 0.05); g.lineTo(-0.45, 0.05); g.closePath(); g.fill(); ell(0, 0.3, 0.3, 0.26); g.fillStyle = '#fff'; ell(0, 0.5, 0.32, 0.22); }
        else if (kind === 'cup') { g.beginPath(); g.moveTo(-0.45, -0.1); g.lineTo(0.45, -0.1); g.lineTo(0.3, 0.55); g.lineTo(-0.3, 0.55); g.closePath(); g.fill(); g.lineWidth = 0.1; g.beginPath(); g.moveTo(-0.15, -0.55); g.quadraticCurveTo(0.1, -0.35, -0.1, -0.2); g.moveTo(0.18, -0.55); g.quadraticCurveTo(0.4, -0.35, 0.2, -0.2); g.stroke(); }
        else if (kind === 'duck') { ell(0, 0.2, 0.55, 0.4); ell(0.25, -0.3, 0.28, 0.28); g.fillStyle = '#ff7a1a'; g.beginPath(); g.moveTo(0.45, -0.3); g.lineTo(0.85, -0.2); g.lineTo(0.45, -0.1); g.closePath(); g.fill(); }
        else if (kind === 'tyre') { g.lineWidth = 0.26; g.beginPath(); g.arc(0, 0, 0.5, 0, 7); g.stroke(); g.lineWidth = 0.08; g.beginPath(); g.arc(0, 0, 0.15, 0, 7); g.stroke(); }
        else if (kind === 'warn') { g.beginPath(); g.moveTo(0, -0.75); g.lineTo(0.8, 0.65); g.lineTo(-0.8, 0.65); g.closePath(); g.lineWidth = 0.14; g.stroke(); g.fillRect(-0.06, -0.3, 0.12, 0.6); ell(0, 0.46, 0.08, 0.08); }
        else if (kind === 'ban') { g.lineWidth = 0.16; g.beginPath(); g.arc(0, 0, 0.62, 0, 7); g.stroke(); g.beginPath(); g.moveTo(-0.44, 0.44); g.lineTo(0.44, -0.44); g.stroke(); }
        else if (kind === 'arrow') { g.beginPath(); g.moveTo(-0.7, -0.15); g.lineTo(0.1, -0.15); g.lineTo(0.1, -0.5); g.lineTo(0.75, 0); g.lineTo(0.1, 0.5); g.lineTo(0.1, 0.15); g.lineTo(-0.7, 0.15); g.closePath(); g.fill(); }
        else if (kind === 'eye') { g.beginPath(); g.moveTo(-0.8, 0); g.quadraticCurveTo(0, -0.7, 0.8, 0); g.quadraticCurveTo(0, 0.7, -0.8, 0); g.fill(); g.fillStyle = '#fff'; ell(0, 0, 0.3, 0.3); g.fillStyle = ink; ell(0.05, 0, 0.14, 0.14); }
        else if (kind === 'heart') { g.beginPath(); g.moveTo(0, 0.6); g.bezierCurveTo(-1, -0.1, -0.5, -0.8, 0, -0.3); g.bezierCurveTo(0.5, -0.8, 1, -0.1, 0, 0.6); g.fill(); }
        else if (kind === 'bolt') { g.beginPath(); g.moveTo(0.15, -0.8); g.lineTo(-0.5, 0.1); g.lineTo(-0.05, 0.1); g.lineTo(-0.2, 0.8); g.lineTo(0.5, -0.15); g.lineTo(0.05, -0.15); g.closePath(); g.fill(); }
        else if (kind === 'pin') { g.beginPath(); g.arc(0, -0.2, 0.42, Math.PI, 0); g.lineTo(0, 0.7); g.closePath(); g.fill(); g.fillStyle = '#fff'; ell(0, -0.2, 0.16, 0.16); }
        g.restore();
    }
    function roundRect(g, x, y, w, h, r) { g.beginPath(); g.moveTo(x + r, y); g.arcTo(x + w, y, x + w, y + h, r); g.arcTo(x + w, y + h, x, y + h, r); g.arcTo(x, y + h, x, y, r); g.arcTo(x, y, x + w, y, r); g.closePath(); }
    function drawBrand(brand, look, tag, night, accent) {
        const c = document.createElement('canvas');
        c.width = 640; c.height = 224;
        const g = c.getContext('2d');
        const bg = night ? '#0b0a16' : brand.a, fg = night ? brand.a : brand.b;
        g.fillStyle = bg; g.fillRect(0, 0, 640, 224);
        if (!night) {
            const gr = g.createLinearGradient(0, 0, 0, 224);
            gr.addColorStop(0, 'rgba(255,255,255,0.18)'); gr.addColorStop(1, 'rgba(0,0,0,0.22)');
            g.fillStyle = gr; g.fillRect(0, 0, 640, 224);
            g.fillStyle = brand.b; g.globalAlpha = 0.18;
            for (let x = -224; x < 640; x += 64) { g.beginPath(); g.moveTo(x, 224); g.lineTo(x + 48, 224); g.lineTo(x + 272, 0); g.lineTo(x + 224, 0); g.fill(); }
            g.globalAlpha = 1;
        }
        g.lineWidth = 10; g.strokeStyle = night ? accent : brand.b; roundRect(g, 8, 8, 624, 208, 22); g.stroke();
        if (night) { g.shadowColor = brand.a; g.shadowBlur = 18; }
        icon(g, brand.icon, 100, 112, 70, night ? brand.b : brand.b);
        g.shadowBlur = night ? 22 : 0;
        g.fillStyle = night ? '#ffffff' : brand.b; g.textAlign = 'left'; g.textBaseline = 'alphabetic';
        let size = 64;
        g.font = '900 ' + size + 'px "Trebuchet MS", Impact, sans-serif';
        while (g.measureText(brand.name).width > 420 && size > 28) { size -= 3; g.font = '900 ' + size + 'px "Trebuchet MS", Impact, sans-serif'; }
        if (night) g.fillStyle = fg;
        g.fillText(brand.name, 196, 112);
        g.shadowBlur = 0;
        g.fillStyle = night ? 'rgba(255,255,255,0.88)' : brand.b;
        let ts = 30;
        g.font = 'italic 700 ' + ts + 'px "Trebuchet MS", sans-serif';
        while (g.measureText(tag).width > 400 && ts > 16) { ts -= 2; g.font = 'italic 700 ' + ts + 'px "Trebuchet MS", sans-serif'; }
        g.fillText(tag, 196, 156);
        return c;
    }
    function drawSignAtlas(tr) {
        const c = document.createElement('canvas');
        c.width = 1024; c.height = 512;
        const g = c.getContext('2d');
        SIGNS.forEach((def, i) => {
            const cx = (i % 4) * 256, cy = Math.floor(i / 4) * 128;
            g.save(); g.translate(cx, cy); g.scale(2, 1);
            g.fillStyle = '#e9edf2'; g.fillRect(0, 0, 128, 128);
            g.fillStyle = '#1b2230'; roundRect(g, 4, 4, 120, 120, 14); g.fill();
            g.fillStyle = def[2]; roundRect(g, 9, 9, 110, 110, 10); g.fill();
            const ink = def[2] === '#ffffff' || def[2] === '#ffd23f' || def[2] === '#ff7ab8' ? '#1b2230' : '#ffffff';
            icon(g, def[3], 64, 40, 30, ink);
            g.fillStyle = ink; g.textAlign = 'center'; g.textBaseline = 'middle';
            const words = tr('aura_drift.sign.' + def[0], def[1]).split(' ');
            g.font = '800 15px "Trebuchet MS", sans-serif';
            const lines = []; let line = '';
            for (const w of words) { const test = line ? line + ' ' + w : w; if (g.measureText(test).width > 100 && line) { lines.push(line); line = w; } else line = test; }
            lines.push(line);
            const shown = lines.slice(0, 3);
            shown.forEach((ln, j) => g.fillText(ln, 64, 78 + j * 17 - (shown.length - 1) * 4));
            g.restore();
        });
        return c;
    }

    /* Canvas painters, exposed so the visual survey can look at the sign atlas and the brand panels. */
    AD.propCanvases = { atlas: drawSignAtlas, brand: drawBrand, brands: BRANDS };

    AD.createRoadsideShow = function (T, scene, shows, terrain) {
        const groups = [], pose = {}, ray = new T.Raycaster(), down = new T.Vector3(0, -1, 0);
        terrain?.updateMatrixWorld(true);
        for (const show of shows) {
            const site = show.site, group = new T.Group();
            group.name = 'roadside-' + show.story;
            group.position.set(site.x, site.z, site.y);
            let along = 0, across = 0;
            if (terrain) {
                ray.set(new T.Vector3(site.x, 400, site.y), down);
                const ground = ray.intersectObject(terrain, true)[0];
                if (ground) {
                    group.position.y = ground.point.y;
                    const normal = ground.face.normal.clone().applyNormalMatrix(new T.Matrix3().getNormalMatrix(ground.object.matrixWorld));
                    along = -(normal.x * site.tx + normal.z * site.ty) / Math.max(.1, normal.y);
                    across = (normal.x * site.ty - normal.z * site.tx) / Math.max(.1, normal.y);
                }
            }
            group.rotation.y = -Math.atan2(site.ty, site.tx);
            const actors = show.parts.map(part => {
                const id = part.model;
                const template = ['hay_bale', 'cone', 'tyre_stack'].includes(id) ? AD.assets.trackPiece(id) :
                    ['cow', 'crab', 'penguin', 'llama', 'sheep'].includes(id) ? AD.assets.animal(id) : AD.assets.item(id);
                const model = template.clone(true), holder = new T.Group();
                const box = new T.Box3().setFromObject(model), centre = box.getCenter(new T.Vector3());
                model.position.x -= centre.x; model.position.y -= box.min.y; model.position.z -= centre.z;
                holder.scale.setScalar(part.height / Math.max(.1, box.max.y - box.min.y));
                holder.add(model); group.add(holder);
                return { part, holder };
            });
            group.visible = false; scene.add(group); groups.push({ show, group, actors, along, across });
        }
        return {
            update(eye, time) {
                for (const { show, group, actors, along, across } of groups) {
                    const site = show.site;
                    group.visible = Math.hypot(eye[0] - site.x, eye[2] - site.y) < 140;
                    if (!group.visible) continue;
                    for (const { part, holder } of actors) {
                        AD.roadsidePose(part, site, time, pose, show.cheerAt, show.reaction);
                        holder.position.set(pose.x, pose.z + along * pose.x + across * pose.y, pose.y);
                        holder.rotation.set(0, Math.PI / 2 - pose.yaw, pose.lean, 'YXZ');
                    }
                }
            },
            dispose() { for (const { group } of groups) group.removeFromParent(); }
        };
    };

    /* ---- the three.js part --------------------------------------------------------------------------- */
    AD.createProps = function (T, scene, track, biomeId, q, t) {
        const look = (AD.LOOK && AD.LOOK[biomeId]) || {};
        const night = !!look.night;
        const tr = (key, fallback) => { let v = null; try { v = typeof t === 'function' ? t(key) : null; } catch (_) { v = null; } return typeof v === 'string' && v && v !== key ? v : fallback; };
        const group = new T.Group();
        group.name = 'props';
        scene.add(group);
        const wind = { value: 0 };
        let reduced = !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
        let glq = q;
        let placements = [], stands = [], cells = new Map(), kinds = new Map(), emitters = [], loose = [], gantry = null, ufo = null;
        let owned = [], texOwn = [], signPool = [], brandMats = [], brandTex = [], signTex = null;
        let lastView = null, lastTime = -1, selected = 0, tris = 0, visibleKinds = 0, clock = 0, prevNow = 0;
        const M = new T.Matrix4(), place = new T.Matrix4(), tmpQ = new T.Quaternion(), tmpP = new T.Vector3(), tmpS = new T.Vector3(), UP = new T.Vector3(0, 1, 0), spinM = new T.Matrix4();
        const colour = new T.Color();

        function swayed(mat, strength) {
            mat.onBeforeCompile = shader => {
                shader.uniforms.uWind = wind;
                shader.vertexShader = 'uniform float uWind;\n' + shader.vertexShader.replace('#include <begin_vertex>',
                    '#include <begin_vertex>\n#ifdef USE_INSTANCING\n float ph = instanceMatrix[3].x * 0.13 + instanceMatrix[3].z * 0.17;\n#else\n float ph = 0.0;\n#endif\n' +
                    ' float sw = transformed.y * transformed.y * ' + strength.toFixed(5) + ';\n transformed.x += sin(uWind * 1.7 + ph) * sw; transformed.z += cos(uWind * 1.3 + ph * 1.3) * sw * 0.7;');
            };
            mat.customProgramCacheKey = () => 'agdrift-sway-' + strength;
        }

        function cheered(mat, isArms) {
            mat.onBeforeCompile = shader => {
                shader.uniforms.uWind = wind;
                shader.vertexShader = 'uniform float uWind;\n' + shader.vertexShader.replace('#include <begin_vertex>',
                    '#include <begin_vertex>\n#ifdef USE_INSTANCING\n float iph = instanceMatrix[3].x * 0.19 + instanceMatrix[3].z * 0.23;\n#else\n float iph = 0.0;\n#endif\n' +
                    ' float cph = sin(transformed.z * 1.45) * 2.1 + cos(transformed.x * 1.6) * 1.8 + iph;\n' +
                    ' float ola = max(0.0, sin(uWind * 4.2 - transformed.z * 0.35 + iph));\n' +
                    ' float hop = abs(sin(uWind * 5.8 + cph)) * 0.14 + ola * 0.25;\n' +
                    ' transformed.y += hop;\n' +
                    ' transformed.z += sin(uWind * 5.1 + cph * 1.3) * (0.08 + ola * 0.07);\n' +
                    ' transformed.x += cos(uWind * 4.5 + cph) * 0.05;\n' +
                    (isArms ? ' transformed.y += sin(uWind * 10.6 + cph * 2.1) * 0.11;\n transformed.z += cos(uWind * 9.8 + cph * 1.7) * 0.14;\n transformed.x += sin(uWind * 8.6 + cph * 1.4) * 0.08;\n' : ''));
            };
            mat.customProgramCacheKey = () => 'agdrift-cheer-' + (isArms ? 'a' : 'b');
        }

        function brandMaterial(srcMat, i) {
            if (brandMats[i]) return brandMats[i];
            const accent = (track.biome && track.biome.accent) || '#ffffff';
            const tex = new T.CanvasTexture(drawBrand(BRANDS[i], look, tr('aura_drift.brand.' + BRANDS[i].id, BRANDS[i].tag), night, accent));
            tex.colorSpace = T.SRGBColorSpace; tex.flipY = false; tex.anisotropy = 4;
            const mat = srcMat.clone();
            mat.map = tex; mat.vertexColors = false; mat.color.set(0xffffff);
            mat.emissive = new T.Color(0xffffff); mat.emissiveMap = tex; mat.emissiveIntensity = night ? 1.5 : 0.12;
            brandTex.push(tex); brandMats[i] = mat;
            return mat;
        }

        /* Clone the template's mesh parts once per kind: shared geometry, cloned materials. */
        function partsOf(tpl, name) {
            const parts = [];
            tpl.updateWorldMatrix(true, true);
            const inv = tpl.matrixWorld.clone().invert();
            let minY = Infinity;
            tpl.traverse(o => {
                if (!o.isMesh) return;
                /* GLTFLoader numbers repeated node names (face, face_1), so match the prefix. */
                const named = re => re.test(o.name) || !!(o.parent && re.test(o.parent.name));
                const nm = named(/^face(_\d+)?$/) ? 'face' : named(/^rotor(_\d+)?$/) ? 'rotor' : named(/^crowd_arms(_\d+)?$/) ? 'crowd_arms' : named(/^crowd(_\d+)?$/) ? 'crowd' : '';
                const mat = Array.isArray(o.material) ? o.material[0] : o.material;
                const m = mat.clone();
                if (nm !== 'face' && o.geometry.attributes.color) m.vertexColors = true;
                if (m.emissiveIntensity && (m.emissive && m.emissive.getHex() !== 0)) m.emissiveIntensity *= look.glow || (night ? 1.5 : 1.1);
                if (nm === 'crowd' || nm === 'crowd_arms') cheered(m, nm === 'crowd_arms');
                else if (nm === '' && /^(pine|snow_pine|oak|birch|flag)/.test(name)) swayed(m, name === 'flag' ? 0.0022 : 0.0012);
                owned.push(m);
                const loc = inv.clone().multiply(o.matrixWorld);
                if (o.geometry && o.geometry.computeBoundingBox) {
                    if (!o.geometry.boundingBox) o.geometry.computeBoundingBox();
                    if (o.geometry.boundingBox) {
                        const b = o.geometry.boundingBox.clone().applyMatrix4(loc);
                        if (isFinite(b.min.y)) minY = Math.min(minY, b.min.y);
                    }
                }
                parts.push({ geo: o.geometry, mat: m, local: loc, role: nm, count: 0, mesh: null, faces: null, tris: o.geometry.index ? o.geometry.index.count / 3 : o.geometry.attributes.position.count / 3 });
            });
            return { parts, minY: isFinite(minY) ? minY : 0 };
        }

        function build() {
            dispose(false);
            const manifest = AD.assets.manifest && AD.assets.manifest();
            const meta = (manifest && manifest.props) || {};
            const scatter = AD.scatterProps(track, biomeId, meta[biomeId] || {});
            const furn = AD.furnitureProps(track);
            placements = [];
            const sharedMeta = meta.shared || {};
            const rule = (AD.TERRAIN && AD.TERRAIN[biomeId]) || {};
            const nearCell = (glq.terrainCell || 8) * (rule.kind === 'flat' && !rule.water ? 2 : 1), farCell = Math.min(32, nearCell * 2);
            for (const p of scatter.concat(furn)) {
                /* The height rules live in the pure part, so the node checks and the
                   renderer agree on where a prop stands. */
                const gy = AD.propHeight(track, biomeId, p);
                if (gy === null) continue;
                p.gy = gy;
                p.footSink = AD.propGroundOffset(track, biomeId, p, nearCell, farCell);
                if (HUG_SET.has(p.name) && AD.terrainSlopeAt) {
                    const sl = AD.terrainSlopeAt(track, biomeId, p.x, p.y, Math.max(2, p.r * 0.6));
                    p.tiltX = clamp(-Math.atan(sl.hz), -0.26, 0.26); p.tiltZ = clamp(Math.atan(sl.hx), -0.26, 0.26);
                }
                if (p.kind === 'furniture' && p.name !== 'gantry' && sharedMeta[p.name]) { p.r = Math.max(p.r, sharedMeta[p.name].radius * 0.5); }
                p.id = placements.length;
                placements.push(p);
            }
            stands = placements.filter(p => p.name === 'grandstand');
            cells = AD.propIndex(placements);
            emitters = placements.filter(p => EMIT[p.name] && p.kind === 'scatter');
            kinds = new Map();
            const counts = new Map();
            for (const p of placements) { const k = p.biome + '/' + p.name; counts.set(k, (counts.get(k) || 0) + 1); }
            const shadows = glq.shadows > 0;
            /* casters: 0 none, 1 repeated tall props (trees) on daylight biomes only, 2 everything (the 180-call medium budget). */
            const lean = glq.casters === 1 && !(AD.LOOK[biomeId] && AD.LOOK[biomeId].night);
            for (const [key, total] of counts) {
                const [biome, name] = key.split('/');
                if (name === 'gantry') continue;
                const tpl = AD.assets.prop(biome, name);
                if (!tpl) continue;
                const cap = Math.min(total, glq.props) + 2;
                const info = partsOf(tpl, name), parts = info.parts;
                const tallish = (meta[biome] && meta[biome][name] ? meta[biome][name].height : 6) >= 5;
                let kt = 0;
                for (const part of parts) {
                    kt += part.tris;
                    if (part.role === 'face') {
                        part.faces = [];
                        part.signs = name === 'road_sign';
                        if (part.signs) continue;
                        const brandCount = new Map();
                        for (const p of placements) if (p.biome + '/' + p.name === key) { const b = p.brand !== undefined ? p.brand : Math.floor(ihash(Math.round(p.s), 11) * AD.BRAND_COUNT); p.brand = b; brandCount.set(b, (brandCount.get(b) || 0) + 1); }
                        for (const [b, n] of brandCount) {
                            const mesh = new T.InstancedMesh(part.geo, brandMaterial(part.mat, b), Math.min(n, glq.props) + 2);
                            mesh.count = 0; mesh.visible = false; mesh.frustumCulled = false; mesh.name = 'face-' + name;
                            mesh.instanceMatrix.setUsage(T.DynamicDrawUsage);
                            group.add(mesh);
                            part.faces[b] = { mesh: mesh, count: 0 };
                        }
                    } else {
                        const mesh = new T.InstancedMesh(part.geo, part.mat, cap);
                        mesh.count = 0; mesh.visible = false; mesh.frustumCulled = false; mesh.name = name;
                        mesh.instanceMatrix.setUsage(T.DynamicDrawUsage);
                        mesh.setColorAt(0, colour.setRGB(1, 1, 1));
                        mesh.instanceColor.setUsage(T.DynamicDrawUsage);
                        mesh.castShadow = shadows && glq.casters !== 0 && (glq.casters === 2 ? (tallish || total < 12) : (lean && tallish && total >= 12)) && part.role !== 'rotor';
                        mesh.receiveShadow = shadows;
                        group.add(mesh);
                        part.mesh = mesh;
                    }
                }
                kinds.set(key, { key: key, name: name, biome: biome, parts: parts, minY: info.minY, tris: kt, cast: shadows && tallish, spin: parts.some(p => p.role === 'rotor') });
            }
            for (const p of placements) {
                const k = kinds.get(p.biome + '/' + p.name);
                const draft = biomeId === 'harbour' ? (WATER_DRAFT[p.name] || 0) : 0;
                p.groundOffset = -(k ? k.minY : 0) * p.sy + (p.footSink || 0) + draft * p.sy;
            }
            /* Sign pool: sixteen meshes with their own material and a clone of the atlas texture. */
            const signKind = kinds.get('shared/road_sign');
            if (signKind) {
                const part = signKind.parts.find(p => p.role === 'face');
                if (part) {
                    signTex = new T.CanvasTexture(drawSignAtlas(tr));
                    signTex.colorSpace = T.SRGBColorSpace; signTex.flipY = false; signTex.anisotropy = 4;
                    /* One material and one atlas; each sign mesh owns a 4 vertex geometry whose uv window picks its cell. */
                    const mat = part.mat.clone();
                    mat.map = signTex; mat.vertexColors = false; mat.color.set(0xffffff);
                    mat.emissive = new T.Color(0xffffff); mat.emissiveMap = signTex; mat.emissiveIntensity = night ? 0.9 : 0.1;
                    owned.push(mat);
                    for (let i = 0; i < 16; i++) {
                        const geo = part.geo.clone();
                        const mesh = new T.Mesh(geo, mat);
                        mesh.matrixAutoUpdate = false; mesh.visible = false; mesh.frustumCulled = false;
                        group.add(mesh);
                        signPool.push({ mesh: mesh, geo: geo, uv0: Float32Array.from(part.geo.attributes.uv.array), cell: -1 });
                    }
                    texOwn.push(signTex);
                }
            }
            buildGantry();
            buildUfo();
            lastView = null;
        }

        function buildGantry() {
            const p = placements.find(o => o.special === 'gantry');
            const tpl = AD.assets.trackPiece('gantry');
            if (!p || !tpl) return;
            const g = tpl.clone(true);
            g.position.set(p.x, p.gy + (p.groundOffset || 0), p.y); g.rotation.y = p.yaw; g.scale.set(1, 1, p.sz);
            const lights = [];
            g.traverse(o => {
                if (o.isMesh) {
                    const mats = Array.isArray(o.material) ? o.material : [o.material];
                    const next = mats.map(m => { const c = m.clone(); owned.push(c); return c; });
                    o.material = Array.isArray(o.material) ? next : next[0];
                    if (/^light_/.test(o.name) || (o.parent && /^light_/.test(o.parent.name))) lights.push(o);
                    o.castShadow = glq.shadows > 0 && glq.casters === 2;
                }
            });
            group.add(g);
            gantry = { group: g, lights: lights, state: '' };
        }
        const LAMP = { off: ['#2a1414', 0.25], red: ['#ff2418', 3.2], green: ['#35ff5a', 3.6] };
        function setGantry(race) {
            if (!gantry) return;
            let state = 'off0';
            if (race) {
                if (race.state === 'countdown') state = 'red' + clamp(Math.floor((3.2 - race.countdown) / 0.64), 0, 5);
                else if (race.state === 'running' && race.time < 2.6) state = 'green';
            }
            if (state === gantry.state) return;
            gantry.state = state;
            const lit = state === 'green' ? 5 : state.indexOf('red') === 0 ? +state.slice(3) : 0;
            const kind = state === 'green' ? 'green' : 'red';
            gantry.lights.forEach(o => {
                const hit = /_(\d)/.exec(/^light_/.test(o.name) ? o.name : (o.parent && o.parent.name) || 'light_1');
                const idx = (hit ? +hit[1] : 1) - 1;
                const spec = idx < lit ? LAMP[kind] : LAMP.off;
                const mats = Array.isArray(o.material) ? o.material : [o.material];
                for (const m of mats) { m.emissive.set(spec[0]); m.emissiveIntensity = spec[1]; m.color.set(spec[0]); }
            });
        }

        function buildUfo() {
            if (biomeId !== 'salt') return;
            const fs = featuresFor(track);
            const sp = fs && fs.setpieces && fs.setpieces[0];
            const tpl = AD.assets.prop('salt', 'ufo');
            if (!sp || !tpl) return;
            const g = tpl.clone(true);
            g.traverse(o => {
                if (o.isMesh) {
                    const mats = Array.isArray(o.material) ? o.material : [o.material];
                    const next = mats.map(m => { const c = m.clone(); if (c.emissiveIntensity) c.emissiveIntensity *= 1.4; owned.push(c); return c; });
                    o.material = Array.isArray(o.material) ? next : next[0];
                    o.castShadow = glq.shadows > 0;
                }
            });
            g.scale.setScalar(6.5);
            g.position.set(sp.x, 56, sp.y);
            group.add(g);
            ufo = { group: g, x: sp.x, y: sp.y };
            emitters.push({ name: 'ufo', x: sp.x, y: sp.y, gy: 0 });
        }

        /* ---- streaming ------------------------------------------------------------------------- */
        const tmpE = new T.Euler(0, 0, 0, 'YXZ');
        function compose(p, extraY) {
            tmpP.set(p.x, p.gy + (extraY || 0), p.y);
            if (p.tiltX || p.tiltZ) { tmpE.set(p.tiltX || 0, p.yaw, p.tiltZ || 0, 'YXZ'); tmpQ.setFromEuler(tmpE); }
            else tmpQ.setFromAxisAngle(UP, p.yaw);
            tmpS.set(p.sx, p.sy, p.sz);
            return place.compose(tmpP, tmpQ, tmpS);
        }
        const AUTUMN = [[1.28, 0.95, 0.5], [1.15, 0.78, 0.42], [1.05, 1.1, 0.6]];
        function tintFor(p) {
            const v = 0.8 + p.tint * 0.34;
            if (biomeId === 'forest' && (p.name === 'oak' || p.name === 'birch') && p.tint > 0.62) { const a = AUTUMN[Math.floor((p.tint - 0.62) / 0.38 * 2.99)]; return colour.setRGB(a[0], a[1], a[2]); }
            if (biomeId === 'desert' && p.name.indexOf('cactus') === 0) return colour.setRGB(0.8 + p.tint * 0.3, 1.0, 0.75 + p.tint * 0.25);
            return colour.setRGB(v, v * (0.96 + p.tint * 0.08), v * (0.98 + (1 - p.tint) * 0.06));
        }
        const live = [];
        function refill(view) {
            const sel = AD.filterProps(placements, cells, view, { radius: glq.propRadius, cap: glq.props, cone: 0.42, near: 70 });
            for (const k of kinds.values()) for (const part of k.parts) { part.count = 0; if (part.faces) for (const f of part.faces) if (f) f.count = 0; }
            live.length = 0;
            const budget = (glq.triangles || 500000) * 0.3;
            let used = 0, n = 0, signs = 0;
            for (const id of sel) {
                const p = placements[id];
                if (p.name === 'gantry') continue;
                const k = kinds.get(p.biome + '/' + p.name);
                if (!k) continue;
                const cost = k.tris * (k.cast ? 2 : 1);
                if (used + cost > budget) break;
                used += cost; n++;
                compose(p, p.groundOffset || 0);
                const tint = tintFor(p);
                for (const part of k.parts) {
                    M.multiplyMatrices(place, part.local);
                    if (part.role === 'face') {
                        if (part.signs) {
                            const slot = signs++;
                            const pool = signPool[slot];
                            if (!pool) continue;
                            pool.mesh.matrix.copy(M); pool.mesh.matrixWorldNeedsUpdate = true; pool.mesh.visible = true;
                            const cell = (p.sign || 0) % 16;
                            if (pool.cell !== cell) {
                                const uv = pool.geo.attributes.uv;
                                for (let i = 0; i < uv.count; i++) {
                                    uv.setXY(i, ((cell % 4) + pool.uv0[i * 2]) * 0.25, (Math.floor(cell / 4) + pool.uv0[i * 2 + 1]) * 0.25);
                                }
                                uv.needsUpdate = true;
                                pool.cell = cell;
                            }
                            continue;
                        }
                        const f = part.faces[p.brand];
                        if (f && f.count < f.mesh.instanceMatrix.count) f.mesh.setMatrixAt(f.count++, M);
                    } else if (part.mesh && part.count < part.mesh.instanceMatrix.count) {
                        if (part.role === 'rotor') live.push({ kind: 'rotor', part: part, slot: part.count, base: place.clone() });
                        else if (p.name === 'grandstand' && (part.role === 'crowd' || part.role === 'crowd_arms')) {
                            live.push({ kind: 'crowd', part, slot: part.count, base: M.clone(), stand: p, offset: 0 });
                        }
                        else if (WATER_ONLY[p.name]) live.push({ kind: 'boat', part: part, slot: part.count, base: place.clone(), phase: p.x * 0.07 + p.y * 0.11 });
                        part.mesh.setMatrixAt(part.count, M);
                        part.mesh.setColorAt(part.count, tint);
                        part.count++;
                    }
                }
            }
            for (let i = signs; i < signPool.length; i++) signPool[i].mesh.visible = false;
            visibleKinds = 0;
            for (const k of kinds.values()) {
                for (const part of k.parts) {
                    if (part.mesh) {
                        part.mesh.count = part.count; part.mesh.visible = part.count > 0;
                        part.mesh.instanceMatrix.needsUpdate = true;
                        if (part.mesh.instanceColor) part.mesh.instanceColor.needsUpdate = true;
                        if (part.count > 0) visibleKinds++;
                    }
                    if (part.faces) for (const f of part.faces) if (f) { f.mesh.count = f.count; f.mesh.visible = f.count > 0; f.mesh.instanceMatrix.needsUpdate = true; if (f.count) visibleKinds++; }
                }
            }
            selected = n; tris = used;
            feedEmitters(view);
        }
        function feedEmitters(view) {
            const sky = scene.userData && scene.userData.agdriftSky;
            if (!sky || !sky.emitters || !emitters.length) return;
            const list = [];
            for (const p of emitters) {
                const d = Math.hypot(p.x - view.x, p.y - view.y);
                if (d < 280) list.push({ d: d, e: p });
            }
            list.sort((a, b) => a.d - b.d);
            const waterY = biomeId === 'harbour' ? WATER_Y : -0.25;
            sky.emitters(list.slice(0, 12).map(o => {
                const spec = EMIT[o.e.name];
                return { x: o.e.x, z: o.e.y, h: spec[1] + Math.max(0, (o.e.gy || 0) - waterY), color: spec[0] };
            }));
        }

        const api = {
            get grandstands() { return stands; },
            get placements() { return placements; },
            update(pose, race) {
                const now = typeof performance !== 'undefined' ? performance.now() : 0;
                const dt = prevNow ? clamp((now - prevNow) / 1000, 0, 0.1) : 0;
                prevNow = now;
                if (!reduced) { clock += dt; wind.value = clock; }
                const e = pose.eye, tg = pose.target || e;
                let fx = tg[0] - e[0], fy = tg[2] - e[2];
                const fl = Math.hypot(fx, fy) || 1;
                fx /= fl; fy /= fl;
                const moved = lastView ? Math.hypot(e[0] - lastView.x, e[2] - lastView.y) : 1e9;
                const turned = lastView ? fx * lastView.fx + fy * lastView.fy : -1;
                if (!lastView || moved > 12 || turned < 0.96 || now - lastTime > 250) {
                    lastView = { x: e[0], y: e[2], fx: fx, fy: fy };
                    lastTime = now;
                    refill(lastView);
                }
                if (live.length) {
                    const ang = reduced ? 0.6 : clock * 1.2;
                    for (const l of live) {
                        if (l.kind === 'crowd') {
                            const cue = l.stand.reaction;
                            const offset = cue && !reduced && race ? AD.roadsidePulse(cue, race.time) * (cue.kind === 'drift' ? .95 : -.52) : 0;
                            if (offset === l.offset) continue;
                            l.offset = offset; M.copy(l.base); M.elements[13] += offset;
                        } else if (l.kind === 'boat') {
                            if (reduced) continue;
                            spinM.makeRotationZ(Math.sin(clock * 1.6 + l.phase) * 0.045);
                            M.multiplyMatrices(l.base, l.part.local).multiply(spinM);
                            M.elements[13] += Math.sin(clock * 2.1 + l.phase) * 0.08;
                        } else {
                            spinM.makeRotationX(ang);
                            M.multiplyMatrices(l.base, l.part.local).multiply(spinM);
                        }
                        l.part.mesh.setMatrixAt(l.slot, M);
                        l.part.mesh.instanceMatrix.needsUpdate = true;
                    }
                }
                if (ufo && !reduced) {
                    ufo.group.rotation.y = clock * 0.5;
                    ufo.group.position.y = 56 + Math.sin(clock * 0.7) * 3;
                    ufo.group.rotation.z = Math.sin(clock * 0.9) * 0.05;
                }
                setGantry(race || null);
            },
            rebuild() { build(); },
            setQuality(next) {
                if (!next) return;
                const changed = next.props !== glq.props || next.shadows !== glq.shadows || next.casters !== glq.casters;
                glq = next;
                if (changed) build();
            },
            setReduced(on) { reduced = !!on; },
            stats() { return { props: selected, triangles: tris, kinds: visibleKinds, placements: placements.length, standIns: 0 }; },
            dispose() { dispose(true); }
        };

        function dispose(final) {
            for (const k of kinds.values()) {
                for (const part of k.parts) {
                    if (part.mesh) part.mesh.dispose();
                    if (part.faces) for (const f of part.faces) if (f) f.mesh.dispose();
                }
            }
            for (const c of group.children.slice()) group.remove(c);
            for (const m of owned) m.dispose();
            for (const s of signPool) s.geo.dispose();
            for (const m of brandMats) if (m) m.dispose();
            for (const x of brandTex) x.dispose();
            for (const x of texOwn) x.dispose();
            owned = []; signPool = []; brandMats = []; brandTex = []; texOwn = []; kinds = new Map(); gantry = null; ufo = null; signTex = null;
            if (final) group.removeFromParent();
        }

        build();
        return api;
    };
})();
