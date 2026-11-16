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
    // FLASH_MS is how long a button shows its confirmation (flash).
    const FLASH_MS = 2000;
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

    // flash shows text on a button for ms (default 2 s) and then gives the button back its own look: the
    // text is one added span (the stylesheet hides the icon while data-ed-flash is set), and the label and
    // title are restored. Nothing moves or fades. A second flash on the same button restarts the time.
    function flash(button, message, ms) {
        if (!button) return;
        if (button.edFlash) clearTimeout(button.edFlash.timer);
        else {
            const span = document.createElement('span');
            span.className = 'ed-flash-text';
            button.appendChild(span);
            button.edFlash = { span, label: button.getAttribute('aria-label'), title: button.getAttribute('title') };
        }
        const saved = button.edFlash;
        saved.span.textContent = text(message);
        button.setAttribute('data-ed-flash', '');
        button.setAttribute('aria-label', text(message));
        button.setAttribute('title', text(message));
        saved.timer = setTimeout(() => {
            saved.span.remove();
            if (saved.label == null) button.toggleAttribute('aria-label', false); else button.setAttribute('aria-label', saved.label);
            if (saved.title == null) button.toggleAttribute('title', false); else button.setAttribute('title', saved.title);
            button.toggleAttribute('data-ed-flash', false);
            button.edFlash = null;
        }, ms || FLASH_MS);
    }

    // limit is the length note of a capped text input (voice guide rule 5): opts {max, counter, count, t, after}.
    // counter true (long text) shows "count / max" from 90 % of max; counter false (one line) shows
    // "At most {max} characters." once the input reaches max. count(value) measures the value (default:
    // value.length; a step label counts code points). t translates. after is the element the note follows
    // (default: the label around the input, else the input). The note describes the input, so a screen reader
    // reads the count on focus; a separate polite live region (.ed-sr-only) speaks only when the state changes:
    // the counter appears, the cap is reached (a counter at max: a textarea with maxlength never goes over), the text goes over the limit, the note hides. It never speaks on
    // every keystroke (W1b amendment 23). The note hides again when the text is shorter. Returns {update, detach}.
    function limit(input, opts) {
        const o = opts || {};
        const max = Number(o.max);
        const measure = typeof o.count === 'function' ? o.count : value => value.length;
        const say = typeof o.t === 'function' ? o.t : key => key;
        const note = document.createElement('div');
        note.className = 'ed-limit';
        note.id = 'ed-limit-' + (++serial);
        note.toggleAttribute('hidden', true);
        const live = document.createElement('span');
        live.className = 'ed-sr-only';
        live.setAttribute('aria-live', 'polite');
        live.setAttribute('data-ed-limit-live', '');
        const anchor = o.after || (typeof input.closest === 'function' && input.closest('label')) || input;
        if (anchor.parentNode) anchor.parentNode.insertBefore(note, anchor.nextSibling || null);
        if (note.parentNode) note.parentNode.insertBefore(live, note.nextSibling || null);
        const described = input.getAttribute('aria-describedby');
        input.setAttribute('aria-describedby', described ? described + ' ' + note.id : note.id);
        let spoken = 'none';
        function update() {
            const length = measure(String(input.value == null ? '' : input.value));
            let kind = '';
            let message = '';
            if (o.counter) {
                if (length * 10 >= max * 9) { kind = 'counter'; message = say('easydrag.ui.limit_counter', { count: ED.format.number(length), max: ED.format.number(max) }); }
            } else if (length >= max) { kind = 'cap'; message = say('easydrag.ui.limit_reached', { max: ED.format.number(max) }); }
            if (note.textContent !== message) note.textContent = message;
            note.toggleAttribute('hidden', !kind);
            note.classList.toggle('is-over', length > max);
            note.setAttribute('data-ed-limit', kind || 'none');
            const state = kind ? kind + (length > max ? '-over' : length >= max ? '-cap' : '') : 'none';
            if (state !== spoken) { spoken = state; live.textContent = message; }
        }
        const events = ['input', 'change', 'blur'];
        events.forEach(name => input.addEventListener(name, update));
        update();
        // The handle sits on the input: code that writes the value (renderHead, renderHeader) calls input.edLimit.update().
        input.edLimit = {
            update,
            detach() {
                input.edLimit = null;
                events.forEach(name => input.removeEventListener(name, update));
                note.remove();
                live.remove();
                if (described) input.setAttribute('aria-describedby', described); else input.toggleAttribute('aria-describedby', false);
            }
        };
        return input.edLimit;
    }

    // popover anchors a small dialog panel to a control (W1c): opts {content (an element, or markup), label (the dialog's name),
    // className, onClose({restore})}. It sits under the anchor inside the nearest .ed-editor (else the body), left-aligned to the
    // anchor and kept inside that host. Focus moves to the first button in it (else the panel); Escape closes it and gives the focus
    // back to the anchor; a press outside the panel and the anchor, and focus that moves elsewhere, close it without moving the focus.
    // Returns {el, close(restoreFocus), update(content), isOpen()}. update swaps the content and keeps the focus on the same
    // [data-ed-pop] control. Nothing animates on close: the panel is removed at once.
    function popover(anchor, opts) {
        const o = opts || {};
        const host = (anchor.closest && anchor.closest('.ed-editor')) || document.body;
        const el = ED.core.el('<div class="ed-popover ed-popover--anchored" role="dialog" tabindex="-1"></div>');
        if (o.label) el.setAttribute('aria-label', text(o.label));
        if (o.className) o.className.split(/\s+/).filter(Boolean).forEach(name => el.classList.add(name));
        let open = true;
        function fill(content) {
            el.innerHTML = '';
            if (content && typeof content === 'object') el.appendChild(content); else el.innerHTML = text(content);
        }
        function place() {
            const h = host.getBoundingClientRect(), a = anchor.getBoundingClientRect();
            const left = Math.max(8, Math.min(a.left - h.left, h.width - 368));
            el.style.left = left + 'px';
            el.style.top = Math.max(8, a.bottom - h.top + 6) + 'px';
        }
        function close(restore) {
            if (!open) return;
            open = false;
            document.removeEventListener('pointerdown', outside, true);
            el.remove();
            if (restore && typeof anchor.focus === 'function') anchor.focus();
            if (typeof o.onClose === 'function') o.onClose({ restore: !!restore });
        }
        function outside(event) {
            if (!el.contains(event.target) && !anchor.contains(event.target)) close(false);
        }
        el.addEventListener('keydown', event => {
            if (event.key !== 'Escape') return;
            event.preventDefault();
            event.stopPropagation();
            close(true);
        });
        el.addEventListener('focusout', event => {
            const to = event.relatedTarget;
            if (to && !el.contains(to) && !anchor.contains(to)) close(false);
        });
        function update(content) {
            const was = document.activeElement && el.contains(document.activeElement) ? document.activeElement.getAttribute('data-ed-pop') : null;
            fill(content);
            place();
            const again = was ? el.querySelector('[data-ed-pop="' + was + '"]') : null;
            if (again && typeof again.focus === 'function') again.focus();
        }
        fill(o.content);
        host.appendChild(el);
        place();
        document.addEventListener('pointerdown', outside, true);
        const first = el.querySelector('button');
        (first || el).focus();
        return { el, close, update, isOpen: () => open };
    }

    ED.kit = { disclosure, disclosureMarkup, attach, isOpen, setOpen, setTitle, setCount, body, callout, chip, toast, toolbar, flash, limit, popover };
})();
