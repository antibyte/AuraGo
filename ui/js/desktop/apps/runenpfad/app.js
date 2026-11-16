/* Runenpfad desktop window lifecycle, HUD, modal dialogs and input controller. */
(function (root) {
    'use strict';

    const LOCALES = new Set(['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']);
    const PACKS = '/api/game-maker/asset-packs/';
    const sessions = new Map();

    function el(tag, className, text) {
        const node = document.createElement(tag);
        if (className) node.className = className;
        if (text !== undefined) node.textContent = text;
        return node;
    }

    function pickLocale(ctx) {
        const raw = String((ctx && ctx.lang) || root.SYSTEM_LANG || document.documentElement.lang || 'de').slice(0, 2).toLowerCase();
        return LOCALES.has(raw) ? raw : 'en';
    }

    function dispose(windowId) {
        const active = sessions.get(windowId);
        if (!active) return;
        sessions.delete(windowId);
        active.cleanup();
    }

    function render(host, windowId, ctx) {
        dispose(windowId);
        ctx = ctx || {};
        const Levels = root.RunenpfadLevels;
        const Sim = root.RunenpfadSim;
        const View = root.RunenpfadView;
        const Audio = root.RunenpfadAudio;

        const controller = new AbortController();
        const locale = pickLocale(ctx);
        let disposed = false;
        let generation = 0;
        let rafId = 0;
        let lastFrameMs = 0;
        let texts = {};
        let sheets = null;
        let renderer = null;
        let audio = null;
        let music = null;
        let postfx = null;
        let sim = null;
        let profile = readProfile();
        let selectedWorld = Math.floor((Math.min(25, Math.max(1, profile.unlockedLevel)) - 1) / 5);
        let userPaused = false;
        let resetArmed = false;

        const keyState = Object.create(null);
        const edgeState = { jumpPressed: false, attackPressed: false, dashPressed: false, healPressed: false };
        const touchState = Object.create(null);

        function t(key) {
            const v = texts[key];
            if (typeof v === 'string') return v;
            if (ctx.t) {
                const ext = ctx.t(key);
                if (typeof ext === 'string' && ext !== key) return ext;
            }
            return key;
        }

        function fill(key, params) {
            return t(key).replace(/\{([a-zA-Z0-9_]+)\}/g, function (m, name) {
                return Object.prototype.hasOwnProperty.call(params, name) ? String(params[name]) : m;
            });
        }

        let rejectedRaw = null; // stored value that failed to import; backed up before the first overwrite

        function readProfile() {
            try {
                const raw = root.localStorage.getItem(Sim.STORAGE_KEY);
                if (!raw) return Sim.createDefaultProfile();
                rejectedRaw = raw;
                const imported = raw.length > 16384 ? null : Sim.importProfile(JSON.parse(raw));
                if (imported) rejectedRaw = null;
                return imported || Sim.createDefaultProfile();
            } catch (err) {
                return Sim.createDefaultProfile();
            }
        }

        function writeProfile() {
            if (!profile) return;
            try {
                if (rejectedRaw !== null) {
                    const bak = Sim.STORAGE_KEY + '.bak';
                    if (rejectedRaw.length <= 16384 && root.localStorage.getItem(bak) === null) root.localStorage.setItem(bak, rejectedRaw);
                    rejectedRaw = null;
                }
                root.localStorage.setItem(Sim.STORAGE_KEY, JSON.stringify(Sim.exportProfile(profile)));
            } catch (err) { /* ignore storage quota/private mode */ }
        }

        // Build DOM tree via safe element/textContent primitives
        const shell = el('section', 'rp-app');
        shell.tabIndex = 0;
        shell.setAttribute('aria-label', ctx.t ? ctx.t('desktop.app_runenpfad') : 'Runenpfad');
        shell.setAttribute('lang', locale);

        const stageWrap = el('div', 'rp-stage');
        const canvas = el('canvas', 'rp-canvas');
        stageWrap.append(canvas);

        const hud = el('div', 'rp-hud');
        const topbar = el('div', 'rp-topbar');
        const brand = el('div', 'rp-brand');
        const stageTitle = el('span', 'rp-stage-title');
        const subTitle = el('span', 'rp-sub-title');
        brand.append(stageTitle, subTitle);

        const stats = el('div', 'rp-stats');
        const heartsPill = el('span', 'rp-pill rp-pill-hearts');
        const healBtn = el('button', 'rp-pill');
        healBtn.type = 'button';
        const coinsPill = el('span', 'rp-pill rp-pill-coins');
        const gemsPill = el('span', 'rp-pill rp-pill-gems');
        const keyPill = el('span', 'rp-pill rp-pill-key');
        const starsPill = el('span', 'rp-pill rp-pill-stars');
        const timePill = el('span', 'rp-pill rp-pill-time');
        const modPill = el('span', 'rp-pill rp-pill-mod');
        modPill.hidden = true;
        const runePill = el('span', 'rp-pill rp-pill-rune');
        runePill.hidden = true;
        const comboPill = el('span', 'rp-pill rp-pill-combo');
        comboPill.hidden = true;
        stats.append(heartsPill, healBtn, coinsPill, gemsPill, keyPill, starsPill, timePill, modPill, runePill, comboPill);

        const actions = el('div', 'rp-actions');
        const heroBtn = el('button', 'rp-btn');
        heroBtn.type = 'button';
        heroBtn.dataset.rpBtn = 'hero';
        const levelsBtn = el('button', 'rp-btn');
        levelsBtn.type = 'button';
        levelsBtn.dataset.rpBtn = 'levels';
        const shopBtn = el('button', 'rp-btn rp-btn-gold');
        shopBtn.type = 'button';
        shopBtn.dataset.rpBtn = 'shop';
        const hudSoundBtn = el('button', 'rp-btn');
        hudSoundBtn.type = 'button';
        hudSoundBtn.dataset.rpBtn = 'sound';
        hudSoundBtn.setAttribute('aria-pressed', 'true');
        const pauseBtn = el('button', 'rp-btn rp-btn-primary');
        pauseBtn.type = 'button';
        pauseBtn.dataset.rpBtn = 'pause';
        actions.append(heroBtn, levelsBtn, shopBtn, hudSoundBtn, pauseBtn);

        topbar.append(brand, stats, actions);

        const noticeRow = el('div', 'rp-notice-row');
        const noticeEl = el('p', 'rp-notice');
        noticeEl.setAttribute('role', 'status');
        const controlsHint = el('p', 'rp-controls-hint');
        noticeRow.append(noticeEl, controlsHint);

        const bossWrap = el('div', 'rp-boss-wrap');
        bossWrap.hidden = true;
        const bossLabel = el('div', 'rp-boss-label');
        const bossName = el('span');
        const bossHpText = el('span');
        bossLabel.append(bossName, bossHpText);
        const bossBar = el('div', 'rp-boss-bar');
        const bossFill = el('div', 'rp-boss-fill');
        bossBar.append(bossFill);
        bossWrap.append(bossLabel, bossBar);

        const bossCard = el('div', 'rp-bosscard');
        const bossCardName = el('strong');
        const bossCardSub = el('span');
        bossCard.setAttribute('aria-hidden', 'true');
        bossCard.append(bossCardName, bossCardSub);

        hud.append(topbar, noticeRow, bossWrap);

        // Touch controls (visible when .rp-narrow is active)
        const touch = el('div', 'rp-touch');
        const touchLeftGroup = el('div', 'rp-touch-group');
        const touchRightGroup = el('div', 'rp-touch-group');
        touch.append(touchLeftGroup, touchRightGroup);

        // Cards: Loading, Error, Win, Lose
        const loadingCard = el('div', 'rp-card rp-loading');
        const loadingText = el('p');
        loadingCard.append(loadingText);

        const errorCard = el('div', 'rp-card rp-error');
        errorCard.hidden = true;
        const errorText = el('p');
        const retryBtn = el('button', 'rp-btn-primary');
        retryBtn.type = 'button';
        errorCard.append(errorText, retryBtn);

        const winCard = el('div', 'rp-card rp-win');
        winCard.hidden = true;
        const winTitle = el('h2');
        const winStars = el('div', 'rp-stars-row');
        const winBody = el('p');
        const winRow = el('div', 'rp-row-actions');
        const winNextBtn = el('button', 'rp-btn-primary');
        winNextBtn.type = 'button';
        const winReplayBtn = el('button', 'rp-btn');
        winReplayBtn.type = 'button';
        const winShopBtn = el('button', 'rp-btn-gold');
        winShopBtn.type = 'button';
        winRow.append(winNextBtn, winReplayBtn, winShopBtn);
        winCard.append(winTitle, winStars, winBody, winRow);

        const loseCard = el('div', 'rp-card rp-lose');
        loseCard.hidden = true;
        const loseTitle = el('h2');
        const loseBody = el('p');
        const loseRetryBtn = el('button', 'rp-btn-primary');
        loseRetryBtn.type = 'button';
        loseCard.append(loseTitle, loseBody, loseRetryBtn);

        // Pause dialog
        const pauseDialog = el('dialog', 'rp-dialog rp-pause-dialog');
        const pauseHead = el('div', 'rp-dialog-head');
        const pauseTitle = el('h2');
        const pauseCloseX = el('button', 'rp-btn');
        pauseCloseX.type = 'button';
        pauseHead.append(pauseTitle, pauseCloseX);
        const pauseControls = el('p');
        const pauseActions = el('div', 'rp-row-actions');
        const resumeBtn = el('button', 'rp-btn-primary');
        resumeBtn.type = 'button';
        const restartBtn = el('button', 'rp-btn');
        restartBtn.type = 'button';
        const soundBtn = el('button', 'rp-btn');
        soundBtn.type = 'button';
        soundBtn.setAttribute('aria-pressed', 'false');
        const [musicBtn, gfxBtn] = [0, 1].map(function () {
            const b = el('button', 'rp-btn');
            b.type = 'button';
            b.hidden = true;
            b.setAttribute('aria-pressed', 'false');
            return b;
        });
        const resetBtn = el('button', 'rp-btn-danger');
        resetBtn.type = 'button';
        pauseActions.append(resumeBtn, restartBtn, soundBtn, musicBtn, gfxBtn, resetBtn);
        pauseDialog.append(pauseHead, pauseControls, pauseActions);

        // World Map / Level Select dialog
        const levelsDialog = el('dialog', 'rp-dialog rp-levels-dialog');
        const levelsHead = el('div', 'rp-dialog-head');
        const levelsTitle = el('h2');
        const levelsCloseBtn = el('button', 'rp-btn');
        levelsCloseBtn.type = 'button';
        levelsHead.append(levelsTitle, levelsCloseBtn);
        const worldTabs = el('div', 'rp-tabs');
        const levelsGrid = el('div', 'rp-level-grid');
        levelsDialog.append(levelsHead, worldTabs, levelsGrid);

        // Workshop & Hero dialog
        const shopDialog = el('dialog', 'rp-dialog rp-shop-dialog');
        const shopHead = el('div', 'rp-dialog-head');
        const shopTitle = el('h2');
        const shopCloseBtn = el('button', 'rp-btn');
        shopCloseBtn.type = 'button';
        shopHead.append(shopTitle, shopCloseBtn);
        const shopCoins = el('p');
        const heroGrid = el('div', 'rp-hero-grid');
        const shopGrid = el('div', 'rp-shop-grid');
        shopDialog.append(shopHead, shopCoins, heroGrid, shopGrid);

        shell.append(stageWrap, hud, bossCard, touch, loadingCard, errorCard, winCard, loseCard, pauseDialog, levelsDialog, shopDialog);
        host.replaceChildren(shell);

        function anyDialogOpen() {
            return pauseDialog.open || levelsDialog.open || shopDialog.open;
        }

        function isPaused() {
            const winActive = !ctx.isActive || ctx.isActive();
            return disposed || userPaused || !winActive || document.hidden || !loadingCard.hidden || !errorCard.hidden || anyDialogOpen() || !sim || sim.state !== 'playing';
        }

        function syncAudioActive() {
            const active = !isPaused();
            if (audio) audio.setActive(active);
            if (music && typeof music.setActive === 'function') music.setActive(active);
        }

        function paintMusic() {
            const on = profile.music !== false;
            musicBtn.hidden = !music;
            musicBtn.textContent = on ? t('runenpfad.pause_music_on') : t('runenpfad.pause_music_off');
            musicBtn.setAttribute('aria-pressed', on ? 'true' : 'false');
            shell.dataset.rpMusic = music && on ? '1' : '0';
        }

        function paintGfx() {
            const gpu = shell.dataset.rpGfx === 'gpu';
            gfxBtn.hidden = !postfx && profile.gfx !== 'canvas';
            gfxBtn.textContent = gpu ? t('runenpfad.pause_gfx_gpu') : t('runenpfad.pause_gfx_canvas');
            gfxBtn.setAttribute('aria-pressed', gpu ? 'true' : 'false');
        }

        // GPU post-processing is optional: every failure path ends in the plain canvas.
        function initPostfx(r, ticket) {
            const Postfx = root.RunenpfadPostfx;
            const toCanvas = function () { shell.dataset.rpGfx = 'canvas'; paintGfx(); };
            delete shell.dataset.rpGfxReason;
            if (!Postfx || typeof Postfx.create !== 'function' || profile.gfx === 'canvas') { toCanvas(); return; }
            let pending = null;
            try {
                pending = Postfx.create({
                    host: stageWrap,
                    source: canvas,
                    reducedMotion: !!((document.body && document.body.dataset.animations === 'false') || (root.matchMedia && root.matchMedia('(prefers-reduced-motion: reduce)').matches)),
                    onFallback: function (reason) {
                        shell.dataset.rpGfxReason = String(reason || 'fallback').slice(0, 80);
                        toCanvas();
                    }
                });
            } catch (err) { pending = null; }
            Promise.resolve(pending).then(function (p) {
                const live = !disposed && renderer === r && ticket === generation;
                if (p && live) {
                    postfx = p;
                    r.attachPostfx(p);
                    if (!shell.dataset.rpGfxReason) shell.dataset.rpGfx = 'gpu';
                } else {
                    if (p && typeof p.dispose === 'function') p.dispose();
                    if (live) shell.dataset.rpGfx = 'canvas';
                }
                if (!disposed) paintGfx();
            }).catch(function () { if (!disposed) toCanvas(); });
        }

        function disposeVisuals() {
            [renderer, postfx, music].forEach(function (m) {
                try { if (m && typeof m.dispose === 'function') m.dispose(); } catch (err) { /* ignore */ }
            });
            renderer = postfx = music = null;
        }

        // Mid-stage progress lives in the simulation's profile clone; adopt it without losing settings.
        function adoptSimProfile() {
            const keep = { sound: profile.sound, music: profile.music, gfx: profile.gfx };
            profile = Object.assign(Sim.exportProfile(sim.profile), keep);
            writeProfile();
        }

        function paintSound() {
            const on = !!(audio && audio.isEnabled());
            const label = on ? t('runenpfad.pause_sound_on') : t('runenpfad.pause_sound_off');
            soundBtn.textContent = label;
            soundBtn.setAttribute('aria-pressed', on ? 'true' : 'false');
            hudSoundBtn.textContent = label;
            hudSoundBtn.setAttribute('aria-pressed', on ? 'true' : 'false');
            shell.dataset.rpSound = on ? '1' : '0';
        }

        function paintTouchControls() {
            touchLeftGroup.replaceChildren();
            touchRightGroup.replaceChildren();
            [
                { group: touchLeftGroup, key: 'ArrowLeft', text: t('runenpfad.touch_left') },
                { group: touchLeftGroup, key: 'ArrowRight', text: t('runenpfad.touch_right') },
                { group: touchRightGroup, key: 'Space', text: t('runenpfad.touch_jump'), edge: 'jumpPressed' },
                { group: touchRightGroup, key: 'KeyJ', text: t('runenpfad.touch_attack'), edge: 'attackPressed' },
                { group: touchRightGroup, key: 'KeyK', text: t('runenpfad.touch_special'), edge: 'dashPressed' }
            ].forEach(function (spec) {
                const b = el('button', 'rp-touch-btn', spec.text);
                b.type = 'button';
                b.addEventListener('pointerdown', function (ev) {
                    ev.preventDefault();
                    if (audio) audio.unlock();
                    touchState[spec.key] = true;
                    if (spec.edge) edgeState[spec.edge] = true;
                });
                const release = function () { touchState[spec.key] = false; };
                b.addEventListener('pointerup', release);
                b.addEventListener('pointercancel', release);
                b.addEventListener('pointerleave', release);
                spec.group.append(b);
            });
        }

        function paintStaticCopy() {
            loadingText.textContent = t('runenpfad.loading');
            errorText.textContent = t('runenpfad.error_assets');
            retryBtn.textContent = t('runenpfad.error_retry');
            controlsHint.textContent = t('runenpfad.controls');
            levelsBtn.textContent = t('runenpfad.btn_levels');
            shopBtn.textContent = t('runenpfad.btn_shop');
            pauseBtn.textContent = t('runenpfad.btn_pause');
            pauseTitle.textContent = t('runenpfad.pause_title');
            pauseCloseX.textContent = t('runenpfad.close');
            pauseControls.textContent = t('runenpfad.controls');
            resumeBtn.textContent = t('runenpfad.pause_resume');
            restartBtn.textContent = t('runenpfad.pause_restart');
            resetBtn.textContent = resetArmed ? t('runenpfad.pause_reset_confirm') : t('runenpfad.pause_reset');
            levelsTitle.textContent = t('runenpfad.levels_title');
            levelsCloseBtn.textContent = t('runenpfad.close');
            shopTitle.textContent = t('runenpfad.shop_title');
            shopCloseBtn.textContent = t('runenpfad.close');
            winTitle.textContent = t('runenpfad.win_title');
            winNextBtn.textContent = t('runenpfad.win_next');
            winReplayBtn.textContent = t('runenpfad.win_replay');
            winShopBtn.textContent = t('runenpfad.btn_shop');
            loseTitle.textContent = t('runenpfad.lose_title');
            loseBody.textContent = t('runenpfad.lose_body');
            loseRetryBtn.textContent = t('runenpfad.lose_retry');
            paintSound();
            paintMusic();
            paintGfx();
            paintTouchControls();
        }

        function paintHud() {
            if (!sim) return;
            const stage = sim.stage;
            const p = sim.player;
            if (sim.mode === 'endless') {
                stageTitle.textContent = t('runenpfad.endless_title');
                subTitle.textContent = fill('runenpfad.endless_stage', { floor: sim.endlessFloor }) + ' · ' + t(stage.world.nameKey);
            } else {
                stageTitle.textContent = fill('runenpfad.hud_level', { world: stage.worldIndex + 1, stage: stage.stageInWorld, name: t(stage.nameKey) });
                subTitle.textContent = t(stage.world.nameKey);
            }
            heartsPill.textContent = '\u2665 ' + p.hp + '/' + p.maxHp;
            healBtn.textContent = t('runenpfad.hud_heal') + ' (' + p.potions + ')';
            healBtn.disabled = p.potions <= 0 || p.hp >= p.maxHp;
            coinsPill.textContent = t('runenpfad.hud_coins') + ': ' + sim.profile.coins;
            gemsPill.textContent = t('runenpfad.hud_gems') + ': ' + sim.gemCount + '/3';
            keyPill.textContent = sim.hasKey ? t('runenpfad.hud_key_on') : t('runenpfad.hud_key_off');
            keyPill.classList.toggle('is-on', sim.hasKey);
            starsPill.textContent = t('runenpfad.hud_stars') + ': ' + Sim.totalStars(profile) + '/75';
            timePill.textContent = Math.floor(sim.elapsed) + 's';

            // Rune trial badge & rune power countdown
            const mod = stage.mod;
            if (mod) {
                modPill.hidden = false;
                modPill.textContent = t('runenpfad.hud_trial') + ': ' + t(mod.nameKey);
                modPill.title = t(mod.descKey);
            } else {
                modPill.hidden = true;
                modPill.title = '';
            }
            if (sim.blessing && sim.blessing.runePower > 0) {
                runePill.hidden = false;
                runePill.textContent = t('runenpfad.hud_rune') + ' ' + Math.ceil(sim.blessing.runePower) + 's';
            } else {
                runePill.hidden = true;
            }
            const combo = sim.combo;
            comboPill.hidden = !(combo && combo.count >= 2);
            if (!comboPill.hidden) comboPill.textContent = fill('runenpfad.hud_combo', { n: String.fromCharCode(215) + (Math.round((combo.mul || 1) * 10) / 10) });
            const intro = sim.bossIntro;
            const introOn = !!(intro && intro.timer > 0);
            if (introOn) {
                bossCardName.textContent = t(intro.nameKey);
                bossCardSub.textContent = t('runenpfad.boss_intro_sub');
            }
            bossCard.classList.toggle('is-on', introOn);
            heroBtn.textContent = t('runenpfad.btn_hero') + ': ' + t(Levels.HERO_DEFS[sim.heroId].nameKey);
            noticeEl.textContent = sim.notice ? t(sim.notice) : '';

            // Boss bar update
            const boss = stage.bossId ? stage.enemies.find(function (e) { return e.id === stage.bossId; }) : null;
            if (boss && !boss.dead) {
                bossWrap.hidden = false;
                bossName.textContent = t(boss.def.bossNameKey || stage.nameKey);
                bossHpText.textContent = boss.hp + ' / ' + boss.maxHp;
                bossFill.style.width = Math.max(0, Math.min(100, Math.round((boss.hp / boss.maxHp) * 100))) + '%';
                if (audio) audio.setAmbienceDuck(0.55);
            } else {
                bossWrap.hidden = true;
                if (audio) audio.setAmbienceDuck(1);
            }

            // Win / Lose cards
            if (sim.state === 'won') {
                winCard.hidden = false;
                loseCard.hidden = true;
                winStars.textContent = '\u2605'.repeat(sim.earnedStars) + '\u2606'.repeat(3 - sim.earnedStars);
                winBody.textContent = fill('runenpfad.win_body', {
                    time: Math.floor(sim.elapsed),
                    gems: sim.gemCount,
                    score: sim.stageScore
                });
                winNextBtn.hidden = sim.mode === 'campaign' && sim.levelId >= Levels.CAMPAIGN_LEVELS.length;
            } else if (sim.state === 'lost') {
                winCard.hidden = true;
                loseCard.hidden = false;
            } else {
                winCard.hidden = true;
                loseCard.hidden = true;
            }

            // Deterministic DOM state markers for verification & accessibility inspection
            shell.dataset.rpLevel = String(sim.levelId);
            shell.dataset.rpMode = sim.mode;
            shell.dataset.rpHero = sim.heroId;
            shell.dataset.rpState = sim.state;
            shell.dataset.rpX = String(Math.round(p.x));
            shell.dataset.rpY = String(Math.round(p.y));
            shell.dataset.rpHp = String(p.hp);
            shell.dataset.rpMaxHp = String(p.maxHp);
            shell.dataset.rpCoins = String(sim.profile.coins);
            shell.dataset.rpGems = String(sim.gemCount);
            shell.dataset.rpKey = sim.hasKey ? '1' : '0';
            shell.dataset.rpStars = String(Sim.totalStars(profile));
            shell.dataset.rpUnlocked = String(profile.unlockedLevel);
        }

        function startStage(options) {
            options = options || {};
            userPaused = false;
            winCard.hidden = true;
            loseCard.hidden = true;
            sim = Sim.createSimulation({
                mode: options.mode || (sim && sim.mode) || 'campaign',
                levelId: options.levelId || (sim && sim.levelId) || Math.min(25, Math.max(1, profile.unlockedLevel)),
                endlessFloor: options.endlessFloor || (sim && sim.endlessFloor) || 1,
                hero: profile.hero,
                profile: profile
            });
            if (audio && audio.isEnabled()) {
                audio.setAmbience(sim.stage.ambience);
            }
            paintHud();
            syncAudioActive();
        }

        function renderLevelsModal() {
            worldTabs.replaceChildren();
            Levels.WORLDS.forEach(function (w, idx) {
                const tab = el('button', 'rp-tab' + (selectedWorld === idx ? ' is-active' : ''), (idx + 1) + '. ' + t(w.nameKey));
                tab.type = 'button';
                tab.addEventListener('click', function () {
                    selectedWorld = idx;
                    renderLevelsModal();
                });
                worldTabs.append(tab);
            });
            const endlessTab = el('button', 'rp-tab' + (selectedWorld === 5 ? ' is-active' : ''), t('runenpfad.endless_title'));
            endlessTab.type = 'button';
            endlessTab.addEventListener('click', function () {
                selectedWorld = 5;
                renderLevelsModal();
            });
            worldTabs.append(endlessTab);

            levelsGrid.replaceChildren();
            if (selectedWorld === 5) {
                const card = el('div', 'rp-shop-item');
                card.append(el('div', 'rp-shop-title', t('runenpfad.endless_title')));
                card.append(el('p', 'rp-shop-desc', fill('runenpfad.endless_best', { floor: profile.endlessBestFloor })));
                const startBtn = el('button', 'rp-btn-primary', t('runenpfad.endless_start'));
                startBtn.type = 'button';
                startBtn.addEventListener('click', function () {
                    levelsDialog.close();
                    startStage({ mode: 'endless', endlessFloor: 1 });
                });
                card.append(startBtn);
                levelsGrid.append(card);
                return;
            }

            const startIdx = selectedWorld * 5;
            Levels.CAMPAIGN_LEVELS.slice(startIdx, startIdx + 5).forEach(function (lvl) {
                const locked = lvl.id > profile.unlockedLevel;
                const stars = profile.starsByLevel[lvl.id] || 0;
                const isCurrent = sim && sim.mode === 'campaign' && sim.levelId === lvl.id;
                const btn = el('button', 'rp-level-tile' + (isCurrent ? ' is-current' : ''));
                btn.type = 'button';
                btn.disabled = locked;
                btn.dataset.rpLevelTile = String(lvl.id);
                btn.append(el('strong', '', (selectedWorld + 1) + '-' + lvl.stageInWorld + ' · ' + t(lvl.nameKey)));
                const starStr = '\u2605'.repeat(stars) + '\u2606'.repeat(3 - stars);
                const best = profile.bestTimeByLevel[lvl.id];
                btn.append(el('span', 'rp-level-meta', starStr + (best ? ' · ' + best + 's' : '')));
                btn.addEventListener('click', function () {
                    levelsDialog.close();
                    startStage({ mode: 'campaign', levelId: lvl.id });
                });
                levelsGrid.append(btn);
            });
        }

        function renderShopModal() {
            shopCoins.textContent = t('runenpfad.hud_coins') + ': ' + profile.coins + ' · ' + t('runenpfad.hud_stars') + ': ' + Sim.totalStars(profile);
            heroGrid.replaceChildren();
            ['knight', 'ranger'].forEach(function (heroId) {
                const def = Levels.HERO_DEFS[heroId];
                const card = el('div', 'rp-hero-card' + (profile.hero === heroId ? ' is-selected' : ''));
                card.append(el('div', 'rp-shop-title', t(def.nameKey)));
                card.append(el('p', 'rp-shop-desc', t(def.descKey)));
                const pick = el('button', profile.hero === heroId ? 'rp-btn-primary' : 'rp-btn', t(def.nameKey));
                pick.type = 'button';
                pick.dataset.rpPickHero = heroId;
                pick.addEventListener('click', function () {
                    profile.hero = heroId;
                    writeProfile();
                    if (sim) startStage({ mode: sim.mode, levelId: sim.levelId, endlessFloor: sim.endlessFloor });
                    renderShopModal();
                });
                card.append(pick);
                heroGrid.append(card);
            });

            shopGrid.replaceChildren();
            Levels.UPGRADE_DEFS.forEach(function (track) {
                const lvl = profile.upgrades[track.id] || 1;
                const cost = Sim.upgradeCost(track.id, lvl);
                const item = el('div', 'rp-shop-item');
                item.append(el('div', 'rp-shop-title', t(track.nameKey) + ' · ' + fill('runenpfad.shop_level', { level: lvl, max: track.maxLevel })));
                item.append(el('p', 'rp-shop-desc', t(track.descKey)));
                const buyBtn = el('button', 'rp-btn-gold', cost === null ? t('runenpfad.shop_max') : fill('runenpfad.shop_buy', { cost: cost }));
                buyBtn.type = 'button';
                buyBtn.dataset.rpBuyTrack = track.id;
                buyBtn.disabled = cost === null || profile.coins < cost;
                buyBtn.addEventListener('click', function () {
                    const res = Sim.buyUpgrade(profile, track.id);
                    if (res.ok) {
                        profile = res.profile;
                        writeProfile();
                        if (audio && audio.isEnabled()) audio.playUiClick();
                        if (sim && sim.state === 'playing') {
                            sim.profile = Sim.exportProfile(profile);
                            sim.stats = Sim.heroStats(sim.heroId, profile);
                            sim.player.maxHp = sim.stats.maxHp;
                            sim.player.hp = Math.min(sim.player.maxHp, sim.player.hp + 1);
                            sim.player.potions = Math.max(sim.player.potions, sim.stats.potions);
                        }
                        paintHud();
                        renderShopModal();
                    }
                });
                item.append(buyBtn);
                shopGrid.append(item);
            });
        }

        function readInput(consume) {
            const left = !!(keyState.ArrowLeft || keyState.KeyA || touchState.ArrowLeft);
            const right = !!(keyState.ArrowRight || keyState.KeyD || touchState.ArrowRight);
            const up = !!(keyState.ArrowUp || keyState.KeyW || touchState.ArrowUp);
            const down = !!(keyState.ArrowDown || keyState.KeyS || touchState.ArrowDown);
            const inp = {
                left: left,
                right: right,
                up: up,
                down: down,
                jumpPressed: edgeState.jumpPressed,
                attackPressed: edgeState.attackPressed,
                dashPressed: edgeState.dashPressed,
                healPressed: edgeState.healPressed
            };
            // Edges survive frames without a simulation substep (high refresh rates).
            if (consume) clearEdges();
            return inp;
        }

        function clearEdges() {
            edgeState.jumpPressed = false;
            edgeState.attackPressed = false;
            edgeState.dashPressed = false;
            edgeState.healPressed = false;
        }

        function resetInputs() {
            Object.keys(keyState).forEach(function (k) { keyState[k] = false; });
            Object.keys(touchState).forEach(function (k) { touchState[k] = false; });
            clearEdges();
        }

        function tick(nowMs) {
            if (disposed) return;
            rafId = root.requestAnimationFrame(tick);
            const dt = lastFrameMs ? Math.min(0.05, (nowMs - lastFrameMs) / 1000) : 1 / 60;
            lastFrameMs = nowMs;
            if (sim && !isPaused()) {
                const before = sim.profile;
                const prevCoins = before.coins;
                const prevUnlocked = before.unlockedLevel;
                const prevStars = Sim.totalStars(before);
                const prevState = sim.state;
                const slow = sim.fx && sim.fx.slowmo > 0;
                const simDt = dt * (slow ? 0.35 : 1);
                const willStep = sim.accum + Math.min(0.1, Math.max(0, simDt)) >= Sim.STEP;
                const events = Sim.stepSimulation(sim, simDt, readInput(willStep));
                if (slow) sim.fx.slowmo = Math.max(0, sim.fx.slowmo - dt);
                const camera = renderer ? renderer.getCamera() : null;
                if (renderer) renderer.handleEvents(events, sim);
                if (audio) audio.handleEvents(events, sim.stage.ambience, camera);
                const Music = root.RunenpfadMusic;
                if (music && Music && typeof music.update === 'function') music.update(Music.intensityFor(sim), sim);
                const after = sim.profile;
                if (after.coins !== prevCoins || after.unlockedLevel !== prevUnlocked || Sim.totalStars(after) !== prevStars || sim.state !== prevState) {
                    adoptSimProfile();
                }
                paintHud();
            } else if (sim && (document.hidden || (ctx.isActive && !ctx.isActive()))) {
                resetInputs();
            }
            if (renderer && sim) {
                renderer.render(sim, nowMs);
            }
        }

        // Controls wiring
        function onKeyDown(ev) {
            if (disposed) return;
            if (ctx.isActive && !ctx.isActive()) return;
            if (audio) audio.unlock();
            if (['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Space'].includes(ev.code)) {
                if (!anyDialogOpen()) ev.preventDefault();
            }
            if (!ev.repeat) {
                if (ev.code === 'Space' || ev.code === 'ArrowUp' || ev.code === 'KeyW') edgeState.jumpPressed = true;
                if (ev.code === 'KeyJ' || ev.code === 'KeyZ' || ev.code === 'Enter') {
                    if (!anyDialogOpen()) edgeState.attackPressed = true;
                }
                if (ev.code === 'KeyK' || ev.code === 'ShiftLeft' || ev.code === 'ShiftRight' || ev.code === 'KeyX') edgeState.dashPressed = true;
                if (ev.code === 'KeyQ' || ev.code === 'KeyH') edgeState.healPressed = true;
                if (ev.code === 'KeyP' || ev.code === 'Escape') {
                    if (anyDialogOpen()) {
                        if (pauseDialog.open) pauseDialog.close();
                        if (levelsDialog.open) levelsDialog.close();
                        if (shopDialog.open) shopDialog.close();
                        userPaused = false;
                        syncAudioActive();
                    } else if (!loadingCard.hidden || !errorCard.hidden) {
                        return;
                    } else {
                        userPaused = true;
                        resetArmed = false;
                        paintStaticCopy();
                        pauseDialog.showModal();
                        syncAudioActive();
                    }
                }
                if (ev.code === 'KeyR' && !anyDialogOpen() && sim) {
                    startStage({ mode: sim.mode, levelId: sim.levelId, endlessFloor: sim.endlessFloor });
                }
            }
            keyState[ev.code] = true;
        }

        function onKeyUp(ev) {
            keyState[ev.code] = false;
        }

        heroBtn.addEventListener('click', function () {
            profile.hero = profile.hero === 'knight' ? 'ranger' : 'knight';
            writeProfile();
            if (sim) startStage({ mode: sim.mode, levelId: sim.levelId, endlessFloor: sim.endlessFloor });
        });
        healBtn.addEventListener('click', function () {
            edgeState.healPressed = true;
        });
        levelsBtn.addEventListener('click', function () {
            renderLevelsModal();
            levelsDialog.showModal();
            syncAudioActive();
        });
        levelsCloseBtn.addEventListener('click', function () { levelsDialog.close(); syncAudioActive(); });
        shopBtn.addEventListener('click', function () {
            renderShopModal();
            shopDialog.showModal();
            syncAudioActive();
        });
        shopCloseBtn.addEventListener('click', function () { shopDialog.close(); syncAudioActive(); });
        pauseBtn.addEventListener('click', function () {
            userPaused = true;
            resetArmed = false;
            paintStaticCopy();
            pauseDialog.showModal();
            syncAudioActive();
        });
        function closePause() {
            if (pauseDialog.open) pauseDialog.close();
            userPaused = false;
            syncAudioActive();
        }
        pauseCloseX.addEventListener('click', closePause);
        resumeBtn.addEventListener('click', closePause);
        restartBtn.addEventListener('click', function () {
            closePause();
            startStage();
        });
        function toggleSound() {
            if (!audio) return;
            audio.setEnabled(!audio.isEnabled());
            profile.sound = audio.isEnabled();
            writeProfile();
            if (audio.isEnabled() && sim) audio.setAmbience(sim.stage.ambience);
            paintSound();
            syncAudioActive();
        }
        musicBtn.addEventListener('click', function () {
            if (!music) return;
            profile.music = profile.music === false;
            if (typeof music.setEnabled === 'function') music.setEnabled(profile.music);
            writeProfile();
            paintMusic();
        });
        gfxBtn.addEventListener('click', function () {
            profile.gfx = profile.gfx === 'canvas' ? 'auto' : 'canvas';
            writeProfile();
            const on = profile.gfx !== 'canvas';
            if (postfx && typeof postfx.setEnabled === 'function') postfx.setEnabled(on);
            if (on && !postfx && renderer) initPostfx(renderer, generation);
            else shell.dataset.rpGfx = on && postfx && !shell.dataset.rpGfxReason ? 'gpu' : 'canvas';
            paintGfx();
        });
        soundBtn.addEventListener('click', toggleSound);
        hudSoundBtn.addEventListener('click', toggleSound);
        resetBtn.addEventListener('click', function () {
            if (!resetArmed) {
                resetArmed = true;
                resetBtn.textContent = t('runenpfad.pause_reset_confirm');
                return;
            }
            resetArmed = false;
            profile = Sim.createDefaultProfile();
            writeProfile();
            selectedWorld = 0;
            closePause();
            startStage({ mode: 'campaign', levelId: 1 });
        });
        winNextBtn.addEventListener('click', function () {
            if (!sim) return;
            if (sim.mode === 'endless') startStage({ mode: 'endless', endlessFloor: sim.endlessFloor + 1 });
            else startStage({ mode: 'campaign', levelId: Math.min(Levels.CAMPAIGN_LEVELS.length, sim.levelId + 1) });
        });
        winReplayBtn.addEventListener('click', function () { startStage(); });
        winShopBtn.addEventListener('click', function () { renderShopModal(); shopDialog.showModal(); });
        loseRetryBtn.addEventListener('click', function () { startStage(); });
        retryBtn.addEventListener('click', function () { runBoot(); });

        [pauseDialog, levelsDialog, shopDialog].forEach(function (dlg) {
            dlg.addEventListener('close', function () {
                if (dlg === pauseDialog) userPaused = false;
                syncAudioActive();
                shell.focus({ preventScroll: true });
            }, { signal: controller.signal });
        });

        root.addEventListener('keydown', onKeyDown, { signal: controller.signal });
        root.addEventListener('keyup', onKeyUp, { signal: controller.signal });
        shell.addEventListener('pointerdown', function () { if (audio) audio.unlock(); }, { signal: controller.signal });
        document.addEventListener('visibilitychange', function () {
            if (document.hidden) resetInputs();
            syncAudioActive();
        }, { signal: controller.signal });
        root.addEventListener('blur', resetInputs, { signal: controller.signal });

        const ro = typeof ResizeObserver === 'function'
            ? new ResizeObserver(function () {
                shell.classList.toggle('rp-narrow', shell.clientWidth < 820);
            })
            : null;
        if (ro) ro.observe(shell);

        async function fetchDictionary() {
            async function loadLang(code) {
                const resp = await fetch('/lang/runenpfad/' + code + '.json?v=' + encodeURIComponent(root.BUILD_VERSION || 'dev'), {
                    signal: controller.signal, credentials: 'same-origin'
                });
                if (!resp.ok) throw new Error('locale ' + code);
                const text = await resp.text();
                if (text.length > 256 * 1024) throw new Error('locale size');
                return JSON.parse(text);
            }
            try {
                return await loadLang(locale);
            } catch (err) {
                if (locale !== 'en') return loadLang('en');
                throw err;
            }
        }

        async function fetchManifest(id, file) {
            const resp = await fetch(PACKS + id + '/' + file, { signal: controller.signal, credentials: 'same-origin', cache: 'no-store' });
            if (!resp.ok) throw new Error('asset manifest ' + id);
            return resp.json();
        }

        async function runBoot() {
            const ticket = ++generation;
            loadingCard.hidden = false;
            errorCard.hidden = true;
            winCard.hidden = true;
            loseCard.hidden = true;
            loadingText.textContent = '…';
            try {
                const loaded = await Promise.all([
                    fetchDictionary(),
                    fetchManifest('aurago-sounds', 'manifest.json')
                ].concat(Levels.REQUIRED_PACK_IDS.map(function (id) { return fetchManifest(id, 'sheet.json'); })));
                if (disposed || ticket !== generation) return;
                texts = loaded[0];
                const soundManifest = loaded[1];
                const manifestsById = {};
                Levels.REQUIRED_PACK_IDS.forEach(function (id, idx) {
                    manifestsById[id] = loaded[idx + 2];
                });
                const indexed = Levels.validateSpriteManifests(manifestsById);
                sheets = await View.loadSpriteSheets(indexed, controller.signal);
                if (disposed || ticket !== generation) return;
                if (audio) audio.dispose();
                audio = Audio.createAudioController({
                    manifest: soundManifest,
                    signal: controller.signal,
                    enabled: profile.sound !== false
                });
                disposeVisuals();
                renderer = View.createRenderer(canvas, sheets);
                const Music = root.RunenpfadMusic;
                try {
                    music = Music && Music.createMusicController(typeof audio.musicPort === 'function' ? audio.musicPort() : null);
                    if (music && typeof music.setEnabled === 'function') music.setEnabled(profile.music !== false);
                } catch (err) { music = null; }
                initPostfx(renderer, ticket);
                paintStaticCopy();
                loadingCard.hidden = true;
                shell.classList.toggle('rp-narrow', shell.clientWidth < 820);
                startStage({ mode: 'campaign', levelId: Math.min(25, Math.max(1, profile.unlockedLevel)) });
                if (!rafId) rafId = root.requestAnimationFrame(tick);
                shell.focus({ preventScroll: true });
            } catch (err) {
                if (disposed || ticket !== generation || (err && err.name === 'AbortError')) return;
                loadingCard.hidden = true;
                errorCard.hidden = false;
                errorText.textContent = t('runenpfad.error_assets');
                retryBtn.textContent = t('runenpfad.error_retry');
                shell.dataset.rpBootError = err && err.message ? String(err.message).slice(0, 300) : 'boot';
            }
        }

        // Expose deterministic state & stage inspection for tests
        shell.__runenpfad = {
            getSim: function () { return sim; },
            getProfile: function () { return profile; },
            startStage: startStage,
            setPlayerPosition: function (x, y) {
                if (!sim) return;
                sim.player.x = x;
                sim.player.y = y;
                sim.player.vx = 0;
                sim.player.vy = 0;
            }
        };

        sessions.set(windowId, {
            cleanup: function () {
                disposed = true;
                controller.abort();
                if (rafId) {
                    root.cancelAnimationFrame(rafId);
                    rafId = 0;
                }
                if (ro) ro.disconnect();
                disposeVisuals();
                if (audio) {
                    audio.dispose();
                    audio = null;
                }
                [pauseDialog, levelsDialog, shopDialog].forEach(function (d) {
                    if (d.open) d.close();
                });
            }
        });

        runBoot();
    }

    root.RunenpfadApp = {
        render: render,
        dispose: dispose
    };
})(typeof window !== 'undefined' ? window : globalThis);
