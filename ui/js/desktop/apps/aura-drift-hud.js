/* Aura Drift - in-race HUD on a 2D overlay canvas (premium arcade layout).
   Drawing primitives, weapon glyphs, portraits, minimap and call-outs live in
   aura-drift-hud-fx.js. This file owns layout, state and the reaction to events.
   Everything is laid out in a virtual space scaled by min(w/1280, h/720); static layers
   are cached and a frame allocates no gradients. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const TICKS = 40, A0 = Math.PI * 0.78, A1 = Math.PI * 2.22;
    const TCOL = [];
    for (let i = 0; i < TICKS; i++) {
        const f = i / (TICKS - 1);
        TCOL.push(f < 0.6 ? '#3ee6ff' : f < 0.78 ? '#c4f7ff' : f < 0.9 ? '#ffc233' : '#ff4d4d');
    }
    const TURBO = ['#38bdf8', '#fb923c', '#f472b6'];
    const TURBO_AT = [0, 1.0, 2.2], TURBO_LEN = [1.0, 1.2, 1.4];
    const TRICK_COL = {
        barrel_roll: '#a78bfa', double_barrel: '#c084fc', front_flip: '#5eead4', back_flip: '#5eead4', flat_spin: '#f9a8d4',
        spin_720: '#f472b6', corkscrew: '#fde047', cow_vault: '#f8fafc', air_time: '#7dd3fc',
        drift_blue: '#38bdf8', drift_orange: '#fb923c', drift_pink: '#f472b6'
    };
    const LAND_COL = { perfect: '#3ee6ff', good: '#8cff6b', wobble: '#ffc233', crash: '#ff5d5d' };
    const WARN_RED = '#ff4d4d', WARN_AMBER = '#ffc233';

    function noopHud(canvas) {
        const c = canvas.getContext('2d');
        return {
            resize(w, h) { canvas.width = Math.round(w); canvas.height = Math.round(h); }, setInset() {},
            event() {}, radio() {}, showMessage() {}, popup() {}, clear() { c.clearRect(0, 0, canvas.width, canvas.height); },
            render() { c.clearRect(0, 0, canvas.width, canvas.height); }, dispose() {}
        };
    }

    AD.createHud = function (canvas, labels, t) {
        const kit = AD.hudKit;
        if (!kit || typeof AD.drawMinimap !== 'function') return noopHud(canvas);
        const tr = kit.makeTr(t);
        const c = canvas.getContext('2d');
        const callouts = kit.createCallouts(3);
        const up = s => String(s).toUpperCase();

        let cw = 960, ch = 540, dpr = 1, k = 1, W = 1280, H = 720, Hb = 720, inset = 0, M = 24, compact = false;
        let G = null, layer = null;
        let time = 0, reduced = false, mqAt = -9;
        let comms = null;
        const queue = [];
        let posPulse = 0, posDir = 0, slotPulse = 0, hitFlash = 0, goT = 0, clockBig = null, turboFlash = null, blast = 0;
        let wrongWay = 0, curRace = null, commsGrad = null;
        const splits = [null, null, null], prevBest = [Infinity, Infinity, Infinity];
        const animalWarn = [];
        const stripGrad = new Map();

        function nameOf(race, slot) {
            const car = race.cars[slot];
            if (!car) return '';
            return car.driver ? car.driver.name : car.v.car.name;
        }
        function initials(car) {
            if (!car.driver) return up(car.v.car.name.slice(0, 3));
            return car.driver.name.split(/\s+/).map(w => w.charAt(0)).join('').slice(0, 3).toUpperCase();
        }
        function weaponName(id) { return tr('weapon.' + id + '.name', null, kit.humanise(id)); }
        function clockText(sec) {
            sec = Math.max(0, Math.ceil(sec - 1e-6));
            return Math.floor(sec / 60) + ':' + String(sec % 60).padStart(2, '0');
        }

        /* ---- layout, cached gradients and the static layer ------------------------------ */
        function layout() {
            compact = cw < 660;
            k = compact ? clamp(cw / 780, 0.5, 1) : clamp(Math.min(cw / 1280, ch / 720), 0.55, 1.6);
            W = cw / k; H = ch / k;
            /* Hb is the bottom anchor: the touch pads own the lowest `inset` px, so the gauges sit above them. */
            Hb = H - inset / k;
            M = compact ? 14 : 24;
            const R = compact ? 62 : 92;
            const g = G = { R: R };
            g.ACx = M + R + 14;
            g.ACy = Hb - M - R * 0.72 - 20;
            g.BX = g.ACx + R + (compact ? 6 : 18);
            g.BW = compact ? 118 : 200;
            g.BH = compact ? 11 : 15;
            g.BS = compact ? 7 : 11;
            g.gap = compact ? 30 : 38;
            const bottom = g.ACy + R * 0.72;
            g.turboY = bottom - g.BH;
            g.nitroY = g.turboY - g.gap;
            g.dmgY = g.nitroY - g.gap + 6;
            g.SW = compact ? 92 : 124; g.SH = compact ? 78 : 104;
            g.WX = W - M - g.SW / 2 - 12; g.WY = Hb - M - g.SH / 2 - 12;
            g.MS = compact ? 108 : 160;
            g.MX = W - M - g.MS - 4; g.MY = M + 4;
            g.RW = compact ? 150 : 196; g.RH = compact ? 22 : 29;
            g.TY = M + (compact ? 76 : 112);
            g.cw = compact ? 290 : 400; g.ch = compact ? 56 : 76;
            g.WYs = M + g.ch + (compact ? 14 : 20);
            g.nitro = c.createLinearGradient(g.BX, 0, g.BX + g.BW, 0);
            g.nitro.addColorStop(0, '#0891b2'); g.nitro.addColorStop(0.6, '#3ee6ff'); g.nitro.addColorStop(1, '#ffffff');
            g.tower = c.createLinearGradient(M, 0, M + g.RW, 0);
            g.tower.addColorStop(0, 'rgba(6,10,24,0.8)'); g.tower.addColorStop(0.7, 'rgba(6,10,24,0.5)'); g.tower.addColorStop(1, 'rgba(6,10,24,0)');
            g.towerMe = c.createLinearGradient(M, 0, M + g.RW, 0);
            g.towerMe.addColorStop(0, 'rgba(70,140,255,0.75)'); g.towerMe.addColorStop(0.7, 'rgba(70,140,255,0.4)'); g.towerMe.addColorStop(1, 'rgba(70,140,255,0)');
            g.hit = c.createRadialGradient(W / 2, H / 2, H * 0.3, W / 2, H / 2, H * 0.95);
            g.hit.addColorStop(0, 'rgba(255,40,40,0)'); g.hit.addColorStop(1, 'rgba(255,40,40,0.6)');
            stripGrad.clear();
            commsGrad = null;
            /* Static layer: soft corner scrims, gauge plates and slot frames. */
            if (!layer) layer = document.createElement('canvas');
            layer.width = Math.max(1, Math.round(cw * dpr));
            layer.height = Math.max(1, Math.round(ch * dpr));
            const s = layer.getContext('2d');
            s.setTransform(dpr * k, 0, 0, dpr * k, 0, 0);
            const scrim = (x, y, r, a) => {
                const gr = s.createRadialGradient(x, y, 0, x, y, r);
                gr.addColorStop(0, 'rgba(3,6,16,' + a + ')'); gr.addColorStop(0.55, 'rgba(3,6,16,' + a * 0.45 + ')'); gr.addColorStop(1, 'rgba(3,6,16,0)');
                s.fillStyle = gr; s.fillRect(x - r, y - r, r * 2, r * 2);
            };
            scrim(g.ACx + 70, g.ACy + 40, compact ? 280 : 400, 0.55);
            scrim(g.WX, Hb, compact ? 220 : 320, 0.5);
            scrim(0, 0, compact ? 220 : 320, 0.36);
            scrim(W, 0, compact ? 170 : 240, 0.3);
            s.lineCap = 'round';
            s.beginPath(); s.arc(g.ACx, g.ACy, R - 8, A0, A1);
            s.lineWidth = R * 0.34; s.strokeStyle = 'rgba(5,9,20,0.55)'; s.stroke();
            for (let i = 0; i < TICKS; i++) {
                const a = A0 + (A1 - A0) * i / (TICKS - 1), long = i % 6 === 0, r2 = R - (long ? 20 : 14);
                s.beginPath(); s.moveTo(g.ACx + Math.cos(a) * R, g.ACy + Math.sin(a) * R); s.lineTo(g.ACx + Math.cos(a) * r2, g.ACy + Math.sin(a) * r2);
                s.lineWidth = long ? 4.5 : 3.2; s.strokeStyle = 'rgba(255,255,255,0.15)'; s.stroke();
            }
            const plate = (x, y, w, h, sk) => {
                kit.para(s, x - 3, y - 3, w + 6, h + 6, sk); s.fillStyle = 'rgba(4,8,18,0.78)'; s.fill();
                s.lineWidth = 1.5; s.strokeStyle = 'rgba(255,255,255,0.2)'; s.stroke();
            };
            plate(g.BX, g.nitroY, g.BW, g.BH, g.BS);
            plate(g.BX, g.turboY, g.BW, g.BH, g.BS);
            plate(g.BX, g.dmgY, g.BW * 0.55, g.BH * 0.55, g.BS * 0.6);
        }

        function resize(w, h, ratio) {
            cw = Math.max(320, Math.round(w));
            ch = Math.max(240, Math.round(h));
            dpr = clamp(ratio || 1, 1, 2);
            canvas.width = Math.round(cw * dpr);
            canvas.height = Math.round(ch * dpr);
            canvas.style.width = cw + 'px';
            canvas.style.height = ch + 'px';
            layout();
        }

        /* ---- widgets -------------------------------------------------------------------------- */
        function bar(x, y, w, h, sk, frac, fill) {
            if (frac <= 0.001) return;
            kit.para(c, x, y, w * clamp(frac, 0, 1), h, sk);
            c.fillStyle = fill; c.fill();
        }

        function drawSpeed(race, car) {
            const v = car.v, g = G, R = g.R;
            const rpm = clamp(v.rpm / 7200, 0, 1);
            const lit = Math.round(rpm * TICKS);
            const limiter = rpm > 0.9;
            const flick = limiter && !reduced && Math.floor(time * 16) % 2 === 0;
            if (limiter) {
                c.beginPath(); c.arc(g.ACx, g.ACy, R - 8, A0 + (A1 - A0) * 0.86, A1);
                c.lineWidth = R * 0.22; c.lineCap = 'butt'; c.strokeStyle = 'rgba(255,70,70,' + (flick ? 0.32 : 0.12) + ')'; c.stroke();
            }
            c.lineCap = 'round';
            for (let i = 0; i < lit; i++) {
                const a = A0 + (A1 - A0) * i / (TICKS - 1), long = i % 6 === 0, r2 = R - (long ? 20 : 14);
                c.beginPath(); c.moveTo(g.ACx + Math.cos(a) * R, g.ACy + Math.sin(a) * R); c.lineTo(g.ACx + Math.cos(a) * r2, g.ACy + Math.sin(a) * r2);
                c.lineWidth = long ? 4.5 : 3.2; c.strokeStyle = flick && i > TICKS * 0.8 ? '#ffffff' : TCOL[i]; c.stroke();
            }
            const boosting = v.burst > 0 || v.boost || v.overdrive > 0;
            if (boosting || v.slipstream > 0.35) {
                const gr = c.createRadialGradient(g.ACx, g.ACy + R * 0.1, R * 0.05, g.ACx, g.ACy + R * 0.1, R * 0.72);
                gr.addColorStop(0, v.overdrive > 0 ? 'rgba(255,138,43,0.28)' : boosting ? 'rgba(72,229,255,0.24)' : 'rgba(255,194,51,0.18)');
                gr.addColorStop(1, 'rgba(0,0,0,0)');
                c.fillStyle = gr; c.beginPath(); c.arc(g.ACx, g.ACy + R * 0.1, R * 0.72, 0, TAU); c.fill();
            }
            const speed = Math.max(0, Math.round(AD.vehicleSpeedKmh(v)));
            kit.text(c, String(speed), g.ACx, g.ACy + R * 0.36, R * 0.86, boosting ? '#ffd66b' : '#ffffff', { align: 'center', weight: 900 });
            kit.text(c, 'km/h', g.ACx, g.ACy + R * 0.36 + R * 0.28, R * 0.19, '#a9d6ff', { align: 'center', weight: 700, spacing: 3, sw: 3 });
            const gear = v.gear === 0 ? 'R' : String(v.gear);
            kit.para(c, g.ACx - R * 0.17, g.ACy - R * 0.56, R * 0.34, R * 0.34, 5);
            c.fillStyle = flick ? 'rgba(255,77,77,0.42)' : 'rgba(255,194,51,0.18)'; c.fill();
            c.lineWidth = 1.8; c.strokeStyle = flick ? '#ff6b6b' : '#ffc233'; c.stroke();
            kit.text(c, gear, g.ACx + 3, g.ACy - R * 0.29, R * 0.3, flick ? '#ffffff' : '#ffc233', { align: 'center', weight: 900, sw: 0 });
            if (v.slipstream > 0.25) kit.text(c, '»»', g.ACx + R * 0.62, g.ACy + R * 0.64, R * 0.26, '#ffc233', { align: 'center', weight: 900, sw: 3 });

            /* Bars: nitro, drift turbo with blue/orange/pink segments, damage pips. */
            const small = compact ? 9 : 11;
            kit.text(c, up(labels.nitro) + '  ' + Math.round(v.nitro * 100), g.BX + g.BS, g.nitroY - 5, small, '#bfeaff', { spacing: 2, weight: 800, sw: 2.5 });
            bar(g.BX, g.nitroY, g.BW, g.BH, g.BS, v.nitro, g.nitro);
            if (v.nitro > 0.98) { kit.para(c, g.BX, g.nitroY, g.BW, g.BH, g.BS); c.lineWidth = 2; c.strokeStyle = 'rgba(255,255,255,0.9)'; c.stroke(); }
            const level = v.driftLevel | 0;
            const tlab = level > 0 ? up(tr('hud.drift' + level)) : up(tr('controls.drift_turbo'));
            kit.text(c, tlab, g.BX + g.BS, g.turboY - 5, small, level > 0 ? TURBO[level - 1] : '#bfeaff', { spacing: 2, weight: 800, sw: 2.5 });
            const total = 3.6, gap = 4;
            let sx = g.BX;
            for (let i = 0; i < 3; i++) {
                const w = (g.BW - gap * 2) * TURBO_LEN[i] / total;
                const f = clamp((v.driftCharge - TURBO_AT[i]) / TURBO_LEN[i], 0, 1);
                kit.para(c, sx, g.turboY, w, g.BH, g.BS); c.fillStyle = 'rgba(255,255,255,0.07)'; c.fill();
                if (f > 0) { kit.para(c, sx, g.turboY, w * f, g.BH, g.BS); c.fillStyle = TURBO[i]; c.fill(); }
                if (level > i) { kit.para(c, sx, g.turboY, w, g.BH, g.BS); c.lineWidth = 2; c.strokeStyle = '#ffffff'; c.stroke(); }
                sx += w + gap;
            }
            if (turboFlash) {
                const f = turboFlash.t / 0.5;
                c.globalAlpha = f * 0.7;
                kit.para(c, g.BX - 4, g.turboY - 4, g.BW + 8, g.BH + 8, g.BS); c.fillStyle = TURBO[turboFlash.level - 1] || '#fff'; c.fill();
                c.globalAlpha = 1;
            }
            /* Damage as ten pips with a number: green, amber, red. */
            const hp = clamp(1 - v.damage / 100, 0, 1);
            const hcol = hp > 0.6 ? '#8cff6b' : hp > 0.3 ? '#ffc233' : '#ff4d4d';
            const pw = g.BW * 0.55 / 10;
            for (let i = 0; i < 10; i++) {
                if (hp * 10 > i + 0.2 && !(hp < 0.3 && !reduced && Math.floor(time * 6) % 2)) {
                    kit.para(c, g.BX + i * pw, g.dmgY, pw - 2, g.BH * 0.55, g.BS * 0.6); c.fillStyle = hcol; c.fill();
                }
            }
            kit.text(c, String(Math.round(hp * 100)), g.BX + g.BW * 0.55 + 14, g.dmgY + g.BH * 0.55 + 1, small + 1, hcol, { mono: true, weight: 800, sw: 2.5 });
            const ix = g.BX - 9, iy = g.dmgY + g.BH * 0.27;
            c.fillStyle = hcol; c.fillRect(ix - 5, iy - 1.5, 10, 3); c.fillRect(ix - 1.5, iy - 5, 3, 10);
        }

        function drawWeapon(race, car) {
            if (!race.weapons) return;
            const g = G, def = car.weapon ? AD.WEAPONS[car.weapon] : null;
            const x0 = g.WX - g.SW / 2, y0 = g.WY - g.SH / 2;
            const rolling = !!def && car.weaponLock > 0;
            const ready = def && !rolling && car.cooldown <= 0.001 && (car.ammo > 0 || car.fuel > 0);
            const col = def ? def.colour : '#6b7a99';
            const pulse = slotPulse > 0 && !reduced ? 1 + slotPulse * 0.1 : 1;
            c.save();
            c.translate(g.WX, g.WY); c.scale(pulse, pulse); c.translate(-g.WX, -g.WY);
            kit.para(c, x0, y0, g.SW, g.SH, 14);
            c.fillStyle = 'rgba(4,8,20,0.7)'; c.fill();
            if (ready) { c.lineWidth = 7; c.strokeStyle = col; c.globalAlpha = 0.28; c.stroke(); c.globalAlpha = 1; }
            c.lineWidth = ready ? 3 : 2; c.strokeStyle = def ? col : 'rgba(150,170,210,0.5)';
            if (!def) c.setLineDash([6, 5]);
            c.stroke(); c.setLineDash([]);
            const gs = g.SH * 0.66;
            const cx = g.WX + 7, cy = g.WY;
            if (rolling) {
                const ids = AD.WEAPON_IDS, n = ids.length, fi = Math.max(0, ids.indexOf(car.weapon));
                const p = clamp(1 - car.weaponLock / 1.1, 0, 1);
                const q = fi - Math.pow(1 - p, 2.2) * 16;
                const sp = gs * 0.95;
                c.save();
                c.beginPath(); c.rect(x0 + 8, y0 + 3, g.SW - 4, g.SH - 6); c.clip();
                const base = Math.floor(q);
                for (let j = base - 1; j <= base + 2; j++) {
                    const off = (j - q) * sp;
                    c.globalAlpha = clamp(1 - Math.abs(off) / (sp * 0.95), 0, 1);
                    AD.drawWeaponGlyph(c, ids[((j % n) + n) % n], cx, cy + off, gs, dpr * k);
                }
                c.restore();
                c.globalAlpha = 1;
            } else if (def) {
                const frac = def.cooldown > 0 ? clamp(1 - car.cooldown / def.cooldown, 0, 1) : 1;
                const rr = g.SH * 0.46;
                c.beginPath(); c.arc(cx, cy, rr, 0, kit.TAU); c.lineWidth = 3; c.strokeStyle = 'rgba(255,255,255,0.14)'; c.stroke();
                c.beginPath(); c.arc(cx, cy, rr, -Math.PI / 2, -Math.PI / 2 + kit.TAU * frac);
                c.lineWidth = 3.5; c.strokeStyle = frac >= 1 ? col : 'rgba(255,255,255,0.8)'; c.stroke();
                c.globalAlpha = frac >= 1 ? 1 : 0.55;
                AD.drawWeaponGlyph(c, def.id, cx, cy, gs, dpr * k);
                c.globalAlpha = 1;
            } else {
                kit.text(c, '--', cx, cy + 10, 30, 'rgba(150,170,210,0.7)', { align: 'center', weight: 800, sw: 0 });
            }
            c.restore();
            if (def && !rolling) {
                kit.text(c, up(weaponName(def.id)), x0 + g.SW + 10, y0 - 9, compact ? 14 : 19, col, { align: 'right', spacing: 1 });
                const n = Math.min(8, def.ammo);
                if (def.ammo > 1) {
                    for (let i = 0; i < n; i++) {
                        kit.para(c, x0 + 14 + i * 9, y0 + g.SH - 11, 6, 7, 3);
                        c.fillStyle = i < car.ammo ? col : 'rgba(255,255,255,0.18)'; c.fill();
                    }
                }
                if (car.fuel > 0) {
                    kit.para(c, x0 + 14, y0 + g.SH - 11, (g.SW - 30) * clamp(car.fuel / 2.5, 0, 1), 6, 3);
                    c.fillStyle = col; c.fill();
                }
            }
        }

        /* Style meter above the weapon slot (everything except drift attack). */
        function drawStyle(race, car) {
            const st = car.style;
            if (!st) return;
            const pending = st.raw * st.multiplier, total = st.banked + pending;
            const active = st.drifting || st.timer > 0;
            if (total < 1 && !active) return;
            const g = G, x = g.WX + g.SW / 2 + 8, y = race.weapons ? g.WY - g.SH / 2 - (compact ? 34 : 50) : Hb - M - (compact ? 16 : 22);
            kit.text(c, kit.number(total), x, y, compact ? 26 : 38, active ? '#ffffff' : '#c5d6ee', { align: 'right', weight: 900 });
            if (st.multiplier > 1.01) {
                const m = 'x' + st.multiplier.toFixed(1);
                kit.text(c, m, x - (compact ? 90 : 128), y - 2, compact ? 17 : 22, st.multiplier > 2 ? '#ffc233' : '#3ee6ff', { align: 'right', weight: 900 });
            }
            const link = clamp(st.timer / (AD.STYLE ? AD.STYLE.comboWindow : 1.5), 0, 1);
            kit.text(c, up(labels.style), x, y - (compact ? 24 : 34), 11, '#9fb9d8', { align: 'right', spacing: 2, weight: 800, sw: 2.5 });
            if (active) {
                const w = compact ? 120 : 170;
                kit.para(c, x - w, y + 6, w * (st.drifting ? 1 : link), 5, 5);
                c.fillStyle = st.multiplier > 2 ? '#ffc233' : '#3ee6ff'; c.fill();
            }
        }

        function drawTopLeft(race, car) {
            const x = M, y = M, mode = race.mode;
            const solo = race.cars.length < 2;
            const big = compact ? 44 : 66;
            let tx = x;
            if (!solo && mode !== 'hot_potato' && mode !== 'elimination') {
                const pos = AD.playerPosition(race);
                const pulse = posPulse > 0 && !reduced ? 1 + posPulse * 0.35 : 1;
                kit.text(c, up(labels.position), x + 2, y + 8, 11, '#9fb9d8', { spacing: 2, weight: 800, sw: 2.5 });
                c.save(); c.translate(x, y + big * 1.02 + 4); c.scale(pulse, pulse);
                kit.text(c, String(pos), 0, 0, big, posPulse > 0 ? (posDir > 0 ? '#8cff6b' : '#ff6b6b') : '#ffffff', { weight: 900 });
                const wpos = (() => { c.font = 'italic 900 ' + big + 'px ' + kit.FAM; return c.measureText(String(pos)).width; })();
                kit.text(c, '/' + race.cars.length, wpos + 4, 0, big * 0.4, '#a9c4e4', { weight: 800 });
                c.restore();
                if (posPulse > 0) kit.text(c, posDir > 0 ? '▲' : '▼', x + big * 1.5, y + big * 0.5, big * 0.3, posDir > 0 ? '#8cff6b' : '#ff6b6b', { weight: 900, sw: 3 });
                tx = x + (compact ? 88 : 128);
            }
            const sizeA = compact ? 20 : 27, sizeB = compact ? 18 : 24;
            const timeCol = '#ffffff';
            if (mode === 'drift_attack') {
                const left = race.clock === null ? 0 : race.clock;
                const hot = left <= 10;
                kit.text(c, up(tr('hud.clock')), x + 2, y + 8, 11, '#9fb9d8', { spacing: 2, weight: 800, sw: 2.5 });
                kit.text(c, clockText(left), x, y + (compact ? 48 : 68), compact ? 46 : 68, hot && Math.floor(time * 4) % 2 ? '#ff6b6b' : '#ffffff', { weight: 900, mono: false });
                const st = car.style, score = st ? st.banked + st.raw * st.multiplier : 0;
                const sy = y + (compact ? 90 : 122);
                kit.text(c, up(tr('hud.score')), x + 2, sy - (compact ? 24 : 34), 11, '#9fb9d8', { spacing: 2, weight: 800, sw: 2.5 });
                kit.text(c, kit.number(score), x, sy, compact ? 28 : 40, '#ffd66b', { weight: 900 });
                const medals = race.rules.medals;
                if (medals) {
                    const top = medals.gold * 1.1, w = compact ? 170 : 230, my = sy + 12;
                    kit.para(c, x, my, w, 7, 6); c.fillStyle = 'rgba(255,255,255,0.14)'; c.fill();
                    kit.para(c, x, my, w * clamp(score / top, 0, 1), 7, 6); c.fillStyle = '#ffd66b'; c.fill();
                    let medal = 'none';
                    for (const key of ['bronze', 'silver', 'gold']) {
                        const mx = x + w * medals[key] / top;
                        c.fillStyle = score >= medals[key] ? '#ffffff' : 'rgba(255,255,255,0.5)'; c.fillRect(mx - 1, my - 4, 3, 15);
                        if (score >= medals[key]) medal = key;
                    }
                    kit.text(c, up(tr('medal.' + medal)), x + w + 12, my + 8, 13, medal === 'none' ? '#9fb9d8' : '#ffd66b', { weight: 800, sw: 3 });
                }
                return;
            }
            if (mode === 'elimination') {
                let alive = 0;
                for (const e of race.cars) if (!e.v.out) alive++;
                kit.text(c, up(tr('hud.next_out')), x + 2, y + 8, 11, '#9fb9d8', { spacing: 2, weight: 800, sw: 2.5 });
                const left = race.clock === null ? 0 : race.clock;
                kit.text(c, clockText(left), x, y + (compact ? 46 : 66), compact ? 44 : 64, left <= 10 && Math.floor(time * 4) % 2 ? '#ff6b6b' : '#ffffff', { weight: 900 });
                kit.text(c, alive + '/' + race.cars.length, x + (compact ? 120 : 170), y + (compact ? 44 : 62), compact ? 26 : 36, '#a9c4e4', { weight: 900 });
                return;
            }
            if (mode === 'hot_potato') {
                const b = race.bomb;
                kit.bomb(c, x + 18, y + 26, 16, time);
                if (b.holder >= 0) {
                    kit.text(c, tr('hud.bomb', { name: nameOf(race, b.holder) }), x + 44, y + 22, compact ? 15 : 19, '#ffffff', { weight: 800 });
                    const hot = b.fuse < 4;
                    kit.text(c, Math.max(0, b.fuse).toFixed(1), x + 2, y + (compact ? 70 : 82), compact ? 42 : 60, hot && Math.floor(time * 6) % 2 ? '#ff6b6b' : '#ffc233', { weight: 900 });
                    kit.para(c, x, y + (compact ? 78 : 92), 190, 6, 6); c.fillStyle = 'rgba(255,255,255,0.15)'; c.fill();
                    kit.para(c, x, y + (compact ? 78 : 92), 190 * clamp(b.fuse / Math.max(10, 16 - b.round), 0, 1), 6, 6); c.fillStyle = hot ? '#ff4d4d' : '#ffc233'; c.fill();
                } else {
                    kit.text(c, String(Math.max(0, Math.ceil(b.delay))), x + 2, y + (compact ? 70 : 82), compact ? 42 : 60, '#9fb9d8', { weight: 900 });
                }
                return;
            }
            kit.text(c, tr('hud.lap_of', { lap: Math.min(car.lap + 1, race.laps), total: race.laps }), tx, y + (compact ? 22 : 30), sizeA, '#ffffff', { weight: 900 });
            kit.text(c, AD.formatTime(race.time), tx, y + (compact ? 44 : 60), sizeB, timeCol, { mono: true, weight: 800 });
            if (!compact) {
                kit.text(c, labels.bestLap + ' ' + (isFinite(car.bestLap) ? AD.formatTime(car.bestLap) : '--:--.---'), tx, y + 80, 12, '#9fb9d8', { mono: true, weight: 700, sw: 2.5 });
                /* Sector splits: colour plus a text mark. */
                for (let i = 0; i < 3; i++) {
                    const sx = x + i * 70, sy = y + 90, s = splits[i];
                    const cur = car.sector === i;
                    kit.para(c, sx, sy, 64, 17, 6);
                    c.fillStyle = s ? 'rgba(6,10,24,0.78)' : 'rgba(6,10,24,0.55)'; c.fill();
                    c.lineWidth = cur ? 2 : 1.2; c.strokeStyle = s ? s.colour : (cur ? '#ffffff' : 'rgba(255,255,255,0.25)'); c.stroke();
                    kit.text(c, 'S' + (i + 1) + (s ? ' ' + s.text : ''), sx + 8, sy + 13, 11, s ? s.colour : '#9fb9d8', { weight: 800, sw: 0, mono: true });
                }
            }
        }

        function drawTower(race, car) {
            const order = race.standings && race.standings.length ? race.standings : race.cars;
            if (order.length < 2 || race.mode === 'time_trial') return;
            const g = G, mi = order.indexOf(car);
            const maxRows = compact ? 4 : 6;
            let from = 0;
            if (order.length > maxRows) from = clamp(mi - 1, 0, order.length - maxRows);
            const rows = Math.min(maxRows, order.length);
            for (let r = 0; r < rows; r++) {
                const e = order[from + r], me = e === car, y = g.TY + r * g.RH, h = g.RH - 4;
                const out = e.v.out;
                c.globalAlpha = out ? 0.5 : 1;
                kit.para(c, M, y, g.RW, h, 8); c.fillStyle = me ? g.towerMe : g.tower; c.fill();
                if (race.nextOut === e.index && !out) {
                    kit.para(c, M, y, g.RW, h, 8); c.lineWidth = 2; c.strokeStyle = reduced || Math.floor(time * 5) % 2 ? WARN_RED : '#ffffff'; c.stroke();
                }
                kit.para(c, M + 30, y + 3, 5, h - 6, 3); c.fillStyle = e.v.car.colour; c.fill();
                kit.text(c, String(from + r + 1), M + 14, y + h - 5, compact ? 12 : 15, me ? '#ffffff' : '#c5d6ee', { align: 'center', mono: true, weight: 900, sw: 2.5 });
                kit.text(c, initials(e), M + 42, y + h - 5, compact ? 13 : 16, me ? '#ffffff' : '#dbe7f7', { weight: 800, sw: 2.5 });
                if (out) { c.fillStyle = '#ff6b6b'; c.fillRect(M + 40, y + h * 0.55, 34, 2); }
                if (!me && !out) {
                    /* The grid sits across the line: wrap the gap into half a lap either way. */
                    const L = race.track.length, raw = e.progress - car.progress;
                    const diff = ((raw + L / 2) % L + L) % L - L / 2;
                    const d = Math.round(Math.abs(diff));
                    kit.text(c, (diff > 0 ? '▲' : '▼') + d + 'm', M + g.RW - 4, y + h - 5, compact ? 11 : 13, '#c5d6ee', { align: 'right', mono: true, weight: 700, sw: 2.5 });
                }
                if (e.finished) kit.flag(c, M + g.RW - 12, y + h / 2, 6);
                else if (race.bomb && race.bomb.holder === e.index) kit.bomb(c, M + g.RW - 14, y + h / 2 + 1, 7, time);
                c.globalAlpha = 1;
            }
        }

        function drawCountdown(race) {
            const cx = W / 2, cy = compact ? 78 : 96;
            if (race.state === 'countdown') {
                const lit = clamp(Math.floor((3.2 - race.countdown) / 0.64) + 1, 0, 5);
                const sp = compact ? 46 : 70, rr = compact ? 15 : 24;
                for (let i = 0; i < 5; i++) {
                    const x = cx + (i - 2) * sp, on = i < lit;
                    c.beginPath(); c.arc(x, cy, rr + 5, 0, kit.TAU); c.fillStyle = 'rgba(4,8,18,0.82)'; c.fill();
                    c.beginPath(); c.arc(x, cy, rr, 0, kit.TAU); c.fillStyle = on ? '#ff2d3d' : '#1a2132'; c.fill();
                    if (on) {
                        c.beginPath(); c.arc(x, cy, rr + 9, 0, kit.TAU); c.fillStyle = 'rgba(255,45,61,0.25)'; c.fill();
                        c.beginPath(); c.arc(x - rr * 0.3, cy - rr * 0.35, rr * 0.35, 0, kit.TAU); c.fillStyle = 'rgba(255,255,255,0.65)'; c.fill();
                    }
                    c.lineWidth = 2; c.strokeStyle = 'rgba(255,255,255,0.35)'; c.beginPath(); c.arc(x, cy, rr, 0, kit.TAU); c.stroke();
                }
                const n = Math.ceil(race.countdown);
                if (n >= 1 && n <= 3) {
                    const u = n - race.countdown;
                    const s = reduced ? 1 : 1 + 0.4 * Math.exp(-u * 7);
                    c.save(); c.translate(cx, H * 0.34); c.scale(s, s);
                    kit.text(c, String(n), 0, 0, compact ? 90 : 150, '#ffffff', { align: 'center', weight: 900, stroke: 'rgba(255,45,61,0.9)', sw: 14 });
                    c.restore();
                }
            } else if (goT > 0) {
                const sp = compact ? 46 : 70, rr = compact ? 15 : 24;
                c.globalAlpha = clamp(goT, 0, 1);
                for (let i = 0; i < 5; i++) {
                    const x = cx + (i - 2) * sp;
                    c.beginPath(); c.arc(x, cy, rr + 5, 0, kit.TAU); c.fillStyle = 'rgba(4,8,18,0.82)'; c.fill();
                    c.beginPath(); c.arc(x, cy, rr + 9, 0, kit.TAU); c.fillStyle = 'rgba(49,255,122,0.28)'; c.fill();
                    c.beginPath(); c.arc(x, cy, rr, 0, kit.TAU); c.fillStyle = '#31ff7a'; c.fill();
                }
                c.globalAlpha = 1;
            }
        }

        function strip(y, text, sub, col, icon, angle) {
            let gr = stripGrad.get(col);
            if (!gr) {
                gr = c.createLinearGradient(W / 2 - 230, 0, W / 2 + 230, 0);
                gr.addColorStop(0, 'rgba(6,10,22,0)'); gr.addColorStop(0.2, 'rgba(6,10,22,0.82)'); gr.addColorStop(0.8, 'rgba(6,10,22,0.82)'); gr.addColorStop(1, 'rgba(6,10,22,0)');
                stripGrad.set(col, gr);
            }
            const h = compact ? 26 : 34;
            c.fillStyle = gr; c.fillRect(W / 2 - 230, y, 460, h);
            c.fillStyle = col; c.fillRect(W / 2 - 150, y, 300, 2); c.fillRect(W / 2 - 150, y + h - 2, 300, 2);
            const r = h * 0.38;
            if (icon === 'bomb') kit.bomb(c, W / 2 - 170, y + h / 2, r, time);
            else if (icon === 'arrow') kit.arrow(c, W / 2 - 170, y + h / 2, r, angle || 0, col);
            else kit.warn(c, W / 2 - 170, y + h / 2, r, col);
            kit.text(c, text, W / 2 - 138, y + h - (compact ? 7 : 9), compact ? 15 : 19, '#ffffff', { weight: 900, sw: 3.5, stroke: 'rgba(4,8,18,0.9)' });
            if (sub) kit.text(c, sub, W / 2 + 190, y + h - (compact ? 7 : 9), compact ? 13 : 16, col, { align: 'right', mono: true, weight: 800, sw: 3 });
            return y + h + 6;
        }

        function bearing(race, tx, ty, pose) {
            const me = race.cars[0].v, th = AD.screenRotation(pose ? pose.heading : me.psi);
            const dx = tx - me.x, dy = ty - me.y;
            return Math.atan2(dx * Math.cos(th) - dy * Math.sin(th), -(dx * Math.sin(th) + dy * Math.cos(th)));
        }

        /* Warnings come straight from live state so they vanish the moment the threat does. */
        function drawWarnings(race, car, pose) {
            let y = G.WYs, count = 0;
            const me = car.v, blink = reduced || Math.floor(time * 6) % 2 === 0;
            const add = (text, sub, col, icon, angle) => { if (count < 2) { y = strip(y, text, sub, col, icon, angle); count++; } };
            if (race.bomb && race.bomb.holder === 0) add(up(tr('hud.have_bomb')), me.hasBomb ? Math.max(0, race.bomb.fuse).toFixed(1) + ' s' : '', blink ? WARN_RED : WARN_AMBER, 'bomb');
            for (const p of race.projectiles) {
                if (p.type === 'rocket' && p.target === 0 && p.owner !== 0) {
                    add(up(weaponName('rocket')), Math.round(Math.hypot(p.x - me.x, p.y - me.y)) + ' m', blink ? WARN_RED : WARN_AMBER, 'arrow', bearing(race, p.x, p.y, pose));
                    break;
                }
            }
            for (const b of race.beams) {
                if (b.type !== 'rail_aim' || b.car === 0) continue;
                const dx = b.x2 - b.x, dy = b.y2 - b.y, len2 = dx * dx + dy * dy || 1;
                const u = clamp(((me.x - b.x) * dx + (me.y - b.y) * dy) / len2, 0, 1);
                if (Math.hypot(b.x + dx * u - me.x, b.y + dy * u - me.y) < 9) { add(up(weaponName('railgun')), '', WARN_RED, 'arrow', bearing(race, b.x, b.y, pose)); break; }
            }
            for (const h of race.hazards) {
                if (h.type === 'cow_shadow' && h.owner !== 0 && h.land > 0 && Math.hypot(h.x - me.x, h.y - me.y) < 90) {
                    add(up(weaponName('cow')), h.land.toFixed(1) + ' s', WARN_AMBER, 'arrow', bearing(race, h.x, h.y, pose));
                    break;
                }
            }
            if (me.towedBy >= 0) add(up(weaponName('harpoon')), '', WARN_RED, 'warn');
            if (race.mode === 'elimination' && race.nextOut >= 0 && race.elimination && race.elimination.warned) {
                add(up(tr('hud.next_out')) + ' ' + (race.nextOut === 0 ? '' : initials(race.cars[race.nextOut])), race.clock !== null ? Math.ceil(race.clock) + ' s' : '', race.nextOut === 0 && blink ? WARN_RED : WARN_AMBER, 'warn');
            }
            if (race.sweeper && race.clock !== null && race.mode !== 'drift_attack') {
                add(tr('hud.sweeper', { seconds: Math.ceil(race.clock) }), '', WARN_AMBER, 'warn');
            }
            if (wrongWay > 1.0) add(up(tr('hud.wrong_way')), '', WARN_RED, 'warn');
            for (let i = animalWarn.length - 1; i >= 0 && count < 2; i--) {
                const a = animalWarn[i], d = Math.round(Math.hypot(a.x - me.x, a.y - me.y));
                if (d < 260) add(up(tr('hud.animals_ahead')), d + ' m', WARN_AMBER, 'warn');
            }
        }

        function drawStatus(car) {
            const v = car.v, items = [];
            if (v.shield > 0) items.push([tr('hud.shield'), v.shield / 4, '#5eead4']);
            if (v.frozen > 0) items.push([tr('hud.frozen'), v.frozen / 0.8, '#bae6fd']);
            if (v.toast > 0) items.push([tr('hud.toast'), v.toast / 0.6, '#fb923c']);
            if (v.steerWobble > 0) items.push([tr('hud.disco'), v.steerWobble / 0.3, '#f472b6']);
            if (v.cloak > 0) items.push([tr('hud.cloak'), v.cloak / 3, '#99f6e4']);
            if (v.overdrive > 0) items.push([tr('hud.overdrive'), v.overdrive / 3, '#f97316']);
            if (!items.length) return;
            const w = compact ? 86 : 118, gap = 8, total = items.length * (w + gap) - gap;
            let x = W / 2 - total / 2;
            const y = Hb - M - (compact ? 14 : 18);
            for (const it of items) {
                kit.para(c, x, y, w, compact ? 18 : 22, 7); c.fillStyle = 'rgba(6,10,24,0.78)'; c.fill();
                c.lineWidth = 1.6; c.strokeStyle = it[2]; c.stroke();
                kit.para(c, x, y + (compact ? 15 : 19), w * clamp(it[1], 0, 1), 3, 3); c.fillStyle = it[2]; c.fill();
                kit.text(c, up(it[0]), x + w / 2 + 3, y + (compact ? 13 : 16), compact ? 10 : 12, '#ffffff', { align: 'center', weight: 800, sw: 0, spacing: 1 });
                x += w + gap;
            }
        }

        function drawComms(dt) {
            if (!comms && queue.length) {
                comms = queue.shift();
            }
            if (!comms) return;
            comms.age += dt;
            if (comms.age >= comms.life) { comms = null; return; }
            const g = G, w = g.cw, h = g.ch, x = W / 2 - w / 2;
            const inP = clamp(comms.age / 0.28, 0, 1), out = clamp((comms.life - comms.age) / 0.3, 0, 1);
            const e = 1 - Math.pow(1 - inP, 3);
            const oy = reduced ? 0 : -(1 - e) * (h + M);
            c.save();
            c.globalAlpha = Math.min(reduced ? inP : 1, out);
            c.translate(0, oy);
            const y = M;
            const accent = comms.accent;
            kit.para(c, x, y, w, h, 14);
            if (!commsGrad) {
                commsGrad = c.createLinearGradient(x, 0, x + w, 0);
                commsGrad.addColorStop(0, 'rgba(8,12,30,0.9)'); commsGrad.addColorStop(1, 'rgba(8,12,30,0.55)');
            }
            c.fillStyle = commsGrad; c.fill();
            c.fillStyle = accent; c.fillRect(x + 14, y, w - 14, 3);
            const ps = h + 8, bob = reduced || comms.age > comms.life - 0.5 ? 0 : Math.sin(comms.age * 15) * 1.4;
            AD.drawPortrait(c, comms.driverId, x - 8, y - 4 + bob, ps, dpr * k);
            const tx = x + ps + 6;
            let ty = y + (compact ? 18 : 22);
            if (comms.name) { kit.text(c, up(comms.name), tx, ty, compact ? 11 : 13, accent, { weight: 900, spacing: 1.5, sw: 2.5 }); ty += compact ? 16 : 20; }
            for (let i = 0; i < comms.lines.length; i++) {
                kit.text(c, comms.lines[i], tx, ty + i * (compact ? 15 : 19), compact ? 13 : 16, '#ffffff', { italic: false, weight: 700, sw: 0 });
            }
            c.restore();
        }

        function drawChain(car) {
            const v = car.v, st = car.tricks;
            if (!v.airborne || !st || !(st.chainPoints > 0)) return;
            kit.text(c, '+' + kit.number(st.chainPoints) + (st.chain > 1 ? '  x' + st.chain : ''), W / 2, H * 0.64, compact ? 22 : 30, '#ffffff', { align: 'center', weight: 900 });
        }

        /* ---- render --------------------------------------------------------------------------- */
        /* A new race (restart, next cup track) starts with a clean slate. */
        function adopt(race) {
            if (race === curRace) return;
            curRace = race;
            for (let i = 0; i < 3; i++) { splits[i] = null; prevBest[i] = Infinity; }
            callouts.clear(); queue.length = 0; comms = null; animalWarn.length = 0;
            posPulse = 0; slotPulse = 0; hitFlash = 0; goT = 0; clockBig = null; turboFlash = null; blast = 0; wrongWay = 0;
        }

        function render(race, dt, settings, pose) {
            adopt(race);
            if (!layer) layout();
            time += dt;
            if (time - mqAt > 1) {
                mqAt = time;
                reduced = !!(settings && settings.reducedMotion) ||
                    !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
            }
            const car = race.cars[0], v = car.v;
            posPulse = Math.max(0, posPulse - dt * 1.4);
            slotPulse = Math.max(0, slotPulse - dt * 3);
            hitFlash = Math.max(0, hitFlash - dt * 2.6);
            blast = Math.max(0, blast - dt * 1.6);
            goT = Math.max(0, goT - dt);
            if (turboFlash && (turboFlash.t -= dt) <= 0) turboFlash = null;
            if (clockBig && (clockBig.t += dt) > 0.95) clockBig = null;
            for (let i = animalWarn.length - 1; i >= 0; i--) if ((animalWarn[i].t -= dt) <= 0) animalWarn.splice(i, 1);
            callouts.update(dt);
            /* Wrong way: moving against the track direction for a moment. */
            if (race.state === 'running' && Math.abs(v.vx) > 5) {
                const p = AD.pointAt(race.track, car.lastS || 0);
                const dir = v.psi + (v.vx < 0 ? Math.PI : 0);
                wrongWay = Math.cos(dir) * p.tx + Math.sin(dir) * p.ty < -0.3 ? wrongWay + dt : 0;
            } else wrongWay = 0;

            c.setTransform(dpr, 0, 0, dpr, 0, 0);
            c.clearRect(0, 0, cw, ch);
            c.drawImage(layer, 0, 0, cw, ch);
            c.setTransform(dpr * k, 0, 0, dpr * k, 0, 0);

            if (hitFlash > 0) { c.globalAlpha = hitFlash; c.fillStyle = G.hit; c.fillRect(0, 0, W, H); c.globalAlpha = 1; }
            if (blast > 0) { c.globalAlpha = blast * 0.5; c.fillStyle = '#ffffff'; c.fillRect(0, 0, W, H); c.globalAlpha = 1; }
            drawSpeed(race, car);
            drawWeapon(race, car);
            if (race.mode !== 'drift_attack') drawStyle(race, car);
            drawTopLeft(race, car);
            drawTower(race, car);
            AD.drawMinimap(c, race, G.MX, G.MY, G.MS, pose ? pose.heading : v.psi, time);
            const cond = race.conditions && race.conditions.state;
            if (cond && cond.kind !== 'clear' && cond.intensity > 0.05) {
                kit.text(c, up(tr('weather.' + cond.kind)) + ' ' + Math.round(cond.intensity * 100) + '%', G.MX + G.MS, G.MY + G.MS + 20, compact ? 11 : 13, '#bfeaff', { align: 'right', spacing: 1.5, weight: 800, sw: 3 });
            }
            drawCountdown(race);
            drawComms(dt);
            if (race.state !== 'countdown') drawWarnings(race, car, pose);
            if (clockBig && race.state === 'running') {
                const s = reduced ? 1 : 1 + 0.5 * Math.exp(-clockBig.t * 8);
                c.save(); c.translate(W / 2, H * 0.3); c.scale(s, s);
                c.globalAlpha = clamp(1 - (clockBig.t - 0.6) / 0.35, 0, 1);
                kit.text(c, String(clockBig.sec), 0, 0, compact ? 80 : 130, '#ff6b6b', { align: 'center', weight: 900, sw: 12 });
                c.restore();
            }
            callouts.draw(c, W / 2, H * 0.54, reduced);
            drawChain(car);
            drawStatus(car);
            c.setTransform(1, 0, 0, 1, 0, 0);
        }

        /* ---- events ------------------------------------------------------------------------------ */
        function event(evt, race) {
            if (race) adopt(race);
            const me = evt.car === 0, sz = compact ? 0.72 : 1;
            const say = (text, colour, size, sub, life) => callouts.push({ text: text, colour: colour, size: Math.round(size * sz), sub: sub, life: life });
            switch (evt.type) {
                case 'go': goT = 1.1; say(labels.ready || 'GO!', '#7dff9b', 118, '', 0.95); break;
                case 'perfect_start': if (me) say(tr('hud.perfect_start'), '#3ee6ff', 58, '+' + kit.number(evt.points || 0)); break;
                case 'wheelspin': if (me) say(tr('hud.wheelspin'), '#ffb066', 44); break;
                case 'lap': {
                    for (let i = 0; i < 3; i++) splits[i] = null;
                    const best = race.cars[0].bestLap;
                    say(tr('hud.lap_of', { lap: Math.min(evt.lap + 1, race.laps), total: race.laps }), '#ffffff', 56,
                        AD.formatTime(evt.time) + (evt.time <= best + 1e-6 ? '  ★ ' + labels.bestLap : ''), 2.2);
                    break;
                }
                case 'final_lap': say(tr('hud.final_lap'), WARN_AMBER, 68, '', 2.4); break;
                case 'sector': {
                    const i = evt.index, prev = prevBest[i];
                    let text, colour;
                    if (!isFinite(prev)) { text = evt.time.toFixed(1); colour = '#7dd3fc'; }
                    else if (evt.time <= prev) { text = '-' + (prev - evt.time).toFixed(2); colour = '#c084fc'; }
                    else { text = '+' + (evt.time - prev).toFixed(2); colour = '#ffb066'; }
                    prevBest[i] = Math.min(prev, evt.time);
                    splits[i] = { text: text, colour: colour };
                    break;
                }
                case 'position': posPulse = 1; posDir = evt.to < evt.from ? 1 : -1; break;
                case 'respawn': if (me) say(tr('hud.respawn'), '#bfeaff', 38); break;
                case 'drift_level': if (me) turboFlash = { level: evt.level, t: 0.5 }; break;
                case 'style_bank': if (me) say('+' + kit.number(evt.points), evt.multiplier > 2 ? WARN_AMBER : '#3ee6ff', 44, 'x' + evt.multiplier.toFixed(1)); break;
                case 'near_miss': if (me) say(tr('hud.near_miss'), '#ffb066', 38, '+' + kit.number(evt.bonus || 0)); break;
                case 'slingshot': if (me) say(tr('hud.slingshot'), '#c4f7ff', 52, '+' + kit.number(evt.points || 0)); break;
                case 'trick': if (me) say(tr('trick.' + evt.id), TRICK_COL[evt.id] || '#a78bfa', 54, '+' + kit.number(evt.points), 1.7); break;
                case 'landing':
                    if (me) { const q = evt.quality; say(tr('landing.' + (q === 'crash' ? 'pancake' : q)), LAND_COL[q] || '#fff', q === 'perfect' ? 62 : 50, evt.points ? '+' + kit.number(evt.points) : '', 1.6); }
                    break;
                case 'reverse_hero': if (me) say(tr('trick.reverse_hero'), '#fde047', 46, '+' + kit.number(evt.points || 0)); break;
                case 'j_turn': if (me) say(tr('trick.j_turn'), '#fde047', 50, '+' + kit.number(evt.points || 0)); break;
                case 'boost_pad': if (me) say(tr('hud.boost'), WARN_AMBER, 42, '', 1.0); break;
                case 'animal_cross': {
                    /* One row per crossing: a repeat of the same crossing, or a neighbour within 14 m, only refreshes it. */
                    const same = animalWarn.find(a => (evt.crossing !== undefined && a.id === evt.crossing) || Math.hypot(a.x - evt.x, a.y - evt.y) < 14);
                    if (same) same.t = 4; else animalWarn.push({ x: evt.x, y: evt.y, t: 4, id: evt.crossing });
                    if (animalWarn.length > 4) animalWarn.shift();
                    break;
                }
                case 'animal_hit': if (me) say(tr('hud.cow_bowling'), '#f8fafc', 52, '+' + kit.number(evt.points || 0)); break;
                case 'strike': if (me) say(tr('hud.strike'), '#ffd66b', 78, 'x' + evt.count, 2.0); break;
                case 'pickup': if (me) slotPulse = 1; break;
                case 'hit':
                    if (evt.car === 0) {
                        hitFlash = evt.heavy ? 0.9 : 0.55;
                        if (evt.weapon && AD.WEAPONS[evt.weapon] && evt.by !== 0) say(weaponName(evt.weapon), '#ff7a7a', 40, '', 1.2);
                    }
                    break;
                case 'toast': if (me) say(tr('hud.toast'), '#fb923c', 50); break;
                case 'elimination_warn': if (race) say(tr('hud.next_out'), evt.car === 0 ? WARN_RED : WARN_AMBER, evt.car === 0 ? 62 : 40, nameOf(race, evt.car), 2.0); break;
                case 'eliminated': if (race) say(tr('hud.eliminated', { name: nameOf(race, evt.car) }), evt.car === 0 ? WARN_RED : '#ffffff', evt.car === 0 ? 62 : 40, 'P' + evt.position, 2.2); break;
                case 'bomb_given': if (race) say(evt.car === 0 ? tr('hud.have_bomb') : tr('hud.bomb', { name: nameOf(race, evt.car) }), evt.car === 0 ? WARN_RED : WARN_AMBER, evt.car === 0 ? 58 : 38, evt.car === 0 ? tr('hud.pass_it') : '', 2.2); break;
                case 'bomb_pass': if (evt.to === 0) say(tr('hud.have_bomb'), WARN_RED, 54, tr('hud.pass_it'), 1.8); break;
                case 'bomb_blast': blast = 1; break;
                case 'clock': clockBig = { sec: evt.seconds, t: 0 }; break;
                case 'sweeper': say(tr('hud.sweeper', { seconds: (race && race.rules && race.rules.sweeper) || 45 }), WARN_AMBER, 40, '', 2.4); break;
                default: break;
            }
        }

        function radio(msg) {
            if (!msg || !msg.text) return;
            const d = AD.DRIVERS.find(e => e.id === msg.driverId) || null;
            c.font = '700 ' + (compact ? 13 : 16) + 'px ' + kit.FAM;
            const lines = kit.wrap(c, msg.text, G.cw - G.ch - 28, 2);
            const item = {
                driverId: msg.driverId || 'marshal', name: d ? d.name : (msg.name || ''), lines: lines, age: 0,
                life: 4, accent: d ? d.portrait.helmet : '#facc15'
            };
            if (comms && comms.age < comms.life - 0.5) { queue.length = 0; queue.push(item); } else { comms = item; }
        }

        return {
            resize: resize,
            /* Reserve the lowest px of the canvas for on-screen touch controls. */
            setInset(px) { px = Math.max(0, px | 0); if (px !== inset) { inset = px; layout(); } },
            event: event,
            radio: radio,
            showMessage(value, seconds) { callouts.push({ text: value, colour: '#ffe6a8', size: 38, life: seconds || 1.6 }); },
            popup(value, colour) { callouts.push({ text: value, colour: colour || '#ffffff', size: 34, life: 1.4 }); },
            clear() { c.setTransform(1, 0, 0, 1, 0, 0); c.clearRect(0, 0, canvas.width, canvas.height); },
            render: render,
            dispose() {
                callouts.clear(); queue.length = 0; comms = null; animalWarn.length = 0; stripGrad.clear();
                layer = null; curRace = null;
            }
        };
    };
})();
