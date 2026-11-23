/* Runenpfad village rooms: the four enterable buildings of Pathrest (tavern with cellar, shop, smithy, wardens' hall). DOM-free data.
 * Appends to RunenpfadPlaceRooms.PLACES / ROOMS (place-rooms.js); residents are defined in folk.js, their talk in folk-story.js. */
(function (root) {
    'use strict';

    const Rooms = root.RunenpfadPlaceRooms;
    if (!Rooms) return;
    const PLACES = Rooms.PLACES;
    const ROOMS = Rooms.ROOMS;

    const TAV = 'tdp-tavern.';
    const TRD = 'tdp-trade.';
    const OUT = { to: 'out' };

    function place(id, facade, rooms) {
        PLACES[id] = { id: id, nameKey: 'runenpfad.place.' + id, kind: 'house', entry: rooms[0], facade: facade, rooms: rooms, world: null };
    }

    function room(id, o) {
        ROOMS[id] = {
            theme: 'wood', floor: o.floor || 0, floorAlt: o.floorAlt == null ? 1 : o.floorAlt, wall: o.wall || 2, dark: o.dark || 0,
            music: o.music || 'in_cozy', map: o.map, things: o.things, exits: o.exits || {}, spawn: o.spawn
        };
    }

    function guest(at, npc, cond) { return { at: at, npc: npc, meeting: npc + '_tavern', if: cond }; }

    // ---- tavern: the Rusty Rune (main room and cellar) ----

    place('tavern', { clip: 'fac-village-a.f0', h: 257, dx: 3 }, ['tavern_main', 'tavern_cellar']);
    room('tavern_main', {
        dark: 0.12, spawn: 'S', exits: { X: OUT, T: { to: 'tavern_cellar', at: 'e' } },
        map: [
            '####################',
            '#.F..k...N....K..n.#',
            '#......B..B...GYY..#',
            '#.................p#',
            '#..h..........h....#',
            '#.PL..D....C..L....#',
            '#..h.....R....h..eT#',
            '#..................#',
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
            { at: 'G', npc: 'gambler', meeting: 'gambler_home' }, { at: 'Y', use: 'dice:gambler' }
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
            '#............#',
            '#.r..........#',
            '#.......b..e.#',
            '###########U##'
        ],
        things: [
            { at: 'b', prop: TAV + 'p5' }, { at: 'K', prop: TAV + 'p6' }, { at: 'U', prop: TAV + 'p13' }, { at: 'r', mob: 'rat' },
            { at: 'l', light: { r: 170, color: '#ffb060', flicker: true } },
            { at: 'C', chest: 'tavern_cellar', loot: { coins: 30, give: 'potion' }, locked: 'clear' }
        ]
    });

    // ---- shop: Tilda's ----

    place('shop', { clip: 'fac-village-a.f1', h: 262, dx: -54 }, ['shop_main']);
    room('shop_main', {
        dark: 0.05, spawn: 'S', exits: { X: OUT },
        map: [
            '##############',
            '#P...N...P..f#',
            '#...B..c.....#',
            '#............#',
            '#.z.....Y...z#',
            '#............#',
            '#.....Y...v.b#',
            '#x...........#',
            '#.....S......#',
            '######X#######'
        ],
        things: [
            { at: 'P', prop: TRD + 'p1' }, { at: 'B', prop: TRD + 'p0' }, { at: 'c', prop: TRD + 'p11' }, { at: 'z', prop: TRD + 'p9' },
            { at: 'Y', prop: TRD + 'p10' }, { at: 'f', prop: TRD + 'p12' }, { at: 'b', prop: TAV + 'p5' }, { at: 'x', prop: TAV + 'p10' },
            { at: 'N', npc: 'shopkeeper', meeting: 'shopkeeper_home' }, { at: 'v', npc: 'villager_f', meeting: 'villager_f_home' }
        ]
    });

    // ---- smithy: Hakon's forge ----

    place('smithy', { clip: 'fac-village-a.f2', h: 250, dx: -65 }, ['smithy_main']);
    room('smithy_main', {
        wall: 3, dark: 0.28, spawn: 'S', exits: { X: OUT },
        map: [
            '##############',
            '#.F.b..W..a..#',
            '#............#',
            '#..AN....v...#',
            '#............#',
            '#.k......w...#',
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

    // ---- wardens' hall: Elder Runa, the chronicle lectern and five rune pedestals ----

    place('hall', { clip: 'fac-village-b.f0', h: 300, dx: -3 }, ['hall_main']);
    room('hall_main', {
        wall: 3, dark: 0.22, spawn: 'S', exits: { X: OUT },
        map: [
            '##################',
            '#.f..fBBLNH.f..f.#',
            '#................#',
            '#................#',
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
            { at: 'z', npc: 'recruit_m', meeting: 'recruit_m_home', if: { flag: 'obs.done' } }
        ]
    });
})(typeof window !== 'undefined' ? window : globalThis);
