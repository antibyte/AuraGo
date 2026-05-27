(function () {
    'use strict';
    const W = 480, H = 640, PLAYER_SPEED = 220, PB_SPEED = 500, EB_SPEED = 260;
    const FCOLS = 10, FROWS = 5, ESP_X = 36, ESP_Y = 32, FTOP = 60, DIVE_SPD = 180;
    const EXTRA_LIFE = 20000, TITLE_IDLE = 15000;
    const instances = new Map();

    function render(host, windowId, context) {
        if (!host) return;
        const ctx = context || {};
        const esc = ctx.esc || (v => String(v == null ? '' : v));
        const t = ctx.t || ((k, f) => f || k);
        const api = ctx.api || ((url, opts) => fetch(url, opts).then(r => r.json()));
        const state = { disposed: false };
        instances.set(windowId, state);

        host.innerHTML = '<div class="galaxa-app"><div class="galaxa-canvas-wrap"><canvas class="galaxa-canvas" data-gc></canvas></div>' +
            '<div class="galaxa-overlay" data-go></div><div class="galaxa-pause-overlay" data-gp hidden><span class="galaxa-pause-text">' + esc(t('galaxa.paused', 'PAUSED')) + '</span></div></div>';

        const canvas = host.querySelector('[data-gc]');
        const overlayEl = host.querySelector('[data-go]');
        const pauseEl = host.querySelector('[data-gp]');
        const c = canvas.getContext('2d');
        c.imageSmoothingEnabled = false;
        let scale = 1, tick = 0, rafId = 0, lastT = 0;

        const G = {
            st: 'TITLE', score: 0, lives: 3, stage: 1, hi: 10000, hiScores: [],
            p: { x: W / 2, y: H - 50, alive: true, inv: 0, dual: false, cap: null },
            bul: [], ebul: [], enemies: [], exp: [], part: [],
            fX: 0, fTmr: 0, dTmr: 0, sTmr: 0, tIdle: 0,
            attract: false, aTmr: 0, ne: { ch: [65, 65, 65], pos: 0, done: false },
            chal: false, chalHits: 0, chalTot: 0, beam: null, shkT: 0, shkM: 0,
            inp: { l: false, r: false, f: false, fp: false, s: false, sp: false, p: false, pp: false, u: false, d: false, rp: false, lp: false, up: false, dp: false },
            kb: { l: false, r: false, u: false, d: false, f: false, s: false, p: false },
            gp: { l: false, r: false, u: false, d: false, f: false, s: false, p: false },
            muted: false, vol: 0.3, _prevSt: 'TITLE'
        };

        let actx = null;
        function audio() {
            if (!actx) try { actx = new (window.AudioContext || window.webkitAudioContext)(); } catch (e) { return null; }
            if (actx && actx.state === 'suspended') actx.resume();
            return actx;
        }
        function beep(type, f0, f1, dur, vol) {
            const a = audio(); if (!a || G.muted) return;
            const o = a.createOscillator(), g = a.createGain();
            o.type = type; o.frequency.setValueAtTime(f0, a.currentTime);
            if (f1 !== f0) o.frequency.linearRampToValueAtTime(f1, a.currentTime + dur);
            g.gain.setValueAtTime(G.vol * vol, a.currentTime);
            g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + dur + 0.02);
            o.connect(g).connect(a.destination); o.start(); o.stop(a.currentTime + dur + 0.02);
        }
        function noise(dur, vol, freq) {
            const a = audio(); if (!a || G.muted) return;
            const buf = a.createBuffer(1, a.sampleRate * dur, a.sampleRate), d = buf.getChannelData(0);
            for (let i = 0; i < d.length; i++) d[i] = (Math.random() * 2 - 1) * (1 - i / d.length);
            const s = a.createBufferSource(), f = a.createBiquadFilter(), g = a.createGain();
            s.buffer = buf; f.type = 'lowpass'; f.frequency.value = freq || 2000;
            g.gain.setValueAtTime(G.vol * vol, a.currentTime);
            g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + dur);
            s.connect(f).connect(g).connect(a.destination); s.start();
        }
        const SFX = {
            shoot() { beep('sine', 800, 1200, 0.08, 0.3); },
            dive() { beep('sawtooth', 600, 200, 0.3, 0.15); },
            eExplode() { noise(0.15, 0.4, 2000); },
            pExplode() { noise(0.4, 0.6, 1200); beep('sine', 60, 60, 0.3, 0.5); },
            stage() { [523, 659, 784, 1047].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.2, 0.25), i * 120); }); },
            challenge() { [440, 554, 659, 880, 1109, 1319].forEach((f, i) => { setTimeout(() => beep('square', f, f, 0.15, 0.2), i * 80); }); },
            extra() { beep('sine', 1200, 1200, 0.2, 0.3); },
            rescue() { beep('sine', 880, 880, 0.2, 0.25); setTimeout(() => beep('sine', 1100, 1100, 0.2, 0.25), 100); },
            beam() { beep('sawtooth', 200, 200, 0.5, 0.15); }
        };

        const PTS = { bee: [50, 100], butterfly: [80, 160], boss: [400, 800] };
        const SP = buildSprites();
        const STARS = [];
        for (let i = 0; i < 80; i++) STARS.push({ x: Math.random() * W, y: Math.random() * H, sp: 10 + Math.random() * 60, br: 0.2 + Math.random() * 0.8, sz: Math.random() > 0.85 ? 2 : 1 });

        function buildSprites() {
            const p = (s) => { const rows = s.trim().split('\n'); return rows.map(r => r.split('').map(ch => parseInt(ch, 16) || 0)); };
            return {
                player: p('0000000110000000\n0000000110000000\n0000011110000000\n0000012110000000\n0000112111000000\n0000112111000000\n0001112111100000\n0001112111100000\n0011111111110000\n0013111111310000\n0013311113310000\n0113311113311000\n0113331133311000\n0113331133311000\n1113331133311100\n0000000000000000'),
                pC: { 1: '#ffffff', 2: '#88bbff', 3: '#4488ff' },
                bee: [p('0000004400000000\n0000044440000000\n0000444544400000\n0000455544400000\n0004445444440000\n0444444444444000\n0464444444464000\n0466444444664000\n0066444444660000\n0000644446000000\n0000040004000000\n0000040004000000\n0000000000000000\n0000000000000000\n0000000000000000\n0000000000000000'),
                    p('0000004400000000\n0000044440000000\n0000444544400000\n0000455544400000\n0004445444440000\n0444444444444000\n0464444444464000\n4466444444664400\n4006644446600400\n0000644446000000\n0000040004000000\n0000000000000000\n0000000000000000\n0000000000000000\n0000000000000000\n0000000000000000')],
                bC: { 4: '#ffcc00', 5: '#ff8800', 6: '#ff4444' },
                bf: [p('0000000660000000\n0000006666000000\n0000066766600000\n0000667776660000\n0006667676666000\n0666666666666600\n6666666666666660\n6066666666666060\n0000666666660000\n0000006666000000\n0000006006000000\n0000006006000000\n0000000000000000\n0000000000000000\n0000000000000000\n0000000000000000'),
                    p('0000000660000000\n0000006666000000\n0000066766600000\n0000667776660000\n0006667676666000\n0666666666666600\n6666666666666660\n0600666666660060\n0000066666660000\n0000006666000000\n0000006006000000\n0000000000000000\n0000000000000000\n0000000000000000\n0000000000000000\n0000000000000000')],
                bfC: { 6: '#ff3366', 7: '#44bbff' },
                boss: p('00000008888000000000\n00000088988800000000\n00000889998880000000\n00008899999888000000\n00088898889888800000\n00888888888888880000\n088a8888888888a88000\n088aa88888888aa88000\n0800aa888888aa008000\n000000aa8888aa0000000\n00000000aaaa000000000\n000000000a000a0000000\n000000000a000a0000000\n00000000000000000000\n00000000000000000000\n00000000000000000000'),
                bossHit: p('0000000bbbb000000000\n000000bbbbbb00000000\n00000bbbbbbbbb0000000\n0000bbbbbbbbbb0000000\n000bbbbbbbbbbbbb00000\n00bbbbbbbbbbbbbbb0000\n0bbbbbbbbbbbbbbbbb000\n0bbbbbbbbbbbbbbbbb000\n0b00bbbbbbbbbbb00b000\n00000bbbbbbbbbb000000\n0000000bbbbbbb0000000\n000000000b000b0000000\n000000000b000b0000000\n00000000000000000000\n00000000000000000000\n00000000000000000000'),
                bossC: { 8: '#44cc44', 9: '#88ff88', a: '#ff4444', b: '#88ccff' }
            };
        }

        function drawSp(cv, sp, cols, x, y, flash) {
            for (let r = 0; r < sp.length; r++) for (let cl = 0; cl < sp[r].length; cl++) {
                const v = sp[r][cl]; if (!v) continue;
                cv.fillStyle = flash ? '#fff' : (cols[v] || '#fff');
                cv.fillRect(Math.floor(x + cl), Math.floor(y + r), 1, 1);
            }
        }
        function drawStars(cv, dt) {
            for (const s of STARS) {
                s.y += s.sp * dt; if (s.y > H) { s.y = 0; s.x = Math.random() * W; }
                cv.fillStyle = 'rgba(255,255,255,' + (s.br * (0.6 + 0.4 * Math.sin(tick * 0.02 + s.x))) + ')';
                cv.fillRect(Math.floor(s.x), Math.floor(s.y), s.sz, s.sz);
            }
        }

        function resize() {
            const w = host.querySelector('.galaxa-canvas-wrap');
            if (!w) return;
            scale = Math.max(1, Math.min(Math.floor(w.clientWidth / W) || 1, Math.floor(w.clientHeight / H) || 1));
            canvas.width = W * scale; canvas.height = H * scale;
            canvas.style.width = (W * scale) + 'px'; canvas.style.height = (H * scale) + 'px';
            c.imageSmoothingEnabled = false;
        }

        function isChal(s) { return s >= 3 && (s - 3) % 4 === 0; }

        function mkFormation() {
            G.enemies = []; G.chal = isChal(G.stage); G.chalHits = 0;
            let idx = 0;
            for (let r = 0; r < FROWS; r++) for (let col = 0; col < FCOLS; col++) {
                let type = 'bee';
                if (r === 0) { if (col < 3 || col > 6) continue; type = 'boss'; } else if (r <= 2) type = 'butterfly';
                const fx = W / 2 + (col - FCOLS / 2 + 0.5) * ESP_X, fy = FTOP + r * ESP_Y;
                const side = idx % 2 === 0 ? -1 : 1;
                const diveDelay = G.chal ? 99999 : (1000 + Math.random() * 3000 + idx * 50);
                G.enemies.push({ type, r, col, x: W / 2 + side * (120 + Math.random() * 80), y: -30 - (idx % 8) * 20,
                    fx, fy, hp: type === 'boss' ? 2 : 1, st: 'ENTER', eTmr: 500 + idx * 80 + r * 100, eProg: 0,
                    fr: 0, frT: 0, dTmr: diveDelay, dPath: null, sTmr: 0, hasCap: false, hitF: 0 });
                idx++;
            }
            G.chalTot = G.enemies.length; G.dTmr = 2000 - Math.min(G.stage * 100, 1200); G.fX = 0;
        }

        function startStage() {
            G.st = 'STAGE_INTRO'; G.sTmr = 2000; G.bul = []; G.ebul = []; G.exp = []; G.part = []; G.beam = null;
            G.p.x = W / 2; G.p.alive = true; G.p.inv = 2000; G.p.cap = null; G.p.dual = false;
            G.chal ? SFX.challenge() : SFX.stage();
        }

        function fire() {
            const max = G.p.dual ? 2 : 1;
            if (G.bul.length >= max) return;
            G.bul.push({ x: G.p.x, y: G.p.y - 8, w: 2, h: 6 });
            if (G.p.dual && G.bul.length < max) G.bul.push({ x: G.p.x + 24, y: G.p.y - 8, w: 2, h: 6 });
            SFX.shoot();
        }
        function boom(x, y) {
            const seed = Math.random();
            G.exp.push({ x, y, t: 0, dur: 300, seed });
            for (let i = 0; i < 8; i++) { const a = (i / 8) * Math.PI * 2 + seed, sp = 40 + (i * 17 % 120); G.part.push({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 200 + (i * 37 % 200), t: 0, col: ['#ffcc00', '#ff4444', '#ff8800', '#fff'][i % 4] }); }
        }
        function addScore(pts) { const prev = G.score; G.score += pts; if (G.score > G.hi) G.hi = G.score; if (Math.floor(G.score / EXTRA_LIFE) > Math.floor(prev / EXTRA_LIFE)) { G.lives++; SFX.extra(); } }
        function hit(a, b) { return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y; }

        function killP() {
            if (!G.p.alive) return; G.p.alive = false; boom(G.p.x, G.p.y); SFX.pExplode(); G.shkT = 300; G.shkM = 4; G.lives--;
            if (G.lives < 0) { G.st = 'GAME_OVER'; G.sTmr = 3000; }
            else setTimeout(() => { if (!state.disposed) { G.p.x = W / 2; G.p.alive = true; G.p.inv = 3000; } }, 1500);
        }

        function updateP(dt) {
            if (!G.p.alive) return;
            const inp = G.inp;
            if (inp.l) G.p.x -= PLAYER_SPEED * dt; if (inp.r) G.p.x += PLAYER_SPEED * dt;
            G.p.x = Math.max(10, Math.min(W - 10, G.p.x));
            if (G.p.inv > 0) G.p.inv -= dt * 1000;
            if (inp.f && !inp.fp) fire();
            if (G.beam && G.beam.active && G.p.x > G.beam.x - 20 && G.p.x < G.beam.x + 20 && G.p.y > G.beam.y) {
                G.p.alive = false; G.beam.cap = true; G.beam.capT = 0; SFX.beam();
            }
        }

        function updateBul(dt) {
            for (let i = G.bul.length - 1; i >= 0; i--) {
                G.bul[i].y -= PB_SPEED * dt;
                if (G.bul[i].y < -10) { G.bul.splice(i, 1); continue; }
                let hitE = false;
                for (let j = G.enemies.length - 1; j >= 0; j--) {
                    const e = G.enemies[j]; if (e.st === 'DEAD') continue;
                    const ew = e.type === 'boss' ? 20 : 16;
                    if (hit(G.bul[i], { x: e.x - ew / 2, y: e.y - 8, w: ew, h: 16 })) {
                        e.hp--; if (e.hp <= 0) { addScore(PTS[e.type][e.st === 'DIVING' ? 1 : 0]); boom(e.x, e.y); SFX.eExplode(); if (e.hasCap) G.p.cap = { x: e.x, y: e.y }; if (G.chal) G.chalHits++; e.st = 'DEAD'; } else e.hitF = 100;
                        hitE = true; break;
                    }
                }
                if (hitE) G.bul.splice(i, 1);
            }
            for (let i = G.ebul.length - 1; i >= 0; i--) {
                const b = G.ebul[i]; b.y += EB_SPEED * dt;
                if (b.y > H + 10) { G.ebul.splice(i, 1); continue; }
                if (G.p.alive && G.p.inv <= 0 && hit(b, { x: G.p.x - 6, y: G.p.y - 6, w: 12, h: 12 })) { killP(); G.ebul.splice(i, 1); }
            }
        }

        function startDive(e) {
            if (e.st !== 'FORM') return; e.st = 'DIVING'; e.dPath = { ph: 0, amp: 30 + Math.random() * 40 }; e.dTmr = 3000; e.sTmr = 500 + Math.random() * 1000; SFX.dive();
            if (e.type === 'boss' && !e.hasCap && !G.beam && G.stage > 1 && Math.random() < 0.3) G.beam = { active: true, owner: e, x: e.x, y: e.y + 16, h: 0, t: 0, cap: false, capT: 0 };
        }

        function updateE(dt) {
            const dtMs = dt * 1000; G.fTmr += dt; G.fX = Math.sin(G.fTmr * 0.5) * 30;
            for (const e of G.enemies) {
                if (e.st === 'DEAD') continue; e.frT += dtMs; if (e.frT > 300) { e.fr = 1 - e.fr; e.frT = 0; } if (e.hitF > 0) e.hitF -= dtMs;
                if (e.st === 'ENTER') { e.eTmr -= dtMs; if (e.eTmr <= 0) { e.eProg += dt * 1.5; const tm = Math.min(e.eProg, 1); e.x += (e.fx - e.x) * tm * 0.05; e.y += (e.fy - e.y) * tm * 0.05; if (tm >= 1 && Math.abs(e.x - e.fx) < 2 && Math.abs(e.y - e.fy) < 2) { e.x = e.fx; e.y = e.fy; e.st = 'FORM'; } } }
                else if (e.st === 'FORM') {
                    e.x = e.fx + G.fX; e.y = e.fy + Math.sin(G.fTmr * 2 + e.col * 0.5) * 3;
                    if (!G.chal) { e.dTmr -= dtMs; if (e.dTmr <= 0 && Math.random() < 0.008 * Math.min(G.stage, 10)) startDive(e); }
                    else { e.dTmr -= dtMs; if (e.dTmr <= 0) startChalDive(e); }
                }
                else if (e.st === 'DIVING') { e.dTmr -= dtMs; if (e.dTmr <= 0 || e.y > H + 20) { e.st = 'RETURN'; e.y = -20; } else { e.y += DIVE_SPD * dt; if (e.dPath) { e.dPath.ph += dt * 3; e.x += Math.sin(e.dPath.ph) * e.dPath.amp * dt * 2; } if (G.beam && G.beam.owner === e) { G.beam.x = e.x; G.beam.y = e.y + 16; } e.sTmr -= dtMs; if (e.sTmr <= 0 && !G.chal) { if (e.type !== 'bee') { G.ebul.push({ x: e.x, y: e.y + 8, w: 2, h: 6 }); if (G.stage > 3 && e.type === 'boss') G.ebul.push({ x: e.x - 6, y: e.y + 8, w: 2, h: 6 }); } e.sTmr = 800 + Math.random() * 1200; } if (G.p.alive && G.p.inv <= 0) { const ew = e.type === 'boss' ? 16 : 12; if (hit({ x: e.x - ew / 2, y: e.y - 8, w: ew, h: 16 }, { x: G.p.x - 6, y: G.p.y - 6, w: 12, h: 12 })) { addScore(PTS[e.type][1]); boom(e.x, e.y); SFX.eExplode(); e.st = 'DEAD'; killP(); } } } }
                else if (e.st === 'RETURN') { e.x += (e.fx + G.fX - e.x) * dt * 3; e.y += (e.fy - e.y) * dt * 3; if (Math.abs(e.x - e.fx - G.fX) < 3 && Math.abs(e.y - e.fy) < 3) { if (G.chal) { e.st = 'DEAD'; if (G.chal) G.chalHits++; } else e.st = 'FORM'; } }
            }
            if (G.beam && G.beam.active) { G.beam.t += dtMs; G.beam.h = Math.min(200, G.beam.h + dt * 300); if (G.beam.t > 3000) { G.beam.active = false; if (G.beam.cap && G.p.cap) { G.beam.owner.hasCap = true; G.p.cap = null; } } }
            G.dTmr -= dtMs;
            if (G.dTmr <= 0 && !G.chal) { const fe = G.enemies.filter(e => e.st === 'FORM'); if (fe.length) startDive(fe[Math.floor(Math.random() * fe.length)]); G.dTmr = Math.max(500, 2000 - G.stage * 100); }
            if (G.enemies.every(e => e.st === 'DEAD')) { G.stage++; startStage(); }
        }

        function startChalDive(e) {
            if (e.st !== 'FORM') return; e.st = 'DIVING'; e.dPath = { ph: 0, amp: 50 + Math.random() * 30 }; e.dTmr = 4000; e.sTmr = 99999;
            SFX.dive();
        }

        function updateExp(dt) { for (let i = G.exp.length - 1; i >= 0; i--) { G.exp[i].t += dt * 1000; if (G.exp[i].t >= G.exp[i].dur) G.exp.splice(i, 1); } for (let i = G.part.length - 1; i >= 0; i--) { const p = G.part[i]; p.x += p.vx * dt; p.y += p.vy * dt; p.t += dt * 1000; if (p.t >= p.life) G.part.splice(i, 1); } }

        function update(dt) {
            if (dt > 0.1) dt = 0.1; tick++;
            if (G.inp.p && !G.inp.pp) { if (G.st === 'PAUSED') { G.st = G._prevSt; pauseEl.hidden = true; } else if (G.st === 'PLAYING') { G._prevSt = G.st; G.st = 'PAUSED'; pauseEl.hidden = false; } }
            if (G.st === 'PAUSED') return;
            if (G.st === 'TITLE') {
                G.tIdle += dt * 1000;
                if (G.tIdle > TITLE_IDLE && !G.attract) { G.attract = true; G.aTmr = 0; G.score = 0; G.lives = 3; G.stage = 1; G.p.x = W / 2; G.p.alive = true; G.p.inv = 0; G.bul = []; G.ebul = []; G.exp = []; G.part = []; mkFormation(); }
                if (G.attract) { updateAttract(dt); updateP(dt); updateBul(dt); updateE(dt); updateExp(dt); if (G.inp.s && !G.inp.sp) { G.attract = false; G.tIdle = 0; G.score = 0; G.lives = 3; G.stage = 1; G.p.dual = false; G.p.cap = null; startStage(); } }
                else if (G.inp.s && !G.inp.sp) { G.score = 0; G.lives = 3; G.stage = 1; G.p.dual = false; G.p.cap = null; startStage(); }
                return;
            }
            if (G.st === 'STAGE_INTRO') { G.sTmr -= dt * 1000; if (G.sTmr <= 0) { G.st = 'PLAYING'; mkFormation(); } return; }
            if (G.st === 'GAME_OVER') { G.sTmr -= dt * 1000; updateExp(dt); if (G.sTmr <= 0) { if (G.score > 0 && isHS(G.score)) { G.st = 'HIGH_SCORE'; G.ne = { ch: [65, 65, 65], pos: 0, done: false }; showHSOverlay(); } else { G.st = 'TITLE'; G.tIdle = 0; showTitle(); } } return; }
            if (G.st === 'HIGH_SCORE') { handleName(); return; }
            if (G.st === 'PLAYING') { updateP(dt); updateBul(dt); updateE(dt); updateExp(dt); if (G.shkT > 0) G.shkT -= dt * 1000; if (G.p.cap) { G.p.cap.y -= 100 * dt; if (G.p.cap.y < G.p.y - 20) { G.p.dual = true; G.p.cap = null; SFX.rescue(); } } }
        }

        function updateAttract(dt) {
            G.aTmr += dt * 1000;
            if (G.aTmr > 500) { G.aTmr = 0; G.inp.l = Math.random() < 0.25; G.inp.r = !G.inp.l && Math.random() < 0.25; if (Math.random() < 0.3) { G.inp.f = true; setTimeout(() => G.inp.f = false, 50); } }
        }

        function renderFrame() {
            c.save(); c.setTransform(scale, 0, 0, scale, 0, 0);
            let sx = 0, sy = 0; if (G.shkT > 0) { sx = (Math.random() - 0.5) * G.shkM; sy = (Math.random() - 0.5) * G.shkM; }
            c.translate(sx, sy); c.fillStyle = '#000'; c.fillRect(-5, -5, W + 10, H + 10); drawStars(c, 1 / 60);
            if (G.st === 'TITLE' && !G.attract) renderTitle();
            else if (G.st === 'STAGE_INTRO') renderStageIntro();
            else renderGame();
            c.restore();
        }

        function renderTitle() {
            c.textAlign = 'center'; c.fillStyle = '#4488ff'; c.font = 'bold 36px "Courier New",monospace'; c.fillText('GALAXA', W / 2, 180);
            c.fillStyle = '#ffcc00'; c.font = 'bold 20px "Courier New",monospace'; c.fillText('DELUXE', W / 2, 210);
            if (Math.sin(tick * 0.08) > 0) { c.fillStyle = '#fff'; c.font = '14px "Courier New",monospace'; c.fillText(t('galaxa.insert_coin', 'PRESS START'), W / 2, 320); }
            c.fillStyle = '#4488ff'; c.font = '12px "Courier New",monospace'; c.fillText(t('galaxa.high_score', 'HIGH SCORE'), W / 2, 260);
            c.fillStyle = '#ffcc00'; c.fillText(String(G.hi).padStart(8, '0'), W / 2, 280);
            if (G.hiScores.length) { c.fillStyle = '#aaccee'; c.font = '11px "Courier New",monospace'; let y = 380; c.fillText('RANK   NAME    SCORE    STAGE', W / 2, y); y += 18; G.hiScores.forEach((h, i) => { c.fillText((i + 1) + '    ' + h.name.padEnd(3) + '   ' + String(h.score).padStart(8) + '   ' + String(h.stage).padStart(3), W / 2, y); y += 16; }); }
            c.fillStyle = '#666'; c.font = '10px "Courier New",monospace'; c.fillText('ARROWS + SPACE  |  GAMEPAD D-PAD + A', W / 2, H - 40);
        }

        function renderStageIntro() {
            c.textAlign = 'center'; c.fillStyle = '#ffcc00'; c.font = 'bold 24px "Courier New",monospace';
            c.fillText(G.chal ? t('galaxa.challenge_stage', 'CHALLENGE STAGE') : t('galaxa.stage', 'STAGE') + ' ' + G.stage, W / 2, H / 2 - 20);
            c.fillStyle = '#fff'; c.font = '14px "Courier New",monospace'; c.fillText('READY', W / 2, H / 2 + 20);
        }

        function renderGame() {
            const p = G.p;
            if (p.alive) { const fl = p.inv > 0 && Math.floor(p.inv / 100) % 2 === 0; drawSp(c, SP.player, SP.pC, p.x - 8, p.y - 8, fl); if (p.dual) drawSp(c, SP.player, SP.pC, p.x + 16, p.y - 8, fl); }
            if (p.cap) drawSp(c, SP.player, SP.pC, p.cap.x - 8, p.cap.y - 8, false);
            for (const b of G.bul) { c.fillStyle = '#ffff88'; c.fillRect(Math.floor(b.x - 1), Math.floor(b.y - 3), 2, 6); }
            for (const b of G.ebul) { c.fillStyle = '#ff4444'; c.fillRect(Math.floor(b.x - 1), Math.floor(b.y - 3), 2, 6); }
            for (const e of G.enemies) {
                if (e.st === 'DEAD') continue; const fl = e.hitF > 0; let sp, cols;
                if (e.type === 'bee') { sp = SP.bee[e.fr]; cols = SP.bC; }
                else if (e.type === 'butterfly') { sp = SP.bf[e.fr]; cols = SP.bfC; }
                else { sp = e.hp < 2 ? SP.bossHit : SP.boss; cols = SP.bossC; }
                const ew = e.type === 'boss' ? 20 : 16; drawSp(c, sp, cols, e.x - ew / 2, e.y - 8, fl);
            }
            if (G.beam && G.beam.active) renderBeam(G.beam);
            for (const ex of G.exp) { const pr = ex.t / ex.dur, f = Math.min(3, Math.floor(pr * 4)), sz = 4 + f * 3; c.globalAlpha = 1 - pr; for (let i = 0; i < sz; i++) { const a = (i / sz) * Math.PI * 2 + ex.seed, d = f * 3; c.fillStyle = ['#ffcc00', '#ff4444', '#ff8800'][i % 3]; c.fillRect(Math.floor(ex.x + Math.cos(a) * d), Math.floor(ex.y + Math.sin(a) * d), 2, 2); } c.globalAlpha = 1; }
            for (const pt of G.part) { c.globalAlpha = Math.max(0, 1 - pt.t / pt.life); c.fillStyle = pt.col; c.fillRect(Math.floor(pt.x), Math.floor(pt.y), 2, 2); } c.globalAlpha = 1;
            renderHUD();
            if (G.st === 'GAME_OVER') { c.fillStyle = 'rgba(0,0,0,0.5)'; c.fillRect(0, H / 2 - 30, W, 60); c.fillStyle = '#ff4444'; c.font = 'bold 24px "Courier New",monospace'; c.textAlign = 'center'; c.fillText(t('galaxa.game_over', 'GAME OVER'), W / 2, H / 2 + 8); }
        }

        function renderBeam(tb) {
            c.strokeStyle = '#4488ff'; c.lineWidth = 2; const w = 20 + Math.sin(tick * 0.15) * 8;
            for (let i = 0; i < 8; i++) { const t = i / 8, y1 = tb.y + t * tb.h, y2 = tb.y + (t + 0.125) * tb.h, ww = w * (1 - t * 0.3); c.globalAlpha = 0.4 + 0.3 * Math.sin(tick * 0.2 + i); c.beginPath(); c.moveTo(tb.x - ww / 2, y1); c.lineTo(tb.x - ww * 0.4, y2); c.stroke(); c.beginPath(); c.moveTo(tb.x + ww / 2, y1); c.lineTo(tb.x + ww * 0.4, y2); c.stroke(); } c.globalAlpha = 1;
        }

        function renderHUD() {
            c.fillStyle = '#4488ff'; c.font = '12px "Courier New",monospace'; c.textAlign = 'left'; c.fillText(t('galaxa.score', 'SCORE'), 10, 16);
            c.fillStyle = '#fff'; c.fillText(String(G.score).padStart(8, '0'), 10, 32);
            c.fillStyle = '#4488ff'; c.textAlign = 'right'; c.fillText(t('galaxa.high_score', 'HIGH SCORE'), W - 10, 16);
            c.fillStyle = '#ffcc00'; c.fillText(String(G.hi).padStart(8, '0'), W - 10, 32);
            c.fillStyle = '#4488ff'; c.textAlign = 'center'; c.fillText(t('galaxa.stage', 'STAGE') + ' ' + G.stage, W / 2, 16);
            for (let i = 0; i < Math.min(G.lives, 5); i++) drawSp(c, SP.player, SP.pC, 10 + i * 18, H - 18, false);
        }

        function showTitle() { overlayEl.classList.remove('active'); overlayEl.innerHTML = ''; }
        function showHSOverlay() {
            overlayEl.classList.add('active');
            let h = '<div class="galaxa-overlay-box"><h2>' + esc(t('galaxa.game_over', 'GAME OVER')) + '</h2>';
            h += '<p>' + esc(t('galaxa.score', 'SCORE')) + ': ' + G.score + '</p><p>' + esc(t('galaxa.stage', 'STAGE')) + ': ' + G.stage + '</p>';
            h += '<p style="margin-top:12px">' + esc(t('galaxa.enter_name', 'ENTER YOUR NAME')) + '</p>';
            h += '<div class="galaxa-name-entry" data-ne>';
            for (let i = 0; i < 3; i++) h += '<div class="galaxa-name-char' + (i === 0 ? ' active' : '') + '" data-ci="' + i + '">A</div>';
            h += '</div><p style="font-size:10px;color:#666">\u2191\u2193 change  \u2190\u2192 select  ENTER confirm</p></div>';
            overlayEl.innerHTML = h;
        }

        function handleName() {
            const ne = G.ne; if (ne.done) return;
            const u = G.inp.u && !G.inp.up, d = G.inp.d && !G.inp.dp, l = G.inp.l && !G.inp.lp, f = G.inp.f && !G.inp.fp, r = G.inp.r && !G.inp.rp;
            if (u) ne.ch[ne.pos] = ne.ch[ne.pos] >= 90 ? 65 : ne.ch[ne.pos] + 1;
            if (d) ne.ch[ne.pos] = ne.ch[ne.pos] <= 65 ? 90 : ne.ch[ne.pos] - 1;
            if (l) ne.pos = Math.max(0, ne.pos - 1);
            if (r) ne.pos = Math.min(2, ne.pos + 1);
            overlayEl.querySelectorAll('[data-ci]').forEach((el, i) => { el.textContent = String.fromCharCode(ne.ch[i]); el.classList.toggle('active', i === ne.pos); });
            if (f) { if (ne.pos < 2) ne.pos++; else { ne.done = true; submitHS(String.fromCharCode(ne.ch[0], ne.ch[1], ne.ch[2]), G.score, G.stage); } }
        }

        function isHS(s) { return G.hiScores.length < 10 || s > G.hiScores[G.hiScores.length - 1].score; }
        async function loadHS() { try { const d = await api('/api/desktop/galaxa/highscore'); G.hiScores = Array.isArray(d) ? d : []; if (G.hiScores.length) G.hi = G.hiScores[0].score; } catch (e) {} }
        async function submitHS(name, score, stage) {
            try { const d = await api('/api/desktop/galaxa/highscore/submit', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, score, stage }) }); G.hiScores = Array.isArray(d) ? d : []; if (G.hiScores.length) G.hi = G.hiScores[0].score; } catch (e) {}
            overlayEl.classList.remove('active'); overlayEl.innerHTML = ''; G.st = 'TITLE'; G.tIdle = 0; showTitle();
        }

        function pollGP() {
            G.gp.l = false; G.gp.r = false; G.gp.u = false; G.gp.d = false; G.gp.f = false; G.gp.s = false; G.gp.p = false;
            try {
                const gps = navigator.getGamepads ? navigator.getGamepads() : [];
                for (const gp of gps) {
                    if (!gp) continue; const dz = 0.3, ax0 = gp.axes[0] || 0, ax1 = gp.axes[1] || 0;
                    if (ax0 < -dz || !!gp.buttons[14]?.pressed) G.gp.l = true;
                    if (ax0 > dz || !!gp.buttons[15]?.pressed) G.gp.r = true;
                    if (ax1 < -dz || !!gp.buttons[12]?.pressed) G.gp.u = true;
                    if (ax1 > dz || !!gp.buttons[13]?.pressed) G.gp.d = true;
                    if (!!gp.buttons[0]?.pressed) G.gp.f = true;
                    if (!!gp.buttons[9]?.pressed) G.gp.s = true;
                    if (!!gp.buttons[8]?.pressed) G.gp.p = true;
                    break;
                }
            } catch (e) {}
        }

        function mergeInput() {
            G.inp.l = G.kb.l || G.gp.l; G.inp.r = G.kb.r || G.gp.r;
            G.inp.u = G.kb.u || G.gp.u; G.inp.d = G.kb.d || G.gp.d;
            G.inp.f = G.kb.f || G.gp.f; G.inp.s = G.kb.s || G.gp.s; G.inp.p = G.kb.p || G.gp.p;
        }

        function onKey(e) {
            if (state.disposed) return; const k = e.key;
            if (k === 'ArrowLeft' || k === 'a') { G.kb.l = true; e.preventDefault(); }
            if (k === 'ArrowRight' || k === 'd') { G.kb.r = true; e.preventDefault(); }
            if (k === 'ArrowUp' || k === 'w') { G.kb.u = true; e.preventDefault(); }
            if (k === 'ArrowDown' || k === 's') { G.kb.d = true; e.preventDefault(); }
            if (k === ' ' || k === 'Enter') { G.kb.f = true; G.kb.s = true; e.preventDefault(); }
            if (k === 'Escape') { G.kb.p = true; e.preventDefault(); }
            if (k === 'm' || k === 'M') G.muted = !G.muted;
        }
        function onKeyUp(e) {
            const k = e.key;
            if (k === 'ArrowLeft' || k === 'a') G.kb.l = false;
            if (k === 'ArrowRight' || k === 'd') G.kb.r = false;
            if (k === 'ArrowUp' || k === 'w') G.kb.u = false;
            if (k === 'ArrowDown' || k === 's') G.kb.d = false;
            if (k === ' ' || k === 'Enter') { G.kb.f = false; G.kb.s = false; }
            if (k === 'Escape') G.kb.p = false;
        }

        function savePrev() { G.inp.fp = G.inp.f; G.inp.sp = G.inp.s; G.inp.pp = G.inp.p; G.inp.lp = G.inp.l; G.inp.rp = G.inp.r; G.inp.up = G.inp.u; G.inp.dp = G.inp.d; }

        function loop(now) {
            if (state.disposed) return;
            const dt = lastT ? Math.min((now - lastT) / 1000, 0.05) : 1 / 60;
            lastT = now;
            savePrev();
            pollGP();
            mergeInput();
            update(dt);
            renderFrame();
            rafId = requestAnimationFrame(loop);
        }

        document.addEventListener('keydown', onKey);
        document.addEventListener('keyup', onKeyUp);
        const ro = new ResizeObserver(() => { if (!state.disposed) resize(); });
        ro.observe(host);
        resize();
        loadHS().then(() => { showTitle(); rafId = requestAnimationFrame(loop); });

        state.dispose = function () {
            state.disposed = true; cancelAnimationFrame(rafId);
            document.removeEventListener('keydown', onKey); document.removeEventListener('keyup', onKeyUp);
            ro.disconnect(); if (actx) try { actx.close(); } catch (e) {}
            instances.delete(windowId);
        };
    }

    function dispose(windowId) { const s = instances.get(windowId); if (s && s.dispose) s.dispose(); }
    window.GalaxaDeluxe = { render, dispose };
})();
