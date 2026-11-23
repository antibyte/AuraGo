/* Runenpfad props art: painted chains, rails, conveyor belts, projectile sprites and the rune warning ring from the
   optional atlas, each with a primitive fallback. Canvas-facing and read-only: never mutates simulation state. */
(function (root) {
    'use strict';

    const TAU = Math.PI * 2;
    const O = { height: 0, scale: 0, flip: false, alpha: 1, composite: '', rotation: 0, frame: undefined, tint: '', tintAmount: 0.55 };
    const LINK_H = 167;
    const LINK_OVERLAP = 0.92;
    const CONVEYOR_H = 24;

    // The decoded atlas when it holds the clip, otherwise null (missing or still loading).
    function atlasFor(clip) {
        const A = root.RunenpfadArt;
        const at = A && typeof A.current === 'function' ? A.current() : null;
        return at && A.has(at, clip) ? at : null;
    }

    // One atlas draw. `scale` converts source pixels to screen pixels (uniform); rotation is applied before the flip.
    function put(ctx, at, clip, time, x, y, scale, rotation, flip, alpha, comp, tint, frame) {
        O.scale = scale;
        O.height = 0;
        O.rotation = rotation || 0;
        O.flip = !!flip;
        O.alpha = alpha == null ? 1 : alpha;
        O.composite = comp || '';
        O.tint = tint || '';
        O.frame = frame;
        return root.RunenpfadArt.draw(ctx, at, clip, time, x, y, O);
    }

    function rgba(color, a) {
        if (color && color.charAt(0) === '#' && color.length >= 7) return 'rgba(' + parseInt(color.slice(1, 3), 16) + ',' + parseInt(color.slice(3, 5), 16) + ',' + parseInt(color.slice(5, 7), 16) + ',' + a + ')';
        return 'rgba(126, 232, 250, ' + a + ')';
    }

    // ---- chain: link sprites along a segment (pendulum arms) ----
    function drawChain(ctx, x1, y1, x2, y2, opts) {
        const o = opts || {};
        const full = Math.hypot(x2 - x1, y2 - y1);
        if (full < 2) return;
        const ux = (x2 - x1) / full;
        const uy = (y2 - y1) / full;
        // optional trim keeps the end of the chain at the edge of whatever hangs on it
        const len = full - Math.min(o.trim || 0, full - 2);
        x2 = x1 + ux * len;
        y2 = y1 + uy * len;
        const dy = y2 - y1;
        const dx = x2 - x1;
        const ang = Math.atan2(dy, dx);
        const at = atlasFor('prop-chain.link');
        const pad = o.shackle ? 12 : 0;
        const span = Math.max(1, len - pad);
        if (at) {
            const n = Math.max(1, Math.round(span / ((o.linkLen || 37) * LINK_OVERLAP)));
            const step = span / n;
            const sc = step / LINK_OVERLAP / LINK_H;
            for (let i = 0; i < n; i++) {
                const d = (i + 0.5) * step;
                put(ctx, at, 'prop-chain.link', 0, x1 + ux * d, y1 + uy * d, sc, ang - Math.PI / 2, false, 1);
            }
            if (o.shackle && atlasFor('prop-chain.shackle')) put(ctx, at, 'prop-chain.shackle', 0, x2 - ux * 9, y2 - uy * 9, sc * 0.82, ang - Math.PI / 2, false, 1);
            return;
        }
        const n = Math.max(1, Math.round(len / 9));
        const step = len / n;
        ctx.save();
        ctx.lineWidth = 1.2;
        for (let i = 0; i < n; i++) {
            const d = (i + 0.5) * step;
            const odd = i % 2 === 1;
            ctx.fillStyle = odd ? '#4b5366' : '#8b93a7';
            ctx.strokeStyle = '#2a2f3d';
            ctx.beginPath();
            ctx.ellipse(x1 + ux * d, y1 + uy * d, step * (odd ? 0.5 : 0.66), odd ? 2 : 3.4, ang, 0, TAU);
            ctx.fill();
            ctx.stroke();
        }
        ctx.restore();
    }

    // ---- rail: bronze strip tiled along a segment (moving platform tracks) ----
    function drawRail(ctx, x1, y1, x2, y2, alpha) {
        const dx = x2 - x1;
        const dy = y2 - y1;
        const len = Math.hypot(dx, dy);
        if (len < 2 || !(alpha > 0.01)) return;
        const ux = dx / len;
        const uy = dy / len;
        const ang = Math.atan2(dy, dx);
        const n = Math.max(1, Math.round(len / 58));
        const step = len / n;
        const at = atlasFor('prop-chain.rail');
        if (at) {
            const sc = step * 1.04 / 149;
            for (let i = 0; i < n; i++) {
                const d = (i + 0.5) * step;
                put(ctx, at, 'prop-chain.rail', 0, x1 + ux * d, y1 + uy * d, sc, ang, false, alpha);
            }
            return;
        }
        ctx.save();
        ctx.globalAlpha = alpha;
        ctx.lineCap = 'round';
        ctx.strokeStyle = '#2a2f3d';
        ctx.lineWidth = 5;
        ctx.beginPath();
        ctx.moveTo(x1, y1);
        ctx.lineTo(x2, y2);
        ctx.stroke();
        ctx.strokeStyle = '#8b93a7';
        ctx.lineWidth = 2;
        ctx.stroke();
        ctx.restore();
    }

    // ---- conveyor: painted end caps and belt pieces plus a moving tread drawn in code ----
    function drawTread(ctx, run, nowSec, reduced, y0, y1) {
        const dir = run.speed >= 0 ? 1 : -1;
        const off = reduced ? 0 : (nowSec * Math.abs(run.speed) * 0.5) % 16;
        const mid = (y0 + y1) / 2;
        const hh = (y1 - y0) / 2;
        ctx.strokeStyle = dir > 0 ? 'rgba(253, 224, 71, 0.9)' : 'rgba(125, 211, 252, 0.9)';
        ctx.lineWidth = 2.5;
        for (let x = run.x - 16 + off * dir + (dir < 0 ? 16 : 0); x < run.x + run.w + 16; x += 16) {
            ctx.beginPath();
            ctx.moveTo(x - 4 * dir, y0);
            ctx.lineTo(x + 3 * dir, mid);
            ctx.lineTo(x - 4 * dir, y0 + hh * 2);
            ctx.stroke();
        }
    }

    function drawConveyor(ctx, run, nowSec, reduced) {
        const at = atlasFor('prop-conveyor.mid') && atlasFor('prop-conveyor.left') && atlasFor('prop-conveyor.right');
        ctx.save();
        ctx.beginPath();
        if (at) {
            const sc = CONVEYOR_H / 74;
            const pw = 224 * sc;
            const cy = run.y - 1 + CONVEYOR_H / 2;
            ctx.rect(run.x, run.y - 3, run.w, CONVEYOR_H + 4);
            ctx.clip();
            if (run.w >= pw * 2) {
                for (let x = run.x + pw; x < run.x + run.w - pw; x += pw) put(ctx, at, 'prop-conveyor.mid', 0, x + pw / 2, cy, sc, 0, false, 1);
                put(ctx, at, 'prop-conveyor.left', 0, run.x + pw / 2, cy, sc, 0, false, 1);
                put(ctx, at, 'prop-conveyor.right', 0, run.x + run.w - pw / 2, cy, sc, 0, false, 1);
            } else {
                // a run shorter than two pieces: each cap keeps its own half of the run
                ctx.restore();
                ctx.save();
                ctx.beginPath();
                ctx.rect(run.x, run.y - 3, run.w / 2, CONVEYOR_H + 4);
                ctx.clip();
                put(ctx, at, 'prop-conveyor.left', 0, run.x + pw / 2, cy, sc, 0, false, 1);
                ctx.restore();
                ctx.save();
                ctx.beginPath();
                ctx.rect(run.x + run.w / 2, run.y - 3, run.w / 2, CONVEYOR_H + 4);
                ctx.clip();
                put(ctx, at, 'prop-conveyor.right', 0, run.x + run.w - pw / 2, cy, sc, 0, false, 1);
                ctx.restore();
                ctx.save();
                ctx.beginPath();
            }
            ctx.beginPath();
            ctx.rect(run.x, run.y, run.w, 8);
            ctx.clip();
            drawTread(ctx, run, nowSec, reduced, run.y + 1, run.y + 6);
        } else {
            ctx.rect(run.x, run.y - 2, run.w, 14);
            ctx.clip();
            ctx.fillStyle = 'rgba(20, 24, 34, 0.55)';
            ctx.fillRect(run.x, run.y - 2, run.w, 12);
            drawTread(ctx, run, nowSec, reduced, run.y, run.y + 8);
        }
        ctx.restore();
    }

    // ---- warning ring on the ground: painted rune ring stretched to the ellipse, steady under reduced motion ----
    function drawTelegraphRing(ctx, cx, cy, rx, ry, t, color, reduced) {
        const k = reduced ? 1 : 0.5 + 0.5 * Math.sin(t * 18);
        const at = atlasFor('fx-telegraph.ring');
        ctx.save();
        // area fill: radial gradient from a faint centre to a strong rim, so the landing zone reads on bright ground
        ctx.translate(cx, cy);
        ctx.scale(1, ry / rx);
        const g = ctx.createRadialGradient(0, 0, 0, 0, 0, rx);
        g.addColorStop(0, rgba(color, 0.1));
        g.addColorStop(1, rgba(color, 0.26 + 0.14 * k));
        ctx.fillStyle = g;
        ctx.beginPath();
        ctx.arc(0, 0, rx, 0, TAU);
        ctx.fill();
        ctx.restore();
        ctx.save();
        // solid outline: dark under, bright on top
        ctx.lineJoin = 'round';
        ctx.beginPath();
        ctx.ellipse(cx, cy, rx, ry, 0, 0, TAU);
        ctx.globalAlpha = 0.85;
        ctx.strokeStyle = '#450a0a';
        ctx.lineWidth = 5;
        ctx.stroke();
        ctx.globalAlpha = 0.9;
        ctx.strokeStyle = '#ffd23f';
        ctx.lineWidth = 2.5;
        ctx.stroke();
        ctx.globalAlpha = 1;
        if (at) {
            // painted runes just inside the outline; small rings keep a minimum scale and are clipped to the ellipse
            ctx.beginPath();
            ctx.ellipse(cx, cy, rx, ry, 0, 0, TAU);
            ctx.clip();
            ctx.translate(cx, cy);
            ctx.scale(1, (ry / rx) / (124 / 325));
            const sc = Math.max(0.4, 2 * rx * 0.9 / 325);
            const tint = color === '#e11d1d' ? '' : color;
            const frame = reduced ? 0 : undefined;
            put(ctx, at, 'fx-telegraph.ring', t, 0, 0, sc, 0, false, 1, '', tint, frame);
            put(ctx, at, 'fx-telegraph.ring', t, 0, 0, sc, 0, false, 0.45 + 0.4 * k, 'lighter', tint, frame);
        } else {
            ctx.strokeStyle = color;
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.ellipse(cx, cy, rx * 0.62, ry * 0.62, 0, 0, TAU);
            ctx.stroke();
            const a0 = reduced ? 0 : t * 0.8;
            ctx.strokeStyle = '#ffd23f';
            ctx.beginPath();
            for (let i = 0; i < 10; i++) {
                const a = a0 + i * TAU / 10;
                ctx.moveTo(cx + Math.cos(a) * rx * 0.72, cy + Math.sin(a) * ry * 0.72);
                ctx.lineTo(cx + Math.cos(a) * rx * 0.92, cy + Math.sin(a) * ry * 0.92);
            }
            ctx.stroke();
        }
        ctx.restore();
    }

    // ---- projectiles ----
    // Atlas clip and source-to-screen scale per kind.
    const PROJECTILE_ART = {
        spear: ['fx-projectiles.spear', 0.22],
        missile: ['fx-projectiles.missile', 0.2],
        orb: ['fx-projectiles.orb', 0.2],
        shock: ['fx-projectiles.crest', 0.28],
        spore: ['fx-projectiles.spore', 0.15],
        cannon: ['fx-projectiles.cannonball', 0.17],
        arrow: ['fx-projectiles.arrow', 0.27],
        shard: ['fx-projectiles.shard', 0.2]
    };

    function artKind(pr) {
        if (pr.kind) return PROJECTILE_ART[pr.kind] ? pr.kind : '';
        if (pr.fromEnemy) return pr.color === '#fb923c' ? 'cannon' : 'shard';
        return 'arrow';
    }

    function boltTrail(ctx, col) {
        ctx.globalCompositeOperation = 'lighter';
        const g = ctx.createLinearGradient(0, 0, -44, 0);
        g.addColorStop(0, rgba(col, 0.95));
        g.addColorStop(1, rgba(col, 0));
        ctx.fillStyle = g;
        ctx.beginPath();
        ctx.moveTo(10, 0);
        ctx.lineTo(-44, -2.4);
        ctx.lineTo(-44, 2.4);
        ctx.closePath();
        ctx.fill();
    }

    function missileFlame(ctx, view, pr) {
        const fl = view.reducedMotion ? 1 : 0.7 + 0.3 * Math.sin(view.nowSec * 50 + pr.x);
        ctx.globalCompositeOperation = 'lighter';
        const g = ctx.createLinearGradient(-6, 0, -34 * fl, 0);
        g.addColorStop(0, 'rgba(255, 210, 120, 0.95)');
        g.addColorStop(0.5, 'rgba(251, 120, 40, 0.6)');
        g.addColorStop(1, 'rgba(251, 120, 40, 0)');
        ctx.fillStyle = g;
        ctx.beginPath();
        ctx.moveTo(-5, -4);
        ctx.lineTo(-34 * fl, 0);
        ctx.lineTo(-5, 4);
        ctx.closePath();
        ctx.fill();
        ctx.globalCompositeOperation = 'source-over';
    }

    // Glow and particles that belong to a kind, whether it is painted or drawn from primitives.
    function kindEffects(kind, ctx, pr, ang, view, hooks) {
        const glow = hooks.glow;
        const rnd = hooks.rnd;
        const calm = view.reducedMotion;
        if (kind === 'spear') glow(pr.x, pr.y, 26, rgba(pr.color || '#fde68a', 0.6), 0.5);
        else if (kind === 'orb') glow(pr.x, pr.y, (8 + (pr.radius ? pr.radius * 0.3 : 0)) * 3.6, rgba(pr.color || '#a78bfa', 0.7), 0.85);
        else if (kind === 'spore') {
            glow(pr.x, pr.y, 30, 'rgba(134, 239, 172, 0.6)', 0.55);
            if (!calm && rnd() < 0.3) hooks.addPart('bit', pr.x, pr.y, (rnd() - 0.5) * 30, -10 - rnd() * 24, 0.5, 2, '#86efac');
        } else if (kind === 'cannon') {
            glow(pr.x, pr.y, 34, 'rgba(251, 146, 60, 0.6)', 0.6);
            if (!calm && rnd() < 0.5) hooks.addPart('dust', pr.x - Math.cos(ang) * 9, pr.y, (rnd() - 0.5) * 20, -10, 0.4, 3 + rnd() * 3, '#8d8576');
        } else if (kind === 'arrow') glow(pr.x, pr.y, 30, rgba(hooks.fireBolt ? '#ff9a4a' : (pr.color || '#7ee8fa'), 0.6), 0.7);
        else if (kind === 'missile') {
            if (!calm && rnd() < 0.5) hooks.addPart('dust', pr.x - Math.cos(ang) * 14, pr.y - Math.sin(ang) * 14, (rnd() - 0.5) * 20, (rnd() - 0.5) * 20, 0.5, 3.5 + rnd() * 3, '#aaa69c');
        } else if (kind === 'shock') {
            const dir = pr.vx >= 0 ? 1 : -1;
            if (!calm && rnd() < 0.5) hooks.addPart('dust', pr.x - dir * 24, pr.y + 10, -dir * 30, -20 - rnd() * 20, 0.45, 4 + rnd() * 4, '#e8d8b8');
        } else if (kind === 'shard') {
            const col = pr.color || '#fb7185';
            glow(pr.x, pr.y, 30, rgba(col, 0.55), 0.6);
            if (!calm && rnd() < 0.35) hooks.addPart('bit', pr.x, pr.y, (rnd() - 0.5) * 40, (rnd() - 0.5) * 40, 0.25, 2, col);
        }
    }

    function paintProjectile(kind, at, ctx, pr, ang, view, hooks) {
        const spec = PROJECTILE_ART[kind];
        const sc = spec[1] * (kind === 'orb' && pr.radius ? 1 + pr.radius * 0.3 / 8 : 1);
        const left = pr.vx < 0;
        // the atlas draw rotates before it flips, so a leftward shot is mirrored and turned by the opposite vector
        const rot = left ? Math.atan2(-pr.vy, -pr.vx) : Math.atan2(pr.vy, pr.vx);
        if (kind === 'missile') {
            ctx.save();
            ctx.translate(pr.x, pr.y);
            ctx.rotate(ang);
            missileFlame(ctx, view, pr);
            ctx.restore();
        } else if (kind === 'arrow') {
            ctx.save();
            ctx.translate(pr.x, pr.y);
            ctx.rotate(ang);
            boltTrail(ctx, hooks.fireBolt ? '#ff9a4a' : (pr.color || '#7ee8fa'));
            ctx.restore();
        }
        if (kind === 'orb' || kind === 'cannon') put(ctx, at, spec[0], 0, pr.x, pr.y, sc, 0, false, 1);
        else if (kind === 'shock') put(ctx, at, spec[0], 0, pr.x, pr.y - 0.5, sc, 0, left, 1);
        else put(ctx, at, spec[0], 0, pr.x, pr.y, sc, rot, left, 1, '', kind === 'arrow' && hooks.fireBolt ? '#ff9a4a' : '', undefined);
    }

    function paintPrimitive(kind, ctx, pr, ang, view, hooks) {
        ctx.save();
        ctx.translate(pr.x, pr.y);
        if (kind === 'spear') {
            ctx.rotate(ang);
            ctx.fillStyle = '#6b4a2f';
            ctx.fillRect(-24, -1.6, 34, 3.2);
            ctx.fillStyle = '#a77d47';
            ctx.fillRect(-24, -1.6, 34, 1.1);
            ctx.fillStyle = '#e8efe9';
            ctx.beginPath();
            ctx.moveTo(-26, 0); ctx.lineTo(-19, -4.5); ctx.lineTo(-17, 0); ctx.lineTo(-19, 4.5);
            ctx.closePath();
            ctx.fill();
            ctx.fillStyle = '#7ee8fa';
            ctx.beginPath();
            ctx.moveTo(18, 0); ctx.lineTo(9, -5.5); ctx.lineTo(5, 0); ctx.lineTo(9, 5.5);
            ctx.closePath();
            ctx.fill();
            ctx.fillStyle = '#ffffff';
            ctx.beginPath();
            ctx.moveTo(15, 0); ctx.lineTo(9, -2.4); ctx.lineTo(7, 0);
            ctx.closePath();
            ctx.fill();
        } else if (kind === 'missile') {
            ctx.rotate(ang);
            missileFlame(ctx, view, pr);
            ctx.fillStyle = '#7b8498';
            ctx.fillRect(-9, -4, 20, 8);
            ctx.fillStyle = '#c9d0dd';
            ctx.fillRect(-9, -4, 20, 2.4);
            ctx.fillStyle = '#d9483b';
            ctx.beginPath();
            ctx.moveTo(11, -4); ctx.lineTo(19, 0); ctx.lineTo(11, 4);
            ctx.closePath();
            ctx.fill();
            ctx.fillStyle = '#4a5263';
            ctx.fillRect(-12, -7, 6, 4);
            ctx.fillRect(-12, 3, 6, 4);
        } else if (kind === 'orb') {
            const r = 8 + (pr.radius ? pr.radius * 0.3 : 0);
            const col = pr.color || '#a78bfa';
            const g = ctx.createRadialGradient(0, 0, 0, 0, 0, r);
            g.addColorStop(0, '#ffffff');
            g.addColorStop(0.45, col);
            g.addColorStop(1, rgba(col, 0.15));
            ctx.fillStyle = g;
            ctx.beginPath();
            ctx.arc(0, 0, r, 0, TAU);
            ctx.fill();
            ctx.globalCompositeOperation = 'lighter';
            ctx.strokeStyle = rgba(col, 0.95);
            ctx.lineWidth = 1.8;
            ctx.rotate(view.reducedMotion ? 0 : view.nowSec * 4 + pr.x * 0.02);
            for (let i = 0; i < 2; i++) {
                ctx.beginPath();
                ctx.ellipse(0, 0, r * 1.45, r * 0.62, 0, 0, TAU);
                ctx.stroke();
                ctx.rotate(1.5);
            }
        } else if (kind === 'shock') {
            const dir = pr.vx >= 0 ? 1 : -1;
            const hgt = 30 * (view.reducedMotion ? 1 : 0.9 + 0.1 * Math.sin(view.nowSec * 34 + pr.x * 0.1));
            ctx.translate(0, 12);
            ctx.scale(dir, 1);
            ctx.globalCompositeOperation = 'lighter';
            const g = ctx.createLinearGradient(0, 0, 0, -hgt);
            g.addColorStop(0, 'rgba(255, 190, 90, 0.95)');
            g.addColorStop(1, 'rgba(255, 240, 200, 0.15)');
            ctx.fillStyle = g;
            ctx.beginPath();
            ctx.moveTo(-34, 0);
            ctx.quadraticCurveTo(-18, -hgt * 0.25, -4, -hgt * 0.7);
            ctx.quadraticCurveTo(10, -hgt * 1.05, 22, -hgt * 0.35);
            ctx.quadraticCurveTo(16, -hgt * 0.12, 26, 0);
            ctx.closePath();
            ctx.fill();
            ctx.strokeStyle = 'rgba(255, 255, 255, 0.8)';
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.moveTo(-14, -hgt * 0.35);
            ctx.quadraticCurveTo(-2, -hgt * 0.8, 14, -hgt * 0.55);
            ctx.stroke();
        } else if (kind === 'spore') {
            const w = view.reducedMotion ? 0 : Math.sin(view.nowSec * 14 + pr.x * 0.05) * 0.14;
            ctx.scale(1 + w, 1 - w);
            ctx.fillStyle = '#3f9a52';
            ctx.beginPath(); ctx.arc(0, 0, 9, 0, TAU); ctx.fill();
            ctx.fillStyle = '#86efac';
            ctx.beginPath(); ctx.arc(-1.5, -2, 6.4, 0, TAU); ctx.fill();
            ctx.fillStyle = '#d9ffd9';
            ctx.beginPath(); ctx.arc(-3.4, -4, 2.2, 0, TAU); ctx.fill();
            ctx.fillStyle = 'rgba(40, 90, 50, 0.75)';
            ctx.beginPath(); ctx.arc(3, 2, 1.8, 0, TAU); ctx.arc(-2, 4, 1.3, 0, TAU); ctx.fill();
        } else if (kind === 'cannon') {
            ctx.rotate(ang);
            const g = ctx.createRadialGradient(-3, -3, 1, 0, 0, 10);
            g.addColorStop(0, '#7b8498');
            g.addColorStop(1, '#1d2230');
            ctx.fillStyle = g;
            ctx.beginPath(); ctx.arc(0, 0, 8.5, 0, TAU); ctx.fill();
            ctx.strokeStyle = 'rgba(251, 146, 60, 0.9)';
            ctx.lineWidth = 1.6;
            ctx.beginPath(); ctx.arc(0, 0, 8.5, -0.9, 0.9); ctx.stroke();
        } else if (kind === 'arrow') {
            const col = hooks.fireBolt ? '#ff9a4a' : (pr.color || '#7ee8fa');
            ctx.rotate(ang);
            boltTrail(ctx, col);
            ctx.globalCompositeOperation = 'lighter';
            ctx.fillStyle = '#ffffff';
            ctx.beginPath();
            ctx.moveTo(14, 0); ctx.lineTo(3, -3.2); ctx.lineTo(-6, 0); ctx.lineTo(3, 3.2);
            ctx.closePath();
            ctx.fill();
            ctx.fillStyle = rgba(col, 0.9);
            for (let i = 1; i <= 3; i++) {
                const gx = -i * 14 - ((view.nowSec * 40) % 14);
                const s = 3.2 - i * 0.6;
                ctx.globalAlpha = 0.9 - i * 0.22;
                ctx.beginPath();
                ctx.moveTo(gx, -s); ctx.lineTo(gx + s, 0); ctx.lineTo(gx, s); ctx.lineTo(gx - s, 0);
                ctx.closePath();
                ctx.fill();
            }
        }
        ctx.restore();
    }

    // Comet used for any projectile kind without its own look.
    function drawComet(ctx, pr, view, hooks) {
        const color = pr.color || (pr.fromEnemy ? '#fb7185' : '#7ee8fa');
        const sp = Math.hypot(pr.vx, pr.vy) || 1;
        const nx = pr.vx / sp;
        const ny = pr.vy / sp;
        const rad = pr.radius ? Math.max(4, Math.min(10, pr.radius * 0.7)) : (pr.fromEnemy ? 6 : 5);
        const len = Math.min(70, 16 + sp * 0.05);
        ctx.save();
        ctx.globalCompositeOperation = 'lighter';
        const g = ctx.createLinearGradient(pr.x, pr.y, pr.x - nx * len, pr.y - ny * len);
        g.addColorStop(0, color);
        g.addColorStop(1, 'rgba(0,0,0,0)');
        ctx.strokeStyle = g;
        ctx.lineCap = 'round';
        ctx.lineWidth = rad * 1.5;
        ctx.globalAlpha = 0.9;
        ctx.beginPath();
        ctx.moveTo(pr.x, pr.y);
        ctx.lineTo(pr.x - nx * len, pr.y - ny * len);
        ctx.stroke();
        ctx.lineWidth = rad * 0.55;
        ctx.strokeStyle = 'rgba(255,255,255,0.85)';
        ctx.beginPath();
        ctx.moveTo(pr.x, pr.y);
        ctx.lineTo(pr.x - nx * len * 0.5, pr.y - ny * len * 0.5);
        ctx.stroke();
        ctx.restore();
        hooks.glow(pr.x, pr.y, rad * 5, rgba(color, 0.6), 0.75);
        ctx.save();
        ctx.translate(pr.x, pr.y);
        ctx.rotate(Math.atan2(ny, nx));
        ctx.fillStyle = color;
        ctx.beginPath();
        ctx.ellipse(0, 0, rad * 1.25, rad * 0.85, 0, 0, TAU);
        ctx.fill();
        ctx.fillStyle = '#ffffff';
        ctx.beginPath();
        ctx.ellipse(rad * 0.15, 0, rad * 0.7, rad * 0.4, 0, 0, TAU);
        ctx.fill();
        ctx.restore();
        if (!view.reducedMotion && hooks.rnd() < 0.35) {
            hooks.addPart('bit', pr.x - nx * 6, pr.y - ny * 6, (hooks.rnd() - 0.5) * 40, (hooks.rnd() - 0.5) * 40, 0.25, 2, color);
        }
    }

    // Paints one projectile; returns true when it was handled (painted, primitive or left to the world pass).
    function drawProjectile(ctx, pr, view, hooks) {
        if (pr.kind === 'fireball' || pr.kind === 'toad_wave') return true;
        const kind = artKind(pr);
        if (!kind) {
            drawComet(ctx, pr, view, hooks);
            return true;
        }
        const ang = Math.atan2(pr.vy, pr.vx || 0.001);
        const at = atlasFor(PROJECTILE_ART[kind][0]);
        if (!at && kind === 'shard') {
            drawComet(ctx, pr, view, hooks);
            return true;
        }
        kindEffects(kind, ctx, pr, ang, view, hooks);
        if (at) paintProjectile(kind, at, ctx, pr, ang, view, hooks);
        else paintPrimitive(kind, ctx, pr, ang, view, hooks);
        return true;
    }

    root.RunenpfadPropsArt = {
        drawChain: drawChain,
        drawRail: drawRail,
        drawConveyor: drawConveyor,
        drawProjectile: drawProjectile,
        drawTelegraphRing: drawTelegraphRing
    };
})(typeof window !== 'undefined' ? window : globalThis);
