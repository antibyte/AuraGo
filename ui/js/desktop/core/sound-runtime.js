(function () {
    'use strict';

    const EVENT_CATEGORY = {
        'window.open': 'windows', 'window.close': 'windows', 'window.minimize': 'windows',
        'window.restore': 'windows', 'window.maximize': 'windows', 'window.snap': 'windows', 'window.deny': 'windows',
        'window.fracture': 'windows', 'window.impact': 'windows',
        'slime.pop': 'windows', 'slime.snap': 'windows', 'orbit.ping': 'navigation',
        'notify.info': 'notifications', 'notify.message': 'notifications', 'notify.error': 'notifications',
        'menu.open': 'navigation', 'menu.close': 'navigation', 'space.switch': 'navigation',
        'dialog.open': 'files', 'dialog.confirm': 'files', 'dialog.cancel': 'files',
        'file.trash': 'files', 'file.delete': 'files', 'file.drop': 'files'
    };

    const THEME_IDS = ['crystal', 'wood', 'analog', 'workshop', 'water', 'amiga', 'cyberpunk', 'black_gloss', 'crazy', 'lollipop', 'organic', 'electronic_dreams', 'windows_xp', 'green_slime', 'voxel_world', 'orbit'];
    // Theme FX are sounds that a theme effect asks for by name (a block landing, a bubble bursting, a sonar ping), not shell
    // events. Every family belongs to one sound theme and plays only while that theme is effective. Its sounds
    // are theme.specialFx[event][variant], rendered lazily one at a time, so a first use never renders the
    // whole bank. Calls within coalesceMs merge into the loudest; a coalesced call older than expireMs is
    // dropped; gapMs apart per family and per event, and at most maxInWindow plays per windowMs. The gain is
    // strength^curve times the event's gain, on top of the peak-normalised buffer (a window sound is 1).
    const THEME_FX = {
        voxel: {
            theme: 'voxel_world', option: 'material', variants: ['wood', 'stone', 'earth'], fallback: 'stone',
            events: { 'window.fracture': { gain: 0.52 }, 'window.impact': { gain: 0.52 } },
            coalesceMs: 75, expireMs: 240, gapMs: 190, windowMs: 1000, maxInWindow: 4, curve: 0.85, seconds: 0.7
        },
        slime: {
            theme: 'green_slime', option: 'variant', variants: ['small', 'medium', 'large'], fallback: 'medium',
            events: { 'slime.pop': { gain: 0.4, gapMs: 350 }, 'slime.snap': { gain: 0.42, gapMs: 900 } },
            coalesceMs: 80, expireMs: 240, gapMs: 0, windowMs: 2000, maxInWindow: 3, curve: 0.85, seconds: 0.3
        },
        orbit: {
            theme: 'orbit', option: 'variant', variants: ['small', 'medium', 'large'], fallback: 'medium',
            events: { 'orbit.ping': { gain: 0.38, gapMs: 350 } },
            coalesceMs: 80, expireMs: 240, gapMs: 0, windowMs: 2000, maxInWindow: 3, curve: 0.85, seconds: 1.0
        }
    };
    const THEME_FX_OF = {};
    const themeFxState = {};
    Object.keys(THEME_FX).forEach(family => {
        Object.keys(THEME_FX[family].events).forEach(eventId => { THEME_FX_OF[eventId] = family; });
        themeFxState[family] = { timer: null, queue: new Map(), recent: [], lastAt: {}, requested: 0, played: 0, limited: 0, expired: 0 };
    });
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
    let soundEpoch = 0;
    let previewEpoch = 0;
    let themeFxBuffers = { theme: '', buffers: {} };
    const pendingThemeFxRenders = new Map();
    let observedSettings = '';
    const pendingRenders = new Map();
    const voices = new Set();
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

    function resolveThemeId(id) {
        if (!id || id === 'desktop') id = currentDesktopTheme().soundTheme;
        return THEME_IDS.indexOf(id) >= 0 ? id : 'crystal';
    }

    function currentThemeId() {
        return resolveThemeId(settingValue('sound.theme'));
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
        const signature = [soundsEnabled(), currentThemeId(), ...['windows', 'notifications', 'navigation', 'files'].map(categoryEnabled)].join('|');
        if (observedSettings && signature !== observedSettings) {
            soundEpoch++;
            previewEpoch++;
            window.DesktopSoundsPreviewActive = false;
            stopVoices();
            cancelThemeFx();
            invalidateThemeFxBuffers();
            if (!soundsEnabled()) closeSoundContext();
        }
        observedSettings = signature;
        if (soundsEnabled() && !unlocked) wireUnlock();
        if (!master) return;
        master.gain.value = volumeGain();
        ['windows', 'notifications', 'navigation', 'files'].forEach(cat => {
            if (categoryGains[cat]) categoryGains[cat].gain.value = categoryEnabled(cat) ? 1 : 0;
        });
    }

    function setDesktopSoundVolume(value) {
        syncDesktopSoundSettings();
        if (master && isFinite(value)) {
            const clamped = Math.min(1, Math.max(0, value));
            master.gain.value = clamped * clamped;
        }
    }

    function stopVoices() {
        voices.forEach(src => { try { src.stop(); } catch (e) {} });
        voices.clear();
    }

    function cancelThemeFx() {
        Object.values(themeFxState).forEach(fx => {
            if (fx.timer !== null) clearTimeout(fx.timer);
            fx.timer = null;
            fx.queue.clear();
            fx.recent = [];
            fx.lastAt = {};
        });
    }

    function invalidateThemeFxBuffers() {
        themeFxBuffers = { theme: '', buffers: {} };
        pendingThemeFxRenders.clear();
    }

    async function closeSoundContext() {
        soundEpoch++;
        previewEpoch++;
        window.DesktopSoundsPreviewActive = false;
        cancelThemeFx();
        invalidateThemeFxBuffers();
        stopVoices();
        const closing = actx;
        if (master) master.gain.value = 0;
        actx = null;
        master = null;
        compressor = null;
        categoryGains = {};
        unlocked = false;
        cache = { theme: '', buffers: {}, stats: {} };
        if (closing && typeof closing.close === 'function') {
            try { await closing.close(); } catch (e) {}
        }
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
            window.DesktopSoundsUnlockWired = false;
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
        if (pendingRenders.has(themeId)) return pendingRenders.get(themeId);
        const pending = renderTheme(themeId).finally(() => pendingRenders.delete(themeId));
        pendingRenders.set(themeId, pending);
        return pending;
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

    // The request of a theme FX call in its cleaned form: the variant (the family's option, e.g. material or
    // size), a strength of 0..1 (0.5 when absent) and a pan of -1..1.
    function cleanThemeFxOptions(fx, options) {
        const rawStrength = options && Number(options.strength);
        const rawPan = options && Number(options.pan);
        const strength = Number.isFinite(rawStrength) ? Math.min(1, Math.max(0, rawStrength)) : 0.5;
        const variant = options && options[fx.option];
        return {
            variant: fx.variants.indexOf(variant) >= 0 ? variant : fx.fallback,
            strength,
            pan: Number.isFinite(rawPan) ? Math.min(1, Math.max(-1, rawPan)) : 0
        };
    }

    function queueThemeFx(eventId, family, options) {
        const fx = THEME_FX[family];
        const fxState = themeFxState[family];
        if (options.preview || options.silent || document.hidden || !soundsEnabled() || !unlocked ||
            !categoryEnabled(EVENT_CATEGORY[eventId]) || currentThemeId() !== fx.theme ||
            (typeof state !== 'undefined' && state.sessionRestoring)) return false;
        const next = cleanThemeFxOptions(fx, options);
        if (next.strength <= 0) return false;
        next.expiresAt = performance.now() + fx.expireMs;
        const previous = fxState.queue.get(eventId);
        if (previous) {
            const total = previous.strength + next.strength;
            const stronger = next.strength >= previous.strength ? next : previous;
            previous.pan = total ? ((previous.pan * previous.strength) + (next.pan * next.strength)) / total : 0;
            previous.strength = Math.max(previous.strength, next.strength);
            previous.variant = stronger.variant;
        } else fxState.queue.set(eventId, next);
        fxState.requested++;
        if (fxState.timer === null) {
            fxState.timer = setTimeout(() => {
                const queued = Array.from(fxState.queue.entries());
                fxState.queue.clear();
                fxState.timer = null;
                queued.forEach(([queuedEvent, queuedOptions]) => {
                    desktopSound(queuedEvent, Object.assign({ [fx.option]: queuedOptions.variant }, queuedOptions, { fxCoalesced: true }));
                });
            }, fx.coalesceMs);
        }
        return true;
    }

    async function ensureThemeFxBuffer(fx, themeId, eventId, variant, epoch) {
        const theme = (window.DesktopSoundThemes || {})[themeId];
        const layers = theme && theme.specialFx && theme.specialFx[eventId] && theme.specialFx[eventId][variant];
        if (!Array.isArray(layers) || !layers.length) return null;
        if (themeFxBuffers.theme !== themeId) themeFxBuffers = { theme: themeId, buffers: {} };
        const cacheKey = eventId + ':' + variant;
        if (themeFxBuffers.buffers[cacheKey]) return themeFxBuffers.buffers[cacheKey];
        const pendingKey = [epoch, themeId, cacheKey].join(':');
        if (pendingThemeFxRenders.has(pendingKey)) return pendingThemeFxRenders.get(pendingKey);
        const synth = window.DesktopSoundSynth;
        const pending = synth.renderLayers(layers, fx.seconds).then(rendered => {
            if (epoch !== soundEpoch || document.hidden || currentThemeId() !== themeId) return null;
            if (themeFxBuffers.theme !== themeId) themeFxBuffers = { theme: themeId, buffers: {} };
            themeFxBuffers.buffers[cacheKey] = rendered.buffer;
            stats.renders++;
            return rendered.buffer;
        }).finally(() => pendingThemeFxRenders.delete(pendingKey));
        pendingThemeFxRenders.set(pendingKey, pending);
        return pending;
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
        let panner = null;
        if (options && Number.isFinite(options.pan) && typeof ctx.createStereoPanner === 'function') {
            panner = ctx.createStereoPanner();
            panner.pan.value = Math.min(1, Math.max(-1, options.pan));
            g.connect(panner);
            panner.connect(dest);
        } else g.connect(dest);
        voices.add(src);
        src.onended = () => { voices.delete(src); src.disconnect(); g.disconnect(); if (panner) panner.disconnect(); };
        src.start();
        stats.plays++;
        stats.lastEvent = options && options.event || '';
        return true;
    }

    async function desktopSound(eventId, options) {
        options = options || {};
        const family = THEME_FX_OF[eventId];
        if (family && !options.fxCoalesced) return queueThemeFx(eventId, family, options);
        if (options.silent || document.hidden) return;
        if (options.sessionRestore || (typeof state !== 'undefined' && state.sessionRestoring)) return;
        if (!soundsEnabled() && !options.preview) return;
        const category = EVENT_CATEGORY[eventId];
        if (!categoryEnabled(category)) return;
        if (!options.preview && !unlocked) {
            wireUnlock();
            return;
        }
        if (!options.preview && rateLimited(eventId)) return;
        const epoch = soundEpoch;
        try {
            const themeId = options.theme ? resolveThemeId(options.theme) : currentThemeId();
            const fx = family ? THEME_FX[family] : null;
            const fxState = family ? themeFxState[family] : null;
            if (fx && (options.preview || themeId !== fx.theme)) return false;
            const fxOptions = fx ? cleanThemeFxOptions(fx, options) : null;
            if (fx && fxOptions.strength <= 0) return false;
            // FX has its own small variant bank. Avoid rendering all 19 ordinary
            // events just to play a single fracture, impact, pop or snap on first use.
            if (fx) await loadSoundBundle();
            const rendered = fx ? null : await ensureBuffers(themeId);
            if (epoch !== soundEpoch || document.hidden || (typeof state !== 'undefined' && state.sessionRestoring)) return;
            if (!categoryEnabled(category)) return;
            if (options.preview) {
                if (options.previewToken !== previewEpoch || !window.DesktopSoundsPreviewActive) return;
            } else if (!soundsEnabled() || !unlocked || themeId !== currentThemeId()) return;
            const buffer = fx
                ? await ensureThemeFxBuffer(fx, themeId, eventId, fxOptions.variant, epoch)
                : (rendered.buffers[eventId] || rendered.buffers['notify.info']);
            if (epoch !== soundEpoch || !buffer || document.hidden || (!options.preview && currentThemeId() !== themeId)) return;
            if (!buffer) return;
            if (fx) {
                const now = performance.now();
                if (!Number.isFinite(options.expiresAt) || now > options.expiresAt) {
                    fxState.expired++;
                    return false;
                }
                fxState.recent = fxState.recent.filter(at => now - at < fx.windowMs);
                const eventGap = fx.events[eventId].gapMs || 0;
                if (fxState.recent.length >= fx.maxInWindow || now - (fxState.recent.at(-1) ?? -Infinity) < fx.gapMs ||
                    now - (fxState.lastAt[eventId] ?? -Infinity) < eventGap) {
                    fxState.limited++;
                    return false;
                }
                fxState.recent.push(now);
                fxState.lastAt[eventId] = now;
            }
            const meta = rendered && rendered.stats[eventId] || {};
            stats.lastPeak = meta.peak || 0;
            stats.lastRms = meta.rms || 0;
            const gain = fx ? Math.pow(fxOptions.strength, fx.curve) * fx.events[eventId].gain : options.gain;
            playBuffer(buffer, category, { event: eventId, gain, pan: fx ? fxOptions.pan : undefined });
            if (fx) fxState.played++;
            return true;
        } catch (e) {
            console.warn('Desktop sound failed', eventId, e);
            return false;
        }
    }

    async function previewDesktopSound(themeId) {
        syncDesktopSoundSettings();
        const token = ++previewEpoch;
        soundEpoch++;
        cancelThemeFx();
        invalidateThemeFxBuffers();
        stopVoices();
        window.DesktopSoundsPreviewActive = true;
        try {
            ensureContext(true);
            if (actx && actx.state === 'suspended') actx.resume().catch(function () {});
            if (!actx) throw new Error('Web Audio unavailable');
            unlocked = true;
            const resolved = resolveThemeId(themeId || currentThemeId());
            await ensureBuffers(resolved);
            for (const event of ['window.open', 'notify.info', 'window.close']) {
                if (token !== previewEpoch || document.hidden) return;
                await desktopSound(event, { preview: true, theme: resolved, previewToken: token });
                await new Promise(r => setTimeout(r, 420));
            }
            if (token === previewEpoch) stats.previewPlays++;
        } finally {
            if (token === previewEpoch) {
                window.DesktopSoundsPreviewActive = false;
                if (!soundsEnabled()) await closeSoundContext();
            }
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
            // Per theme FX family: calls accepted into the coalescing queue, plays, calls dropped by the
            // rate limits or for age, what waits in the queue, the rendered variants and the settings.
            themeFx: Object.fromEntries(Object.keys(THEME_FX).map(family => {
                const fx = THEME_FX[family];
                const fxState = themeFxState[family];
                return [family, {
                    theme: fx.theme,
                    requested: fxState.requested,
                    played: fxState.played,
                    limited: fxState.limited,
                    expired: fxState.expired,
                    queued: fxState.queue.size,
                    rendered: themeFxBuffers.theme === fx.theme ? Object.keys(themeFxBuffers.buffers).length : 0,
                    seconds: fx.seconds,
                    gains: Object.fromEntries(Object.keys(fx.events).map(eventId => [eventId, fx.events[eventId].gain]))
                }];
            })),
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
        syncDesktopSoundSettings();
        if (key === 'sound.enabled' && value !== 'true') closeSoundContext();
    }

    document.addEventListener('visibilitychange', () => {
        if (document.hidden) {
            soundEpoch++;
            previewEpoch++;
            window.DesktopSoundsPreviewActive = false;
            cancelThemeFx();
            invalidateThemeFxBuffers();
            stopVoices();
            if (!soundsEnabled()) closeSoundContext();
        }
    });
    window.addEventListener('pagehide', () => closeSoundContext());
    window.addEventListener('aurago:auth-ended', () => closeSoundContext());

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
