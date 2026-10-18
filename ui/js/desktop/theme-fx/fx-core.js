(function () {
    'use strict';

    // Host for the lazily loaded theme effects (bundle "desktop-theme-fx"). An effect module
    // registers itself under its theme id; the shell (syncDesktopThemeFx in core/theme-runtime.js)
    // starts and stops it. The host owns the layer, the AbortSignal and the hidden-tab pause.
    const implementations = new Map();
    let active = null;

    function register(id, impl) {
        if (!id || !impl || typeof impl.start !== 'function') return;
        implementations.set(String(id), impl);
    }

    function stop() {
        const current = active;
        if (!current) return;
        active = null;
        try {
            if (typeof current.instance.stop === 'function') current.instance.stop();
        } catch (err) {
            console.warn('Theme effects failed to stop', err);
        }
        current.controller.abort();
        current.layer.remove();
    }

    function setPaused(current, paused) {
        const method = paused ? current.instance.pause : current.instance.resume;
        if (typeof method !== 'function') return;
        try {
            method.call(current.instance);
        } catch (err) {
            console.warn('Theme effects failed to ' + (paused ? 'pause' : 'resume'), err);
        }
    }

    function start(id, options) {
        stop();
        const key = String(id || '');
        const impl = implementations.get(key);
        if (!impl) return false;
        const layer = document.createElement('div');
        layer.className = 'vd-theme-fx-layer';
        layer.setAttribute('aria-hidden', 'true');
        document.body.appendChild(layer);
        const controller = new AbortController();
        const current = { id: key, layer, controller, instance: {} };
        active = current;
        try {
            current.instance = impl.start({ layer, signal: controller.signal, options: options || {} }) || {};
        } catch (err) {
            console.warn('Theme effects failed to start', err);
            stop();
            return false;
        }
        document.addEventListener('visibilitychange', () => setPaused(current, document.hidden), { signal: controller.signal });
        if (document.hidden) setPaused(current, true);
        return true;
    }

    function activeId() {
        return active ? active.id : '';
    }

    window.AuraDesktopThemeFx = { register, start, stop, activeId };
})();
