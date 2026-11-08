// Cyber Neon Metropolis: the physical playfield (plan 6.1). Shell + lower kit + both ramp channels + top area,
// Data Core, FIREWALL/VAULT, orbits, ramps, standups. DOM-free.
import { composeLayout } from '../kit/compose.js';
import { shell, leftChannel, rightChannel } from '../kit/shell.js';
import { lowerKit, CX } from '../kit/lower.js';
import { wall, post } from '../kit/geom.js';
import { neonLamps, neonFlashers, neonGI } from './lamps.js';

const PINK = '#ff48c4', CYAN = '#37d9ff', VIOLET = '#a98bff';
// Paths that are only entered from another path or a hole must never catch a field ball.
const CHAINED = 99;
const FW_ANGLE = Math.atan2(-0.906, 0.423);

const standup = (id, x, y) => ({ id, type: 'standup', x, y, angle: Math.atan2(0.20 - y, CX - x), width: 0.020 });

function topArea() {
    const guides = [-0.0675, -0.0225, 0.0225, 0.0675].map((x, i) => wall('laneGuide' + (i + 1), 'metal', [[x, 0.905], [x, 0.955]]));
    return {
        walls: guides,
        devices: [
            ...[-0.045, 0, 0.045].map((x, i) => ({ id: 'laneT' + (i + 1), type: 'rollover', x, y: 0.930 })),
            { id: 'popL', type: 'bumper', x: -0.048, y: 0.836 }, { id: 'popR', type: 'bumper', x: 0.045, y: 0.845 }, { id: 'popB', type: 'bumper', x: 0, y: 0.778 }
        ]
    };
}

function dataCore() {
    return {
        walls: [
            wall('coreChanL', 'metal', [[-0.024, 0.585], [-0.024, 0.693]]),
            wall('coreChanR', 'metal', [[0.024, 0.585], [0.024, 0.693]])
        ],
        posts: [post('spireBase', 0, 0.700, 0.025, 'metal', 'none')],
        devices: [{ id: 'dataCore', type: 'hole', kind: 'vuk', x: 0, y: 0.640, r: 0.017, capacity: 1, eject: { path: 'coreTube', speed: 2.6 } }],
        paths: [{
            id: 'coreTube', kind: 'tube', entry: { a: [-0.017, 0.640], b: [0.017, 0.640] }, minSpeed: CHAINED, width: 0.034, color: CYAN,
            points: [[0, 0.640, 0], [0, 0.650, 0.06], [0.005, 0.730, 0.090], [0, 0.800, 0.070], [0, 0.880, 0.03]],
            exit: { kind: 'field', dir: [0, -1], speedScale: 0.3 }
        }]
    };
}

function firewall() {
    return {
        walls: [wall('vaultWall', 'metal', [[-0.139, 0.695], [-0.148, 0.750], [-0.112, 0.790], [-0.065, 0.770], [-0.069, 0.727]])],
        devices: [
            { id: 'firewall', type: 'dropBank', angle: FW_ANGLE, width: 0.024, targets: [
                { id: 'fw1', x: -0.128, y: 0.700 }, { id: 'fw2', x: -0.104, y: 0.711 }, { id: 'fw3', x: -0.080, y: 0.722 }] },
            { id: 'vault', type: 'hole', kind: 'kickhole', x: -0.121, y: 0.724, r: 0.016, capacity: 3, hidden: true, eject: { angleDeg: 295, speed: 1.4 } },
            { id: 'lockDiverter', type: 'diverter', at: [-0.150, 0.885, 0.130], on: false }
        ]
    };
}

function orbitsAndRamps() {
    const lc = leftChannel(), rc = rightChannel();
    // The left channel's right wall stops short of the VAULT enclosure (a 26 mm gap would swallow a ball); a roof over the
    // corridor between the orbit guide and the channel keeps balls from settling in its bottom.
    const lw = lc.walls.map(w => w.id === 'chanL_right' ? { ...w, points: [[-0.132, 0.570], [-0.132, 0.655]] } : w);
    lw.push(wall('chanL_roof', 'plastic', [[-0.2215, 0.700], [-0.180, 0.670]], { height: 0.04 }));
    return {
        walls: [...lw, ...rc.walls],
        allowGaps: [...lc.allowGaps, ...rc.allowGaps],
        devices: [{ id: 'spinnerL', type: 'spinner', a: [-0.2575, 0.660], b: [-0.2215, 0.660] }],
        paths: [
            { id: 'leftRamp', kind: 'plastic', entry: lc.entry, width: 0.05, color: PINK,
              points: [[-0.156, 0.625, 0], [-0.156, 0.700, 0.035], [-0.150, 0.790, 0.068], [-0.110, 0.870, 0.090]],
              exit: { kind: 'path', path: 'leftReturn' } },
            { id: 'leftReturn', kind: 'wire', entry: { a: [-0.130, 0.860], b: [-0.090, 0.860] }, minSpeed: CHAINED, width: 0.034, color: PINK,
              points: [[-0.110, 0.870, 0.090], [0, 0.935, 0.100], [0.120, 0.900, 0.100], [0.200, 0.780, 0.090], [0.208, 0.620, 0.065], [0.192, 0.490, 0.035], [0.172, 0.430, 0.008], [0.1685, 0.418, 0]],
              exit: { kind: 'field', dir: [0, -1], speedScale: 0.7 } },
            { id: 'rightRamp', kind: 'wire', entry: rc.entry, width: 0.034, color: CYAN,
              points: [[0.128, 0.640, 0], [0.130, 0.720, 0.040], [0.130, 0.800, 0.085], [0.100, 0.880, 0.125], [0.020, 0.965, 0.155], [-0.080, 0.955, 0.150], [-0.150, 0.885, 0.130]],
              exit: { kind: 'switch', diverter: 'lockDiverter', on: { kind: 'path', path: 'vaultFeed' }, off: { kind: 'path', path: 'rightReturn' } } },
            { id: 'rightReturn', kind: 'wire', entry: { a: [-0.170, 0.870], b: [-0.130, 0.870] }, minSpeed: CHAINED, width: 0.034, color: CYAN,
              points: [[-0.150, 0.885, 0.130], [-0.205, 0.800, 0.110], [-0.218, 0.680, 0.085], [-0.212, 0.540, 0.050], [-0.2025, 0.440, 0.012], [-0.2025, 0.425, 0]],
              exit: { kind: 'field', dir: [0, -1], speedScale: 0.7 } },
            { id: 'vaultFeed', kind: 'tube', entry: { a: [-0.170, 0.870], b: [-0.130, 0.870] }, minSpeed: CHAINED, width: 0.034, color: VIOLET,
              points: [[-0.150, 0.885, 0.130], [-0.165, 0.895, 0.06], [-0.172, 0.900, 0.03]],
              exit: { kind: 'hole', hole: 'vault' } }
        ]
    };
}

export const layout = composeLayout(
    { id: 'neon', version: 1, meta: {
        title: 'Cyber Neon Metropolis', accent: CYAN, secondary: PINK, cx: CX, troughSize: 4,
        giColor: { lower: '#d8f0ff', upper: '#c9b6ff' },
        attract: [['sweepUp', 1600], ['chase', 2400], ['radial', 1500], ['sparkle', 2500], ['spiral', 2000], ['strobe', 600], ['rainbow', 3000]]
    } },
    shell(),
    lowerKit({ cx: CX, kickback: 'left' }),
    topArea(), dataCore(), firewall(), orbitsAndRamps(),
    { allowGaps: [['firewall', 'vaultWall']] },
    {
        flippers: [{ id: 'upperRight', key: 'right', x: 0.170, y: 0.780, side: -1, length: 0.050 }],
        devices: [
            standup('neonN', -0.100, 0.525), standup('neonE', -0.064, 0.585),
            standup('neonO', 0.064, 0.585), standup('neonN2', 0.068, 0.525)
        ],
        shots: [
            { id: 'leftOrbit', on: 'trigger:leftOrbitUp@1', combo: true, bonus: 'orbits', setup: [] },
            { id: 'leftRamp', on: 'pathMade:leftRamp', combo: true, bonus: 'ramps', setup: [] },
            { id: 'vault', on: 'hole:vault', combo: true, setup: [{ type: 'dropBank', id: 'firewall' }] },
            { id: 'core', on: 'hole:dataCore', combo: true, setup: [] },
            { id: 'rightRamp', on: 'pathMade:rightRamp', combo: true, bonus: 'ramps', setup: [] },
            { id: 'rightOrbit', on: 'trigger:rightOrbitUp@1', combo: true, bonus: 'orbits', setup: [] }
        ],
        lamps: neonLamps(), flashers: neonFlashers(), gi: neonGI(),
        toys: [
            { id: 'skyline', kind: 'skyline', x: 0, y: 1.0, h: 0, rot: 0, scale: 1 },
            { id: 'spire', kind: 'spire', x: 0, y: 0.700, h: 0, rot: 0, scale: 1 },
            { id: 'subway', kind: 'subway', x: -0.172, y: 0.900, h: 0.03, rot: 0, scale: 1 }
        ],
        labels: [
            { key: 'lamp_lock', x: -0.1155, y: 0.642, size: 0.007, angle: 0, color: VIOLET },
            { key: 'lamp_jackpot', x: -0.156, y: 0.560, size: 0.007, angle: 0, color: PINK }
        ]
    }
);

export default layout;
