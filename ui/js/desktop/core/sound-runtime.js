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
        'file.trash': 'files', 'file.delete': 'files', 'file.drop': 'files',
        'session.start': 'startup', 'notify.success': 'notifications', 'notify.warning': 'notifications',
        'ui.select': 'interface', 'ui.toggle.on': 'interface', 'ui.toggle.off': 'interface', 'ui.tick': 'interface'
    };
    const CATEGORIES = ['windows', 'notifications', 'navigation', 'files', 'interface', 'startup'];
    // Variants per shell event, in spec order (mirrors DesktopSoundEngine.ROLES; ui/desktop_sound_settings_test.go checks both).
    const EVENT_VARIANTS = {
        'session.start': 1, 'notify.message': 1, 'notify.info': 1, 'notify.success': 1, 'notify.warning': 1, 'notify.error': 1,
        'dialog.open': 1, 'dialog.confirm': 1, 'dialog.cancel': 1, 'window.open': 3, 'window.close': 3, 'window.maximize': 3,
        'window.minimize': 3, 'window.restore': 3, 'window.snap': 3, 'window.deny': 2, 'file.drop': 3, 'file.trash': 2,
        'file.delete': 2, 'space.switch': 2, 'menu.open': 3, 'menu.close': 3, 'ui.select': 3, 'ui.toggle.on': 3, 'ui.toggle.off': 3, 'ui.tick': 3
    };
    const PREWARM_ORDER = ['window.open', 'window.close', 'menu.open', 'menu.close', 'window.minimize', 'window.restore',
        'window.maximize', 'window.snap', 'window.deny', 'ui.select', 'ui.toggle.on', 'ui.toggle.off', 'ui.tick', 'space.switch',
        'dialog.open', 'dialog.confirm', 'dialog.cancel', 'file.drop', 'file.trash', 'file.delete',
        'notify.info', 'notify.message', 'notify.success', 'notify.warning', 'notify.error', 'session.start'];
    const WORKER_PATH = '/js/desktop/bundles/desktop-sounds.bundle.js';
    const FIRST_REPLY_MS = 4000, START_WAIT_MS = 1500, FEEDBACK_GAP_MS = 45;
    // A sound that arrives later than this after its event is dropped instead of played out of sync.
    // Notices and dialogs tolerate a cold cache; window, menu and touch sounds must stay tight.
    const LATE_MS = { notifications: 800, files: 400, windows: 200, navigation: 150, interface: 120 };

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
    let lastEventAt = {};
    let recentVoices = [];
    let notifyBurst = { at: 0, played: false };
    let soundEpoch = 0;
    let previewEpoch = 0;
    let themeFxBuffers = { theme: '', buffers: {} };
    const pendingThemeFxRenders = new Map();
    let observedSettings = '';
    let observedTheme = '';
    const voices = new Set();
    let stats = { plays: 0, renders: 0, previewPlays: 0, dropped: 0, lastEvent: '', lastLufs: 0, lastPeakDb: 0 };

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
            // Safety limiter: mastered sounds peak at or below -3 dBTP and pass untouched; only overlaps are caught.
            compressor.threshold.value = -2;
            compressor.knee.value = 0;
            compressor.ratio.value = 12;
            compressor.attack.value = 0.003;
            compressor.release.value = 0.12;
            master.connect(compressor);
            compressor.connect(actx.destination);
            CATEGORIES.forEach(cat => {
                categoryGains[cat] = actx.createGain();
                categoryGains[cat].gain.value = 1;
                categoryGains[cat].connect(master);
            });
        }
        syncDesktopSoundSettings();
        return actx;
    }

    function syncDesktopSoundSettings() {
        const signature = [soundsEnabled(), currentThemeId(), ...CATEGORIES.map(categoryEnabled)].join('|');
        const themeNow = currentThemeId();
        if (observedSettings && signature !== observedSettings) {
            soundEpoch++;
            previewEpoch++;
            window.DesktopSoundsPreviewActive = false;
            stopVoices();
            cancelThemeFx();
            invalidateThemeFxBuffers();
            // Another theme: the cached sounds and the queued renders belong to the old one (a category switch keeps them).
            if (themeNow !== observedTheme) dropRenderCache();
            if (!soundsEnabled()) closeSoundContext();
        }
        observedSettings = signature;
        observedTheme = themeNow;
        if (soundsEnabled() && unlocked) prewarm(currentThemeId());
        if (soundsEnabled() && !unlocked) wireUnlock();
        if (!master) return;
        master.gain.value = volumeGain();
        CATEGORIES.forEach(cat => {
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
        dropRenderCache();
        const closing = actx;
        if (master) master.gain.value = 0;
        actx = null;
        master = null;
        compressor = null;
        categoryGains = {};
        unlocked = false;
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
            prewarm(currentThemeId());
            document.removeEventListener('pointerdown', unlock, true);
            document.removeEventListener('keydown', unlock, true);
            window.DesktopSoundsUnlockWired = false;
        };
        document.addEventListener('pointerdown', unlock, true);
        document.addEventListener('keydown', unlock, true);
    }

    // The sound bundle on the main thread: only the fallback renderer needs it (the Worker loads the same file itself).
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

    let renderer = '';             // '' until a sound renders, then 'worker' or 'main'
    let worker = null, workerAnswered = false, workerBroken = false, firstReplyTimer = null, requestSeq = 0;
    const renderQueue = [];        // waiting jobs; urgent ones are put in front
    const renderJobs = new Map();  // key -> job (queued or running), so one sound renders once
    let running = null;            // at most one job in flight: queue order and queue jumping stay ours
    let cache = { theme: '', entries: {} };   // entries[event + '#' + variant] = { result, buffer }
    let prewarmedTheme = '';
    const warned = new Set();
    function warnOnce(key, ...args) { if (!warned.has(key)) { warned.add(key); console.warn(...args); } }

    function requestRender(theme, event, variant, urgent) {
        const key = theme + '|' + event + '|' + variant;
        const existing = renderJobs.get(key);
        if (existing) {
            const at = renderQueue.indexOf(existing);
            if (urgent && at > 0) { renderQueue.splice(at, 1); renderQueue.unshift(existing); }
            return existing.promise;
        }
        const job = { key, theme, event, variant, id: 0 };
        job.promise = new Promise((resolve, reject) => { job.resolve = resolve; job.reject = reject; });
        renderJobs.set(key, job);
        if (urgent) renderQueue.unshift(job); else renderQueue.push(job);
        pumpRenders();
        return job.promise;
    }

    function finishJob(job, error, result) {
        if (renderJobs.get(job.key) === job) renderJobs.delete(job.key);
        if (running === job) running = null;
        // Test seam: a promise in window.DesktopSoundsRenderGate holds every reply until it settles.
        Promise.resolve(window.DesktopSoundsRenderGate).catch(() => {}).then(() => { if (error) job.reject(error); else job.resolve(result); });
        setTimeout(pumpRenders, 0);   // one sound per task
    }

    function pumpRenders() {
        if (running || !renderQueue.length) return;
        const job = renderQueue.shift();
        running = job;
        if (!workerBroken && ensureWorker()) {
            job.id = ++requestSeq;
            if (!workerAnswered && firstReplyTimer === null) firstReplyTimer = setTimeout(() => workerFailed(new Error('sound worker did not answer within 4 s')), FIRST_REPLY_MS);
            try {
                worker.postMessage({ id: job.id, theme: job.theme, event: job.event, variant: job.variant });
            } catch (e) { workerFailed(e); }
        } else renderOnMainThread(job);
    }

    function ensureWorker() {
        if (worker) return true;
        if (typeof Worker !== 'function') { workerBroken = true; return false; }
        try {
            const lazy = window.AuraLazyAssets;
            worker = new Worker(lazy && typeof lazy.versionedURL === 'function' ? lazy.versionedURL(WORKER_PATH) : WORKER_PATH);
        } catch (e) { worker = null; workerBroken = true; return false; }
        worker.onmessage = onWorkerMessage;
        worker.onerror = event => { if (event && event.preventDefault) event.preventDefault(); workerFailed(new Error('sound worker error')); };
        worker.onmessageerror = () => workerFailed(new Error('sound worker message error'));
        return true;
    }

    function onWorkerMessage(event) {
        const msg = event.data || {}, job = running;
        if (!job || msg.id !== job.id) return;   // reply for a job dropped by stopRenders
        if (firstReplyTimer !== null) { clearTimeout(firstReplyTimer); firstReplyTimer = null; }
        workerAnswered = true;
        renderer = 'worker';
        if (msg.ok) finishJob(job, null, { L: msg.L, R: msg.R, sampleRate: msg.sampleRate, seconds: msg.seconds, lufs: msg.lufs, peakDb: msg.peakDb });
        else finishJob(job, new Error(msg.error || 'sound render failed'));
    }

    function workerFailed(error) {
        if (firstReplyTimer !== null) { clearTimeout(firstReplyTimer); firstReplyTimer = null; }
        warnOnce('worker', 'Desktop sound worker unavailable; rendering on the main thread', error);
        workerBroken = true;   // stays on the main thread for this page load
        if (worker) { try { worker.terminate(); } catch (e) {} worker = null; }
        const job = running;
        running = null;
        if (job) renderQueue.unshift(job);
        pumpRenders();
    }

    function renderOnMainThread(job) {
        loadSoundBundle().then(() => new Promise(resolve => setTimeout(resolve, 0))).then(() => {
            if (running !== job) return;
            const engine = window.DesktopSoundEngine;
            if (!engine) throw new Error('DesktopSoundEngine unavailable');
            const out = engine.render(job.theme, job.event, job.variant);
            renderer = 'main';
            finishJob(job, null, out);
        }).catch(err => { if (running === job) finishJob(job, err); });
    }

    function stopRenders() {   // sounds off, pagehide, auth end: drop the queue and terminate the worker
        if (firstReplyTimer !== null) { clearTimeout(firstReplyTimer); firstReplyTimer = null; }
        const dropped = renderQueue.splice(0);
        if (running) dropped.push(running);
        running = null;
        renderJobs.clear();
        dropped.forEach(job => job.resolve(null));
        if (worker) { try { worker.terminate(); } catch (e) {} worker = null; }
        workerAnswered = false;
    }

    const lastVariant = {};
    let cacheGen = 0;   // bumped when the cache is dropped, so a render that finishes afterwards is not cached
    // Drops everything rendered or queued: sounds went off, or the sound theme changed.
    function dropRenderCache() {
        cacheGen++;
        stopRenders();
        prewarmedTheme = '';
        cache = { theme: '', entries: {} };
        Object.keys(lastVariant).forEach(k => delete lastVariant[k]);
    }

    function ensureEventBuffer(themeId, eventId, variant, urgent) {
        const entryKey = eventId + '#' + variant, gen = cacheGen;
        if (cache.theme === themeId && cache.entries[entryKey]) return Promise.resolve(cache.entries[entryKey]);
        return requestRender(themeId, eventId, variant, urgent).then(result => {
            if (!result || gen !== cacheGen) return null;
            stats.renders++;
            const entry = { result, buffer: null };
            if (cache.theme !== themeId) cache = { theme: themeId, entries: {} };   // only one theme is cached
            cache.entries[entryKey] = entry;
            return entry;
        });
    }

    function audioBufferFor(entry) {
        if (entry.buffer) return entry.buffer;
        if (!actx || !entry.result || !entry.result.L) return null;
        const r = entry.result;
        const buffer = actx.createBuffer(2, r.L.length, r.sampleRate);
        buffer.copyToChannel(r.L, 0);
        buffer.copyToChannel(r.R, 1);
        entry.buffer = buffer;
        entry.result = { sampleRate: r.sampleRate, seconds: r.seconds, lufs: r.lufs, peakDb: r.peakDb, length: r.L.length };
        return buffer;
    }

    function prewarm(themeId) {   // variant 0 of every event in priority order, then the other variants
        if (prewarmedTheme === themeId) return;
        prewarmedTheme = themeId;
        for (let i = renderQueue.length - 1; i >= 0; i--) {
            const job = renderQueue[i];
            if (job.theme !== themeId) { renderQueue.splice(i, 1); renderJobs.delete(job.key); job.resolve(null); }
        }
        const quiet = p => p.catch(err => warnOnce('prewarm:' + themeId, 'Desktop sound prewarm failed', err));
        PREWARM_ORDER.forEach(eventId => quiet(ensureEventBuffer(themeId, eventId, 0, false)));
        PREWARM_ORDER.forEach(eventId => { for (let v = 1; v < EVENT_VARIANTS[eventId]; v++) quiet(ensureEventBuffer(themeId, eventId, v, false)); });
    }

    // Round-robin without repeating the previous variant; prefers a variant that is already rendered.
    function chooseVariant(themeId, eventId) {
        const count = EVENT_VARIANTS[eventId] || 1;
        if (count <= 1) return 0;
        const prev = lastVariant[eventId];
        const start = prev == null ? 0 : (prev + 1) % count;
        for (let k = 0; k < count; k++) {
            const v = (start + k) % count;
            if (v !== prev && cache.theme === themeId && cache.entries[eventId + '#' + v]) { lastVariant[eventId] = v; return v; }
        }
        lastVariant[eventId] = start;
        return start;
    }

    function rateLimited(eventId) {
        const now = performance.now();
        if (eventId.startsWith('ui.')) {
            if (now - (lastEventAt[eventId] || 0) < 40) return true;
            lastEventAt[eventId] = now;
            return false;
        }
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

    // One theme FX sound (event + family variant). Rendered on demand; the result is dropped when the settings moved on.
    async function ensureThemeFxBuffer(fx, themeId, eventId, variant, epoch) {
        if (themeFxBuffers.theme !== themeId) themeFxBuffers = { theme: themeId, buffers: {} };
        const cacheKey = eventId + ':' + variant;
        if (themeFxBuffers.buffers[cacheKey]) return themeFxBuffers.buffers[cacheKey];
        const pendingKey = [epoch, themeId, cacheKey].join(':');
        if (pendingThemeFxRenders.has(pendingKey)) return pendingThemeFxRenders.get(pendingKey);
        const pending = requestRender(themeId, eventId, variant, true).then(result => {
            if (!result || epoch !== soundEpoch || document.hidden || currentThemeId() !== themeId) return null;
            if (themeFxBuffers.theme !== themeId) themeFxBuffers = { theme: themeId, buffers: {} };
            const entry = { result, buffer: null };
            themeFxBuffers.buffers[cacheKey] = entry;
            stats.renders++;
            return entry;
        }).finally(() => pendingThemeFxRenders.delete(pendingKey));
        pendingThemeFxRenders.set(pendingKey, pending);
        return pending;
    }

    function playBuffer(buffer, category, options) {
        const ctx = ensureContext(true);
        if (!ctx || !buffer || !master) return false;
        if (ctx.state === 'suspended') ctx.resume().catch(function () {});
        const dest = options && options.preview ? master : (categoryGains[category] || master);
        const src = ctx.createBufferSource();
        src.buffer = buffer;
        src.playbackRate.value = options && options.fx ? 1 + (Math.random() * 0.04 - 0.02) : (options && Number.isFinite(options.rate) ? options.rate : 1);
        const g = ctx.createGain();
        g.gain.value = options && options.gain != null ? options.gain : 1;
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
        if (eventId === 'session.start' && !options.preview) return playSessionStart();
        if (options.silent || document.hidden) return false;
        if (options.sessionRestore || (typeof state !== 'undefined' && state.sessionRestoring)) return false;
        if (!soundsEnabled() && !options.preview) return false;
        const category = EVENT_CATEGORY[eventId];
        if (!category) return false;
        if (!options.preview && !categoryEnabled(category)) return false;   // previews may audition a switched-off category
        if (!options.preview && !unlocked) { wireUnlock(); return false; }
        if (!options.preview && rateLimited(eventId)) return false;
        const epoch = soundEpoch, requestedAt = performance.now();
        try {
            const themeId = options.theme ? resolveThemeId(options.theme) : currentThemeId();
            const fx = family ? THEME_FX[family] : null;
            const fxState = family ? themeFxState[family] : null;
            if (fx && (options.preview || themeId !== fx.theme)) return false;
            const fxOptions = fx ? cleanThemeFxOptions(fx, options) : null;
            if (fx && fxOptions.strength <= 0) return false;
            const variant = fx ? fxOptions.variant : (Number.isInteger(options.variant) ? options.variant : chooseVariant(themeId, eventId));
            const entry = fx ? await ensureThemeFxBuffer(fx, themeId, eventId, variant, epoch) : await ensureEventBuffer(themeId, eventId, variant, true);
            if (!entry || epoch !== soundEpoch || document.hidden || (typeof state !== 'undefined' && state.sessionRestoring)) return false;
            if (options.preview) {
                if (options.previewToken !== previewEpoch || !window.DesktopSoundsPreviewActive) return false;
            } else {
                if (!soundsEnabled() || !unlocked || themeId !== currentThemeId() || !categoryEnabled(category)) return false;
                // A sound never plays out of sync with what the user saw; the buffer stays cached for next time.
                if (!fx && performance.now() - requestedAt > (LATE_MS[category] ?? 150)) { stats.dropped++; return false; }
            }
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
            const buffer = audioBufferFor(entry);
            if (!buffer) return false;
            stats.lastLufs = entry.result.lufs;
            stats.lastPeakDb = entry.result.peakDb;
            const gain = fx ? Math.pow(fxOptions.strength, fx.curve) * fx.events[eventId].gain : options.gain;
            const played = playBuffer(buffer, category, { event: eventId, gain, pan: fx ? fxOptions.pan : undefined, fx: !!fx, rate: options.rate, preview: !!options.preview });
            if (played && fx) fxState.played++;
            return played;
        } catch (e) {
            warnOnce('play:' + eventId, 'Desktop sound failed', eventId, e);
            return false;
        }
    }

    let startRequested = false, startSound = '';   // '', 'played', 'skipped-off', 'skipped-autoplay', 'skipped-late'
    async function playSessionStart() {
        if (startRequested) return false;   // once per page load, never later on the first click
        startRequested = true;
        if (document.hidden || !soundsEnabled() || !categoryEnabled('startup') || (typeof state !== 'undefined' && state.sessionRestoring)) { startSound = 'skipped-off'; return false; }
        const ctx = ensureContext(true);
        if (!ctx) { startSound = 'skipped-off'; return false; }
        if (ctx.state !== 'running') {
            try { await Promise.race([ctx.resume(), new Promise(resolve => setTimeout(resolve, 200))]); } catch (e) {}
        }
        if (ctx.state !== 'running') { startSound = 'skipped-autoplay'; return false; }
        unlocked = true;   // the browser lets this page play without a gesture
        const themeId = currentThemeId(), epoch = soundEpoch;
        const entry = await Promise.race([ensureEventBuffer(themeId, 'session.start', 0, true).catch(() => null), new Promise(resolve => setTimeout(() => resolve(null), START_WAIT_MS))]);
        if (!entry || epoch !== soundEpoch || document.hidden || !soundsEnabled() || currentThemeId() !== themeId) { startSound = 'skipped-late'; prewarm(themeId); return false; }
        const buffer = audioBufferFor(entry);
        const played = !!buffer && playBuffer(buffer, 'startup', { event: 'session.start' });
        startSound = played ? 'played' : 'skipped-late';
        prewarm(themeId);
        return played;
    }

    async function runPreview(themeId, events, spacing) {
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
            const pending = {};
            events.slice().reverse().forEach(id => { pending[id] = ensureEventBuffer(resolved, id, 0, true); });   // first event ends up in front
            const entries = await Promise.all(events.map(id => pending[id]));
            for (let i = 0; i < events.length; i++) {
                if (token !== previewEpoch || document.hidden) return;
                await desktopSound(events[i], { preview: true, theme: resolved, previewToken: token, variant: 0 });
                const seconds = entries[i] && entries[i].result ? entries[i].result.seconds : 0.4;
                await new Promise(r => setTimeout(r, spacing === 'fixed' ? 420 : Math.min(1600, Math.max(300, seconds * 1000 + 120))));
            }
            if (token === previewEpoch) stats.previewPlays++;
        } finally {
            if (token === previewEpoch) {
                window.DesktopSoundsPreviewActive = false;
                if (!soundsEnabled()) await closeSoundContext();
                else if (cache.theme !== currentThemeId()) { prewarmedTheme = ''; if (unlocked) prewarm(currentThemeId()); }   // a preview of another theme replaced the cache
            }
        }
    }
    function previewDesktopSound(themeId) { return runPreview(themeId, ['window.open', 'notify.message', 'window.close'], 'fixed'); }
    function previewSoundCategory(category, themeId) {
        if (CATEGORIES.indexOf(category) < 0) return Promise.reject(new Error('unknown sound category: ' + category));
        return runPreview(themeId, Object.keys(EVENT_VARIANTS).filter(id => EVENT_CATEGORY[id] === category), 'measured');
    }

    function measured(result) { return { seconds: result.seconds, lufs: result.lufs, peakDb: result.peakDb, sampleRate: result.sampleRate, length: result.length != null ? result.length : result.L.length }; }
    async function renderTheme(themeId) {
        const id = resolveThemeId(themeId), events = {};
        for (const eventId of Object.keys(EVENT_VARIANTS)) {
            events[eventId] = [];
            for (let v = 0; v < EVENT_VARIANTS[eventId]; v++) {
                const entry = await ensureEventBuffer(id, eventId, v, true);
                if (!entry) throw new Error('sound render cancelled: ' + id + '/' + eventId);
                events[eventId].push(measured(entry.result));
            }
        }
        return { theme: id, renderer, events };
    }
    async function renderEvent(themeId, eventId, variant) {   // measured values of one sound; used by browser tests
        const id = resolveThemeId(themeId);
        const result = THEME_FX_OF[eventId] ? await requestRender(id, eventId, String(variant), true)
            : ((await ensureEventBuffer(id, eventId, Number(variant) || 0, true)) || {}).result;
        if (!result) throw new Error('sound render cancelled: ' + id + '/' + eventId);
        return Object.assign(measured(result), { theme: id, event: eventId, variant, renderer });
    }

    function inspect() {
        return {
            enabled: soundsEnabled(),
            theme: currentThemeId(),
            volume: volumeGain(),
            unlocked,
            bundleLoaded,   // the main-thread fallback only; the Worker loads its own copy
            renderer,
            startSound,
            cachedTheme: cache.theme,
            renderedEvents: Object.keys(cache.entries).length,
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
            categories: Object.fromEntries(CATEGORIES.map(cat => [cat, categoryEnabled(cat)])),
            plays: stats.plays,
            renders: stats.renders,
            previewPlays: stats.previewPlays,
            dropped: stats.dropped,
            lastEvent: stats.lastEvent,
            lastLufs: stats.lastLufs,
            lastPeakDb: stats.lastPeakDb,
            themes: THEME_IDS.slice()
        };
    }

    function applySoundSettingsChange(key, value) {
        syncDesktopSoundSettings();
        if (key === 'sound.enabled' && value !== 'true') closeSoundContext();
    }

    const SWITCH_SELECTOR = '.vd-switch > input[type="checkbox"][data-setting-key], #vd-quick-settings input[type="checkbox"][data-quick-setting], #vd-quick-settings input[type="checkbox"][data-quick-dnd]';
    const RANGE_SELECTOR = 'input[type="range"].vd-setting-range[data-setting-key], #vd-quick-settings input[type="range"][data-quick-setting]';
    const DOCK_SELECTOR = '#vd-taskbar-apps .vd-dock-button, #vd-taskbar-apps .vd-task-button';
    let lastTickAt = 0;
    const feedbackSkipped = el => !el || el.disabled || !!el.closest('[data-sound="off"]');
    document.addEventListener('change', event => {
        const el = event.target;
        if (!el || typeof el.matches !== 'function' || !el.matches(SWITCH_SELECTOR) || feedbackSkipped(el)) return;
        desktopSound(el.checked ? 'ui.toggle.on' : 'ui.toggle.off');
    }, true);
    document.addEventListener('input', event => {
        const el = event.target;
        if (!el || typeof el.matches !== 'function' || !el.matches(RANGE_SELECTOR) || feedbackSkipped(el)) return;
        const now = performance.now();
        if (now - lastTickAt < FEEDBACK_GAP_MS) return;
        lastTickAt = now;
        const min = parseFloat(el.min || '0'), max = parseFloat(el.max || '100'), value = parseFloat(el.value);
        const pos = max > min && Number.isFinite(value) ? Math.min(1, Math.max(0, (value - min) / (max - min))) : 0.5;
        desktopSound('ui.tick', { rate: Math.pow(2, (pos * 2 - 1) / 12) });   // pitch follows the value within +-1 semitone
    }, true);
    document.addEventListener('click', event => {
        const el = event.target && typeof event.target.closest === 'function' ? event.target.closest(DOCK_SELECTOR) : null;
        if (el && !feedbackSkipped(el)) desktopSound('ui.select');
    }, true);

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
        previewCategory: previewSoundCategory,
        renderEvent,
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
