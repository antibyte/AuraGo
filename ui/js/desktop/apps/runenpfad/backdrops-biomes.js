/* Runenpfad backdrop painters (2/2): desert, glacier and citadel silhouettes and peak decoration. */
(function (root) {
    'use strict';

    const B = root.RunenpfadBackdrops;
    if (!B) return;
    const P = B.P;
    const TAU = B.TAU;
    const hash1 = B.hash1;
    const pn = B.pn;
    const mix = B.mix;
    const rgba = B.rgba;
    const wrapDraw = B.wrapDraw;
    const crestH = B.crestH;
    const gear = B.gear;
    const tower = B.tower;
    const propsPines = B.propsPines;
    const propsOaks = B.propsOaks;
    const propsTrunks = B.propsTrunks;
    const propsCastle = B.propsCastle;
    const propsCrystals = B.propsCrystals;
    const propsRocks = B.propsRocks;
    const propsMushrooms = B.propsMushrooms;
    const propsCeiling = B.propsCeiling;

    function propsMesas(env) {
        const { c, L, hs, span } = env;
        const dk = mix(L.color, '#000000', 0.08);
        const N = 7;
        for (let i = 0; i < N; i++) {
            const x = ((i + 0.2 + env.r(i, 1) * 0.6) * P) / N;
            const w = 110 + env.r(i, 2) * 120;
            const h = 50 + env.r(i, 3) * 90;
            const y = span - crestH(hs, x) + 4;
            wrapDraw(x, w, function (xx) {
                c.save();
                c.beginPath();
                c.moveTo(xx - w * 0.52, y);
                c.lineTo(xx - w * 0.4, y - h * 0.52);
                c.lineTo(xx - w * 0.44, y - h * 0.56);
                c.lineTo(xx - w * 0.3, y - h);
                c.lineTo(xx + w * 0.3, y - h);
                c.lineTo(xx + w * 0.42, y - h * 0.58);
                c.lineTo(xx + w * 0.38, y - h * 0.52);
                c.lineTo(xx + w * 0.52, y);
                c.closePath();
                c.fillStyle = dk;
                c.fill();
                c.clip();
                for (let b = 0; b < 7; b++) {
                    c.fillStyle = b % 2 ? 'rgba(255, 190, 130, 0.06)' : 'rgba(40, 10, 30, 0.07)';
                    c.fillRect(xx - w, y - h + (b * h) / 7, w * 2, h / 7);
                }
                c.fillStyle = 'rgba(40, 10, 30, 0.2)';
                c.fillRect(xx + w * 0.1, y - h, w, h);
                c.restore();
            });
        }
    }

    function propsRuins(env) {
        const { c, L, hs, span } = env;
        const dk = mix(L.color, '#000000', 0.2);
        const N = 5;
        for (let i = 0; i < N; i++) {
            const x = ((i + 0.2 + env.r(i, 1) * 0.6) * P) / N;
            const s = 0.8 + env.r(i, 2) * 0.6;
            const kind = i % 4;
            wrapDraw(x, 90 * s, function (xx) {
                const y = span - crestH(hs, xx) + 6;
                c.fillStyle = dk;
                if (kind === 0) {
                    c.beginPath();
                    c.moveTo(xx - 8 * s, y);
                    c.lineTo(xx - 5.5 * s, y - 118 * s);
                    c.lineTo(xx, y - 136 * s);
                    c.lineTo(xx + 5.5 * s, y - 118 * s);
                    c.lineTo(xx + 8 * s, y);
                    c.fill();
                    c.fillRect(xx - 14 * s, y - 8 * s, 28 * s, 10 * s);
                    c.fillStyle = 'rgba(255, 190, 120, 0.4)';
                    c.fillRect(xx - 1 * s, y - 110 * s, 2 * s, 80 * s);
                } else if (kind === 1) {
                    for (let k = 0; k < 4; k++) {
                        const ph = (k === 2 ? 38 : 72 + (k % 2) * 12) * s;
                        c.fillRect(xx - 44 * s + k * 26 * s, y - ph, 9 * s, ph + 2);
                        if (k !== 2) c.fillRect(xx - 47 * s + k * 26 * s, y - ph - 5 * s, 15 * s, 6 * s);
                    }
                    c.fillRect(xx - 47 * s, y - 88 * s, 52 * s, 8 * s);
                } else if (kind === 2) {
                    c.beginPath();
                    c.moveTo(xx - 80 * s, y);
                    c.lineTo(xx, y - 100 * s);
                    c.lineTo(xx + 80 * s, y);
                    c.closePath();
                    c.fill();
                    c.strokeStyle = 'rgba(255, 190, 130, 0.14)';
                    c.lineWidth = 1.5;
                    for (let k = 1; k < 5; k++) {
                        c.beginPath();
                        c.moveTo(xx - 80 * s * (1 - k / 5), y - 20 * s * k);
                        c.lineTo(xx + 80 * s * (1 - k / 5), y - 20 * s * k);
                        c.stroke();
                    }
                } else {
                    c.beginPath();
                    c.rect(xx - 40 * s, y - 80 * s, 80 * s, 82 * s);
                    c.moveTo(xx - 16 * s, y + 2);
                    c.lineTo(xx - 16 * s, y - 36 * s);
                    c.arc(xx, y - 36 * s, 16 * s, Math.PI, 0);
                    c.lineTo(xx + 16 * s, y + 2);
                    c.closePath();
                    c.fill('evenodd');
                    c.fillRect(xx - 46 * s, y - 88 * s, 92 * s, 9 * s);
                }
            });
        }
    }

    function propsCacti(env) {
        const { c, L, hs, span } = env;
        const dk = mix(L.color, '#000000', 0.22);
        c.strokeStyle = dk;
        c.fillStyle = dk;
        c.lineCap = 'round';
        c.lineJoin = 'round';
        for (let i = 0; i < 30; i++) {
            const x = ((i + env.r(i, 1) * 0.9) * P) / 30;
            const k = env.r(i, 2);
            wrapDraw(x, 40, function (xx) {
                const y = span - crestH(hs, xx) + 5;
                if (k < 0.55) {
                    const h = 36 + env.r(i, 3) * 50;
                    const w = 5 + h * 0.07;
                    c.lineWidth = w;
                    c.beginPath();
                    c.moveTo(xx, y);
                    c.lineTo(xx, y - h);
                    c.stroke();
                    const a1 = h * 0.45;
                    c.lineWidth = w * 0.72;
                    c.beginPath();
                    c.moveTo(xx, y - a1);
                    c.lineTo(xx - h * 0.28, y - a1);
                    c.lineTo(xx - h * 0.28, y - a1 - h * 0.28);
                    c.moveTo(xx, y - h * 0.62);
                    c.lineTo(xx + h * 0.24, y - h * 0.62);
                    c.lineTo(xx + h * 0.24, y - h * 0.62 - h * 0.22);
                    c.stroke();
                } else if (k < 0.8) {
                    c.beginPath();
                    c.ellipse(xx, y - 8, 11, 10, 0, 0, TAU);
                    c.fill();
                    c.beginPath();
                    c.ellipse(xx + 13, y - 4, 7, 6, 0, 0, TAU);
                    c.fill();
                } else {
                    c.lineWidth = 1.6;
                    c.beginPath();
                    for (let b = 0; b < 7; b++) {
                        c.moveTo(xx, y);
                        c.quadraticCurveTo(xx + (b - 3) * 4, y - 18, xx + (b - 3) * 9, y - 24 - (b % 2) * 6);
                    }
                    c.stroke();
                }
            });
        }
    }

    function propsIceSpires(env) {
        const { c, L, span } = env;
        const N = 22;
        for (let i = 0; i < N; i++) {
            const x = ((i + env.r(i, 1) * 0.8) * P) / N;
            const h = 90 + env.r(i, 2) * 180;
            const w = 18 + env.r(i, 3) * 28;
            const j = (env.r(i, 4) - 0.5) * w;
            const y = span + 8;
            wrapDraw(x, w, function (xx) {
                const g = c.createLinearGradient(0, y, 0, y - h);
                g.addColorStop(0, L.color);
                g.addColorStop(1, mix(L.color, '#7fd7ff', 0.55));
                c.fillStyle = g;
                c.beginPath();
                c.moveTo(xx - w, y);
                c.lineTo(xx - w * 0.3, y - h * 0.62);
                c.lineTo(xx + j, y - h);
                c.lineTo(xx + w * 0.45, y - h * 0.55);
                c.lineTo(xx + w, y);
                c.closePath();
                c.fill();
                c.fillStyle = 'rgba(170, 225, 255, 0.24)';
                c.beginPath();
                c.moveTo(xx - w * 0.3, y - h * 0.62);
                c.lineTo(xx + j, y - h);
                c.lineTo(xx + w * 0.1, y - h * 0.4);
                c.lineTo(xx - w * 0.5, y);
                c.lineTo(xx - w, y);
                c.closePath();
                c.fill();
                c.strokeStyle = 'rgba(200, 240, 255, 0.55)';
                c.lineWidth = 1.5;
                c.beginPath();
                c.moveTo(xx - w * 0.3, y - h * 0.62);
                c.lineTo(xx + j, y - h);
                c.stroke();
            });
        }
    }

    function propsTowers(env, withChains) {
        const { c, L, hs, span } = env;
        const col = mix(L.color, '#000000', 0.12);
        const N = withChains ? 7 : (L.gears ? 9 : 12);
        const list = [];
        for (let i = 0; i < N; i++) {
            const x = ((i + 0.2 + env.r(i, 1) * 0.6) * P) / N;
            const w = (L.gears ? 52 : 38) + env.r(i, 2) * 34;
            const h = (L.gears ? 120 : 80) + env.r(i, 3) * (L.gears ? 170 : 120);
            const kind = Math.floor(env.r(i, 4) * 3);
            list.push({ x: x, w: w, h: h, kind: kind, i: i });
            if (L.gears && env.r(i, 6) > 0.45) {
                wrapDraw(x, 80, function (xx) {
                    gear(c, xx + (env.r(i, 7) > 0.5 ? 1 : -1) * w * 0.5, span - crestH(hs, xx) - h * 0.45, 30 + env.r(i, 8) * 26, mix(L.color, '#ffffff', 0.05), 12);
                });
            }
        }
        for (let n = 0; n < list.length; n++) {
            const t = list[n];
            wrapDraw(t.x, t.w, function (xx) { tower(env, xx, span - crestH(hs, xx) + 5, t.w, t.h, t.kind, col, !!L.windows, t.i); });
        }
        if (withChains) {
            c.strokeStyle = mix(L.color, '#ffffff', 0.12);
            c.lineWidth = 3;
            c.setLineDash([6, 4]);
            for (let n = 0; n < list.length; n++) {
                const a = list[n];
                const b = list[(n + 1) % list.length];
                const bx = b.x + (n + 1 === list.length ? P : 0);
                const ya = span - crestH(hs, a.x) - a.h * 0.8;
                const yb = span - crestH(hs, b.x) - b.h * 0.8;
                for (let off = -P; off <= P; off += P) {
                    c.beginPath();
                    c.moveTo(a.x + off, ya);
                    c.quadraticCurveTo((a.x + bx) / 2 + off, Math.max(ya, yb) + 46, bx + off, yb);
                    c.stroke();
                }
                wrapDraw((a.x + bx) / 2 % P, 12, function (xx) {
                    c.fillStyle = 'rgba(255, 150, 70, 0.9)';
                    c.beginPath();
                    c.arc(xx, Math.max(ya, yb) + 22, 3, 0, TAU);
                    c.fill();
                });
            }
            c.setLineDash([]);
        }
    }

    function propsChimneys(env) {
        const { c, L, hs, span, entry } = env;
        const col = mix(L.color, '#000000', 0.15);
        const N = 9;
        for (let i = 0; i < N; i++) {
            const x = ((i + 0.2 + env.r(i, 1) * 0.6) * P) / N;
            const w = 90 + env.r(i, 2) * 80;
            const h = 36 + env.r(i, 3) * 40;
            wrapDraw(x, w, function (xx) {
                const y = span - crestH(hs, xx) + 5;
                c.fillStyle = col;
                c.beginPath();
                c.moveTo(xx - w / 2, y);
                c.lineTo(xx - w / 2, y - h);
                const teeth = Math.round(w / 28);
                for (let k = 0; k < teeth; k++) {
                    const tx = xx - w / 2 + (k * w) / teeth;
                    c.lineTo(tx, y - h - 16);
                    c.lineTo(tx + w / teeth, y - h);
                }
                c.lineTo(xx + w / 2, y);
                c.closePath();
                c.fill();
                for (let k = 0; k < 4; k++) {
                    if (env.r(i * 9 + k, 5) > 0.5) {
                        c.fillStyle = '#ff8a3a';
                        c.fillRect(xx - w / 2 + 12 + k * (w / 4.6), y - h * 0.6, 5, 7);
                    }
                }
                c.fillStyle = col;
                const nch = 1 + Math.floor(env.r(i, 6) * 2.5);
                for (let k = 0; k < nch; k++) {
                    const cx = xx - w * 0.3 + k * w * 0.3;
                    const ch = 60 + env.r(i * 5 + k, 7) * 80;
                    c.beginPath();
                    c.moveTo(cx - 7, y - h + 4);
                    c.lineTo(cx - 6, y - h - ch);
                    c.lineTo(cx - 10, y - h - ch);
                    c.lineTo(cx - 10, y - h - ch - 6);
                    c.lineTo(cx + 10, y - h - ch - 6);
                    c.lineTo(cx + 10, y - h - ch);
                    c.lineTo(cx + 6, y - h - ch);
                    c.lineTo(cx + 7, y - h + 4);
                    c.fill();
                    if (L.smoke) entry.smoke.push({ x: cx, h: crestH(hs, xx) + h + ch + 8, n: i * 5 + k });
                }
            });
        }
    }

    function propsSpires(env) {
        const { c, L, hs, span } = env;
        const col = mix(L.color, '#000000', 0.1);
        for (let i = 0; i < 14; i++) {
            const x = ((i + env.r(i, 1) * 0.8) * P) / 14;
            const w = 12 + env.r(i, 2) * 16;
            const h = 130 + env.r(i, 3) * 190;
            wrapDraw(x, w * 2, function (xx) {
                const y = span + 8;
                c.fillStyle = col;
                c.beginPath();
                c.moveTo(xx - w, y);
                c.lineTo(xx - w * 0.8, y - h * 0.8);
                c.lineTo(xx - w * 1.2, y - h * 0.8);
                c.lineTo(xx, y - h);
                c.lineTo(xx + w * 1.2, y - h * 0.8);
                c.lineTo(xx + w * 0.8, y - h * 0.8);
                c.lineTo(xx + w, y);
                c.closePath();
                c.fill();
                c.fillRect(xx - w * 1.4, y - h * 0.45, w * 2.8, 5);
                c.fillStyle = 'rgba(255, 110, 60, 0.85)';
                if (env.r(i, 4) > 0.3) c.fillRect(xx - 1.5, y - h * 0.65, 3, 9);
                if (env.r(i, 5) > 0.5) c.fillRect(xx - 1.5, y - h * 0.3, 3, 9);
            });
        }
    }

    const PROPS = {
        pines: function (env) { propsPines(env, false); },
        snowpines: function (env) { propsPines(env, true); },
        oaks: propsOaks,
        trunks: propsTrunks,
        castle: propsCastle,
        crystals: propsCrystals,
        rocks: propsRocks,
        mushrooms: propsMushrooms,
        mesas: propsMesas,
        ruins: propsRuins,
        cacti: propsCacti,
        icespires: propsIceSpires,
        towers: function (env) { propsTowers(env, false); },
        chains: function (env) { propsTowers(env, true); },
        chimneys: propsChimneys,
        spires: propsSpires
    };

    // Faceted shading, snow caps, ridge highlights and frozen falls for peak crests.
    function decoratePeaks(env) {
        const { c, L, hs, span } = env;
        const n = P / 4;
        const hz = env.biome.sky[env.biome.sky.length - 1];
        const maxima = [];
        for (let i = 2; i < n - 2; i++) {
            if (hs[i] > hs[i - 1] && hs[i] >= hs[i + 1] && hs[i] > L.amp * 0.38) maxima.push(i);
        }
        if (L.facets) {
            for (let m = 0; m < maxima.length; m++) {
                const i = maxima[m];
                let j = i + 1;
                while (j < n && hs[j] < hs[j - 1]) j++;
                const xp = i * 4;
                const xv = (j - 1) * 4;
                const yp = span - hs[i];
                const yv = span - hs[j - 1];
                c.fillStyle = 'rgba(4, 8, 24, 0.2)';
                c.beginPath();
                c.moveTo(xp, yp);
                for (let k = i + 1; k < j; k++) c.lineTo(k * 4, span - hs[k]);
                c.lineTo(xv, yv);
                c.lineTo(xp + 6, yp + (yv - yp) * 0.45);
                c.closePath();
                c.fill();
                let b = i - 1;
                while (b > 0 && hs[b] < hs[b + 1]) b--;
                c.strokeStyle = rgba(mix(L.color, '#ffffff', 0.4), 0.4);
                c.lineWidth = 1.6;
                c.beginPath();
                for (let k = b; k <= i; k++) {
                    if (k === b) c.moveTo(k * 4, span - hs[k]);
                    else c.lineTo(k * 4, span - hs[k]);
                }
                c.stroke();
            }
        }
        if (L.snow) {
            const thr = L.amp * 0.5;
            c.fillStyle = rgba(mix('#e4f4ff', hz, 0.3 + (L.mist || 0.3) * 0.5), 0.72);
            let i = 0;
            while (i < n) {
                if (hs[i] <= thr) { i++; continue; }
                let j = i;
                while (j < n && hs[j] > thr) j++;
                c.beginPath();
                for (let k = i; k < j; k++) c.lineTo(k * 4, span - hs[k]);
                for (let k = j - 1; k >= i; k--) {
                    const d = Math.min(46, (hs[k] - thr) * 0.3) * (0.55 + 0.9 * pn(k * 4, L.cells * 6 + 1, L.seed + 3)) + 2;
                    c.lineTo(k * 4, span - hs[k] + d);
                }
                c.closePath();
                c.fill();
                i = j;
            }
        }
        if (L.falls) {
            for (let m = 0; m < maxima.length; m += 2) {
                const i = maxima[m];
                const x = i * 4 + 30 + env.r(m, 3) * 30;
                const top = span - crestH(hs, x) + L.amp * 0.18;
                wrapDraw(x, 14, function (xx) {
                    const g = c.createLinearGradient(0, top, 0, span);
                    g.addColorStop(0, 'rgba(200, 244, 255, 0.7)');
                    g.addColorStop(1, 'rgba(120, 200, 240, 0.1)');
                    c.fillStyle = g;
                    c.beginPath();
                    c.moveTo(xx - 5, top);
                    c.lineTo(xx + 5, top);
                    c.lineTo(xx + 12, span);
                    c.lineTo(xx - 12, span);
                    c.closePath();
                    c.fill();
                    c.strokeStyle = 'rgba(255, 255, 255, 0.4)';
                    c.lineWidth = 1.2;
                    c.beginPath();
                    c.moveTo(xx - 2, top + 4);
                    c.lineTo(xx - 5, span);
                    c.moveTo(xx + 3, top + 10);
                    c.lineTo(xx + 6, span);
                    c.stroke();
                });
            }
        }
    }

    function drawBody(env) {
        const { c, L, hs, span } = env;
        const hz = env.biome.sky[env.biome.sky.length - 1];
        const H = span + 24;
        const g = c.createLinearGradient(0, Math.max(0, span - (L.amp || 40) - 10), 0, span);
        g.addColorStop(0, L.color);
        const bottom = mix(L.color, hz, L.mist || 0.3);
        g.addColorStop(1, bottom);
        env.entry.solid = bottom;
        c.fillStyle = g;
        c.beginPath();
        c.moveTo(0, H);
        for (let i = 0; i <= P / 4; i++) c.lineTo(i * 4, span - hs[i]);
        c.lineTo(P, H);
        c.closePath();
        c.fill();
    }

    B.PROPS = PROPS;
    B.decoratePeaks = decoratePeaks;
    B.drawBody = drawBody;
    B.propsCeiling = propsCeiling;
})(typeof window !== 'undefined' ? window : globalThis);
