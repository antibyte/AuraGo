/* Aura Drift - HUD toolkit: canvas-drawn weapon glyphs, driver portraits, the rotating
   minimap, kinetic call-outs and small icons. Pure 2D canvas, no DOM, no THREE.
   aura-drift-hud.js lays everything out; this file only knows how to draw things. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const TAU = Math.PI * 2;
    const FAM = '"Segoe UI","Helvetica Neue",Arial,system-ui,sans-serif';
    const MONO = 'Consolas,"SF Mono","Courier New",monospace';
    const INK = '#0b1020';
    const clamp = AD.clamp;

    /* English fallbacks for keys the locale bundles do not carry yet (T5.4 adds them). */
    const FALLBACK = {
        'hud.final_lap': 'Final lap!', 'hud.lap_of': 'Lap {lap}/{total}', 'hud.eliminated': '{name} is out!',
        'hud.next_out': 'Next out', 'hud.bomb': '{name} has the bomb', 'hud.have_bomb': 'You have the bomb!',
        'hud.pass_it': 'Ram someone to pass it on!', 'hud.clock': 'Time', 'hud.score': 'Score',
        'hud.perfect_start': 'Perfect start!', 'hud.wheelspin': 'Wheelspin!', 'hud.slingshot': 'Slingshot!',
        'hud.drift1': 'Blue turbo', 'hud.drift2': 'Orange turbo', 'hud.drift3': 'Pink turbo',
        'hud.near_miss': 'Close call!', 'hud.boost': 'Boost!', 'hud.strike': 'Strike!',
        'hud.cow_bowling': 'Cow bowling!', 'hud.respawn': 'Back on track', 'hud.shield': 'Shield',
        'hud.frozen': 'Frozen', 'hud.toast': 'Toast!', 'hud.disco': 'Disco fever', 'hud.cloak': 'Cloaked',
        'hud.overdrive': 'Overdrive', 'hud.wrong_way': 'Wrong way!', 'hud.sweeper': 'Sweeper wagon! {seconds} s left',
        'landing.perfect': 'Perfect landing!', 'landing.good': 'Nice landing', 'landing.wobble': 'Wobbly',
        'landing.pancake': 'Pancake!', 'medal.bronze': 'Bronze', 'medal.silver': 'Silver', 'medal.gold': 'Gold',
        'medal.none': 'No medal'
    };
    function humanise(key) {
        const last = String(key).split('.').pop().replace(/_/g, ' ');
        return last.replace(/\b[a-z]/g, m => m.toUpperCase());
    }
    /* Translate `aura_drift.<sub>`; a missing key comes back as the key itself. */
    function makeTr(t) {
        return function (sub, values, fb) {
            const key = 'aura_drift.' + sub;
            let out = typeof t === 'function' ? t(key) : key;
            if (!out || out === key) out = FALLBACK[sub] || (fb !== undefined ? fb : humanise(sub));
            return values ? AD.fmt(out, values) : out;
        };
    }

    /* ---- drawing helpers --------------------------------------------------------------- */
    const kit = AD.hudKit = { FAM, MONO, TAU, makeTr, humanise };

    /* Parallelogram path: the whole HUD leans forward, like the car does. */
    kit.para = function (c, x, y, w, h, skew) {
        c.beginPath();
        c.moveTo(x + skew, y); c.lineTo(x + w + skew, y); c.lineTo(x + w, y + h); c.lineTo(x, y + h);
        c.closePath();
    };

    /* Outlined text. opts: align, weight, italic (default true), mono, stroke, sw, spacing, alpha. */
    kit.text = function (c, s, x, y, size, fill, opts) {
        const o = opts || {};
        c.font = (o.italic === false ? '' : 'italic ') + (o.weight || 800) + ' ' + size + 'px ' + (o.mono ? MONO : FAM);
        c.textAlign = o.align || 'left';
        c.textBaseline = 'alphabetic';
        if (o.spacing) c.letterSpacing = o.spacing + 'px';
        if (o.alpha !== undefined) c.globalAlpha = o.alpha;
        const sw = o.sw === undefined ? clamp(size * 0.16, 2, 7) : o.sw;
        if (sw > 0) {
            c.lineJoin = 'round';
            c.lineWidth = sw;
            c.strokeStyle = o.stroke || 'rgba(4,8,18,0.82)';
            c.strokeText(s, x, y);
        }
        c.fillStyle = fill;
        c.fillText(s, x, y);
        if (o.spacing) c.letterSpacing = '0px';
        if (o.alpha !== undefined) c.globalAlpha = 1;
    };

    /* Wrap to a width, honouring the font already set on the context. */
    kit.wrap = function (c, s, maxW, maxLines) {
        const words = String(s).split(/\s+/), lines = [];
        let line = '';
        for (const w of words) {
            const test = line ? line + ' ' + w : w;
            if (line && c.measureText(test).width > maxW) { lines.push(line); line = w; } else line = test;
        }
        if (line) lines.push(line);
        if (lines.length > maxLines) { lines.length = maxLines; lines[maxLines - 1] += '...'; }
        return lines;
    };

    kit.number = (function () {
        let last = NaN, text = '0';
        return function (n) {
            n = Math.round(n);
            if (n !== last) {
                last = n;
                text = String(Math.abs(n)).replace(/\B(?=(\d{3})+(?!\d))/g, ',');
                if (n < 0) text = '-' + text;
            }
            return text;
        };
    })();

    function ink(c, fill, lw) {
        if (fill) { c.fillStyle = fill; c.fill(); }
        c.lineWidth = lw || 0.09; c.strokeStyle = INK; c.lineJoin = 'round'; c.lineCap = 'round'; c.stroke();
    }
    function ell(c, x, y, rx, ry, rot) { c.beginPath(); c.ellipse(x, y, rx, ry, rot || 0, 0, TAU); }
    function poly(c, p) {
        c.beginPath(); c.moveTo(p[0], p[1]);
        for (let i = 2; i < p.length; i += 2) c.lineTo(p[i], p[i + 1]);
        c.closePath();
    }
    function line(c, col, w, a, b, x, y) { c.beginPath(); c.moveTo(a, b); c.lineTo(x, y); c.lineWidth = w; c.strokeStyle = col; c.lineCap = 'round'; c.stroke(); }
    function star(c, x, y, r, col) {
        poly(c, [x, y - r, x + r * 0.22, y - r * 0.22, x + r, y, x + r * 0.22, y + r * 0.22, x, y + r, x - r * 0.22, y + r * 0.22, x - r, y, x - r * 0.22, y - r * 0.22]);
        c.fillStyle = col; c.fill();
    }

    /* ---- weapon glyphs: unit box [-1, 1], y down -------------------------------------------- */
    const GLYPHS = {
        oil(c) {
            c.beginPath(); c.moveTo(-0.92, 0.12);
            c.bezierCurveTo(-0.95, -0.45, -0.35, -0.62, 0.05, -0.42);
            c.bezierCurveTo(0.5, -0.66, 1, -0.2, 0.86, 0.26);
            c.bezierCurveTo(0.92, 0.72, 0.25, 0.88, -0.18, 0.66);
            c.bezierCurveTo(-0.68, 0.84, -1, 0.52, -0.92, 0.12);
            c.closePath(); ink(c, '#2a2a3d');
            c.lineWidth = 0.08;
            c.strokeStyle = '#7c5cff'; c.beginPath(); c.arc(-0.08, 0.06, 0.5, 3.5, 4.55); c.stroke();
            c.strokeStyle = '#2dd4bf'; c.beginPath(); c.arc(-0.08, 0.06, 0.5, 4.7, 5.5); c.stroke();
            c.strokeStyle = '#ff5ab0'; c.beginPath(); c.arc(0.1, 0.12, 0.3, 0.2, 1.4); c.stroke();
            c.beginPath(); c.moveTo(0.55, -0.95); c.bezierCurveTo(0.8, -0.65, 0.85, -0.45, 0.55, -0.4);
            c.bezierCurveTo(0.25, -0.45, 0.3, -0.65, 0.55, -0.95); ink(c, '#2a2a3d');
        },
        gnome_mine(c) {
            poly(c, [-0.42, 0.0, 0.42, 0.0, 0.0, 0.98]); ink(c, '#f8fafc');
            ell(c, 0, -0.02, 0.36, 0.34); ink(c, '#fcd5b0');
            ell(c, 0, 0.05, 0.09, 0.08); ink(c, '#f58a9a', 0.05);
            poly(c, [-0.62, -0.22, 0.62, -0.22, 0.0, -1.0]); ink(c, '#ef4444');
            line(c, '#fecaca', 0.07, -0.34, -0.38, 0.1, -0.7);
            ell(c, 0, -0.98, 0.1, 0.1); ink(c, '#fde047', 0.06);
            c.fillStyle = INK; ell(c, -0.14, -0.06, 0.04, 0.05); c.fill(); ell(c, 0.14, -0.06, 0.04, 0.05); c.fill();
        },
        emp(c) {
            c.lineWidth = 0.1; c.strokeStyle = '#38bdf8';
            c.beginPath(); c.arc(0, 0, 0.92, 0, TAU); c.stroke();
            c.strokeStyle = '#7dd3fc'; c.globalAlpha = 0.6; c.beginPath(); c.arc(0, 0, 0.62, 0, TAU); c.stroke(); c.globalAlpha = 1;
            poly(c, [0.12, -0.8, -0.42, 0.1, -0.04, 0.1, -0.18, 0.82, 0.44, -0.16, 0.04, -0.16, 0.34, -0.8]);
            ink(c, '#fde047');
        },
        rocket(c) {
            c.save(); c.rotate(-Math.PI / 4);
            c.beginPath(); c.moveTo(0.1, 0.78); c.quadraticCurveTo(0.34, 1.1, 0, 1.2); c.quadraticCurveTo(-0.34, 1.1, -0.1, 0.78); c.closePath(); ink(c, '#fb923c');
            poly(c, [-0.3, 0.4, -0.7, 0.8, -0.3, 0.74]); ink(c, '#ef4444');
            poly(c, [0.3, 0.4, 0.7, 0.8, 0.3, 0.74]); ink(c, '#ef4444');
            c.beginPath(); c.moveTo(-0.3, 0.78); c.lineTo(-0.3, -0.3); c.quadraticCurveTo(-0.3, -0.85, 0, -1.1);
            c.quadraticCurveTo(0.3, -0.85, 0.3, -0.3); c.lineTo(0.3, 0.78); c.closePath(); ink(c, '#f1f5f9');
            c.beginPath(); c.moveTo(-0.3, -0.3); c.quadraticCurveTo(-0.3, -0.85, 0, -1.1); c.quadraticCurveTo(0.3, -0.85, 0.3, -0.3); c.closePath(); ink(c, '#ef4444');
            ell(c, 0, 0.12, 0.15, 0.15); ink(c, '#38bdf8', 0.06);
            c.restore();
        },
        mg(c) {
            c.save(); c.rotate(-0.5);
            for (let i = 0; i < 3; i++) line(c, 'rgba(255,255,255,0.55)', 0.06, -1.0, -0.5 + i * 0.4 + 0.22, -0.55, -0.5 + i * 0.4 + 0.22);
            c.beginPath(); c.moveTo(-0.5, -0.3); c.lineTo(0.5, -0.3); c.arc(0.5, 0, 0.3, -Math.PI / 2, Math.PI / 2); c.lineTo(-0.5, 0.3); c.arc(-0.5, 0, 0.3, Math.PI / 2, -Math.PI / 2); c.closePath();
            ink(c, '#c26a30');
            c.save(); c.clip();
            for (let i = -2; i <= 2; i++) line(c, '#7c3a14', 0.07, i * 0.3 - 0.1, -0.35, i * 0.3 + 0.1, 0.35);
            c.restore();
            c.beginPath(); c.moveTo(-0.55, 0.0);
            for (let i = 0; i < 6; i++) c.lineTo(-0.55 + (i + 1) * 0.2, i % 2 ? 0.12 : -0.12);
            c.lineWidth = 0.08; c.strokeStyle = '#facc15'; c.stroke();
            ell(c, 0.28, 0.5, 0.2, 0.2); c.restore();
        },
        shield(c) {
            c.beginPath(); c.moveTo(0, -0.98); c.bezierCurveTo(0.5, -0.8, 0.8, -0.78, 0.9, -0.7);
            c.bezierCurveTo(0.95, 0.1, 0.5, 0.7, 0, 1.0); c.bezierCurveTo(-0.5, 0.7, -0.95, 0.1, -0.9, -0.7);
            c.bezierCurveTo(-0.8, -0.78, -0.5, -0.8, 0, -0.98); c.closePath(); ink(c, '#14b8a6');
            c.beginPath(); c.moveTo(0, -0.7); c.bezierCurveTo(0.4, -0.58, 0.6, -0.56, 0.68, -0.5); c.bezierCurveTo(0.7, 0.05, 0.38, 0.5, 0, 0.74);
            c.lineTo(0, -0.7); c.closePath(); c.fillStyle = '#5eead4'; c.fill();
            star(c, 0, -0.05, 0.38, '#f0fdfa');
        },
        ducks(c) {
            ell(c, 0.05, 0.38, 0.78, 0.46); ink(c, '#fde047');
            c.beginPath(); c.arc(-0.34, -0.38, 0.4, 0, TAU); ink(c, '#fde047');
            c.beginPath(); c.moveTo(-0.7, -0.4); c.quadraticCurveTo(-1.05, -0.38, -0.98, -0.2); c.quadraticCurveTo(-0.85, -0.14, -0.66, -0.22); c.closePath(); ink(c, '#fb923c', 0.07);
            ell(c, -0.44, -0.5, 0.06, 0.07); c.fillStyle = INK; c.fill();
            c.beginPath(); c.moveTo(0.2, 0.28); c.quadraticCurveTo(0.6, 0.1, 0.72, 0.4); c.quadraticCurveTo(0.45, 0.62, 0.15, 0.5); c.closePath(); ink(c, '#facc15', 0.06);
            ell(c, 0.78, 0.2, 0.12, 0.12); ink(c, '#fde047', 0.05);
        },
        railgun(c) {
            c.save(); c.rotate(-0.45);
            line(c, 'rgba(165,243,252,0.9)', 0.1, 0.55, 0, 1.1, 0);
            line(c, '#ffffff', 0.04, 0.55, 0, 1.1, 0);
            c.beginPath(); c.rect(-0.95, -0.2, 1.5, 0.4); ink(c, '#64748b');
            c.beginPath(); c.rect(-0.95, -0.14, 0.6, 0.28); ink(c, '#334155', 0.05);
            for (let i = 0; i < 3; i++) { c.beginPath(); c.rect(-0.2 + i * 0.26, -0.3, 0.12, 0.6); ink(c, '#22d3ee', 0.06); }
            poly(c, [-0.8, 0.2, -0.55, 0.2, -0.65, 0.7, -0.9, 0.7]); ink(c, '#475569');
            c.restore();
        },
        flamethrower(c) {
            c.beginPath(); c.rect(-0.95, -0.2, 0.78, 0.4); ink(c, '#94a3b8');
            c.beginPath(); c.rect(-0.2, -0.13, 0.28, 0.26); ink(c, '#475569');
            c.beginPath(); c.moveTo(0.1, -0.1); c.bezierCurveTo(0.5, -0.8, 0.95, -0.35, 0.98, 0.0); c.bezierCurveTo(0.95, 0.4, 0.5, 0.8, 0.1, 0.1); c.closePath(); ink(c, '#f97316');
            c.beginPath(); c.moveTo(0.14, -0.05); c.bezierCurveTo(0.45, -0.4, 0.72, -0.2, 0.74, 0.0); c.bezierCurveTo(0.72, 0.22, 0.45, 0.42, 0.14, 0.05); c.closePath(); c.fillStyle = '#fde047'; c.fill();
        },
        frost(c) {
            ell(c, 0, 0, 0.9, 0.9); ink(c, '#38bdf8');
            ell(c, -0.25, -0.3, 0.3, 0.2, -0.6); c.fillStyle = 'rgba(255,255,255,0.35)'; c.fill();
            for (let i = 0; i < 3; i++) {
                c.save(); c.rotate(i * Math.PI / 3);
                line(c, '#f0f9ff', 0.1, 0, -0.68, 0, 0.68);
                for (const s of [-1, 1]) { line(c, '#f0f9ff', 0.07, 0, s * 0.44, 0.18, s * 0.62); line(c, '#f0f9ff', 0.07, 0, s * 0.44, -0.18, s * 0.62); }
                c.restore();
            }
        },
        air_horn(c) {
            c.beginPath(); c.rect(-0.95, -0.14, 0.4, 0.28); ink(c, '#475569');
            poly(c, [-0.58, -0.2, 0.3, -0.7, 0.3, 0.7, -0.58, 0.2]); ink(c, '#fbbf24');
            ell(c, 0.3, 0, 0.14, 0.7); ink(c, '#fde68a');
            c.lineWidth = 0.09; c.strokeStyle = '#fef08a'; c.lineCap = 'round';
            for (let i = 0; i < 3; i++) { c.globalAlpha = 1 - i * 0.25; c.beginPath(); c.arc(0.3, 0, 0.42 + i * 0.2, -0.7, 0.7); c.stroke(); }
            c.globalAlpha = 1;
        },
        black_hole(c) {
            const g = c.createRadialGradient(0, 0, 0.05, 0, 0, 0.9);
            g.addColorStop(0, '#000'); g.addColorStop(0.55, '#1e0a3c'); g.addColorStop(0.85, '#7c3aed'); g.addColorStop(1, 'rgba(124,58,237,0)');
            c.fillStyle = g; c.beginPath(); c.arc(0, 0, 0.98, 0, TAU); c.fill();
            c.save(); c.rotate(-0.45);
            c.lineWidth = 0.13; c.strokeStyle = '#fb923c'; ell(c, 0, 0, 0.92, 0.28); c.stroke();
            c.lineWidth = 0.05; c.strokeStyle = '#fde047'; ell(c, 0, 0, 0.92, 0.28); c.stroke();
            c.restore();
            ell(c, 0, 0, 0.38, 0.38); c.fillStyle = '#000'; c.fill();
            c.lineWidth = 0.06; c.strokeStyle = '#c4b5fd';
            for (let i = 0; i < 3; i++) { c.beginPath(); c.arc(0, 0, 0.55, i * 2.1, i * 2.1 + 1.0); c.stroke(); }
        },
        disco(c) {
            line(c, '#cbd5e1', 0.06, 0, -1.0, 0, -0.78);
            c.beginPath(); c.arc(0, 0.1, 0.78, 0, TAU); ink(c, '#cbd5e1');
            c.save(); c.beginPath(); c.arc(0, 0.1, 0.76, 0, TAU); c.clip();
            const cols = ['#f472b6', '#38bdf8', '#fde047', '#a78bfa', '#ffffff', '#5eead4'];
            for (let r = 0; r < 7; r++) for (let q = 0; q < 7; q++) {
                c.fillStyle = cols[(r * 3 + q * 5 + (r ^ q)) % cols.length];
                c.globalAlpha = 0.5 + ((r * 7 + q * 3) % 5) * 0.1;
                c.fillRect(-0.8 + q * 0.23, -0.7 + r * 0.23, 0.2, 0.2);
            }
            c.globalAlpha = 1;
            ell(c, -0.28, -0.28, 0.3, 0.2, -0.7); c.fillStyle = 'rgba(255,255,255,0.5)'; c.fill();
            c.restore();
            c.beginPath(); c.arc(0, 0.1, 0.78, 0, TAU); c.lineWidth = 0.09; c.strokeStyle = INK; c.stroke();
            star(c, 0.78, -0.62, 0.2, '#ffffff'); star(c, -0.85, 0.5, 0.14, '#fde047');
        },
        glove(c) {
            c.beginPath(); c.moveTo(-0.5, 0.5); c.bezierCurveTo(-0.85, 0.1, -0.7, -0.8, 0.1, -0.85);
            c.bezierCurveTo(0.85, -0.85, 0.95, -0.1, 0.62, 0.22); c.lineTo(0.5, 0.5); c.closePath(); ink(c, '#ef4444');
            c.beginPath(); c.moveTo(-0.66, 0.0); c.bezierCurveTo(-1.0, -0.15, -0.95, 0.3, -0.5, 0.36); c.closePath(); ink(c, '#dc2626', 0.07);
            c.beginPath(); c.moveTo(-0.15, -0.55); c.quadraticCurveTo(0.2, -0.75, 0.55, -0.45); c.lineWidth = 0.07; c.strokeStyle = '#fecaca'; c.stroke();
            c.beginPath(); c.rect(-0.5, 0.5, 1.0, 0.45); ink(c, '#f8fafc');
            line(c, '#cbd5e1', 0.06, -0.5, 0.72, 0.5, 0.72);
        },
        harpoon(c) {
            c.save(); c.rotate(-0.7);
            line(c, '#fbbf24', 0.05, -1.05, 0.22, -0.8, 0.0);
            c.beginPath(); c.moveTo(-1.05, 0.22); c.bezierCurveTo(-0.9, 0.6, -0.6, -0.4, -0.3, 0.1); c.lineWidth = 0.05; c.strokeStyle = '#e2e8f0'; c.stroke();
            c.beginPath(); c.rect(-0.8, -0.07, 1.2, 0.14); ink(c, '#a16207', 0.06);
            poly(c, [0.4, -0.3, 1.05, 0.0, 0.4, 0.3, 0.52, 0.0]); ink(c, '#cbd5e1');
            poly(c, [0.4, -0.3, 0.7, -0.14, 0.52, 0.0]); c.fillStyle = '#f1f5f9'; c.fill();
            c.restore();
        },
        cow(c) {
            poly(c, [-0.7, -0.5, -0.98, -0.82, -0.52, -0.7]); ink(c, '#fde68a', 0.07);
            poly(c, [0.7, -0.5, 0.98, -0.82, 0.52, -0.7]); ink(c, '#fde68a', 0.07);
            ell(c, -0.82, -0.2, 0.26, 0.14, 0.4); ink(c, '#f5f5f4', 0.07);
            ell(c, 0.82, -0.2, 0.26, 0.14, -0.4); ink(c, '#f5f5f4', 0.07);
            c.beginPath(); c.moveTo(-0.62, -0.62); c.quadraticCurveTo(0, -0.85, 0.62, -0.62); c.quadraticCurveTo(0.74, 0.1, 0.5, 0.85); c.lineTo(-0.5, 0.85); c.quadraticCurveTo(-0.74, 0.1, -0.62, -0.62); c.closePath(); ink(c, '#f8fafc');
            c.save(); c.clip(); ell(c, -0.42, -0.45, 0.3, 0.26, 0.3); c.fillStyle = '#1e293b'; c.fill(); ell(c, 0.5, 0.0, 0.2, 0.3); c.fill(); c.restore();
            ell(c, 0, 0.5, 0.46, 0.34); ink(c, '#f9a8d4');
            c.fillStyle = INK; ell(c, -0.16, 0.5, 0.06, 0.09); c.fill(); ell(c, 0.16, 0.5, 0.06, 0.09); c.fill();
            ell(c, -0.26, -0.18, 0.08, 0.1); c.fill(); ell(c, 0.26, -0.18, 0.08, 0.1); c.fill();
        },
        cloak(c) {
            c.beginPath(); c.moveTo(-0.7, 0.9);
            c.lineTo(-0.7, -0.1); c.bezierCurveTo(-0.7, -1.0, 0.7, -1.0, 0.7, -0.1); c.lineTo(0.7, 0.9);
            c.lineTo(0.47, 0.68); c.lineTo(0.23, 0.9); c.lineTo(0, 0.68); c.lineTo(-0.23, 0.9); c.lineTo(-0.47, 0.68); c.closePath();
            c.globalAlpha = 0.72; c.fillStyle = '#99f6e4'; c.fill(); c.globalAlpha = 1;
            c.setLineDash([0.16, 0.12]); c.lineWidth = 0.09; c.strokeStyle = INK; c.stroke(); c.setLineDash([]);
            ell(c, -0.25, -0.15, 0.13, 0.2); ink(c, '#0f766e', 0.04); ell(c, 0.25, -0.15, 0.13, 0.2); ink(c, '#0f766e', 0.04);
            ell(c, 0, 0.3, 0.12, 0.1); ink(c, '#0f766e', 0.04);
        },
        overdrive(c) {
            c.beginPath(); c.moveTo(0, -1.0); c.bezierCurveTo(0.7, -0.4, 0.95, 0.15, 0.55, 0.7); c.bezierCurveTo(0.35, 0.95, -0.35, 0.95, -0.55, 0.7);
            c.bezierCurveTo(-0.95, 0.15, -0.7, -0.4, 0, -1.0); c.closePath(); ink(c, '#f97316');
            c.beginPath(); c.moveTo(0, -0.45); c.bezierCurveTo(0.4, -0.05, 0.5, 0.25, 0.28, 0.6); c.bezierCurveTo(0.12, 0.8, -0.12, 0.8, -0.28, 0.6);
            c.bezierCurveTo(-0.5, 0.25, -0.4, -0.05, 0, -0.45); c.closePath(); c.fillStyle = '#fde047'; c.fill();
            c.lineWidth = 0.13; c.strokeStyle = '#f8fafc'; c.lineCap = 'round'; c.lineJoin = 'round';
            c.beginPath(); c.moveTo(-0.22, 0.12); c.lineTo(0, 0.34); c.lineTo(0.22, 0.12); c.stroke();
            c.beginPath(); c.moveTo(-0.22, -0.14); c.lineTo(0, 0.08); c.lineTo(0.22, -0.14); c.stroke();
        }
    };
    kit.GLYPHS = GLYPHS;

    const glyphCache = new Map();
    /* Draws the weapon icon of `id` centred at (x, y), `size` CSS px wide. */
    AD.drawWeaponGlyph = function (c, id, x, y, size, dpr) {
        const draw = GLYPHS[id];
        if (!draw) return;
        const px = Math.max(8, Math.round(size * (dpr || 1)));
        const key = id + '@' + px;
        let img = glyphCache.get(key);
        if (!img) {
            img = document.createElement('canvas');
            img.width = img.height = px;
            const g = img.getContext('2d');
            g.translate(px / 2, px / 2);
            g.scale(px * 0.5 * 0.9, px * 0.5 * 0.9);
            draw(g);
            glyphCache.set(key, img);
        }
        c.drawImage(img, x - size / 2, y - size / 2, size, size);
    };

    /* ---- driver portraits -------------------------------------------------------------------- */
    const SKINS = ['#f2c9a5', '#e0a97c', '#c88b5e', '#f6d6b8'];
    const portraitCache = new Map();
    function paintPortrait(g, id, s, d) {
        const marshal = !d;
        const helmet = d ? d.portrait.helmet : '#facc15';
        const skin = SKINS[AD.hashString(id) % SKINS.length];
        const hair = d ? d.portrait.hair : '#44403c';
        g.save();
        g.beginPath(); g.arc(s / 2, s / 2, s / 2, 0, TAU); g.clip();
        const bg = g.createLinearGradient(0, 0, 0, s);
        bg.addColorStop(0, '#2a3358'); bg.addColorStop(1, '#0d1226');
        g.fillStyle = bg; g.fillRect(0, 0, s, s);
        g.globalAlpha = 0.35; g.fillStyle = helmet; g.fillRect(0, 0, s, s); g.globalAlpha = 1;
        /* shoulders and collar */
        g.beginPath(); g.ellipse(s * 0.5, s * 1.02, s * 0.5, s * 0.3, 0, 0, TAU);
        g.fillStyle = marshal ? '#fb923c' : helmet; g.fill();
        g.fillStyle = marshal ? '#fde047' : '#f8fafc'; g.fillRect(s * 0.28, s * 0.86, s * 0.07, s * 0.2); g.fillRect(s * 0.65, s * 0.86, s * 0.07, s * 0.2);
        g.fillStyle = skin; g.fillRect(s * 0.43, s * 0.7, s * 0.14, s * 0.2);
        /* hair behind the face */
        g.beginPath(); g.ellipse(s * 0.5, s * 0.5, s * 0.3, s * 0.32, 0, 0, TAU); g.fillStyle = hair; g.fill();
        /* face */
        g.beginPath(); g.ellipse(s * 0.5, s * 0.56, s * 0.235, s * 0.29, 0, 0, TAU); g.fillStyle = skin; g.fill();
        g.fillStyle = 'rgba(160,80,50,0.35)';
        g.beginPath(); g.ellipse(s * 0.5, s * 0.64, s * 0.04, s * 0.03, 0, 0, TAU); g.fill();
        if (marshal) {
            /* eyes, brows, whistle puff and a flag cap */
            g.fillStyle = '#fff'; g.beginPath(); g.ellipse(s * 0.41, s * 0.5, s * 0.045, s * 0.05, 0, 0, TAU); g.ellipse(s * 0.59, s * 0.5, s * 0.045, s * 0.05, 0, 0, TAU); g.fill();
            g.fillStyle = INK; g.beginPath(); g.arc(s * 0.42, s * 0.51, s * 0.02, 0, TAU); g.arc(s * 0.6, s * 0.51, s * 0.02, 0, TAU); g.fill();
            g.strokeStyle = hair; g.lineWidth = s * 0.03; g.lineCap = 'round';
            g.beginPath(); g.moveTo(s * 0.35, s * 0.42); g.lineTo(s * 0.46, s * 0.44); g.moveTo(s * 0.65, s * 0.42); g.lineTo(s * 0.54, s * 0.44); g.stroke();
            g.fillStyle = hair; g.beginPath(); g.moveTo(s * 0.34, s * 0.68); g.quadraticCurveTo(s * 0.5, s * 0.6, s * 0.66, s * 0.68); g.quadraticCurveTo(s * 0.5, s * 0.75, s * 0.34, s * 0.68); g.fill();
            g.fillStyle = '#e2e8f0'; g.beginPath(); g.arc(s * 0.5, s * 0.78, s * 0.045, 0, TAU); g.fill();
            g.beginPath(); g.moveTo(s * 0.2, s * 0.4); g.quadraticCurveTo(s * 0.5, s * -0.02, s * 0.8, s * 0.4); g.lineTo(s * 0.8, s * 0.4); g.closePath();
            g.fillStyle = helmet; g.fill();
            g.fillStyle = INK; g.fillRect(s * 0.2, s * 0.37, s * 0.6, s * 0.05);
            g.fillStyle = '#fff'; for (let i = 0; i < 6; i++) if (i % 2) g.fillRect(s * (0.2 + i * 0.1), s * 0.37, s * 0.1, s * 0.05);
            g.beginPath(); g.moveTo(s * 0.5, s * 0.42); g.quadraticCurveTo(s * 0.9, s * 0.4, s * 0.86, s * 0.47); g.quadraticCurveTo(s * 0.7, s * 0.45, s * 0.5, s * 0.45); g.fillStyle = '#ca8a04'; g.fill();
        } else {
            /* eyes show through the visor */
            g.fillStyle = d.portrait.visor;
            g.beginPath(); g.moveTo(s * 0.24, s * 0.4); g.quadraticCurveTo(s * 0.5, s * 0.34, s * 0.76, s * 0.4); g.lineTo(s * 0.72, s * 0.57); g.quadraticCurveTo(s * 0.5, s * 0.61, s * 0.28, s * 0.57); g.closePath();
            g.globalAlpha = 0.94; g.fill(); g.globalAlpha = 1;
            g.fillStyle = 'rgba(255,255,255,0.8)';
            g.beginPath(); g.ellipse(s * 0.4, s * 0.49, s * 0.032, s * 0.04, 0, 0, TAU); g.ellipse(s * 0.6, s * 0.49, s * 0.032, s * 0.04, 0, 0, TAU); g.fill();
            g.strokeStyle = 'rgba(255,255,255,0.28)'; g.lineWidth = s * 0.035;
            g.beginPath(); g.moveTo(s * 0.3, s * 0.57); g.lineTo(s * 0.4, s * 0.4); g.moveTo(s * 0.42, s * 0.58); g.lineTo(s * 0.5, s * 0.4); g.stroke();
            /* helmet dome with a stripe */
            g.beginPath(); g.moveTo(s * 0.2, s * 0.46); g.bezierCurveTo(s * 0.14, s * 0.0, s * 0.86, s * 0.0, s * 0.8, s * 0.46);
            g.lineTo(s * 0.76, s * 0.4); g.quadraticCurveTo(s * 0.5, s * 0.33, s * 0.24, s * 0.4); g.closePath();
            g.fillStyle = helmet; g.fill();
            g.fillStyle = 'rgba(255,255,255,0.8)'; g.fillRect(s * 0.47, s * 0.08, s * 0.06, s * 0.27);
            g.fillStyle = 'rgba(255,255,255,0.3)'; g.beginPath(); g.ellipse(s * 0.34, s * 0.2, s * 0.07, s * 0.1, -0.5, 0, TAU); g.fill();
            /* sideburn tufts and mouth */
            g.fillStyle = hair; g.fillRect(s * 0.26, s * 0.56, s * 0.04, s * 0.1); g.fillRect(s * 0.7, s * 0.56, s * 0.04, s * 0.1);
            if (d.portrait.moustache) {
                g.beginPath(); g.moveTo(s * 0.5, s * 0.67);
                g.bezierCurveTo(s * 0.4, s * 0.62, s * 0.3, s * 0.68, s * 0.27, s * 0.76); g.bezierCurveTo(s * 0.36, s * 0.7, s * 0.44, s * 0.72, s * 0.5, s * 0.7);
                g.bezierCurveTo(s * 0.56, s * 0.72, s * 0.64, s * 0.7, s * 0.73, s * 0.76); g.bezierCurveTo(s * 0.7, s * 0.68, s * 0.6, s * 0.62, s * 0.5, s * 0.67);
                g.fillStyle = hair; g.fill();
            }
            g.strokeStyle = '#7f1d1d'; g.lineWidth = s * 0.025; g.lineCap = 'round';
            g.beginPath(); g.arc(s * 0.5, s * 0.73, s * 0.07, 0.25, Math.PI - 0.25); g.stroke();
        }
        g.restore();
        g.beginPath(); g.arc(s / 2, s / 2, s / 2 - 1.5, 0, TAU); g.lineWidth = 3; g.strokeStyle = 'rgba(255,255,255,0.85)'; g.stroke();
    }
    /* (x, y) is the top left of the size x size square. */
    AD.drawPortrait = function (c, driverId, x, y, size, dpr) {
        const px = Math.max(16, Math.round(size * (dpr || 1)));
        const key = driverId + '@' + px;
        let img = portraitCache.get(key);
        if (!img) {
            const d = AD.DRIVERS.find(e => e.id === driverId) || null;
            img = document.createElement('canvas');
            img.width = img.height = px;
            paintPortrait(img.getContext('2d'), String(driverId), px, d);
            portraitCache.set(key, img);
        }
        c.drawImage(img, x, y, size, size);
    };

    /* ---- minimap ----------------------------------------------------------------------------- */
    const pathCache = new WeakMap();
    function trackPath(track) {
        let p = pathCache.get(track);
        if (!p) {
            p = new Path2D();
            const s = track.samples;
            for (let i = 0; i < s.length; i += 3) { if (i) p.lineTo(s[i].x, s[i].y); else p.moveTo(s[i].x, s[i].y); }
            p.closePath();
            pathCache.set(track, p);
        }
        return p;
    }
    const HAZARD = { oil: '#1e1b4b', gnome_mine: '#f97316', ice: '#7dd3fc', black_hole: '#a855f7', disco: '#f472b6', cow_shadow: '#fde68a' };
    const VIEW = 230;           /* metres from the centre to the rim */

    /* Circular, track-up minimap: the world rotates so the camera heading points up. */
    AD.drawMinimap = function (c, race, x, y, size, heading, time) {
        const track = race.track, me = race.cars[0].v;
        const r = size / 2, cx = x + r, cy = y + r;
        const theta = AD.screenRotation(heading || 0);
        const S = (r - 4) / VIEW;
        const cosT = Math.cos(theta), sinT = Math.sin(theta);
        c.save();
        c.beginPath(); c.arc(cx, cy, r, 0, TAU);
        c.fillStyle = 'rgba(5,9,20,0.6)'; c.fill();
        c.clip();
        c.save();
        c.translate(cx, cy); c.rotate(theta); c.scale(S, S); c.translate(-me.x, -me.y);
        c.lineJoin = 'round'; c.lineCap = 'round';
        const path = trackPath(track);
        c.lineWidth = track.width * 1.1 + 5 / S; c.strokeStyle = 'rgba(2,5,14,0.92)'; c.stroke(path);
        c.lineWidth = track.width * 1.0; c.strokeStyle = 'rgba(176,210,255,0.78)'; c.stroke(path);
        const p0 = AD.pointAt(track, 0);
        c.lineWidth = 3 / S; c.strokeStyle = '#ffffff';
        c.beginPath(); c.moveTo(p0.x - p0.nx * track.width * 0.6, p0.y - p0.ny * track.width * 0.6);
        c.lineTo(p0.x + p0.nx * track.width * 0.6, p0.y + p0.ny * track.width * 0.6); c.stroke();
        for (const h of race.hazards) {
            c.globalAlpha = 0.6; c.fillStyle = HAZARD[h.type] || '#fff';
            c.beginPath(); c.arc(h.x, h.y, Math.max(h.r, 5 / S), 0, TAU); c.fill();
        }
        c.globalAlpha = 1;
        c.restore();
        const toMap = (wx, wy) => {
            const dx = wx - me.x, dy = wy - me.y;
            return [(dx * cosT - dy * sinT) * S, (dx * sinT + dy * cosT) * S];
        };
        const rim = r - 6;
        c.translate(cx, cy);
        const fs = race.features;
        if (fs && fs.crossings) {
            for (const cr of fs.crossings) {
                const m = toMap(cr.x, cr.y);
                if (Math.hypot(m[0], m[1]) > rim) continue;
                poly(c, [m[0], m[1] - 6, m[0] + 5.5, m[1] + 4, m[0] - 5.5, m[1] + 4]);
                c.fillStyle = '#fbbf24'; c.fill(); c.lineWidth = 1.2; c.strokeStyle = INK; c.stroke();
            }
        }
        const holder = race.bomb ? race.bomb.holder : -1;
        for (let i = 1; i < race.cars.length; i++) {
            const car = race.cars[i];
            if (car.v.out) continue;
            let m = toMap(car.v.x, car.v.y);
            const d = Math.hypot(m[0], m[1]);
            const edge = d > rim;
            if (edge) m = [m[0] / d * rim, m[1] / d * rim];
            c.globalAlpha = edge ? 0.65 : 1;
            c.beginPath(); c.arc(m[0], m[1], edge ? 3 : 4, 0, TAU);
            c.fillStyle = car.v.car.colour; c.fill(); c.lineWidth = 1.6; c.strokeStyle = INK; c.stroke();
            if (holder === i) {
                c.beginPath(); c.arc(m[0], m[1], 7 + Math.sin((time || 0) * 10) * 1.5, 0, TAU);
                c.lineWidth = 2; c.strokeStyle = '#ff3b3b'; c.stroke();
            }
            c.globalAlpha = 1;
        }
        if (race.ghost && race.ghost.active) {
            const m = toMap(race.ghost.v.x, race.ghost.v.y);
            if (Math.hypot(m[0], m[1]) < rim) {
                c.beginPath(); c.arc(m[0], m[1], 4, 0, TAU); c.lineWidth = 1.6; c.strokeStyle = '#9fd6ff'; c.stroke();
            }
        }
        /* The player: an arrow, upright when the camera follows the nose. */
        c.save();
        c.rotate(me.psi + theta + Math.PI / 2);
        poly(c, [0, -8, 6, 7, 0, 3.5, -6, 7]);
        c.fillStyle = '#ffffff'; c.fill(); c.lineWidth = 1.6; c.strokeStyle = INK; c.stroke();
        c.restore();
        c.restore();
        c.beginPath(); c.arc(cx, cy, r - 1, 0, TAU);
        c.lineWidth = 2.5; c.strokeStyle = 'rgba(94,234,255,0.8)'; c.stroke();
        c.beginPath(); c.arc(cx, cy, r + 2.5, 0, TAU);
        c.lineWidth = 1.5; c.strokeStyle = 'rgba(4,8,18,0.7)'; c.stroke();
    };

    /* ---- small icons (centred at x, y, radius r) ------------------------------------------ */
    kit.bomb = function (c, x, y, r, time) {
        c.beginPath(); c.arc(x, y + r * 0.1, r * 0.78, 0, TAU); c.fillStyle = '#1f2937'; c.fill();
        c.lineWidth = Math.max(1.4, r * 0.16); c.strokeStyle = '#f8fafc'; c.stroke();
        c.beginPath(); c.moveTo(x + r * 0.35, y - r * 0.6); c.quadraticCurveTo(x + r * 0.7, y - r * 1.0, x + r * 0.95, y - r * 0.8);
        c.lineWidth = Math.max(1.2, r * 0.14); c.strokeStyle = '#d4a373'; c.stroke();
        star(c, x + r * 0.95, y - r * 0.85, r * (0.4 + 0.15 * Math.sin((time || 0) * 22)), '#fde047');
        c.beginPath(); c.arc(x - r * 0.28, y - r * 0.12, r * 0.18, 0, TAU); c.fillStyle = 'rgba(255,255,255,0.4)'; c.fill();
    };
    kit.flag = function (c, x, y, r) {
        const n = 4, s = r * 2 / n;
        c.save(); c.translate(x - r, y - r);
        for (let i = 0; i < n; i++) for (let j = 0; j < n; j++) { c.fillStyle = (i + j) % 2 ? '#0b1020' : '#f8fafc'; c.fillRect(i * s, j * s, s, s); }
        c.lineWidth = 1.2; c.strokeStyle = INK; c.strokeRect(0, 0, r * 2, r * 2);
        c.restore();
    };
    kit.warn = function (c, x, y, r, col) {
        poly(c, [x, y - r, x + r * 1.05, y + r * 0.85, x - r * 1.05, y + r * 0.85]);
        c.fillStyle = col; c.fill(); c.lineWidth = Math.max(1.5, r * 0.18); c.strokeStyle = INK; c.lineJoin = 'round'; c.stroke();
        c.fillStyle = INK; c.fillRect(x - r * 0.1, y - r * 0.4, r * 0.2, r * 0.7);
        c.beginPath(); c.arc(x, y + r * 0.55, r * 0.12, 0, TAU); c.fill();
    };
    /* Arrow pointing along `angle` (0 = up on screen). */
    kit.arrow = function (c, x, y, r, angle, col) {
        c.save(); c.translate(x, y); c.rotate(angle);
        poly(c, [0, -r, r * 0.8, r * 0.7, 0, r * 0.25, -r * 0.8, r * 0.7]);
        c.fillStyle = col; c.fill(); c.lineWidth = Math.max(1.5, r * 0.16); c.strokeStyle = INK; c.lineJoin = 'round'; c.stroke();
        c.restore();
    };

    /* ---- kinetic call-outs ------------------------------------------------------------------- */
    const shineCache = new Map();
    function shine(c, colour, size) {
        const key = colour + '|' + size;
        let g = shineCache.get(key);
        if (!g) {
            g = c.createLinearGradient(0, -size * 0.8, 0, size * 0.12);
            g.addColorStop(0, '#ffffff'); g.addColorStop(0.55, colour); g.addColorStop(1, colour);
            if (shineCache.size > 64) shineCache.clear();
            shineCache.set(key, g);
        }
        return g;
    }

    /* Up to `max` stacked items, newest in front. An item is
       { text, sub, colour, size, life }. */
    kit.createCallouts = function (max) {
        const items = [];
        let seq = 0, lastDt = 0.016;
        return {
            push(spec) {
                const item = {
                    text: spec.text, sub: spec.sub || '', colour: spec.colour || '#ffffff', size: spec.size || 46,
                    life: spec.life || 1.7, age: 0, y: 0, tilt: ((seq++ % 3) - 1) * 0.05 + (spec.tilt || 0), glow: spec.glow || spec.colour
                };
                for (const e of items) if (e.text === item.text && e.age < 0.35) return;
                items.push(item);
                while (items.length > max) items.shift();
            },
            update(dt) {
                lastDt = dt;
                for (let i = items.length - 1; i >= 0; i--) {
                    items[i].age += dt;
                    if (items[i].age >= items[i].life) items.splice(i, 1);
                }
            },
            clear() { items.length = 0; },
            get count() { return items.length; },
            draw(c, cx, cy, reduced) {
                let y = cy, prev = null;
                for (let i = items.length - 1; i >= 0; i--) {
                    const it = items[i];
                    if (prev) y -= prev.size * 0.8 + (it.sub ? it.size * 0.66 : 0) + 12;
                    prev = it;
                    it.y = reduced || it.age < 0.02 ? y : it.y + (y - it.y) * (1 - Math.exp(-lastDt * 16));
                    const fadeIn = Math.min(1, it.age / (reduced ? 0.2 : 0.05));
                    const fadeOut = Math.min(1, (it.life - it.age) / 0.4);
                    const spring = reduced ? 1 : 1 + 0.4 * Math.exp(-it.age * 9) * Math.cos(it.age * 20);
                    const drift = reduced ? 0 : -18 * (it.age / it.life);
                    c.save();
                    c.globalAlpha = Math.max(0, Math.min(fadeIn, fadeOut)) * (i === items.length - 1 ? 1 : 0.82);
                    c.translate(cx, it.y + drift);
                    if (!reduced) c.rotate(it.tilt * (it.age < 0.5 ? 1 : 0.6));
                    c.scale(spring, spring);
                    c.font = 'italic 900 ' + it.size + 'px ' + FAM;
                    c.textAlign = 'center'; c.textBaseline = 'alphabetic';
                    c.lineJoin = 'round';
                    c.lineWidth = Math.max(5, it.size * 0.26); c.strokeStyle = 'rgba(4,8,18,0.88)';
                    c.strokeText(it.text, 0, 0);
                    c.lineWidth = Math.max(2.5, it.size * 0.11); c.strokeStyle = it.glow;
                    c.globalAlpha *= 0.9; c.strokeText(it.text, 0, 0); c.globalAlpha /= 0.9;
                    c.fillStyle = shine(c, it.colour, it.size);
                    c.fillText(it.text, 0, 0);
                    if (it.sub) {
                        kit.text(c, it.sub, 0, it.size * 0.62, Math.round(it.size * 0.46), '#ffffff', { align: 'center', weight: 800 });
                    }
                    c.restore();
                }
            }
        };
    };
})();
