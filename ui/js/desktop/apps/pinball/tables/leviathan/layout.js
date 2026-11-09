// Deep Sea Leviathan: the physical playfield (plan 6.2). Shell + lower kit (kickback right) + both ramp channels + top area,
// coral pops, sonar bank, captive pearl, Abyss magnet, sunken-ship scoop, current and tentacle ramps, upper left flipper. DOM-free.
import { composeLayout } from '../kit/compose.js';
import { shell, leftChannel, rightChannel } from '../kit/shell.js';
import { lowerKit, CX } from '../kit/lower.js';
import { wall, post, arcPoints } from '../kit/geom.js';
import { leviathanLamps, leviathanFlashers, leviathanGI, SONAR, CORAL, PING, PEARL_X, SHIP, HEAD, MAGNET } from './lamps.js';

const TEAL = '#43e3d5', GOLD = '#dcad63', BLUE = '#5aa8ff', CORAL_COLOR = '#ff7a5c';

function topArea() {
    const guides = [-0.110, -0.070, -0.030, 0.010].map((x, i) => wall('laneGuide' + (i + 1), 'metal', [[x, 0.965], [x, 1.005]]));
    return {
        walls: guides,
        devices: [
            ...[-0.090, -0.050, -0.010].map((x, i) => ({ id: 'laneT' + (i + 1), type: 'rollover', x, y: 0.985 })),
            ...CORAL.map(([id, x, y]) => ({ id, type: 'bumper', x, y }))
        ]
    };
}

// Three sonar standups in one slanted line (no flat face for a ball to perch on) and the outer two; the bank touches nothing.
function sonar() {
    return {
        devices: SONAR.map(([id, x, y, angle]) => ({ id, type: 'standup', x, y, angle, width: 0.020 })),
        allowGaps: [['sonar1', 'sonar2'], ['sonar2', 'sonar3'], ['sonar4', 'outer'], ['sonar5', 'shooterWall'], ['sonar4', 'orbitL_curveOut'], ['sonar5', 'orbitR_curveOut']] // 8 mm between faces / wedge corners: a ball cannot get in
    };
}

// Captive pearl: the channel rises from its mouth, the pearl rests at the mouth (the mouth posts keep the striker out). A roof over the
// closed end sheds balls that land on it.
function pearl() {
    const top = 0.700, w = 0.017;
    return {
        walls: [
            wall('pearlRoofL', 'plastic', [[PEARL_X - w, top], [PEARL_X, top + 0.028]], { height: 0.04 }),
            wall('pearlRoofR', 'plastic', [[PEARL_X, top + 0.028], [PEARL_X + w, top]], { height: 0.04 })
        ],
        devices: [{ id: 'pearl', type: 'captive', channel: { a: [PEARL_X, 0.600], b: [PEARL_X, top], width: 0.034 }, rest: [PEARL_X, 0.6102], endSwitch: 'pearlTop' }]
    };
}

// Sunken ship: a scoop with a backstop ring behind it, mouth facing down-left. A fast ball bounces off the ring back through the hole.
function ship() {
    const [sx, sy] = SHIP, ring = arcPoints(sx, sy, 0.030, -30, 120, 10);
    return {
        walls: [wall('shipRing', 'metal', ring, { height: 0.035 })],
        posts: [post('headBase', HEAD[0], HEAD[1], 0.035, 'plastic', 'none')],
        devices: [{ id: 'shipScoop', type: 'hole', kind: 'scoop', x: sx, y: sy, r: 0.020, captureSpeed: 3.0, capacity: 1, eject: { angleDeg: 240, speed: 1.5 } }]
    };
}

function mount() {
    // Upper left flipper: a block between the orbit guide and its pivot, sloped so a ball rolls off it onto the bat, and a slanted ceiling over
    // the bat that turns the flipper's steep launch into a flat shot toward the sunken ship (and feeds balls that land on it back to the bat).
    return {
        flippers: [{ id: 'upperLeft', key: 'left', x: -0.196, y: 0.760, side: 1, length: 0.050 }],
        walls: [
            wall('ulMount', 'plastic', [[-0.2215, 0.795], [-0.2010, 0.7665]], { height: 0.04 }),
            wall('ulCeiling', 'plastic', [[-0.170, 0.825], [-0.1197, 0.8577]], { height: 0.04 })
        ],
        allowGaps: [['ulMount', 'orbitL_inner']]
    };
}

function channels() {
    const lc = leftChannel(), rc = rightChannel();
    // A roof over the corridor between the left orbit guide and the left channel closes the V pocket above the orbit curve.
    return {
        walls: [...lc.walls, wall('chanL_roof', 'plastic', [[-0.2215, 0.700], [-0.180, 0.670]], { height: 0.04 }), ...rc.walls],
        allowGaps: [...lc.allowGaps, ...rc.allowGaps],
        devices: [
            { id: 'spinnerL', type: 'spinner', a: [-0.2575, 0.660], b: [-0.2215, 0.660] },
            { id: 'spinnerR', type: 'spinner', a: [0.1875, 0.660], b: [0.2235, 0.660] }
        ],
        entries: { left: lc.entry, right: rc.entry }
    };
}

function rampPaths(entries) {
    // Tentacle helix: 270 degrees round the head (r 0.05) from the east side, rising 0.095 to 0.145, then over the top-left at h 0.17.
    const helix = [-20, 40, 100, 160, 220, 250].map((deg, i) => {
        const a = deg * Math.PI / 180;
        return [+(HEAD[0] + 0.05 * Math.cos(a)).toFixed(4), +(HEAD[1] + 0.05 * Math.sin(a)).toFixed(4), [0.095, 0.108, 0.118, 0.128, 0.138, 0.145][i]];
    });
    return [
        { id: 'currentRamp', kind: 'wire', entry: entries.left, width: 0.034, color: BLUE, switches: [{ id: 'currentTop', at: 0.5 }],
          points: [[-0.156, 0.625, 0], [-0.158, 0.700, 0.030], [-0.150, 0.790, 0.065], [-0.115, 0.880, 0.100], [-0.050, 0.960, 0.120], [0.030, 0.985, 0.122], [0.120, 0.970, 0.118],
              [0.185, 0.900, 0.105], [0.210, 0.780, 0.085], [0.208, 0.640, 0.062], [0.192, 0.500, 0.035], [0.172, 0.432, 0.008], [0.1685, 0.418, 0]],
          exit: { kind: 'field', dir: [0, -1], speedScale: 0.7 } },
        { id: 'tentacleRamp', kind: 'plastic', entry: entries.right, width: 0.05, color: CORAL_COLOR, switches: [{ id: 'tentacleTop', at: 0.5 }],
          points: [[0.128, 0.640, 0], [0.140, 0.720, 0.040], [0.162, 0.790, 0.075], ...helix,
              [0.030, 0.880, 0.162], [-0.040, 0.940, 0.172], [-0.130, 0.960, 0.170], [-0.205, 0.870, 0.150], [-0.222, 0.780, 0.125], [-0.224, 0.680, 0.090],
              [-0.214, 0.540, 0.052], [-0.2025, 0.440, 0.014], [-0.2025, 0.425, 0]],
          exit: { kind: 'field', dir: [0, -1], speedScale: 0.7 } }
    ];
}

function build() {
    const ch = channels();
    return composeLayout(
        { id: 'leviathan', version: 1, meta: {
            title: 'Deep Sea Leviathan', accent: TEAL, secondary: GOLD, cx: CX, troughSize: 4,
            giColor: { lower: '#bff6ff', upper: '#7fd3ff' },
            attract: [['breath', 2400], ['sweepUp', 1800], ['radial', 1600], ['sparkle', 2400], ['ring', 1600]]
        } },
        shell(),
        lowerKit({ cx: CX, kickback: 'right' }),
        topArea(), sonar(), pearl(), ship(), mount(),
        { walls: ch.walls, allowGaps: ch.allowGaps, devices: ch.devices, paths: rampPaths(ch.entries) },
        {
            devices: [...PING.map(([id, x, y]) => ({ id, type: 'rollover', x, y })), { id: 'abyss', type: 'magnet', x: MAGNET[0], y: MAGNET[1], r: 0.035 }],
            shots: [
                { id: 'leftOrbit', on: 'trigger:leftOrbitUp@1', combo: true, bonus: 'orbits', setup: [] },
                { id: 'current', on: 'pathMade:currentRamp', combo: true, bonus: 'ramps', setup: [] },
                { id: 'pearl', on: 'captive:pearl', combo: true, setup: [] },
                { id: 'ship', on: 'hole:shipScoop', combo: true, setup: [] },
                { id: 'tentacle', on: 'pathMade:tentacleRamp', combo: true, bonus: 'ramps', setup: [] },
                { id: 'rightOrbit', on: 'trigger:rightOrbitUp@1', combo: true, bonus: 'orbits', setup: [] }
            ],
            lamps: leviathanLamps(), flashers: leviathanFlashers(), gi: leviathanGI(),
            toys: [
                { id: 'head', kind: 'leviathan', x: HEAD[0], y: HEAD[1], h: 0, rot: 0, scale: 1 },
                { id: 'hull', kind: 'ship', x: SHIP[0], y: SHIP[1], h: 0, rot: 0.6, scale: 1 },
                ...[[-0.246, 0.565], [-0.238, 0.588], [0.213, 0.565], [0.206, 0.588]].map(([x, y], i) => ({ id: 'kelp' + (i + 1), kind: 'kelp', x, y, h: 0, rot: i, scale: 1 }))
            ],
            labels: [
                { key: 'lamp_lock', x: 0.170, y: 0.705, size: 0.007, angle: 0, color: '#a98bff' },
                { key: 'lamp_jackpot', x: -0.156, y: 0.560, size: 0.007, angle: 0, color: BLUE }
            ]
        }
    );
}

export const layout = build();
export default layout;
