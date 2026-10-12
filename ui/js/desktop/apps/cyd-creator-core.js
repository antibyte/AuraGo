// CYD Creator core: the shared namespace window.CYDCreatorParts (P), DOM and lifecycle helpers
// (P.util), translations (P.i18n), error texts (P.errors), the per-window event bus, the autosave
// state machine (P.autosave) and the in-window dialog (P.dialog). Pure parts run in Node.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const KEY_PREFIX = 'cyd_creator.';

    // ── P.util ──────────────────────────────────────────────────────────────

    function attrName(key) {
        if (/^aria[A-Z]/.test(key)) return 'aria-' + key.slice(4).toLowerCase();
        return key;
    }

    function appendChildren(node, children) {
        children.forEach(child => {
            if (child == null || child === false || child === true) return;
            if (Array.isArray(child)) { appendChildren(node, child); return; }
            if (typeof child === 'string' || typeof child === 'number') { node.appendChild(document.createTextNode(String(child))); return; }
            node.appendChild(child);
        });
    }

    // el(tag, attrs, ...children) builds an element. attrs: class, text (textContent), dataset,
    // style (camelCase or --custom-properties), on<Event> listeners, aria*/other attributes. There is
    // deliberately no html key: every dynamic string reaches the DOM as text.
    function el(tag, attrs, ...children) {
        const node = document.createElement(tag);
        const a = attrs || {};
        Object.keys(a).forEach(key => {
            const value = a[key];
            if (value == null || value === false) return;
            if (key === 'html') throw new Error('P.util.el does not support html');
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

    function safeCall(fn) {
        try { fn(); } catch (err) { console.error('cyd-creator cleanup failed', err); }
    }

    // bag collects cleanups; dispose() runs them newest first, exactly once.
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
            interval(fn, ms) {
                const id = setInterval(fn, ms);
                b.add(() => clearInterval(id));
                return id;
            },
            timeout(fn, ms) {
                const id = setTimeout(fn, ms);
                b.add(() => clearTimeout(id));
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

    function debounce(fn, ms) {
        let timer = 0;
        let lastArgs = null;
        let lastThis = null;
        function run() {
            timer = 0;
            const args = lastArgs || [];
            lastArgs = null;
            return fn.apply(lastThis, args);
        }
        function wrapped(...args) {
            lastArgs = args;
            lastThis = this;
            clearTimeout(timer);
            timer = setTimeout(run, ms);
        }
        wrapped.cancel = () => { clearTimeout(timer); timer = 0; lastArgs = null; };
        wrapped.flush = () => {
            if (!timer) return undefined;
            clearTimeout(timer);
            return run();
        };
        return wrapped;
    }

    function clamp(v, min, max) {
        const n = Number(v);
        if (!Number.isFinite(n)) return min;
        return Math.min(max, Math.max(min, n));
    }

    // uid returns prefix_<12 lowercase hex>, the shape of the Go RandomIDs.
    function uid(prefix) {
        const bytes = new Uint8Array(6);
        const c = (typeof crypto !== 'undefined' && crypto && typeof crypto.getRandomValues === 'function') ? crypto : null;
        if (c) c.getRandomValues(bytes);
        else for (let i = 0; i < bytes.length; i++) bytes[i] = Math.floor(Math.random() * 256);
        const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('');
        return String(prefix) + '_' + hex;
    }

    function escapeText(value) {
        return String(value == null ? '' : value)
            .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
    }

    function deepClone(value) {
        if (value === undefined) return undefined;
        if (typeof structuredClone === 'function') return structuredClone(value);
        return JSON.parse(JSON.stringify(value));
    }

    function deepFreeze(value) {
        if (value && typeof value === 'object' && !Object.isFrozen(value)) {
            Object.freeze(value);
            Object.keys(value).forEach(key => deepFreeze(value[key]));
        }
        return value;
    }

    P.util = { el, bag, debounce, clamp, uid, escapeText, deepClone, deepFreeze };

    // ── formatting ──────────────────────────────────────────────────────────

    function locale() {
        const lang = (typeof window !== 'undefined' && window.SYSTEM_LANG) || (typeof document !== 'undefined' && document.documentElement && document.documentElement.lang) || 'en';
        return String(lang || 'en');
    }

    P.format = {
        bytes(n) {
            const v = Number(n) || 0;
            if (v < 1024) return v + ' B';
            if (v < 1024 * 1024) return (v / 1024).toFixed(1) + ' KiB';
            return (v / 1024 / 1024).toFixed(2) + ' MiB';
        },
        percent(used, total) {
            const u = Number(used) || 0;
            const t = Number(total) || 0;
            if (t <= 0) return '';
            return Math.round((u / t) * 100) + ' %';
        },
        time(iso) {
            const d = new Date(iso);
            if (!iso || Number.isNaN(d.getTime())) return '';
            try {
                return new Intl.DateTimeFormat(locale(), { dateStyle: 'short', timeStyle: 'short' }).format(d);
            } catch (_) {
                return d.toISOString().slice(0, 16).replace('T', ' ');
            }
        },
        duration(ms) {
            const s = Math.max(0, Math.round((Number(ms) || 0) / 1000));
            const m = Math.floor(s / 60);
            return m > 0 ? m + ':' + String(s % 60).padStart(2, '0') + ' min' : s + ' s';
        }
    };

    // ── translations ────────────────────────────────────────────────────────

    // P.i18n(inst) returns t(key, vars). Keys may be written with or without the "cyd_creator."
    // prefix; a missing key renders as its full name so tests can spot it. vars fill both
    // "{{name}}" (shell strings) and "{name}" (designer strings of plan 4b). t.or(key, fallback,
    // vars) renders fallback when the key is missing, for families another plan owns.
    P.i18n = function i18n(inst) {
        void inst;
        function full(key) {
            const k = String(key || '');
            return k.startsWith(KEY_PREFIX) ? k : KEY_PREFIX + k;
        }
        function lookup(key) {
            const dict = (typeof window !== 'undefined' && window.I18N) || {};
            const name = full(key);
            return Object.prototype.hasOwnProperty.call(dict, name) ? String(dict[name]) : null;
        }
        // fill replaces the placeholders in one pass, so a value (a file name, a compiler message from
        // the server) is never read as a placeholder itself; unknown placeholders stay.
        function fill(text, vars) {
            if (!vars) return text;
            return text.replace(/\{\{(\w+)\}\}|\{(\w+)\}/g, (match, double, single) => {
                const name = double || single;
                if (!Object.prototype.hasOwnProperty.call(vars, name)) return match;
                return vars[name] == null ? '' : String(vars[name]);
            });
        }
        function t(key, vars) {
            const text = lookup(key);
            return fill(text == null ? full(key) : text, vars);
        }
        t.has = key => lookup(key) != null;
        t.or = (key, fallback, vars) => {
            const text = lookup(key);
            return fill(text == null ? String(fallback == null ? '' : fallback) : text, vars);
        };
        t.lang = locale;
        return t;
    };

    // ── errors ──────────────────────────────────────────────────────────────

    P.errors = {
        code(err) {
            return err && typeof err.code === 'string' ? err.code : '';
        },
        // reason is details.reason of a typed error (CYD_BUSY: file_open, publication_review).
        reason(err) {
            const details = err && err.details;
            return details && typeof details.reason === 'string' ? details.reason : '';
        },
        // message returns the translated sentence for err: error.<CODE> when the code is known,
        // error.network when the request never reached the server, else error.generic. A project
        // fenced after an interrupted publication (CYD_BUSY publication_review) has its own text.
        message(t, err) {
            const code = P.errors.code(err);
            if (code === 'CYD_BUSY' && P.errors.reason(err) === 'publication_review') return t('cyd_creator.error.publication_review');
            if (code && typeof t.has === 'function' && t.has('cyd_creator.error.' + code)) return t('cyd_creator.error.' + code);
            if (err && err.name === 'ApiError' && Number(err.status) === 0) return t('cyd_creator.error.network');
            return t('cyd_creator.error.generic');
        }
    };

    // ── event bus ───────────────────────────────────────────────────────────

    P.createBus = function createBus() {
        const handlers = new Map();
        function off(type, fn) {
            const list = handlers.get(type);
            if (!list) return;
            const index = list.indexOf(fn);
            if (index >= 0) list.splice(index, 1);
        }
        return {
            on(type, fn) {
                if (!handlers.has(type)) handlers.set(type, []);
                handlers.get(type).push(fn);
                return () => off(type, fn);
            },
            off,
            emit(type, ...args) {
                (handlers.get(type) || []).slice().forEach(fn => {
                    try { fn(...args); } catch (err) { console.error('cyd-creator bus handler failed', type, err); }
                });
            },
            clear() { handlers.clear(); }
        };
    };

    // ── autosave ────────────────────────────────────────────────────────────

    // classifySaveError maps a failed save to the saver state that follows it. CYD_BUSY retries only
    // for reason file_open (an open file blocked a rename) or no reason; publication_review fences the
    // project until an administrator resolves an interrupted publication, so retrying never lands.
    function classifySaveError(err) {
        const status = Number(err && err.status) || 0;
        const code = (err && err.code) || '';
        if (code === 'CYD_PROJECT_CONFLICT' || code === 'CYD_FILE_CONFLICT') return 'conflict';
        if (code === 'CYD_PROJECT_LOCKED' || status === 423) return 'locked';
        if (code === 'CYD_BUSY') {
            const reason = P.errors.reason(err);
            return !reason || reason === 'file_open' ? 'offline' : 'failed';
        }
        if (status === 0 || status === 408 || status === 429 || status >= 500) return 'offline';
        return 'failed';
    }

    // P.autosave.create(o) serialises saves of one document. o:
    //   token        the server version the first save sends (If-Match)
    //   save(payload, token) → Promise<newToken>
    //   onState(state, error)   saved | dirty | saving | offline | failed | conflict | locked
    //   onConflict(error) → Promise<{action: 'reload'|'overwrite'|'keep', token?}>
    //   delay (1000), retryMs (5000), retryMaxMs (60000), timers {setTimeout, clearTimeout}
    // A change saves after `delay`; changes made while a request runs follow right after it.
    // offline retries with doubling backoff; failed waits for the next change or flush(); locked
    // waits for unlock(); a kept conflict saves again only on flush(). reset(token) drops local
    // changes after the caller reloaded the server copy.
    function createAutosave(options) {
        const o = options || {};
        const timers = o.timers || { setTimeout: (fn, ms) => setTimeout(fn, ms), clearTimeout: id => clearTimeout(id) };
        const delay = o.delay == null ? 1000 : o.delay;
        const retryBase = o.retryMs || 5000;
        const retryMax = o.retryMaxMs || 60000;
        let token = o.token;
        let latest;
        let latestSeq = 0;
        let savedSeq = 0;
        let state = 'saved';
        let error = null;
        let timer = null;
        let retryTimer = null;
        let retryDelay = retryBase;
        let inFlight = null;
        // conflictPending is the outcome of the open conflict choice. Until it is made, flush() (Ctrl+S,
        // closing the window) waits for it instead of sending the stale version again, which would
        // only stack a second conflict dialog.
        let conflictPending = null;
        let disposed = false;

        function setState(next) {
            if (state === next) return;
            state = next;
            if (typeof o.onState === 'function') {
                try { o.onState(next, error); } catch (err) { console.error('cyd-creator autosave onState failed', err); }
            }
        }
        function clearTimers() {
            if (timer) timers.clearTimeout(timer);
            if (retryTimer) timers.clearTimeout(retryTimer);
            timer = null;
            retryTimer = null;
        }
        function dirty() { return latestSeq !== savedSeq; }

        function change(payload) {
            if (disposed) return;
            latest = payload;
            latestSeq++;
            if (state === 'locked' || state === 'conflict' || inFlight) return;
            setState('dirty');
            if (timer) timers.clearTimeout(timer);
            timer = timers.setTimeout(() => { timer = null; save(); }, delay);
        }

        function save() {
            if (disposed) return Promise.resolve(false);
            if (conflictPending) return conflictPending;
            if (inFlight) return inFlight;
            clearTimers();
            if (!dirty()) { setState('saved'); return Promise.resolve(true); }
            const seq = latestSeq;
            const payload = latest;
            setState('saving');
            inFlight = Promise.resolve()
                .then(() => o.save(payload, token))
                .then(newToken => {
                    inFlight = null;
                    if (disposed) return false;
                    token = newToken;
                    savedSeq = seq;
                    error = null;
                    retryDelay = retryBase;
                    if (typeof o.onSaved === 'function') {
                        try { o.onSaved(token); } catch (err) { console.error('cyd-creator autosave onSaved failed', err); }
                    }
                    if (dirty()) return save();
                    setState('saved');
                    return true;
                }, err => {
                    inFlight = null;
                    if (disposed) return false;
                    error = err;
                    const kind = classifySaveError(err);
                    if (kind === 'conflict') return conflict(err);
                    if (kind === 'locked') { setState('locked'); return false; }
                    if (kind === 'offline') {
                        setState('offline');
                        retryTimer = timers.setTimeout(() => { retryTimer = null; save(); }, retryDelay);
                        retryDelay = Math.min(retryDelay * 2, retryMax);
                        return false;
                    }
                    setState('failed');
                    return false;
                });
            return inFlight;
        }

        function conflict(err) {
            setState('conflict');
            const outcome = resolveConflict(err);
            conflictPending = outcome;
            outcome.then(() => { if (conflictPending === outcome) conflictPending = null; });
            return outcome;
        }

        async function resolveConflict(err) {
            let choice = { action: 'keep' };
            if (typeof o.onConflict === 'function') {
                try { choice = (await o.onConflict(err)) || choice; } catch (e) { console.error('cyd-creator autosave onConflict failed', e); }
            }
            // The choice is made: saves run again (Overwrite saves right here).
            conflictPending = null;
            if (disposed) return false;
            if (choice.action === 'reload') {
                reset(choice.token === undefined ? token : choice.token);
                return true;
            }
            if (choice.action === 'overwrite') {
                if (choice.token !== undefined) token = choice.token;
                return save();
            }
            return false;
        }

        function flush() {
            if (disposed) return Promise.resolve(false);
            if (conflictPending) return conflictPending;
            if (timer) { timers.clearTimeout(timer); timer = null; }
            if (state === 'locked') return Promise.resolve(false);
            if (inFlight) return inFlight.then(() => flush());
            if (!dirty()) { setState('saved'); return Promise.resolve(true); }
            return save();
        }

        function unlock() {
            if (state !== 'locked') return;
            if (dirty()) save(); else setState('saved');
        }

        function reset(newToken) {
            clearTimers();
            token = newToken;
            savedSeq = latestSeq;
            error = null;
            retryDelay = retryBase;
            setState('saved');
        }

        return {
            change,
            flush,
            unlock,
            reset,
            setToken(newToken) { token = newToken; },
            token: () => token,
            state: () => state,
            error: () => error,
            dirty,
            dispose() { disposed = true; clearTimers(); }
        };
    }

    P.autosave = { create: createAutosave, classify: classifySaveError };

    // ── in-window dialog ────────────────────────────────────────────────────

    // Open dialogs, oldest first. One capturing keydown listener serves them all: the newest dialog
    // whose host holds the focus handles Escape and Tab, so a confirm stacked over another dialog
    // closes alone and focus elsewhere on the desktop is left alone. When nothing has focus, the
    // newest dialog takes the key only while its window is shown (not minimised or hidden). A dialog
    // whose host left the document without closing it is closed on the next key.
    const openDialogs = [];
    function dialogForKey() {
        openDialogs.slice().forEach(entry => { if (!document.contains(entry.backdrop)) entry.api.close(null); });
        const active = document.activeElement;
        if (!active || active === document.body || active === document.documentElement) {
            const newest = openDialogs[openDialogs.length - 1];
            return newest && newest.host.getClientRects().length > 0 ? newest : null;
        }
        for (let i = openDialogs.length - 1; i >= 0; i--) {
            if (openDialogs[i].host.contains(active)) return openDialogs[i];
        }
        return null;
    }

    // Each owner bag gets one cleanup that closes its open dialogs; a dialog leaves the set when it
    // closes, so a long-lived owner keeps no closed dialog alive.
    const ownedDialogs = new WeakMap();
    function adoptDialog(owner, api) {
        if (owner.disposed) { api.close(null); return () => {}; }
        let open = ownedDialogs.get(owner);
        if (!open) {
            open = new Set();
            ownedDialogs.set(owner, open);
            owner.add(() => { ownedDialogs.delete(owner); Array.from(open).forEach(d => d.close(null)); });
        }
        open.add(api);
        return () => open.delete(api);
    }
    function onDialogKey(event) {
        const entry = dialogForKey();
        if (entry) entry.onKey(event);
    }

    // P.dialog.open(host, spec) shows a modal inside host (the app root, so it stays in the window).
    // spec: {title, body (Node or array), actions: [{id, label, kind: 'primary'|'danger'|'', disabled}],
    // onAction(id, dialog), className, owner}. Escape, the backdrop and an action without onAction
    // close it; the promise resolves with the closing value (null for Escape/backdrop). owner (a
    // P.util.bag) closes the dialog when the owner is disposed.
    function openDialog(host, spec) {
        const s = spec || {};
        const util = P.util;
        let resolveFn;
        const promise = new Promise(resolve => { resolveFn = resolve; });
        const previousFocus = document.activeElement;
        const titleId = util.uid('cc_dlg');
        const errorBox = util.el('p', { class: 'cc-dialog-error', role: 'alert', hidden: true });
        const actions = util.el('div', { class: 'cc-dialog-actions' });
        const dialog = util.el('div', {
            class: 'cc-dialog' + (s.className ? ' ' + s.className : ''),
            role: 'dialog', 'aria-modal': 'true', 'aria-labelledby': titleId
        },
        util.el('h2', { class: 'cc-dialog-title', id: titleId, text: s.title || '' }),
        util.el('div', { class: 'cc-dialog-body' }, s.body || null),
        errorBox,
        actions);
        const backdrop = util.el('div', { class: 'cc-modal-backdrop' }, dialog);
        let closed = false;
        let entry = null;
        let release = () => {};
        const api = {
            el: dialog,
            close(value) {
                if (closed) return;
                closed = true;
                release();
                const index = openDialogs.indexOf(entry);
                if (index >= 0) openDialogs.splice(index, 1);
                if (!openDialogs.length) document.removeEventListener('keydown', onDialogKey, true);
                backdrop.remove();
                if (previousFocus && typeof previousFocus.focus === 'function' && document.contains(previousFocus)) previousFocus.focus();
                resolveFn(value === undefined ? null : value);
            },
            setBusy(busy) {
                dialog.classList.toggle('is-busy', !!busy);
                actions.querySelectorAll('button').forEach(b => { b.disabled = !!busy || b.dataset.locked === '1'; });
            },
            setError(text) {
                errorBox.textContent = text || '';
                errorBox.hidden = !text;
            },
            get closed() { return closed; }
        };
        (s.actions || []).forEach(action => {
            const button = util.el('button', {
                type: 'button',
                class: 'cc-btn' + (action.kind ? ' cc-btn--' + action.kind : ''),
                dataset: { ccDialogAction: action.id, locked: action.disabled ? '1' : null },
                disabled: !!action.disabled,
                text: action.label
            });
            button.addEventListener('click', () => {
                if (typeof s.onAction === 'function') s.onAction(action.id, api);
                else api.close(action.id);
            });
            actions.appendChild(button);
        });
        function focusables() {
            return Array.from(dialog.querySelectorAll('button,input,select,textarea,[tabindex]:not([tabindex="-1"]),a[href]'))
                .filter(node => !node.disabled && node.getClientRects().length > 0);
        }
        function onKey(event) {
            if (closed) return;
            if (event.key === 'Escape') {
                event.preventDefault();
                event.stopPropagation();
                api.close(null);
                return;
            }
            if (event.key === 'Tab') {
                const list = focusables();
                if (!list.length) return;
                const index = list.indexOf(document.activeElement);
                const next = event.shiftKey ? (index <= 0 ? list.length - 1 : index - 1) : (index === list.length - 1 ? 0 : index + 1);
                event.preventDefault();
                list[next].focus();
            }
        }
        backdrop.addEventListener('mousedown', event => { if (event.target === backdrop) api.close(null); });
        entry = { api, host, backdrop, onKey };
        if (!openDialogs.length) document.addEventListener('keydown', onDialogKey, true);
        openDialogs.push(entry);
        host.appendChild(backdrop);
        const first = dialog.querySelector('[autofocus]') || focusables()[0];
        if (first) first.focus();
        api.promise = promise;
        if (s.owner && typeof s.owner.add === 'function') release = adoptDialog(s.owner, api);
        return api;
    }

    P.dialog = { open: openDialog };
})();
