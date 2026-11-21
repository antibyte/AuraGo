// Clockwork Observatory playfield art. rev 2 (painted underlay) draws only functional accents; rev 1 is the compact procedural fallback:
// mahogany-to-night-sky gradient, brass border lines along the walls, a brass clock dial round the orrery disc, a seeded star chart and glows.
// Deterministic: all scatter comes from rngNext on a fixed seed. No lettering.
import { rngNext } from '../../config.js';
import { paintAccents } from './art-accents.js';
import { DISC, DISC_R, WELLS, SCOPE, C } from './lamps.js';

const BRASS = C.brass, COPPER = C.copper, NIGHT = C.night, MOON = C.moon;
const alphaHex = a => Math.round(Math.max(0, Math.min(1, a)) * 255).toString(16).padStart(2, '0');

function sky(kit) {
    const { ctx, W, H } = kit;
    const g = ctx.createLinearGradient(0, 0, 0, H);      // canvas row 0 is the top of the table
    g.addColorStop(0, 'rgba(7,12,34,.80)'); g.addColorStop(0.45, 'rgba(14,20,48,.70)'); g.addColorStop(1, 'rgba(58,26,14,.78)');
    ctx.fillStyle = g; ctx.fillRect(0, 0, W, H);
}

function glow(kit, x, y, r, color, a) {
    const { ctx } = kit, p = kit.px(x, y), rr = kit.m(r), g = ctx.createRadialGradient(p[0], p[1], 0, p[0], p[1], rr);
    g.addColorStop(0, color + alphaHex(a)); g.addColorStop(1, color + '00');
    ctx.save(); ctx.globalCompositeOperation = 'lighter'; ctx.fillStyle = g;
    ctx.beginPath(); ctx.arc(p[0], p[1], rr, 0, Math.PI * 2); ctx.fill(); ctx.restore();
}

// Seeded star chart in the upper two thirds with a few constellation lines; keeps clear of the dial.
function starChart(kit) {
    const { ctx } = kit, rng = { seed: 0x0B5E7A }, pts = [];
    ctx.save();
    for (let i = 0; i < 150; i++) {
        const x = (rngNext(rng) - 0.5) * 0.48, y = 0.25 + rngNext(rng) * 0.74, s = rngNext(rng), k = rngNext(rng);
        if (Math.hypot(x - DISC[0], y - DISC[1]) < 0.085) continue;
        const p = kit.px(x, y);
        ctx.globalAlpha = 0.18 + s * 0.4; ctx.fillStyle = k < 0.7 ? MOON : BRASS;
        ctx.beginPath(); ctx.arc(p[0], p[1], Math.max(0.8, kit.m(0.0007 + s * s * 0.0016)), 0, Math.PI * 2); ctx.fill();
        if (s > 0.55) pts.push([x, y]);
    }
    ctx.globalAlpha = 0.16; ctx.strokeStyle = BRASS; ctx.lineWidth = Math.max(1, kit.m(0.0005)); ctx.setLineDash([kit.m(0.006), kit.m(0.004)]);
    for (let i = 0; i + 1 < pts.length; i += 2) {
        if (Math.hypot(pts[i][0] - pts[i + 1][0], pts[i][1] - pts[i + 1][1]) > 0.09) continue;
        kit.path([pts[i], pts[i + 1]], false); ctx.stroke();
    }
    ctx.restore();
}

// Brass clock dial: two rings round the disc and twelve ticks (the hour lamps sit on r 0.066 above it).
function dial(kit) {
    const { ctx } = kit, c = kit.px(DISC[0], DISC[1]);
    ctx.save();
    ctx.fillStyle = 'rgba(8,12,30,.55)';
    ctx.beginPath(); ctx.arc(c[0], c[1], kit.m(0.078), 0, Math.PI * 2); ctx.arc(c[0], c[1], kit.m(DISC_R + 0.004), 0, Math.PI * 2, true); ctx.fill('evenodd');
    ctx.strokeStyle = BRASS; ctx.shadowColor = BRASS; ctx.shadowBlur = kit.m(0.004);
    [[0.0765, 0.0016, 0.85], [0.0555, 0.0010, 0.7]].forEach(([r, lw, a]) => {
        ctx.globalAlpha = a; ctx.lineWidth = kit.m(lw); ctx.beginPath(); ctx.arc(c[0], c[1], kit.m(r), 0, Math.PI * 2); ctx.stroke();
    });
    ctx.shadowBlur = 0; ctx.globalAlpha = 0.8; ctx.lineWidth = kit.m(0.0011); ctx.lineCap = 'round';
    for (let k = 0; k < 12; k++) {
        const a = k * Math.PI / 6, major = k % 3 === 0, r0 = kit.m(major ? 0.0575 : 0.0595), r1 = kit.m(0.0745);
        ctx.beginPath(); ctx.moveTo(c[0] + Math.cos(a) * r0, c[1] + Math.sin(a) * r0); ctx.lineTo(c[0] + Math.cos(a) * r1, c[1] + Math.sin(a) * r1); ctx.stroke();
    }
    ctx.restore();
}

// Brass engraving along the outer walls plus copper lines along the ramp and orbit paths.
function borders(kit) {
    for (const w of kit.layout.walls || []) {
        if (!w.points || !/^(wall|rail|side|arc|top|orbit|chan|inlane|outlane|guide)/i.test(w.id || '') || w.points[0][1] < 0.2) continue;
        kit.glowLine(w.points.map(p => [p[0], p[1]]), BRASS, 0.0010, 0.004, 0.30);
    }
    for (const p of kit.layout.paths || []) {
        if (!p.points || p.kind === 'tube') continue;
        const pts = p.points.map(q => [q[0], q[1]]);
        kit.glowLine(pts, p.color || COPPER, 0.020, 0.03, 0.07);
        kit.glowLine(pts, p.color || COPPER, 0.0012, 0.005, 0.38);
    }
}

export function paintArt(kit) {
    if (kit.artRev >= 2) { paintAccents(kit); return; }   // painted underlay: functional accents only
    const { ctx } = kit;
    sky(kit);
    starChart(kit);
    glow(kit, DISC[0], DISC[1], 0.12, BRASS, 0.30);
    WELLS.forEach(([, x, y], i) => glow(kit, x, y, 0.075, i ? NIGHT : COPPER, 0.40));
    glow(kit, SCOPE[0], 0.64, 0.07, BRASS, 0.28);
    glow(kit, 0.045, 0.60, 0.05, MOON, 0.12);
    dial(kit);
    borders(kit);
    ctx.globalCompositeOperation = 'source-over';
}

export default paintArt;
