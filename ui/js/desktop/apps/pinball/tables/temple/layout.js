// Lost Temple of the Sun: the physical playfield (plan 6.3). Shell + lower kit + both ramp channels + jungle pops, the SUN
// bank in front of the idol's doors, the idol scoop with a trap-proof exit tube, the lock in the right orbit, jewels, the
// upper right flipper fed by the right orbit and the stone and gold ramps. DOM-free.
import { composeLayout } from '../kit/compose.js';
import { shell, leftChannel, rightChannel } from '../kit/shell.js';
import { lowerKit, CX } from '../kit/lower.js';
import { wall } from '../kit/geom.js';
import { templeLamps, templeFlashers, templeGI, SUN, SUN_ANGLE } from './lamps.js';

const GOLD = '#ffcb65', JADE = '#8ad6a3', STONE = '#c9b79a';
// Paths that are only entered from another path or a hole must never catch a field ball.
const CHAINED = 99;

const standup = (id, x, y) => ({ id, type: 'standup', x, y, angle: Math.atan2(0.20 - y, CX - x), width: 0.020 });

// Top lanes and the three jungle pops around the idol's temple front.
function topArea() {
    const guides = [-0.06, -0.02, 0.02, 0.06].map((x, i) => wall('laneGuide' + (i + 1), 'metal', [[x, 0.955], [x, 1.005]]));
    return {
        walls: guides,
        devices: [
            ...[-0.04, 0, 0.04].map((x, i) => ({ id: 'laneT' + (i + 1), type: 'rollover', x, y: 0.980 })),
            { id: 'jungle1', type: 'bumper', x: -0.090, y: 0.850 }, { id: 'jungle2', type: 'bumper', x: 0.060, y: 0.835 }, { id: 'jungle3', type: 'bumper', x: 0, y: 0.890 }
        ]
    };
}

// SUN bank in front of the doors; behind it the idol channel (x +-26 mm) closed by the doors and a peaked back wall.
// The scoop kicks its ball out through a tube that ends below the bank, so a closing door can never trap it.
function idolTemple() {
    return {
        walls: [
            wall('idolFront', 'plastic', [[-0.026, 0.650], [-0.026, 0.775], [-0.012, 0.805], [0, 0.830], [0.012, 0.805], [0.026, 0.775], [0.026, 0.650]], { height: 0.04 })
        ],
        devices: [
            { id: 'sun', type: 'dropBank', angle: SUN_ANGLE, width: 0.022, targets: SUN.map(([id, x, y]) => ({ id, x, y })) },
            { id: 'idolDoors', type: 'door', a: [-0.026, 0.660], b: [0.026, 0.660], open: false, material: 'plastic' },
            { id: 'idolScoop', type: 'hole', kind: 'scoop', x: 0, y: 0.693, r: 0.020, capacity: 1, eject: { path: 'idolExit', speed: 1.8 } },
            standup('jewelL', -0.070, 0.545), standup('jewelR', 0.070, 0.545)
        ],
        paths: [{
            id: 'idolExit', kind: 'tube', entry: { a: [-0.017, 0.693], b: [0.017, 0.693] }, minSpeed: CHAINED, width: 0.034, color: GOLD,
            points: [[0, 0.693, 0], [0, 0.675, 0.016], [0, 0.640, 0.032], [0, 0.585, 0.022], [0, 0.565, 0.004]],
            exit: { kind: 'field', dir: [-0.18, -1], speedScale: 0.5 }
        }]
    };
}

// Orbit lock in the right orbit lane: captures only while enabled; ejects down the lane into the upper flipper.
function orbitLock() {
    return {
        devices: [{ id: 'orbitLock', type: 'hole', kind: 'kickhole', x: 0.2055, y: 0.770, r: 0.016, capacity: 3, hidden: true, enabled: false, captureSpeed: 4.5, eject: { angleDeg: 270, speed: 0.6 } }]
    };
}

function orbitsAndRamps() {
    const lc = leftChannel(), rc = rightChannel();
    // A roof over the corridor between the left orbit guide and the channel keeps balls from settling in its bottom.
    const lw = lc.walls.map(w => w.id === 'chanL_right' ? { ...w, points: [[-0.132, 0.570], [-0.132, 0.655]] } : w);
    lw.push(wall('chanL_roof', 'plastic', [[-0.2215, 0.700], [-0.180, 0.670]], { height: 0.04 }));
    return {
        walls: [...lw, ...rc.walls, wall('pyramid', 'plastic', [[0.095, 0.900], [0.155, 0.900], [0.155, 0.940], [0.125, 0.965], [0.095, 0.940]], { closed: true, render: 'none', height: 0.05 })],
        allowGaps: [...lc.allowGaps, ...rc.allowGaps],
        devices: [{ id: 'spinnerL', type: 'spinner', a: [-0.2575, 0.660], b: [-0.2215, 0.660] }],
        paths: [
            // Stone ramp: up the left, across the top at h 0.10, around the pyramid, down the right to the right inlane.
            { id: 'leftRamp', kind: 'plastic', entry: lc.entry, width: 0.05, color: STONE,
              points: [[-0.156, 0.625, 0], [-0.156, 0.700, 0.035], [-0.150, 0.790, 0.068], [-0.110, 0.870, 0.090]],
              exit: { kind: 'path', path: 'leftReturn' } },
            { id: 'leftReturn', kind: 'wire', entry: { a: [-0.130, 0.860], b: [-0.090, 0.860] }, minSpeed: CHAINED, width: 0.034, color: STONE,
              points: [[-0.110, 0.870, 0.090], [-0.040, 0.925, 0.100], [0.040, 0.935, 0.100], [0.100, 0.990, 0.105], [0.150, 0.990, 0.105], [0.200, 0.900, 0.100], [0.208, 0.700, 0.085], [0.205, 0.560, 0.060], [0.192, 0.490, 0.035], [0.172, 0.430, 0.008], [0.1685, 0.418, 0]],
              exit: { kind: 'field', dir: [0, -1], speedScale: 0.7 } },
            // Gold ramp: up the right, over the top at h 0.155, down the left to the left inlane.
            { id: 'rightRamp', kind: 'wire', entry: rc.entry, width: 0.034, color: GOLD,
              points: [[0.128, 0.640, 0], [0.130, 0.720, 0.040], [0.125, 0.800, 0.085], [0.090, 0.875, 0.125], [0.020, 0.920, 0.155], [-0.080, 0.940, 0.155], [-0.150, 0.885, 0.130]],
              exit: { kind: 'path', path: 'rightReturn' } },
            { id: 'rightReturn', kind: 'wire', entry: { a: [-0.170, 0.870], b: [-0.130, 0.870] }, minSpeed: CHAINED, width: 0.034, color: GOLD,
              points: [[-0.150, 0.885, 0.130], [-0.205, 0.800, 0.110], [-0.218, 0.680, 0.085], [-0.212, 0.540, 0.050], [-0.2025, 0.440, 0.012], [-0.2025, 0.425, 0]],
              exit: { kind: 'field', dir: [0, -1], speedScale: 0.7 } }
        ]
    };
}

export const layout = composeLayout(
    { id: 'temple', version: 1, meta: {
        title: 'Lost Temple of the Sun', accent: GOLD, secondary: JADE, cx: CX, troughSize: 4,
        giColor: { lower: '#ffd59a', upper: '#ffb347' },
        attract: [['sweepUp', 1600], ['ring', 1600], ['spiral', 2000], ['sparkle', 2400], ['radial', 1500], ['chase', 1800], ['breath', 1600]]
    } },
    shell(),
    lowerKit({ cx: CX, kickback: 'left' }),
    topArea(), idolTemple(), orbitLock(), orbitsAndRamps(),
    {
        flippers: [{ id: 'upperRight', key: 'right', x: 0.175, y: 0.490, side: -1, length: 0.050 }],
        shots: [
            { id: 'leftOrbit', on: 'trigger:leftOrbitUp@1', combo: true, bonus: 'orbits', setup: [] },
            { id: 'leftRamp', on: 'pathMade:leftRamp', combo: true, bonus: 'ramps', setup: [] },
            { id: 'idol', on: 'hole:idolScoop', combo: true, setup: [{ type: 'door', id: 'idolDoors', open: true }, { type: 'dropBank', id: 'sun' }] },
            { id: 'rightRamp', on: 'pathMade:rightRamp', combo: true, bonus: 'ramps', setup: [] },
            { id: 'rightOrbit', on: 'trigger:rightOrbitUp@1', combo: true, bonus: 'orbits', setup: [] },
            { id: 'lock', on: 'hole:orbitLock', combo: true, setup: [{ type: 'holeEnable', id: 'orbitLock', on: true }] }
        ],
        lamps: templeLamps(), flashers: templeFlashers(), gi: templeGI(),
        toys: [
            { id: 'idolHead', kind: 'idol', x: 0, y: 0.790, h: 0, rot: 0, scale: 1 },
            { id: 'pyramidToy', kind: 'pyramid', x: 0.125, y: 0.930, h: 0, rot: 0, scale: 1 },
            { id: 'doorsToy', kind: 'doors', x: 0, y: 0.660, h: 0, rot: 0, scale: 1 },
            { id: 'sunDisc', kind: 'sunDisc', x: 0, y: 1.0, h: 0, rot: 0, scale: 1 },
            { id: 'foliageL', kind: 'foliage', x: -0.2575, y: 0.70, h: 0, rot: 0, scale: 1 },
            { id: 'foliageR', kind: 'foliage', x: 0.2235, y: 0.62, h: 0, rot: Math.PI, scale: 1 }
        ],
        labels: [
            { key: 'lamp_lock', x: 0.1625, y: 0.735, size: 0.007, angle: 0, color: JADE },
            { key: 'lamp_jackpot', x: -0.156, y: 0.560, size: 0.007, angle: 0, color: GOLD }
        ]
    }
);

export default layout;
