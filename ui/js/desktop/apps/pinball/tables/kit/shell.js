// Shared playfield shell: outer walls, arc, shooter lane, orbit lanes, ramp entry channels (absolute x). DOM-free.
import { arcPoints, wall, post } from './geom.js';

export const SHELL = Object.freeze({ half: 0.2575, arcY: 0.8125, laneX: 0.2235 });

export const PLUNGER = Object.freeze({
    x: 0.2405, yRest: 0.0535, travel: 0.06, pullRate: 1.6, vMin: 0.55, vMax: 3.4, vAuto: 3.0, laneX: [0.2235, 0.2575], zoneYMax: 0.12
});

export function shell() {
    const { half, arcY, laneX } = SHELL;
    const walls = [];
    // Outer wall: left side up, 37-point arc over the top, right side down.
    walls.push(wall('outer', 'wood', [[-half, 0], ...arcPoints(0, arcY, half, 180, 0, 37), [half, 0]]));
    walls.push(wall('shooterWall', 'wood', [[laneX, 0], [laneX, 0.770]]));

    // Left orbit lane.
    const lIn = arcPoints(-0.170, 0.620, 0.0515, 180, 240, 7), lOut = arcPoints(-0.170, 0.620, 0.0875, 180, 240, 7);
    walls.push(wall('orbitL_inner', 'metal', [[-0.2215, 0.620], [-0.2215, arcY], ...arcPoints(0, arcY, 0.2215, 180, 150, 7).slice(1)]));
    walls.push(wall('orbitL_curveIn', 'metal', lIn));
    walls.push(wall('orbitL_curveOut', 'metal', lOut));
    walls.push(wall('orbitL_fill', 'wood', [lOut[lOut.length - 1], [-half, 0.500]]));

    // Right orbit lane.
    const rIn = arcPoints(0.1395, 0.620, 0.048, 0, -60, 7), rOut = arcPoints(0.1395, 0.620, 0.084, 0, -60, 7);
    walls.push(wall('orbitR_inner', 'metal', [[0.1875, 0.620], [0.1875, 0.930]]));
    walls.push(wall('orbitR_curveIn', 'metal', rIn));
    walls.push(wall('orbitR_curveOut', 'metal', rOut));
    walls.push(wall('orbitR_fill', 'wood', [rOut[rOut.length - 1], [laneX, 0.500]]));

    return {
        walls,
        posts: [post('shooterPost', laneX, 0.775, 0.004)],
        devices: [
            { id: 'shooterGate', type: 'gate', a: [laneX, 0.800], b: [half, 0.835], pass: [0, 1], mode: 'oneWay' },
            { id: 'leftOrbitUp', type: 'trigger', a: [-half, 0.700], b: [-0.2215, 0.700] },
            { id: 'rightOrbitUp', type: 'trigger', a: [0.1875, 0.700], b: [laneX, 0.700] }
        ],
        plunger: { ...PLUNGER, laneX: [...PLUNGER.laneX] },
        drainY: 0.075,
        // The pockets between the orbit bottoms and the outer walls are sealed wedges.
        allowGaps: [
            ['orbitR_curveOut', 'shooterWall'], ['orbitL_curveOut', 'outer']
        ]
    };
}

// Ramp entry channels shared by all tables (plan section 6). Tables add them where a ramp path starts.
export function leftChannel() {
    return {
        walls: [
            wall('chanL_left', 'plastic', [[-0.1958, 0.5754], [-0.180, 0.598], [-0.180, 0.670]], { height: 0.04 }),
            wall('chanL_right', 'plastic', [[-0.132, 0.570], [-0.132, 0.670]], { height: 0.04 })
        ],
        allowGaps: [['chanL_left', 'orbitL_curveIn']],
        entry: { a: [-0.180, 0.625], b: [-0.132, 0.625] }
    };
}

export function rightChannel() {
    return {
        walls: [
            wall('chanR_left', 'plastic', [[0.104, 0.590], [0.104, 0.690]], { height: 0.04 }),
            wall('chanR_right', 'plastic', [[0.1635, 0.5784], [0.152, 0.600], [0.152, 0.690], [0.1875, 0.715]], { height: 0.04 })
        ],
        allowGaps: [['chanR_right', 'orbitR_curveIn']],
        entry: { a: [0.104, 0.640], b: [0.152, 0.640] }
    };
}
