// Shared feedback for config actions: busy buttons, result lines next to the
// button, state banners that never hang, and the draft patch for tests that
// run with the values typed in (not yet saved).
(function () {
    'use strict';

    const MASK = '••••••••';
    // Positive words our handlers answer with on HTTP 200. Anything else (disabled,
    // no_credentials, no_url, not_configured, ...) is an error, see classify().
    const SUCCESS_STATUS = ['ok', 'success', 'connected', 'online', 'ready', 'running', 'healthy', 'done', 'completed', 'saved',
        'started', 'starting', 'stopping', 'active', 'enabled', 'available', 'up', 'valid', 'created', 'exists', 'applied',
        'accepted', 'updated', 'configured', 'sent', 'deleted', 'revoked', 'imported', 'authorized', 'acknowledged'];
    const WARNING_STATUS = ['partial', 'warning', 'degraded', 'untested', 'skipped', 'setup_required', 'stopped'];
    const ERROR_STATUS = ['error', 'failed', 'failure', 'fail', 'offline', 'disconnected'];
    const BANNER_STATES = Object.freeze(['off', 'unconfigured', 'checking', 'connected', 'degraded', 'error', 'unknown']);
    const MAX_REASON = 300;
    // Last failed check per Config section, read by section_state.js for the sidebar dot.
    const SECTION_STATUS_KEY = 'aurago.config.sectionStatus.v1';
    const SECTION_STATUS_STATES = Object.freeze(['connected', 'degraded', 'error', 'off', 'unconfigured']);
    const DEFAULT_CHECK_TIMEOUT = 15000;

    const prevDisabled = new WeakMap();
    const runs = new Map();
    const bannerRuns = new WeakMap();
    let runCounter = 0;

    function text(key, fallback, params) {
        if (typeof t === 'function') {
            const value = t(key, params);
            if (value && value !== key) return value;
        }
        return fallback;
    }

    function byId(value) {
        if (!value) return null;
        if (typeof value === 'string') return document.getElementById(value);
        return value;
    }

    function clip(value) {
        const clean = String(value == null ? '' : value).replace(/\s+/g, ' ').trim();
        return clean.length > MAX_REASON ? clean.slice(0, MAX_REASON - 1) + '…' : clean;
    }

    function isMasked(value) {
        if (typeof cfgIsMaskedSecret === 'function') return cfgIsMaskedSecret(value);
        return value === MASK;
    }

    // ---- reading a response ------------------------------------------------

    function stringOf(value) {
        if (typeof value === 'string') return value;
        if (value && typeof value === 'object' && typeof value.message === 'string') return value.message;
        return '';
    }

    // Server reason for a parsed body: error(+details), message, detail/reason,
    // errors[0], plain text (never HTML), HTTP code. ctx: {text, httpStatus, reasonText}.
    function reason(body, ctx) {
        ctx = ctx || {};
        if (ctx.reasonText != null) {
            const custom = typeof ctx.reasonText === 'function' ? ctx.reasonText(body, ctx) : ctx.reasonText;
            if (custom) return clip(custom);
        }
        if (body && typeof body === 'object') {
            const error = stringOf(body.error);
            if (error) {
                const details = stringOf(body.details);
                return clip(details && details !== error ? error + ' ' + details : error);
            }
            for (const key of ['message', 'detail', 'reason']) {
                const found = stringOf(body[key]);
                if (found) return clip(found);
            }
            if (Array.isArray(body.errors) && body.errors.length) {
                const first = stringOf(body.errors[0]);
                if (first) return clip(first);
            }
        } else if (typeof body === 'string' && body.trim() && !/^\s*</.test(body)) {
            return clip(body);
        }
        const raw = typeof ctx.text === 'string' ? ctx.text : '';
        if (raw.trim() && !/^\s*</.test(raw)) return clip(raw);
        return ctx.httpStatus ? 'HTTP ' + ctx.httpStatus : '';
    }

    function classify(body, httpOK) {
        if (body && typeof body === 'object' && !Array.isArray(body)) {
            const status = typeof body.status === 'string' ? body.status.toLowerCase() : '';
            if (status && ERROR_STATUS.includes(status)) return 'error';
            if (!httpOK || body.success === false || body.ok === false || body.connected === false) return 'error';
            if (status && SUCCESS_STATUS.includes(status)) return 'success';
            if (status && WARNING_STATUS.includes(status)) return 'warning';
            if (status) return 'error';
            return body.error ? 'error' : 'success';
        }
        return httpOK ? 'success' : 'error';
    }

    function failure(error, opts) {
        const outcome = { ok: false, state: 'error', httpStatus: 0, body: null, text: '', reason: '', network: true };
        if (error && error.name === 'AbortError') {
            outcome.state = 'cancelled';
            outcome.network = false;
            return outcome;
        }
        const network = text('config.common.network_error', 'Network error');
        // A TypeError is how fetch reports a failed request; other errors carry their own message.
        outcome.reason = error && error.name !== 'TypeError' && error.message ? clip(error.message) : network;
        return outcome;
    }

    // Reads a fetch Response (once), a thrown error, an already parsed body or text.
    // Never rejects. -> {ok, state: success|warning|error|cancelled, httpStatus, body, text, reason}
    async function read(value, opts) {
        opts = opts || {};
        try {
            if (value instanceof Error || (typeof DOMException === 'function' && value instanceof DOMException)) {
                return failure(value, opts);
            }
            const outcome = { ok: false, state: 'error', httpStatus: 0, body: null, text: '', reason: '' };
            let httpOK = true;
            if (value && typeof value === 'object' && typeof value.text === 'function' && 'status' in value) {
                outcome.httpStatus = value.status;
                httpOK = value.ok !== false && value.status < 400;
                try { outcome.text = await value.text(); } catch (error) { return failure(error, opts); }
                if (outcome.text) {
                    try { outcome.body = JSON.parse(outcome.text); } catch (_) { outcome.body = null; }
                }
            } else if (typeof value === 'string') {
                outcome.text = value;
            } else {
                outcome.body = typeof value === 'boolean' ? { ok: value } : (value == null ? {} : value);
            }
            outcome.state = classify(outcome.body, httpOK);
            outcome.ok = outcome.state === 'success' || outcome.state === 'warning';
            if (outcome.state === 'success') {
                const detail = reason(outcome.body, { text: '', reasonText: opts.reasonText });
                outcome.reason = detail;
            } else {
                outcome.reason = reason(outcome.body, { text: outcome.text, httpStatus: outcome.httpStatus, reasonText: opts.reasonText });
            }
            if (typeof opts.interpret === 'function') {
                const custom = opts.interpret(outcome);
                if (custom && typeof custom === 'object') {
                    Object.assign(outcome, custom);
                    outcome.ok = outcome.state === 'success' || outcome.state === 'warning';
                }
            }
            return outcome;
        } catch (error) {
            return failure(error, opts);
        }
    }

    // ---- busy buttons -------------------------------------------------------

    function elementsOf(target) {
        if (!target) return [];
        if (typeof target === 'string') return [...document.querySelectorAll(target)];
        if (target.nodeType === 1) return [target];
        return [...target].flatMap(item => elementsOf(item));
    }

    // Busy marks the button (disabled, aria-busy, data-config-action-busy) and
    // restores its previous disabled state afterwards. quiet only disables.
    function busy(target, on, opts) {
        const quiet = !!(opts && opts.quiet);
        elementsOf(target).forEach(element => {
            if (on) {
                if (!prevDisabled.has(element)) prevDisabled.set(element, element.disabled);
                element.disabled = true;
                if (quiet) return;
                element.setAttribute('aria-busy', 'true');
                element.dataset.configActionBusy = 'true';
                element.classList.add('is-busy');
                return;
            }
            element.disabled = prevDisabled.has(element) ? prevDisabled.get(element) : false;
            prevDisabled.delete(element);
            if (quiet) return;
            element.setAttribute('aria-busy', 'false');
            element.dataset.configActionBusy = 'false';
            element.classList.remove('is-busy');
        });
    }

    // ---- result line --------------------------------------------------------

    function resultHTML(id, extraClass) {
        return '<span' + (id ? ' id="' + escapeText(id) + '"' : '') + ' class="cfg-action-result' + (extraClass ? ' ' + escapeText(extraClass) : '')
            + '" role="status" aria-live="polite" aria-atomic="true"></span>';
    }

    function escapeText(value) {
        return String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
    }

    function prepareResult(element) {
        element.classList.add('cfg-action-result');
        if (!element.hasAttribute('role')) element.setAttribute('role', 'status');
        if (!element.hasAttribute('aria-live')) element.setAttribute('aria-live', 'polite');
        if (!element.hasAttribute('aria-atomic')) element.setAttribute('aria-atomic', 'true');
        return element;
    }

    // Writes the result line. state: pending|success|warning|error; '' clears it.
    function showResult(target, state, message) {
        const element = byId(target);
        if (!element) return null;
        prepareResult(element);
        if (!state || !message) {
            element.textContent = '';
            delete element.dataset.state;
            element.classList.remove('is-success', 'is-warning', 'is-danger');
            return element;
        }
        element.dataset.state = state;
        element.textContent = message;
        // Existing styles (adg-test-result) key off these classes.
        element.classList.toggle('is-success', state === 'success');
        element.classList.toggle('is-warning', state === 'warning');
        element.classList.toggle('is-danger', state === 'error');
        return element;
    }

    function resultFor(button, opts) {
        const explicit = byId(opts.target);
        if (explicit) return prepareResult(explicit);
        if (button.id) {
            const existing = document.getElementById(button.id + '-result');
            if (existing) return prepareResult(existing);
        }
        const span = prepareResult(document.createElement('span'));
        if (button.id) span.id = button.id + '-result';
        const reasonLine = button.id ? document.getElementById(button.id + '-reason') : null;
        (reasonLine || button).insertAdjacentElement('afterend', span);
        return span;
    }

    function resolveText(value, fallback, outcome) {
        const picked = typeof value === 'function' ? value(outcome) : value;
        return picked == null || picked === '' ? fallback : String(picked);
    }

    function toast(mode, message, kind) {
        if (!mode || mode === 'never' || typeof showToast !== 'function' || !message) return;
        if (mode === 'always' || (mode === 'error' && kind === 'error')) showToast(message, kind);
    }

    // Runs run() behind a busy button and writes the outcome next to it.
    // Never rejects. -> outcome {ok, state, reason, ...} or {skipped: 'busy'|'locked'|'cancelled'|'missing'}
    async function actionResult(button, run, opts) {
        opts = opts || {};
        const element = byId(button);
        if (!element) return { skipped: 'missing' };
        if (element.dataset.configActionBusy === 'true') return { skipped: 'busy' };
        if (element.dataset.configActionLocked === 'true') {
            if (typeof element.focus === 'function') element.focus();
            return { skipped: 'locked' };
        }
        const key = element.id ? 'id:' + element.id : element;
        const lock = elementsOf(opts.lock).filter(item => item !== element);
        let line = null;
        let mine = 0;
        try {
            if (opts.confirm) {
                const confirm = typeof opts.confirm === 'string' ? { message: opts.confirm } : opts.confirm;
                const approved = typeof showConfirm === 'function'
                    ? await showConfirm(confirm.title || text('common.confirm_title', 'Confirm'), confirm.message || '')
                    : true;
                if (!approved) return { skipped: 'cancelled' };
            }
            mine = ++runCounter;
            runs.set(key, mine);
            line = resultFor(element, opts);
            busy(element, true);
            busy(lock, true, { quiet: true });
            showResult(line, 'pending', resolveText(opts.pending, text('config.feedback.running', 'Running…')));
            let raw;
            try { raw = typeof run === 'function' ? await run() : undefined; } catch (error) { raw = error; }
            const outcome = raw === undefined ? await read({ ok: true }, opts) : await read(raw, opts);
            const stale = runs.get(key) !== mine;
            const live = element.isConnected ? line : (element.id ? document.getElementById(element.id + '-result') : null);
            if (!stale && live) {
                if (outcome.state === 'cancelled') {
                    showResult(live, '', '');
                } else if (outcome.state === 'success') {
                    const done = resolveText(opts.success, text('config.feedback.done', 'Done.'), outcome);
                    showResult(live, 'success', opts.successDetail && outcome.reason ? done + ' ' + outcome.reason : done);
                } else if (outcome.state === 'warning') {
                    const warn = resolveText(opts.warning, '', outcome);
                    showResult(live, 'warning', outcome.reason ? (warn ? warn + ' ' + outcome.reason : outcome.reason) : (warn || text('config.feedback.done', 'Done.')));
                } else {
                    const failed = resolveText(opts.failure, text('config.feedback.failed', 'Failed.'), outcome);
                    showResult(live, 'error', outcome.reason ? failed + ' ' + outcome.reason : failed);
                }
                toast(opts.toast, live.textContent, outcome.state === 'success' ? 'success' : (outcome.state === 'warning' ? 'warning' : 'error'));
                if (opts.clearAfter > 0 && outcome.state !== 'error') {
                    setTimeout(() => {
                        if (runs.get(key) === mine && live.dataset.state !== 'pending') showResult(live, '', '');
                    }, opts.clearAfter);
                }
            }
            try {
                if (outcome.state === 'success' || outcome.state === 'warning') {
                    if (typeof opts.onSuccess === 'function') opts.onSuccess(outcome);
                } else if (outcome.state === 'error' && typeof opts.onError === 'function') {
                    opts.onError(outcome);
                }
            } catch (_) { /* a callback must not turn a finished run into a rejection */ }
            return outcome;
        } catch (error) {
            if (line) showResult(line, 'error', text('config.feedback.failed', 'Failed.') + ' ' + clip(error && error.message));
            return failure(error, opts);
        } finally {
            if (mine) {
                busy(element, false);
                busy(lock, false, { quiet: true });
                if (window.AuraConfigActions && typeof window.AuraConfigActions.refresh === 'function') window.AuraConfigActions.refresh();
            }
        }
    }

    // ---- state banner -------------------------------------------------------

    function bannerLabel(state) {
        const fallback = { off: 'Off', unconfigured: 'Not set up', checking: 'Checking…', connected: 'Connected', degraded: 'Limited', error: 'Error', unknown: 'Unknown' };
        return text('config.status.' + state, fallback[state] || state);
    }

    function safeHref(href) {
        return /^(https?:\/\/|\/|#)/i.test(String(href || '')) ? String(href) : '';
    }

    function bannerDetail(state, opts) {
        if (opts.detail != null && opts.detail !== '') return String(opts.detail);
        if (opts.hint && (state === 'off' || state === 'unconfigured')) {
            return typeof opts.hint === 'string' ? opts.hint : text('config.status.' + state + '_hint', '');
        }
        return '';
    }

    function bannerState(state) {
        return BANNER_STATES.includes(state) ? state : 'unknown';
    }

    function activeConfigSection() {
        const state = window.AuraConfigState;
        try { return state && typeof state.snapshot === 'function' ? String(state.snapshot().section || '') : ''; } catch (_) { return ''; }
    }

    // A banner result belongs to a Config section: error/degraded is remembered for
    // the browser session, any later answer clears it. Announces
    // aura-config-section-status {section, state, reason} for the sidebar.
    function reportSectionStatus(section, state, detail) {
        if (!section || section === 'overview' || !SECTION_STATUS_STATES.includes(state)) return;
        let map = {};
        try { map = JSON.parse(sessionStorage.getItem(SECTION_STATUS_KEY) || '{}') || {}; } catch (_) { map = {}; }
        if (typeof map !== 'object' || Array.isArray(map)) map = {};
        const failed = state === 'error' || state === 'degraded';
        const reasonText = failed ? clip(detail) : '';
        if (failed) map[section] = { state: state, reason: reasonText, at: Date.now() };
        else delete map[section];
        try { sessionStorage.setItem(SECTION_STATUS_KEY, JSON.stringify(map)); } catch (_) { /* private browsing can disable storage */ }
        document.dispatchEvent(new CustomEvent('aura-config-section-status', { detail: { section: section, state: state, reason: reasonText } }));
    }

    function bannerHTML(id, state, opts) {
        opts = opts || {};
        state = bannerState(state);
        const detail = bannerDetail(state, opts);
        const links = (opts.links || []).map(link => {
            const href = safeHref(link && link.href);
            return href ? '<a class="cfg-state-link" href="' + escapeText(href) + '" target="_blank" rel="noopener noreferrer">' + escapeText(link.label || href) + '</a>' : '';
        }).join('');
        return '<div' + (id ? ' id="' + escapeText(id) + '"' : '') + ' class="cfg-state-banner" data-state="' + state + '" role="status" aria-live="polite"'
            + (state === 'checking' ? ' aria-busy="true"' : '') + '>'
            + '<span class="cfg-state-dot" aria-hidden="true"></span>'
            + '<span class="cfg-state-label">' + escapeText(opts.label || bannerLabel(state)) + '</span>'
            + '<span class="cfg-state-detail">' + escapeText(detail) + '</span>'
            + links
            + '</div>';
    }

    // Renders a banner state into an element. opts: {label, detail, hint, retry, links, reason}.
    function statusBanner(target, state, opts) {
        const element = byId(target);
        if (!element) return null;
        opts = opts || {};
        state = bannerState(state);
        if (state === 'error' && opts.reason && opts.detail == null) opts = Object.assign({}, opts, { detail: opts.reason });
        element.classList.add('cfg-state-banner');
        element.dataset.state = state;
        if (!element.hasAttribute('role')) element.setAttribute('role', 'status');
        if (!element.hasAttribute('aria-live')) element.setAttribute('aria-live', 'polite');
        element.setAttribute('aria-busy', state === 'checking' ? 'true' : 'false');
        const dot = document.createElement('span');
        dot.className = 'cfg-state-dot';
        dot.setAttribute('aria-hidden', 'true');
        const label = document.createElement('span');
        label.className = 'cfg-state-label';
        label.textContent = opts.label || bannerLabel(state);
        const detail = document.createElement('span');
        detail.className = 'cfg-state-detail';
        detail.textContent = bannerDetail(state, opts);
        const parts = [dot, label, detail];
        (opts.links || []).forEach(link => {
            const href = safeHref(link && link.href);
            if (!href) return;
            const anchor = document.createElement('a');
            anchor.className = 'cfg-state-link';
            anchor.href = href;
            anchor.target = '_blank';
            anchor.rel = 'noopener noreferrer';
            anchor.textContent = link.label || href;
            parts.push(anchor);
        });
        if (typeof opts.retry === 'function' && (state === 'error' || state === 'unknown' || state === 'degraded')) {
            const retry = document.createElement('button');
            retry.type = 'button';
            retry.className = 'cfg-state-retry';
            retry.textContent = text('config.status.retry', 'Check again');
            retry.addEventListener('click', event => {
                event.preventDefault();
                opts.retry();
            });
            parts.push(retry);
        }
        element.replaceChildren(...parts);
        reportSectionStatus(opts.section || (element.isConnected ? activeConfigSection() : ''), state, bannerDetail(state, opts));
        return element;
    }

    // Checks a service: checking -> read -> map(body, outcome) -> {state, detail, ...}.
    // A failing request shows error(reason); a falsy map result or a timeout
    // shows unknown with a retry. The banner never stays on "checking".
    async function checkBanner(target, fetcher, map, opts) {
        const element = byId(target);
        if (!element) return null;
        opts = opts || {};
        const mine = (bannerRuns.get(element) || 0) + 1;
        bannerRuns.set(element, mine);
        const timeoutMs = opts.timeoutMs > 0 ? opts.timeoutMs : DEFAULT_CHECK_TIMEOUT;
        const retry = opts.retry === false ? null : (typeof opts.retry === 'function' ? opts.retry : () => checkBanner(element, fetcher, map, opts));
        // The section is taken when the check starts: the answer may arrive after the user moved on.
        const section = opts.section || (element.isConnected ? activeConfigSection() : '');
        const base = { links: opts.links, retry: retry, hint: opts.hint, section: section };
        const draw = (state, extra) => {
            if (bannerRuns.get(element) !== mine) return null;
            statusBanner(element, state, Object.assign({}, base, extra));
            return { state: state, detail: extra && extra.detail };
        };
        draw('checking', { label: opts.checkingLabel, detail: opts.checkingDetail, retry: null });
        const controller = typeof AbortController === 'function' ? new AbortController() : null;
        let timer = 0;
        const timeout = new Promise(resolve => {
            timer = setTimeout(() => resolve('timeout'), timeoutMs);
        });
        let outcome;
        try {
            const attempt = (async () => {
                let raw;
                try { raw = await fetcher(controller ? controller.signal : undefined); } catch (error) { raw = error; }
                return read(raw, opts);
            })();
            outcome = await Promise.race([attempt, timeout]);
        } finally {
            clearTimeout(timer);
        }
        if (outcome === 'timeout') {
            if (controller) controller.abort();
            return draw('unknown', { detail: text('config.feedback.timeout', 'No answer in time.') });
        }
        if (outcome.state === 'cancelled') return draw('unknown', {});
        if (outcome.network || outcome.httpStatus >= 400) {
            return draw('error', { detail: outcome.reason });
        }
        let mapped;
        try {
            mapped = typeof map === 'function' ? map(outcome.body, outcome) : defaultMap(outcome);
        } catch (_) {
            mapped = null;
        }
        if (!mapped || !BANNER_STATES.includes(mapped.state)) return draw('unknown', {});
        const extra = Object.assign({}, mapped);
        delete extra.state;
        if (mapped.state === 'error' && extra.detail == null) extra.detail = outcome.reason;
        return draw(mapped.state, extra);
    }

    function defaultMap(outcome) {
        if (outcome.state === 'success') return { state: 'connected', detail: outcome.reason };
        if (outcome.state === 'warning') return { state: 'degraded', detail: outcome.reason };
        return { state: 'error', detail: outcome.reason };
    }

    // ---- draft patch ----------------------------------------------------------

    function writePath(object, path, value) {
        const keys = path.split('.').filter(Boolean);
        if (!keys.length) return;
        let current = object;
        keys.slice(0, -1).forEach((key, index) => {
            const next = keys[index + 1];
            if (current[key] == null || typeof current[key] !== 'object') current[key] = /^\d+$/.test(next) ? [] : {};
            current = current[key];
        });
        current[keys[keys.length - 1]] = value;
    }

    function isSecretField(element) {
        return element.type === 'password' || element.dataset.secret === 'true' || !!(element.closest && element.closest('.password-wrap'));
    }

    // Current draft values of every data-path field under the prefixes, in the
    // shape of the PUT /api/config patch. Masked and empty secrets are left out
    // so the server falls back to the stored value.
    function draftPatch(prefixes, root) {
        const list = (Array.isArray(prefixes) ? prefixes : [prefixes]).filter(Boolean).map(String);
        const state = window.AuraConfigState;
        const patch = {};
        if (!state || !list.length) return patch;
        const seen = new Set();
        (root || document).querySelectorAll('[data-path]').forEach(element => {
            const path = element.dataset.path;
            if (!path || seen.has(path)) return;
            if (!list.some(prefix => path === prefix || path.startsWith(prefix + '.'))) return;
            seen.add(path);
            const value = state.get(path);
            if (value === undefined) return;
            if (isMasked(value)) return;
            if (isSecretField(element) && (value === '' || value == null)) return;
            writePath(patch, path, typeof structuredClone === 'function' ? structuredClone(value) : JSON.parse(JSON.stringify(value)));
        });
        return patch;
    }

    const api = {
        read: read,
        reason: reason,
        busy: busy,
        actionResult: actionResult,
        showResult: showResult,
        resultHTML: resultHTML,
        statusBanner: statusBanner,
        bannerHTML: bannerHTML,
        checkBanner: checkBanner,
        draftPatch: draftPatch,
        reportSectionStatus: reportSectionStatus,
        BANNER_STATES: BANNER_STATES
    };
    window.AuraConfigFeedback = Object.freeze(api);

    const cfgActionResult = actionResult.bind(null);
    cfgActionResult.show = showResult;
    cfgActionResult.html = resultHTML;
    const cfgStatusBanner = statusBanner.bind(null);
    cfgStatusBanner.html = bannerHTML;
    cfgStatusBanner.check = checkBanner;
    window.cfgActionResult = cfgActionResult;
    window.cfgStatusBanner = cfgStatusBanner;
    window.cfgResponseReason = reason;
    window.cfgDraftPatch = draftPatch;
})();
