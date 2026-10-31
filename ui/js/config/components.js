// Shared config building blocks: delete button, dialog markup, status badge and
// list states. Needs js/config/icons.js; dialogs are opened and closed with
// openModal(id, {initialFocus, onClose, canClose}) / closeModal(id) from shared-core.js,
// which also own focus trapping, Escape and the backdrop - nothing here traps focus.
(function () {
    'use strict';

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
    }

    function text(key, fallback, params) {
        if (typeof t === 'function') {
            const value = t(key, params);
            if (value && value !== key) return value;
        }
        return fallback;
    }

    function icon(name, opts) {
        return typeof window.cfgIcon === 'function' ? window.cfgIcon(name, opts) : '';
    }

    // attrs: {name: value} (false/null/undefined skipped, true = bare attribute) or a ready attribute string.
    function attrString(attrs) {
        if (!attrs) return '';
        if (typeof attrs === 'string') return ' ' + attrs.trim();
        return Object.keys(attrs).filter(name => attrs[name] !== false && attrs[name] != null).map(name =>
            attrs[name] === true ? ' ' + name : ' ' + name + '="' + esc(attrs[name]) + '"').join('');
    }

    // Delete control: one style everywhere (btn-danger + trash icon). Visible text unless iconOnly,
    // then aria-label + title carry the name. label defaults to "Delete"; pass the item name in it
    // ("Delete Foo") for icon-only buttons in lists. Asking first is up to the handler
    // (showConfirmDialog({danger: true, ...})).
    function cfgDeleteButton(opts) {
        opts = opts || {};
        const label = opts.label != null ? opts.label : text('config.common.delete', 'Delete');
        const cls = 'btn-danger cfg-delete-btn' + (opts.iconOnly ? ' is-icon-only' : '') + (opts.cls ? ' ' + esc(opts.cls) : '');
        const a11y = opts.iconOnly ? ' aria-label="' + esc(label) + '" title="' + esc(label) + '"' : '';
        return '<button type="button" class="' + cls + '"' + a11y + attrString(opts.attrs) + '>'
            + icon('trash') + (opts.iconOnly ? '' : '<span>' + esc(label) + '</span>') + '</button>';
    }

    // Dialog markup for the shared .modal-overlay. body/actions are HTML (actions: string or array).
    // size: 'sm' | 'lg' | 'xl' (default: standard width). Hidden until openModal(id).
    function cfgModal(opts) {
        opts = opts || {};
        const id = opts.id || 'cfg-modal';
        const titleID = id + '-title';
        const size = opts.size ? ' modal--' + esc(opts.size) : '';
        const actions = Array.isArray(opts.actions) ? opts.actions.join('') : (opts.actions || '');
        const close = text('config.common.close', 'Close');
        return '<div class="modal-overlay cfg-modal-overlay" id="' + esc(id) + '" role="dialog" aria-modal="true" aria-labelledby="' + esc(titleID) + '">'
            + '<div class="modal' + size + '">'
            + '<div class="modal-header">'
            + '<h2 class="modal-title" id="' + esc(titleID) + '">' + esc(opts.title) + '</h2>'
            + '<button type="button" class="modal-close" aria-label="' + esc(close) + '" title="' + esc(close) + '" onclick="closeModal(\'' + esc(id) + '\')">' + icon('x') + '</button>'
            + '</div>'
            + '<div class="modal-body">' + (opts.body || '') + '</div>'
            + (actions ? '<div class="modal-actions">' + actions + '</div>' : '')
            + '</div></div>';
    }

    const BADGE = {
        ok: 'ok', success: 'ok', connected: 'ok', online: 'ok', ready: 'ok',
        warn: 'warn', warning: 'warn', degraded: 'warn',
        error: 'error', danger: 'error', failed: 'error', offline: 'error',
        idle: 'idle', neutral: 'idle', off: 'idle', unconfigured: 'idle', unknown: 'idle',
        running: 'running', progress: 'running', checking: 'running', busy: 'running'
    };

    // Status pill: status ok|warn|error|idle|running (aliases: success, warning, neutral, off, checking ...).
    function cfgBadge(status, label, opts) {
        opts = opts || {};
        const mapped = BADGE[String(status || '').toLowerCase()] || 'idle';
        return '<span class="pw-badge' + (opts.cls ? ' ' + esc(opts.cls) : '') + '" data-status="' + mapped + '">' + esc(label) + '</span>';
    }

    // Loading / empty / error state inside a list container (element or id). Replaces its content.
    // state 'loading' | 'empty' | 'error'; title and text optional (defaults come from config.common.*);
    // retry (function) adds "Try again" to the error state. Returns the element.
    function cfgListState(target, opts) {
        const el = typeof target === 'string' ? document.getElementById(target) : target;
        if (!el) return null;
        opts = opts || {};
        const state = opts.state || 'loading';
        let html;
        if (state === 'loading') {
            html = '<div class="pw-loading-state cfg-list-state" role="status"><span class="pw-skeleton skeleton" aria-hidden="true"></span><span>'
                + esc(opts.title || text('config.common.loading', 'Loading...')) + '</span></div>';
        } else if (state === 'error') {
            html = '<div class="pw-error-state cfg-list-state" role="alert"><strong>' + esc(opts.title || text('config.common.load_failed', 'This list could not be loaded.')) + '</strong>'
                + (opts.text ? '<p>' + esc(opts.text) + '</p>' : '')
                + (typeof opts.retry === 'function' ? '<button type="button" class="btn-secondary" data-cfg-retry>' + icon('refresh') + '<span>' + esc(text('config.common.retry', 'Try again')) + '</span></button>' : '')
                + '</div>';
        } else {
            html = '<div class="pw-empty-state cfg-list-state" role="status"><h3>' + esc(opts.title || text('config.common.empty', 'Nothing here yet.')) + '</h3>'
                + (opts.text ? '<p>' + esc(opts.text) + '</p>' : '') + (opts.actionHTML || '') + '</div>';
        }
        el.innerHTML = html;
        const retry = el.querySelector('[data-cfg-retry]');
        if (retry) retry.addEventListener('click', () => opts.retry());
        return el;
    }

    window.cfgDeleteButton = cfgDeleteButton;
    window.cfgModal = cfgModal;
    window.cfgBadge = cfgBadge;
    window.cfgListState = cfgListState;
    window.AuraConfigComponents = { deleteButton: cfgDeleteButton, modal: cfgModal, badge: cfgBadge, listState: cfgListState };
})();
