/* Runenpfad backdrop painters (1/2): shared noise helpers plus woodland, cavern and cave-ceiling silhouettes. */
(function (root) {
    'use strict';

    const P = 2048; // strip period in px; all noise wraps so strips tile seamlessly
    const TAU = Math.PI * 2;

    // Deterministic hash noise so a biome backdrop is stable across frames and sessions.
    function hash1(n, seed) {
        let h = Math.imul(n ^ seed, 0x27d4eb2d);
        h ^= h >>> 15;
        h = Math.imul(h, 0x85ebca6b);
        h ^= h >>> 13;
        return (h >>> 0) / 4294967296;
    }

    function noise1(x, seed) {
        const i = Math.floor(x);
        const f = x - i;
        const u = f * f * (3 - 2 * f);
        return hash1(i, seed) * (1 - u) + hash1(i + 1, seed) * u;
    }

    // Periodic value noise over the strip period (cell index wraps).
    function pn(x, cells, seed) {
        x = ((x % P) + P) % P;
        const u = (x / P) * cells;
        const i = Math.floor(u);
        const f = u - i;
        const s = f * f * (3 - 2 * f);
        return hash1(i % cells, seed) * (1 - s) + hash1((i + 1) % cells, seed) * s;
    }

    function clamp(v, lo, hi) {
        return v < lo ? lo : v > hi ? hi : v;
    }

    function hex(c) {
        const m = /^#([0-9a-f]{6})$/i.exec(c || '');
        const n = m ? parseInt(m[1], 16) : 0;
        return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
    }

    function mix(a, b, t) {
        const A = hex(a);
        const B = hex(b);
        let out = '#';
        for (let i = 0; i < 3; i++) out += Math.round(A[i] + (B[i] - A[i]) * t).toString(16).padStart(2, '0');
        return out;
    }

    function rgba(c, a) {
        const A = hex(c);
        return 'rgba(' + A[0] + ',' + A[1] + ',' + A[2] + ',' + a + ')';
    }

    function wrapDraw(x, hw, fn) {
        fn(x);
        if (x - hw < 0) fn(x + P);
        if (x + hw > P) fn(x - P);
    }

    // Crest heights (px above the baseline) sampled every 4 px over one period.
    function buildCrest(L) {
        const n = P / 4;
        const hs = new Float32Array(n + 1);
        const cells = L.cells || 6;
        const amp = L.amp || 0;
        const seed = L.seed;
        for (let i = 0; i < n; i++) {
            const x = i * 4;
            let v;
            if (L.base === 'peaks') {
                const a = pn(x, cells, seed) * 0.62 + pn(x, cells * 2 + 1, seed + 7) * 0.28 + pn(x, cells * 5 + 1, seed + 13) * 0.1;
                v = amp * (0.12 + 0.88 * Math.pow(1 - Math.abs(a * 2 - 1), 1.7));
            } else if (L.base === 'dunes') {
                const u = (x / P) * cells;
                const fl = Math.floor(u);
                const f = u - fl;
                let g = f < 0.72 ? f / 0.72 : (1 - f) / 0.28;
                g = g * g * (3 - 2 * g);
                v = amp * (0.55 + 0.45 * hash1(fl % cells, seed)) * g * 0.9 + amp * 0.1 * pn(x, cells * 4 + 1, seed + 3);
            } else if (L.base === 'flat') {
                v = amp * pn(x, cells, seed);
            } else {
                v = amp * (pn(x, cells, seed) * 0.7 + pn(x, cells * 3 + 1, seed + 5) * 0.3);
            }
            hs[i] = v;
        }
        hs[n] = hs[0];
        return hs;
    }

    function crestH(hs, x) {
        x = ((x % P) + P) % P;
        const f = x / 4;
        const i = Math.floor(f);
        const t = f - i;
        return hs[i] * (1 - t) + hs[i + 1] * t;
    }

    // ---- silhouette props (drawn into a strip, y grows downwards, base at env.span) ----

    function pine(c, x, y, w, ht, col, snow) {
        const tiers = ht > 100 ? 6 : 5;
        c.fillStyle = col;
        c.fillRect(x - w * 0.05, y - ht * 0.14, w * 0.1, ht * 0.16);
        c.beginPath();
        c.moveTo(x, y - ht);
        const pts = [];
        for (let k = 0; k < tiers; k++) {
            const ty = y - ht + (ht * 0.9 * (k + 1)) / tiers;
            const tw = w * 0.5 * (0.3 + (0.7 * (k + 1)) / tiers);
            pts.push([tw, ty]);
            c.lineTo(x + tw * 0.52, ty - ht * 0.045);
            c.lineTo(x + tw, ty);
        }
        c.lineTo(x + w * 0.04, y - ht * 0.12);
        c.lineTo(x - w * 0.04, y - ht * 0.12);
        for (let k = tiers - 1; k >= 0; k--) {
            c.lineTo(x - pts[k][0], pts[k][1]);
            c.lineTo(x - pts[k][0] * 0.52, pts[k][1] - ht * 0.045);
        }
        c.closePath();
        c.fill();
        if (snow) {
            c.fillStyle = 'rgba(236, 248, 255, 0.55)';
            for (let k = 0; k < tiers; k++) {
                const ty0 = y - ht + (ht * 0.9 * k) / tiers + (k ? ht * 0.02 : 0);
                const tw = pts[k][0] * 0.62;
                c.beginPath();
                c.moveTo(x, ty0);
                c.lineTo(x + tw, ty0 + ht * 0.09);
                c.lineTo(x - tw, ty0 + ht * 0.09);
                c.closePath();
                c.fill();
            }
        }
    }

    function oak(c, x, y, ht, col, hi) {
        const tw = ht * 0.07;
        c.fillStyle = col;
        c.beginPath();
        c.moveTo(x - tw * 1.5, y);
        c.quadraticCurveTo(x - tw * 0.6, y - ht * 0.35, x - tw * 0.5, y - ht * 0.64);
        c.lineTo(x + tw * 0.5, y - ht * 0.64);
        c.quadraticCurveTo(x + tw * 0.6, y - ht * 0.35, x + tw * 1.5, y);
        c.fill();
        const R = ht * 0.3;
        const cy = y - ht * 0.72;
        const blobs = [[-0.95, 0.15, 0.62], [0.95, 0.2, 0.6], [0, -0.3, 0.82], [-0.5, -0.1, 0.72], [0.5, -0.05, 0.74], [0, 0.28, 0.58], [-1.3, 0.35, 0.42], [1.3, 0.38, 0.4]];
        for (let i = 0; i < blobs.length; i++) {
            c.beginPath();
            c.arc(x + blobs[i][0] * R, cy + blobs[i][1] * R, blobs[i][2] * R, 0, TAU);
            c.fill();
        }
        c.fillStyle = hi;
        for (let i = 0; i < 5; i++) {
            c.beginPath();
            c.arc(x + blobs[i][0] * R - R * 0.14, cy + blobs[i][1] * R - R * 0.2, blobs[i][2] * R * 0.66, 0, TAU);
            c.fill();
        }
    }

    function crenels(c, x, y, w, step, hgt) {
        const n = Math.max(2, Math.round(w / step));
        const cw = w / n;
        for (let k = 0; k < n; k += 2) c.fillRect(x + k * cw, y - hgt, cw, hgt + 1);
    }

    function castle(c, x, y, s, col) {
        c.fillStyle = col;
        c.fillRect(x - 24 * s, y - 86 * s, 46 * s, 90 * s); // keep behind
        crenels(c, x - 24 * s, y - 86 * s, 46 * s, 9 * s, 8 * s);
        c.beginPath();
        c.rect(x - 78 * s, y - 46 * s, 156 * s, 50 * s);
        c.moveTo(x - 15 * s, y + 2);
        c.lineTo(x - 15 * s, y - 18 * s);
        c.arc(x, y - 18 * s, 15 * s, Math.PI, 0);
        c.lineTo(x + 15 * s, y + 2);
        c.closePath();
        c.fill('evenodd');
        for (let k = 0; k < 11; k++) if (k !== 3 && k !== 7) c.fillRect(x - 78 * s + k * 14 * s, y - 54 * s, 8 * s, 9 * s);
        c.fillRect(x - 96 * s, y - 120 * s, 34 * s, 124 * s);
        crenels(c, x - 96 * s, y - 120 * s, 34 * s, 9 * s, 9 * s);
        c.beginPath();
        c.moveTo(x + 56 * s, y + 2);
        c.lineTo(x + 56 * s, y - 96 * s);
        c.lineTo(x + 74 * s, y - 108 * s);
        c.lineTo(x + 86 * s, y - 86 * s);
        c.lineTo(x + 88 * s, y + 2);
        c.closePath();
        c.fill();
        c.fillStyle = 'rgba(255, 206, 120, 0.55)';
        c.fillRect(x - 83 * s, y - 96 * s, 4 * s, 10 * s);
        c.fillRect(x - 83 * s, y - 70 * s, 4 * s, 10 * s);
        c.fillRect(x + 68 * s, y - 68 * s, 4 * s, 9 * s);
        c.fillRect(x - 3 * s, y - 62 * s, 4 * s, 9 * s);
        c.strokeStyle = col;
        c.lineWidth = 1.5 * s;
        c.beginPath();
        c.moveTo(x - 79 * s, y - 120 * s);
        c.lineTo(x - 79 * s, y - 146 * s);
        c.stroke();
        c.beginPath();
        c.moveTo(x - 79 * s, y - 146 * s);
        c.lineTo(x - 64 * s, y - 141 * s);
        c.lineTo(x - 79 * s, y - 136 * s);
        c.fill();
    }

    function gear(c, x, y, r, col, teeth) {
        const s = TAU / teeth;
        c.fillStyle = col;
        c.beginPath();
        for (let i = 0; i < teeth; i++) {
            const a = i * s;
            const pts = [[0, 0.8], [0.12, 1], [0.45, 1], [0.57, 0.8]];
            for (let k = 0; k < 4; k++) c.lineTo(x + Math.cos(a + pts[k][0] * s) * r * pts[k][1], y + Math.sin(a + pts[k][0] * s) * r * pts[k][1]);
        }
        c.closePath();
        c.moveTo(x + r * 0.6, y);
        c.arc(x, y, r * 0.6, 0, TAU, true);
        c.fill('evenodd');
        c.beginPath();
        c.arc(x, y, r * 0.2, 0, TAU);
        c.fill();
        c.lineWidth = Math.max(2, r * 0.1);
        c.strokeStyle = col;
        c.beginPath();
        for (let k = 0; k < 4; k++) {
            c.moveTo(x, y);
            c.lineTo(x + Math.cos(k * Math.PI / 2 + 0.4) * r * 0.62, y + Math.sin(k * Math.PI / 2 + 0.4) * r * 0.62);
        }
        c.stroke();
    }

    function tower(env, x, y, w, h, kind, col, lit, idx) {
        const c = env.c;
        c.fillStyle = col;
        if (kind === 1) {
            c.fillRect(x - w / 2, y - h, w, h);
            c.beginPath();
            c.moveTo(x - w / 2 - 4, y - h);
            c.lineTo(x, y - h - w * 0.95);
            c.lineTo(x + w / 2 + 4, y - h);
            c.closePath();
            c.fill();
        } else if (kind === 2) {
            c.fillRect(x - w / 2, y - h * 0.6, w, h * 0.6);
            c.fillRect(x - w * 0.32, y - h, w * 0.64, h * 0.42);
            crenels(c, x - w * 0.32, y - h, w * 0.64, 10, 7);
            c.fillRect(x - w / 2 - 3, y - h * 0.6, w + 6, 4);
        } else {
            c.fillRect(x - w / 2, y - h, w, h);
            c.fillRect(x - w / 2 - 3, y - h, w + 6, 4);
            crenels(c, x - w / 2 - 3, y - h - 1, w + 6, 11, 8);
        }
        const cols = Math.max(1, Math.floor(w / 24));
        const rows = Math.floor(h / 36);
        for (let rr = 1; rr < rows; rr++) {
            for (let cc = 0; cc < cols; cc++) {
                const hv = env.r(idx * 131 + rr * 17 + cc, 9);
                if (hv < 0.42) continue;
                c.fillStyle = lit ? (hv > 0.8 ? '#ffb04a' : '#ff6a3a') : 'rgba(255, 128, 80, 0.2)';
                c.fillRect(x - w / 2 + (cc + 0.5) * (w / cols) - 2, y - h + rr * 36 - 18, 4, 8);
            }
        }
    }

    // ---- layer painters ----

    function propsPines(env, snow) {
        const { c, L, hs, span } = env;
        const N = L.big ? 46 : 70;
        const dk = mix(L.color, '#000000', 0.2);
        const lt = mix(L.color, '#ffffff', 0.1);
        for (let i = 0; i < N; i++) {
            const x = ((i + env.r(i, 1) * 0.9) * P) / N;
            const ht = (L.big ? 90 : 60) + env.r(i, 2) * (L.big ? 110 : 80);
            const w = ht * (0.36 + env.r(i, 3) * 0.14);
            const y = span - crestH(hs, x) + 5;
            const col = env.r(i, 4) > 0.5 ? dk : mix(dk, lt, 0.4);
            wrapDraw(x, w, function (xx) { pine(c, xx, y, w, ht, col, snow); });
        }
    }

    function propsOaks(env) {
        const { c, L, hs, span } = env;
        const N = 28;
        const dk = mix(L.color, '#000000', 0.18);
        const hi = rgba(mix(L.color, '#a8e08a', 0.28), 0.45);
        for (let i = 0; i < N; i++) {
            const x = ((i + env.r(i, 1) * 0.8) * P) / N;
            const ht = 80 + env.r(i, 2) * 90;
            const y = span - crestH(hs, x) + 4;
            const col = env.r(i, 4) > 0.55 ? dk : L.color;
            wrapDraw(x, ht * 0.5, function (xx) { oak(c, xx, y, ht, col, hi); });
        }
    }

    function propsTrunks(env) {
        const { c, L, span } = env;
        const N = 6;
        const dk = mix(L.color, '#000000', 0.12);
        const hi = rgba(mix(L.color, '#9ad48a', 0.3), 0.32);
        for (let i = 0; i < N; i++) {
            const x = ((i + 0.2 + env.r(i, 1) * 0.6) * P) / N;
            const w = 22 + env.r(i, 2) * 26;
            const top = span * (0.55 + env.r(i, 3) * 0.4);
            wrapDraw(x, 150, function (xx) {
                c.fillStyle = dk;
                c.beginPath();
                c.moveTo(xx - w * 1.5, span + 20);
                c.quadraticCurveTo(xx - w * 0.5, span - 30, xx - w * 0.5, span - top * 0.4);
                c.lineTo(xx - w * 0.38, span - top);
                c.lineTo(xx + w * 0.38, span - top);
                c.lineTo(xx + w * 0.5, span - top * 0.4);
                c.quadraticCurveTo(xx + w * 0.5, span - 30, xx + w * 1.5, span + 20);
                c.closePath();
                c.fill();
                c.fillStyle = hi;
                c.fillRect(xx - w * 0.42, span - top, w * 0.16, top);
                const side = env.r(i, 7) > 0.5 ? 1 : -1;
                c.fillStyle = dk;
                c.beginPath();
                c.moveTo(xx, span - top * 0.7);
                c.lineTo(xx + side * 70, span - top * 0.95);
                c.lineTo(xx + side * 74, span - top * 0.95 + 9);
                c.lineTo(xx, span - top * 0.6);
                c.fill();
                const R = 34 + env.r(i, 5) * 22;
                const pts = [[0, 0, 1], [-1.1, 0.3, 0.8], [1.1, 0.35, 0.8], [-0.5, -0.5, 0.8], [0.6, -0.45, 0.78], [side * 1.9, 0.3, 0.55]];
                for (let k = 0; k < pts.length; k++) {
                    c.fillStyle = dk;
                    c.beginPath();
                    c.arc(xx + pts[k][0] * R, span - top - 6 + pts[k][1] * R, pts[k][2] * R, 0, TAU);
                    c.fill();
                }
                c.fillStyle = hi;
                for (let k = 0; k < 3; k++) {
                    c.beginPath();
                    c.arc(xx + pts[k][0] * R - R * 0.15, span - top - 6 + pts[k][1] * R - R * 0.2, pts[k][2] * R * 0.62, 0, TAU);
                    c.fill();
                }
            });
        }
    }

    function propsCastle(env) {
        const { c, L, hs, span } = env;
        const dk = mix(L.color, '#000000', 0.25);
        for (let k = 0; k < 3; k++) {
            const x = ((k + 0.25 + env.r(k, 1) * 0.5) * P) / 3;
            const s = 0.8 + env.r(k, 2) * 0.5;
            wrapDraw(x, 120 * s, function (xx) { castle(c, xx, span - crestH(hs, xx) + 6, s, dk); });
        }
    }

    function propsCrystals(env) {
        const { c, L, hs, span } = env;
        const pal = L.palette || ['#7ee8fa'];
        for (let i = 0; i < 24; i++) {
            const x = ((i + env.r(i, 1) * 0.8) * P) / 24;
            const base = span - crestH(hs, x) + 2;
            const pc = pal[Math.floor(env.r(i, 2) * pal.length)];
            const n = 3 + Math.floor(env.r(i, 3) * 3);
            wrapDraw(x, 70, function (xx) {
                const glow = c.createRadialGradient(xx, base - 24, 2, xx, base - 24, 80);
                glow.addColorStop(0, rgba(pc, 0.3));
                glow.addColorStop(1, rgba(pc, 0));
                c.fillStyle = glow;
                c.fillRect(xx - 80, base - 104, 160, 130);
                for (let k = 0; k < n; k++) {
                    const ang = ((k - (n - 1) / 2) / n) * 0.9 + (env.r(i * 7 + k, 4) - 0.5) * 0.25;
                    const h = 26 + env.r(i * 7 + k, 5) * 56;
                    const w = 6 + env.r(i * 7 + k, 6) * 7;
                    c.save();
                    c.translate(xx + (k - n / 2) * 6, base);
                    c.rotate(ang);
                    const g = c.createLinearGradient(0, 0, 0, -h);
                    g.addColorStop(0, mix(pc, L.color, 0.7));
                    g.addColorStop(1, pc);
                    c.fillStyle = g;
                    c.beginPath();
                    c.moveTo(-w, 0);
                    c.lineTo(-w * 0.9, -h * 0.72);
                    c.lineTo(0, -h);
                    c.lineTo(w * 0.9, -h * 0.72);
                    c.lineTo(w, 0);
                    c.closePath();
                    c.fill();
                    c.fillStyle = 'rgba(255, 255, 255, 0.34)';
                    c.beginPath();
                    c.moveTo(-w, 0);
                    c.lineTo(-w * 0.9, -h * 0.72);
                    c.lineTo(0, -h);
                    c.lineTo(-w * 0.15, -h * 0.3);
                    c.closePath();
                    c.fill();
                    c.restore();
                }
            });
        }
    }

    function propsRocks(env) {
        const { c, L, hs, span } = env;
        const dk = mix(L.color, '#000000', L.warm ? 0.1 : 0.25);
        const N = 34;
        for (let i = 0; i < N; i++) {
            const x = ((i + env.r(i, 1) * 0.9) * P) / N;
            const h = 26 + env.r(i, 2) * 90;
            const w = 10 + h * 0.3 + env.r(i, 3) * 8;
            const y = span - crestH(hs, x) + 5;
            wrapDraw(x, w, function (xx) {
                c.fillStyle = dk;
                c.beginPath();
                c.moveTo(xx - w, y);
                c.lineTo(xx - w * 0.25, y - h * 0.78);
                c.lineTo(xx + w * 0.05, y - h);
                c.lineTo(xx + w * 0.5, y - h * 0.55);
                c.lineTo(xx + w, y);
                c.closePath();
                c.fill();
                c.fillStyle = 'rgba(0, 0, 0, 0.22)';
                c.beginPath();
                c.moveTo(xx + w * 0.05, y - h);
                c.lineTo(xx + w * 0.5, y - h * 0.55);
                c.lineTo(xx + w, y);
                c.lineTo(xx + w * 0.05, y);
                c.closePath();
                c.fill();
                c.strokeStyle = 'rgba(255, 255, 255, 0.12)';
                c.lineWidth = 1.5;
                c.beginPath();
                c.moveTo(xx - w, y);
                c.lineTo(xx - w * 0.25, y - h * 0.78);
                c.lineTo(xx + w * 0.05, y - h);
                c.stroke();
            });
        }
        if (L.warm) {
            c.strokeStyle = dk;
            c.lineCap = 'butt';
            for (let k = 0; k < 2; k++) {
                const x = ((k + 0.3 + env.r(k, 11) * 0.4) * P) / 2;
                const r = 42 + env.r(k, 12) * 24;
                wrapDraw(x, r + 20, function (xx) {
                    const y = span - crestH(hs, xx) + 6;
                    c.lineWidth = 22;
                    c.beginPath();
                    c.arc(xx, y - 8, r, Math.PI, 0);
                    c.stroke();
                    c.fillRect(xx - r - 11, y - 8, 22, 14);
                    c.fillRect(xx + r - 11, y - 8, 22, 14);
                });
            }
        }
    }

    function propsMushrooms(env) {
        const { c, L, hs, span } = env;
        const pal = L.palette || ['#5eead4'];
        const N = 18;
        for (let i = 0; i < N; i++) {
            const x = ((i + env.r(i, 1) * 0.8) * P) / N;
            const base = span - crestH(hs, x) + 3;
            const h = 40 + env.r(i, 2) * 110;
            const rx = 24 + env.r(i, 3) * 44;
            const ry = 10 + rx * 0.3;
            const pc = pal[Math.floor(env.r(i, 4) * pal.length)];
            const sw = 5 + h * 0.05;
            const bend = (env.r(i, 5) - 0.5) * 30;
            wrapDraw(x, rx + 40, function (xx) {
                const glow = c.createRadialGradient(xx + bend, base - h, 4, xx + bend, base - h, rx * 2);
                glow.addColorStop(0, rgba(pc, 0.3));
                glow.addColorStop(1, rgba(pc, 0));
                c.fillStyle = glow;
                c.fillRect(xx + bend - rx * 2, base - h - rx * 2, rx * 4, rx * 4);
                c.fillStyle = mix(L.color, '#000000', 0.2);
                c.beginPath();
                c.moveTo(xx - sw * 1.5, base);
                c.quadraticCurveTo(xx - sw * 0.5, base - h * 0.5, xx + bend - sw * 0.6, base - h);
                c.lineTo(xx + bend + sw * 0.6, base - h);
                c.quadraticCurveTo(xx + sw * 0.5, base - h * 0.5, xx + sw * 1.5, base);
                c.fill();
                const cx = xx + bend;
                const cy = base - h;
                c.fillStyle = mix(pc, L.color, 0.55);
                c.beginPath();
                c.ellipse(cx, cy, rx, ry, 0, Math.PI, 0);
                c.quadraticCurveTo(cx, cy + ry * 0.7, cx - rx, cy);
                c.fill();
                c.strokeStyle = rgba(pc, 0.9);
                c.lineWidth = 2;
                c.beginPath();
                c.moveTo(cx - rx, cy);
                c.quadraticCurveTo(cx, cy + ry * 0.7, cx + rx, cy);
                c.stroke();
                c.fillStyle = rgba(pc, 0.8);
                for (let k = 0; k < 3; k++) {
                    c.beginPath();
                    c.arc(cx + (k - 1) * rx * 0.45, cy - ry * (0.4 + 0.2 * (k % 2)), 2 + (k % 2), 0, TAU);
                    c.fill();
                }
            });
        }
    }

    function propsCeiling(env) {
        const { c, L, biome, entry } = env;
        const hz = biome.sky[biome.sky.length - 1];
        const ceilH = function (x) { return 26 + 44 * pn(x, 9, L.seed + 3); };
        const g = c.createLinearGradient(0, 0, 0, L.span);
        g.addColorStop(0, mix(L.color, '#000000', 0.2));
        g.addColorStop(1, mix(L.color, hz, L.mist || 0.3));
        c.fillStyle = g;
        c.beginPath();
        c.moveTo(0, 0);
        c.lineTo(P, 0);
        for (let x = P; x >= 0; x -= 8) c.lineTo(x, ceilH(x));
        c.closePath();
        c.fill();
        const N = L.drips ? 22 : 30;
        const len0 = L.len[0];
        const len1 = L.len[1];
        for (let row = 0; row < 2; row++) {
            for (let i = row; i < N; i += 2) {
                const x = ((i + 0.1 + env.r(i, 1) * 0.8) * P) / N;
                const len = (len0 + env.r(i, 2) * (len1 - len0)) * (row ? 0.7 : 1);
                const w = 12 + len * 0.2;
                const j = (env.r(i, 3) - 0.5) * 12;
                const y0 = ceilH(x) - 6;
                if (L.drips && !row && i % 3 === 0) entry.tips.push({ x: x + j, y: y0 + len + 6, n: i });
                                wrapDraw(x, w, function (xx) {
                    c.fillStyle = row ? mix(L.color, hz, 0.14) : mix(L.color, '#000000', 0.12);
                    c.beginPath();
                    c.moveTo(xx - w, y0);
                    c.quadraticCurveTo(xx - w * 0.22, y0 + len * 0.55, xx + j, y0 + len + 6);
                    c.quadraticCurveTo(xx + w * 0.22, y0 + len * 0.55, xx + w, y0);
                    c.closePath();
                    c.fill();
                    c.strokeStyle = row ? 'rgba(180, 160, 255, 0.12)' : 'rgba(190, 170, 255, 0.4)';
                    c.lineWidth = 1.5;
                    c.beginPath();
                    c.moveTo(xx - w * 0.7, y0 + 4);
                    c.quadraticCurveTo(xx - w * 0.2, y0 + len * 0.55, xx + j, y0 + len + 4);
                    c.stroke();
                    if (!row && i % 5 === 0) {
                        c.fillStyle = 'rgba(126, 232, 250, 0.9)';
                        c.beginPath();
                        c.arc(xx + j, y0 + len + 5, 2, 0, TAU);
                        c.fill();
                    }
                });
            }
        }
    }

    root.RunenpfadBackdrops = {
        P: P, TAU: TAU, hash1: hash1, noise1: noise1, pn: pn, clamp: clamp, mix: mix, rgba: rgba, wrapDraw: wrapDraw,
        buildCrest: buildCrest, crestH: crestH, gear: gear, tower: tower,
        propsPines: propsPines, propsOaks: propsOaks, propsTrunks: propsTrunks, propsCastle: propsCastle,
        propsCrystals: propsCrystals, propsRocks: propsRocks, propsMushrooms: propsMushrooms, propsCeiling: propsCeiling
    };
})(typeof window !== 'undefined' ? window : globalThis);
