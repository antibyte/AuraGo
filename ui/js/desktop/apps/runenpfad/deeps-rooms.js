/* Runenpfad round 11: "the deeps", the twelve rooms of the Ember Wyrm's cave under Pathrest, plus the room hook that remembers which rooms the hero has seen
 * and the map table use. DOM-free data. Appends to RunenpfadPlaceRooms.PLACES / ROOMS (place-rooms.js); the things are interior-things.js, Brandur and his
 * brood are brandur.js / deep-mobs.js. Entered only from the hub well door. Lava `~` is solid for walkers and open for flyers and shots (place-rooms.js). */
(function (root) {
    'use strict';

    const Rooms = root.RunenpfadPlaceRooms;
    if (!Rooms) return;
    const PLACES = Rooms.PLACES;
    const ROOMS = Rooms.ROOMS;

    const MIN = 'tdp-mine.';
    const CAV = 'tdp-cave.';
    const LAIR = 'tdp-lair.';
    const ID = 'deeps';
    const ROOM_IDS = ['dp_shaft', 'dp_rails', 'dp_camp', 'dp_bronze', 'dp_roots', 'dp_crystal', 'dp_hall', 'dp_bridge', 'dp_vents', 'dp_nest', 'dp_hoard', 'dp_lair'];
    // Grid cells of the map panel.
    const MAP = {
        dp_camp: [0, 0], dp_shaft: [1, 0], dp_rails: [2, 0], dp_bronze: [3, 0],
        dp_crystal: [1, 1], dp_roots: [2, 1], dp_hall: [3, 1], dp_bridge: [4, 1],
        dp_nest: [2, 2], dp_vents: [3, 2], dp_hoard: [4, 2], dp_lair: [5, 2]
    };
    const FLAG_A = 'dr.visit_a';
    const FLAG_B = 'dr.visit_b';
    const FLAG_MAP = 'dr.map';
    const AMBER = { r: 200, color: '#ffb060', flicker: true };
    const CYAN = { r: 170, color: '#6fe8ff', flicker: false };
    const EMBER = { r: 210, color: '#ff7a2a', flicker: true };

    function pushEvent(sim, type, detail) {
        if (root.RunenpfadPhysics) root.RunenpfadPhysics.pushEvent(sim, type, detail);
    }

    function room(id, o) {
        const r = {
            theme: o.theme, floor: o.floor || 0, floorAlt: o.floorAlt == null ? 1 : o.floorAlt, wall: o.wall || 2, dark: o.dark || 0,
            music: o.music || 'in_cave', map: o.map, things: o.things, exits: o.exits || {}, spawn: o.spawn
        };
        ['guardian', 'slippery', 'orders', 'big', 'view'].forEach(function (k) { if (o[k] != null) r[k] = o[k]; });
        ROOMS[id] = r;
    }

    // A dart that fires along row/column `dir` every `every` seconds.
    function dart(at, dir, every, offset) { return { at: at, dart: dir, every: every, offset: offset }; }
    // A fire vent: harmless for 1.7 s, warns 0.5 s, then erupts for 1.0 s.
    function vent(at, offset) { return { at: at, spikes: { period: 3.2, up: 1.0, offset: offset }, look: 'vent' }; }

    // ---- 1. the shaft: the way down, a bedroll and the chalk marks of a lost scout ----

    room('dp_shaft', {
        theme: 'mine', dark: 0.45, music: 'in_mine', spawn: 'S',
        exits: { U: { to: 'out' }, E: { to: 'dp_rails', at: 'w' }, W: { to: 'dp_camp', at: 'e' } },
        map: [
            '####U#########',
            '#.bplc..p.k###',
            '#...S........#',
            '#.,,,,,,,,,..#',
            '#.,,,,P,,,,..#',
            'Ww..,,,,,,,.eE',
            '#T.....iiiii.#',
            '#r.........b.#',
            '##....s..yk.##',
            '##############'
        ],
        things: [
            { at: 'p', prop: MIN + 'p8' }, { at: 'l', prop: MIN + 'p9' }, { at: 'P', prop: MIN + 'p10' }, { at: 'k', prop: MIN + 'p0' },
            { at: 'i', prop: MIN + 'p2' }, { at: 'b', prop: MIN + 'p4' }, { at: 'T', prop: MIN + 'p11' }, { at: 'y', prop: MIN + 'p7' },
            { at: 'r', prop: MIN + 'p12' }, { at: 'r', rest: true },
            { at: 'c', sign: 'bram_chalk' }
        ]
    });

    // ---- 2. the rail gallery: a lever frees the stuck lift down to the roots ----

    room('dp_rails', {
        theme: 'mine', dark: 0.5, music: 'in_mine',
        exits: { W: { to: 'dp_shaft', at: 'e' }, E: { to: 'dp_bronze', at: 'w' }, D: { to: 'dp_roots', at: 'u' } },
        map: [
            '####################',
            '#..f..##..r.f....V.#',
            '#.....#.L,,,.......#',
            '#..k..#..K.........#',
            '#..f............f..#',
            'WwiiiiiiiiiiiiiiiieE',
            '#..K......r..L.....#',
            '##...#r,,,,,,.d..k##',
            '##############g#####',
            '##############D#####'
        ],
        things: [
            { at: 'f', prop: MIN + 'p3' }, { at: 'L', prop: MIN + 'p8' }, { at: 'k', prop: MIN + 'p0' }, { at: 'K', prop: MIN + 'p1' },
            { at: 'i', prop: MIN + 'p2' },
            { at: 'r', mob: 'rat' },
            { at: 'V', lever: 'rails' },
            { at: 'g', gate: 'down', open: { on: ['rails'] }, stay: true }
        ]
    });

    // ---- 3. the kobold camp: bedrolls around a cook fire, and a clan charm in the chest ----

    room('dp_camp', {
        theme: 'mine', dark: 0.5, music: 'in_mine',
        exits: { E: { to: 'dp_shaft', at: 'w' } },
        map: [
            '##############',
            '##.f.,,...f.##',
            '#.bb.x.......#',
            '#..b....F....#',
            '#.,,.........#',
            '#..q....,,,.eE',
            '#.....o.,,,..#',
            '#..##.....q..#',
            '#C.....#...y##',
            '##############'
        ],
        things: [
            { at: 'f', prop: MIN + 'p3' }, { at: 'b', prop: MIN + 'p12' }, { at: 'F', prop: MIN + 'p13' }, { at: 'q', prop: MIN + 'p14' },
            { at: 'y', prop: MIN + 'p7' }, { at: 'o', prop: MIN + 'p4' },
            { at: 'x', sign: 'kobold_scratch' },
            { at: 'C', chest: 'dp_camp', loot: { coins: 40, item: { clan_charm: 1 } } }
        ]
    });

    // ---- 4. the bronze gallery: two cracked plugs close the way to the bell bronze; the pot keeps a key ----

    room('dp_bronze', {
        theme: 'mine', dark: 0.6, music: 'in_mine',
        exits: { W: { to: 'dp_rails', at: 'e' } },
        map: [
            '##############',
            '#####.,,.#####',
            '#f,.#.b..#.y.#',
            '#.,.#.,,.#..C#',
            'Ww..x.v..z...#',
            '#...#.,,.#...#',
            '#p.T#..ob#.p.#',
            '#####q...#####',
            '##############'
        ],
        things: [
            { at: 'f', prop: MIN + 'p3' }, { at: 'p', prop: MIN + 'p8' }, { at: 'y', prop: MIN + 'p7' }, { at: 'T', prop: MIN + 'p11' },
            { at: 'q', prop: MIN + 'p4' }, { at: 'v', prop: MIN + 'p2' },
            { at: 'x', crack: true }, { at: 'z', crack: true },
            { at: 'b', mob: 'bat' },
            { at: 'o', pot: 'pot', drop: 'key' },
            { at: 'C', chest: 'dp_bronze', loot: { coins: 20, item: { bell_bronze: 1 } } }
        ]
    });

    // ---- 5. the root hall: three veins light up bottom, middle, top; the vault gate answers ----

    room('dp_roots', {
        theme: 'cave', dark: 0.85, music: 'in_cave',
        orders: { veins: ['vein_low', 'vein_mid', 'vein_top'] },
        exits: { U: { to: 'dp_rails', at: 'd' }, W: { to: 'dp_crystal', at: 'e' }, E: { to: 'dp_hall', at: 'w' } },
        map: [
            '##########U#########',
            '########..u...######',
            '###.....lh..r...####',
            '##.m.yx..,,#.T..####',
            '##..#...#,,.....m###',
            '##.o#..m#......l.###',
            'Ww...,,......M...geE',
            '##..r,,........y.###',
            '##.m.b,....#,,,..###',
            '###....#.m.x,B,b####',
            '#######...o...r#####',
            '####################'
        ],
        things: [
            { at: 'y', prop: CAV + 'p3' }, { at: 'm', prop: CAV + 'p5' }, { at: 'r', prop: CAV + 'p9' }, { at: 'o', prop: CAV + 'p0' }, { at: 'l', prop: CAV + 'p4' },
            { at: 'x', mob: 'spider' }, { at: 'b', mob: 'bat' },
            { at: 'h', sign: 'vein_hint' },
            { at: 'B', brazier: 'vein_low', set: 'veins' }, { at: 'M', brazier: 'vein_mid', set: 'veins' }, { at: 'T', brazier: 'vein_top', set: 'veins' },
            { at: 'g', gate: 'vault', open: { order: 'veins' }, stay: true }
        ]
    });

    // ---- 6. the crystal nook: two crystal blocks onto two plates open the way to Mira ----

    room('dp_crystal', {
        theme: 'cave', dark: 0.6, music: 'in_cave',
        exits: { E: { to: 'dp_roots', at: 'w' } },
        map: [
            '##################',
            '###########yN.####',
            '###########...####',
            '############g#####',
            '##########.....q##',
            '#a....k...,,,.y..#',
            '#########.,,,...eE',
            '#########.p.l,,,.#',
            '#########..x.,,,.#',
            '#b....k...y..,,,.#',
            '###########..q.###',
            '##################'
        ],
        things: [
            { at: 'p', prop: MIN + 'p6' }, { at: 'y', prop: MIN + 'p7' }, { at: 'q', prop: CAV + 'p3' }, { at: 'l', light: CYAN },
            { at: 'x', mob: 'spider' },
            { at: 'k', block: 'crystal' }, { at: 'a', plate: 'pa' }, { at: 'b', plate: 'pb' },
            { at: 'g', gate: 'nook', open: { on: ['pa', 'pb'] }, stay: true },
            { at: 'N', npc: 'mira', meeting: 'mira_deeps', if: { step: 2, notFlag: 'mira.found', not: { step: 5 } } }
        ]
    });

    // ---- 7. the hall of wardens: tablets 3 to 5, mummies, and a key gate with the last tablet behind it ----

    room('dp_hall', {
        theme: 'cave', dark: 0.55, music: 'in_cave',
        exits: { W: { to: 'dp_roots', at: 'e' }, E: { to: 'dp_bridge', at: 'w' } },
        map: [
            '####################',
            '###n...n####n...n###',
            '###.xbl.####.lby.###',
            '###....m####.....###',
            '##....T..##..t...###',
            '##........l.......##',
            'Ww...f..,,,,..f...eE',
            '##.......l.m......##',
            '###...t......T....##',
            '##########g#########',
            '#########z.C########',
            '####################'
        ],
        things: [
            { at: 'T', prop: LAIR + 'p6' }, { at: 't', prop: LAIR + 'p7' }, { at: 'f', prop: LAIR + 'p4' }, { at: 'b', prop: LAIR + 'p8' },
            { at: 'n', prop: CAV + 'p13' }, { at: 'l', light: CYAN },
            { at: 'm', mob: 'mummy' },
            { at: 'x', sign: 'tab_3', look: 'tablet', tablet: 3 }, { at: 'y', sign: 'tab_4', look: 'tablet', tablet: 4 },
            { at: 'g', gate: 'wardens', open: { key: true }, stay: true },
            { at: 'z', sign: 'tab_5', look: 'tablet', tablet: 5 },
            { at: 'C', chest: 'dp_hall', loot: { coins: 60, item: { desk_key: 1 } } }
        ]
    });

    // ---- 8. the root bridge: a chasm, three darts and a lever for the shortcut home ----

    room('dp_bridge', {
        theme: 'cave', dark: 0.6, music: 'in_cave',
        exits: { W: { to: 'dp_hall', at: 'e' }, S: { to: 'dp_shaft', at: 's' }, D: { to: 'dp_vents', at: 'u' } },
        map: [
            '######################',
            '######          ######',
            '######          #...##',
            '##...#          #ryv.#',
            '#.l.f.          .....#',
            '#.....          ...b.#',
            'Ww.y.p...............#',
            '#.,,,...........q....#',
            '#.,,,.          ....gS',
            '##..b.          #l...#',
            '######          #.d.##',
            '##################D###'
        ],
        things: [
            { at: 'l', prop: CAV + 'p13' }, { at: 'f', prop: MIN + 'p3' }, { at: 'y', prop: MIN + 'p7' },
            dart('p', 'right', 2.8, 0), dart('q', 'left', 3.2, 1.1), dart('r', 'down', 2.6, 0.6),
            { at: 'b', mob: 'bat' },
            { at: 'v', lever: 'short' },
            { at: 'g', gate: 'short', open: { on: ['short'] }, stay: true }
        ]
    });

    // ---- 9. the vent field: lava channels cut the cave in three; the fords are timed by fire vents ----

    room('dp_vents', {
        theme: 'glut', dark: 0.35, music: 'in_cave',
        exits: { U: { to: 'dp_bridge', at: 'd' }, W: { to: 'dp_nest', at: 'e' }, E: { to: 'dp_hoard', at: 'w' } },
        map: [
            '##########U###########',
            '##...~~...u....~~...##',
            '##...~~.o......1....##',
            '#.o..~~....s....2.o..#',
            '#..#.~~k.#.....~~,,,.#',
            '#...s~~..#..m.k~~,,,.#',
            'Ww...~~,,.3....~~...eE',
            '#....~~,,....4.~~....#',
            '#..#.~~4..k.##.~~s...#',
            '#.r...2.m...r.o~~.#..#',
            '#..m.1.....#...~~.km.#',
            '##...~~...~~...~~...##',
            '######################'
        ],
        things: [
            { at: 'o', prop: LAIR + 'p2' }, { at: 's', prop: LAIR + 'p9' }, { at: 'r', prop: LAIR + 'p8' }, { at: 'k', prop: LAIR + 'p10' },
            vent('1', 0), vent('2', 1.6), vent('3', 0.8), vent('4', 2.4),
            { at: 'm', mob: 'emberling' }
        ]
    });

    // ---- 10. the nest: Brandur's brood, a dragon scale, and a chest for who clears the room ----

    room('dp_nest', {
        theme: 'glut', dark: 0.45, music: 'in_cave',
        exits: { E: { to: 'dp_vents', at: 'w' } },
        map: [
            '##################',
            '###.....##....#.##',
            '#..N..d....r.....#',
            '#.,,,,......m....#',
            '#.,,,,...........#',
            '#.....s..~.,,,...#',
            '#.S.....~~~,,,..eE',
            '#...c.r..~.,,,...#',
            '#....o.....,,,k..#',
            '#.C.s...d...m....#',
            '##....##.......###',
            '##################'
        ],
        things: [
            { at: 'N', prop: LAIR + 'p3' }, { at: 'o', prop: LAIR + 'p2' }, { at: 's', prop: LAIR + 'p9' }, { at: 'r', prop: LAIR + 'p8' },
            { at: 'c', prop: LAIR + 'p1' }, { at: 'k', prop: LAIR + 'p10' },
            { at: 'd', mob: 'drakeling' }, { at: 'm', mob: 'emberling' },
            { at: 'S', chest: 'dp_scale', loot: { item: { dragon_scale: 1 } }, look: LAIR + 'p15' },
            { at: 'C', chest: 'dp_nest', loot: { coins: 80, give: 'potion' }, locked: 'clear' }
        ]
    });

    // ---- 11. the hoard: two stone blocks open the seal before the lair; a last rest and the sixth tablet ----

    room('dp_hoard', {
        theme: 'glut', dark: 0.4, music: 'in_cave',
        exits: { W: { to: 'dp_vents', at: 'e' }, E: { to: 'dp_lair', at: 'a' } },
        map: [
            '##################',
            '##.....###########',
            '#.r...s...k..a####',
            '#.,,.H...#########',
            '#.,,.,,,.###..T###',
            'Ww,,#,,,.....c.geE',
            '#.,,.,,,.###..q###',
            '#.,,.H...#########',
            '#.qo..s...k..b####',
            '##.....###########',
            '##################'
        ],
        things: [
            { at: 'H', prop: LAIR + 'p0' }, { at: 'c', prop: LAIR + 'p1' }, { at: 'o', prop: LAIR + 'p2' }, { at: 'q', prop: LAIR + 'p10' }, { at: 's', prop: LAIR + 'p9' },
            { at: 'r', prop: LAIR + 'p12' }, { at: 'r', rest: true },
            { at: 'k', block: 'stone' }, { at: 'a', plate: 'sa' }, { at: 'b', plate: 'sb' },
            { at: 'g', gate: 'seal', open: { on: ['sa', 'sb'] }, stay: true },
            { at: 'T', sign: 'tab_6', look: 'tablet', tablet: 6 }
        ]
    });

    // ---- 12. the lair: a wide floor inside a rim of lava, four pillar braziers, and Brandur asleep at G ----

    room('dp_lair', {
        theme: 'glut', dark: 0.35, music: 'in_lair', big: true, view: 15, guardian: 'brandur',
        exits: { B: { to: 'dp_hoard', at: 'e' } },
        map: [
            '############################',
            '#~~~~~~~~~~~~~~~~~~~~~~~~~~#',
            '#~~~....n.....K....n....~~~#',
            '#~~..o..............o....~~#',
            '#~.H..c....x..G..y...cH...~#',
            '#~...e...............e....~#',
            '#~........1......2........~#',
            '#~.s....................s.~#',
            '#~..o...........o.........~#',
            '#~.z.......o...........o..~#',
            '#~......................z.~#',
            '#~..c.....3......4.....c..~#',
            '#~.H.e...............eH...~#',
            '#~....s......c.......s....~#',
            '#~~....o..l......l.o.....~~#',
            '#~~~....n..........n....~~~#',
            '#~~~~~~~~~~~~.a.~~~~~~~~~~~#',
            '##############B#############'
        ],
        things: [
            { at: 'H', prop: LAIR + 'p0' }, { at: 'o', prop: LAIR + 'p2' }, { at: 's', prop: LAIR + 'p9' }, { at: 'c', prop: LAIR + 'p1' },
            { at: 'e', prop: LAIR + 'p10' }, { at: 'z', prop: LAIR + 'p8' }, { at: 'n', prop: LAIR + 'p13' },
            { at: 'x', prop: LAIR + 'p6' }, { at: 'y', prop: LAIR + 'p7' }, { at: 'l', light: EMBER },
            { at: '1', brazier: 'pillar_1', look: 'pillar', timed: 8 }, { at: '2', brazier: 'pillar_2', look: 'pillar', timed: 8 },
            { at: '3', brazier: 'pillar_3', look: 'pillar', timed: 8 }, { at: '4', brazier: 'pillar_4', look: 'pillar', timed: 8 },
            { at: 'G', mob: 'brandur' },
            { at: 'K', use: 'brandur' }
        ]
    });

    function extend(placeId, ids) {
        const p = PLACES[placeId];
        if (p) ids.forEach(function (r) { if (p.rooms.indexOf(r) < 0) p.rooms.push(r); });
    }

    PLACES[ID] = {
        id: ID, nameKey: 'runenpfad.place.deeps', kind: 'cave', entry: 'dp_shaft', world: null,
        facade: { clip: 'fac-deep.f0', h: 170, dx: 0 }, rooms: []
    };
    extend(ID, ROOM_IDS);

    // ---- the map: which rooms the hero has seen (two flag bit masks), which are known by Edda's map ----

    function flagOf(profile, key) {
        const f = profile && profile.npcFlags;
        return f && typeof f[key] === 'number' ? f[key] : 0;
    }

    function bitOf(roomId) {
        const i = ROOM_IDS.indexOf(roomId);
        return i < 0 ? null : (i < 6 ? { key: FLAG_A, bit: 1 << i } : { key: FLAG_B, bit: 1 << (i - 6) });
    }

    // { roomId: true } for every room of the deeps the profile has visited.
    function visited(profile) {
        const out = {};
        ROOM_IDS.forEach(function (id) {
            const b = bitOf(id);
            if (flagOf(profile, b.key) & b.bit) out[id] = true;
        });
        return out;
    }

    // Marks a room as visited (campaign only). Returns true when a new bit was set.
    function markVisited(sim, roomId) {
        const N = root.RunenpfadNpcs;
        const b = bitOf(roomId);
        if (!b || !N || !sim || sim.mode !== 'campaign' || !sim.profile) return false;
        const cur = flagOf(sim.profile, b.key);
        if (cur & b.bit) return false;
        return !!N.setFlag(sim, b.key, cur | b.bit);
    }

    // Undirected links of the exits that run both ways (the one-way shortcut is not drawn).
    function links() {
        const out = [];
        ROOM_IDS.forEach(function (a) {
            const ex = ROOMS[a].exits;
            Object.keys(ex).forEach(function (m) {
                const b = ex[m].to;
                if (b === 'out' || ROOM_IDS.indexOf(b) <= ROOM_IDS.indexOf(a)) return;
                const back = ROOMS[b].exits;
                if (Object.keys(back).some(function (k) { return back[k].to === a; })) out.push([a, b]);
            });
        });
        return out;
    }

    // null outside the deeps; otherwise the rooms with grid cell and state for the map panel.
    function mapState(sim) {
        const it = sim && sim.interior;
        if (!it || it.placeId !== ID) return null;
        const seen = visited(sim.profile);
        const revealed = flagOf(sim.profile, FLAG_MAP) > 0;
        return {
            rooms: ROOM_IDS.map(function (id) {
                return { id: id, x: MAP[id][0], y: MAP[id][1], visited: !!seen[id], known: revealed || !!seen[id], current: it.roomId === id };
            }),
            links: links(),
            revealed: revealed
        };
    }

    // ---- registrations: the room hook and the map table ----

    const HOOK_ID = 'deeps_map';
    const hooks = root.RunenpfadRoomHooks || (root.RunenpfadRoomHooks = []);
    if (!hooks.some(function (x) { return x.id === HOOK_ID; })) {
        hooks.push({
            id: HOOK_ID,
            setup: function (sim, it) { if (it && it.placeId === ID) markVisited(sim, it.roomId); },
            step: function () { /* nothing to do per tick */ }
        });
    }

    const Things = root.RunenpfadInteriorThings;
    if (Things && Things.USES) {
        Things.USES.map = {
            prompt: 'read',
            use: function (sim) {
                const N = root.RunenpfadNpcs;
                const D = root.RunenpfadDialogue;
                if (N && sim.mode === 'campaign') N.setFlag(sim, FLAG_MAP, 1);
                sim.notice = 'runenpfad.notice.map_read';
                pushEvent(sim, 'in_read');
                if (D && D.openVirtual) D.openVirtual(sim, 'lore', 'edda_map');
            }
        };
    }

    root.RunenpfadDeepsRooms = {
        PLACE_ID: ID,
        ROOM_IDS: ROOM_IDS,
        MAP: MAP,
        FLAGS: { visitA: FLAG_A, visitB: FLAG_B, map: FLAG_MAP },
        visited: visited,
        markVisited: markVisited,
        links: links,
        mapState: mapState
    };
})(typeof window !== 'undefined' ? window : globalThis);
