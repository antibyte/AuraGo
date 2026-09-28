(function () {
    'use strict';

    const SCREENSAVER_THEMES = ['abyss', 'event_horizon', 'aurora', 'ink', 'stardust'];
    const SCREENSAVER_IDLE_MINUTES = [1, 2, 3, 5, 10, 15, 30, 60];
    const SCREENSAVER_ASSETS = {
        styles: ['/css/desktop-screensaver.css'],
        scripts: ['/js/desktop/screensavers/host.js']
    };
    const SCREENSAVER_ACTIVITY_EVENTS = ['pointermove', 'pointerdown', 'keydown', 'wheel', 'touchstart'];
    const SCREENSAVER_WAKE_EVENTS = ['pointermove', 'pointerdown', 'mousedown', 'keydown', 'wheel', 'touchstart'];
    const SCREENSAVER_SWALLOW_EVENTS = ['pointerup', 'mouseup', 'click', 'dblclick', 'contextmenu', 'keyup', 'touchend'];
    const SCREENSAVER_FRAME_ACTIVITY_TYPE = 'aurago.desktop.activity';
    const SCREENSAVER_TICK_MS = 1000;
    const SCREENSAVER_WAKE_MOVE_PX = 8;
    const SCREENSAVER_PREVIEW_GRACE_MS = 800;
    const SCREENSAVER_SWALLOW_MS = 600;

    const ss = {
        wired: false,
        timer: null,
        lastActivity: Date.now(),
        idleOverrideMs: 0,
        active: false,
        starting: null,
        generation: 0,
        theme: '',
        preview: false,
        startedAt: 0,
        graceUntil: 0,
        wakePoint: null,
        swallowUntil: 0,
        lastTheme: '',
        activations: 0,
        suppressedReason: '',
        hostLoading: null,
        callObserver: null,
        wiredDocs: new WeakSet(),
        bridgedFrames: new WeakSet()
    };

    function screensaverEnabled() {
        return settingValue('screensaver.enabled') === 'true';
    }

    function screensaverIdleMs() {
        if (ss.idleOverrideMs > 0) return ss.idleOverrideMs;
        const minutes = parseInt(settingValue('screensaver.idle_minutes') || '5', 10);
        return (SCREENSAVER_IDLE_MINUTES.indexOf(minutes) >= 0 ? minutes : 5) * 60000;
    }

    function resolveScreensaverTheme(requested) {
        const id = String(requested || '');
        if (SCREENSAVER_THEMES.indexOf(id) >= 0) return id;
        if (id === 'random') {
            const pool = SCREENSAVER_THEMES.filter(theme => theme !== ss.lastTheme);
            return pool[Math.floor(Math.random() * pool.length)] || SCREENSAVER_THEMES[0];
        }
        return SCREENSAVER_THEMES[0];
    }

    function noteScreensaverActivity() {
        ss.lastActivity = Date.now();
    }

    function onScreensaverActivity() {
        if (!ss.active) noteScreensaverActivity();
    }

    function audibleVideoPlaying() {
        const videos = document.getElementsByTagName('video');
        for (let i = 0; i < videos.length; i++) {
            const video = videos[i];
            if (video.paused || video.ended || video.muted || video.volume === 0 || video.readyState < 2) continue;
            const rect = video.getBoundingClientRect();
            if (rect.width >= 2 && rect.height >= 2) return true;
        }
        return false;
    }

    function opaqueFrameFocused() {
        const active = document.activeElement;
        if (!active || active.tagName !== 'IFRAME') return false;
        if (ss.bridgedFrames.has(active)) return false;
        try {
            const doc = active.contentDocument;
            return !(doc && ss.wiredDocs.has(doc));
        } catch (_) {
            return true;
        }
    }

    function screensaverSuppressionReason() {
        if (document.hidden) return 'hidden';
        if (document.fullscreenElement) return 'fullscreen';
        if (document.getElementById('vd-sip-incoming')) return 'call';
        if (audibleVideoPlaying()) return 'video';
        if (opaqueFrameFocused()) return 'frame-focus';
        return '';
    }

    function wireSameOriginFrames() {
        const frames = document.getElementsByTagName('iframe');
        for (let i = 0; i < frames.length; i++) {
            let doc = null;
            try { doc = frames[i].contentDocument; } catch (_) { doc = null; }
            if (!doc || ss.wiredDocs.has(doc)) continue;
            ss.wiredDocs.add(doc);
            SCREENSAVER_ACTIVITY_EVENTS.forEach(type => {
                doc.addEventListener(type, onScreensaverActivity, { capture: true, passive: true });
            });
        }
    }

    function frameForWindow(source) {
        if (!source) return null;
        const frames = document.getElementsByTagName('iframe');
        for (let i = 0; i < frames.length; i++) {
            if (frames[i].contentWindow === source) return frames[i];
        }
        return null;
    }

    function onScreensaverFrameMessage(event) {
        const msg = event && event.data;
        if (!msg || msg.type !== SCREENSAVER_FRAME_ACTIVITY_TYPE) return;
        const frame = frameForWindow(event.source);
        if (!frame) return;
        ss.bridgedFrames.add(frame);
        onScreensaverActivity();
    }

    function tickScreensaver() {
        if (ss.active || ss.starting || !screensaverEnabled()) return;
        wireSameOriginFrames();
        const reason = screensaverSuppressionReason();
        ss.suppressedReason = reason;
        if (reason) {
            noteScreensaverActivity();
            return;
        }
        if (Date.now() - ss.lastActivity < screensaverIdleMs()) return;
        startScreensaver(settingValue('screensaver.theme'), { preview: false });
    }

    function wireScreensaverIdle() {
        if (ss.wired) return;
        ss.wired = true;
        noteScreensaverActivity();
        SCREENSAVER_ACTIVITY_EVENTS.forEach(type => {
            document.addEventListener(type, onScreensaverActivity, { capture: true, passive: true });
        });
        window.addEventListener('message', onScreensaverFrameMessage);
        ss.timer = window.setInterval(tickScreensaver, SCREENSAVER_TICK_MS);
    }

    function unwireScreensaverIdle() {
        if (!ss.wired) return;
        ss.wired = false;
        SCREENSAVER_ACTIVITY_EVENTS.forEach(type => {
            document.removeEventListener(type, onScreensaverActivity, { capture: true, passive: true });
        });
        window.removeEventListener('message', onScreensaverFrameMessage);
        if (ss.timer) window.clearInterval(ss.timer);
        ss.timer = null;
    }

    function loadScreensaverHost() {
        if (window.AuraScreensaverHost) return Promise.resolve(window.AuraScreensaverHost);
        if (ss.hostLoading) return ss.hostLoading;
        const loader = window.AuraLazyAssets && window.AuraLazyAssets.loadAll;
        if (!loader) return Promise.reject(new Error('screensaver asset loader unavailable'));
        ss.hostLoading = loader(SCREENSAVER_ASSETS).then(() => {
            if (!window.AuraScreensaverHost) throw new Error('screensaver host unavailable');
            return window.AuraScreensaverHost;
        }).finally(() => {
            ss.hostLoading = null;
        });
        return ss.hostLoading;
    }

    function wakeEventFar(event) {
        if (event.type !== 'pointermove') return true;
        if (event.pointerType === 'touch') return true;
        if (!ss.wakePoint) {
            ss.wakePoint = { x: event.clientX, y: event.clientY };
            return false;
        }
        const dx = event.clientX - ss.wakePoint.x;
        const dy = event.clientY - ss.wakePoint.y;
        return dx * dx + dy * dy > SCREENSAVER_WAKE_MOVE_PX * SCREENSAVER_WAKE_MOVE_PX;
    }

    function onScreensaverWakeEvent(event) {
        if (!ss.active) return;
        if (event.cancelable) event.preventDefault();
        event.stopImmediatePropagation();
        if (Date.now() < ss.graceUntil) {
            ss.wakePoint = null;
            return;
        }
        if (!wakeEventFar(event)) return;
        stopScreensaver('input');
    }

    function onScreensaverSwallowEvent(event) {
        if (!ss.active && Date.now() > ss.swallowUntil) return;
        if (event.cancelable) event.preventDefault();
        event.stopImmediatePropagation();
    }

    function wireScreensaverWake() {
        SCREENSAVER_WAKE_EVENTS.forEach(type => window.addEventListener(type, onScreensaverWakeEvent, { capture: true, passive: false }));
        SCREENSAVER_SWALLOW_EVENTS.forEach(type => window.addEventListener(type, onScreensaverSwallowEvent, { capture: true, passive: false }));
        if (!ss.callObserver && typeof MutationObserver === 'function') {
            ss.callObserver = new MutationObserver(() => {
                if (document.getElementById('vd-sip-incoming')) stopScreensaver('call');
            });
            ss.callObserver.observe(document.body, { childList: true });
        }
    }

    function unwireScreensaverWake() {
        SCREENSAVER_WAKE_EVENTS.forEach(type => window.removeEventListener(type, onScreensaverWakeEvent, { capture: true, passive: false }));
        if (ss.callObserver) ss.callObserver.disconnect();
        ss.callObserver = null;
        window.setTimeout(() => {
            if (ss.active) return;
            SCREENSAVER_SWALLOW_EVENTS.forEach(type => window.removeEventListener(type, onScreensaverSwallowEvent, { capture: true, passive: false }));
        }, SCREENSAVER_SWALLOW_MS + 50);
    }

    function prefersReducedMotion() {
        try {
            return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
        } catch (_) {
            return false;
        }
    }

    function startScreensaver(requestedTheme, options) {
        if (ss.active) return Promise.resolve(true);
        if (ss.starting) return ss.starting;
        const preview = !!(options && options.preview);
        const theme = resolveScreensaverTheme(requestedTheme || settingValue('screensaver.theme'));
        const generation = ++ss.generation;
        ss.active = true;
        ss.theme = theme;
        ss.preview = preview;
        ss.startedAt = Date.now();
        ss.graceUntil = ss.startedAt + (preview ? SCREENSAVER_PREVIEW_GRACE_MS : 0);
        ss.wakePoint = null;
        ss.lastTheme = theme;
        wireScreensaverWake();
        ss.starting = loadScreensaverHost().then(host => {
            if (generation !== ss.generation || !ss.active) return false;
            ss.activations++;
            return host.start({
                theme,
                preview,
                clock: settingValue('screensaver.clock') !== 'false',
                reducedMotion: prefersReducedMotion(),
                lang: document.documentElement.lang || 'en',
                t,
                isCurrent: () => generation === ss.generation && ss.active
            }).then(() => true);
        }).catch(err => {
            console.warn('Screensaver failed to start', err);
            if (generation === ss.generation) stopScreensaver('error');
            return false;
        }).finally(() => {
            if (generation === ss.generation) ss.starting = null;
        });
        return ss.starting;
    }

    function stopScreensaver(reason) {
        if (!ss.active) return;
        ss.generation++;
        ss.active = false;
        ss.starting = null;
        ss.preview = false;
        ss.swallowUntil = reason === 'input' ? Date.now() + SCREENSAVER_SWALLOW_MS : 0;
        unwireScreensaverWake();
        noteScreensaverActivity();
        if (window.AuraScreensaverHost) window.AuraScreensaverHost.stop({ reason: reason || 'stop' });
    }

    function previewDesktopScreensaver(theme) {
        noteScreensaverActivity();
        return startScreensaver(theme || settingValue('screensaver.theme'), { preview: true });
    }

    function syncScreensaverSettings() {
        if (screensaverEnabled()) {
            wireScreensaverIdle();
            return;
        }
        unwireScreensaverIdle();
        if (ss.active && !ss.preview) stopScreensaver('disabled');
    }

    function setScreensaverIdleOverrideMs(ms) {
        const value = Number(ms);
        ss.idleOverrideMs = isFinite(value) && value > 0 ? value : 0;
        noteScreensaverActivity();
    }

    function inspectScreensaver() {
        const host = window.AuraScreensaverHost;
        return {
            enabled: screensaverEnabled(),
            wired: ss.wired,
            active: ss.active,
            starting: !!ss.starting,
            theme: ss.theme,
            preview: ss.preview,
            idleMs: screensaverIdleMs(),
            idleForMs: Date.now() - ss.lastActivity,
            suppressedReason: ss.suppressedReason,
            activations: ss.activations,
            themes: SCREENSAVER_THEMES.slice(),
            host: host && typeof host.inspect === 'function' ? host.inspect() : null
        };
    }

    window.previewDesktopScreensaver = previewDesktopScreensaver;
    window.DesktopScreensaver = {
        start: theme => startScreensaver(theme, { preview: false }),
        preview: previewDesktopScreensaver,
        stop: stopScreensaver,
        inspect: inspectScreensaver,
        setIdleOverrideMs: setScreensaverIdleOverrideMs,
        noteActivity: noteScreensaverActivity,
        themes: SCREENSAVER_THEMES.slice()
    };

    window.addEventListener('pagehide', () => {
        stopScreensaver('pagehide');
        unwireScreensaverIdle();
    });

    const origApplyForScreensaver = applyDesktopSettings;
    applyDesktopSettings = function () {
        origApplyForScreensaver();
        syncScreensaverSettings();
    };
})();
