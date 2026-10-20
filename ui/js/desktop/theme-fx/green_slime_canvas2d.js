(function () {
    'use strict';

    // Canvas 2D renderer of the Green Slime effect: the fallback for browsers without WebGL2 and the
    // renderer of record until a better one takes the slot. It draws the scene contract of
    // green_slime_sim.js (gel contour from the radius table, drips, bubbles, strands, drops, splats)
    // with everything clipped by the scene's occluders. A renderer is
    // { kind, render(scene), resize(cssWidth, cssHeight, dpr), dispose() }.

    const TWO_PI = Math.PI * 2;

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }
    function smoothstep(a, b, x) { const t = clamp((x - a) / (b - a), 0, 1); return t * t * (3 - 2 * t); }
    function rgba(r, g, b, a) { return 'rgba(' + r + ',' + g + ',' + b + ',' + clamp(a, 0, 1).toFixed(3) + ')'; }

    function tableRadius(table, s, length) {
        const f = (((s % length) + length) % length) / length * table.length - .5;
        const k0 = Math.floor(f), t = f - k0;
        const n = table.length;
        return table[((k0 % n) + n) % n] * (1 - t) + table[(((k0 + 1) % n) + n) % n] * t;
    }

    function createCanvas2DRenderer(canvas) {
        const geometry = window.AuraGreenSlimeSim.geometry;
        // The material and DOM window must be presented in the same compositor
        // frame. A desynchronized canvas can visibly trail a fast pointer drag.
        const ctx = canvas.getContext('2d', { alpha: true });
        if (!ctx) return null;
        let width = 1;
        let height = 1;

        function resize(cssWidth, cssHeight, dpr) {
            width = cssWidth;
            height = cssHeight;
            canvas.width = Math.max(1, Math.round(cssWidth * dpr));
            canvas.height = Math.max(1, Math.round(cssHeight * dpr));
            ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        }

        function clipOut(ctx2d, blockers) {
            ctx2d.beginPath();
            ctx2d.rect(0, 0, width, height);
            ctx2d.clip();
            for (const rect of blockers) {
                ctx2d.beginPath();
                ctx2d.rect(0, 0, width, height);
                const radii = rect.radii || (rect.radius ? [rect.radius, rect.radius, rect.radius, rect.radius] : null);
                if (radii) ctx2d.roundRect(rect.x, rect.y, rect.width, rect.height, radii);
                else ctx2d.rect(rect.x, rect.y, rect.width, rect.height);
                ctx2d.clip('evenodd');
            }
        }

        // Closed polygon of the gel: every perimeter point moved outward by the radius table (minus
        // "inset"). Arcs get at least four points, so even square corners are rounded off.
        function gelPath(lay, table, inset) {
            const path = new Path2D();
            const p = {};
            let first = true;
            for (const g of lay.segs) {
                if (!g.arc && g.len <= 0) continue;
                const steps = g.arc ? Math.max(4, Math.ceil(g.len / 5)) : Math.max(1, Math.ceil(g.len / 8));
                for (let i = 0; i < steps; i++) {
                    const t = i / steps;
                    geometry.segmentPoint(g, t, p);
                    const r = tableRadius(table, g.start + t * g.len, lay.length) - inset;
                    if (first) { path.moveTo(p.x + p.nx * r, p.y + p.ny * r); first = false; }
                    else path.lineTo(p.x + p.nx * r, p.y + p.ny * r);
                }
            }
            path.closePath();
            return path;
        }

        function contourPoint(lay, table, s, inset) {
            const p = geometry.pointAt(lay, s, {});
            const r = tableRadius(table, s, lay.length) - inset;
            return { x: p.x + p.nx * r, y: p.y + p.ny * r, nx: p.nx, ny: p.ny, angle: Math.atan2(p.nx, -p.ny) };
        }

        function drawDrop(drop) {
            const size = drop.r;
            ctx.globalAlpha = drop.alpha;
            if (drop.spray || size < 2.4) {
                // Micro-droplets: a flat disc with a pin highlight, no gradient or blur.
                ctx.fillStyle = '#9fcf3c';
                ctx.beginPath();
                ctx.arc(drop.x, drop.y, Math.max(.6, size), 0, TWO_PI);
                ctx.fill();
                ctx.fillStyle = 'rgba(250,255,222,.9)';
                ctx.beginPath();
                ctx.arc(drop.x - size * .3, drop.y - size * .3, Math.max(.3, size * .32), 0, TWO_PI);
                ctx.fill();
                ctx.globalAlpha = 1;
                return;
            }
            // A falling drop stretches with its speed and squashes/stretches with a damped wobble.
            const stretch = 1 + Math.min(1.2, Math.abs(drop.vy) / 220);
            ctx.save();
            ctx.translate(drop.x, drop.y);
            ctx.scale(1 - drop.wobble * .55, stretch * (1 + drop.wobble));
            const gradient = ctx.createLinearGradient(-size, -size, size, size);
            gradient.addColorStop(0, '#f2ffbc');
            gradient.addColorStop(.26, '#c3f34c');
            gradient.addColorStop(.62, '#75ac1e');
            gradient.addColorStop(1, '#354f0c');
            ctx.fillStyle = gradient;
            ctx.shadowColor = 'rgba(126,158,40,.48)';
            ctx.shadowBlur = 12;
            ctx.beginPath();
            ctx.moveTo(0, -size);
            ctx.bezierCurveTo(size * .9, -size * .3, size * .82, size * .48, size * .35, size * .76);
            ctx.quadraticCurveTo(0, size, -size * .35, size * .76);
            ctx.bezierCurveTo(-size * .82, size * .48, -size * .9, -size * .3, 0, -size);
            ctx.closePath();
            ctx.fill();
            ctx.shadowBlur = 0;
            ctx.fillStyle = 'rgba(250,255,214,.9)';
            ctx.beginPath();
            ctx.ellipse(-size * .25, 0, size * .14, size * .4, -.25, 0, TWO_PI);
            ctx.fill();
            ctx.restore();
            ctx.globalAlpha = 1;
        }

        // A drop that landed on the taskbar edge: a flattened blob that spreads with a short
        // oscillation and then slowly thins out.
        function drawSplat(splat) {
            const spread = splat.spread;
            const w = splat.r * (.9 + 1.5 * spread);
            const h = splat.r * clamp(1.25 - .6 * spread, .38, 1.3);
            const x = splat.x, y = splat.y;
            ctx.save();
            ctx.globalAlpha = splat.alpha;
            const gradient = ctx.createRadialGradient(x - w * .25, y - h * .8, 0, x, y - h * .4, Math.max(w, h) * 1.1);
            gradient.addColorStop(0, '#d9ff7c');
            gradient.addColorStop(.4, '#9ccf34');
            gradient.addColorStop(.85, '#4f7a12');
            gradient.addColorStop(1, '#2e4a0a');
            ctx.fillStyle = gradient;
            ctx.shadowColor = 'rgba(126,158,40,.4)';
            ctx.shadowBlur = 8;
            ctx.beginPath();
            ctx.moveTo(x - w - 3, y);
            ctx.quadraticCurveTo(x - w * .92, y, x - w * .8, y - h * .35);
            ctx.bezierCurveTo(x - w * .55, y - h * 1.08, x + w * .55, y - h * 1.08, x + w * .8, y - h * .35);
            ctx.quadraticCurveTo(x + w * .92, y, x + w + 3, y);
            ctx.closePath();
            ctx.fill();
            ctx.shadowBlur = 0;
            ctx.fillStyle = 'rgba(250,255,222,.85)';
            ctx.beginPath();
            ctx.ellipse(x - w * .28, y - h * .62, w * .2, h * .14, -.2, 0, TWO_PI);
            ctx.fill();
            ctx.restore();
        }

        function drawDrip(drip) {
            const y = drip.y;
            const len = drip.length;
            const end = y + len;
            const x = drip.x + drip.sway;
            const root = drip.root, neck = drip.neck, bulb = drip.bulb;
            const gradient = ctx.createLinearGradient(drip.x - root, y, x + root, end);
            gradient.addColorStop(0, '#b4d96c');
            gradient.addColorStop(.25, '#a1cc45');
            gradient.addColorStop(.56, '#699722');
            gradient.addColorStop(.86, '#36591a');
            gradient.addColorStop(1, '#9abd47');
            ctx.fillStyle = gradient;
            ctx.shadowColor = 'rgba(12,28,1,.5)';
            ctx.shadowBlur = 5;
            ctx.shadowOffsetY = 3;
            ctx.beginPath();
            ctx.moveTo(drip.x - root, y);
            ctx.bezierCurveTo(drip.x - neck, y + len * .3, x - neck, end - bulb * 1.8, x - bulb, end - bulb);
            ctx.bezierCurveTo(x - bulb * 1.35, end + bulb, x + bulb * 1.35, end + bulb, x + bulb, end - bulb);
            ctx.bezierCurveTo(x + neck, end - bulb * 1.8, drip.x + neck, y + len * .3, drip.x + root, y);
            ctx.closePath();
            ctx.fill();
            ctx.shadowBlur = 0;
            ctx.shadowOffsetY = 0;
            ctx.fillStyle = 'rgba(249,255,210,.92)';
            ctx.beginPath();
            ctx.ellipse(x - bulb * .3, end - bulb * .4, 1.2, bulb * .55, -.3, 0, TWO_PI);
            ctx.fill();
            ctx.strokeStyle = 'rgba(241,255,196,.65)';
            ctx.lineWidth = 1;
            ctx.beginPath();
            ctx.moveTo(drip.x - root * .6, y + 2);
            ctx.bezierCurveTo(drip.x - neck * .65, y + len * .3, x - neck * .7, end - bulb * 2, x - bulb * .65, end - bulb);
            ctx.stroke();
        }

        // Gas pocket inside the gel: clearer centre, dark refracted rim, pin highlight.
        function drawPocket(b) {
            const r = Math.max(1.2, b.r), x = b.x, y = b.y;
            ctx.save();
            ctx.globalAlpha = clamp(b.r / 2.2, .3, 1);
            const gradient = ctx.createRadialGradient(x - r * .2, y - r * .25, r * .1, x, y, r);
            gradient.addColorStop(0, 'rgba(244,255,200,.62)');
            gradient.addColorStop(.6, 'rgba(206,240,130,.3)');
            gradient.addColorStop(.88, 'rgba(48,84,14,.45)');
            gradient.addColorStop(1, 'rgba(26,48,6,.7)');
            ctx.fillStyle = gradient;
            ctx.beginPath();
            ctx.ellipse(x, y, r, r * 1.1, 0, 0, TWO_PI);
            ctx.fill();
            ctx.strokeStyle = 'rgba(226,250,150,.5)';
            ctx.lineWidth = Math.max(.6, r * .1);
            ctx.beginPath();
            ctx.arc(x, y, r * .9, Math.PI * .15, Math.PI * .8);
            ctx.stroke();
            ctx.fillStyle = 'rgba(255,255,244,.9)';
            ctx.beginPath();
            ctx.ellipse(x - r * .35, y - r * .4, Math.max(.4, r * .2), Math.max(.3, r * .11), -.6, 0, TWO_PI);
            ctx.fill();
            ctx.restore();
        }

        // Dome of a bubble seen through its own film: the gel shows through, the silhouette is a
        // darker green ring (long path through the film), a curved soft-box reflection and a pin
        // highlight sit up-left, the lime caustic lower-right; before failure a faint interference tint.
        function drawDome(b, alpha) {
            const x = b.x, y = b.y, r = b.r;
            const d = clamp(b.dome, 0, 1.02);
            ctx.save();
            ctx.globalAlpha = alpha == null ? 1 : alpha;
            ctx.fillStyle = rgba(14, 30, 3, .26 * d);
            ctx.beginPath();
            ctx.ellipse(x + r * .14 * d, y + r * .22 * d, r * 1.02, r * .96, 0, 0, TWO_PI);
            ctx.fill();
            const body = ctx.createRadialGradient(x - r * .2 * d, y - r * .24 * d, r * .05, x, y, r);
            body.addColorStop(0, rgba(236, 255, 176, .1 + .12 * b.film));
            body.addColorStop(.58, rgba(160, 208, 78, .07 + .1 * b.film));
            body.addColorStop(.86, rgba(70, 112, 22, .06 + .34 * d));
            body.addColorStop(1, rgba(24, 46, 6, .1 + .7 * d));
            ctx.fillStyle = body;
            ctx.beginPath();
            ctx.arc(x, y, r, 0, TWO_PI);
            ctx.fill();
            ctx.save();
            ctx.beginPath();
            ctx.arc(x, y, r, 0, TWO_PI);
            ctx.clip();
            const caustic = ctx.createRadialGradient(x + r * .42, y + r * .46, 0, x + r * .42, y + r * .46, r * .62);
            caustic.addColorStop(0, rgba(228, 255, 128, .4 * d));
            caustic.addColorStop(1, rgba(228, 255, 128, 0));
            ctx.fillStyle = caustic;
            ctx.fillRect(x - r, y - r, r * 2, r * 2);
            if (b.interference > .02) {
                const tint = ctx.createLinearGradient(x - r * .5, y - r * .6, x + r * .4, y + r * .1);
                tint.addColorStop(0, rgba(255, 214, 84, .13 * b.interference));
                tint.addColorStop(.45, rgba(255, 110, 214, .11 * b.interference));
                tint.addColorStop(.8, rgba(110, 200, 255, .08 * b.interference));
                tint.addColorStop(1, rgba(255, 214, 84, 0));
                ctx.fillStyle = tint;
                ctx.fillRect(x - r, y - r, r * 2, r * 2);
            }
            ctx.translate(x - r * .3, y - r * .36);
            ctx.rotate(-.62);
            ctx.scale(1, .44);
            const box = ctx.createRadialGradient(0, 0, 0, 0, 0, r * .5);
            box.addColorStop(0, rgba(255, 255, 244, .82 * d));
            box.addColorStop(.6, rgba(244, 255, 214, .36 * d));
            box.addColorStop(1, rgba(244, 255, 214, 0));
            ctx.fillStyle = box;
            ctx.beginPath();
            ctx.arc(0, 0, r * .5, 0, TWO_PI);
            ctx.fill();
            ctx.restore();
            ctx.fillStyle = rgba(255, 255, 248, .95 * Math.min(1, d * 1.3));
            ctx.beginPath();
            ctx.ellipse(x - r * .46, y - r * .5, Math.max(.4, r * .13), Math.max(.3, r * .075), -.62, 0, TWO_PI);
            ctx.fill();
            ctx.strokeStyle = rgba(238, 255, 180, .5 * d);
            ctx.lineWidth = Math.max(.6, r * .07);
            ctx.beginPath();
            ctx.arc(x, y, r * .88, Math.PI * .12, Math.PI * .72);
            ctx.stroke();
            ctx.strokeStyle = rgba(30, 56, 8, .1 + .5 * d);
            ctx.lineWidth = 1;
            ctx.beginPath();
            ctx.arc(x, y, r, 0, TWO_PI);
            ctx.stroke();
            ctx.restore();
        }

        // The film retracts from the rupture point behind a thick, bright rim.
        function drawBurst(b) {
            ctx.save();
            ctx.beginPath();
            ctx.rect(b.x - b.r * 2.2, b.y - b.r * 2.2, b.r * 4.4, b.r * 4.4);
            ctx.moveTo(b.hx + b.holeR, b.hy);
            ctx.arc(b.hx, b.hy, b.holeR, 0, TWO_PI);
            ctx.clip('evenodd');
            drawDome(b, 1 - .55 * b.t);
            ctx.restore();
            if (b.holeR < .5) return;
            ctx.save();
            ctx.beginPath();
            ctx.arc(b.x, b.y, b.r * 1.02, 0, TWO_PI);
            ctx.clip();
            ctx.fillStyle = rgba(30, 54, 8, .28 * (1 - b.t));
            ctx.beginPath();
            ctx.arc(b.hx, b.hy, b.holeR, 0, TWO_PI);
            ctx.fill();
            ctx.strokeStyle = 'rgba(94,142,30,.85)';
            ctx.lineWidth = b.rim + 1.6;
            ctx.beginPath();
            ctx.arc(b.hx, b.hy, b.holeR, 0, TWO_PI);
            ctx.stroke();
            ctx.strokeStyle = 'rgba(250,255,214,.95)';
            ctx.lineWidth = Math.max(.6, b.rim * .55);
            ctx.stroke();
            ctx.restore();
        }

        // Where the dome stood: a depression with a damped ring oscillation and a small central rebound.
        function drawCrater(b) {
            const a = Math.min(1, Math.abs(b.crater));
            const pit = b.crater >= 0;
            const x = b.x, y = b.y, r = b.r;
            ctx.save();
            if (a > .03) {
                ctx.lineCap = 'round';
                ctx.lineWidth = Math.max(1, r * .2);
                // Lit from the upper left: the far wall of a pit is dark, the near wall catches light.
                ctx.strokeStyle = pit ? rgba(22, 44, 6, .6 * a) : rgba(240, 255, 190, .75 * a);
                ctx.beginPath();
                ctx.arc(x, y, r * .92, Math.PI * .72, Math.PI * 1.78);
                ctx.stroke();
                ctx.strokeStyle = pit ? rgba(240, 255, 190, .75 * a) : rgba(22, 44, 6, .6 * a);
                ctx.beginPath();
                ctx.arc(x, y, r * .92, -Math.PI * .28, Math.PI * .78);
                ctx.stroke();
                ctx.strokeStyle = rgba(228, 252, 160, .35 * a * (1 - b.t));
                ctx.lineWidth = 1;
                ctx.beginPath();
                ctx.arc(x, y, r * (1.18 + .5 * b.t), 0, TWO_PI);
                ctx.stroke();
            }
            if (b.rebound > .04) {
                const bump = ctx.createRadialGradient(x - r * .08, y - r * .1, 0, x, y, r * .4);
                bump.addColorStop(0, rgba(250, 255, 222, .75 * b.rebound));
                bump.addColorStop(.5, rgba(214, 246, 130, .3 * b.rebound));
                bump.addColorStop(1, rgba(214, 246, 130, 0));
                ctx.fillStyle = bump;
                ctx.beginPath();
                ctx.arc(x, y, r * .4, 0, TWO_PI);
                ctx.fill();
            }
            ctx.restore();
        }

        function drawBubble(b) {
            if (b.r < .4) return;
            if (b.stage === 'nucleate') drawPocket(b);
            else if (b.stage === 'burst') drawBurst(b);
            else if (b.stage === 'crater') drawCrater(b);
            else drawDome(b);
        }

        // A drawn-out thread: tapered, drooping, beaded once it is thin enough.
        function drawStrand(strand, time) {
            const dx = strand.bx - strand.ax, dy = strand.by - strand.ay;
            const length = Math.hypot(dx, dy) || 1;
            const cx = (strand.ax + strand.bx) / 2, cy = (strand.ay + strand.by) / 2 + Math.min(16, length * .09);
            const count = clamp(Math.ceil(length / 6), 10, 36);
            const left = [], right = [], spine = [];
            for (let i = 0; i <= count; i++) {
                const u = i / count, m = 1 - u;
                const x = m * m * strand.ax + 2 * m * u * cx + u * u * strand.bx;
                const y = m * m * strand.ay + 2 * m * u * cy + u * u * strand.by;
                const tx = 2 * m * (cx - strand.ax) + 2 * u * (strand.bx - cx);
                const ty = 2 * m * (cy - strand.ay) + 2 * u * (strand.by - cy);
                const tl = Math.hypot(tx, ty) || 1;
                const nx = -ty / tl, ny = tx / tl;
                let r = strand.mid + (strand.root - strand.mid) * Math.pow(m, 3) + (strand.tip - strand.mid) * Math.pow(u, 5);
                if (strand.beads) {
                    // Rayleigh-Plateau: beads swell, the necks between them thin.
                    let bead = 0;
                    for (let j = 0; j < strand.beads; j++) {
                        const centre = (j + 1) / (strand.beads + 1) + Math.sin(time * 2.1 + j * 1.7) * .012;
                        bead += Math.exp(-Math.pow((u - centre) / .05, 2));
                    }
                    r *= 1 + (.62 - 1 + 1.9 * Math.min(1, bead)) * smoothstep(.05, .2, u) * (1 - smoothstep(.85, .97, u));
                }
                left.push(x + nx * r, y + ny * r);
                right.push(x - nx * r, y - ny * r);
                spine.push(x - nx * r * .35, y - ny * r * .35);
            }
            const gradient = ctx.createLinearGradient(strand.ax, strand.ay, strand.bx, strand.by);
            gradient.addColorStop(0, '#a9d352');
            gradient.addColorStop(.55, '#79ad28');
            gradient.addColorStop(1, '#4f7a14');
            ctx.save();
            ctx.fillStyle = gradient;
            ctx.strokeStyle = 'rgba(36,60,10,.55)';
            ctx.lineWidth = .8;
            ctx.lineJoin = 'round';
            ctx.beginPath();
            ctx.moveTo(left[0], left[1]);
            for (let i = 2; i < left.length; i += 2) ctx.lineTo(left[i], left[i + 1]);
            for (let i = right.length - 2; i >= 0; i -= 2) ctx.lineTo(right[i], right[i + 1]);
            ctx.closePath();
            ctx.fill();
            ctx.stroke();
            ctx.strokeStyle = 'rgba(250,255,214,.8)';
            ctx.lineWidth = Math.max(.6, strand.mid * .4);
            ctx.beginPath();
            ctx.moveTo(spine[0], spine[1]);
            for (let i = 2; i < spine.length - 2; i += 2) ctx.lineTo(spine[i], spine[i + 1]);
            ctx.stroke();
            const bulb = strand.tip * 1.15;
            const tipGradient = ctx.createRadialGradient(strand.bx - bulb * .3, strand.by - bulb * .3, 0, strand.bx, strand.by, bulb);
            tipGradient.addColorStop(0, '#e8ffa6');
            tipGradient.addColorStop(.5, '#8fc232');
            tipGradient.addColorStop(1, '#4a7512');
            ctx.fillStyle = tipGradient;
            ctx.beginPath();
            ctx.arc(strand.bx, strand.by, bulb, 0, TWO_PI);
            ctx.fill();
            ctx.restore();
        }

        function drawWindow(win, time) {
            const rect = win.rect;
            const occluders = win.occluders;
            const lay = geometry.layout(rect, win.radii);
            const table = win.radius;
            ctx.save();
            const paint = { x: rect.x - 48, y: rect.y - 48, width: rect.width + 96, height: rect.height + 144 };
            ctx.beginPath();
            ctx.rect(paint.x, paint.y, paint.width, paint.height);
            ctx.clip();
            const blockers = occluders.filter(r => r.x < paint.x + paint.width && r.x + r.width > paint.x && r.y < paint.y + paint.height && r.y + r.height > paint.y);
            clipOut(ctx, blockers);
            // Draw attached drops first; the body covers their roots, so a neck
            // grows out of one continuous volume instead of sitting on its edge.
            for (const drip of win.drips) drawDrip(drip);
            const gradient = ctx.createLinearGradient(rect.x, rect.y - 20, rect.x + rect.width * .45, rect.y + rect.height);
            gradient.addColorStop(0, '#8eaf3f');
            gradient.addColorStop(.22, '#6b8f25');
            gradient.addColorStop(.58, '#42631b');
            gradient.addColorStop(1, '#85a834');
            ctx.lineJoin = 'round';
            ctx.fillStyle = gradient;
            // The DOM already supplies the window's soft cast shadow. Blurring a
            // second window-sized silhouette every frame is costly in software.
            const outer = gelPath(lay, table, 0);
            ctx.fill(outer);
            // A filled membrane, bounded by the live outer curve and the anchored
            // round window aperture. Unlike a displaced stroke, it cannot detach.
            ctx.save();
            ctx.clip(outer);
            ctx.lineWidth = 4;
            ctx.strokeStyle = 'rgba(29,49,9,.65)';
            ctx.stroke(outer);
            const volume = ctx.createLinearGradient(rect.x - 20, rect.y - 20, rect.x + rect.width, rect.y + rect.height);
            volume.addColorStop(0, '#cae884');
            volume.addColorStop(.3, '#96bc49');
            volume.addColorStop(.65, '#6d9026');
            volume.addColorStop(1, '#b5d75b');
            ctx.lineWidth = 11;
            ctx.strokeStyle = volume;
            ctx.stroke(gelPath(lay, table, 7));
            // Long, soft reflections flow over the curved material, with thin
            // Fresnel glints at grazing angles and darker absorbed light inside.
            const seed = Array.from(win.id).reduce((sum, letter) => sum + letter.charCodeAt(0), 0) * .17;
            for (let i = 0; i < 8; i++) {
                const p = contourPoint(lay, table, (((i * 4.13 + .8 + seed + time * .12) / 32) % 1) * lay.length, 7);
                ctx.save();
                ctx.translate(p.x, p.y);
                ctx.rotate(p.angle);
                ctx.scale(7, 1);
                const sheen = ctx.createRadialGradient(0, 0, .3, 0, 0, 6);
                sheen.addColorStop(0, 'rgba(244,255,211,.72)');
                sheen.addColorStop(.3, 'rgba(218,245,150,.3)');
                sheen.addColorStop(1, 'rgba(218,245,150,0)');
                ctx.fillStyle = sheen;
                ctx.fillRect(-6, -6, 12, 12);
                ctx.restore();
            }
            const reflection = ctx.createLinearGradient(rect.x, rect.y, rect.x + rect.width * .6, rect.y + rect.height);
            reflection.addColorStop(0, 'rgba(250,255,224,.95)');
            reflection.addColorStop(.42, 'rgba(222,246,159,.5)');
            reflection.addColorStop(.75, 'rgba(198,232,121,.13)');
            reflection.addColorStop(1, 'rgba(239,255,180,.64)');
            ctx.strokeStyle = reflection;
            ctx.lineWidth = 1.4;
            ctx.stroke(gelPath(lay, table, 3));
            ctx.beginPath();
            ctx.roundRect(rect.x - 1.5, rect.y - 1.5, rect.width + 3, rect.height + 3, win.radii.map(r => r + 1.5));
            ctx.strokeStyle = 'rgba(20,38,6,.6)';
            ctx.lineWidth = 5;
            ctx.stroke();
            ctx.strokeStyle = 'rgba(194,226,112,.62)';
            ctx.lineWidth = 1;
            ctx.stroke();
            // Air inclusions advect through the membrane; each has a dark lens,
            // a refracted lower rim and an off-centre wet highlight.
            for (let i = 0; i < 22; i++) {
                const flow = (i * 1.457 + seed + time * (.07 + (i % 3) * .025)) / 32;
                const p = contourPoint(lay, table, (flow % 1) * lay.length, 7 + (i % 3) * 2);
                const radius = 1.1 + (i % 4) * .65;
                ctx.fillStyle = 'rgba(38,66,12,.45)';
                ctx.beginPath();
                ctx.ellipse(p.x, p.y, radius, radius * 1.23, p.angle, 0, TWO_PI);
                ctx.fill();
                ctx.strokeStyle = 'rgba(222,247,162,.65)';
                ctx.lineWidth = .7;
                ctx.beginPath(); ctx.arc(p.x, p.y, radius, .1, Math.PI * .85); ctx.stroke();
                ctx.fillStyle = 'rgba(251,255,229,.85)';
                ctx.beginPath(); ctx.ellipse(p.x - radius * .33, p.y - radius * .5, .65, radius * .43, -.4, 0, TWO_PI); ctx.fill();
            }
            ctx.restore();
            for (const bubble of win.bubbles) drawBubble(bubble);
            ctx.restore();
            if (win.strands.length) {
                // A thread reaches far outside the window's paint box.
                ctx.save();
                clipOut(ctx, occluders);
                for (const strand of win.strands) drawStrand(strand, time);
                ctx.restore();
            }
        }

        function render(scene) {
            ctx.clearRect(0, 0, width, height);
            for (const win of scene.windows) drawWindow(win, scene.time);
            if (scene.splats.length) {
                ctx.save();
                clipOut(ctx, scene.dropOccluders.filter(item => !item.taskbar));
                for (const splat of scene.splats) drawSplat(splat);
                ctx.restore();
            }
            if (scene.drops.length) {
                const blockers = scene.dropOccluders;
                ctx.save();
                clipOut(ctx, blockers);
                for (const drop of scene.drops) drawDrop(drop);
                ctx.restore();
            }
        }

        function dispose() { canvas.remove(); }

        return { kind: 'canvas2d', render, resize, dispose };
    }

    window.AuraGreenSlimeCanvas2D = Object.freeze({ create: createCanvas2DRenderer });
})();
