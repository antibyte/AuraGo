// Generates ui/img/wallpapers/themes/forest.svg - a misty conifer forest at golden dusk.
// Usage: node assets/desktop-wallpaper-forest/generate.mjs
// Deterministic (fixed seed), no dependencies, no external references.
import { writeFileSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const out = resolve(here, '../../ui/img/wallpapers/themes/forest.svg');

function mulberry32(a) {
    return function () {
        a |= 0; a = (a + 0x6D2B79F5) | 0;
        let t = Math.imul(a ^ (a >>> 15), 1 | a);
        t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
        return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
}
const rnd = mulberry32(0x4f0e57);
const rr = (a, b) => a + (b - a) * rnd();
// Distant layers are written with whole-pixel coordinates (sub-pixel detail is invisible there) to keep the file small.
let prec = 1;
const n1 = (v) => String(+v.toFixed(prec));
const W = 1920, H = 1080;
const GLOW = { x: 1240, y: 600 };

// One conifer, no trunk unless opts.trunk. 'spruce' = narrow spire with drooping tiers, 'fir' = rounder and fuller.
// Tiers are asymmetric (each side has its own length), the outline is ragged and the whole tree leans a little.
function tree(x, base, h, opts = {}) {
    const fir = !!opts.fir;
    const tiers = fir ? 5 + Math.floor(rnd() * 4) : Math.max(5, Math.min(13, 5 + Math.floor(h / 40) + Math.floor(rnd() * 3)));
    const broken = rnd() < 0.12;
    const tipY = base - h + (broken ? h * rr(0.04, 0.09) : 0);
    const body = base - tipY;
    const wMax = h * (fir ? rr(0.24, 0.36) : rr(0.14, 0.24));
    const lean = rr(-0.052, 0.052);
    const cxAt = (y) => x + lean * (y - base);
    const pw = fir ? 0.6 : 0.85;
    const side = (s) => {
        const pts = [];
        let prevW = 0, prevY = tipY, prevTipX = 0, prevTipY = tipY;
        for (let i = 0; i < tiers; i++) {
            const t = (i + 1) / tiers;
            const step = body / tiers;
            const y = tipY + body * Math.pow(t, 1.05) * 0.97 + rr(-0.15, 0.15) * step;
            const w = wMax * Math.pow(t, pw) * rr(0.75, 1.25) * (i === 0 ? 0.6 : 1);
            const droop = step * rr(0.1, 0.5);
            const nx = prevW * rr(0.22, 0.5);
            const ny = prevY - step * rr(0.05, 0.3);
            if (i > 0 && h > 260) pts.push([s * ((prevTipX + nx) / 2 + rr(-1, 1) * step * 0.05), (prevTipY + ny) / 2 + step * rr(0.02, 0.1)]);
            pts.push([s * nx, ny]);
            // Ragged branch edge: two small jags between notch and tip.
            for (const f of (h > 260 ? [0.38, 0.72] : [0.55])) {
                pts.push([s * (nx + (w - nx) * f + rr(-0.06, 0.04) * w), ny + (y + droop - ny) * f + rr(-0.12, 0.08) * step]);
            }
            pts.push([s * w, y + droop]);
            prevW = w; prevY = y; prevTipX = w; prevTipY = y + droop;
        }
        return pts;
    };
    const r = side(1), l = side(-1);
    const abs = (p) => [cxAt(p[1]) + p[0], p[1]];
    const bottom = opts.trunk ? base : base + 30;
    const stub = broken ? h * 0.006 : 0.4;
    let d = `M${n1(cxAt(tipY) - stub)} ${n1(tipY)}L${n1(cxAt(tipY) + stub)} ${n1(tipY)}`;
    for (const p of r) { const a = abs(p); d += `L${n1(a[0])} ${n1(a[1])}`; }
    const lastY = r[r.length - 1][1];
    if (opts.trunk) {
        const tw = h * 0.016 + 3;
        const ty = lastY - h * 0.01;
        d += `L${n1(cxAt(ty) + tw * 0.7)} ${n1(ty)}L${n1(cxAt(bottom) + tw * 1.2)} ${n1(bottom)}L${n1(cxAt(bottom) - tw * 1.2)} ${n1(bottom)}L${n1(cxAt(ty) - tw * 0.7)} ${n1(ty)}`;
    } else {
        d += `L${n1(cxAt(bottom) + 6)} ${n1(bottom)}L${n1(cxAt(bottom) - 6)} ${n1(bottom)}`;
    }
    for (let i = l.length - 1; i >= 0; i--) { const a = abs(l[i]); d += `L${n1(a[0])} ${n1(a[1])}`; }
    return d + 'Z';
}

// Rolling ground under a ridge, running down to the bottom edge; the next nearer ridge covers it.
function ground(base, amp) {
    let d = `M-20 ${H + 10}`;
    for (let x = -20; x <= W + 20; x += 40) d += `L${n1(x)} ${n1(base + 26 + amp * Math.sin(x / 170 + base) + rr(-3, 3))}`;
    return d + `L${W + 20} ${H + 10}Z`;
}

function ridge(r) {
    prec = r.base < 800 ? 0 : 1;
    let d = ground(r.base, r.amp);
    for (let i = 0; i < r.n; i++) {
        const x = r.xs ? r.xs() : rr(-40, W + 40);
        d += tree(x, r.base + rr(-r.jit, r.jit), rr(r.hMin, r.hMax), { fir: rnd() < r.firP, trunk: r.trunk });
    }
    prec = 1;
    return d;
}

// Very distant hills with a tiny treeline, barely darker than the haze.
function farHills() {
    prec = 0;
    const hill = (x) => 592 + 16 * Math.sin(x / 260 + 1) + 9 * Math.sin(x / 97);
    let d = `M-20 ${H}`;
    for (let x = -20; x <= W + 20; x += 20) d += `L${n1(x)} ${n1(hill(x))}`;
    d += `L${W + 20} ${H}Z`;
    for (let i = 0; i < 90; i++) {
        const x = rr(-20, W + 20);
        d += tree(x, hill(x) + 4, rr(18, 46), { fir: rnd() < 0.4 });
    }
    prec = 1;
    return d;
}

const ridges = [
    { base: 610, n: 72, hMin: 60, hMax: 125, jit: 6, amp: 8, firP: 0.35 },
    { base: 690, n: 48, hMin: 110, hMax: 230, jit: 8, amp: 10, firP: 0.25 },
    { base: 780, n: 38, hMin: 180, hMax: 340, jit: 10, amp: 10, firP: 0.12 },
    { base: 880, n: 46, hMin: 250, hMax: 470, jit: 12, amp: 12, firP: 0.05 },
];
// Front ridge: few, very tall trees framing the scene at the far left and far right.
const front = { base: 1010, n: 8, hMin: 520, hMax: 880, jit: 10, amp: 4, firP: 0, trunk: true, xs: () => (rnd() < 0.5 ? rr(-80, 230) : rr(1600, 2000)) };

const defs = [];
const body = [];
const tops = [];
const colors = [['#4b5b47', '#7d7a55', 0.55], ['#2e4438', '#58634a', 0.4], ['#1c2f27', '#36483a', 0.25], ['#0f1c17', '#0f1c17', 0.12]];
ridges.forEach((r, i) => {
    const d = ridge(r);
    const hMax = r.hMax;
    r.top = r.base - hMax * 0.4;
    defs.push(`<path id="t${i + 1}" d="${d}"/>`);
    defs.push(`<linearGradient id="r${i + 1}" gradientUnits="userSpaceOnUse" x1="0" y1="${n1(r.base - hMax)}" x2="0" y2="${n1(r.base + 70)}"><stop offset="0" stop-color="${colors[i][0]}"/><stop offset="${n1((hMax + 10) / (hMax + 70))}" stop-color="${colors[i][1]}"/><stop offset="1" stop-color="${colors[i][0]}"/></linearGradient>`);
    // Warm light is radial around the sun (squashed vertically) so ground below the tree line is not lit.
    defs.push(`<radialGradient id="w${i + 1}" gradientUnits="userSpaceOnUse" cx="${GLOW.x}" cy="${GLOW.y + 10}" r="760" gradientTransform="translate(${GLOW.x} ${GLOW.y + 10}) scale(1 0.42) translate(-${GLOW.x} -${GLOW.y + 10})"><stop offset="0" stop-color="#e8aa58" stop-opacity="${colors[i][2]}"/><stop offset="0.6" stop-color="#e0a050" stop-opacity="${n1(colors[i][2] * 0.45)}"/><stop offset="1" stop-color="#e0a050" stop-opacity="0"/></radialGradient>`);
    defs.push(`<linearGradient id="f${i + 1}" gradientUnits="userSpaceOnUse" x1="0" y1="${n1(r.top)}" x2="0" y2="${n1(r.base + 60)}"><stop offset="0" stop-color="#b8cbb8" stop-opacity="0"/><stop offset="${n1((r.base + 10 - r.top) / (r.base + 60 - r.top))}" stop-color="#b8cbb8" stop-opacity="${i > 2 ? 0.1 : i > 1 ? 0.22 : 0.35}"/><stop offset="1" stop-color="#b8cbb8" stop-opacity="0"/></linearGradient>`);
});
const ridgeSvg = ridges.map((r, i) => `<use href="#t${i + 1}" fill="url(#r${i + 1})"/><use href="#t${i + 1}" fill="url(#w${i + 1})"/><rect x="-10" y="${n1(r.top)}" width="${W + 20}" height="${n1(H - r.top + 10)}" fill="url(#f${i + 1})"/>`);
const frontD = ridge(front);

const fogEllipse = (cx, cy, rx, ry, o) => `<ellipse cx="${cx}" cy="${cy}" rx="${rx}" ry="${ry}" fill="#dcebdc" opacity="${o}"/>`;
const fogs = ridges.map((r, i) => fogEllipse([980, 520, 1350, 900][i], r.base - 6, 1500 + i * 60, 34 + i * 6, i > 1 ? 0.09 : 0.15));

// Fireflies, mostly lower middle-right.
let flies = '';
for (let i = 0; i < 42; i++) {
    const x = i < 34 ? rr(760, 1860) : rr(260, 760);
    const y = i < 34 ? rr(640, 1000) : rr(700, 980);
    const r = rr(1, 2.6);
    const o = rr(0.25, 0.8);
    flies += `<circle cx="${n1(x)}" cy="${n1(y)}" r="${n1(r * 5)}" fill="url(#fly)" opacity="${n1(o * 0.7)}"/><circle cx="${n1(x)}" cy="${n1(y)}" r="${n1(r)}" fill="#ffd27a" opacity="${n1(o * 100) / 100}"/>`;
}

// Foreground: irregular moss/fern mound, highest at the corners, with grass blades.
const mound = (x) => 1080 - (34 + 90 * Math.pow(Math.abs(x - 960) / 960, 2) + 12 * Math.sin(x / 130) + 9 * Math.sin(x / 47 + 2) + rr(-5, 5));
let ferns = `M-20 ${H + 10}`;
for (let x = -20; x <= W + 20; x += 14) ferns += `L${n1(x)} ${n1(mound(x))}`;
ferns += `L${W + 20} ${H + 10}Z`;
for (let i = 0; i < 70; i++) {
    const x = rr(-10, W + 10);
    const y = mound(x) + 4;
    const hh = rr(22, 70) * (0.6 + 0.8 * Math.abs(x - 960) / 960);
    const bend = rr(-26, 26);
    ferns += `M${n1(x - 3)} ${n1(y)}Q${n1(x + bend * 0.3)} ${n1(y - hh * 0.6)} ${n1(x + bend)} ${n1(y - hh)}Q${n1(x + bend * 0.5 + 4)} ${n1(y - hh * 0.55)} ${n1(x + 4)} ${n1(y)}Z`;
}

const svg = `<?xml version="1.0" encoding="UTF-8"?>
<!-- Generated by assets/desktop-wallpaper-forest/generate.mjs - do not edit by hand. -->
<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}" preserveAspectRatio="xMidYMid slice">
<defs>
<linearGradient id="sky" x1="0" y1="0" x2="0" y2="1">
<stop offset="0" stop-color="#0a1714"/><stop offset="0.38" stop-color="#142a23"/><stop offset="0.52" stop-color="#3a4630"/><stop offset="0.58" stop-color="#5e5236"/><stop offset="0.62" stop-color="#8a6a3c"/><stop offset="0.7" stop-color="#4a4630"/><stop offset="1" stop-color="#101c17"/>
</linearGradient>
<radialGradient id="glow" cx="0.5" cy="0.5" r="0.5">
<stop offset="0" stop-color="#ffd590" stop-opacity="0.65"/><stop offset="0.35" stop-color="#ffc072" stop-opacity="0.3"/><stop offset="1" stop-color="#ffb060" stop-opacity="0"/>
</radialGradient>
<radialGradient id="glowWide" cx="0.5" cy="0.5" r="0.5">
<stop offset="0" stop-color="#e9a64f" stop-opacity="0.34"/><stop offset="1" stop-color="#e9a64f" stop-opacity="0"/>
</radialGradient>
<linearGradient id="far" gradientUnits="userSpaceOnUse" x1="0" y1="560" x2="0" y2="640"><stop offset="0" stop-color="#5f6648"/><stop offset="1" stop-color="#6f6c4a"/></linearGradient>
<radialGradient id="farw" gradientUnits="userSpaceOnUse" cx="1240" cy="610" r="900" gradientTransform="translate(1240 610) scale(1 0.3) translate(-1240 -610)"><stop offset="0" stop-color="#f0b060" stop-opacity="0.6"/><stop offset="1" stop-color="#e0a050" stop-opacity="0"/></radialGradient>
<radialGradient id="fly" cx="0.5" cy="0.5" r="0.5"><stop offset="0" stop-color="#ffd27a" stop-opacity="0.55"/><stop offset="1" stop-color="#ffd27a" stop-opacity="0"/></radialGradient>
<radialGradient id="vig" cx="0.6" cy="0.5" r="0.78"><stop offset="0.5" stop-color="#000" stop-opacity="0"/><stop offset="1" stop-color="#000" stop-opacity="0.6"/></radialGradient>
<linearGradient id="left" x1="0" y1="0" x2="1" y2="0"><stop offset="0" stop-color="#02060a" stop-opacity="0.55"/><stop offset="1" stop-color="#02060a" stop-opacity="0"/></linearGradient>
<path id="t0" d="${farHills()}"/>
${defs.join('\n')}
<filter id="softFog" x="-20%" y="-100%" width="140%" height="300%"><feGaussianBlur stdDeviation="24"/></filter>
</defs>
<rect width="${W}" height="${H}" fill="url(#sky)"/>
<circle cx="${GLOW.x}" cy="${GLOW.y}" r="1100" fill="url(#glowWide)"/>
<circle cx="${GLOW.x}" cy="${GLOW.y}" r="560" fill="url(#glow)"/>
<circle cx="${GLOW.x}" cy="${GLOW.y}" r="200" fill="url(#glow)"/>
<use href="#t0" fill="url(#far)"/><use href="#t0" fill="url(#farw)"/>
${ridgeSvg[0]}
<g filter="url(#softFog)">${fogs[0]}</g>
${ridgeSvg[1]}
<g filter="url(#softFog)">${fogs[1]}</g>
${ridgeSvg[2]}
<g filter="url(#softFog)">${fogs[2]}</g>
${ridgeSvg[3]}
<g filter="url(#softFog)">${fogs[3]}</g>
${flies}
<path fill="#050b09" d="${frontD}"/>
<g fill="#030706"><path d="${ferns}"/></g>
<rect width="${W}" height="${H}" fill="url(#vig)"/>
<rect width="700" height="${H}" fill="url(#left)"/>
</svg>
`;
mkdirSync(dirname(out), { recursive: true });
writeFileSync(out, svg);
console.log(`wrote ${out} (${(svg.length / 1024).toFixed(1)} KB)`);
