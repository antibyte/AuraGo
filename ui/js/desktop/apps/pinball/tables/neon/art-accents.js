// Neon rev 2 (painted underlay): functional accents only. The painting already holds the city, plazas and lanes, so this adds
// soft glow lines leading from each shot arrow into its lane and a glow at every ramp/tube entry. No washes, scenery or logo.
import { hexRgb, rgbCss } from '../../render/art-kit.js';

export function paintAccents(kit) {
    const { ctx, layout } = kit;
    ctx.save();
    ctx.globalCompositeOperation = 'lighter';
    for (const l of layout.lamps || []) {
        if (!/^arrow_/.test(l.id || '') || (l.mount && l.mount !== 'insert')) continue;
        const col = l.color || '#37d9ff', dx = -Math.sin(l.angle || 0), dy = Math.cos(l.angle || 0);
        kit.glowLine([[l.x + dx * 0.014, l.y + dy * 0.014], [l.x + dx * 0.062, l.y + dy * 0.062]], col, 0.0022, 0.010, 0.30);
    }
    for (const p of layout.paths || []) {
        if (!p.points || p.points.length < 2) continue;
        const c = hexRgb(p.color || '#37d9ff'), e = kit.px(p.points[0][0], p.points[0][1]), r = kit.m(0.032);
        const g = ctx.createRadialGradient(e[0], e[1], 0, e[0], e[1], r);
        g.addColorStop(0, rgbCss(c, 1, 0.30)); g.addColorStop(1, rgbCss(c, 1, 0));
        ctx.fillStyle = g; ctx.beginPath(); ctx.arc(e[0], e[1], r, 0, Math.PI * 2); ctx.fill();
    }
    ctx.restore();
}

export default paintAccents;
