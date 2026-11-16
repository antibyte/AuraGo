(function () {
    'use strict';

    const states = new WeakMap();
    const activeTargets = new Set();
    const allowedTypes = new Set(['image/png', 'image/jpeg', 'image/webp', 'image/bmp']);

    function path(value) {
        if (!isCustom(value)) return '';
        return value.slice('file:'.length);
    }

    function isCustom(value) {
        return typeof value === 'string' && value.startsWith('file:');
    }

    function stateFor(target) {
        let state = states.get(target);
        if (!state) {
            state = { generation: 0, controller: null, objectURL: '', value: '', fit: 'cover' };
            states.set(target, state);
        }
        activeTargets.add(target);
        return state;
    }

    function revokeState(state) {
        if (state.controller) state.controller.abort();
        state.controller = null;
        if (state.objectURL) URL.revokeObjectURL(state.objectURL);
        state.objectURL = '';
    }

    function emitStatus(target, pathValue, status, code, callback) {
        if (target && target.dataset) target.dataset.customWallpaperState = status;
        if (typeof callback === 'function') {
            try { callback(pathValue, status, code || ''); } catch (_) { /* status observers cannot block wallpaper updates */ }
        }
    }

    function clearVisual(target, state) {
        if (!target || !target.style) return;
        target.style.removeProperty('--vd-custom-wallpaper-image');
        target.style.removeProperty('--vd-custom-wallpaper-fit');
        if (target.dataset) delete target.dataset.customWallpaperState;
        if (state) state.value = '';
    }

    function isCurrent(target, state, generation, signal, options, value, relative) {
        if (signal.aborted || states.get(target) !== state || state.generation !== generation) return false;
        if (options.signal && options.signal.aborted) return false;
        if (typeof options.isCurrent === 'function') {
            try { return !!options.isCurrent(value, relative); } catch (_) { return false; }
        }
        return true;
    }

    async function decodeImage(objectURL) {
        if (typeof Image !== 'function') throw new Error('image_decoder_unavailable');
        const image = new Image();
        image.decoding = 'async';
        image.src = objectURL;
        if (typeof image.decode === 'function') {
            await image.decode();
        } else {
            await new Promise((resolve, reject) => {
                image.onload = resolve;
                image.onerror = () => reject(new Error('wallpaper_invalid_image'));
            });
        }
        if (!image.naturalWidth || !image.naturalHeight || image.naturalWidth > 8192 || image.naturalHeight > 8192 || image.naturalWidth * image.naturalHeight > 16_000_000) {
            throw new Error('wallpaper_invalid_image');
        }
    }

    async function apply(target, value, fit, options) {
        options = options && typeof options === 'object' ? options : {};
        const relative = path(value);
        if (!target || !target.style || !target.dataset || !relative) {
            if (target) release(target);
            emitStatus(target, relative, 'invalid', 'wallpaper_invalid_path', options.onStatus);
            return false;
        }

        const state = stateFor(target);
        revokeState(state);
        state.generation++;
        const generation = state.generation;
        const controller = new AbortController();
        state.controller = controller;
        state.value = value;
        state.fit = fit === 'contain' ? 'contain' : 'cover';
        const signal = controller.signal;
        const externalSignal = options.signal;
        const abortWithExternal = () => controller.abort();
        if (externalSignal) {
            if (externalSignal.aborted) controller.abort();
            else externalSignal.addEventListener('abort', abortWithExternal, { once: true });
        }
        emitStatus(target, relative, 'loading', '', options.onStatus);

        const current = () => isCurrent(target, state, generation, signal, options, value, relative);
        let objectURL = '';
        try {
            if (!current()) return false;
            const query = new URLSearchParams({ path: relative });
            const response = await fetch('/api/desktop/wallpaper?' + query.toString(), {
                method: 'GET',
                credentials: 'same-origin',
                cache: 'no-store',
                signal
            });
            if (!current()) return false;
            if (!response.ok) {
                let code = 'wallpaper_load_failed';
                try {
                    const body = await response.json();
                    if (typeof body.code === 'string' && body.code.startsWith('wallpaper_')) code = body.code;
                } catch (_) { /* HTTP status still produces a stable generic failure */ }
                const error = new Error(code);
                error.code = code;
                error.status = response.status;
                throw error;
            }

            const blob = await response.blob();
            if (!current()) return false;
            if (blob.size <= 0 || blob.size > 20 * 1024 * 1024 || !allowedTypes.has(String(blob.type || '').toLowerCase())) {
                const error = new Error('wallpaper_invalid_image');
                error.code = error.message;
                throw error;
            }
            objectURL = URL.createObjectURL(blob);
            await decodeImage(objectURL);
            if (!current()) {
                URL.revokeObjectURL(objectURL);
                objectURL = '';
                return false;
            }

            state.objectURL = objectURL;
            objectURL = '';
            target.style.setProperty('--vd-custom-wallpaper-image', `url(${JSON.stringify(state.objectURL)})`);
            target.style.setProperty('--vd-custom-wallpaper-fit', state.fit);
            emitStatus(target, relative, 'ready', '', options.onStatus);
            return true;
        } catch (error) {
            if (objectURL) URL.revokeObjectURL(objectURL);
            if (!current()) return false;
            target.style.removeProperty('--vd-custom-wallpaper-image');
            target.style.removeProperty('--vd-custom-wallpaper-fit');
            const code = String(error && (error.code || error.message) || 'wallpaper_load_failed');
            const stateName = code === 'wallpaper_not_found' ? 'missing'
                : code === 'wallpaper_invalid_path' || code === 'wallpaper_invalid_image' || code === 'wallpaper_unsupported_format' || code === 'wallpaper_format_mismatch' || code === 'wallpaper_too_large' || code === 'wallpaper_dimensions_exceeded' || code === 'wallpaper_pixel_limit'
                    ? 'invalid'
                    : 'error';
            emitStatus(target, relative, stateName, code, options.onStatus);
            return false;
        } finally {
            if (externalSignal) externalSignal.removeEventListener('abort', abortWithExternal);
            if (state.controller === controller) state.controller = null;
        }
    }

    function release(target) {
        if (!target) return;
        const state = states.get(target);
        if (state) {
            state.generation++;
            revokeState(state);
            state.value = '';
        }
        clearVisual(target, state);
        activeTargets.delete(target);
    }

    function releaseAll() {
        for (const target of activeTargets) release(target);
        activeTargets.clear();
    }

    window.addEventListener('pagehide', releaseAll);
    window.AuraDesktopCustomWallpaper = Object.freeze({ isCustom, path, apply, release });
})();
