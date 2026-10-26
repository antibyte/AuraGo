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
        current.paused = !!paused;
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
        const current = {
            id: key,
            layer,
            controller,
            options: options || {},
            motion: !!(options && options.motion),
            paused: false,
            instance: {}
        };
        active = current;
        try {
            current.instance = impl.start({ layer, signal: controller.signal, options: options || {} }) || {};
        } catch (err) {
            console.warn('Theme effects failed to start', err);
            stop();
            return false;
        }
        document.addEventListener('visibilitychange', () => setPaused(current, document.hidden), { signal: controller.signal });
        window.addEventListener('pagehide', stop, { once: true, signal: controller.signal });
        if (document.hidden) setPaused(current, true);
        return true;
    }

    function activeId() {
        return active ? active.id : '';
    }

    // Tests/debug: forwards to instance.command(name, payload) when the active effect has one.
    function command(name, payload) {
        const current = active;
        if (!current || !current.instance || typeof current.instance.command !== 'function') return null;
        try {
            const result = current.instance.command(name, payload);
            return result === undefined ? null : result;
        } catch (_) {
            return null;
        }
    }

    function inspect() {
        const current = active;
        if (!current) return { activeId: '', motion: false, paused: false, rendererReady: false };
        let details = {};
        try {
            if (current.instance && typeof current.instance.inspect === 'function') {
                const result = current.instance.inspect();
                if (result && typeof result === 'object') details = result;
            }
        } catch (_) { /* diagnostics must not affect the effect host */ }
        return {
            ...details,
            activeId: current.id,
            motion: current.motion,
            paused: current.paused,
            aborted: current.controller.signal.aborted
        };
    }

    window.AuraDesktopThemeFx = { register, start, stop, activeId, inspect, command };
})();
