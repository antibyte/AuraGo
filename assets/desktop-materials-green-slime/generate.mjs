// Generates the three gel tiles of the Green Slime app materials (ui/css/desktop-materials-green-slime.css):
//   --vd-mat-canvas  a few soft gel bubbles floating in the moss background,
//   --vd-mat-bar     a gel lip along the TOP edge of a bar with fat drops hanging down into the bar's top padding,
//   --vd-mat-side    a glossy gel rail on the RIGHT edge of a sidebar with beads sliding down it.
//
// Usage: node assets/desktop-materials-green-slime/generate.mjs [previewDir] [--css=<file>] [--keep]
//   - prints the three CSS values (ready to paste into the stylesheet) and the byte size of each data URI,
//   - with a previewDir it also writes preview.html (mock Code Studio window, status bars, sidebar, canvas, raw tiles)
//     plus bar.svg, side.svg and canvas.svg there. The preview takes every other token from the stylesheet and
//     overrides --vd-mat-canvas, --vd-mat-bar and --vd-mat-side with the fresh values; --css= previews another
//     stylesheet, --keep previews it unchanged.
//
// Deterministic (no randomness), no dependencies, no external references. Edit the constants, not the output.
// All geometry is written in CSS px; each SVG uses a viewBox that is UNIT times larger, so every path number is a short
// integer on a 1/UNIT px grid.
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const args = process.argv.slice(2);
const flag = (name) => args.find((a) => a.startsWith(`--${name}`));
const CSS_FILE = flag('css=') ? resolve(flag('css=').slice(6)) : resolve(here, '../../ui/css/desktop-materials-green-slime.css');
const outDir = args.find((a) => !a.startsWith('--'));

let UNIT = 20; // viewBox units per CSS px of the tile being built
const TAU = Math.PI * 2;
const K = 0.5523; // a quarter ellipse is one cubic whose handles are K * radius long

// ---------------------------------------------------------------------------------------------------------
// Palette: lime gel on moss (see ui/css/desktop-theme-green-slime.css)
// ---------------------------------------------------------------------------------------------------------
const GEL = {
    pale: '#D4F58C',  // thin gel at the lip, lit and milky
    mid: '#A5D941',   // theme lime
    deep: '#7FB02A',  // thick gel in the bulbs
    core: '#4C8A16',  // darker core of a bulb towards its tip
    rim: '#5C9022',   // darker refracted rim just inside the edge
    edge: '#3F6614',  // dark outline that defines the gel on the dark moss
    white: '#FFFFFF', // crisp specular highlight
    glow: '#F4FFCC',  // light that passes through a bulb and lands on its far side
    opacity: '.9',    // translucent body
};

// ---------------------------------------------------------------------------------------------------------
// Bar tile: lip + fat drops. Lengths are CSS px measured below the sagging lower edge of the lip.
// ---------------------------------------------------------------------------------------------------------
const BAR = {
    w: 420,
    h: 12,
    // Lower edge of the lip: base + sine waves (amplitude px, whole cycles per tile, phase), so the tile repeats seamlessly
    // and the edge sags softly between the drops instead of running like a ruler.
    lip: { base: 3.5, waves: [[0.5, 3, 0.9], [0.2, 7, 2.4]], step: 40 },
    // Shoulders: the half width where a drop leaves the lip, as a multiple of the bulb half width (about 1.6 x the bulb width).
    shoulder: 1.6,
    // x = drop centre, tip = depth of the lowest point below the bar's top edge (at most 11), r = bulb half width (the bulb is 2 r wide), neck = narrowest half width as a
    // fraction of r (1 = no neck), ry = bulb height as a fraction of r, neckAt = where the neck sits between lip and bulb
    // centre, lean = bulb offset, skew = left/right asymmetry. Irregular spacing: one long, one medium, two short drops.
    drips: [
        { x: 50, tip: 10.9, r: 5.5, neck: 0.66, ry: 1.1, neckAt: 0.45, lean: 0.3, skew: 0.05 },
        { x: 152, tip: 9.8, r: 6.0, neck: 0.72, ry: 1.02, neckAt: 0.45, lean: -0.2, skew: -0.05 },
        { x: 218, tip: 8.5, r: 4.8, neck: 0.8, ry: 1.0, neckAt: 0.45, lean: 0.2, skew: 0.06 },
        { x: 352, tip: 10.3, r: 5.8, neck: 0.68, ry: 1.06, neckAt: 0.45, lean: -0.3, skew: -0.04 },
    ],
    // Crisp highlight line along the lip: [x0, x1] pairs with small gaps between them.
    lipGlints: [[12, 84], [96, 190], [204, 268], [282, 340], [356, 412]],
    lipGlintAbove: 2.3, // distance of the line above the lower edge of the lip
};

// ---------------------------------------------------------------------------------------------------------
// Side tile: rail flush with the right edge + beads (tail up, bulb down) + one droplet that left the first bead
// ---------------------------------------------------------------------------------------------------------
const SIDE = {
    w: 12,
    h: 240,
    rail: { base: 9.1, waves: [[0.22, 3, 1.1], [0.12, 8, 0.4]], step: 30 }, // left edge of the rail, x(y)
    // top = where the tail leaves the rail, len = tail tip to bottom of the bulb, bulge = how far it reaches left of the rail.
    beads: [
        { top: 14, len: 37, bulge: 4.6 },
        { top: 98, len: 26, bulge: 3.6 },
        { top: 156, len: 41, bulge: 5.2 },
    ],
    drops: [{ x: 10.2, top: 62, len: 8.5, r: 1.7 }], // centre x, top y, height, half width
    railGlints: [[3, 40], [52, 92], [104, 150], [166, 214], [224, 236]], // [y0, y1] of the highlight line on the rail
    tipX: 10.4, // where every tail merges into the rail
};

// ---------------------------------------------------------------------------------------------------------
// Canvas tile: a handful of soft gel bubbles, irregular in size and place
// ---------------------------------------------------------------------------------------------------------
const CANVAS = {
    w: 520,
    h: 380,
    unit: 4,
    // [cx, cy, radius]; none touches the tile edge, so the tile repeats without cut bubbles.
    bubbles: [[92, 66, 27], [238, 30, 8], [364, 116, 35], [178, 192, 15], [458, 262, 21], [58, 296, 12], [286, 322, 30]],
    // Brightness stays tiny so text on the canvas keeps its contrast: rim 0.15, specular 0.14 (see README, "Contrast").
    rim: 0.15,
    spec: 0.14,
};

// ---------------------------------------------------------------------------------------------------------
// Path helpers: geometry in CSS px, output as compact relative integer path data
// ---------------------------------------------------------------------------------------------------------
const u = (v) => Math.round(v * UNIT);
function ints(list) {
    let out = '';
    for (const v of list) {
        const s = String(v === 0 ? 0 : v);
        out += out && s[0] !== '-' ? ' ' + s : s;
    }
    return out;
}

class Pen {
    constructor(x, y) {
        this.x = u(x);
        this.y = u(y);
        this.d = 'M' + ints([this.x, this.y]);
        this.cmd = 'M';
    }
    emit(cmd, values) {
        const s = ints(values);
        this.d += this.cmd === cmd ? (s[0] === '-' ? '' : ' ') + s : cmd + s;
        this.cmd = cmd;
    }
    C(x1, y1, x2, y2, x, y) {
        const p = [x1, y1, x2, y2, x, y].map(u);
        this.emit('c', [p[0] - this.x, p[1] - this.y, p[2] - this.x, p[3] - this.y, p[4] - this.x, p[5] - this.y]);
        this.x = p[4];
        this.y = p[5];
    }
    Q(x1, y1, x, y) {
        const p = [x1, y1, x, y].map(u);
        this.emit('q', [p[0] - this.x, p[1] - this.y, p[2] - this.x, p[3] - this.y]);
        this.x = p[2];
        this.y = p[3];
    }
    V(y) {
        const p = u(y);
        this.emit('v', [p - this.y]);
        this.y = p;
    }
    L(x, y) {
        const p = [x, y].map(u);
        this.emit('l', [p[0] - this.x, p[1] - this.y]);
        this.x = p[0];
        this.y = p[1];
    }
    Z() {
        this.d += 'z';
        this.cmd = 'z';
        return this.d;
    }
}

// f(t) = base + sum of sines, with its derivative
function wave({ base, waves }, period) {
    const f = (t) => base + waves.reduce((s, [a, k, p]) => s + a * Math.sin((TAU * k * t) / period + p), 0);
    const df = (t) => waves.reduce((s, [a, k, p]) => s + ((a * TAU * k) / period) * Math.cos((TAU * k * t) / period + p), 0);
    return { f, df };
}

// A thin lens (a crescent that tapers to points): start S, end E, arc midpoint M, unit normal n at M, thickness at the middle.
function lens(S, M, E, n, thick) {
    const cx = 2 * M[0] - (S[0] + E[0]) / 2;
    const cy = 2 * M[1] - (S[1] + E[1]) / 2;
    const pen = new Pen(S[0], S[1]);
    pen.Q(cx + n[0] * thick, cy + n[1] * thick, E[0], E[1]);
    pen.Q(cx - n[0] * thick, cy - n[1] * thick, S[0], S[1]);
    return pen.Z();
}

const rad = (deg) => (deg * Math.PI) / 180;

// A lens that hugs an ellipse (centre cx,cy, radii rx,ry) between two angles (degrees, screen coordinates, y down).
function arcLens(cx, cy, rx, ry, a1, a2, rho, thick) {
    const at = (deg) => [cx + rho * rx * Math.cos(rad(deg)), cy + rho * ry * Math.sin(rad(deg))];
    const mid = (a1 + a2) / 2;
    const nx = Math.cos(rad(mid)) * ry;
    const ny = Math.sin(rad(mid)) * rx;
    const nl = Math.hypot(nx, ny);
    return lens(at(a1), at(mid), at(a2), [nx / nl, ny / nl], thick);
}

// A rotated ellipse as path data (a highlight blob).
function ellipsePath(cx, cy, rx, ry, deg) {
    const c = Math.cos(rad(deg)), s = Math.sin(rad(deg));
    const P = (x, y) => [cx + x * c - y * s, cy + x * s + y * c];
    const pen = new Pen(...P(rx, 0));
    const seg = (x1, y1, x2, y2, x, y) => pen.C(...P(x1, y1), ...P(x2, y2), ...P(x, y));
    seg(rx, K * ry, K * rx, ry, 0, ry);
    seg(-K * rx, ry, -rx, K * ry, -rx, 0);
    seg(-rx, -K * ry, -K * rx, -ry, 0, -ry);
    seg(K * rx, -ry, rx, -K * ry, rx, 0);
    return pen.Z();
}

const op = (v) => String(v).replace(/^0./, '.'); // 0.15 -> .15, saves a byte per use
const stops = (list) => list.map(([o, c, op]) => `<stop offset='${o}' stop-color='${c}'${op === undefined ? '' : ` stop-opacity='${op}'`}/>`).join('');

// ---------------------------------------------------------------------------------------------------------
// Bar tile
// ---------------------------------------------------------------------------------------------------------
function buildBar() {
    UNIT = 20;
    const lip = wave(BAR.lip, BAR.w);
    // One closed shape; its top and sides lie outside the tile, so only the lower edge is ever stroked or shadowed.
    const pen = new Pen(-4, -3);
    pen.V(lip.f(-4));
    let x = -4;
    const lipTo = (xe) => {
        const n = Math.max(1, Math.ceil((xe - x) / BAR.lip.step));
        for (let i = 1; i <= n; i++) {
            const x0 = x + ((xe - x) * (i - 1)) / n, x1 = x + ((xe - x) * i) / n, d = (x1 - x0) / 3;
            pen.C(x0 + d, lip.f(x0) + lip.df(x0) * d, x1 - d, lip.f(x1) - lip.df(x1) * d, x1, lip.f(x1));
        }
        x = xe;
    };
    const bulbs = [];
    for (const d of BAR.drips) {
        const rl = d.r * (1 - d.skew), rr = d.r * (1 + d.skew);
        const xb = d.x + d.lean;
        const yb = lip.f(d.x);
        const len = d.tip - yb; // depth below the lip
        const ry = Math.min(d.r * d.ry, len * 0.76);
        const yc = yb + len - ry; // centre of the bulb
        const hasNeck = d.neck < 0.98 && yc - yb >= 0.9;
        const nl = hasNeck ? d.r * d.neck * (1 - d.skew * 1.2) : rl, nr = hasNeck ? d.r * d.neck * (1 + d.skew * 1.2) : rr;
        const yn = hasNeck ? yb + (yc - yb) * d.neckAt : yc; // where the neck is narrowest
        const xL = xb - BAR.shoulder * d.r * (1 - d.skew), xR = xb + BAR.shoulder * d.r * (1 + d.skew);
        lipTo(xL);
        const yL = lip.f(xL), yR = lip.f(xR);
        pen.C(xL + (xb - nl - xL) * 0.6, yL, xb - nl, yn - (yn - yL) * 0.55, xb - nl, yn); // wide shoulder out of the lip
        if (hasNeck) pen.C(xb - nl, yn + (yc - yn) * 0.5, xb - rl, yc - (yc - yn) * 0.5, xb - rl, yc); // short neck widens
        pen.C(xb - rl, yc + K * ry, xb - K * rl, yc + ry, xb, yc + ry); // round bulb
        pen.C(xb + K * rr, yc + ry, xb + rr, yc + K * ry, xb + rr, yc);
        if (hasNeck) pen.C(xb + rr, yc - (yc - yn) * 0.5, xb + nr, yn + (yc - yn) * 0.5, xb + nr, yn);
        pen.C(xb + nr, yn - (yn - yR) * 0.55, xR - (xR - xb - nr) * 0.6, yR, xR, yR); // shoulder back into the lip
        x = xR;
        bulbs.push({ xb, yc, rx: (rl + rr) / 2, ry, tip: yc + ry, w: rl + rr, r: d.r });
    }
    lipTo(BAR.w + 4);
    pen.V(-3);
    const body = pen.Z();

    // Volume: a crisp highlight line along the lip; on every bulb a specular blob on the lit (upper left) flank with a
    // thin streak below it, a pale crescent where the light that crossed the gel leaves on the far side, and a darker
    // core towards the tip.
    const white = [], glow = [], cores = [];
    for (const [x0, x1] of BAR.lipGlints) {
        const xm = (x0 + x1) / 2, y = (t) => lip.f(t) - BAR.lipGlintAbove;
        white.push(lens([x0, y(x0)], [xm, y(xm)], [x1, y(x1)], [0, -1], 0.7));
    }
    for (const b of bulbs) {
        const px = (deg, rho) => [b.xb + rho * b.rx * Math.cos(rad(deg)), b.yc + rho * b.ry * Math.sin(rad(deg))];
        const [hx, hy] = px(228, 0.56);
        white.push(ellipsePath(hx, hy, 0.34 * b.r, 0.17 * b.r, 228 + 90));
        white.push(arcLens(b.xb, b.yc, b.rx, b.ry, 196, 160, 0.8, 0.5));
        glow.push(arcLens(b.xb, b.yc, b.rx, b.ry, 14, 78, 0.8, 0.17 * b.r));
        cores.push(`<ellipse cx='${u(b.xb + 0.05 * b.rx)}' cy='${u(b.yc + 0.34 * b.ry)}' rx='${u(0.62 * b.rx)}' ry='${u(0.55 * b.ry)}' fill='url(#k)'/>`);
    }

    const svg =
        `<svg xmlns='http://www.w3.org/2000/svg' width='${BAR.w}' height='${BAR.h}' viewBox='0 0 ${BAR.w * UNIT} ${BAR.h * UNIT}'>` +
        `<defs>` +
        `<linearGradient id='g' gradientUnits='userSpaceOnUse' x2='0' y2='${u(11.4)}'>${stops([[0, GEL.pale], ['.28', '#B7E660'], ['.52', GEL.mid], ['.8', GEL.deep], [1, '#6A9F22']])}</linearGradient>` +
        `<radialGradient id='k'>${stops([[0, GEL.core, '.5'], [1, GEL.core, 0]])}</radialGradient>` +
        `<filter id='s' x='-.02' y='-.3' width='1.04' height='1.8'><feGaussianBlur stdDeviation='${u(0.55)}'/></filter>` +
        `<path id='p' d='${body}'/>` +
        `</defs>` +
        `<use href='#p' y='${u(1)}' fill='#000' fill-opacity='.28' filter='url(#s)'/>` +
        `<use href='#p' fill='url(#g)' fill-opacity='${GEL.opacity}'/>` +
        cores.join('') +
        `<use href='#p' fill='none' stroke='${GEL.rim}' stroke-opacity='.35' stroke-width='${u(1.7)}'/>` +
        `<use href='#p' fill='none' stroke='${GEL.edge}' stroke-opacity='.55' stroke-width='${u(0.7)}'/>` +
        `<path fill='${GEL.glow}' fill-opacity='.45' d='${glow.join('')}'/>` +
        `<path fill='${GEL.white}' fill-opacity='.88' d='${white.join('')}'/>` +
        `</svg>`;
    return { svg, bulbs };
}

// ---------------------------------------------------------------------------------------------------------
// Side tile
// ---------------------------------------------------------------------------------------------------------
// A teardrop: the tail starts at (tipX, top), the bulb (half width R) ends at `bottom`.
function teardrop(cx, tipX, top, bottom, R) {
    const ry = R * 1.1;
    const yc = bottom - ry;
    const lt = yc - top;
    const pen = new Pen(tipX, top);
    pen.C(tipX - R * 0.1, top + lt * 0.42, cx - R, yc - lt * 0.5, cx - R, yc);
    pen.C(cx - R, yc + K * ry, cx - K * R, yc + ry, cx, yc + ry);
    pen.C(cx + K * R, yc + ry, cx + R, yc + K * ry, cx + R, yc);
    pen.C(cx + R, yc - lt * 0.5, tipX + R * 0.1, top + lt * 0.42, tipX, top);
    return { d: pen.Z(), cx, yc, rx: R, ry };
}

function buildSide() {
    UNIT = 20;
    const rail = wave(SIDE.rail, SIDE.h);
    // The rail's left edge runs down the tile; the right side lies outside the tile, so only the left edge is outlined.
    const pen = new Pen(rail.f(-4), -4);
    const steps = Math.ceil((SIDE.h + 8) / SIDE.rail.step);
    for (let i = 1; i <= steps; i++) {
        const y0 = -4 + ((SIDE.h + 8) * (i - 1)) / steps, y1 = -4 + ((SIDE.h + 8) * i) / steps, d = (y1 - y0) / 3;
        pen.C(rail.f(y0) + rail.df(y0) * d, y0 + d, rail.f(y1) - rail.df(y1) * d, y1 - d, rail.f(y1), y1);
    }
    pen.L(SIDE.w + 4, SIDE.h + 4);
    pen.L(SIDE.w + 4, -4);
    const railPath = pen.Z();

    const blobs = [], white = [], glow = [];
    for (const b of SIDE.beads) {
        const R = (b.bulge + 3.3) / 2; // the bead's right edge touches the tile edge, its left edge reaches `bulge` past the rail
        const t = teardrop(12.3 - R, SIDE.tipX, b.top, b.top + b.len, R);
        blobs.push(t.d);
        white.push(arcLens(t.cx, t.yc, t.rx, t.ry, 246, 168, 0.68, 0.8));
        glow.push(arcLens(t.cx, t.yc, t.rx, t.ry, 8, 80, 0.74, 0.55));
    }
    for (const dr of SIDE.drops) {
        const t = teardrop(dr.x, dr.x + 0.15, dr.top, dr.top + dr.len, dr.r);
        blobs.push(t.d);
        white.push(arcLens(t.cx, t.yc, t.rx, t.ry, 244, 170, 0.62, 0.45));
    }
    const railGlints = []; // highlight line on the lit flank of the rail
    for (const [y0, y1] of SIDE.railGlints) {
        const x = (t) => rail.f(t) + 0.95, ym = (y0 + y1) / 2;
        railGlints.push(lens([x(y0), y0], [x(ym), ym], [x(y1), y1], [-1, 0], 0.62));
    }

    return (
        `<svg xmlns='http://www.w3.org/2000/svg' width='${SIDE.w}' height='${SIDE.h}' viewBox='0 0 ${SIDE.w * UNIT} ${SIDE.h * UNIT}'>` +
        `<defs>` +
        `<linearGradient id='r' gradientUnits='userSpaceOnUse' x1='${u(9)}' x2='${u(12)}'>${stops([[0, '#C2ED76'], ['.3', GEL.pale], ['.62', GEL.mid], [1, GEL.deep]])}</linearGradient>` +
        `<linearGradient id='v' x2='0' y2='1'>${stops([[0, GEL.pale], ['.4', '#B7E660'], ['.7', GEL.mid], [1, GEL.deep]])}</linearGradient>` +
        `<linearGradient id='o' x2='0' y2='1'>${stops([['.08', GEL.edge, 0], ['.4', GEL.edge, '.6']])}</linearGradient>` +
        `<path id='p' d='${railPath}'/>` +
        `<g id='b'>${blobs.map((d) => `<path d='${d}'/>`).join('')}</g>` +
        `</defs>` +
        `<use href='#p' x='${u(-0.7)}' y='${u(0.5)}' fill='#000' fill-opacity='.3'/>` +
        `<use href='#p' fill='url(#r)' fill-opacity='.95'/>` +
        `<use href='#p' fill='none' stroke='${GEL.edge}' stroke-opacity='.55' stroke-width='${u(0.7)}'/>` +
        `<path fill='${GEL.white}' fill-opacity='.7' d='${railGlints.join('')}'/>` +
        `<use href='#b' x='${u(-0.7)}' y='${u(0.6)}' fill='#000' fill-opacity='.3'/>` +
        `<use href='#b' fill='url(#v)' fill-opacity='.93'/>` +
        `<use href='#b' fill='none' stroke='url(#o)' stroke-width='${u(0.7)}'/>` +
        `<path fill='${GEL.glow}' fill-opacity='.4' d='${glow.join('')}'/>` +
        `<path fill='${GEL.white}' fill-opacity='.85' d='${white.join('')}'/>` +
        `</svg>`
    );
}

// ---------------------------------------------------------------------------------------------------------
// Canvas tile: soft gel bubbles (a faint fill that is brightest at the rim, a very thin rim, one crisp specular highlight)
// ---------------------------------------------------------------------------------------------------------
function buildCanvas() {
    UNIT = CANVAS.unit;
    const spec = [], sheen = [];
    const circles = CANVAS.bubbles.map(([cx, cy, R]) => {
        const at = (deg, rho) => [cx + rho * R * Math.cos(rad(deg)), cy + rho * R * Math.sin(rad(deg))];
        const [hx, hy] = at(226, 0.6);
        spec.push(ellipsePath(hx, hy, Math.max(1.1, 0.2 * R), Math.max(0.6, 0.095 * R), 226 + 90));
        if (R >= 14) sheen.push(arcLens(cx, cy, R, R, 28, 74, 0.8, 0.07 * R));
        return `<circle cx='${u(cx)}' cy='${u(cy)}' r='${u(R)}'/>`;
    });
    const rim = CANVAS.rim;
    return (
        `<svg xmlns='http://www.w3.org/2000/svg' width='${CANVAS.w}' height='${CANVAS.h}' viewBox='0 0 ${CANVAS.w * UNIT} ${CANVAS.h * UNIT}'>` +
        `<defs><radialGradient id='b'>${stops([[0, '#C8F075', '.02'], ['.6', '#C8F075', '.035'], ['.86', '#D6F68C', op((rim * 0.55).toFixed(2))], ['.95', '#E2FBA8', op(rim)], [1, '#E2FBA8', 0]])}</radialGradient></defs>` +
        `<g fill='url(#b)'>${circles.join('')}</g>` +
        `<path fill='#E2FBA8' fill-opacity='.1' d='${sheen.join('')}'/>` +
        `<path fill='#fff' fill-opacity='${op(CANVAS.spec)}' d='${spec.join('')}'/>` +
        `</svg>`
    );
}

// ---------------------------------------------------------------------------------------------------------
// CSS values
// ---------------------------------------------------------------------------------------------------------
const encode = (svg) => svg.replace(/</g, '%3C').replace(/>/g, '%3E').replace(/#/g, '%23');
const uri = (svg) => 'data:image/svg+xml,' + encode(svg);

const bar = buildBar();
const sideSvg = buildSide();
const canvasSvg = buildCanvas();
const barUri = uri(bar.svg);
const sideUri = uri(sideSvg);
const canvasUri = uri(canvasSvg);

// Milky lime in-scatter that the gel throws onto the surface next to it: a soft haze under the lip, a glow left of the rail.
const HAZE = (a) => `rgba(196, 240, 112, ${a})`;
// The bar tile starts at 37% of the free width (plus 9 em), not at 0: bars of different widths and font sizes in one app get
// different offsets, so their drops do not line up in columns.
const BAR_POS = 'calc(37% + 9em) 0';
const barValue =
    `url("${barUri}") ${BAR_POS} / ${BAR.w}px ${BAR.h}px repeat-x, ` +
    `linear-gradient(180deg, ${HAZE(0.2)} 3px, ${HAZE(0)} 11px) 0 0 / 100% 11px no-repeat, ` +
    `linear-gradient(180deg, rgba(240, 255, 190, 0.12) 0, rgba(240, 255, 190, 0.03) 48%, rgba(0, 0, 0, 0) 49%), ` +
    `linear-gradient(180deg, #324820 0, #2D4219 55%, #24361A 100%), var(--vd-mat-bar-color)`;
const sideValue =
    `url("${sideUri}") 100% 0 / ${SIDE.w}px ${SIDE.h}px repeat-y, ` +
    `linear-gradient(270deg, ${HAZE(0)} 3px, ${HAZE(0.16)} 4px, ${HAZE(0)} 9px) 0 0 / 100% 100% no-repeat, ` +
    `linear-gradient(90deg, #121A0B 0, #17220E 100%), var(--vd-mat-side-color)`;
const canvasValue =
    `url("${canvasUri}") 0 0 / ${CANVAS.w}px ${CANVAS.h}px, ` +
    `radial-gradient(120% 60% at 50% 0, rgba(165, 217, 65, 0.08), transparent 70%), var(--vd-mat-canvas-color)`;

console.log('--vd-mat-canvas: ' + canvasValue + ';');
console.log('--vd-mat-bar: ' + barValue + ';');
console.log('--vd-mat-side: ' + sideValue + ';');
console.log('bar drops (centre x / bulb width / tip y, px): ' + bar.bulbs.map((b) => `${b.xb.toFixed(0)}/${b.w.toFixed(1)}/${b.tip.toFixed(1)}`).join('  '));
console.log(`canvas data URI ${canvasUri.length} bytes, bar data URI ${barUri.length} bytes, side data URI ${sideUri.length} bytes`);

if (outDir) {
    mkdirSync(outDir, { recursive: true });
    const css = readFileSync(CSS_FILE, 'utf8');
    const override = flag('keep') ? '' : `\n.desktop-body[data-theme="green_slime"] { --vd-mat-canvas: ${canvasValue}; --vd-mat-bar: ${barValue}; --vd-mat-side: ${sideValue}; }`;
    writeFileSync(join(outDir, 'preview.html'), previewHtml(css + override, barUri, sideUri, canvasUri));
    writeFileSync(join(outDir, 'bar.svg'), bar.svg);
    writeFileSync(join(outDir, 'side.svg'), sideSvg);
    writeFileSync(join(outDir, 'canvas.svg'), canvasSvg);
    console.log('preview: ' + join(outDir, 'preview.html') + '  (?view=stack|status|side|canvas|tiles|zoom&z=<scale>&x=<px>&y=<px>[&side=1])');
}

// ---------------------------------------------------------------------------------------------------------
// Preview page: a Code-Studio-like window, status bars, a sidebar next to a well, a canvas with text, the raw tiles
// ---------------------------------------------------------------------------------------------------------
function previewHtml(css, barUri, sideUri, canvasUri) {
    return `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Green Slime gel preview</title>
<style>
${css}
html, body { margin: 0; }
body { background: #0b1008; color: #F1F7DC; font: 12px 'Trebuchet MS', 'Segoe UI', system-ui, sans-serif; padding: 12px; }
.view { display: none; }
body[data-view="stack"] #stack, body[data-view="tiles"] #tiles, body[data-view="all"] #stack, body[data-view="all"] #tiles { display: block; }
body[data-view="side"] #side, body[data-view="all"] #side { display: flex; }
body[data-view="zoom"] #zoom, body[data-view="status"] #statusdemo, body[data-view="canvas"] #canvasdemo { display: block; }
.win { width: 860px; border: 1px solid #5E7A33; border-radius: 12px; overflow: hidden; background: var(--vd-mat-canvas); margin-bottom: 16px; }
.bar { background: var(--vd-mat-bar); box-shadow: var(--vd-mat-bar-edge); border-bottom: 1px solid var(--vd-mat-divider); display: flex; align-items: center; gap: 8px; box-sizing: border-box; }
.menu { padding: 6px 10px; min-height: 30px; gap: 16px; color: #C6D4A6; font-size: 13px; }
.toolbar { padding: 4px 10px; min-height: 42px; gap: 4px; }
.tabs { padding: 4px 8px 0; min-height: 34px; gap: 2px; align-items: flex-end; }
.sidehead { padding: 6px 10px; min-height: 34px; font-size: 12px; color: var(--vd-mat-label-color); font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
.termbar { padding: 6px 10px; min-height: 34px; font-size: 12px; color: #C6D4A6; border-top: 1px solid var(--vd-mat-divider); }
.status { padding: 0 10px; min-height: 26px; font-size: 11px; justify-content: space-between; color: #C6D4A6; border-bottom: 0; border-top: 1px solid var(--vd-mat-divider); }
.btn { display: inline-flex; align-items: center; justify-content: center; height: 28px; padding: 0 14px; color: var(--vd-mat-btn-text); background: var(--vd-mat-btn); border: 1px solid var(--vd-mat-btn-border); border-radius: var(--vd-mat-btn-radius); box-shadow: var(--vd-mat-btn-shadow); font: inherit; }
.btn.icon { width: 28px; padding: 0; }
.btn.primary { color: var(--vd-mat-primary-text); background: var(--vd-mat-primary); border-color: var(--vd-mat-primary-border); box-shadow: var(--vd-mat-primary-shadow); }
.sep { width: 1px; height: 20px; background: var(--vd-mat-divider); margin: 0 4px; }
.title { flex: 1; text-align: center; color: var(--vd-mat-heading-color); font-family: var(--vd-mat-heading-font); font-weight: 600; }
.tab { padding: 6px 14px; border-radius: 9px 9px 0 0; color: #C6D4A6; background: transparent; }
.tab.on { background: var(--vd-mat-selected); color: var(--vd-mat-selected-text); box-shadow: var(--vd-mat-selected-edge); }
.tab.solid { background: #2D4219; border: 1px solid #4E6A2A; border-bottom: 0; }
.well { background: var(--vd-mat-well); box-shadow: var(--vd-mat-well-shadow); font: 12px/1.6 var(--vd-mat-mono-font); padding: 10px 14px; color: #DDE8C0; box-sizing: border-box; }
.well b { color: #C9F27A; font-weight: 400; } .well i { color: #4FE0C0; font-style: normal; } .well u { color: #8FA070; text-decoration: none; }
.cols { display: flex; }
.cols .left { width: 260px; flex: none; display: flex; flex-direction: column; }
.cols .right { flex: 1; min-width: 0; }
.ed { height: 120px; } .term { height: 56px; }
.sidebar { flex: 1; background: var(--vd-mat-side); box-shadow: var(--vd-mat-side-edge); padding: 6px 18px 6px 10px; box-sizing: border-box; border-right: 1px solid var(--vd-mat-divider); }
.sidebar .lab { font: 700 11px var(--vd-mat-label-font); letter-spacing: .08em; text-transform: uppercase; color: var(--vd-mat-label-color); margin: 10px 4px 6px; }
.row { padding: 6px 8px; border-radius: 8px; color: #E4EDC9; }
.row.sel { background: var(--vd-mat-selected); color: var(--vd-mat-selected-text); box-shadow: var(--vd-mat-selected-edge); }
#side { gap: 0; width: 640px; height: 400px; border: 1px solid #5E7A33; border-radius: 12px; overflow: hidden; background: var(--vd-mat-canvas); margin-bottom: 16px; }
#side .sidebar { width: 220px; flex: none; padding: 10px 18px 10px 10px; }
#side .well { flex: 1; height: 100%; }
#canvasdemo { width: 760px; height: 360px; border: 1px solid #5E7A33; border-radius: 12px; background: var(--vd-mat-canvas); padding: 22px 28px; box-sizing: border-box; }
#canvasdemo h2 { margin: 0 0 4px; font: 700 20px var(--vd-mat-heading-font); color: var(--vd-mat-heading-color); }
#canvasdemo p { margin: 0 0 12px; max-width: 520px; line-height: 1.5; color: #C6D4A6; font-size: 13px; }
#canvasdemo .t { color: #F1F7DC; font-size: 14px; margin-bottom: 8px; }
#canvasdemo .card { display: inline-block; vertical-align: top; width: 200px; margin: 6px 10px 6px 0; padding: 12px 14px; background: var(--vd-mat-card); border: 1px solid var(--vd-mat-card-border); border-radius: var(--vd-mat-card-radius); box-shadow: var(--vd-mat-card-shadow); color: #E4EDC9; }
#tiles { width: 1300px; }
#tiles h4 { margin: 10px 0 4px; font-weight: 400; color: #C6D4A6; }
.raw { background: var(--vd-mat-bar-color); padding: 0; }
.raw img { display: block; }
.rawside { display: flex; gap: 14px; background: #121A0B; padding: 0 20px; }
.rawcanvas { background: var(--vd-mat-canvas-color); }
</style></head>
<body class="desktop-body" data-theme="green_slime" data-view="all">
<div class="view win" id="stack">
  <div class="bar menu"><span>File</span><span>Edit</span><span>Selection</span><span>View</span><span>Go</span><span>Run</span></div>
  <div class="bar toolbar"><span class="btn icon">&#9776;</span><span class="btn icon">&#9664;</span><span class="btn icon">&#9654;</span><span class="sep"></span><span class="btn">Save</span><span class="btn">Format</span><span class="btn primary">Run</span><span class="title">main.go - <b>aurago</b></span><span class="btn">Settings</span></div>
  <div class="cols">
    <div class="left">
      <div class="bar sidehead"><span>Explorer</span></div>
      <div class="sidebar"><div class="row sel">main.go</div><div class="row">slime.css</div><div class="row">README.md</div><div class="row">go.mod</div><div class="lab">Outline</div><div class="row">func drip</div><div class="row">type Gel</div></div>
    </div>
    <div class="right">
      <div class="bar tabs"><span class="tab on">main.go</span><span class="tab solid">slime.css</span><span class="tab">README.md</span><span class="tab solid">go.mod</span></div>
      <div class="well ed"><u>01</u> <b>package</b> main<br><u>02</u> <br><u>03</u> <b>func</b> <i>drip</i>(w *Window) *Gel {<br><u>04</u> &nbsp;&nbsp;&nbsp;&nbsp;<b>return</b> &amp;Gel{Tint: <i>"#A5D941"</i>}<br><u>05</u> }</div>
      <div class="bar termbar"><span>TERMINAL</span><span class="btn">bash</span><span class="btn icon">+</span></div>
      <div class="well term"><u>$</u> go test ./ui -run DesktopAppMaterials<br>ok &nbsp;aurago/ui &nbsp;0.84s</div>
    </div>
  </div>
  <div class="bar status"><span>main &#10003; &nbsp; 0 errors, 0 warnings</span><span>Ln 4, Col 12 &nbsp; UTF-8 &nbsp; Go</span></div>
</div>
<div class="view win" id="statusdemo" style="width:640px">
  <div class="bar status" style="min-height:22px;justify-content:flex-start;gap:18px;border-top:0"><span>3 items</span><span>12.4 MB free</span><span>Listing: details</span><span>Sorted by name, ascending</span><span>Ready</span></div>
  <div class="well" style="height:20px;padding:0"></div>
  <div class="bar status" style="min-height:26px;gap:12px;border-top:0"><span>main &#10003; &nbsp;0 errors, 0 warnings, 0 infos</span><span>Ln 4, Col 12 &nbsp; Spaces: 4 &nbsp; UTF-8 &nbsp; LF &nbsp; Go</span></div>
  <div class="well" style="height:20px;padding:0"></div>
  <div class="bar menu" style="min-height:28px;border-top:0"><span>File</span><span>Edit</span><span>View</span><span>Window</span><span>Help</span><span style="flex:1"></span><span>Documents - 14 elements</span></div>
</div>
<div class="view" id="side">
  <div class="sidebar"><div class="lab">Explorer</div><div class="row sel">main.go</div><div class="row">slime.css</div><div class="row">README.md</div><div class="row">go.mod</div><div class="lab">Outline</div><div class="row">func drip</div><div class="row">type Gel</div><div class="row">const lip</div><div class="row">var beads</div><div class="row">func slide</div><div class="row">func settle</div><div class="row">func burst</div></div>
  <div class="well"><u>01</u> <b>package</b> main<br><u>02</u><br><u>03</u> <b>func</b> <i>slide</i>() {}</div>
</div>
<div class="view" id="canvasdemo">
  <h2>Darstellung</h2>
  <p>Hintergrund, Akzentfarbe und visuelle Dichte. Muted text sits right on the canvas, so the bubbles must stay faint under it.</p>
  <div class="t">Plain text on the canvas in the main text colour, 14 px.</div>
  <p>A second paragraph in the muted colour (#C6D4A6) crossing the brightest bubbles of the tile, so their rims and highlights can be judged against real text at 1x and 2x.</p>
  <div class="card">Card on the canvas with a title and text</div><div class="card">Another card, blobby and squishy</div><div class="card">A third card to cover more bubbles</div>
</div>
<div class="view" id="tiles">
  <h4>bar tile, 3x on the bar colour</h4>
  <div class="raw"><img alt="" src="${barUri}" width="1260" height="36"></div>
  <h4>bar tile, 1x repeated</h4>
  <div class="bar" style="height:40px;border:0;box-shadow:none"></div>
  <h4>canvas tile, 1x, and side tile, 5x, twice</h4>
  <div style="display:flex;gap:20px"><div class="rawcanvas"><img alt="" src="${canvasUri}" width="520" height="380"></div><div class="rawside"><img alt="" src="${sideUri}" width="60" height="1200"><img alt="" src="${sideUri}" width="60" height="1200" style="margin-left:60px"></div></div>
</div>
<div class="view" id="zoom"><div class="raw" style="overflow:hidden"><img id="zimg" alt="" src="${barUri}"></div><img id="simg" alt="" src="${sideUri}" hidden><img id="cimg" alt="" src="${canvasUri}" hidden></div>
<script>
  const q = new URLSearchParams(location.search);
  const v = q.get('view');
  if (v) document.body.dataset.view = v;
  if (v === 'zoom') {
    const z = +q.get('z') || 10, x = +q.get('x') || 0, side = q.get('side'), cv = q.get('canvas');
    const im = document.getElementById('zimg');
    if (side) im.src = document.getElementById('simg').src;
    if (cv) im.src = document.getElementById('cimg').src;
    im.style.width = (cv ? 520 : side ? 12 : 420) * z + 'px';
    im.style.marginLeft = -x * z + 'px';
    im.style.marginTop = -(+q.get('y') || 0) * z + 'px';
    document.body.style.padding = '0';
    document.body.style.background = side ? '#121A0B' : cv ? getComputedStyle(document.body).getPropertyValue('--vd-mat-canvas-color') : getComputedStyle(document.body).getPropertyValue('--vd-mat-bar-color');
  }
</script>
</body></html>
`;
}
