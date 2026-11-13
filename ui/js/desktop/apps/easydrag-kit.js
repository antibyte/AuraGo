// EasyDrag kit (W1a, FD-09): the small parts every surface shares, drawn from the tokens of the
// stylesheet. disclosure replaces the native details element (a button head with aria-expanded and
// aria-controls, so Enter and Space work), callout is the tinted hint with a status disc and no
// border, chip is the neutral label, toast is the in-app message with at most one action, toolbar is a
// row of equal controls. disclosureMarkup feeds the string templates; attach(root) makes the heads of
// that markup work through one delegated click listener (easydrag.js calls it for .ed-app).
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    // TOAST_MS is how long a toast stays while neither the pointer nor the focus is on it.
    const TOAST_MS = 8000;
    let serial = 0;

    const esc = value => ED.core.esc(value);
    const text = value => (value == null ? '' : String(value));
    const has = value => value != null && value !== '';

    function head(root) { return root ? root.querySelector('.ed-disclosure-head') : null; }
    function body(root) { return root ? root.querySelector('.ed-disclosure-body') : null; }
    function isOpen(root) { const h = head(root); return !!h && h.getAttribute('aria-expanded') === 'true'; }
    function setOpen(root, open) {
        const h = head(root), b = body(root);
        if (!h || !b) return;
        h.setAttribute('aria-expanded', open ? 'true' : 'false');
        b.toggleAttribute('hidden', !open);
    }
    function setTitle(root, title) {
        const node = root && root.querySelector('.ed-disclosure-title');
        if (node) node.textContent = text(title);
    }
    function setCount(root, count) {
        const node = root && root.querySelector('.ed-disclosure-count');
        if (!node) return;
        node.textContent = text(count);
        node.toggleAttribute('hidden', !has(count));
    }
    function toggle(root) { setOpen(root, !isOpen(root)); }

    // disclosureMarkup returns the disclosure as a string: opts {title, count, open, body (markup), id,
    // className, attrs {name: value}, headLabel}. title, count, headLabel and attrs are escaped; body is markup.
    function disclosureMarkup(opts) {
        const o = opts || {};
        const id = o.id || 'ed-disc-' + (++serial);
        const open = !!o.open;
        const attrs = Object.keys(o.attrs || {}).map(name => ' ' + name + '="' + esc(o.attrs[name]) + '"').join('');
        return '<div class="ed-disclosure' + (o.className ? ' ' + esc(o.className) : '') + '"' + attrs + '>' +
            '<button type="button" class="ed-disclosure-head" aria-expanded="' + (open ? 'true' : 'false') + '" aria-controls="' + esc(id) + '"' + (o.headLabel ? ' aria-label="' + esc(o.headLabel) + '"' : '') + '>' +
            '<span class="ed-disclosure-chev" aria-hidden="true"></span><span class="ed-disclosure-title">' + esc(o.title) + '</span>' +
            '<span class="ed-disclosure-count"' + (has(o.count) ? '' : ' hidden') + '>' + esc(o.count) + '</span></button>' +
            '<div class="ed-disclosure-body" id="' + esc(id) + '"' + (open ? '' : ' hidden') + '>' + (o.body || '') + '</div></div>';
    }

    // disclosure returns the element; its head is bound, so it needs no attached root.
    function disclosure(opts) {
        const o = opts || {};
        const root = ED.core.el(disclosureMarkup(o));
        setTitle(root, o.title);
        setCount(root, o.count);
        const h = head(root);
        h.edBound = true;
        h.addEventListener('click', () => toggle(root));
        return root;
    }

    // attach makes the heads of disclosureMarkup work under root (one listener, also for markup that arrives later).
    function attach(root) {
        root.addEventListener('click', event => {
            const h = event.target && event.target.closest ? event.target.closest('.ed-disclosure-head') : null;
            if (h && !h.edBound) toggle(h.parentNode);
        });
    }

    const INFO_DISC = '<span class="ed-callout-disc" aria-hidden="true"><svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round"><path d="M8 7v4.5M8 4.6v.1"/></svg></span>';

    // HUE maps a status value to the colour family of the callout tint. The stylesheet keys the tint to
    // data-ed-hue, so no rule outside the status block has to spell a status.
    const HUE = { ok: 'ok', stopped: 'ok', failed: 'danger', uncertain: 'warn', blocked: 'warn', info: 'info', running: 'info', waiting: 'info' };

    // callout is the hint box: opts {status, text, action: {label, onClick}, role}. status is one of the ED.status
    // values, or 'info' (the default) for a neutral piece of information.
    function callout(opts) {
        const o = opts || {};
        const status = o.status || 'info';
        const lead = status === 'info' || !ED.status ? INFO_DISC : ED.status.disc(status, 18);
        const action = o.action && o.action.label ? '<button type="button" class="ed-callout-action"></button>' : '';
        const el = ED.core.el('<div class="ed-callout" data-status="' + esc(status) + '" data-ed-hue="' + (HUE[status] || 'neutral') + '"' + (o.role ? ' role="' + esc(o.role) + '"' : '') + '>' + lead + '<div class="ed-callout-text"></div>' + action + '</div>');
        el.querySelector('.ed-callout-text').textContent = text(o.text);
        const button = el.querySelector('.ed-callout-action');
        if (button) {
            button.textContent = text(o.action.label);
            if (typeof o.action.onClick === 'function') button.addEventListener('click', o.action.onClick);
        }
        return el;
    }

    // chip is the neutral label: opts {text, glyph (a name of core.ICONS)}.
    function chip(opts) {
        const o = opts || {};
        const el = ED.core.el('<span class="ed-chip ed-chip--neutral">' + (o.glyph ? ED.core.icon(o.glyph) : '') + '<span class="ed-chip-text"></span></span>');
        el.querySelector('.ed-chip-text').textContent = text(o.text);
        return el;
    }

    // toolbar is a row of equal controls: items [{id, label, icon, title, pressed, disabled, iconOnly, onClick}].
    function toolbar(items) {
        const el = ED.core.el('<div class="ed-toolbar" role="group"></div>');
        (items || []).filter(Boolean).forEach(item => {
            const b = ED.core.el('<button type="button" class="ed-btn ed-toolbar-item">' + (item.icon ? ED.core.icon(item.icon) : '') + '<span class="ed-toolbar-label"></span></button>');
            b.querySelector('.ed-toolbar-label').textContent = text(item.label);
            if (item.id) b.setAttribute('data-ed-toolbar', item.id);
            if (item.title) b.setAttribute('title', item.title);
            if (item.iconOnly) b.setAttribute('aria-label', text(item.label));
            if (item.pressed != null) b.setAttribute('aria-pressed', item.pressed ? 'true' : 'false');
            if (item.disabled) b.disabled = true;
            if (typeof item.onClick === 'function') b.addEventListener('click', item.onClick);
            el.appendChild(b);
        });
        return el;
    }

    // toast: opts {status, text, action: {label, icon, onClick}, host, closeLabel, duration (ms, 0 = stays),
    // start (false: stay hidden until show()), onHide({hadFocus}), live, notify}. With a host it builds the
    // in-app toast and returns {el, show(message), hide(), isShown(), dispose()}. Without a host it wraps
    // opts.notify (the desktop notification takes a title and a message and has no action) and returns null.
    // The toast holds while the pointer or the focus is on it. The action hides the toast first and then
    // runs, so the focus goes back before the action acts.
    function toast(opts) {
        const o = opts || {};
        if (!o.host) {
            if (typeof o.notify === 'function') o.notify({ title: o.title || 'EasyDrag', message: text(o.text), type: o.status === 'failed' ? 'error' : undefined });
            return null;
        }
        const lead = o.status && ED.status ? ED.status.disc(o.status, 18) : '';
        const action = o.action && o.action.label ? '<button type="button" class="ed-toast-action" data-ed-toast-action>' + (o.action.icon ? ED.core.icon(o.action.icon) : '') + '<span></span></button>' : '';
        const close = o.closeLabel ? '<button type="button" class="ed-icon-btn" data-ed-toast-close aria-label="' + esc(o.closeLabel) + '" title="' + esc(o.closeLabel) + '">' + ED.core.icon('x') + '</button>' : '';
        const el = ED.core.el('<div class="ed-toast" data-ed-toast' + (o.live ? ' role="status"' : '') + ' hidden>' + lead + '<span class="ed-toast-text" data-ed-toast-text></span>' + action + close + '</div>');
        el.hidden = true;
        if (action) el.querySelector('[data-ed-toast-action] span').textContent = text(o.action.label);
        const duration = o.duration == null ? TOAST_MS : o.duration;
        let timer = 0;
        const arm = () => { clearTimeout(timer); timer = duration > 0 ? setTimeout(hide, duration) : 0; };
        function hide() {
            clearTimeout(timer);
            timer = 0;
            if (el.hidden) return;
            const hadFocus = el.contains(document.activeElement);
            el.hidden = true;
            if (typeof o.onHide === 'function') o.onHide({ hadFocus });
        }
        function show(message) {
            if (message !== undefined) el.querySelector('[data-ed-toast-text]').textContent = text(message);
            el.hidden = false;
            arm();
        }
        el.addEventListener('focusin', () => clearTimeout(timer));
        el.addEventListener('focusout', () => { if (!el.hidden) arm(); });
        el.addEventListener('pointerenter', () => clearTimeout(timer));
        el.addEventListener('pointerleave', () => { if (!el.hidden) arm(); });
        el.addEventListener('click', event => {
            if (event.target.closest('[data-ed-toast-close]')) { hide(); return; }
            if (!event.target.closest('[data-ed-toast-action]')) return;
            hide();
            if (typeof o.action.onClick === 'function') o.action.onClick();
        });
        o.host.appendChild(el);
        if (o.start !== false) show(o.text);
        else el.querySelector('[data-ed-toast-text]').textContent = text(o.text);
        return { el, show, hide, isShown: () => !el.hidden, dispose() { clearTimeout(timer); el.remove(); } };
    }

    ED.kit = { disclosure, disclosureMarkup, attach, isOpen, setOpen, setTitle, setCount, body, callout, chip, toast, toolbar };
})();
