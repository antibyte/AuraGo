(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createAudio = function (ctx) {
        function audio() {
            if (!ctx.actx) try { ctx.actx = new (window.AudioContext || window.webkitAudioContext)(); } catch (e) { return null; }
            if (ctx.actx && ctx.actx.state === 'suspended') ctx.actx.resume();
            if (ctx.actx && !ctx.masterCompressor) {
                ctx.masterCompressor = ctx.actx.createDynamicsCompressor();
                ctx.masterCompressor.threshold.value = -12;
                ctx.masterCompressor.knee.value = 10;
                ctx.masterCompressor.ratio.value = 4;
                ctx.masterCompressor.attack.value = 0.003;
                ctx.masterCompressor.release.value = 0.15;
                ctx.masterCompressor.connect(ctx.actx.destination);
                try {
                    ctx.reverbNode = ctx.actx.createConvolver();
                    const rate = ctx.actx.sampleRate, length = Math.floor(rate * 0.4);
                    const impulse = ctx.actx.createBuffer(2, length, rate);
                    for (let ch = 0; ch < 2; ch++) { const d = impulse.getChannelData(ch); for (let i = 0; i < length; i++) d[i] = (Math.random() * 2 - 1) * Math.pow(1 - i / length, 3); }
                    ctx.reverbNode.buffer = impulse;
                    ctx.reverbGain = ctx.actx.createGain(); ctx.reverbGain.gain.value = 0.12;
                    ctx.reverbNode.connect(ctx.reverbGain); ctx.reverbGain.connect(ctx.masterCompressor);
                } catch (_) { ctx.reverbNode = null; ctx.reverbGain = null; }
            }
            return ctx.actx;
        }

        function beep(type, f0, f1, dur, vol, panX) {
            const a = audio(); if (!a || ctx.G.muted) return;
            const o = a.createOscillator(), g = a.createGain();
            o.type = type; o.frequency.setValueAtTime(f0, a.currentTime);
            if (f1 !== f0) o.frequency.linearRampToValueAtTime(f1, a.currentTime + dur);
            g.gain.setValueAtTime(ctx.G.vol * vol, a.currentTime);
            g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + dur + 0.02);
            if (panX !== undefined && a.createStereoPanner) {
                const p = a.createStereoPanner();
                p.pan.value = Math.max(-1, Math.min(1, (panX / (ctx.W / 2)) - 1));
                o.connect(g).connect(p).connect(a.destination);
            } else {
                o.connect(g).connect(a.destination);
            }
            o.start(); o.stop(a.currentTime + dur + 0.02);
        }

        function noise(dur, vol, freq, panX) {
            const a = audio(); if (!a || ctx.G.muted) return;
            const buf = a.createBuffer(1, a.sampleRate * dur, a.sampleRate), d = buf.getChannelData(0);
            for (let i = 0; i < d.length; i++) d[i] = (Math.random() * 2 - 1) * (1 - i / d.length);
            const s = a.createBufferSource(), f = a.createBiquadFilter(), g = a.createGain();
            s.buffer = buf; f.type = 'lowpass'; f.frequency.value = freq || 2000;
            g.gain.setValueAtTime(ctx.G.vol * vol, a.currentTime);
            g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + dur);
            if (panX !== undefined && a.createStereoPanner) {
                const p = a.createStereoPanner();
                p.pan.value = Math.max(-1, Math.min(1, (panX / (ctx.W / 2)) - 1));
                s.connect(f).connect(g).connect(p).connect(a.destination);
            } else {
                s.connect(f).connect(g).connect(a.destination);
            }
            s.start();
        }

        function schedNoise(startTime, dur, vol, freq, dest, panX) {
            const a = audio(); if (!a || ctx.G.muted) return null;
            const buf = a.createBuffer(1, Math.max(1, Math.floor(a.sampleRate * dur)), a.sampleRate), d = buf.getChannelData(0);
            for (let i = 0; i < d.length; i++) d[i] = (Math.random() * 2 - 1) * (1 - i / d.length);
            const s = a.createBufferSource(), f = a.createBiquadFilter(), g = a.createGain();
            s.buffer = buf; f.type = freq > 4000 ? 'highpass' : 'lowpass'; f.frequency.value = freq || 2000;
            g.gain.setValueAtTime(ctx.G.vol * vol, startTime);
            g.gain.exponentialRampToValueAtTime(0.001, startTime + dur);
            const target = dest || a.destination;
            if (panX !== undefined && a.createStereoPanner) {
                const p = a.createStereoPanner();
                p.pan.value = Math.max(-1, Math.min(1, (panX / (ctx.W / 2)) - 1));
                s.connect(f).connect(g).connect(p).connect(target);
            } else {
                s.connect(f).connect(g).connect(target);
            }
            s.start(startTime); s.stop(startTime + dur + 0.01);
            return s;
        }

        function pv() { return 0.95 + Math.random() * 0.1; }
        function vv() { return 0.9 + Math.random() * 0.2; }

        // NEW: Audio ducking — temporarily lower music master gain on loud SFX
        let duckTimer = 0, duckTarget = 1;
        function duckMusic(amount, durMs) {
            if (!ctx.MusicEngine.masterGain) return;
            const a = audio(); if (!a) return;
            duckTarget = Math.max(0.2, 1 - amount);
            ctx.MusicEngine.masterGain.gain.linearRampToValueAtTime(ctx.G.muted ? 0 : ctx.G.vol * 0.35 * duckTarget, a.currentTime + 0.04);
            duckTimer = durMs;
        }
        function updateDuck(dtMs) {
            if (duckTimer > 0) { duckTimer -= dtMs; if (duckTimer <= 0) { duckTimer = 0; const a = audio(); if (a && ctx.MusicEngine.masterGain) ctx.MusicEngine.masterGain.gain.linearRampToValueAtTime(ctx.G.muted ? 0 : ctx.G.vol * 0.35, a.currentTime + 0.2); } }
        }
        ctx.duckMusic = duckMusic;
        ctx.updateDuck = updateDuck;

        const SFX = {
            shoot(panX) { const _p = pv(); beep('sine', 800 * _p, 1200 * _p, 0.08, 0.3 * vv(), panX); beep('square', 400 * _p, 200 * _p, 0.05, 0.08 * vv(), panX); },
            laserShoot(panX) { const _p = pv(), _v = vv(); beep('sine', 1200 * _p, 400 * _p, 0.15, 0.25 * _v, panX); beep('sawtooth', 800 * _p, 200 * _p, 0.1, 0.15 * _v, panX); noise(0.08, 0.1 * _v, 3000, panX); },
            dive(panX) { const _p = pv(); beep('sawtooth', 600 * _p, 200 * _p, 0.3, 0.15 * vv(), panX); },
            eExplode(panX) { const _p = pv(), _v = vv(); noise(0.15, 0.4 * _v, 2000, panX); noise(0.08, 0.2 * _v, 5000, panX); beep('sine', 200 * _p, 80 * _p, 0.1, 0.2 * _v, panX); beep('triangle', 60 * _p, 30 * _p, 0.15, 0.15 * _v, panX); },
            bigExplode(panX) { const _p = pv(), _v = vv(); noise(0.3, 0.5 * _v, 1500, panX); noise(0.15, 0.3 * _v, 4000, panX); beep('sine', 80 * _p, 40 * _p, 0.25, 0.4 * _v, panX); noise(0.2, 0.2 * _v, 600, panX); },
            pExplode(panX) { const _p = pv(), _v = vv(); noise(0.4, 0.6 * _v, 1200, panX); noise(0.2, 0.35 * _v, 3000, panX); beep('sine', 60 * _p, 60 * _p, 0.3, 0.5 * _v, panX); beep('sawtooth', 100 * _p, 30 * _p, 0.2, 0.3 * _v, panX); },
            stage() { [523, 659, 784, 1047].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.2, 0.25), i * 120); }); },
            challenge() { [440, 554, 659, 880, 1109, 1319].forEach((f, i) => { setTimeout(() => beep('square', f, f, 0.15, 0.2), i * 80); }); },
            extra() { beep('sine', 1200, 1200, 0.2, 0.3); },
            rescue() { beep('sine', 880, 880, 0.2, 0.25); setTimeout(() => beep('sine', 1100, 1100, 0.2, 0.25), 100); },
            beam() { beep('sawtooth', 200, 200, 0.5, 0.15); },
            perfect() { [523, 659, 784, 1047, 1319, 1568].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.15, 0.3), i * 60); }); },
            puCollect(panX) { [600, 800, 1000, 1200].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.06, 0.2, panX), i * 40); }); },
            bomb(panX) { const _v = vv(); noise(0.5, 0.7 * _v, 800, panX); noise(0.3, 0.4 * _v, 200, panX); beep('sawtooth', 100, 50, 0.4, 0.5 * _v, panX); },
            combo(n) { const _p = pv(); beep('sine', (440 + n * 110) * _p, (440 + n * 110) * _p, 0.12, (0.25 + n * 0.05) * vv()); },
            bossWarning() { beep('sawtooth', 440, 220, 0.5, 0.3); setTimeout(() => beep('sawtooth', 440, 220, 0.5, 0.3), 500); },
            shieldHit() { const _p = pv(), _v = vv(); beep('triangle', 2000 * _p, 4000 * _p, 0.05, 0.3 * _v); beep('sine', 3000 * _p, 1500 * _p, 0.08, 0.2 * _v); },
            respawn() { beep('sine', 200, 800, 0.3, 0.25); setTimeout(() => beep('sine', 600, 1200, 0.2, 0.2), 80); },
            shieldBreak() { noise(0.2, 0.5 * vv(), 3000); beep('sawtooth', 200 * pv(), 100, 0.15, 0.4 * vv()); },
            bossJingle() { [220, 262, 330, 220, 165, 220].forEach((f, i) => { setTimeout(() => beep('sawtooth', f, f, 0.15, 0.2 + i * 0.02), i * 100); }); },
            stageClear() { [523, 659, 784, 1047, 1319, 1568, 2093].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.15, 0.2), i * 80); }); },
            puUpgrade(panX) { [800, 1000, 1200, 1400, 1600].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.05, 0.25, panX), i * 30); }); },
            weaponUp() { [600, 800, 1000, 1200].forEach((f, i) => { setTimeout(() => beep('triangle', f, f, 0.08, 0.2), i * 60); }); },
            homingLock(panX) { const _p = pv(); beep('sine', 1200 * _p, 1200 * _p, 0.04, 0.15, panX); beep('sine', 1800 * _p, 1800 * _p, 0.03, 0.1, panX); },
            supernova(panX) { const _v = vv(); noise(0.8, 0.9 * _v, 600, panX); noise(0.5, 0.5 * _v, 100, panX); beep('sawtooth', 80, 40, 0.6, 0.7 * _v, panX); beep('sine', 200, 50, 0.5, 0.5 * _v, panX); },
            miniBossWarning() { beep('sawtooth', 330, 165, 0.4, 0.3); setTimeout(() => beep('sawtooth', 330, 165, 0.4, 0.3), 400); },
            bossHitSFX(panX) { const _p = pv(), _v = vv(); beep('sawtooth', 280 * _p, 60 * _p, 0.12, 0.45 * _v, panX); noise(0.1, 0.35 * _v, 900, panX); },
            warpJump() { beep('sawtooth', 180, 3600, 0.35, 0.45); beep('sine', 90, 3000, 0.28, 0.35); setTimeout(() => noise(0.15, 0.3, 4000), 250); },
            coinInsert() { beep('triangle', 440, 880, 0.06, 0.45); setTimeout(() => beep('triangle', 880, 1760, 0.06, 0.45), 70); },
            comboBreak() { beep('sawtooth', 440 * pv(), 200, 0.18, 0.2 * vv()); },
            killStreak() { [880, 1100, 1320, 1760].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.09, 0.28), i * 55); }); },
            freeze(panX) { const _p = pv(), _v = vv(); beep('sine', 1200 * _p, 3600 * _p, 0.06, 0.35 * _v, panX); beep('triangle', 900 * _p, 2800 * _p, 0.05, 0.28 * _v, panX); setTimeout(() => { beep('triangle', 400, 180, 0.09, 0.15, panX); noise(0.12, 0.18, 7000, panX); }, 100); },
            powerupExpire() { beep('sawtooth', 880, 440, 0.07, 0.4); },
            enemyHitSfx(panX) { const _p = pv(), _v = vv(); beep('sine', 380 * _p, 180 * _p, 0.03, 0.25 * _v, panX); beep('sine', 550 * _p, 300 * _p, 0.02, 0.15 * _v, panX); },
            stalkerDive(panX) { const _p = pv(), _v = vv(); beep('sawtooth', 900 * _p, 300 * _p, 0.2, 0.2 * _v, panX); noise(0.1, 0.15 * _v, 5000, panX); },
            hunterDive(panX) { const _p = pv(), _v = vv(); beep('sawtooth', 1200 * _p, 180 * _p, 0.35, 0.28 * _v, panX); noise(0.15, 0.22 * _v, 3500, panX); beep('square', 400 * _p, 120 * _p, 0.12, 0.18 * _v, panX); },
            hunterShot(panX) { const _p = pv(), _v = vv(); beep('sawtooth', 700 * _p, 350 * _p, 0.1, 0.22 * _v, panX); beep('square', 500 * _p, 200 * _p, 0.06, 0.14 * _v, panX); },
            spinnerShot(panX) { const _p = pv(), _v = vv(); beep('sine', 1400 * _p, 2200 * _p, 0.07, 0.2 * _v, panX); beep('triangle', 900 * _p, 1500 * _p, 0.05, 0.12 * _v, panX); },
            bomberDrop(panX) { const _p = pv(), _v = vv(); beep('sawtooth', 300 * _p, 80 * _p, 0.12, 0.2 * _v, panX); noise(0.08, 0.12 * _v, 800, panX); },
            lasherShot(panX) { const _p = pv(), _v = vv(); beep('sine', 600 * _p, 1800 * _p, 0.14, 0.22 * _v, panX); beep('triangle', 400 * _p, 1200 * _p, 0.08, 0.15 * _v, panX); },
            sniperShot(panX) { const _p = pv(), _v = vv(); beep('sine', 1800 * _p, 600 * _p, 0.08, 0.25 * _v, panX); beep('square', 1200 * _p, 400 * _p, 0.05, 0.12 * _v, panX); },
            comboMilestone(n, panX) { [880, 1100, 1320, 1760, 2200].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.08, 0.2 + n * 0.03, panX), i * 40); }); },
            shieldReflect(panX) { const _p = pv(); beep('triangle', 3000 * _p, 1500 * _p, 0.06, 0.3, panX); beep('sine', 2000 * _p, 4000 * _p, 0.04, 0.2, panX); },
            closeCall(panX) { const _p = pv(); noise(0.06, 0.12 * vv(), 6000, panX); beep('sine', 1500 * _p, 800 * _p, 0.04, 0.1, panX); },
            envAmbience(theme) { if (theme === 'storm') noise(2, 0.03, 400); else if (theme === 'blackhole') { beep('sine', 40, 40, 2, 0.04); beep('sine', 55, 55, 2, 0.03); } else if (theme === 'crystal') beep('sine', 2400, 2000, 0.3, 0.02); },
            rageMode(panX) { beep('sawtooth', 300, 600, 0.25, 0.35, panX); beep('square', 200, 400, 0.2, 0.2, panX); noise(0.15, 0.2, 4000, panX); },
            chainLightning(hop, panX) { const _r = 1 + hop * 0.15; beep('sine', 1800 * _r, 2400 * _r, 0.06, 0.3, panX); noise(0.04, 0.2, 6000, panX); beep('triangle', 1200 * _r, 800 * _r, 0.04, 0.15, panX); },
            orbitalShieldHit(panX) { beep('sine', 3000, 1500, 0.08, 0.3, panX); beep('triangle', 2000, 800, 0.12, 0.25, panX); noise(0.06, 0.15, 5000, panX); },
            relicActivate() { beep('sine', 440, 440, 0.3, 0.25); beep('triangle', 660, 660, 0.25, 0.15); beep('sine', 880, 880, 0.2, 0.1); setTimeout(() => beep('sine', 1100, 1100, 0.15, 0.08), 150); },
            nearMiss(panX) { noise(0.05, 0.15, 7000, panX); beep('sine', 80, 60, 0.12, 0.2); },
            bossPhaseTrans() { beep('sawtooth', 100, 50, 0.6, 0.4); noise(0.4, 0.3, 800); setTimeout(() => beep('sawtooth', 200, 100, 0.3, 0.25), 200); },
            mutationStart() { beep('sawtooth', 180, 90, 0.4, 0.3); noise(0.3, 0.15, 3000); beep('sine', 60, 40, 0.5, 0.2); },
            comboExtend() { [600, 800, 1000, 1200].forEach((f, i) => { setTimeout(() => beep('triangle', f, f, 0.06, 0.2), i * 30); }); },
            scoreRoll() { beep('triangle', 1200, 1400, 0.02, 0.08); },
            deathBloom() { noise(0.3, 0.4, 8000); beep('sine', 2000, 500, 0.25, 0.3); },
            envWind() { noise(1.5, 0.04, 600); beep('sine', 30, 30, 1.5, 0.02); },
            // NEW: Parry success — crisp upward "shwing" with reverb tail
            parrySuccess(panX) { const _p = pv(), _v = vv(); beep('triangle', 1800 * _p, 4200 * _p, 0.06, 0.35 * _v, panX); beep('sine', 2400 * _p, 3600 * _p, 0.05, 0.25 * _v, panX); setTimeout(() => { beep('sine', 1200, 800, 0.1, 0.18 * _v, panX); }, 60); },
            parryStart(panX) { const _p = pv(); beep('sine', 2200 * _p, 2600 * _p, 0.04, 0.18, panX); beep('triangle', 1400 * _p, 2000 * _p, 0.03, 0.12, panX); },
            parryMiss(panX) { const _p = pv(); beep('sawtooth', 400 * _p, 200 * _p, 0.08, 0.12, panX); },
            // NEW: Super activation — build sweep + impact boom, unique per ship type
            superActivate(shipType, panX) {
                const _v = vv();
                if (shipType === 'classic') { beep('sawtooth', 200, 1400, 0.3, 0.4 * _v, panX); beep('square', 100, 300, 0.25, 0.2 * _v, panX); setTimeout(() => noise(0.4, 0.5 * _v, 800, panX), 250); }
                else if (shipType === 'interceptor') { beep('sine', 300, 3000, 0.3, 0.4 * _v, panX); beep('sawtooth', 150, 2000, 0.25, 0.25 * _v, panX); setTimeout(() => noise(0.15, 0.3 * _v, 5000, panX), 200); }
                else if (shipType === 'heavy') { beep('sawtooth', 80, 60, 0.5, 0.6 * _v, panX); noise(0.5, 0.5 * _v, 400, panX); beep('square', 120, 80, 0.3, 0.3 * _v, panX); }
                else if (shipType === 'stealth') { beep('sine', 600, 1800, 0.35, 0.35 * _v, panX); beep('triangle', 900, 2400, 0.3, 0.25 * _v, panX); setTimeout(() => { beep('sine', 1200, 1200, 0.2, 0.2 * _v, panX); beep('sine', 1500, 1500, 0.2, 0.15 * _v, panX); beep('sine', 1800, 1800, 0.2, 0.12 * _v, panX); }, 150); }
                else { beep('sawtooth', 150, 800, 0.4, 0.45 * _v, panX); noise(0.3, 0.4 * _v, 600, panX); }
            },
            // NEW: Biome reveal stinger
            biomeReveal() { [262, 330, 392, 523, 659].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.18, 0.22), i * 90); }); setTimeout(() => { beep('triangle', 523, 523, 0.4, 0.2); beep('sine', 784, 784, 0.4, 0.12); }, 500); },
            // NEW: Bonus sub-stage jingle
            bonusStart() { [523, 659, 784, 659, 784, 988].forEach((f, i) => { setTimeout(() => beep('square', f, f, 0.1, 0.2), i * 70); }); },
            bonusEnd(rating) { const base = rating === 'S' ? [784, 988, 1175, 1568] : rating === 'A' ? [659, 784, 988, 1319] : [523, 659, 784, 1047]; base.forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.12, 0.25), i * 80); }); },
            // NEW: UI sounds
            uiHover() { beep('sine', 1000, 1000, 0.03, 0.1); },
            uiClick() { beep('triangle', 1200, 1200, 0.04, 0.15); },
            uiBack() { beep('triangle', 600, 400, 0.05, 0.12); },
            uiToggle() { beep('square', 800, 1000, 0.04, 0.1); },
            shopBuy() { beep('sine', 880, 1200, 0.08, 0.2); setTimeout(() => beep('sine', 1320, 1320, 0.08, 0.2), 80); },
            shopError() { beep('sawtooth', 200, 120, 0.12, 0.2); },
            achievementUnlock() { [523, 659, 784, 1047].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.15, 0.28), i * 70); }); },
            // NEW: Whoosh for dives/parries/supers — noise + bandpass sweep
            whoosh(speed, panX) { const _p = pv(); const dur = Math.max(0.1, Math.min(0.4, 60 / Math.max(50, speed))); const a = audio(); if (!a || ctx.G.muted) return; const buf = a.createBuffer(1, Math.floor(a.sampleRate * dur), a.sampleRate), d = buf.getChannelData(0); for (let i = 0; i < d.length; i++) d[i] = (Math.random() * 2 - 1) * (1 - i / d.length); const s = a.createBufferSource(), f = a.createBiquadFilter(), g = a.createGain(); s.buffer = buf; f.type = 'bandpass'; f.frequency.setValueAtTime(400 * _p, a.currentTime); f.frequency.linearRampToValueAtTime(3000 * _p, a.currentTime + dur); g.gain.setValueAtTime(ctx.G.vol * 0.2, a.currentTime); g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + dur); if (panX !== undefined && a.createStereoPanner) { const p = a.createStereoPanner(); p.pan.value = Math.max(-1, Math.min(1, (panX / (ctx.W / 2)) - 1)); s.connect(f).connect(g).connect(p).connect(a.destination); } else { s.connect(f).connect(g).connect(a.destination); } s.start(); },
            // NEW: Per-enemy-type explosion with layered sub-bass + mid crack + debris noise
            eExplodeTyped(type, size, panX) { const _p = pv(), _v = vv(); const big = size === 'big' || type === 'boss' || type === 'miniboss'; noise(big ? 0.3 : 0.15, big ? 0.5 * _v : 0.4 * _v, big ? 1500 : 2000, panX); noise(big ? 0.15 : 0.08, big ? 0.3 * _v : 0.2 * _v, big ? 4000 : 5000, panX); const baseF = type === 'boss' ? 60 : type === 'miniboss' ? 70 : type === 'kamikaze' ? 100 : 200; beep('sine', baseF * _p, baseF * 0.4 * _p, big ? 0.25 : 0.1, big ? 0.4 * _v : 0.2 * _v, panX); beep('triangle', baseF * 0.3 * _p, baseF * 0.15 * _p, big ? 0.3 : 0.15, big ? 0.25 * _v : 0.15 * _v, panX); if (big) { noise(0.2, 0.2 * _v, 600, panX); beep('sawtooth', 80 * _p, 40 * _p, 0.4, 0.3 * _v, panX); } },
            // NEW: Super stinger audio
            superChargeStart(panX) { const _v = vv(); beep('sawtooth', 80, 800, 0.4, 0.35 * _v, panX); beep('triangle', 60, 600, 0.35, 0.25 * _v, panX); noise(0.3, 0.2 * _v, 1500, panX); },
            superNovaBarrage(panX) { const _v = vv(); beep('sawtooth', 200 * pv(), 1400, 0.3, 0.45 * _v, panX); beep('square', 100, 300, 0.25, 0.25 * _v, panX); setTimeout(() => noise(0.4, 0.5 * _v, 800, panX), 250); },
            superPhaseDash(panX) { const _v = vv(); beep('sine', 300, 3000, 0.3, 0.4 * _v, panX); beep('sawtooth', 150, 2000, 0.25, 0.25 * _v, panX); setTimeout(() => noise(0.15, 0.3 * _v, 5000, panX), 200); },
            superAegisCannon(panX) { const _v = vv(); beep('sawtooth', 80, 60, 0.5, 0.6 * _v, panX); noise(0.5, 0.5 * _v, 400, panX); beep('square', 120, 80, 0.3, 0.3 * _v, panX); },
            superShadowClone(panX) { const _v = vv(); beep('sine', 600, 1800, 0.35, 0.35 * _v, panX); beep('triangle', 900, 2400, 0.3, 0.25 * _v, panX); setTimeout(() => { beep('sine', 1200, 1200, 0.2, 0.2 * _v, panX); beep('sine', 1500, 1500, 0.2, 0.15 * _v, panX); beep('sine', 1800, 1800, 0.2, 0.12 * _v, panX); }, 150); },
            // NEW: Stage archetype cues
            archetypeSwarmLoop() { const _v = vv(); beep('square', 100, 100, 0.1, 0.18 * _v); setTimeout(() => beep('square', 100, 100, 0.1, 0.18 * _v), 500); },
            archetypeEscortPad() { const _v = vv(); beep('sawtooth', 110, 220, 1.0, 0.15 * _v); beep('triangle', 165, 330, 1.0, 0.1 * _v); },
            archetypeAsteroidWarning(panX) { const _p = pv(); beep('sine', 1800 * _p, 600 * _p, 0.15, 0.25, panX); beep('triangle', 1200 * _p, 400 * _p, 0.1, 0.15, panX); },
            // NEW: Risk-It mode toggle UI sound
            riskItToggle() { beep('square', 600, 900, 0.05, 0.18); setTimeout(() => beep('triangle', 900, 1200, 0.05, 0.18), 50); },
            // NEW: Rank jingles
            rankJingleSplus() { const _v = vv(); [784, 988, 1175, 1568, 2093, 2637].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.12, 0.3 * _v), i * 60); }); },
            rankJingleS() { const _v = vv(); [659, 784, 988, 1319, 1568].forEach((f, i) => { setTimeout(() => beep('triangle', f, f, 0.1, 0.25 * _v), i * 70); }); },
            rankJingleA() { const _v = vv(); [523, 659, 784, 1047].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.1, 0.22 * _v), i * 80); }); },
            rankJingleB() { beep('triangle', 440, 440, 0.15, 0.2); setTimeout(() => beep('triangle', 660, 660, 0.15, 0.2), 150); },
            rankJingleC() { const _v = vv(); beep('sawtooth', 220, 110, 0.4, 0.25 * _v); beep('triangle', 165, 82, 0.3, 0.2 * _v); }
        };

        const MusicEngine = {
            nodes: [], masterGain: null, playing: null, loopId: 0, tempoMult: 1, stopped: false, intensity: 5,
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
                deep_boss: {
                    bpm: 170,
                    bass: { wave: 'sawtooth', vol: 0.06, notes: [{ f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 87, d: 0.5 }, { f: 87, d: 0.5 }, { f: 87, d: 0.5 }, { f: 87, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 82, d: 1 }, { f: 98, d: 1 }, { f: 110, d: 1 }, { f: 131, d: 1 }, { f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 131, d: 0.5 }, { f: 131, d: 0.5 }, { f: 110, d: 0.5 }, { f: 110, d: 0.5 }, { f: 98, d: 0.5 }, { f: 98, d: 0.5 }, { f: 82, d: 0.5 }, { f: 82, d: 0.5 }] },
                    lead: { wave: 'square', vol: 0.04, notes: [{ f: 196, d: 0.5 }, { f: 233, d: 0.5 }, { f: 294, d: 0.5 }, { f: 196, d: 0.5 }, { f: 220, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 220, d: 0.5 }, { f: 233, d: 0.5 }, { f: 294, d: 0.5 }, { f: 392, d: 0.5 }, { f: 233, d: 0.5 }, { f: 220, d: 0.5 }, { f: 262, d: 0.5 }, { f: 330, d: 0.5 }, { f: 220, d: 0.5 }, { f: 392, d: 0.5 }, { f: 466, d: 0.5 }, { f: 392, d: 0.5 }, { f: 294, d: 0.5 }, { f: 262, d: 0.5 }, { f: 392, d: 0.5 }, { f: 466, d: 0.5 }, { f: 392, d: 0.5 }, { f: 294, d: 0.5 }, { f: 233, d: 0.5 }, { f: 294, d: 0.5 }, { f: 196, d: 0.5 }, { f: 233, d: 1 }, { f: 294, d: 1 }, { f: 196, d: 1 }, { f: 220, d: 1 }] },
                    harmony: { wave: 'sine', vol: 0.025, notes: [{ f: 147, d: 1 }, { f: 175, d: 1 }, { f: 196, d: 1 }, { f: 220, d: 1 }, { f: 233, d: 1 }, { f: 147, d: 1 }, { f: 220, d: 1 }, { f: 262, d: 1 }, { f: 196, d: 1 }, { f: 233, d: 1 }, { f: 294, d: 1 }, { f: 392, d: 1 }, { f: 262, d: 1 }, { f: 294, d: 1 }, { f: 196, d: 1 }, { f: 220, d: 1 }] },
                    arpeggio: { wave: 'sawtooth', vol: 0.015, notes: [{ f: 98, d: 0.25 }, { f: 147, d: 0.25 }, { f: 196, d: 0.25 }, { f: 147, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }, { f: 98, d: 0.25 }, { f: 147, d: 0.25 }, { f: 196, d: 0.25 }, { f: 147, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }, { f: 131, d: 0.25 }, { f: 196, d: 0.25 }, { f: 262, d: 0.25 }, { f: 196, d: 0.25 }, { f: 110, d: 0.25 }, { f: 165, d: 0.25 }, { f: 220, d: 0.25 }, { f: 165, d: 0.25 }] },
                    percussion: { vol: 0.06, notes: [{ f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -3, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -3, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -3, d: 0.5 }, { f: -1, d: 0.25 }, { f: -2, d: 0.25 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }, { f: -1, d: 0.5 }, { f: -2, d: 0.5 }, { f: -3, d: 0.5 }, { f: -2, d: 0.5 }] }
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
                const prevTheme = this.playing;
                const prevGain = this.masterGain;
                this.stop(); this.playing = theme; this.stopped = false;
                const a = audio(); if (!a) return;
                if (prevGain) { prevGain.gain.linearRampToValueAtTime(0, a.currentTime + 0.3); setTimeout(() => { try { prevGain.disconnect(); } catch (_) {} }, 350); }
                if (prevTheme && prevTheme !== theme && !ctx.G.muted) {
                    const stingerVol = ctx.G.vol * 0.15;
                    if (theme === 'boss' || theme === 'miniboss' || theme === 'deep_boss') {
                        beep('sawtooth', 220, 110, 0.3, stingerVol);
                        setTimeout(() => beep('sawtooth', 165, 82, 0.2, stingerVol), 150);
                    } else if (theme === 'gameplay' && (prevTheme === 'boss' || prevTheme === 'victory')) {
                        [523, 659, 784].forEach((f, i) => setTimeout(() => beep('sine', f, f, 0.1, stingerVol), i * 60));
                    } else if (theme === 'victory') {
                        [784, 988, 1175, 1568].forEach((f, i) => setTimeout(() => beep('sine', f, f, 0.12, stingerVol), 2800 + i * 150));
                    }
                }
                this.masterGain = a.createGain();
                this.masterGain.gain.value = ctx.G.muted ? 0 : ctx.G.vol * 0.35;
                this.masterGain.connect(ctx.masterCompressor || a.destination);
                const th = this.themes[theme]; if (!th) return;
                const beatDur = (60 / th.bpm) / this.tempoMult;
                const loop = theme !== 'gameover' && theme !== 'victory';
                const schedVoices = () => {
                    if (this.stopped || !this.masterGain) return;
                    this.nodes = [];
                    let maxDur = 0;
                    const percBoost = this.intensity <= 2 ? 0.7 : this.intensity <= 4 ? 1 : this.intensity <= 7 ? 1.3 : 1.6;
                    for (const vn of ['bass', 'lead', 'harmony', 'arpeggio']) {
                        const voice = th[vn]; if (!voice) continue;
                        const iFactor = this.intensity <= 2 ? (vn === 'bass' || vn === 'harmony' ? 1 : vn === 'lead' ? 0.2 : 0)
                            : this.intensity <= 4 ? (vn === 'arpeggio' ? 0.3 : 1)
                            : this.intensity <= 7 ? (vn === 'arpeggio' ? 0.7 : 1) : 1;
                        if (iFactor <= 0) continue;
                        let offset = 0;
                        for (const n of voice.notes) {
                            if (n.f > 0) {
                                const o = a.createOscillator(), g = a.createGain();
                                o.type = voice.wave; o.frequency.value = n.f;
                                g.gain.setValueAtTime(voice.vol * (ctx.G.muted ? 0 : 1) * iFactor, a.currentTime + offset);
                                g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + offset + n.d * beatDur + 0.01);
                                if (vn === 'bass') {
                                    const filt = a.createBiquadFilter(); filt.type = 'lowpass';
                                    filt.frequency.setValueAtTime(250, a.currentTime + offset);
                                    filt.frequency.linearRampToValueAtTime(700, a.currentTime + offset + n.d * beatDur * 0.3);
                                    filt.frequency.linearRampToValueAtTime(180, a.currentTime + offset + n.d * beatDur);
                                    o.connect(filt).connect(g).connect(this.masterGain);
                                } else { o.connect(g).connect(this.masterGain); }
                                if (ctx.reverbNode && (vn === 'lead' || vn === 'harmony')) {
                                    const rvbSend = a.createGain(); rvbSend.gain.value = 0.08;
                                    g.connect(rvbSend); rvbSend.connect(ctx.reverbNode);
                                }
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
                                const o = a.createOscillator(), g = a.createGain();
                                o.frequency.setValueAtTime(150, a.currentTime + offset);
                                o.frequency.exponentialRampToValueAtTime(40, a.currentTime + offset + 0.08);
                                g.gain.setValueAtTime(th.percussion.vol * 1.8 * percBoost, a.currentTime + offset);
                                g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + offset + 0.15);
                                o.connect(g).connect(this.masterGain);
                                o.start(a.currentTime + offset); o.stop(a.currentTime + offset + 0.16);
                                this.nodes.push(o);
                                const ns = schedNoise(a.currentTime + offset, 0.06, th.percussion.vol * 0.6, 120, this.masterGain);
                                if (ns) this.nodes.push(ns);
                            }
                            else if (n.f === -2) {
                                const ns = schedNoise(a.currentTime + offset, 0.04, th.percussion.vol * 0.7, 9000, this.masterGain);
                                if (ns) this.nodes.push(ns);
                                const ns2 = schedNoise(a.currentTime + offset, 0.02, th.percussion.vol * 0.3, 3000, this.masterGain);
                                if (ns2) this.nodes.push(ns2);
                            }
                            else if (n.f === -3) {
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
            setMuted(m) { if (this.masterGain) this.masterGain.gain.value = m ? 0 : ctx.G.vol * 0.35; },
            setIntensity(level) {
                this.intensity = level;
                const volMult = 1 + Math.min(level, 5) * 0.08;
                if (this.masterGain) this.masterGain.gain.value = ctx.G.muted ? 0 : ctx.G.vol * 0.35 * volMult;
            },
            addLayer(layerId, layerDef) {
                if (!this.layerGains) this.layerGains = {};
                const a = audio(); if (!a) return null;
                const gainNode = a.createGain();
                gainNode.gain.value = 0;
                gainNode.connect(this.masterGain || a.destination);
                this.layerGains[layerId] = { gain: gainNode, def: layerDef, active: false };
                return this.layerGains[layerId];
            },
            removeLayer(themeId, layerId) {
                if (!this.layerGains || !this.layerGains[layerId]) return;
                try { this.layerGains[layerId].gain.disconnect(); } catch (_) {}
                delete this.layerGains[layerId];
            },
            setLayerGain(themeId, layerId, value) {
                if (!this.layerGains || !this.layerGains[layerId]) return;
                this.layerGains[layerId].gain.gain.value = Math.min(0.04, value);
            },
            transpose(semitones) {
                if (!this.semitoneOffset) this.semitoneOffset = 0;
                this.semitoneOffset = semitones;
                if (this.playing) {
                    const wasPlaying = this.playing;
                    this.play(wasPlaying);
                }
            }
        };

        ctx.audio = audio;
        ctx.beep = beep;
        ctx.noise = noise;
        ctx.schedNoise = schedNoise;
        ctx.SFX = SFX;
        ctx.MusicEngine = MusicEngine;
    };
})();
