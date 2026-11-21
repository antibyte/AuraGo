// Path fit rules for the table validator: ramps, habitrails and tubes must fit under the glass, inside the outer wall and
// above or below every path they cross. DOM-free. Dimensions come from kit/cabinet-dims.js.
import { createPaths } from '../physics/paths.js';
import { glassHeight, pathHalfWidth, pathTop, GLASS_MARGIN, OUTLINE_MARGIN, CROSS_SEP } from './kit/cabinet-dims.js';
import { pointInPolygon, distPointSeg } from './kit/geom.js';

const EPS = 1e-9;
const SHARED_ARC = 0.07;      // m of arc length round a shared endpoint where overlap is exempt (chained paths, fan-outs)
const SHARED_END = 0.002;     // endpoints of two paths this close (m, in 3D plan) count as shared

const mm = v => (v * 1000).toFixed(1);
const at = s => '(' + s[0].toFixed(3) + ', ' + s[1].toFixed(3) + ')';

export function checkPathFit(layout, err) {
    const defs = (Array.isArray(layout.paths) ? layout.paths : []).filter(d => d && d.id !== undefined);
    if (!defs.length) return;
    const dbg = createPaths(defs, { layout }).debug();
    const items = dbg.map((d, i) => {
        const def = defs[i], arc = [0];
        for (let k = 1; k < d.samples.length; k++) arc.push(arc[k - 1] + Math.hypot(d.samples[k][0] - d.samples[k - 1][0], d.samples[k][1] - d.samples[k - 1][1], d.samples[k][2] - d.samples[k - 1][2]));
        return { id: d.id, def, samples: d.samples, arc, half: pathHalfWidth(def), top: pathTop(def) };
    });
    for (const it of items) { checkGlass(it, err); checkOutline(layout, it, err); }
    checkCrossings(items, err);
}

function checkGlass(it, err) {
    let worst = null;
    for (const s of it.samples) {
        const need = s[2] + it.top + GLASS_MARGIN - glassHeight(s[1]);
        if (need > EPS && (!worst || need > worst.need)) worst = { need, s };
    }
    if (worst) err('path-glass', 'path "' + it.id + '" at ' + at(worst.s) + ' h ' + worst.s[2].toFixed(3) + ' needs ' + mm(worst.need) + ' mm more glass');
}

function checkOutline(layout, it, err) {
    const outer = (layout.walls || []).find(w => w.id === 'outer');
    if (!outer || !Array.isArray(outer.points) || outer.points.length < 3) return;
    const poly = outer.points, need = it.half + OUTLINE_MARGIN;
    let worst = null;
    for (const s of it.samples) {
        let d = Infinity;
        for (let i = 0; i < poly.length; i++) {
            const a = poly[i], b = poly[(i + 1) % poly.length];
            d = Math.min(d, distPointSeg(s[0], s[1], a[0], a[1], b[0], b[1]));
        }
        const margin = pointInPolygon([s[0], s[1]], poly) ? d : -d;
        if (margin < need - EPS && (!worst || margin < worst.margin)) worst = { margin, s };
    }
    if (worst) err('path-outline', 'path "' + it.id + '" at ' + at(worst.s) + ' is ' + mm(worst.margin) + ' mm from the outer wall (needs ' + mm(need) + ' mm)');
}

// True when sample k of an item lies within SHARED_ARC of an endpoint that coincides with an endpoint of the other item.
function nearShared(a, b) {
    const ends = it => [it.samples[0], it.samples[it.samples.length - 1]];
    const flags = it => new Uint8Array(it.samples.length);
    const fa = flags(a), fb = flags(b);
    const pairs = [];
    ends(a).forEach((p, ia) => ends(b).forEach((q, ib) => {
        if (Math.hypot(p[0] - q[0], p[1] - q[1], p[2] - q[2]) <= SHARED_END) pairs.push([ia, ib]);
    }));
    const mark = (it, f, end) => {
        const total = it.arc[it.arc.length - 1];
        for (let k = 0; k < f.length; k++) if ((end === 0 ? it.arc[k] : total - it.arc[k]) <= SHARED_ARC) f[k] = 1;
    };
    for (const [ia, ib] of pairs) { mark(a, fa, ia); mark(b, fb, ib); }
    return [fa, fb];
}

function checkCrossings(items, err) {
    for (let i = 0; i < items.length; i++) for (let j = i + 1; j < items.length; j++) {
        const A = items[i], B = items[j], reach = A.half + B.half, [fa, fb] = nearShared(A, B);
        let worst = null;
        for (let p = 0; p < A.samples.length; p++) {
            const s = A.samples[p];
            for (let q = 0; q < B.samples.length; q++) {
                const t = B.samples[q];
                if (Math.hypot(s[0] - t[0], s[1] - t[1]) >= reach) continue;
                if (fa[p] && fb[q]) continue;
                const sep = Math.abs(s[2] - t[2]);
                if (sep < CROSS_SEP - EPS && (!worst || sep < worst.sep)) worst = { sep, s, t };
            }
        }
        if (worst) err('path-cross', 'paths "' + A.id + '" and "' + B.id + '" overlap at ' + at(worst.s) + ' with only ' + mm(worst.sep) + ' mm of height between them (needs ' + mm(CROSS_SEP) + ' mm)');
    }
}
