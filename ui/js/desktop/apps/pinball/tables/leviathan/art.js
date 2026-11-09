// Deep Sea Leviathan playfield art (plan 6.2 / 7 / T22): a darker deep-water wash, light shafts and caustics, sunken stone ruins and
// columns, a giant tentacle silhouette curling across the lower field, sonar rings around the sonar targets, jellyfish and plankton,
// the vortex round the Leviathan's head and the logo, painted over the darkened underlay.
// paintArt(kit) runs above the underlay and below lane strips, wall shadows and inserts, so every insert stays readable.
// Deterministic: all scatter comes from rngNext on fixed seeds.
import { FONT_STACK, rngNext } from '../../config.js';

const TEAL = '#43e3d5', GOLD = '#dcad63', ICE = '#bff6ff', SKY = '#7fd3ff', DEEP = '#031019';
const HEAD = [0.105, 0.880], SHIP = [0.100, 0.780], MAGNET = [0.045, 0.540], PEARL = [-0.082, 0.610];
const SONAR_AT = [[0.007, 0.657], [-0.0016, 0.6451], [-0.0102, 0.6331], [-0.2356, 0.5221], [0.2025, 0.5237]];
const FIELD = { x0: -0.2575, x1: 0.2575, y0: 0.05, y1: 1.065 };

const alphaHex = a => Math.round(Math.max(0, Math.min(1, a)) * 255).toString(16).padStart(2, '0');

// Additive soft pool of light (metres).
function pool(kit, x, y, rx, ry, color, alpha) {
    const { ctx } = kit, p = kit.px(x, y);
    ctx.save();
    ctx.globalCompositeOperation = 'lighter';
    ctx.translate(p[0], p[1]); ctx.scale(1, ry / rx);
    const g = ctx.createRadialGradient(0, 0, 0, 0, 0, kit.m(rx));
    g.addColorStop(0, color + alphaHex(alpha)); g.addColorStop(1, color + '00');
    ctx.fillStyle = g; ctx.beginPath(); ctx.arc(0, 0, kit.m(rx), 0, Math.PI * 2); ctx.fill();
    ctx.restore();
}

// Keep-out zones so detail stays calm near inserts, devices and flippers.
function guard(layout) {
    const zones = [];
    for (const l of layout.lamps || []) if (!l.mount || l.mount === 'insert') zones.push([l.x, l.y, Math.max(l.w || 0.016, l.h || 0.016) * 0.75 + 0.006]);
    for (const d of layout.devices || []) {
        if (d.type === 'bumper') zones.push([d.x, d.y, 0.034]);
        else if (d.x !== undefined && d.y !== undefined) zones.push([d.x, d.y, (d.r || 0.012) + 0.014]);
    }
    for (const f of layout.flippers || []) zones.push([f.x, f.y, 0.07]);
    return (x, y, extra = 0) => {
        for (const z of zones) if ((x - z[0]) ** 2 + (y - z[1]) ** 2 < (z[2] + extra) ** 2) return true;
        return false;
    };
}

function clipField(kit) {
    const a = kit.px(FIELD.x0, FIELD.y1), b = kit.px(FIELD.x1, FIELD.y0);
    kit.ctx.beginPath(); kit.ctx.rect(a[0], a[1], b[0] - a[0], b[1] - a[1]); kit.ctx.clip();
}

// Deep water: the colour column darkens toward the top (the abyss) and the bottom corners; a cold teal bloom lifts the middle.
function waterWash(kit) {
    const { ctx, W, H } = kit;
    const at = y => Math.max(0, Math.min(1, kit.px(0, y)[1] / H));
    const g = ctx.createLinearGradient(0, 0, 0, H);
    g.addColorStop(0, 'rgba(1,6,14,.72)'); g.addColorStop(at(0.93), 'rgba(2,10,22,.50)'); g.addColorStop(at(0.70), 'rgba(2,14,30,.34)');
    g.addColorStop(at(0.40), 'rgba(3,22,40,.30)'); g.addColorStop(at(0.14), 'rgba(2,12,26,.46)'); g.addColorStop(1, 'rgba(1,6,14,.62)');
    ctx.fillStyle = g; ctx.fillRect(0, 0, W, H);
    // the open lower field gets a rich ocean-blue body so the underlay's dark blue never reads as flat black
    pool(kit, 0, 0.40, 0.30, 0.30, '#0f5f86', 0.40);
    pool(kit, 0, 0.18, 0.28, 0.14, '#0b4a6e', 0.30);
}

// Sun shafts falling through the surface, brighter near the top, fading downward.
function lightShafts(kit) {
    const { ctx } = kit, rng = { seed: 0x5A47F5 };
    ctx.save(); clipField(kit);
    ctx.globalCompositeOperation = 'lighter';
    for (let i = 0; i < 9; i++) {
        const x0 = -0.30 + i * 0.075 + (rngNext(rng) - 0.5) * 0.03, w0 = 0.012 + rngNext(rng) * 0.012, lean = 0.10 + rngNext(rng) * 0.05, len = 0.60 + rngNext(rng) * 0.25;
        const a = kit.px(x0, 1.07), b = kit.px(x0 + w0, 1.07), c = kit.px(x0 + w0 * 3.6 - lean, 1.07 - len), d = kit.px(x0 - w0 * 2 - lean, 1.07 - len);
        const g = ctx.createLinearGradient(a[0], a[1], (c[0] + d[0]) / 2, c[1]);
        g.addColorStop(0, 'rgba(170,240,255,.20)'); g.addColorStop(0.5, 'rgba(90,200,230,.07)'); g.addColorStop(1, 'rgba(60,170,220,0)');
        ctx.fillStyle = g; ctx.beginPath(); ctx.moveTo(a[0], a[1]); ctx.lineTo(b[0], b[1]); ctx.lineTo(c[0], c[1]); ctx.lineTo(d[0], d[1]); ctx.closePath(); ctx.fill();
    }
    ctx.restore();
}

// Caustic light network: thousands of tiny bright cell edges (a warped lattice) added over the lower two thirds of the field.
function caustics(kit) {
    const { ctx } = kit, rng = { seed: 0xCA057 };
    ctx.save(); clipField(kit);
    ctx.globalCompositeOperation = 'lighter';
    ctx.lineCap = 'round'; ctx.lineJoin = 'round';
    const warp = (x, y) => [x + Math.sin(y * 52 + 1.3) * 0.012 + Math.sin(y * 21 + x * 17) * 0.009, y + Math.cos(x * 47 + 0.4) * 0.012 + Math.sin(x * 19 - y * 13) * 0.008];
    const cell = 0.036;
    for (let y = 0.07; y < 0.78; y += cell * 0.82) {
        const fade = Math.max(0, 1 - Math.max(0, y - 0.20) / 0.62);
        for (let x = -0.28; x < 0.28; x += cell) {
            const jx = x + (rngNext(rng) - 0.5) * cell * 0.7, jy = y + (rngNext(rng) - 0.5) * cell * 0.7;
            const poly = [];
            const n = 5 + Math.floor(rngNext(rng) * 2), r = cell * (0.40 + rngNext(rng) * 0.22);
            for (let k = 0; k < n; k++) { const a = k / n * Math.PI * 2 + rngNext(rng) * 0.4, rr = r * (0.8 + rngNext(rng) * 0.4); poly.push(warp(jx + Math.cos(a) * rr, jy + Math.sin(a) * rr * 0.9)); }
            kit.path(poly, true);
            const a = (0.020 + rngNext(rng) * 0.040) * fade;
            ctx.strokeStyle = `rgba(150,236,255,${a})`; ctx.lineWidth = kit.m(0.0016 + rngNext(rng) * 0.0012); ctx.stroke();
            ctx.strokeStyle = `rgba(230,255,255,${a * 1.2})`; ctx.lineWidth = Math.max(1, kit.m(0.0004)); ctx.stroke();
        }
    }
    ctx.restore();
}

// Broken stone column silhouette: fluted shaft, capital and a jagged break; rim-lit from the right in teal.
function column(kit, x, y, h, w, seed, broken = 0.0, alpha = 0.72) {
    const { ctx } = kit, rng = { seed };
    const base = kit.px(x, y), W = kit.m(w), Hh = kit.m(h), top = base[1] - Hh;
    ctx.save();
    ctx.globalAlpha = alpha;
    const g = ctx.createLinearGradient(base[0] - W, 0, base[0] + W, 0);
    g.addColorStop(0, 'rgba(8,24,34,.95)'); g.addColorStop(0.55, 'rgba(18,48,62,.95)'); g.addColorStop(1, 'rgba(34,96,112,.95)');
    ctx.fillStyle = g;
    ctx.beginPath();
    ctx.moveTo(base[0] - W * 1.25, base[1]); ctx.lineTo(base[0] - W * 0.95, base[1] - Hh * 0.05); ctx.lineTo(base[0] - W * 0.82, top + (broken ? Hh * broken * 0.2 : 0));
    if (broken) {
        const pts = 5; for (let i = 1; i <= pts; i++) ctx.lineTo(base[0] - W * 0.82 + (W * 1.64) * i / pts, top + Hh * broken * (rngNext(rng) * 0.4 + (i % 2 ? 0 : 0.2)));
    } else { ctx.lineTo(base[0] - W * 1.1, top); ctx.lineTo(base[0] + W * 1.1, top); }
    ctx.lineTo(base[0] + W * 0.82, base[1] - Hh * 0.05); ctx.lineTo(base[0] + W * 1.25, base[1]); ctx.closePath(); ctx.fill();
    // flutes
    ctx.strokeStyle = 'rgba(2,10,16,.55)'; ctx.lineWidth = Math.max(1, kit.m(0.0007));
    for (let i = -2; i <= 2; i++) { ctx.beginPath(); ctx.moveTo(base[0] + i * W * 0.32, base[1] - Hh * 0.06); ctx.lineTo(base[0] + i * W * 0.30, top + Hh * (broken ? broken * 0.5 : 0.08)); ctx.stroke(); }
    // capital + base blocks
    if (!broken) { ctx.fillStyle = 'rgba(26,70,86,.95)'; ctx.fillRect(base[0] - W * 1.35, top - kit.m(0.003), W * 2.7, kit.m(0.005)); }
    ctx.fillStyle = 'rgba(14,40,52,.95)'; ctx.fillRect(base[0] - W * 1.5, base[1] - kit.m(0.004), W * 3.0, kit.m(0.005));
    // rim light and algae glow
    ctx.strokeStyle = 'rgba(100,240,235,.55)'; ctx.lineWidth = Math.max(1, kit.m(0.0010));
    ctx.beginPath(); ctx.moveTo(base[0] + W * 0.84, base[1] - Hh * 0.06); ctx.lineTo(base[0] + W * 0.80, top + Hh * 0.12); ctx.stroke();
    for (let i = 0; i < 4; i++) {
        const yy = base[1] - Hh * (0.1 + rngNext(rng) * 0.7), xx = base[0] + (rngNext(rng) - 0.5) * W * 1.4;
        ctx.fillStyle = 'rgba(80,255,200,.45)'; ctx.beginPath(); ctx.arc(xx, yy, kit.m(0.0016), 0, Math.PI * 2); ctx.fill();
    }
    ctx.restore();
}

// Ruined colonnade fading into the murk at the lower sides plus a collapsed arch behind the logo.
function ruins(kit) {
    const { ctx } = kit;
    ctx.save(); clipField(kit);
    // left and right: two columns each at different depths, the far ones paler and smaller
    const set = [
        [-0.226, 0.152, 0.088, 0.0092, 0x11A1, 0.40, 0.55], [-0.196, 0.130, 0.060, 0.0070, 0x11A2, 0, 0.45], [-0.168, 0.112, 0.045, 0.0060, 0x11A3, 0.55, 0.40],
        [0.196, 0.130, 0.060, 0.0070, 0x22A1, 0, 0.45], [0.168, 0.112, 0.045, 0.0060, 0x22A2, 0.5, 0.40], [0.226, 0.150, 0.095, 0.0092, 0x22A3, 0.25, 0.55]
    ];
    for (const [x, y, h, w, seed, br, a] of set) column(kit, x, y, h, w, seed, br, a);
    // fallen drum pieces and rubble along the floor line
    const rng = { seed: 0xBEE5 };
    for (let i = 0; i < 18; i++) {
        const left = i % 2 === 0, x = (left ? -0.245 : 0.235) + (left ? 1 : -1) * rngNext(rng) * 0.07, y = 0.075 + rngNext(rng) * 0.035;
        const p = kit.px(x, y), w = kit.m(0.004 + rngNext(rng) * 0.006), h = kit.m(0.0025 + rngNext(rng) * 0.003);
        ctx.fillStyle = `rgba(${14 + rngNext(rng) * 14},${40 + rngNext(rng) * 24},${52 + rngNext(rng) * 24},.8)`;
        ctx.beginPath(); ctx.ellipse(p[0], p[1], w, h, rngNext(rng) - 0.5, 0, Math.PI * 2); ctx.fill();
    }
    // the great broken arch behind the logo
    ctx.globalAlpha = 0.5;
    const c = kit.px(-0.017, 0.205), R = kit.m(0.115);
    ctx.strokeStyle = 'rgba(14,52,66,.92)'; ctx.lineWidth = kit.m(0.018); ctx.lineCap = 'butt';
    ctx.beginPath(); ctx.arc(c[0], c[1] + kit.m(0.03), R, Math.PI * 1.08, Math.PI * 1.62); ctx.stroke();
    ctx.strokeStyle = 'rgba(100,240,235,.35)'; ctx.lineWidth = Math.max(1, kit.m(0.0010));
    ctx.beginPath(); ctx.arc(c[0], c[1] + kit.m(0.03), R + kit.m(0.0085), Math.PI * 1.08, Math.PI * 1.62); ctx.stroke();
    ctx.restore();
}

// A tapered, sucker-studded tentacle along a Catmull-Rom spine: dark body, teal rim light, glowing sucker rows.
function tentacle(kit, spine, w0, w1, seed, alpha = 0.85, tipCurl = 1) {
    const { ctx } = kit, rng = { seed };
    const pts = [];
    const N = 64;
    const cr = (p0, p1, p2, p3, t) => {
        const t2 = t * t, t3 = t2 * t;
        return [0.5 * ((2 * p1[0]) + (-p0[0] + p2[0]) * t + (2 * p0[0] - 5 * p1[0] + 4 * p2[0] - p3[0]) * t2 + (-p0[0] + 3 * p1[0] - 3 * p2[0] + p3[0]) * t3),
            0.5 * ((2 * p1[1]) + (-p0[1] + p2[1]) * t + (2 * p0[1] - 5 * p1[1] + 4 * p2[1] - p3[1]) * t2 + (-p0[1] + 3 * p1[1] - 3 * p2[1] + p3[1]) * t3)];
    };
    const segs = spine.length - 1;
    for (let i = 0; i <= N; i++) {
        const f = i / N * segs, s = Math.min(segs - 1, Math.floor(f)), t = f - s;
        pts.push(cr(spine[Math.max(0, s - 1)], spine[s], spine[s + 1], spine[Math.min(segs, s + 2)], t));
    }
    const left = [], right = [], nrm = [];
    for (let i = 0; i <= N; i++) {
        const a = pts[Math.max(0, i - 1)], b = pts[Math.min(N, i + 1)], dx = b[0] - a[0], dy = b[1] - a[1], l = Math.hypot(dx, dy) || 1;
        const w = (w0 + (w1 - w0) * Math.pow(i / N, 0.8)) * (1 + 0.06 * Math.sin(i * 0.9));
        nrm.push([-dy / l, dx / l, w]);
        left.push([pts[i][0] - dy / l * w, pts[i][1] + dx / l * w]); right.push([pts[i][0] + dy / l * w, pts[i][1] - dx / l * w]);
    }
    ctx.save(); clipField(kit);
    // drop shadow glow
    ctx.globalAlpha = alpha;
    const outline = () => { ctx.beginPath(); left.forEach((p, i) => { const q = kit.px(p[0], p[1]); if (i) ctx.lineTo(q[0], q[1]); else ctx.moveTo(q[0], q[1]); }); for (let i = N; i >= 0; i--) { const q = kit.px(right[i][0], right[i][1]); ctx.lineTo(q[0], q[1]); } ctx.closePath(); };
    ctx.save(); ctx.shadowColor = 'rgba(0,0,0,.8)'; ctx.shadowBlur = kit.m(0.016); ctx.shadowOffsetY = kit.m(0.004); outline(); ctx.fillStyle = 'rgba(3,14,24,.92)'; ctx.fill(); ctx.restore();
    // body shading: dark teal gradient across the width approximated by a band stack
    outline(); ctx.save(); ctx.clip();
    for (let k = 0; k < 4; k++) {
        const o = (k - 1.5) / 4; ctx.beginPath();
        pts.forEach((p, i) => { const n = nrm[i], q = kit.px(p[0] + n[0] * n[2] * o * 0.9, p[1] + n[1] * n[2] * o * 0.9); if (i) ctx.lineTo(q[0], q[1]); else ctx.moveTo(q[0], q[1]); });
        ctx.strokeStyle = `rgba(${22 + k * 18},${70 + k * 30},${92 + k * 26},.78)`; ctx.lineWidth = kit.m(w0 * 0.55); ctx.lineCap = 'round'; ctx.stroke();
    }
    // mottled skin
    for (let i = 0; i < 70; i++) {
        const j = Math.floor(rngNext(rng) * N), n = nrm[j], o = (rngNext(rng) - 0.5) * 1.6, q = kit.px(pts[j][0] + n[0] * n[2] * o, pts[j][1] + n[1] * n[2] * o);
        ctx.fillStyle = `rgba(${rngNext(rng) < 0.5 ? '6,26,38' : '60,160,170'},${0.10 + rngNext(rng) * 0.14})`; ctx.beginPath(); ctx.arc(q[0], q[1], kit.m(0.002 + rngNext(rng) * 0.005), 0, Math.PI * 2); ctx.fill();
    }
    ctx.restore();
    // rim light on the outer edge, glow
    ctx.save(); ctx.globalCompositeOperation = 'lighter'; ctx.shadowColor = TEAL; ctx.shadowBlur = kit.m(0.010);
    ctx.beginPath(); right.forEach((p, i) => { const q = kit.px(p[0], p[1]); if (i) ctx.lineTo(q[0], q[1]); else ctx.moveTo(q[0], q[1]); });
    ctx.strokeStyle = 'rgba(90,240,225,.85)'; ctx.lineWidth = Math.max(1, kit.m(0.0020)); ctx.stroke(); ctx.restore();
    // suckers along the inner (left) side: pairs of rings shrinking toward the tip, bioluminescent cores
    ctx.save(); ctx.globalCompositeOperation = 'lighter';
    for (let i = 3; i < N - 1; i += 3) {
        const n = nrm[i], f = 1 - i / N * 0.8, o = -0.45 * n[2], q = kit.px(pts[i][0] + n[0] * o, pts[i][1] + n[1] * o), r = kit.m(w0 * 0.34 * f + 0.0012);
        ctx.strokeStyle = `rgba(150,240,235,${0.55 * f})`; ctx.lineWidth = Math.max(1, kit.m(0.0010)); ctx.beginPath(); ctx.arc(q[0], q[1], r, 0, Math.PI * 2); ctx.stroke();
        const g = ctx.createRadialGradient(q[0], q[1], 0, q[0], q[1], r * 0.8); g.addColorStop(0, `rgba(120,255,235,${0.65 * f})`); g.addColorStop(1, 'rgba(60,200,200,0)');
        ctx.fillStyle = g; ctx.beginPath(); ctx.arc(q[0], q[1], r * 0.8, 0, Math.PI * 2); ctx.fill();
    }
    ctx.restore();
    ctx.restore();
    return pts;
}

// Giant tentacles curling across the lower field: one from the lower right sweeping across, one from the lower left reaching up the
// outlane, the tips curled. Kept translucent so the inserts and lanes on top stay readable.
function tentacles(kit) {
    tentacle(kit, [[0.300, 0.060], [0.262, 0.150], [0.225, 0.235], [0.150, 0.330], [0.060, 0.372], [-0.035, 0.366], [-0.085, 0.332], [-0.092, 0.292], [-0.062, 0.268], [-0.030, 0.278]], 0.036, 0.005, 0x7E471, 0.88);
    tentacle(kit, [[-0.300, 0.050], [-0.268, 0.140], [-0.232, 0.210], [-0.250, 0.300], [-0.215, 0.410], [-0.150, 0.480], [-0.128, 0.535]], 0.027, 0.004, 0x7E472, 0.80);
}

// Sonar rings: concentric broken arcs that expand away from each sonar target; stronger arcs outward.
function sonarRings(kit) {
    const { ctx } = kit;
    ctx.save(); clipField(kit);
    ctx.globalCompositeOperation = 'lighter'; ctx.lineCap = 'round';
    SONAR_AT.forEach(([x, y], k) => {
        const c = kit.px(x, y);
        pool(kit, x, y, 0.065, 0.065, TEAL, 0.22);
        for (let i = 0; i < 4; i++) {
            const r = kit.m(0.022 + i * 0.016), a = 0.62 - i * 0.14;
            ctx.strokeStyle = `rgba(100,245,235,${a})`; ctx.lineWidth = Math.max(1, kit.m(0.0016 - i * 0.00025));
            ctx.shadowColor = TEAL; ctx.shadowBlur = kit.m(0.006);
            ctx.setLineDash([kit.m(0.016 + i * 0.006), kit.m(0.006 + i * 0.004)]);
            ctx.beginPath(); ctx.arc(c[0], c[1], r, 0, Math.PI * 2); ctx.stroke();
        }
        ctx.setLineDash([]);
    });
    // a long sonar sweep sector across the lower middle, pointing at the Abyss magnet
    const m = kit.px(MAGNET[0], MAGNET[1]);
    for (let i = 0; i < 5; i++) {
        ctx.strokeStyle = `rgba(100,240,235,${0.34 - i * 0.055})`; ctx.lineWidth = Math.max(1, kit.m(0.0012)); ctx.shadowBlur = 0;
        ctx.setLineDash([kit.m(0.010), kit.m(0.008)]); ctx.beginPath(); ctx.arc(m[0], m[1], kit.m(0.030 + i * 0.020), 0, Math.PI * 2); ctx.stroke();
    }
    // pearl channel halo: faint rings round the captive pearl
    const pc = kit.px(PEARL[0], PEARL[1]);
    for (let i = 0; i < 3; i++) { ctx.strokeStyle = `rgba(220,255,255,${0.28 - i * 0.07})`; ctx.setLineDash([]); ctx.lineWidth = Math.max(1, kit.m(0.0010)); ctx.beginPath(); ctx.arc(pc[0], pc[1], kit.m(0.020 + i * 0.012), 0, Math.PI * 2); ctx.stroke(); }
    ctx.restore();
}

// Whirlpool under the Leviathan's head and a golden lamplit pool round the wreck.
function vortex(kit) {
    const { ctx } = kit;
    ctx.save(); clipField(kit);
    pool(kit, HEAD[0], HEAD[1], 0.12, 0.10, '#0b6f8f', 0.55);
    pool(kit, HEAD[0], HEAD[1], 0.07, 0.06, TEAL, 0.22);
    ctx.globalCompositeOperation = 'lighter'; ctx.lineCap = 'round';
    const c = kit.px(HEAD[0], HEAD[1]);
    for (let k = 0; k < 4; k++) {
        ctx.beginPath();
        for (let i = 0; i <= 80; i++) {
            const t = i / 80, a = t * Math.PI * 3.4 + k * Math.PI / 2, r = kit.m(0.034 + t * 0.075);
            const x = c[0] + Math.cos(a) * r, y = c[1] + Math.sin(a) * r * 0.92;
            if (i) ctx.lineTo(x, y); else ctx.moveTo(x, y);
        }
        ctx.strokeStyle = `rgba(110,240,235,${0.34 - k * 0.04})`; ctx.lineWidth = Math.max(1, kit.m(0.0018)); ctx.shadowColor = TEAL; ctx.shadowBlur = kit.m(0.006); ctx.stroke();
    }
    ctx.restore();
    pool(kit, SHIP[0], SHIP[1], 0.075, 0.075, GOLD, 0.26);
    pool(kit, SHIP[0] + 0.01, SHIP[1] + 0.02, 0.03, 0.03, '#ffe2a0', 0.18);
}

// Bioluminescent plankton: glowing motes of three colours, denser in the water column near the ruins; kept off inserts.
function plankton(kit) {
    const { ctx } = kit, rng = { seed: 0xB10 }, blocked = guard(kit.layout);
    ctx.save(); clipField(kit);
    ctx.globalCompositeOperation = 'lighter';
    const cols = ['120,255,225', '100,200,255', '230,255,255', '255,225,160'];
    for (let i = 0; i < 420; i++) {
        const x = -0.26 + rngNext(rng) * 0.52, y = 0.06 + rngNext(rng) * 1.0, k = rngNext(rng), c = cols[k < 0.5 ? 0 : k < 0.8 ? 1 : k < 0.94 ? 2 : 3];
        if (blocked(x, y, 0.0)) continue;
        const p = kit.px(x, y), r = kit.m(0.0012 + Math.pow(rngNext(rng), 3) * 0.0038);
        const g = ctx.createRadialGradient(p[0], p[1], 0, p[0], p[1], r * 3.4);
        g.addColorStop(0, `rgba(${c},.85)`); g.addColorStop(0.3, `rgba(${c},.28)`); g.addColorStop(1, `rgba(${c},0)`);
        ctx.fillStyle = g; ctx.beginPath(); ctx.arc(p[0], p[1], r * 3.4, 0, Math.PI * 2); ctx.fill();
    }
    ctx.restore();
}

// A jellyfish: translucent glowing bell, frilled rim, trailing tentacle filaments. (x, y in metres, s = bell radius in metres)
function jelly(kit, x, y, s, color, seed, alpha = 0.7) {
    const { ctx } = kit, rng = { seed }, c = kit.px(x, y), R = kit.m(s);
    ctx.save(); ctx.globalCompositeOperation = 'lighter'; ctx.globalAlpha = alpha; ctx.lineCap = 'round';
    pool(kit, x, y - s * 0.3, s * 2.2, s * 2.2, color, 0.30);
    const g = ctx.createRadialGradient(c[0], c[1] - R * 0.2, 0, c[0], c[1], R * 1.1);
    g.addColorStop(0, color + 'aa'); g.addColorStop(0.7, color + '33'); g.addColorStop(1, color + '00');
    ctx.fillStyle = g; ctx.beginPath(); ctx.moveTo(c[0] - R, c[1]); ctx.bezierCurveTo(c[0] - R, c[1] - R * 1.5, c[0] + R, c[1] - R * 1.5, c[0] + R, c[1]);
    ctx.quadraticCurveTo(c[0], c[1] + R * 0.3, c[0] - R, c[1]); ctx.fill();
    ctx.strokeStyle = color + 'cc'; ctx.lineWidth = Math.max(1, kit.m(0.0008)); ctx.stroke();
    for (let i = 0; i < 9; i++) {
        const tx = c[0] + (i / 8 - 0.5) * R * 1.7, len = R * (1.8 + rngNext(rng) * 2.2);
        ctx.strokeStyle = color + (i % 2 ? '55' : '88'); ctx.lineWidth = Math.max(1, kit.m(0.0006));
        ctx.beginPath(); ctx.moveTo(tx, c[1] + R * 0.1);
        ctx.bezierCurveTo(tx + (rngNext(rng) - 0.5) * R, c[1] + len * 0.35, tx - (rngNext(rng) - 0.5) * R * 1.4, c[1] + len * 0.7, tx + (rngNext(rng) - 0.5) * R, c[1] + len); ctx.stroke();
    }
    ctx.restore();
}

function jellies(kit) {
    ctx_clip(kit, () => {
        jelly(kit, -0.205, 0.760, 0.013, '#7fd3ff', 0x3E11, 0.55);
        jelly(kit, 0.215, 0.470, 0.011, '#43e3d5', 0x3E12, 0.5);
        jelly(kit, -0.185, 0.385, 0.009, '#a98bff', 0x3E13, 0.5);
        jelly(kit, 0.185, 0.330, 0.014, '#7fd3ff', 0x3E14, 0.55);
        jelly(kit, -0.060, 0.480, 0.007, '#58f59a', 0x3E15, 0.40);
        jelly(kit, 0.050, 0.450, 0.008, '#43e3d5', 0x3E16, 0.40);
        jelly(kit, 0.020, 0.980, 0.010, '#a98bff', 0x3E17, 0.45);
    });
}
function ctx_clip(kit, fn) { kit.ctx.save(); clipField(kit); try { fn(); } finally { kit.ctx.restore(); } }

// Gold-and-teal ornament lines tracing the wall-side lane guides and the lower inlane rails, like inlaid trident metalwork.
function trim(kit) {
    const { ctx } = kit;
    const line = (pts, alpha = 0.7, width = 0.0012) => {
        ctx.save(); ctx.lineCap = 'round'; ctx.lineJoin = 'round';
        kit.path(pts, false); ctx.strokeStyle = 'rgba(0,0,0,.55)'; ctx.lineWidth = kit.m(width * 2.2); ctx.stroke();
        ctx.shadowColor = TEAL; ctx.shadowBlur = kit.m(0.006);
        kit.path(pts, false); ctx.strokeStyle = GOLD; ctx.globalAlpha = alpha; ctx.lineWidth = kit.m(width * 1.2); ctx.stroke();
        ctx.shadowBlur = 0; ctx.strokeStyle = '#ffeab8'; ctx.globalAlpha = alpha * 0.7; ctx.lineWidth = kit.m(width * 0.45); ctx.stroke();
        ctx.restore();
    };
    for (const x of [-0.11, -0.07, -0.03, 0.01]) line([[x, 0.96], [x, 1.01]], 0.8);
    line([[-0.2475, 0.12], [-0.2475, 0.50]], 0.5); line([[0.2145, 0.12], [0.2145, 0.50]], 0.5);
    line([[-0.165, 0.150], [-0.095, 0.175], [0.065, 0.175], [0.131, 0.150]], 0.5, 0.0010);
}

function logo(kit) {
    const { ctx } = kit, p = kit.px(-0.017, 0.2485);
    pool(kit, -0.017, 0.2485, 0.13, 0.03, '#2fd0d8', 0.20);
    ctx.save();
    ctx.translate(p[0], p[1]);
    ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
    if ('letterSpacing' in ctx) ctx.letterSpacing = `${kit.m(0.0026)}px`;
    const fit = (str, px, w) => { ctx.font = `900 ${px}px ${FONT_STACK}`; const m = ctx.measureText(str).width; return m > w ? px * w / m : px; };
    const s1 = fit('LEVIATHAN', kit.m(0.030), kit.m(0.215)), s2 = fit('DEEP SEA', kit.m(0.0105), kit.m(0.10));
    ctx.font = `900 ${s1}px ${FONT_STACK}`;
    ctx.fillStyle = 'rgba(0,0,0,.72)'; ctx.fillText('LEVIATHAN', kit.m(0.0012), -kit.m(0.0018) + kit.m(0.0022));
    const g = ctx.createLinearGradient(0, -kit.m(0.018), 0, kit.m(0.012));
    g.addColorStop(0, '#e9ffff'); g.addColorStop(0.45, '#58e6d9'); g.addColorStop(1, '#127f9a');
    ctx.shadowColor = TEAL; ctx.shadowBlur = kit.m(0.011); ctx.fillStyle = g; ctx.fillText('LEVIATHAN', 0, -kit.m(0.0018));
    ctx.shadowBlur = 0; ctx.lineWidth = kit.m(0.0007); ctx.strokeStyle = 'rgba(2,34,48,.9)'; ctx.strokeText('LEVIATHAN', 0, -kit.m(0.0018));
    ctx.font = `800 ${s2}px ${FONT_STACK}`; ctx.fillStyle = 'rgba(220,173,99,.96)'; ctx.shadowColor = 'rgba(0,0,0,.8)'; ctx.shadowBlur = kit.m(0.003); ctx.fillText('DEEP SEA', 0, -kit.m(0.0245));
    ctx.restore();
    // wave flourish under the title and golden end caps
    const wave = (x0, x1, y, amp, ph) => {
        const pts = []; for (let i = 0; i <= 40; i++) { const t = i / 40; pts.push([x0 + (x1 - x0) * t, y + Math.sin(t * Math.PI * 4 + ph) * amp]); } return pts;
    };
    ctx.save(); ctx.lineCap = 'round'; ctx.globalCompositeOperation = 'lighter';
    ctx.shadowColor = TEAL; ctx.shadowBlur = kit.m(0.005);
    kit.path(wave(-0.112, -0.035, 0.2315, 0.0014, 0), false); ctx.strokeStyle = 'rgba(110,240,230,.8)'; ctx.lineWidth = Math.max(1, kit.m(0.0009)); ctx.stroke();
    kit.path(wave(0.001, 0.078, 0.2315, 0.0014, Math.PI), false); ctx.stroke();
    ctx.restore();
}

export function paintArt(kit) {
    const { ctx } = kit;
    waterWash(kit);
    lightShafts(kit);
    caustics(kit);
    ruins(kit);
    tentacles(kit);
    vortex(kit);
    sonarRings(kit);
    pool(kit, -0.156, 0.74, 0.05, 0.12, SKY, 0.10);
    pool(kit, 0.17, 0.62, 0.05, 0.10, '#a98bff', 0.07);
    pool(kit, PEARL[0], 0.64, 0.045, 0.09, ICE, 0.12);
    jellies(kit);
    plankton(kit);
    trim(kit);
    logo(kit);
    ctx.globalCompositeOperation = 'source-over';
}

export default paintArt;
