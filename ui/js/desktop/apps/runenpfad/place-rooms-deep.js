/* Runenpfad deep rooms: the five guardian chambers behind the caves and the four rooms of the Old Observatory. DOM-free data.
 * Appends to RunenpfadPlaceRooms.PLACES / ROOMS (place-rooms.js); the engine for the things is interior-things.js, the guardians are
 * interior-bosses.js. Every cave room has an arrival marker `a` next to its back exit `B`; place-rooms.js points the way in at it. */
(function (root) {
    'use strict';

    const Rooms = root.RunenpfadPlaceRooms;
    if (!Rooms) return;
    const PLACES = Rooms.PLACES;
    const ROOMS = Rooms.ROOMS;

    const CAV = 'tdp-cave.';
    const RUI = 'tdp-ruin.';
    const STR = 'tdp-stars.';
    const LOR = 'tdp-lore.';
    const GLOW = { r: 200, color: '#ffb060', flicker: true };
    const POOL = { r: 170, color: '#9ad8ff', flicker: false };
    const BLUE = { r: 190, color: '#6fd0ff', flicker: false };
    const CHEST_LOOT = { coins: 80, give: 'potion' };

    function room(id, o) {
        const r = {
            theme: o.theme, floor: o.floor || 0, floorAlt: o.floorAlt == null ? 1 : o.floorAlt, wall: o.wall || 2, dark: o.dark || 0,
            music: o.music || 'in_cave', map: o.map, things: o.things, exits: o.exits || {}, spawn: o.spawn
        };
        ['guardian', 'slippery', 'orders'].forEach(function (k) { if (o[k] != null) r[k] = o[k]; });
        ROOMS[id] = r;
    }

    // Adds a room to the list of an existing place once (the module may be evaluated again by a test harness).
    function extend(placeId, roomId) {
        const p = PLACES[placeId];
        if (p && p.rooms.indexOf(roomId) < 0) p.rooms.push(roomId);
    }

    // The nook behind every guardian: the page lectern and the chest, shut by a gate until the room is clear.
    function nook(gate, lectern, chest, page, id) {
        return [
            { at: gate, gate: 'nook', open: { clear: true }, stay: true },
            { at: lectern, page: page },
            { at: chest, chest: id, loot: CHEST_LOOT, locked: 'clear' }
        ];
    }

    // ---- beast den: the brood chamber ----

    room('beast_den_b', {
        theme: 'cave', dark: 0.55, guardian: 'brood', spawn: null,
        exits: { B: { to: 'beast_den', at: 'x' } },
        map: [
            '######################',
            '##.l.####q.........q##',
            '##.h.###.............#',
            '###c####..........G..#',
            '#.....q#.............#',
            '#p....r#.............#',
            '#......#.............#',
            '#....................#',
            '#.n..p.##...........##',
            '#......#######g#######',
            '#.....r#####....m#####',
            '#..a...#####L...C#####',
            '###B##################'
        ],
        things: [
            { at: 'B', light: GLOW }, { at: 'l', light: GLOW },
            { at: 'p', pot: 'pot' }, { at: 'r', pot: 'crate' }, { at: 'n', pot: 'barrel' },
            { at: 'c', crack: true }, { at: 'h', pickup: 'heart' },
            { at: 'q', prop: CAV + 'p13' }, { at: 'm', prop: CAV + 'p5' },
            { at: 'G', mob: 'brood' }
        ].concat(nook('g', 'L', 'C', 2, 'beast_den_b'))
    });

    // ---- crystal grotto: two crystal blocks onto two plates, then the weaver ----

    room('crystal_grotto_b', {
        theme: 'cave', floor: 1, floorAlt: 0, wall: 3, dark: 0.6, guardian: 'weaver',
        exits: { B: { to: 'crystal_grotto', at: 'x' } },
        map: [
            '######################',
            '#...s..#####L...C#####',
            '#e....f#####.....#####',
            '#......#######n#######',
            '#......##...........##',
            '#.k..k.#.............#',
            '#......#...........G.#',
            '#..l...w.............#',
            '#......w......l......#',
            '#......w.............#',
            '#..c...#.............#',
            '#......#.............#',
            '##.a.####...........##',
            '###B##################'
        ],
        things: [
            { at: 'c', prop: CAV + 'p3' }, { at: 'l', light: BLUE },
            { at: 'k', block: 'crystal' }, { at: 'e', plate: 'pa' }, { at: 'f', plate: 'pb' },
            { at: 's', sign: 'grotto_mark' },
            { at: 'w', gate: 'seal', open: { on: ['pa', 'pb'] }, stay: true },
            { at: 'G', mob: 'weaver' }
        ].concat(nook('n', 'L', 'C', 4, 'crystal_grotto_b'))
    });

    // ---- sun tomb: five braziers in the order of the mural, then the tomb ward ----

    room('sun_tomb_c', {
        theme: 'tomb', floor: 1, floorAlt: 0, wall: 3, dark: 0.5, guardian: 'tombward',
        orders: { realms: ['wood', 'root', 'sand', 'ice', 'brass'] },
        exits: { B: { to: 'sun_tomb_b', at: 'x' } },
        map: [
            '##B###################',
            '#.a.#####S..........l#',
            '#....p.q..i.w.g.r.u..#',
            '#....p.q..I.W.Z.R.U..#',
            '#...#####............#',
            '##############k#######',
            '#...............######',
            '#...............######',
            '#...............#....#',
            '#.......l.......n.l..#',
            '#..G............#....#',
            '#...............#L..C#',
            '#...............######',
            '######################'
        ],
        things: [
            { at: 'l', light: GLOW },
            { at: 'S', sign: 'tomb_mural' },
            { at: 'p', spikes: { period: 2.8, up: 0.8, offset: 0 } }, { at: 'q', spikes: { period: 2.8, up: 0.8, offset: 1.4 } },
            { at: 'I', brazier: 'ice', set: 'realms' }, { at: 'W', brazier: 'wood', set: 'realms' }, { at: 'Z', brazier: 'brass', set: 'realms' },
            { at: 'R', brazier: 'root', set: 'realms' }, { at: 'U', brazier: 'sand', set: 'realms' },
            { at: 'i', prop: RUI + 'p8' }, { at: 'w', prop: CAV + 'p10' }, { at: 'g', prop: RUI + 'p12' },
            { at: 'r', prop: CAV + 'p5' }, { at: 'u', prop: RUI + 'p1' },
            { at: 'k', gate: 'tomb', open: { order: 'realms' }, stay: true },
            { at: 'G', mob: 'tombward' }
        ].concat(nook('n', 'L', 'C', 6, 'sun_tomb_c'))
    });

    // ---- ice cave: a sliding route to the plate, then the frost heart over rough ground ----

    room('ice_cave_b', {
        theme: 'ice', dark: 0.5, guardian: 'frostheart', slippery: true,
        exits: { B: { to: 'ice_cave', at: 'x' } },
        map: [
            '##B###################',
            '#,a,s#.k....i..##,,,,#',
            '#,,,,#..l......##L,,C#',
            '#,,,,,.i.....l.##,,,,#',
            '#,,,,#i..iei...##,,,,#',
            '#,,,,#.........##,,,,#',
            '##########g#######n###',
            '#c,,,,,,,,,,,,,,,,,,c#',
            '#,,,,,,,,,,,,,,,,,,,,#',
            '#,,,,,,,,,,,,,,,,,,,,#',
            '#,,,,,,,,,,,,,,,,,,,,#',
            '#,,G,,,,,,,,,,,,,,,,,#',
            '#c,,,,,,,,,,,,,,,,,,c#',
            '######################'
        ],
        things: [
            { at: 'i', prop: RUI + 'p8' }, { at: 'c', prop: RUI + 'p11' }, { at: 's', sign: 'ice_note' }, { at: 'l', light: POOL },
            { at: 'k', block: 'ice' }, { at: 'e', plate: 'floe' },
            { at: 'g', gate: 'floe', open: { on: ['floe'] }, stay: true },
            { at: 'G', mob: 'frostheart' }
        ].concat(nook('n', 'L', 'C', 8, 'ice_cave_b'))
    });

    // ---- boiler room: valves shut the vents, darts, then the boiler ward in the wide hall ----

    room('boiler_room_c', {
        theme: 'works', dark: 0.45, guardian: 'boilerward',
        exits: { B: { to: 'boiler_room_b', at: 'x' } },
        map: [
            '##B###################',
            '#.a.V####WD###########',
            '#...S.11....22...#####',
            '#.....11.l..22...#####',
            '#....######E##...#####',
            '##############...#####',
            '#................#####',
            '#................#####',
            '#................#...#',
            '#.......l........n...#',
            '#..G.............#...#',
            '#................#L.C#',
            '#v....b.........g#####',
            '######################'
        ],
        things: [
            { at: 'S', sign: 'boiler_note', look: 'scroll' }, { at: 'l', light: GLOW },
            { at: 'V', lever: 'valve_a' }, { at: 'W', lever: 'valve_b' },
            { at: '1', spikes: { period: 3, up: 1, offset: 0 }, look: 'vent', when: { off: ['valve_a'] } },
            { at: '2', spikes: { period: 3, up: 1, offset: 1.5 }, look: 'vent', when: { off: ['valve_b'] } },
            { at: 'D', dart: 'down', every: 2.8, offset: 0 }, { at: 'E', dart: 'up', every: 3.2, offset: 1.1 },
            { at: 'v', prop: RUI + 'p13' }, { at: 'b', prop: RUI + 'p14' }, { at: 'g', prop: RUI + 'p12' },
            { at: 'G', mob: 'boilerward' }
        ].concat(nook('n', 'L', 'C', 10, 'boiler_room_c'))
    });

    extend('beast_den', 'beast_den_b');
    extend('crystal_grotto', 'crystal_grotto_b');
    extend('sun_tomb', 'sun_tomb_c');
    extend('ice_cave', 'ice_cave_b');
    extend('boiler_room', 'boiler_room_c');

    // ---- the Old Observatory (opened by Cog after the finale) ----

    PLACES.observatory = {
        id: 'observatory', nameKey: 'runenpfad.place.observatory', kind: 'house', entry: 'obs_entry', world: 2,
        facade: { clip: 'fac-observatory.f0', h: 250, dx: 0 }, rooms: ['obs_entry', 'obs_gears', 'obs_lens', 'obs_core'],
        requires: { flag: 'obs.open' }
    };

    room('obs_entry', {
        theme: 'tomb', floor: 1, floorAlt: 0, wall: 3, dark: 0.6, music: 'in_obs', spawn: 'S',
        exits: { D: { to: 'out' }, E: { to: 'obs_gears', at: 'a' } },
        map: [
            '#################',
            '#p.T....Q..H...p#',
            '#..............V#',
            '#...............#',
            '#...............#',
            '#..p....X.....egE',
            '#.....c.........#',
            '#.p.............#',
            '#.......S.....l.#',
            '########D########'
        ],
        things: [
            { at: 'l', light: POOL },
            { at: 'p', pot: 'pot' },
            { at: 'T', prop: STR + 'p8' }, { at: 'H', prop: STR + 'p10' },
            { at: 'c', prop: LOR + 'p11' },
            { at: 'X', sign: 'obs_chart' }, { at: 'Q', sign: 'obs_plaque' },
            { at: 'V', lever: 'door_lever' },
            { at: 'g', gate: 'star_door', open: { on: ['door_lever'] }, stay: true }
        ]
    });

    room('obs_gears', {
        theme: 'works', floor: 1, floorAlt: 0, wall: 3, dark: 0.6, music: 'in_obs',
        exits: { B: { to: 'obs_entry', at: 'e' }, E: { to: 'obs_lens', at: 'a' } },
        map: [
            '####################',
            '#########D##########',
            '#.m.....c.....l....#',
            '#...H.k.......pH...#',
            '#....yyyyyyyyyy..egE',
            'Ba...k........qH..gE',
            '#....yyyyyyyyyy....#',
            '#...H.k.......rH...#',
            '#o.........c....c.l#',
            '############U#######',
            '####################'
        ],
        things: [
            { at: 'm', sign: 'obs_gears' }, { at: 'l', light: POOL },
            { at: 'o', prop: STR + 'p9' }, { at: 'y', prop: RUI + 'p12' }, { at: 'H', prop: RUI + 'p13' },
            { at: 'k', block: 'stone' },
            { at: 'p', plate: 'gear_a' }, { at: 'q', plate: 'gear_b' }, { at: 'r', plate: 'gear_c' },
            { at: 'D', dart: 'down', every: 2.8, offset: 0 }, { at: 'U', dart: 'up', every: 3.4, offset: 1 },
            { at: 'c', mob: 'clockbug' },
            { at: 'g', gate: 'gears', open: { on: ['gear_a', 'gear_b', 'gear_c'] }, stay: true }
        ]
    });

    room('obs_lens', {
        theme: 'works', floor: 1, floorAlt: 0, wall: 3, dark: 0.62, music: 'in_obs',
        orders: { stars: ['boar', 'crown', 'eye', 'wyrm', 'gear'] },
        exits: { B: { to: 'obs_gears', at: 'e' }, E: { to: 'obs_core', at: 'a' } },
        map: [
            '####################',
            '#..................#',
            '#...M....Q....J....#',
            '#...z....q....j...s#',
            '#................egE',
            '#....l........l...gE',
            '#........O.........#',
            '#..................#',
            '#.....N......K.....#',
            'Ba....w......k.....#',
            '#..................#',
            '####################'
        ],
        things: [
            { at: 'l', light: POOL },
            { at: 'O', prop: STR + 'p5' },
            { at: 'j', prop: STR + 'p0' }, { at: 'k', prop: STR + 'p1' }, { at: 'z', prop: STR + 'p2' },
            { at: 'w', prop: STR + 'p3' }, { at: 'q', prop: STR + 'p4' },
            { at: 'J', brazier: 'boar', set: 'stars' }, { at: 'K', brazier: 'crown', set: 'stars' }, { at: 'M', brazier: 'eye', set: 'stars' },
            { at: 'N', brazier: 'wyrm', set: 'stars' }, { at: 'Q', brazier: 'gear', set: 'stars' },
            { at: 's', sign: 'obs_door' },
            { at: 'g', gate: 'inner', open: { order: 'stars', pages: 10 }, stay: true }
        ]
    });

    room('obs_core', {
        theme: 'tomb', floor: 1, floorAlt: 0, wall: 3, dark: 0.6, music: 'in_obs', guardian: 'counter',
        exits: { B: { to: 'obs_lens', at: 'e' } },
        map: [
            '######################',
            '########...KK...######',
            '########.l.KK.C.######',
            '########........######',
            '###########gg#########',
            '#...l................#',
            '#....................#',
            '#.................G..#',
            '#....................#',
            'Ba...................#',
            '#....................#',
            '#....................#',
            '#....................#',
            '######################'
        ],
        things: [
            { at: 'G', mob: 'counter' },
            { at: 'g', gate: 'core', open: { clear: true }, stay: true },
            { at: 'K', use: 'core' },
            { at: 'C', chest: 'obs_core', loot: { coins: 150, give: 'potion' }, locked: 'clear' }
        ]
    });
})(typeof window !== 'undefined' ? window : globalThis);
