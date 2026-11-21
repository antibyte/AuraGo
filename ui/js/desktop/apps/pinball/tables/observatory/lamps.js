// Clockwork Observatory: insert lamps, flashers and GI (plan 5.4). DOM-free.
// Lamp index = array index + 1, so the order matters for shows. The lower-kit lamps (lanes, kickbackL, shoot again, bonus) come from the kit.
export const C = { brass: '#d9a650', night: '#5d8cff', copper: '#c8743a', moon: '#e8eefc', red: '#ff7a5c', amber: '#ffb347' };

// The S-T-A-R bank: four standups on a line falling 25 degrees to the right with 8 mm gaps (the leviathan sonar recipe). Plan 5.3 put it at (0.112, 0.530),
// but there it walls off the left flipper's shots to the meridian and the right orbit; right of the disc it is out of every flipper corridor and the disc flings balls at it.
const STAR_AT = [0.092, 0.792];
export const STAR_ANGLE = -(Math.PI / 2 + 25 * Math.PI / 180);
const BANK = 25 * Math.PI / 180, STEP = 0.028;
export const STAR = ['S', 'T', 'A', 'R'].map((ch, i) => [
    'star' + ch, +(STAR_AT[0] + (i - 1.5) * STEP * Math.cos(BANK)).toFixed(4), +(STAR_AT[1] - (i - 1.5) * STEP * Math.sin(BANK)).toFixed(4), ch]);
export const DISC = [-0.040, 0.790], DISC_R = 0.050, SCOPE = [0.045, 0.640], POPS = [['popA', -0.085, 0.885], ['popB', 0.060, 0.880]];
// Plan 5.3 had wellMars at (-0.105, 0.470): from there the throw only enters the telescope channel inside a 1.7 degree window (the left channel wall blocks the shallow line).
export const WELLS = [['wellMars', -0.065, 0.440], ['wellSaturn', 0.071, 0.470]];

// Shot stacks: arrow at the entry, jackpot and combo lamps below it (toward the flippers). [shot, arrow, jackpot, combo, colour]
const STACKS = [
    ['leftOrbit', [-0.2395, 0.640], [-0.2395, 0.613], [-0.2345, 0.590], C.night],
    ['comet', [-0.156, 0.625], [-0.156, 0.600], [-0.156, 0.580], C.copper],
    ['orrery', [-0.060, 0.700], [-0.060, 0.676], [-0.060, 0.655], C.moon],
    ['telescope', [0.045, 0.598], [0.006, 0.598], [0.006, 0.576], C.brass],
    ['meridian', [0.128, 0.640], [0.128, 0.615], [0.128, 0.595], C.night],
    ['rightOrbit', [0.2055, 0.640], [0.2055, 0.615], [0.200, 0.595], C.night]
];

export function observatoryLamps() {
    const L = [];
    const add = (id, x, y, shape, w, h, color, extra = {}) => L.push({ id, x, y, shape, w, h, angle: 0, color, mount: 'insert', ...extra });
    for (const [shot, a, j, c, color] of STACKS) {
        add('arrow_' + shot, a[0], a[1], 'arrow', 0.015, 0.022, color, { group: 'arrows' });
        add('jp_' + shot, j[0], j[1], 'diamond', 0.014, 0.014, C.brass, { group: 'jackpots' });
        add('combo_' + shot, c[0], c[1], 'chevron', 0.014, 0.012, '#58f59a', { group: 'combos' });
    }
    [['meteor', -0.050, C.copper], ['transit', -0.020, C.moon], ['conjunction', 0.010, C.night], ['supernova', 0.040, C.red]]
        .forEach(([name, x, color]) => add('mode_' + name, x, 0.540, 'circle', 0.015, 0.015, color, { group: 'modes' }));
    add('modeStart', -0.017, 0.503, 'roundRect', 0.026, 0.016, '#58f59a', { label: 'lamp_mode' });
    add('superJackpot', -0.017, 0.470, 'star', 0.020, 0.020, C.brass, { label: 'lamp_super' });
    add('extraBall', -0.017, 0.436, 'roundRect', 0.030, 0.016, C.amber, { label: 'lamp_extra_ball' });
    add('wizard', -0.017, 0.397, 'star', 0.028, 0.028, C.moon, { label: 'lamp_wizard' });
    WELLS.forEach(([id, x, y], i) => add(id + 'Lit', x, y - 0.042, 'circle', 0.018, 0.018, i ? C.night : C.copper));
    add('pendulumLit', 0.045, 0.512, 'circle', 0.014, 0.014, C.brass);
    // S-T-A-R: a lamp in front of each standup, along its face normal.
    STAR.forEach(([id, x, y, ch]) => add(id, x + 0.024 * Math.cos(STAR_ANGLE), y + 0.024 * Math.sin(STAR_ANGLE), 'circle', 0.014, 0.014, C.night, { group: 'star', device: id, text: ch }));
    // Clock ring round the disc, clock12 at the top.
    for (let k = 1; k <= 12; k++) {
        const a = (90 - 30 * k) * Math.PI / 180;
        add('clock' + k, DISC[0] + 0.066 * Math.cos(a), DISC[1] + 0.066 * Math.sin(a), 'circle', 0.012, 0.012, C.moon, { group: 'clock' });
    }
    [-0.045, 0, 0.045].forEach((x, i) => add('laneT' + (i + 1), x, 0.965, 'circle', 0.016, 0.016, C.amber, { device: 'laneT' + (i + 1), group: 'topLanes' }));
    add('spinnerLit', 0.2055, 0.688, 'roundRect', 0.014, 0.012, C.brass, { label: 'lamp_spinner' });
    POPS.forEach(([id, x, y]) => add(id, x, y, 'circle', 0.030, 0.030, C.brass, { mount: 'bumper', device: id }));
    // Sky: the light-show columns either side of the lower centre.
    for (let i = 0; i < 14; i++) add('sky' + (i + 1), -0.017 + (i % 2 ? 0.042 : -0.042), 0.328 + Math.floor(i / 2) * 0.0215, 'star', 0.012, 0.012, i % 4 < 2 ? C.moon : C.night, { group: 'sky' });
    return L;
}

export function observatoryFlashers() {
    return [
        { id: 'flTelescope', x: 0.045, y: 0.705, h: 0.08, color: C.brass, r: 0.012 },
        { id: 'flOrrery', x: -0.040, y: 0.860, h: 0.06, color: C.moon, r: 0.012 },
        { id: 'flComet', x: -0.156, y: 0.720, h: 0.05, color: C.copper, r: 0.012 },
        { id: 'flMeridian', x: 0.130, y: 0.760, h: 0.05, color: C.night, r: 0.012 },
        { id: 'flPendulum', x: 0.0865, y: 0.585, h: 0.05, color: C.brass, r: 0.012 },
        { id: 'flWells', x: -0.155, y: 0.500, h: 0.04, color: C.night, r: 0.012 }
    ];
}

export function observatoryGI() {
    const lower = [], upper = [];
    for (const y of [0.10, 0.20, 0.30, 0.40, 0.50]) lower.push([-0.2475, y]);
    for (const y of [0.12, 0.25, 0.38, 0.50]) lower.push([0.2145, y]);
    lower.push([-0.20, 0.44], [0.17, 0.44]);
    for (let a = 25; a <= 155; a += 15) upper.push([0.2425 * Math.cos(a * Math.PI / 180), 0.8125 + 0.2425 * Math.sin(a * Math.PI / 180)]);
    upper.push([-0.2475, 0.62], [-0.2475, 0.76], [0.2045, 0.80], [0.2045, 0.70]);
    return [
        { id: 'lower', color: '#ffd9a0', yMin: 0, yMax: 0.55, bulbs: lower },
        { id: 'upper', color: '#9fbfff', yMin: 0.55, yMax: 1.07, bulbs: upper }
    ];
}
