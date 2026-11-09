// Lost Temple of the Sun playfield art (plan 6.3 / 7 / T25): carved stone floor, gold inlay, a big sun mandala around the
// SUN bank and idol, jungle vines along the outlanes, glyph borders and the logo, painted over the darkened underlay.
// paintArt(kit) runs above the underlay and below lane strips, wall shadows and inserts, so every insert stays readable.
// Deterministic: all scatter comes from rngNext on fixed seeds.
import { FONT_STACK, rngNext } from '../../config.js';

const GOLD = '#ffcb65', GOLD_D = '#b9852a', AMBER = '#ffb347', JADE = '#8ad6a3', FOLIAGE = ['#1f5a2c', '#2f7a38', '#3f9444', '#2a6b33'];
const MAND = { x: 0, y: 0.70 };

const alphaHex = a => Math.round(Math.max(0, Math.min(1, a)) * 255).toString(16).padStart(2, '0');

// Additive soft pool of warm light (metres).
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

// Keep-out zones so engraving stays calm near inserts, devices and flippers.
function guard(layout) {
    const zones = [];
    for (const l of layout.lamps || []) if (!l.mount || l.mount === 'insert') zones.push([l.x, l.y, Math.max(l.w || 0.016, l.h || 0.016) * 0.75 + 0.007]);
    for (const d of layout.devices || []) {
        if (d.type === 'bumper') zones.push([d.x, d.y, 0.034]);
        else if (d.x !== undefined && d.y !== undefined) zones.push([d.x, d.y, (d.r || 0.012) + 0.016]);
    }
    for (const f of layout.flippers || []) zones.push([f.x, f.y, 0.07]);
    return (x, y, extra = 0) => {
        for (const z of zones) if ((x - z[0]) ** 2 + (y - z[1]) ** 2 < (z[2] + extra) ** 2) return true;
        return false;
    };
}

// Dark warm stone wash: tames the busy underlay inside the playfield but leaves the painted jungle temple at the top.
function stoneWash(kit) {
    const { ctx, W, H } = kit;
    const at = y => Math.max(0, Math.min(1, kit.px(0, y)[1] / H));
    const g = ctx.createLinearGradient(0, 0, 0, H);
    g.addColorStop(0, 'rgba(20,12,4,.20)'); g.addColorStop(at(1.0), 'rgba(20,12,4,.45)'); g.addColorStop(at(0.93), 'rgba(14,9,4,.90)');
    g.addColorStop(at(0.60), 'rgba(12,8,4,.90)'); g.addColorStop(1, 'rgba(8,6,3,.92)');
    ctx.fillStyle = g; ctx.fillRect(0, 0, W, H);
}

// Carved flagstones in offset rows with bevelled joints, per-slab tone, hairline cracks and moss.
function flagstones(kit) {
    const { ctx } = kit, rng = { seed: 0x7E3910 }, S = 0.0345;
    ctx.save();
    const a = kit.px(-0.2575, 0.93), b = kit.px(0.2575, 0.06);
    ctx.beginPath(); ctx.rect(a[0], a[1], b[0] - a[0], b[1] - a[1]); ctx.clip();
    let row = 0;
    for (let y = 0.06; y < 0.95; y += S, row++) {
        const off = (row & 1) * S * 0.5;
        for (let x = -0.27 - off; x < 0.27; x += S) {
            const k = rngNext(rng), tone = 0.55 + k * 0.45;
            const p = kit.px(x + 0.0012, y + S - 0.0012), w = kit.m(S - 0.0024), h = kit.m(S - 0.0024);
            ctx.fillStyle = `rgba(${Math.round(112 * tone)},${Math.round(92 * tone)},${Math.round(66 * tone)},${0.40 + k * 0.18})`;
            ctx.fillRect(p[0], p[1], w, h);
            ctx.fillStyle = 'rgba(255,226,170,.10)'; ctx.fillRect(p[0], p[1], w, Math.max(1, kit.m(0.0007)));
            ctx.fillStyle = 'rgba(255,226,170,.06)'; ctx.fillRect(p[0], p[1], Math.max(1, kit.m(0.0007)), h);
            ctx.fillStyle = 'rgba(0,0,0,.34)'; ctx.fillRect(p[0], p[1] + h, w, Math.max(1, kit.m(0.0010)));
            ctx.fillRect(p[0] + w, p[1], Math.max(1, kit.m(0.0010)), h);
            if (rngNext(rng) < 0.16) {
                ctx.strokeStyle = 'rgba(0,0,0,.34)'; ctx.lineWidth = Math.max(1, kit.m(0.0004));
                ctx.beginPath(); let cx = p[0] + w * rngNext(rng), cy = p[1];
                ctx.moveTo(cx, cy);
                for (let i = 0; i < 4; i++) { cx += (rngNext(rng) - 0.5) * w * 0.35; cy += h * 0.24; ctx.lineTo(cx, cy); }
                ctx.stroke();
            }
            if (rngNext(rng) < 0.12) {
                const mx = p[0] + w * rngNext(rng), my = p[1] + h * (0.55 + 0.45 * rngNext(rng)), r = kit.m(0.006 + 0.008 * rngNext(rng));
                const g = ctx.createRadialGradient(mx, my, 0, mx, my, r);
                g.addColorStop(0, 'rgba(70,130,60,.28)'); g.addColorStop(1, 'rgba(70,130,60,0)');
                ctx.fillStyle = g; ctx.fillRect(mx - r, my - r, r * 2, r * 2);
            }
        }
    }
    ctx.restore();
}

// Tiny carved glyph (monkey/serpent/eye/step/sun flavour from simple strokes) centred at pixel (x, y), size s px.
function glyph(ctx, x, y, s, kind) {
    ctx.beginPath();
    switch (kind % 7) {
        case 0: ctx.arc(x, y, s * 0.36, 0, Math.PI * 2); ctx.moveTo(x, y - s * 0.12); ctx.arc(x, y, s * 0.1, 0, Math.PI * 2); break;
        case 1: ctx.moveTo(x - s * 0.4, y + s * 0.3); ctx.lineTo(x - s * 0.4, y); ctx.lineTo(x - s * 0.13, y); ctx.lineTo(x - s * 0.13, y - s * 0.3); ctx.lineTo(x + s * 0.13, y - s * 0.3); ctx.lineTo(x + s * 0.13, y); ctx.lineTo(x + s * 0.4, y); ctx.lineTo(x + s * 0.4, y + s * 0.3); break;
        case 2: ctx.moveTo(x - s * 0.4, y); ctx.quadraticCurveTo(x, y - s * 0.5, x + s * 0.4, y); ctx.quadraticCurveTo(x, y + s * 0.5, x - s * 0.4, y); ctx.moveTo(x + s * 0.1, y); ctx.arc(x, y, s * 0.1, 0, Math.PI * 2); break;
        case 3: ctx.moveTo(x, y - s * 0.4); ctx.lineTo(x + s * 0.36, y); ctx.lineTo(x, y + s * 0.4); ctx.lineTo(x - s * 0.36, y); ctx.closePath(); break;
        case 4: ctx.moveTo(x - s * 0.4, y - s * 0.3); ctx.bezierCurveTo(x - s * 0.1, y - s * 0.5, x - s * 0.1, y, x, y); ctx.bezierCurveTo(x + s * 0.1, y, x + s * 0.1, y + s * 0.5, x + s * 0.4, y + s * 0.3); break;
        case 5: for (let i = -1; i <= 1; i++) { ctx.moveTo(x + i * s * 0.26, y - s * 0.36); ctx.lineTo(x + i * s * 0.26, y + s * 0.36); } ctx.moveTo(x - s * 0.4, y); ctx.lineTo(x + s * 0.4, y); break;
        default: ctx.moveTo(x - s * 0.4, y - s * 0.4); ctx.lineTo(x + s * 0.4, y - s * 0.4); ctx.lineTo(x + s * 0.4, y); ctx.lineTo(x - s * 0.2, y); ctx.lineTo(x - s * 0.2, y + s * 0.3); ctx.lineTo(x + s * 0.2, y + s * 0.3); break;
    }
    ctx.stroke();
}

// Engraved gold stroke: dark groove below, bright core, warm glow.
function inlay(kit, pts, width = 0.0022, alpha = 0.9, closed = false) {
    const { ctx } = kit;
    ctx.save();
    ctx.lineCap = 'round'; ctx.lineJoin = 'round';
    kit.path(pts, closed);
    ctx.strokeStyle = 'rgba(0,0,0,.6)'; ctx.lineWidth = kit.m(width * 2.1); ctx.stroke();
    ctx.shadowColor = AMBER; ctx.shadowBlur = kit.m(0.007);
    kit.path(pts, closed);
    ctx.strokeStyle = GOLD_D; ctx.globalAlpha = alpha; ctx.lineWidth = kit.m(width * 1.2); ctx.stroke();
    ctx.shadowBlur = 0; ctx.strokeStyle = '#ffe3a0'; ctx.globalAlpha = alpha * 0.8; ctx.lineWidth = kit.m(width * 0.45); ctx.stroke();
    ctx.restore();
}

// The sun mandala: layered rings, 24 gold rays (long/short), a glyph ring and a face disc.
function mandala(kit) {
    const { ctx } = kit, c = kit.px(MAND.x, MAND.y), R = kit.m(0.205);
    ctx.save();
    ctx.beginPath(); ctx.rect(kit.px(-0.2575, 0.93)[0], kit.px(0, 0.93)[1], kit.W, kit.px(0, 0.20)[1] - kit.px(0, 0.93)[1]); ctx.clip();
    pool(kit, MAND.x, MAND.y, 0.26, 0.26, '#ffb347', 0.38);
    ctx.translate(c[0], c[1]);
    // rays
    for (let i = 0; i < 24; i++) {
        const a = i / 24 * Math.PI * 2 + Math.PI / 24, long = i % 2 === 0, len = R * (long ? 1.0 : 0.74), w = Math.PI / 24 * (long ? 0.62 : 0.5);
        const r0 = R * 0.52;
        ctx.beginPath();
        ctx.moveTo(Math.cos(a - w) * r0, Math.sin(a - w) * r0); ctx.lineTo(Math.cos(a) * len, Math.sin(a) * len); ctx.lineTo(Math.cos(a + w) * r0, Math.sin(a + w) * r0); ctx.closePath();
        const g = ctx.createLinearGradient(0, 0, Math.cos(a) * len, Math.sin(a) * len);
        g.addColorStop(0.45, 'rgba(170,110,28,.42)'); g.addColorStop(1, long ? 'rgba(255,210,110,.62)' : 'rgba(255,175,65,.46)');
        ctx.fillStyle = g; ctx.fill();
        ctx.strokeStyle = 'rgba(0,0,0,.45)'; ctx.lineWidth = Math.max(1, kit.m(0.0012)); ctx.stroke();
        ctx.strokeStyle = 'rgba(255,214,130,.55)'; ctx.lineWidth = Math.max(1, kit.m(0.0006)); ctx.stroke();
    }
    // rings
    const ring = (r, lw, col, dash = []) => { ctx.strokeStyle = col; ctx.lineWidth = kit.m(lw); ctx.setLineDash(dash.map(d => kit.m(d))); ctx.beginPath(); ctx.arc(0, 0, R * r, 0, Math.PI * 2); ctx.stroke(); ctx.setLineDash([]); };
    ring(0.50, 0.0042, 'rgba(0,0,0,.62)'); ring(0.50, 0.0020, 'rgba(255,200,100,.80)');
    ring(0.34, 0.0030, 'rgba(0,0,0,.55)'); ring(0.34, 0.0014, 'rgba(255,190,90,.7)');
    ring(0.42, 0.014, 'rgba(18,10,4,.55)');
    ring(0.60, 0.0010, 'rgba(255,200,100,.5)', [0.004, 0.006]); ring(0.96, 0.0010, 'rgba(255,200,100,.35)', [0.012, 0.005]);
    // glyph ring between 0.36 and 0.48
    ctx.strokeStyle = 'rgba(255,205,120,.78)'; ctx.lineWidth = Math.max(1, kit.m(0.0009)); ctx.lineCap = 'round'; ctx.lineJoin = 'round';
    const N = 28;
    for (let i = 0; i < N; i++) { const a = i / N * Math.PI * 2, gx = Math.cos(a) * R * 0.42, gy = Math.sin(a) * R * 0.42; ctx.save(); ctx.translate(gx, gy); ctx.rotate(a + Math.PI / 2); glyph(ctx, 0, 0, R * 0.075, i * 3 + (i >> 2)); ctx.restore(); }
    // face disc: embossed gold with a calm sun face
    const fr = R * 0.30, fg = ctx.createRadialGradient(-fr * 0.3, -fr * 0.3, 0, 0, 0, fr);
    fg.addColorStop(0, 'rgba(255,214,120,.62)'); fg.addColorStop(1, 'rgba(160,100,24,.50)');
    ctx.fillStyle = fg; ctx.beginPath(); ctx.arc(0, 0, fr, 0, Math.PI * 2); ctx.fill();
    ctx.strokeStyle = 'rgba(0,0,0,.5)'; ctx.lineWidth = kit.m(0.0016); ctx.stroke();
    ctx.strokeStyle = 'rgba(70,36,6,.8)'; ctx.lineWidth = kit.m(0.0020); ctx.lineCap = 'round';
    ctx.beginPath(); ctx.moveTo(-fr * 0.5, -fr * 0.12); ctx.quadraticCurveTo(-fr * 0.3, -fr * 0.26, -fr * 0.12, -fr * 0.12); ctx.moveTo(fr * 0.5, -fr * 0.12); ctx.quadraticCurveTo(fr * 0.3, -fr * 0.26, fr * 0.12, -fr * 0.12);
    ctx.moveTo(-fr * 0.04, -fr * 0.1); ctx.lineTo(-fr * 0.1, fr * 0.2); ctx.lineTo(fr * 0.06, fr * 0.2);
    ctx.moveTo(-fr * 0.3, fr * 0.46); ctx.quadraticCurveTo(0, fr * 0.6, fr * 0.3, fr * 0.46); ctx.stroke();
    ctx.restore();
}

// Gold inlay routed along the shot paths, lane guides and the idol channel; broken around inserts.
function inlayRoutes(kit) {
    const blocked = guard(kit.layout);
    // split a polyline into pieces that stay clear of inserts
    const clearPieces = (pts, extra = 0.004) => {
        const out = []; let cur = [];
        for (let i = 0; i < pts.length - 1; i++) {
            const a = pts[i], b = pts[i + 1], n = 14;
            for (let k = 0; k <= n; k++) {
                const p = [a[0] + (b[0] - a[0]) * k / n, a[1] + (b[1] - a[1]) * k / n];
                if (blocked(p[0], p[1], extra)) { if (cur.length > 1) out.push(cur); cur = []; } else cur.push(p);
            }
        }
        if (cur.length > 1) out.push(cur);
        return out;
    };
    for (const p of kit.layout.paths || []) {
        if (p.kind === 'tube' || !p.points) continue;
        const pts = p.points.map(q => [q[0], q[1]]), half = (p.width || 0.034) / 2 + 0.004;
        const side = s => pts.map((q, i) => { const a = pts[Math.max(0, i - 1)], b = pts[Math.min(pts.length - 1, i + 1)], dx = b[0] - a[0], dy = b[1] - a[1], l = Math.hypot(dx, dy) || 1; return [q[0] - dy / l * half * s, q[1] + dx / l * half * s]; });
        for (const s of [-1, 1]) for (const seg of clearPieces(side(s), 0.002)) inlay(kit, seg, 0.0012, 0.75);
    }
    // idol channel frame and chevron steps up to the idol
    inlay(kit, [[-0.032, 0.650], [-0.032, 0.775], [-0.016, 0.812], [0, 0.838], [0.016, 0.812], [0.032, 0.775], [0.032, 0.650]], 0.0016, 0.9);
    for (let i = 0; i < 4; i++) { const y = 0.715 + i * 0.016, w = 0.020 - i * 0.002; if (!blocked(0, y, 0.002)) inlay(kit, [[-w, y], [0, y + 0.007], [w, y]], 0.0010, 0.55); }
    // lane guides at the top
    for (const x of [-0.06, -0.02, 0.02, 0.06]) inlay(kit, [[x, 0.955], [x, 1.01]], 0.0012, 0.8);
    // lower playfield: long gold rails following the inlane guides and a lyre around the lower centre
    inlay(kit, [[-0.2475, 0.12], [-0.2475, 0.50]], 0.0012, 0.5);
    inlay(kit, [[0.2145, 0.12], [0.2145, 0.50]], 0.0012, 0.5);
    inlay(kit, [[-0.17, 0.215], [-0.10, 0.285], [0.10, 0.285], [0.17, 0.215]], 0.0014, 0.65);
}

// Greek-key meander band along a vertical or horizontal line segment (metres); skips cells that touch inserts.
function meander(kit, x0, y0, x1, y1, cell, alpha = 0.55) {
    const blocked = guard(kit.layout), { ctx } = kit;
    const L = Math.hypot(x1 - x0, y1 - y0), n = Math.floor(L / (cell * 2)), ux = (x1 - x0) / L, uy = (y1 - y0) / L, nx = -uy, ny = ux;
    ctx.save();
    ctx.lineCap = 'square'; ctx.lineJoin = 'miter';
    for (let i = 0; i < n; i++) {
        const o = [x0 + ux * cell * 2 * i, y0 + uy * cell * 2 * i];
        if (blocked(o[0] + ux * cell, o[1] + uy * cell, 0.001)) continue;
        const P = (a, b) => [o[0] + ux * cell * a + nx * cell * b, o[1] + uy * cell * a + ny * cell * b];
        const pts = [P(0, -0.8), P(0, 0.8), P(1.7, 0.8), P(1.7, -0.3), P(0.8, -0.3), P(0.8, 0.3), P(1.2, 0.3)];
        kit.path(pts, false); ctx.strokeStyle = 'rgba(0,0,0,.55)'; ctx.lineWidth = kit.m(cell * 0.45); ctx.stroke();
        kit.path(pts, false); ctx.strokeStyle = `rgba(255,203,101,${alpha})`; ctx.lineWidth = kit.m(cell * 0.2); ctx.stroke();
    }
    ctx.restore();
}

function glyphBorders(kit) {
    const { ctx } = kit, blocked = guard(kit.layout);
    // glyph row above the flippers' feed and below the SUN bank, framed by two gold lines
    const row = (y, x0, x1, seed) => {
        inlay(kit, [[x0, y + 0.012], [x1, y + 0.012]], 0.0009, 0.55); inlay(kit, [[x0, y - 0.012], [x1, y - 0.012]], 0.0009, 0.55);
        const rng = { seed }; ctx.save(); ctx.strokeStyle = 'rgba(255,205,120,.7)'; ctx.lineWidth = Math.max(1, kit.m(0.0008)); ctx.lineCap = 'round'; ctx.lineJoin = 'round';
        for (let x = x0 + 0.01; x < x1 - 0.005; x += 0.0215) { if (blocked(x, y, 0.001)) continue; const p = kit.px(x, y); glyph(ctx, p[0], p[1], kit.m(0.0145), Math.floor(rngNext(rng) * 7)); }
        ctx.restore();
    };
    row(0.375, -0.115, 0.115, 0x6A11);
    meander(kit, -0.2420, 0.12, -0.2420, 0.50, 0.0042, 0.4);
    meander(kit, 0.2170, 0.12, 0.2170, 0.50, 0.0042, 0.4);
    meander(kit, -0.18, 0.205, 0.18, 0.205, 0.0042, 0.45);
}

// Hanging jungle vines with leaves from the side walls, denser around the outlanes and the top corners.
function vines(kit) {
    const { ctx } = kit, rng = { seed: 0x71BE5 }, blocked = guard(kit.layout);
    const leaf = (x, y, ang, s, col) => {
        ctx.save(); ctx.translate(x, y); ctx.rotate(ang); ctx.fillStyle = col;
        ctx.beginPath(); ctx.moveTo(0, 0); ctx.quadraticCurveTo(s * 0.55, -s * 0.34, s, 0); ctx.quadraticCurveTo(s * 0.55, s * 0.34, 0, 0); ctx.fill();
        ctx.strokeStyle = 'rgba(0,0,0,.38)'; ctx.lineWidth = Math.max(1, s * 0.07); ctx.beginPath(); ctx.moveTo(0, 0); ctx.lineTo(s * 0.9, 0); ctx.stroke();
        ctx.restore();
    };
    const vine = (x, y, dirx, len, seg) => {
        const pts = [[x, y]]; let a = Math.atan2(-1, dirx * 0.6), px = x, py = y;
        for (let i = 0; i < seg; i++) { a += (rngNext(rng) - 0.5) * 0.9; px += Math.cos(a) * len / seg; py += Math.sin(a) * len / seg; pts.push([px, py]); }
        ctx.save(); ctx.lineCap = 'round'; ctx.lineJoin = 'round';
        kit.path(pts, false); ctx.strokeStyle = 'rgba(0,0,0,.45)'; ctx.lineWidth = kit.m(0.0034); ctx.stroke();
        kit.path(pts, false); ctx.strokeStyle = '#3a5a26'; ctx.lineWidth = kit.m(0.0017); ctx.stroke();
        ctx.restore();
        for (let i = 1; i < pts.length; i++) {
            if (blocked(pts[i][0], pts[i][1], 0)) continue;
            const q = kit.px(pts[i][0], pts[i][1]), sz = kit.m(0.0085 + rngNext(rng) * 0.006), base = Math.atan2(pts[i][1] - pts[i - 1][1], pts[i][0] - pts[i - 1][0]);
            for (const s of [-1, 1]) leaf(q[0], q[1], -base + s * (0.9 + rngNext(rng) * 0.6) + Math.PI * 0.0, sz, FOLIAGE[Math.floor(rngNext(rng) * 4)]);
            if (rngNext(rng) < 0.12) { ctx.fillStyle = '#e04a3a'; ctx.beginPath(); ctx.arc(q[0], q[1], kit.m(0.0022), 0, Math.PI * 2); ctx.fill(); }
        }
    };
    ctx.save();
    ctx.beginPath(); const a = kit.px(-0.2575, 0.93), b = kit.px(0.2575, 0.06); ctx.rect(a[0], a[1], b[0] - a[0], b[1] - a[1]); ctx.clip();
    for (let i = 0; i < 9; i++) { vine(-0.2475, 0.50 - i * 0.045, 1, 0.07 + rngNext(rng) * 0.05, 7); vine(0.2145, 0.50 - i * 0.045, -1, 0.07 + rngNext(rng) * 0.05, 7); }
    for (let i = 0; i < 5; i++) { vine(-0.2475, 0.90 - i * 0.055, 1, 0.05 + rngNext(rng) * 0.05, 6); vine(0.2145, 0.90 - i * 0.055, -1, 0.05 + rngNext(rng) * 0.05, 6); }
    ctx.restore();
}

function logo(kit) {
    const { ctx } = kit, p = kit.px(0, 0.262);
    ctx.save();
    ctx.translate(p[0], p[1]);
    ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
    const fit = (str, px, w) => { ctx.font = `900 ${px}px ${FONT_STACK}`; const m = ctx.measureText(str).width; return m > w ? px * w / m : px; };
    if ('letterSpacing' in ctx) ctx.letterSpacing = `${kit.m(0.0026)}px`;
    const s1 = fit('LOST TEMPLE', kit.m(0.032), kit.m(0.20)), s2 = fit('OF THE SUN', kit.m(0.0125), kit.m(0.12));
    ctx.font = `900 ${s1}px ${FONT_STACK}`;
    ctx.fillStyle = 'rgba(0,0,0,.7)'; ctx.fillText('LOST TEMPLE', kit.m(0.0012), -kit.m(0.0030) + kit.m(0.0020));
    const g = ctx.createLinearGradient(0, -kit.m(0.02), 0, kit.m(0.01));
    g.addColorStop(0, '#fff0b8'); g.addColorStop(0.45, '#ffc95a'); g.addColorStop(1, '#a8681a');
    ctx.shadowColor = AMBER; ctx.shadowBlur = kit.m(0.010); ctx.fillStyle = g; ctx.fillText('LOST TEMPLE', 0, -kit.m(0.0030));
    ctx.shadowBlur = 0; ctx.lineWidth = kit.m(0.0007); ctx.strokeStyle = 'rgba(60,28,4,.85)'; ctx.strokeText('LOST TEMPLE', 0, -kit.m(0.0030));
    ctx.font = `800 ${s2}px ${FONT_STACK}`; ctx.fillStyle = 'rgba(138,214,163,.92)'; ctx.fillText('OF THE SUN', 0, kit.m(0.0150));
    ctx.restore();
    inlay(kit, [[-0.105, 0.2385], [-0.02, 0.2385]], 0.0010, 0.7); inlay(kit, [[0.02, 0.2385], [0.105, 0.2385]], 0.0010, 0.7);
    const d = kit.px(0, 0.2385), r = kit.m(0.005);
    ctx.save(); ctx.fillStyle = GOLD; ctx.shadowColor = AMBER; ctx.shadowBlur = kit.m(0.006);
    ctx.beginPath(); ctx.moveTo(d[0], d[1] - r); ctx.lineTo(d[0] + r, d[1]); ctx.lineTo(d[0], d[1] + r); ctx.lineTo(d[0] - r, d[1]); ctx.closePath(); ctx.fill(); ctx.restore();
}

export function paintArt(kit) {
    const { ctx } = kit;
    stoneWash(kit);
    flagstones(kit);
    pool(kit, 0, 0.78, 0.14, 0.12, '#ffb347', 0.28);   // idol firelight
    mandala(kit);
    inlayRoutes(kit);
    glyphBorders(kit);
    vines(kit);
    pool(kit, -0.156, 0.74, 0.05, 0.12, '#ffcb65', 0.12);
    pool(kit, 0.13, 0.76, 0.05, 0.12, '#8ad6a3', 0.08);
    pool(kit, 0, 0.30, 0.17, 0.12, '#ffb347', 0.16);
    pool(kit, 0, 0.14, 0.20, 0.13, '#ffb347', 0.22);
    pool(kit, -0.20, 0.30, 0.06, 0.18, '#8ad6a3', 0.08); pool(kit, 0.18, 0.30, 0.06, 0.18, '#8ad6a3', 0.08);
    logo(kit);
    ctx.globalCompositeOperation = 'source-over';
}

export default paintArt;
