// Clockwork Observatory: the physical playfield (plan 5.3 as amended by AM4). Shell + lower kit (kickback left) + both ramp channels + top area,
// the orrery disc, the telescope VUK with its pendulum, the S-T-A-R bank, two gravity wells, comet / meridian / telescope paths. DOM-free.
import { composeLayout } from '../kit/compose.js';
import { shell, leftChannel, rightChannel } from '../kit/shell.js';
import { lowerKit, CX } from '../kit/lower.js';
import { wall, post } from '../kit/geom.js';
import { observatoryLamps, observatoryFlashers, observatoryGI, C, STAR, STAR_ANGLE, DISC, DISC_R, SCOPE, POPS, WELLS } from './lamps.js';

const CHAINED = 99; // paths that are only entered from another path or a hole must never catch a field ball

function topArea() {
    return {
        walls: [-0.0675, -0.0225, 0.0225, 0.0675].map((x, i) => wall('laneGuide' + (i + 1), 'metal', [[x, 0.940], [x, 0.990]])),
        devices: [
            ...[-0.045, 0, 0.045].map((x, i) => ({ id: 'laneT' + (i + 1), type: 'rollover', x, y: 0.965 })),
            ...POPS.map(([id, x, y]) => ({ id, type: 'bumper', x, y }))
        ]
    };
}

// Orrery disc: a flush motor-driven turntable with a hub post in the middle. Balls arrive from the orrery lane below it and from the telescope tube.
function disc() {
    return {
        posts: [post('orreryHub', DISC[0], DISC[1], 0.0055, 'metal', 'none')],
        devices: [
            { id: 'orrery', type: 'orrery', x: DISC[0], y: DISC[1], r: DISC_R, speed: 6, maxSpeed: 26, accel: 20, mu: 0.5 },
            { id: 'orreryGate', type: 'trigger', a: [-0.120, 0.725], b: [0.012, 0.725] }
        ]
    };
}

// Telescope: the neon Data Core channel shifted to x 0.045, a VUK at its head, a pendulum across its mouth.
function telescope() {
    return {
        walls: [
            wall('scopeChanL', 'metal', [[0.021, 0.585], [0.021, 0.693]]),
            wall('scopeChanR', 'metal', [[0.069, 0.585], [0.069, 0.693]])
        ],
        posts: [post('scopeBase', SCOPE[0], 0.700, 0.025, 'metal', 'none')],
        devices: [
            { id: 'telescope', type: 'hole', kind: 'vuk', x: SCOPE[0], y: SCOPE[1], r: 0.017, capacity: 1, eject: { path: 'telescopeTube', speed: 2.0 } },
            { id: 'pendulum', type: 'pendulum', pivot: [0.045, 0.568], pivotH: 0.0885, length: 0.075, bobR: 0.011, axis: [1, 0],
              period: 1.8, fastPeriod: 1.15, amp: 0.42, park: 1.10, mode: 'swing', material: 'metal', model: 'pendulum' }
        ]
    };
}

// S-T-A-R: four standups in one slanted line, 8 mm gaps, so a ball on top rolls across them and off the end.
function starBank() {
    return {
        devices: STAR.map(([id, x, y]) => ({ id, type: 'standup', x, y, angle: STAR_ANGLE, width: 0.020 })),
        allowGaps: [['starS', 'starT'], ['starT', 'starA'], ['starA', 'starR']]
    };
}

function channels() {
    const lc = leftChannel(), rc = rightChannel();
    return {
        walls: [...lc.walls, wall('chanL_roof', 'plastic', [[-0.2215, 0.700], [-0.180, 0.670]], { height: 0.04 }), ...rc.walls],
        allowGaps: [...lc.allowGaps, ...rc.allowGaps],
        devices: [{ id: 'spinnerR', type: 'spinner', a: [0.1875, 0.660], b: [0.2235, 0.660] }],
        entries: { left: lc.entry, right: rc.entry }
    };
}

function paths(entries) {
    return [
        { id: 'cometRamp', kind: 'plastic', entry: entries.left, width: 0.05, color: C.copper,
          points: [[-0.156, 0.625, 0], [-0.156, 0.700, 0.030], [-0.150, 0.790, 0.050], [-0.120, 0.870, 0.058]],
          exit: { kind: 'path', path: 'cometReturn' } },
        { id: 'cometReturn', kind: 'wire', entry: { a: [-0.140, 0.860], b: [-0.100, 0.860] }, minSpeed: CHAINED, width: 0.034, color: C.copper,
          points: [[-0.120, 0.870, 0.058], [-0.080, 0.945, 0.056], [0.000, 0.985, 0.054], [0.080, 0.960, 0.046], [0.150, 0.890, 0.037],
              [0.198, 0.790, 0.037], [0.206, 0.640, 0.034], [0.192, 0.500, 0.022], [0.172, 0.432, 0.006], [0.1685, 0.418, 0]],
          exit: { kind: 'field', dir: [0, -1], speedScale: 0.8 } },
        { id: 'meridianRamp', kind: 'wire', entry: entries.right, width: 0.034, color: C.night,
          points: [[0.128, 0.640, 0], [0.130, 0.720, 0.035], [0.130, 0.800, 0.062], [0.115, 0.900, 0.082], [0.060, 0.985, 0.092], [-0.030, 1.010, 0.095],
              [-0.110, 0.985, 0.094], [-0.170, 0.910, 0.088], [-0.205, 0.810, 0.075], [-0.218, 0.680, 0.062], [-0.212, 0.540, 0.045], [-0.2025, 0.440, 0.012], [-0.2025, 0.425, 0]],
          exit: { kind: 'field', dir: [0, -1], speedScale: 0.75 } },
        // The barrel is brass (outer radius 0.0225 round the ball line), so the glass check uses a 0.037 allowance and no glass tube is drawn.
        { id: 'telescopeTube', kind: 'tube', skin: 'telescope', top: 0.037, entry: { a: [0.028, 0.640], b: [0.062, 0.640] }, minSpeed: CHAINED, width: 0.034, color: C.brass,
          points: [[0.045, 0.640, 0], [0.0425, 0.648, 0.012], [0.0272, 0.700, 0.039], [0.012, 0.752, 0.066], [0.006, 0.768, 0.030], [0.002, 0.775, 0]],
          exit: { kind: 'field', dir: [-0.4, -1], speedScale: 0.3 } } // drops onto the disc rim
    ];
}

function build() {
    const ch = channels();
    return composeLayout(
        { id: 'observatory', version: 1, meta: {
            title: 'Clockwork Observatory', accent: C.brass, secondary: C.night, cx: CX, troughSize: 4, replayScore: 40000000,
            giColor: { lower: '#ffd9a0', upper: '#9fbfff' },
            attract: [['spiral', 2000], ['ring', 1600], ['sweepUp', 1600], ['sparkle', 2400], ['radial', 1500], ['breath', 1800]]
        } },
        shell(),
        lowerKit({ cx: CX, kickback: 'left' }),
        topArea(), disc(), telescope(), starBank(),
        { walls: ch.walls, allowGaps: ch.allowGaps, devices: ch.devices, paths: paths(ch.entries) },
        {
            devices: WELLS.map(([id, x, y]) => ({ id, type: 'magnet', x, y, r: 0.035 })),
            shots: [
                { id: 'leftOrbit', on: 'trigger:leftOrbitUp@1', combo: true, bonus: 'orbits', setup: [] },
                { id: 'comet', on: 'pathMade:cometRamp', combo: true, bonus: 'ramps', setup: [] },
                { id: 'orrery', on: 'trigger:orreryGate@1', combo: true, setup: [] },
                { id: 'telescope', on: 'hole:telescope', combo: true, setup: [{ type: 'pendulum', id: 'pendulum', mode: 'park' }] },
                { id: 'meridian', on: 'pathMade:meridianRamp', combo: true, bonus: 'ramps', setup: [] },
                { id: 'rightOrbit', on: 'trigger:rightOrbitUp@1', combo: true, bonus: 'orbits', setup: [] }
            ],
            lamps: observatoryLamps(), flashers: observatoryFlashers(), gi: observatoryGI(),
            toys: [
                { id: 'orrery', kind: 'orrery', x: DISC[0], y: DISC[1], h: 0, rot: 0, scale: 1 },
                { id: 'scope', kind: 'telescope', x: 0.045, y: 0.700, h: 0, rot: 0, scale: 1, path: 'telescopeTube', from: 0.10, to: 0.66 },
                { id: 'gearTrain', kind: 'gears', x: -0.207, y: 1.030, h: 0.043, rot: 0, scale: 1 },
                { id: 'dome', kind: 'dome', x: 0.207, y: 1.030, h: 0.043, rot: 0, scale: 1 },
                { id: 'clock', kind: 'clockDial', x: DISC[0], y: DISC[1], h: 0, rot: 0, scale: 1 }
            ],
            labels: [{ key: 'lamp_jackpot', x: -0.156, y: 0.560, size: 0.007, angle: 0, color: C.brass }]
        }
    );
}

export const layout = build();
export default layout;
