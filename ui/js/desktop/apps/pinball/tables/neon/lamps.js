// Cyber Neon Metropolis: insert lamps, flashers and GI (plan 6.1). DOM-free.
// Lamp index = array index + 1. The lower-kit lamps (lanes, kickback, shoot again, bonus) come from the kit.
const C = { cyan: '#37d9ff', pink: '#ff48c4', amber: '#ffb347', green: '#58f59a', violet: '#a98bff', white: '#f2f6ff', red: '#ff4f5e', gold: '#ffd24a' };

// Shot stacks: arrow at the entry, jackpot and combo lamps below it (toward the flippers).
const STACKS = [
    ['leftOrbit', [-0.2395, 0.640], [-0.2395, 0.613], [-0.2345, 0.590], C.pink, 0],
    ['leftRamp', [-0.156, 0.625], [-0.156, 0.600], [-0.156, 0.580], C.pink, 0],
    ['vault', [-0.0871, 0.6748], [-0.0778, 0.6548], [-0.0693, 0.6367], C.violet, 0.437],
    ['core', [0, 0.666], [-0.042, 0.652], [0.042, 0.652], C.cyan, 0],
    ['rightRamp', [0.128, 0.640], [0.128, 0.615], [0.128, 0.595], C.cyan, 0],
    ['rightOrbit', [0.2055, 0.640], [0.2055, 0.615], [0.2, 0.595], C.cyan, 0]
];

export function neonLamps() {
    const L = [];
    const add = (id, x, y, shape, w, h, color, extra = {}) => L.push({ id, x, y, shape, w, h, angle: 0, color, mount: 'insert', ...extra });
    for (const [shot, a, j, c, color, ang] of STACKS) {
        add('arrow_' + shot, a[0], a[1], 'arrow', shot === 'core' ? 0.012 : 0.015, shot === 'core' ? 0.014 : 0.022, color, { angle: ang, group: 'arrows' });
        add('jp_' + shot, j[0], j[1], 'diamond', 0.014, 0.014, C.gold, { group: 'jackpots' });
        add('combo_' + shot, c[0], c[1], 'chevron', 0.014, 0.012, C.green, { group: 'combos' });
    }
    // Data Core: four mode lamps in a diamond around the VUK, super jackpot / mode start / extra ball / wizard below it.
    add('mode_' + 'hack', -0.052, 0.690, 'circle', 0.015, 0.015, C.green, { group: 'modes' });
    add('mode_' + 'blackout', 0.052, 0.690, 'circle', 0.015, 0.015, C.red, { group: 'modes' });
    add('mode_' + 'chase', -0.046, 0.628, 'circle', 0.015, 0.015, C.pink, { group: 'modes' });
    add('mode_' + 'overclock', 0.046, 0.628, 'circle', 0.015, 0.015, C.amber, { group: 'modes' });
    add('superJackpot', 0, 0.603, 'star', 0.020, 0.020, C.gold, { label: 'lamp_super' });
    add('modeStart', 0, 0.572, 'roundRect', 0.026, 0.016, C.green, { label: 'lamp_mode' });
    add('extraBall', 0, 0.535, 'roundRect', 0.030, 0.016, C.amber, { label: 'lamp_extra_ball' });
    add('wizard', 0, 0.495, 'star', 0.028, 0.028, C.white, { label: 'lamp_wizard' });
    // FIREWALL: one lamp per target (in front of it), the lock lamps beside the left ramp.
    [[-0.128, 0.700], [-0.104, 0.711], [-0.080, 0.722]].forEach(([x, y], i) => {
        add('fw' + (i + 1), x + 0.423 * 0.018, y - 0.906 * 0.018, 'rect', 0.016, 0.009, C.red, { angle: 0.437, group: 'firewall', device: 'fw' + (i + 1) });
    });
    add('lockLit', -0.1155, 0.628, 'roundRect', 0.024, 0.012, C.violet, { label: 'lamp_lock' });
    [1, 2, 3].forEach(n => add('lock' + n, -0.1205 + (n - 1) * 0.0115, 0.606 - (n - 1) * 0.011, 'circle', 0.011, 0.011, C.violet, { group: 'locks' }));
    // N-E-O-N letters behind the standups, top lanes, spinner.
    [['neonN', -0.100, 0.549], ['neonE', -0.064, 0.609], ['neonO', 0.064, 0.609], ['neonN2', 0.068, 0.549]].forEach(([id, x, y]) =>
        add(id, x, y, 'circle', 0.014, 0.014, C.cyan, { group: 'neon', text: id === 'neonN2' ? 'N' : id.slice(4) }));
    [-0.045, 0, 0.045].forEach((x, i) => add('laneT' + (i + 1), x, 0.930, 'circle', 0.016, 0.016, C.amber, { device: 'laneT' + (i + 1), group: 'topLanes' }));
    add('spinnerLit', -0.2395, 0.682, 'roundRect', 0.014, 0.012, C.gold, { label: 'lamp_spinner' });
    // Pops light their bumpers.
    for (const id of ['popL', 'popR', 'popB']) add(id, ({ popL: -0.048, popR: 0.045, popB: 0 })[id], ({ popL: 0.836, popR: 0.845, popB: 0.778 })[id], 'circle', 0.030, 0.030, C.pink, { mount: 'bumper', device: id });
    // Light-show lamps: stream rows up the middle, skyline lamps on the toy.
    for (let i = 0; i < 16; i++) add('stream' + (i + 1), -0.017 + (i % 2 ? 0.035 : -0.035), 0.328 + Math.floor(i / 2) * 0.0215, 'rect', 0.014, 0.012, C.cyan, { group: 'stream' });
    [-0.150, -0.090, -0.030, 0.030, 0.090, 0.150].forEach((x, i) => add('skyline' + (i + 1), x, 0.985, 'rect', 0.020, 0.012, i % 2 ? C.pink : C.cyan, { mount: 'toy', group: 'skyline' }));
    return L;
}

export function neonFlashers() {
    return [
        { id: 'flCore', x: 0, y: 0.700, h: 0.08, color: C.cyan, r: 0.012 },
        { id: 'flVault', x: -0.112, y: 0.762, h: 0.05, color: C.violet, r: 0.012 },
        { id: 'flLeftRamp', x: -0.156, y: 0.720, h: 0.05, color: C.pink, r: 0.012 },
        { id: 'flRightRamp', x: 0.130, y: 0.760, h: 0.05, color: C.cyan, r: 0.012 },
        { id: 'flPops', x: 0, y: 0.900, h: 0.05, color: C.white, r: 0.012 }
    ];
}

export function neonGI() {
    const lower = [], upper = [];
    for (const y of [0.10, 0.20, 0.30, 0.40, 0.50]) { lower.push([-0.2475, y]); }
    for (const y of [0.12, 0.25, 0.38, 0.50]) lower.push([0.2145, y]);
    lower.push([-0.20, 0.44], [0.17, 0.44]);
    for (let a = 25; a <= 155; a += 15) upper.push([0.2425 * Math.cos(a * Math.PI / 180), 0.8125 + 0.2425 * Math.sin(a * Math.PI / 180)]);
    upper.push([-0.2475, 0.62], [-0.2475, 0.76], [0.2045, 0.80], [0.2045, 0.70]);
    return [
        { id: 'lower', color: '#d8f0ff', yMin: 0, yMax: 0.55, bulbs: lower },
        { id: 'upper', color: '#c9b6ff', yMin: 0.55, yMax: 1.07, bulbs: upper }
    ];
}
