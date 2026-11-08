// Cyber Neon Metropolis playfield art (plan 7 / T16): a synthwave night city painted over the darkened underlay.
// paintArt(kit) runs above the underlay and below lane strips, wall shadows and inserts, so every insert stays readable.
// Deterministic: all scatter comes from rngNext on a fixed seed.
import { FONT_STACK, rngNext } from '../../config.js';

const CYAN = '#37d9ff', PINK = '#ff48c4', VIOLET = '#a98bff', RED = '#ff5a3c', AMBER = '#ffb347';
const CORE = [0, 0.66];
const ARC = { cx: 0, cy: 0.8125, r: 0.2575 };

function glowCircle(ctx, x, y, r, inner, outer) {
    const g = ctx.createRadialGradient(x, y, 0, x, y, r);
    g.addColorStop(0, inner); g.addColorStop(1, outer);
    ctx.fillStyle = g; ctx.beginPath(); ctx.arc(x, y, r, 0, Math.PI * 2); ctx.fill();
}

// Soft coloured pool of light on the playfield (metres), drawn with additive blending.
function zone(kit, x, y, rx, ry, color, alpha) {
    const { ctx } = kit, p = kit.px(x, y);
    ctx.save();
    ctx.globalCompositeOperation = 'lighter';
    ctx.translate(p[0], p[1]); ctx.scale(1, ry / rx);
    glowCircle(ctx, 0, 0, kit.m(rx), color + alphaHex(alpha), color + '00');
    ctx.restore();
}
const alphaHex = a => Math.round(Math.max(0, Math.min(1, a)) * 255).toString(16).padStart(2, '0');

function darkWash(kit) {
    const { ctx, W, H } = kit;
    const g = ctx.createLinearGradient(0, 0, 0, H);
    g.addColorStop(0, 'rgba(18,6,44,.46)'); g.addColorStop(0.55, 'rgba(8,4,26,.50)'); g.addColorStop(1, 'rgba(2,3,14,.62)');
    ctx.fillStyle = g; ctx.fillRect(0, 0, W, H);
}

// Distant city along the top arc: layered silhouettes with lit windows and a neon rim.
function skyline(kit) {
    const { ctx } = kit, rng = { seed: 0x5CA7 };
    const arcPath = () => {
        ctx.beginPath();
        const c = kit.px(ARC.cx, ARC.cy), r = kit.m(ARC.r - 0.004);
        ctx.arc(c[0], c[1], r, Math.PI, Math.PI * 2);
        ctx.closePath();
    };
    ctx.save();
    arcPath(); ctx.clip();
    const layers = [{ base: 0.935, top: 0.045, col: 'rgba(30,12,66,.78)', win: 0.22, edge: VIOLET }, { base: 0.935, top: 0.07, col: 'rgba(14,6,38,.92)', win: 0.32, edge: PINK }];
    layers.forEach((L, li) => {
        const step = 0.0155 - li * 0.002;
        for (let x = -0.27; x < 0.27; x += step) {
            const w = step * (0.9 + rngNext(rng) * 0.9), hgt = 0.02 + rngNext(rng) * L.top + (1 - Math.abs(x) / 0.27) * 0.03;
            const a = kit.px(x, L.base + hgt), b = kit.px(x + w, L.base), bw = b[0] - a[0], bh = b[1] - a[1];
            ctx.fillStyle = L.col; ctx.fillRect(a[0], a[1], bw, bh);
            ctx.fillStyle = L.edge; ctx.globalAlpha = 0.65; ctx.fillRect(a[0], a[1], bw, Math.max(1.5, kit.m(0.0007))); ctx.globalAlpha = 1;
            const cw = kit.m(0.0024), ch = kit.m(0.0034);
            for (let wy = a[1] + ch * 1.6; wy < b[1] - ch; wy += ch * 2.1) {
                for (let wx = a[0] + cw * 0.8; wx < a[0] + bw - cw; wx += cw * 1.9) {
                    if (rngNext(rng) > L.win) continue;
                    const k = rngNext(rng);
                    ctx.fillStyle = k < 0.45 ? CYAN : k < 0.75 ? PINK : AMBER; ctx.globalAlpha = 0.25 + rngNext(rng) * 0.35;
                    ctx.fillRect(wx, wy, cw, ch);
                }
            }
            ctx.globalAlpha = 1;
        }
    });
    // Haze at the base of the skyline.
    const y0 = kit.px(0, 0.935)[1], y1 = kit.px(0, 1.06)[1];
    const hz = ctx.createLinearGradient(0, y0, 0, y1);
    hz.addColorStop(0, 'rgba(255,72,196,.20)'); hz.addColorStop(0.6, 'rgba(55,217,255,.05)'); hz.addColorStop(1, 'rgba(0,0,0,0)');
    ctx.fillStyle = hz; ctx.fillRect(0, y1, kit.W, y0 - y1);
    ctx.restore();
}

// Striped synthwave sun and a perspective grid floor in the lower half.
function gridFloor(kit) {
    const { ctx } = kit, hor = 0.40, vp = [0, hor];
    ctx.save();
    // sun
    const sc = kit.px(0, hor), sr = kit.m(0.095);
    ctx.save();
    ctx.beginPath(); ctx.rect(0, 0, kit.W, sc[1]); ctx.clip();
    const sg = ctx.createLinearGradient(0, sc[1] - sr, 0, sc[1]);
    sg.addColorStop(0, 'rgba(255,214,90,.55)'); sg.addColorStop(0.55, 'rgba(255,72,160,.52)'); sg.addColorStop(1, 'rgba(160,40,255,.50)');
    ctx.fillStyle = sg; ctx.beginPath(); ctx.arc(sc[0], sc[1], sr, 0, Math.PI * 2); ctx.fill();
    ctx.fillStyle = 'rgba(8,4,24,.96)';
    for (let i = 0; i < 6; i++) { const t = i / 6, y = sc[1] - sr * (0.08 + t * 0.62), th = kit.m(0.0016 + t * 0.0034) * 0.9; ctx.fillRect(sc[0] - sr, y - th, sr * 2, th); }
    ctx.restore();
    // horizon glow
    const hp = kit.px(0, hor), hg = ctx.createLinearGradient(0, hp[1] - kit.m(0.03), 0, hp[1] + kit.m(0.03));
    hg.addColorStop(0, 'rgba(255,72,196,0)'); hg.addColorStop(0.5, 'rgba(255,72,196,.45)'); hg.addColorStop(1, 'rgba(55,217,255,0)');
    ctx.fillStyle = hg; ctx.fillRect(0, hp[1] - kit.m(0.03), kit.W, kit.m(0.06));
    // floor
    ctx.beginPath(); const fa = kit.px(-0.26, hor), fb = kit.px(0.26, 0.04);
    ctx.rect(fa[0], fa[1], fb[0] - fa[0], fb[1] - fa[1]); ctx.clip();
    const fl = ctx.createLinearGradient(0, fa[1], 0, fb[1]);
    fl.addColorStop(0, 'rgba(70,20,120,.38)'); fl.addColorStop(1, 'rgba(10,6,40,.1)');
    ctx.fillStyle = fl; ctx.fillRect(fa[0], fa[1], fb[0] - fa[0], fb[1] - fa[1]);
    ctx.lineCap = 'round';
    ctx.shadowColor = PINK; ctx.shadowBlur = kit.m(0.004);
    for (let i = -14; i <= 14; i++) {                       // radial lines to the vanishing point
        const x = i * 0.045, a = kit.px(vp[0] + i * 0.0035, vp[1]), b = kit.px(x, 0.04);
        ctx.strokeStyle = i % 2 ? 'rgba(255,72,196,.34)' : 'rgba(55,217,255,.34)'; ctx.lineWidth = kit.m(0.0007);
        ctx.beginPath(); ctx.moveTo(a[0], a[1]); ctx.lineTo(b[0], b[1]); ctx.stroke();
    }
    for (let i = 1; i <= 11; i++) {                          // depth lines, spacing grows toward the player
        const t = i / 11, y = hor - 0.37 * t * t;
        const a = kit.px(-0.26, y), b = kit.px(0.26, y);
        ctx.strokeStyle = `rgba(55,217,255,${0.14 + 0.28 * t})`; ctx.lineWidth = kit.m(0.0006 + 0.0007 * t);
        ctx.beginPath(); ctx.moveTo(a[0], a[1]); ctx.lineTo(b[0], b[1]); ctx.stroke();
    }
    ctx.restore();
}

function insertGuard(layout) {
    const zones = [];
    for (const l of layout.lamps || []) if (!l.mount || l.mount === 'insert') zones.push([l.x, l.y, Math.max(l.w || 0.016, l.h || 0.016) * 0.75 + 0.006]);
    for (const d of layout.devices || []) {
        if (d.type === 'bumper') zones.push([d.x, d.y, 0.034]);
        else if (d.x !== undefined && d.y !== undefined) zones.push([d.x, d.y, (d.r || 0.012) + 0.014]);
    }
    for (const f of layout.flippers || []) zones.push([f.x, f.y, 0.07]);
    zones.push([CORE[0], CORE[1], 0.035]);
    return (x, y) => {
        if (y < 0.20 || y > 0.99 || Math.abs(x) > 0.235 || Math.hypot(x - ARC.cx, y - ARC.cy) > ARC.r - 0.02 && y > ARC.cy) return true;
        for (const z of zones) if ((x - z[0]) ** 2 + (y - z[1]) ** 2 < z[2] * z[2]) return true;
        return false;
    };
}

// Manhattan/45 degree circuit traces that route around the inserts; vias at the ends.
function circuits(kit) {
    const { ctx } = kit, rng = { seed: 0xC1C017 }, blocked = insertGuard(kit.layout), P = 0.0105;
    const DIRS = [[1, 0], [0, 1], [-1, 0], [0, -1], [1, 1], [-1, 1], [-1, -1], [1, -1]];
    const trace = (n, color, alpha) => {
        for (let tries = 0; tries < 40; tries++) {
            let x = (Math.floor(rngNext(rng) * 44) - 22) * P, y = 0.22 + Math.floor(rngNext(rng) * 70) * P;
            if (blocked(x, y)) continue;
            let d = Math.floor(rngNext(rng) * 8);
            const pts = [[x, y]];
            for (let s = 0; s < n; s++) {
                const turn = rngNext(rng);
                if (turn < 0.22) d = (d + (rngNext(rng) < 0.5 ? 1 : 7)) % 8; else if (turn < 0.30) d = rngNext(rng) < 0.7 ? Math.floor(rngNext(rng) * 4) : 4 + Math.floor(rngNext(rng) * 4);
                const len = 1 + Math.floor(rngNext(rng) * 2);
                const nx = x + DIRS[d][0] * P * len, ny = y + DIRS[d][1] * P * len;
                let ok = true;
                for (let k = 1; k <= 4; k++) if (blocked(x + (nx - x) * k / 4, y + (ny - y) * k / 4)) { ok = false; break; }
                if (!ok) { d = (d + 2 + Math.floor(rngNext(rng) * 4)) % 8; continue; }
                x = nx; y = ny; pts.push([x, y]);
            }
            if (pts.length < 4) continue;
            kit.glowLine(pts, color, 0.0012, 0.006, alpha);
            for (const e of [pts[0], pts[pts.length - 1]]) {
                const q = kit.px(e[0], e[1]);
                ctx.save(); ctx.globalAlpha = alpha; ctx.strokeStyle = color; ctx.lineWidth = kit.m(0.0009);
                ctx.beginPath(); ctx.arc(q[0], q[1], kit.m(0.0026), 0, Math.PI * 2); ctx.stroke(); ctx.restore();
            }
            return;
        }
    };
    for (let i = 0; i < 34; i++) trace(7 + Math.floor(rngNext(rng) * 9), i % 3 === 0 ? PINK : i % 3 === 1 ? CYAN : VIOLET, 0.26 + rngNext(rng) * 0.2);
}

function hexCell(ctx, x, y, r) {
    ctx.beginPath();
    for (let i = 0; i < 6; i++) { const a = Math.PI / 6 + i * Math.PI / 3; (i ? ctx.lineTo : ctx.moveTo).call(ctx, x + Math.cos(a) * r, y + Math.sin(a) * r); }
    ctx.closePath();
}

// Hex plate behind the Data Core plus concentric target rings.
function coreArt(kit) {
    const { ctx } = kit, c = kit.px(CORE[0], CORE[1]);
    ctx.save();
    ctx.beginPath(); ctx.arc(c[0], c[1], kit.m(0.115), 0, Math.PI * 2); ctx.clip();
    const r = kit.m(0.0125), dx = r * Math.sqrt(3), dy = r * 1.5;
    ctx.lineWidth = Math.max(1, kit.m(0.0005));
    for (let row = -12; row <= 12; row++) for (let col = -12; col <= 12; col++) {
        const x = c[0] + col * dx + (row & 1 ? dx / 2 : 0), y = c[1] + row * dy;
        const d = Math.hypot(x - c[0], y - c[1]) / kit.m(0.115);
        ctx.strokeStyle = `rgba(55,217,255,${0.32 * (1 - d) + 0.04})`; hexCell(ctx, x, y, r * 0.93); ctx.stroke();
        if (((row * 7 + col * 13) & 15) === 3) { ctx.fillStyle = 'rgba(55,217,255,.16)'; hexCell(ctx, x, y, r * 0.93); ctx.fill(); }
    }
    ctx.restore();
    ctx.save();
    ctx.shadowColor = CYAN; ctx.shadowBlur = kit.m(0.008); ctx.strokeStyle = CYAN;
    [[0.045, 0.0016, 0.7, []], [0.07, 0.0010, 0.5, [0.03, 0.012]], [0.1, 0.0014, 0.55, [0.06, 0.02]]].forEach(([rad, lw, a, dash]) => {
        ctx.globalAlpha = a; ctx.lineWidth = kit.m(lw); ctx.setLineDash(dash.map(v => kit.m(v)));
        ctx.beginPath(); ctx.arc(c[0], c[1], kit.m(rad), 0, Math.PI * 2); ctx.stroke();
    });
    ctx.restore();
}

// FIREWALL (red/orange brick wall + hazard chevrons) and VAULT (magenta plate) zone dressing.
function firewallVault(kit) {
    const { ctx } = kit;
    ctx.save();
    const a = kit.px(-0.14, 0.69), b = kit.px(-0.076, 0.735);
    ctx.globalCompositeOperation = 'lighter';
    ctx.lineCap = 'round';
    for (let i = 0; i < 9; i++) {
        const t = i / 8, x = -0.136 + t * 0.060, y = 0.680 + t * 0.045;
        kit.glowLine([[x - 0.004, y - 0.0085], [x + 0.0045, y - 0.0085 + 0.0016]], i % 2 ? RED : AMBER, 0.0022, 0.006, 0.45);
    }
    void a; void b;
    ctx.restore();
    // vault enclosure plate: magenta tint with scan grid
    const poly = [[-0.139, 0.695], [-0.148, 0.750], [-0.112, 0.790], [-0.065, 0.770], [-0.069, 0.727]];
    ctx.save();
    kit.path(poly, true); ctx.clip();
    const p = kit.px(-0.108, 0.745), g = ctx.createRadialGradient(p[0], p[1], 0, p[0], p[1], kit.m(0.06));
    g.addColorStop(0, 'rgba(255,72,196,.30)'); g.addColorStop(1, 'rgba(120,20,160,.08)');
    ctx.fillStyle = g; ctx.fillRect(0, 0, kit.W, kit.H);
    ctx.strokeStyle = 'rgba(255,140,220,.22)'; ctx.lineWidth = Math.max(1, kit.m(0.0004));
    for (let k = -0.16; k < -0.05; k += 0.008) { const s = kit.px(k, 0.68), e = kit.px(k, 0.8); ctx.beginPath(); ctx.moveTo(s[0], s[1]); ctx.lineTo(e[0], e[1]); ctx.stroke(); }
    for (let k = 0.68; k < 0.8; k += 0.008) { const s = kit.px(-0.16, k), e = kit.px(-0.05, k); ctx.beginPath(); ctx.moveTo(s[0], s[1]); ctx.lineTo(e[0], e[1]); ctx.stroke(); }
    ctx.restore();
    kit.glowLine([...poly, poly[0]], VIOLET, 0.0016, 0.008, 0.7);
}

// Neon edges: top lane strips, orbit/ramp tracks under the 3D ramps, inlane and outlane guides.
function strips(kit) {
    const { ctx } = kit;
    const lane = (x0, x1, col) => {
        const a = kit.px(x0, 0.968), b = kit.px(x1, 0.885);
        const g = ctx.createLinearGradient(0, a[1], 0, b[1]);
        g.addColorStop(0, 'rgba(255,179,71,0)'); g.addColorStop(1, 'rgba(255,179,71,.16)');
        ctx.fillStyle = g; ctx.fillRect(a[0], a[1], b[0] - a[0], b[1] - a[1]);
        kit.glowLine([[x0, 0.968], [x0, 0.885]], col, 0.0018, 0.008, 0.7); kit.glowLine([[x1, 0.968], [x1, 0.885]], col, 0.0018, 0.008, 0.7);
    };
    lane(-0.0675, -0.0225, AMBER); lane(-0.0225, 0.0225, AMBER); lane(0.0225, 0.0675, AMBER);
    for (const p of kit.layout.paths || []) {
        if (p.kind === 'tube' || !p.points) continue;
        const pts = p.points.map(q => [q[0], q[1]]);
        kit.glowLine(pts, p.color || CYAN, 0.022, 0.03, 0.10);
        kit.glowLine(pts, p.color || CYAN, 0.0014, 0.006, 0.5);
    }
    for (const w of kit.layout.walls || []) {
        if (!/^(in|out)lane|guide|sling.*face/i.test(w.id || '') || !w.points || w.points[0][1] > 0.5) continue;
        kit.glowLine(w.points, /out/i.test(w.id) ? PINK : CYAN, 0.0014, 0.007, 0.4);
    }
}

function scanlines(kit) {
    const { ctx } = kit;
    ctx.save();
    ctx.fillStyle = 'rgba(0,0,0,.14)';
    const step = kit.m(0.0052), th = Math.max(1, step * 0.32);
    for (let y = 0; y < kit.H; y += step) ctx.fillRect(0, y, kit.W, th);
    ctx.restore();
}

function logo(kit) {
    const { ctx } = kit, p = kit.px(0, 0.262);
    ctx.save();
    ctx.translate(p[0], p[1]); ctx.transform(1, 0, -0.12, 1, 0, 0);
    ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
    const fit = (str, px, w) => { ctx.font = `italic 900 ${px}px ${FONT_STACK}`; const m = ctx.measureText(str).width; return m > w ? px * w / m : px; };
    if ('letterSpacing' in ctx) ctx.letterSpacing = `${kit.m(0.002)}px`;
    const s1 = fit('NEON', kit.m(0.034), kit.m(0.17)), s2 = fit('METROPOLIS', kit.m(0.0175), kit.m(0.17));
    ctx.font = `italic 900 ${s1}px ${FONT_STACK}`;
    ctx.shadowColor = CYAN; ctx.shadowBlur = kit.m(0.012); ctx.fillStyle = 'rgba(160,240,255,.55)'; ctx.fillText('NEON', 0, -kit.m(0.0045));
    ctx.shadowBlur = 0; ctx.lineWidth = kit.m(0.0008); ctx.strokeStyle = CYAN; ctx.strokeText('NEON', 0, -kit.m(0.0045));
    ctx.font = `italic 900 ${s2}px ${FONT_STACK}`;
    ctx.shadowColor = PINK; ctx.shadowBlur = kit.m(0.010); ctx.fillStyle = 'rgba(255,150,220,.6)'; ctx.fillText('METROPOLIS', 0, kit.m(0.0175));
    ctx.shadowBlur = 0; ctx.strokeStyle = PINK; ctx.lineWidth = kit.m(0.0006); ctx.strokeText('METROPOLIS', 0, kit.m(0.0175));
    ctx.restore();
    kit.glowLine([[-0.085, 0.2395], [0.085, 0.2395]], PINK, 0.0012, 0.006, 0.5);
}

export function paintArt(kit) {
    const { ctx } = kit;
    darkWash(kit);
    skyline(kit);
    // coloured zones
    zone(kit, 0, 0.66, 0.15, 0.15, CYAN, 0.30);
    zone(kit, -0.112, 0.705, 0.075, 0.06, RED, 0.34);
    zone(kit, -0.105, 0.760, 0.065, 0.06, PINK, 0.30);
    zone(kit, 0.13, 0.78, 0.07, 0.15, CYAN, 0.12);
    zone(kit, -0.16, 0.72, 0.05, 0.15, PINK, 0.12);
    gridFloor(kit);
    circuits(kit);
    coreArt(kit);
    firewallVault(kit);
    strips(kit);
    logo(kit);
    scanlines(kit);
    ctx.globalCompositeOperation = 'source-over';
}

export default paintArt;
