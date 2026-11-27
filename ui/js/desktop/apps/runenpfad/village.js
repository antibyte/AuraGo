/* Runenpfad village hub: the peaceful side-view street of Pathrest (campaign stage 0), its doors, decor and ambient villagers. DOM-free.
 * No enemies, hazards, key, portal, gems or boss; levels.js builds the stage from STAGE and this zone module. */
(function (root) {
    'use strict';

    const ZONE_IDS = ['village'];
    const ZONE_TILES = 65;
    const BARK_RANGE = 110;
    const BARK_EVERY = 3.5;

    const STAGE = { id: 0, worldIndex: 0, stageInWorld: 0, nameKey: 'runenpfad.village.name', parTime: 0, zones: ['village'], enemies: [], hub: true };

    // Door centres in tiles from the zone start (at least 9 apart). Facades are measured from the final art.
    const DOORS = [
        { t: 5, placeId: 'deeps', kind: 'cave', off: true, facade: { clip: 'fac-deep.f0', h: 170, dx: 0 } },
        { t: 14, placeId: 'edda_house', kind: 'house', facade: { clip: 'fac-deep.f3', h: 252, dx: -14 } },
        { t: 24, placeId: 'tavern', kind: 'house', facade: { clip: 'fac-village-a.f0', h: 257, dx: 3 } },
        { t: 34, placeId: 'shop', kind: 'house', facade: { clip: 'fac-village-a.f1', h: 262, dx: -54 } },
        { t: 43, placeId: 'hall', kind: 'house', facade: { clip: 'fac-village-b.f0', h: 300, dx: -3 } },
        { t: 52, placeId: 'smithy', kind: 'house', facade: { clip: 'fac-village-a.f2', h: 250, dx: -65 } },
        { t: 61, placeId: 'arch', kind: 'arch', facade: { clip: 'fac-village-b.f2', h: 250, dx: 0 } }
    ];

    // Decor without a door: [tile, clip, height px, flip, id?]. The ids 'well' and 'barn' change with the ember chapter (ember.js).
    const DECOR = [
        [5, 'fac-village-b.f3', 150, false, 'well'],
        [18.9, 'fac-deep.f2', 196, false, 'barn'],
        [1.5, 'vil-props.p1', 70, false],
        [2.8, 'vil-props.p2', 110, false],
        [7.4, 'vil-props.p0', 150, false],
        [9.2, 'vil-props.p4', 60, false],
        [10.6, 'vil-props.p1', 70, true],
        [17, 'vil-props.p5', 100, false],
        [28.4, 'vil-props.p0', 150, false],
        [38.4, 'vil-props.p6', 150, false],
        [47, 'vil-props.p3', 100, true],
        [56.4, 'vil-props.p7', 150, false],
        [63.6, 'vil-props.p4', 60, true],
        [64.3, 'vil-props.p1', 70, false]
    ];

    // Ambient villagers: tile, sprite sheet, facing, bark keys (runenpfad.village.bark.1..6).
    const BARKS = [
        { t: 37.2, sheet: 'npc-villager', facing: 1, lines: ['runenpfad.village.bark.1', 'runenpfad.village.bark.2', 'runenpfad.village.bark.3'] },
        { t: 58.8, sheet: 'npc-guard', facing: -1, lines: ['runenpfad.village.bark.4', 'runenpfad.village.bark.5', 'runenpfad.village.bark.6'] }
    ];

    function has(zone) { return ZONE_IDS.indexOf(zone) >= 0; }

    // Flat street of 65 tiles: doors, decor and barks go into ctx.extras (copied onto the stage). No rng draws.
    function build(zone, ctx) {
        const T = ctx.TILE;
        const cx = ctx.cursorX;
        const y = ctx.groundY * T;
        ctx.fillGround(cx, ZONE_TILES, ctx.groundY);
        const extras = ctx.extras;
        extras.hub = true;
        extras.hubProps = DECOR.map(function (d) { return { id: d[4] || null, clip: d[1], x: (cx + d[0]) * T, y: y, h: d[2], flip: !!d[3], hidden: false, steam: false }; });
        extras.barks = BARKS.map(function (b) {
            return { x: (cx + b.t) * T, y: y, sheet: b.sheet, facing: b.facing, lines: b.lines.slice(), near: false, line: 0, t: 0 };
        });
        if (!extras.doors) extras.doors = [];
        const rooms = root.RunenpfadPlaceRooms && root.RunenpfadPlaceRooms.PLACES;
        DOORS.forEach(function (d) {
            if (d.kind !== 'arch' && !(rooms && rooms[d.placeId])) return;
            const x = (cx + d.t) * T;
            extras.doors.push({
                id: 'door_' + extras.doors.length, placeId: d.placeId, kind: d.kind, x: x, y: y, w: 56,
                zoneX0: x - 4.5 * T, zoneX1: x + 4.5 * T, near: false, cool: 0, off: !!d.off, steam: false,
                facade: { clip: d.facade.clip, h: d.facade.h, dx: d.facade.dx }
            });
        });
        return ZONE_TILES;
    }

    // While the hero stands within 110 px a villager is `near` and its line advances every 3.5 s.
    function stepBarks(sim, dt) {
        if (root.RunenpfadEmber) root.RunenpfadEmber.stepHub(sim, dt);
        const stage = sim.stage;
        if (!stage || !stage.barks) return;
        const px = sim.player.x;
        stage.barks.forEach(function (b) {
            const near = Math.abs(px - b.x) <= BARK_RANGE;
            if (!near) {
                b.near = false;
                b.t = 0;
                return;
            }
            b.near = true;
            b.t += dt;
            if (b.t >= BARK_EVERY) {
                b.t -= BARK_EVERY;
                b.line = (b.line + 1) % b.lines.length;
            }
        });
    }

    root.RunenpfadVillage = {
        STAGE: STAGE,
        ZONE_IDS: ZONE_IDS,
        ZONE_TILES: ZONE_TILES,
        has: has,
        build: build,
        stepBarks: stepBarks
    };
})(typeof window !== 'undefined' ? window : globalThis);
