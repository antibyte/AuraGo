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
                    const big = kind === 'mega_laser';
                    ctx.fm(1450 * _p, 360 * _p, big ? 0.14 : 0.11, (big ? 0.25 : 0.21) * _v, panX, 1.5, { q: 3.2, reverb: 0.14 });
                    beep('sawtooth', 920 * _p, 190 * _p, 0.085, 0.13 * _v, panX, { q: 2.4, cut0: 6800, cut1: 900 });
                    noise(0.055, 0.08 * _v, 3600, panX, { filterType: 'bandpass', q: 2.2 });
                    return;
                }
                if (kind === 'spread' || kind === 'mega_spread') {
                    beep('square', 740 * _p, 1180 * _p, 0.06, 0.19 * _v, panX, { detune: -12, q: 1.8 });
                    beep('triangle', 520 * _p, 960 * _p, 0.055, 0.12 * _v, panX, { detune: 12 });
                    noise(0.03, 0.05 * _v, 4800, panX);
                    return;
                }
                if (kind === 'rapid' || kind === 'ultra_rapid') {
                    const hi = kind === 'ultra_rapid' ? 1.12 : 1;
                    beep('square', 960 * _p * hi, 1520 * _p * hi, 0.038, 0.17 * _v, panX, { q: 2.0 });
                    beep('sine', 1900 * _p, 620 * _p, 0.025, 0.08 * _v, panX);
                    return;
                }
                if (kind === 'vulcan') {
                    beep('square', 1500 * _p, 1100 * _p, 0.03, 0.14 * _v, panX, { q: 2.2 });
                    beep('sine', 2400 * _p, 1600 * _p, 0.02, 0.06 * _v, panX);
                    return;
                }
                if (kind === 'cannon') {
                    ctx.fm(260 * _p, 48 * _p, 0.14, 0.3 * _v, panX, 2.2, { reverb: 0.16 });
                    noise(0.07, 0.16 * _v, 1400, panX, { q: 1.6, cut0: 2800, cut1: 300 });
                    beep('square', 140 * _p, 60 * _p, 0.1, 0.12 * _v, panX);
                    return;
                }
                if (kind === 'wave') {
                    beep('sine', 620 * _p, 980 * _p, 0.09, 0.2 * _v, panX, { detune: 9, reverb: 0.12 });
                    beep('sine', 640 * _p, 940 * _p, 0.09, 0.14 * _v, panX, { detune: -9 });
                    ctx.fm(880 * _p, 1240 * _p, 0.07, 0.1 * _v, panX, 0.8);
                    return;
                }
                if (kind === 'flak') {
                    beep('square', 330 * _p, 120 * _p, 0.07, 0.2 * _v, panX, { q: 1.8, cut0: 2400, cut1: 500 });
                    noise(0.05, 0.14 * _v, 1800, panX, { filterType: 'bandpass', q: 1.4 });
                    ctx.fm(180 * _p, 70 * _p, 0.09, 0.14 * _v, panX, 1.8);
                    return;
                }
                if (kind === 'rail') {
                    ctx.fm(420 * _p, 2600 * _p, 0.09, 0.22 * _v, panX, 3.0, { q: 2.5 });
                    beep('sawtooth', 1800 * _p, 3400 * _p, 0.07, 0.12 * _v, panX, { q: 2.2 });
                    noise(0.06, 0.18 * _v, 7200, panX, { filterType: 'highpass' });
                    ctx.scheduleSfx(() => beep('sine', 2900 * _p, 900 * _p, 0.11, 0.1 * _v, panX, { reverb: 0.22 }), 40);
                    return;
                }
                if (kind === 'pierce' || kind === 'mega_pierce') {
                    beep('sawtooth', 1700 * _p, 2300 * _p, 0.05, 0.14 * _v, panX, { q: 2.4, cut0: 7000, cut1: 2500 });
                    beep('sine', 980 * _p, 1300 * _p, 0.05, 0.1 * _v, panX);
                    return;
                }
                if (kind === 'ricochet' || kind === 'mega_ricochet') {
                    beep('triangle', 1480 * _p, 1720 * _p, 0.06, 0.16 * _v, panX, { q: 2.0, reverb: 0.14 });
                    beep('sine', 2960 * _p, 2400 * _p, 0.04, 0.07 * _v, panX);
                    return;
                }
                beep('sine', 920 * _p, 1340 * _p, 0.065, 0.25 * _v, panX, { q: 1.8 });
                beep('square', 480 * _p, 180 * _p, 0.045, 0.09 * _v, panX, { cut0: 3200, cut1: 600 });
                noise(0.022, 0.04 * _v, 5200, panX);
            },
            shoot(panX) { this.shootTyped('normal', panX); },
            laserShoot(panX) { this.shootTyped('laser', panX); },
            playerHurt(panX) {
                if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                beep('sawtooth', 340 * _p, 95 * _p, 0.11, 0.3 * _v, panX, { q: 2.5, reverb: 0.18 });
                ctx.fm(180 * _p, 58 * _p, 0.12, 0.22 * _v, panX, 2.5);
                noise(0.075, 0.16 * _v, 1900, panX, { q: 1.8 });
            },
            dive(panX) { const _p = pv(), _v = vv(); beep('sawtooth', 660 * _p, 185 * _p, 0.32, 0.16 * _v, panX, { q: 2.2, reverb: 0.15 }); beep('sine', 330 * _p, 92 * _p, 0.28, 0.1 * _v, panX); },
            eExplode(panX) {
                const _p = pv(), _v = vv();
                noise(0.025, 0.26 * _v, 6200, panX);
                noise(0.16, 0.38 * _v, 2100, panX, { q: 1.6, cut0: 3800, cut1: 420 });
                ctx.fm(220 * _p, 52 * _p, 0.13, 0.24 * _v, panX, 1.8);
                beep('triangle', 78 * _p, 28 * _p, 0.17, 0.2 * _v, panX);
            },
            bigExplode(panX) {
                const _p = pv(), _v = vv();
                noise(0.035, 0.34 * _v, 5400, panX);
                noise(0.34, 0.48 * _v, 1600, panX, { q: 1.8, cut0: 3400, cut1: 260, reverb: 0.22 });
                ctx.fm(110 * _p, 32 * _p, 0.3, 0.4 * _v, panX, 2.2, { reverb: 0.18 });
                beep('sine', 64 * _p, 24 * _p, 0.35, 0.36 * _v, panX);
            },
            pExplode(panX) {
                const _p = pv(), _v = vv();
                noise(0.04, 0.38 * _v, 5800, panX);
                noise(0.42, 0.56 * _v, 1300, panX, { q: 2.0, cut0: 3200, cut1: 200, reverb: 0.28 });
                ctx.fm(130 * _p, 28 * _p, 0.36, 0.44 * _v, panX, 2.4, { reverb: 0.22 });
                beep('sine', 58 * _p, 22 * _p, 0.42, 0.46 * _v, panX);
                ctx.scheduleSfx(() => noise(0.26, 0.24 * _v, 750, panX, { reverb: 0.25 }), 110);
            },
            stage() { [523, 659, 784, 1047].forEach((f, i) => { ctx.scheduleSfx(() => { beep('triangle', f, f, 0.22, 0.24, undefined, { reverb: 0.24 }); beep('sine', f * 0.5, f * 0.5, 0.2, 0.12); }, i * 115); }); },
            challenge() { [440, 554, 659, 880, 1109, 1319].forEach((f, i) => { ctx.scheduleSfx(() => beep('square', f, f, 0.15, 0.2, undefined, { reverb: 0.2 }), i * 78); }); },
            extra() { [880, 1175, 1760].forEach((f, i) => ctx.scheduleSfx(() => ctx.fm(f, f, 0.18, 0.26, undefined, 2, { reverb: 0.28 }), i * 70)); },
            rescue() { beep('triangle', 880, 880, 0.2, 0.25, undefined, { reverb: 0.22 }); ctx.scheduleSfx(() => ctx.fm(1320, 1320, 0.26, 0.26, undefined, 2, { reverb: 0.28 }), 100); },
            beam() { ctx.fm(220, 195, 0.5, 0.16, undefined, 1.5, { reverb: 0.2 }); beep('sawtooth', 110, 105, 0.48, 0.1); },
            perfect() { [523, 659, 784, 1047, 1319, 1568].forEach((f, i) => { ctx.scheduleSfx(() => ctx.fm(f, f, 0.17, 0.28, undefined, 2, { reverb: 0.26 }), i * 58); }); },
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
                const rev = rarity === 'legendary' ? 0.32 : rarity === 'rare' ? 0.24 : 0.14;
                notes.forEach((f, i) => ctx.scheduleSfx(() => beep('sine', f, f * 1.01, 0.075, 0.18 + i * 0.02, panX, { reverb: rev }), i * gap));
                if (rarity === 'rare' || rarity === 'legendary') this.powerupChime(panX);
            },
            weaponArm(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                beep('triangle', 520 * _p, 1480 * _p, 0.075, 0.22 * _v, panX, { q: 2.0, reverb: 0.16 });
                beep('square', 310 * _p, 940 * _p, 0.055, 0.12 * _v, panX);
                noise(0.03, 0.06 * _v, 6500, panX);
            },
            puCollect(panX) { this.puCollectRarity('common', panX); },
            bomb(panX) {
                const _v = vv();
                noise(0.5, 0.65 * _v, 900, panX, { q: 2.0, cut0: 2600, cut1: 140, reverb: 0.28 });
                ctx.fm(115, 32, 0.45, 0.48 * _v, panX, 2.2);
                beep('sine', 56, 22, 0.48, 0.42 * _v, panX);
            },
            combo(n) { const _p = pv(); const f = (440 + n * 110) * _p; beep('sine', f, f * 1.02, 0.12, (0.24 + n * 0.04) * vv(), undefined, { reverb: 0.18 }); },
            bossWarning() {
                const _v = vv();
                ctx.fm(440, 210, 0.48, 0.3 * _v, undefined, 1.5, { reverb: 0.28 });
                beep('sawtooth', 220, 105, 0.48, 0.22 * _v);
                ctx.scheduleSfx(() => {
                    ctx.fm(440, 210, 0.48, 0.3 * _v, undefined, 1.5, { reverb: 0.28 });
                    beep('sawtooth', 220, 105, 0.48, 0.22 * _v);
                }, 480);
            },
            shieldHit() { const _p = pv(), _v = vv(); ctx.fm(2100 * _p, 3800 * _p, 0.065, 0.28 * _v, undefined, 2.5, { reverb: 0.22 }); beep('sine', 3000 * _p, 1400 * _p, 0.085, 0.18 * _v); },
            respawn() { if (ctx.G.muted) return; beep('sine', 220, 880, 0.3, 0.24, undefined, { reverb: 0.22 }); ctx.scheduleSfx(() => { if (ctx.G.muted) return; beep('triangle', 660, 1320, 0.22, 0.2, undefined, { reverb: 0.25 }); }, 80); ctx.scheduleSfx(() => { if (ctx.G.muted) return; ctx.fm(1760, 3520, 0.28, 0.12, undefined, 2, { reverb: 0.3 }); noise(0.18, 0.05, 8500); }, 35); },
            shieldBreak() { noise(0.22, 0.46 * vv(), 3200, undefined, { reverb: 0.22 }); ctx.fm(260 * pv(), 75, 0.18, 0.36 * vv(), undefined, 2.4); },
            bossJingle() { [220, 262, 330, 220, 165, 220].forEach((f, i) => { ctx.scheduleSfx(() => beep('sawtooth', f, f, 0.16, 0.2 + i * 0.02, undefined, { reverb: 0.2 }), i * 100); }); },
            stageClear() { this.stageClearFanfare(); },
            puUpgrade(panX) { [800, 1000, 1200, 1400, 1600].forEach((f, i) => { ctx.scheduleSfx(() => ctx.fm(f, f, 0.065, 0.24, panX, 2, { reverb: 0.22 }), i * 32); }); },
            weaponUp() { [600, 800, 1000, 1200].forEach((f, i) => { ctx.scheduleSfx(() => beep('triangle', f, f, 0.085, 0.22, undefined, { reverb: 0.2 }), i * 58); }); },
            homingLock(panX) { const _p = pv(); beep('sine', 1320 * _p, 1320 * _p, 0.04, 0.15, panX); ctx.scheduleSfx(() => beep('sine', 1980 * _p, 1980 * _p, 0.035, 0.12, panX, { reverb: 0.15 }), 35); },
            supernova(panX) { const _v = vv(); noise(0.75, 0.78 * _v, 700, panX, { q: 2.0, cut0: 3200, cut1: 110, reverb: 0.32 }); ctx.fm(95, 26, 0.6, 0.58 * _v, panX, 2.5, { reverb: 0.25 }); beep('sine', 180, 36, 0.52, 0.45 * _v, panX); },
            miniBossWarning() { ctx.fm(330, 165, 0.38, 0.28, undefined, 1.5, { reverb: 0.22 }); ctx.scheduleSfx(() => ctx.fm(330, 165, 0.38, 0.28, undefined, 1.5, { reverb: 0.22 }), 380); },
            bossHitSFX(panX) { const _p = pv(), _v = vv(); ctx.fm(280 * _p, 58 * _p, 0.12, 0.42 * _v, panX, 2.0); noise(0.09, 0.32 * _v, 1100, panX, { q: 1.8 }); },
            warpJump() { beep('sawtooth', 180, 3600, 0.35, 0.38, undefined, { q: 2.2, reverb: 0.28 }); beep('sine', 90, 2800, 0.3, 0.3); ctx.scheduleSfx(() => noise(0.18, 0.26, 4200, undefined, { reverb: 0.24 }), 220); },
            coinInsert() { ctx.fm(988, 988, 0.07, 0.38, undefined, 2, { reverb: 0.25 }); ctx.scheduleSfx(() => ctx.fm(1319, 1319, 0.22, 0.4, undefined, 2, { reverb: 0.3 }), 65); },
            comboBreak() { beep('sawtooth', 420 * pv(), 160, 0.18, 0.2 * vv(), undefined, { q: 2.0 }); },
            killStreak() { [880, 1109, 1319, 1760].forEach((f, i) => { ctx.scheduleSfx(() => beep('sine', f, f, 0.09, 0.26, undefined, { reverb: 0.22 }), i * 52); }); },
            freeze(panX) { const _p = pv(), _v = vv(); ctx.fm(1400 * _p, 3800 * _p, 0.08, 0.32 * _v, panX, 2.8, { reverb: 0.3 }); beep('triangle', 900 * _p, 2800 * _p, 0.06, 0.24 * _v, panX); ctx.scheduleSfx(() => { beep('triangle', 420, 180, 0.1, 0.15, panX); noise(0.14, 0.18, 7500, panX, { reverb: 0.25 }); }, 90); },
            powerupExpire() { beep('sawtooth', 880, 440, 0.08, 0.34); },
            enemyHitSfx(type, panX) {
                if (ctx.G.muted) return;
                let hitType = type, px = panX;
                if (px === undefined) { hitType = 'bee'; px = type; }
                const _p = pv(), _v = vv();
                const base = hitType === 'shield_bee' ? 560 : hitType === 'boss' || hitType === 'miniboss' ? 210 : 400;
                beep('sine', base * _p, (base * 0.42) * _p, 0.032, 0.22 * _v, px, { q: 1.8 });
                beep('square', (base * 1.35) * _p, (base * 0.65) * _p, 0.022, 0.1 * _v, px);
            },
            stalkerDive(panX) { const _p = pv(), _v = vv(); beep('sawtooth', 920 * _p, 280 * _p, 0.22, 0.2 * _v, panX, { q: 2.2, reverb: 0.15 }); noise(0.1, 0.14 * _v, 4800, panX); },
            hunterDive(panX) { const _p = pv(), _v = vv(); beep('sawtooth', 1240 * _p, 175 * _p, 0.34, 0.26 * _v, panX, { q: 2.5, reverb: 0.18 }); noise(0.14, 0.2 * _v, 3600, panX); beep('square', 420 * _p, 115 * _p, 0.12, 0.16 * _v, panX); },
            hunterShot(panX) { const _p = pv(), _v = vv(); beep('sawtooth', 740 * _p, 320 * _p, 0.095, 0.21 * _v, panX, { q: 2.0 }); beep('square', 520 * _p, 190 * _p, 0.06, 0.13 * _v, panX); },
            spinnerShot(panX) { const _p = pv(), _v = vv(); ctx.fm(1400 * _p, 2250 * _p, 0.07, 0.19 * _v, panX, 1.5); beep('triangle', 920 * _p, 1520 * _p, 0.05, 0.11 * _v, panX); },
            bomberDrop(panX) { const _p = pv(), _v = vv(); beep('sawtooth', 310 * _p, 78 * _p, 0.12, 0.2 * _v, panX, { q: 1.8 }); noise(0.08, 0.12 * _v, 850, panX); },
            lobShot(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                beep('triangle', 420 * _p, 160 * _p, 0.1, 0.18 * _v, panX, { q: 1.6 });
                noise(0.05, 0.08 * _v, 600, panX);
            },
            pincerShot(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                beep('square', 980 * _p, 640 * _p, 0.06, 0.16 * _v, panX, { q: 2.0 });
                ctx.scheduleSfx(() => beep('square', 1040 * _p, 700 * _p, 0.06, 0.14 * _v, panX, { q: 2.0 }), 30);
            },
            lasherShot(panX) { const _p = pv(), _v = vv(); beep('sine', 620 * _p, 1850 * _p, 0.13, 0.21 * _v, panX, { q: 2.2 }); beep('triangle', 420 * _p, 1240 * _p, 0.08, 0.14 * _v, panX); },
            sniperShot(panX) { const _p = pv(), _v = vv(); ctx.fm(1900 * _p, 540 * _p, 0.085, 0.24 * _v, panX, 2.0, { reverb: 0.14 }); beep('square', 1250 * _p, 380 * _p, 0.05, 0.12 * _v, panX); },
            comboMilestone(n, panX) { [880, 1109, 1319, 1760, 2217].forEach((f, i) => { ctx.scheduleSfx(() => beep('sine', f, f, 0.085, 0.2 + n * 0.025, panX, { reverb: 0.22 }), i * 38); }); },
            shieldReflect(panX) { const _p = pv(); ctx.fm(3000 * _p, 1500 * _p, 0.065, 0.28, panX, 2.2, { reverb: 0.2 }); beep('sine', 2000 * _p, 4000 * _p, 0.045, 0.18, panX); },
            closeCall(panX) { const _p = pv(); noise(0.06, 0.13 * vv(), 6200, panX); beep('sine', 1600 * _p, 780 * _p, 0.045, 0.11, panX, { reverb: 0.15 }); },
            envAmbience(theme) { if (theme === 'storm') noise(2, 0.03, 400); else if (theme === 'blackhole') { beep('sine', 40, 40, 2, 0.04); beep('sine', 55, 55, 2, 0.03); } else if (theme === 'crystal') ctx.fm(2400, 2000, 0.3, 0.025, undefined, 2, { reverb: 0.3 }); },
            rageMode(panX) { beep('sawtooth', 300, 620, 0.24, 0.32, panX, { q: 2.4 }); beep('square', 200, 410, 0.2, 0.18, panX); noise(0.14, 0.18, 4200, panX); },
            chainLightning(hop, panX) { const _r = 1 + hop * 0.15; ctx.fm(1850 * _r, 2500 * _r, 0.065, 0.28, panX, 2.5, { reverb: 0.18 }); noise(0.04, 0.18, 6500, panX); beep('triangle', 1200 * _r, 780 * _r, 0.045, 0.14, panX); },
            orbitalShieldHit(panX) { ctx.fm(3000, 1450, 0.08, 0.28, panX, 2.2, { reverb: 0.18 }); beep('triangle', 2000, 800, 0.11, 0.22, panX); noise(0.055, 0.14, 5200, panX); },
            relicActivate() { [440, 659, 880, 1109].forEach((f, i) => ctx.scheduleSfx(() => ctx.fm(f, f, 0.24 - i * 0.03, 0.22 - i * 0.03, undefined, 2, { reverb: 0.3 }), i * 45)); },
            nearMiss(panX) { noise(0.05, 0.14, 7000, panX); beep('sine', 85, 58, 0.11, 0.18, panX); },
            bossPhaseTrans() { ctx.fm(110, 48, 0.58, 0.38, undefined, 2.2, { reverb: 0.28 }); noise(0.38, 0.28, 850); ctx.scheduleSfx(() => beep('sawtooth', 220, 95, 0.3, 0.24, undefined, { reverb: 0.2 }), 190); },
            mutationStart() { ctx.fm(185, 88, 0.4, 0.28, undefined, 1.8, { reverb: 0.22 }); noise(0.28, 0.14, 3200); beep('sine', 62, 38, 0.48, 0.2); },
            comboExtend() { [600, 800, 1000, 1200].forEach((f, i) => { ctx.scheduleSfx(() => beep('triangle', f, f, 0.06, 0.2, undefined, { reverb: 0.16 }), i * 28); }); },
            scoreRoll() { beep('triangle', 1240, 1440, 0.02, 0.08); },
            deathBloom() { noise(0.28, 0.36, 7800, undefined, { reverb: 0.25 }); ctx.fm(2000, 480, 0.25, 0.28, undefined, 2, { reverb: 0.24 }); },
            envWind() { noise(1.5, 0.04, 600); beep('sine', 30, 30, 1.5, 0.02); },
            // NEW: Parry success — crisp upward "shwing" with reverb tail
            parrySuccess(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                ctx.fm(2100 * _p, 4900 * _p, 0.06, 0.36 * _v, panX, 2.4, { reverb: 0.32 });
                beep('square', 1650 * _p, 2900 * _p, 0.045, 0.2 * _v, panX, { q: 2.2 });
                ctx.scheduleSfx(() => beep('sine', 2637 * _p, 1760 * _p, 0.11, 0.18 * _v, panX, { reverb: 0.35 }), 45);
            },
            parryStart(panX) { const _p = pv(); beep('sine', 2200 * _p, 2700 * _p, 0.04, 0.18, panX, { reverb: 0.15 }); beep('triangle', 1400 * _p, 2050 * _p, 0.032, 0.12, panX); },
            parryMiss(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                beep('sawtooth', 280 * _p, 90 * _p, 0.09, 0.16 * _v, panX, { q: 1.8 });
                noise(0.05, 0.1 * _v, 700, panX);
            },
            // NEW: Super activation — build sweep + impact boom, unique per ship type
            superActivate(shipType, panX) {
                const _v = vv();
                if (shipType === 'classic') { beep('sawtooth', 200, 1480, 0.3, 0.38 * _v, panX, { q: 2.4, reverb: 0.28 }); beep('square', 100, 320, 0.25, 0.2 * _v, panX); ctx.scheduleSfx(() => noise(0.38, 0.46 * _v, 850, panX, { reverb: 0.25 }), 240); }
                else if (shipType === 'interceptor') { ctx.fm(320, 3100, 0.3, 0.38 * _v, panX, 2, { reverb: 0.3 }); beep('sawtooth', 160, 2100, 0.25, 0.24 * _v, panX); ctx.scheduleSfx(() => noise(0.15, 0.28 * _v, 5200, panX, { reverb: 0.22 }), 190); }
                else if (shipType === 'heavy') { ctx.fm(88, 42, 0.5, 0.54 * _v, panX, 2.2, { reverb: 0.28 }); noise(0.48, 0.46 * _v, 420, panX); beep('square', 130, 68, 0.32, 0.28 * _v, panX); }
                else if (shipType === 'stealth') { beep('sine', 620, 1860, 0.34, 0.34 * _v, panX, { reverb: 0.3 }); beep('triangle', 930, 2480, 0.3, 0.24 * _v, panX); ctx.scheduleSfx(() => { [1240, 1560, 1860].forEach((f, i) => beep('sine', f, f, 0.2, (0.2 - i * 0.04) * _v, panX, { reverb: 0.32 })); }, 140); }
                else { beep('sawtooth', 150, 840, 0.38, 0.42 * _v, panX, { reverb: 0.25 }); noise(0.3, 0.36 * _v, 650, panX); }
            },
            // NEW: Biome reveal stinger
            biomeReveal() { [262, 330, 392, 523, 659].forEach((f, i) => { ctx.scheduleSfx(() => beep('sine', f, f, 0.18, 0.22, undefined, { reverb: 0.28 }), i * 88); }); ctx.scheduleSfx(() => { beep('triangle', 523, 523, 0.42, 0.2, undefined, { reverb: 0.32 }); beep('sine', 784, 784, 0.42, 0.14, undefined, { reverb: 0.32 }); }, 480); },
            // NEW: Bonus sub-stage jingle
            bonusStart() { [523, 659, 784, 659, 784, 988].forEach((f, i) => { ctx.scheduleSfx(() => beep('square', f, f, 0.1, 0.2, undefined, { reverb: 0.18 }), i * 68); }); },
            bonusEnd(rating) { const base = rating === 'S' ? [784, 988, 1175, 1568] : rating === 'A' ? [659, 784, 988, 1319] : [523, 659, 784, 1047]; base.forEach((f, i) => { ctx.scheduleSfx(() => ctx.fm(f, f, 0.13, 0.25, undefined, 2, { reverb: 0.26 }), i * 78); }); },
            // NEW: UI sounds
            uiHover() { beep('sine', 1050, 1120, 0.028, 0.09); },
            uiClick() { beep('triangle', 1240, 1480, 0.038, 0.15, undefined, { reverb: 0.12 }); },
            uiBack() { beep('triangle', 640, 420, 0.05, 0.12); },
            uiToggle() { beep('square', 820, 1080, 0.04, 0.11); },
            shopBuy() { ctx.fm(880, 1175, 0.08, 0.22, undefined, 2, { reverb: 0.22 }); ctx.scheduleSfx(() => ctx.fm(1319, 1760, 0.12, 0.24, undefined, 2, { reverb: 0.28 }), 75); },
            shopError() { beep('sawtooth', 210, 115, 0.13, 0.2, undefined, { q: 2.0 }); },
            achievementUnlock() { [523, 659, 784, 1047].forEach((f, i) => { ctx.scheduleSfx(() => ctx.fm(f, f, 0.16, 0.27, undefined, 2, { reverb: 0.28 }), i * 68); }); },
            // NEW: Whoosh for dives/parries/supers — noise + bandpass sweep
            whoosh(speed, panX) { noise(Math.max(0.08, Math.min(0.3, 60 / Math.max(50, speed))), 0.18, 2200, panX, { filterType: 'bandpass', q: 2.2 }); },
            eExplodeTyped(type, size, panX) {
                const _p = pv(), _v = vv();
                const big = size === 'big' || type === 'boss' || type === 'miniboss';
                noise(0.025, (big ? 0.34 : 0.24) * _v, 5800, panX);
                noise(big ? 0.32 : 0.16, (big ? 0.48 : 0.36) * _v, big ? 1500 : 2100, panX, { q: 1.7, reverb: big ? 0.24 : 0.08 });
                const baseF = type === 'boss' ? 62 : type === 'miniboss' ? 72 : type === 'kamikaze' ? 105 : 195;
                ctx.fm(baseF * _p, baseF * 0.35 * _p, big ? 0.26 : 0.12, (big ? 0.38 : 0.22) * _v, panX, 1.9);
                beep('triangle', baseF * 0.45 * _p, baseF * 0.18 * _p, big ? 0.3 : 0.15, (big ? 0.26 : 0.15) * _v, panX);
                if (big) {
                    noise(0.22, 0.22 * _v, 580, panX);
                    beep('sawtooth', 82 * _p, 34 * _p, 0.38, 0.28 * _v, panX, { reverb: 0.2 });
                }
            },
            // NEW: Super stinger audio
            superChargeStart(panX) { const _v = vv(); beep('sawtooth', 90, 880, 0.38, 0.34 * _v, panX, { q: 2.4, reverb: 0.24 }); beep('triangle', 65, 660, 0.34, 0.24 * _v, panX); noise(0.28, 0.18 * _v, 1600, panX); },
            superNovaBarrage(panX) { const _v = vv(); beep('sawtooth', 220 * pv(), 1480, 0.3, 0.42 * _v, panX, { q: 2.4, reverb: 0.28 }); beep('square', 110, 330, 0.25, 0.24 * _v, panX); ctx.scheduleSfx(() => noise(0.38, 0.46 * _v, 850, panX, { reverb: 0.25 }), 240); },
            superPhaseDash(panX) { const _v = vv(); ctx.fm(320, 3100, 0.3, 0.38 * _v, panX, 2, { reverb: 0.3 }); beep('sawtooth', 160, 2100, 0.25, 0.24 * _v, panX); ctx.scheduleSfx(() => noise(0.15, 0.28 * _v, 5200, panX, { reverb: 0.22 }), 190); },
            superAegisCannon(panX) { const _v = vv(); ctx.fm(88, 44, 0.5, 0.54 * _v, panX, 2.2, { reverb: 0.28 }); noise(0.48, 0.46 * _v, 420, panX); beep('square', 130, 68, 0.32, 0.28 * _v, panX); },
            superShadowClone(panX) { const _v = vv(); beep('sine', 620, 1860, 0.34, 0.34 * _v, panX, { reverb: 0.28 }); beep('triangle', 930, 2480, 0.3, 0.24 * _v, panX); ctx.scheduleSfx(() => { [1240, 1560, 1860].forEach((f, i) => beep('sine', f, f, 0.2, (0.2 - i * 0.04) * _v, panX, { reverb: 0.3 })); }, 140); },
            // NEW: Stage archetype cues
            archetypeSwarmLoop() { const _v = vv(); beep('square', 100, 100, 0.1, 0.18 * _v); ctx.scheduleSfx(() => beep('square', 100, 100, 0.1, 0.18 * _v), 500); },
            archetypeEscortPad() { const _v = vv(); beep('sawtooth', 110, 220, 1.0, 0.15 * _v, undefined, { reverb: 0.25 }); beep('triangle', 165, 330, 1.0, 0.1 * _v); },
            archetypeAsteroidWarning(panX) { const _p = pv(); beep('sine', 1800 * _p, 600 * _p, 0.15, 0.25, panX, { reverb: 0.18 }); beep('triangle', 1200 * _p, 400 * _p, 0.1, 0.15, panX); },
            // NEW: Risk-It mode toggle UI sound
            riskItToggle() { beep('square', 600, 900, 0.05, 0.18); ctx.scheduleSfx(() => beep('triangle', 900, 1200, 0.05, 0.18, undefined, { reverb: 0.16 }), 50); },
            // NEW: Rank jingles
            rankJingleSplus() { const _v = vv(); [784, 988, 1175, 1568, 2093, 2637].forEach((f, i) => { ctx.scheduleSfx(() => ctx.fm(f, f, 0.13, 0.28 * _v, undefined, 2, { reverb: 0.3 }), i * 58); }); },
            rankJingleS() { const _v = vv(); [659, 784, 988, 1319, 1568].forEach((f, i) => { ctx.scheduleSfx(() => beep('triangle', f, f, 0.11, 0.25 * _v, undefined, { reverb: 0.25 }), i * 68); }); },
            rankJingleA() { const _v = vv(); [523, 659, 784, 1047].forEach((f, i) => { ctx.scheduleSfx(() => beep('sine', f, f, 0.11, 0.22 * _v, undefined, { reverb: 0.2 }), i * 78); }); },
            rankJingleB() { beep('triangle', 440, 440, 0.15, 0.2); ctx.scheduleSfx(() => beep('triangle', 660, 660, 0.15, 0.2, undefined, { reverb: 0.18 }), 150); },
            rankJingleC() { const _v = vv(); beep('sawtooth', 220, 110, 0.4, 0.25 * _v); beep('triangle', 165, 82, 0.3, 0.2 * _v); },
            // NEW: Boss phase stingers
            bossPhaseTransition() { const _v = vv(); ctx.fm(110, 48, 0.58, 0.38 * _v, undefined, 2.2, { reverb: 0.28 }); noise(0.38, 0.28 * _v, 850); ctx.scheduleSfx(() => beep('sawtooth', 220, 95, 0.3, 0.24 * _v, undefined, { reverb: 0.2 }), 190); },
            bossPhaseCrescendo() { const _v = vv(); beep('sawtooth', 200, 840, 0.48, 0.34 * _v, undefined, { q: 2.2, reverb: 0.25 }); beep('triangle', 400, 1680, 0.38, 0.24 * _v); noise(0.28, 0.18 * _v, 4200); },
            bossDeathStinger() { const _v = vv(); [110, 82, 65, 49, 41].forEach((f, i) => { ctx.scheduleSfx(() => { ctx.fm(f, f * 0.5, 0.56, 0.33 * _v, undefined, 2.0, { reverb: 0.28 }); beep('sine', f * 2, f, 0.48, 0.24 * _v); }, i * 380); }); },
            bossKillFanfare(panX) { if (ctx.G.muted) return;
                const _v = vv();
                if (ctx.duckMusic) ctx.duckMusic(0.8, 1400);
                noise(0.08, 0.05 * _v, 200, panX);
                ctx.scheduleSfx(() => {
                    if (ctx.G.muted) return;
                    if (this.bossDeathStinger) this.bossDeathStinger();
                    else this.bossDeathRumble(panX);
                    if (this.subThump) this.subThump(panX);
                }, 120);
            },
            // NEW: Ambient loops (one-shot, called by biome system)
            ambientStormWind() { noise(1.5, 0.04, 400); beep('sine', 30, 30, 1.5, 0.02); },
            ambientBlackholeDrone() { beep('sine', 40, 40, 2, 0.04); beep('sine', 55, 55, 2, 0.03); },
            ambientCrystalSparkle() { ctx.fm(2400, 2000, 0.3, 0.022, undefined, 2, { reverb: 0.28 }); },
            // NEW: Deep sub-bass thump layered under boss/big explosions (paired with fxBossShockwave)
            subThump(panX) { const _p = pv(), _v = vv(); beep('sine', 52 * _p, 24 * _p, 0.5, 0.52 * _v, panX); beep('triangle', 38 * _p, 20 * _p, 0.4, 0.3 * _v, panX); noise(0.32, 0.28 * _v, 240, panX); },
            // NEW: Bright powerup chime with convolver reverb send (rare/legendary pickups)
            powerupChime(panX) { [660, 990, 1320].forEach((f, i) => ctx.scheduleSfx(() => ctx.fm(f, f, 0.22, 0.22, panX, 3, { reverb: 0.32 }), i * 65)); },
            warpWhoosh() { noise(0.35, 0.24, 1300, undefined, { filterType: 'bandpass', q: 2.0, reverb: 0.22 }); ctx.fm(90, 920, 0.32, 0.2, undefined, 1.8, { reverb: 0.2 }); },
            comboRiser(level, panX) { const base = 440 * Math.pow(1.1892, Math.min(level, 8)); [1, 1.26, 1.5, 2, 2.52].forEach((iv, i) => { ctx.scheduleSfx(() => beep('sine', base * iv, base * iv, 0.09, (0.2 + level * 0.02) * vv(), panX, { reverb: 0.22 }), i * 44); }); },
            megaComboStinger(level, panX) { if (ctx.G.muted) return;
                if (ctx.duckMusic) ctx.duckMusic(0.45, 700);
                this.comboRiser(level, panX);
                const base = 523 * Math.pow(1.122, Math.min(level, 6));
                [0, 4, 7, 12].forEach((semi, i) => {
                    const f = base * Math.pow(2, semi / 12);
                    ctx.scheduleSfx(() => beep('square', f, f, 0.1, 0.22 + i * 0.03, panX, { reverb: 0.26 }), 220 + i * 55);
                });
            },
            // NEW: Short high sparkle twinkle for precision score bonuses (headshots)
            sparkleTwinkle(panX) { const _p = pv(); ctx.fm(2400 * _p, 2400 * _p, 0.055, 0.16, panX, 2, { reverb: 0.24 }); ctx.scheduleSfx(() => ctx.fm(3600 * _p, 3600 * _p, 0.065, 0.14, panX, 2, { reverb: 0.28 }), 52); },
            // NEW: Graze — brief high-pitched "zip" when bullets skim the player
            graze(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                beep('triangle', 3100 * _p, 4400 * _p, 0.035, 0.16 * _v, panX, { reverb: 0.14 });
                beep('sine', 2450 * _p, 3300 * _p, 0.03, 0.1 * _v, panX);
                noise(0.025, 0.07 * _v, 9200, panX);
            },
            // NEW: Boss entrance rumble — deep ominous stinger + sub-bass growl
            bossEntrance(panX) { const _p = pv(), _v = vv(); ctx.fm(64 * _p, 28 * _p, 0.78, 0.44 * _v, panX, 2.0, { reverb: 0.28 }); beep('triangle', 42 * _p, 20 * _p, 0.6, 0.3 * _v, panX); noise(0.58, 0.24 * _v, 140, panX); ctx.scheduleSfx(() => { beep('sawtooth', 124 * _p, 78 * _p, 0.3, 0.28 * _v, panX, { reverb: 0.22 }); noise(0.2, 0.15 * _v, 320, panX); }, 290); },
            // NEW: Stage clear fanfare — triumphant ascending arpeggio with reverb tail
            stageFanfare(panX) {
                const _v = vv();
                [523, 659, 784, 1047, 1319, 1568, 2093, 2637].forEach((f, i) => {
                    ctx.scheduleSfx(() => { beep('triangle', f, f, 0.12, 0.25 * _v, panX, { reverb: 0.28 }); if (i >= 4) beep('sine', f * 0.5, f * 0.5, 0.1, 0.1 * _v, panX); }, i * 60);
                });
                ctx.scheduleSfx(() => { beep('sine', 1047, 1047, 0.4, 0.2 * _v, panX, { reverb: 0.32 }); beep('triangle', 1568, 1568, 0.4, 0.15 * _v, panX, { reverb: 0.32 }); }, 560);
            },
            stageClearFanfare(panX) { if (ctx.G.muted) return;
                if (ctx.duckMusic) ctx.duckMusic(0.4, 1600);
                const melody = [523, 659, 784, 1047, 784, 1047, 1319, 1568];
                const harmony = [392, 523, 659, 784, 659, 784, 1047, 1175];
                melody.forEach((f, i) => ctx.scheduleSfx(() => {
                    if (ctx.G.muted) return;
                    beep('square', f, f, 0.14, 0.22, panX, { reverb: 0.25 });
                    beep('triangle', harmony[i], harmony[i], 0.14, 0.13, panX, { reverb: 0.2 });
                }, i * 90));
            },
            // NEW: Magnet pull — subtle electronic hum that rises in pitch
            magnetPull(panX) { const _p = pv(); beep('sine', 420 * _p, 740 * _p, 0.14, 0.08, panX, { reverb: 0.15 }); beep('triangle', 620 * _p, 940 * _p, 0.11, 0.05, panX); },
            // NEW: Player death whoosh — dramatic downward air-rush before explosion
            playerDeathWhoosh(panX) { noise(0.38, 0.34, 1650, panX, { reverb: 0.25 }); ctx.fm(190, 34, 0.38, 0.3, panX, 2.2, { reverb: 0.22 }); },
            fireTrailCrackle(panX) { const _v = vv(); noise(0.038, 0.04 * _v, 6200, panX); if (Math.random() < 0.3) beep('sawtooth', 160 + Math.random() * 100, 80, 0.03, 0.03 * _v, panX); },
            // NEW: Boss death rumble — extended deep rumble + debris fade after boss kill
            bossDeathRumble(panX) {
                const _p = pv(), _v = vv();
                beep('sine', 38 * _p, 20 * _p, 1.15, 0.48 * _v, panX);
                beep('triangle', 26 * _p, 20 * _p, 0.95, 0.28 * _v, panX);
                noise(0.78, 0.24 * _v, 110, panX, { reverb: 0.26 });
                ctx.scheduleSfx(() => { noise(0.38, 0.15 * _v, 220, panX); beep('sawtooth', 52 * _p, 26 * _p, 0.38, 0.2 * _v, panX); }, 380);
                ctx.scheduleSfx(() => { noise(0.22, 0.08 * _v, 320, panX); }, 780);
            },
            screenShatter() { if (ctx.G.muted) return; const _v = vv(); noise(0.24, 0.34 * _v, 5400, undefined, { reverb: 0.3 }); [1047, 784, 523].forEach((f, i) => ctx.scheduleSfx(() => ctx.fm(f, f * 0.55, 0.085, 0.22 * _v, undefined, 2.4, { reverb: 0.26 }), i * 38)); },
            bulletTimeEnter() { if (ctx.G.muted) return; ctx.fm(1200, 2400, 0.13, 0.24 * vv(), undefined, 2, { reverb: 0.28 }); noise(0.08, 0.12 * vv(), 8200); },
            bulletTimeExit() { if (ctx.G.muted) return; beep('triangle', 840, 380, 0.11, 0.18 * vv(), undefined, { reverb: 0.18 }); },
            rankSlam() { if (ctx.G.muted) return; const _v = vv(); beep('square', 220, 110, 0.05, 0.32 * _v); [523, 659, 784, 1047].forEach((f, i) => ctx.scheduleSfx(() => beep('square', f, f * 0.98, 0.06, (0.34 - i * 0.02) * _v, undefined, { reverb: 0.22 }), i * 32)); },
            hyperShift() { if (ctx.G.muted) return; beep('sawtooth', 180, 760, 0.2, 0.28 * vv(), undefined, { q: 2.2, reverb: 0.24 }); ctx.scheduleSfx(() => ctx.fm(360, 1120, 0.16, 0.22 * vv(), undefined, 2, { reverb: 0.24 }), 75); },
            mirrorPing(panX) { if (ctx.G.muted) return; ctx.fm(1650, 2300, 0.055, 0.16 * vv(), panX, 2.5, { reverb: 0.25 }); beep('triangle', 1200, 800, 0.06, 0.12 * vv(), panX); },
            modeSelect() { if (ctx.G.muted) return; beep('square', 660, 990, 0.05, 0.2 * vv(), undefined, { reverb: 0.15 }); ctx.scheduleSfx(() => beep('triangle', 990, 1320, 0.055, 0.18 * vv(), undefined, { reverb: 0.2 }), 58); },
            gauntletWave() { if (ctx.G.muted) return; [523, 659, 784].forEach((f, i) => ctx.scheduleSfx(() => beep('sawtooth', f, f, 0.085, 0.24 * vv(), undefined, { reverb: 0.2 }), i * 68)); },
            expansionWeapon(type, panX) {
                if (ctx.G.muted) return;
                const volume = vv();
                if (type === 'emp_mine_layer') { beep('sine', 1100, 180, 0.25, 0.17 * volume, panX); noise(0.16, 0.1 * volume, 2400, panX); }
                else if (type === 'cluster_mine_layer') { noise(0.12, 0.2 * volume, 3200, panX); beep('triangle', 240, 70, 0.12, 0.14 * volume, panX); }
                else if (type === 'swarm_launcher') beep('square', 480, 1000, 0.09, 0.12 * volume, panX);
                else if (type === 'seeker_torpedo') beep('sawtooth', 95, 42, 0.25, 0.2 * volume, panX);
            },
            rocketLaunch(panX) { if (ctx.G.muted) return; beep('sawtooth', 190, 560, 0.11, 0.23 * vv(), panX, { q: 2.0 }); noise(0.08, 0.13 * vv(), 460, panX, { filterType: 'bandpass', q: 1.8 }); },
            rocketHit(panX) { if (ctx.G.muted) return; ctx.fm(135, 48, 0.14, 0.28 * vv(), panX, 2.0); noise(0.16, 0.22 * vv(), 560, panX); },
            mineDrop(panX) { if (ctx.G.muted) return; beep('square', 250, 110, 0.06, 0.16 * vv(), panX); },
            flakBurst(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                ctx.fm(150 * _p, 46 * _p, 0.11, 0.24 * _v, panX, 2.0);
                noise(0.09, 0.2 * _v, 1200, panX, { q: 1.5, cut0: 3400, cut1: 400 });
                noise(0.03, 0.1 * _v, 6000, panX);
            },
            mineExplode(panX) { if (ctx.G.muted) return; ctx.fm(105, 38, 0.16, 0.26 * vv(), panX, 2.1); noise(0.2, 0.22 * vv(), 650, panX); },
            megabomb(panX) { if (ctx.G.muted) return; noise(0.72, 0.72 * vv(), 1400, panX, { q: 2.0, cut0: 3600, cut1: 120, reverb: 0.32 }); ctx.fm(90, 24, 0.55, 0.52 * vv(), panX, 2.4, { reverb: 0.26 }); ctx.scheduleSfx(() => beep('sine', 44, 20, 0.42, 0.35 * vv(), panX), 115); },
            weatherCrack() { if (ctx.G.muted) return; noise(0.06, 0.14 * vv(), 7200, undefined, { reverb: 0.2 }); beep('sawtooth', 1280, 280, 0.045, 0.12 * vv()); },
            // NEW: Super-ready cue — bright ascending chime with reverb tail when the meter first fills
            superReady(panX) { if (ctx.G.muted) return;
                const a = audio(); if (!a) return;
                const _p = pv(), _v = vv();
                beep('sine', 880 * _p, 1320 * _p, 0.14, 0.25 * _v, panX, { reverb: 0.28 });
                beep('triangle', 1320 * _p, 1760 * _p, 0.12, 0.14 * _v, panX, { reverb: 0.24 });
                ctx.scheduleSfx(() => { if (ctx.G.muted) return; beep('sine', 1760, 2200, 0.12, 0.16 * _v, panX, { reverb: 0.3 }); }, 90);
                ctx.scheduleSfx(() => ctx.fm(2637 * _p, 2637 * _p, 0.35, 0.16 * _v, panX, 3, { reverb: 0.35 }), 160);
            },
            // NEW: Last-life heartbeat — two soft low thumps on a slow cadence
            heartbeat() { if (ctx.G.muted) return;
                const _v = vv();
                beep('sine', 72, 44, 0.13, 0.19 * _v);
                ctx.scheduleSfx(() => { if (ctx.G.muted) return; beep('sine', 64, 40, 0.11, 0.13 * _v); }, 150);
            },
            // NEW: Multi-kill sting — low boom + rising fifth (punchy cluster reward)
            multiKill(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                ctx.fm(95 * _p, 38 * _p, 0.18, 0.42 * _v, panX, 1.8);
                beep('sawtooth', 220 * _p, 110 * _p, 0.12, 0.23 * _v, panX);
                ctx.scheduleSfx(() => { if (ctx.G.muted) return; beep('square', 392 * _p, 588 * _p, 0.14, 0.24 * _v, panX, { reverb: 0.24 }); beep('triangle', 588 * _p, 784 * _p, 0.12, 0.15 * _v, panX, { reverb: 0.24 }); }, 70);
            },
            // NEW: Dive feint — an aborting trill that falls away as the enemy pulls up
            feintWhistle(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                beep('triangle', 1400 * _p, 700 * _p, 0.1, 0.16 * _v, panX, { q: 2.4, reverb: 0.2 });
                beep('sine', 900 * _p, 1500 * _p, 0.14, 0.12 * _v, panX, { reverb: 0.22 });
                noise(0.06, 0.08 * _v, 3200, panX);
            },
            // NEW: Squad dive — a staggered three-note launch call
            squadDive(panX) { if (ctx.G.muted) return;
                const _v = vv();
                [0, 1, 2].forEach(i => ctx.scheduleSfx(() => { if (ctx.G.muted) return; ctx.fm((320 + i * 60), (150 + i * 30), 0.16, 0.2 * _v, panX, 2.0, { reverb: 0.2 }); }, i * 130));
                noise(0.12, 0.14 * _v, 2600, panX);
            },
            // NEW: Rage mode — a low growl under a revenge dive
            rageGrowl(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                ctx.fm(150 * _p, 74 * _p, 0.28, 0.3 * _v, panX, 2.2, { reverb: 0.2 });
                beep('sawtooth', 240 * _p, 120 * _p, 0.22, 0.2 * _v, panX, { q: 2.4 });
                noise(0.16, 0.14 * _v, 900, panX);
            },
            // NEW: Formation shuffle — a short mechanical displacement rumble
            shuffleRumble(panX) { if (ctx.G.muted) return;
                const _v = vv();
                noise(0.16, 0.16 * _v, 520, panX, { filterType: 'bandpass', q: 2.0 });
                beep('triangle', 300, 210, 0.14, 0.12 * _v, panX);
            },
            // NEW: Smolder ignition — variant-specific flame-up / turbine whine / sputter as the diver catches fire
            smolderIgnite(panX) { if (ctx.G.muted) return;
                const variant = arguments[1] || 'ember';
                const _p = pv(), _v = vv();
                if (variant === 'sputter') {
                    noise(0.14, 0.18 * _v, 2200, panX, { filterType: 'bandpass', q: 2.2 });
                    [0, 1, 2].forEach(i => ctx.scheduleSfx(() => {
                        if (ctx.G.muted) return;
                        ctx.fm((760 - i * 170) * _p, (320 - i * 60) * _p, 0.055, 0.16 * _v, panX, 2.5, { reverb: 0.16 });
                    }, i * 52));
                    return;
                }
                if (variant === 'spiral') {
                    ctx.fm(860 * _p, 230 * _p, 0.26, 0.2 * _v, panX, 3.2, { q: 2.2, reverb: 0.2 });
                    noise(0.2, 0.16 * _v, 1650, panX, { filterType: 'bandpass', q: 1.8 });
                    return;
                }
                if (variant === 'glide') {
                    noise(0.24, 0.22 * _v, 820, panX, { filterType: 'bandpass', q: 1.5 });
                    beep('sawtooth', 510 * _p, 145 * _p, 0.24, 0.17 * _v, panX, { q: 2.3, reverb: 0.2 });
                    return;
                }
                noise(0.22, 0.2 * _v, 1400, panX, { filterType: 'bandpass', q: 1.6 });
                beep('sawtooth', 680 * _p, 185 * _p, 0.22, 0.18 * _v, panX, { q: 2.0, reverb: 0.18 });
                beep('triangle', 340 * _p, 110 * _p, 0.18, 0.12 * _v, panX);
            },
            // NEW: Smolder mid-fall sputter — internal misfire pop and electric spark snap
            smolderSputter(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                noise(0.055, 0.14 * _v, 2800, panX, { filterType: 'bandpass', q: 2.2 });
                ctx.fm(960 * _p, 290 * _p, 0.06, 0.14 * _v, panX, 2.6, { reverb: 0.14 });
            },
            // NEW: Smolder detonation — implosion suck + deep delayed blast + shrapnel crackle tail
            smolderBlast(panX) { if (ctx.G.muted) return;
                const variant = arguments[1] || 'ember';
                const _p = pv(), _v = vv();
                beep('sine', 160 * _p, 580 * _p, 0.04, 0.14 * _v, panX, { q: 2.4 });
                ctx.fm(126 * _p, 34 * _p, 0.44, 0.44 * _v, panX, 2.4, { reverb: 0.28 });
                noise(0.32, 0.32 * _v, variant === 'sputter' ? 1150 : 700, panX, { reverb: 0.22 });
                beep('sine', 190 * _p, 52 * _p, 0.32, 0.24 * _v, panX, { reverb: 0.24 });
                ctx.scheduleSfx(() => {
                    if (ctx.G.muted) return;
                    noise(0.11, 0.14 * _v, 4400, panX, { reverb: 0.22 });
                }, 80);
            },
            // NEW: Armor/shield metallic ricochet ping
            armorRicochet(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                ctx.fm(2350 * _p, 3950 * _p, 0.055, 0.2 * _v, panX, 2.8, { reverb: 0.24 });
                beep('triangle', 1680 * _p, 920 * _p, 0.045, 0.12 * _v, panX);
            },
            // NEW: Weakpoint / critical hit crunch + harmonic chime
            critHit(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                ctx.fm(340 * _p, 95 * _p, 0.09, 0.26 * _v, panX, 2.1);
                beep('sine', 1568 * _p, 2093 * _p, 0.08, 0.18 * _v, panX, { reverb: 0.22 });
            },
            // NEW: Enemy dive launch thruster ignition burst
            diveIgnite(panX) { if (ctx.G.muted) return;
                const _p = pv(), _v = vv();
                noise(0.085, 0.13 * _v, 1950, panX, { filterType: 'bandpass', q: 2.0 });
                beep('sawtooth', 250 * _p, 520 * _p, 0.075, 0.13 * _v, panX, { q: 2.0, reverb: 0.14 });
            }
        };
        SFX.attackWarning = pan => ctx.fm(680, 920, 0.10, 0.18, pan, 1.5, { reverb: 0.18 });
        SFX.sectorAttack = (sector, pan) => { const i = GC.SECTORS.findIndex(s => s.id === sector); ctx.fm(180 + i * 40, 60 + i * 20, 0.18, 0.27, pan, 2 + i / 2, { reverb: 0.2 }); noise(0.06, 0.1, 1600, pan); };
        SFX.waveArrive = () => beep('triangle', 330, 495, 0.12, 0.11, undefined, { reverb: 0.18 });
        for (const [name, play] of Object.entries(SFX)) {
            SFX[name] = (...args) => {
                const previous = ctx.sfxPriority;
                ctx.sfxPriority = /Warning|parry|shield|pExplode|respawn|Ready/.test(name) ? 3 : /shoot|Shoot|rocketLaunch|weaponArm/.test(name) ? 2 : 1;
                try { return play.apply(SFX, args); } finally { ctx.sfxPriority = previous; }
            };
        }
        ctx.SFX = SFX;
    };
})();
