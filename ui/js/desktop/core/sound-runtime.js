(function () {
    'use strict';

    const EVENT_CATEGORY = {
        'window.open': 'windows', 'window.close': 'windows', 'window.minimize': 'windows',
        'window.restore': 'windows', 'window.maximize': 'windows', 'window.snap': 'windows', 'window.deny': 'windows',
        'notify.info': 'notifications', 'notify.message': 'notifications', 'notify.error': 'notifications',
        'menu.open': 'navigation', 'menu.close': 'navigation', 'space.switch': 'navigation',
        'dialog.open': 'files', 'dialog.confirm': 'files', 'dialog.cancel': 'files',
        'file.trash': 'files', 'file.delete': 'files', 'file.drop': 'files'
    };

    const THEME_IDS = ['crystal', 'wood', 'analog', 'workshop', 'water'];
    let actx = null;
    let master = null;
    let compressor = null;
    let categoryGains = {};
    let unlocked = false;
    let bundleLoaded = false;
    let bundleLoading = null;
    let cache = { theme: '', buffers: {}, stats: {} };
    let lastEventAt = {};
    let recentVoices = [];
    let notifyBurst = { at: 0, played: false };
    let stats = { plays: 0, renders: 0, previewPlays: 0, lastEvent: '', lastPeak: 0, lastRms: 0 };

    function soundsEnabled() {
        return settingValue('sound.enabled') === 'true';
    }

    function categoryEnabled(cat) {
        if (!cat) return true;
        const key = 'sound.' + cat;
        const val = settingValue(key);
        return val !== 'false';
    }

    function volumeGain() {
        const v = parseFloat(settingValue('sound.volume') || '0.6');
        const clamped = isFinite(v) ? Math.min(1, Math.max(0, v)) : 0.6;
        return clamped * clamped;
    }

    function currentThemeId() {
        const id = settingValue('sound.theme') || 'crystal';
        return THEME_IDS.indexOf(id) >= 0 ? id : 'crystal';
    }

    function ensureContext(force) {
        if (!force && !soundsEnabled() && !window.DesktopSoundsPreviewActive) return null;
        const Ctor = window.AudioContext || window.webkitAudioContext;
        if (!Ctor) return null;
        if (!actx) {
            try {
                actx = new Ctor();
            } catch (e) {
                actx = null;
                return null;
            }
            master = actx.createGain();
            compressor = actx.createDynamicsCompressor();
            compressor.threshold.value = -18;
            compressor.ratio.value = 3;
            master.connect(compressor);
            compressor.connect(actx.destination);
            ['windows', 'notifications', 'navigation', 'files'].forEach(cat => {
                categoryGains[cat] = actx.createGain();
                categoryGains[cat].gain.value = 1;
                categoryGains[cat].connect(master);
            });
        }
        syncDesktopSoundSettings();
        return actx;
    }

    function syncDesktopSoundSettings() {
        if (!master) return;
        master.gain.value = volumeGain();
        ['windows', 'notifications', 'navigation', 'files'].forEach(cat => {
            if (categoryGains[cat]) categoryGains[cat].gain.value = categoryEnabled(cat) ? 1 : 0;
        });
    }

    function setDesktopSoundVolume(value) {
        if (!master) {
            ensureContext(true);
        }
        syncDesktopSoundSettings();
        if (master && isFinite(value)) {
            const clamped = Math.min(1, Math.max(0, value));
            master.gain.value = clamped * clamped;
        }
    }

    async function closeSoundContext() {
        if (actx && typeof actx.close === 'function') {
            try { await actx.close(); } catch (e) {}
        }
        actx = null;
        master = null;
        compressor = null;
        categoryGains = {};
        unlocked = false;
        cache = { theme: '', buffers: {}, stats: {} };
    }

    function wireUnlock() {
        if (window.DesktopSoundsUnlockWired) return;
        window.DesktopSoundsUnlockWired = true;
        const unlock = () => {
            if (!soundsEnabled() && !window.DesktopSoundsPreviewActive) return;
            const ctx = ensureContext(true);
            if (!ctx) return;
            if (ctx.state === 'suspended') ctx.resume().catch(function () {});
            unlocked = true;
            document.removeEventListener('pointerdown', unlock, true);
            document.removeEventListener('keydown', unlock, true);
        };
        document.addEventListener('pointerdown', unlock, true);
        document.addEventListener('keydown', unlock, true);
    }

    function loadSoundBundle() {
        if (bundleLoaded) return Promise.resolve();
        if (bundleLoading) return bundleLoading;
        const loader = window.AuraDesktopModules && window.AuraDesktopModules.loadBundle;
        if (!loader) return Promise.reject(new Error('desktop sound bundle loader unavailable'));
        bundleLoading = loader('desktop-sounds').then(() => {
            bundleLoaded = true;
        }).catch(err => {
            bundleLoading = null;
            throw err;
        });
        return bundleLoading;
    }

    async function renderTheme(themeId) {
        await loadSoundBundle();
        const theme = (window.DesktopSoundThemes || {})[themeId];
        if (!theme) throw new Error('unknown sound theme: ' + themeId);
        const synth = window.DesktopSoundSynth;
        if (!synth) throw new Error('DesktopSoundSynth unavailable');
        const buffers = {};
        const meta = {};
        for (const eventId of synth.EVENTS) {
            const layers = (theme.events && (theme.events[eventId] || theme.events._default)) || [];
            const rendered = await synth.renderLayers(layers, synth.MAX_SECONDS);
            buffers[eventId] = rendered.buffer;
            meta[eventId] = rendered.stats;
            stats.renders++;
        }
        cache = { theme: themeId, buffers, stats: meta };
        return cache;
    }

    async function ensureBuffers(themeId) {
        if (cache.theme === themeId && cache.buffers && Object.keys(cache.buffers).length) return cache;
        return renderTheme(themeId);
    }

    function rateLimited(eventId) {
        const now = performance.now();
        if (eventId.startsWith('notify.')) {
            if (now - notifyBurst.at < 120) {
                if (notifyBurst.played) return true;
                notifyBurst.played = true;
            } else {
                notifyBurst = { at: now, played: false };
            }
        }
        const last = lastEventAt[eventId] || 0;
        if (now - last < 70) return true;
        lastEventAt[eventId] = now;
        recentVoices = recentVoices.filter(t => now - t < 250);
        if (recentVoices.length >= 6) return true;
        recentVoices.push(now);
        return false;
    }

    function playBuffer(buffer, category, options) {
        const ctx = ensureContext(true);
        if (!ctx || !buffer || !master) return false;
        if (ctx.state === 'suspended') ctx.resume().catch(function () {});
        const dest = categoryGains[category] || master;
        const src = ctx.createBufferSource();
        src.buffer = buffer;
        const jitterRate = 1 + ((Math.random() * 0.06) - 0.03);
        src.playbackRate.value = jitterRate;
        const g = ctx.createGain();
        const jitterGain = Math.pow(10, ((Math.random() * 3) - 1.5) / 20);
        g.gain.value = (options && options.gain != null ? options.gain : 1) * jitterGain;
        src.connect(g);
        g.connect(dest);
        src.start();
        stats.plays++;
        stats.lastEvent = options && options.event || '';
        return true;
    }

    async function desktopSound(eventId, options) {
        options = options || {};
        if (options.silent || document.hidden) return;
        if (options.sessionRestore) return;
        if (!soundsEnabled() && !options.preview) return;
        const category = EVENT_CATEGORY[eventId];
        if (!categoryEnabled(category)) return;
        if (!options.preview && !unlocked) {
            wireUnlock();
            return;
        }
        if (rateLimited(eventId) && !options.preview) return;
        try {
            const themeId = options.theme || currentThemeId();
            await ensureBuffers(themeId);
            const buffer = cache.buffers[eventId] || cache.buffers['notify.info'];
            if (!buffer) return;
            const meta = cache.stats[eventId] || {};
            stats.lastPeak = meta.peak || 0;
            stats.lastRms = meta.rms || 0;
            playBuffer(buffer, category, { event: eventId, gain: options.gain });
        } catch (e) {
            console.warn('Desktop sound failed', eventId, e);
        }
    }

    async function previewDesktopSound(themeId) {
        window.DesktopSoundsPreviewActive = true;
        try {
            ensureContext(true);
            if (actx && actx.state === 'suspended') actx.resume().catch(function () {});
            unlocked = true;
            await ensureBuffers(themeId || currentThemeId());
            await desktopSound('window.open', { preview: true, theme: themeId });
            await new Promise(r => setTimeout(r, 180));
            await desktopSound('notify.info', { preview: true, theme: themeId });
            await new Promise(r => setTimeout(r, 220));
            await desktopSound('window.close', { preview: true, theme: themeId });
            stats.previewPlays++;
        } finally {
            window.DesktopSoundsPreviewActive = false;
        }
    }

    function inspect() {
        return {
            enabled: soundsEnabled(),
            theme: currentThemeId(),
            volume: volumeGain(),
            unlocked,
            bundleLoaded,
            cachedTheme: cache.theme,
            renderedEvents: Object.keys(cache.buffers || {}).length,
            categories: {
                windows: categoryEnabled('windows'),
                notifications: categoryEnabled('notifications'),
                navigation: categoryEnabled('navigation'),
                files: categoryEnabled('files')
            },
            plays: stats.plays,
            renders: stats.renders,
            previewPlays: stats.previewPlays,
            lastEvent: stats.lastEvent,
            lastPeak: stats.lastPeak,
            lastRms: stats.lastRms,
            themes: THEME_IDS.slice()
        };
    }

    function applySoundSettingsChange(key, value) {
        if (key === 'sound.enabled' && value !== 'true') {
            closeSoundContext();
            return;
        }
        if (key === 'sound.enabled' && value === 'true') {
            wireUnlock();
        }
        if (key === 'sound.theme') {
            cache = { theme: '', buffers: {}, stats: {} };
            if (soundsEnabled() && unlocked) ensureBuffers(currentThemeId()).catch(function () {});
        }
        syncDesktopSoundSettings();
    }

    window.desktopSound = desktopSound;
    window.syncDesktopSoundSettings = syncDesktopSoundSettings;
    window.setDesktopSoundVolume = setDesktopSoundVolume;
    window.previewDesktopSound = previewDesktopSound;
    window.applySoundSettingsChange = applySoundSettingsChange;
    window.DesktopSounds = {
        play: desktopSound,
        preview: previewDesktopSound,
        inspect,
        renderTheme,
        close: closeSoundContext
    };

    const origApply = applyDesktopSettings;
    applyDesktopSettings = function () {
        origApply();
        syncDesktopSoundSettings();
    };
})();
