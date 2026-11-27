// Chibi Sensei app core: the shared namespace window.ChibiSenseiParts (P), the DOM builder P.el, the icon set, the cleanup bag,
// the event bus, formatters, the mapping from API errors to sentences and the constant lists the other modules read. Everything
// that reaches the page goes through el() as text: there is deliberately no html option, no innerHTML and no template string in
// any chibi-sensei file (a guard test greps for it). The file touches neither document nor fetch at load time, so a Node test can
// load it in a vm. Loaded first; the other modules hang their parts on P. Classic script.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};
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
            if (key === 'html') throw new Error('P.el does not support html');
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
        plus: { d: ['M12 5v14', 'M5 12h14'] },
        trash: { d: ['M4 7h16', 'M9 7V4h6v3', 'M6 7l1 13h10l1-13', 'M10 11v6', 'M14 11v6'] },
        edit: { d: ['M4 20h4l11-11-4-4L4 16z', 'M13.5 6.5l4 4'] },
        upload: { d: ['M12 16V4', 'M7 9l5-5 5 5', 'M5 20h14'] },
        file: { d: ['M6 3h8l4 4v14H6z', 'M14 3v4h4', 'M9 13h6', 'M9 17h6'] },
        sparkle: { d: ['M12 3l2 6 6 2-6 2-2 6-2-6-6-2 6-2z'] },
        broom: { d: ['M14 4l6 6', 'M17 7l-7 7', 'M10 14l-6 2 2 4 6-2z', 'M7 17l2 2'] },
        export: { d: ['M12 3v12', 'M7 10l5 5 5-5', 'M5 20h14'] },
        check: { d: ['M5 12.5l4.5 4.5L19 7'] },
        alert: { d: ['M12 4l9 16H3z', 'M12 10v4', 'M12 17v.5'] },
        info: { d: ['M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18z', 'M12 11v6', 'M12 7.5v.5'] },
        close: { d: ['M6 6l12 12', 'M18 6L6 18'] },
        refresh: { d: ['M20 11a8 8 0 1 0-2.3 6', 'M20 5v6h-6'] },
        undo: { d: ['M9 7L4 12l5 5', 'M4 12h10a5 5 0 0 1 0 10h-3'] },
        history: { d: ['M4 12a8 8 0 1 0 2.5-5.8', 'M4 4v4h4', 'M12 8v4l3 2'] },
        left: { d: ['M15 5l-7 7 7 7'] },
        right: { d: ['M9 5l7 7-7 7'] },
        lock: { d: ['M6 11h12v9H6z', 'M9 11V8a3 3 0 0 1 6 0v3'] },
        search: { d: ['M11 18a7 7 0 1 0 0-14 7 7 0 0 0 0 14z', 'M16 16l5 5'] },
        cube: { d: ['M12 3l8 4.5v9L12 21l-8-4.5v-9z', 'M4 7.5l8 4.5 8-4.5', 'M12 12v9'] }
    };

    function icon(name, className) {
        const spec = ICONS[name] || ICONS.info;
        const svg = document.createElementNS(SVG_NS, 'svg');
        svg.setAttribute('viewBox', '0 0 24 24');
        svg.setAttribute('class', 'csn-icon' + (className ? ' ' + className : ''));
        svg.setAttribute('aria-hidden', 'true');
        svg.setAttribute('focusable', 'false');
        svg.setAttribute('fill', 'none');
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
        try { fn(); } catch (err) { console.error('chibi-sensei cleanup failed', err); }
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
                    try { fn(payload); } catch (err) { console.error('chibi-sensei handler failed', name, err); }
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

    // ---- formatting ---------------------------------------------------------------------------------------------------------

    function locale() {
        const lang = (typeof window !== 'undefined' && window.SYSTEM_LANG) || (typeof document !== 'undefined' && document.documentElement && document.documentElement.lang) || 'en';
        return String(lang || 'en');
    }

    const format = {
        number(n) {
            const v = Number(n);
            if (!Number.isFinite(v)) return '0';
            try {
                return new Intl.NumberFormat(locale()).format(v);
            } catch (_) {
                return String(v);
            }
        },
        // whole minutes; under one minute says 1
        minutes(n) {
            const v = Math.max(0, Math.round(Number(n) || 0));
            return format.number(Math.max(1, v));
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
        // a fraction 0..1 as a whole percent
        share(fraction) {
            return Math.round(clamp(fraction, 0, 1) * 100) + ' %';
        },
        // seconds as "12 s", "4 min" or "1 h 5 min" in the user's language
        duration(seconds) {
            const s = Math.max(0, Math.round(Number(seconds) || 0));
            const unit = (value, name) => {
                try { return new Intl.NumberFormat(locale(), { style: 'unit', unit: name, unitDisplay: 'short' }).format(value); } catch (_) { return String(value) + ' ' + name; }
            };
            if (s < 60) return unit(s, 'second');
            const m = Math.floor(s / 60);
            return m < 60 ? unit(m, 'minute') : unit(Math.floor(m / 60), 'hour') + ' ' + unit(m % 60, 'minute');
        },
        bytes(n) {
            const units = ['byte', 'kilobyte', 'megabyte', 'gigabyte'];
            let x = Math.max(0, Number(n) || 0), i = 0;
            while (x >= 1024 && i < units.length - 1) { x /= 1024; i++; }
            try { return new Intl.NumberFormat(locale(), { style: 'unit', unit: units[i], unitDisplay: 'short', maximumFractionDigits: i ? 1 : 0 }).format(x); } catch (_) { return Math.round(x) + ' ' + units[i]; }
        },
        decimal(n, digits) {
            const v = Number(n);
            if (!Number.isFinite(v)) return '0';
            try { return new Intl.NumberFormat(locale(), { maximumFractionDigits: digits == null ? 3 : digits }).format(v); } catch (_) { return String(v); }
        }
    };

    // ---- errors -------------------------------------------------------------------------------------------------------------

    // normalize turns whatever a request threw into {status, code, message, issues, network}. Server answers are flat:
    // {error, code, message, issues?}.
    function normalizeError(err) {
        if (err && err.csn) return err;
        const body = err && err.body && typeof err.body === 'object' ? err.body : {};
        return {
            csn: true,
            status: Number(err && err.status) || 0,
            code: String(body.code || body.error || (err && err.code) || ''),
            message: String(body.message || (err && err.message) || ''),
            issues: Array.isArray(body.issues) ? body.issues : [],
            data: body,
            network: !(err && err.status)
        };
    }

    function errorFromResponse(status, body, retryAfter) {
        const b = body && typeof body === 'object' ? body : {};
        return {
            csn: true,
            status,
            code: String(b.code || b.error || ''),
            message: String(b.message || b.error || ''),
            issues: Array.isArray(b.issues) ? b.issues : [],
            retryAfter: Number(retryAfter) || 0,
            data: b,
            network: false
        };
    }

    const ERROR_CODES = ['network', 'unauthorized', 'desktop_scope_required', 'desktop_readonly', 'chibi_disabled', 'desktop_disabled', 'not_found', 'conflict', 'validation_failed', 'dataset_full', 'import_failed', 'job_active', 'not_archived', 'budget_exceeded', 'provider_unavailable', 'too_large', 'busy', 'precondition_required', 'confirmation_required', 'confirmation_invalid', 'generator_daily_limit', 'storage_full', 'data_changed', 'already_undone', 'file_too_large', 'unsupported_file', 'profile_unknown', 'job_not_found', 'undo_conflict', 'bad_response', 'prompt_withheld', 'output_cut', 'invalid_answer', 'model_error', 'interrupted', 'internal', 'too_many_rows', 'job_not_finished', 'timeout', 'too_many_streams', 'preflight_failed', 'plan_expired', 'run_active', 'run_not_terminal', 'run_not_found', 'artifact_not_found', 'admin_required', 'kaggle_unavailable', 'license_consent_required', 'runtime_unavailable', 'runtime_not_installed', 'runtime_busy', 'model_missing', 'run_not_succeeded', 'samples_missing'];
    const CHECK_IDS = ['count_min', 'structure', 'short_answer', 'duplicates', 'conflicts', 'too_long', 'label_balance', 'json_valid', 'split_sizes', 'language_fit'];

    function describeError(t, err) {
        const e = normalizeError(err);
        let code = '';
        if (e.network) code = 'network';
        else if (e.status === 401 || e.code === 'unauthorized') code = 'unauthorized';
        else if (ERROR_CODES.indexOf(e.code) >= 0) code = e.code;
        else if (e.status === 412) code = 'conflict';
        else if (e.status === 422) code = 'validation_failed';
        else if (e.status === 404) code = 'not_found';
        else if (e.status === 413) code = 'too_large';
        else if (e.status === 429) code = 'busy';
        if (code) return t('chibi_sensei.error.' + code);
        const base = t('chibi_sensei.error.generic');
        return e.message ? base + ' ' + e.message.slice(0, 200) : base;
    }

    // A job that the server no longer knows (404 or job_not_found) leaves the map; true when it was dropped.
    function pruneMissingJob(jobs, id, err) {
        const e = normalizeError(err);
        if (e.status !== 404 && e.code !== 'job_not_found') return false;
        return jobs.delete(id);
    }

    // A server-issued key is used only when the locale has it; otherwise the fallback key.
    function knownKey(t, key, fallback, vars) {
        const text = typeof key === 'string' && key.indexOf('chibi_sensei.') === 0 ? t(key, vars) : '';
        return text && text !== key ? text : t(fallback, vars);
    }

    function checkText(t, check) {
        const c = check || {};
        const id = CHECK_IDS.indexOf(c.id) >= 0 ? c.id : 'unknown';
        return t('chibi_sensei.check.' + id, { value: format.number(c.value), threshold: format.number(c.threshold), count: format.number(c.row_count) });
    }

    // ---- constant lists and literal key maps (literal keys stay scannable by the guard test) ---------------------------------

    const STEP_IDS = ['goal', 'data', 'training', 'test', 'use'];
    const READINESS = ['todo', 'in_progress', 'ready', 'done', 'blocked'];
    const RECIPES = ['assistant', 'specialist', 'free'];
    const TASK_KINDS = ['classify', 'extract', 'rewrite', 'to_json'];
    const RECIPE_KEYS = {
        assistant: 'chibi_sensei.recipe.assistant',
        specialist: 'chibi_sensei.recipe.specialist',
        free: 'chibi_sensei.recipe.free'
    };
    const STEP_KEYS = {
        goal: 'chibi_sensei.step.goal',
        data: 'chibi_sensei.step.data',
        training: 'chibi_sensei.step.training',
        test: 'chibi_sensei.step.test',
        use: 'chibi_sensei.step.use'
    };

    function recipeKey(id) { return RECIPE_KEYS[id] || RECIPE_KEYS.assistant; }
    function stepKey(id) { return STEP_KEYS[id] || STEP_KEYS.goal; }

    // need(inst, name) is true when the part P[name] is loaded. A stale cached module set leaves a part out; then the window shows
    // the "part missing" card (through inst.showState when the root offers it) and the caller stops.
    function need(inst, name) {
        if (P[name]) return true;
        const t = inst && typeof inst.t === 'function' ? inst.t : key => key;
        const card = el('div', { class: 'csn-state', role: 'status', dataset: { csn: 'state', state: 'part_missing' } },
            icon('alert', 'csn-state-icon'),
            el('h2', { text: t('chibi_sensei.state.part_missing_title') }),
            el('p', { text: t('chibi_sensei.state.part_missing_text') }));
        if (inst && typeof inst.showState === 'function') inst.showState(card);
        else if (inst && inst.host) { clear(inst.host); inst.host.appendChild(card); }
        return false;
    }

    // ---- runs ------------------------------------------------------------------------------------------------------------------
    const RUN_STATES = ['queued', 'packaging', 'uploading', 'creating_dataset', 'waiting_dataset', 'starting', 'running', 'downloading', 'verifying', 'cleaning', 'succeeded', 'failed', 'cancelled'];
    const RUN_TERMINAL = ['succeeded', 'failed', 'cancelled'];
    function runTerminal(state) { return RUN_TERMINAL.indexOf(state) >= 0; }
    // runs are newest first; an unknown state is neither active nor terminal, so it never keeps a poll alive.
    function activeRun(runs) {
        return (Array.isArray(runs) ? runs : []).find(r => r && RUN_STATES.indexOf(r.state) >= 0 && !runTerminal(r.state)) || null;
    }
    // upsertRun merges a (possibly partial) run into the list: fields the update lacks are kept, the list is newest first.
    function upsertRun(runs, run, limit) {
        const list = Array.isArray(runs) ? runs : [];
        const prev = list.find(r => r && r.id === run.id);
        const next = list.filter(r => r && r.id !== run.id);
        next.push(prev ? Object.assign({}, prev, run) : run);
        next.sort((a, b) => String(b.created || '').localeCompare(String(a.created || '')));
        return next.slice(0, limit || 20);
    }

    // ---- links: never an embedded outside resource; opened on demand in a new tab without referrer -----------------------
    function safeLink(url) {
        const s = String(url || '').trim();
        if (s.charAt(0) === '/' && s.charAt(1) !== '/' && s.charAt(1) !== '\\') return s;
        try { const u = new URL(s); return u.protocol === 'https:' ? u.href : ''; } catch (_) { return ''; }
    }
    function openLink(url) {
        const safe = safeLink(url);
        if (safe) window.open(safe, '_blank', 'noopener,noreferrer');
        return !!safe;
    }
    // The address as selectable read-only text, a copy button and an open button (disabled for anything but https or a path).
    function linkRow(t, url, name) {
        const safe = safeLink(url);
        const input = el('input', { class: 'csn-input csn-link-text', type: 'text', readonly: true, value: String(url || ''), ariaLabel: t('chibi_sensei.link.address'), dataset: { csn: name + '-url' } });
        const note = el('span', { class: 'csn-hint', role: 'status', dataset: { csn: name + '-copied' } });
        const copy = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: name + '-copy' }, text: t('chibi_sensei.link.copy'), onClick: async () => {
            let ok = false;
            try { if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') { await navigator.clipboard.writeText(input.value); ok = true; } } catch (_) { ok = false; }
            if (!ok) { input.focus(); input.select(); }
            note.textContent = t(ok ? 'chibi_sensei.link.copied' : 'chibi_sensei.link.copy_failed');
        } });
        const open = el('button', { type: 'button', class: 'csn-btn', dataset: { csn: name + '-open' }, text: t('chibi_sensei.link.open'), onClick: () => openLink(url) });
        open.disabled = !safe;
        open.title = safe ? '' : t('chibi_sensei.link.blocked');
        return el('div', { class: 'csn-linkrow', dataset: { csn: name } }, input, copy, open, note);
    }

    // ---- test and chat (P3) ------------------------------------------------------------------------------------------------------
    const CHAT_TARGETS = ['tuned', 'base'];
    const RUNTIME_STATES = ['disabled', 'unsupported', 'not_installed', 'installing', 'ready', 'failed'];
    const SAMPLE_FILTERS = ['all', 'different', 'wrong'];
    // Limits of POST /runtime/chat; GET /meta "chat" overrides them when present.
    const CHAT_LIMITS = Object.freeze({ max_targets: 2, max_turns: 20, max_chars: 16000, max_tokens_cap: 1024 });
    function chatLimits(meta) {
        const m = meta && meta.chat && typeof meta.chat === 'object' ? meta.chat : {};
        const pick = (name) => (Number.isInteger(m[name]) && m[name] > 0 ? m[name] : CHAT_LIMITS[name]);
        return { max_targets: pick('max_targets'), max_turns: pick('max_turns'), max_chars: pick('max_chars'), max_tokens_cap: pick('max_tokens_cap') };
    }
    // Characters as the server counts them (Unicode code points, Go's utf8.RuneCountInString).
    function charCount(text) { return Array.from(String(text || '')).length; }

    P.el = el;
    P.clear = clear;
    P.icon = icon;
    P.bag = bag;
    P.createBus = createBus;
    P.debounce = debounce;
    P.clamp = clamp;
    P.format = format;
    P.errors = { normalize: normalizeError, fromResponse: errorFromResponse, describe: describeError, knownKey };
    P.checkText = checkText;
    P.ERROR_CODES = ERROR_CODES;
    P.pruneMissingJob = pruneMissingJob;
    P.CHECK_IDS = CHECK_IDS;
    P.UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
    P.STEP_IDS = STEP_IDS;
    P.READINESS = READINESS;
    P.RECIPES = RECIPES;
    P.TASK_KINDS = TASK_KINDS;
    P.recipeKey = recipeKey;
    P.stepKey = stepKey;
    P.need = need;
    P.ICON_NAMES = Object.keys(ICONS);
    P.RUN_STATES = RUN_STATES;
    P.runTerminal = runTerminal;
    P.activeRun = activeRun;
    P.upsertRun = upsertRun;
    P.safeLink = safeLink;
    P.openLink = openLink;
    P.linkRow = linkRow;
    P.CHAT_TARGETS = CHAT_TARGETS;
    P.RUNTIME_STATES = RUNTIME_STATES;
    P.SAMPLE_FILTERS = SAMPLE_FILTERS;
    P.CHAT_LIMITS = CHAT_LIMITS;
    P.chatLimits = chatLimits;
    P.charCount = charCount;
})();
