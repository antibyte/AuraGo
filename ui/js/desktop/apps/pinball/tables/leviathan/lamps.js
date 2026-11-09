// Deep Sea Leviathan: insert lamps, flashers and GI (plan 6.2). DOM-free.
// Lamp index = array index + 1. The lower-kit lamps (lanes, kickbackR, shoot again, bonus) come from the kit.
const C = { teal: '#43e3d5', gold: '#dcad63', coral: '#ff7a5c', blue: '#5aa8ff', green: '#58f59a', violet: '#a98bff', white: '#eaffff', red: '#ff4f5e', amber: '#ffb347' };

// Sonar bank: sonar1..3 are collinear on a line falling 25 degrees to the right with 8 mm gaps (a gap must be narrower than 2 r sin(tilt) or a ball settles in it), so a ball that
// lands on top rolls across them and off the end (a staircase would trap it). sonar4/5 sit flush on the wedge fillers under the orbit mouths, out of every shot corridor.
// [id, x, y, normal angle]; the normals of the bank lean 25 degrees left of straight down.
const BANK_A = 25 * Math.PI / 180, BANK_STEP = 0.028;
export const SONAR_TILT = Math.PI / 2 + BANK_A;
const bank = (n, id) => [id, +(0.007 + (n - 2) * BANK_STEP * Math.cos(BANK_A)).toFixed(5), +(0.657 - (n - 2) * BANK_STEP * Math.sin(BANK_A)).toFixed(5), -SONAR_TILT];
export const SONAR = [
    bank(1, 'sonar1'), bank(2, 'sonar2'), bank(3, 'sonar3'),
    ['sonar4', -0.2356, 0.5221, Math.atan2(-0.7036, 0.7106)], ['sonar5', 0.2025, 0.5237, Math.atan2(-0.664, -0.747)]
];
export const CORAL = [['coral1', -0.090, 0.905], ['coral2', -0.015, 0.910], ['coral3', -0.0525, 0.842]];
export const PING = [['ping1', -0.100, 0.452], ['ping2', -0.017, 0.462], ['ping3', 0.066, 0.452]];
export const PEARL_X = -0.082, SHIP = [0.100, 0.780], HEAD = [0.105, 0.880], MAGNET = [0.045, 0.540];

// Shot stacks: arrow at the entry, jackpot and combo lamps below it (toward the flippers). [shot, arrow, jackpot, combo, colour, angle]
const STACKS = [
    ['leftOrbit', [-0.2395, 0.640], [-0.2395, 0.613], [-0.2345, 0.590], C.teal, 0],
    ['current', [-0.156, 0.625], [-0.156, 0.600], [-0.156, 0.580], C.blue, 0],
    ['pearl', [PEARL_X, 0.572], [PEARL_X, 0.548], [PEARL_X, 0.526], C.white, 0],
    ['ship', [0.098, 0.738], [0.092, 0.714], [0.088, 0.692], C.gold, -0.13],
    ['tentacle', [0.128, 0.640], [0.128, 0.615], [0.128, 0.595], C.coral, 0],
    ['rightOrbit', [0.2055, 0.640], [0.2055, 0.615], [0.2, 0.595], C.teal, 0]
];

export function leviathanLamps() {
    const L = [];
    const add = (id, x, y, shape, w, h, color, extra = {}) => L.push({ id, x, y, shape, w, h, angle: 0, color, mount: 'insert', ...extra });
    for (const [shot, a, j, c, color, ang] of STACKS) {
        add('arrow_' + shot, a[0], a[1], 'arrow', 0.015, 0.022, color, { angle: ang, group: 'arrows' });
        add('jp_' + shot, j[0], j[1], 'diamond', 0.014, 0.014, C.gold, { group: 'jackpots' });
        add('combo_' + shot, c[0], c[1], 'chevron', 0.014, 0.012, C.green, { group: 'combos' });
    }
    // Sunken ship (the mode scoop): four mode lamps in a column left of the approach, super jackpot and mode start on the right deck.
    [['tsunami', C.blue, 0.062, 0.780], ['sonar', C.green, 0.062, 0.755], ['treasure', C.gold, 0.062, 0.730], ['kraken', C.coral, 0.062, 0.705]]
        .forEach(([name, color, x, y]) => add('mode_' + name, x, y, 'circle', 0.015, 0.015, color, { group: 'modes' }));
    add('superJackpot', 0.140, 0.800, 'star', 0.020, 0.020, C.gold, { label: 'lamp_super' });
    add('modeStart', 0.1425, 0.752, 'roundRect', 0.026, 0.014, C.green, { label: 'lamp_mode', angle: 0 });
    add('extraBall', 0.0, 0.600, 'roundRect', 0.030, 0.016, C.amber, { label: 'lamp_extra_ball' });
    add('wizard', -0.017, 0.395, 'star', 0.028, 0.028, C.white, { label: 'lamp_wizard' });
    // Sonar targets: one lamp in front of each face.
    SONAR.forEach(([id, x, y, a]) => add(id, x + 0.024 * Math.cos(a), y + 0.024 * Math.sin(a), 'circle', 0.013, 0.013, C.green, { group: 'sonar', device: id }));
    // Pearl strikes (three light the lock), beside the pearl channel.
    [0.622, 0.648, 0.674].forEach((y, i) => add('pearl' + (i + 1), PEARL_X - 0.0335, y, 'circle', 0.011, 0.011, C.white, { group: 'pearl' }));
    // Lock lamps in the sealed pocket beside the tentacle channel.
    add('lockLit', 0.170, 0.682, 'roundRect', 0.024, 0.012, C.violet, { label: 'lamp_lock', angle: Math.PI / 2 });
    [1, 2, 3].forEach(n => add('lock' + n, 0.170, 0.600 + n * 0.022, 'circle', 0.011, 0.011, C.violet, { group: 'locks' }));
    // Deep Grab (magnet) lamp below the magnet, spinner lamp in the right orbit, top lanes.
    add('grab', MAGNET[0], MAGNET[1] - 0.045, 'circle', 0.022, 0.022, C.teal);
    add('spinnerLit', 0.2055, 0.688, 'roundRect', 0.014, 0.012, C.gold, { label: 'lamp_spinner' });
    [-0.090, -0.050, -0.010].forEach((x, i) => add('laneT' + (i + 1), x, 0.985, 'circle', 0.016, 0.016, C.amber, { device: 'laneT' + (i + 1), group: 'topLanes' }));
    // The Leviathan's eyes glow on the head toy.
    add('eyeL', HEAD[0] - 0.014, HEAD[1] + 0.012, 'circle', 0.012, 0.012, C.red, { mount: 'toy', group: 'eyes' });
    add('eyeR', HEAD[0] + 0.014, HEAD[1] + 0.012, 'circle', 0.012, 0.012, C.red, { mount: 'toy', group: 'eyes' });
    // Coral pops light their bumpers.
    CORAL.forEach(([id, x, y]) => add(id, x, y, 'circle', 0.030, 0.030, C.coral, { mount: 'bumper', device: id }));
    // Sonar pings: three rollovers across the lower field, where every shot up the middle crosses.
    PING.forEach(([id, x, y]) => add(id, x, y, 'circle', 0.014, 0.014, C.teal, { device: id, group: 'pings' }));
    // Light-show lamps: bubble columns either side of the lower centre, a wave row between them.
    for (let i = 0; i < 10; i++) add('bubble' + (i + 1), -0.017 + (i % 2 ? 0.07 : -0.07), 0.318 + Math.floor(i / 2) * 0.026, 'circle', 0.010, 0.010, C.teal, { group: 'bubbles' });
    for (let i = 0; i < 5; i++) add('wave' + (i + 1), -0.017 + (i - 2) * 0.018, 0.352, 'rect', 0.012, 0.008, i % 2 ? C.blue : C.teal, { group: 'waves' });
    return L;
}

export function leviathanFlashers() {
    return [
        { id: 'flShip', x: SHIP[0], y: SHIP[1] + 0.045, h: 0.06, color: C.gold, r: 0.012 },
        { id: 'flPearl', x: PEARL_X, y: 0.725, h: 0.05, color: C.white, r: 0.012 },
        { id: 'flTentacle', x: 0.150, y: 0.760, h: 0.07, color: C.coral, r: 0.012 },
        { id: 'flCurrent', x: -0.155, y: 0.730, h: 0.05, color: C.blue, r: 0.012 },
        { id: 'flCoral', x: -0.050, y: 0.880, h: 0.05, color: C.teal, r: 0.012 }
    ];
}

export function leviathanGI() {
    const lower = [], upper = [];
    for (const y of [0.10, 0.20, 0.30, 0.40, 0.50]) lower.push([-0.2475, y]);
    for (const y of [0.12, 0.25, 0.38, 0.50]) lower.push([0.2145, y]);
    lower.push([-0.20, 0.44], [0.17, 0.44]);
    for (let a = 25; a <= 155; a += 15) upper.push([0.2425 * Math.cos(a * Math.PI / 180), 0.8125 + 0.2425 * Math.sin(a * Math.PI / 180)]);
    upper.push([-0.2475, 0.62], [-0.2475, 0.76], [0.2045, 0.80], [0.2045, 0.70]);
    return [
        { id: 'lower', color: '#bff6ff', yMin: 0, yMax: 0.55, bulbs: lower },
        { id: 'upper', color: '#7fd3ff', yMin: 0.55, yMax: 1.07, bulbs: upper }
    ];
}
