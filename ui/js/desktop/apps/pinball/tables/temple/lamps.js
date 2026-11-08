// Lost Temple of the Sun: insert lamps, flashers and GI (plan 6.3). DOM-free.
// Lamp index = array index + 1. The lower-kit lamps (lanes, kickback, shoot again, bonus) come from the kit.
const C = { gold: '#ffcb65', jade: '#8ad6a3', amber: '#ffb347', red: '#ff5a3c', turq: '#43e3d5', white: '#fff2d0', violet: '#c9a7ff' };

// SUN bank: a row tilted 18 degrees (high on the left), so a ball that lands on top of it rolls off to the right.
// [id, x, y]; the normal points down and slightly left.
export const SUN = [['sunS', -0.036, 0.612], ['sunU', -0.012, 0.604], ['sunN', 0.012, 0.596], ['sunStar', 0.036, 0.588]];
export const SUN_ANGLE = Math.atan2(-0.008, 0.024) - Math.PI / 2;

// Shot stacks: arrow at the entry, jackpot and combo lamps below it (toward the flippers).
const STACKS = [
    ['leftOrbit', [-0.2395, 0.640], [-0.2395, 0.613], [-0.2345, 0.590], C.jade, 0],
    ['leftRamp', [-0.156, 0.625], [-0.156, 0.600], [-0.156, 0.580], C.gold, 0],
    ['idol', [0, 0.632], [-0.055, 0.650], [0.055, 0.650], C.amber, 0],
    ['rightRamp', [0.128, 0.640], [0.128, 0.615], [0.128, 0.595], C.gold, 0],
    ['rightOrbit', [0.2055, 0.640], [0.2055, 0.615], [0.2, 0.595], C.jade, 0],
    ['lock', [0.2055, 0.748], [0.2055, 0.724], [0.2055, 0.700], C.violet, 0]
];

export function templeLamps() {
    const L = [];
    const add = (id, x, y, shape, w, h, color, extra = {}) => L.push({ id, x, y, shape, w, h, angle: 0, color, mount: 'insert', ...extra });
    for (const [shot, a, j, c, color, ang] of STACKS) {
        add('arrow_' + shot, a[0], a[1], 'arrow', 0.015, 0.022, color, { angle: ang, group: 'arrows' });
        add('jp_' + shot, j[0], j[1], 'diamond', 0.014, 0.014, C.gold, { group: 'jackpots' });
        add('combo_' + shot, c[0], c[1], 'chevron', 0.014, 0.012, C.jade, { group: 'combos' });
    }
    // SUN: one lamp per target, in front of it.
    SUN.forEach(([id, x, y]) =>
        add(id, x + 0.022 * Math.cos(SUN_ANGLE), y + 0.022 * Math.sin(SUN_ANGLE), id === 'sunStar' ? 'star' : 'circle', 0.016, 0.016, C.amber, { group: 'sun', device: id, ...(id === 'sunStar' ? {} : { text: id.slice(3) }) }));
    add('jewelL', -0.070, 0.520, 'diamond', 0.018, 0.018, C.turq, { group: 'jewels', device: 'jewelL' });
    add('jewelR', 0.070, 0.520, 'diamond', 0.018, 0.018, C.red, { group: 'jewels', device: 'jewelR' });
    add('doorsOpen', 0, 0.742, 'chevron', 0.016, 0.012, C.white, { group: 'doors' });
    // Four mode lamps beside the temple front; mode start, super jackpot, extra ball and wizard in the centre column.
    ['expedition', 'serpent', 'riddle', 'eclipse'].forEach((name, i) =>
        add('mode_' + name, i % 2 ? 0.075 : -0.075, i < 2 ? 0.760 : 0.725, 'circle', 0.015, 0.015, [C.jade, C.turq, C.amber, C.red][i], { group: 'modes' }));
    add('modeStart', 0, 0.545, 'roundRect', 0.026, 0.016, C.jade, { label: 'lamp_mode' });
    add('superJackpot', 0, 0.510, 'star', 0.020, 0.020, C.gold, { label: 'lamp_super' });
    add('extraBall', 0, 0.475, 'roundRect', 0.030, 0.016, C.amber, { label: 'lamp_extra_ball' });
    add('wizard', 0, 0.435, 'star', 0.028, 0.028, C.white, { label: 'lamp_wizard' });
    // Orbit lock lamps beside the lock lane.
    add('lockLit', 0.1625, 0.778, 'roundRect', 0.024, 0.012, C.violet, { label: 'lamp_lock' });
    [1, 2, 3].forEach(n => add('lock' + n, 0.1465 + (n - 1) * 0.0165, 0.755, 'circle', 0.011, 0.011, C.violet, { group: 'locks' }));
    // The idol's eyes sit on the toy; top lanes and spinner.
    add('idolEyeL', -0.009, 0.797, 'circle', 0.007, 0.007, C.gold, { mount: 'toy' });
    add('idolEyeR', 0.009, 0.797, 'circle', 0.007, 0.007, C.gold, { mount: 'toy' });
    [-0.04, 0, 0.04].forEach((x, i) => add('laneT' + (i + 1), x, 0.980, 'circle', 0.016, 0.016, C.amber, { device: 'laneT' + (i + 1), group: 'topLanes' }));
    add('spinnerLit', -0.2395, 0.682, 'roundRect', 0.014, 0.012, C.gold, { label: 'lamp_spinner' });
    // Pops light their bumpers.
    for (const [id, x, y] of [['jungle1', -0.090, 0.850], ['jungle2', 0.060, 0.835], ['jungle3', 0, 0.890]]) add(id, x, y, 'circle', 0.030, 0.030, C.jade, { mount: 'bumper', device: id });
    // Light-show lamps: stream rows up the middle, sun rays on the sun disc.
    for (let i = 0; i < 16; i++) add('stream' + (i + 1), -0.017 + (i % 2 ? 0.035 : -0.035), 0.328 + Math.floor(i / 2) * 0.0215, 'rect', 0.014, 0.012, C.gold, { group: 'stream' });
    [-0.150, -0.090, -0.030, 0.030, 0.090, 0.150].forEach((x, i) => add('ray' + (i + 1), x, 1.020, 'rect', 0.020, 0.012, i % 2 ? C.amber : C.gold, { mount: 'toy', group: 'rays' }));
    return L;
}

export function templeFlashers() {
    return [
        { id: 'flIdol', x: 0, y: 0.790, h: 0.07, color: C.gold, r: 0.012 },
        { id: 'flSun', x: 0, y: 0.625, h: 0.04, color: C.amber, r: 0.012 },
        { id: 'flLeftRamp', x: -0.156, y: 0.720, h: 0.05, color: C.gold, r: 0.012 },
        { id: 'flRightRamp', x: 0.130, y: 0.760, h: 0.05, color: C.turq, r: 0.012 },
        { id: 'flJungle', x: -0.015, y: 0.862, h: 0.05, color: C.jade, r: 0.012 }
    ];
}

export function templeGI() {
    const lower = [], upper = [];
    for (const y of [0.10, 0.20, 0.30, 0.40, 0.50]) lower.push([-0.2475, y]);
    for (const y of [0.12, 0.25, 0.38, 0.50]) lower.push([0.2145, y]);
    lower.push([-0.20, 0.44], [0.17, 0.44]);
    for (let a = 25; a <= 155; a += 15) upper.push([0.2425 * Math.cos(a * Math.PI / 180), 0.8125 + 0.2425 * Math.sin(a * Math.PI / 180)]);
    upper.push([-0.2475, 0.62], [-0.2475, 0.76], [0.2045, 0.80], [0.2045, 0.70]);
    return [
        { id: 'lower', color: '#ffd59a', yMin: 0, yMax: 0.55, bulbs: lower },
        { id: 'upper', color: '#ffb347', yMin: 0.55, yMax: 1.07, bulbs: upper }
    ];
}
