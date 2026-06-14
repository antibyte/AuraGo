(function () {
    'use strict';

    /**
     * Shared helpers for AuraGo desktop media apps (Radio, TeeVee, etc.).
     * Exposed as window.AuraDesktopMediaHelpers.
     *
     * Keep this file dependency-free: it is loaded as a plain script and must
     * work in the embedded desktop window context without any build step.
     */

    function clean(value) {
        return String(value == null ? '' : value).trim();
    }

    function cleanID(value) {
        return clean(value).toLowerCase();
    }

    function escapeHTML(value) {
        return String(value == null ? '' : value)
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#39;');
    }

    function normalizeSearch(value) {
        return clean(value).toLowerCase().normalize('NFD').replace(/[\u0300-\u036f]/g, '');
    }

    /**
     * Computes a short, stable numeric hash for a string.
     *
     * @param {string} value
     * @returns {string} Hex string of the hash.
     */
    function hashString(value) {
        const text = clean(value);
        let hash = 5381;
        for (let i = 0; i < text.length; i++) {
            hash = ((hash << 5) + hash) + text.charCodeAt(i);
        }
        return (hash >>> 0).toString(16);
    }

    function countryFlag(code) {
        const value = clean(code).toUpperCase();
        if (!/^[A-Z]{2}$/.test(value)) return '';
        return String.fromCodePoint.apply(String, value.split('').map(ch => 0x1F1E6 + ch.charCodeAt(0) - 65));
    }

    function countryDisplayName(code) {
        const value = clean(code).toUpperCase();
        if (!/^[A-Z]{2}$/.test(value)) return value;
        try {
            const locale = document.documentElement.lang || navigator.language || 'en';
            const display = new Intl.DisplayNames([locale], { type: 'region' }).of(value);
            return display ? value + ' - ' + display : value;
        } catch (_) {
            return value;
        }
    }

    /**
     * Debounces a function and exposes a clear method for disposal.
     *
     * @param {Function} fn
     * @param {number} delay
     * @returns {{call: (...args: any[]) => void, clear: () => void}}
     */
    function debounce(fn, delay) {
        let timer = 0;
        return {
            call(...args) {
                clearTimeout(timer);
                timer = setTimeout(() => fn.apply(this, args), delay);
            },
            clear() {
                clearTimeout(timer);
            }
        };
    }

    /**
     * Creates a toast controller bound to a container element.
     *
     * @param {HTMLElement} container - Element that will display the message.
     * @returns {{show: (message: string, duration?: number) => void, clear: () => void}}
     */
    function createToast(container) {
        let timer = 0;
        return {
            show(message, duration) {
                if (!container) return;
                container.textContent = message || '';
                container.hidden = false;
                clearTimeout(timer);
                timer = setTimeout(() => { container.hidden = true; }, duration || 3600);
            },
            clear() {
                clearTimeout(timer);
                if (container) container.hidden = true;
            }
        };
    }

    /**
     * Updates the browser media session metadata.
     *
     * @param {Object} entry - The media entry.
     * @param {string} entry.name - Display title.
     * @param {string} [entry.country] - TeeVee-style country code.
     * @param {string} [entry.countrycode] - Radio-style country code.
     * @param {string} [entry.logo] - TeeVee-style artwork URL.
     * @param {string} [entry.favicon] - Radio-style artwork URL.
     * @param {string} [album] - Album name shown in media session.
     */
    function updateMediaSession(entry, album) {
        if (!('mediaSession' in navigator) || !entry) return;
        try {
            const country = clean(entry.country || entry.countrycode || '');
            const artwork = clean(entry.logo || entry.favicon || '');
            navigator.mediaSession.metadata = new MediaMetadata({
                title: clean(entry.name) || 'AuraGo Media',
                artist: country,
                album: album || 'AuraGo',
                artwork: artwork ? [{ src: artwork, sizes: '96x96', type: 'image/png' }] : []
            });
        } catch (_) {}
    }

    window.AuraDesktopMediaHelpers = {
        clean,
        cleanID,
        escapeHTML,
        normalizeSearch,
        hashString,
        countryFlag,
        countryDisplayName,
        debounce,
        createToast,
        updateMediaSession
    };
})();
