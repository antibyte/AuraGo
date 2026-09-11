/**
 * Gallery library helpers: preferences, item metadata, formatting, filtering,
 * sorting, date grouping and diffing. Pure functions only; no DOM access apart
 * from reading the page locale. Consumed by gallery.js (window.GalleryApp).
 */
(function () {
    'use strict';

    const PREFS_KEY = 'aurago.desktop.gallery.v1';
    const TILE_SIZES = [104, 136, 168, 208, 256, 320];
    const DEFAULT_TILE_INDEX = 2;
    const TABS = ['Photos', 'Videos'];
    const SORTS = ['newest', 'oldest', 'name', 'size'];
    const TAB_KIND = { Photos: 'image', Videos: 'video' };
    const DAY_MS = 86400000;

    function loadPrefs() {
        try {
            const raw = window.localStorage.getItem(PREFS_KEY);
            const parsed = raw ? JSON.parse(raw) : {};
            return parsed && typeof parsed === 'object' ? parsed : {};
        } catch (_) {
            return {};
        }
    }

    function savePrefs(prefs) {
        try {
            window.localStorage.setItem(PREFS_KEY, JSON.stringify(prefs));
        } catch (_) { /* storage unavailable */ }
    }

    async function fetchJSON(url, options) {
        const response = await fetch(url, Object.assign({ credentials: 'same-origin' }, options || {}));
        let body = null;
        try { body = await response.json(); } catch (_) { body = null; }
        if (!response.ok) {
            const error = new Error((body && body.error) || response.statusText || String(response.status));
            error.status = response.status;
            throw error;
        }
        return body;
    }

    function itemTime(item) {
        const raw = item && (item.mod_time || item.modified || item.created);
        if (!raw) return 0;
        const time = new Date(raw).getTime();
        return Number.isFinite(time) ? time : 0;
    }

    function itemKind(item, tab) {
        const kind = String((item && item.media_kind) || '').toLowerCase();
        if (kind === 'image' || kind === 'video' || kind === 'audio') return kind;
        return TAB_KIND[tab] || 'image';
    }

    function formatDuration(seconds) {
        const total = Math.max(0, Math.round(Number(seconds) || 0));
        const hours = Math.floor(total / 3600);
        const minutes = Math.floor((total % 3600) / 60);
        const secs = total % 60;
        const pad = value => String(value).padStart(2, '0');
        return hours > 0 ? `${hours}:${pad(minutes)}:${pad(secs)}` : `${minutes}:${pad(secs)}`;
    }

    function pageLocale() {
        return document.documentElement.lang || navigator.language || 'en';
    }

    function formatDateTime(value) {
        const time = value ? new Date(value) : null;
        if (!time || !Number.isFinite(time.getTime())) return '';
        try {
            return new Intl.DateTimeFormat(pageLocale(), { dateStyle: 'medium', timeStyle: 'short' }).format(time);
        } catch (_) {
            return time.toLocaleString();
        }
    }

    function dirOf(path) {
        const parts = String(path || '').split('/');
        parts.pop();
        return parts.join('/');
    }

    function totalSize(items) {
        let total = 0;
        for (const item of items) total += Number(item.size) || 0;
        return total;
    }

    function queryTokens(query) {
        return String(query || '').toLowerCase().split(/\s+/).filter(Boolean);
    }

    function matchesQuery(item, tokens) {
        if (!tokens.length) return true;
        const haystack = `${item.name || ''} ${item.path || ''}`.toLowerCase();
        return tokens.every(token => haystack.includes(token));
    }

    function comparator(sort) {
        const byName = (a, b) => String(a.name).localeCompare(String(b.name));
        switch (sort) {
            case 'oldest': return (a, b) => (itemTime(a) - itemTime(b)) || byName(a, b);
            case 'name': return (a, b) => String(a.name).localeCompare(String(b.name), undefined, { numeric: true, sensitivity: 'base' });
            case 'size': return (a, b) => ((Number(b.size) || 0) - (Number(a.size) || 0)) || byName(a, b);
            default: return (a, b) => (itemTime(b) - itemTime(a)) || byName(a, b);
        }
    }

    function groupingAllowed(sort) {
        return sort === 'newest' || sort === 'oldest';
    }

    /**
     * Returns groupFor(item, now) which yields { key, label } for the date
     * section an item belongs to (today, yesterday, this week, month, month+year).
     */
    function createGrouper(t) {
        let monthFormatter = null;
        let monthYearFormatter = null;
        return function groupFor(item, now) {
            const time = itemTime(item);
            if (!time) return { key: 'unknown', label: t('desktop.gallery_group_unknown') };
            const date = new Date(time);
            const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
            const startOfDay = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
            const diffDays = Math.floor((startOfToday - startOfDay) / DAY_MS);
            if (diffDays <= 0) return { key: 'today', label: t('desktop.gallery_group_today') };
            if (diffDays === 1) return { key: 'yesterday', label: t('desktop.gallery_group_yesterday') };
            if (diffDays < 7) return { key: 'week', label: t('desktop.gallery_group_this_week') };
            const key = `m-${date.getFullYear()}-${date.getMonth()}`;
            try {
                if (date.getFullYear() === now.getFullYear()) {
                    if (!monthFormatter) monthFormatter = new Intl.DateTimeFormat(pageLocale(), { month: 'long' });
                    return { key, label: monthFormatter.format(date) };
                }
                if (!monthYearFormatter) monthYearFormatter = new Intl.DateTimeFormat(pageLocale(), { month: 'long', year: 'numeric' });
                return { key, label: monthYearFormatter.format(date) };
            } catch (_) {
                return { key, label: `${date.getMonth() + 1}/${date.getFullYear()}` };
            }
        };
    }

    /** Compares a fresh listing against the known items: { added, changed }. */
    function diffLibrary(previousByPath, fresh, freshByPath) {
        let added = 0;
        let changed = false;
        for (const item of fresh) {
            const old = previousByPath.get(item.path);
            if (!old) { added += 1; changed = true; continue; }
            if (old.size !== item.size || itemTime(old) !== itemTime(item)) changed = true;
        }
        if (!changed) {
            for (const path of previousByPath.keys()) {
                if (!freshByPath.has(path)) { changed = true; break; }
            }
        }
        return { added, changed };
    }

    /** Validates a new file name; returns the trimmed name or '' when unusable. */
    function cleanFileName(name, current) {
        const trimmed = String(name || '').trim();
        if (!trimmed || trimmed === current) return '';
        if (trimmed.includes('/') || trimmed.includes('\\')) return '';
        return trimmed;
    }

    /** Returns a renamed copy of an item (path, name and web_path updated). */
    function renamedItem(item, name) {
        const newPath = `${dirOf(item.path)}/${name}`;
        const updated = Object.assign({}, item, { name, path: newPath });
        if (item.web_path) updated.web_path = String(item.web_path).replace(/[^/]*$/, encodeURIComponent(name).replace(/%2F/gi, '/'));
        return updated;
    }

    /** Picks the `<key>_one` singular variant for exactly one item, otherwise the plural key. */
    function countLabel(t, key, count, vars) {
        const n = Number(count) || 0;
        return t(n === 1 ? `${key}_one` : key, Object.assign({ count: n }, vars || {}));
    }

    window.GalleryLibrary = {
        TILE_SIZES,
        countLabel,
        DEFAULT_TILE_INDEX,
        TABS,
        SORTS,
        TAB_KIND,
        loadPrefs,
        savePrefs,
        fetchJSON,
        itemTime,
        itemKind,
        formatDuration,
        formatDateTime,
        pageLocale,
        dirOf,
        totalSize,
        queryTokens,
        matchesQuery,
        comparator,
        groupingAllowed,
        createGrouper,
        diffLibrary,
        cleanFileName,
        renamedItem
    };
})();
