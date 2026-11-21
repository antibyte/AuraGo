// AuraPinball playfield art kit (plan section 7): canvas painter with the metre -> pixel mapping shared with the lamp mask.
// createArtKit(ctx2d, {W, H, layout, tr, palette, underlay}) -> kit. Row 0 of the canvas is the top (CanvasTexture.flipY).
import { FONT_STACK, rngNext } from '../config.js';
import { FIELD_W, canvasPx, lampPolygon } from '../lights/shapes.js';

export function hexRgb(hex, fallback = [255, 255, 255]) {
    const m = /^#?([0-9a-f]{6})$/i.exec(String(hex || '').trim());
    if (!m) return fallback;
    const n = parseInt(m[1], 16);
    return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}
export const rgbCss = (c, k = 1, a = 1) => `rgba(${Math.min(255, Math.round(c[0] * k))},${Math.min(255, Math.round(c[1] * k))},${Math.min(255, Math.round(c[2] * k))},${a})`;

// Translation helper: tr may be missing or return the key; fall back to a readable upper-case word.
export function trText(tr, key, vars, fallback) {
    let out = null;
    try { out = typeof tr === 'function' ? tr(key, vars) : null; } catch { out = null; }
    if (out && out !== key && !String(out).startsWith('pinball.')) return String(out);
    return fallback !== undefined ? fallback : String(key).replace(/^[a-z0-9]+_/, '').replace(/_/g, ' ').toUpperCase();
}

const WALL_W = { wood: 0.010, metal: 0.0036, rubber: 0.008, plastic: 0.006 };

export function createArtKit(ctx, { W, H, layout, tr, palette = {}, underlay = null, artRev = 1 } = {}) {
    const S = W / FIELD_W;
    const accent = hexRgb(palette.accent || (layout && layout.meta && layout.meta.accent), [55, 217, 255]);
    const secondary = hexRgb(palette.secondary || (layout && layout.meta && layout.meta.secondary), [255, 72, 196]);
    const px = (x, y) => canvasPx(x, y, W, H);
    const m = v => v * S;
    const kit = { ctx, W, H, S, px, m, layout, tr, artRev: Number(artRev) || 1, accent, secondary, palette: { ...palette, accentRgb: accent, secondaryRgb: secondary } };

    function path(points, closed) {
        ctx.beginPath();
        points.forEach((p, i) => { const q = px(p[0], p[1]); if (i) ctx.lineTo(q[0], q[1]); else ctx.moveTo(q[0], q[1]); });
        if (closed) ctx.closePath();
    }
    kit.path = path;

    kit.underlay = (img, { brightness = 0.42, saturate = 0.9 } = {}) => {
        ctx.save();
        const g = ctx.createLinearGradient(0, 0, 0, H);
        g.addColorStop(0, rgbCss(accent, 0.1)); g.addColorStop(1, '#05070b');
        ctx.fillStyle = g; ctx.fillRect(0, 0, W, H);
        if (img) {
            if ('filter' in ctx) ctx.filter = `saturate(${saturate})`;
            ctx.drawImage(img, 0, 0, W, H);
            ctx.filter = 'none';
            ctx.fillStyle = `rgba(0,0,0,${1 - brightness})`; ctx.fillRect(0, 0, W, H);
        }
        ctx.restore();
    };

    kit.glowLine = (points, color, width = 0.0035, glow = 0.012, alpha = 1) => {
        ctx.save();
        ctx.lineCap = 'round'; ctx.lineJoin = 'round';
        ctx.shadowColor = color; ctx.shadowBlur = m(glow); ctx.strokeStyle = color; ctx.globalAlpha = alpha;
        ctx.lineWidth = m(width); path(points, false); ctx.stroke();
        ctx.shadowBlur = 0; ctx.lineWidth = m(width) * 0.35; ctx.strokeStyle = '#ffffff'; ctx.globalAlpha = alpha * 0.55; path(points, false); ctx.stroke();
        ctx.restore();
    };

    // size in metres (cap height), angle in radians (0 = upright); fits maxWidth (metres) by shrinking.
    kit.text = (str, x, y, { size = 0.012, color = '#fff', align = 'center', angle = 0, weight = 'bold', maxWidth = 0, alpha = 1 } = {}) => {
        ctx.save();
        const p = px(x, y);
        ctx.translate(p[0], p[1]); ctx.rotate(-angle);
        let fs = m(size);
        ctx.font = `${weight} ${fs}px ${FONT_STACK}`;
        if (maxWidth) { const w = ctx.measureText(str).width, lim = m(maxWidth); if (w > lim) { fs *= lim / w; ctx.font = `${weight} ${fs}px ${FONT_STACK}`; } }
        ctx.textAlign = align; ctx.textBaseline = 'middle'; ctx.fillStyle = color; ctx.globalAlpha = alpha;
        ctx.fillText(str, 0, 0);
        ctx.restore();
    };

    kit.arrow = (x, y, angle, size, color, alpha = 1) => {
        ctx.save();
        const p = px(x, y), s = m(size);
        ctx.translate(p[0], p[1]); ctx.rotate(-angle); ctx.globalAlpha = alpha; ctx.fillStyle = color;
        ctx.beginPath(); ctx.moveTo(0, -s); ctx.lineTo(s * 0.7, 0); ctx.lineTo(s * 0.28, 0); ctx.lineTo(s * 0.28, s * 0.8);
        ctx.lineTo(-s * 0.28, s * 0.8); ctx.lineTo(-s * 0.28, 0); ctx.lineTo(-s * 0.7, 0); ctx.closePath(); ctx.fill();
        ctx.restore();
    };

    let noise = null;
    kit.grain = (amount = 0.05) => {
        if (!noise && typeof document !== 'undefined') {
            noise = document.createElement('canvas'); noise.width = noise.height = 256;
            const g = noise.getContext('2d'), img = g.createImageData(256, 256), holder = { seed: 0xA11CE };
            for (let i = 0; i < img.data.length; i += 4) { const v = 90 + Math.floor(rngNext(holder) * 150); img.data[i] = img.data[i + 1] = img.data[i + 2] = v; img.data[i + 3] = 255; }
            g.putImageData(img, 0, 0);
        }
        if (!noise) return;
        ctx.save(); ctx.globalAlpha = amount; ctx.globalCompositeOperation = 'soft-light';
        ctx.fillStyle = ctx.createPattern(noise, 'repeat'); ctx.fillRect(0, 0, W, H);
        ctx.restore();
    };

    kit.vignette = (strength = 0.5) => {
        ctx.save();
        ctx.translate(W / 2, H / 2); ctx.scale(1, H / W);
        const r = Math.hypot(W / 2, W / 2), g = ctx.createRadialGradient(0, 0, r * 0.45, 0, 0, r * 1.02);
        g.addColorStop(0, 'rgba(0,0,0,0)'); g.addColorStop(1, `rgba(0,0,0,${strength})`);
        ctx.fillStyle = g; ctx.fillRect(-W / 2, -W / 2, W, W);
        ctx.restore();
    };

    kit.lanes = () => {
        const pl = layout && layout.plunger;
        ctx.save();
        if (pl && pl.laneX) {
            const a = px(pl.laneX[0], 0.78), b = px(pl.laneX[1], 0);
            const g = ctx.createLinearGradient(a[0], 0, b[0], 0);
            g.addColorStop(0, 'rgba(6,8,12,.76)'); g.addColorStop(0.5, 'rgba(28,36,48,.62)'); g.addColorStop(1, 'rgba(6,8,12,.76)');
            ctx.fillStyle = g; ctx.fillRect(a[0], a[1], b[0] - a[0], b[1] - a[1]);
            ctx.strokeStyle = 'rgba(190,205,225,.14)'; ctx.lineWidth = Math.max(1, m(0.0004));
            for (let i = 0; i < 6; i++) { const x = a[0] + (b[0] - a[0]) * (0.15 + i * 0.14); ctx.beginPath(); ctx.moveTo(x, a[1]); ctx.lineTo(x, b[1]); ctx.stroke(); }
            // Plunger calibration scale ticks near the shooter rest zone.
            const yRest = pl.yRest || 0.0535, travel = pl.travel || 0.06;
            for (let step = 0; step <= 10; step++) {
                const y = yRest + 0.015 + (1 - step / 10) * travel * 1.4, major = step % 5 === 0;
                const pL = px(pl.laneX[0] + 0.002, y), pR = px(pl.laneX[0] + (major ? 0.0085 : 0.0052), y);
                ctx.strokeStyle = major ? rgbCss(accent, 1.1, 0.78) : 'rgba(210,225,245,.38)';
                ctx.lineWidth = Math.max(1, m(major ? 0.0008 : 0.00045));
                ctx.beginPath(); ctx.moveTo(pL[0], pL[1]); ctx.lineTo(pR[0], pR[1]); ctx.stroke();
            }
            for (let y = 0.12; y < 0.72; y += 0.11) kit.arrow((pl.laneX[0] + pl.laneX[1]) / 2, y, 0, 0.0075, rgbCss(accent, 1, 0.55));
        }
        for (const d of (layout && layout.devices) || []) {
            if (d.type === 'rollover') {
                const a = px(d.x - 0.0125, d.y + 0.045), b = px(d.x + 0.0125, d.y - 0.045), r = m(0.0125);
                ctx.fillStyle = 'rgba(210,225,255,.06)'; ctx.strokeStyle = rgbCss(accent, 1, 0.32); ctx.lineWidth = Math.max(1, m(0.0006));
                ctx.beginPath(); ctx.roundRect(a[0], a[1], b[0] - a[0], b[1] - a[1], r); ctx.fill(); ctx.stroke();
            } else if (d.type === 'hole' && !d.hidden) {
                // Brushed metallic scoop wear-plate & mounting rivets around visible scoops / VUKs.
                const c = px(d.x, d.y), r = m((d.r || 0.017) * 1.48);
                const gr = ctx.createRadialGradient(c[0], c[1], r * 0.5, c[0], c[1], r);
                gr.addColorStop(0, 'rgba(18,22,30,.85)'); gr.addColorStop(0.75, 'rgba(135,150,170,.36)'); gr.addColorStop(1, 'rgba(10,14,20,0)');
                ctx.fillStyle = gr; ctx.beginPath(); ctx.arc(c[0], c[1], r, 0, Math.PI * 2); ctx.fill();
                ctx.strokeStyle = rgbCss(accent, 1, 0.42); ctx.lineWidth = Math.max(1, m(0.0007));
                ctx.beginPath(); ctx.arc(c[0], c[1], r * 0.88, 0, Math.PI * 2); ctx.stroke();
                ctx.fillStyle = 'rgba(220,232,248,.55)';
                for (let k = 0; k < 6; k++) {
                    const ang = (k / 6) * Math.PI * 2;
                    ctx.beginPath(); ctx.arc(c[0] + Math.cos(ang) * r * 0.78, c[1] + Math.sin(ang) * r * 0.78, Math.max(1, m(0.0011)), 0, Math.PI * 2); ctx.fill();
                }
            }
        }
        ctx.restore();
    };

    kit.wallShadows = () => {
        const walls = ((layout && layout.walls) || []).filter(w => w.render && w.render !== 'none' && w.points && w.points.length > 1);
        ctx.save();
        ctx.lineCap = 'round'; ctx.lineJoin = 'round';
        ctx.shadowColor = 'rgba(0,0,0,.8)'; ctx.shadowBlur = m(0.011); ctx.shadowOffsetX = m(0.004); ctx.shadowOffsetY = m(0.0055);
        for (const kind of ['plastic', 'rubber', 'metal', 'wood']) {
            ctx.strokeStyle = '#000'; ctx.lineWidth = m(WALL_W[kind]);
            ctx.beginPath();
            for (const w of walls) if (w.render === kind) w.points.forEach((p, i) => { const q = px(p[0], p[1]); if (i) ctx.lineTo(q[0], q[1]); else ctx.moveTo(q[0], q[1]); });
            ctx.stroke();
        }
        ctx.restore();
    };

    // Unlit insert: translucent plastic at colour x 0.3 with bevel rim, specular streak and dark label ink.
    kit.inserts = () => {
        const lamps = (layout && layout.lamps) || [];
        for (const lamp of lamps) {
            if (lamp.mount && lamp.mount !== 'insert') continue;
            const poly = lampPolygon(lamp, 28).map(p => px(p[0], p[1])), c = hexRgb(lamp.color);
            const cxp = px(lamp.x, lamp.y), rad = m(Math.max(lamp.w || 0.016, lamp.h || 0.016) * 0.6);
            const outline = () => { ctx.beginPath(); poly.forEach((p, i) => (i ? ctx.lineTo(p[0], p[1]) : ctx.moveTo(p[0], p[1]))); ctx.closePath(); };
            ctx.save();
            ctx.lineJoin = 'round';
            outline(); ctx.strokeStyle = '#020305'; ctx.lineWidth = m(0.0042); ctx.stroke();          // recess bezel
            outline(); ctx.strokeStyle = 'rgba(205,215,230,.30)'; ctx.lineWidth = m(0.0017); ctx.stroke(); // chrome-ish eyelet edge
            const g = ctx.createRadialGradient(cxp[0], cxp[1], 0, cxp[0], cxp[1], rad);
            g.addColorStop(0, rgbCss(c, 0.5)); g.addColorStop(1, rgbCss(c, 0.24));
            outline(); ctx.fillStyle = g; ctx.fill();
            ctx.save(); outline(); ctx.clip();
            const bb = poly.reduce((a, p) => [Math.min(a[0], p[0]), Math.min(a[1], p[1]), Math.max(a[2], p[0]), Math.max(a[3], p[1])], [1e9, 1e9, -1e9, -1e9]);
            const sg = ctx.createLinearGradient(0, bb[1], 0, bb[3]);
            sg.addColorStop(0, 'rgba(255,255,255,.20)'); sg.addColorStop(0.45, 'rgba(255,255,255,0)'); sg.addColorStop(1, 'rgba(0,0,0,.18)');
            ctx.fillStyle = sg; ctx.fillRect(bb[0], bb[1], bb[2] - bb[0], bb[3] - bb[1]);
            ctx.restore();
            outline(); ctx.strokeStyle = rgbCss(c, 0.9, 0.55); ctx.lineWidth = m(0.0005); ctx.stroke();
            ctx.restore();
            const label = lamp.label ? trText(tr, lamp.label) : lamp.text;
            if (label) {
                const w = lamp.w || 0.016, h = lamp.h || w, lines = String(label).split('\n');
                const size = Math.min(h * 0.42 / Math.max(1, lines.length * 0.9), w * 0.62);
                lines.forEach((ln, i) => kit.text(ln, lamp.x, lamp.y + ((lines.length - 1) / 2 - i) * size * 1.15, { size, color: 'rgba(3,5,8,.92)', maxWidth: w * 0.86 }));
            }
        }
    };

    kit.labels = () => {
        for (const l of (layout && layout.labels) || []) {
            const c = hexRgb(l.color, accent);
            kit.text(trText(tr, l.key, undefined, l.text || l.key), l.x, l.y, { size: l.size || 0.012, color: rgbCss(c, 1, 0.9), angle: l.angle || 0 });
        }
    };

    kit.rolloverStars = () => {
        const lamps = (layout && layout.lamps) || [];
        for (const d of (layout && layout.devices) || []) {
            if (d.type !== 'rollover') continue;
            if (lamps.some(l => (!l.mount || l.mount === 'insert') && Math.hypot(l.x - d.x, l.y - d.y) < 0.012)) continue;
            const star = lampPolygon({ x: d.x, y: d.y + 0.004, shape: 'star', w: 0.016, h: 0.016, angle: 0 }, 10).map(p => px(p[0], p[1]));
            ctx.save();
            ctx.beginPath(); star.forEach((p, i) => (i ? ctx.lineTo(p[0], p[1]) : ctx.moveTo(p[0], p[1]))); ctx.closePath();
            ctx.fillStyle = rgbCss(accent, 0.3); ctx.strokeStyle = 'rgba(0,0,0,.85)'; ctx.lineWidth = m(0.0012);
            ctx.stroke(); ctx.fill();
            ctx.restore();
        }
    };

    // Soft drop shadows under hardware so it sits on the print (offset +x, toward the player; deterministic, no random).
    kit.contactShadows = () => {
        const L = layout || {}, OX = 0.0025, OY = -0.002;      // metres; playfield y points up, so "down the canvas" is -y
        ctx.save();
        const blob = (x, y, rx, ry, rot, a) => {
            const c = px(x + OX, y + OY), r = m(Math.max(rx, ry));
            ctx.save(); ctx.translate(c[0], c[1]); ctx.rotate(-rot); ctx.scale(rx / Math.max(rx, ry), ry / Math.max(rx, ry));
            const g = ctx.createRadialGradient(0, 0, r * 0.35, 0, 0, r);
            g.addColorStop(0, `rgba(0,0,0,${a})`); g.addColorStop(1, 'rgba(0,0,0,0)');
            ctx.fillStyle = g; ctx.beginPath(); ctx.arc(0, 0, r, 0, Math.PI * 2); ctx.fill(); ctx.restore();
        };
        for (const p of L.posts || []) if (p.render !== 'none' && p.r) blob(p.x, p.y, (p.r + 0.004) * 1.5, (p.r + 0.004) * 1.5, 0, 0.42);
        for (const d of L.devices || []) {
            if (d.type === 'bumper') { const r = (d.r ?? 0.0215) * 1.35; blob(d.x, d.y, r * 1.45, r * 1.45, 0, 0.5); }
            else if (d.type === 'standup') blob(d.x, d.y, (d.width || 0.02) * 0.85, 0.009, d.angle || 0, 0.4);
            else if (d.type === 'dropBank') for (const t of d.targets || []) blob(t.x, t.y, (d.width || 0.022) * 0.85, 0.009, d.angle || 0, 0.4);
        }
        // sling bases and wall feet: a shadow stroke under each wall that stands on the field
        ctx.lineCap = 'round'; ctx.lineJoin = 'round';
        const walls = (L.walls || []).filter(w => w.render && w.render !== 'none' && w.points && w.points.length > 1);
        if ('filter' in ctx) ctx.filter = 'blur(' + Math.max(1, m(0.0035)).toFixed(1) + 'px)';
        for (const w of walls) {
            if (w.render === 'wood') continue;
            ctx.strokeStyle = w.render === 'rubber' ? 'rgba(0,0,0,.45)' : 'rgba(0,0,0,.36)';
            ctx.lineWidth = m((WALL_W[w.render] || 0.006) + 0.004);
            ctx.beginPath();
            w.points.forEach((p, i) => { const q = px(p[0] + OX, p[1] + OY); if (i) ctx.lineTo(q[0], q[1]); else ctx.moveTo(q[0], q[1]); });
            if (w.closed) ctx.closePath();
            ctx.stroke();
        }
        ctx.filter = 'none';
        ctx.restore();
    };

    // Full layer stack. `paint` is the table's paintArt hook (theme decoration), drawn above the underlay.
    kit.autoArt = ({ paint = null, brightness = 0.42, saturate = 0.9 } = {}) => {
        const rev2 = kit.artRev >= 2;
        kit.underlay(underlay, { brightness, saturate });
        if (paint) { ctx.save(); try { paint(kit); } finally { ctx.restore(); } }
        if (rev2) kit.contactShadows();
        kit.lanes(); kit.wallShadows(); kit.inserts(); kit.labels(); kit.rolloverStars();
        kit.vignette(rev2 ? 0.22 : 0.5); kit.grain(rev2 ? 0.035 : 0.07);
    };
    return kit;
}
