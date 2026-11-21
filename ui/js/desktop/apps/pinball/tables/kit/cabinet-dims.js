// Single source of truth for cabinet dimensions (DOM-free). Render (cabinet.js, camera.js, ramps, clearance)
// and validation (validate-paths.js) must import from here instead of hard-coding glass heights.
import { BALL_R } from '../../physics/constants.js';

export const CABINET = Object.freeze({
    frontY: -0.125, backY: 1.09,          // lockdown bar edge / back wall (playfield y, m)
    sideIn: 0.2675, sideOut: 0.2975, bottom: -0.24,
    glassFront: 0.065, glassBack: 0.14,   // underside of the glass above the playfield surface (m)
    glassThickness: 0.005, railLip: 0.006 // side-rail cap rises this much above the glass top
});
export const GLASS_MARGIN = 0.008;        // required air gap between any path top and the glass
export const OUTLINE_MARGIN = 0.004;      // path footprint to outer wall
export const CROSS_SEP = Math.round((2 * BALL_R + 0.010) * 1e6) / 1e6; // 2*BALL_R + 0.010 (= 0.037): min vertical separation of overlapping paths
export const PATH_TOP = Object.freeze({ wire: 0.029, plastic: 0.029, tube: 0.034 }); // highest point above the path line

export function glassHeight(y) {
    const t = Math.min(1, Math.max(0, (y - CABINET.frontY) / (CABINET.backY - CABINET.frontY)));
    return CABINET.glassFront + (CABINET.glassBack - CABINET.glassFront) * t;
}

export function topAt(y) {
    return glassHeight(y) + CABINET.glassThickness + CABINET.railLip;
}

export function pathHalfWidth(def) {
    const kind = def && def.kind;
    if (kind === 'tube') return 0.020;
    if (kind === 'wire') return Math.max(((def && def.width) ?? 0.034) / 2, 0.018);
    return ((def && def.width) ?? 0.05) / 2 + 0.005;
}

export function pathTop(def) {
    if (def && Number.isFinite(def.top)) return def.top;
    const kind = def && Object.prototype.hasOwnProperty.call(PATH_TOP, def.kind) ? def.kind : 'plastic';
    return PATH_TOP[kind];
}

export function maxPathHeight(y, kind) {
    const k = Object.prototype.hasOwnProperty.call(PATH_TOP, kind) ? kind : 'plastic';
    return glassHeight(y) - PATH_TOP[k] - GLASS_MARGIN;
}
