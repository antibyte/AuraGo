// HomeDash in-root modal sheet (F2; design-frontend 6). trap({container, layer, dialog, opener, onEscape, onBackdrop}) makes `layer` modal inside
// `container` until release(): the container's other children are inert (a child with aria-live stays live), Tab and Shift+Tab stay in the layer,
// Escape runs onEscape, a click on the layer itself runs onBackdrop, and release() restores the inert state and focuses the opener (default: the
// element that had focus). create(container) builds one E.prim.sheet layer on top of trap. open({title, text: [paragraphs], actions: [{id, label,
// tone, autofocus}], kind}) resolves the chosen action id, or null for Escape, a click on the backdrop or close(). The confirm helper (title, text,
// confirmLabel, cancelLabel, tone) resolves true or false; a danger one starts on Cancel. The light sheet (hd-moreinfo.js) uses trap directly.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.prim || !E.prim.sheet) throw new Error('hd-sheet.js: load hd-core.js and hd-prim-displays.js first');
    const { el, setAttr } = E.core, FOCUSABLE = 'button, [href], input, select, textarea, [tabindex]';
    function trap(o) {
        const opts = o || {}, container = opts.container, layer = opts.layer, doc = container.ownerDocument, life = E.core.bag();
        const opener = opts.opener === undefined ? doc.activeElement || null : opts.opener;
        const inerted = Array.from(container.children).filter(n => n !== layer && !n.hasAttribute('inert') && !n.hasAttribute('aria-live'));
        for (const n of inerted) n.setAttribute('inert', '');
        const focusables = () => Array.from(layer.querySelectorAll(FOCUSABLE)).filter(n => !n.hasAttribute('disabled') && n.getAttribute('tabindex') !== '-1' && !n.closest('[hidden]'));
        life.on(layer, 'click', ev => { if (ev.target === layer && opts.onBackdrop) opts.onBackdrop(); });
        life.on(layer, 'keydown', ev => {
            if (ev.key === 'Escape') { ev.preventDefault(); ev.stopPropagation(); if (opts.onEscape) opts.onEscape(); return; }
            if (ev.key !== 'Tab') return;
            const list = focusables(); if (!list.length) return;
            const i = list.indexOf(doc.activeElement), next = ev.shiftKey ? (i <= 0 ? list.length - 1 : i - 1) : (i === list.length - 1 ? 0 : i + 1);
            ev.preventDefault(); list[next].focus();
        });
        let done = false;
        return function release() {
            if (done) return; done = true; life.dispose();
            for (const n of inerted) n.removeAttribute('inert');
            if (opener && typeof opener.focus === 'function' && opener.isConnected !== false) opener.focus();
        };
    }
    function create(container) {
        const sheet = E.prim.sheet({ kind: 'confirm' }), life = E.core.bag(), dialog = sheet.el.querySelector('.hd-sheet');
        container.appendChild(sheet.el);
        let resolveOpen = null, release = null;
        function close(value) {
            if (!resolveOpen) return;
            const done = resolveOpen; resolveOpen = null;
            sheet.set({ open: false }); sheet.foot.replaceChildren(); sheet.body.replaceChildren();
            const off = release; release = null; if (off) off();
            done(value === undefined ? null : value);
        }
        function open(spec) {
            close(null);
            const s = spec || {};
            sheet.set({ open: true, title: s.title || '' }); setAttr(dialog, 'data-kind', s.kind || 'confirm');
            for (const line of [].concat(s.text || [])) sheet.body.appendChild(el('p', { class: 'hd-sheet-text', text: line }));
            let first = null;
            for (const a of s.actions || []) {
                const b = el('button', { type: 'button', class: 'hd-sheet-btn', 'data-action': a.id, 'data-tone': a.tone || 'default', text: a.label });
                sheet.foot.appendChild(b); if (a.autofocus || !first) first = b;
            }
            release = trap({ container, layer: sheet.el, dialog, onEscape: () => close(null), onBackdrop: () => close(null) });
            return new Promise(resolve => { resolveOpen = resolve; if (first) first.focus(); });
        }
        life.on(sheet.el, 'click', ev => { const b = ev.target && ev.target.closest ? ev.target.closest('.hd-sheet-btn') : null; if (b) close(b.getAttribute('data-action')); });
        return Object.freeze({
            el: sheet.el, open, close, isOpen: () => !!resolveOpen,
            confirm: spec => open({ title: spec.title, text: spec.text, kind: 'confirm', actions: [{ id: 'cancel', label: spec.cancelLabel, tone: 'default' },
                { id: 'confirm', label: spec.confirmLabel, tone: spec.tone || 'primary', autofocus: spec.tone !== 'danger' }] }).then(v => v === 'confirm'),
            destroy() { close(null); life.dispose(); sheet.destroy(); }
        });
    }
    E.sheet = Object.freeze({ create, trap });
})();
