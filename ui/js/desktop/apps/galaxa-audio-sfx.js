(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};

    GC.createAudioSfx = function (ctx) {
        const pv = () => (ctx.pv ? ctx.pv() : 1);
        const vv = () => (ctx.vv ? ctx.vv() : 1);
        const beep = (...args) => ctx.beep(...args);
        const noise = (...args) => ctx.noise(...args);
        const audio = () => ctx.audio();
                const SFX = {
            shootTyped(kind, panX) {
                if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                if (kind === 'laser' || kind === 'mega_laser') {
                    beep('sine', 1200 * _p, 400 * _p, 0.12, 0.22 * _v, panX);
                    beep('sawtooth', 800 * _p, 200 * _p, 0.08, 0.12 * _v, panX);
                    noise(0.06, 0.08 * _v, 3000, panX);
                    return;
                }
                if (kind === 'spread' || kind === 'mega_spread') {
                    beep('square', 700 * _p, 1100 * _p, 0.06, 0.2 * _v, panX);
                    beep('triangle', 500 * _p, 900 * _p, 0.05, 0.1 * _v, panX);
                    return;
                }
                if (kind === 'rapid' || kind === 'ultra_rapid') {
                    beep('square', 900 * _p, 1400 * _p, 0.04, 0.18 * _v, panX);
                    return;
                }
                beep('sine', 800 * _p, 1200 * _p, 0.07, 0.28 * _v, panX);
                beep('square', 400 * _p, 200 * _p, 0.04, 0.07 * _v, panX);
            },
            shoot(panX) { this.shootTyped('normal', panX); },
            laserShoot(panX) { this.shootTyped('laser', panX); },
            playerHurt(panX) {
                if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                beep('sawtooth', 320 * _p, 120 * _p, 0.08, 0.28 * _v, panX);
                noise(0.06, 0.14 * _v, 1800, panX);
            },
            dive(panX) { const _p = pv(); beep('sawtooth', 600 * _p, 200 * _p, 0.3, 0.15 * vv(), panX); },
            eExplode(panX) { const _p = pv(), _v = vv(); noise(0.15, 0.4 * _v, 2000, panX); noise(0.08, 0.2 * _v, 5000, panX); beep('sine', 200 * _p, 80 * _p, 0.1, 0.2 * _v, panX); beep('triangle', 60 * _p, 30 * _p, 0.15, 0.15 * _v, panX); },
            bigExplode(panX) { const _p = pv(), _v = vv(); noise(0.3, 0.5 * _v, 1500, panX); noise(0.15, 0.3 * _v, 4000, panX); beep('sine', 80 * _p, 40 * _p, 0.25, 0.4 * _v, panX); noise(0.2, 0.2 * _v, 600, panX); },
            pExplode(panX) { const _p = pv(), _v = vv(); noise(0.4, 0.6 * _v, 1200, panX); noise(0.2, 0.35 * _v, 3000, panX); noise(0.06, 0.18 * _v, 4500, panX); beep('sine', 60 * _p, 60 * _p, 0.3, 0.5 * _v, panX); beep('sawtooth', 100 * _p, 30 * _p, 0.2, 0.3 * _v, panX); },
            stage() { [523, 659, 784, 1047].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.2, 0.25), i * 120); }); },
            challenge() { [440, 554, 659, 880, 1109, 1319].forEach((f, i) => { setTimeout(() => beep('square', f, f, 0.15, 0.2), i * 80); }); },
            extra() { beep('sine', 1200, 1200, 0.2, 0.3); },
            rescue() { beep('sine', 880, 880, 0.2, 0.25); setTimeout(() => beep('sine', 1100, 1100, 0.2, 0.25), 100); },
            beam() { beep('sawtooth', 200, 200, 0.5, 0.15); },
            perfect() { [523, 659, 784, 1047, 1319, 1568].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.15, 0.3), i * 60); }); },
            puCollectRarity(rarity, panX) {
                if (ctx.G.muted) return;
                const tiers = {
                    common: [600, 800, 1000],
                    uncommon: [700, 900, 1100, 1300],
                    rare: [800, 1000, 1200, 1600],
                    legendary: [880, 1175, 1568, 2093, 2637]
                };
                const notes = tiers[rarity] || tiers.common;
                const gap = rarity === 'legendary' ? 55 : rarity === 'rare' ? 45 : 35;
                notes.forEach((f, i) => setTimeout(() => beep('sine', f, f, 0.06, 0.18 + i * 0.02, panX), i * gap));
                if (rarity === 'rare' || rarity === 'legendary') this.powerupChime(panX);
            },
            weaponArm(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                beep('triangle', 500 * _p, 1400 * _p, 0.07, 0.22 * _v, panX);
                beep('square', 300 * _p, 900 * _p, 0.05, 0.12 * _v, panX);
            },
            puCollect(panX) { this.puCollectRarity('common', panX); },
            bomb(panX) { const _v = vv(); noise(0.5, 0.7 * _v, 800, panX); noise(0.3, 0.4 * _v, 200, panX); beep('sawtooth', 100, 50, 0.4, 0.5 * _v, panX); },
            combo(n) { const _p = pv(); beep('sine', (440 + n * 110) * _p, (440 + n * 110) * _p, 0.12, (0.25 + n * 0.05) * vv()); },
            bossWarning() { beep('sawtooth', 440, 220, 0.5, 0.3); setTimeout(() => beep('sawtooth', 440, 220, 0.5, 0.3), 500); },
            shieldHit() { const _p = pv(), _v = vv(); beep('triangle', 2000 * _p, 4000 * _p, 0.05, 0.3 * _v); beep('sine', 3000 * _p, 1500 * _p, 0.08, 0.2 * _v); },
            respawn() { beep('sine', 200, 800, 0.3, 0.25); setTimeout(() => beep('sine', 600, 1200, 0.2, 0.2), 80); },
            shieldBreak() { noise(0.2, 0.5 * vv(), 3000); beep('sawtooth', 200 * pv(), 100, 0.15, 0.4 * vv()); },
            bossJingle() { [220, 262, 330, 220, 165, 220].forEach((f, i) => { setTimeout(() => beep('sawtooth', f, f, 0.15, 0.2 + i * 0.02), i * 100); }); },
            stageClear() { this.stageClearFanfare(); },
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
            enemyHitSfx(type, panX) {
                if (ctx.G.muted) return;
                let hitType = type, px = panX;
                if (px === undefined) { hitType = 'bee'; px = type; }
                const _p = pv(), _v = vv();
                const base = hitType === 'shield_bee' ? 520 : hitType === 'boss' || hitType === 'miniboss' ? 220 : 380;
                beep('sine', base * _p, (base * 0.45) * _p, 0.03, 0.22 * _v, px);
                beep('square', (base * 1.3) * _p, (base * 0.7) * _p, 0.02, 0.1 * _v, px);
            },
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
            parrySuccess(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                beep('triangle', 2000 * _p, 4800 * _p, 0.05, 0.38 * _v, panX);
                beep('square', 1600 * _p, 2800 * _p, 0.04, 0.2 * _v, panX);
                setTimeout(() => beep('sine', 1400, 900, 0.08, 0.16 * _v, panX), 50);
            },
            parryStart(panX) { const _p = pv(); beep('sine', 2200 * _p, 2600 * _p, 0.04, 0.18, panX); beep('triangle', 1400 * _p, 2000 * _p, 0.03, 0.12, panX); },
            parryMiss(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                beep('sawtooth', 280 * _p, 90 * _p, 0.09, 0.16 * _v, panX);
                noise(0.05, 0.1 * _v, 700, panX);
            },
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
            rankJingleC() { const _v = vv(); beep('sawtooth', 220, 110, 0.4, 0.25 * _v); beep('triangle', 165, 82, 0.3, 0.2 * _v); },
            // NEW: Boss phase stingers
            bossPhaseTransition() { const _v = vv(); beep('sawtooth', 100, 50, 0.6, 0.4 * _v); noise(0.4, 0.3 * _v, 800); setTimeout(() => beep('sawtooth', 200, 100, 0.3, 0.25 * _v), 200); },
            bossPhaseCrescendo() { const _v = vv(); beep('sawtooth', 200, 800, 0.5, 0.35 * _v); beep('triangle', 400, 1600, 0.4, 0.25 * _v); noise(0.3, 0.2 * _v, 4000); },
            bossDeathStinger() { const _v = vv(); [110, 82, 65, 49, 41].forEach((f, i) => { setTimeout(() => { beep('sawtooth', f, f * 0.5, 0.6, 0.35 * _v); beep('sine', f * 2, f, 0.5, 0.25 * _v); }, i * 400); }); },
            bossKillFanfare(panX) { if (ctx.G.muted) return;
                const _v = vv();
                if (ctx.duckMusic) ctx.duckMusic(0.15, 1400);
                noise(0.08, 0.05 * _v, 200, panX);
                setTimeout(() => {
                    if (ctx.G.muted) return;
                    if (this.bossDeathStinger) this.bossDeathStinger();
                    else this.bossDeathRumble(panX);
                    if (this.subThump) this.subThump(panX);
                }, 120);
            },
            // NEW: Ambient loops (one-shot, called by biome system)
            ambientStormWind() { noise(1.5, 0.04, 400); beep('sine', 30, 30, 1.5, 0.02); },
            ambientBlackholeDrone() { beep('sine', 40, 40, 2, 0.04); beep('sine', 55, 55, 2, 0.03); },
            ambientCrystalSparkle() { beep('sine', 2400, 2000, 0.3, 0.02); },
            // NEW: Deep sub-bass thump layered under boss/big explosions (paired with fxBossShockwave)
            subThump(panX) { const _p = pv(), _v = vv(); beep('sine', 48 * _p, 26 * _p, 0.5, 0.55 * _v, panX); beep('triangle', 36 * _p, 20 * _p, 0.4, 0.3 * _v, panX); noise(0.35, 0.3 * _v, 250, panX); },
            // NEW: Bright powerup chime with convolver reverb send (rare/legendary pickups)
            powerupChime(panX) {
                const a = audio(); if (!a || ctx.G.muted) return;
                const _v = vv();
                const notes = [1760, 2637, 5274];
                for (let i = 0; i < notes.length; i++) {
                    const o = a.createOscillator(), g = a.createGain();
                    o.type = 'sine'; o.frequency.setValueAtTime(notes[i] * pv(), a.currentTime + i * 0.05);
                    const peak = ctx.G.vol * (0.22 - i * 0.05) * _v;
                    g.gain.setValueAtTime(0.0001, a.currentTime + i * 0.05);
                    g.gain.exponentialRampToValueAtTime(Math.max(0.001, peak), a.currentTime + i * 0.05 + 0.015);
                    g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + i * 0.05 + 0.35);
                    let tail = g;
                    if (panX !== undefined && a.createStereoPanner) { const p = a.createStereoPanner(); p.pan.value = Math.max(-1, Math.min(1, (panX / (ctx.W / 2)) - 1)); g.connect(p); tail = p; }
                    o.connect(g); tail.connect(a.destination);
                    if (ctx.reverbNode) { const rvbSend = a.createGain(); rvbSend.gain.value = 0.25; g.connect(rvbSend); rvbSend.connect(ctx.reverbNode); }
                    o.start(a.currentTime + i * 0.05); o.stop(a.currentTime + i * 0.05 + 0.4);
                }
            },
            // NEW: Warp whoosh — long bandpass-filtered noise sweep for stage transitions (paired with warp streaks)
            warpWhoosh() {
                const a = audio(); if (!a || ctx.G.muted) return;
                const dur = 0.7, _v = vv();
                const buf = a.createBuffer(1, Math.floor(a.sampleRate * dur), a.sampleRate), d = buf.getChannelData(0);
                for (let i = 0; i < d.length; i++) d[i] = (Math.random() * 2 - 1) * Math.sin((i / d.length) * Math.PI);
                const s = a.createBufferSource(), f = a.createBiquadFilter(), g = a.createGain();
                s.buffer = buf; f.type = 'bandpass'; f.Q.value = 1.2;
                f.frequency.setValueAtTime(250, a.currentTime);
                f.frequency.exponentialRampToValueAtTime(3200, a.currentTime + dur * 0.55);
                f.frequency.exponentialRampToValueAtTime(900, a.currentTime + dur);
                g.gain.setValueAtTime(ctx.G.vol * 0.28 * _v, a.currentTime);
                g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + dur);
                s.connect(f).connect(g).connect(a.destination);
                if (ctx.reverbNode) { const rvbSend = a.createGain(); rvbSend.gain.value = 0.3; g.connect(rvbSend); rvbSend.connect(ctx.reverbNode); }
                s.start(); s.stop(a.currentTime + dur + 0.01);
            },
            // NEW: Combo milestone arpeggio — base pitch rises a minor third per combo level
            comboRiser(level, panX) { const base = 440 * Math.pow(1.1892, Math.min(level, 8)); [1, 1.26, 1.5, 2, 2.52].forEach((iv, i) => { setTimeout(() => beep('sine', base * iv, base * iv, 0.09, (0.2 + level * 0.02) * vv(), panX), i * 45); }); },
            megaComboStinger(level, panX) { if (ctx.G.muted) return;
                if (ctx.duckMusic) ctx.duckMusic(0.45, 700);
                this.comboRiser(level, panX);
                const base = 523 * Math.pow(1.122, Math.min(level, 6));
                [0, 4, 7, 12].forEach((semi, i) => {
                    const f = base * Math.pow(2, semi / 12);
                    setTimeout(() => beep('square', f, f, 0.1, 0.22 + i * 0.03, panX), 220 + i * 55);
                });
            },
            // NEW: Short high sparkle twinkle for precision score bonuses (headshots)
            sparkleTwinkle(panX) { const _p = pv(); beep('sine', 2400 * _p, 2400 * _p, 0.05, 0.16, panX); setTimeout(() => beep('sine', 3600 * _p, 3600 * _p, 0.06, 0.14, panX), 55); },
            // NEW: Graze — brief high-pitched "zip" when bullets skim the player
            graze(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                beep('triangle', 3000 * _p, 4200 * _p, 0.035, 0.16 * _v, panX);
                beep('sine', 2400 * _p, 3200 * _p, 0.03, 0.1 * _v, panX);
                noise(0.025, 0.07 * _v, 9000, panX);
            },
            // NEW: Boss entrance rumble — deep ominous stinger + sub-bass growl
            bossEntrance(panX) { const _p = pv(), _v = vv(); beep('sawtooth', 60 * _p, 30 * _p, 0.8, 0.45 * _v, panX); beep('triangle', 40 * _p, 20 * _p, 0.6, 0.3 * _v, panX); noise(0.6, 0.25 * _v, 120, panX); setTimeout(() => { beep('sawtooth', 120 * _p, 80 * _p, 0.3, 0.3 * _v, panX); noise(0.2, 0.15 * _v, 300, panX); }, 300); },
            // NEW: Stage clear fanfare — triumphant ascending arpeggio with reverb tail
            stageFanfare(panX) {
                const _v = vv();
                [523, 659, 784, 1047, 1319, 1568, 2093, 2637].forEach((f, i) => {
                    setTimeout(() => { beep('triangle', f, f, 0.12, 0.25 * _v, panX); if (i >= 4) beep('sine', f * 0.5, f * 0.5, 0.1, 0.1 * _v, panX); }, i * 60);
                });
                setTimeout(() => { beep('sine', 1047, 1047, 0.4, 0.2 * _v, panX); beep('triangle', 1568, 1568, 0.4, 0.15 * _v, panX); }, 560);
            },
            stageClearFanfare(panX) { if (ctx.G.muted) return;
                if (ctx.duckMusic) ctx.duckMusic(0.4, 1600);
                const melody = [523, 659, 784, 1047, 784, 1047, 1319, 1568];
                const harmony = [392, 523, 659, 784, 659, 784, 1047, 1175];
                melody.forEach((f, i) => setTimeout(() => {
                    if (ctx.G.muted) return;
                    beep('square', f, f, 0.14, 0.22, panX);
                    beep('triangle', harmony[i], harmony[i], 0.14, 0.12, panX);
                }, i * 90));
                setTimeout(() => { if (!ctx.G.muted && this.warpWhoosh) this.warpWhoosh(); }, 200);
            },
            // NEW: Magnet pull — subtle electronic hum that rises in pitch
            magnetPull(panX) { const _p = pv(); beep('sine', 400 * _p, 700 * _p, 0.15, 0.08, panX); beep('triangle', 600 * _p, 900 * _p, 0.12, 0.05, panX); },
            // NEW: Player death whoosh — dramatic downward air-rush before explosion
            playerDeathWhoosh(panX) {
                const _v = vv(), _p = pv(); const a = audio(); if (!a || ctx.G.muted) return;
                const dur = 0.4; const buf = a.createBuffer(1, Math.floor(a.sampleRate * dur), a.sampleRate), d = buf.getChannelData(0);
                for (let i = 0; i < d.length; i++) d[i] = (Math.random() * 2 - 1) * (1 - i / d.length);
                const s = a.createBufferSource(), f = a.createBiquadFilter(), g = a.createGain();
                s.buffer = buf; f.type = 'bandpass'; f.frequency.setValueAtTime(3000 * _p, a.currentTime); f.frequency.exponentialRampToValueAtTime(200 * _p, a.currentTime + dur);
                g.gain.setValueAtTime(ctx.G.vol * 0.35 * _v, a.currentTime); g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + dur);
                if (panX !== undefined && a.createStereoPanner) { const p = a.createStereoPanner(); p.pan.value = Math.max(-1, Math.min(1, (panX / (ctx.W / 2)) - 1)); s.connect(f).connect(g).connect(p).connect(a.destination); } else { s.connect(f).connect(g).connect(a.destination); }
                s.start(); s.stop(a.currentTime + dur + 0.01);
                beep('sawtooth', 200 * _p, 50 * _p, 0.3, 0.3 * _v, panX);
            },
            // NEW: Combo fire trail — subtle crackle at high combo (throttled by caller)
            fireTrailCrackle(panX) { const _v = vv(); noise(0.04, 0.04 * _v, 6000, panX); if (Math.random() < 0.3) beep('sawtooth', 150 + Math.random() * 100, 80, 0.03, 0.03 * _v, panX); },
            // NEW: Boss death rumble — extended deep rumble + debris fade after boss kill
            bossDeathRumble(panX) {
                const _p = pv(), _v = vv();
                beep('sine', 36 * _p, 18 * _p, 1.2, 0.5 * _v, panX);
                beep('triangle', 24 * _p, 12 * _p, 1.0, 0.3 * _v, panX);
                noise(0.8, 0.25 * _v, 100, panX);
                setTimeout(() => { noise(0.4, 0.15 * _v, 200, panX); beep('sawtooth', 50 * _p, 25 * _p, 0.4, 0.2 * _v, panX); }, 400);
                setTimeout(() => { noise(0.2, 0.08 * _v, 300, panX); }, 800);
            },
            screenShatter() { if (ctx.G.muted) return; const _v = vv(); noise(0.25, 0.35 * _v, 5000); [880, 660, 440].forEach((f, i) => setTimeout(() => beep('sawtooth', f, f * 0.5, 0.08, 0.22 * _v), i * 40)); },
            bulletTimeEnter() { if (ctx.G.muted) return; beep('sine', 1200, 2400, 0.12, 0.25 * vv()); noise(0.08, 0.12 * vv(), 8000); },
            bulletTimeExit() { if (ctx.G.muted) return; beep('triangle', 800, 400, 0.1, 0.18 * vv()); },
            rankSlam() { if (ctx.G.muted) return; const _v = vv(); [440, 554, 659, 880].forEach((f, i) => setTimeout(() => beep('square', f, f, 0.07, 0.28 * _v), i * 45)); },
            hyperShift() { if (ctx.G.muted) return; beep('sawtooth', 180, 720, 0.2, 0.3 * vv()); setTimeout(() => beep('sine', 360, 1080, 0.15, 0.22 * vv()), 80); },
            mirrorPing(panX) { if (ctx.G.muted) return; beep('sine', 1600, 2200, 0.05, 0.16 * vv(), panX); beep('triangle', 1200, 800, 0.06, 0.12 * vv(), panX); },
            modeSelect() { if (ctx.G.muted) return; beep('square', 660, 990, 0.05, 0.2 * vv()); setTimeout(() => beep('triangle', 990, 1320, 0.05, 0.18 * vv()), 60); },
            gauntletWave() { if (ctx.G.muted) return; [523, 659, 784].forEach((f, i) => setTimeout(() => beep('sawtooth', f, f, 0.08, 0.24 * vv()), i * 70)); },
            rocketLaunch(panX) { if (ctx.G.muted) return; beep('sawtooth', 180, 520, 0.1, 0.24 * vv(), panX); noise(0.07, 0.12 * vv(), 380, panX); },
            rocketHit(panX) { if (ctx.G.muted) return; beep('square', 120, 60, 0.12, 0.28 * vv(), panX); noise(0.15, 0.2 * vv(), 500, panX); },
            mineDrop(panX) { if (ctx.G.muted) return; beep('square', 240, 110, 0.06, 0.16 * vv(), panX); },
            mineExplode(panX) { if (ctx.G.muted) return; beep('sawtooth', 90, 45, 0.14, 0.26 * vv(), panX); noise(0.2, 0.22 * vv(), 600, panX); },
            megabomb(panX) { if (ctx.G.muted) return; noise(0.75, 0.75 * vv(), 1400, panX); beep('sawtooth', 80, 25, 0.55, 0.55 * vv(), panX); setTimeout(() => beep('sine', 40, 20, 0.4, 0.35 * vv(), panX), 120); },
            weatherCrack() { if (ctx.G.muted) return; noise(0.06, 0.14 * vv(), 7000); beep('sawtooth', 1200, 300, 0.04, 0.12 * vv()); }
        };
        ctx.SFX = SFX;
    };
})();
