/* Runenpfad village rooms: the enterable buildings of Pathrest (tavern with cellar, kitchen and upstairs; shop with store and attic; smithy with
 * forge and coal cellar; wardens' hall with archive and bell room) and Edda's cottage. DOM-free data.
 * Appends to RunenpfadPlaceRooms.PLACES / ROOMS (place-rooms.js); residents are defined in folk.js, their talk in folk-story.js and ember-story.js. */
(function (root) {
    'use strict';

    const Rooms = root.RunenpfadPlaceRooms;
    if (!Rooms) return;
    const PLACES = Rooms.PLACES;
    const ROOMS = Rooms.ROOMS;

    const TAV = 'tdp-tavern.';
    const TRD = 'tdp-trade.';
    const HOM = 'tdp-home.';
    const HSE = 'tdp-house.';
    const CAV = 'tdp-cave.';
    const OUT = { to: 'out' };
    const WARM = { r: 170, color: '#ffcf80', flicker: true };
    const LAMP = { r: 150, color: '#ffd9a0', flicker: false };
    const COAL = { r: 170, color: '#ffb060', flicker: true };

    function place(id, facade, rooms) {
        PLACES[id] = { id: id, nameKey: 'runenpfad.place.' + id, kind: 'house', entry: rooms[0], facade: facade, rooms: rooms, world: null };
    }

    function room(id, o) {
        const r = {
            theme: o.theme || 'wood', floor: o.floor || 0, floorAlt: o.floorAlt == null ? 1 : o.floorAlt, wall: o.wall || 2, dark: o.dark || 0,
            music: o.music || 'in_cozy', map: o.map, things: o.things, exits: o.exits || {}, spawn: o.spawn
        };
        if (o.orders != null) r.orders = o.orders;
        ROOMS[id] = r;
    }

    function guest(at, npc, cond) { return { at: at, npc: npc, meeting: npc + '_tavern', if: cond }; }

    // ---- tavern: the Rusty Rune (main room, cellar, kitchen and the rooms upstairs) ----

    place('tavern', { clip: 'fac-village-a.f0', h: 257, dx: 3 }, ['tavern_main', 'tavern_cellar', 'tavern_kitchen', 'tavern_upstairs']);
    room('tavern_main', {
        dark: 0.12, spawn: 'S', exits: { X: OUT, T: { to: 'tavern_cellar', at: 'e' }, J: { to: 'tavern_kitchen', at: 'd' }, V: { to: 'tavern_upstairs', at: 'd' } },
        map: [
            '############J#######',
            '#.F..k...N..j.K..n.#',
            '#......B..B...GYY..#',
            '#...u.............p#',
            '#..h..........h...vV',
            '#.PL..D....C..L....#',
            '#..h.....R....h..eT#',
            '#.............oi...#',
            '#.w...sOs..sOs...x.#',
            '#..................#',
            '#p..a..l.S..m...z..#',
            '#########X##########'
        ],
        things: [
            { at: 'F', prop: TAV + 'p7' }, { at: 'k', prop: TAV + 'p14' }, { at: 'B', prop: TAV + 'p4' }, { at: 'K', prop: TAV + 'p6' },
            { at: 'n', prop: TAV + 'p12' }, { at: 'p', prop: TAV + 'p15' }, { at: 'h', prop: TAV + 'p3' }, { at: 'L', prop: TAV + 'p0' },
            { at: 'R', prop: TAV + 'p11' }, { at: 'T', prop: TAV + 'p13' }, { at: 's', prop: TAV + 'p2' }, { at: 'O', prop: TAV + 'p1' },
            { at: 'N', npc: 'innkeeper', meeting: 'innkeeper_home' }, { at: 'D', npc: 'bard', meeting: 'bard_home' },
            { at: 'P', npc: 'patron', meeting: 'patron_home' }, { at: 'C', npc: 'child', meeting: 'child_home' },
            // Guests who show up as the story moves on (meeting ids <npc>_tavern in folk-story.js).
            guest('w', 'witch', { questDone: 'witch_mushrooms' }),
            guest('x', 'chief', { any: [{ flag: 'chief.respect' }, { flag: 'chief.paid' }, { flag: 'chief.charmed' }] }),
            guest('m', 'merchant', { progress: 11 }),
            guest('a', 'seer', { flag: 'seer.friend' }),
            guest('z', 'wizard', { progress: 21 }),
            guest('l', 'alchemist', { flag: 'alchemist.revealed' }),
            // Gundar always sits at his dice table (meeting gambler_home in journal-story.js).
            { at: 'G', npc: 'gambler', meeting: 'gambler_home' }, { at: 'Y', use: 'dice:gambler' },
            // The ember chapter: Bram drinks here once the cave is open, the kobold siblings after the dragon (meetings in ember-story.js).
            { at: 'u', npc: 'bram', meeting: 'bram_home', if: { step: 2 } },
            { at: 'o', npc: 'nokk', meeting: 'nokk_home', if: { step: 5 } }, { at: 'i', npc: 'pip', meeting: 'pip_home', if: { step: 5 } }
        ]
    });
    room('tavern_cellar', {
        floor: 1, floorAlt: 0, wall: 3, dark: 0.6, music: 'in_cave', exits: { U: { to: 'tavern_main', at: 'e' } },
        map: [
            '##############',
            '#bb..K....bbC#',
            '#.r..........#',
            '#.....r...b..#',
            '#..l......l..#',
            '#.r.....r....#',
            '#.....nm.....#',
            '#.r..........#',
            '#.......b..e.#',
            '###########U##'
        ],
        things: [
            { at: 'b', prop: TAV + 'p5' }, { at: 'K', prop: TAV + 'p6' }, { at: 'U', prop: TAV + 'p13' }, { at: 'r', mob: 'rat' },
            { at: 'l', light: { r: 170, color: '#ffb060', flicker: true } },
            { at: 'C', chest: 'tavern_cellar', loot: { coins: 30, give: 'potion' }, locked: 'clear' },
            // Hiding from the dragon's trouble: the kobold siblings sit among the barrels while the cave is open.
            { at: 'n', npc: 'nokk', meeting: 'nokk_home', if: { step: 2, not: { step: 5 } } },
            { at: 'm', npc: 'pip', meeting: 'pip_home', if: { step: 2, not: { step: 5 } } }
        ]
    });
    // The kitchen behind the bar: Berta's hearth, the pantry and the table where a meal is made (cook window).
    room('tavern_kitchen', {
        dark: 0.1, exits: { D: { to: 'tavern_main', at: 'j' } },
        map: [
            '##############',
            '#.PP.H..hh.WW#',
            '#,,,.......o.#',
            '#,,,.g.......#',
            '#............#',
            '#.zOz....T...#',
            '#.....,,.YY..#',
            '#.l.........o#',
            '#......d.....#',
            '#######D######'
        ],
        things: [
            { at: 'P', prop: HOM + 'p2' }, { at: 'H', prop: HOM + 'p2' }, { at: 'h', prop: HSE + 'p15' }, { at: 'W', prop: HOM + 'p3' },
            { at: 'g', prop: TAV + 'p11' }, { at: 'z', prop: TAV + 'p2' }, { at: 'O', prop: TAV + 'p1' }, { at: 'T', prop: HOM + 'p1' },
            { at: 'l', light: LAMP }, { at: 'o', pot: 'pot' },
            // The cook station: the table above, the two use cells in front of it.
            { at: 'Y', use: 'cook', prompt: 'cook' }
        ]
    });
    // Upstairs: Finn's room, Gundar's room and a locked-away third room that Edda once rented; its key lies in her attic.
    room('tavern_upstairs', {
        dark: 0.15, exits: { D: { to: 'tavern_main', at: 'v' } },
        map: [
            '####################',
            '#.T.lfw#klCh#xo.nl.#',
            '#b.....#....#..z..b#',
            '#..g...#.gc.#..g...#',
            '####.####G#####.####',
            '#p.l...B........l.p#',
            '#.,,,,,,,,,,,,,,,,.#',
            '#..................#',
            '#p...,...d......,.p#',
            '#########D##########'
        ],
        things: [
            // Finn's room.
            { at: 'T', prop: HSE + 'p2' }, { at: 'f', sign: 'finn_song', look: 'scroll' }, { at: 'w', prop: HSE + 'p1' },
            { at: 'b', prop: HSE + 'p0' }, { at: 'g', prop: HSE + 'p3' }, { at: 'l', light: WARM },
            // Edda's old room behind the gate: the trunk, a shelf, the cat on the rug.
            { at: 'k', prop: HSE + 'p7' }, { at: 'h', prop: HSE + 'p15' }, { at: 'c', prop: HSE + 'p10' },
            { at: 'C', chest: 'edda_chest', loot: { coins: 60, item: { rune_attune: 1 } }, look: HSE + 'p8' },
            { at: 'G', gate: 'edda_room', open: { item: 'room_key' }, stay: true },
            // Gundar's room: crates, a barrel of ale, his note.
            { at: 'x', prop: TAV + 'p10' }, { at: 'o', prop: TAV + 'p5' }, { at: 'n', sign: 'gundar_note', look: 'plaque' }, { at: 'z', prop: TAV + 'p2' },
            // The landing.
            { at: 'p', prop: TAV + 'p15' }, { at: 'B', prop: TAV + 'p3' }
        ]
    });

    // ---- shop: Tilda's (with the store and the attic) ----

    place('shop', { clip: 'fac-village-a.f1', h: 262, dx: -54 }, ['shop_main', 'shop_store', 'shop_attic']);
    room('shop_main', {
        dark: 0.05, spawn: 'S', exits: { X: OUT, W: { to: 'shop_store', at: 'e' } },
        map: [
            '##############',
            '#P...N...P..f#',
            '#...B..c.....#',
            '#..........M.#',
            '#.z.....Y...z#',
            'Wr...........#',
            '#.....Y...v.b#',
            '#x...........#',
            '#.....S......#',
            '######X#######'
        ],
        things: [
            { at: 'P', prop: TRD + 'p1' }, { at: 'B', prop: TRD + 'p0' }, { at: 'c', prop: TRD + 'p11' }, { at: 'z', prop: TRD + 'p9' },
            { at: 'Y', prop: TRD + 'p10' }, { at: 'f', prop: TRD + 'p12' }, { at: 'b', prop: TAV + 'p5' }, { at: 'x', prop: TAV + 'p10' },
            { at: 'N', npc: 'shopkeeper', meeting: 'shopkeeper_home' }, { at: 'v', npc: 'villager_f', meeting: 'villager_f_home' },
            { at: 'M', npc: 'mira', meeting: 'mira_home', if: { any: [{ flag: 'mira.found' }, { step: 5 }] } }
        ]
    });
    // The store behind the counter: four crates shut a nook with the lucky amulet; push one up the column to get in.
    room('shop_store', {
        dark: 0.1, exits: { E: { to: 'shop_main', at: 'r' }, U: { to: 'shop_attic', at: 'd' } },
        map: [
            '##############',
            '#P.P.zz.#...C#',
            '#b......#...l#',
            '#..,,,..#....#',
            '#.Y......KKKK#',
            '#zz......,,b.#',
            '#z....,...Y..#',
            'Uu....c.....eE',
            '#.x..l.......#',
            '##############'
        ],
        things: [
            { at: 'P', prop: TRD + 'p1' }, { at: 'z', prop: TRD + 'p9' }, { at: 'b', prop: TAV + 'p5' }, { at: 'Y', prop: TRD + 'p10' },
            { at: 'c', prop: TRD + 'p11' }, { at: 'x', prop: TAV + 'p10' }, { at: 'l', light: WARM },
            { at: 'K', block: 'crate' },
            { at: 'C', chest: 'shop_shelf', loot: { coins: 50, item: { lucky_amulet: 1 } } }
        ]
    });
    room('shop_attic', {
        dark: 0.3, exits: { D: { to: 'shop_store', at: 'u' } },
        map: [
            '############',
            '#j.k..z.h..#',
            '#.....n...C#',
            '#....c....w#',
            '#...g.....j#',
            '#.l........#',
            '#..d.......#',
            '###D########'
        ],
        things: [
            { at: 'j', prop: HSE + 'p9' }, { at: 'k', prop: HSE + 'p7' }, { at: 'z', prop: HSE + 'p6' }, { at: 'h', prop: HSE + 'p15' },
            { at: 'c', prop: HSE + 'p10' }, { at: 'w', prop: HSE + 'p5' }, { at: 'g', prop: HSE + 'p3' }, { at: 'l', light: LAMP },
            { at: 'n', sign: 'tilda_gear', look: 'plaque' },
            { at: 'C', chest: 'shop_attic', loot: { coins: 30, give: 'rune_frost' } }
        ]
    });

    // ---- smithy: Hakon's forge (with the forge room and the coal cellar) ----

    place('smithy', { clip: 'fac-village-a.f2', h: 250, dx: -65 }, ['smithy_main', 'smithy_forge', 'smithy_coal']);
    room('smithy_main', {
        wall: 3, dark: 0.28, spawn: 'S', exits: { X: OUT, E: { to: 'smithy_forge', at: 'w' } },
        map: [
            '##############',
            '#.F.b..W..a..#',
            '#............#',
            '#..AN....v...#',
            '#............#',
            '#.k......w..rE',
            '#............#',
            '#x..........x#',
            '#.....S......#',
            '######X#######'
        ],
        things: [
            { at: 'F', prop: TRD + 'p5' }, { at: 'b', prop: TRD + 'p6' }, { at: 'W', prop: TRD + 'p2' }, { at: 'a', prop: TRD + 'p3' },
            { at: 'A', prop: TRD + 'p4' }, { at: 'k', prop: TRD + 'p8' }, { at: 'w', prop: TRD + 'p7' }, { at: 'x', prop: TAV + 'p10' },
            { at: 'N', npc: 'smith', meeting: 'smith_home' }, { at: 'v', npc: 'villager_m', meeting: 'villager_m_home' }
        ]
    });
    // The forge room: the great furnace with the use cells in front (forge window), a trapdoor down to the coal.
    room('smithy_forge', {
        wall: 3, dark: 0.22, exits: { W: { to: 'smithy_main', at: 'r' }, T: { to: 'smithy_coal', at: 'e' } },
        map: [
            '##############',
            '#.R.bF...cQ..#',
            '#...,YY,A,...#',
            '#...,,,,,,...#',
            '#............#',
            '#.R......k...#',
            'Ww...........#',
            '#..........eT#',
            '#.l.........l#',
            '##############'
        ],
        things: [
            { at: 'R', prop: HOM + 'p8' }, { at: 'b', prop: TRD + 'p6' }, { at: 'F', prop: HOM + 'p7' }, { at: 'c', prop: HOM + 'p7' },
            { at: 'Q', prop: HOM + 'p6' }, { at: 'A', prop: HOM + 'p4' }, { at: 'k', prop: TRD + 'p8' }, { at: 'T', prop: HOM + 'p15' },
            { at: 'l', light: COAL },
            { at: 'Y', use: 'forge', prompt: 'forge' }
        ]
    });
    // The coal cellar: rats in the heaps, a cracked wall that hides a small chest.
    room('smithy_coal', {
        theme: 'cave', floor: 1, floorAlt: 0, wall: 3, dark: 0.6, music: 'in_cave', exits: { U: { to: 'smithy_forge', at: 'e' } },
        map: [
            '##############',
            '#.c.l..c...kC#',
            '#..#c......###',
            '#...,..c..r..#',
            '#..##,......r#',
            '#.....,.r....#',
            '#.c........l.#',
            '#..e.........#',
            '###U##########'
        ],
        things: [
            { at: 'c', prop: HOM + 'p7' }, { at: 'l', light: COAL }, { at: 'r', mob: 'rat' }, { at: 'k', crack: true },
            { at: 'C', chest: 'smithy_coal', loot: { coins: 40, give: 'potion' }, locked: 'clear' }
        ]
    });

    // ---- wardens' hall: Elder Runa, the chronicle lectern and five rune pedestals (with the archive and the bell room) ----

    place('hall', { clip: 'fac-village-b.f0', h: 300, dx: -3 }, ['hall_main', 'hall_archive', 'hall_bell']);
    room('hall_main', {
        wall: 3, dark: 0.22, spawn: 'S', exits: { X: OUT, W: { to: 'hall_archive', at: 'x' }, U: { to: 'hall_bell', at: 'd' } },
        map: [
            '##################',
            '#.f..fBBLNH.f..f.#',
            '#................#',
            'WG..............uU',
            '#.a..b..c..g..h..#',
            '#................#',
            '#...C......yEz...#',
            '#.q...........q..#',
            '#.......R........#',
            '#.......S........#',
            '#.p............p.#',
            '########X#########'
        ],
        things: [
            { at: 'f', prop: TRD + 'p12' }, { at: 'L', prop: TRD + 'p13' }, { at: 'H', prop: TRD + 'p15' }, { at: 'q', prop: TAV + 'p3' },
            { at: 'R', prop: TAV + 'p11' }, { at: 'p', prop: TAV + 'p15' },
            { at: 'N', npc: 'elder', meeting: 'elder_home' },
            // One pedestal per world boss defeated (progress 6, 11, 16, 21) and one for the finale; unlit ones stay dark.
            { at: 'a', prop: TRD + 'p14', lit: { progress: 6 } }, { at: 'b', prop: TRD + 'p14', lit: { progress: 11 } },
            { at: 'c', prop: TRD + 'p14', lit: { progress: 16 } }, { at: 'g', prop: TRD + 'p14', lit: { progress: 21 } },
            { at: 'h', prop: TRD + 'p14', lit: { flag: 'epilogue.paid' } },
            // The task board, and the guests of the epilogue: Cog after the finale, Edda and her recruits after the observatory.
            { at: 'B', use: 'board' },
            { at: 'C', npc: 'automaton', meeting: 'automaton_hall', if: { flag: 'epilogue.paid' } },
            { at: 'E', npc: 'edda', meeting: 'edda_home', if: { flag: 'obs.done' } },
            { at: 'y', npc: 'recruit_f', meeting: 'recruit_f_home', if: { flag: 'obs.done' } },
            { at: 'z', npc: 'recruit_m', meeting: 'recruit_m_home', if: { flag: 'obs.done' } },
            // The archive door opens once the first tremor has been felt (step 1); the bell room is always open.
            { at: 'G', gate: 'archive', open: { step: 1 }, stay: true }
        ]
    });
    // The archive: two stone tablets, a ring of four shelves to pull in the order the sun walks, a niche behind them.
    room('hall_archive', {
        wall: 3, dark: 0.3, exits: { X: { to: 'hall_main', at: 'G' } },
        map: [
            '################',
            '##C#kk..N..kk..#',
            '##G#,.........E#',
            '#.....D....h...#',
            '#...t......u...#',
            '#W............xX',
            '#..k..,,,,,,k..#',
            '#.p....g.....p.#',
            '#.......S......#',
            '################'
        ],
        things: [
            { at: 'k', prop: HOM + 'p12' }, { at: 'D', prop: HOM + 'p14' }, { at: 'g', prop: HSE + 'p3' }, { at: 'p', prop: TAV + 'p15' },
            { at: 't', sign: 'tab_1', tablet: 1, look: 'stand' }, { at: 'u', sign: 'tab_2', tablet: 2, look: 'stand' },
            { at: 'h', sign: 'archive_hint', look: 'scroll' },
            { at: 'E', brazier: 'shelf_e', set: 'books', look: 'shelf' }, { at: 'S', brazier: 'shelf_s', set: 'books', look: 'shelf' },
            { at: 'W', brazier: 'shelf_w', set: 'books', look: 'shelf' }, { at: 'N', brazier: 'shelf_n', set: 'books', look: 'shelf' },
            { at: 'G', gate: 'niche', open: { order: 'books' }, stay: true },
            { at: 'C', chest: 'hall_niche', loot: { coins: 60, give: 'potion' } }
        ],
        orders: { books: ['shelf_e', 'shelf_s', 'shelf_w', 'shelf_n'] }
    });
    // The bell room: the old bell on its frame and the rope; ringing it is a story beat (use 'bell').
    room('hall_bell', {
        wall: 3, dark: 0.2, exits: { D: { to: 'hall_main', at: 'u' } },
        map: [
            '##########',
            '#.l....l.#',
            '#a.,B,,..#',
            '#..,YY,..#',
            '#..,YY,.x#',
            '#..,,,R..#',
            '#........#',
            '#o.......#',
            '#...d....#',
            '####D#####'
        ],
        things: [
            { at: 'a', prop: HOM + 'p8' }, { at: 'R', prop: HOM + 'p11' }, { at: 'x', prop: TAV + 'p10' },
            { at: 'o', prop: TAV + 'p5' }, { at: 'l', light: WARM },
            { at: 'Y', use: 'bell', prompt: 'ring' }
        ]
    });

    // ---- Edda's cottage (the old seer's house at the edge of the village) ----

    place('edda_house', { clip: 'fac-deep.f3', h: 260, dx: 0 }, ['edda_home', 'edda_study', 'edda_attic']);
    room('edda_home', {
        dark: 0.08, spawn: 'S', exits: { X: OUT, E: { to: 'edda_study', at: 'w' }, U: { to: 'edda_attic', at: 'd' } },
        map: [
            '######U#######',
            '#.w..hu.h.P..#',
            '#b.........a.#',
            '#..m.........#',
            '#....T...s...#',
            '#....g......eE',
            '#.,,,,,,,,,..#',
            '#.n.......P..#',
            '#.l...S....l.#',
            '######X#######'
        ],
        things: [
            { at: 'w', prop: HSE + 'p1' }, { at: 'h', prop: HSE + 'p15' }, { at: 'P', prop: HSE + 'p14' }, { at: 'b', prop: HSE + 'p0' },
            { at: 'a', prop: HSE + 'p4' }, { at: 'g', prop: HSE + 'p3' }, { at: 'T', prop: HSE + 'p2' }, { at: 's', prop: HSE + 'p5' },
            { at: 'l', light: WARM },
            { at: 'm', sign: 'edda_letter_1', look: 'scroll' }, { at: 'n', sign: 'edda_letter_2', look: 'scroll' }
        ]
    });
    // The study: the map table (use 'map'), the locked desk and shelves of notes.
    room('edda_study', {
        dark: 0.12, exits: { W: { to: 'edda_home', at: 'e' } },
        map: [
            '############',
            '#kk.M..kkc.#',
            '#..........#',
            '#...YY.....#',
            '#.,,,,,,...#',
            '#.........n#',
            'Ww.....D...#',
            '#.........C#',
            '#.l.......l#',
            '############'
        ],
        things: [
            { at: 'k', prop: HOM + 'p12' }, { at: 'c', prop: HSE + 'p13' }, { at: 'M', prop: HSE + 'p7' }, { at: 'D', prop: HOM + 'p14' },
            { at: 'l', light: LAMP }, { at: 'n', sign: 'edda_notes', look: 'scroll' },
            { at: 'C', chest: 'edda_desk', loot: { coins: 50, give: 'potion' }, look: HSE + 'p12', key: 'desk_key' },
            { at: 'Y', use: 'map', prompt: 'read' }
        ]
    });
    room('edda_attic', {
        dark: 0.4, exits: { D: { to: 'edda_home', at: 'u' } },
        map: [
            '############',
            '#j.z.h..z.j#',
            '#.o.....C..#',
            '#..g.......#',
            '#.l....j..o#',
            '#z.......l.#',
            '#..d.......#',
            '###D########'
        ],
        things: [
            { at: 'j', prop: HSE + 'p9' }, { at: 'z', prop: HSE + 'p6' }, { at: 'h', prop: HSE + 'p15' }, { at: 'g', prop: HSE + 'p3' },
            { at: 'l', light: LAMP }, { at: 'o', pot: 'pot' },
            { at: 'C', chest: 'edda_attic', loot: { coins: 30, item: { room_key: 1 } }, look: HSE + 'p8' }
        ]
    });
})(typeof window !== 'undefined' ? window : globalThis);
