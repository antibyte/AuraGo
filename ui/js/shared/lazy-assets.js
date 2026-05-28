(function () {
    'use strict';

    const scriptPromises = new Map();
    const stylePromises = new Map();

    function versionedURL(src) {
        const value = String(src || '').trim();
        if (!value) return '';
        if (/^(?:https?:)?\/\//i.test(value) || value.startsWith('data:') || value.startsWith('blob:')) {
            return value;
        }
        const version = window.BUILD_VERSION || 'dev';
        if (!version || /(?:[?&])v=/.test(value)) return value;
        return value + (value.includes('?') ? '&' : '?') + 'v=' + encodeURIComponent(version);
    }

    function cacheKey(src) {
        const url = versionedURL(src);
        try {
            const parsed = new URL(url, window.location.origin);
            return parsed.pathname + parsed.search;
        } catch (_err) {
            return url;
        }
    }

    function loadScript(src) {
        const url = versionedURL(src);
        if (!url) return Promise.resolve();
        const key = cacheKey(src);
        if (scriptPromises.has(key)) return scriptPromises.get(key);
        const existing = Array.from(document.scripts || []).find(script => cacheKey(script.getAttribute('src') || '') === key);
        if (existing) {
            existing.dataset.auragoLoaded = 'true';
            const resolved = Promise.resolve();
            scriptPromises.set(key, resolved);
            return resolved;
        }
        const promise = new Promise((resolve, reject) => {
            const script = existing || document.createElement('script');
            script.dataset.auragoLazyAsset = 'script';
            script.onload = () => {
                script.dataset.auragoLoaded = 'true';
                resolve();
            };
            script.onerror = () => {
                scriptPromises.delete(key);
                reject(new Error('Failed to load script: ' + src));
            };
            if (!existing) {
                script.src = url;
                document.head.appendChild(script);
            }
        });
        scriptPromises.set(key, promise);
        return promise;
    }

    function loadStyle(href) {
        const url = versionedURL(href);
        if (!url) return Promise.resolve();
        const key = cacheKey(href);
        if (stylePromises.has(key)) return stylePromises.get(key);
        const existing = Array.from(document.querySelectorAll('link[rel~="stylesheet"]'))
            .find(link => cacheKey(link.getAttribute('href') || '') === key);
        if (existing) {
            existing.dataset.auragoLoaded = 'true';
            const resolved = Promise.resolve();
            stylePromises.set(key, resolved);
            return resolved;
        }
        const promise = new Promise((resolve, reject) => {
            const link = existing || document.createElement('link');
            link.rel = 'stylesheet';
            link.dataset.auragoLazyAsset = 'style';
            link.onload = () => {
                link.dataset.auragoLoaded = 'true';
                resolve();
            };
            link.onerror = () => {
                stylePromises.delete(key);
                reject(new Error('Failed to load stylesheet: ' + href));
            };
            if (!existing) {
                link.href = url;
                document.head.appendChild(link);
            }
        });
        stylePromises.set(key, promise);
        return promise;
    }

    function loadAll(assets) {
        const styles = assets && Array.isArray(assets.styles) ? assets.styles : [];
        const scripts = assets && Array.isArray(assets.scripts) ? assets.scripts : [];
        return Promise.all(styles.map(loadStyle)).then(() => {
            let chain = Promise.resolve();
            scripts.forEach(src => {
                chain = chain.then(() => loadScript(src));
            });
            return chain;
        });
    }

    window.AuraLazyAssets = {
        loadScript,
        loadStyle,
        loadAll,
        versionedURL
    };
})();
