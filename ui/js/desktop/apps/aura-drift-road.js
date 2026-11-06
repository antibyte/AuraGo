/* Aura Drift - 3D road: ribbon, surface zones, tile textures, kerbs, shoulders, barriers, start line.
 *
 * Pure part (node): AD.roadRibbon. The rest needs THREE. The surface under each half of every segment
 * comes from AD.surfaceAt(track, s, +-1), so what you see is what the physics uses. The barrier sits on
 * the corridor edge (hw+0.05) where resolveWall stops the car (decision D8). Sim metres map to
 * three x,z with y up; the road lies at y=0. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const W = AD.world;
    const clamp = AD.clamp;
    const lerp = (a, b, t) => a + (b - a) * t;
    const sstep = W.sstep;
    const TILE_V = 18;

    /* ---- ribbon (pure) --------------------------------------------------------------------------- */
    /* Two columns (left/right edge at halfWidth+grow), one ring per sample plus a closing ring that
       coincides with ring 0; v runs 0..V where V is an integer so a texture seam closes. */
    AD.roadRibbon = function (track, grow, vPeriod) {
        const S = track.samples, N = S.length, V = Math.max(1, Math.round(track.length / (vPeriod || TILE_V)));
        const positions = new Float32Array((N + 1) * 2 * 3), uvs = new Float32Array((N + 1) * 2 * 2);
        const indices = new Uint32Array(N * 6);
        for (let k = 0; k <= N; k++) {
            const s = S[k % N], v = k === N ? V : s.s / track.length * V;
            for (let c = 0; c < 2; c++) {
                const side = c === 0 ? -1 : 1, off = side * (s.halfWidth + (grow || 0)), o = k * 2 + c;
                positions[o * 3] = s.x + s.nx * off; positions[o * 3 + 1] = 0; positions[o * 3 + 2] = s.y + s.ny * off;
                uvs[o * 2] = c; uvs[o * 2 + 1] = v;
            }
            if (k < N) {
                const a = k * 2, b = a + 1, c2 = a + 2, d = a + 3, i = k * 6;
                indices[i] = a; indices[i + 1] = c2; indices[i + 2] = b; indices[i + 3] = b; indices[i + 4] = c2; indices[i + 5] = d;
            }
        }
        return { positions: positions, uvs: uvs, indices: indices, vTotal: V };
    };

    function ringTable(track) {
        const S = track.samples, N = S.length, R = { N: N, L: track.length, x: [], z: [], nx: [], nz: [], tx: [], tz: [], hw: [], s: [] };
        for (let k = 0; k <= N; k++) {
            const s = S[k % N];
            R.x.push(s.x); R.z.push(s.y); R.nx.push(s.nx); R.nz.push(s.ny); R.tx.push(s.tx); R.tz.push(s.ty);
            R.hw.push(s.halfWidth); R.s.push(k === N ? track.length : s.s);
        }
        return R;
    }
    function ringAt(R, sv) {
        let lo = 0, hi = R.N;
        while (lo < hi - 1) { const m = (lo + hi) >> 1; if (R.s[m] <= sv) lo = m; else hi = m; }
        const t = clamp((sv - R.s[lo]) / ((R.s[lo + 1] - R.s[lo]) || 1), 0, 1), k = lo;
        return {
            k: k, x: lerp(R.x[k], R.x[k + 1], t), z: lerp(R.z[k], R.z[k + 1], t), nx: lerp(R.nx[k], R.nx[k + 1], t), nz: lerp(R.nz[k], R.nz[k + 1], t),
            tx: lerp(R.tx[k], R.tx[k + 1], t), tz: lerp(R.tz[k], R.tz[k + 1], t), hw: lerp(R.hw[k], R.hw[k + 1], t)
        };
    }

    /* ---- geometry helpers ------------------------------------------------------------------------ */
    function makeGeo(T, o) {
        const g = new T.BufferGeometry();
        g.setAttribute('position', new T.BufferAttribute(new Float32Array(o.pos), 3));
        if (o.col) g.setAttribute('color', new T.BufferAttribute(new Float32Array(o.col), 3));
        if (o.uv) g.setAttribute('uv', new T.BufferAttribute(new Float32Array(o.uv), 2));
        const big = o.pos.length / 3 > 65535;
        g.setIndex(new T.BufferAttribute(big ? new Uint32Array(o.idx) : new Uint16Array(o.idx), 1));
        if (o.normals) g.computeVertexNormals();
        g.computeBoundingSphere();
        return g;
    }
    function bag() { return { pos: [], col: [], uv: [], idx: [], n: 0 }; }
    function vert(b, x, y, z, c, u, v) {
        b.pos.push(x, y, z); b.col.push(c[0], c[1], c[2]); b.uv.push(u, v);
        return b.n++;
    }
    function quad(b, a, bb, c, d) { b.idx.push(a, bb, c, a, c, d); }

    /* ---- textures ---------------------------------------------------------------------------------- */
    const N = 512;
    function canvas(n, h) { const c = document.createElement('canvas'); c.width = n; c.height = h || n; return c; }
    function texFrom(T, c, srgb, repeatU) {
        const t = new T.CanvasTexture(c);
        if (srgb) t.colorSpace = T.SRGBColorSpace;
        t.wrapS = repeatU ? T.RepeatWrapping : T.ClampToEdgeWrapping;
        t.wrapT = T.RepeatWrapping;
        t.anisotropy = 8;
        return t;
    }
    const cov = (d, half, px) => clamp((half - d) * px + 0.5, 0, 1);

    /* One factory per surface. A pixel function gets u (across 0..1), v (along 0..1 tile), the tile
       noise and writes o.r/g/b (0..255) and o.k (roughness 0..1). */
    function surfaceFactory(kind, p) {
        const base = p.base;
        const lanes = (u, a, b, w) => Math.max(Math.exp(-Math.pow((u - a) / w, 2)), Math.exp(-Math.pow((u - b) / w, 2)));
        const paintLines = (o, u, v, lo) => {
            if (!p.lines) return;
            const wear = 0.5 + 0.5 * sstep(0.3, 0.46, lo);
            let c = Math.max(cov(Math.abs(u - 0.075), 0.0065, N), cov(Math.abs(u - 0.925), 0.0065, N));
            const fv = (v * p.dashes) % 1, inDash = Math.min(fv, 0.34 - fv);
            c = Math.max(c, cov(Math.abs(u - 0.5), 0.0055, N) * clamp(inDash * N / p.dashes + 0.5, 0, 1));
            c *= wear * p.lineStrength;
            if (c <= 0) return;
            o.r = lerp(o.r, p.line[0], c); o.g = lerp(o.g, p.line[1], c); o.b = lerp(o.b, p.line[2], c);
            o.k = lerp(o.k, 0.52, c);
        };
        switch (kind) {
            case 'wet': return (u, v, lo, hi, o) => {
                const wheel = lanes(u, 0.27, 0.73, 0.07), pud = sstep(0.5, 0.62, lo + 0.1 * (hi - 0.5));
                const m = (0.86 + 0.3 * (lo - 0.5) + 0.3 * (hi - 0.5) - 0.1 * wheel) * (1 - 0.32 * pud);
                o.r = base[0] * m; o.g = base[1] * m; o.b = base[2] * m * (1 + 0.1 * pud);
                o.k = lerp(0.5 + 0.2 * (hi - 0.5), 0.05, Math.max(pud, wheel * 0.5));
                paintLines(o, u, v, lo);
            };
            case 'gravel': return (u, v, lo, hi, o) => {
                const rut = lanes(u, 0.3, 0.7, 0.05), pebble = 0.6 + 0.8 * hi;
                const m = pebble * (0.9 + 0.3 * (lo - 0.5)) * (1 - 0.18 * rut);
                const warm = 0.9 + 0.2 * lo;
                o.r = base[0] * m * warm; o.g = base[1] * m; o.b = base[2] * m * (2 - warm);
                o.k = 0.95 - 0.1 * (hi > 0.8 ? 1 : 0);
            };
            case 'sand': return (u, v, lo, hi, o) => {
                const rip = 0.06 * Math.sin((v * 26 + lo * 3.2) * 6.283), rut = lanes(u, 0.3, 0.7, 0.05);
                const m = 0.95 + rip + 0.1 * (hi - 0.5) + 0.12 * (lo - 0.5) - 0.12 * rut;
                o.r = base[0] * m; o.g = base[1] * m; o.b = base[2] * m * (1 - 0.06 * rut);
                o.k = 0.97 - 0.2 * rut;
            };
            case 'mud': return (u, v, lo, hi, o) => {
                const rut = lanes(u, 0.3, 0.7, 0.055), pud = sstep(0.56, 0.66, lo) * 0.8 + rut * 0.6;
                const m = (0.92 + 0.4 * (lo - 0.5) + 0.26 * (hi - 0.5)) * (1 - 0.28 * clamp(pud, 0, 1));
                o.r = base[0] * m; o.g = base[1] * m; o.b = base[2] * m * (1 + 0.2 * clamp(pud, 0, 1));
                o.k = lerp(0.78, 0.07, clamp(pud, 0, 1));
            };
            case 'snow': return (u, v, lo, hi, o) => {
                const track = lanes(u, 0.3, 0.7, 0.05), shade = sstep(0.34, 0.5, lo);
                let m = 0.97 + 0.05 * (hi - 0.5) - 0.11 * track;
                const blue = (1 - shade) * 0.1;
                if (hi > 0.93) m += 0.1;
                o.r = base[0] * m * (1 - blue * 1.2); o.g = base[1] * m * (1 - blue * 0.5); o.b = Math.min(255, base[2] * m);
                o.k = hi > 0.93 ? 0.3 : 0.62 - 0.12 * track;
            };
            case 'ice': return (u, v, lo, hi, o) => {
                const crack = p.crack ? p.crack[(Math.floor(v * N) % N) * N + Math.floor(u * N)] : 0;
                const m = 0.88 + 0.5 * (lo - 0.5) + 0.12 * (hi - 0.5) + (crack < -0.2 ? 0.45 : 0);
                o.r = base[0] * m * 0.92; o.g = base[1] * m; o.b = Math.min(255, base[2] * m * 1.04);
                o.k = 0.1 + 0.16 * hi + (crack < -0.2 ? 0.35 : 0);
            };
            case 'salt': return (u, v, lo, hi, o) => {
                const crack = p.crack ? p.crack[(Math.floor(v * N) % N) * N + Math.floor(u * N)] : 0;
                const wheel = lanes(u, 0.27, 0.73, 0.07);
                const m = 0.95 + 0.12 * (lo - 0.5) + 0.1 * (hi - 0.5) + crack - 0.06 * wheel;
                o.r = base[0] * m; o.g = base[1] * m * 0.99; o.b = base[2] * m * (0.97 + 0.03 * lo);
                o.k = 0.62 + 0.2 * (hi - 0.5) + (crack < -0.2 ? 0.25 : 0) - 0.15 * wheel;
                paintLines(o, u, v, lo);
            };
            default: return (u, v, lo, hi, o) => { /* asphalt */
                const wheel = lanes(u, 0.27, 0.73, 0.075);
                let m = 0.94 + 0.34 * (lo - 0.5) + 0.42 * (hi - 0.5) - 0.13 * wheel;
                if (hi > 0.82) m += 0.12 + 0.5 * (hi - 0.82);
                else if (hi < 0.14) m -= 0.1;
                o.r = base[0] * m; o.g = base[1] * m; o.b = base[2] * m * (1 + 0.03 * (lo - 0.5));
                o.k = 0.9 + 0.16 * (hi - 0.5) - 0.22 * wheel;
                paintLines(o, u, v, lo);
            };
        }
    }

    /* Cracks, patches and tar seams drawn over the pixel pass; the same shapes go on both canvases. */
    function overlay(g, mode, seed, p) {
        const r = AD.rng(seed), rd = a => Math.floor(r() * a);
        const a = mode === 'a';
        for (let i = 0; i < p.patches; i++) {
            const x = 20 + rd(N - 150), y = 20 + rd(N - 90), w = 50 + rd(110), h = 24 + rd(60);
            const dark = r() < 0.6;
            g.fillStyle = a ? (dark ? 'rgba(0,0,0,0.1)' : 'rgba(255,255,255,0.06)') : 'rgba(215,215,215,0.9)';
            g.fillRect(x, y, w, h);
            g.strokeStyle = a ? 'rgba(0,0,0,0.26)' : 'rgb(255,255,255)';
            g.lineWidth = 1;
            g.strokeRect(x + 0.5, y + 0.5, w, h);
        }
        g.lineCap = 'round';
        for (let i = 0; i < p.seams; i++) {
            const y = 30 + rd(N - 60);
            g.strokeStyle = a ? 'rgba(8,8,10,0.55)' : 'rgb(255,255,255)';
            g.lineWidth = 2;
            g.beginPath(); g.moveTo(0, y);
            for (let x = 0; x <= N; x += 32) g.lineTo(x, y + (r() - 0.5) * 3);
            g.stroke();
        }
        for (let i = 0; i < p.cracks; i++) {
            let x = 30 + rd(N - 60), y = 30 + rd(N - 60), ang = r() * 6.283;
            const len = 6 + rd(16);
            g.strokeStyle = a ? 'rgba(6,6,8,0.6)' : 'rgb(255,255,255)';
            g.lineWidth = 0.6 + r() * 0.7;
            g.beginPath(); g.moveTo(x, y);
            for (let s = 0; s < len; s++) {
                ang += (r() - 0.5) * 1.1;
                x += Math.cos(ang) * 7; y += Math.sin(ang) * 7;
                g.lineTo(x, y);
                if (r() < 0.12) { g.moveTo(x, y); const bx = x + Math.cos(ang + 1) * 18, by = y + Math.sin(ang + 1) * 18; g.lineTo(bx, by); g.moveTo(x, y); }
            }
            g.stroke();
        }
    }

    function makeSurfaceTexture(T, kind, biome, look, lines, dashes) {
        const tn = W.tileNoise();
        const roadRgb = W.hexRgb(biome.road), lineRgb = W.hexRgb(biome.line);
        const tint = (c, k) => c.map(v => clamp(v * k, 0, 255));
        const p = { dashes: dashes, lines: lines, line: lineRgb, lineStrength: 1, patches: 0, seams: 0, cracks: 0 };
        const nk = look.night ? 1.5 : 1;
        if (kind === 'asphalt') { p.base = tint(roadRgb, 1.5 * nk); p.patches = 8; p.seams = 4; p.cracks = 26; }
        else if (kind === 'wet') { p.base = tint(roadRgb, 1.35 * nk); p.patches = 5; p.seams = 3; p.cracks = 14; }
        else if (kind === 'gravel') p.base = [150, 141, 128];
        else if (kind === 'sand') p.base = [226, 190, 128];
        else if (kind === 'mud') p.base = [92, 70, 50];
        else if (kind === 'snow') p.base = [242, 247, 253];
        else if (kind === 'ice') { p.base = [158, 208, 238]; p.crack = W.voronoi(N, 9, 31); }
        else if (kind === 'salt') { p.base = tint(roadRgb, 1.1); p.crack = W.voronoi(N, 22, 5); p.patches = 2; p.cracks = 4; }
        else p.base = tint(roadRgb, 1.4);
        const fn = surfaceFactory(kind, p);
        const ca = canvas(N), cr = canvas(N), ga = ca.getContext('2d'), gr = cr.getContext('2d');
        const ia = ga.createImageData(N, N), ir = gr.createImageData(N, N), da = ia.data, dr = ir.data, o = { r: 0, g: 0, b: 0, k: 0 };
        for (let j = 0; j < N; j++) {
            for (let i = 0; i < N; i++) {
                const q = j * N + i;
                fn(i / N, j / N, tn.lo[q], tn.hi[q], o);
                da[q * 4] = o.r; da[q * 4 + 1] = o.g; da[q * 4 + 2] = o.b; da[q * 4 + 3] = 255;
                const k = clamp(o.k, 0, 1) * 255;
                dr[q * 4] = dr[q * 4 + 1] = dr[q * 4 + 2] = k; dr[q * 4 + 3] = 255;
            }
        }
        ga.putImageData(ia, 0, 0); gr.putImageData(ir, 0, 0);
        if (p.patches || p.cracks) {
            const seed = AD.hashString(biome.road + kind);
            overlay(ga, 'a', seed, p);
            overlay(gr, 'r', seed, p);
        }
        return { map: texFrom(T, ca, true, false), rough: texFrom(T, cr, false, false) };
    }

    /* Small wall textures: concrete panels with a paint band, a tyre stack. */
    function concreteTexture(T, base, band) {
        const c = canvas(256), g = c.getContext('2d'), r = AD.rng(77);
        g.fillStyle = base; g.fillRect(0, 0, 256, 256);
        for (let i = 0; i < 1400; i++) {
            const v = r();
            g.fillStyle = v < 0.5 ? 'rgba(0,0,0,0.07)' : 'rgba(255,255,255,0.06)';
            g.fillRect(r() * 256, r() * 256, 1 + r() * 3, 1 + r() * 3);
        }
        for (let i = 0; i < 9; i++) { g.fillStyle = 'rgba(30,24,16,0.10)'; g.fillRect(r() * 256, 0, 2 + r() * 8, 256 * r()); }
        g.fillStyle = band; g.fillRect(0, 100, 256, 34);
        g.fillStyle = 'rgba(255,255,255,0.85)'; g.fillRect(0, 98, 256, 3); g.fillRect(0, 133, 256, 3);
        g.fillStyle = 'rgba(0,0,0,0.5)'; g.fillRect(0, 0, 3, 256); g.fillRect(253, 0, 3, 256);
        return texFrom(T, c, true, true);
    }
    function tyreTexture(T) {
        const c = canvas(256), g = c.getContext('2d'), r = AD.rng(91);
        g.fillStyle = '#0c0c0e'; g.fillRect(0, 0, 256, 256);
        for (let row = 0; row < 3; row++) {
            for (let col = 0; col < 3; col++) {
                const cx = (col + (row & 1 ? 0.5 : 0)) * 85.3 + 6, cy = row * 85.3 + 42, rad = 40;
                g.fillStyle = '#1d1d21'; g.beginPath(); g.arc(cx, cy, rad, 0, 6.283); g.fill();
                g.strokeStyle = '#36363c'; g.lineWidth = 7; g.beginPath(); g.arc(cx, cy, rad - 6, 0, 6.283); g.stroke();
                g.fillStyle = '#050506'; g.beginPath(); g.arc(cx, cy, 15, 0, 6.283); g.fill();
                const paint = r();
                if (paint < 0.3) { g.strokeStyle = '#f2f1ea'; g.lineWidth = 9; g.beginPath(); g.arc(cx, cy, rad - 7, 3.6, 5.8); g.stroke(); }
                else if (paint < 0.45) { g.strokeStyle = '#d93a2b'; g.lineWidth = 9; g.beginPath(); g.arc(cx, cy, rad - 7, 0.4, 2.7); g.stroke(); }
            }
        }
        return texFrom(T, c, true, true);
    }
    function checkerTexture(T, cols) {
        const c = canvas(cols * 8, 16), g = c.getContext('2d');
        for (let j = 0; j < 2; j++) for (let i = 0; i < cols; i++) {
            g.fillStyle = (i + j) & 1 ? '#f7f5ee' : '#16161a';
            g.fillRect(i * 8, j * 8, 8, 8);
        }
        const t = new T.CanvasTexture(c);
        t.colorSpace = T.SRGBColorSpace; t.magFilter = T.NearestFilter; t.anisotropy = 8;
        return t;
    }

    /* ---- barrier styles --------------------------------------------------------------------------- */
    const STYLE = { alpine: 'armco', industry: 'hazard', forest: 'tyres', metro: 'concrete', harbour: 'concrete', glacier: 'snow', salt: 'rope', desert: 'rock' };
    const P = (l, y, u) => ({ l: l, y: y, u: u || 0 });
    const PROFILES = {
        armco: [P(0, 0.30), P(-0.09, 0.38), P(0, 0.46), P(-0.09, 0.55), P(0, 0.64), P(-0.09, 0.73), P(0, 0.82), P(-0.04, 0.92)],
        concrete: [P(-0.38, 0, 0), P(-0.31, 0.1, 0.09), P(-0.17, 0.42, 0.38), P(-0.12, 0.88, 0.8), P(-0.12, 1.1, 1), P(0.12, 1.1, 1), P(0.12, 0, 0)],
        tyres: [P(-0.34, 0, 0), P(-0.41, 0.3, 0.27), P(-0.35, 0.56, 0.5), P(-0.41, 0.84, 0.76), P(-0.3, 1.1, 1), P(0.05, 1.15, 1), P(0.3, 0.9, 0.8), P(0.3, 0, 0)],
        snow: [P(-0.95, 0), P(-0.62, 0.38), P(-0.3, 0.68), P(0.1, 0.82), P(0.55, 0.62), P(1.1, 0)],
        rock: [P(-0.55, 0), P(-0.64, 0.45), P(-0.36, 1.0), P(0.1, 1.0), P(0.5, 0.7), P(0.72, 0)]
    };
    PROFILES.hazard = PROFILES.armco;

    function buildWalls(T, R, biomeId, biome, look, own) {
        const style = STYLE[biomeId] || 'armco', prof = PROFILES[style] || PROFILES.armco;
        const b = bag(), Nn = R.N, tint = W.hexLin;
        const accent = W.hexLin(biome.accent), rr = AD.rng(AD.hashString(biomeId + '/wall'));
        const jag = new Float32Array(Nn + 1), hv = new Float32Array(Nn + 1);
        for (let k = 0; k <= Nn; k++) {
            const kk = k % Nn;
            jag[k] = rr(); hv[k] = style === 'snow' ? 0.8 + 0.55 * W.noise2(kk / 9, 3.3) : (style === 'rock' ? 0.85 + 0.6 * jag[kk] : 1);
        }
        hv[Nn] = hv[0]; jag[Nn] = jag[0];
        const lin = { silver: tint('#b4c0ce'), silver2: tint('#8d99a8'), yellow: tint('#f2b51a'), black: tint('#1c1c1e'), white: tint('#f4f4f6'), red: tint('#e0392e') };
        const rock = [tint('#d08a4e'), tint('#a96a3b'), tint('#e0a56a'), tint('#8f5632')];
        const tiles = Math.max(1, Math.round(R.L / 4));
        const col = [1, 1, 1];
        function colour(k, pi, y) {
            const kk = k % Nn;
            switch (style) {
                case 'armco': { const c = (Math.floor(k / 2) & 1) ? lin.silver : lin.silver2; return c; }
                case 'hazard': return (Math.floor(k / 1) % 2) ? lin.yellow : lin.black;
                case 'snow': { const t = clamp(y / 0.8, 0, 1); col[0] = lerp(0.58, 0.93, t); col[1] = lerp(0.68, 0.97, t); col[2] = lerp(0.85, 1, t); return col; }
                case 'rock': { const c = rock[(kk + pi) % 4], t = 0.55 + 0.45 * clamp(y / 1.3, 0, 1); col[0] = c[0] * t; col[1] = c[1] * t; col[2] = c[2] * t; return col; }
                default: return [1, 1, 1];
            }
        }
        for (let side = -1; side <= 1; side += 2) {
            const base = b.n;
            for (let k = 0; k <= Nn; k++) {
                const kk = k % Nn, edge = R.hw[k] + 0.05;
                for (let pi = 0; pi < prof.length; pi++) {
                    const pt = prof[pi];
                    let lat = edge + pt.l, y = pt.y * hv[k];
                    if (style === 'rock' && pi > 0 && pi < prof.length - 1) {
                        const jr = jag[(k * 7 + pi * 13) % Nn], jr2 = jag[(k * 5 + pi * 29 + 3) % Nn];
                        lat += (jr - 0.5) * 0.9;
                        y *= 0.55 + 1.0 * jr2;
                    }
                    if (style === 'snow') lat += (W.noise2(kk / 6, pi * 1.7) - 0.5) * 0.5 * (pi > 0 && pi < prof.length - 1 ? 1 : 0);
                    const c = colour(k, pi, y);
                    vert(b, R.x[k] + R.nx[k] * side * lat, y, R.z[k] + R.nz[k] * side * lat, c, R.s[k] / R.L * tiles, pt.u);
                }
            }
            const P2 = prof.length;
            for (let k = 0; k < Nn; k++) for (let pi = 0; pi < P2 - 1; pi++) {
                const a = base + k * P2 + pi, a2 = base + (k + 1) * P2 + pi;
                quad(b, a, a2, a2 + 1, a + 1);
            }
        }
        const geo = own(makeGeo(T, { pos: b.pos, col: b.col, uv: b.uv, idx: b.idx, normals: style !== 'rock' }));
        let mat, tex = null;
        if (style === 'concrete') {
            tex = own(concreteTexture(T, biomeId === 'harbour' ? '#7d8794' : '#8e8d93', '#' + new T.Color().setRGB(accent[0], accent[1], accent[2]).getHexString()));
            mat = new T.MeshStandardMaterial({ map: tex, roughness: 0.82, metalness: 0, side: T.DoubleSide });
        } else if (style === 'tyres') {
            tex = own(tyreTexture(T));
            mat = new T.MeshStandardMaterial({ map: tex, roughness: 0.9, metalness: 0, side: T.DoubleSide });
        } else if (style === 'armco' || style === 'hazard') {
            mat = new T.MeshStandardMaterial({ vertexColors: true, roughness: 0.4, metalness: 0.65, side: T.DoubleSide });
        } else if (style === 'rock') {
            mat = new T.MeshStandardMaterial({ vertexColors: true, roughness: 0.95, flatShading: true, side: T.DoubleSide });
        } else if (style === 'snow') {
            mat = new T.MeshStandardMaterial({ vertexColors: true, roughness: 0.7, side: T.DoubleSide });
        } else mat = new T.MeshStandardMaterial({ vertexColors: true, roughness: 0.7, side: T.DoubleSide });
        own(mat);
        const mesh = new T.Mesh(geo, mat);
        mesh.castShadow = true; mesh.receiveShadow = true; mesh.name = 'barrier-' + style;
        const out = [mesh];
        const dummy = new T.Object3D();
        function posts(step, geoP, matP, h, color) {
            const per = Math.ceil(Nn / step), im = new T.InstancedMesh(geoP, matP, per * 2);
            let n = 0;
            for (let side = -1; side <= 1; side += 2) for (let k = 0; k < Nn; k += step) {
                const lat = R.hw[k] + 0.08;
                dummy.position.set(R.x[k] + R.nx[k] * side * lat, h / 2, R.z[k] + R.nz[k] * side * lat);
                dummy.rotation.set(0, Math.atan2(-R.tz[k], R.tx[k]), 0);
                dummy.updateMatrix();
                im.setMatrixAt(n, dummy.matrix);
                if (color) im.setColorAt(n, color(n, k));
                n++;
            }
            im.count = n; im.instanceMatrix.needsUpdate = true;
            if (im.instanceColor) im.instanceColor.needsUpdate = true;
            im.castShadow = true; im.receiveShadow = true; im.frustumCulled = false;
            return im;
        }
        if (style === 'armco' || style === 'hazard') {
            const g = own(new T.BoxGeometry(0.12, 1.0, 0.12)), m = own(new T.MeshStandardMaterial({ color: style === 'hazard' ? 0x2a2a2d : 0x8d949c, roughness: 0.5, metalness: 0.6 }));
            out.push(posts(1, g, m, 1.0));
        }
        if (style === 'rope') {
            const g = own(new T.CylinderGeometry(0.06, 0.08, 1.0, 6)), m = own(new T.MeshStandardMaterial({ color: 0xffffff, roughness: 0.6 }));
            const cc = new T.Color();
            out.push(posts(2, g, m, 1.0, (n, k) => cc.setRGB(...((Math.floor(k / 2) & 1) ? lin.white : lin.red))));
            /* the rope itself: a thin hexagonal tube that sags between the posts */
            const rb = bag(), seg = 6, rad = 0.035;
            for (let side = -1; side <= 1; side += 2) {
                const base = rb.n;
                for (let k = 0; k <= Nn; k++) {
                    const sag = (k & 1) ? 0.7 : 0.92, c = (Math.floor(k / 2) & 1) ? lin.red : lin.white;
                    const lat = R.hw[k] + 0.08;
                    for (let a = 0; a < seg; a++) {
                        const an = a / seg * 6.283, dl = Math.cos(an) * rad, dy = Math.sin(an) * rad;
                        const l2 = lat + dl;
                        vert(rb, R.x[k] + R.nx[k] * side * l2, sag + dy, R.z[k] + R.nz[k] * side * l2, c, 0, 0);
                    }
                }
                for (let k = 0; k < Nn; k++) for (let a = 0; a < seg; a++) {
                    const a0 = base + k * seg + a, a1 = base + k * seg + (a + 1) % seg;
                    quad(rb, a0, a1, a1 + seg, a0 + seg);
                }
            }
            const rg = own(makeGeo(T, { pos: rb.pos, col: rb.col, uv: rb.uv, idx: rb.idx, normals: true }));
            const rm = own(new T.MeshStandardMaterial({ vertexColors: true, roughness: 0.6, side: T.DoubleSide }));
            const rmesh = new T.Mesh(rg, rm);
            rmesh.castShadow = true;
            out.push(rmesh);
        }
        if (style === 'concrete') {
            /* neon cap line along the top of the wall in the biome accent colour */
            const nb = bag();
            for (let side = -1; side <= 1; side += 2) {
                const base = nb.n;
                for (let k = 0; k <= Nn; k++) {
                    const lat = R.hw[k] - 0.1;
                    for (let r = 0; r < 2; r++) {
                        const y = 1.02 + r * 0.07;
                        vert(nb, R.x[k] + R.nx[k] * side * lat, y, R.z[k] + R.nz[k] * side * lat, accent, 0, 0);
                    }
                }
                for (let k = 0; k < Nn; k++) quad(nb, base + k * 2, base + k * 2 + 1, base + k * 2 + 3, base + k * 2 + 2);
            }
            const ng = own(makeGeo(T, { pos: nb.pos, col: nb.col, idx: nb.idx }));
            const nm = own(new T.MeshBasicMaterial({ vertexColors: true, color: new T.Color(2.4, 2.4, 2.4), toneMapped: false, side: T.DoubleSide }));
            out.push(new T.Mesh(ng, nm));
        }
        return { meshes: out, style: style, triangles: geo.index.count / 3 };
    }

    /* ---- the road object ------------------------------------------------------------------------------ */
    const RUNOFF = { gravel: '#7d766a', sand: '#cdb07a', wet: '#1c2430', salt: '#e9e5da', snow: '#e6edf6', mud: '#4a3a2c', ice: '#a8cfe6', asphalt: '#33363c' };
    const LINED = { asphalt: true, wet: true, salt: true };
    const WET_MAT = {
        asphalt: { a: 1, b: 0.24, coat: 0.7, dark: 0.82 }, wet: { a: 0.8, b: 0.2, coat: 0.5, dark: 0.92 }, mud: { a: 1, b: 0.5, coat: 0, dark: 0.82 },
        gravel: { a: 1, b: 0.8, coat: 0, dark: 0.8 }, sand: { a: 1, b: 0.85, coat: 0, dark: 0.82 }, snow: { a: 1, b: 0.9, coat: 0, dark: 0.97 },
        ice: { a: 1, b: 0.75, coat: 0, dark: 1 }, salt: { a: 1, b: 0.45, coat: 0.4, dark: 0.88 }
    };

    AD.createRoad = function (T, track, look, q) {
        const t0 = typeof performance !== 'undefined' ? performance.now() : 0;
        const biomeId = track.biomeId, biome = track.biome, R = ringTable(track), Nn = R.N, L = R.L, S = track.samples;
        const group = new T.Group();
        group.name = 'road';
        const owned = [];
        const own = o => { owned.push(o); return o; };
        const stats = { triangles: 0, meshes: 0, ms: 0, kerbs: 0, barrier: '' };
        /* square texels: the tile is as long as the road is wide; ~9 m dash period, dashes per tile is an integer */
        let wsum = 0;
        for (let k = 0; k < Nn; k++) wsum += R.hw[k] * 2;
        const tileLen = wsum / Nn, V = Math.max(1, Math.round(L / tileLen)), dashes = Math.max(2, Math.round(L / V / 9));
        const ringV = k => (k === Nn ? V : R.s[k] / L * V);

        /* racing line darkening and edge dirt as vertex colours on five columns */
        const COLS = [-1, -0.5, 0, 0.5, 1];
        const turn = new Float32Array(Nn);
        for (let k = 0; k < Nn; k++) { const a = S[(k - 3 + Nn) % Nn], b = S[(k + 3) % Nn]; turn[k] = a.tx * b.ty - a.ty * b.tx; }
        function wear(k, f) {
            const kk = k % Nn, tr = turn[kk], mag = sstep(0.01, 0.06, Math.abs(tr));
            const line = Math.exp(-Math.pow((f - 0.55 * Math.sign(tr)) / 0.3, 2)) * mag * 0.26;
            const edge = 0.08 * sstep(0.7, 1, Math.abs(f));
            const n = 0.045 * (W.noise2(R.s[k] / 8, f * 2.6 + 4) - 0.5) * 2;
            return clamp(1 - line - edge + n, 0.5, 1.08);
        }

        /* surface meshes grouped by what AD.surfaceAt says for each half of each segment */
        const groups = {};
        const g = type => groups[type] || (groups[type] = bag());
        const colV = [];
        for (let k = 0; k <= Nn; k++) { const row = []; for (const f of COLS) row.push(wear(k, f)); colV.push(row); }
        const emit = (b, k, ci) => {
            const f = COLS[ci], w = colV[k][ci], hw = R.hw[k];
            return vert(b, R.x[k] + R.nx[k] * f * hw, 0, R.z[k] + R.nz[k] * f * hw, [w, w, w], (f + 1) / 2, ringV(k));
        };
        for (let k = 0; k < Nn; k++) {
            const mid = (R.s[k] + R.s[k + 1]) / 2;
            for (let h = -1; h <= 1; h += 2) {
                const type = AD.surfaceAt(track, mid, h).type, b = g(type);
                const cs = h > 0 ? [2, 3, 4] : [2, 1, 0];
                for (let c = 0; c < 2; c++) {
                    const a = emit(b, k, cs[c]), bb = emit(b, k + 1, cs[c]), cc = emit(b, k + 1, cs[c + 1]), d = emit(b, k, cs[c + 1]);
                    if (h > 0) quad(b, a, d, cc, bb); else quad(b, a, bb, cc, d);
                }
            }
        }
        const wetMats = [];
        const surfaceCoat = { dummy: 0 };
        void surfaceCoat;
        for (const type of Object.keys(groups)) {
            const kind = WET_MAT[type] ? type : 'asphalt';
            const tex = makeSurfaceTexture(T, kind, biome, look, !!LINED[kind], dashes);
            own(tex.map); own(tex.rough);
            const phys = WET_MAT[kind].coat > 0;
            const opts = { map: tex.map, roughnessMap: tex.rough, vertexColors: true, roughness: 1, metalness: 0 };
            const mat = own(phys ? new T.MeshPhysicalMaterial(Object.assign(opts, { clearcoat: 0.001, clearcoatRoughness: 0.08 })) : new T.MeshStandardMaterial(opts));
            const geo = own(makeGeo(T, { pos: groups[type].pos, col: groups[type].col, uv: groups[type].uv, idx: groups[type].idx }));
            const mesh = new T.Mesh(geo, mat);
            mesh.receiveShadow = true; mesh.name = 'road-' + type;
            group.add(mesh);
            wetMats.push({ mat: mat, kind: kind });
            stats.triangles += geo.index.count / 3; stats.meshes++;
        }

        /* red/white kerbs: the stripe pair period is length/round(length/11), sides staggered */
        {
            const pairs = Math.max(2, Math.round(L / 11)), half = L / pairs / 2, kb = bag();
            const red = W.hexLin('#e2362b'), white = W.hexLin('#f4f0e6');
            for (let side = -1; side <= 1; side += 2) {
                for (let n = 0; n < pairs * 2; n++) {
                    const s0 = n * half, s1 = (n + 1) * half, c = ((n + (side > 0 ? 0 : 1)) & 1) ? white : red;
                    const pts = [ringAt(R, s0)];
                    for (let k = ringAt(R, s0).k + 1; k < Nn && R.s[k] < s1 - 1e-4; k++) pts.push(ringAt(R, R.s[k]));
                    pts.push(ringAt(R, Math.min(s1, L - 1e-4)));
                    let prev = -1;
                    for (const pt of pts) {
                        const a = vert(kb, pt.x + pt.nx * side * (pt.hw - 1.25), 0.03, pt.z + pt.nz * side * (pt.hw - 1.25), c, 0, 0);
                        const bb = vert(kb, pt.x + pt.nx * side * (pt.hw - 0.2), 0.03, pt.z + pt.nz * side * (pt.hw - 0.2), c, 0, 0);
                        if (prev >= 0) quad(kb, prev, a, bb, prev + 1);
                        prev = a;
                    }
                }
            }
            const kg = own(makeGeo(T, { pos: kb.pos, col: kb.col, idx: kb.idx, normals: false }));
            const km = own(new T.MeshStandardMaterial({ vertexColors: true, roughness: 0.55, side: T.DoubleSide, polygonOffset: true, polygonOffsetFactor: -1, polygonOffsetUnits: -1 }));
            const kmesh = new T.Mesh(kg, km);
            kmesh.receiveShadow = true; kmesh.name = 'kerbs';
            group.add(kmesh);
            stats.triangles += kg.index.count / 3; stats.meshes++; stats.kerbs = pairs * 4;
        }

        /* shoulders: runoff colour at the barrier fading into the terrain colour, sloping down to -0.36 */
        {
            const pal = typeof AD.terrainPalette === 'function' ? AD.terrainPalette(biomeId, look) : null;
            const sb = bag(), inner = W.hexLin(RUNOFF[biome.runoff] || RUNOFF.gravel), tc = [0, 0, 0];
            for (let side = -1; side <= 1; side += 2) {
                const base = sb.n;
                for (let k = 0; k <= Nn; k++) {
                    const kk = k % Nn, hw = R.hw[k], cur = Math.max(1e-4, S[kk].curvature !== undefined ? S[kk].curvature : track.curvature[kk]);
                    const insideTurn = turn[kk] * side > 0;
                    const ext = insideTurn ? clamp(0.8 / cur - hw, 1.5, 10) : 10;
                    const cols = [[0, -0.02], [Math.min(2.4, ext * 0.3), -0.12], [ext, -0.36]];
                    for (let ci = 0; ci < 3; ci++) {
                        const lat = hw + cols[ci][0], x = R.x[k] + R.nx[k] * side * lat, z = R.z[k] + R.nz[k] * side * lat;
                        let c = inner;
                        if (pal) {
                            AD.terrainColor(pal, -0.4, 0.02, x, z, 10, tc);
                            const t = ci / 2, nj = 0.92 + 0.16 * W.noise2(x / 5, z / 5);
                            c = [lerp(inner[0], tc[0], t) * nj, lerp(inner[1], tc[1], t) * nj, lerp(inner[2], tc[2], t) * nj];
                        }
                        vert(sb, x, cols[ci][1], z, c, x / 32, z / 32);
                    }
                }
                for (let k = 0; k < Nn; k++) for (let ci = 0; ci < 2; ci++) quad(sb, base + k * 3 + ci, base + (k + 1) * 3 + ci, base + (k + 1) * 3 + ci + 1, base + k * 3 + ci + 1);
            }
            const sg = own(makeGeo(T, { pos: sb.pos, col: sb.col, uv: sb.uv, idx: sb.idx }));
            const dtex = own(typeof AD.makeTerrainDetail === 'function' ? AD.makeTerrainDetail(T, biomeId, look) : new T.Texture());
            const sm = own(new T.MeshStandardMaterial({ vertexColors: true, map: dtex, roughness: 0.95, metalness: 0, side: T.DoubleSide }));
            const smesh = new T.Mesh(sg, sm);
            smesh.receiveShadow = true; smesh.name = 'shoulder';
            group.add(smesh);
            wetMats.push({ mat: sm, kind: 'gravel' });
            stats.triangles += sg.index.count / 3; stats.meshes++;
        }

        /* barriers on the corridor edge */
        const walls = buildWalls(T, R, biomeId, biome, look, own);
        for (const m of walls.meshes) group.add(m);
        stats.triangles += walls.triangles; stats.meshes += walls.meshes.length; stats.barrier = walls.style;

        /* start line (checkered) and the two sector lines */
        {
            const cols = Math.max(12, Math.round(R.hw[0] * 2)), ct = own(checkerTexture(T, cols));
            const strip = (sv, len, tex, color) => {
                const p = ringAt(R, sv), b = bag(), hw = p.hw - 1.3, y = 0.018;
                const pts = [[-1, -1], [1, -1], [1, 1], [-1, 1]];
                const ids = pts.map(c => vert(b, p.x + p.nx * c[0] * hw + p.tx * c[1] * len / 2, y, p.z + p.nz * c[0] * hw + p.tz * c[1] * len / 2, [1, 1, 1], (c[0] + 1) / 2, (c[1] + 1) / 2));
                quad(b, ids[0], ids[1], ids[2], ids[3]);
                const geo = own(makeGeo(T, { pos: b.pos, col: b.col, uv: b.uv, idx: b.idx }));
                const mat = own(new T.MeshStandardMaterial({ map: tex || null, color: color || 0xffffff, roughness: 0.7, polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -2, side: T.DoubleSide }));
                const mesh = new T.Mesh(geo, mat);
                mesh.receiveShadow = true;
                group.add(mesh);
                stats.triangles += 2; stats.meshes++;
            };
            strip(L - 0.9, 1.8, ct);
            for (const sv of track.sectors || []) strip(sv, 0.5, null, new T.Color(W.hexLin(biome.line)[0], W.hexLin(biome.line)[1], W.hexLin(biome.line)[2]));
        }

        stats.ms = typeof performance !== 'undefined' ? performance.now() - t0 : 0;
        const nightBase = look.night ? 0.38 : 0;
        let applied = -1;
        const api = {
            group: group,
            setWetness(w) {
                const wet = clamp(Math.max(w, nightBase), 0, 1);
                if (Math.abs(wet - applied) < 0.001) return;
                applied = wet;
                for (const e of wetMats) {
                    const spec = WET_MAT[e.kind];
                    e.mat.roughness = lerp(spec.a, spec.b, wet);
                    e.mat.color.setScalar(lerp(1, spec.dark, wet));
                    if (spec.coat > 0 && e.mat.clearcoat !== undefined) e.mat.clearcoat = 0.001 + spec.coat * wet;
                }
            },
            stats() { return stats; },
            dispose() {
                for (const o of owned) if (o && typeof o.dispose === 'function') o.dispose();
                owned.length = 0;
                group.removeFromParent();
            }
        };
        api.setWetness(0);
        return api;
    };
})();
