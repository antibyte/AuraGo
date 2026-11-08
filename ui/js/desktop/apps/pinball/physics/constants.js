// Physics constants (SI). DOM-free.
export const STEP = 1 / 480;
export const BALL_R = 0.0135;
export const BALL_M = 0.08;
export const G = 9.81;
export const TILT_RAD = 6.5 * Math.PI / 180;
export const G_SLOPE = G * Math.sin(TILT_RAD);
export const G_NORMAL = G * Math.cos(TILT_RAD);
export const MAX_SPEED = 12;
export const SUBSTEP_TRAVEL = 0.35 * BALL_R;
export const SUBSTEP_MIN = 2;
export const SUBSTEP_MAX = 32;

export const ROLL_DECEL = Object.freeze({ field: 0.055, plastic: 0.16, wire: 0.26, tube: 0.30 });

// e = vn < 0.03 ? 0 : clamp(e0 - eSlope * vn, eMin, e0); mu is the friction coefficient.
export const MATERIALS = Object.freeze({
    wood: Object.freeze({ e0: 0.55, eSlope: 0.05, eMin: 0.35, mu: 0.15 }),
    metal: Object.freeze({ e0: 0.45, eSlope: 0.04, eMin: 0.30, mu: 0.10 }),
    plastic: Object.freeze({ e0: 0.50, eSlope: 0.05, eMin: 0.30, mu: 0.20 }),
    rubber: Object.freeze({ e0: 0.88, eSlope: 0.07, eMin: 0.55, mu: 0.60 }),
    flipper: Object.freeze({ e0: 0.82, eSlope: 0.03, eMin: 0.60, mu: 0.80 }),
    target: Object.freeze({ e0: 0.45, eSlope: 0.04, eMin: 0.30, mu: 0.20 }),
    ball: Object.freeze({ e0: 0.93, eSlope: 0, eMin: 0.93, mu: 0.05 })
});

export function restitution(material, vn) {
    const m = MATERIALS[material] || MATERIALS.metal;
    if (vn < 0.03) return 0;
    return Math.max(m.eMin, Math.min(m.e0, m.e0 - m.eSlope * vn));
}

export const FLIPPER = Object.freeze({
    length: 0.059, rBase: 0.012, rTip: 0.006, rest: -0.5236, up: 0.4887,
    alphaUp: 3000, omegaMax: 50, alphaDown: 1200, omegaDownMax: 26, stopRebound: 0.15, restRebound: 0.25
});

// Playfield extents (m).
export const FIELD = Object.freeze({ xMin: -0.2575, xMax: 0.2575, yMin: 0, yMax: 1.07 });
