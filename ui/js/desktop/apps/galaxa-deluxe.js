(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    const instances = new Map();

    function backupSettingsIfNeeded() {
        try {
            const current = localStorage.getItem('galaxa_settings');
            if (current && !localStorage.getItem('galaxa_settings_v1_backup')) {
                localStorage.setItem('galaxa_settings_v1_backup', current);
            }
        } catch (e) { /* localStorage unavailable, skip backup */ }
    }
    function loadSettings() {
        try { const s = JSON.parse(localStorage.getItem('galaxa_settings') || '{}'); return { vol: s.vol || 30, diff: s.diff || 'normal', mute: s.mute || false, ship: s.ship || 'classic', mode: s.mode || 'classic', crt: s.crt !== undefined ? s.crt : true, particles: s.particles || 'high', shake: s.shake !== undefined ? s.shake : 1, parry: s.parry !== undefined ? s.parry : true, riskIt: s.riskIt || false, adaptiveMusic: s.adaptiveMusic !== undefined ? s.adaptiveMusic : true }; } catch (e) { return { vol: 30, diff: 'normal', mute: false, ship: 'classic', mode: 'classic', crt: true, particles: 'high', shake: 1, parry: true, riskIt: false, adaptiveMusic: true }; }
    }
    function loadAchievements() { try { const a = JSON.parse(localStorage.getItem('galaxa_achievements') || '{}'); return a; } catch (e) { return {}; } }

    // Declarative settings menu items. The renderer iterates this list to
    // build the SETTINGS screen, and the input handler dispatches on `type`
    // to toggle / cycle / adjust the value. Adding a new setting only
    // requires a new entry here plus a key in loadSettings() defaults.
    GC.SETTINGS_ITEMS = [
        { id: 'sound', label: 'SOUND', type: 'toggle', key: 'mute' },
        { id: 'difficulty', label: 'DIFFICULTY', type: 'cycle', key: 'diff', values: ['easy', 'normal', 'hard'] },
        { id: 'volume', label: 'VOLUME', type: 'slider', key: 'vol', min: 0, max: 100, step: 10 },
        { id: 'ship', label: 'SHIP', type: 'cycle', key: 'ship', values: 'SHIPS' },
        { id: 'crt', label: 'CRT EFFECT', type: 'toggle', key: 'crt' },
        { id: 'particles', label: 'PARTICLES', type: 'cycle', key: 'particles', values: ['high', 'medium', 'low'] },
        { id: 'shake', label: 'SHAKE', type: 'cycle', key: 'shake', values: [0, 0.25, 0.5, 0.75, 1] },
        { id: 'riskIt', label: 'RISK-IT MODE', type: 'toggle', key: 'riskIt', onChange: 'riskItToggle' },
        { id: 'adaptiveMusic', label: 'ADAPTIVE MUSIC', type: 'toggle', key: 'adaptiveMusic' },
        { id: 'quit', label: 'QUIT', type: 'action', action: 'quit' }
    ];

    // Apply a left/right input to a settings item. Returns true if the value
    // changed so the caller can play a confirmation SFX if desired.
    GC.applySettingsInput = function (ctx, item, dir) {
        if (!ctx || !item) return false;
        const s = ctx.settings;
        switch (item.type) {
            case 'toggle': {
                s[item.key] = !s[item.key];
                if (ctx.saveSettings) ctx.saveSettings();
                if (item.onChange && ctx.SFX && typeof ctx.SFX[item.onChange] === 'function') ctx.SFX[item.onChange]();
                return true;
            }
            case 'cycle': {
                let pool;
                if (item.values === 'SHIPS') pool = Object.keys(ctx.SHIP_TYPES || {});
                else pool = item.values.slice();
                const idx = pool.indexOf(s[item.key]);
                const next = pool[(idx + pool.length + (dir < 0 ? -1 : 1)) % pool.length];
                s[item.key] = next;
                if (ctx.saveSettings) ctx.saveSettings();
                return true;
            }
            case 'slider': {
                const step = item.step || 1;
                const next = Math.max(item.min, Math.min(item.max, s[item.key] + (dir < 0 ? -step : step)));
                if (next !== s[item.key]) { s[item.key] = next; if (ctx.saveSettings) ctx.saveSettings(); return true; }
                return false;
            }
        }
        return false;
    };

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

        const settings = loadSettings();
        backupSettingsIfNeeded();
        if (!settings.crt) wrapEl.classList.remove('galaxa-crt');

        function saveSettings() { try { localStorage.setItem('galaxa_settings', JSON.stringify(settings)); } catch (e) {} }
        function saveAchievements() { try { localStorage.setItem('galaxa_achievements', JSON.stringify(gameCtx.G.achievements)); } catch (e) {} }
        function unlockAchievement(id) {
            if (gameCtx.G.achievements[id]) return;
            gameCtx.G.achievements[id] = true; saveAchievements();
            const def = GC.ACHIEVEMENTS[id];
            gameCtx.G.achievementPopups.push({ text: (def ? def.name : id), t: 0, dur: 3000 });
            gameCtx.SFX.perfect();
        }
        function diffMod(key) {
            const ship = GC.SHIP_TYPES[settings.ship] || GC.SHIP_TYPES.classic;
            if (settings.diff === 'easy') return { diveRate: 0.7, ebSpd: 0.8, lives: 5 + ship.lifeMod, puFromBee: true }[key];
            if (settings.diff === 'hard') return { diveRate: 1.5, ebSpd: 1.3, lives: 2 + ship.lifeMod, puFromBee: false }[key];
            return { diveRate: 1, ebSpd: 1, lives: 3 + ship.lifeMod, puFromBee: true }[key];
        }
        function getShipSpeed() { return GC.PLAYER_SPEED * (GC.SHIP_TYPES[settings.ship] || GC.SHIP_TYPES.classic).speedMult; }
        function getShipHitbox() { const mod = (GC.SHIP_TYPES[settings.ship] || GC.SHIP_TYPES.classic).hitboxMod; return { x: gameCtx.G.p.x - 8 + mod, y: gameCtx.G.p.y - 8 + mod, w: 16 - mod * 2, h: 16 - mod * 2 }; }
        function getShipInvMult() { return (GC.SHIP_TYPES[settings.ship] || GC.SHIP_TYPES.classic).invMult; }
        function setPUClass(type) {
            const cls = ['galaxa-powerup-active'];
            for (const k of [...GC.PU_TYPES, ...Object.keys(GC.PU_UPGRADE)]) cls.push('galaxa-powerup-' + k);
            cls.forEach(c2 => wrapEl.classList.remove(c2));
            if (type) wrapEl.classList.add('galaxa-powerup-active', 'galaxa-powerup-' + type);
        }
        let scale = 1, tick = 0, rafId = 0, clockT = 0, resizeRaf = 0, _frameBudgetSkip = 0;
        function frameDelta() {
            const now = performance.now();
            const raw = clockT ? (now - clockT) / 1000 : (1 / 60);
            clockT = now;
            return Math.min(Math.max(raw, 0.001), 0.05);
        }

        const gameCtx = Object.create(GC);
        Object.assign(gameCtx, {
            esc, t, api, state, canvas, overlayEl, wrapEl, c, settings,
            scale, tick: 0, rafId: 0, clockT: 0, resizeRaf: 0, _frameBudgetSkip: 0,
            actx: null, masterCompressor: null, reverbNode: null, reverbGain: null,
            frameDelta, saveSettings, saveAchievements, unlockAchievement,
            diffMod, getShipSpeed, getShipHitbox, getShipInvMult, setPUClass
        });

        GC.createAudio(gameCtx);
        GC.createTweens(gameCtx);
        GC.createSprites(gameCtx);
        GC.createBackground(gameCtx);
        GC.createEntitiesCore(gameCtx);
        GC.createEntitiesSpawning(gameCtx);
        GC.createEntitiesBehaviors(gameCtx);
        GC.createEntities(gameCtx);
        GC.createRenderer(gameCtx);
        GC.createRenderEffects(gameCtx);
        GC.createRenderStage(gameCtx);
        GC.createRenderHUD(gameCtx);
        GC.createSupers(gameCtx);
        GC.createBiomeTransitions(gameCtx);
        GC.createComboLadder(gameCtx);
        GC.createAdaptiveMusic(gameCtx);
        GC.createDemo(gameCtx);
        GC.createGame(gameCtx);
        GC.createShop(gameCtx);
        GC.createRelics(gameCtx);

        gameCtx.G = {
            st: 'TITLE', score: 0, lives: 3, stage: 1, hi: 10000, hiScores: [],
            p: { x: GC.W / 2, y: GC.H - 50, alive: true, inv: 0, dual: false, cap: null, reviveTimer: 0 },
            bul: [], ebul: [], enemies: [], exp: [], part: [],
            fX: 0, fTmr: 0, dTmr: 0, sTmr: 0, tIdle: 0,
            attract: false, aTmr: 0, ne: { ch: [65, 65, 65], pos: 0, done: false },
            chal: false, chalHits: 0, chalTot: 0, beam: null, shkT: 0, shkM: 0, shkX: 0, shkY: 0,
            powerups: [], activePU: null, puTimer: 0, shieldHits: 0,
            scorePopups: [], flashT: 0, warpT: 0, warpFlash: 0, perfectT: 0, contTmr: 0, contCnt: 0,
            damageVignetteT: 0, freezeT: 0, lightningT: 0, lightningX: 0, bgTheme: 'nebula',
            pauseSel: 0, settingsSel: 0, settingsVolDrag: false,
            combo: 0, comboTimer: 0, comboMult: 1, comboBanner: null,
            trails: [],
            timeScale: 1, timeSlowTimer: 0,
            bossWarningT: 0, bossWarningShown: false,
            weaponLv: 1, killCount: 0, puUpgrade: null, upgradeBanner: null,
            weaponXP: 0, weaponEvo: null, evoChoiceOpen: false,
            slowMoT: 0, chromAb: 0, displayScore: 0, shipTilt: 0, shipPitch: 0, muzzleT: 0, deathParts: [], pendingBooms: [], levelSkipTimer: 0,
            introTmr: 0, stageEmptyT: 0, stageClearLock: 0,
            beatPhase: 0, beatT: 0, plasmaRings: [], titleParts: [], drones: [], droneTimer: 0,
            blackhole: null,
            overcharge: 0, overchargeTimer: 0, scoreMult: 1, glassCannon: false, bulletStorm: false,
            powerSurge: false, darkness: false, turbo: false, stageModifier: null,
            credits: parseInt(localStorage.getItem('galaxa_credits') || '0'), shopOpen: false,
            noDamageStages: 0, pacifistStage: true, frozenKills: 0, ricochetKills: 0,
            blackholeKills: 0, dailyStreak: parseInt(localStorage.getItem('galaxa_daily_streak') || '0'),
            shipStageProgress: {}, transitionType: 0,
            envHazards: [], solarFlareT: 0, solarFlareActive: false, emStormT: 0,
            gravityBomb: null, mirrorActive: false, mirrorTimer: 0, voidZones: [], voidZoneT: 0,
            swipeT: 0, swipeDir: 1, portalT: 0, portalR: 0, glitchT: 0, glitchStrips: [],
            _closeCallCooldown: 0, _synergyChecked: null, shieldReflect: false, laserSlow: false, droneRicochet: false,
            orbitalShields: null, orbitalShieldTimer: 0,
            intensityScore: 5, stageKills: 0, stageDamageTaken: 0, stageAccuracyShots: 0, stageAccuracyHits: 0,
            rageKills: 0, chainMasterHits: 0, orbitalBlocks: 0, mutationStages: 0,
            mirrorField: false, gravityWell: false, phasing: false, ricochetWorld: false,
            inp: { l: false, r: false, f: false, fp: false, s: false, sp: false, p: false, pp: false, u: false, d: false, rp: false, lp: false, up: false, dp: false, parry: false, parryp: false, super: false, superp: false },
            kb: { l: false, r: false, u: false, d: false, f: false, s: false, p: false, parry: false, super: false },
            gp: { l: false, r: false, u: false, d: false, f: false, s: false, p: false, parry: false, super: false },
            ai: { l: false, r: false, u: false, d: false, f: false, s: false, p: false, parry: false, super: false },
            demoMode: false,
            muted: settings.mute, vol: settings.vol / 100, _prevSt: 'TITLE', gameMode: 'classic',
            achievements: loadAchievements(), achievementPopups: [], collectedPU: new Set(), stageStartTime: 0, perfectCount: 0, bossKillTotal: parseInt(localStorage.getItem('galaxa_boss_kills') || '0'),
            // NEW: Super State Machine (Phase 2)
            superPhase: 'idle', superPhaseT: 0, superBurstFired: false, superFreezeWorld: false,
            // NEW: Cinematic Camera
            camZoom: 1, camX: 0, camY: 0,
            // NEW: Biome Transitions
            transitionActive: false, transitionT: 0, transitionType: 'wipe',
            // NEW: Ranked Performance
            stageBoni: { noDamageRun: false, speedDemon: false, pacifist: false }, stageRank: null,
            // NEW: Adaptive Music State
            musicModulation: { combo: 0, bossPhase: 0, health: 3 }, biomeAmbientLayer: null,
            // NEW: Parry system
            parryActive: 0, parryCooldown: 0, parryCount: 0, parrySuccessFlash: 0,
            // NEW: Super / Overdrive meter
            superMeter: 0, superActive: 0, superType: null, superTimer: 0, superCooldown: 0,
            // NEW: Hitstop (short global freeze for impact)
            hitstopT: 0,
            // NEW: Floating combat text (damage/crit/elemental)
            combatText: [],
            // NEW: Ship banking target (eased tilt)
            shipTiltTarget: 0, shipPitchTarget: 0,
            // NEW: Biome progression
            biome: 'nebula', biomeName: 'NEBULA', biomeRevealT: 0,
            // NEW: Motion trail history buffers per bullet (capped)
            trailBudget: 0, parryMasterCount: 0,
            // NEW: Bonus sub-stage flag
            bonusStage: false, bonusStageT: 0, bonusRating: null
        };
        gameCtx.G.lives = diffMod('lives');

        gameCtx.initBG();
        gameCtx.mkNebula();

        document.addEventListener('keydown', gameCtx.onKey);
        document.addEventListener('keyup', gameCtx.onKeyUp);
        const ro = new ResizeObserver(() => {
            if (state.disposed) return;
            cancelAnimationFrame(gameCtx.resizeRaf);
            gameCtx.resizeRaf = requestAnimationFrame(() => { if (!state.disposed) gameCtx.resize(); });
        });
        ro.observe(host);
        gameCtx.resize();
        gameCtx.loadHS().then(() => { gameCtx.showTitle(); gameCtx.rafId = requestAnimationFrame(gameCtx.loop); if (gameCtx.checkDailyStreak) gameCtx.checkDailyStreak(); });
        if (gameCtx.setupTouch) gameCtx.setupTouch();

        state.dispose = function () {
            state.disposed = true; cancelAnimationFrame(gameCtx.rafId); gameCtx.MusicEngine.stop(); if (gameCtx.GalagaMusic) gameCtx.GalagaMusic.stop(); gameCtx.G.pendingBooms = []; gameCtx.G.levelSkipTimer = 0;
            gameCtx.G.demoMode = false; if (gameCtx.G.ai) { Object.keys(gameCtx.G.ai).forEach(function(k) { gameCtx.G.ai[k] = false; }); }
            document.removeEventListener('keydown', gameCtx.onKey); document.removeEventListener('keyup', gameCtx.onKeyUp);
            ro.disconnect(); gameCtx.radialGradientCache.clear(); gameCtx.spriteAtlasCache.delete.bind(gameCtx.spriteAtlasCache);
            if (gameCtx.reverbNode) try { gameCtx.reverbNode.disconnect(); } catch (_) {}
            if (gameCtx.reverbGain) try { gameCtx.reverbGain.disconnect(); } catch (_) {}
            if (gameCtx.actx) try { gameCtx.actx.close(); } catch (e) {}
            setPUClass(null); wrapEl.classList.remove('galaxa-boss-warning');
            instances.delete(windowId);
        };
    }

    function dispose(windowId) { const s = instances.get(windowId); if (s && s.dispose) s.dispose(); }
    window.GalaxaDeluxe = { render, dispose };
})();
