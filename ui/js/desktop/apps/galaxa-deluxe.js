(function () {
    'use strict';
    const W = 480, H = 640, PLAYER_SPEED = 220, PB_SPEED = 500, EB_SPEED = 260;
    const FCOLS = 10, FROWS = 5, ESP_X = 36, ESP_Y = 32, FTOP = 60, DIVE_SPD = 180;
    const EXTRA_LIFE = 20000, TITLE_IDLE = 15000;
const PU_TYPES = ['rapid', 'spread', 'shield', 'bomb', 'speed', 'magnet', 'laser', 'multibomb', 'timeslow', 'pierce', 'homing', 'supernova', 'freeze'];
        const PU_COL = { rapid: '#00ffcc', spread: '#ff6600', shield: '#4488ff', bomb: '#ff4444', speed: '#ffee00', magnet: '#ff44ff', laser: '#eeeeff', multibomb: '#cc2222', timeslow: '#aa44ff', pierce: '#88ffaa', homing: '#ff88aa', supernova: '#ffffff', freeze: '#88eeff' };
        const PU_DUR = { rapid: 8000, spread: 10000, speed: 6000, magnet: 8000, laser: 5000, timeslow: 4000, pierce: 6000, homing: 0, freeze: 4000 };
        const PU_UPGRADE = { rapid: 'ultra_rapid', spread: 'mega_spread', speed: 'hyper_speed', magnet: 'super_magnet', laser: 'mega_laser', pierce: 'mega_pierce' };
        const PU_UPGRADE_COL = { ultra_rapid: '#00ffee', mega_spread: '#ff8800', hyper_speed: '#ffff44', super_magnet: '#ff88ff', mega_laser: '#ccddff', mega_pierce: '#aaffcc' };
        const PU_TRAIL_COL = { rapid: '0,255,204', ultra_rapid: '0,255,238', spread: '255,102,0', mega_spread: '255,136,0', shield: '68,136,255', speed: '255,238,0', hyper_speed: '255,255,68', magnet: '255,68,255', super_magnet: '255,136,255', laser: '180,200,255', mega_laser: '160,180,255', timeslow: '170,68,255', pierce: '136,255,170', mega_pierce: '170,255,204', homing: '255,136,170', freeze: '136,238,255' };
    const COMBO_TIMEOUT = 2000;
    const COMBO_THRESH = [2, 3, 5, 8];
    const COMBO_MULT = [1, 2, 4, 4, 8];
    const COMBO_TEXT = ['', 'DOUBLE KILL', 'TRIPLE KILL', 'RAMPAGE', 'UNSTOPPABLE'];
    const instances = new Map();

    function render(host, windowId, context) {
        if (!host) return;
        const ctx = context || {};
        const esc = ctx.esc || (v => String(v == null ? '' : v));
        const t = ctx.t || ((k, f) => f || k);
        const api = ctx.api || ((url, opts) => fetch(url, opts).then(r => r.json()));
        const state = { disposed: false };
        instances.set(windowId, state);

        host.innerHTML = '<div class="galaxa-app"><div class="galaxa-canvas-wrap galaxa-crt"><canvas class="galaxa-canvas" data-gc></canvas></div>' +
            '<div class="galaxa-overlay" data-go></div></div>';

        const canvas = host.querySelector('[data-gc]');
        const overlayEl = host.querySelector('[data-go]');
        const wrapEl = host.querySelector('.galaxa-canvas-wrap');
        const c = canvas.getContext('2d');
        c.imageSmoothingEnabled = false;
        let scale = 1, tick = 0, rafId = 0, lastT = 0;

        function loadSettings() {
            try { const s = JSON.parse(localStorage.getItem('galaxa_settings') || '{}'); return { vol: s.vol || 30, diff: s.diff || 'normal', mute: s.mute || false }; } catch (e) { return { vol: 30, diff: 'normal', mute: false }; }
        }
        function saveSettings() { try { localStorage.setItem('galaxa_settings', JSON.stringify(settings)); } catch (e) {} }
        let settings = loadSettings();

        function diffMod(key) {
            if (settings.diff === 'easy') return { diveRate: 0.7, ebSpd: 0.8, lives: 5, puFromBee: true }[key];
            if (settings.diff === 'hard') return { diveRate: 1.5, ebSpd: 1.3, lives: 2, puFromBee: false }[key];
            return { diveRate: 1, ebSpd: 1, lives: 3, puFromBee: true }[key];
        }

        function setPUClass(type) {
            const cls = ['galaxa-powerup-active'];
            for (const k of [...PU_TYPES, ...Object.keys(PU_UPGRADE)]) cls.push('galaxa-powerup-' + k);
            cls.forEach(c2 => wrapEl.classList.remove(c2));
            if (type) wrapEl.classList.add('galaxa-powerup-active', 'galaxa-powerup-' + type);
        }

        const G = {
            st: 'TITLE', score: 0, lives: 3, stage: 1, hi: 10000, hiScores: [],
            p: { x: W / 2, y: H - 50, alive: true, inv: 0, dual: false, cap: null, reviveTimer: 0 },
            bul: [], ebul: [], enemies: [], exp: [], part: [],
            fX: 0, fTmr: 0, dTmr: 0, sTmr: 0, tIdle: 0,
            attract: false, aTmr: 0, ne: { ch: [65, 65, 65], pos: 0, done: false },
            chal: false, chalHits: 0, chalTot: 0, beam: null, shkT: 0, shkM: 0,
            powerups: [], activePU: null, puTimer: 0, shieldHits: 0,
            scorePopups: [], flashT: 0, warpT: 0, warpFlash: 0, perfectT: 0, contTmr: 0, contCnt: 0,
            damageVignetteT: 0, freezeT: 0,
            pauseSel: 0, settingsSel: 0, settingsVolDrag: false,
            combo: 0, comboTimer: 0, comboMult: 1, comboBanner: null,
            trails: [],
            timeScale: 1, timeSlowTimer: 0,
            bossWarningT: 0, bossWarningShown: false,
            weaponLv: 1, killCount: 0, puUpgrade: null, upgradeBanner: null,
            slowMoT: 0, chromAb: 0, displayScore: 0, shipTilt: 0, muzzleT: 0, deathParts: [],
            beatPhase: 0, beatT: 0, plasmaRings: [], titleParts: [],
            inp: { l: false, r: false, f: false, fp: false, s: false, sp: false, p: false, pp: false, u: false, d: false, rp: false, lp: false, up: false, dp: false },
            kb: { l: false, r: false, u: false, d: false, f: false, s: false, p: false },
            gp: { l: false, r: false, u: false, d: false, f: false, s: false, p: false },
            muted: settings.mute, vol: settings.vol / 100, _prevSt: 'TITLE'
        };
        G.lives = diffMod('lives');

        let actx = null;
        function audio() {
            if (!actx) try { actx = new (window.AudioContext || window.webkitAudioContext)(); } catch (e) { return null; }
            if (actx && actx.state === 'suspended') actx.resume();
            return actx;
        }
        function beep(type, f0, f1, dur, vol, panX) {
            const a = audio(); if (!a || G.muted) return;
            const o = a.createOscillator(), g = a.createGain();
            o.type = type; o.frequency.setValueAtTime(f0, a.currentTime);
            if (f1 !== f0) o.frequency.linearRampToValueAtTime(f1, a.currentTime + dur);
            g.gain.setValueAtTime(G.vol * vol, a.currentTime);
            g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + dur + 0.02);
            if (panX !== undefined && a.createStereoPanner) {
                const p = a.createStereoPanner();
                p.pan.value = Math.max(-1, Math.min(1, (panX / (W / 2)) - 1));
                o.connect(g).connect(p).connect(a.destination);
            } else {
                o.connect(g).connect(a.destination);
            }
            o.start(); o.stop(a.currentTime + dur + 0.02);
        }
        function noise(dur, vol, freq, panX) {
            const a = audio(); if (!a || G.muted) return;
            const buf = a.createBuffer(1, a.sampleRate * dur, a.sampleRate), d = buf.getChannelData(0);
            for (let i = 0; i < d.length; i++) d[i] = (Math.random() * 2 - 1) * (1 - i / d.length);
            const s = a.createBufferSource(), f = a.createBiquadFilter(), g = a.createGain();
            s.buffer = buf; f.type = 'lowpass'; f.frequency.value = freq || 2000;
            g.gain.setValueAtTime(G.vol * vol, a.currentTime);
            g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + dur);
            if (panX !== undefined && a.createStereoPanner) {
                const p = a.createStereoPanner();
                p.pan.value = Math.max(-1, Math.min(1, (panX / (W / 2)) - 1));
                s.connect(f).connect(g).connect(p).connect(a.destination);
            } else {
                s.connect(f).connect(g).connect(a.destination);
            }
            s.start();
        }
        function schedNoise(startTime, dur, vol, freq, dest, panX) {
            const a = audio(); if (!a || G.muted) return null;
            const buf = a.createBuffer(1, Math.max(1, Math.floor(a.sampleRate * dur)), a.sampleRate), d = buf.getChannelData(0);
            for (let i = 0; i < d.length; i++) d[i] = (Math.random() * 2 - 1) * (1 - i / d.length);
            const s = a.createBufferSource(), f = a.createBiquadFilter(), g = a.createGain();
            s.buffer = buf; f.type = freq > 4000 ? 'highpass' : 'lowpass'; f.frequency.value = freq || 2000;
            g.gain.setValueAtTime(G.vol * vol, startTime);
            g.gain.exponentialRampToValueAtTime(0.001, startTime + dur);
            const target = dest || a.destination;
            if (panX !== undefined && a.createStereoPanner) {
                const p = a.createStereoPanner();
                p.pan.value = Math.max(-1, Math.min(1, (panX / (W / 2)) - 1));
                s.connect(f).connect(g).connect(p).connect(target);
            } else {
                s.connect(f).connect(g).connect(target);
            }
            s.start(startTime); s.stop(startTime + dur + 0.01);
            return s;
        }

        const SFX = {
            shoot(panX) { beep('sine', 800, 1200, 0.08, 0.3, panX); beep('square', 400, 200, 0.05, 0.08, panX); },
            laserShoot(panX) { beep('sine', 1200, 400, 0.15, 0.25, panX); beep('sawtooth', 800, 200, 0.1, 0.15, panX); noise(0.08, 0.1, 3000, panX); },
            dive(panX) { beep('sawtooth', 600, 200, 0.3, 0.15, panX); },
            eExplode(panX) { noise(0.15, 0.4, 2000, panX); noise(0.08, 0.2, 5000, panX); beep('sine', 200, 80, 0.1, 0.2, panX); beep('triangle', 60, 30, 0.15, 0.15, panX); },
            bigExplode(panX) { noise(0.3, 0.5, 1500, panX); noise(0.15, 0.3, 4000, panX); beep('sine', 80, 40, 0.25, 0.4, panX); noise(0.2, 0.2, 600, panX); },
            pExplode(panX) { noise(0.4, 0.6, 1200, panX); noise(0.2, 0.35, 3000, panX); beep('sine', 60, 60, 0.3, 0.5, panX); beep('sawtooth', 100, 30, 0.2, 0.3, panX); },
            stage() { [523, 659, 784, 1047].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.2, 0.25), i * 120); }); },
            challenge() { [440, 554, 659, 880, 1109, 1319].forEach((f, i) => { setTimeout(() => beep('square', f, f, 0.15, 0.2), i * 80); }); },
            extra() { beep('sine', 1200, 1200, 0.2, 0.3); },
            rescue() { beep('sine', 880, 880, 0.2, 0.25); setTimeout(() => beep('sine', 1100, 1100, 0.2, 0.25), 100); },
            beam() { beep('sawtooth', 200, 200, 0.5, 0.15); },
            perfect() { [523, 659, 784, 1047, 1319, 1568].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.15, 0.3), i * 60); }); },
            puCollect(panX) { [600, 800, 1000, 1200].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.06, 0.2, panX), i * 40); }); },
            bomb(panX) { noise(0.5, 0.7, 800, panX); noise(0.3, 0.4, 200, panX); beep('sawtooth', 100, 50, 0.4, 0.5, panX); },
            combo(n) { beep('sine', 440 + n * 110, 440 + n * 110, 0.12, 0.25 + n * 0.05); },
            bossWarning() { beep('sawtooth', 440, 220, 0.5, 0.3); setTimeout(() => beep('sawtooth', 440, 220, 0.5, 0.3), 500); },
            shieldHit() { beep('triangle', 2000, 4000, 0.05, 0.3); beep('sine', 3000, 1500, 0.08, 0.2); },
            respawn() { beep('sine', 200, 800, 0.3, 0.25); setTimeout(() => beep('sine', 600, 1200, 0.2, 0.2), 80); },
            shieldBreak() { noise(0.2, 0.5, 3000); beep('sawtooth', 200, 100, 0.15, 0.4); },
            bossJingle() { [220, 262, 330, 220, 165, 220].forEach((f, i) => { setTimeout(() => beep('sawtooth', f, f, 0.15, 0.2 + i * 0.02), i * 100); }); },
            stageClear() { [523, 659, 784, 1047, 1319, 1568, 2093].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.15, 0.2), i * 80); }); },
            puUpgrade(panX) { [800, 1000, 1200, 1400, 1600].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.05, 0.25, panX), i * 30); }); },
            weaponUp() { [600, 800, 1000, 1200].forEach((f, i) => { setTimeout(() => beep('triangle', f, f, 0.08, 0.2), i * 60); }); },
            homingLock(panX) { beep('sine', 1200, 1200, 0.04, 0.15, panX); beep('sine', 1800, 1800, 0.03, 0.1, panX); },
            supernova(panX) { noise(0.8, 0.9, 600, panX); noise(0.5, 0.5, 100, panX); beep('sawtooth', 80, 40, 0.6, 0.7, panX); beep('sine', 200, 50, 0.5, 0.5, panX); },
            miniBossWarning() { beep('sawtooth', 330, 165, 0.4, 0.3); setTimeout(() => beep('sawtooth', 330, 165, 0.4, 0.3), 400); },
            bossHitSFX(panX) { beep('sawtooth', 280, 60, 0.12, 0.45, panX); noise(0.1, 0.35, 900, panX); },
            warpJump() { beep('sawtooth', 180, 3600, 0.35, 0.45); beep('sine', 90, 3000, 0.28, 0.35); setTimeout(() => noise(0.15, 0.3, 4000), 250); },
            coinInsert() { beep('triangle', 440, 880, 0.06, 0.45); setTimeout(() => beep('triangle', 880, 1760, 0.06, 0.45), 70); },
            comboBreak() { beep('sawtooth', 440, 200, 0.18, 0.2); },
            killStreak() { [880, 1100, 1320, 1760].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.09, 0.28), i * 55); }); },
            freeze(panX) { beep('sine', 1200, 3600, 0.06, 0.35, panX); beep('triangle', 900, 2800, 0.05, 0.28, panX); setTimeout(() => { beep('triangle', 400, 180, 0.09, 0.15, panX); noise(0.12, 0.18, 7000, panX); }, 100); },
            powerupExpire() { beep('sawtooth', 880, 440, 0.07, 0.4); },
            enemyHitSfx(panX) { beep('sine', 380, 180, 0.03, 0.25, panX); beep('sine', 550, 300, 0.02, 0.15, panX); }
        };

        const MusicEngine = {
            nodes: [], masterGain: null, playing: null, loopId: 0, tempoMult: 1, stopped: false,
themes: {
                title: {
                    bpm: 120,
                    bass: { wave: 'triangle', vol: 0.06, notes: [{ f: 131, d: 2 }, { f: 0, d: 2 }, { f: 156, d: 2 }, { f: 0, d: 2 }, { f: 131, d: 2 }, { f: 0, d: 1 }, { f: 117, d: 1 }, { f: 0, d: 2 }, { f: 156, d: 2 }, { f: 0, d: 1 }, { f: 131, d: 1 }, { f: 0, d: 2 }] },
                    lead: { wave: 'sine', vol: 0.08, notes: [{ f: 262, d: 1 }, { f: 233, d: 1 }, { f: 311, d: 1 }, { f: 294, d: 1 }, { f: 262, d: 2 }, { f: 233, d: 2 }, { f: 349, d: 1 }, { f: 311, d: 1 }, { f: 294, d: 1 }, { f: 262, d: 1 }, { f: 233, d: 2 }, { f: 262, d: 2 }] },
                    harmony: { wave: 'sine', vol: 0.04, notes: [{ f: 311, d: 2 }, { f: 349, d: 2 }, { f: 262, d: 2 }, { f: 294, d: 2 }, { f: 349, d: 2 }, { f: 311, d: 2 }, { f: 262, d: 2 }, { f: 233, d: 2 }] },
                    arpeggio: { wave: 'square', vol: 0.02, notes: [{ f: 262, d: 0.5 }, { f: 311, d: 0.5 }, { f: 349, d: 0.5 }, { f: 262, d: 0.5 }, { f: 233, d: 0.5 }, { f: 294, d: 0.5 }, { f: 349, d: 0.5 }, { f: 233, d: 0.5 }, { f: 262, d: 0.5 }, { f: 311, d: 0.5 }, { f: 349, d: 0.5 }, { f: 262, d: 0.5 }, { f: 233, d: 0.5 }, { f: 294, d: 0.5 }, { f: 349, d: 0.5 }, { f: 233, d: 0.5 }] },
                    percussion: { vol: 0.04, notes: [{ f: -1, d: 1 }, { f: 0, d: 1 }, { f: -2, d: 0.5 }, { f: 0, d: 0.5 }, { f: -1, d: 1 }, { f: 0, d: 1 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -1, d: 1 }, { f: 0, d: 1 }, { f: -2, d: 0.5 }, { f: 0, d: 0.5 }, { f: -1, d: 1 }, { f: 0, d: 1 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }] }
                },
                gameplay: {
                    bpm: 140,
                    bass: { wave: 'triangle', vol: 0.07, notes: [{ f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 0, d: 0.5 }, { f: 156, d: 0.5 }, { f: 156, d: 0.5 }, { f: 156, d: 0.5 }, { f: 0, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 0, d: 0.5 }, { f: 117, d: 0.5 }, { f: 117, d: 0.5 }, { f: 117, d: 0.5 }, { f: 0, d: 0.5 }, { f: 131, d: 0.5 }, { f: 156, d: 0.5 }, { f: 175, d: 0.5 }, { f: 0, d: 0.5 }, { f: 156, d: 0.5 }, { f: 175, d: 0.5 }, { f: 196, d: 0.5 }, { f: 0, d: 0.5 }, { f: 131, d: 0.5 }, { f: 147, d: 0.5 }, { f: 175, d: 0.5 }, { f: 0, d: 0.5 }, { f: 117, d: 0.5 }, { f: 131, d: 0.5 }, { f: 156, d: 0.5 }, { f: 0, d: 0.5 }] },
                    lead: { wave: 'square', vol: 0.05, notes: [{ f: 262, d: 0.5 }, { f: 311, d: 0.5 }, { f: 392, d: 0.5 }, { f: 262, d: 0.5 }, { f: 233, d: 0.5 }, { f: 294, d: 0.5 }, { f: 349, d: 0.5 }, { f: 233, d: 0.5 }, { f: 207, d: 0.5 }, { f: 262, d: 0.5 }, { f: 311, d: 0.5 }, { f: 207, d: 0.5 }, { f: 196, d: 0.5 }, { f: 233, d: 0.5 }, { f: 294, d: 0.5 }, { f: 196, d: 0.5 }, { f: 349, d: 0.5 }, { f: 392, d: 0.5 }, { f: 440, d: 1 }, { f: 392, d: 0.5 }, { f: 349, d: 0.5 }, { f: 440, d: 1 }, { f: 392, d: 0.5 }, { f: 349, d: 0.5 }, { f: 294, d: 0.5 }, { f: 349, d: 0.5 }, { f: 392, d: 1 }, { f: 294, d: 0.5 }, { f: 233, d: 0.5 }, { f: 262, d: 1 }, { f: 233, d: 0.5 }, { f: 196, d: 0.5 }] },
                    harmony: { wave: 'sine', vol: 0.03, notes: [{ f: 262, d: 1 }, { f: 311, d: 1 }, { f: 233, d: 1 }, { f: 294, d: 1 }, { f: 207, d: 1 }, { f: 262, d: 1 }, { f: 196, d: 1 }, { f: 233, d: 1 }, { f: 349, d: 1 }, { f: 392, d: 1 }, { f: 440, d: 1 }, { f: 392, d: 1 }, { f: 294, d: 1 }, { f: 349, d: 1 }, { f: 262, d: 1 }, { f: 233, d: 1 }] },
                    arpeggio: { wave: 'sine', vol: 0.02, notes: [{ f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 156, d: 0.25 }, { f: 233, d: 0.25 }, { f: 311, d: 0.25 }, { f: 233, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 117, d: 0.25 }, { f: 175, d: 0.25 }, { f: 233, d: 0.25 }, { f: 175, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 156, d: 0.25 }, { f: 233, d: 0.25 }, { f: 311, d: 0.25 }, { f: 233, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 117, d: 0.25 }, { f: 175, d: 0.25 }, { f: 233, d: 0.25 }, { f: 175, d: 0.25 }] },
                    percussion: { vol: 0.04, notes: [{ f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -2, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -2, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -2, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -2, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }] }
                },
                boss: {
                    bpm: 160,
                    bass: { wave: 'sawtooth', vol: 0.05, notes: [{ f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 110, d: 1 }, { f: 123, d: 1 }, { f: 131, d: 1 }, { f: 147, d: 1 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }] },
                    lead: { wave: 'square', vol: 0.04, notes: [{ f: 220, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 220, d: 0.5 }, { f: 247, d: 0.5 }, { f: 294, d: 0.5 }, { f: 330, d: 0.5 }, { f: 247, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 440, d: 0.5 }, { f: 262, d: 0.5 }, { f: 247, d: 0.5 }, { f: 294, d: 0.5 }, { f: 330, d: 0.5 }, { f: 247, d: 0.5 }, { f: 440, d: 0.5 }, { f: 523, d: 0.5 }, { f: 440, d: 0.5 }, { f: 330, d: 0.5 }, { f: 294, d: 0.5 }, { f: 440, d: 0.5 }, { f: 523, d: 0.5 }, { f: 440, d: 0.5 }, { f: 330, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 220, d: 0.5 }, { f: 262, d: 1 }, { f: 330, d: 1 }, { f: 220, d: 1 }, { f: 247, d: 1 }] },
                    harmony: { wave: 'sine', vol: 0.03, notes: [{ f: 165, d: 1 }, { f: 196, d: 1 }, { f: 220, d: 1 }, { f: 247, d: 1 }, { f: 262, d: 1 }, { f: 165, d: 1 }, { f: 247, d: 1 }, { f: 294, d: 1 }, { f: 220, d: 1 }, { f: 262, d: 1 }, { f: 330, d: 1 }, { f: 440, d: 1 }, { f: 294, d: 1 }, { f: 330, d: 1 }, { f: 220, d: 1 }, { f: 247, d: 1 }] },
                    arpeggio: { wave: 'sawtooth', vol: 0.02, notes: [{ f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }, { f: 123, d: 0.25 }, { f: 185, d: 0.25 }, { f: 247, d: 0.25 }, { f: 185, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 123, d: 0.25 }, { f: 185, d: 0.25 }, { f: 247, d: 0.25 }, { f: 185, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }, { f: 123, d: 0.25 }, { f: 185, d: 0.25 }, { f: 247, d: 0.25 }, { f: 185, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 123, d: 0.25 }, { f: 185, d: 0.25 }, { f: 247, d: 0.25 }, { f: 185, d: 0.25 }] },
                    percussion: { vol: 0.05, notes: [{ f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -3, d: 0.25 }, { f: -2, d: 0.25 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }] }
                },
                miniboss: {
                    bpm: 150,
                    bass: { wave: 'sawtooth', vol: 0.06, notes: [{ f: 147, d: 0.5 }, { f: 147, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 123, d: 0.5 }, { f: 123, d: 0.5 }, { f: 147, d: 0.5 }, { f: 147, d: 0.5 }, { f: 175, d: 0.5 }, { f: 175, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }] },
                    lead: { wave: 'square', vol: 0.04, notes: [{ f: 294, d: 0.5 }, { f: 349, d: 0.5 }, { f: 440, d: 0.5 }, { f: 294, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 392, d: 0.5 }, { f: 262, d: 0.5 }, { f: 349, d: 0.5 }, { f: 440, d: 0.5 }, { f: 523, d: 0.5 }, { f: 349, d: 0.5 }, { f: 330, d: 0.5 }, { f: 392, d: 0.5 }, { f: 440, d: 0.5 }, { f: 330, d: 0.5 }] },
                    harmony: { wave: 'sine', vol: 0.03, notes: [{ f: 220, d: 1 }, { f: 262, d: 1 }, { f: 294, d: 1 }, { f: 330, d: 1 }, { f: 349, d: 1 }, { f: 262, d: 1 }, { f: 294, d: 1 }, { f: 220, d: 1 }] },
                    arpeggio: { wave: 'sawtooth', vol: 0.015, notes: [{ f: 147, d: 0.25 }, { f: 220, d: 0.25 }, { f: 294, d: 0.25 }, { f: 220, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }, { f: 123, d: 0.25 }, { f: 185, d: 0.25 }, { f: 247, d: 0.25 }, { f: 185, d: 0.25 }, { f: 147, d: 0.25 }, { f: 220, d: 0.25 }, { f: 294, d: 0.25 }, { f: 220, d: 0.25 }, { f: 175, d: 0.25 }, { f: 262, d: 0.25 }, { f: 349, d: 0.25 }, { f: 262, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }] },
                    percussion: { vol: 0.05, notes: [{ f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }] }
                },
                gameover: {
                    bpm: 100,
                    bass: { wave: 'triangle', vol: 0.06, notes: [{ f: 131, d: 1 }, { f: 117, d: 1 }, { f: 104, d: 1 }, { f: 98, d: 1 }, { f: 87, d: 1 }, { f: 78, d: 2 }, { f: 131, d: 0.5 }, { f: 0, d: 0.5 }, { f: 117, d: 0.5 }, { f: 0, d: 0.5 }, { f: 104, d: 1 }, { f: 78, d: 2 }] },
                    lead: { wave: 'sine', vol: 0.1, notes: [{ f: 262, d: 1 }, { f: 233, d: 1 }, { f: 207, d: 1 }, { f: 196, d: 1 }, { f: 175, d: 1 }, { f: 156, d: 2 }, { f: 262, d: 0.5 }, { f: 233, d: 0.5 }, { f: 207, d: 0.5 }, { f: 196, d: 0.5 }, { f: 175, d: 1 }, { f: 156, d: 2 }] },
                    harmony: { wave: 'sine', vol: 0.04, notes: [{ f: 311, d: 1 }, { f: 294, d: 1 }, { f: 262, d: 1 }, { f: 233, d: 1 }, { f: 207, d: 1 }, { f: 0, d: 2 }, { f: 311, d: 1 }, { f: 294, d: 1 }, { f: 262, d: 1 }, { f: 233, d: 1 }, { f: 207, d: 1 }, { f: 0, d: 2 }] },
                    percussion: { vol: 0.03, notes: [{ f: -1, d: 1 }, { f: 0, d: 2 }, { f: -1, d: 1 }, { f: 0, d: 3 }, { f: -1, d: 1 }, { f: 0, d: 5 }] }
                },
                challenge: {
                    bpm: 170,
                    bass: { wave: 'sawtooth', vol: 0.06, notes: [{ f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 147, d: 0.5 }, { f: 147, d: 0.5 }, { f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 147, d: 0.5 }, { f: 147, d: 0.5 }, { f: 165, d: 0.5 }, { f: 165, d: 0.5 }, { f: 98, d: 0.25 }, { f: 131, d: 0.25 }, { f: 98, d: 0.25 }, { f: 131, d: 0.25 }, { f: 110, d: 0.25 }, { f: 147, d: 0.25 }, { f: 110, d: 0.25 }, { f: 147, d: 0.25 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 165, d: 0.5 }, { f: 165, d: 0.5 }, { f: 147, d: 0.5 }, { f: 147, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }] },
                    lead: { wave: 'square', vol: 0.05, notes: [{ f: 196, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 392, d: 0.5 }, { f: 440, d: 0.5 }, { f: 392, d: 0.5 }, { f: 330, d: 0.5 }, { f: 262, d: 0.5 }, { f: 220, d: 0.5 }, { f: 294, d: 0.5 }, { f: 349, d: 0.5 }, { f: 440, d: 0.5 }, { f: 523, d: 0.5 }, { f: 440, d: 0.5 }, { f: 349, d: 0.5 }, { f: 294, d: 0.5 }, { f: 262, d: 0.25 }, { f: 330, d: 0.25 }, { f: 392, d: 0.25 }, { f: 523, d: 0.25 }, { f: 440, d: 0.5 }, { f: 523, d: 0.5 }, { f: 659, d: 0.5 }, { f: 523, d: 0.5 }, { f: 440, d: 0.5 }, { f: 349, d: 0.5 }, { f: 294, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 392, d: 0.5 }, { f: 440, d: 0.5 }, { f: 523, d: 0.5 }] },
                    harmony: { wave: 'sine', vol: 0.03, notes: [{ f: 196, d: 1 }, { f: 262, d: 1 }, { f: 330, d: 1 }, { f: 392, d: 1 }, { f: 440, d: 1 }, { f: 349, d: 1 }, { f: 294, d: 1 }, { f: 262, d: 1 }, { f: 330, d: 1 }, { f: 392, d: 1 }, { f: 440, d: 1 }, { f: 523, d: 1 }, { f: 659, d: 1 }, { f: 523, d: 1 }, { f: 440, d: 1 }, { f: 349, d: 1 }] },
                    arpeggio: { wave: 'square', vol: 0.02, notes: [{ f: 98, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 110, d: 0.25 }, { f: 147, d: 0.25 }, { f: 220, d: 0.25 }, { f: 294, d: 0.25 }, { f: 131, d: 0.25 }, { f: 165, d: 0.25 }, { f: 262, d: 0.25 }, { f: 330, d: 0.25 }, { f: 147, d: 0.25 }, { f: 196, d: 0.25 }, { f: 294, d: 0.25 }, { f: 392, d: 0.25 }, { f: 98, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 110, d: 0.25 }, { f: 147, d: 0.25 }, { f: 220, d: 0.25 }, { f: 294, d: 0.25 }, { f: 131, d: 0.25 }, { f: 165, d: 0.25 }, { f: 262, d: 0.25 }, { f: 330, d: 0.25 }, { f: 147, d: 0.25 }, { f: 196, d: 0.25 }, { f: 294, d: 0.25 }, { f: 392, d: 0.25 }] },
                    percussion: { vol: 0.05, notes: [{ f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -3, d: 0.25 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -3, d: 0.25 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }] }
                },
                victory: {
                    bpm: 180,
                    bass: { wave: 'triangle', vol: 0.08, notes: [{f:131,d:0.5},{f:0,d:0.5},{f:165,d:0.5},{f:0,d:0.5},{f:196,d:0.5},{f:0,d:0.5},{f:262,d:1},{f:220,d:0.5},{f:0,d:0.5},{f:262,d:0.5},{f:0,d:0.5},{f:330,d:0.5},{f:0,d:0.5},{f:392,d:1}] },
                    lead: { wave: 'sine', vol: 0.14, notes: [{f:523,d:0.5},{f:659,d:0.5},{f:784,d:0.5},{f:1047,d:1.5},{f:880,d:0.5},{f:1047,d:0.5},{f:1175,d:0.5},{f:1397,d:1.5}] },
                    harmony: { wave: 'sine', vol: 0.07, notes: [{f:392,d:1},{f:494,d:1},{f:587,d:1},{f:784,d:2},{f:659,d:1},{f:784,d:1},{f:880,d:1},{f:1047,d:2}] },
                    arpeggio: { wave: 'triangle', vol: 0.04, notes: [{f:262,d:0.25},{f:330,d:0.25},{f:392,d:0.25},{f:523,d:0.25},{f:330,d:0.25},{f:392,d:0.25},{f:523,d:0.25},{f:659,d:0.25},{f:440,d:0.25},{f:523,d:0.25},{f:659,d:0.25},{f:880,d:0.25},{f:523,d:0.25},{f:659,d:0.25},{f:880,d:0.25},{f:1047,d:0.25}] },
                    percussion: { vol: 0.07, notes: [{f:-1,d:0.5},{f:-2,d:0.5},{f:-2,d:0.5},{f:-1,d:0.5},{f:-2,d:0.5},{f:-1,d:0.5},{f:-3,d:0.5},{f:-2,d:0.5},{f:-1,d:0.5},{f:-2,d:0.5},{f:-2,d:0.5},{f:-1,d:0.5},{f:-3,d:0.5},{f:-1,d:0.5},{f:-2,d:0.5},{f:-1,d:0.5}] }
                }
            },
            play(theme) {
                if (this.playing === theme && !this.stopped) return;
                this.stop(); this.playing = theme; this.stopped = false;
                const a = audio(); if (!a) return;
                this.masterGain = a.createGain();
                this.masterGain.gain.value = G.muted ? 0 : G.vol * 0.35;
                this.masterGain.connect(a.destination);
                const th = this.themes[theme]; if (!th) return;
                const beatDur = (60 / th.bpm) / this.tempoMult;
                const loop = theme !== 'gameover' && theme !== 'victory';
                const schedVoices = () => {
                    if (this.stopped || !this.masterGain) return;
                    this.nodes = [];
                    let maxDur = 0;
                    for (const vn of ['bass', 'lead', 'harmony', 'arpeggio']) {
                        const voice = th[vn]; if (!voice) continue;
                        let offset = 0;
                        for (const n of voice.notes) {
                            if (n.f > 0) {
                                const o = a.createOscillator(), g = a.createGain();
                                o.type = voice.wave; o.frequency.value = n.f;
                                g.gain.setValueAtTime(voice.vol * (G.muted ? 0 : 1), a.currentTime + offset);
                                g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + offset + n.d * beatDur + 0.01);
                                o.connect(g).connect(this.masterGain);
                                o.start(a.currentTime + offset); o.stop(a.currentTime + offset + n.d * beatDur + 0.02);
                                this.nodes.push(o);
                            }
                            offset += n.d * beatDur;
                        }
                        maxDur = Math.max(maxDur, offset);
                    }
                    if (th.percussion) {
                        let offset = 0;
                        for (const n of th.percussion.notes) {
                            if (n.f === -1) {
                                // Enhanced kick drum with pitch drop
                                const o = a.createOscillator(), g = a.createGain();
                                o.frequency.setValueAtTime(150, a.currentTime + offset);
                                o.frequency.exponentialRampToValueAtTime(40, a.currentTime + offset + 0.08);
                                g.gain.setValueAtTime(th.percussion.vol * 1.8, a.currentTime + offset);
                                g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + offset + 0.15);
                                o.connect(g).connect(this.masterGain);
                                o.start(a.currentTime + offset); o.stop(a.currentTime + offset + 0.16);
                                this.nodes.push(o);
                                const ns = schedNoise(a.currentTime + offset, 0.06, th.percussion.vol * 0.6, 120, this.masterGain);
                                if (ns) this.nodes.push(ns);
                            }
                            else if (n.f === -2) {
                                // Enhanced snare/hihat
                                const ns = schedNoise(a.currentTime + offset, 0.04, th.percussion.vol * 0.7, 9000, this.masterGain);
                                if (ns) this.nodes.push(ns);
                                const ns2 = schedNoise(a.currentTime + offset, 0.02, th.percussion.vol * 0.3, 3000, this.masterGain);
                                if (ns2) this.nodes.push(ns2);
                            }
                            else if (n.f === -3) {
                                // Enhanced tom/crash
                                const ns = schedNoise(a.currentTime + offset, 0.07, th.percussion.vol * 0.8, 2500, this.masterGain);
                                if (ns) this.nodes.push(ns);
                                const o = a.createOscillator(), g = a.createGain();
                                o.frequency.setValueAtTime(200, a.currentTime + offset);
                                o.frequency.exponentialRampToValueAtTime(80, a.currentTime + offset + 0.1);
                                g.gain.setValueAtTime(th.percussion.vol * 0.5, a.currentTime + offset);
                                g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + offset + 0.12);
                                o.connect(g).connect(this.masterGain);
                                o.start(a.currentTime + offset); o.stop(a.currentTime + offset + 0.13);
                                this.nodes.push(o);
                            }
                            offset += n.d * beatDur;
                        }
                        maxDur = Math.max(maxDur, offset);
                    }
                    if (loop) { this.loopId = setTimeout(() => { schedVoices(); }, maxDur * 1000 + 50); }
                };
                schedVoices();
            },
            stop() { this.stopped = true; clearTimeout(this.loopId); for (const n of this.nodes) try { n.stop(); } catch (e) {} this.nodes = []; if (this.masterGain) try { this.masterGain.disconnect(); } catch (e) {} this.playing = null; },
            setTempo(mult) { this.tempoMult = mult; if (this.playing) this.play(this.playing); },
            setMuted(m) { if (this.masterGain) this.masterGain.gain.value = m ? 0 : G.vol * 0.35; },
            setIntensity(level) {
                const volMult = 1 + Math.min(level, 5) * 0.08;
                const tempoMult = this.tempoMult;
                if (this.masterGain) this.masterGain.gain.value = G.muted ? 0 : G.vol * 0.35 * volMult;
            }
        };

        const PTS = { bee: [50, 100], butterfly: [80, 160], boss: [400, 800], miniboss: [600, 1200] };
        const SP = buildSprites();
        const STARS = [];
        const STAR_COLS = ['#ffffff', '#ffeecc', '#ccddff', '#ffcccc', '#ccffcc'];
        for (let i = 0; i < 60; i++) STARS.push({ x: Math.random() * W, y: Math.random() * H, sp: 10 + Math.random() * 20, br: 0.15 + Math.random() * 0.3, sz: 1, layer: 0, col: STAR_COLS[Math.floor(Math.random() * STAR_COLS.length)] });
        for (let i = 0; i < 45; i++) STARS.push({ x: Math.random() * W, y: Math.random() * H, sp: 30 + Math.random() * 30, br: 0.3 + Math.random() * 0.4, sz: Math.random() > 0.7 ? 2 : 1, layer: 1, col: STAR_COLS[Math.floor(Math.random() * STAR_COLS.length)] });
        for (let i = 0; i < 30; i++) STARS.push({ x: Math.random() * W, y: Math.random() * H, sp: 60 + Math.random() * 60, br: 0.5 + Math.random() * 0.5, sz: 2, layer: 2, col: STAR_COLS[Math.floor(Math.random() * STAR_COLS.length)] });
        for (let i = 0; i < 15; i++) STARS.push({ x: Math.random() * W, y: Math.random() * H, sp: 100 + Math.random() * 80, br: 0.7 + Math.random() * 0.3, sz: 2, layer: 3, twinkle: Math.random() * 6.28, col: '#ffffff' });
        let shootingStars = [];
        let nebulaCv = null, nebulaColors = [];
        const radialGradientCache = new Map();
        const spritePixelCache = new WeakMap();
        const flashPixelColors = {};
        let bgPlanets = [], bgComets = [];
        function initBG() {
            bgPlanets = [];
            const themes = ['nebula', 'asteroid', 'blackhole', 'ringed', 'storm', 'crystal'];
            const theme = themes[(G.stage - 1) % themes.length];
            if (theme === 'asteroid') {
                for (let i = 0; i < 12; i++) bgPlanets.push({ x: Math.random() * W, y: Math.random() * H, r: 3 + Math.random() * 10, sp: 6 + Math.random() * 14, col: ['#554433', '#665544', '#443322', '#776655'][Math.floor(Math.random()*4)], type: 'asteroid', rot: Math.random()*6.28 });
            } else if (theme === 'blackhole') {
                bgPlanets.push({ x: W / 2, y: H / 3, r: 35, sp: 0, col: '#110022', type: 'blackhole', rotSp: 0.02 });
                for (let i = 0; i < 16; i++) bgPlanets.push({ x: W / 2 + (Math.random() - 0.5) * 220, y: H / 3 + (Math.random() - 0.5) * 220, r: 1.5 + Math.random() * 3.5, sp: 12 + Math.random() * 22, col: ['#443366', '#553377', '#332255'][Math.floor(Math.random()*3)], type: 'debris', orbit: Math.random() * 6.28, orbitR: 50 + Math.random() * 90, orbitSp: 0.4 + Math.random() * 1.2 });
            } else if (theme === 'ringed') {
                for (let i = 0; i < 2; i++) bgPlanets.push({ x: Math.random() * W, y: Math.random() * H, r: 10 + Math.random() * 12, sp: 2 + Math.random() * 3, col: i === 0 ? '#334466' : '#664433', type: 'planet', atmoCol: i === 0 ? 'rgba(100,160,255,0.12)' : 'rgba(255,160,100,0.12)', ringCol: i === 0 ? 'rgba(180,200,255,0.08)' : 'rgba(255,200,150,0.08)', ringR: 1.6 + Math.random() * 0.4 });
            } else if (theme === 'storm') {
                for (let i = 0; i < 2; i++) bgPlanets.push({ x: Math.random() * W, y: Math.random() * H, r: 14 + Math.random() * 10, sp: 1 + Math.random() * 2, col: i === 0 ? '#443322' : '#2a3a2a', type: 'planet', atmoCol: i === 0 ? 'rgba(200,150,80,0.1)' : 'rgba(100,200,120,0.08)', storm: true, stormCol: i === 0 ? 'rgba(180,120,40,0.06)' : 'rgba(80,180,100,0.06)' });
                for (let i = 0; i < 5; i++) bgPlanets.push({ x: Math.random() * W, y: Math.random() * H, r: 2 + Math.random() * 4, sp: 20 + Math.random() * 30, col: '#554433', type: 'asteroid' });
            } else if (theme === 'crystal') {
                for (let i = 0; i < 6; i++) bgPlanets.push({ x: Math.random() * W, y: Math.random() * H, r: 3 + Math.random() * 6, sp: 4 + Math.random() * 8, col: ['#446688', '#664488', '#448866'][i % 3], type: 'crystal', crystalCol: ['#88ccff', '#ff88cc', '#88ffcc'][i % 3] });
            } else {
                for (let i = 0; i < 4; i++) bgPlanets.push({ x: Math.random() * W, y: Math.random() * H, r: 6 + Math.random() * 14, sp: 2 + Math.random() * 4, col: ['#224466', '#446622', '#662244', '#443366'][i % 4], type: 'planet', atmoCol: ['rgba(68,136,255,0.12)', 'rgba(136,255,68,0.08)', 'rgba(255,136,68,0.1)', 'rgba(180,100,255,0.08)'][i % 4] });
            }
            bgComets = [];
        }
        initBG();

        function mkNebula() {
            const cols = [
                ['#1a0033', '#0d1a2e', '#0a2218'], ['#2a0a1a', '#1a1a3e', '#0d2a22'],
                ['#3a1a0a', '#1a2a3a', '#0a3a2a'], ['#0a1a3a', '#2a0a2a', '#1a3a0a'],
                ['#1a2a1a', '#3a0a1a', '#0a0a3a'], ['#1a0a2a', '#0a2a1a', '#2a1a0a'],
                ['#0a2a2a', '#1a0a3a', '#2a2a0a']
            ];
            nebulaColors = cols[(G.stage - 1) % cols.length];
            nebulaCv = ensureNebulaCanvas();
            const nc = nebulaCv.getContext('2d');
            nc.clearRect(0, 0, W, H);
            for (let i = 0; i < 4; i++) {
                const cx = W * (0.15 + i * 0.22 + Math.random() * 0.1), cy = H * (0.2 + i * 0.18 + Math.random() * 0.1), r = 100 + i * 35 + Math.random() * 30;
                const gr = nc.createRadialGradient(cx, cy, 0, cx, cy, r);
                gr.addColorStop(0, nebulaColors[i % 3]); gr.addColorStop(0.6, nebulaColors[i % 3] + '66'); gr.addColorStop(1, 'transparent');
                nc.fillStyle = gr; nc.fillRect(0, 0, W, H);
            }
        }

        function ensureNebulaCanvas() {
            if (!nebulaCv) {
                nebulaCv = document.createElement('canvas');
            }
            if (nebulaCv.width !== W) nebulaCv.width = W;
            if (nebulaCv.height !== H) nebulaCv.height = H;
            return nebulaCv;
        }

        function cachedRadialGradient(ctx, key, x, y, innerR, outerR, stops) {
            const cacheKey = [
                key,
                Math.round(x),
                Math.round(y),
                Math.round(innerR),
                Math.round(outerR),
                stops.map(stop => stop.join('@')).join('|')
            ].join(':');
            if (radialGradientCache.has(cacheKey)) return radialGradientCache.get(cacheKey);
            const gradient = ctx.createRadialGradient(x, y, innerR, x, y, outerR);
            stops.forEach(([offset, color]) => gradient.addColorStop(offset, color));
            radialGradientCache.set(cacheKey, gradient);
            return gradient;
        }

        function buildSprites() {
            const p = (s) => { const rows = s.trim().split('\n'); return rows.map(r => r.split('').map(ch => parseInt(ch, 16) || 0)); };
            return {
                player: p([
                    '000000000007700000000000', '000000000177710000000000', '000000001727721000000000', '000000017727771000000000',
                    '000000177766771000000000', '000001777666777100000000', '000017772666277710000000', '000177723366332771000000',
                    '001777233333333277100000', '017772333333333327710000', '177723334444433327771100', 'a7723334444444333277a000',
                    'a7233444444444443337a000', '072334444334444433270000', '072344443333344443200000', '002344433333333443200000',
                    '000123433333333432100000', '000012333333333321000000', '000000155555555100000000', '000000055555555000000000',
                    '000000055505555000000000', '000000005505500000000000', '000000000550500000000000', '000000000000000000000000'
                ].join('\n')),
                pC: { 1: '#ffffff', 2: '#88ccff', 3: '#4488ff', 4: '#2266cc', 5: '#ff8800', 6: '#44ffaa', 7: '#cceeff', a: '#ff5544' },
                bee: [
                    p([
                        '00000000444000000000', '00000004455400000000', '00000044555440000000', '00000445555444000000',
                        '00004445654444000000', '00044444664444400000', '00444444444444400000', '06444444444444460000',
                        '66444444444444660000', '00664444444446600000', '00006444444600000000', '00006444444600000000',
                        '00000440044000000000', '00000040004000000000', '00000000000000000000', '00000000000000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000444000000000', '00000004455400000000', '00000044555440000000', '00000445555444000000',
                        '00004445654444000000', '00444444664444400000', '04444444444444440000', '46444444444444460000',
                        '06644444444446600000', '00064444444600000000', '00006444444600000000', '00000440044000000000',
                        '00000440044000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000444000000000', '00000004455400000000', '00000044555440000000', '00000445555444000000',
                        '00004445654444000000', '00044444664444400000', '40444444444444400004', '46604444444446600040',
                        '00066444444466000000', '00006444444600000000', '00006444444600000000', '00000440044000000000',
                        '00000440044000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n'))
                ],
                bC: { 4: '#ffcc00', 5: '#ff9900', 6: '#ff4444' },
                bf: [
                    p([
                        '00000000660000000000', '00000006666000000000', '00000066776600000000', '00000667776600000000',
                        '00006667676660000000', '00066666666666000000', '00666666666666600000', '60666666666666060000',
                        '66006666666666000060', '00006666666660000000', '00000066660000000000', '00000060060000000000',
                        '00000060060000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000660000000000', '00000006666000000000', '00000066776600000000', '00000667776600000000',
                        '00006667676660000000', '00066666666666000000', '06666666666666660000', '06006666666660060000',
                        '00006666666660000000', '00000066660000000000', '00000060060000000000', '00000060060000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000660000000000', '00000006666000000000', '00000066776600000000', '00000667776600000000',
                        '00006667676660000000', '00066666666666000000', '60666666666666060000', '00060666666606000000',
                        '00000666666600000000', '00000066660000000000', '00000060060000000000', '00000060060000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n'))
                ],
                bfC: { 6: '#ff3366', 7: '#44bbff' },
                boss: p([
                    '000000000088880000000000', '000000008899888000000000', '000000088999988800000000', '0000008899aa998800000000',
                    '000008889aa9988800000000', '0000888a9999aa8880000000', '000888a999999aa880000000', '00888aa9999999aa88000000',
                    '0888aaa999999aaa88000000', '8888aaa999999aaa88800000', '8888aaa99999aaa888800000', '88800aa99999aa088800000',
                    '888000aaa99aaa0088800000', '0880000aaaaaa00088000000', '00800000aaaa000080000000', '00000000aa00000000000000',
                    '00000000aa00aa0000000000', '00000000bb00bb0000000000', '00000000bb00bb0000000000', '000000000000000000000000',
                    '000000000000000000000000', '000000000000000000000000', '000000000000000000000000', '000000000000000000000000'
                ].join('\n')),
                bossHit: p([
                    '000000000bbbb0000000000', '0000000bbbbbbb000000000', '000000bbbbbbbbb00000000', '0000bbbbbbbbbbbbb000000',
                    '0000bbbbbbbbbbbbb000000', '000bbbbbbbbbbbbbbbbb000', '00bbbbbbbbbbbbbbbbb0000', '0bbbbbbbbbbbbbbbbb00000',
                    '0b00bbbbbbbbbbb00b00000', '0000bbbbbbbbbbbb0000000', '00000bbbbbbbbbb00000000', '000000bbbbbbbb000000000',
                    '0000000bbbbbbb000000000', '000000000b00b0000000000', '000000000b00b0000000000', '000000000000000000000000',
                    '000000000000000000000000', '000000000000000000000000', '000000000000000000000000', '000000000000000000000000',
                    '000000000000000000000000', '000000000000000000000000', '000000000000000000000000', '000000000000000000000000'
                ].join('\n')),
                bossCrit: p([
                    '000000000cccc0000000000', '0000000ccccccc000000000', '000000cccccccccc0000000', '0000ccccccccccccc000000',
                    '0000ccccccccccccc000000', '000ccccccccccccccccc000', '00cccccbbccccbbccc00000', '0cccbbbbbccccbbbbcc0000',
                    '0c00bbbbbccccbbb00c0000', '0000bbbbccccbbbb0000000', '00000bbbccccbbb00000000', '000000bbccccbb000000000',
                    '0000000bbcccb0000000000', '000000000c00c0000000000', '000000000c00c0000000000', '000000000000000000000000',
                    '000000000000000000000000', '000000000000000000000000', '000000000000000000000000', '000000000000000000000000',
                    '000000000000000000000000', '000000000000000000000000', '000000000000000000000000', '000000000000000000000000'
                ].join('\n')),
                bossC: { 8: '#44cc44', 9: '#88ff88', a: '#ff4444', b: '#88ccff', c: '#ff8800' },
                pwShield: p([
                    '00000001111000000000', '00000111111100000000', '00001110001110000000', '00011100000111000000',
                    '00110000000001100000', '00110000000001100000', '01100000000000110000', '01100000000000110000',
                    '01100000000000110000', '01100000000000110000', '01100000000000110000', '01100000000000110000',
                    '00110000000001100000', '00110000000001100000', '00001100000111000000', '00001110001110000000',
                    '00000111001110000000', '00000011111100000000', '00000001111000000000', '00000000110000000000'
                ].join('\n')),
                pwC: { 1: '#4488ff' }
            };
        }

        function getPixelSprite(sp, cols, flash) {
            const colorKey = flash ? flashPixelColors : cols;
            if (!sp || !colorKey || typeof colorKey !== 'object') return null;
            let byColor = spritePixelCache.get(sp);
            if (!byColor) {
                byColor = new WeakMap();
                spritePixelCache.set(sp, byColor);
            }
            if (byColor.has(colorKey)) return byColor.get(colorKey);
            const pixels = [];
            for (let r = 0; r < sp.length; r++) for (let cl = 0; cl < sp[r].length; cl++) {
                const v = sp[r][cl]; if (!v) continue;
                pixels.push({ x: cl, y: r, color: flash ? '#fff' : (cols[v] || '#fff') });
            }
            byColor.set(colorKey, pixels);
            return pixels;
        }

        function drawPixelSprite(ctx, pixels, x, y, scale) {
            const sz = (scale && scale > 1) ? scale : 1;
            for (let i = 0, n = pixels.length; i < n; i++) {
                const px = pixels[i];
                ctx.fillStyle = px.color;
                ctx.fillRect(x + px.x | 0, y + px.y | 0, sz, sz);
            }
        }

        function drawSp(cv, sp, cols, x, y, flash) {
            const pixels = getPixelSprite(sp, cols, flash);
            if (pixels) {
                drawPixelSprite(cv, pixels, x, y);
                return;
            }
            for (let r = 0; r < sp.length; r++) for (let cl = 0; cl < sp[r].length; cl++) {
                const v = sp[r][cl]; if (!v) continue;
                cv.fillStyle = flash ? '#fff' : (cols[v] || '#fff');
                cv.fillRect(Math.floor(x + cl), Math.floor(y + r), 1, 1);
            }
        }
        let _rcTick = -1, _rcCache = null;
        function rainbowPC() {
            if (tick === _rcTick) return _rcCache;
            const hue = (tick * 5) % 360;
            _rcCache = { 1: 'hsl(' + hue + ',100%,70%)', 2: 'hsl(' + ((hue + 120) % 360) + ',100%,70%)', 3: 'hsl(' + ((hue + 240) % 360) + ',100%,70%)' };
            _rcTick = tick;
            return _rcCache;
        }
        function drawStars(cv, dt) {
            const warp = G.warpT > 0 ? 10 : 1;
            for (const s of STARS) {
                const lm = s.layer === 0 ? 0.3 : s.layer === 1 ? 0.6 : s.layer === 2 ? 1 : 1.4;
                s.y += s.sp * dt * warp * lm;
                if (s.y > H) { s.y = 0; s.x = Math.random() * W; s.col = STAR_COLS[Math.floor(Math.random() * STAR_COLS.length)]; }
                let brightness = s.br * (0.6 + 0.4 * Math.sin(tick * 0.02 + s.x));
                if (s.layer === 3) { s.twinkle += dt * 3; brightness *= 0.5 + 0.5 * Math.sin(s.twinkle); }
                const colBase = s.col || '#ffffff';
                const r = parseInt(colBase.slice(1, 3), 16), g = parseInt(colBase.slice(3, 5), 16), b = parseInt(colBase.slice(5, 7), 16);
                cv.fillStyle = 'rgba(' + r + ',' + g + ',' + b + ',' + brightness + ')';
                const stretch = warp > 1 && s.layer >= 2 ? s.sz + 4 : s.sz;
                cv.fillRect(Math.floor(s.x), Math.floor(s.y), s.sz, stretch);
                if (s.layer >= 2 && brightness > 0.6) {
                    cv.globalAlpha = brightness * 0.3;
                    cv.fillRect(Math.floor(s.x - 1), Math.floor(s.y - 1), s.sz + 2, stretch + 2);
                    cv.globalAlpha = 1;
                }
            }
            // Shooting stars
            if (Math.random() < 0.003 && warp <= 1) {
                shootingStars.push({ x: Math.random() * W, y: -5, vx: -40 - Math.random() * 80, vy: 120 + Math.random() * 100, life: 1.5, t: 0, col: '#ffffff' });
            }
            let sslen = 0;
            for (let i = 0; i < shootingStars.length; i++) {
                const ss = shootingStars[i]; ss.x += ss.vx * dt; ss.y += ss.vy * dt; ss.t += dt;
                if (ss.t < ss.life && ss.y < H + 10 && ss.x > -50) {
                    const alpha = Math.max(0, 1 - ss.t / ss.life);
                    cv.globalAlpha = alpha;
                    cv.strokeStyle = ss.col; cv.lineWidth = 1;
                    cv.shadowBlur = 6; cv.shadowColor = ss.col;
                    cv.beginPath();
                    cv.moveTo(ss.x, ss.y); cv.lineTo(ss.x - ss.vx * dt * 3, ss.y - ss.vy * dt * 3);
                    cv.stroke();
                    cv.shadowBlur = 0;
                    shootingStars[sslen++] = ss;
                }
            }
            shootingStars.length = sslen; cv.globalAlpha = 1;

            if (G.warpT > 0) {
                const warpAlpha = Math.min(1, G.warpT / 500);
                const cx = W / 2, cy = H / 2;
                cv.save();
                cv.globalAlpha = warpAlpha * 0.85;
                cv.strokeStyle = 'rgba(220,240,255,' + warpAlpha + ')';
                cv.lineWidth = 1; cv.beginPath();
                for (const s of STARS) {
                    if (s.layer < 2 || s.sz !== 1) continue;
                    const dx = s.x - cx, dy = s.y - cy;
                    const dist = Math.hypot(dx, dy);
                    if (dist < 5) continue;
                    const len = Math.min(40, dist * 0.3 + 10) * warpAlpha;
                    const nx = dx / dist, ny = dy / dist;
                    cv.moveTo(s.x - nx * len, s.y - ny * len);
                    cv.lineTo(s.x, s.y);
                }
                cv.stroke();
                cv.lineWidth = 2; cv.beginPath();
                for (const s of STARS) {
                    if (s.layer < 2 || s.sz !== 2) continue;
                    const dx = s.x - cx, dy = s.y - cy;
                    const dist = Math.hypot(dx, dy);
                    if (dist < 5) continue;
                    const len = Math.min(40, dist * 0.3 + 10) * warpAlpha;
                    const nx = dx / dist, ny = dy / dist;
                    cv.moveTo(s.x - nx * len, s.y - ny * len);
                    cv.lineTo(s.x, s.y);
                }
                cv.stroke();
                cv.restore();
            }
            drawBG(cv, dt);
        }
        function drawBG(cv, dt) {
            const warp = G.warpT > 0 ? 10 : 1;
            for (const p of bgPlanets) {
                if (p.type === 'blackhole') {
                    p.rotSp += dt; const rr = p.r + Math.sin(tick * 0.02) * 3;
                    cv.save(); cv.globalAlpha = 0.5;
                    const gr = cachedRadialGradient(cv, 'blackhole', p.x, p.y, 0, p.r + 8, [[0, '#000'], [0.4, '#110033'], [0.7, '#220044'], [1, 'transparent']]);
                    cv.fillStyle = gr; cv.fillRect(p.x - rr - 8, p.y - rr - 8, (rr + 8) * 2, (rr + 8) * 2);
                    cv.beginPath(); cv.arc(p.x, p.y, p.r * 0.6, 0, Math.PI * 2); cv.fillStyle = '#000'; cv.fill();
                    cv.restore();
                } else if (p.type === 'debris') {
                    p.orbit += p.orbitSp * dt; const dx = Math.cos(p.orbit) * p.orbitR, dy = Math.sin(p.orbit) * p.orbitR;
                    cv.fillStyle = p.col; cv.globalAlpha = 0.35;
                    cv.fillRect(Math.floor(p.x + dx), Math.floor(p.y + dy), p.r * 2, p.r * 2);
                    cv.globalAlpha = 1;
                } else if (p.type === 'asteroid') {
                    p.y += p.sp * dt * warp * 0.3; if (p.y > H + p.r) { p.y = -p.r; p.x = Math.random() * W; }
                    cv.save(); cv.globalAlpha = 0.3; cv.translate(p.x, p.y); if (p.rot) cv.rotate(p.rot);
                    cv.fillStyle = p.col;
                    cv.fillRect(Math.floor(-p.r / 2), Math.floor(-p.r / 2), p.r, p.r);
                    cv.restore();
                } else if (p.type === 'planet') {
                    p.y += p.sp * dt * warp * 0.15; if (p.y > H + p.r * 2) { p.y = -p.r * 2; p.x = Math.random() * W; }
                    cv.save(); cv.globalAlpha = 0.35;
                    cv.beginPath(); cv.arc(p.x, p.y, p.r, 0, Math.PI * 2); cv.fillStyle = p.col; cv.fill();
                    if (p.atmoCol) { cv.beginPath(); cv.arc(p.x, p.y, p.r + 5, 0, Math.PI * 2); cv.fillStyle = p.atmoCol; cv.fill(); }
                    if (p.ringCol && p.ringR) {
                        cv.strokeStyle = p.ringCol; cv.lineWidth = 2; cv.beginPath();
                        cv.ellipse(p.x, p.y, p.r * p.ringR, p.r * p.ringR * 0.25, tick * 0.005, 0, Math.PI * 2);
                        cv.stroke();
                    }
                    if (p.storm && p.stormCol) {
                        for (let si = 0; si < 3; si++) {
                            const sa = tick * 0.01 + si * 2.1;
                            cv.beginPath(); cv.arc(p.x + Math.cos(sa) * p.r * 0.5, p.y + Math.sin(sa) * p.r * 0.3, p.r * 0.25, 0, Math.PI * 2);
                            cv.fillStyle = p.stormCol; cv.fill();
                        }
                    }
                    cv.restore();
                } else if (p.type === 'crystal') {
                    p.y += p.sp * dt * warp * 0.2; if (p.y > H + p.r * 2) { p.y = -p.r * 2; p.x = Math.random() * W; }
                    cv.save(); cv.globalAlpha = 0.4; cv.translate(p.x, p.y); cv.rotate(tick * 0.01);
                    cv.fillStyle = p.col;
                    cv.beginPath();
                    for (let ci = 0; ci < 6; ci++) {
                        const ca = (ci / 6) * Math.PI * 2;
                        const cx2 = Math.cos(ca) * p.r, cy2 = Math.sin(ca) * p.r;
                        if (ci === 0) cv.moveTo(cx2, cy2); else cv.lineTo(cx2, cy2);
                    }
                    cv.closePath(); cv.fill();
                    if (p.crystalCol) {
                        cv.strokeStyle = p.crystalCol; cv.lineWidth = 1; cv.stroke();
                        cv.globalAlpha = 0.2;
                        cv.fillStyle = p.crystalCol; cv.fill();
                    }
                    cv.restore();
                }
            }
            if (Math.random() < 0.004) {
                bgComets.push({ x: Math.random() * W, y: 0, vx: -30 - Math.random() * 70, vy: 160 + Math.random() * 140, life: 600, t: 0, size: 2 + Math.random() * 2 });
            }
            let cmlen = 0;
            for (let i = 0; i < bgComets.length; i++) {
                const cm = bgComets[i]; cm.x += cm.vx * dt; cm.y += cm.vy * dt; cm.t += dt * 1000;
                if (cm.t < cm.life && cm.y <= H) {
                    const alpha = Math.max(0, 1 - cm.t / cm.life);
                    cv.globalAlpha = alpha * 0.7;
                    cv.strokeStyle = '#aaccff'; cv.lineWidth = cm.size; cv.beginPath();
                    cv.moveTo(cm.x, cm.y); cv.lineTo(cm.x - cm.vx * dt * 2.5, cm.y - cm.vy * dt * 2.5);
                    cv.stroke();
                    cv.fillStyle = '#ddeeff';
                    cv.fillRect(Math.floor(cm.x - 1), Math.floor(cm.y - 1), 2, 2);
                    bgComets[cmlen++] = cm;
                }
            }
            bgComets.length = cmlen;
            cv.globalAlpha = 1;
        }
        function drawNebula(cv) {
            if (!nebulaCv) return;
            const pulse = 0.25 + 0.1 * Math.sin(G.fTmr * 0.3);
            cv.globalAlpha = pulse * (G.chal ? 1.3 : 1);
            const ny = (G.fTmr * 15) % H;
            cv.drawImage(nebulaCv, 0, ny - H); cv.drawImage(nebulaCv, 0, ny);
            if (G.chal) {
                cv.globalAlpha = Math.max(0, 0.08 + 0.05 * Math.sin(G.fTmr * 1.5));
                cv.fillStyle = '#ff440015'; cv.fillRect(0, 0, W, H);
            }
            cv.globalAlpha = 1;
        }

        function resize() {
            if (!wrapEl) return;
            scale = Math.max(1, Math.min(Math.floor(wrapEl.clientWidth / W) || 1, Math.floor(wrapEl.clientHeight / H) || 1));
            canvas.width = W * scale; canvas.height = H * scale;
            canvas.style.width = (W * scale) + 'px'; canvas.style.height = (H * scale) + 'px';
            c.imageSmoothingEnabled = false;
        }

        function isChal(s) { return s >= 3 && (s - 3) % 4 === 0; }

        function isMiniBossStage() { return G.stage >= 5 && G.stage % 5 === 0; }

        function mkFormation() {
            G.enemies = []; G.chal = isChal(G.stage); G.chalHits = 0; G.chalTot = 0;
            G.bossWarningShown = false;
            const isMini = isMiniBossStage();
            const formType = (G.stage - 1) % 6;
            let idx = 0;

            function pushEnemy(type, r, col, fx, fy, hp) {
                const side = idx % 2 === 0 ? -1 : 1;
                const diveDelay = G.chal ? (800 + idx * 200) : (1000 + Math.random() * 3000 + idx * 50);
                G.enemies.push({ type, r, col, x: W / 2 + side * (120 + Math.random() * 80), y: -30 - (idx % 8) * 20,
                    fx, fy, hp, maxHp: hp, st: 'ENTER', eTmr: 500 + idx * 80 + r * 100, eProg: 0,
                    fr: 0, frT: 0, dTmr: diveDelay / diffMod('diveRate'), dPath: null, sTmr: 0, hasCap: false, hitF: 0 });
                idx++;
            }

            if (isMini) {
                const mbHP = 4 + Math.floor(G.stage / 5);
                pushEnemy('miniboss', 0, 4, W / 2, FTOP, mbHP);
            }

            for (let r = 0; r < FROWS; r++) for (let col = 0; col < FCOLS; col++) {
                let type = 'bee';
                if (r === 0) { if (col < 3 || col > 6) continue; if (!isMini) type = 'boss'; }
                else if (r <= 2) type = 'butterfly';

                let fx, fy;
                const cx = W / 2, cy = FTOP;

                if (formType === 0) {
                    fx = cx + (col - FCOLS / 2 + 0.5) * ESP_X;
                    fy = cy + r * ESP_Y;
                } else if (formType === 1) {
                    const vDepth = r * (1 - Math.abs(col - FCOLS / 2) / (FCOLS / 2)) ;
                    fx = cx + (col - FCOLS / 2 + 0.5) * ESP_X;
                    fy = cy + r * ESP_Y * 0.7 + vDepth * 12;
                } else if (formType === 2) {
                    const angle = -0.6 + (col / (FCOLS - 1)) * 1.2;
                    const radius = 100 + r * ESP_Y;
                    fx = cx + Math.sin(angle) * radius;
                    fy = cy + r * 20 + (1 - Math.cos(angle)) * 40;
                } else if (formType === 3) {
                    const zig = (col % 2 === 0 ? 1 : -1) * r * 12;
                    fx = cx + (col - FCOLS / 2 + 0.5) * ESP_X + zig;
                    fy = cy + r * ESP_Y;
                } else if (formType === 4) {
                    const diamondR = Math.abs(col - FCOLS / 2 + 0.5) / (FCOLS / 2);
                    fx = cx + (col - FCOLS / 2 + 0.5) * ESP_X * (1 + diamondR * 0.3);
                    fy = cy + r * ESP_Y * (1 - diamondR * 0.4);
                } else {
                    const heartT = col / (FCOLS - 1) * Math.PI * 2;
                    const heartX = 16 * Math.pow(Math.sin(heartT), 3);
                    const heartY = -(13 * Math.cos(heartT) - 5 * Math.cos(2 * heartT) - 2 * Math.cos(3 * heartT) - Math.cos(4 * heartT));
                    fx = cx + heartX * 3.5 + (r - 2) * 4;
                    fy = cy + 30 + heartY * 3 + r * 8;
                    if (fy < FTOP) fy = FTOP + Math.abs(fy - FTOP);
                }

                const bossHP = G.stage >= 5 ? 2 + Math.floor((G.stage - 5) / 4) : 2;
                const enemyHP = type === 'boss' ? bossHP : 1;
                pushEnemy(type, r, col, fx, fy, enemyHP);
            }
            G.chalTot = G.enemies.length;
            G.dTmr = (2000 - Math.min(G.stage * 100, 1200)) / diffMod('diveRate');
            G.fX = 0;
            mkNebula(); initBG();
            if (isMini) SFX.miniBossWarning();
        }

        function startStage() {
            G.st = 'STAGE_INTRO'; G.sTmr = 2000; G.bul = []; G.ebul = []; G.exp = []; G.part = [];
            G.beam = null; G.powerups = []; G.activePU = null; G.puTimer = 0; G.shieldHits = 0;
            G.scorePopups = []; G.warpT = 0; G.warpFlash = 0; G.perfectT = 0;
            G.combo = 0; G.comboTimer = 0; G.comboMult = 1; G.comboBanner = null;
            G.trails = []; G.timeScale = 1; G.timeSlowTimer = 0; G.freezeT = 0; G.damageVignetteT = 0;
            G.bossWarningT = 0; G.bossWarningShown = false;
            G.weaponLv = Math.max(1, G.weaponLv); G.puUpgrade = null; G.upgradeBanner = null; G.killCount = 0; G.slowMoT = 0;
            G.p.x = W / 2; G.p.alive = true; G.p.inv = 2000; G.p.cap = null; G.p.dual = false; G.p.reviveTimer = 0;
            setPUClass(null);
            G.chal ? SFX.challenge() : SFX.stageClear();
            MusicEngine.setTempo(1 + G.stage * 0.05);
            MusicEngine.play(G.chal ? 'challenge' : 'gameplay');
        }

        let lastFireT = 0;
        function fire(now) {
            if (G.activePU && (G.activePU.type === 'laser' || G.activePU.type === 'mega_laser')) {
                const cd = G.activePU.type === 'mega_laser' ? 200 : 300;
                if (now - lastFireT < cd) return;
                lastFireT = now;
                G.bul.push({ x: G.p.x, y: G.p.y - 8, w: G.activePU.type === 'mega_laser' ? 6 : 4, h: 14, vx: 0, vy: -PB_SPEED * 1.5, laser: true });
                if (G.p.dual) G.bul.push({ x: G.p.x + 28, y: G.p.y - 8, w: G.activePU.type === 'mega_laser' ? 6 : 4, h: 14, vx: 0, vy: -PB_SPEED * 1.5, laser: true });
                SFX.laserShoot(G.p.x);
                return;
            }
            const isUltraRapid = G.activePU && G.activePU.type === 'ultra_rapid';
            const isRapid = G.activePU && G.activePU.type === 'rapid';
            const cd = isUltraRapid ? 80 : isRapid ? 120 : 250;
            if (now - lastFireT < cd) return;
            lastFireT = now;
            const isPierce = G.activePU && (G.activePU.type === 'pierce' || G.activePU.type === 'mega_pierce');
            const isHoming = G.activePU && G.activePU.type === 'homing';
            const isMegaSpread = G.activePU && G.activePU.type === 'mega_spread';
            const isSpread = G.activePU && G.activePU.type === 'spread';
            if (isHoming && G.activePU.shots > 0) {
                const nearestE = G.enemies.filter(e => e.st !== 'DEAD').sort((a2, b2) => {
                    const da = Math.hypot(a2.x - G.p.x, a2.y - G.p.y);
                    const db = Math.hypot(b2.x - G.p.x, b2.y - G.p.y);
                    return da - db;
                })[0];
                if (nearestE) {
                    const dx = nearestE.x - G.p.x, dy = nearestE.y - G.p.y;
                    const dist = Math.hypot(dx, dy);
                    G.bul.push({ x: G.p.x, y: G.p.y - 8, w: 3, h: 6, vx: (dx / dist) * PB_SPEED * 0.7, vy: (dy / dist) * PB_SPEED * 0.7, homing: true, target: nearestE });
                    G.activePU.shots--;
                    SFX.homingLock(G.p.x);
                    if (G.activePU.shots <= 0) { G.activePU = null; G.puTimer = 0; setPUClass(null); }
                }
                return;
            }
            if (isMegaSpread) {
                for (let a = -25; a <= 25; a += 10) {
                    const rad = a * Math.PI / 180;
                    G.bul.push({ x: G.p.x, y: G.p.y - 8, w: 2, h: 6, vx: Math.sin(rad) * PB_SPEED * 0.3, vy: -PB_SPEED, pierce: isPierce });
                }
            } else if (isSpread) {
                for (let a = -15; a <= 15; a += 15) {
                    const rad = a * Math.PI / 180;
                    G.bul.push({ x: G.p.x, y: G.p.y - 8, w: 2, h: 6, vx: Math.sin(rad) * PB_SPEED * 0.3, vy: -PB_SPEED, pierce: isPierce });
                }
            } else {
                const lv = G.weaponLv;
                if (lv >= 3) {
                    G.bul.push({ x: G.p.x - 6, y: G.p.y - 8, w: 2, h: 6, vx: 0, vy: -PB_SPEED, pierce: isPierce });
                    G.bul.push({ x: G.p.x, y: G.p.y - 8, w: 2, h: 6, vx: 0, vy: -PB_SPEED, pierce: isPierce });
                    G.bul.push({ x: G.p.x + 6, y: G.p.y - 8, w: 2, h: 6, vx: 0, vy: -PB_SPEED, pierce: isPierce });
                } else if (lv >= 2) {
                    G.bul.push({ x: G.p.x - 4, y: G.p.y - 8, w: 2, h: 6, vx: 0, vy: -PB_SPEED, pierce: isPierce });
                    G.bul.push({ x: G.p.x + 4, y: G.p.y - 8, w: 2, h: 6, vx: 0, vy: -PB_SPEED, pierce: isPierce });
                } else {
                    const max = G.p.dual ? 2 : 1;
                    if (G.bul.filter(b => !b.vx && !b.laser).length >= max) return;
                    G.bul.push({ x: G.p.x, y: G.p.y - 8, w: 2, h: 6, vx: 0, vy: -PB_SPEED, pierce: isPierce });
                    if (G.p.dual) G.bul.push({ x: G.p.x + 28, y: G.p.y - 8, w: 2, h: 6, vx: 0, vy: -PB_SPEED, pierce: isPierce });
                }
                if (lv >= 4 && !isRapid && !isUltraRapid) {
                    G.bul.push({ x: G.p.x, y: G.p.y - 8, w: 2, h: 6, vx: -Math.sin(0.2) * PB_SPEED * 0.2, vy: -PB_SPEED, pierce: isPierce });
                    G.bul.push({ x: G.p.x, y: G.p.y - 8, w: 2, h: 6, vx: Math.sin(0.2) * PB_SPEED * 0.2, vy: -PB_SPEED, pierce: isPierce });
                }
            }
            SFX.shoot(G.p.x); G.muzzleT = 50;
        }

        function boom(x, y, isBoss) {
            const dur = isBoss ? 900 : 450;
            const pCount = isBoss ? 50 : 20;
            const sparkCount = isBoss ? 24 : 10;
            const debrisCount = isBoss ? 14 : 6;
            const smokeCount = isBoss ? 14 : 7;
            const flashCount = isBoss ? 8 : 4;
            G.exp.push({ x, y, t: 0, dur, seed: Math.random(), isBoss });
            if (isBoss) { G.exp.push({ x, y, t: 0, dur: 700, seed: Math.random(), isBoss: false, shockwave: true }); G.exp.push({ x, y, t: 0, dur: 180, seed: Math.random(), isBoss: false, flash: true }); }
            else { G.exp.push({ x, y, t: 0, dur: 100, seed: Math.random(), isBoss: false, flash: true }); }
            const fireCols = ['#ffcc00', '#ff4444', '#ff8800', '#fff', '#ffee88', '#ff6622', '#ffaa00'];
            for (let i = 0; i < pCount; i++) {
                const a = (i / pCount) * Math.PI * 2 + Math.random() * 0.8, sp = 60 + (i * 23 % 160) * (isBoss ? 2 : 1.2);
                const cols = fireCols[i % fireCols.length];
                const sz = i % 4 === 0 ? 4 : i % 3 === 0 ? 3 : 2;
                G.part.push({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: (280 + (i * 41 % 280)) * (isBoss ? 1.6 : 1.1), t: 0, col: cols, size: sz });
            }
            for (let i = 0; i < sparkCount; i++) {
                const a = Math.random() * Math.PI * 2, sp = 90 + Math.random() * 150;
                G.part.push({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 120 + Math.random() * 180, t: 0, col: Math.random() > 0.5 ? '#ffffff' : '#ffeeaa', size: 1, spark: true });
            }
            for (let i = 0; i < debrisCount; i++) {
                const a = Math.random() * Math.PI * 2, sp = 25 + Math.random() * 50;
                const sz = isBoss ? 3 + Math.random() * 4 : 2 + Math.random() * 3;
                G.part.push({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp - 18, life: 600 + Math.random() * 500, t: 0, col: isBoss ? '#999' : '#777', size: sz, debris: true, rot: Math.random() * 6.28 });
            }
            for (let i = 0; i < smokeCount; i++) {
                const a = Math.random() * Math.PI * 2, sp = 12 + Math.random() * 25;
                G.part.push({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp - 15, life: 600 + Math.random() * 500, t: 0, col: Math.random() > 0.5 ? '#666' : '#555', size: 3 + (isBoss ? 3 : 0), smoke: true });
            }
            for (let i = 0; i < flashCount; i++) {
                const a = Math.random() * Math.PI * 2, sp = 40 + Math.random() * 80;
                G.part.push({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 60 + Math.random() * 40, t: 0, col: '#ffffff', size: 2, spark: true });
            }
            if (isBoss) {
                for (let i = 0; i < 6; i++) {
                    setTimeout(() => { if (!state.disposed) boom(x + (Math.random() - 0.5) * 50, y + (Math.random() - 0.5) * 40, false); }, i * 100);
                }
                G.plasmaRings.push({ x, y, r: 0, maxR: 140, t: 0, dur: 800, col: '#ff4444' });
                G.plasmaRings.push({ x, y, r: 0, maxR: 100, t: 0, dur: 550, col: '#ff8800' });
                G.plasmaRings.push({ x, y, r: 0, maxR: 60, t: 0, dur: 320, col: '#ffcc00' });
                G.plasmaRings.push({ x, y, r: 0, maxR: 30, t: 0, dur: 200, col: '#ffffff' });
            }
        }

        function bulletImpact(x, y, col) {
            for (let i = 0; i < 4; i++) {
                const a = Math.random() * Math.PI * 2, sp = 30 + Math.random() * 50;
                G.part.push({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 80 + Math.random() * 60, t: 0, col: col || '#ffff88', size: 1, spark: true });
            }
        }

        function addScore(pts, x, y, col) {
            const prev = G.score;
            const multiplied = pts * G.comboMult;
            G.score += multiplied;
            if (G.score > G.hi) G.hi = G.score;
            const text = G.comboMult > 1 ? '+' + multiplied + ' x' + G.comboMult : '+' + multiplied;
            if (x !== undefined) G.scorePopups.push({ x, y, text, t: 0, dur: 800, col: col || '#ffcc00', big: G.comboMult > 1 });
            if (Math.floor(G.score / EXTRA_LIFE) > Math.floor(prev / EXTRA_LIFE)) { G.lives++; SFX.extra(); }
        }

        function updateCombo(dtMs) {
            if (G.comboTimer > 0) {
                G.comboTimer -= dtMs || 16;
                if (G.comboTimer <= 0) { if (G.combo > 2) SFX.comboBreak(); G.combo = 0; G.comboMult = 1; G.comboBanner = null; }
            }
        }

        function registerKill() {
            G.combo++;
            G.comboTimer = COMBO_TIMEOUT;
            let level = 0;
            for (let i = COMBO_THRESH.length - 1; i >= 0; i--) { if (G.combo >= COMBO_THRESH[i]) { level = i + 1; break; } }
            G.comboMult = COMBO_MULT[level] || 1;
            if (level > 0 && COMBO_TEXT[level]) {
                G.comboBanner = { text: COMBO_TEXT[level], mult: G.comboMult, t: 0, dur: 1200 };
                SFX.combo(level);
                if (level >= 4) SFX.killStreak();
            }
        }

        function hit(a, b) { return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y; }

        function dropPU(e) {
            const chance = e.type === 'miniboss' ? 1 : (e.type === 'boss' ? 0.35 : (e.type === 'bee' && !diffMod('puFromBee') ? 0 : 0.12));
            if (Math.random() < chance) {
                const type = PU_TYPES[Math.floor(Math.random() * PU_TYPES.length)];
                G.powerups.push({ x: e.x, y: e.y, type, t: 0 });
            }
        }

        function collectPU(pu) {
            if (pu.type === 'bomb' || pu.type === 'multibomb') {
                SFX.bomb(pu.x);
                const bonus = pu.type === 'multibomb' ? 500 : 0;
                for (const e of G.enemies) { if (e.st !== 'DEAD') { addScore(PTS[e.type][0] + bonus, e.x, e.y, PU_COL[pu.type]); boom(e.x, e.y, e.type === 'boss'); e.st = 'DEAD'; } }
                G.flashT = 100; G.activePU = null; G.puTimer = 0; setPUClass(null); return;
            }
            if (pu.type === 'supernova') {
                SFX.supernova(pu.x);
                for (const e of G.enemies) { if (e.st !== 'DEAD') { addScore(PTS[e.type][0] + 1000, e.x, e.y, '#fff'); boom(e.x, e.y, e.type === 'boss'); e.st = 'DEAD'; } }
                for (let i = G.ebul.length - 1; i >= 0; i--) { bulletImpact(G.ebul[i].x, G.ebul[i].y, '#fff'); }
                G.ebul = []; G.flashT = 200; G.activePU = null; G.puTimer = 0; setPUClass(null);
                G.shkT = 500; G.shkM = 8;
                return;
            }
            if (pu.type === 'freeze') {
                SFX.freeze(pu.x);
                G.freezeT = PU_DUR.freeze;
                G.activePU = { type: 'freeze', timer: PU_DUR.freeze }; G.puTimer = PU_DUR.freeze; setPUClass('freeze');
                for (const e of G.enemies) {
                    if (e.st === 'DEAD') continue;
                    for (let _fi = 0; _fi < 8; _fi++) {
                        const _fa = (_fi / 8) * Math.PI * 2;
                        G.part.push({ x: e.x + Math.cos(_fa) * 14, y: e.y + Math.sin(_fa) * 14, vx: Math.cos(_fa) * 35, vy: Math.sin(_fa) * 35 - 10, life: 500, t: 0, col: '#88eeff', size: 2 });
                        G.part.push({ x: e.x, y: e.y, vx: (Math.random()-0.5)*40, vy: -20-Math.random()*30, life: 350, t: 0, col: '#ccf4ff', size: 1, spark: true });
                    }
                }
                G.flashT = 30; return;
            }
            const isUpgradeable = PU_UPGRADE[pu.type];
            const isSameType = G.activePU && G.activePU.type === pu.type;
            if (isUpgradeable && isSameType && !G.puUpgrade) {
                G.puUpgrade = PU_UPGRADE[pu.type]; G.activePU.type = G.puUpgrade; G.puTimer = PU_DUR[pu.type] || 0;
                G.upgradeBanner = { text: 'POWER UP!', type: G.puUpgrade, t: 0, dur: 1500 };
                SFX.puUpgrade(pu.x); setPUClass(G.puUpgrade);
            } else if (pu.type === 'homing') {
                G.activePU = { type: 'homing', timer: 0, shots: 5 }; G.puTimer = 30000; setPUClass('homing');
            } else if (pu.type === 'shield') { G.shieldHits = 3; G.activePU = { type: 'shield', timer: 0 }; G.puTimer = 0; setPUClass('shield'); }
            else if (pu.type === 'pierce') {
                G.activePU = { type: G.puUpgrade === 'mega_pierce' ? 'mega_pierce' : 'pierce', timer: PU_DUR.pierce }; G.puTimer = PU_DUR.pierce; setPUClass(G.activePU.type);
            }
            else if (pu.type === 'speed' || pu.type === 'magnet' || pu.type === 'laser' || pu.type === 'timeslow' || pu.type === 'rapid' || pu.type === 'spread') {
                const upType = (isUpgradeable && isSameType) ? PU_UPGRADE[pu.type] : pu.type;
                G.activePU = { type: upType, timer: PU_DUR[pu.type] || 0 }; G.puTimer = PU_DUR[pu.type] || 0; setPUClass(upType);
                if (pu.type === 'timeslow') { G.timeScale = 0.35; G.timeSlowTimer = PU_DUR.timeslow; }
            }
            else { G.activePU = { type: pu.type, timer: PU_DUR[pu.type] || 0 }; G.puTimer = PU_DUR[pu.type] || 0; setPUClass(pu.type); }
            SFX.puCollect(pu.x);
            const puCol = PU_COL[pu.type] || PU_UPGRADE_COL[pu.type];
            for (let i = 0; i < 12; i++) {
                const a = (i / 12) * Math.PI * 2, sp = 60 + Math.random() * 40;
                G.part.push({ x: pu.x, y: pu.y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: 200 + Math.random() * 100, t: 0, col: puCol, size: 2, spark: true });
            }
        }

        function killP() {
            if (!G.p.alive) return;
            if (G.shieldHits > 0) { G.shieldHits--; G.damageVignetteT = 300; if (G.shieldHits <= 0) { G.activePU = null; G.puTimer = 0; setPUClass(null); SFX.shieldBreak(); } else SFX.shieldHit(); return; }
G.p.alive = false; boom(G.p.x, G.p.y); SFX.pExplode(G.p.x); G.shkT = 300; G.shkM = 4; G.lives--;
            G.flashT = 50; G.chromAb = 300; G.damageVignetteT = 800; G.activePU = null; G.shieldHits = 0; G.timeScale = 1; G.timeSlowTimer = 0; G.puUpgrade = null;
            G.weaponLv = Math.max(1, G.weaponLv - 1);
            for (let i = 0; i < 8; i++) {
                const a = Math.random() * 6.28, sp = 30 + Math.random() * 50;
                G.deathParts.push({ x: G.p.x, y: G.p.y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp - 20, life: 800, t: 0, col: SP.pC[1 + (i % 4)] || '#fff', sz: 3 + Math.random() * 3, rot: Math.random() * 6.28 });
            }
            setPUClass(null);
            if (G.lives < 0) { G.st = 'GAME_OVER'; G.sTmr = 3000; G.contTmr = 10; G.contCnt = 10; MusicEngine.play('gameover'); }
            else { G.p.reviveTimer = 1500; }
        }

        function updateP(dt, now) {
            if (!G.p.alive) {
                if (G.p.reviveTimer > 0 && G.st === 'PLAYING') {
                    G.p.reviveTimer -= dt * 1000;
                    if (G.p.reviveTimer <= 0) { G.p.x = W / 2; G.p.alive = true; G.p.inv = 3000; G.p.reviveTimer = 0; SFX.respawn(); }
                }
                return;
            }
            const inp = G.inp;
            const spd = G.activePU && (G.activePU.type === 'speed' || G.activePU.type === 'hyper_speed') ? PLAYER_SPEED * (G.activePU.type === 'hyper_speed' ? 2.2 : 1.8) : PLAYER_SPEED;
            if (inp.l) G.p.x -= spd * dt; if (inp.r) G.p.x += spd * dt;
            G.p.x = Math.max(10, Math.min(W - 10, G.p.x));
            if (G.p.inv > 0) G.p.inv -= dt * 1000;
            if (inp.f) fire(now);
            if (G.beam && G.beam.active && G.p.x > G.beam.x - 20 && G.p.x < G.beam.x + 20 && G.p.y > G.beam.y) {
                if (G.p.alive) { killP(); G.beam.cap = true; G.beam.capT = 0; SFX.beam(); }
            }
            if (G.activePU && G.activePU.type !== 'shield') {
                G.puTimer -= dt * 1000;
                if (G.puTimer <= 0) {
                    if (G.activePU.type === 'timeslow') { G.timeScale = 1; G.timeSlowTimer = 0; }
                    G.activePU = null; G.puUpgrade = null; setPUClass(null);
                }
            }
            if (G.timeSlowTimer > 0 && G.activePU && G.activePU.type === 'timeslow') {
                G.timeSlowTimer -= dt * 1000;
            }
            if (G.activePU && G.activePU.type === 'magnet' && G.p.alive) {
                for (const pu of G.powerups) {
                    const dx = G.p.x - pu.x, dy = G.p.y - pu.y;
                    const dist = Math.sqrt(dx * dx + dy * dy);
                    if (dist < 80 && dist > 5) { pu.x += dx / dist * 120 * dt; pu.y += dy / dist * 120 * dt; }
                }
            }
            if (G.p.alive) {
                const eg = 0.5 + Math.sin(tick * 0.15) * 0.3;
                const tRgb = G.activePU && PU_TRAIL_COL[G.activePU.type] ? PU_TRAIL_COL[G.activePU.type] : '255,150,50';
                const tCol1 = 'rgba(' + tRgb + ',' + eg + ')';
                const tCol2 = 'rgba(' + tRgb + ',0.4)';
                if (G.trails.length < 80) {
                    G.trails.push({ x: G.p.x - 6, y: G.p.y + 12, vx: (Math.random() - 0.5) * 10, vy: 20 + Math.random() * 15, life: 150, t: 0, col: tCol1, size: 2 });
                    G.trails.push({ x: G.p.x + 3, y: G.p.y + 12, vx: (Math.random() - 0.5) * 10, vy: 20 + Math.random() * 15, life: 150, t: 0, col: tCol1, size: 2 });
                    G.trails.push({ x: G.p.x - 4, y: G.p.y + 14, vx: (Math.random() - 0.5) * 5, vy: 15 + Math.random() * 10, life: 100, t: 0, col: tCol2, size: 1 });
                    if (G.p.dual) {
                        G.trails.push({ x: G.p.x + 28, y: G.p.y + 12, vx: (Math.random() - 0.5) * 10, vy: 20 + Math.random() * 15, life: 150, t: 0, col: tCol1, size: 2 });
                        G.trails.push({ x: G.p.x + 34, y: G.p.y + 12, vx: (Math.random() - 0.5) * 10, vy: 20 + Math.random() * 15, life: 150, t: 0, col: tCol1, size: 2 });
                    }
                }
            }
            for (let i = G.powerups.length - 1; i >= 0; i--) {
                G.powerups[i].y += 60 * dt; G.powerups[i].t += dt * 1000;
                if (G.powerups[i].y > H + 20) { G.powerups.splice(i, 1); continue; }
                if (G.p.alive && hit({ x: G.powerups[i].x - 5, y: G.powerups[i].y - 5, w: 10, h: 10 }, { x: G.p.x - 6, y: G.p.y - 6, w: 12, h: 12 })) {
                    collectPU(G.powerups[i]); G.powerups.splice(i, 1);
                }
            }
        }

        function updateBul(dt) {
            for (let i = G.bul.length - 1; i >= 0; i--) {
                const b = G.bul[i];
                if (b.homing && b.target && b.target.st !== 'DEAD') {
                    const dx = b.target.x - b.x, dy = b.target.y - b.y;
                    const dist = Math.hypot(dx, dy);
                    if (dist > 5) {
                        b.vx += (dx / dist) * 800 * dt; b.vy += (dy / dist) * 800 * dt;
                        const spd = Math.hypot(b.vx, b.vy);
                        const maxSpd = PB_SPEED * 0.8;
                        if (spd > maxSpd) { b.vx *= maxSpd / spd; b.vy *= maxSpd / spd; }
                    }
                    b.x += b.vx * dt; b.y += b.vy * dt;
                } else if (b.vx) { b.x += b.vx * dt; b.y += b.vy * dt; } else b.y -= (b.laser ? PB_SPEED * 1.5 : PB_SPEED) * dt;
                if (b.y < -10 || b.x < -10 || b.x > W + 10 || b.y > H + 10) { G.bul.splice(i, 1); continue; }
                let removed = false;
                for (let j = G.enemies.length - 1; j >= 0; j--) {
                    const e = G.enemies[j]; if (e.st === 'DEAD') continue;
                    const ew = (e.type === 'boss' || e.type === 'miniboss') ? 24 : 16;
                    if (hit(b, { x: e.x - ew / 2, y: e.y - 8, w: ew, h: 16 })) {
                        e.hp--;
                        if (e.hp <= 0) {
                            const pts = PTS[e.type] ? PTS[e.type][e.st === 'DIVING' ? 1 : 0] : 200;
                            registerKill();
                            addScore(pts, e.x, e.y, e.type === 'bee' ? '#ffcc00' : e.type === 'butterfly' ? '#ff3366' : '#44cc44');
                            boom(e.x, e.y, e.type === 'boss' || e.type === 'miniboss'); SFX.eExplode(e.x); dropPU(e);
                            if (e.type === 'boss' || e.type === 'miniboss') { G.timeScale = 0.3; G.slowMoT = 1500; }
                            if (e.hasCap) G.p.cap = { x: e.x, y: e.y };
                            if (G.chal) G.chalHits++; e.st = 'DEAD';
                            G.killCount++;
                            if (G.killCount % 10 === 0 && G.weaponLv < 4) { G.weaponLv++; SFX.weaponUp(); }
                        } else e.hitF = 100;
                        if (!b.laser && !b.pierce) { removed = true; break; }
                    }
                }
                if (removed) G.bul.splice(i, 1);
            }
            const eDt = dt * G.timeScale;
            for (let i = G.ebul.length - 1; i >= 0; i--) {
                const b = G.ebul[i]; b.y += EB_SPEED * diffMod('ebSpd') * eDt;
                if (b.y > H + 10) { G.ebul.splice(i, 1); continue; }
                if (G.p.alive && G.p.inv <= 0 && hit(b, { x: G.p.x - 6, y: G.p.y - 6, w: 12, h: 12 })) { killP(); G.ebul.splice(i, 1); }
            }
        }

        function startDive(e) {
            if (e.st !== 'FORM') return; e.st = 'DIVING'; e.dPath = { ph: 0, amp: 30 + Math.random() * 40, vx: (Math.random() - 0.5) * 130 }; e.dTmr = 3000; e.sTmr = 500 + Math.random() * 1000; SFX.dive();
            if ((e.type === 'boss' || e.type === 'miniboss') && !e.hasCap && !G.beam && G.stage > 1 && Math.random() < 0.3) G.beam = { active: true, owner: e, x: e.x, y: e.y + 16, h: 0, t: 0, cap: false, capT: 0 };
        }

        function updateE(dt) {
            const eDt = dt * G.timeScale;
            const dtMs = eDt * 1000; G.fTmr += dt; G.fX = Math.sin(G.fTmr * 0.5) * 30;
            for (const e of G.enemies) {
                if (e.st === 'DEAD') continue; e.frT += dtMs; if (e.frT > 300) { e.fr = 1 - e.fr; e.frT = 0; } if (e.hitF > 0) e.hitF -= dtMs;
                if (G.freezeT > 0 && e.st !== 'ENTER') continue;
                if (e.st === 'ENTER') {
                    e.eTmr -= dtMs; if (e.eTmr <= 0) { e.eProg += eDt * 1.5; const tm = Math.min(e.eProg, 1); e.x += (e.fx - e.x) * tm * 0.05; e.y += (e.fy - e.y) * tm * 0.05; if (tm >= 1 && Math.abs(e.x - e.fx) < 2 && Math.abs(e.y - e.fy) < 2) { e.x = e.fx; e.y = e.fy; e.st = 'FORM'; for (let _ei = 0; _ei < 2; _ei++) { const _ea = Math.random() * Math.PI * 2; G.part.push({ x: e.x, y: e.y, vx: Math.cos(_ea)*25, vy: Math.sin(_ea)*25, life: 200, t: 0, col: e.type === 'bee' ? '#ffcc00' : e.type === 'butterfly' ? '#ff3366' : '#44cc44', size: 1, spark: true }); } if ((e.type === 'boss' || e.type === 'miniboss') && !G.bossWarningShown) { G.bossWarningT = 2000; G.bossWarningShown = true; if (e.type === 'miniboss') SFX.miniBossWarning(); else SFX.bossWarning(); } } }
                }
                else if (e.st === 'FORM') {
                    e.x = e.fx + G.fX; e.y = e.fy + Math.sin(G.fTmr * 2 + e.col * 0.5) * 3;
                    if (!G.chal) { e.dTmr -= dtMs; if (e.dTmr <= 0 && Math.random() < 0.008 * Math.min(G.stage, 10) * diffMod('diveRate')) startDive(e); }
                    else { e.dTmr -= dtMs; if (e.dTmr <= 0) startChalDive(e); }
                }
                else if (e.st === 'DIVING') {
                    e.dTmr -= dtMs;
                    if (e.dTmr <= 0 || e.y > H + 20) { e.st = 'RETURN'; e.y = -20; }
                    else {
                        e.y += DIVE_SPD * eDt; if (e.dPath) { e.dPath.ph += eDt * 3; e.x += Math.sin(e.dPath.ph) * e.dPath.amp * eDt * 2 + e.dPath.vx * eDt; }
                        if (G.beam && G.beam.owner === e) { G.beam.x = e.x; G.beam.y = e.y + 16; }
                        e.sTmr -= dtMs;
                        if (e.sTmr <= 0 && !G.chal) {
                            if (e.type !== 'bee') {
                                G.ebul.push({ x: e.x, y: e.y + 8, w: 2, h: 6 });
                                if (G.stage >= 5 && e.type === 'boss') { G.ebul.push({ x: e.x - 8, y: e.y + 8, w: 2, h: 6 }); G.ebul.push({ x: e.x + 8, y: e.y + 8, w: 2, h: 6 }); }
                                if (G.stage >= 8 && e.type === 'boss') { for (let k = 0; k < 3; k++) setTimeout(() => { if (!state.disposed && e.st === 'DIVING') G.ebul.push({ x: e.x, y: e.y + 8, w: 2, h: 6 }); }, k * 150); }
                                if (e.type === 'miniboss') { G.ebul.push({ x: e.x - 10, y: e.y + 8, w: 2, h: 6 }); G.ebul.push({ x: e.x + 10, y: e.y + 8, w: 2, h: 6 }); for (let k = 0; k < 2; k++) setTimeout(() => { if (!state.disposed && e.st === 'DIVING') { G.ebul.push({ x: e.x - 6, y: e.y + 8, w: 2, h: 6 }); G.ebul.push({ x: e.x + 6, y: e.y + 8, w: 2, h: 6 }); } }, k * 180); }
                            }
                            e.sTmr = e.type === 'miniboss' ? 500 + Math.random() * 800 : 800 + Math.random() * 1200;
                        }
                        if (G.p.alive && G.p.inv <= 0) {
                            const ew = (e.type === 'boss' || e.type === 'miniboss') ? 16 : 12;
                            if (hit({ x: e.x - ew / 2, y: e.y - 8, w: ew, h: 16 }, { x: G.p.x - 6, y: G.p.y - 6, w: 12, h: 12 })) {
                                registerKill(); addScore(PTS[e.type] ? PTS[e.type][1] : 200, e.x, e.y); boom(e.x, e.y, e.type === 'boss' || e.type === 'miniboss'); SFX.eExplode(e.x); if (G.chal) G.chalHits++; e.st = 'DEAD'; killP();
                            }
                        }
                    }
                }
                else if (e.st === 'RETURN') { e.x += (e.fx + G.fX - e.x) * eDt * 3; e.y += (e.fy - e.y) * eDt * 3; if (Math.abs(e.x - e.fx - G.fX) < 3 && Math.abs(e.y - e.fy) < 3) { if (G.chal) { e.st = 'DEAD'; G.chalHits++; } else e.st = 'FORM'; } }
            }
            if (G.beam && G.beam.active) { G.beam.t += dtMs; G.beam.h = Math.min(200, G.beam.h + eDt * 300); if (G.beam.t > 3000) { G.beam.active = false; if (G.beam.cap && G.p.cap) { G.beam.owner.hasCap = true; G.p.cap = null; } } }
            G.dTmr -= dtMs;
            if (G.dTmr <= 0 && !G.chal) { const fe = G.enemies.filter(e => e.st === 'FORM'); if (fe.length) startDive(fe[Math.floor(Math.random() * fe.length)]); G.dTmr = Math.max(500, (2000 - G.stage * 100) / diffMod('diveRate')); }
            const alive = G.enemies.filter(e => e.st !== 'DEAD');
            if (alive.length === 0) {
                if (G.st === 'GAME_OVER') return;
                if (G.chal && G.chalHits === G.chalTot) { G.perfectT = 2000; addScore(5000, W / 2, H / 2 - 40, '#00ffcc'); SFX.perfect(); }
                G.warpT = 1500; G.warpFlash = 50; G.stage++;
                SFX.warpJump(); if (!G.chal) { MusicEngine.play('victory'); setTimeout(() => { if (!state.disposed && MusicEngine.playing === 'victory') MusicEngine.play('gameplay'); }, 3500); }
                startStage();
            }
        }

        function startChalDive(e) {
            if (e.st !== 'FORM') return; e.st = 'DIVING'; e.dPath = { ph: 0, amp: 50 + Math.random() * 30, vx: (Math.random() - 0.5) * 130 }; e.dTmr = 4000; e.sTmr = 99999; SFX.dive();
        }

        function updateExp(dt) {
            const dtMs = dt * 1000;
            // In-place compaction avoids O(n²) splice shifting
            let elen = 0;
            for (let i = 0; i < G.exp.length; i++) { const ex = G.exp[i]; ex.t += dtMs; if (ex.t < ex.dur) G.exp[elen++] = ex; }
            G.exp.length = elen;
            // Cap particle count to prevent runaway allocations
            if (G.part.length > 200) G.part.length = 200;
            let plen = 0;
            for (let i = 0; i < G.part.length; i++) {
                const p = G.part[i];
                p.x += p.vx * dt; p.y += p.vy * dt;
                if (p.debris) { p.vy += 60 * dt; p.vx *= 0.98; p.rot += dt * 3; }
                else if (p.smoke) { p.vy -= 8 * dt; p.size += dt * 2; }
                else if (p.spark) { p.vx *= 0.95; p.vy *= 0.95; }
                p.t += dtMs; if (p.t < p.life) G.part[plen++] = p;
            }
            G.part.length = plen;
            let tlen = 0;
            for (let i = 0; i < G.trails.length; i++) { const tr = G.trails[i]; tr.x += tr.vx * dt; tr.y += tr.vy * dt; tr.t += dtMs; if (tr.t < tr.life) G.trails[tlen++] = tr; }
            G.trails.length = tlen;
            let slen = 0;
            for (let i = 0; i < G.scorePopups.length; i++) { const sp = G.scorePopups[i]; sp.y -= 40 * dt; sp.t += dtMs; if (sp.t < sp.dur) G.scorePopups[slen++] = sp; }
            G.scorePopups.length = slen;
            if (G.flashT > 0) G.flashT -= dtMs;
            if (G.damageVignetteT > 0) G.damageVignetteT -= dtMs;
            if (G.freezeT > 0) { G.freezeT -= dtMs; wrapEl.classList.add('galaxa-freeze'); if (G.freezeT <= 0) { G.freezeT = 0; wrapEl.classList.remove('galaxa-freeze'); if (G.activePU && G.activePU.type === 'freeze') { G.activePU = null; G.puTimer = 0; setPUClass(null); } } }
            if (G.warpT > 0) G.warpT -= dtMs;
            if (G.warpFlash > 0) G.warpFlash -= dtMs;
            if (G.perfectT > 0) G.perfectT -= dtMs;
            if (G.bossWarningT > 0) G.bossWarningT -= dtMs;
            if (G.comboBanner) { G.comboBanner.t += dtMs; if (G.comboBanner.t >= G.comboBanner.dur) G.comboBanner = null; }
            if (G.upgradeBanner) { G.upgradeBanner.t += dtMs; if (G.upgradeBanner.t >= G.upgradeBanner.dur) G.upgradeBanner = null; }
            if (G.slowMoT > 0) { G.slowMoT -= dtMs; if (G.slowMoT <= 0) G.timeScale = 1; }
            if (G.chromAb > 0) G.chromAb -= dtMs;
            if (G.muzzleT > 0) G.muzzleT -= dtMs;
            if (G.displayScore < G.score) { G.displayScore += Math.max(1, Math.ceil((G.score - G.displayScore) * 0.1)); if (G.displayScore > G.score) G.displayScore = G.score; }
            G.beatT += dt; const _bpm = (MusicEngine.themes[MusicEngine.playing] || {}).bpm || 120; G.beatPhase = (G.beatT % (60 / (_bpm * MusicEngine.tempoMult))) / (60 / (_bpm * MusicEngine.tempoMult));
            let prlen = 0;
            for (let i = 0; i < G.plasmaRings.length; i++) { const _pr = G.plasmaRings[i]; _pr.t += dtMs; _pr.r = (_pr.t / _pr.dur) * _pr.maxR; if (_pr.t < _pr.dur) G.plasmaRings[prlen++] = _pr; }
            G.plasmaRings.length = prlen;
            const inp2 = G.inp;
            if (inp2.l) G.shipTilt = Math.max(-0.15, G.shipTilt - dt * 2);
            else if (inp2.r) G.shipTilt = Math.min(0.15, G.shipTilt + dt * 2);
            else G.shipTilt *= Math.max(0, 1 - dt * 5);
            let dlen = 0;
            for (let i = 0; i < G.deathParts.length; i++) {
                const dp = G.deathParts[i]; dp.x += dp.vx * dt; dp.y += dp.vy * dt; dp.vy += 40 * dt; dp.rot += dt * 4; dp.t += dtMs;
                if (dp.t < dp.life) G.deathParts[dlen++] = dp;
            }
            G.deathParts.length = dlen;
            if (Math.random() < 0.008) {
                G.trails.push({ x: Math.random() * W, y: 0, vx: -30 - Math.random() * 50, vy: 100 + Math.random() * 80, life: 400, t: 0, col: '#ffffff', size: 1, spark: true });
            }
        }

        function update(dt, now) {
            if (dt > 0.1) dt = 0.1; tick++;
            const dtMs = dt * 1000;
            updateCombo(dtMs);
            if (G.inp.p && !G.inp.pp) {
                if (G.st === 'PAUSED') { G.st = G._prevSt; } else if (G.st === 'PLAYING') { G._prevSt = G.st; G.st = 'PAUSED'; G.pauseSel = 0; }
                else if (G.st === 'SETTINGS') { G.st = 'TITLE'; }
            }
            if (G.st === 'PAUSED') { updatePauseMenu(); return; }
            if (G.st === 'SETTINGS') { updateSettingsMenu(); return; }
            if (G.st === 'TITLE') {
                G.tIdle += dt * 1000;
                if (G.tIdle > TITLE_IDLE && !G.attract) { G.attract = true; G.aTmr = 0; G.score = 0; G.lives = diffMod('lives'); G.stage = 1; G.p.x = W / 2; G.p.alive = true; G.p.inv = 0; G.bul = []; G.ebul = []; G.exp = []; G.part = []; G.trails = []; mkFormation(); MusicEngine.play('title'); }
                if (G.attract) { updateAttract(dt); updateP(dt, now); updateBul(dt); updateE(dt); updateExp(dt); if (G.inp.s && !G.inp.sp) { G.attract = false; G.tIdle = 0; G.score = 0; G.lives = diffMod('lives'); G.stage = 1; G.p.dual = false; G.p.cap = null; G.weaponLv = 1; G.killCount = 0; G.displayScore = 0; G.deathParts = []; startStage(); MusicEngine.play('gameplay'); } }
                else if (G.inp.s && !G.inp.sp) { SFX.coinInsert(); G.titleParts = []; G.score = 0; G.lives = diffMod('lives'); G.stage = 1; G.p.dual = false; G.p.cap = null; G.weaponLv = 1; G.killCount = 0; G.displayScore = 0; G.deathParts = []; startStage(); MusicEngine.play('gameplay'); }
                if (!G.attract) {
                    if (Math.random() < 0.04) { const _tc = ['#4488ff','#ffcc00','#ff4444','#00ffcc','#ff88aa']; G.titleParts.push({ x: Math.random() * W, y: H + 5, vx: (Math.random()-0.5)*20, vy: -30 - Math.random()*40, life: 2500, t: 0, col: _tc[Math.floor(Math.random()*_tc.length)], size: 1 + Math.floor(Math.random()*2) }); }
                    let _tplen = 0; for (let _ti = 0; _ti < G.titleParts.length; _ti++) { const _tp = G.titleParts[_ti]; _tp.x += _tp.vx * dt; _tp.y += _tp.vy * dt; _tp.t += dt * 1000; if (_tp.t < _tp.life && _tp.y >= -10) G.titleParts[_tplen++] = _tp; } G.titleParts.length = _tplen;
                }
                return;
            }
            if (G.st === 'STAGE_INTRO') { G.sTmr -= dt * 1000; if (G.sTmr <= 0) { G.st = 'PLAYING'; mkFormation(); } return; }
            if (G.st === 'GAME_OVER') {
                G.sTmr -= dt * 1000; updateExp(dt);
                if (G.contTmr > 0) { G.contTmr -= dt; G.contCnt = Math.ceil(G.contTmr); }
                if (G.contTmr > 0 && G.inp.s && !G.inp.sp) { G.lives = diffMod('lives'); G.st = 'PLAYING'; G.p.alive = true; G.p.x = W / 2; G.p.inv = 3000; G.activePU = null; G.shieldHits = 0; G.powerups = []; G.timeScale = 1; G.freezeT = 0; G.damageVignetteT = 0; G.combo = 0; G.comboMult = 1; mkFormation(); MusicEngine.play('gameplay'); }
                if (G.sTmr <= 0 && G.contTmr <= 0) {
                    if (G.score > 0 && isHS(G.score)) { G.st = 'HIGH_SCORE'; G.ne = { ch: [65, 65, 65], pos: 0, done: false }; showHSOverlay(); }
                    else { G.st = 'TITLE'; G.tIdle = 0; showTitle(); MusicEngine.play('title'); }
                }
                return;
            }
            if (G.st === 'HIGH_SCORE') { handleName(); return; }
            if (G.st === 'PLAYING') {
                updateP(dt, now); updateBul(dt); updateE(dt); updateExp(dt);
                if (G.shkT > 0) G.shkT -= dt * 1000;
                if (G.p.cap) { G.p.cap.y -= 100 * dt; if (G.p.cap.y < G.p.y - 20) { G.p.dual = true; G.p.cap = null; SFX.rescue(); } }
                let bossAlive = false, minibossAlive = false, _aliveN = 0;
                for (let _ai = 0; _ai < G.enemies.length; _ai++) { const _ae = G.enemies[_ai]; if (_ae.st === 'DEAD') continue; _aliveN++; if (_ae.type === 'boss') bossAlive = true; else if (_ae.type === 'miniboss') { bossAlive = true; minibossAlive = true; } }
                const baseTheme = G.chal ? 'challenge' : 'gameplay';
                const bossTheme = minibossAlive ? 'miniboss' : 'boss';
                if (bossAlive && MusicEngine.playing !== bossTheme) { SFX.bossJingle(); MusicEngine.play(bossTheme); }
                else if (!bossAlive && (MusicEngine.playing === 'boss' || MusicEngine.playing === 'miniboss')) MusicEngine.play(baseTheme);
                else if (!bossAlive && MusicEngine.playing !== baseTheme && MusicEngine.playing !== 'challenge' && MusicEngine.playing !== 'victory') MusicEngine.play(baseTheme);
                if (_aliveN !== MusicEngine._lastIntensity) { MusicEngine.setIntensity(_aliveN); MusicEngine._lastIntensity = _aliveN; }
            }
        }

        function updateAttract(dt) {
            G.aTmr += dt * 1000;
            if (G.aTmr > 300) {
                G.aTmr = 0;
                const ne = G.enemies.filter(e => e.st === 'FORM');
                if (ne.length) { const tgt = ne[0]; G.inp.l = G.p.x > tgt.x + 5; G.inp.r = G.p.x < tgt.x - 5; G.inp.f = Math.abs(G.p.x - tgt.x) < 20; }
                else { G.inp.l = Math.random() < 0.2; G.inp.r = !G.inp.l && Math.random() < 0.2; G.inp.f = Math.random() < 0.2; }
            }
        }

        function updatePauseMenu() {
            const u = G.inp.u && !G.inp.up, d = G.inp.d && !G.inp.dp, f = G.inp.f && !G.inp.fp;
            if (u) G.pauseSel = (G.pauseSel + 2) % 3;
            if (d) G.pauseSel = (G.pauseSel + 1) % 3;
            if (f) {
                if (G.pauseSel === 0) { G.st = G._prevSt; }
                else if (G.pauseSel === 1) { G.st = 'TITLE'; G.tIdle = 0; G.score = 0; G.lives = diffMod('lives'); G.stage = 1; G.p.dual = false; G.p.cap = null; G.activePU = null; G.shieldHits = 0; G.timeScale = 1; G.combo = 0; G.comboMult = 1; G.weaponLv = 1; G.killCount = 0; G.puUpgrade = null; G.displayScore = 0; G.deathParts = []; setPUClass(null); showTitle(); MusicEngine.play('title'); }
                else if (G.pauseSel === 2) { G.st = 'TITLE'; G.tIdle = 0; showTitle(); MusicEngine.play('title'); }
            }
        }

        function updateSettingsMenu() {
            const u = G.inp.u && !G.inp.up, d = G.inp.d && !G.inp.dp, f = G.inp.f && !G.inp.fp, l = G.inp.l && !G.inp.lp, r = G.inp.r && !G.inp.rp;
            if (u) G.settingsSel = Math.max(0, G.settingsSel - 1);
            if (d) G.settingsSel = Math.min(3, G.settingsSel + 1);
            if (f) {
                if (G.settingsSel === 3) { G.st = 'TITLE'; }
                else if (G.settingsSel === 0) { G.muted = !G.muted; settings.mute = G.muted; MusicEngine.setMuted(G.muted); saveSettings(); }
            }
            if (l || r) {
                if (G.settingsSel === 1) { settings.diff = l ? (settings.diff === 'hard' ? 'normal' : settings.diff === 'normal' ? 'easy' : 'easy') : (settings.diff === 'easy' ? 'normal' : settings.diff === 'normal' ? 'hard' : 'hard'); saveSettings(); }
                if (G.settingsSel === 2) { settings.vol = Math.max(0, Math.min(100, settings.vol + (l ? -10 : 10))); G.vol = settings.vol / 100; if (MusicEngine.masterGain) MusicEngine.masterGain.gain.value = G.muted ? 0 : G.vol * 0.35; saveSettings(); }
            }
        }

        function renderFlame(cv, fx, fy, intensity, tk) {
            const f1 = Math.abs(Math.sin(tk * 0.35 + fx * 0.08)) * 4;
            const f2 = Math.abs(Math.sin(tk * 0.55 + fx * 0.12)) * 3;
            const f3 = Math.abs(Math.sin(tk * 0.7 + fx * 0.2)) * 2;
            cv.fillStyle = 'rgba(255,255,240,' + intensity + ')';
            cv.fillRect(Math.floor(fx), Math.floor(fy), 2, 3);
            cv.fillStyle = 'rgba(255,230,60,' + (intensity * 0.95) + ')';
            cv.fillRect(Math.floor(fx - 1), Math.floor(fy + 2), 4, 2 + Math.ceil(f1 * 0.5));
            cv.fillStyle = 'rgba(255,140,20,' + (intensity * 0.85) + ')';
            cv.fillRect(Math.floor(fx - 1), Math.floor(fy + 4), 4, 3 + Math.ceil(f1));
            cv.fillStyle = 'rgba(255,60,10,' + (intensity * 0.6) + ')';
            cv.fillRect(Math.floor(fx), Math.floor(fy + 7), 3, 2 + Math.ceil(f2));
            cv.fillStyle = 'rgba(200,40,10,' + (intensity * 0.35) + ')';
            cv.fillRect(Math.floor(fx), Math.floor(fy + 9), 2, 2 + Math.ceil(f3));
            cv.fillStyle = 'rgba(160,20,10,' + (intensity * 0.15) + ')';
            cv.fillRect(Math.floor(fx + 0.5), Math.floor(fy + 11), 1, 1 + Math.ceil(f3 * 0.5));
        }

        function renderFrame() {
            c.save(); c.setTransform(scale, 0, 0, scale, 0, 0);
            let sx = 0, sy = 0; if (G.shkT > 0) { sx = (Math.random() - 0.5) * G.shkM; sy = (Math.random() - 0.5) * G.shkM; }
            c.translate(sx, sy); c.fillStyle = '#000'; c.fillRect(-5, -5, W + 10, H + 10);
            drawNebula(c); drawStars(c, 1 / 60);
            if (G.chromAb > 0) {
                const ca = G.chromAb / 300;
                c.globalAlpha = ca * 0.12;
                c.fillStyle = '#ff0000'; c.fillRect(2, 0, W, H);
                c.fillStyle = '#0000ff'; c.fillRect(-2, 0, W, H);
                c.globalAlpha = 1;
            }
            if (G.damageVignetteT > 0) {
                const _dv = Math.min(1, G.damageVignetteT / 400) * 0.65;
                c.save();
                const _dvg = c.createRadialGradient(W * 0.5, H * 0.5, H * 0.2, W * 0.5, H * 0.5, H * 0.85);
                _dvg.addColorStop(0, 'rgba(180,0,0,0)');
                _dvg.addColorStop(1, 'rgba(220,0,0,' + _dv + ')');
                c.fillStyle = _dvg; c.fillRect(0, 0, W, H);
                c.restore();
            }
            if (G.activePU && G.activePU.type !== 'shield' && G.p && G.p.alive) {
                const egCol = PU_COL[G.activePU.type] || '#ffffff';
                const egGrad = cachedRadialGradient(c, 'powerup-edge:' + egCol, W / 2, H / 2, W * 0.25, W * 0.75, [
                    [0, 'rgba(0,0,0,0)'],
                    [1, egCol + '55']
                ]);
                c.globalAlpha = 0.5 + Math.sin(tick * 0.05) * 0.2;
                c.fillStyle = egGrad; c.fillRect(0, 0, W, H);
                c.globalAlpha = 1;
            }
            if (G.warpFlash > 0) { c.fillStyle = 'rgba(255,255,255,' + (G.warpFlash / 50) + ')'; c.fillRect(0, 0, W, H); }
            if (G.flashT > 0) { c.fillStyle = 'rgba(255,255,255,' + (G.flashT > 30 ? 0.5 : G.flashT / 60) + ')'; c.fillRect(0, 0, W, H); }
            if (G.st === 'TITLE' && !G.attract) renderTitle();
            else if (G.st === 'STAGE_INTRO') renderStageIntro();
            else if (G.st === 'SETTINGS') renderSettings();
            else if (G.st === 'PAUSED') { renderGame(); renderPause(); }
            else renderGame();
            c.restore();
        }

        function renderTitle() {
            for (const _tp of G.titleParts) { const _ta = Math.max(0, 1 - _tp.t / _tp.life); c.globalAlpha = _ta; c.fillStyle = _tp.col; c.shadowBlur = 6; c.shadowColor = _tp.col; c.fillRect(Math.floor(_tp.x), Math.floor(_tp.y), _tp.size, _tp.size); } c.globalAlpha = 1; c.shadowBlur = 0;
            c.textAlign = 'center';
            // Glowing title
            const titlePulse = 1 + Math.sin(tick * 0.04) * 0.03;
            c.save(); c.translate(W / 2, 180); c.scale(titlePulse, titlePulse);
            c.shadowBlur = 15; c.shadowColor = '#4488ff';
            c.fillStyle = '#4488ff'; c.font = 'bold 36px "Courier New",monospace'; c.fillText('GALAXA', 0, 0);
            c.shadowBlur = 0; c.restore();
            c.save(); c.translate(W / 2, 210); c.scale(titlePulse, titlePulse);
            c.shadowBlur = 10; c.shadowColor = '#ffcc00';
            c.fillStyle = '#ffcc00'; c.font = 'bold 20px "Courier New",monospace'; c.fillText('DELUXE', 0, 0);
            c.shadowBlur = 0; c.restore();
            if (Math.sin(tick * 0.08) > 0) { c.fillStyle = '#fff'; c.font = '14px "Courier New",monospace'; c.fillText(t('galaxa.insert_coin', 'PRESS START'), W / 2, 320); }
            c.fillStyle = '#4488ff'; c.font = '12px "Courier New",monospace'; c.fillText(t('galaxa.high_score', 'HIGH SCORE'), W / 2, 260);
            c.fillStyle = '#ffcc00'; c.fillText(String(G.hi).padStart(8, '0'), W / 2, 280);
            if (G.hiScores.length) { c.fillStyle = '#aaccee'; c.font = '11px "Courier New",monospace'; let y = 380; c.fillText('RANK   NAME    SCORE    STAGE', W / 2, y); y += 18; G.hiScores.forEach((h, i) => { c.fillText((i + 1) + '    ' + h.name.padEnd(3) + '   ' + String(h.score).padStart(8) + '   ' + String(h.stage).padStart(3), W / 2, y); y += 16; }); }
            c.fillStyle = '#666'; c.font = '10px "Courier New",monospace'; c.fillText('ARROWS+SPACE  GAMEPAD  SHIFT+S=SETTINGS  M=MUTE', W / 2, H - 40);
        }

        function renderStageIntro() {
            c.textAlign = 'center';
            const sc = Math.max(1, 3 - (G.sTmr / 2000) * 2);
            c.save(); c.translate(W / 2, H / 2 - 20); c.scale(sc, sc);
            c.shadowBlur = 12; c.shadowColor = '#ffcc00';
            c.fillStyle = '#ffcc00'; c.font = 'bold 24px "Courier New",monospace';
            c.fillText(G.chal ? t('galaxa.challenge_stage', 'CHALLENGE STAGE') : t('galaxa.stage', 'STAGE') + ' ' + G.stage, 0, 0);
            c.shadowBlur = 0; c.restore();
            c.fillStyle = '#fff'; c.font = '14px "Courier New",monospace'; c.fillText('READY', W / 2, H / 2 + 20);
        }

        function renderGame() {
            const p = G.p;

            // Beat-synced background pulse
            if (G.beatPhase > 0.88 && nebulaCv) {
                const _bp = (G.beatPhase - 0.88) * 8.33 * 0.06;
                c.globalAlpha = _bp; c.fillStyle = '#1a0033'; c.fillRect(0, 0, W, H); c.globalAlpha = 1;
            }

            if (G.bossWarningT > 0) {
                const flash = Math.sin(G.bossWarningT * 0.01) > 0;
                c.fillStyle = flash ? 'rgba(255,0,0,0.15)' : 'rgba(255,0,0,0.05)';
                c.fillRect(0, 0, W, H);
                c.shadowBlur = 10; c.shadowColor = '#ff0000';
                c.fillStyle = '#ff4444'; c.font = 'bold 28px "Courier New",monospace'; c.textAlign = 'center';
                c.fillText('WARNING', W / 2, H / 2 - 20);
                c.shadowBlur = 0;
                wrapEl.classList.add('galaxa-boss-warning');
            } else {
                wrapEl.classList.remove('galaxa-boss-warning');
            }

            for (const tr of G.trails) {
                if (tr.col.startsWith('rgba')) { c.fillStyle = tr.col; }
                else {
                    const alpha = Math.max(0, 1 - tr.t / tr.life);
                    c.fillStyle = tr.col; c.globalAlpha = alpha * 0.6;
                }
                c.fillRect(Math.floor(tr.x), Math.floor(tr.y), tr.size || 2, tr.size || 2);
            }
            c.globalAlpha = 1;

            for (const dp of G.deathParts) {
                const alpha = Math.max(0, 1 - dp.t / dp.life);
                c.globalAlpha = alpha;
                c.save(); c.translate(dp.x, dp.y); c.rotate(dp.rot);
                c.fillStyle = dp.col;
                c.fillRect(-dp.sz / 2, -dp.sz / 2, dp.sz, dp.sz);
                c.restore();
            }
            c.globalAlpha = 1;

            if (G.muzzleT > 0 && p.alive) {
                c.globalAlpha = G.muzzleT / 50;
                c.fillStyle = '#ffff88';
                c.fillRect(Math.floor(p.x - 2), Math.floor(p.y - 14), 4, 4);
                c.fillRect(Math.floor(p.x - 1), Math.floor(p.y - 16), 2, 2);
                c.globalAlpha = 1;
            }

            if (p.alive) {
                c.save(); c.translate(p.x, p.y); c.rotate(G.shipTilt); c.translate(-p.x, -p.y);
                if (p.inv > 0) {
                    const rpc = rainbowPC();
                    drawSp(c, SP.player, rpc, p.x - 12, p.y - 12, false);
                    if (p.dual) drawSp(c, SP.player, rpc, p.x + 28, p.y - 12, false);
                } else {
                    drawSp(c, SP.player, SP.pC, p.x - 12, p.y - 12, false);
                    if (p.dual) drawSp(c, SP.player, SP.pC, p.x + 28, p.y - 12, false);
                }
                if (p.alive) {
                    const eg = 0.5 + Math.sin(tick * 0.15) * 0.3;
                    const flameGlowCol = G.activePU && PU_COL[G.activePU.type] ? PU_COL[G.activePU.type] : '#ff6600';
                    c.shadowBlur = 8; c.shadowColor = flameGlowCol;
                    renderFlame(c, p.x - 6, p.y + 11, eg, tick);
                    renderFlame(c, p.x + 3, p.y + 11, eg, tick);
                    if (p.dual) {
                        renderFlame(c, p.x + 28, p.y + 11, eg, tick);
                        renderFlame(c, p.x + 34, p.y + 11, eg, tick);
                    }
                    c.shadowBlur = 0;
                }
                c.restore();
            }
            if (p.cap) drawSp(c, SP.player, SP.pC, p.cap.x - 12, p.cap.y - 12, false);
            if (G.shieldHits > 0 && p.alive) {
                c.strokeStyle = '#4488ff'; c.lineWidth = 1.5; c.globalAlpha = 0.5 + Math.sin(tick * 0.1) * 0.2;
                c.shadowBlur = 10; c.shadowColor = '#4488ff';
                c.beginPath(); c.arc(p.x, p.y, 18, 0, Math.PI * 2); c.stroke(); c.shadowBlur = 0; c.globalAlpha = 1;
                for (let i = 0; i < G.shieldHits; i++) {
                    const a = tick * 0.05 + i * 2.1;
                    c.fillStyle = '#4488ff'; c.fillRect(Math.floor(p.x + Math.cos(a) * 18 - 1), Math.floor(p.y + Math.sin(a) * 18 - 1), 3, 3);
                }
            }
            if (G.activePU && G.activePU.type !== 'shield' && p.alive) {
                const auraCol = PU_COL[G.activePU.type];
                const auraPulse = 0.15 + Math.sin(tick * 0.08) * 0.1;
                c.shadowBlur = 12; c.shadowColor = auraCol;
                c.strokeStyle = auraCol; c.lineWidth = 1; c.globalAlpha = auraPulse;
                c.beginPath(); c.arc(p.x, p.y, 20 + Math.sin(tick * 0.12) * 3, 0, Math.PI * 2); c.stroke();
                c.shadowBlur = 0; c.globalAlpha = 1;
            }

            if (G.activePU && (G.activePU.type === 'laser' || G.activePU.type === 'mega_laser') && G.p.alive && G.muzzleT > 0) {
                const _lAlpha = G.muzzleT / 50;
                let _nearE = null, _nearD2 = Infinity;
                for (let _li = 0; _li < G.enemies.length; _li++) { const _le = G.enemies[_li]; if (_le.st === 'DEAD' || _le.y <= 0 || _le.y >= H) continue; const _ld = (_le.x-G.p.x)*(_le.x-G.p.x)+(_le.y-G.p.y)*(_le.y-G.p.y); if (_ld < _nearD2) { _nearD2 = _ld; _nearE = _le; } }
                if (_nearE && _nearD2 < 220 * 220) {
                    const _lx1 = G.p.x, _ly1 = G.p.y - 8, _lx2 = _nearE.x, _ly2 = _nearE.y;
                    c.globalAlpha = _lAlpha * 0.7;
                    c.strokeStyle = G.activePU.type === 'mega_laser' ? '#ffffff' : '#aaccff';
                    c.lineWidth = G.activePU.type === 'mega_laser' ? 2 : 1;
                    c.shadowBlur = 8; c.shadowColor = '#4488ff';
                    c.beginPath(); c.moveTo(_lx1, _ly1);
                    for (let _li = 1; _li < 6; _li++) { const _lt = _li / 6; c.lineTo(_lx1 + (_lx2-_lx1)*_lt + (Math.random()-0.5)*16, _ly1 + (_ly2-_ly1)*_lt + (Math.random()-0.5)*16); }
                    c.lineTo(_lx2, _ly2); c.stroke();
                    c.shadowBlur = 0; c.globalAlpha = 1;
                }
            }
            for (const _pr of G.plasmaRings) {
                const _prAlpha = Math.max(0, 1 - _pr.t / _pr.dur) * 0.75;
                c.globalAlpha = _prAlpha;
                c.strokeStyle = _pr.col;
                c.lineWidth = Math.max(1, 3 * (1 - _pr.t / _pr.dur));
                c.shadowBlur = 14; c.shadowColor = _pr.col;
                c.beginPath(); c.arc(_pr.x, _pr.y, _pr.r, 0, Math.PI * 2); c.stroke();
                c.shadowBlur = 0;
            }
            c.globalAlpha = 1;
            // bullet trails (no shadow)
            for (const b of G.bul) {
                if (!b.laser) {
                    c.fillStyle = 'rgba(255,255,136,0.3)';
                    c.fillRect(Math.floor(b.x - 1), Math.floor(b.y + 3), 2, 4);
                }
            }
            // player bullets — shadow set once for the whole batch
            c.shadowColor = '#ffff88'; c.shadowBlur = 6;
            for (const b of G.bul) {
                if (!b.laser) {
                    c.fillStyle = '#ffff88';
                    c.fillRect(Math.floor(b.x - 1), Math.floor(b.y - 3), 2, 6);
                    c.globalAlpha = 0.4;
                    c.fillStyle = '#ffff44';
                    c.fillRect(Math.floor(b.x - 2), Math.floor(b.y - 4), 4, 8);
                    c.globalAlpha = 1;
                }
            }
            c.shadowBlur = 0;
            // laser bullets — shadow set once for the whole batch
            c.shadowColor = '#aaccff'; c.shadowBlur = 14;
            for (const b of G.bul) {
                if (b.laser) {
                    c.fillStyle = '#ffffff'; c.fillRect(Math.floor(b.x - 2), Math.floor(b.y - 7), 4, 14);
                    c.fillStyle = 'rgba(170,200,255,0.5)'; c.fillRect(Math.floor(b.x - 3), Math.floor(b.y - 9), 6, 18);
                    c.fillStyle = 'rgba(100,150,255,0.25)'; c.fillRect(Math.floor(b.x - 4), Math.floor(b.y - 11), 8, 22);
                }
            }
            c.shadowBlur = 0;
            // enemy bullet trails (no shadow)
            for (const b of G.ebul) {
                c.fillStyle = 'rgba(255,68,68,0.25)';
                c.fillRect(Math.floor(b.x - 1), Math.floor(b.y + 3), 2, 4);
            }
            // enemy bullets — shadow set once for the whole batch
            c.shadowColor = '#ff4444'; c.shadowBlur = 6;
            for (const b of G.ebul) {
                c.fillStyle = '#ff6666';
                c.fillRect(Math.floor(b.x - 1), Math.floor(b.y - 3), 2, 6);
                c.globalAlpha = 0.35;
                c.fillStyle = '#ff4444';
                c.fillRect(Math.floor(b.x - 2), Math.floor(b.y - 4), 4, 8);
                c.globalAlpha = 1;
            }
            c.shadowBlur = 0;

            // Boss telegraph lines — show dive path before attack
            if (G.p && G.p.alive) {
                c.setLineDash([2, 4]);
                for (const _te of G.enemies) {
                    if (_te.st !== 'DIVING' || _te.type === 'bee' || _te.sTmr === undefined || _te.sTmr > 250 || _te.sTmr < 0 || G.freezeT > 0) continue;
                    const _ta = (1 - _te.sTmr / 250) * 0.5;
                    c.globalAlpha = _ta; c.strokeStyle = '#ff4444'; c.lineWidth = 1;
                    c.beginPath(); c.moveTo(_te.x, _te.y + 8); c.lineTo(G.p.x, G.p.y - 8); c.stroke();
                }
                c.setLineDash([]); c.globalAlpha = 1;
            }

            for (const e of G.enemies) {
                if (e.st === 'DEAD') continue;
                if (e.st === 'DIVING') {
                    c.globalAlpha = 0.12;
                    let sp, cols;
if (e.type === 'bee') { sp = SP.bee[e.fr]; cols = SP.bC; } else if (e.type === 'butterfly') { sp = SP.bf[e.fr]; cols = SP.bfC; } else if (e.type === 'miniboss') { sp = e.hp <= 1 ? SP.bossCrit : e.hp <= Math.ceil(e.maxHp / 2) ? SP.bossHit : SP.boss; cols = SP.bossC; } else { sp = e.hp <= 1 ? SP.bossCrit : e.hp <= Math.ceil(e.maxHp / 2) ? SP.bossHit : SP.boss; cols = SP.bossC; }
                    drawSp(c, sp, cols, e.x - 12, e.y - 18, false);
                    drawSp(c, sp, cols, e.x - 12, e.y - 10, false);
                    c.globalAlpha = 1;
                }
                const fl = e.hitF > 0; let sp, cols;
                if (e.type === 'bee') { sp = SP.bee[e.fr]; cols = SP.bC; } else if (e.type === 'butterfly') { sp = SP.bf[e.fr]; cols = SP.bfC; } else if (e.type === 'miniboss') { sp = e.hp <= 1 ? SP.bossCrit : e.hp <= Math.ceil(e.maxHp / 2) ? SP.bossHit : SP.boss; cols = SP.bossC; } else { sp = e.hp <= 1 ? SP.bossCrit : e.hp <= Math.ceil(e.maxHp / 2) ? SP.bossHit : SP.boss; cols = SP.bossC; }
                drawSp(c, sp, cols, e.x - 12, e.y - 12, fl);
                if (!fl && G.beatPhase > 0.82 && (e.type === 'bee' || e.type === 'butterfly')) {
                    // beat glow drawn in batched pass below to avoid per-enemy shadowBlur changes
                }
            }

            // batched beat-glow pass — one shadow setup per color type instead of per enemy
            if (G.beatPhase > 0.82) {
                const _ba = (G.beatPhase - 0.82) * 5.5 * 0.25;
                c.globalAlpha = _ba;
                c.shadowBlur = 5; c.shadowColor = '#8899ff';
                for (const e of G.enemies) {
                    if (e.st === 'DEAD' || e.hitF > 0 || e.type !== 'bee') continue;
                    drawSp(c, SP.bee[e.fr], SP.bC, e.x - 12, e.y - 12, false);
                }
                c.shadowColor = '#88ffaa';
                for (const e of G.enemies) {
                    if (e.st === 'DEAD' || e.hitF > 0 || e.type !== 'butterfly') continue;
                    drawSp(c, SP.bf[e.fr], SP.bfC, e.x - 12, e.y - 12, false);
                }
                c.shadowBlur = 0; c.globalAlpha = 1;
            }

            // Enemy HP bars
            for (const _he of G.enemies) {
                if (_he.st === 'DEAD' || _he.maxHp <= 1) continue;
                const _bw = 20, _bh = 2, _bx = _he.x - 10, _by = _he.y - 18;
                c.fillStyle = '#111'; c.fillRect(_bx - 1, _by - 1, _bw + 2, _bh + 2);
                c.fillStyle = '#333'; c.fillRect(_bx, _by, _bw, _bh);
                const _hr = _he.hp / _he.maxHp;
                c.fillStyle = _hr > 0.5 ? '#44cc44' : _hr > 0.25 ? '#ffcc00' : '#ff4444';
                c.fillRect(_bx, _by, Math.ceil(_bw * _hr), _bh);
            }
            // Frozen enemy overlay
            if (G.freezeT > 0) {
                const _iceAlpha = Math.min(1, G.freezeT / 400) * 0.42;
                c.globalAlpha = _iceAlpha; c.fillStyle = '#88eeff';
                for (const _ie of G.enemies) { if (_ie.st === 'DEAD') continue; c.fillRect(Math.floor(_ie.x - 13), Math.floor(_ie.y - 13), 26, 26); }
                c.globalAlpha = 1;
                if (Math.random() < 0.08 && G.enemies.length > 0) {
                    const _fe = G.enemies[Math.floor(Math.random() * G.enemies.length)];
                    if (_fe.st !== 'DEAD') G.part.push({ x: _fe.x + (Math.random()-0.5)*18, y: _fe.y + (Math.random()-0.5)*18, vx: (Math.random()-0.5)*12, vy: -8 - Math.random()*15, life: 280, t: 0, col: '#ccf4ff', size: 1, spark: true });
                }
            }

            for (const pu of G.powerups) {
                const glow = 0.3 + Math.sin(tick * 0.1 + pu.t * 0.01) * 0.2;
                const pulse = 1 + Math.sin(tick * 0.06 + pu.t * 0.005) * 0.15;
                c.shadowBlur = 8; c.shadowColor = PU_COL[pu.type];
                c.globalAlpha = glow * 0.7; c.fillStyle = PU_COL[pu.type];
                c.beginPath(); c.arc(pu.x, pu.y, 10 * pulse, 0, Math.PI * 2); c.fill(); c.globalAlpha = 1;
                c.save(); c.translate(pu.x, pu.y); c.rotate(tick * 0.02 + pu.t * 0.001);
                c.fillStyle = PU_COL[pu.type]; c.font = 'bold 8px monospace'; c.textAlign = 'center';
                if (pu.type === 'rapid') { c.fillRect(-1, -4, 2, 8); c.fillRect(-3, -1, 6, 2); }
                else if (pu.type === 'spread') { for (let a2 = -1; a2 <= 1; a2++) c.fillRect(a2 * 3, Math.abs(a2) * 2 - 2, 2, 4); }
                else if (pu.type === 'shield') { c.strokeStyle = PU_COL.shield; c.lineWidth = 1; c.beginPath(); c.arc(0, 0, 4, 0, Math.PI * 2); c.stroke(); }
                else if (pu.type === 'speed') { c.fillRect(-3, 0, 6, 2); c.fillRect(1, -3, 2, 3); c.fillRect(1, 2, 2, 3); }
                else if (pu.type === 'magnet') { c.beginPath(); c.arc(0, 0, 3, 0, Math.PI * 2); c.stroke(); c.fillRect(-1, -4, 2, 2); }
                else if (pu.type === 'laser') { c.fillRect(-1, -5, 2, 10); }
                else if (pu.type === 'multibomb') { for (let i2 = 0; i2 < 6; i2++) { const a2 = i2 * 1.05; c.fillRect(Math.floor(Math.cos(a2) * 4), Math.floor(Math.sin(a2) * 4), 2, 2); } }
                else if (pu.type === 'timeslow') { c.beginPath(); c.arc(0, 0, 4, -Math.PI / 2, Math.PI / 2); c.stroke(); c.fillRect(0, -4, 1, 4); }
                else if (pu.type === 'pierce') { c.fillRect(-1, -5, 2, 10); c.fillRect(-3, 0, 6, 1); }
                else if (pu.type === 'homing') { c.beginPath(); c.moveTo(0, -4); c.lineTo(3, 2); c.lineTo(-3, 2); c.closePath(); c.stroke(); }
                else if (pu.type === 'supernova') { for (let i2 = 0; i2 < 8; i2++) { const a2 = i2 * 0.785; c.fillRect(Math.floor(Math.cos(a2) * 5), Math.floor(Math.sin(a2) * 5), 2, 2); } }
                else if (pu.type === 'freeze') {
                    c.strokeStyle = PU_COL.freeze; c.lineWidth = 1;
                    c.beginPath();
                    for (let i2 = 0; i2 < 6; i2++) { const a2 = i2 * Math.PI / 3; c.moveTo(0, 0); c.lineTo(Math.round(Math.cos(a2) * 5), Math.round(Math.sin(a2) * 5)); }
                    c.stroke();
                }
                else { for (let i2 = 0; i2 < 5; i2++) { const a2 = i2 * 1.26; c.fillRect(Math.floor(Math.cos(a2) * 4), Math.floor(Math.sin(a2) * 4), 2, 2); } }
                c.restore();
                c.shadowBlur = 0;
            }

            if (G.activePU && G.p.alive) {
                c.fillStyle = PU_COL[G.activePU.type]; c.font = '9px "Courier New",monospace'; c.textAlign = 'center';
                const labels = { rapid: 'RAPID FIRE', spread: 'SPREAD SHOT', shield: 'SHIELD', speed: 'SPEED BOOST', magnet: 'MAGNET', laser: 'LASER', timeslow: 'TIME SLOW' };
                const label = labels[G.activePU.type];
                if (label) c.fillText(label, p.x, p.y + 22);
                if (G.activePU.type !== 'shield' && PU_DUR[G.activePU.type]) {
                    const bw = 40, bh = 3, bx = p.x - bw / 2, by = p.y + 24;
                    c.fillStyle = '#333'; c.fillRect(bx, by, bw, bh);
                    c.fillStyle = PU_COL[G.activePU.type]; c.fillRect(bx, by, bw * (G.puTimer / PU_DUR[G.activePU.type]), bh);
                }
            }

            if (G.beam && G.beam.active) renderBeam(G.beam);

            for (const ex of G.exp) {
                const pr = ex.t / ex.dur;
                if (ex.flash) {
                    c.globalAlpha = Math.max(0, 1 - pr);
                    c.fillStyle = '#fff';
                    const fr = ex.isBoss ? 25 : 12;
                    c.beginPath(); c.arc(ex.x, ex.y, fr * (1 - pr * 0.5), 0, Math.PI * 2); c.fill();
                    c.globalAlpha = 1;
                } else if (ex.shockwave) {
                    c.globalAlpha = Math.max(0, 1 - pr) * 0.5;
                    c.strokeStyle = '#ffcc00'; c.lineWidth = Math.max(1, 3 - pr * 3);
                    c.shadowBlur = 8; c.shadowColor = '#ff8800';
                    c.beginPath(); c.arc(ex.x, ex.y, pr * 50, 0, Math.PI * 2); c.stroke();
                    c.shadowBlur = 0; c.globalAlpha = 1;
                } else {
                    const sz = ex.isBoss ? 10 + Math.floor(pr * 14) : 4 + Math.floor(pr * 4) * 3;
                    c.globalAlpha = Math.max(0, 1 - pr);
                    c.shadowBlur = ex.isBoss ? 12 : 6; c.shadowColor = '#ff8800';
                    for (let i = 0; i < sz; i++) {
                        const a = (i / sz) * Math.PI * 2 + ex.seed, d = (ex.isBoss ? 8 : 3) * (1 + pr * 2.5);
                        const ci = Math.floor(pr * 3); c.fillStyle = ['#ffcc00', '#ff8800', '#ff4444'][ci < 3 ? ci : 2];
                        c.fillRect(Math.floor(ex.x + Math.cos(a) * d), Math.floor(ex.y + Math.sin(a) * d), ex.isBoss ? 3 : 2, ex.isBoss ? 3 : 2);
                    }
                    c.shadowBlur = 0; c.globalAlpha = 1;
                }
            }
            for (const pt of G.part) {
                const alpha = Math.max(0, 1 - pt.t / pt.life);
                c.globalAlpha = alpha;
                if (pt.spark) {
                    c.shadowBlur = 4; c.shadowColor = pt.col;
                    c.fillStyle = pt.col; c.fillRect(Math.floor(pt.x), Math.floor(pt.y), 1, 1);
                    c.shadowBlur = 0;
                } else if (pt.debris) {
                    c.save(); c.translate(pt.x, pt.y); c.rotate(pt.rot);
                    c.fillStyle = pt.col; c.fillRect(-pt.size / 2, -pt.size / 2, pt.size, pt.size);
                    c.restore();
                } else if (pt.smoke) {
                    c.globalAlpha = alpha * 0.35; c.fillStyle = pt.col;
                    c.fillRect(Math.floor(pt.x), Math.floor(pt.y), pt.size || 3, pt.size || 3);
                } else {
                    c.fillStyle = pt.col;
                    if (pt.size >= 3) { c.shadowBlur = 6; c.shadowColor = pt.col; }
                    c.fillRect(Math.floor(pt.x), Math.floor(pt.y), pt.size || 2, pt.size || 2);
                    c.shadowBlur = 0;
                }
            } c.globalAlpha = 1;
            for (const sp of G.scorePopups) {
                const _spAlpha = Math.max(0, 1 - sp.t / sp.dur);
                const _spScale = sp.big ? (1 + Math.max(0, 1 - sp.t / 200) * 0.7) : 1;
                c.globalAlpha = _spAlpha;
                c.save(); c.translate(Math.floor(sp.x), Math.floor(sp.y)); c.scale(_spScale, _spScale);
                if (sp.big) { c.shadowBlur = 8; c.shadowColor = sp.col; }
                c.fillStyle = sp.col;
                c.font = (sp.big ? 'bold 13px' : 'bold 10px') + ' "Courier New",monospace';
                c.textAlign = 'center'; c.fillText(sp.text, 0, 0);
                if (sp.big) c.shadowBlur = 0;
                c.restore();
            } c.globalAlpha = 1;

            if (G.perfectT > 0) {
                c.shadowBlur = 8; c.shadowColor = '#00ffcc';
                c.fillStyle = '#00ffcc'; c.font = 'bold 22px "Courier New",monospace'; c.textAlign = 'center';
                c.fillText(t('galaxa.perfect_bonus', 'PERFECT BONUS') + ' +5000', W / 2, H / 2 - 40);
                c.shadowBlur = 0;
            }

            if (G.comboBanner) {
                const alpha = Math.max(0, 1 - G.comboBanner.t / G.comboBanner.dur);
                const sc = 1 + (G.comboBanner.t < 200 ? (200 - G.comboBanner.t) / 200 * 0.5 : 0);
                c.save(); c.globalAlpha = alpha;
                c.translate(W / 2, H / 2 + 30); c.scale(sc, sc);
                c.shadowBlur = 12; c.shadowColor = '#ffcc00';
                c.fillStyle = '#ffcc00'; c.font = 'bold 20px "Courier New",monospace'; c.textAlign = 'center';
                c.fillText(G.comboBanner.text, 0, 0);
                c.fillStyle = '#fff'; c.font = 'bold 14px "Courier New",monospace';
                c.fillText('x' + G.comboBanner.mult, 0, 20);
                c.shadowBlur = 0; c.restore();
            }

            if (G.upgradeBanner) {
                const alpha = Math.max(0, 1 - G.upgradeBanner.t / G.upgradeBanner.dur);
                const sc = 1 + (G.upgradeBanner.t < 300 ? (300 - G.upgradeBanner.t) / 300 * 0.8 : 0);
                c.save(); c.globalAlpha = alpha;
                c.translate(W / 2, H / 2 + 60); c.scale(sc, sc);
                c.shadowBlur = 15; c.shadowColor = PU_UPGRADE_COL[G.upgradeBanner.type] || '#fff';
                c.fillStyle = PU_UPGRADE_COL[G.upgradeBanner.type] || '#fff'; c.font = 'bold 18px "Courier New",monospace'; c.textAlign = 'center';
                c.fillText(G.upgradeBanner.text, 0, 0);
                c.shadowBlur = 0; c.restore();
            }

            if (G.slowMoT > 0) {
                c.fillStyle = 'rgba(255,255,255,0.03)'; c.fillRect(0, 0, W, H);
            }

            let boss = null; for (let _bi = 0; _bi < G.enemies.length; _bi++) { const _be = G.enemies[_bi]; if ((_be.type === 'boss' || _be.type === 'miniboss') && _be.st !== 'DEAD') { boss = _be; break; } }
            if (boss) {
                const barW = 220, barH = 8, barX = W / 2 - barW / 2, barY = 40;
                c.fillStyle = '#222'; c.fillRect(barX - 1, barY - 1, barW + 2, barH + 2);
                c.fillStyle = '#333'; c.fillRect(barX, barY, barW, barH);
                const hpRatio = boss.hp / boss.maxHp;
                const grad = c.createLinearGradient(barX, barY, barX + barW * hpRatio, barY);
                grad.addColorStop(0, hpRatio > 0.5 ? '#ff4444' : '#ff2222'); grad.addColorStop(1, hpRatio > 0.5 ? '#ff8844' : '#ff4444');
                c.shadowBlur = 8; c.shadowColor = hpRatio > 0.3 ? '#ff4444' : '#ff0000';
                c.fillStyle = grad; c.fillRect(barX, barY, barW * hpRatio, barH);
                if (hpRatio <= 0.3 && Math.sin(tick * 0.15) > 0) {
                    c.strokeStyle = '#ff0000'; c.lineWidth = 1; c.strokeRect(barX - 2, barY - 2, barW + 4, barH + 4);
                }
                c.shadowBlur = 0;
                c.fillStyle = '#fff'; c.font = 'bold 11px "Courier New",monospace'; c.textAlign = 'center';
                c.fillText(boss.type === 'miniboss' ? 'MINI-BOSS' : 'BOSS', W / 2, barY - 4);
            }

            if (G.timeScale < 1) {
                c.fillStyle = 'rgba(170,68,255,0.08)'; c.fillRect(0, 0, W, H);
                wrapEl.classList.add('galaxa-timeslow');
            } else {
                wrapEl.classList.remove('galaxa-timeslow');
            }

            renderHUD();
            if (G.st === 'GAME_OVER') {
                c.fillStyle = 'rgba(0,0,0,0.5)'; c.fillRect(0, H / 2 - 40, W, 80);
                c.fillStyle = '#ff4444'; c.font = 'bold 24px "Courier New",monospace'; c.textAlign = 'center'; c.fillText(t('galaxa.game_over', 'GAME OVER'), W / 2, H / 2 - 10);
                if (G.contTmr > 0) {
                    c.fillStyle = '#ffcc00'; c.font = '16px "Courier New",monospace';
                    c.fillText(t('galaxa.continue_prompt', 'CONTINUE?') + ' ' + G.contCnt, W / 2, H / 2 + 20);
                    const _cAngle = (G.contTmr / 10) * Math.PI * 2 - Math.PI / 2;
                    c.strokeStyle = '#ffcc00'; c.lineWidth = 3; c.globalAlpha = 0.7;
                    c.shadowBlur = 6; c.shadowColor = '#ffcc00';
                    c.beginPath(); c.arc(W / 2, H / 2 + 46, 16, -Math.PI / 2, _cAngle); c.stroke();
                    c.shadowBlur = 0; c.lineWidth = 1; c.globalAlpha = 1;
                }
            }
        }

        function renderBeam(tb) {
            c.shadowBlur = 8; c.shadowColor = '#4488ff';
            c.strokeStyle = '#4488ff'; c.lineWidth = 2; c.globalAlpha = 0.55;
            const w = 20 + Math.sin(tick * 0.15) * 8;
            c.beginPath();
            for (let i = 0; i < 8; i++) { const t2 = i / 8, y1 = tb.y + t2 * tb.h, y2 = tb.y + (t2 + 0.125) * tb.h, ww = w * (1 - t2 * 0.3); c.moveTo(tb.x - ww / 2, y1); c.lineTo(tb.x - ww * 0.4, y2); c.moveTo(tb.x + ww / 2, y1); c.lineTo(tb.x + ww * 0.4, y2); }
            c.stroke();
            c.globalAlpha = 1; c.shadowBlur = 0;
        }

        function renderHUD() {
            c.fillStyle = '#4488ff'; c.font = '12px "Courier New",monospace'; c.textAlign = 'left'; c.fillText(t('galaxa.score', 'SCORE'), 10, 16);
            c.fillStyle = '#fff'; c.fillText(String(G.displayScore).padStart(8, '0'), 10, 32);
            if (G.comboMult > 1) {
                c.fillStyle = '#ffcc00'; c.font = 'bold 11px "Courier New",monospace';
                c.fillText('x' + G.comboMult, 10, 44);
            }
            c.fillStyle = '#4488ff'; c.textAlign = 'right'; c.fillText(t('galaxa.high_score', 'HIGH SCORE'), W - 10, 16);
            c.fillStyle = '#ffcc00'; c.fillText(String(G.hi).padStart(8, '0'), W - 10, 32);
            const stagePulse = G.warpT > 0 ? 1 + Math.sin(tick * 0.15) * 0.3 : 1;
            c.save(); c.translate(W / 2, 16); c.scale(stagePulse, stagePulse);
            c.fillStyle = '#4488ff'; c.font = 'bold 12px "Courier New",monospace'; c.textAlign = 'center';
            c.fillText(t('galaxa.stage', 'STAGE') + ' ' + G.stage, 0, 0);
            c.restore();
            if (G.chal) {
                let _cr = 0; for (let _ci = 0; _ci < G.enemies.length; _ci++) if (G.enemies[_ci].st !== 'DEAD') _cr++;
                c.fillStyle = '#ff8800'; c.font = 'bold 10px "Courier New",monospace'; c.textAlign = 'center';
                c.fillText(t('galaxa.challenge_stage', 'CHALLENGE') + ' ' + _cr + '/' + G.chalTot, W / 2, 28);
            }
            let alive2cnt = 0; for (let _hi = 0; _hi < G.enemies.length; _hi++) { const _hh = G.enemies[_hi]; if (_hh.st !== 'DEAD' && _hh.type !== 'boss' && _hh.type !== 'miniboss') alive2cnt++; }
            if (alive2cnt > 0 && alive2cnt <= 5) {
                c.fillStyle = '#888'; c.font = '10px "Courier New",monospace'; c.textAlign = 'center';
                c.fillText(alive2cnt + ' LEFT', W / 2, G.chal ? 38 : 28);
            }
            if (G.weaponLv > 1) {
                c.fillStyle = '#44cc88'; c.font = '9px "Courier New",monospace'; c.textAlign = 'left';
                c.fillText('W' + G.weaponLv, 10, 54);
            }
            if (G.activePU && G.activePU.type !== 'shield' && PU_DUR[G.activePU.type]) {
                const barW = W * 0.6, barH = 3, barX = W / 2 - barW / 2, barY = 4;
                const ratio = G.puTimer / PU_DUR[G.activePU.type];
                c.fillStyle = '#222'; c.fillRect(barX, barY, barW, barH);
                c.fillStyle = PU_COL[G.activePU.type]; c.fillRect(barX, barY, barW * ratio, barH);
                if (ratio < 0.3 && Math.sin(tick * 0.2) > 0) { c.fillStyle = '#fff'; c.fillRect(barX, barY, barW * ratio, barH); }
            }
            for (let i = 0; i < Math.min(G.lives, 5); i++) drawSp(c, SP.player, SP.pC, 10 + i * 26, H - 24, false);
            if (G.activePU) {
                const puIconX = W - 20, puIconY = H - 20;
                const expiring = G.activePU.type !== 'shield' && PU_DUR[G.activePU.type] && G.puTimer < 2000;
                if (!expiring || Math.sin(tick * 0.2) > 0) {
                    c.fillStyle = PU_COL[G.activePU.type] || '#fff'; c.font = 'bold 9px monospace'; c.textAlign = 'right';
                    c.fillText(G.activePU.type.toUpperCase().substring(0, 4), puIconX, puIconY);
                }
            }
        }

        function renderPause() {
            if (G.st !== 'PAUSED') return;
            c.fillStyle = 'rgba(0,0,0,0.75)'; c.fillRect(0, 0, W, H);
            c.textAlign = 'center'; c.fillStyle = '#ffcc00'; c.font = 'bold 26px "Courier New",monospace';
            c.shadowBlur = 10; c.shadowColor = '#ffcc00';
            c.fillText(t('galaxa.paused', 'PAUSED'), W / 2, H / 2 - 60);
            c.shadowBlur = 0;
            c.fillStyle = '#aaccee'; c.font = '12px "Courier New",monospace';
            c.fillText(t('galaxa.score', 'SCORE') + ': ' + G.score + '  ' + t('galaxa.stage', 'STAGE') + ': ' + G.stage, W / 2, H / 2 - 35);
            const items = [t('galaxa.resume', 'RESUME'), t('galaxa.restart', 'RESTART'), t('galaxa.quit', 'QUIT')];
            items.forEach((it, i) => {
                c.fillStyle = i === G.pauseSel ? '#ffcc00' : '#888'; c.font = i === G.pauseSel ? 'bold 16px "Courier New",monospace' : '14px "Courier New",monospace';
                if (i === G.pauseSel) { c.shadowBlur = 6; c.shadowColor = '#ffcc00'; }
                c.fillText(it, W / 2, H / 2 + i * 30);
                c.shadowBlur = 0;
            });
        }

        function renderSettings() {
            c.fillStyle = 'rgba(0,0,0,0.88)'; c.fillRect(0, 0, W, H);
            c.textAlign = 'center'; c.fillStyle = '#ffcc00'; c.font = 'bold 22px "Courier New",monospace';
            c.shadowBlur = 10; c.shadowColor = '#ffcc00';
            c.fillText(t('galaxa.settings', 'SETTINGS'), W / 2, 120);
            c.shadowBlur = 0;
            const items = [
                { label: t('galaxa.sound', 'SOUND'), val: G.muted ? 'OFF' : 'ON' },
                { label: t('galaxa.difficulty', 'DIFFICULTY'), val: t('galaxa.' + settings.diff, settings.diff.toUpperCase()) },
                { label: t('galaxa.volume', 'VOLUME'), val: settings.vol + '%' },
                { label: t('galaxa.quit', 'QUIT'), val: '' }
            ];
            items.forEach((it, i) => {
                const sel = i === G.settingsSel;
                c.fillStyle = sel ? '#ffcc00' : '#888'; c.font = sel ? 'bold 14px "Courier New",monospace' : '12px "Courier New",monospace';
                if (sel) { c.shadowBlur = 6; c.shadowColor = '#ffcc00'; }
                c.fillText(it.label + (it.val ? ': ' + it.val : ''), W / 2, 180 + i * 40);
                c.shadowBlur = 0;
                if (i === 2) {
                    const bw = 200, bh = 8, bx = W / 2 - bw / 2, by = 200 + i * 40;
                    c.fillStyle = '#222'; c.fillRect(bx, by, bw, bh);
                    c.fillStyle = '#4488ff'; c.fillRect(bx, by, bw * settings.vol / 100, bh);
                    if (sel) { c.strokeStyle = '#4488ff'; c.lineWidth = 1; c.strokeRect(bx - 1, by - 1, bw + 2, bh + 2); }
                }
            });
            c.fillStyle = '#666'; c.font = '10px "Courier New",monospace';
            c.fillText('\u2191\u2193 select  \u2190\u2192 change  ENTER confirm', W / 2, 360);
            c.fillText('ARROWS+SPACE  GAMEPAD D-PAD+A', W / 2, 380);
        }

        function showTitle() { overlayEl.classList.remove('active'); overlayEl.innerHTML = ''; MusicEngine.play('title'); }
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
            if (k === 'm' || k === 'M') { G.muted = !G.muted; settings.mute = G.muted; MusicEngine.setMuted(G.muted); saveSettings(); }
            if ((k === 'S' || k === 's') && G.st === 'TITLE' && !G.attract && !G.kb.d) { G.st = 'SETTINGS'; G.settingsSel = 0; }
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
            savePrev(); pollGP(); mergeInput();
            update(dt, now);
            renderFrame();
            rafId = requestAnimationFrame(loop);
        }

        document.addEventListener('keydown', onKey);
        document.addEventListener('keyup', onKeyUp);
        const ro = new ResizeObserver(() => { if (!state.disposed) resize(); });
        ro.observe(host); resize();
        loadHS().then(() => { showTitle(); rafId = requestAnimationFrame(loop); });

        state.dispose = function () {
            state.disposed = true; cancelAnimationFrame(rafId); MusicEngine.stop();
            document.removeEventListener('keydown', onKey); document.removeEventListener('keyup', onKeyUp);
            ro.disconnect(); radialGradientCache.clear(); if (actx) try { actx.close(); } catch (e) {}
            setPUClass(null); wrapEl.classList.remove('galaxa-boss-warning');
            instances.delete(windowId);
        };
    }

    function dispose(windowId) { const s = instances.get(windowId); if (s && s.dispose) s.dispose(); }
    window.GalaxaDeluxe = { render, dispose };
})();
