/* Runenpfad place data: enterable houses and caves, their top-down rooms, prop footprints and the room validator. DOM-free. */
(function (root) {
    'use strict';

    const CELL = 48;
    const LOOT_RE_FALLBACK = /^[a-z][a-z0-9_]{1,23}$/;
    const MIN_MOB_DIST = 4;

    function def(w, h, solid, layer, light) {
        return { w: w, h: h, solid: solid, layer: layer || 'obj', light: light || null };
    }
    function lamp(r, color, flicker) { return { r: r, color: color, flicker: !!flicker }; }

    // Footprint (cells), solidity, draw layer and light of every prop of the four td-props sheets.
    const PROP_DEFS = {
        'tdp-tavern.p0': def(2, 1, true), 'tdp-tavern.p1': def(1, 1, true), 'tdp-tavern.p2': def(1, 1, false), 'tdp-tavern.p3': def(2, 1, true),
        'tdp-tavern.p4': def(3, 1, true), 'tdp-tavern.p5': def(1, 1, true), 'tdp-tavern.p6': def(2, 1, true),
        'tdp-tavern.p7': def(2, 1, true, 'obj', lamp(230, '#ffb060', true)), 'tdp-tavern.p8': def(1, 2, true), 'tdp-tavern.p9': def(2, 1, true),
        'tdp-tavern.p10': def(1, 1, true), 'tdp-tavern.p11': def(2, 2, false, 'floor'), 'tdp-tavern.p12': def(1, 1, true),
        'tdp-tavern.p13': def(1, 1, false, 'floor'), 'tdp-tavern.p14': def(1, 1, true), 'tdp-tavern.p15': def(1, 1, true),
        'tdp-trade.p0': def(3, 1, true), 'tdp-trade.p1': def(2, 1, true), 'tdp-trade.p2': def(2, 1, true), 'tdp-trade.p3': def(1, 1, true),
        'tdp-trade.p4': def(1, 1, true), 'tdp-trade.p5': def(2, 1, true, 'obj', lamp(220, '#ff8a3a', true)), 'tdp-trade.p6': def(1, 1, true),
        'tdp-trade.p7': def(2, 1, true), 'tdp-trade.p8': def(2, 1, true), 'tdp-trade.p9': def(1, 1, true), 'tdp-trade.p10': def(2, 1, true),
        'tdp-trade.p11': def(1, 1, true), 'tdp-trade.p12': def(1, 1, true), 'tdp-trade.p13': def(1, 1, true),
        'tdp-trade.p14': def(1, 1, true, 'obj', lamp(110, '#9ad8ff', false)), 'tdp-trade.p15': def(1, 1, true),
        'tdp-cave.p0': def(1, 1, true), 'tdp-cave.p1': def(1, 1, true), 'tdp-cave.p2': def(2, 1, true),
        'tdp-cave.p3': def(1, 1, true, 'obj', lamp(150, '#6fd0ff', false)), 'tdp-cave.p4': def(1, 1, true, 'obj', lamp(150, '#ffb347', false)),
        'tdp-cave.p5': def(1, 1, false, 'obj', lamp(90, '#7cffb0', false)), 'tdp-cave.p6': def(1, 1, false), 'tdp-cave.p7': def(1, 1, true, 'obj', lamp(210, '#ff9a40', true)),
        'tdp-cave.p8': def(1, 1, true), 'tdp-cave.p9': def(1, 1, true), 'tdp-cave.p10': def(1, 1, true), 'tdp-cave.p11': def(1, 1, false),
        'tdp-cave.p12': def(1, 1, false), 'tdp-cave.p13': def(1, 1, true, 'obj', lamp(190, '#ffb060', true)), 'tdp-cave.p14': def(1, 1, false),
        'tdp-cave.p15': def(2, 1, false, 'floor'),
        'tdp-ruin.p0': def(2, 1, true), 'tdp-ruin.p1': def(1, 1, true), 'tdp-ruin.p2': def(1, 1, true), 'tdp-ruin.p3': def(1, 1, true),
        'tdp-ruin.p4': def(1, 1, true, 'obj', lamp(190, '#ffa040', true)), 'tdp-ruin.p5': def(1, 1, true), 'tdp-ruin.p6': def(1, 1, true),
        'tdp-ruin.p7': def(1, 1, false), 'tdp-ruin.p8': def(1, 1, true), 'tdp-ruin.p9': def(1, 1, true), 'tdp-ruin.p10': def(1, 1, true),
        'tdp-ruin.p11': def(1, 1, true, 'obj', lamp(140, '#9fe8ff', false)), 'tdp-ruin.p12': def(1, 1, true), 'tdp-ruin.p13': def(1, 1, true),
        'tdp-ruin.p14': def(2, 1, true, 'obj', lamp(200, '#ff8a40', true)), 'tdp-ruin.p15': def(2, 1, true)
    };

    function padMap(rows) {
        const w = rows.reduce(function (m, r) { return Math.max(m, r.length); }, 0);
        return rows.map(function (r) { while (r.length < w) r += ' '; return r; });
    }

    const PLACES = {};
    const ROOMS = {};

    // Facade dx: px from the door centre to the facade bottom centre, measured per art frame.
    const FACADE_DX = {"hermit_hut":-29,"beast_den":2,"kobold_den":-19,"crystal_grotto":3,"nomad_tent":-1,"sun_tomb":2,"summit_hut":1,"ice_cave":0,"brass_lab":2,"boiler_room":0};

    function place(id, kind, world, facadeClip, h, rooms) {
        PLACES[id] = { id: id, nameKey: 'runenpfad.place.' + id, kind: kind, entry: rooms[0], facade: { clip: facadeClip, h: h, dx: FACADE_DX[id] || 0 }, rooms: rooms, world: world };
    }

    function room(id, o) {
        ROOMS[id] = {
            theme: o.theme, floor: o.floor || 0, floorAlt: o.floorAlt == null ? 1 : o.floorAlt, wall: o.wall || 2, dark: o.dark || 0,
            music: o.music || 'in_cozy', map: padMap(o.map), things: o.things, exits: o.exits || {}, spawn: o.spawn
        };
    }

    const WALL_LIGHT = lamp(200, '#ffb060', true);
    const TAV = 'tdp-tavern.';
    const TRD = 'tdp-trade.';
    const CAV = 'tdp-cave.';
    const RUI = 'tdp-ruin.';
    const OUT = { to: 'out' };

    // ---- level places: houses ----

    place('hermit_hut', 'house', 0, 'fac-village-b.f1', 260, ['hermit_hut']);
    room('hermit_hut', {
        theme: 'wood', dark: 0, music: 'in_cozy', spawn: 'S', exits: { D: OUT },
        map: [
            '############',
            '#.K...F...p#',
            '#..........#',
            '#.......N..#',
            '#b.........#',
            '#..r..T....#',
            '#.....S....#',
            '#a.......x.#',
            '#####D######'
        ],
        things: [
            { at: 'K', prop: TAV + 'p9' }, { at: 'F', prop: TAV + 'p7' }, { at: 'p', prop: TAV + 'p15' },
            { at: 'N', npc: 'hermit', meeting: 'hermit_home' }, { at: 'b', prop: TAV + 'p8' }, { at: 'b', rest: true },
            { at: 'T', prop: TAV + 'p1' }, { at: 'a', prop: TAV + 'p5' }, { at: 'x', prop: TAV + 'p10' },
            { at: 'r', prop: TAV + 'p11' }
        ]
    });

    place('kobold_den', 'house', 1, 'fac-wild-a.f0', 200, ['kobold_den']);
    room('kobold_den', {
        theme: 'cave', dark: 0.25, music: 'in_cozy', spawn: 'S', exits: { D: OUT },
        map: [
            '##############',
            '#bb......k...#',
            '#.l..NB...l..#',
            '#............#',
            '#.T...L...T..#',
            '#............#',
            '#............#',
            '#a..........x#',
            '#.....S......#',
            '######D#######'
        ],
        things: [
            { at: 'b', prop: TAV + 'p5' }, { at: 'k', prop: TAV + 'p6' }, { at: 'B', prop: TAV + 'p4' },
            { at: 'N', npc: 'barkeep', meeting: 'barkeep_home' }, { at: 'l', light: WALL_LIGHT },
            { at: 'T', prop: TAV + 'p1' }, { at: 'L', prop: TAV + 'p0' }, { at: 'a', prop: TAV + 'p5' }, { at: 'x', prop: TAV + 'p10' }
        ]
    });

    place('nomad_tent', 'house', 2, 'fac-wild-a.f2', 180, ['nomad_tent']);
    room('nomad_tent', {
        theme: 'tomb', dark: 0, music: 'in_cozy', spawn: 'S', exits: { D: OUT },
        map: [
            '############',
            '#u..f..f..u#',
            '#.....lN...#',
            '#..........#',
            '#...g......#',
            '#.u.......u#',
            '#..........#',
            '#.....S....#',
            '#####D######'
        ],
        things: [
            { at: 'u', prop: RUI + 'p1' }, { at: 'f', prop: RUI + 'p4' }, { at: 'l', prop: TRD + 'p13' },
            { at: 'N', npc: 'scribe', meeting: 'scribe_home' }, { at: 'g', prop: TAV + 'p11' }
        ]
    });

    place('summit_hut', 'house', 3, 'fac-wild-b.f0', 236, ['summit_hut']);
    room('summit_hut', {
        theme: 'wood', dark: 0, music: 'in_cozy', spawn: 'S', exits: { D: OUT },
        map: [
            '#############',
            '#x.x..F.....#',
            '#...........#',
            '#.....N.....#',
            '#b..........#',
            '#...r.......#',
            '#...........#',
            '#.....S.....#',
            '######D######'
        ],
        things: [
            { at: 'x', prop: TAV + 'p10' }, { at: 'F', prop: TAV + 'p7' }, { at: 'F', rest: true },
            { at: 'N', npc: 'climber', meeting: 'climber_home' }, { at: 'b', prop: TAV + 'p8' }, { at: 'r', prop: TAV + 'p11' }
        ]
    });

    place('brass_lab', 'house', 4, 'fac-wild-b.f2', 190, ['brass_lab']);
    room('brass_lab', {
        theme: 'works', dark: 0, music: 'in_cozy', spawn: 'S', exits: { D: OUT },
        map: [
            '##############',
            '#B...g..g.k..#',
            '#..........v.#',
            '#......N.....#',
            '#............#',
            '#.g........g.#',
            '#............#',
            '#............#',
            '#.....S......#',
            '######D#######'
        ],
        things: [
            { at: 'B', prop: RUI + 'p14' }, { at: 'g', prop: RUI + 'p12' }, { at: 'k', prop: RUI + 'p15' }, { at: 'v', prop: RUI + 'p13' },
            { at: 'N', npc: 'automaton', meeting: 'automaton_home' }
        ]
    });

    // ---- level places: caves ----

    place('beast_den', 'cave', 0, 'fac-village-a.f3', 190, ['beast_den']);
    room('beast_den', {
        theme: 'cave', dark: 0.55, music: 'in_cave', spawn: 'S', exits: { D: OUT },
        map: [
            '    ########    ',
            '  ###......###  ',
            ' ##..t....r..## ',
            '##....s.....C.##',
            '#...n........##',
            '#..o.....s...m.#',
            '##...r.......s.#',
            '##.s.........###',
            ' ###...t.....## ',
            '  ####q.S...####',
            '     ##D#####   '
        ],
        things: [
            { at: 's', mob: 'slime' }, { at: 'r', mob: 'rat' }, { at: 'n', prop: CAV + 'p11' }, { at: 'o', prop: CAV + 'p6' },
            { at: 'm', prop: CAV + 'p5' }, { at: 't', prop: CAV + 'p0' }, { at: 'q', prop: CAV + 'p13' },
            { at: 'C', chest: 'beast_den', loot: { coins: 40, give: 'rune_fire' }, locked: 'clear' }
        ]
    });

    place('crystal_grotto', 'cave', 1, 'fac-wild-a.f1', 190, ['crystal_grotto']);
    room('crystal_grotto', {
        theme: 'cave', floor: 1, floorAlt: 0, wall: 3, dark: 0.6, music: 'in_cave', spawn: 'S', exits: { D: OUT },
        map: [
            '     ########',
            '   ###..y.p.####',
            '  ##..c.......c.##',
            ' ##...b.....p...##',
            ' #..o......m...C.#',
            '##...p........b..#',
            '#...b........o...#',
            '##..c.....y....p.#',
            ' ##....m......o.##',
            '  ###....S....###',
            '    ####D#####'
        ],
        things: [
            { at: 'p', mob: 'spider' }, { at: 'b', mob: 'bat' }, { at: 'c', prop: CAV + 'p3' }, { at: 'y', prop: CAV + 'p4' },
            { at: 'm', prop: CAV + 'p5' }, { at: 'o', prop: CAV + 'p9' },
            { at: 'C', chest: 'crystal_grotto', loot: { coins: 30, give: 'rune_storm' }, locked: 'clear' }
        ]
    });

    place('sun_tomb', 'cave', 2, 'fac-wild-a.f3', 170, ['sun_tomb_a', 'sun_tomb_b']);
    room('sun_tomb_a', {
        theme: 'tomb', floor: 0, floorAlt: 1, wall: 2, dark: 0.35, music: 'in_cave', spawn: 'S',
        exits: { D: OUT, E: { to: 'sun_tomb_b', at: 'e' } },
        map: [
            '######E#######',
            '#f....e.....f#',
            '#...i......i.#',
            '#.m........m.#',
            '#............#',
            '#..u......u..#',
            '#............#',
            '#.....S......#',
            '######D#######'
        ],
        things: [
            { at: 'f', prop: RUI + 'p4' }, { at: 'i', prop: RUI + 'p2' }, { at: 'u', prop: RUI + 'p1' }, { at: 'm', mob: 'mummy' }
        ]
    });
    room('sun_tomb_b', {
        theme: 'tomb', floor: 1, floorAlt: 0, wall: 3, dark: 0.5, music: 'in_cave',
        exits: { U: { to: 'sun_tomb_a', at: 'e' } },
        map: [
            '################',
            '#..i........i..#',
            '#.z.....C...z..#',
            '#.......m......#',
            '#...m.......m..#',
            '#.i..........i.#',
            '#..............#',
            '#..z.......z...#',
            '#..............#',
            '#.....e........#',
            '######U#########'
        ],
        things: [
            { at: 'i', prop: RUI + 'p2' }, { at: 'z', prop: RUI + 'p0' }, { at: 'm', mob: 'mummy' },
            { at: 'C', chest: 'sun_tomb', loot: { coins: 60, give: 'potion' }, locked: 'clear' }
        ]
    });

    place('ice_cave', 'cave', 3, 'fac-wild-b.f1', 212, ['ice_cave']);
    room('ice_cave', {
        theme: 'ice', dark: 0.5, music: 'in_cave', spawn: 'S', exits: { D: OUT },
        map: [
            '     ########',
            '   ###..c...###',
            ' ##...w......i.##',
            '##.i.......w....##',
            '#...o..w....c..C.#',
            '#..c.......i..w..#',
            '##....w.....o...##',
            '#...i............#',
            ' ##.....c.....i.##',
            '  ###....S....###',
            '    ####D#####'
        ],
        things: [
            { at: 'w', mob: 'icewisp' }, { at: 'i', prop: RUI + 'p8' }, { at: 'c', prop: RUI + 'p11' }, { at: 'o', prop: RUI + 'p10' },
            { at: 'C', chest: 'ice_cave', loot: { coins: 40, give: 'rune_frost' }, locked: 'clear' }
        ]
    });

    place('boiler_room', 'cave', 4, 'fac-wild-b.f3', 210, ['boiler_room_a', 'boiler_room_b']);
    room('boiler_room_a', {
        theme: 'works', dark: 0.45, music: 'in_cave', spawn: 'S',
        exits: { D: OUT, E: { to: 'boiler_room_b', at: 'w' } },
        map: [
            '##############',
            '#B..c..g.v...#',
            '#............#',
            '#..c....c....#',
            '#...........eE',
            '#.g..........#',
            '#............#',
            '#.....S......#',
            '######D#######'
        ],
        things: [
            { at: 'B', prop: RUI + 'p14' }, { at: 'g', prop: RUI + 'p12' }, { at: 'v', prop: RUI + 'p13' }, { at: 'c', mob: 'clockbug' }
        ]
    });
    room('boiler_room_b', {
        theme: 'works', dark: 0.45, music: 'in_cave',
        exits: { W: { to: 'boiler_room_a', at: 'e' } },
        map: [
            '################',
            '#.B...v....g..C#',
            '#..............#',
            '#....c.....c...#',
            '#.v............#',
            'Ww.............#',
            '#..........c...#',
            '#..g...........#',
            '#..............#',
            '################'
        ],
        things: [
            { at: 'B', prop: RUI + 'p14' }, { at: 'v', prop: RUI + 'p13' }, { at: 'g', prop: RUI + 'p12' }, { at: 'c', mob: 'clockbug' },
            { at: 'C', chest: 'boiler_room', loot: { coins: 60, give: 'potion' }, locked: 'clear' }
        ]
    });

    // ---- layout and validation ----

    function profileApi() { return root.RunenpfadProfile || null; }

    // Parses a room into solid grids, markers, prop footprints and lights. Pure: a fresh result per call.
    function layout(roomId, sim) {
        const d = ROOMS[roomId];
        if (!d) return null;
        const rows = d.map.length;
        const cols = rows ? d.map[0].length : 0;
        const walls = new Uint8Array(cols * rows);
        const solid = new Uint8Array(cols * rows);
        const markers = {};
        for (let y = 0; y < rows; y++) {
            for (let x = 0; x < cols; x++) {
                const ch = d.map[y].charAt(x) || ' ';
                if (ch === '#' || ch === ' ') walls[y * cols + x] = 1;
                else if (ch !== '.' && ch !== ',') (markers[ch] || (markers[ch] = [])).push({ cx: x, cy: y });
            }
        }
        solid.set(walls);
        const props = [];
        const lights = [];
        const bad = [];
        d.things.forEach(function (t) {
            const cells = t.at ? (markers[t.at] || []) : [];
            if (t.prop) {
                const pd = PROP_DEFS[t.prop] || def(1, 1, true);
                const w = t.w || pd.w;
                const h = t.h || pd.h;
                const layer = t.layer || pd.layer;
                const isSolid = layer !== 'floor' && (t.solid == null ? pd.solid : !!t.solid);
                // `lit` (a dialogue condition) switches the prop's light on only while it holds; without a sim the light stays on.
                const D = root.RunenpfadDialogue;
                const light = t.lit && sim && D && !D.evalCond(t.lit, sim) ? null : (t.light || pd.light);
                cells.forEach(function (c) {
                    for (let yy = c.cy - h + 1; yy <= c.cy; yy++) {
                        for (let xx = c.cx; xx < c.cx + w; xx++) {
                            if (xx < 0 || yy < 0 || xx >= cols || yy >= rows || walls[yy * cols + xx]) { bad.push(t.prop + ' footprint at ' + c.cx + ',' + c.cy); continue; }
                            if (isSolid) solid[yy * cols + xx] = 1;
                        }
                    }
                    props.push({ clip: t.prop, x: (c.cx + w / 2) * CELL, y: (c.cy + 1) * CELL, wCells: w, hCells: h, layer: layer, light: light || null });
                    if (light) lights.push({ x: (c.cx + w / 2) * CELL, y: (c.cy + 1 - h / 2) * CELL, r: light.r, color: light.color, flicker: !!light.flicker });
                });
            } else if (t.light) {
                cells.forEach(function (c) {
                    lights.push({ x: (c.cx + 0.5) * CELL, y: (c.cy + 0.5) * CELL, r: t.light.r, color: t.light.color, flicker: !!t.light.flicker });
                });
            }
        });
        return { id: roomId, def: d, cols: cols, rows: rows, walls: walls, solid: solid, markers: markers, props: props, lights: lights, bad: bad };
    }

    function cellCentre(c) { return { x: (c.cx + 0.5) * CELL, y: (c.cy + 0.5) * CELL + 7 }; }

    const THING_KINDS = ['prop', 'npc', 'mob', 'chest', 'pickup', 'light', 'rest'];

    function floodFrom(lay, blocked, seeds) {
        const seen = new Uint8Array(lay.cols * lay.rows);
        const queue = [];
        seeds.forEach(function (s) {
            const i = s.cy * lay.cols + s.cx;
            if (!blocked[i] && !seen[i]) { seen[i] = 1; queue.push(s); }
        });
        while (queue.length) {
            const c = queue.pop();
            [[1, 0], [-1, 0], [0, 1], [0, -1]].forEach(function (dv) {
                const nx = c.cx + dv[0];
                const ny = c.cy + dv[1];
                if (nx < 0 || ny < 0 || nx >= lay.cols || ny >= lay.rows) return;
                const j = ny * lay.cols + nx;
                if (blocked[j] || seen[j]) return;
                seen[j] = 1;
                queue.push({ cx: nx, cy: ny });
            });
        }
        return seen;
    }

    function validate() {
        const probs = [];
        const P = profileApi();
        const lootRe = (P && P.LOOT_RE) || LOOT_RE_FALLBACK;
        const lootSeen = {};
        const folk = root.RunenpfadFolk || null;
        const arrivals = {};
        Object.keys(ROOMS).forEach(function (id) {
            Object.keys(ROOMS[id].exits || {}).forEach(function (m) {
                const ex = ROOMS[id].exits[m];
                if (ex.to !== 'out' && ex.at) (arrivals[ex.to] || (arrivals[ex.to] = [])).push(ex.at);
            });
        });
        Object.keys(ROOMS).forEach(function (id) {
            const d = ROOMS[id];
            const where = 'room ' + id;
            if (!d.map || !d.map.length) { probs.push(where + ': empty map'); return; }
            const w = d.map[0].length;
            if (d.map.some(function (r) { return r.length !== w; })) { probs.push(where + ': unequal row lengths'); return; }
            if (w < 10 || w > 22 || d.map.length < 8 || d.map.length > 14) probs.push(where + ': size ' + w + 'x' + d.map.length + ' outside 10-22 x 8-14');
            const lay = layout(id);
            lay.bad.forEach(function (b) { probs.push(where + ': ' + b + ' out of bounds or in a wall'); });
            const hasMarker = function (m, what) { if (!lay.markers[m]) probs.push(where + ': ' + what + ' marker ' + m + ' missing in the map'); };
            if (d.spawn) hasMarker(d.spawn, 'spawn');
            Object.keys(d.exits || {}).forEach(function (m) {
                hasMarker(m, 'exit');
                const ex = d.exits[m];
                if (ex.to !== 'out') {
                    if (!ROOMS[ex.to]) probs.push(where + ': exit ' + m + ' leads to unknown room ' + ex.to);
                    else if (!layout(ex.to).markers[ex.at]) probs.push(where + ': exit ' + m + ' arrival marker ' + ex.at + ' missing in ' + ex.to);
                }
            });
            // Solid cells for reachability: walls, solid props, plus every NPC and chest cell.
            const blocked = new Uint8Array(lay.solid);
            d.things.forEach(function (t, i) {
                const n = THING_KINDS.filter(function (k) { return t[k] != null; }).length;
                if (n !== 1 || (t.prop == null && t.at == null)) probs.push(where + ': thing ' + i + ' needs exactly one kind and an at marker');
                if (t.at) hasMarker(t.at, 'thing');
                if (t.prop && !PROP_DEFS[t.prop]) probs.push(where + ': unknown prop ' + t.prop);
                if (t.mob && ['rat', 'slime', 'bat', 'spider', 'mummy', 'icewisp', 'clockbug'].indexOf(t.mob) < 0) probs.push(where + ': unknown mob ' + t.mob);
                if (t.npc) {
                    if (folk && folk.SPRITES && !folk.SPRITES[t.npc]) probs.push(where + ': npc ' + t.npc + ' has no folk sprite');
                    if (!t.meeting) probs.push(where + ': npc ' + t.npc + ' needs a meeting id');
                }
                if (t.chest) {
                    if (!lootRe.test(t.chest)) probs.push(where + ': bad loot id ' + t.chest);
                    if (lootSeen[t.chest]) probs.push(where + ': duplicate loot id ' + t.chest);
                    lootSeen[t.chest] = true;
                    const loot = t.loot || {};
                    if (loot.give && !/^(potion|heart|meal|rune_(fire|frost|storm|magnet))$/.test(loot.give)) probs.push(where + ': bad loot give ' + loot.give);
                    if (loot.item && P && Object.keys(loot.item).some(function (k) { return !P.STORY_ITEMS[k]; })) probs.push(where + ': bad loot item');
                    if (t.locked === 'clear' && !d.things.some(function (o) { return o.mob; })) probs.push(where + ': chest locked by clear but no mobs');
                }
                if ((t.npc || t.chest) && t.at) (lay.markers[t.at] || []).forEach(function (c) { blocked[c.cy * lay.cols + c.cx] = 1; });
            });
            const seeds = [];
            const seedMarkers = (arrivals[id] || []).concat(d.spawn ? [d.spawn] : []);
            seedMarkers.forEach(function (m) { (lay.markers[m] || []).forEach(function (c) { seeds.push(c); }); });
            if (!seeds.length) { probs.push(where + ': no entry point (spawn or arrival)'); return; }
            const seen = floodFrom(lay, blocked, seeds);
            const reached = function (c) { return !!seen[c.cy * lay.cols + c.cx]; };
            const near = function (c) {
                return reached(c) || [[1, 0], [-1, 0], [0, 1], [0, -1]].some(function (dv) {
                    const nx = c.cx + dv[0];
                    const ny = c.cy + dv[1];
                    return nx >= 0 && ny >= 0 && nx < lay.cols && ny < lay.rows && seen[ny * lay.cols + nx];
                });
            };
            d.things.forEach(function (t) {
                const cells = lay.markers[t.at] || [];
                const solidTarget = !!(t.npc || t.chest || t.rest);
                if (t.npc || t.chest || t.rest || t.mob || t.pickup) {
                    cells.forEach(function (c) {
                        if (!(solidTarget ? near(c) : reached(c))) probs.push(where + ': ' + (t.npc || t.chest || t.mob || t.pickup || 'rest') + ' at ' + c.cx + ',' + c.cy + ' is unreachable');
                        if (t.mob) {
                            seeds.forEach(function (s) {
                                if (Math.hypot(s.cx - c.cx, s.cy - c.cy) < MIN_MOB_DIST) probs.push(where + ': mob at ' + c.cx + ',' + c.cy + ' is too close to an entry');
                            });
                        }
                    });
                }
            });
            Object.keys(d.exits || {}).forEach(function (m) {
                (lay.markers[m] || []).forEach(function (c) { if (!reached(c)) probs.push(where + ': exit ' + m + ' is unreachable'); });
            });
        });
        Object.keys(PLACES).forEach(function (pid) {
            const p = PLACES[pid];
            const where = 'place ' + pid;
            if (!ROOMS[p.entry]) { probs.push(where + ': unknown entry room'); return; }
            if (!p.rooms || p.rooms.indexOf(p.entry) < 0) probs.push(where + ': entry not in rooms');
            const entryExits = ROOMS[p.entry].exits || {};
            if (!Object.keys(entryExits).some(function (m) { return entryExits[m].to === 'out'; })) probs.push(where + ': entry room has no out exit');
            if (!ROOMS[p.entry].spawn) probs.push(where + ': entry room has no spawn');
            const seen = {};
            const stack = [p.entry];
            while (stack.length) {
                const rid = stack.pop();
                if (seen[rid] || !ROOMS[rid]) continue;
                seen[rid] = true;
                Object.keys(ROOMS[rid].exits || {}).forEach(function (m) { if (ROOMS[rid].exits[m].to !== 'out') stack.push(ROOMS[rid].exits[m].to); });
            }
            (p.rooms || []).forEach(function (rid) {
                if (!ROOMS[rid]) probs.push(where + ': unknown room ' + rid);
                else if (!seen[rid]) probs.push(where + ': room ' + rid + ' is not reachable from the entry');
            });
            if (['house', 'cave'].indexOf(p.kind) < 0) probs.push(where + ': bad kind');
            if (!p.facade || !p.facade.clip) probs.push(where + ': no facade');
        });
        return probs;
    }

    root.RunenpfadPlaceRooms = {
        CELL: CELL,
        PLACES: PLACES,
        ROOMS: ROOMS,
        PROP_DEFS: PROP_DEFS,
        layout: layout,
        cellCentre: cellCentre,
        validate: validate
    };
})(typeof window !== 'undefined' ? window : globalThis);
