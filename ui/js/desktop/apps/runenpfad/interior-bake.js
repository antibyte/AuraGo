/* Runenpfad interior room baking: the static layer of a top-down room (mirror-tiled floor, wall faces, rock-mass wall tops, contact shadows, floor props) painted once into an offscreen canvas. Canvas-facing, read-only, never throws. */
(function (root) {
    'use strict';

    const CELL = 48;
    const SCALE = 2; // bake resolution: the canvas holds SCALE device pixels per world px
    const BLOCK = 4; // floor texture covers 4x4 cells; neighbours are mirrored so seams never show
    const PROP_W = 0.94;

    // Flat fallbacks and tints per theme (floor, alternate floor, wall top, wall face).
    const THEMES = {
        wood: { floor: '#7a5430', alt: '#6c6459', top: '#241810', face: '#5a4128' },
        cave: { floor: '#4b3d34', alt: '#3e3a44', top: '#22202a', face: '#2e2b34' },
        tomb: { floor: '#b99a5e', alt: '#a98a52', top: '#3f301b', face: '#8f7545' },
        ice: { floor: '#9ec7df', alt: '#cfe6f2', top: '#1f3c52', face: '#4d7894' },
        works: { floor: '#3d3f47', alt: '#6b5a2a', top: '#1a1a1f', face: '#57331f' },
        mine: { floor: '#5a4632', alt: '#4a3a2c', top: '#1d1712', face: '#3e3024' },
        glut: { floor: '#3a2622', alt: '#4a2a1e', top: '#150c0a', face: '#2c1814' }
    };
    const LAVA = { fill: '#c2410c', hot: '#fb923c', rim: '#2a0d06' };
    const PROP_FALLBACK = { 'tdp-tavern': '#8a5a2b', 'tdp-trade': '#7a6a50', 'tdp-cave': '#6b6458', 'tdp-ruin': '#a08a5a',
        'tdp-mine': '#6b5640', 'tdp-lair': '#4a3430', 'tdp-home': '#8a6a48', 'tdp-house': '#8a6a48' };

    function Art() { return root.RunenpfadArt || null; }

    function hash2(x, y, s) {
        let n = (Math.imul(x | 0, 374761393) + Math.imul(y | 0, 668265263) + Math.imul(s | 0, 144665)) | 0;
        n = Math.imul(n ^ (n >>> 13), 1274126177);
        return ((n ^ (n >>> 16)) >>> 0) / 4294967296;
    }

    function themeOf(room) {
        const th = room && room.def && room.def.theme;
        return THEMES[th] ? th : 'wood';
    }

    // Sheet image plus the first frame rect of a clip, or null while the sheet is missing.
    function clipFrame(atlas, clipId) {
        const c = atlas && atlas.clips && atlas.clips[clipId];
        if (!c) return null;
        const f = c.sheet.frames[c.frames[0]];
        return f && f[2] > 0 ? { img: c.sheet.img, f: f, clip: c } : null;
    }

    // 0 void, 1 wall, 2 floor, 3 alternate floor, 4 lava (markers count as floor).
    function kindAt(map, cx, cy) {
        const row = map[cy];
        if (!row || cx < 0 || cx >= row.length) return 0;
        const ch = row.charAt(cx);
        if (ch === '#') return 1;
        if (ch === ' ') return 0;
        if (ch === '~') return 4;
        return ch === ',' ? 3 : 2;
    }
    function isFloor(k) { return k >= 2; }

    // Ids of the sheets a room draws, used to re-bake once more of them have loaded.
    function sheetsOf(room) {
        const ids = ['tex-' + themeOf(room)];
        const rows = room && room.def && room.def.map ? room.def.map : [];
        if (rows.some(function (r) { return String(r).indexOf('~') >= 0; }) && ids.indexOf('tex-glut') < 0) ids.push('tex-glut');
        const props = room && room.props ? room.props : [];
        for (let i = 0; i < props.length; i++) {
            if (props[i].layer !== 'floor') continue;
            const id = String(props[i].clip).split('.')[0];
            if (ids.indexOf(id) < 0) ids.push(id);
        }
        return ids;
    }

    function signature(room, atlas) {
        const ids = sheetsOf(room);
        let s = '';
        for (let i = 0; i < ids.length; i++) s += atlas && atlas.sheets && atlas.sheets[ids[i]] ? '1' : '0';
        return s;
    }

    // Draws a prop with its feet at (p.x, p.y) so its width matches the footprint; rounded-rect fallback without art.
    function drawProp(ctx, atlas, p, o) {
        const opt = o || {};
        const A = Art();
        const wpx = Math.max(12, (p.wCells || 1) * CELL * PROP_W);
        const cf = clipFrame(atlas, p.clip);
        if (cf && A) {
            const sc = wpx / cf.f[2];
            const maxH = Math.max(p.hCells || 1, 3) * CELL;
            const k = cf.f[3] * sc > maxH ? maxH / (cf.f[3] * sc) : 1;
            const draw = { scale: sc * k, pixel: sc * k * (opt.screen || 1) >= 0.95 };
            if (opt.alpha != null) draw.alpha = opt.alpha;
            if (opt.flash) draw.flash = opt.flash;
            if (A.draw(ctx, atlas, p.clip, 0, p.x, p.y, draw)) return true;
        }
        const flat = p.layer === 'floor';
        const h = Math.max(14, (p.hCells || 1) * CELL * (flat ? 0.7 : 0.78));
        const fam = String(p.clip).split('.')[0];
        ctx.save();
        ctx.fillStyle = PROP_FALLBACK[fam] || '#7a6040';
        ctx.strokeStyle = 'rgba(0,0,0,0.55)';
        ctx.lineWidth = 2;
        roundRect(ctx, p.x - wpx / 2, p.y - h - (flat ? 2 : 0), wpx, h, 7);
        ctx.fill();
        ctx.stroke();
        ctx.fillStyle = 'rgba(255,255,255,0.14)';
        roundRect(ctx, p.x - wpx / 2 + 3, p.y - h + 1, wpx - 6, Math.max(4, h * 0.28), 4);
        ctx.fill();
        ctx.restore();
        return false;
    }

    function roundRect(ctx, x, y, w, h, r) {
        const rr = Math.min(r, w / 2, h / 2);
        ctx.beginPath();
        ctx.moveTo(x + rr, y);
        ctx.lineTo(x + w - rr, y);
        ctx.arcTo(x + w, y, x + w, y + rr, rr);
        ctx.lineTo(x + w, y + h - rr);
        ctx.arcTo(x + w, y + h, x + w - rr, y + h, rr);
        ctx.lineTo(x + rr, y + h);
        ctx.arcTo(x, y + h, x, y + h - rr, rr);
        ctx.lineTo(x, y + rr);
        ctx.arcTo(x, y, x + rr, y, rr);
        ctx.closePath();
    }

    function floorLayer(g, map, cols, rows, variant, tex, col, theme) {
        g.save();
        g.beginPath();
        let any = false;
        for (let cy = 0; cy < rows; cy++) {
            for (let cx = 0; cx < cols; cx++) {
                const k = kindAt(map, cx, cy);
                if (k !== 4 && isFloor(k) && (k === 3 ? 1 : 0) === variant) {
                    g.rect(cx * CELL, cy * CELL, CELL, CELL);
                    any = true;
                }
            }
        }
        if (!any) { g.restore(); return; }
        g.clip();
        g.fillStyle = col;
        g.fillRect(0, 0, cols * CELL, rows * CELL);
        if (tex) {
            const span = BLOCK * CELL;
            g.imageSmoothingEnabled = true;
            for (let by = 0; by * span < rows * CELL; by++) {
                for (let bx = 0; bx * span < cols * CELL; bx++) {
                    g.save();
                    g.translate(bx * span + (bx & 1 ? span : 0), by * span + (by & 1 ? span : 0));
                    g.scale(bx & 1 ? -1 : 1, by & 1 ? -1 : 1);
                    g.drawImage(tex.img, tex.f[0], tex.f[1], tex.f[2], tex.f[3], 0, 0, span, span);
                    g.restore();
                }
            }
        } else {
            // Flat colour with a faint per-cell tint so the floor does not read as one slab.
            for (let cy = 0; cy < rows; cy++) {
                for (let cx = 0; cx < cols; cx++) {
                    const k = kindAt(map, cx, cy);
                    if (k === 4 || !isFloor(k) || (k === 3 ? 1 : 0) !== variant) continue;
                    const v = hash2(cx, cy, 5);
                    g.fillStyle = v < 0.5 ? 'rgba(0,0,0,' + (0.01 + v * 0.05).toFixed(3) + ')' : 'rgba(255,255,255,' + ((v - 0.5) * 0.05).toFixed(3) + ')';
                    g.fillRect(cx * CELL, cy * CELL, CELL, CELL);
                    g.fillStyle = 'rgba(0,0,0,0.05)';
                    g.fillRect(cx * CELL, cy * CELL + CELL - 1, CELL, 1);
                    g.fillRect(cx * CELL + CELL - 1, cy * CELL, 1, CELL);
                }
            }
        }
        g.restore();
        void theme;
    }

    // Flat wall tops as one rock mass: a clipped base fill with deterministic blobs crossing cell borders.
    function wallTops(g, map, cols, rows, th, theme) {
        g.save();
        g.beginPath();
        let any = false;
        for (let cy = 0; cy < rows; cy++) {
            for (let cx = 0; cx < cols; cx++) {
                if (kindAt(map, cx, cy) !== 1 || isFloor(kindAt(map, cx, cy + 1))) continue;
                g.rect(cx * CELL, cy * CELL, CELL, CELL);
                any = true;
            }
        }
        if (!any) { g.restore(); return; }
        g.clip();
        g.fillStyle = th.top;
        g.fillRect(0, 0, cols * CELL, rows * CELL);
        const rocky = theme === 'cave' || theme === 'ice' || theme === 'tomb' || theme === 'glut';
        for (let cy = 0; cy < rows; cy++) {
            for (let cx = 0; cx < cols; cx++) {
                if (kindAt(map, cx, cy) !== 1 || isFloor(kindAt(map, cx, cy + 1))) continue;
                const n = rocky ? 3 : 0;
                for (let i = 0; i < n; i++) {
                    const a = hash2(cx, cy, 11 + i);
                    const b = hash2(cx, cy, 31 + i);
                    const c = hash2(cx, cy, 71 + i);
                    const bx = cx * CELL + a * CELL;
                    const by = cy * CELL + b * CELL;
                    const rx = 12 + c * 20;
                    const ry = rx * (0.55 + a * 0.4);
                    g.fillStyle = c < 0.5 ? 'rgba(0,0,0,' + (0.18 + c * 0.3).toFixed(3) + ')' : 'rgba(255,255,255,' + (0.03 + (c - 0.5) * 0.08).toFixed(3) + ')';
                    g.beginPath();
                    g.ellipse(bx, by, rx, ry, a * 3, 0, Math.PI * 2);
                    g.fill();
                }
                if (!rocky) {
                    // Timber or masonry top: faint horizontal courses, shifted per row.
                    const v = hash2(cx, cy, 9);
                    g.fillStyle = 'rgba(255,255,255,' + (0.025 + v * 0.03).toFixed(3) + ')';
                    g.fillRect(cx * CELL, cy * CELL + 11, CELL, 2);
                    g.fillRect(cx * CELL, cy * CELL + 35, CELL, 2);
                    g.fillStyle = 'rgba(0,0,0,0.2)';
                    g.fillRect(cx * CELL, cy * CELL + 24, CELL, 2);
                }
                // Lip along an edge that faces floor: reads as the thickness of the wall.
                const x = cx * CELL;
                const y = cy * CELL;
                g.fillStyle = 'rgba(255,255,255,0.10)';
                if (isFloor(kindAt(map, cx, cy - 1))) g.fillRect(x, y, CELL, 3);
                if (isFloor(kindAt(map, cx - 1, cy))) g.fillRect(x, y, 3, CELL);
                if (isFloor(kindAt(map, cx + 1, cy))) g.fillRect(x + CELL - 3, y, 3, CELL);
            }
        }
        g.restore();
    }

    // Wall faces (wall cells with floor to the south): one cell of the front texture with a dark cap line.
    function wallFaces(g, map, cols, rows, th, atlas, theme, wallIdx) {
        const tex = clipFrame(atlas, 'tex-' + theme + '.t' + wallIdx);
        for (let cy = 0; cy < rows; cy++) {
            for (let cx = 0; cx < cols; cx++) {
                if (kindAt(map, cx, cy) !== 1 || !isFloor(kindAt(map, cx, cy + 1))) continue;
                const x = cx * CELL;
                const y = cy * CELL;
                if (tex) {
                    const hw = tex.f[2] / 2;
                    const hh = tex.f[3] / 2;
                    g.imageSmoothingEnabled = true;
                    g.drawImage(tex.img, tex.f[0] + (cx & 1) * hw, tex.f[1] + (hash2(cx, cy, 3) < 0.5 ? 0 : 1) * hh, hw, hh, x, y, CELL, CELL);
                } else {
                    g.fillStyle = th.face;
                    g.fillRect(x, y, CELL, CELL);
                    g.fillStyle = 'rgba(0,0,0,0.18)';
                    const off = (cy & 1) * (CELL / 2);
                    g.fillRect(x, y + CELL / 2, CELL, 2);
                    g.fillRect(x + off, y, 2, CELL / 2);
                    g.fillRect(x + ((off + CELL / 2) % CELL), y + CELL / 2, 2, CELL / 2);
                }
                const grad = g.createLinearGradient(0, y, 0, y + CELL);
                grad.addColorStop(0, 'rgba(0,0,0,0.42)');
                grad.addColorStop(0.45, 'rgba(0,0,0,0.05)');
                grad.addColorStop(1, 'rgba(0,0,0,0.16)');
                g.fillStyle = grad;
                g.fillRect(x, y, CELL, CELL);
                g.fillStyle = 'rgba(6,4,8,0.78)';
                g.fillRect(x, y, CELL, 4);
                g.fillStyle = 'rgba(255,255,255,0.12)';
                g.fillRect(x, y + 4, CELL, 1);
                if (!isFloor(kindAt(map, cx - 1, cy)) && kindAt(map, cx - 1, cy) !== 1) { g.fillStyle = 'rgba(0,0,0,0.55)'; g.fillRect(x, y, 3, CELL); }
                if (!isFloor(kindAt(map, cx + 1, cy)) && kindAt(map, cx + 1, cy) !== 1) { g.fillStyle = 'rgba(0,0,0,0.55)'; g.fillRect(x + CELL - 3, y, 3, CELL); }
            }
        }
    }

    // Lava cells: the mirrored molten texture (or a flat orange with hot blobs) and a dark basalt rim where it meets the floor.
    function lavaLayer(g, map, cols, rows, atlas) {
        let any = false;
        g.save();
        g.beginPath();
        for (let cy = 0; cy < rows; cy++) {
            for (let cx = 0; cx < cols; cx++) {
                if (kindAt(map, cx, cy) === 4) { g.rect(cx * CELL, cy * CELL, CELL, CELL); any = true; }
            }
        }
        if (!any) { g.restore(); return; }
        g.clip();
        g.fillStyle = LAVA.fill;
        g.fillRect(0, 0, cols * CELL, rows * CELL);
        const tex = clipFrame(atlas, 'tex-glut.t1');
        if (tex) {
            const span = BLOCK * CELL;
            g.imageSmoothingEnabled = true;
            for (let by = 0; by * span < rows * CELL; by++) {
                for (let bx = 0; bx * span < cols * CELL; bx++) {
                    g.save();
                    g.translate(bx * span + (bx & 1 ? span : 0), by * span + (by & 1 ? span : 0));
                    g.scale(bx & 1 ? -1 : 1, by & 1 ? -1 : 1);
                    g.drawImage(tex.img, tex.f[0], tex.f[1], tex.f[2], tex.f[3], 0, 0, span, span);
                    g.restore();
                }
            }
        } else {
            for (let cy = 0; cy < rows; cy++) {
                for (let cx = 0; cx < cols; cx++) {
                    if (kindAt(map, cx, cy) !== 4) continue;
                    const v = hash2(cx, cy, 17);
                    g.fillStyle = v < 0.5 ? 'rgba(80,10,0,' + (0.1 + v * 0.3).toFixed(3) + ')' : 'rgba(251,146,60,' + ((v - 0.5) * 0.5).toFixed(3) + ')';
                    g.beginPath();
                    g.ellipse(cx * CELL + CELL * (0.3 + v * 0.4), cy * CELL + CELL * (0.3 + hash2(cx, cy, 19) * 0.4), 10 + v * 14, 7 + v * 9, v * 3, 0, Math.PI * 2);
                    g.fill();
                }
            }
        }
        // Dark rim along every edge that meets non-lava, strongest where a wall stands north of the lava.
        for (let cy = 0; cy < rows; cy++) {
            for (let cx = 0; cx < cols; cx++) {
                if (kindAt(map, cx, cy) !== 4) continue;
                const x = cx * CELL;
                const y = cy * CELL;
                const edge = function (nx, ny, gx, gy, rx, ry, rw, rh) {
                    if (kindAt(map, nx, ny) === 4) return;
                    const gr = g.createLinearGradient(x + gx[0], y + gy[0], x + gx[1], y + gy[1]);
                    gr.addColorStop(0, 'rgba(26,8,4,0.78)');
                    gr.addColorStop(1, 'rgba(26,8,4,0)');
                    g.fillStyle = gr;
                    g.fillRect(x + rx, y + ry, rw, rh);
                };
                edge(cx, cy - 1, [0, 0], [0, 14], 0, 0, CELL, 14);
                edge(cx, cy + 1, [0, 0], [CELL, CELL - 10], 0, CELL - 10, CELL, 10);
                edge(cx - 1, cy, [0, 10], [0, 0], 0, 0, 10, CELL);
                edge(cx + 1, cy, [CELL, CELL - 10], [0, 0], CELL - 10, 0, 10, CELL);
            }
        }
        g.restore();
    }

    // Faint icy sheen on the plain floor cells of a slippery room (rough ',' cells stay matte).
    function slipperySheen(g, map, cols, rows) {
        g.save();
        for (let cy = 0; cy < rows; cy++) {
            for (let cx = 0; cx < cols; cx++) {
                if (kindAt(map, cx, cy) !== 2) continue;
                const x = cx * CELL;
                const y = cy * CELL;
                const gr = g.createLinearGradient(x, y, x + CELL, y + CELL);
                gr.addColorStop(0, 'rgba(225, 245, 255, 0.16)');
                gr.addColorStop(0.45, 'rgba(225, 245, 255, 0.04)');
                gr.addColorStop(1, 'rgba(225, 245, 255, 0.10)');
                g.fillStyle = gr;
                g.fillRect(x, y, CELL, CELL);
                g.fillStyle = 'rgba(255, 255, 255, 0.10)';
                g.fillRect(x + 6, y + 6, 14, 2);
                g.fillRect(x + 28, y + 30, 9, 2);
            }
        }
        g.restore();
    }

    // Soft shadows on floor cells next to walls: strongest under the north wall.
    function contactShadows(g, map, cols, rows) {
        const wallish = function (cx, cy) { return !isFloor(kindAt(map, cx, cy)); };
        for (let cy = 0; cy < rows; cy++) {
            for (let cx = 0; cx < cols; cx++) {
                if (!isFloor(kindAt(map, cx, cy))) continue;
                const x = cx * CELL;
                const y = cy * CELL;
                let gr;
                if (wallish(cx, cy - 1)) {
                    gr = g.createLinearGradient(0, y, 0, y + 36);
                    gr.addColorStop(0, 'rgba(0,0,0,0.58)');
                    gr.addColorStop(1, 'rgba(0,0,0,0)');
                    g.fillStyle = gr;
                    g.fillRect(x, y, CELL, 36);
                }
                if (wallish(cx - 1, cy)) {
                    gr = g.createLinearGradient(x, 0, x + 16, 0);
                    gr.addColorStop(0, 'rgba(0,0,0,0.34)');
                    gr.addColorStop(1, 'rgba(0,0,0,0)');
                    g.fillStyle = gr;
                    g.fillRect(x, y, 16, CELL);
                }
                if (wallish(cx + 1, cy)) {
                    gr = g.createLinearGradient(x + CELL, 0, x + CELL - 16, 0);
                    gr.addColorStop(0, 'rgba(0,0,0,0.34)');
                    gr.addColorStop(1, 'rgba(0,0,0,0)');
                    g.fillStyle = gr;
                    g.fillRect(x + CELL - 16, y, 16, CELL);
                }
                if (wallish(cx, cy + 1)) {
                    gr = g.createLinearGradient(0, y + CELL, 0, y + CELL - 12);
                    gr.addColorStop(0, 'rgba(0,0,0,0.26)');
                    gr.addColorStop(1, 'rgba(0,0,0,0)');
                    g.fillStyle = gr;
                    g.fillRect(x, y + CELL - 12, CELL, 12);
                }
            }
        }
    }

    // Bakes the whole room into one canvas holding SCALE pixels per world px; null without a DOM.
    function bakeRoom(room, atlas, makeCanvas) {
        try {
            const def = room && room.def;
            const map = def && def.map;
            if (!room || !map || typeof makeCanvas !== 'function') return null;
            const cols = room.cols || (map[0] ? map[0].length : 0);
            const rows = room.rows || map.length;
            const cv = makeCanvas(cols * CELL * SCALE, rows * CELL * SCALE);
            const g = cv && cv.getContext ? cv.getContext('2d') : null;
            if (!g) return null;
            const theme = themeOf(room);
            const th = THEMES[theme];
            g.scale(SCALE, SCALE);
            g.fillStyle = '#000';
            g.fillRect(0, 0, cols * CELL, rows * CELL);
            // The glut floors use t0 only: tex-glut.t1 is the molten lava.
            const floorIdx = theme !== 'glut' && def.floor === 1 ? 1 : 0;
            const altIdx = theme === 'glut' ? 0 : (def.floorAlt === 1 ? 1 : (def.floorAlt === 0 ? 0 : 1));
            floorLayer(g, map, cols, rows, 0, clipFrame(atlas, 'tex-' + theme + '.t' + floorIdx), th.floor, theme);
            floorLayer(g, map, cols, rows, 1, clipFrame(atlas, 'tex-' + theme + '.t' + altIdx), th.alt, theme);
            lavaLayer(g, map, cols, rows, atlas);
            if (def.slippery) slipperySheen(g, map, cols, rows);
            wallTops(g, map, cols, rows, th, theme);
            wallFaces(g, map, cols, rows, th, atlas, theme, def.wall === 3 ? 3 : 2);
            contactShadows(g, map, cols, rows);
            const props = room.props || [];
            for (let i = 0; i < props.length; i++) {
                if (props[i].layer === 'floor') drawProp(g, atlas, props[i], { screen: SCALE });
            }
            cv.rpScale = SCALE;
            return cv;
        } catch (err) {
            return null;
        }
    }

    root.RunenpfadInteriorBake = {
        CELL: CELL,
        SCALE: SCALE,
        THEMES: THEMES,
        bakeRoom: bakeRoom,
        signature: signature,
        sheetsOf: sheetsOf,
        drawProp: drawProp,
        clipFrame: clipFrame,
        roundRect: roundRect,
        hash2: hash2
    };
})(typeof window !== 'undefined' ? window : globalThis);
