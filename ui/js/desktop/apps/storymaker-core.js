// Storymaker app core: the shared namespace window.StorymakerParts (S), the DOM builder S.el, the icon set, the cleanup bag,
// the event bus, formatters and the mapping from API errors to sentences. Everything that reaches the page goes through
// el() as text: there is deliberately no html option, no innerHTML and no template string with data in any storymaker file
// (a guard test greps for it). Loaded first; the other modules hang their parts on S. Classic script.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const SVG_NS = 'http://www.w3.org/2000/svg';

    // ---- DOM ------------------------------------------------------------------------------------------------------------

    function attrName(key) {
        return /^aria[A-Z]/.test(key) ? 'aria-' + key.slice(4).toLowerCase() : key;
    }

    function appendChildren(node, children) {
        children.forEach(child => {
            if (child == null || child === false || child === true) return;
            if (Array.isArray(child)) { appendChildren(node, child); return; }
            if (typeof child === 'string' || typeof child === 'number') { node.appendChild(document.createTextNode(String(child))); return; }
            node.appendChild(child);
        });
    }

    // el(tag, attrs, ...children): class, text (textContent), dataset, style (camelCase or --custom), on<Event> listeners,
    // aria<Name> and any other attribute; a false/null value omits the attribute, true sets it empty.
    function el(tag, attrs, ...children) {
        const node = document.createElement(tag);
        const a = attrs || {};
        Object.keys(a).forEach(key => {
            const value = a[key];
            if (value == null || value === false) return;
            if (key === 'html') throw new Error('S.el does not support html');
            if (key === 'class') { node.className = String(value); return; }
            if (key === 'text') { node.textContent = String(value); return; }
            if (key === 'dataset') {
                Object.keys(value).forEach(name => { if (value[name] != null) node.dataset[name] = String(value[name]); });
                return;
            }
            if (key === 'style') {
                Object.keys(value).forEach(name => {
                    if (name.startsWith('--')) node.style.setProperty(name, String(value[name]));
                    else node.style[name] = value[name];
                });
                return;
            }
            if (/^on[A-Z]/.test(key) && typeof value === 'function') { node.addEventListener(key.slice(2).toLowerCase(), value); return; }
            node.setAttribute(attrName(key), value === true ? '' : String(value));
        });
        appendChildren(node, children);
        return node;
    }

    function clear(node) {
        while (node.firstChild) node.removeChild(node.firstChild);
        return node;
    }

    // ---- icons: 24 x 24 strokes drawn with attributes, never with markup strings --------------------------------------------

    const ICONS = {
        play: { fill: true, d: ['M7 4.5v15l12-7.5z'] },
        pause: { d: ['M8 5v14', 'M16 5v14'] },
        stop: { fill: true, d: ['M7 7h10v10H7z'] },
        plus: { d: ['M12 5v14', 'M5 12h14'] },
        trash: { d: ['M4 7h16', 'M9 7V4h6v3', 'M6 7l1 13h10l1-13', 'M10 11v6', 'M14 11v6'] },
        up: { d: ['M6 15l6-6 6 6'] },
        down: { d: ['M6 9l6 6 6-6'] },
        left: { d: ['M15 5l-7 7 7 7'] },
        export: { d: ['M12 3v12', 'M7 10l5 5 5-5', 'M5 20h14'] },
        code: { d: ['M9 7l-5 5 5 5', 'M15 7l5 5-5 5'] },
        film: { d: ['M4 5h16v14H4z', 'M8 5v14', 'M16 5v14', 'M4 9h4', 'M4 15h4', 'M16 9h4', 'M16 15h4'] },
        user: { d: ['M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8z', 'M4 20c0-4 4-6 8-6s8 2 8 6'] },
        box: { d: ['M12 3l8 4.5v9L12 21l-8-4.5v-9z', 'M4 7.5l8 4.5 8-4.5', 'M12 12v9'] },
        scenery: { d: ['M3 19l6-10 4 6 2-3 6 7z', 'M16 6.5a1.5 1.5 0 1 0 0 .1'] },
        check: { d: ['M5 12.5l4.5 4.5L19 7'] },
        alert: { d: ['M12 4l9 16H3z', 'M12 10v4', 'M12 17v.5'] },
        info: { d: ['M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18z', 'M12 11v6', 'M12 7.5v.5'] },
        close: { d: ['M6 6l12 12', 'M18 6L6 18'] },
        refresh: { d: ['M20 11a8 8 0 1 0-2.3 6', 'M20 5v6h-6'] },
        folder: { d: ['M3 7h6l2 2h10v10H3z'] },
        copy: { d: ['M9 9h11v11H9z', 'M5 15V4h11'] },
        lock: { d: ['M6 11h12v9H6z', 'M9 11V8a3 3 0 0 1 6 0v3'] },
        sliders: { d: ['M4 7h10', 'M18 7h2', 'M4 17h2', 'M10 17h10', 'M14 5v4', 'M6 15v4'] },
        sparkle: { d: ['M12 3l2 6 6 2-6 2-2 6-2-6-6-2 6-2z'] }
    };

    function icon(name, className) {
        const spec = ICONS[name] || ICONS.info;
        const svg = document.createElementNS(SVG_NS, 'svg');
        svg.setAttribute('viewBox', '0 0 24 24');
        svg.setAttribute('class', 'stm-icon' + (className ? ' ' + className : ''));
        svg.setAttribute('aria-hidden', 'true');
        svg.setAttribute('focusable', 'false');
        svg.setAttribute('fill', spec.fill ? 'currentColor' : 'none');
        svg.setAttribute('stroke', 'currentColor');
        svg.setAttribute('stroke-width', '2');
        svg.setAttribute('stroke-linecap', 'round');
        svg.setAttribute('stroke-linejoin', 'round');
        spec.d.forEach(d => {
            const path = document.createElementNS(SVG_NS, 'path');
            path.setAttribute('d', d);
            svg.appendChild(path);
        });
        return svg;
    }

    // ---- lifecycle ----------------------------------------------------------------------------------------------------------

    function safeCall(fn) {
        try { fn(); } catch (err) { console.error('storymaker cleanup failed', err); }
    }

    // bag collects cleanups; dispose() runs them newest first, exactly once; anything added afterwards runs at once.
    function bag() {
        let fns = [];
        let disposed = false;
        const b = {
            add(fn) {
                if (typeof fn !== 'function') return fn;
                if (disposed) { safeCall(fn); return fn; }
                fns.push(fn);
                return fn;
            },
            on(target, type, fn, opts) {
                if (!target || typeof target.addEventListener !== 'function') return () => {};
                target.addEventListener(type, fn, opts);
                const off = () => target.removeEventListener(type, fn, opts);
                b.add(off);
                return off;
            },
            timeout(fn, ms) {
                const id = setTimeout(fn, ms);
                b.add(() => clearTimeout(id));
                return id;
            },
            interval(fn, ms) {
                const id = setInterval(fn, ms);
                b.add(() => clearInterval(id));
                return id;
            },
            dispose() {
                if (disposed) return;
                disposed = true;
                const list = fns.reverse();
                fns = [];
                list.forEach(safeCall);
            },
            get disposed() { return disposed; }
        };
        return b;
    }

    // bus: on(name, fn) returns the unsubscribe function; emit never throws into the emitter.
    function createBus() {
        const handlers = new Map();
        return {
            on(name, fn) {
                if (!handlers.has(name)) handlers.set(name, new Set());
                handlers.get(name).add(fn);
                return () => handlers.get(name) && handlers.get(name).delete(fn);
            },
            emit(name, payload) {
                (handlers.get(name) ? Array.from(handlers.get(name)) : []).forEach(fn => {
                    try { fn(payload); } catch (err) { console.error('storymaker handler failed', name, err); }
                });
            },
            clear() { handlers.clear(); }
        };
    }

    function debounce(fn, ms) {
        let timer = 0;
        const wrapped = (...args) => {
            clearTimeout(timer);
            timer = setTimeout(() => { timer = 0; fn(...args); }, ms);
        };
        wrapped.cancel = () => { clearTimeout(timer); timer = 0; };
        wrapped.pending = () => timer !== 0;
        return wrapped;
    }

    function clamp(value, min, max) {
        const n = Number(value);
        if (!Number.isFinite(n)) return min;
        return Math.min(max, Math.max(min, n));
    }

    function uuid() {
        if (typeof crypto !== 'undefined' && crypto && typeof crypto.randomUUID === 'function') return crypto.randomUUID();
        const bytes = new Uint8Array(16);
        if (typeof crypto !== 'undefined' && crypto && typeof crypto.getRandomValues === 'function') crypto.getRandomValues(bytes);
        else for (let i = 0; i < 16; i++) bytes[i] = Math.floor(Math.random() * 256);
        bytes[6] = (bytes[6] & 0x0f) | 0x40;
        bytes[8] = (bytes[8] & 0x3f) | 0x80;
        const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('');
        return hex.slice(0, 8) + '-' + hex.slice(8, 12) + '-' + hex.slice(12, 16) + '-' + hex.slice(16, 20) + '-' + hex.slice(20);
    }

    // ---- formatting ---------------------------------------------------------------------------------------------------------

    function locale() {
        const lang = (typeof window !== 'undefined' && window.SYSTEM_LANG) || (document.documentElement && document.documentElement.lang) || 'en';
        return String(lang || 'en');
    }

    const format = {
        // m:ss.t from a frame number
        clock(frame, fps) {
            const rate = Number(fps) > 0 ? Number(fps) : 30;
            const tenths = Math.max(0, Math.floor((Number(frame) || 0) / rate * 10));
            const minutes = Math.floor(tenths / 600);
            const seconds = Math.floor((tenths % 600) / 10);
            return minutes + ':' + String(seconds).padStart(2, '0') + '.' + (tenths % 10);
        },
        // seconds with one decimal at most, no trailing ".0"
        seconds(value) {
            const n = Math.round((Number(value) || 0) * 10) / 10;
            return String(n);
        },
        dateTime(iso) {
            const d = new Date(iso);
            if (!iso || Number.isNaN(d.getTime())) return '';
            try {
                return new Intl.DateTimeFormat(locale(), { dateStyle: 'medium', timeStyle: 'short' }).format(d);
            } catch (_) {
                return d.toISOString().slice(0, 16).replace('T', ' ');
            }
        },
        bytes(n) {
            const v = Number(n) || 0;
            if (v < 1024) return v + ' B';
            if (v < 1048576) return (v / 1024).toFixed(1) + ' KiB';
            return (v / 1048576).toFixed(1) + ' MiB';
        },
        percent(fraction) {
            return Math.round(clamp(fraction, 0, 1) * 100) + ' %';
        }
    };

    // The localized name of a catalog entry: its name_key when the language has it, else the bare name of the catalog.
    function catalogName(t, entry) {
        if (!entry) return '';
        const key = typeof entry.name_key === 'string' ? entry.name_key : '';
        if (key) {
            const text = t(key);
            if (text && text !== key) return text;
        }
        return String(entry.name || entry.id || '');
    }

    // ---- errors -------------------------------------------------------------------------------------------------------------

    // normalize turns whatever a request threw into {status, code, message, issues, network}. Server answers are flat:
    // {error, code, message, issues?}.
    function normalizeError(err) {
        if (err && err.stm) return err;
        const body = err && err.body && typeof err.body === 'object' ? err.body : {};
        const out = {
            stm: true,
            status: Number(err && err.status) || 0,
            code: String(body.code || body.error || (err && err.code) || ''),
            message: String(body.message || (err && err.message) || ''),
            issues: Array.isArray(body.issues) ? body.issues : [],
            network: !(err && err.status)
        };
        return out;
    }

    function errorFromResponse(status, body, retryAfter) {
        const b = body && typeof body === 'object' ? body : {};
        return {
            stm: true,
            status,
            code: String(b.code || b.error || ''),
            message: String(b.message || b.error || ''),
            issues: Array.isArray(b.issues) ? b.issues : [],
            retryAfter: Number(retryAfter) || 0,
            network: false
        };
    }

    // describe(t, err, context) is the sentence for an error; unknown ones say what happened in general terms and append the server's
    // own message as detail (shown as text only). context "render" is an export start: a 422 there is a refused export (the story or
    // a code scene has errors), not a refused save.
    function describeError(t, err, context) {
        const e = normalizeError(err);
        let key = '';
        if (e.network) key = 'storymaker.error.network';
        else if (e.status === 401 || e.code === 'unauthorized') key = 'storymaker.error.unauthorized';
        else if (e.code === 'desktop_scope_required') key = 'storymaker.error.scope';
        else if (e.code === 'desktop_readonly') key = 'storymaker.error.readonly';
        else if (e.code === 'storymaker_disabled') key = 'storymaker.error.disabled';
        else if (context === 'render' && (e.code === 'invalid_story' || e.code === 'code_error')) key = 'storymaker.job_error.' + e.code;
        else if (e.code === 'story_unreadable') key = 'storymaker.error.story_unreadable';
        else if (e.code === 'invalid_story' || e.status === 422) key = 'storymaker.error.invalid_story';
        else if (e.status === 412) key = 'storymaker.error.conflict';
        else if (e.status === 428) key = 'storymaker.error.required';
        else if (e.status === 413) key = 'storymaker.error.too_large';
        else if (e.status === 404) key = 'storymaker.error.not_found';
        else if (e.code === 'busy' || e.status === 429) key = 'storymaker.error.busy';
        // a refused render start answers with the code of the job error it would have ended in
        else if (JOB_ERROR_CODES.indexOf(e.code) >= 0 && e.code !== 'cancelled') key = 'storymaker.job_error.' + e.code;
        if (key) return t(key);
        const base = t('storymaker.error.generic');
        return e.message ? base + ' ' + e.message : base;
    }

    // The job error codes of the spec (Error handling) that a phase 1 render can end with.
    const JOB_ERROR_CODES = ['invalid_story', 'storymaker_disabled', 'browser_unavailable', 'ffmpeg_unavailable', 'render_failed', 'render_timeout', 'cancelled', 'busy', 'code_error'];

    function describeJobError(t, error) {
        const code = error && typeof error.code === 'string' ? error.code : '';
        if (JOB_ERROR_CODES.indexOf(code) >= 0) return t('storymaker.job_error.' + code);
        return t('storymaker.job_error.unknown');
    }

    const ISSUE_CODES = ['invalid_json', 'unknown_field', 'schema_mismatch', 'too_large', 'limit_exceeded', 'missing_field', 'invalid_value', 'duplicate_id', 'unknown_name', 'unknown_asset', 'unknown_spot', 'unknown_cast', 'unknown_cue', 'canonicalised', 'not_supported_yet', 'narration_missing', 'code_error', 'deadline', 'layout_unverified', 'line_unread', 'dead_stretch', 'first_frame_empty', 'text_too_small', 'fade_between_worlds', 'entry_monotony', 'transition_variety', 'cue_late'];

    function issueTitle(t, code) {
        if (ISSUE_CODES.indexOf(code) >= 0) return t('storymaker.issue.' + code);
        return t('storymaker.issue.unknown');
    }

    S.el = el;
    S.clear = clear;
    S.icon = icon;
    S.bag = bag;
    S.createBus = createBus;
    S.debounce = debounce;
    S.clamp = clamp;
    S.uuid = uuid;
    S.format = format;
    S.catalogName = catalogName;
    S.errors = { normalize: normalizeError, fromResponse: errorFromResponse, describe: describeError, describeJob: describeJobError, issueTitle };
    S.ISSUE_CODES = ISSUE_CODES;
    S.JOB_ERROR_CODES = JOB_ERROR_CODES;
    S.ICON_NAMES = Object.keys(ICONS);
})();
