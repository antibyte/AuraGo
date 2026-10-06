// EasyDrag shared helpers: icons, DOM and format helpers, events, storage, API client, dialogs.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    // 24×24 stroke icons (own drawings, MIT-compatible shapes). Keys match NodeTypeInfo.icon and UI names.
    const ICONS = {
        'tool': '<path d="M14.7 6.3a4 4 0 0 0-5.4 5.4L4 17v3h3l5.3-5.3a4 4 0 0 0 5.4-5.4l-2.6 2.6-2.4-.6-.6-2.4z"/>',
        'hand-click': '<path d="M8 13V5.5a1.5 1.5 0 0 1 3 0V12"/><path d="M11 11.5v-2a1.5 1.5 0 0 1 3 0V12"/><path d="M14 10.5a1.5 1.5 0 0 1 3 0V12"/><path d="M17 11.5a1.5 1.5 0 0 1 3 0V16a6 6 0 0 1-6 6h-2a6 6 0 0 1-5-2.7L4.3 15a1.5 1.5 0 0 1 2.4-1.8L8 15"/><path d="M3 4l1.5 1.5M8 2v1.5M13 4l-1.5 1.5"/>',
        'clock': '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 3"/>',
        'calendar-time': '<path d="M11.8 21H6a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v4"/><path d="M16 3v4M8 3v4M4 11h10"/><circle cx="18" cy="18" r="4"/><path d="M18 16.5V18l1 1"/>',
        'webhook': '<path d="M4.9 15.9a4 4 0 1 0 6.1 3.4h6"/><path d="M15.8 17.8a4 4 0 1 0-2.6-7.4L10 4.9"/><path d="M12 6.6a4 4 0 1 0-6.4 4.2L8.7 16"/>',
        'mail': '<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M3 7l9 6 9-6"/>',
        'broadcast': '<path d="M18.4 18.4a9 9 0 1 0-12.8 0"/><path d="M15.5 15.5a5 5 0 1 0-7 0"/><circle cx="12" cy="12" r="1"/>',
        'home-signal': '<path d="M5 12H3l9-9 9 9h-2"/><path d="M5 12v7a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-7"/><path d="M10 13.5a3 3 0 0 1 4 0M8.5 11.5a5 5 0 0 1 7 0"/><circle cx="12" cy="16" r=".5"/>',
        'device-desktop': '<rect x="3" y="4" width="18" height="12" rx="1"/><path d="M7 20h10M9 16v4M15 16v4"/>',
        'phone-incoming': '<path d="M5 4h4l2 5-2.5 1.5a11 11 0 0 0 5 5L15 13l5 2v4a2 2 0 0 1-2 2A16 16 0 0 1 3 6a2 2 0 0 1 2-2"/><path d="M15 9l5-5M15 5v4h4"/>',
        'calendar-event': '<rect x="4" y="5" width="16" height="16" rx="2"/><path d="M16 3v4M8 3v4M4 11h16"/><rect x="8" y="15" width="2" height="2"/>',
        'power': '<path d="M7 6a7.75 7.75 0 1 0 10 0"/><path d="M12 4v8"/>',
        'coin': '<circle cx="12" cy="12" r="9"/><path d="M14.8 9A2 2 0 0 0 13 8h-2a2 2 0 0 0 0 4h2a2 2 0 0 1 0 4h-2a2 2 0 0 1-1.8-1"/><path d="M12 7v10"/>',
        'checks': '<path d="M7 12l5 5L22 7"/><path d="M2 12l5 5m5-5l5-5"/>',
        'sparkles': '<path d="M16 18a2 2 0 0 1 2 2 2 2 0 0 1 2-2 2 2 0 0 1-2-2 2 2 0 0 1-2 2zM16 6a2 2 0 0 1 2 2 2 2 0 0 1 2-2 2 2 0 0 1-2-2 2 2 0 0 1-2 2zM9 18a6 6 0 0 1 6-6 6 6 0 0 1-6-6 6 6 0 0 1-6 6 6 6 0 0 1 6 6z"/>',
        'file-type-pdf': '<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M5 12V5a2 2 0 0 1 2-2h7l5 5v4"/><path d="M5 18h1.5a1.5 1.5 0 0 0 0-3H5v6M17 18h2M20 15h-3v6M11 15v6h1a2 2 0 0 0 2-2v-2a2 2 0 0 0-2-2z"/>',
        'file-search': '<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M12 21H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h7l5 5v4.5"/><circle cx="16.5" cy="17.5" r="2.5"/><path d="M18.5 19.5L21 22"/>',
        'file-text': '<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h7l5 5v11a2 2 0 0 1-2 2z"/><path d="M9 9h1M9 13h6M9 17h6"/>',
        'file-pencil': '<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h7l5 5v11a2 2 0 0 1-2 2z"/><path d="M10 18l5-5a1.4 1.4 0 0 0-2-2l-5 5v2z"/>',
        'search': '<circle cx="10" cy="10" r="7"/><path d="M21 21l-6-6"/>',
        'world-www': '<path d="M19.5 7A9 9 0 0 0 12 3a8.991 8.991 0 0 0-7.484 4"/><path d="M11.5 3a16.989 16.989 0 0 0-1.826 4"/><path d="M12.5 3a16.989 16.989 0 0 1 1.828 4"/><path d="M19.5 17a9 9 0 0 1-7.5 4a8.991 8.991 0 0 1-7.484-4"/><path d="M11.5 21a16.989 16.989 0 0 1-1.826-4"/><path d="M12.5 21a16.989 16.989 0 0 0 1.828-4"/><path d="M2 10l1 4 1.5-4 1.5 4 1-4M17 10l1 4 1.5-4 1.5 4 1-4M9.5 10l1 4 1.5-4 1.5 4 1-4"/>',
        'api': '<path d="M4 13h5M12 16V8h3a2 2 0 0 1 2 2v1a2 2 0 0 1-2 2h-3M20 8v8M9 16v-5.5a2.5 2.5 0 0 0-5 0V16"/>',
        'brand-telegram': '<path d="M15 10l-4 4 6 6 4-16-18 7 4 2 2 6 3-4"/>',
        'bell': '<path d="M10 5a2 2 0 0 1 4 0 7 7 0 0 1 4 6v3a4 4 0 0 0 2 3H4a4 4 0 0 0 2-3v-3a7 7 0 0 1 4-6"/><path d="M9 17v1a3 3 0 0 0 6 0v-1"/>',
        'brand-discord': '<circle cx="9" cy="12" r="1"/><circle cx="15" cy="12" r="1"/><path d="M7.5 7.5c3.5-1 5.5-1 9 0M7 16.5c3.5 1 6.5 1 10 0"/><path d="M15.5 17c0 1 1.5 3 2 3 1.5 0 2.833-1.667 3.5-3 .667-1.667.5-5.833-1.5-11.5-1.457-1.015-3-1.34-4.5-1.5l-1 2.5M8.5 17c0 1-1.356 3-1.832 3-1.429 0-2.698-1.667-3.333-3-.635-1.667-.476-5.833 1.428-11.5C6.151 4.485 7.545 4.16 9 4l1 2.5"/>',
        'home': '<path d="M5 12H3l9-9 9 9h-2"/><path d="M5 12v7a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-7"/><path d="M9 21v-6a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v6"/>',
        'calendar-plus': '<path d="M12.5 21H6a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v5"/><path d="M16 3v4M8 3v4M4 11h16M16 19h6M19 16v6"/>',
        'checkbox': '<path d="M9 11l3 3 8-8"/><path d="M20 12v6a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h9"/>',
        'git-branch': '<circle cx="7" cy="18" r="2"/><circle cx="7" cy="6" r="2"/><circle cx="17" cy="6" r="2"/><path d="M7 8v8M9 18h6a2 2 0 0 0 2-2V8"/><path d="M14 11l3-3 3 3"/>',
        'arrows-split': '<path d="M21 17h-8l-3.5-5H3M21 7h-8l-3.495 5"/><path d="M18 10l3-3-3-3M18 20l3-3-3-3"/>',
        'arrow-merge': '<path d="M8 7l4-4 4 4M12 3v5.394a6.737 6.737 0 0 1-3 5.606 6.737 6.737 0 0 0-3 5.606V21M12 3v5.394a6.737 6.737 0 0 0 3 5.606 6.737 6.737 0 0 1 3 5.606V21"/>',
        'hourglass': '<path d="M6.5 7h11M6.5 17h11M6 20v-2a6 6 0 1 1 12 0v2a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1zM6 4v2a6 6 0 1 0 12 0V4a1 1 0 0 0-1-1H7a1 1 0 0 0-1 1z"/>',
        'pencil': '<path d="M4 20h4L18.5 9.5a2.828 2.828 0 1 0-4-4L4 16v4M13.5 6.5l4 4"/>',
        'player-stop': '<rect x="5" y="5" width="14" height="14" rx="2"/>',
        'plus': '<path d="M12 5v14M5 12h14"/>',
        'minus': '<path d="M5 12h14"/>',
        'play': '<path d="M7 4v16l13-8z"/>',
        'stop': '<rect x="6" y="6" width="12" height="12" rx="1.5"/>',
        'x': '<path d="M18 6L6 18M6 6l12 12"/>',
        'trash': '<path d="M4 7h16M10 11v6M14 11v6M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2l1-12M9 7V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v3"/>',
        'copy': '<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/>',
        'undo': '<path d="M9 14L4 9l5-5"/><path d="M4 9h10.5a5.5 5.5 0 0 1 0 11H11"/>',
        'redo': '<path d="M15 14l5-5-5-5"/><path d="M20 9H9.5a5.5 5.5 0 0 0 0 11H13"/>',
        'fit': '<path d="M4 8V6a2 2 0 0 1 2-2h2M4 16v2a2 2 0 0 0 2 2h2M16 4h2a2 2 0 0 1 2 2v2M16 20h2a2 2 0 0 0 2-2v-2"/><rect x="9" y="9" width="6" height="6" rx="1"/>',
        'chevron-left': '<path d="M15 6l-6 6 6 6"/>',
        'chevron-right': '<path d="M9 6l6 6-6 6"/>',
        'chevron-down': '<path d="M6 9l6 6 6-6"/>',
        'dots': '<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
        'alert': '<path d="M12 9v4M12 17h.01"/><path d="M10.24 3.957L2.414 17.003A1.914 1.914 0 0 0 4.05 20h15.9a1.914 1.914 0 0 0 1.636-2.997L13.76 3.957a2.061 2.061 0 0 0-3.52 0z"/>',
        'check': '<path d="M5 12l5 5L20 7"/>',
        'lock': '<rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V7a4 4 0 1 1 8 0v4"/>',
        'settings': '<path d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 0 0 2.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 0 0 1.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 0 0-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 0 0-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 0 0-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 0 0-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 0 0 1.066-2.573c-.94-1.543.826-3.31 2.37-2.37 1 .608 2.296.07 2.572-1.065z"/><circle cx="12" cy="12" r="3"/>',
        'list': '<path d="M9 6h11M9 12h11M9 18h11M5 6v.01M5 12v.01M5 18v.01"/>',
        'upload': '<path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2M7 9l5-5 5 5M12 4v12"/>',
        'download': '<path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2M7 11l5 5 5-5M12 4v12"/>',
        'history': '<path d="M12 8v4l2 2"/><path d="M3.05 11a9 9 0 1 1 .5 4m-.5 5v-5h5"/>',
        'keyboard': '<rect x="2" y="6" width="20" height="12" rx="2"/><path d="M6 10h.01M10 10h.01M14 10h.01M18 10h.01M6 14h.01M18 14h.01M10 14h4"/>',
        'eye-off': '<path d="M10.585 10.587a2 2 0 0 0 2.829 2.828"/><path d="M16.681 16.673A8.717 8.717 0 0 1 12 18c-3.6 0-6.6-2-9-6 1.272-2.12 2.712-3.678 4.32-4.674m2.86-1.146A9.055 9.055 0 0 1 12 6c3.6 0 6.6 2 9 6-.666 1.11-1.379 2.067-2.138 2.87M3 3l18 18"/>',
        'link': '<path d="M9 15l6-6M11 6l.463-.536a5 5 0 0 1 7.071 7.072L18 13M13 18l-.397.534a5.068 5.068 0 0 1-7.127 0 4.972 4.972 0 0 1 0-7.071L6 11"/>',
        'bolt': '<path d="M13 3v7h6l-8 11v-7H5l8-11"/>',
        'arrow-left': '<path d="M5 12h14M5 12l6 6M5 12l6-6"/>',
        'grid': '<rect x="4" y="4" width="6" height="6" rx="1"/><rect x="14" y="4" width="6" height="6" rx="1"/><rect x="4" y="14" width="6" height="6" rx="1"/><rect x="14" y="14" width="6" height="6" rx="1"/>',
        'external': '<path d="M12 6H6a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-6M11 13l9-9M15 4h5v5"/>',
        'refresh': '<path d="M20 11A8.1 8.1 0 0 0 4.5 9M4 5v4h4M4 13a8.1 8.1 0 0 0 15.5 2m.5 4v-4h-4"/>',
        'info': '<circle cx="12" cy="12" r="9"/><path d="M12 8h.01M11 12h1v4h1"/>',
        'flask': '<path d="M9 3h6M10 9h4M10 3v6L6 18a2 2 0 0 0 1.8 3h8.4a2 2 0 0 0 1.8-3l-4-9V3"/>',
        'rocket': '<path d="M4 13a8 8 0 0 1 7 7a6 6 0 0 0 3-5a9 9 0 0 0 6-8a3 3 0 0 0-3-3a9 9 0 0 0-8 6a6 6 0 0 0-5 3"/><path d="M7 14a6 6 0 0 0-3 6a6 6 0 0 0 6-3"/><circle cx="15" cy="9" r="1"/>',
        'key': '<path d="M16.555 3.843l3.602 3.602a2.877 2.877 0 0 1 0 4.069l-2.643 2.643a2.877 2.877 0 0 1-4.069 0l-.301-.301-6.558 6.558a2 2 0 0 1-1.239.578L5.172 21H4a1 1 0 0 1-.993-.883L3 20v-1.172a2 2 0 0 1 .467-1.284l.119-.13L4 17h2v-2h2v-2l2.144-2.144-.301-.301a2.877 2.877 0 0 1 0-4.069l2.643-2.643a2.877 2.877 0 0 1 4.069 0z"/><path d="M15 9h.01"/>',
        'drag': '<path d="M9 5h.01M9 12h.01M9 19h.01M15 5h.01M15 12h.01M15 19h.01"/>',
        'pin': '<path d="M15 4.5l-4 4-4 1.5-1.5 1.5 7 7 1.5-1.5 1.5-4 4-4M9 15l-4.5 4.5M14.5 4L20 9.5"/>'
    };

    function icon(name, cls) {
        const body = Object.prototype.hasOwnProperty.call(ICONS, name) ? ICONS[name] : ICONS.tool;
        return '<svg class="ed-icon' + (cls ? ' ' + esc(cls) : '') + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">' + body + '</svg>';
    }

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
    }

    // tr translates a dynamic key and falls back when the key has no translation.
    function tr(t, key, fallback) {
        const value = t(key);
        return value && value !== key ? value : fallback;
    }

    function clamp(value, lo, hi) { return Math.min(hi, Math.max(lo, value)); }

    function debounce(fn, ms) {
        let timer = 0;
        const wrapped = (...args) => { clearTimeout(timer); timer = setTimeout(() => fn(...args), ms); };
        wrapped.cancel = () => clearTimeout(timer);
        wrapped.flush = (...args) => { clearTimeout(timer); fn(...args); };
        return wrapped;
    }

    // frame coalesces many requests into one requestAnimationFrame callback.
    function frame(fn) {
        let id = 0;
        return {
            request() { if (!id) id = requestAnimationFrame(() => { id = 0; fn(); }); },
            cancel() { if (id) cancelAnimationFrame(id); id = 0; }
        };
    }

    function emitter() {
        const map = new Map();
        const api = {
            on(type, fn) {
                if (!map.has(type)) map.set(type, new Set());
                map.get(type).add(fn);
                return () => api.off(type, fn);
            },
            off(type, fn) { const set = map.get(type); if (set) set.delete(fn); },
            emit(type, payload) {
                const set = map.get(type);
                if (!set) return;
                Array.from(set).forEach(fn => {
                    if (!set.has(fn)) return; // removed by an earlier handler of this emit
                    try { fn(payload); } catch (err) { console.error('EasyDrag event handler failed', type, err); }
                });
            },
            clear() { map.clear(); }
        };
        return api;
    }

    // bag collects cleanup functions and runs them once.
    function bag() {
        const items = [];
        return {
            add(fn) { if (typeof fn === 'function') items.push(fn); return fn; },
            listen(target, type, fn, options) {
                target.addEventListener(type, fn, options);
                items.push(() => target.removeEventListener(type, fn, options));
            },
            dispose() { while (items.length) { try { items.pop()(); } catch (err) { console.warn('EasyDrag cleanup failed', err); } } }
        };
    }

    function el(markup) {
        const tpl = document.createElement('template');
        tpl.innerHTML = String(markup).trim();
        return tpl.content.firstElementChild;
    }

    function isEditable(target) {
        if (!target || !target.closest) return false;
        return !!target.closest('input, textarea, select, [contenteditable="true"], .ed-code');
    }

    const IS_MAC = /Mac|iPhone|iPad/.test(navigator.platform || '');

    function isMod(event) { return IS_MAC ? event.metaKey : event.ctrlKey; }

    function shortcut(text) { return IS_MAC ? text.replace(/Ctrl\+/g, '⌘').replace(/Shift\+/g, '⇧') : text; }

    const ALPHABET = 'abcdefghijklmnopqrstuvwxyz234567';

    function randomID(prefix, length) {
        const bytes = new Uint8Array(length);
        (window.crypto || window.msCrypto).getRandomValues(bytes);
        let out = prefix;
        for (let i = 0; i < length; i++) out += ALPHABET[bytes[i] % ALPHABET.length];
        return out;
    }

    function lang() {
        return String(window.SYSTEM_LANG || document.documentElement.lang || 'en').toLowerCase().split('-')[0];
    }

    const fmt = {
        number(value) {
            try { return new Intl.NumberFormat(lang()).format(value); } catch (err) { return String(value); }
        },
        duration(ms) {
            const n = Number(ms) || 0;
            if (n < 1000) return Math.round(n) + ' ms';
            if (n < 10000) return (n / 1000).toFixed(1) + ' s';
            // Whole seconds first, then minutes: 119 999 ms reads 2:00 min, not 1:60 min.
            const total = Math.round(n / 1000);
            if (total < 60) return total + ' s';
            return Math.floor(total / 60) + ':' + String(total % 60).padStart(2, '0') + ' min';
        },
        dateTime(iso) {
            const date = iso instanceof Date ? iso : new Date(iso);
            if (isNaN(date.getTime())) return '';
            try { return date.toLocaleString(lang(), { dateStyle: 'medium', timeStyle: 'short' }); } catch (err) { return date.toISOString(); }
        },
        relative(iso) {
            const date = iso instanceof Date ? iso : new Date(iso);
            if (isNaN(date.getTime())) return '';
            const diff = (date.getTime() - Date.now()) / 1000;
            const abs = Math.abs(diff);
            const units = [['year', 31536000], ['month', 2592000], ['week', 604800], ['day', 86400], ['hour', 3600], ['minute', 60], ['second', 1]];
            for (const [unit, seconds] of units) {
                if (abs >= seconds || unit === 'second') {
                    try { return new Intl.RelativeTimeFormat(lang(), { numeric: 'auto' }).format(Math.round(diff / seconds), unit); } catch (err) { return fmt.dateTime(date); }
                }
            }
            return '';
        },
        bytes(n) {
            const value = Number(n) || 0;
            if (value < 1024) return value + ' B';
            if (value < 1048576) return (value / 1024).toFixed(1) + ' KB';
            return (value / 1048576).toFixed(1) + ' MB';
        }
    };

    const storage = {
        get(key, fallback) {
            try { const raw = localStorage.getItem(key); return raw == null ? fallback : JSON.parse(raw); } catch (err) { return fallback; }
        },
        set(key, value) {
            try { localStorage.setItem(key, JSON.stringify(value)); return true; } catch (err) { return false; }
        },
        remove(key) { try { localStorage.removeItem(key); } catch (err) { /* storage may be blocked */ } }
    };

    // pathSegment encodes one URL path segment. "", "." and ".." would leave the route after URL
    // normalisation, so they throw a FLOW_BAD_REQUEST error instead.
    function pathSegment(value) {
        const s = String(value == null ? '' : value);
        if (s === '' || s === '.' || s === '..') {
            throw Object.assign(new Error('invalid path segment'), { body: { error: 'invalid path segment', code: 'FLOW_BAD_REQUEST' } });
        }
        return encodeURIComponent(s);
    }

    // createApi wraps the desktop api() for every flows route. A bad id or name rejects the request
    // (the *Url builders throw) before anything is sent.
    function createApi(api) {
        const base = '/api/desktop/flows';
        const enc = pathSegment;
        const json = (method, body) => ({ method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body || {}) });
        const query = params => {
            const p = new URLSearchParams();
            Object.entries(params || {}).forEach(([k, v]) => { if (v !== undefined && v !== null && v !== '') p.set(k, v); });
            const s = p.toString();
            return s ? '?' + s : '';
        };
        const flow = id => base + '/' + enc(id);
        const client = {
            list: () => api(base),
            create: body => api(base, json('POST', body)),
            get: id => api(flow(id)),
            save: (id, doc, revision) => api(flow(id), json('PUT', { doc, base_revision: revision })),
            remove: id => api(flow(id), { method: 'DELETE' }),
            preview: id => api(flow(id) + '/publish-preview'),
            publish: (id, revision) => api(flow(id) + '/publish', json('POST', { base_revision: revision })),
            setEnabled: (id, enabled) => api(flow(id) + '/enabled', json('POST', { enabled })),
            test: (id, body) => api(flow(id) + '/test', json('POST', body)),
            runNow: id => api(flow(id) + '/run', json('POST')),
            runs: (id, params) => api(flow(id) + '/runs' + query(params)),
            run: (runId, includeDoc) => api(base + '/runs/' + enc(runId) + (includeDoc ? '?include=doc' : '')),
            cancel: runId => api(base + '/runs/' + enc(runId) + '/cancel', json('POST')),
            eventsUrl: (runId, after) => base + '/runs/' + enc(runId) + '/events' + query({ after }),
            testData: (id, node) => api(flow(id) + '/test-data/' + enc(node)),
            saveTestData: (id, node, data) => api(flow(id) + '/test-data/' + enc(node), json('PUT', { data })),
            exportUrl: id => flow(id) + '/export',
            nodeTypes: () => api(base + '/node-types' + query({ lang: lang() })),
            options: (type, param) => api(base + '/node-types/' + enc(type) + '/options/' + enc(param) + query({ lang: lang() })),
            templates: () => api(base + '/templates' + query({ lang: lang() })),
            validate: (doc, mode) => api(base + '/validate', json('POST', { doc, mode })),
            secrets: () => api(base + '/secrets'),
            saveSecret: (name, value) => api(base + '/secrets/' + enc(name), json('PUT', { value })),
            deleteSecret: name => api(base + '/secrets/' + enc(name), { method: 'DELETE' })
        };
        Object.keys(client).forEach(name => {
            if (name.endsWith('Url')) return;
            const call = client[name];
            client[name] = (...args) => { try { return call(...args); } catch (err) { return Promise.reject(err); } };
        });
        return client;
    }

    function errorCode(err) { return (err && err.body && err.body.code) || ''; }

    // errorText turns an API error into a translated sentence; codes map to easydrag.ui.error_<code>.
    function errorText(t, err) {
        const code = errorCode(err);
        if (!code) return t('easydrag.ui.error_network');
        return tr(t, 'easydrag.ui.error_' + code.toLowerCase(), t('easydrag.ui.error_generic'));
    }

    // issueText translates validation issues by code; unknown codes keep the server text.
    function issueText(t, issue) {
        if (!issue) return '';
        return tr(t, 'easydrag.ui.issue_' + String(issue.code || '').toLowerCase(), issue.message || issue.code || '');
    }

    // stepErrorText describes a failed step: translated code plus the tool's own message.
    function stepErrorText(t, step) {
        if (!step || !step.error_code) return '';
        const code = tr(t, 'easydrag.ui.error_' + step.error_code.toLowerCase(), step.error_code);
        return step.error_message ? code + ': ' + step.error_message : code;
    }

    let modalCount = 0;

    // modal opens a dialog inside host. actions: [{id, label, primary, danger}].
    // onAction(id, dialog) may return false (keep open) or a promise; a rejected one keeps it open.
    // dismissible: false shows no close button and ignores Escape and backdrop clicks. cancel is the
    // id that Escape, the close button and a backdrop click resolve to (null without it).
    function modal(host, options) {
        const opts = options || {};
        const dismissible = opts.dismissible !== false;
        const cancelId = opts.cancel == null ? null : opts.cancel;
        const titleId = 'ed-modal-title-' + (++modalCount);
        const previous = document.activeElement;
        const overlay = el('<div class="ed-modal-backdrop" role="presentation"></div>');
        const actions = (opts.actions || []).map(a =>
            '<button type="button" class="ed-btn' + (a.primary ? ' ed-btn--primary' : '') + (a.danger ? ' ed-btn--danger' : '') +
            '" data-ed-action="' + esc(a.id) + '">' + (a.icon ? icon(a.icon) : '') + '<span>' + esc(a.label) + '</span></button>').join('');
        overlay.innerHTML = '<div class="ed-modal ' + esc(opts.className || '') + '" role="dialog" aria-modal="true" aria-labelledby="' + titleId + '">' +
            '<header class="ed-modal-head"><h2 class="ed-modal-title" id="' + titleId + '">' + esc(opts.title || '') + '</h2>' +
            (dismissible ? '<button type="button" class="ed-icon-btn" data-ed-action="close" aria-label="' + esc(opts.closeLabel || '') + '">' + icon('x') + '</button>' : '') + '</header>' +
            '<div class="ed-modal-body">' + (opts.body || '') + '</div>' +
            (actions ? '<footer class="ed-modal-foot">' + actions + '</footer>' : '') + '</div>';
        host.appendChild(overlay);
        let closed = false;
        let resolveDone;
        const done = new Promise(resolve => { resolveDone = resolve; });
        const dialog = {
            el: overlay.querySelector('.ed-modal'),
            body: overlay.querySelector('.ed-modal-body'),
            close(result) {
                if (closed) return;
                closed = true;
                overlay.classList.add('is-closing');
                setTimeout(() => overlay.remove(), 140);
                if (previous && typeof previous.focus === 'function') previous.focus();
                resolveDone(result);
            },
            done,
            setBusy(busy) { overlay.querySelectorAll('[data-ed-action]').forEach(b => { if (b.dataset.edAction !== 'close') b.disabled = !!busy; }); }
        };
        const dismiss = () => { if (dismissible) dialog.close(cancelId); };
        const focusFirst = () => {
            const focus = dialog.el.querySelector('[autofocus], .ed-modal-body input, .ed-modal-body textarea, .ed-btn--primary') || dialog.el.querySelector('button');
            if (focus) focus.focus({ preventScroll: true });
        };
        // A press on the backdrop must not move focus out of the dialog.
        overlay.addEventListener('mousedown', (event) => { if (event.target === overlay) event.preventDefault(); });
        overlay.addEventListener('click', async (event) => {
            if (event.target === overlay) {
                if (dismissible) dismiss();
                else if (!dialog.el.contains(document.activeElement)) focusFirst();
                return;
            }
            const button = event.target.closest('[data-ed-action]');
            if (!button) return;
            const id = button.dataset.edAction;
            if (id === 'close') { dismiss(); return; }
            if (!opts.onAction) { dialog.close(id); return; }
            dialog.setBusy(true);
            try {
                const keep = await opts.onAction(id, dialog);
                if (keep !== false) dialog.close(id);
            } catch (err) {
                console.error('EasyDrag dialog action failed', err);
            } finally {
                if (!closed) dialog.setBusy(false);
            }
        });
        overlay.addEventListener('keydown', (event) => {
            if (event.key === 'Escape') { event.stopPropagation(); dismiss(); return; }
            if (event.key !== 'Tab') return;
            const focusables = Array.from(dialog.el.querySelectorAll('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])')).filter(n => !n.disabled && n.offsetParent !== null);
            if (!focusables.length) return;
            const first = focusables[0];
            const last = focusables[focusables.length - 1];
            if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
            else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
        });
        focusFirst();
        requestAnimationFrame(() => overlay.classList.add('is-open'));
        return dialog;
    }

    // capturePointer follows one pointer on target. onEnd(event, cancelled) runs once: on pointerup
    // (not cancelled), or on pointercancel, lost capture or a mouse move with no button down, which
    // means the release was missed (cancelled). abort() ends it as cancelled; detach() ends it silently.
    function capturePointer(target, event, onMove, onEnd) {
        const id = event.pointerId;
        const types = ['pointerup', 'pointercancel', 'lostpointercapture'];
        let done = false;
        const move = ev => {
            if (ev.pointerId !== id) return;
            if (ev.pointerType === 'mouse' && ev.buttons === 0) finish(ev, true);
            else onMove(ev);
        };
        const end = ev => { if (ev.pointerId === id) finish(ev, ev.type !== 'pointerup'); };
        function detach() {
            done = true;
            target.removeEventListener('pointermove', move);
            types.forEach(type => target.removeEventListener(type, end));
        }
        function finish(ev, cancelled) {
            if (done) return;
            detach();
            onEnd(ev, cancelled);
        }
        try { target.setPointerCapture(id); } catch (err) { /* the pointer is already released */ }
        target.addEventListener('pointermove', move);
        types.forEach(type => target.addEventListener(type, end));
        return {
            abort() { finish({ type: 'abort', pointerId: id, clientX: event.clientX, clientY: event.clientY }, true); },
            detach
        };
    }

    // catOf returns the colour category of a node type (generic tools share "tool").
    function catOf(info) {
        if (!info) return 'tool';
        return String(info.category || '').startsWith('tool:') ? 'tool' : (info.category || 'tool');
    }

    ED.core = {
        ICONS, icon, esc, tr, clamp, debounce, frame, emitter, bag, el, isEditable, isMod, shortcut, IS_MAC,
        randomID, lang, fmt, storage, createApi, errorCode, errorText, issueText, stepErrorText, modal, capturePointer, catOf
    };
})();
