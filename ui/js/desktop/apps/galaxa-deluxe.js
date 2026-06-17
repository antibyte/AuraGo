(function () {
    'use strict';
    const W = 480, H = 640, PLAYER_SPEED = 220, PB_SPEED = 500, EB_SPEED = 260;
    const FCOLS = 10, FROWS = 5, ESP_X = 36, ESP_Y = 32, FTOP = 60, DIVE_SPD = 180;
    const EXTRA_LIFE = 20000, TITLE_IDLE = 15000;
const PU_TYPES = ['rapid', 'spread', 'shield', 'bomb', 'speed', 'magnet', 'laser', 'multibomb', 'timeslow', 'pierce', 'homing', 'supernova', 'freeze', 'levelskip', 'ricochet', 'drone', 'blackhole_bomb'];
        const PU_COL = { rapid: '#00ffcc', spread: '#ff6600', shield: '#4488ff', bomb: '#ff4444', speed: '#ffee00', magnet: '#ff44ff', laser: '#eeeeff', multibomb: '#cc2222', timeslow: '#aa44ff', pierce: '#88ffaa', homing: '#ff88aa', supernova: '#ffffff', freeze: '#88eeff', levelskip: '#ff88ff', ricochet: '#ffaa44', drone: '#44ffaa', blackhole_bomb: '#8844ff' };
        const PU_DUR = { rapid: 8000, spread: 10000, speed: 6000, magnet: 8000, laser: 5000, timeslow: 4000, pierce: 6000, homing: 0, freeze: 4000, ricochet: 8000, drone: 10000 };
        const PU_UPGRADE = { rapid: 'ultra_rapid', spread: 'mega_spread', speed: 'hyper_speed', magnet: 'super_magnet', laser: 'mega_laser', pierce: 'mega_pierce', ricochet: 'mega_ricochet', drone: 'dual_drone' };
        const PU_UPGRADE_COL = { ultra_rapid: '#00ffee', mega_spread: '#ff8800', hyper_speed: '#ffff44', super_magnet: '#ff88ff', mega_laser: '#ccddff', mega_pierce: '#aaffcc', mega_ricochet: '#ffcc66', dual_drone: '#66ffcc' };
        const PU_TRAIL_COL = { rapid: '0,255,204', ultra_rapid: '0,255,238', spread: '255,102,0', mega_spread: '255,136,0', shield: '68,136,255', speed: '255,238,0', hyper_speed: '255,255,68', magnet: '255,68,255', super_magnet: '255,136,255', laser: '180,200,255', mega_laser: '160,180,255', timeslow: '170,68,255', pierce: '136,255,170', mega_pierce: '170,255,204', homing: '255,136,170', freeze: '136,238,255', levelskip: '255,136,255', ricochet: '255,170,68', mega_ricochet: '255,204,102', drone: '68,255,170', dual_drone: '102,255,204', blackhole_bomb: '136,68,255' };
        // NEW: Powerup rarity tiers for weighted drops
        const PU_RARITY = {
            common: ['rapid', 'spread', 'speed', 'pierce'],
            uncommon: ['shield', 'magnet', 'laser', 'ricochet'],
            rare: ['homing', 'drone', 'timeslow', 'freeze'],
            legendary: ['bomb', 'multibomb', 'supernova', 'blackhole_bomb', 'levelskip']
        };
        const PU_RARITY_WEIGHT = { common: 50, uncommon: 30, rare: 15, legendary: 5 };
        // NEW: Powerup synergies — when two specific powerups are active simultaneously
        const PU_SYNERGIES = {
            'rapid+pierce': { name: 'Phaser', col: '#00ffaa', desc: 'Double fire rate + pierce' },
            'spread+homing': { name: 'Swarm', col: '#ff8844', desc: 'Spread bullets curve toward targets' },
            'shield+magnet': { name: 'Aegis', col: '#88aaff', desc: 'Shield reflects bullets + pulls powerups' },
            'laser+timeslow': { name: 'Chrono-Beam', col: '#cc88ff', desc: 'Laser slows hit enemies' },
            'drone+ricochet': { name: 'Bouncer', col: '#66ffaa', desc: 'Drone bullets bounce off walls' }
        };
    const COMBO_TIMEOUT = 2000;
    const COMBO_THRESH = [2, 3, 5, 8, 10, 15, 20];
    const COMBO_MULT = [1, 2, 4, 4, 8, 8, 16, 16];
    const COMBO_TEXT = ['', 'DOUBLE KILL', 'TRIPLE KILL', 'RAMPAGE', 'UNSTOPPABLE', 'GODLIKE', 'LEGENDARY', 'BEYOND'];
    // NEW: Attack pattern library for bullet-hell elements
    const ATTACK_PATTERNS = {
        spiral: function(e, count, speed, spread, ebSpd) {
            const a0 = (e.shootPh || 0) + Math.PI / 3;
            e.shootPh = a0;
            const bullets = [];
            for (let i = 0; i < count; i++) {
                const a = a0 + i * (Math.PI * 2 / count);
                bullets.push({ x: e.x, y: e.y + 8, w: 2, h: 4, vx: Math.cos(a) * ebSpd * speed, vy: Math.sin(a) * ebSpd * speed, kind: 'spiral' });
            }
            return bullets;
        },
        circle: function(e, count, speed, ebSpd) {
            const bullets = [];
            for (let i = 0; i < count; i++) {
                const a = (i / count) * Math.PI * 2;
                bullets.push({ x: e.x, y: e.y + 8, w: 2, h: 4, vx: Math.cos(a) * ebSpd * speed, vy: Math.sin(a) * ebSpd * speed, kind: 'spiral' });
            }
            return bullets;
        },
        wave: function(e, count, speed, amplitude, ebSpd) {
            const bullets = [];
            for (let i = 0; i < count; i++) {
                const a = -Math.PI / 2 + (i - count / 2) * amplitude;
                bullets.push({ x: e.x, y: e.y + 8, w: 2, h: 4, vx: Math.cos(a) * ebSpd * speed * 0.3, vy: Math.sin(a) * ebSpd * speed, kind: 'spiral' });
            }
            return bullets;
        },
        aimed_burst: function(e, count, speed, delay, ebSpd, targetX, targetY) {
            const dx = targetX - e.x, dy = targetY - e.y, dist = Math.hypot(dx, dy) || 1;
            const baseA = Math.atan2(dy, dx);
            const bullets = [];
            for (let i = 0; i < count; i++) {
                const a = baseA + (i - count / 2) * 0.15;
                bullets.push({ x: e.x, y: e.y + 8, w: 2, h: 5, vx: Math.cos(a) * ebSpd * speed, vy: Math.sin(a) * ebSpd * speed, kind: 'hunter' });
            }
            return bullets;
        },
        random_spread: function(e, count, speed, angle, ebSpd) {
            const bullets = [];
            for (let i = 0; i < count; i++) {
                const a = -Math.PI / 2 + (Math.random() - 0.5) * angle;
                bullets.push({ x: e.x, y: e.y + 8, w: 2, h: 6, vx: Math.cos(a) * ebSpd * speed, vy: Math.sin(a) * ebSpd * speed });
            }
            return bullets;
        },
        wall: function(e, rows, cols, speed, ebSpd) {
            const bullets = [];
            for (let r = 0; r < rows; r++) {
                for (let c = 0; c < cols; c++) {
                    bullets.push({ x: e.x + (c - cols / 2) * 14, y: e.y + 8 + r * 10, w: 2, h: 4, vx: 0, vy: ebSpd * speed });
                }
            }
            return bullets;
        }
    };
    // NEW: Stage modifiers (roulette) — random modifier per stage from stage 5+
    const STAGE_MODIFIERS = [
        { id: 'double_score', name: 'Double Score', desc: 'All points x2', col: '#ffcc00', apply: function(G) { G.scoreMult = 2; } },
        { id: 'glass_cannon', name: 'Glass Cannon', desc: 'Everyone has 1 HP', col: '#ff4444', apply: function(G) { G.glassCannon = true; } },
        { id: 'bullet_storm', name: 'Bullet Storm', desc: '2x enemy fire rate', col: '#ff8800', apply: function(G) { G.bulletStorm = true; } },
        { id: 'power_surge', name: 'Power Surge', desc: '3x powerup drops', col: '#44ff88', apply: function(G) { G.powerSurge = true; } },
        { id: 'darkness', name: 'Darkness', desc: 'Reduced visibility', col: '#444466', apply: function(G) { G.darkness = true; } },
        { id: 'turbo', name: 'Turbo', desc: '1.5x speed everything', col: '#44aaff', apply: function(G) { G.turbo = true; G.timeScale = 1.5; } }
    ];
    // NEW: Enemy type definitions for new enemy types
    const NEW_ENEMY_TYPES = {
        weaver: { name: 'Weaver', stageMin: 7, hp: 1, pts: [130, 260], col: '#ff8844', desc: 'Moves in sine wave, shoots in direction' },
        splitter: { name: 'Splitter', stageMin: 8, hp: 2, pts: [140, 280], col: '#88ff44', desc: 'Splits into 2 mini enemies on death' },
        shield_bee: { name: 'Shield Bee', stageMin: 4, hp: 2, pts: [70, 140], col: '#ffcc00', desc: 'Bee with 1 extra shield HP' },
        kamikaze: { name: 'Kamikaze', stageMin: 6, hp: 1, pts: [150, 300], col: '#ff2222', desc: 'Charges at player, explodes on contact' },
        carrier: { name: 'Carrier', stageMin: 9, hp: 3, pts: [200, 400], col: '#cc88ff', desc: 'Releases 3 bees on death' },
        teleporter: { name: 'Teleporter', stageMin: 10, hp: 2, pts: [160, 320], col: '#44ffff', desc: 'Teleports every 2s to random position' }
    };
    const ACHIEVEMENTS = {
        first_blood: { name: 'First Blood', desc: 'Kill your first enemy' },
        combo_king: { name: 'Combo King', desc: 'Reach 15 combo' },
        untouchable: { name: 'Untouchable', desc: 'Complete a challenge stage without damage' },
        dual_wielder: { name: 'Dual Wielder', desc: 'Rescue a captured fighter' },
        survivor: { name: 'Survivor', desc: 'Reach stage 10' },
        legend: { name: 'Legend', desc: 'Reach stage 20' },
        perfectionist: { name: 'Perfectionist', desc: 'Get 3 perfect challenge stages' },
        power_collector: { name: 'Power Collector', desc: 'Collect all powerup types in one run' },
        speed_demon: { name: 'Speed Demon', desc: 'Complete a stage in under 30 seconds' },
        boss_slayer: { name: 'Boss Slayer', desc: 'Defeat 10 bosses total' },
        // NEW Phase 3 achievements
        millionaire: { name: 'Millionaire', desc: 'Score 1,000,000 points' },
        no_powerups: { name: 'Purist', desc: 'Reach stage 10 without collecting any powerup' },
        one_life: { name: 'Iron Man', desc: 'Reach stage 5 starting with 1 life' },
        bullet_hell: { name: 'Bullet Hell', desc: 'Have 100+ enemy bullets on screen at once' },
        boss_rusher: { name: 'Boss Rusher', desc: 'Reach stage 10 in Boss Rush mode' },
        collector: { name: 'Collector', desc: 'Collect all 17 powerup types in a single run' },
        speedrun: { name: 'Speedrunner', desc: 'Complete stages 1-5 in under 2 minutes' },
        pacifist: { name: 'Pacifist', desc: 'Clear a stage without firing (drones/blackhole only)' },
        overcharge: { name: 'Overcharged', desc: 'Use Overcharge 5 times in one run' },
        shopaholic: { name: 'Shopaholic', desc: 'Visit the shop 10 times' },
        daily_warrior: { name: 'Daily Warrior', desc: 'Complete 7 daily challenges' },
        flawless_boss: { name: 'Flawless', desc: 'Defeat a boss without taking damage' },
        combo_god: { name: 'Combo God', desc: 'Reach 30 combo' },
        weapon_master: { name: 'Weapon Master', desc: 'Reach weapon level 4' },
        ship_collector: { name: 'Fleet Commander', desc: 'Reach stage 10 with all 4 ship types' },
        no_damage_run: { name: 'Untouchable II', desc: 'Complete 3 stages in a row without damage' },
        black_hole_master: { name: 'Singularity', desc: 'Kill 20 enemies with one Black Hole Bomb' },
        freeze_frame: { name: 'Cryo Master', desc: 'Kill 50 enemies while they are frozen' },
        ricochet_master: { name: 'Trick Shot', desc: 'Kill 30 enemies with ricochet bullets' },
        supernova_survivor: { name: 'Supernova Survivor', desc: 'Survive an enemy-triggered Supernova' }
    };
    const SHIP_TYPES = {
        classic: { name: 'Classic', speedMult: 1, lifeMod: 0, hitboxMod: 0, invMult: 1, diveTargetMod: 1 },
        interceptor: { name: 'Interceptor', speedMult: 1.3, lifeMod: -1, hitboxMod: -2, invMult: 1, diveTargetMod: 1 },
        heavy: { name: 'Heavy', speedMult: 0.8, lifeMod: 1, hitboxMod: 2, invMult: 1, diveTargetMod: 1 },
        stealth: { name: 'Stealth', speedMult: 1, lifeMod: 0, hitboxMod: 0, invMult: 0.6, diveTargetMod: 0.5 }
    };
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
        let scale = 1, tick = 0, rafId = 0, clockT = 0, resizeRaf = 0;
        function frameDelta() {
            const now = performance.now();
            const raw = clockT ? (now - clockT) / 1000 : (1 / 60);
            clockT = now;
            return Math.min(Math.max(raw, 0.001), 0.05);
        }

        function loadSettings() {
            try { const s = JSON.parse(localStorage.getItem('galaxa_settings') || '{}'); return { vol: s.vol || 30, diff: s.diff || 'normal', mute: s.mute || false, ship: s.ship || 'classic', mode: s.mode || 'classic', crt: s.crt !== undefined ? s.crt : true, particles: s.particles || 'high', shake: s.shake !== undefined ? s.shake : 1 }; } catch (e) { return { vol: 30, diff: 'normal', mute: false, ship: 'classic', mode: 'classic', crt: true, particles: 'high', shake: 1 }; }
        }
        function saveSettings() { try { localStorage.setItem('galaxa_settings', JSON.stringify(settings)); } catch (e) {} }
        function loadAchievements() { try { const a = JSON.parse(localStorage.getItem('galaxa_achievements') || '{}'); return a; } catch (e) { return {}; } }
        function saveAchievements() { try { localStorage.setItem('galaxa_achievements', JSON.stringify(G.achievements)); } catch (e) {} }
        function unlockAchievement(id) {
            if (G.achievements[id]) return;
            G.achievements[id] = true; saveAchievements();
            const def = ACHIEVEMENTS[id];
            G.achievementPopups.push({ text: (def ? def.name : id), t: 0, dur: 3000 });
            SFX.perfect();
        }
        let settings = loadSettings();
        if (!settings.crt) wrapEl.classList.remove('galaxa-crt');

        function diffMod(key) {
            const ship = SHIP_TYPES[settings.ship] || SHIP_TYPES.classic;
            if (settings.diff === 'easy') return { diveRate: 0.7, ebSpd: 0.8, lives: 5 + ship.lifeMod, puFromBee: true }[key];
            if (settings.diff === 'hard') return { diveRate: 1.5, ebSpd: 1.3, lives: 2 + ship.lifeMod, puFromBee: false }[key];
            return { diveRate: 1, ebSpd: 1, lives: 3 + ship.lifeMod, puFromBee: true }[key];
        }
        function getShipSpeed() { return PLAYER_SPEED * (SHIP_TYPES[settings.ship] || SHIP_TYPES.classic).speedMult; }
        function getShipHitbox() { const mod = (SHIP_TYPES[settings.ship] || SHIP_TYPES.classic).hitboxMod; return { x: G.p.x - 6 + mod, y: G.p.y - 6 + mod, w: 12 - mod * 2, h: 12 - mod * 2 }; }
        function getShipInvMult() { return (SHIP_TYPES[settings.ship] || SHIP_TYPES.classic).invMult; }

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
            damageVignetteT: 0, freezeT: 0, lightningT: 0, lightningX: 0, bgTheme: 'nebula',
            pauseSel: 0, settingsSel: 0, settingsVolDrag: false,
            combo: 0, comboTimer: 0, comboMult: 1, comboBanner: null,
            trails: [],
            timeScale: 1, timeSlowTimer: 0,
            bossWarningT: 0, bossWarningShown: false,
            weaponLv: 1, killCount: 0, puUpgrade: null, upgradeBanner: null,
            slowMoT: 0, chromAb: 0, displayScore: 0, shipTilt: 0, muzzleT: 0, deathParts: [], pendingBooms: [], levelSkipTimer: 0, stageWipeT: 0,
            introTmr: 0, stageEmptyT: 0, stageClearLock: 0,
            beatPhase: 0, beatT: 0, plasmaRings: [], titleParts: [], drones: [], droneTimer: 0,
            blackhole: null,
            // NEW Phase 2-3 state fields
            overcharge: 0, overchargeTimer: 0, scoreMult: 1, glassCannon: false, bulletStorm: false,
            powerSurge: false, darkness: false, turbo: false, stageModifier: null,
            credits: parseInt(localStorage.getItem('galaxa_credits') || '0'), shopOpen: false,
            noDamageStages: 0, pacifistStage: true, frozenKills: 0, ricochetKills: 0,
            blackholeKills: 0, dailyStreak: parseInt(localStorage.getItem('galaxa_daily_streak') || '0'),
            shipStageProgress: {}, transitionType: 0,
            swipeT: 0, swipeDir: 1, portalT: 0, portalR: 0, glitchT: 0, glitchStrips: [],
            _closeCallCooldown: 0, _synergyChecked: null, shieldReflect: false, laserSlow: false, droneRicochet: false,
            inp: { l: false, r: false, f: false, fp: false, s: false, sp: false, p: false, pp: false, u: false, d: false, rp: false, lp: false, up: false, dp: false },
            kb: { l: false, r: false, u: false, d: false, f: false, s: false, p: false },
            gp: { l: false, r: false, u: false, d: false, f: false, s: false, p: false },
            muted: settings.mute, vol: settings.vol / 100, _prevSt: 'TITLE', gameMode: 'classic',
            achievements: loadAchievements(), achievementPopups: [], collectedPU: new Set(), stageStartTime: 0, perfectCount: 0, bossKillTotal: parseInt(localStorage.getItem('galaxa_boss_kills') || '0')
        };
        G.lives = diffMod('lives');

        let actx = null;
        let masterCompressor = null;
        let reverbNode = null;
        let reverbGain = null;
        function audio() {
            if (!actx) try { actx = new (window.AudioContext || window.webkitAudioContext)(); } catch (e) { return null; }
            if (actx && actx.state === 'suspended') actx.resume();
            if (actx && !masterCompressor) {
                masterCompressor = actx.createDynamicsCompressor();
                masterCompressor.threshold.value = -12;
                masterCompressor.knee.value = 10;
                masterCompressor.ratio.value = 4;
                masterCompressor.attack.value = 0.003;
                masterCompressor.release.value = 0.15;
                masterCompressor.connect(actx.destination);
                try {
                    reverbNode = actx.createConvolver();
                    const rate = actx.sampleRate, length = Math.floor(rate * 0.4);
                    const impulse = actx.createBuffer(2, length, rate);
                    for (let ch = 0; ch < 2; ch++) { const d = impulse.getChannelData(ch); for (let i = 0; i < length; i++) d[i] = (Math.random() * 2 - 1) * Math.pow(1 - i / length, 3); }
                    reverbNode.buffer = impulse;
                    reverbGain = actx.createGain(); reverbGain.gain.value = 0.12;
                    reverbNode.connect(reverbGain); reverbGain.connect(masterCompressor);
                } catch (_) { reverbNode = null; reverbGain = null; }
            }
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
            shoot(panX) { const _pv = 0.95 + Math.random() * 0.1; beep('sine', 800 * _pv, 1200 * _pv, 0.08, 0.3, panX); beep('square', 400 * _pv, 200 * _pv, 0.05, 0.08, panX); },
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
            enemyHitSfx(panX) { beep('sine', 380, 180, 0.03, 0.25, panX); beep('sine', 550, 300, 0.02, 0.15, panX); },
            stalkerDive(panX) { beep('sawtooth', 900, 300, 0.2, 0.2, panX); noise(0.1, 0.15, 5000, panX); },
            hunterDive(panX) { beep('sawtooth', 1200, 180, 0.35, 0.28, panX); noise(0.15, 0.22, 3500, panX); beep('square', 400, 120, 0.12, 0.18, panX); },
            hunterShot(panX) { beep('sawtooth', 700, 350, 0.1, 0.22, panX); beep('square', 500, 200, 0.06, 0.14, panX); },
            spinnerShot(panX) { beep('sine', 1400, 2200, 0.07, 0.2, panX); beep('triangle', 900, 1500, 0.05, 0.12, panX); },
            bomberDrop(panX) { beep('sawtooth', 300, 80, 0.12, 0.2, panX); noise(0.08, 0.12, 800, panX); },
            lasherShot(panX) { beep('sine', 600, 1800, 0.14, 0.22, panX); beep('triangle', 400, 1200, 0.08, 0.15, panX); },
            sniperShot(panX) { beep('sine', 1800, 600, 0.08, 0.25, panX); beep('square', 1200, 400, 0.05, 0.12, panX); },
            comboMilestone(n, panX) { [880, 1100, 1320, 1760, 2200].forEach((f, i) => { setTimeout(() => beep('sine', f, f, 0.08, 0.2 + n * 0.03, panX), i * 40); }); },
            shieldReflect(panX) { beep('triangle', 3000, 1500, 0.06, 0.3, panX); beep('sine', 2000, 4000, 0.04, 0.2, panX); },
            closeCall(panX) { noise(0.06, 0.12, 6000, panX); beep('sine', 1500, 800, 0.04, 0.1, panX); },
            envAmbience(theme) { if (theme === 'storm') noise(2, 0.03, 400); else if (theme === 'blackhole') { beep('sine', 40, 40, 2, 0.04); beep('sine', 55, 55, 2, 0.03); } else if (theme === 'crystal') beep('sine', 2400, 2000, 0.3, 0.02); }
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
                this.stop(); this.playing = theme; this.stopped = false;
                const a = audio(); if (!a) return;
                if (prevTheme && prevTheme !== theme && !G.muted) {
                    const stingerVol = G.vol * 0.15;
                    if (theme === 'boss' || theme === 'miniboss' || theme === 'deep_boss') {
                        beep('sawtooth', 220, 110, 0.3, stingerVol);
                        setTimeout(() => beep('sawtooth', 165, 82, 0.2, stingerVol), 150);
                    } else if (theme === 'gameplay' && (prevTheme === 'boss' || prevTheme === 'victory')) {
                        [523, 659, 784].forEach((f, i) => setTimeout(() => beep('sine', f, f, 0.1, stingerVol), i * 60));
                    }
                }
                this.masterGain = a.createGain();
                this.masterGain.gain.value = G.muted ? 0 : G.vol * 0.35;
                this.masterGain.connect(masterCompressor || a.destination);
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
                                g.gain.setValueAtTime(voice.vol * (G.muted ? 0 : 1) * (vn === 'arpeggio' ? Math.min(1, Math.max(0, (this.intensity - 4) / 4)) : vn === 'lead' ? Math.min(1, Math.max(0.3, this.intensity / 8)) : vn === 'harmony' ? Math.min(1, Math.max(0, (this.intensity - 2) / 5)) : 1), a.currentTime + offset);
                                g.gain.exponentialRampToValueAtTime(0.001, a.currentTime + offset + n.d * beatDur + 0.01);
                                if (vn === 'bass') {
                                    const filt = a.createBiquadFilter(); filt.type = 'lowpass';
                                    filt.frequency.setValueAtTime(250, a.currentTime + offset);
                                    filt.frequency.linearRampToValueAtTime(700, a.currentTime + offset + n.d * beatDur * 0.3);
                                    filt.frequency.linearRampToValueAtTime(180, a.currentTime + offset + n.d * beatDur);
                                    o.connect(filt).connect(g).connect(this.masterGain);
                                } else { o.connect(g).connect(this.masterGain); }
                                if (reverbNode && (vn === 'lead' || vn === 'harmony')) {
                                    const rvbSend = a.createGain(); rvbSend.gain.value = 0.08;
                                    g.connect(rvbSend); rvbSend.connect(reverbNode);
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
                this.intensity = level;
                const volMult = 1 + Math.min(level, 5) * 0.08;
                if (this.masterGain) this.masterGain.gain.value = G.muted ? 0 : G.vol * 0.35 * volMult;
            }
        };

        const PTS = { bee: [50, 100], butterfly: [80, 160], boss: [400, 800], miniboss: [600, 1200], stalker: [120, 240], sniper: [100, 200], hunter: [200, 400], spinner: [90, 180], bomber: [110, 220], lasher: [80, 160], weaver: [130, 260], splitter: [140, 280], shield_bee: [70, 140], kamikaze: [150, 300], carrier: [200, 400], teleporter: [160, 320] };
        const SP = buildSprites();
        SP.shield_bee = SP.bee; // Reuse bee sprite for shield bee
        const STARS = [];
        const STAR_COLS = ['#ffffff', '#ffeecc', '#ccddff', '#ffcccc', '#ccffcc'];
        for (let i = 0; i < 60; i++) STARS.push({ x: Math.random() * W, y: Math.random() * H, sp: 10 + Math.random() * 20, br: 0.15 + Math.random() * 0.3, sz: 1, layer: 0, col: STAR_COLS[Math.floor(Math.random() * STAR_COLS.length)] });
        for (let i = 0; i < 45; i++) STARS.push({ x: Math.random() * W, y: Math.random() * H, sp: 30 + Math.random() * 30, br: 0.3 + Math.random() * 0.4, sz: Math.random() > 0.7 ? 2 : 1, layer: 1, col: STAR_COLS[Math.floor(Math.random() * STAR_COLS.length)] });
        for (let i = 0; i < 30; i++) STARS.push({ x: Math.random() * W, y: Math.random() * H, sp: 60 + Math.random() * 60, br: 0.5 + Math.random() * 0.5, sz: 2, layer: 2, col: STAR_COLS[Math.floor(Math.random() * STAR_COLS.length)] });
        for (let i = 0; i < 15; i++) STARS.push({ x: Math.random() * W, y: Math.random() * H, sp: 100 + Math.random() * 80, br: 0.7 + Math.random() * 0.3, sz: 2, layer: 3, twinkle: Math.random() * 6.28, col: '#ffffff' });
        let shootingStars = [];
        let nebulaCv = null, nebulaColors = [];
        const radialGradientCache = new Map();
        const spriteAtlasCache = new WeakMap();
        const flashPixelColors = {};
        let bgPlanets = [], bgComets = [];
        function initBG() {
            bgPlanets = [];
            const themes = ['nebula', 'asteroid', 'blackhole', 'ringed', 'storm', 'crystal'];
            const theme = themes[(G.stage - 1) % themes.length];
            G.bgTheme = theme;
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
                stalker: [
                    p([
                        '00000000880000000000', '00000008898000000000', '00000088998800000000', '00000889aa9880000000',
                        '00088899aa9888000000', '00888999aa9988800000', '08889999aa9998880000', '88899999aa9999888000',
                        '088999999a9999880000', '0088999999a998800000', '0008899999a988000000', '000088999aa988000000',
                        '00000889aa9880000000', '00000088998800000000', '00000008898000000000', '00000000880000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000880000000000', '00000008898000000000', '00000088998800000000', '00000889aa9880000000',
                        '00008899aa9880000000', '00088999aa9988000000', '00889999aa9998800000', '08899999aa9999880000',
                        '889999999a9999988000', '0889999999a999880000', '0088999999a998800000', '0008899999a988000000',
                        '000088999aa988000000', '00000889aa9880000000', '00000088998800000000', '00000008898000000000',
                        '00000000880000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000880000000000', '00000008898000000000', '00000088998800000000', '00000889aa9880000000',
                        '00008899aa9880000000', '00088999aa9988000000', '00889999aa9998800000', '08899999aa9999880000',
                        '88899999aa9999888000', '08889999aa9998880000', '00888999aa9988800000', '00088899aa9888000000',
                        '00000889aa9880000000', '00000088998800000000', '00000008898000000000', '00000000880000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n'))
                ],
                stalkerC: { 8: '#6622aa', 9: '#8844cc', a: '#aa66ee' },
                sniper: [
                    p([
                        '00000000000000000000', '00000000440000000000', '00000004454000000000', '00000044554400000000',
                        '00000445554400000000', '00004455554400000000', '00044555554400000000', '00445555554400000000',
                        '00445555554400000000', '00044555554400000000', '00004455554400000000', '00000445554400000000',
                        '00000044554400000000', '00000004454000000000', '00000000440000000000', '00000000000000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000000000000000', '00000000440000000000', '00000004454000000000', '00000044554400000000',
                        '00000445554400000000', '00004455654400000000', '00044556654400000000', '00445566654400000000',
                        '00445566654400000000', '00044556654400000000', '00004455654400000000', '00000445554400000000',
                        '00000044554400000000', '00000004454000000000', '00000000440000000000', '00000000000000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000000000000000', '00000000440000000000', '00000004454000000000', '00000044554400000000',
                        '00000445554400000000', '00004455554400000000', '00044555554400000000', '00445555554400000000',
                        '00445555554400000000', '00044555554400000000', '00004455554400000000', '00000445554400000000',
                        '00000044554400000000', '00000004454000000000', '00000000440000000000', '00000000000000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n'))
                ],
                sniperC: { 4: '#ffcc00', 5: '#ffaa00', 6: '#ffff44' },
                hunter: [
                    p([
                        '00000000000000000000', '00000000888800000000', '00000008899880000000', '00000088999aa8800000',
                        '00000889999aa9880000', '00088999999aaa988000', '00889999999aaaa98800', '08889999999aaaa98880',
                        '88889999999aaaa98888', '08889999999aaaa98880', '00889999999aaa988000', '0008899999aaa9880000',
                        '000008899aa988000000', '00000088998800000000', '00000008898000000000', '00000000888000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000000000000000', '00000000888800000000', '00000008899880000000', '00000088999aa8800000',
                        '00000889999aa9880000', '00088999999aaa988000', '00889999999aaaa98800', '08889999999aaaa98880',
                        '88889999999aaaa98888', '08889999999aaaa98880', '00889999999aaa988000', '0008899999aaa9880000',
                        '000008899aa988000000', '00000088998800000000', '00000008898000000000', '00000000888000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000000000000000', '00000000bb8800000000', '0000000bb99880000000', '000000bb999aa8800000',
                        '00000bb9999aa9880000', '0000bb99999aaa988000', '000bb999999aaa988000', '00bb9999999aaa988800',
                        '0bb99999999aaa988880', '00bb9999999aaa988800', '000bb99999aaa9880000', '0000bb9999aaa9880000',
                        '00000bb999aa98800000', '000000bb998880000000', '0000000bb98800000000', '00000000bb8800000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n'))
                ],
                hunterC: { 8: '#cc4400', 9: '#ff6600', a: '#ffaa00', b: '#ff2200' },
                spinner: [
                    p([
                        '00000000000000000000', '00000000660000000000', '00000006666000000000', '00000066776600000000',
                        '00000667776600000000', '00066666666666000000', '00666666666666600000', '06666666666666660000',
                        '66666666666666666600', '06666666666666660000', '00666666666666600000', '00066666666666000000',
                        '00000667776600000000', '00000066776600000000', '00000006666000000000', '00000000660000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000000000000000', '00000000660000000000', '00000006666000000000', '00000066776600000000',
                        '00000667776600000000', '00066666666666000000', '06666666666666660000', '60666666666666606000',
                        '06666666666666660000', '00066666666666000000', '00000667776600000000', '00000066776600000000',
                        '00000006666000000000', '00000000660000000000', '00000000000000000000', '00000000000000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000000000000000', '00000000660000000000', '00000006666000000000', '00000066776600000000',
                        '00000667776600000000', '00066666666666000000', '00666666666666600000', '06666666666666660000',
                        '66666666666666666600', '06666666666666660000', '00666666666666600000', '00066666666666000000',
                        '00000667776600000000', '00000066776600000000', '00000006666000000000', '00000000660000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n'))
                ],
                spinnerC: { 6: '#00cccc', 7: '#44ffff', 8: '#0088aa' },
                bomber: [
                    p([
                        '00000000000000000000', '00000000880000000000', '00000008898000000000', '00000088998800000000',
                        '00000889999880000000', '00008899999888000000', '00088999999988000000', '00889999999998800000',
                        '08889999999998880000', '00889999999998800000', '00088999999988000000', '00008899999888000000',
                        '00000889999880000000', '00000088998800000000', '00000008898000000000', '00000000880000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000000000000000', '00000000880000000000', '00000008898000000000', '00000088998800000000',
                        '00000889999880000000', '00008899999888000000', '000889aa999988000000', '00889aaa999988000000',
                        '08889aaa999988800000', '00889aaa999988000000', '000889aa999988000000', '00008899999888000000',
                        '00000889999880000000', '00000088998800000000', '00000008898000000000', '00000000880000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000000000000000', '00000000880000000000', '00000008898000000000', '00000088998800000000',
                        '00000889999880000000', '00008899999888000000', '00088999999988000000', '00889999999998800000',
                        '08889999999998880000', '00889999999998800000', '00088999999988000000', '00008899999888000000',
                        '00000889999880000000', '00000088998800000000', '00000008898000000000', '00000000880000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n'))
                ],
                bomberC: { 8: '#aa44cc', 9: '#cc66ff', a: '#ff44aa' },
                lasher: [
                    p([
                        '00000000000000000000', '00000000440000000000', '00000004454000000000', '00000044554400000000',
                        '00000445554400000000', '00004455554400000000', '00044555554400000000', '00445555554400000000',
                        '00445555554400000000', '00044555554400000000', '00004455554400000000', '00000445554400000000',
                        '00000044554400000000', '00000004454000000000', '00000000440000000000', '00000000000000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000000000000000', '00000000440000000000', '00000004454000000000', '00000044554400000000',
                        '00000445554400000000', '00004455654400000000', '00044556654400000000', '00445566654400000000',
                        '00445566654400000000', '00044556654400000000', '00004455654400000000', '00000445554400000000',
                        '00000044554400000000', '00000004454000000000', '00000000440000000000', '00000000000000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000000000000000', '00000000440000000000', '00000004454000000000', '00000044554400000000',
                        '00000445554400000000', '00004455554400000000', '00044555554400000000', '00445555554400000000',
                        '00445555554400000000', '00044555554400000000', '00004455554400000000', '00000445554400000000',
                        '00000044554400000000', '00000004454000000000', '00000000440000000000', '00000000000000000000',
                        '00000000000000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n'))
                ],
                lasherC: { 4: '#44ff88', 5: '#00cc66', 6: '#aaffcc' },
                // NEW: Enemy type sprites (reuse existing shapes with new colors)
                weaver: [
                    p([
                        '00000000880000000000', '00000008898000000000', '00000088998800000000', '00000889999880000000',
                        '00008899999888000000', '00088999999988000000', '00889999999998800000', '08889999999998880000',
                        '00889999999998800000', '00088999999988000000', '00008899999888000000', '00000889999880000000',
                        '00000088998800000000', '00000008898000000000', '00000000880000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000880000000000', '00000008898000000000', '00000088998800000000', '00000889999880000000',
                        '00008899999888000000', '00088999999988000000', '00889999999998800000', '08889999999998880000',
                        '00889999999998800000', '00088999999988000000', '00008899999888000000', '00000889999880000000',
                        '00000088998800000000', '00000008898000000000', '00000000880000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000880000000000', '00000008898000000000', '00000088998800000000', '00000889999880000000',
                        '00008899999888000000', '00088999999988000000', '00889999999998800000', '08889999999998880000',
                        '00889999999998800000', '00088999999988000000', '00008899999888000000', '00000889999880000000',
                        '00000088998800000000', '00000008898000000000', '00000000880000000000', '00000000000000000000'
                    ].join('\n'))
                ],
                weaverC: { 8: '#ff8844', 9: '#ffaa66' },
                splitter: [
                    p([
                        '00000000660000000000', '00000006666000000000', '00000066776600000000', '00000667776600000000',
                        '00066666666666000000', '00666666666666600000', '06666666666666660000', '66666666666666666600',
                        '06666666666666660000', '00666666666666600000', '00066666666666000000', '00000667776600000000',
                        '00000066776600000000', '00000006666000000000', '00000000660000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000660000000000', '00000006666000000000', '00000066776600000000', '00000667776600000000',
                        '00066666666666000000', '00666666666666600000', '06666666666666660000', '66666666666666666600',
                        '06666666666666660000', '00666666666666600000', '00066666666666000000', '00000667776600000000',
                        '00000066776600000000', '00000006666000000000', '00000000660000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000660000000000', '00000006666000000000', '00000066776600000000', '00000667776600000000',
                        '00066666666666000000', '00666666666666600000', '06666666666666660000', '66666666666666666600',
                        '06666666666666660000', '00666666666666600000', '00066666666666000000', '00000667776600000000',
                        '00000066776600000000', '00000006666000000000', '00000000660000000000', '00000000000000000000'
                    ].join('\n'))
                ],
                splitterC: { 6: '#88ff44', 7: '#aaff66' },
                shield_bee: null, // Will be set after SP is built
                shield_beeC: { 4: '#ffcc00', 5: '#ffaa00', 6: '#ff4444' },
                kamikaze: [
                    p([
                        '00000000440000000000', '00000004454000000000', '00000044554400000000', '00000445554400000000',
                        '00004455554400000000', '00044555554400000000', '00445555554400000000', '00445555554400000000',
                        '00044555554400000000', '00004455554400000000', '00000445554400000000', '00000044554400000000',
                        '00000004454000000000', '00000000440000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000440000000000', '00000004454000000000', '00000044554400000000', '00000445554400000000',
                        '00004455554400000000', '00044555554400000000', '00445555554400000000', '00445555554400000000',
                        '00044555554400000000', '00004455554400000000', '00000445554400000000', '00000044554400000000',
                        '00000004454000000000', '00000000440000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000440000000000', '00000004454000000000', '00000044554400000000', '00000445554400000000',
                        '00004455554400000000', '00044555554400000000', '00445555554400000000', '00445555554400000000',
                        '00044555554400000000', '00004455554400000000', '00000445554400000000', '00000044554400000000',
                        '00000004454000000000', '00000000440000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n'))
                ],
                kamikazeC: { 4: '#ff2222', 5: '#ff4444' },
                carrier: [
                    p([
                        '00000000888800000000', '00000008899880000000', '00000088999aa8800000', '00000889999aa9880000',
                        '00088999999aaa988000', '00889999999aaaa98800', '08889999999aaaa98880', '88889999999aaaa98888',
                        '08889999999aaaa98880', '00889999999aaa988000', '0008899999aaa9880000', '000008899aa988000000',
                        '00000088998800000000', '00000008898000000000', '00000000888000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000888800000000', '00000008899880000000', '00000088999aa8800000', '00000889999aa9880000',
                        '00088999999aaa988000', '00889999999aaaa98800', '08889999999aaaa98880', '88889999999aaaa98888',
                        '08889999999aaaa98880', '00889999999aaa988000', '0008899999aaa9880000', '000008899aa988000000',
                        '00000088998800000000', '00000008898000000000', '00000000888000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000888800000000', '00000008899880000000', '00000088999aa8800000', '00000889999aa9880000',
                        '00088999999aaa988000', '00889999999aaaa98800', '08889999999aaaa98880', '88889999999aaaa98888',
                        '08889999999aaaa98880', '00889999999aaa988000', '0008899999aaa9880000', '000008899aa988000000',
                        '00000088998800000000', '00000008898000000000', '00000000888000000000', '00000000000000000000'
                    ].join('\n'))
                ],
                carrierC: { 8: '#cc88ff', 9: '#ddaaff', a: '#eeccff' },
                teleporter: [
                    p([
                        '00000000660000000000', '00000006666000000000', '00000066776600000000', '00000667776600000000',
                        '00006667676660000000', '00066666666666000000', '00666666666666600000', '60666666666666060000',
                        '66006666666666000060', '00006666666660000000', '00000066660000000000', '00000060060000000000',
                        '00000060060000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000660000000000', '00000006666000000000', '00000066776600000000', '00000667776600000000',
                        '00006667676660000000', '00066666666666000000', '00666666666666600000', '60666666666666060000',
                        '66006666666666000060', '00006666666660000000', '00000066660000000000', '00000060060000000000',
                        '00000060060000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n')),
                    p([
                        '00000000660000000000', '00000006666000000000', '00000066776600000000', '00000667776600000000',
                        '00006667676660000000', '00066666666666000000', '00666666666666600000', '60666666666666060000',
                        '66006666666666000060', '00006666666660000000', '00000066660000000000', '00000060060000000000',
                        '00000060060000000000', '00000000000000000000', '00000000000000000000', '00000000000000000000'
                    ].join('\n'))
                ],
                teleporterC: { 6: '#44ffff', 7: '#66ffff' },
                bossRedC: { 8: '#cc2222', 9: '#ff4444', a: '#ff6666', b: '#ffaa44', c: '#ffcc00' },
                bossBlueC: { 8: '#2244cc', 9: '#4488ff', a: '#66aaff', b: '#88ccff', c: '#aaeeff' },
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

        function getSpriteData(sp, cols, flash) {
            const colorKey = flash ? flashPixelColors : cols;
            if (!sp || !colorKey || typeof colorKey !== 'object') return null;
            let byColor = spriteAtlasCache.get(sp);
            if (!byColor) { byColor = new WeakMap(); spriteAtlasCache.set(sp, byColor); }
            if (byColor.has(colorKey)) return byColor.get(colorKey);
            const h = sp.length, w = sp[0] ? sp[0].length : 0;
            const pixels = [];
            for (let r = 0; r < h; r++) for (let cl = 0; cl < sp[r].length; cl++) {
                const v = sp[r][cl]; if (!v) continue;
                pixels.push({ x: cl, y: r, color: flash ? '#fff' : (cols[v] || '#fff') });
            }
            let atlas = null;
            try {
                atlas = new OffscreenCanvas(w, h);
                const sctx = atlas.getContext('2d');
                for (const px of pixels) { sctx.fillStyle = px.color; sctx.fillRect(px.x, px.y, 1, 1); }
            } catch (_) {}
            const result = { pixels, atlas };
            byColor.set(colorKey, result);
            return result;
        }

        function drawSp(cv, sp, cols, x, y, flash, noCache) {
            if (noCache) {
                const px = [];
                for (let r = 0; r < sp.length; r++) for (let cl = 0; cl < sp[r].length; cl++) {
                    const v = sp[r][cl]; if (!v) continue;
                    px.push({ x: cl, y: r, color: flash ? '#fff' : (cols[v] || '#fff') });
                }
                cv.save(); cv.translate(Math.floor(x), Math.floor(y));
                for (let i = 0, n = px.length; i < n; i++) { cv.fillStyle = px[i].color; cv.fillRect(px[i].x, px[i].y, 1, 1); }
                cv.restore();
                return;
            }
            const data = getSpriteData(sp, cols, flash);
            if (!data) return;
            if (data.atlas) { cv.drawImage(data.atlas, Math.floor(x), Math.floor(y)); }
            else {
                cv.save(); cv.translate(Math.floor(x), Math.floor(y));
                for (let i = 0, n = data.pixels.length; i < n; i++) { cv.fillStyle = data.pixels[i].color; cv.fillRect(data.pixels[i].x, data.pixels[i].y, 1, 1); }
                cv.restore();
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
        function updateBackground(dt) {
            const warp = G.warpT > 0 ? 10 : 1;
            for (const s of STARS) {
                const lm = s.layer === 0 ? 0.3 : s.layer === 1 ? 0.6 : s.layer === 2 ? 1 : 1.4;
                s.y += s.sp * dt * warp * lm;
                if (s.y > H) { s.y = 0; s.x = Math.random() * W; s.col = STAR_COLS[Math.floor(Math.random() * STAR_COLS.length)]; }
                if (s.layer === 3) s.twinkle += dt * 3;
            }
            if (Math.random() < 0.003 && warp <= 1) {
                shootingStars.push({ x: Math.random() * W, y: -5, vx: -40 - Math.random() * 80, vy: 120 + Math.random() * 100, life: 1.5, t: 0, col: '#ffffff' });
            }
            let sslen = 0;
            for (let i = 0; i < shootingStars.length; i++) {
                const ss = shootingStars[i];
                ss.prevX = ss.x; ss.prevY = ss.y;
                ss.x += ss.vx * dt; ss.y += ss.vy * dt; ss.t += dt;
                if (ss.t < ss.life && ss.y < H + 10 && ss.x > -50) shootingStars[sslen++] = ss;
            }
            shootingStars.length = sslen;
            for (const p of bgPlanets) {
                if (p.type === 'blackhole') p.rotSp = (p.rotSp || 0) + dt;
                else if (p.type === 'debris') p.orbit += p.orbitSp * dt;
                else if (p.type === 'asteroid') { p.y += p.sp * dt * warp * 0.3; if (p.y > H + p.r) { p.y = -p.r; p.x = Math.random() * W; } }
                else if (p.type === 'planet') { p.y += p.sp * dt * warp * 0.15; if (p.y > H + p.r * 2) { p.y = -p.r * 2; p.x = Math.random() * W; } }
                else if (p.type === 'crystal') { p.y += p.sp * dt * warp * 0.2; if (p.y > H + p.r * 2) { p.y = -p.r * 2; p.x = Math.random() * W; } }
            }
            if (Math.random() < 0.004) {
                bgComets.push({ x: Math.random() * W, y: 0, vx: -30 - Math.random() * 70, vy: 160 + Math.random() * 140, life: 600, t: 0, size: 2 + Math.random() * 2 });
            }
            let cmlen = 0;
            for (let i = 0; i < bgComets.length; i++) {
                const cm = bgComets[i];
                cm.prevX = cm.x; cm.prevY = cm.y;
                cm.x += cm.vx * dt; cm.y += cm.vy * dt; cm.t += dt * 1000;
                if (cm.t < cm.life && cm.y <= H) bgComets[cmlen++] = cm;
            }
            bgComets.length = cmlen;
            if (G.bgTheme === 'storm' && Math.random() < 0.005) { G.lightningT = 150; G.lightningX = Math.random() * W; }
            if (G.lightningT > 0) G.lightningT -= dt * 1000;
            if (Math.random() < 0.002) SFX.envAmbience(G.bgTheme);
        }

        function drawStars(cv) {
            const warp = G.warpT > 0 ? 10 : 1;
            for (const s of STARS) {
                let brightness = s.br * (0.6 + 0.4 * Math.sin(tick * 0.02 + s.x));
                if (s.layer === 3) brightness *= 0.5 + 0.5 * Math.sin(s.twinkle);
                const colBase = s.col || '#ffffff';
                const r = parseInt(colBase.slice(1, 3), 16), g = parseInt(colBase.slice(3, 5), 16), b = parseInt(colBase.slice(5, 7), 16);
                cv.fillStyle = 'rgba(' + r + ',' + g + ',' + b + ',' + brightness + ')';
                const stretch = warp > 1 && s.layer >= 2 ? s.sz + 4 : s.sz;
                cv.fillRect(s.x, s.y, s.sz, stretch);
                if (s.layer >= 2 && brightness > 0.6) {
                    cv.globalAlpha = brightness * 0.3;
                    cv.fillRect(s.x - 1, s.y - 1, s.sz + 2, stretch + 2);
                    cv.globalAlpha = 1;
                }
            }
            for (let i = 0; i < shootingStars.length; i++) {
                const ss = shootingStars[i];
                const alpha = Math.max(0, 1 - ss.t / ss.life);
                cv.globalAlpha = alpha;
                cv.strokeStyle = ss.col; cv.lineWidth = 1;
                cv.beginPath();
                cv.moveTo(ss.x, ss.y);
                cv.lineTo(ss.prevX != null ? ss.prevX : ss.x - ss.vx * 0.05, ss.prevY != null ? ss.prevY : ss.y - ss.vy * 0.05);
                cv.stroke();
            }
            cv.globalAlpha = 1;

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
            drawBG(cv);
        }
        function drawBG(cv) {
            for (const p of bgPlanets) {
                if (p.type === 'blackhole') {
                    const rr = p.r + Math.sin(tick * 0.02) * 3;
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
                    cv.save(); cv.globalAlpha = 0.3; cv.translate(p.x, p.y); if (p.rot) cv.rotate(p.rot);
                    cv.fillStyle = p.col;
                    cv.fillRect(Math.floor(-p.r / 2), Math.floor(-p.r / 2), p.r, p.r);
                    cv.restore();
                } else if (p.type === 'planet') {
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
            for (let i = 0; i < bgComets.length; i++) {
                const cm = bgComets[i];
                const alpha = Math.max(0, 1 - cm.t / cm.life);
                cv.globalAlpha = alpha * 0.7;
                cv.strokeStyle = '#aaccff'; cv.lineWidth = cm.size; cv.beginPath();
                cv.moveTo(cm.x, cm.y);
                cv.lineTo(cm.prevX != null ? cm.prevX : cm.x - cm.vx * 0.05, cm.prevY != null ? cm.prevY : cm.y - cm.vy * 0.05);
                cv.stroke();
                cv.fillStyle = '#ddeeff';
                cv.fillRect(cm.x - 1, cm.y - 1, 2, 2);
            }
            if (G.lightningT > 0) {
                const la = G.lightningT / 150;
                cv.globalAlpha = la * 0.3;
                cv.fillStyle = '#ffffff';
                cv.fillRect(0, 0, W, H);
                cv.globalAlpha = la * 0.8;
                cv.strokeStyle = '#ffffff';
                cv.lineWidth = 2;
                cv.shadowBlur = 12;
                cv.shadowColor = '#88ccff';
                cv.beginPath();
                let lx = G.lightningX, ly = 0;
                cv.moveTo(lx, ly);
                while (ly < H) {
                    lx += (Math.random() - 0.5) * 40;
                    ly += 15 + Math.random() * 25;
                    cv.lineTo(lx, ly);
                }
                cv.stroke();
                cv.shadowBlur = 0;
            }
            cv.globalAlpha = 1;
        }
        function drawNebula(cv) {
            if (!nebulaCv) return;
            const pulse = 0.25 + 0.1 * Math.sin(G.fTmr * 0.3);
            cv.globalAlpha = pulse * (G.chal ? 1.3 : 1);
            const y0 = -((G.fTmr * 15) % H);
            cv.drawImage(nebulaCv, 0, y0); cv.drawImage(nebulaCv, 0, y0 + H);
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

        function isChal(s) { return settings.mode !== 'endless' && s >= 3 && (s - 3) % 4 === 0; }

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
                const enemy = { type, r, col, x: W / 2 + side * (120 + Math.random() * 80), y: -30 - (idx % 8) * 20,
                    fx, fy, hp, maxHp: hp, st: 'ENTER', eTmr: 500 + idx * 80 + r * 100,
                    fr: 0, frT: 0, dTmr: diveDelay / diffMod('diveRate'), dPath: null,
                    sTmr: (type === 'spinner' || type === 'bomber' || type === 'lasher') ? 800 + Math.random() * 1200 : 0,
                    shootPh: 0, hasCap: false, hitF: 0, elite: type === 'hunter',
                    // NEW: Boss phase system
                    bossPhase: (type === 'boss' || type === 'miniboss') ? 1 : 0,
                    bossPhaseTransition: 0, bossPhaseHP: [0.6, 0.3, 0],
                    // NEW: Sprite animation system
                    animFrame: 0, animTimer: 0, animSpeed: 120, animFrames: 3 };
                G.enemies.push(enemy);
                idx++;
            }

            if (isMini) {
                const mbHP = 4 + Math.floor(G.stage / 5);
                pushEnemy('miniboss', 0, 4, W / 2, FTOP, mbHP);
            }
            if (settings.mode === 'boss_rush') {
                const bossHP = 3 + Math.floor(G.stage * 0.8);
                pushEnemy('boss', 0, 4, W / 2, FTOP, bossHP);
                G.chalTot = G.enemies.length;
                G.dTmr = 500 / diffMod('diveRate');
                G.fX = 0; mkNebula(); initBG();
                return;
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

                const bossHP = G.stage >= 5 ? 2 + Math.floor((G.stage - 5) / 4) : (settings.mode === 'endless' ? 2 + Math.floor(G.stage / 3) : 2);
                const enemyHP = type === 'boss' ? bossHP : 1;
                pushEnemy(type, r, col, fx, fy, enemyHP);
            }
            if (settings.mode === 'endless') {
                const scale = 1 + (G.stage - 1) * 0.1;
                for (const e of G.enemies) { e.hp = Math.ceil(e.hp * scale); e.maxHp = e.hp; }
            }
            if (!isMini && !G.chal) {
                if (G.stage >= 4) {
                    const stalkerCount = Math.min(3, Math.floor((G.stage - 3) / 2));
                    for (let si = 0; si < stalkerCount; si++) {
                        const sfx = W / 2 + (si - stalkerCount / 2 + 0.5) * ESP_X;
                        pushEnemy('stalker', 1, 8, sfx, FTOP + ESP_Y, 1);
                    }
                }
                if (G.stage >= 6) {
                    const sniperCount = Math.min(2, Math.floor((G.stage - 5) / 3));
                    for (let si = 0; si < sniperCount; si++) {
                        const sfx = W / 2 + (si % 2 === 0 ? -1 : 1) * (ESP_X * 2 + si * ESP_X * 0.5);
                        pushEnemy('sniper', 0, 4, sfx, FTOP, 1);
                    }
                }
                const eliteChance = 0.22 + Math.min(G.stage, 12) * 0.035;
                if (G.stage >= 2 && Math.random() < eliteChance) {
                    const hunterHP = 2 + Math.floor(G.stage / 3);
                    pushEnemy('hunter', 0, 5, W / 2 + (Math.random() - 0.5) * 100, FTOP + ESP_Y * 0.5, hunterHP);
                }
                if (G.stage >= 3 && Math.random() < 0.38) {
                    pushEnemy('spinner', 2, 3, W / 2 + (Math.random() - 0.5) * 130, FTOP + ESP_Y * 2, 2);
                }
                if (G.stage >= 4 && Math.random() < 0.32) {
                    pushEnemy('bomber', 1, 6, W / 2 + (Math.random() - 0.5) * 110, FTOP + ESP_Y, 2);
                }
                if (G.stage >= 5 && Math.random() < 0.28) {
                    pushEnemy('lasher', 0, 2, W / 2 + (Math.random() - 0.5) * 70, FTOP, 1);
                }
                // NEW: Additional enemy types at higher stages
                if (G.stage >= 4 && Math.random() < 0.2) {
                    pushEnemy('shield_bee', 1, 4, W / 2 + (Math.random() - 0.5) * 100, FTOP + ESP_Y, 2);
                }
                if (G.stage >= 6 && Math.random() < 0.18) {
                    pushEnemy('kamikaze', 0, 2, W / 2 + (Math.random() - 0.5) * 80, FTOP, 1);
                }
                if (G.stage >= 7 && Math.random() < 0.22) {
                    pushEnemy('weaver', 1, 5, W / 2 + (Math.random() - 0.5) * 120, FTOP + ESP_Y, 1);
                }
                if (G.stage >= 8 && Math.random() < 0.16) {
                    pushEnemy('splitter', 2, 3, W / 2 + (Math.random() - 0.5) * 100, FTOP + ESP_Y * 2, 2);
                }
                if (G.stage >= 9 && Math.random() < 0.14) {
                    pushEnemy('carrier', 0, 4, W / 2 + (Math.random() - 0.5) * 90, FTOP, 3);
                }
                if (G.stage >= 10 && Math.random() < 0.12) {
                    pushEnemy('teleporter', 1, 6, W / 2 + (Math.random() - 0.5) * 110, FTOP + ESP_Y, 2);
                }
            }
            G.chalTot = G.enemies.length;
            G.dTmr = (2000 - Math.min(G.stage * 100, 1200)) / diffMod('diveRate');
            G.fX = 0;
            mkNebula(); initBG();
            if (isMini) SFX.miniBossWarning();
        }

        function advanceToNextStage(fromSkip) {
            if (G.stageClearLock > 0 || G.st === 'STAGE_INTRO' || G.st === 'GAME_OVER') return;
            G.stageClearLock = 600;
            G.stageEmptyT = 0;
            // NEW: Random transition type (0=warp, 1=swipe, 2=portal, 3=glitch)
            G.transitionType = Math.floor(Math.random() * 4);
            if (G.transitionType === 0) { G.warpT = 1500; G.warpFlash = 50; }
            else if (G.transitionType === 1) { G.swipeT = 1200; G.swipeDir = Math.random() > 0.5 ? 1 : -1; }
            else if (G.transitionType === 2) { G.portalT = 1400; G.portalR = 0; }
            else { G.glitchT = 1000; G.glitchStrips = []; for (let _gi = 0; _gi < 12; _gi++) G.glitchStrips.push({ y: _gi * (H / 12), offset: 0, targetOffset: (Math.random() - 0.5) * 60 }); }
            G.stage++;
            if (G.stage >= 10) unlockAchievement('survivor');
            if (G.stage >= 20) unlockAchievement('legend');
            if (G.score >= 1000000) unlockAchievement('millionaire');
            const stageTime = (performance.now ? performance.now() : Date.now()) - G.stageStartTime;
            if (stageTime < 30000 && G.stage > 2) unlockAchievement('speed_demon');
            SFX.warpJump();
            if (!G.chal && !fromSkip) {
                MusicEngine.play('victory');
                setTimeout(() => { if (!state.disposed && MusicEngine.playing === 'victory') MusicEngine.play('gameplay'); }, 3500);
            }
            startStage();
        }

        function startStage() {
            G.enemies = [];
            G.chal = isChal(G.stage);
            G.stageWipeT = 400;
            G.st = 'STAGE_INTRO';
            G.introTmr = 1200;
            G.stageStartTime = performance.now ? performance.now() : Date.now();
            G.bul = []; G.ebul = []; G.exp = []; G.part = []; G.pendingBooms = []; G.levelSkipTimer = 0;
            G.beam = null; G.powerups = []; G.activePU = null; G.puTimer = 0; G.shieldHits = 0;
            G.scorePopups = []; G.warpT = 0; G.warpFlash = 0; G.perfectT = 0;
            G.combo = 0; G.comboTimer = 0; G.comboMult = 1; G.comboBanner = null;
            G.trails = []; G.timeScale = 1; G.timeSlowTimer = 0; G.freezeT = 0; G.damageVignetteT = 0;
            G.bossWarningT = 0; G.bossWarningShown = false;
            G.weaponLv = Math.max(1, G.weaponLv); G.puUpgrade = null; G.upgradeBanner = null; G.killCount = 0; G.slowMoT = 0;
            // NEW: Reset new state fields
            G.swipeT = 0; G.portalT = 0; G.glitchT = 0; G.glitchStrips = [];
            G._closeCallCooldown = 0; G._synergyChecked = null; G.shieldReflect = false; G.laserSlow = false; G.droneRicochet = false;
            G.scoreMult = 1; G.glassCannon = false; G.bulletStorm = false; G.powerSurge = false; G.darkness = false; G.turbo = false;
            G.pacifistStage = true; G.overcharge = 0; G.overchargeTimer = 0;
            G.p.x = W / 2; G.p.alive = true; G.p.inv = 2000; G.p.cap = null; G.p.dual = false; G.p.reviveTimer = 0;
            G.stageEmptyT = 0;
            setPUClass(null);
            G.chal ? SFX.challenge() : SFX.stageClear();
            MusicEngine.setTempo(1 + G.stage * 0.05);
            MusicEngine.play(G.chal ? 'challenge' : 'gameplay');
            mkFormation();
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
                    let _fc = 0; for (const _fb of G.bul) if (!_fb.vx && !_fb.laser) _fc++;
                    if (_fc >= max) return;
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

        function boom(x, y, isBoss, enemyType) {
            const dur = isBoss ? 900 : 450;
            const pCount = isBoss ? 50 : 20;
            const sparkCount = isBoss ? 24 : 10;
            const debrisCount = isBoss ? 14 : 6;
            const smokeCount = isBoss ? 14 : 7;
            const flashCount = isBoss ? 8 : 4;
            G.exp.push({ x, y, t: 0, dur, seed: Math.random(), isBoss });
            if (isBoss) { G.exp.push({ x, y, t: 0, dur: 700, seed: Math.random(), isBoss: false, shockwave: true }); G.exp.push({ x, y, t: 0, dur: 180, seed: Math.random(), isBoss: false, flash: true }); }
            else { G.exp.push({ x, y, t: 0, dur: 100, seed: Math.random(), isBoss: false, flash: true }); G.exp.push({ x, y, t: 0, dur: 300, seed: Math.random(), isBoss: false, shockwave: true }); }
            // NEW: Per-enemy-type death animation colors
            const typeCols = {
                bee: ['#ffcc00', '#ffaa00', '#ffee88', '#fff'],
                butterfly: ['#ff3366', '#ff6688', '#ff88aa', '#fff'],
                stalker: ['#6622aa', '#8844cc', '#aa66ee', '#fff'],
                sniper: ['#ffcc00', '#ffaa00', '#ffff44', '#fff'],
                hunter: ['#ff6600', '#ff8844', '#ffaa00', '#fff'],
                spinner: ['#00cccc', '#44ffff', '#88ffff', '#fff'],
                bomber: ['#aa44cc', '#cc66ff', '#ff44aa', '#fff'],
                lasher: ['#44ff88', '#00cc66', '#aaffcc', '#fff'],
                weaver: ['#ff8844', '#ffaa66', '#ffcc88', '#fff'],
                splitter: ['#88ff44', '#aaff66', '#ccff88', '#fff'],
                shield_bee: ['#ffcc00', '#ffdd44', '#ffee88', '#fff'],
                kamikaze: ['#ff2222', '#ff4444', '#ff6666', '#fff'],
                carrier: ['#cc88ff', '#ddaaff', '#eeccff', '#fff'],
                teleporter: ['#44ffff', '#66ffff', '#88ffff', '#fff']
            };
            const fireCols = (enemyType && typeCols[enemyType]) ? typeCols[enemyType] : ['#ffcc00', '#ff4444', '#ff8800', '#fff', '#ffee88', '#ff6622', '#ffaa00'];
            for (let i = 0; i < pCount; i++) {
                const a = (i / pCount) * Math.PI * 2 + Math.random() * 0.8, sp = 60 + (i * 23 % 160) * (isBoss ? 2 : 1.2);
                const cols = fireCols[i % fireCols.length];
                const sz = i % 4 === 0 ? 4 : i % 3 === 0 ? 3 : 2;
                G.part.push({ x, y, vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, life: (280 + (i * 41 % 280)) * (isBoss ? 1.6 : 1.1), t: 0, col: cols, size: sz });
            }
            // NEW: Type-specific death effects
            if (enemyType === 'bee') {
                for (let i = 0; i < 6; i++) { const a = Math.random() * Math.PI * 2; G.part.push({ x, y, vx: Math.cos(a) * 30, vy: Math.sin(a) * 30 - 20, life: 400, t: 0, col: '#ffcc00', size: 2, spark: true }); }
            } else if (enemyType === 'butterfly') {
                for (let i = 0; i < 8; i++) { const a = (i / 8) * Math.PI * 2; G.part.push({ x, y, vx: Math.cos(a) * 50, vy: Math.sin(a) * 50, life: 300, t: 0, col: '#ff3366', size: 1, spark: true }); }
            } else if (enemyType === 'stalker') {
                G.exp.push({ x, y, t: 0, dur: 400, seed: Math.random(), isBoss: false, implosion: true, col: '#8844cc' });
            } else if (enemyType === 'hunter') {
                for (let i = 0; i < 10; i++) { const a = Math.random() * Math.PI * 2; G.part.push({ x, y, vx: Math.cos(a) * 70, vy: Math.sin(a) * 70, life: 350, t: 0, col: '#ff6600', size: 3, debris: true, rot: Math.random() * 6.28 }); }
            } else if (enemyType === 'spinner') {
                G.plasmaRings.push({ x, y, r: 0, maxR: 40, t: 0, dur: 300, col: '#44ffff' });
            } else if (enemyType === 'bomber') {
                for (let i = 0; i < 3; i++) { G.pendingBooms.push({ x: x + (Math.random() - 0.5) * 30, y: y + (Math.random() - 0.5) * 20, isBoss: false, delay: i * 80 }); }
            } else if (enemyType === 'lasher') {
                G.flashT = Math.max(G.flashT, 50);
                for (let i = 0; i < 6; i++) { const a = (i / 6) * Math.PI * 2; G.part.push({ x, y, vx: Math.cos(a) * 60, vy: Math.sin(a) * 60, life: 200, t: 0, col: '#44ff88', size: 2, spark: true }); }
            } else if (enemyType === 'kamikaze') {
                G.shkT = Math.max(G.shkT, 300); G.shkM = Math.max(G.shkM, 5);
                G.exp.push({ x, y, t: 0, dur: 300, seed: Math.random(), isBoss: false, flash: true });
            } else if (enemyType === 'carrier') {
                for (let i = 0; i < 3; i++) { G.pendingBooms.push({ x: x + (Math.random() - 0.5) * 40, y: y + (Math.random() - 0.5) * 30, isBoss: false, delay: i * 150 }); }
            } else if (enemyType === 'teleporter') {
                for (let i = 0; i < 12; i++) { const a = (i / 12) * Math.PI * 2; G.part.push({ x, y, vx: Math.cos(a) * 80, vy: Math.sin(a) * 80, life: 250, t: 0, col: '#44ffff', size: 1, spark: true }); }
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
                    G.pendingBooms.push({ x: x + (Math.random() - 0.5) * 50, y: y + (Math.random() - 0.5) * 40, isBoss: false, delay: i * 100 });
                }
                G.shkT = Math.max(G.shkT, 800); G.shkM = Math.max(G.shkM, 7);
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
            const multiplied = pts * G.comboMult * (G.scoreMult || 1);
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
            if (G.combo >= 15) unlockAchievement('combo_king');
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
            let chance = e.type === 'miniboss' ? 1 : (e.type === 'boss' ? 0.35 : (e.type === 'bee' && !diffMod('puFromBee') ? 0 : 0.12));
            // NEW: Power Surge modifier triples drop chance
            if (G.powerSurge) chance *= 3;
            if (Math.random() < chance) {
                // NEW: Weighted rarity-based powerup selection
                let type;
                const roll = Math.random() * 100;
                if (roll < PU_RARITY_WEIGHT.legendary) {
                    type = PU_RARITY.legendary[Math.floor(Math.random() * PU_RARITY.legendary.length)];
                } else if (roll < PU_RARITY_WEIGHT.legendary + PU_RARITY_WEIGHT.rare) {
                    type = PU_RARITY.rare[Math.floor(Math.random() * PU_RARITY.rare.length)];
                } else if (roll < PU_RARITY_WEIGHT.legendary + PU_RARITY_WEIGHT.rare + PU_RARITY_WEIGHT.uncommon) {
                    type = PU_RARITY.uncommon[Math.floor(Math.random() * PU_RARITY.uncommon.length)];
                } else {
                    type = PU_RARITY.common[Math.floor(Math.random() * PU_RARITY.common.length)];
                }
                if (type === 'levelskip' && (e.type !== 'boss' && e.type !== 'miniboss')) type = 'rapid';
                if (type === 'levelskip' && Math.random() > 0.05) type = PU_RARITY.legendary[Math.floor(Math.random() * (PU_RARITY.legendary.length - 1))];
                G.powerups.push({ x: e.x, y: e.y, type, t: 0 });
            }
        }

        function collectPU(pu) {
            if (pu.type === 'bomb' || pu.type === 'multibomb') {
                SFX.bomb(pu.x);
                const bonus = pu.type === 'multibomb' ? 500 : 0;
                for (const e of G.enemies) { if (e.st !== 'DEAD') { addScore(PTS[e.type][0] + bonus, e.x, e.y, PU_COL[pu.type]); boom(e.x, e.y, e.type === 'boss', e.type); e.st = 'DEAD'; } }
                G.flashT = 100; G.activePU = null; G.puTimer = 0; setPUClass(null); return;
            }
            if (pu.type === 'supernova') {
                SFX.supernova(pu.x);
                for (const e of G.enemies) { if (e.st !== 'DEAD') { addScore(PTS[e.type][0] + 1000, e.x, e.y, '#fff'); boom(e.x, e.y, e.type === 'boss', e.type); e.st = 'DEAD'; } }
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
            if (pu.type === 'levelskip') {
                SFX.supernova(pu.x);
                let delay = 0;
                for (const e of G.enemies) {
                    if (e.st !== 'DEAD') {
                        const pts = PTS[e.type] ? PTS[e.type][0] : 200;
                        addScore(pts + 200, e.x, e.y, '#ff88ff');
                        G.pendingBooms.push({ x: e.x, y: e.y, isBoss: e.type === 'boss' || e.type === 'miniboss', delay });
                        e.st = 'DEAD';
                        delay += 120;
                    }
                }
                for (let i = G.ebul.length - 1; i >= 0; i--) bulletImpact(G.ebul[i].x, G.ebul[i].y, '#ff88ff');
                G.ebul = [];
                G.levelSkipTimer = delay + 800;
                G.flashT = 200; G.shkT = 300; G.shkM = 6;
                G.activePU = null; G.puTimer = 0; setPUClass(null);
                return;
            }
            const isUpgradeable = PU_UPGRADE[pu.type];
            const isSameType = G.activePU && G.activePU.type === pu.type;
            if (pu.type === 'drone') {
                const count = (isUpgradeable && isSameType) ? 2 : 1;
                G.drones = [];
                for (let di = 0; di < count; di++) G.drones.push({ x: G.p.x + (di === 0 ? -20 : 20), y: G.p.y - 20, targetX: G.p.x + (di === 0 ? -25 : 25), targetY: G.p.y - 30, fireT: 0 });
                G.droneTimer = PU_DUR.drone; G.activePU = { type: count > 1 ? 'dual_drone' : 'drone', timer: PU_DUR.drone }; G.puTimer = PU_DUR.drone; setPUClass(count > 1 ? 'dual_drone' : 'drone');
                SFX.puCollect(pu.x); return;
            }
            if (pu.type === 'blackhole_bomb') {
                G.blackhole = { x: G.p.x, y: G.p.y - 60, targetX: G.p.x, targetY: G.p.y - 120, t: 0 };
                SFX.bomb(pu.x); G.activePU = null; G.puTimer = 0; setPUClass(null);
                for (let i = 0; i < 8; i++) { const a = (i / 8) * Math.PI * 2; G.part.push({ x: G.p.x, y: G.p.y - 60, vx: Math.cos(a) * 50, vy: Math.sin(a) * 50, life: 300, t: 0, col: '#8844ff', size: 2, spark: true }); }
                return;
            }
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
            else if (pu.type === 'ricochet') {
                G.activePU = { type: G.puUpgrade === 'mega_ricochet' ? 'mega_ricochet' : 'ricochet', timer: PU_DUR.ricochet }; G.puTimer = PU_DUR.ricochet; setPUClass(G.activePU.type);
            }
            else { G.activePU = { type: pu.type, timer: PU_DUR[pu.type] || 0 }; G.puTimer = PU_DUR[pu.type] || 0; setPUClass(pu.type); }
            G.collectedPU.add(pu.type); if (G.collectedPU.size >= PU_TYPES.length) unlockAchievement('power_collector');
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
G.p.alive = false; boom(G.p.x, G.p.y, false, 'player'); SFX.pExplode(G.p.x); G.shkT = 300; G.shkM = 4; G.lives--;
            wrapEl.classList.add('galaxa-desaturate'); setTimeout(() => { if (!state.disposed) wrapEl.classList.remove('galaxa-desaturate'); }, 800);
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
            const baseSpd = getShipSpeed();
            const spd = G.activePU && (G.activePU.type === 'speed' || G.activePU.type === 'hyper_speed') ? baseSpd * (G.activePU.type === 'hyper_speed' ? 2.2 : 1.8) : baseSpd;
            if (inp.l) G.p.x -= spd * dt; if (inp.r) G.p.x += spd * dt;
            G.p.x = Math.max(10, Math.min(W - 10, G.p.x));
            if (G.p.inv > 0) G.p.inv -= dt * 1000;
            if (inp.f && G.st === 'PLAYING') fire(now);
            if (G.beam && G.beam.active && G.p.x > G.beam.x - 20 && G.p.x < G.beam.x + 20 && G.p.y > G.beam.y) {
                if (G.p.alive) { killP(); G.beam.cap = true; G.beam.capT = 0; SFX.beam(); }
            }
            if (G.droneTimer > 0) {
                G.droneTimer -= dt * 1000;
                if (G.droneTimer <= 0) { G.drones = []; }
                else {
                    for (const dr of G.drones) {
                        dr.x += (dr.targetX - dr.x) * dt * 5;
                        dr.y += (dr.targetY - dr.y) * dt * 5;
                        dr.fireT -= dt * 1000;
                        if (dr.fireT <= 0) {
                            const nearE = G.enemies.filter(e => e.st !== 'DEAD').sort((a, b) => Math.hypot(a.x - dr.x, a.y - dr.y) - Math.hypot(b.x - dr.x, b.y - dr.y))[0];
                            if (nearE && Math.hypot(nearE.x - dr.x, nearE.y - dr.y) < 250) {
                                const dx = nearE.x - dr.x, dy = nearE.y - dr.y, dist = Math.hypot(dx, dy);
                                G.bul.push({ x: dr.x, y: dr.y - 4, w: 2, h: 4, vx: (dx / dist) * PB_SPEED * 0.5, vy: (dy / dist) * PB_SPEED * 0.5 });
                            }
                            dr.fireT = 300;
                        }
                    }
                }
            }
            if (G.blackhole) {
                G.blackhole.t += dt * 1000;
                G.blackhole.x += (G.blackhole.targetX - G.blackhole.x) * dt * 2;
                G.blackhole.y += (G.blackhole.targetY - G.blackhole.y) * dt * 2;
                for (const e of G.enemies) {
                    if (e.st === 'DEAD') continue;
                    const dx = G.blackhole.x - e.x, dy = G.blackhole.y - e.y, dist = Math.hypot(dx, dy);
                    if (dist > 5 && dist < 100) { e.x += (dx / dist) * 80 * dt; e.y += (dy / dist) * 80 * dt; }
                }
                for (const b of G.ebul) {
                    const dx = G.blackhole.x - b.x, dy = G.blackhole.y - b.y, dist = Math.hypot(dx, dy);
                    if (dist > 5 && dist < 80) { b.x += (dx / dist) * 100 * dt; b.y += (dy / dist) * 100 * dt; }
                }
                if (G.blackhole.t > 3000) {
                    SFX.bigExplode(G.blackhole.x);
                    for (const e of G.enemies) {
                        if (e.st === 'DEAD') continue;
                        const dist = Math.hypot(e.x - G.blackhole.x, e.y - G.blackhole.y);
                        if (dist < 120) { addScore(PTS[e.type] ? PTS[e.type][0] : 100, e.x, e.y, '#8844ff'); boom(e.x, e.y, e.type === 'boss' || e.type === 'miniboss', e.type); e.st = 'DEAD'; }
                    }
                    G.flashT = 150; G.shkT = 400; G.shkM = 6;
                    G.blackhole = null;
                }
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
            // NEW: Overcharge timer decay
            if (G.overchargeTimer > 0) {
                G.overchargeTimer -= dt * 1000;
                if (G.overchargeTimer <= 0) { G.overcharge = 0; G.overchargeTimer = 0; }
            }
            // NEW: Powerup synergy detection
            if (G.activePU && G.puUpgrade) {
                const baseType = Object.keys(PU_UPGRADE).find(k => PU_UPGRADE[k] === G.activePU.type);
                if (baseType) {
                    for (const otherType of Object.keys(PU_SYNERGIES)) {
                        const [t1, t2] = otherType.split('+');
                        if ((baseType === t1 || baseType === t2) && G._synergyChecked !== otherType) {
                            // Check if we have the other powerup's effect active
                            const otherActive = (t1 === 'shield' && G.shieldHits > 0) ||
                                (t2 === 'shield' && G.shieldHits > 0) ||
                                (G.activePU && (G.activePU.type === t1 || G.activePU.type === t2 || G.activePU.type === PU_UPGRADE[t1] || G.activePU.type === PU_UPGRADE[t2]));
                            if (otherActive && baseType !== (t1 === baseType ? t2 : t1)) {
                                G._synergyChecked = otherType;
                                const syn = PU_SYNERGIES[otherType];
                                G.upgradeBanner = { text: 'SYNERGY: ' + syn.name, type: 'synergy', t: 0, dur: 2000 };
                                G.scorePopups.push({ x: G.p.x, y: G.p.y - 30, text: syn.name + '!', t: 0, dur: 1500, col: syn.col, big: true });
                                SFX.puUpgrade(G.p.x);
                                // Apply synergy effects
                                if (otherType === 'shield+magnet') G.shieldReflect = true;
                                if (otherType === 'laser+timeslow') G.laserSlow = true;
                                if (otherType === 'drone+ricochet') G.droneRicochet = true;
                            }
                        }
                    }
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
                    if (Math.abs(inp.r ? 1 : 0 - (inp.l ? 1 : 0)) > 0 && G.trails.length < 75) {
                        const wakeDir = inp.l ? 1 : -1;
                        G.trails.push({ x: G.p.x + wakeDir * 10, y: G.p.y + 8, vx: wakeDir * (40 + Math.random() * 30), vy: 10 + Math.random() * 10, life: 120, t: 0, col: 'rgba(255,200,100,0.3)', size: 1 });
                    }
                }
            }
            let plw = 0;
            for (let i = 0; i < G.powerups.length; i++) {
                const _pu = G.powerups[i];
                _pu.y += 60 * dt; _pu.t += dt * 1000;
                if (_pu.y > H + 20) continue;
                if (G.p.alive && hit({ x: _pu.x - 5, y: _pu.y - 5, w: 10, h: 10 }, { x: G.p.x - 6, y: G.p.y - 6, w: 12, h: 12 })) {
                    // NEW: Overcharge — reject powerup by pressing down
                    if (inp.d && _pu.type !== 'bomb' && _pu.type !== 'multibomb' && _pu.type !== 'supernova' && _pu.type !== 'levelskip') {
                        G.overcharge++;
                        G.overchargeTimer = 15000; // 15s to collect another
                        if (G.overcharge >= 5) unlockAchievement('overcharge');
                        // Visual feedback
                        for (let _oi = 0; _oi < 8; _oi++) {
                            const _oa = (_oi / 8) * Math.PI * 2;
                            G.part.push({ x: _pu.x, y: _pu.y, vx: Math.cos(_oa) * 40, vy: Math.sin(_oa) * 40 - 20, life: 300, t: 0, col: '#ffaa00', size: 2, spark: true });
                        }
                        G.scorePopups.push({ x: _pu.x, y: _pu.y - 10, text: 'OVERCHARGE ' + G.overcharge + '/3', t: 0, dur: 1200, col: '#ffaa00', big: false });
                        SFX.puCollect(_pu.x);
                        continue;
                    }
                    collectPU(_pu); continue;
                }
                G.powerups[plw++] = _pu;
            }
            G.powerups.length = plw;
            }
        }

        function updateBul(dt) {
            const dtMs = dt * 1000;
            const hasRicochet = G.activePU && (G.activePU.type === 'ricochet' || G.activePU.type === 'mega_ricochet');
            const maxBounces = G.activePU && G.activePU.type === 'mega_ricochet' ? 4 : 2;
            let bw = 0;
            for (let i = 0; i < G.bul.length; i++) {
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
                if (b.y < -10 || b.y > H + 10) continue;
                if (b.x < 0 || b.x > W) {
                    if (hasRicochet && (b.bounces || 0) < maxBounces) {
                        b.vx = -(b.vx || 0); b.x = Math.max(1, Math.min(W - 1, b.x)); b.bounces = (b.bounces || 0) + 1;
                        for (let _bi = 0; _bi < 3; _bi++) G.part.push({ x: b.x, y: b.y, vx: (Math.random()-0.5)*40, vy: (Math.random()-0.5)*40, life: 120, t: 0, col: '#ffaa44', size: 1, spark: true });
                    } else continue;
                }
                let removed = false;
                for (let j = G.enemies.length - 1; j >= 0; j--) {
                    const e = G.enemies[j]; if (e.st === 'DEAD') continue;
                    const ew = (e.type === 'boss' || e.type === 'miniboss') ? 24 : (e.type === 'hunter' ? 18 : e.type === 'sniper' ? 14 : 16);
                    if (hit(b, { x: e.x - ew / 2, y: e.y - 8, w: ew, h: 16 })) {
                        e.hp--;
                        if (e.hp <= 0) {
                            const pts = PTS[e.type] ? PTS[e.type][e.st === 'DIVING' ? 1 : 0] : 200;
                            registerKill();
                            addScore(pts, e.x, e.y, e.type === 'bee' ? '#ffcc00' : e.type === 'butterfly' ? '#ff3366' : '#44cc44');
                            boom(e.x, e.y, e.type === 'boss' || e.type === 'miniboss', e.type); SFX.eExplode(e.x); dropPU(e);
                            if (e.type === 'boss' || e.type === 'miniboss') { G.timeScale = 0.3; G.slowMoT = 1500; }
                            if (e.hasCap) G.p.cap = { x: e.x, y: e.y };
                            if (G.chal) G.chalHits++; e.st = 'DEAD';
                            // NEW: Splitter splits into 2 mini enemies on death
                            if (e.type === 'splitter') {
                                for (let _si = 0; _si < 2; _si++) {
                                    const sx = e.x + (_si === 0 ? -15 : 15);
                                    const sy = e.y - 10;
                                    G.enemies.push({ type: 'bee', r: 0, col: 0, x: sx, y: sy, fx: sx, fy: sy, hp: 1, maxHp: 1, st: 'DIVING', eTmr: 0, fr: 0, frT: 0, dTmr: 2000, dPath: { ph: 0, amp: 20, vx: (_si === 0 ? -40 : 40) }, sTmr: 500, shootPh: 0, hasCap: false, hitF: 0, elite: false, bossPhase: 0, bossPhaseTransition: 0, bossPhaseHP: [0,0,0], animFrame: 0, animTimer: 0, animSpeed: 120, animFrames: 4 });
                                }
                            }
                            // NEW: Carrier releases 3 bees on death
                            if (e.type === 'carrier') {
                                for (let _ci = 0; _ci < 3; _ci++) {
                                    const ca = (_ci / 3) * Math.PI * 2;
                                    G.enemies.push({ type: 'bee', r: 0, col: 0, x: e.x, y: e.y, fx: e.x + Math.cos(ca) * 40, fy: e.y + Math.sin(ca) * 40, hp: 1, maxHp: 1, st: 'ENTER', eTmr: 300 + _ci * 100, fr: 0, frT: 0, dTmr: 1500, dPath: null, sTmr: 800, shootPh: 0, hasCap: false, hitF: 0, elite: false, bossPhase: 0, bossPhaseTransition: 0, bossPhaseHP: [0,0,0], animFrame: 0, animTimer: 0, animSpeed: 120, animFrames: 4 });
                                }
                            }
                            G.killCount++;
                            if (G.killCount === 1) unlockAchievement('first_blood');
                            if (e.type === 'boss' || e.type === 'miniboss') { G.bossKillTotal++; if (G.bossKillTotal >= 10) unlockAchievement('boss_slayer'); try { localStorage.setItem('galaxa_boss_kills', String(G.bossKillTotal)); } catch(e2) {} }
                            const _remainingAlive = G.enemies.filter(_en => _en.st !== 'DEAD' && _en !== e).length;
                            if (_remainingAlive === 0 && e.type !== 'boss' && e.type !== 'miniboss') { G.timeScale = 0.3; G.slowMoT = 500; }
                            if (G.killCount % 10 === 0 && G.weaponLv < 4) { G.weaponLv++; SFX.weaponUp(); }
                        } else e.hitF = 100;
                        if (!b.laser && !b.pierce) { removed = true; break; }
                    }
                }
                if (!removed) G.bul[bw++] = b;
            }
            G.bul.length = bw;
            const eDt = dt * G.timeScale;
            const ebSpd = EB_SPEED * diffMod('ebSpd');
            const origELen = G.ebul.length;
            let ew = 0;
            for (let i = 0; i < origELen; i++) {
                const b = G.ebul[i];
                b.t = (b.t || 0) + dtMs;
                if (b.kind === 'mine') {
                    b.y += (b.vy || ebSpd * 0.2) * eDt;
                    b.x += (b.vx || 0) * eDt;
                    if (b.fuse !== undefined) b.fuse -= dtMs;
                    const nearP = G.p.alive && Math.hypot(G.p.x - b.x, G.p.y - b.y) < 36;
                    if (b.fuse !== undefined && b.fuse <= 0 || nearP) {
                        for (let mi = 0; mi < 6; mi++) { const ma = (mi / 6) * Math.PI * 2; G.ebul.push({ x: b.x, y: b.y, w: 2, h: 3, vx: Math.cos(ma) * ebSpd * 0.35, vy: Math.sin(ma) * ebSpd * 0.35, kind: 'spiral' }); }
                        bulletImpact(b.x, b.y, '#cc66ff');
                        continue;
                    }
                } else if (b.vx !== undefined || b.vy !== undefined) {
                    b.x += (b.vx || 0) * eDt;
                    b.y += (b.vy || 0) * eDt;
                } else {
                    b.y += ebSpd * eDt;
                }
                if (b.y > H + 14 || b.y < -14 || b.x < -14 || b.x > W + 14) continue;
                if (G.p.alive && G.p.inv <= 0 && hit(b, { x: G.p.x - 6, y: G.p.y - 6, w: 12, h: 12 })) { killP(); continue; }
                // NEW: Danger-close bonus — near miss detection
                if (G.p.alive && G.p.inv <= 0 && !G._closeCallCooldown) {
                    const _cdx = G.p.x - b.x, _cdy = G.p.y - b.y;
                    const _cdist = Math.hypot(_cdx, _cdy);
                    if (_cdist < 18 && _cdist > 8) {
                        G._closeCallCooldown = 500;
                        addScore(500, G.p.x, G.p.y - 10, '#ffaa00');
                        G.scorePopups.push({ x: G.p.x, y: G.p.y - 20, text: 'CLOSE CALL!', t: 0, dur: 1000, col: '#ffaa00', big: true });
                        SFX.closeCall(G.p.x);
                    }
                }
                G.ebul[ew++] = b;
            }
            for (let i = origELen; i < G.ebul.length; i++) G.ebul[ew++] = G.ebul[i];
            G.ebul.length = ew;
        }

        function enemyFire(e) {
            if (G.chal || G.st !== 'PLAYING') return;
            const spd = EB_SPEED * diffMod('ebSpd');
            const px = e.x, py = e.y + 8;
            // NEW: Boss phase-based attack patterns
            if ((e.type === 'boss' || e.type === 'miniboss') && e.bossPhase) {
                const ebSpd = spd;
                switch (e.bossPhase) {
                    case 1:
                        G.ebul.push({ x: px, y: py, w: 2, h: 6 });
                        if (G.stage >= 5) { G.ebul.push({ x: px - 8, y: py, w: 2, h: 6 }); G.ebul.push({ x: px + 8, y: py, w: 2, h: 6 }); }
                        break;
                    case 2:
                        // Spread shot + aimed burst
                        G.ebul.push(...ATTACK_PATTERNS.aimed_burst(e, 5, 0.55, 0, ebSpd, G.p.x, G.p.y));
                        G.ebul.push(...ATTACK_PATTERNS.random_spread(e, 4, 0.4, 0.8, ebSpd));
                        SFX.hunterShot(e.x);
                        break;
                    case 3:
                        // Bullet hell: spiral + circle + wall
                        G.ebul.push(...ATTACK_PATTERNS.spiral(e, 12, 0.35, 0, ebSpd));
                        G.ebul.push(...ATTACK_PATTERNS.circle(e, 8, 0.3, ebSpd));
                        if (Math.random() < 0.5) G.ebul.push(...ATTACK_PATTERNS.wall(e, 3, 5, 0.25, ebSpd));
                        SFX.spinnerShot(e.x);
                        G.shkT = Math.max(G.shkT, 200); G.shkM = Math.max(G.shkM, 3);
                        break;
                }
                return;
            }
            switch (e.type) {
                case 'hunter':
                    if (!G.p.alive) break;
                    SFX.hunterShot(e.x);
                    { const dx = G.p.x - px, dy = G.p.y - py, dist = Math.hypot(dx, dy) || 1, baseA = Math.atan2(dy, dx);
                      for (let i = -2; i <= 2; i++) { const a = baseA + i * 0.22; G.ebul.push({ x: px, y: py, w: 2, h: 5, vx: Math.cos(a) * spd * 0.62, vy: Math.sin(a) * spd * 0.62, kind: 'hunter' }); } }
                    break;
                case 'spinner':
                    SFX.spinnerShot(e.x);
                    e.shootPh = (e.shootPh || 0) + Math.PI / 3;
                    for (let i = 0; i < 8; i++) { const a = e.shootPh + i * Math.PI / 4; G.ebul.push({ x: px, y: py, w: 2, h: 4, vx: Math.cos(a) * spd * 0.44, vy: Math.sin(a) * spd * 0.44, kind: 'spiral' }); }
                    break;
                case 'bomber':
                    SFX.bomberDrop(e.x);
                    for (let i = -1; i <= 1; i++) G.ebul.push({ x: px + i * 10, y: py, w: 3, h: 3, vx: i * 38, vy: spd * 0.22, kind: 'mine', fuse: 2200, t: 0 });
                    break;
                case 'lasher':
                    if (!G.p.alive) break;
                    SFX.lasherShot(e.x);
                    { const dx = G.p.x - px, dy = G.p.y - py, dist = Math.hypot(dx, dy) || 1;
                      G.ebul.push({ x: px, y: py, w: 4, h: 10, vx: (dx / dist) * spd * 0.52, vy: (dy / dist) * spd * 0.52, kind: 'plasma' });
                      G.ebul.push({ x: px - 6, y: py, w: 3, h: 7, vx: (dx / dist) * spd * 0.4 + 28, vy: (dy / dist) * spd * 0.4, kind: 'plasma' });
                      G.ebul.push({ x: px + 6, y: py, w: 3, h: 7, vx: (dx / dist) * spd * 0.4 - 28, vy: (dy / dist) * spd * 0.4, kind: 'plasma' }); }
                    break;
                // NEW: Enemy type firing patterns
                case 'weaver':
                    if (!G.p.alive) break;
                    { const dx = G.p.x - px, dy = G.p.y - py, dist = Math.hypot(dx, dy) || 1;
                      G.ebul.push({ x: px, y: py, w: 2, h: 5, vx: (dx / dist) * spd * 0.45, vy: (dy / dist) * spd * 0.45, kind: 'hunter' }); }
                    break;
                case 'splitter':
                    G.ebul.push(...ATTACK_PATTERNS.random_spread(e, 5, 0.35, 0.6, spd));
                    break;
                case 'shield_bee':
                    G.ebul.push({ x: px, y: py, w: 2, h: 6 });
                    break;
                case 'kamikaze':
                    // Kamikaze doesn't shoot — it charges
                    break;
                case 'carrier':
                    G.ebul.push(...ATTACK_PATTERNS.aimed_burst(e, 3, 0.4, 0, spd, G.p.x, G.p.y));
                    break;
                case 'teleporter':
                    G.ebul.push(...ATTACK_PATTERNS.circle(e, 6, 0.3, spd));
                    break;
                case 'stalker':
                    G.ebul.push({ x: px, y: py, w: 2, h: 6 });
                    G.ebul.push({ x: px - 6, y: py - 2, w: 2, h: 6 });
                    G.ebul.push({ x: px + 6, y: py - 2, w: 2, h: 6 });
                    break;
                case 'sniper':
                    if (G.p.alive) {
                        const dx = G.p.x - px, dy = G.p.y - py, dist = Math.hypot(dx, dy);
                        if (dist > 1) { G.ebul.push({ x: px, y: py, w: 2, h: 6, vx: (dx / dist) * spd * 0.6, vy: (dy / dist) * spd * 0.6, kind: 'sniper' }); SFX.sniperShot(e.x); }
                    }
                    break;
                default:
                    if (e.type === 'bee') break;
                    G.ebul.push({ x: px, y: py, w: 2, h: 6 });
                    if (G.stage >= 5 && e.type === 'boss') { G.ebul.push({ x: px - 8, y: py, w: 2, h: 6 }); G.ebul.push({ x: px + 8, y: py, w: 2, h: 6 }); }
                    if (G.stage >= 8 && e.type === 'boss') { for (let k = 0; k < 3; k++) setTimeout(() => { if (!state.disposed && e.st === 'DIVING') G.ebul.push({ x: e.x, y: e.y + 8, w: 2, h: 6 }); }, k * 150); }
                    if (e.type === 'miniboss') { G.ebul.push({ x: px - 10, y: py, w: 2, h: 6 }); G.ebul.push({ x: px + 10, y: py, w: 2, h: 6 }); for (let k = 0; k < 2; k++) setTimeout(() => { if (!state.disposed && e.st === 'DIVING') { G.ebul.push({ x: e.x - 6, y: e.y + 8, w: 2, h: 6 }); G.ebul.push({ x: e.x + 6, y: e.y + 8, w: 2, h: 6 }); } }, k * 180); }
            }
        }

        function diveRateMult(e) {
            if (e.type === 'hunter') return 5;
            if (e.type === 'stalker') return 3;
            if (e.type === 'kamikaze') return 4;
            return 1;
        }

        function startDive(e) {
            if (e.st !== 'FORM') return;
            e.st = 'DIVING';
            e.dPath = { ph: 0, amp: e.type === 'hunter' ? 18 + Math.random() * 20 : 30 + Math.random() * 40, vx: (Math.random() - 0.5) * (e.type === 'hunter' ? 70 : 130) };
            e.dTmr = e.type === 'hunter' ? 4500 : 3000;
            e.sTmr = e.type === 'hunter' ? 280 + Math.random() * 420 : 500 + Math.random() * 1000;
            if (e.type === 'hunter') SFX.hunterDive(e.x); else SFX.dive();
            if ((e.type === 'boss' || e.type === 'miniboss') && !e.hasCap && !G.beam && G.stage > 1 && Math.random() < 0.3) G.beam = { active: true, owner: e, x: e.x, y: e.y + 16, h: 0, t: 0, cap: false, capT: 0 };
        }

        function updateE(dt) {
            const eDt = dt * G.timeScale;
            const dtMs = eDt * 1000; G.fTmr += dt; G.fX = Math.sin(G.fTmr * 0.5) * 30;
            for (const e of G.enemies) {
                if (e.st === 'DEAD') continue;
                // NEW: Sprite animation system (replaces old fr/frT toggle)
                e.animTimer += dtMs;
                const maxFrames = e.animFrames || 3;
                if (e.animTimer >= e.animSpeed) {
                    e.animFrame = (e.animFrame + 1) % maxFrames;
                    e.animTimer -= e.animSpeed;
                }
                // Keep old fr for backward compat in rendering (maps to animFrame)
                e.fr = e.animFrame % maxFrames;
                e.frT = e.animTimer;
                if (e.hitF > 0) e.hitF -= dtMs;
                // NEW: Boss phase transition
                if ((e.type === 'boss' || e.type === 'miniboss') && e.bossPhase > 0 && e.bossPhase < 3 && e.bossPhaseTransition <= 0) {
                    const hpRatio = e.hp / e.maxHp;
                    if (hpRatio <= e.bossPhaseHP[e.bossPhase - 1]) {
                        e.bossPhase++;
                        e.bossPhaseTransition = 800;
                        e.invulnerable = true;
                        G.shkT = Math.max(G.shkT, 400); G.shkM = Math.max(G.shkM, 5);
                        G.flashT = 100;
                        SFX.bossWarning();
                        // Spawn phase transition particles
                        for (let _pi = 0; _pi < 20; _pi++) {
                            const _pa = Math.random() * Math.PI * 2;
                            G.part.push({ x: e.x, y: e.y, vx: Math.cos(_pa) * 80, vy: Math.sin(_pa) * 80, life: 400, t: 0, col: e.bossPhase === 2 ? '#ff8800' : '#ff4444', size: 2, spark: true });
                        }
                    }
                }
                if (e.bossPhaseTransition > 0) {
                    e.bossPhaseTransition -= dtMs;
                    if (e.bossPhaseTransition <= 0) e.invulnerable = false;
                }
                if (G.freezeT > 0 && e.st !== 'ENTER') continue;
                if (e.st === 'ENTER') {
                    e.eTmr -= dtMs;
                    if (e.eTmr <= 0) {
                        const enterK = Math.min(1, eDt * 5);
                        e.x += (e.fx - e.x) * enterK;
                        e.y += (e.fy - e.y) * enterK;
                        if (Math.abs(e.x - e.fx) < 2 && Math.abs(e.y - e.fy) < 2) {
                            e.x = e.fx + G.fX;
                            e.y = e.fy + Math.sin(G.fTmr * 2 + e.col * 0.5) * 3;
                            e.st = 'FORM';
                            for (let _ei = 0; _ei < 2; _ei++) { const _ea = Math.random() * Math.PI * 2; G.part.push({ x: e.x, y: e.y, vx: Math.cos(_ea)*25, vy: Math.sin(_ea)*25, life: 200, t: 0, col: e.type === 'bee' ? '#ffcc00' : e.type === 'butterfly' ? '#ff3366' : e.type === 'hunter' ? '#ff6600' : e.type === 'spinner' ? '#44ffff' : e.type === 'bomber' ? '#cc66ff' : e.type === 'lasher' ? '#44ff88' : '#44cc44', size: 1, spark: true }); }
                            if ((e.type === 'boss' || e.type === 'miniboss') && !G.bossWarningShown) { G.bossWarningT = 2000; G.bossWarningShown = true; if (e.type === 'miniboss') SFX.miniBossWarning(); else SFX.bossWarning(); }
                            if (e.type === 'hunter') { G.bossWarningT = Math.max(G.bossWarningT || 0, 1000); SFX.hunterDive(e.x); }
                        }
                    }
                }
                else if (e.st === 'FORM') {
                    e.x = e.fx + G.fX; e.y = e.fy + Math.sin(G.fTmr * 2 + e.col * 0.5) * 3;
                    // NEW: Weaver sine-wave horizontal movement
                    if (e.type === 'weaver') {
                        e.x += Math.sin(G.fTmr * 3 + e.col) * 40;
                        e.sTmr -= dtMs;
                        if (e.sTmr <= 0 && G.p.alive && G.freezeT <= 0) {
                            enemyFire(e);
                            e.sTmr = 1800 + Math.random() * 1200;
                        }
                    }
                    // NEW: Teleporter behavior
                    if (e.type === 'teleporter') {
                        e.teleportTimer = (e.teleportTimer || 0) - dtMs;
                        if (e.teleportTimer <= 0) {
                            e.teleportTimer = 2000 + Math.random() * 1000;
                            e.x = 40 + Math.random() * (W - 80);
                            e.y = FTOP + Math.random() * 100;
                            for (let _ti = 0; _ti < 8; _ti++) {
                                const _ta = (_ti / 8) * Math.PI * 2;
                                G.part.push({ x: e.x, y: e.y, vx: Math.cos(_ta) * 30, vy: Math.sin(_ta) * 30, life: 200, t: 0, col: '#44ffff', size: 1, spark: true });
                            }
                        }
                        e.sTmr -= dtMs;
                        if (e.sTmr <= 0 && G.p.alive && G.freezeT <= 0) {
                            enemyFire(e);
                            e.sTmr = 1500 + Math.random() * 1000;
                        }
                    }
                    if ((e.type === 'sniper' || e.type === 'spinner' || e.type === 'bomber' || e.type === 'lasher' || e.type === 'weaver' || e.type === 'splitter' || e.type === 'shield_bee' || e.type === 'carrier' || e.type === 'teleporter') && G.p.alive && G.freezeT <= 0) {
                        e.sTmr -= dtMs;
                        if (e.sTmr <= 0) {
                            enemyFire(e);
                            e.sTmr = e.type === 'spinner' ? 1600 + Math.random() * 1200 : e.type === 'bomber' ? 2400 + Math.random() * 1400 : e.type === 'lasher' ? 2100 + Math.random() * 1600 : 2000 + Math.random() * 1500;
                        }
                    }
                    if ((e.type === 'stalker' || e.type === 'hunter') && G.freezeT <= 0) { e.dTmr -= dtMs * 2; }
                    else if (!G.chal) { e.dTmr -= dtMs; }
                    if (G.st === 'PLAYING' && e.dTmr <= 0 && !G.chal && Math.random() < 0.008 * Math.min(G.stage, 10) * diffMod('diveRate') * diveRateMult(e)) startDive(e);
                    else { e.dTmr -= dtMs; if (e.dTmr <= 0) { if (G.chal && G.st === 'PLAYING') startChalDive(e); else if (G.st === 'PLAYING') startDive(e); } }
                }
                else if (e.st === 'DIVING') {
                    e.dTmr -= dtMs;
                    if (e.dTmr <= 0 || e.y > H + 20) { e.st = 'RETURN'; e.y = -20; }
                    else {
                        const diveSpd = DIVE_SPD * (e.type === 'hunter' ? 2.1 : e.type === 'stalker' ? 1.5 : e.type === 'kamikaze' ? 2.5 : 1);
                        e.y += diveSpd * eDt;
                        if (e.type === 'hunter' && G.p.alive) {
                            e.x += (G.p.x - e.x) * eDt * 4.8;
                            e.y += (G.p.y - e.y) * eDt * 1.1;
                        } else if (e.type === 'stalker' && G.p.alive) { e.x += (G.p.x - e.x) * eDt * 2.5; }
                        // NEW: Kamikaze charges directly at player
                        else if (e.type === 'kamikaze' && G.p.alive) {
                            const kdx = G.p.x - e.x, kdy = G.p.y - e.y, kdist = Math.hypot(kdx, kdy) || 1;
                            e.x += (kdx / kdist) * diveSpd * 1.8 * eDt;
                            e.y += (kdy / kdist) * diveSpd * 1.8 * eDt;
                        }
                        else if (e.dPath) { e.dPath.ph += eDt * 3; e.x += e.dPath.vx * eDt + Math.cos(e.dPath.ph) * e.dPath.amp * 3 * eDt; }
                        if (G.beam && G.beam.owner === e) { G.beam.x = e.x; G.beam.y = e.y + 16; }
                        e.sTmr -= dtMs;
                        if (e.sTmr <= 0 && !G.chal) {
                            enemyFire(e);
                            e.sTmr = e.type === 'hunter' ? 350 + Math.random() * 450 : e.type === 'miniboss' ? 500 + Math.random() * 800 : (e.type === 'spinner' || e.type === 'bomber' || e.type === 'lasher') ? 600 + Math.random() * 700 : 800 + Math.random() * 1200;
                        }
                        if (G.p.alive && G.p.inv <= 0) {
                            const ew = (e.type === 'boss' || e.type === 'miniboss') ? 16 : e.type === 'hunter' ? 14 : 12;
                            if (hit({ x: e.x - ew / 2, y: e.y - 8, w: ew, h: 16 }, { x: G.p.x - 6, y: G.p.y - 6, w: 12, h: 12 })) {
                                // NEW: Kamikaze explodes on contact, damaging player
                                if (e.type === 'kamikaze') {
                                    boom(e.x, e.y, false, 'kamikaze');
                                    G.shkT = Math.max(G.shkT, 300); G.shkM = Math.max(G.shkM, 5);
                                }
                                registerKill(); addScore(PTS[e.type] ? PTS[e.type][1] : 200, e.x, e.y); boom(e.x, e.y, e.type === 'boss' || e.type === 'miniboss', e.type); SFX.eExplode(e.x); if (G.chal) G.chalHits++; e.st = 'DEAD'; killP();
                            }
                        }
                    }
                }
                else if (e.st === 'RETURN') { e.x += (e.fx + G.fX - e.x) * eDt * 3; e.y += (e.fy - e.y) * eDt * 3; if (Math.abs(e.x - e.fx - G.fX) < 3 && Math.abs(e.y - e.fy) < 3) { if (G.chal) { e.st = 'DEAD'; G.chalHits++; } else e.st = 'FORM'; } }
            }
            if (G.beam && G.beam.active) { G.beam.t += dtMs; G.beam.h = Math.min(200, G.beam.h + eDt * 300); if (G.beam.t > 3000) { G.beam.active = false; if (G.beam.cap && G.p.cap) { G.beam.owner.hasCap = true; G.p.cap = null; } } }
            G.dTmr -= dtMs;
            if (G.dTmr <= 0 && !G.chal && G.st === 'PLAYING') {
                const fe = G.enemies.filter(e => e.st === 'FORM');
                if (fe.length) {
                    const hunters = fe.filter(e => e.type === 'hunter' || e.type === 'stalker');
                    const pick = hunters.length && Math.random() < 0.45 ? hunters[Math.floor(Math.random() * hunters.length)] : fe[Math.floor(Math.random() * fe.length)];
                    startDive(pick);
                }
                G.dTmr = Math.max(500, (2000 - G.stage * 100) / diffMod('diveRate'));
            }
            const alive = G.enemies.filter(e => e.st !== 'DEAD');
            if (alive.length === 0 && G.levelSkipTimer <= 0 && G.st === 'PLAYING' && G.stageClearLock <= 0) {
                if (G.chal && G.chalHits === G.chalTot) { G.perfectT = 2000; addScore(5000, W / 2, H / 2 - 40, '#00ffcc'); SFX.perfect(); G.perfectCount++; if (G.perfectCount >= 3) unlockAchievement('perfectionist'); unlockAchievement('untouchable'); }
                advanceToNextStage(false);
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
            const _partCap = settings.particles === 'low' ? 60 : settings.particles === 'medium' ? 100 : 150;
            if (G.part.length > _partCap) G.part.length = _partCap;
            let plen = 0;
            for (let i = 0; i < G.part.length; i++) {
                const p = G.part[i];
                p.x += p.vx * dt; p.y += p.vy * dt;
                if (G.bgTheme === 'blackhole') { const _bhDx = W/2 - p.x, _bhDy = H/3 - p.y, _bhDist = Math.hypot(_bhDx, _bhDy); if (_bhDist > 10 && _bhDist < 150) { const _bhF = 60 / _bhDist; p.vx += (_bhDx / _bhDist) * _bhF * dt; p.vy += (_bhDy / _bhDist) * _bhF * dt; } }
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
            if (G.stageWipeT > 0) G.stageWipeT -= dtMs;
            if (G.damageVignetteT > 0) G.damageVignetteT -= dtMs;
            if (G.freezeT > 0) { G.freezeT -= dtMs; wrapEl.classList.add('galaxa-freeze'); if (G.freezeT <= 0) { G.freezeT = 0; wrapEl.classList.remove('galaxa-freeze'); if (G.activePU && G.activePU.type === 'freeze') { G.activePU = null; G.puTimer = 0; setPUClass(null); } } }
            if (G.warpT > 0) G.warpT -= dtMs;
            if (G.swipeT > 0) G.swipeT -= dtMs;
            if (G.portalT > 0) G.portalT -= dtMs;
            if (G.glitchT > 0) G.glitchT -= dtMs;
            if (G._closeCallCooldown > 0) G._closeCallCooldown -= dtMs;
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
            let bmlen = 0;
            for (let i = 0; i < G.pendingBooms.length; i++) {
                const bm = G.pendingBooms[i]; bm.delay -= dtMs;
                if (bm.delay <= 0) { boom(bm.x, bm.y, bm.isBoss); } else { G.pendingBooms[bmlen++] = bm; }
            }
            G.pendingBooms.length = bmlen;
            if (G.stageClearLock > 0) G.stageClearLock -= dtMs;
            if (G.levelSkipTimer > 0) {
                G.levelSkipTimer -= dtMs;
                if (G.levelSkipTimer <= 0 && G.st === 'PLAYING' && G.stageClearLock <= 0) {
                    G.levelSkipTimer = 0;
                    advanceToNextStage(true);
                }
            }
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
            if (dt > 0.1) dt = 0.1;
            const dtMs = dt * 1000;
            updateBackground(dt);
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
                else if (G.inp.s && !G.inp.sp) { SFX.coinInsert(); G.titleParts = []; G.score = 0; G.lives = diffMod('lives'); G.stage = 1; G.p.dual = false; G.p.cap = null; G.weaponLv = 1; G.killCount = 0; G.displayScore = 0; G.deathParts = []; G.collectedPU = new Set(); G.perfectCount = 0; G.bossKillTotal = 0; startStage(); MusicEngine.play('gameplay'); }
                if (!G.attract) {
                    if (Math.random() < 0.04) { const _tc = ['#4488ff','#ffcc00','#ff4444','#00ffcc','#ff88aa']; G.titleParts.push({ x: Math.random() * W, y: H + 5, vx: (Math.random()-0.5)*20, vy: -30 - Math.random()*40, life: 2500, t: 0, col: _tc[Math.floor(Math.random()*_tc.length)], size: 1 + Math.floor(Math.random()*2) }); }
                    let _tplen = 0; for (let _ti = 0; _ti < G.titleParts.length; _ti++) { const _tp = G.titleParts[_ti]; _tp.x += _tp.vx * dt; _tp.y += _tp.vy * dt; _tp.t += dt * 1000; if (_tp.t < _tp.life && _tp.y >= -10) G.titleParts[_tplen++] = _tp; } G.titleParts.length = _tplen;
                }
                return;
            }
            if (G.st === 'STAGE_INTRO') {
                G.introTmr -= dt * 1000;
                updateP(dt, now);
                updateBul(dt);
                updateE(dt);
                updateExp(dt);
                if (G.introTmr <= 0) { G.st = 'PLAYING'; G.introTmr = 0; }
                return;
            }
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
                if (G.p.cap) { G.p.cap.y -= 100 * dt; if (G.p.cap.y < G.p.y - 20) { G.p.dual = true; G.p.cap = null; SFX.rescue(); unlockAchievement('dual_wielder'); } }
                let bossAlive = false, minibossAlive = false, _aliveN = 0;
                for (let _ai = 0; _ai < G.enemies.length; _ai++) { const _ae = G.enemies[_ai]; if (_ae.st === 'DEAD') continue; _aliveN++; if (_ae.type === 'boss') bossAlive = true; else if (_ae.type === 'miniboss') { bossAlive = true; minibossAlive = true; } }
                if (_aliveN === 0 && G.levelSkipTimer <= 0 && G.stageClearLock <= 0) {
                    G.stageEmptyT += dtMs;
                    if (G.stageEmptyT > 350) {
                        G.stageEmptyT = 0;
                        mkFormation();
                        let _recovered = 0;
                        for (let _ri = 0; _ri < G.enemies.length; _ri++) { if (G.enemies[_ri].st !== 'DEAD') _recovered++; }
                        if (_recovered === 0) advanceToNextStage(false);
                    }
                } else {
                    G.stageEmptyT = 0;
                }
                const baseTheme = G.chal ? 'challenge' : 'gameplay';
                const bossTheme = minibossAlive ? 'miniboss' : 'boss';
                const effectiveBossTheme = G.stage >= 15 ? 'deep_boss' : bossTheme;
                if (bossAlive && MusicEngine.playing !== effectiveBossTheme) { SFX.bossJingle(); MusicEngine.play(effectiveBossTheme); }
                else if (!bossAlive && (MusicEngine.playing === 'boss' || MusicEngine.playing === 'miniboss' || MusicEngine.playing === 'deep_boss')) MusicEngine.play(baseTheme);
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
            if (d) G.settingsSel = Math.min(7, G.settingsSel + 1);
            if (f) {
                if (G.settingsSel === 7) { G.st = 'TITLE'; }
                else if (G.settingsSel === 0) { G.muted = !G.muted; settings.mute = G.muted; MusicEngine.setMuted(G.muted); saveSettings(); }
                else if (G.settingsSel === 4) { settings.crt = !settings.crt; if (settings.crt) wrapEl.classList.add('galaxa-crt'); else wrapEl.classList.remove('galaxa-crt'); saveSettings(); }
            }
            if (l || r) {
                if (G.settingsSel === 1) { settings.diff = l ? (settings.diff === 'hard' ? 'normal' : settings.diff === 'normal' ? 'easy' : 'easy') : (settings.diff === 'easy' ? 'normal' : settings.diff === 'normal' ? 'hard' : 'hard'); saveSettings(); }
                if (G.settingsSel === 2) { settings.vol = Math.max(0, Math.min(100, settings.vol + (l ? -10 : 10))); G.vol = settings.vol / 100; if (MusicEngine.masterGain) MusicEngine.masterGain.gain.value = G.muted ? 0 : G.vol * 0.35; saveSettings(); }
                if (G.settingsSel === 3) { const ships = Object.keys(SHIP_TYPES); const idx = ships.indexOf(settings.ship); settings.ship = l ? ships[(idx + ships.length - 1) % ships.length] : ships[(idx + 1) % ships.length]; saveSettings(); }
                if (G.settingsSel === 5) { const modes = ['high', 'medium', 'low']; const idx = modes.indexOf(settings.particles); settings.particles = l ? modes[(idx + modes.length - 1) % modes.length] : modes[(idx + 1) % modes.length]; saveSettings(); }
                if (G.settingsSel === 6) { settings.shake = Math.max(0, Math.min(1, settings.shake + (l ? -0.25 : 0.25))); saveSettings(); }
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

        function renderFrame(dt) {
            c.save(); c.setTransform(scale, 0, 0, scale, 0, 0);
            let sx = 0, sy = 0; if (G.shkT > 0 && settings.shake > 0) { const _decay = Math.min(1, G.shkT / 200); const _sm = settings.shake || 1; sx = (Math.random() - 0.5) * G.shkM * _decay * _sm; sy = (Math.random() - 0.5) * G.shkM * _decay * _sm; }
            c.translate(sx, sy); c.fillStyle = '#000'; c.fillRect(-5, -5, W + 10, H + 10);
            drawNebula(c); drawStars(c);
            if (G.chromAb > 0) {
                const ca = Math.min(1, G.chromAb / 200);
                const offset = Math.round(ca * 3);
                c.globalCompositeOperation = 'lighter';
                c.globalAlpha = ca * 0.08;
                c.fillStyle = '#ff0000'; c.fillRect(offset, 0, W, H);
                c.fillStyle = '#0000ff'; c.fillRect(-offset, 0, W, H);
                c.globalAlpha = ca * 0.04;
                c.fillStyle = '#00ff00'; c.fillRect(0, offset, W, H);
                c.globalCompositeOperation = 'source-over';
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
            // NEW: Alternative screen transitions
            if (G.swipeT > 0) {
                const progress = 1 - (G.swipeT / 1200);
                const wipeX = G.swipeDir > 0 ? W * progress : W * (1 - progress);
                c.fillStyle = '#000';
                if (G.swipeDir > 0) c.fillRect(0, 0, wipeX, H);
                else c.fillRect(wipeX, 0, W - wipeX, H);
                c.strokeStyle = '#4488ff'; c.lineWidth = 2;
                c.beginPath(); c.moveTo(wipeX, 0); c.lineTo(wipeX, H); c.stroke();
            }
            if (G.portalT > 0) {
                const progress = 1 - (G.portalT / 1400);
                G.portalR = progress * Math.max(W, H) * 0.8;
                c.save();
                c.beginPath(); c.arc(W / 2, H / 2, G.portalR, 0, Math.PI * 2); c.clip();
                c.fillStyle = '#000'; c.fillRect(0, 0, W, H);
                c.restore();
                c.strokeStyle = '#ffcc00'; c.lineWidth = 3; c.shadowBlur = 15; c.shadowColor = '#ffcc00';
                c.beginPath(); c.arc(W / 2, H / 2, G.portalR, 0, Math.PI * 2); c.stroke();
                c.shadowBlur = 0;
            }
            if (G.glitchT > 0) {
                const progress = 1 - (G.glitchT / 1000);
                c.save();
                for (const strip of G.glitchStrips) {
                    strip.offset += (strip.targetOffset - strip.offset) * 0.1;
                    const alpha = Math.abs(Math.sin(progress * Math.PI * 3 + strip.y * 0.1)) * 0.8;
                    c.globalAlpha = alpha;
                    c.fillStyle = '#000';
                    c.fillRect(0, strip.y, W, H / 12);
                    if (Math.random() < 0.3) {
                        c.fillStyle = ['#ff0000', '#00ff00', '#0000ff', '#ffffff'][Math.floor(Math.random() * 4)];
                        c.fillRect(strip.offset, strip.y, 2, H / 12);
                    }
                }
                c.restore();
            }
            if (G.flashT > 0) { c.fillStyle = 'rgba(255,255,255,' + (G.flashT > 30 ? 0.5 : G.flashT / 60) + ')'; c.fillRect(0, 0, W, H); }
            if (G.stageWipeT > 0) { const wp = G.stageWipeT / 400; c.fillStyle = '#000'; c.fillRect(0, 0, W, H * wp); c.fillRect(0, H * (1 - wp), W, H * wp); }
            if (G.levelSkipTimer > 0) {
                const _lsA = Math.min(1, G.levelSkipTimer / 500) * 0.15;
                c.fillStyle = 'rgba(255,136,255,' + _lsA + ')'; c.fillRect(0, 0, W, H);
            }
            if (G.st === 'TITLE' && !G.attract) renderTitle();
            else if (G.st === 'STAGE_INTRO') { renderGame(); renderStageIntro(); }
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
            const achKeys = Object.keys(G.achievements).filter(k => G.achievements[k]);
            if (achKeys.length > 0) {
                c.fillStyle = '#ffcc00'; c.font = 'bold 10px "Courier New",monospace'; c.textAlign = 'center';
                c.fillText('ACHIEVEMENTS: ' + achKeys.length + '/' + Object.keys(ACHIEVEMENTS).length, W / 2, H - 70);
                c.fillStyle = '#888'; c.font = '9px "Courier New",monospace';
                const achNames = achKeys.slice(0, 4).map(k => ACHIEVEMENTS[k] ? ACHIEVEMENTS[k].name : k);
                c.fillText(achNames.join(' | '), W / 2, H - 55);
            }
            c.fillStyle = '#666'; c.font = '10px "Courier New",monospace'; c.fillText('ARROWS+SPACE  GAMEPAD  SHIFT+S=SETTINGS  M=MUTE', W / 2, H - 40);
        }

        function renderStageIntro() {
            c.textAlign = 'center';
            const sc = Math.max(1, 3 - (G.introTmr / 1200) * 2);
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
                const _egGlow = G.activePU && PU_COL[G.activePU.type] ? PU_COL[G.activePU.type] : '#ff6600';
                const _egInt = 0.25 + Math.sin(tick * 0.15) * 0.15;
                const _eglG = cachedRadialGradient(c, 'engGlow:' + _egGlow, p.x, p.y + 14, 0, 18, [[0, _egGlow + '88'], [0.5, _egGlow + '22'], [1, 'transparent']]);
                c.globalAlpha = _egInt; c.fillStyle = _eglG; c.fillRect(p.x - 20, p.y - 4, 40, 36); c.globalAlpha = 1;
                if (p.inv > 0) {
                    const rpc = rainbowPC();
                    drawSp(c, SP.player, rpc, p.x - 12, p.y - 12, false, true);
                    if (p.dual) drawSp(c, SP.player, rpc, p.x + 28, p.y - 12, false, true);
                } else {
                    drawSp(c, SP.player, SP.pC, p.x - 12, p.y - 12, false);
                    if (p.dual) drawSp(c, SP.player, SP.pC, p.x + 28, p.y - 12, false);
                }
                if (p.alive) {
                    const eg = 0.5 + Math.sin(tick * 0.15) * 0.3;
                    const flameGlowCol = G.activePU && PU_COL[G.activePU.type] ? PU_COL[G.activePU.type] : '#ff6600';
                    renderFlame(c, p.x - 6, p.y + 11, eg, tick);
                    renderFlame(c, p.x + 3, p.y + 11, eg, tick);
                    if (p.dual) {
                        renderFlame(c, p.x + 28, p.y + 11, eg, tick);
                        renderFlame(c, p.x + 34, p.y + 11, eg, tick);
                    }
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
                const trCol = b.kind === 'plasma' ? 'rgba(68,255,136,0.3)' : b.kind === 'spiral' ? 'rgba(68,255,255,0.28)' : b.kind === 'mine' ? 'rgba(204,102,255,0.35)' : b.kind === 'hunter' ? 'rgba(255,136,68,0.3)' : 'rgba(255,68,68,0.25)';
                c.fillStyle = trCol;
                const tw = b.w || 2, th = b.h || 4;
                c.fillRect(Math.floor(b.x - tw / 2), Math.floor(b.y + 2), tw, th);
            }
            // enemy bullets — batched by kind
            const ebKinds = [
                { kinds: ['plasma'], fill: '#66ffaa', glow: '#44ff88', shadow: '#00cc66' },
                { kinds: ['spiral', 'sniper'], fill: '#66ffff', glow: '#44dddd', shadow: '#00cccc' },
                { kinds: ['mine'], fill: '#dd88ff', glow: '#cc66ff', shadow: '#aa44cc' },
                { kinds: ['hunter'], fill: '#ff8844', glow: '#ff6622', shadow: '#ff4400' },
                { kinds: ['bolt'], fill: '#ff6666', glow: '#ff4444', shadow: '#ff4444' }
            ];
            for (const batch of ebKinds) {
                c.shadowColor = batch.shadow; c.shadowBlur = 6;
                for (const b of G.ebul) {
                    if (!batch.kinds.includes(b.kind || 'bolt')) continue;
                    const bw = b.w || 2, bh = b.h || 6;
                    if (b.kind === 'mine') {
                        const pulse = 0.7 + Math.sin(tick * 0.2 + b.t * 0.01) * 0.3;
                        c.globalAlpha = pulse;
                        c.fillStyle = batch.fill;
                        c.beginPath(); c.arc(b.x, b.y, 4, 0, Math.PI * 2); c.fill();
                        c.globalAlpha = 1;
                        continue;
                    }
                    c.fillStyle = batch.fill;
                    c.fillRect(Math.floor(b.x - bw / 2), Math.floor(b.y - bh / 2), bw, bh);
                    c.globalAlpha = 0.35;
                    c.fillStyle = batch.glow;
                    c.fillRect(Math.floor(b.x - bw / 2 - 1), Math.floor(b.y - bh / 2 - 1), bw + 2, bh + 2);
                    c.globalAlpha = 1;
                }
            }
            c.shadowBlur = 0;

            // Boss telegraph lines — show dive path before attack
            if (G.p && G.p.alive) {
                c.setLineDash([2, 4]);
                for (const _te of G.enemies) {
                    if (_te.st !== 'DIVING' || _te.type === 'bee' || G.freezeT > 0) continue;
                    const _showTel = _te.type === 'hunter' || _te.type === 'sniper' || _te.type === 'lasher' || (_te.sTmr !== undefined && _te.sTmr <= 250 && _te.sTmr >= 0);
                    if (!_showTel) continue;
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
                    const _bossVar = (G.stage - 1) % 3;
                    const _bossCols = _bossVar === 1 ? SP.bossRedC : _bossVar === 2 ? SP.bossBlueC : SP.bossC;
                    if (e.type === 'bee') { sp = SP.bee[e.fr]; cols = SP.bC; } else if (e.type === 'butterfly') { sp = SP.bf[e.fr]; cols = SP.bfC; } else if (e.type === 'stalker') { sp = SP.stalker[e.fr]; cols = SP.stalkerC; } else if (e.type === 'sniper') { sp = SP.sniper[e.fr]; cols = SP.sniperC; } else if (e.type === 'hunter') { sp = SP.hunter[e.fr]; cols = SP.hunterC; } else if (e.type === 'spinner') { sp = SP.spinner[e.fr]; cols = SP.spinnerC; } else if (e.type === 'bomber') { sp = SP.bomber[e.fr]; cols = SP.bomberC; } else if (e.type === 'lasher') { sp = SP.lasher[e.fr]; cols = SP.lasherC; } else if (e.type === 'weaver') { sp = SP.weaver[e.fr]; cols = SP.weaverC; } else if (e.type === 'splitter') { sp = SP.splitter[e.fr]; cols = SP.splitterC; } else if (e.type === 'shield_bee') { sp = SP.shield_bee[e.fr]; cols = SP.shield_beeC; } else if (e.type === 'kamikaze') { sp = SP.kamikaze[e.fr]; cols = SP.kamikazeC; } else if (e.type === 'carrier') { sp = SP.carrier[e.fr]; cols = SP.carrierC; } else if (e.type === 'teleporter') { sp = SP.teleporter[e.fr]; cols = SP.teleporterC; } else if (e.type === 'miniboss') { sp = e.hp <= 1 ? SP.bossCrit : e.hp <= Math.ceil(e.maxHp / 2) ? SP.bossHit : SP.boss; cols = _bossCols; } else { sp = e.hp <= 1 ? SP.bossCrit : e.hp <= Math.ceil(e.maxHp / 2) ? SP.bossHit : SP.boss; cols = _bossCols; }
                    drawSp(c, sp, cols, e.x - 12, e.y - 18, false);
                    drawSp(c, sp, cols, e.x - 12, e.y - 10, false);
                    c.globalAlpha = 1;
                }
                const fl = e.hitF > 0; let sp, cols;
                const bossVariant = (G.stage - 1) % 3;
                const bossCols = bossVariant === 1 ? SP.bossRedC : bossVariant === 2 ? SP.bossBlueC : SP.bossC;
                if (e.type === 'bee') { sp = SP.bee[e.fr]; cols = SP.bC; } else if (e.type === 'butterfly') { sp = SP.bf[e.fr]; cols = SP.bfC; } else if (e.type === 'stalker') { sp = SP.stalker[e.fr]; cols = SP.stalkerC; } else if (e.type === 'sniper') { sp = SP.sniper[e.fr]; cols = SP.sniperC; } else if (e.type === 'hunter') { sp = SP.hunter[e.fr]; cols = SP.hunterC; } else if (e.type === 'spinner') { sp = SP.spinner[e.fr]; cols = SP.spinnerC; } else if (e.type === 'bomber') { sp = SP.bomber[e.fr]; cols = SP.bomberC; } else if (e.type === 'lasher') { sp = SP.lasher[e.fr]; cols = SP.lasherC; } else if (e.type === 'weaver') { sp = SP.weaver[e.fr]; cols = SP.weaverC; } else if (e.type === 'splitter') { sp = SP.splitter[e.fr]; cols = SP.splitterC; } else if (e.type === 'shield_bee') { sp = SP.shield_bee[e.fr]; cols = SP.shield_beeC; } else if (e.type === 'kamikaze') { sp = SP.kamikaze[e.fr]; cols = SP.kamikazeC; } else if (e.type === 'carrier') { sp = SP.carrier[e.fr]; cols = SP.carrierC; } else if (e.type === 'teleporter') { sp = SP.teleporter[e.fr]; cols = SP.teleporterC; } else if (e.type === 'miniboss') { sp = e.hp <= 1 ? SP.bossCrit : e.hp <= Math.ceil(e.maxHp / 2) ? SP.bossHit : SP.boss; cols = bossCols; } else { sp = e.hp <= 1 ? SP.bossCrit : e.hp <= Math.ceil(e.maxHp / 2) ? SP.bossHit : SP.boss; cols = bossCols; }
                if (e.type === 'hunter' && e.st !== 'DEAD') {
                    c.globalAlpha = 0.25 + Math.sin(tick * 0.12) * 0.1;
                    c.shadowBlur = 10; c.shadowColor = '#ff6600';
                    drawSp(c, sp, cols, e.x - 12, e.y - 12, false);
                    c.shadowBlur = 0; c.globalAlpha = 1;
                }
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
                c.shadowColor = '#aa66ee';
                for (const e of G.enemies) {
                    if (e.st === 'DEAD' || e.hitF > 0 || e.type !== 'stalker') continue;
                    drawSp(c, SP.stalker[e.fr], SP.stalkerC, e.x - 12, e.y - 12, false);
                }
                c.shadowColor = '#ffff44';
                for (const e of G.enemies) {
                    if (e.st === 'DEAD' || e.hitF > 0 || e.type !== 'sniper') continue;
                    drawSp(c, SP.sniper[e.fr], SP.sniperC, e.x - 12, e.y - 12, false);
                }
                c.shadowColor = '#ff6600';
                for (const e of G.enemies) {
                    if (e.st === 'DEAD' || e.hitF > 0 || e.type !== 'hunter') continue;
                    drawSp(c, SP.hunter[e.fr], SP.hunterC, e.x - 12, e.y - 12, false);
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
                else if (pu.type === 'levelskip') {
                    c.strokeStyle = PU_COL.levelskip; c.fillStyle = PU_COL.levelskip; c.lineWidth = 1;
                    c.beginPath(); c.moveTo(-3, -4); c.lineTo(1, 0); c.lineTo(-3, 4); c.closePath(); c.fill();
                    c.beginPath(); c.moveTo(1, -4); c.lineTo(5, 0); c.lineTo(1, 4); c.closePath(); c.fill();
                }
                else { for (let i2 = 0; i2 < 5; i2++) { const a2 = i2 * 1.26; c.fillRect(Math.floor(Math.cos(a2) * 4), Math.floor(Math.sin(a2) * 4), 2, 2); } }
                c.restore();
                c.shadowBlur = 0;
            }
            for (const dr of G.drones) {
                c.fillStyle = '#44ffaa'; c.shadowBlur = 4; c.shadowColor = '#44ffaa';
                c.fillRect(Math.floor(dr.x - 3), Math.floor(dr.y - 3), 6, 6);
                c.fillStyle = '#88ffcc'; c.fillRect(Math.floor(dr.x - 1), Math.floor(dr.y - 1), 2, 2);
                c.shadowBlur = 0;
            }
            if (G.blackhole) {
                const bha = Math.min(1, G.blackhole.t / 500);
                const bhr = 15 + Math.sin(tick * 0.1) * 5;
                c.globalAlpha = 0.6;
                const bhGr = c.createRadialGradient(G.blackhole.x, G.blackhole.y, 0, G.blackhole.x, G.blackhole.y, bhr + 20);
                bhGr.addColorStop(0, '#000'); bhGr.addColorStop(0.4, '#220044'); bhGr.addColorStop(0.7, '#440088'); bhGr.addColorStop(1, 'transparent');
                c.fillStyle = bhGr; c.fillRect(G.blackhole.x - bhr - 20, G.blackhole.y - bhr - 20, (bhr + 20) * 2, (bhr + 20) * 2);
                c.globalAlpha = 0.8;
                c.strokeStyle = '#8844ff'; c.lineWidth = 1.5;
                c.shadowBlur = 8; c.shadowColor = '#8844ff';
                c.beginPath(); c.arc(G.blackhole.x, G.blackhole.y, bhr, 0, Math.PI * 2); c.stroke();
                c.shadowBlur = 0; c.globalAlpha = 1;
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
                if (pt.spark) {
                    c.globalAlpha = alpha; c.fillStyle = pt.col; c.fillRect(Math.floor(pt.x), Math.floor(pt.y), 1, 1);
                } else if (pt.smoke) {
                    c.globalAlpha = alpha * 0.35; c.fillStyle = pt.col;
                    c.fillRect(Math.floor(pt.x), Math.floor(pt.y), pt.size || 3, pt.size || 3);
                } else if (pt.debris) {
                    c.globalAlpha = alpha;
                    c.save(); c.translate(pt.x, pt.y); c.rotate(pt.rot);
                    c.fillStyle = pt.col; c.fillRect(-pt.size / 2, -pt.size / 2, pt.size, pt.size);
                    c.restore();
                } else {
                    c.globalAlpha = alpha; c.fillStyle = pt.col;
                    if (pt.size >= 3) { c.shadowBlur = 6; c.shadowColor = pt.col; c.fillRect(Math.floor(pt.x), Math.floor(pt.y), pt.size || 2, pt.size || 2); c.shadowBlur = 0; }
                    else c.fillRect(Math.floor(pt.x), Math.floor(pt.y), pt.size || 2, pt.size || 2);
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
            for (let _api = 0; _api < G.achievementPopups.length; _api++) {
                const ap = G.achievementPopups[_api];
                const apAlpha = ap.t < 300 ? ap.t / 300 : ap.t > ap.dur - 500 ? (ap.dur - ap.t) / 500 : 1;
                const apY = H - 60 - _api * 30;
                c.globalAlpha = apAlpha;
                c.fillStyle = 'rgba(0,0,0,0.7)'; c.fillRect(W / 2 - 100, apY - 10, 200, 22);
                c.strokeStyle = '#ffcc00'; c.lineWidth = 1; c.strokeRect(W / 2 - 100, apY - 10, 200, 22);
                c.shadowBlur = 6; c.shadowColor = '#ffcc00';
                c.fillStyle = '#ffcc00'; c.font = 'bold 10px "Courier New",monospace'; c.textAlign = 'center';
                c.fillText('ACHIEVEMENT: ' + ap.text, W / 2, apY + 4);
                c.shadowBlur = 0;
            }
            c.globalAlpha = 1;
            let _apLen = 0;
            for (let _api = 0; _api < G.achievementPopups.length; _api++) { G.achievementPopups[_api].t += 16; if (G.achievementPopups[_api].t < G.achievementPopups[_api].dur) G.achievementPopups[_apLen++] = G.achievementPopups[_api]; }
            G.achievementPopups.length = _apLen;

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
            if (G.combo > 0) {
                const comboRatio = Math.min(1, G.combo / 20);
                const cmx = W - 28, cmy = 54, cmr = 14;
                c.strokeStyle = '#333'; c.lineWidth = 2; c.beginPath(); c.arc(cmx, cmy, cmr, -Math.PI * 0.75, Math.PI * 0.75); c.stroke();
                const cmCol = G.combo >= 10 ? '#ff4444' : G.combo >= 5 ? '#ffcc00' : '#4488ff';
                c.strokeStyle = cmCol; c.lineWidth = 2;
                c.shadowBlur = 4; c.shadowColor = cmCol;
                c.beginPath(); c.arc(cmx, cmy, cmr, -Math.PI * 0.75, -Math.PI * 0.75 + comboRatio * Math.PI * 1.5); c.stroke();
                c.shadowBlur = 0;
                c.fillStyle = cmCol; c.font = 'bold 8px monospace'; c.textAlign = 'center';
                c.fillText(G.combo, cmx, cmy + 3);
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
                const ratio = G.puTimer / PU_DUR[G.activePU.type];
                const isExpiringSoon = G.puTimer < 2000 && G.puTimer > 0;
                const puCol = PU_COL[G.activePU.type];
                const barW = W * 0.6, barH = 3, barX = W / 2 - barW / 2, barY = 4;
                c.fillStyle = '#222'; c.fillRect(barX, barY, barW, barH);
                c.fillStyle = puCol; c.fillRect(barX, barY, barW * ratio, barH);
                if ((ratio < 0.3 || isExpiringSoon) && Math.sin(tick * (isExpiringSoon ? 0.4 : 0.2)) > 0) { c.fillStyle = '#fff'; c.fillRect(barX, barY, barW * ratio, barH); }
                if (G.p && G.p.alive) {
                    const cx = G.p.x, cy = G.p.y, r = 24;
                    const startA = -Math.PI / 2, endA = startA + ratio * Math.PI * 2;
                    c.strokeStyle = puCol; c.lineWidth = 2; c.globalAlpha = 0.5;
                    c.shadowBlur = 4; c.shadowColor = puCol;
                    c.beginPath(); c.arc(cx, cy, r, startA, endA); c.stroke();
                    c.shadowBlur = 0; c.globalAlpha = 1;
                }
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
            c.fillText(t('galaxa.settings', 'SETTINGS'), W / 2, 80);
            c.shadowBlur = 0;
            const shipName = t('galaxa.' + settings.ship, (SHIP_TYPES[settings.ship] || SHIP_TYPES.classic).name);
            const shakeLabel = settings.shake === 0 ? 'OFF' : settings.shake === 0.25 ? 'LOW' : settings.shake === 0.5 ? 'MED' : settings.shake === 0.75 ? 'HIGH' : 'MAX';
            const items = [
                { label: t('galaxa.sound', 'SOUND'), val: G.muted ? 'OFF' : 'ON' },
                { label: t('galaxa.difficulty', 'DIFFICULTY'), val: t('galaxa.' + settings.diff, settings.diff.toUpperCase()) },
                { label: t('galaxa.volume', 'VOLUME'), val: settings.vol + '%' },
                { label: t('galaxa.ship_select', 'SHIP'), val: shipName },
                { label: t('galaxa.crt_effect', 'CRT EFFECT'), val: settings.crt ? 'ON' : 'OFF' },
                { label: t('galaxa.particle_density', 'PARTICLES'), val: t('galaxa.' + settings.particles, settings.particles.toUpperCase()) },
                { label: t('galaxa.shake_intensity', 'SHAKE'), val: shakeLabel },
                { label: t('galaxa.quit', 'QUIT'), val: '' }
            ];
            items.forEach((it, i) => {
                const sel = i === G.settingsSel;
                c.fillStyle = sel ? '#ffcc00' : '#888'; c.font = sel ? 'bold 14px "Courier New",monospace' : '12px "Courier New",monospace';
                if (sel) { c.shadowBlur = 6; c.shadowColor = '#ffcc00'; }
                c.fillText(it.label + (it.val ? ': ' + it.val : ''), W / 2, 130 + i * 36);
                c.shadowBlur = 0;
                if (i === 2) {
                    const bw = 200, bh = 8, bx = W / 2 - bw / 2, by = 138 + i * 36;
                    c.fillStyle = '#222'; c.fillRect(bx, by, bw, bh);
                    c.fillStyle = '#4488ff'; c.fillRect(bx, by, bw * settings.vol / 100, bh);
                    if (sel) { c.strokeStyle = '#4488ff'; c.lineWidth = 1; c.strokeRect(bx - 1, by - 1, bw + 2, bh + 2); }
                }
            });
            c.fillStyle = '#666'; c.font = '10px "Courier New",monospace';
            c.fillText('\u2191\u2193 select  \u2190\u2192 change  ENTER confirm', W / 2, 430);
            c.fillText('ARROWS+SPACE  GAMEPAD D-PAD+A', W / 2, 450);
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

        let _frameBudgetSkip = 0;
        function loop() {
            if (state.disposed) return;
            const dt = frameDelta();
            savePrev(); pollGP(); mergeInput();
            update(dt, performance.now());
            tick++;
            renderFrame(dt);
            if (dt > 0.018) { _frameBudgetSkip = Math.min(3, _frameBudgetSkip + 1); } else if (_frameBudgetSkip > 0) _frameBudgetSkip--;
            rafId = requestAnimationFrame(loop);
        }

        document.addEventListener('keydown', onKey);
        document.addEventListener('keyup', onKeyUp);
        const ro = new ResizeObserver(() => {
            if (state.disposed) return;
            cancelAnimationFrame(resizeRaf);
            resizeRaf = requestAnimationFrame(() => { if (!state.disposed) resize(); });
        });
        ro.observe(host); resize();
        loadHS().then(() => { showTitle(); rafId = requestAnimationFrame(loop); });

        state.dispose = function () {
            state.disposed = true; cancelAnimationFrame(rafId); MusicEngine.stop(); G.pendingBooms = []; G.levelSkipTimer = 0;
            document.removeEventListener('keydown', onKey); document.removeEventListener('keyup', onKeyUp);
            ro.disconnect(); radialGradientCache.clear(); spriteAtlasCache.delete.bind(spriteAtlasCache);
            if (reverbNode) try { reverbNode.disconnect(); } catch (_) {}
            if (reverbGain) try { reverbGain.disconnect(); } catch (_) {}
            if (actx) try { actx.close(); } catch (e) {}
            setPUClass(null); wrapEl.classList.remove('galaxa-boss-warning');
            instances.delete(windowId);
        };
    }

    function dispose(windowId) { const s = instances.get(windowId); if (s && s.dispose) s.dispose(); }
    window.GalaxaDeluxe = { render, dispose };
})();
